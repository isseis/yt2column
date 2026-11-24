//go:build test

package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/mergepr"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

// guidePath is the mergepr guide, relative to repoRoot.
const guidePath = "docs/dev/guides/mergepr_guide.md"

// toolWord is the name a document calls the tool by.
const toolWord = doctestutil.ToolMergepr

// mergemainDocPath is the /mergemain command definition, relative to repoRoot.
const mergemainDocPath = ".claude/commands/mergemain.md"

// settingsPath is the shared Claude Code settings file, relative to repoRoot.
const settingsPath = ".claude/settings.json"

// definitionCalls are the subcommands each command definition that drives a
// merge or a take-in must call, so a definition that lost a call is not read
// as agreeing because the calls left pass. runpr.md merges through mergepr.md
// step 4, so it calls merge only through that definition.
var definitionCalls = map[string][]string{
	".claude/commands/mergepr.md": {"prepare", "paragraph", "check-body", "merge", "cleanup", "discard"},
	".claude/commands/runpr.md":   {"prepare", "paragraph", "check-body", "cleanup", "discard"},
	mergemainDocPath:              {"mergemain-begin", "mergemain-merge", "mergemain-restore", "mergemain-push"},
}

// mergemainPromptFreeSubcommands are the take-in subcommands allow-listed in
// .claude/settings.json: every documented call in mergemain.md must be a bare,
// standalone command so Claude Code runs it without a permission prompt.
var mergemainPromptFreeSubcommands = map[string]struct{}{
	"mergemain-begin": {},
	"mergemain-merge": {},
	"mergemain-push":  {},
}

// nonCommandBlock is a fenced block that holds no command to run: a Mermaid
// diagram, whose labels name the subcommands, or a text block, which holds
// output examples and examples of the /mergepr slash command.
var nonCommandBlock = regexp.MustCompile("(?ms)^```(?:mermaid|text)\n.*?^```\n")

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	return doctestutil.ReadDoc(t, filepath.Join(repoRoot, filepath.FromSlash(rel)))
}

// wantProtocolFlag is the one --protocol form the documents write.
func wantProtocolFlag() string { return "--protocol " + strconv.Itoa(mergepr.Protocol) }

// TestDocumentedProtocolMatches: every --protocol N of a mergepr call in the
// command documents and the guide, and every --protocol N the guide writes in
// prose, is this binary's Protocol. TestDocumentedInvocationsParse also
// refuses a wrong value in a call it parses; this test alone covers the prose
// flags and the blocks that test skips.
//
// In detail: every --protocol N of a mergepr call in the
// command documents and the guide, prose included, is the binary's Protocol
// in the form the tool reads, so the installed tool refuses a definition
// written for another contract and accepts one written for its own. The
// guide must write at least one, so a guide that stops writing them is not
// read as agreeing.
func TestDocumentedProtocolMatches(t *testing.T) {
	docs := append(doctestutil.CommandDocs(t, repoRoot), guidePath)
	guideFlags := 0
	for _, rel := range docs {
		for _, f := range doctestutil.ProtocolFlags(readRepoFile(t, rel)) {
			// In the guide, a flag written with no tool before it, as in its
			// prose, is mergepr's too: the guide is mergepr's alone.
			mine := f.Tool == doctestutil.ToolMergepr || (rel == guidePath && f.Tool == "")
			if !mine {
				continue
			}
			if rel == guidePath {
				guideFlags++
			}
			if f.Text != wantProtocolFlag() {
				t.Errorf("%s: %q, want %q", rel, f.Text, wantProtocolFlag())
			}
		}
	}
	if guideFlags == 0 {
		t.Errorf("%s writes no %s call with --protocol", guidePath, toolWord)
	}
}

// placeholderValues are the values the documents' placeholders stand for.
var placeholderValues = map[string]string{
	"<state-file>":   "/repo/.git/mergepr-1/state.json",
	"<subject-file>": "/repo/.git/mergepr-1/subject.txt",
	"<body-file>":    "/repo/.git/mergepr-1/body.txt",
	// <drop> and <rejected-drop> stand for one value each, so a call writes
	// each at most once: the same value twice is a duplicate drop.
	"<drop>":          "1:disputed",
	"<rejected-drop>": "0123456789abcdef0123456789abcdef01234567:3",
	"<number>":        "42",
	"<PR>":            "42",
	"$ARGUMENTS":      "42",
	"FILE":            "/repo/.git/mergepr-1/state.json",
	"[PR]":            "42",
	// The take-in subcommands take the three values mergemain-begin prints.
	"<head-branch>": "feature/foo",
	"<base-head>":   "2222222222222222222222222222222222222222",
	"<main-head>":   "4444444444444444444444444444444444444444",
}

