//go:build test

package chief

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testOldValues is an old-value table for the tests that read plans, so they
// do not depend on the old values _context.md still lists.
var testOldValues = OldValues{
	"frontier-required": RequirementOpus,
	"standard":          RequirementSonnet,
}

// testContext is a _context.md holding only an old-value table, the part of
// it that TestTasksRead's plan reading uses.
const testContext = "### PR marker conventions\n\n| `standard` | `sonnet` |\n"

// repoContext returns the repository's current _context.md.
func repoContext(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", ".claude", "commands", "_context.md"))
	if err != nil {
		t.Fatalf("read _context.md: %v", err)
	}
	return data
}

// TestParseOldValues reads an old-value table as the script does: rows of
// the section only, an old value with two rows or a blank dropped.
func TestParseOldValues(t *testing.T) {
	doubled := []byte("### PR marker conventions\n\n| `a` | `opus` |\n| `a` | `sonnet` |\n| `b` | `sonnet` |\n| `d e` | `opus` |\n\n### Next\n\n| `c` | `opus` |\n")
	if got := ParseOldValues(doubled); len(got) != 1 || got["b"] != RequirementSonnet {
		t.Errorf("ParseOldValues(doubled) = %v, want only b (a twice, d e with a blank, c outside)", got)
	}
}

