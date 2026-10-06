//go:build test

package main

import "testing"

func TestSanitize(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		secrets []string
		want    string
	}{
		{
			name:    "redacts a whole secret value",
			line:    "request failed with key SECRETVALUE123",
			secrets: []string{"SECRETVALUE123"},
			want:    "request failed with key [REDACTED]",
		},
		{
			name:    "redacts the trailing eight characters of a long secret",
			line:    "Authorization: Bearer IJKLMNOP",
			secrets: []string{"ABCDEFGHIJKLMNOP"},
			want:    "Authorization: Bearer [REDACTED]",
		},
		{
			name:    "redacts the trailing eight characters of a multibyte secret",
			line:    "tail: bcdefghé",
			secrets: []string{"abcdefghé"},
			want:    "tail: [REDACTED]",
		},
		{
			name:    "redacts a secret of exactly eight bytes",
			line:    "key=12345678.",
			secrets: []string{"12345678"},
			want:    "key=[REDACTED].",
		},
		{
			name:    "redacts a secret shorter than eight bytes",
			line:    "key=abc",
			secrets: []string{"abc"},
			want:    "key=[REDACTED]",
		},
		{
			name:    "empty secret values are ignored",
			line:    "unchanged",
			secrets: []string{"", ""},
			want:    "unchanged",
		},
		{
			name:    "redacts a secret that holds control characters before escaping",
			line:    "x\x1b[2Jy",
			secrets: []string{"x\x1b[2Jy"},
			want:    "[REDACTED]",
		},
		{
			name:    "redacts several distinct secrets",
			line:    "k1=SECRETAAAA k2=SECRETBBBB",
			secrets: []string{"SECRETAAAA", "SECRETBBBB"},
			want:    "k1=[REDACTED] k2=[REDACTED]",
		},
		{
			name:    "redacts the longer secret before a shorter one that is its prefix",
			line:    "token=ABCDEFGH",
			secrets: []string{"ABCD", "ABCDEFGH"},
			want:    "token=[REDACTED]",
		},
		{
			name: "escapes an escape sequence",
			line: "a\x1b[2Jb",
			want: `a\x1b[2Jb`,
		},
		{
			name: "escapes a format character",
			line: "a\u202eb",
			want: `a\u202eb`,
		},
		{
			name: "escapes invalid UTF-8 bytes",
			line: string([]byte{'A', 0xff, 'B'}),
			want: `A\xffB`,
		},
		{
			name: "escapes a backslash",
			line: `a\x1b`,
			want: `a\\x1b`,
		},
		{
			name: "escapes a newline",
			line: "a\nb",
			want: `a\nb`,
		},
		{
			name: "leaves printable text unchanged",
			line: "hello, 世界",
			want: "hello, 世界",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitize(tc.line, tc.secrets...); got != tc.want {
				t.Fatalf("sanitize(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}
