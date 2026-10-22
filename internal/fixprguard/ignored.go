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
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The watch list: the files git ignores that the tool still compares with
// the baseline, because a later session or a later make test reads them. It
// is kept apart from the deny list of the scope rules, which decides what a
// round may change, and shares only goBuildInputSuffixes with it. Matching
// reads the repository-relative path alone, never the file system, and folds
// case with Unicode simple folding (foldEqual). A case-insensitive macOS
// volume also applies the full folding that expands one rune into several
// (U+FB01 to "fi"); a name equal to an item only under that expansion is not
// matched, a residual risk the guide records.
var (
	// watchTopDir matches every path whose first element it is, the element
	// itself included.
	watchTopDir = ".claude"
	// watchTopFiles match a path of one element.
	watchTopFiles = []string{".env", ".envrc", "claude.local.md"}
	// watchNames match a base name in any directory.
	watchNames = []string{"claude.md", "makefile", "gnumakefile", "go.mod", "go.work"}
	// watchSuffixes match the end of a base name in any directory, a name
	// that is only the suffix included; goBuildInputSuffixes are matched the
	// same way.
	watchSuffixes = []string{".go", ".mk"}
	// watchAllSuffixes is every suffix the watch list matches.
	watchAllSuffixes = slices.Concat(watchSuffixes, goBuildInputSuffixes)
)

// WatchListPatterns lists the watch list in the form the guide prints it:
// ".claude/" matches every path below the top-level .claude, a pattern
// starting with "/" matches that file at the top level only, a bare name
// matches a file name in any directory, and "*.ext" matches the end of a file
// name in any directory. Matching folds case as foldEqual does.
func WatchListPatterns() []string {
	patterns := []string{watchTopDir + "/"}
	for _, f := range watchTopFiles {
		patterns = append(patterns, "/"+f)
	}
	patterns = append(patterns, watchNames...)
	for _, s := range watchAllSuffixes {
		patterns = append(patterns, "*"+s)
	}
	return patterns
}

// isWatched reports whether a repository-relative slash path is on the watch
// list. The path is matched as it is, never cleaned or normalized: an
// invalid byte or a control character in a name does not take a path off the
// list.
func isWatched(p string) bool {
	elems := strings.Split(p, "/")
	if foldEqual(elems[0], watchTopDir) {
		return true
	}
	equalTo := func(name string) func(string) bool {
		return func(s string) bool { return foldEqual(name, s) }
	}
	if len(elems) == 1 && slices.ContainsFunc(watchTopFiles, equalTo(p)) {
		return true
	}
	name := elems[len(elems)-1]
	if slices.ContainsFunc(watchNames, equalTo(name)) {
		return true
	}
	return slices.ContainsFunc(watchAllSuffixes, func(s string) bool {
		return hasFoldSuffix(name, s)
	})
}

// foldUnit is one unit of a string as foldEqual compares it: a valid rune as
// its code point, which is not negative, or an invalid byte b as -1-b.
type foldUnit int32

// foldUnits splits s into its valid runes and its invalid bytes.
func foldUnits(s string) []foldUnit {
	units := make([]foldUnit, 0, len(s))
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			units = append(units, foldUnit(-1-int32(s[0])))
		} else {
			units = append(units, foldUnit(r))
		}
		s = s[size:]
	}
	return units
}

// sameFold reports whether two units are equal under Unicode simple case
// folding: two runes whose unicode.SimpleFold orbits meet (k, K and KELVIN
// SIGN; s, S and LONG S), or the same invalid byte. An invalid byte equals no
// rune, U+FFFD included.
func sameFold(a, b foldUnit) bool {
	if a == b {
		return true
	}
	if a < 0 || b < 0 {
		return false
	}
	for r := unicode.SimpleFold(rune(a)); r != rune(a); r = unicode.SimpleFold(r) {
		if r == rune(b) {
			return true
		}
	}
	return false
}

// foldEqual reports whether a and b are equal unit by unit under sameFold.
// strings.EqualFold is not used: it reads every invalid byte as U+FFFD and so
// equates different invalid byte sequences.
func foldEqual(a, b string) bool {
	return slices.EqualFunc(foldUnits(a), foldUnits(b), sameFold)
}

