//go:build test

package artifact

import (
	"encoding/json"
	"errors"
	"maps"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/writer"
)

// Distinct markers used to tell where a value went and that it never reached
// an error message.
const (
	markArtifactBody = "mark-artifact-body-R4kd"
	markArtifactRev  = "mark-artifact-revision-T7wm"
)

// testCallTime is the call time of every record the helpers build. A second
// time lets a test tell two records apart by their call time alone.
var (
	testCallTime  = time.Date(2026, time.October, 16, 1, 0, 0, 0, time.UTC)
	testCallTime2 = time.Date(2027, time.March, 2, 9, 30, 15, 0, time.UTC)
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
		Stage:    writer.StageGeneration,
		Title:    "Title",
		Body:     markArtifactBody,
		Model:    "test-model",
		CallTime: testCallTime,
		System:   testIdentity(true, "system template"),
		User:     testIdentity(false, "user template"),
		Input:    writer.Digest("material"),
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
// holds a slice, so it is not comparable with ==. Two unknown usages are equal
// however they became unknown: the reason is not stored.
func sameRecord(a, b writer.StageRecord) bool {
	if a.Stage != b.Stage || a.Title != b.Title || a.Body != b.Body ||
		a.Model != b.Model || a.ModelVersion != b.ModelVersion ||
		!a.CallTime.Equal(b.CallTime) ||
		a.System != b.System || a.User != b.User || a.Input != b.Input || a.ID != b.ID {
		return false
	}
	if !sameUsage(a.Usage, b.Usage) {
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

// sameUsage reports whether two usages are equal: both unknown, or both known
// with the same counts.
func sameUsage(a, b llm.Usage) bool {
	aCounts, aKnown := a.Counts()
	bCounts, bKnown := b.Counts()
	if aKnown != bKnown {
		return false
	}
	if !aKnown {
		return true
	}
	return aCounts == bCounts
}

// mustMarshal marshals a record or fails the test.
func mustMarshal(t *testing.T, rec writer.StageRecord) []byte {
	t.Helper()
	data, _, err := Marshal(rec)
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
		"usage":         usageFields(rec.Usage),
		"call_time":     rec.CallTime.Format(CallTimeLayout),
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

// usageFields builds the wire form of a usage as a field-map value: nil for an
// unknown usage, an object of the four counts for a known one.
func usageFields(usage llm.Usage) any {
	counts, known := usage.Counts()
	if !known {
		return nil
	}
	return map[string]any{
		"input":       float64(counts.Input),
		"cache_read":  float64(counts.CacheRead),
		"cache_write": float64(counts.CacheWrite),
		"output":      float64(counts.Output),
	}
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
// writer.ErrMalformedOutput, that the message carries none of the marks, and
// returns the error for further checks.
func requireMalformed(t *testing.T, data []byte) error {
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
	return err
}

// requireMalformedUsage is requireMalformed for a rejection the usage rule
// itself must produce: the error also names usage.
func requireMalformedUsage(t *testing.T, data []byte) {
	t.Helper()
	err := requireMalformed(t, data)
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("Parse error %q does not name usage", err)
	}
}

func TestMarshalParseRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		rec  writer.StageRecord
	}{
		{"generation", validGeneration()},
		{"generation with another call time", func() writer.StageRecord {
			rec := validGeneration()
			rec.CallTime = testCallTime2
			rec.ID = recordID(rec)
			return rec
		}()},
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
		{"generation with a known usage", func() writer.StageRecord {
			rec := validGeneration()
			rec.Usage = llm.KnownUsage(llm.TokenCounts{Input: 46, CacheRead: 7, CacheWrite: 0, Output: 85})
			rec.ID = recordID(rec)
			return rec
		}()},
		{"generation with an unknown usage", func() writer.StageRecord {
			rec := validGeneration()
			rec.Usage = llm.UnknownUsage(llm.RuleNotRead, "")
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
			fields := mutate(t, generation, func(f map[string]any) { f["version"] = float64(artifactVersion + 1) })
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
	if _, _, err := Marshal(rec); !errors.Is(err, ErrMalformed) {
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
	data, _, err := Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	got := mustParse(t, data)
	if got.Body != rec.Body {
		t.Errorf("body round trip = %d bytes, want %d", len(got.Body), len(rec.Body))
	}
}

// usageMap returns a fresh copy of a known-usage field map, so a case can
// change one member without touching the others.
func usageMap(counts map[string]any) map[string]any {
	return maps.Clone(counts)
}

// validUsageFields is an all-counts-known usage in the field-map form.
var validUsageFields = map[string]any{"input": 1, "cache_read": 2, "cache_write": 3, "output": 4}

func TestParseRecordRejectsUsage(t *testing.T) {
	generation := fieldsFrom(validGeneration())
	members := []string{"input", "cache_read", "cache_write", "output"}
	invalidValues := []struct {
		name  string
		value any
	}{
		{"negative", json.RawMessage("-1")},
		{"negative zero", json.RawMessage("-0")},
		{"fraction", json.RawMessage("1.5")},
		{"exponent", json.RawMessage("1e3")},
		{"out of int64", json.RawMessage("9223372036854775808")},
		{"string", "x"},
		{"boolean", true},
		{"null", nil},
		{"array", []any{}},
		{"object", map[string]any{}},
	}

	t.Run("usage member", func(t *testing.T) {
		cases := []struct {
			name  string
			named bool // whether the rejection must name usage
			data  func(t *testing.T) []byte
		}{
			{"missing", true, func(t *testing.T) []byte {
				fields := mutate(t, generation, func(f map[string]any) { delete(f, "usage") })
				return marshalFields(t, fields)
			}},
			{"duplicate", false, func(t *testing.T) []byte {
				// A duplicate top-level member is rejected by the artifact
				// key check, which never names the untrusted key.
				data := mustMarshal(t, validGeneration()) // the zero usage writes null
				return []byte(strings.Replace(string(data), `"usage":null`, `"usage":null,"usage":null`, 1))
			}},
			{"a string", true, func(t *testing.T) []byte {
				fields := mutate(t, generation, func(f map[string]any) { f["usage"] = "x" })
				return marshalFields(t, fields)
			}},
			{"a number", true, func(t *testing.T) []byte {
				fields := mutate(t, generation, func(f map[string]any) { f["usage"] = 1 })
				return marshalFields(t, fields)
			}},
			{"a boolean", true, func(t *testing.T) []byte {
				fields := mutate(t, generation, func(f map[string]any) { f["usage"] = true })
				return marshalFields(t, fields)
			}},
			{"an array", true, func(t *testing.T) []byte {
				fields := mutate(t, generation, func(f map[string]any) { f["usage"] = []any{} })
				return marshalFields(t, fields)
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.named {
					requireMalformedUsage(t, tc.data(t))
					return
				}
				requireMalformed(t, tc.data(t))
			})
		}
	})

	t.Run("usage object keys", func(t *testing.T) {
		t.Run("unlisted key", func(t *testing.T) {
			fields := mutate(t, generation, func(f map[string]any) {
				counts := usageMap(validUsageFields)
				counts["extra"] = 1
				f["usage"] = counts
			})
			requireMalformedUsage(t, marshalFields(t, fields))
		})
		t.Run("duplicate key", func(t *testing.T) {
			rec := validGeneration()
			rec.Usage = llm.KnownUsage(llm.TokenCounts{Input: 1, CacheRead: 2, CacheWrite: 3, Output: 4})
			rec.ID = recordID(rec)
			data := mustMarshal(t, rec)
			requireMalformedUsage(t, []byte(strings.Replace(string(data), `"input":1`, `"input":1,"input":1`, 1)))
		})
		for _, member := range members {
			t.Run(member+" missing", func(t *testing.T) {
				fields := mutate(t, generation, func(f map[string]any) {
					counts := usageMap(validUsageFields)
					delete(counts, member)
					f["usage"] = counts
				})
				requireMalformedUsage(t, marshalFields(t, fields))
			})
			for _, invalid := range invalidValues {
				t.Run(member+" "+invalid.name, func(t *testing.T) {
					fields := mutate(t, generation, func(f map[string]any) {
						counts := usageMap(validUsageFields)
						counts[member] = invalid.value
						f["usage"] = counts
					})
					requireMalformedUsage(t, marshalFields(t, fields))
				})
			}
		}
	})

	t.Run("the message carries no value", func(t *testing.T) {
		const marker = "usage-value-marker-Q7z"
		fields := mutate(t, generation, func(f map[string]any) {
			counts := usageMap(validUsageFields)
			counts["input"] = marker
			f["usage"] = counts
		})
		if err := requireMalformed(t, marshalFields(t, fields)); strings.Contains(err.Error(), marker) {
			t.Errorf("Parse error %q contains the injected string value", err)
		}

		fields = mutate(t, generation, func(f map[string]any) {
			counts := usageMap(validUsageFields)
			counts["output"] = json.RawMessage("9223372036854775808")
			f["usage"] = counts
		})
		if err := requireMalformed(t, marshalFields(t, fields)); strings.Contains(err.Error(), "9223372036854775808") {
			t.Errorf("Parse error %q contains the injected number", err)
		}
	})
}

func TestParseRejectsVersion1(t *testing.T) {
	// A version 1 artifact has no usage element. The version check rejects it
	// before the usage or id checks, with the recovery step.
	fields := fieldsFrom(validGeneration())
	fields["version"] = float64(1)
	delete(fields, "usage")
	delete(fields, "call_time")
	err := requireMalformed(t, marshalFields(t, fields))
	for _, want := range []string{"unsupported artifact version 1", "version 3", "new --artifacts directory"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Parse error = %v, want it to contain %q", err, want)
		}
	}
}

func TestParseRejectsVersion2(t *testing.T) {
	// A version 2 artifact has a usage element but no call_time. The version
	// check rejects it as a version difference, not as a missing call_time.
	fields := fieldsFrom(validGeneration())
	fields["version"] = float64(2)
	delete(fields, "call_time")
	err := requireMalformed(t, marshalFields(t, fields))
	for _, want := range []string{"unsupported artifact version 2", "version 3", "new --artifacts directory"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Parse error = %v, want it to contain %q", err, want)
		}
	}
}

func TestParseRecordRejectsCallTime(t *testing.T) {
	// Each case breaks only the call_time member. For a value time.Parse
	// accepts (a fractional second, a one-digit hour, the zero time), the id
	// is recomputed from the parsed value, so that if the call-time rule were
	// removed the id check would still match and the artifact would load: the
	// case proves the call-time rule rejects the input, not the id rule.
	baseFields := func(t *testing.T, raw string) map[string]any {
		t.Helper()
		fields := fieldsFrom(validGeneration())
		fields["call_time"] = raw
		if parsed, err := time.Parse(CallTimeLayout, raw); err == nil {
			rec := validGeneration()
			rec.CallTime = parsed
			fields["id"] = recordID(rec)
		}
		return fields
	}
	cases := []struct {
		name string
		data func(t *testing.T) []byte
	}{
		{"missing", func(t *testing.T) []byte {
			fields := fieldsFrom(validGeneration())
			delete(fields, "call_time")
			return marshalFields(t, fields)
		}},
		{"duplicate", func(t *testing.T) []byte {
			// A duplicate key is rejected by the artifact key check, which
			// never names the untrusted value.
			data := mustMarshal(t, validGeneration())
			return []byte(strings.Replace(string(data), `"call_time":`, `"call_time":"x","call_time":`, 1))
		}},
		{"not a string", func(t *testing.T) []byte {
			fields := fieldsFrom(validGeneration())
			fields["call_time"] = 1
			return marshalFields(t, fields)
		}},
		{"null", func(t *testing.T) []byte {
			fields := fieldsFrom(validGeneration())
			fields["call_time"] = nil
			return marshalFields(t, fields)
		}},
		{"unpaired surrogate", func(t *testing.T) []byte {
			fields := fieldsFrom(validGeneration())
			fields["call_time"] = json.RawMessage(`"\ud800"`)
			return marshalFields(t, fields)
		}},
		{"offset", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T01:00:00+00:00")) }},
		{"offset east", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T10:00:00+09:00")) }},
		{"lowercase z", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T01:00:00z")) }},
		{"fractional second", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T01:00:00.5Z")) }},
		{"missing second", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T01:00Z")) }},
		{"one-digit hour", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T1:00:00Z")) }},
		{"leading space", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, " 2026-10-16T01:00:00Z")) }},
		{"trailing space", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T01:00:00Z ")) }},
		{"space separator", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16 01:00:00Z")) }},
		{"lowercase t", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16t01:00:00Z")) }},
		{"day out of range", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-02-30T00:00:00Z")) }},
		{"hour out of range", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T24:00:00Z")) }},
		{"leap second", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "2026-10-16T23:59:60Z")) }},
		{"zero time", func(t *testing.T) []byte { return marshalFields(t, baseFields(t, "0001-01-01T00:00:00Z")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireMalformed(t, tc.data(t))
		})
	}
}

func TestRecordIDCoversCallTime(t *testing.T) {
	first := validGeneration()
	second := validGeneration()
	second.CallTime = testCallTime2
	second.ID = recordID(second)
	if first.ID == second.ID {
		t.Error("records differing only in the call time share an id")
	}

	t.Run("a rewritten call time is rejected", func(t *testing.T) {
		// A canonical time of the right shape, so only the id check can reject
		// the file.
		data := mustMarshal(t, first)
		rewritten := strings.Replace(string(data),
			first.CallTime.Format(CallTimeLayout),
			testCallTime2.Format(CallTimeLayout), 1)
		requireMalformed(t, []byte(rewritten))
	})
}

func TestMarshalRejectsNonCanonicalCallTime(t *testing.T) {
	cases := []struct {
		name     string
		callTime time.Time
	}{
		{"zero", time.Time{}},
		{"not UTC", testCallTime.In(time.FixedZone("JST", 9*60*60))},
		{"fractional second", testCallTime.Add(500 * time.Millisecond)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := validGeneration()
			rec.CallTime = tc.callTime
			rec.ID = recordID(rec)
			if _, _, err := Marshal(rec); !errors.Is(err, ErrMalformed) {
				t.Fatalf("Marshal error = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestRecordIDCoversUsage(t *testing.T) {
	base := validGeneration()
	first := base
	first.Usage = llm.KnownUsage(llm.TokenCounts{Input: 1, CacheRead: 0, CacheWrite: 0, Output: 2})
	first.ID = recordID(first)
	second := base
	second.Usage = llm.KnownUsage(llm.TokenCounts{Input: 1, CacheRead: 0, CacheWrite: 0, Output: 3})
	second.ID = recordID(second)
	if first.ID == second.ID {
		t.Error("records differing only in the output token count share an id")
	}

	t.Run("a rewritten token count is rejected", func(t *testing.T) {
		data := mustMarshal(t, first)
		rewritten := strings.Replace(string(data), `"output":2`, `"output":3`, 1)
		requireMalformed(t, []byte(rewritten))
	})

	t.Run("different unknown reasons share an id", func(t *testing.T) {
		a := base
		a.Usage = llm.UnknownUsage(llm.RuleNotRead, "")
		a.ID = recordID(a)
		b := base
		b.Usage = llm.UnknownUsage(llm.RuleRecordedUnknown, "completion_tokens")
		b.ID = recordID(b)
		if a.ID != b.ID {
			t.Error("unknown usages with different reasons have different ids")
		}
	})
}