// TestReadModelRequirement reads the requirement of each plan sample's next
// PR (PR-2, or the plan itself without markers), so opus and
// sonnet+opus-review are told apart here, which the parity test cannot do.
func TestReadModelRequirement(t *testing.T) {
	old := testOldValues
	cases := map[string]struct {
		target string
		want   RequirementReading
	}{
		"marker-opus.md":                  {"2", RequirementReading{RequirementOpus, "opus"}},
		"marker-sonnet-review.md":         {"2", RequirementReading{RequirementSonnetReview, "sonnet+opus-review"}},
		"marker-sonnet.md":                {"2", RequirementReading{RequirementSonnet, "sonnet"}},
		"marker-old-frontier-required.md": {"2", RequirementReading{RequirementOpus, "frontier-required"}},
		"marker-old-standard.md":          {"2", RequirementReading{RequirementSonnet, "standard"}},
		"marker-unreadable.md":            {"2", RequirementReading{RequirementUnreadable, "Opus"}},
		"marker-absent.md":                {"2", RequirementReading{Value: RequirementAbsent}},
		"marker-two-values.md":            {"2", RequirementReading{Value: RequirementUnreadable}},
		"marker-fenced-value.md":          {"2", RequirementReading{Value: RequirementUnreadable}},
		"marker-tilde-fence.md":           {"2", RequirementReading{Value: RequirementUnreadable}},
		"marker-sonnet.md crlf":           {"2", RequirementReading{RequirementSonnet, "sonnet"}},
		"marker-duplicate.md":             {"2", RequirementReading{Value: RequirementUnreadable}},
		"single-sonnet.md":                {singleTarget, RequirementReading{RequirementSonnet, "sonnet"}},
		"single-no-section.md":            {singleTarget, RequirementReading{Value: RequirementAbsent}},
		"single-outside-chapter3.md":      {singleTarget, RequirementReading{Value: RequirementAbsent}},
		"marker-opus.md as single":        {singleTarget, RequirementReading{Value: RequirementUnreadable}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			file := strings.TrimSuffix(name, " as single")
			if got := readRequirement(planSample(t, file), c.target, old); got != c.want {
				t.Errorf("readRequirement = %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestReadAutoSendField checks the auto-send field: absent reads as auto, and
// two fields or a value other than the two are unreadable.
func TestReadAutoSendField(t *testing.T) {
	cases := map[string]struct {
		target string
		want   AutoSend
	}{
		"marker-opus.md":          {"2", AutoSendAuto},
		"marker-sonnet-review.md": {"2", AutoSendAuto},
		"marker-hold.md":          {"2", AutoSendHold},
		"marker-unreadable.md":    {"2", AutoSendUnreadable},
		"marker-two-values.md":    {"2", AutoSendUnreadable},
		"marker-fenced-value.md":  {"2", AutoSendUnreadable},
		"single-sonnet.md":        {singleTarget, AutoSendHold},
		"single-no-section.md":    {singleTarget, AutoSendAuto},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := readAutoSend(planSample(t, name), c.target); got != c.want {
				t.Errorf("readAutoSend = %d, want %d", got, c.want)
			}
		})
	}
}

// TestParsePlanProgress checks the next plan PR and task completion.
func TestParsePlanProgress(t *testing.T) {
	old := testOldValues
	cases := map[string]struct {
		next string // "" when there is none
		done bool
	}{
		"marker-opus.md":         {"2", false},
		"marker-all-merged.md":   {"", true},
		"single-sonnet.md":       {singleTarget, false},
		"single-no-section.md":   {singleTarget, false},
		"single-recorded.md":     {"", true},
		"single-real-record.md":  {"", true},
		"marker-real-forms.md":   {"3", false},
		"marker-old-standard.md": {"2", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			plan, err := ParsePlan(planSample(t, name), old)
			if err != nil {
				t.Fatal(err)
			}
			next, ok := plan.NextPR()
			got := ""
			if ok {
				got = next.Target
			}
			if got != c.next {
				t.Errorf("NextPR = %q, want %q", got, c.next)
			}
			if plan.Done() != c.done {
				t.Errorf("Done = %v, want %v", plan.Done(), c.done)
			}
		})
	}
	t.Run("marker values", func(t *testing.T) {
		plan, err := ParsePlan(planSample(t, "marker-sonnet-review.md"), old)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.PRs) != 3 {
			t.Fatalf("read %d PRs, want 3", len(plan.PRs))
		}
		third := plan.PRs[2]
		if third.Requirement != (RequirementReading{RequirementSonnet, "standard"}) || third.AutoSend != AutoSendHold || third.Created {
			t.Errorf("PR-3 = %+v", third)
		}
	})
	t.Run("draft plan is not done", func(t *testing.T) {
		doc := strings.Replace(string(planSample(t, "marker-all-merged.md")), "`approved`", "`draft`", 1)
		plan, err := ParsePlan([]byte(doc), old)
		if err != nil || plan.Done() {
			t.Errorf("Done = %v, %v; want false", plan.Done(), err)
		}
	})
}

// TestParsePlanRejects checks that a plan whose PRs cannot be told apart is
// rejected rather than read in part.
func TestParsePlanRejects(t *testing.T) {
	base := string(planSample(t, "marker-opus.md"))
	cases := map[string]string{
		"duplicate marker":     string(planSample(t, "marker-duplicate.md")),
		"marker number 0":      strings.Replace(base, "### PR-3 ", "### PR-03 ", 1),
		"merged box missing":   string(planSample(t, "marker-no-merged-box.md")),
		"near-miss checkbox":   string(planSample(t, "single-near-miss.md")),
		"quoted checkbox":      string(planSample(t, "single-near-miss-quoted.md")),
		"numbered checkbox":    string(planSample(t, "single-near-miss-numbered.md")),
		"full-width blank":     string(planSample(t, "single-near-miss-wide-blank.md")),
		"unclosed fence":       base + "\n```\n",
		"Status without a row": strings.Replace(base, "| Status | `approved` |\n", "", 1),
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if doc == base {
				t.Fatal("the edit did not apply")
			}
			if _, err := ParsePlan([]byte(doc), nil); !errors.Is(err, errTaskDocument) {
				t.Errorf("error = %v, want errTaskDocument", err)
			}
		})
	}
}

