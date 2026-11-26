//go:build test

package chief

import "testing"

// TestUnitWidth checks the display width of display units. The boundary
// rows are code points chosen by hand from the lines of Unicode 17.0.0's
// EastAsianWidth.txt (recorded in the plan), not from wideRanges, so a table
// built wrong from the file fails here.
func TestUnitWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want int
	}{
		{"ascii letter", "a", 1},
		{"kanji", "\u6f22", 2},
		{"hiragana", "\u3042", 2},
		{"fullwidth A", "\uff21", 2},
		{"stop sign mark", "\U0001F6D1", 2},
		{"orange diamond mark", "\U0001F536", 2},
		{"green circle mark", "\U0001F7E2", 2},
		{"ellipsis is ambiguous", "\u2026", 1},
		{"box drawing is ambiguous", "\u2500", 1},
		{"combining acute accent", "\u0301", 1},
		{"variation selector 16", "\ufe0f", 1},
		{"ideographic space is F", "\u3000", 2},
		// 10FD..10FF ; N and 1100..115F ; W and 1160..11FF ; N
		{"before hangul choseong", "\u10ff", 1},
		{"first hangul choseong", "\u1100", 2},
		{"last hangul choseong", "\u115f", 2},
		{"after hangul choseong", "\u1160", 1},
		// FF01..FF03 ; F, FF60 ; F, FF61 ; H, and no line for FF00
		{"before fullwidth forms", "\uff00", 1},
		{"first fullwidth form", "\uff01", 2},
		{"last fullwidth form", "\uff60", 2},
		{"first halfwidth form", "\uff61", 1},
		// 1F6D0..1F6D2 ; W and 1F6D3..1F6D4 ; N
		{"last of a wide emoji range", "\U0001F6D2", 2},
		{"after a wide emoji range", "\U0001F6D3", 1},
		// FA6E..FA6F ; W, an unassigned CJK code point listed explicitly
		{"unassigned CJK compatibility", "\ufa6e", 2},
		// 3347A..3FFFD ; W, and no line for 3FFFE
		{"last of plane 3", "\U0003FFFD", 2},
		{"after plane 3", "\U0003FFFE", 1},
		// 0376..0377 and 037A ; N, and no line for 0378
		{"listed on no line", "\u0378", 1},
		// 1FADF..1FAEA ; W; U+1FAE9 was assigned after Unicode 15.0
		{"wide after unicode 15.0", "\U0001FAE9", 2},
		// An escape counts as its length in bytes.
		{"long unicode escape", `\U000e0001`, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unitWidth(tt.text); got != tt.want {
				t.Errorf("unitWidth(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

// TestWideRangesSortedAndDisjoint checks the order isWide's binary search
// relies on, which the boundary rows of TestUnitWidth reach only in part.
func TestWideRangesSortedAndDisjoint(t *testing.T) {
	t.Parallel()
	for i, r := range wideRanges {
		if r.lo > r.hi {
			t.Errorf("range %d: %#x > %#x", i, r.lo, r.hi)
		}
		if i > 0 && wideRanges[i-1].hi >= r.lo {
			t.Errorf("range %d starts at %#x, not after %#x", i, r.lo, wideRanges[i-1].hi)
		}
	}
}
