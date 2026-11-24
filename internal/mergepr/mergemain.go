package mergepr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/strictjson"
	"github.com/isseis/yt2column/internal/tasknaming"
)

// The take-in tool is the four mergemain-* subcommands the /mergemain command
// calls to merge origin/main into a PR branch. The tool makes every decision
// that can be made deterministically: whether the PR and the work tree meet
// the premise, which conflicts may be resolved, which protocol numbers change
// and to what, whether a merge commit may be pushed, and whether the work tree
// may be reset. The agent that runs /mergemain only resolves the Markdown
// conflicts, applies the README and ADR rules, commits, and runs the checks.
//
// The values a later subcommand needs (the base head, the head branch, and the
// main being taken in) are passed back by the caller as arguments, never read
// from a state file; every subcommand checks them again against git and, for
// begin and push, against GitHub. The one value kept in git is the main that
// begin fetched, in refs/mergemain/<PR>/main, which merge and push compare the
// argument with.

// mergemainPRFields are the only PR members the take-in tool reads: never the
// body, the comments, or the reviews, which are untrusted text.
const mergemainPRFields = "state,baseRefName,headRefName,headRefOid,isCrossRepository"

const (
	// headsPrefix is the namespace of branches.
	headsPrefix = "refs/heads/"
	// originRemote is the only remote the take-in tool works with.
	originRemote = "origin"
	// ghRepoVariable is the environment variable that makes gh read another
	// repository than the git remotes name.
	ghRepoVariable = "GH_REPO"
	// adrDir is the directory of the ADRs.
	adrDir = "docs/dev/adr"
	// extensionDir is the directory whose changes need make ext-check.
	extensionDir = "extension"
	// markdownSuffix is the file name suffix of a Markdown file.
	markdownSuffix = ".md"
	// maxPRDigits bounds the PR number so it always fits an int.
	maxPRDigits = 9
	// commitIDLength is the length of a full SHA-1 commit ID, the only form
	// the take-in tool accepts as an argument.
	commitIDLength = 40
	// regularMode and executableMode are the index modes of a regular file.
	regularMode    = "100644"
	executableMode = "100755"
	// protocolPlaceholder replaces every accepted declaration line when two
	// versions of a tool's file are compared apart from that line. It is an
	// accepted line itself, so a merged result holds it exactly once when the
	// versions do.
	protocolPlaceholder = "const " + protocolWord + " = 1"
)

const (
	// mergeSubcommand is git's merge subcommand.
	mergeSubcommand = "merge"
	// mergeAttribute is the gitattributes attribute that chooses a file's
	// merge driver.
	mergeAttribute = "merge"
)

// readmePaths are the README files the README sync rule applies to.
var readmePaths = []string{"README.md", "README.en.md"}

// extCheckPaths are the files outside extension/ whose change needs make
// ext-check: the node tests read them.
var extCheckPaths = []string{"README.md", "README.en.md", "docs/dev/guides/development.md"}

// mergeConfig is the configuration every merge of the take-in tool runs with,
// the merge in the work tree and the merges merge-tree computes alike, so the
// conflicts the tool classifies are the ones the merge left. Directory rename
// detection is off, so a merge never moves a file the two sides did not move;
// rerere is off, so no recorded resolution is applied without the tool seeing
// the conflict; the user's attributes file is not read, so only the
// repository's .gitattributes choose a merge driver (checkMergeDrivers checks
// those, and that no other attributes or default driver are set); and the
// rename limits of the merge and of the tool's own rename detection are the
// same, so both find the same renames.
var mergeConfig = []string{
	"-c", "merge.directoryRenames=false",
	"-c", "rerere.enabled=false",
	"-c", "core.attributesFile=/dev/null",
	"-c", "merge.renameLimit=" + renameLimit,
	"-c", "diff.renameLimit=" + renameLimit,
}

// renameLimit is the rename detection limit of every merge and diff of the
// take-in, git's default for a merge.
const renameLimit = "7000"

// PathConflictClass is the class of one conflicted path. The zero value is
// PathConflictOutOfScope, the class that stops the run.
type PathConflictClass int

// The classes of a conflicted path.
const (
	PathConflictOutOfScope   PathConflictClass = iota // any conflict the rules do not allow
	PathConflictMarkdown                              // a content conflict of a .md file
	PathConflictProtocolOnly                          // a content conflict of a protocol declaration line only
)

// String is the class name the output shows.
func (c PathConflictClass) String() string {
	switch c {
	case PathConflictMarkdown:
		return "markdown"
	case PathConflictProtocolOnly:
		return "protocol_only"
	default:
		return "out_of_scope"
	}
}

// MergemainStop is why a take-in subcommand stopped. The zero value is
// MergemainStopUnknown, which asks for no restore and is reported as is.
type MergemainStop int

// The reasons a take-in subcommand stops.
const (
	MergemainStopUnknown         MergemainStop = iota // a stop of no declared kind
	MergemainStopPremise                              // the PR or the work tree does not meet the premise
	MergemainStopNothingToTakeIn                      // origin/main is already an ancestor of the head
	MergemainStopOutOfScope                           // a conflict or a change the rules do not allow
	MergemainStopPushRejected                         // origin rejected the push and the branch is unchanged
	MergemainStopPushUnknown                          // whether the push landed cannot be told
	MergemainStopOtherChanges                         // something else changed the branch or the work tree
)

// String is the reason name the output shows.
func (s MergemainStop) String() string {
	switch s {
	case MergemainStopPremise:
		return "premise"
	case MergemainStopNothingToTakeIn:
		return "nothing_to_take_in"
	case MergemainStopOutOfScope:
		return "out_of_scope"
	case MergemainStopPushRejected:
		return "push_rejected"
	case MergemainStopPushUnknown:
		return "push_unknown"
	case MergemainStopOtherChanges:
		return "other_changes"
	default:
		return "unknown"
	}
}

// RestoreNeed is whether the agent must call mergemain-restore after a stop.
// The zero value is RestoreUnknown, which restores nothing.
type RestoreNeed int

// Whether a stop asks for a restore.
const (
	RestoreUnknown RestoreNeed = iota
	RestoreNotNeeded
	RestoreNeeded
)

// String is the restore name the output shows.
func (r RestoreNeed) String() string {
	switch r {
	case RestoreNotNeeded:
		return "not_needed"
	case RestoreNeeded:
		return "needed"
	default:
		return "unknown"
	}
}

// PathConflict is one conflicted path and its class.
type PathConflict struct {
	Path  string
	Class PathConflictClass
}

// MergemainStopError is a stop of a take-in subcommand: the reason, whether
// the agent must restore, what did not match, and, for a stop on conflicts,
// every conflicted path with its class.
type MergemainStopError struct {
	Stop      MergemainStop
	Restore   RestoreNeed
	Detail    string
	Conflicts []PathConflict
}

// Error names the reason, the restore need, and the detail.
func (e *MergemainStopError) Error() string {
	return fmt.Sprintf("mergemain stopped: %v (restore %v): %s", e.Stop, e.Restore, e.Detail)
}

// stopError builds a stop.
func stopError(stop MergemainStop, restore RestoreNeed, format string, args ...any) *MergemainStopError {
	return &MergemainStopError{Stop: stop, Restore: restore, Detail: fmt.Sprintf(format, args...)}
}

// MergemainPR is a PR number the take-in tool accepts: decimal digits only,
// without a sign or a leading zero, so the same PR always gives the same ref
// name. Only ParseMergemainPR makes one; the zero value names no PR.
type MergemainPR struct {
	number string
}

