package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB
var dbErr error

func main() {
	fmt.Printf("Main.go is running with PID: %v\n", os.Getpid())

	// Generate private key. Write to disk.
	getOrGeneratePrivateKey()

	// Test signing a message
	// signMessage()

	// Open (create if it doesn't exist) cp.db data file current directory.
	db, dbErr = bolt.Open("cp.db", 0600, nil)
	if dbErr != nil {
		log.Fatalf("failed to open database: %v", dbErr)
	}
	defer db.Close()

	// Create db buckets.
	dbErr = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("USER"))
		if err != nil {
			return fmt.Errorf("failed to create bucket: %s", err)
		}
		return nil
	})

	if dbErr != nil {
		log.Fatalf("failed to update database: %v", dbErr)
	}

	// Create HTTP request multiplexer.
	mux := http.NewServeMux()

	// Parse port number from command line flag.
	port := flag.String("port", "8000", "port to listen on")
	flag.Parse()

	// Create HTTP server.
	server := &http.Server{
		Addr:    ":" + *port,
		Handler: mux,
	}

	// Register handler functions.
	mux.HandleFunc("POST /user/signup/", handleSignup)
	mux.HandleFunc("GET /user/", handleGetAllUsers)
	mux.HandleFunc("POST /admin/shutdown/", func(w http.ResponseWriter, req *http.Request) {
		handleShutdownServer(w, req, server)
	})

	fmt.Printf("Starting server on port %s...\n", *port)

	// Serve it up.
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("failed to start server: %v", err)
	}
}
