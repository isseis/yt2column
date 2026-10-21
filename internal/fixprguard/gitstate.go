package fixprguard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

const (
	gitCommand = "git"
	// gitTimeout bounds every git call, including ls-remote and push.
	gitTimeout = 2 * time.Minute

	// baselineVersion prefixes the baseline token, so a token of another
	// shape is rejected rather than misread.
	baselineVersion = "v1"
	// baselineFields is the number of "."-separated fields after the version.
	baselineFields = 7
	// noRemoteBranch stands for a push target branch that does not exist yet.
	noRemoteBranch = "none"
	// digestLength is the length of a hex-encoded SHA-256.
	digestLength = 64
	// sha1OIDLength and sha256OIDLength are the hex lengths of a git object ID.
	sha1OIDLength   = 40
	sha256OIDLength = 64
	// maxGitFileBytes bounds the .git file and the commondir file read while
	// locating the git directories.
	maxGitFileBytes = 4096
	// defaultRemote is the push remote when no configuration names one.
	defaultRemote = "origin"
	// headsPrefix is the namespace of branch refs.
	headsPrefix = "refs/heads/"
)

// The parts of the git state a baseline holds, as the stop output names them.
const (
	PartHead   = "head"
	PartRemote = "remote"
	PartHooks  = "hooks"
	PartConfig = "config"
	PartInfo   = "info"
)

// gitHardening comes before every git subcommand the tool runs. It turns off
// the configuration that can launch a command during the operations the tool
// performs, so a changed setting cannot run code inside this process: the
// fsmonitor hook, a configured SSH command, the ext transport, and automatic
// gc and maintenance (which run hooks of their own). A pager and colors are
// off so output is parsed as git wrote it. Diff commands also pass
// --no-ext-diff and --no-textconv. The hooks themselves still run on commit
// and push; they are compared with the baseline before those commands.
var gitHardening = []string{
	"--no-pager",
	"-c", "core.fsmonitor=false",
	"-c", "core.sshCommand=ssh",
	"-c", "protocol.ext.allow=never",
	"-c", "gc.auto=0",
	"-c", "maintenance.auto=false",
	"-c", "color.ui=false",
}

