//go:build test

package main

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// crossCheckStateParts runs the state check block of cross-check-review.md, written to
// a file outside the fixture, in f and returns its output as part name to
// hash.
func crossCheckStateParts(t *testing.T, f *gitFixture, block string) map[string]string {
	t.Helper()
	run := f.runScript(scriptBash{name: "path-bash", path: "bash"}, block, nil, scriptRunOptions{})
	if run.code != 0 || run.stderr != "" {
		t.Fatalf("the state check block failed: %v", run)
	}
	parts := map[string]string{}
	for line := range strings.Lines(run.stdout) {
		name, hash, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || len(hash) != 64 {
			t.Fatalf("the state check block printed %q, not <part> <sha256>", line)
		}
		parts[name] = hash
	}
	got := slices.Sorted(maps.Keys(parts))
	if want := []string{"config", "flags", "head", "hooks", "index", "info", "ops", "replace", "settings", "status", "watched"}; !slices.Equal(got, want) {
		t.Fatalf("the state check block printed the parts %v, want %v", got, want)
	}
	return parts
}

// TestCrossCheckStateCheckBlock runs the state check block of
// cross-check-review.md, which is a command example the procedure runs, and
// checks that it sees the changes git status does not show: a change to the
// content of a skip-worktree file, a new file in the hooks directory, a
// replace ref, a merge in progress, an ignored file below .agents/, an ignored
// instruction file another harness reads, and a nested .git entry below
// .claude/ or .agents/.
func TestCrossCheckStateCheckBlock(t *testing.T) {
	if _, err := exec.LookPath("shasum"); err != nil {
		t.Fatalf("shasum is not on PATH: %v", err)
	}
	section := requireSection(t, "cross-check-review.md", doctestutil.ReadDoc(t, crossCheckReviewPath), "## The state check")
	blockLines, _ := codeTexts(section)
	script := filepath.Join(t.TempDir(), "state.sh")
	if err := os.WriteFile(script, []byte(strings.Join(blockLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	f := newGitFixture(t)
	f.write("flagged.txt", "one\n")
	f.commit("add flagged.txt")
	f.git("update-index", "--skip-worktree", "flagged.txt")
	before := crossCheckStateParts(t, f, script)

	f.write("flagged.txt", "two\n")
	if f.git("status", "--porcelain") != "" {
		t.Fatal("git status shows the skip-worktree change, so the case checks nothing")
	}
	afterFlag := crossCheckStateParts(t, f, script)
	if afterFlag["flags"] == before["flags"] {
		t.Error("the flags part did not change with the content of a skip-worktree file")
	}

	f.write(".git/hooks/not-a-hook", "x\n")
	afterHook := crossCheckStateParts(t, f, script)
	if afterHook["hooks"] == afterFlag["hooks"] {
		t.Error("the hooks part did not change with a new file in the hooks directory")
	}
	// A replace ref and a merge in progress change neither git status nor
	// any file the other parts read.
	f.git("replace", f.rev("HEAD"), strings.TrimSpace(f.git("commit-tree", f.rev("HEAD^{tree}"), "-m", "other")))
	afterReplace := crossCheckStateParts(t, f, script)
	if afterReplace["replace"] == afterHook["replace"] {
		t.Error("the replace part did not change with a new replace ref")
	}
	f.write(".git/MERGE_HEAD", f.rev("HEAD")+"\n")
	afterMerge := crossCheckStateParts(t, f, script)
	if afterMerge["ops"] == afterReplace["ops"] {
		t.Error("the ops part did not change with a merge in progress")
	}
	f.write(".git/MERGE_HEAD", f.rev("HEAD~0^{tree}")+"\n")
	if crossCheckStateParts(t, f, script)["ops"] == afterMerge["ops"] {
		t.Error("the ops part did not change with the content of MERGE_HEAD")
	}
	for _, name := range []string{"status", "head", "config", "info", "watched", "settings", "index"} {
		if afterMerge[name] != before[name] {
			t.Errorf("the %s part changed although nothing it covers changed", name)
		}
	}

	// Ignored files git status does not show: a skill below .agents/, each
	// instruction file another harness reads, and a nested .git entry below
	// .claude/ and .agents/. Each must change the watched part while git
	// status stays the same. The lowercase .git below .agents/ is one git
	// never lists on any platform, so only the block's nested .git scan can
	// see it.
	f.write(".git/info/exclude", ".agents/\n.claude/\nAGENTS.md\nAGENT.md\nGEMINI.md\nAGENTS.override.md\n")
	prev := crossCheckStateParts(t, f, script)
	status := f.git("status", "--porcelain", "--untracked-files=all")
	for _, path := range []string{
		".agents/skills/evil/SKILL.md",
		"AGENTS.md",
		"AGENT.md",
		"GEMINI.md",
		"sub/AGENTS.override.md",
		".claude/nested/.git/HEAD",
		".agents/nested/.GIT/HEAD",
		".agents/lower/.git/HEAD",
	} {
		f.write(path, "x\n")
		if got := f.git("status", "--porcelain", "--untracked-files=all"); got != status {
			t.Fatalf("git status shows %s (%q), so the case checks nothing", path, got)
		}
		after := crossCheckStateParts(t, f, script)
		if after["watched"] == prev["watched"] {
			t.Errorf("the watched part did not change with the ignored %s", path)
		}
		prev = after
	}

	// A corrupt index makes git status and git ls-files fail: the block must
	// fail instead of hashing their empty output as a clean state.
	f.write(".git/index", "not an index\n")
	if run := f.runScript(scriptBash{name: "path-bash", path: "bash"}, script, nil, scriptRunOptions{}); run.code == 0 {
		t.Errorf("the state check block exited 0 with a corrupt index: %v", run)
	}
}
