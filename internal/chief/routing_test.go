//go:build test

package chief

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// sonnetReviewRow is the routing row of sonnet+opus-review in the
// repository's _context.md.
const sonnetReviewRow = "| `runplan-sonnet+opus-review` | `/runplan` for the requirement `sonnet+opus-review` | Claude Code | `sonnet` |"

// TestRoutingReadsRepositoryContext reads the routing table of the
// repository's own _context.md: every route key has a route.
func TestRoutingReadsRepositoryContext(t *testing.T) {
	routing, err := ParseRouting(repoContext(t))
	if err != nil {
		t.Fatalf("ParseRouting(_context.md): %v", err)
	}
	for _, key := range paneinput.RouteKeys() {
		route, err := routing.Route(key)
		if err != nil {
			t.Errorf("route %d: %v", key, err)
			continue
		}
		if _, err := route.ModelText(); err != nil {
			t.Errorf("route %d has no model: %v", key, err)
		}
	}
	for _, key := range []paneinput.RouteKey{paneinput.RouteUnknown, paneinput.RouteKey(42)} {
		if _, err := routing.Route(key); !errors.Is(err, errRouting) {
			t.Errorf("Route(%d) error = %v, want errRouting", key, err)
		}
	}
}

// TestRoutingRowsMapToTheirKeys gives every row of a table a distinct model
// and checks that each route key gets its own row's.
func TestRoutingRowsMapToTheirKeys(t *testing.T) {
	rows := map[paneinput.RouteKey]string{
		paneinput.RouteTaskDocuments:       "task-documents",
		paneinput.RouteRunplanOpus:         "runplan-opus",
		paneinput.RouteRunplanSonnetReview: "runplan-sonnet+opus-review",
		paneinput.RouteRunplanSonnet:       "runplan-sonnet",
		paneinput.RouteRunplanTakeover:     "runplan-takeover",
		paneinput.RouteRunpr:               "runpr",
	}
	doc := "### Command routing\n\n| Row | Command | Agent | Model |\n|---|---|---|---|\n"
	for _, name := range rows {
		doc += "| `" + name + "` | x | OpenCode | `Model " + name + "` |\n"
	}
	routing, err := ParseRouting([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	for key, name := range rows {
		route, err := routing.Route(key)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := route.ModelText(); got != "Model "+name {
			t.Errorf("route %d has the model of %q, want row %q", key, got, name)
		}
	}
}

// TestRoutingRowChangesOneRoute moves the sonnet+opus-review row to OpenCode
// in a copy of _context.md and checks that only that route changes.
func TestRoutingRowChangesOneRoute(t *testing.T) {
	doc := string(repoContext(t))
	moved := strings.Replace(sonnetReviewRow, "| Claude Code | `sonnet` |", "| OpenCode | `DeepSeek V4.1 Flash 1017 OpenCode Go` |", 1)
	edited := strings.Replace(doc, sonnetReviewRow, moved, 1)
	if edited == doc {
		t.Fatalf("_context.md has no row %q", sonnetReviewRow)
	}
	before, err := ParseRouting([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	after, err := ParseRouting([]byte(edited))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range paneinput.RouteKeys() {
		was, _ := before.Route(key)
		now, _ := after.Route(key)
		changed := was != now
		if changed != (key == paneinput.RouteRunplanSonnetReview) {
			t.Errorf("route %d changed = %v (%+v -> %+v)", key, changed, was, now)
		}
	}
	now, _ := after.Route(paneinput.RouteRunplanSonnetReview)
	if text, _ := now.ModelText(); now.Agent() != paneinput.AgentOpenCode || text != "DeepSeek V4.1 Flash 1017 OpenCode Go" {
		t.Errorf("moved route = %d %q", now.Agent(), text)
	}
}

// TestRoutingRejects edits the repository's table into shapes the reader
// must reject as a whole.
func TestRoutingRejects(t *testing.T) {
	doc := string(repoContext(t))
	row := func(model string) string {
		return strings.Replace(sonnetReviewRow, "`sonnet` |", model+" |", 1)
	}
	edits := map[string][2]string{
		"missing row":          {sonnetReviewRow + "\n", ""},
		"repeated row":         {sonnetReviewRow, sonnetReviewRow + "\n" + sonnetReviewRow},
		"unknown row":          {sonnetReviewRow, sonnetReviewRow + "\n| `runpr-fast` | `/runpr` | OpenCode | `a` |"},
		"row name not code":    {sonnetReviewRow, sonnetReviewRow + "\n| runpr | `/runpr` | OpenCode | `a` |"},
		"extra column":         {sonnetReviewRow, sonnetReviewRow + " x |"},
		"other agent":          {"| `runpr` | `/runpr` | OpenCode |", "| `runpr` | `/runpr` | Codex |"},
		"Claude model outside": {sonnetReviewRow, row("`haiku`")},
		"Claude model as name": {sonnetReviewRow, row("`claude-sonnet-5-5`")},
		"model not code":       {sonnetReviewRow, row("sonnet")},
		"OpenCode model blank": {"`DeepSeek V4.1 Flash 1017 OpenCode Go` |\n| `runplan-takeover`", "`DeepSeek  V4.1` |\n| `runplan-takeover`"},
		"empty command":        {"| `runpr` | `/runpr` |", "| `runpr` |  |"},
		"header changed":       {"| Row | Command | Agent | Model |", "| Row | Command | Model | Agent |"},
		"heading twice":        {"### PR title and squash subject\n", "### Command routing\n\n| Row | Command | Agent | Model |\n|---|---|---|---|\n\n### PR title and squash subject\n"},
		"heading missing":      {"### Command routing\n", "### Command table\n"},
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			edited := strings.Replace(doc, edit[0], edit[1], 1)
			if edited == doc {
				t.Fatalf("_context.md has no %q", edit[0])
			}
			if _, err := ParseRouting([]byte(edited)); !errors.Is(err, errRouting) {
				t.Errorf("error = %v, want errRouting", err)
			}
		})
	}
}

// TestReadRoutingFromOriginMain checks that the table is read from
// origin/main's _context.md in the worktree.
func TestReadRoutingFromOriginMain(t *testing.T) {
	rec := newRecorder(t)
	rec.reply("git show refs/remotes/origin/main:.claude/commands/_context.md", repoContext(t))
	if _, err := ReadRouting(context.Background(), rec.commands(nil), "/work/tree"); err != nil {
		t.Fatal(err)
	}
	if calls := rec.recorded(); len(calls) != 1 || calls[0].dir != "/work/tree" {
		t.Errorf("calls = %+v", calls)
	}
}
