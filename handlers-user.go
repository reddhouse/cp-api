package main

import (
	"encoding/json"
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
		if err := b.Put(key, []byte(ubs.Email)); err != nil {
			return err
		}

		// Use userAfterSignup instance from this point forward.
		uas = userAfterSignup{
			UserId: id,
			Email:  ubs.Email,
		}

		return nil
	})

	if err != nil {
		log.Fatalf("[error-api] putting new user in db: %v", err)
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
