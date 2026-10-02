// Package strictjson extracts values from JSON documents that passed strict
// checks. It rejects the byte sequences and structural shapes that
// encoding/json would silently repair, so callers only ever handle values that
// came from a document that was validated as a whole.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"unicode/utf8"
)

// maxArrayElements caps the number of elements AsArray splits from one array.
// An array with more elements is rejected before every element is split, which
// bounds the memory the split allocates when a document packs many small
// elements into one array. The cap is larger than the transcript parser's own
// event limit, so that parser still reports its count check for arrays it would
// otherwise accept.
const maxArrayElements = 1 << 17

// Static errors for the strict JSON decoding, including the sequences
// encoding/json would silently repair (invalid UTF-8 and unpaired UTF-16
// surrogate escapes) and the structural shapes rejected here.
var (
	errInvalidEncoding   = errors.New("input is not valid UTF-8")
	errUnpairedSurrogate = errors.New("input contains an unpaired surrogate escape")
	errTrailingJSON      = errors.New("unexpected data after the JSON value")
	errNotJSONObject     = errors.New("JSON value is not an object")
	errNotArray          = errors.New("JSON value is not an array")
	errTooManyElements   = errors.New("array has too many elements")
	errZeroValue         = errors.New("value is unset")
	errNonStringKey      = errors.New("JSON object key is not a string")
	errDuplicateMember   = errors.New("duplicate consumed member")
	errValueNotString    = errors.New("value is not a string")
	errValueNotNumber    = errors.New("value is not a number")
	errMissingField      = errors.New("missing required member")
	errEmptyField        = errors.New("required member is empty")
)

// Object is a JSON object taken from a document that passed ParseObject's
// checks. Its members keep document order. The zero value has no members.
type Object struct {
	members []jsonMember
}

// Value is one JSON value inside a document that passed ParseObject's checks.
// It can only be obtained from an Object or another Value. The zero value has
// no content, and its accessors return an error.
type Value struct {
	raw json.RawMessage
}

// jsonMember is one member of a JSON object, kept in document order.
type jsonMember struct {
	key string
	raw json.RawMessage
}

// ParseObject rejects invalid UTF-8 and unpaired surrogate escapes anywhere in
// data, then decodes exactly one top-level JSON object with no trailing data.
func ParseObject(data []byte) (Object, error) {
	if err := validateJSONEncoding(data); err != nil {
		return Object{}, err
	}
	decoder := newRawDecoder(data)
	members, err := decodeObject(decoder)
	if err != nil {
		return Object{}, err
	}
	if err := ensureNoTrailingJSON(decoder); err != nil {
		return Object{}, err
	}
	return Object{members: members}, nil
}

// Collect returns the consumed members by key, rejecting a consumed key that
// appears more than once. Members whose keys are not listed are ignored,
// duplicates among them included.
func (o Object) Collect(keys ...string) (map[string]Value, error) {
	consumed := make(map[string]Value, len(keys))
	for _, member := range o.members {
		if !slices.Contains(keys, member.key) {
			continue
		}
		if _, ok := consumed[member.key]; ok {
			return nil, fmt.Errorf("%w: %q", errDuplicateMember, member.key)
		}
		consumed[member.key] = Value{raw: member.raw}
	}
	return consumed, nil
}

// Has reports whether a member with the key exists. It is meant for
// diagnostics only and never rejects.
func (o Object) Has(key string) bool {
	for _, member := range o.members {
		if member.key == key {
			return true
		}
	}
	return false
}

// Required returns the named member, rejecting a missing one.
func Required(members map[string]Value, key string) (Value, error) {
	value, ok := members[key]
	if !ok {
		return Value{}, fmt.Errorf("%w: %s", errMissingField, key)
	}
	return value, nil
}

// RequiredString returns the named member as a non-empty string, rejecting a
// missing member, null, another kind, or the empty string.
func RequiredString(members map[string]Value, key string) (string, error) {
	value, err := Required(members, key)
	if err != nil {
		return "", err
	}
	text, err := value.AsString()
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	if text == "" {
		return "", fmt.Errorf("%w: %s", errEmptyField, key)
	}
	return text, nil
}

// OptionalString returns the named member as a non-empty string when it is
// present. present is false for a missing member; null, another kind, or the
// empty string is rejected.
func OptionalString(members map[string]Value, key string) (value string, present bool, err error) {
	raw, ok := members[key]
	if !ok {
		return "", false, nil
	}
	text, err := raw.AsString()
	if err != nil {
		return "", false, fmt.Errorf("%s: %w", key, err)
	}
	if text == "" {
		return "", false, fmt.Errorf("%w: %s", errEmptyField, key)
	}
	return text, true, nil
}

// AsString returns the value as a string, rejecting null and other kinds.
func (v Value) AsString() (string, error) {
	if v.raw == nil {
		return "", errZeroValue
	}
	token, err := newRawDecoder(v.raw).Token()
	if err != nil {
		return "", err
	}
	text, ok := token.(string)
	if !ok {
		return "", errValueNotString
	}
	return text, nil
}

