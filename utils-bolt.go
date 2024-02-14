package main

import (
	"fmt"
	"os"
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
		fmt.Printf("[err][api] creating ULID: %v [%s]\n", err, cts())
		os.Exit(1)
	}

	binId, err := id.MarshalBinary()
	if err != nil {
		fmt.Printf("[err][api] marshaling ULID: %v [%s]\n", err, cts())
		os.Exit(1)
	}

	return id, binId
}
