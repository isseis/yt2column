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
	"time"

	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/cmdrunner/testutil"
)

// beginSteps scripts a Begin call: head, repository, the stage-one page, the
// permission lookups, and the stage-two response when stageTwo is not empty.
func beginSteps(stageOne string, lookups []cmdrunnertestutil.Step, stageTwo string) []cmdrunnertestutil.Step {
	steps := append(listSteps([]string{stageOne}), lookups...)
	if stageTwo != "" {
		steps = append(steps, graphqlStep(stageTwo))
	}
	return steps
}

func withBody(th testThread, body string) testThread {
	th.comments = slices.Clone(th.comments)
	for i := range th.comments {
		th.comments[i].body = body
	}
	return th
}

func TestNewGitHub(t *testing.T) {
	fake := cmdrunnertestutil.New(t)
	if _, err := NewGitHub(fake, Login{}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("NewGitHub(zero reviewer) = %v, want ErrInvalidInput", err)
	}
	if _, err := NewGitHub(nil, mustLogin(t, testReviewer)); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("NewGitHub(nil runner) = %v, want ErrInvalidInput", err)
	}
}

// TestGitHubWithoutReviewerJudgesNothing: a GitHub built without a reviewer
// answers Head but refuses every call that judges threads before running gh;
// the fake has no step for them.
func TestGitHubWithoutReviewerJudgesNothing(t *testing.T) {
	if _, err := NewGitHubWithoutReviewer(nil); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("NewGitHubWithoutReviewer(nil) = %v, want ErrInvalidInput", err)
	}
	g, err := NewGitHubWithoutReviewer(cmdrunnertestutil.New(t, headStep(sameRepositoryHead)))
	if err != nil {
		t.Fatalf("NewGitHubWithoutReviewer = %v", err)
	}
	ctx, pr := t.Context(), mustPR(t)
	if _, err := g.Head(ctx, pr); err != nil {
		t.Errorf("Head = %v", err)
	}
	accept := []Accept{{ThreadID: testThreadID(1), Fingerprint: Fingerprint(strings.Repeat("a", fingerprintLength))}}
	// A valid body file, so Reply can fail only for the missing reviewer.
	bodyFile := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(bodyFile, []byte("Fixed.\n"), 0o600); err != nil {
		t.Fatalf("write body: %v", err)
	}
	calls := map[string]func() error{
		"List":        func() error { _, err := g.List(ctx, pr); return err },
		"Begin":       func() error { _, err := g.Begin(ctx, pr, nil); return err },
		"ThreadPaths": func() error { _, err := g.ThreadPaths(ctx, pr, accept); return err },
		"Reply": func() error {
			return g.Reply(ctx, ReplyRequest{PR: pr, ThreadID: accept[0].ThreadID, CommentID: 1, Fingerprint: accept[0].Fingerprint, BodyFile: bodyFile})
		},
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s = %v, want ErrInvalidInput", name, err)
		}
	}
}

func TestCurrentPR(t *testing.T) {
	step := func(out string, err error) cmdrunnertestutil.Step {
		return cmdrunnertestutil.Step{Name: "gh", Args: []string{"pr", "view", "--json", "number"}, Out: out, Err: err}
	}
	g, _ := newTestGitHub(t, step(`{"number":42}`, nil))
	pr, err := g.CurrentPR(t.Context())
	if err != nil || pr.String() != testPR {
		t.Errorf("CurrentPR = %v, %v, want %s", pr, err, testPR)
	}
	for _, tc := range []struct {
		name string
		out  string
		err  error
	}{
		{name: "not_a_number", out: `{"number":"42"}`},
		{name: "missing", out: `{}`},
		{name: "outside_PR_grammar", out: `{"number":0}`},
		{name: "gh_failure", err: &cmdrunner.ExitError{Code: ghExitFailure}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGitHub(t, step(tc.out, tc.err))
			if _, err := g.CurrentPR(t.Context()); !errors.Is(err, ErrResponseRejected) {
				t.Errorf("CurrentPR = %v, want ErrResponseRejected", err)
			}
		})
	}
}

