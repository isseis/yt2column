//go:build test

package codexreview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/codexreview/testutil"
)

// Events a completed run of codex exec review --json writes, in the recorded
// order, and the ones a failed run adds.
const (
	eventThreadStarted  = `{"type":"thread.started","thread_id":"t-1"}`
	eventTurnStarted    = `{"type":"turn.started"}`
	eventTurnCompleted  = `{"type":"turn.completed","usage":{"input_tokens":0}}`
	eventTurnFailed     = `{"type":"turn.failed","error":{"message":"m"}}`
	eventError          = `{"type":"error","message":"m"}`
	eventAgentMessage   = `{"type":"item.completed","item":{"type":"agent_message","text":"t"}}`
	fakeVersion         = "codex-cli 0.163.0"
	noFindingsMessage   = "The change adds one function. No defects were found.\n"
	changedFileContents = "one\ntwo\nthree\n"
)

// shortLimit is the time limit of a test that waits for a timeout: long
// enough for a race-enabled fake to start and act under load.
const shortLimit = 3 * time.Second

var completedEvents = []string{eventThreadStarted, eventTurnStarted, eventAgentMessage, eventTurnCompleted}

// fixture is a repository whose BASE..HEAD adds a.go, with the fake codex on
// PATH and the test's current directory at the worktree root.
type fixture struct {
	fake *codexreviewtestutil.Fake
	repo *codexreviewtestutil.Repo
	base string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	fake := codexreviewtestutil.NewFake(t)
	repo := codexreviewtestutil.NewRepo(t)
	base := repo.Head()
	repo.Write("a.go", changedFileContents)
	repo.CommitAll("add a.go")
	t.Chdir(repo.Dir)
	return fixture{fake: fake, repo: repo, base: base}
}

// completed returns the behavior of a run that completes with message.
func completed(message string) codexreviewtestutil.Behavior {
	return codexreviewtestutil.Behavior{Version: fakeVersion, Events: completedEvents, LastMessage: &message}
}

// findingOn returns a final message with one finding at path:start-end.
func findingOn(path string, start, end int) string {
	return "One defect.\n\nReview comment:\n\n- [P1] A defect \u2014 " + path + ":" +
		strconv.Itoa(start) + "-" + strconv.Itoa(end) + "\n  The body.\n"
}

// mustRun runs Run and fails the test when it returns an error.
func mustRun(t *testing.T, base string) Result {
	t.Helper()
	r, err := Run(context.Background(), base, os.Environ())
	if err != nil {
		t.Fatalf("Run = %v", err)
	}
	return r
}

// assertKind checks the kind and the Detail of r.
func assertKind(t *testing.T, r Result, kind Kind, detail string) {
	t.Helper()
	if r.Kind != kind || r.Detail != detail {
		t.Fatalf("result = %s/%q, want %s/%q (%+v)", r.Kind, r.Detail, kind, detail, r)
	}
}

// TestRunPassesFixedArguments checks the exact arguments codex receives: the
// base, the turned-off configuration and features, the read-only sandbox, no
// approval, and the final message file in the run directory.
func TestRunPassesFixedArguments(t *testing.T) {
	fx := newFixture(t)
	fx.fake.Set(completed(noFindingsMessage))
	r := mustRun(t, fx.base)
	assertKind(t, r, KindNoFindings, "")
	want := []string{
		"exec", "review", "--base", fx.base,
		"--ignore-user-config", "--ignore-rules",
		"--disable", "apps", "--disable", "plugins", "--disable", "remote_plugin", "--disable", "hooks",
		"--disable", "browser_use", "--disable", "computer_use", "--disable", "image_generation",
		"--disable", "multi_agent", "--disable", "tool_suggest", "--disable", "skill_mcp_dependency_install",
		"-c", `sandbox_mode="read-only"`, "-c", `approval_policy="never"`,
		"-c", `shell_environment_policy.inherit="core"`,
		"--json", "-o", filepath.Join(r.RunDir, "last-message.txt"),
	}
	if got := fx.fake.Args(); !slices.Equal(got, want) {
		t.Errorf("codex arguments = %q, want %q", got, want)
	}
	if got := fx.fake.Starts(); !slices.Equal(got, []string{"version", "review"}) {
		t.Errorf("codex starts = %q, want version then review", got)
	}
	if r.CodexVersion != fakeVersion {
		t.Errorf("CodexVersion = %q, want %q", r.CodexVersion, fakeVersion)
	}
}

