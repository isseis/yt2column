// Package articlecache holds the server's in-memory article cache: the article
// key, each key's record and its state transitions, the record limit with its
// eviction, and the handles the generation and post jobs report through. It
// knows nothing of HTTP, LLMs, or the Webhook.
package articlecache

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
)

// keyDomain separates the key's encoding from any other use of the same
// secret, and names the encoding's version: a change to the encoding changes
// this string.
const keyDomain = "yt2column article key v1"

// keyHexLen is the length of a key's string form: 32 bytes in hexadecimal.
const keyHexLen = 2 * sha256.Size

// ErrInvalidKey is wrapped by ParseKey for a string that is not 64 lowercase
// hexadecimal characters. It never holds the string.
var ErrInvalidKey = errors.New("invalid article key")

// KeyInput is the validated request values the key is computed from. Only the
// server's body validation builds one.
type KeyInput struct {
	VideoURL    string // the normalized URL transcript.ValidateVideoURL returned
	Transcript  string
	Title       string // "" when the member was absent
	ChannelName string // "" when the member was absent
	Description string // "" when the member was absent
}

// Key identifies one article in the cache. It can only be built by
// Keyer.Key or ParseKey.
type Key struct {
	sum [sha256.Size]byte
}

// Keyer computes keys with one per-process secret. It can only be built by
// NewKeyer, and it never exposes the secret.
type Keyer struct {
	secret [32]byte
}

// NewKeyer returns a Keyer that uses secret as the HMAC key.
func NewKeyer(secret [32]byte) Keyer {
	return Keyer{secret: secret}
}

// Key returns the HMAC-SHA256, keyed with the secret, of keyDomain, a 0x00
// byte, and the five values in field order, each as its byte length in 8
// big-endian bytes followed by its bytes. The length prefix keeps the encoding
// unambiguous: moving the boundary between two values changes the bytes.
func (k Keyer) Key(in KeyInput) Key {
	mac := hmac.New(sha256.New, k.secret[:])
	mac.Write([]byte(keyDomain))
	mac.Write([]byte{0})
	for _, value := range []string{in.VideoURL, in.Transcript, in.Title, in.ChannelName, in.Description} {
		writeField(mac, value)
	}
	var key Key
	mac.Sum(key.sum[:0])
	return key
}

// writeField writes value's length prefix and bytes. A hash.Hash never
// returns an error from Write.
func writeField(h hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	h.Write(length[:])
	h.Write([]byte(value))
}

// ParseKey accepts exactly the form Key.String returns: 64 characters, each
// one of 0-9 and a-f. It does not trim or fold case, and its error wraps
// ErrInvalidKey without the string.
func ParseKey(s string) (Key, error) {
	if len(s) != keyHexLen {
		return Key{}, ErrInvalidKey
	}
	for i := range len(s) {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return Key{}, ErrInvalidKey
		}
	}
	var key Key
	if _, err := hex.Decode(key.sum[:], []byte(s)); err != nil {
		return Key{}, ErrInvalidKey
	}
	return key, nil
}

// String returns the key as 64 lowercase hexadecimal characters.
func (k Key) String() string {
	return hex.EncodeToString(k.sum[:])
}
