package fixprguard

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const (
	// nonceBytes is 128 bits of randomness per block.
	nonceBytes = 16
	// maxNonceAttempts bounds the redraws when a nonce occurs in the data.
	maxNonceAttempts = 8

	fenceBeginPrefix = "BEGIN-UNTRUSTED-"
	fenceEndPrefix   = "END-UNTRUSTED-"
)

// Block is data wrapped between a BEGIN-UNTRUSTED-<nonce> line and an
// END-UNTRUSTED-<nonce> line. Text is the whole block, ending in a newline.
type Block struct {
	Nonce string
	Text  string
}

// Fence wraps data in a block whose delimiter does not occur in data. The
// nonce is drawn from rand and redrawn while it occurs in data (compared
// without ASCII case), so data cannot contain a line that ends the block; it
// fails after maxNonceAttempts draws. rand is a parameter so a test can force a
// collision.
func Fence(data []byte, rand io.Reader) (Block, error) {
	folded := bytes.ToLower(data)
	buf := make([]byte, nonceBytes)
	for range maxNonceAttempts {
		if _, err := io.ReadFull(rand, buf); err != nil {
			return Block{}, fmt.Errorf("read nonce: %w", err)
		}
		nonce := hex.EncodeToString(buf)
		if bytes.Contains(folded, []byte(nonce)) {
			continue
		}
		var text strings.Builder
		text.WriteString(fenceBeginPrefix)
		text.WriteString(nonce)
		text.WriteByte('\n')
		text.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			text.WriteByte('\n')
		}
		text.WriteString(fenceEndPrefix)
		text.WriteString(nonce)
		text.WriteByte('\n')
		return Block{Nonce: nonce, Text: text.String()}, nil
	}
	return Block{}, errNonceExhausted
}
