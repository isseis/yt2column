//go:build test

package chief

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// simPane is one agent pane of the simulated Herdr: its agent as Herdr
// reports it and the screen it shows now.
type simPane struct {
	fakeAgent
	screen string
}

// sendWorld is a world whose task panes change when chief types into them,
// as the real agents did while sampling: /clear gives a Claude Code pane a
// new session, /model writes its record to the new session's transcript,
// /new shows OpenCode's new-session screen, and a command makes the agent
// work. A test changes react to make one of these go wrong.
type sendWorld struct {
	*world
	mu       sync.Mutex
	panes    []*simPane
	byPane   map[string]*simPane
	config   string
	sessions int
	// react is what typing text into p does.
	react func(p *simPane, text string) error
}

// sessionPrefix makes the session IDs of the simulated Claude Code panes.
const sessionPrefix = "00000000-0000-4000-8000-1000000000"

func newSendWorld(t *testing.T) *sendWorld {
	t.Helper()
	w := newWorld(t)
	sw := &sendWorld{world: w, byPane: map[string]*simPane{}, config: filepath.Join(filepath.Dir(w.chiefTree), "claude")}
	w.env.ClaudeConfigDir = sw.config
	sw.react = sw.realReact
	return sw
}

// addTask adds a task workspace with one pane of each kind given ("claude",
// "opencode"), idle on its prompt, and returns its worktree.
func (sw *sendWorld) addTask(ws fakeWorkspace, kinds ...string) string {
	sw.t.Helper()
	tree := sw.addTree(ws)
	for i, kind := range kinds {
		p := &simPane{fakeAgent: fakeAgent{Pane: fmt.Sprintf("%s:p%d", ws.ID, i+1), Workspace: ws.ID, Kind: kind, Status: "idle", Dir: tree, Seq: 10}}
		switch kind {
		case "claude":
			p.Session = sw.newSession()
			p.screen = readScreenSample(sw.t, "claude-idle.ansi")
		default:
			p.screen = readScreenSample(sw.t, "opencode-done.ansi")
		}
		sw.panes = append(sw.panes, p)
		sw.byPane[p.Pane] = p
		sw.agents = append(sw.agents, p.fakeAgent)
	}
	return tree
}

// newSession returns a new session ID.
func (sw *sendWorld) newSession() string {
	sw.sessions++
	id := fmt.Sprintf("%s%02d", sessionPrefix, sw.sessions)
	return id
}

// transcript returns the path of a pane's transcript for session.
func (sw *sendWorld) transcript(p *simPane, session string) string {
	return filepath.Join(sw.config, "projects", strings.NewReplacer("/", "-", ".", "-").Replace(p.Dir), session+".jsonl")
}

// writeTranscript appends lines to the transcript of a pane's session.
func (sw *sendWorld) writeTranscript(p *simPane, session string, lines ...string) error {
	path := sw.transcript(p, session)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			return err
		}
	}
	return nil
}

// modeLine is the first line of a new transcript.
func modeLine(session string) string {
	return fmt.Sprintf(`{"type":"mode","mode":"normal","sessionId":%q}`, session)
}

// switchLine is the transcript record of /model to the display name shown.
func switchLine(shown string) string {
	return fmt.Sprintf(`{"type":"user","message":{"role":"user","content":"<local-command-stdout>Set model to `+"`%s`"+` and saved as your default for new sessions</local-command-stdout>"}}`, shown)
}

// modelNames are the display names /model reported in the samples.
var modelNames = map[string]string{"opus": "Opus 5.5", "sonnet": "Sonnet 5.5"}

// realReact is what the sampled agents did with each input.
func (sw *sendWorld) realReact(p *simPane, text string) error {
	switch {
	case p.Kind == "claude" && text == "/clear":
		p.Session = sw.newSession()
		p.screen = readScreenSample(sw.t, "claude-after-clear.ansi")
		return sw.writeTranscript(p, p.Session, modeLine(p.Session))
	case p.Kind == "claude" && strings.HasPrefix(text, "/model "):
		p.screen = readScreenSample(sw.t, "claude-after-model.ansi")
		return sw.writeTranscript(p, p.Session, switchLine(modelNames[strings.TrimPrefix(text, "/model ")]))
	case p.Kind == "opencode" && text == "/new":
		p.screen = readScreenSample(sw.t, "opencode-after-new.ansi")
		return nil
	default:
		p.Status, p.Seq = "working", p.Seq+1
		if p.Kind == "claude" {
			p.screen = readScreenSample(sw.t, "claude-working.ansi")
		}
		return nil
	}
}

// firstPane returns the first pane of workspace w1.
func (sw *sendWorld) firstPane() *simPane {
	sw.t.Helper()
	p, ok := sw.byPane["w1:p1"]
	if !ok {
		sw.t.Fatal("no pane w1:p1")
	}
	return p
}

// finish sets the replies: the Herdr reads come from the panes as they are
// when read, a prompt changes its pane through react, and the transcripts of
// the panes' first sessions exist.
func (sw *sendWorld) finish() Deps {
	sw.t.Helper()
	deps := sw.world.finish()
	for _, p := range sw.panes {
		if p.Kind == "claude" {
			if err := sw.writeTranscript(p, p.Session, modeLine(p.Session)); err != nil {
				sw.t.Fatal(err)
			}
		}
		for _, src := range []string{"visible", "recent"} {
			sw.rec.replyFunc("herdr agent read "+p.Pane+" --source "+src+" --lines 200 --format ansi", func() reply {
				sw.mu.Lock()
				defer sw.mu.Unlock()
				return reply{out: []byte(p.screen)}
			})
		}
		sw.rec.replyFunc("herdr agent get "+p.Pane, func() reply {
			sw.mu.Lock()
			defer sw.mu.Unlock()
			return reply{out: agentGetOutput(p.fakeAgent)}
		})
	}
	sw.rec.replyFunc("herdr agent list", func() reply {
		sw.mu.Lock()
		defer sw.mu.Unlock()
		agents := []fakeAgent{sw.chief}
		for _, p := range sw.panes {
			agents = append(agents, p.fakeAgent)
		}
		return reply{out: agentListOutput(agents)}
	})
	sw.rec.onPrompt = func(pane, text string) error {
		sw.mu.Lock()
		defer sw.mu.Unlock()
		p, ok := sw.byPane[pane]
		if !ok {
			sw.t.Errorf("a prompt went to %s, which is no task pane", pane)
			return errNoReply
		}
		return sw.react(p, text)
	}
	deps.Sleep = func(context.Context, time.Duration) error { return nil }
	return deps
}
