//go:build test

package mergepr

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// lineOf builds an expected correction line.
func lineOf(commit string, line int, text string) correctionLine {
	return correctionLine{Commit: commit, Line: line, Text: []byte(text)}
}

func equalLines(a, b []correctionLine) bool {
	return slices.EqualFunc(a, b, func(x, y correctionLine) bool {
		return x.Commit == y.Commit && x.Line == y.Line && bytes.Equal(x.Text, y.Text)
	})
}

// revList returns the subjects git rev-list prints for head ^base, oldest
// first, with the extra options.
func revList(r *gitRepo, head, base string, opts ...string) []string {
	args := append([]string{"rev-list", "--reverse"}, opts...)
	args = append(args, "--no-commit-header", "--format=%s", head, "^"+base)
	return strings.Split(r.Git(args...), "\n")
}

func TestRangeLines(t *testing.T) {
	r := newGitRepo(t)
	m0 := r.RawCommit([]byte("main\n"), 100, nil)
	// A main commit the branch takes in: its line is not the PR's.
	mainLine := r.RawCommit([]byte("main fix\n\nEditorial correction: main only\n"), 150, nil, m0)
	r.SetMain(mainLine)
	// Two lines of history whose commit times interleave, so rev-list's
	// default order mixes them and --topo-order does not.
	a1 := r.RawCommit([]byte("Editorial correction: in the subject; no decision changed\n\nbody\n"), 200, nil, m0)
	b1 := r.RawCommit([]byte("b1\n\nEditorial correction: b-one\n"), 300, nil, m0)
	a2 := r.RawCommit([]byte("a2\n\n"+
		"  Editorial correction: indented\n"+
		"editorial correction: lower case\n"+
		"see Editorial correction: mid-sentence\n"+
		"Editorial correction: docs/dev/adr/0001-x.md: fixed; no decision changed\n"+
		"Editorial correction: docs/tasks/0001_x/01_requirements.md: fixed; no decision changed\n"+
		"Editorial correction: free form, no path\r\n"), 400, nil, a1)
	// The encoding header names Latin-1; git log would re-encode the byte
	// 0xe9 to UTF-8, the object stores it as written.
	b2 := r.RawCommit([]byte("b2\n\nEditorial correction: caf\xe9\n"), 500, []string{"encoding ISO-8859-1"}, b1)
	merge := r.RawCommit([]byte("merge b\n\nEditorial correction: from the merge\n"), 600, nil, a2, b2)
	head := r.RawCommit([]byte("take main in\n"), 700, nil, merge, mainLine)

	byDefault := revList(r, head, mainLine)
	byTopo := revList(r, head, mainLine, "--topo-order")
	if slices.Equal(byDefault, byTopo) {
		t.Fatalf("precondition: the default order %v equals --topo-order", byDefault)
	}
	if want := []string{"Editorial correction: in the subject; no decision changed", "a2", "b1", "b2", "merge b", "take main in"}; !slices.Equal(byTopo, want) {
		t.Fatalf("precondition: --topo-order = %v, want %v", byTopo, want)
	}
	want := []correctionLine{
		lineOf(a1, 1, "Editorial correction: in the subject; no decision changed"),
		lineOf(a2, 6, "Editorial correction: docs/dev/adr/0001-x.md: fixed; no decision changed"),
		lineOf(a2, 7, "Editorial correction: docs/tasks/0001_x/01_requirements.md: fixed; no decision changed"),
		lineOf(a2, 8, "Editorial correction: free form, no path\r"),
		lineOf(b1, 3, "Editorial correction: b-one"),
		lineOf(b2, 3, "Editorial correction: caf\xe9"),
		lineOf(merge, 3, "Editorial correction: from the merge"),
	}
	src := newGitCorrectionSource(r.Dir)
	got, err := src.rangeLines(t.Context(), mainBranch, head)
	if err != nil || !equalLines(got, want) {
		t.Fatalf("rangeLines = %+v, %v; want %+v", got, err, want)
	}

	t.Run("other base", func(t *testing.T) {
		// A base other than main is read from its own remote-tracking ref:
		// with b2 as the base, only the A line and the merge remain.
		rt := r.withT(t)
		rt.Git("update-ref", remoteBranchPrefix+"release", b2)
		got, err := src.rangeLines(t.Context(), "release", merge)
		if want := []correctionLine{want[0], want[1], want[2], want[3], want[6]}; err != nil || !equalLines(got, want) {
			t.Errorf("rangeLines from release = %+v, %v; want %+v", got, err, want)
		}
	})
	t.Run("replace ref", func(t *testing.T) {
		rt := r.withT(t)
		other := rt.RawCommit([]byte("Editorial correction: replacement\n"), 200, nil, m0)
		rt.Git("replace", a1, other)
		t.Cleanup(func() { rt.Git("replace", "-d", a1) })
		if got, err := src.rangeLines(t.Context(), mainBranch, head); err != nil || !equalLines(got, want) {
			t.Errorf("rangeLines with a replace ref = %+v, %v; want %+v", got, err, want)
		}
	})
	t.Run("shallow clone", func(t *testing.T) {
		dir := r.withT(t).ShallowClone(head)
		_, err := newGitCorrectionSource(dir).rangeLines(t.Context(), mainBranch, head)
		if !errors.Is(err, errCorrectionUnreadable) {
			t.Errorf("rangeLines in a shallow clone error = %v, want errCorrectionUnreadable", err)
		}
	})
}

