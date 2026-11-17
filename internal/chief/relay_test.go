//go:build test && unix

package chief

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// checkOf returns the check value of the question pane p shows now, whether
// chief relays it or not, as the tool computes it.
func (sw *sendWorld) checkOf(p *simPane) CheckValue {
	sw.t.Helper()
	sw.mu.Lock()
	defer sw.mu.Unlock()
	a, err := parseAgentGet(agentGetOutput(p.fakeAgent))
	if err != nil {
		sw.t.Fatal(err)
	}
	screen := readScreen(a.Kind, p.screen)
	if screen.Question == nil {
		sw.t.Fatalf("pane %s shows no question", p.Pane)
	}
	return checkValue(a, *screen.Question)
}

// relay runs Relay on workspace w1 with check and answer and returns its one
// result.
func (sw *sendWorld) relay(deps Deps, check CheckValue, answer string) ActionResult {
	sw.t.Helper()
	req, err := ParseRelayRequest(sw.trees["w1"], string(check), answer)
	if err != nil {
		sw.t.Fatal(err)
	}
	out, err := Relay(context.Background(), deps, req)
	if err != nil {
		sw.t.Fatal(err)
	}
	if len(out.Actions) != 1 {
		sw.t.Fatalf("Relay returned %d results, want 1", len(out.Actions))
	}
	return out.Actions[0]
}

// questionPane adds workspace w1 with one pane of kind blocked on the
// question of sample.
func (sw *sendWorld) questionPane(kind, sample string) *simPane {
	sw.t.Helper()
	sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: detachedHead}, kind)
	p := sw.firstPane()
	sw.ask(p, sample)
	return p
}

// TestCheckValueChanges checks that the check value changes with each part
// it is made of, and not with the agent's status, which a snapshot reads
// from the agent list and a relay from the agent itself.
func TestCheckValueChanges(t *testing.T) {
	base := Agent{Pane: PaneID{id: "w1:p1"}, Workspace: "w1", Kind: paneinput.AgentClaudeCode, Status: AgentBlocked, Session: "00000000-0000-4000-8000-000000000001", Dir: "/w", StateChangeSeq: 10}
	question := func() Question {
		q := parseClaudeScreen(readScreenSample(t, "claude-question-single.ansi")).Question
		if q == nil {
			t.Fatal("the sample shows no question")
		}
		return *q
	}
	want := checkValue(base, question())
	if got := checkValue(base, question()); got != want {
		t.Fatalf("the same pane and question gave %s and %s", want, got)
	}
	if !checkValueForm.MatchString(string(want)) {
		t.Errorf("check value %q does not have the form the parser takes", want)
	}
	cases := map[string]func(a *Agent, q *Question){
		"agent kind":            func(a *Agent, _ *Question) { a.Kind = paneinput.AgentOpenCode },
		"pane":                  func(a *Agent, _ *Question) { a.Pane = PaneID{id: "w1:p2"} },
		"agent session":         func(a *Agent, _ *Question) { a.Session = "00000000-0000-4000-8000-000000000002" },
		"state_change_seq":      func(a *Agent, _ *Question) { a.StateChangeSeq++ },
		"question text":         func(_ *Agent, q *Question) { q.Text += "?" },
		"option label":          func(_ *Agent, q *Question) { q.Options[0].Label = "Apricot" },
		"option description":    func(_ *Agent, q *Question) { q.Options[0].Description = "" },
		"option number":         func(_ *Agent, q *Question) { q.Options[0].Number = 7 },
		"option relayability":   func(_ *Agent, q *Question) { q.Options[0].Relayable = false },
		"one option fewer":      func(_ *Agent, q *Question) { q.Options = q.Options[:len(q.Options)-1] },
		"selection position":    func(_ *Agent, q *Question) { q.Cursor = 3 },
		"text moved to a label": func(_ *Agent, q *Question) { q.Options[0].Label = q.Text + q.Options[0].Label; q.Text = "" },
	}
	idle := base
	idle.Status = AgentIdle
	if got := checkValue(idle, question()); got != want {
		t.Errorf("changing the status changed the value")
	}
	seen := map[CheckValue]string{want: "base"}
	for name, change := range cases {
		a, q := base, question()
		change(&a, &q)
		got := checkValue(a, q)
		if prev, dup := seen[got]; dup {
			t.Errorf("changing the %s gives the value of %s", name, prev)
		}
		seen[got] = name
	}
	// The real screens: moving the selection changes the value.
	for _, agent := range []struct {
		kind  paneinput.AgentKind
		agent string
	}{{paneinput.AgentClaudeCode, "claude"}, {paneinput.AgentOpenCode, "opencode"}} {
		a := base
		a.Kind = agent.kind
		at1 := readScreen(agent.kind, readScreenSample(t, agent.agent+"-question-single.ansi")).Question
		at3 := readScreen(agent.kind, readScreenSample(t, agent.agent+"-question-single-cursor.ansi")).Question
		if at1 == nil || at3 == nil || checkValue(a, *at1) == checkValue(a, *at3) {
			t.Errorf("%s: moving the selection kept the check value", agent.agent)
		}
	}
}

