//go:build test

package chief

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
)

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

// TestTasksReadRejectsWrongDirectoryForm reads a directory whose name the
// directory form rejects but whose slug matches the branch, which only the
// form check can catch. It is separate from the reader's other rejection
// cases because that test stays as it is.
func TestTasksReadRejectsWrongDirectoryForm(t *testing.T) {
	const wt = "/work/tree"
	rec := newRecorder(t)
	rec.reply("git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/", []byte("docs/tasks/0001_test-task\x00"))
	if _, err := NewTasks(rec.commands(nil)).Read(context.Background(), wt, "user/0001-test-task-02"); !errors.Is(err, errTaskDocument) {
		t.Errorf("error = %v, want errTaskDocument", err)
	}
}

// requirementsSamples maps each sample under testdata/requirements to the
// task directory it is placed in for the issue-line tests.
var requirementsSamples = map[string]string{
	"link-with-remarks.md": "0010_link_with_remarks",
	"plain-number.md":      "0011_plain_number",
	"similar-numbers.md":   "0012_similar_numbers",
	"malformed-link.md":    "0013_malformed_link",
	"broken-link.md":       "0014_broken_link",
	"two-lines.md":         "0015_two_lines",
	"fenced.md":            "0016_fenced",
	"indented.md":          "0017_indented",
	"remarks-only.md":      "0018_remarks_only",
	"same-issue-a.md":      "0019_same_issue_a",
	"same-issue-b.md":      "0020_same_issue_b",
	"prose-mention.md":     "0021_prose_mention",
}

// issueRepo returns a repository whose refs/remotes/origin/main holds every
// requirements sample in its task directory, plus documents the search must
// not read: one a level deeper than a task directory, one a level above, and
// another file of a task directory.
func issueRepo(t *testing.T) *testRepo {
	t.Helper()
	repo := newTestRepo(t)
	for file, dir := range requirementsSamples {
		repo.Write("docs/tasks/"+dir+"/01_requirements.md", string(readTestdata(t, "requirements/"+file)))
	}
	cited := issueLinePrefix + "[#999](https://github.com/example/repo/issues/999)\n"
	repo.Write("docs/tasks/0030_deep/nested/01_requirements.md", cited)
	repo.Write("docs/tasks/01_requirements.md", cited)
	repo.Write("docs/tasks/0031_other_file/02_architecture.md", cited)
	repo.CommitAll("tasks")
	repo.Git("update-ref", originMain, "HEAD")
	return repo
}

