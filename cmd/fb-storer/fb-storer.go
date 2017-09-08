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
	"time"
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

	c, err := NewConsumer("comments-store", "sometag")
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf("Ready to receive events on queue %s", "comments-store")
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

	go storeComments(deliveries, c.done, cfg)

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

func storeComments(deliveries <-chan amqp.Delivery, done chan error, cfg *config.Config) {

	// Setup ElasticSearch client
	es := es.New(cfg.ES)
	ctx := context.Background()

	var last_index string
	for d := range deliveries {
		// Create day-index if needed
		date := time.Now()
		index_name := fmt.Sprintf("fbcomments-%04d.%02d.%02d", date.Year(), date.Month(), date.Day())
		if last_index != index_name {
			exists, err := es.Client.IndexExists(index_name).Do(ctx)
			if err != nil {
				log.Fatalf("IndexExists(%s) error: %s", index_name, err)
			}

			if !exists {
				_, err := es.Client.CreateIndex(index_name).BodyString(fbbot.CommentMapping).Do(ctx)
				if err != nil {
					log.Fatalf("Cannot create ES index: %s", err)
				}
				log.Infof("Created new index %s", index_name)
			}
		}

		// Remember last index used
		last_index = index_name

		entry := fbbot.FBComment{}
		log.Debugf(
			"got %d B delivery: [%v] %q",
			len(d.Body),
			d.DeliveryTag,
			d.Body,
		)
		if err := json.Unmarshal(d.Body, &entry); err != nil {
			log.Fatalf("Cannot read message: %s", err)
		}

		put1, err := es.Client.Index().
			Index(index_name).
			Type("fbcomment").
			Id(entry.ID).
			BodyJson(entry).
			Do(ctx)

		if err != nil {
			log.Errorf("Index failed: %s", err)
			d.Nack(false, true)
		} else {
			log.Infof("Indexed comment %s to index %s, type %s", put1.Id, put1.Index, put1.Type)
			d.Ack(false)
		}
	}
	log.Infof("handle: deliveries channel closed")
	done <- nil
}