func TestRangeLinesStopsMidway(t *testing.T) {
	const (
		base = "1111111111111111111111111111111111111111"
		c1   = "5555555555555555555555555555555555555555"
		c2   = "6666666666666666666666666666666666666666"
		c3   = "7777777777777777777777777777777777777777"
	)
	ref := remoteBranchPrefix + mainBranch
	listing := []commandStep{
		{name: gitCommand, args: hardened("rev-parse", "--is-shallow-repository"), out: "false\n"},
		{name: gitCommand, args: hardened("check-ref-format", ref)},
		{name: gitCommand, args: hardened("show-ref", "--verify", "--hash", ref), out: base + "\n"},
		{name: gitCommand, args: hardened("rev-parse", "--verify", "--quiet", base+"^{commit}"), out: base + "\n"},
		{name: gitCommand, args: hardened("rev-parse", "--verify", "--quiet", testHeadOID+"^{commit}"), out: testHeadOID + "\n"},
		{
			name: gitCommand, args: hardened("rev-list", "--reverse", "--topo-order", "--no-commit-header", revListFormat, testHeadOID, "^"+base),
			out: c1 + " " + base + "\x00one\n" + c2 + " " + c1 + "\x00two\n" + c3 + " " + c2 + "\x00three\n",
		},
		{name: gitCommand, args: hardened("cat-file", "commit", c1), out: "tree x\n\nEditorial correction: one\n"},
	}
	tests := []struct {
		name string
		c2   commandStep
	}{
		{"cat-file fails", commandStep{name: gitCommand, args: hardened("cat-file", "commit", c2), err: &cmdrunner.ExitError{Code: 128}}},
		{"no empty line after the header", commandStep{name: gitCommand, args: hardened("cat-file", "commit", c2), out: "tree x\nEditorial correction: two\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The script ends at the second commit, so reading the third
			// fails the test.
			runner := &fakeRunner{t: t, steps: append(slices.Clone(listing), tt.c2)}
			src := &gitCorrectionSource{reader: gitReader{run: runner}}
			got, err := src.rangeLines(t.Context(), mainBranch, testHeadOID)
			if !errors.Is(err, errCorrectionUnreadable) || got != nil {
				t.Errorf("rangeLines = %+v, %v; want no line and errCorrectionUnreadable", got, err)
			}
			runner.done()
		})
	}
}

func TestMergedLines(t *testing.T) {
	origin := newGitRepo(t)
	// A header line that starts with a space continues the header, so its
	// empty-looking line " " does not end it.
	signature := []string{"gpgsig -----BEGIN SSH SIGNATURE-----", " U1NIU0lH", " ", " -----END SSH SIGNATURE-----"}
	squash := origin.RawCommit([]byte("Editorial correction: in the subject (#1)\n\n"+
		"Summary.\n\n"+
		"Editorial correction: one\n"+
		"Dropped editorial corrections: 1 disputed by a reviewer, 0 reverted on the branch, 0 with rejected characters\n\n"+
		" Editorial correction: indented\n"+
		"Co-Authored-By: Test <test@example.com>\n"), 100, signature)
	origin.Git("update-ref", "refs/heads/"+mainBranch, squash)
	local := newGitRepo(t)
	local.Git("remote", "add", "origin", "file://"+filepath.ToSlash(origin.Dir))
	src := newGitCorrectionSource(local.Dir)

	got, err := src.mergedLines(t.Context(), mainBranch, squash)
	want := [][]byte{
		[]byte("Editorial correction: in the subject (#1)"),
		[]byte("Editorial correction: one"),
		[]byte("Dropped editorial corrections: 1 disputed by a reviewer, 0 reverted on the branch, 0 with rejected characters"),
	}
	if err != nil || !slices.EqualFunc(got, want, bytes.Equal) {
		t.Fatalf("mergedLines = %q, %v; want %q", got, err, want)
	}
	if _, err := src.mergedLines(t.Context(), "no-such-branch", squash); !isExitError(err) {
		t.Errorf("mergedLines of a branch origin lacks: error = %v, want the fetch's exit status", err)
	}
}