func TestHeadRequiresSameRepository(t *testing.T) {
	const repo = `"headRepository":{"id":"R_1","name":"yt2column"}`
	for _, tc := range []struct {
		name string
		out  string
		err  error
		want error
	}{
		{name: "same_repository", out: sameRepositoryHead},
		{name: "cross_true", out: `{"isCrossRepository":true,` + repo + `}`, want: ErrCrossRepository},
		{name: "cross_null", out: `{"isCrossRepository":null,` + repo + `}`, want: ErrCrossRepository},
		{name: "cross_missing", out: `{` + repo + `}`, want: ErrCrossRepository},
		{name: "cross_string", out: `{"isCrossRepository":"false",` + repo + `}`, want: ErrCrossRepository},
		{name: "head_repository_null", out: `{"isCrossRepository":false,"headRepository":null}`, want: ErrCrossRepository},
		{name: "head_repository_missing", out: `{"isCrossRepository":false}`, want: ErrCrossRepository},
		{name: "head_repository_string", out: `{"isCrossRepository":false,"headRepository":"yt2column"}`, want: ErrCrossRepository},
		{name: "cross_duplicate", out: `{"isCrossRepository":false,"isCrossRepository":true,` + repo + `}`, want: ErrResponseRejected},
		{name: "not_json", out: `false`, want: ErrResponseRejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step := headStep(tc.out)
			step.Err = tc.err
			g, _ := newTestGitHub(t, step)
			info, err := g.Head(t.Context(), mustPR(t))
			if !errors.Is(err, tc.want) {
				t.Fatalf("Head = %v, want %v", err, tc.want)
			}
			if tc.want == nil && info != (HeadInfo{Protocol: Protocol, SameRepository: true}) {
				t.Errorf("Head = %+v", info)
			}
		})
	}
	t.Run("zero_pr_number", func(t *testing.T) {
		g, _ := newTestGitHub(t)
		if _, err := g.Head(t.Context(), PRNumber{}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Head(zero) = %v, want ErrInvalidInput", err)
		}
	})
}

func TestBeginAndListCheckForkFirst(t *testing.T) {
	cross := `{"isCrossRepository":true,"headRepository":{"name":"fork"}}`
	// The fake holds only the head step, so any later call fails the test.
	t.Run("begin", func(t *testing.T) {
		g, _ := newTestGitHub(t, headStep(cross))
		if _, err := g.Begin(t.Context(), mustPR(t), nil); !errors.Is(err, ErrCrossRepository) {
			t.Errorf("Begin = %v, want ErrCrossRepository", err)
		}
	})
	t.Run("list", func(t *testing.T) {
		g, _ := newTestGitHub(t, headStep(cross))
		if _, err := g.List(t.Context(), mustPR(t)); !errors.Is(err, ErrCrossRepository) {
			t.Errorf("List = %v, want ErrCrossRepository", err)
		}
	})
	t.Run("begin_rejects_accept_before_any_call", func(t *testing.T) {
		g, _ := newTestGitHub(t)
		if _, err := g.Begin(t.Context(), mustPR(t), []Accept{{ThreadID: "PRRT_x", Fingerprint: "short"}}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Begin = %v, want ErrInvalidInput", err)
		}
	})
}

