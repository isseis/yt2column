//go:build test

package redact

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/secret"
)

// TestLine verifies that Line redacts secret values, their exposed tails, and
// the escaped spelling of both, and escapes non-printable input, including
// cases where the marker itself would collide with a secret.
func TestLine(t *testing.T) {
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
			if got := Line(tc.line, tc.secrets...); got != tc.want {
				t.Fatalf("Line(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

// TestLineSecretOfEveryPrintableByte verifies that, for a secret holding every
// printable ASCII byte, the chosen marker is printable and no protected value
// survives in the output.
func TestLineSecretOfEveryPrintableByte(t *testing.T) {
	var secretBytes []byte
	for b := byte(0x20); b <= 0x7e; b++ {
		secretBytes = append(secretBytes, b)
	}
	value := string(secretBytes)
	line := "key=" + value
	got := Line(line, value)
	for i := range len(got) {
		if got[i] < 0x20 || got[i] > 0x7e {
			t.Fatalf("Line(%q) = %q: byte %#x is outside 0x20..0x7e", line, got, got[i])
		}
	}
	for _, protected := range append([]string{value}, value[len(value)-exposedTail:]) {
		if strings.Contains(got, protected) {
			t.Fatalf("Line(%q) = %q: protected value %q survived", line, got, protected)
		}
	}
}

// TestSecretStrings verifies that SecretStrings redacts a secret's value and
// its exposed tail, and that a zero secret contributes nothing.
func TestSecretStrings(t *testing.T) {
	const key = "sk-ant-APIKEYVALUE-0123456789-TAIL8ANT"
	keySecret, err := secret.New(key)
	if err != nil {
		t.Fatalf("secret.New: %v", err)
	}
	got := SecretStrings(secret.Secret{}, keySecret)
	line := Line("POST with "+key+" failed", got...)
	if strings.Contains(line, key) {
		t.Errorf("Line left the secret value: %q", line)
	}
	tail := key[len(key)-8:]
	if strings.Contains(line, tail) {
		t.Errorf("Line left the exposed tail %q: %q", tail, line)
	}
	if got := SecretStrings(secret.Secret{}); len(got) != 0 {
		t.Errorf("SecretStrings(zero) = %v, want none", got)
	}
}

// TestSecretStringsWebhookParts verifies that the Webhook URL contributes its
// slackwebhook.SensitiveParts, so a line holding only its path is redacted.
func TestSecretStringsWebhookParts(t *testing.T) {
	const marker = "WEBHOOKVALUE"
	path := "/services/T0/B0/" + marker + "-TAIL8HKS"
	webhookURL, err := secret.New("https://hooks.slack.com" + path)
	if err != nil {
		t.Fatalf("secret.New: %v", err)
	}
	got := SecretStrings(webhookURL)
	line := Line("POST "+path+" failed", got...)
	if strings.Contains(line, marker) {
		t.Errorf("Line left part of the webhook path: %q", line)
	}
}

// TestWriterRedactsWithoutEscaping verifies that Writer replaces the protected
// strings in what it is given without escaping anything else, so text already
// escaped by log/slog is not escaped twice.
func TestWriterRedactsWithoutEscaping(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		secrets []string
		want    string
	}{
		{
			name:    "redacts a whole value",
			input:   "key=SECRETVALUE123",
			secrets: []string{"SECRETVALUE123"},
			want:    "key=[REDACTED]",
		},
		{
			name:    "redacts the exposed tail",
			input:   "token=IJKLMNOP",
			secrets: []string{"ABCDEFGHIJKLMNOP"},
			want:    "token=[REDACTED]",
		},
		{
			name:    "redacts the escaped spelling of a secret",
			input:   `key=\x1b`,
			secrets: []string{"\x1b"},
			want:    `key=[REDACTED]`,
		},
		{
			name:    "does not escape the rest of the input while redacting",
			input:   `key=SECRETVALUE123\tmore`,
			secrets: []string{"SECRETVALUE123"},
			want:    `key=[REDACTED]\tmore`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := Writer(&buf, tc.secrets)
			n, err := io.WriteString(w, tc.input)
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			if n != len(tc.input) {
				t.Errorf("Write reported %d bytes, want %d", n, len(tc.input))
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("Writer wrote %q, want %q", got, tc.want)
			}
		})
	}
}
