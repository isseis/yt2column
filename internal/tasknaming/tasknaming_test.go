//go:build test

package tasknaming_test

import (
	"errors"
	"testing"

	"github.com/isseis/yt2column/internal/tasknaming"
)

// TestParseNumber reads the four-digit number and rejects every other form.
func TestParseNumber(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"0000", 0},
		{"0001", 1},
		{"0099", 99},
		{"0100", 100},
		{"9999", 9999},
	} {
		n, err := tasknaming.ParseNumber(tc.in)
		if err != nil || n.String() != tc.in || n.Int() != tc.want {
			t.Errorf("ParseNumber(%q) = %q, %d, %v; want %q, %d", tc.in, n.String(), n.Int(), err, tc.in, tc.want)
		}
	}
	for _, in := range []string{"", "1", "000", "00000", "00a0", "a000", "-001", "+001", " 001", "001 ", "\uff10\uff10\uff10\uff11", "\u0660\u0660\u0660\u0661"} {
		if n, err := tasknaming.ParseNumber(in); !errors.Is(err, tasknaming.ErrInvalid) || n.String() != "" {
			t.Errorf("ParseNumber(%q) = %q, %v; want ErrInvalid and the zero value", in, n.String(), err)
		}
	}
}

// TestZeroNumber holds that the zero value is an invalid number, not 0000.
func TestZeroNumber(t *testing.T) {
	var zero tasknaming.Number
	if zero.String() != "" || zero.Int() != 0 {
		t.Errorf("zero Number = %q, %d; want no digits and 0", zero.String(), zero.Int())
	}
	n, err := tasknaming.ParseNumber("0000")
	if err != nil {
		t.Fatalf("ParseNumber(0000): %v", err)
	}
	if zero == n {
		t.Error("0000 is indistinguishable from no number")
	}
}

// TestParseBranch reads the number and slug of a task branch name and
// rejects the names chief's TestBranchTask rejects.
func TestParseBranch(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		slug string
	}{
		{"isseis/0088-chief-of-staff-06", "0088", "0088-chief-of-staff"},
		{"isseis/0001-test-task-02", "0001", "0001-test-task"},
		{"user/0100-number-reservation-01", "0100", "0100-number-reservation"},
		{"a/0088-x-01", "0088", "0088-x"},
	} {
		n, slug, ok := tasknaming.ParseBranch(tc.in)
		if !ok || n.String() != tc.want || slug != tc.slug {
			t.Errorf("ParseBranch(%q) = %q, %q, %v; want %q, %q", tc.in, n.String(), slug, ok, tc.want, tc.slug)
		}
	}
	for _, in := range []string{
		"", "main", "0088-chief-of-staff-06", "isseis/0088-chief-of-staff", "isseis/0088-chief-of-staff-6",
		"isseis/088-chief-06", "isseis/0088-Chief-06", "a/b/0088-x-01", "-x/0088-x-01", "x-/0088-x-01",
		"my_user/0088-x-01", "isseis/0088-x-001", "isseis/0088-x-01-", "isseis/0088_x-01", "isseis/0088-x-01\n",
	} {
		if n, slug, ok := tasknaming.ParseBranch(in); ok || n.String() != "" || slug != "" {
			t.Errorf("ParseBranch(%q) = %q, %q, %v; want no number and false", in, n.String(), slug, ok)
		}
	}
}

// TestParseTaskDir reads the number of a task directory name and rejects
// every other form, 0000_template included in the accepted ones.
func TestParseTaskDir(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"0000_template", "0000"},
		{"0001_test_task", "0001"},
		{"0088_chief_of_staff", "0088"},
	} {
		n, ok := tasknaming.ParseTaskDir(tc.in)
		if !ok || n.String() != tc.want {
			t.Errorf("ParseTaskDir(%q) = %q, %v; want %q", tc.in, n.String(), ok, tc.want)
		}
	}
	for _, in := range []string{
		"", "0088", "0088-chief", "088_x", "00088_x", "0088_", "0088_Chief", "0088_x-1",
		"0088__x", "0088_x_", "x_0088", "0088_\uff58", "_0088_x", "0088_x\n",
	} {
		if n, ok := tasknaming.ParseTaskDir(in); ok || n.String() != "" {
			t.Errorf("ParseTaskDir(%q) = %q, %v; want no number and false", in, n.String(), ok)
		}
	}
}

// TestParseADRFile reads the number of an ADR file name and rejects every
// other form.
func TestParseADRFile(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"0000-zero.md", "0000"},
		{"0001-first.md", "0001"},
		{"0051-number-reservation.md", "0051"},
	} {
		n, ok := tasknaming.ParseADRFile(tc.in)
		if !ok || n.String() != tc.want {
			t.Errorf("ParseADRFile(%q) = %q, %v; want %q", tc.in, n.String(), ok, tc.want)
		}
	}
	for _, in := range []string{
		"", "0051.md", "0051-.md", "051-x.md", "00511-x.md", "0051-x.markdown", "0051-Number.md",
		"0051-number_reservation.md", "0051_number.md", "x0051-x.md", "0051-x.md.bak", "0051-x.md\n",
	} {
		if n, ok := tasknaming.ParseADRFile(in); ok || n.String() != "" {
			t.Errorf("ParseADRFile(%q) = %q, %v; want no number and false", in, n.String(), ok)
		}
	}
}
