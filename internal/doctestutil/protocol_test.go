//go:build test

package doctestutil

import (
	"slices"
	"testing"
)

// TestProtocolFlags pins which tool each --protocol belongs to, the choice
// that splits the flags between the fixprguard and the codexreview guards.
func TestProtocolFlags(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	for _, tc := range []struct {
		name string
		doc  string
		want []ProtocolFlag
	}{
		{
			"both_tools_on_one_line",
			"fixprguard head --protocol 4 && codexreview --protocol 1 --base " + sha + "\n",
			[]ProtocolFlag{{"--protocol 4", "4", ToolFixprguard}, {"--protocol 1", "1", ToolCodexreview}},
		},
		{
			"another_flag_before_protocol",
			"codexreview --base " + sha + " --protocol 1\nfixprguard list --reviewer <reviewer> --protocol=4\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolCodexreview}, {"--protocol=4", "4", ToolFixprguard}},
		},
		{
			// The pipe ends codexreview's command, so the flag after it is
			// not codexreview's.
			"pipe",
			"codexreview --protocol 1 --base " + sha + " | tee x --protocol 9\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolCodexreview}, {"--protocol 9", "9", ""}},
		},
		{
			"semicolon",
			"codexreview --base " + sha + "; tee --protocol 9\n",
			[]ProtocolFlag{{"--protocol 9", "9", ""}},
		},
		{
			"and_list",
			"codexreview --base " + sha + " && tee --protocol 9\n",
			[]ProtocolFlag{{"--protocol 9", "9", ""}},
		},
		{
			// A substitution ends the command too, so the flag after one is
			// not the call's: the documents write a call's values literally.
			"substitution",
			"codexreview --base \"$(git rev-parse HEAD)\" --protocol 1\n",
			[]ProtocolFlag{{"--protocol 1", "1", ""}},
		},
		{
			"backquote_in_block",
			"codexreview --base `git rev-parse HEAD` --protocol 1\n",
			[]ProtocolFlag{{"--protocol 1", "1", ""}},
		},
		{
			// Each code span is its own command.
			"code_spans_in_prose",
			"Run `codexreview --protocol 1 --base <BASE>`, then `fixprguard begin --protocol 4`.\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolCodexreview}, {"--protocol 4", "4", ToolFixprguard}},
		},
		{
			"path_and_quotes",
			"\"/go/bin/codexreview\" --protocol 1 --base " + sha + "\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolCodexreview}},
		},
		{
			// A longer word that contains the name is not the tool.
			"name_inside_a_word",
			"codexreview-old --protocol 2\n",
			[]ProtocolFlag{{"--protocol 2", "2", ""}},
		},
		{
			// The value is the whole word, so a guard rejects one with a
			// tail the tool would reject.
			"value_with_a_tail",
			"codexreview --protocol 1junk --base " + sha + "\nfixprguard head --protocol=4x\n",
			[]ProtocolFlag{{"--protocol 1junk", "1junk", ToolCodexreview}, {"--protocol=4x", "4x", ToolFixprguard}},
		},
		{
			// chief's flag is its own, next to another tool's.
			"chief_and_codexreview",
			"chief snapshot --protocol 1 && codexreview --protocol 1 --base " + sha + "\n",
			[]ProtocolFlag{{"--protocol 1", "1", ToolChief}, {"--protocol 1", "1", ToolCodexreview}},
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