// ParseMergemainPR accepts one to nine decimal digits without a leading zero.
// Anything else, a sign, a space, or a leading zero included, is a premise
// stop that changed nothing.
func ParseMergemainPR(s string) (MergemainPR, error) {
	ok := len(s) >= 1 && len(s) <= maxPRDigits && s[0] != '0'
	for i := range len(s) {
		ok = ok && s[i] >= '0' && s[i] <= '9'
	}
	if !ok {
		return MergemainPR{}, stopError(MergemainStopPremise, RestoreNotNeeded, "the PR argument %q is not a decimal PR number", s)
	}
	return MergemainPR{number: s}, nil
}

// String returns the digits.
func (p MergemainPR) String() string { return p.number }

// mainRefOf is the ref begin records the fetched main in.
func (p MergemainPR) mainRefOf() string { return "refs/mergemain/" + p.number + "/main" }

// isCommitArg reports whether s is a full SHA-1 commit ID in lowercase
// hexadecimal, the only form a commit argument may take.
func isCommitArg(s string) bool {
	return len(s) == commitIDLength && isObjectID(s)
}

// Mergemain runs the take-in subcommands in the work tree its git runs in.
type Mergemain struct {
	git gitReader
	gh  Runner
	// lookupEnv reads this process's environment; tests substitute it.
	lookupEnv func(string) (string, bool)
}

func newMergemain(git, gh Runner, lookupEnv func(string) (string, bool)) *Mergemain {
	return &Mergemain{git: gitReader{run: git}, gh: gh, lookupEnv: lookupEnv}
}

// mergemainPR is the part of a PR the take-in tool reads.
type mergemainPR struct {
	headBranch string
	headOID    string
}

// readPR reads the PR through gh after checking that gh can only read it from
// origin's repository, and returns its head when it is open, into main, and
// from a branch of this repository.
//
// Contract: gh picks the repository from the git remotes and GH_REPO, so the
// PR is read only when origin is the only remote and GH_REPO is not set; only
// the members of mergemainPRFields are requested and read; every value is
// checked for its exact form, and any other shape is a premise stop.
func (m *Mergemain) readPR(ctx context.Context, pr MergemainPR, restore RestoreNeed) (mergemainPR, error) {
	if _, set := m.lookupEnv(ghRepoVariable); set {
		return mergemainPR{}, stopError(MergemainStopPremise, restore, "%s is set, so gh could read the PR from another repository than origin", ghRepoVariable)
	}
	out, err := m.git.git(ctx, "remote")
	if err != nil {
		return mergemainPR{}, stopError(MergemainStopPremise, restore, "could not list the remotes: %v", err)
	}
	if string(out) != originRemote+"\n" {
		return mergemainPR{}, stopError(MergemainStopPremise, restore, "the remotes are not origin alone, so gh could read the PR from another repository")
	}
	out, err = m.gh.Run(ctx, ghCommand, "pr", "view", pr.String(), "--json", mergemainPRFields)
	if err != nil {
		return mergemainPR{}, stopError(MergemainStopPremise, restore, "could not read PR #%s: %v", pr, err)
	}
	v, err := parseMergemainPR(out)
	if err != nil {
		return mergemainPR{}, stopError(MergemainStopPremise, restore, "PR #%s: %v", pr, err)
	}
	return v, nil
}

// errPRShape reports a PR value that is not the expected form or state.
var errPRShape = errors.New("the PR is not open, into main, and from a branch of this repository")

// parseMergemainPR reads gh's PR object strictly: each member present once
// with its JSON kind, the state OPEN, the base main, not a cross-repository
// head, a branch name of the accepted form other than main, and a full
// lowercase commit ID.
func parseMergemainPR(out []byte) (mergemainPR, error) {
	obj, err := strictjson.ParseObject(out)
	if err != nil {
		return mergemainPR{}, err
	}
	members, err := obj.Collect("state", "baseRefName", "headRefName", "headRefOid", "isCrossRepository")
	if err != nil {
		return mergemainPR{}, err
	}
	state, err := strictjson.RequiredString(members, "state")
	if err != nil {
		return mergemainPR{}, err
	}
	base, err := strictjson.RequiredString(members, "baseRefName")
	if err != nil {
		return mergemainPR{}, err
	}
	branch, err := strictjson.RequiredString(members, "headRefName")
	if err != nil {
		return mergemainPR{}, err
	}
	head, err := strictjson.RequiredString(members, "headRefOid")
	if err != nil {
		return mergemainPR{}, err
	}
	crossValue, err := strictjson.Required(members, "isCrossRepository")
	if err != nil {
		return mergemainPR{}, err
	}
	cross, err := crossValue.AsBool()
	if err != nil {
		return mergemainPR{}, err
	}
	switch {
	case state != openState:
		return mergemainPR{}, fmt.Errorf("%w: state %q", errPRShape, state)
	case base != mainBranch:
		return mergemainPR{}, fmt.Errorf("%w: base %q", errPRShape, base)
	case cross:
		return mergemainPR{}, fmt.Errorf("%w: the head is a branch of another repository", errPRShape)
	case !isCommitArg(head):
		return mergemainPR{}, fmt.Errorf("%w: head %q is not a full commit ID", errPRShape, head)
	}
	if err := checkBranchName(branch); err != nil {
		return mergemainPR{}, err
	}
	return mergemainPR{headBranch: branch, headOID: head}, nil
}

// errTakeInFile reports a file of the merge the take-in cannot work on: a
// tool's file still conflicted or without exactly one accepted declaration
// line, or a path that is not a regular file.
var errTakeInFile = errors.New("a file of the merge is not in the form the take-in works on")

// errBranchName reports a head branch the take-in tool does not work on.
var errBranchName = errors.New("the head branch is not a branch the take-in tool works on")

