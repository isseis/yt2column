//go:build test

package fixprguard

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// listSteps scripts a List call: head, repository, the stage-one pages, then
// the permission lookups.
func listSteps(pages []string, lookups ...cmdrunnertestutil.Step) []cmdrunnertestutil.Step {
	steps := []cmdrunnertestutil.Step{headStep(sameRepositoryHead), repoStep()}
	for _, page := range pages {
		steps = append(steps, graphqlStep(page))
	}
	return append(steps, lookups...)
}

func reviewerComments(n int) []testComment {
	comments := make([]testComment, n)
	for i := range comments {
		comments[i] = reviewerComment(int64(i + 1))
	}
	return comments
}

func listOne(t *testing.T, th testThread, lookups ...cmdrunnertestutil.Step) ListedThread {
	t.Helper()
	return listOneWith(t, []string{testReviewer}, th, lookups...)
}

// listOneWith is listOne with the given reviewer names.
func listOneWith(t *testing.T, reviewers []string, th testThread, lookups ...cmdrunnertestutil.Step) ListedThread {
	t.Helper()
	g, _ := newTestGitHubWith(t, reviewers, listSteps([]string{stageOneResponse(t, []testThread{th}, false, "")}, lookups...)...)
	list, err := g.List(t.Context(), mustPR(t))
	if err != nil {
		t.Fatalf("List = %v", err)
	}
	if len(list.Threads) != 1 {
		t.Fatalf("List returned %d threads, want 1", len(list.Threads))
	}
	return list.Threads[0]
}

func TestCommentCap(t *testing.T) {
	t.Run("100_comments_judged_complete", func(t *testing.T) {
		got := listOne(t, testThread{id: testThreadID(1), comments: reviewerComments(100)})
		if !got.Trusted {
			t.Errorf("thread with 100 trusted comments = %+v, want trusted", got)
		}
	})
	t.Run("more_than_100_is_incomplete", func(t *testing.T) {
		got := listOne(t, testThread{id: testThreadID(1), comments: reviewerComments(100), more: true})
		if got.Trusted || !slices.Equal(got.Findings, []Finding{{Reason: ReasonIncompleteComments}}) {
			t.Errorf("thread with more comments = %+v, want untrusted (e)", got)
		}
	})
	t.Run("comment_past_the_fiftieth_is_judged", func(t *testing.T) {
		comments := append(reviewerComments(50), userComment(51, "mallory"))
		got := listOne(t, testThread{id: testThreadID(1), comments: comments}, roleStep("mallory", "read"))
		want := []Finding{{Reason: ReasonNoWritePermission, Account: mustLogin(t, "mallory")}}
		if got.Trusted || !slices.Equal(got.Findings, want) {
			t.Errorf("thread with an untrusted 51st comment = %+v, want untrusted (a)", got)
		}
	})
	t.Run("page_over_100_threads_rejected", func(t *testing.T) {
		threads := make([]testThread, maxThreadsPerPage+1)
		for i := range threads {
			threads[i] = testThread{id: testThreadID(i), comments: reviewerComments(1)}
		}
		g, _ := newTestGitHub(t, listSteps([]string{stageOneResponse(t, threads, false, "")})...)
		if _, err := g.List(t.Context(), mustPR(t)); !errors.Is(err, ErrResponseRejected) {
			t.Errorf("List = %v, want ErrResponseRejected", err)
		}
	})
	t.Run("page_over_100_comments_rejected", func(t *testing.T) {
		th := testThread{id: testThreadID(1), comments: reviewerComments(101)}
		g, _ := newTestGitHub(t, listSteps([]string{stageOneResponse(t, []testThread{th}, false, "")})...)
		if _, err := g.List(t.Context(), mustPR(t)); !errors.Is(err, ErrResponseRejected) {
			t.Errorf("List = %v, want ErrResponseRejected", err)
		}
	})
}

