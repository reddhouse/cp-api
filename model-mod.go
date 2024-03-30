package main

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

type Exim struct {
	Author     string `json:"author"`
	IsApproved bool   `json:"isApproved"`
	Target     string `json:"target"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Paragraph1 string `json:"paragraph1"`
	Paragraph2 string `json:"paragraph2"`
	Paragraph3 string `json:"paragraph3"`
	Link       string `json:"link"`
}

// Writes Exim to db.
func (e *Exim) createEximTx(binId []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Retrieve buckets.
		eb := tx.Bucket([]byte("MOD_EXIM"))

		// TODO: Delete me.
		stats := eb.Stats()
		fmt.Printf("[api] There are currently %d keys in the MOD_EXIM bucket [%s]\n", stats.KeyN, cts())

		// Marshal Exim to be stored.
		eximJs, err := json.Marshal(e)
		if err != nil {
			return err
		}

		// Write key/value pair.
		if err := eb.Put(binId, eximJs); err != nil {
			return err
		}

		return nil
	})
}
