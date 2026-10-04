//go:build test

package writer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/transcript"
)

// markerTemplate embeds the four template values, each between its own tags.
const markerTemplate = "<t>{{.Title}}</t><c>{{.ChannelName}}</c><d>{{.Description}}</d><s>{{.Transcript}}</s>"

// markerPrompt is what markerTemplate expands to for t.
func markerPrompt(t transcript.Transcript) string {
	texts := make([]string, len(t.Segments))
	for i, seg := range t.Segments {
		texts[i] = seg.Text
	}
	return "<t>" + t.Title + "</t><c>" + t.ChannelName + "</c><d>" + t.Description + "</d><s>" + strings.Join(texts, "\n") + "</s>"
}

// markerOptions overrides both templates with markerTemplate.
func markerOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		SystemTemplatePath: writeOverrideFile(t, markerTemplate),
		UserTemplatePath:   writeOverrideFile(t, markerTemplate),
	}
}

func TestWriteRejectsInvalidTranscript(t *testing.T) {
	const watch = "https://www.youtube.com/watch?v="
	cases := []struct {
		name   string
		modify func(tr *transcript.Transcript)
	}{
		// The VideoURL of each invalid VideoID is the concatenated URL, so
		// only the VideoID check can reject it.
		{"empty VideoID", func(tr *transcript.Transcript) { tr.VideoID = ""; tr.VideoURL = watch }},
		{"10-character VideoID", func(tr *transcript.Transcript) { tr.VideoID = "dQw4w9WgXc"; tr.VideoURL = watch + tr.VideoID }},
		{"VideoID with slash", func(tr *transcript.Transcript) { tr.VideoID = "dQw4w/WgXcQ"; tr.VideoURL = watch + tr.VideoID }},
		{"VideoID with dot-dot", func(tr *transcript.Transcript) { tr.VideoID = "dQw4..gXcQx"; tr.VideoURL = watch + tr.VideoID }},
		{"youtu.be VideoURL", func(tr *transcript.Transcript) { tr.VideoURL = "https://youtu.be/" + testVideoID }},
		{"http VideoURL", func(tr *transcript.Transcript) { tr.VideoURL = "http://www.youtube.com/watch?v=" + testVideoID }},
		{"VideoURL with extra query", func(tr *transcript.Transcript) { tr.VideoURL = watch + testVideoID + "&t=1s" }},
		{"VideoURL of another video", func(tr *transcript.Transcript) { tr.VideoURL = watch + "aaaaaaaaaaa" }},
		{"empty VideoURL", func(tr *transcript.Transcript) { tr.VideoURL = "" }},
		{"nil Segments", func(tr *transcript.Transcript) { tr.Segments = nil }},
		{"empty Segments", func(tr *transcript.Transcript) { tr.Segments = []transcript.Segment{} }},
		{"empty segment text", func(tr *transcript.Transcript) { tr.Segments[1].Text = "" }},
		{"whitespace-only segment text", func(tr *transcript.Transcript) { tr.Segments[1].Text = " \t\n\u3000" }},
		{"invalid UTF-8 Title", func(tr *transcript.Transcript) { tr.Title += "\xff" }},
		{"invalid UTF-8 ChannelName", func(tr *transcript.Transcript) { tr.ChannelName += "\xff" }},
		{"invalid UTF-8 Description", func(tr *transcript.Transcript) { tr.Description += "\xff" }},
		{"invalid UTF-8 segment text", func(tr *transcript.Transcript) { tr.Segments[1].Text += "\xff" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newFakeClient()
			w := mustNew(t, client, Options{})
			tr := validTranscript()
			tc.modify(&tr)
			a, err := w.Write(context.Background(), tr)
			if !errors.Is(err, ErrInvalidTranscript) {
				t.Fatalf("Write error = %v, want ErrInvalidTranscript", err)
			}
			if a != (Article{}) {
				t.Errorf("Write article = %+v, want the zero value", a)
			}
			if len(client.Calls) != 0 {
				t.Errorf("Generate called %d times, want 0", len(client.Calls))
			}
			for _, mark := range []string{markTitle, markChannel, markDescription, markSegment} {
				if strings.Contains(err.Error(), mark) {
					t.Errorf("Write error %q contains the transcript value marked %q", err, mark)
				}
			}
		})
	}
}