func TestThreadCap(t *testing.T) {
	pages := func(t *testing.T, total int) []string {
		var out []string
		for start := 0; start < total; start += maxThreadsPerPage {
			var threads []testThread
			for i := start; i < min(start+maxThreadsPerPage, total); i++ {
				threads = append(threads, testThread{id: testThreadID(i), resolved: true, comments: reviewerComments(1)})
			}
			last := start+maxThreadsPerPage >= total
			cursor := ""
			if !last {
				cursor = "cursor"
			}
			out = append(out, stageOneResponse(t, threads, !last, cursor))
		}
		return out
	}
	t.Run("2000", func(t *testing.T) {
		g, fake := newTestGitHub(t, listSteps(pages(t, 2000))...)
		if _, err := g.List(t.Context(), mustPR(t)); err != nil {
			t.Fatalf("List = %v", err)
		}
		if after := requestVariables(t, fake.Stdin[3])["after"]; after != "cursor" {
			t.Errorf("second page after = %v, want the first page's endCursor", after)
		}
	})
	t.Run("pages_without_threads", func(t *testing.T) {
		empty := make([]string, maxListPages)
		for i := range empty {
			empty[i] = stageOneResponse(t, nil, true, "same-cursor")
		}
		g, _ := newTestGitHub(t, listSteps(empty)...)
		if _, err := g.List(t.Context(), mustPR(t)); !errors.Is(err, ErrLimitExceeded) {
			t.Errorf("List = %v, want ErrLimitExceeded", err)
		}
	})
	t.Run("2001", func(t *testing.T) {
		g, _ := newTestGitHub(t, listSteps(pages(t, 2001))...)
		if _, err := g.List(t.Context(), mustPR(t)); !errors.Is(err, ErrLimitExceeded) {
			t.Errorf("List = %v, want ErrLimitExceeded", err)
		}
	})
}

func TestResponseSizeCap(t *testing.T) {
	base := stageOneResponse(t, nil, false, "")
	padded := func(size int) string { return base + strings.Repeat(" ", size-len(base)) }
	t.Run("at_cap", func(t *testing.T) {
		g, _ := newTestGitHub(t, listSteps([]string{padded(maxResponseBytes)})...)
		if _, err := g.List(t.Context(), mustPR(t)); err != nil {
			t.Errorf("List = %v", err)
		}
	})
	t.Run("over_cap", func(t *testing.T) {
		g, _ := newTestGitHub(t, listSteps([]string{padded(maxResponseBytes + 1)})...)
		if _, err := g.List(t.Context(), mustPR(t)); !errors.Is(err, ErrResponseRejected) {
			t.Errorf("List = %v, want ErrResponseRejected", err)
		}
	})
	// The runner's own cap (set by the command) fails the call with
	// ErrOutputTooLarge before any output reaches the parser.
	t.Run("runner_cap", func(t *testing.T) {
		step := graphqlStep("")
		step.Err = cmdrunner.ErrOutputTooLarge
		g, _ := newTestGitHub(t, headStep(sameRepositoryHead), repoStep(), step)
		if _, err := g.List(t.Context(), mustPR(t)); !errors.Is(err, ErrResponseRejected) {
			t.Errorf("List = %v, want ErrResponseRejected", err)
		}
	})
}

