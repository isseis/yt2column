package chief

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/strictjson"
)

// SwitchKind is what a Claude Code transcript says about /model. The zero
// value is SwitchUnreadable: a line of the transcript could not be read, so
// whether the model switched cannot be told.
type SwitchKind int

const (
	// SwitchUnreadable is a transcript with a line that is not a JSON object.
	SwitchUnreadable SwitchKind = iota
	// SwitchNone is a transcript with no record of /model.
	SwitchNone
	// SwitchFound is a transcript whose last /model record names a model.
	SwitchFound
)

// ModelSwitch is the last /model record of a transcript. Model is the
// routing model the record's model name holds as a word, ClaudeModelUnknown
// when it holds neither or both.
type ModelSwitch struct {
	Kind  SwitchKind
	Model paneinput.ClaudeModel
}

// modelSwitchRecord matches the message content Claude Code 2.1.322 wrote
// for /model, as the sample shows it; the group is the model's display name,
// such as "Opus 5.5".
var modelSwitchRecord = regexp.MustCompile("^<local-command-stdout>Set model to `([^`\n]+)` and saved as your default for new sessions</local-command-stdout>$")

// readModelSwitch reads the last /model record of a transcript, one JSON
// object per line. Contract: a line that is not one JSON object makes the
// whole transcript SwitchUnreadable (a line being written reads so, and the
// caller reads again); only a "user" line whose message content is the
// sampled record is a switch, and its model is decided only by a word of the
// display name equal to a routing model, never by a part of a word.
func readModelSwitch(transcript []byte) ModelSwitch {
	result := ModelSwitch{Kind: SwitchNone}
	for line := range bytes.SplitSeq(transcript, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		object, err := strictjson.ParseObject(line)
		if err != nil {
			return ModelSwitch{}
		}
		name, ok := switchedModelName(object)
		if !ok {
			continue
		}
		result = ModelSwitch{Kind: SwitchFound, Model: routingModelOf(name)}
	}
	return result
}

// switchedModelName returns the display name of a /model record line.
func switchedModelName(object strictjson.Object) (string, bool) {
	members, err := object.Collect("type", "message")
	if err != nil {
		return "", false
	}
	if kind, err := strictjson.RequiredString(members, "type"); err != nil || kind != "user" {
		return "", false
	}
	messageValue, err := strictjson.Required(members, "message")
	if err != nil {
		return "", false
	}
	message, err := messageValue.AsObject()
	if err != nil {
		return "", false
	}
	fields, err := message.Collect("content")
	if err != nil {
		return "", false
	}
	content, err := strictjson.RequiredString(fields, "content")
	if err != nil {
		return "", false
	}
	m := modelSwitchRecord.FindStringSubmatch(content)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// routingModelOf returns the routing model a display name holds as exactly
// one of its words, compared without case.
func routingModelOf(name string) paneinput.ClaudeModel {
	found := paneinput.ClaudeModelUnknown
	for word := range strings.FieldsSeq(name) {
		m, err := paneinput.ParseClaudeModel(strings.ToLower(word))
		if err != nil {
			continue
		}
		if found != paneinput.ClaudeModelUnknown && found != m {
			return paneinput.ClaudeModelUnknown
		}
		found = m
	}
	return found
}
