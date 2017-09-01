package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	fbbotscan "github.com/moensch/fbbotscan"
	"os"
)

var (
	appId        string
	appSecret    string
	logLevel     string
	monitorPages []string
)

func init() {
	flag.StringVar(&appId, "a", os.Getenv("FB_APP_ID"), "Application ID")
	flag.StringVar(&appSecret, "s", os.Getenv("FB_APP_SECRET"), "Application Secret")
	flag.StringVar(&logLevel, "l", "error", "Log level (debug|info|warn|error)")
}

func main() {
	flag.Parse()

	monitorPages := [...]string{
		"HuffPost",
		"washingtonpost",
	}

	lvl, _ := log.ParseLevel(logLevel)
	log.SetLevel(lvl)

	var app = fbbotscan.New(appId, appSecret)

	for _, pageId := range monitorPages {
		log.Infof("Scanning Page: %s", pageId)
		posts, err := app.LoadFeed(pageId, 2, 1504239401)
		if err != nil {
			log.Fatalf("ERROR: %s", err)
		}

		for _, post := range posts {
			log.Infof("Post ID: %s", post.ID)

			comments, err := app.LoadComments(post.ID, 0)
			if err != nil {
				log.Fatalf("Failed to load comments on %s: %s", post.ID, err)
			}

			for _, comment := range comments {
				log.Infof("  Comment ID: %s (From: %s)", comment.ID, comment.From.Name)
				subcomments, err := app.LoadComments(comment.ID, 0)
				if err != nil {
					log.Fatalf("Failed to load subcomments on %s: %s", post.ID, err)
				}

				for _, subcomment := range subcomments {
					log.Infof("    Sub-Comment ID: %s", subcomment.ID)
				}
			}
		}
	}
}
