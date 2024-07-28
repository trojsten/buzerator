package main

import (
	"encoding/json"

	"github.com/charmbracelet/log"
	"go.etcd.io/bbolt"
)

type Team struct {
	ID    string
	Name  string
	Token string
}

func ListTeams() ([]Team, error) {
	teams := []Team{}

	err := App.db.View(func(tx *bbolt.Tx) error {
		teamsBucket := tx.Bucket([]byte("teams"))
		return teamsBucket.ForEach(func(k, v []byte) error {
			var team Team
			err := json.Unmarshal(v, &team)
			if err != nil {
				return err
			}

			teams = append(teams, team)
			return nil
		})
	})

	return teams, err
}

func (t *Team) Save() error {
	return App.db.Update(func(tx *bbolt.Tx) error {
		teamsBucket := tx.Bucket([]byte("teams"))

		data, err := json.Marshal(t)
		if err != nil {
			return err
		}

		return teamsBucket.Put([]byte(t.ID), data)
	})
}

func (t *Team) Connect() {
	App.wg.Add(1)
	defer App.wg.Done()

	err := ConnectTeam(*t)
	if err != nil {
		log.Error("Team disconnected.", "team", t.ID, "err", err)
	}
}