// placeholder is a value a document leaves to be filled in.
var placeholder = regexp.MustCompile(`<[A-Za-z_-]+>|\$ARGUMENTS|\[PR\]|\bFILE\b`)

// fillPlaceholders returns args with each placeholder replaced by the value
// it stands for, failing t for a placeholder with no value.
func fillPlaceholders(t *testing.T, args []string) []string {
	t.Helper()
	filled := slices.Clone(args)
	for i := range filled {
		filled[i] = placeholder.ReplaceAllStringFunc(filled[i], func(p string) string {
			value, ok := placeholderValues[p]
			if !ok {
				t.Errorf("%q: no value for the placeholder %s", args, p)
			}
			return value
		})
	}
	return filled
}

// TestDocumentedInvocationsParse: every mergepr call the command documents
// and the guide write passes the tool's own argument check with its
// placeholders filled in, so the installed tool runs it instead of returning
// a usage error. Each command definition that merges must call every
// subcommand it runs itself.
func TestDocumentedInvocationsParse(t *testing.T) {
	docs := append(doctestutil.CommandDocs(t, repoRoot), guidePath)
	called := map[string]map[string]struct{}{}
	for _, rel := range docs {
		doc := nonCommandBlock.ReplaceAllString(readRepoFile(t, rel), "")
		for _, call := range doctestutil.Invocations(doc, toolWord) {
			inv, err := parse(fillPlaceholders(t, call.Args))
			if err != nil {
				t.Errorf("%s: %s %q is a usage error: %v", rel, toolWord, call.Args, err)
				continue
			}
			if rel == mergemainDocPath {
				if _, mustBePromptFree := mergemainPromptFreeSubcommands[inv.subcommand]; mustBePromptFree && !doctestutil.PromptFree(call, toolWord) {
					t.Errorf("%s: %s %s call %q %s %q is not a bare standalone command allowed without prompt",
						rel, toolWord, inv.subcommand, call.Lead, call.Word, call.Args)
				}
			}
			if called[rel] == nil {
				called[rel] = map[string]struct{}{}
			}
			called[rel][inv.subcommand] = struct{}{}
		}
	}
	for rel, subs := range definitionCalls {
		for _, sub := range subs {
			if _, ok := called[rel][sub]; !ok {
				t.Errorf("%s writes no call of %s %s", rel, toolWord, sub)
			}
		}
	}
}

// cmdNames returns the names of the directories under cmd/, the commands a
// document can call with --protocol.
func cmdNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot, "cmd"))
	if err != nil {
		t.Fatalf("read cmd: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// TestProtocolToolsAgree: the tools the protocol-number check finds in the
// work tree, the commands the command documents call with --protocol, and
// the tools whose --protocol flags the document guards check are the same
// set, and the work tree has no unreadable Protocol declaration. A tool the
// merge does not check (a declaration moved or turned into a var), or one
// whose --protocol no guard checks, fails here.
func TestProtocolToolsAgree(t *testing.T) {
	found, unreadable, err := mergepr.WorktreeProtocolTools(repoRoot)
	if err != nil {
		t.Fatalf("WorktreeProtocolTools: %v", err)
	}
	if len(unreadable) != 0 {
		t.Errorf("tools with an unreadable Protocol declaration: %v", unreadable)
	}
	names := cmdNames(t)
	calledSet := map[string]struct{}{}
	for _, rel := range doctestutil.CommandDocs(t, repoRoot) {
		for _, name := range doctestutil.ProtocolCallers(readRepoFile(t, rel), names) {
			calledSet[name] = struct{}{}
		}
	}
	called := slices.Sorted(maps.Keys(calledSet))
	guarded := slices.Sorted(slices.Values(doctestutil.ProtocolTools()))
	if !slices.Equal(found, called) || !slices.Equal(found, guarded) {
		t.Errorf("tools with a Protocol constant %v, commands the documents call with --protocol %v, and tools the document guards check %v differ", found, called, guarded)
	}
}

// mergemainAllowedRules are the only Bash allow rules that may name mergepr:
// the three take-in subcommands that perform deterministic verification and
// never destroy uncommitted local work. mergemain-restore, prepare, and merge
// must stay outside allow so they prompt for approval.
var mergemainAllowedRules = []string{
	"Bash(" + toolWord + " mergemain-begin *)",
	"Bash(" + toolWord + " mergemain-merge *)",
	"Bash(" + toolWord + " mergemain-push *)",
}

// TestSettingsAllowsOnlySafeMergemainSubcommands: .claude/settings.json allows
// mergepr in exactly the three safe mergemain-* forms, so mergemain-restore,
// prepare, merge, and wider wildcards still prompt for human approval.
func TestSettingsAllowsOnlySafeMergemainSubcommands(t *testing.T) {
	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, settingsPath)), &settings); err != nil {
		t.Fatalf("parse %s: %v", settingsPath, err)
	}
	extra, missing := doctestutil.BashToolRuleMismatches(settings.Permissions.Allow, toolWord, mergemainAllowedRules)
	for _, rule := range extra {
		t.Errorf("%s permissions.allow has %q; only %q may name %s", settingsPath, rule, mergemainAllowedRules, toolWord)
	}
	for _, rule := range missing {
		t.Errorf("%s permissions.allow lacks %q", settingsPath, rule)
	}
}

