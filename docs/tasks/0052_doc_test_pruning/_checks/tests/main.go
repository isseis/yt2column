//go:build test

// Command tests verifies which top-level declarations one pull request of the
// document-test pruning task changes, and that the names it removes are gone.
// It takes two commits, the merge-base with main and the head of the pull
// request:
//
//	go run -tags test ./docs/tasks/0052_doc_test_pruning/_checks/tests/ [-final] <merge-base> <head>
//
// It runs three checks.
//
//  1. Declarations. In every target file (the Go files of the task's scope:
//     the document test files, internal/doctestutil, and
//     internal/config/test_helpers.go), it compares the top-level declarations
//     at the two commits. A declaration is a function, a method (named
//     "Type.Method"), a type, or one name of a var or const spec; imports are
//     not declarations. Two declarations are the same when their gofmt output,
//     doc comment included, is the same, so a const whose alignment changed
//     because a sibling was removed is not reported. A spec inside a
//     parenthesized var or const block also carries the block's doc comment,
//     so editing that comment changes every spec of the block. Each
//     declaration that changed, disappeared, or appeared must be on the change
//     list of its file with that kind of change allowed: a test or helper the
//     architecture document removes may only disappear; a test it partly
//     removes or merges, a test whose input a replacement changes, and
//     cmd/fixprguard's readRepoFile may only change; the renamed test's new
//     name may only appear. Anything else fails, so a kept test cannot be
//     deleted and a removed one cannot be edited instead.
//  2. Removed names. Each removable name (a removed test, a removed or
//     replaced helper, the old name of the renamed test) that the range
//     removed from its file must not remain as an identifier in a Go file
//     outside docs/tasks at the head. A name defined in a package's test code
//     is looked for in that package's directory only; a name exported from
//     internal/doctestutil or internal/config is looked for everywhere, since
//     other packages refer to it. A name a later pull request removes is still
//     at the head of an earlier one, and that is not a failure.
//  3. With -final, every removable name must be absent as an identifier,
//     whether or not this range removed it, so a test the task removes whole
//     and the renamed test's old name cannot survive the last pull request.
//
// The names of check 2 (and of check 3 with -final) that appear in a comment
// or a string literal of a Go file outside docs/tasks, or in another file
// outside docs/tasks (the documents and .claude/commands), are listed for
// reading, not failed: a word such as "numbered" can appear in ordinary prose.
//
// Run it from the repository root, once per pull request. make test and CI do
// not run it, and it is not maintained after the merge. It reads no
// environment variable: the underscore keeps the directory out of go list, and
// the //go:build test first line makes the repository-wide environment access
// check treat the file as test code, so that check does not scan it.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strings"
)

// targetFiles are the Go files of the task's scope besides the files of
// internal/doctestutil, which are listed at each commit.
var targetFiles = []string{
	"cmd/yt2column/docs_test.go",
	"cmd/yt2column/fixpr_docs_test.go",
	"cmd/yt2column/codex_review_docs_test.go",
	"cmd/yt2column/extension_docs_test.go",
	"cmd/yt2column/output_layout_docs_test.go",
	"cmd/yt2column/copied_transcript_docs_test.go",
	"cmd/yt2column/docpaths_test.go",
	"cmd/yt2column/readme_structure_test.go",
	"cmd/yt2column/readme_sync_test.go",
	"cmd/yt2column/run_test.go",
	"cmd/yt2column-server/docs_test.go",
	"cmd/yt2column-nmhost/main_test.go",
	"cmd/fixprguard/docs_test.go",
	"cmd/codexreview/docs_test.go",
	"internal/pipeline/pipeline_test.go",
	"internal/config/test_helpers.go",
}

// doctestutilDir holds the shared document-test helpers; every Go file in it is
// a target file.
const doctestutilDir = "internal/doctestutil"

// changeKind is a kind of change check 1 sees in a declaration.
type changeKind int

const (
	removed changeKind = iota + 1
	changed
	added
)

// String names the kind in the output.
func (k changeKind) String() string {
	switch k {
	case removed:
		return "removed"
	case changed:
		return "changed"
	case added:
		return "added"
	}
	return fmt.Sprintf("changeKind(%d)", int(k))
}

