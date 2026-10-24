// Command _material_check compares the material the evaluation path (the CLI
// and yt-dlp's cache) gives the generation prompt with the material the
// production path (the browser extension and the server) gives it, for the
// five videos of task 0018 step 5a (02_evaluation.md §3.11).
//
// It reads the evaluation-path material from a copy of the evaluation cache
// with YtDlpSource, configured with a yt-dlp path that always fails so a cache
// miss is never fetched again, and the production-path material from the files
// the human saved with the console expression of §3.11, parsed with
// NewCopiedSource. It prints only the kind and amount of each difference and
// short quotes, never the full text of either material.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/strictjson"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// videoIDs are the five videos of 0008 evaluation record §2.1.
var videoIDs = []string{
	"roNZ-rLxhUE",
	"18tbwwPM91A",
	"zW1RBacSqjk",
	"leE02uqgaKg",
	"EqnGQDU71-Y",
}

const (
	// maxMaterialBytes bounds a saved material file; the longest transcript of
	// the five videos is well under 1 MiB.
	maxMaterialBytes = 8 << 20
	// quoteRunes bounds each side of a quoted difference.
	quoteRunes = 40
	// maxQuotedHunks bounds the hunks quoted per value.
	maxQuotedHunks = 3
)

// pageState names which description state a material file must hold.
type pageState int

const (
	stateCollapsed pageState = iota
	stateExpanded
)

// material is one saved console capture. A metadata member is absent when its
// element was absent; the extension would also leave out a blank value.
type material struct {
	url                  string
	hl                   string
	gl                   string
	selection            string
	title                *string
	channelName          *string
	description          *string
	descriptionCollapsed bool
}

var (
	errMaterialShape = errors.New("material file has an unexpected shape")
	errMaterialState = errors.New("material file holds the other description state")
	errMaterialVideo = errors.New("material file was captured on another video")
)

