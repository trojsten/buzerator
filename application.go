package main

import (
	"sync"

	"github.com/slack-go/slack/socketmode"
	bolt "go.etcd.io/bbolt"
)

var App application

type application struct {
	db     *bolt.DB
	slack  map[string]*socketmode.Client
	wg     sync.WaitGroup
	webUI  *webUI
	config Config
}