// TestRunPassesOnlyAllowedEnvironment checks that codex receives only the
// allowed variables: a secret in the caller's environment, or any other
// variable, never reaches codex or the commands it runs.
func TestRunPassesOnlyAllowedEnvironment(t *testing.T) {
	fx := newFixture(t)
	t.Setenv("DEEPSEEK_API_KEY", "secret-key")
	t.Setenv("SLACK_WEBHOOK_URL", "https://hooks.example.com/secret")
	t.Setenv("GIT_DIR", "/elsewhere")
	t.Setenv("LANG", "C.UTF-8")
	t.Setenv("HTTPS_PROXY", "http://proxy.example.com:8080")
	fx.fake.Set(completed(noFindingsMessage))
	assertKind(t, mustRun(t, fx.base), KindNoFindings, "")
	got := map[string]string{}
	for _, entry := range fx.fake.Env() {
		name, value, _ := strings.Cut(entry, "=")
		got[name] = value
	}
	want := map[string]string{
		"PATH":                os.Getenv("PATH"),
		"HOME":                os.Getenv("HOME"),
		"XDG_CONFIG_HOME":     os.Getenv("XDG_CONFIG_HOME"),
		"TMPDIR":              os.Getenv("TMPDIR"),
		"LANG":                "C.UTF-8",
		"HTTPS_PROXY":         "http://proxy.example.com:8080",
		"GIT_OPTIONAL_LOCKS":  "0",
		"GIT_TERMINAL_PROMPT": "0",
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("codex environment %s = %q, want %q", name, got[name], value)
		}
	}
	for name := range got {
		if !slices.Contains(codexEnvNames, name) && name != "GIT_OPTIONAL_LOCKS" && name != "GIT_TERMINAL_PROMPT" {
			t.Errorf("codex received %s, which is not an allowed variable", name)
		}
	}
}

