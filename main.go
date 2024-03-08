package main

import (
	"os"
)

import "github.com/charmbracelet/log"

func main() {
	err := App.config.Load()
	if err != nil {
		log.Error("Could not load config.", "err", err)
		os.Exit(1)
	}

	if App.config.Debug {
		log.SetLevel(log.DebugLevel)
	}

	err = OpenDatabase(App.config.DatabaseFile)
	if err != nil {
		log.Error("Could not open database.", "err", err)
		os.Exit(1)
	}

	App.wg.Add(3)
	go ConnectSlack()
	go ServeUI()
	go RunScheduler()
	App.wg.Wait()
}
