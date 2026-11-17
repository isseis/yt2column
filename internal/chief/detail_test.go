//go:build test && unix

package chief

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
	"github.com/isseis/yt2column/internal/privatefile"
)

// prURL is the URL of the open PR of openPRJSON.
const prURL = "https://github.com/owner/repo/pull/42"

// prHead is the head commit the detail tests' PR has.
const prHead = "e229481341330097cfa2b5976837cee8a7cf7718"

// viewOf returns a gh pr view sample made into the view of the open PR 42:
// its number, URL, state, and head changed, and the edit applied.
func viewOf(t *testing.T, sample string, edit func(map[string]any)) []byte {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(readTestdata(t, "gh/"+sample), &v); err != nil {
		t.Fatal(err)
	}
	v["number"], v["url"], v["state"], v["headRefName"], v["headRefOid"] = 42, prURL, "OPEN", taskBranch, prHead
	if edit != nil {
		edit(v)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// detailWorld is a send world with one workspace whose branch has PR 42
// open, a Claude Code pane and an OpenCode pane, and the PR's view and
// checks.
func detailWorld(t *testing.T) (*sendWorld, string) {
	t.Helper()
	return detailWorldAt(t, "", "claude", "opencode")
}

// detailWorldAt is detailWorld with the worktree directory named tree (""
// for the default name) and one pane of each kind given.
func detailWorldAt(t *testing.T, tree string, kinds ...string) (*sendWorld, string) {
	t.Helper()
	sw := newSendWorld(t)
	// origin/main has the approved requirements, so the open PR is /mkarch's.
	tree = sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, PRs: openPRJSON, Docs: &fakeDocs{Dir: taskDir, Req: reqApproved}, Tree: tree}, kinds...)
	sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", nil))
	sw.rec.replyIn(tree, "gh pr checks 42 --json "+prChecksFields, readTestdata(t, "gh/pr-checks-200-fail.json"))
	sw.git(tree, "rev-parse HEAD", prHead+"\n")
	// The Claude Code pane is the one that created the PR.
	if p := sw.byPane["w1:p1"]; p.Kind == "claude" {
		p.ran = "/mkarch 0001"
	}
	return sw, tree
}

// addClaudePane adds an idle Claude Code pane w1:p<n> to the workspace at
// tree; call it before finish.
func (sw *sendWorld) addClaudePane(tree string, n int) *simPane {
	sw.t.Helper()
	p := &simPane{fakeAgent: fakeAgent{Pane: fmt.Sprintf("w1:p%d", n), Workspace: "w1", Kind: "claude", Status: "idle", Session: sw.newSession(), Dir: tree, Seq: 10}}
	p.screen = readScreenSample(sw.t, "claude-idle.ansi")
	sw.panes = append(sw.panes, p)
	sw.byPane[p.Pane] = p
	return p
}

// makerReport is the last report of the command that created PR 42 in the
// detail and reset tests. It names the PR by its number alone, never by its
// URL, so only the session's PR record ties it to the PR.
const makerReport = "Opened PR 42 with the architecture document of task 0001 for review."

// openPRLink is the transcript's PR record of the open PR 42.
var openPRLink = prURLRecordLine(42, prURL)

// endWithReport appends to the transcript of p's session the PR record of
// PR 42 and a finished last report of text.
func (sw *sendWorld) endWithReport(p *simPane, text string) {
	sw.t.Helper()
	sw.appendTranscript(p, openPRLink, answerLine("msg_report", text))
}

// breakTranscript appends to the transcript of p's session a line cut off
// while it was written, which makes the transcript unreadable.
func (sw *sendWorld) breakTranscript(p *simPane) {
	sw.t.Helper()
	sw.appendTranscript(p, `{"type":"assistant"`)
}

// removeTranscript removes the transcript of p's session, which Herdr still
// reports.
func (sw *sendWorld) removeTranscript(p *simPane) {
	sw.t.Helper()
	if err := os.Remove(sw.transcript(p, p.Session)); err != nil {
		sw.t.Fatal(err)
	}
}

func detailOf(t *testing.T, deps Deps, tree string) PRDetail {
	t.Helper()
	out, err := Detail(context.Background(), deps, tree)
	if err != nil || out.Detail == nil {
		t.Fatalf("Detail = %+v, %v", out, err)
	}
	return *out.Detail
}

