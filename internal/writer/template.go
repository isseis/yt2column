package writer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"syscall"
	"text/template"
	"text/template/parse"
	"unicode/utf8"
)

// maxTemplateBytes caps a prompt template, embedded default or override file.
const maxTemplateBytes = 65536 // 64 KiB

// Template names. text/template reports them in its own error messages, so a
// failure names the template it came from.
const (
	systemTemplateName = "system"
	userTemplateName   = "user"
)

// probeNameSuffix names the second parse that detects a define with the
// template's own name (see checkNoDefinitions).
const probeNameSuffix = ".probe"

// overrideOpenFlags opens an override file read-only without blocking on a
// FIFO that has no writer (O_NONBLOCK) and without making a terminal the
// controlling terminal (O_NOCTTY). The type of the opened file is checked
// right after, so neither flag is relied on for anything but the open itself.
const overrideOpenFlags = os.O_RDONLY | syscall.O_NONBLOCK | syscall.O_NOCTTY

// variadic marks a function with no upper bound on its argument count.
const variadic = -1

// funcArity is the number of arguments a function accepts, counting a value
// piped into it. max is variadic when there is no upper bound.
type funcArity struct {
	min, max int
}

// allowedFuncs is the set of text/template builtin functions a template may
// call, with the argument counts their signatures in text/template/funcs.go
// accept (eq takes one or more there but fails at run time with only one).
// Each returns a bool, an int, a single byte, or one of its arguments, so no
// call can produce a string larger than its input.
var allowedFuncs = map[string]funcArity{
	"and":   {1, variadic},
	"or":    {1, variadic},
	"not":   {1, 1},
	"eq":    {2, variadic},
	"ne":    {2, 2},
	"lt":    {2, 2},
	"le":    {2, 2},
	"gt":    {2, 2},
	"ge":    {2, 2},
	"len":   {1, 1},
	"index": {1, variadic},
}

// templateSource identifies a template in error messages. An empty path
// selects the embedded default; a non-empty path is the override file the
// caller named, which is not a secret and is shown quoted.
type templateSource struct {
	name string
	path string
}

func (s templateSource) String() string {
	if s.path == "" {
		return s.name + " template (embedded default)"
	}
	return fmt.Sprintf("%s template (override file %q)", s.name, s.path)
}

// loadTemplate returns the checked template for src: embedded when src.path
// is empty, otherwise the content of the override file, read here once.
func loadTemplate(src templateSource, embedded string) (*template.Template, error) {
	text := embedded
	if src.path != "" {
		data, err := readOverrideFile(src)
		if err != nil {
			return nil, err
		}
		text = string(data)
	}
	return parseTemplate(src, text)
}

// readOverrideFile opens the override file, following symbolic links, and
// reads it with readOpenedFile. Every failure wraps both ErrInvalidTemplate
// and the underlying os error.
func readOverrideFile(src templateSource) ([]byte, error) {
	file, err := os.OpenFile(src.path, overrideOpenFlags, 0) //nolint:gosec // opens, read-only, the override file the caller named
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidTemplate, src, err)
	}
	defer func() { _ = file.Close() }()
	return readOpenedFile(src, file)
}

// openedFile is the part of *os.File that readOpenedFile uses.
type openedFile interface {
	io.Reader
	Stat() (fs.FileInfo, error)
}

// readOpenedFile refuses anything but a regular file and reads at most
// maxTemplateBytes+1 bytes, so parseTemplate rejects an oversized file
// without loading it all. The type is checked on the opened file itself, so
// the file cannot be swapped between the check and the read.
func readOpenedFile(src templateSource, file openedFile) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidTemplate, src, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidTemplate, src, errNotRegularFile)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxTemplateBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidTemplate, src, err)
	}
	return data, nil
}

// parseTemplate checks text in this order and returns the parsed template:
// at most maxTemplateBytes, valid UTF-8, not blank, parses as text/template
// with no added functions, contains no define or block, and uses only
// allowlisted syntax. Every failure wraps ErrInvalidTemplate.
func parseTemplate(src templateSource, text string) (*template.Template, error) {
	if len(text) > maxTemplateBytes {
		return nil, fmt.Errorf("%w: %s: exceeds the limit of %d bytes", ErrInvalidTemplate, src, maxTemplateBytes)
	}
	if !utf8.ValidString(text) {
		return nil, fmt.Errorf("%w: %s: is not valid UTF-8", ErrInvalidTemplate, src)
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: %s: is empty or whitespace only", ErrInvalidTemplate, src)
	}
	tmpl, err := template.New(src.name).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: does not parse: %q", ErrInvalidTemplate, src, err.Error())
	}
	if err := checkNoDefinitions(src.name, text, tmpl); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidTemplate, src, err)
	}
	checker := syntaxChecker{tree: tmpl.Tree}
	if err := checker.check(tmpl.Root); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidTemplate, src, err)
	}
	return tmpl, nil
}

// checkNoDefinitions rejects {{define}} and {{block}}. A definition under
// another name adds a second template to the set. A definition under the
// template's own name instead replaces the empty body, or is dropped when it
// is itself empty, so the set still holds one template; parsing text again
// under a different name turns that definition into a second template too.
func checkNoDefinitions(name, text string, tmpl *template.Template) error {
	if len(tmpl.Templates()) != 1 {
		return errTemplateDefinition
	}
	probe, err := template.New(name + probeNameSuffix).Parse(text)
	if err != nil || len(probe.Templates()) != 1 {
		return errTemplateDefinition
	}
	return nil
}

// syntaxChecker walks a parse tree and accepts only allowlisted nodes. Every
// node is checked whether or not expansion would reach it (both branches of
// an if, its condition, and the inside of parentheses and pipelines).
type syntaxChecker struct {
	tree *parse.Tree
}

