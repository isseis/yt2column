// Command _checks verifies a before/after relation of task 0051: an LLM
// variable value without "+" loads to the same observable result before and
// after the change. It compares the merge-base of origin/main and HEAD
// with HEAD.
//
// The values are the rows of the merge-base's TestLoadParsesLLMValue (none of
// them contains "+"), the empty value, and unset, each given to
// YT2COLUMN_LLM and to YT2COLUMN_REVIEW_LLM. For each commit the script checks
// the commit out into a temporary worktree, adds a probe test to
// internal/config that loads every value with LoadFor (both stages needed)
// and prints the result, runs it, and removes the worktree. A result is
// "accepted" with each stage's provider, model ID, Claude effort, and
// DeepSeek effort (the merge-base has one Claude effort, which a claude stage
// uses, and no DeepSeek effort, printed as the zero value), or the list of
// rejections, each with its variable, sentinel, and reason. The reason of
// reasonLLMForm is printed as a placeholder, because the task changes its text:
// it is compared by being each commit's own reasonLLMForm, not by wording.
//
// Run it once on the PR branch, after every change of the PR is committed:
//
//	go run ./docs/tasks/0051_llm_effort/_checks/
//
// It prints both commit IDs, one line per value whose results differ, and a
// summary, and exits 1 when any result differs. make test and CI do not run
// it. The directory starts with an underscore, which keeps it out of go list.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	testFile  = "internal/config/config_test.go"
	probeFile = "internal/config/zz_compat_probe_test.go"
	// unsetValue stands for an unset variable in the value list. It is not a
	// value either commit could be given, since it contains a NUL byte.
	unsetValue = "\x00unset"
)

// caseRowRE matches the value of one row of TestLoadParsesLLMValue's table at
// the merge-base: a line starting with a Go string literal in braces.
var caseRowRE = regexp.MustCompile(`^\t\t\{("(?:[^"\\]|\\.)*"),`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "no_effort_compat:", err)
		os.Exit(2)
	}
}

func run() error {
	base, err := git("merge-base", "origin/main", "HEAD")
	if err != nil {
		return err
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	fmt.Printf("merge-base %s\nhead       %s\n", base, head)
	values, err := baseValues(base)
	if err != nil {
		return err
	}
	before, err := probe(base, values, baseEfforts)
	if err != nil {
		return fmt.Errorf("merge-base: %w", err)
	}
	after, err := probe(head, values, headEfforts)
	if err != nil {
		return fmt.Errorf("head: %w", err)
	}
	if len(before) != len(after) {
		return fmt.Errorf("the probes printed %d and %d results", len(before), len(after))
	}
	differ := 0
	for i := range before {
		if before[i] != after[i] {
			differ++
			fmt.Printf("DIFFERS\n  before: %s\n  after:  %s\n", before[i], after[i])
		}
	}
	fmt.Printf("%d values x 2 variables: %d results compared, %d differ\n", len(values), len(before), differ)
	if differ > 0 {
		os.Exit(1)
	}
	return nil
}

// baseValues returns the values of the merge-base's TestLoadParsesLLMValue
// rows, followed by the empty value and unset. It fails if a row contains "+",
// which this check does not cover.
func baseValues(base string) ([]string, error) {
	source, err := git("show", base+":"+testFile)
	if err != nil {
		return nil, err
	}
	_, body, found := strings.Cut(source, "func TestLoadParsesLLMValue(")
	if !found {
		return nil, fmt.Errorf("%s at %s has no TestLoadParsesLLMValue", testFile, base)
	}
	body, _, _ = strings.Cut(body, "\n}\n")
	var values []string
	for line := range strings.Lines(body) {
		match := caseRowRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		value, err := strconv.Unquote(match[1])
		if err != nil {
			return nil, fmt.Errorf("row %q: %w", line, err)
		}
		if strings.Contains(value, "+") {
			return nil, fmt.Errorf("row value %q contains \"+\"", value)
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("no rows found in TestLoadParsesLLMValue at %s", base)
	}
	seen := map[string]bool{}
	var unique []string
	for _, v := range append(values, "", unsetValue) {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}
	return unique, nil
}

// The probe's expressions for a stage s's Claude and DeepSeek effort, as int
// values, at each commit. The merge-base stores one Claude effort for the run,
// which the claude stages use.
const (
	baseEfforts = "efforts := func(s llmSelection) (int, int) { if s.provider != ProviderClaude { return 0, 0 }; return int(cfg.llm.claudeEffort), 0 }"
	headEfforts = "efforts := func(s LLMSelection) (int, int) { return int(s.claudeEffort), int(s.deepSeekEffort) }"
)

// probe checks commit out into a temporary worktree, runs the probe test
// there with efforts as the commit's effort expression, and returns its
// result lines.
func probe(commit string, values []string, efforts string) ([]string, error) {
	dir, err := os.MkdirTemp("", "no-effort-compat-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	tree := filepath.Join(dir, "tree")
	if _, err := git("worktree", "add", "--detach", "--quiet", tree, commit); err != nil {
		return nil, err
	}
	defer git("worktree", "remove", "--force", tree) //nolint:errcheck // best-effort cleanup of a temporary worktree
	if err := os.WriteFile(filepath.Join(tree, probeFile), []byte(probeSource(values, efforts)), 0o600); err != nil {
		return nil, err
	}
	cmd := exec.Command("go", "test", "-tags", "test", "-count=1", "-run", "^TestCompatProbe$", "-v", "./internal/config/")
	cmd.Dir = tree
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("probe test failed: %w\n%s", err, out)
	}
	var results []string
	for line := range strings.Lines(string(out)) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "RESULT "); ok {
			results = append(results, rest)
		}
	}
	return results, nil
}

