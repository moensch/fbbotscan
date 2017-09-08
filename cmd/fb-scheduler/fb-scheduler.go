package main

import (
	"database/sql"
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"
	fb "github.com/moensch/fbbotscan"
	"github.com/moensch/fbbotscan/config"
	"github.com/moensch/fbbotscan/db"
	"github.com/moensch/fbbotscan/pubsub"
	"time"
)

var (
	logLevel   string
	configFile string
)

func init() {
	flag.StringVar(&configFile, "f", "/etc/fbbotscan.toml", "Path to TOML configuration file")
	flag.StringVar(&logLevel, "l", "error", "Log level (debug|info|warn|error)")
}

func main() {
	flag.Parse()
	lvl, _ := log.ParseLevel(logLevel)
	log.SetLevel(lvl)

	cfg, err := config.LoadFile(configFile)
	if err != nil {
		log.Fatalf("%s", err)
	}
	publisher := pubsub.New(cfg.AMQP)
	appdb := db.New(cfg.DB)

	if err := appdb.Connect(); err != nil {
		log.Fatalf("%s", err)
	}
	if err := publisher.Connect(); err != nil {
		log.Fatalf("%s", err)
	}
	if err := publisher.SetupChannel(); err != nil {
		log.Fatalf("Failed to get AMQP channel")
	}

	for _, queueName := range []string{"comments", "posts"} {
		log.Infof("Declaring queue for: %s-fetch", queueName)
		queue, err := publisher.QueueDeclare(fmt.Sprintf("%s-fetch", queueName))

		if err != nil {
			log.Fatalf("Failed to declare queue: %s", err)
		}

		log.Infof("Declared AMQP queue: %s", queue.Name)
	}

	for {
		// Check pages for new posts
		log.Infof("Loading page feeds that haven't been checked in %d seconds", cfg.FB.FeedCheckSeconds)
		rows, err := appdb.GetSchedulerPosts(cfg.FB.FeedCheckSeconds)
		if err != nil {
			log.Fatalf("%s", err)
		}

		for rows.Next() {
			entry := fb.QueueEntry{}

			var last_checked sql.NullInt64
			if err := rows.Scan(&entry.ObjectID, &entry.ObjectType, &last_checked); err != nil {
				log.Errorf("Scan error: %s", err)
			}

			if last_checked.Valid == true {
				entry.LastChecked = last_checked.Int64
			} else {
				entry.LastChecked = 1
			}

			log.Infof("Scheduling page to scan feed: %s (last checked: %s)", entry.ObjectID, time.Unix(entry.LastChecked, 0).String())

			err = publisher.PublishJSON(
				"posts-fetch",
				entry,
			)

			if err != nil {
				log.Fatalf("Failed to publish: %s", err)
			}
			if err := appdb.SetScheduled(entry.ObjectType, entry.ObjectID); err != nil {
				log.Fatalf("Cannot set to scheduled: %s", err)
			}
		}

		// Check new objects for comments
		log.Infof("Loading object comments that haven't been checked in %d seconds", cfg.FB.CommentsCheckSeconds)
		rows, err = appdb.GetSchedulerComments(cfg.FB.CommentsCheckSeconds)
		if err != nil {
			log.Fatalf("%s", err)
		}

		for rows.Next() {
			body := fb.QueueEntry{}

			var last_checked sql.NullInt64
			if err := rows.Scan(&body.ObjectID, &body.ObjectType, &last_checked); err != nil {
				log.Errorf("Scan error: %s", err)
			}

			if last_checked.Valid == true {
				body.LastChecked = last_checked.Int64
			} else {
				body.LastChecked = 1
			}

			log.Infof("Scheduling %s to scan for comments: %s (last checked: %s)", body.ObjectType, body.ObjectID, time.Unix(body.LastChecked, 0).String())

			err = publisher.PublishJSON(
				"comments-fetch",
				body,
			)

			if err != nil {
				log.Fatalf("Failed to publish: %s", err)
			}

			if err := appdb.SetScheduled(body.ObjectType, body.ObjectID); err != nil {
				log.Fatalf("Cannot set to scheduled: %s", err)
			}
		}

		// Sleep and re-do
		log.Infof("Sleeping")
		time.Sleep(5 * time.Second)
	}
}
