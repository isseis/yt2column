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

// validGeneration returns a generation record that passes Parse. The body
// carries a mark, so a rejection's error message can be checked for it.
func validGeneration() writer.StageRecord {
	rec := writer.StageRecord{
		Stage:  writer.StageGeneration,
		Title:  "Title",
		Body:   markArtifactBody,
		Model:  "test-model",
		System: testIdentity(true, "system template"),
		User:   testIdentity(false, "user template"),
		Input:  writer.Digest("material"),
	}
	rec.ID = recordID(rec)
	return rec
}

// validReview returns a review record, with a replacement and a deletion, that
// passes Parse. The evidence carries a mark, so a rejection's error message can
// be checked for it.
func validReview() writer.StageRecord {
	rec := validGeneration()
	rec.Stage = writer.StageReview
	rec.Revisions = []writer.Revision{
		{Before: markArtifactBody, Action: writer.ActionReplace, After: markArtifactBody + " revised", Reason: writer.ReasonMeaningChange, Evidence: markArtifactRev},
		{Before: "extra", Action: writer.ActionDelete, Reason: writer.ReasonUnsupported, Evidence: markArtifactRev},
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
		{"generation with a model version", func() writer.StageRecord {
			rec := validGeneration()
			rec.ModelVersion = "2026-10-14"
			rec.ID = recordID(rec)
			return rec
		}()},
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
			got := mustParse(t, mustMarshal(t, tc.rec))
			if !sameRecord(got, tc.rec) {
				t.Errorf("Parse(Marshal(record)) = %+v, want %+v", got, tc.rec)
			}
		})
	}
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
		{"empty model", recordCase(validGeneration(), func(r *writer.StageRecord) { r.Model = "" })},
		{"model with control character", recordCase(validGeneration(), func(r *writer.StageRecord) { r.Model = "m\x1b" })},
		{"missing model_version", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "model_version") })
			return marshalFields(t, fields)
		}},
		{"model_version not a string", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["model_version"] = 1 })
			return marshalFields(t, fields)
		}},
		{"model_version with control character", recordCase(validGeneration(), func(r *writer.StageRecord) { r.ModelVersion = "v\x00" })},
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
		{"input not hex", recordCase(validGeneration(), func(r *writer.StageRecord) { r.Input = "x" })},
		{"id not hex", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["id"] = "x" })
			return marshalFields(t, fields)
		}},
		{"id does not match the content", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { f["id"] = writer.Digest("other") })
			return marshalFields(t, fields)
		}},
		{"empty title", recordCase(validGeneration(), func(r *writer.StageRecord) { r.Title = "" })},
		{"missing body", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "body") })
			return marshalFields(t, fields)
		}},
		{"missing title", func(t *testing.T) []byte {
			fields := mutate(t, generation, func(f map[string]any) { delete(f, "title") })
			return marshalFields(t, fields)
		}},
		{"review revision not an object", func(t *testing.T) []byte {
			fields := mutate(t, review, func(f map[string]any) {
				f["revisions"] = []any{"x"}
			})
			return marshalFields(t, fields)
		}},
		{"review revision duplicate key", func(t *testing.T) []byte {
			valid := mustMarshal(t, validReview())
			return []byte(strings.Replace(string(valid), `"reason":"meaning_change"`, `"reason":"meaning_change","reason":"meaning_change"`, 1))
		}},
		{"system duplicate key", func(t *testing.T) []byte {
			valid := mustMarshal(t, validGeneration())
			return []byte(strings.Replace(string(valid), `"embedded":true`, `"embedded":true,"embedded":true`, 1))
		}},
		{"revisions on a generation", func(t *testing.T) []byte {
			fields := fieldsFrom(validGeneration())
			fields["revisions"] = []any{}
			return marshalFields(t, fields)
		}},
		{"review without revisions", func(t *testing.T) []byte {
			rec := validReview()
			rec.Revisions = nil
			rec.ID = recordID(rec)
			fields := fieldsFrom(rec)
			delete(fields, "revisions")
			return marshalFields(t, fields)
		}},
		{"body starts with a byte order mark", recordCase(validGeneration(), func(r *writer.StageRecord) { r.Body = "\uFEFF" + markArtifactBody })},
		{"body has a carriage return", recordCase(validGeneration(), func(r *writer.StageRecord) { r.Body = markArtifactBody + "\r\n" })},
		{"title has leading whitespace", recordCase(validGeneration(), func(r *writer.StageRecord) { r.Title = " Title" })},
		{"title has a line feed", recordCase(validGeneration(), func(r *writer.StageRecord) { r.Title = "Ti\ntle" })},
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
		{"review revision empty before", recordCase(validReview(), func(r *writer.StageRecord) { r.Revisions[0].Before = "" })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireMalformed(t, tc.data(t))
		})
	}
}

