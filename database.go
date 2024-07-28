package main

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

func OpenDatabase(filename string) error {
	var err error
	App.db, err = bolt.Open(filename, 0600, nil)
	if err != nil {
		return err
	}

	err = App.db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte("messages"))
		if err != nil {
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte("questions"))
		if err != nil {
			return err
		}

		_, err = tx.CreateBucketIfNotExists([]byte("teams"))
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	if App.config.MigrateToTeam != "" {
		return migrateDatabase()
	}

	return nil
}

func migrateDatabase() error {
	return App.db.Update(func(tx *bolt.Tx) error {
		questions := tx.Bucket([]byte("questions"))

		return questions.ForEach(func(k, v []byte) error {
			var question Question
			err := json.Unmarshal(v, &question)
			if err != nil {
				return err
			}

			if question.TeamID == "" {
				question.TeamID = App.config.MigrateToTeam
			}

			data, err := json.Marshal(question)
			if err != nil {
				return err
			}
			return questions.Put(k, data)
		})
	})
}
