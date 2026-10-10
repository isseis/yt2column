//go:build test

package writer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// reviewResponse wraps revision objects in the response envelope.
func reviewResponse(items ...string) string {
	return `{"revisions":[` + strings.Join(items, ",") + `]}`
}

// replaceItem builds one revision object that replaces before with after.
func replaceItem(before, after, reason, evidence string) string {
	return `{"before":` + jsonString(before) +
		`,"after":` + jsonString(after) +
		`,"reason":` + jsonString(reason) +
		`,"evidence":` + jsonString(evidence) + `}`
}

// deleteItem builds one revision object that deletes before.
func deleteItem(before, reason, evidence string) string {
	return `{"before":` + jsonString(before) +
		`,"delete":true` +
		`,"reason":` + jsonString(reason) +
		`,"evidence":` + jsonString(evidence) + `}`
}

// jsonString returns s as a JSON string literal. It is hand-rolled because the
// values here only need the escapes a JSON string uses.
func jsonString(s string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&builder, `\u%04x`, r)
				continue
			}
			builder.WriteRune(r)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}

func TestParseReviewResponse(t *testing.T) {
	t.Run("accepts an empty list", func(t *testing.T) {
		revisions, err := parseReviewResponse(reviewResponse())
		if err != nil {
			t.Fatalf("parseReviewResponse() error = %v, want nil", err)
		}
		if len(revisions) != 0 {
			t.Errorf("parseReviewResponse() returned %d revisions, want 0", len(revisions))
		}
	})

	t.Run("accepts a replacement in order", func(t *testing.T) {
		text := reviewResponse(
			replaceItem(markRevisionBefore, markRevisionAfter, "meaning_change", markRevisionEvidence),
			deleteItem(markRevisionBefore+"-2", "unsupported", markRevisionEvidence+"-2"),
		)
		revisions, err := parseReviewResponse(text)
		if err != nil {
			t.Fatalf("parseReviewResponse() error = %v, want nil", err)
		}
		want := []Revision{
			{Before: markRevisionBefore, Action: ActionReplace, After: markRevisionAfter, Reason: ReasonMeaningChange, Evidence: markRevisionEvidence},
			{Before: markRevisionBefore + "-2", Action: ActionDelete, Reason: ReasonUnsupported, Evidence: markRevisionEvidence + "-2"},
		}
		if len(revisions) != len(want) {
			t.Fatalf("parseReviewResponse() returned %d revisions, want %d", len(revisions), len(want))
		}
		for i := range want {
			if revisions[i] != want[i] {
				t.Errorf("revisions[%d] = %+v, want %+v", i, revisions[i], want[i])
			}
		}
	})

	t.Run("accepts a line feed in evidence", func(t *testing.T) {
		text := reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter, "speaker", "first\nsecond"))
		if _, err := parseReviewResponse(text); err != nil {
			t.Errorf("parseReviewResponse() error = %v, want nil", err)
		}
	})

	t.Run("accepts trailing whitespace", func(t *testing.T) {
		if _, err := parseReviewResponse(reviewResponse() + "\n\t "); err != nil {
			t.Errorf("parseReviewResponse() error = %v, want nil", err)
		}
	})

	t.Run("accepts the item limit", func(t *testing.T) {
		items := make([]string, maxRevisions)
		for i := range items {
			items[i] = replaceItem(fmt.Sprintf("b%d", i), fmt.Sprintf("a%d", i), "unsupported", fmt.Sprintf("e%d", i))
		}
		revisions, err := parseReviewResponse(reviewResponse(items...))
		if err != nil {
			t.Fatalf("parseReviewResponse() error = %v, want nil", err)
		}
		if len(revisions) != maxRevisions {
			t.Errorf("parseReviewResponse() returned %d revisions, want %d", len(revisions), maxRevisions)
		}
	})

	t.Run("accepts the byte limit", func(t *testing.T) {
		text := reviewResponse() + strings.Repeat(" ", maxTextBytes-len(reviewResponse()))
		if len(text) != maxTextBytes {
			t.Fatalf("test text length = %d, want %d", len(text), maxTextBytes)
		}
		if _, err := parseReviewResponse(text); err != nil {
			t.Errorf("parseReviewResponse() error = %v, want nil", err)
		}
	})

	tooMany := make([]string, maxRevisions+1)
	for i := range tooMany {
		tooMany[i] = replaceItem(fmt.Sprintf("b%d", i), fmt.Sprintf("a%d", i), "unsupported", fmt.Sprintf("e%d", i))
	}

	rejected := []struct {
		name string
		text string
	}{
		{"no revisions key", `{}`},
		{"top level is an array", `[]`},
		{"revisions is not an array", `{"revisions":{}}`},
		{"revisions is null", `{"revisions":null}`},
		{"element is not an object", `{"revisions":["x"]}`},
		{"element is null", `{"revisions":[null]}`},
		{"before is missing", `{"revisions":[{"after":"a","reason":"unsupported","evidence":"e"}]}`},
		{"before is empty", reviewResponse(replaceItem("", markRevisionAfter, "unsupported", markRevisionEvidence))},
		{"before is not a string", `{"revisions":[{"before":1,"after":"a","reason":"unsupported","evidence":"e"}]}`},
		{"before has a line feed", reviewResponse(replaceItem(markRevisionBefore+"\n"+markRevisionAfter, markRevisionAfter, "unsupported", markRevisionEvidence))},
		{"before has a carriage return", reviewResponse(replaceItem(markRevisionBefore+"\r"+markRevisionAfter, markRevisionAfter, "unsupported", markRevisionEvidence))},
		{"before has a control character", reviewResponse(replaceItem(markRevisionBefore+"\x01", markRevisionAfter, "unsupported", markRevisionEvidence))},
		{"both after and delete", `{"revisions":[{"before":` + jsonString(markRevisionBefore) + `,"after":` + jsonString(markRevisionAfter) + `,"delete":true,"reason":"unsupported","evidence":` + jsonString(markRevisionEvidence) + `}]}`},
		{"neither after nor delete", `{"revisions":[{"before":` + jsonString(markRevisionBefore) + `,"reason":"unsupported","evidence":` + jsonString(markRevisionEvidence) + `}]}`},
		{"delete is false", `{"revisions":[{"before":` + jsonString(markRevisionBefore) + `,"delete":false,"reason":"unsupported","evidence":` + jsonString(markRevisionEvidence) + `}]}`},
		{"delete is null", reviewResponse(`{"before":` + jsonString(markRevisionBefore) + `,"delete":null,"reason":"unsupported","evidence":` + jsonString(markRevisionEvidence) + `}`)},
		{"delete is not a boolean", `{"revisions":[{"before":` + jsonString(markRevisionBefore) + `,"delete":"true","reason":"unsupported","evidence":` + jsonString(markRevisionEvidence) + `}]}`},
		{"after is empty", reviewResponse(replaceItem(markRevisionBefore, "", "unsupported", markRevisionEvidence))},
		{"after is not a string", reviewResponse(`{"before":` + jsonString(markRevisionBefore) + `,"after":1,"reason":"unsupported","evidence":` + jsonString(markRevisionEvidence) + `}`)},
		{"after has a line feed", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter+"\n", "unsupported", markRevisionEvidence))},
		{"after has a control character", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter+"\x1b", "unsupported", markRevisionEvidence))},
		{"reason is missing", `{"revisions":[{"before":"b","after":"a","evidence":"e"}]}`},
		{"reason is unknown", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter, "unknown", markRevisionEvidence))},
		{"reason differs in case", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter, "Meaning_Change", markRevisionEvidence))},
		{"reason has surrounding spaces", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter, " unsupported ", markRevisionEvidence))},
		{"reason is not a string", `{"revisions":[{"before":"b","after":"a","reason":1,"evidence":"e"}]}`},
		{"evidence is missing", `{"revisions":[{"before":"b","after":"a","reason":"unsupported"}]}`},
		{"evidence is empty", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter, "unsupported", ""))},
		{"evidence is whitespace only", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter, "unsupported", " \t "))},
		{"evidence has a carriage return", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter, "unsupported", markRevisionEvidence+"\r"))},
		{"evidence has a control character", reviewResponse(replaceItem(markRevisionBefore, markRevisionAfter, "unsupported", markRevisionEvidence+"\x01"))},
		{"unknown item key", `{"revisions":[{"before":"b","after":"a","reason":"unsupported","evidence":"e","extra":1}]}`},
		{"duplicate item key", `{"revisions":[{"before":"b","before":"b2","after":"a","reason":"unsupported","evidence":"e"}]}`},
		{"unknown top level key", `{"revisions":[],"extra":1}`},
		{"duplicate top level key", `{"revisions":[],"revisions":[]}`},
		{"trailing content", reviewResponse() + " x"},
		{"code fence", "```json\n" + reviewResponse() + "\n```"},
		{"invalid UTF-8", reviewResponse() + "\xff"},
		{"unpaired surrogate", `{"revisions":[{"before":"\ud800","after":"a","reason":"unsupported","evidence":"e"}]}`},
		{"too many items", reviewResponse(tooMany...)},
		{"over the byte limit", reviewResponse() + strings.Repeat(" ", maxTextBytes)},
		{"not JSON", "not json"},
	}
	for _, tc := range rejected {
		t.Run("rejects/"+tc.name, func(t *testing.T) {
			revisions, err := parseReviewResponse(tc.text)
			if !errors.Is(err, ErrMalformedOutput) {
				t.Fatalf("parseReviewResponse() error = %v, want ErrMalformedOutput", err)
			}
			if revisions != nil {
				t.Errorf("parseReviewResponse() revisions = %+v, want nil", revisions)
			}
			for _, mark := range []string{markRevisionBefore, markRevisionAfter, markRevisionEvidence} {
				if strings.Contains(err.Error(), mark) {
					t.Errorf("parseReviewResponse() error %q contains the value marked %q", err, mark)
				}
			}
		})
	}
}

