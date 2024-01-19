package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

/*
Global database variable and related error.
*/
var db *bolt.DB
var dbErr error

/*
Global utilities variables and associated interfaces.
*/
var uBolt utils_bolt
var uHttp utils_http

type idSetter interface {
	setId(id int)
}

/*
Primary types and related methods.
*/
type user struct {
	Id    int    `json:"id"`
	Email string `json:"email"`
}

func (u *user) setId(id int) {
	u.Id = id
}

/*
Http handlers.
*/
func handleSignup(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling POST to %s\n", req.URL.Path)
	var u user
	type response struct {
		UserId int `json:"id"`
	}
	// Enforce JSON Content-Type.
	if err := uHttp.verifyContentType(w, req); err != nil {
		return
	}
	// Decode JSON request body (stream) into user struct.
	if err := uHttp.decodeJsonIntoStruct(w, req, &u); err != nil {
		return
	}
	// Create user in database.
	uBolt.writeSequentially("USER", &u)
	// Marshal response struct into JSON response payload.
	js, err := json.Marshal(response{UserId: u.Id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func handleGetAllUsers(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling GET to %s\n", req.URL.Path)
	var users []user
	
	db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys.
		b := tx.Bucket([]byte("USER"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var u user
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

func handleShutdownServer(w http.ResponseWriter, req *http.Request, server *http.Server) {
	log.Printf("handling POST to %s\n", req.URL.Path)
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

	// Instantiate utilities structs.
	uBolt = utils_bolt{db: db}
	uHttp = utils_http{}

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

	// Serve it up.
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("failed to start server: %v", err)
	}

}
