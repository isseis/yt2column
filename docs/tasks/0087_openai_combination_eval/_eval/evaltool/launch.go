package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// launchLogName is the file under the 0087 root where launch appends one line
// per launch, before it starts the runs.
const launchLogName = "launch.log"

// The lock launch holds from its checks until every run it started exits,
// so two launches never pass the reservation on the same spend. It holds a
// per-launch token, which launch also hands each run with launchTokenFlag;
// the run script starts a paid run only when the two match, so a run started
// without launch never escapes the checks.
const (
	launchLockName  = "launch.lock"
	launchTokenFlag = "--launch-token"
	launchOutDir    = "launch-output"
	tokenBytes      = 16
	// maxRetries is the number of retries an article may have (requirements
	// decision 9).
	maxRetries = 2
)

// clockSkew bounds how far -now may be from the system clock.
const clockSkew = time.Minute

// cacheGap is how long launch waits after the last run of another config that
// used the same OpenAI model, so the next config does not read that config's
// prompt cache, which OpenAI keeps for at least 30 minutes (design section 3.5).
const cacheGap = 30 * time.Minute

// errLaunch reports a check that stopped launch before it started any run.
var errLaunch = errors.New("launch stopped before starting any run")

// stopKind says which check stopped launch.
type stopKind int

// The checks that can stop launch. The zero value is not a check.
const (
	stopInput       stopKind = iota + 1 // a flag or an input file launch reads
	stopLock                            // another launch holds the lock
	stopClock                           // -now is far from the system clock
	stopBounds                          // the bounds table is stale or unpriced
	stopApproval                        // no approval, or not of this config, or before it
	stopRunRecords                      // a run record or log cannot be read
	stopRetry                           // an article has used its retries
	stopRunState                        // a run has not finished, or a target has a current run
	stopCacheGap                        // a config sharing an OpenAI model ended too recently
	stopStage1                          // stage 1's judgments are incomplete or unconfirmed
	stopReservation                     // the spend plus the bound would exceed the limit
)

// launchStop is a stop with its check; it wraps errLaunch.
type launchStop struct {
	Kind stopKind
	err  error
}

func (e *launchStop) Error() string { return e.err.Error() }

func (e *launchStop) Unwrap() error { return e.err }

// stop returns a launchStop of kind with the formatted reason.
func stop(kind stopKind, format string, args ...any) error {
	return &launchStop{Kind: kind, err: fmt.Errorf("%w: "+format, append([]any{errLaunch}, args...)...)} //nolint:err113 // wraps errLaunch; every caller passes a constant format
}

// launchEntry is one line of launch.log.
type launchEntry struct {
	At       time.Time
	Config   string
	Articles []string
}

// parseLaunchLog reads launch.log: "<RFC 3339 time>\t<config>\t<article>,...".
func parseLaunchLog(r io.Reader) ([]launchEntry, error) {
	var out []launchEntry
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 3 { //nolint:mnd // three columns
			return nil, fmt.Errorf("%w: launch log line %q", errRunRecord, scanner.Text())
		}
		at, err := time.Parse(time.RFC3339, fields[0])
		if err != nil {
			return nil, fmt.Errorf("%w: launch log time: %w", errRunRecord, err)
		}
		out = append(out, launchEntry{At: at, Config: fields[1], Articles: strings.Split(fields[2], ",")})
	}
	return out, scanner.Err()
}

// launchRequest is what launch was asked to start.
type launchRequest struct {
	Root, RunScript, CLI string
	Stage1Dir            string // score/1, which holds the stage-1 judgments
	Config               string
	Retry                string // one article to retry, or "" for all ten
	Now                  time.Time
	BoundsInput          boundsInput
	Bounds               *table
	Configs              configTable
}

// targets returns the articles the request starts.
func (q launchRequest) targets() []string {
	if q.Retry != "" {
		return []string{q.Retry}
	}
	return articles()
}

// checkLaunch runs every check of design section 3.5 and plan section 1.2 item 10, and
// returns nil only when the runs may start. Each failure names its check.
func checkLaunch(q launchRequest) error {
	cfg, ok := q.Configs.byID(q.Config)
	if !ok {
		return stop(stopInput, "%s is not a config", q.Config)
	}
	if q.Retry != "" && !articleRE.MatchString(q.Retry) {
		return stop(stopInput, "%q is not an article", q.Retry)
	}
	// The committed bounds table must be the one the current inputs give,
	// which also stops a model the rate table cannot price.
	fresh, _, err := estimateAndBounds(q.BoundsInput, q.Configs)
	if err != nil {
		return stop(stopBounds, "%w", err)
	}
	if !slices.EqualFunc(fresh.Rows, q.Bounds.Rows, slices.Equal[[]string]) {
		return stop(stopBounds, "the bounds table differs from what estimate computes now")
	}
	approvals, err := readParsed(filepath.Join(q.Root, "approval"), parseApprovals)
	if err != nil {
		return stop(stopApproval, "%w", err)
	}
	last := approvals[len(approvals)-1]
	if !slices.Contains(last.Configs, q.Config) {
		return stop(stopApproval, "the approval does not include %s", q.Config)
	}
	if q.Now.Before(last.At) {
		return stop(stopApproval, "now is before the last approval")
	}
	runs, err := loadRuns0087(q.Root, q.Configs)
	if err != nil {
		return stop(stopRunRecords, "%w", err)
	}
	if err := checkRetryLimit(q, runs); err != nil {
		return err
	}
	if err := checkRunsState(q, runs); err != nil {
		return err
	}
	if err := checkCacheGap(q, cfg, runs); err != nil {
		return err
	}
	if cfg.Stage == 2 { //nolint:mnd // stage 2 waits for stage 1's confirmed judgments
		if err := checkStage1Confirmed(q, runs); err != nil {
			return err
		}
	}
	return checkReservation(q, runs, last.Limit)
}