// TestBranchPlanStepsDone checks the takeover condition that every target step
// of the PR is done or skipped.
func TestBranchPlanStepsDone(t *testing.T) {
	cases := map[string]struct {
		file, target string
		want         bool
	}{
		"done and skipped":       {"marker-opus.md", "2", true},
		"one not started":        {"marker-opus.md", "3", false},
		"single, one not done":   {"single-sonnet.md", singleTarget, false},
		"single, every one done": {"single-recorded.md", singleTarget, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			plan, err := ParsePlan(planSample(t, c.file), nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := plan.StepsDone(c.target)
			if err != nil || got != c.want {
				t.Errorf("StepsDone(%s) = %v, %v; want %v", c.target, got, err, c.want)
			}
		})
	}
	t.Run("an unreadable target step list", func(t *testing.T) {
		doc := strings.Replace(string(planSample(t, "marker-opus.md")), "2-1 / 2-2", "2-1, 2-2", 1)
		plan, err := ParsePlan([]byte(doc), nil)
		if err != nil {
			t.Fatalf("the plan was rejected for its step list: %v", err)
		}
		if _, err := plan.StepsDone("2"); !errors.Is(err, errTaskDocument) {
			t.Errorf("error = %v, want errTaskDocument", err)
		}
	})
	t.Run("a target step without a checkbox", func(t *testing.T) {
		doc := strings.Replace(string(planSample(t, "marker-opus.md")), "2-1 / 2-2", "2-1 / 2-9", 1)
		plan, err := ParsePlan([]byte(doc), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.StepsDone("2"); !errors.Is(err, errTaskDocument) {
			t.Errorf("error = %v, want errTaskDocument", err)
		}
	})
}

// TestReadDocInfo reads the status table by the Cell text rule.
func TestReadDocInfo(t *testing.T) {
	table := func(rows string) []byte {
		return []byte("# T\n\n## Document Status\n\n| Item | Value |\n|---|---|\n" + rows + "\nText after.\n")
	}
	ok := map[string]struct {
		doc  []byte
		want DocInfo
	}{
		"approved full":    {table("| Status | `approved` |\n| Process | `full` |\n"), DocInfo{DocApproved, ProcessFull}},
		"draft simple":     {table("| Status | `draft` |\n| Process | `simple` |\n"), DocInfo{DocDraft, ProcessSimple}},
		"no Process row":   {table("| Status | `withdrawn` |\n"), DocInfo{DocWithdrawn, ProcessUnrecorded}},
		"fenced table":     {[]byte("## Document Status\n\n```\n| Status | `draft` |\n```\n\n| Status | `approved` |\n"), DocInfo{DocApproved, ProcessUnrecorded}},
		"later table only": {[]byte("## Document Status\n\n| Status | `approved` |\n\n| Status | `draft` |\n"), DocInfo{DocApproved, ProcessUnrecorded}},
	}
	for name, c := range ok {
		t.Run(name, func(t *testing.T) {
			got, err := readDocInfo(c.doc, true)
			if err != nil || got != c.want {
				t.Errorf("readDocInfo = %+v, %v; want %+v", got, err, c.want)
			}
		})
	}
	bad := map[string][]byte{
		"two Process rows":   table("| Status | `approved` |\n| Process | `full` |\n| Process | `simple` |\n"),
		"other Process":      table("| Status | `approved` |\n| Process | `Simple` |\n"),
		"other Status":       table("| Status | approved |\n"),
		"trailing blank":     table("| Status | `approved`  |\n"),
		"two Status rows":    table("| Status | `approved` |\n| Status | `draft` |\n"),
		"no heading":         []byte("| Status | `approved` |\n"),
		"no table":           []byte("## Document Status\n\nNone.\n"),
		"Process unreadable": table("| Status | `approved` |\n| Process | full |\n"),
	}
	for name, doc := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := readDocInfo(doc, true); !errors.Is(err, errTaskDocument) {
				t.Errorf("error = %v, want errTaskDocument", err)
			}
		})
	}
}

// TestBranchTask reads the task of a feature branch name.
func TestBranchTask(t *testing.T) {
	task, slug, ok := branchTask("isseis/0088-chief-of-staff-06")
	if !ok || task != taskID(t, "0088") || slug != "0088-chief-of-staff" {
		t.Errorf("branchTask = %v %q %v", task, slug, ok)
	}
	for _, branch := range []string{"main", "0088-chief-of-staff-06", "isseis/0088-chief-of-staff", "isseis/0088-chief-of-staff-6", "isseis/088-chief-06", "isseis/0088-Chief-06", "a/b/0088-x-01", "-x/0088-x-01"} {
		if _, _, ok := branchTask(branch); ok {
			t.Errorf("branchTask(%q) read a task", branch)
		}
	}
}

