//go:build test

package devfake

import (
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"
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
		{"failure past the enumeration", ArticleStandard, FailureTimeout + 1, FailAtDraft, 0},
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
	checkSpellings(t, "Failures", Failures(), []string{"empty", "truncated", "api-error", "timeout"})
	checkSpellings(t, "FailAts", FailAts(), []string{"draft", "shorten", "review"})

	accepted := map[string]struct{}{}
	for _, list := range [][]string{spellings(Articles()), spellings(Failures()), spellings(FailAts())} {
		for _, s := range list {
			accepted[s] = struct{}{}
		}
	}
	outside := []string{
		(ArticleElements + 1).String(), Article(-1).String(),
		(FailureTimeout + 1).String(), Failure(-1).String(),
		(FailAtReview + 1).String(), FailAt(-1).String(),
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
