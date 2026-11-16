package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"

	"github.com/isseis/yt2column/internal/llm/devfake"
)

// Argument names.
const (
	argArticle  = "article"
	argFail     = "fail"
	argFailStep = "fail-step"
	argDelay    = "delay"
)

// defaultDelaySeconds is the delay of a start that gives none.
const defaultDelaySeconds = "0"

// errRepeated is the rejection of an argument given twice. The spellings and
// the combination rules are devfake.ParseScenario's, so this package keeps only
// the rejections of the command line itself.
var errRepeated = errors.New("given more than once")

// errNoneSelected is the rejection of an explicit "none". A start selects no
// failure by leaving both arguments out; the parser's none spelling exists for
// the development build's defaults, not for this command line.
var errNoneSelected = errors.New(`"none" is not an accepted value; leave both arguments out to select no failure`)

// errExtraWord is the rejection of a word left after the arguments.
var errExtraWord = errors.New("unexpected extra argument")

// stringFlag is a flag.Value that keeps the value as it was written and
// rejects a second call, so an argument given twice is an error instead of the
// last value winning. ParseScenario checks the value.
type stringFlag struct {
	value string
	set   bool
}

// Set implements flag.Value.
func (f *stringFlag) Set(s string) error {
	if f.set {
		return errRepeated
	}
	f.value = s
	f.set = true
	return nil
}

// String implements flag.Value.
func (f *stringFlag) String() string {
	if f == nil || !f.set {
		return ""
	}
	return f.value
}

// or returns the value as written, or fallback when the argument was left out.
func (f *stringFlag) or(fallback string) string {
	if !f.set {
		return fallback
	}
	return f.value
}

// usageText is what -h prints and what follows a rejected argument.
func usageText() string {
	return fmt.Sprintf(`usage: %[1]s [-%[2]s %[6]s] [-%[3]s %[7]s -%[4]s %[8]s] [-%[5]s SECONDS]
  -%[2]s      the fixed article to return (default %[9]s)
  -%[3]s         the failure to return; give it together with -%[4]s
  -%[4]s    the call that fails; %[10]s needs -%[2]s %[11]s
  -%[5]s        seconds each call waits before it answers, a whole number from 0 to %[12]d (default 0)
`, programName, argArticle, argFail, argFailStep, argDelay,
		devfake.Spellings(devfake.Articles()), devfake.Spellings(devfake.Failures()), devfake.Spellings(devfake.FailAts()),
		devfake.ArticleStandard, devfake.FailAtShorten, devfake.ArticleLong, devfake.MaxDelaySeconds)
}

// parseArgs reads the command line into a checked Scenario. Contract: it
// touches no file and no network. It rejects an unknown argument, a repeated
// argument, a missing value, and any word left after the arguments; it leaves
// the spellings and the combination and range rules to
// devfake.ParseScenario, which the development build also uses. It returns
// flag.ErrHelp for -h and -help.
//
// The development server has no argument for the settings check: it passes
// the accepted result, because it never calls the API.
func parseArgs(args []string) (devfake.Scenario, error) {
	article := &stringFlag{}
	failure := &stringFlag{}
	failAt := &stringFlag{}
	delay := &stringFlag{}

	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var(article, argArticle, "")
	fs.Var(failure, argFail, "")
	fs.Var(failAt, argFailStep, "")
	fs.Var(delay, argDelay, "")
	if err := fs.Parse(args); err != nil {
		return devfake.Scenario{}, err
	}
	// flag stops at the first word that is not an argument, and consumes a
	// bare "--"; both are extra words here.
	if fs.NArg() > 0 {
		return devfake.Scenario{}, fmt.Errorf("%w %q", errExtraWord, fs.Arg(0))
	}
	if slices.Contains(args, "--") {
		return devfake.Scenario{}, fmt.Errorf("%w %q", errExtraWord, "--")
	}
	if failure.set && failure.value == devfake.FailureNone.String() {
		return devfake.Scenario{}, fmt.Errorf("%w; accepted: %s", errNoneSelected, devfake.Spellings(devfake.Failures()))
	}
	if failAt.set && failAt.value == devfake.FailAtNone.String() {
		return devfake.Scenario{}, fmt.Errorf("%w; accepted: %s", errNoneSelected, devfake.Spellings(devfake.FailAts()))
	}
	return devfake.ParseScenario(devfake.ScenarioSpelling{
		Article:       article.or(devfake.ArticleStandard.String()),
		Failure:       failure.or(devfake.FailureNone.String()),
		FailAt:        failAt.or(devfake.FailAtNone.String()),
		DelaySeconds:  delay.or(defaultDelaySeconds),
		SettingsCheck: devfake.SettingsCheckOK.String(),
	})
}