// recordCase builds the wire form of a mutated record with the id recomputed
// from the mutated content, so only the rule under test can reject it. Without
// the recomputation the id check would reject any content change and the case
// could not prove its own rule rejects.
func recordCase(base writer.StageRecord, mutate func(*writer.StageRecord)) func(*testing.T) []byte {
	return func(t *testing.T) []byte {
		mutate(&base)
		base.ID = recordID(base)
		return marshalFields(t, fieldsFrom(base))
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
	t.Run("body is whitespace only", func(t *testing.T) {
		fields := mutate(t, fieldsFrom(validGeneration()), func(f map[string]any) { f["body"] = " \t " })
		requireMalformed(t, marshalFields(t, fields))
	})
	t.Run("body is raw HTML", func(t *testing.T) {
		fields := mutate(t, fieldsFrom(validGeneration()), func(f map[string]any) { f["body"] = "<div>x</div>" })
		requireMalformed(t, marshalFields(t, fields))
	})
}

// revisionsN builds n replacement revisions, for the revision-count boundary.
func revisionsN(n int) []writer.Revision {
	revisions := make([]writer.Revision, n)
	for i := range revisions {
		revisions[i] = writer.Revision{
			Before:   "b" + strconv.Itoa(i),
			Action:   writer.ActionReplace,
			After:    "a" + strconv.Itoa(i),
			Reason:   writer.ReasonMeaningChange,
			Evidence: markArtifactRev,
		}
	}
	return revisions
}

func TestParseRecordLimits(t *testing.T) {
	// The text cap: a text of exactly testMaxTextBytes is accepted, one byte
	// more is rejected.
	t.Run("text at the limit", func(t *testing.T) {
		rec := validGeneration()
		rec.Title = "T"
		rec.Body = strings.Repeat("a", testMaxTextBytes-len("# ")-len(rec.Title)-1)
		rec.ID = recordID(rec)
		got := mustParse(t, mustMarshal(t, rec))
		if want := testMaxTextBytes - len("# ") - len(rec.Title) - 1; len(got.Body) != want {
			t.Fatalf("body length = %d, want %d", len(got.Body), want)
		}
	})
	t.Run("text over the limit", func(t *testing.T) {
		rec := validGeneration()
		rec.Title = "T"
		rec.Body = strings.Repeat("a", testMaxTextBytes-len("# ")-len(rec.Title))
		rec.ID = recordID(rec)
		// Built without Marshal, which now refuses a record Parse rejects.
		requireMalformed(t, marshalFields(t, fieldsFrom(rec)))
	})

	// The revision cap: exactly testMaxRevisions entries are accepted, one more
	// is rejected.
	t.Run("revisions at the limit", func(t *testing.T) {
		rec := validReview()
		rec.Revisions = revisionsN(testMaxRevisions)
		rec.ID = recordID(rec)
		got := mustParse(t, mustMarshal(t, rec))
		if len(got.Revisions) != testMaxRevisions {
			t.Fatalf("revisions = %d, want %d", len(got.Revisions), testMaxRevisions)
		}
	})
	t.Run("revisions over the limit", func(t *testing.T) {
		rec := validReview()
		rec.Revisions = revisionsN(testMaxRevisions + 1)
		rec.ID = recordID(rec)
		// Built without Marshal, which now refuses a record Parse rejects.
		requireMalformed(t, marshalFields(t, fieldsFrom(rec)))
	})

	// The file cap: an artifact one byte over is rejected before its content
	// is read. A valid artifact never reaches the cap (a 1 MiB text and a
	// 1 MiB list fit well below 4 MiB), so only the over case exists here.
	t.Run("file over the limit", func(t *testing.T) {
		requireMalformed(t, make([]byte, maxArtifactBytes+1))
	})
}

func TestMarshalRejectsUnloadableRecord(t *testing.T) {
	// A revision list on a non-review stage is omitted from the file, so the
	// written id (which includes it) would not match the reload. Marshal must
	// refuse it instead of writing a file Load always rejects.
	rec := validGeneration()
	rec.Revisions = []writer.Revision{{
		Before:   "x",
		Action:   writer.ActionReplace,
		After:    "y",
		Reason:   writer.ReasonMeaningChange,
		Evidence: markArtifactRev,
	}}
	rec.ID = recordID(rec)
	if _, err := Marshal(rec); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Marshal error = %v, want ErrMalformed", err)
	}
}

func TestMarshalKeepsEscapingHeavyBodyWithinTheLimit(t *testing.T) {
	// A body at the text cap of "&" would expand six-fold under HTML escaping
	// and exceed the file cap; Marshal must not escape it, so the artifact it
	// writes still round-trips through Parse (and therefore through Load).
	rec := validGeneration()
	rec.Title = "T"
	rec.Body = strings.Repeat("&", testMaxTextBytes-len("# ")-len(rec.Title)-1)
	rec.ID = recordID(rec)
	data, err := Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	got := mustParse(t, data)
	if got.Body != rec.Body {
		t.Errorf("body round trip = %d bytes, want %d", len(got.Body), len(rec.Body))
	}
}
