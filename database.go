package main

import bolt "go.etcd.io/bbolt"

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

		return nil
	})
	return err
}