// changeList gives, for each target file, the declarations that may differ
// and the one kind of change each may show, from the architecture document's
// verdict list (3.1), its helper tables (3.4.1 and 3.4.2), and the plan's
// rename ledger.
var changeList = map[string]map[string]changeKind{
	"cmd/yt2column/docs_test.go": {
		"TestSlackDocsContract":             changed,
		"TestProductionProfileDocsContract": changed,
		"TestCostDocsContract":              changed,
		"TestMigrationDocsContract":         removed,
		"checkMigrationSection":             removed,
		"TestQuickStartLinks":               removed,
		"TestProductionProfileLinks":        removed,
		"TestNumberedItems":                 removed,
		"numberedItems":                     removed,
		"numberedItemRE":                    removed,
		"promptsReadmePath":                 removed,
		"reviewEvaluationPath":              removed,
		"pricingRatesPath":                  removed,
	},
	"cmd/yt2column/readme_structure_test.go": {
		"TestReadmeVersionLinks": removed,
		"beforeFirstHeading":     removed,
		"linksTo":                removed,
	},
	"cmd/yt2column/run_test.go": {
		"TestRunEvaluationProcedureFlags": removed,
		"evaluationProcedureCommands":     removed,
		"fencedCode":                      removed,
		"procedureArgs":                   removed,
	},
	"cmd/yt2column/fixpr_docs_test.go": {
		"TestReadmeSyncStepPlacement": removed,
		"readmeSyncIn":                removed,
		"readmeSyncMarker":            removed,
		"mktransPath":                 removed,
		"numbered":                    removed,
	},
	"cmd/yt2column/codex_review_docs_test.go": {
		"TestCodeRabbitConfigStopsReview":  changed,
		"TestCodexMentionOnlyInRequestRow": changed,
		"TestPostingCommandsInventory":     changed,
		"commandDocs":                      removed,
		"commandsDir":                      removed,
	},
	"internal/doctestutil/readme.go": {
		"ReadmeMigrationHeading":     removed,
		"ReadmeQuickStartHeading":    removed,
		"ReadmeRequirementsHeading":  removed,
		"ReadmeConfigurationHeading": removed,
	},
	"internal/config/test_helpers.go": {
		"DeepSeekAPIKeyEnvForTest": removed,
	},
	"cmd/fixprguard/docs_test.go": {
		"TestGuideNamesGitStateParts": changed,
		"readRepoFile":                changed,
	},
	"cmd/codexreview/docs_test.go": {
		"TestInstallTargetDocumented": removed,
		"TestInstallTargetInMakefile": added,
	},
	"internal/pipeline/pipeline_test.go": {
		"TestPackageReferenceListsPackages": changed,
		"TestPromptsREADMEMatchesContract":  changed,
		"TestInterfaceDocComments":          removed,
		"interfaceDoc":                      removed,
		"hasCJK":                            removed,
		"markdownSections":                  removed,
	},
}

// removable is one name the task removes: a test, a helper, or the old name of
// a renamed test.
type removable struct {
	file string
	name string
}

