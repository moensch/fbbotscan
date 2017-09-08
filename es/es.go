package es

import (
	"fmt"
	"github.com/moensch/fbbotscan/config"
	"gopkg.in/olivere/elastic.v5"
	"log"
)

type ES struct {
	Client *elastic.Client
	Config *config.ESConfig
}

func New(cfg *config.ESConfig) *ES {
	es := &ES{
		Config: cfg,
	}

	if err := es.Initialize(); err != nil {
		log.Fatalf("Failed to initialize ES: %s", err)
	}

	return es
}

func (es *ES) Initialize() error {
	//log.Debugf("ES URL: %s", es.Config.URL)

	var err error
	/*
		es.Client, err = elastic.NewClient(
			elastic.SetURL(fmt.Sprintf("%s", es.Config.URL)),
			elastic.SetErrorLog(log.New(os.Stderr, "ELASTIC ", log.LstdFlags)),
			elastic.SetInfoLog(log.New(os.Stdout, "", log.LstdFlags)),
			elastic.SetTraceLog(log.New(os.Stderr, "[[ELASTIC]]", 0)),
		)
	*/
	es.Client, err = elastic.NewClient(
		elastic.SetURL(fmt.Sprintf("%s", es.Config.URL)),
	)

	if err != nil {
		return fmt.Errorf("Cannot setup Elastic Client: %s", err)
	}

	return nil
}