// checkBranchName accepts a branch name that is not main or HEAD in any case,
// does not start with "-", and holds no byte git check-ref-format rejects in
// a ref (a control character, a space, ~ ^ : ? * [ \). The full ref grammar is
// checked again by git when the name is used.
func checkBranchName(branch string) error {
	if branch == "" || branch == mainBranch || strings.EqualFold(branch, "HEAD") || strings.HasPrefix(branch, "-") {
		return fmt.Errorf("%w: %q", errBranchName, branch)
	}
	for i := range len(branch) {
		if c := branch[i]; c < 0x21 || c == 0x7f || strings.IndexByte(`~^:?*[\`, c) >= 0 {
			return fmt.Errorf("%w: %q", errBranchName, branch)
		}
	}
	return nil
}

// checkRefFormat runs git check-ref-format on the branch's ref, so a name git
// would not accept as a branch is refused before it reaches a refspec.
func (m *Mergemain) checkRefFormat(ctx context.Context, branch string, restore RestoreNeed) error {
	if err := checkBranchName(branch); err != nil {
		return stopError(MergemainStopPremise, restore, "%v", err)
	}
	if _, err := m.git.git(ctx, "check-ref-format", headsPrefix+branch); err != nil {
		return stopError(MergemainStopPremise, restore, "the head branch %q is not a valid branch name", branch)
	}
	return nil
}

// MergemainBeginResult is what begin passes to the later subcommands: the
// base head, the PR's head branch, and the main to take in.
type MergemainBeginResult struct {
	BaseHead   string
	HeadBranch string
	Main       string
}

// MergemainBegin checks the premise of a take-in and fetches what it needs.
//
// Contract: it changes no branch, no index, and no file of the work tree; the
// only writes are the two refs its fetch names: main into
// refs/mergemain/<PR>/main and the head branch into
// refs/remotes/origin/<branch>. Every stop is a premise stop (or nothing to
// take in) that asks for no restore.
func (m *Mergemain) MergemainBegin(ctx context.Context, pr MergemainPR) (MergemainBeginResult, error) {
	if pr.number == "" {
		return MergemainBeginResult{}, stopError(MergemainStopPremise, RestoreNotNeeded, "no PR number")
	}
	if err := m.checkTop(ctx, RestoreNotNeeded); err != nil {
		return MergemainBeginResult{}, err
	}
	v, err := m.readPR(ctx, pr, RestoreNotNeeded)
	if err != nil {
		return MergemainBeginResult{}, err
	}
	if err := m.checkRefFormat(ctx, v.headBranch, RestoreNotNeeded); err != nil {
		return MergemainBeginResult{}, err
	}
	if err := m.checkStartState(ctx, v.headBranch); err != nil {
		return MergemainBeginResult{}, err
	}
	remoteRef := remoteBranchPrefix + v.headBranch
	if _, err := m.git.git(ctx, "fetch", "-q", "--no-tags", "--no-prune", "--no-recurse-submodules", "--refmap=", "--", originRemote,
		"+"+headsPrefix+mainBranch+":"+pr.mainRefOf(), "+"+headsPrefix+v.headBranch+":"+remoteRef); err != nil {
		return MergemainBeginResult{}, stopError(MergemainStopPremise, RestoreNotNeeded, "could not fetch main and %s from origin: %v", v.headBranch, err)
	}
	main, err := m.refCommit(ctx, pr.mainRefOf())
	if err != nil {
		return MergemainBeginResult{}, stopError(MergemainStopPremise, RestoreNotNeeded, "could not read %s: %v", pr.mainRefOf(), err)
	}
	remoteHead, err := m.refCommit(ctx, remoteRef)
	if err != nil {
		return MergemainBeginResult{}, stopError(MergemainStopPremise, RestoreNotNeeded, "could not read %s: %v", remoteRef, err)
	}
	head, err := m.git.resolveCommit(ctx, "HEAD")
	if err != nil {
		return MergemainBeginResult{}, stopError(MergemainStopPremise, RestoreNotNeeded, "could not read HEAD: %v", err)
	}
	if head != v.headOID || remoteHead != v.headOID {
		return MergemainBeginResult{}, stopError(MergemainStopPremise, RestoreNotNeeded, "HEAD %s, %s %s, and the PR head %s are not one commit", head, remoteRef, remoteHead, v.headOID)
	}
	if err := m.checkSomethingToTakeIn(ctx, head, main, RestoreNotNeeded); err != nil {
		return MergemainBeginResult{}, err
	}
	return MergemainBeginResult{BaseHead: head, HeadBranch: v.headBranch, Main: main}, nil
}

// checkTop stops unless the tool runs at the top of a work tree of a full
// (not shallow) repository, so every path it reads and writes is relative to
// the top and merge bases are computed over the whole history.
func (m *Mergemain) checkTop(ctx context.Context, restore RestoreNeed) error {
	out, err := m.git.git(ctx, "rev-parse", "--show-prefix", "--is-shallow-repository")
	if err != nil {
		return stopError(MergemainStopPremise, restore, "not in a git work tree: %v", err)
	}
	if string(out) != "\nfalse\n" {
		return stopError(MergemainStopPremise, restore, "not at the top of the work tree of a full repository")
	}
	return nil
}

// checkStartState stops unless HEAD is on the branch, no merge is in
// progress, no index entry hides a change, and the work tree has no change,
// an untracked file included.
func (m *Mergemain) checkStartState(ctx context.Context, branch string) error {
	if err := m.checkOnBranch(ctx, branch, MergemainStopPremise, RestoreNotNeeded); err != nil {
		return err
	}
	if _, inMerge, err := m.mergeHead(ctx); err != nil || inMerge {
		return stopError(MergemainStopPremise, RestoreNotNeeded, "a merge is in progress or MERGE_HEAD cannot be read")
	}
	out, err := m.git.git(ctx, "ls-files", "-v", "-z")
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNotNeeded, "could not list the index: %v", err)
	}
	for entry := range bytes.SplitSeq(out, []byte{0}) {
		if len(entry) > 0 && (entry[0] == 'S' || ('a' <= entry[0] && entry[0] <= 'z')) {
			return stopError(MergemainStopPremise, RestoreNotNeeded, "an index entry has skip-worktree or assume-unchanged set")
		}
	}
	clean, err := m.workTreeClean(ctx, true)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNotNeeded, "could not read the work tree status: %v", err)
	}
	if !clean {
		return stopError(MergemainStopPremise, RestoreNotNeeded, "the work tree has changes or untracked files")
	}
	return nil
}

// checkOnBranch stops with stop unless HEAD is the branch.
func (m *Mergemain) checkOnBranch(ctx context.Context, branch string, stop MergemainStop, restore RestoreNeed) error {
	out, err := m.git.git(ctx, "symbolic-ref", "-q", "HEAD")
	if err != nil || string(out) != headsPrefix+branch+"\n" {
		return stopError(stop, restore, "HEAD is not the branch %s", branch)
	}
	return nil
}

// workTreeClean reports whether git status lists nothing: no tracked change,
// and, when untracked is true, no untracked file that is not ignored.
func (m *Mergemain) workTreeClean(ctx context.Context, untracked bool) (bool, error) {
	mode := "--untracked-files=no"
	if untracked {
		mode = "--untracked-files=all"
	}
	out, err := m.git.git(ctx, "status", "--porcelain=v1", "-z", "--ignore-submodules=none", mode)
	if err != nil {
		return false, err
	}
	return len(out) == 0, nil
}

// mergeHead returns MERGE_HEAD and whether a merge is in progress.
func (m *Mergemain) mergeHead(ctx context.Context) (string, bool, error) {
	out, err := m.git.git(ctx, "rev-parse", "-q", "--verify", "MERGE_HEAD^{commit}")
	if err != nil {
		if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 && len(exitErr.Stdout) == 0 {
			return "", false, nil
		}
		return "", false, err
	}
	oid := strings.TrimSuffix(string(out), "\n")
	if !isObjectID(oid) {
		return "", false, fmt.Errorf("%w: MERGE_HEAD %q", errUnexpectedGitOutput, out)
	}
	return oid, true, nil
}

// refCommit returns the commit ref points at, read with show-ref --verify so
// a missing ref is never read as another revision.
func (m *Mergemain) refCommit(ctx context.Context, ref string) (string, error) {
	out, err := m.git.git(ctx, "show-ref", "--verify", "--hash", ref)
	if err != nil {
		return "", err
	}
	oid := strings.TrimSuffix(string(out), "\n")
	if !isObjectID(oid) {
		return "", fmt.Errorf("%w: show-ref printed %q", errUnexpectedGitOutput, out)
	}
	return m.git.resolveCommit(ctx, oid)
}

// isAncestor reports whether a is an ancestor of b (or b itself).
func (m *Mergemain) isAncestor(ctx context.Context, a, b string) (bool, error) {
	_, err := m.git.git(ctx, "merge-base", "--is-ancestor", a, b)
	if err == nil {
		return true, nil
	}
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code == 1 {
		return false, nil
	}
	return false, err
}

// checkSomethingToTakeIn stops when main is already an ancestor of head.
func (m *Mergemain) checkSomethingToTakeIn(ctx context.Context, head, main string, restore RestoreNeed) error {
	inHead, err := m.isAncestor(ctx, main, head)
	if err != nil {
		return stopError(MergemainStopPremise, restore, "could not compare main with the head: %v", err)
	}
	if inHead {
		return stopError(MergemainStopNothingToTakeIn, RestoreNotNeeded, "main %s is already an ancestor of the head %s", main, head)
	}
	return nil
}

// mergeBase returns the one merge base of a and b; two or more (a criss-cross
// history) is an error, because the merge would then use a virtual base the
// rename detection cannot compare with.
func (m *Mergemain) mergeBase(ctx context.Context, a, b string) (string, error) {
	out, err := m.git.git(ctx, "merge-base", "--all", a, b)
	if err != nil {
		return "", err
	}
	text := strings.TrimSuffix(string(out), "\n")
	if text == "" || strings.Contains(text, "\n") || !isObjectID(text) {
		return "", fmt.Errorf("%w: not exactly one merge base: %q", errUnexpectedGitOutput, out)
	}
	return text, nil
}

// MergemainMergeResult is what merge reports when it leaves the merge for the
// agent to finish: the conflicts and their classes (Markdown ones for the
// agent, protocol-only ones already resolved), the protocol numbers it set,
// and the facts the README, ADR, and ext-check rules need.
type MergemainMergeResult struct {
	Conflicts []PathConflict
	// Protocol is each tool whose number the merge set, with the new value,
	// sorted by tool.
	Protocol []ProtocolBump
	// ReadmeChanged is true when the merge changed or conflicted on README.md
	// or README.en.md.
	ReadmeChanged bool
	// ADRMainChanged is true when main changed docs/dev/adr/ after the merge
	// base.
	ADRMainChanged bool
	// ADRConflictNumbers are the ADR numbers the branch added that main's ADRs
	// also use, sorted.
	ADRConflictNumbers []string
	// ExtCheckHint is true when the merge changed extension/ or a file the
	// extension's tests read.
	ExtCheckHint bool
}

// ProtocolBump is one tool whose number the take-in sets, and its value.
type ProtocolBump struct {
	Tool  string
	Value int
}

// mergeArgs are the values begin passed on, checked for their form.
type mergeArgs struct {
	pr       MergemainPR
	branch   string
	baseHead string
	main     string
}

// checkMergeArgs checks the form of the arguments merge and restore take.
func (m *Mergemain) checkMergeArgs(ctx context.Context, a mergeArgs, stop MergemainStop) error {
	if a.pr.number == "" {
		return stopError(stop, RestoreNotNeeded, "no PR number")
	}
	if !isCommitArg(a.baseHead) || !isCommitArg(a.main) {
		return stopError(stop, RestoreNotNeeded, "the base head and main must be full lowercase commit IDs")
	}
	return m.checkRefFormat(ctx, a.branch, RestoreNotNeeded)
}

// MergemainMerge merges main into the branch without committing and
// classifies the conflicts.
//
// Contract: before the merge starts, every stop asks for no restore and the
// work tree is unchanged; once the merge has started, every stop asks for a
// restore. The merge is never a fast-forward and never commits. A conflict out
// of scope stops before anything is resolved; otherwise the tool resolves the
// protocol-only conflicts and sets the number of every tool the branch
// changed since the merge base to main's number plus one, and leaves the
// Markdown conflicts to the agent.
func (m *Mergemain) MergemainMerge(ctx context.Context, pr MergemainPR, headBranch, baseHead, main string) (MergemainMergeResult, error) {
	a := mergeArgs{pr: pr, branch: headBranch, baseHead: baseHead, main: main}
	base, expected, err := m.checkBeforeMerge(ctx, a)
	if err != nil {
		return MergemainMergeResult{}, err
	}

	// The merge starts here; from now on a stop asks for a restore.
	mergeErr := m.runMerge(ctx, main)
	if err := m.checkMergeStarted(ctx, a, mergeErr); err != nil {
		return MergemainMergeResult{}, err
	}
	entries, err := m.unmergedEntries(ctx)
	if err != nil {
		return MergemainMergeResult{}, stopError(MergemainStopPremise, RestoreNeeded, "could not list the conflicts: %v", err)
	}
	if mergeErr != nil && len(entries) == 0 {
		return MergemainMergeResult{}, stopError(MergemainStopOutOfScope, RestoreNeeded, "the merge failed without leaving a conflicted path: %v", mergeErr)
	}
	conflicts, resolved, err := m.classifyConflicts(ctx, base, baseHead, main, entries, expected)
	if err != nil {
		return MergemainMergeResult{}, stopError(MergemainStopPremise, RestoreNeeded, "could not classify the conflicts: %v", err)
	}
	if slices.ContainsFunc(conflicts, func(c PathConflict) bool { return c.Class == PathConflictOutOfScope }) {
		stop := stopError(MergemainStopOutOfScope, RestoreNeeded, "a conflict is out of scope")
		stop.Conflicts = conflicts
		return MergemainMergeResult{}, stop
	}
	bumps, err := m.setProtocolNumbers(ctx, expected, resolved)
	if err != nil {
		return MergemainMergeResult{}, stopError(MergemainStopOutOfScope, RestoreNeeded, "could not set the protocol numbers: %v", err)
	}
	result := MergemainMergeResult{Conflicts: conflicts, Protocol: bumps}
	if err := m.collectFacts(ctx, &result, base, baseHead, main, conflicts); err != nil {
		return MergemainMergeResult{}, stopError(MergemainStopPremise, RestoreNeeded, "could not read what the merge changed: %v", err)
	}
	return result, nil
}

// checkBeforeMerge checks everything merge needs before it changes anything
// and returns the merge base and the tools whose number the take-in sets.
// Every stop asks for no restore.
func (m *Mergemain) checkBeforeMerge(ctx context.Context, a mergeArgs) (string, map[string]int, error) {
	if err := m.checkMergeArgs(ctx, a, MergemainStopPremise); err != nil {
		return "", nil, err
	}
	if err := m.checkTop(ctx, RestoreNotNeeded); err != nil {
		return "", nil, err
	}
	if err := m.checkStartState(ctx, a.branch); err != nil {
		return "", nil, err
	}
	head, err := m.git.resolveCommit(ctx, "HEAD")
	if err != nil || head != a.baseHead {
		return "", nil, stopError(MergemainStopPremise, RestoreNotNeeded, "HEAD is not the base head %s", a.baseHead)
	}
	recorded, err := m.refCommit(ctx, a.pr.mainRefOf())
	if err != nil || recorded != a.main {
		return "", nil, stopError(MergemainStopPremise, RestoreNotNeeded, "main %s is not the main begin recorded in %s", a.main, a.pr.mainRefOf())
	}
	if err := m.checkSomethingToTakeIn(ctx, a.baseHead, a.main, RestoreNotNeeded); err != nil {
		return "", nil, err
	}
	base, err := m.mergeBase(ctx, a.baseHead, a.main)
	if err != nil {
		return "", nil, stopError(MergemainStopPremise, RestoreNotNeeded, "could not find one merge base: %v", err)
	}
	expected, err := m.branchProtocolTools(ctx, a.baseHead, a.main)
	if err != nil {
		return "", nil, stopError(MergemainStopPremise, RestoreNotNeeded, "could not read the protocol numbers: %v", err)
	}
	if err := m.checkMergeDrivers(ctx, base, a.baseHead, a.main, RestoreNotNeeded); err != nil {
		return "", nil, err
	}
	if err := m.checkIgnoredNotOverwritten(ctx, a.main, a.baseHead); err != nil {
		return "", nil, err
	}
	return base, expected, nil
}

// checkMergeStarted stops unless the merge left MERGE_HEAD at main. git
// refuses some merges before it changes anything; then HEAD is still the base
// head and the work tree clean, and the stop asks for no restore, which would
// refuse a state without MERGE_HEAD.
func (m *Mergemain) checkMergeStarted(ctx context.Context, a mergeArgs, mergeErr error) error {
	mergeHead, inMerge, err := m.mergeHead(ctx)
	if err == nil && inMerge && mergeHead == a.main {
		return nil
	}
	restore := RestoreNeeded
	if head, headErr := m.git.resolveCommit(ctx, "HEAD"); err == nil && !inMerge && headErr == nil && head == a.baseHead {
		if clean, cleanErr := m.workTreeClean(ctx, true); cleanErr == nil && clean {
			restore = RestoreNotNeeded
		}
	}
	return stopError(MergemainStopPremise, restore, "the merge did not leave MERGE_HEAD at main: %v", errors.Join(mergeErr, err))
}

// runMerge merges main into HEAD in the work tree: never a fast-forward, never
// a commit, never overwriting an ignored file, with the ort strategy and the
// take-in configuration. The labels of its conflict markers are HEAD and the
// commit ID, the labels restore computes the expected merge with.
func (m *Mergemain) runMerge(ctx context.Context, main string) error {
	// --no-overwrite-ignore backs up checkIgnoredNotOverwritten, which stops
	// such a merge before it starts.
	args := slices.Concat(mergeConfig, []string{mergeSubcommand, "-q", "--no-ff", "--no-commit", "--no-squash", "--no-edit", "--no-autostash", "--no-overwrite-ignore", "--strategy=ort", main})
	_, err := m.git.git(ctx, args...)
	return err
}

// stageEntry is one index entry of a conflicted path.
type stageEntry struct {
	mode  string
	oid   string
	stage int
}

// parseStageEntries reads NUL-terminated "<mode> <object> <stage>\t<path>"
// records, the form of ls-files -s -z and of merge-tree's conflicted file
// information, into the entries of each path.
func parseStageEntries(records [][]byte) (map[string][]stageEntry, error) {
	entries := make(map[string][]stageEntry)
	for _, record := range records {
		info, name, ok := bytes.Cut(record, []byte{'\t'})
		fields := strings.Split(string(info), " ")
		if !ok || len(name) == 0 || len(fields) != 3 || !isObjectID(fields[1]) || len(fields[2]) != 1 || fields[2][0] < '0' || fields[2][0] > '3' {
			return nil, fmt.Errorf("%w: stage entry %q", errUnexpectedGitOutput, record)
		}
		p := string(name)
		entries[p] = append(entries[p], stageEntry{mode: fields[0], oid: fields[1], stage: int(fields[2][0] - '0')})
	}
	return entries, nil
}

// splitNUL splits NUL-terminated records; output that does not end with a
// NUL is an error, never a shorter list.
func splitNUL(out []byte) ([][]byte, error) {
	if len(out) == 0 {
		return nil, nil
	}
	if out[len(out)-1] != 0 {
		return nil, fmt.Errorf("%w: output does not end with a NUL", errUnexpectedGitOutput)
	}
	return bytes.Split(out[:len(out)-1], []byte{0}), nil
}

// unmergedEntries returns the index entries of the conflicted paths.
func (m *Mergemain) unmergedEntries(ctx context.Context) (map[string][]stageEntry, error) {
	out, err := m.git.git(ctx, "ls-files", "-u", "-z")
	if err != nil {
		return nil, err
	}
	records, err := splitNUL(out)
	if err != nil {
		return nil, err
	}
	return parseStageEntries(records)
}

// resolvedProtocol is a protocol-only conflict with the merged content of its
// three versions, the declaration line replaced by protocolPlaceholder.
type resolvedProtocol struct {
	tool   string
	merged []byte
}

// classifyConflicts classifies every conflicted path (3.8.2 of the design).
// It returns the conflicts sorted by path and, for each protocol-only
// conflict, the merged content apart from the declaration line.
//
// Contract: a path is in scope only when every stage is a regular file, no
// stage is binary to git, the path is no side's rename source or
// destination, and it is either a .md file with all three stages or both
// sides' additions, or a file directly inside internal/<name>/ (not
// _test.go) of a tool in expected whose three versions each hold one
// accepted declaration line and merge without conflict apart from it. Every
// other path is out of scope; the zero class is out of scope too.
func (m *Mergemain) classifyConflicts(ctx context.Context, base, ours, theirs string, entries map[string][]stageEntry, expected map[string]int) ([]PathConflict, map[string]resolvedProtocol, error) {
	if len(entries) == 0 {
		return nil, nil, nil
	}
	renamed, err := m.renamedPaths(ctx, base, ours, theirs)
	if err != nil {
		return nil, nil, err
	}
	paths := slices.Sorted(maps.Keys(entries))
	attrs, err := m.mergeAttributes(ctx, "", paths)
	if err != nil {
		return nil, nil, err
	}
	conflicts := make([]PathConflict, 0, len(paths))
	resolved := make(map[string]resolvedProtocol)
	for _, p := range paths {
		class, merged, err := m.classifyPath(ctx, p, entries[p], renamed, attrs[p], expected)
		if err != nil {
			return nil, nil, err
		}
		conflicts = append(conflicts, PathConflict{Path: p, Class: class})
		if class == PathConflictProtocolOnly {
			pkg, _ := toolPackage(p)
			resolved[p] = resolvedProtocol{tool: pkg, merged: merged}
		}
	}
	return conflicts, resolved, nil
}

// classifyPath classifies one conflicted path; merged is set only for a
// protocol-only conflict.
func (m *Mergemain) classifyPath(ctx context.Context, p string, stages []stageEntry, renamed map[string]struct{}, mergeAttr string, expected map[string]int) (PathConflictClass, []byte, error) {
	byStage := make(map[int]stageEntry)
	for _, e := range stages {
		if _, dup := byStage[e.stage]; dup || e.stage == 0 {
			return PathConflictOutOfScope, nil, nil
		}
		byStage[e.stage] = e
	}
	if _, ok := renamed[p]; ok {
		return PathConflictOutOfScope, nil, nil
	}
	// A merge attribute other than the default text merge (unset, binary, or
	// a custom driver) is not a content conflict the rules cover.
	if mergeAttr != "unspecified" && mergeAttr != "set" {
		return PathConflictOutOfScope, nil, nil
	}
	contents := make(map[int][]byte)
	for stage, e := range byStage {
		if e.mode != regularMode && e.mode != executableMode {
			return PathConflictOutOfScope, nil, nil
		}
		blob, err := m.blob(ctx, e.oid)
		if err != nil {
			return PathConflictOutOfScope, nil, err
		}
		if isBinary(blob) {
			return PathConflictOutOfScope, nil, nil
		}
		contents[stage] = blob
	}
	_, has1 := byStage[1]
	_, has2 := byStage[2]
	_, has3 := byStage[3]
	if !has2 || !has3 {
		return PathConflictOutOfScope, nil, nil // modify/delete
	}
	if strings.HasSuffix(p, markdownSuffix) {
		return PathConflictMarkdown, nil, nil
	}
	pkg, isTool := toolPackage(p)
	if _, changed := expected[pkg]; !isTool || !changed || !has1 {
		return PathConflictOutOfScope, nil, nil
	}
	merged, clean, err := m.mergeApartFromProtocol(ctx, contents[1], contents[2], contents[3])
	if err != nil {
		return PathConflictOutOfScope, nil, err
	}
	if !clean {
		return PathConflictOutOfScope, nil, nil
	}
	return PathConflictProtocolOnly, merged, nil
}

// checkMergeDrivers stops unless git merges every path both sides changed
// with its default text merge, which reports every conflict: no
// $GIT_DIR/info/attributes and no merge.default driver may be set, and the
// merge attribute of each such path and of each rename, read from the
// .gitattributes of ours, of theirs, and of the index, must be unspecified or
// "set". A driver such as union, or a configured one, would
// otherwise resolve a conflict of any file without the tool seeing it.
func (m *Mergemain) checkMergeDrivers(ctx context.Context, base, ours, theirs string, restore RestoreNeed) error {
	if err := m.checkDriverConfig(ctx, restore); err != nil {
		return err
	}
	both, err := m.changedOnBothSides(ctx, base, ours, theirs)
	if err != nil {
		return stopError(MergemainStopPremise, restore, "could not list the paths both sides changed: %v", err)
	}
	// A renamed file is merged under its new path, whose attribute chooses
	// the driver, so every rename source and destination is checked too.
	renamed, err := m.renamedPaths(ctx, base, ours, theirs)
	if err != nil {
		return stopError(MergemainStopPremise, restore, "could not list the renames: %v", err)
	}
	for p := range renamed {
		if !slices.Contains(both, p) {
			both = append(both, p)
		}
	}
	slices.Sort(both)
	if len(both) == 0 {
		return nil
	}
	// Which .gitattributes a merge reads differs between git merge and git
	// merge-tree, so the attribute must be the text merge in all of them:
	// ours, theirs, and the index.
	var conflicts []PathConflict
	for _, source := range []string{ours, theirs, ""} {
		attrs, err := m.mergeAttributes(ctx, source, both)
		if err != nil {
			return stopError(MergemainStopPremise, restore, "could not read the merge attributes: %v", err)
		}
		for _, p := range both {
			if a := attrs[p]; a != "unspecified" && a != "set" && !slices.ContainsFunc(conflicts, func(c PathConflict) bool { return c.Path == p }) {
				conflicts = append(conflicts, PathConflict{Path: p, Class: PathConflictOutOfScope})
			}
		}
	}
	if len(conflicts) > 0 {
		stop := stopError(MergemainStopOutOfScope, restore, "a path both sides changed has a merge driver other than the text merge")
		stop.Conflicts = conflicts
		return stop
	}
	return nil
}

// checkDriverConfig stops when $GIT_DIR/info/attributes is not empty or
// merge.default is set: either could choose another merge driver for any path.
func (m *Mergemain) checkDriverConfig(ctx context.Context, restore RestoreNeed) error {
	out, err := m.git.git(ctx, "rev-parse", "--git-path", "info/attributes")
	if err != nil {
		return stopError(MergemainStopPremise, restore, "could not find info/attributes: %v", err)
	}
	attrsPath := strings.TrimSuffix(string(out), "\n")
	if !filepath.IsAbs(attrsPath) {
		top, err := m.top(ctx)
		if err != nil {
			return stopError(MergemainStopPremise, restore, "could not find the top of the work tree: %v", err)
		}
		attrsPath = filepath.Join(top, attrsPath)
	}
	if info, err := os.Stat(attrsPath); err == nil && info.Size() > 0 {
		return stopError(MergemainStopPremise, restore, "$GIT_DIR/info/attributes is set and could choose a merge driver")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return stopError(MergemainStopPremise, restore, "could not read info/attributes: %v", err)
	}
	if _, err := m.git.git(ctx, "config", "--get", "merge.default"); err == nil {
		return stopError(MergemainStopPremise, restore, "merge.default is set and would merge files with another driver")
	} else if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); !ok || exitErr.Code != 1 {
		return stopError(MergemainStopPremise, restore, "could not read merge.default: %v", err)
	}
	return nil
}

// changedOnBothSides returns the paths that both ours and theirs changed
// since base, sorted.
func (m *Mergemain) changedOnBothSides(ctx context.Context, base, ours, theirs string) ([]string, error) {
	var sets []map[string]struct{}
	for _, side := range []string{ours, theirs} {
		out, err := m.git.git(ctx, "diff-tree", "-r", "-z", "--name-only", "--no-renames", base, side)
		if err != nil {
			return nil, err
		}
		records, err := splitNUL(out)
		if err != nil {
			return nil, err
		}
		set := make(map[string]struct{}, len(records))
		for _, r := range records {
			set[string(r)] = struct{}{}
		}
		sets = append(sets, set)
	}
	var both []string
	for p := range sets[0] {
		if _, ok := sets[1][p]; ok {
			both = append(both, p)
		}
	}
	slices.Sort(both)
	return both, nil
}

// binaryProbe is how many leading bytes git looks at to tell a binary file:
// one with a NUL among them.
const binaryProbe = 8000

// isBinary reports whether git would read blob as binary content.
func isBinary(blob []byte) bool {
	return bytes.IndexByte(blob[:min(len(blob), binaryProbe)], 0) >= 0
}

// blob returns the content of a blob.
func (m *Mergemain) blob(ctx context.Context, oid string) ([]byte, error) {
	if !isObjectID(oid) {
		return nil, fmt.Errorf("%w: blob %q", errUnexpectedGitOutput, oid)
	}
	return m.git.git(ctx, "cat-file", "blob", oid)
}

// mergeAttributes returns the merge attribute of each path as the
// .gitattributes of the commit source give it, or of the index when source
// is empty: "unspecified", "set", "unset", or a driver name.
func (m *Mergemain) mergeAttributes(ctx context.Context, source string, paths []string) (map[string]string, error) {
	from := "--cached"
	if source != "" {
		from = "--source=" + source
	}
	out, err := m.git.git(ctx, slices.Concat(mergeConfig, []string{"check-attr", "-z", from, mergeAttribute, "--"}, paths)...)
	if err != nil {
		return nil, err
	}
	records, err := splitNUL(out)
	if err != nil {
		return nil, err
	}
	if len(records)%3 != 0 {
		return nil, fmt.Errorf("%w: check-attr printed %d fields", errUnexpectedGitOutput, len(records))
	}
	attrs := make(map[string]string)
	for i := 0; i < len(records); i += 3 {
		if string(records[i+1]) != mergeAttribute {
			return nil, fmt.Errorf("%w: check-attr attribute %q", errUnexpectedGitOutput, records[i+1])
		}
		attrs[string(records[i])] = string(records[i+2])
	}
	for _, p := range paths {
		if _, ok := attrs[p]; !ok {
			return nil, fmt.Errorf("%w: check-attr did not report %q", errUnexpectedGitOutput, p)
		}
	}
	return attrs, nil
}

// renamedPaths returns every path that is a rename source or destination
// between base and either side, found with git's default rename detection,
// the one the merge uses.
func (m *Mergemain) renamedPaths(ctx context.Context, base string, sides ...string) (map[string]struct{}, error) {
	renamed := make(map[string]struct{})
	for _, side := range sides {
		out, err := m.git.git(ctx, slices.Concat(mergeConfig, []string{"diff-tree", "-r", "-z", "-M", "-l" + renameLimit, "--name-status", "--no-ext-diff", "--no-textconv", base, side})...)
		if err != nil {
			return nil, err
		}
		records, err := splitNUL(out)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(records); {
			status := string(records[i])
			switch {
			case strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C"):
				if i+2 >= len(records) {
					return nil, fmt.Errorf("%w: diff-tree rename record", errUnexpectedGitOutput)
				}
				renamed[string(records[i+1])] = struct{}{}
				renamed[string(records[i+2])] = struct{}{}
				i += 3
			case status != "" && i+1 < len(records):
				i += 2
			default:
				return nil, fmt.Errorf("%w: diff-tree record %q", errUnexpectedGitOutput, status)
			}
		}
	}
	return renamed, nil
}

// stripProtocol replaces the accepted declaration line of content with
// protocolPlaceholder. It returns false when content holds a declaration line
// that is not the accepted form, or more than one; a content without any
// declaration line is returned unchanged with count 0.
func stripProtocol(content []byte) (stripped []byte, count int, ok bool) {
	lines := strings.SplitAfter(string(content), "\n")
	var b strings.Builder
	for _, line := range lines {
		text := strings.TrimSuffix(line, "\n")
		if strings.Contains(text, protocolWord) && declarationLine.MatchString(text) {
			if !acceptedLine.MatchString(text) {
				return nil, 0, false
			}
			count++
			b.WriteString(protocolPlaceholder)
			b.WriteString(line[len(text):])
			continue
		}
		b.WriteString(line)
	}
	if count > 1 {
		return nil, 0, false
	}
	return []byte(b.String()), count, true
}

// mergeApartFromProtocol merges three versions of a tool's file with their
// declaration lines replaced by the placeholder. It reports false when a
// version does not hold exactly one accepted declaration line, when the
// merge conflicts, or when the result does not hold the placeholder exactly
// once.
func (m *Mergemain) mergeApartFromProtocol(ctx context.Context, base, ours, theirs []byte) ([]byte, bool, error) {
	var stripped [3][]byte
	for i, content := range [][]byte{base, ours, theirs} {
		s, count, ok := stripProtocol(content)
		if !ok || count != 1 {
			return nil, false, nil
		}
		stripped[i] = s
	}
	merged, clean, err := m.mergeFile(ctx, stripped[0], stripped[1], stripped[2])
	if err != nil || !clean {
		return nil, false, err
	}
	if _, count, ok := stripProtocol(merged); !ok || count != 1 {
		return nil, false, nil
	}
	return merged, true, nil
}

// mergeFile runs git merge-file on three contents written to a temporary
// directory outside the work tree, which it removes before returning.
func (m *Mergemain) mergeFile(ctx context.Context, base, ours, theirs []byte) ([]byte, bool, error) {
	dir, err := os.MkdirTemp("", "mergemain-")
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	names := []string{"ours", "base", "theirs"}
	for i, content := range [][]byte{ours, base, theirs} {
		if err := os.WriteFile(filepath.Join(dir, names[i]), content, fileMode); err != nil {
			return nil, false, err
		}
	}
	out, err := m.git.git(ctx, "merge-file", "-p", "-q", filepath.Join(dir, "ours"), filepath.Join(dir, "base"), filepath.Join(dir, "theirs"))
	if err == nil {
		return out, true, nil
	}
	// merge-file exits with the number of conflicts, and with a negative
	// status (255 here) on an error.
	if exitErr, ok := errors.AsType[*cmdrunner.ExitError](err); ok && exitErr.Code > 0 && exitErr.Code < 128 {
		return nil, false, nil
	}
	return nil, false, err
}

// branchProtocolTools returns each tool whose number the branch changed since
// it last took in main, with the number it must have after the take-in:
// main's number plus one (1 for a tool main does not have).
//
// Contract: "changed" is the rule of the merge's protocol-number check
// (commitChange) over the commits of head that main does not reach; a tool
// the branch removed is left out; an unreadable declaration at main or the
// head is an error, never an unchanged tool.
func (m *Mergemain) branchProtocolTools(ctx context.Context, head, main string) (map[string]int, error) {
	check := &protocolCheck{m.git}
	commits, err := m.git.listPRCommits(ctx, head, main, rangeOrderDefault)
	if err != nil {
		return nil, err
	}
	readings, err := check.readRevisions(ctx, main, head, commits)
	if err != nil {
		return nil, err
	}
	if err := checkReadable(main, readings[main], head, readings[head]); err != nil {
		return nil, err
	}
	expected := make(map[string]int)
	for pkg, changes := range changedTools(commits, readings) {
		if valueOf(readings[head], pkg).State == protocolAbsent {
			if removedBy(changes) {
				continue
			}
			return nil, &protocolUnreadableError{Kind: unreadableHeadMissing, Rev: head, Detail: "tool " + pkg}
		}
		expected[pkg] = expectedProtocol(valueOf(readings[main], pkg))
	}
	return expected, nil
}

// checkIgnoredNotOverwritten stops, before the merge, when an ignored file
// of the work tree is at a path main tracks and head does not, or is a
// directory or a parent of such a path: the merge would overwrite it.
func (m *Mergemain) checkIgnoredNotOverwritten(ctx context.Context, main, head string) error {
	ignored, err := m.ignoredPaths(ctx)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNotNeeded, "could not list the ignored files: %v", err)
	}
	if len(ignored) == 0 {
		return nil
	}
	mainPaths, err := m.treePaths(ctx, main)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNotNeeded, "could not list main's files: %v", err)
	}
	headPaths, err := m.treePaths(ctx, head)
	if err != nil {
		return stopError(MergemainStopPremise, RestoreNotNeeded, "could not list the head's files: %v", err)
	}
	for p := range mainPaths {
		if _, tracked := headPaths[p]; tracked {
			delete(mainPaths, p)
		}
	}
	if p, ok := blockingPath(ignored, mainPaths); ok {
		return stopError(MergemainStopPremise, RestoreNotNeeded, "the ignored file %s is where main tracks a file", p)
	}
	return nil
}

// ignoredPaths lists the ignored untracked files of the work tree, an ignored
// directory as one entry ending in "/".
func (m *Mergemain) ignoredPaths(ctx context.Context) ([]string, error) {
	out, err := m.git.git(ctx, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--directory")
	if err != nil {
		return nil, err
	}
	records, err := splitNUL(out)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(records))
	for _, r := range records {
		paths = append(paths, string(r))
	}
	return paths, nil
}

// treePaths lists the paths of the files a commit tracks.
func (m *Mergemain) treePaths(ctx context.Context, commit string) (map[string]struct{}, error) {
	out, err := m.git.git(ctx, "ls-tree", "-r", "-z", "--full-tree", "--name-only", commit)
	if err != nil {
		return nil, err
	}
	records, err := splitNUL(out)
	if err != nil {
		return nil, err
	}
	paths := make(map[string]struct{}, len(records))
	for _, r := range records {
		paths[string(r)] = struct{}{}
	}
	return paths, nil
}

// blockingPath returns an ignored path that is one of the tracked paths, a
// parent directory of one, or a path inside one (a tracked file where an
// ignored directory is).
func blockingPath(ignored []string, tracked map[string]struct{}) (string, bool) {
	for _, ig := range ignored {
		name := strings.TrimSuffix(ig, "/")
		if _, ok := tracked[name]; ok {
			return ig, true
		}
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			if _, ok := tracked[dir]; ok {
				return ig, true
			}
		}
	}
	parents := make(map[string]struct{})
	for t := range tracked {
		for dir := path.Dir(t); dir != "."; dir = path.Dir(dir) {
			parents[dir] = struct{}{}
		}
	}
	for _, ig := range ignored {
		if _, ok := parents[strings.TrimSuffix(ig, "/")]; ok {
			return ig, true
		}
	}
	return "", false
}

// setProtocolNumbers writes the resolved protocol-only conflicts with the
// expected number and sets the number of every other tool in expected in the
// merged index, staging each file it writes.
//
// Contract: a file is written only when its index entry is a regular file
// and the work tree holds a regular file there; the number written is the one
// expected gives; a tool whose merged files do not hold exactly one accepted
// declaration line is an error.
func (m *Mergemain) setProtocolNumbers(ctx context.Context, expected map[string]int, resolved map[string]resolvedProtocol) ([]ProtocolBump, error) {
	top, err := m.top(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range slices.Sorted(maps.Keys(resolved)) {
		r := resolved[p]
		if err := m.writeStaged(ctx, top, p, setPlaceholder(r.merged, expected[r.tool])); err != nil {
			return nil, err
		}
	}
	bumps := make([]ProtocolBump, 0, len(expected))
	for _, pkg := range slices.Sorted(maps.Keys(expected)) {
		if err := m.setIndexProtocol(ctx, top, pkg, expected[pkg]); err != nil {
			return nil, err
		}
		bumps = append(bumps, ProtocolBump{Tool: pkg, Value: expected[pkg]})
	}
	return bumps, nil
}

// setPlaceholder replaces the line that is exactly protocolPlaceholder (the
// one stripProtocol wrote) with the declaration of n, leaving any other text
// that merely contains the placeholder alone.
func setPlaceholder(content []byte, n int) []byte {
	lines := strings.SplitAfter(string(content), "\n")
	for i, line := range lines {
		if strings.TrimSuffix(line, "\n") == protocolPlaceholder {
			lines[i] = declaration(n) + line[len(protocolPlaceholder):]
			break
		}
	}
	return []byte(strings.Join(lines, ""))
}

// declaration is the accepted declaration line of n.
func declaration(n int) string { return "const " + protocolWord + " = " + strconv.Itoa(n) }

// top returns the top of the work tree.
func (m *Mergemain) top(ctx context.Context) (string, error) {
	out, err := m.git.git(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top := strings.TrimSuffix(string(out), "\n")
	if top == "" || strings.Contains(top, "\n") || !filepath.IsAbs(top) {
		return "", fmt.Errorf("%w: work tree top %q", errUnexpectedGitOutput, out)
	}
	return top, nil
}

// setIndexProtocol sets the number of the tool in the merged index: it reads
// the tool's files at stage 0, requires exactly one accepted declaration
// line, and rewrites that file when its number differs.
func (m *Mergemain) setIndexProtocol(ctx context.Context, top, pkg string, value int) error {
	out, err := m.git.git(ctx, "ls-files", "-s", "-z", "--", ":(top,glob)"+toolDir+"/"+pkg+"/*.go", ":(top,glob,exclude)"+toolDir+"/"+pkg+"/*_test.go")
	if err != nil {
		return err
	}
	records, err := splitNUL(out)
	if err != nil {
		return err
	}
	entries, err := parseStageEntries(records)
	if err != nil {
		return err
	}
	found := ""
	var foundContent []byte
	for _, p := range slices.Sorted(maps.Keys(entries)) {
		es := entries[p]
		if len(es) != 1 || es[0].stage != 0 {
			return fmt.Errorf("%w: tool %s: %s is still conflicted", errTakeInFile, pkg, p)
		}
		content, err := m.blob(ctx, es[0].oid)
		if err != nil {
			return err
		}
		_, count, ok := stripProtocol(content)
		if !ok || (count == 1 && found != "") {
			return fmt.Errorf("%w: tool %s: the declaration of %s is not one accepted line", errTakeInFile, pkg, p)
		}
		if count == 1 {
			found, foundContent = p, content
		}
	}
	if found == "" {
		return fmt.Errorf("%w: tool %s has no accepted declaration line after the merge", errTakeInFile, pkg)
	}
	stripped, _, _ := stripProtocol(foundContent)
	updated := setPlaceholder(stripped, value)
	if bytes.Equal(updated, foundContent) {
		return nil
	}
	return m.writeStaged(ctx, top, found, updated)
}

// writeStaged writes content to the work tree file p and stages it. It
// refuses a path whose index entry is not a regular file or whose work tree
// entry is not a regular file, so a symbolic link is never followed.
func (m *Mergemain) writeStaged(ctx context.Context, top, p string, content []byte) error {
	full := filepath.Join(top, filepath.FromSlash(p))
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a regular file", errTakeInFile, p)
	}
	if err := os.WriteFile(full, content, info.Mode().Perm()); err != nil {
		return err
	}
	_, err = m.git.git(ctx, "add", "--", ":(top,literal)"+p)
	return err
}

// collectFacts fills in the README, ADR, and ext-check facts of a merge.
func (m *Mergemain) collectFacts(ctx context.Context, result *MergemainMergeResult, base, head, main string, conflicts []PathConflict) error {
	out, err := m.git.git(ctx, "diff-index", "--cached", "-z", "--name-only", "--no-renames", head)
	if err != nil {
		return err
	}
	records, err := splitNUL(out)
	if err != nil {
		return err
	}
	changed := make(map[string]struct{})
	for _, r := range records {
		changed[string(r)] = struct{}{}
	}
	for _, c := range conflicts {
		changed[c.Path] = struct{}{}
	}
	for p := range changed {
		if slices.Contains(readmePaths, p) {
			result.ReadmeChanged = true
		}
		if slices.Contains(extCheckPaths, p) || strings.HasPrefix(p, extensionDir+"/") {
			result.ExtCheckHint = true
		}
	}
	out, err = m.git.git(ctx, "diff-tree", "-r", "-z", "--name-only", "--no-renames", base, main, "--", ":(top,literal)"+adrDir)
	if err != nil {
		return err
	}
	result.ADRMainChanged = len(out) > 0
	numbers, err := m.adrNumberOverlap(ctx, base, head, main)
	if err != nil {
		return err
	}
	result.ADRConflictNumbers = numbers
	return nil
}

// adrNumberOverlap returns the numbers of the ADR files the branch added
// after the merge base that one of main's ADR files also has, sorted.
func (m *Mergemain) adrNumberOverlap(ctx context.Context, base, head, main string) ([]string, error) {
	out, err := m.git.git(ctx, "diff-tree", "-r", "-z", "--name-status", "--no-renames", "--diff-filter=A", base, head, "--", ":(top,literal)"+adrDir)
	if err != nil {
		return nil, err
	}
	records, err := splitNUL(out)
	if err != nil {
		return nil, err
	}
	if len(records)%2 != 0 {
		return nil, fmt.Errorf("%w: diff-tree name-status", errUnexpectedGitOutput)
	}
	added := make(map[string]struct{})
	for i := 0; i < len(records); i += 2 {
		if n, ok := adrNumber(string(records[i+1])); ok {
			added[n] = struct{}{}
		}
	}
	if len(added) == 0 {
		return nil, nil
	}
	out, err = m.git.git(ctx, "ls-tree", "-z", "--name-only", "--full-tree", main, "--", adrDir+"/")
	if err != nil {
		return nil, err
	}
	records, err = splitNUL(out)
	if err != nil {
		return nil, err
	}
	var overlap []string
	for _, r := range records {
		if n, ok := adrNumber(string(r)); ok {
			if _, dup := added[n]; dup && !slices.Contains(overlap, n) {
				overlap = append(overlap, n)
			}
		}
	}
	slices.Sort(overlap)
	return overlap, nil
}

// adrNumber returns the number of a file directly inside docs/dev/adr/ whose
// name is the ADR form.
func adrNumber(p string) (string, bool) {
	dir, name := path.Split(p)
	if dir != adrDir+"/" {
		return "", false
	}
	n, ok := tasknaming.ParseADRFile(name)
	if !ok {
		return "", false
	}
	return n.String(), true
}
