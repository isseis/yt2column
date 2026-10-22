//go:build test

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
	"github.com/isseis/yt2column/internal/fixprguard"
)

const (
	testPR       = "42"
	testReviewer = "coderabbitai"
	// testSecondReviewer is the second automated reviewer of the tests that
	// pass two --reviewer flags.
	testSecondReviewer = "chatgpt-codex-connector"
	testBranch         = "feature"
	testTime           = "2026-10-21T00:00:00Z"
	// testFile is the file the initial commit holds and the threads point at.
	testFile = "docs/notes/a.md"

	// testDirMode is the mode of directories a test creates.
	testDirMode = 0o750
	// testRandByte is the byte fixedRand repeats in tests.
	testRandByte = 0xab
	nodesKey     = "nodes"

	sameRepositoryHead = `{"isCrossRepository":false,"headRepository":{"id":"R_1","name":"yt2column"},"headRepositoryOwner":{"id":"U_1","login":"isseis"}}`
)

// testRepo is a work tree on branch feature with one commit, pushed to a
// local bare repository as origin. The test's current directory is the work
// tree, and HOME and XDG_CONFIG_HOME point at an empty directory, so the
// user's git configuration (commit signing, hooks) cannot reach the test.
// Tests that use it cannot run in parallel.
type testRepo struct {
	t   *testing.T
	Dir string
}

func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("create home: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	r := &testRepo{t: t, Dir: filepath.Join(root, "work")}
	remote := filepath.Join(root, "remote.git")
	r.gitIn(root, "init", "-q", "--bare", "--template=", remote)
	r.gitIn(root, "init", "-q", "--template=", "-b", testBranch, r.Dir)
	t.Chdir(r.Dir)
	r.Git("config", "user.name", "Test")
	r.Git("config", "user.email", "test@example.com")
	r.Git("config", "commit.gpgSign", "false")
	r.Git("remote", "add", "origin", remote)
	r.Write(testFile, "one\n")
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "initial")
	r.Git("push", "-q", "-u", "origin", testBranch)
	return r
}

// Git runs git in the work tree for the test's own setup, outside the tool,
// and returns its output.
func (r *testRepo) Git(args ...string) string {
	r.t.Helper()
	return r.gitIn(r.Dir, args...)
}

