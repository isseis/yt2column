//go:build test

package llm

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestUsageZeroValue(t *testing.T) {
	var zero Usage
	if _, ok := zero.Counts(); ok {
		t.Error("zero Usage.Counts() ok = true, want false (unknown)")
	}
	if got := zero.UnknownReason(); got != "not-read" {
		t.Errorf("zero Usage.UnknownReason() = %q, want %q", got, "not-read")
	}

	zeroCounts := KnownUsage(TokenCounts{})
	if _, ok := zeroCounts.Counts(); !ok {
		t.Fatal("KnownUsage(TokenCounts{}).Counts() ok = false, want true")
	}
	if zeroCounts == zero {
		t.Error("KnownUsage(TokenCounts{}) == the zero Usage, want them distinct")
	}
	if got := zeroCounts.UnknownReason(); got != "" {
		t.Errorf("KnownUsage(TokenCounts{}).UnknownReason() = %q, want empty", got)
	}
}

// TestUnknownReason checks each rule renders its documented space-free token.
func TestUnknownReason(t *testing.T) {
	cases := map[string]struct {
		usage Usage
		want  string
	}{
		"known":                         {KnownUsage(TokenCounts{}), ""},
		"not read":                      {UnknownUsage(RuleNotRead, ""), "not-read"},
		"missing without a member":      {UnknownUsage(RuleMissing, ""), "usage:missing"},
		"missing with a member":         {UnknownUsage(RuleMissing, "output_tokens"), "usage.output_tokens:missing"},
		"duplicate without a member":    {UnknownUsage(RuleDuplicate, ""), "usage:duplicate"},
		"duplicate with a member":       {UnknownUsage(RuleDuplicate, "completion_tokens"), "usage.completion_tokens:duplicate"},
		"not an object":                 {UnknownUsage(RuleNotObject, ""), "usage:not-an-object"},
		"not a non-negative integer":    {UnknownUsage(RuleNotNonNegativeInteger, "input_tokens"), "usage.input_tokens:not-a-non-negative-integer"},
		"recorded unknown":              {UnknownUsage(RuleRecordedUnknown, ""), "recorded-unknown"},
		"inconsistent without a member": {UnknownUsage(RuleInconsistent, ""), "usage:inconsistent"},
		"inconsistent with a member":    {UnknownUsage(RuleInconsistent, "input_tokens_details"), "usage.input_tokens_details:inconsistent"},
		"rule outside the enumeration":  {UnknownUsage(UnknownRule(99), "output_tokens"), "invalid-rule"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.usage.UnknownReason(); got != tc.want {
				t.Errorf("UnknownReason() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestKnownUsageWithPricingContext checks that the pricing context is kept
// apart from the counts: it is accepted only from the enumeration, it does not
// change the counts, and it makes otherwise equal usages differ.
func TestKnownUsageWithPricingContext(t *testing.T) {
	counts := TokenCounts{Input: 1, CacheRead: 2, CacheWrite: 3, Output: 4}

	for _, pricing := range []PricingContext{PricingContextUnspecified, PricingContextStandard30m, PricingContextUnverified} {
		usage, err := KnownUsageWithPricingContext(counts, pricing)
		if err != nil {
			t.Fatalf("KnownUsageWithPricingContext(%d) error = %v", pricing, err)
		}
		if got, ok := usage.Counts(); !ok || got != counts {
			t.Errorf("context %d: Counts() = %+v, %v, want %+v, true", pricing, got, ok, counts)
		}
		if got := usage.PricingContext(); got != pricing {
			t.Errorf("PricingContext() = %d, want %d", got, pricing)
		}
	}

	for _, pricing := range []PricingContext{PricingContextUnverified + 1, -1} {
		usage, err := KnownUsageWithPricingContext(counts, pricing)
		if !errors.Is(err, ErrInvalidPricingContext) {
			t.Errorf("KnownUsageWithPricingContext(%d) error = %v, want ErrInvalidPricingContext", pricing, err)
		}
		if usage != (Usage{}) {
			t.Errorf("KnownUsageWithPricingContext(%d) = %+v, want the zero Usage", pricing, usage)
		}
	}

	if got := KnownUsage(counts).PricingContext(); got != PricingContextUnspecified {
		t.Errorf("KnownUsage(...).PricingContext() = %d, want PricingContextUnspecified", got)
	}
	unspecified, _ := KnownUsageWithPricingContext(counts, PricingContextUnspecified)
	if unspecified != KnownUsage(counts) {
		t.Error("KnownUsageWithPricingContext(counts, PricingContextUnspecified) != KnownUsage(counts)")
	}
	standard, _ := KnownUsageWithPricingContext(counts, PricingContextStandard30m)
	unverified, _ := KnownUsageWithPricingContext(counts, PricingContextUnverified)
	if standard == KnownUsage(counts) || standard == unverified {
		t.Error("usages with the same counts and different pricing contexts compare equal")
	}

	var zero Usage
	if got := zero.PricingContext(); got != PricingContextUnspecified {
		t.Errorf("zero Usage.PricingContext() = %d, want PricingContextUnspecified", got)
	}
	if got := UnknownUsage(RuleMissing, "").PricingContext(); got != PricingContextUnspecified {
		t.Errorf("UnknownUsage(...).PricingContext() = %d, want PricingContextUnspecified", got)
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
