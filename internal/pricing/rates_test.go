//go:build test

package pricing

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestRateTableValid checks that the real rate table satisfies every rule
// validate can report.
func TestRateTableValid(t *testing.T) {
	if err := validate(rateTable); err != nil {
		t.Fatalf("validate(rateTable) = %v, want nil", err)
	}
}

// TestRateTableModels pins, for each model, how its rate set is chosen and how
// many rate sets it carries. The threshold and the values are checked by
// behavior in TestEstimatePromptTier and TestEstimateCategories, not here.
func TestRateTableModels(t *testing.T) {
	want := []struct {
		model  string
		scheme rateScheme
		sets   int
	}{
		{"deepseek-flash", schemeTimeOfDay, 2},
		{"claude-opus-5-5", schemeFlat, 1},
		{"claude-sonnet-5-5", schemeFlat, 1},
		{"claude-haiku-5-5", schemePromptTier, 2},
	}
	if len(rateTable) != len(want) {
		t.Fatalf("rate table has %d entries, want %d", len(rateTable), len(want))
	}
	for i, w := range want {
		m := rateTable[i]
		if m.model != w.model {
			t.Errorf("entry %d model = %q, want %q", i, m.model, w.model)
		}
		if m.scheme != w.scheme {
			t.Errorf("%s scheme = %v, want %v", w.model, m.scheme, w.scheme)
		}
		sets := 1
		if m.alternate != (rates{}) {
			sets = 2
		}
		if sets != w.sets {
			t.Errorf("%s has %d rate sets, want %d", w.model, sets, w.sets)
		}
	}
}

// TestRateFieldsUnsigned checks that a rate and every field of a rate set are
// unsigned, so a negative rate cannot be written at all.
func TestRateFieldsUnsigned(t *testing.T) {
	rateType := reflect.TypeFor[microUSDPerMTok]()
	if !isUnsignedKind(rateType.Kind()) {
		t.Fatalf("microUSDPerMTok kind = %v, want an unsigned integer", rateType.Kind())
	}
	setType := reflect.TypeFor[rates]()
	for _, name := range []string{"input", "cacheRead", "cacheWrite", "output"} {
		field, ok := setType.FieldByName(name)
		if !ok {
			t.Fatalf("rates has no field %q", name)
		}
		if field.Type != rateType {
			t.Errorf("rates.%s type = %v, want %v", name, field.Type, rateType)
		}
	}
}

// isUnsignedKind reports whether k is an unsigned integer kind.
func isUnsignedKind(k reflect.Kind) bool {
	switch k {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	default:
		return false
	}
}

// TestValidateRejects breaks one rule at a time in an otherwise valid table
// and checks that validate reports exactly that rule's sentinel.
func TestValidateRejects(t *testing.T) {
	validPeak := peakRule{
		weekdays: []time.Weekday{time.Monday},
		ranges:   []minuteRange{{start: 60, end: 240}},
	}
	cases := []struct {
		name   string
		mutate func([]modelRates)
		want   error
	}{
		{"empty model", func(tb []modelRates) { tb[0].model = "" }, errEmptyModel},
		{"duplicate model", func(tb []modelRates) { tb[1].model = tb[0].model }, errDuplicateModel},
		{"unset scheme", func(tb []modelRates) { tb[0].scheme = schemeUnset }, errUnknownScheme},
		{"flat model with a peak rule", func(tb []modelRates) { tb[0].peak = validPeak }, errUnusedField},
		{"flat model with a threshold", func(tb []modelRates) { tb[0].threshold = 1 }, errUnusedField},
		{"flat model with an alternate set", func(tb []modelRates) { tb[0].alternate = rates{input: 1} }, errUnusedField},
		{"time-of-day model without an alternate set", func(tb []modelRates) { tb[1].alternate = rates{} }, errMissingAlternate},
		{"time-of-day model with a threshold", func(tb []modelRates) { tb[1].threshold = 1 }, errUnusedField},
		{"prompt-tier model without an alternate set", func(tb []modelRates) { tb[2].alternate = rates{} }, errMissingAlternate},
		{"prompt-tier model with a peak rule", func(tb []modelRates) { tb[2].peak = validPeak }, errUnusedField},
		{"prompt-tier model without a threshold", func(tb []modelRates) { tb[2].threshold = 0 }, errNonPositiveThreshold},
		{"peak rule without weekdays", func(tb []modelRates) {
			tb[1].peak = peakRule{ranges: []minuteRange{{start: 60, end: 240}}}
		}, errInvalidPeakRule},
		{"peak rule without ranges", func(tb []modelRates) {
			tb[1].peak = peakRule{weekdays: []time.Weekday{time.Monday}}
		}, errInvalidPeakRule},
		{"peak rule with a duplicate weekday", func(tb []modelRates) {
			tb[1].peak = peakRule{weekdays: []time.Weekday{time.Monday, time.Monday}, ranges: []minuteRange{{start: 60, end: 240}}}
		}, errInvalidPeakRule},
		{"peak rule with a weekday outside the week", func(tb []modelRates) {
			tb[1].peak = peakRule{weekdays: []time.Weekday{time.Weekday(7)}, ranges: []minuteRange{{start: 60, end: 240}}}
		}, errInvalidPeakRule},
		{"peak rule with start after end", func(tb []modelRates) {
			tb[1].peak = peakRule{weekdays: []time.Weekday{time.Monday}, ranges: []minuteRange{{start: 240, end: 60}}}
		}, errInvalidPeakRule},
		{"peak rule with a range beyond the day", func(tb []modelRates) {
			tb[1].peak = peakRule{weekdays: []time.Weekday{time.Monday}, ranges: []minuteRange{{start: 0, end: 1441}}}
		}, errInvalidPeakRule},
		{"peak rule with overlapping ranges", func(tb []modelRates) {
			tb[1].peak = peakRule{weekdays: []time.Weekday{time.Monday}, ranges: []minuteRange{{start: 60, end: 240}, {start: 120, end: 360}}}
		}, errInvalidPeakRule},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := validTable()
			tc.mutate(table)
			if err := validate(table); !errors.Is(err, tc.want) {
				t.Errorf("validate = %v, want %v", err, tc.want)
			}
		})
	}
}

