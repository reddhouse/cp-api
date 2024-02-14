package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

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
	w.Write([]byte("bye!\n"))
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
	var userInst user
	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())
	strId := req.PathValue("ulid")

	// Decode & unmarshal ulid from string into userInst.UserId.
	if err := unmarshalUlid(w, &userInst.UserId, strId); err != nil {
		return
	}
	// Convert ulid to byte slice to use as db key.
	binId, err := getBinId(w, userInst.UserId)
	if err != nil {
		return
	}

	// Read user from db.
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("USER_AUTH"))
		// Retrieve authGroup from userId.
		authGrp := b.Get(binId)
		if authGrp == nil {
			return fmt.Errorf("authGroup does not exist for the userId")
		}
		// Unmarshal authGrp into userInst.
		err := json.Unmarshal(authGrp, &userInst.AuthGrp)
		if err != nil {
			return err
		}
		return nil
	})

	// Handle database error.
	if err != nil {
		fmt.Printf("[err][api] querying db for user's authGroup: %v [%s]\n", err, cts())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Success. Reply with user authGroup.
	encodeJsonAndRespond(w, userInst.AuthGrp)
}
