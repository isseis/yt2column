// Package tasknaming holds the name forms of task branches, task
// directories, and ADR files, and the four-digit number they carry. The forms
// are the ones CLAUDE.md ("Requirements and Acceptance Criteria"), _context.md
// ("Feature branch name"), and adr.md ("Location and naming") state, and the
// readers such a name should be shared with use this package rather than their
// own expression.
package tasknaming

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

// ErrInvalid reports a value that is not exactly four ASCII digits.
var ErrInvalid = errors.New("invalid task naming")

// numberDigits is the number of digits a Number has.
const numberDigits = 4

// Number is a validated four-digit number, 0000 to 9999. Its zero value is
// invalid and stands for no number: String returns "" for it, so it is never
// mistaken for 0000, which 0000_template makes a real number.
type Number struct {
	digits string
}

// ParseNumber accepts exactly four ASCII digits.
func ParseNumber(s string) (Number, error) {
	if len(s) != numberDigits {
		return Number{}, fmt.Errorf("%w: number %q is not four ASCII digits", ErrInvalid, s)
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return Number{}, fmt.Errorf("%w: number %q is not four ASCII digits", ErrInvalid, s)
		}
	}
	return Number{digits: s}, nil
}

// String returns the four digits, or "" for the zero value.
func (n Number) String() string { return n.digits }

// Int returns the value of the number, or 0 for the zero value. It cannot
// tell the zero value from 0000; a caller that must tell a number from no
// number compares String.
func (n Number) Int() int {
	v, err := strconv.Atoi(n.digits)
	if err != nil {
		return 0
	}
	return v
}

var (
	// branchForm is a feature branch name: a GitHub user name, the task
	// directory name with "_" replaced by "-", and a two-digit sequence
	// number. The first group is the slug the directory name must give (the
	// number, a hyphen, and the name), the second the number.
	branchForm = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?/(([0-9]{4})(?:-[a-z0-9]+)+)-[0-9]{2}$`)
	// dirForm is a task directory name: the number, "_", and lowercase ASCII
	// words joined by "_".
	dirForm = regexp.MustCompile(`^([0-9]{4})_[a-z0-9]+(?:_[a-z0-9]+)*$`)
	// adrForm is an ADR file name: the number, "-", and lowercase ASCII words
	// joined by "-", with the ".md" extension.
	adrForm = regexp.MustCompile(`^([0-9]{4})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
)

// ParseBranch returns the number and the slug of a task branch name, and
// false for any other name.
func ParseBranch(name string) (Number, string, bool) {
	m := branchForm.FindStringSubmatch(name)
	if m == nil {
		return Number{}, "", false
	}
	// The form guarantees four ASCII digits, so ParseNumber cannot fail.
	n, _ := ParseNumber(m[2])
	return n, m[1], true
}

// ParseTaskDir returns the number of a task directory name, and false for any
// other name.
func ParseTaskDir(name string) (Number, bool) {
	m := dirForm.FindStringSubmatch(name)
	if m == nil {
		return Number{}, false
	}
	// The form guarantees four ASCII digits, so ParseNumber cannot fail.
	n, _ := ParseNumber(m[1])
	return n, true
}

// ParseADRFile returns the number of an ADR file name, and false for any
// other name. 0000 is a well-formed name here; the ADR guard rejects it
// itself, so that every other reader of the form agrees with the guard.
func ParseADRFile(name string) (Number, bool) {
	m := adrForm.FindStringSubmatch(name)
	if m == nil {
		return Number{}, false
	}
	// The form guarantees four ASCII digits, so ParseNumber cannot fail.
	n, _ := ParseNumber(m[1])
	return n, true
}