// removables are the names the task removes, in the order they are reported.
var removables = []removable{
	// Tests removed whole (3.1).
	{"cmd/yt2column/docs_test.go", "TestMigrationDocsContract"},
	{"cmd/yt2column/docs_test.go", "TestQuickStartLinks"},
	{"cmd/yt2column/docs_test.go", "TestProductionProfileLinks"},
	{"cmd/yt2column/readme_structure_test.go", "TestReadmeVersionLinks"},
	{"cmd/yt2column/run_test.go", "TestRunEvaluationProcedureFlags"},
	{"cmd/yt2column/fixpr_docs_test.go", "TestReadmeSyncStepPlacement"},
	{"internal/pipeline/pipeline_test.go", "TestInterfaceDocComments"},
	// The renamed test's old name (plan 1.5).
	{"cmd/codexreview/docs_test.go", "TestInstallTargetDocumented"},
	// Helpers that lose their users (3.4.1), with TestNumberedItems.
	{"cmd/yt2column/docs_test.go", "checkMigrationSection"},
	{"cmd/yt2column/docs_test.go", "numberedItems"},
	{"cmd/yt2column/docs_test.go", "numberedItemRE"},
	{"cmd/yt2column/docs_test.go", "TestNumberedItems"},
	{"cmd/yt2column/docs_test.go", "promptsReadmePath"},
	{"cmd/yt2column/docs_test.go", "reviewEvaluationPath"},
	{"cmd/yt2column/docs_test.go", "pricingRatesPath"},
	{"cmd/yt2column/readme_structure_test.go", "beforeFirstHeading"},
	{"cmd/yt2column/readme_structure_test.go", "linksTo"},
	{"cmd/yt2column/run_test.go", "evaluationProcedureCommands"},
	{"cmd/yt2column/run_test.go", "fencedCode"},
	{"cmd/yt2column/run_test.go", "procedureArgs"},
	{"cmd/yt2column/fixpr_docs_test.go", "readmeSyncIn"},
	{"cmd/yt2column/fixpr_docs_test.go", "readmeSyncMarker"},
	{"cmd/yt2column/fixpr_docs_test.go", "mktransPath"},
	{"cmd/yt2column/fixpr_docs_test.go", "numbered"},
	{"internal/doctestutil/readme.go", "ReadmeMigrationHeading"},
	{"internal/doctestutil/readme.go", "ReadmeQuickStartHeading"},
	{"internal/doctestutil/readme.go", "ReadmeRequirementsHeading"},
	{"internal/doctestutil/readme.go", "ReadmeConfigurationHeading"},
	{"internal/config/test_helpers.go", "DeepSeekAPIKeyEnvForTest"},
	{"cmd/yt2column/codex_review_docs_test.go", "commandsDir"},
	{"internal/pipeline/pipeline_test.go", "interfaceDoc"},
	{"internal/pipeline/pipeline_test.go", "hasCJK"},
	// Helpers that a replacement removes (3.4.2).
	{"cmd/yt2column/codex_review_docs_test.go", "commandDocs"},
	{"internal/pipeline/pipeline_test.go", "markdownSections"},
}

// global reports whether other packages can refer to a removable name: an
// exported name of a non-test file of internal/doctestutil or internal/config.
func (r removable) global() bool {
	return ast.IsExported(r.name) && !strings.HasSuffix(r.file, "_test.go") &&
		(strings.HasPrefix(r.file, doctestutilDir+"/") || strings.HasPrefix(r.file, "internal/config/"))
}