// GitEnv returns environ without any inherited GIT_* variable, which could
// point git at another repository, index or configuration, and with optional
// locks and terminal prompts turned off. The runner a Worktree is given must
// run commands with this environment.
func GitEnv(environ []string) []string {
	added := []string{"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0"}
	env := make([]string, 0, len(environ)+len(added))
	for _, entry := range environ {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	return append(env, added...)
}

// Baseline holds the git state taken at begin: the HEAD commit, a digest of
// the branch HEAD is on, the push target's branch OID ("none" when the branch
// does not exist there) and a digest of the push target, and digests of the
// hooks, the configuration, and the info files. The session keeps it as a
// token and passes it back; it is parsed strictly, never repaired. The zero
// value is not usable.
type Baseline struct {
	head      string
	branch    string
	remoteOID string
	remoteID  string
	hooks     string
	config    string
	info      string
}

// newBaseline is the only constructor; ParseBaseline and Worktree.Baseline
// both use it.
func newBaseline(head, branch, remoteOID, remoteID, hooks, config, info string) Baseline {
	return Baseline{head: head, branch: branch, remoteOID: remoteOID, remoteID: remoteID, hooks: hooks, config: config, info: info}
}

// ParseBaseline parses a token String produced, rejecting any other shape
// with ErrInvalidInput.
func ParseBaseline(s string) (Baseline, error) {
	parts := strings.Split(s, ".")
	if len(parts) != baselineFields+1 || parts[0] != baselineVersion {
		return Baseline{}, fmt.Errorf("%w: baseline shape", ErrInvalidInput)
	}
	b := newBaseline(parts[1], parts[2], parts[3], parts[4], parts[5], parts[6], parts[7])
	if !validOID(b.head) || (b.remoteOID != noRemoteBranch && !validOID(b.remoteOID)) {
		return Baseline{}, fmt.Errorf("%w: baseline object ID", ErrInvalidInput)
	}
	for _, d := range []string{b.branch, b.remoteID, b.hooks, b.config, b.info} {
		if !validHex(d, digestLength) {
			return Baseline{}, fmt.Errorf("%w: baseline digest", ErrInvalidInput)
		}
	}
	return b, nil
}

// String returns the token form; it is empty for the zero Baseline.
func (b Baseline) String() string {
	if !b.usable() {
		return ""
	}
	return strings.Join([]string{baselineVersion, b.head, b.branch, b.remoteOID, b.remoteID, b.hooks, b.config, b.info}, ".")
}

// MarshalText writes the token form.
func (b Baseline) MarshalText() ([]byte, error) { return []byte(b.String()), nil }

func (b Baseline) usable() bool { return b.head != "" }

// validOID accepts a lowercase hex SHA-1 or SHA-256 object ID.
func validOID(s string) bool {
	return validHex(s, sha1OIDLength) || validHex(s, sha256OIDLength)
}

func validHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := range len(s) {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// GitStateError reports which parts of the git state differ from the
// baseline. It matches ErrGitStateChanged.
type GitStateError struct {
	Parts []string
}

// Error names the differing parts; the part names are fixed words.
func (e *GitStateError) Error() string {
	return ErrGitStateChanged.Error() + ": " + strings.Join(e.Parts, ",")
}

// Unwrap returns ErrGitStateChanged.
func (e *GitStateError) Unwrap() error { return ErrGitStateChanged }

// gitDirs are the per-worktree git directory and the common one. They are
// the same outside a linked worktree.
type gitDirs struct {
	gitDir    string
	commonDir string
}

// locateGitDirs finds the git directories from the files alone, without
// running git: a .git directory in the current directory, or a .git file
// naming a linked worktree's directory, whose commondir file names the shared
// one. The current directory must be the top of the work tree.
func locateGitDirs() (gitDirs, error) {
	info, err := os.Lstat(".git")
	if err != nil {
		return gitDirs{}, fmt.Errorf("%w: not at the top of a work tree", ErrInvalidInput)
	}
	if info.IsDir() {
		return gitDirs{gitDir: ".git", commonDir: ".git"}, nil
	}
	if !info.Mode().IsRegular() {
		return gitDirs{}, fmt.Errorf("%w: .git is not a directory or a file", ErrInvalidInput)
	}
	content, err := readSmallFile(".git")
	if err != nil {
		return gitDirs{}, err
	}
	gitDir, ok := strings.CutPrefix(strings.TrimSuffix(content, "\n"), "gitdir: ")
	if !ok || gitDir == "" || strings.ContainsAny(gitDir, "\n\x00") {
		return gitDirs{}, fmt.Errorf("%w: .git file shape", ErrInvalidInput)
	}
	dirs := gitDirs{gitDir: gitDir, commonDir: gitDir}
	common, err := readSmallFile(filepath.Join(gitDir, "commondir"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return gitDirs{}, err
	default:
		common = strings.TrimSuffix(common, "\n")
		if common == "" || strings.ContainsAny(common, "\n\x00") {
			return gitDirs{}, fmt.Errorf("%w: commondir file shape", ErrInvalidInput)
		}
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitDir, common)
		}
		dirs.commonDir = common
	}
	return dirs, nil
}

// readSmallFile reads a regular file of at most maxGitFileBytes.
func readSmallFile(name string) (string, error) {
	f, err := os.Open(name) //nolint:gosec // a git metadata file of the current work tree
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: git metadata file is not a regular file", ErrInvalidInput)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxGitFileBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxGitFileBytes {
		return "", fmt.Errorf("%w: git metadata file size", ErrInvalidInput)
	}
	return string(data), nil
}