// TestRunPreconditions checks that each failed precondition returns its kind
// and Detail before the review starts, so codex never reviews.
func TestRunPreconditions(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(t *testing.T, fx fixture) (base string)
		behavior func(fx fixture, b *codexreviewtestutil.Behavior)
		kind     Kind
		detail   string
		// versionStarted: codex --version ran before the failed check.
		versionStarted bool
	}{
		{
			name:    "short_base",
			prepare: func(_ *testing.T, fx fixture) string { return fx.base[:39] },
			kind:    KindUsage, detail: detailBadArguments,
		},
		{
			name:    "uppercase_base",
			prepare: func(_ *testing.T, fx fixture) string { return strings.ToUpper(fx.base) },
			kind:    KindUsage, detail: detailBadArguments,
		},
		{
			name: "subdirectory",
			prepare: func(t *testing.T, fx fixture) string {
				fx.repo.Write("sub/b.go", "b\n")
				fx.repo.CommitAll("add sub")
				t.Chdir(filepath.Join(fx.repo.Dir, "sub"))
				return fx.base
			},
			kind: KindPrecondition, detail: detailNotWorktreeRoot,
		},
		{
			name: "outside_any_repository",
			prepare: func(t *testing.T, fx fixture) string {
				t.Chdir(fx.fake.Dir)
				return fx.base
			},
			kind: KindPrecondition, detail: detailNotWorktreeRoot,
		},
		{
			name: "base_on_another_branch",
			prepare: func(_ *testing.T, fx fixture) string {
				fx.repo.Git("switch", "-q", "-c", "side", fx.base)
				fx.repo.Write("side.go", "side\n")
				side := fx.repo.CommitAll("side")
				fx.repo.Git("switch", "-q", "main")
				return side
			},
			kind: KindPrecondition, detail: detailBaseNotAncestor,
		},
		{
			name:    "unknown_base",
			prepare: func(_ *testing.T, _ fixture) string { return strings.Repeat("0", 40) },
			kind:    KindPrecondition, detail: detailBaseNotAncestor,
		},
		{
			name: "modified_file",
			prepare: func(_ *testing.T, fx fixture) string {
				fx.repo.Write("a.go", "changed\n")
				return fx.base
			},
			kind: KindPrecondition, detail: DetailDirtyTree,
		},
		{
			name: "untracked_file_hidden_by_configuration",
			prepare: func(_ *testing.T, fx fixture) string {
				fx.repo.Git("config", "status.showUntrackedFiles", "no")
				fx.repo.Write("new/untracked.go", "u\n")
				return fx.base
			},
			kind: KindPrecondition, detail: DetailDirtyTree,
		},
		{
			name: "project_codex_directory",
			prepare: func(_ *testing.T, fx fixture) string {
				fx.repo.Write(".codex/config.toml", "model = \"m\"\n")
				fx.repo.CommitAll("add .codex")
				return fx.base
			},
			kind: KindPrecondition, detail: detailProjectCodexConfig,
		},
		{
			name: "codex_not_on_path",
			prepare: func(t *testing.T, fx fixture) string {
				if err := os.Remove(filepath.Join(fx.fake.BinDir, "codex")); err != nil {
					t.Fatalf("remove codex: %v", err)
				}
				return fx.base
			},
			kind: KindCodexNotFound, detail: "",
		},
		{
			name:     "version_fails",
			behavior: func(_ fixture, b *codexreviewtestutil.Behavior) { b.VersionExit = 1 },
			kind:     KindPrecondition, detail: detailVersionFailed, versionStarted: true,
		},
		{
			name:     "version_hangs",
			behavior: func(_ fixture, b *codexreviewtestutil.Behavior) { b.VersionHang = true },
			kind:     KindPrecondition, detail: detailVersionTimeout, versionStarted: true,
		},
		{
			name:     "version_not_one_line",
			behavior: func(_ fixture, b *codexreviewtestutil.Behavior) { b.Version = "codex-cli\n0.163.0" },
			kind:     KindPrecondition, detail: detailVersionUnreadable, versionStarted: true,
		},
		{
			// A change after the first clean check must still keep codex from
			// reviewing: the baseline itself has to be clean.
			name: "change_while_version_runs",
			behavior: func(fx fixture, b *codexreviewtestutil.Behavior) {
				b.VersionWrites = map[string]string{filepath.Join(fx.repo.Dir, "a.go"): "edited\n"}
			},
			kind: KindPrecondition, detail: DetailDirtyTree, versionStarted: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortenTimeouts(t, codexTimeout, stopGrace, 3*time.Second, gitTimeout)
			fx := newFixture(t)
			b := completed(noFindingsMessage)
			if tt.behavior != nil {
				tt.behavior(fx, &b)
			}
			fx.fake.Set(b)
			base := fx.base
			if tt.prepare != nil {
				base = tt.prepare(t, fx)
			}
			r := mustRun(t, base)
			assertKind(t, r, tt.kind, tt.detail)
			var wantStarts []string
			if tt.versionStarted {
				wantStarts = []string{"version"}
			}
			if got := fx.fake.Starts(); !slices.Equal(got, wantStarts) {
				t.Errorf("codex starts = %q, want %q", got, wantStarts)
			}
		})
	}
}

