package main

import (
	"log"
	"time"

	"github.com/oklog/ulid"
	"golang.org/x/exp/rand"
)

// Generates a 16-byte ULID (Universally Unique Lexicographically Sortable
// Identifier) and then marshal it to a binary format.
// Uses x/exp/rand instead of math/rand which is safe for concurrent use by
// multiple goroutines.
func createUlid() (ulid.ULID, []byte) {
	t := time.Now().UTC()
	entropy := rand.New(rand.NewSource(uint64(t.UnixNano())))
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		log.Fatalf("[error-api] creating ULID: %v", err)
	}

	bid, err := id.MarshalBinary()
	if err != nil {
		log.Fatalf("[error-api] marshaling ULID: %v", err)
	}

	return id, bid
}
