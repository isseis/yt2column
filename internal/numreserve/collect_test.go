//go:build test

package numreserve

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// TestCollectEachInputRaisesCandidate gives each input of each space a number
// above every other input, so only that input can decide the candidate. The
// baseline rows show the other inputs alone give a lower one. The Fake fails
// on any command not scripted, such as gh pr list, so no row can count a
// fork's PR head.
func TestCollectEachInputRaisesCandidate(t *testing.T) {
	tests := []struct {
		name string
		s    scenario
		want string
	}{
		{"task baseline", scenario{space: SpaceTask}, "0101"},
		{"task origin/main docs/tasks", scenario{space: SpaceTask, mainTasks: []string{"0000_template/", "0100_a/", "0105_b/"}}, "0106"},
		{"task origin branch", scenario{space: SpaceTask, branches: []remoteBranch{{name: "owner/0105-foo-01", sha: otherSHA}}}, "0106"},
		{"task local branch", scenario{space: SpaceTask, local: []string{"owner/0105-foo-01"}}, "0106"},
		{"task record tag", scenario{space: SpaceTask, tags: []string{"0105"}}, "0106"},
		{"task names of other forms are not counted", scenario{
			space:    SpaceTask,
			branches: []remoteBranch{{name: "poc/0105-wasm", sha: otherSHA}},
			local:    []string{"0110-wip", "main"},
		}, "0101"},
		{"task line matched at the end of a name is skipped", scenario{
			space:       SpaceTask,
			remoteExtra: []string{mainSHA + "\trefs/tags/x/refs/tags/task-number/0110", mainSHA + "\trefs/remotes/x/refs/heads/owner/0110-a-01"},
		}, "0101"},
		{"adr baseline", scenario{space: SpaceADR}, "0052"},
		{"adr origin/main docs/dev/adr", scenario{space: SpaceADR, mainADRs: []string{"0051-a.md", "0055-b.md", "README.md"}}, "0056"},
		{"adr origin branch", scenario{space: SpaceADR, branches: []remoteBranch{{name: "poc/x", sha: otherSHA, adrs: []string{"0055-b.md"}}}}, "0056"},
		{"adr HEAD", scenario{space: SpaceADR, headADRs: []string{"0055-b.md"}}, "0056"},
		{"adr work tree only", scenario{space: SpaceADR, workADRs: []string{"0055-b.md"}}, "0056"},
		{"adr record tag", scenario{space: SpaceADR, tags: []string{"0055"}}, "0056"},
		{"adr branches sharing a head are read once", scenario{space: SpaceADR, branches: []remoteBranch{
			{name: "a", sha: otherSHA, adrs: []string{"0055-b.md"}},
			{name: "b", sha: otherSHA},
		}}, "0056"},
		{"adr names of other forms are not counted", scenario{
			space:    SpaceADR,
			branches: []remoteBranch{{name: "a", sha: otherSHA, adrs: []string{"README.md", "0060_bad.md", "notes.txt", "0061-dir/"}}},
			workADRs: []string{"0062-Draft.md", "0063.md"},
		}, "0052"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := tt.s.tag(tt.want)
			steps := append(tt.s.collectSteps(t), postStep(tag, created(t), nil))
			got, err := run(t, tt.s.space, defaultBounds(), steps...)
			if err != nil {
				t.Fatalf("Reserve: %v", err)
			}
			if got.Number.String() != tt.want || got.Tag != tag || got.Commit != mainSHA {
				t.Errorf("Reserve = %+v, want %s (%s at %s)", got, tt.want, tag, mainSHA)
			}
		})
	}
}

// TestCollectSpacesIndependent checks a record tag of one space, even one
// listed by git ls-remote, does not move the other space's candidate.
func TestCollectSpacesIndependent(t *testing.T) {
	tests := []struct {
		space Space
		other string
		want  string
	}{
		{SpaceTask, "refs/tags/adr-number/0200", "0101"},
		{SpaceADR, "refs/tags/task-number/0200", "0052"},
	}
	for _, tt := range tests {
		t.Run(tt.other, func(t *testing.T) {
			s := scenario{space: tt.space, remoteExtra: []string{mainSHA + "\t" + tt.other}}
			steps := append(s.collectSteps(t), postStep(s.tag(tt.want), created(t), nil))
			got, err := run(t, tt.space, defaultBounds(), steps...)
			if err != nil {
				t.Fatalf("Reserve: %v", err)
			}
			if got.Number.String() != tt.want {
				t.Errorf("Reserve = %s, want %s", got.Number, tt.want)
			}
		})
	}
}