// TestRunKinds checks the kind and Detail of each way a review that leaves the
// git state alone can end.
func TestRunKinds(t *testing.T) {
	exactLimit := strings.Repeat("x", maxMessageBytes-1) + "\n"
	tests := []struct {
		name     string
		behavior func(fx fixture) codexreviewtestutil.Behavior
		kind     Kind
		detail   string
		findings []Finding
		// shortLimit runs the row with the short time limit.
		shortLimit bool
	}{
		{
			name:     "findings",
			behavior: func(fx fixture) codexreviewtestutil.Behavior { return completed(findingOn(fx.repo.Dir+"/a.go", 2, 3)) },
			kind:     KindFindings,
			findings: []Finding{{Path: "a.go", Start: 2, End: 3, InRange: true, Title: "A defect", Body: "The body."}},
		},
		{
			name: "finding_on_unchanged_file",
			behavior: func(fx fixture) codexreviewtestutil.Behavior {
				return completed(findingOn(fx.repo.Dir+"/README.md", 1, 1))
			},
			kind:     KindFindings,
			findings: []Finding{{Path: "README.md", Start: 1, End: 1, InRange: false, Title: "A defect", Body: "The body."}},
		},
		{
			name:     "no_findings",
			behavior: func(_ fixture) codexreviewtestutil.Behavior { return completed(noFindingsMessage) },
			kind:     KindNoFindings,
		},
		{
			// The tool's own clock is read before the output limits.
			name: "timeout_with_output_over_limit",
			behavior: func(_ fixture) codexreviewtestutil.Behavior {
				b := completed(noFindingsMessage)
				b.StdoutPadding = maxStreamBytes
				b.Hang = true
				return b
			},
			kind: KindTimeout, shortLimit: true,
		},
		{
			name: "exit_status",
			behavior: func(_ fixture) codexreviewtestutil.Behavior {
				b := completed(noFindingsMessage)
				b.Exit = 3
				return b
			},
			kind: KindCodexFailed, detail: "exit_status_3",
		},
		{
			name: "no_turn_completed",
			behavior: func(_ fixture) codexreviewtestutil.Behavior {
				b := completed(noFindingsMessage)
				b.Events = []string{eventThreadStarted, eventTurnStarted, eventAgentMessage}
				return b
			},
			kind: KindIncomplete, detail: detailNoTurnCompleted,
		},
		{
			name: "stdout_too_large",
			behavior: func(_ fixture) codexreviewtestutil.Behavior {
				b := completed(noFindingsMessage)
				b.StdoutPadding = maxStreamBytes
				return b
			},
			kind: KindOutputTooLarge, detail: detailOutput,
		},
		{
			name: "stderr_too_large",
			behavior: func(_ fixture) codexreviewtestutil.Behavior {
				b := completed(noFindingsMessage)
				b.StderrPadding = maxStreamBytes + 1
				return b
			},
			kind: KindOutputTooLarge, detail: detailOutput,
		},
		{
			name:     "final_message_at_limit",
			behavior: func(_ fixture) codexreviewtestutil.Behavior { return completed(exactLimit) },
			kind:     KindNoFindings,
		},
		{
			name:     "final_message_over_limit",
			behavior: func(_ fixture) codexreviewtestutil.Behavior { return completed(exactLimit + "x") },
			kind:     KindOutputTooLarge, detail: detailFinalMessage,
		},
		{
			name:     "final_message_empty",
			behavior: func(_ fixture) codexreviewtestutil.Behavior { return completed("") },
			kind:     KindOutputEmpty, detail: detailMessageEmpty,
		},
		{
			name: "final_message_missing",
			behavior: func(_ fixture) codexreviewtestutil.Behavior {
				b := completed("")
				b.LastMessage = nil
				return b
			},
			kind: KindOutputEmpty, detail: detailMessageMissing,
		},
		{
			// The link's target holds a valid message; only the file-type check
			// rejects it.
			name: "final_message_is_symlink",
			behavior: func(fx fixture) codexreviewtestutil.Behavior {
				target := filepath.Join(fx.fake.Dir, "elsewhere.txt")
				b := completed("")
				b.LastMessage = nil
				b.Writes = map[string]string{target: noFindingsMessage}
				b.LastMessageLink = target
				return b
			},
			kind: KindUnreadable, detail: detailMessageNotRegular,
		},
		{
			name: "final_message_unreadable",
			behavior: func(_ fixture) codexreviewtestutil.Behavior {
				return completed("First paragraph.\n\nSecond paragraph.\n")
			},
			kind: KindUnreadable, detail: detailMessageGrammar,
		},
		{
			name:     "finding_outside_root",
			behavior: func(fx fixture) codexreviewtestutil.Behavior { return completed(findingOn(fx.fake.Dir+"/a.go", 1, 1)) },
			kind:     KindUnreadable, detail: detailFindingOutsideRoot,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := 30 * time.Second
			if tt.shortLimit {
				limit = shortLimit
			}
			shortenTimeouts(t, limit, time.Second, versionTimeout, gitTimeout)
			fx := newFixture(t)
			fx.fake.Set(tt.behavior(fx))
			r := mustRun(t, fx.base)
			assertKind(t, r, tt.kind, tt.detail)
			findings := tt.findings
			if findings == nil {
				findings = []Finding{}
			}
			if !reflect.DeepEqual(r.Findings, findings) {
				t.Errorf("findings = %+v, want %+v", r.Findings, findings)
			}
			if r.RunDir == "" || r.CodexVersion != fakeVersion {
				t.Errorf("RunDir = %q, CodexVersion = %q, want both set", r.RunDir, r.CodexVersion)
			}
		})
	}
}

