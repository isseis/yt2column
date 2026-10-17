package artifact

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/strictjson"
	"github.com/isseis/yt2column/internal/writer"
)

// JSON keys of an artifact. Parse and Marshal both read these constants, so
// the format is defined once.
const (
	keyVersion      = "version"
	keyStage        = "stage"
	keyModel        = "model"
	keyModelVersion = "model_version"
	keyUsage        = "usage"
	keyCallTime     = "call_time"
	keySystem       = "system"
	keyUser         = "user"
	keyInput        = "input"
	keyID           = "id"
	keyTitle        = "title"
	keyBody         = "body"
	keyRevisions    = "revisions"
	keyEmbedded     = "embedded"
	keyDigest       = "digest"
	keyCacheRead    = "cache_read"
	keyCacheWrite   = "cache_write"
	keyOutput       = "output"
)

// artifactKeys are the top-level keys CollectOnly accepts.
var artifactKeys = []string{
	keyVersion, keyStage, keyModel, keyModelVersion, keyUsage, keyCallTime,
	keySystem, keyUser, keyInput, keyID, keyTitle, keyBody, keyRevisions,
}

// usageKeys are the keys a known-usage object must carry, in the order they are
// read and written.
var usageKeys = []string{keyInput, keyCacheRead, keyCacheWrite, keyOutput}

// identityKeys are the keys a template identity object may carry.
var identityKeys = []string{keyEmbedded, keyDigest}

// recordJSON is the wire form of a StageRecord. Field order is the key order
// in the output; Revisions is a pointer so a review with no revisions still
// writes an empty array while generation and shorten omit the key. Usage is a
// pointer so a known usage writes an object and an unknown usage writes null.
type recordJSON struct {
	Version      int             `json:"version"`
	Stage        string          `json:"stage"`
	Model        string          `json:"model"`
	ModelVersion string          `json:"model_version"`
	Usage        *usageJSON      `json:"usage"`
	CallTime     string          `json:"call_time"`
	System       identityJSON    `json:"system"`
	User         identityJSON    `json:"user"`
	Input        string          `json:"input"`
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Revisions    *[]revisionJSON `json:"revisions,omitempty"`
}

// usageJSON is the wire form of a known token usage. An unknown usage is
// written as the JSON literal null instead.
type usageJSON struct {
	Input      uint64 `json:"input"`
	CacheRead  uint64 `json:"cache_read"`
	CacheWrite uint64 `json:"cache_write"`
	Output     uint64 `json:"output"`
}

// identityJSON is the wire form of a TemplateIdentity.
type identityJSON struct {
	Embedded bool   `json:"embedded"`
	Digest   string `json:"digest"`
}

