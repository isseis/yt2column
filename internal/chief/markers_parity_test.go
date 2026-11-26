//go:build test

package chief

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// decisionScript is the repository's executable form of the PR marker
// reading.
var decisionScript = filepath.Join("..", "..", ".claude", "commands", "_lib", "high-review-decision.sh")

// bashMajor matches the major version in `bash --version`.
var bashMajor = regexp.MustCompile(`version ([0-9]+)\.`)

// decisionClass is what high-review-decision.sh can tell apart about a value
// when no commit is covered by an implementation record: low, no value,
// and everything else.
func decisionClass(r Requirement) string {
	switch r {
	case RequirementLow:
		return "NONE value-low"
	case RequirementAbsent:
		return "NONE value-absent"
	default:
		return "RUN"
	}
}

// TestMarkerReadingMatchesDecisionScript runs the tool's reading and
// high-review-decision.sh on the same plans and checks that they fall in the
// same class. The repository is shaped so the script decides by the value
// alone: origin/main and HEAD hold the same plan, HEAD is one commit ahead
// and changes a file other than the plan, and no commit holds a record line.
// origin/main's _context.md is the repository's current file, so the routing
// table and the other tables in it are checked not to read as old values.
func TestMarkerReadingMatchesDecisionScript(t *testing.T) {
	// This parent stays serial on purpose; only its subtests are parallel, and
	// they run alone, before the parallel top-level tests are released. The
	// subtests start git and bash processes, and a child process holds a copy
	// of every open file descriptor between fork and exec: a flock another
	// parallel test took through cachelock would then stay held for a moment
	// after that test closed it, and its next Acquire would report busy.
	if testBash == "" {
		t.Fatal("bash is not on PATH; the parity test needs bash 4 or later")
	}
	out, err := runBashVersion()
	if err != nil {
		t.Fatalf("bash --version: %v", err)
	}
	m := bashMajor.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("cannot read the version of %s: %q", testBash, out)
	}
	if major, _ := strconv.Atoi(m[1]); major < 4 {
		t.Fatalf("the bash on PATH (%s) is older than 4: %q", testBash, out)
	}
	script, err := filepath.Abs(decisionScript)
	if err != nil {
		t.Fatal(err)
	}
	contextDoc := repoContext(t)
	old := ParseOldValues(contextDoc)
	const plan = "docs/tasks/0001_parity/03_implementation_plan.md"
	cases := []struct{ file, target string }{
		{"marker-high.md", "2"},
		{"marker-standard-review.md", "2"},
		{"marker-low.md", "2"},
		{"marker-low.md crlf", "2"},
		{"marker-opus.md", "2"},
		{"marker-sonnet-review.md", "2"},
		{"marker-sonnet.md", "2"},
		{"marker-old-frontier-required.md", "2"},
		{"marker-old-frontier-recommended.md", "2"},
		{"marker-old-standard.md", "2"},
		{"marker-old-standard.md", "3"},
		{"marker-unreadable.md", "2"},
		{"marker-absent.md", "2"},
		{"marker-two-values.md", "2"},
		{"marker-fenced-value.md", "2"},
		{"marker-tilde-fence.md", "2"},
		{"marker-sonnet.md crlf", "2"},
		{"marker-duplicate.md", "2"},
		{"single-low.md", singleTarget},
		{"single-sonnet.md", singleTarget},
		{"single-no-section.md", singleTarget},
		{"single-outside-chapter3.md", singleTarget},
		{"marker-opus.md", singleTarget},
	}
	for _, c := range cases {
		t.Run(c.file+" "+c.target, func(t *testing.T) {
			t.Parallel()
			doc := planSample(t, c.file)
			repo := newTestRepo(t)
			repo.Write(plan, string(doc))
			repo.Write(".claude/commands/_context.md", string(contextDoc))
			base := repo.CommitAll("plan")
			repo.Git("update-ref", "refs/remotes/origin/main", base)
			repo.Write("other.txt", "change\n")
			repo.CommitAll("change")
			got, _, _ := strings.Cut(repo.Run(testBash, script, plan, c.target, base), "\n")
			want := decisionClass(readRequirement(doc, c.target, old).Value)
			if got != want {
				t.Errorf("script = %q, tool reads as %q", got, want)
			}
		})
	}
}

// runBashVersion returns the output of `bash --version`, run with the test
// environment.
func runBashVersion() (string, error) {
	cmd := exec.Command(testBash, "--version") //nolint:gosec // runs the bash resolved at start
	cmd.Env = testEnv(os.TempDir())
	out, err := cmd.Output()
	return string(out), err
}
