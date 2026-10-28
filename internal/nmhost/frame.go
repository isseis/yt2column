// Package nmhost is the Native Messaging host: it relays one message from the
// browser to the server's Unix domain socket and writes the server's reply, or
// one of the host's fixed errors, back as one message. It parses a message
// only to tell its form and, in the new form, to check its members and take
// its operation and key; it never interprets the article input (beyond
// checking that a start's input is an object) or the server's reply.
package nmhost

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// lengthBytes is the size of the length that precedes every Native
	// Messaging message, in the machine's native byte order.
	lengthBytes = 4
	// maxFrameBytes is the largest message the browser accepts from a host.
	// No reply comes near it: the largest is the base64 of a 512 KiB server
	// response (699,052 bytes) plus the fixed members.
	maxFrameBytes = 1 << 20
)

// errFrameTooLarge reports a reply over maxFrameBytes. writeFrame writes
// nothing in that case.
var errFrameTooLarge = errors.New("the reply frame exceeds the browser's message limit")

// readLength reads the length of one Native Messaging message. A stream that
// ends before the four bytes is an error (io.EOF or io.ErrUnexpectedEOF).
func readLength(r io.Reader) (uint32, error) {
	var buf [lengthBytes]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.NativeEndian.Uint32(buf[:]), nil
}

// writeFrame writes payload as one Native Messaging message.
//
// Contract: the length and the payload go out in a single Write, so standard
// output never holds a length without its payload, and a payload over
// maxFrameBytes is rejected before anything is written.
func writeFrame(w io.Writer, payload []byte) error {
	if len(payload) > maxFrameBytes {
		return fmt.Errorf("%w: %d bytes", errFrameTooLarge, len(payload))
	}
	frame := make([]byte, lengthBytes+len(payload))
	binary.NativeEndian.PutUint32(frame, uint32(len(payload))) //nolint:gosec // len(payload) <= maxFrameBytes, checked above
	copy(frame[lengthBytes:], payload)
	if _, err := w.Write(frame); err != nil {
		return fmt.Errorf("write the reply frame: %w", err)
	}
	return nil
}