func main() {
	cacheDir := flag.String("cache", "", "copy of the evaluation cache (material/cache)")
	materialDir := flag.String("material", "", "directory holding <video ID>/{collapsed,expanded}.json")
	flag.Parse()
	if *cacheDir == "" || *materialDir == "" {
		fmt.Fprintln(os.Stderr, "usage: _material_check -cache <dir> -material <dir>")
		os.Exit(2)
	}
	failed := false
	for _, id := range videoIDs {
		if err := checkVideo(os.Stdout, *cacheDir, *materialDir, id); err != nil {
			fmt.Fprintf(os.Stdout, "error: %v\n\n", err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func checkVideo(w io.Writer, cacheDir, materialDir, id string) error {
	fmt.Fprintf(w, "## %s\n\n", id)
	eval, err := readEvaluation(cacheDir, id)
	if err != nil {
		return fmt.Errorf("evaluation path: %w", err)
	}
	collapsed, err := readMaterial(filepath.Join(materialDir, id, "collapsed.json"), id, stateCollapsed)
	if err != nil {
		return fmt.Errorf("collapsed.json: %w", err)
	}
	expanded, err := readMaterial(filepath.Join(materialDir, id, "expanded.json"), id, stateExpanded)
	if err != nil {
		return fmt.Errorf("expanded.json: %w", err)
	}
	fmt.Fprintf(w, "- display: hl=%s gl=%s (collapsed capture), hl=%s gl=%s (expanded capture)\n",
		collapsed.hl, collapsed.gl, expanded.hl, expanded.gl)

	// The extension sends the selection with whichever description state the
	// page is in; the selection of the collapsed capture is the one taken.
	prod, err := transcript.NewCopiedSource(transcript.CopiedInput{
		VideoID:     id,
		Raw:         collapsed.selection,
		Title:       derefOrEmpty(collapsed.title),
		ChannelName: derefOrEmpty(collapsed.channelName),
		Description: derefOrEmpty(collapsed.description),
	})
	if err != nil {
		if errors.Is(err, transcript.ErrCopiedTranscriptFormat) {
			fmt.Fprintf(w, "- transcript: REJECTED by NewCopiedSource (ErrCopiedTranscriptFormat): %v\n\n", err)
			return nil
		}
		return fmt.Errorf("NewCopiedSource: %w", err)
	}
	videoURL, _ := transcript.NormalizedVideoURL(id)
	got, err := prod.Fetch(context.Background(), videoURL)
	if err != nil {
		return fmt.Errorf("CopiedSource.Fetch: %w", err)
	}

	fmt.Fprintf(w, "- segments: evaluation %d (%s..%s), production %d (%s..%s)\n",
		len(eval.Segments), firstStart(eval.Segments), lastStart(eval.Segments),
		len(got.Segments), firstStart(got.Segments), lastStart(got.Segments))
	reportBoundaries(w, eval.Segments, got.Segments)
	compare(w, "transcript", joinSegments(eval.Segments), joinSegments(got.Segments))
	compareOptional(w, "title", eval.Title, collapsed.title)
	compareOptional(w, "channel name", eval.ChannelName, collapsed.channelName)
	if collapsed.channelName != nil {
		if err := writer.CheckChannelName(*collapsed.channelName); err != nil {
			fmt.Fprintf(w, "  - the server would reject this channel name: %v\n", err)
		}
	}
	compareOptional(w, "description (collapsed)", eval.Description, collapsed.description)
	compareOptional(w, "description (expanded)", eval.Description, expanded.description)
	fmt.Fprintln(w)
	return nil
}

// readEvaluation reads the cached transcript as the CLI does. The yt-dlp path
// always fails, so a missing cache entry is an error instead of a fetch.
func readEvaluation(cacheDir, id string) (transcript.Transcript, error) {
	source, err := transcript.NewYtDlpSource(transcript.Options{
		CacheDir:  cacheDir,
		YtDlpPath: "/usr/bin/false",
		Timeout:   time.Minute,
	})
	if err != nil {
		return transcript.Transcript{}, err
	}
	videoURL, _ := transcript.NormalizedVideoURL(id)
	return source.Fetch(context.Background(), videoURL)
}

// readMaterial parses one console capture. Every member the expression always
// writes is required; the three metadata members are optional because the
// expression leaves out a member whose element is absent.
func readMaterial(path, id string, want pageState) (material, error) {
	data, err := readBounded(path)
	if err != nil {
		return material{}, err
	}
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return material{}, err
	}
	members, err := object.CollectOnly("url", "hl", "gl", "selection", "title", "channelName", "description", "descriptionCollapsed")
	if err != nil {
		return material{}, err
	}
	var m material
	if m.url, err = strictjson.RequiredString(members, "url"); err != nil {
		return material{}, err
	}
	if gotID, _, err := transcript.ValidateVideoURL(m.url); err != nil || gotID != id {
		return material{}, errMaterialVideo
	}
	if m.hl, err = strictjson.RequiredString(members, "hl"); err != nil {
		return material{}, err
	}
	if m.gl, err = strictjson.RequiredString(members, "gl"); err != nil {
		return material{}, err
	}
	selection, err := strictjson.Required(members, "selection")
	if err != nil {
		return material{}, err
	}
	if m.selection, err = selection.AsString(); err != nil {
		return material{}, err
	}
	for key, dst := range map[string]**string{"title": &m.title, "channelName": &m.channelName, "description": &m.description} {
		value, ok := members[key]
		if !ok {
			continue
		}
		text, err := value.AsString()
		if err != nil {
			return material{}, fmt.Errorf("%s: %w", key, err)
		}
		*dst = &text
	}
	collapsedValue, err := strictjson.Required(members, "descriptionCollapsed")
	if err != nil {
		return material{}, err
	}
	if m.descriptionCollapsed, err = collapsedValue.AsBool(); err != nil {
		return material{}, err
	}
	switch want {
	case stateCollapsed:
		if !m.descriptionCollapsed {
			return material{}, errMaterialState
		}
	case stateExpanded:
		if m.descriptionCollapsed {
			return material{}, errMaterialState
		}
	default:
		return material{}, errMaterialShape
	}
	return m, nil
}

func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxMaterialBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxMaterialBytes {
		return nil, fmt.Errorf("%w: larger than %d bytes", errMaterialShape, maxMaterialBytes)
	}
	return data, nil
}

// joinSegments joins the segment texts the way internal/writer puts them into
// the prompt: every Segment.Text, unmodified and in order, joined by one "\n".
func joinSegments(segments []transcript.Segment) string {
	texts := make([]string, len(segments))
	for i, seg := range segments {
		texts[i] = seg.Text
	}
	return strings.Join(texts, "\n")
}

func firstStart(segments []transcript.Segment) string {
	if len(segments) == 0 {
		return "-"
	}
	return clock(segments[0].StartMs)
}

func lastStart(segments []transcript.Segment) string {
	if len(segments) == 0 {
		return "-"
	}
	return clock(segments[len(segments)-1].StartMs)
}

func clock(ms int64) string {
	s := ms / 1000
	return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// compareOptional reports a production value the extension would not send: an
// absent element, or a blank value, which acceptMetadata leaves out.
func compareOptional(w io.Writer, name, eval string, prod *string) {
	switch {
	case prod == nil:
		fmt.Fprintf(w, "- %s: production ABSENT (element not found); evaluation %d chars\n", name, runes(eval))
	case strings.TrimFunc(*prod, unicode.IsSpace) == "":
		fmt.Fprintf(w, "- %s: production BLANK (the extension leaves it out); evaluation %d chars\n", name, runes(eval))
	default:
		compare(w, name, eval, *prod)
	}
}

// compare classifies the difference between two values: equal, newline
// positions only, a production value that is a prefix of the evaluation value
// (a cut-off range), or a line diff whose hunks are counted as changed text or
// a range on one side only.
func compare(w io.Writer, name, eval, prod string) {
	if eval == prod {
		fmt.Fprintf(w, "- %s: EQUAL (%d chars)\n", name, runes(eval))
		return
	}
	if dropNewlines(eval) == dropNewlines(prod) {
		fmt.Fprintf(w, "- %s: NEWLINES ONLY (evaluation %d lines, production %d lines)\n",
			name, lineCount(eval), lineCount(prod))
		return
	}
	if newlinesAsSpaces(eval) == newlinesAsSpaces(prod) {
		fmt.Fprintf(w, "- %s: NEWLINE/SPACE ONLY (equal when each newline is read as a space; evaluation %d lines, production %d lines)\n",
			name, lineCount(eval), lineCount(prod))
		return
	}
	if p := dropNewlines(prod); p != "" && strings.HasPrefix(dropNewlines(eval), p) {
		fmt.Fprintf(w, "- %s: CUT OFF (production is a prefix of evaluation ignoring newlines; %d of %d chars; missing %d chars)\n",
			name, runes(p), runes(dropNewlines(eval)), runes(dropNewlines(eval))-runes(p))
		fmt.Fprintf(w, "  - production ends: %q\n", tail(p, quoteRunes))
		return
	}
	hunks := diffLines(splitLines(eval), splitLines(prod))
	var changed, evalOnly, prodOnly, evalLines, prodLines, evalChars, prodChars int
	for _, h := range hunks {
		switch {
		case len(h.eval) > 0 && len(h.prod) > 0:
			changed++
		case len(h.eval) > 0:
			evalOnly++
		default:
			prodOnly++
		}
		evalLines += len(h.eval)
		prodLines += len(h.prod)
		evalChars += runes(strings.Join(h.eval, ""))
		prodChars += runes(strings.Join(h.prod, ""))
	}
	fmt.Fprintf(w, "- %s: DIFFERENT (evaluation %d lines/%d chars, production %d lines/%d chars)\n",
		name, lineCount(eval), runes(eval), lineCount(prod), runes(prod))
	fmt.Fprintf(w, "  - hunks: %d changed text, %d evaluation only, %d production only; differing lines %d (evaluation) / %d (production), chars %d / %d\n",
		changed, evalOnly, prodOnly, evalLines, prodLines, evalChars, prodChars)
	for i, h := range hunks {
		if i == maxQuotedHunks {
			fmt.Fprintf(w, "  - (%d more hunks not quoted)\n", len(hunks)-maxQuotedHunks)
			break
		}
		fmt.Fprintf(w, "  - at evaluation line %d: evaluation %q / production %q\n",
			h.evalLine+1, head(strings.Join(h.eval, "⏎"), quoteRunes), head(strings.Join(h.prod, "⏎"), quoteRunes))
	}
}

// hunk is one run of lines that differ between the two values.
type hunk struct {
	evalLine int
	eval     []string
	prod     []string
}

// diffLines returns the differing runs of a longest-common-subsequence line
// diff. The inputs are a few thousand lines at most, so the quadratic table
// is small.
func diffLines(a, b []string) []hunk {
	n, m := len(a), len(b)
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var hunks []hunk
	var cur *hunk
	flush := func() {
		if cur != nil {
			hunks = append(hunks, *cur)
			cur = nil
		}
	}
	open := func(i int) {
		if cur == nil {
			cur = &hunk{evalLine: i}
		}
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			flush()
			i++
			j++
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			open(i)
			cur.prod = append(cur.prod, b[j])
			j++
		default:
			open(i)
			cur.eval = append(cur.eval, a[i])
			i++
		}
	}
	flush()
	return hunks
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func lineCount(s string) int {
	return strings.Count(s, "\n") + 1
}

func newlinesAsSpaces(s string) string {
	return strings.ReplaceAll(s, "\n", " ")
}

// reportBoundaries reports how the production segments line up with the
// evaluation segments: how many production segments start at the same second
// as an evaluation segment, and how many production segment texts hold a
// newline (the copied panel text never does; a merged cue is joined by a
// space).
func reportBoundaries(w io.Writer, eval, prod []transcript.Segment) {
	starts := make(map[int64]struct{}, len(eval))
	for _, seg := range eval {
		starts[seg.StartMs/1000] = struct{}{}
	}
	aligned, withNewline := 0, 0
	for _, seg := range prod {
		if _, ok := starts[seg.StartMs/1000]; ok {
			aligned++
		}
		if strings.Contains(seg.Text, "\n") {
			withNewline++
		}
	}
	evalNewline := 0
	for _, seg := range eval {
		if strings.Contains(seg.Text, "\n") {
			evalNewline++
		}
	}
	fmt.Fprintf(w, "- segment starts: %d of %d production segments start at the second of an evaluation segment; segments holding a newline: evaluation %d, production %d\n",
		aligned, len(prod), evalNewline, withNewline)
}

func dropNewlines(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

func runes(s string) int {
	return utf8.RuneCountInString(s)
}

func head(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func tail(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n:])
}
