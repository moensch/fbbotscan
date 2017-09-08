package main

/*
The fetcher listens to events on the -fetch AMQP queues,
 pulls comments and posts from Facebook, stores metadata
 in postgres and publishes the full comment to the "comments"
 AMQP queue
*/
import (
	"encoding/json"
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	fbbot "github.com/moensch/fbbotscan"
	"github.com/moensch/fbbotscan/config"
	"github.com/moensch/fbbotscan/db"
	"github.com/moensch/fbbotscan/pubsub"
	"github.com/streadway/amqp"
	"strings"
	"time"
)

var (
	configFile string
	logLevel   string
	fetchType  string
)

func init() {
	flag.StringVar(&configFile, "f", "/etc/fbbotscan.toml", "Path to TOML configuration file")
	flag.StringVar(&logLevel, "l", "error", "Log level (debug|info|warn|error)")
	flag.StringVar(&fetchType, "t", "comments", "Fetch type (comments|posts)")
}

func main() {
	flag.Parse()
	lvl, _ := log.ParseLevel(logLevel)
	log.SetLevel(lvl)

	c, err := NewConsumer(fmt.Sprintf("%s-fetch", fetchType), "sometag")
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf("Ready to receive events on queue %s", fmt.Sprintf("%s-fetch", fetchType))
	select {}

	log.Printf("shutting down")

	if err := c.Shutdown(); err != nil {
		log.Fatalf("error during shutdown: %s", err)
	}
}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
	fbapp   *fbbot.FBApp
	pubsub  *pubsub.PubSub
	appdb   *db.DB
}

