// Package mergepr implements the mechanics behind the /mergepr command:
// resolving and verifying the repository, the PR, and its refs once, then
// merging and cleaning up. Values never pass through a shell, refs are
// fully-qualified, and every mutable value is re-verified immediately before
// the irreversible operation that consumes it.
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
)

const (
	gitCommand = "git"
	ghCommand  = "gh"

	originRemote = "origin"
	refsHeads    = "refs/heads/"
	refsRemotes  = "refs/remotes/"
	originRefs   = refsRemotes + originRemote + "/"
	githubHost   = "github.com"

	repoFlag = "-R"
	jsonFlag = "--json"

	openState   = "OPEN"
	mergedState = "MERGED"
	oidLength   = 40

	stateFileName = "state.json"
	logFileName   = "log.txt"
	statFileName  = "stat.txt"

	maxLogBytes     = 256 << 10
	maxStatBytes    = 64 << 10
	maxSubjectBytes = 4 << 10

	stateFileMode = 0o600

	prViewFields  = "number,title,state,headRefName,headRefOid,baseRefName,isCrossRepository,url"
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
// temporary directory when workDir is empty). It stops instead of truncating
// the commit log or diff stat.
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
	owner, repo, pr, err := t.resolvePR(ctx, prArg)
	if err != nil {
		return Prepared{}, err
	}
	logOut, statOut, err := t.draftingMaterial(ctx, owner, repo, pr)
	if err != nil {
		return Prepared{}, err
	}
	return writePrepared(workDir, owner, repo, pr, logOut, statOut)
}

// resolvePR verifies everything about the PR itself before any network
// mutation: repository identity, PR state, ref-name safety, and the local
// worktree and head branch.
func (t *Tool) resolvePR(ctx context.Context, prArg string) (string, string, prInfo, error) {
	owner, repo, err := t.repoIdentity(ctx)
	if err != nil {
		return "", "", prInfo{}, err
	}
	number, useCurrent, err := parsePRArg(prArg, owner, repo)
	if err != nil {
		return "", "", prInfo{}, err
	}
	pr, err := t.fetchPR(ctx, owner, repo, number, useCurrent)
	if err != nil {
		return "", "", prInfo{}, err
	}
	if pr.State != openState {
		return "", "", prInfo{}, fmt.Errorf("%w: state is %s", errPRNotOpen, pr.State)
	}
	if pr.IsCrossRepository {
		return "", "", prInfo{}, errCrossRepository
	}
	for _, name := range []string{pr.HeadRefName, pr.BaseRefName} {
		if err := checkRefNameSyntax(name); err != nil {
			return "", "", prInfo{}, err
		}
		if err := t.gitCheckRefFormat(ctx, name); err != nil {
			return "", "", prInfo{}, err
		}
	}
	if err := t.requireCleanWorktree(ctx); err != nil {
		return "", "", prInfo{}, err
	}
	localOID, err := t.localHeadOID(ctx, pr.HeadRefName)
	if err != nil {
		return "", "", prInfo{}, err
	}
	if localOID != "" && localOID != pr.HeadRefOID {
		return "", "", prInfo{}, fmt.Errorf("%w: %s is %s, want %s", errHeadBranchMismatch, pr.HeadRefName, localOID, pr.HeadRefOID)
	}
	return owner, repo, pr, nil
}

