//go:build test

package fixprguard

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// writeTextFile writes content to a new file under a temporary directory and
// returns its path.
func writeTextFile(t *testing.T, content string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "text.md")
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestBodyFileChecks(t *testing.T) {
	symlink := func(t *testing.T) string {
		target := writeTextFile(t, "reply")
		link := filepath.Join(t.TempDir(), "link.md")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		return link
	}
	fifo := func(t *testing.T) string {
		name := filepath.Join(t.TempDir(), "fifo")
		if err := syscall.Mkfifo(name, 0o600); err != nil {
			t.Fatal(err)
		}
		return name
	}
	file := func(content string) func(*testing.T) string {
		return func(t *testing.T) string { return writeTextFile(t, content) }
	}
	atCap := strings.Repeat("a", maxTextFileBytes)
	for _, tc := range []struct {
		name    string
		path    func(*testing.T) string
		title   bool
		wantErr bool
	}{
		{name: "regular_file", path: file("Fixed in the next commit.\n")},
		{name: "at_size_cap", path: file(atCap)},
		{name: "over_size_cap", path: file(atCap + "a"), wantErr: true},
		{name: "symbolic_link", path: symlink, wantErr: true},
		{name: "fifo", path: fifo, wantErr: true},
		{name: "directory", path: func(t *testing.T) string { return t.TempDir() }, wantErr: true},
		{name: "missing", path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "none") }, wantErr: true},
		{name: "invalid_utf8", path: file("ok \xff"), wantErr: true},
		{name: "empty", path: file(""), wantErr: true},
		{name: "title_one_line", path: file("fix: a title"), title: true},
		{name: "title_newline", path: file("fix: a title\n"), title: true, wantErr: true},
		{name: "title_two_lines", path: file("fix: a title\nmore"), title: true, wantErr: true},
		{name: "title_carriage_return", path: file("fix: a\rtitle"), title: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := tc.path(t)
			read := readTextFile
			if tc.title {
				read = readTitleFile
			}
			got, err := read(name)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidInput) {
					t.Errorf("read = %v, want ErrInvalidInput", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("read = %v", err)
			}
			want, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("read returned %d bytes, want the file's %d bytes", len(got), len(want))
			}
		})
	}
}

// resolvedResponse is the resolveReviewThread response for id.
func resolvedResponse(t *testing.T, id string, isResolved bool) string {
	t.Helper()
	return mustJSON(t, map[string]any{"data": map[string]any{"resolveReviewThread": map[string]any{
		"thread": map[string]any{"id": id, "isResolved": isResolved},
	}}})
}

// replyLookupSteps scripts the reads of a Reply call: the repository, the
// stage-one page holding threads, and the permission lookups.
func replyLookupSteps(t *testing.T, threads []testThread, lookups ...cmdrunnertestutil.Step) []cmdrunnertestutil.Step {
	t.Helper()
	return append([]cmdrunnertestutil.Step{repoStep(), graphqlStep(stageOneResponse(t, threads, false, ""))}, lookups...)
}

// postStep is the reply to comment 1, the first comment of the threads the
// reply tests script.
func postStep() cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: "gh", Args: []string{
		"api", "--method", "POST", "repos/" + testOwner + "/" + testRepoName + "/pulls/" + testPR + "/comments/1/replies", "--input", "-",
	}}
}

// replyRequest replies to the first comment of th with the fingerprint the
// tool computes for it.
func replyRequest(t *testing.T, th testThread, body string) ReplyRequest {
	t.Helper()
	return ReplyRequest{
		PR:          mustPR(t),
		ThreadID:    th.id,
		CommentID:   th.comments[0].id,
		Fingerprint: testFingerprint(t, th),
		BodyFile:    writeTextFile(t, body),
	}
}

func TestReplyRefusesUntrustedThread(t *testing.T) {
	// Another bot needs no permission lookup, so the only failing condition is
	// trust: the fingerprint and the comment match.
	th := testThread{id: testThreadID(1), comments: []testComment{botComment(1, "other-bot")}}
	t.Run("not_named", func(t *testing.T) {
		g, _ := newTestGitHub(t, replyLookupSteps(t, []testThread{th})...)
		if err := g.Reply(t.Context(), replyRequest(t, th, "reply")); !errors.Is(err, ErrThreadNotTrusted) {
			t.Errorf("Reply = %v, want ErrThreadNotTrusted", err)
		}
	})
	t.Run("named_with_current_fingerprint", func(t *testing.T) {
		g, _ := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}),
			postStep(), graphqlStep(resolvedResponse(t, th.id, true)))...)
		req := replyRequest(t, th, "reply")
		req.Accept = []Accept{{ThreadID: th.id, Fingerprint: req.Fingerprint}}
		if err := g.Reply(t.Context(), req); err != nil {
			t.Errorf("Reply = %v, want the named thread answered", err)
		}
	})
}