// checkRetryLimit refuses to start an article that already has its
// maxRetries retired attempts, not counting the runs the human confirmed as
// stopped before payment. It applies to every target, so a full launch cannot
// start a third retry either.
func checkRetryLimit(q launchRequest, runs []run) error {
	prepay, err := optional(filepath.Join(q.Root, "prepayment.tsv"), parsePrepayment, map[runKey]bool{})
	if err != nil {
		return stop(stopRetry, "%w", err)
	}
	retired := map[string]int{}
	for _, r := range runs {
		if r.Config == q.Config && r.Attempt != attemptCurrent && !prepay[r.key()] {
			retired[r.Article]++
		}
	}
	for _, a := range q.targets() {
		if retired[a] >= maxRetries {
			return stop(stopRetry, "%s %s already has %d retries", q.Config, a, retired[a])
		}
	}
	return nil
}

// checkRunsState requires every earlier run to have finished, and the target
// articles to have no current run (a retry needs its previous run retired).
func checkRunsState(q launchRequest, runs []run) error {
	for _, r := range runs {
		if !r.finished() {
			return stop(stopRunState, "%s %s %s has not finished", r.Config, r.Article, r.Attempt)
		}
		if r.Config == q.Config && r.Attempt == attemptCurrent && slices.Contains(q.targets(), r.Article) {
			return stop(stopRunState, "%s %s already has a current run", r.Config, r.Article)
		}
	}
	return nil
}

// checkCacheGap requires 30 minutes since the end of the last run of any other
// config that calls one of the same OpenAI models.
func checkCacheGap(q launchRequest, cfg config, runs []run) error {
	models := cfg.openAIModels()
	for _, r := range runs {
		if r.Config == q.Config {
			continue
		}
		other, _ := q.Configs.byID(r.Config)
		shared := slices.ContainsFunc(other.openAIModels(), func(m string) bool { return slices.Contains(models, m) })
		if shared && q.Now.Sub(r.When.End) < cacheGap {
			return stop(stopCacheGap, "%s ended at %s, less than %s before now, and shares an OpenAI model",
				r.Config, fmtTime(r.When.End), cacheGap)
		}
	}
	return nil
}

// stage1ConfirmedName is the file in score/1 the main session writes, with the
// time, once the human has confirmed the stage-1 judgments.
const stage1ConfirmedName = "human-confirmed"

