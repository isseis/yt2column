package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// videos are the five videos of the evaluation, in the order of the 0018 and
// 0087 records.
var videos = []string{"roNZ-rLxhUE", "18tbwwPM91A", "zW1RBacSqjk", "leE02uqgaKg", "EqnGQDU71-Y"}

// articles returns the ten article names, "<video ID>-<1|2>", in record order.
func articles() []string {
	var out []string
	for _, v := range videos {
		out = append(out, v+"-1", v+"-2")
	}
	return out
}

// articleRE is the form of an article name.
var articleRE = regexp.MustCompile(`^([A-Za-z0-9_-]{11})-([12])$`)

// videoOf returns the video ID of an article name.
func videoOf(article string) string {
	return article[:len(article)-2]
}

// The attempt label of a run: the current run of an article, or the N-th run
// moved to failed/ before a retry.
const attemptCurrent = "cur"

// errRunRecord reports a run record (.when) or run directory that does not
// match the form the run script writes.
var errRunRecord = errors.New("run record does not match its form")

// when is one run record. End and Exit are absent when the run did not finish.
type when struct {
	Config, Article, After string
	LLM, ReviewLLM         string
	YtDlpPath              string
	KeepCache              bool
	BinarySHA256           string
	Start                  time.Time
	End                    time.Time
	HasEnd                 bool
	Exit                   int
	HasExit                bool
	// RetiredUnfinished is when retire.sh --unfinished moved a run whose
	// record had no exit line; zero otherwise.
	RetiredUnfinished time.Time
}

// run is one execution of the CLI for one article.
type run struct {
	Config, Article, Attempt string
	When                     when
	Log                      runLog
	WorkDir                  string // the --artifacts directory of the run
}

// key returns the identity of a run: config, article, attempt.
func (r run) key() runKey {
	return runKey{Config: r.Config, Article: r.Article, Attempt: r.Attempt}
}

// runKey identifies a run.
type runKey struct{ Config, Article, Attempt string }

// finished reports whether the run record has its end and exit lines.
func (r run) finished() bool { return r.When.HasEnd && r.When.HasExit }

// settled reports whether no CLI of the run can still be running: it
// finished, or a human retired it as unfinished after checking that.
func (r run) settled() bool { return r.finished() || !r.When.RetiredUnfinished.IsZero() }

// lastActivity is the latest time the run is known to have been running: its
// end, or for a run retired unfinished, when it was retired.
func (r run) lastActivity() time.Time {
	if r.When.HasEnd {
		return r.When.End
	}
	return r.When.RetiredUnfinished
}

// succeeded reports whether the run produced its article: it exited 0 and
// wrote both cost totals.
func (r run) succeeded() bool {
	return r.When.HasExit && r.When.Exit == 0 && r.Log.TotalRun != nil && r.Log.TotalArticle != nil
}

// whenKeys0087 are the keys of a 0087 run record, each on its own line as
// "<key>=<value>". end and exit are written when the CLI returns.
var whenKeys0087 = []string{
	wordConfig, wordArticle, "after", "llm", "review_llm", "ytdlp_path",
	"keep_cache", "binary_sha256", "start", "end", "exit", "retired_unfinished",
}

// parseWhen0087 reads a run record the 0087 run script wrote.
func parseWhen0087(r io.Reader) (when, error) {
	values := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok || !slices.Contains(whenKeys0087, key) {
			return when{}, fmt.Errorf("%w: line %q", errRunRecord, scanner.Text())
		}
		if _, dup := values[key]; dup {
			return when{}, fmt.Errorf("%w: second %s line", errRunRecord, key)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return when{}, err
	}
	for _, k := range whenKeys0087[:9] {
		if _, ok := values[k]; !ok {
			return when{}, fmt.Errorf("%w: no %s line", errRunRecord, k)
		}
	}
	w := when{
		Config: values[wordConfig], Article: values[wordArticle], After: values["after"],
		LLM: values["llm"], ReviewLLM: values["review_llm"], YtDlpPath: values["ytdlp_path"],
		BinarySHA256: values["binary_sha256"],
	}
	switch values["keep_cache"] {
	case "yes":
		w.KeepCache = true
	case "no":
	default:
		return when{}, fmt.Errorf("%w: keep_cache", errRunRecord)
	}
	var err error
	if w.Start, err = time.Parse(time.RFC3339, values["start"]); err != nil {
		return when{}, fmt.Errorf("%w: start: %w", errRunRecord, err)
	}
	if v, ok := values["end"]; ok {
		if w.End, err = time.Parse(time.RFC3339, v); err != nil {
			return when{}, fmt.Errorf("%w: end: %w", errRunRecord, err)
		}
		w.HasEnd = true
	}
	if v, ok := values["retired_unfinished"]; ok {
		if w.RetiredUnfinished, err = time.Parse(time.RFC3339, v); err != nil {
			return when{}, fmt.Errorf("%w: retired_unfinished: %w", errRunRecord, err)
		}
	}
	if v, ok := values["exit"]; ok {
		if w.Exit, err = strconv.Atoi(v); err != nil {
			return when{}, fmt.Errorf("%w: exit: %w", errRunRecord, err)
		}
		w.HasExit = true
	}
	return w, nil
}

