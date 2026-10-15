package artifact

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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
	keySystem       = "system"
	keyUser         = "user"
	keyInput        = "input"
	keyID           = "id"
	keyTitle        = "title"
	keyBody         = "body"
	keyRevisions    = "revisions"
	keyEmbedded     = "embedded"
	keyDigest       = "digest"
)

// artifactKeys are the top-level keys CollectOnly accepts.
var artifactKeys = []string{
	keyVersion, keyStage, keyModel, keyModelVersion, keySystem, keyUser,
	keyInput, keyID, keyTitle, keyBody, keyRevisions,
}

// identityKeys are the keys a template identity object may carry.
var identityKeys = []string{keyEmbedded, keyDigest}

// recordJSON is the wire form of a StageRecord. Field order is the key order
// in the output; Revisions is a pointer so a review with no revisions still
// writes an empty array while generation and shorten omit the key.
type recordJSON struct {
	Version      int             `json:"version"`
	Stage        string          `json:"stage"`
	Model        string          `json:"model"`
	ModelVersion string          `json:"model_version"`
	System       identityJSON    `json:"system"`
	User         identityJSON    `json:"user"`
	Input        string          `json:"input"`
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Revisions    *[]revisionJSON `json:"revisions,omitempty"`
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

// Marshal builds the JSON body of one artifact from a record. It computes the
// record's identifier from its content and stores it under the id key. It does
// not escape HTML, so a body's "<", ">", and "&" do not expand (the file is
// read back by this package, not by a browser). It then rejects the result
// unless Parse accepts it, so a save never writes an artifact the load path
// refuses.
func Marshal(record writer.StageRecord) ([]byte, error) {
	out := recordJSON{
		Version:      artifactVersion,
		Stage:        record.Stage.String(),
		Model:        record.Model,
		ModelVersion: record.ModelVersion,
		System:       identityJSON{Embedded: record.System.Embedded, Digest: record.System.Digest},
		User:         identityJSON{Embedded: record.User.Embedded, Digest: record.User.Digest},
		Input:        record.Input,
		ID:           recordID(record),
		Title:        record.Title,
		Body:         record.Body,
	}
	if record.Stage == writer.StageReview {
		out.Revisions = revisionsJSON(record.Revisions)
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(out); err != nil {
		return nil, err
	}
	data := bytes.TrimSuffix(buffer.Bytes(), []byte("\n"))
	if len(data) > maxArtifactBytes {
		return nil, malformedf("artifact exceeds the limit of %d bytes", maxArtifactBytes)
	}
	// Round-trip through Parse so a save never writes an artifact the load
	// path refuses: a value Parse rejects (a revision list on a non-review
	// stage, an unnormalized title, an invalid model name) fails here, at
	// save time, rather than silently writing an unloadable file.
	if _, err := Parse(data); err != nil {
		return nil, err
	}
	return data, nil
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
		return writer.StageUnset, malformedf("unsupported artifact version %d", version)
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

// parseText reads and validates the required title and body. The body must not
// start with a byte order mark, which this package rejects rather than
// removes.
func parseText(members map[string]strictjson.Value) (title, body string, err error) {
	title, err = strictjson.RequiredString(members, keyTitle)
	if err != nil {
		return "", "", malformedf("title: %s", err)
	}
	body, err = strictjson.RequiredString(members, keyBody)
	if err != nil {
		return "", "", malformedf("body: %s", err)
	}
	if strings.HasPrefix(body, byteOrderMark) {
		return "", "", malformed("body starts with a byte order mark")
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
		boolWire(record.System.Embedded),
		record.System.Digest,
		boolWire(record.User.Embedded),
		record.User.Digest,
		record.Input,
		record.Title,
		record.Body,
		strconv.Itoa(len(record.Revisions)),
	}
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