func TestMergedLinesRejectsInvalidMergeID(t *testing.T) {
	// The scripted runner has no step, so the ID is refused before any git.
	runner := &fakeRunner{t: t}
	src := &gitCorrectionSource{reader: gitReader{run: runner}}
	if _, err := src.mergedLines(t.Context(), mainBranch, testMergeOID[:12]); !errors.Is(err, errMergeNotObjectID) {
		t.Errorf("mergedLines error = %v, want errMergeNotObjectID", err)
	}
}

func TestMergedLinesRejectsInvalidNames(t *testing.T) {
	for _, name := range invalidBaseNames {
		t.Run(name, func(t *testing.T) {
			// The scripted runner fails on any command after the name check,
			// so no refspec is built from the name.
			runner := &fakeRunner{t: t, steps: []commandStep{{
				name: gitCommand,
				args: hardened("check-ref-format", remoteBranchPrefix+name),
				err:  &cmdrunner.ExitError{Code: 1},
			}}}
			src := &gitCorrectionSource{reader: gitReader{run: runner}}
			_, err := src.mergedLines(t.Context(), name, testMergeOID)
			if e, ok := errors.AsType[*rangeError](err); !ok || e.Stage != rangeStageBase {
				t.Errorf("mergedLines(%q) error = %v, want a base-stage range error", name, err)
			}
			runner.done()
		})
	}
}

func TestTrailerReadingIgnoresConfig(t *testing.T) {
	// The default configuration reads no trailer here; trailer.separators
	// ":=" makes git read "Key=value" as one.
	message := []byte("subject\n\nbody\n\nKey=value\n")
	const config = "[trailer]\n\tseparators = :=\n"
	r := newGitRepo(t)
	tests := []struct {
		name string
		// write puts the configuration in place and returns the directory
		// where plain git reads it.
		write func(t *testing.T) string
	}{
		{"HOME", func(t *testing.T) string {
			writeConfig(t, filepath.Join(r.Home, ".gitconfig"), config)
			return t.TempDir()
		}},
		// GIT_CONFIG_GLOBAL also keeps git from this file, so this row
		// overlaps the HOME row's guard.
		{"XDG_CONFIG_HOME", func(t *testing.T) string {
			writeConfig(t, filepath.Join(r.Home, ".config", "git", "config"), config)
			return t.TempDir()
		}},
		{"repository", func(t *testing.T) string {
			rt := r.withT(t)
			rt.Git("config", "trailer.separators", ":=")
			t.Cleanup(func() { rt.Git("config", "--unset", "trailer.separators") })
			sub := filepath.Join(r.Dir, "sub")
			if err := os.MkdirAll(sub, testRepoDirMode); err != nil {
				t.Fatalf("create %s: %v", sub, err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(sub) })
			return sub
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.write(t)
			if out := r.withT(t).gitInput(dir, message, "interpret-trailers", "--parse", "--no-divider"); out == "" {
				t.Fatal("precondition: plain git read no trailer with the configuration")
			}
			// The trailer reading makes its directory inside the repository,
			// where git would find .git/config by looking upward; the
			// relative parent is the form a relative TMPDIR gives.
			t.Chdir(r.Dir)
			for _, parent := range []string{r.Dir, "."} {
				src := newGitCorrectionSource(r.Dir)
				src.tempParent = parent
				out, err := src.trailers(t.Context(), message)
				if err != nil || len(out) != 0 {
					t.Errorf("trailers in %q = %q, %v; want no output", parent, out, err)
				}
			}
			left, err := filepath.Glob(filepath.Join(r.Dir, "mergepr-trailers-*"))
			if err != nil || len(left) != 0 {
				t.Errorf("left behind %v (%v), want the directory removed", left, err)
			}
		})
	}
}

// writeConfig writes a git configuration file and removes it at the end of
// the test.
func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}

