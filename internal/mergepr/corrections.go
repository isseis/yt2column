package mergepr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The squash body carries the editorial correction lines of the PR's commits
// (the lines a commit message records an editorial correction of a task
// document or an ADR with). This file reads them from the commit objects,
// sorts them into accepted and rejected lines, builds the block of lines the
// body must hold, checks a subject and body against it before the merge, and
// compares the merged commit with the body after it.

const (
	// correctionPrefix starts a correction line: in the first column, case
	// sensitive, with one space after the colon. The rest of the line has no
	// required form.
	correctionPrefix = "Editorial correction: "
	// droppedPrefix starts the line that records how many correction lines
	// were dropped, and under which case.
	droppedPrefix = "Dropped editorial corrections: "
)

// reservedPrefixes are the prefixes whose lines in a subject or body must be
// exactly the correction block.
var reservedPrefixes = [][]byte{[]byte(correctionPrefix), []byte(droppedPrefix)}

// correctionLine is one correction line as the commit object stores it.
type correctionLine struct {
	Commit string // full object ID
	Line   int    // 1-based line number in the commit message; the subject is 1
	Text   []byte // the line without its LF
}

// correctionSource reads correction lines from git. The production source
// runs git without the GIT_* variables and with cmdrunner.GitHardening, and
// refuses a shallow repository; tests substitute a fake.
type correctionSource interface {
	// rangeLines resolves origin/<baseRefName> and headOID to object IDs and
	// returns every correction line of the commits reachable from the head
	// and not from the base, in range order. Every error wraps
	// errCorrectionUnreadable.
	rangeLines(ctx context.Context, baseRefName, headOID string) ([]correctionLine, error)
	// mergedLines fetches baseRefName from origin and returns the lines of
	// the merge commit's message that start with a reserved prefix.
	mergedLines(ctx context.Context, baseRefName, mergeOID string) ([][]byte, error)
	// trailers returns what git interpret-trailers --parse --no-divider
	// prints for message, with no configuration of the user or of a
	// repository applied. Every error wraps errCorrectionUnreadable.
	trailers(ctx context.Context, message []byte) ([]byte, error)
}

// gitCorrectionSource is the production correctionSource.
type gitCorrectionSource struct {
	reader gitReader
	// env is the environment the trailer reading narrows (newTrailerRunner).
	env []string
	// tempParent is the directory the trailer reading makes its empty
	// directory in; empty means os.TempDir. Only a test changes it.
	tempParent string
}

// newGitCorrectionSource returns the production source reading the
// repository in dir (the current directory when empty).
func newGitCorrectionSource(dir string) *gitCorrectionSource {
	return &gitCorrectionSource{reader: gitReader{run: newGitRunner(dir)}, env: gitEnviron()}
}

// rangeLines implements correctionSource. Contract: the message is read from
// the commit object as stored (git cat-file commit, never git log, which
// re-encodes it), past the first empty line of the object; a failure on any
// commit returns no line at all, never the lines read before it.
func (s *gitCorrectionSource) rangeLines(ctx context.Context, baseRefName, headOID string) ([]correctionLine, error) {
	_, commits, err := s.reader.listRange(ctx, baseRefName, headOID, rangeOrderTopo)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCorrectionUnreadable, err)
	}
	var lines []correctionLine
	for _, c := range commits {
		message, err := s.reader.commitMessage(ctx, c.oid)
		if err != nil {
			return nil, fmt.Errorf("%w: commit %s: %w", errCorrectionUnreadable, c.oid, err)
		}
		for i, text := range messageLines(message) {
			if bytes.HasPrefix(text, []byte(correctionPrefix)) {
				lines = append(lines, correctionLine{Commit: c.oid, Line: i + 1, Text: text})
			}
		}
	}
	return lines, nil
}

// mergedLines implements correctionSource. Contract: the fetch refspec is
// built only from a base name git check-ref-format accepts, and the message
// is read from the commit object as stored.
func (s *gitCorrectionSource) mergedLines(ctx context.Context, baseRefName, mergeOID string) ([][]byte, error) {
	if !isObjectID(mergeOID) {
		return nil, fmt.Errorf("%w: %q", errMergeNotObjectID, mergeOID)
	}
	ref, err := s.reader.baseRef(ctx, baseRefName)
	if err != nil {
		return nil, err
	}
	if _, err := s.reader.git(ctx, "fetch", "origin", "+refs/heads/"+baseRefName+":"+ref); err != nil {
		return nil, fmt.Errorf("fetch %s: %w", ref, err)
	}
	message, err := s.reader.commitMessage(ctx, mergeOID)
	if err != nil {
		return nil, fmt.Errorf("commit %s: %w", mergeOID, err)
	}
	return reservedLines(messageLines(message)), nil
}

