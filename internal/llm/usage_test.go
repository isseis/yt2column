//go:build test

package llm

import (
	"os"
	"strings"
	"testing"
)

func TestUsageZeroValue(t *testing.T) {
	var zero Usage
	unknownCounts, ok := zero.Counts()
	if ok {
		t.Error("zero Usage.Counts() ok = true, want false (unknown)")
	}
	if unknownCounts != (TokenCounts{}) {
		t.Errorf("zero Usage.Counts() = %+v, want the zero counts", unknownCounts)
	}
	if got := zero.UnknownReason(); got != "not-read" {
		t.Errorf("zero Usage.UnknownReason() = %q, want %q", got, "not-read")
	}

	zeroCounts := KnownUsage(TokenCounts{})
	counts, ok := zeroCounts.Counts()
	if !ok {
		t.Fatal("KnownUsage(TokenCounts{}).Counts() ok = false, want true")
	}
	if counts != (TokenCounts{}) {
		t.Errorf("KnownUsage(TokenCounts{}).Counts() = %+v, want the zero counts", counts)
	}
	if zeroCounts == zero {
		t.Error("KnownUsage(TokenCounts{}) == the zero Usage, want them distinct")
	}
	if got := zeroCounts.UnknownReason(); got != "" {
		t.Errorf("KnownUsage(TokenCounts{}).UnknownReason() = %q, want empty", got)
	}

	known := KnownUsage(TokenCounts{Input: 1, CacheRead: 2, CacheWrite: 3, Output: 4})
	got, ok := known.Counts()
	want := TokenCounts{Input: 1, CacheRead: 2, CacheWrite: 3, Output: 4}
	if !ok || got != want {
		t.Errorf("KnownUsage(...).Counts() = %+v, %v; want %+v, true", got, ok, want)
	}
}

func TestUnknownReason(t *testing.T) {
	cases := map[string]struct {
		usage Usage
		want  string
	}{
		"known":                        {KnownUsage(TokenCounts{}), ""},
		"not read":                     {UnknownUsage(RuleNotRead, ""), "not-read"},
		"missing without a member":     {UnknownUsage(RuleMissing, ""), "usage:missing"},
		"missing with a member":        {UnknownUsage(RuleMissing, "output_tokens"), "usage.output_tokens:missing"},
		"duplicate without a member":   {UnknownUsage(RuleDuplicate, ""), "usage:duplicate"},
		"duplicate with a member":      {UnknownUsage(RuleDuplicate, "completion_tokens"), "usage.completion_tokens:duplicate"},
		"not an object":                {UnknownUsage(RuleNotObject, ""), "usage:not-an-object"},
		"not a non-negative integer":   {UnknownUsage(RuleNotNonNegativeInteger, "input_tokens"), "usage.input_tokens:not-a-non-negative-integer"},
		"recorded unknown":             {UnknownUsage(RuleRecordedUnknown, ""), "recorded-unknown"},
		"rule outside the enumeration": {UnknownUsage(UnknownRule(99), "output_tokens"), "invalid-rule"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.usage.UnknownReason()
			if got != tc.want {
				t.Errorf("UnknownReason() = %q, want %q", got, tc.want)
			}
			if strings.ContainsAny(got, " \t\n") {
				t.Errorf("UnknownReason() = %q, must not contain whitespace", got)
			}
		})
	}
}

// TestNoProviderMemberNames checks that the provider-free package internal/llm
// names no provider's usage members. The adapter packages name them, in their
// own directories.
func TestNoProviderMemberNames(t *testing.T) {
	members := []string{
		"prompt_cache_hit_tokens",
		"prompt_cache_miss_tokens",
		"completion_tokens",
		"input_tokens",
		"cache_read_input_tokens",
		"cache_creation_input_tokens",
		"output_tokens",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		checked++
		for _, member := range members {
			if strings.Contains(string(data), member) {
				t.Errorf("%s contains the provider member name %q", name, member)
			}
		}
	}
	if checked == 0 {
		t.Fatal("checked no production source file directly under internal/llm")
	}
}
