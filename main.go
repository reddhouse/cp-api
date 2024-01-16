package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"

	bolt "go.etcd.io/bbolt"
)

var db *bolt.DB

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

		// Marshal user data into bytes.
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

func signup(w http.ResponseWriter, req *http.Request) {
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

	// Decode JSON request body into Go value(s).
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&u); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create user in database.
	userId := createUser(&u)

	// Marshal Go value(s) into JSON response payload.
	js, err := json.Marshal(response{UserId: userId})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func getAllUsers(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling GET to %s\n", req.URL.Path)

	db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys.
		b := tx.Bucket([]byte("USER"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)
		}

		return nil
	})
}

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func main() {
	var dbErr error
	// Open (create if it doesn't exist) cp.db data file current directory.
	db, dbErr = bolt.Open("cp.db", 0600, nil)
	if dbErr != nil {
		log.Fatalf("failed to open database: %v", dbErr)
	}
	defer db.Close()

	// Create buckets.
	db.Update(func(tx *bolt.Tx) error {
		_, dbErr = tx.CreateBucket([]byte("USER"))
		if dbErr != nil {
			return fmt.Errorf("failed to create bucket: %s", dbErr)
		}
		return nil
	})

	// Create HTTP request multiplexer and register the handler functions.
	mux := http.NewServeMux()
	
	mux.HandleFunc("POST /user/signup/", signup)
	mux.HandleFunc("GET /user/", getAllUsers)

	fmt.Println("Starting server on port 8000...")
	log.Fatal(http.ListenAndServe("localhost:8000", mux))
}
