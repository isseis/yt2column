//go:build test

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

const (
	commandsDir = ".claude/commands"
	skillsDir   = ".agents/skills"
)

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

// skillFrontMatter returns the name and description of a skill's YAML front
// matter, failing t when the document does not open with a block of exactly
// those two keys.
func skillFrontMatter(t *testing.T, rel, doc string) (name, description string) {
	t.Helper()
	rest, ok := strings.CutPrefix(doc, "---\n")
	if !ok {
		t.Fatalf("%s does not start with YAML front matter", rel)
	}
	block, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		t.Fatalf("%s: front matter is not closed", rel)
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(block, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			t.Fatalf("%s: front matter line %q is not a key and a value", rel, line)
		}
		fields[key] = strings.Trim(value, `"`)
	}
	if len(fields) != 2 {
		t.Fatalf("%s: front matter keys = %v, want exactly name and description", rel, fields)
	}
	return fields["name"], fields["description"]
}

// TestSkillsMatchCommands: .agents/skills holds one thin entry per developer
// command and nothing else, and each entry points at its command definition,
// the shared context, and the harness entry.
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
			name, description := skillFrontMatter(t, rel, doc)
			if name != cmd {
				t.Errorf("%s: name = %q, want %q", rel, name, cmd)
			}
			if strings.TrimSpace(description) == "" {
				t.Errorf("%s: description is empty", rel)
			}
			links := markdownLinkTargets(doc)
			for _, target := range []string{
				commandsDir + "/_context.md",
				commandsDir + "/_lib/harness.md",
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
