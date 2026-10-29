//go:build test

package main

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/codexreview"
	"github.com/isseis/yt2column/internal/doctestutil"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

// toolName is the tool's command name: the name of this package's directory
// under cmd/, which go install gives the binary.
func toolName(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Base(dir)
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	return doctestutil.ReadDoc(t, filepath.Join(repoRoot, filepath.FromSlash(rel)))
}

// bashRule matches a Bash permission rule and captures its command pattern.
var bashRule = regexp.MustCompile(`^Bash\((.*)\)$`)

// launchFree are the only command words a Bash rule of the settings may
// allow. Each was checked not to run a command it is given: coderabbit runs
// a review, and codexreview runs codex only with its fixed arguments. A rule
// for any other word may run codex unprompted (a wrapper such as env, a
// shell, an interpreter, a wildcard that matches one of them), so adding one
// means checking the command and naming it here.
var launchFree = set("coderabbit", "codexreview")

// allowsCodex reports whether a permission rule may let codex run without the
// approval prompt: a Bash rule whose command word is not one of launchFree,
// written as is (a wildcard or a path is another word).
func allowsCodex(rule string) bool {
	if rule != "Bash" && !strings.HasPrefix(rule, "Bash(") {
		return false
	}
	m := bashRule.FindStringSubmatch(rule)
	if m == nil {
		return true
	}
	word, _, _ := strings.Cut(strings.TrimSpace(m[1]), " ")
	word, _, _ = strings.Cut(word, ":")
	_, ok := launchFree[word]
	return !ok
}

// promptingModes are the permission modes that keep the prompt for a Bash
// command no rule allows.
var promptingModes = set("", "default", "plan", "acceptEdits")

