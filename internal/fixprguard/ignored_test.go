//go:build test

package fixprguard

import (
	"strings"
	"testing"
)

// TestIsWatched: the rows come from the watch list table and the examples of
// the requirements, written out here rather than built from the data in
// ignored.go, so removing an item from the data fails its row. Every item of
// the table has a row.
func TestIsWatched(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// The top-level .claude directory, and .claude itself.
		{"claude_settings", ".claude/settings.local.json", true},
		{"claude_itself", ".claude", true},
		{"claude_deep", ".claude/worktrees/x/internal/a.txt", true},
		{"claude_not_top", "a/.claude/settings.local.json", false},
		{"claude_prefix_only", ".claude.bak/settings.local.json", false},

		// Top-level files.
		{"env", ".env", true},
		{"envrc", ".envrc", true},
		{"claude_local_md", "claude.local.md", true},
		{"env_not_top", "a/.env", false},
		{"envrc_not_top", "extension/.envrc", false},
		{"claude_local_md_not_top", "docs/CLAUDE.local.md", false},

		// Names in any directory.
		{"claude_md", "docs/CLAUDE.md", true},
		{"makefile", "extension/node_modules/pkg/Makefile", true},
		{"gnumakefile", "build/GNUmakefile", true},
		{"go_mod", "build/x/go.mod", true},
		{"go_work", "go.work", true},
		{"go_sum", "go.sum", false},
		{"agents_md", "AGENTS.md", false},

		// Suffixes in any directory.
		{"go", "build/x_test.go", true},
		{"mk", "build/rules.mk", true},
		{"c", "a/x.c", true},
		{"h", ".venv/include/x.h", true},
		{"cc", "a/x.cc", true},
		{"cpp", "a/x.cpp", true},
		{"cxx", "a/x.cxx", true},
		{"hh", "a/x.hh", true},
		{"hpp", "a/x.hpp", true},
		{"hxx", "a/x.hxx", true},
		{"m", "a/x.m", true},
		{"s", "a/x.s", true},
		{"sx", "a/x.sx", true},
		{"f", "a/x.f", true},
		{"for", "a/x.for", true},
		{"f90", "a/x.f90", true},
		{"swig", "a/x.swig", true},
		{"swigcxx", "a/x.swigcxx", true},
		{"syso", "build/payload.syso", true},
		{"name_is_only_suffix", "build/.go", true},
		{"suffix_not_at_end", "build/x.go.txt", false},

		// Case folding, ASCII and Unicode simple folding.
		{"ascii_upper_name", "build/MAKEFILE", true},
		{"ascii_upper_suffix", "build/X.GO", true},
		{"ascii_upper_top_dir", ".CLAUDE/settings.local.json", true},
		{"kelvin_sign", "go.wor\u212a", true},
		{"long_s", "a/x.\u017f", true},

		// Bytes that are not valid UTF-8, and control characters.
		{"invalid_utf8_before_suffix", "build/\xff.go", true},
		{"invalid_utf8_in_suffix", "build/x.g\xff", false},
		{"control_character", "build/a\nb.go", true},

		// Not on the list.
		{"ds_store", ".DS_Store", false},
		{"swap_file", ".x.swp", false},
		{"coverage", "coverage.out", false},
		{"vscode", ".vscode/settings.json", false},
		{"venv_python", ".venv/bin/python", false},
		{"node_module_js", "extension/node_modules/pkg/index.js", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWatched(tt.path); got != tt.want {
				t.Errorf("isWatched(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}

	// No watch list item contains U+FFFD or an invalid byte, so isWatched
	// cannot show how foldEqual treats them; these rows check foldEqual
	// itself. Reading every invalid byte as U+FFFD would equate the first
	// pair and the last.
	folds := []struct {
		name string
		a, b string
		want bool
	}{
		{"different_invalid_bytes", "x\xff", "x\xfe", false},
		{"same_invalid_byte", "x\xff", "x\xff", true},
		{"invalid_byte_and_replacement_rune", "x\xff", "x\ufffd", false},
	}
	for _, tt := range folds {
		t.Run("fold/"+tt.name, func(t *testing.T) {
			if got := foldEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("foldEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestWatchListPatternsMatch: every pattern the guide prints names paths
// isWatched matches, and the list has the 28 items of the requirements table
// (1 directory, 3 top-level files, 5 names, 2 suffixes, 17 Go build inputs).
func TestWatchListPatternsMatch(t *testing.T) {
	patterns := WatchListPatterns()
	if len(patterns) != 28 {
		t.Errorf("WatchListPatterns() has %d items, want 28: %q", len(patterns), patterns)
	}
	for _, pattern := range patterns {
		if p := watchSamplePath(pattern); !isWatched(p) {
			t.Errorf("WatchListPatterns lists %q, but %q is not watched", pattern, p)
		}
	}
}

// watchSamplePath returns a path that pattern, in the guide's notation,
// describes.
func watchSamplePath(pattern string) string {
	switch {
	case strings.HasSuffix(pattern, "/"):
		return pattern + "x"
	case strings.HasPrefix(pattern, "/"):
		return strings.TrimPrefix(pattern, "/")
	case strings.HasPrefix(pattern, "*"):
		return "a/b" + strings.TrimPrefix(pattern, "*")
	default:
		return "a/" + pattern
	}
}
