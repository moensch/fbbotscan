package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	fbbot "github.com/moensch/fbbotscan"
	"github.com/moensch/fbbotscan/config"
	"github.com/moensch/fbbotscan/es"
	"github.com/moensch/fbbotscan/pubsub"
	"github.com/streadway/amqp"
	"gopkg.in/olivere/elastic.v5"
)

var (
	configFile string
	logLevel   string
)

func init() {
	flag.StringVar(&configFile, "f", "/etc/fbbotscan.toml", "Path to TOML configuration file")
	flag.StringVar(&logLevel, "l", "error", "Log level (debug|info|warn|error)")
}

func main() {
	flag.Parse()
	lvl, _ := log.ParseLevel(logLevel)
	log.SetLevel(lvl)

	c, err := NewConsumer("comments-classify", "sometag")
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf("Ready to receive events on queue %s", "comments-classify")
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
	pubsub  *pubsub.PubSub
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

	log.Printf("Discarding matches scoring lower than %3.5f", cfg.Classify.DiscardScore)
	log.Printf("Considering matches scoring higher ghan %3.5f", cfg.Classify.MatchScore)
	log.Printf("dialing %q", cfg.AMQP.URI)
	c.conn, err = amqp.Dial(cfg.AMQP.URI)
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

	go classifyComments(deliveries, c.done, cfg)

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

func classifyComments(deliveries <-chan amqp.Delivery, done chan error, cfg *config.Config) {

	// Setup ElasticSearch client
	es := es.New(cfg.ES)
	ctx := context.Background()

	for d := range deliveries {
		entry := fbbot.FBComment{}
		if err := json.Unmarshal(d.Body, &entry); err != nil {
			log.Fatalf("Cannot read message: %s", err)
		}

		// Using MLT query
		// GET _search
		// {
		//   "query": {
		//     "more_like_this" : {
		//       "fields" : ["message"],
		//       "like" : ["Lorem ipsum dolor sit amet, consectetur adipiscing elit. rdiet nulla. Vestibulum ac ex rhoncus, semper nisi ut, consequat metus. Mauris dignissim dignissim ex, id condimentum mauris. Morbi cursus sapien vel justo convallis, vitae commodo est dignissim. Nulla ut lorem nec mauris blandit volutpat vitae sit amet velit. Nullam sit amet consequat quam. Proin ut augue porta, consequat sapien sodalque tortor. Donec vel diam cursus, facilisis neque ac, dignissim purus. Praesent fringilla at est in luctus. Nunc pulvinar."],
		//       "min_term_freq" : 1,
		//       "max_query_terms": 150,
		//       "min_doc_freq":1
		//     }
		//   }
		// }

		log.Infof("Classify comment %s from %s: %s", entry.ID, entry.From.ID, entry.Message)
		if entry.Message != "" {
			mlt := elastic.NewMoreLikeThisQuery().
				Analyzer("english").
				Field("message").
				LikeText(entry.Message).
				MinDocFreq(1).
				MaxQueryTerms(150).
				MinTermFreq(1).
				MinimumShouldMatch("60%")

			// TODO: Dynamic index search
			searchResult, err := es.Client.Search().
				Index("fbcomments-2017.09.07").
				Index("fbcomments-2017.09.08").
				Query(mlt).
				Pretty(true).
				Do(ctx)
			if err != nil {
				log.Errorf("Search failure: %s", err)
			}

			log.Debugf("Query took %d milliseconds", searchResult.TookInMillis)
			log.Debugf("Found a total of %d matches", searchResult.TotalHits())
			/*
				var ttyp fbbot.FBComment
				for _, item := range searchResult.Each(reflect.TypeOf(ttyp)) {
					if t, ok := item.(fbbot.FBComment); ok {
						log.Infof("FBComment by %s: %s\n", t.From.ID, t.Message)
					}
				}
			*/

			for _, hit := range searchResult.Hits.Hits {
				var comment fbbot.FBComment
				err := json.Unmarshal(*hit.Source, &comment)
				if err != nil {
					log.Errorf("Cannot deserialize search result: %s", err)
					continue
				}
				if comment.ID == entry.ID {
					log.Debugf("Discarding match which is the same comment ID")
					continue
				}

				switch {
				case *hit.Score < cfg.Classify.DiscardScore:
					log.Infof("Found matching comment in Index %s, score %3.5f, ID %s from %s: %s", hit.Index, *hit.Score, comment.ID, comment.From.ID, comment.Message)
				case *hit.Score > cfg.Classify.MatchScore:
					log.Errorf("Found matching comment in Index %s, score %3.5f, ID %s from %s: %s", hit.Index, *hit.Score, comment.ID, comment.From.ID, comment.Message)
				default:
					log.Warnf("Found matching comment in Index %s, score %3.5f, ID %s from %s: %s", hit.Index, *hit.Score, comment.ID, comment.From.ID, comment.Message)
				}
			}
		}
		d.Ack(false)
	}
	log.Infof("handle: deliveries channel closed")
	done <- nil
}