func TestWriteAcceptsEmptyMetadata(t *testing.T) {
	client := newFakeClient()
	w := mustNew(t, client, Options{})
	tr := validTranscript()
	tr.Title, tr.ChannelName, tr.Description = "", "", ""
	if _, err := w.Write(context.Background(), tr); err != nil {
		t.Fatalf("Write error = %v, want nil", err)
	}
	if len(client.Calls) != 1 {
		t.Errorf("Generate called %d times, want 1", len(client.Calls))
	}
}

func TestWriteEmbedsValues(t *testing.T) {
	client := newFakeClient()
	w := mustNew(t, client, markerOptions(t))
	tr := validTranscript()
	tr.Segments = []transcript.Segment{
		{Text: " leading space " + markSegment},
		{Text: "\n" + markSegment + " surrounded by newlines\n"},
		{Text: "\ttab\t"},
	}
	if _, err := w.Write(context.Background(), tr); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	want := "<t>" + markTitle + "</t><c>" + markChannel + "</c><d>" + markDescription + "</d><s>" +
		" leading space " + markSegment + "\n\n" + markSegment + " surrounded by newlines\n\n\ttab\t</s>"
	req := client.Calls[0].Request
	if req.SystemPrompt != want {
		t.Errorf("system prompt = %q, want %q", req.SystemPrompt, want)
	}
	if req.UserPrompt != want {
		t.Errorf("user prompt = %q, want %q", req.UserPrompt, want)
	}
}

func TestWriteOmitsStartMs(t *testing.T) {
	const startMs = 987654321
	write := func(t *testing.T, tr transcript.Transcript) (system, user string) {
		t.Helper()
		client := newFakeClient()
		w := mustNew(t, client, Options{})
		if _, err := w.Write(context.Background(), tr); err != nil {
			t.Fatalf("Write error = %v", err)
		}
		return client.Calls[0].Request.SystemPrompt, client.Calls[0].Request.UserPrompt
	}

	t.Run("value absent", func(t *testing.T) {
		tr := validTranscript()
		tr.Segments[1].StartMs = startMs
		system, user := write(t, tr)
		if strings.Contains(system+user, "987654321") {
			t.Error("a prompt contains the StartMs value")
		}
	})
	// A formatted timestamp (1:02:03) would not contain the raw number, so
	// prompts that differ only in StartMs must also be identical.
	t.Run("prompts independent of StartMs", func(t *testing.T) {
		a := validTranscript()
		b := validTranscript()
		b.Segments[0].StartMs = startMs
		b.Segments[1].StartMs = 3723000
		systemA, userA := write(t, a)
		systemB, userB := write(t, b)
		if systemA != systemB || userA != userB {
			t.Error("prompts differ between transcripts that differ only in StartMs")
		}
	})
}

func TestWriteDoesNotInterpretValues(t *testing.T) {
	client := newFakeClient()
	w := mustNew(t, client, markerOptions(t))
	tr := validTranscript()
	tr.Title = "{{.Title}}"
	tr.Description = `{{printf "%s" "x"}}`
	tr.Segments[1].Text = "{{"
	if _, err := w.Write(context.Background(), tr); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	want := markerPrompt(tr)
	if got := client.Calls[0].Request.UserPrompt; got != want {
		t.Errorf("user prompt = %q, want %q", got, want)
	}
}

