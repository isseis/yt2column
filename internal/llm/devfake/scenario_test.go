//go:build test

package devfake

import (
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/isseis/yt2column/internal/llm"
)

func TestNewScenario(t *testing.T) {
	accepted := []struct {
		name    string
		article Article
		failure Failure
		failAt  FailAt
		delay   time.Duration
		want    slog.Value
	}{
		{
			"zero values", ArticleStandard, FailureNone, FailAtNone, 0,
			group(slog.String("article", "standard"), slog.String("fail", "none"), slog.Int64("delay", 0)),
		},
		{
			"long article failing at shorten", ArticleLong, FailureEmpty, FailAtShorten, 0,
			group(slog.String("article", "long"), slog.String("fail", "empty"), slog.String("failStep", "shorten"), slog.Int64("delay", 0)),
		},
		{
			"elements article failing at review", ArticleElements, FailureAPIError, FailAtReview, time.Second,
			group(slog.String("article", "elements"), slog.String("fail", "api-error"), slog.String("failStep", "review"), slog.Int64("delay", 1)),
		},
		{
			"standard article failing at draft", ArticleStandard, FailureTimeout, FailAtDraft, 0,
			group(slog.String("article", "standard"), slog.String("fail", "timeout"), slog.String("failStep", "draft"), slog.Int64("delay", 0)),
		},
		{
			"delay of MaxDelay", ArticleStandard, FailureTruncated, FailAtDraft, MaxDelay,
			group(slog.String("article", "standard"), slog.String("fail", "truncated"), slog.String("failStep", "draft"), slog.Int64("delay", 600)),
		},
	}
	for _, tc := range accepted {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			s, err := NewScenario(tc.article, tc.failure, tc.failAt, tc.delay)
			if err != nil {
				t.Fatalf("NewScenario() error = %v, want nil", err)
			}
			// A Scenario holds a function, so it is compared by what it reports.
			if got := s.LogValue(); !got.Equal(tc.want) {
				t.Errorf("LogValue() = %v, want %v", got, tc.want)
			}
		})
	}

	rejected := []struct {
		name    string
		article Article
		failure Failure
		failAt  FailAt
		delay   time.Duration
	}{
		{"article past the enumeration", ArticleElements + 1, FailureNone, FailAtNone, 0},
		{"negative article", -1, FailureNone, FailAtNone, 0},
		{"failure past the enumeration", ArticleStandard, FailureAPIKeyRejected + 1, FailAtDraft, 0},
		{"failing call past the enumeration", ArticleStandard, FailureEmpty, FailAtReview + 1, 0},
		{"failure without a failing call", ArticleStandard, FailureEmpty, FailAtNone, 0},
		{"failing call without a failure", ArticleStandard, FailureNone, FailAtDraft, 0},
		{"shorten failure of the standard article", ArticleStandard, FailureEmpty, FailAtShorten, 0},
		{"shorten failure of the elements article", ArticleElements, FailureTimeout, FailAtShorten, 0},
		{"negative delay", ArticleStandard, FailureNone, FailAtNone, -time.Nanosecond},
		{"delay over MaxDelay", ArticleStandard, FailureNone, FailAtNone, MaxDelay + time.Nanosecond},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			if _, err := NewScenario(tc.article, tc.failure, tc.failAt, tc.delay); !errors.Is(err, ErrInvalidScenario) {
				t.Errorf("NewScenario() error = %v, want ErrInvalidScenario", err)
			}
		})
	}
}

// TestEnumStrings pins the command-line spellings, which the development
// server accepts and documents, to the design's table.
func TestEnumStrings(t *testing.T) {
	checkSpellings(t, "Articles", Articles(), []string{"standard", "long", "elements"})
	checkSpellings(t, "Failures", Failures(), []string{"empty", "truncated", "api-error", "timeout", "api_key_rejected"})
	checkSpellings(t, "FailAts", FailAts(), []string{"draft", "shorten", "review"})
	checkSpellings(t, "SettingsChecks", SettingsChecks(), []string{"ok", "key_rejected", "unverified"})

	accepted := map[string]struct{}{}
	for _, list := range [][]string{
		spellings(Articles()), spellings(Failures()), spellings(FailAts()), spellings(SettingsChecks()),
	} {
		for _, s := range list {
			accepted[s] = struct{}{}
		}
	}
	outside := []string{
		(ArticleElements + 1).String(), Article(-1).String(),
		(FailureAPIKeyRejected + 1).String(), Failure(-1).String(),
		(FailAtReview + 1).String(), FailAt(-1).String(),
		(SettingsCheckUnverified + 1).String(), SettingsCheck(-1).String(),
		FailureNone.String(), FailAtNone.String(),
	}
	for _, s := range outside {
		if _, ok := accepted[s]; ok {
			t.Errorf("a value outside the accepted lists is spelled %q, an accepted spelling", s)
		}
	}
}

