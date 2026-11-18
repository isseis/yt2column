//go:build test

package numreserve

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
	"github.com/isseis/yt2column/internal/tasknaming"
)

func number(t *testing.T, s string) tasknaming.Number {
	t.Helper()
	n, err := tasknaming.ParseNumber(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestCheckAdded runs the pre-merge check on scripted inputs. The Fake fails
// on any command not scripted, so a row whose scenario adds no number also
// checks that the check reads nothing after the head and merge-base trees,
// and a row that adds to one space checks that only that space's record
// tags and trees are read.
func TestCheckAdded(t *testing.T) {
	baseTasks := []string{"0000_template/", "0100_number_reservation/"}
	baseADRs := []string{"0051-number-reservation.md", "README.md"}
	withTask := func(name string) []string { return append(slices.Clone(baseTasks), name) }
	withADR := func(name string) []string { return append(slices.Clone(baseADRs), name) }
	tasks := func(head ...string) checkScenario {
		return checkScenario{
			headTasks: append(slices.Clone(baseTasks), head...), baseTasks: baseTasks,
			headADRs: baseADRs, baseADRs: baseADRs,
			mainTasks: baseTasks, mainADRs: baseADRs, addsTask: true,
		}
	}
	adrs := func(head ...string) checkScenario {
		return checkScenario{
			headTasks: baseTasks, baseTasks: baseTasks,
			headADRs: append(slices.Clone(baseADRs), head...), baseADRs: baseADRs,
			mainTasks: baseTasks, mainADRs: baseADRs, addsADR: true,
		}
	}
	with := func(s checkScenario, change func(*checkScenario)) checkScenario {
		change(&s)
		return s
	}
	type issue struct {
		space  Space
		number string
		kind   AddedIssueKind
		path   string
		usedBy string
		branch string
	}
	tests := []struct {
		name string
		s    checkScenario
		want []issue
	}{
		{"adds no number", checkScenario{headTasks: baseTasks, baseTasks: baseTasks, headADRs: withADR("notes.txt"), baseADRs: baseADRs}, nil},
		{
			// The PR head still has 0094_old, which main renamed to 0096_old
			// after the PR branched: against the latest main 0094_old would
			// look added, against the merge-base it is not.
			"entry renamed on main after the PR branched",
			checkScenario{headTasks: withTask("0094_old/"), baseTasks: withTask("0094_old/"), headADRs: baseADRs, baseADRs: baseADRs},
			nil,
		},
		{"number already on main", checkScenario{headTasks: baseTasks, baseTasks: baseTasks, headADRs: baseADRs, baseADRs: baseADRs}, nil},
		{"task at the threshold without a record", tasks("0103_new/"), nil},
		{"task above the threshold without a record", tasks("0104_new/"), []issue{{SpaceTask, "0104", AddedUnreserved, "docs/tasks/0104_new", "", ""}}},
		{"task above the threshold with a record", with(tasks("0104_new/"), func(s *checkScenario) { s.taskTags = []string{"0104"} }), nil},
		{"adr at the threshold without a record", with(adrs("0051-other.md"), func(s *checkScenario) {
			s.baseADRs = []string{"0050-a.md", "README.md"}
			s.headADRs = []string{"0050-a.md", "0051-other.md", "README.md"}
			s.mainADRs = s.baseADRs
		}), nil},
		{"adr above the threshold without a record", adrs("0052-new.md"), []issue{{SpaceADR, "0052", AddedUnreserved, "docs/dev/adr/0052-new.md", "", ""}}},
		{"adr above the threshold with a record", with(adrs("0052-new.md"), func(s *checkScenario) { s.adrTags = []string{"0052"} }), nil},
		{"another entry on the latest main", with(tasks("0099_new/"), func(s *checkScenario) { s.mainTasks = withTask("0099_other/") }), []issue{
			{SpaceTask, "0099", AddedInUseOnMain, "docs/tasks/0099_new", "docs/tasks/0099_other", ""},
		}},
		{"the same entry on the latest main", with(tasks("0099_new/"), func(s *checkScenario) { s.mainTasks = withTask("0099_new/") }), nil},
		{"another adr on the latest main", with(adrs("0050-new.md"), func(s *checkScenario) { s.mainADRs = withADR("0050-other.md") }), []issue{
			{SpaceADR, "0050", AddedInUseOnMain, "docs/dev/adr/0050-new.md", "docs/dev/adr/0050-other.md", ""},
		}},
		{"another branch adds another name", with(tasks("0099_new/"), func(s *checkScenario) {
			s.branches = []checkBranch{{name: "owner/0099-other-01", sha: otherSHA, tasks: withTask("0099_other/")}}
		}), []issue{{SpaceTask, "0099", AddedOnOtherBranch, "docs/tasks/0099_new", "docs/tasks/0099_other", "owner/0099-other-01"}}},
		{"another branch adds another adr", with(adrs("0050-new.md"), func(s *checkScenario) {
			s.branches = []checkBranch{{name: "poc/x", sha: otherSHA, adrs: withADR("0050-other.md")}}
		}), []issue{{SpaceADR, "0050", AddedOnOtherBranch, "docs/dev/adr/0050-new.md", "docs/dev/adr/0050-other.md", "poc/x"}}},
		{"another branch adds the same name", with(tasks("0099_new/"), func(s *checkScenario) {
			s.branches = []checkBranch{{name: "owner/0099-new-02", sha: otherSHA, tasks: withTask("0099_new/")}}
		}), nil},
		{"another branch holds main's entry of the number", with(tasks("0099_new/"), func(s *checkScenario) {
			s.mainTasks = withTask("0099_other/")
			s.branches = []checkBranch{{name: "old", sha: otherSHA, tasks: withTask("0099_other/")}}
		}), []issue{{SpaceTask, "0099", AddedInUseOnMain, "docs/tasks/0099_new", "docs/tasks/0099_other", ""}}},
		{"branches sharing a head are read once", with(tasks("0099_new/"), func(s *checkScenario) {
			s.branches = []checkBranch{
				{name: "a", sha: otherSHA, tasks: withTask("0099_other/")},
				{name: "b", sha: otherSHA, tasks: withTask("0099_other/")},
			}
		}), []issue{
			{SpaceTask, "0099", AddedOnOtherBranch, "docs/tasks/0099_new", "docs/tasks/0099_other", "a"},
			{SpaceTask, "0099", AddedOnOtherBranch, "docs/tasks/0099_new", "docs/tasks/0099_other", "b"},
		}},
		{"malformed entries the PR adds under docs/tasks", checkScenario{
			headTasks: withTask("notes.md"), baseTasks: baseTasks, headADRs: baseADRs, baseADRs: baseADRs,
		}, []issue{{SpaceTask, "", AddedMalformed, "docs/tasks/notes.md", "", ""}}},
		{"malformed directory beside an added task", with(tasks("0104_new/", "draft/"), func(s *checkScenario) { s.taskTags = []string{"0104"} }), []issue{
			{SpaceTask, "", AddedMalformed, "docs/tasks/draft", "", ""},
		}},
		{"a malformed tag of the other space is not judged", with(tasks("0104_new/"), func(s *checkScenario) {
			s.taskTags = []string{"0104"}
			s.remoteExtra = []string{mainSHA + "\trefs/tags/adr-number/bad"}
		}), nil},
		{"latest main entries of another form are skipped", with(tasks("0099_new/"), func(s *checkScenario) { s.mainTasks = withTask("stray.md") }), nil},
		{"both spaces", with(tasks("0104_new/"), func(s *checkScenario) {
			s.headADRs = withADR("0052-new.md")
			s.addsADR = true
		}), []issue{
			{SpaceTask, "0104", AddedUnreserved, "docs/tasks/0104_new", "", ""},
			{SpaceADR, "0052", AddedUnreserved, "docs/dev/adr/0052-new.md", "", ""},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := newTestTool(cmdrunnertestutil.New(t, tt.s.steps(t)...), defaultBounds())
			got, err := tool.CheckAdded(context.Background(), checkHeadSHA, checkHeadBranch)
			if err != nil {
				t.Fatalf("CheckAdded: %v", err)
			}
			var want []AddedNumberIssue
			for _, w := range tt.want {
				var n tasknaming.Number
				if w.number != "" {
					n = number(t, w.number)
				}
				want = append(want, AddedNumberIssue{Space: w.space, Number: n, Kind: w.kind, Path: w.path, UsedBy: w.usedBy, Branch: w.branch})
			}
			if !slices.Equal(got, want) {
				t.Errorf("CheckAdded =\n%+v\nwant\n%+v", got, want)
			}
		})
	}
}

// TestCheckAddedStops checks every input the check cannot read stops it with
// an error, so mergepr cannot merge on a partial reading.
func TestCheckAddedStops(t *testing.T) {
	adds := checkScenario{
		headTasks: []string{"0100_a/", "0104_new/"}, baseTasks: []string{"0100_a/"},
		headADRs: []string{"0051-a.md"}, baseADRs: []string{"0051-a.md"},
		mainTasks: []string{"0100_a/"}, addsTask: true,
	}
	upToStep := func(i int, last cmdrunnertestutil.Step) func(t *testing.T) []cmdrunnertestutil.Step {
		return func(t *testing.T) []cmdrunnertestutil.Step { return append(adds.steps(t)[:i], last) }
	}
	tests := []struct {
		name  string
		head  string
		steps func(t *testing.T) []cmdrunnertestutil.Step
		want  StopKind
	}{
		{"head is not a commit ID", "--upload-pack=x", func(*testing.T) []cmdrunnertestutil.Step { return nil }, StopMalformed},
		{"outside a repository", checkHeadSHA, upToStep(0, rootStep("", gitFailed(128))), StopLocalGit},
		{"branch listing fails", checkHeadSHA, upToStep(1, lsRemoteStepFor("", gitFailed(128))), StopNetworkOrGitHub},
		{"merge-base fails", checkHeadSHA, upToStep(3, gitStep("", gitFailed(1), "merge-base", checkHeadSHA, mainSHA)), StopLocalGit},
		{"merge-base prints another form", checkHeadSHA, upToStep(3, gitStep("HEAD\n", nil, "merge-base", checkHeadSHA, mainSHA)), StopMalformed},
		{"head tree unreadable", checkHeadSHA, upToStep(4, treeStep(checkHeadSHA, tasksDir, "garbage\x00", nil)), StopMalformed},
		{"record tag of another form", checkHeadSHA, upToStep(8, lsRemoteStepFor(
			mainSHA+"\trefs/heads/main\n"+mainSHA+"\trefs/tags/task-number/0104/x\n", nil,
			"refs/tags/task-number/*")), StopMalformed},
		{"branch fetch fails", checkHeadSHA, func(t *testing.T) []cmdrunnertestutil.Step {
			return append(adds.steps(t)[:9],
				fetchStep(gitFailed(128), mainSHA, checkHeadSHA),
				gitStep("", nil, "cat-file", "--batch-check"))
		}, StopNetworkOrGitHub},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := newTestTool(cmdrunnertestutil.New(t, tt.steps(t)...), defaultBounds())
			issues, err := tool.CheckAdded(context.Background(), tt.head, checkHeadBranch)
			if issues != nil {
				t.Errorf("CheckAdded issues = %+v, want none with the error", issues)
			}
			if got := stopKind(t, err); got != tt.want {
				t.Errorf("CheckAdded stop = %v (%v), want %v", got, err, tt.want)
			}
		})
	}
}