// draftingMaterial fetches origin, waits for CI, and reads the bounded commit
// log and diff stat the message is drafted from.
func (t *Tool) draftingMaterial(ctx context.Context, owner, repo string, pr prInfo) ([]byte, []byte, error) {
	if _, err := t.run.Run(ctx, gitCommand, "fetch", originRemote); err != nil {
		return nil, nil, fmt.Errorf("fetch origin: %w", err)
	}
	if _, err := t.run.Run(ctx, ghCommand, "pr", "checks", strconv.Itoa(pr.Number), "--watch", "--fail-fast", repoFlag, owner+"/"+repo); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errChecksFailed, err)
	}
	baseRef := originRefs + pr.BaseRefName
	logOut, err := t.run.Run(ctx, gitCommand, "log", "--no-show-signature", "--format=%h %s%n%n%b", baseRef+".."+pr.HeadRefOID)
	if err != nil {
		return nil, nil, fmt.Errorf("read commit log: %w", err)
	}
	if len(logOut) > maxLogBytes {
		return nil, nil, fmt.Errorf("%w: commit log is %d bytes, limit %d", errTooLarge, len(logOut), maxLogBytes)
	}
	statOut, err := t.run.Run(ctx, gitCommand, "diff", "--stat", baseRef+"..."+pr.HeadRefOID)
	if err != nil {
		return nil, nil, fmt.Errorf("read diff stat: %w", err)
	}
	if len(statOut) > maxStatBytes {
		return nil, nil, fmt.Errorf("%w: diff stat is %d bytes, limit %d", errTooLarge, len(statOut), maxStatBytes)
	}
	return logOut, statOut, nil
}

