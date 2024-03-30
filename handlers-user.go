package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Go prefers that the key used in context.WithValue be of a custom type.
type contextKeyType string

const userIdContextKey = contextKeyType("userId")
const maxLoginCodeAttempts = 3

// Checks authorization header for "Bearer " prefix and valid token.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	// Return a closure that captures and calls the "next" handler in the call chain.
	return func(w http.ResponseWriter, req *http.Request) {
		var user *User = new(User)
		var authHeader = req.Header.Get("Authorization")

		// Check if the Authorization header starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			err := fmt.Errorf("invalid Authorization header")
			sendErrorResponse(w, err, http.StatusBadRequest)
			return
		}

		// Strip "Bearer " from the beginning of the token
		trimmedHeader := strings.TrimPrefix(authHeader, "Bearer ")

		var parts = strings.Split(trimmedHeader, ".")
		if len(parts) != 2 {
			err := fmt.Errorf("authorization header should consist of two parts")
			sendErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		var reqUserId = parts[0]
		var reqSessionInfo = parts[1]

		// Decode & unmarshal ulid from string into user.UserId.
		if err := unmarshalUlid(w, &user.UserId, reqUserId); err != nil {
			return
		}

		// Convert ulid to byte slice to use as db key.
		binId, err := getBinId(w, user.UserId)
		if err != nil {
			return
		}

		// Execute db transaction.
		err = user.authMiddlewareTx(binId)
		if err != nil {
			fmt.Printf("[err][api] retrieving authGrp from db: %v [%s]\n", err, cts())
			sendErrorResponse(w, err, http.StatusInternalServerError)
			return
		}

		// Put login code + current session info into the same format as the
		// the signed part of an Auth token.
		currentSessionInfo := fmt.Sprintf("%s.%s", strconv.Itoa(user.AuthGrp.LoginCode), user.AuthGrp.LogoutTs)

		// Verify signature of the signed part of an Auth token.
		if !verifySignature(currentSessionInfo, reqSessionInfo) {
			sendErrorResponse(w, fmt.Errorf("Unauthorized"), http.StatusUnauthorized)
			return
		}

		// Add userId to the request context.
		ctx := context.WithValue(req.Context(), userIdContextKey, user.UserId)

		// Call the next handler in the chain with context.
		next.ServeHTTP(w, req.WithContext(ctx))
	}
}

func handleSignup(w http.ResponseWriter, req *http.Request) {
	type ReqBody struct {
		Email string `json:"email"`
	}
	type ResBody struct {
		UserId string `json:"userId"`
	}
	var reqBody ReqBody
	var user *User = new(User)
	var resBody ResBody

	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into ReqBody struct.
	if err := unmarshalJson(w, &reqBody, req); err != nil {
		return
	}

	// Create ULID.
	id, binId := createUlid()

	resBody.UserId = id.String()

	user.UserId = id
	user.Email = reqBody.Email
	user.AuthGrp.LoginCode = generateLoginCode()
	user.AuthGrp.LoginAttempts = 0
	// Note, AuthGrp.LogoutTs will default to zero value.

	// Execute db transaction.
	err := user.signupTx(binId)
	if err != nil {
		fmt.Printf("[err][api] updating db with new user: %v [%s]\n", err, cts())
		const statusUnprocessableEntity = 422
		sendErrorResponse(w, err, statusUnprocessableEntity)
		return
	}

	// Success. Reply with userId.
	encodeJsonAndRespond(w, resBody)

	// Send email to user in production environment.
	if env != nil && *env == "prod" {
		err = sendEmail(user.Email, "Login code for Cooperative Party!", fmt.Sprintf("Thanks for signing up! You may now login using the following code: %v", user.AuthGrp.LoginCode))
		if err != nil {
			fmt.Printf("[err][api] sending email to user: %v [%s]\n", err, cts())
		}
	}
}

