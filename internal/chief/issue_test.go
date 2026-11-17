//go:build test

package chief

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
)

// issueAnswer returns an issueOrPullRequest answer around node.
func issueAnswer(node string) string {
	return `{"data":{"repository":{"issueOrPullRequest":` + node + `}}}`
}

// issueNode returns an Issue node numbered 7 with the given state and the
// labels JSON.
func issueNode(state, labels string) string {
	return fmt.Sprintf(`{"__typename":"Issue","number":7,"state":%q,"title":"a title","labels":%s}`, state, labels)
}

// labelsOf returns a complete labels member naming each of names.
func labelsOf(names ...string) string {
	nodes := make([]string, len(names))
	for i, name := range names {
		nodes[i] = fmt.Sprintf(`{"name":%q}`, name)
	}
	return fmt.Sprintf(`{"totalCount":%d,"nodes":[%s]}`, len(names), strings.Join(nodes, ","))
}

const notFoundAnswer = `{"data":{"repository":{"issueOrPullRequest":null}},"errors":[{"type":"NOT_FOUND","path":["repository","issueOrPullRequest"],"message":"m"}]}`

func TestParseIssueSamples(t *testing.T) {
	cases := []struct {
		file   string
		exit   int
		number string
		want   IssueStatus
		title  string
	}{
		{"gh/issue-open-839.json", 0, "839", IssueOpen, "Sample title of an open issue"},
		{"gh/issue-draft-932.json", 0, "932", IssueDraft, "Sample title of a draft issue"},
		{"gh/issue-closed-838.json", 0, "838", IssueClosed, "Sample title of a closed issue"},
		{"gh/issue-pr-897.json", 0, "897", IssuePullRequest, ""},
		{"gh/issue-missing.json", 1, "999999", IssueNotFound, ""},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			issue, err := parseIssue(readTestdata(t, c.file), c.exit, issueNumber(t, c.number))
			if err != nil {
				t.Fatal(err)
			}
			if issue.Status != c.want || issue.Title != c.title || issue.TitleCut {
				t.Errorf("parseIssue = %+v, want status %d title %q", issue, c.want, c.title)
			}
		})
	}
}

func TestParseIssueStatus(t *testing.T) {
	cases := []struct {
		name   string
		answer string
		want   IssueStatus
	}{
		{"open with other labels", issueAnswer(issueNode("OPEN", labelsOf("devenv", "priority:high"))), IssueOpen},
		{"open with no label", issueAnswer(issueNode("OPEN", labelsOf())), IssueOpen},
		{"open with the draft label", issueAnswer(issueNode("OPEN", labelsOf("devenv", "draft"))), IssueDraft},
		{"closed", issueAnswer(issueNode("CLOSED", labelsOf())), IssueClosed},
		{"closed with the draft label is closed", issueAnswer(issueNode("CLOSED", labelsOf("draft"))), IssueClosed},
		{"Draft is not the draft label", issueAnswer(issueNode("OPEN", labelsOf("Draft"))), IssueOpen},
		{"DRAFT is not the draft label", issueAnswer(issueNode("OPEN", labelsOf("DRAFT"))), IssueOpen},
		{"draft-x is not the draft label", issueAnswer(issueNode("OPEN", labelsOf("draft-x", "x-draft", " draft", "draft "))), IssueOpen},
		{"a pull request", issueAnswer(`{"__typename":"PullRequest","number":7}`), IssuePullRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issue, err := parseIssue([]byte(c.answer), 0, issueNumber(t, "7"))
			if err != nil {
				t.Fatal(err)
			}
			if issue.Status != c.want {
				t.Errorf("Status = %d, want %d", issue.Status, c.want)
			}
		})
	}
}