// AsInt64 returns the value as an integer, rejecting non-numbers and
// out-of-range numbers.
func (v Value) AsInt64() (int64, error) {
	if v.raw == nil {
		return 0, errZeroValue
	}
	token, err := newRawDecoder(v.raw).Token()
	if err != nil {
		return 0, err
	}
	number, ok := token.(json.Number)
	if !ok {
		return 0, errValueNotNumber
	}
	value, err := number.Int64()
	if err != nil {
		return 0, err
	}
	return value, nil
}

// AsObject returns the value as an object, rejecting null and other kinds.
func (v Value) AsObject() (Object, error) {
	if v.raw == nil {
		return Object{}, errZeroValue
	}
	members, err := decodeObject(newRawDecoder(v.raw))
	if err != nil {
		return Object{}, err
	}
	return Object{members: members}, nil
}

// AsArray returns the value as an array, rejecting null and other kinds, and an
// array with more than maxArrayElements elements.
func (v Value) AsArray() ([]Value, error) {
	if v.raw == nil {
		return nil, errZeroValue
	}
	decoder := newRawDecoder(v.raw)
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return nil, errNotArray
	}
	values := []Value{}
	for decoder.More() {
		if len(values) >= maxArrayElements {
			return nil, fmt.Errorf("%w: limit %d", errTooManyElements, maxArrayElements)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		values = append(values, Value{raw: raw})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return values, nil
}

// decodeObject reads exactly one JSON object and returns its members. The
// caller's decoder must be positioned before the object.
func decodeObject(decoder *json.Decoder) ([]jsonMember, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, errNotJSONObject
	}

	var members []jsonMember
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, errNonStringKey
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		members = append(members, jsonMember{key: key, raw: raw})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return members, nil
}

// newRawDecoder returns a decoder that keeps numbers exact until the caller
// decides their type.
func newRawDecoder(data []byte) *json.Decoder {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder
}

// ensureNoTrailingJSON rejects any data after the single top-level JSON value.
func ensureNoTrailingJSON(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errTrailingJSON
		}
		return err
	}
	return nil
}

// validateJSONEncoding rejects raw byte sequences that encoding/json would
// repair silently: invalid UTF-8 and unpaired UTF-16 surrogate escapes. The
// check covers the whole document, including members the caller ignores, and
// runs before any field is consumed.
func validateJSONEncoding(data []byte) error {
	if !utf8.Valid(data) {
		return errInvalidEncoding
	}
	return validateSurrogates(data)
}

// Offsets within a \uXXXX escape while scanning a JSON string.
const (
	hexDigitsOffset     = 2 // index of the first hex digit after the backslash and 'u'
	hexDigitCount       = 4 // number of hex digits in one escape
	escapeLength        = 6 // length of one \uXXXX escape
	escapeLastHexOffset = 5 // index of the last hex digit within one escape
)

// Surrogate escape kinds found while scanning JSON strings.
const (
	surrogateNone = iota
	surrogateHigh
	surrogateLow
)

// validateSurrogates scans the raw bytes for \uXXXX escapes inside JSON strings
// and rejects a high surrogate without an immediately following low surrogate,
// as well as a low surrogate without a preceding high surrogate.
func validateSurrogates(data []byte) error {
	inString := false
	for i := 0; i < len(data); i++ {
		current := data[i]
		if !inString {
			if current == '"' {
				inString = true
			}
			continue
		}
		switch current {
		case '"':
			inString = false
		case '\\':
			if i+1 >= len(data) {
				return nil
			}
			if data[i+1] != 'u' {
				i++
				continue
			}
			kind, ok := surrogateKind(data, i+hexDigitsOffset)
			if !ok {
				i++
				continue
			}
			switch kind {
			case surrogateHigh:
				next := i + escapeLength
				lowKind := surrogateNone
				lowOK := false
				if next+1 < len(data) && data[next] == '\\' && data[next+1] == 'u' {
					lowKind, lowOK = surrogateKind(data, next+hexDigitsOffset)
				}
				if !lowOK || lowKind != surrogateLow {
					return errUnpairedSurrogate
				}
				i = next + escapeLastHexOffset
			case surrogateLow:
				return errUnpairedSurrogate
			default:
				i += escapeLastHexOffset
			}
		}
	}
	return nil
}

// surrogateKind classifies four hex digits as a high surrogate, a low
// surrogate, or neither. ok is false when the digits are not four hex digits.
func surrogateKind(data []byte, start int) (int, bool) {
	if start+hexDigitCount > len(data) {
		return surrogateNone, false
	}
	for offset := range hexDigitCount {
		if !isHexDigit(data[start+offset]) {
			return surrogateNone, false
		}
	}
	if data[start] != 'd' && data[start] != 'D' {
		return surrogateNone, true
	}
	switch data[start+1] {
	case '8', '9', 'a', 'A', 'b', 'B':
		return surrogateHigh, true
	case 'c', 'C', 'd', 'D', 'e', 'E', 'f', 'F':
		return surrogateLow, true
	}
	return surrogateNone, true
}

func isHexDigit(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}
