package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/artifact"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/writer"
)

// The tests build every input in t.TempDir; none reads the evaluation
// directories outside it.

// testConfigTable is the config table of run.sh, as the tests use it.
const testConfigTable = "C-sol\t1\topenai/gpt-6.1-sol+medium\topenai/gpt-6.1-sol+medium\tfull\n" +
	"C-astra\t1\topenai/gpt-6-astra+medium\topenai/gpt-6-astra+medium\tfull\n" +
	"A-sol\t1\tclaude/claude-opus-5-5+high\topenai/gpt-6.1-sol+medium\tresume\n" +
	"A-astra\t1\tclaude/claude-opus-5-5+high\topenai/gpt-6-astra+medium\tresume\n" +
	"B-sol\t2\topenai/gpt-6.1-sol+medium\tclaude/claude-opus-5-5+high\tfull\n" +
	"B-astra\t2\topenai/gpt-6-astra+medium\tclaude/claude-opus-5-5+high\tfull\n"

// mustConfigs parses testConfigTable.
func mustConfigs(t *testing.T) configTable {
	t.Helper()
	c, err := parseConfigTable(strings.NewReader(testConfigTable))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// writeFile writes content to path, creating its directory.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// stageLines are one stage's lines of a fake log.
type stageLines struct {
	Stage, Origin   string
	In, CR, CW, Out uint64
	USD             string // "unknown" writes an unknown cost line
	Model           string
	At              time.Time
}

// logText builds a run's standard-error text: the tokens and cost lines of
// each stage, the totals (when total is not ""), and zsh's time line (when
// wall is not "").
func logText(lines []stageLines, totalRun, totalArticle, wall string) string {
	var b strings.Builder
	b.WriteString("yt2column: wrote the article to /tmp/x.md (model: m, model version: (none))\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "yt2column: tokens: %s %s known input=%d cache_read=%d cache_write=%d output=%d model=%s\n",
			l.Stage, l.Origin, l.In, l.CR, l.CW, l.Out, l.Model)
	}
	for _, l := range lines {
		at := l.At
		if at.IsZero() {
			at = time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)
		}
		if l.USD == "unknown" {
			fmt.Fprintf(&b, "yt2column: cost: %s %s unknown reason=unpriced-model at=%s\n", l.Stage, l.Origin, at.Format(time.RFC3339))
			continue
		}
		fmt.Fprintf(&b, "yt2column: cost: %s %s known usd=%s flat at=%s\n", l.Stage, l.Origin, l.USD, at.Format(time.RFC3339))
	}
	if totalRun != "" {
		fmt.Fprintf(&b, "yt2column: cost-total: run %s\n", totalRun)
	}
	if totalArticle != "" {
		fmt.Fprintf(&b, "yt2column: cost-total: article %s\n", totalArticle)
	}
	if wall != "" {
		fmt.Fprintf(&b, "env YT2COLUMN_YTDLP_PATH=/usr/bin/false x  0.02s user 0.01s system 0%% cpu %s total\n", wall)
	}
	return b.String()
}

// whenText builds a 0087 run record.
func whenText(cfg, article, after string, start, end time.Time, exit int, finished bool) string {
	c := mustConfigsNoT()
	conf, ok := c.byID(cfg)
	if !ok {
		panic("whenText: unknown config " + cfg)
	}
	s := fmt.Sprintf("config=%s\narticle=%s\nafter=%s\nllm=%s\nreview_llm=%s\nytdlp_path=/usr/bin/false\nkeep_cache=yes\nbinary_sha256=abc\nstart=%s\n",
		cfg, article, after, conf.LLM.Raw, conf.ReviewLLM.Raw, start.UTC().Format(time.RFC3339))
	if finished {
		s += fmt.Sprintf("end=%s\nexit=%d\n", end.UTC().Format(time.RFC3339), exit)
	}
	return s
}

// mustConfigsNoT parses testConfigTable where no *testing.T is at hand; the
// table is a constant the tests parse first with mustConfigs.
func mustConfigsNoT() configTable {
	c, err := parseConfigTable(strings.NewReader(testConfigTable))
	if err != nil {
		panic(err)
	}
	return c
}

// fakeRun is one run to write into a fake 0087 root.
type fakeRun struct {
	Config, Article, Attempt, After string
	Start, End                      time.Time
	Exit                            int
	Unfinished                      bool
	Log                             string
}

// writeRun writes a fake run under root in the layout run.sh and retire.sh
// use.
func writeRun(t *testing.T, root string, r fakeRun) {
	t.Helper()
	if r.After == "" {
		r.After = "none"
	}
	if r.End.IsZero() {
		r.End = r.Start.Add(time.Minute)
	}
	logDir := filepath.Join(root, "log", r.Config)
	if r.Attempt != "" && r.Attempt != attemptCurrent {
		logDir = filepath.Join(root, "failed", r.Config, r.Article+"-"+r.Attempt, "log")
	}
	writeFile(t, filepath.Join(logDir, r.Article+".txt"), r.Log)
	writeFile(t, filepath.Join(logDir, r.Article+".when"), whenText(r.Config, r.Article, r.After, r.Start, r.End, r.Exit, !r.Unfinished))
}

// t0 is the start of the fake evaluation.
var t0 = time.Date(2026, 11, 12, 1, 0, 0, 0, time.UTC)

