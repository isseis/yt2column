//go:build test

package chief

import (
	"errors"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// TestNewRunningCommand checks, for each of the five kinds, an argument of
// the accepted form, no argument, and arguments that must be dropped, and
// that a command the constructor makes shows as the text typed into a pane.
func TestNewRunningCommand(t *testing.T) {
	cases := []struct {
		name        string
		kind        paneinput.CommandKind
		arg         string
		wantText    string
		wantDropped bool
	}{
		{"mkreq with an issue", paneinput.CommandMkreq, "890", "/mkreq 890", false},
		{"mkarch with a task", paneinput.CommandMkarch, "0093", "/mkarch 0093", false},
		{"mkplan with a task", paneinput.CommandMkplan, "0093", "/mkplan 0093", false},
		{"runplan with a task", paneinput.CommandRunplan, "0093", "/runplan 0093", false},
		{"runpr with a PR", paneinput.CommandRunpr, "926", "/runpr 926", false},
		{"no argument", paneinput.CommandRunplan, "", "/runplan", false},
		{"a second argument", paneinput.CommandMkreq, "890 0093", "/mkreq", true},
		{"a control character", paneinput.CommandRunplan, "00\x1b93", "/runplan", true},
		{"a newline that would start another command", paneinput.CommandRunplan, "0093\n/runpr 1", "/runplan", true},
		{"a text that is not a number", paneinput.CommandRunpr, "main", "/runpr", true},
		{"an issue with a leading zero", paneinput.CommandMkreq, "0890", "/mkreq", true},
		{"a task that is not four digits", paneinput.CommandMkarch, "93", "/mkarch", true},
		{"a PR number given to a task command", paneinput.CommandRunplan, "926", "/runplan", true},
		{"a task number given to /runpr", paneinput.CommandRunpr, "0093", "/runpr", true},
		{"a flag", paneinput.CommandRunpr, "--force", "/runpr", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd, dropped, err := newRunningCommand(c.kind, c.arg)
			if err != nil {
				t.Fatalf("newRunningCommand: %v", err)
			}
			if dropped != c.wantDropped || cmd.Text() != c.wantText || cmd.Kind() != c.kind {
				t.Errorf("newRunningCommand = %q, kind %d, dropped %v; want %q, kind %d, dropped %v", cmd.Text(), cmd.Kind(), dropped, c.wantText, c.kind, c.wantDropped)
			}
		})
	}
}

// TestNewRunningCommandRejectsOtherKinds checks that a kind outside the five
// task commands makes no command.
func TestNewRunningCommandRejectsOtherKinds(t *testing.T) {
	for _, kind := range []paneinput.CommandKind{paneinput.CommandUnknown, paneinput.CommandKind(-1), paneinput.CommandKind(99)} {
		cmd, dropped, err := newRunningCommand(kind, "0093")
		if !errors.Is(err, paneinput.ErrInvalid) || dropped || cmd != (RunningCommand{}) {
			t.Errorf("newRunningCommand(%d) = %+v, %v, %v; want the zero value and ErrInvalid", kind, cmd, dropped, err)
		}
	}
}

// TestRunningCommandTextMatchesThePaneInput checks that the text of each
// kind is the line chief types for the same command, so the name table and
// the typed text agree.
func TestRunningCommandTextMatchesThePaneInput(t *testing.T) {
	task, err := paneinput.ParseTaskID("0093")
	if err != nil {
		t.Fatal(err)
	}
	issue, err := paneinput.ParseIssueNumber("890")
	if err != nil {
		t.Fatal(err)
	}
	pr, err := fixprguard.ParsePRNumber("926")
	if err != nil {
		t.Fatal(err)
	}
	mkreq, _ := paneinput.NewMkreq(issue)
	mkarch, _ := paneinput.NewMkarch(task)
	mkplan, _ := paneinput.NewMkplan(task)
	runplan, _ := paneinput.NewRunplan(task, paneinput.RouteRunplanOpus)
	runpr, _ := paneinput.NewRunpr(pr)
	cases := []struct {
		kind paneinput.CommandKind
		arg  string
		sent paneinput.Command
	}{
		{paneinput.CommandMkreq, "890", mkreq},
		{paneinput.CommandMkarch, "0093", mkarch},
		{paneinput.CommandMkplan, "0093", mkplan},
		{paneinput.CommandRunplan, "0093", runplan},
		{paneinput.CommandRunpr, "926", runpr},
	}
	for _, c := range cases {
		sent, err := c.sent.Text()
		if err != nil {
			t.Fatal(err)
		}
		cmd, _, err := newRunningCommand(c.kind, c.arg)
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Text() != sent {
			t.Errorf("kind %d: Text() = %q, the pane input is %q", c.kind, cmd.Text(), sent)
		}
	}
}

// TestRunningCommandRevalidates checks that a value built inside the package
// without the constructor shows only what validates: an argument that does
// not validate is dropped, and a kind outside the five shows nothing. The
// constructor is the only way to a command with an unvalidated argument
// outside the package, and the text method guards the rest.
func TestRunningCommandRevalidates(t *testing.T) {
	cases := []struct {
		name string
		cmd  RunningCommand
		text string
		kind paneinput.CommandKind
	}{
		{"the zero value", RunningCommand{}, "", paneinput.CommandUnknown},
		{"a kind outside the five", RunningCommand{kind: paneinput.CommandKind(99), arg: "0093"}, "", paneinput.CommandUnknown},
		{"an argument that was not validated", RunningCommand{kind: paneinput.CommandRunplan, arg: "0093\n/runpr 1"}, "/runplan", paneinput.CommandRunplan},
		{"an argument of another kind's form", RunningCommand{kind: paneinput.CommandRunplan, arg: "926"}, "/runplan", paneinput.CommandRunplan},
		{"a valid argument", RunningCommand{kind: paneinput.CommandRunplan, arg: "0093"}, "/runplan 0093", paneinput.CommandRunplan},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.cmd.Text() != c.text || c.cmd.Kind() != c.kind {
				t.Errorf("Text() = %q, Kind() = %d; want %q, %d", c.cmd.Text(), c.cmd.Kind(), c.text, c.kind)
			}
		})
	}
}
