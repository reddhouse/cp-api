package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB
var dbErr error
var isTestRun *bool

func loadEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}
}

func main() {
	log.SetPrefix("[cp-api] ")
	// Parse command line flag.
	isTestRun = flag.Bool("test", false, "instruct server of special test client")
	flag.Parse()

	// Env variable are not currently needed in cp-admin end-to-end test.
	if !*isTestRun {
		loadEnvVariables()
	}
	fmt.Printf("[cp-api] Main.go (cp-api) has PID: %v\n", os.Getpid())

	// Generate private key. Write to disk.
	getOrGeneratePrivateKey()

	// Open (create if it doesn't exist) cp.db data file current directory.
	db, dbErr = bolt.Open("cp.db", 0600, nil)
	if dbErr != nil {
		log.Fatalf("error opening database: %v", dbErr)
	}
	defer db.Close()

	// Create db buckets.
	dbErr = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("USER"))
		if err != nil {
			return fmt.Errorf("error creating bucket: %s", err)
		}
		return nil
	})

	if dbErr != nil {
		log.Fatalf("error updating database: %v", dbErr)
	}

	// Create HTTP request multiplexer.
	mux := http.NewServeMux()

	var port string
	if *isTestRun {
		port = "8001"
	} else {
		port = "8000"
	}

	// Create HTTP server.
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Register handler functions.
	mux.HandleFunc("POST /user/signup/", handleSignup)
	mux.HandleFunc("GET /user/", handleGetAllUsers)
	mux.HandleFunc("POST /admin/shutdown/", func(w http.ResponseWriter, req *http.Request) {
		handleShutdownServer(w, req, server)
	})

	fmt.Printf("[cp-api] Starting server on port %s...\n", port)

	// Serve it up.
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("error starting server: %v", err)
	}
}
