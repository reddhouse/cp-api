package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type authGroup struct {
	LoginCode     int       `json:"loginCode"`
	LoginAttempts int       `json:"loginAttempts"`
	SignoutTs     time.Time `json:"signoutTs"`
}

type user struct {
	UserId  ulid.ULID
	Email   string
	AuthGrp authGroup
}

func handleSignup(w http.ResponseWriter, req *http.Request) {
	type requestBody struct {
		Email string `json:"email"`
	}
	type responseBody struct {
		UserId string `json:"userId"`
	}
	var userInst user
	var requestBodyInst requestBody

	log.Printf("Handling POST to %s\n", req.URL.Path)

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode JSON request body (stream) into responseBody struct.
	if err := decodeJsonIntoStruct(w, req, &requestBodyInst); err != nil {
		return
	}

	// Create ULID and db key(s).
	id, binaryId := createUlid()

	// Update user instance.
	userInst.UserId = id
	userInst.Email = requestBodyInst.Email
	userInst.AuthGrp.LoginCode = generateLoginCode()
	userInst.AuthGrp.LoginAttempts = 0

	// Marshal authGroup to be stored.
	agJs, err := json.Marshal(userInst.AuthGrp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Marshal response struct for response payload.
	resJs, err := json.Marshal(responseBody{UserId: userInst.UserId.String()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create user in database.
	err = db.Update(func(tx *bolt.Tx) error {
		// Retrieve buckets.
		eb := tx.Bucket([]byte("USER_EMAIL"))
		ab := tx.Bucket([]byte("USER_AUTH"))

		// Check if email already exists.
		err := eb.ForEach(func(k, v []byte) error {
			if string(v) == userInst.Email {
				return errors.New("email already exists")
			}
			return nil
		})

		// Abort update if email already exists.
		if err != nil {
			return err
		}

		// Write key/value pairs.
		if err := eb.Put(binaryId, []byte(userInst.Email)); err != nil {
			return err
		}
		if err := ab.Put(binaryId, agJs); err != nil {
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
		err = sendEmail(userInst.Email, "Login code for Cooperative Party!", fmt.Sprintf("Thanks for signing up! You may now login using the following code: %v", userInst.AuthGrp.LoginCode))
		if err != nil {
			log.Printf("[error-api] sending email to user: %v", err)
		}
	} else {
		// Todo: Store code in admin struct in database.
	}
}

// func handleLoginCode(w http.ResponseWriter, req *http.Request) {
// 	type requestBody struct {
// 		Email string `json:"email"`
// 	}
// 	type responseBody struct {
// 		Token string `json:"token"`
// 	}
// 	var userInst user
// 	var requestBodyInst requestBody

// 	log.Printf("Handling GET to %s\n", req.URL.Path)

// 	// Enforce JSON Content-Type.
// 	if err := verifyContentType(w, req); err != nil {
// 		return
// 	}
// 	// Decode JSON request body (stream) into responseBody struct.
// 	if err := decodeJsonIntoStruct(w, req, &requestBodyInst); err != nil {
// 		return
// 	}

// 	// Lookup userId by email, and retrieve corresponding authGroup.

// }