// TestDetailItems checks the items of a PR's detail: the PR, its size, its
// reviews, its jobs, and /runpr with the routing table's agent and model.
func TestDetailItems(t *testing.T) {
	sw, tree := detailWorld(t)
	sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-852-reviews.json", nil))
	d := detailOf(t, sw.finish(), tree)
	if d.PR.Number != 42 || d.PR.URL != prURL || d.View == nil || d.ViewErr != "" {
		t.Fatalf("detail = %+v", d)
	}
	v := d.View
	if v.ChangedFiles != 4 || v.Additions != 1187 || v.Commits != 8 || len(v.Files) != 4 {
		t.Errorf("size = %+v", v)
	}
	if len(v.Reviews) != 6 || v.Reviews[0] != (Review{Author: "chatgpt-codex-connector", State: "COMMENTED"}) {
		t.Errorf("reviews = %+v", v.Reviews)
	}
	if len(d.Checks) != 7 {
		t.Errorf("checks = %+v", d.Checks)
	}
	if d.Runpr == nil || d.Runpr.Command == nil || d.Runpr.Text != "/runpr 42" || d.Runpr.Agent != paneinput.AgentOpenCode || d.Runpr.Model != openCodeModel {
		t.Errorf("runpr = %+v, want /runpr 42 to OpenCode %s", d.Runpr, openCodeModel)
	}
}

// TestDetailKeepsCheckBuckets checks that every job is returned with its
// bucket as gh gave it, skipping kept apart from pass, including when gh
// exits 8 (pending) or 1 (failed) with its result on standard output; an
// exit without a readable result is a failed read.
func TestDetailKeepsCheckBuckets(t *testing.T) {
	checks := readTestdata(t, "gh/pr-checks-200-fail.json")
	cases := []struct {
		name  string
		reply func(sw *sendWorld, tree, line string)
		ok    bool
	}{
		{"exit 0", func(sw *sendWorld, tree, line string) { sw.rec.replyIn(tree, line, checks) }, true},
		{"exit 8", func(sw *sendWorld, tree, line string) {
			sw.rec.failIn(tree, line, &cmdrunner.ExitError{Code: 8, Stdout: checks})
		}, true},
		{"exit 1", func(sw *sendWorld, tree, line string) {
			sw.rec.failIn(tree, line, &cmdrunner.ExitError{Code: 1, Stdout: checks})
		}, true},
		{"exit 1 without a result", func(sw *sendWorld, tree, line string) {
			sw.rec.failIn(tree, line, &cmdrunner.ExitError{Code: 1})
		}, false},
		{"exit 2 with a result", func(sw *sendWorld, tree, line string) {
			sw.rec.failIn(tree, line, &cmdrunner.ExitError{Code: 2, Stdout: checks})
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, tree := detailWorld(t)
			c.reply(sw, tree, "gh pr checks 42 --json "+prChecksFields)
			d := detailOf(t, sw.finish(), tree)
			if !c.ok {
				if d.ChecksErr == "" || d.Checks != nil {
					t.Errorf("checks = %+v, error %q; want a failed read", d.Checks, d.ChecksErr)
				}
				return
			}
			buckets := map[string]int{}
			for _, ch := range d.Checks {
				buckets[ch.Bucket]++
			}
			if d.ChecksErr != "" || buckets["skipping"] != 3 || buckets["pass"] != 2 || buckets["fail"] != 1 || buckets["pending"] != 1 {
				t.Errorf("buckets = %v, error %q", buckets, d.ChecksErr)
			}
		})
	}
}

// TestDetailOmitsReviewBody checks that no review body reaches the output,
// while each review's author and state do.
func TestDetailOmitsReviewBody(t *testing.T) {
	sw, tree := detailWorld(t)
	view := viewOf(t, "pr-view-852-reviews.json", func(v map[string]any) {
		for _, r := range v["reviews"].([]any) {
			r.(map[string]any)["body"] = "REVIEW-BODY-MARK"
		}
	})
	sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, view)
	out, err := Detail(context.Background(), sw.finish(), tree)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(out.Output())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "REVIEW-BODY-MARK") {
		t.Errorf("a review body reached the output: %s", data)
	}
	if !strings.Contains(string(data), `"author":"chatgpt-codex-connector","state":"COMMENTED"`) {
		t.Errorf("the output lacks a review's author and state: %s", data)
	}
}