func TestBeginAssessesEveryCall(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{userComment(1, "alice")}}
	stageOne := stageOneResponse(t, []testThread{th}, false, "")
	steps := slices.Concat(
		beginSteps(stageOne, []cmdrunnertestutil.Step{roleStep("alice", "write")}, stageTwoResponse(t, withBody(th, "please fix"))),
		beginSteps(stageOne, []cmdrunnertestutil.Step{roleStep("alice", "read")}, ""),
	)
	g, _ := newTestGitHub(t, steps...)
	first, err := g.Begin(t.Context(), mustPR(t), nil)
	if err != nil {
		t.Fatalf("first Begin = %v", err)
	}
	if len(first.Threads) != 1 || len(first.Untrusted) != 0 {
		t.Fatalf("first Begin = %+v, want the thread processed", first)
	}
	second, err := g.Begin(t.Context(), mustPR(t), nil)
	if err != nil {
		t.Fatalf("second Begin = %v", err)
	}
	if len(second.Threads) != 0 || len(second.Untrusted) != 1 {
		t.Errorf("second Begin = %+v, want the thread untrusted after write access was lost", second)
	}
}

func TestBeginNeverFetchesUntrustedBody(t *testing.T) {
	trusted := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1), userComment(2, "alice")}}
	untrusted := testThread{id: testThreadID(2), comments: []testComment{reviewerComment(3), userComment(4, "mallory")}}
	resolved := testThread{id: testThreadID(3), resolved: true, comments: []testComment{userComment(5, "eve")}}
	stageOne := stageOneResponse(t, []testThread{trusted, untrusted, resolved}, false, "")
	lookups := []cmdrunnertestutil.Step{roleStep("alice", "admin"), roleStep("mallory", "read")}

	t.Run("only_processed_threads_fetched", func(t *testing.T) {
		g, fake := newTestGitHub(t, beginSteps(stageOne, lookups, stageTwoResponse(t, withBody(trusted, "please fix")))...)
		round, err := g.Begin(t.Context(), mustPR(t), nil)
		if err != nil {
			t.Fatalf("Begin = %v", err)
		}
		ids, _ := requestVariables(t, fake.Stdin[len(fake.Stdin)-1])["ids"].([]any)
		if !slices.Equal(ids, []any{trusted.id}) {
			t.Errorf("stage-two ids = %v, want only %s", ids, trusted.id)
		}
		if len(round.Threads) != 1 || round.Threads[0].Body != "@coderabbitai: please fix\n\n@alice: please fix" {
			t.Errorf("Threads = %+v", round.Threads)
		}
		if len(round.Untrusted) != 1 || round.Untrusted[0].ThreadID != untrusted.id {
			t.Errorf("Untrusted = %+v, want %s", round.Untrusted, untrusted.id)
		}
	})
	t.Run("no_fetch_when_nothing_is_trusted", func(t *testing.T) {
		stageOne := stageOneResponse(t, []testThread{untrusted}, false, "")
		g, _ := newTestGitHub(t, beginSteps(stageOne, []cmdrunnertestutil.Step{roleStep("mallory", "read")}, "")...)
		round, err := g.Begin(t.Context(), mustPR(t), nil)
		if err != nil {
			t.Fatalf("Begin = %v", err)
		}
		if len(round.Threads) != 0 || len(round.Untrusted) != 1 {
			t.Errorf("Begin = %+v", round)
		}
		// Empty lists are arrays, not null, for the commands that read the output.
		out, err := json.Marshal(round)
		if err != nil {
			t.Fatalf("Marshal = %v", err)
		}
		for _, member := range []string{`"threads":[]`, `"changed":[]`, `"acceptNotFound":[]`} {
			if !strings.Contains(string(out), member) {
				t.Errorf("begin output lacks %s: %s", member, out)
			}
		}
	})
}

