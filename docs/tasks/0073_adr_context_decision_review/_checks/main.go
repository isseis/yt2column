//go:build test

// Command _checks runs the one-off diff checks for task 0073, which reviews
// the background and decision sections of ADR-0001 to ADR-0038 as editorial
// corrections, one group of ADRs per pull request. It has three subcommands:
//
//	check <group> [<base> <head>]  compare the base and head commits
//	mentions <group> [<commit>]    list what names each ADR of the group
//	freshness <group>              compare the merge-base with origin/main
//
// check reads every document and its data file from the commits it compares,
// never from the working tree. mentions and freshness read the data file from
// HEAD and the documents from the commits they look at.
//
// Run it from the repository root:
//
//	go build -tags test -o /tmp/0073check ./docs/tasks/0073_adr_context_decision_review/_checks/
//	/tmp/0073check check s1
//
// The base commit of check defaults to the merge-base of origin/main and
// HEAD, and the head to HEAD. Every subcommand exits 0 when nothing is wrong,
// 1 on a violation (for freshness: a group ADR or the index changed on
// origin/main), and 2 when it cannot read its input. make test and CI do not
// run it; the directory starts with an underscore, which keeps it out of
// go list.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// taskDir is the task directory; dataPath and the group's review record live
// under it.
const (
	taskDir  = "docs/tasks/0073_adr_context_decision_review/"
	dataPath = taskDir + "_checks/data.json"
	checkDir = taskDir + "_checks/"
)

// The groups s1 to s5 together review ADR-0001 to ADR-0038 for the first
// time, each ADR exactly once; a later group only takes ADRs carried over.
const (
	firstADR      = 1
	lastADR       = 38
	primaryGroups = 5
	maxGroupADRs  = 8
)

// groupNameRE is the form of a group name: s followed by its position.
var groupNameRE = regexp.MustCompile(`^s([1-9][0-9]*)$`)