// TestNoCheckValueForUnrelayable checks that a snapshot gives a check value
// to each single-select question of a blocked agent and to none of the
// questions chief does not relay.
func TestNoCheckValueForUnrelayable(t *testing.T) {
	samples := map[string]bool{
		"claude-question-single.ansi":          true,
		"claude-merge-question.ansi":           true,
		"opencode-question-single.ansi":        true,
		"opencode-merge-question.ansi":         true,
		"claude-question-multi.ansi":           false,
		"claude-question-tabs.ansi":            false,
		"claude-question-tabs-second.ansi":     false,
		"claude-question-tabs-review.ansi":     false,
		"claude-permission.ansi":               false,
		"claude-permission-warning.ansi":       false,
		"claude-permission-wrapped.ansi":       false,
		"opencode-question-multi.ansi":         false,
		"opencode-question-tabs.ansi":          false,
		"opencode-question-tabs-second.ansi":   false,
		"opencode-question-tabs-review.ansi":   false,
		"opencode-question-single-cursor.ansi": true,
	}
	w := newWorld(t)
	i := 0
	for name := range samples {
		i++
		kind, _, _ := strings.Cut(name, "-")
		w.add(fakeWorkspace{ID: "t" + string(rune('a'+i)), Label: name, Kind: kind, Branch: detachedHead, Status: "blocked", Screen: name})
	}
	// An OpenCode question Herdr does not report blocked still makes the
	// row a question, but a relay would refuse it, so it gets no value.
	w.add(fakeWorkspace{ID: "tz", Label: "opencode idle", Kind: "opencode", Branch: detachedHead, Status: "idle", Screen: "opencode-question-single.ansi"})
	samples["opencode idle"] = false
	snap := take(t, w)
	out := snap.Output()
	for name, want := range samples {
		r := rowOf(t, snap, name)
		if len(r.Questions) != 1 {
			t.Errorf("%s: %d questions, want 1", name, len(r.Questions))
			continue
		}
		if got := r.Questions[0].Check != ""; got != want {
			t.Errorf("%s: check value present %v, want %v", name, got, want)
		}
		i := slices.IndexFunc(out.Rows, func(row RowOutput) bool { return row.Label == name })
		if i < 0 {
			t.Errorf("%s: no output row", name)
			continue
		}
		if q := out.Rows[i].Questions[0]; q.Relayable != want || (q.Check != "") != want {
			t.Errorf("%s: output relayable %v check %q, want relayable %v", name, q.Relayable, q.Check, want)
		}
	}
}

