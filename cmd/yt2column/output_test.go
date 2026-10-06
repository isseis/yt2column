//go:build test

package main

import (
	"strings"
	"testing"
)

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
		{
			name:    "a secret equal to the marker does not survive in it",
			line:    "model=REDACTED",
			secrets: []string{"REDACTED"},
			want:    "model=!!!!!!!!!!",
		},
		{
			name:    "a secret equal to the whole marker does not survive in it",
			line:    "model=[REDACTED]",
			secrets: []string{"[REDACTED]"},
			want:    "model=!!!!!!!!!!",
		},
		{
			name:    "redacts a secret that escaping would otherwise synthesize",
			line:    "key=\x1b",
			secrets: []string{`\x1b`},
			want:    "key=[REDACTED]",
		},
		{
			name:    "redacts an invalid UTF-8 tail without lossy decoding",
			line:    string([]byte{'k', 'e', 'y', '=', '1', '2', '3', '4', '5', 0xff, 0xfe, 0xfd}),
			secrets: []string{string([]byte{'0', '1', '2', '3', '4', '5', 0xff, 0xfe, 0xfd})},
			want:    "key=[REDACTED]",
		},
		{
			name:    "a secret spanning the text before the marker does not survive",
			line:    "aa[",
			secrets: []string{"a["},
			want:    "a!!!!!!!!!!",
		},
		{
			name:    "a secret spanning the text after the marker does not survive",
			line:    "]xx",
			secrets: []string{"]x"},
			want:    "!!!!!!!!!!x",
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

func TestSanitizeSecretOfEveryPrintableByte(t *testing.T) {
	var secret []byte
	for b := byte(0x20); b <= 0x7e; b++ {
		secret = append(secret, b)
	}
	value := string(secret)
	line := "key=" + value
	got := sanitize(line, value)
	for i := range len(got) {
		if got[i] < 0x20 || got[i] > 0x7e {
			t.Fatalf("sanitize(%q) = %q: byte %#x is outside 0x20..0x7e", line, got, got[i])
		}
	}
	for _, protected := range append([]string{value}, value[len(value)-exposedTail:]) {
		if strings.Contains(got, protected) {
			t.Fatalf("sanitize(%q) = %q: protected value %q survived", line, got, protected)
		}
	}
}
