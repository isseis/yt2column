//go:build test

package main

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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

// definitionCalls are the subcommands each command definition that drives a
// merge must call, so a definition that lost a call is not read as agreeing
// because the calls left pass. runpr.md merges through mergepr.md step 4, so
// it calls merge only through that definition.
var definitionCalls = map[string][]string{
	".claude/commands/mergepr.md": {"prepare", "merge", "cleanup", "discard"},
	".claude/commands/runpr.md":   {"prepare", "cleanup", "discard"},
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
	"<number>":       "42",
	"<PR>":           "42",
	"$ARGUMENTS":     "42",
	"FILE":           "/repo/.git/mergepr-1/state.json",
	"[PR]":           "42",
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
