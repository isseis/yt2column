//go:build test

package fixprguard

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
		{name: "directory", path: func(t *testing.T) string { return t.TempDir() }, wantErr: true},
		{name: "missing", path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "none") }, wantErr: true},
		{name: "invalid_utf8", path: file("ok \xff"), wantErr: true},
		{name: "empty", path: file(""), wantErr: true},
		{name: "blank", path: file(" \n\t\n"), wantErr: true},
		{name: "nul_byte", path: file("reply\x00more"), wantErr: true},
		{name: "title_one_line", path: file("fix: a title"), title: true},
		{name: "title_newline", path: file("fix: a title\n"), title: true, wantErr: true},
		{name: "title_two_lines", path: file("fix: a title\nmore"), title: true, wantErr: true},
		{name: "title_carriage_return", path: file("fix: a\rtitle"), title: true, wantErr: true},
		{name: "title_escape", path: file("fix: \x1b[31mred"), title: true, wantErr: true},
		{name: "title_c1_next_line", path: file("fix: a\u0085title"), title: true, wantErr: true},
		{name: "title_line_separator", path: file("fix: a\u2028title"), title: true, wantErr: true},
		{name: "title_delete", path: file("fix: a\x7ftitle"), title: true, wantErr: true},
		{name: "title_paragraph_separator", path: file("fix: a\u2029title"), title: true, wantErr: true},
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

// postedID is the database ID GitHub gives the reply in these tests.
const postedID = 500

// postStep is the reply to comment 1, the first comment of the threads the
// reply tests script; GitHub answers with the new comment.
func postStep() cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: "gh", Args: []string{
		"api", "--method", "POST", "repos/" + testOwner + "/" + testRepoName + "/pulls/" + testPR + "/comments/1/replies", "--input", "-",
	}, Out: `{"id":500,"body":"x"}`}
}

// withReply is th after the tool's reply was posted.
func withReply(th testThread) testThread {
	th.comments = append(slices.Clone(th.comments), userComment(postedID, testOwner))
	return th
}

// writeSteps scripts the writes of a successful Reply to th: the post, the
// fresh fetch showing only the reply added, and the resolve.
func writeSteps(t *testing.T, th testThread) []cmdrunnertestutil.Step {
	t.Helper()
	return []cmdrunnertestutil.Step{
		postStep(),
		graphqlStep(stageOneResponse(t, []testThread{withReply(th)}, false, "")),
		graphqlStep(resolvedResponse(t, th.id, true)),
	}
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
		g, _ := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}), writeSteps(t, th)...)...)
		req := replyRequest(t, th, "reply")
		req.Accept = []Accept{{ThreadID: th.id, Fingerprint: req.Fingerprint}}
		if err := g.Reply(t.Context(), req); err != nil {
			t.Errorf("Reply = %v, want the named thread answered", err)
		}
	})
}

// TestReplyTrustsEveryReviewer: with two reviewers, the second reviewer's
// thread is answered without being named.
func TestReplyTrustsEveryReviewer(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{botComment(1, testSecondReviewer)}}
	g, _ := newTestGitHubWith(t, bothReviewers, append(replyLookupSteps(t, []testThread{th}), writeSteps(t, th)...)...)
	if err := g.Reply(t.Context(), replyRequest(t, th, "reply")); err != nil {
		t.Errorf("Reply = %v, want the second reviewer's thread answered", err)
	}
}

func TestReplyReassesses(t *testing.T) {
	// The same thread with the same comments: only alice's permission changes
	// between the two calls.
	th := testThread{id: testThreadID(1), comments: []testComment{userComment(1, "alice")}}
	// The second call deliberately sees the thread as before the first reply,
	// so the role is the only difference between the two calls.
	g, _ := newTestGitHub(t, slices.Concat(
		replyLookupSteps(t, []testThread{th}, roleStep("alice", "write")),
		writeSteps(t, th),
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
	threads := []testThread{trusted, named}
	for name, req := range map[string]func(*testing.T) ReplyRequest{
		"trusted_fingerprint_differs": func(t *testing.T) ReplyRequest {
			req := replyRequest(t, trusted, "reply")
			req.Fingerprint = testFP
			return req
		},
		"named_accept_fingerprint_differs": func(t *testing.T) ReplyRequest {
			req := replyRequest(t, named, "reply")
			req.Accept = []Accept{{ThreadID: named.id, Fingerprint: testFP}}
			return req
		},
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, replyLookupSteps(t, threads)...)
			if err := g.Reply(t.Context(), req(t)); !errors.Is(err, ErrThreadChanged) {
				t.Errorf("Reply = %v, want ErrThreadChanged", err)
			}
		})
	}
}

