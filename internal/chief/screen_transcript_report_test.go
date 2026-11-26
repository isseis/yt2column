//go:build test

package chief

import (
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// The PR the sampled /runpr report and the sampled /mkplan run name, and
// the texts their last reports start with.
const (
	runprSamplePR    = 940
	runprSampleURL   = "https://github.com/owner/yt2column/pull/940"
	mkplanSamplePR   = 935
	mkplanSampleURL  = "https://github.com/owner/yt2column/pull/935"
	runprReportHead  = "PR #940 をマージしました。"
	runprReportNext  = "NEXT: なし（タスク文書の承認がない PR のため）"
	mkplanLongHead   = "I opened PR #935 with task 0095's implementation plan in `draft`."
	mkplanLongTail   = "Next: review the PR, then run `/runpr 935` in a Claude Code pane."
	mkplanShortHead  = "Codex has started its review of PR #935"
	compactHead      = "タスク 0088 の PR-5（答えの中継）を開きました"
	recapFirstReport = "コミットのフックが終わるまで待機します。"
	recapLastReport  = "コミットのフックの完了を待っています。"
	syntheticReport  = "PR-1 の PR #944 を作成して push しました"
	autoContinueHead = "利用上限に達したため、ここで止めます。"
)

// transcriptSample returns the lines of a transcript sample.
func transcriptSample(t *testing.T, name string) []string {
	t.Helper()
	return strings.Split(strings.TrimSpace(string(readTestdata(t, "transcript/"+name))), "\n")
}

// sampleLine returns the index of the only line of lines that contains
// mark, failing the test when there is not exactly one.
func sampleLine(t *testing.T, lines []string, mark string) int {
	t.Helper()
	found := -1
	for i, l := range lines {
		if strings.Contains(l, mark) {
			if found >= 0 {
				t.Fatalf("two sample lines hold %q", mark)
			}
			found = i
		}
	}
	if found < 0 {
		t.Fatalf("no sample line holds %q", mark)
	}
	return found
}

// joined joins transcript lines.
func joined(lines ...[]string) string {
	var all []string
	for _, l := range lines {
		all = append(all, l...)
	}
	return strings.Join(all, "\n")
}

// TestReadSessionReport reads the last report, the PR record, and the last
// human input of the transcript samples and of transcripts made from them:
// the report is the last finished response's text in full, read past
// subagent and synthetic responses and Claude Code's own records, and is
// absent once the human typed after it, while it has not finished, or when
// it has no text.
func TestReadSessionReport(t *testing.T) {
	t.Parallel()
	runpr := transcriptSample(t, "claude-runpr-report.jsonl")
	mkplan := transcriptSample(t, "claude-command-report.jsonl")
	recap := transcriptSample(t, "claude-recap-notification.jsonl")
	synthetic := transcriptSample(t, "claude-synthetic.jsonl")
	compact := transcriptSample(t, "claude-compact.jsonl")
	auto := transcriptSample(t, "claude-auto-continuation.jsonl")
	queued := transcriptSample(t, "claude-queued-input.jsonl")
	clearModel := string(readTestdata(t, "transcript/claude-clear-model.jsonl"))

	cont := sampleLine(t, mkplan, `"content":"continue"`)
	resumed := sampleLine(t, auto, `"kind":"auto-continuation"`)
	autoReport := sampleLine(t, auto, autoContinueHead)
	synthHuman := sampleLine(t, synthetic, `"content":"Short text."`)
	synthLine := sampleLine(t, synthetic, `"model":"<synthetic>"`)
	runprCommand := HumanInput{Kind: HumanInputCommand, Name: "runpr", Args: "940"}
	mkplanCommand := HumanInput{Kind: HumanInputCommand, Name: "mkplan", Args: "0095"}
	none := HumanInput{Kind: HumanInputNone}
	other := HumanInput{Kind: HumanInputOther}

	// A finished response written in two lines, the second holding a
	// thinking part, a tool call, and a part of an unknown type around two
	// text parts.
	twoLines := joined(mkplan[:1], []string{
		`{"type":"assistant","isSidechain":false,"message":{"id":"msg_2","model":"claude-opus-5-5","stop_reason":"end_turn","content":[{"type":"text","text":"  first"}]}}`,
		`{"type":"assistant","isSidechain":false,"message":{"id":"msg_2","model":"claude-opus-5-5","stop_reason":"end_turn","content":[{"type":"thinking","text":"hidden"},{"type":"text","text":"second"},{"type":"tool_use","text":"hidden"},{"type":"image","text":"hidden"},{"type":"text","text":"third  "}]}}`,
	})
	// A subagent's finished response after the report.
	reportLine := runpr[sampleLine(t, runpr, runprReportHead)]
	sidechain := reportLine
	for _, r := range [][2]string{{`"isSidechain":false`, `"isSidechain":true`}, {runprReportHead, "A subagent's answer."}, {"msg_000000000000000000000008", "msg_sub"}} {
		next := strings.Replace(sidechain, r[0], r[1], 1)
		if next == sidechain {
			t.Fatalf("the report line no longer holds %q", r[0])
		}
		sidechain = next
	}

	cases := []struct {
		name      string
		text      string
		kind      SessionReportKind
		head      string
		contains  string
		tail      string
		lastHuman HumanInput
		pr        int
		url       string
	}{
		{"the /runpr run", joined(runpr), SessionReportFound, runprReportHead, runprReportNext, "", runprCommand, runprSamplePR, runprSampleURL},
		{"the PR-creating run up to its long report", joined(mkplan[:cont]), SessionReportFound, mkplanLongHead, "**What's left**", mkplanLongTail, mkplanCommand, mkplanSamplePR, mkplanSampleURL},
		{"the human typed after the report", joined(mkplan[:cont+1]), SessionReportAbsent, "", "", "", other, mkplanSamplePR, mkplanSampleURL},
		{"a new response started after the human typed", joined(mkplan[:cont+3]), SessionReportAbsent, "", "", "", other, mkplanSamplePR, mkplanSampleURL},
		{"the next report, with a recap after it", joined(mkplan), SessionReportFound, mkplanShortHead, "", "from that session's transcript.", other, mkplanSamplePR, mkplanSampleURL},
		{"text the human typed while the agent worked, after the report", joined(runpr, queued[2:3]), SessionReportAbsent, "", "", "", other, runprSamplePR, runprSampleURL},
		{"a peer's message and a task notification after the report", joined(runpr, queued[:2]), SessionReportFound, runprReportHead, "", "", runprCommand, runprSamplePR, runprSampleURL},
		{"a subagent's finished response after the report", joined(runpr, []string{sidechain}), SessionReportFound, runprReportHead, runprReportNext, "今回の報告をサンプルとして採ります。", runprCommand, runprSamplePR, runprSampleURL},
		{"a subagent's finished response after an unfinished one", joined(runpr[:len(runpr)-2], []string{sidechain}), SessionReportAbsent, "", "", "", runprCommand, runprSamplePR, runprSampleURL},
		{"a recap after the report", joined(recap[:3]), SessionReportFound, recapFirstReport, "", "", none, 0, ""},
		{"a response to a task notification", joined(recap), SessionReportFound, recapLastReport, "", "", none, 0, ""},
		{"a compact summary after the report", joined(compact), SessionReportFound, compactHead, "", "", none, 886, "https://github.com/owner/yt2column/pull/886"},
		{"only a synthetic response after the report", joined(synthetic[:synthHuman], synthetic[synthHuman+1:synthLine+2]), SessionReportFound, syntheticReport, "", "", none, 944, "https://github.com/owner/yt2column/pull/944"},
		{"a synthetic response after an unfinished one", joined(runpr[:len(runpr)-2], synthetic[synthLine:synthLine+1]), SessionReportAbsent, "", "", "", runprCommand, runprSamplePR, runprSampleURL},
		{"the human typed after a synthetic response", joined(synthetic), SessionReportAbsent, "", "", "", other, 944, "https://github.com/owner/yt2column/pull/944"},
		{"a report in a thinking line and a text line", joined(auto[:autoReport+1]), SessionReportFound, autoContinueHead, "", "", none, 0, ""},
		{"an auto-continuation after the report", joined(auto[:resumed+1]), SessionReportFound, autoContinueHead, "", "", none, 0, ""},
		{"the work resumed after an auto-continuation", joined(auto), SessionReportAbsent, "", "", "", none, 0, ""},
		{"a line of the report that did not end the turn", joined(auto[:autoReport-1], []string{strings.Replace(auto[autoReport-1], `"end_turn"`, `"tool_use"`, 1)}, auto[autoReport:autoReport+1]), SessionReportAbsent, "", "", "", none, 0, ""},
		{"a report whose stop reason is null", strings.Replace(joined(runpr), `"stop_reason":"end_turn"`, `"stop_reason":null`, 1), SessionReportAbsent, "", "", "", runprCommand, runprSamplePR, runprSampleURL},
		{"a report with no text", joined(auto[:autoReport]), SessionReportAbsent, "", "", "", none, 0, ""},
		{"a report whose text is blank", joined(runpr[:len(runpr)-2], []string{`{"type":"assistant","isSidechain":false,"message":{"id":"msg_blank","model":"claude-opus-5-5","stop_reason":"end_turn","content":[{"type":"text","text":" \n "}]}}`}), SessionReportAbsent, "", "", "", runprCommand, runprSamplePR, runprSampleURL},
		{"text parts joined in written order", twoLines, SessionReportFound, "first\n\nsecond\n\nthird", "", "first\n\nsecond\n\nthird", mkplanCommand, 0, ""},
		{"a response whose lines are apart, the last line its own", joined(mkplan[:1], []string{
			`{"type":"assistant","isSidechain":false,"message":{"id":"msg_x","model":"m","stop_reason":"end_turn","content":[{"type":"text","text":"one"}]}}`,
			`{"type":"assistant","isSidechain":false,"message":{"id":"msg_y","model":"m","stop_reason":"end_turn","content":[{"type":"text","text":"two"}]}}`,
			`{"type":"assistant","isSidechain":false,"message":{"id":"msg_x","model":"m","stop_reason":"end_turn","content":[{"type":"text","text":"three"}]}}`,
		}), SessionReportFound, "one\n\nthree", "", "one\n\nthree", mkplanCommand, 0, ""},
		{"the human typed between two lines of the last response", joined(mkplan[:1], []string{
			`{"type":"assistant","isSidechain":false,"message":{"id":"msg_x","model":"m","stop_reason":"end_turn","content":[{"type":"text","text":"one"}]}}`,
			typedLine("more"),
			`{"type":"assistant","isSidechain":false,"message":{"id":"msg_x","model":"m","stop_reason":"end_turn","content":[{"type":"text","text":"two"}]}}`,
		}), SessionReportFound, "one\n\ntwo", "", "one\n\ntwo", other, 0, ""},
		{"a session started by /clear with no response", clearModel, SessionReportAbsent, "", "", "", none, 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := readSessionReport([]byte(c.text))
			if got.Kind != c.kind || got.LastHuman != c.lastHuman {
				t.Fatalf("readSessionReport = kind %v, human %+v; want kind %v, human %+v", got.Kind, got.LastHuman, c.kind, c.lastHuman)
			}
			if c.kind != SessionReportFound && got.Text != "" {
				t.Errorf("Text = %q; want none without a report", got.Text)
			}
			if !strings.HasPrefix(got.Text, c.head) || !strings.Contains(got.Text, c.contains) || !strings.HasSuffix(got.Text, c.tail) {
				t.Errorf("Text = %q; want it to start with %q, hold %q, and end with %q", got.Text, c.head, c.contains, c.tail)
			}
			if got.Text != strings.TrimSpace(got.Text) {
				t.Errorf("Text = %q is not trimmed", got.Text)
			}
			if c.pr != 0 && !got.Names(c.pr, c.url) {
				t.Errorf("Names(%d, %s) = false; want the PR record read", c.pr, c.url)
			}
		})
	}
}