// TestIssueIndexIgnoresGrepPatternType checks that a repository setting of
// grep's pattern type does not change the search: with a fixed-string
// setting the anchored pattern would match nothing.
func TestIssueIndexIgnoresGrepPatternType(t *testing.T) {
	repo := issueRepo(t)
	repo.Git("config", "grep.patternType", "fixed")
	ix, err := NewTasks(repo.RealCommands()).IssueIndex(t.Context(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ix.Tasks(issueNumber(t, "839"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"0010_link_with_remarks"}; !slices.Equal(got, want) {
		t.Errorf("Tasks(839) = %v, want %v", got, want)
	}
}

// TestIssueTasksReadsTheRealGitGrep runs the real git grep over a repository
// of the samples and checks which task directories cite each issue.
func TestIssueTasksReadsTheRealGitGrep(t *testing.T) {
	repo := issueRepo(t)
	ix, err := NewTasks(repo.RealCommands()).IssueIndex(t.Context(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		issue string
		want  []string
	}{
		{"839", []string{"0010_link_with_remarks"}},
		{"927", nil}, // inside the remarks after the full-width parenthesis
		{"934", nil},
		{"2", []string{"0011_plain_number"}},
		{"20", []string{"0012_similar_numbers"}},
		{"5", []string{"0013_malformed_link"}},
		{"6", []string{"0013_malformed_link"}}, // the destination of a link whose text disagrees
		{"9", []string{"0014_broken_link"}},
		{"7", []string{"0015_two_lines"}},
		{"8", []string{"0015_two_lines"}},
		{"31", []string{"0016_fenced"}},
		{"40", nil}, // the line does not start in the first column
		{"50", nil},
		{"70", nil}, // mentioned in prose, not on an issue line
		{"60", []string{"0019_same_issue_a", "0020_same_issue_b"}},
		{"61", nil},
		{"999", nil}, // only in files the search must not read
		{"12345", nil},
	}
	for _, c := range cases {
		t.Run("issue "+c.issue, func(t *testing.T) {
			got, err := ix.Tasks(issueNumber(t, c.issue))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("tasks of the issue = %v, want %v", got, c.want)
			}
		})
	}
}

// TestIssueTasksWithNoLine treats git grep's exit status 1 as no line.
func TestIssueTasksWithNoLine(t *testing.T) {
	repo := newTestRepo(t)
	repo.Write("docs/tasks/0001_a/01_requirements.md", "no issue line\n")
	repo.CommitAll("tasks")
	repo.Git("update-ref", originMain, "HEAD")
	got, err := issueOneTasks(t, NewTasks(repo.RealCommands()), repo.Dir)
	if err != nil || len(got) != 0 {
		t.Errorf("tasks of the issue = %v, %v, want none", got, err)
	}
}

// issueOneTasks reads the issue index in wt and returns the tasks of issue 1.
func issueOneTasks(t *testing.T, tasks Tasks, wt string) ([]string, error) {
	t.Helper()
	ix, err := tasks.IssueIndex(t.Context(), wt)
	if err != nil {
		return nil, err
	}
	return ix.Tasks(issueNumber(t, "1"))
}

// TestIssueTasksFailures checks the readings that are not "no line".
func TestIssueTasksFailures(t *testing.T) {
	const wt = "/work/tree"
	search := "git " + strings.Join(issueLineSearch, " ")
	good := "refs/remotes/origin/main:docs/tasks/0001_a/01_requirements.md:5:" + issueLinePrefix + "#1\n"
	t.Run("one git grep in the worktree", func(t *testing.T) {
		rec := newRecorder(t)
		rec.reply(search, []byte(good))
		got, err := issueOneTasks(t, NewTasks(rec.commands(nil)), wt)
		if err != nil || !slices.Equal(got, []string{"0001_a"}) {
			t.Fatalf("tasks of the issue = %v, %v", got, err)
		}
		if calls := rec.recorded(); len(calls) != 1 || calls[0].dir != wt {
			t.Errorf("calls = %+v, want one in %s", calls, wt)
		}
	})
	for name, out := range map[string]string{
		"a binary file":          "Binary file refs/remotes/origin/main:docs/tasks/0001_a/01_requirements.md matches\n",
		"another ref":            "origin/main:docs/tasks/0001_a/01_requirements.md:5:" + issueLinePrefix + "#1\n",
		"another file":           "refs/remotes/origin/main:docs/tasks/0001_a/02_architecture.md:5:" + issueLinePrefix + "#1\n",
		"a deeper path":          "refs/remotes/origin/main:docs/tasks/0001_a/x/01_requirements.md:5:" + issueLinePrefix + "#1\n",
		"a bad directory name":   "refs/remotes/origin/main:docs/tasks/Task/01_requirements.md:5:" + issueLinePrefix + "#1\n",
		"no line number":         "refs/remotes/origin/main:docs/tasks/0001_a/01_requirements.md:" + issueLinePrefix + "#1\n",
		"a good line then noise": good + "noise\n",
	} {
		t.Run(name, func(t *testing.T) {
			rec := newRecorder(t)
			rec.reply(search, []byte(out))
			if got, err := issueOneTasks(t, NewTasks(rec.commands(nil)), wt); !errors.Is(err, errTaskDocument) {
				t.Errorf("tasks of the issue = %v, %v, want errTaskDocument", got, err)
			}
		})
	}
	t.Run("an exit status other than 1 is an error", func(t *testing.T) {
		for _, failure := range []error{&cmdrunner.ExitError{Code: 128}, errors.New("git is gone")} {
			rec := newRecorder(t)
			rec.fail(search, failure)
			if got, err := issueOneTasks(t, NewTasks(rec.commands(nil)), wt); err == nil {
				t.Errorf("tasks of the issue = %v, want an error", got)
			}
		}
	})
	t.Run("no issue", func(t *testing.T) {
		rec := newRecorder(t)
		rec.reply(search, []byte(good))
		ix, err := NewTasks(rec.commands(nil)).IssueIndex(t.Context(), wt)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := ix.Tasks(paneinput.IssueNumber{}); !errors.Is(err, errNoIssueNumber) {
			t.Errorf("Tasks = %v, %v, want errNoIssueNumber", got, err)
		}
	})
}

func TestIssueNumbersOf(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{issueLinePrefix + "[#839](https://example.test/issues/839)", []string{"839"}},
		{issueLinePrefix + "[#839](https://example.test/issues/839)" + issueMentionEnd + "related: #1, #2)", []string{"839"}},
		{issueLinePrefix + "[#5](https://example.test/issues/6)", []string{"5", "6"}},
		{issueLinePrefix + "[#5](https://example.test/pull/5/)", []string{"5"}},
		{issueLinePrefix + "[#5](https://example.test/issues/005)", []string{"5"}},
		{issueLinePrefix + "[#5](https://example.test/issues/x)", []string{"5"}},
		{issueLinePrefix + "see https://example.test/o/r/issues/8#x and /pull/9?y", []string{"8", "9"}},
		{issueLinePrefix + "#2", []string{"2"}},
		{issueLinePrefix + "#2, #3 and [#4](u/4)", []string{"2", "3", "4"}},
		{issueLinePrefix + "[#9(https://example.test/issues/9", []string{"9"}},
		{issueLinePrefix + issueMentionEnd + "#9)", nil},
		{issueLinePrefix + "none", nil},
		{issueLinePrefix + "#0", nil},
	}
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			got := slices.Sorted(maps.Keys(issueNumbersOf(c.line)))
			if !slices.Equal(got, c.want) {
				t.Errorf("issueNumbersOf = %v, want %v", got, c.want)
			}
		})
	}
}

