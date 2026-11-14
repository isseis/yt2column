package chief

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// errTranscript reports a Claude Code transcript whose place cannot be
// decided or that cannot be read; nothing is sent while it cannot be.
var errTranscript = errors.New("the Claude Code transcript cannot be read")

// transcriptLimit caps a transcript read. Chief reads only the transcript of
// the session /clear has just started, which holds a few lines.
const transcriptLimit = 1 << 20

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
		return "", fmt.Errorf("%w: the working directory %q has no known transcript directory", errTranscript, dir)
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

// readTranscript reads a transcript file: a regular file, at most
// transcriptLimit bytes.
func readTranscript(path string) ([]byte, error) {
	f, err := openTranscript(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, transcriptLimit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errTranscript, err)
	}
	if len(data) > transcriptLimit {
		return nil, fmt.Errorf("%w: %s exceeds the read cap", errTranscript, path)
	}
	return data, nil
}

// transcriptOpens reports whether the transcript at path can be opened as a
// regular file, without reading it: the transcript of a long session may be
// larger than the read cap, and only its place is checked before the first
// input.
func transcriptOpens(path string) error {
	f, err := openTranscript(path)
	if err != nil {
		return err
	}
	return f.Close()
}

// openTranscript opens a transcript that is a regular file.
func openTranscript(path string) (*os.File, error) {
	f, err := os.Open(path) //nolint:gosec // the path is built by transcriptPath from checked parts
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