// TestReadSessionReportLastHuman reads the last human input of the sampled
// human-input transcript and of its prefixes and rewrites: only a line
// Claude Code declares the human's counts, a command is read in the sampled
// form alone, a line that cannot be told makes the transcript unreadable,
// and an auto-continuation is not the human's and is not refused.
func TestReadSessionReportLastHuman(t *testing.T) {
	t.Parallel()
	sample := strings.Split(strings.TrimSpace(string(readTestdata(t, "transcript/claude-human-input.jsonl"))), "\n")
	// The sample's response line was cut to the members the human input
	// reader reads; it gets the members a response line must have.
	response := sampleLine(t, sample, `"type":"assistant"`)
	sample[response] = `{"type":"assistant","isSidechain":false,"message":{"id":"msg_1","model":"claude-opus-5-5","stop_reason":"tool_use","content":[{"type":"text","text":"Reading the plan."}]}}`
	queued := transcriptSample(t, "claude-queued-input.jsonl")
	resumed := transcriptSample(t, "claude-auto-continuation.jsonl")
	resume := resumed[sampleLine(t, resumed, `"kind":"auto-continuation"`)]
	whole := strings.Join(sample, "\n")
	upTo := strings.Join(sample[:3], "\n")
	command := HumanInput{Kind: HumanInputCommand, Name: "runplan", Args: "0088"}
	cases := []struct {
		name string
		text string
		want HumanInput
	}{
		{"the whole sample, ending in typed text", whole, HumanInput{Kind: HumanInputOther}},
		{"up to the lines after the command", strings.Join(sample[:len(sample)-1], "\n"), command},
		{"up to the command", upTo, command},
		{"before any human line", strings.Join(sample[:2], "\n"), HumanInput{Kind: HumanInputNone}},
		{"a broken line", whole + "\n{\"type\":", HumanInput{}},
		{"two names that differ", strings.Replace(upTo, "<command-message>runplan<", "<command-message>mkplan<", 1), HumanInput{Kind: HumanInputOther}},
		{"the command line without its human origin", strings.Replace(upTo, `"origin":{"kind":"human"},`, "", 1), HumanInput{Kind: HumanInputNone}},
		{"a later line of an unsampled origin", upTo + "\n" + strings.Replace(sample[len(sample)-1], `"kind":"human"`, `"kind":"scheduled"`, 1), HumanInput{}},
		{"a message the human typed while the agent worked", upTo + "\n" + queued[2], HumanInput{Kind: HumanInputOther}},
		{"a peer's message and a task notification delivered while the agent worked", upTo + "\n" + queued[0] + "\n" + queued[1], command},
		{"a queued command of an unsampled origin", upTo + "\n" + strings.Replace(queued[2], `"kind":"human"`, `"kind":"coordinator"`, 1), HumanInput{}},
		{"a queued command without an origin", upTo + "\n" + strings.Replace(queued[2], `"origin":{"kind":"human"},`, "", 1), HumanInput{}},
		{"a later human line repeating a member", upTo + "\n" + strings.Replace(sample[len(sample)-1], `"type":"user",`, `"type":"user","type":"user",`, 1), HumanInput{}},
		{"an auto-continuation after the command", upTo + "\n" + resume, command},
	}
	for _, c := range cases {
		got := readSessionReport([]byte(c.text))
		if got.LastHuman != c.want || got.LastHuman != readLastHumanInput([]byte(c.text)) {
			t.Errorf("%s: LastHuman = %+v, want %+v, the value readLastHumanInput reads", c.name, got.LastHuman, c.want)
		}
		if unreadable := c.want == (HumanInput{}); unreadable != (got.Kind == SessionReportUnreadable) {
			t.Errorf("%s: kind = %v; want unreadable only when the human input cannot be told", c.name, got.Kind)
		}
	}
}