// TestDetailReportSource checks where the summary comes from: the last
// report of the one Claude Code pane whose session has a PR record naming
// the PR, read from its transcript, else the PR description with why, and
// what was found on each Claude Code pane. The report's own text never
// makes a pane a candidate, a screen is never read, and a pane whose
// transcript cannot be read counts as one with no report.
func TestDetailReportSource(t *testing.T) {
	cases := []struct {
		name string
		// kinds are the panes of the workspace, nil for detailWorld's.
		kinds []string
		// extra adds a second Claude Code pane, w1:p3.
		extra bool
		// set writes the Claude Code panes' transcripts and screens after
		// the world is finished.
		set    func(sw *sendWorld, panes []*simPane)
		want   ReportSource
		pane   string
		reason DescriptionReason
		found  []PaneReportKind
	}{
		{name: "the report on one pane", set: func(sw *sendWorld, ps []*simPane) {
			sw.endWithReport(ps[0], makerReport)
		}, want: ReportFromPane, pane: "w1:p1"},
		{name: "a PR record naming another PR", set: func(sw *sendWorld, ps []*simPane) {
			sw.appendTranscript(ps[0], prURLRecordLine(43, strings.TrimSuffix(prURL, "42")+"43"), answerLine("msg_report", makerReport))
		}, want: ReportFromDescription, reason: DescriptionReasonNoCandidate, found: []PaneReportKind{PaneReportNotNamed}},
		{name: "a report naming the PR's URL, with no PR record", set: func(sw *sendWorld, ps []*simPane) {
			sw.appendTranscript(ps[0], answerLine("msg_report", makerReport+" "+prURL))
		}, want: ReportFromDescription, reason: DescriptionReasonNoCandidate, found: []PaneReportKind{PaneReportNotNamed}},
		{name: "no report", set: func(*sendWorld, []*simPane) {}, want: ReportFromDescription, reason: DescriptionReasonNoCandidate, found: []PaneReportKind{PaneReportAbsent}},
		{name: "a report on the screen alone", set: func(sw *sendWorld, ps []*simPane) {
			ps[0].screen = claudeReportScreenNamingURL(sw.t, prURL)
		}, want: ReportFromDescription, reason: DescriptionReasonNoCandidate, found: []PaneReportKind{PaneReportAbsent}},
		{name: "no transcript file", set: func(sw *sendWorld, ps []*simPane) {
			sw.removeTranscript(ps[0])
		}, want: ReportFromDescription, reason: DescriptionReasonNoCandidate, found: []PaneReportKind{PaneReportAbsent}},
		{name: "an unreadable transcript", set: func(sw *sendWorld, ps []*simPane) {
			sw.endWithReport(ps[0], makerReport)
			sw.breakTranscript(ps[0])
		}, want: ReportFromDescription, reason: DescriptionReasonNoCandidate, found: []PaneReportKind{PaneReportUnreadable}},
		{name: "an unreadable pane and a pane with the report", extra: true, set: func(sw *sendWorld, ps []*simPane) {
			sw.breakTranscript(ps[0])
			sw.endWithReport(ps[1], makerReport)
		}, want: ReportFromPane, pane: "w1:p3"},
		{name: "the report on two panes", extra: true, set: func(sw *sendWorld, ps []*simPane) {
			sw.endWithReport(ps[0], makerReport)
			sw.endWithReport(ps[1], makerReport)
		}, want: ReportFromDescription, reason: DescriptionReasonSeveralCandidates, found: []PaneReportKind{PaneReportCandidate, PaneReportCandidate}},
		{name: "no Claude Code pane", kinds: []string{"opencode"}, set: func(*sendWorld, []*simPane) {}, want: ReportFromDescription, reason: DescriptionReasonNoClaudePane},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kinds := c.kinds
			if kinds == nil {
				kinds = []string{"claude", "opencode"}
			}
			sw, tree := detailWorldAt(t, "", kinds...)
			if c.extra {
				sw.addClaudePane(tree, 3)
			}
			deps := sw.finish()
			var claude []*simPane
			for _, p := range sw.panes {
				if p.Kind == "claude" {
					claude = append(claude, p)
				}
			}
			c.set(sw, claude)
			out, err := Detail(context.Background(), deps, tree)
			if err != nil || out.Detail == nil {
				t.Fatalf("Detail = %+v, %v", out, err)
			}
			d := *out.Detail
			if d.Report != c.want || d.RecordErr != "" {
				t.Fatalf("source = %s, record error %q; want %s", d.Report, d.RecordErr, c.want)
			}
			var found []PaneReportKind
			for _, p := range d.ReportPanes {
				found = append(found, p.Kind)
			}
			if d.DescriptionReason != c.reason || !slices.Equal(found, c.found) {
				t.Errorf("reason = %s, panes %+v; want %s, %v", d.DescriptionReason, d.ReportPanes, c.reason, c.found)
			}
			reason := out.Output().DescriptionReason
			switch c.want {
			case ReportFromPane:
				if d.ReportText != makerReport || d.record == nil || d.record.Pane != c.pane || reason != nil {
					t.Errorf("report = %q from %+v, output reason %+v; want the report of %s", d.ReportText, d.record, reason, c.pane)
				}
			case ReportFromDescription:
				if !strings.HasPrefix(d.ReportText, "## ") {
					t.Errorf("description = %q", d.ReportText)
				}
				// The names are the output's contract, which chief.md
				// documents, so they are written out here.
				reasons := map[DescriptionReason]string{DescriptionReasonNoClaudePane: "no_claude_pane", DescriptionReasonNoCandidate: "no_candidate", DescriptionReasonSeveralCandidates: "several_candidates"}
				results := map[PaneReportKind]string{PaneReportUnreadable: "unreadable", PaneReportAbsent: "no_report", PaneReportNotNamed: "not_named", PaneReportCandidate: "candidate"}
				var want []string
				for _, k := range c.found {
					want = append(want, results[k])
				}
				var got []string
				for i, p := range reasonPanes(reason) {
					got = append(got, p.Result)
					// A pane with no report or an unreadable one says why, with
					// its transcript's path.
					if i < len(c.found) && (c.found[i] == PaneReportAbsent || c.found[i] == PaneReportUnreadable) && !strings.Contains(p.Detail, sw.config) {
						t.Errorf("pane %s detail = %q; want the transcript's path", p.Pane, p.Detail)
					}
				}
				if reason == nil || reason.Reason != reasons[c.reason] || !slices.Equal(got, want) {
					t.Errorf("output reason = %+v; want %s with %v", reason, reasons[c.reason], want)
				}
			}
		})
	}
}

