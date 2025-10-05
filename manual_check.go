package main

import (
	"encoding/json"

	"github.com/charmbracelet/log"
	"github.com/slack-go/slack"
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
		slackErr, ok := err.(slack.SlackErrorResponse)
		if ok && (slackErr.Err == "not_in_channel" || slackErr.Err == "channel_not_found") {
			if slackErr.Err == "not_in_channel" {
				log.Info("I am no longer in the channel. Deleting question.", "question", inst.QuestionID, "channel", inst.Question.Channel)
			} else {
				log.Info("Channel is archived or not found. Deleting question.", "question", inst.QuestionID, "channel", inst.Question.Channel)
			}
			err := inst.Question.Delete()
			if err != nil {
				log.Error("Could not delete question.", "question", inst.QuestionID, "err", err)
			}
			continue
		}

		if err != nil {
			log.Error("Could not check new messages.", "question", inst.QuestionID, "err", err)
		}
	}

	return nil
}
