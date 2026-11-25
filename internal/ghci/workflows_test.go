package ghci

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The evaluation in this package treats "every GitHub Actions check suite on
// the head commit has passed" as "CI passed". That holds only while ci.yml is
// the only workflow that attaches a check suite to a pull request's head
// commit automatically: with a second one, ReadPR can report passed before the
// later suite appears, and a failure of a workflow that is not CI (such as a
// review bot run in Actions) blocks the merge. The guard below keeps the
// repository's workflows in that shape: ci.yml runs on pull_request alone,
// another workflow may run on schedule (which fires only on the default
// branch), and workflow_dispatch (which runs on the ref a person selects, so a
// pull request's branch included) is allowed only in manualWorkflowFiles, the
// review workflow kept on a person's decision. Any other trigger, and
// workflow_dispatch in another file, fails the test, so adding one means
// revisiting the evaluation (for example by declaring the required workflows).
const (
	workflowsDir   = "../../.github/workflows"
	ciWorkflowFile = "ci.yml"
)

var (
	ciWorkflowTriggers = []string{"pull_request"}
	// Triggers that never attach a check suite to a pull request's head
	// commit: schedule fires only on the default branch.
	scheduledWorkflowTriggers = []string{"schedule"}
	// workflow_dispatch runs on the ref a person selects, so it can target a
	// pull request's branch. Only these files may use it.
	manualWorkflowTrigger = "workflow_dispatch"
	manualWorkflowFiles   = []string{"deepseek-review.yml"}
)

var errWorkflowTriggers = errors.New("unreadable workflow triggers")

func TestWorkflowTriggersKeepSingleHeadWorkflow(t *testing.T) {
	if err := checkWorkflowTriggers(workflowsDir); err != nil {
		t.Fatal(err)
	}
}

func TestCheckWorkflowTriggers(t *testing.T) {
	const ci = "on:\n  pull_request:\n    branches: [ main ]\n\njobs:\n  test:\n    runs-on: ubuntu-latest\n"
	cases := []struct {
		name    string
		files   map[string]string
		wantErr bool
	}{
		{name: "ci_only", files: map[string]string{"ci.yml": ci}},
		{
			name: "declared_manual_workflow_with_scheduled_and_commented_triggers",
			files: map[string]string{
				"ci.yml": ci,
				"deepseek-review.yml": "name: bot\non:\n  # pull_request:\n  #   types: [opened]\n  workflow_dispatch:\n" +
					"    inputs:\n      push:\n        type: string\n  schedule:\n    - cron: '0 0 * * *'\njobs: {}\n",
			},
		},
		{
			name: "undeclared_manual_workflow",
			files: map[string]string{
				"ci.yml":  ci,
				"bot.yml": "name: bot\non:\n  workflow_dispatch:\n    inputs:\n      push:\n        type: string\njobs: {}\n",
			},
			wantErr: true,
		},
		{name: "declared_manual_workflow_with_pull_request", files: map[string]string{"ci.yml": ci, "deepseek-review.yml": "on:\n  workflow_dispatch:\n  pull_request:\n"}, wantErr: true},
		{name: "other_workflow_on_pull_request", files: map[string]string{"ci.yml": ci, "bot.yml": "on:\n  pull_request:\njobs: {}\n"}, wantErr: true},
		{name: "other_workflow_on_push", files: map[string]string{"ci.yml": ci, "bot.yaml": "on:\n  push:\n    branches: ['**']\n"}, wantErr: true},
		{name: "other_workflow_on_workflow_run", files: map[string]string{"ci.yml": ci, "bot.yml": "on:\n  workflow_run:\n    workflows: [CI]\n"}, wantErr: true},
		{name: "other_workflow_inline_list", files: map[string]string{"ci.yml": ci, "bot.yml": "on: [workflow_dispatch, pull_request]\n"}, wantErr: true},
		{name: "other_workflow_inline_scalar", files: map[string]string{"ci.yml": ci, "bot.yml": "on: pull_request_target\n"}, wantErr: true},
		{name: "other_workflow_schedule_only", files: map[string]string{"ci.yml": ci, "bot.yml": "on:\n  schedule:\n    - cron: '0 0 * * *'\n"}},
		{name: "undeclared_workflow_inline_manual", files: map[string]string{"ci.yml": ci, "bot.yml": "on: [workflow_dispatch, schedule] # manual\n"}, wantErr: true},
		{name: "declared_workflow_inline_manual_and_scheduled", files: map[string]string{"ci.yml": ci, "deepseek-review.yml": "on: [workflow_dispatch, schedule] # manual\n"}},
		{name: "ci_with_extra_trigger", files: map[string]string{"ci.yml": "on:\n  pull_request:\n  push:\n"}, wantErr: true},
		{name: "ci_missing", files: map[string]string{"bot.yml": "on:\n  workflow_dispatch:\n"}, wantErr: true},
		{name: "quoted_on_key_is_unreadable", files: map[string]string{"ci.yml": ci, "bot.yml": "\"on\":\n  pull_request:\n"}, wantErr: true},
		{name: "two_on_keys_are_unreadable", files: map[string]string{"ci.yml": ci + "on:\n  push:\n"}, wantErr: true},
		{name: "inconsistent_indent_is_unreadable", files: map[string]string{"ci.yml": "on:\n    pull_request:\n  push:\n"}, wantErr: true},
		{name: "empty_on_block_is_unreadable", files: map[string]string{"ci.yml": ci, "bot.yml": "on:\njobs: {}\n"}, wantErr: true},
		{name: "flow_mapping_is_unreadable", files: map[string]string{"ci.yml": ci, "bot.yml": "on: {workflow_dispatch: {}}\n"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := checkWorkflowTriggers(dir)
			if tc.wantErr && err == nil {
				t.Fatal("checkWorkflowTriggers returned nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("checkWorkflowTriggers unexpected error: %v", err)
			}
		})
	}
}

// checkWorkflowTriggers reads the top-level "on" key of every workflow in dir
// and checks the triggers against ciWorkflowTriggers, scheduledWorkflowTriggers,
// and manualWorkflowFiles.
func checkWorkflowTriggers(dir string) error {
	var paths []string
	for _, pattern := range []string{"*.yml", "*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return err
		}
		paths = append(paths, matches...)
	}
	sawCI := false
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		triggers, err := workflowTriggers(string(data))
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if filepath.Base(path) == ciWorkflowFile {
			sawCI = true
			if !slices.Equal(triggers, ciWorkflowTriggers) {
				return fmt.Errorf("%s: triggers %v, want exactly %v", ciWorkflowFile, triggers, ciWorkflowTriggers)
			}
			continue
		}
		for _, trigger := range triggers {
			if slices.Contains(scheduledWorkflowTriggers, trigger) {
				continue
			}
			if trigger == manualWorkflowTrigger && slices.Contains(manualWorkflowFiles, filepath.Base(path)) {
				continue
			}
			return fmt.Errorf("%s: trigger %q can attach a check suite to a pull request head; "+
				"internal/ghci assumes only %s does automatically (allowed here: %v, and %q only in %v)",
				filepath.Base(path), trigger, ciWorkflowFile, scheduledWorkflowTriggers, manualWorkflowTrigger, manualWorkflowFiles)
		}
	}
	if !sawCI {
		return fmt.Errorf("%s not found in %s", ciWorkflowFile, dir)
	}
	return nil
}

