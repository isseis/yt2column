//go:build test

package doctestutil

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestProtocolFlags pins which tool each --protocol belongs to, the choice
// that splits the flags across the fixprguard, chief, and mergepr guards.
func TestProtocolFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want []ProtocolFlag
	}{
		{
			"another_flag_before_protocol",
			"mergepr prepare 42 --protocol 1\nfixprguard list --reviewer <reviewer> --protocol=4\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolMergepr}, {"--protocol=4", "4", ToolFixprguard}},
		},
		{
			// The pipe ends mergepr's command, so the flag after it is
			// not mergepr's.
			"pipe",
			"mergepr prepare --protocol 1 42 | tee x --protocol 9\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolMergepr}, {"--protocol 9", "9", ""}},
		},
		{
			"semicolon",
			"mergepr prepare 42; tee --protocol 9\n",
			[]ProtocolFlag{{"--protocol 9", "9", ""}},
		},
		{
			"and_list",
			"mergepr prepare 42 && tee --protocol 9\n",
			[]ProtocolFlag{{"--protocol 9", "9", ""}},
		},
		{
			// A substitution ends the command too, so the flag after one is
			// not the call's: the documents write a call's values literally.
			"substitution",
			"mergepr prepare \"$(git rev-parse HEAD)\" --protocol 1\n",
			[]ProtocolFlag{{"--protocol 1", "1", ""}},
		},
		{
			"backquote_in_block",
			"mergepr prepare `git rev-parse HEAD` --protocol 1\n",
			[]ProtocolFlag{{"--protocol 1", "1", ""}},
		},
		{
			// Each code span is its own command.
			"code_spans_in_prose",
			"Run `mergepr prepare --protocol 1 <PR>`, then `fixprguard begin --protocol 4`.\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolMergepr}, {"--protocol 4", "4", ToolFixprguard}},
		},
		{
			"path_and_quotes",
			"\"/go/bin/mergepr\" prepare --protocol 1 42\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolMergepr}},
		},
		{
			// A longer word that contains the name is not the tool.
			"name_inside_a_word",
			"mergepr-old --protocol 2\n",
			[]ProtocolFlag{{"--protocol 2", "2", ""}},
		},
		{
			// The value is the whole word, so a guard rejects one with a
			// tail the tool would reject.
			"value_with_a_tail",
			"mergepr prepare --protocol 1junk 42\nfixprguard head --protocol=4x\n",
			[]ProtocolFlag{{"--protocol 1junk", "1junk", ToolMergepr}, {"--protocol=4x", "4x", ToolFixprguard}},
		},
		{
			// chief's flag is its own, next to another tool's.
			"chief_and_mergepr",
			"chief snapshot --protocol 1 && mergepr prepare --protocol 1 42\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolChief}, {"--protocol 1", "1", ToolMergepr}},
		},
		{
			// A longer word that contains chief's name is not chief.
			"chief_name_inside_a_word",
			"chiefs --protocol 2\n",
			[]ProtocolFlag{{"--protocol 2", "2", ""}},
		},
		{
			// mergepr's flag is its own, after the subcommand, next to
			// another tool's.
			"mergepr_and_fixprguard",
			"mergepr prepare --protocol 1 42 && fixprguard head --protocol 6\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolMergepr}, {"--protocol 6", "6", ToolFixprguard}},
		},
		{
			// A flag in prose with no tool before it is the fixprguard
			// guard's, as all flags were.
			"no_tool",
			"pass --protocol 4 each time\n",
			[]ProtocolFlag{{"--protocol 4", "4", ""}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProtocolFlags(tc.doc); !slices.Equal(got, tc.want) {
				t.Errorf("ProtocolFlags = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestProtocolCallers pins which of the given names each --protocol belongs
// to, read as ProtocolFlags reads a flag's tool, and that a flag with none of
// them before it gives nothing.
func TestProtocolCallers(t *testing.T) {
	names := []string{"alpha", "beta"}
	for _, tc := range []struct {
		name string
		doc  string
		want []string
	}{
		{"one_call", "alpha run --protocol 1\n", []string{"alpha"}},
		{"no_name", "pass --protocol 6 each time\n", nil},
		// Only the given names count, not the guards' tools.
		{"known_tool_not_in_names", "fixprguard head --protocol 6\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProtocolCallers(tc.doc, names); !slices.Equal(got, tc.want) {
				t.Errorf("ProtocolCallers = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCommandDocs: the command documents are the Markdown files under both
// .claude/commands and .agents/skills, and a missing or empty directory fails
// the caller instead of passing a guard over fewer documents.
func TestCommandDocs(t *testing.T) {
	write := func(t *testing.T, root, rel string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte("text\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	t.Run("both directories", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, ".claude/commands/a.md")
		write(t, root, ".claude/commands/_lib/b.md")
		write(t, root, ".claude/commands/c.sh")
		write(t, root, ".agents/skills/a/SKILL.md")
		got := CommandDocs(t, root)
		slices.Sort(got)
		want := []string{".agents/skills/a/SKILL.md", ".claude/commands/_lib/b.md", ".claude/commands/a.md"}
		if !slices.Equal(got, want) {
			t.Errorf("CommandDocs = %v, want %v", got, want)
		}
	})

	for _, tc := range []struct {
		name  string
		files []string
		dir   string
	}{
		{name: "skills directory missing", files: []string{".claude/commands/a.md"}, dir: ".agents/skills"},
		{name: "skills directory without a document", files: []string{".claude/commands/a.md", ".agents/skills/a/notes.txt"}, dir: ".agents/skills"},
		{name: "commands directory missing", files: []string{".agents/skills/a/SKILL.md"}, dir: ".claude/commands"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range tc.files {
				write(t, root, f)
			}
			fake := &recordingTB{TB: t}
			CommandDocs(fake, root)
			if !strings.Contains(fake.failure, tc.dir) {
				t.Errorf("CommandDocs failed with %q, want %s named", fake.failure, tc.dir)
			}
		})
	}
}