// configDigest covers where the git directories are and the repository and
// worktree configuration files. Recording the locations means a .git file
// rewritten to point at a copy of the repository is a config difference.
func configDigest(dirs gitDirs) (string, error) {
	h := sha256.New()
	for _, dir := range []string{dirs.gitDir, dirs.commonDir} {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		writeField(h, abs)
	}
	for _, name := range []string{filepath.Join(dirs.commonDir, "config"), filepath.Join(dirs.gitDir, "config.worktree")} {
		if err := digestPath(h, name); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// infoDigest covers info/exclude and info/attributes, which can hide files
// from git status.
func infoDigest(dirs gitDirs) (string, error) {
	h := sha256.New()
	for _, name := range []string{filepath.Join(dirs.commonDir, "info", "exclude"), filepath.Join(dirs.commonDir, "info", "attributes")} {
		if err := digestPath(h, name); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hooksDigest covers every entry under the hooks directory: its relative
// path, type, permission bits, symbolic link target, and regular file
// content. Changing only an existing hook's executable bit changes it, since
// that alone makes git run the hook.
func hooksDigest(dir string) (string, error) {
	h := sha256.New()
	root := dir
	info, err := os.Lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		writeField(h, "absent")
		return hex.EncodeToString(h.Sum(nil)), nil
	case err != nil:
		return "", err
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(dir)
		if err != nil {
			return "", err
		}
		writeField(h, "link")
		writeField(h, target)
		if root, err = filepath.EvalSymlinks(dir); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return hex.EncodeToString(h.Sum(nil)), nil
			}
			return "", err
		}
	}
	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		writeField(h, filepath.ToSlash(rel))
		return digestEntry(h, name, entry)
	})
	if err != nil {
		return "", fmt.Errorf("read hooks: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// digestEntry writes the type and permission bits of an entry WalkDir found
// (without following a link), and its link target or content.
func digestEntry(h hash.Hash, name string, entry fs.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	mode := info.Mode()
	writeField(h, mode.Type().String())
	writeField(h, strconv.FormatUint(uint64(mode.Perm()), 8))
	switch {
	case mode&fs.ModeSymlink != 0:
		target, err := os.Readlink(name)
		if err != nil {
			return err
		}
		writeField(h, target)
	case mode.IsRegular():
		sum, err := fileSHA256(name)
		if err != nil {
			return err
		}
		writeField(h, sum)
	}
	return nil
}

// digestPath writes a marker for a missing file, or the link target and the
// content of a file, following a link.
func digestPath(h hash.Hash, name string) error {
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		writeField(h, "absent")
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(name)
		if err != nil {
			return err
		}
		writeField(h, "link")
		writeField(h, target)
		if info, err = os.Stat(name); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				writeField(h, "dangling")
				return nil
			}
			return err
		}
	}
	writeField(h, info.Mode().Type().String())
	if !info.Mode().IsRegular() {
		return nil
	}
	sum, err := fileSHA256(name)
	if err != nil {
		return err
	}
	writeField(h, sum)
	return nil
}

// fileSHA256 hashes a file's content. The caller has checked that it is a
// regular file, so opening it cannot block on a FIFO.
func fileSHA256(name string) (string, error) {
	f, err := os.Open(name) //nolint:gosec // a git metadata file of the current work tree
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func digestString(fields ...string) string {
	h := sha256.New()
	for _, f := range fields {
		writeField(h, f)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// git runs a git subcommand behind gitHardening under gitTimeout. A failure
// becomes ErrGitFailed naming the rule only; the command's arguments and
// standard error are dropped.
func (w *Worktree) git(ctx context.Context, rule string, args ...string) ([]byte, error) {
	out, err := w.gitRaw(ctx, args...)
	if err != nil {
		return nil, gitFailure(rule, err)
	}
	return out, nil
}

func (w *Worktree) gitRaw(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	return w.run.Run(ctx, gitCommand, append(append([]string{}, gitHardening...), args...)...)
}

func gitFailure(rule string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %s timed out", ErrGitFailed, rule)
	}
	if errors.Is(err, cmdrunner.ErrOutputTooLarge) {
		return fmt.Errorf("%w: %s output over the cap", ErrGitFailed, rule)
	}
	return fmt.Errorf("%w: %s failed", ErrGitFailed, rule)
}

// gitConfig reads one configuration value; ok is false when it is not set.
func (w *Worktree) gitConfig(ctx context.Context, key string) (value string, ok bool, err error) {
	out, err := w.gitRaw(ctx, "config", "--get", key)
	if err != nil {
		if exitErr, isExit := errors.AsType[*cmdrunner.ExitError](err); isExit && exitErr.Code == 1 {
			return "", false, nil
		}
		return "", false, gitFailure("config", err)
	}
	return strings.TrimSuffix(string(out), "\n"), true, nil
}

// fileDigests computes the config and info digests from the files alone.
func fileDigests(dirs gitDirs) (config, info string, err error) {
	if config, err = configDigest(dirs); err != nil {
		return "", "", fmt.Errorf("%w: read config: %w", ErrGitFailed, err)
	}
	if info, err = infoDigest(dirs); err != nil {
		return "", "", fmt.Errorf("%w: read info: %w", ErrGitFailed, err)
	}
	return config, info, nil
}

// hooksDir asks git where the hooks are (core.hooksPath or the common git
// directory's hooks). rev-parse runs no hook, and CheckState calls it only
// after the configuration that decides the answer matched the baseline.
func (w *Worktree) hooksDir(ctx context.Context) (string, error) {
	out, err := w.git(ctx, "rev-parse", "rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", err
	}
	dir := strings.TrimSuffix(string(out), "\n")
	if dir == "" || strings.ContainsAny(dir, "\n\x00") {
		return "", fmt.Errorf("%w: rev-parse output", ErrGitFailed)
	}
	return dir, nil
}

// CheckState compares the configuration, info and hooks with the baseline
// before git or gh is launched. The configuration and info are read from the
// files alone, so a changed setting stops the run before any git process
// could read it; the hooks are compared next, before any command that could
// run one. Every subcommand that launches git or gh after begin calls it
// first.
func (w *Worktree) CheckState(ctx context.Context, base Baseline) error {
	if !base.usable() {
		return fmt.Errorf("%w: baseline", ErrInvalidInput)
	}
	dirs, err := locateGitDirs()
	if err != nil {
		return err
	}
	config, info, err := fileDigests(dirs)
	if err != nil {
		return err
	}
	var parts []string
	if config != base.config {
		parts = append(parts, PartConfig)
	}
	if info != base.info {
		parts = append(parts, PartInfo)
	}
	if parts != nil {
		return &GitStateError{Parts: parts}
	}
	dir, err := w.hooksDir(ctx)
	if err != nil {
		return err
	}
	hooks, err := hooksDigest(dir)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrGitFailed, err)
	}
	if hooks != base.hooks {
		return &GitStateError{Parts: []string{PartHooks}}
	}
	return nil
}