// adrPathRE is the form of an ADR path the data may name.
var adrPathRE = regexp.MustCompile(`^docs/dev/adr/([0-9]{4})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)

// checkData is the data file the diff check reads from the head commit.
type checkData struct {
	TaskDir  string           `json:"task_dir"`
	Index    string           `json:"index"`
	Headings sectionHeadings  `json:"headings"`
	Groups   map[string]group `json:"groups"` // keyed by the group name given as the argument
}

// sectionHeadings are the ADR section headings the checks locate sections by.
type sectionHeadings struct {
	Status       string `json:"status"`
	Alternatives string `json:"alternatives"`
	Consequences string `json:"consequences"`
	History      string `json:"history"`
}

// group is one implementation pull request: the ADRs it reviews and every
// change the checks allow. Every ADR an entry names must be in ADRs or CarriedIn.
type group struct {
	ADRs              []string       `json:"adrs"`       // repository-relative paths, first review
	CarriedIn         []string       `json:"carried_in"` // carried over from an earlier group
	RetitledADRs      []string       `json:"retitled_adrs"`
	LinkChanges       []change       `json:"link_changes"` // After empty: the link was removed
	LineChanges       []change       `json:"line_changes"` // lines of the alternatives, consequences, and history sections
	UnrecordedCommits []commitReason `json:"unrecorded_commits"`
	CheckerChanges    string         `json:"checker_changes"`  // why the Go files changed, if they did
	TaskDocChanges    []fileReason   `json:"task_doc_changes"` // other task documents of this task the group changes
}

// change allows one link target or one line of an ADR to change.
// A link target is the repository-relative path it resolves to, anchor dropped,
// or the URL as written.
type change struct {
	ADR    string `json:"adr"`
	Before string `json:"before"`
	After  string `json:"after"`
	Reason string `json:"reason"`
}

// fileReason names a task document of this task that a group changes besides
// its data, its review record, and the implementation plan, and why.
type fileReason struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// commitReason names a commit that changes an ADR without a correction line.
type commitReason struct {
	Commit string `json:"commit"`
	Reason string `json:"reason"`
}

// reviewed returns the ADRs the group reviews: its first reviews, then the
// ADRs carried in.
func (g group) reviewed() []string {
	return append(slices.Clone(g.ADRs), g.CarriedIn...)
}

// errUsage reports a malformed command line.
var errUsage = errors.New("usage: 0073check check <group> [<base> <head>] | mentions <group> [<commit>] | freshness <group>")

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "0073check:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

// run dispatches to the subcommand and returns its exit code; an error means
// the input could not be read and maps to exit code 2.
func run(args []string) (int, error) {
	if len(args) < 2 {
		return 0, errUsage
	}
	sub, name, rest := args[0], args[1], args[2:]
	switch sub {
	case "check":
		return runCheck(name, rest)
	case "mentions":
		return runMentions(name, rest)
	case "freshness":
		if len(rest) != 0 {
			return 0, errUsage
		}
		return runFreshness(name)
	default:
		return 0, errUsage
	}
}

// loadData reads and parses the data file at a commit.
func loadData(commit string) (checkData, error) {
	var d checkData
	raw, err := git("show", commit+":"+dataPath)
	if err != nil {
		return d, err
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, fmt.Errorf("%s at %s: %w", dataPath, commit, err)
	}
	if d.TaskDir != taskDir {
		return d, fmt.Errorf("%s at %s: task_dir is %q, want %q", dataPath, commit, d.TaskDir, taskDir)
	}
	for _, h := range []string{d.Index, d.Headings.Status, d.Headings.Alternatives, d.Headings.Consequences, d.Headings.History} {
		if strings.TrimSpace(h) == "" {
			return d, fmt.Errorf("%s at %s: the index path and every heading must be set", dataPath, commit)
		}
	}
	for name := range d.Groups {
		if !groupNameRE.MatchString(name) {
			return d, fmt.Errorf("%s at %s: group name %q is not of the form s<N>", dataPath, commit, name)
		}
	}
	return d, nil
}

// groupFor returns the named group, or a read error when the data has none.
func groupFor(d checkData, name string) (group, error) {
	g, ok := d.Groups[name]
	if !ok {
		return g, fmt.Errorf("the data has no group %q", name)
	}
	return g, nil
}

// groupPosition returns the number in a group name; loadData has checked the
// form.
func groupPosition(name string) int {
	n, _ := strconv.Atoi(groupNameRE.FindStringSubmatch(name)[1])
	return n
}

// adrNumber returns the four-digit number of an ADR path, or "" when the path
// is not an ADR path.
func adrNumber(path string) string {
	m := adrPathRE.FindStringSubmatch(path)
	if m == nil {
		return ""
	}
	return m[1]
}

func resolve(rev string) (string, error) {
	out, err := git("rev-parse", "--verify", "--end-of-options", strings.TrimSpace(rev)+"^{commit}")
	return strings.TrimSpace(out), err
}

// isAncestor reports whether anc is an ancestor of desc.
func isAncestor(anc, desc string) (bool, error) {
	err := exec.Command("git", "merge-base", "--is-ancestor", anc, desc).Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %w", anc, desc, err)
}

// changedFiles returns the sorted paths the range changed, without rename
// detection or the user's diff settings.
func changedFiles(base, head string) ([]string, error) {
	paths, err := gitLines("diff", "--no-renames", "--no-relative", "--no-ext-diff", "--name-only", base, head)
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

// diffPaths returns the paths a commit changed against its parent.
func diffPaths(commit string) ([]string, error) {
	return gitLines("diff-tree", "--no-commit-id", "--name-only", "-r", "--no-renames", "--no-ext-diff", commit)
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		msg := ""
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			msg = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return string(out), nil
}

func gitLines(args ...string) ([]string, error) {
	out, err := git(args...)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSuffix(out, "\n")
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