// TestRunStateChanged checks that every compared item codex changes is
// reported as state_changed, ahead of the run's own result.
func TestRunStateChanged(t *testing.T) {
	tests := []struct {
		name string
		// act sets up the repository and returns what the fake codex does.
		act     func(t *testing.T, fx fixture, b *codexreviewtestutil.Behavior)
		changed []string
		// refs returns the expected ChangedRefs.
		refs func(fx fixture) []string
		// shortLimit runs the row with the short time limit.
		shortLimit bool
	}{
		{
			name: "tracked_file",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				b.Writes = map[string]string{filepath.Join(fx.repo.Dir, "a.go"): "edited\n"}
			},
			changed: []string{itemStatus},
		},
		{
			name: "untracked_file_hidden_by_configuration",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				fx.repo.Git("config", "status.showUntrackedFiles", "no")
				b.Writes = map[string]string{filepath.Join(fx.repo.Dir, "new.go"): "n\n"}
			},
			changed: []string{itemStatus},
		},
		{
			name: "commit",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				b.GitDir = fx.repo.Dir
				b.Git = [][]string{{"commit", "-q", "--allow-empty", "-m", "by codex"}}
			},
			changed: []string{itemHead, itemRefs},
			refs: func(fx fixture) []string {
				return []string{"refs/heads/main (worktree " + fx.repo.Dir + ")"}
			},
		},
		{
			name: "detach_at_same_commit",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				b.GitDir = fx.repo.Dir
				b.Git = [][]string{{"switch", "-q", "--detach"}}
			},
			changed: []string{itemHead},
		},
		{
			name: "branch",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				b.GitDir = fx.repo.Dir
				b.Git = [][]string{{"branch", "made-by-codex"}}
			},
			changed: []string{itemRefs},
			refs:    func(fixture) []string { return []string{"refs/heads/made-by-codex"} },
		},
		{
			name: "deleted_branch",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				fx.repo.Git("branch", "old")
				b.GitDir = fx.repo.Dir
				b.Git = [][]string{{"branch", "-D", "old"}}
			},
			changed: []string{itemRefs},
			refs:    func(fixture) []string { return []string{"refs/heads/old"} },
		},
		{
			name: "tag",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				b.GitDir = fx.repo.Dir
				b.Git = [][]string{{"tag", "v-by-codex"}}
			},
			changed: []string{itemRefs},
			refs:    func(fixture) []string { return []string{"refs/tags/v-by-codex"} },
		},
		{
			name: "stash",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				b.Writes = map[string]string{filepath.Join(fx.repo.Dir, "a.go"): "edited\n"}
				b.GitDir = fx.repo.Dir
				b.Git = [][]string{{"stash", "push", "-q"}}
			},
			changed: []string{itemStash},
		},
		{
			name: "branch_in_another_worktree",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				other := filepath.Join(fx.fake.Dir, "other")
				fx.repo.Git("worktree", "add", "-q", "-b", "side", other)
				b.GitDir = other
				b.Git = [][]string{{"commit", "-q", "--allow-empty", "-m", "elsewhere"}}
			},
			changed: []string{itemRefs},
			refs: func(fx fixture) []string {
				return []string{"refs/heads/side (worktree " + filepath.Join(fx.fake.Dir, "other") + ")"}
			},
		},
		{
			name: "with_failed_exit",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				b.Writes = map[string]string{filepath.Join(fx.repo.Dir, "a.go"): "edited\n"}
				b.Exit = 3
			},
			changed: []string{itemStatus},
		},
		{
			name: "with_timeout",
			act: func(_ *testing.T, fx fixture, b *codexreviewtestutil.Behavior) {
				b.Writes = map[string]string{filepath.Join(fx.repo.Dir, "a.go"): "edited\n"}
				b.Hang = true
			},
			changed:    []string{itemStatus},
			shortLimit: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := 30 * time.Second
			if tt.shortLimit {
				limit = shortLimit
			}
			shortenTimeouts(t, limit, time.Second, versionTimeout, gitTimeout)
			fx := newFixture(t)
			b := completed(noFindingsMessage)
			tt.act(t, fx, &b)
			fx.fake.Set(b)
			r := mustRun(t, fx.base)
			assertKind(t, r, KindStateChanged, "")
			if !slices.Equal(r.Changed, tt.changed) {
				t.Errorf("Changed = %q, want %q", r.Changed, tt.changed)
			}
			wantRefs := []string{}
			if tt.refs != nil {
				wantRefs = tt.refs(fx)
			}
			if !slices.Equal(r.ChangedRefs, wantRefs) {
				t.Errorf("ChangedRefs = %q, want %q", r.ChangedRefs, wantRefs)
			}
		})
	}
}

