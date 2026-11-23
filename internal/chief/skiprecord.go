package chief

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/isseis/yt2column/internal/chief/paneinput"
	"github.com/isseis/yt2column/internal/fixprguard"
	"github.com/isseis/yt2column/internal/privatefile"
	"github.com/isseis/yt2column/internal/strictjson"
)

// skipPrefix starts a skip record's name.
const skipPrefix = "skip-"

// skipFileName is a skip record's name: the hex SHA-256 of its pane, session,
// and state_change_seq, the identity of the skipped reset.
var skipFileName = regexp.MustCompile(`^skip-[0-9a-f]{64}\.json$`)

// skipScene is where a reset was skipped. The zero value is no scene and is
// never read from a record.
type skipScene int

const (
	// sceneAfterRunpr is the reset of the pane of the command that created
	// the PR, after /runpr was sent.
	sceneAfterRunpr skipScene = iota + 1
	// sceneAfterAutoSend is the reset of the /runpr pane, after the
	// automatic send of the next command.
	sceneAfterAutoSend
)

// String names the scene for the records and the output.
func (s skipScene) String() string {
	switch s {
	case sceneAfterRunpr:
		return "after_runpr"
	case sceneAfterAutoSend:
		return "after_autosend"
	default:
		return unknownName
	}
}

// skipScenes are the scenes by the names a record holds.
var skipScenes = map[string]skipScene{
	sceneAfterRunpr.String():    sceneAfterRunpr,
	sceneAfterAutoSend.String(): sceneAfterAutoSend,
}

// ranCommand is the command that ran in a pane, as the transcript check
// compares it: its kind, its argument (empty for /mkreq, whose issue no
// document names, meaning any non-empty argument), and its route. It is used
// only to show and to compare, never to type.
type ranCommand struct {
	kind     paneinput.CommandKind
	argument string
	route    paneinput.RouteKey
}

// text returns the command line as the human reads it: "/mkreq" for a
// /mkreq, whose argument is not known.
func (c ranCommand) text() string {
	if c.argument == "" {
		return "/" + c.kind.Name()
	}
	return "/" + c.kind.Name() + " " + c.argument
}

// maker returns the command as the check of the reset after /runpr compares
// it.
func (c ranCommand) maker() maker {
	return maker{route: c.route, name: c.kind.Name(), args: c.argument}
}

// commandKinds are the command kinds by their names.
var commandKinds = map[string]paneinput.CommandKind{
	paneinput.CommandMkreq.Name():   paneinput.CommandMkreq,
	paneinput.CommandMkarch.Name():  paneinput.CommandMkarch,
	paneinput.CommandMkplan.Name():  paneinput.CommandMkplan,
	paneinput.CommandRunplan.Name(): paneinput.CommandRunplan,
	paneinput.CommandRunpr.Name():   paneinput.CommandRunpr,
}

// routeName returns the routing table's row name of key, "" for none.
func routeName(key paneinput.RouteKey) string {
	for name, k := range routingRows {
		if k == key {
			return name
		}
	}
	return ""
}

// errSkipRecord reports a skip record, or the values of one, that is not in
// the accepted form.
var errSkipRecord = errors.New("unreadable skip record")

