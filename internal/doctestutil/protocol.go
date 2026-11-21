//go:build test

package doctestutil

import (
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The tools whose calls the command documents give a --protocol N. The
// guard of each checks the numbers that ProtocolFlags assigns to it.
const (
	ToolFixprguard = "fixprguard"
	ToolChief      = "chief"
	ToolMergepr    = "mergepr"
)

var protocolTools = []string{ToolFixprguard, ToolChief, ToolMergepr}

// ProtocolTools returns a copy of the tools whose --protocol flags the guards
// check, so a test can compare the list with the tools the protocol-number
// check finds.
func ProtocolTools() []string { return slices.Clone(protocolTools) }

// ProtocolFlag is one --protocol N, or --protocol=N, that a document writes.
type ProtocolFlag struct {
	// Text is the flag as written, such as "--protocol 4".
	Text string
	// Value is N: the whole word after the flag, so a word such as "1junk",
	// which the tool rejects, is not read as 1.
	Value string
	// Tool is the tool whose call the flag belongs to: the nearest of the
	// tools of ProtocolTools written as a word before it in the same command,
	// or "" when none is.
	Tool string
}

var (
	// protocolFlag matches the spellings the documents use. The value is a
	// word that starts with a digit, which leaves out placeholders such as
	// "N", and runs to the next space, so a guard compares all of it.
	protocolFlag = regexp.MustCompile(`--protocol[ =]([0-9]\S*)`)
	// CommandBoundary ends a command and starts the next: a line break, a
	// pipe, a list operator, a command substitution, or a backquote, which in
	// prose also closes a code span. The protocol and command invocation
	// guards split commands with it.
	CommandBoundary = regexp.MustCompile("[\n|;&`]|\\$\\(")
)

// ProtocolFlags returns every --protocol flag of doc, prose included, in
// order, each with the tool it belongs to. A flag outside any command of a
// known tool has Tool "": the fixprguard guard checks it, as it checked every
// flag before the other tools had calls, so each flag is checked by exactly
// one guard. A command substitution between the tool and its flag ends the
// command, so the documents write a call's values literally.
func ProtocolFlags(doc string) []ProtocolFlag {
	return protocolFlagsOf(doc, protocolTools)
}

// ProtocolCallers returns, for each --protocol flag of doc, the one of names
// written last as a word before it in the same command, read as ProtocolFlags
// reads a flag's tool. A flag with none of names before it gives nothing, so
// the result names only the commands that pass --protocol.
func ProtocolCallers(doc string, names []string) []string {
	var callers []string
	for _, f := range protocolFlagsOf(doc, names) {
		if f.Tool != "" {
			callers = append(callers, f.Tool)
		}
	}
	return callers
}

// protocolFlagsOf returns every --protocol flag of doc with the one of names
// written last before it in its command as its Tool.
func protocolFlagsOf(doc string, names []string) []ProtocolFlag {
	var flags []ProtocolFlag
	for _, command := range CommandBoundary.Split(doc, -1) {
		for _, m := range protocolFlag.FindAllStringSubmatchIndex(command, -1) {
			flags = append(flags, ProtocolFlag{
				Text:  command[m[0]:m[1]],
				Value: command[m[2]:m[3]],
				Tool:  lastTool(command[:m[0]], names),
			})
		}
	}
	return flags
}

// lastTool returns the one of names written last as a word in text, or "".
// A word is the name alone or a path ending in it, with optional quotes:
// "mergepr-x", "my_fixprguard", or "chiefs" is not one.
func lastTool(text string, names []string) string {
	for _, word := range slices.Backward(strings.Fields(text)) {
		word = path.Base(strings.Trim(word, `"'`))
		if slices.Contains(names, word) {
			return word
		}
	}
	return ""
}

// CommandDocs returns the paths, relative to root and with forward slashes,
// of every Markdown file under root's .claude/commands, failing t when there
// is none.
func CommandDocs(t testing.TB, root string) []string {
	t.Helper()
	var docs []string
	err := filepath.WalkDir(filepath.Join(root, ".claude", "commands"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			rel, relErr := filepath.Rel(root, p)
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