// checkStage1Confirmed requires the stage-1 judgments to be complete for every
// obtained article of the stage-1 configs and confirmed by the human.
func checkStage1Confirmed(q launchRequest, runs []run) error {
	js, err := readParsed(filepath.Join(q.Stage1Dir, "judgments.tsv"), parseJudgments)
	if err != nil {
		return stop(stopStage1, "stage-1 judgments: %w", err)
	}
	stage1 := q.Configs.ids(1)
	if err := js.checkComplete(obtained(runs, stage1)); err != nil {
		return stop(stopStage1, "%w", err)
	}
	for _, id := range stage1 {
		if !slices.ContainsFunc(runs, func(r run) bool { return r.Config == id }) {
			return stop(stopStage1, "stage-1 config %s has not run", id)
		}
	}
	data, err := os.ReadFile(filepath.Join(q.Stage1Dir, stage1ConfirmedName))
	if err != nil {
		return stop(stopStage1, "no human confirmation of the stage-1 judgments: %w", err)
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err != nil {
		return stop(stopStage1, "the human confirmation does not hold a time: %w", err)
	}
	return nil
}

// checkReservation requires the spend so far plus the bound of every run to
// start to stay within the limit of the last approval.
func checkReservation(q launchRequest, runs []run, limit micro) error {
	bounds, err := runBounds(q.Bounds)
	if err != nil {
		return stop(stopReservation, "%w", err)
	}
	spent, err := computeSpend(runs, bounds, nil)
	if err != nil {
		return stop(stopReservation, "%w", err)
	}
	reserve := bounds[q.Config] * micro(len(q.targets()))
	if spent.Total()+reserve > limit {
		return stop(stopReservation, "spent %s plus the bound %s of the runs exceeds the limit %s",
			spent.Total(), reserve, limit)
	}
	return nil
}

// acquireLock creates the launch lock with a new token. It fails when the
// lock exists: another launch is running, or one stopped without removing
// it, which a human removes after checking that no run is in flight.
func acquireLock(root string) (string, func(), error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := hex.EncodeToString(raw)
	path := filepath.Join(root, launchLockName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // path under the operator's root
	if err != nil {
		return "", nil, stop(stopLock, "cannot take %s (another launch, or a stale lock): %w", path, err)
	}
	_, err = f.WriteString(token)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	release := func() { _ = os.Remove(path) }
	if err != nil {
		release()
		return "", nil, err
	}
	return token, release, nil
}

// launchRuns takes the lock, runs every check, and only when they pass
// starts the runs and waits for them; the lock is held until they exit.
func launchRuns(q launchRequest) (map[string]error, error) {
	if err := os.MkdirAll(q.Root, 0o700); err != nil {
		return nil, err
	}
	token, release, err := acquireLock(q.Root)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := checkLaunch(q); err != nil {
		return nil, err
	}
	return startRuns(q, token)
}

// startRuns appends the launch to launch.log, starts the run script for each
// target in parallel with the launch token, and waits for every one. It
// returns each article's exit error, nil for a run that exited 0. Each run
// script's own output goes to launch-output/, so a reason it stopped for is
// kept.
func startRuns(q launchRequest, token string) (map[string]error, error) {
	logPath := filepath.Join(q.Root, launchLogName)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // path under the operator's root
	if err != nil {
		return nil, err
	}
	_, err = fmt.Fprintf(f, "%s\t%s\t%s\n", fmtTime(q.Now), q.Config, strings.Join(q.targets(), ","))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	outDir := filepath.Join(q.Root, launchOutDir)
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return nil, err
	}
	stamp := q.Now.UTC().Format("20060102T150405Z")
	var started []string
	cmds := map[string]*exec.Cmd{}
	files := map[string]*os.File{}
	waitAll := func() map[string]error {
		results := map[string]error{}
		for _, a := range started {
			results[a] = cmds[a].Wait()
			_ = files[a].Close()
		}
		return results
	}
	for _, a := range q.targets() {
		out, err := os.Create(filepath.Join(outDir, stamp+"-"+q.Config+"-"+a+".txt")) //nolint:gosec // path under the operator's root
		if err != nil {
			waitAll()
			return nil, err
		}
		cmd := exec.CommandContext(context.Background(), q.RunScript, "run", q.Config, a, q.CLI, launchTokenFlag, token) //nolint:gosec // operator's run script
		cmd.Stdout, cmd.Stderr = out, out
		if err := cmd.Start(); err != nil {
			_ = out.Close()
			waitAll()
			return nil, fmt.Errorf("start %s: %w", a, err)
		}
		started = append(started, a)
		cmds[a], files[a] = cmd, out
	}
	return waitAll(), nil
}

// runLaunch implements "launch".
func runLaunch(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("launch", flag.ContinueOnError)
	var q launchRequest
	fs.StringVar(&q.Root, "root", "", "the 0087 directory")
	fs.StringVar(&q.RunScript, "runsh", "", "the run script")
	fs.StringVar(&q.CLI, "cli", "", "the pinned CLI binary")
	fs.StringVar(&q.Stage1Dir, "stage1", "", "score/1, the stage-1 judgments directory")
	fs.StringVar(&q.Config, wordConfig, "", "the config to run")
	fs.StringVar(&q.Retry, "retry", "", "one article to retry (optional)")
	boundsInputPath := fs.String("bounds-input", "", "bounds-input.json")
	boundsPath := fs.String("bounds", "", "the bounds table")
	now := fs.String("now", "", "the current time, RFC 3339")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var err error
	if q.Now, err = time.Parse(time.RFC3339, *now); err != nil {
		return stop(stopInput, "-now: %w", err)
	}
	if d := time.Since(q.Now); d > clockSkew || d < -clockSkew {
		return stop(stopClock, "-now is more than %s from the system clock", clockSkew)
	}
	if q.BoundsInput, err = loadBoundsInput(*boundsInputPath); err != nil {
		return stop(stopInput, "%w", err)
	}
	if q.Bounds, err = loadBoundsTable(*boundsPath); err != nil {
		return stop(stopInput, "%w", err)
	}
	if q.Configs, err = loadConfigTable(q.RunScript); err != nil {
		return stop(stopInput, "%w", err)
	}
	results, err := launchRuns(q)
	if err != nil {
		return err
	}
	failed := 0
	for _, a := range q.targets() {
		status := "exit 0"
		if results[a] != nil {
			status = results[a].Error()
			failed++
		}
		if _, err := fmt.Fprintf(stdout, "%s %s: %s\n", q.Config, a, status); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(stdout, "%s: %d of %d runs exited non-zero\n", q.Config, failed, len(q.targets()))
	return err
}