// workflowTriggers returns the trigger names of a workflow's top-level "on"
// key, sorted. It accepts the three forms the repository uses or may use: a
// block mapping, an inline list, and an inline scalar. Any other shape (a
// quoted key, a flow mapping, two "on" keys, an uneven indent, an empty block)
// is rejected rather than read more widely.
func workflowTriggers(text string) ([]string, error) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if line == "on:" || strings.HasPrefix(line, "on: ") || strings.HasPrefix(line, "on:#") {
			if start >= 0 {
				return nil, fmt.Errorf("%w: two top-level on keys", errWorkflowTriggers)
			}
			start = i
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("%w: no top-level on key", errWorkflowTriggers)
	}
	if inline := stripComment(strings.TrimPrefix(lines[start], "on:")); inline != "" {
		return inlineTriggers(inline)
	}
	var triggers []string
	indent := ""
	for _, line := range lines[start+1:] {
		body := strings.TrimLeft(line, " ")
		if body == "" || strings.HasPrefix(body, "#") {
			continue
		}
		lead := line[:len(line)-len(body)]
		if lead == "" {
			break
		}
		if indent == "" {
			indent = lead
		}
		if len(lead) > len(indent) {
			continue
		}
		if lead != indent {
			return nil, fmt.Errorf("%w: uneven indent", errWorkflowTriggers)
		}
		name, _, ok := strings.Cut(body, ":")
		if !ok || !validTriggerName(name) {
			return nil, fmt.Errorf("%w: malformed trigger line", errWorkflowTriggers)
		}
		triggers = append(triggers, name)
	}
	if len(triggers) == 0 {
		return nil, fmt.Errorf("%w: empty on block", errWorkflowTriggers)
	}
	slices.Sort(triggers)
	return triggers, nil
}

func inlineTriggers(value string) ([]string, error) {
	if inner, ok := strings.CutPrefix(value, "["); ok {
		inner, ok = strings.CutSuffix(inner, "]")
		if !ok {
			return nil, fmt.Errorf("%w: malformed inline list", errWorkflowTriggers)
		}
		var triggers []string
		for item := range strings.SplitSeq(inner, ",") {
			name := strings.TrimSpace(item)
			if !validTriggerName(name) {
				return nil, fmt.Errorf("%w: malformed inline list", errWorkflowTriggers)
			}
			triggers = append(triggers, name)
		}
		slices.Sort(triggers)
		return triggers, nil
	}
	if !validTriggerName(value) {
		return nil, fmt.Errorf("%w: malformed inline value", errWorkflowTriggers)
	}
	return []string{value}, nil
}

func stripComment(s string) string {
	if before, _, ok := strings.Cut(s, "#"); ok {
		s = before
	}
	return strings.TrimSpace(s)
}

func validTriggerName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && c != '_' {
			return false
		}
	}
	return true
}
