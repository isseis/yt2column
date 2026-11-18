package devfake

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MaxDelaySeconds is the longest delay in seconds. ParseScenario compares the
// number of seconds with it before multiplying, so a huge number cannot
// overflow into a duration NewScenario accepts. The test of the development
// build's documented values reads it, so the document and the rule cannot
// drift apart.
const MaxDelaySeconds = uint64(MaxDelay / time.Second)

// The reasons a delay spelling is rejected, one per class: its form, and its
// range.
var (
	errDelayForm  = errors.New("not a whole number of seconds written in decimal digits without a leading zero")
	errDelayRange = errors.New("over the longest delay")
)

// ScenarioSpelling is a scenario as the five text values a build or a command
// line gives it: the article, the failure, the failing call, the delay in whole
// seconds, and the settings check result. Every value must be present; none has
// a default.
type ScenarioSpelling struct {
	Article       string
	Failure       string
	FailAt        string
	DelaySeconds  string
	SettingsCheck string
}

// ParseScenario converts the five spellings into a checked Scenario. It
// rejects, with an error wrapping ErrInvalidScenario, an empty value, a
// spelling no accepted value has, a delay written in another form, and the
// combination NewScenario rejects. No value is trimmed, lower-cased, or
// otherwise repaired.
//
// Contract: the failure and the failing call also accept the "none" spelling,
// which selects no failure; the caller gives it when it has nothing to select,
// as the default of the development build's variables does. Every other value
// must be spelled exactly as its String returns it.
func ParseScenario(s ScenarioSpelling) (Scenario, error) {
	article, err := parseSpelling(Articles(), "article", s.Article)
	if err != nil {
		return Scenario{}, err
	}
	failure, err := parseSpelling(failureChoices(), "failure", s.Failure)
	if err != nil {
		return Scenario{}, err
	}
	failAt, err := parseSpelling(failAtChoices(), "failing call", s.FailAt)
	if err != nil {
		return Scenario{}, err
	}
	delay, err := parseDelaySeconds(s.DelaySeconds)
	if err != nil {
		return Scenario{}, err
	}
	settingsCheck, err := parseSpelling(SettingsChecks(), "settings check", s.SettingsCheck)
	if err != nil {
		return Scenario{}, err
	}
	scenario, err := NewScenario(article, failure, failAt, delay)
	if err != nil {
		return Scenario{}, err
	}
	scenario.settingsCheck = settingsCheck
	return scenario, nil
}

// failureChoices and failAtChoices add the none value a spelling may select,
// so a caller that selects no failure names it instead of leaving a value out.
// NewScenario accepts a none value only together with the other one.
func failureChoices() []Failure { return append(Failures(), FailureNone) }
func failAtChoices() []FailAt   { return append(FailAts(), FailAtNone) }

// parseSpelling returns the accepted value whose spelling is text. kind names
// the value in the rejection, and the rejection lists the accepted spellings.
func parseSpelling[T interface {
	comparable
	fmt.Stringer
}](values []T, kind, text string) (T, error) {
	for _, value := range values {
		if value.String() == text {
			return value, nil
		}
	}
	var zero T
	return zero, fmt.Errorf("%w: %s %q is not an accepted value; accepted: %s", ErrInvalidScenario, kind, text, Spellings(values))
}

// Spellings returns the values' spellings, separated by "|". ParseScenario's
// rejection uses it, so the message lists what ParseScenario accepts.
func Spellings[T fmt.Stringer](values []T) string {
	names := make([]string, len(values))
	for i, value := range values {
		names[i] = value.String()
	}
	return strings.Join(names, "|")
}

// parseDelaySeconds parses the delay spelling: decimal digits, no leading zero
// on a number of two or more digits, and at most MaxDelaySeconds.
func parseDelaySeconds(text string) (time.Duration, error) {
	if text == "" || strings.TrimLeft(text, "0123456789") != "" || (len(text) > 1 && text[0] == '0') {
		return 0, fmt.Errorf("%w: delay %q: %w", ErrInvalidScenario, text, errDelayForm)
	}
	seconds, err := strconv.ParseUint(text, 10, 64)
	if err != nil || seconds > MaxDelaySeconds {
		return 0, fmt.Errorf("%w: delay %q: %w", ErrInvalidScenario, text, errDelayRange)
	}
	return time.Duration(seconds) * time.Second, nil
}

// Spelling returns the scenario as the five spellings ParseScenario accepts, so
// a scenario parsed from them reports the same values back. The failure and the
// failing call are "none" when no call fails, and the delay is in whole seconds.
func (s Scenario) Spelling() ScenarioSpelling {
	return ScenarioSpelling{
		Article:       s.article.String(),
		Failure:       s.failure.String(),
		FailAt:        s.failAt.String(),
		DelaySeconds:  strconv.FormatInt(int64(s.delay/time.Second), 10),
		SettingsCheck: s.settingsCheck.String(),
	}
}