// TestSettingsAllowsTool: .claude/settings.json allows the tool, under the
// name go install gives it, without the approval prompt, and allows no rule
// that may run codex itself, nor a mode that skips the prompt: the tool is
// the only way the agents start codex, so its fixed arguments cannot be
// bypassed.
func TestSettingsAllowsTool(t *testing.T) {
	var settings struct {
		Permissions struct {
			Allow       []string `json:"allow"`
			DefaultMode string   `json:"defaultMode"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, ".claude/settings.json")), &settings); err != nil {
		t.Fatalf("parse .claude/settings.json: %v", err)
	}
	want := "Bash(" + toolName(t) + " *)"
	if !slices.Contains(settings.Permissions.Allow, want) {
		t.Errorf("permissions.allow %q does not contain %q", settings.Permissions.Allow, want)
	}
	for _, rule := range settings.Permissions.Allow {
		if allowsCodex(rule) {
			t.Errorf("permissions.allow has %q, which may run codex without the tool", rule)
		}
	}
	if mode := settings.Permissions.DefaultMode; !isSetMember(promptingModes, mode) {
		t.Errorf("permissions.defaultMode is %q, which may run codex without the prompt", mode)
	}
}

func isSetMember(m map[string]struct{}, key string) bool { _, ok := m[key]; return ok }

// TestAllowsCodex: the rules that may let codex run without the tool are
// found, and the two existing Bash rules are not.
func TestAllowsCodex(t *testing.T) {
	for _, tc := range []struct {
		rule string
		want bool
	}{
		{"Bash(codex *)", true},
		{"Bash(codex:*)", true},
		{"Bash(/opt/homebrew/bin/codex *)", true},
		{"Bash(*)", true},
		{"Bash", true},
		{"Bash(codex*)", true},
		{"Bash(e*)", true},
		{"Bash(/bin/b*)", true},
		{"Bash(env *)", true},
		{"Bash(eval *)", true},
		{"Bash(arch *)", true},
		{"Bash(python3 -c *)", true},
		// git runs a command through an alias starting with "!".
		{"Bash(git status*)", true},
		{"Bash(codexreview *)", false},
		{"Bash(coderabbit review *)", false},
		{"Read(codex)", false},
	} {
		if got := allowsCodex(tc.rule); got != tc.want {
			t.Errorf("allowsCodex(%q) = %v, want %v", tc.rule, got, tc.want)
		}
	}
}

// codexreviewProtocolMismatches returns the --protocol flags of doc in a
// codexreview call whose value is not Protocol. The flags of other calls are
// the fixprguard guard's (doctestutil.ProtocolFlags).
func codexreviewProtocolMismatches(doc string) []string {
	var mismatches []string
	for _, f := range doctestutil.ProtocolFlags(doc) {
		if f.Tool == doctestutil.ToolCodexreview && f.Value != strconv.Itoa(codexreview.Protocol) {
			mismatches = append(mismatches, f.Text)
		}
	}
	return mismatches
}

// TestDocumentedProtocolMatches: every --protocol N of a codexreview call in
// the command documents, prose included, is the binary's Protocol, so the
// installed tool refuses a document written for another contract.
func TestDocumentedProtocolMatches(t *testing.T) {
	for _, rel := range doctestutil.CommandDocs(t, repoRoot) {
		for _, text := range codexreviewProtocolMismatches(readRepoFile(t, rel)) {
			t.Errorf("%s: %q, want --protocol %d", rel, text, codexreview.Protocol)
		}
	}
}

// TestCodexreviewProtocolMismatches: a wrong number in a codexreview call is
// found, in a code block or a code span, and a fixprguard call's number is
// left to its own guard.
func TestCodexreviewProtocolMismatches(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	right := strconv.Itoa(codexreview.Protocol)
	wrong := strconv.Itoa(codexreview.Protocol + 1)
	for _, tc := range []struct {
		name string
		doc  string
		want []string
	}{
		{"right_number", "```bash\ncodexreview --protocol " + right + " --base " + sha + "\n```\n", nil},
		{"wrong_number_in_block", "```bash\ncodexreview --protocol " + wrong + " --base " + sha + "\n```\n", []string{"--protocol " + wrong}},
		{"wrong_number_in_span", "Run `codexreview --base <BASE> --protocol=" + wrong + "`.\n", []string{"--protocol=" + wrong}},
		{"fixprguard_call_skipped", "fixprguard head --protocol " + wrong + " && codexreview --protocol " + right + " --base " + sha + "\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := codexreviewProtocolMismatches(tc.doc); !slices.Equal(got, tc.want) {
				t.Errorf("mismatches = %q, want %q", got, tc.want)
			}
		})
	}
}

var (
	// assignment is a variable assignment before a command word.
	assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// redirection is a redirection word, which may come before a command.
	redirection = regexp.MustCompile(`^[0-9]*[<>]`)
	// shellScriptFlag is a shell option cluster that holds -c, such as -lc.
	shellScriptFlag = regexp.MustCompile(`^-[A-Za-z]*c[A-Za-z]*$`)
	// optionValue is a number or a duration, the value an option such as
	// xargs -n or timeout takes before the command.
	optionValue = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?[smhd]?$`)
	// codeSpan matches one inline code span.
	codeSpan = regexp.MustCompile("`([^`\n]+)`")
	// quoteMarker is the blockquote marker of a line inside an agent prompt.
	quoteMarker = regexp.MustCompile(`^\s*(>\s?)+`)
)

var (
	// wrappers run the command named by their first argument that is not an
	// option or an option's value; each maps to its options that take the
	// next word as their value (env -u NAME, xargs -I REPLACE, timeout -k
	// DURATION).
	wrappers = map[string]map[string]struct{}{
		"env": set("-u", "-C", "-S"), "xargs": set("-I", "-n", "-P", "-L", "-s", "-E", "-d"),
		"timeout": set("-k", "-s"), "gtimeout": set("-k", "-s"), "nice": set("-n"),
		"sudo": set("-u", "-g", "-C", "-h", "-p"), "exec": set("-a"),
		"command": nil, "builtin": nil, "time": nil, "nohup": nil, "stdbuf": nil, "caffeinate": nil,
		"npx": nil, "bunx": nil, "pnpx": nil, "pnpm": nil, "dlx": nil,
	}
	// shells run the script given after -c.
	shells = set("sh", "bash", "zsh", "dash")
	// reservedWords may come before a command at a command position.
	reservedWords = set("if", "then", "else", "elif", "while", "until", "do", "!")
)

func set(words ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[w] = struct{}{}
	}
	return m
}

func isWrapper(name string) bool { _, ok := wrappers[name]; return ok }
func isShell(name string) bool   { _, ok := shells[name]; return ok }

// runsCodex reports whether text, a code block line or a command written as
// a code span, has codex at a command position: the start of a command,
// after a separator (doctestutil.CommandBoundary), after the reserved words,
// variable assignments, and redirections that may precede a command, or as
// the command a wrapper or a shell's -c script runs. codex counts by name, by
// path (/opt/homebrew/bin/codex, ./codex, @openai/codex), quoted, escaped
// with a backslash, or opening a group. The list of wrappers and of options
// with a value is not complete; the permission prompt stops a form it misses.
func runsCodex(text string) bool {
	for _, command := range doctestutil.CommandBoundary.Split(text, -1) {
		if commandRunsCodex(strings.Fields(command)) {
			return true
		}
	}
	return false
}

