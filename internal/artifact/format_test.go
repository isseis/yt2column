//go:build test

package artifact

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/writer"
)

// Distinct markers used to tell where a value went and that it never reached
// an error message.
const (
	markArtifactBody = "mark-artifact-body-R4kd"
	markArtifactRev  = "mark-artifact-revision-T7wm"
)

// testMaxTextBytes mirrors the writer's unexported text limit, so the boundary
// test targets the same value the writer checks.
const testMaxTextBytes = 1 << 20

// testMaxRevisions mirrors the writer's unexported revision limit.
const testMaxRevisions = 256

// testIdentity builds a template identity whose digest is a real SHA-256.
func testIdentity(embedded bool, text string) writer.TemplateIdentity {
	return writer.TemplateIdentity{Embedded: embedded, Digest: writer.Digest(text)}
}

// validGeneration returns a generation record that passes Parse.
func validGeneration() writer.StageRecord {
	rec := writer.StageRecord{
		Stage:  writer.StageGeneration,
		Title:  "Title",
		Body:   "Body text.",
		Model:  "test-model",
		System: testIdentity(true, "system template"),
		User:   testIdentity(false, "user template"),
		Input:  writer.Digest("material"),
	}
	rec.ID = recordID(rec)
	return rec
}

// validReview returns a review record, with a replacement and a deletion, that
// passes Parse.
func validReview() writer.StageRecord {
	rec := validGeneration()
	rec.Stage = writer.StageReview
	rec.Revisions = []writer.Revision{
		{Before: "Body text.", Action: writer.ActionReplace, After: "Body revised.", Reason: writer.ReasonMeaningChange, Evidence: "evidence one"},
		{Before: "extra", Action: writer.ActionDelete, Reason: writer.ReasonUnsupported, Evidence: "evidence two"},
	}
	rec.ID = recordID(rec)
	return rec
}

// sameRecord reports whether two records carry the same values. StageRecord
// holds a slice, so it is not comparable with ==.
func sameRecord(a, b writer.StageRecord) bool {
	if a.Stage != b.Stage || a.Title != b.Title || a.Body != b.Body ||
		a.Model != b.Model || a.ModelVersion != b.ModelVersion ||
		a.System != b.System || a.User != b.User || a.Input != b.Input || a.ID != b.ID {
		return false
	}
	if len(a.Revisions) != len(b.Revisions) {
		return false
	}
	for i := range a.Revisions {
		if a.Revisions[i] != b.Revisions[i] {
			return false
		}
	}
	return true
}

// mustMarshal marshals a record or fails the test.
func mustMarshal(t *testing.T, rec writer.StageRecord) []byte {
	t.Helper()
	data, err := Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	return data
}

// mustParse parses an artifact or fails the test.
func mustParse(t *testing.T, data []byte) writer.StageRecord {
	t.Helper()
	rec, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse error = %v", err)
	}
	return rec
}

// fieldsFrom builds the wire form of a record as a field map, so a test can
// break one field at a time.
func fieldsFrom(rec writer.StageRecord) map[string]any {
	fields := map[string]any{
		"version":       float64(artifactVersion),
		"stage":         rec.Stage.String(),
		"model":         rec.Model,
		"model_version": rec.ModelVersion,
		"system":        map[string]any{"embedded": rec.System.Embedded, "digest": rec.System.Digest},
		"user":          map[string]any{"embedded": rec.User.Embedded, "digest": rec.User.Digest},
		"input":         rec.Input,
		"id":            rec.ID,
		"title":         rec.Title,
		"body":          rec.Body,
	}
	if rec.Stage == writer.StageReview {
		revisions := make([]any, len(rec.Revisions))
		for i, revision := range rec.Revisions {
			wire, _ := revision.Reason.WireValue()
			item := map[string]any{"before": revision.Before, "reason": wire, "evidence": revision.Evidence}
			if revision.Action == writer.ActionDelete {
				item["delete"] = true
			} else {
				item["after"] = revision.After
			}
			revisions[i] = item
		}
		fields["revisions"] = revisions
	}
	return fields
}

