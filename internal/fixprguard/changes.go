package fixprguard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
)

const (
	// maxChanges caps the changed paths one round may have; more stop the
	// round instead of judging part of them.
	maxChanges = 1000
	// maxDigestBytes caps a file whose content is digested; a larger file is
	// a ChangeOther and so out of scope.
	maxDigestBytes = 8 << 20
	// binarySniffBytes is how much of an untracked file is searched for a NUL
	// byte to call it binary, as git does.
	binarySniffBytes = 8000

	modeRegular    = "100644"
	modeExecutable = "100755"
	modeSymlink    = "120000"
	modeGitlink    = "160000"
)

// errChangeCap reports more than maxChanges changed paths.
var errChangeCap = fmt.Errorf("%w: change cap", ErrLimitExceeded)

// ChangeKind is the kind of change a path has in the change set. The zero
// value is unknown, which is never in scope.
type ChangeKind int

// Change kinds.
const (
	ChangeUnknown ChangeKind = iota
	ChangeAdd
	ChangeModify
	ChangeDelete
	ChangeOther // type change, symlink, submodule, non-regular or oversized file
)

// MarshalText names the kind. ChangeUnknown and unknown values are an error.
func (k ChangeKind) MarshalText() ([]byte, error) {
	switch k {
	case ChangeAdd:
		return []byte("add"), nil
	case ChangeModify:
		return []byte("modify"), nil
	case ChangeDelete:
		return []byte("delete"), nil
	case ChangeOther:
		return []byte("other"), nil
	default:
		return nil, fmt.Errorf("%w: change kind", errUnknownEnum)
	}
}

// change is one changed path relative to a commit: the work tree's version
// against the commit's, or an untracked file. mode is the work tree's git
// mode ("" for a deletion), and digest the SHA-256 of a regular file's
// content (the link target for a symbolic link, "" for a deletion or an
// oversized file). added and deleted are line counts unless binary.
type change struct {
	path    string
	kind    ChangeKind
	mode    string
	digest  string
	added   int
	deleted int
	binary  bool
}

// JudgedSetID identifies a change set over each entry's path, kind, mode and
// content: a hex-encoded SHA-256.
type JudgedSetID string

// ParseJudgedSetID rejects anything but a hex SHA-256.
func ParseJudgedSetID(s string) (JudgedSetID, error) {
	if !validHex(s, digestLength) {
		return "", fmt.Errorf("%w: judged set ID", ErrInvalidInput)
	}
	return JudgedSetID(s), nil
}

// judgedSetID hashes the changes in path order. Lines are left out: they
// follow from the content.
func judgedSetID(changes []change) JudgedSetID {
	h := sha256.New()
	for _, c := range changes {
		writeField(h, c.path)
		writeField(h, strconv.Itoa(int(c.kind)))
		writeField(h, c.mode)
		writeField(h, c.digest)
	}
	return JudgedSetID(hex.EncodeToString(h.Sum(nil)))
}

// rawEntry is one record of git diff --raw -z.
type rawEntry struct {
	oldMode string
	newMode string
	status  byte
}

