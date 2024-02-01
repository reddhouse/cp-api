package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type userBeforeSignup struct {
	Email string `json:"email"`
}

type userAfterSignup struct {
	UserId ulid.ULID `json:"userId"`
	Email  string    `json:"email"`
}

func handleSignup(w http.ResponseWriter, req *http.Request) {
	log.Printf("Handling POST to %s\n", req.URL.Path)
	var ubs userBeforeSignup
	var uas userAfterSignup
	type response struct {
		UserId string `json:"userId"`
	}
	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode JSON request body (stream) into user struct.
	if err := decodeJsonIntoStruct(w, req, &ubs); err != nil {
		return
	}

	// Create ULID and key(s) to be stored.
	id, bid := createUlid()
	keyEmail := createCompositeKey(bid, "email_addr")

	// Create a login code.
	// loginCode := generateLoginCode()

	// Create user in database.
	err := db.Update(func(tx *bolt.Tx) error {
		// Retrieve the USER bucket.
		b := tx.Bucket([]byte("USER"))

		// Check if email already exists.
		err := b.ForEach(func(k, v []byte) error {
			if string(v) == ubs.Email {
				return errors.New("email already exists")
			}
			return nil
		})

		if err != nil {
			return err
		}

		// Write key/value pairs.
		if err := b.Put(keyEmail, []byte(ubs.Email)); err != nil {
			return err
		}

		return nil
	})

	// Send error response if provided email is not unique, and do not proceed.
	if err != nil {
		log.Printf("[error-api] putting new user in db: %v", err)
		const statusUnprocessableEntity = 422
		http.Error(w, err.Error(), statusUnprocessableEntity)
		return
	}

	// Use userAfterSignup instance from this point forward.
	uas = userAfterSignup{
		UserId: id,
		Email:  ubs.Email,
	}

	// Marshal response struct into JSON response payload.
	js, err := json.Marshal(response{UserId: uas.UserId.String()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)

	if env != nil && *env == "prod" {
		// Send email to user.
		err = sendEmail(uas.Email, "Welcome to the Cooperative Party!", "Thank you for signing up!")
		if err != nil {
			log.Printf("[error-api] sending email to user: %v", err)
		}
	} else {
		// Todo: Store code in admin struct in database.
	}
}

func handleGetAllUsers(w http.ResponseWriter, req *http.Request) {
	log.Printf("Handling GET to %s\n", req.URL.Path)
	var users []userAfterSignup
	db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys.
		b := tx.Bucket([]byte("USER"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var u userAfterSignup
			err := json.Unmarshal(v, &u)
			if err != nil {
				return err
			}
			users = append(users, u)
		}

		return nil
	})

	js, err := json.Marshal(users)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