// TestRunIgnoresIgnoredFiles checks that a file git ignores is not compared:
// the read-only sandbox, not the comparison, protects it.
func TestRunIgnoresIgnoredFiles(t *testing.T) {
	fx := newFixture(t)
	fx.repo.Write(".gitignore", "*.ignored\n")
	fx.repo.CommitAll("ignore a pattern")
	fx.repo.Write("old.ignored", "before\n")
	b := completed(noFindingsMessage)
	// Editing an ignored file and creating a new one: listing ignored files
	// in the comparison would show the new one.
	b.Writes = map[string]string{
		filepath.Join(fx.repo.Dir, "old.ignored"): "after\n",
		filepath.Join(fx.repo.Dir, "new.ignored"): "new\n",
	}
	fx.fake.Set(b)
	assertKind(t, mustRun(t, fx.base), KindNoFindings, "")
}

// TestRunTimeoutStopsProcessGroup checks that the timeout stops codex's
// process group, including a grandchild codex started.
func TestRunTimeoutStopsProcessGroup(t *testing.T) {
	shortenTimeouts(t, shortLimit, time.Second, versionTimeout, gitTimeout)
	fx := newFixture(t)
	b := completed(noFindingsMessage)
	b.Hang = true
	b.GrandchildPIDFile = filepath.Join(fx.fake.Dir, "grandchild.pid")
	fx.fake.Set(b)
	assertKind(t, mustRun(t, fx.base), KindTimeout, "")
	pid := codexreviewtestutil.ReadPID(t, b.GrandchildPIDFile)
	if !codexreviewtestutil.WaitGone(pid) {
		t.Errorf("grandchild %d still runs after the timeout", pid)
	}
}