func TestReplyReassesses(t *testing.T) {
	// The same thread with the same comments: only alice's permission changes
	// between the two calls.
	th := testThread{id: testThreadID(1), comments: []testComment{userComment(1, "alice")}}
	g, _ := newTestGitHub(t, slices.Concat(
		replyLookupSteps(t, []testThread{th}, roleStep("alice", "write")),
		[]cmdrunnertestutil.Step{postStep(), graphqlStep(resolvedResponse(t, th.id, true))},
		replyLookupSteps(t, []testThread{th}, roleStep("alice", "read")),
	)...)
	if err := g.Reply(t.Context(), replyRequest(t, th, "first")); err != nil {
		t.Fatalf("first Reply = %v", err)
	}
	if err := g.Reply(t.Context(), replyRequest(t, th, "second")); !errors.Is(err, ErrThreadNotTrusted) {
		t.Errorf("second Reply = %v, want ErrThreadNotTrusted after write access was lost", err)
	}
}

func TestReplyRefusesChangedFingerprint(t *testing.T) {
	trusted := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	named := testThread{id: testThreadID(2), comments: []testComment{botComment(2, "other-bot")}}
	resolved := testThread{id: testThreadID(3), resolved: true, comments: []testComment{reviewerComment(3)}}
	threads := []testThread{trusted, named, resolved}
	for name, req := range map[string]func() ReplyRequest{
		"trusted_fingerprint_differs": func() ReplyRequest {
			req := replyRequest(t, trusted, "reply")
			req.Fingerprint = testFP
			return req
		},
		"named_accept_fingerprint_differs": func() ReplyRequest {
			req := replyRequest(t, named, "reply")
			req.Accept = []Accept{{ThreadID: named.id, Fingerprint: testFP}}
			return req
		},
		"resolved_thread": func() ReplyRequest { return replyRequest(t, resolved, "reply") },
		"unknown_thread": func() ReplyRequest {
			req := replyRequest(t, trusted, "reply")
			req.ThreadID = "PRRT_gone"
			return req
		},
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, replyLookupSteps(t, threads)...)
			if err := g.Reply(t.Context(), req()); !errors.Is(err, ErrThreadChanged) {
				t.Errorf("Reply = %v, want ErrThreadChanged", err)
			}
		})
	}
}

func TestReplyRefusesForeignComment(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	other := testThread{id: testThreadID(2), comments: []testComment{reviewerComment(2)}}
	g, _ := newTestGitHub(t, replyLookupSteps(t, []testThread{th, other})...)
	req := replyRequest(t, th, "reply")
	req.CommentID = other.comments[0].id
	if err := g.Reply(t.Context(), req); !errors.Is(err, ErrCommentNotInThread) {
		t.Errorf("Reply = %v, want ErrCommentNotInThread", err)
	}
}

func TestReplyRejectsInputBeforeAnyCall(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	for name, edit := range map[string]func(*ReplyRequest){
		"zero_pr":           func(r *ReplyRequest) { r.PR = PRNumber{} },
		"thread_id_url":     func(r *ReplyRequest) { r.ThreadID = "https://github.com/x" },
		"zero_comment_id":   func(r *ReplyRequest) { r.CommentID = 0 },
		"short_fingerprint": func(r *ReplyRequest) { r.Fingerprint = "abc" },
		"bad_accept":        func(r *ReplyRequest) { r.Accept = []Accept{{ThreadID: "PRRT_x/..", Fingerprint: testFP}} },
		"missing_body_file": func(r *ReplyRequest) { r.BodyFile = filepath.Join(t.TempDir(), "none") },
	} {
		t.Run(name, func(t *testing.T) {
			// The fake holds no step, so any gh call fails the test.
			g, _ := newTestGitHub(t)
			req := replyRequest(t, th, "reply")
			edit(&req)
			if err := g.Reply(t.Context(), req); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Reply = %v, want ErrInvalidInput", err)
			}
		})
	}
}

// trickyTexts are texts a shell would execute or end early, and values gh's -F
// would convert to another type.
var trickyTexts = map[string]string{
	"heredoc_terminator": "fixed\nREPLYBODY_EOF\nrm -rf ~\n",
	"command_subst":      "see $(touch /tmp/pwned) here",
	"backticks":          "run `id` now",
	"quotes":             `it's "quoted" \' end`,
	"at_file":            "@/etc/passwd",
	"true":               "true",
	"null":               "null",
	"leading_dash":       "--help",
}