// trailers implements correctionSource. Contract: git runs in an empty
// directory made for this call and removed before it returns, outside any
// repository, with no system, global, or repository configuration
// (newTrailerRunner), and reads the message from its standard input, so no
// temporary file holds it.
func (s *gitCorrectionSource) trailers(ctx context.Context, message []byte) (out []byte, err error) {
	dir, err := os.MkdirTemp(s.tempParent, "mergepr-trailers-")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCorrectionUnreadable, err)
	}
	defer func() {
		if rmErr := os.RemoveAll(dir); rmErr != nil && err == nil {
			out, err = nil, fmt.Errorf("%w: remove %s: %w", errCorrectionUnreadable, dir, rmErr)
		}
	}()
	args := append(cmdrunner.GitHardening(), "interpret-trailers", "--parse", "--no-divider")
	out, err = newTrailerRunner(dir, s.env).RunInput(ctx, message, gitCommand, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: git interpret-trailers: %w", errCorrectionUnreadable, err)
	}
	return out, nil
}

// commitMessage returns the message of the commit oid as the object stores
// it: every byte after the first empty line, the line that ends the header
// (a gpgsig or mergetag header's lines start with a space, so they never
// end it). An object without that empty line is an error.
func (g gitReader) commitMessage(ctx context.Context, oid string) ([]byte, error) {
	object, err := g.git(ctx, "cat-file", "commit", oid)
	if err != nil {
		return nil, err
	}
	_, message, ok := bytes.Cut(object, []byte("\n\n"))
	if !ok {
		return nil, fmt.Errorf("%w: commit %s has no empty line after its header", errUnexpectedGitOutput, oid)
	}
	return message, nil
}

// messageLines splits a message at each LF; the first line is the subject.
// A carriage return before an LF stays in its line. A final LF ends the last
// line and starts no empty line after it.
func messageLines(message []byte) [][]byte {
	if len(message) == 0 {
		return nil
	}
	return bytes.Split(bytes.TrimSuffix(message, []byte("\n")), []byte("\n"))
}

