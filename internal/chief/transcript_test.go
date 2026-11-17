//go:build test && unix

package chief

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// TestReadClaudeReport checks the kind and the reason the report entry gives
// for each state of a pane's transcript: a place that cannot be decided, a
// file that cannot be read, is too large, or is outside the accepted shape is
// unreadable; a missing file is no report, and its detail says whether the
// transcript directory is missing too.
func TestReadClaudeReport(t *testing.T) {
	const session = "00000000-0000-4000-8000-000000000001"
	config := t.TempDir()
	env := Env{ClaudeConfigDir: config}
	claude := func(dir string) Agent {
		return Agent{Pane: PaneID{}, Kind: paneinput.AgentClaudeCode, Session: session, Dir: dir}
	}
	// place makes the transcript directory of a working directory and
	// returns the session's transcript path in it.
	place := func(t *testing.T, dir string) string {
		t.Helper()
		path, err := transcriptPath(env, dir, session)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write := func(t *testing.T, dir, content string) {
		t.Helper()
		if err := os.WriteFile(place(t, dir), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	report := joined(transcriptSample(t, "claude-runpr-report.jsonl"))

	cases := []struct {
		name   string
		env    Env
		agent  Agent
		setup  func(t *testing.T, dir string)
		kind   SessionReportKind
		reason NotSentReason
	}{
		{"a readable report", env, claude("/work/found"), func(t *testing.T, dir string) { write(t, dir, report) }, SessionReportFound, NotSentUnknown},
		{"a readable transcript without a report", env, claude("/work/absent"), func(t *testing.T, dir string) { write(t, dir, commandLine("/runpr 940")) }, SessionReportAbsent, NotSentNoReport},
		{"no session", env, Agent{Kind: paneinput.AgentClaudeCode, Dir: "/work/a"}, nil, SessionReportUnreadable, NotSentNoTranscript},
		{"a working directory with an unsampled character", env, claude("/work/a_b"), nil, SessionReportUnreadable, NotSentPathUnsupported},
		{"no configuration directory", Env{}, claude("/work/a"), nil, SessionReportUnreadable, NotSentNoTranscript},
		{"a missing file", env, claude("/work/missing"), func(t *testing.T, dir string) { place(t, dir) }, SessionReportAbsent, NotSentNoReport},
		{"content outside the accepted shape", env, claude("/work/broken"), func(t *testing.T, dir string) { write(t, dir, report+"\n{\"type\":") }, SessionReportUnreadable, NotSentNoTranscript},
		{"a file that cannot be opened", env, claude("/work/closed"), func(t *testing.T, dir string) {
			write(t, dir, report)
			path := place(t, dir)
			if err := os.Chmod(path, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
		}, SessionReportUnreadable, NotSentNoTranscript},
		{"a file one byte over the cap", env, claude("/work/large"), func(t *testing.T, dir string) {
			write(t, dir, "")
			if err := os.Truncate(place(t, dir), sessionTranscriptLimit+1); err != nil {
				t.Fatal(err)
			}
		}, SessionReportUnreadable, NotSentNoTranscript},
		{"a FIFO", env, claude("/work/fifo"), func(t *testing.T, dir string) {
			if err := syscall.Mkfifo(place(t, dir), 0o600); err != nil {
				t.Fatal(err)
			}
		}, SessionReportUnreadable, NotSentNoTranscript},
		{"a directory", env, claude("/work/dir"), func(t *testing.T, dir string) {
			if err := os.Mkdir(place(t, dir), 0o700); err != nil {
				t.Fatal(err)
			}
		}, SessionReportUnreadable, NotSentNoTranscript},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.setup != nil {
				c.setup(t, c.agent.Dir)
			}
			got := readClaudeReport(c.env, c.agent)
			if got.Report.Kind != c.kind || got.Reason != c.reason {
				t.Fatalf("readClaudeReport = kind %v, reason %v (%s); want kind %v, reason %v", got.Report.Kind, got.Reason, got.Detail, c.kind, c.reason)
			}
			if (c.kind == SessionReportFound) != (got.Detail == "") {
				t.Errorf("Detail = %q; want one exactly when there is no report", got.Detail)
			}
			if c.kind == SessionReportFound && !strings.HasPrefix(got.Report.Text, runprReportHead) {
				t.Errorf("Text = %q; want the sampled report", got.Report.Text)
			}
		})
	}

	// The same session file, missing with and without its transcript
	// directory: both are no report, and only the detail tells them apart.
	nodir := readClaudeReport(env, claude("/work/nodir"))
	place(t, "/work/nodir")
	nofile := readClaudeReport(env, claude("/work/nodir"))
	for _, got := range []ClaudeReport{nodir, nofile} {
		if got.Report.Kind != SessionReportAbsent || got.Reason != NotSentNoReport || got.Detail == "" {
			t.Errorf("a missing session file = %+v; want no report with a detail", got)
		}
	}
	if nodir.Detail == nofile.Detail {
		t.Errorf("a missing transcript directory and a missing file give the same detail %q", nodir.Detail)
	}
}