// reasonPanes returns the panes of a description reason, none for nil.
func reasonPanes(r *DescriptionReasonOutput) []DescriptionPaneOutput {
	if r == nil {
		return nil
	}
	return r.Panes
}

// TestDetailCapsALongReport checks that a report longer than the cap is
// shown from its head, cut at the cap and marked so, still as the pane's
// report.
func TestDetailCapsALongReport(t *testing.T) {
	sw, tree := detailWorld(t)
	deps := sw.finish()
	long := makerReport + "\n\n" + strings.Repeat("x", headMaxBytes)
	sw.endWithReport(sw.firstPane(), long)
	d := detailOf(t, deps, tree)
	if d.Report != ReportFromPane || !d.ReportTruncated || len(d.ReportText) > headMaxBytes || !strings.HasPrefix(d.ReportText, makerReport) {
		t.Errorf("source %s, truncated %v, %d bytes; want the report's head, cut at %d bytes", d.Report, d.ReportTruncated, len(d.ReportText), headMaxBytes)
	}
}

// TestDetailKeepsTheListedState checks that the kept report holds the
// pane's state as the workspace's list gave it before the transcript was
// read, not as a later read of the pane gives it, so a later reset of the
// pane stops as one that changed.
func TestDetailKeepsTheListedState(t *testing.T) {
	sw, tree := detailWorld(t)
	deps := sw.finish()
	p := sw.firstPane()
	sw.endWithReport(p, makerReport)
	later := p.fakeAgent
	later.Seq += 5
	sw.rec.replyFunc("herdr agent get w1:p1", func() reply { return reply{out: agentGetOutput(later)} })
	d := detailOf(t, deps, tree)
	if d.Report != ReportFromPane || d.record == nil || d.record.Seq != int64(p.Seq) {
		t.Fatalf("detail = %s, record %+v; want the report kept with state %d", d.Report, d.record, p.Seq)
	}
	out := sendRunpr(t, deps, tree)
	if len(out.Actions) != 2 || out.Actions[1].Outcome != OutcomeNotSent || out.Actions[1].Reason != NotSentPaneChanged {
		t.Errorf("actions = %+v; want the reset not sent, pane_changed", out.Actions)
	}
}