// TestScenarioLogValue checks that LogValue reports the delay in whole
// seconds, rounded down. TestNewScenario's accepted rows check the other
// attributes.
func TestScenarioLogValue(t *testing.T) {
	s := mustScenario(t, ArticleLong, FailureTruncated, FailAtShorten, 1500*time.Millisecond)
	want := group(slog.String("article", "long"), slog.String("fail", "truncated"), slog.String("failStep", "shorten"), slog.Int64("delay", 1))
	if got := s.LogValue(); !got.Equal(want) {
		t.Errorf("LogValue() = %v, want %v", got, want)
	}
}

func group(attrs ...slog.Attr) slog.Value { return slog.GroupValue(attrs...) }

// TestParseScenario checks the five spellings: an accepted set gives the
// scenario NewScenario builds and the selected settings check, and each value
// is rejected when it is empty, unknown, written in another form, or makes a
// combination NewScenario rejects.
func TestParseScenario(t *testing.T) {
	accepted := []struct {
		name      string
		spelling  ScenarioSpelling
		want      slog.Value
		wantCheck error
	}{
		{
			"no failure and an accepted key",
			ScenarioSpelling{Article: "standard", Failure: "none", FailAt: "none", DelaySeconds: "0", SettingsCheck: "ok"},
			group(slog.String("article", "standard"), slog.String("fail", "none"), slog.Int64("delay", 0)),
			nil,
		},
		{
			"every value selected",
			ScenarioSpelling{Article: "long", Failure: "truncated", FailAt: "shorten", DelaySeconds: "600", SettingsCheck: "key_rejected"},
			group(slog.String("article", "long"), slog.String("fail", "truncated"), slog.String("failStep", "shorten"), slog.Int64("delay", 600)),
			llm.ErrAuthRejected,
		},
		{
			"the rejected key at the draft call",
			ScenarioSpelling{Article: "standard", Failure: "api_key_rejected", FailAt: "draft", DelaySeconds: "1", SettingsCheck: "unverified"},
			group(slog.String("article", "standard"), slog.String("fail", "api_key_rejected"), slog.String("failStep", "draft"), slog.Int64("delay", 1)),
			ErrSimulatedAPIFailure,
		},
		{
			"the elements article failing at review",
			ScenarioSpelling{Article: "elements", Failure: "api-error", FailAt: "review", DelaySeconds: "0", SettingsCheck: "ok"},
			group(slog.String("article", "elements"), slog.String("fail", "api-error"), slog.String("failStep", "review"), slog.Int64("delay", 0)),
			nil,
		},
	}
	for _, tc := range accepted {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			s, err := ParseScenario(tc.spelling)
			if err != nil {
				t.Fatalf("ParseScenario(%+v) error = %v, want nil", tc.spelling, err)
			}
			if got := s.LogValue(); !got.Equal(tc.want) {
				t.Errorf("LogValue() = %v, want %v", got, tc.want)
			}
			check := s.CheckKeyResult()
			switch {
			case tc.wantCheck == nil && check != nil:
				t.Errorf("CheckKeyResult() = %v, want nil", check)
			case tc.wantCheck != nil && !errors.Is(check, tc.wantCheck):
				t.Errorf("CheckKeyResult() = %v, want an error wrapping %v", check, tc.wantCheck)
			}
		})
	}

	valid := ScenarioSpelling{Article: "long", Failure: "empty", FailAt: "draft", DelaySeconds: "10", SettingsCheck: "ok"}
	// want, when set, is a sentinel the rejection must also wrap. The
	// development server's test pins the wording an operator reads.
	rejected := []struct {
		name   string
		change func(*ScenarioSpelling)
		want   error
	}{
		{"empty article", func(s *ScenarioSpelling) { s.Article = "" }, nil},
		{"empty failure", func(s *ScenarioSpelling) { s.Failure = "" }, nil},
		{"empty failing call", func(s *ScenarioSpelling) { s.FailAt = "" }, nil},
		{"empty delay", func(s *ScenarioSpelling) { s.DelaySeconds = "" }, errDelayForm},
		{"empty settings check", func(s *ScenarioSpelling) { s.SettingsCheck = "" }, nil},
		{"unknown article", func(s *ScenarioSpelling) { s.Article = "short" }, nil},
		{"unknown failure", func(s *ScenarioSpelling) { s.Failure = "crash" }, nil},
		{"unknown failing call", func(s *ScenarioSpelling) { s.FailAt = "write" }, nil},
		{"unknown settings check", func(s *ScenarioSpelling) { s.SettingsCheck = "maybe" }, nil},
		{"padded article", func(s *ScenarioSpelling) { s.Article = " long" }, nil},
		{"upper-case article", func(s *ScenarioSpelling) { s.Article = "LONG" }, nil},
		{"a failing call without a failure", func(s *ScenarioSpelling) { s.Failure = "none" }, nil},
		{"a failure without a failing call", func(s *ScenarioSpelling) { s.FailAt = "none" }, nil},
		{"shorten call of a standard article", func(s *ScenarioSpelling) {
			s.Article, s.Failure, s.FailAt = "standard", "truncated", "shorten"
		}, nil},
		{"delay over the longest", func(s *ScenarioSpelling) { s.DelaySeconds = "601" }, errDelayRange},
		{"delay that overflows a duration", func(s *ScenarioSpelling) { s.DelaySeconds = "18446744074" }, errDelayRange},
		{"huge delay", func(s *ScenarioSpelling) { s.DelaySeconds = "99999999999999999999999" }, errDelayRange},
		{"delay with a leading zero", func(s *ScenarioSpelling) { s.DelaySeconds = "01" }, errDelayForm},
		{"delay with a unit", func(s *ScenarioSpelling) { s.DelaySeconds = "1s" }, errDelayForm},
		{"padded delay", func(s *ScenarioSpelling) { s.DelaySeconds = " 1" }, errDelayForm},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			spelling := valid
			tc.change(&spelling)
			_, err := ParseScenario(spelling)
			if !errors.Is(err, ErrInvalidScenario) {
				t.Fatalf("ParseScenario(%+v) error = %v, want ErrInvalidScenario", spelling, err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("ParseScenario(%+v) error = %v, want an error wrapping %v", spelling, err, tc.want)
			}
		})
	}
}

