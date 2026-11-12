package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestParseRunLog reads every documented line form and the time line, and
// rejects a line that breaks its form or repeats a stage.
func TestParseRunLog(t *testing.T) {
	good := strings.Join([]string{
		"yt2column: saved the artifacts to /x",
		`yt2column: tokens: generation reused known input=21451 cache_read=0 cache_write=0 output=10986 model=claude-opus-5-5`,
		`yt2column: tokens: shorten run unknown reason=not-read nomodel`,
		`yt2column: tokens: review run known input=3 cache_read=900 cache_write=15059 output=1092 model=gpt-\"x\"`,
		"yt2column: cost: generation reused known usd=0.305524 flat at=2026-10-23T12:34:33Z",
		"yt2column: cost: shorten run unknown reason=usage-unknown at=2026-10-23T13:24:42Z",
		"yt2column: cost: review run known usd=0.139100 short-prompt at=2026-10-23T13:25:07Z",
		"yt2column: cost-total: run unknown",
		"yt2column: cost-total: article known usd=0.610696",
		"YT2COLUMN_YTDLP_PATH=/usr/bin/false  0.03s user 0.04s system 0% cpu 1:19.45 total",
	}, "\n")
	l, err := parseRunLog(strings.NewReader(good))
	if err != nil {
		t.Fatalf("valid log rejected: %v", err)
	}
	gen := l.Tokens[stageGeneration]
	if gen.Origin != originReused || !gen.Known || gen.Counts.Input != 21451 || gen.Counts.Output != 10986 || gen.Model != "claude-opus-5-5" {
		t.Errorf("generation tokens = %+v", gen)
	}
	if sh := l.Tokens[stageShorten]; sh.Known || sh.Reason != "not-read" || sh.Model != "" {
		t.Errorf("shorten tokens = %+v", sh)
	}
	if rv := l.Tokens[stageReview]; rv.Counts.CacheRead != 900 || rv.Model != `gpt-"x"` {
		t.Errorf("review tokens = %+v", rv)
	}
	if c := l.Costs[stageGeneration]; !c.Known || c.USD != 305524 || c.Origin != originReused {
		t.Errorf("generation cost = %+v", c)
	}
	if c := l.Costs[stageShorten]; c.Known || c.Reason != "usage-unknown" {
		t.Errorf("shorten cost = %+v", c)
	}
	if l.TotalRun == nil || l.TotalRun.Known || l.TotalArticle == nil || l.TotalArticle.USD != 610696 {
		t.Errorf("totals = %+v %+v", l.TotalRun, l.TotalArticle)
	}
	if !l.HasWall || l.Wall != 79450*time.Millisecond {
		t.Errorf("wall = %v %v", l.HasWall, l.Wall)
	}
	if !l.hasRunTokens() || l.lastRunStage() != stageReview {
		t.Errorf("run stages: has %v last %q", l.hasRunTokens(), l.lastRunStage())
	}

	for name, bad := range map[string]string{
		"tokens form":       "yt2column: tokens: generation run known input=1 output=2 model=m",
		"stage name":        "yt2column: tokens: draft run known input=1 cache_read=0 cache_write=0 output=2 model=m",
		"cost amount":       "yt2column: cost: review run known usd=1e-3 flat at=2026-10-23T13:25:07Z",
		"cost time":         "yt2column: cost: review run known usd=0.1 flat at=yesterday",
		"total form":        "yt2column: cost-total: run maybe",
		"duplicate stage":   "yt2column: tokens: review run unknown reason=x nomodel\nyt2column: tokens: review run unknown reason=y nomodel",
		"duplicate total":   "yt2column: cost-total: run unknown\nyt2column: cost-total: run unknown",
		"duplicate cost":    "yt2column: cost: review run unknown reason=x at=2026-10-23T13:25:07Z\nyt2column: cost: review run unknown reason=y at=2026-10-23T13:25:07Z",
		"second time line":  "a 0% cpu 1.0 total\nb 0% cpu 2.0 total",
		"unterminated name": `yt2column: tokens: review run unknown reason=x model=a\`,
	} {
		if _, err := parseRunLog(strings.NewReader(bad)); !errors.Is(err, errLogLine) {
			t.Errorf("%s: err = %v, want errLogLine", name, err)
		}
	}
}

// TestParseClock reads zsh's three elapsed-time forms.
func TestParseClock(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"41.366":     41366 * time.Millisecond,
		"2:15.45":    135450 * time.Millisecond,
		"1:02:03.45": time.Hour + 2*time.Minute + 3450*time.Millisecond,
	} {
		got, err := parseClock(in)
		if err != nil || got != want {
			t.Errorf("parseClock(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"1:x.5", "-1:02.0"} {
		if _, err := parseClock(bad); !errors.Is(err, errLogLine) {
			t.Errorf("parseClock(%q): err = %v", bad, err)
		}
	}
}

// TestLoad0018 reads the 0018 layout, whose run records hold fewer items in
// JST, and derives roNZ-rLxhUE-1's shorten time from B3's log rather than a
// constant.
func TestLoad0018(t *testing.T) {
	root := t.TempDir()
	fake0018(t, root)
	writeFile(t, filepath.Join(root, "failed", "B3", "roNZ-rLxhUE-1-f1", "roNZ-rLxhUE-1.txt"),
		logText([]stageLines{{Stage: stageShorten, Origin: originRun, In: 1, Out: 2, USD: "0.175392", Model: "claude-opus-5-5"}}, "", "", "1.0"))
	writeFile(t, filepath.Join(root, "failed", "B3", "roNZ-rLxhUE-1-f1", "roNZ-rLxhUE-1.when"),
		"start 2026-10-23 Fri 21:43:28 JST\nend 2026-10-23 Fri 21:44:00 JST\nexit=1\n")
	d, err := load0018(root)
	if err != nil {
		t.Fatal(err)
	}
	r := d.Runs["C03"]["roNZ-rLxhUE-1"]
	if r.When.After != stageShorten || !r.When.HasExit || r.When.Exit != 0 {
		t.Errorf("C03 record = %+v", r.When)
	}
	if want := time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC); !r.When.Start.Equal(want) {
		t.Errorf("start = %v, want %v (JST read as UTC+9)", r.When.Start, want)
	}
	if d.Runs["B3"]["zW1RBacSqjk-1"].When.After != "none" {
		t.Errorf("a record without an after line must read as none")
	}
	if len(d.Failed) != 1 || d.Failed[0].Attempt != "f1" || d.Failed[0].When.Exit != 1 {
		t.Errorf("failed = %+v", d.Failed)
	}
	got, err := d.shortenTime0018("roNZ-rLxhUE-1")
	if err != nil || got != 47*time.Second {
		t.Errorf("shorten time = %v, %v; want the 47 s of the log", got, err)
	}
	if _, err := d.shortenTime0018("zW1RBacSqjk-1"); !errors.Is(err, errRunRecord) {
		t.Errorf("an article whose B3 run called no shorten: err = %v", err)
	}
	if _, err := parseWhen0018(strings.NewReader("start 2026-10-23 Fri 21:43:28 UTC\n")); !errors.Is(err, errRunRecord) {
		t.Errorf("a non-JST time: err = %v", err)
	}
}

// TestParseWhen0087Rejects rejects a run record with an unknown, repeated, or
// missing key, or a bad keep_cache.
func TestParseWhen0087Rejects(t *testing.T) {
	good := whenText("C-sol", "roNZ-rLxhUE-1", "none", t0, t0.Add(time.Minute), 0, true)
	if _, err := parseWhen0087(strings.NewReader(good)); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
	for name, bad := range map[string]string{
		"unknown key":    good + "color=red\n",
		"repeated key":   good + "exit=1\n",
		"missing key":    strings.Replace(good, "keep_cache=yes\n", "", 1),
		"bad keep_cache": strings.Replace(good, "keep_cache=yes", "keep_cache=maybe", 1),
	} {
		if _, err := parseWhen0087(strings.NewReader(bad)); !errors.Is(err, errRunRecord) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