// TestDetailPartialFailure checks that an item that cannot be read is
// marked so and the others are still given.
func TestDetailPartialFailure(t *testing.T) {
	t.Run("view", func(t *testing.T) {
		sw, tree := detailWorld(t)
		sw.rec.failIn(tree, "gh pr view 42 --json "+prViewFields, errNoReply)
		d := detailOf(t, sw.finish(), tree)
		if d.ViewErr == "" || d.View != nil || len(d.Checks) != 7 || d.PR.Number != 42 {
			t.Errorf("detail = %+v; want the view unavailable and the checks given", d)
		}
	})
	t.Run("checks", func(t *testing.T) {
		sw, tree := detailWorld(t)
		sw.rec.failIn(tree, "gh pr checks 42 --json "+prChecksFields, errNoReply)
		d := detailOf(t, sw.finish(), tree)
		if d.ChecksErr == "" || d.View == nil || d.Report != ReportFromDescription {
			t.Errorf("detail = %+v; want the checks unavailable and the rest given", d)
		}
	})
	for name, edit := range map[string]func(map[string]any){
		"a view of another number": func(v map[string]any) { v["number"] = 43 },
		"a view of another URL":    func(v map[string]any) { v["url"] = prURL + "0" },
		"a view of a merged PR":    func(v map[string]any) { v["state"] = "MERGED" },
	} {
		t.Run(name, func(t *testing.T) {
			sw, tree := detailWorld(t)
			sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", edit))
			d := detailOf(t, sw.finish(), tree)
			if d.ViewErr == "" || d.View != nil {
				t.Errorf("detail = %+v; want the view rejected", d)
			}
		})
	}
}