// hasFoldSuffix reports whether name ends with suffix under sameFold. The
// units are compared, not the bytes, since folded runes differ in length
// (KELVIN SIGN is three bytes, k one).
func hasFoldSuffix(name, suffix string) bool {
	n, s := foldUnits(name), foldUnits(suffix)
	return len(n) >= len(s) && slices.EqualFunc(n[len(n)-len(s):], s, sameFold)
}

// settingsLocalPath is always a target file, whether git ignores it, lists
// it, or it exists at all: the next Claude Code session runs its hooks.
const settingsLocalPath = ".claude/settings.local.json"

// listIgnoredArgs lists every ignored file, NUL separated. Without
// --directory git names each file of an ignored directory, so a .go file
// inside one is a path of its own; only a nested repository stays one item,
// with a trailing "/". --exclude-standard reads every .gitignore,
// info/exclude, and core.excludesFile, so a file hidden by a new ignore rule
// is listed too. ls-files runs no hook.
var listIgnoredArgs = []string{"ls-files", "--others", "--ignored", "--exclude-standard", "-z"}

// targetLimits bounds one pass over the target files. Production passes
// defaultTargetLimits; tests pass smaller values.
type targetLimits struct {
	files       int           // target files, settingsLocalPath included
	walkEntries int           // entries visited under directory items and .claude
	fileBytes   int64         // one regular file
	totalBytes  int64         // bytes read in one pass
	timeout     time.Duration // the whole pass, the listing included
}

// defaultTargetLimits leave room above what a work tree with
// extension/node_modules holds (a few target files, under 10 KB read), and
// above a single amalgamated C source (SQLite is about 9 MB) for one file.
var defaultTargetLimits = targetLimits{
	files:       maxTargetFiles,
	walkEntries: maxWalkEntries,
	fileBytes:   maxTargetFileBytes,
	totalBytes:  maxTargetReadBytes,
	timeout:     gitTimeout,
}

const (
	maxTargetFiles     = 10_000
	maxWalkEntries     = 100_000
	maxTargetFileBytes = 16 << 20
	maxTargetReadBytes = 64 << 20
	// execPermBits are the permission bits an entry records.
	execPermBits = 0o111
)

// The reasons a pass cannot record every target file. Each names its rule
// only, never a path, a name, or a value: stop prints the whole error chain,
// and a name is chosen by whoever wrote the file.
var (
	errListIgnoredFailed  = fmt.Errorf("%w: list ignored files failed", ErrGitFailed)
	errListIgnoredCap     = fmt.Errorf("%w: list ignored files output over the cap", ErrGitFailed)
	errListIgnoredShape   = fmt.Errorf("%w: list ignored files output shape", ErrGitFailed)
	errIgnoredTargetCount = fmt.Errorf("%w: ignored target count over the cap", ErrGitFailed)
	errIgnoredWalkEntries = fmt.Errorf("%w: ignored directory entries over the cap", ErrGitFailed)
	errIgnoredFileSize    = fmt.Errorf("%w: ignored file over the size cap", ErrGitFailed)
	errIgnoredReadTotal   = fmt.Errorf("%w: ignored read total over the cap", ErrGitFailed)
	errIgnoredTimeout     = fmt.Errorf("%w: read ignored files timed out", ErrGitFailed)
	errReadIgnoredDir     = fmt.Errorf("%w: read ignored directory", ErrGitFailed)
	errReadIgnoredFile    = fmt.Errorf("%w: read ignored file", ErrGitFailed)
	errIgnoredLinkToDir   = fmt.Errorf("%w: ignored link to a directory", ErrGitFailed)
	errClaudeGitName      = fmt.Errorf("%w: .git name below .claude", ErrGitFailed)
)

// targetState is what one pass over the target files yields: digest covers
// every target file's path and state, and record has one line per target
// file, its entry digest and its quoted path. Neither is printed; record is
// written only to a file in the git directory.
type targetState struct {
	digest string
	record []byte
}