func TestApplyRevisions(t *testing.T) {
	const (
		jia  = "\u7532" // U+7532
		yi   = "\u4e59" // U+4E59
		bing = "\u4e19" // U+4E19
	)
	replace := func(before, after string) Revision {
		return Revision{Before: before, Action: ActionReplace, After: after, Reason: ReasonMeaningChange, Evidence: "e"}
	}
	remove := func(before string) Revision {
		return Revision{Before: before, Action: ActionDelete, Reason: ReasonMeaningChange, Evidence: "e"}
	}

	accepted := []struct {
		name      string
		text      string
		revisions []Revision
		want      string
	}{
		{"requirement example", "# T\n" + jia + yi, []Revision{replace(jia, yi), replace(yi, bing)}, "# T\n" + yi + bing},
		{"order does not matter", "# T\n" + jia + yi, []Revision{replace(yi, bing), replace(jia, yi)}, "# T\n" + yi + bing},
		{"deletion removes the text", "# T\n" + jia + yi, []Revision{remove(jia)}, "# T\n" + yi},
		{"touching ranges are accepted", "# T\nab", []Revision{replace("a", "x"), replace("b", "y")}, "# T\nxy"},
		{"title line replacement", "# T\nbody", []Revision{replace("T", "U")}, "# U\nbody"},
		{"empty revisions keep the text", "# T\nbody", nil, "# T\nbody"},
	}
	for _, tc := range accepted {
		t.Run("accepts/"+tc.name, func(t *testing.T) {
			got, err := applyRevisions(tc.text, tc.revisions)
			if err != nil {
				t.Fatalf("applyRevisions() error = %v, want nil", err)
			}
			if got != tc.want {
				t.Errorf("applyRevisions() = %q, want %q", got, tc.want)
			}
		})
	}

	rejected := []struct {
		name      string
		text      string
		revisions []Revision
	}{
		{"no occurrence", "# T\nbody", []Revision{replace("z", "y")}},
		{"one byte overlap", "# T\nabc", []Revision{replace("ab", "x"), replace("bc", "y")}},
		{"same before twice", "# T\nab", []Revision{replace("a", "x"), replace("a", "y")}},
		{"before inside a heading", "# T\n## H\nbody", []Revision{replace("H", "X")}},
		{"replacement removes a heading", "# T\n## H\nbody", []Revision{replace("## H", "X")}},
		{"before creates a heading", "# T\nbody", []Revision{replace("body", "## body")}},
		{"before changes a fenced heading line", "# T\n```\n## x\n```\nend", []Revision{replace("x", "y")}},
	}
	for _, tc := range rejected {
		t.Run("rejects/"+tc.name, func(t *testing.T) {
			if _, err := applyRevisions(tc.text, tc.revisions); !errors.Is(err, ErrMalformedOutput) {
				t.Fatalf("applyRevisions() error = %v, want ErrMalformedOutput", err)
			}
		})
	}

	t.Run("rejects overlapping occurrences that strings.Count misses", func(t *testing.T) {
		const aa = "\u3042\u3042"       // U+3042 twice
		text := "# T\n" + aa + "\u3042" // U+3042 three times
		if got := strings.Count(text, aa); got != 1 {
			t.Fatalf("strings.Count = %d, want 1 (the non-overlapping count this check must not use)", got)
		}
		if _, err := applyRevisions(text, []Revision{replace(aa, "x")}); !errors.Is(err, ErrMalformedOutput) {
			t.Fatalf("applyRevisions() error = %v, want ErrMalformedOutput", err)
		}
	})
}
