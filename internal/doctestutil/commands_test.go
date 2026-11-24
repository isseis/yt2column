//go:build test

package doctestutil

import (
	"slices"
	"testing"
)

// TestRunsCommandHonoursTheName: RunsCommand finds the command it is asked
// about, through a wrapper as well, and not another one.
func TestRunsCommandHonoursTheName(t *testing.T) {
	for _, tc := range []struct {
		text, name string
		want       bool
	}{
		{"herdr agent prompt --pane x hi", "herdr", true},
		{"env FOO=1 herdr pane run x", "herdr", true},
		{"widget exec review", "herdr", false},
		{"herdr agent prompt x", "widget", false},
	} {
		if got := RunsCommand(tc.text, tc.name); got != tc.want {
			t.Errorf("RunsCommand(%q, %q) = %v, want %v", tc.text, tc.name, got, tc.want)
		}
	}
}

func TestRunsCommandForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want bool
	}{
		{"block_line_start", "```bash\nwidget exec review --base x\n```\n", true},
		{"after_pipe", "```bash\necho x | widget exec -\n```\n", true},
		{"after_or", "```bash\ntrue || widget review\n```\n", true},
		{"after_semicolon", "```bash\ncd x; widget review\n```\n", true},
		{"after_and", "```bash\ncd x && widget review\n```\n", true},
		{"background", "```bash\nsleep 1 & widget review\n```\n", true},
		{"substitution", "```bash\nout=$(widget review)\n```\n", true},
		{"backquote", "```bash\nout=`widget review`\n```\n", true},
		{"env", "```bash\nenv FOO=1 widget review\n```\n", true},
		{"env_ignore_environment", "```bash\nenv -i widget review\n```\n", true},
		{"sudo_preserve_env", "```bash\nsudo -E widget review\n```\n", true},
		{"caffeinate", "```bash\ncaffeinate -i widget review\n```\n", true},
		{"subshell_no_argument", "```bash\n(cd x && widget)\n```\n", true},
		{"env_path", "```bash\nenv -P /bin widget review\n```\n", true},
		{"env_unset", "```bash\nenv -u FOO widget review\n```\n", true},
		// env -S splits its value into env's own arguments.
		{"env_split_string", "```bash\nenv -S \"widget exec review\"\n```\n", true},
		{"env_split_string_attached", "```bash\nenv -S'-i widget exec'\n```\n", true},
		{"env_split_string_in_cluster", "```bash\nenv -iSwidget exec\n```\n", true},
		{"env_split_string_long", "```bash\nenv --split-string='widget exec'\n```\n", true},
		// -u takes the rest of its cluster as the name, so this S is no -S.
		{"env_unset_cluster_holds_s", "```bash\nenv -uSwidget echo\n```\n", false},
		{"assignment", "```bash\nFOO=1 widget review\n```\n", true},
		{"redirection", "```bash\n2>/dev/null widget review\n```\n", true},
		// A redirection written as its own word takes the next word as its
		// target, so the command after them is the one that runs; in a block
		// that is not quoted, a leading '>' is also kept as a redirection.
		{"redirection_word", "```bash\n> /tmp/log widget review\n```\n", true},
		{"redirection_append_word", "```bash\n2>> /tmp/log widget review\n```\n", true},
		{"redirection_both", "```bash\n&>/tmp/log widget review\n```\n", true},
		{"redirection_both_word", "```bash\n&> /tmp/log widget review\n```\n", true},
		{"redirection_both_append_word", "```bash\n&>> /tmp/log widget review\n```\n", true},
		{"heredoc_word", "```bash\n<< EOF widget review\n```\n", true},
		{"heredoc", "```bash\n<<EOF widget review\n```\n", true},
		{"herestring_word", "```bash\n<<< input widget review\n```\n", true},
		// The target is consumed, so a word after it is the command.
		{"redirection_target_named_widget", "```bash\n> widget echo x\n```\n", false},
		{"exec", "```bash\nexec widget review\n```\n", true},
		{"command", "```bash\ncommand widget review\n```\n", true},
		{"xargs", "```bash\nprintf x | xargs -n 1 widget review\n```\n", true},
		{"xargs_replace", "```bash\nprintf x | xargs -I {} widget review {}\n```\n", true},
		{"time", "```bash\ntime widget review\n```\n", true},
		{"timeout", "```bash\ntimeout 600 widget exec review\n```\n", true},
		{"timeout_kill_after", "```bash\ntimeout -k 5s 10m widget exec review\n```\n", true},
		{"nice", "```bash\nnice -n 5 widget review\n```\n", true},
		{"sudo", "```bash\nsudo widget review\n```\n", true},
		{"nohup", "```bash\nnohup widget review\n```\n", true},
		{"if", "```bash\nif widget exec review; then\n```\n", true},
		{"if_not", "```bash\nif ! widget exec review; then\n```\n", true},
		{"then_after_semicolon", "```bash\nif true; then widget review; fi\n```\n", true},
		{"group", "```bash\n{ widget review; }\n```\n", true},
		{"subshell", "```bash\n(widget review)\n```\n", true},
		{"backslash", "```bash\n\\widget review\n```\n", true},
		{"sh_c", "```bash\nsh -c 'widget review'\n```\n", true},
		{"zsh_c", "```bash\nzsh -c 'widget review'\n```\n", true},
		{"bash_lc", "```bash\nbash -lc 'widget review'\n```\n", true},
		// No boundary inside the script, so only the -c case finds it.
		{"bash_c_wrapper", "```bash\nbash -c 'env widget review'\n```\n", true},
		{"absolute_path", "```bash\n/opt/homebrew/bin/widget review\n```\n", true},
		{"relative_path", "```bash\n./widget review\n```\n", true},
		{"quoted", "```bash\n\"widget\" review\n```\n", true},
		{"npx", "```bash\nnpx @example/widget review\n```\n", true},
		{"npx_option", "```bash\nnpx -y @example/widget review\n```\n", true},
		{"bunx", "```bash\nbunx @example/widget review\n```\n", true},
		{"pnpm_dlx", "```bash\npnpm dlx @example/widget review\n```\n", true},
		{"blockquoted_block", "> ```bash\n> widget review\n> ```\n", true},
		{"blockquoted_block_redirection", "> ```bash\n> > /tmp/log widget review\n> ```\n", true},
		{"code_span", "Run `widget exec review --base <BASE>` once.\n", true},
		{"other_language_block", "```text\nwidget exec review --base <BASE>\n```\n", true},
		// A block line that starts with the quoted word is read as a command,
		// so a jq continuation must not start with it.
		{"quoted_line_start", "```bash\n  \"widget\" as $k\n```\n", true},

		// Mentions in data and prose do not invoke the command.
		{"jq_kind", "```bash\n  else {kind: \"widget\", url: .html_url, created_at} end\n```\n", false},
		{"jq_comparison", "```bash\n  | select(.kind == \"widget\")\n```\n", false},
		{"comment_kind_span", "A `widget` is any other comment by the same author.\n", false},
		{"html_marker_span", "Its first line is `<!-- widget-pull-request-review-summary -->`.\n", false},
		{"login", "```bash\nchatgpt-widget-connector review\n```\n", false},
		{"mention", "| PR review request | `@widget review` - the body of the comment |\n", false},
		{"tool", "```bash\nwidgetreview --protocol 1 --base <BASE>\n```\n", false},
		{"prose_without_span", "widget exec review.\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.ContainsFunc(CommandTexts(tc.doc), func(text string) bool { return RunsCommand(text, "widget") })
			if got != tc.want {
				t.Errorf("runs widget = %v, want %v (texts %q)", got, tc.want, CommandTexts(tc.doc))
			}
		})
	}
}

