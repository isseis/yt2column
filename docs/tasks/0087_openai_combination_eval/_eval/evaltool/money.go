package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/isseis/yt2column/internal/pricing"
)

// micro is an amount of US dollars in units of 10^-6 USD, the precision of the
// CLI's cost lines. Sums of micro values are exact.
type micro int64

// microPlaces is the number of decimal places a micro amount carries.
const microPlaces = 6

// microPerUSD is the number of micro units in one dollar.
const microPerUSD = 1_000_000

// pricingPlaces is the precision pricing.Amount.Format gives exactly.
const pricingPlaces = 12

// errAmount reports an amount that is not a plain non-negative decimal with at
// most six places.
var errAmount = errors.New("amount is not a non-negative decimal with at most six places")

// parseMicro reads a decimal such as "0.305524". It rejects a sign, an
// exponent, and more than six places rather than rounding.
func parseMicro(s string) (micro, error) {
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > microPlaces || !allDigits(whole) || !allDigits(frac) {
		return 0, fmt.Errorf("%w: %q", errAmount, s)
	}
	frac += strings.Repeat("0", microPlaces-len(frac))
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errAmount, s)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errAmount, s)
	}
	return micro(w*microPerUSD + f), nil
}

// allDigits reports whether s holds only ASCII digits (true for "").
func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// String formats the amount with exactly six places, as the CLI does.
func (m micro) String() string {
	return fmt.Sprintf("%d.%06d", m/microPerUSD, m%microPerUSD)
}

// ceilMicro converts an exact pricing amount to micro units, rounding up, so a
// bound built from it never falls below the exact value.
func ceilMicro(a pricing.Amount) (micro, error) {
	exact := a.Format(pricingPlaces)
	whole, frac, _ := strings.Cut(exact, ".")
	head, tail := frac[:microPlaces], frac[microPlaces:]
	m, err := parseMicro(whole + "." + head)
	if err != nil {
		return 0, err
	}
	if strings.Trim(tail, "0") != "" {
		m++
	}
	return m, nil
}

// avgMicro returns sum/n rounded half up, for display only; decisions compare
// sums so they never depend on the rounding.
func avgMicro(sum micro, n int) micro {
	return (sum*2 + micro(n)) / (micro(n) * 2) //nolint:mnd // doubling rounds half up
}

// fmtDuration formats a duration as minutes, seconds, and one decimal place
// ("1:53.4"), rounding half up to a tenth of a second, the form of the 0018
// evaluation record.
func fmtDuration(d time.Duration) string {
	tenths := (d.Milliseconds() + 50) / 100                     //nolint:mnd // 100 ms is one tenth; 50 ms rounds half up
	minutes := tenths / 600                                     //nolint:mnd // 600 tenths per minute
	rest := tenths % 600                                        //nolint:mnd // 600 tenths per minute
	return fmt.Sprintf("%d:%02d.%d", minutes, rest/10, rest%10) //nolint:mnd // ten tenths per second
}

// avgDuration returns sum/n rounded half up to a millisecond, for display.
func avgDuration(sum time.Duration, n int) time.Duration {
	ms := sum.Milliseconds()
	return time.Duration((ms*2+int64(n))/(int64(n)*2)) * time.Millisecond //nolint:mnd // doubling rounds half up
}
