package main

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// redactedMarker replaces every occurrence of a secret value and its trailing
// characters in a line the CLI writes to standard error. It is used unless a
// protected string would appear inside it, in which case a collision-free
// marker is chosen instead (see chooseMarker).
const redactedMarker = "[REDACTED]"

// exposedTail is how many trailing characters of a secret are redacted on
// their own, so a truncated key printed by an error is still hidden. The tail
// is taken both by characters (for valid UTF-8) and by bytes (so a value with
// invalid UTF-8 is still hidden without lossy decoding).
const exposedTail = 8

// sanitize prepares one line for standard error. It renders the line with the
// same escaping as strconv.Quote (see escapeNonPrintable), then replaces every
// secret value, its trailing characters, and the escaped spelling of both with
// the redaction marker.
//
// Redacting the escaped rendering means escaping cannot synthesize a secret
// value that was absent from the original line: a secret whose literal value
// is "\x1b" is still found when the input holds an actual ESC byte, because
// escaping turns that byte into "\x1b". The replacements are made through an
// opaque placeholder and the marker is chosen so that no protected value is a
// substring of it, so the marker cannot reintroduce a secret.
func sanitize(line string, secretValues ...string) string {
	rendered := escapeNonPrintable(line)
	replacements := secretReplacements(secretValues)
	if len(replacements) == 0 {
		return rendered
	}
	placeholder := choosePlaceholder(rendered, replacements)
	for _, replacement := range replacements {
		rendered = strings.ReplaceAll(rendered, replacement, placeholder)
	}
	return strings.ReplaceAll(rendered, placeholder, chooseMarker(replacements))
}

// secretReplacements returns the distinct strings to redact, longest first:
// the value of each secret, its trailing exposedTail characters taken by
// character and by byte, and the escaped rendering of each. Byte-based
// suffixes matter because a secret may hold invalid UTF-8, which a rune
// conversion would replace with U+FFFD and then fail to match.
func secretReplacements(secretValues []string) []string {
	seen := make(map[string]struct{})
	var replacements []string
	add := func(value string) {
		if value == "" {
			return
		}
		for _, candidate := range []string{value, escapeNonPrintable(value)} {
			if candidate == "" {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			replacements = append(replacements, candidate)
		}
	}
	for _, value := range secretValues {
		add(value)
		if runes := []rune(value); len(runes) > exposedTail {
			add(string(runes[len(runes)-exposedTail:]))
		}
		if len(value) > exposedTail {
			add(value[len(value)-exposedTail:])
		}
	}
	slices.SortFunc(replacements, func(a, b string) int {
		return cmp.Compare(len(b), len(a))
	})
	return replacements
}

// choosePlaceholder returns a string that does not occur in rendered and
// contains none of the protected strings, so replacing a protected string with
// it cannot collide with the text around it or with another replacement. The
// escaped rendering of a line never holds a raw control byte, so one is
// normally available.
func choosePlaceholder(rendered string, replacements []string) string {
	for b := range 256 {
		candidate := string([]byte{byte(b)})
		if strings.Contains(rendered, candidate) {
			continue
		}
		if containsAnySubstring(candidate, replacements) {
			continue
		}
		return candidate
	}
	return "\x00\x01\x02"
}

// chooseMarker returns redactedMarker unless a protected string is a substring
// of it, in which case a secret equal to the marker (or a part of it) would
// survive redaction. The alternative is built from a printable byte that
// occurs in no protected string, so no protected value can be a substring of
// the marker. Bytes are tried in order so the choice is deterministic.
func chooseMarker(replacements []string) string {
	if !containsAnySubstring(redactedMarker, replacements) {
		return redactedMarker
	}
	for b := byte('!'); b <= '~'; b++ {
		if marker := strings.Repeat(string([]byte{b}), len(redactedMarker)); !containsAnySubstring(marker, replacements) {
			return marker
		}
	}
	for b := range 256 {
		if marker := strings.Repeat(string([]byte{byte(b)}), len(redactedMarker)); !containsAnySubstring(marker, replacements) {
			return marker
		}
	}
	return redactedMarker
}

// containsAnySubstring reports whether any of substrings occurs in s.
func containsAnySubstring(s string, substrings []string) bool {
	for _, substring := range substrings {
		if substring != "" && strings.Contains(s, substring) {
			return true
		}
	}
	return false
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