// TestRelaySendsChosenOption checks that a relay presses the key of the
// option number the human chose, not the option under the selection, and
// reports what it sent.
func TestRelaySendsChosenOption(t *testing.T) {
	for kind, agent := range map[string]paneinput.AgentKind{"claude": paneinput.AgentClaudeCode, "opencode": paneinput.AgentOpenCode} {
		t.Run(kind, func(t *testing.T) {
			sw := newSendWorld(t)
			p := sw.questionPane(kind, kind+"-question-single-cursor.ansi")
			deps := sw.finish()
			res := sw.relay(deps, sw.checkOf(p), "1=1")
			if res.Outcome != OutcomeSent || res.Input != InputRelay || res.Workspace != sw.trees["w1"] || res.Pane != "w1:p1" || res.Command != "1. Apple" || res.Agent != agent {
				t.Errorf("result = %+v, want option 1 relayed to w1:p1", res)
			}
			if got := sw.rec.keys(); !slices.Equal(got, []string{"w1:p1 1"}) {
				t.Errorf("keys = %v, want the key of option 1 to w1:p1", got)
			}
			if got := sw.rec.prompts(); len(got) != 0 {
				t.Errorf("typed %v", got)
			}
			if a := actionOutput(res); a.Input != "relay" || a.Outcome != "sent" || a.Command != "1. Apple" || a.Detail != relaySentDetail {
				t.Errorf("output = %+v, want a detail that claims only the key and the question left", a)
			}
			records, err := deps.Store.Records(Worktree{path: sw.trees["w1"]})
			if err != nil {
				t.Fatal(err)
			}
			starts, ends, err := records.sendFiles()
			if err != nil || len(starts) != 1 || len(ends) != 1 {
				t.Fatalf("send records: %d starts, %d ends, %v; want one send", len(starts), len(ends), err)
			}
			var rec SendRecord
			if err := records.store().read(sendPrefix+starts[0]+startSuffix, &rec); err != nil || rec.Input != "relay" || !slices.Equal(rec.Prompts, []string{"key 1"}) || rec.Kind != "" || rec.Arg != "" {
				t.Errorf("send record = %+v, %v; want the relay's key and no command kind", rec, err)
			}
			var end sendEnd
			if err := records.store().read(sendPrefix+starts[0]+endSuffix, &end); err != nil || end.Outcome != "sent" || end.Reason != relaySentDetail {
				t.Errorf("end record = %+v, %v; want sent with the relay's detail", end, err)
			}
			if stop, err := records.Stopped(); err != nil || stop != nil {
				t.Errorf("stop = %+v, %v; want none", stop, err)
			}
		})
	}
}

// TestRelayRereadsTheScreenBeforeTheKey checks that a relay reads the screen
// again just before the key: a question redrawn after the match while
// Herdr's agent reading stays the same is not given the key.
func TestRelayRereadsTheScreenBeforeTheKey(t *testing.T) {
	for _, kind := range []string{"claude", "opencode"} {
		t.Run(kind, func(t *testing.T) {
			sw := newSendWorld(t)
			p := sw.questionPane(kind, kind+"-question-single.ansi")
			deps := sw.finish()
			check := sw.checkOf(p)
			other := readScreenSample(t, kind+"-merge-question.ansi")
			reads := 0
			sw.rec.replyFunc("herdr agent read w1:p1 --source visible --lines 200 --format ansi", func() reply {
				sw.mu.Lock()
				defer sw.mu.Unlock()
				reads++
				if reads == 2 {
					// Another question is drawn after the match; the agent
					// reading does not change.
					p.screen = other
				}
				return reply{out: []byte(p.screen)}
			})
			res := sw.relay(deps, check, "1=1")
			if res.Outcome != OutcomeNotSent || res.Reason != NotSentCheckMismatch || reads != 2 {
				t.Errorf("result = %+v after %d screen reads, want not sent, check mismatch, after the second", res, reads)
			}
			if got := sw.rec.keys(); len(got) != 0 {
				t.Errorf("keys = %v", got)
			}
		})
	}
}

