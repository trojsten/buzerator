package main

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	SlackAppToken string
	SlackBotToken string
	RootURL       string
	ListenAddress string
	DatabaseFile  string
	Debug         bool
}

func (c *Config) Load() error {
	c.SlackAppToken = os.Getenv("SLACK_APP_TOKEN")
	if !strings.HasPrefix(c.SlackAppToken, "xapp-") {
		return fmt.Errorf("slack app token should start with xapp")
	}

	c.SlackBotToken = os.Getenv("SLACK_BOT_TOKEN")
	if !strings.HasPrefix(c.SlackBotToken, "xoxb-") {
		return fmt.Errorf("slack bot token should start with xoxb")
	}

	c.RootURL = os.Getenv("ROOT_URL")
	if c.RootURL == "" {
		c.RootURL = "http://localhost:8080"
	}

	c.ListenAddress = os.Getenv("LISTEN_ADDRESS")
	if c.ListenAddress == "" {
		c.ListenAddress = ":8080"
	}

	c.DatabaseFile = os.Getenv("DATABASE_FILE")
	if c.DatabaseFile == "" {
		c.DatabaseFile = "data.db"
	}

	if os.Getenv("DEBUG") == "true" {
		c.Debug = true
	}

	return nil
}
