package rooms

import (
	"crypto/rand"
	"encoding/hex"
)

// codeCharset is the guest-code alphabet: uppercase letters and digits with
// ambiguous glyphs (0/O, 1/I/L) excluded so codes are easy to read and share.
const codeCharset = "BCDFGHJKMNPQRSTVWXYZ2345679"

const codeLength = 4

// newRoomID returns a short random hex identifier for a room.
func newRoomID() string {
	var b [5]byte
	mustRand(b[:])
	return hex.EncodeToString(b[:]) // 10 hex chars
}

// newCode returns a random guest code drawn from codeCharset.
func newCode() string {
	b := make([]byte, codeLength)
	mustRand(b)
	for i := range b {
		b[i] = codeCharset[int(b[i])%len(codeCharset)]
	}
	return string(b)
}

func mustRand(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic("rooms: crypto/rand failed: " + err.Error())
	}
}