// nonCommandInfo lists fenced-block info strings that hold diagrams, examples,
// or prose rather than executable shell commands. Any other info string (bash,
// sh, zsh, shell, or empty) is scanned as executable commands.
var nonCommandInfo = map[string]struct{}{
	"mermaid":  {},
	"text":     {},
	"markdown": {},
	"md":       {},
}

// forbiddenGitMutations are the git subcommands mergemain.md must never invoke
// directly in an executable command block: taking in, restoring, cleaning,
// rebasing, and pushing must go through mergepr's take-in subcommands or stay
// prohibited.
var forbiddenGitMutations = map[string]struct{}{
	"merge":  {},
	"reset":  {},
	"push":   {},
	"clean":  {},
	"rebase": {},
}

// gitValuedGlobalOpts are git global options that take the next word as their
// argument when written without "=".
var gitValuedGlobalOpts = map[string]struct{}{
	"-C":             {},
	"-c":             {},
	"--git-dir":      {},
	"--work-tree":    {},
	"--namespace":    {},
	"--config-env":   {},
	"--super-prefix": {},
	"--attr-source":  {},
}

// stripWordDecorations removes grouping punctuation, backslash escapes, and
// enclosing shell quotes from a command word.
func stripWordDecorations(w string) string {
	w = strings.TrimLeft(w, `({\`)
	w = strings.TrimRight(w, `)};`)
	return strings.Trim(w, `"'`)
}

// joinContinuations joins lines ending in a backslash into single logical
// command lines and drops comment lines.
func joinContinuations(lines []string) []string {
	var logical []string
	var cur strings.Builder
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if cur.Len() == 0 && (line == "" || strings.HasPrefix(line, "#")) {
			continue
		}
		if cont, ok := strings.CutSuffix(line, `\`); ok {
			cur.WriteString(cont)
			cur.WriteByte(' ')
			continue
		}
		cur.WriteString(line)
		logical = append(logical, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		logical = append(logical, cur.String())
	}
	return logical
}

// gitSubcommandAfterOptions skips git global options and returns the first
// non-option word (the git subcommand), or "".
func gitSubcommandAfterOptions(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := strings.Trim(args[i], `"'`)
		if hasKey(gitValuedGlobalOpts, arg) {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return stripWordDecorations(arg)
	}
	return ""
}

func hasKey(m map[string]struct{}, k string) bool { _, ok := m[k]; return ok }

// directGitMutations scans every executable fenced code block of doc and
// returns each line that directly runs a forbidden git mutation (skipping git
// global options and unwrapping wrappers, env -S, redirections, and shell -c
// via doctestutil). An unclosed fenced code block is reported as a violation so
// the check fails closed.
func directGitMutations(doc string) []string {
	var hits []string
	for _, block := range doctestutil.FencedCommandBlocks(doc) {
		if !block.Closed {
			hits = append(hits, "unclosed fenced code block")
			continue
		}
		info, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(block.Info)), " ")
		if hasKey(nonCommandInfo, info) {
			continue
		}
		for _, line := range joinContinuations(block.Lines) {
			for _, args := range doctestutil.CommandArgs(line, "git") {
				if sub := gitSubcommandAfterOptions(args); hasKey(forbiddenGitMutations, sub) {
					hits = append(hits, sub+": "+line)
				}
			}
		}
	}
	return hits
}

