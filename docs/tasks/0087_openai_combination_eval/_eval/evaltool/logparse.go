package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The stages and origins of the CLI's usage lines (docs/dev/guides/cli.md,
// the sections on token usage and estimated cost).
const (
	stageGeneration = "generation"
	stageShorten    = "shorten"
	stageReview     = "review"
	originRun       = "run"
	originReused    = "reused"
)

// stages lists the three stages in pipeline order.
var stages = []string{stageGeneration, stageShorten, stageReview}

// The line prefixes the CLI writes; every line that starts with one of them
// must match its documented form.
const (
	prefixTokens    = "yt2column: tokens: " //nolint:gosec // a log line prefix, not a credential
	prefixCost      = "yt2column: cost: "
	prefixCostTotal = "yt2column: cost-total: "
)

// Words the log lines and the tables share.
const (
	wordNoModel = "nomodel"
	wordUnknown = "unknown"
	wordArticle = "article"
	wordConfig  = "config"
)

// errLogLine reports a usage, cost, or time line that does not match its
// documented form, or a line that repeats one already read.
var errLogLine = errors.New("log line does not match its documented form")

// tokenCounts are the four counts of a known usage line.
type tokenCounts struct {
	Input, CacheRead, CacheWrite, Output uint64
}

// tokenLine is one "tokens:" line.
type tokenLine struct {
	Stage, Origin string
	Known         bool
	Counts        tokenCounts
	Reason        string // unknown lines only
	Model         string // "" for nomodel
}

// costLine is one "cost:" line.
type costLine struct {
	Stage, Origin string
	Known         bool
	USD           micro
	Basis         string
	Reason        string
	At            time.Time
}

// totalLine is one "cost-total:" line.
type totalLine struct {
	Known bool
	USD   micro
}

// runLog is what one run's standard-error file says: its usage and cost lines,
// its totals, and the wall time zsh's time measured.
type runLog struct {
	Tokens       map[string]tokenLine // by stage
	Costs        map[string]costLine  // by stage
	TotalRun     *totalLine
	TotalArticle *totalLine
	Wall         time.Duration
	HasWall      bool
}

var (
	tokensKnownRE   = regexp.MustCompile(`^(generation|shorten|review) (run|reused) known input=(\d+) cache_read=(\d+) cache_write=(\d+) output=(\d+) (model=(.*)|nomodel)$`)
	tokensUnknownRE = regexp.MustCompile(`^(generation|shorten|review) (run|reused) unknown reason=(\S+) (model=(.*)|nomodel)$`)
	costKnownRE     = regexp.MustCompile(`^(generation|shorten|review) (run|reused) known usd=(\d+\.\d+) (\S+) at=(\S+)$`)
	costUnknownRE   = regexp.MustCompile(`^(generation|shorten|review) (run|reused) unknown reason=(\S+) at=(\S+)$`)
	totalRE         = regexp.MustCompile(`^(run|article) (?:known usd=(\d+\.\d+)|unknown)$`)
	// timeRE matches the line zsh's time builtin writes with its default
	// TIMEFMT: "... <user>s user <system>s system <cpu>% cpu <wall> total".
	timeRE = regexp.MustCompile(` cpu (\d+(?::\d+){0,2}\.\d+|\d+(?::\d+){1,2}) total$`)
)

