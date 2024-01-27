package main

import (
	"encoding/binary"
	"log"
	"time"

	"github.com/oklog/ulid"
	"golang.org/x/exp/rand"
)

// Returns an 8-byte big endian representation of v. The binary.BigEndian
// functions are used to ensure that the integers are encoded in a way that
// preserves their order when the bytes are compared lexicographically, which
// is how bbolt compares keys.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

// Create a 12-byte key with the first 4 bytes representing the items's ID,
// and the last 8 bytes representing the timestamp.
func createKey(id int, timestamp time.Time) []byte {
	buf := make([]byte, 12) // 4 bytes for id, 8 bytes for timestamp
	binary.BigEndian.PutUint32(buf[:4], uint32(id))
	binary.BigEndian.PutUint64(buf[4:], uint64(timestamp.UnixNano()))
	return buf
}

// Create a 4-byte key prefix with the first 4 bytes representing the item's ID.
func createIdPrefix(id int) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(id))
	return buf
}

// prefix := createUserIDPrefix(userID)
// c := b.Cursor()
// for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
//     // process key/value pair
// }

// Generate a 16-byte ULID (Universally Unique Lexicographically Sortable
// Identifier) and then marshal it to a binary format.
// Using x/exp/rand instead of math/rand which is safe for concurrent use by
// multiple goroutines.
func createUlidKey() []byte {
	t := time.Now().UTC()
	entropy := rand.New(rand.NewSource(uint64(t.UnixNano())))
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		log.Fatalf("failed to create ULID: %v", err)
	}
	bsId, err := id.MarshalBinary()
	if err != nil {
		log.Fatalf("failed to marshal ULID: %v", err)
	}
	return bsId
}

func getTimestampFromUlid(bsId []byte) time.Time {
	// ULIDs as strings are 26 characters (ulid.EncodedSize constant). Here, we
	// have previously marshalled the ULID to binary, which is 16 bytes.
	if len(bsId) != 16 {
		log.Fatalf("invalid ULID binary size: got %v, want 16", len(bsId))
	}

	var id ulid.ULID
	copy(id[:], bsId)

	// Extract the timestamp
	timestamp := id.Time()

	// Convert the timestamp to a time.Time
	time := time.Unix(int64(timestamp)/1000, 0)
	return time
}