// revisionJSON is the wire form of a Revision: After for a replacement,
// Delete for a deletion.
type revisionJSON struct {
	Before   string `json:"before"`
	After    string `json:"after,omitempty"`
	Delete   bool   `json:"delete,omitempty"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
}

// Marshal builds the JSON body of one artifact from a record and returns it
// with the record's identifier, computed from its content and stored under the
// id key. It does
// not escape HTML, so a body's "<", ">", and "&" do not expand (the file is
// read back by this package, not by a browser). It then rejects the result
// unless Parse accepts it, so a save never writes an artifact the load path
// refuses.
func Marshal(record writer.StageRecord) (data []byte, id string, err error) {
	if err := checkCallTime(record.CallTime); err != nil {
		return nil, "", err
	}
	id = recordID(record)
	out := recordJSON{
		Version:      artifactVersion,
		Stage:        record.Stage.String(),
		Model:        record.Model,
		ModelVersion: record.ModelVersion,
		CallTime:     record.CallTime.Format(CallTimeLayout),
		System:       identityJSON{Embedded: record.System.Embedded, Digest: record.System.Digest},
		User:         identityJSON{Embedded: record.User.Embedded, Digest: record.User.Digest},
		Input:        record.Input,
		ID:           id,
		Title:        record.Title,
		Body:         record.Body,
	}
	if record.Stage == writer.StageReview {
		out.Revisions = revisionsJSON(record.Revisions)
	}
	if counts, known := record.Usage.Counts(); known {
		out.Usage = &usageJSON{
			Input:      counts.Input,
			CacheRead:  counts.CacheRead,
			CacheWrite: counts.CacheWrite,
			Output:     counts.Output,
		}
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(out); err != nil {
		return nil, "", err
	}
	data = bytes.TrimSuffix(buffer.Bytes(), []byte("\n"))
	if len(data) > maxArtifactBytes {
		return nil, "", malformedf("artifact exceeds the limit of %d bytes", maxArtifactBytes)
	}
	// Round-trip through Parse so a save never writes an artifact the load
	// path refuses: a value Parse rejects (a revision list on a non-review
	// stage, an unnormalized title, an invalid model name) fails here, at
	// save time, rather than silently writing an unloadable file.
	if _, err := Parse(data); err != nil {
		return nil, "", err
	}
	return data, id, nil
}

// Parse validates one artifact's bytes and returns its record. It rejects any
// content that breaks a rule, never partly accepts a file, and names the
// broken rule only, never an artifact value.
func Parse(data []byte) (writer.StageRecord, error) {
	if len(data) > maxArtifactBytes {
		return writer.StageRecord{}, malformedf("artifact exceeds the limit of %d bytes", maxArtifactBytes)
	}
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return writer.StageRecord{}, malformed("artifact is not one JSON object")
	}
	members, err := object.CollectOnly(artifactKeys...)
	if err != nil {
		return writer.StageRecord{}, malformedf("artifact has an unlisted or duplicate key: %s", err)
	}
	stage, err := parseStage(members)
	if err != nil {
		return writer.StageRecord{}, err
	}
	model, err := parseModel(members)
	if err != nil {
		return writer.StageRecord{}, err
	}
	modelVersion, err := parseModelVersion(members)
	if err != nil {
		return writer.StageRecord{}, err
	}
	usage, err := parseUsage(members)
	if err != nil {
		return writer.StageRecord{}, err
	}
	callTime, err := parseCallTime(members)
	if err != nil {
		return writer.StageRecord{}, err
	}
	system, err := parseIdentityMember(members, keySystem)
	if err != nil {
		return writer.StageRecord{}, err
	}
	user, err := parseIdentityMember(members, keyUser)
	if err != nil {
		return writer.StageRecord{}, err
	}
	input, err := parseDigestMember(members, keyInput)
	if err != nil {
		return writer.StageRecord{}, err
	}
	id, err := parseDigestMember(members, keyID)
	if err != nil {
		return writer.StageRecord{}, err
	}
	title, body, err := parseText(members)
	if err != nil {
		return writer.StageRecord{}, err
	}
	revisions, err := parseRevisions(members, stage)
	if err != nil {
		return writer.StageRecord{}, err
	}
	record := writer.StageRecord{
		Stage:        stage,
		Title:        title,
		Body:         body,
		Revisions:    revisions,
		Model:        model,
		ModelVersion: modelVersion,
		Usage:        usage,
		CallTime:     callTime,
		System:       system,
		User:         user,
		Input:        input,
		ID:           id,
	}
	if recordID(record) != id {
		return writer.StageRecord{}, malformed("id does not match the artifact's content")
	}
	return record, nil
}

// parseStage reads the format version and stage. It rejects an unsupported
// version and an unknown stage.
func parseStage(members map[string]strictjson.Value) (writer.Stage, error) {
	versionValue, err := strictjson.Required(members, keyVersion)
	if err != nil {
		return writer.StageUnset, malformedf("version: %s", err)
	}
	version, err := versionValue.AsInt64()
	if err != nil {
		return writer.StageUnset, malformedf("version: %s", err)
	}
	if version != artifactVersion {
		return writer.StageUnset, malformedf(
			"unsupported artifact version %d; this CLI reads version %d only; move away the artifacts an older CLI saved, or use a new --artifacts directory, then rerun",
			version, artifactVersion)
	}
	wire, err := strictjson.RequiredString(members, keyStage)
	if err != nil {
		return writer.StageUnset, malformedf("stage: %s", err)
	}
	stage, ok := stageFromWire(wire)
	if !ok {
		return writer.StageUnset, malformed("stage is not known")
	}
	return stage, nil
}

// parseModel reads the required model name and applies the response rule.
func parseModel(members map[string]strictjson.Value) (string, error) {
	model, err := strictjson.RequiredString(members, keyModel)
	if err != nil {
		return "", malformedf("model: %s", err)
	}
	if err := writer.CheckModelText("model", model); err != nil {
		return "", malformedFrom(err)
	}
	return model, nil
}

// parseModelVersion reads the required model version, which may be empty; a
// non-empty one must satisfy the response rule.
func parseModelVersion(members map[string]strictjson.Value) (string, error) {
	value, err := strictjson.Required(members, keyModelVersion)
	if err != nil {
		return "", malformedf("model_version: %s", err)
	}
	version, err := value.AsString()
	if err != nil {
		return "", malformedf("model_version: %s", err)
	}
	if version == "" {
		return "", nil
	}
	if err := writer.CheckModelText("model_version", version); err != nil {
		return "", malformedFrom(err)
	}
	return version, nil
}

// parseUsage reads the required usage member: the JSON literal null is an
// unknown usage, an object is a known one whose four count members are all
// required non-negative integers. An unlisted or duplicate key, a missing
// member, or a value that is not a non-negative integer is ErrMalformed. The
// reason is not recorded. It never carries a file value.
func parseUsage(members map[string]strictjson.Value) (llm.Usage, error) {
	value, present := members[keyUsage]
	if !present {
		return llm.Usage{}, malformed("usage: missing")
	}
	if value.IsNull() {
		return llm.UnknownUsage(llm.RuleRecordedUnknown, ""), nil
	}
	object, err := value.AsObject()
	if err != nil {
		return llm.Usage{}, malformedf("usage: %s", err)
	}
	usageMembers, err := object.CollectOnly(usageKeys...)
	if err != nil {
		return llm.Usage{}, malformed("usage has an unlisted or duplicate key")
	}
	counts := llm.TokenCounts{}
	for _, member := range []struct {
		key  string
		dest *uint64
	}{
		{keyInput, &counts.Input},
		{keyCacheRead, &counts.CacheRead},
		{keyCacheWrite, &counts.CacheWrite},
		{keyOutput, &counts.Output},
	} {
		raw, present := usageMembers[member.key]
		if !present {
			return llm.Usage{}, malformedf("usage.%s: missing", member.key)
		}
		number, err := raw.AsNonNegativeInt64()
		if err != nil {
			return llm.Usage{}, malformedf("usage.%s: not a non-negative integer", member.key)
		}
		// AsNonNegativeInt64 returns a value in [0, MaxInt64].
		*member.dest = uint64(number) //nolint:gosec // number is non-negative
	}
	return llm.KnownUsage(counts), nil
}

// parseCallTime reads the required call_time member: a UTC time in whole
// seconds, written in exactly CallTimeLayout. It rejects any other spelling,
// including a targeted zone, a fractional second, a missing second, and a
// range-exceeding date, by requiring the parsed value to re-render as the same
// bytes. It rejects the zero time, because the in-memory zero value means "no
// call time". The reason never carries a file value.
func parseCallTime(members map[string]strictjson.Value) (time.Time, error) {
	raw, err := strictjson.RequiredString(members, keyCallTime)
	if err != nil {
		return time.Time{}, malformedf("call_time: %s", err)
	}
	parsed, err := time.Parse(CallTimeLayout, raw)
	if err != nil {
		return time.Time{}, malformed("call_time: not a UTC time in whole seconds")
	}
	if parsed.Format(CallTimeLayout) != raw {
		return time.Time{}, malformed("call_time: not a UTC time in whole seconds")
	}
	if parsed.IsZero() {
		return time.Time{}, malformed("call_time: zero time")
	}
	return parsed, nil
}

// checkCallTime rejects a record whose call time Marshal must not write: the
// zero time, a time not in UTC, or a time with sub-second precision. Writing
// one would store a wrong or lossy value that Parse cannot detect, because the
// layout's trailing Z is a literal character.
func checkCallTime(callTime time.Time) error {
	if callTime.IsZero() {
		return malformed("call_time: zero time")
	}
	if callTime.Location() != time.UTC || callTime.Nanosecond() != 0 {
		return malformed("call_time: not a UTC time in whole seconds")
	}
	return nil
}

// parseIdentityMember reads a required template identity object.
func parseIdentityMember(members map[string]strictjson.Value, key string) (writer.TemplateIdentity, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return writer.TemplateIdentity{}, malformedf("%s: %s", key, err)
	}
	identity, err := parseIdentity(value)
	if err != nil {
		return writer.TemplateIdentity{}, err
	}
	return identity, nil
}

// parseIdentity reads one template identity object: an embedded flag and a
// digest of the template text.
func parseIdentity(value strictjson.Value) (writer.TemplateIdentity, error) {
	object, err := value.AsObject()
	if err != nil {
		return writer.TemplateIdentity{}, malformedf("template identity: %s", err)
	}
	members, err := object.CollectOnly(identityKeys...)
	if err != nil {
		return writer.TemplateIdentity{}, malformed("template identity has an unlisted or duplicate key")
	}
	embeddedValue, err := strictjson.Required(members, keyEmbedded)
	if err != nil {
		return writer.TemplateIdentity{}, malformedf("embedded: %s", err)
	}
	embedded, err := embeddedValue.AsBool()
	if err != nil {
		return writer.TemplateIdentity{}, malformedf("embedded: %s", err)
	}
	digest, err := strictjson.RequiredString(members, keyDigest)
	if err != nil {
		return writer.TemplateIdentity{}, malformedf("template digest: %s", err)
	}
	if !isHexDigest(digest) {
		return writer.TemplateIdentity{}, malformed("template digest is not a SHA-256 digest in hex")
	}
	return writer.TemplateIdentity{Embedded: embedded, Digest: digest}, nil
}

// parseDigestMember reads a required non-empty SHA-256 hex digest member.
func parseDigestMember(members map[string]strictjson.Value, key string) (string, error) {
	value, err := strictjson.RequiredString(members, key)
	if err != nil {
		return "", malformedf("%s: %s", key, err)
	}
	if !isHexDigest(value) {
		return "", malformedf("%s is not a SHA-256 digest in hex", key)
	}
	return value, nil
}

// parseText reads and validates the required title and body. A body starting
// with a byte order mark is rejected, not stripped: writer.CheckStageText
// rejects any text its checks would have normalized, the removed mark
// included.
func parseText(members map[string]strictjson.Value) (title, body string, err error) {
	title, err = strictjson.RequiredString(members, keyTitle)
	if err != nil {
		return "", "", malformedf("title: %s", err)
	}
	body, err = strictjson.RequiredString(members, keyBody)
	if err != nil {
		return "", "", malformedf("body: %s", err)
	}
	if err := writer.CheckStageText(title, body); err != nil {
		return "", "", malformedFrom(err)
	}
	return title, body, nil
}

// parseRevisions reads the review list. It is required for a review and must
// be absent for generation and shorten.
func parseRevisions(members map[string]strictjson.Value, stage writer.Stage) ([]writer.Revision, error) {
	if stage != writer.StageReview {
		if _, ok := members[keyRevisions]; ok {
			return nil, malformed("revisions is present for a stage that has none")
		}
		return nil, nil
	}
	raw, err := strictjson.Required(members, keyRevisions)
	if err != nil {
		return nil, malformedf("revisions: %s", err)
	}
	revisions, err := writer.CheckRevisionList(raw)
	if err != nil {
		return nil, malformedFrom(err)
	}
	return revisions, nil
}

// revisionsJSON builds the wire revisions from a record's list.
func revisionsJSON(revisions []writer.Revision) *[]revisionJSON {
	out := make([]revisionJSON, len(revisions))
	for i, revision := range revisions {
		wire, _ := revision.Reason.WireValue()
		item := revisionJSON{Before: revision.Before, Reason: wire, Evidence: revision.Evidence}
		if revision.Action == writer.ActionDelete {
			item.Delete = true
		} else {
			item.After = revision.After
		}
		out[i] = item
	}
	return &out
}

// recordID returns the identifier of a record, computed from every value that
// defines its content. The id field itself is not part of it. The order and
// the revision tag keep distinct records from sharing an identifier.
func recordID(record writer.StageRecord) string {
	parts := []string{
		strconv.Itoa(artifactVersion),
		record.Stage.String(),
		record.Model,
		record.ModelVersion,
	}
	parts = append(parts, usageIDParts(record.Usage)...)
	parts = append(parts, record.CallTime.Format(CallTimeLayout))
	parts = append(parts,
		boolWire(record.System.Embedded),
		record.System.Digest,
		boolWire(record.User.Embedded),
		record.User.Digest,
		record.Input,
		record.Title,
		record.Body,
		strconv.Itoa(len(record.Revisions)),
	)
	for _, revision := range record.Revisions {
		wire, _ := revision.Reason.WireValue()
		if revision.Action == writer.ActionDelete {
			parts = append(parts, revision.Before, "delete", wire, revision.Evidence)
		} else {
			parts = append(parts, revision.Before, "after", revision.After, wire, revision.Evidence)
		}
	}
	return writer.Digest(parts...)
}

// usageIDParts is the usage part of an id: the fixed word usage-known and the
// four decimal counts for a known usage, or usage-unknown for an unknown one.
// The reason is not part of it, so an unknown usage keeps the same id however
// it became unknown.
func usageIDParts(usage llm.Usage) []string {
	counts, known := usage.Counts()
	if !known {
		return []string{"usage-unknown"}
	}
	return []string{
		"usage-known",
		strconv.FormatUint(counts.Input, 10),
		strconv.FormatUint(counts.CacheRead, 10),
		strconv.FormatUint(counts.CacheWrite, 10),
		strconv.FormatUint(counts.Output, 10),
	}
}

// malformedFrom wraps an error from the writer's text, model, or list checks.
// That error already names the broken rule and wraps writer.ErrMalformedOutput,
// so this only adds ErrMalformed to the chain.
func malformedFrom(err error) error {
	return fmt.Errorf("%w: %w", ErrMalformed, err)
}

// boolWire is the wire spelling of a boolean part of a digest.
func boolWire(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// isHexDigest reports whether value is exactly one SHA-256 digest in lower-case
// hex, the only spelling Marshal writes.
func isHexDigest(value string) bool {
	if len(value) != sha256HexLen || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