const (
	commitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestClassifyCorrectionLines(t *testing.T) {
	// Each row puts one character between "x" and "y" of a correction line.
	inv, bad := []RejectKind{RejectInvisible}, []RejectKind{RejectInvalidUTF8}
	tests := []struct {
		name string
		mid  string
		want []RejectKind // none for an accepted line
	}{
		{"tab", "\t", nil},
		{"space", " ", nil},
		{"japanese", "\u65e5\u672c\u8a9e", nil},
		{"hangul filler", "\u3164", nil},
		{"replacement character", "\ufffd", nil},
		{"greater-than", ">", nil},
		{"before the noncharacters", "\ufdcf", nil},
		{"after the noncharacters", "\ufdf0", nil},
		{"vertical tab", "\x0b", inv},
		{"carriage return", "\r", inv},
		{"delete", "\x7f", inv},
		{"next line", "\u0085", inv},
		{"soft hyphen", "\u00ad", inv},
		{"zero width space", "\u200b", inv},
		{"right-to-left override", "\u202e", inv},
		{"byte order mark", "\ufeff", inv},
		{"line separator", "\u2028", inv},
		{"paragraph separator", "\u2029", inv},
		{"first noncharacter", "\ufdd0", bad},
		{"last noncharacter of the block", "\ufdef", bad},
		{"U+FFFE", "\ufffe", bad},
		{"U+1FFFE", "\U0001fffe", bad},
		{"U+10FFFF", "\U0010ffff", bad},
		{"byte 0xff", "\xff", bad},
		{"cut multibyte sequence", "\xe3\x81", bad},
		{"surrogate", "\xed\xa0\x80", bad},
		{"at", "@", []RejectKind{RejectAt}},
		{"less-than", "<", []RejectKind{RejectLessThan}},
		{"every kind, in constant order", "<@\xff\x01<", []RejectKind{RejectInvisible, RejectInvalidUTF8, RejectAt, RejectLessThan}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := "Editorial correction: x" + tt.mid + "y"
			set := classifyCorrectionLines([]correctionLine{lineOf(commitA, 3, text)})
			if tt.want == nil {
				if len(set.Rejected) != 0 || len(set.Accepted) != 1 || string(set.Accepted[0]) != text {
					t.Errorf("set = %+v, want %q accepted as it is", set, text)
				}
				return
			}
			if len(set.Accepted) != 0 || len(set.Rejected) != 1 || !slices.Equal(set.Rejected[0].Kinds, tt.want) {
				t.Errorf("set = %+v, want one rejected line with kinds %v", set, tt.want)
			}
		})
	}

	t.Run("first appearance", func(t *testing.T) {
		const a, b, r = "Editorial correction: a", "Editorial correction: b", "Editorial correction: @r"
		set := classifyCorrectionLines([]correctionLine{
			lineOf(commitA, 3, b), lineOf(commitA, 4, r), lineOf(commitA, 5, a),
			lineOf(commitB, 1, a), lineOf(commitB, 3, r), lineOf(commitB, 4, b),
		})
		if want := [][]byte{[]byte(b), []byte(a)}; !slices.EqualFunc(set.Accepted, want, bytes.Equal) {
			t.Errorf("Accepted = %q, want %q", set.Accepted, want)
		}
		if len(set.Rejected) != 1 || set.Rejected[0].Commit != commitA || set.Rejected[0].Line != 4 {
			t.Errorf("Rejected = %+v, want one line at %s line 4", set.Rejected, commitA)
		}
	})
}

func TestShownForm(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		kept     string // shownForm(text, false)
		mismatch string // shownForm(text, true)
	}{
		{"trailing blank", "a ", `"a "`, `"a "`},
		{"tab", "a\tb", "\"a\tb\"", `"aU+0009b"`},
		{"invisible", "a\u200bb\r", `"aU+200BbU+000D"`, `"aU+200BbU+000D"`},
		{"noncharacters", "\ufffe\U0010ffff", `"U+FFFEU+10FFFF"`, `"U+FFFEU+10FFFF"`},
		{"bytes that are not UTF-8", "a\xff\xe3\x81", `"a\xff\xe3\x81"`, `"a\xff\xe3\x81"`},
		{"printable characters", "\u65e5 @<\"", "\"\u65e5 @<\"\"", "\"\u65e5 @<\"\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shownForm([]byte(tt.text), false); got != tt.kept {
				t.Errorf("shownForm(%q, false) = %q, want %q", tt.text, got, tt.kept)
			}
			if got := shownForm([]byte(tt.text), true); got != tt.mismatch {
				t.Errorf("shownForm(%q, true) = %q, want %q", tt.text, got, tt.mismatch)
			}
		})
	}
}

// Correction lines the block and check tables share.
const (
	lineA      = "Editorial correction: docs/a.md: a; no decision changed"
	lineB      = "Editorial correction: docs/b.md: b; no decision changed"
	lineC      = "Editorial correction: docs/c.md: c; no decision changed"
	lineReject = "Editorial correction: docs/r.md: x@y; no decision changed"
)

// rejectedAt is the declaration that drops the rejected line of the tables.
var rejectedAt = RejectedDrop{Commit: commitB, Line: 3}

func dropped(disputed, reverted, rejected int) string {
	return string(droppedLine(disputed, reverted, rejected))
}