func main() {
	final := flag.Bool("final", false, "also require every removable name to be absent")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: tests [-final] <merge-base> <head>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	base := revParse(flag.Arg(0))
	head := revParse(flag.Arg(1))
	fmt.Printf("merge-base   %s\n", base)
	fmt.Printf("head         %s\n", head)
	failed := false

	// Check 1: declarations.
	files := slices.Clone(targetFiles)
	for _, rev := range []string{base, head} {
		for _, f := range listFiles(rev, doctestutilDir) {
			if strings.HasSuffix(f, ".go") && !slices.Contains(files, f) {
				files = append(files, f)
			}
		}
	}
	slices.Sort(files)
	baseDecls := map[string]map[string]string{}
	headDecls := map[string]map[string]string{}
	var removedDecls, changedDecls, addedDecls, wrong []string
	for _, f := range files {
		before := declarations(base, f)
		after := declarations(head, f)
		baseDecls[f], headDecls[f] = before, after
		for _, name := range unionKeys(before, after) {
			b, inBase := before[name]
			a, inHead := after[name]
			var kind changeKind
			switch {
			case !inHead:
				kind = removed
			case !inBase:
				kind = added
			case a != b:
				kind = changed
			default:
				continue
			}
			line := fmt.Sprintf("%s::%s", f, name)
			status := "OK  "
			if allowed, listed := changeList[f][name]; !listed || allowed != kind {
				status = "NG  "
				why := "not on the change list"
				if listed {
					why = "the change list allows only " + allowed.String()
				}
				wrong = append(wrong, fmt.Sprintf("%s (%s; %s)", line, kind, why))
			}
			entry := status + " " + line
			switch kind {
			case removed:
				removedDecls = append(removedDecls, entry)
			case added:
				addedDecls = append(addedDecls, entry)
			default:
				changedDecls = append(changedDecls, entry)
			}
		}
	}
	printList("removed declarations", removedDecls)
	printList("changed declarations", changedDecls)
	printList("added declarations", addedDecls)
	if len(wrong) > 0 {
		failed = true
		fmt.Printf("NG   declarations outside the change list: %d\n", len(wrong))
		for _, w := range wrong {
			fmt.Printf("     %s\n", w)
		}
	} else {
		fmt.Println("OK   every changed declaration is on the change list with its kind of change")
	}

	// Check 2 and 3: removed names.
	sources := headGoSources(head)
	var gone, required []removable
	for _, r := range removables {
		_, inBase := baseDecls[r.file][r.name]
		_, inHead := headDecls[r.file][r.name]
		if inBase && !inHead {
			gone = append(gone, r)
		}
		if *final {
			required = append(required, r)
		}
	}
	fmt.Printf("---- names removed in this range: %d\n", len(gone))
	for _, r := range gone {
		fmt.Printf("     %s::%s\n", r.file, r.name)
	}
	if left := identifierUses(gone, sources); len(left) > 0 {
		failed = true
		fmt.Printf("NG   removed names still used as identifiers: %d\n", len(left))
		for _, l := range left {
			fmt.Printf("     %s\n", l)
		}
	} else {
		fmt.Println("OK   no name removed in this range remains as an identifier")
	}
	if *final {
		if left := identifierUses(required, sources); len(left) > 0 {
			failed = true
			fmt.Printf("NG   removable names still present: %d\n", len(left))
			for _, l := range left {
				fmt.Printf("     %s\n", l)
			}
		} else {
			fmt.Printf("OK   all %d removable names are absent\n", len(required))
		}
	}

	listed := slices.Clone(gone)
	for _, r := range required {
		if !slices.Contains(listed, r) {
			listed = append(listed, r)
		}
	}
	mentions := sourceMentions(listed, sources)
	mentions = append(mentions, documentMentions(head, listed)...)
	printList("mentions in comments and documents, to read", mentions)

	if failed {
		os.Exit(1)
	}
}

// printList prints a titled list.
func printList(title string, lines []string) {
	fmt.Printf("---- %s: %d\n", title, len(lines))
	for _, l := range lines {
		fmt.Printf("%s\n", l)
	}
}

// unionKeys returns the keys of both maps, sorted.
func unionKeys(a, b map[string]string) []string {
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// declarations returns the top-level declarations of the file at rev, keyed by
// name, with the gofmt output of each (doc comment included). A file absent at
// rev has none.
func declarations(rev, file string) map[string]string {
	src, ok := show(rev, file)
	if !ok {
		return map[string]string{}
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		fail("parse "+rev+":"+file, err)
	}
	out := map[string]string{}
	add := func(name string, node ast.Node, prefix string) {
		if name == "_" {
			name = fmt.Sprintf("_#%d", fset.Position(node.Pos()).Line)
		}
		if _, dup := out[name]; dup {
			fail("declarations of "+rev+":"+file, fmt.Errorf("%s declared twice", name))
		}
		out[name] = prefix + format(fset, f, node)
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) == 1 {
				name = receiverName(d.Recv.List[0].Type) + "." + name
			}
			add(name, d, "")
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			for _, spec := range d.Specs {
				// A spec of a declaration without parentheses carries its doc
				// comment on the declaration, so the declaration is the unit.
				// A spec inside parentheses also carries the block's doc
				// comment, which the spec's own output leaves out.
				var node ast.Node = spec
				prefix := ""
				if !d.Lparen.IsValid() {
					node = d
				} else if d.Doc != nil {
					prefix = d.Doc.Text() + "\n"
				}
				switch s := spec.(type) {
				case *ast.TypeSpec:
					add(s.Name.Name, node, prefix)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						add(n.Name, node, prefix)
					}
				}
			}
		}
	}
	return out
}

// receiverName returns the type name of a method receiver.
func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	case *ast.IndexListExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return fmt.Sprintf("%T", expr)
}

// format returns the gofmt output of node with the comments inside it and its
// doc comment.
func format(fset *token.FileSet, f *ast.File, node ast.Node) string {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, &printer.CommentedNode{Node: node, Comments: f.Comments}); err != nil {
		fail("format", err)
	}
	return buf.String()
}

