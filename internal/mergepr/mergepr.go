// Package mergepr implements the mechanics behind the /mergepr command:
// resolving and verifying the repository, the PR, and its refs once, then
// merging and cleaning up. Values never pass through a shell, refs are
// fully-qualified, every remote operation uses a pinned URL rather than the
// remote name, and every mutable value is re-verified immediately before the
// irreversible operation that consumes it.
package mergepr

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	gitCommand = "git"
	ghCommand  = "gh"

	originRemote  = "origin"
	trustedBranch = "main"
	commandFile   = ".claude/commands/mergepr.md"
	refsHeads     = "refs/heads/"
	refsRemotes   = "refs/remotes/"
	originRefs    = refsRemotes + originRemote + "/"
	// refsWildcard maps the remote's branches into this repository's own
	// remote-tracking namespace and forces only those destinations, so a
	// configured refspec cannot rewrite a local branch and a force-pushed
	// branch can still update its remote-tracking ref.
	refsWildcard = "+" + refsHeads + "*:" + originRefs + "*"
	githubHost   = "github.com"

	repoFlag = "-R"
	jsonFlag = "--json"

	openState   = "OPEN"
	mergedState = "MERGED"
	oidLength   = 40

	stateFileName = "state.json"
	logFileName   = "log.txt"
	statFileName  = "stat.txt"
	bodyFileName  = "body.txt"

	maxLogBytes     = 256 << 10
	maxStatBytes    = 64 << 10
	maxBodyBytes    = 64 << 10
	maxSubjectBytes = 4 << 10
	maxDiffBytes    = 256 << 10

	// commandTimeout bounds a single git or gh call. checkTimeout is the
	// larger budget gh pr checks --watch may legitimately need for a running CI.
	commandTimeout = 5 * time.Minute
	checksTimeout  = 30 * time.Minute

	stateFileMode = 0o600
	workDirMode   = 0o700

	prViewFields  = "number,title,state,headRefName,headRefOid,baseRefName,isCrossRepository,url,body"
	mergeViewJSON = "state,headRefName,headRefOid,baseRefName,isCrossRepository,mergeCommit"
)

