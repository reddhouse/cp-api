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
	LoginCode     int       `json:"loginCode,omitempty"`
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
	var responseBodyInst responseBody

	log.Printf("Handling POST to %s\n", req.URL.Path)

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into requestBody struct.
	if err := unmarshalJson(w, req, &requestBodyInst); err != nil {
		return
	}

	// Create ULID and db key(s).
	id, binId := createUlid()

	// Update instance fields.
	userInst.UserId = id
	userInst.Email = requestBodyInst.Email
	userInst.AuthGrp.LoginCode = generateLoginCode()
	userInst.AuthGrp.LoginAttempts = 0
	
	responseBodyInst.UserId = id.String()

	// Marshal authGroup to be stored.
	agJs, err := json.Marshal(userInst.AuthGrp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write email and authGroup to database.
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
		if err := eb.Put([]byte(userInst.Email), binId); err != nil {
			return err
		}
		if err := ab.Put(binId, agJs); err != nil {
			return err
		}

		return nil
	})

	// Handle database error.
	if err != nil {
		log.Printf("[error-api] updating db with new user: %v", err)
		const statusUnprocessableEntity = 422
		http.Error(w, err.Error(), statusUnprocessableEntity)
		return
	}

	// Success. Reply with userId.
	encodeJsonAndRespond(w, responseBodyInst)

	// Send email to user in production environment.
	if env != nil && *env == "prod" {
		err = sendEmail(userInst.Email, "Login code for Cooperative Party!", fmt.Sprintf("Thanks for signing up! You may now login using the following code: %v", userInst.AuthGrp.LoginCode))
		if err != nil {
			log.Printf("[error-api] sending email to user: %v", err)
		}
	} else {
		// Todo: Store code in admin struct in database to facilitate testing.
	}
}

func handleLogin(w http.ResponseWriter, req *http.Request) {
	type requestBody struct {
		Email string `json:"email"`
	}
	type responseBody struct {
		UserId string `json:"userId"`
	}
	var userInst user
	var requestBodyInst requestBody
	var responseBodyInst responseBody

	log.Printf("Handling POST to %s\n", req.URL.Path)

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into requestBody struct.
	if err := unmarshalJson(w, req, &requestBodyInst); err != nil {
		return
	}

	userInst.Email = requestBodyInst.Email

	// Read userId and authGrp.
	err := db.View(func(tx *bolt.Tx) error {
		eb := tx.Bucket([]byte("USER_EMAIL"))
		ab := tx.Bucket([]byte("USER_AUTH"))
		// Retrieve userId from email.
		binId := eb.Get([]byte(requestBodyInst.Email))
		if binId == nil {
			return errors.New("provided email is not on file")
		}
		// Unmarshal userId into userInst.
		err := userInst.UserId.UnmarshalBinary(binId)
		if err != nil {
			return err
		}
		// Retrieve authGroup.
		authGrp := ab.Get(binId)
		if authGrp == nil {
			return errors.New("authGroup does not exist for the userId with corresponds with the provided email")
		}
		// Unmarshal authGrp into userInst.
		err = json.Unmarshal(authGrp, &userInst.AuthGrp)
		if err != nil {
			return err
		}
		return nil
	})

	// Handle database error.
	if err != nil {
		log.Printf("[error-api] querying db for user email: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseBodyInst.UserId = userInst.UserId.String()

	// Success. Reply with userId.
	encodeJsonAndRespond(w, responseBodyInst)

	// Send email to user in production environment.
	if env != nil && *env == "prod" {
		err = sendEmail(userInst.Email, "Login code for Cooperative Party!", fmt.Sprintf("It looks like you're attempting to login to Cooperative Party. Please proceed by entering the following code: %v", userInst.AuthGrp.LoginCode))
		if err != nil {
			log.Printf("[error-api] sending email to user: %v", err)
		}
	} else {
		// Todo: Store code in admin struct in database to facilitate testing.
	}
}

func handleLoginCode(w http.ResponseWriter, req *http.Request) {
	type requestBody struct {
		UserId string `json:"userId,omitempty"`
		Code   int    `json:"code,omitempty"`
	}
	type responseBody struct {
		Token             string `json:"token,omitempty"`
		RemainingAttempts int    `json:"remainingAttempts"`
	}
	var userInst user
	var requestBodyInst requestBody
	var responseBodyInst responseBody

	log.Printf("Handling GET to %s\n", req.URL.Path)

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into requestBody struct.
	if err := unmarshalJson(w, req, &requestBodyInst); err != nil {
		return
	}

	// Decode & unmarshal ulid from string into userInst.UserId.
	if err := unmarshalUlid(w, &userInst.UserId, requestBodyInst.UserId); err != nil {
		return
	}

	// Read authGroup. Write authGroup.loginAttempts.
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("USER_AUTH"))

		// Convert ulid to byte slice to use as db key.
		binId, err := userInst.UserId.MarshalBinary()
		if err != nil {
			log.Fatalf("[error-api] marshaling ULID: %v", err)
		}
		// Retrieve authGroup.
		authGrp := b.Get(binId)
		if authGrp == nil {
			return errors.New("authGroup does not exist for specified userId")
		}
		// Unmarshal authGrp into userInst.
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
		// Marshal authGroup to be stored.
		agJs, err := json.Marshal(userInst.AuthGrp)
		if err != nil {
			return err
		}
		// Write loginAttempts back to db.
		if err := b.Put(binId, agJs); err != nil {
			return err
		}

		return nil
	})

	// Handle database error.
	if err != nil {
		log.Printf("[error-api] updating db in login-code transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseBodyInst.RemainingAttempts = maxLoginCodeAttempts - userInst.AuthGrp.LoginAttempts

	// Handle incorrect login code.
	if userInst.AuthGrp.LoginCode != requestBodyInst.Code {
		encodeJsonAndRespond(w, responseBodyInst)
		return
	}

	// Success. Create and reply with token.
	rawToken := fmt.Sprintf("cooperative-party.%s.%s", userInst.AuthGrp.LoginCode, userInst.AuthGrp.SignoutTs)
	signedToken := signMessage(rawToken)

	responseBodyInst.Token = signedToken

	encodeJsonAndRespond(w, responseBodyInst)
}
