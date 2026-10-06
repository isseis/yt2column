package main

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// redactedMarker replaces every occurrence of a secret value and its trailing
// characters in a line the CLI writes to standard error.
const redactedMarker = "[REDACTED]"

// exposedTail is how many trailing characters of a secret are redacted on
// their own, so a truncated key printed by an error is still hidden.
const exposedTail = 8

// sanitize prepares one line for standard error: it first redacts the secret
// values and their trailing characters, then escapes the characters that would let
// untrusted text move the terminal cursor or forge a line. Redaction runs first
// because escaping changes how a value looks, so a secret would no longer be
// found in the output.
func sanitize(line string, secretValues ...string) string {
	return escapeNonPrintable(redactSecrets(line, secretValues))
}

// redactSecrets replaces every secret and its trailing characters with the marker.
// Longer strings are replaced first, so a secret that contains another one as a
// substring is not broken apart before it is matched.
func redactSecrets(line string, secretValues []string) string {
	for _, secret := range secretReplacements(secretValues) {
		line = strings.ReplaceAll(line, secret, redactedMarker)
	}
	return line
}

// secretReplacements returns the distinct strings to redact, longest first: the
// value of each secret, and its trailing exposedTail characters when it is
// longer than that.
func secretReplacements(secretValues []string) []string {
	seen := make(map[string]struct{})
	var replacements []string
	add := func(value string) {
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		replacements = append(replacements, value)
	}
	for _, value := range secretValues {
		add(value)
		if runes := []rune(value); len(runes) > exposedTail {
			add(string(runes[len(runes)-exposedTail:]))
		}
	}
	slices.SortFunc(replacements, func(a, b string) int {
		return cmp.Compare(len(b), len(a))
	})
	return replacements
}

// escapeNonPrintable renders s with the escape format of strconv.Quote but
// without the surrounding quotes: bytes that are not valid UTF-8, runes that
// are not printable (control and format characters such as U+202E), and the
// backslash are escaped. Escaping the backslash keeps a literal "\x1b" in the
// input from being mistaken for an escaped ESC.
func escapeNonPrintable(s string) string {
	quoted := strconv.Quote(s)
	return quoted[1 : len(quoted)-1]
}