// TestRunCancelStopsProcessGroup checks that canceling the run, as a signal
// does, stops codex's process group, and that the result depends only on
// whether the state changed: state_changed when it did, no result when not.
func TestRunCancelStopsProcessGroup(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change bool
	}{
		{"state_unchanged", false},
		{"state_changed", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fx := newFixture(t)
			b := completed(noFindingsMessage)
			b.Hang = true
			b.GrandchildPIDFile = filepath.Join(fx.fake.Dir, "grandchild.pid")
			if tt.change {
				b.Writes = map[string]string{filepath.Join(fx.repo.Dir, "a.go"): "edited\n"}
			}
			fx.fake.Set(b)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			type outcome struct {
				r   Result
				err error
			}
			done := make(chan outcome, 1)
			go func() {
				r, err := Run(ctx, fx.base, os.Environ())
				done <- outcome{r, err}
			}()
			pid := codexreviewtestutil.ReadPID(t, b.GrandchildPIDFile)
			cancel()
			var got outcome
			select {
			case got = <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("Run did not return after cancellation")
			}
			if !codexreviewtestutil.WaitGone(pid) {
				t.Errorf("grandchild %d still runs after cancellation", pid)
			}
			if tt.change {
				if got.err != nil {
					t.Fatalf("Run = %v, want a state_changed result", got.err)
				}
				assertKind(t, got.r, KindStateChanged, "")
				return
			}
			if !errors.Is(got.err, errInterrupted) || !reflect.DeepEqual(got.r, Result{}) {
				t.Errorf("Run = %+v, %v, want %v", got.r, got.err, errInterrupted)
			}
		})
	}
}

// TestRunAfterStateUnreadable checks that a run whose state after codex
// cannot be read returns no result.
func TestRunAfterStateUnreadable(t *testing.T) {
	fx := newFixture(t)
	b := completed(noFindingsMessage)
	b.Removes = []string{filepath.Join(fx.fake.BinDir, "git")}
	fx.fake.Set(b)
	r, err := Run(context.Background(), fx.base, os.Environ())
	if !errors.Is(err, errAfterStateUnreadable) || !reflect.DeepEqual(r, Result{}) {
		t.Errorf("Run = %+v, %v, want no result and %v", r, err, errAfterStateUnreadable)
	}
}

// TestRunInstructionsChanged checks that InstructionsChanged is set exactly
// when BASE..HEAD changes a file named like one codex reads instructions from.
func TestRunInstructionsChanged(t *testing.T) {
	tests := []struct {
		name string
		// before is committed before BASE; inRange and then later are
		// committed between BASE and HEAD.
		before, inRange, later string
		want                   bool
	}{
		{name: "nested_agents", inRange: "internal/x/AGENTS.md", want: true},
		{name: "agents_override", inRange: "AGENTS.override.md", want: true},
		{name: "claude", inRange: "CLAUDE.md", want: true},
		{name: "other_case", inRange: "sub/agents.md", want: true},
		{name: "similar_name", inRange: "docs/AGENTS.md.bak", want: false},
		{name: "before_base_only", before: "AGENTS.md", inRange: "b.go", want: false},
		{name: "earlier_commit_of_range", inRange: "AGENTS.md", later: "c.go", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := codexreviewtestutil.NewFake(t)
			repo := codexreviewtestutil.NewRepo(t)
			if tt.before != "" {
				repo.Write(tt.before, "x\n")
				repo.CommitAll("before")
			}
			base := repo.Head()
			repo.Write(tt.inRange, "y\n")
			repo.CommitAll("in range")
			if tt.later != "" {
				repo.Write(tt.later, "z\n")
				repo.CommitAll("later in range")
			}
			t.Chdir(repo.Dir)
			fake.Set(completed(noFindingsMessage))
			r := mustRun(t, base)
			assertKind(t, r, KindNoFindings, "")
			if r.InstructionsChanged != tt.want {
				t.Errorf("InstructionsChanged = %v, want %v", r.InstructionsChanged, tt.want)
			}
		})
	}
}

