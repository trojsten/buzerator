package main

import (
	"github.com/slack-go/slack/socketmode"
	bolt "go.etcd.io/bbolt"
	"sync"
)

var App application

type application struct {
	db       *bolt.DB
	slack    *socketmode.Client
	myUserId string
	wg       sync.WaitGroup
	webUI    *webUI
	config   Config
}