// repoPartPattern is the character set GitHub allows in an owner or repo name.
var repoPartPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// Runner runs an external command without a shell and returns its standard
// output. Tests substitute a scripted fake; production uses NewOSRunner.
type Runner interface {
	// Run executes name with args and returns standard output. A non-zero exit
	// is an error that identifies the command and includes its standard error.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Tool implements the /mergepr mechanics against the repository in the current
// working directory.
type Tool struct {
	run Runner
}

// New returns a Tool that runs external commands through runner.
func New(runner Runner) (*Tool, error) {
	if runner == nil {
		return nil, errNoRunner
	}
	return &Tool{run: runner}, nil
}

// command runs one external command with a deadline, so a stalled git or gh
// call cannot hang the workflow forever.
func (t *Tool) command(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return t.run.Run(ctx, name, args...)
}

// identity is a repository resolved once and pinned: the owner and repo name,
// origin's validated fetch URL, and one validated push URL. Remote operations
// use these URLs, never the remote name, so ambient configuration cannot
// redirect them.
type identity struct {
	Owner    string
	Repo     string
	FetchURL string
	PushURL  string
}

func (id identity) repo() string { return id.Owner + "/" + id.Repo }

// State pins the values Prepare verified so that Merge and Cleanup consume the
// same form instead of re-resolving it from configuration or live state.
type State struct {
	Number      int    `json:"number"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	HeadRefName string `json:"headRefName"`
	HeadRefOID  string `json:"headRefOid"`
	BaseRefName string `json:"baseRefName"`
	Title       string `json:"title"`
	URL         string `json:"url"`
}

func (s State) repo() string { return s.Owner + "/" + s.Repo }

func (s State) validate() error {
	if s.Number <= 0 || !repoPartPattern.MatchString(s.Owner) || !repoPartPattern.MatchString(s.Repo) {
		return fmt.Errorf("%w: number, owner, or repo", errInvalidState)
	}
	if err := checkRefNameSyntax(s.HeadRefName); err != nil {
		return fmt.Errorf("%w: head: %w", errInvalidState, err)
	}
	if err := checkRefNameSyntax(s.BaseRefName); err != nil {
		return fmt.Errorf("%w: base: %w", errInvalidState, err)
	}
	if _, err := hex.DecodeString(s.HeadRefOID); err != nil || len(s.HeadRefOID) != oidLength {
		return fmt.Errorf("%w: headRefOid", errInvalidState)
	}
	return nil
}

// Prepared is the result of Prepare: the pinned state and the files holding the
// drafting material.
type Prepared struct {
	State     State
	StatePath string
	LogPath   string
	StatPath  string
	BodyPath  string
}

// Report describes what Merge or Cleanup changed.
type Report struct {
	MergeCommitOID string
	RemoteDeleted  bool
	LocalDeleted   bool
	BaseUpdated    bool
}

// Prepare verifies the repository, PR, refs, worktree, and CI, fetches origin,
// and writes the pinned state and drafting material into workDir (a fresh
// temporary directory when workDir is empty). It stops instead of truncating an
// input that does not fit.
func (t *Tool) Prepare(ctx context.Context, prArg, workDir string) (Prepared, error) {
	created := false
	if workDir == "" {
		dir, err := os.MkdirTemp("", "mergepr-")
		if err != nil {
			return Prepared{}, fmt.Errorf("create work directory: %w", err)
		}
		workDir = dir
		created = true
	}
	// A directory inside the checkout, including one MkdirTemp created under an
	// in-repo TMPDIR, would make the files Prepare writes dirty the worktree, so
	// Merge's clean-worktree recheck would reject the state Prepare produced.
	if err := requireOutsideWorktree(workDir); err != nil {
		if created {
			_ = os.RemoveAll(workDir)
		}
		return Prepared{}, err
	}
	if !created {
		// Create a named directory now, not after the CI wait, so writing the
		// prepared files cannot be the step that fails.
		if err := os.MkdirAll(workDir, workDirMode); err != nil {
			return Prepared{}, fmt.Errorf("create work directory: %w", err)
		}
	}
	// MkdirAll leaves an existing directory's permissions unchanged, so a
	// group- or world-writable work directory could let another local user
	// pre-create or race a predictable output name as a symlink. Require a
	// private directory owned by this user before writing anything into it.
	if err := requirePrivateDir(workDir); err != nil {
		if created {
			_ = os.RemoveAll(workDir)
		}
		return Prepared{}, err
	}
	prepared, err := t.prepare(ctx, prArg, workDir)
	if err != nil {
		if created {
			_ = os.RemoveAll(workDir)
		}
		return Prepared{}, err
	}
	return prepared, nil
}

func (t *Tool) prepare(ctx context.Context, prArg, workDir string) (Prepared, error) {
	id, pr, err := t.resolvePR(ctx, prArg)
	if err != nil {
		return Prepared{}, err
	}
	if err := t.requireToolUnchanged(ctx, id, pr.BaseRefName, pr.HeadRefName, pr.HeadRefOID); err != nil {
		return Prepared{}, err
	}
	logOut, statOut, bodyOut, err := t.draftingMaterial(ctx, id, pr)
	if err != nil {
		return Prepared{}, err
	}
	return writePrepared(workDir, id, pr, logOut, statOut, bodyOut)
}

// requireOutsideWorktree rejects a work directory inside the repository
// worktree, where the files Prepare writes would make the checkout dirty. Both
// paths have their symlinks resolved, so an outside symlink that points into
// the checkout is rejected too.
func requireOutsideWorktree(workDir string) error {
	root, err := worktreeRoot()
	if err != nil {
		return err
	}
	resolvedRoot, err := resolveExisting(root)
	if err != nil {
		return err
	}
	resolvedWork, err := resolveExisting(workDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedWork)
	if err != nil {
		return nil // a different volume cannot be inside the worktree
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("%w: %s", errWorkDirInWorktree, workDir)
	}
	return nil
}

// requirePrivateDir rejects a work directory that other local users can write
// to, or that a previous run created under a different account, because either
// lets them pre-create a predictable output name as a symlink.
func requirePrivateDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect work directory: %w", err)
	}
	if !info.IsDir() || info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%w: %s", errInsecureWorkDir, path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("%w: %s", errInsecureWorkDir, path)
	}
	return nil
}

// resolveExisting resolves symlinks on the longest existing prefix of path and
// appends the rest unchanged, so a not-yet-created work directory is still
// checked against its real parent.
func resolveExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	rest := ""
	current := abs
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(resolved, rest), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(current), rest)
		current = parent
	}
}

// worktreeRoot walks up from the working directory to the nearest ancestor that
// contains a .git entry, which is the root of this worktree.
func worktreeRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errNoWorktree
		}
		dir = parent
	}
}

// requireToolUnchanged refuses to run when the command definition in this
// checkout is not the trusted main revision, and refuses to merge a PR that
// changes the merge tool or its command definition. The PR check compares the
// PR's pinned head commit with its merge base (three-dot), so it detects only
// changes the PR introduced, not unrelated changes that landed on the base.
func (t *Tool) requireToolUnchanged(ctx context.Context, id identity, base, head, headOID string) error {
	refspecs := []string{
		"+" + refsHeads + base + ":" + originRefs + base,
		"+" + refsHeads + head + ":" + originRefs + head,
	}
	if base != trustedBranch {
		refspecs = append(refspecs, "+"+refsHeads+trustedBranch+":"+originRefs+trustedBranch)
	}
	if _, err := t.command(ctx, commandTimeout, gitCommand, append([]string{"fetch", id.FetchURL}, refspecs...)...); err != nil {
		return fmt.Errorf("fetch refs for tool check: %w", err)
	}
	// The command definition that drives this tool must be the trusted main
	// revision, so a checkout on a branch that edited it cannot run it.
	// git diff --quiet exits 1 for a difference and 128 for a failure such as a
	// missing object; keep git's error so a failure is not misread as a change.
	if _, err := t.command(ctx, commandTimeout, gitCommand, append([]string{"diff", "--quiet", originRefs + trustedBranch, "--"}, topPathspecs(commandFile)...)...); err != nil {
		return fmt.Errorf("%w: %w", errCommandChanged, err)
	}
	if _, err := t.command(ctx, commandTimeout, gitCommand, append([]string{"diff", "--quiet", originRefs + base + "..." + headOID, "--"}, topPathspecs("cmd/mergepr", "internal/mergepr", commandFile)...)...); err != nil {
		return fmt.Errorf("%w: %w", errToolChanged, err)
	}
	return nil
}

// topPathspecs anchors repository-root-relative paths at the worktree root.
// Git resolves a plain pathspec against the current directory, so a guard run
// from a subdirectory would otherwise match nothing and pass.
func topPathspecs(paths ...string) []string {
	specs := make([]string, len(paths))
	for i, path := range paths {
		specs[i] = ":(top)" + path
	}
	return specs
}

// resolvePR verifies everything about the PR itself before any network
// mutation: repository identity, PR state, ref-name safety, and the local
// worktree, HEAD, and head branch.
func (t *Tool) resolvePR(ctx context.Context, prArg string) (identity, prInfo, error) {
	id, err := t.repoIdentity(ctx)
	if err != nil {
		return identity{}, prInfo{}, err
	}
	number, useCurrent, err := parsePRArg(prArg, id.Owner, id.Repo)
	if err != nil {
		return identity{}, prInfo{}, err
	}
	// gh refuses to infer the current branch's PR when -R is given, so name the
	// branch explicitly.
	branch := ""
	if useCurrent {
		if branch, err = t.currentBranch(ctx); err != nil {
			return identity{}, prInfo{}, err
		}
	}
	pr, err := t.fetchPR(ctx, id.Owner, id.Repo, number, branch)
	if err != nil {
		return identity{}, prInfo{}, err
	}
	if pr.State != openState {
		return identity{}, prInfo{}, fmt.Errorf("%w: state is %s", errPRNotOpen, pr.State)
	}
	if pr.IsCrossRepository {
		return identity{}, prInfo{}, errCrossRepository
	}
	for _, name := range []string{pr.HeadRefName, pr.BaseRefName} {
		if err := checkRefNameSyntax(name); err != nil {
			return identity{}, prInfo{}, err
		}
		if err := t.gitCheckRefFormat(ctx, name); err != nil {
			return identity{}, prInfo{}, err
		}
	}
	if err := t.requireCleanWorktree(ctx); err != nil {
		return identity{}, prInfo{}, err
	}
	// Switching branches later must not orphan a detached tip, so require HEAD
	// to be attached before anything else is touched.
	if err := t.requireBaseSwitchable(ctx, pr.BaseRefName); err != nil {
		return identity{}, prInfo{}, err
	}
	localOID, err := t.localHeadOID(ctx, pr.HeadRefName)
	if err != nil {
		return identity{}, prInfo{}, err
	}
	if localOID != "" && localOID != pr.HeadRefOID {
		return identity{}, prInfo{}, fmt.Errorf("%w: %s is %s, want %s", errHeadBranchMismatch, pr.HeadRefName, localOID, pr.HeadRefOID)
	}
	return id, pr, nil
}

// draftingMaterial fetches into this repository's own remote-tracking refs,
// waits for CI, and reads the bounded commit log, diff stat, and PR body the
// message is drafted from.
func (t *Tool) draftingMaterial(ctx context.Context, id identity, pr prInfo) ([]byte, []byte, []byte, error) {
	if _, err := t.command(ctx, commandTimeout, gitCommand, "fetch", id.FetchURL, refsWildcard); err != nil {
		return nil, nil, nil, fmt.Errorf("fetch origin: %w", err)
	}
	if _, err := t.command(ctx, checksTimeout, ghCommand, "pr", "checks", strconv.Itoa(pr.Number), "--watch", "--fail-fast", repoFlag, id.repo()); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", errChecksFailed, err)
	}
	baseRef := originRefs + pr.BaseRefName
	logOut, err := t.command(ctx, commandTimeout, gitCommand, "log", "--no-show-signature", "--format=%h %s%n%n%b", baseRef+".."+pr.HeadRefOID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read commit log: %w", err)
	}
	if len(logOut) > maxLogBytes {
		return nil, nil, nil, fmt.Errorf("%w: commit log is %d bytes, limit %d", errTooLarge, len(logOut), maxLogBytes)
	}
	statOut, err := t.command(ctx, commandTimeout, gitCommand, "diff", "--stat", baseRef+"..."+pr.HeadRefOID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read diff stat: %w", err)
	}
	if len(statOut) > maxStatBytes {
		return nil, nil, nil, fmt.Errorf("%w: diff stat is %d bytes, limit %d", errTooLarge, len(statOut), maxStatBytes)
	}
	bodyOut := []byte(pr.Body)
	if len(bodyOut) > maxBodyBytes {
		return nil, nil, nil, fmt.Errorf("%w: PR body is %d bytes, limit %d", errTooLarge, len(bodyOut), maxBodyBytes)
	}
	return logOut, statOut, bodyOut, nil
}

func writePrepared(workDir string, id identity, pr prInfo, logOut, statOut, bodyOut []byte) (Prepared, error) {
	state := State{
		Number:      pr.Number,
		Owner:       id.Owner,
		Repo:        id.Repo,
		HeadRefName: pr.HeadRefName,
		HeadRefOID:  pr.HeadRefOID,
		BaseRefName: pr.BaseRefName,
		Title:       pr.Title,
		URL:         pr.URL,
	}
	statePath := filepath.Join(workDir, stateFileName)
	if err := writeState(statePath, state); err != nil {
		return Prepared{}, err
	}
	logPath := filepath.Join(workDir, logFileName)
	if err := writeFile(logPath, logOut); err != nil {
		return Prepared{}, err
	}
	statPath := filepath.Join(workDir, statFileName)
	if err := writeFile(statPath, statOut); err != nil {
		return Prepared{}, err
	}
	bodyPath := filepath.Join(workDir, bodyFileName)
	if err := writeFile(bodyPath, bodyOut); err != nil {
		return Prepared{}, err
	}
	return Prepared{State: state, StatePath: statePath, LogPath: logPath, StatPath: statPath, BodyPath: bodyPath}, nil
}

// Merge re-verifies the PR against the pinned state immediately before the
// irreversible merge — including once more after the CI wait — merges with
// --match-head-commit, and cleans up. When the PR is already MERGED it skips
// the merge and resumes cleanup.
func (t *Tool) Merge(ctx context.Context, statePath, subjectPath, bodyPath string) (Report, error) {
	state, err := loadState(statePath)
	if err != nil {
		return Report{}, err
	}
	subject, err := readSubject(subjectPath)
	if err != nil {
		return Report{}, err
	}
	// Pin the approved body now: the CI wait below is long enough for the file
	// to change before the merge reads it.
	body, err := readBody(bodyPath)
	if err != nil {
		return Report{}, err
	}
	id, err := t.requireIdentity(ctx, state)
	if err != nil {
		return Report{}, err
	}
	// The message files are written outside the worktree, but the pause before
	// the merge is long enough for the worktree to change; refuse to merge and
	// then be unable to clean up.
	if err := t.requireCleanWorktree(ctx); err != nil {
		return Report{}, err
	}
	if err := t.requireBaseSwitchable(ctx, state.BaseRefName); err != nil {
		return Report{}, err
	}
	live, err := t.fetchMergeView(ctx, state)
	if err != nil {
		return Report{}, err
	}
	if live.State == mergedState {
		return t.cleanupAfterMerge(ctx, state, &live, &id)
	}
	if err := verifyMergeable(live, state); err != nil {
		return Report{}, err
	}
	if _, err := t.command(ctx, checksTimeout, ghCommand, "pr", "checks", strconv.Itoa(state.Number), "--watch", "--fail-fast", repoFlag, state.repo()); err != nil {
		return Report{}, fmt.Errorf("%w: %w", errChecksFailed, err)
	}
	// The CI wait can be long. Re-read the PR after it and re-check everything
	// before the irreversible merge, because --match-head-commit pins only the
	// head OID.
	after, err := t.fetchMergeView(ctx, state)
	if err != nil {
		return Report{}, err
	}
	if err := verifyMergeable(after, state); err != nil {
		return Report{}, err
	}
	// The CI wait can be long, so re-check the local prerequisites that a
	// concurrent change could have invalidated before the irreversible merge.
	if err := t.requireCleanWorktree(ctx); err != nil {
		return Report{}, err
	}
	if err := t.requireBaseSwitchable(ctx, state.BaseRefName); err != nil {
		return Report{}, err
	}
	// A local base that is ahead of or diverged from origin/<base> would make
	// updateBase's fast-forward fail only after the merge and remote deletion,
	// so require it to be fast-forwardable before the irreversible merge.
	if err := t.requireBaseFastForwardable(ctx, id, state); err != nil {
		return Report{}, err
	}
	if _, err := t.command(ctx, commandTimeout, ghCommand, "pr", "merge", strconv.Itoa(state.Number), "--squash", "--subject", subject, "--body", body, "--match-head-commit", state.HeadRefOID, repoFlag, state.repo()); err != nil {
		// The merge may have completed even though the response was lost, so
		// re-read the state and run cleanup instead of reporting a failure that
		// leaves a merged PR unrecorded.
		merged, viewErr := t.fetchMergeView(ctx, state)
		if viewErr == nil && merged.State == mergedState {
			return t.cleanupAfterMerge(ctx, state, &merged, &id)
		}
		return Report{}, fmt.Errorf("merge PR: %w", err)
	}
	return t.finishMerge(ctx, state, &id)
}

// requireIdentity resolves this checkout's repository and requires it to match
// the state, so a state file carried in from another checkout cannot merge its
// PR here.
func (t *Tool) requireIdentity(ctx context.Context, state State) (identity, error) {
	id, err := t.repoIdentity(ctx)
	if err != nil {
		return identity{}, err
	}
	if !sameRepo(id.Owner, id.Repo, state.Owner, state.Repo) {
		return identity{}, errRepoMismatch
	}
	return id, nil
}

// verifyMergeable rejects a view that is cross-repository, not open, or drifted
// from the pinned state.
func verifyMergeable(live mergeView, state State) error {
	if live.IsCrossRepository {
		return errCrossRepository
	}
	if live.State != openState {
		return fmt.Errorf("%w: state is %s", errPRNotOpen, live.State)
	}
	return verifyOpen(live, state)
}

// finishMerge confirms the terminal state and cleans up. A merge queue accepts
// the PR and returns before it merges, so it reports that state instead.
func (t *Tool) finishMerge(ctx context.Context, state State, id *identity) (Report, error) {
	merged, err := t.fetchMergeView(ctx, state)
	if err != nil {
		return Report{}, err
	}
	if merged.State != mergedState {
		return Report{}, fmt.Errorf("%w: state is %s; wait and re-run `mergepr cleanup --state <file>`", errMergeQueued, merged.State)
	}
	return t.cleanupAfterMerge(ctx, state, &merged, id)
}

// cleanupAfterMerge runs cleanup and, when it fails, reports the confirmed merge
// commit so an operator can tell a completed merge from a failed one and resume
// with `mergepr cleanup`.
func (t *Tool) cleanupAfterMerge(ctx context.Context, state State, merged *mergeView, id *identity) (Report, error) {
	report, err := t.cleanupWith(ctx, state, merged, id)
	if err != nil {
		oid := ""
		if merged != nil && merged.MergeCommit != nil {
			oid = merged.MergeCommit.OID
		}
		return Report{MergeCommitOID: oid}, fmt.Errorf("%w (merge commit %s): %w", errMergeCleanup, oid, err)
	}
	return report, nil
}

// verifyOpen rejects a PR that drifted from the pinned state while the command
// was paused.
func verifyOpen(live mergeView, state State) error {
	if live.HeadRefName != state.HeadRefName || live.HeadRefOID != state.HeadRefOID {
		return fmt.Errorf("%w: head is %s at %s, pinned %s at %s", errHeadDrift, live.HeadRefName, live.HeadRefOID, state.HeadRefName, state.HeadRefOID)
	}
	if live.BaseRefName != state.BaseRefName {
		return fmt.Errorf("%w: base is %s, pinned %s", errBaseDrift, live.BaseRefName, state.BaseRefName)
	}
	return nil
}

// Cleanup resumes the post-merge cleanup from a Prepare state file after a
// failed Merge run. It never merges.
func (t *Tool) Cleanup(ctx context.Context, statePath string) (Report, error) {
	state, err := loadState(statePath)
	if err != nil {
		return Report{}, err
	}
	return t.cleanupWith(ctx, state, nil, nil)
}

// Diff returns the bounded patch for one changed path, resolved literally, so
// the drafter can inspect a file without reimplementing the git plumbing.
func (t *Tool) Diff(ctx context.Context, statePath, path string) ([]byte, error) {
	state, err := loadState(statePath)
	if err != nil {
		return nil, err
	}
	if err := checkDiffPath(path); err != nil {
		return nil, err
	}
	baseRef := originRefs + state.BaseRefName
	// --no-ext-diff and --no-textconv stop a configured external diff or
	// textconv driver from executing on this untrusted patch.
	out, err := t.command(ctx, commandTimeout, gitCommand, "--literal-pathspecs", "diff", "--no-ext-diff", "--no-textconv", baseRef+"..."+state.HeadRefOID, "--", path)
	if err != nil {
		return nil, fmt.Errorf("read diff: %w", err)
	}
	if len(out) > maxDiffBytes {
		return nil, fmt.Errorf("%w: diff is %d bytes, limit %d", errTooLarge, len(out), maxDiffBytes)
	}
	return out, nil
}

// checkDiffPath rejects an empty path. A leading dash is safe because the diff
// passes the path after "--" with --literal-pathspecs.
func checkDiffPath(path string) error {
	if path == "" {
		return fmt.Errorf("%w: %q", errInvalidPath, path)
	}
	return nil
}

// cleanupWith needs a fresh merge view and a pinned repository identity; the
// resume path passes the view it already has, and Merge passes neither.
func (t *Tool) cleanupWith(ctx context.Context, state State, live *mergeView, id *identity) (Report, error) {
	if live == nil {
		fetched, err := t.fetchMergeView(ctx, state)
		if err != nil {
			return Report{}, err
		}
		live = &fetched
	}
	if err := verifyMerged(live, state); err != nil {
		return Report{}, err
	}
	report := Report{MergeCommitOID: live.MergeCommit.OID}

	// Resolve and pin origin now; the switch below cannot change which URLs the
	// later fetch and prune use.
	if id == nil {
		resolved, err := t.repoIdentity(ctx)
		if err != nil {
			return Report{}, err
		}
		id = &resolved
	}
	if !sameRepo(id.Owner, id.Repo, state.Owner, state.Repo) {
		return Report{}, errRepoMismatch
	}
	if err := t.requireCleanWorktree(ctx); err != nil {
		return Report{}, err
	}
	if err := t.requireBaseSwitchable(ctx, state.BaseRefName); err != nil {
		return Report{}, err
	}
	if err := t.requireMergeInBase(ctx, *id, state, live.MergeCommit.OID); err != nil {
		return Report{}, err
	}
	var err error
	if report.RemoteDeleted, err = t.deleteRemoteBranch(ctx, *id, state); err != nil {
		return Report{}, err
	}
	if report.BaseUpdated, err = t.updateBase(ctx, *id, state); err != nil {
		return Report{}, err
	}
	if report.LocalDeleted, err = t.deleteLocalBranch(ctx, state); err != nil {
		return Report{}, err
	}
	if _, err := t.command(ctx, commandTimeout, gitCommand, "fetch", "--prune", id.FetchURL, refsWildcard); err != nil {
		return Report{}, fmt.Errorf("prune origin: %w", err)
	}
	return report, nil
}

func verifyMerged(live *mergeView, state State) error {
	switch {
	case live.State != mergedState:
		return fmt.Errorf("%w: state is %s", errPRNotMerged, live.State)
	case live.HeadRefName != state.HeadRefName || live.HeadRefOID != state.HeadRefOID:
		return fmt.Errorf("%w: head is %s at %s, pinned %s at %s", errHeadDrift, live.HeadRefName, live.HeadRefOID, state.HeadRefName, state.HeadRefOID)
	case live.BaseRefName != state.BaseRefName:
		return fmt.Errorf("%w: base is %s, pinned %s", errBaseDrift, live.BaseRefName, state.BaseRefName)
	case live.MergeCommit == nil || live.MergeCommit.OID == "":
		return errNoMergeCommit
	}
	return nil
}

// requireMergeInBase fetches the base and requires the recorded merge commit to
// be an ancestor, so a force-pushed base that dropped the merge cannot let both
// branch refs be deleted.
func (t *Tool) requireMergeInBase(ctx context.Context, id identity, state State, mergeOID string) error {
	baseRef := originRefs + state.BaseRefName
	if _, err := t.command(ctx, commandTimeout, gitCommand, "fetch", id.FetchURL, "+"+refsHeads+state.BaseRefName+":"+baseRef); err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	if _, err := t.command(ctx, commandTimeout, gitCommand, "merge-base", "--is-ancestor", mergeOID, baseRef); err != nil {
		return fmt.Errorf("%w: %s is not in %s", errMergeMissing, mergeOID, baseRef)
	}
	return nil
}

func (t *Tool) deleteRemoteBranch(ctx context.Context, id identity, state State) (bool, error) {
	headRef := refsHeads + state.HeadRefName
	out, err := t.command(ctx, commandTimeout, gitCommand, "ls-remote", "--heads", id.PushURL, headRef)
	if err != nil {
		return false, fmt.Errorf("list remote branch: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return false, nil
	}
	lease := "--force-with-lease=" + headRef + ":" + state.HeadRefOID
	// --recurse-submodules=no overrides a repository-local
	// push.recurseSubmodules=only, which would otherwise make this push exit
	// successfully without deleting the superproject branch.
	if _, err := t.command(ctx, commandTimeout, gitCommand, "push", "--recurse-submodules=no", lease, id.PushURL, "--delete", headRef); err != nil {
		return false, fmt.Errorf("delete remote branch: %w", err)
	}
	return true, nil
}

func (t *Tool) updateBase(ctx context.Context, id identity, state State) (bool, error) {
	// --no-overwrite-ignore keeps a local ignored file that the base branch
	// tracks; the default would silently replace it.
	if _, err := t.command(ctx, commandTimeout, gitCommand, "switch", "--no-overwrite-ignore", state.BaseRefName); err != nil {
		return false, fmt.Errorf("switch to base branch: %w", err)
	}
	baseRef := originRefs + state.BaseRefName
	if _, err := t.command(ctx, commandTimeout, gitCommand, "fetch", id.FetchURL, "+"+refsHeads+state.BaseRefName+":"+baseRef); err != nil {
		return false, fmt.Errorf("fetch base branch: %w", err)
	}
	// The fast-forward can also newly track a path that was ignored on the old
	// base, so keep the same protection the switch uses.
	if _, err := t.command(ctx, commandTimeout, gitCommand, "merge", "--ff-only", "--no-overwrite-ignore", baseRef); err != nil {
		return false, fmt.Errorf("fast-forward base branch: %w", err)
	}
	if err := t.requireBaseCurrent(ctx, baseRef); err != nil {
		return false, err
	}
	return true, nil
}

func (t *Tool) deleteLocalBranch(ctx context.Context, state State) (bool, error) {
	localOID, err := t.localHeadOID(ctx, state.HeadRefName)
	if err != nil {
		return false, err
	}
	if localOID == "" {
		return false, nil
	}
	if localOID != state.HeadRefOID {
		return false, fmt.Errorf("%w: %s is %s, want %s", errLocalBranchDrift, state.HeadRefName, localOID, state.HeadRefOID)
	}
	if err := t.requireNotCheckedOut(ctx, state.HeadRefName, errBranchCheckedOut); err != nil {
		return false, err
	}
	// update-ref -d deletes only while the ref still holds the merged OID,
	// which branch -D would not check.
	if _, err := t.command(ctx, commandTimeout, gitCommand, "update-ref", "-d", refsHeads+state.HeadRefName, state.HeadRefOID); err != nil {
		return false, fmt.Errorf("delete local branch: %w", err)
	}
	return true, nil
}

// requireNotCheckedOut returns sentinel when any worktree has the branch checked
// out; callers run it only when this worktree is on a different branch, so a
// match is always another worktree. update-ref -d would not check this.
func (t *Tool) requireNotCheckedOut(ctx context.Context, name string, sentinel error) error {
	out, err := t.command(ctx, commandTimeout, gitCommand, "worktree", "list", "--porcelain")
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}
	want := "branch " + refsHeads + name
	for line := range strings.Lines(string(out)) {
		if strings.TrimSpace(line) == want {
			return fmt.Errorf("%w: %s", sentinel, name)
		}
	}
	return nil
}

// repoIdentity resolves the repository once and pins it: origin's fetch URL,
// every push URL, and the repository gh selects must name the same GitHub
// repository over https or ssh, so no later step can act on a different one.
func (t *Tool) repoIdentity(ctx context.Context) (identity, error) {
	if err := t.requireSupportedConfig(ctx); err != nil {
		return identity{}, err
	}
	fetchURL, err := t.command(ctx, commandTimeout, gitCommand, "remote", "get-url", originRemote)
	if err != nil {
		return identity{}, fmt.Errorf("read origin fetch URL: %w", err)
	}
	fetch := strings.TrimSpace(string(fetchURL))
	fetchOwner, fetchRepo, err := parseGitHubRemote(fetch)
	if err != nil {
		return identity{}, err
	}
	pushURLs, err := t.command(ctx, commandTimeout, gitCommand, "remote", "get-url", "--push", "--all", originRemote)
	if err != nil {
		return identity{}, fmt.Errorf("read origin push URLs: %w", err)
	}
	// git push updates every configured push URL, so one unvalidated mirror
	// could receive the branch deletion.
	var pinnedPush string
	for line := range strings.Lines(string(pushURLs)) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		owner, repo, err := parseGitHubRemote(line)
		if err != nil || !sameRepo(owner, repo, fetchOwner, fetchRepo) {
			return identity{}, errRemoteMismatch
		}
		if pinnedPush == "" {
			pinnedPush = line
		}
	}
	if pinnedPush == "" {
		return identity{}, errRemoteMismatch
	}
	selected, err := t.command(ctx, commandTimeout, ghCommand, "repo", "view", jsonFlag, "nameWithOwner,url")
	if err != nil {
		return identity{}, fmt.Errorf("resolve repository gh selects: %w", err)
	}
	var view struct {
		NameWithOwner string `json:"nameWithOwner"`
		URL           string `json:"url"`
	}
	if err := json.Unmarshal(selected, &view); err != nil {
		return identity{}, fmt.Errorf("parse repository gh selects: %w", err)
	}
	selectedOwner, selectedRepo, err := parseGitHubRemote(view.URL)
	if err != nil || !strings.EqualFold(view.NameWithOwner, fetchOwner+"/"+fetchRepo) || !sameRepo(selectedOwner, selectedRepo, fetchOwner, fetchRepo) {
		return identity{}, fmt.Errorf("%w: gh selects %q", errRepoMismatch, view.NameWithOwner)
	}
	return identity{Owner: fetchOwner, Repo: fetchRepo, FetchURL: fetch, PushURL: pinnedPush}, nil
}

// parseGitHubRemote extracts owner and repo from an https or ssh GitHub remote
// URL. Plain http is rejected because a credential-bearing http URL could be
// sent in the clear, and the URL is never included in an error because a remote
// URL can embed credentials.
func parseGitHubRemote(raw string) (string, string, error) {
	if raw == "" {
		return "", "", errInvalidRemote
	}
	var path string
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil || !strings.EqualFold(parsed.Hostname(), githubHost) {
			return "", "", errInvalidRemote
		}
		// A query or fragment can carry a credential, which would be pinned and
		// passed to git in argv.
		if parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", "", errInvalidRemote
		}
		switch parsed.Scheme {
		case "https":
			// A credential in an https URL would be passed to git in argv.
			if parsed.User != nil {
				return "", "", errInvalidRemote
			}
		case "ssh":
			if user := parsed.User; user != nil {
				if _, hasPassword := user.Password(); hasPassword || user.Username() != "git" {
					return "", "", errInvalidRemote
				}
			}
		default:
			return "", "", errInvalidRemote
		}
		path = parsed.Path
	} else if user, rest, found := strings.Cut(raw, "@"); found {
		// GitHub's SSH user is always "git"; any other user is likely a token.
		if user != "git" {
			return "", "", errInvalidRemote
		}
		host, subpath, ok := strings.Cut(rest, ":")
		if !ok || !strings.EqualFold(host, githubHost) {
			return "", "", errInvalidRemote
		}
		path = subpath
	} else {
		return "", "", errInvalidRemote
	}
	path = strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	owner, repo, ok := strings.Cut(path, "/")
	if !ok || !repoPartPattern.MatchString(owner) || !repoPartPattern.MatchString(repo) {
		return "", "", errInvalidRemote
	}
	return owner, repo, nil
}

// parsePRArg accepts an empty argument (the current branch's PR), a positive
// number, or a same-repository GitHub pull URL. Anything else is rejected
// rather than guessed.
func parsePRArg(arg, owner, repo string) (int, bool, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return 0, true, nil
	}
	if number, err := strconv.Atoi(arg); err == nil {
		if number <= 0 {
			return 0, false, fmt.Errorf("%w: %q", errInvalidPRArg, arg)
		}
		return number, false, nil
	}
	parsed, err := url.Parse(arg)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), githubHost) {
		return 0, false, fmt.Errorf("%w: %q", errInvalidPRArg, arg)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" {
		return 0, false, fmt.Errorf("%w: %q", errInvalidPRArg, arg)
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return 0, false, fmt.Errorf("%w: %q", errInvalidPRArg, arg)
	}
	if !sameRepo(parts[0], parts[1], owner, repo) {
		return 0, false, fmt.Errorf("%w: URL names %s/%s, origin is %s/%s", errInvalidPRArg, parts[0], parts[1], owner, repo)
	}
	return number, false, nil
}

type prInfo struct {
	Number            int    `json:"number"`
	Title             string `json:"title"`
	State             string `json:"state"`
	HeadRefName       string `json:"headRefName"`
	HeadRefOID        string `json:"headRefOid"`
	BaseRefName       string `json:"baseRefName"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	URL               string `json:"url"`
	Body              string `json:"body"`
}

// fetchPR reads the PR selected by number, or by head branch when branch is
// non-empty.
func (t *Tool) fetchPR(ctx context.Context, owner, repo string, number int, branch string) (prInfo, error) {
	args := []string{"pr", "view"}
	if branch != "" {
		args = append(args, branch)
	} else {
		args = append(args, strconv.Itoa(number))
	}
	args = append(args, jsonFlag, prViewFields, repoFlag, owner+"/"+repo)
	out, err := t.command(ctx, commandTimeout, ghCommand, args...)
	if err != nil {
		return prInfo{}, fmt.Errorf("read PR: %w", err)
	}
	var pr prInfo
	if err := json.Unmarshal(out, &pr); err != nil {
		return prInfo{}, fmt.Errorf("parse PR: %w", err)
	}
	// gh also reads a numeric or URL-shaped selector as a PR number or URL, so
	// require the PR it returns to be the one asked for.
	if branch != "" && pr.HeadRefName != branch {
		return prInfo{}, fmt.Errorf("%w: gh returned PR #%d with head %q, want head %q", errInvalidPRArg, pr.Number, pr.HeadRefName, branch)
	}
	if branch == "" && pr.Number != number {
		return prInfo{}, fmt.Errorf("%w: gh returned PR #%d, want #%d", errInvalidPRArg, pr.Number, number)
	}
	return pr, nil
}

// currentBranch returns the short name of the branch HEAD is attached to,
// validated so gh cannot read it as an option.
func (t *Tool) currentBranch(ctx context.Context) (string, error) {
	out, err := t.command(ctx, commandTimeout, gitCommand, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return "", errDetachedHead
	}
	ref := strings.TrimSpace(string(out))
	name, ok := strings.CutPrefix(ref, refsHeads)
	if !ok {
		return "", fmt.Errorf("%w: HEAD is %q", errInvalidBranch, ref)
	}
	if err := checkRefNameSyntax(name); err != nil {
		return "", err
	}
	return name, nil
}

type commitRef struct {
	OID string `json:"oid"`
}

type mergeView struct {
	State             string     `json:"state"`
	HeadRefName       string     `json:"headRefName"`
	HeadRefOID        string     `json:"headRefOid"`
	BaseRefName       string     `json:"baseRefName"`
	IsCrossRepository bool       `json:"isCrossRepository"`
	MergeCommit       *commitRef `json:"mergeCommit"`
}

func (t *Tool) fetchMergeView(ctx context.Context, state State) (mergeView, error) {
	out, err := t.command(ctx, commandTimeout, ghCommand, "pr", "view", strconv.Itoa(state.Number), jsonFlag, mergeViewJSON, repoFlag, state.repo())
	if err != nil {
		return mergeView{}, fmt.Errorf("read PR state: %w", err)
	}
	var view mergeView
	if err := json.Unmarshal(out, &view); err != nil {
		return mergeView{}, fmt.Errorf("parse PR state: %w", err)
	}
	return view, nil
}

// checkRefNameSyntax rejects names that git could read as an option (a leading
// dash) or as a reflog expression (the "@{" sequence) before git's own
// check-ref-format sees them.
func checkRefNameSyntax(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.Contains(name, "@{") {
		return fmt.Errorf("%w: %q", errInvalidBranch, name)
	}
	return nil
}

func (t *Tool) gitCheckRefFormat(ctx context.Context, name string) error {
	if _, err := t.command(ctx, commandTimeout, gitCommand, "check-ref-format", "--branch", name); err != nil {
		return fmt.Errorf("%w: %q: %w", errInvalidBranch, name, err)
	}
	return nil
}

func (t *Tool) requireCleanWorktree(ctx context.Context) error {
	out, err := t.command(ctx, commandTimeout, gitCommand, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("read worktree status: %w", err)
	}
	if status := strings.TrimSpace(string(out)); status != "" {
		return fmt.Errorf("%w:\n%s", errDirtyWorktree, status)
	}
	return nil
}

// requireBaseSwitchable rejects a detached HEAD, so a later branch switch cannot
// leave its tip unreachable, and a base branch checked out in another worktree,
// which git switch refuses only after the irreversible merge.
func (t *Tool) requireBaseSwitchable(ctx context.Context, base string) error {
	current, err := t.currentBranch(ctx)
	if err != nil {
		return err
	}
	if current == base {
		return nil
	}
	return t.requireNotCheckedOut(ctx, base, errBaseCheckedOut)
}

// localHeadOID returns the OID of the local branch, or an empty string when it
// does not exist. for-each-ref matches its argument as a prefix, so the exact
// ref name is selected from its output instead of trusting the first match.
func (t *Tool) localHeadOID(ctx context.Context, name string) (string, error) {
	out, err := t.command(ctx, commandTimeout, gitCommand, "for-each-ref", "--format=%(refname) %(objectname)", refsHeads+name)
	if err != nil {
		return "", fmt.Errorf("read local branch %q: %w", name, err)
	}
	want := refsHeads + name
	for line := range strings.Lines(string(out)) {
		ref, oid, ok := strings.Cut(strings.TrimSpace(line), " ")
		if ok && ref == want {
			return oid, nil
		}
	}
	return "", nil
}

// requireBaseFastForwardable fetches the base and requires the local base
// branch to be absent or an ancestor of the freshly fetched remote base, so the
// cleanup fast-forward cannot fail after the merge and remote deletion.
func (t *Tool) requireBaseFastForwardable(ctx context.Context, id identity, state State) error {
	baseRef := originRefs + state.BaseRefName
	if _, err := t.command(ctx, commandTimeout, gitCommand, "fetch", id.FetchURL, "+"+refsHeads+state.BaseRefName+":"+baseRef); err != nil {
		return fmt.Errorf("fetch base branch: %w", err)
	}
	localOID, err := t.localHeadOID(ctx, state.BaseRefName)
	if err != nil {
		return err
	}
	if localOID == "" {
		return nil
	}
	if _, err := t.command(ctx, commandTimeout, gitCommand, "merge-base", "--is-ancestor", localOID, baseRef); err != nil {
		return fmt.Errorf("%w: local %s is not an ancestor of %s", errBaseNotCurrent, state.BaseRefName, baseRef)
	}
	return nil
}

func (t *Tool) requireBaseCurrent(ctx context.Context, baseRef string) error {
	head, err := t.command(ctx, commandTimeout, gitCommand, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read HEAD: %w", err)
	}
	base, err := t.command(ctx, commandTimeout, gitCommand, "rev-parse", baseRef)
	if err != nil {
		return fmt.Errorf("read %s: %w", baseRef, err)
	}
	if strings.TrimSpace(string(head)) == strings.TrimSpace(string(base)) {
		return nil
	}
	commits, err := t.command(ctx, commandTimeout, gitCommand, "log", "--oneline", baseRef+"..HEAD")
	if err != nil {
		return fmt.Errorf("%w: local base has commits origin lacks (%w)", errBaseNotCurrent, err)
	}
	return fmt.Errorf("%w:\n%s", errBaseNotCurrent, strings.TrimSpace(string(commits)))
}

// readBody reads and bounds the squash body. It is read before the CI wait and
// passed to gh directly, so a file changed during the wait cannot alter the
// approved commit message.
func readBody(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // an operator-supplied path, not untrusted content
	if err != nil {
		return "", fmt.Errorf("read body file: %w", err)
	}
	if len(data) > maxBodyBytes {
		return "", fmt.Errorf("%w: body is %d bytes, limit %d", errTooLarge, len(data), maxBodyBytes)
	}
	return string(data), nil
}

// requireSupportedConfig refuses to run when the repository's local git config
// contains a key outside the known-safe allowlist. Global and system config are
// disabled for every child process (see childEnv), so only the local (and, when
// enabled, worktree) config could otherwise redirect a URL or execute a
// program. Rejecting by allowlist means an executable key added to git later is
// rejected by default instead of being missed.
func (t *Tool) requireSupportedConfig(ctx context.Context) error {
	// --local excludes the command-scope overrides from childEnv; --null uses NUL
	// between entries and a newline between key and value, so a key that
	// contains "=" is not truncated.
	out, err := t.command(ctx, commandTimeout, gitCommand, "config", "--local", "--null", "--includes", "--list")
	if err != nil {
		return fmt.Errorf("read git config: %w", err)
	}
	worktree := false
	for entry := range strings.SplitSeq(string(out), "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		if key == "" {
			continue
		}
		// Git treats a valueless key as true, and accepts "true", "yes", "on",
		// and "1"; only an explicit false-ish value disables the extension.
		if strings.EqualFold(key, "extensions.worktreeconfig") && (value == "" || gitBoolTrue(value)) {
			worktree = true
		}
		if !allowedConfig(key) {
			return fmt.Errorf("%w: section %q", errUnsupportedConfig, configSection(key))
		}
	}
	if !worktree {
		return nil
	}
	out, err = t.command(ctx, commandTimeout, gitCommand, "config", "--worktree", "--null", "--list")
	if err != nil {
		return fmt.Errorf("read worktree config: %w", err)
	}
	for entry := range strings.SplitSeq(string(out), "\x00") {
		key, _, _ := strings.Cut(entry, "\n")
		if key != "" && !allowedConfig(key) {
			return fmt.Errorf("%w: section %q", errUnsupportedConfig, configSection(key))
		}
	}
	return nil
}

