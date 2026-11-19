//go:build ignore

// textwidth_gen writes the wide-character table of textwidth.go from
// Unicode's EastAsianWidth.txt. Run it in this directory:
//
//	go run textwidth_gen.go <path to EastAsianWidth.txt>
//
// It rewrites only the lines between the two marker comments of
// textwidth.go and leaves the hand-written code alone, so running it again on
// the same file changes nothing.
//
// Contract: a code point is wide when its East_Asian_Width is W or F, taking
// the @missing defaults first and the explicit lines over them, as UAX #44
// reads the file. A line of any other form stops the run; nothing is guessed.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	target      = "textwidth.go"
	beginMarker = "// Begin generated wide ranges (textwidth_gen.go). Do not edit."
	endMarker   = "// End generated wide ranges."
	maxRune     = 0x10ffff
)

var (
	versionLine = regexp.MustCompile(`^# EastAsianWidth-(\d+\.\d+\.\d+)\.txt$`)
	dataLine    = regexp.MustCompile(`^([0-9A-F]{4,6})(?:\.\.([0-9A-F]{4,6}))?\s*;\s*(A|F|H|N|Na|W)\s*(?:#.*)?$`)
	missingLine = regexp.MustCompile(`^# @missing: ([0-9A-F]{4,6})\.\.([0-9A-F]{4,6})\s*;\s*(A|F|H|N|Na|W)\s*$`)
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run textwidth_gen.go <EastAsianWidth.txt>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "textwidth_gen:", err)
		os.Exit(1)
	}
}

func run(source string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	version, wide, err := parse(data)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	block := render(version, hex.EncodeToString(sum[:]), ranges(wide))
	old, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	updated, err := replaceBlock(old, block)
	if err != nil {
		return err
	}
	formatted, err := format.Source(updated)
	if err != nil {
		return err
	}
	return os.WriteFile(target, formatted, 0o644)
}

// parse returns the file's version and, for every code point, whether it is
// wide (W or F).
func parse(data []byte) (string, []bool, error) {
	wide := make([]bool, maxRune+1)
	set := func(lo, hi string, value string) error {
		first, err := strconv.ParseUint(lo, 16, 32)
		if err != nil {
			return err
		}
		last := first
		if hi != "" {
			if last, err = strconv.ParseUint(hi, 16, 32); err != nil {
				return err
			}
		}
		if first > last || last > maxRune {
			return fmt.Errorf("bad range %s..%s", lo, hi)
		}
		for r := first; r <= last; r++ {
			wide[r] = value == "W" || value == "F"
		}
		return nil
	}
	version := ""
	var explicit [][3]string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimRight(scanner.Text(), " \r")
		switch {
		case n == 1:
			m := versionLine.FindStringSubmatch(line)
			if m == nil {
				return "", nil, fmt.Errorf("line 1 is not the version line: %q", line)
			}
			version = m[1]
		case missingLine.MatchString(line):
			// The defaults come first: they apply before every explicit line.
			m := missingLine.FindStringSubmatch(line)
			if err := set(m[1], m[2], m[3]); err != nil {
				return "", nil, fmt.Errorf("line %d: %w", n, err)
			}
		case line == "" || strings.HasPrefix(line, "#"):
		default:
			m := dataLine.FindStringSubmatch(line)
			if m == nil {
				return "", nil, fmt.Errorf("line %d has another form: %q", n, line)
			}
			explicit = append(explicit, [3]string{m[1], m[2], m[3]})
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	if version == "" {
		return "", nil, errors.New("empty file")
	}
	for _, e := range explicit {
		if err := set(e[0], e[1], e[2]); err != nil {
			return "", nil, err
		}
	}
	return version, wide, nil
}

// ranges merges the wide code points into sorted, disjoint ranges.
func ranges(wide []bool) [][2]int {
	var out [][2]int
	for r := 0; r <= maxRune; r++ {
		if !wide[r] {
			continue
		}
		if len(out) > 0 && out[len(out)-1][1] == r-1 {
			out[len(out)-1][1] = r
			continue
		}
		out = append(out, [2]int{r, r})
	}
	return out
}

func render(version, sum string, rs [][2]int) string {
	var b strings.Builder
	fmt.Fprintln(&b, beginMarker)
	fmt.Fprintf(&b, "//\n// Source: EastAsianWidth.txt of Unicode %s,\n", version)
	fmt.Fprintf(&b, "// https://www.unicode.org/Public/%s/ucd/EastAsianWidth.txt\n", version)
	fmt.Fprintf(&b, "// SHA-256: %s\n", sum)
	fmt.Fprintf(&b, "\n// wideRanges holds the code points whose East_Asian_Width is W or F, as\n// sorted, disjoint, inclusive ranges.\n")
	fmt.Fprintln(&b, "var wideRanges = []runeRange{")
	for _, r := range rs {
		fmt.Fprintf(&b, "\t{0x%04X, 0x%04X},\n", r[0], r[1])
	}
	fmt.Fprintln(&b, "}")
	fmt.Fprintln(&b)
	fmt.Fprint(&b, endMarker)
	return b.String()
}

// replaceBlock replaces the lines from the begin marker through the end
// marker with block. Each marker must be on a line of its own, once.
func replaceBlock(src []byte, block string) ([]byte, error) {
	text := string(src)
	begin := strings.Index(text, beginMarker)
	end := strings.Index(text, endMarker)
	if begin < 0 || end < begin || strings.Count(text, beginMarker) != 1 || strings.Count(text, endMarker) != 1 {
		return nil, errors.New(target + ": the two markers are not there once each, in order")
	}
	return []byte(text[:begin] + block + text[end+len(endMarker):]), nil
}
