// Package secret provides a type that holds a value which must never appear
// in output, such as an API key or a Webhook URL.
package secret

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
)

// redacted is the fixed marker written in place of the original value.
const redacted = "[REDACTED]"

// errEmptyValue is returned by New when the value is empty.
var errEmptyValue = errors.New("secret: empty value")

// errZeroValue is returned by Reveal on the zero value.
var errZeroValue = errors.New("secret: zero value")

// Secret holds a value that must never appear in output (API key, Webhook URL,
// ...). The original value is captured in a closure so that reflection over the
// field, as done by fmt and log/slog, cannot reach it.
type Secret struct {
	value func() string
}

// New returns a Secret built from a non-empty value.
// It returns an error when value is empty.
func New(value string) (Secret, error) {
	if value == "" {
		return Secret{}, errEmptyValue
	}
	return Secret{value: func() string { return value }}, nil
}

// Reveal returns the original value.
// The zero value returns an error instead of an empty string.
func (s Secret) Reveal() (string, error) {
	if s.value == nil {
		return "", errZeroValue
	}
	return s.value(), nil
}

// Format writes a fixed string for every verb and never the original value.
func (Secret) Format(f fmt.State, verb rune) {
	if verb == 'q' {
		_, _ = f.Write([]byte(strconv.Quote(redacted)))
		return
	}
	_, _ = f.Write([]byte(redacted))
}

// String returns a fixed string, never the original value.
func (Secret) String() string {
	return redacted
}

// GoString returns a fixed string, never the original value.
func (Secret) GoString() string {
	return redacted
}

// LogValue returns a fixed value for slog attributes.
func (Secret) LogValue() slog.Value {
	return slog.StringValue(redacted)
}

// MarshalJSON returns a fixed string, never the original value.
func (Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(redacted)
}
