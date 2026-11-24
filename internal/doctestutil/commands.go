//go:build test

package doctestutil

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// The helpers below read the commands a command document writes, so a guard
// can check how a tool is called (Invocations, PromptFree) and that a
// command is never run directly (RunsCommand).

// Invocation is one call of a tool a document writes as a command.
type Invocation struct {
	// Lead is the words of the command before the tool's word.
	Lead []string
	// Word is the tool's word as written: its name, a path, or quoted.
	Word string
	// Args is the words after the tool's word, up to the end of its command.
	Args []string
	// Joined reports that the command is one of several in its text (after
	// cd x && or before a pipe), which the Bash tool runs as one call.
	Joined bool
}

// Invocations returns each call of tool that doc writes as a command
// (CommandTexts): a command (CommandBoundary) holding a word that
// is the tool by name or path.
func Invocations(doc, tool string) []Invocation {
	var calls []Invocation
	for _, text := range CommandTexts(doc) {
		commands := CommandBoundary.Split(text, -1)
		for _, command := range commands {
			words := strings.Fields(command)
			i := slices.IndexFunc(words, func(w string) bool { return path.Base(strings.Trim(w, `"'`)) == tool })
			if i < 0 {
				continue
			}
			call := Invocation{Word: words[i], Args: words[i+1:], Joined: len(commands) > 1}
			if i > 0 {
				call.Lead = words[:i]
			}
			calls = append(calls, call)
		}
	}
	return calls
}

// PromptFree reports whether call is written in the one form the permission
// rule Bash(<tool> *) allows without the prompt: the tool's bare name first
// in its command, with nothing before it, and no other command in the same
// text.
func PromptFree(call Invocation, tool string) bool {
	return len(call.Lead) == 0 && call.Word == tool && !call.Joined
}