func TestReplyBodyVerbatim(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	for name, body := range trickyTexts {
		t.Run(name, func(t *testing.T) {
			g, fake := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}),
				postStep(), graphqlStep(resolvedResponse(t, th.id, true)))...)
			if err := g.Reply(t.Context(), replyRequest(t, th, body)); err != nil {
				t.Fatalf("Reply = %v", err)
			}
			posted := fake.Stdin[len(fake.Stdin)-2]
			var request map[string]any
			if err := json.Unmarshal(posted, &request); err != nil {
				t.Fatalf("reply request is not JSON: %v", err)
			}
			if got, ok := request["body"].(string); !ok || got != body || len(request) != 1 {
				t.Errorf("reply request = %s, want only body %q", posted, body)
			}
			if got := requestVariables(t, fake.Stdin[len(fake.Stdin)-1])["threadId"]; got != th.id {
				t.Errorf("resolved thread %v, want %s", got, th.id)
			}
		})
	}
}

func TestEditPRVerbatim(t *testing.T) {
	for name, text := range trickyTexts {
		if strings.Contains(text, "\n") {
			continue // a title is one line; the body case below covers it
		}
		t.Run(name, func(t *testing.T) {
			body := "## Summary\n" + text + "\nREPLYBODY_EOF\n"
			fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{Name: "gh", Args: []string{"pr", "edit", testPR, "--title=" + text, "--body-file", "-"}})
			g, err := NewGitHub(fake, mustLogin(t, testReviewer))
			if err != nil {
				t.Fatal(err)
			}
			req := EditPRRequest{PR: mustPR(t), TitleFile: writeTextFile(t, text), BodyFile: writeTextFile(t, body)}
			if err := g.EditPR(t.Context(), req); err != nil {
				t.Fatalf("EditPR = %v", err)
			}
			if got := string(fake.Stdin[0]); got != body {
				t.Errorf("body on standard input = %q, want %q", got, body)
			}
		})
	}
	t.Run("rejects_before_any_call", func(t *testing.T) {
		g, _ := newTestGitHub(t)
		for name, req := range map[string]EditPRRequest{
			"zero_pr":         {TitleFile: writeTextFile(t, "title"), BodyFile: writeTextFile(t, "body")},
			"two_line_title":  {PR: mustPR(t), TitleFile: writeTextFile(t, "a\nb"), BodyFile: writeTextFile(t, "body")},
			"empty_body_file": {PR: mustPR(t), TitleFile: writeTextFile(t, "title"), BodyFile: writeTextFile(t, "")},
		} {
			if err := g.EditPR(t.Context(), req); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("%s: EditPR = %v, want ErrInvalidInput", name, err)
			}
		}
	})
	t.Run("gh_failure", func(t *testing.T) {
		fake := cmdrunnertestutil.New(t, cmdrunnertestutil.Step{
			Name: "gh", Args: []string{"pr", "edit", testPR, "--title=title", "--body-file", "-"}, Err: &cmdrunner.ExitError{Code: ghExitFailure},
		})
		g, err := NewGitHub(fake, mustLogin(t, testReviewer))
		if err != nil {
			t.Fatal(err)
		}
		req := EditPRRequest{PR: mustPR(t), TitleFile: writeTextFile(t, "title"), BodyFile: writeTextFile(t, "body")}
		if err := g.EditPR(t.Context(), req); !errors.Is(err, ErrResponseRejected) {
			t.Errorf("EditPR = %v, want ErrResponseRejected", err)
		}
	})
}

func TestReplyPostFailureSkipsResolve(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	post := postStep()
	post.Err = &cmdrunner.ExitError{Code: ghExitFailure}
	// No resolve step is scripted, so resolving after the failure fails the test.
	g, _ := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}), post)...)
	err := g.Reply(t.Context(), replyRequest(t, th, "reply"))
	if !errors.Is(err, ErrResponseRejected) || errors.Is(err, ErrReplyNotResolved) {
		t.Errorf("Reply = %v, want ErrResponseRejected without ErrReplyNotResolved", err)
	}
}

func TestReplyResolveFailure(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	failed := graphqlStep("")
	failed.Err = &cmdrunner.ExitError{Code: ghExitFailure}
	for name, resolve := range map[string]cmdrunnertestutil.Step{
		"gh_fails":       failed,
		"not_resolved":   graphqlStep(resolvedResponse(t, th.id, false)),
		"other_thread":   graphqlStep(resolvedResponse(t, testThreadID(2), true)),
		"graphql_errors": graphqlStep(`{"data":null,"errors":[{"message":"x"}]}`),
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}), postStep(), resolve)...)
			err := g.Reply(t.Context(), replyRequest(t, th, "reply"))
			if !errors.Is(err, ErrReplyNotResolved) || StopKind(err) != StopReplyNotResolved {
				t.Errorf("Reply = %v, want ErrReplyNotResolved", err)
			}
		})
	}
}
