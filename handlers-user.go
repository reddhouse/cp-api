package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type authGroup struct {
	LoginCode     int
	LoginAttempts int
	// SignoutTs time.Time
}

type user struct {
	UserId  ulid.ULID `json:"userId"`
	Email   string    `json:"email"`
	AuthGrp authGroup `json:"authGrp"`
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

	// Create ULID and db key(s).
	id, bid := createUlid()
	keyEmail := createCompositeKey(bid, "email_addr")
	keyAuth := createCompositeKey(bid, "auth_grp")

	// Update user instance.
	u.UserId = id
	u.AuthGrp.LoginCode = generateLoginCode()
	u.AuthGrp.LoginAttempts = 0

	// Marshal authGroup to be stored.
	agJs, err := json.Marshal(u.AuthGrp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Marshal response struct for response payload.
	resJs, err := json.Marshal(response{UserId: u.UserId.String()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create user in database.
	err = db.Update(func(tx *bolt.Tx) error {
		// Retrieve the USER bucket.
		b := tx.Bucket([]byte("USER"))

		// Check if email already exists.
		err := b.ForEach(func(k, v []byte) error {
			if string(v) == u.Email {
				return errors.New("email already exists")
			}
			return nil
		})

		// Abort update if email already exists.
		if err != nil {
			return err
		}

		// Write key/value pairs.
		if err := b.Put(keyEmail, []byte(u.Email)); err != nil {
			return err
		}
		if err := b.Put(keyAuth, agJs); err != nil {
			return err
		}

		return nil
	})

	// Send response, error case.
	if err != nil {
		log.Printf("[error-api] updating db with new user: %v", err)
		const statusUnprocessableEntity = 422
		http.Error(w, err.Error(), statusUnprocessableEntity)
		return
	}

	// Send http response, success case.
	w.Header().Set("Content-Type", "application/json")
	w.Write(resJs)

	if env != nil && *env == "prod" {
		// Send email to user.
		err = sendEmail(u.Email, "Login code for Cooperative Party!", fmt.Sprintf("Thanks for signing up! You may now login using the following code: %v", u.AuthGrp.LoginCode))
		if err != nil {
			log.Printf("[error-api] sending email to user: %v", err)
		}
	} else {
		// Todo: Store code in admin struct in database.
	}
}
