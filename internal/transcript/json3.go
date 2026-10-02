package transcript

import (
	"errors"
	"fmt"
	"strings"

	"github.com/isseis/yt2column/internal/strictjson"
)

const (
	// maxSubtitlesBytes caps the size of one json3 subtitle file.
	maxSubtitlesBytes = 8 << 20
	// maxSubtitleEvents caps the number of events in one json3 file.
	maxSubtitleEvents = 65536
)

// Static errors for the json3 and info.json parsers. The strict JSON decoding
// they rely on, and its errors, live in internal/strictjson.
var (
	errInputTooLarge     = errors.New("input exceeds the size limit")
	errMissingEvents     = errors.New("missing events member")
	errEventsNotArray    = errors.New("events is not a valid array")
	errTooManyEvents     = errors.New("too many events")
	errMissingTimestamp  = errors.New("content event is missing tStartMs")
	errNegativeTimestamp = errors.New("tStartMs is negative")
	errSegsNotArray      = errors.New("segs is not a valid array")
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
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return nil, err
	}
	consumed, err := object.Collect("events")
	if err != nil {
		return nil, err
	}
	events, ok := consumed["events"]
	if !ok {
		return nil, errMissingEvents
	}
	return decodeEvents(events)
}

func decodeEvents(value strictjson.Value) ([]Segment, error) {
	events, err := value.AsArray()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errEventsNotArray, err)
	}
	if len(events) > maxSubtitleEvents {
		return nil, fmt.Errorf("%w: limit %d", errTooManyEvents, maxSubtitleEvents)
	}

	segments := []Segment{}
	for _, event := range events {
		eventMembers, err := event.AsObject()
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
	return segments, nil
}

// decodeEvent returns one segment and whether the event should be kept. Events
// without segs and events whose concatenated text is whitespace only are
// discarded; their tStartMs is not validated.
func decodeEvent(members strictjson.Object) (Segment, bool, error) {
	consumed, err := members.Collect("segs")
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
	consumed, err = members.Collect("tStartMs")
	if err != nil {
		return Segment{}, false, err
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

func decodeSegs(value strictjson.Value) (string, error) {
	segs, err := value.AsArray()
	if err != nil {
		return "", fmt.Errorf("%w: %w", errSegsNotArray, err)
	}

	var text strings.Builder
	for _, seg := range segs {
		segMembers, err := seg.AsObject()
		if err != nil {
			return "", err
		}
		consumed, err := segMembers.Collect("utf8")
		if err != nil {
			return "", err
		}
		utf8Value, ok := consumed["utf8"]
		if !ok {
			continue
		}
		value, err := utf8Value.AsString()
		if err != nil {
			return "", err
		}
		text.WriteString(value)
	}
	return text.String(), nil
}

// decodeTimestamp decodes a tStartMs value that must be a non-negative integer.
func decodeTimestamp(value strictjson.Value) (int64, error) {
	startMs, err := value.AsInt64()
	if err != nil {
		return 0, err
	}
	if startMs < 0 {
		return 0, errNegativeTimestamp
	}
	return startMs, nil
}