// lstatFunc is the lstat a pass calls on each target file before it opens
// it; production passes os.Lstat.
type lstatFunc func(name string) (fs.FileInfo, error)

// collectTargets records the state of every target file, or stops with
// ErrGitFailed when it cannot record all of them; it never returns a partial
// state.
func (w *Worktree) collectTargets(ctx context.Context, lim targetLimits) (targetState, error) {
	return w.collectTargetsWith(ctx, lim, os.Lstat)
}

// collectTargetsWith runs the pass on its own goroutine and waits at most
// lim.timeout for it. A read that cannot be cancelled (a link into a hung
// network file system) keeps its goroutine after the wait ends; the result
// channel is buffered so that goroutine can still finish, and the process
// ends soon after. A cancelled ctx ends the wait the same way.
func (w *Worktree) collectTargetsWith(ctx context.Context, lim targetLimits, lstat lstatFunc) (targetState, error) {
	ctx, cancel := context.WithTimeout(ctx, lim.timeout)
	defer cancel()
	type result struct {
		state targetState
		err   error
	}
	done := make(chan result, 1)
	go func() {
		state, err := w.scanTargets(ctx, lim, lstat)
		done <- result{state, err}
	}()
	select {
	case r := <-done:
		return r.state, r.err
	case <-ctx.Done():
		return targetState{}, errIgnoredTimeout
	}
}

func (w *Worktree) scanTargets(ctx context.Context, lim targetLimits, lstat lstatFunc) (targetState, error) {
	out, err := w.listRun.Run(ctx, gitCommand, slices.Concat(gitHardening, listIgnoredArgs)...)
	if err != nil {
		return targetState{}, listIgnoredFailure(err)
	}
	files, dirs, err := parseIgnoredListing(out)
	if err != nil {
		return targetState{}, err
	}
	paths, err := selectTargets(ctx, files, dirs, lim)
	if err != nil {
		return targetState{}, err
	}
	return targetStateOf(ctx, paths, lim, lstat)
}

// listIgnoredFailure classifies a failed listing. A warning on standard
// error is a failure too: the runner sets StderrIsFailure.
func listIgnoredFailure(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return errIgnoredTimeout
	case errors.Is(err, cmdrunner.ErrOutputTooLarge):
		return errListIgnoredCap
	default:
		return errListIgnoredFailed
	}
}

