package transcript

import (
	"errors"
	"fmt"

	"github.com/isseis/yt2column/internal/strictjson"
)

// maxInfoBytes caps the size of one info.json file.
const maxInfoBytes = 8 << 20

// Static errors for the info.json parser.
var (
	errMissingID  = errors.New("missing id member")
	errIDMismatch = errors.New("id does not match the requested video ID")
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
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return videoInfo{}, err
	}
	consumed, err := object.Collect("id", "title", "channel", "description")
	if err != nil {
		return videoInfo{}, err
	}

	idValue, ok := consumed["id"]
	if !ok {
		return videoInfo{}, errMissingID
	}
	id, err := idValue.AsString()
	if err != nil {
		return videoInfo{}, err
	}
	if id != wantID {
		return videoInfo{}, fmt.Errorf("%w: %q", errIDMismatch, id)
	}

	title, err := strictjson.RequiredString(consumed, "title")
	if err != nil {
		return videoInfo{}, err
	}
	channel, err := strictjson.RequiredString(consumed, "channel")
	if err != nil {
		return videoInfo{}, err
	}

	description := ""
	if descriptionValue, ok := consumed["description"]; ok {
		description, err = descriptionValue.AsString()
		if err != nil {
			return videoInfo{}, err
		}
	}
	return videoInfo{Title: title, ChannelName: channel, Description: description}, nil
}
