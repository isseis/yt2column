//go:build test

package numreserve

import (
	"slices"
	"strings"
	"testing"

	cmdrunnertestutil "github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// The commits of a check scenario besides mainSHA and otherSHA.
const (
	checkHeadSHA = "5555555555555555555555555555555555555555"
	checkBaseSHA = "6666666666666666666666666666666666666666"
	// checkHeadBranch is the PR's head branch, listed by git ls-remote at
	// checkHeadSHA.
	checkHeadBranch = "owner/0104-new-01"
)

// checkBranch is one origin branch of a check scenario, with the entries its
// tree holds under docs/tasks/ and docs/dev/adr/ (a name ending in "/" is a
// tree).
type checkBranch struct {
	name  string
	sha   string
	tasks []string
	adrs  []string
}

// checkScenario describes what CheckAdded reads: the PR head and merge-base
// trees, and, when the PR adds a number, the latest origin/main, the other
// origin branches, and the record tags.
type checkScenario struct {
	headTasks, baseTasks []string
	headADRs, baseADRs   []string
	// mainTasks and mainADRs are the latest origin/main's entries.
	mainTasks, mainADRs []string
	branches            []checkBranch
	taskTags, adrTags   []string
	// remoteExtra holds raw lines the second git ls-remote adds after the
	// others.
	remoteExtra []string
	// addsTask and addsADR declare the spaces the PR adds a number to, which
	// are the spaces the check must go on to read; with neither, the check
	// must stop reading after the head and merge-base trees.
	addsTask, addsADR bool
}

// branchLines lists main, the PR head branch, and the other branches.
func (s checkScenario) branchLines() string {
	var b strings.Builder
	b.WriteString(mainSHA + "\trefs/heads/main\n")
	b.WriteString(checkHeadSHA + "\trefs/heads/" + checkHeadBranch + "\n")
	for _, br := range s.branches {
		b.WriteString(br.sha + "\trefs/heads/" + br.name + "\n")
	}
	return b.String()
}

// firstSteps are the commands every check makes, up to and including the
// trees of the head and the merge-base.
func (s checkScenario) firstSteps(t *testing.T) []cmdrunnertestutil.Step {
	t.Helper()
	return []cmdrunnertestutil.Step{
		rootStep(t.TempDir(), nil),
		lsRemoteStepFor(s.branchLines(), nil),
		fetchStep(nil, mainSHA, checkHeadSHA),
		gitStep(checkBaseSHA+"\n", nil, "merge-base", checkHeadSHA, mainSHA),
		treeStep(checkHeadSHA, tasksDir, treeOut(tasksDir, s.headTasks...), nil),
		treeStep(checkBaseSHA, tasksDir, treeOut(tasksDir, s.baseTasks...), nil),
		treeStep(checkHeadSHA, adrDir, treeOut(adrDir, s.headADRs...), nil),
		treeStep(checkBaseSHA, adrDir, treeOut(adrDir, s.baseADRs...), nil),
	}
}

// steps are every command the check of the scenario makes, each succeeding.
func (s checkScenario) steps(t *testing.T) []cmdrunnertestutil.Step {
	t.Helper()
	steps := s.firstSteps(t)
	task, adr := s.addsTask, s.addsADR
	if !task && !adr {
		return steps
	}
	var patterns []string
	var tags strings.Builder
	if task {
		patterns = append(patterns, "refs/tags/task-number/*")
		for _, n := range s.taskTags {
			tags.WriteString(mainSHA + "\trefs/tags/task-number/" + n + "\n")
		}
	}
	if adr {
		patterns = append(patterns, "refs/tags/adr-number/*")
		for _, n := range s.adrTags {
			tags.WriteString(mainSHA + "\trefs/tags/adr-number/" + n + "\n")
		}
	}
	for _, line := range s.remoteExtra {
		tags.WriteString(line + "\n")
	}
	steps = append(steps, lsRemoteStepFor(s.branchLines()+tags.String(), nil, patterns...))
	shas := []string{mainSHA, checkHeadSHA}
	for _, br := range s.branches {
		if !slices.Contains(shas, br.sha) {
			shas = append(shas, br.sha)
		}
	}
	steps = append(steps, fetchStep(nil, shas...))
	space := func(dir string, main []string, entries func(checkBranch) []string) {
		steps = append(steps, treeStep(mainSHA, dir, treeOut(dir, main...), nil))
		read := map[string]struct{}{}
		for _, br := range s.branches {
			if _, done := read[br.sha]; done {
				continue
			}
			read[br.sha] = struct{}{}
			steps = append(steps, treeStep(br.sha, dir, treeOut(dir, entries(br)...), nil))
		}
	}
	if task {
		space(tasksDir, s.mainTasks, func(b checkBranch) []string { return b.tasks })
	}
	if adr {
		space(adrDir, s.mainADRs, func(b checkBranch) []string { return b.adrs })
	}
	return steps
}