func TestParseIssueRejects(t *testing.T) {
	node := issueNode("OPEN", labelsOf("devenv"))
	valid := issueAnswer(node)
	if _, err := parseIssue([]byte(valid), 0, issueNumber(t, "7")); err != nil {
		t.Fatalf("the base answer is rejected: %v", err)
	}
	cases := []struct {
		name   string
		answer string
		exit   int
	}{
		{"data after the object", valid + " {}", 0},
		{"two objects", valid + valid, 0},
		{"not an object", `[]`, 0},
		{"empty", ``, 0},
		{"invalid UTF-8 in the title", strings.Replace(valid, "a title", "a \xff title", 1), 0},
		{"an unpaired surrogate", strings.Replace(valid, "a title", `a \ud800 title`, 1), 0},
		{"a duplicated number", strings.Replace(valid, `"number":7`, `"number":7,"number":7`, 1), 0},
		{"a duplicated state", strings.Replace(valid, `"state":"OPEN"`, `"state":"OPEN","state":"OPEN"`, 1), 0},
		{"a duplicated issueOrPullRequest", strings.Replace(valid, `{"issueOrPullRequest":`, `{"issueOrPullRequest":null,"issueOrPullRequest":`, 1), 0},
		{"a duplicated labels member", strings.Replace(valid, `"labels":`, `"labels":null,"labels":`, 1), 0},
		{"Title next to title", strings.Replace(valid, `"title":"a title"`, `"title":"a title","Title":"x"`, 1), 0},
		{"STATE next to state", strings.Replace(valid, `"state":"OPEN"`, `"state":"OPEN","STATE":"CLOSED"`, 1), 0},
		{"Number next to number", strings.Replace(valid, `"number":7`, `"number":7,"Number":8`, 1), 0},
		{"Labels next to labels", strings.Replace(valid, `"labels":`, `"Labels":null,"labels":`, 1), 0},
		{"IssueOrPullRequest next to the member", strings.Replace(valid, `{"issueOrPullRequest":`, `{"IssueOrPullRequest":null,"issueOrPullRequest":`, 1), 0},
		{"Repository next to repository", strings.Replace(valid, `{"repository":`, `{"Repository":null,"repository":`, 1), 0},
		{"Data next to data", `{"Data":null,` + valid[1:], 0},
		{"Errors next to errors", `{"Errors":[],` + valid[1:], 0},
		{"a typename in another case", strings.Replace(valid, `"__typename"`, `"__typename":"Issue","__Typename"`, 1), 0},
		{"null data", `{"data":null}`, 0},
		{"null repository", `{"data":{"repository":null}}`, 0},
		{"null issueOrPullRequest", issueAnswer(`null`), 0},
		{"issueOrPullRequest is not an object", issueAnswer(`7`), 0},
		{"a number as a string", strings.Replace(valid, `"number":7`, `"number":"7"`, 1), 0},
		{"another number", strings.Replace(valid, `"number":7`, `"number":8`, 1), 0},
		{"a null number", strings.Replace(valid, `"number":7`, `"number":null`, 1), 0},
		{"a fractional number", strings.Replace(valid, `"number":7`, `"number":7.5`, 1), 0},
		{"a state of MERGED", strings.Replace(valid, `"OPEN"`, `"MERGED"`, 1), 0},
		{"a lower-case state", strings.Replace(valid, `"OPEN"`, `"open"`, 1), 0},
		{"a null state", strings.Replace(valid, `"state":"OPEN"`, `"state":null`, 1), 0},
		{"a null title", strings.Replace(valid, `"title":"a title"`, `"title":null`, 1), 0},
		{"a title that is a number", strings.Replace(valid, `"title":"a title"`, `"title":1`, 1), 0},
		{"no title", strings.Replace(valid, `"title":"a title",`, ``, 1), 0},
		{"a null typename", strings.Replace(valid, `"__typename":"Issue"`, `"__typename":null`, 1), 0},
		{"an unknown typename", strings.Replace(valid, `"Issue"`, `"Discussion"`, 1), 0},
		{"no typename", strings.Replace(valid, `"__typename":"Issue",`, ``, 1), 0},
		{"a pull request answer with another number", issueAnswer(`{"__typename":"PullRequest","number":8}`), 0},
		{"errors beside an answer", `{"errors":[],` + valid[1:], 0},
		{"null errors beside an answer", `{"errors":null,` + valid[1:], 0},
		{"labels is null", strings.Replace(valid, labelsOf("devenv"), `null`, 1), 0},
		{"no labels", strings.Replace(valid, `,"labels":`+labelsOf("devenv"), ``, 1), 0},
		{"only some of the labels", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":2,"nodes":[{"name":"devenv"}]}`, 1), 0},
		{"more labels than the count", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":0,"nodes":[{"name":"devenv"}]}`, 1), 0},
		{"a negative label count", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":-1,"nodes":[]}`, 1), 0},
		{"a label count as a string", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":"1","nodes":[{"name":"devenv"}]}`, 1), 0},
		{"no label count", strings.Replace(valid, labelsOf("devenv"), `{"nodes":[{"name":"devenv"}]}`, 1), 0},
		{"no label nodes", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":0}`, 1), 0},
		{"label nodes that are not an array", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":1,"nodes":{"name":"devenv"}}`, 1), 0},
		{"a null label name", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":2,"nodes":[{"name":"devenv"},{"name":null}]}`, 1), 0},
		{"a label name that is a number", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":1,"nodes":[{"name":5}]}`, 1), 0},
		{"a label with no name", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":1,"nodes":[{}]}`, 1), 0},
		{"a label that is not an object", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":1,"nodes":["devenv"]}`, 1), 0},
		{"a null label node", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":1,"nodes":[null]}`, 1), 0},
		{"a label name in another case", strings.Replace(valid, labelsOf("devenv"), `{"totalCount":1,"nodes":[{"name":"devenv","Name":"draft"}]}`, 1), 0},
		{"a closed issue whose labels are partial", issueAnswer(issueNode("CLOSED", `{"totalCount":3,"nodes":[]}`)), 0},
		{"the not-found answer with exit 0", notFoundAnswer, 0},
		{"an issue answer with exit 1", valid, 1},
		{"a good answer with another exit", valid, 2},
		{"not-found with exit 2", notFoundAnswer, 2},
		{"two errors", strings.Replace(notFoundAnswer, `}]}`, `},{"type":"NOT_FOUND","path":["repository","issueOrPullRequest"]}]}`, 1), 1},
		{"no error", strings.Replace(notFoundAnswer, `,"errors":[{"type":"NOT_FOUND","path":["repository","issueOrPullRequest"],"message":"m"}]`, ``, 1), 1},
		{"an empty error list", strings.Replace(notFoundAnswer, `[{"type":"NOT_FOUND","path":["repository","issueOrPullRequest"],"message":"m"}]`, `[]`, 1), 1},
		{"another error type", strings.Replace(notFoundAnswer, `NOT_FOUND`, `FORBIDDEN`, 1), 1},
		{"an error type in another case", strings.Replace(notFoundAnswer, `NOT_FOUND`, `not_found`, 1), 1},
		{"a deeper path", strings.Replace(notFoundAnswer, `"issueOrPullRequest"],`, `"issueOrPullRequest","x"],`, 1), 1},
		{"another path", strings.Replace(notFoundAnswer, `["repository","issueOrPullRequest"]`, `["repository","issue"]`, 1), 1},
		{"a path of numbers", strings.Replace(notFoundAnswer, `["repository","issueOrPullRequest"]`, `[1,2]`, 1), 1},
		{"no path", strings.Replace(notFoundAnswer, `"path":["repository","issueOrPullRequest"],`, ``, 1), 1},
		{"a value beside the error", strings.Replace(notFoundAnswer, `"issueOrPullRequest":null}}`, `"issueOrPullRequest":{"__typename":"PullRequest","number":7}}}`, 1), 1},
		{"no member at all", strings.Replace(notFoundAnswer, `"issueOrPullRequest":null`, ``, 1), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.answer == valid && c.exit == 0 {
				t.Fatal("the case changes nothing")
			}
			issue, err := parseIssue([]byte(c.answer), c.exit, issueNumber(t, "7"))
			if !errors.Is(err, errGHOutput) {
				t.Errorf("parseIssue = %+v, %v, want errGHOutput", issue, err)
			}
			if issue.Status != IssueUnreadable {
				t.Errorf("a rejected answer has status %d", issue.Status)
			}
		})
	}
}

func TestParseIssueNotFoundIsAccepted(t *testing.T) {
	// A message that is not the usual one is no reason to refuse: only the
	// declared type and path decide.
	answer := strings.Replace(notFoundAnswer, `"message":"m"`, `"message":"anything at all"`, 1)
	issue, err := parseIssue([]byte(answer), 1, issueNumber(t, "7"))
	if err != nil || issue.Status != IssueNotFound {
		t.Errorf("parseIssue = %+v, %v, want IssueNotFound", issue, err)
	}
	noMessage := strings.Replace(notFoundAnswer, `,"message":"m"`, ``, 1)
	if issue, err := parseIssue([]byte(noMessage), 1, issueNumber(t, "7")); err != nil || issue.Status != IssueNotFound {
		t.Errorf("without a message: parseIssue = %+v, %v, want IssueNotFound", issue, err)
	}
}

// paddedAnswer returns an answer of exactly size bytes for issue 7, with the
// padding in a member the reader ignores.
func paddedAnswer(t *testing.T, size int) []byte {
	t.Helper()
	base := issueAnswer(issueNode("OPEN", labelsOf()))
	head := base[:len(base)-1] + `,"padding":"`
	tail := `"}`
	pad := size - len(head) - len(tail)
	if pad < 0 {
		t.Fatalf("size %d is below the base answer", size)
	}
	return []byte(head + strings.Repeat("x", pad) + tail)
}

func TestParseIssueSizeLimit(t *testing.T) {
	at := paddedAnswer(t, issueResponseMax)
	if len(at) != issueResponseMax {
		t.Fatalf("answer is %d bytes", len(at))
	}
	if _, err := parseIssue(at, 0, issueNumber(t, "7")); err != nil {
		t.Errorf("an answer of exactly the limit is rejected: %v", err)
	}
	over := paddedAnswer(t, issueResponseMax+1)
	if _, err := parseIssue(over, 0, issueNumber(t, "7")); !errors.Is(err, errGHOutput) {
		t.Errorf("an answer over the limit = %v, want errGHOutput", err)
	}
}

func TestParseIssueTitle(t *testing.T) {
	read := func(t *testing.T, title string) Issue {
		t.Helper()
		encoded, err := json.Marshal(title)
		if err != nil {
			t.Fatal(err)
		}
		answer := strings.Replace(issueAnswer(issueNode("OPEN", labelsOf())), `"a title"`, string(encoded), 1)
		issue, err := parseIssue([]byte(answer), 0, issueNumber(t, "7"))
		if err != nil {
			t.Fatal(err)
		}
		return issue
	}
	for _, unit := range []string{"a", "é", "€", "😀"} {
		t.Run("limit with "+unit, func(t *testing.T) {
			exact := strings.Repeat(unit, issueTitleMax)
			if got := read(t, exact); got.Title != exact || got.TitleCut {
				t.Errorf("a title of %d characters: %d characters, cut %v", issueTitleMax, utf8.RuneCountInString(got.Title), got.TitleCut)
			}
			got := read(t, exact+unit)
			if got.Title != exact || !got.TitleCut {
				t.Errorf("a title of %d characters: %d characters, cut %v", issueTitleMax+1, utf8.RuneCountInString(got.Title), got.TitleCut)
			}
			if !utf8.ValidString(got.Title) {
				t.Error("the cut title is not valid UTF-8")
			}
		})
	}
	t.Run("a hidden character is made visible before the limit applies", func(t *testing.T) {
		// A zero-width space becomes a visible escape longer than itself, so
		// the limit counts the escape, not the one character.
		got := read(t, strings.Repeat("\u200b", issueTitleMax))
		if !got.TitleCut || strings.ContainsRune(got.Title, '\u200b') {
			t.Errorf("title = %q, cut %v", got.Title, got.TitleCut)
		}
	})
	t.Run("a terminal escape loses its ESC", func(t *testing.T) {
		if got := read(t, "\x1b[31mred"); strings.ContainsRune(got.Title, 0x1b) {
			t.Errorf("title = %q", got.Title)
		}
	})
}

func TestReadIssueRuns(t *testing.T) {
	dir := "/chief/tree"
	issue := issueNumber(t, "7")
	line := "gh " + strings.Join(issueArgs(issue), " ")
	setup := func(t *testing.T) (GitReader, *recorder) {
		t.Helper()
		rec := newRecorder(t)
		return NewGitReader(rec.commands([]string{"PATH=/x", "GH_REPO=other/repo", "GH_HOST=example.test"})), rec
	}
	t.Run("an issue", func(t *testing.T) {
		g, rec := setup(t)
		rec.reply(line, []byte(issueAnswer(issueNode("OPEN", labelsOf()))))
		got, err := g.ReadIssue(t.Context(), dir, issue)
		if err != nil || got.Status != IssueOpen || got.Number != issue {
			t.Fatalf("ReadIssue = %+v, %v", got, err)
		}
		calls := rec.recorded()
		if len(calls) != 1 || calls[0].dir != dir {
			t.Fatalf("calls = %+v, want one in %s", calls, dir)
		}
		if !slices.Equal(calls[0].args, issueArgs(issue)) {
			t.Errorf("args = %v", calls[0].args)
		}
		for _, e := range calls[0].env {
			if strings.HasPrefix(e, "GH_REPO=") || strings.HasPrefix(e, "GH_HOST=") {
				t.Errorf("gh ran with %s, so it could read another repository", e)
			}
		}
	})
	t.Run("a number that exists nowhere is exit 1 with the answer on standard output", func(t *testing.T) {
		g, rec := setup(t)
		rec.fail(line, &cmdrunner.ExitError{Code: 1, Stdout: []byte(notFoundAnswer)})
		got, err := g.ReadIssue(t.Context(), dir, issue)
		if err != nil || got.Status != IssueNotFound {
			t.Fatalf("ReadIssue = %+v, %v", got, err)
		}
	})
	t.Run("a failure of gh is an error, not a status", func(t *testing.T) {
		for name, failure := range map[string]error{
			"exit 4": &cmdrunner.ExitError{Code: 4, Stdout: []byte(notFoundAnswer)},
			"exit 1 with a body that is not not-found": &cmdrunner.ExitError{Code: 1, Stdout: []byte(issueAnswer(issueNode("OPEN", labelsOf())))},
			"output too large":                         cmdrunner.ErrOutputTooLarge,
			"a plain error":                            errors.New("gh is not installed"),
		} {
			t.Run(name, func(t *testing.T) {
				g, rec := setup(t)
				rec.fail(line, failure)
				if got, err := g.ReadIssue(t.Context(), dir, issue); err == nil || got.Status != IssueUnreadable {
					t.Errorf("ReadIssue = %+v, %v, want an error", got, err)
				}
			})
		}
	})
	t.Run("no number", func(t *testing.T) {
		g, rec := setup(t)
		if _, err := g.ReadIssue(t.Context(), dir, paneinput.IssueNumber{}); !errors.Is(err, errNoIssueNumber) {
			t.Errorf("ReadIssue = %v, want errNoIssueNumber", err)
		}
		if calls := rec.recorded(); len(calls) != 0 {
			t.Errorf("gh ran: %v", calls)
		}
	})
}

// TestRecorderRejectsOtherGitAndHerdrWrites pins the recorder's other new
// rules: only the tool's one git grep passes, and the worktree and herdr
// operations that write do not.
func TestRecorderRejectsOtherGitAndHerdrWrites(t *testing.T) {
	passes := []recordedCall{
		{name: gitProgram, args: issueLineSearch},
		{name: gitProgram, args: []string{"worktree", "list", "--porcelain"}},
		{name: gitProgram, args: []string{"merge-base", "--is-ancestor", "a", "b"}},
		{name: herdrProgram, args: []string{"pane", "list"}},
		{name: herdrProgram, args: []string{"pane", "process-info", "--pane", "w1:p1"}},
	}
	for _, c := range passes {
		if !isReadOnly(c) {
			t.Errorf("the recorder fails %s", c.line())
		}
	}
	fails := []recordedCall{
		{name: gitProgram, args: append(slices.Clone(issueLineSearch), "-O", "cmd")},
		{name: gitProgram, args: []string{"grep", "-O", "cmd", "x"}},
		{name: gitProgram, args: []string{"worktree", "add", "x"}},
		{name: gitProgram, args: []string{"worktree", "remove", "x"}},
		{name: gitProgram, args: []string{"merge-base", "a", "b"}},
		{name: herdrProgram, args: []string{"pane", "close", "w1:p1"}},
		{name: herdrProgram, args: []string{"workspace", "close", "w1"}},
	}
	for _, c := range fails {
		if isReadOnly(c) {
			t.Errorf("the recorder passes %s", c.line())
		}
	}
}

// TestRecorderPassesOnlyTheFixedIssueRead pins the recorder's rule: `gh api
// graphql` is not a read-only prefix, because it can send a mutation or take
// another method; only the tool's one shape passes.
func TestRecorderPassesOnlyTheFixedIssueRead(t *testing.T) {
	good := issueArgs(issueNumber(t, "7"))
	if !isReadOnly(recordedCall{name: ghProgram, args: good}) {
		t.Fatal("the tool's own call is not read-only")
	}
	with := func(edit func([]string) []string) []string { return edit(slices.Clone(good)) }
	bad := map[string][]string{
		"a mutation":        with(func(a []string) []string { a[9] = "query=mutation { addComment }"; return a }),
		"another query":     with(func(a []string) []string { a[9] += " "; return a }),
		"a method":          with(func(a []string) []string { return append(a, "-X", "POST") }),
		"a method first":    with(func(a []string) []string { return append([]string{"-X", "DELETE"}, a...) }),
		"another owner":     with(func(a []string) []string { a[3] = "owner=other"; return a }),
		"another repo":      with(func(a []string) []string { a[5] = "repo=other"; return a }),
		"a number string":   with(func(a []string) []string { a[7] = "n=abc"; return a }),
		"a zero number":     with(func(a []string) []string { a[7] = "n=0"; return a }),
		"a signed number":   with(func(a []string) []string { a[7] = "n=+7"; return a }),
		"another field":     with(func(a []string) []string { a[6] = "-f"; return a }),
		"another endpoint":  with(func(a []string) []string { a[1] = "repos/o/r/issues"; return a }),
		"a template":        with(func(a []string) []string { return append(a, "--template", "{{.}}") }),
		"no query":          good[:8],
		"graphql alone":     {"api", "graphql"},
		"another gh action": {"issue", "close", "7"},
	}
	for name, args := range bad {
		t.Run(name, func(t *testing.T) {
			if isReadOnly(recordedCall{name: ghProgram, args: args}) {
				t.Errorf("the recorder passes %v", args)
			}
		})
	}
}
