//go:build test

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// reviewRecordsPath is the work tree's copy of the review records script.
// The tests run it directly to check its behavior; that the command
// documents run only origin/main's copy is TestReviewRecordsCallSite's
// concern.
const reviewRecordsPath = "../../.claude/commands/_lib/review-records.sh"

// The fixture's plans: one with PR markers and one without. Their contents
// are the samples under testdata/review_records/.
const (
	rrMarkersPlan = "docs/tasks/0001_markers/03_implementation_plan.md"
	rrSinglePlan  = "docs/tasks/0002_single/03_implementation_plan.md"
	rrSource      = "src.txt"
)

// rrPlanSample returns the content of a plan sample.
func rrPlanSample(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "review_records", name))
	if err != nil {
		t.Fatalf("read the plan sample %s: %v", name, err)
	}
	return string(data)
}

// fableReviewPath is the Fable partial review's procedure, whose "Record
// lines" define the Fable records the fable mode reads.
const fableReviewPath = "../../.claude/commands/_lib/fable-review.md"

// recordTableForm returns the form of the record line named line in a
// record-line table of section: the first code span of the second cell of
// the row whose first cell is the code span line. It fails t unless the form
// holds each of placeholders once. The fixtures write their lines from the
// forms, so a form a document changes and the script does not follow fails
// the tests.
func recordTableForm(t *testing.T, where, section, line string, placeholders ...string) string {
	t.Helper()
	for row := range strings.Lines(section) {
		cells := strings.Split(strings.TrimSpace(row), " | ")
		if len(cells) < 2 || cells[0] != "| `"+line+"`" {
			continue
		}
		form, ok := firstCodeSpan(cells[1])
		if !ok {
			break
		}
		for _, placeholder := range placeholders {
			if strings.Count(form, placeholder) != 1 {
				t.Fatalf("the form %q of %s does not hold %s once", form, line, placeholder)
			}
		}
		return form
	}
	t.Fatalf("%s has no `%s` row with a form", where, line)
	return ""
}

// localReviewStep7 returns step 7 of local-cli-review.md's procedure, whose
// table defines the local CLI review's record lines.
func localReviewStep7(t *testing.T) string {
	t.Helper()
	procedure := requireSection(t, "local-cli-review.md", doctestutil.ReadDoc(t, localReviewPath), "## Procedure")
	return procedureStep(procedure, 7)
}

// seenRangeForm reads the form of the seen-range line from the table of
// local-cli-review.md step 7.
func seenRangeForm(t *testing.T) string {
	t.Helper()
	return recordTableForm(t, "local-cli-review.md step 7", localReviewStep7(t), "Local CLI reviewed:", "<START12>", "<END12>", "(coderabbit)")
}

// rrFixture is one fixture repository: main has two commits (m0, then mb,
// which adds the plans and _context.md when provided), refs/remotes/origin/main
// points at mb, and the branch feature forks from it.
type rrFixture struct {
	*gitFixture
	form       string
	forms      fableForms
	bash       scriptBash
	contextDoc string
	m0, mb     string
}

// sibling returns a new fixture with f's forms, bash, and contextDoc, for a
// setup that checks first that another layout changes the output.
func (f *rrFixture) sibling() *rrFixture {
	f.t.Helper()
	s := newRRFixtureWithContext(f.t, f.form, f.contextDoc)
	s.forms, s.bash = f.forms, f.bash
	return s
}

// probe builds a branch with build on a sibling and returns the first line
// the cross-check mode prints for it at launch.
func (f *rrFixture) probe(build func(s *rrFixture)) string {
	f.t.Helper()
	s := f.sibling()
	build(s)
	run := s.runScript(s.bash, reviewRecordsPath, append([]string{"cross-check"}, rrFable(s, "launch")...), scriptRunOptions{})
	first, _, _ := strings.Cut(run.stdout, "\n")
	return first
}

// requireProbe fails the case unless probe prints want, so a case whose
// layout does not change the output by the rule it checks checks nothing.
func (f *rrFixture) requireProbe(want string, build func(s *rrFixture)) {
	f.t.Helper()
	if got := f.probe(build); got != want {
		f.t.Fatalf("the probe layout printed %q, not %q, so the case checks nothing", got, want)
	}
}

func newRRFixture(t *testing.T, form string) *rrFixture {
	t.Helper()
	return newRRFixtureWithContext(t, form, doctestutil.ReadDoc(t, contextPath))
}

// newRRFixtureWithContext builds a fixture repository whose origin/main holds
// contextDoc at .claude/commands/_context.md (omitted when contextDoc is "").
func newRRFixtureWithContext(t *testing.T, form, contextDoc string) *rrFixture {
	t.Helper()
	f := &rrFixture{gitFixture: newGitFixture(t), form: form, contextDoc: contextDoc}
	f.write(rrSource, "m0\n")
	f.commit("m0")
	f.m0 = f.rev("HEAD")
	f.write(rrMarkersPlan, rrPlanSample(t, "markers.md"))
	f.write(rrSinglePlan, rrPlanSample(t, "single.md"))
	if contextDoc != "" {
		f.write(contextDocRepoPath, contextDoc)
	}
	f.commit("add the plans")
	f.mb = f.rev("HEAD")
	f.git("update-ref", "refs/remotes/origin/main", f.mb)
	f.git("switch", "-q", "-c", "feature")
	return f
}

func (f *rrFixture) head() string {
	f.t.Helper()
	return f.rev("HEAD")
}

// work commits a change to the source file with subject and returns the
// commit.
func (f *rrFixture) work(subject string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.repo, rrSource))
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(rrSource, string(data)+subject+"\n")
	f.commit(subject)
	return f.head()
}

// record makes an empty commit whose message is subject and body.
func (f *rrFixture) record(subject string, body ...string) {
	f.t.Helper()
	message := subject
	if len(body) > 0 {
		message += "\n\n" + strings.Join(body, "\n")
	}
	f.git("commit", "-q", "--allow-empty", "-m", message)
}

// seen is a seen-range line for start..end reviewed by cli, in the form of
// local-cli-review.md.
func (f *rrFixture) seen(start, end, cli string) string {
	return strings.NewReplacer("<START12>", start[:12], "<END12>", end[:12], "(coderabbit)", "("+cli+")").Replace(f.form)
}

// plan writes the sample to the plan at path and commits it alone.
func (f *rrFixture) plan(path, sample string) {
	f.t.Helper()
	f.write(path, rrPlanSample(f.t, sample))
	f.commit("update the plan with " + sample)
}

// merge merges the branch side into the current branch and returns the merge
// commit.
func (f *rrFixture) merge(side string) string {
	f.t.Helper()
	f.git("merge", "-q", "--no-ff", "--no-edit", side)
	return f.head()
}

// side makes a commit on a new branch name forked from base, with a change
// to its own file, comes back to feature, and returns the commit.
func (f *rrFixture) side(name, base string) string {
	f.t.Helper()
	f.git("switch", "-q", "-c", name, base)
	f.write(name+".txt", name+"\n")
	f.commit(name)
	commit := f.head()
	f.git("switch", "-q", "feature")
	return commit
}

// mainMerge moves origin/main on by a commit adding docs/<name>.md, merges it
// into feature, and returns the merge commit after checking that it is a
// confirmable merge commit: exactly two parents, the first the branch's head
// before the merge (a branch commit or the merge-base), the second the new
// origin/main.
func (f *rrFixture) mainMerge(name string) string {
	f.t.Helper()
	first := f.head()
	f.git("switch", "-q", "--detach", "refs/remotes/origin/main")
	f.write("docs/"+name+".md", name+"\n")
	f.commit(name)
	next := f.head()
	f.git("switch", "-q", "feature")
	f.git("update-ref", "refs/remotes/origin/main", next)
	merge := f.merge(next)
	parents := strings.Fields(f.git("rev-list", "--parents", "-n", "1", merge))
	branch := strings.Fields(f.git("rev-list", merge, "^refs/remotes/origin/main"))
	base := strings.TrimSpace(f.git("merge-base", merge, "refs/remotes/origin/main"))
	if len(parents) != 3 || parents[1] != first || parents[2] != next ||
		(!slices.Contains(branch, first) && first != base) {
		f.t.Fatalf("the merge of origin/main %v is not a confirmable merge commit, so the case checks nothing", parents)
	}
	return merge
}

// unconfirmableMerge commits onto feature a merge commit of a shape the
// merge check cannot take the range of, and returns it: "octopus" has three
// parents (the branch, origin/main moved on, and the old origin/main, so
// only the count of parents tells it from a confirmable one); "main-first"
// has origin/main moved on as its first parent and the branch as its
// second. The third shape, a second parent origin/main does not reach, is
// what merge and side make.
func (f *rrFixture) unconfirmableMerge(shape string) string {
	f.t.Helper()
	head := f.head()
	old := f.rev("refs/remotes/origin/main")
	next := strings.TrimSpace(f.git("commit-tree", f.rev(old+"^{tree}"), "-p", old, "-m", "main moves on"))
	f.git("update-ref", "refs/remotes/origin/main", next)
	var parents []string
	switch shape {
	case "octopus":
		parents = []string{"-p", head, "-p", next, "-p", old}
	case "main-first":
		parents = []string{"-p", next, "-p", head}
	default:
		f.t.Fatalf("unknown merge shape %q", shape)
	}
	args := append([]string{"commit-tree", f.rev(head + "^{tree}")}, parents...)
	merge := strings.TrimSpace(f.git(append(args, "-m", "merge "+shape)...))
	f.git("reset", "-q", "--hard", merge)
	return merge
}

// partial is the output for the unseen commits ids: PARTIAL with their
// count, then each as "<12-hex-id> <subject>" in the order git rev-list
// --reverse lists the branch.
func (f *rrFixture) partial(ids ...string) []string {
	f.t.Helper()
	out := []string{"PARTIAL " + strconv.Itoa(len(ids))}
	for c := range strings.FieldsSeq(f.git("rev-list", "--reverse", "HEAD", "^refs/remotes/origin/main")) {
		if slices.Contains(ids, c) {
			subject := strings.TrimSpace(f.git("log", "-1", "--format=%s", c))
			out = append(out, c[:12]+" "+subject)
		}
	}
	if len(out) != len(ids)+1 {
		f.t.Fatalf("partial: some of %v are not on the branch", ids)
	}
	return out
}

// rrCase is one coverage or cross-check case: setup builds the branch and
// returns the arguments after the mode; context, when non-nil, supplies the
// _context.md placed on origin/main (empty string omits it); want is the
// expected output, nil when the run must fail with the mode's fail-closed
// output alone and exit 1.
type rrCase struct {
	name    string
	context func(t *testing.T) string
	setup   func(f *rrFixture) []string
	env     func(f *rrFixture) []string
	want    func(f *rrFixture) []string
}

var rrAll = func(*rrFixture) []string { return []string{"ALL"} }

// rrNone returns the arguments for LAST_REVIEWED none, HEAD, and plan when
// it is not "".
func rrNone(f *rrFixture, plan string) []string {
	args := []string{"none", f.head()}
	if plan != "" {
		args = append(args, plan)
	}
	return args
}

// rrSubmodule commits a submodule sub at m0 with the .gitmodules gitmodules,
// records the commit as seen, and commits a move of sub to mb, checking that
// plain git with the fixture's settings shows no change in the move.
func rrSubmodule(f *rrFixture, gitmodules string) {
	f.t.Helper()
	f.write(".gitmodules", gitmodules)
	f.git("add", ".gitmodules")
	f.git("update-index", "--add", "--cacheinfo", "160000,"+f.m0+",sub")
	f.git("commit", "-q", "-m", "add sub")
	f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
	f.git("update-index", "--cacheinfo", "160000,"+f.mb+",sub")
	f.git("commit", "-q", "-m", "move sub")
	if f.git("diff-tree", "--no-commit-id", "-r", "HEAD~1", "HEAD") != "" {
		f.t.Fatal("plain git shows the move of sub, so the case checks nothing")
	}
}

