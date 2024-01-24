package main

import (
	"encoding/binary"
	"time"
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
