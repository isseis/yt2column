//go:build test && unix

package chief

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/cmdrunner"
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
	sw := newSendWorld(t)
	tree := sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: taskBranch, PRs: openPRJSON}, "claude", "opencode")
	sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", nil))
	sw.rec.replyIn(tree, "gh pr checks 42 --json "+prChecksFields, readTestdata(t, "gh/pr-checks-200-fail.json"))
	sw.git(tree, "rev-parse HEAD", prHead+"\n")
	return sw, tree
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

// TestDetailReportSource checks where the summary comes from: the one pane
// whose screen holds the command's report naming the PR, else the PR
// description, and that the source says which.
func TestDetailReportSource(t *testing.T) {
	cases := []struct {
		name   string
		screen func(t *testing.T) string
		both   bool
		want   ReportSource
	}{
		{"the report on one pane", func(t *testing.T) string { return reportScreen(t, prURL) }, false, ReportFromScreen},
		{"a report naming another PR", func(t *testing.T) string { return reportScreen(t, prURL+"0") }, false, ReportFromDescription},
		{"no report", func(t *testing.T) string { return readScreenSample(t, "claude-idle.ansi") }, false, ReportFromDescription},
		{"the report on two panes", func(t *testing.T) string { return reportScreen(t, prURL) }, true, ReportFromDescription},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sw, tree := detailWorld(t)
			if c.both {
				p := &simPane{fakeAgent: fakeAgent{Pane: "w1:p3", Workspace: "w1", Kind: "claude", Status: "idle", Session: sw.newSession(), Dir: tree, Seq: 10}}
				sw.panes = append(sw.panes, p)
				sw.byPane[p.Pane] = p
				p.screen = c.screen(t)
			}
			deps := sw.finish()
			sw.firstPane().screen = c.screen(t)
			d := detailOf(t, deps, tree)
			if d.Report != c.want {
				t.Fatalf("source = %s, want %s", d.Report, c.want)
			}
			switch c.want {
			case ReportFromScreen:
				if !isSampleReport(d.ReportText) {
					t.Errorf("report = %q", d.ReportText)
				}
			case ReportFromDescription:
				if !strings.HasPrefix(d.ReportText, "## ") {
					t.Errorf("description = %q", d.ReportText)
				}
			}
		})
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
	t.Run("a view of another PR", func(t *testing.T) {
		sw, tree := detailWorld(t)
		sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", func(v map[string]any) { v["state"] = "MERGED" }))
		d := detailOf(t, sw.finish(), tree)
		if d.ViewErr == "" || d.View != nil {
			t.Errorf("detail = %+v; want the view rejected", d)
		}
	})
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
// source still the report.
func TestDetailAfterReset(t *testing.T) {
	sw, tree := detailWorld(t)
	deps := sw.finish()
	sw.firstPane().screen = reportScreen(t, prURL)
	first := detailOf(t, deps, tree)
	if first.Report != ReportFromScreen || first.RecordErr != "" {
		t.Fatalf("first detail = %+v", first)
	}
	sw.firstPane().screen = readScreenSample(t, "claude-after-clear.ansi")
	second := detailOf(t, deps, tree)
	if second.Report != ReportFromScreen || second.ReportText != first.ReportText {
		t.Errorf("second detail = %s %q, want the kept report", second.Report, second.ReportText)
	}
}

// TestDetailKeepsALargeDescription checks that a detail whose description
// is at the read cap and made of characters JSON escapes is kept and read
// back, so a later detail and the reset still find it.
func TestDetailKeepsALargeDescription(t *testing.T) {
	sw, tree := detailWorld(t)
	big := strings.Repeat("<", descriptionMaxBytes+10)
	sw.rec.replyIn(tree, "gh pr view 42 --json "+prViewFields, viewOf(t, "pr-view-867-open.json", func(v map[string]any) { v["body"] = big }))
	deps := sw.finish()
	for i := range 2 {
		d := detailOf(t, deps, tree)
		if d.RecordErr != "" || d.record == nil || d.Report != ReportFromDescription || !d.ReportTruncated || len(d.ReportText) != descriptionMaxBytes {
			t.Fatalf("detail %d: record error %q, source %s, %d bytes", i, d.RecordErr, d.Report, len(d.ReportText))
		}
	}
}
