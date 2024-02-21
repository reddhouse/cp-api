package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	bolt "go.etcd.io/bbolt"
)

var cpPrivateKey *rsa.PrivateKey
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
	loadEnvVariables()

	// Parse command line flag.
	env = flag.String("env", "dev", "environment in which to run server (dev, prod)")
	flag.Parse()
	// TODO: Do something in production. Note use of guard against nil pointer.
	// if env != nil && *env == "prod" { foobar() }

	fmt.Printf("[api] main.go has PID: %v [%s]\n", os.Getpid(), cts())

	// Open (create if it doesn't exist) cp.db data file current directory.
	db, dbErr = bolt.Open("cp.db", 0600, nil)
	if dbErr != nil {
		fmt.Printf("[err][api] opening database: %v [%s]\n", dbErr, cts())
		os.Exit(1)
	}
	defer db.Close()

	// Create all db buckets and set initial Administrator.
	dbErr = db.Update(func(tx *bolt.Tx) error {
		aeb, err := tx.CreateBucketIfNotExists([]byte("ADMIN_EMAIL"))
		if err != nil {
			return err
		}
		// Check to see if at least one Administrator exists.
		c := aeb.Cursor()
		adminOneExists := false
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if string(v) == os.Getenv("ADMINISTRATOR_ONE_EMAIL") {
				adminOneExists = true
				break
			}
		}
		// If no administrators, create one, log ULID, and persist to db.
		if !adminOneExists {
			// Generate ULID for Administrator #1.
			adminOneId, adminOneBinId := createUlid()
			fmt.Printf("[api] Hello Administrator! Your ID is: %v [%s]\n", adminOneId, cts())
			err := aeb.Put(adminOneBinId, []byte(os.Getenv("ADMINISTRATOR_ONE_EMAIL")))
			if err != nil {
				return err
			}
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("USER_EMAIL")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("USER_VERIFIED")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("USER_ADDR")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("USER_AUTH")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("BYPASS")); err != nil {
			return err
		}
		return nil
	})

	if dbErr != nil {
		fmt.Printf("[err][api] initializing database: %v [%s]\n", dbErr, cts())
		os.Exit(1)
	}

	// Set global private key variable.
	setPrivateKey()

	// Create HTTP request multiplexer.
	mux := http.NewServeMux()

	// Create HTTP server.
	server := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}

	// Register handler functions. http.HandlerFunc() is a type conversion.
	mux.HandleFunc("POST /user/signup/", handleSignup)
	mux.HandleFunc("POST /user/login/", handleLogin)
	mux.HandleFunc("POST /user/login-code/", handleLoginCode)
	mux.HandleFunc("POST /user/logout/", authMiddleware(handleLogout))
	mux.HandleFunc("GET /admin/bypass-email/{ulid}", adminMiddleware(handleGetUserAuthGrp))
	mux.HandleFunc("POST /admin/log-bucket-custom-key/{bucket}", adminMiddleware(handleLogBucketUlidValue))
	mux.HandleFunc("POST /admin/log-bucket/{bucket}", adminMiddleware(handleLogBucketUlidKey))
	mux.HandleFunc("POST /admin/shutdown/", adminMiddleware(func(w http.ResponseWriter, req *http.Request) {
		handleShutdownServer(w, req, server)
	}))

	fmt.Printf("[api] starting server on port 8000... [%s]\n", cts())

	// Serve it up.
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("[err][api] starting server: %v [%s]\n", err, cts())
		os.Exit(1)
	}
}
