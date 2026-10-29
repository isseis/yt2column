//go:build test

package main

import (
	"encoding/json"
	"io/fs"
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

// commandDocs returns the repository-relative paths of every command document
// under .claude/commands.
func commandDocs(t *testing.T) []string {
	t.Helper()
	var docs []string
	err := filepath.WalkDir(filepath.Join(repoRoot, ".claude", "commands"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			rel, relErr := filepath.Rel(repoRoot, p)
			docs = append(docs, filepath.ToSlash(rel))
			return relErr
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk command documents: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("no command documents found")
	}
	return docs
}

// bashRule matches a Bash permission rule and captures its command pattern.
var bashRule = regexp.MustCompile(`^Bash\((.*)\)$`)

// allowsCodex reports whether a permission rule lets the codex command run
// without the approval prompt: a rule for all of Bash, or one whose command
// word is codex, by name or by path, in the "codex *" or "codex:*" form.
func allowsCodex(rule string) bool {
	if rule == "Bash" {
		return true
	}
	m := bashRule.FindStringSubmatch(rule)
	if m == nil {
		return false
	}
	word, _, _ := strings.Cut(strings.TrimSpace(m[1]), " ")
	word, _, _ = strings.Cut(word, ":")
	return word == "*" || path.Base(word) == "codex"
}

// TestSettingsAllowsTool: .claude/settings.json allows the tool, under the
// name go install gives it, without the approval prompt, and allows no rule
// that runs codex itself: the tool is the only way the agents start codex,
// so its fixed arguments cannot be bypassed.
func TestSettingsAllowsTool(t *testing.T) {
	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
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
			t.Errorf("permissions.allow has %q, which runs codex without the tool", rule)
		}
	}
}

