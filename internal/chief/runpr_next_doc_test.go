//go:build test

package chief

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// runprDoc is the repository's /runpr command document, relative to this
// package's directory.
var runprDoc = filepath.Join("..", "..", ".claude", "commands", "runpr.md")

// runprNextSections are the headings of the /runpr document whose NEXT line
// examples the tool must read: the merge question of step 4 and the final
// report of step 5.
var runprNextSections = []string{
	"## 4. Confirm, record, and merge",
	"## 5. Switch branch, then show NEXT",
}

// nextRoute is one way the tool reads a NEXT line from a /runpr report: its
// name, for a failure, and a reader giving the kind of NEXT it found.
type nextRoute struct {
	name string
	read func(text string) NextKind
}

// nextRoutes are every route the tool reads a report's NEXT through. A route
// added to the tool is added here, so the document's examples are checked
// through it.
var nextRoutes = []nextRoute{
	{name: "screen", read: func(text string) NextKind { return readRunprNext(text).Kind }},
	{name: "transcript", read: func(text string) NextKind { return readReportNext(text).Next.Kind }},
}

var (
	// nextExampleWord matches an example that starts with the word NEXT: the
	// word alone, followed by a colon, a space, or another non-word character.
	nextExampleWord = regexp.MustCompile(`^NEXT\b`)
	// codeSpanInProse matches one inline code span, its content allowed to
	// cross lines.
	codeSpanInProse = regexp.MustCompile("`([^`]+)`")
	// spanLineBreak matches a line break and the blanks after it, where a code
	// span over several lines is joined.
	spanLineBreak = regexp.MustCompile(`\r?\n[ \t]*`)
)

// checkRunprNextExamples returns a failure for each section of doc that is
// missing, holds no NEXT line example, or holds an example a route cannot read
// as none or a command. Contract: the examples are read from the document and
// never copied into the test, so an example a route can still read keeps the
// check passing.
func checkRunprNextExamples(doc string, routes []nextRoute) []string {
	var failures []string
	for _, heading := range runprNextSections {
		section := doctestutil.SectionOutsideBlocks(doc, heading)
		if section == "" {
			failures = append(failures, fmt.Sprintf("%q: section not found", heading))
			continue
		}
		examples := nextLineExamples(section)
		if len(examples) == 0 {
			failures = append(failures, fmt.Sprintf("%q: no NEXT line example", heading))
			continue
		}
		for _, example := range examples {
			for _, route := range routes {
				if kind := route.read(example); kind != NextNone && kind != NextCommand {
					failures = append(failures, fmt.Sprintf("%q: the %s route cannot read %q", heading, route.name, example))
				}
			}
		}
	}
	return failures
}

// nextLineExamples returns the NEXT line examples of a section: the code spans
// outside its fenced code blocks whose text, with its surrounding spaces
// removed, starts with the word NEXT after an optional list bullet. A span
// whose whole text is NEXT or NEXT: only names the form and is not an example.
// A span over several lines is joined with one space, as one line of a report
// reads.
func nextLineExamples(section string) []string {
	var prose strings.Builder
	for _, line := range doctestutil.ScanDocLines(slices.Collect(strings.Lines(section))) {
		if !line.Block {
			prose.WriteString(line.Text)
		}
		prose.WriteString("\n")
	}
	var examples []string
	for _, m := range codeSpanInProse.FindAllStringSubmatch(prose.String(), -1) {
		example := strings.TrimSpace(spanLineBreak.ReplaceAllString(m[1], " "))
		text := strings.TrimPrefix(example, "- ")
		if !nextExampleWord.MatchString(text) || text == "NEXT" || text == "NEXT:" {
			continue
		}
		// Keep a list bullet in the example: the report grammar allows it, so
		// the readers are checked against the form as the document writes it.
		examples = append(examples, example)
	}
	return examples
}

// TestNextLineExamples checks the rules that pick the examples out of a
// section: spans outside fenced blocks, a list bullet kept, a span over two
// lines joined, and a span that only names the form dropped. The fixtures are
// not the document's text.
func TestNextLineExamples(t *testing.T) {
	cases := []struct {
		name    string
		section string
		want    []string
	}{
		{
			name:    "outside a fence",
			section: "```text\n`NEXT: /mkplan 4343`\n```\nAfter: `NEXT: /mkplan 4242`\n",
			want:    []string{"NEXT: /mkplan 4242"},
		},
		{
			name:    "after a list bullet",
			section: "- `- NEXT: /mkplan 4242`\n",
			want:    []string{"- NEXT: /mkplan 4242"},
		},
		{
			name:    "over two lines",
			section: "`NEXT: /mkplan\n4242`\n",
			want:    []string{"NEXT: /mkplan 4242"},
		},
		{
			name:    "only the form",
			section: "Write `NEXT` or `NEXT: ` before the command.\n",
			want:    nil,
		},
	}
	for _, c := range cases {
		if got := nextLineExamples(c.section); !slices.Equal(got, c.want) {
			t.Errorf("%s: nextLineExamples = %q; want %q", c.name, got, c.want)
		}
	}
}

// TestCheckRunprNextExamples checks the failures the document check reports:
// a missing section, a section with no example, an example a route cannot
// read, and a route that reads none of them. The fixtures are not the
// document's text.
func TestCheckRunprNextExamples(t *testing.T) {
	const (
		step4 = "## 4. Confirm, record, and merge\n"
		step5 = "## 5. Switch branch, then show NEXT\n"
		valid = step4 + "\n`- NEXT: /mkplan 4242`\n\n" + step5 + "\n`NEXT: /runplan 4242`\n"
	)
	brokenRoute := nextRoute{name: "broken", read: func(string) NextKind { return NextUnmatched }}
	cases := []struct {
		name   string
		doc    string
		routes []nextRoute
		want   []string
	}{
		{name: "both sections readable", doc: valid},
		{
			name: "missing section",
			doc:  step5 + "\n`NEXT: /runplan 4242`\n",
			want: []string{"section not found"},
		},
		{
			name: "no example",
			doc:  step4 + "\n`NEXT` and `NEXT: `\n\n" + step5 + "\n`NEXT: /runplan 4242`\n",
			want: []string{"no NEXT line example"},
		},
		{
			name: "unreadable example",
			doc:  step4 + "\n`NEXT /mkplan 4242`\n\n" + step5 + "\n`NEXT: /runplan 4242`\n",
			want: []string{"the screen route cannot read", "the transcript route cannot read"},
		},
		{
			name:   "one route broken",
			doc:    valid,
			routes: []nextRoute{brokenRoute, nextRoutes[1]},
			want:   []string{"the broken route cannot read", "the broken route cannot read"},
		},
	}
	for _, c := range cases {
		routes := c.routes
		if routes == nil {
			routes = nextRoutes
		}
		got := checkRunprNextExamples(c.doc, routes)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d failures %q; want %d", c.name, len(got), got, len(c.want))
			continue
		}
		for i, want := range c.want {
			if !strings.Contains(got[i], want) {
				t.Errorf("%s: failure %q does not hold %q", c.name, got[i], want)
			}
		}
	}
}

// TestRunprNextExamplesReadable checks that every NEXT line example of the
// /runpr document's step 4 and step 5 is one the tool reads as none or a
// command, through each route it reads a report's NEXT by.
func TestRunprNextExamplesReadable(t *testing.T) {
	doc := doctestutil.ReadDoc(t, runprDoc)
	for _, failure := range checkRunprNextExamples(doc, nextRoutes) {
		t.Error(failure)
	}
}
