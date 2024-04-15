package main

import (
	"encoding/json"
	"fmt"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type Exim struct {
	EximId     ulid.ULID `json:"eximId"`
	Author     string    `json:"author"`
	IsApproved bool      `json:"isApproved"`
	Target     string    `json:"target"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	Paragraph1 string    `json:"paragraph1"`
	Paragraph2 string    `json:"paragraph2"`
	Paragraph3 string    `json:"paragraph3"`
	Link       string    `json:"link"`
}

type Exims []Exim

// Writes Exim to db.
func (e *Exim) createEximTx(binId []byte) error {
	// Marshal Exim to be stored.
	eximJs, err := json.Marshal(e)
	if err != nil {
		return err
	}

	return db.Update(func(tx *bolt.Tx) error {
		// Retrieve buckets.
		eb := tx.Bucket([]byte("MOD_EXIM"))

		// Write key/value pair.
		if err := eb.Put(binId, eximJs); err != nil {
			return err
		}

		return nil
	})
}

func (e *Exims) getEximsTx() error {
	return db.View(func(tx *bolt.Tx) error {
		// Retrieve bucket.
		eb := tx.Bucket([]byte("MOD_EXIM"))

		// Iterate over exims.
		return eb.ForEach(func(k, v []byte) error {
			// Unmarshal value to Exim.
			var exim Exim
			err := json.Unmarshal(v, &exim)
			if err != nil {
				return err
			}

			// Append to slice.
			*e = append(*e, exim)

			return nil
		})
	})
}

func (e *Exim) getEximDetailsTx(eximBinId []byte) error {
	return db.View(func(tx *bolt.Tx) error {
		// Retrieve bucket.
		eb := tx.Bucket([]byte("MOD_EXIM"))

		// Retrieve exim.
		eximBytes := eb.Get(eximBinId)
		if eximBytes == nil {
			return fmt.Errorf("exim does not exist")
		}

		// Unmarshal value to receiver.
		err := json.Unmarshal(eximBytes, e)
		if err != nil {
			return err
		}

		return nil
	})
}