// TestReadSessionReportRejects checks that a transcript outside the
// accepted shape is unreadable with nothing but its kind set, while an
// unknown line type, member, content part, or stop reason is read past.
func TestReadSessionReportRejects(t *testing.T) {
	t.Parallel()
	base := joined(transcriptSample(t, "claude-runpr-report.jsonl"))
	if got := readSessionReport([]byte(base)); got.Kind != SessionReportFound {
		t.Fatalf("the /runpr sample reads as %v; want a report", got.Kind)
	}
	reportID := `"id":"msg_000000000000000000000008"`
	link := `"prNumber":940,"prUrl":"https://github.com/owner/yt2column/pull/940","prRepository":"owner/yt2column"`
	edit := func(old, repl string) string { return strings.Replace(base, old, repl, 1) }
	rejected := []struct {
		name string
		text string
	}{
		{"a repeated stop reason", edit(`"stop_reason":"end_turn"`, `"stop_reason":"end_turn","stop_reason":"tool_use"`)},
		{"a repeated message", edit(`{"type":"assistant","isSidechain":false,"message":{"id":"msg_000000000000000000000008"`, `{"type":"assistant","isSidechain":false,"message":{},"message":{"id":"msg_000000000000000000000008"`)},
		{"a repeated PR number", edit(link, `"prNumber":1,`+link)},
		{"a repeated sidechain flag", edit(`{"type":"assistant","isSidechain":false,"message":{"id":"msg_000000000000000000000008"`, `{"type":"assistant","isSidechain":false,"isSidechain":false,"message":{"id":"msg_000000000000000000000008"`)},
		{"a repeated part type", edit(`"content":[{"type":"text","text":"PR #940`, `"content":[{"type":"text","type":"text","text":"PR #940`)},
		{"a repeated part text", edit(`"content":[{"type":"text","text":"PR #940`, `"content":[{"type":"text","text":"x","text":"PR #940`)},
		{"a repeated type", edit(`{"type":"pr-link",`, `{"type":"pr-link","type":"pr-link",`)},
		{"a stop reason that is a number", edit(`"stop_reason":"end_turn"`, `"stop_reason":1`)},
		{"a text part whose text is not a string", edit(`"content":[{"type":"text","text":"PR #940`, `"content":[{"type":"text","text":1},{"type":"text","text":"PR #940`)},
		{"a part whose type is not a string", edit(`"content":[{"type":"text","text":"PR #940`, `"content":[{"type":1},{"type":"text","text":"PR #940`)},
		{"content that is not an array", edit(`"content":[{"type":"text","text":"PR #940`, `"content":{"type":"text"},"x":[{"type":"text","text":"PR #940`)},
		{"a sidechain flag that is not a boolean", edit(`{"type":"assistant","isSidechain":false,"message":{"id":"msg_000000000000000000000008"`, `{"type":"assistant","isSidechain":"false","message":{"id":"msg_000000000000000000000008"`)},
		{"a message ID that is not a string", edit(reportID, `"id":1`)},
		{"a model that is not a string", edit(`"model":"claude-opus-5-5","stop_reason":"end_turn"`, `"model":null,"stop_reason":"end_turn"`)},
		{"a PR number given as a string", edit(`"prNumber":940`, `"prNumber":"940"`)},
		{"a PR number of zero", edit(`"prNumber":940`, `"prNumber":0`)},
		{"a negative PR number", edit(`"prNumber":940`, `"prNumber":-940`)},
		{"a PR number with a fraction", edit(`"prNumber":940`, `"prNumber":940.0`)},
		{"a repository that is not a string", edit(`"prRepository":"owner/yt2column"`, `"prRepository":null`)},
		{"invalid UTF-8", edit(runprReportHead, "\xff")},
		{"an unpaired surrogate escape", edit(runprReportHead, `\ud800`)},
		{"a response line without its sidechain flag", edit(`{"type":"assistant","isSidechain":false,"message":{"id":"msg_000000000000000000000008"`, `{"type":"assistant","message":{"id":"msg_000000000000000000000008"`)},
		{"a response line without a message ID", edit(reportID+",", "")},
		{"a response line without a model", edit(`"model":"claude-opus-5-5","stop_reason":"end_turn"`, `"stop_reason":"end_turn"`)},
		{"a response line without a stop reason", edit(`"stop_reason":"end_turn",`, "")},
		{"a response line without content", edit(`"model":"claude-opus-5-5","stop_reason":"end_turn","content":[`, `"model":"claude-opus-5-5","stop_reason":"end_turn","x":[`)},
		{"a text part without its text", edit(`"content":[{"type":"text","text":"PR #940`, `"content":[{"type":"text"},{"type":"text","text":"PR #940`)},
		{"a PR record without its number", edit(`"prNumber":940,`, "")},
		{"a PR record without its URL", edit(`"prUrl":"https://github.com/owner/yt2column/pull/940",`, "")},
		{"a PR record without its repository", edit(`,"prRepository":"owner/yt2column"`, "")},
		{"an empty type", edit(`{"type":"system","subtype":"turn_duration"}`, `{"type":"","subtype":"turn_duration"}`)},
		{"an empty PR URL", edit(`"prUrl":"https://github.com/owner/yt2column/pull/940"`, `"prUrl":""`)},
		{"an empty message ID", edit(reportID, `"id":""`)},
		{"a line that is not an object", base + "\n[]"},
		{"a half-written last line", base + "\n" + `{"type":"assistant","isSidechain":false,"mess`},
	}
	for _, c := range rejected {
		t.Run(c.name, func(t *testing.T) {
			if c.text == base {
				t.Fatal("the rewrite did not change the sample")
			}
			got := readSessionReport([]byte(c.text))
			if got.Kind != SessionReportUnreadable || got.Text != "" || got.LastHuman != (HumanInput{}) || got.Names(runprSamplePR, runprSampleURL) {
				t.Errorf("readSessionReport = %+v; want unreadable with nothing else set", got)
			}
		})
	}

	readPast := []struct {
		name string
		text string
		kind SessionReportKind
	}{
		{"an unknown line type", base + "\n" + `{"type":"brand-new","detail":{"a":1}}`, SessionReportFound},
		{"an unknown member of a response line", edit(`{"type":"assistant","isSidechain":false,"message":{"id":"msg_000000000000000000000008"`, `{"type":"assistant","isSidechain":false,"brandNew":[1],"message":{"brandNew":{},"id":"msg_000000000000000000000008"`), SessionReportFound},
		{"an unknown member of a PR record", edit(link, link+`,"brandNew":1`), SessionReportFound},
		{"an unknown stop reason", edit(`"stop_reason":"end_turn"`, `"stop_reason":"brand_new"`), SessionReportAbsent},
	}
	for _, c := range readPast {
		t.Run(c.name, func(t *testing.T) {
			if c.text == base {
				t.Fatal("the rewrite did not change the sample")
			}
			if got := readSessionReport([]byte(c.text)); got.Kind != c.kind {
				t.Errorf("readSessionReport kind = %v; want %v", got.Kind, c.kind)
			}
		})
	}
	unknownPart := edit(`"content":[{"type":"text","text":"PR #940`, `"content":[{"type":"brand_new","text":"hidden"},{"type":"text","text":"PR #940`)
	if got := readSessionReport([]byte(unknownPart)); got.Kind != SessionReportFound || strings.Contains(got.Text, "hidden") {
		t.Errorf("a part of an unknown type: %v %q; want the report without the part", got.Kind, got.Text)
	}
}