// checkRanCommand checks that cmd is a command of scene in its accepted
// form: a kind of the scene, a route the kind takes, and an argument of the
// kind's form, written as its parser writes it.
func checkRanCommand(cmd ranCommand, scene skipScene) error {
	switch scene {
	case sceneAfterRunpr:
		if cmd.kind == paneinput.CommandRunpr || cmd.kind.Name() == "" {
			return fmt.Errorf("%w: the command is no command that creates a PR", errSkipRecord)
		}
	case sceneAfterAutoSend:
		if cmd.kind != paneinput.CommandRunpr {
			return fmt.Errorf("%w: the command of the reset after an automatic send is not /runpr", errSkipRecord)
		}
	default:
		return fmt.Errorf("%w: no scene", errSkipRecord)
	}
	var routeOK, argOK bool
	switch cmd.kind {
	case paneinput.CommandMkreq:
		routeOK, argOK = cmd.route == paneinput.RouteTaskDocuments, cmd.argument == ""
	case paneinput.CommandMkarch, paneinput.CommandMkplan:
		task, err := paneinput.ParseTaskID(cmd.argument)
		routeOK, argOK = cmd.route == paneinput.RouteTaskDocuments, err == nil && task.String() == cmd.argument
	case paneinput.CommandRunplan:
		task, err := paneinput.ParseTaskID(cmd.argument)
		switch cmd.route {
		case paneinput.RouteRunplanHigh, paneinput.RouteRunplanStandardReview, paneinput.RouteRunplanLow, paneinput.RouteRunplanTakeover:
			routeOK = true
		}
		argOK = err == nil && task.String() == cmd.argument
	case paneinput.CommandRunpr:
		pr, err := fixprguard.ParsePRNumber(cmd.argument)
		routeOK, argOK = cmd.route == paneinput.RouteRunpr, err == nil && pr.String() == cmd.argument
	}
	switch {
	case !routeOK:
		return fmt.Errorf("%w: the route is not one of the command", errSkipRecord)
	case !argOK:
		return fmt.Errorf("%w: the argument is not of the command's form", errSkipRecord)
	}
	return nil
}

// skipRecordFile is the JSON form of a skip record.
type skipRecordFile struct {
	Workspace string `json:"workspace"`
	Pane      string `json:"pane"`
	Session   string `json:"session"`
	Seq       int64  `json:"seq"`
	Agent     string `json:"agent"`
	Scene     string `json:"scene"`
	Command   string `json:"command"`
	Argument  string `json:"argument"`
	Route     string `json:"route"`
	Saved     string `json:"saved"`
}

// memberCommand and memberPane are the members of a skip record that name
// its command and its pane.
const (
	memberCommand = "command"
	memberPane    = "pane"
	memberAgent   = "agent"
	memberSpace   = "workspace"
)

// skipRecordKeys are the members of a skip record, every one required.
var skipRecordKeys = []string{memberSpace, memberPane, "session", "seq", memberAgent, "scene", memberCommand, "argument", "route", "saved"}

// skipRecord is a skipped reset kept so a later round offers it to the
// human. Only newSkipRecord and parseSkipRecord make one, so every value
// passed the checks of a record.
type skipRecord struct {
	workspace string
	pane      PaneID
	session   string
	seq       int64
	agent     paneinput.AgentKind
	scene     skipScene
	command   ranCommand
	saved     time.Time
}

// newSkipRecord returns the record of a reset skipped in the Herdr workspace
// for the pane a, as read when it was skipped. Contract: a value outside the
// accepted form is an error, never written: the workspace ID of Herdr's
// form, an agent chief knows, a Claude Code pane with a session of Herdr's
// form and an OpenCode pane with none, and a command of the scene.
func newSkipRecord(workspace string, a Agent, scene skipScene, cmd ranCommand, saved time.Time) (skipRecord, error) {
	rec := skipRecord{workspace: workspace, pane: a.Pane, session: a.Session, seq: a.StateChangeSeq, agent: a.Kind, scene: scene, command: cmd, saved: saved.UTC().Truncate(time.Second)}
	if err := rec.check(); err != nil {
		return skipRecord{}, err
	}
	return rec, nil
}

// check checks the values of rec.
func (r skipRecord) check() error {
	if !workspaceIDForm.MatchString(r.workspace) {
		return fmt.Errorf("%w: the workspace is not a Herdr workspace ID", errSkipRecord)
	}
	if !paneIDForm.MatchString(r.pane.String()) {
		return fmt.Errorf("%w: the pane is not a Herdr pane ID", errSkipRecord)
	}
	if r.seq < 0 {
		return fmt.Errorf("%w: a negative state_change_seq", errSkipRecord)
	}
	switch r.agent {
	case paneinput.AgentClaudeCode:
		if !sessionIDForm.MatchString(r.session) {
			return fmt.Errorf("%w: a Claude Code pane without a session", errSkipRecord)
		}
	case paneinput.AgentOpenCode:
		if r.session != "" {
			return fmt.Errorf("%w: an OpenCode pane with a session", errSkipRecord)
		}
		// No human input of an OpenCode pane is recorded, so its reset
		// after /runpr is never offered.
		if r.scene == sceneAfterRunpr {
			return fmt.Errorf("%w: an OpenCode pane after /runpr", errSkipRecord)
		}
	default:
		return fmt.Errorf("%w: no agent chief knows", errSkipRecord)
	}
	return checkRanCommand(r.command, r.scene)
}