// goSource is one parsed Go file of the head.
type goSource struct {
	path string
	fset *token.FileSet
	file *ast.File
}

// headGoSources parses every Go file of the head outside docs/tasks.
func headGoSources(head string) []goSource {
	var out []goSource
	for _, p := range listFiles(head, ".") {
		if !strings.HasSuffix(p, ".go") || strings.HasPrefix(p, "docs/tasks/") {
			continue
		}
		src, ok := show(head, p)
		if !ok {
			fail("read "+head+":"+p, fmt.Errorf("missing"))
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, src, parser.ParseComments)
		if err != nil {
			fail("parse "+head+":"+p, err)
		}
		out = append(out, goSource{p, fset, f})
	}
	return out
}

// identifierUses returns where each name is used as an identifier in the
// sources it can be referred from.
func identifierUses(names []removable, sources []goSource) []string {
	var out []string
	for _, r := range names {
		for _, s := range sources {
			if !r.global() && path.Dir(s.path) != path.Dir(r.file) {
				continue
			}
			ast.Inspect(s.file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == r.name {
					out = append(out, fmt.Sprintf("%s: %s", s.fset.Position(id.Pos()), r.name))
				}
				return true
			})
		}
	}
	return out
}

// sourceMentions returns the comments and string literals of the Go sources
// that contain a name as a word.
func sourceMentions(names []removable, sources []goSource) []string {
	var out []string
	for _, r := range names {
		word := regexp.MustCompile(`\b` + regexp.QuoteMeta(r.name) + `\b`)
		for _, s := range sources {
			for _, group := range s.file.Comments {
				for _, c := range group.List {
					if word.MatchString(c.Text) {
						out = append(out, fmt.Sprintf("LIST %s: %s: %s", s.fset.Position(c.Pos()), r.name, truncate(c.Text)))
					}
				}
			}
			ast.Inspect(s.file, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && word.MatchString(lit.Value) {
					out = append(out, fmt.Sprintf("LIST %s: %s: %s", s.fset.Position(lit.Pos()), r.name, truncate(lit.Value)))
				}
				return true
			})
		}
	}
	return out
}

// documentMentions returns the lines of the files outside docs/tasks that are
// not Go files and contain a name as a word.
func documentMentions(head string, names []removable) []string {
	var out []string
	for _, r := range names {
		cmd := exec.Command("git", "grep", "-n", "-w", "-F", "-e", r.name, head, "--", ".", ":!docs/tasks/", ":!*.go")
		res, err := cmd.Output()
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
				continue
			}
			fail("git grep "+r.name, err)
		}
		for line := range strings.SplitSeq(strings.TrimSuffix(string(res), "\n"), "\n") {
			out = append(out, fmt.Sprintf("LIST %s: %s", r.name, truncate(strings.TrimPrefix(line, head+":"))))
		}
	}
	return out
}

// truncate shortens a line for a list entry.
func truncate(text string) string {
	if len(text) > 120 {
		return text[:120] + "..."
	}
	return text
}

// listFiles lists the files under dir at rev.
func listFiles(rev, dir string) []string {
	out, err := exec.Command("git", "ls-tree", "-r", "--name-only", rev, "--", dir).Output()
	if err != nil {
		fail("ls-tree "+rev, err)
	}
	var files []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// show returns the content of file at rev, and false when the file is absent.
func show(rev, file string) ([]byte, bool) {
	if exec.Command("git", "cat-file", "-e", rev+":"+file).Run() != nil {
		return nil, false
	}
	out, err := exec.Command("git", "show", rev+":"+file).Output()
	if err != nil {
		fail("show "+rev+":"+file, err)
	}
	return out, true
}

// revParse resolves a commit.
func revParse(rev string) string {
	out, err := exec.Command("git", "rev-parse", "--verify", rev+"^{commit}").Output()
	if err != nil {
		fail("rev-parse "+rev, err)
	}
	return strings.TrimSpace(string(out))
}

// fail prints an error that stops every check and exits non-zero.
func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "tests: %s: %v\n", what, err)
	os.Exit(1)
}
