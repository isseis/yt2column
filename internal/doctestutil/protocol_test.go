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