// TestAllowsCodex: the rules that would let codex run without the tool are
// found, and the tool's own rule and the other existing rule are not.
func TestAllowsCodex(t *testing.T) {
	for _, tc := range []struct {
		rule string
		want bool
	}{
		{"Bash(codex *)", true},
		{"Bash(codex:*)", true},
		{"Bash(codex exec review *)", true},
		{"Bash(/opt/homebrew/bin/codex *)", true},
		{"Bash(*)", true},
		{"Bash", true},
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
	for _, rel := range commandDocs(t) {
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
	// commandSeparator ends a command and starts the next at a command
	// position: a line break, a pipe, a list operator, a command
	// substitution, or a backquote.
	commandSeparator = regexp.MustCompile("[\n|;&`]|\\$\\(")
	// assignment is a variable assignment before a command word.
	assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// codeSpan matches one inline code span.
	codeSpan = regexp.MustCompile("`([^`\n]+)`")
	// quoteMarker is the blockquote marker of a line inside an agent prompt.
	quoteMarker = regexp.MustCompile(`^\s*(>\s?)+`)
)

// wrappers run the command named by their first argument that is not an
// option; shells run the script given after -c.
var (
	wrappers = map[string]struct{}{"env": {}, "exec": {}, "command": {}, "xargs": {}, "time": {}, "nohup": {}, "npx": {}}
	shells   = map[string]struct{}{"sh": {}, "bash": {}, "zsh": {}}
)

// runsCodex reports whether text, a code block line or a command written as
// a code span, has codex at a command position: the start of a command, after
// a separator, after the variable assignments that precede a command, or as
// the command a wrapper or a shell's -c script runs. codex counts by name, by
// path (/opt/homebrew/bin/codex, ./codex, @openai/codex), and quoted.
func runsCodex(text string) bool {
	for _, command := range commandSeparator.Split(text, -1) {
		if commandRunsCodex(strings.Fields(command)) {
			return true
		}
	}
	return false
}

// commandRunsCodex reports whether the command made of words runs codex.
func commandRunsCodex(words []string) bool {
	for i := 0; i < len(words); i++ {
		word := strings.Trim(words[i], `"'`)
		name := path.Base(word)
		switch {
		case assignment.MatchString(word):
		case isWrapper(name):
			for i+1 < len(words) && isOption(words[i+1]) {
				i++
			}
		case isShell(name):
			j := slices.Index(words[i+1:], "-c")
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

func isWrapper(name string) bool { _, ok := wrappers[name]; return ok }
func isShell(name string) bool   { _, ok := shells[name]; return ok }

// isOption reports whether word is an option, or the number an option such as
// xargs -n takes, which a wrapper skips before its command.
func isOption(word string) bool {
	if strings.HasPrefix(word, "-") {
		return true
	}
	_, err := strconv.Atoi(word)
	return err == nil
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
// fixed arguments, the only form the permission settings allow.
func TestNoDirectCodexInvocation(t *testing.T) {
	for _, rel := range commandDocs(t) {
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
		{"assignment", "```bash\nFOO=1 codex review\n```\n", true},
		{"exec", "```bash\nexec codex review\n```\n", true},
		{"command", "```bash\ncommand codex review\n```\n", true},
		{"xargs", "```bash\nprintf x | xargs -n 1 codex review\n```\n", true},
		{"time", "```bash\ntime codex review\n```\n", true},
		{"nohup", "```bash\nnohup codex review\n```\n", true},
		{"sh_c", "```bash\nsh -c 'codex review'\n```\n", true},
		{"bash_c_later_command", "```bash\nbash -c \"cd x && codex review\"\n```\n", true},
		{"absolute_path", "```bash\n/opt/homebrew/bin/codex review\n```\n", true},
		{"relative_path", "```bash\n./codex review\n```\n", true},
		{"quoted", "```bash\n\"codex\" review\n```\n", true},
		{"npx", "```bash\nnpx @openai/codex review\n```\n", true},
		{"npx_option", "```bash\nnpx -y @openai/codex review\n```\n", true},
		{"blockquoted_block", "> ```bash\n> codex review\n> ```\n", true},
		{"code_span", "Run `codex exec review --base <BASE>` once.\n", true},
		{"other_language_block", "```text\ncodex exec review --base <BASE>\n```\n", true},

		// The uses of the word in the command documents today.
		{"jq_kind", "```bash\n  else {kind: \"codex\", url: .html_url, created_at} end\n```\n", false},
		{"jq_comparison", "```bash\n  | select(.kind == \"codex\")\n```\n", false},
		{"comment_kind_span", "A `codex` is any other comment by the same author.\n", false},
		{"html_marker", "```bash\n  | select((.body // \"\" | split(\"\\n\"))[0] == \"<!-- codex-pull-request-review-summary -->\")\n```\n", false},
		{"html_marker_span", "Its first line is `<!-- codex-pull-request-review-summary -->`.\n", false},
		{"login", "| Awaited PR reviewer | `chatgpt-codex-connector` — the reviewer |\n", false},
		{"mention", "| PR review request | `@codex review` — the body of the comment |\n", false},
		{"tool", "```bash\ncodexreview --protocol 1 --base <BASE>\n```\n", false},
		{"sh_script_file", "```bash\nsh codex.sh\n```\n", false},
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

// TestInstallTargetDocumented: the Makefile has the install target of the
// tool's name, and the Commands section of CLAUDE.md lists it, so the user
// finds how to install the tool the documents run.
func TestInstallTargetDocumented(t *testing.T) {
	target := "install-" + toolName(t)
	makefile := readRepoFile(t, "Makefile")
	if !slices.ContainsFunc(strings.Split(makefile, "\n"), func(line string) bool { return strings.HasPrefix(line, target+":") }) {
		t.Errorf("Makefile has no %s target", target)
	}
	commands := doctestutil.SectionByHeading(readRepoFile(t, "CLAUDE.md"), "## Commands")
	if !slices.ContainsFunc(strings.Split(commands, "\n"), func(line string) bool { return strings.HasPrefix(line, "- `make "+target+"` - ") }) {
		t.Errorf("the Commands section of CLAUDE.md has no item for make %s", target)
	}
}
