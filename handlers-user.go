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

const maxLoginCodeAttempts = 3

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
	// Decode JSON request body (stream) into requestBody struct.
	if err := decodeJsonIntoDst(w, req, &requestBodyInst); err != nil {
		return
	}

	// Create ULID and db key(s).
	id, binId := createUlid()

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
		if err := eb.Put(binId, []byte(userInst.Email)); err != nil {
			return err
		}
		if err := ab.Put(binId, agJs); err != nil {
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

func handleLoginCode(w http.ResponseWriter, req *http.Request) {
	type requestBody struct {
		UserId string `json:"userId"`
		Code   int    `json:"code"`
	}
	type responseBody struct {
		Token             string `json:"token"`
		RemainingAttempts int    `json:"remainingAttempts"`
	}
	var userInst user
	var requestBodyInst requestBody

	log.Printf("Handling GET to %s\n", req.URL.Path)

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode JSON request body (stream) into requestBody struct.
	if err := decodeJsonIntoDst(w, req, &requestBodyInst); err != nil {
		return
	}

	// Decode userId from string into userInst.UserId (type ulid.ULID).
	if err := decodeUlidIntoDst(w, req, &userInst.UserId, requestBodyInst.UserId); err != nil {
		return
	}

	// Lookup userId by email, and retrieve corresponding authGroup.
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("USER_AUTH"))

		binId, err := userInst.UserId.MarshalBinary()
		if err != nil {
			log.Fatalf("[error-api] marshaling ULID: %v", err)
		}

		// Retrieve authGroup. Unmarshal into userInst.
		authGrp := b.Get(binId)
		if authGrp == nil {
			return errors.New("authGroup does not exist for specified userId")
		}

		err = json.Unmarshal(authGrp, &userInst.AuthGrp)
		if err != nil {
			return err
		}

		// If loginAttempts have been exceeded, return error.
		if userInst.AuthGrp.LoginAttempts >= maxLoginCodeAttempts {
			return errors.New("login attempts exceeded")
		}

		// Check loginCode and adust loginAttempts as necessary.
		if userInst.AuthGrp.LoginCode == requestBodyInst.Code {
			userInst.AuthGrp.LoginAttempts = 0
		} else {
			userInst.AuthGrp.LoginAttempts++
		}

		// Write loginAttempts back to db.
		agJs, err := json.Marshal(userInst.AuthGrp)
		if err != nil {
			return err
		}
		if err := b.Put(binId, agJs); err != nil {
			return err
		}

		return nil
	})

	// Send response, first error case.
	if err != nil {
		log.Printf("[error-api] updating db in login-code transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send response, second error case.
	if userInst.AuthGrp.LoginCode != requestBodyInst.Code {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Generate token.
	rawToken := fmt.Sprintf("cooperative-party.%s.%s", userInst.AuthGrp.LoginCode, userInst.AuthGrp.SignoutTs)
	signedToken := signMessage(rawToken)

	// Marshal response struct for response payload.
	resJs, err := json.Marshal(responseBody{Token: signedToken, RemainingAttempts: maxLoginCodeAttempts - userInst.AuthGrp.LoginAttempts})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send http response, success case.
	w.Header().Set("Content-Type", "application/json")
	w.Write(resJs)
}