// TestReplyRefusesMissingThread covers threads with no comment to answer: a
// resolved or unknown thread is absent from the unresolved threads, and an
// empty one is present without comments. An absent thread reads as the zero
// thread, so the empty-thread check refuses all three; the empty row carries
// its own fingerprint, so no later check can refuse it instead.
func TestReplyRefusesMissingThread(t *testing.T) {
	resolved := testThread{id: testThreadID(1), resolved: true, comments: []testComment{reviewerComment(1)}}
	empty := testThread{id: testThreadID(2)}
	for name, th := range map[string]testThread{
		"resolved_thread": resolved,
		"unknown_thread":  {id: "PRRT_gone", comments: []testComment{reviewerComment(3)}},
		"empty_thread":    empty,
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, replyLookupSteps(t, []testThread{resolved, empty})...)
			req := ReplyRequest{
				PR: mustPR(t), ThreadID: th.id, CommentID: 1,
				Fingerprint: testFingerprint(t, th), BodyFile: writeTextFile(t, "reply"),
			}
			if err := g.Reply(t.Context(), req); !errors.Is(err, ErrThreadChanged) {
				t.Errorf("Reply = %v, want ErrThreadChanged", err)
			}
		})
	}
}

func TestReplyRefusesForeignComment(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1), reviewerComment(3)}}
	other := testThread{id: testThreadID(2), comments: []testComment{reviewerComment(2)}}
	for name, id := range map[string]int64{
		"other_threads_comment": other.comments[0].id,
		// GitHub's replies endpoint takes only a thread's first comment.
		"not_first_comment": th.comments[1].id,
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, replyLookupSteps(t, []testThread{th, other})...)
			req := replyRequest(t, th, "reply")
			req.CommentID = id
			if err := g.Reply(t.Context(), req); !errors.Is(err, ErrCommentNotInThread) {
				t.Errorf("Reply = %v, want ErrCommentNotInThread", err)
			}
		})
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
		"missing_body_file": func(r *ReplyRequest) { r.BodyFile = filepath.Join(filepath.Dir(r.BodyFile), "none") },
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

func TestReplyReadFailurePropagates(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	failed := graphqlStep("")
	failed.Err = &cmdrunner.ExitError{Code: ghExitFailure}
	// No write step is scripted, so a post after the failure fails the test.
	g, _ := newTestGitHub(t, repoStep(), failed)
	if err := g.Reply(t.Context(), replyRequest(t, th, "reply")); !errors.Is(err, ErrResponseRejected) {
		t.Errorf("Reply = %v, want ErrResponseRejected", err)
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

// TestReplyBodyVerbatim feeds the texts through the one path a body takes: a
// fixed argv and a JSON request on standard input. The rows illustrate what that
// path must carry unchanged; they do not each guard a separate mechanism, since
// the fake runs no shell. A shell or a -F/-f flag would fail every row through
// the argv, and any change to the text fails the rows it alters.
func TestReplyBodyVerbatim(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	lookups := replyLookupSteps(t, []testThread{th})
	for name, body := range trickyTexts {
		t.Run(name, func(t *testing.T) {
			writes := writeSteps(t, th)
			g, fake := newTestGitHub(t, append(slices.Clone(lookups), writes...)...)
			if err := g.Reply(t.Context(), replyRequest(t, th, body)); err != nil {
				t.Fatalf("Reply = %v", err)
			}
			posted := fake.Stdin[len(lookups)]
			var request map[string]any
			if err := json.Unmarshal(posted, &request); err != nil {
				t.Fatalf("reply request is not JSON: %v", err)
			}
			if got, ok := request["body"].(string); !ok || got != body || len(request) != 1 {
				t.Errorf("reply request = %s, want only body %q", posted, body)
			}
			if got := requestVariables(t, fake.Stdin[len(lookups)+len(writes)-1])["threadId"]; got != th.id {
				t.Errorf("resolved thread %v, want %s", got, th.id)
			}
		})
	}
}

// editStep is the gh pr edit call for title.
func editStep(title string) cmdrunnertestutil.Step {
	return cmdrunnertestutil.Step{Name: "gh", Args: []string{"pr", "edit", testPR, "--title=" + title, "--body-file", "-"}}
}

func TestEditPRVerbatim(t *testing.T) {
	for name, text := range trickyTexts {
		if strings.Contains(text, "\n") {
			continue // a title is one line; the body case below covers it
		}
		t.Run(name, func(t *testing.T) {
			body := "## Summary\n" + text + "\nREPLYBODY_EOF\n"
			g, fake := newTestGitHub(t, editStep(text))
			req := EditPRRequest{PR: mustPR(t), TitleFile: writeTextFile(t, text), BodyFile: writeTextFile(t, body)}
			if err := g.EditPR(t.Context(), req); err != nil {
				t.Fatalf("EditPR = %v", err)
			}
			if got := string(fake.Stdin[0]); got != body {
				t.Errorf("body on standard input = %q, want %q", got, body)
			}
		})
	}
	// These rows check that EditPR rejects before any gh call, not the file
	// checks themselves, which TestBodyFileChecks covers.
	for name, files := range map[string][2]string{
		"zero_pr":         {"title", "body"},
		"two_line_title":  {"a\nb", "body"},
		"empty_body_file": {"title", ""},
	} {
		t.Run("rejects_before_any_call/"+name, func(t *testing.T) {
			g, _ := newTestGitHub(t)
			req := EditPRRequest{PR: mustPR(t), TitleFile: writeTextFile(t, files[0]), BodyFile: writeTextFile(t, files[1])}
			if name == "zero_pr" {
				req.PR = PRNumber{}
			}
			if err := g.EditPR(t.Context(), req); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("EditPR = %v, want ErrInvalidInput", err)
			}
		})
	}
	t.Run("gh_failure", func(t *testing.T) {
		step := editStep("title")
		step.Err = &cmdrunner.ExitError{Code: ghExitFailure}
		g, _ := newTestGitHub(t, step)
		req := EditPRRequest{PR: mustPR(t), TitleFile: writeTextFile(t, "title"), BodyFile: writeTextFile(t, "body")}
		if err := g.EditPR(t.Context(), req); !errors.Is(err, ErrResponseRejected) {
			t.Errorf("EditPR = %v, want ErrResponseRejected", err)
		}
	})
}

func TestReplyPostFailureSkipsResolve(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	// No step after the post is scripted, so fetching or resolving after the
	// failure fails the test.
	t.Run("github_refused", func(t *testing.T) {
		post := postStep()
		post.Err = &cmdrunner.ExitError{Code: ghExitFailure}
		g, _ := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}), post)...)
		err := g.Reply(t.Context(), replyRequest(t, th, "reply"))
		if !errors.Is(err, ErrResponseRejected) || errors.Is(err, ErrReplyNotResolved) {
			t.Errorf("Reply = %v, want ErrResponseRejected without ErrReplyNotResolved", err)
		}
	})
	// A timeout, a cancellation, or an unreadable success may leave a posted
	// reply, so the caller must not post again.
	timeout := postStep()
	timeout.Err = context.DeadlineExceeded
	cancelled := postStep()
	cancelled.Err = context.Canceled
	noID := postStep()
	noID.Out = `{"body":"x"}`
	for name, post := range map[string]cmdrunnertestutil.Step{"timed_out": timeout, "cancelled": cancelled, "no_comment_id": noID} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}), post)...)
			if err := g.Reply(t.Context(), replyRequest(t, th, "reply")); !errors.Is(err, ErrReplyNotResolved) || StopKind(err) != StopReplyNotResolved {
				t.Errorf("Reply = %v, want ErrReplyNotResolved", err)
			}
		})
	}
}

