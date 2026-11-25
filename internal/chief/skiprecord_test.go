//go:build test && unix

package chief

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
)

// skipAgent is a Claude Code pane as a skip record names it.
func skipAgent(t *testing.T) Agent {
	t.Helper()
	pane, err := ParsePaneID("w1:p1")
	if err != nil {
		t.Fatal(err)
	}
	return Agent{Pane: pane, Workspace: "w1", Kind: paneinput.AgentClaudeCode, Session: "00000000-0000-4000-8000-100000000001", StateChangeSeq: 12}
}

// mkarchRan is /mkarch 0001, the command that created a task-document PR.
var mkarchRan = ranCommand{kind: paneinput.CommandMkarch, argument: "0001", route: paneinput.RouteTaskDocuments}

// skipJSON returns the JSON of a valid skip record of skipAgent with
// edit applied to its members.
func skipJSON(t *testing.T, edit func(map[string]any)) []byte {
	t.Helper()
	rec, err := newSkipRecord("w1", skipAgent(t), sceneAfterRunpr, mkarchRan, time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(rec.file())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(m)
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestSkipRecordKept checks that a skip record is written and read back
// with its values, and that a second skip of the same pane, session, and
// state_change_seq is taken as kept already and leaves one record.
func TestSkipRecordKept(t *testing.T) {
	held := heldRecords(t, newStore(t), "/work/a")
	saved := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	rec, err := newSkipRecord("w1", skipAgent(t), sceneAfterRunpr, mkarchRan, saved)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := held.recordSkip(rec); err != nil {
			t.Fatalf("recordSkip = %v", err)
		}
	}
	found, err := held.skipRecords()
	if err != nil || len(found) != 1 || found[0].err != nil {
		t.Fatalf("skipRecords = %+v, %v; want one readable record", found, err)
	}
	if found[0].rec != rec {
		t.Errorf("read %+v, want %+v", found[0].rec, rec)
	}
}

// TestParseSkipRecord checks that a valid record is read, and that a record
// with an unknown, repeated, missing, or differently cased member, trailing
// data, or a value outside its accepted form is unreadable, never repaired.
func TestParseSkipRecord(t *testing.T) {
	if _, err := parseSkipRecord(skipJSON(t, nil)); err != nil {
		t.Fatalf("the valid record: %v", err)
	}
	set := func(key string, v any) func(map[string]any) {
		return func(m map[string]any) { m[key] = v }
	}
	valid := string(skipJSON(t, nil))
	raw := map[string]string{
		"a repeated member": strings.Replace(valid, `"pane":"w1:p1"`, `"pane":"w9:p9","pane":"w1:p1"`, 1),
		"trailing data":     valid + "{}",
		"not an object":     "[]",
	}
	for name, data := range raw {
		if data == valid {
			t.Fatalf("%s: the edit changed nothing", name)
		}
		if _, err := parseSkipRecord([]byte(data)); !errors.Is(err, errSkipRecord) {
			t.Errorf("%s: %v, want errSkipRecord", name, err)
		}
	}
	edits := map[string]func(map[string]any){
		"an unknown member":                 set("extra", "x"),
		"a missing member":                  func(m map[string]any) { delete(m, "route") },
		"a pane of another form":            set("pane", "-p1"),
		"a negative seq":                    set("seq", -1),
		"a seq as text":                     set("seq", "12"),
		"an agent chief does not know":      set("agent", "claude"),
		"no scene":                          set("scene", "unknown"),
		"a command of no kind":              set("command", "fixpr"),
		"a route of no row":                 set("route", "runplan"),
		"a saved time of another form":      set("saved", "2026-11-14"),
		"a route the command does not take": set("route", "runpr"),
		"a task number of another form":     set("argument", "1"),
		"/mkreq with an argument":           func(m map[string]any) { m["command"], m["argument"] = "mkreq", "12" },
		"/runpr of no PR number": func(m map[string]any) {
			m["scene"], m["command"], m["argument"], m["route"] = "after_autosend", "runpr", "0x1", "runpr"
		},
		"/runpr after /runpr": func(m map[string]any) { m["command"], m["argument"], m["route"] = "runpr", "42", "runpr" },
		// /mergemain creates no PR, so no reset is skipped after it, in
		// either scene.
		"/mergemain after /runpr": func(m map[string]any) { m["command"], m["argument"], m["route"] = "mergemain", "42", "mergemain" },
		"/mergemain after an automatic send": func(m map[string]any) {
			m["scene"], m["command"], m["argument"], m["route"] = "after_autosend", "mergemain", "42", "mergemain"
		},
		"/mkarch after an automatic send":    set("scene", "after_autosend"),
		"a Claude Code pane without session": set("session", ""),
		"an OpenCode pane with a session": func(m map[string]any) {
			m["agent"], m["scene"], m["command"], m["argument"], m["route"] = "opencode", "after_autosend", "runpr", "42", "runpr"
		},
		"an OpenCode pane after /runpr": func(m map[string]any) { m["agent"], m["session"] = "opencode", "" },
		"a workspace of another form":   set("workspace", "w-1"),
	}
	for name, edit := range edits {
		if _, err := parseSkipRecord(skipJSON(t, edit)); !errors.Is(err, errSkipRecord) {
			t.Errorf("%s: %v, want errSkipRecord", name, err)
		}
	}
	// The forms the rejections above take apart are accepted whole.
	accepted := map[string]func(map[string]any){
		"/mkreq without an argument":  func(m map[string]any) { m["command"], m["argument"] = "mkreq", "" },
		"/runplan by a runplan route": func(m map[string]any) { m["command"], m["route"] = "runplan", "runplan-takeover" },
		"/runpr after an automatic send": func(m map[string]any) {
			m["scene"], m["command"], m["argument"], m["route"] = "after_autosend", "runpr", "42", "runpr"
		},
		"an OpenCode pane": func(m map[string]any) {
			m["agent"], m["session"], m["scene"], m["command"], m["argument"], m["route"] = "opencode", "", "after_autosend", "runpr", "42", "runpr"
		},
	}
	for name, edit := range accepted {
		if _, err := parseSkipRecord(skipJSON(t, edit)); err != nil {
			t.Errorf("%s: %v, want it read", name, err)
		}
	}
}

// TestSkipRecordUnderAnotherName checks that a record whose file name is not
// its identity is unreadable: neither its use nor a send to its pane would
// remove it.
func TestSkipRecordUnderAnotherName(t *testing.T) {
	held := heldRecords(t, newStore(t), "/work/a")
	name := skipPrefix + strings.Repeat("0", 64) + recordSuffix
	if err := os.WriteFile(filepath.Join(held.dir, name), skipJSON(t, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := held.skipRecords()
	if err != nil || len(found) != 1 || !errors.Is(found[0].err, errSkipRecord) {
		t.Errorf("skipRecords = %+v, %v; want the record unreadable", found, err)
	}
}

// TestStartRemovesOldSkips checks that start removes a skip record saved
// more than detailMaxAge ago and an unreadable one last changed that long
// ago, and keeps a newer record and a newer unreadable one.
func TestStartRemovesOldSkips(t *testing.T) {
	now := time.Date(2026, 11, 30, 9, 0, 0, 0, time.UTC)
	store := newStore(t)
	held := heldRecords(t, store, "/work/a")
	write := func(seq int64, saved time.Time) string {
		a := skipAgent(t)
		a.StateChangeSeq = seq
		rec, err := newSkipRecord("w1", a, sceneAfterRunpr, mkarchRan, saved)
		if err != nil {
			t.Fatal(err)
		}
		if err := held.recordSkip(rec); err != nil {
			t.Fatal(err)
		}
		return rec.name()
	}
	oldName := write(1, now.Add(-detailMaxAge-time.Hour))
	newName := write(2, now.Add(-detailMaxAge+time.Hour))
	unreadable := func(name string, changed time.Time) {
		path := filepath.Join(held.dir, skipPrefix+strings.Repeat(name, 64)+recordSuffix)
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, changed, changed); err != nil {
			t.Fatal(err)
		}
	}
	unreadable("a", now.Add(-detailMaxAge-time.Hour))
	unreadable("b", now.Add(-detailMaxAge+time.Hour))
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Start(context.Background(), herdrWith(t, liveAgent(ownerA())), ownerA(), now); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(held.dir)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), skipPrefix) {
			left = append(left, e.Name())
		}
	}
	want := map[string]bool{newName: true, skipPrefix + strings.Repeat("b", 64) + recordSuffix: true}
	if len(left) != len(want) || !want[left[0]] || !want[left[1]] {
		t.Errorf("skip records left = %v; want the new record and the new unreadable one, not %s", left, oldName)
	}
}