// TestTasksReadDir reads a task by its directory name, with the same
// documents and rules as Read.
func TestTasksReadDir(t *testing.T) {
	const wt = "/work/tree"
	lsTasks := "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/"
	lsDir := "git ls-tree -z --name-only refs/remotes/origin/main -- docs/tasks/0001_test_task/"
	req := []byte("## Document Status\n\n| Item | Value |\n|---|---|\n| Status | `approved` |\n| Process | `full` |\n")
	t.Run("every document", func(t *testing.T) {
		rec := newRecorder(t)
		rec.reply(lsTasks, []byte("docs/tasks/0000_template\x00docs/tasks/0001_test_task\x00"))
		rec.reply(lsDir, []byte("docs/tasks/0001_test_task/01_requirements.md\x00docs/tasks/0001_test_task/03_implementation_plan.md\x00"))
		rec.reply("git show refs/remotes/origin/main:docs/tasks/0001_test_task/01_requirements.md", req)
		rec.reply("git show refs/remotes/origin/main:docs/tasks/0001_test_task/03_implementation_plan.md", planSample(t, "marker-old-standard.md"))
		rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", []byte(testContext))
		tasks := NewTasks(rec.commands(nil))
		byDir, err := tasks.ReadDir(t.Context(), wt, "0001_test_task")
		if err != nil {
			t.Fatal(err)
		}
		byBranch, err := tasks.Read(t.Context(), wt, "user/0001-test-task-02")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(byDir, byBranch) {
			t.Errorf("ReadDir = %+v, Read = %+v", byDir, byBranch)
		}
	})
	// A name that is not a task directory's form is refused before git runs;
	// a well-formed name origin/main lacks is refused after the listing.
	for name, dir := range map[string]string{
		"a name that is not a task directory": "../0001_test_task",
		"a slash":                             "0001_test_task/x",
		"no number":                           "test_task",
	} {
		t.Run(name, func(t *testing.T) {
			rec := newRecorder(t)
			if _, err := NewTasks(rec.commands(nil)).ReadDir(t.Context(), wt, dir); !errors.Is(err, errTaskDocument) {
				t.Errorf("ReadDir(%q) error = %v, want errTaskDocument", dir, err)
			}
			if calls := rec.recorded(); len(calls) != 0 {
				t.Errorf("git ran for a malformed name: %v", calls)
			}
		})
	}
	t.Run("a directory origin/main lacks", func(t *testing.T) {
		rec := newRecorder(t)
		rec.reply(lsTasks, []byte("docs/tasks/0001_test_task\x00"))
		if _, err := NewTasks(rec.commands(nil)).ReadDir(t.Context(), wt, "0002_test_task"); !errors.Is(err, errTaskDocument) {
			t.Errorf("error = %v, want errTaskDocument", err)
		}
	})
}