// The step indexes of collectSteps.
const (
	taskRoot   = 0
	taskRemote = 1
	taskFetch  = 2
	taskMain   = 3
	taskLocal  = 4
	adrRoot    = 0
	adrRemote  = 1
	adrFetch   = 2
	adrMain    = 3
)

// upTo returns the scenario's steps before index i, then last.
func upTo(t *testing.T, s scenario, i int, last ...cmdrunnertestutil.Step) []cmdrunnertestutil.Step {
	t.Helper()
	return append(s.collectSteps(t)[:i], last...)
}

// TestCollectRejects checks every input of a form the tool does not accept,
// every exceeded listing bound, and every failed command stops the run
// before any creation, with the declared kind.
func TestCollectRejects(t *testing.T) {
	task := scenario{space: SpaceTask}
	adr := scenario{space: SpaceADR}
	two := scenario{space: SpaceADR, branches: []remoteBranch{{name: "owner/0100-a-01", sha: otherSHA}}}
	tooLarge := fmt.Errorf("git: %w", cmdrunner.ErrOutputTooLarge)
	remoteOnly := func(out string) func(t *testing.T) []cmdrunnertestutil.Step {
		return func(t *testing.T) []cmdrunnertestutil.Step {
			return []cmdrunnertestutil.Step{rootStep(t.TempDir(), nil), lsRemoteStep(task.tagPrefix(), out, nil)}
		}
	}
	withRemote := func(s scenario) func(t *testing.T) []cmdrunnertestutil.Step {
		return func(t *testing.T) []cmdrunnertestutil.Step { return upTo(t, s, taskRemote+1) }
	}
	tests := []struct {
		name   string
		space  Space
		bounds func(*bounds)
		steps  func(t *testing.T) []cmdrunnertestutil.Step
		want   StopKind
	}{
		{name: "ls-remote short sha", space: SpaceTask, steps: withRemote(scenario{space: SpaceTask, remoteExtra: []string{"abc\trefs/heads/x"}}), want: StopMalformed},
		{name: "ls-remote uppercase sha", space: SpaceTask, steps: withRemote(scenario{space: SpaceTask, remoteExtra: []string{strings.Repeat("A", 40) + "\trefs/heads/x"}}), want: StopMalformed},
		{name: "ls-remote space for tab", space: SpaceTask, steps: withRemote(scenario{space: SpaceTask, remoteExtra: []string{otherSHA + " refs/heads/x"}}), want: StopMalformed},
		{name: "ls-remote empty line", space: SpaceTask, steps: withRemote(scenario{space: SpaceTask, remoteExtra: []string{""}}), want: StopMalformed},
		{name: "record tag nested name", space: SpaceTask, steps: withRemote(scenario{space: SpaceTask, tags: []string{"a/0002"}}), want: StopMalformed},
		{name: "record tag peeled line", space: SpaceTask, steps: withRemote(scenario{space: SpaceTask, tags: []string{"0005^{}"}}), want: StopMalformed},
		{name: "record tag three digits", space: SpaceTask, steps: withRemote(scenario{space: SpaceTask, tags: []string{"105"}}), want: StopMalformed},
		{name: "record tag with control characters", space: SpaceTask, steps: withRemote(scenario{space: SpaceTask, tags: []string{"\x1b[31m" + strings.Repeat("9", 300)}}), want: StopMalformed},
		{name: "adr record tag nested name", space: SpaceADR, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, scenario{space: SpaceADR, tags: []string{"x/0001"}}, adrRemote+1)
		}, want: StopMalformed},
		{name: "origin has no main", space: SpaceTask, steps: remoteOnly(otherSHA + "\trefs/heads/owner/0100-a-01\n"), want: StopMalformed},
		{
			name: "origin has more branches than the bound", space: SpaceTask, bounds: func(b *bounds) { b.maxBranches = 1 },
			steps: withRemote(scenario{space: SpaceTask, branches: []remoteBranch{{name: "x", sha: otherSHA}}}), want: StopListLimit,
		},
		{name: "ls-remote output exceeds the cap", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskRemote, lsRemoteStep(task.tagPrefix(), "", tooLarge))
		}, want: StopListLimit},
		{name: "ls-remote fails", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskRemote, lsRemoteStep(task.tagPrefix(), "", gitFailed(128)))
		}, want: StopNetworkOrGitHub},
		{name: "ls-remote times out", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskRemote, lsRemoteStep(task.tagPrefix(), "", context.DeadlineExceeded))
		}, want: StopNetworkOrGitHub},
		{name: "git cannot be run", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskRemote, lsRemoteStep(task.tagPrefix(), "", os.ErrNotExist))
		}, want: StopLocalGit},
		{name: "fetch fails with a head missing", space: SpaceADR, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, two, adrFetch, fetchStep(gitFailed(128), mainSHA, otherSHA),
				gitStep(mainSHA+" commit 200\n"+otherSHA+" missing\n", nil, "cat-file", "--batch-check"))
		}, want: StopNetworkOrGitHub},
		{name: "fetch fails with no head missing", space: SpaceADR, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, two, adrFetch, fetchStep(gitFailed(1), mainSHA, otherSHA),
				gitStep(mainSHA+" commit 200\n"+otherSHA+" commit 200\n", nil, "cat-file", "--batch-check"))
		}, want: StopNetworkOrGitHub},
		{name: "fetch fails and cat-file fails", space: SpaceADR, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, two, adrFetch, fetchStep(gitFailed(128), mainSHA, otherSHA), gitStep("", gitFailed(128), "cat-file", "--batch-check"))
		}, want: StopLocalGit},
		{name: "fetch times out", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskFetch, fetchStep(context.DeadlineExceeded, mainSHA))
		}, want: StopNetworkOrGitHub},
		{name: "main docs/tasks holds a file", space: SpaceTask, steps: mainTasks(treeOut(tasksDir, "0000_template/", "README.md")), want: StopMalformed},
		{name: "main docs/tasks holds a directory of another form", space: SpaceTask, steps: mainTasks(treeOut(tasksDir, "0100_a/", "0101-b/")), want: StopMalformed},
		{name: "main docs/tasks is empty", space: SpaceTask, steps: mainTasks(""), want: StopMalformed},
		{name: "main listing without the final NUL", space: SpaceTask, steps: mainTasks(strings.TrimSuffix(treeOut(tasksDir, "0100_a/"), "\x00")), want: StopMalformed},
		{name: "main listing entry below a subdirectory", space: SpaceTask, steps: mainTasks("040000 tree " + blobSHA + "\tdocs/tasks/0100_a/0101_b\x00"), want: StopMalformed},
		{name: "main listing entry outside the directory", space: SpaceTask, steps: mainTasks("040000 tree " + blobSHA + "\tdocs/0100_a\x00"), want: StopMalformed},
		{name: "main listing entry of another form", space: SpaceTask, steps: mainTasks("040000 tree " + blobSHA + " docs/tasks/0100_a\x00"), want: StopMalformed},
		{name: "main listing fails", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskMain, treeStep(mainSHA, tasksDir, "", gitFailed(128)))
		}, want: StopLocalGit},
		{name: "main docs/dev/adr holds another file", space: SpaceADR, steps: mainADRs(treeOut(adrDir, "0051-a.md", "notes.md")), want: StopMalformed},
		{name: "main docs/dev/adr holds a tree named like an ADR", space: SpaceADR, steps: mainADRs(treeOut(adrDir, "0051-a.md", "0052-b.md/")), want: StopMalformed},
		{name: "main docs/dev/adr holds only the index", space: SpaceADR, steps: mainADRs(treeOut(adrDir, "README.md")), want: StopMalformed},
		{name: "branch listing entry of another form", space: SpaceADR, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, two, adrMain+1, treeStep(otherSHA, adrDir, "bad\x00", nil))
		}, want: StopMalformed},
		{name: "local branch line outside refs/heads", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskLocal, localStep("refs/tags/x\n", nil))
		}, want: StopMalformed},
		{name: "local branch listing fails", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskLocal, localStep("", gitFailed(129)))
		}, want: StopLocalGit},
		{name: "local branch listing exceeds the cap", space: SpaceTask, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			return upTo(t, task, taskLocal, localStep("", tooLarge))
		}, want: StopListLimit},
		{name: "task run outside a repository", space: SpaceTask, steps: func(_ *testing.T) []cmdrunnertestutil.Step {
			return []cmdrunnertestutil.Step{rootStep("", gitFailed(128))}
		}, want: StopLocalGit},
		{name: "work tree root not found", space: SpaceADR, steps: func(_ *testing.T) []cmdrunnertestutil.Step {
			return []cmdrunnertestutil.Step{rootStep("", gitFailed(128))}
		}, want: StopLocalGit},
		{name: "work tree root not absolute", space: SpaceADR, steps: func(_ *testing.T) []cmdrunnertestutil.Step {
			return []cmdrunnertestutil.Step{rootStep("relative/dir", nil)}
		}, want: StopLocalGit},
		{name: "work tree without docs/dev/adr", space: SpaceADR, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			steps := adr.collectSteps(t)
			steps[adrRoot] = rootStep(t.TempDir(), nil)
			return steps
		}, want: StopLocalGit},
		{name: "HEAD listing fails", space: SpaceADR, steps: func(t *testing.T) []cmdrunnertestutil.Step {
			steps := adr.collectSteps(t)
			return append(steps[:len(steps)-1], treeStep("HEAD", adrDir, "", gitFailed(128)))
		}, want: StopLocalGit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := defaultBounds()
			if tt.bounds != nil {
				tt.bounds(&b)
			}
			_, err := run(t, tt.space, b, tt.steps(t)...)
			if got := stopKind(t, err); got != tt.want {
				t.Errorf("kind = %v (%v), want %v", got, err, tt.want)
			}
		})
	}
}

