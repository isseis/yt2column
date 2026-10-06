//go:build test || integration

// Package transcripttestutil provides transcript-stage fixtures that more than
// one package uses: fake yt-dlp scripts and a directory listing. The scripts
// are real executables, so a test runs the production yt-dlp boundary instead
// of a fake executor.
package transcripttestutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// stoppingLifetime caps how long a stopping fake waits before exiting by
// itself. It stays below the go test timeout, so a leaked fake does not outlive
// the test run.
const stoppingLifetime = 3 * time.Minute

// stoppingStep is the sleep granularity of the fake's wait loops.
const stoppingStep = 500 * time.Millisecond

// subtitleSuffix and infoSuffix are the names yt-dlp gives the files the
// transcript cache reads. They match the source's fixed subtitle language and
// format; the package cannot reference the unexported constants.
const (
	subtitleSuffix = ".ja.json3"
	infoSuffix     = ".info.json"
)

// StoppingFake is a fake yt-dlp that records its state, creates Ready, and then
// stops until the release file appears or its lifetime cap passes. It is used
// to hold a run open while a test acts.
type StoppingFake struct {
	Script   string
	Ready    string
	Release  string
	PID      string
	ChildPID string
	FD3      string // "open" or "closed": whether descriptor 3 was inherited
}

// NewTripwire returns a fake yt-dlp that writes marker and exits non-zero when
// it is started. A test asserts the marker is absent to prove yt-dlp did not
// run.
func NewTripwire(t *testing.T, dir string) (script, marker string) {
	t.Helper()
	marker = filepath.Join(dir, "yt-dlp-tripwire-ran")
	script = writeScript(t, dir, "yt-dlp-tripwire", "#!/bin/sh\n"+moveIntoPlace(marker, quote(""))+"\nexit 1\n")
	return script, marker
}

// NewStopping returns a fake yt-dlp that records its own PID, its child's PID,
// and whether descriptor 3 is open, then creates Ready and waits for Release or
// the lifetime cap. It registers a cleanup that creates Release, so a failing
// test does not leave the fake running; it never signals a recorded PID, which
// could have been recycled.
func NewStopping(t *testing.T, dir string) *StoppingFake {
	t.Helper()
	f := &StoppingFake{
		Ready:    filepath.Join(dir, "yt-dlp-ready"),
		Release:  filepath.Join(dir, "yt-dlp-release"),
		PID:      filepath.Join(dir, "yt-dlp.pid"),
		ChildPID: filepath.Join(dir, "yt-dlp-child.pid"),
		FD3:      filepath.Join(dir, "yt-dlp-fd3"),
	}
	t.Cleanup(func() { _ = os.WriteFile(f.Release, nil, 0o600) })

	steps := strconv.Itoa(int(stoppingLifetime / stoppingStep))
	// The child loop reads the release path from $1, so the path is quoted once
	// as an argument instead of nested inside the sh -c string.
	child := `n=0; while [ ! -e "$1" ] && [ $n -lt ` + steps + ` ]; do sleep 0.5; n=$((n+1)); done`
	body := "#!/bin/sh\n" +
		moveIntoPlace(f.PID, `"$$"`) + "\n" +
		"if [ -e /dev/fd/3 ]; then " + moveIntoPlace(f.FD3, quote("open")) + "; else " + moveIntoPlace(f.FD3, quote("closed")) + "; fi\n" +
		"sh -c " + quote(child) + " sh " + quote(f.Release) + " &\n" +
		moveIntoPlace(f.ChildPID, `"$!"`) + "\n" +
		moveIntoPlace(f.Ready, quote("")) + "\n" +
		"n=0\nwhile [ ! -e " + quote(f.Release) + " ] && [ $n -lt " + steps + " ]; do sleep 0.5; n=$((n+1)); done\n" +
		"exit 0\n"
	f.Script = writeScript(t, dir, "yt-dlp-stopping", body)
	return f
}

// NewStderrFailure returns a fake yt-dlp that writes message to standard error
// and exits non-zero.
func NewStderrFailure(t *testing.T, dir, message string) string {
	t.Helper()
	messageFile := filepath.Join(dir, "yt-dlp-stderr.txt")
	writeFile(t, messageFile, message)
	script := "#!/bin/sh\ncat " + quote(messageFile) + " >&2\nexit 1\n"
	return writeScript(t, dir, "yt-dlp-stderr-failure", script)
}

// NewSuccess returns a fake yt-dlp that writes subtitles and info into the
// directory named by its -P argument and exits zero. The files carry the names
// the transcript cache reads for id.
func NewSuccess(t *testing.T, dir, id, subtitles, info string) string {
	t.Helper()
	subtitlesFile := filepath.Join(dir, "yt-dlp-subtitles.json3")
	infoFile := filepath.Join(dir, "yt-dlp-info.json")
	writeFile(t, subtitlesFile, subtitles)
	writeFile(t, infoFile, info)

	script := "#!/bin/sh\n" +
		`d=` + "\n" +
		`prev=` + "\n" +
		`for a in "$@"; do if [ "$prev" = "-P" ]; then d="$a"; fi; prev="$a"; done` + "\n" +
		"cp " + quote(subtitlesFile) + ` "$d/` + id + subtitleSuffix + `"` + "\n" +
		"cp " + quote(infoFile) + ` "$d/` + id + infoSuffix + `"` + "\n" +
		"exit 0\n"
	return writeScript(t, dir, "yt-dlp-success", script)
}

// ListDir returns the sorted names in dir, or nil when dir does not exist.
func ListDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

// writeScript writes an executable script and returns its path.
func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil { //nolint:gosec // the fake yt-dlp must be executable
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// writeFile writes content to path inside the test's temporary directory.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// moveIntoPlace returns a shell command that writes expr to a temporary file
// and renames it over path, so a reader never sees a partial file.
func moveIntoPlace(path, expr string) string {
	q := quote(path)
	return "printf '%s' " + expr + " > " + q + ".tmp && mv " + q + ".tmp " + q
}

// quote returns s single-quoted for the shell.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