// TestCheckAddedRefusedUnderTest checks the production Tool's check runs no
// command inside go test, as Reserve does (TestNewRefusesUnderTest).
func TestCheckAddedRefusedUnderTest(t *testing.T) {
	_, err := New(os.Environ()).CheckAdded(context.Background(), checkHeadSHA, checkHeadBranch)
	if !errors.Is(err, ErrUnderTest) {
		t.Errorf("CheckAdded = %v, want ErrUnderTest", err)
	}
}

// TestAddedIssueLine checks each kind's line names the kind, the space, the
// number, and the paths, and that a name cannot add a line.
func TestAddedIssueLine(t *testing.T) {
	n := number(t, "0104")
	tests := []struct {
		issue AddedNumberIssue
		want  string
	}{
		{
			AddedNumberIssue{Space: SpaceTask, Number: n, Kind: AddedUnreserved, Path: "docs/tasks/0104_a"},
			`unreserved task 0104 "docs/tasks/0104_a": above the threshold 0103 and no record tag`,
		},
		{
			AddedNumberIssue{Space: SpaceADR, Number: n, Kind: AddedInUseOnMain, Path: "docs/dev/adr/0104-a.md", UsedBy: "docs/dev/adr/0104-b.md"},
			`in-use-on-main adr 0104 "docs/dev/adr/0104-a.md": origin/main already has "docs/dev/adr/0104-b.md"`,
		},
		{
			AddedNumberIssue{Space: SpaceTask, Number: n, Kind: AddedOnOtherBranch, Path: "docs/tasks/0104_a", UsedBy: "docs/tasks/0104_b", Branch: "x\ny"},
			`on-other-branch task 0104 "docs/tasks/0104_a": origin branch "x\ny" adds "docs/tasks/0104_b"`,
		},
		{
			AddedNumberIssue{Space: SpaceTask, Kind: AddedMalformed, Path: "docs/tasks/a\nb"},
			`malformed task "docs/tasks/a\nb": not a task directory`,
		},
		{
			AddedNumberIssue{Space: SpaceADR, Number: n, Kind: AddedMalformed + 1, Path: "docs/dev/adr/0104-a.md"},
			`unknown-finding adr 0104 "docs/dev/adr/0104-a.md"`,
		},
		{
			AddedNumberIssue{Space: SpaceTask, Number: n, Kind: addedInvalid, Path: "docs/tasks/0104_a"},
			`unknown-finding task 0104 "docs/tasks/0104_a"`,
		},
	}
	for _, tt := range tests {
		got := tt.issue.Line()
		if got != tt.want {
			t.Errorf("Line() = %s, want %s", got, tt.want)
		}
		if strings.ContainsAny(got, "\r\n") {
			t.Errorf("Line() = %q holds a line break", got)
		}
	}
}