// TestRelayCheckValueMismatch checks that a relay sends nothing when the pane
// no longer gives the check value the human was shown.
func TestRelayCheckValueMismatch(t *testing.T) {
	cases := map[string]func(sw *sendWorld, p *simPane){
		"the question was answered": func(sw *sendWorld, p *simPane) {
			p.Status, p.Seq, p.screen = "idle", p.Seq+1, readScreenSample(sw.t, "claude-idle.ansi")
		},
		// Only the screen changes, so the question's text and options alone
		// must make the value differ.
		"another question is asked": func(sw *sendWorld, p *simPane) {
			p.screen = readScreenSample(sw.t, "claude-merge-question.ansi")
		},
		"the same question is asked again": func(_ *sendWorld, p *simPane) { p.Seq++ },
		"the session changed":              func(sw *sendWorld, p *simPane) { p.Session = sw.newSession() },
		"the selection moved": func(sw *sendWorld, p *simPane) {
			p.screen = readScreenSample(sw.t, "claude-question-single-cursor.ansi")
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			sw := newSendWorld(t)
			p := sw.questionPane("claude", "claude-question-single.ansi")
			deps := sw.finish()
			check := sw.checkOf(p)
			sw.mu.Lock()
			change(sw, p)
			sw.mu.Unlock()
			res := sw.relay(deps, check, "1=1")
			if res.Outcome != OutcomeNotSent || res.Reason != NotSentCheckMismatch {
				t.Errorf("result = %+v, want not sent, check mismatch", res)
			}
			if got := sw.rec.keys(); len(got) != 0 {
				t.Errorf("keys = %v", got)
			}
		})
	}
	t.Run("the same question is asked again as its screen is read", func(t *testing.T) {
		sw := newSendWorld(t)
		p := sw.questionPane("claude", "claude-question-single.ansi")
		deps := sw.finish()
		check := sw.checkOf(p)
		// The question is answered and the same one asked again as the
		// screen is read: the screen shows the same question, and only an
		// agent read after it carries the new state_change_seq.
		sw.rec.replyFunc("herdr agent read w1:p1 --source visible --lines 200 --format ansi", func() reply {
			sw.mu.Lock()
			defer sw.mu.Unlock()
			p.Seq += 2
			return reply{out: []byte(p.screen)}
		})
		res := sw.relay(deps, check, "1=1")
		if res.Outcome != OutcomeNotSent || res.Reason != NotSentCheckMismatch {
			t.Errorf("result = %+v, want not sent, check mismatch", res)
		}
		if got := sw.rec.keys(); len(got) != 0 {
			t.Errorf("keys = %v", got)
		}
	})
}

// TestRelayRereadsBeforeTheKey checks that a pane whose agent changes after
// its question was matched, and before the key, gets no key.
func TestRelayRereadsBeforeTheKey(t *testing.T) {
	sw := newSendWorld(t)
	p := sw.questionPane("claude", "claude-question-single.ansi")
	deps := sw.finish()
	check := sw.checkOf(p)
	gets := 0
	sw.rec.replyFunc("herdr agent get w1:p1", func() reply {
		sw.mu.Lock()
		defer sw.mu.Unlock()
		gets++
		if gets == 2 {
			// The question is answered and asked again after the match.
			p.Seq += 2
		}
		return reply{out: agentGetOutput(p.fakeAgent)}
	})
	res := sw.relay(deps, check, "1=1")
	if res.Outcome != OutcomeNotSent || res.Reason != NotSentCheckMismatch || gets != 2 {
		t.Errorf("result = %+v after %d agent reads, want not sent, check mismatch, after the second", res, gets)
	}
	if got := sw.rec.keys(); len(got) != 0 {
		t.Errorf("keys = %v", got)
	}
}

// TestRelayRefusesTabbedQuestion checks that a relay sends nothing to a
// tabbed question even with the check value of the question as it is, and
// likewise to the other shapes chief does not relay.
func TestRelayRefusesTabbedQuestion(t *testing.T) {
	for _, sample := range []string{
		"claude-question-tabs.ansi", "claude-question-tabs-second.ansi", "claude-question-tabs-review.ansi",
		"opencode-question-tabs.ansi", "opencode-question-tabs-second.ansi", "opencode-question-tabs-review.ansi",
		"claude-question-multi.ansi", "opencode-question-multi.ansi", "claude-permission.ansi", "claude-permission-warning.ansi", "claude-permission-wrapped.ansi",
	} {
		t.Run(sample, func(t *testing.T) {
			sw := newSendWorld(t)
			kind, _, _ := strings.Cut(sample, "-")
			p := sw.questionPane(kind, sample)
			deps := sw.finish()
			res := sw.relay(deps, sw.checkOf(p), "1=1")
			if res.Outcome != OutcomeNotSent || res.Reason != NotSentNotRelayable {
				t.Errorf("result = %+v, want not sent, not relayable", res)
			}
			if got := sw.rec.keys(); len(got) != 0 {
				t.Errorf("keys = %v", got)
			}
		})
	}
}