// TestCommandArgsReturnsEachInvocation: CommandArgs extracts the trailing
// arguments for every command-position invocation (including multiple commands
// inside shell -c and env -S without leaking outer quotes) and ignores
// non-command-position mentions.
func TestCommandArgsReturnsEachInvocation(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want [][]string
	}{
		{
			name: "chained_invocations",
			text: `widget one && widget two`,
			want: [][]string{{"one"}, {"two"}},
		},
		{
			name: "shell_c_and_env_split_string",
			text: `bash -c 'widget one' && env -S"widget -x two"`,
			want: [][]string{{"one"}, {"-x", "two"}},
		},
		{
			name: "not_in_command_position",
			text: `echo widget one`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CommandArgs(tc.text, "widget")
			if len(got) != len(tc.want) {
				t.Fatalf("CommandArgs(%q) = %q, want %q", tc.text, got, tc.want)
			}
			for i := range tc.want {
				if !slices.Equal(got[i], tc.want[i]) {
					t.Errorf("CommandArgs(%q)[%d] = %q, want %q", tc.text, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestFencedCommandBlocksPreservesUnquotedRedirection: FencedCommandBlocks
// strips only the opening fence's blockquote depth, preserves info strings,
// and marks unclosed fences with Closed=false.
func TestFencedCommandBlocksPreservesUnquotedRedirection(t *testing.T) {
	doc := "```bash\n> /tmp/log cmd\n```\n\n> ```sh\n> > /tmp/log cmd2\n> ```\n\n```bash\nunclosed\n"
	want := []FencedBlock{
		{Info: "bash", Lines: []string{"> /tmp/log cmd"}, Closed: true},
		{Info: "sh", Lines: []string{"> /tmp/log cmd2"}, Closed: true},
		{Info: "bash", Lines: []string{"unclosed"}, Closed: false},
	}
	got := FencedCommandBlocks(doc)
	if len(got) != len(want) {
		t.Fatalf("FencedCommandBlocks returned %d blocks, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Info != want[i].Info || got[i].Closed != want[i].Closed || !slices.Equal(got[i].Lines, want[i].Lines) {
			t.Errorf("block[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestBashToolRuleMismatches: BashToolRuleMismatches flags every extra Bash
// rule naming the tool (by bare name, path, quotes, legacy colon, or wildcard
// prefix), reports missing allowed rules, and ignores unrelated tools, prefix
// collisions, and non-Bash rules.
func TestBashToolRuleMismatches(t *testing.T) {
	allowed := []string{"Bash(widget alpha *)", "Bash(widget beta *)"}
	for _, tc := range []struct {
		name           string
		allow          []string
		extra, missing []string
	}{
		{
			name: "exact_with_unrelated_rules",
			allow: []string{
				"Bash(other *)",
				"Bash(widgetx alpha *)",
				"Read(widget)",
				"Bash(widget alpha *)",
				"Bash(widget beta *)",
			},
		},
		{
			name: "extra_spellings_and_subcommands",
			allow: []string{
				"Bash(widget alpha *)",
				"Bash(widget beta *)",
				"Bash(widget gamma *)",
				"Bash(widget *)",
				"Bash(widget:*)",
				"Bash(/usr/local/bin/widget alpha *)",
				`Bash("widget" alpha *)`,
				"Bash(widget*)",
			},
			extra: []string{
				"Bash(widget gamma *)",
				"Bash(widget *)",
				"Bash(widget:*)",
				"Bash(/usr/local/bin/widget alpha *)",
				`Bash("widget" alpha *)`,
				"Bash(widget*)",
			},
		},
		{
			name:    "missing_allowed_rule",
			allow:   []string{"Bash(widget alpha *)"},
			missing: []string{"Bash(widget beta *)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extra, missing := BashToolRuleMismatches(tc.allow, "widget", allowed)
			if !slices.Equal(extra, tc.extra) || !slices.Equal(missing, tc.missing) {
				t.Errorf("BashToolRuleMismatches = extra %q, missing %q; want %q, %q", extra, missing, tc.extra, tc.missing)
			}
		})
	}
}