// fullRunLog is the log of a successful full run of an OpenAI config whose
// article cost is usd, split evenly over generation and review, with wall
// time wall.
func fullRunLog(model, genUSD, reviewUSD, total, wall string) string {
	return logText([]stageLines{
		{Stage: stageGeneration, Origin: originRun, In: 3, CW: 17300, Out: 2000, USD: genUSD, Model: model},
		{Stage: stageReview, Origin: originRun, In: 3, CW: 15000, Out: 800, USD: reviewUSD, Model: model},
	}, "known usd="+total, "known usd="+total, wall)
}

// testRecord builds a stage record that artifact.Marshal accepts.
func testRecord(stage writer.Stage, title, body, model, input string) writer.StageRecord {
	usage, _ := llm.KnownUsageWithPricingContext(llm.TokenCounts{Input: 10, Output: 20}, llm.PricingContextUnspecified)
	rec := writer.StageRecord{
		Stage:    stage,
		Title:    title,
		Body:     body,
		Model:    model,
		Usage:    usage,
		CallTime: time.Date(2026, 10, 23, 12, 34, 33, 0, time.UTC),
		System:   writer.TemplateIdentity{Embedded: true, Digest: writer.Digest("system")},
		User:     writer.TemplateIdentity{Embedded: true, Digest: writer.Digest("user")},
		Input:    input,
	}
	if stage == writer.StageReview {
		rec.Revisions = []writer.Revision{}
	}
	return rec
}

// writeRecord marshals rec into dir as the article's stage file and returns
// its identifier.
func writeRecord(t *testing.T, dir, article string, rec writer.StageRecord) string {
	t.Helper()
	data, id, err := artifact.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, videoOf(article)+stageFiles[rec.Stage.String()]), string(data))
	return id
}

// fake0018 writes a 0018 tree with B3g, B3, and C03 logs for every article:
// generation takes genWall, B3's second phase b3Wall, and C03 c03Wall; B3's
// roNZ-rLxhUE-1 ran its shorten 47 s after its start (not the real 54 s, so a
// constant cannot pass for the value read from the log). It writes base/B3 and
// work/B3 artifacts and returns the generation and shorten identifiers.
func fake0018(t *testing.T, root string) (genIDs, shortenIDs map[string]string) {
	t.Helper()
	genIDs, shortenIDs = map[string]string{}, map[string]string{}
	start := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	jstLine := func(prefix string, at time.Time) string {
		return prefix + " " + at.In(jst).Format("2006-01-02 Mon 15:04:05") + " JST\n"
	}
	for i, a := range articles() {
		gen := testRecord(writer.StageGeneration, "T"+a, "\nBody of "+a+".\n", "claude-opus-5-5", writer.Digest("material", a))
		genIDs[a] = writeRecord(t, filepath.Join(root, "base", "B3", a), a, gen)
		writeRecord(t, filepath.Join(root, "work", "B3", a), a, gen)
		if a == "roNZ-rLxhUE-1" {
			sh := testRecord(writer.StageShorten, "S"+a, "\nShort body.\n", "claude-opus-5-5", genIDs[a])
			shortenIDs[a] = writeRecord(t, filepath.Join(root, "work", "B3", a), a, sh)
		}
		genLog := logText([]stageLines{{Stage: stageGeneration, Origin: originRun, In: 100, Out: 50, USD: "0.300000", Model: "claude-opus-5-5"}},
			"known usd=0.300000", "known usd=0.300000", fmt.Sprintf("1:%02d.00", i))
		writeFile(t, filepath.Join(root, "log", "B3g", a+".txt"), genLog)
		writeFile(t, filepath.Join(root, "log", "B3g", a+".when"), jstLine("start", start)+jstLine("end", start.Add(time.Minute))+"exit=0\n")
		b3Stages := []stageLines{
			{Stage: stageGeneration, Origin: originReused, In: 100, Out: 50, USD: "0.300000", Model: "claude-opus-5-5"},
			{Stage: stageReview, Origin: originRun, In: 100, Out: 50, USD: "0.100000", Model: "claude-opus-5-5", At: start.Add(2 * time.Hour)},
		}
		if a == "roNZ-rLxhUE-1" {
			b3Stages = append(b3Stages, stageLines{
				Stage: stageShorten, Origin: originRun, In: 100, Out: 50, USD: "0.200000",
				Model: "claude-opus-5-5", At: start.Add(time.Hour + 47*time.Second),
			})
		}
		writeFile(t, filepath.Join(root, "log", "B3", a+".txt"), logText(b3Stages, "known usd=0.100000", "known usd=0.400000", "20.000"))
		writeFile(t, filepath.Join(root, "log", "B3", a+".when"), jstLine("start", start.Add(time.Hour))+jstLine("end", start.Add(time.Hour+time.Minute))+"exit=0\n")
		writeFile(t, filepath.Join(root, "log", "C03", a+".txt"), logText(b3Stages[:2], "known usd=0.100000", "known usd=0.400000", "10.000"))
		after := ""
		if a == "roNZ-rLxhUE-1" {
			after = "after shorten\n"
		}
		writeFile(t, filepath.Join(root, "log", "C03", a+".when"), after+jstLine("start", start.Add(3*time.Hour))+jstLine("end", start.Add(3*time.Hour+time.Minute))+"exit=0\n")
	}
	return genIDs, shortenIDs
}
