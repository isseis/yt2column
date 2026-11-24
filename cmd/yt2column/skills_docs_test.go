//go:build test

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const (
	commandsDir = ".claude/commands"
	skillsDir   = ".agents/skills"
)

var errInvalidSkillFrontMatter = errors.New("invalid skill front matter")

// skillCommands returns the names of the developer commands that have an
// Antigravity skill: every top-level command definition except the files
// starting with "_" (the shared context and procedures) and chief, which
// drives Herdr panes and stays on Claude Code.
func skillCommands(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repositoryRoot, commandsDir))
	if err != nil {
		t.Fatalf("read %s: %v", commandsDir, err)
	}
	var names []string
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if e.IsDir() || !ok || strings.HasPrefix(name, "_") || name == "chief" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// parseSkillFrontMatter parses the YAML front matter of a skill document,
// requiring an opening and closing "---" fence with exactly the unique keys
// "name" and "description", each with a non-empty bare scalar or valid
// double-quoted string value.
func parseSkillFrontMatter(doc string) (name, description string, err error) {
	rest, ok := strings.CutPrefix(doc, "---\n")
	if !ok {
		return "", "", fmt.Errorf("%w: does not start with YAML front matter", errInvalidSkillFrontMatter)
	}
	block, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return "", "", fmt.Errorf("%w: front matter is not closed", errInvalidSkillFrontMatter)
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(block, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			return "", "", fmt.Errorf("%w: line %q is not a key and a value", errInvalidSkillFrontMatter, line)
		}
		if key != "name" && key != "description" {
			return "", "", fmt.Errorf("%w: unexpected key %q", errInvalidSkillFrontMatter, key)
		}
		if _, dup := fields[key]; dup {
			return "", "", fmt.Errorf("%w: duplicate key %q", errInvalidSkillFrontMatter, key)
		}
		if strings.HasPrefix(value, `"`) || strings.HasSuffix(value, `"`) {
			// strconv.Unquote rejects unbalanced quotes, unescaped inner quotes,
			// and invalid escape sequences in double-quoted YAML scalars.
			unquoted, unquoteErr := strconv.Unquote(value)
			if unquoteErr != nil {
				return "", "", fmt.Errorf("%w: invalid quoted value %q: %v", errInvalidSkillFrontMatter, value, unquoteErr)
			}
			value = unquoted
		} else if strings.HasPrefix(value, "'") || strings.HasSuffix(value, "'") {
			return "", "", fmt.Errorf("%w: single-quoted value %q is not accepted", errInvalidSkillFrontMatter, value)
		}
		if strings.TrimSpace(value) == "" {
			return "", "", fmt.Errorf("%w: %s is empty", errInvalidSkillFrontMatter, key)
		}
		fields[key] = value
	}
	if len(fields) != 2 {
		return "", "", fmt.Errorf("%w: keys = %v, want exactly name and description", errInvalidSkillFrontMatter, fields)
	}
	return fields["name"], fields["description"], nil
}

