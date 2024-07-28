package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pelletier/go-toml/query"
	bolt "go.etcd.io/bbolt"
)

type Question struct {
	ID              uint64   // question unique identifier
	Channel         string   // slack channel identifier
	Message         string   // question message text
	Users           []string // involved users
	Cron            string   // crontab expression of the question
	CurrentInstance string   // timestamp of the latest instance
	IsActive        bool     // whether this question is active
}

func (q *Question) Save() error {
	return App.db.Update(func(tx *bolt.Tx) error {
		questionsBucket := tx.Bucket([]byte("questions"))

		if q.ID == 0 {
			id, err := questionsBucket.NextSequence()
			if err != nil {
				return err
			}
			q.ID = id
		}

		data, err := json.Marshal(q)
		if err != nil {
			return err
		}

		return questionsBucket.Put([]byte(strconv.FormatUint(q.ID, 10)), data)
	})
}

func (q *Question) Delete() error {
	return App.db.Update(func(tx *bolt.Tx) error {
		instanceBucket := tx.Bucket([]byte("messages"))
		instances := [][]byte{}
		instanceBucket.ForEach(func(k, v []byte) error {
			var qi QuestionInstance
			err := json.Unmarshal(v, &qi)
			if err != nil {
				return err
			}

			if qi.QuestionID == q.ID {
				instances = append(instances, k)
			}
			return nil
		})

		for _, instance := range instances {
			err := instanceBucket.Delete(instance)
			if err != nil {
				return err
			}
		}

		questionsBucket := tx.Bucket([]byte("questions"))
		return questionsBucket.Delete([]byte(strconv.FormatUint(q.ID, 10)))
	})
}

func (q *Question) Instance() (QuestionInstance, error) {
	var instance QuestionInstance

	err := App.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket([]byte("messages")).Get([]byte(fmt.Sprintf("%s:%s", q.Channel, q.CurrentInstance)))
		if data == nil {
			return nil
		}

		err := json.Unmarshal(data, &instance)
		return err
	})

	instance.Question = q
	return instance, err
}

func (q *Question) NewInstance() error {
	qi := QuestionInstance{
		Question:   q,
		QuestionID: q.ID,
		Responses:  make(map[string]bool),
	}

	for _, user := range q.Users {
		qi.Responses[user] = false
	}

	err := qi.PostMessage()
	if err != nil {
		return fmt.Errorf("failed posting message: %w", err)
	}

	err = qi.Save()
	if err != nil {
		return fmt.Errorf("failed saving question instance: %w", err)
	}

	q.CurrentInstance = qi.Timestamp
	err = q.Save()
	if err != nil {
		return fmt.Errorf("failed saving question: %w", err)
	}

	return nil
}

func LoadQuestion(id uint64) (Question, error) {
	var q Question
	err := App.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket([]byte("questions")).Get([]byte(strconv.FormatUint(id, 10)))
		if data == nil {
			return nil
		}

		return json.Unmarshal(data, &q)
	})
	return q, err
}