func TestReplyRechecksBeforeResolve(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	edited := withReply(th)
	edited.comments[0].edited = testLaterAt
	resolved := withReply(th)
	resolved.resolved = true
	arrived := withReply(th)
	arrived.comments = append(arrived.comments, userComment(postedID+1, "mallory"))
	// Each row breaks one condition: comment_arrived keeps the reply in place
	// but adds a comment after it, other_comment has the right count but not
	// the reply, comment_edited changes a judged comment.
	for name, now := range map[string][]testThread{
		"comment_arrived": {arrived},
		"comment_edited":  {edited},
		"reply_missing":   {th},
		"other_comment":   {{id: th.id, comments: append(slices.Clone(th.comments), userComment(postedID+1, "mallory"))}},
		"thread_resolved": {resolved},
		"thread_gone":     {},
	} {
		t.Run(name, func(t *testing.T) {
			// No resolve step is scripted, so resolving fails the test.
			g, _ := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}),
				postStep(), graphqlStep(stageOneResponse(t, now, false, "")))...)
			err := g.Reply(t.Context(), replyRequest(t, th, "reply"))
			if !errors.Is(err, ErrReplyNotResolved) || !errors.Is(err, ErrThreadChanged) || StopKind(err) != StopReplyNotResolved {
				t.Errorf("Reply = %v, want ErrReplyNotResolved for a changed thread", err)
			}
		})
	}
}

func TestReplyResolveFailure(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	failed := graphqlStep("")
	failed.Err = &cmdrunner.ExitError{Code: ghExitFailure}
	for name, resolve := range map[string]cmdrunnertestutil.Step{
		"gh_fails":            failed,
		"not_resolved":        graphqlStep(resolvedResponse(t, th.id, false)),
		"other_thread":        graphqlStep(resolvedResponse(t, testThreadID(2), true)),
		"is_resolved_missing": graphqlStep(`{"data":{"resolveReviewThread":{"thread":{"id":"` + th.id + `"}}}}`),
	} {
		t.Run(name, func(t *testing.T) {
			steps := writeSteps(t, th)
			steps[len(steps)-1] = resolve
			g, _ := newTestGitHub(t, append(replyLookupSteps(t, []testThread{th}), steps...)...)
			err := g.Reply(t.Context(), replyRequest(t, th, "reply"))
			if !errors.Is(err, ErrReplyNotResolved) || StopKind(err) != StopReplyNotResolved {
				t.Errorf("Reply = %v, want ErrReplyNotResolved", err)
			}
		})
	}
}