func TestBeginStageTwoFingerprintMismatch(t *testing.T) {
	th := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	stageOne := stageOneResponse(t, []testThread{th}, false, "")
	for name, later := range map[string]testThread{
		"comment_added":  {id: th.id, comments: []testComment{reviewerComment(1), userComment(2, "mallory")}},
		"comment_edited": {id: th.id, comments: []testComment{{id: 1, kind: typenameBot, login: testReviewer, edited: testLaterAt}}},
		"resolved":       {id: th.id, resolved: true, comments: th.comments},
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, beginSteps(stageOne, nil, stageTwoResponse(t, withBody(later, "injected")))...)
			round, err := g.Begin(t.Context(), mustPR(t), nil)
			if err != nil {
				t.Fatalf("Begin = %v", err)
			}
			want := []ChangedThread{{ThreadID: th.id, URL: testCommentURL(1), Stage: changedSinceJudgment}}
			if len(round.Threads) != 0 || !slices.Equal(round.Changed, want) {
				t.Errorf("Begin = %+v, want the thread reported as changed", round)
			}
		})
	}
	for name, stageTwo := range map[string]string{
		"extra_node": stageTwoResponse(t, withBody(th, "x"), withBody(testThread{id: testThreadID(2), comments: th.comments}, "x")),
		"other_node": stageTwoResponse(t, withBody(testThread{id: testThreadID(2), comments: th.comments}, "x")),
		"null_node":  `{"data":{"nodes":[null]}}`,
		"no_body":    strings.Replace(stageTwoResponse(t, th), `"body":"",`, ``, 1),
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, beginSteps(stageOne, nil, stageTwo)...)
			if _, err := g.Begin(t.Context(), mustPR(t), nil); !errors.Is(err, ErrResponseRejected) {
				t.Errorf("Begin = %v, want ErrResponseRejected", err)
			}
		})
	}
}

func TestBeginReportsAllReasons(t *testing.T) {
	th := testThread{id: testThreadID(1), more: true, comments: []testComment{
		userComment(1, "alice"),
		botComment(2, "dependabot"),
		{id: 3, noAuthor: true},
		userComment(4, "bob"),
		{id: 5, kind: "Organization", login: "acme"},
	}}
	lookups := []cmdrunnertestutil.Step{roleStep("alice", "read"), permissionStep("bob", "", permissionFailure(404))}
	g, _ := newTestGitHub(t, beginSteps(stageOneResponse(t, []testThread{th}, false, ""), lookups, "")...)
	round, err := g.Begin(t.Context(), mustPR(t), nil)
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	want := []Finding{
		{Reason: ReasonIncompleteComments},
		{Reason: ReasonNoWritePermission, Account: mustLogin(t, "alice")},
		{Reason: ReasonOtherBot, Account: mustLogin(t, "dependabot")},
		{Reason: ReasonNoAuthorName, CommentURL: testCommentURL(3)},
		{Reason: ReasonPermissionUnavailable, Cause: CauseNotFound},
		{Reason: ReasonUnknownAuthorKind},
	}
	if len(round.Untrusted) != 1 || !slices.Equal(round.Untrusted[0].Findings, want) {
		t.Errorf("Untrusted = %+v, want findings %+v", round.Untrusted, want)
	}
	if round.PermissionLookups != (LookupBudget{Used: 2, Remaining: maxPermissionLookups - 2}) {
		t.Errorf("budget = %+v", round.PermissionLookups)
	}
}

func TestBeginAcceptsNamedThread(t *testing.T) {
	th := testThread{id: testThreadID(2), comments: []testComment{userComment(1, "mallory")}}
	accept := []Accept{
		{ThreadID: th.id, Fingerprint: testFingerprint(t, th)},
		{ThreadID: "PRRT_gone", Fingerprint: testFP},
	}
	g, fake := newTestGitHub(t, beginSteps(stageOneResponse(t, []testThread{th}, false, ""),
		[]cmdrunnertestutil.Step{roleStep("mallory", "read")}, stageTwoResponse(t, withBody(th, "checked by the user")))...)
	round, err := g.Begin(t.Context(), mustPR(t), accept)
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	if len(round.Threads) != 1 || !round.Threads[0].Named || round.Threads[0].Body != "@mallory: checked by the user" {
		t.Errorf("Threads = %+v, want the named thread", round.Threads)
	}
	if len(round.Untrusted) != 0 || len(round.Changed) != 0 {
		t.Errorf("Untrusted = %+v, Changed = %+v, want none", round.Untrusted, round.Changed)
	}
	if !slices.Equal(round.AcceptNotFound, []string{"PRRT_gone"}) {
		t.Errorf("AcceptNotFound = %v", round.AcceptNotFound)
	}
	if ids, _ := requestVariables(t, fake.Stdin[len(fake.Stdin)-1])["ids"].([]any); !slices.Equal(ids, []any{th.id}) {
		t.Errorf("stage-two ids = %v", ids)
	}
}