func mainTasks(listing string) func(t *testing.T) []cmdrunnertestutil.Step {
	return func(t *testing.T) []cmdrunnertestutil.Step {
		return upTo(t, scenario{space: SpaceTask}, taskMain, treeStep(mainSHA, tasksDir, listing, nil))
	}
}

func mainADRs(listing string) func(t *testing.T) []cmdrunnertestutil.Step {
	return func(t *testing.T) []cmdrunnertestutil.Step {
		return upTo(t, scenario{space: SpaceADR}, adrMain, treeStep(mainSHA, adrDir, listing, nil))
	}
}

// TestFetchFailureNamesMissingBranch checks the detail of a failed fetch
// names the branch whose head git cat-file reports missing, and only it, and
// that cat-file is asked about every fetched head.
func TestFetchFailureNamesMissingBranch(t *testing.T) {
	s := scenario{space: SpaceADR, branches: []remoteBranch{
		{name: "owner/0100-present-01", sha: otherSHA},
		{name: "owner/0100-gone-01", sha: thirdSHA},
	}}
	fake := cmdrunnertestutil.New(t, upTo(t, s, adrFetch, fetchStep(gitFailed(128), mainSHA, otherSHA, thirdSHA),
		gitStep(mainSHA+" commit 1\n"+otherSHA+" commit 1\n"+thirdSHA+" missing\n", nil, "cat-file", "--batch-check"))...)
	_, err := newTestTool(fake, defaultBounds()).Reserve(context.Background(), SpaceADR)
	if kind := stopKind(t, err); kind != StopNetworkOrGitHub {
		t.Errorf("kind = %v, want network-or-github", kind)
	}
	text := err.Error()
	if !strings.Contains(text, `"owner/0100-gone-01"`) || strings.Contains(text, "present") {
		t.Errorf("stop %q does not name exactly the missing branch", text)
	}
	if want := mainSHA + "\n" + otherSHA + "\n" + thirdSHA + "\n"; string(fake.Stdin[len(fake.Stdin)-1]) != want {
		t.Errorf("cat-file stdin = %q, want %q", fake.Stdin[len(fake.Stdin)-1], want)
	}
}

