package main

import (
	"log"
	"strings"
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

// Creates a 32-byte key which is the concatenation of the ULID + descriptor.
func createCompositeKey(bid []byte, descriptor string) []byte {
	const maxDescriptorLength = 16
	if len(descriptor) > maxDescriptorLength {
		log.Fatalf("[error-api] creating key (descriptor must be less than or equal to %d bytes)", maxDescriptorLength)
	}
	// Add padding if the descriptor is too short.
	padding := strings.Repeat("\x00", maxDescriptorLength-len(descriptor))
	bpd := []byte(descriptor + padding)

	// Return descriptor with ULID prefix.
	return append(bid, bpd...)
}

// Extracts the ULID from a composite key.
func decodeCompositeKey(k []byte) (ulid.ULID, string) {
	const ulidLength = 16
	const descriptorLength = 16
	const totalLength = ulidLength + descriptorLength

	if len(k) < (ulidLength + descriptorLength) {
		log.Fatalf("[error-api] invalid composite key (should be exactly %d bytes)", totalLength)
	}
	// Return the first 16 bytes of the composite key, which is the ULID.
	var id ulid.ULID
	if err := id.UnmarshalBinary(k[:ulidLength]); err != nil {
		log.Fatalf("[error-api] unmarshaling ULID: %v", err)
	}
	descriptor := string(k[ulidLength:totalLength])
	trimmedDescriptor := strings.TrimRight(descriptor, "\x00")

	return id, trimmedDescriptor
}
