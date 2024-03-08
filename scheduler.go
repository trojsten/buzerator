package main

import (
	"encoding/json"
	"github.com/adhocore/gronx"
	"github.com/charmbracelet/log"
	bolt "go.etcd.io/bbolt"
	"time"
)

func schedulerTick() {
	logger := log.WithPrefix("scheduler")
	logger.Debug("Running scheduler tick.")
	var questions []Question

	err := App.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("questions")).ForEach(func(k, v []byte) error {
			var q Question
			err := json.Unmarshal(v, &q)
			if err != nil {
				return err
			}

			if q.IsActive {
				questions = append(questions, q)
			}
			return nil
		})
	})
	if err != nil {
		logger.Error("Cannot list questions.", "err", err)
	}

	gron := gronx.New()
	now := time.Now().Truncate(1 * time.Minute)

	for _, question := range questions {
		qlog := logger.With("question", question.ID)
		due, err := gron.IsDue(question.Cron, now)
		if err != nil {
			qlog.Error("Error while checking cron.", "err", err)
			continue
		}

		if due {
			qlog.Info("Creating new instance of a question.")
			err = question.NewInstance()
			if err != nil {
				qlog.Error("Could not create new instance.", "err", err)
			}
		}
	}
}

func RunScheduler() {
	time.Sleep(30 * time.Second)
	defer App.wg.Done()

	for {
		go schedulerTick()
		time.Sleep(1 * time.Minute)
	}
}