func TestCorrectionBlock(t *testing.T) {
	abc := []correctionLine{lineOf(commitA, 3, lineA), lineOf(commitA, 4, lineB), lineOf(commitB, 1, lineC)}
	withRejected := []correctionLine{lineOf(commitA, 3, lineA), lineOf(commitB, 3, lineReject)}
	tests := []struct {
		name  string
		lines []correctionLine
		drops Drops
		want  []string
	}{
		{"no line", nil, Drops{}, nil},
		{"no drop", abc, Drops{}, []string{lineA, lineB, lineC}},
		{"a disputed line", abc, Drops{Accepted: []Drop{{2, DropDisputed}}}, []string{lineA, lineC, dropped(1, 0, 0)}},
		// The dropped line's wording is the record main keeps, so this row
		// spells it out.
		{
			"a reverted and a disputed line", abc,
			Drops{Accepted: []Drop{{3, DropDisputed}, {1, DropReverted}}},
			[]string{lineB, "Dropped editorial corrections: 1 disputed by a reviewer, 1 reverted on the branch, 0 with rejected characters"},
		},
		{"a rejected line dropped", withRejected, Drops{Rejected: []RejectedDrop{rejectedAt}}, []string{lineA, dropped(0, 0, 1)}},
		{"only a rejected line", withRejected[1:], Drops{Rejected: []RejectedDrop{rejectedAt}}, []string{dropped(0, 0, 1)}},
		{"every line dropped", abc[:1], Drops{Accepted: []Drop{{1, DropReverted}}}, []string{dropped(0, 1, 0)}},
		// The same bytes in two commits are one line, so one drop removes
		// both appearances.
		{"a repeated line dropped once", append(slices.Clone(abc), lineOf(commitB, 5, lineA)), Drops{Accepted: []Drop{{1, DropDisputed}}}, []string{lineB, lineC, dropped(1, 0, 0)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := correctionBlock(classifyCorrectionLines(tt.lines), tt.drops)
			if err != nil {
				t.Fatalf("correctionBlock error = %v", err)
			}
			var want [][]byte
			var paragraph []byte
			for _, l := range tt.want {
				want = append(want, []byte(l))
				paragraph = append(paragraph, l+"\n"...)
			}
			if want != nil {
				paragraph = append(paragraph, '\n')
			}
			if !slices.EqualFunc(block, want, bytes.Equal) {
				t.Errorf("block = %q, want %q", block, want)
			}
			if got := paragraphBytes(block); !bytes.Equal(got, paragraph) {
				t.Errorf("paragraph = %q, want %q", got, paragraph)
			}
		})
	}

	t.Run("no limit on lines or length", func(t *testing.T) {
		long := correctionPrefix + strings.Repeat("x", 1<<20)
		lines := []correctionLine{lineOf(commitA, 1, long)}
		for i := range 5000 {
			lines = append(lines, lineOf(commitB, i+2, correctionPrefix+strconv.Itoa(i)))
		}
		block, err := correctionBlock(classifyCorrectionLines(lines), Drops{})
		if err != nil || len(block) != 5001 || string(block[0]) != long {
			t.Errorf("block of %d lines, first %d bytes, error %v; want 5001 lines, the first %d bytes", len(block), len(block[0]), err, len(long))
		}
	})
}

