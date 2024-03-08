package main

import (
	"encoding/json"
	"github.com/charmbracelet/log"
	bolt "go.etcd.io/bbolt"
)

const manualCheckCron string = "25 * * * *"

func CheckAllThreads() error {
	var instances []QuestionInstance

	err := App.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("messages")).ForEach(func(k, v []byte) error {
			var qi QuestionInstance
			err := json.Unmarshal(v, &qi)
			if err != nil {
				return err
			}

			err = qi.LoadQuestion()
			if err != nil {
				return err
			}

			instances = append(instances, qi)
			return nil
		})
	})
	if err != nil {
		return err
	}

	for _, inst := range instances {
		err = inst.CheckNewMessages()
		if err != nil {
			log.Error("Could not check new messages.", "question", inst.QuestionID, "err", err)
		}
	}

	return nil
}
