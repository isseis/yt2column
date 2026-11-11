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
		return fmt.Errorf("%w: %s is not a config", errLaunch, q.Config)
	}
	if q.Retry != "" && !articleRE.MatchString(q.Retry) {
		return fmt.Errorf("%w: %q is not an article", errLaunch, q.Retry)
	}
	// The committed bounds table must be the one the current inputs give,
	// which also stops a model the rate table cannot price.
	fresh, _, err := estimateAndBounds(q.BoundsInput, q.Configs)
	if err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
	}
	if !slices.EqualFunc(fresh.Rows, q.Bounds.Rows, slices.Equal[[]string]) {
		return fmt.Errorf("%w: the bounds table differs from what estimate computes now", errLaunch)
	}
	approvals, err := readParsed(filepath.Join(q.Root, "approval"), parseApprovals)
	if err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
	}
	last := approvals[len(approvals)-1]
	if !slices.Contains(last.Configs, q.Config) {
		return fmt.Errorf("%w: the approval does not include %s", errLaunch, q.Config)
	}
	if q.Now.Before(last.At) {
		return fmt.Errorf("%w: now is before the last approval", errLaunch)
	}
	runs, err := loadRuns0087(q.Root, q.Configs)
	if err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
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

// checkRetryLimit refuses a retry of an article that already has its
// maxRetries retired attempts, not counting the runs the human confirmed as
// stopped before payment.
func checkRetryLimit(q launchRequest, runs []run) error {
	if q.Retry == "" {
		return nil
	}
	prepay, err := optional(filepath.Join(q.Root, "prepayment.tsv"), parsePrepayment, map[runKey]bool{})
	if err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
	}
	n := 0
	for _, r := range runs {
		if r.Config == q.Config && r.Article == q.Retry && r.Attempt != attemptCurrent && !prepay[r.key()] {
			n++
		}
	}
	if n >= maxRetries {
		return fmt.Errorf("%w: %s %s already has %d retries", errLaunch, q.Config, q.Retry, n)
	}
	return nil
}

// checkRunsState requires every earlier run to have finished, and the target
// articles to have no current run (a retry needs its previous run retired).
func checkRunsState(q launchRequest, runs []run) error {
	for _, r := range runs {
		if !r.finished() {
			return fmt.Errorf("%w: %s %s %s has not finished", errLaunch, r.Config, r.Article, r.Attempt)
		}
		if r.Config == q.Config && r.Attempt == attemptCurrent && slices.Contains(q.targets(), r.Article) {
			return fmt.Errorf("%w: %s %s already has a current run", errLaunch, r.Config, r.Article)
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
			return fmt.Errorf("%w: %s ended at %s, less than %s before now, and shares an OpenAI model",
				errLaunch, r.Config, fmtTime(r.When.End), cacheGap)
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
		return fmt.Errorf("%w: stage-1 judgments: %w", errLaunch, err)
	}
	stage1 := q.Configs.ids(1)
	if err := js.checkComplete(obtained(runs, stage1)); err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
	}
	for _, id := range stage1 {
		if !slices.ContainsFunc(runs, func(r run) bool { return r.Config == id }) {
			return fmt.Errorf("%w: stage-1 config %s has not run", errLaunch, id)
		}
	}
	data, err := os.ReadFile(filepath.Join(q.Stage1Dir, stage1ConfirmedName))
	if err != nil {
		return fmt.Errorf("%w: no human confirmation of the stage-1 judgments: %w", errLaunch, err)
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err != nil {
		return fmt.Errorf("%w: the human confirmation does not hold a time: %w", errLaunch, err)
	}
	return nil
}

// checkReservation requires the spend so far plus the bound of every run to
// start to stay within the limit of the last approval.
func checkReservation(q launchRequest, runs []run, limit micro) error {
	bounds, err := runBounds(q.Bounds)
	if err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
	}
	spent, err := computeSpend(runs, bounds, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
	}
	reserve := bounds[q.Config] * micro(len(q.targets()))
	if spent.Total()+reserve > limit {
		return fmt.Errorf("%w: spent %s plus the bound %s of the runs exceeds the limit %s",
			errLaunch, spent.Total(), reserve, limit)
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
		return "", nil, fmt.Errorf("%w: cannot take %s (another launch, or a stale lock): %w", errLaunch, path, err)
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
		return fmt.Errorf("%w: -now: %w", errLaunch, err)
	}
	if d := time.Since(q.Now); d > clockSkew || d < -clockSkew {
		return fmt.Errorf("%w: -now is more than %s from the system clock", errLaunch, clockSkew)
	}
	if q.BoundsInput, err = loadBoundsInput(*boundsInputPath); err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
	}
	if q.Bounds, err = loadBoundsTable(*boundsPath); err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
	}
	if q.Configs, err = loadConfigTable(q.RunScript); err != nil {
		return fmt.Errorf("%w: %w", errLaunch, err)
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
