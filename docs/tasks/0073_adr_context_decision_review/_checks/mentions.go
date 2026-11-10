//go:build test

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

// mentionContext is how many characters of a long line mentions prints on
// each side of the name it found.
const mentionContext = 80

// issueLinkRE finds a link to a GitHub issue in an ADR.
var issueLinkRE = regexp.MustCompile(`https://github\.com/[^/\s]+/[^/\s]+/issues/[0-9]+`)

// mention is one line outside docs/tasks/ that names an ADR.
type mention struct {
	path string
	line int
	text string
}

// mentionPatterns returns the extended regular expressions that find an ADR
// named by its number (ADR-NNNN, or without the hyphen) or by a file name
// starting with its number.
func mentionPatterns(num string) []string {
	return []string{
		`ADR[- ]?` + num + `([^0-9]|$)`,
		`(^|[^0-9])` + num + `-[a-z0-9]+(-[a-z0-9]+)*\.md`,
	}
}

// findMentions lists the lines at a commit that name the ADR, across the whole
// repository except docs/tasks/ and the ADR itself.
func findMentions(commit, adr string) ([]mention, error) {
	num := adrNumber(adr)
	args := []string{"grep", "-n", "-I", "-E", "--full-name", "--no-color"}
	for _, p := range mentionPatterns(num) {
		args = append(args, "-e", p)
	}
	args = append(args, commit, "--", ":(top)", ":(top,exclude)docs/tasks", ":(top,exclude)"+adr)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
			return nil, nil // no match
		}
		return nil, fmt.Errorf("git grep for ADR-%s at %s: %w", num, commit, err)
	}
	var found []mention
	for line := range strings.SplitSeq(strings.TrimSuffix(string(out), "\n"), "\n") {
		// Each line is <commit>:<path>:<line>:<text>.
		rest, ok := strings.CutPrefix(line, commit+":")
		if !ok {
			return nil, fmt.Errorf("git grep printed an unexpected line %q", line)
		}
		p, rest, ok1 := strings.Cut(rest, ":")
		n, text, ok2 := strings.Cut(rest, ":")
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("git grep printed an unexpected line %q", line)
		}
		var ln int
		if _, err := fmt.Sscanf(n, "%d", &ln); err != nil {
			return nil, fmt.Errorf("git grep printed an unexpected line %q", line)
		}
		found = append(found, mention{p, ln, text})
	}
	return found, nil
}

// excerpt returns the part of a long line around the first name of the ADR.
func excerpt(text, num string) string {
	r := []rune(text)
	if len(r) <= 2*mentionContext {
		return text
	}
	at := 0
	for _, p := range mentionPatterns(num) {
		if loc := regexp.MustCompile(p).FindStringIndex(text); loc != nil {
			at = len([]rune(text[:loc[0]]))
			break
		}
	}
	from, to := max(0, at-mentionContext), min(len(r), at+mentionContext)
	s := string(r[from:to])
	if from > 0 {
		s = "..." + s
	}
	if to < len(r) {
		s += "..."
	}
	return s
}

// runMentions prints, for each ADR of the group, the lines that name it at
// the commit and the issue links inside it.
func runMentions(name string, args []string) (int, error) {
	rev := "HEAD"
	switch len(args) {
	case 0:
	case 1:
		rev = args[0]
	default:
		return 0, errUsage
	}
	commit, err := resolve(rev)
	if err != nil {
		return 0, err
	}
	d, err := loadData("HEAD")
	if err != nil {
		return 0, err
	}
	g, err := groupFor(d, name)
	if err != nil {
		return 0, err
	}
	fmt.Printf("group %s\ncommit %s\n", name, commit)
	for _, adr := range uniqueADRPaths(g.reviewed()) {
		num := adrNumber(adr)
		fmt.Printf("== %s\n", adr)
		found, err := findMentions(commit, adr)
		if err != nil {
			return 0, err
		}
		for _, m := range found {
			fmt.Printf("MENTION %s:%d: %s\n", m.path, m.line, excerpt(m.text, num))
		}
		text, err := git("show", commit+":"+adr)
		if err != nil {
			return 0, err
		}
		for i, l := range strings.Split(text, "\n") {
			for _, u := range issueLinkRE.FindAllString(l, -1) {
				fmt.Printf("ISSUE %s:%d: %s\n", adr, i+1, u)
			}
		}
	}
	return 0, nil
}

// runFreshness compares the merge-base of origin/main and HEAD with
// origin/main: it prints every file that changed, exits 1 when a group ADR or
// the index changed, and prints the lines that name a group ADR at one commit
// and not at the other.
func runFreshness(name string) (int, error) {
	out, err := git("merge-base", "origin/main", "HEAD")
	if err != nil {
		return 0, err
	}
	base, err := resolve(out)
	if err != nil {
		return 0, err
	}
	target, err := resolve("origin/main")
	if err != nil {
		return 0, err
	}
	d, err := loadData("HEAD")
	if err != nil {
		return 0, err
	}
	g, err := groupFor(d, name)
	if err != nil {
		return 0, err
	}
	fmt.Printf("group %s\nbase %s\norigin/main %s\n", name, base, target)
	changed, err := changedFiles(base, target)
	if err != nil {
		return 0, err
	}
	watched := append(uniqueADRPaths(g.reviewed()), d.Index)
	code := 0
	for _, p := range changed {
		if slices.Contains(watched, p) {
			fmt.Printf("CHANGED-WATCHED %s\n", p)
			code = 1
			continue
		}
		fmt.Printf("CHANGED %s\n", p)
	}
	for _, adr := range uniqueADRPaths(g.reviewed()) {
		num := adrNumber(adr)
		before, err := findMentions(base, adr)
		if err != nil {
			return 0, err
		}
		after, err := findMentions(target, adr)
		if err != nil {
			return 0, err
		}
		key := func(m mention) string { return m.path + ": " + m.text }
		left := map[string]int{}
		for _, m := range after {
			left[key(m)]++
		}
		for _, m := range before {
			if left[key(m)] > 0 {
				left[key(m)]--
				continue
			}
			fmt.Printf("MENTION-GONE %s %s:%d: %s\n", adr, m.path, m.line, excerpt(m.text, num))
		}
		for _, m := range after {
			if left[key(m)] > 0 {
				left[key(m)]--
				fmt.Printf("MENTION-NEW %s %s:%d: %s\n", adr, m.path, m.line, excerpt(m.text, num))
			}
		}
	}
	if code == 1 {
		fmt.Println("FAIL a group ADR or the index changed on origin/main")
	} else {
		fmt.Println("OK no group ADR and not the index changed on origin/main")
	}
	return code, nil
}