func TestBeginAcceptChangedFingerprint(t *testing.T) {
	seen := testThread{id: testThreadID(2), comments: []testComment{userComment(1, "mallory"), userComment(2, "mallory")}}
	accept := []Accept{{ThreadID: seen.id, Fingerprint: testFingerprint(t, seen)}}
	for name, now := range map[string]testThread{
		"comment_added":   {id: seen.id, comments: append(slices.Clone(seen.comments), userComment(3, "mallory"))},
		"comment_edited":  {id: seen.id, comments: []testComment{userComment(1, "mallory"), {id: 2, kind: typenameUser, login: "mallory", edited: testLaterAt}}},
		"comment_deleted": {id: seen.id, comments: seen.comments[:1]},
	} {
		t.Run(name, func(t *testing.T) {
			// No stage-two step: the changed thread's body must not be fetched.
			g, _ := newTestGitHub(t, beginSteps(stageOneResponse(t, []testThread{now}, false, ""),
				[]cmdrunnertestutil.Step{roleStep("mallory", "read")}, "")...)
			round, err := g.Begin(t.Context(), mustPR(t), accept)
			if err != nil {
				t.Fatalf("Begin = %v", err)
			}
			wantChanged := []ChangedThread{{ThreadID: seen.id, URL: testCommentURL(1), Stage: changedSinceAccept}}
			if len(round.Threads) != 0 || !slices.Equal(round.Changed, wantChanged) {
				t.Errorf("Begin = %+v, want the thread reported as changed", round)
			}
			if len(round.Untrusted) != 1 || round.Untrusted[0].Fingerprint != testFingerprint(t, now) {
				t.Errorf("Untrusted = %+v, want the thread with its current fingerprint", round.Untrusted)
			}
		})
	}
}

func TestBeginRefusesEmptyNamedThread(t *testing.T) {
	empty := testThread{id: testThreadID(1)}
	accept := []Accept{{ThreadID: empty.id, Fingerprint: testFingerprint(t, empty)}}
	// No stage-two step: a thread without comments is never selected.
	g, _ := newTestGitHub(t, beginSteps(stageOneResponse(t, []testThread{empty}, false, ""), nil, "")...)
	round, err := g.Begin(t.Context(), mustPR(t), accept)
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	if len(round.Threads) != 0 || len(round.Untrusted) != 1 || len(round.Changed) != 0 {
		t.Errorf("Begin = %+v, want the empty thread reported as untrusted only, not processed", round)
	}
}

func TestBeginAuthorType(t *testing.T) {
	reviewer := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	otherBot := testThread{id: testThreadID(2), comments: []testComment{botComment(2, "dependabot")}}
	accept := []Accept{{ThreadID: otherBot.id, Fingerprint: testFingerprint(t, otherBot)}}
	g, _ := newTestGitHub(t, beginSteps(stageOneResponse(t, []testThread{reviewer, otherBot}, false, ""), nil,
		stageTwoResponse(t, reviewer, otherBot))...)
	round, err := g.Begin(t.Context(), mustPR(t), accept)
	if err != nil {
		t.Fatalf("Begin = %v", err)
	}
	if len(round.Threads) != 2 || round.Threads[0].AuthorType != "Bot" || round.Threads[1].AuthorType != "User" {
		t.Errorf("Threads = %+v, want Bot for the reviewer and User for a named other bot", round.Threads)
	}
}