// jst is the zone the 0018 run records were written in.
var jst = time.FixedZone("JST", 9*60*60) //nolint:mnd // UTC+9

// parseWhen0018 reads a 0018 run record: "after <stage>" (resumed runs only),
// "start <date> <weekday> <time> JST", "end ...", and "exit=<code>".
func parseWhen0018(r io.Reader) (when, error) {
	var w when
	var hasStart bool
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		var err error
		switch {
		case strings.HasPrefix(line, "after "):
			w.After = strings.TrimPrefix(line, "after ")
		case strings.HasPrefix(line, "start "):
			w.Start, err = parseJST(strings.TrimPrefix(line, "start "))
			hasStart = true
		case strings.HasPrefix(line, "end "):
			w.End, err = parseJST(strings.TrimPrefix(line, "end "))
			w.HasEnd = true
		case strings.HasPrefix(line, "exit="):
			w.Exit, err = strconv.Atoi(strings.TrimPrefix(line, "exit="))
			w.HasExit = true
		default:
			return when{}, fmt.Errorf("%w: line %q", errRunRecord, line)
		}
		if err != nil {
			return when{}, fmt.Errorf("%w: %w", errRunRecord, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return when{}, err
	}
	if !hasStart {
		return when{}, fmt.Errorf("%w: no start line", errRunRecord)
	}
	if w.After == "" {
		w.After = "none"
	}
	return w, nil
}

// parseJST reads "2026-10-23 Fri 22:23:48 JST" in the JST zone. Go's parser
// would give an unknown zone abbreviation a zero offset, so the suffix is
// checked and removed and the zone applied explicitly.
func parseJST(s string) (time.Time, error) {
	trimmed, ok := strings.CutSuffix(s, " JST")
	if !ok {
		return time.Time{}, fmt.Errorf("%w: time %q is not in JST", errRunRecord, s)
	}
	return time.ParseInLocation("2006-01-02 Mon 15:04:05", trimmed, jst)
}

// readParsed opens path and parses it with parse.
func readParsed[T any](path string, parse func(io.Reader) (T, error)) (T, error) {
	f, err := os.Open(path) //nolint:gosec // paths come from the evaluation directories the flags name
	if err != nil {
		var zero T
		return zero, err
	}
	defer func() { _ = f.Close() }()
	v, err := parse(f)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

// loadRun reads one run's log and record. workDir is the run's --artifacts
// directory; it need not exist.
func loadRun(config, article, attempt, logPath, whenPath, workDir string, parseWhen func(io.Reader) (when, error)) (run, error) {
	w, err := readParsed(whenPath, parseWhen)
	if err != nil {
		return run{}, err
	}
	l, err := readParsed(logPath, parseRunLog)
	if err != nil {
		return run{}, err
	}
	return run{Config: config, Article: article, Attempt: attempt, When: w, Log: l, WorkDir: workDir}, nil
}

// failedDirRE is the name of a run moved to failed/: "<article>-f<N>".
var failedDirRE = regexp.MustCompile(`^(.+)-f([1-9][0-9]*)$`)

// loadRuns0087 reads every run under the 0087 root: the current runs under
// log/<config>/ and the runs moved under failed/<config>/<article>-f<N>/ by
// the retire script. Every config directory must name a config of configs.
func loadRuns0087(root string, configs configTable) ([]run, error) {
	var runs []run
	logRoot := filepath.Join(root, "log")
	cfgDirs, err := readDirNames(logRoot)
	if err != nil {
		return nil, err
	}
	for _, cfg := range cfgDirs {
		if _, ok := configs.byID(cfg); !ok {
			return nil, fmt.Errorf("%w: %s is not a config", errRunRecord, filepath.Join(logRoot, cfg))
		}
		got, err := loadLogDir(filepath.Join(logRoot, cfg), cfg, attemptCurrent, filepath.Join(root, "work", cfg))
		if err != nil {
			return nil, err
		}
		runs = append(runs, got...)
	}
	failedRoot := filepath.Join(root, "failed")
	cfgDirs, err = readDirNames(failedRoot)
	if err != nil {
		return nil, err
	}
	for _, cfg := range cfgDirs {
		if _, ok := configs.byID(cfg); !ok {
			return nil, fmt.Errorf("%w: %s is not a config", errRunRecord, filepath.Join(failedRoot, cfg))
		}
		attempts, err := readDirNames(filepath.Join(failedRoot, cfg))
		if err != nil {
			return nil, err
		}
		for _, name := range attempts {
			m := failedDirRE.FindStringSubmatch(name)
			if m == nil || !articleRE.MatchString(m[1]) {
				return nil, fmt.Errorf("%w: failed run directory %q", errRunRecord, name)
			}
			dir := filepath.Join(failedRoot, cfg, name)
			r, err := loadRun(cfg, m[1], "f"+m[2], filepath.Join(dir, "log", m[1]+".txt"),
				filepath.Join(dir, "log", m[1]+".when"), filepath.Join(dir, "work"), parseWhen0087)
			if err != nil {
				return nil, err
			}
			if err := checkWhenIdentity(r); err != nil {
				return nil, err
			}
			runs = append(runs, r)
		}
	}
	return runs, nil
}

// loadLogDir reads the current runs of one config: each <article>.when must
// have its <article>.txt and the other way round.
func loadLogDir(dir, cfg, attempt, workRoot string) ([]run, error) {
	names, err := readDirNames(dir)
	if err != nil {
		return nil, err
	}
	seen := map[string]int{}
	for _, n := range names {
		base, ext := strings.TrimSuffix(n, filepath.Ext(n)), filepath.Ext(n)
		if (ext != ".when" && ext != ".txt") || !articleRE.MatchString(base) {
			return nil, fmt.Errorf("%w: unexpected file %s", errRunRecord, filepath.Join(dir, n))
		}
		seen[base]++
	}
	var runs []run
	for _, a := range sortedKeys(seen) {
		if seen[a] != 2 { //nolint:mnd // the pair .when and .txt
			return nil, fmt.Errorf("%w: %s lacks its .when or .txt", errRunRecord, filepath.Join(dir, a))
		}
		r, err := loadRun(cfg, a, attempt, filepath.Join(dir, a+".txt"), filepath.Join(dir, a+".when"),
			filepath.Join(workRoot, a), parseWhen0087)
		if err != nil {
			return nil, err
		}
		if err := checkWhenIdentity(r); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, nil
}

// checkWhenIdentity rejects a run whose record names another config or
// article than the directory it was found in.
func checkWhenIdentity(r run) error {
	if r.When.Config != r.Config || r.When.Article != r.Article {
		return fmt.Errorf("%w: record of %s/%s/%s names %s/%s", errRunRecord,
			r.Config, r.Article, r.Attempt, r.When.Config, r.When.Article)
	}
	return nil
}

// readDirNames returns the sorted entry names of dir, or none when dir does
// not exist. Names starting with "." (such as the .DS_Store Finder leaves)
// are skipped; no file the evaluation writes starts with one.
func readDirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

// sortedKeys returns the keys of m in ascending order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// run0018 holds the 0018 runs aggregate and reproduce0018 read, by config:
// "B3g" (B3's generation-only first phase), "B3" (its second phase), "C03",
// and the failed B3 run.
type data0018 struct {
	Runs map[string]map[string]run // config -> article -> current run
	// Failed holds the runs under failed/<config>/<article>-f<N>/, whose files
	// sit directly in that directory.
	Failed []run
	Root   string
}

// configs0018 are the 0018 evaluation IDs the tools read.
var configs0018 = []string{"B3g", "B3", "C03"}

// load0018 reads the 0018 logs the comparison needs.
func load0018(root string) (data0018, error) {
	d := data0018{Runs: map[string]map[string]run{}, Root: root}
	for _, cfg := range configs0018 {
		d.Runs[cfg] = map[string]run{}
		for _, a := range articles() {
			r, err := loadRun(cfg, a, attemptCurrent,
				filepath.Join(root, "log", cfg, a+".txt"), filepath.Join(root, "log", cfg, a+".when"),
				filepath.Join(root, "work", cfg, a), parseWhen0018)
			if err != nil {
				return data0018{}, err
			}
			d.Runs[cfg][a] = r
		}
	}
	names, err := readDirNames(filepath.Join(root, "failed", "B3"))
	if err != nil {
		return data0018{}, err
	}
	for _, name := range names {
		m := failedDirRE.FindStringSubmatch(name)
		if m == nil {
			return data0018{}, fmt.Errorf("%w: failed run directory %q", errRunRecord, name)
		}
		dir := filepath.Join(root, "failed", "B3", name)
		r, err := loadRun("B3", m[1], "f"+m[2], filepath.Join(dir, m[1]+".txt"),
			filepath.Join(dir, m[1]+".when"), filepath.Join(dir, "work"), parseWhen0018)
		if err != nil {
			return data0018{}, err
		}
		d.Failed = append(d.Failed, r)
	}
	return d, nil
}

// shortenTime0018 returns how long B3's run of article took to reach the end
// of its shorten call: from the run's start to the at= time of its
// "cost: shorten run" line. For roNZ-rLxhUE-1 it is the shorten time of B3's
// successful retry, which 0018 added to C03's duration (54 s).
func (d data0018) shortenTime0018(article string) (time.Duration, error) {
	r := d.Runs["B3"][article]
	c, ok := r.Log.Costs[stageShorten]
	if !ok || c.Origin != originRun {
		return 0, fmt.Errorf("%w: B3 %s has no shorten run cost line", errRunRecord, article)
	}
	return c.At.Sub(r.When.Start), nil
}
