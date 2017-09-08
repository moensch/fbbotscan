package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	log "github.com/Sirupsen/logrus"
	"os"
)

type Config struct {
	FB       *FBConfig
	AMQP     *AMQPConfig
	DB       *DBConfig
	ES       *ESConfig
	Classify *ClassifyConfig
}

type FBConfig struct {
	AppID                string `toml:"app_id"`
	AppSecret            string `toml:"app_secret"`
	FeedCheckSeconds     int    `toml:"feed_check_seconds"`
	CommentsCheckSeconds int    `toml:"comments_check_seconds"`
}

type AMQPConfig struct {
	URI string `toml:"uri"`
}

type DBConfig struct {
	Type    string `toml:"type"`
	Connstr string `toml:"connstr"`
}

type ESConfig struct {
	URL string `toml:"url"`
}

type ClassifyConfig struct {
	DiscardScore float64 `toml:"discard_score"`
	MatchScore   float64 `toml:"match_score"`
}

func (c *Config) Valid() bool {
	// TODO
	return true
}

func LoadFile(path string) (*Config, error) {
	log.Debugf("loading configuration file from %s", path)
	if path == "" {
		return nil, fmt.Errorf("Invalid path: %s", path)
	}

	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("Cannot find config file %s: %s", path, err)
	}

	var cfg = &Config{}

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, fmt.Errorf("Unable to load config file: %s", err)
	}

	log.Debugf("Successfully loaded config %s", path)

	if !cfg.Valid() {
		return nil, fmt.Errorf("Invalid configuration")
	}
	return cfg, nil
}
