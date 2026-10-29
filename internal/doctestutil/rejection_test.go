//go:build test

package doctestutil

import (
	"errors"
	"strings"
	"testing"
)

// TestRejectionDisplayCategory checks the classification of a display cell:
// each category's spelling in each language maps to that category, and any
// other cell, a padded spelling included, maps to no category. The empty row
// also fails when a category's spelling is empty.
func TestRejectionDisplayCategory(t *testing.T) {
	for _, lang := range []struct {
		name     string
		spelling RejectionDisplaySpelling
		other    RejectionDisplaySpelling
	}{
		{"ja", RejectionDisplayJA, RejectionDisplayEN},
		{"en", RejectionDisplayEN, RejectionDisplayJA},
	} {
		s := lang.spelling
		for _, tc := range []struct {
			name string
			cell string
			want RejectionDisplay
		}{
			{"fixed message", s.FixedMessage, DisplayFixedMessage},
			{"unexpected reply", s.UnexpectedReply, DisplayUnexpectedReply},
			{"conditional form", s.FixedMessageUnlessNoRequestID, DisplayFixedMessageUnlessNoRequestID},
		} {
			t.Run(lang.name+"/"+tc.name, func(t *testing.T) {
				got, err := s.Category(tc.cell)
				if err != nil || got != tc.want {
					t.Errorf("Category(%q) = %v, %v; want %v", tc.cell, got, err, tc.want)
				}
			})
		}
		unknown := []struct{ name, cell string }{
			{"empty", ""},
			{"space around a spelling", " " + s.UnexpectedReply + " "},
			{"the other language's spelling", lang.other.FixedMessage},
			{"extra text after a spelling", s.FixedMessage + " x"},
			{"the conditional form without its closing parenthesis", dropLastRune(s.FixedMessageUnlessNoRequestID)},
		}
		// Japanese has no letter case, so the row applies only where upper
		// case changes the spelling.
		if upper := strings.ToUpper(s.FixedMessage); upper != s.FixedMessage {
			unknown = append(unknown, struct{ name, cell string }{"a different letter case", upper})
		}
		for _, tc := range unknown {
			t.Run(lang.name+"/"+tc.name, func(t *testing.T) {
				got, err := s.Category(tc.cell)
				if !errors.Is(err, ErrUnknownRejectionDisplay) {
					t.Errorf("Category(%q) = %v, %v; want ErrUnknownRejectionDisplay", tc.cell, got, err)
				}
			})
		}
	}
}

// dropLastRune returns s without its last character.
func dropLastRune(s string) string {
	runes := []rune(s)
	return string(runes[:len(runes)-1])
}