// TestCheckKeyResult checks that each settings check result reports the error
// its name promises, and that an unknown result fails secure.
func TestCheckKeyResult(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		if err := (Scenario{settingsCheck: SettingsCheckOK}).CheckKeyResult(); err != nil {
			t.Errorf("CheckKeyResult() = %v, want nil", err)
		}
	})
	for _, tc := range []struct {
		name    string
		check   SettingsCheck
		want    error
		notWant error
	}{
		{"key rejected", SettingsCheckKeyRejected, llm.ErrAuthRejected, ErrSimulatedAPIFailure},
		{"unverified", SettingsCheckUnverified, ErrSimulatedAPIFailure, llm.ErrAuthRejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (Scenario{settingsCheck: tc.check}).CheckKeyResult()
			if !errors.Is(err, tc.want) {
				t.Errorf("CheckKeyResult() = %v, want an error wrapping %v", err, tc.want)
			}
			if errors.Is(err, tc.notWant) {
				t.Errorf("CheckKeyResult() = %v, want an error that is not %v", err, tc.notWant)
			}
		})
	}

	t.Run("unknown result fails secure", func(t *testing.T) {
		s := Scenario{settingsCheck: SettingsCheckUnverified + 1}
		if err := s.CheckKeyResult(); !errors.Is(err, ErrUnexpectedCall) {
			t.Errorf("CheckKeyResult() = %v, want ErrUnexpectedCall", err)
		}
	})
}

func mustScenario(t *testing.T, article Article, failure Failure, failAt FailAt, delay time.Duration) Scenario {
	t.Helper()
	s, err := NewScenario(article, failure, failAt, delay)
	if err != nil {
		t.Fatalf("NewScenario(%v, %v, %v, %v) error = %v", article, failure, failAt, delay, err)
	}
	return s
}

func spellings[T interface{ String() string }](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.String()
	}
	return out
}

func checkSpellings[T interface{ String() string }](t *testing.T, name string, values []T, want []string) {
	t.Helper()
	if got := spellings(values); !slices.Equal(got, want) {
		t.Errorf("%s() spellings = %q, want %q", name, got, want)
	}
}
