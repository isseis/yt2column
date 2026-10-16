//go:build test

package pricing

import (
	"math/big"
	"testing"
)

// TestAmountFormat pins the rounding and the rendering of Amount.Format. The
// fixtures are US dollar values in units of 10^-12 USD written as decimal
// strings, so the expected strings below are the decimal value rounded to the
// requested number of places (half up).
func TestAmountFormat(t *testing.T) {
	cases := []struct {
		name   string
		pico   string
		places int
		want   string
	}{
		{"zero six places", "0", 6, "0.000000"},
		{"zero four places", "0", 4, "0.0000"},
		{"exactly half a digit rounds up", "50000000", 4, "0.0001"},
		{"below half a digit rounds down", "40000000", 4, "0.0000"},
		{"six places keep every digit", "2345000000", 6, "0.002345"},
		{"four places round the fifth", "2345000000", 4, "0.0023"},
		{"integer part is one", "1234567000000", 6, "1.234567"},
		{"rounding carries into the integer part", "999999999999", 6, "1.000000"},
		{"zero places round half up", "1500000000000", 0, "2"},
		{"places equal the stored precision", "1234567000000", 12, "1.234567000000"},
		{"no exponent for a large amount", "123456789012345678901234567890", 6, "123456789012345678.901235"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pico, ok := new(big.Int).SetString(tc.pico, 10)
			if !ok {
				t.Fatalf("bad fixture %q", tc.pico)
			}
			got := Amount{pico: pico}.Format(tc.places)
			if got != tc.want {
				t.Errorf("Format(%d) = %q, want %q", tc.places, got, tc.want)
			}
		})
	}
}