// TestRealListingSamples reads the captured git outputs, so the accepted
// forms are the ones git really prints.
func TestRealListingSamples(t *testing.T) {
	ctx := context.Background()
	remote := readSample(t, "git/ls-remote.txt")
	fake := cmdrunnertestutil.New(t,
		treeStep(mainSHA, tasksDir, readSample(t, "git/ls-tree-tasks.z"), nil),
		treeStep(mainSHA, adrDir, readSample(t, "git/ls-tree-adr.z"), nil),
	)
	tool := newTestTool(fake, defaultBounds())
	tasks, err := tool.readMainList(ctx, mainSHA, tasksDir)
	if err != nil || maxOf(tasks) != 100 || len(tasks) != 101 {
		t.Errorf("readMainList(docs/tasks/) = %d numbers, max %d, %v; want 101, max 100", len(tasks), maxOf(tasks), err)
	}
	adrs, err := tool.readMainList(ctx, mainSHA, adrDir)
	if err != nil || maxOf(adrs) != 51 || len(adrs) != 51 {
		t.Errorf("readMainList(docs/dev/adr/) = %d numbers, max %d, %v; want 51, max 51", len(adrs), maxOf(adrs), err)
	}

	// The probe tag in the sample is outside the task namespace, so it is
	// skipped, not counted and not rejected.
	fake = cmdrunnertestutil.New(t, lsRemoteStep("refs/tags/task-number/", remote, nil))
	refs, err := newTestTool(fake, defaultBounds()).listRemote(ctx, "refs/tags/task-number/")
	if err != nil || len(refs.branches) != 12 || len(refs.tags) != 0 || refs.mainSHA != "424dc7dc47527e37ab65eaf3f3f76567d596fe45" {
		t.Errorf("listRemote(task) = %d branches, %d tags, main %s, %v; want 12, 0, 424dc7dc4752", len(refs.branches), len(refs.tags), refs.mainSHA, err)
	}
}

