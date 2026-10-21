//go:build test

package fixprguard

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

const (
	testOwner    = "isseis"
	testRepoName = "yt2column"
	testPR       = "42"
	testReviewer = "coderabbitai"
	testTime     = "2026-10-21T00:00:00Z"
	testLaterAt  = "2026-10-21T01:00:00Z"

	nodesKey = "nodes"

	// ghExitFailure is the exit code gh uses for a failed request.
	ghExitFailure = 1

	sameRepositoryHead = `{"isCrossRepository":false,"headRepository":{"id":"R_1","name":"yt2column"},"headRepositoryOwner":{"id":"U_1","login":"isseis"}}`
	repositoryView     = `{"nameWithOwner":"isseis/yt2column"}`
)

// testComment scripts one comment of a GitHub response. kind is the author's
// __typename; noAuthor writes author: null; omitKind and omitLogin leave the
// member out. edited is the lastEditedAt value, null when empty.
type testComment struct {
	id        int64
	kind      string
	login     string
	noAuthor  bool
	omitKind  bool
	omitLogin bool
	updated   string
	edited    string
	body      string
}

func userComment(id int64, login string) testComment {
	return testComment{id: id, kind: typenameUser, login: login}
}

func botComment(id int64, login string) testComment {
	return testComment{id: id, kind: typenameBot, login: login}
}

// reviewerComment is a comment by the trusted reviewer bot, which needs no
// permission lookup.
func reviewerComment(id int64) testComment {
	return botComment(id, testReviewer)
}

// testThread scripts one review thread; more sets the comments' hasNextPage.
type testThread struct {
	id       string
	resolved bool
	comments []testComment
	more     bool
}

func testThreadID(n int) string { return "PRRT_t" + strconv.Itoa(n) }

func testCommentURL(id int64) string {
	return "https://github.com/isseis/yt2column/pull/42#discussion_r" + strconv.FormatInt(id, 10)
}

func (c testComment) json(withBody bool) map[string]any {
	node := map[string]any{
		"databaseId":   c.id,
		"url":          testCommentURL(c.id),
		"path":         "internal/fixprguard/github.go",
		"line":         1,
		"createdAt":    testTime,
		"updatedAt":    testTime,
		"lastEditedAt": nil,
	}
	if c.updated != "" {
		node["updatedAt"] = c.updated
	}
	if c.edited != "" {
		node["lastEditedAt"] = c.edited
	}
	if c.noAuthor {
		node["author"] = nil
	} else {
		a := map[string]any{}
		if !c.omitKind {
			a["__typename"] = c.kind
		}
		if !c.omitLogin {
			a["login"] = c.login
		}
		node["author"] = a
	}
	if withBody {
		node["body"] = c.body
	}
	return node
}

func (th testThread) json(withBody bool) map[string]any {
	comments := make([]any, 0, len(th.comments))
	for _, c := range th.comments {
		comments = append(comments, c.json(withBody))
	}
	return map[string]any{
		"id":         th.id,
		"isResolved": th.resolved,
		"comments": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": th.more},
			nodesKey:   comments,
		},
	}
}

func mustJSON(t testing.TB, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode test response: %v", err)
	}
	return string(data)
}

// stageOneResponse is one stage-one page holding threads.
func stageOneResponse(t testing.TB, threads []testThread, hasNextPage bool, cursor string) string {
	t.Helper()
	nodes := make([]any, 0, len(threads))
	for _, th := range threads {
		nodes = append(nodes, th.json(false))
	}
	pageInfo := map[string]any{"hasNextPage": hasNextPage, "endCursor": nil}
	if cursor != "" {
		pageInfo["endCursor"] = cursor
	}
	return mustJSON(t, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
		"reviewThreads": map[string]any{"pageInfo": pageInfo, nodesKey: nodes},
	}}}})
}

// stageTwoResponse is a stage-two response holding threads with bodies.
func stageTwoResponse(t testing.TB, threads ...testThread) string {
	t.Helper()
	nodes := make([]any, 0, len(threads))
	for _, th := range threads {
		nodes = append(nodes, th.json(true))
	}
	return mustJSON(t, map[string]any{"data": map[string]any{nodesKey: nodes}})
}

// permissionResponse is gh api --include output for a successful lookup.
func permissionResponse(role string) string {
	return "HTTP/2.0 200 OK\r\nContent-Type: application/json; charset=utf-8\r\n\r\n" +
		`{"permission":"read","role_name":"` + role + `","user":{"login":"x"}}`
}

// permissionFailure is the error gh api --include returns for an HTTP status.
func permissionFailure(status int) error {
	return &cmdrunner.ExitError{
		Code:   ghExitFailure,
		Stdout: fmt.Appendf(nil, "HTTP/2.0 %d Error\r\nContent-Type: application/json\r\n\r\n{\"message\":\"x\"}", status),
		Stderr: []byte("gh: request failed"),
	}
}

func headStep(out string) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: "gh", Args: []string{"pr", "view", testPR, "--json", "isCrossRepository,headRepository,headRepositoryOwner"}, Out: out}
}

func repoStep() cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: "gh", Args: []string{"repo", "view", "--json", "nameWithOwner"}, Out: repositoryView}
}

func graphqlStep(out string) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: "gh", Args: []string{"api", "graphql", "--input", "-"}, Out: out}
}

func permissionStep(login, out string, err error) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{
		Name: "gh",
		Args: []string{"api", "--include", "repos/" + testOwner + "/" + testRepoName + "/collaborators/" + login + "/permission"},
		Out:  out,
		Err:  err,
	}
}

// roleStep is a successful permission lookup returning role.
func roleStep(login, role string) cmdrunnertestutil.Step {
	return permissionStep(login, permissionResponse(role), nil)
}

// newTestGitHub returns a GitHub over a fake that expects exactly steps, with
// the test reviewer.
func newTestGitHub(t *testing.T, steps ...cmdrunnertestutil.Step) (*GitHub, *cmdrunnertestutil.Fake) {
	t.Helper()
	fake := cmdrunnertestutil.New(t, steps...)
	g, err := NewGitHub(fake, mustLogin(t, testReviewer))
	if err != nil {
		t.Fatalf("NewGitHub: %v", err)
	}
	return g, fake
}

func mustLogin(t testing.TB, s string) Login {
	t.Helper()
	login, err := ParseLogin(s)
	if err != nil {
		t.Fatalf("ParseLogin(%q): %v", s, err)
	}
	return login
}

func mustPR(t testing.TB) PRNumber {
	t.Helper()
	pr, err := ParsePRNumber(testPR)
	if err != nil {
		t.Fatalf("ParsePRNumber: %v", err)
	}
	return pr
}

// testFingerprint is the fingerprint the tool computes for th, the value a user
// is shown and names the thread with.
func testFingerprint(t testing.TB, th testThread) Fingerprint {
	t.Helper()
	parsed, err := parseThreadPage([]byte(stageOneResponse(t, []testThread{th}, false, "")))
	if err != nil || len(parsed.threads) != 1 {
		t.Fatalf("parse test thread: %v", err)
	}
	return fingerprint(parsed.threads[0])
}

// requestVariables decodes the variables of the GraphQL request a fake call
// received on standard input.
func requestVariables(t testing.TB, stdin []byte) map[string]any {
	t.Helper()
	var request struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(stdin, &request); err != nil {
		t.Fatalf("decode GraphQL request: %v", err)
	}
	return request.Variables
}
