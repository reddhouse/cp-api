package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB
var dbErr error

type User struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
}

func createUser(u *User) int {
	var id uint64
	err := db.Update(func(tx *bolt.Tx) error {
		// Retrieve the USER bucket.
		b := tx.Bucket([]byte("USER"))
		// Generate ID for this user.
		// This returns an error only if the Tx is closed or not writeable.
		// That can't happen in an Update() call so ignore the error check.
		id, _ = b.NextSequence()
		u.Id = int(id)

		// Marshal user struct into JSON (byte slice).
		buf, err := json.Marshal(u)
		if err != nil {
			return err
		}

		// Persist bytes to USER bucket.
		return b.Put(itob(u.Id), buf)
	})
	if err != nil {
		log.Fatalf("failed to persist user to db: %v", err)
	}
	return int(id)
}

func handleSignup(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling POST to %s\n", req.URL.Path)

	var u User

	type response struct {
		UserId int `json:"id"`
	}

	// Enforce JSON Content-Type.
	contentType := req.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if mediaType != "application/json" {
		http.Error(w, "expect application/json Content-Type", http.StatusUnsupportedMediaType)
		return
	}

	// Decode JSON request body (stream) into user struct.
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&u); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create user in database.
	userId := createUser(&u)

	// Marshal response struct into JSON response payload.
	js, err := json.Marshal(response{UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func handleGetAllUsers(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling GET to %s\n", req.URL.Path)

	var users []User

	db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys.
		b := tx.Bucket([]byte("USER"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var u User
			err := json.Unmarshal(v, &u)
			if err != nil {
				return err
			}
			users = append(users, u)
		}

		return nil
	})

	js, err := json.Marshal(users)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func shutdownServer(server *http.Server) {
	fmt.Println("Shutting down server...")
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
			log.Fatalf("failed to shutdown server: %v", err)
		}
	}()
}

func handleShutdownServer(w http.ResponseWriter, req *http.Request, server *http.Server) {
	log.Printf("handling POST to %s\n", req.URL.Path)
	shutdownServer(server)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Bye!\n"))

}

func main() {
	fmt.Println("Main.go is running with PID:", os.Getpid())

	// db and dbErr are declared as global variables.
	// Open (create if it doesn't exist) cp.db data file current directory.
	db, dbErr = bolt.Open("cp.db", 0600, nil)
	if dbErr != nil {
		log.Fatalf("failed to open database: %v", dbErr)
	}
	defer db.Close()

	// Create db buckets.
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("USER"))
		if err != nil {
			return fmt.Errorf("failed to create bucket: %s", err)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("failed to update database: %v", dbErr)
	}

	// Create HTTP request multiplexer and register the handler functions.
	mux := http.NewServeMux()

	// Create HTTP server.
	server := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}

	// Register handler functions.
	mux.HandleFunc("POST /user/signup/", handleSignup)
	mux.HandleFunc("GET /user/", handleGetAllUsers)
	mux.HandleFunc("POST /admin/shutdown/", func(w http.ResponseWriter, req *http.Request) {
		handleShutdownServer(w, req, server)
	})

	fmt.Println("Starting server on port 8000...")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("failed to start server: %v", err)
	}

}
