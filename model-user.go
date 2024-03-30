package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type AuthGrp struct {
	LoginCode     int       `json:"loginCode"`
	LoginAttempts int       `json:"loginAttempts"`
	LogoutTs      time.Time `json:"logoutTs"`
}

type User struct {
	UserId  ulid.ULID
	Email   string
	AuthGrp AuthGrp
}

// Reads authGrp from db and sets corresponding value on receiver.
func (u *User) authMiddlewareTx(binId []byte) error {
	return db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("USER_AUTH"))

		// Retrieve authGrp.
		authGrp := b.Get(binId)
		if authGrp == nil {
			return fmt.Errorf("authGrp does not exist for specified userId")
		}
		// Unmarshal authGrp into u.
		err := json.Unmarshal(authGrp, &u.AuthGrp)
		if err != nil {
			return err
		}

		return nil
	})
}

// Writes email and authGrp to database.
func (u *User) signupTx(binId []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		// Retrieve buckets.
		eb := tx.Bucket([]byte("USER_EMAIL"))
		ab := tx.Bucket([]byte("USER_AUTH"))

		// Check if email already exists.
		err := eb.ForEach(func(k, v []byte) error {
			if string(k) == u.Email {
				return fmt.Errorf("email already exists (%s)", u.Email)
			}
			return nil
		})

		// Abort update if email already exists.
		if err != nil {
			return err
		}

		// Marshal authGrp to be stored.
		agJs, err := json.Marshal(u.AuthGrp)
		if err != nil {
			return err
		}

		// Write key/value pairs.
		if err := eb.Put([]byte(u.Email), binId); err != nil {
			return err
		}
		if err := ab.Put(binId, agJs); err != nil {
			return err
		}

		return nil
	})
}

// Reads userId and authGrp from db and sets AuthGrp value on receiver.
func (u *User) loginTx() error {
	return db.View(func(tx *bolt.Tx) error {
		eb := tx.Bucket([]byte("USER_EMAIL"))
		ab := tx.Bucket([]byte("USER_AUTH"))
		// Retrieve userId from db with email lookup.
		binId := eb.Get([]byte(u.Email))
		if binId == nil {
			return fmt.Errorf("provided email is not on file (%s)", u.Email)
		}
		// Unmarshal userId into user.
		err := u.UserId.UnmarshalBinary(binId)
		if err != nil {
			return err
		}
		// Retrieve authGrp.
		authGrp := ab.Get(binId)
		if authGrp == nil {
			return fmt.Errorf("authGrp does not exist for the userId with corresponds with the provided email")
		}
		// Unmarshal authGrp into user.
		err = json.Unmarshal(authGrp, &u.AuthGrp)
		if err != nil {
			return err
		}
		return nil
	})
}

// Reads authGrp from db and sets corresponding value on receiver.
// Calculates new value of loginAttempts and writes it to db.
func (u *User) loginCodeTx(binId []byte, code int) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("USER_AUTH"))

		// Retrieve authGrp.
		authGrp := b.Get(binId)
		if authGrp == nil {
			return fmt.Errorf("authGrp does not exist for specified userId")
		}
		// Unmarshal authGrp into u.
		err := json.Unmarshal(authGrp, &u.AuthGrp)
		if err != nil {
			return err
		}
		// If loginAttempts have been exceeded, return error.
		if u.AuthGrp.LoginAttempts >= maxLoginCodeAttempts {
			return fmt.Errorf("login attempts exceeded")
		}
		// Check loginCode and adust loginAttempts as necessary.
		if u.AuthGrp.LoginCode == code {
			u.AuthGrp.LoginAttempts = 0
		} else {
			u.AuthGrp.LoginAttempts++
		}
		// Marshal authGrp to be stored.
		agJs, err := json.Marshal(u.AuthGrp)
		if err != nil {
			return err
		}
		// Write loginAttempts back to db.
		if err := b.Put(binId, agJs); err != nil {
			return err
		}

		return nil
	})
}

// Uses hard-coded maxLoginCodeAttempts to calculate remaining attempts.
func (u *User) calculateRemainingAttempts() int {
	return maxLoginCodeAttempts - u.AuthGrp.LoginAttempts
}

// Writes LoginCode and LogoutTs to db.
func (u *User) logoutTx(binId []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("USER_AUTH"))

		// Marshal authGrp to be stored.
		agJs, err := json.Marshal(u.AuthGrp)
		if err != nil {
			return err
		}

		// Write authGrp to db.
		if err := b.Put(binId, agJs); err != nil {
			return err
		}
		return nil
	})
}