// probeSource is the probe test added to internal/config. Apart from efforts,
// it uses only names both commits define: LoadFor, Need, the stage fields
// generation and review with their provider and model fields, the
// variable-name constants, and reasonLLMForm.
func probeSource(values []string, efforts string) string {
	var list strings.Builder
	for _, v := range values {
		fmt.Fprintf(&list, "\t%q,\n", v)
	}
	return `//go:build test

package config

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCompatProbe(t *testing.T) {
	values := []string{
` + list.String() + `	}
	for _, name := range []string{llmEnv, reviewLLMEnv} {
		for _, value := range values {
			env := map[string]string{
				anthropicAPIKeyEnv: "compat-probe-key",
				deepSeekAPIKeyEnv:  "compat-probe-key",
				cacheDirEnv:        "/tmp/yt2column-compat-probe",
			}
			if value != ` + strconv.Quote(unsetValue) + ` {
				env[name] = value
			}
			lookup := func(n string) (string, bool) { v, ok := env[n]; return v, ok }
			cfg, err := LoadFor(lookup, Need{Generation: true, Review: true})
			result := ""
			if err == nil {
				` + efforts + `
				generation, review := cfg.llm.generation, cfg.llm.review
				genClaude, genDeepSeek := efforts(generation)
				reviewClaude, reviewDeepSeek := efforts(review)
				result = fmt.Sprintf("accepted generation %d %q %d %d review %d %q %d %d",
					generation.provider, generation.model, genClaude, genDeepSeek,
					review.provider, review.model, reviewClaude, reviewDeepSeek)
			} else {
				result = "rejected " + strings.Join(compatRejections(err), " ")
			}
			fmt.Printf("RESULT %s=%q %s\n", name, value, result)
		}
	}
}

func compatRejections(err error) []string {
	var out []string
	if multi, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range multi.Unwrap() {
			out = append(out, compatRejections(child)...)
		}
		return out
	}
	varErr, ok := err.(*VarError)
	if !ok {
		return []string{fmt.Sprintf("[other %q]", err.Error())}
	}
	sentinel := "other"
	switch {
	case errors.Is(varErr, ErrMissing):
		sentinel = "ErrMissing"
	case errors.Is(varErr, ErrInvalid):
		sentinel = "ErrInvalid"
	}
	reason := varErr.Reason
	if reason == reasonLLMForm {
		reason = "<reasonLLMForm>"
	}
	return []string{fmt.Sprintf("[%s %s %q]", varErr.Name, sentinel, reason)}
}
`
}

// git runs git with args and returns its trimmed standard output.
func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