// validTable is a table that satisfies every rule, for TestValidateRejects to
// break one rule at a time.
func validTable() []modelRates {
	return []modelRates{
		{
			model:  "flat-model",
			scheme: schemeFlat,
			base:   rates{input: 1},
		},
		{
			model:     "time-model",
			scheme:    schemeTimeOfDay,
			base:      rates{input: 1},
			alternate: rates{input: 2},
			peak: peakRule{
				weekdays: []time.Weekday{time.Monday},
				ranges:   []minuteRange{{start: 60, end: 240}},
			},
		},
		{
			model:     "prompt-model",
			scheme:    schemePromptTier,
			base:      rates{input: 1},
			alternate: rates{input: 2},
			threshold: 100,
		},
	}
}

// sourceDateRE matches the checked-date comment the rate table's entries must
// carry.
var sourceDateRE = regexp.MustCompile(`checked \d{4}-\d{2}-\d{2}`)

// TestRateTableSourceComments checks that every rate table entry and every
// peak rule carries, on the comment line just above it, a source URL and a
// checked date.
func TestRateTableSourceComments(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "rates.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse rates.go: %v", err)
	}
	commentEndingOn := map[int]string{}
	for _, group := range file.Comments {
		commentEndingOn[fset.Position(group.End()).Line] = group.Text()
	}
	check := func(what string, startLine int) {
		t.Helper()
		comment, ok := commentEndingOn[startLine-1]
		if !ok {
			t.Errorf("%s has no comment on the preceding line", what)
			return
		}
		if !strings.Contains(comment, "https://") {
			t.Errorf("%s comment %q has no source URL", what, comment)
		}
		if !sourceDateRE.MatchString(comment) {
			t.Errorf("%s comment %q has no checked date", what, comment)
		}
	}

	var entries []ast.Expr
	var peaks []ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if isModelRatesTable(lit) {
			entries = append(entries, lit.Elts...)
		}
		if id, ok := lit.Type.(*ast.Ident); ok && id.Name == "peakRule" {
			peaks = append(peaks, lit)
		}
		return true
	})
	if len(entries) != len(rateTable) {
		t.Fatalf("found %d rate table entries, want %d", len(entries), len(rateTable))
	}
	for i, entry := range entries {
		check("entry "+rateTable[i].model, fset.Position(entry.Pos()).Line)
	}
	if len(peaks) != 1 {
		t.Fatalf("found %d peak rules, want 1", len(peaks))
	}
	for _, peak := range peaks {
		check("peak rule", fset.Position(peak.Pos()).Line)
	}
}

// isModelRatesTable reports whether lit is a []modelRates composite literal.
func isModelRatesTable(lit *ast.CompositeLit) bool {
	array, ok := lit.Type.(*ast.ArrayType)
	if !ok {
		return false
	}
	id, ok := array.Elt.(*ast.Ident)
	return ok && id.Name == "modelRates"
}