// TestRunDirKeepsOnlyLastMessage checks that the run directory is private to
// its owner and keeps the final message alone.
func TestRunDirKeepsOnlyLastMessage(t *testing.T) {
	fx := newFixture(t)
	message := findingOn(fx.repo.Dir+"/a.go", 1, 1)
	fx.fake.Set(completed(message))
	r := mustRun(t, fx.base)
	assertKind(t, r, KindFindings, "")
	info, err := os.Stat(r.RunDir)
	if err != nil {
		t.Fatalf("stat run dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("run dir mode = %o, want 700", perm)
	}
	entries, err := os.ReadDir(r.RunDir)
	if err != nil {
		t.Fatalf("read run dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "last-message.txt" {
		t.Fatalf("run dir holds %v, want last-message.txt alone", entries)
	}
	data, err := os.ReadFile(filepath.Join(r.RunDir, "last-message.txt"))
	if err != nil || string(data) != message {
		t.Errorf("kept final message = %q, %v, want the message codex wrote", data, err)
	}
}

// TestCheckEvents checks the rules that decide, from the JSONL output alone,
// whether a run completed. TestRunKinds shows that a broken rule makes the run
// incomplete.
func TestCheckEvents(t *testing.T) {
	lines := func(events ...string) []byte { return []byte(strings.Join(events, "\n") + "\n") }
	tests := []struct {
		name   string
		stdout []byte
		detail string
	}{
		{"completed", lines(completedEvents...), ""},
		{"blank_lines_between_events", []byte(eventTurnStarted + "\n\n" + eventTurnCompleted + "\n\n"), ""},
		{"no_turn_completed", lines(eventThreadStarted, eventTurnStarted), detailNoTurnCompleted},
		{"two_turns_completed", lines(eventTurnCompleted, eventTurnCompleted), detailManyTurnsCompleted},
		{"error_event", lines(eventThreadStarted, eventError, eventTurnCompleted), detailErrorEvent},
		{"turn_failed", lines(eventThreadStarted, eventTurnFailed, eventTurnCompleted), detailTurnFailed},
		{"not_json", lines("Reading files", eventTurnCompleted), detailUnreadableEvent},
		{"duplicate_type", lines(`{"type":"turn.completed","type":"error"}`), detailUnreadableEvent},
		{"type_not_string", lines(`{"type":1}`, eventTurnCompleted), detailUnreadableEvent},
		{"no_type", lines(`{"item":{}}`, eventTurnCompleted), detailUnreadableEvent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkEvents(tt.stdout); got != tt.detail {
				t.Errorf("checkEvents = %q, want %q", got, tt.detail)
			}
		})
	}
}

// TestRunCodexFailedDetails checks the Details of a codex run that ends
// without the tool stopping it: killed by a signal, or exited while a
// process that left its group keeps the output open.
func TestRunCodexFailedDetails(t *testing.T) {
	t.Run("signaled", func(t *testing.T) {
		fx := newFixture(t)
		b := completed(noFindingsMessage)
		b.KillSelf = true
		fx.fake.Set(b)
		assertKind(t, mustRun(t, fx.base), KindCodexFailed, detailSignaled)
	})
	t.Run("pipe_held", func(t *testing.T) {
		fx := newFixture(t)
		b := completed(noFindingsMessage)
		b.GrandchildPIDFile = filepath.Join(fx.fake.Dir, "grandchild.pid")
		b.GrandchildEscapes = true
		fx.fake.Set(b)
		r := mustRun(t, fx.base)
		// Registered after Run so the grandchild, which escaped the group,
		// is killed when the test ends.
		codexreviewtestutil.ReadPID(t, b.GrandchildPIDFile)
		assertKind(t, r, KindCodexFailed, detailPipeHeld)
	})
}

// TestRunInterruptedBeforeReview checks that a cancellation while the
// preconditions run returns no result and never starts the review.
func TestRunInterruptedBeforeReview(t *testing.T) {
	shortenTimeouts(t, codexTimeout, stopGrace, 30*time.Second, gitTimeout)
	fx := newFixture(t)
	b := completed(noFindingsMessage)
	b.VersionHang = true
	fx.fake.Set(b)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	type outcome struct {
		r   Result
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		r, err := Run(ctx, fx.base, os.Environ())
		done <- outcome{r, err}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for !slices.Equal(fx.fake.Starts(), []string{"version"}) {
		if time.Now().After(deadline) {
			t.Fatal("codex --version did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	var got outcome
	select {
	case got = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	if !errors.Is(got.err, errInterrupted) || !reflect.DeepEqual(got.r, Result{}) {
		t.Errorf("Run = %+v, %v, want no result and %v", got.r, got.err, errInterrupted)
	}
	if starts := fx.fake.Starts(); !slices.Equal(starts, []string{"version"}) {
		t.Errorf("codex starts = %q, want version alone", starts)
	}
}