// deadlineRunner records the time left before each call's deadline (zero when
// the context has none) and delegates to a scripted fake.
type deadlineRunner struct {
	*cmdrunnertestutil.Fake
	deadlines []time.Duration
}

func (r *deadlineRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.RunInput(ctx, nil, name, args...)
}

func (r *deadlineRunner) RunInput(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	var left time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		left = time.Until(deadline)
	}
	r.deadlines = append(r.deadlines, left)
	return r.Fake.RunInput(ctx, stdin, name, args...)
}

func TestGitHubCallsHaveTimeout(t *testing.T) {
	runner := &deadlineRunner{Fake: cmdrunnertestutil.New(t, headStep(sameRepositoryHead))}
	g, err := NewGitHub(runner, mustLogin(t, testReviewer))
	if err != nil {
		t.Fatalf("NewGitHub = %v", err)
	}
	if _, err := g.Head(t.Context(), mustPR(t)); err != nil {
		t.Fatalf("Head = %v", err)
	}
	if len(runner.deadlines) != 1 || runner.deadlines[0] <= 0 || runner.deadlines[0] > githubTimeout {
		t.Errorf("deadlines = %v, want one within %v", runner.deadlines, githubTimeout)
	}
}

func TestListOmitsBodies(t *testing.T) {
	trusted := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	untrusted := testThread{id: testThreadID(2), comments: []testComment{userComment(2, "mallory")}}
	resolved := testThread{id: testThreadID(3), resolved: true, comments: []testComment{reviewerComment(3)}}
	g, _ := newTestGitHub(t, listSteps([]string{stageOneResponse(t, []testThread{trusted, untrusted, resolved}, false, "")},
		roleStep("mallory", "read"))...)
	list, err := g.List(t.Context(), mustPR(t))
	if err != nil {
		t.Fatalf("List = %v", err)
	}
	out, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("Marshal = %v", err)
	}
	if strings.Contains(string(out), `"body"`) {
		t.Errorf("list output has a body member: %s", out)
	}
	// A trusted thread's findings are an empty array, not null, for the
	// commands that read the output.
	if !strings.Contains(string(out), `"findings":[]`) {
		t.Errorf("list output lacks an empty findings array: %s", out)
	}
	want := []ListedThread{
		{ThreadID: trusted.id, URL: testCommentURL(1), Trusted: true, Fingerprint: testFingerprint(t, trusted), Findings: []Finding{}},
		{
			ThreadID: untrusted.id, URL: testCommentURL(2), Fingerprint: testFingerprint(t, untrusted),
			Findings: []Finding{{Reason: ReasonNoWritePermission, Account: mustLogin(t, "mallory")}},
		},
	}
	if len(list.Threads) != len(want) {
		t.Fatalf("Threads = %+v, want %+v", list.Threads, want)
	}
	for i := range want {
		got := list.Threads[i]
		if got.ThreadID != want[i].ThreadID || got.URL != want[i].URL || got.Trusted != want[i].Trusted ||
			got.Fingerprint != want[i].Fingerprint || !slices.Equal(got.Findings, want[i].Findings) {
			t.Errorf("Threads[%d] = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestThreadPathsRequiresTrustedOrNamed(t *testing.T) {
	trusted := testThread{id: testThreadID(1), comments: []testComment{reviewerComment(1)}}
	untrusted := testThread{id: testThreadID(2), comments: []testComment{userComment(2, "mallory")}}
	resolved := testThread{id: testThreadID(3), resolved: true, comments: []testComment{reviewerComment(3)}}
	empty := testThread{id: testThreadID(4)}
	stageOne := stageOneResponse(t, []testThread{trusted, untrusted, resolved, empty}, false, "")
	pathSteps := func(lookups ...cmdrunnertestutil.Step) []cmdrunnertestutil.Step {
		return append([]cmdrunnertestutil.Step{repoStep(), graphqlStep(stageOne)}, lookups...)
	}
	accept := func(th testThread) Accept { return Accept{ThreadID: th.id, Fingerprint: testFingerprint(t, th)} }

	t.Run("trusted_and_named", func(t *testing.T) {
		g, _ := newTestGitHub(t, pathSteps(roleStep("mallory", "read"))...)
		paths, err := g.ThreadPaths(t.Context(), mustPR(t), []Accept{accept(trusted), accept(untrusted)})
		if err != nil {
			t.Fatalf("ThreadPaths = %v", err)
		}
		want := []ThreadPath{
			{ThreadID: trusted.id, Path: "internal/fixprguard/github.go"},
			{ThreadID: untrusted.id, Path: "internal/fixprguard/github.go", Named: true},
		}
		if !slices.Equal(paths, want) {
			t.Errorf("ThreadPaths = %+v, want %+v", paths, want)
		}
	})
	for name, given := range map[string]Accept{
		"fingerprint_differs": {ThreadID: trusted.id, Fingerprint: testFP},
		"unknown_thread":      {ThreadID: "PRRT_gone", Fingerprint: testFP},
		"resolved_thread":     accept(resolved),
		"empty_thread":        accept(empty),
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, pathSteps()...)
			if _, err := g.ThreadPaths(t.Context(), mustPR(t), []Accept{given}); !errors.Is(err, ErrThreadChanged) {
				t.Errorf("ThreadPaths = %v, want ErrThreadChanged", err)
			}
		})
	}
	t.Run("named_twice", func(t *testing.T) {
		g, _ := newTestGitHub(t)
		if _, err := g.ThreadPaths(t.Context(), mustPR(t), []Accept{accept(trusted), accept(trusted)}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("ThreadPaths = %v, want ErrInvalidInput", err)
		}
	})
}