// commandRunsCodex reports whether the command made of words runs codex.
func commandRunsCodex(words []string) bool {
	for i := 0; i < len(words); i++ {
		word := strings.TrimLeft(strings.Trim(words[i], `"'`), `({\`)
		word = strings.TrimRight(strings.Trim(word, `"'`), `)};`)
		name := path.Base(word)
		switch {
		case word == "", isSetMember(reservedWords, word), assignment.MatchString(word), redirection.MatchString(word):
		case isWrapper(name):
			valued := wrappers[name]
			for i+1 < len(words) && (strings.HasPrefix(words[i+1], "-") || optionValue.MatchString(words[i+1])) {
				if isSetMember(valued, words[i+1]) {
					i++
				}
				i++
			}
		case isShell(name):
			j := slices.IndexFunc(words[i+1:], shellScriptFlag.MatchString)
			if j < 0 {
				return false
			}
			script := strings.Join(words[i+2+j:], " ")
			return runsCodex(strings.Trim(script, `"'`))
		default:
			return name == "codex"
		}
	}
	return false
}

// commandTexts returns the texts of doc that are written as commands: each
// line of a fenced code block, and each code span outside the blocks that
// holds an argument after a space. Blockquote markers are dropped first, so
// the blocks of an agent prompt are read too.
func commandTexts(doc string) []string {
	var lines []string
	for line := range strings.Lines(doc) {
		lines = append(lines, quoteMarker.ReplaceAllString(line, ""))
	}
	var texts []string
	for _, line := range doctestutil.ScanDocLines(lines) {
		switch {
		case line.Fence:
		case line.Block:
			texts = append(texts, line.Text)
		default:
			for _, m := range codeSpan.FindAllStringSubmatch(line.Text, -1) {
				if strings.Contains(strings.TrimSpace(m[1]), " ") {
					texts = append(texts, m[1])
				}
			}
		}
	}
	return texts
}

// TestNoDirectCodexInvocation: no command document writes codex as a
// command to run, so every run of Codex CLI goes through the tool and its
// fixed arguments, the only form the permission settings allow. A document
// names it in prose as Codex CLI.
func TestNoDirectCodexInvocation(t *testing.T) {
	for _, rel := range doctestutil.CommandDocs(t, repoRoot) {
		for _, text := range commandTexts(readRepoFile(t, rel)) {
			if runsCodex(text) {
				t.Errorf("%s runs codex directly: %q", rel, text)
			}
		}
	}
}

// TestRunsCodex runs the check on synthesized documents: each form that runs
// codex is found, and each existing use of the word codex in the command
// documents is not.
func TestRunsCodex(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		want bool
	}{
		{"block_line_start", "```bash\ncodex exec review --base x\n```\n", true},
		{"after_pipe", "```bash\necho x | codex exec -\n```\n", true},
		{"after_or", "```bash\ntrue || codex review\n```\n", true},
		{"after_semicolon", "```bash\ncd x; codex review\n```\n", true},
		{"after_and", "```bash\ncd x && codex review\n```\n", true},
		{"background", "```bash\nsleep 1 & codex review\n```\n", true},
		{"substitution", "```bash\nout=$(codex review)\n```\n", true},
		{"backquote", "```bash\nout=`codex review`\n```\n", true},
		{"env", "```bash\nenv FOO=1 codex review\n```\n", true},
		{"env_ignore_environment", "```bash\nenv -i codex review\n```\n", true},
		{"sudo_preserve_env", "```bash\nsudo -E codex review\n```\n", true},
		{"caffeinate", "```bash\ncaffeinate -i codex review\n```\n", true},
		{"subshell_no_argument", "```bash\n(cd x && codex)\n```\n", true},
		{"env_unset", "```bash\nenv -u FOO codex review\n```\n", true},
		{"assignment", "```bash\nFOO=1 codex review\n```\n", true},
		{"redirection", "```bash\n2>/dev/null codex review\n```\n", true},
		{"exec", "```bash\nexec codex review\n```\n", true},
		{"command", "```bash\ncommand codex review\n```\n", true},
		{"xargs", "```bash\nprintf x | xargs -n 1 codex review\n```\n", true},
		{"xargs_replace", "```bash\nprintf x | xargs -I {} codex review {}\n```\n", true},
		{"time", "```bash\ntime codex review\n```\n", true},
		{"timeout", "```bash\ntimeout 600 codex exec review\n```\n", true},
		{"timeout_kill_after", "```bash\ntimeout -k 5s 10m codex exec review\n```\n", true},
		{"nice", "```bash\nnice -n 5 codex review\n```\n", true},
		{"sudo", "```bash\nsudo codex review\n```\n", true},
		{"nohup", "```bash\nnohup codex review\n```\n", true},
		{"if", "```bash\nif codex exec review; then\n```\n", true},
		{"if_not", "```bash\nif ! codex exec review; then\n```\n", true},
		{"then_after_semicolon", "```bash\nif true; then codex review; fi\n```\n", true},
		{"group", "```bash\n{ codex review; }\n```\n", true},
		{"subshell", "```bash\n(codex review)\n```\n", true},
		{"backslash", "```bash\n\\codex review\n```\n", true},
		{"sh_c", "```bash\nsh -c 'codex review'\n```\n", true},
		{"zsh_c", "```bash\nzsh -c 'codex review'\n```\n", true},
		{"bash_lc", "```bash\nbash -lc 'codex review'\n```\n", true},
		// No boundary inside the script, so only the -c case finds it.
		{"bash_c_wrapper", "```bash\nbash -c 'env codex review'\n```\n", true},
		{"absolute_path", "```bash\n/opt/homebrew/bin/codex review\n```\n", true},
		{"relative_path", "```bash\n./codex review\n```\n", true},
		{"quoted", "```bash\n\"codex\" review\n```\n", true},
		{"npx", "```bash\nnpx @openai/codex review\n```\n", true},
		{"npx_option", "```bash\nnpx -y @openai/codex review\n```\n", true},
		{"bunx", "```bash\nbunx @openai/codex review\n```\n", true},
		{"pnpm_dlx", "```bash\npnpm dlx @openai/codex review\n```\n", true},
		{"blockquoted_block", "> ```bash\n> codex review\n> ```\n", true},
		{"code_span", "Run `codex exec review --base <BASE>` once.\n", true},
		{"other_language_block", "```text\ncodex exec review --base <BASE>\n```\n", true},
		// A block line that starts with the quoted word is read as a command,
		// so a jq continuation must not start with it.
		{"quoted_line_start", "```bash\n  \"codex\" as $k\n```\n", true},

		// The uses of the word in the command documents today.
		{"jq_kind", "```bash\n  else {kind: \"codex\", url: .html_url, created_at} end\n```\n", false},
		{"jq_comparison", "```bash\n  | select(.kind == \"codex\")\n```\n", false},
		{"comment_kind_span", "A `codex` is any other comment by the same author.\n", false},
		{"html_marker", "```bash\n  | select((.body // \"\" | split(\"\\n\"))[0] == \"<!-- codex-pull-request-review-summary -->\")\n```\n", false},
		{"html_marker_span", "Its first line is `<!-- codex-pull-request-review-summary -->`.\n", false},
		{"login", "| Awaited PR reviewer | `chatgpt-codex-connector` - the reviewer |\n", false},
		{"mention", "| PR review request | `@codex review` - the body of the comment |\n", false},
		{"tool", "```bash\ncodexreview --protocol 1 --base <BASE>\n```\n", false},
		{"prose_without_span", "Codex CLI runs codex exec review.\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.ContainsFunc(commandTexts(tc.doc), runsCodex)
			if got != tc.want {
				t.Errorf("runs codex = %v, want %v (texts %q)", got, tc.want, commandTexts(tc.doc))
			}
		})
	}
}

// TestInstallTargetDocumented: the Makefile's install target of the tool's
// name installs this package, and the Commands section of CLAUDE.md lists the
// target with the same go install command, so the user finds how to install
// the tool the documents run.
func TestInstallTargetDocumented(t *testing.T) {
	target := "install-" + toolName(t)
	install := "go install ./cmd/" + toolName(t)
	lines := strings.Split(readRepoFile(t, "Makefile"), "\n")
	i := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, target+":") })
	if i < 0 || i+1 >= len(lines) || lines[i+1] != "\t$(GOCMD) install ./cmd/"+toolName(t) {
		t.Errorf("Makefile has no %s target whose recipe is $(GOCMD) install ./cmd/%s", target, toolName(t))
	}
	commands := doctestutil.SectionByHeading(readRepoFile(t, "CLAUDE.md"), "## Commands")
	if !slices.ContainsFunc(strings.Split(commands, "\n"), func(line string) bool {
		return strings.HasPrefix(line, "- `make "+target+"` - ") && strings.Contains(line, "(`"+install+"`)")
	}) {
		t.Errorf("the Commands section of CLAUDE.md has no item for make %s naming (`%s`)", target, install)
	}
}
