package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/llm/devfake"
)

// Argument names.
const (
	argArticle  = "article"
	argFail     = "fail"
	argFailStep = "fail-step"
	argDelay    = "delay"
)

var (
	errRepeated     = errors.New("given more than once")
	errUnknownValue = errors.New("not an accepted value")
	errDelayForm    = errors.New("not a whole number of seconds written in decimal digits without a leading zero")
	errDelayRange   = errors.New("over the longest delay")
	errExtraWord    = errors.New("unexpected extra argument")
)

// enumFlag is a flag.Value that accepts the spellings of one devfake
// enumeration, and only once. Its zero value is "not given", and value then
// holds the enumeration's zero value, which means no choice.
type enumFlag[T interface {
	comparable
	fmt.Stringer
}] struct {
	accepted []T
	value    T
	set      bool
}

// Set implements flag.Value. It rejects a second call and any spelling that
// is not the String of an accepted value; it never repairs a spelling.
func (f *enumFlag[T]) Set(s string) error {
	if f.set {
		return errRepeated
	}
	i := slices.IndexFunc(f.accepted, func(v T) bool { return v.String() == s })
	if i < 0 {
		return fmt.Errorf("%w %q; accepted: %s", errUnknownValue, s, spellings(f.accepted))
	}
	f.value = f.accepted[i]
	f.set = true
	return nil
}

// String implements flag.Value.
func (f *enumFlag[T]) String() string {
	if f == nil || !f.set {
		return ""
	}
	return f.value.String()
}

// spellings returns the accepted values' spellings, separated by "|".
func spellings[T fmt.Stringer](values []T) string {
	names := make([]string, len(values))
	for i, v := range values {
		names[i] = v.String()
	}
	return strings.Join(names, "|")
}

// delayFlag is a flag.Value for the delay in whole seconds.
type delayFlag struct {
	value time.Duration
	set   bool
}

// maxDelaySeconds is the longest delay in seconds. Set compares the number
// with it before it multiplies, so a huge number cannot overflow into a value
// devfake.NewScenario's range check accepts.
const maxDelaySeconds = uint64(devfake.MaxDelay / time.Second)

// Set implements flag.Value. It accepts decimal digits only, with no leading
// zero on a number of two or more digits, and at most maxDelaySeconds.
func (f *delayFlag) Set(s string) error {
	if f.set {
		return errRepeated
	}
	if s == "" || strings.TrimLeft(s, "0123456789") != "" || (len(s) > 1 && s[0] == '0') {
		return errDelayForm
	}
	seconds, err := strconv.ParseUint(s, 10, 64)
	if err != nil || seconds > maxDelaySeconds {
		return errDelayRange
	}
	f.value = time.Duration(seconds) * time.Second
	f.set = true
	return nil
}

// String implements flag.Value.
func (f *delayFlag) String() string {
	if f == nil || !f.set {
		return ""
	}
	return strconv.FormatInt(int64(f.value/time.Second), 10)
}

// usageText is what -h prints and what follows a rejected argument.
func usageText() string {
	return fmt.Sprintf(`usage: %[1]s [-%[2]s %[6]s] [-%[3]s %[7]s -%[4]s %[8]s] [-%[5]s SECONDS]
  -%[2]s      the fixed article to return (default %[9]s)
  -%[3]s         the failure to return; give it together with -%[4]s
  -%[4]s    the call that fails; %[10]s needs -%[2]s %[11]s
  -%[5]s        seconds each call waits before it answers, a whole number from 0 to %[12]d (default 0)
`, programName, argArticle, argFail, argFailStep, argDelay,
		spellings(devfake.Articles()), spellings(devfake.Failures()), spellings(devfake.FailAts()),
		devfake.ArticleStandard, devfake.FailAtShorten, devfake.ArticleLong, maxDelaySeconds)
}

// parseArgs reads the command line into a checked Scenario. Contract: it
// touches no file and no network. It rejects an unknown argument, a value that
// is not an accepted spelling, a repeated argument, a missing value, and any
// word left after the arguments, and it leaves the combination and range
// rules to devfake.NewScenario. It returns flag.ErrHelp for -h and -help.
func parseArgs(args []string) (devfake.Scenario, error) {
	article := &enumFlag[devfake.Article]{accepted: devfake.Articles()}
	failure := &enumFlag[devfake.Failure]{accepted: devfake.Failures()}
	failAt := &enumFlag[devfake.FailAt]{accepted: devfake.FailAts()}
	delay := &delayFlag{}

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
	return devfake.NewScenario(article.value, failure.value, failAt.value, delay.value)
}