func writePrepared(workDir, owner, repo string, pr prInfo, logOut, statOut []byte) (Prepared, error) {
	state := State{
		Number:      pr.Number,
		Owner:       owner,
		Repo:        repo,
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
	return Prepared{State: state, StatePath: statePath, LogPath: logPath, StatPath: statPath}, nil
}

// Merge re-verifies the PR against the pinned state immediately before the
// irreversible merge, merges with --match-head-commit, and cleans up. When the
// PR is already MERGED it skips the merge and resumes cleanup.
func (t *Tool) Merge(ctx context.Context, statePath, subjectPath, bodyPath string) (Report, error) {
	state, err := loadState(statePath)
	if err != nil {
		return Report{}, err
	}
	subject, err := readSubject(subjectPath)
	if err != nil {
		return Report{}, err
	}
	if _, err := os.Stat(bodyPath); err != nil {
		return Report{}, fmt.Errorf("read body file: %w", err)
	}
	live, err := t.fetchMergeView(ctx, state)
	if err != nil {
		return Report{}, err
	}
	switch {
	case live.IsCrossRepository:
		return Report{}, errCrossRepository
	case live.State == mergedState:
		return t.cleanupWith(ctx, state, &live)
	case live.State != openState:
		return Report{}, fmt.Errorf("%w: state is %s", errPRNotOpen, live.State)
	}
	if live.HeadRefName != state.HeadRefName || live.HeadRefOID != state.HeadRefOID {
		return Report{}, fmt.Errorf("%w: head is %s at %s, pinned %s at %s", errHeadDrift, live.HeadRefName, live.HeadRefOID, state.HeadRefName, state.HeadRefOID)
	}
	if live.BaseRefName != state.BaseRefName {
		return Report{}, fmt.Errorf("%w: base is %s, pinned %s", errBaseDrift, live.BaseRefName, state.BaseRefName)
	}
	if _, err := t.run.Run(ctx, ghCommand, "pr", "checks", strconv.Itoa(state.Number), "--watch", "--fail-fast", repoFlag, state.repo()); err != nil {
		return Report{}, fmt.Errorf("%w: %w", errChecksFailed, err)
	}
	if _, err := t.run.Run(ctx, ghCommand, "pr", "merge", strconv.Itoa(state.Number), "--squash", "--subject", subject, "--body-file", bodyPath, "--match-head-commit", state.HeadRefOID, repoFlag, state.repo()); err != nil {
		return Report{}, fmt.Errorf("merge PR: %w", err)
	}
	return t.cleanupWith(ctx, state, nil)
}

// Cleanup resumes the post-merge cleanup from a Prepare state file after a
// failed Merge run. It never merges.
func (t *Tool) Cleanup(ctx context.Context, statePath string) (Report, error) {
	state, err := loadState(statePath)
	if err != nil {
		return Report{}, err
	}
	return t.cleanupWith(ctx, state, nil)
}

// cleanupWith needs a fresh merge view, so a caller that already has one (the
// resume path) passes it instead of asking GitHub twice.
func (t *Tool) cleanupWith(ctx context.Context, state State, live *mergeView) (Report, error) {
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

	// Re-verify origin's identity now: configuration could have changed since
	// prepare, and the next steps act on the remote.
	owner, repo, err := t.repoIdentity(ctx)
	if err != nil {
		return Report{}, err
	}
	if owner != state.Owner || repo != state.Repo {
		return Report{}, errRepoMismatch
	}
	if err := t.requireCleanWorktree(ctx); err != nil {
		return Report{}, err
	}
	if report.RemoteDeleted, err = t.deleteRemoteBranch(ctx, state); err != nil {
		return Report{}, err
	}
	if report.BaseUpdated, err = t.updateBase(ctx, state); err != nil {
		return Report{}, err
	}
	if report.LocalDeleted, err = t.deleteLocalBranch(ctx, state); err != nil {
		return Report{}, err
	}
	if _, err := t.run.Run(ctx, gitCommand, "fetch", "--prune", originRemote); err != nil {
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

func (t *Tool) deleteRemoteBranch(ctx context.Context, state State) (bool, error) {
	headRef := refsHeads + state.HeadRefName
	out, err := t.run.Run(ctx, gitCommand, "ls-remote", "--heads", originRemote, headRef)
	if err != nil {
		return false, fmt.Errorf("list remote branch: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return false, nil
	}
	lease := "--force-with-lease=" + headRef + ":" + state.HeadRefOID
	if _, err := t.run.Run(ctx, gitCommand, "push", lease, originRemote, "--delete", headRef); err != nil {
		return false, fmt.Errorf("delete remote branch: %w", err)
	}
	return true, nil
}

func (t *Tool) updateBase(ctx context.Context, state State) (bool, error) {
	if _, err := t.run.Run(ctx, gitCommand, "switch", state.BaseRefName); err != nil {
		return false, fmt.Errorf("switch to base branch: %w", err)
	}
	baseRef := originRefs + state.BaseRefName
	if _, err := t.run.Run(ctx, gitCommand, "fetch", originRemote, refsHeads+state.BaseRefName+":"+baseRef); err != nil {
		return false, fmt.Errorf("fetch base branch: %w", err)
	}
	if _, err := t.run.Run(ctx, gitCommand, "merge", "--ff-only", baseRef); err != nil {
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
	if _, err := t.run.Run(ctx, gitCommand, "branch", "-D", state.HeadRefName); err != nil {
		return false, fmt.Errorf("delete local branch: %w", err)
	}
	return true, nil
}

// repoIdentity resolves the repository once and requires origin's fetch URL,
// origin's push URL, and the repository gh selects to name the same GitHub
// repository, so no later step can act on a different one.
func (t *Tool) repoIdentity(ctx context.Context) (string, string, error) {
	fetchURL, err := t.run.Run(ctx, gitCommand, "remote", "get-url", originRemote)
	if err != nil {
		return "", "", fmt.Errorf("read origin fetch URL: %w", err)
	}
	pushURL, err := t.run.Run(ctx, gitCommand, "remote", "get-url", "--push", originRemote)
	if err != nil {
		return "", "", fmt.Errorf("read origin push URL: %w", err)
	}
	fetchOwner, fetchRepo, err := parseGitHubRemote(strings.TrimSpace(string(fetchURL)))
	if err != nil {
		return "", "", err
	}
	pushOwner, pushRepo, err := parseGitHubRemote(strings.TrimSpace(string(pushURL)))
	if err != nil {
		return "", "", err
	}
	if fetchOwner != pushOwner || fetchRepo != pushRepo {
		return "", "", errRemoteMismatch
	}
	selected, err := t.run.Run(ctx, ghCommand, "repo", "view", jsonFlag, "nameWithOwner", "-q", ".nameWithOwner")
	if err != nil {
		return "", "", fmt.Errorf("resolve repository gh selects: %w", err)
	}
	if got := strings.TrimSpace(string(selected)); got != fetchOwner+"/"+fetchRepo {
		return "", "", fmt.Errorf("%w: gh selects %q", errRepoMismatch, got)
	}
	return fetchOwner, fetchRepo, nil
}

// parseGitHubRemote extracts owner and repo from an https, http, ssh, or
// scp-like GitHub remote URL. It never includes the URL in an error, because a
// remote URL can embed credentials.
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
		switch parsed.Scheme {
		case "https", "http", "ssh":
		default:
			return "", "", errInvalidRemote
		}
		path = parsed.Path
	} else if _, rest, found := strings.Cut(raw, "@"); found {
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
	if parts[0] != owner || parts[1] != repo {
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
}

func (t *Tool) fetchPR(ctx context.Context, owner, repo string, number int, useCurrent bool) (prInfo, error) {
	args := []string{"pr", "view"}
	if !useCurrent {
		args = append(args, strconv.Itoa(number))
	}
	args = append(args, jsonFlag, prViewFields, repoFlag, owner+"/"+repo)
	out, err := t.run.Run(ctx, ghCommand, args...)
	if err != nil {
		return prInfo{}, fmt.Errorf("read PR: %w", err)
	}
	var pr prInfo
	if err := json.Unmarshal(out, &pr); err != nil {
		return prInfo{}, fmt.Errorf("parse PR: %w", err)
	}
	if !useCurrent && pr.Number != number {
		return prInfo{}, fmt.Errorf("%w: gh returned PR #%d, want #%d", errInvalidPRArg, pr.Number, number)
	}
	return pr, nil
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
	out, err := t.run.Run(ctx, ghCommand, "pr", "view", strconv.Itoa(state.Number), jsonFlag, mergeViewJSON, repoFlag, state.repo())
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
	if _, err := t.run.Run(ctx, gitCommand, "check-ref-format", "--branch", name); err != nil {
		return fmt.Errorf("%w: %q: %w", errInvalidBranch, name, err)
	}
	return nil
}

func (t *Tool) requireCleanWorktree(ctx context.Context) error {
	out, err := t.run.Run(ctx, gitCommand, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("read worktree status: %w", err)
	}
	if status := strings.TrimSpace(string(out)); status != "" {
		return fmt.Errorf("%w:\n%s", errDirtyWorktree, status)
	}
	return nil
}

// localHeadOID returns the OID of the local branch, or an empty string when it
// does not exist. for-each-ref exits zero either way, so a missing branch is
// not confused with a failing git.
func (t *Tool) localHeadOID(ctx context.Context, name string) (string, error) {
	out, err := t.run.Run(ctx, gitCommand, "for-each-ref", "--format=%(objectname)", refsHeads+name)
	if err != nil {
		return "", fmt.Errorf("read local branch %q: %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *Tool) requireBaseCurrent(ctx context.Context, baseRef string) error {
	head, err := t.run.Run(ctx, gitCommand, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read HEAD: %w", err)
	}
	base, err := t.run.Run(ctx, gitCommand, "rev-parse", baseRef)
	if err != nil {
		return fmt.Errorf("read %s: %w", baseRef, err)
	}
	if strings.TrimSpace(string(head)) == strings.TrimSpace(string(base)) {
		return nil
	}
	commits, err := t.run.Run(ctx, gitCommand, "log", "--oneline", baseRef+"..HEAD")
	if err != nil {
		return fmt.Errorf("%w: local base has commits origin lacks (%w)", errBaseNotCurrent, err)
	}
	return fmt.Errorf("%w:\n%s", errBaseNotCurrent, strings.TrimSpace(string(commits)))
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
	if err := os.WriteFile(path, data, stateFileMode); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}
