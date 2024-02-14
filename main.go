package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB
var dbErr error
var env *string

func loadEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("[err][api] loading .env file: %v [%s]\n", err, cts())
		os.Exit(1)
	}
}

func main() {
	// Parse command line flag.
	env = flag.String("env", "dev", "environment in which to run server (dev, prod)")
	flag.Parse()

	// Env variable are not currently needed in cp-admin end-to-end test.
	if env != nil && *env == "prod" {
		loadEnvVariables()
	}
	fmt.Printf("[api] main.go has PID: %v [%s]\n", os.Getpid(), cts())

	// Generate private key. Write to disk.
	getOrGeneratePrivateKey()

	// Open (create if it doesn't exist) cp.db data file current directory.
	db, dbErr = bolt.Open("cp.db", 0600, nil)
	if dbErr != nil {
		fmt.Printf("[err][api] opening database: %v [%s]\n", dbErr, cts())
		os.Exit(1)
	}
	defer db.Close()

	// Create db buckets.
	dbErr = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("USER_EMAIL"))
		if err != nil {
			return fmt.Errorf("error creating USER_EMAIL bucket: %w", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("USER_VERIFIED"))
		if err != nil {
			return fmt.Errorf("error creating USER_VERIFIED bucket: %w", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("USER_ADDR"))
		if err != nil {
			return fmt.Errorf("error creating USER_ADDR bucket: %w", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("USER_AUTH"))
		if err != nil {
			return fmt.Errorf("error creating USER_AUTH bucket: %w", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("BYPASS"))
		if err != nil {
			return fmt.Errorf("error creating BYPASS bucket: %w", err)
		}
		return nil
	})

	if dbErr != nil {
		fmt.Printf("[err][api] updating database: %v [%s]\n", dbErr, cts())
		os.Exit(1)
	}

	// Create HTTP request multiplexer.
	mux := http.NewServeMux()

	// Create HTTP server.
	server := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}

	// Register handler functions.
	mux.HandleFunc("POST /user/signup/", handleSignup)
	mux.HandleFunc("POST /user/login/", handleLogin)
	mux.HandleFunc("POST /user/login-code/", handleLoginCode)
	mux.HandleFunc("POST /user/logout/", authMiddleware(handleLogout))
	mux.HandleFunc("GET /admin/bypass-email/{ulid}", handleGetUserAuthGrp)
	mux.HandleFunc("POST /admin/log-bucket-custom-key/{bucket}", handleLogBucketUlidValue)
	mux.HandleFunc("POST /admin/log-bucket/{bucket}", handleLogBucketUlidKey)
	mux.HandleFunc("POST /admin/shutdown/", func(w http.ResponseWriter, req *http.Request) {
		handleShutdownServer(w, req, server)
	})

	fmt.Printf("[api] starting server on port 8000... [%s]\n", cts())

	// Serve it up.
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("[err][api] starting server: %v [%s]\n", err, cts())
		os.Exit(1)
	}
}
