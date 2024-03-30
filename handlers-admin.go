package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

// Checks custom admin-auth header for valid token, and call next handler in chain.
func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	// Return a closure that captures and calls the "next" handler in the call chain.
	return func(w http.ResponseWriter, r *http.Request) {
		var admin *Admin = new(Admin)
		var authHeader = r.Header.Get("Admin-Authorization")
		// An auth header has no prefix, and is made up of an ULID and a
		// key-signed signature of that same ULID, separated by a period.
		var parts = strings.Split(authHeader, ".")
		if len(parts) != 2 {
			err := fmt.Errorf("authorization header should consist of two parts")
			sendErrorResponse(w, err, http.StatusBadRequest)
			return
		}
		var reqAdminId = parts[0]
		var reqSignature = parts[1]

		// Decode & unmarshal ulid from string into adminId.
		if err := unmarshalUlid(w, &admin.AdminId, reqAdminId); err != nil {
			return
		}

		// Execute db transaction.
		err := admin.adminMiddlewareTx()
		if err != nil {
			fmt.Printf("[err][api] getting admin info from db: %v [%s]\n", err, cts())
			sendErrorResponse(w, err, http.StatusInternalServerError)
			return
		}

		// Verify signature of the signed part of the Admin-Authorization token.
		if !verifySignature(reqAdminId, reqSignature) {
			sendErrorResponse(w, fmt.Errorf("Unauthorized"), http.StatusUnauthorized)
			return
		}

		// Call the next handler in the chain.
		next(w, r)
	}
}

func handleShutdownServer(w http.ResponseWriter, req *http.Request, server *http.Server) {
	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())
	fmt.Printf("[api] shutting down server... [%s]\n", cts())
	// Use a separate goroutine to allow HTTP handler to finish and send its
	// response back to the client in it the main goroutine.
	go func() {
		// Create a context with a timeout of 5 seconds.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Call server.Shutdown with the context, to stop the server from accepting
		// new requests, waiting up to 5 seconds for all currently processing
		// requests to finish.
		if err := server.Shutdown(ctx); err != nil {
			fmt.Printf("[err][api] shutting down server: %v [%s]\n", err, cts())
			os.Exit(1)
		}
	}()
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("bye!"))
}

func handleLogBucketUlidKey(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())
	bucket := req.PathValue("bucket")
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys.
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var id ulid.ULID
			if err := id.UnmarshalBinary(k); err != nil {
				return err
			}
			fmt.Printf("%s | %s\n", id, v)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("[err][api] getting ulid-k/v pairs from db: %v [%s]\n", err, cts())
		// Send a 500 Internal Server Error status code
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Send a 200 OK status code
	w.WriteHeader(http.StatusOK)
}

func handleLogBucketUlidValue(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())
	bucket := req.PathValue("bucket")
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys.
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var id ulid.ULID
			if err := id.UnmarshalBinary(v); err != nil {
				return err
			}
			fmt.Printf("%s | %s\n", k, id)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("[err][api] getting k/ulid-v pairs from db: %v [%s]\n", err, cts())
		// Send a 500 Internal Server Error status code
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Send a 200 OK status code
	w.WriteHeader(http.StatusOK)
}

func handleGetUserAuthGrp(w http.ResponseWriter, req *http.Request) {
	var user *User = new(User)
	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())
	strId := req.PathValue("ulid")

	// Decode & unmarshal ulid from string into user.UserId.
	if err := unmarshalUlid(w, &user.UserId, strId); err != nil {
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
		fmt.Printf("[err][api] querying db for user's authGrp: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	// Success. Reply with user authGrp.
	encodeJsonAndRespond(w, user.AuthGrp)
}
