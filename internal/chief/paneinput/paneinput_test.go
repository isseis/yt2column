//go:build test

package paneinput_test

import (
	"errors"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
)

// task0088 returns the task number 0088.
func task0088(t *testing.T) paneinput.TaskID {
	t.Helper()
	task, err := paneinput.ParseTaskID("0088")
	if err != nil {
		t.Fatalf("ParseTaskID: %v", err)
	}
	return task
}

func mustPR(t *testing.T, s string) fixprguard.PRNumber {
	t.Helper()
	pr, err := fixprguard.ParsePRNumber(s)
	if err != nil {
		t.Fatalf("ParsePRNumber(%q): %v", s, err)
	}
	return pr
}

func TestParseNumbers(t *testing.T) {
	t.Run("task", func(t *testing.T) {
		if got := task0088(t).String(); got != "0088" {
			t.Errorf("ParseTaskID(0088) = %q", got)
		}
		for _, s := range []string{"", "88", "00888", "008a", "-088", "0\uff18\uff188", " 088"} {
			if _, err := paneinput.ParseTaskID(s); !errors.Is(err, paneinput.ErrInvalid) {
				t.Errorf("ParseTaskID(%q) error = %v, want ErrInvalid", s, err)
			}
		}
	})
	t.Run("issue", func(t *testing.T) {
		issue, err := paneinput.ParseIssueNumber("836")
		if err != nil || issue.String() != "836" {
			t.Errorf("ParseIssueNumber(836) = %q, %v", issue.String(), err)
		}
		for _, s := range []string{"", "0", "0836", "+1", "-1", "1e3", "#836", "1234567890"} {
			if _, err := paneinput.ParseIssueNumber(s); !errors.Is(err, paneinput.ErrInvalid) {
				t.Errorf("ParseIssueNumber(%q) error = %v, want ErrInvalid", s, err)
			}
		}
	})
	t.Run("OpenCode model", func(t *testing.T) {
		const name = "DeepSeek V4.1 Flash 1017 OpenCode Go"
		m, err := paneinput.ParseOpenCodeModel(name)
		if err != nil || m.String() != name {
			t.Fatalf("ParseOpenCodeModel(%q) = %q, %v", name, m.String(), err)
		}
		for _, s := range []string{"", " DeepSeek", "DeepSeek ", "DeepSeek  V4.1", "DeepSeek\tV4.1", "DeepSeek \u00b7 V4.1", "a\x7f"} {
			if _, err := paneinput.ParseOpenCodeModel(s); !errors.Is(err, paneinput.ErrInvalid) {
				t.Errorf("ParseOpenCodeModel(%q) error = %v, want ErrInvalid", s, err)
			}
		}
	})
}

// TestOpenCodeModelMatchesWhole pins the whole-text comparison: a model name
// that is a prefix of the shown text does not match it.
func TestOpenCodeModelMatchesWhole(t *testing.T) {
	short, err := paneinput.ParseOpenCodeModel("DeepSeek V4.1")
	if err != nil {
		t.Fatal(err)
	}
	if short.Matches("DeepSeek V4.1 Flash 1017 OpenCode Go") {
		t.Error("a prefix of the shown model matched it")
	}
	if !short.Matches("DeepSeek V4.1") {
		t.Error("the same text did not match")
	}
	if (paneinput.OpenCodeModel{}).Matches("") {
		t.Error("the zero model matched an empty status line")
	}
}

// TestCommandText checks the line each constructor's command types.
func TestCommandText(t *testing.T) {
	task := task0088(t)
	issue, err := paneinput.ParseIssueNumber("836")
	if err != nil {
		t.Fatal(err)
	}
	build := map[string]struct {
		make  func() (paneinput.Command, error)
		text  string
		route paneinput.RouteKey
		kind  paneinput.CommandKind
		arg   string
	}{
		"mkreq":    {func() (paneinput.Command, error) { return paneinput.NewMkreq(issue) }, "/mkreq 836", paneinput.RouteTaskDocuments, paneinput.CommandMkreq, "836"},
		"mkarch":   {func() (paneinput.Command, error) { return paneinput.NewMkarch(task) }, "/mkarch 0088", paneinput.RouteTaskDocuments, paneinput.CommandMkarch, "0088"},
		"mkplan":   {func() (paneinput.Command, error) { return paneinput.NewMkplan(task) }, "/mkplan 0088", paneinput.RouteTaskDocuments, paneinput.CommandMkplan, "0088"},
		"runplan":  {func() (paneinput.Command, error) { return paneinput.NewRunplan(task, paneinput.RouteRunplanSonnet) }, "/runplan 0088", paneinput.RouteRunplanSonnet, paneinput.CommandRunplan, "0088"},
		"takeover": {func() (paneinput.Command, error) { return paneinput.NewRunplan(task, paneinput.RouteRunplanTakeover) }, "/runplan 0088", paneinput.RouteRunplanTakeover, paneinput.CommandRunplan, "0088"},
		"runpr":    {func() (paneinput.Command, error) { return paneinput.NewRunpr(mustPR(t, "870")) }, "/runpr 870", paneinput.RouteRunpr, paneinput.CommandRunpr, "870"},
	}
	for name, tc := range build {
		t.Run(name, func(t *testing.T) {
			c, err := tc.make()
			if err != nil {
				t.Fatalf("constructor: %v", err)
			}
			text, err := c.Text()
			if err != nil || text != tc.text {
				t.Errorf("Text() = %q, %v; want %q", text, err, tc.text)
			}
			if c.Route() != tc.route {
				t.Errorf("Route() = %d, want %d", c.Route(), tc.route)
			}
			kind, arg, err := c.Parts()
			if err != nil || kind != tc.kind || arg != tc.arg {
				t.Errorf("Parts() = %d, %q, %v; want %d, %q", kind, arg, err, tc.kind, tc.arg)
			}
		})
	}
}