// parseRunLog reads one run's standard-error file.
func parseRunLog(r io.Reader) (runLog, error) {
	out := runLog{Tokens: map[string]tokenLine{}, Costs: map[string]costLine{}}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20) //nolint:mnd // a log line is short; 1 MiB bounds a broken one
	for scanner.Scan() {
		if err := out.addLine(scanner.Text()); err != nil {
			return runLog{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return runLog{}, err
	}
	return out, nil
}

// addLine reads one line into l. A line with none of the documented prefixes
// and no time suffix is the CLI's other output and is skipped.
func (l *runLog) addLine(line string) error {
	switch {
	case strings.HasPrefix(line, prefixTokens):
		return l.addTokens(strings.TrimPrefix(line, prefixTokens))
	case strings.HasPrefix(line, prefixCost):
		return l.addCost(strings.TrimPrefix(line, prefixCost))
	case strings.HasPrefix(line, prefixCostTotal):
		return l.addTotal(strings.TrimPrefix(line, prefixCostTotal))
	}
	if m := timeRE.FindStringSubmatch(line); m != nil {
		if l.HasWall {
			return fmt.Errorf("%w: a second time line", errLogLine)
		}
		d, err := parseClock(m[1])
		if err != nil {
			return err
		}
		l.Wall, l.HasWall = d, true
	}
	return nil
}

func (l *runLog) addTokens(rest string) error {
	var t tokenLine
	if m := tokensKnownRE.FindStringSubmatch(rest); m != nil {
		counts, err := parseCounts(m[3:7])
		if err != nil {
			return err
		}
		model, err := parseModelField(m[8], m[7] == wordNoModel)
		if err != nil {
			return err
		}
		t = tokenLine{Stage: m[1], Origin: m[2], Known: true, Counts: counts, Model: model}
	} else if m := tokensUnknownRE.FindStringSubmatch(rest); m != nil {
		model, err := parseModelField(m[5], m[4] == wordNoModel)
		if err != nil {
			return err
		}
		t = tokenLine{Stage: m[1], Origin: m[2], Reason: m[3], Model: model}
	} else {
		return fmt.Errorf("%w: tokens line", errLogLine)
	}
	if _, dup := l.Tokens[t.Stage]; dup {
		return fmt.Errorf("%w: second tokens line for %s", errLogLine, t.Stage)
	}
	l.Tokens[t.Stage] = t
	return nil
}

func (l *runLog) addCost(rest string) error {
	var c costLine
	if m := costKnownRE.FindStringSubmatch(rest); m != nil {
		usd, err := parseMicro(m[3])
		if err != nil {
			return err
		}
		at, err := time.Parse(time.RFC3339, m[5])
		if err != nil {
			return fmt.Errorf("%w: cost time: %w", errLogLine, err)
		}
		c = costLine{Stage: m[1], Origin: m[2], Known: true, USD: usd, Basis: m[4], At: at}
	} else if m := costUnknownRE.FindStringSubmatch(rest); m != nil {
		at, err := time.Parse(time.RFC3339, m[4])
		if err != nil {
			return fmt.Errorf("%w: cost time: %w", errLogLine, err)
		}
		c = costLine{Stage: m[1], Origin: m[2], Reason: m[3], At: at}
	} else {
		return fmt.Errorf("%w: cost line", errLogLine)
	}
	if _, dup := l.Costs[c.Stage]; dup {
		return fmt.Errorf("%w: second cost line for %s", errLogLine, c.Stage)
	}
	l.Costs[c.Stage] = c
	return nil
}

func (l *runLog) addTotal(rest string) error {
	m := totalRE.FindStringSubmatch(rest)
	if m == nil {
		return fmt.Errorf("%w: cost-total line", errLogLine)
	}
	total := &totalLine{}
	if m[2] != "" {
		usd, err := parseMicro(m[2])
		if err != nil {
			return err
		}
		total = &totalLine{Known: true, USD: usd}
	}
	slot := &l.TotalRun
	if m[1] == wordArticle {
		slot = &l.TotalArticle
	}
	if *slot != nil {
		return fmt.Errorf("%w: second cost-total %s line", errLogLine, m[1])
	}
	*slot = total
	return nil
}

// parseCounts reads the four counts of a known tokens line.
func parseCounts(fields []string) (tokenCounts, error) {
	var n [4]uint64
	for i, f := range fields {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return tokenCounts{}, fmt.Errorf("%w: token count: %w", errLogLine, err)
		}
		n[i] = v
	}
	return tokenCounts{Input: n[0], CacheRead: n[1], CacheWrite: n[2], Output: n[3]}, nil
}

// parseModelField undoes the escaping the CLI applies to a model name: the
// text after "model=" is a Go string literal's content.
func parseModelField(escaped string, noModel bool) (string, error) {
	if noModel {
		return "", nil
	}
	model, err := strconv.Unquote(`"` + escaped + `"`)
	if err != nil {
		return "", fmt.Errorf("%w: model field: %w", errLogLine, err)
	}
	return model, nil
}

// parseClock reads zsh's elapsed time: "41.366", "1:19.45", or "1:02:03.45".
func parseClock(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	secs, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil || secs < 0 {
		return 0, fmt.Errorf("%w: time %q", errLogLine, s)
	}
	total := time.Duration(secs*float64(time.Second) + 0.5*float64(time.Millisecond)).Truncate(time.Millisecond) //nolint:mnd // round to the millisecond
	unit := time.Minute
	for i := len(parts) - 2; i >= 0; i-- { //nolint:mnd // the fields before the seconds
		v, err := strconv.Atoi(parts[i])
		if err != nil || v < 0 {
			return 0, fmt.Errorf("%w: time %q", errLogLine, s)
		}
		total += time.Duration(v) * unit
		unit = time.Hour
	}
	return total, nil
}

// hasRunTokens reports whether the run called at least one stage itself: the
// CLI writes a tokens line with origin run only after calling the LLM.
func (l runLog) hasRunTokens() bool {
	for _, t := range l.Tokens {
		if t.Origin == originRun {
			return true
		}
	}
	return false
}

// lastRunStage returns the last stage, in pipeline order, the run called
// itself, or "" when it called none. For a failed run it is the stage that
// failed, since a run stops at its first failing stage.
func (l runLog) lastRunStage() string {
	last := ""
	for _, s := range stages {
		if t, ok := l.Tokens[s]; ok && t.Origin == originRun {
			last = s
		}
	}
	return last
}
