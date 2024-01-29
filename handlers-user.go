package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type user struct {
	UserId ulid.ULID `json:"userId"`
	Email  string    `json:"email"`
}

func handleSignup(w http.ResponseWriter, req *http.Request) {
	log.Printf("Handling POST to %s\n", req.URL.Path)
	var u user
	type response struct {
		UserId string `json:"userId"`
	}
	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode JSON request body (stream) into user struct.
	if err := decodeJsonIntoStruct(w, req, &u); err != nil {
		return
	}
	// Create user in database.
	err := db.Update(func(tx *bolt.Tx) error {
		// Retrieve the USER bucket.
		b := tx.Bucket([]byte("USER"))

		// Create a ULID.
		id, bid := createUlid()

		// Write key/value pairs.
		key := createCompositeKey(bid, "user_id")
		if err := b.Put(key, bid); err != nil {
			return err
		}
		key = createCompositeKey(bid, "email_addr")
		if err := b.Put(key, []byte(u.Email)); err != nil {
			return err
		}

		// Update the user struct with the new ID.
		u.UserId = id

		return nil
	})

	if err != nil {
		log.Fatalf("error putting new user in db: %v", err)
	}

	// Marshal response struct into JSON response payload.
	js, err := json.Marshal(response{UserId: u.UserId.String()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func handleGetAllUsers(w http.ResponseWriter, req *http.Request) {
	log.Printf("Handling GET to %s\n", req.URL.Path)
	var users []user
	db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys.
		b := tx.Bucket([]byte("USER"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var u user
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