// TestParseCommandKind checks that each kind's name reads back as the kind
// and that no other text, a near miss included, reads as one.
func TestParseCommandKind(t *testing.T) {
	for _, kind := range []paneinput.CommandKind{paneinput.CommandMkreq, paneinput.CommandMkarch, paneinput.CommandMkplan, paneinput.CommandRunplan, paneinput.CommandRunpr} {
		got, err := paneinput.ParseCommandKind(kind.Name())
		if err != nil || got != kind {
			t.Errorf("ParseCommandKind(%q) = %d, %v; want %d", kind.Name(), got, err, kind)
		}
	}
	for _, name := range []string{"", "runplan-takeover", "Runplan", "/runplan", "runplan ", "fixpr", "clear"} {
		if got, err := paneinput.ParseCommandKind(name); !errors.Is(err, paneinput.ErrInvalid) || got != paneinput.CommandUnknown {
			t.Errorf("ParseCommandKind(%q) = %d, %v; want CommandUnknown and ErrInvalid", name, got, err)
		}
	}
}

// TestConstructorsRejectInvalid checks that each constructor rejects a zero
// argument and a route that does not belong to its command.
func TestConstructorsRejectInvalid(t *testing.T) {
	task := task0088(t)
	cases := map[string]func() error{
		"mkreq without issue": func() error { _, err := paneinput.NewMkreq(paneinput.IssueNumber{}); return err },
		"mkarch without task": func() error { _, err := paneinput.NewMkarch(paneinput.TaskID{}); return err },
		"mkplan without task": func() error { _, err := paneinput.NewMkplan(paneinput.TaskID{}); return err },
		"runplan without task": func() error {
			_, err := paneinput.NewRunplan(paneinput.TaskID{}, paneinput.RouteRunplanOpus)
			return err
		},
		"runplan on docs route":   func() error { _, err := paneinput.NewRunplan(task, paneinput.RouteTaskDocuments); return err },
		"runplan on runpr route":  func() error { _, err := paneinput.NewRunplan(task, paneinput.RouteRunpr); return err },
		"runplan on zero route":   func() error { _, err := paneinput.NewRunplan(task, paneinput.RouteUnknown); return err },
		"runplan on out of range": func() error { _, err := paneinput.NewRunplan(task, paneinput.RouteKey(99)); return err },
		"runpr without PR":        func() error { _, err := paneinput.NewRunpr(fixprguard.PRNumber{}); return err },
		"Claude route zero model": func() error { _, err := paneinput.NewClaudeRoute(paneinput.ClaudeModelUnknown); return err },
		"Claude route out of range": func() error {
			_, err := paneinput.NewClaudeRoute(paneinput.ClaudeModel(7))
			return err
		},
		"OpenCode route zero model": func() error { _, err := paneinput.NewOpenCodeRoute(paneinput.OpenCodeModel{}); return err },
		"model switch zero":         func() error { _, err := paneinput.SwitchModel(paneinput.ClaudeModelUnknown); return err },
		"model switch out of range": func() error { _, err := paneinput.SwitchModel(paneinput.ClaudeModel(-1)); return err },
		"command prompt of zero":    func() error { _, err := paneinput.TypeCommand(paneinput.Command{}); return err },
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, paneinput.ErrInvalid) {
				t.Errorf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

// TestZeroValuesAreUnusable checks that the zero values, which no
// constructor makes, yield no text to type. Enumeration values outside their
// range are rejected by the constructors (TestConstructorsRejectInvalid).
func TestZeroValuesAreUnusable(t *testing.T) {
	if _, err := (paneinput.Command{}).Text(); !errors.Is(err, paneinput.ErrInvalid) {
		t.Errorf("zero Command Text error = %v, want ErrInvalid", err)
	}
	if kind, arg, err := (paneinput.Command{}).Parts(); !errors.Is(err, paneinput.ErrInvalid) || kind != paneinput.CommandUnknown || arg != "" {
		t.Errorf("zero Command Parts = %d, %q, %v; want CommandUnknown, \"\", and ErrInvalid", kind, arg, err)
	}
	if _, err := (paneinput.Prompt{}).Text(); !errors.Is(err, paneinput.ErrInvalid) {
		t.Errorf("zero Prompt Text error = %v, want ErrInvalid", err)
	}
	if _, err := (paneinput.Route{}).ModelText(); !errors.Is(err, paneinput.ErrInvalid) {
		t.Errorf("zero Route ModelText error = %v, want ErrInvalid", err)
	}
}

// TestPromptText checks the line each prompt types.
func TestPromptText(t *testing.T) {
	runplan, err := paneinput.NewRunplan(task0088(t), paneinput.RouteRunplanOpus)
	if err != nil {
		t.Fatal(err)
	}
	command, err := paneinput.TypeCommand(runplan)
	if err != nil {
		t.Fatal(err)
	}
	opus, err := paneinput.SwitchModel(paneinput.ClaudeModelOpus)
	if err != nil {
		t.Fatal(err)
	}
	sonnet, err := paneinput.SwitchModel(paneinput.ClaudeModelSonnet)
	if err != nil {
		t.Fatal(err)
	}
	for want, p := range map[string]paneinput.Prompt{
		"/clear":        paneinput.ClearSession(),
		"/new":          paneinput.NewSession(),
		"/model opus":   opus,
		"/model sonnet": sonnet,
		"/runplan 0088": command,
	} {
		if got, err := p.Text(); err != nil || got != want {
			t.Errorf("Text() = %q, %v; want %q", got, err, want)
		}
	}
}

// TestRouteModelText checks the display form of each kind of route and that
// ParseClaudeModel accepts only the two arguments.
func TestRouteModelText(t *testing.T) {
	sonnet, err := paneinput.ParseClaudeModel("sonnet")
	if err != nil {
		t.Fatal(err)
	}
	claude, err := paneinput.NewClaudeRoute(sonnet)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := claude.ModelText(); err != nil || got != "sonnet" || claude.Agent() != paneinput.AgentClaudeCode {
		t.Errorf("Claude route = %d %q, %v", claude.Agent(), got, err)
	}
	m, err := paneinput.ParseOpenCodeModel("DeepSeek V4.1 Flash 1017 OpenCode Go")
	if err != nil {
		t.Fatal(err)
	}
	open, err := paneinput.NewOpenCodeRoute(m)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := open.ModelText(); err != nil || got != m.String() || open.Agent() != paneinput.AgentOpenCode {
		t.Errorf("OpenCode route = %d %q, %v", open.Agent(), got, err)
	}
	for _, s := range []string{"", "Opus", "haiku", "opus ", "claude-opus-5-5"} {
		if _, err := paneinput.ParseClaudeModel(s); !errors.Is(err, paneinput.ErrInvalid) {
			t.Errorf("ParseClaudeModel(%q) error = %v, want ErrInvalid", s, err)
		}
	}
}

// TestRelayKeysFromNumbersOnly checks that a relay key is made only from an
// agent whose keys were checked and an option number of one digit, and that
// its key is that number.
func TestRelayKeysFromNumbersOnly(t *testing.T) {
	for _, agent := range []paneinput.AgentKind{paneinput.AgentClaudeCode, paneinput.AgentOpenCode} {
		for n := 1; n <= paneinput.MaxOptionNumber; n++ {
			k, err := paneinput.SelectOption(agent, n)
			if err != nil {
				t.Fatalf("SelectOption(%d, %d): %v", agent, n, err)
			}
			if key, err := k.Key(); err != nil || key != string(rune('0'+n)) {
				t.Errorf("SelectOption(%d, %d).Key() = %q, %v", agent, n, key, err)
			}
		}
	}
	for _, c := range []struct {
		agent  paneinput.AgentKind
		number int
	}{
		{paneinput.AgentClaudeCode, 0},
		{paneinput.AgentClaudeCode, -1},
		{paneinput.AgentOpenCode, paneinput.MaxOptionNumber + 1},
		{paneinput.AgentUnknown, 1},
		{paneinput.AgentKind(9), 1},
	} {
		if _, err := paneinput.SelectOption(c.agent, c.number); !errors.Is(err, paneinput.ErrInvalid) {
			t.Errorf("SelectOption(%d, %d) error = %v, want ErrInvalid", c.agent, c.number, err)
		}
	}
	if _, err := (paneinput.RelayKey{}).Key(); !errors.Is(err, paneinput.ErrInvalid) {
		t.Errorf("zero RelayKey Key error = %v, want ErrInvalid", err)
	}
}
