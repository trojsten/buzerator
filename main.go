package main

import (
	"os"
	"time"

	"github.com/charmbracelet/log"
)

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

	App.wg.Add(2)
	go ConnectSlack()
	go ServeUI()
	go RunScheduler()

	<-time.After(5 * time.Second)
	App.wg.Wait()
}
