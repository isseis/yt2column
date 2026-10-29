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
	ToolFixprguard  = "fixprguard"
	ToolCodexreview = "codexreview"
)

var protocolTools = []string{ToolFixprguard, ToolCodexreview}

// ProtocolFlag is one --protocol N, or --protocol=N, that a document writes.
type ProtocolFlag struct {
	// Text is the flag as written, such as "--protocol 4".
	Text string
	// Value is N.
	Value string
	// Tool is the tool whose call the flag belongs to: the nearest of
	// ToolFixprguard and ToolCodexreview written as a word before it in the
	// same command, or "" when neither is.
	Tool string
}

var (
	// protocolFlag matches the spellings the documents use.
	protocolFlag = regexp.MustCompile("--protocol[ =]([0-9]+)")
	// CommandBoundary ends a command and starts the next: a line break, a
	// pipe, a list operator, a command substitution, or a backquote, which in
	// prose also closes a code span. The protocol guards and the codex
	// invocation guard split commands with it.
	CommandBoundary = regexp.MustCompile("[\n|;&`]|\\$\\(")
)

// ProtocolFlags returns every --protocol flag of doc, prose included, in
// order, each with the tool it belongs to. A flag outside any command of a
// known tool has Tool "": the fixprguard guard checks it, as it checked every
// flag before codexreview had calls, so each flag is checked by exactly one
// guard. A command substitution between the tool and its flag ends the
// command, so the documents write a call's values literally.
func ProtocolFlags(doc string) []ProtocolFlag {
	var flags []ProtocolFlag
	for _, command := range CommandBoundary.Split(doc, -1) {
		for _, m := range protocolFlag.FindAllStringSubmatchIndex(command, -1) {
			flags = append(flags, ProtocolFlag{
				Text:  command[m[0]:m[1]],
				Value: command[m[2]:m[3]],
				Tool:  lastTool(command[:m[0]]),
			})
		}
	}
	return flags
}

// lastTool returns the known tool written last as a word in text, or "".
// A word is the tool's name alone or a path ending in it, with optional
// quotes: "codexreview-x" or "my_fixprguard" is not one.
func lastTool(text string) string {
	for _, word := range slices.Backward(strings.Fields(text)) {
		word = path.Base(strings.Trim(word, `"'`))
		if slices.Contains(protocolTools, word) {
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
