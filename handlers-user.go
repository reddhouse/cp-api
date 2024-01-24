package main

import (
	"encoding/json"
	"log"
	"net/http"

	bolt "go.etcd.io/bbolt"
)

type user struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
}

func handleSignup(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling POST to %s\n", req.URL.Path)
	var u user
	type response struct {
		UserId int `json:"id"`
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
	var id uint64
	err := db.Update(func(tx *bolt.Tx) error {
		// Retrieve the USER bucket.
		b := tx.Bucket([]byte("USER"))
		// Get auto-incrementing ID for new user.
		// Tx will not be closed nor un-writeable in Update() so ignore error.
		id, _ = b.NextSequence()
		u.Id = int(id)
		// Marshal user struct into JSON (byte slice).
		buf, err := json.Marshal(u)
		if err != nil {
			return err
		}
		// Persist bytes to USER bucket.
		return b.Put(itob(int(id)), buf)
	})
	if err != nil {
		log.Fatalf("failed to put new user in db: %v", err)
	}

	// Marshal response struct into JSON response payload.
	js, err := json.Marshal(response{UserId: u.Id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func handleGetAllUsers(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling GET to %s\n", req.URL.Path)
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