// TestReviewSkippable checks the rule of "review skippable": every changed
// file is the task's plan or implementation handoff, and the list is whole.
func TestReviewSkippable(t *testing.T) {
	dir := "docs/tasks/" + taskDir
	plan, handoff := dir+"/"+planFile, dir+"/"+implementationHandoffFile
	cases := []struct {
		name    string
		files   []string
		changed int
		docsDir string
		want    bool
	}{
		{"the plan", []string{plan}, 1, dir, true},
		{"the plan and the handoff", []string{plan, handoff}, 2, dir, true},
		{"another file too", []string{plan, dir + "/" + architectureFile}, 2, dir, false},
		{"another task's plan", []string{"docs/tasks/0002_x/" + planFile}, 1, dir, false},
		{"a list shorter than the count", []string{plan}, 2, dir, false},
		{"no files", nil, 0, dir, false},
		{"no task directory", []string{plan}, 1, "", false},
	}
	for _, c := range cases {
		if got := reviewSkippable(prView{Files: c.files, ChangedFiles: c.changed}, c.docsDir); got != c.want {
			t.Errorf("%s: reviewSkippable = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestDetailAfterReset checks that the report kept by a detail is returned
// by a later detail for the same head after the pane was emptied, with its
// source still the pane's report.
func TestDetailAfterReset(t *testing.T) {
	sw, tree := detailWorld(t)
	deps := sw.finish()
	p := sw.firstPane()
	sw.endWithReport(p, makerReport)
	first := detailOf(t, deps, tree)
	if first.Report != ReportFromPane || first.RecordErr != "" {
		t.Fatalf("first detail = %+v", first)
	}
	if err := sw.realReact(p, "/clear"); err != nil {
		t.Fatal(err)
	}
	second := detailOf(t, deps, tree)
	if second.Report != ReportFromPane || second.ReportText != first.ReportText {
		t.Errorf("second detail = %s %q, want the kept report", second.Report, second.ReportText)
	}
}

// TestDetailReportRecord checks the report record: when only a detail of the
// description is kept and the report is written later, chief detail keeps a
// report record once, which a later detail and the reset use and a third
// detail never replaces; a reading that finds no single report keeps none;
// and the detail gathered just before /runpr never keeps one.
func TestDetailReportRecord(t *testing.T) {
	setup := func(t *testing.T) (*sendWorld, string, Deps, WorkspaceRecords) {
		t.Helper()
		sw, tree := detailWorld(t)
		deps := sw.finish()
		if d := detailOf(t, deps, tree); d.Report != ReportFromDescription || d.RecordErr != "" {
			t.Fatalf("first detail = %s, record error %q; want the description kept", d.Report, d.RecordErr)
		}
		records, err := deps.Store.Records(Worktree{path: tree})
		if err != nil {
			t.Fatal(err)
		}
		return sw, tree, deps, records
	}
	reportKept := func(t *testing.T, records WorkspaceRecords) (detailRecord, bool) {
		t.Helper()
		var rec detailRecord
		err := records.store().read(reportName(42, prHead), &rec)
		if err != nil && !errors.Is(err, privatefile.ErrNotExist) {
			t.Fatal(err)
		}
		return rec, err == nil
	}
	t.Run("kept once and used", func(t *testing.T) {
		sw, tree, deps, records := setup(t)
		p := sw.firstPane()
		sw.endWithReport(p, makerReport)
		if d := detailOf(t, deps, tree); d.Report != ReportFromPane || d.ReportText != makerReport || d.RecordErr != "" {
			t.Fatalf("second detail = %s %q, record error %q; want the report", d.Report, d.ReportText, d.RecordErr)
		}
		first, ok := reportKept(t, records)
		if !ok || first.Source != ReportFromPane.String() || first.Pane != "w1:p1" {
			t.Fatalf("report record = %+v, %v; want one from w1:p1", first, ok)
		}
		sw.appendTranscript(p, typedLine("and once more"), answerLine("msg_again", "Another report."))
		if d := detailOf(t, deps, tree); d.ReportText != makerReport {
			t.Errorf("third detail = %q; want the kept report", d.ReportText)
		}
		if again, _ := reportKept(t, records); again != first {
			t.Errorf("the report record changed to %+v", again)
		}
		// The pane's last report is the kept one again, as /mkarch left it.
		sw.setTranscript(p, commandLine("/mkarch 0001"), openPRLink, answerLine("msg_report", makerReport))
		out := sendRunpr(t, deps, tree)
		if len(out.Actions) != 2 || out.Actions[1].Outcome != OutcomeSent || out.Actions[1].Pane != "w1:p1" {
			t.Errorf("actions = %+v; want the reset of w1:p1 sent", out.Actions)
		}
	})
	t.Run("no single report", func(t *testing.T) {
		sw, tree, deps, records := setup(t)
		sw.appendTranscript(sw.firstPane(), answerLine("msg_report", makerReport))
		d := detailOf(t, deps, tree)
		if d.Report != ReportFromDescription || d.DescriptionReason != DescriptionReasonNoCandidate {
			t.Errorf("detail = %s, reason %s; want the kept description, no candidate", d.Report, d.DescriptionReason)
		}
		if _, ok := reportKept(t, records); ok {
			t.Error("a report record was kept without a report")
		}
	})
	t.Run("not before /runpr", func(t *testing.T) {
		sw, tree, deps, records := setup(t)
		sw.endWithReport(sw.firstPane(), makerReport)
		// The worktree's HEAD moved past the PR's head, so the detail before
		// /runpr finds no record for it and gathers one of the PR's head.
		sw.git(tree, "rev-parse HEAD", strings.Repeat("f", 40)+"\n")
		out := sendRunpr(t, deps, tree)
		if _, ok := reportKept(t, records); ok {
			t.Error("the detail before /runpr kept a report record")
		}
		if len(out.Actions) != 2 || out.Actions[1].Reason != NotSentNoReport {
			t.Errorf("actions = %+v; want the reset not sent for no kept report", out.Actions)
		}
	})
}

// TestDetailKeepsALargeDescription checks that a detail whose description
// is at the read cap and made of characters JSON escapes is kept and read
// back, so a later detail and the reset still find it.
func TestDetailKeepsALargeDescription(t *testing.T) {
	sw, tree := detailWorld(t)
	big := strings.Repeat("<", headMaxBytes+10)
	sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", func(v map[string]any) { v["body"] = big }))
	deps := sw.finish()
	for i := range 2 {
		d := detailOf(t, deps, tree)
		if d.RecordErr != "" || d.record == nil || d.Report != ReportFromDescription || !d.ReportTruncated || len(d.ReportText) != headMaxBytes {
			t.Fatalf("detail %d: record error %q, source %s, %d bytes", i, d.RecordErr, d.Report, len(d.ReportText))
		}
	}
}

// TestDetailMarksCappedCommits checks that a PR for which gh returns its cap
// of commits is shown as having that many or more, with where the total is,
// and one below the cap is not.
func TestDetailMarksCappedCommits(t *testing.T) {
	for _, c := range []struct {
		commits int
		capped  bool
	}{{ghCommitsCap - 1, false}, {ghCommitsCap, true}} {
		sw, tree := detailWorld(t)
		sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", func(v map[string]any) {
			one := v["commits"].([]any)[0]
			commits := make([]any, c.commits)
			for i := range commits {
				commits[i] = one
			}
			v["commits"] = commits
		}))
		out, err := Detail(context.Background(), sw.finish(), tree)
		if err != nil {
			t.Fatal(err)
		}
		size := out.Output().Size
		if size == nil || size.Commits != c.commits || size.CommitsAtLeast != c.capped || (size.CommitsNote != "") != c.capped {
			t.Errorf("%d commits: size = %+v, want capped %v", c.commits, size, c.capped)
		}
	}
}

// TestReviewItemShowsKeptSkippable checks that a review item is marked
// review-skippable only from a detail kept for the worktree's HEAD that
// judged it so: not before a detail is gathered, not from a detail that
// judged it not skippable, and not after HEAD moves on.
func TestReviewItemShowsKeptSkippable(t *testing.T) {
	plan := "docs/tasks/" + taskDir + "/" + planFile
	file := func(path string) any {
		return map[string]any{"path": path, "additions": 1, "deletions": 0, "changeType": "MODIFIED"}
	}
	for _, tc := range []struct {
		name  string
		files []any
		want  bool
	}{
		{"the plan alone", []any{file(plan)}, true},
		// A kept detail that judged the review not skippable never marks it.
		{"the plan and code", []any{file(plan), file("cmd/chief/main.go")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sw, tree := detailWorld(t)
			sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", func(v map[string]any) {
				v["files"], v["changedFiles"] = tc.files, len(tc.files)
			}))
			deps := sw.finish()
			skippable := func() bool {
				t.Helper()
				snap, err := TakeSnapshot(context.Background(), deps, sw.owner())
				if err != nil {
					t.Fatal(err)
				}
				out := snap.Output()
				if len(out.Pending) != 1 || out.Pending[0].Kind != "review" {
					t.Fatalf("pending = %+v, want one review item", out.Pending)
				}
				// The one item keeps the session's first number in every round.
				if out.Pending[0].Number != 1 {
					t.Errorf("the review item's number is %d, want 1", out.Pending[0].Number)
				}
				return out.Pending[0].ReviewSkippable
			}
			if skippable() {
				t.Error("the review item is skippable before any detail was gathered")
			}
			if d := detailOf(t, deps, tree); d.ReviewSkippable != tc.want {
				t.Fatalf("detail = %+v, want review skippable %v", d, tc.want)
			}
			if got := skippable(); got != tc.want {
				t.Errorf("the review item's skippable = %v after its detail, want %v", got, tc.want)
			}
			sw.git(tree, "rev-parse HEAD", strings.Repeat("f", 40)+"\n")
			if skippable() {
				t.Error("the review item is skippable from a detail of another HEAD")
			}
		})
	}
}