// headState reads the HEAD commit and the branch HEAD is on. A detached HEAD
// has no branch to push and is refused.
func (w *Worktree) headState(ctx context.Context) (oid, branch string, err error) {
	out, err := w.git(ctx, "rev-parse HEAD", "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", "", err
	}
	oid = strings.TrimSuffix(string(out), "\n")
	if !validOID(oid) {
		return "", "", fmt.Errorf("%w: rev-parse output", ErrGitFailed)
	}
	out, err = w.gitRaw(ctx, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 {
			return oid, "", nil
		}
		return "", "", gitFailure("symbolic-ref", err)
	}
	ref := strings.TrimSuffix(string(out), "\n")
	branch, ok := strings.CutPrefix(ref, headsPrefix)
	if !ok || branch == "" || strings.ContainsAny(branch, "\n\x00") {
		return oid, "", nil
	}
	return oid, branch, nil
}

// pushTarget is where push sends the branch: the remote git resolves for the
// branch (branch.<b>.pushRemote, remote.pushDefault, branch.<b>.remote, then
// origin), its push URL, and the branch of the same name there.
type pushTarget struct {
	remote string
	url    string
	branch string
}

func (p pushTarget) id() string { return digestString(p.remote, p.url, p.branch) }

func (w *Worktree) resolvePushTarget(ctx context.Context, branch string) (pushTarget, error) {
	remote := ""
	for _, key := range []string{"branch." + branch + ".pushRemote", "remote.pushDefault", "branch." + branch + ".remote"} {
		value, ok, err := w.gitConfig(ctx, key)
		if err != nil {
			return pushTarget{}, err
		}
		if ok && value != "" {
			remote = value
			break
		}
	}
	if remote == "" {
		remote = defaultRemote
	}
	// A remote named "." or like an option would not be a remote.
	if remote == "." || strings.HasPrefix(remote, "-") || strings.ContainsAny(remote, "\n\x00") {
		return pushTarget{}, fmt.Errorf("%w: push remote", ErrGitFailed)
	}
	out, err := w.git(ctx, "remote get-url", "remote", "get-url", "--push", remote)
	if err != nil {
		return pushTarget{}, err
	}
	url := strings.TrimSuffix(string(out), "\n")
	if url == "" || strings.ContainsAny(url, "\n\x00") {
		return pushTarget{}, fmt.Errorf("%w: push URL", ErrGitFailed)
	}
	return pushTarget{remote: remote, url: url, branch: branch}, nil
}