// TestErrorsOmitCommandDetails matches the error text because its content is
// the contract: no standard output or error of the failed command (secret) and
// no argument (the --input and --json flags) may reach it.
func TestErrorsOmitCommandDetails(t *testing.T) {
	const secret = "SECRET-DETAIL"
	failure := &cmdrunner.ExitError{Code: 1, Stdout: []byte(secret), Stderr: []byte(secret)}
	fail := func(step cmdrunnertestutil.Step) cmdrunnertestutil.Step {
		step.Err = failure
		return step
	}
	th := testThread{id: testThreadID(1), comments: []testComment{userComment(1, "alice")}}
	stageOne := stageOneResponse(t, []testThread{th}, false, "")
	for name, steps := range map[string][]cmdrunnertestutil.Step{
		"head":      {fail(headStep(""))},
		"repo_view": {headStep(sameRepositoryHead), fail(repoStep())},
		"stage_one": {headStep(sameRepositoryHead), repoStep(), fail(graphqlStep(""))},
	} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGitHub(t, steps...)
			_, err := g.Begin(t.Context(), mustPR(t), nil)
			if !errors.Is(err, ErrResponseRejected) {
				t.Fatalf("Begin = %v, want ErrResponseRejected", err)
			}
			if msg := err.Error(); strings.Contains(msg, secret) || strings.Contains(msg, "--input") || strings.Contains(msg, "--json") {
				t.Errorf("error %q carries the command's arguments or output", msg)
			}
		})
	}
	t.Run("permission_lookup", func(t *testing.T) {
		g, _ := newTestGitHub(t, beginSteps(stageOne, []cmdrunnertestutil.Step{fail(permissionStep("alice", "", nil))}, "")...)
		round, err := g.Begin(t.Context(), mustPR(t), nil)
		if err != nil {
			t.Fatalf("Begin = %v", err)
		}
		out, err := json.Marshal(round)
		if err != nil {
			t.Fatalf("Marshal = %v", err)
		}
		if strings.Contains(string(out), secret) || strings.Contains(string(out), "collaborators") {
			t.Errorf("output carries the command's arguments or output: %s", out)
		}
	})
}
