package pricing

import (
	"math/big"
	"strings"
)

// Amount is an exact non-negative amount of US dollars, in units of 10^-12
// USD, so that every rate times every count is a whole number. The zero value
// is 0 USD. An Amount is immutable: no method mutates or exposes its integer.
// Compare amounts by their Format output, never with ==.
type Amount struct {
	pico *big.Int // nil means 0
}

// picoDigits is the number of decimal places one Amount unit represents
// (10^-12 USD). decimalBase is the base of the decimal notation.
const (
	picoDigits  = 12
	decimalBase = 10
)

// Format returns the amount in US dollars with exactly places digits after
// the decimal point, rounded half up ("0.002345" for places 6). It never uses
// an exponent and never omits the integer part. places is non-negative.
func (a Amount) Format(places int) string {
	units := a.pico
	if units == nil {
		units = new(big.Int)
	}
	var scaled *big.Int
	if places >= picoDigits {
		scaled = new(big.Int).Mul(units, pow10(places-picoDigits))
	} else {
		divisor := pow10(picoDigits - places)
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(units, divisor, remainder)
		if remainder.Lsh(remainder, 1).Cmp(divisor) >= 0 {
			quotient.Add(quotient, big.NewInt(1))
		}
		scaled = quotient
	}
	return decimalString(scaled, places)
}

// add returns a + b. The result shares no integer with either operand.
func (a Amount) add(b Amount) Amount {
	if b.pico == nil {
		return a
	}
	sum := new(big.Int)
	if a.pico != nil {
		sum.Set(a.pico)
	}
	return Amount{pico: sum.Add(sum, b.pico)}
}

// pow10 returns 10^n for a non-negative n.
func pow10(n int) *big.Int {
	result := big.NewInt(1)
	for range n {
		result.Mul(result, big.NewInt(decimalBase))
	}
	return result
}

// decimalString writes the non-negative scaled integer with exactly places
// digits after the decimal point.
func decimalString(scaled *big.Int, places int) string {
	digits := scaled.String()
	if places == 0 {
		return digits
	}
	if len(digits) <= places {
		return "0." + strings.Repeat("0", places-len(digits)) + digits
	}
	split := len(digits) - places
	return digits[:split] + "." + digits[split:]
}