func coverageCases() []rrCase {
	// unseenAfterSeen is a branch whose first commit a seen-range line covers
	// and whose last commit nothing covers.
	unseenAfterSeen := func(f *rrFixture) []string {
		c1 := f.work("c1")
		f.record("record", f.seen(f.mb, c1, "coderabbit"))
		f.work("c2")
		return rrNone(f, "")
	}
	return []rrCase{
		// The commits judged.
		{name: "no commits", setup: func(f *rrFixture) []string { return rrNone(f, "") }, want: rrAll},
		// coverage (and unreviewed) must not depend on origin/main:_context.md.
		{name: "without _context.md", context: func(*testing.T) string { return "" }, setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: rrAll},
		{name: "unseen commit", setup: func(f *rrFixture) []string {
			f.work("c1")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "seen by coderabbit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: rrAll},
		{name: "seen by codex", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "codex"))
			return rrNone(f, "")
		}, want: rrAll},
		{name: "commit after the seen range", setup: unseenAfterSeen, want: func(f *rrFixture) []string {
			return f.partial(f.rev("HEAD"))
		}},
		{name: "LAST_REVIEWED limits the commits judged", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			c2 := f.work("c2")
			f.record("record", f.seen(c1, c2, "coderabbit"))
			return []string{c1, f.head()}
		}, want: rrAll},
		{name: "LAST_REVIEWED is HEAD", setup: func(f *rrFixture) []string {
			f.work("c1")
			return []string{f.head(), f.head()}
		}, want: rrAll},
		// A rewritten history cannot be judged, even when the commits after
		// the fork are seen.
		{name: "LAST_REVIEWED is not an ancestor of HEAD", setup: func(f *rrFixture) []string {
			other := f.side("other", f.mb)
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			return []string{other, f.head()}
		}},
		// HEAD went back behind LAST_REVIEWED: LAST_REVIEWED..HEAD is empty,
		// yet what Codex reviewed is not what is merged.
		{name: "HEAD behind LAST_REVIEWED", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			back := f.head()
			f.work("c2")
			return []string{f.head(), back}
		}},
		{name: "empty commit", setup: func(f *rrFixture) []string {
			f.record("empty")
			return rrNone(f, "")
		}, want: rrAll},
		// A replace ref makes c1 read as an empty commit to plain git.
		{name: "replace ref", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			tree := f.rev(f.mb + "^{tree}")
			empty := strings.TrimSpace(f.git("commit-tree", tree, "-p", f.mb, "-m", "c1"))
			f.git("replace", c1, empty)
			if strings.TrimSpace(f.git("diff-tree", "--no-commit-id", "-r", f.mb, c1)) != "" {
				f.t.Fatal("the replace ref does not hide c1's change from plain git, so the case checks nothing")
			}
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// The branch's .gitmodules asks git to ignore the submodule, which
		// hides a move of its commit from plain git.
		{name: "submodule ignored by .gitmodules", setup: func(f *rrFixture) []string {
			rrSubmodule(f, "[submodule \"sub\"]\n\tpath = sub\n\turl = ./sub\n\tignore = all\n")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "submodule ignored by the configuration", setup: func(f *rrFixture) []string {
			f.git("config", "submodule.sub.ignore", "all")
			rrSubmodule(f, "[submodule \"sub\"]\n\tpath = sub\n\turl = ./sub\n")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "merge commit inside a seen range", setup: func(f *rrFixture) []string {
			f.side("side", f.mb)
			merge := f.merge("side")
			f.record("record", f.seen(f.mb, merge, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		{name: "not run lines do not cancel a seen range", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record",
				"Local CLI review: not run "+f.mb[:12]+".."+c1[:12]+" (coderabbit: other; codex: timeout)",
				f.seen(f.mb, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: rrAll},
		{name: "not run line alone", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", "Local CLI review: not run "+f.mb[:12]+".."+c1[:12]+" (coderabbit: other; codex: timeout)")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		// The checks of a seen-range line.
		{name: "indented and quoted lines", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", "  "+f.seen(f.mb, c1, "coderabbit"), "see "+f.seen(f.mb, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		{name: "text after the form", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit")+" and more")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		{name: "another reviewer", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", strings.Replace(f.seen(f.mb, c1, "coderabbit"), "(coderabbit)", "(fable)", 1))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		{name: "START outside the branch", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.m0, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1")) }},
		// START is not an ancestor of END, yet END..START would cover c1.
		{name: "START not an ancestor of END", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			s1 := f.side("side", f.mb)
			f.merge("side")
			f.record("record", f.seen(s1, c1, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string {
			return f.partial(f.rev("HEAD~2"), f.rev("HEAD~1^2"), f.rev("HEAD~1"))
		}},
		// END is on a side branch that is merged only after the line.
		{name: "END not an ancestor of the line's commit", setup: func(f *rrFixture) []string {
			s1 := f.side("side", f.mb)
			f.record("record", f.seen(f.mb, s1, "coderabbit"))
			f.merge("side")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD^2"), f.rev("HEAD")) }},
		// A branch whose name is c1's abbreviation points at origin/main;
		// resolving the name would make the line cover c1 and c2.
		{name: "abbreviation that is also a branch name", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			c2 := f.work("c2")
			f.git("branch", c1[:12], f.mb)
			f.record("record", f.seen(c1, c2, "coderabbit"))
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~2"), f.rev("HEAD~1")) }},
		// A line on origin/main is not read; the END checks back this up,
		// since a commit origin/main reaches cannot have a branch commit as
		// an ancestor.
		{name: "line on origin/main", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			tree := f.rev(f.mb + "^{tree}")
			onMain := strings.TrimSpace(f.git("commit-tree", tree, "-p", f.mb, "-m", "main record\n\n"+f.seen(f.mb, c1, "coderabbit")))
			f.git("update-ref", "refs/remotes/origin/main", onMain)
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// The shape of PR #987: a merge of origin/main moves the merge-base
		// past the line's START. coverage keeps its START check, so the line
		// is not read and c1 is unseen, as is the merge commit.
		{name: "seen range from the old merge-base before a merge", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			f.mainMerge("m2")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~2"), f.rev("HEAD")) }},
		// The plan checkbox commits.
		{name: "green gate confirmed in a marker section", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-greengate.md")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		{name: "green gate text reworded", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-greengate-reworded.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "PR created in a marker section", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created.md")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		// runpr.md marks these two boxes in its pre-merge commit, together
		// with the checklist chapter's line, after the escalation has run;
		// the case checks the marking rule, and that commit stays unseen.
		{name: "PR merged and next branch in a marker section", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created.md")
			f.plan(rrMarkersPlan, "markers-merged.md")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		{name: "next branch not applicable", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-skipped.md")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		{name: "single-PR record added", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded.md")
			return rrNone(f, rrSinglePlan)
		}, want: rrAll},
		{name: "single-PR record merged", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded.md")
			f.plan(rrSinglePlan, "single-merged.md")
			return rrNone(f, rrSinglePlan)
		}, want: rrAll},
		{name: "PLAN not passed", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created.md")
			return rrNone(f, "")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "PLAN names another task's plan", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created.md")
			return rrNone(f, "docs/tasks/0009_missing/03_implementation_plan.md")
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox in a code fence", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-fenced.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox in the checklist chapter of a plan with markers", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-section6.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox in another chapter", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-other-chapter.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox text reworded", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-reworded.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "next branch text reworded", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-next-reworded.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "list prefix changed", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-prefix-changed.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record with a URL of another site", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-bad-url.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record with another second line", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-wrong-second.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record of three lines", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-three.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record and another line", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-and-more.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "URL of another site", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-bad-url.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// A plan with CRLF line ends is read like one with LF ends, as the
		// Opus review's decision script reads it.
		{name: "CRLF plan", setup: func(f *rrFixture) []string {
			crlf := func(sample string) string {
				return strings.ReplaceAll(rrPlanSample(f.t, sample), "\n", "\r\n")
			}
			f.write(rrMarkersPlan, crlf("markers.md"))
			f.commit("CRLF plan")
			f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
			f.write(rrMarkersPlan, crlf("markers-created.md"))
			f.commit("CRLF checkbox")
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		// A checkbox commit that also turns every line end into CRLF.
		{name: "checkbox and line ends", setup: func(f *rrFixture) []string {
			f.write(rrMarkersPlan, strings.ReplaceAll(rrPlanSample(f.t, "markers-created.md"), "\n", "\r\n"))
			f.commit("checkbox and CRLF")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// The lines read the same with or without the final newline, so no
		// checkbox changes: a change must mark one.
		{name: "final newline only", setup: func(f *rrFixture) []string {
			f.write(rrMarkersPlan, strings.TrimSuffix(rrPlanSample(f.t, "markers.md"), "\n"))
			f.commit("drop the final newline")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox and another line", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-created-other.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "checkbox and another file", setup: func(f *rrFixture) []string {
			f.write(rrMarkersPlan, rrPlanSample(f.t, "markers-created.md"))
			f.write(rrSource, "m0\nchanged\n")
			f.commit("checkbox and source")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "unclosed fence", setup: func(f *rrFixture) []string {
			f.plan(rrMarkersPlan, "markers-unclosed.md")
			f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
			f.plan(rrMarkersPlan, "markers-unclosed-created.md")
			return rrNone(f, rrMarkersPlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record in another chapter", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-section7.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "single-PR record unchecked", setup: func(f *rrFixture) []string {
			f.plan(rrSinglePlan, "single-recorded-unchecked.md")
			return rrNone(f, rrSinglePlan)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// A plan the branch marks binary, or gives a diff driver, is read the
		// same: the script reads blobs, not diffs.
		{name: "attributes of the branch", setup: func(f *rrFixture) []string {
			f.write(".gitattributes", "*.md binary\n")
			f.commit("attributes")
			f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
			f.plan(rrMarkersPlan, "markers-created.md")
			if !strings.Contains(f.git("diff", "HEAD~1", "HEAD"), "Binary") {
				f.t.Fatal("the attributes do not make plain git show the plan as binary, so the case checks nothing")
			}
			return rrNone(f, rrMarkersPlan)
		}, want: rrAll},
		// The inputs and the failures.
		{name: "LAST_REVIEWED of another form", setup: func(f *rrFixture) []string {
			return []string{"HEAD", f.head()}
		}},
		{name: "HEAD of another form", setup: func(*rrFixture) []string {
			return []string{"none", "HEAD"}
		}},
		{name: "HEAD abbreviated", setup: func(f *rrFixture) []string {
			return []string{"none", f.head()[:12]}
		}},
		{name: "HEAD names no commit", setup: func(*rrFixture) []string {
			return []string{"none", strings.Repeat("0", 40)}
		}},
		{name: "LAST_REVIEWED names no commit", setup: func(f *rrFixture) []string {
			return []string{strings.Repeat("0", 40), f.head()}
		}},
		{name: "PLAN of another form", setup: func(f *rrFixture) []string {
			return rrNone(f, "../"+rrMarkersPlan)
		}},
		{name: "too many arguments", setup: func(f *rrFixture) []string {
			return append(rrNone(f, rrMarkersPlan), "extra")
		}},
		{name: "too few arguments", setup: func(*rrFixture) []string { return []string{"none"} }},
		{name: "no origin/main", setup: func(f *rrFixture) []string {
			f.git("update-ref", "-d", "refs/remotes/origin/main")
			return rrNone(f, "")
		}},
		{name: "origin/main names a tree", setup: func(f *rrFixture) []string {
			f.git("update-ref", "refs/remotes/origin/main", f.rev(f.mb+"^{tree}"))
			return rrNone(f, "")
		}},
		// The environment. GIT_DIR and GIT_WORK_TREE name a second fixture,
		// which has no HEAD commit of this one, so a script that honored
		// them would fail.
		{name: "GIT_ variables", setup: unseenAfterSeen, env: func(f *rrFixture) []string {
			other := newRRFixture(f.t, f.form)
			return []string{"GIT_DIR=" + filepath.Join(other.repo, ".git"), "GIT_WORK_TREE=" + other.repo}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		// A function named command, which only env -i drops, would stop every
		// git the script runs.
		{name: "clean environment variable preset", setup: unseenAfterSeen, env: func(*rrFixture) []string {
			return []string{
				"REVIEW_RECORDS_CLEAN_ENV=1", "BASH_FUNC_command%%=() { return 0; }",
			}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "exported functions", setup: unseenAfterSeen, env: func(*rrFixture) []string {
			return []string{
				"BASH_FUNC_git%%=() { return 0; }",
				"BASH_FUNC_awk%%=() { return 0; }",
				"BASH_FUNC_printf%%=() { builtin printf 'ALL\\n'; }",
			}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
		{name: "BASH_ENV", setup: unseenAfterSeen, env: func(f *rrFixture) []string {
			bashEnv := filepath.Join(f.home, "bash-env")
			// command is not one of the functions the script unsets, so
			// only env -i (which drops BASH_ENV) keeps it out.
			fake := "command() { return 0; }\n"
			if err := os.WriteFile(bashEnv, []byte(fake), 0o600); err != nil {
				f.t.Fatalf("write BASH_ENV: %v", err)
			}
			return []string{"BASH_ENV=" + bashEnv}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD")) }},
	}
}

// rrConfigScenario is a git setting a user could have that changes what
// plain git prints, with the plain git command whose output it changes.
type rrConfigScenario struct {
	name   string
	config [2]string
	plain  []string
}

func rrConfigScenarios() []rrConfigScenario {
	diffLast := []string{"diff", "HEAD~1", "HEAD"}
	return []rrConfigScenario{
		{name: "diff.noprefix", config: [2]string{"diff.noprefix", "true"}, plain: diffLast},
		{name: "color.diff", config: [2]string{"color.diff", "always"}, plain: diffLast},
		{name: "diff.external", config: [2]string{"diff.external", "true"}, plain: diffLast},
		{name: "core.abbrev", config: [2]string{"core.abbrev", "20"}, plain: []string{"log", "-1", "--format=%h"}},
		{name: "diff.relative", config: [2]string{"diff.relative", "true"}, plain: []string{"-C", "docs", "diff", "--name-only", "HEAD~4", "HEAD~3"}},
		{name: "diff driver", config: [2]string{"diff.upper.textconv", "tr a-z A-Z <"}, plain: []string{"diff", "HEAD~4", "HEAD~3"}},
		{name: "i18n.logOutputEncoding", config: [2]string{"i18n.logOutputEncoding", "ISO-8859-1"}, plain: []string{"log", "-1", "--format=%s"}},
		{name: "core.quotePath", config: [2]string{"core.quotePath", "false"}, plain: []string{"diff", "--name-only", "HEAD~1", "HEAD"}},
	}
}

// rrConfigBranch builds the branch the configuration scenarios run on: a
// plan checkbox commit, a seen commit, and an unseen commit whose file name
// and subject are not ASCII (so core.quotePath and i18n.logOutputEncoding
// change plain git's output).
func rrConfigBranch(f *rrFixture) []string {
	// The attribute names a driver only the diff driver scenario defines.
	f.write(".gitattributes", "*.md diff=upper\n")
	f.commit("attributes")
	f.record("record", f.seen(f.mb, f.head(), "coderabbit"))
	f.plan(rrMarkersPlan, "markers-created.md")
	c := f.work("c1")
	f.record("record", f.seen(f.mb, c, "coderabbit"))
	f.write("caf\u00e9.txt", "x\n")
	f.commit("unseen caf\u00e9")
	return rrNone(f, rrMarkersPlan)
}

// checkConfigScenarios runs mode on the branch build makes, in a fixture
// newFixture returns, without and with each git configuration scenario in
// each scope, and checks that the setting changes plain git's output but not
// the script's. base is the start of the output without the setting; a
// branch on which the script prints another output fails the scenario, so a
// branch that no longer exercises the reading does not pass with nothing
// compared.
func checkConfigScenarios(t *testing.T, name string, bash scriptBash, newFixture func(*testing.T) *rrFixture,
	mode string, build func(*rrFixture) []string, base func(*rrFixture) string,
) {
	t.Helper()
	for _, scope := range []string{"repository", "global"} {
		for _, s := range rrConfigScenarios() {
			t.Run(name+"/"+scope+"/"+s.name, func(t *testing.T) {
				t.Parallel()
				f := newFixture(t)
				args := append([]string{mode}, build(f)...)
				want := f.runScript(bash, reviewRecordsPath, args, scriptRunOptions{})
				if prefix := base(f); want.code != 0 || !strings.HasPrefix(want.stdout, prefix) {
					t.Fatalf("without %s the script did not print %q, so the scenario checks nothing: %v", s.name, prefix, want)
				}
				before := f.git(s.plain...)
				file := filepath.Join(f.repo, ".git", "config")
				if scope == "global" {
					file = filepath.Join(f.home, ".gitconfig")
				}
				f.git("config", "--file", file, s.config[0], s.config[1])
				if after := f.git(s.plain...); after == before {
					t.Fatalf("%s does not change plain git's output %q, so the scenario checks nothing", s.name, before)
				}
				if got := f.runScript(bash, reviewRecordsPath, args, scriptRunOptions{}); got != want {
					t.Errorf("with %s set the script printed %v, without it %v", s.name, got, want)
				}
			})
		}
	}
}

// runRRCases executes every case in cases under mode and bash, checking
// fail-closed output failOut (exit 1 with diagnostic) when want is nil and
// exact stdout (exit 0) otherwise.
func runRRCases(t *testing.T, bash scriptBash, mode, failOut, form string, forms fableForms, cases []rrCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var f *rrFixture
			if c.context != nil {
				f = newRRFixtureWithContext(t, form, c.context(t))
			} else {
				f = newRRFixture(t, form)
			}
			f.forms, f.bash = forms, bash
			args := append([]string{mode}, c.setup(f)...)
			var opts scriptRunOptions
			if c.env != nil {
				opts.extraEnv = c.env(f)
			}
			run := f.runScript(bash, reviewRecordsPath, args, opts)
			if c.want == nil {
				if run.code != 1 || run.stdout != failOut || run.stderr == "" {
					t.Errorf("got %v, want exit 1, output %q alone, and a diagnostic", run, failOut)
				}
				return
			}
			want := strings.Join(c.want(f), "\n") + "\n"
			if run.code != 0 || run.stdout != want {
				t.Errorf("got %v, want output %q and exit 0", run, want)
			}
		})
	}
}

// TestReviewRecordsCoverage runs the coverage mode of the review records
// script on fixture repositories: the commits judged, the checks of a
// seen-range line, the plan checkbox commits, the failures, and that the
// caller's environment and git configuration change nothing. Each case runs
// with each bash scriptBashes returns.
//
// Some guards back each other, so breaking one alone fails no case:
// unsetting the GIT_ variables (env -i drops them too), reading lines only
// from origin/main..HEAD (END must be a branch commit and an ancestor of
// the line's commit), the check that END is a branch commit (a range whose
// END origin/main reaches covers no branch commit), resolving a prefix only
// as an ID (the ref check rejects a prefix a ref is named after), the check
// that PLAN is given (the changed path must equal it), and the check that
// END is not the line's own commit (a message cannot name its own commit).
//
// Most configuration scenarios and the attributes case guard the reading
// through plumbing commands, which no such setting reaches today: they fail
// only when the script starts reading through a porcelain diff. Only
// core.abbrev and i18n.logOutputEncoding reach a command the script runs
// (git log for the listed subjects).
func TestReviewRecordsCoverage(t *testing.T) {
	for _, tool := range []string{"git", "awk", "tr"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("%s is not on PATH: %v", tool, err)
		}
	}
	form := seenRangeForm(t)
	for _, bash := range scriptBashes(t) {
		t.Run(bash.name, func(t *testing.T) {
			t.Parallel()
			runRRCases(t, bash, "coverage", "PARTIAL\n", form, fableForms{}, coverageCases())
			checkConfigScenarios(t, "git config", bash, func(t *testing.T) *rrFixture { return newRRFixture(t, form) },
				"coverage", rrConfigBranch, func(*rrFixture) string { return "PARTIAL 1\n" })
		})
	}
}

// unreviewedOverrides are the outputs of the coverage cases that the two
// differences of unreviewed change.
var unreviewedOverrides = map[string]func(*rrFixture) []string{
	// A merge commit inside a seen range is seen.
	"merge commit inside a seen range": rrAll,
	// The widened START check reads the line, so only the merge is unseen
	// (the shape of PR #987).
	"seen range from the old merge-base before a merge": func(f *rrFixture) []string {
		return f.partial(f.rev("HEAD"))
	},
}

// unreviewedCases are the cases of the unreviewed mode beyond the coverage
// cases it reuses. Their setups return the arguments of unreviewed.
func unreviewedCases() []rrCase {
	head := func(f *rrFixture) []string { return []string{f.head()} }
	return []rrCase{
		// A line from the old merge-base whose END is after the merge holds
		// main's commits, so it is not read.
		{name: "seen range from the old merge-base ending after a merge", setup: func(f *rrFixture) []string {
			f.work("c1")
			merge := f.mainMerge("m2")
			f.record("record", f.seen(f.mb, merge, "coderabbit"))
			return head(f)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1^1"), f.rev("HEAD~1")) }},
		// The checkpoint mark, which makes the fable mode cover what follows
		// it, does not make unreviewed count the merge and the fix after it
		// as seen; the mark itself is an empty commit.
		{name: "merge after the checkpoint mark", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			f.record("mark", f.forms.checkpoint)
			f.mainMerge("m2")
			f.work("checks fix")
			return head(f)
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1"), f.rev("HEAD")) }},
		// The standalone path's fix, which its local CLI review saw.
		{name: "fix seen by the standalone path's review", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			r := f.head()
			fix := f.work("fable fix")
			f.record("record", f.seen(r, fix, "codex"))
			return head(f)
		}, want: rrAll},
		// A commit that changes README.en.md alone and a CHECKS fix are
		// listed when no CLI saw them.
		{name: "README.en.md and a CHECKS fix after the record", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.seen(f.mb, c1, "coderabbit"))
			f.readme("one\n")
			f.work("checks fix")
			return []string{f.head(), rrMarkersPlan}
		}, want: func(f *rrFixture) []string { return f.partial(f.rev("HEAD~1"), f.rev("HEAD")) }},
	}
}

// rrUnreviewedConfigBranch builds a branch on which the unreviewed mode
// lists two commits only through the widened START check: a seen commit
// and a plan checkbox commit, a merge of origin/main, a seen-range line from
// the old merge-base over the first two, and a last commit whose file name
// and subject are not ASCII. HEAD~4..HEAD~3 is the checkbox commit and
// HEAD~1..HEAD the last commit, so the configuration scenarios' plain git
// commands change their output.
func rrUnreviewedConfigBranch(f *rrFixture) []string {
	f.write(".gitattributes", "*.md diff=upper\n")
	f.commit("attributes")
	f.plan(rrMarkersPlan, "markers-created.md")
	checkbox := f.head()
	f.mainMerge("m2")
	f.record("record", f.seen(f.mb, checkbox, "coderabbit"))
	f.write("caf\u00e9.txt", "x\n")
	f.commit("unseen caf\u00e9")
	return []string{f.head(), rrMarkersPlan}
}

// TestReviewRecordsUnreviewed runs the unreviewed mode of the review records
// script. It reuses the coverage cases that judge origin/main..HEAD
// (LAST_REVIEWED none), with that argument dropped, so the rules and
// failures the two modes share are checked on the same branches; the cases
// of LAST_REVIEWED..HEAD do not apply. unreviewedOverrides gives the outputs
// the two differences change, and unreviewedCases adds the cases of those
// differences. Each case runs with each bash scriptBashes returns, and the
// configuration scenarios run on rrUnreviewedConfigBranch.
func TestReviewRecordsUnreviewed(t *testing.T) {
	form := seenRangeForm(t)
	forms := readFableForms(t)
	var cases []rrCase
	used := map[string]struct{}{}
	for _, c := range coverageCases() {
		if want, ok := unreviewedOverrides[c.name]; ok {
			c.want = want
			used[c.name] = struct{}{}
		}
		setup := c.setup
		c.setup = func(f *rrFixture) []string {
			args := setup(f)
			if len(args) == 0 || args[0] != "none" {
				f.t.Skip("the case judges LAST_REVIEWED..HEAD, which unreviewed does not take")
			}
			return args[1:]
		}
		cases = append(cases, c)
	}
	for name := range unreviewedOverrides {
		if _, ok := used[name]; !ok {
			t.Fatalf("unreviewedOverrides names %q, which is no coverage case", name)
		}
	}
	cases = append(cases, unreviewedCases()...)
	fixture := func(t *testing.T) *rrFixture {
		f := newRRFixture(t, form)
		f.forms = forms
		return f
	}
	for _, bash := range scriptBashes(t) {
		t.Run(bash.name, func(t *testing.T) {
			t.Parallel()
			runRRCases(t, bash, "unreviewed", "PARTIAL\n", form, forms, cases)
			checkConfigScenarios(t, "git config", bash, fixture, "unreviewed", rrUnreviewedConfigBranch,
				func(*rrFixture) string { return "PARTIAL 2\n" })
		})
	}
}

// TestReviewRecordsMode checks that a mode other than coverage, unreviewed,
// cross-check, and its compatibility alias fable fails with no output, so a
// caller reads it as the failure of its mode.
func TestReviewRecordsMode(t *testing.T) {
	for _, bash := range scriptBashes(t) {
		for _, mode := range [][]string{nil, {"Coverage"}, {"Unreviewed"}, {"Cross-check"}, {"Fable"}} {
			f := newRRFixture(t, "")
			run := f.runScript(bash, reviewRecordsPath, mode, scriptRunOptions{})
			if run.code != 1 || run.stdout != "" || run.stderr == "" {
				t.Errorf("%s with mode %q: got %v, want exit 1, no output, and a diagnostic", bash.name, mode, run)
			}
		}
	}
}

// fableForms are the record line forms the fable mode reads, from the
// documents that define them: the Fable records, the checkpoint mark, and
// the merge check records of fable-review.md's "Record lines", and the
// not-run line of local-cli-review.md step 7.
type fableForms struct {
	run, notRun, checkpoint, localNotRun, mergeRun, mergeNotRun string
}

func readFableForms(t *testing.T) fableForms {
	t.Helper()
	records := requireSection(t, "fable-review.md", doctestutil.ReadDoc(t, fableReviewPath), "## Record lines")
	where := "fable-review.md \"Record lines\""
	return fableForms{
		run: recordTableForm(t, where, records, "Fable review: run",
			"<START12>", "<END12>", "<model-id>", "<seconds>", "<tokens>"),
		notRun: recordTableForm(t, where, records, "Fable review: not run",
			"<START12>", "<END12>", "<reason>", "[; <seconds>s, tokens <tokens>]"),
		checkpoint: recordTableForm(t, where, records, "Fable review: checkpoint"),
		localNotRun: recordTableForm(t, "local-cli-review.md step 7", localReviewStep7(t), "Local CLI review: not run",
			"<START12>", "<END12>"),
		mergeRun: recordTableForm(t, where, records, "Merge check: run",
			"<MERGE>", "<model-id>", "<seconds>", "<tokens>"),
		mergeNotRun: recordTableForm(t, where, records, "Merge check: not run",
			"<MERGE>", "<reason>", "[; <seconds>s, tokens <tokens>]"),
	}
}

// mergeRun is a merge check run record naming the commit merge with the
// default Claude Code high model, every count and the cost filled in.
func (f *rrFixture) mergeRun(merge string) string {
	return f.mergeRunModel(merge, "claude-opus-example")
}

// mergeRunModel is a merge check run record naming merge with model.
func (f *rrFixture) mergeRunModel(merge, model string) string {
	return strings.NewReplacer("<MERGE>", merge, "<model-id>", model, "<n>", "0", "<seconds>", "90", "<tokens>", "1234").
		Replace(f.forms.mergeRun)
}

// mergeNotRun is a merge check not-run record naming the commit merge with
// reason, without the cost.
func (f *rrFixture) mergeNotRun(merge, reason string) string {
	return strings.NewReplacer("<MERGE>", merge, "<reason>", reason, "[; <seconds>s, tokens <tokens>]", "").
		Replace(f.forms.mergeNotRun)
}

// rrRange replaces the range placeholders of form with start and end.
func rrRange(form, start, end string) string {
	return strings.NewReplacer("<START12>", start[:12], "<END12>", end[:12]).Replace(form)
}

// fableRun is a legacy Fable run record for start..end, every count and the
// cost filled in.
func (f *rrFixture) fableRun(start, end string) string {
	return f.fableRunModel(start, end, "claude-fable-example")
}

// fableRunModel is a legacy Fable run record for start..end with model.
func (f *rrFixture) fableRunModel(start, end, model string) string {
	return strings.NewReplacer("<model-id>", model, "<n>", "1", "<seconds>", "120", "<tokens>", "4567").
		Replace(rrRange(f.forms.run, start, end))
}

// crossCheckRun is a new Cross-check run record for start..end with model.
func (f *rrFixture) crossCheckRun(start, end, model string) string {
	return strings.Replace(f.fableRunModel(start, end, model), "Fable review:", "Cross-check review:", 1)
}

// fableNotRun is a legacy Fable not-run record for start..end with reason,
// with the cost when withCost is true and without it otherwise.
func (f *rrFixture) fableNotRun(start, end, reason string, withCost bool) string {
	line := strings.Replace(rrRange(f.forms.notRun, start, end), "<reason>", reason, 1)
	if withCost {
		line = strings.NewReplacer("[", "", "]", "", "<seconds>", "75", "<tokens>", "unknown").Replace(line)
	} else {
		line = strings.Replace(line, "[; <seconds>s, tokens <tokens>]", "", 1)
	}
	return line
}

// crossCheckNotRun is a new Cross-check not-run record for start..end.
func (f *rrFixture) crossCheckNotRun(start, end, reason string, withCost bool) string {
	return strings.Replace(f.fableNotRun(start, end, reason, withCost), "Fable review:", "Cross-check review:", 1)
}

// crossCheckCheckpoint is the new Cross-check checkpoint mark line.
func (f *rrFixture) crossCheckCheckpoint() string {
	return strings.Replace(f.forms.checkpoint, "Fable review:", "Cross-check review:", 1)
}

// localNotRun is a local CLI review not-run line for start..end.
func (f *rrFixture) localNotRun(start, end string) string {
	return strings.ReplaceAll(rrRange(f.forms.localNotRun, start, end), "<kind>", "other")
}

// readme commits a change to README.en.md alone.
func (f *rrFixture) readme(content string) {
	f.t.Helper()
	f.write("README.en.md", content)
	f.commit("sync README.en.md")
}

// rrFable returns the fable mode's arguments for entry, HEAD, and the
// fixture's plan with PR markers.
func rrFable(f *rrFixture, entry string) []string {
	return []string{entry, f.head(), rrMarkersPlan}
}

// rrValid is the output of a valid record.
var rrValid = func(*rrFixture) []string { return []string{"VALID"} }

// rrMissing returns the output MISSING from the commit name resolves to.
func rrMissing(name string) func(f *rrFixture) []string {
	return func(f *rrFixture) []string { return []string{"MISSING " + f.rev(name)} }
}

// crossCheckCases are the cross-check mode's cases; a case whose setup does
// not pass an entry runs at launch.
func crossCheckCases() []rrCase {
	launch := func(f *rrFixture) []string { return rrFable(f, "launch") }
	// recorded adds one work commit c1 and a run record of merge-base..c1
	// in the empty commit after it.
	recorded := func(f *rrFixture) {
		c1 := f.work("c1")
		f.record("record", f.fableRun(f.mb, c1))
	}
	// unseenAfterRecord adds a work commit after a valid record.
	unseenAfterRecord := func(f *rrFixture) []string {
		recorded(f)
		f.work("c2")
		return launch(f)
	}
	return []rrCase{
		// The record (both new Cross-check and legacy Fable prefixes, and
		// both Claude Code and Antigravity cross-check model IDs).
		{name: "no commits", setup: launch, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "no record", setup: func(f *rrFixture) []string {
			f.work("c1")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "legacy Fable run record", setup: func(f *rrFixture) []string {
			recorded(f)
			return launch(f)
		}, want: rrValid},
		{name: "new Cross-check run record with Antigravity model", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.crossCheckRun(f.mb, c1, "claude-opus-5-5-medium"))
			return launch(f)
		}, want: rrValid},
		{name: "not run record with its cost", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.fableNotRun(f.mb, c1, "timeout", true))
			return launch(f)
		}, want: rrValid},
		{name: "not run record without a cost", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.fableNotRun(f.mb, c1, "agent-cannot-call", false))
			return launch(f)
		}, want: rrValid},
		{name: "new Cross-check not run with wrong-model", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.crossCheckNotRun(f.mb, c1, "wrong-model", true))
			return launch(f)
		}, want: rrValid},
		{name: "new Cross-check not run with legacy not-fable", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.crossCheckNotRun(f.mb, c1, "not-fable", false))
			return launch(f)
		}, want: rrValid},
		{name: "not run record of another reason", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.fableNotRun(f.mb, c1, "busy", false))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "work after the record", setup: unseenAfterRecord, want: rrMissing("HEAD~1")},
		{name: "indented and quoted records", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", "  "+f.fableRun(f.mb, c1), "see "+f.fableRun(f.mb, c1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "checkpoint mark alone", setup: func(f *rrFixture) []string {
			f.work("c1")
			f.record("mark", f.forms.checkpoint)
			return rrFable(f, "checkpoint")
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// A record whose range fails the checks is ignored. START m0 is a
		// commit origin/main reaches, but the range holds mb, which
		// origin/main reaches too, so the widened START check fails as well.
		{name: "record with START outside the branch", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.fableRun(f.m0, c1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "record naming its own commit's descendant", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			c2 := f.work("c2")
			f.git("reset", "-q", "--hard", c1)
			f.record("record", f.fableRun(f.mb, c2))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// The last record decides, in either order.
		{name: "last of two records valid", setup: func(f *rrFixture) []string {
			recorded(f)
			r1 := f.head()
			c2 := f.work("c2")
			f.record("record", f.fableRun(r1, c2))
			return launch(f)
		}, want: rrValid},
		{name: "last of two records invalid", setup: func(f *rrFixture) []string {
			recorded(f)
			r1 := f.head()
			c2 := f.work("c2")
			f.record("record", f.fableRun(r1, c2))
			f.work("c3")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "last record covers less than an earlier one", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			c2 := f.work("c2")
			f.record("record", f.fableRun(f.mb, c2))
			f.record("record", f.fableRun(f.mb, c1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.rev("HEAD~3")} }},
		// The fixes of the same run.
		{name: "fix inside a seen range of the record's commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			fix := f.work("fix")
			f.record("record", f.fableRun(f.mb, c1), f.seen(f.mb, fix, "coderabbit"))
			return launch(f)
		}, want: rrValid},
		{name: "fix inside a not-run line of the record's commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			fix := f.work("fix")
			f.record("record", f.fableRun(f.mb, c1), f.localNotRun(c1, fix))
			return launch(f)
		}, want: rrValid},
		{name: "fix outside every range of the record's commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.work("fix")
			f.record("record", f.fableRun(f.mb, c1), f.seen(f.mb, c1, "coderabbit"))
			return launch(f)
		}, want: rrMissing("HEAD~2")},
		{name: "fix inside a range of a later commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			fix := f.work("fix")
			f.record("record", f.fableRun(f.mb, c1))
			f.record("record", f.seen(f.mb, fix, "coderabbit"))
			return launch(f)
		}, want: rrMissing("HEAD~3")},
		// The side commit is inside the record's line by its IDs, and the
		// merge of the side branch is unconfirmable, so the record is not
		// valid whether or not the line counts; the case pins START, the
		// merge-base because a merge commit follows END. The check that a
		// line's END is an ancestor of its commit is held by "record naming
		// its own commit's descendant" and coverage's "END not an ancestor of
		// the line's commit".
		{name: "range of the record's commit beyond it", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			s1 := f.side("side", c1)
			f.record("record", f.fableRun(f.mb, c1), f.seen(c1, s1, "coderabbit"))
			f.merge("side")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// The record starts inside the branch, so it stays valid after
		// origin/main moves to m3. x, the first commit it misses, has the
		// parent m2, which origin/main reaches behind the merge-base m3.
		// START is the merge-base because merge commits follow END; the
		// fallback for a parent outside the branch is no longer reached,
		// since without a merge commit after END that parent is END or a
		// commit after it.
		{name: "parent of the first missing commit behind the merge-base", setup: func(f *rrFixture) []string {
			c0 := f.work("c0")
			c1 := f.work("c1")
			f.record("record", f.fableRun(c0, c1))
			tree := f.rev(f.mb + "^{tree}")
			m2 := strings.TrimSpace(f.git("commit-tree", tree, "-p", f.mb, "-m", "m2"))
			m3 := strings.TrimSpace(f.git("commit-tree", tree, "-p", m2, "-m", "m3"))
			f.git("update-ref", "refs/remotes/origin/main", m3)
			f.side("x", m2)
			f.merge("x")
			f.git("merge", "-q", "--no-ff", "--no-edit", m3)
			if f.rev("x^1") != m2 || strings.TrimSpace(f.git("merge-base", "HEAD", m3)) != m3 {
				f.t.Fatal("x's parent is not m2 or the merge-base is not m3, so the case checks nothing")
			}
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// A merge of origin/main moves the merge-base past a record that
		// starts at the old one (the shape of PR #987). The record passes the
		// widened START check, so it is read, and the merge waits for the
		// merge check.
		{name: "record from a merge-base origin/main moved past", setup: func(f *rrFixture) []string {
			recorded(f)
			f.mainMerge("m2")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.head()} }},
		// The commits without content. These stay covered at the pre-push
		// entry too.
		{name: "README.en.md after the record", setup: func(f *rrFixture) []string {
			f.readme("one\n")
			recorded(f)
			f.readme("two\n")
			return launch(f)
		}, want: rrValid},
		{name: "README.en.md created after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			f.readme("one\n")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "README.en.md and another file", setup: func(f *rrFixture) []string {
			f.readme("one\n")
			recorded(f)
			f.write("README.en.md", "two\n")
			f.write(rrSource, "changed\n")
			f.commit("README.en.md and source")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "plan checkbox after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			f.plan(rrMarkersPlan, "markers-created.md")
			return launch(f)
		}, want: rrValid},
		{name: "plan checkbox and README.en.md together", setup: func(f *rrFixture) []string {
			f.readme("one\n")
			recorded(f)
			f.write(rrMarkersPlan, rrPlanSample(f.t, "markers-created.md"))
			f.write("README.en.md", "two\n")
			f.commit("checkbox and README.en.md")
			return launch(f)
		}, want: rrValid},
		{name: "another line of the plan", setup: func(f *rrFixture) []string {
			recorded(f)
			f.plan(rrMarkersPlan, "markers-created-other.md")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "checkbox of another task's plan", setup: func(f *rrFixture) []string {
			recorded(f)
			f.plan(rrSinglePlan, "single-recorded.md")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "plan absent", setup: func(f *rrFixture) []string {
			f.readme("one\n")
			recorded(f)
			f.readme("two\n")
			return []string{"launch", f.head(), "docs/tasks/0009_missing/03_implementation_plan.md"}
		}, want: rrValid},
		// The merge brings in an empty commit, so it changes nothing against
		// its first parent. Its second parent is a side branch origin/main
		// does not reach, so it is an unconfirmable merge commit: it
		// invalidates the record even though a merge check record names it,
		// and START is the merge-base because a merge commit follows END.
		{name: "merge after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			f.git("switch", "-q", "-c", "side")
			f.record("empty side")
			f.git("switch", "-q", "feature")
			merge := f.merge("side")
			f.record("check", f.mergeRun(merge))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "attributes of the branch", setup: func(f *rrFixture) []string {
			f.write(".gitattributes", "*.md binary\n")
			f.commit("attributes")
			recorded(f)
			f.plan(rrMarkersPlan, "markers-created.md")
			if !strings.Contains(f.git("diff", "HEAD~1", "HEAD"), "Binary") {
				f.t.Fatal("the attributes do not make plain git show the plan as binary, so the case checks nothing")
			}
			return launch(f)
		}, want: rrValid},
		{name: "run record of another form", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", strings.Replace(f.fableRun(f.mb, c1), ", invalid 1", "", 1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "run record of another model", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", strings.Replace(f.fableRun(f.mb, c1), "claude-fable-example", "claude-opus-example", 1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// The checkpoint mark.
		// The last mark decides: the fix between the two marks is after the
		// first one only.
		{name: "work between two marks", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.work("gate fix")
			f.record("mark", f.forms.checkpoint)
			f.work("later fix")
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~3")},
		// The shape of a task-document PR's CHECKS fix after its mark.
		{name: "work after the legacy mark at the checkpoint", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.work("gate fix")
			return rrFable(f, "checkpoint")
		}, want: rrValid},
		{name: "work after the Cross-check mark at the checkpoint", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.crossCheckRun(f.mb, c1, "claude-opus-5-5-medium"))
			f.record("mark", f.crossCheckCheckpoint())
			f.work("gate fix")
			return rrFable(f, "checkpoint")
		}, want: rrValid},
		{name: "work after the mark at launch", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.work("gate fix")
			return launch(f)
		}, want: rrMissing("HEAD~1")},
		{name: "work before the mark", setup: func(f *rrFixture) []string {
			recorded(f)
			f.work("phase group")
			f.record("mark", f.forms.checkpoint)
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~2")},
		{name: "implementation record after the mark", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.work("phase group")
			f.record("record", "Implementation model: "+f.mb[:12]+".."+f.head()[:12]+" claude-opus-example")
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~2")},
		{name: "mark in a commit with content", setup: func(f *rrFixture) []string {
			recorded(f)
			f.write(rrSource, "marked\n")
			f.commit("mark\n\n" + f.forms.checkpoint)
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~1")},
		// A commit with content before the mark invalidates the record even
		// when the CHECKS fix after the mark does not.
		{name: "work before the mark and a fix after it", setup: func(f *rrFixture) []string {
			recorded(f)
			f.work("manual work")
			f.record("mark", f.forms.checkpoint)
			f.work("checks fix")
			return rrFable(f, "checkpoint")
		}, want: rrMissing("HEAD~3")},
		{name: "merge after the mark", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			f.mainMerge("m2")
			return rrFable(f, "checkpoint")
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.head()} }},
		// The fix of the merge check after the mark is covered by the mark,
		// whatever its seen ranges say.
		{name: "merge after the mark and an unseen fix", setup: func(f *rrFixture) []string {
			recorded(f)
			f.record("mark", f.forms.checkpoint)
			merge := f.mainMerge("m2")
			f.work("merge fix")
			f.record("check", f.mergeRun(merge))
			return rrFable(f, "checkpoint")
		}, want: rrValid},
		// The merge commits after the record.
		{name: "merge after a record from a branch commit", setup: func(f *rrFixture) []string {
			c0 := f.work("c0")
			c1 := f.work("c1")
			f.record("record", f.fableRun(c0, c1))
			f.mainMerge("m2")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.head()} }},
		{name: "two merges after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			f.mainMerge("m2")
			f.mainMerge("m3")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 2", f.rev("HEAD^1"), f.head()} }},
		// The merge came after Fable was launched and before its record was
		// committed: it is after END, before the record's commit.
		{name: "merge between the launch and the record", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.mainMerge("m2")
			f.record("record", f.fableRun(f.mb, c1))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.rev("HEAD^1")} }},
		{name: "merge check after the merge", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			f.record("check", f.mergeRun(merge))
			return launch(f)
		}, want: rrValid},
		{name: "merge check after the merge with Antigravity high model", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			f.record("check", f.mergeRunModel(merge, "gemini-4-neon-high"))
			return launch(f)
		}, want: rrValid},
		{name: "merge check after the merge with non-high model ignored", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			f.record("check", f.mergeRunModel(merge, "claude-fable-example"))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.rev("HEAD^1")} }},
		{name: "not run merge check after the merge", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			f.record("check", f.mergeNotRun(merge, "agent-cannot-call"))
			return launch(f)
		}, want: rrValid},
		{name: "not run merge check with wrong-model and legacy not-opus", setup: func(f *rrFixture) []string {
			recorded(f)
			m1 := f.mainMerge("m2")
			f.record("check 1", f.mergeNotRun(m1, "wrong-model"))
			m2 := f.mainMerge("m3")
			f.record("check 2", f.mergeNotRun(m2, "not-opus"))
			return launch(f)
		}, want: rrValid},
		// The merge check record names a merge commit before its own commit,
		// so it can share that commit with the Fable record.
		{name: "Fable record and merge check in one commit", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			merge := f.mainMerge("m2")
			f.record("record", f.fableRun(f.mb, c1), f.mergeRun(merge))
			return launch(f)
		}, want: rrValid},
		// A fix of the same run made before the merge: the record's own
		// seen range starts at the old merge-base, which the widened START
		// check accepts.
		{name: "fix of the same run before a merge", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			fix := f.work("fix")
			f.record("record", f.fableRun(f.mb, c1), f.seen(f.mb, fix, "coderabbit"))
			f.mainMerge("m2")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.head()} }},
		// Rule 4: a fix after the merge inside a seen range of the merge
		// check's commit.
		{name: "fix of the merge check seen by a CLI", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			fix := f.work("merge fix")
			f.record("check", f.mergeRun(merge), f.seen(merge, fix, "coderabbit"))
			return launch(f)
		}, want: rrValid},
		{name: "fix of the merge check under a not-run line", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			fix := f.work("merge fix")
			f.record("check", f.mergeRun(merge), f.localNotRun(merge, fix))
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// X is inside the seen range of the merge check's commit but before
		// the merge, so rule 4 does not cover it, and START is the new
		// merge-base because a merge commit follows END.
		{name: "seen range of the merge check holding a commit before the merge", setup: func(f *rrFixture) []string {
			recorded(f)
			r := f.head()
			f.work("x")
			merge := f.mainMerge("m2")
			f.record("check", f.mergeRun(merge), f.seen(r, merge, "coderabbit"))
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// A commit with content before the merge invalidates the record, and
		// Fable then sees the whole PR from the new merge-base, whether the
		// record started at the old merge-base or at a branch commit. Without
		// x the same layout waits for the merge check, so the record is read.
		{name: "work then merge after a record from the merge-base", setup: func(f *rrFixture) []string {
			f.requireProbe("MERGE 1", func(s *rrFixture) {
				recorded(s)
				s.mainMerge("m2")
			})
			recorded(f)
			f.work("x")
			f.mainMerge("m2")
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		{name: "work then merge after a record from a branch commit", setup: func(f *rrFixture) []string {
			build := func(s *rrFixture, x bool) {
				c0 := s.work("c0")
				c1 := s.work("c1")
				s.record("record", s.fableRun(c0, c1))
				if x {
					s.work("x")
				}
				s.mainMerge("m2")
			}
			f.requireProbe("MERGE 1", func(s *rrFixture) { build(s, false) })
			build(f, true)
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// A record from the old merge-base whose END is after the merge
		// holds main's commits, so it is not read; the same record from the
		// new merge-base is.
		{name: "record from the old merge-base ending after the merge", setup: func(f *rrFixture) []string {
			f.requireProbe("VALID", func(s *rrFixture) {
				s.work("c1")
				merge := s.mainMerge("m2")
				s.record("record", s.fableRun(s.rev("refs/remotes/origin/main"), merge))
			})
			f.work("c1")
			merge := f.mainMerge("m2")
			f.record("record", f.fableRun(f.mb, merge))
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// The names of a merge check record. N is not a merge commit: were
		// the name counted, rule 4 would cover x, as it does when N is a
		// confirmable merge commit.
		{name: "merge check naming a commit that is not a merge", setup: func(f *rrFixture) []string {
			build := func(s *rrFixture, n func() string) {
				recorded(s)
				named := n()
				x := s.work("x")
				s.record("check", s.mergeRun(named), s.seen(named, x, "coderabbit"))
			}
			f.requireProbe("VALID", func(s *rrFixture) {
				build(s, func() string { return s.mainMerge("m2") })
			})
			build(f, func() string {
				f.record("n")
				return f.head()
			})
			return launch(f)
		}, want: rrMissing("HEAD~2")},
		// A merge check record confirms only the merge of its ID: the record
		// naming the first merge leaves the second waiting.
		{name: "merge check naming another merge", setup: func(f *rrFixture) []string {
			recorded(f)
			first := f.mainMerge("m2")
			f.record("check", f.mergeRun(first))
			f.mainMerge("m3")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.head()} }},
		// mm is a merge commit of origin/main, outside the branch, named with
		// a seen range over x after the branch's merge: counting the name
		// would cover x by rule 4, as naming the branch's merge does. mm's
		// first parent is not the merge-base either, so the case holds the
		// branch condition and the first-parent condition together.
		{name: "merge check naming a merge outside the branch", setup: func(f *rrFixture) []string {
			build := func(s *rrFixture, outside bool) {
				tree := s.rev(s.mb + "^{tree}")
				m2 := strings.TrimSpace(s.git("commit-tree", tree, "-p", s.mb, "-m", "m2"))
				mm := strings.TrimSpace(s.git("commit-tree", tree, "-p", s.mb, "-p", m2, "-m", "main merge"))
				s.git("update-ref", "refs/remotes/origin/main", mm)
				recorded(s)
				merge := s.mainMerge("m3")
				s.record("check", s.mergeRun(merge))
				x := s.work("x")
				named := merge
				if outside {
					named = mm
				}
				s.record("check", s.mergeRun(named), s.seen(merge, x, "coderabbit"))
			}
			f.requireProbe("VALID", func(s *rrFixture) { build(s, false) })
			build(f, true)
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// C merges origin/main's B into the fork point A: its second parent
		// is origin/main, but its first is behind the merge-base B. Named
		// with a seen range over x, it would cover x by rule 4, as a merge
		// whose first parent is a branch commit does.
		{name: "merge check naming a merge whose first parent is behind the merge-base", setup: func(f *rrFixture) []string {
			build := func(s *rrFixture, branchFirst bool) {
				tree := s.rev(s.mb + "^{tree}")
				b := strings.TrimSpace(s.git("commit-tree", tree, "-p", s.mb, "-m", "B"))
				s.git("update-ref", "refs/remotes/origin/main", b)
				first := s.mb
				if branchFirst {
					first = s.work("c0")
				}
				c := strings.TrimSpace(s.git("commit-tree", tree, "-p", first, "-p", b, "-m", "merge B"))
				s.git("reset", "-q", "--hard", c)
				c1 := s.work("c1")
				s.record("record", s.fableRun(b, c1))
				x := s.work("x")
				s.record("check", s.mergeRun(c), s.seen(c1, x, "coderabbit"))
			}
			f.requireProbe("VALID", func(s *rrFixture) { build(s, true) })
			build(f, false)
			return launch(f)
		}, want: rrMissing("HEAD~2")},
		// The seen range of y starts at the merge's second parent, the
		// merge-base until the second merge moves it; the widened START check
		// keeps the line, so y stays covered by rule 4.
		{name: "fix of a merge check kept after a second merge", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			y := f.work("merge fix")
			f.record("check", f.mergeRun(merge), f.seen(f.rev(merge+"^2"), y, "coderabbit"))
			f.mainMerge("m3")
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.head()} }},
		{name: "merge check lines of another form", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			f.record("check",
				strings.Replace(f.mergeRun(merge), merge, merge[:12], 1),
				"  "+f.mergeRun(merge),
				"see "+f.mergeRun(merge),
				strings.Replace(f.mergeRun(merge), "claude-opus-example", "claude-sonnet-example", 1),
				f.mergeNotRun(merge, "no-target"))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.rev("HEAD^1")} }},
		// A name of no object, and a name of a tree, are read past without
		// failing.
		{name: "merge check naming no object and a tree", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.mainMerge("m2")
			f.record("check", f.mergeRun(strings.Repeat("0", 40)), f.mergeRun(f.rev(merge+"^{tree}")))
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MERGE 1", f.rev("HEAD^1")} }},
		// The unconfirmable merge commits invalidate the record even when a
		// merge check record names them; "merge after the record" is the
		// third shape.
		{name: "octopus merge after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.unconfirmableMerge("octopus")
			f.record("check", f.mergeRun(merge))
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		{name: "merge with main's side first after the record", setup: func(f *rrFixture) []string {
			recorded(f)
			merge := f.unconfirmableMerge("main-first")
			f.record("check", f.mergeRun(merge))
			return launch(f)
		}, want: rrMissing("refs/remotes/origin/main")},
		// A replace ref makes the side merge's second parent origin/main to
		// plain git, which would make it confirmable.
		{name: "replace ref making a merge look confirmable", setup: func(f *rrFixture) []string {
			recorded(f)
			r := f.head()
			f.git("switch", "-q", "-c", "side")
			f.record("empty side")
			f.git("switch", "-q", "feature")
			merge := f.merge("side")
			f.record("check", f.mergeRun(merge))
			f.git("replace", "--graft", merge, r, f.mb)
			if f.rev(merge+"^2") != f.mb {
				f.t.Fatal("the replace ref does not change the merge's second parent for plain git, so the case checks nothing")
			}
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// A grafts file does the same for plain git as the replace ref.
		{name: "grafts file making a merge look confirmable", setup: func(f *rrFixture) []string {
			recorded(f)
			r := f.head()
			f.git("switch", "-q", "-c", "side")
			f.record("empty side")
			f.git("switch", "-q", "feature")
			merge := f.merge("side")
			f.record("check", f.mergeRun(merge))
			f.write(".git/info/grafts", merge+" "+r+" "+f.mb+"\n")
			if f.rev(merge+"^2") != f.mb {
				f.t.Fatal("the grafts file does not change the merge's second parent for plain git, so the case checks nothing")
			}
			return launch(f)
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		// The inputs and the failures.
		{name: "ENTRY of another form", setup: func(f *rrFixture) []string { return rrFable(f, "Launch") }},
		{name: "HEAD of another form", setup: func(*rrFixture) []string {
			return []string{"launch", "HEAD", rrMarkersPlan}
		}},
		{name: "HEAD names no commit", setup: func(*rrFixture) []string {
			return []string{"launch", strings.Repeat("0", 40), rrMarkersPlan}
		}},
		{name: "PLAN of another form", setup: func(f *rrFixture) []string {
			return []string{"launch", f.head(), "../" + rrMarkersPlan}
		}},
		{name: "PLAN not passed", setup: func(f *rrFixture) []string { return []string{"launch", f.head()} }},
		{name: "too many arguments", setup: func(f *rrFixture) []string { return append(rrFable(f, "launch"), "extra") }},
		{name: "no origin/main", setup: func(f *rrFixture) []string {
			f.git("update-ref", "-d", "refs/remotes/origin/main")
			return launch(f)
		}},
		// The environment, as for coverage.
		{name: "GIT_ variables", setup: unseenAfterRecord, env: func(f *rrFixture) []string {
			other := newRRFixture(f.t, f.form)
			return []string{"GIT_DIR=" + filepath.Join(other.repo, ".git"), "GIT_WORK_TREE=" + other.repo}
		}, want: rrMissing("HEAD~1")},
		{name: "exported functions", setup: unseenAfterRecord, env: func(*rrFixture) []string {
			return []string{
				"BASH_FUNC_git%%=() { return 0; }",
				"BASH_FUNC_printf%%=() { builtin printf 'VALID\\n'; }",
				"REVIEW_RECORDS_CLEAN_ENV=1",
			}
		}, want: rrMissing("HEAD~1")},
		{name: "BASH_ENV", setup: unseenAfterRecord, env: func(f *rrFixture) []string {
			bashEnv := filepath.Join(f.home, "bash-env")
			if err := os.WriteFile(bashEnv, []byte("command() { return 0; }\n"), 0o600); err != nil {
				f.t.Fatalf("write BASH_ENV: %v", err)
			}
			return []string{"BASH_ENV=" + bashEnv}
		}, want: rrMissing("HEAD~1")},
	}
}

// rrFableConfigBranch builds a branch on which the fable mode prints
// MISSING from the commit before the last: a record, a plan checkbox commit,
// two empty commits, and a last commit whose file name and subject are not
// ASCII, the first commit no rule covers. HEAD~4..HEAD~3 is the checkbox
// commit, as in rrConfigBranch, so the configuration scenarios' plain git
// commands change their output.
func rrFableConfigBranch(f *rrFixture) []string {
	f.write(".gitattributes", "*.md diff=upper\n")
	f.commit("attributes")
	f.record("record", f.fableRun(f.mb, f.head()))
	f.plan(rrMarkersPlan, "markers-created.md")
	f.record("empty")
	f.record("empty")
	f.write("caf\u00e9.txt", "x\n")
	f.commit("unseen caf\u00e9")
	return rrFable(f, "launch")
}

// rrFableMergeConfigBranch builds a branch on which the fable mode prints
// VALID at the checkpoint entry only through the merge rules: a record from
// the old merge-base, a plan checkbox commit, a merge of origin/main that
// adds a Markdown file under docs, the merge check record naming it, the
// checkpoint mark, and a last commit whose file name and subject are not
// ASCII. HEAD~4..HEAD~3 is the merge's change from its first parent and
// HEAD~1..HEAD the last commit, so the configuration scenarios' plain git
// commands change their output.
func rrFableMergeConfigBranch(f *rrFixture) []string {
	f.write(".gitattributes", "*.md diff=upper\n")
	f.commit("attributes")
	f.record("record", f.fableRun(f.mb, f.head()))
	f.plan(rrMarkersPlan, "markers-created.md")
	merge := f.mainMerge("m2")
	f.record("check", f.mergeRun(merge))
	f.record("mark", f.forms.checkpoint)
	f.write("caf\u00e9.txt", "x\n")
	f.commit("unseen caf\u00e9")
	return rrFable(f, "checkpoint")
}

// TestReviewRecordsCrossCheck runs the cross-check mode of the review records
// script on fixture repositories: the last record (both Cross-check and legacy
// Fable prefixes), the fixes of the same run, the commits without content, the
// checkpoint mark, the merge commits and the merge check records, model
// pattern matching boundaries, fail-closed _context.md validation, the
// failures, and that the caller's environment and git configuration change
// nothing. Each case runs with each bash scriptBashes returns (bash 3.2 and
// bash 4+).
//
// Some conditions are backed by others, so breaking one alone fails no case:
// that a widened START is a commit origin/main reaches (START must be an
// ancestor of END, a branch commit, and the range must hold no commit
// origin/main reaches); that a merge check record's name is a branch commit
// (a confirmable merge commit origin/main reaches would have the merge-base
// as its first parent and be a common ancestor descending from it, which
// cannot be); that the name is not the record's own commit (a message cannot
// name its own commit); that the name is a proper ancestor of the record's
// commit (rule 4 needs it to be an ancestor of the record's
// commit, and a merge after END that is not one reaches the branch only
// through a merge that is unconfirmable); and that a rule 4 commit is an
// ancestor of the merge check's commit (a seen range's END is a proper
// ancestor of it).
//
// The environment cases and the failures of the shared branch loading repeat
// TestReviewRecordsCoverage's on purpose: they show the cross-check mode keeps
// the same guarantees and fails with its own output. The configuration
// scenarios guard the reading through plumbing commands, which no such
// setting reaches today: they fail only when the cross-check mode starts
// reading through a porcelain command.
func TestReviewRecordsCrossCheck(t *testing.T) {
	form := seenRangeForm(t)
	forms := readFableForms(t)
	realContext := doctestutil.ReadDoc(t, contextPath)

	const (
		claudeCrossCheckRow = "| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |"
		validMappingTable   = "### Harness model mapping\n\n" +
			"| Harness | Role | Launch | Model | Verification pattern |\n" +
			"|---|---|---|---|---|\n" +
			"| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |\n" +
			"| `Claude Code` | `standard` | `agent` | `sonnet` | `claude-sonnet-*` |\n" +
			"| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |\n" +
			"| `Antigravity` | `cross-check` | `swarm` | `claude-opus-5-5-medium` | `claude-opus-5-5-medium` |\n" +
			"| `OpenCode` | `low` | `none` | `DeepSeek` | `-` |\n\n"
	)

	for _, bash := range scriptBashes(t) {
		t.Run(bash.name, func(t *testing.T) {
			t.Parallel()
			runRRCases(t, bash, "cross-check", "MISSING\n", form, forms, crossCheckCases())

			t.Run("pattern matching", func(t *testing.T) {
				t.Parallel()
				dotPrefixContext := replaceContextRow(t, realContext, claudeCrossCheckRow,
					"| `Claude Code` | `cross-check` | `agent` | `fable` | `v1.0-*` |")
				exactDotContext := replaceContextRow(t, realContext, claudeCrossCheckRow,
					"| `Claude Code` | `cross-check` | `agent` | `fable` | `v1.0` |")
				maxLiteral := strings.Repeat("a", 64)
				maxLiteralContext := replaceContextRow(t, realContext, claudeCrossCheckRow,
					"| `Claude Code` | `cross-check` | `agent` | `fable` | `"+maxLiteral+"` |")
				maxPrefix := strings.Repeat("b", 63)
				maxPrefixContext := replaceContextRow(t, realContext, claudeCrossCheckRow,
					"| `Claude Code` | `cross-check` | `agent` | `fable` | `"+maxPrefix+"*` |")
				fencedHeadingContext := realContext + "\n```markdown\n### Harness model mapping\n```\n"

				cases := []struct {
					name  string
					ctx   string
					model string
					valid bool
				}{
					{"48-char suffix accepted", realContext, "claude-fable-" + strings.Repeat("a", 48), true},
					{"49-char suffix rejected", realContext, "claude-fable-" + strings.Repeat("a", 49), false},
					{"empty suffix rejected", realContext, "claude-fable-", false},
					{"uppercase suffix rejected", realContext, "claude-fable-V4", false},
					{"space in model id rejected", realContext, "claude-fable-4 7", false},
					{"unregistered model rejected", realContext, "gemini-4-argon-high", false},
					{"prefix dot literal matches", dotPrefixContext, "v1.0-a", true},
					{"prefix dot is not regex wildcard", dotPrefixContext, "v1X0-a", false},
					{"exact dot literal matches", exactDotContext, "v1.0", true},
					{"exact dot is not regex wildcard", exactDotContext, "v1X0", false},
					{"64-char literal pattern accepted", maxLiteralContext, maxLiteral, true},
					{"63-char prefix plus star accepted", maxPrefixContext, maxPrefix + "x", true},
					{"fenced Harness model mapping heading ignored", fencedHeadingContext, "claude-opus-5-5-medium", true},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						t.Parallel()
						f := newRRFixtureWithContext(t, form, tc.ctx)
						f.forms = forms
						c1 := f.work("c1")
						f.record("record", f.crossCheckRun(f.mb, c1, tc.model))
						run := f.runScript(bash, reviewRecordsPath, append([]string{"cross-check"}, rrFable(f, "launch")...), scriptRunOptions{})
						want := "MISSING " + f.mb + "\n"
						if tc.valid {
							want = "VALID\n"
						}
						if run.code != 0 || run.stdout != want {
							t.Errorf("got %v, want output %q and exit 0", run, want)
						}
					})
				}
			})

			t.Run("fail-closed context validation", func(t *testing.T) {
				t.Parallel()
				// Positive control: validMappingTable itself passes so each
				// mutation below proves its single change caused the failure.
				t.Run("valid mapping table control", func(t *testing.T) {
					t.Parallel()
					f := newRRFixtureWithContext(t, form, replaceMappingSection(t, realContext, validMappingTable))
					f.forms = forms
					c1 := f.work("c1")
					f.record("record", f.crossCheckRun(f.mb, c1, "claude-opus-5-5-medium"))
					run := f.runScript(bash, reviewRecordsPath, append([]string{"cross-check"}, rrFable(f, "launch")...), scriptRunOptions{})
					if run.code != 0 || run.stdout != "VALID\n" || run.stderr != "" {
						t.Errorf("got %v, want exit 0, stdout VALID, empty stderr", run)
					}
				})

				cases := []struct {
					name       string
					ctx        string
					wantStderr string
				}{
					{"missing _context.md", "", "cannot read origin/main:.claude/commands/_context.md"},
					{"missing mapping section", replaceMappingSection(t, realContext, ""), "missing Harness model mapping section"},
					{"duplicate mapping section", replaceMappingSection(t, realContext, validMappingTable+validMappingTable), "duplicate Harness model mapping section"},
					{"unclosed fence in _context.md", realContext + "\n```text\nunclosed\n", "unclosed code fence in _context.md"},
					{"empty table with zero data rows", replaceMappingSection(t, realContext, "### Harness model mapping\n\n| Harness | Role | Launch | Model | Verification pattern |\n|---|---|---|---|---|\n\n"), "missing or empty Harness model mapping table"},
					{"wrong header", replaceMappingSection(t, realContext, "### Harness model mapping\n\n| Harness | Role | Launch | Model | Pattern |\n|---|---|---|---|---|\n| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |\n\n"), "invalid Harness model mapping header"},
					{"wrong separator", replaceMappingSection(t, realContext, "### Harness model mapping\n\n| Harness | Role | Launch | Model | Verification pattern |\n|---|---|---|---|\n| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |\n\n"), "invalid Harness model mapping separator"},
					{"wrong column count", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "| `OpenCode` | `low` | `none` | `DeepSeek` | `-` |", "| `OpenCode` | `low` | `none` | `DeepSeek` |")), "must have 5 columns"},
					{"indented table row", replaceMappingSection(t, realContext, "### Harness model mapping\n\n | Harness | Role | Launch | Model | Verification pattern |\n|---|---|---|---|---|\n| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |\n\n"), "indented table row"},
					{"unquoted cell", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`fable`", "fable")), "single code span"},
					{"multiple tables", replaceMappingSection(t, realContext, validMappingTable+"prose\n\n| Harness | Role | Launch | Model | Verification pattern |\n\n"), "multiple tables"},
					{"unknown Harness", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`OpenCode`", "`OtherHarness`")), "unknown Harness"},
					{"unknown Role", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`low`", "`ultra`")), "unknown Role"},
					{"unknown Launch", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`none`", "`cli`")), "unknown Launch"},
					{"whitespace-only Model", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`DeepSeek`", "`   `")), "invalid Model"},
					{"duplicate Harness Role", replaceMappingSection(t, realContext, strings.TrimSuffix(validMappingTable, "\n")+"| `Claude Code` | `cross-check` | `agent` | `fable2` | `claude-fable-2-*` |\n\n"), "duplicate (Harness, Role)"},
					{"none launch with non-dash pattern", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "| `-` |", "| `deepseek` |")), "Launch none row must have Verification pattern -"},
					{"agent launch with dash pattern", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`-`")), "cannot have Verification pattern -"},
					{"swarm launch with dash pattern", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "| `Antigravity` | `cross-check` | `swarm` | `claude-opus-5-5-medium` | `claude-opus-5-5-medium` |", "| `Antigravity` | `cross-check` | `swarm` | `claude-opus-5-5-medium` | `-` |")), "cannot have Verification pattern -"},
					{"65-char literal pattern", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`"+strings.Repeat("a", 65)+"`")), "invalid literal Verification pattern"},
					{"64-char prefix plus star", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`"+strings.Repeat("a", 64)+"*`")), "invalid prefix Verification pattern"},
					{"regex dot-star metacharacter", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`.*`")), "invalid prefix Verification pattern"},
					{"regex char class metacharacter", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`claude-[a-z]+`")), "invalid literal Verification pattern"},
					{"regex alternation pipe forbidden", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`claude-(a|b)`")), "forbidden character"},
					{"regex anchors metacharacter", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`^claude-fable$`")), "invalid literal Verification pattern"},
					{"whitespace in pattern", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`claude fable`")), "invalid literal Verification pattern"},
					{"mid-wildcard", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-fable-*`", "`claude*fable`")), "invalid literal Verification pattern"},
					{"broken uncollected role row fails whole table", replaceMappingSection(t, realContext, replaceContextRow(t, validMappingTable, "`claude-sonnet-*`", "`claude-[0-9]+`")), "invalid literal Verification pattern"},
					{"zero cross-check patterns", replaceMappingSection(t, realContext, "### Harness model mapping\n\n| Harness | Role | Launch | Model | Verification pattern |\n|---|---|---|---|---|\n| `Claude Code` | `high` | `agent` | `opus` | `claude-opus-*` |\n\n"), "no cross-check verification pattern"},
					{"zero high patterns", replaceMappingSection(t, realContext, "### Harness model mapping\n\n| Harness | Role | Launch | Model | Verification pattern |\n|---|---|---|---|---|\n| `Claude Code` | `cross-check` | `agent` | `fable` | `claude-fable-*` |\n\n"), "no high-role verification pattern"},
				}
				// Every fail-closed rejection exits 1 with "MISSING\n" on
				// stdout, so matching the stderr diagnostic substring is the
				// only way to verify that each malformed fixture triggered the
				// specific validation rule under test rather than an earlier one.
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						t.Parallel()
						f := newRRFixtureWithContext(t, form, tc.ctx)
						f.forms = forms
						c1 := f.work("c1")
						f.record("record", f.crossCheckRun(f.mb, c1, "claude-opus-5-5-medium"))
						run := f.runScript(bash, reviewRecordsPath, append([]string{"cross-check"}, rrFable(f, "launch")...), scriptRunOptions{})
						if run.code != 1 || run.stdout != "MISSING\n" || !strings.Contains(run.stderr, tc.wantStderr) {
							t.Errorf("got %v, want exit 1, stdout MISSING, stderr containing %q", run, tc.wantStderr)
						}
					})
				}

				t.Run("symlinked _context.md on origin/main", func(t *testing.T) {
					t.Parallel()
					f := newRRFixtureWithContext(t, form, "")
					linkPath := filepath.Join(f.repo, filepath.FromSlash(contextDocRepoPath))
					if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink("target.md", linkPath); err != nil {
						t.Fatal(err)
					}
					f.commit("add symlinked context")
					f.mb = f.head()
					f.git("update-ref", "refs/remotes/origin/main", f.mb)
					run := f.runScript(bash, reviewRecordsPath, append([]string{"cross-check"}, rrFable(f, "launch")...), scriptRunOptions{})
					if run.code != 1 || run.stdout != "MISSING\n" || !strings.Contains(run.stderr, "not a regular file") {
						t.Errorf("got %v, want exit 1, stdout MISSING, stderr containing %q", run, "not a regular file")
					}
				})
			})

			fixture := func(t *testing.T) *rrFixture {
				f := newRRFixture(t, form)
				f.forms = forms
				return f
			}
			checkConfigScenarios(t, "git config", bash, fixture, "cross-check", rrFableConfigBranch,
				func(f *rrFixture) string { return "MISSING " + f.rev("HEAD~1") + "\n" })
			checkConfigScenarios(t, "git config with a merge", bash, fixture, "cross-check", rrFableMergeConfigBranch,
				func(*rrFixture) string { return "VALID\n" })
		})
	}
}

// TestReviewRecordsFableAlias checks that the migration compatibility alias
// mode "fable" dispatches to the same cross-check logic and fail-closed output
// across both bash 3.2 and bash 4+ (representative VALID, MISSING, and
// fail-closed cases).
func TestReviewRecordsFableAlias(t *testing.T) {
	form := seenRangeForm(t)
	forms := readFableForms(t)
	cases := []rrCase{
		{name: "VALID with Cross-check record", setup: func(f *rrFixture) []string {
			c1 := f.work("c1")
			f.record("record", f.crossCheckRun(f.mb, c1, "claude-opus-5-5-medium"))
			return rrFable(f, "launch")
		}, want: rrValid},
		{name: "MISSING without record", setup: func(f *rrFixture) []string {
			f.work("c1")
			return rrFable(f, "launch")
		}, want: func(f *rrFixture) []string { return []string{"MISSING " + f.mb} }},
		{name: "fail-closed on missing _context.md", context: func(*testing.T) string { return "" }, setup: func(f *rrFixture) []string {
			f.work("c1")
			return rrFable(f, "launch")
		}},
	}
	for _, bash := range scriptBashes(t) {
		t.Run(bash.name, func(t *testing.T) {
			t.Parallel()
			runRRCases(t, bash, "fable", "MISSING\n", form, forms, cases)
		})
	}
}

// The code block lines a command document may write the review records
// script in: the reading of origin/main's commit, by its full reference so a
// local branch named origin/main cannot stand in; the write-out of that
// commit's copy to <records-script>, by the commit ID the reading printed;
// and the run of that copy.
const (
	reviewRecordsRef          = "git rev-parse --verify 'refs/remotes/origin/main^{commit}'"
	reviewRecordsShow         = "git --no-replace-objects show <origin-main-commit>:.claude/commands/_lib/review-records.sh > <records-script>"
	reviewRecordsRunCoverage  = "bash <records-script> coverage <LAST_REVIEWED> <HEAD> [<PLAN>]"
	reviewRecordsRunRemaining = "bash <records-script> coverage none <HEAD> [<PLAN>]"
	reviewRecordsRunFable     = "bash <records-script> fable <ENTRY> <HEAD> <PLAN>"
)

// reviewRecordsNames are the code spans that only name the review records
// script or its placeholder.
var reviewRecordsNames = []string{
	".claude/commands/_lib/review-records.sh", "_lib/review-records.sh", "<records-script>",
}

// TestReviewRecordsCallSite checks that the command documents run the review
// records script only as origin/main's copy: every line of a backquote-fenced
// block that names the script or its placeholder is the write-out or a run,
// and every code span that names them is the name alone. Running the work
// tree's copy would let the branch under review change the decision that
// lets it skip a PR review or a Fable partial review. task-pr-handoff.md's
// Part B step 2.5, runpr.md's step 3, and fable-review.md's validity decision,
// which the launch and the checkpoint entries run, have the reading of the
// reference, the write-out, and the run on consecutive lines, so a renamed form
// does not pass by matching nothing.
// A line that names neither the script nor its placeholder, or a block
// fenced otherwise, is out of its reach.
func TestReviewRecordsCallSite(t *testing.T) {
	for _, rel := range doctestutil.CommandDocs(t, repositoryRoot) {
		blockLines, spans := codeTexts(doctestutil.ReadDoc(t, filepath.Join(repositoryRoot, rel)))
		for i, code := range slices.Concat(blockLines, spans) {
			if !strings.Contains(code, "review-records") && !strings.Contains(code, "<records-script>") {
				continue
			}
			inBlock := i < len(blockLines)
			switch {
			case inBlock && (code == reviewRecordsShow || code == reviewRecordsRunCoverage || code == reviewRecordsRunRemaining || code == reviewRecordsRunFable):
			case !inBlock && slices.Contains(reviewRecordsNames, code):
			default:
				t.Errorf("%s has %q, which neither names the review records script nor writes out or runs origin/main's copy", rel, code)
			}
		}
	}
	for _, site := range []struct {
		name, path, run string
		section         func(*testing.T, string) string
	}{
		{"handoff step 2.5", handoffPath, reviewRecordsRunRemaining, itemSection("\n2.5. ", "\n3. ")},
		{"runpr.md's step 3", runprPath, reviewRecordsRunCoverage, headingSection(runprStep3)},
		{"fable-review.md's validity decision", fableReviewPath, reviewRecordsRunFable, headingSection("## The validity decision")},
	} {
		blockLines, _ := codeTexts(site.section(t, doctestutil.ReadDoc(t, site.path)))
		if i := slices.Index(blockLines, reviewRecordsShow); i < 1 || i+1 >= len(blockLines) ||
			blockLines[i-1] != reviewRecordsRef || blockLines[i+1] != site.run {
			t.Errorf("%s has no code block lines %q, %q, %q in this order",
				site.name, reviewRecordsRef, reviewRecordsShow, site.run)
		}
	}
}

// Paths of the other documents that define or write record lines.
const (
	opusReviewPath = "../../.claude/commands/_lib/opus-review.md"
	runplanPath    = "../../.claude/commands/runplan.md"
)

// recordPrefix is a record line's prefix: its text up to and including the
// first colon.
func recordPrefix(t *testing.T, where, line string) string {
	t.Helper()
	prefix, _, ok := strings.Cut(line, ":")
	if !ok {
		t.Fatalf("%s has the record line %q, which has no colon", where, line)
	}
	return prefix + ":"
}

// tableLinePrefixes returns the prefixes of the record lines named in the
// first column of the record-line tables in section, the tables whose
// header's first cell is "Line".
func tableLinePrefixes(t *testing.T, where, section string) []string {
	t.Helper()
	var prefixes []string
	inTable := false
	for row := range strings.Lines(section) {
		trimmed := strings.TrimSpace(row)
		if !strings.HasPrefix(trimmed, "|") {
			inTable = false
			continue
		}
		first, _, ok := strings.Cut(strings.TrimPrefix(trimmed, "| "), " | ")
		if first == "Line" {
			inTable = true
			continue
		}
		if !inTable || !ok || !strings.HasPrefix(first, "`") {
			continue
		}
		if line, found := firstCodeSpan(first); found {
			prefixes = append(prefixes, recordPrefix(t, where, line))
		}
	}
	return prefixes
}

// TestRecordLinePrefixesDistinct checks that the record lines the Fable
// partial review, the merge check, and the seen-range line add cannot be
// read as an existing record line or as one another: no new prefix equals
// an existing one or another new one, and neither starts with the other.
// The existing prefixes are those of local-cli-review.md step 7's table
// (the seen-range line aside) and of the code blocks of
// opus-review.md's "Record lines", which _lib/high-review-decision.sh and
// the local CLI review's readers read; the new ones are those of
// fable-review.md's "Record lines" table and the seen-range line.
func TestRecordLinePrefixesDistinct(t *testing.T) {
	const seenPrefix = "Local CLI reviewed:"
	existing := map[string]struct{}{}
	var added []string
	for _, prefix := range tableLinePrefixes(t, "local-cli-review.md step 7", localReviewStep7(t)) {
		if prefix == seenPrefix {
			added = append(added, prefix)
			continue
		}
		existing[prefix] = struct{}{}
	}
	localCount := len(existing)
	opus := requireSection(t, "opus-review.md", doctestutil.ReadDoc(t, opusReviewPath), "## Record lines")
	blockLines, _ := codeTexts(opus)
	for _, line := range blockLines {
		existing[recordPrefix(t, "opus-review.md \"Record lines\"", line)] = struct{}{}
	}
	fable := requireSection(t, "fable-review.md", doctestutil.ReadDoc(t, fableReviewPath), "## Record lines")
	// The table's rows are of two families, the Fable lines and the merge
	// check lines. The rows of one family may share a prefix (three rows are
	// "Fable review:"), so each distinct prefix is compared once; the
	// families give the lower bounds, so a merge check row moved into the
	// Fable family's prefixes fails them.
	families := map[bool]map[string]struct{}{false: {}, true: {}}
	for _, prefix := range tableLinePrefixes(t, "fable-review.md \"Record lines\"", fable) {
		families[strings.HasPrefix(prefix, mergeCheckFamily)][prefix] = struct{}{}
	}
	for _, family := range families {
		for prefix := range family {
			added = append(added, prefix)
		}
	}
	// The lower bounds keep a table the reading misses from passing with
	// nothing compared.
	if localCount < 3 || len(existing)-localCount < 2 || len(families[false]) < 3 || len(families[true]) < 3 ||
		!slices.Contains(added, seenPrefix) {
		t.Fatalf("read %d prefixes from local-cli-review.md step 7, %d more from opus-review.md, %d Fable and %d merge check prefixes from fable-review.md, and the seen-range line %v; want at least 3, 2, 3, 3, and true",
			localCount, len(existing)-localCount, len(families[false]), len(families[true]), slices.Contains(added, seenPrefix))
	}
	for i, n := range added {
		for e := range existing {
			if strings.HasPrefix(n, e) || strings.HasPrefix(e, n) {
				t.Errorf("the new record line prefix %q overlaps the existing prefix %q", n, e)
			}
		}
		for _, o := range added[i+1:] {
			if strings.HasPrefix(n, o) || strings.HasPrefix(o, n) {
				t.Errorf("the new record line prefixes %q and %q overlap", n, o)
			}
		}
	}
}

// mergeCheckFamily starts the prefix of every merge check line in
// fable-review.md's "Record lines".
const mergeCheckFamily = "Merge check"

// mentionCheckReference is how a section names the mention check that runs
// before a commit of record lines.
const mentionCheckReference = "`_lib/local-cli-review.md` step 7, \"Mention check\""

// TestRecordCommitsCheckMention checks that each section that makes a commit
// holding a Fable line runs the mention check first: it holds the check's
// command or names local-cli-review.md step 7's "Mention check". A Fable line
// carries the session's summaries of Fable's findings, which may hold the
// mention.
func TestRecordCommitsCheckMention(t *testing.T) {
	runplan := doctestutil.ReadDoc(t, runplanPath)
	var step78 string
	for line := range strings.Lines(runplan) {
		if strings.HasPrefix(line, "7.8. ") {
			step78 = line
		}
	}
	fable := doctestutil.ReadDoc(t, fableReviewPath)
	for _, site := range []struct{ name, text string }{
		{"local-cli-review.md step 7", localReviewStep7(t)},
		{"runplan.md step 7.8", step78},
		{"fable-review.md \"Record lines\"", requireSection(t, "fable-review.md", fable, "## Record lines")},
		{"fable-review.md \"The standalone path\"", requireSection(t, "fable-review.md", fable, "## The standalone path")},
	} {
		text := flat(site.text)
		if text == "" {
			t.Errorf("%s is missing", site.name)
			continue
		}
		if !strings.Contains(text, mentionCheckCommand) && !strings.Contains(text, flat(mentionCheckReference)) {
			t.Errorf("%s neither runs %s nor names %s", site.name, mentionCheckCommand, mentionCheckReference)
		}
	}
}