func TestWriteExpansionFailure(t *testing.T) {
	cases := []struct {
		name     string
		template string
		title    string
	}{
		{"blank expansion", "{{.Title}}", " \t\n\u3000"},
		{"execution error", "{{index .Title 100}}", "short"},
	}
	for _, target := range overrideTargets {
		for _, tc := range cases {
			t.Run(target.name+"/"+tc.name, func(t *testing.T) {
				client := newFakeClient()
				w := mustNew(t, client, target.options(writeOverrideFile(t, tc.template)))
				tr := validTranscript()
				tr.Title = tc.title
				a, err := w.Write(context.Background(), tr)
				if !errors.Is(err, ErrInvalidTemplate) {
					t.Fatalf("Write error = %v, want ErrInvalidTemplate", err)
				}
				if errors.Is(err, errPromptTooLarge) {
					t.Errorf("Write error = %v, want a failure other than the size limit", err)
				}
				if a != (Article{}) {
					t.Errorf("Write article = %+v, want the zero value", a)
				}
				if len(client.Calls) != 0 {
					t.Errorf("Generate called %d times, want 0", len(client.Calls))
				}
			})
		}
	}
}

func TestWritePromptSizeLimit(t *testing.T) {
	cases := []struct {
		name      string
		size      int
		wantCalls int
	}{
		{"at limit", maxPromptBytes, 1},
		{"over limit", maxPromptBytes + 1, 0},
	}
	for _, target := range overrideTargets {
		for _, tc := range cases {
			t.Run(target.name+"/"+tc.name, func(t *testing.T) {
				// Only the target template embeds the title; the other one is
				// constant, so only the target prompt can reach the limit.
				opts := Options{
					SystemTemplatePath: writeOverrideFile(t, "x"),
					UserTemplatePath:   writeOverrideFile(t, "x"),
				}
				opts = mergeOptions(opts, target.options(writeOverrideFile(t, "{{.Title}}")))
				client := newFakeClient()
				w := mustNew(t, client, opts)
				tr := validTranscript()
				tr.Title = strings.Repeat("a", tc.size)
				_, err := w.Write(context.Background(), tr)
				if tc.wantCalls == 1 && err != nil {
					t.Fatalf("Write error = %v, want nil", err)
				}
				if tc.wantCalls == 0 && (!errors.Is(err, ErrInvalidTemplate) || !errors.Is(err, errPromptTooLarge)) {
					t.Fatalf("Write error = %v, want ErrInvalidTemplate for the size limit", err)
				}
				if len(client.Calls) != tc.wantCalls {
					t.Errorf("Generate called %d times, want %d", len(client.Calls), tc.wantCalls)
				}
			})
		}
	}
}

// mergeOptions returns base with every non-empty path of override applied.
func mergeOptions(base, override Options) Options {
	if override.SystemTemplatePath != "" {
		base.SystemTemplatePath = override.SystemTemplatePath
	}
	if override.UserTemplatePath != "" {
		base.UserTemplatePath = override.UserTemplatePath
	}
	return base
}

func TestBoundedWriter(t *testing.T) {
	const limit = 4
	t.Run("one write at limit", func(t *testing.T) {
		w := &boundedWriter{limit: limit}
		if n, err := w.Write([]byte("abcd")); err != nil || n != limit {
			t.Fatalf("Write = %d, %v; want %d, nil", n, err, limit)
		}
	})
	t.Run("one write over limit", func(t *testing.T) {
		w := &boundedWriter{limit: limit}
		if _, err := w.Write([]byte("abcde")); !errors.Is(err, errPromptTooLarge) {
			t.Fatalf("Write error = %v, want errPromptTooLarge", err)
		}
		if w.buf.Len() != 0 {
			t.Errorf("buffer holds %d bytes of a refused write, want 0", w.buf.Len())
		}
	})
	t.Run("writes accumulate", func(t *testing.T) {
		w := &boundedWriter{limit: limit}
		for _, p := range []string{"ab", "cd"} {
			if _, err := w.Write([]byte(p)); err != nil {
				t.Fatalf("Write(%q) error = %v", p, err)
			}
		}
		if _, err := w.Write([]byte("e")); !errors.Is(err, errPromptTooLarge) {
			t.Fatalf("Write past the total error = %v, want errPromptTooLarge", err)
		}
		if got := w.buf.String(); got != "abcd" {
			t.Errorf("buffer = %q, want %q", got, "abcd")
		}
	})
}