var (
	// assignment is a variable assignment before a command word.
	assignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// redirection is a redirection word, which may come before a command.
	redirection = regexp.MustCompile(`^[0-9]*[<>]`)
	// bareRedirection is a redirection operator written as a word of its
	// own, whose target is the next word (> file, 2>> file, << EOF,
	// <<< word), unlike a word that holds its target (>file, 2>&1). The '&'
	// of &> needs no rule: CommandBoundary splits there.
	bareRedirection = regexp.MustCompile(`^[0-9]*(?:<|>|>>|>\||<>|<<|<<-|<<<)$`)
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

// envWrapper is env, the wrapper whose -S splits a string into arguments.
const envWrapper = "env"

var (
	// wrappers run the command named by their first argument that is not an
	// option or an option's value; each maps to its options that take the
	// next word as their value (env -u NAME, xargs -I REPLACE, timeout -k
	// DURATION).
	wrappers = map[string]map[string]struct{}{
		envWrapper: set("-u", "-C", "-P"), "xargs": set("-I", "-n", "-P", "-L", "-s", "-E", "-d"),
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

// RunsCommand reports whether text, a code block line or a command written
// as a code span, has the command name at a command position: the start of a command,
// after a separator (CommandBoundary), after the reserved words,
// variable assignments, and redirections that may precede a command, or as
// the command a wrapper, env -S's split string, or a shell's -c script
// runs. The name counts by name, by path (/opt/homebrew/bin/widget, ./widget,
// @example/widget for widget), quoted, escaped with a backslash, or opening a
// group. The
// list of wrappers and of options with a value is not complete; the
// permission prompt stops a form it misses.
func RunsCommand(text, name string) bool {
	return len(CommandArgs(text, name)) > 0
}

// CommandArgs returns the whitespace-split argument slices passed to each
// command-position invocation of name in text, using the same command-position
// rules as RunsCommand. Returned words may still carry surrounding shell
// quotes or grouping punctuation.
func CommandArgs(text, name string) [][]string {
	var all [][]string
	for _, command := range CommandBoundary.Split(text, -1) {
		all = append(all, commandArgs(strings.Fields(command), name)...)
	}
	return all
}

// commandArgs returns the argument slices for each invocation of name in words.
func commandArgs(words []string, name string) [][]string {
	for i := 0; i < len(words); i++ {
		word := strings.TrimLeft(strings.Trim(words[i], `"'`), `({\`)
		word = strings.TrimRight(strings.Trim(word, `"'`), `)};`)
		base := path.Base(word)
		switch {
		case bareRedirection.MatchString(word):
			// The next word is the redirection's target, not a command.
			i++
		case word == "", isSetMember(reservedWords, word), assignment.MatchString(word), redirection.MatchString(word):
		case isWrapper(base):
			valued := wrappers[base]
			for i+1 < len(words) && (strings.HasPrefix(words[i+1], "-") || optionValue.MatchString(words[i+1])) {
				if split, ok := envSplitString(base, words[i+1]); ok {
					// The split string is env's own arguments, so read it
					// and the words after it as one env command.
					var parts []string
					if split != "" {
						parts = append(parts, split)
					}
					parts = append(parts, words[i+2:]...)
					splitText := strings.Trim(strings.Join(parts, " "), `"'`)
					rest := slices.Concat([]string{envWrapper}, strings.Fields(splitText))
					return commandArgs(rest, name)
				}
				if isSetMember(valued, words[i+1]) {
					i++
				}
				i++
			}
		case isShell(base):
			j := slices.IndexFunc(words[i+1:], shellScriptFlag.MatchString)
			if j < 0 {
				return nil
			}
			script := strings.Join(words[i+2+j:], " ")
			return CommandArgs(strings.Trim(script, `"'`), name)
		default:
			if base == name {
				return [][]string{words[i+1:]}
			}
			return nil
		}
	}
	return nil
}

// envSplitString reports whether opt is env's -S (alone, in an option
// cluster such as -iS, or as --split-string), and returns the part of its
// value written in opt; the rest of the value is the words after opt.
func envSplitString(wrapper, opt string) (string, bool) {
	if wrapper != envWrapper {
		return "", false
	}
	if v, ok := strings.CutPrefix(opt, "--split-string"); ok && (v == "" || v[0] == '=') {
		return strings.TrimPrefix(v, "="), true
	}
	if !strings.HasPrefix(opt, "-") || strings.HasPrefix(opt, "--") {
		return "", false
	}
	for j, c := range opt[1:] {
		switch {
		case c == 'S':
			return opt[j+2:], true
		case isSetMember(wrappers[envWrapper], "-"+string(c)):
			return "", false
		}
	}
	return "", false
}

// FencedCommandBlocks returns the fenced code blocks of doc, including blocks
// inside Markdown blockquotes (such as an agent prompt). Each block's lines
// have only the opening fence's blockquote depth removed, so a leading '>' in
// an unquoted block stays a shell redirection. A block with no closing fence
// runs to the end of doc and is returned with Closed false.
func FencedCommandBlocks(doc string) []FencedBlock {
	var raw, lines []string
	for line := range strings.Lines(doc) {
		raw = append(raw, strings.TrimSuffix(line, "\n"))
		lines = append(lines, quoteMarker.ReplaceAllString(line, ""))
	}
	var blocks []FencedBlock
	var current *FencedBlock
	depth := 0
	for i, line := range ScanDocLines(lines) {
		switch {
		case line.Fence && current == nil:
			depth = quoteDepth(raw[i])
			current = &FencedBlock{Info: strings.TrimLeft(strings.TrimSpace(line.Text), "`")}
		case line.Fence:
			current.Closed = true
			blocks = append(blocks, *current)
			current = nil
			depth = 0
		case current != nil:
			current.Lines = append(current.Lines, stripQuotes(raw[i], depth))
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}
	return blocks
}

// CommandTexts returns the texts of doc that are written as commands: each
// line of a fenced code block, and each code span outside the blocks that
// holds an argument after a space. Blockquote markers are dropped first, so
// the blocks of an agent prompt are read too; inside a block whose opening
// fence is not quoted, a line keeps its leading '>', which is then a
// redirection (> file widget ...), not a quote marker.
func CommandTexts(doc string) []string {
	var raw, lines []string
	for line := range strings.Lines(doc) {
		raw = append(raw, strings.TrimSuffix(line, "\n"))
		lines = append(lines, quoteMarker.ReplaceAllString(line, ""))
	}
	var texts []string
	// open is true between an opening and a closing fence; depth is the
	// number of quote markers before that opening fence, which are the only
	// ones a line of the block has; a '>' after them is a redirection.
	open, depth := false, 0
	for i, line := range ScanDocLines(lines) {
		switch {
		case line.Fence:
			open = !open
			depth = 0
			if open {
				depth = quoteDepth(raw[i])
			}
		case line.Block:
			texts = append(texts, stripQuotes(raw[i], depth))
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

// quoteDepth returns how many blockquote markers start line.
func quoteDepth(line string) int {
	return strings.Count(quoteMarker.FindString(line), ">")
}

// stripQuotes removes up to depth blockquote markers, each with one space
// after it, from the start of line, and the blanks before each.
func stripQuotes(line string, depth int) string {
	for range depth {
		rest, ok := strings.CutPrefix(strings.TrimLeft(line, " \t"), ">")
		if !ok {
			break
		}
		line = strings.TrimPrefix(rest, " ")
	}
	return line
}

func isSetMember(m map[string]struct{}, key string) bool { _, ok := m[key]; return ok }

// bashPermissionRule matches a Claude Code Bash permission rule and captures
// its command pattern.
var bashPermissionRule = regexp.MustCompile(`^Bash\((.*)\)$`)

// namesToolInBashRule reports whether rule is a Bash permission rule whose
// command word is tool, by name, by path, or quoted, or a glob pattern that
// starts with tool's name and holds a wildcard.
func namesToolInBashRule(rule, tool string) bool {
	m := bashPermissionRule.FindStringSubmatch(rule)
	if m == nil {
		return false
	}
	word, _, _ := strings.Cut(strings.TrimSpace(m[1]), " ")
	word, _, _ = strings.Cut(word, ":")
	base := path.Base(strings.Trim(word, `"'`))
	rest, ok := strings.CutPrefix(base, tool)
	return ok && (rest == "" || strings.ContainsAny(rest, "*?["))
}

// BashToolRuleMismatches returns the Bash permission rules in allow that name
// tool but are not listed in allowed, and the entries of allowed that allow
// lacks.
func BashToolRuleMismatches(allow []string, tool string, allowed []string) (extra, missing []string) {
	for _, rule := range allow {
		if namesToolInBashRule(rule, tool) && !slices.Contains(allowed, rule) {
			extra = append(extra, rule)
		}
	}
	for _, rule := range allowed {
		if !slices.Contains(allow, rule) {
			missing = append(missing, rule)
		}
	}
	return extra, missing
}