func (r *testRepo) gitIn(dir string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec // test setup runs git with fixed arguments
	cmd.Dir = dir
	cmd.Env = fixprguard.GitEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

// Write writes a file in the work tree, creating its directory.
func (r *testRepo) Write(name, content string) {
	r.t.Helper()
	path := filepath.Join(r.Dir, name)
	if err := os.MkdirAll(filepath.Dir(path), testDirMode); err != nil {
		r.t.Fatalf("create dir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // a path inside the test's temporary repository
		r.t.Fatalf("write %s: %v", name, err)
	}
}

// realGit runs git for real with the tool's environment.
func realGit() cmdrunner.Runner {
	return cmdrunner.NewOS(cmdrunner.Options{Env: fixprguard.GitEnv(os.Environ())})
}

// Baseline takes a baseline of the repository as begin would.
func (r *testRepo) Baseline() fixprguard.Baseline {
	r.t.Helper()
	base, err := fixprguard.NewWorktree(realGit(), nil).Baseline(r.t.Context())
	if err != nil {
		r.t.Fatalf("Baseline: %v", err)
	}
	return base
}

// JudgedSet is the judged set ID of the current changes, as inspect would
// print it.
func (r *testRepo) JudgedSet(base fixprguard.Baseline) fixprguard.JudgedSetID {
	r.t.Helper()
	inspection, err := fixprguard.NewWorktree(realGit(), nil).Inspect(r.t.Context(), base, nil)
	if err != nil {
		r.t.Fatalf("Inspect: %v", err)
	}
	return inspection.JudgedSet
}

// fixedRand reads as an endless run of one byte, so a test knows the nonce.
type fixedRand byte

func (r fixedRand) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

// testRun is an env whose gh and make go through fakes expecting exactly the
// scripted steps, whose git runs for real, and whose output is captured. No
// environment variable is set unless a test replaces lookupEnv.
type testRun struct {
	env
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

func newTestRun(t *testing.T, gh []cmdrunnertestutil.Step, makeSteps ...cmdrunnertestutil.Step) *testRun {
	t.Helper()
	r := &testRun{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	r.env = env{
		gh:        cmdrunnertestutil.New(t, gh...),
		git:       realGit(),
		make:      cmdrunnertestutil.New(t, makeSteps...),
		stdout:    r.Stdout,
		stderr:    r.Stderr,
		rand:      fixedRand(testRandByte),
		lookupEnv: func(string) (string, bool) { return "", false },
	}
	return r
}

// Run runs the command line and returns the exit code.
func (r *testRun) Run(t *testing.T, args ...string) int {
	t.Helper()
	return run(t.Context(), args, r.env)
}

// Decode decodes the JSON output into v.
func (r *testRun) Decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Stdout.Bytes(), v); err != nil {
		t.Fatalf("decode output %q: %v", r.Stdout.String(), err)
	}
}

// stopJSON is the stop output as a command document reads it.
type stopJSON struct {
	Stop  string   `json:"stop"`
	Parts []string `json:"parts"`
}

// beginJSON is begin's output as a command document reads it.
type beginJSON struct {
	Baseline     string            `json:"baseline"`
	ProcessCount int               `json:"processCount"`
	Untrusted    []json.RawMessage `json:"untrusted"`
	Nonce        string            `json:"nonce"`
	Threads      string            `json:"threads"`
}

// inspectJSON is inspect's output as a command document reads it.
type inspectJSON struct {
	Changes    int `json:"changes"`
	OutOfScope []struct {
		Path string `json:"path"`
	} `json:"outOfScope"`
}

// StopKind is the stop field of the output.
func (r *testRun) StopKind(t *testing.T) string {
	t.Helper()
	var out stopJSON
	r.Decode(t, &out)
	return out.Stop
}

// args prefixes a subcommand's flags with --protocol.
func args(subcommand string, rest ...string) []string {
	return append([]string{subcommand, "--" + flagProtocol, strconv.Itoa(fixprguard.Protocol)}, rest...)
}

func ghStep(out string, err error, args ...string) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: "gh", Args: args, Out: out, Err: err}
}

func headStep(out string) cmdrunnertestutil.Step {
	return ghStep(out, nil, "pr", "view", testPR, "--json", "isCrossRepository,headRepository,headRepositoryOwner")
}

func repoStep() cmdrunnertestutil.Step {
	return ghStep(`{"nameWithOwner":"isseis/yt2column"}`, nil, "repo", "view", "--json", "nameWithOwner")
}

func graphqlStep(out string) cmdrunnertestutil.Step {
	return ghStep(out, nil, "api", "graphql", "--input", "-")
}

// roleStep is a permission lookup that answers role for login.
func roleStep(login, role string) cmdrunnertestutil.Step {
	return ghStep("HTTP/2.0 200 OK\r\nContent-Type: application/json\r\n\r\n"+`{"permission":"read","role_name":"`+role+`"}`, nil,
		"api", "--include", "repos/isseis/yt2column/collaborators/"+login+"/permission")
}

// ghComment scripts one review comment; kind is the author's __typename.
type ghComment struct {
	id    int64
	kind  string
	login string
	body  string
}

// ghThread scripts one unresolved review thread on testFile.
type ghThread struct {
	id       string
	comments []ghComment
}

// reviewerThread is thread 1, opened by the reviewer bot with body.
func reviewerThread(body string) ghThread {
	return ghThread{id: testThreadID, comments: []ghComment{{id: 1, kind: "Bot", login: testReviewer, body: body}}}
}

// botThread is thread n, opened by the bot login with comment n.
func botThread(n int, login string) ghThread {
	return ghThread{id: "PRRT_t" + strconv.Itoa(n), comments: []ghComment{{id: int64(n), kind: "Bot", login: login}}}
}

// reviewerFlags is one --reviewer flag per name, in order.
func reviewerFlags(names ...string) []string {
	var flags []string
	for _, name := range names {
		flags = append(flags, "--"+flagReviewer, name)
	}
	return flags
}

