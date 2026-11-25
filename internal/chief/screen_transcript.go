package chief

import (
	"bytes"
	"regexp"
	"slices"
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

// HumanInputKind is what the last human input of a Claude Code session was.
// The zero value is HumanInputUnreadable: a line of the transcript could not
// be read, so the last human input cannot be told.
type HumanInputKind int

const (
	// HumanInputUnreadable is a transcript with a line that is not a JSON
	// object.
	HumanInputUnreadable HumanInputKind = iota
	// HumanInputNone is a session the human typed nothing into.
	HumanInputNone
	// HumanInputCommand is a custom command, such as /mkarch 0088.
	HumanInputCommand
	// HumanInputOther is any other human input: free text, or a line of
	// another shape.
	HumanInputOther
)

// HumanInput is the last line of a session the human typed. Name and Args are
// set for HumanInputCommand: the command's name without its slash, and its
// arguments.
type HumanInput struct {
	Kind HumanInputKind
	Name string
	Args string
}

// customCommandRecord matches the message content Claude Code 2.1.325 wrote
// for a custom command the human typed, as the sample shows it: its name
// twice and its arguments.
var customCommandRecord = regexp.MustCompile(`^<command-message>([a-z0-9-]+)</command-message>\n<command-name>/([a-z0-9-]+)</command-name>\n<command-args>([^<\n]*)</command-args>$`)

// humanOrigin is the origin kind Claude Code declares on a line the human
// typed; a peer's message, a task notification, and the continuation Claude
// Code adds when a usage limit resets declare the others the samples show. A
// line of another kind is not read more widely: it makes the transcript
// unreadable.
const humanOrigin = "human"

// otherOrigins are the sampled origin kinds of lines the human did not type:
// a peer's message, a task notification, and the line Claude Code writes to
// resume the work once a usage limit has reset.
var otherOrigins = []string{"peer", "task-notification", "auto-continuation"}

// queuedCommand is the attachment type of a message delivered while the
// agent works: the human's text typed mid-turn, a peer's message, or a task
// notification, told apart by the attachment's own origin.
const queuedCommand = "queued_command"

// readLastHumanInput reads the last line of a transcript the human typed.
// Contract: a line is the human's only when Claude Code declares it so (a
// "user" line whose origin kind is "human", or a queued-command attachment
// whose own origin kind is "human", the text typed while the agent worked),
// never by its text; a line that
// is not one JSON object, that repeats a member read here, or whose origin is
// of no sampled kind makes the whole transcript HumanInputUnreadable, so a
// line that might be the human's is never skipped; the last human line is a
// command only in the sampled form with its two names equal, and any other
// human line is HumanInputOther.
func readLastHumanInput(transcript []byte) HumanInput {
	result := HumanInput{Kind: HumanInputNone}
	if !scanHumanInputs(transcript, func(in HumanInput) { result = in }) {
		return HumanInput{}
	}
	return result
}

// scanHumanInputs calls visit with each line of the transcript the human
// typed, in order, by the rule readLastHumanInput states, and reports
// whether every line could be read; a caller must not use what it visited
// when it reports false.
func scanHumanInputs(transcript []byte, visit func(HumanInput)) bool {
	for line := range bytes.SplitSeq(transcript, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		object, err := strictjson.ParseObject(line)
		if err != nil {
			return false
		}
		in, human, ok := humanInputOf(object)
		if !ok {
			return false
		}
		if human {
			visit(in)
		}
	}
	return true
}

// humanOriginOf reads an origin object: human is true for the human's kind;
// ok is false for an origin of no sampled kind.
func humanOriginOf(v strictjson.Value) (human, ok bool) {
	origin, err := objectString(v, "kind")
	switch {
	case err != nil:
		return false, false
	case origin == humanOrigin:
		return true, true
	case slices.Contains(otherOrigins, origin):
		return false, true
	default:
		return false, false
	}
}

// queuedInput reads an attachment line: a queued command is the human's
// when its own origin says so, and the human's text typed while the agent
// worked is never a command of the sampled form, so it is HumanInputOther; a
// queued command without an origin of a sampled kind cannot be told.
// Attachments of other types are not the human's.
func queuedInput(members map[string]strictjson.Value) (HumanInput, bool, bool) {
	value, ok := members["attachment"]
	if !ok {
		return HumanInput{}, false, false
	}
	attachment, err := value.AsObject()
	if err != nil {
		return HumanInput{}, false, false
	}
	fields, err := attachment.Collect("type", "origin")
	if err != nil {
		return HumanInput{}, false, false
	}
	if kind, err := strictjson.RequiredString(fields, "type"); err != nil || kind != queuedCommand {
		return HumanInput{}, false, err == nil
	}
	originValue, ok := fields["origin"]
	if !ok {
		return HumanInput{}, false, false
	}
	human, ok := humanOriginOf(originValue)
	if !ok || !human {
		return HumanInput{}, false, ok
	}
	return HumanInput{Kind: HumanInputOther}, true, true
}

// humanInputOf returns the human input of a line and whether the line is
// the human's; ok is false for a line that cannot be told to be or not to
// be the human's.
func humanInputOf(object strictjson.Object) (in HumanInput, human, ok bool) {
	members, err := object.Collect("type", "origin", "message", "attachment")
	if err != nil {
		return HumanInput{}, false, false
	}
	kind, err := strictjson.RequiredString(members, "type")
	if err != nil {
		return HumanInput{}, false, false
	}
	if kind == "attachment" {
		return queuedInput(members)
	}
	originValue, hasOrigin := members["origin"]
	if kind != "user" || !hasOrigin {
		return HumanInput{}, false, true
	}
	if human, ok := humanOriginOf(originValue); !ok || !human {
		return HumanInput{}, false, ok
	}
	other := HumanInput{Kind: HumanInputOther}
	messageValue, err := strictjson.Required(members, "message")
	if err != nil {
		return other, true, true
	}
	message, err := messageValue.AsObject()
	if err != nil {
		return other, true, true
	}
	fields, err := message.Collect("content")
	if err != nil {
		return other, true, true
	}
	content, err := strictjson.RequiredString(fields, "content")
	if err != nil {
		return other, true, true
	}
	m := customCommandRecord.FindStringSubmatch(content)
	if m == nil || m[1] != m[2] {
		return other, true, true
	}
	return HumanInput{Kind: HumanInputCommand, Name: m[2], Args: m[3]}, true, true
}

// LaterInput is whether the human typed anything after the last task
// command of a Claude Code session. The zero value is LaterInputUnreadable.
type LaterInput int

const (
	// LaterInputUnreadable is a transcript that cannot be read, or that ends
	// inside a line.
	LaterInputUnreadable LaterInput = iota
	// LaterInputNoTranscript is a session whose transcript file does not
	// exist yet.
	LaterInputNoTranscript
	// LaterInputNone is no human input after the last task command.
	LaterInputNone
	// LaterInputAfter is a human input after the last task command.
	LaterInputAfter
	// LaterInputNoTaskEmpty is a session with no task command and no human
	// input at all.
	LaterInputNoTaskEmpty
	// LaterInputNoTaskInput is a session with no task command but some human
	// input.
	LaterInputNoTaskInput
)

// readRunningCommand reads the task command the human last typed into a
// session, for display, and whether the human typed anything after it.
// Contract: a transcript that does not end at a line's end is
// RunningUnknownPartialLine and LaterInputUnreadable, never read without its
// last line; the human inputs are the ones readLastHumanInput reads, in one
// scan, so a transcript it cannot read is RunningUnknownUnreadable and
// LaterInputUnreadable; only a command whose name is one of the six task
// commands' is a running command, and the last such command is the one read;
// input says whether the human typed anything after it, whose text is never
// read; a session with no such command is RunningUnknownNotTaskCommand, with
// input telling a session with no human input at all from one with some;
// the argument is kept only when it validates for the kind, and dropped
// reports one that did not.
func readRunningCommand(transcript []byte) (cmd RunningCommand, dropped bool, input LaterInput, why RunningUnknown) {
	if len(transcript) > 0 && transcript[len(transcript)-1] != '\n' {
		return RunningCommand{}, false, LaterInputUnreadable, RunningUnknownPartialLine
	}
	found, later, seen := false, false, false
	readable := scanHumanInputs(transcript, func(in HumanInput) {
		seen = true
		if c, d, ok := taskCommandOf(in); ok {
			cmd, dropped, later, found = c, d, false, true
			return
		}
		later = found
	})
	switch {
	case !readable:
		return RunningCommand{}, false, LaterInputUnreadable, RunningUnknownUnreadable
	case !found && seen:
		return RunningCommand{}, false, LaterInputNoTaskInput, RunningUnknownNotTaskCommand
	case !found:
		return RunningCommand{}, false, LaterInputNoTaskEmpty, RunningUnknownNotTaskCommand
	case later:
		return cmd, dropped, LaterInputAfter, RunningUnknownNone
	}
	return cmd, dropped, LaterInputNone, RunningUnknownNone
}

// taskCommandOf returns the running command of a human input that is one of
// the six task commands; ok is false for any other input.
func taskCommandOf(in HumanInput) (cmd RunningCommand, dropped, ok bool) {
	if in.Kind != HumanInputCommand {
		return RunningCommand{}, false, false
	}
	kind, err := paneinput.ParseCommandKind(in.Name)
	if err != nil {
		return RunningCommand{}, false, false
	}
	cmd, dropped, err = newRunningCommand(kind, in.Args)
	if err != nil {
		return RunningCommand{}, false, false
	}
	return cmd, dropped, true
}
