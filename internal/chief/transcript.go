package chief

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// errTranscript reports a Claude Code transcript whose place cannot be
// decided or that cannot be read; nothing is sent while it cannot be.
var errTranscript = errors.New("the Claude Code transcript cannot be read")

// errUnsupportedPath reports a working directory whose transcript directory
// cannot be decided: it holds a character the sampled rule does not cover.
// It is wrapped in errTranscript.
var errUnsupportedPath = errors.New("the working directory holds a character other than letters, digits, '/', '.', and '-'")

// The caps of a transcript read. The session /clear has just started holds
// a few lines; a session read for its last report, its PR records, and its
// last human input holds the whole run of a command, which reached megabytes
// in the samples, and its last human input may be at its start.
const (
	transcriptLimit        = 1 << 20
	sessionTranscriptLimit = 64 << 20
)

// The re-reads of the transcript after /model: Claude Code writes the record
// a moment after the command, so it is read again at this interval up to
// this many times.
const (
	transcriptReads    = 10
	transcriptInterval = 500 * time.Millisecond
)

// transcriptDirChars are the characters a working directory may hold for its
// transcript directory to be decided: the sampled rule replaces "/" and "."
// with "-", and a directory with another character could be named by a rule
// the samples did not show.
var transcriptDirChars = regexp.MustCompile(`^/[A-Za-z0-9/.-]*$`)

// transcriptPath returns the transcript of session for an agent working in
// dir: <config>/projects/<dir with "/" and "." replaced by "-">/<session>.jsonl,
// where config is CLAUDE_CONFIG_DIR or else $HOME/.claude. Contract: a place
// that cannot be decided by that rule (no session, a relative directory, a
// character outside transcriptDirChars, no absolute configuration directory)
// is an error, never a guess.
func transcriptPath(env Env, dir, session string) (string, error) {
	if !sessionIDForm.MatchString(session) {
		return "", fmt.Errorf("%w: no agent session", errTranscript)
	}
	if !transcriptDirChars.MatchString(dir) {
		return "", fmt.Errorf("%w: %w: %q", errTranscript, errUnsupportedPath, dir)
	}
	config := env.ClaudeConfigDir
	if config == "" && filepath.IsAbs(env.Home) {
		config = filepath.Join(env.Home, ".claude")
	}
	if !filepath.IsAbs(config) {
		return "", fmt.Errorf("%w: no Claude Code configuration directory", errTranscript)
	}
	name := strings.NewReplacer("/", "-", ".", "-").Replace(dir)
	return filepath.Join(config, "projects", name, session+".jsonl"), nil
}

// transcriptFailure is the reason and the detail of a send that stops
// because the transcript of a Claude Code pane cannot be found or read. A
// path the rule does not cover gets a reason of its own, with what the
// human can do about it.
func transcriptFailure(err error) (NotSentReason, string) {
	if errors.Is(err, errUnsupportedPath) {
		return NotSentPathUnsupported, err.Error() + "; chief cannot find this pane's Claude Code transcript, so it sends nothing to the pane. Use a worktree whose path has only letters, digits, '/', '.', and '-', or type the command in the pane yourself"
	}
	return NotSentNoTranscript, err.Error()
}

// readTranscript reads a transcript file: a regular file, at most limit
// bytes.
func readTranscript(path string, limit int64) ([]byte, error) {
	f, err := openTranscript(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errTranscript, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %s exceeds the read cap", errTranscript, path)
	}
	return data, nil
}

// openTranscript opens a transcript that is a regular file.
func openTranscript(path string) (*os.File, error) {
	// O_NONBLOCK keeps the open of a FIFO from waiting for a writer; the
	// check below then refuses it.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // the path is built by transcriptPath from checked parts
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errTranscript, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("%w: %w", errTranscript, err)
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("%w: %s is not a regular file", errTranscript, path)
	}
	return f, nil
}

// ClaudeReport is what chief read about a Claude Code pane's last report.
// Report.Kind is SessionReportUnreadable when the transcript's place cannot
// be decided, the file cannot be read, or its content is outside the
// accepted shape; Reason is then the reason value a reset shows
// (no_transcript or unsupported_path). Reason is no_report for
// SessionReportAbsent and unset for SessionReportFound. Detail says why the
// report is unreadable or absent, with the transcript's path when it is
// known.
type ClaudeReport struct {
	Report SessionReport
	Reason NotSentReason
	Detail string
}

// readClaudeReport reads the last report of a Claude Code pane from its
// session's transcript. Contract: a session file that does not exist is no
// report, never an unreadable transcript (a session /clear has just started
// has written none yet), and its Detail says whether the working
// directory's transcript directory exists either; every other failure to
// place, open, or read the file, and content outside the accepted shape, is
// unreadable, and the screen is never read instead.
func readClaudeReport(env Env, a Agent) ClaudeReport {
	path, err := transcriptPath(env, a.Dir, a.Session)
	if err != nil {
		reason, detail := transcriptFailure(err)
		return ClaudeReport{Reason: reason, Detail: detail}
	}
	data, err := readTranscript(path, sessionTranscriptLimit)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		detail := path + " does not exist"
		if _, derr := os.Stat(filepath.Dir(path)); errors.Is(derr, fs.ErrNotExist) {
			detail += "; the transcript directory " + filepath.Dir(path) + " does not exist either, so chief may not read the directory Claude Code writes to"
		}
		return ClaudeReport{Report: SessionReport{Kind: SessionReportAbsent}, Reason: NotSentNoReport, Detail: detail}
	case err != nil:
		return ClaudeReport{Reason: NotSentNoTranscript, Detail: err.Error()}
	}
	report := readSessionReport(data)
	switch report.Kind {
	case SessionReportFound:
		return ClaudeReport{Report: report}
	case SessionReportAbsent:
		return ClaudeReport{Report: report, Reason: NotSentNoReport, Detail: path + ": " + report.absence}
	default:
		return ClaudeReport{Report: report, Reason: NotSentNoTranscript, Detail: fmt.Sprintf("%v: %s is not in the shape the sampled transcripts have", errTranscript, path)}
	}
}