// TestRelayRefusesOption checks that a relay sends nothing for an option the
// question does not have or does not relay (its free-text and chat options).
func TestRelayRefusesOption(t *testing.T) {
	cases := []struct {
		kind, answer string
	}{
		{"claude", "1=4"}, // Type something.
		{"claude", "1=5"}, // Chat about this
		{"claude", "1=6"},
		{"opencode", "1=4"}, // Type your own answer
		{"opencode", "1=5"},
	}
	for _, c := range cases {
		t.Run(c.kind+" "+c.answer, func(t *testing.T) {
			sw := newSendWorld(t)
			p := sw.questionPane(c.kind, c.kind+"-question-single.ansi")
			deps := sw.finish()
			res := sw.relay(deps, sw.checkOf(p), c.answer)
			if res.Outcome != OutcomeNotSent || res.Reason != NotSentBadOption {
				t.Errorf("result = %+v, want not sent, bad option", res)
			}
			if got := sw.rec.keys(); len(got) != 0 {
				t.Errorf("keys = %v", got)
			}
		})
	}
}

// TestRelayNeedsAnAgentOnTheQuestion checks that a relay sends nothing to an
// agent Herdr does not report blocked, whatever its screen shows.
func TestRelayNeedsAnAgentOnTheQuestion(t *testing.T) {
	for _, kind := range []string{"claude", "opencode"} {
		for _, status := range []string{"idle", "working"} {
			t.Run(kind+" "+status, func(t *testing.T) {
				sw := newSendWorld(t)
				p := sw.questionPane(kind, kind+"-question-single.ansi")
				p.Status = status
				deps := sw.finish()
				res := sw.relay(deps, sw.checkOf(p), "1=1")
				if res.Outcome != OutcomeNotSent || res.Reason != NotSentNotAccepting {
					t.Errorf("result = %+v, want not sent, not accepting", res)
				}
				if got := sw.rec.keys(); len(got) != 0 {
					t.Errorf("keys = %v", got)
				}
			})
		}
	}
}