// allowedConfigKeys are the repository-local keys the tool tolerates in addition
// to the prefixes in allowedConfig. They are non-executable and git may need
// them to interpret a clone or worktree.
var allowedConfigKeys = map[string]struct{}{
	"core.repositoryformatversion": {},
	"core.bare":                    {},
	"core.worktree":                {},
	"core.filemode":                {},
	"core.ignorecase":              {},
	"core.symlinks":                {},
	"core.precomposeunicode":       {},
	"core.logallrefupdates":        {},
	"core.abbrev":                  {},
	"core.quotepath":               {},
	"core.autocrlf":                {},
	"core.safecrlf":                {},
	"core.eol":                     {},
	"core.trustctime":              {},
	"core.checkstat":               {},
	"core.ignorestat":              {},
	"core.commitgraph":             {},
	"core.multipackindex":          {},
	"core.untrackedcache":          {},
	"core.sparsecheckout":          {},
	"core.sparsecheckoutcone":      {},
	"core.bigfilethreshold":        {},
	"user.name":                    {},
	"user.email":                   {},
}

// allowedConfig reports whether a repository-local config key is one the tool
// tolerates: it cannot execute a program and git may need it to interpret the
// repository. Everything else is rejected.
func allowedConfig(key string) bool {
	key = strings.ToLower(key)
	if _, ok := allowedConfigKeys[key]; ok {
		return true
	}
	switch {
	case strings.HasPrefix(key, "remote."):
		// uploadpack/receivepack name a program git runs for the remote.
		return !strings.HasSuffix(key, ".uploadpack") && !strings.HasSuffix(key, ".receivepack") &&
			!strings.HasSuffix(key, ".vcs") && !strings.HasSuffix(key, ".proxy")
	case strings.HasPrefix(key, "gc."):
		// recentObjectsHook names a program git runs during maintenance.
		return key != "gc.recentobjectshook"
	case strings.HasPrefix(key, "push."):
		// followTags would make the branch-deletion push also push tags,
		// pushOption could inject --follow-tags into the same push, and
		// recurseSubmodules=only would make the deletion push report success
		// without deleting the superproject branch.
		return key != "push.followtags" && key != "push.pushoption" && key != "push.recursesubmodules"
	case strings.HasPrefix(key, "branch."),
		strings.HasPrefix(key, "extensions."),
		strings.HasPrefix(key, "index."),
		strings.HasPrefix(key, "fetch."),
		strings.HasPrefix(key, "pull."),
		strings.HasPrefix(key, "status."),
		strings.HasPrefix(key, "commit."),
		strings.HasPrefix(key, "tag."),
		strings.HasPrefix(key, "log."),
		strings.HasPrefix(key, "rebase."):
		return true
	}
	return false
}