// TestSkillFrontMatter pins the YAML front-matter grammar of SKILL.md files:
// valid bare and double-quoted strings pass, while unescaped inner quotes,
// unbalanced quotes, duplicate or unknown keys, and missing fences fail.
func TestSkillFrontMatter(t *testing.T) {
	for _, tc := range []struct {
		name     string
		doc      string
		wantName string
		wantDesc string
		wantErr  bool
	}{
		{
			name:     "bare name and double-quoted description",
			doc:      "---\nname: fixpr\ndescription: \"Resolve unresolved PR review threads\"\n---\n",
			wantName: "fixpr",
			wantDesc: "Resolve unresolved PR review threads",
		},
		{
			name:     "escaped inner quotes in double-quoted description",
			doc:      "---\nname: fixpr\ndescription: \"Run \\\"fixpr\\\" safely\"\n---\n",
			wantName: "fixpr",
			wantDesc: `Run "fixpr" safely`,
		},
		{
			name:     "bare description",
			doc:      "---\nname: fixpr\ndescription: Resolve unresolved PR review threads\n---\n",
			wantName: "fixpr",
			wantDesc: "Resolve unresolved PR review threads",
		},
		{
			name:    "unescaped inner quotes",
			doc:     "---\nname: fixpr\ndescription: \"Run \"fixpr\" safely\"\n---\n",
			wantErr: true,
		},
		{
			name:    "unbalanced opening quote",
			doc:     "---\nname: fixpr\ndescription: \"unclosed\n---\n",
			wantErr: true,
		},
		{
			name:    "unbalanced closing quote",
			doc:     "---\nname: fixpr\ndescription: unclosed\"\n---\n",
			wantErr: true,
		},
		{
			name:    "single quote character only",
			doc:     "---\nname: fixpr\ndescription: \"\n---\n",
			wantErr: true,
		},
		{
			name:    "single-quoted value",
			doc:     "---\nname: fixpr\ndescription: 'Resolve threads'\n---\n",
			wantErr: true,
		},
		{
			name:    "duplicate key",
			doc:     "---\nname: fixpr\nname: other\ndescription: \"Resolve threads\"\n---\n",
			wantErr: true,
		},
		{
			name:    "unknown key",
			doc:     "---\nname: fixpr\ndescription: \"Resolve threads\"\nextra: value\n---\n",
			wantErr: true,
		},
		{
			name:    "missing key",
			doc:     "---\nname: fixpr\n---\n",
			wantErr: true,
		},
		{
			name:    "empty quoted description",
			doc:     "---\nname: fixpr\ndescription: \"\"\n---\n",
			wantErr: true,
		},
		{
			name:    "missing opening fence",
			doc:     "name: fixpr\ndescription: \"Resolve threads\"\n---\n",
			wantErr: true,
		},
		{
			name:    "missing closing fence",
			doc:     "---\nname: fixpr\ndescription: \"Resolve threads\"\n",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotDesc, err := parseSkillFrontMatter(tc.doc)
			if tc.wantErr {
				if !errors.Is(err, errInvalidSkillFrontMatter) {
					t.Fatalf("parseSkillFrontMatter error = %v, want %v", err, errInvalidSkillFrontMatter)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSkillFrontMatter unexpected error: %v", err)
			}
			if gotName != tc.wantName || gotDesc != tc.wantDesc {
				t.Errorf("parseSkillFrontMatter = (%q, %q), want (%q, %q)", gotName, gotDesc, tc.wantName, tc.wantDesc)
			}
		})
	}
}

// TestSkillsMatchCommands: .agents/skills holds one thin entry per developer
// command and nothing else, and each entry points at its command definition,
// the shared context, the harness entry and Antigravity adapter, and the
// prohibition documents.
func TestSkillsMatchCommands(t *testing.T) {
	want := skillCommands(t)
	entries, err := os.ReadDir(filepath.Join(repositoryRoot, skillsDir))
	if err != nil {
		t.Fatalf("read %s: %v", skillsDir, err)
	}
	var got []string
	for _, e := range entries {
		if !e.IsDir() {
			t.Errorf("%s/%s is a file; only one directory per command is allowed", skillsDir, e.Name())
			continue
		}
		got = append(got, e.Name())
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("skill directories = %v, want one per command %v (chief has none)", got, want)
	}

	for _, cmd := range want {
		t.Run(cmd, func(t *testing.T) {
			dir := filepath.Join(repositoryRoot, skillsDir, cmd)
			files, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read %s: %v", dir, err)
			}
			if len(files) != 1 || files[0].IsDir() || files[0].Name() != "SKILL.md" {
				t.Errorf("%s/%s holds %d entries, want only SKILL.md", skillsDir, cmd, len(files))
			}
			rel := skillsDir + "/" + cmd + "/SKILL.md"
			doc := doctestutil.ReadDoc(t, filepath.Join(repositoryRoot, filepath.FromSlash(rel)))
			name, _, err := parseSkillFrontMatter(doc)
			if err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			if name != cmd {
				t.Errorf("%s: name = %q, want %q", rel, name, cmd)
			}
			links := markdownLinkTargets(doc)
			for _, target := range []string{
				commandsDir + "/_context.md",
				commandsDir + "/_lib/harness.md",
				commandsDir + "/_lib/harness-antigravity.md",
				commandsDir + "/" + cmd + ".md",
				"AGENTS.md",
				"CLAUDE.md",
			} {
				if !slices.Contains(links, "../../../"+target) {
					t.Errorf("%s does not link to %s", rel, target)
				}
			}
		})
	}
}
