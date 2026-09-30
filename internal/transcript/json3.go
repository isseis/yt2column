package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	// maxSubtitlesBytes caps the size of one json3 subtitle file.
	maxSubtitlesBytes = 8 << 20
	// maxSubtitleEvents caps the number of events in one json3 file.
	maxSubtitleEvents = 65536
)

// Static errors for the strict JSON decoding shared by the parsers, including
// the sequences encoding/json would silently repair (invalid UTF-8 and unpaired
// UTF-16 surrogate escapes) and the structural shapes the parsers reject.
var (
	errInvalidEncoding   = errors.New("input is not valid UTF-8")
	errUnpairedSurrogate = errors.New("input contains an unpaired surrogate escape")
	errTrailingJSON      = errors.New("unexpected data after the JSON value")
	errNotJSONObject     = errors.New("JSON value is not an object")
	errInputTooLarge     = errors.New("input exceeds the size limit")
	errMissingEvents     = errors.New("missing events member")
	errEventsNotArray    = errors.New("events is not an array")
	errTooManyEvents     = errors.New("too many events")
	errMissingTimestamp  = errors.New("content event is missing tStartMs")
	errNegativeTimestamp = errors.New("tStartMs is negative")
	errSegsNotArray      = errors.New("segs is not an array")
	errNonStringKey      = errors.New("JSON object key is not a string")
	errDuplicateMember   = errors.New("duplicate consumed member")
	errValueNotString    = errors.New("value is not a string")
	errValueNotNumber    = errors.New("value is not a number")
)

// Offsets within a \uXXXX escape while scanning a JSON string.
const (
	hexDigitsOffset     = 2 // index of the first hex digit after the backslash and 'u'
	hexDigitCount       = 4 // number of hex digits in one escape
	escapeLength        = 6 // length of one \uXXXX escape
	escapeLastHexOffset = 5 // index of the last hex digit within one escape
)

// parseSubtitles parses a json3 subtitle file and returns one segment per event
// that carries text. path is attached to the returned *ParseError on failure.
func parseSubtitles(path string, data []byte) ([]Segment, error) {
	segments, err := decodeSubtitles(data)
	if err != nil {
		return nil, &ParseError{Path: path, Err: fmt.Errorf("%w: %w", ErrParseSubtitles, err)}
	}
	return segments, nil
}

func decodeSubtitles(data []byte) ([]Segment, error) {
	if len(data) > maxSubtitlesBytes {
		return nil, fmt.Errorf("%w: limit %d bytes", errInputTooLarge, maxSubtitlesBytes)
	}
	decoder, err := newStrictDecoder(data)
	if err != nil {
		return nil, err
	}
	members, err := decodeJSONObject(decoder)
	if err != nil {
		return nil, err
	}
	if err := ensureNoTrailingJSON(decoder); err != nil {
		return nil, err
	}
	consumed, err := collectMembers(members, "events")
	if err != nil {
		return nil, err
	}
	events, ok := consumed["events"]
	if !ok {
		return nil, errMissingEvents
	}
	return decodeEvents(events)
}

