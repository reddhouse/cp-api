package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type authGroup struct {
	LoginCode     int       `json:"loginCode,omitempty"`
	LoginAttempts int       `json:"loginAttempts"`
	LogoutTs      time.Time `json:"logoutTs"`
}

type user struct {
	UserId  ulid.ULID
	Email   string
	AuthGrp authGroup
}

const maxLoginCodeAttempts = 3

// Check authorization header for "Bearer " prefix and valid token.
func authMiddleware(next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	// Return a closure that captures and calls the "next" handler in the call chain.
	return func(w http.ResponseWriter, r *http.Request) {
		var userInst user
		var authHeader = r.Header.Get("Authorization")

		// Check if the Authorization header starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			err := errors.New("invalid Authorization header")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Strip "Bearer " from the beginning of the token
		trimmedHeader := strings.TrimPrefix(authHeader, "Bearer ")

		var parts = strings.Split(trimmedHeader, ".")
		if len(parts) != 2 {
			err := errors.New("authorization header should consist of two parts")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var reqUserId = parts[0]
		var reqSessionInfo = parts[1]

		// Decode & unmarshal ulid from string into userInst.UserId.
		if err := unmarshalUlid(w, &userInst.UserId, reqUserId); err != nil {
			return
		}

		// Read authGroup.
		err := db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("USER_AUTH"))
			// Convert ulid to byte slice to use as db key.
			binId, err := userInst.UserId.MarshalBinary()
			if err != nil {
				return err
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
			return nil
		})

		// Handle database error.
		if err != nil {
			log.Printf("[error-api] retrieving authGrp from db: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		currentSessionInfo := fmt.Sprintf("%s.%s", strconv.Itoa(userInst.AuthGrp.LoginCode), userInst.AuthGrp.LogoutTs)

		// Verify signature of session info.
		if !verifySignature(currentSessionInfo, reqSessionInfo) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Call the next handler in the chain.
		next(w, r)
	}
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
			if string(k) == userInst.Email {
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
			return err
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
	sessionInfo := fmt.Sprintf("%s.%s", strconv.Itoa(userInst.AuthGrp.LoginCode), userInst.AuthGrp.LogoutTs)
	signedSessionInfo := signMessage(sessionInfo)
	responseBodyInst.Token = fmt.Sprintf("%s.%s", userInst.UserId, signedSessionInfo)

	encodeJsonAndRespond(w, responseBodyInst)
}

func handleLogout(w http.ResponseWriter, req *http.Request) {
	type requestBody struct {
		UserId string `json:"userId"`
	}
	var userInst user
	var requestBodyInst requestBody

	log.Printf("Handling POST to %s\n", req.URL.Path)

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

	// Write loginCode and logoutTs.
	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("USER_AUTH"))
		// Convert ulid to byte slice to use as db key.
		binId, err := userInst.UserId.MarshalBinary()
		if err != nil {
			return err
		}
		// Explicitly set LoginAttempts to zero, even though they were reset
		// during login-code validation, and even though the marshaling process
		// would default to zero value anyway.
		userInst.AuthGrp.LoginAttempts = 0
		// Generate new login code and set logoutTs to now.
		userInst.AuthGrp.LoginCode = generateLoginCode()
		userInst.AuthGrp.LogoutTs = time.Now()
		// Marshal authGroup to be stored.
		agJs, err := json.Marshal(userInst.AuthGrp)
		if err != nil {
			return err
		}
		// Write authGroup back to db.
		if err := b.Put(binId, agJs); err != nil {
			return err
		}
		return nil
	})

	// Handle database error.
	if err != nil {
		log.Printf("[error-api] updating db in logout transaction: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Success. Respond with 204 No Content.
	w.WriteHeader(http.StatusNoContent)
}