// marshalFields encodes a field map or fails the test.
func marshalFields(t *testing.T, fields map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal fields: %v", err)
	}
	return data
}

// requireMalformed checks that Parse rejected data with ErrMalformed and
// writer.ErrMalformedOutput, and that the message carries none of the marks.
func requireMalformed(t *testing.T, data []byte) {
	t.Helper()
	rec, err := Parse(data)
	if err == nil {
		t.Fatalf("Parse(%s) = %+v, want an error", data, rec)
	}
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("Parse error = %v, want ErrMalformed", err)
	}
	if !errors.Is(err, writer.ErrMalformedOutput) {
		t.Errorf("Parse error = %v, want it to wrap writer.ErrMalformedOutput", err)
	}
	for _, mark := range []string{markArtifactBody, markArtifactRev} {
		if strings.Contains(err.Error(), mark) {
			t.Errorf("Parse error %q contains the artifact value marked %q", err, mark)
		}
	}
}

func TestMarshalParseRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		rec  writer.StageRecord
	}{
		{"generation", validGeneration()},
		{"review", validReview()},
		{"review with no revisions", func() writer.StageRecord {
			rec := validReview()
			rec.Revisions = nil
			rec.ID = recordID(rec)
			return rec
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := mustMarshal(t, tc.rec)
			got := mustParse(t, data)
			if got.Stage != tc.rec.Stage || got.Title != tc.rec.Title || got.Body != tc.rec.Body {
				t.Errorf("Parse(Marshal(record)) = %+v, want %+v", got, tc.rec)
			}
			if got.Model != tc.rec.Model || got.ModelVersion != tc.rec.ModelVersion {
				t.Errorf("model = %q/%q, want %q/%q", got.Model, got.ModelVersion, tc.rec.Model, tc.rec.ModelVersion)
			}
			if got.System != tc.rec.System || got.User != tc.rec.User {
				t.Errorf("identities = %+v/%+v, want %+v/%+v", got.System, got.User, tc.rec.System, tc.rec.User)
			}
			if got.Input != tc.rec.Input || got.ID != tc.rec.ID {
				t.Errorf("input/id = %q/%q, want %q/%q", got.Input, got.ID, tc.rec.Input, tc.rec.ID)
			}
			if len(got.Revisions) != len(tc.rec.Revisions) {
				t.Fatalf("revisions = %+v, want %+v", got.Revisions, tc.rec.Revisions)
			}
			for i := range got.Revisions {
				if got.Revisions[i] != tc.rec.Revisions[i] {
					t.Errorf("revisions[%d] = %+v, want %+v", i, got.Revisions[i], tc.rec.Revisions[i])
				}
			}
			// The id is a function of the content, so it survives the trip.
			if got.ID == "" || got.ID != recordID(got) {
				t.Errorf("id = %q, want the content identifier %q", got.ID, recordID(got))
			}
		})
	}
}

func TestParseRejectsRevisionStageMismatch(t *testing.T) {
	// A generation record carrying revisions is rejected, and a review record
	// with no revisions key is rejected.
	t.Run("generation with revisions", func(t *testing.T) {
		fields := fieldsFrom(validGeneration())
		fields["revisions"] = []any{}
		requireMalformed(t, marshalFields(t, fields))
	})
	t.Run("review without revisions", func(t *testing.T) {
		fields := fieldsFrom(validReview())
		delete(fields, "revisions")
		requireMalformed(t, marshalFields(t, fields))
	})
}

