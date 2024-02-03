package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

func handleShutdownServer(w http.ResponseWriter, req *http.Request, server *http.Server) {
	log.Printf("Handling POST to %s\n", req.URL.Path)
	log.Println("Shutting down server...")
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
			log.Fatalf("[error-api] shutting down server: %v", err)
		}
	}()
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Bye!\n"))
}

func handleLogBucket(w http.ResponseWriter, req *http.Request) {
	log.Printf("Handling POST to %s\n", req.URL.Path)
	bucket := req.PathValue("bucket")
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys.
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var id ulid.ULID
			if err := id.UnmarshalBinary(k); err != nil {
				log.Fatalf("[error-api] unmarshaling ULID: %v", err)
			}
			fmt.Printf("%s | %s\n", id, v)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("[api-debug] error: %v\n", err)
		// Send a 500 Internal Server Error status code
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Send a 200 OK status code
	w.WriteHeader(http.StatusOK)
}
