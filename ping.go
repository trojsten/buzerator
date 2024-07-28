package main

import (
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/slack-go/slack"
	bolt "go.etcd.io/bbolt"
	"strconv"
	"strings"
	"time"
)

const pingCron = "10 16 * * 1,3,5"

func PingMissingUsers() error {
	// teamID, userID, []channelID
	teamUserChannels := map[string]map[string][]string{}

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

			ts, err := strconv.ParseFloat(qi.Timestamp, 64)
			if err != nil {
				log.Error("Cannot parse timestamp for message.", "message", string(k), "err", err)
				return nil // we ignore this error as it should not really happen, and it should not break the loop
			}

			if qi.Question.CurrentInstance != qi.Timestamp {
				log.Debug("Skipping - not current.", "instance", qi.Timestamp, "current", qi.Question.CurrentInstance)
				return nil
			}

			posted := time.Unix(int64(ts), 0)
			if time.Now().Sub(posted) < 24*time.Hour {
				log.Debug("Skipping - too soon.", "instance", qi.Timestamp)
				return nil
			}

			for user, replied := range qi.Responses {
				if !replied {
					if _, ok := teamUserChannels[qi.Question.TeamID]; !ok {
						teamUserChannels[qi.Question.TeamID] = make(map[string][]string)
					}
					teamUserChannels[qi.Question.TeamID][user] = append(teamUserChannels[qi.Question.TeamID][user], qi.Question.Channel)
				}
			}
			return nil
		})
	})
	if err != nil {
		return err
	}

	for team, userChannels := range teamUserChannels {
		client, ok := App.slack[team]
		if !ok {
			log.Error("Not pinging team as we do not have a connection there.", "team", team)
			continue
		}

		for user, channels := range userChannels {
			log.Info("Pinging.", "team", team, "user", user, "channels", channels)
			msg := "Ahoj, zatiaľ si sa nevyjadril/-a do môjho update threadu v týchto kanáloch:\n%s\nNájdi si prosím minútku a doplň odpovede 😇"
			var channelMentions []string
			for _, channel := range channels {
				channelMentions = append(channelMentions, fmt.Sprintf("<#%s>", channel))
			}

			_, _, err := client.PostMessage(user, slack.MsgOptionText(fmt.Sprintf(msg, strings.Join(channelMentions, ", ")), false))
			if err != nil {
				log.Error("Could not send ping message.", "user", user, "err", err)
			}
		}
	}
	return nil
}