func TestParseRecordRejects(t *testing.T) {
	generation := fieldsFrom(validGeneration())
	review := fieldsFrom(validReview())
	cases := []struct {
		name string
		data func(t *testing.T) []byte
	}{
		{"empty", func(_ *testing.T) []byte { return nil }},
		{"not JSON", func(_ *testing.T) []byte { return []byte("not json") }},
		{"not an object", func(_ *testing.T) []byte { return []byte(`[1]`) }},
		{"trailing data", func(t *testing.T) []byte {
			return append(mustMarshal(t, validGeneration()), 'x')
		}},
		{"invalid UTF-8", func(_ *testing.T) []byte {
			return []byte("{\"title\":\"\xff\"}")
		}},
		{"unpaired surrogate", func(_ *testing.T) []byte {
			return []byte(`{"title":"\ud800"}`)
		}},
		{"unknown key", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["extra"] = 1 })
			return marshalFields(t, fields)
		}},
		{"duplicate key", func(t *testing.T) []byte {
			valid := mustMarshal(t, validGeneration())
			return []byte(strings.Replace(string(valid), `"title":"Title"`, `"title":"Title","title":"Title"`, 1))
		}},
		{"unsupported version", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["version"] = float64(2) })
			return marshalFields(t, fields)
		}},
		{"version not a number", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["version"] = "1" })
			return marshalFields(t, fields)
		}},
		{"missing version", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "version") })
			return marshalFields(t, fields)
		}},
		{"unknown stage", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["stage"] = "other" })
			return marshalFields(t, fields)
		}},
		{"missing stage", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "stage") })
			return marshalFields(t, fields)
		}},
		{"missing model", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "model") })
			return marshalFields(t, fields)
		}},
		{"empty model", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["model"] = "" })
			return marshalFields(t, fields)
		}},
		{"model with control character", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["model"] = "m\x1b" })
			return marshalFields(t, fields)
		}},
		{"missing model_version", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "model_version") })
			return marshalFields(t, fields)
		}},
		{"model_version not a string", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["model_version"] = 1 })
			return marshalFields(t, fields)
		}},
		{"model_version with control character", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["model_version"] = "v\x00" })
			return marshalFields(t, fields)
		}},
		{"missing system", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "system") })
			return marshalFields(t, fields)
		}},
		{"system not an object", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["system"] = "x" })
			return marshalFields(t, fields)
		}},
		{"system unknown key", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) {
				f["system"] = map[string]any{"embedded": true, "digest": writer.Digest("x"), "extra": 1}
			})
			return marshalFields(t, fields)
		}},
		{"embedded not a boolean", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) {
				f["system"] = map[string]any{"embedded": "true", "digest": writer.Digest("x")}
			})
			return marshalFields(t, fields)
		}},
		{"digest not hex", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) {
				f["system"] = map[string]any{"embedded": true, "digest": "not-a-digest"}
			})
			return marshalFields(t, fields)
		}},
		{"input not hex", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["input"] = "x" })
			return marshalFields(t, fields)
		}},
		{"id not hex", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["id"] = "x" })
			return marshalFields(t, fields)
		}},
		{"id does not match the content", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["id"] = writer.Digest("other") })
			return marshalFields(t, fields)
		}},
		{"empty title", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["title"] = "" })
			return marshalFields(t, fields)
		}},
		{"missing body", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "body") })
			return marshalFields(t, fields)
		}},
		{"body starts with a byte order mark", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["body"] = "\uFEFF" + markArtifactBody })
			return marshalFields(t, fields)
		}},
		{"body has a carriage return", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["body"] = markArtifactBody + "\r\n" })
			return marshalFields(t, fields)
		}},
		{"title has leading whitespace", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["title"] = " Title" })
			return marshalFields(t, fields)
		}},
		{"title has a line feed", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["title"] = "Ti\ntle" })
			return marshalFields(t, fields)
		}},
		{"review revision unknown key", func(t *testing.T) []byte {
			fields := mutate(t, review, func(f map[string]any) {
				rev := f["revisions"].([]any)[0].(map[string]any)
				rev["extra"] = 1
			})
			return marshalFields(t, fields)
		}},
		{"review revision delete false", func(t *testing.T) []byte {
			fields := mutate(t, review, func(f map[string]any) {
				rev := f["revisions"].([]any)[0].(map[string]any)
				delete(rev, "after")
				rev["delete"] = false
			})
			return marshalFields(t, fields)
		}},
		{"review revision unknown reason", func(t *testing.T) []byte {
			fields := mutate(t, review, func(f map[string]any) {
				rev := f["revisions"].([]any)[1].(map[string]any)
				rev["reason"] = "made_up"
			})
			return marshalFields(t, fields)
		}},
		{"review revision empty before", func(t *testing.T) []byte {
			fields := mutate(t, review, func(f map[string]any) {
				rev := f["revisions"].([]any)[0].(map[string]any)
				rev["before"] = ""
			})
			return marshalFields(t, fields)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireMalformed(t, tc.data(t))
		})
	}
}

