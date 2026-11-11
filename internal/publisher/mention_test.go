//go:build test

package publisher

import (
	"slices"
	"testing"
)

func TestMentionAts(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []int
	}{
		{"at start", "@name", []int{0}},
		{"after a space", "hi @name", []int{3}},
		{"after punctuation", "(@name)", []int{1}},
		{"name starting with a digit", "@1abc", []int{0}},
		{"name starting with an underscore", "@_x", []int{0}},
		{"name starting with a dot", "@.x", []int{0}},
		{"name starting with a hyphen", "@-x", []int{0}},
		{"non-ASCII name", "@名前", []int{0}},
		{"two mentions", "@a @b", []int{0, 3}},

		{"at end", "x @", []int{2}},
		{"alone", "@", []int{0}},
		{"followed by a space", "@ name", []int{0}},
		{"followed by punctuation", "@!", []int{0}},
		{"followed by emphasis markup", "@**channel**", []int{0}},
		{"followed by a zero-width space", "@" + zeroWidthSpace + "name", []int{0}},

		{"after a letter", "Tech@Work", nil},
		{"email address", "user@example.com", nil},
		{"after a digit", "a1@b", nil},
		{"after a non-ASCII letter", "さん@xxx", nil},
		{"after a letter at end", "x@", nil},
		{"empty", "", nil},

		{"after an underscore", "foo_@bar", []int{4}},
		{"after an at sign", "x@@y", []int{2}},
		{"after a zero-width space", "a" + zeroWidthSpace + "@here", []int{2}},
		{"after a backslash", `a\@here`, []int{2}},
		{"after a slash", "youtube.com/@name", []int{12}},
		{"after a colon", "x:@y", []int{2}},
		{"after a full-width colon", "a：@b", []int{2}},
		{"after emphasis markup", "_a_@here", []int{3}},
		{"after a newline", "a\n@x", []int{2}},
		{"after a combining mark", "e\u0301@x", []int{2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mentionAts([]rune(tc.text))
			if !slices.Equal(got, tc.want) {
				t.Errorf("mentionAts(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestReplaceLookAlike(t *testing.T) {
	cases := []struct {
		name string
		in   rune
		want rune
	}{
		{"full-width at sign", '＠', '@'},
		{"small commercial at", '﹫', '@'},
		{"full-width less-than sign", '＜', '<'},
		{"small less-than sign", '﹤', '<'},
		{"full-width exclamation mark", '！', '!'},
		{"small exclamation mark", '﹗', '!'},
		{"at sign", '@', '@'},
		{"letter", 'a', 'a'},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := replaceLookAlike(tc.in); got != tc.want {
				t.Errorf("replaceLookAlike(%U) = %U, want %U", tc.in, got, tc.want)
			}
		})
	}
}
