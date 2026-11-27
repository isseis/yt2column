//go:build test && unix

package chief

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// TestReadClaudeReport checks the kind and the reason the report entry gives
// for each state of a pane's transcript: a place that cannot be decided, a
// file that cannot be read, is too large, or is outside the accepted shape is
// unreadable; a missing file is no report, and its detail says whether the
// transcript directory is missing too.
func TestReadClaudeReport(t *testing.T) {
	t.Parallel()
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
	// The sampled report padded with blank lines to one byte over the cap:
	// only the cap refuses it, since its shape reads as a report.
	overCap := report + strings.Repeat("\n", sessionTranscriptLimit+1-len(report))
	if got := readSessionReport([]byte(overCap)); got.Kind != SessionReportFound {
		t.Fatalf("the padded report reads as %v without the cap; want a report", got.Kind)
	}

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
		{"content outside the accepted shape", env, claude("/work/broken"), func(t *testing.T, dir string) { write(t, dir, report+"\n{\"type\":") }, SessionReportUnreadable, NotSentNoTranscript},
		{"a file that cannot be opened", env, claude("/work/closed"), func(t *testing.T, dir string) {
			if os.Geteuid() == 0 {
				t.Skip("root opens a file of mode 0")
			}
			write(t, dir, report)
			path := place(t, dir)
			if err := os.Chmod(path, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
		}, SessionReportUnreadable, NotSentNoTranscript},
		{"a file one byte over the cap", env, claude("/work/large"), func(t *testing.T, dir string) { write(t, dir, overCap) }, SessionReportUnreadable, NotSentNoTranscript},
		{"a FIFO", env, claude("/work/fifo"), func(t *testing.T, dir string) {
			if err := syscall.Mkfifo(place(t, dir), 0o600); err != nil {
				t.Fatal(err)
			}
		}, SessionReportUnreadable, NotSentNoTranscript},
		// Reading a directory fails anyway; the FIFO row is the one that
		// needs the regular-file check, since a FIFO without a writer reads
		// as empty.
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
		if got.Report.Kind != SessionReportAbsent || got.Reason != NotSentNoReport || got.Detail == "" || got.Report.LastHuman != (HumanInput{Kind: HumanInputNone}) {
			t.Errorf("a missing session file = %+v; want no report with a detail", got)
		}
	}
	if !strings.HasPrefix(nodir.Detail, nofile.Detail) || len(nodir.Detail) <= len(nofile.Detail) {
		t.Errorf("a missing transcript directory gives the detail %q; want the missing file's %q with more", nodir.Detail, nofile.Detail)
	}
}

// TestReadCompleteTranscript checks the read for display: a transcript that
// ends in a newline, or is empty, is read at once; one that does not is read
// again after each wait of 200 ms, up to three times, and is returned once its line ends; one
// whose line never ends, or whose wait fails, is errPartialLine; and one over
// the cap is errOverCap.
func TestReadCompleteTranscript(t *testing.T) {
	t.Parallel()
	const line = `{"type":"user"}` + "\n"
	const half = `{"type":"us`
	cases := []struct {
		name string
		// first is the file's content at the first read.
		first string
		// endAt is the wait after which the rest of the line is written, 0
		// for never.
		endAt int
		// fail makes the waits fail.
		fail      bool
		limit     int64
		wantWaits int
		wantErr   error
		want      string
	}{
		{name: "a transcript ending in a newline", first: line, limit: 1 << 10, want: line},
		{name: "an empty transcript", first: "", limit: 1 << 10, want: ""},
		{name: "a line that ends after the last wait", first: line + half, endAt: 3, limit: 1 << 10, wantWaits: 3, want: line + line},
		{name: "a line that never ends", first: line + half, limit: 1 << 10, wantWaits: 3, wantErr: errPartialLine},
		{name: "a wait that fails", first: line + half, fail: true, limit: 1 << 10, wantWaits: 1, wantErr: errPartialLine},
		{name: "a transcript over the cap", first: line + line, limit: int64(len(line)), wantErr: errOverCap},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "s.jsonl")
			if err := os.WriteFile(path, []byte(c.first), 0o600); err != nil {
				t.Fatal(err)
			}
			waits := 0
			sleep := func(_ context.Context, d time.Duration) error {
				waits++
				// The plan decided 200 ms; a wait of another length is a change
				// of that decision, not of the constant alone.
				if d != 200*time.Millisecond {
					t.Errorf("wait %d is %v, want 200ms", waits, d)
				}
				if c.fail {
					return context.Canceled
				}
				if waits == c.endAt {
					return os.WriteFile(path, []byte(line+line), 0o600)
				}
				return nil
			}
			got, err := readCompleteTranscript(context.Background(), sleep, path, c.limit)
			if waits != c.wantWaits {
				t.Errorf("waited %d times, want %d", waits, c.wantWaits)
			}
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("error = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil || string(got) != c.want {
				t.Errorf("readCompleteTranscript = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}