func handleLogin(w http.ResponseWriter, req *http.Request) {
	type ReqBody struct {
		Email string `json:"email"`
	}
	type ResBody struct {
		UserId string `json:"userId"`
	}
	var reqBody ReqBody
	var user *User = new(User)
	var resBody ResBody

	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into ReqBody struct.
	if err := unmarshalJson(w, &reqBody, req); err != nil {
		return
	}

	user.Email = reqBody.Email

	// Execute db transaction.
	err := user.loginTx()
	if err != nil {
		fmt.Printf("[err][api] querying db for user email: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	resBody.UserId = user.UserId.String()

	// Success. Reply with userId.
	encodeJsonAndRespond(w, resBody)

	// Send email to user in production environment.
	if env != nil && *env == "prod" {
		err = sendEmail(user.Email, "Login code for Cooperative Party!", fmt.Sprintf("It looks like you're attempting to login to Cooperative Party. Please proceed by entering the following code: %v", user.AuthGrp.LoginCode))
		if err != nil {
			fmt.Printf("[err][api] sending email to user: %v [%s]\n", err, cts())
		}
	}
}

func handleLoginCode(w http.ResponseWriter, req *http.Request) {
	type ReqBody struct {
		UserId string `json:"userId"`
		Code   int    `json:"code"`
	}
	type ResBody struct {
		Token             string `json:"token"`
		RemainingAttempts int    `json:"remainingAttempts"`
	}
	var reqBody ReqBody
	var user *User = new(User)
	var resBody ResBody

	fmt.Printf("[api] handling GET to %s [%s]\n", req.URL.Path, cts())

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into ReqBody struct.
	if err := unmarshalJson(w, &reqBody, req); err != nil {
		return
	}
	// Decode & unmarshal ulid from string into user.UserId.
	if err := unmarshalUlid(w, &user.UserId, reqBody.UserId); err != nil {
		return
	}

	// Convert ulid to byte slice to use as db key.
	binId, err := getBinId(w, user.UserId)
	if err != nil {
		return
	}

	// Execute db transaction.
	err = user.loginCodeTx(binId, reqBody.Code)
	if err != nil {
		fmt.Printf("[err][api] updating db in login-code transaction: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	resBody.RemainingAttempts = user.calculateRemainingAttempts()

	// Handle incorrect login code.
	if user.AuthGrp.LoginCode != reqBody.Code {
		encodeJsonAndRespond(w, resBody)
		return
	}

	// Success. Create and reply with token.
	sessionInfo := fmt.Sprintf("%s.%s", strconv.Itoa(user.AuthGrp.LoginCode), user.AuthGrp.LogoutTs)
	signedSessionInfo := signMessage(sessionInfo)
	resBody.Token = fmt.Sprintf("%s.%s", user.UserId, signedSessionInfo)

	encodeJsonAndRespond(w, resBody)
}

func handleLogout(w http.ResponseWriter, req *http.Request) {
	type ReqBody struct {
		UserId string `json:"userId"`
	}
	var reqBody ReqBody
	var user *User = new(User)

	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into ReqBody struct.
	if err := unmarshalJson(w, &reqBody, req); err != nil {
		return
	}
	// Decode & unmarshal ulid from string into user.UserId.
	if err := unmarshalUlid(w, &user.UserId, reqBody.UserId); err != nil {
		return
	}

	// Convert ulid to byte slice to use as db key.
	binId, err := getBinId(w, user.UserId)
	if err != nil {
		return
	}

	// Explicitly set LoginAttempts to zero, even though they were reset
	// during login-code validation, and even though the marshaling process
	// would default to zero value anyway.
	user.AuthGrp.LoginAttempts = 0
	// Generate new login code and set logoutTs to now.
	user.AuthGrp.LoginCode = generateLoginCode()
	user.AuthGrp.LogoutTs = time.Now()

	// Execute db transaction.
	err = user.logoutTx(binId)
	if err != nil {
		fmt.Printf("[err][api] updating db in logout transaction: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	// Success. Respond with 204 No Content.
	w.WriteHeader(http.StatusNoContent)
}