// mutate deep-copies a field map and applies change to the copy, so a case
// never mutates a shared base through a nested value.
func mutate(t *testing.T, base map[string]any, change func(map[string]any)) map[string]any {
	t.Helper()
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("copy fields: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("copy fields: %v", err)
	}
	change(out)
	return out
}

func TestParseRecordTextRules(t *testing.T) {
	t.Run("valid text is accepted", func(t *testing.T) {
		mustParse(t, mustMarshal(t, validGeneration()))
	})
	t.Run("body is whitespace only", func(t *testing.T) {
		fields := mutate(t, fieldsFrom(validGeneration()), func(f map[string]any) { f["body"] = " \t " })
		requireMalformed(t, marshalFields(t, fields))
	})
	t.Run("body is raw HTML", func(t *testing.T) {
		fields := mutate(t, fieldsFrom(validGeneration()), func(f map[string]any) { f["body"] = "<div>x</div>" })
		requireMalformed(t, marshalFields(t, fields))
	})
}

func TestParseRecordLimits(t *testing.T) {
	// The text cap: a text of exactly maxTextBytes is accepted, one byte more
	// is rejected.
	t.Run("text at the limit", func(t *testing.T) {
		rec := validGeneration()
		rec.Title = "T"
		rec.Body = strings.Repeat("a", testMaxTextBytes-len("# ")-len(rec.Title)-1)
		rec.ID = recordID(rec)
		got := mustParse(t, mustMarshal(t, rec))
		if len(textLength(got)) != testMaxTextBytes {
			t.Fatalf("text length = %d, want %d", len(textLength(got)), testMaxTextBytes)
		}
	})
	t.Run("text over the limit", func(t *testing.T) {
		rec := validGeneration()
		rec.Title = "T"
		rec.Body = strings.Repeat("a", testMaxTextBytes-len("# ")-len(rec.Title))
		rec.ID = recordID(rec)
		requireMalformed(t, mustMarshal(t, rec))
	})

	// The revision cap: exactly testMaxRevisions entries are accepted, one more
	// is rejected.
	t.Run("revisions at the limit", func(t *testing.T) {
		rec := validReview()
		rec.Revisions = make([]writer.Revision, testMaxRevisions)
		for i := range rec.Revisions {
			rec.Revisions[i] = writer.Revision{
				Before:   "b" + strconv.Itoa(i),
				Action:   writer.ActionReplace,
				After:    "a" + strconv.Itoa(i),
				Reason:   writer.ReasonMeaningChange,
				Evidence: markArtifactRev,
			}
		}
		rec.ID = recordID(rec)
		got := mustParse(t, mustMarshal(t, rec))
		if len(got.Revisions) != testMaxRevisions {
			t.Fatalf("revisions = %d, want %d", len(got.Revisions), testMaxRevisions)
		}
	})
	t.Run("revisions over the limit", func(t *testing.T) {
		rec := validReview()
		rec.Revisions = make([]writer.Revision, testMaxRevisions+1)
		for i := range rec.Revisions {
			rec.Revisions[i] = writer.Revision{
				Before:   "b" + strconv.Itoa(i),
				Action:   writer.ActionReplace,
				After:    "a" + strconv.Itoa(i),
				Reason:   writer.ReasonMeaningChange,
				Evidence: markArtifactRev,
			}
		}
		rec.ID = recordID(rec)
		requireMalformed(t, mustMarshal(t, rec))
	})

	// The file cap: an artifact one byte over is rejected before its content
	// is read. A valid artifact never reaches the cap (a 1 MiB text and a
	// 1 MiB list fit well below 4 MiB), so only the over case exists here.
	t.Run("file over the limit", func(t *testing.T) {
		requireMalformed(t, make([]byte, maxArtifactBytes+1))
	})
}

// textLength returns the generated-text form Parse checked for a record, used
// to assert the boundary length.
func textLength(rec writer.StageRecord) string {
	return "# " + rec.Title + "\n" + rec.Body
}