// userThread is thread 1, opened by the user login.
func userThread(login string) ghThread {
	return ghThread{id: testThreadID, comments: []ghComment{{id: 1, kind: "User", login: login}}}
}

// testThreadID is the ID of the one thread a test scripts.
const testThreadID = "PRRT_t1"

func (th ghThread) json(withBody bool) map[string]any {
	nodes := make([]any, 0, len(th.comments))
	for _, c := range th.comments {
		node := map[string]any{
			"databaseId":   c.id,
			"url":          "https://github.com/isseis/yt2column/pull/42#discussion_r" + strconv.FormatInt(c.id, 10),
			"path":         testFile,
			"line":         1,
			"author":       map[string]any{"__typename": c.kind, "login": c.login},
			"createdAt":    testTime,
			"updatedAt":    testTime,
			"lastEditedAt": nil,
		}
		if withBody {
			node["body"] = c.body
		}
		nodes = append(nodes, node)
	}
	return map[string]any{
		"id":         th.id,
		"isResolved": false,
		"comments":   map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, nodesKey: nodes},
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode test response: %v", err)
	}
	return string(data)
}

// threadsPage is one stage-one page holding threads, without bodies.
func threadsPage(t *testing.T, more bool, threads ...ghThread) string {
	t.Helper()
	nodes := make([]any, 0, len(threads))
	for _, th := range threads {
		nodes = append(nodes, th.json(false))
	}
	pageInfo := map[string]any{"hasNextPage": more, "endCursor": nil}
	if more {
		pageInfo["endCursor"] = "cursor"
	}
	return mustJSON(t, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
		"reviewThreads": map[string]any{"pageInfo": pageInfo, nodesKey: nodes},
	}}}})
}

// threadBodies is a stage-two response holding threads with bodies.
func threadBodies(t *testing.T, threads ...ghThread) string {
	t.Helper()
	nodes := make([]any, 0, len(threads))
	for _, th := range threads {
		nodes = append(nodes, th.json(true))
	}
	return mustJSON(t, map[string]any{"data": map[string]any{nodesKey: nodes}})
}

// fingerprintOf is the fingerprint list prints for th, the value a session
// passes back with --thread or --fingerprint; lookups script the permission
// lookups th's authors need.
func fingerprintOf(t *testing.T, th ghThread, lookups ...cmdrunnertestutil.Step) string {
	t.Helper()
	steps := append([]cmdrunnertestutil.Step{headStep(sameRepositoryHead), repoStep(), graphqlStep(threadsPage(t, false, th))}, lookups...)
	r := newTestRun(t, steps)
	if code := r.Run(t, args(cmdList, "--reviewer", testReviewer, testPR)...); code != exitContinue {
		t.Fatalf("list exit %d: %s", code, r.Stderr)
	}
	var out struct {
		Threads []struct {
			Fingerprint string `json:"fingerprint"`
		} `json:"threads"`
	}
	r.Decode(t, &out)
	if len(out.Threads) != 1 {
		t.Fatalf("list threads = %d, want 1", len(out.Threads))
	}
	return out.Threads[0].Fingerprint
}

// accept is the --accept or --thread value naming th.
func accept(t *testing.T, th ghThread) string {
	t.Helper()
	return th.id + "=" + fingerprintOf(t, th)
}

// fenceLines splits a fenced block into its lines, failing unless it ends in
// a newline.
func fenceLines(t *testing.T, block string) []string {
	t.Helper()
	rest, ok := strings.CutSuffix(block, "\n")
	if !ok {
		t.Fatalf("block does not end in a newline: %q", block)
	}
	return strings.Split(rest, "\n")
}

// validBaseline is a baseline token that fits the grammar, for a command
// line that must fail for another reason. Its version must follow the
// package's baselineVersion, or ParseBaseline rejects it here.
func validBaseline(t *testing.T) string {
	t.Helper()
	const hexDigest, hexOID = 64, 40
	digest := strings.Repeat("a", hexDigest)
	token := strings.Join([]string{"v2", strings.Repeat("1", hexOID), digest, "none", digest, digest, digest, digest}, ".")
	if _, err := fixprguard.ParseBaseline(token); err != nil {
		t.Fatalf("test setup: baseline %q: %v", token, err)
	}
	return token
}
