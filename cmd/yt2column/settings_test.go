//go:build test

package main

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// settingsPath is the shared Claude Code settings file of the repository.
const settingsPath = ".claude/settings.json"

// bashRule matches a Bash permission rule and captures its command pattern.
var bashRule = regexp.MustCompile(`^Bash\((.*)\)$`)

// unpromptedWords are the only command words a Bash allow rule of the
// settings may name. Each was checked to run no command it is given:
// coderabbit runs a review, chief runs only herdr, git, and gh with
// arguments it builds, numreserve runs only git and gh with arguments it
// builds, and mergepr runs only git and gh with arguments it builds (and
// cmd/mergepr/docs_test.go restricts its rules to the three safe mergemain
// subcommands). A rule for any other word may run an arbitrary command without
// the approval prompt (a wrapper such as env, a shell, an interpreter, git
// through an alias, gh writing to GitHub, or a wildcard that matches one of
// them), so adding one means checking the command and naming it here.
var unpromptedWords = map[string]struct{}{
	"coderabbit": {},
	"chief":      {},
	"numreserve": {},
	"mergepr":    {},
}

// promptingModes are the permission modes that keep the prompt for a Bash
// command no rule allows.
var promptingModes = map[string]struct{}{
	"":            {},
	"default":     {},
	"plan":        {},
	"acceptEdits": {},
}

// allowsUnvetted reports whether an allow rule lets a Bash command whose
// command word is not one of unpromptedWords run without the approval
// prompt, the word written as is (a wildcard or a path is another word).
// Only the first word is read: Claude Code matches each command of a
// compound command against the rules on its own, so a rule's later words
// cannot allow a second command. A ":*" that ends the whole pattern, the
// legacy prefix form, is removed before the first word is cut, and is the
// only colon read past; any other colon, a ":*" before a later word
// included, makes the word another one.
func allowsUnvetted(rule string) bool {
	if rule != "Bash" && !strings.HasPrefix(rule, "Bash(") {
		return false
	}
	m := bashRule.FindStringSubmatch(rule)
	if m == nil {
		return true
	}
	pattern := strings.TrimSuffix(strings.TrimSpace(m[1]), ":*")
	word, _, _ := strings.Cut(pattern, " ")
	_, ok := unpromptedWords[word]
	return !ok
}

// TestSettingsAllowsOnlyVettedCommands: .claude/settings.json allows no Bash
// rule outside the vetted command words, and sets no permission mode that
// skips the prompt, so a command an agent runs through a wrapper, a shell,
// gh, or git push still reaches the approval prompt. An enabled sandbox
// must turn off its automatic approval of Bash commands explicitly, since
// that approval would skip the prompt for every command run in it.
func TestSettingsAllowsOnlyVettedCommands(t *testing.T) {
	var settings struct {
		Permissions struct {
			Allow       []string `json:"allow"`
			DefaultMode string   `json:"defaultMode"`
		} `json:"permissions"`
		Sandbox struct {
			Enabled                  bool  `json:"enabled"`
			AutoAllowBashIfSandboxed *bool `json:"autoAllowBashIfSandboxed"`
		} `json:"sandbox"`
	}
	doc := doctestutil.ReadDoc(t, filepath.Join(repositoryRoot, filepath.FromSlash(settingsPath)))
	if err := json.Unmarshal([]byte(doc), &settings); err != nil {
		t.Fatalf("parse %s: %v", settingsPath, err)
	}
	for _, rule := range settings.Permissions.Allow {
		if allowsUnvetted(rule) {
			t.Errorf("permissions.allow has %q, which may run an unvetted command without the prompt", rule)
		}
	}
	if _, ok := promptingModes[settings.Permissions.DefaultMode]; !ok {
		t.Errorf("permissions.defaultMode is %q, which skips the prompt", settings.Permissions.DefaultMode)
	}
	if sb := settings.Sandbox; sb.Enabled && (sb.AutoAllowBashIfSandboxed == nil || *sb.AutoAllowBashIfSandboxed) {
		t.Errorf("sandbox is enabled without autoAllowBashIfSandboxed set to false, which may skip the prompt")
	}
}

// TestAllowsUnvetted: the rules that may run an unvetted command without the
// prompt are found, and the existing Bash rules are not.
func TestAllowsUnvetted(t *testing.T) {
	for _, tc := range []struct {
		rule string
		want bool
	}{
		{"Bash(*)", true},
		{"Bash", true},
		{"Bash(gh *)", true},
		{"Bash(git push*)", true},
		// git runs a command through an alias starting with "!".
		{"Bash(git status*)", true},
		{"Bash(env *)", true},
		{"Bash(bash *)", true},
		{"Bash(eval *)", true},
		{"Bash(python3 -c *)", true},
		{"Bash(e*)", true},
		{"Bash(/bin/b*)", true},
		{"Bash(codex *)", true},
		// A word that starts with a vetted name is another command.
		{"Bash(chiefx *)", true},
		{"Bash(coderabbit review *)", false},
		{"Bash(chief *)", false},
		{"Bash(chief:*)", false},
		// Only a trailing ":*" is the legacy prefix form.
		{"Bash(chief:helper *)", true},
		{"Bash(chief:* helper)", true},
		{"Bash(chief run:*)", false},
		{"Bash(numreserve task)", false},
		{"Bash(numreserve adr)", false},
		{"Bash(mergepr mergemain-begin *)", false},
		{"Read(.envrc)", false},
	} {
		if got := allowsUnvetted(tc.rule); got != tc.want {
			t.Errorf("allowsUnvetted(%q) = %v, want %v", tc.rule, got, tc.want)
		}
	}
}