// parseIgnoredListing accepts only the listing shape: items that each end in
// NUL, none empty and none twice, each a relative path whose elements are
// neither empty nor "." nor "..". An item with one trailing "/" is a
// directory item. Anything else fails the whole listing; nothing is
// repaired or skipped.
func parseIgnoredListing(out []byte) (files, dirs []string, err error) {
	if len(out) == 0 {
		return nil, nil, nil
	}
	if out[len(out)-1] != 0 {
		return nil, nil, errListIgnoredShape
	}
	seen := make(map[string]struct{})
	for item := range strings.SplitSeq(string(out[:len(out)-1]), "\x00") {
		if _, dup := seen[item]; dup {
			return nil, nil, errListIgnoredShape
		}
		seen[item] = struct{}{}
		p, isDir := strings.CutSuffix(item, "/")
		if !validListedPath(p) {
			return nil, nil, errListIgnoredShape
		}
		if isDir {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
	}
	return files, dirs, nil
}

// validListedPath accepts a relative path with no empty, ".", or ".."
// element; an empty path, a leading "/", and "//" each make an empty element.
func validListedPath(p string) bool {
	for elem := range strings.SplitSeq(p, "/") {
		if elem == "" || elem == "." || elem == ".." {
			return false
		}
	}
	return true
}

// selectTargets keeps the listed files on the watch list, walks every
// directory item for more, adds settingsLocalPath, and returns the paths in
// byte order.
func selectTargets(ctx context.Context, files, dirs []string, lim targetLimits) ([]string, error) {
	targets := map[string]struct{}{settingsLocalPath: {}}
	add := func(p string) error {
		if _, ok := targets[p]; ok {
			return nil
		}
		if len(targets) >= lim.files {
			return errIgnoredTargetCount
		}
		targets[p] = struct{}{}
		return nil
	}
	for _, p := range files {
		if isWatched(p) {
			if err := add(p); err != nil {
				return nil, err
			}
		}
	}
	walked := 0
	for _, dir := range dirs {
		if err := walkIgnoredDir(ctx, dir, &walked, lim, add); err != nil {
			return nil, err
		}
	}
	if err := checkClaudeGitNames(ctx, dirs, &walked, lim); err != nil {
		return nil, err
	}
	return slices.Sorted(maps.Keys(targets)), nil
}

// checkClaudeGitNames walks the top-level .claude directory, counting each
// entry against lim.walkEntries, and stops at any entry whose name folds to
// .git, unless its parent is a directory item (a nested repository git
// lists as ignored, which the walk above covered and this one skips). A
// nested repository git does not ignore is not skipped, so its .git stops
// the pass; git status shows it as untracked but not what changes inside.
// The literal name .claude is walked, the one Claude Code reads. git lists nothing below an entry so named,
// in any case when core.ignorecase is on, so a file there would be on the
// watch list and yet in no listing. A .claude that is not a directory is
// left to the listing.
func checkClaudeGitNames(ctx context.Context, dirs []string, walked *int, lim targetLimits) error {
	info, err := os.Lstat(watchTopDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return errReadIgnoredDir
	case !info.IsDir(), slices.Contains(dirs, watchTopDir):
		// Not a directory, or a nested repository walked above.
		return nil
	}
	return filepath.WalkDir(watchTopDir, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return errReadIgnoredDir
		case p == watchTopDir:
			return nil
		case ctx.Err() != nil:
			return errIgnoredTimeout
		case slices.Contains(dirs, p):
			// Walked above; a directory item replaced since then fails, as
			// in walkIgnoredDir.
			if !d.IsDir() {
				return errReadIgnoredDir
			}
			return filepath.SkipDir
		}
		*walked++
		if *walked > lim.walkEntries {
			return errIgnoredWalkEntries
		}
		if foldEqual(d.Name(), ".git") {
			return errClaudeGitName
		}
		return nil
	})
}

// walkIgnoredDir visits everything below a directory item (a nested
// repository git lists as one item) without following a link, counting each
// entry against lim.walkEntries, and adds the regular files and links on the
// watch list. A FIFO or another special file is left out, as git leaves it
// out of the listing.
func walkIgnoredDir(ctx context.Context, dir string, walked *int, lim targetLimits, add func(string) error) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return errReadIgnoredDir
		}
		if p == dir {
			if !d.IsDir() {
				return errReadIgnoredDir
			}
			return nil
		}
		if ctx.Err() != nil {
			return errIgnoredTimeout
		}
		*walked++
		if *walked > lim.walkEntries {
			return errIgnoredWalkEntries
		}
		if t := d.Type(); (t.IsRegular() || t&fs.ModeSymlink != 0) && isWatched(p) {
			return add(p)
		}
		return nil
	})
}

// targetStateOf builds each target file's entry in path order. The digest
// covers every entry; each record line has the digest of one entry and the
// path quoted with strconv.QuoteToASCII, so a path with a newline, a control
// character, or invalid UTF-8 stays on one printable line.
func targetStateOf(ctx context.Context, paths []string, lim targetLimits, lstat lstatFunc) (targetState, error) {
	all := sha256.New()
	var record bytes.Buffer
	var total int64
	for _, p := range paths {
		if ctx.Err() != nil {
			return targetState{}, errIgnoredTimeout
		}
		fields, err := targetEntry(p, lim, &total, lstat)
		if err != nil {
			return targetState{}, err
		}
		one := sha256.New()
		for _, f := range fields {
			writeField(all, f)
			writeField(one, f)
		}
		record.WriteString(hex.EncodeToString(one.Sum(nil)) + " " + strconv.QuoteToASCII(p) + "\n")
	}
	return targetState{digest: hex.EncodeToString(all.Sum(nil)), record: record.Bytes()}, nil
}