// check dispatches on the node type. Only allowed node types have a case;
// every other type (with, range, template, break, continue, variables, the
// dot itself, nil, field chains, and any type a later Go release adds) falls
// through to default and is rejected. A function name has no case here: it
// is accepted only in the first position of a command (checkCommand), so one
// reached anywhere else is rejected too.
func (c syntaxChecker) check(node parse.Node) error {
	switch n := node.(type) {
	case *parse.ListNode:
		return c.checkList(n)
	// Comments never reach the tree: the parser drops them unless
	// parse.ParseComments is set, which it is not here.
	case *parse.TextNode, *parse.NumberNode, *parse.BoolNode:
		return nil
	case *parse.ActionNode:
		return c.checkPipe(n.Pipe)
	case *parse.IfNode:
		return c.checkIf(n)
	case *parse.PipeNode:
		return c.checkPipe(n)
	case *parse.FieldNode:
		return c.checkField(n)
	case *parse.StringNode:
		return c.checkString(n)
	default:
		return c.reject(node, describeDisallowed(node))
	}
}

// disallowedNames names, in the words of prompts/README.md, the rejected
// node types a template author is likely to write. It only labels error
// messages; the rejection itself is the default case of check.
var disallowedNames = map[parse.NodeType]string{
	parse.NodeWith:       "with",
	parse.NodeRange:      "range",
	parse.NodeTemplate:   "template",
	parse.NodeBreak:      "break",
	parse.NodeContinue:   "continue",
	parse.NodeDot:        "the dot (.)",
	parse.NodeVariable:   "variable",
	parse.NodeNil:        "nil",
	parse.NodeChain:      "field of a parenthesized value",
	parse.NodeIdentifier: "function used as an argument (wrap the call in parentheses)",
}

func describeDisallowed(node parse.Node) string {
	if name, ok := disallowedNames[node.Type()]; ok {
		return name
	}
	return fmt.Sprintf("%T", node)
}

func (c syntaxChecker) checkList(list *parse.ListNode) error {
	if list == nil {
		return nil
	}
	for _, node := range list.Nodes {
		if err := c.check(node); err != nil {
			return err
		}
	}
	return nil
}

func (c syntaxChecker) checkIf(n *parse.IfNode) error {
	if err := c.checkPipe(n.Pipe); err != nil {
		return err
	}
	if err := c.checkList(n.List); err != nil {
		return err
	}
	return c.checkList(n.ElseList)
}

func (c syntaxChecker) checkPipe(pipe *parse.PipeNode) error {
	if len(pipe.Decl) > 0 {
		return c.reject(pipe, "variable declaration")
	}
	for i, cmd := range pipe.Cmds {
		if err := c.checkCommand(cmd, i > 0); err != nil {
			return err
		}
	}
	return nil
}

// checkCommand checks the shape text/template evaluates a command with, so a
// call that fails for every input is rejected here rather than at Write. A
// command led by a function name is a call: the function must be allowed and
// get an accepted number of arguments, counting the value piped in when the
// command is a later stage of a pipeline (piped). Any other command is a
// single value (a field, a constant, a parenthesized pipeline) and takes no
// arguments and no piped value.
func (c syntaxChecker) checkCommand(cmd *parse.CommandNode, piped bool) error {
	// The parser never yields an empty command; the guard keeps Args[0]
	// below from panicking if a later Go release did.
	if len(cmd.Args) == 0 {
		return c.reject(cmd, "empty command")
	}
	fn, ok := cmd.Args[0].(*parse.IdentifierNode)
	if !ok {
		if len(cmd.Args) > 1 || piped {
			return c.reject(cmd, "arguments given to a value that is not a function")
		}
		return c.check(cmd.Args[0])
	}
	count := len(cmd.Args) - 1
	if piped {
		count++
	}
	if err := c.checkCall(fn, count); err != nil {
		return err
	}
	for _, arg := range cmd.Args[1:] {
		if err := c.check(arg); err != nil {
			return err
		}
	}
	return nil
}

// checkCall accepts an allowed function called with count arguments within
// its arity.
func (c syntaxChecker) checkCall(fn *parse.IdentifierNode, count int) error {
	arity, ok := allowedFuncs[fn.Ident]
	if !ok {
		return c.reject(fn, fmt.Sprintf("function %q", fn.Ident))
	}
	if count < arity.min || (arity.max != variadic && count > arity.max) {
		return c.reject(fn, fmt.Sprintf("function %q called with %d arguments", fn.Ident, count))
	}
	return nil
}

// checkField accepts a reference to one exported field of templateData and
// nothing deeper (.Title.Foo is rejected).
func (c syntaxChecker) checkField(n *parse.FieldNode) error {
	if len(n.Ident) == 1 {
		if field, ok := reflect.TypeFor[templateData]().FieldByName(n.Ident[0]); ok && field.IsExported() {
			return nil
		}
	}
	return c.reject(n, fmt.Sprintf("field reference %q", "."+strings.Join(n.Ident, ".")))
}

// checkString rejects a string constant whose unescaped value is not valid
// UTF-8 ({{"\xff"}}), which would put invalid bytes into the prompt.
func (c syntaxChecker) checkString(n *parse.StringNode) error {
	if utf8.ValidString(n.Text) {
		return nil
	}
	return c.reject(n, "string constant that is not valid UTF-8")
}

// reject reports what was rejected and where ("system:3:5").
func (c syntaxChecker) reject(node parse.Node, what string) error {
	location, _ := c.tree.ErrorContext(node)
	return fmt.Errorf("%w: %s at %s", errDisallowedSyntax, what, location)
}