// hasReservedPrefix reports whether line starts in its first column with a
// reserved prefix.
func hasReservedPrefix(line []byte) bool {
	for _, p := range reservedPrefixes {
		if bytes.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// reservedLines returns the lines that start with a reserved prefix, in
// order.
func reservedLines(lines [][]byte) [][]byte {
	var found [][]byte
	for _, l := range lines {
		if hasReservedPrefix(l) {
			found = append(found, l)
		}
	}
	return found
}

// RejectKind is a kind of character a correction line may not hold. String
// returns the report word: "invisible", "invalid-utf8", "at", or "lt". The
// zero value is not a valid kind.
type RejectKind int

// The kinds of rejected character, in the order a RejectedLine lists them.
const (
	_                 RejectKind = iota // the zero value, never a valid kind
	RejectInvisible                     // general category Cc other than tab, Cf, Zl, Zp
	RejectInvalidUTF8                   // a byte sequence that is not UTF-8, or a noncharacter
	RejectAt                            // '@'
	RejectLessThan                      // '<'
)

// String returns the word the reports use.
func (k RejectKind) String() string {
	switch k {
	case RejectInvisible:
		return "invisible"
	case RejectInvalidUTF8:
		return "invalid-utf8"
	case RejectAt:
		return "at"
	case RejectLessThan:
		return "lt"
	default:
		return invalidName
	}
}

// RejectedLine is a correction line that holds a rejected character, at the
// position where it first appears in range order.
type RejectedLine struct {
	Commit string       // full object ID
	Line   int          // 1-based line number in the commit message
	Kinds  []RejectKind // in the order of the constants, each once
	Shown  string       // the shown form, tabs kept as they are
}

// isNoncharacter reports whether r is one of Unicode's noncharacters:
// U+FDD0 to U+FDEF, and the last two code points of each plane. git commit
// rewrites them as not UTF-8, so they are rejected with invalid UTF-8.
func isNoncharacter(r rune) bool {
	return (r >= 0xFDD0 && r <= 0xFDEF) || r&0xFFFE == 0xFFFE
}

// isInvisible reports whether r is a character a reader cannot see: general
// category Cc other than tab, Cf, Zl, or Zp, by the unicode tables of the Go
// release that builds the tool.
func isInvisible(r rune) bool {
	if r == '\t' {
		return false
	}
	return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp)
}

// rejectKinds returns the kinds of rejected character text holds, in the
// order of the constants, each once; none for an accepted line. Contract: it
// only classifies; it never removes or replaces a character.
func rejectKinds(text []byte) []RejectKind {
	found := make(map[RejectKind]struct{})
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRune(text[i:])
		i += size
		switch {
		case r == utf8.RuneError && size == 1, isNoncharacter(r):
			found[RejectInvalidUTF8] = struct{}{}
		case isInvisible(r):
			found[RejectInvisible] = struct{}{}
		case r == '@':
			found[RejectAt] = struct{}{}
		case r == '<':
			found[RejectLessThan] = struct{}{}
		}
	}
	kinds := make([]RejectKind, 0, len(found))
	for _, k := range []RejectKind{RejectInvisible, RejectInvalidUTF8, RejectAt, RejectLessThan} {
		if _, ok := found[k]; ok {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

// shownForm returns the line as the tool prints it on a terminal: inside
// double quotes, so trailing blanks show; an invisible character or a
// noncharacter as U+XXXX; a byte that is not UTF-8 as \xNN; every other
// character, '@' and '<' included, as it is. A tab stays a tab unless
// showTab, which the mismatch reports use, turns it into U+0009 too.
func shownForm(text []byte, showTab bool) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRune(text[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, text[i])
		case r == '\t' && !showTab:
			b.WriteRune(r)
		case r == '\t', isInvisible(r), isNoncharacter(r):
			fmt.Fprintf(&b, "U+%04X", r)
		default:
			b.Write(text[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
	return b.String()
}

// shownForms returns the mismatch-report shown form of each line.
func shownForms(lines [][]byte) []string {
	shown := make([]string, len(lines))
	for i, l := range lines {
		shown[i] = shownForm(l, true)
	}
	return shown
}

// correctionSet is the range's correction lines sorted for the block.
type correctionSet struct {
	// Accepted are the accepted lines; line n of corrections.txt, and the
	// number a --drop names, is Accepted[n-1].
	Accepted [][]byte
	// Rejected are the rejected lines, in range order.
	Rejected []RejectedLine
}

// classifyCorrectionLines keeps the first appearance of each byte sequence
// in range order and sorts the lines into accepted and rejected ones.
// Contract: a line is compared as bytes only, and a line that holds a
// rejected character is never accepted in a changed form.
func classifyCorrectionLines(lines []correctionLine) correctionSet {
	var set correctionSet
	seen := make(map[string]struct{})
	for _, l := range lines {
		if _, ok := seen[string(l.Text)]; ok {
			continue
		}
		seen[string(l.Text)] = struct{}{}
		if kinds := rejectKinds(l.Text); len(kinds) > 0 {
			set.Rejected = append(set.Rejected, RejectedLine{Commit: l.Commit, Line: l.Line, Kinds: kinds, Shown: shownForm(l.Text, false)})
			continue
		}
		set.Accepted = append(set.Accepted, l.Text)
	}
	return set
}

// DropCase is the case under which an accepted correction line is dropped at
// the approval question. The zero value is not a valid case.
type DropCase int

// The cases under which an accepted line may be dropped.
const (
	_            DropCase = iota // the zero value, never a valid case
	DropDisputed                 // --drop N:disputed: a reviewer disputed that it is an editorial correction
	DropReverted                 // --drop N:reverted: reverted later on the branch and still reverted at the head
)

// dropCaseWords are the words a --drop value names its case with.
var dropCaseWords = map[string]DropCase{"disputed": DropDisputed, "reverted": DropReverted}

// String returns the word a --drop value names the case with.
func (c DropCase) String() string {
	for word, dc := range dropCaseWords {
		if dc == c {
			return word
		}
	}
	return invalidName
}

// Drop declares that accepted correction line Index (1-based, the line
// number in corrections.txt) is dropped under Case.
type Drop struct {
	Index int
	Case  DropCase
}

// String returns the --drop value.
func (d Drop) String() string { return strconv.Itoa(d.Index) + ":" + d.Case.String() }

// RejectedDrop declares that the rejected line first appearing at Commit's
// message line Line is dropped.
type RejectedDrop struct {
	Commit string
	Line   int
}

// String returns the --drop-rejected value.
func (d RejectedDrop) String() string { return d.Commit + ":" + strconv.Itoa(d.Line) }

// Drops is the whole declaration a caller passes to Paragraph, CheckBody,
// and Merge. Its zero value drops nothing.
type Drops struct {
	Accepted []Drop
	Rejected []RejectedDrop
}

// Validate rejects a declaration that names the same accepted line twice
// (under the same case or not) or the same rejected line twice, so a caller
// can stop before reading anything.
func (d Drops) Validate() error {
	indexes := make(map[int]struct{})
	for _, a := range d.Accepted {
		if _, ok := indexes[a.Index]; ok {
			return fmt.Errorf("%w: line %d is named twice", errInvalidDrop, a.Index)
		}
		indexes[a.Index] = struct{}{}
	}
	rejected := make(map[RejectedDrop]struct{})
	for _, r := range d.Rejected {
		if _, ok := rejected[r]; ok {
			return fmt.Errorf("%w: %s is named twice", errInvalidDrop, r)
		}
		rejected[r] = struct{}{}
	}
	return nil
}

// ParseDrop parses one --drop value, <number>:<case>. Contract: the number
// is a positive decimal without a sign or a leading zero that fits an int,
// and the case is "disputed" or "reverted"; any other form is
// errInvalidDrop, never read more widely.
func ParseDrop(s string) (Drop, error) {
	number, word, ok := strings.Cut(s, ":")
	if !ok {
		return Drop{}, fmt.Errorf("%w: %q has no ':'", errInvalidDrop, s)
	}
	index, err := parseLineNumber(number)
	if err != nil {
		return Drop{}, fmt.Errorf("%w: %q: %w", errInvalidDrop, s, err)
	}
	c, ok := dropCaseWords[word]
	if !ok {
		return Drop{}, fmt.Errorf("%w: %q names no case (disputed or reverted)", errInvalidDrop, s)
	}
	return Drop{Index: index, Case: c}, nil
}

// ParseRejectedDrop parses one --drop-rejected value, <commit>:<line>.
// Contract: the commit is 40 lowercase hexadecimal digits and the line
// number is read as ParseDrop reads its number; any other form is
// errInvalidDrop.
func ParseRejectedDrop(s string) (RejectedDrop, error) {
	commit, number, ok := strings.Cut(s, ":")
	if !ok {
		return RejectedDrop{}, fmt.Errorf("%w: %q has no ':'", errInvalidDrop, s)
	}
	if len(commit) != 40 || !isObjectID(commit) {
		return RejectedDrop{}, fmt.Errorf("%w: %q is not a 40-digit lowercase hexadecimal commit ID", errInvalidDrop, commit)
	}
	line, err := parseLineNumber(number)
	if err != nil {
		return RejectedDrop{}, fmt.Errorf("%w: %q: %w", errInvalidDrop, s, err)
	}
	return RejectedDrop{Commit: commit, Line: line}, nil
}

// parseLineNumber reads a positive decimal of ASCII digits without a sign or
// a leading zero that fits an int.
func parseLineNumber(s string) (int, error) {
	if s == "" || s[0] < '1' || s[0] > '9' || strings.Trim(s, "0123456789") != "" {
		return 0, errNotLineNumber
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errNotLineNumber
	}
	return n, nil
}

// droppedLine returns the line that records the dropped lines, by case.
func droppedLine(disputed, reverted, rejected int) []byte {
	return fmt.Appendf(nil, "%s%d disputed by a reviewer, %d reverted on the branch, %d with rejected characters", droppedPrefix, disputed, reverted, rejected)
}

// correctionBlock applies drops to set and returns the block the body must
// hold: the accepted lines not dropped, in number order, then the dropped
// line when at least one line is dropped. Contract: a drop that names no
// line and a rejected line no drop declares stop with a
// *CorrectionMismatchError, so the block never silently leaves out a line
// or takes in a rejected one.
func correctionBlock(set correctionSet, drops Drops) ([][]byte, error) {
	if err := drops.Validate(); err != nil {
		return nil, err
	}
	mismatch := &CorrectionMismatchError{}
	dropped := make(map[int]struct{})
	var disputed, reverted int
	for _, d := range drops.Accepted {
		if d.Index < 1 || d.Index > len(set.Accepted) {
			mismatch.UnknownDrops = append(mismatch.UnknownDrops, "--drop "+d.String())
			continue
		}
		switch d.Case {
		case DropDisputed:
			disputed++
		case DropReverted:
			reverted++
		default:
			mismatch.UnknownDrops = append(mismatch.UnknownDrops, "--drop "+d.String())
			continue
		}
		dropped[d.Index] = struct{}{}
	}
	declared := make(map[RejectedDrop]struct{})
	for _, d := range drops.Rejected {
		declared[d] = struct{}{}
	}
	matched := 0
	for _, r := range set.Rejected {
		if _, ok := declared[RejectedDrop{Commit: r.Commit, Line: r.Line}]; ok {
			matched++
			continue
		}
		mismatch.Undeclared = append(mismatch.Undeclared, r)
	}
	if matched < len(declared) {
		named := make(map[RejectedDrop]struct{})
		for _, r := range set.Rejected {
			named[RejectedDrop{Commit: r.Commit, Line: r.Line}] = struct{}{}
		}
		for _, d := range drops.Rejected {
			if _, ok := named[d]; !ok {
				mismatch.UnknownDrops = append(mismatch.UnknownDrops, "--drop-rejected "+d.String())
			}
		}
	}
	if len(mismatch.UnknownDrops) > 0 || len(mismatch.Undeclared) > 0 {
		return nil, mismatch
	}
	var block [][]byte
	for i, line := range set.Accepted {
		if _, ok := dropped[i+1]; !ok {
			block = append(block, line)
		}
	}
	if len(dropped)+matched > 0 {
		block = append(block, droppedLine(disputed, reverted, matched))
	}
	return block, nil
}

// paragraphBytes returns the content of paragraph.txt for block: each line
// ended by an LF and one more LF after the last, or no byte at all for an
// empty block, so that joining a summary that ends in an empty line, these
// bytes, and the rest of the body leaves exactly one empty line between
// paragraphs either way.
func paragraphBytes(block [][]byte) []byte {
	if len(block) == 0 {
		return nil
	}
	var b bytes.Buffer
	for _, line := range block {
		b.Write(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.Bytes()
}

// correctionParagraph runs checks 1 to 3 of the correction check: it reads
// the range's correction lines again from git, applies drops, and returns
// the expected block. paragraph uses it alone; checkCorrections starts with
// it, so both go through the same code. Contract: drops are validated
// before anything is read, and a read that fails stops with
// errCorrectionUnreadable instead of returning a shorter block.
func correctionParagraph(ctx context.Context, src correctionSource, baseRefName, headOID string, drops Drops) ([][]byte, error) {
	if err := drops.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", errCorrectionMismatch, err)
	}
	lines, err := src.rangeLines(ctx, baseRefName, headOID)
	if err != nil {
		return nil, err
	}
	return correctionBlock(classifyCorrectionLines(lines), drops)
}

// checkCorrections runs checks 1 to 7 of the correction check on the
// subject and the body the merge passes to GitHub, and returns the expected
// block. Contract: it passes only when the subject has no reserved-prefix
// line, the body's reserved-prefix lines are the block byte for byte and in
// order, they form one paragraph of their own, and putting them in the body
// does not change the trailers git reads from the message; otherwise it
// stops with a *CorrectionMismatchError, or with errCorrectionUnreadable
// when git could not be read.
func checkCorrections(ctx context.Context, src correctionSource, baseRefName, headOID string, drops Drops, subject, body []byte) ([][]byte, error) {
	block, err := correctionParagraph(ctx, src, baseRefName, headOID, drops)
	if err != nil {
		return nil, err
	}
	mismatch := &CorrectionMismatchError{}
	// Check 4: the subject is part of the merge commit's message too.
	mismatch.Extra = shownForms(reservedLines(messageLines(subject)))
	// Check 5: the body's reserved-prefix lines are the block.
	spans := lineSpans(body)
	var found [][]byte
	var at []int
	for i, s := range spans {
		if line := body[s.start:s.text]; hasReservedPrefix(line) {
			found = append(found, line)
			at = append(at, i)
		}
	}
	missing, extra := lineDiff(block, found)
	mismatch.Missing = shownForms(missing)
	mismatch.Extra = append(mismatch.Extra, shownForms(extra)...)
	// Check 6: they form one paragraph of their own.
	mismatch.NotParagraph = !oneParagraph(spans, at)
	if len(mismatch.Missing) > 0 || len(mismatch.Extra) > 0 || mismatch.NotParagraph {
		return nil, mismatch
	}
	if len(block) == 0 {
		return block, nil
	}
	// Check 7: the block leaves the trailers git reads as they are.
	changed, err := trailersChanged(ctx, src, subject, body, spans, at)
	if err != nil {
		return nil, err
	}
	if changed {
		return nil, &CorrectionMismatchError{TrailersChanged: true}
	}
	return block, nil
}

// lineSpan is one line of a body: text is where its LF starts (or the end
// of the body), end where the next line starts.
type lineSpan struct {
	start, text, end int
}

// lineSpans splits body into its lines as messageLines does, with offsets.
func lineSpans(body []byte) []lineSpan {
	var spans []lineSpan
	for start := 0; start < len(body); {
		i := bytes.IndexByte(body[start:], '\n')
		if i < 0 {
			spans = append(spans, lineSpan{start: start, text: len(body), end: len(body)})
			break
		}
		spans = append(spans, lineSpan{start: start, text: start + i, end: start + i + 1})
		start += i + 1
	}
	return spans
}

// oneParagraph reports whether the lines at the indexes in at are one run
// of consecutive lines with the start of the body or an empty line before
// it and the end of the body or an empty line after it. No line is a
// paragraph of nothing, which holds.
func oneParagraph(spans []lineSpan, at []int) bool {
	if len(at) == 0 {
		return true
	}
	first, last := at[0], at[len(at)-1]
	if last-first+1 != len(at) {
		return false
	}
	empty := func(i int) bool { return spans[i].start == spans[i].text }
	if first > 0 && !empty(first-1) {
		return false
	}
	return last+1 >= len(spans) || empty(last+1)
}

// trailersChanged reports whether git reads other trailers from the message
// (subject, an empty line, and the body) than from the same message without
// the block's paragraph. Contract: the subject goes first because git reads
// a message's first paragraph as its subject, never as trailers; the
// paragraph is removed with the empty line that separates it, so the other
// paragraphs keep their shape.
func trailersChanged(ctx context.Context, src correctionSource, subject, body []byte, spans []lineSpan, at []int) (bool, error) {
	first, last := at[0], at[len(at)-1]
	cutStart, cutEnd := spans[first].start, spans[last].end
	switch {
	case last+1 < len(spans):
		cutEnd = spans[last+1].end
	case first > 0:
		cutStart = spans[first-1].start
	}
	without := append(slices.Clone(body[:cutStart]), body[cutEnd:]...)
	message := func(b []byte) []byte {
		return slices.Concat(subject, []byte("\n\n"), b)
	}
	with, err := src.trailers(ctx, message(body))
	if err != nil {
		return false, err
	}
	reference, err := src.trailers(ctx, message(without))
	if err != nil {
		return false, err
	}
	return !bytes.Equal(with, reference), nil
}

// lineDiff compares got with want, line by line as bytes: missing are the
// lines of want and extra the lines of got that are outside a longest
// common subsequence of the two, so a line out of order shows as both.
func lineDiff(want, got [][]byte) (missing, extra [][]byte) {
	if slices.EqualFunc(want, got, bytes.Equal) {
		return nil, nil
	}
	// lcs[i][j] is the length of a longest common subsequence of want[i:]
	// and got[j:].
	lcs := make([][]int, len(want)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(got)+1)
	}
	for i := range slices.Backward(want) {
		for j := range slices.Backward(got) {
			if bytes.Equal(want[i], got[j]) {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < len(want) && j < len(got) {
		switch {
		case bytes.Equal(want[i], got[j]):
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			missing = append(missing, want[i])
			i++
		default:
			extra = append(extra, got[j])
			j++
		}
	}
	return append(missing, want[i:]...), append(extra, got[j:]...)
}

// compareMergedLines is the post-merge comparison: it reads the merge
// commit's reserved-prefix lines (fetching the base from origin first) and
// compares them, as bytes and in order, with those of the subject and body
// the merge passed to GitHub. It returns how many lines matched. Contract:
// the comparison is with the bytes the merge checked and sent, never with a
// file read again, and a commit that cannot be read is reported, never
// taken as a match.
func compareMergedLines(ctx context.Context, src correctionSource, baseRefName, mergeOID string, subject, body []byte) (int, error) {
	sent := reservedLines(slices.Concat(messageLines(subject), messageLines(body)))
	merged, err := src.mergedLines(ctx, baseRefName, mergeOID)
	if err != nil {
		return 0, &CorrectionsNotCarriedError{MergeCommit: mergeOID, Detail: err.Error()}
	}
	missing, extra := lineDiff(sent, merged)
	if len(missing) > 0 || len(extra) > 0 {
		return 0, &CorrectionsNotCarriedError{MergeCommit: mergeOID, Missing: shownForms(missing), Extra: shownForms(extra)}
	}
	return len(sent), nil
}
