//go:build test

package nmhost

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// nativeLength returns n as the four native-endian bytes that precede a
// message.
func nativeLength(n uint32) []byte {
	return binary.NativeEndian.AppendUint32(nil, n)
}

func TestReadLength(t *testing.T) {
	// The bytes of 0x01020304 differ in every position, so a reader that used
	// the other byte order would return a different value.
	const want = 0x01020304
	got, err := readLength(bytes.NewReader(append(nativeLength(want), "rest"...)))
	if err != nil {
		t.Fatalf("readLength: %v", err)
	}
	if got != want {
		t.Errorf("readLength = %#x, want %#x", got, want)
	}
}

func TestReadLengthUnreadable(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  error
	}{
		{"empty", nil, io.EOF},
		{"short", []byte{1, 2, 3}, io.ErrUnexpectedEOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := readLength(bytes.NewReader(tt.input)); !errors.Is(err, tt.want) {
				t.Errorf("readLength error = %v, want %v", err, tt.want)
			}
		})
	}
}

// writeCounter records every Write call it receives.
type writeCounter struct {
	writes [][]byte
}

func (w *writeCounter) Write(p []byte) (int, error) {
	w.writes = append(w.writes, bytes.Clone(p))
	return len(p), nil
}

func TestWriteFrame(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"small", []byte(`{"v":1}`)},
		{"at the limit", bytes.Repeat([]byte("a"), maxFrameBytes)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w writeCounter
			if err := writeFrame(&w, tt.payload); err != nil {
				t.Fatalf("writeFrame: %v", err)
			}
			if len(w.writes) != 1 {
				t.Fatalf("writeFrame made %d writes, want 1", len(w.writes))
			}
			want := append(nativeLength(uint32(len(tt.payload))), tt.payload...)
			if !bytes.Equal(w.writes[0], want) {
				t.Errorf("frame differs from the length followed by the payload")
			}
		})
	}
}

func TestWriteFrameOverLimit(t *testing.T) {
	var w writeCounter
	err := writeFrame(&w, bytes.Repeat([]byte("a"), maxFrameBytes+1))
	if !errors.Is(err, errFrameTooLarge) {
		t.Errorf("writeFrame error = %v, want errFrameTooLarge", err)
	}
	if len(w.writes) != 0 {
		t.Errorf("writeFrame made %d writes, want none", len(w.writes))
	}
}