// name returns the file name of the record: the identity of the skipped
// reset, its pane, session, and state_change_seq.
func (r skipRecord) name() string {
	return skipName(r.pane, r.session, r.seq)
}

// skipName is the file name of the skip record of a pane's session and
// state_change_seq. No part holds a NUL byte, so two identities never give
// one name.
func skipName(pane PaneID, session string, seq int64) string {
	sum := sha256.Sum256([]byte(pane.String() + "\x00" + session + "\x00" + strconv.FormatInt(seq, 10)))
	return skipPrefix + hex.EncodeToString(sum[:]) + recordSuffix
}

// file returns the JSON form of the record.
func (r skipRecord) file() skipRecordFile {
	return skipRecordFile{
		Workspace: r.workspace,
		Pane:      r.pane.String(),
		Session:   r.session,
		Seq:       r.seq,
		Agent:     agentLabel(r.agent),
		Scene:     r.scene.String(),
		Command:   r.command.kind.Name(),
		Argument:  r.command.argument,
		Route:     routeName(r.command.route),
		Saved:     r.saved.Format(time.RFC3339),
	}
}

// recordAgents are the agents by the names a record holds.
var recordAgents = map[string]paneinput.AgentKind{
	agentLabel(paneinput.AgentClaudeCode): paneinput.AgentClaudeCode,
	agentLabel(paneinput.AgentOpenCode):   paneinput.AgentOpenCode,
}

// parseSkipRecord reads a skip record. Contract: the data is one JSON object
// with exactly the record's members, each once and of its type, and nothing
// after it (strictjson, since encoding/json takes the last of two members of
// one name and matches names without regard to case); every value passes
// the checks of newSkipRecord; anything else is errSkipRecord, never read
// in part or repaired.
func parseSkipRecord(data []byte) (skipRecord, error) {
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return skipRecord{}, fmt.Errorf("%w: %w", errSkipRecord, err)
	}
	members, err := object.CollectOnly(skipRecordKeys...)
	if err != nil {
		return skipRecord{}, fmt.Errorf("%w: %w", errSkipRecord, err)
	}
	strs := map[string]string{}
	for _, key := range skipRecordKeys {
		if key == "seq" {
			continue
		}
		v, err := strictjson.Required(members, key)
		if err != nil {
			return skipRecord{}, fmt.Errorf("%w: %w", errSkipRecord, err)
		}
		s, err := v.AsString()
		if err != nil {
			return skipRecord{}, fmt.Errorf("%w: %s: %w", errSkipRecord, key, err)
		}
		strs[key] = s
	}
	seqValue, err := strictjson.Required(members, "seq")
	if err != nil {
		return skipRecord{}, fmt.Errorf("%w: %w", errSkipRecord, err)
	}
	seq, err := seqValue.AsNonNegativeInt64()
	if err != nil {
		return skipRecord{}, fmt.Errorf("%w: seq: %w", errSkipRecord, err)
	}
	pane, err := ParsePaneID(strs[memberPane])
	if err != nil {
		return skipRecord{}, fmt.Errorf("%w: %w", errSkipRecord, err)
	}
	agent, ok := recordAgents[strs["agent"]]
	if !ok {
		return skipRecord{}, fmt.Errorf("%w: agent", errSkipRecord)
	}
	scene, ok := skipScenes[strs["scene"]]
	if !ok {
		return skipRecord{}, fmt.Errorf("%w: scene", errSkipRecord)
	}
	kind, ok := commandKinds[strs[memberCommand]]
	if !ok {
		return skipRecord{}, fmt.Errorf("%w: command", errSkipRecord)
	}
	route, ok := routingRows[strs["route"]]
	if !ok {
		return skipRecord{}, fmt.Errorf("%w: route", errSkipRecord)
	}
	saved, err := time.Parse(time.RFC3339, strs["saved"])
	if err != nil {
		return skipRecord{}, fmt.Errorf("%w: saved: %w", errSkipRecord, err)
	}
	rec := skipRecord{workspace: strs[memberSpace], pane: pane, session: strs["session"], seq: seq, agent: agent, scene: scene, command: ranCommand{kind: kind, argument: strs["argument"], route: route}, saved: saved}
	if err := rec.check(); err != nil {
		return skipRecord{}, err
	}
	return rec, nil
}