// remoteOID reads the push target branch's OID with ls-remote, or
// noRemoteBranch when it does not exist. ls-remote matches patterns by
// suffix, so only the exact ref name is read.
func (w *Worktree) remoteOID(ctx context.Context, target pushTarget) (string, error) {
	ref := headsPrefix + target.branch
	out, err := w.git(ctx, "ls-remote", "ls-remote", "--", target.remote, ref)
	if err != nil {
		return "", err
	}
	oid := noRemoteBranch
	for line := range strings.Lines(string(out)) {
		got, name, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		if !ok || name != ref {
			continue
		}
		if oid != noRemoteBranch || !validOID(got) {
			return "", fmt.Errorf("%w: ls-remote output", ErrGitFailed)
		}
		oid = got
	}
	return oid, nil
}

// checkIndexFlags refuses an index entry with skip-worktree or
// assume-unchanged set: either hides a changed file from git status.
// ls-files -v tags such entries with S or a lowercase letter.
func (w *Worktree) checkIndexFlags(ctx context.Context) error {
	out, err := w.git(ctx, "ls-files", "ls-files", "-v", "-z")
	if err != nil {
		return err
	}
	for entry := range bytes.SplitSeq(out, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		if tag := entry[0]; tag == 'S' || ('a' <= tag && tag <= 'z') {
			return &GitStateError{Parts: []string{PartInfo}}
		}
	}
	return nil
}

// Baseline takes the git state the later subcommands compare with. It stops
// when HEAD is not on a branch or an index entry hides changes, because the
// later checks could not see past either.
func (w *Worktree) Baseline(ctx context.Context) (Baseline, error) {
	dirs, err := locateGitDirs()
	if err != nil {
		return Baseline{}, err
	}
	config, info, err := fileDigests(dirs)
	if err != nil {
		return Baseline{}, err
	}
	dir, err := w.hooksDir(ctx)
	if err != nil {
		return Baseline{}, err
	}
	hooks, err := hooksDigest(dir)
	if err != nil {
		return Baseline{}, fmt.Errorf("%w: %w", ErrGitFailed, err)
	}
	if err := w.checkIndexFlags(ctx); err != nil {
		return Baseline{}, err
	}
	head, branch, err := w.headState(ctx)
	if err != nil {
		return Baseline{}, err
	}
	if branch == "" {
		return Baseline{}, fmt.Errorf("%w: HEAD is not on a branch", ErrInvalidInput)
	}
	target, err := w.resolvePushTarget(ctx, branch)
	if err != nil {
		return Baseline{}, err
	}
	remote, err := w.remoteOID(ctx, target)
	if err != nil {
		return Baseline{}, err
	}
	return newBaseline(head, digestString(branch), remote, target.id(), hooks, config, info), nil
}

// stateCheck selects the parts compareState reads besides the files.
type stateCheck int

const (
	// checkLocal compares HEAD and the index flags.
	checkLocal stateCheck = iota
	// checkWithRemote also compares the push target and its branch OID.
	checkWithRemote
)

// compareState runs CheckState and then compares HEAD, the branch, and the
// index flags, plus the push target when check asks for it. It returns the
// HEAD commit.
func (w *Worktree) compareState(ctx context.Context, base Baseline, check stateCheck) (string, error) {
	if err := w.CheckState(ctx, base); err != nil {
		return "", err
	}
	if err := w.checkIndexFlags(ctx); err != nil {
		return "", err
	}
	head, branch, err := w.headState(ctx)
	if err != nil {
		return "", err
	}
	var parts []string
	if head != base.head || branch == "" || digestString(branch) != base.branch {
		parts = append(parts, PartHead)
	}
	if check == checkWithRemote && branch != "" {
		target, err := w.resolvePushTarget(ctx, branch)
		if err != nil {
			return "", err
		}
		oid, err := w.remoteOID(ctx, target)
		if err != nil {
			return "", err
		}
		if target.id() != base.remoteID || oid != base.remoteOID {
			parts = append(parts, PartRemote)
		}
	}
	if parts != nil {
		return "", &GitStateError{Parts: parts}
	}
	return head, nil
}