// TestDirectGitMutations: the guard detects direct forbidden git mutations in
// executable code blocks (including with global options, wrappers, blockquoted
// fences, paths, continuations, shell -c, decorated subcommands, and unclosed
// fences) and does not false-positive on git commit, git diff, prose code
// spans, or non-command text blocks.
func TestDirectGitMutations(t *testing.T) {
	for _, tc := range []struct {
		name     string
		doc      string
		wantSubs []string
	}{
		{
			name: "allowed_commands_and_prose",
			doc: "Never run `git merge`, `git reset`, `git clean`, `git rebase`, or `git push` directly.\n\n" +
				"```bash\n" +
				"mergepr mergemain-merge --protocol 4 --head-branch b --base-head a --main m 1\n" +
				"git commit --no-edit\n" +
				"git diff --name-only a HEAD\n" +
				"git add README.md\n" +
				"```\n\n" +
				"```text\ngit push origin branch\n```\n",
		},
		{
			name:     "direct_merge",
			doc:      "```bash\ngit merge origin/main\n```\n",
			wantSubs: []string{"merge"},
		},
		{
			name:     "direct_reset_with_global_opts",
			doc:      "```sh\ngit -C /repo -c core.fsmonitor=false --git-dir /repo/.git --no-pager reset --hard HEAD\n```\n",
			wantSubs: []string{"reset"},
		},
		{
			name:     "direct_push_attached_opts_and_path",
			doc:      "```\nFOO=1 /usr/bin/git -C/repo --work-tree=/w push origin branch\n```\n",
			wantSubs: []string{"push"},
		},
		{
			name:     "direct_clean_and_rebase",
			doc:      "```bash\ngit clean -fd\ngit rebase origin/main\n```\n",
			wantSubs: []string{"clean", "rebase"},
		},
		{
			name:     "wrapped_and_chained",
			doc:      "```bash\necho ok && env -u GIT_DIR git --attr-source HEAD merge origin/main\n```\n",
			wantSubs: []string{"merge"},
		},
		{
			name:     "blockquoted_fence_and_redirection",
			doc:      "> ```bash\n> > /tmp/log git push origin branch\n> ```\n",
			wantSubs: []string{"push"},
		},
		{
			name:     "shell_c_chained_after_allowed_git_and_continuation",
			doc:      "```bash\nbash -c \"git commit --no-edit && git \\\n  -C /repo \\\n  push origin branch\"\n```\n",
			wantSubs: []string{"push"},
		},
		{
			name:     "decorated_subcommand",
			doc:      "```bash\ngit \\reset --hard\n(git \"push\")\ngit \"-C\" /repo merge origin/main\n```\n",
			wantSubs: []string{"reset", "push", "merge"},
		},
		{
			name:     "unclosed_fence_fails_closed",
			doc:      "```bash\ngit commit --no-edit\n",
			wantSubs: []string{"unclosed fenced code block"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := directGitMutations(tc.doc)
			if len(got) != len(tc.wantSubs) {
				t.Fatalf("directGitMutations = %q (%d hits), want %d (%q)", got, len(got), len(tc.wantSubs), tc.wantSubs)
			}
			for i, want := range tc.wantSubs {
				if want == "unclosed fenced code block" {
					if got[i] != want {
						t.Errorf("hit[%d] = %q, want %q", i, got[i], want)
					}
					continue
				}
				if !strings.HasPrefix(got[i], want+": ") {
					t.Errorf("hit[%d] = %q, want prefix %q", i, got[i], want+": ")
				}
			}
		})
	}
}

// TestMergemainNoDirectGitMutation: .claude/commands/mergemain.md never invokes
// git merge, git reset, git clean, git rebase, or git push directly in any
// executable command block; all take-in, restore, and push operations go
// through mergepr.
func TestMergemainNoDirectGitMutation(t *testing.T) {
	doc := readRepoFile(t, mergemainDocPath)
	for _, hit := range directGitMutations(doc) {
		t.Errorf("%s directly invokes forbidden git mutation: %s", mergemainDocPath, hit)
	}
}
