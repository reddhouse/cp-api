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

	// Parse command line flag.
	env = flag.String("env", "dev", "environment in which to run server (dev, e2e, prod)")
	flag.Parse()

	// Load environment variables in dev only. Guard pointer dereference.
	if env != nil && *env == "dev" {
		loadEnvVariables()
	}

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

		// Write Administrator's ULID:email to db.
		_, adminOneBinId, err := parseUlidString(os.Getenv("ADMIN_ONE_ULID"))
		if err != nil {
			return err
		}
		err = aeb.Put(adminOneBinId, []byte(os.Getenv("ADMIN_ONE_EMAIL")))
		if err != nil {
			return err
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
		if _, err := tx.CreateBucketIfNotExists([]byte("MOD_EXIM")); err != nil {
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

	// Create file server.
	fileServer := http.FileServer(http.Dir("./ui/static/"))

	// Create HTTP request multiplexer.
	mux := http.NewServeMux()

	// Create HTTP server.
	server := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}

	// Register handler functions.
	// The mux.HandleFunc method takes the provided function and wraps it in a
	// value of type http.HandlerFunc, which is a type that satisfies the
	// http.Handler interface which requires a method with the signature
	// ServeHTTP(http.ResponseWriter, *http.Request).
	mux.HandleFunc("GET /", ssrHome)

	// Since the fileServer is already pointed to the ./ui/static dir, when a
	// request is made to /static/main.js (example) it should only search for
	// the "/main.js" file, hence strip prefix.
	mux.Handle("GET /static/", http.StripPrefix("/static", fileServer))

	mux.HandleFunc("GET /exim/details/", ssrEximDetails)
	mux.HandleFunc("GET /api/exim/{ulid}", handleGetEximDetails)
	mux.HandleFunc("GET /exim/create/", ssrCreateExim)
	mux.HandleFunc("POST /api/exim/create/", authMiddleware(handleCreateExim))
	mux.HandleFunc("POST /api/user/signup/", handleSignup)
	mux.HandleFunc("POST /api/user/login/", handleLogin)
	mux.HandleFunc("POST /api/user/login-code/", handleLoginCode)
	mux.HandleFunc("POST /api/user/logout/", authMiddleware(handleLogout))
	mux.HandleFunc("GET /api/admin/bypass-email/{ulid}", adminMiddleware(handleGetUserAuthGrp))
	mux.HandleFunc("POST /api/admin/log-bucket-custom-key/{bucket}", adminMiddleware(handleLogBucketUlidValue))
	mux.HandleFunc("POST /api/admin/log-bucket/{bucket}", adminMiddleware(handleLogBucketUlidKey))
	mux.HandleFunc("POST /api/admin/shutdown/", adminMiddleware(func(w http.ResponseWriter, req *http.Request) {
		handleShutdownServer(w, req, server)
	}))

	fmt.Printf("[api] starting server on port 8000... [%s]\n", cts())

	// Serve it up.
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("[err][api] starting server: %v [%s]\n", err, cts())
		os.Exit(1)
	}
}