func TestParseDrop(t *testing.T) {
	for in, want := range map[string]Drop{"1:disputed": {1, DropDisputed}, "12:reverted": {12, DropReverted}} {
		if got, err := ParseDrop(in); err != nil || got != want {
			t.Errorf("ParseDrop(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, in := range []string{
		"1", ":disputed", "1:", "x:disputed", "+1:disputed", "01:disputed", "0:disputed", "-1:disputed", " 1:disputed",
		"99999999999999999999:disputed", "1:Disputed", "1:other", "1:disputed:x",
	} {
		if got, err := ParseDrop(in); !errors.Is(err, errInvalidDrop) {
			t.Errorf("ParseDrop(%q) = %+v, %v; want errInvalidDrop", in, got, err)
		}
	}
}

func TestParseRejectedDrop(t *testing.T) {
	if got, err := ParseRejectedDrop(commitB + ":3"); err != nil || got != rejectedAt {
		t.Errorf("ParseRejectedDrop = %+v, %v; want %+v", got, err, rejectedAt)
	}
	for _, in := range []string{
		commitB, commitB + ":", commitB + ":0", commitB + ":03", commitB + ":+3", commitB[1:] + ":3",
		strings.ToUpper(commitB) + ":3", strings.Repeat("b", 64) + ":3", strings.Repeat("g", 40) + ":3", ":3",
	} {
		if got, err := ParseRejectedDrop(in); !errors.Is(err, errInvalidDrop) {
			t.Errorf("ParseRejectedDrop(%q) = %+v, %v; want errInvalidDrop", in, got, err)
		}
	}
}

const (
	testSubject  = "docs(0001): fix the guide (#42)"
	testTrailer  = "Co-Authored-By: Test <test@example.com>"
	testSummary2 = "Summary of the change."
)

// bodyOf joins paragraphs, each a list of lines, with one empty line between
// them and an LF after the last line.
func bodyOf(paragraphs ...[]string) []byte {
	parts := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		parts[i] = strings.Join(p, "\n")
	}
	return []byte(strings.Join(parts, "\n\n") + "\n")
}

func TestCheckCorrections(t *testing.T) {
	ab := []correctionLine{lineOf(commitA, 3, lineA), lineOf(commitA, 4, lineB)}
	withRejected := append(slices.Clone(ab), lineOf(commitB, 3, lineReject))
	summary, trailer := []string{testSummary2}, []string{testTrailer}
	dropR := Drops{Rejected: []RejectedDrop{rejectedAt}}
	type want struct {
		missing, extra  int
		notParagraph    bool
		trailersChanged bool
		undeclared      int
		pass            bool
	}
	tests := []struct {
		name    string
		lines   []correctionLine
		drops   Drops
		subject string
		body    []byte
		want    want
		unknown []string // the UnknownDrops values
	}{
		{"block before the trailers", ab, Drops{}, testSubject, bodyOf(summary, []string{lineA, lineB}, []string{"Closes #1"}, trailer), want{pass: true}, nil},
		{"block last after Closes", ab, Drops{}, testSubject, bodyOf(summary, []string{"Closes #1"}, []string{lineA, lineB}), want{pass: true}, nil},
		{"block alone", ab, Drops{}, testSubject, bodyOf([]string{lineA, lineB}), want{pass: true}, nil},
		{"block last without a final LF", ab, Drops{}, testSubject, []byte(testSummary2 + "\n\n" + lineA + "\n" + lineB), want{pass: true}, nil},
		{"an extra last line without a final LF", ab, Drops{}, testSubject, []byte(testSummary2 + "\n\n" + lineA + "\n" + lineB + "\n" + lineC), want{extra: 1}, nil},
		{"no correction line", nil, Drops{}, testSubject, bodyOf(summary, trailer), want{pass: true}, nil},
		{"a rejected line dropped", withRejected, dropR, testSubject, bodyOf(summary, []string{lineA, lineB, dropped(0, 0, 1)}, trailer), want{pass: true}, nil},
		{"a missing line", ab, Drops{}, testSubject, bodyOf(summary, []string{lineA}, trailer), want{missing: 1}, nil},
		{"an extra line", ab, Drops{}, testSubject, bodyOf(summary, []string{lineA, lineB, lineC}, trailer), want{extra: 1}, nil},
		{"lines out of order", ab, Drops{}, testSubject, bodyOf(summary, []string{lineB, lineA}, trailer), want{missing: 1, extra: 1}, nil},
		{"a reserved line in the subject", ab, Drops{}, lineA, bodyOf(summary, []string{lineA, lineB}, trailer), want{extra: 1}, nil},
		{"the rejected line in the body", withRejected, dropR, testSubject, bodyOf(summary, []string{lineA, lineB, lineReject, dropped(0, 0, 1)}, trailer), want{extra: 1}, nil},
		{"a dropped line nothing was dropped for", ab, Drops{}, testSubject, bodyOf(summary, []string{lineA, lineB, dropped(1, 0, 0)}, trailer), want{extra: 1}, nil},
		{"a reserved line with no correction line", nil, Drops{}, testSubject, bodyOf(summary, []string{lineA}, trailer), want{extra: 1}, nil},
		{"an undeclared rejected line", withRejected, Drops{}, testSubject, bodyOf(summary, []string{lineA, lineB}, trailer), want{undeclared: 1}, nil},
		{"a number past the accepted lines", ab, Drops{Accepted: []Drop{{3, DropDisputed}}}, testSubject, bodyOf(summary, []string{lineA, lineB}, trailer), want{}, []string{"--drop 3:disputed"}},
		{"a declaration naming no rejected line", ab, dropR, testSubject, bodyOf(summary, []string{lineA, lineB}, trailer), want{}, []string{"--drop-rejected " + rejectedAt.String()}},
		// One declaration names the rejected line and one an accepted line:
		// only the second is reported.
		{
			"a declaration naming an accepted line", withRejected,
			Drops{Rejected: []RejectedDrop{rejectedAt, {Commit: commitA, Line: 3}}},
			testSubject,
			bodyOf(summary, []string{lineA, lineB, dropped(0, 0, 1)}, trailer),
			want{},
			[]string{"--drop-rejected " + commitA + ":3"},
		},
		{"no empty line before", ab, Drops{}, testSubject, bodyOf(append(slices.Clone(summary), lineA, lineB), trailer), want{notParagraph: true}, nil},
		{"a line inside", ab, Drops{}, testSubject, bodyOf(summary, []string{lineA, "between", lineB}, trailer), want{notParagraph: true}, nil},
		{"in the trailers' paragraph", ab, Drops{}, testSubject, bodyOf(summary, []string{lineA, lineB, testTrailer}), want{notParagraph: true}, nil},
		{"after the trailers", ab, Drops{}, testSubject, bodyOf(summary, trailer, []string{lineA, lineB}), want{trailersChanged: true}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := checkCorrections(t.Context(), newFakeSource(t, tt.lines...), mainBranch, testHeadOID, tt.drops, []byte(tt.subject), tt.body)
			if tt.want.pass {
				if err != nil {
					t.Fatalf("checkCorrections error = %v, want a pass", err)
				}
				if got := reservedLines(messageLines(tt.body)); !slices.EqualFunc(block, got, bytes.Equal) {
					t.Errorf("block = %q, want the body's lines %q", block, got)
				}
				return
			}
			e, ok := errors.AsType[*CorrectionMismatchError](err)
			if !ok || !errors.Is(err, errCorrectionMismatch) {
				t.Fatalf("checkCorrections error = %v, want a *CorrectionMismatchError", err)
			}
			got := want{missing: len(e.Missing), extra: len(e.Extra), notParagraph: e.NotParagraph, trailersChanged: e.TrailersChanged, undeclared: len(e.Undeclared)}
			if got != tt.want {
				t.Errorf("error = %+v, want %+v", e, tt.want)
			}
			if tt.want.undeclared > 0 && (e.Undeclared[0].Commit != commitB || e.Undeclared[0].Line != 3 || !slices.Equal(e.Undeclared[0].Kinds, []RejectKind{RejectAt})) {
				t.Errorf("Undeclared = %+v, want the line at %s line 3 with kind at", e.Undeclared, commitB)
			}
			if !slices.Equal(e.UnknownDrops, tt.unknown) {
				t.Errorf("UnknownDrops = %q, want %q", e.UnknownDrops, tt.unknown)
			}
		})
	}

	t.Run("unreadable", func(t *testing.T) {
		src := newFakeSource(t)
		src.err = fmt.Errorf("%w: rev-list failed", errCorrectionUnreadable)
		_, err := checkCorrections(t.Context(), src, "release", testOtherOID, Drops{}, []byte(testSubject), bodyOf(summary))
		if !errors.Is(err, errCorrectionUnreadable) || src.gotBase != "release" || src.gotHead != testOtherOID {
			t.Errorf("checkCorrections error = %v, read %q..%q; want errCorrectionUnreadable reading release..%s", err, src.gotBase, src.gotHead, testOtherOID)
		}
	})
	// Check 7 reads the trailers twice, with and without the block; each
	// read's failure stops on its own.
	for _, call := range []int{1, 2} {
		t.Run(fmt.Sprintf("trailer reading %d fails", call), func(t *testing.T) {
			src := newFakeSource(t, ab...)
			src.trailFailAt = call
			_, err := checkCorrections(t.Context(), src, mainBranch, testHeadOID, Drops{}, []byte(testSubject), bodyOf(summary, []string{lineA, lineB}, trailer))
			if !errors.Is(err, errCorrectionUnreadable) {
				t.Errorf("checkCorrections error = %v, want errCorrectionUnreadable", err)
			}
		})
	}
	t.Run("a line named twice", func(t *testing.T) {
		for _, drops := range []Drops{
			{Accepted: []Drop{{1, DropDisputed}, {1, DropReverted}}},
			{Rejected: []RejectedDrop{rejectedAt, rejectedAt}},
		} {
			src := newFakeSource(t, withRejected...)
			_, err := checkCorrections(t.Context(), src, mainBranch, testHeadOID, drops, []byte(testSubject), bodyOf(summary))
			if !errors.Is(err, errCorrectionMismatch) || !errors.Is(err, errInvalidDrop) || src.gotBase != "" {
				t.Errorf("checkCorrections(%+v) error = %v, read %q; want errCorrectionMismatch and errInvalidDrop before reading", drops, err, src.gotBase)
			}
		}
	})
	t.Run("shown forms", func(t *testing.T) {
		lines := []correctionLine{lineOf(commitA, 3, lineA+"\t")}
		_, err := checkCorrections(t.Context(), newFakeSource(t, lines...), mainBranch, testHeadOID, Drops{}, []byte(testSubject), bodyOf(summary, []string{lineA + "\u200b"}))
		e, ok := errors.AsType[*CorrectionMismatchError](err)
		if !ok || !slices.Equal(e.Missing, []string{`"` + lineA + `U+0009"`}) || !slices.Equal(e.Extra, []string{`"` + lineA + `U+200B"`}) {
			t.Errorf("error = %+v, want the lines in their shown forms", err)
		}
	})
}