func decodeEvents(raw json.RawMessage) ([]Segment, error) {
	decoder := newRawDecoder(raw)
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return nil, errEventsNotArray
	}

	segments := []Segment{}
	count := 0
	for decoder.More() {
		count++
		if count > maxSubtitleEvents {
			return nil, fmt.Errorf("%w: limit %d", errTooManyEvents, maxSubtitleEvents)
		}
		eventMembers, err := decodeJSONObject(decoder)
		if err != nil {
			return nil, err
		}
		segment, keep, err := decodeEvent(eventMembers)
		if err != nil {
			return nil, err
		}
		if keep {
			segments = append(segments, segment)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return segments, nil
}

// decodeEvent returns one segment and whether the event should be kept. Events
// without segs and events whose concatenated text is whitespace only are
// discarded; their tStartMs is not validated.
func decodeEvent(members []jsonMember) (Segment, bool, error) {
	consumed, err := collectMembers(members, "tStartMs", "segs")
	if err != nil {
		return Segment{}, false, err
	}
	segs, ok := consumed["segs"]
	if !ok {
		return Segment{}, false, nil
	}
	text, err := decodeSegs(segs)
	if err != nil {
		return Segment{}, false, err
	}
	if strings.TrimSpace(text) == "" {
		return Segment{}, false, nil
	}
	start, ok := consumed["tStartMs"]
	if !ok {
		return Segment{}, false, errMissingTimestamp
	}
	startMs, err := decodeTimestamp(start)
	if err != nil {
		return Segment{}, false, err
	}
	return Segment{StartMs: startMs, Text: text}, true, nil
}

func decodeSegs(raw json.RawMessage) (string, error) {
	decoder := newRawDecoder(raw)
	token, err := decoder.Token()
	if err != nil {
		return "", err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return "", errSegsNotArray
	}

	var text strings.Builder
	for decoder.More() {
		segMembers, err := decodeJSONObject(decoder)
		if err != nil {
			return "", err
		}
		consumed, err := collectMembers(segMembers, "utf8")
		if err != nil {
			return "", err
		}
		utf8Raw, ok := consumed["utf8"]
		if !ok {
			continue
		}
		value, err := decodeString(utf8Raw)
		if err != nil {
			return "", err
		}
		text.WriteString(value)
	}
	if _, err := decoder.Token(); err != nil {
		return "", err
	}
	return text.String(), nil
}

// decodeTimestamp decodes a tStartMs value that must be a non-negative integer.
func decodeTimestamp(raw json.RawMessage) (int64, error) {
	value, err := decodeInt64(raw)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, errNegativeTimestamp
	}
	return value, nil
}

// jsonMember is one member of a JSON object, kept in document order.
type jsonMember struct {
	key string
	raw json.RawMessage
}

// decodeJSONObject reads exactly one JSON object and returns its members. The
// caller's decoder must be positioned before the object.
func decodeJSONObject(decoder *json.Decoder) ([]jsonMember, error) {
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

// collectMembers returns the raw value of every member whose key is consumed,
// rejecting a consumed key that appears more than once. Unknown members are
// ignored, so duplicates among them are accepted.
func collectMembers(members []jsonMember, keys ...string) (map[string]json.RawMessage, error) {
	consumed := make(map[string]json.RawMessage, len(keys))
	for _, member := range members {
		if !slices.Contains(keys, member.key) {
			continue
		}
		if _, ok := consumed[member.key]; ok {
			return nil, fmt.Errorf("%w: %q", errDuplicateMember, member.key)
		}
		consumed[member.key] = member.raw
	}
	return consumed, nil
}

// newStrictDecoder validates the raw bytes and returns a decoder that keeps
// numbers exact until the caller decides their type.
func newStrictDecoder(data []byte) (*json.Decoder, error) {
	if err := validateJSONEncoding(data); err != nil {
		return nil, err
	}
	return newRawDecoder(data), nil
}

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

func decodeString(raw json.RawMessage) (string, error) {
	token, err := newRawDecoder(raw).Token()
	if err != nil {
		return "", err
	}
	value, ok := token.(string)
	if !ok {
		return "", errValueNotString
	}
	return value, nil
}

func decodeInt64(raw json.RawMessage) (int64, error) {
	token, err := newRawDecoder(raw).Token()
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

// validateJSONEncoding rejects raw byte sequences that encoding/json would
// repair silently: invalid UTF-8 and unpaired UTF-16 surrogate escapes. The
// check covers the whole document, including members the parser ignores, and
// runs before any field is consumed.
func validateJSONEncoding(data []byte) error {
	if !utf8.Valid(data) {
		return errInvalidEncoding
	}
	return validateSurrogates(data)
}

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