// TestRelayTakesTheSnapshotsValue checks the whole path: the check value a
// snapshot prints relays the chosen option to the pane that shows the
// question, among the workspace's panes, and the running row then still
// shows the command chief sent before, not the relay.
func TestRelayTakesTheSnapshotsValue(t *testing.T) {
	for _, c := range []struct{ asking, other, sample string }{
		{"opencode", "claude", "opencode-question-single.ansi"},
		{"claude", "opencode", "claude-merge-question.ansi"},
	} {
		t.Run(c.asking, func(t *testing.T) {
			sw := newSendWorld(t)
			tree := sw.addTask(fakeWorkspace{ID: "w1", Label: "ws", Branch: detachedHead}, c.other, c.asking)
			p := sw.byPane["w1:p2"]
			sw.ask(p, c.sample)
			deps := sw.finish()
			records, err := deps.Store.Records(Worktree{path: tree})
			if err != nil {
				t.Fatal(err)
			}
			held, err := records.Lock()
			if err != nil {
				t.Fatal(err)
			}
			id, err := held.BeginSend(SendRecord{Pane: "w1:p2", Input: InputCommand.String(), Command: "/runpr 42", Model: "m"}, sw.now)
			if err == nil {
				err = held.EndSend(id, OutcomeSent.String(), "", sw.now)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := held.Close(); err != nil {
				t.Fatal(err)
			}
			snap, err := TakeSnapshot(context.Background(), deps, sw.owner())
			if err != nil {
				t.Fatal(err)
			}
			out := snap.Output()
			if len(out.Rows) != 1 || len(out.Rows[0].Questions) != 1 || out.Rows[0].Questions[0].Pane != "w1:p2" {
				t.Fatalf("rows = %+v, want one question in w1:p2", out.Rows)
			}
			res := sw.relay(deps, out.Rows[0].Questions[0].Check, "1=2")
			if res.Outcome != OutcomeSent || res.Pane != "w1:p2" || res.Workspace != tree {
				t.Fatalf("result = %+v, want option 2 sent to w1:p2 of %s", res, tree)
			}
			if got := sw.rec.keys(); !slices.Equal(got, []string{"w1:p2 2"}) {
				t.Errorf("keys = %v", got)
			}
			snap, err = TakeSnapshot(context.Background(), deps, sw.owner())
			if err != nil {
				t.Fatal(err)
			}
			if r := snap.Rows[0]; r.State != StateRunning || r.Running == nil || r.Running.Command != "/runpr 42" {
				t.Errorf("row after the relay = %+v, running %+v; want the command sent before", r, r.Running)
			}
		})
	}
}

// TestRelayUnreadablePane checks that a pane that cannot be read is reported
// as unreadable, not as a changed question.
func TestRelayUnreadablePane(t *testing.T) {
	sw := newSendWorld(t)
	p := sw.questionPane("claude", "claude-question-single.ansi")
	deps := sw.finish()
	check := sw.checkOf(p)
	sw.rec.replyFunc("herdr agent read w1:p1 --source visible --lines 200 --format ansi", func() reply {
		return reply{err: errors.New("herdr failed")}
	})
	res := sw.relay(deps, check, "1=1")
	if res.Outcome != OutcomeNotSent || res.Reason != NotSentUnreadable {
		t.Errorf("result = %+v, want not sent, unreadable", res)
	}
	if got := sw.rec.keys(); len(got) != 0 {
		t.Errorf("keys = %v", got)
	}
}

// TestParseRelayRequest checks the argument grammar of chief relay.
func TestParseRelayRequest(t *testing.T) {
	check := strings.Repeat("a", 64)
	for _, answer := range []string{"1=1", "1=9"} {
		if _, err := ParseRelayRequest("/w", check, answer); err != nil {
			t.Errorf("ParseRelayRequest(%q) = %v", answer, err)
		}
	}
	bad := []struct{ ws, check, answer string }{
		{"w", check, "1=1"},
		{"/w", check[:63], "1=1"},
		{"/w", strings.ToUpper(check), "1=1"},
		{"/w", check + "a", "1=1"},
		{"/w", check, "1=10"},
		{"/w", check, "1=a"},
		{"/w", check, "1="},
		{"/w", check, "=1"},
		{"/w", check, "1"},
		{"/w", check, "1=0"},
		{"/w", check, "2=1"},
		{"/w", check, "01=1"},
	}
	for _, b := range bad {
		if _, err := ParseRelayRequest(b.ws, b.check, b.answer); !errors.Is(err, ErrBadRequest) {
			t.Errorf("ParseRelayRequest(%q, %q, %q) = %v, want ErrBadRequest", b.ws, b.check, b.answer, err)
		}
	}
}

// TestRelayFailureAfterKeyStops checks that a failure once the key may have
// been pressed is not retried: the workspace stops, and a later relay sends
// nothing until release.
func TestRelayFailureAfterKeyStops(t *testing.T) {
	cases := map[string]struct {
		react  func(p *simPane, key string) error
		reason NotSentReason
	}{
		"herdr fails":                     {func(*simPane, string) error { return errors.New("herdr failed") }, NotSentUnreadable},
		"the agent stays on the question": {func(*simPane, string) error { return nil }, NotSentNoStep},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			sw := newSendWorld(t)
			p := sw.questionPane("claude", "claude-question-single.ansi")
			deps := sw.finish()
			sw.keyReact = c.react
			res := sw.relay(deps, sw.checkOf(p), "1=2")
			if res.Outcome != OutcomeSentThenFailed || res.Reason != c.reason || res.Step != StepKey {
				t.Fatalf("result = %+v, want sent then failed after the key, %v", res, c.reason)
			}
			if got := sw.rec.keys(); !slices.Equal(got, []string{"w1:p1 2"}) {
				t.Errorf("keys = %v, want the key pressed once and not retried", got)
			}
			sw.keyReact = sw.realKeyReact
			pressed := len(sw.rec.keys())
			res = sw.relay(deps, sw.checkOf(p), "1=2")
			if res.Outcome != OutcomeNotSent || res.Reason != NotSentStopped {
				t.Errorf("second relay = %+v, want not sent, stopped", res)
			}
			if got := sw.rec.keys(); len(got) != pressed {
				t.Errorf("the second relay pressed %v", got[pressed:])
			}
		})
	}
}