func TestTrailerReading(t *testing.T) {
	r := newGitRepo(t)
	src := newGitCorrectionSource(t.TempDir())
	tests := []struct {
		name    string
		message string
		want    string
		// plainEmpty: plain git --parse reads nothing here, so the row
		// reads the trailer only because of --no-divider.
		plainEmpty bool
	}{
		{"same paragraph as the block", "s\n\n" + lineA + "\n" + dropped(1, 0, 0) + "\n" + testTrailer + "\n", "", false},
		{"own paragraph", "s\n\n" + lineA + "\n" + dropped(1, 0, 0) + "\n\n" + testTrailer + "\n", testTrailer + "\n", false},
		{"Signed-off-by in the block's paragraph", "s\n\n" + lineA + "\nSigned-off-by: Test <test@example.com>\n", "Signed-off-by: Test <test@example.com>\n", false},
		{"after a divider", "s\n\nintro\n---\n\n" + testTrailer + "\n", testTrailer + "\n", true},
		{"first paragraph", testTrailer + "\n", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.plainEmpty {
				if out := r.withT(t).gitInput(t.TempDir(), []byte(tt.message), "interpret-trailers", "--parse"); out != "" {
					t.Fatalf("precondition: git interpret-trailers --parse = %q, want nothing", out)
				}
			}
			got, err := src.trailers(t.Context(), []byte(tt.message))
			if err != nil || string(got) != tt.want {
				t.Errorf("trailers = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestCompareMergedLines(t *testing.T) {
	subject := []byte(testSubject)
	body := bodyOf([]string{testSummary2}, []string{lineA, dropped(1, 0, 0)}, []string{testTrailer})
	sent := [][]byte{[]byte(lineA), []byte(dropped(1, 0, 0))}
	tests := []struct {
		name           string
		merged         [][]byte
		mergedErr      error
		missing, extra []string
		detail         bool
	}{
		{"same lines", sent, nil, nil, nil, false},
		{"a missing line", sent[:1], nil, []string{`"` + dropped(1, 0, 0) + `"`}, nil, false},
		{"an extra line", append(slices.Clone(sent), []byte(lineB)), nil, nil, []string{`"` + lineB + `"`}, false},
		{"a changed line", [][]byte{[]byte(lineA + " "), sent[1]}, nil, []string{`"` + lineA + `"`}, []string{`"` + lineA + ` "`}, false},
		{"unreadable", nil, errors.New("fetch failed"), nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &fakeCorrectionSource{merged: tt.merged, mergedErr: tt.mergedErr}
			n, err := compareMergedLines(t.Context(), src, "release", testMergeOID, subject, body)
			if src.gotBase != "release" || src.gotMerge != testMergeOID {
				t.Errorf("read the merge commit %q from %q, want %s from release", src.gotMerge, src.gotBase, testMergeOID)
			}
			if tt.missing == nil && tt.extra == nil && !tt.detail {
				if err != nil || n != len(sent) {
					t.Errorf("compareMergedLines = %d, %v; want %d, nil", n, err, len(sent))
				}
				return
			}
			e, ok := errors.AsType[*CorrectionsNotCarriedError](err)
			if !ok || !errors.Is(err, errCorrectionsNotCarried) || e.MergeCommit != testMergeOID {
				t.Fatalf("compareMergedLines error = %v, want a *CorrectionsNotCarriedError for %s", err, testMergeOID)
			}
			if !slices.Equal(e.Missing, tt.missing) || !slices.Equal(e.Extra, tt.extra) || (e.Detail != "") != tt.detail {
				t.Errorf("error = %+v, want missing %q, extra %q, detail %v", e, tt.missing, tt.extra, tt.detail)
			}
		})
	}
}

// The words and prefixes below are what the reports print and the command
// documents match, so the text is the contract.
func TestReportWords(t *testing.T) {
	words := map[RejectKind]string{RejectInvisible: "invisible", RejectInvalidUTF8: "invalid-utf8", RejectAt: "at", RejectLessThan: "lt"}
	for k, want := range words {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
	if got := (&CorrectionMismatchError{Missing: []string{"x"}}).Error(); !strings.HasPrefix(got, "correction check: ") {
		t.Errorf("CorrectionMismatchError = %q, want the prefix correction check: ", got)
	}
	notCarried := (&CorrectionsNotCarriedError{MergeCommit: testMergeOID, Extra: []string{"x"}}).Error()
	if !strings.HasPrefix(notCarried, "post-merge check: ") || !strings.Contains(notCarried, testMergeOID) {
		t.Errorf("CorrectionsNotCarriedError = %q, want the prefix post-merge check: and the merge commit", notCarried)
	}
}

// isExitError reports whether err wraps a command's exit status.
func isExitError(err error) bool {
	_, ok := errors.AsType[*cmdrunner.ExitError](err)
	return ok
}

func TestTrailerReadingRefusesUnlistableParent(t *testing.T) {
	// A parent holding the list separator would be split into two
	// GIT_CEILING_DIRECTORIES entries, so the reading stops instead.
	parent := filepath.Join(t.TempDir(), "a"+string(filepath.ListSeparator)+"b")
	if err := os.MkdirAll(parent, testRepoDirMode); err != nil {
		t.Fatalf("create %s: %v", parent, err)
	}
	src := newGitCorrectionSource(t.TempDir())
	src.tempParent = parent
	_, err := src.trailers(t.Context(), []byte("s\n\n"+testTrailer+"\n"))
	if !errors.Is(err, errCorrectionUnreadable) || !errors.Is(err, errCeilingNotListable) {
		t.Errorf("trailers error = %v, want errCorrectionUnreadable and errCeilingNotListable", err)
	}
	if left, _ := filepath.Glob(filepath.Join(parent, "mergepr-trailers-*")); len(left) != 0 {
		t.Errorf("left behind %v, want the directory removed", left)
	}
}