// configSection returns the config section (before the first "."), which cannot
// embed a credential, for an error message.
func configSection(key string) string {
	section, _, _ := strings.Cut(key, ".")
	return section
}

// gitBoolTrue reports whether a git boolean value is true: git accepts "true",
// "yes", "on", and "1", case-insensitively.
func gitBoolTrue(value string) bool {
	switch strings.ToLower(value) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}

// sameRepo reports whether two GitHub owner/repo pairs name the same
// repository, which GitHub treats case-insensitively.
func sameRepo(ownerA, repoA, ownerB, repoB string) bool {
	return strings.EqualFold(ownerA, ownerB) && strings.EqualFold(repoA, repoB)
}

func readSubject(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // an operator-supplied path, not untrusted content
	if err != nil {
		return "", fmt.Errorf("read subject file: %w", err)
	}
	if len(data) > maxSubjectBytes {
		return "", fmt.Errorf("%w: subject is %d bytes, limit %d", errTooLarge, len(data), maxSubjectBytes)
	}
	subject := strings.TrimRight(string(data), "\n")
	if subject == "" || strings.ContainsAny(subject, "\r\n") {
		return "", errInvalidSubject
	}
	return subject, nil
}

func writeState(path string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')
	if err := writeFile(path, data); err != nil {
		return err
	}
	return nil
}

func loadState(path string) (State, error) {
	data, err := os.ReadFile(path) //nolint:gosec // an operator-supplied path, not untrusted content
	if err != nil {
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("%w: %w", errInvalidState, err)
	}
	if err := state.validate(); err != nil {
		return State{}, err
	}
	return state, nil
}

func writeFile(path string, data []byte) error {
	// O_NOFOLLOW makes the open fail when the final component is a symlink, so
	// a race that swaps a predictable output name for a symlink cannot redirect
	// the truncating write onto a file the invoking user can write.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, stateFileMode) //nolint:gosec // an operator-supplied path, not untrusted content
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	// OpenFile preserves the mode of an existing file, so enforce it on the open
	// descriptor even when a reused work directory already held a readable file.
	if err := f.Chmod(stateFileMode); err != nil {
		_ = f.Close()
		return fmt.Errorf("set mode on %s: %w", filepath.Base(path), err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}