// recordSkip writes rec. Contract: a record is never replaced; a record of
// the same identity that exists already is the same skip kept before, which
// is no failure.
func (h *HeldWorkspace) recordSkip(rec skipRecord) error {
	err := h.store().create(rec.name(), rec.file())
	if errors.Is(err, privatefile.ErrExist) {
		return nil
	}
	return err
}

// skipRecordFound is one skip record of a workspace's directory: the record,
// or why its file cannot be read.
type skipRecordFound struct {
	path string
	rec  skipRecord
	err  error
}

// skipRecords reads every skip record of the workspace. A record that cannot
// be read is returned with its error, so the caller can name it; err is a
// failure to list the directory.
func (r WorkspaceRecords) skipRecords() ([]skipRecordFound, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var found []skipRecordFound
	for _, e := range entries {
		if !skipFileName.MatchString(e.Name()) {
			continue
		}
		path := filepath.Join(r.dir, e.Name())
		f := skipRecordFound{path: path}
		data, err := privatefile.Read(path, recordLimit)
		switch {
		case errors.Is(err, privatefile.ErrNotExist):
			// Removed under the exclusion since it was listed.
			continue
		case err != nil:
			f.err = err
		default:
			f.rec, f.err = parseSkipRecord(data)
			if f.err == nil && f.rec.name() != e.Name() {
				// A record under another identity's name would be removed
				// by neither its use nor a send to its pane.
				f.err = fmt.Errorf("%w: the name is not the record's identity", errSkipRecord)
			}
		}
		found = append(found, f)
	}
	return found, nil
}

// useSkip removes rec, so its reset item never comes back. Contract: a
// record already gone is an error, since another process used it.
func (h *HeldWorkspace) useSkip(rec skipRecord) error {
	return privatefile.Remove(filepath.Join(h.dir, rec.name()))
}

// removePaneSkips removes every skip record of pane, after an input was
// typed into it: a later skip of that pane is a new one. A record that
// cannot be read is left for the cleanup of start.
func (h *HeldWorkspace) removePaneSkips(pane PaneID) error {
	found, err := h.skipRecords()
	if err != nil {
		return err
	}
	var errs []error
	for _, f := range found {
		if f.err != nil || f.rec.pane != pane {
			continue
		}
		if err := privatefile.Remove(f.path); err != nil && !errors.Is(err, privatefile.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// removeOldSkips removes the skip records saved more than detailMaxAge
// before now, and the skip records that cannot be read whose file was last
// changed that long ago. It returns how many it removed.
func (h *HeldWorkspace) removeOldSkips(now time.Time) (int, error) {
	found, err := h.skipRecords()
	if err != nil {
		return 0, err
	}
	removed := 0
	var errs []error
	for _, f := range found {
		saved := f.rec.saved
		if f.err != nil {
			info, err := os.Lstat(f.path)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			saved = info.ModTime()
		}
		if now.Sub(saved) <= detailMaxAge {
			continue
		}
		if err := privatefile.Remove(f.path); err != nil && !errors.Is(err, privatefile.ErrNotExist) {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}
