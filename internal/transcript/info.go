package transcript

import (
	"encoding/json"
	"errors"
	"fmt"
)

// maxInfoBytes caps the size of one info.json file.
const maxInfoBytes = 8 << 20

// Static errors for the info.json parser.
var (
	errMissingID    = errors.New("missing id member")
	errIDMismatch   = errors.New("id does not match the requested video ID")
	errMissingField = errors.New("missing required member")
	errEmptyField   = errors.New("required member is empty")
)

// videoInfo is the metadata extracted from info.json.
type videoInfo struct {
	Title       string
	ChannelName string
	Description string
}

// parseInfo parses an info.json file and returns its metadata. The id member
// must match wantID. path is attached to the returned *ParseError on failure.
func parseInfo(path, wantID string, data []byte) (videoInfo, error) {
	info, err := decodeInfo(wantID, data)
	if err != nil {
		return videoInfo{}, &ParseError{Path: path, Err: fmt.Errorf("%w: %w", ErrParseInfo, err)}
	}
	return info, nil
}

func decodeInfo(wantID string, data []byte) (videoInfo, error) {
	if len(data) > maxInfoBytes {
		return videoInfo{}, fmt.Errorf("%w: limit %d bytes", errInputTooLarge, maxInfoBytes)
	}
	decoder, err := newStrictDecoder(data)
	if err != nil {
		return videoInfo{}, err
	}
	members, err := decodeJSONObject(decoder)
	if err != nil {
		return videoInfo{}, err
	}
	if err := ensureNoTrailingJSON(decoder); err != nil {
		return videoInfo{}, err
	}
	consumed, err := collectMembers(members, "id", "title", "channel", "description")
	if err != nil {
		return videoInfo{}, err
	}

	idRaw, ok := consumed["id"]
	if !ok {
		return videoInfo{}, errMissingID
	}
	id, err := decodeString(idRaw)
	if err != nil {
		return videoInfo{}, err
	}
	if id != wantID {
		return videoInfo{}, fmt.Errorf("%w: %q", errIDMismatch, id)
	}

	title, err := requiredString(consumed, "title")
	if err != nil {
		return videoInfo{}, err
	}
	channel, err := requiredString(consumed, "channel")
	if err != nil {
		return videoInfo{}, err
	}

	description := ""
	if descriptionRaw, ok := consumed["description"]; ok {
		description, err = decodeString(descriptionRaw)
		if err != nil {
			return videoInfo{}, err
		}
	}
	return videoInfo{Title: title, ChannelName: channel, Description: description}, nil
}

// requiredString returns the named member's value, rejecting a missing, empty,
// or non-string value.
func requiredString(consumed map[string]json.RawMessage, key string) (string, error) {
	raw, ok := consumed[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", errMissingField, key)
	}
	value, err := decodeString(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	if value == "" {
		return "", fmt.Errorf("%w: %s", errEmptyField, key)
	}
	return value, nil
}
