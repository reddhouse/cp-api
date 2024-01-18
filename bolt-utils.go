package main

import (
	"encoding/json"
	"log"

	bolt "go.etcd.io/bbolt"
)

type Bolt_Utils struct {
	db *bolt.DB
}

func (bu *Bolt_Utils) writeSequentially(bucketName string, s IdSetter) {
	var id uint64
	err := bu.db.Update(func(tx *bolt.Tx) error {
		// Retrieve the USER bucket.
		b := tx.Bucket([]byte(bucketName))
		// Generate an ID for this user based on existing sequence.
		// This returns an error only if the Tx is closed or not writeable.
		// That can't happen in an Update() call so ignore the error check.
		id, _ = b.NextSequence()
		// Set the Id field in the struct whose setter was passed in.
		s.SetId(int(id))

		// Marshal user struct into JSON (byte slice).
		buf, err := json.Marshal(s)
		if err != nil {
			return err
		}

		// Persist bytes to USER bucket.
		return b.Put(itob(int(id)), buf)
	})
	if err != nil {
		log.Fatalf("failed to persist user to db: %v", err)
	}
}