// TestSessionReportNames checks that a PR record names a PR only when both
// its number and its URL equal the PR's, each taken whole.
func TestSessionReportNames(t *testing.T) {
	t.Parallel()
	const url42 = "https://github.com/o/r/pull/42"
	report := readSessionReport([]byte(`{"type":"pr-link","prNumber":42,"prUrl":"` + url42 + `","prRepository":"o/r"}`))
	if report.Kind != SessionReportAbsent {
		t.Fatalf("kind = %v; want a readable session with no report", report.Kind)
	}
	cases := []struct {
		name   string
		number int
		url    string
		want   bool
	}{
		{"the same number and URL", 42, url42, true},
		{"the same number, another repository", 42, "https://github.com/o/other/pull/42", false},
		{"another number, the same URL", 4, url42, false},
		{"a number that starts with it, the same URL", 421, url42, false},
		{"a URL that starts with it", 42, url42 + "1", false},
	}
	for _, c := range cases {
		if got := report.Names(c.number, c.url); got != c.want {
			t.Errorf("%s: Names(%d, %s) = %v, want %v", c.name, c.number, c.url, got, c.want)
		}
	}
}

// TestReadReportNext reads the NEXT and the PR numbers of reports: a line in
// the grammar is read, a line that names NEXT otherwise cannot be read, a
// line that only mentions it is no NEXT, and lines that disagree do not
// match; PR numbers are read line by line, never across a line break.
func TestReadReportNext(t *testing.T) {
	t.Parallel()
	task, err := paneinput.ParseTaskID("0015")
	if err != nil {
		t.Fatal(err)
	}
	runplan := RunprNext{Kind: NextCommand, Command: NextCommandRunplan, Task: task}
	none := RunprNext{Kind: NextNone}
	unmatched := RunprNext{}
	absent := RunprNext{Kind: NextAbsent}
	cases := []struct {
		name string
		text string
		next RunprNext
		prs  []int
	}{
		{"a command", "Merged #7.\n\nNEXT: /runplan 0015", runplan, []int{7}},
		{"a command in backquotes", "NEXT: `/runplan 0015`", runplan, nil},
		{"none with a reason, as a list item", "  - NEXT: なし（理由）  ", none, nil},
		{"bold", "**NEXT:** `/runplan 0015`", unmatched, nil},
		{"bold before the colon", "**NEXT**: /runplan 0015", unmatched, nil},
		{"code", "`NEXT: /runplan 0015`", unmatched, nil},
		{"a quote", "> NEXT: /runplan 0015", unmatched, nil},
		{"a numbered item", "2. NEXT: `/runplan 0015`", unmatched, nil},
		{"another bullet", "* NEXT: /runplan 0015", unmatched, nil},
		{"a full-width colon", "NEXT：/runplan 0015", unmatched, nil},
		{"no value", "NEXT:", unmatched, nil},
		{"an unknown command", "NEXT: /mkreq 0015", unmatched, nil},
		{"a mention in a sentence", "the PR that NEXT: opens", absent, nil},
		{"another case", "Next: /runplan 0015", absent, nil},
		{"no colon", "NEXT — /runplan 0015", absent, nil},
		{"no NEXT", "Merged.", absent, nil},
		{"two lines that agree", "NEXT: /runplan 0015\nNEXT: `/runplan 0015`", runplan, nil},
		{"two lines that differ", "NEXT: /runplan 0015\nNEXT: なし", unmatched, nil},
		{"a readable line after an unreadable one", "**NEXT:** x\nNEXT: /runplan 0015", unmatched, nil},
		{"PR numbers and URLs", "#42 and https://github.com/o/r/pull/7, #42 again", absent, []int{42, 7}},
		{"a number split by a line break", "https://github.com/o/r/pull/4\n2 merged", absent, []int{4}},
	}
	for _, c := range cases {
		got := readReportNext(c.text)
		if got.Next != c.next || !slices.Equal(got.PRs, c.prs) {
			t.Errorf("%s: readReportNext = %+v; want next %+v, PRs %v", c.name, got, c.next, c.prs)
		}
	}
	sample := readSessionReport([]byte(joined(transcriptSample(t, "claude-runpr-report.jsonl"))))
	if got := readReportNext(sample.Text); got.Next != none || !slices.Contains(got.PRs, runprSamplePR) {
		t.Errorf("the sampled /runpr report reads as %+v; want NEXT none and #%d", got, runprSamplePR)
	}
}