func NewConsumer(queueName, ctag string) (*Consumer, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     ctag,
		done:    make(chan error),
	}

	var err error

	cfg, err := config.LoadFile(configFile)
	if err != nil {
		log.Fatalf("%s", err)
	}
	c.pubsub = pubsub.New(cfg.AMQP)
	c.appdb = db.New(cfg.DB)
	if err := c.appdb.Connect(); err != nil {
		log.Fatalf("%s", err)
	}

	c.fbapp = fbbot.New(configFile)

	log.Printf("dialing %q", c.fbapp.Config.AMQP.URI)
	c.conn, err = amqp.Dial(c.fbapp.Config.AMQP.URI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		fmt.Printf("closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
	}()

	log.Printf("got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	log.Printf("Queue bound to Exchange, starting Consume (consumer tag %q)", c.tag)
	deliveries, err := c.channel.Consume(
		queueName, // name
		c.tag,     // consumerTag,
		false,     // noAck
		false,     // exclusive
		false,     // noLocal
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Consume: %s", err)
	}

	switch queueName {
	case "comments-fetch":
		go handleComments(deliveries, c.done, c.fbapp, c.appdb, cfg)
	case "posts-fetch":
		go handlePosts(deliveries, c.done, c.fbapp, c.appdb)
	}

	return c, nil
}

func (c *Consumer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Consumer cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer log.Printf("AMQP shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func handleComments(deliveries <-chan amqp.Delivery, done chan error, app *fbbot.FBApp, appdb *db.DB, cfg *config.Config) {
	pub := pubsub.New(cfg.AMQP)
	if err := pub.Connect(); err != nil {
		log.Fatalf("%s", err)
	}
	if err := pub.SetupChannel(); err != nil {
		log.Fatalf("Failed to get AMQP channel")
	}

	for _, queueName := range []string{"comments-store", "comments-classify"} {
		queue, err := pub.QueueDeclare(queueName)
		if err != nil {
			log.Fatalf("Failed to declare queue: %s", err)
		}

		log.Infof("Declared AMQP queue: %s", queue.Name)
	}

	for d := range deliveries {
		entry := fbbot.QueueEntry{}
		log.Debugf(
			"got %d B delivery: [%v] %q",
			len(d.Body),
			d.DeliveryTag,
			d.Body,
		)
		if err := json.Unmarshal(d.Body, &entry); err != nil {
			log.Fatalf("Cannot read message: %s", err)
		}

		log.Infof("Will fetch comments for: %s (type: %s / last checked: %d)", entry.ObjectID, entry.ObjectType, entry.LastChecked)

		now := time.Now().Unix()
		comments, err := app.LoadComments(entry.ObjectID, entry.LastChecked)

		// Extract post ID
		var post_id string
		if strings.Contains(entry.ObjectID, "_") {
			id_parts := strings.Split(entry.ObjectID, "_")
			switch entry.ObjectType {
			case "comment":
				// Comment IDs are "<post_id>_<comment_id>"
				post_id = id_parts[0]
			case "post":
				// Post IDs are "<page_id>_<post_id>"
				post_id = id_parts[1]
			}
		}
		if err != nil {
			log.Errorf("Failed to retrieve comments for %s: %s", entry.ObjectID, err)
			//d.Nack(false, true)
			d.Ack(false)
			continue // Next entry
		}

		err = appdb.UpdateLastCheck(entry.ObjectType, entry.ObjectID, now)
		if err != nil {
			log.Errorf("Failed to update last_check: %s", err)
			d.Nack(false, true)
		}

		for _, comment := range comments {
			log.Infof("  Comment ID: %s (From: %s) / Parent: %s", comment.ID, comment.From.Name, comment.Parent.ID)
			jsonblob, err := json.Marshal(comment)
			if err != nil {
				log.Errorf("Cannot do json: %s", err)
			}

			comment_id := strings.Split(comment.ID, "_")[1]

			// Store comment in database (metadata only)
			if err := appdb.InsertComment(comment_id, post_id, comment.Parent.ID, comment.From.ID); err != nil {
				log.Errorf("Insert failed: %s", err)
				log.Errorf("comment_id: %s / post_id: %s / parent_id: %s / from: %s", comment_id, post_id, comment.Parent.ID, comment.From.ID)
				log.Errorf("%s", comment.PermalinkURL)
				log.Errorf("JSON: %s", string(jsonblob))

			}

			// Publish full comment to store queue
			err = pub.PublishJSON(
				"comments-store",
				comment,
			)
			// Publish full comment to classify queue
			err = pub.PublishJSON(
				"comments-classify",
				comment,
			)
			if err != nil {
				log.Errorf("Cannot publish new comment: %s", err)
			}

		}

		log.Infof("Successfully fetched comments for %s %s", entry.ObjectType, entry.ObjectID)

		d.Ack(false)
	}
	log.Infof("handle: deliveries channel closed")
	done <- nil
}

func handlePosts(deliveries <-chan amqp.Delivery, done chan error, app *fbbot.FBApp, appdb *db.DB) {
	for d := range deliveries {
		entry := fbbot.QueueEntry{}
		log.Debugf(
			"got %d B delivery: [%v] %q",
			len(d.Body),
			d.DeliveryTag,
			d.Body,
		)
		if err := json.Unmarshal(d.Body, &entry); err != nil {
			log.Fatalf("Cannot read message: %s", err)
		}

		log.Infof("Will fetch feed for page %s (last checked: %d)", entry.ObjectID, entry.LastChecked)

		now := time.Now().Unix()
		posts, err := app.LoadFeed(entry.ObjectID, 5, entry.LastChecked)
		if err != nil {
			log.Errorf("Failed to retrieve feed for %s: %s", entry.ObjectID, err)
			d.Ack(false)
			continue // Next entry
		}

		err = appdb.UpdateLastCheck(entry.ObjectType, entry.ObjectID, now)
		if err != nil {
			log.Errorf("Failed to update last_check: %s", err)
			d.Nack(false, true)
		}

		for _, post := range posts {
			log.Infof("Post ID: %s / Permalink: %s", post.ID, post.PermalinkURL)

			if err := appdb.InsertPost(post.ID); err != nil {
				log.Errorf("Insert post failed: %s", err)
			}
		}
		d.Ack(false)
	}
	log.Infof("handle: deliveries channel closed")
	done <- nil
}
