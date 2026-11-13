package chief

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// errRouting reports a routing table the tool cannot read. While it cannot be
// read, no command is offered or sent.
var errRouting = errors.New("unreadable routing table")

// contextPath is the repository path of _context.md.
const contextPath = ".claude/commands/_context.md"

// The routing table's heading and its fixed header rows.
const (
	routingHeading   = "### Command routing"
	routingHeader    = "| Row | Command | Agent | Model |"
	routingSeparator = "|---|---|---|---|"
)

// routingRows names each route key's row in the table.
var routingRows = map[string]paneinput.RouteKey{
	"task-documents":             paneinput.RouteTaskDocuments,
	"runplan-opus":               paneinput.RouteRunplanOpus,
	"runplan-sonnet+opus-review": paneinput.RouteRunplanSonnetReview,
	"runplan-sonnet":             paneinput.RouteRunplanSonnet,
	"runplan-takeover":           paneinput.RouteRunplanTakeover,
	"runpr":                      paneinput.RouteRunpr,
}

// Routing is the parsed routing table: one route for each route key.
type Routing struct {
	routes map[paneinput.RouteKey]paneinput.Route
}

// Route returns the route of key. The zero key, a key outside the range, and
// the zero Routing have none.
func (r Routing) Route(key paneinput.RouteKey) (paneinput.Route, error) {
	route, ok := r.routes[key]
	if !ok {
		return paneinput.Route{}, fmt.Errorf("%w: no route %d", errRouting, int(key))
	}
	return route, nil
}

// ParseRouting reads the routing table of _context.md. Contract: the table is
// read only in its fixed shape (its heading once, its header, then exactly
// one row for each route key and nothing else before a non-table line), and
// every agent and model must be one of the fixed sets; anything else rejects
// the whole table, so no command is sent by a table read in part.
func ParseRouting(contextDoc []byte) (Routing, error) {
	lines := docLines(contextDoc)
	start := -1
	for i, l := range lines {
		if l.fenced || l.text != routingHeading {
			continue
		}
		if start >= 0 {
			return Routing{}, fmt.Errorf("%w: %q twice", errRouting, routingHeading)
		}
		start = i + 1
	}
	if start < 0 {
		return Routing{}, fmt.Errorf("%w: no %q", errRouting, routingHeading)
	}
	var table []string
	for _, l := range lines[start:] {
		if l.fenced || anyHeading.MatchString(l.text) {
			break
		}
		if strings.HasPrefix(l.text, "|") {
			table = append(table, l.text)
			continue
		}
		if len(table) > 0 {
			break
		}
	}
	if len(table) < 2 || table[0] != routingHeader || table[1] != routingSeparator {
		return Routing{}, fmt.Errorf("%w: header", errRouting)
	}
	routes := map[paneinput.RouteKey]paneinput.Route{}
	for _, row := range table[2:] {
		key, route, err := parseRoutingRow(row)
		if err != nil {
			return Routing{}, err
		}
		if _, dup := routes[key]; dup {
			return Routing{}, fmt.Errorf("%w: row %d twice", errRouting, int(key))
		}
		routes[key] = route
	}
	for _, key := range paneinput.RouteKeys() {
		if _, ok := routes[key]; !ok {
			return Routing{}, fmt.Errorf("%w: no row for route %d", errRouting, int(key))
		}
	}
	return Routing{routes: routes}, nil
}

// parseRoutingRow reads one row: "| `<row>` | <command> | <agent> | `<model>` |".
func parseRoutingRow(row string) (paneinput.RouteKey, paneinput.Route, error) {
	inner, ok := strings.CutPrefix(row, "| ")
	if ok {
		inner, ok = strings.CutSuffix(inner, " |")
	}
	cells := strings.Split(inner, " | ")
	if !ok || len(cells) != 4 || strings.TrimSpace(cells[1]) == "" {
		return 0, paneinput.Route{}, fmt.Errorf("%w: row %q", errRouting, row)
	}
	name, ok := codeSpan(cells[0])
	key, known := routingRows[name]
	if !ok || !known {
		return 0, paneinput.Route{}, fmt.Errorf("%w: row name %q", errRouting, cells[0])
	}
	model, ok := codeSpan(cells[3])
	if !ok {
		return 0, paneinput.Route{}, fmt.Errorf("%w: model %q", errRouting, cells[3])
	}
	var route paneinput.Route
	var err error
	switch cells[2] {
	case "Claude Code":
		var m paneinput.ClaudeModel
		if m, err = paneinput.ParseClaudeModel(model); err == nil {
			route, err = paneinput.NewClaudeRoute(m)
		}
	case "OpenCode":
		var m paneinput.OpenCodeModel
		if m, err = paneinput.ParseOpenCodeModel(model); err == nil {
			route, err = paneinput.NewOpenCodeRoute(m)
		}
	default:
		return 0, paneinput.Route{}, fmt.Errorf("%w: agent %q", errRouting, cells[2])
	}
	if err != nil {
		return 0, paneinput.Route{}, fmt.Errorf("%w: %w", errRouting, err)
	}
	return key, route, nil
}

// codeSpan returns the text of a cell that is one code span and nothing else.
func codeSpan(cell string) (string, bool) {
	inner, ok := strings.CutPrefix(cell, "`")
	if !ok {
		return "", false
	}
	inner, ok = strings.CutSuffix(inner, "`")
	if !ok || inner == "" || strings.Contains(inner, "`") {
		return "", false
	}
	return inner, true
}

// ReadRouting reads the routing table from origin/main's _context.md in
// worktree, so a branch that edits the table cannot change where its own
// commands go.
func ReadRouting(ctx context.Context, cmds Commands, worktree string) (Routing, error) {
	out, err := cmds.git(ctx, worktree, "show", originMain+":"+contextPath)
	if err != nil {
		return Routing{}, err
	}
	return ParseRouting(out)
}