// enumerateChanges lists every change of the work tree against the commit
// against: tracked changes (index and work tree together) with rename
// detection off, so a rename is a deletion and an addition, plus untracked
// files git does not ignore. Each path is examined with lstat; only a regular
// file is opened. It changes nothing in the index.
func (w *Worktree) enumerateChanges(ctx context.Context, against string) ([]change, error) {
	diffArgs := []string{"--no-renames", "--no-ext-diff", "--no-textconv", "--ignore-submodules=none", "-z", against, "--"}
	rawOut, err := w.git(ctx, "diff --raw", append([]string{"diff", "--raw"}, diffArgs...)...)
	if err != nil {
		return nil, err
	}
	raw, err := parseRaw(rawOut)
	if err != nil {
		return nil, err
	}
	numOut, err := w.git(ctx, "diff --numstat", append([]string{"diff", "--numstat"}, diffArgs...)...)
	if err != nil {
		return nil, err
	}
	counts, err := parseNumstat(numOut)
	if err != nil {
		return nil, err
	}
	untrackedOut, err := w.git(ctx, "ls-files", "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var untracked []string
	for p := range bytes.SplitSeq(untrackedOut, []byte{0}) {
		if len(p) > 0 {
			untracked = append(untracked, string(p))
		}
	}
	if len(raw)+len(untracked) > maxChanges {
		return nil, errChangeCap
	}
	changes := make([]change, 0, len(raw)+len(untracked))
	for p, entry := range raw {
		c, err := trackedChange(p, entry, counts[p])
		if err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	for _, p := range untracked {
		if _, dup := raw[p]; dup {
			return nil, fmt.Errorf("%w: path listed twice", ErrGitFailed)
		}
		c, err := untrackedChange(p)
		if err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	slices.SortFunc(changes, func(a, b change) int { return strings.Compare(a.path, b.path) })
	return changes, nil
}

// parseRaw reads ":<old mode> <new mode> <old oid> <new oid> <status>\0<path>\0"
// records; with rename detection off each has one path.
func parseRaw(out []byte) (map[string]rawEntry, error) {
	entries := map[string]rawEntry{}
	fields := bytes.Split(out, []byte{0})
	if len(fields) > 0 && len(fields[len(fields)-1]) == 0 {
		fields = fields[:len(fields)-1]
	}
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("%w: diff --raw output", ErrGitFailed)
	}
	for i := 0; i < len(fields); i += 2 {
		meta := strings.Fields(strings.TrimPrefix(string(fields[i]), ":"))
		p := string(fields[i+1])
		if len(meta) != 5 || len(meta[4]) == 0 || p == "" {
			return nil, fmt.Errorf("%w: diff --raw output", ErrGitFailed)
		}
		if _, dup := entries[p]; dup {
			return nil, fmt.Errorf("%w: path listed twice", ErrGitFailed)
		}
		entries[p] = rawEntry{oldMode: meta[0], newMode: meta[1], status: meta[4][0]}
	}
	return entries, nil
}

// lineCount is one record of git diff --numstat.
type lineCount struct {
	added, deleted int
	binary         bool
}

// parseNumstat reads "<added>\t<deleted>\t<path>\0" records; a binary file
// has "-" for both counts.
func parseNumstat(out []byte) (map[string]lineCount, error) {
	counts := map[string]lineCount{}
	for record := range bytes.SplitSeq(out, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		added, rest, ok1 := strings.Cut(string(record), "\t")
		deleted, p, ok2 := strings.Cut(rest, "\t")
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("%w: diff --numstat output", ErrGitFailed)
		}
		if added == "-" && deleted == "-" {
			counts[p] = lineCount{binary: true}
			continue
		}
		a, err1 := strconv.Atoi(added)
		d, err2 := strconv.Atoi(deleted)
		if err1 != nil || err2 != nil || a < 0 || d < 0 {
			return nil, fmt.Errorf("%w: diff --numstat output", ErrGitFailed)
		}
		counts[p] = lineCount{added: a, deleted: d}
	}
	return counts, nil
}

// trackedChange classifies one tracked change. A type change, a symbolic
// link or submodule on either side, and anything that is not a regular file
// in the work tree now are ChangeOther.
func trackedChange(p string, entry rawEntry, lines lineCount) (change, error) {
	c := change{path: p, added: lines.added, deleted: lines.deleted, binary: lines.binary}
	switch entry.status {
	case 'A':
		c.kind = ChangeAdd
	case 'M':
		c.kind = ChangeModify
	case 'D':
		c.kind = ChangeDelete
	default:
		c.kind = ChangeOther
	}
	if entry.oldMode == modeSymlink || entry.oldMode == modeGitlink || entry.newMode == modeGitlink {
		c.kind = ChangeOther
	}
	if c.kind == ChangeDelete {
		if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
			// The path exists again (as a directory, say): not a deletion.
			c.kind = ChangeOther
		}
		return c, nil
	}
	return examine(c)
}

// untrackedChange classifies an untracked path; lines are counted from the
// content. ls-files lists a nested repository as a directory ending in "/".
func untrackedChange(p string) (change, error) {
	c, err := examine(change{path: p, kind: ChangeAdd})
	if err != nil || c.kind != ChangeAdd {
		return c, err
	}
	data, err := readRegular(p)
	if err != nil {
		return change{}, err
	}
	if bytes.IndexByte(data[:min(len(data), binarySniffBytes)], 0) >= 0 {
		c.binary = true
		return c, nil
	}
	c.added = bytes.Count(data, []byte{'\n'})
	if len(data) > 0 && data[len(data)-1] != '\n' {
		c.added++
	}
	return c, nil
}

// examine records the work tree's mode and digest for c with lstat, turning c
// into ChangeOther unless it is a regular file of at most maxDigestBytes.
func examine(c change) (change, error) {
	info, err := os.Lstat(strings.TrimSuffix(c.path, "/"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			c.kind = ChangeOther
			return c, nil
		}
		return change{}, fmt.Errorf("%w: lstat a changed path", ErrGitFailed)
	}
	mode := info.Mode()
	switch {
	case mode&fs.ModeSymlink != 0:
		target, err := os.Readlink(c.path)
		if err != nil {
			return change{}, fmt.Errorf("%w: read a changed link", ErrGitFailed)
		}
		c.kind, c.mode, c.digest = ChangeOther, modeSymlink, digestString(target)
		return c, nil
	case !mode.IsRegular() || strings.HasSuffix(c.path, "/"):
		c.kind, c.mode = ChangeOther, mode.Type().String()
		return c, nil
	case info.Size() > maxDigestBytes:
		c.kind, c.mode = ChangeOther, gitFileMode(mode)
		return c, nil
	}
	data, err := readRegular(c.path)
	if err != nil {
		return change{}, err
	}
	sum := sha256.Sum256(data)
	c.mode, c.digest = gitFileMode(mode), hex.EncodeToString(sum[:])
	return c, nil
}

func gitFileMode(mode fs.FileMode) string {
	if mode.Perm()&0o111 != 0 {
		return modeExecutable
	}
	return modeRegular
}

// readRegular reads at most maxDigestBytes of a file lstat found regular. A
// file that grew past the cap meanwhile is an error, not a truncated read.
func readRegular(p string) ([]byte, error) {
	f, err := os.Open(p) //nolint:gosec // a changed path of the current work tree, checked regular with lstat
	if err != nil {
		return nil, fmt.Errorf("%w: open a changed path", ErrGitFailed)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxDigestBytes+1))
	if err != nil || len(data) > maxDigestBytes {
		return nil, fmt.Errorf("%w: read a changed path", ErrGitFailed)
	}
	return data, nil
}
