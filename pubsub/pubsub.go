package pubsub

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/moensch/fbbotscan/config"
	"github.com/streadway/amqp"
)

type PubSub struct {
	Config  *config.AMQPConfig
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func New(cfg *config.AMQPConfig) *PubSub {
	pubsub := &PubSub{
		Config: cfg,
	}

	if err := pubsub.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %s", err)
	}

	return pubsub
}

func (p *PubSub) Initialize() error {
	log.Debugf("AMQP Connection: %s", p.Config.URI)
	return nil
}

func (p *PubSub) Connect() error {
	log.Debugf("Connecting to AMQP: %s", p.Config.URI)
	var err error
	p.Conn, err = amqp.Dial(p.Config.URI)
	if err != nil {
		return fmt.Errorf("Cannot connect to AMQP: %s", err)
	}
	log.Info("Successfully connected to AMQP server")

	return err
}

func (p *PubSub) SetupChannel() error {
	var err error
	p.Channel, err = p.Conn.Channel()
	if err != nil {
		return err
	}
	return err
}

// Setup a persistent queue
func (p *PubSub) QueueDeclare(name string) (amqp.Queue, error) {
	return p.Channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // noWait
		nil,   // arguments
	)
}

// Shortcut to publishing arbitrary interfaces
//  as JSON
func (p *PubSub) PublishJSON(routingKey string, data interface{}) error {
	jsonblob, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("Failed to create JSON message: %s", err)
	}

	return p.Publish(routingKey, "application/json", jsonblob)
}

// Publish a byte string using a given routing key (in our case, always the queue name)
func (p *PubSub) Publish(routingKey string, contentType string, body []byte) error {
	log.Debugf("Sending message to %s: %s", routingKey, string(body))

	err := p.Channel.Publish(
		"",         // default exchange
		routingKey, // routing key (queue name)
		true,       // mandatory
		false,      // immediate
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     contentType,
			ContentEncoding: "",
			Body:            body,
			DeliveryMode:    amqp.Persistent,
			Priority:        0,
		},
	)

	return err
}