// entry starts with a kind marker and the number of values that follow, so
// no path or link text can make one entry read as another or move a
// boundary between entries.
func entry(kind string, values ...string) []string {
	return append([]string{kind, strconv.Itoa(len(values))}, values...)
}

// targetEntry is one target file's entry: "absent" and the path; "file", the
// path, the executable bits, and the content's SHA-256; "link", the path, the
// link text, and what the link resolves to (linkTarget); or "other", the
// path, and the type. Modification time, owner, and the read and write bits
// are left out: they change without changing what runs.
func targetEntry(p string, lim targetLimits, total *int64, lstat lstatFunc) ([]string, error) {
	info, err := lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		// Removed after the listing, or settingsLocalPath below a .claude
		// that is not a directory.
		return entry("absent", p), nil
	case err != nil:
		return nil, errReadIgnoredFile
	}
	switch mode := info.Mode(); {
	case mode.IsRegular():
		perm, sum, err := readTarget(p, false, lim, total)
		if err != nil {
			return nil, err
		}
		return entry("file", p, perm, sum), nil
	case mode&fs.ModeSymlink != 0:
		text, err := os.Readlink(p)
		if err != nil {
			return nil, errReadIgnoredFile
		}
		resolved, err := linkTarget(p, lim, total)
		if err != nil {
			return nil, err
		}
		return entry("link", append([]string{p, text}, resolved...)...), nil
	default:
		return entry("other", p, mode.Type().String()), nil
	}
}

// linkTarget is what a link resolves to, following every link on the way: a
// regular file's "file", executable bits, and SHA-256 read through the link;
// "other" and the type; or "unresolved" when nothing is there, the links
// loop, or an element is not a directory, so a later target is still a
// change. A directory stops the pass rather than recording what is below it,
// and so does any other failure.
func linkTarget(p string, lim targetLimits, total *int64) ([]string, error) {
	info, err := os.Stat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ELOOP), errors.Is(err, syscall.ENOTDIR):
		return []string{"unresolved"}, nil
	case err != nil:
		return nil, errReadIgnoredFile
	case info.IsDir():
		return nil, errIgnoredLinkToDir
	case info.Mode().IsRegular():
		perm, sum, err := readTarget(p, true, lim, total)
		if err != nil {
			return nil, err
		}
		return []string{"file", perm, sum}, nil
	default:
		return []string{"other", info.Mode().Type().String()}, nil
	}
}

func execBits(mode fs.FileMode) string {
	return strconv.FormatUint(uint64(mode.Perm()&execPermBits), 8)
}

// readTarget returns the executable bits and the SHA-256 of a regular
// file, both from the opened file, so a file swapped in after lstat cannot
// pair the old bits with new content. openRegular opens without blocking and
// checks the opened file, so a FIFO swapped in fails instead of hanging.
func readTarget(p string, follow bool, lim targetLimits, total *int64) (perm, sum string, err error) {
	f, err := openRegular(p, follow)
	if err != nil {
		return "", "", errReadIgnoredFile
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", "", errReadIgnoredFile
	}
	sum, err = hashCapped(f, info.Size(), lim, total)
	if err != nil {
		return "", "", err
	}
	return execBits(info.Mode()), sum, nil
}

// hashCapped hashes r, whose reported size is size. The size is checked
// against both caps before reading, and the read stops one byte past the
// smaller remaining cap, so content that grows past the reported size (a
// file extended meanwhile, a file system that reports 0) is not read on.
// total adds the bytes read, however many paths reach the same content.
func hashCapped(r io.Reader, size int64, lim targetLimits, total *int64) (string, error) {
	remaining := lim.totalBytes - *total
	switch {
	case size > lim.fileBytes:
		return "", errIgnoredFileSize
	case size > remaining:
		return "", errIgnoredReadTotal
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(r, min(lim.fileBytes, remaining)+1))
	*total += n
	switch {
	case err != nil:
		return "", errReadIgnoredFile
	case n > lim.fileBytes:
		return "", errIgnoredFileSize
	case *total > lim.totalBytes:
		return "", errIgnoredReadTotal
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
