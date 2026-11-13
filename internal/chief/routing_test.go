//go:build test

package chief

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// repositoryRow returns the row of the repository's routing table whose Row
// cell is rowName, so a test finds a route without pinning the row's other
// text: the agent and model of a row are values the table is free to change.
func repositoryRow(t *testing.T, rowName string) string {
	t.Helper()
	for line := range strings.SplitSeq(string(repoContext(t)), "\n") {
		if strings.HasPrefix(line, "| `"+rowName+"` |") {
			return line
		}
	}
	t.Fatalf("_context.md has no row %q", rowName)
	return ""
}

// rowWithCell returns a routing row with cell i replaced, cells being numbered
// from the Row cell at 0.
func rowWithCell(t *testing.T, row string, i int, cell string) string {
	t.Helper()
	cells := strings.Split(row, " | ")
	if len(cells) != 4 || i < 0 || i > 3 {
		t.Fatalf("row %q cannot be edited at cell %d", row, i)
	}
	cells[i] = cell
	if i == 0 {
		cells[0] = "| " + cell
	}
	if i == 3 {
		cells[3] = cell + " |"
	}
	return strings.Join(cells, " | ")
}

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

// TestRoutingRowChangesOneRoute moves the sonnet+opus-review row to the other
// agent in a copy of _context.md and checks that only that route changes.
func TestRoutingRowChangesOneRoute(t *testing.T) {
	doc := string(repoContext(t))
	before, err := ParseRouting([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	current, err := before.Route(paneinput.RouteRunplanSonnetReview)
	if err != nil {
		t.Fatal(err)
	}
	// Move the route to the other agent, so the edit changes the row whichever
	// agent the table currently sends it to.
	agent, model := "OpenCode", "DeepSeek V4.1 Flash 1017 OpenCode Go"
	wantAgent := paneinput.AgentOpenCode
	if current.Agent() == paneinput.AgentOpenCode {
		agent, model = "Claude Code", "opus"
		wantAgent = paneinput.AgentClaudeCode
	}
	row := repositoryRow(t, "runplan-sonnet+opus-review")
	moved := rowWithCell(t, rowWithCell(t, row, 2, agent), 3, "`"+model+"`")
	edited := strings.Replace(doc, row, moved, 1)
	if edited == doc {
		t.Fatalf("_context.md has no row for runplan-sonnet+opus-review")
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
	if text, _ := now.ModelText(); now.Agent() != wantAgent || text != model {
		t.Errorf("moved route = %d %q, want %d %q", now.Agent(), text, wantAgent, model)
	}
}

// TestRoutingRejects edits the repository's table into shapes the reader
// must reject as a whole.
func TestRoutingRejects(t *testing.T) {
	doc := string(repoContext(t))
	review := repositoryRow(t, "runplan-sonnet+opus-review")
	sonnet := repositoryRow(t, "runplan-sonnet")
	runpr := repositoryRow(t, "runpr")
	edits := map[string][2]string{
		"missing row":          {review + "\n", ""},
		"repeated row":         {review, review + "\n" + review},
		"unknown row":          {review, review + "\n| `runpr-fast` | `/runpr` | OpenCode | `a` |"},
		"row name not code":    {review, rowWithCell(t, review, 0, "runplan-sonnet+opus-review")},
		"extra column":         {review, review + " x |"},
		"other agent":          {runpr, rowWithCell(t, runpr, 2, "Codex")},
		"Claude model outside": {review, rowWithCell(t, rowWithCell(t, review, 2, "Claude Code"), 3, "`haiku`")},
		"Claude model as name": {review, rowWithCell(t, rowWithCell(t, review, 2, "Claude Code"), 3, "`claude-sonnet-5-5`")},
		"model not code":       {review, rowWithCell(t, rowWithCell(t, review, 2, "Claude Code"), 3, "sonnet")},
		"OpenCode model blank": {sonnet, rowWithCell(t, rowWithCell(t, sonnet, 2, "OpenCode"), 3, "`DeepSeek  V4.1`")},
		"empty command":        {runpr, rowWithCell(t, runpr, 1, "")},
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