// TestTasksRead reads a task's documents from origin/main through git, with
// the recorder standing in for git.
func TestTasksRead(t *testing.T) {
	const wt = "/work/tree"
	lsTasks := "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/"
	lsDir := "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/0001_test_task/"
	req := []byte("## Document Status\n\n| Item | Value |\n|---|---|\n| Status | `approved` |\n| Process | `full` |\n")
	t.Run("every document", func(t *testing.T) {
		rec := newRecorder(t)
		rec.reply(lsTasks, []byte("docs/tasks/0000_template\x00docs/tasks/0001_test_task\x00docs/tasks/0010_other\x00"))
		rec.reply(lsDir, []byte("docs/tasks/0001_test_task/01_requirements.md\x00docs/tasks/0001_test_task/03_implementation_plan.md\x00"))
		rec.reply("git show refs/remotes/origin/main:docs/tasks/0001_test_task/01_requirements.md", req)
		rec.reply("git show refs/remotes/origin/main:docs/tasks/0001_test_task/03_implementation_plan.md", planSample(t, "marker-old-standard.md"))
		rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", []byte(testContext))
		docs, err := NewTasks(rec.commands(nil)).Read(context.Background(), wt, "user/0001-test-task-02")
		if err != nil {
			t.Fatal(err)
		}
		if docs.Dir != "docs/tasks/0001_test_task" || docs.Requirements == nil || *docs.Requirements != (DocInfo{DocApproved, ProcessFull}) || docs.Architecture != nil || docs.Plan == nil {
			t.Fatalf("docs = %+v", docs)
		}
		next, _ := docs.Plan.NextPR()
		if next.Requirement.Value != RequirementSonnet {
			t.Errorf("the old value was not read through origin/main's table: %+v", next.Requirement)
		}
		for _, c := range rec.recorded() {
			if c.dir != wt {
				t.Errorf("%s ran in %q", c.line(), c.dir)
			}
		}
	})
	t.Run("no directory yet", func(t *testing.T) {
		rec := newRecorder(t)
		rec.reply(lsTasks, []byte("docs/tasks/0010_other\x00"))
		docs, err := NewTasks(rec.commands(nil)).Read(context.Background(), wt, "user/0001-test-task-02")
		if err != nil || docs.Dir != "" || docs.Task != taskID(t, "0001") {
			t.Errorf("docs = %+v, %v", docs, err)
		}
	})
	for name, listing := range map[string]string{
		"two directories":     "docs/tasks/0001_test_task\x00docs/tasks/0001_test_other\x00",
		"slug does not match": "docs/tasks/0001_test_tusk\x00",
	} {
		t.Run(name, func(t *testing.T) {
			rec := newRecorder(t)
			rec.reply(lsTasks, []byte(listing))
			if _, err := NewTasks(rec.commands(nil)).Read(context.Background(), wt, "user/0001-test-task-02"); !errors.Is(err, errTaskDocument) {
				t.Errorf("error = %v, want errTaskDocument", err)
			}
		})
	}
	t.Run("branch plan from HEAD", func(t *testing.T) {
		rec := newRecorder(t)
		rec.reply("git show HEAD:docs/tasks/0001_test_task/03_implementation_plan.md", planSample(t, "marker-opus.md"))
		tasks := NewTasks(rec.commands(nil))
		plan, err := tasks.BranchPlan(context.Background(), wt, TaskDocs{Dir: "docs/tasks/0001_test_task"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if done, err := plan.StepsDone("2"); err != nil || !done {
			t.Errorf("StepsDone = %v, %v", done, err)
		}
		if _, err := tasks.BranchPlan(context.Background(), wt, TaskDocs{}, nil); !errors.Is(err, errTaskDocument) {
			t.Errorf("BranchPlan without a directory: %v", err)
		}
	})
	t.Run("not a feature branch", func(t *testing.T) {
		rec := newRecorder(t)
		if _, err := NewTasks(rec.commands(nil)).Read(context.Background(), wt, "main"); !errors.Is(err, errNotTaskBranch) {
			t.Errorf("error = %v, want errNotTaskBranch", err)
		}
		if calls := rec.recorded(); len(calls) != 0 {
			t.Errorf("ran %v", calls)
		}
	})
}
