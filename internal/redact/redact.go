// Package redact replaces secret values in the text a program writes to
// output, and renders a line for a terminal.
package redact

import (
	"cmp"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/isseis/yt2column/internal/secret"
	"github.com/isseis/yt2column/internal/slackwebhook"
)

// redactedMarker replaces every occurrence of a secret value and its trailing
// characters in a line. It is used unless it would make a protected string
// appear in the result, in which case a collision-free marker is chosen
// instead (see chooseMarker).
const redactedMarker = "[REDACTED]"

// exposedTail is how many trailing characters of a secret are redacted on
// their own, so a truncated key printed by an error is still hidden. The tail
// is taken both by characters (for valid UTF-8) and by bytes (so a value with
// invalid UTF-8 is still hidden without lossy decoding).
const exposedTail = 8

// Line renders one line for a terminal: non-printable characters are escaped
// as strconv.Quote does, then every secret, its trailing characters, and their
// escaped spellings are replaced with a marker. (Moved from
// cmd/yt2column/output.go sanitize, unchanged.)
//
// Redacting the escaped rendering means escaping cannot synthesize a secret
// value that was absent from the original line: a secret whose literal value
// is "\x1b" is still found when the input holds an actual ESC byte, because
// escaping turns that byte into "\x1b". The replacements are made through an
// opaque placeholder and the marker is chosen so that no protected value can
// be reconstructed inside it or across its boundaries, so the marker cannot
// reintroduce a secret.
func Line(line string, secrets ...string) string {
	rendered := escapeNonPrintable(line)
	replacements := secretReplacements(secrets)
	if len(replacements) == 0 {
		return rendered
	}
	placeholder := choosePlaceholder(rendered, replacements)
	for _, replacement := range replacements {
		rendered = strings.ReplaceAll(rendered, replacement, placeholder)
	}
	return strings.ReplaceAll(rendered, placeholder, chooseMarker(rendered, placeholder, replacements))
}

// Writer returns an io.Writer that replaces, in every write, each secret, its
// trailing characters, and their escaped spellings with a marker before
// passing the bytes to w. It does not escape anything else, so text already
// quoted by log/slog is not quoted twice. The replacement set is computed
// once, at construction.
func Writer(w io.Writer, secrets []string) io.Writer {
	return &redactWriter{w: w, replacements: secretReplacements(secrets)}
}

// redactWriter replaces the protected strings in each write, without escaping.
type redactWriter struct {
	w            io.Writer
	replacements []string
}

// Write redacts p and writes the result to the underlying writer. It reports
// the length of p as consumed, not the length of the transformed output.
func (rw *redactWriter) Write(p []byte) (int, error) {
	if len(rw.replacements) == 0 {
		return rw.w.Write(p)
	}
	rendered := string(p)
	placeholder := choosePlaceholder(rendered, rw.replacements)
	for _, replacement := range rw.replacements {
		rendered = strings.ReplaceAll(rendered, replacement, placeholder)
	}
	rendered = strings.ReplaceAll(rendered, placeholder, chooseMarker(rendered, placeholder, rw.replacements))
	if _, err := io.WriteString(rw.w, rendered); err != nil {
		return 0, err
	}
	return len(p), nil
}

// SecretStrings returns the strings to redact for the given secrets: each
// value, and for webhookURL also its slackwebhook.SensitiveParts. A zero
// secret contributes nothing. (Moved from cmd/yt2column/run.go
// configuredSecrets, generalized.)
func SecretStrings(webhookURL secret.Secret, others ...secret.Secret) []string {
	var secrets []string
	for _, s := range others {
		if value, err := s.Reveal(); err == nil {
			secrets = append(secrets, value)
		}
	}
	if value, err := webhookURL.Reveal(); err == nil {
		secrets = append(secrets, value)
		secrets = append(secrets, slackwebhook.SensitiveParts(value)...)
	}
	return secrets
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

// chooseMarker returns a marker to put in place of placeholder in rendered
// (which already holds placeholder where each protected string occurred), such
// that the final result contains none of the protected strings. redactedMarker
// is preferred, then each printable ASCII byte (0x21..0x7e) repeated to the
// length of redactedMarker; bytes are tried in order so the choice is
// deterministic and the chosen marker is always printable. As an unreachable
// last resort it returns redactedMarker.
func chooseMarker(rendered, placeholder string, replacements []string) string {
	candidates := []string{redactedMarker}
	for b := byte('!'); b <= '~'; b++ {
		candidates = append(candidates, strings.Repeat(string([]byte{b}), len(redactedMarker)))
	}
	for _, candidate := range candidates {
		if !containsAnySubstring(strings.ReplaceAll(rendered, placeholder, candidate), replacements) {
			return candidate
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
