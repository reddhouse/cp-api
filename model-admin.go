package main

import (
	"fmt"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type Admin struct {
	AdminId ulid.ULID
}

// Checks if AdminId exists in ADMIN_EMAIL bucket.
func (a *Admin) adminMiddlewareTx() error {
	return db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("ADMIN_EMAIL"))
		// Convert ulid to byte slice to use as db key.
		binId, err := a.AdminId.MarshalBinary()
		if err != nil {
			return err
		}
		// Check if AdminId exists as key in ADMIN_EMAIL bucket.
		emailAddr := b.Get(binId)
		if emailAddr == nil {
			return fmt.Errorf("administrator does not exist for specified adminId")
		}
		return nil
	})
}