func TestResponseRejected(t *testing.T) {
	base := stageOneResponse(t, []testThread{{id: testThreadID(1), comments: []testComment{reviewerComment(7)}}}, false, "")
	// Each row replaces one fragment of the valid base response.
	for _, tc := range []struct {
		name, old, new string
	}{
		{"errors_with_data", `{"data":`, `{"errors":[{"message":"x"}],"data":`},
		{"errors_null", `{"data":`, `{"errors":null,"data":`},
		{"data_null", `{"data":{"repository"`, `{"data":null,"x":{"repository"`},
		{"pull_request_null", `"pullRequest":{`, `"pullRequest":null,"x":{`},
		{"duplicate_comment_member", `"databaseId":7`, `"databaseId":7,"databaseId":8`},
		{"duplicate_author_member", `"login":"coderabbitai"`, `"login":"coderabbitai","login":"mallory"`},
		{"duplicate_thread_member", `"isResolved":false`, `"isResolved":false,"isResolved":true`},
		{"database_id_null", `"databaseId":7`, `"databaseId":null`},
		{"database_id_zero", `"databaseId":7`, `"databaseId":0`},
		{"database_id_negative", `"databaseId":7`, `"databaseId":-7`},
		{"database_id_fraction", `"databaseId":7`, `"databaseId":7.5`},
		{"url_null", `"url":"` + testCommentURL(7) + `"`, `"url":null`},
		{"url_not_github", `"url":"https://github.com/`, `"url":"https://evil.example/`},
		{"url_with_space", `#discussion_r7"`, `#discussion r7"`},
		{"path_null", `"path":"internal/fixprguard/github.go"`, `"path":null`},
		{"created_null", `"createdAt":"` + testTime + `"`, `"createdAt":null`},
		{"updated_missing", `"updatedAt":"` + testTime + `",`, ``},
		{"last_edited_empty", `"lastEditedAt":null`, `"lastEditedAt":""`},
		{"author_missing", `"author":{"__typename":"Bot","login":"coderabbitai"},`, ``},
		{"author_string", `"author":{"__typename":"Bot","login":"coderabbitai"}`, `"author":"coderabbitai"`},
		{"is_resolved_string", `"isResolved":false`, `"isResolved":"false"`},
		{"thread_id_path", `"id":"PRRT_t1"`, `"id":"PRRT_t1/../x"`},
		{"comments_missing", `"comments":{`, `"x":{`},
		{"has_next_page_without_cursor", `"endCursor":null,"hasNextPage":false`, `"endCursor":null,"hasNextPage":true`},
		{"trailing_data", base, base + `{}`},
		{"invalid_utf8", `"coderabbitai"`, "\"coderabbit\xffai\""},
		{"whole_response_null", base, `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(base, tc.old) {
				t.Fatalf("base response does not contain %q: %s", tc.old, base)
			}
			out := strings.Replace(base, tc.old, tc.new, 1)
			g, _ := newTestGitHub(t, listSteps([]string{out})...)
			if _, err := g.List(t.Context(), mustPR(t)); !errors.Is(err, ErrResponseRejected) {
				t.Errorf("List(%s) = %v, want ErrResponseRejected", out, err)
			}
		})
	}
	t.Run("base_accepted", func(t *testing.T) {
		g, _ := newTestGitHub(t, listSteps([]string{base})...)
		if _, err := g.List(t.Context(), mustPR(t)); err != nil {
			t.Errorf("List(base) = %v", err)
		}
	})
}

func TestFingerprintChanges(t *testing.T) {
	base := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1), userComment(2, "alice")}}
	variants := map[string]testThread{
		"base":            base,
		"other_thread":    {id: testThreadID(2), comments: base.comments},
		"added":           {id: base.id, comments: append(slices.Clone(base.comments), reviewerComment(3))},
		"deleted":         {id: base.id, comments: base.comments[:1]},
		"edited":          {id: base.id, comments: []testComment{reviewerComment(1), {id: 2, kind: typenameUser, login: "alice", edited: testLaterAt}}},
		"updated":         {id: base.id, comments: []testComment{reviewerComment(1), {id: 2, kind: typenameUser, login: "alice", updated: testLaterAt}}},
		"author_changed":  {id: base.id, comments: []testComment{reviewerComment(1), userComment(2, "mallory")}},
		"author_deleted":  {id: base.id, comments: []testComment{reviewerComment(1), {id: 2, noAuthor: true}}},
		"more_comments":   {id: base.id, comments: base.comments, more: true},
		"kind_changed":    {id: base.id, comments: []testComment{reviewerComment(1), botComment(2, "alice")}},
		"created_changed": {id: base.id, comments: []testComment{reviewerComment(1), {id: 2, kind: typenameUser, login: "alice", created: testLaterAt}}},
		"author_emptied":  {id: base.id, comments: []testComment{reviewerComment(1), {id: 2, omitKind: true, omitLogin: true}}},
	}
	seen := map[Fingerprint]string{}
	for name, th := range variants {
		fp := testFingerprint(t, th)
		if !validFingerprint(fp) {
			t.Errorf("%s: fingerprint %q is not a valid fingerprint", name, fp)
		}
		if other, dup := seen[fp]; dup {
			t.Errorf("%s and %s share a fingerprint", name, other)
		}
		seen[fp] = name
	}
}

func TestStageOneQueryHasNoBody(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	g, fake := newTestGitHub(t, listSteps([]string{stageOneResponse(t, []testThread{th}, false, "")})...)
	if _, err := g.List(t.Context(), mustPR(t)); err != nil {
		t.Fatalf("List = %v", err)
	}
	var request struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(fake.Stdin[2], &request); err != nil {
		t.Fatalf("decode stage-one request: %v", err)
	}
	if !strings.Contains(request.Query, "reviewThreads") {
		t.Fatalf("third call is not the stage-one query: %s", request.Query)
	}
	if strings.Contains(strings.ToLower(request.Query), "body") {
		t.Errorf("stage-one query requests a body field: %s", request.Query)
	}
}