// TestBranchBoundEdge checks a run with exactly maxBranches origin branches
// still decides a number; one more stops (TestCollectRejects).
func TestBranchBoundEdge(t *testing.T) {
	s := scenario{space: SpaceTask, branches: []remoteBranch{{name: "x", sha: otherSHA}}}
	b := defaultBounds()
	b.maxBranches = 2
	got, err := run(t, SpaceTask, b, append(s.collectSteps(t), postStep(s.tag("0101"), created(t), nil))...)
	if err != nil || got.Number.String() != "0101" {
		t.Errorf("Reserve = %+v, %v; want 0101", got, err)
	}
}

// TestStopDetailBoundsValues checks an untrusted value is cut to 200 bytes
// before it is quoted, and a list of inputs names at most ten of them.
func TestStopDetailBoundsValues(t *testing.T) {
	long := strings.Repeat("x", 300)
	s := scenario{space: SpaceTask, tags: []string{long}}
	_, err := run(t, SpaceTask, defaultBounds(), upTo(t, s, taskRemote+1)...)
	stopKind(t, err)
	if text := err.Error(); !strings.Contains(text, strconv.Quote(s.tag(long)[:200])) || strings.Contains(text, s.tag(long)[:201]) {
		t.Errorf("stop %q does not carry the value cut to 200 bytes", text)
	}

	var local []string
	for i := range 11 {
		local = append(local, fmt.Sprintf("owner/%04d-stray-01", 200+i))
	}
	s = scenario{space: SpaceTask, local: local}
	_, err = run(t, SpaceTask, defaultBounds(), s.collectSteps(t)...)
	stopKind(t, err)
	text := err.Error()
	if !strings.Contains(text, "and 1 more") || !strings.Contains(text, local[9]) || strings.Contains(text, local[10]) {
		t.Errorf("stop %q does not name exactly ten inputs and count the rest", text)
	}
}
