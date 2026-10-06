//go:build test

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// joinRaw joins path components without cleaning, so a ".." survives to be
// resolved in the kernel's order rather than textually by the test itself.
func joinRaw(base string, parts ...string) string {
	return strings.Join(append([]string{base}, parts...), string(os.PathSeparator))
}

func TestOutPathInsideCacheDir(t *testing.T) {
	t.Run("existing cache directory", func(t *testing.T) {
		base := t.TempDir()
		cache := filepath.Join(base, "cache")
		if err := os.Mkdir(cache, 0o700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(base, "link")
		if err := os.Symlink(cache, link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(base, filepath.Join(cache, "up")); err != nil {
			t.Fatal(err)
		}
		afile := filepath.Join(base, "afile")
		if err := os.WriteFile(afile, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}

		cases := []struct {
			name  string
			out   string
			cache string
			want  bool
		}{
			{"the cache directory itself", cache, cache, true},
			{"a direct child", filepath.Join(cache, "article.md"), cache, true},
			{"a cache-shaped name", filepath.Join(cache, "dQw4w9WgXcQ.current"), cache, true},
			{"dot dot back into the cache", joinRaw(cache, "..", "cache", "article.md"), cache, true},
			{"through a symbolic link to the cache", filepath.Join(link, "article.md"), cache, true},
			{"the symbolic link itself", link, cache, true},
			{"a symbolic link before dot dot", joinRaw(cache, "up", "..", "cache", "article.md"), cache, false},
			{"an outside sibling", filepath.Join(base, "article.md"), cache, false},
			{"the parent of the cache", base, cache, false},
			{"a non-directory in the middle", filepath.Join(afile, "article.md"), cache, false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := outPathInsideCacheDir(tc.out, tc.cache); got != tc.want {
					t.Fatalf("outPathInsideCacheDir(%q, %q) = %v, want %v", tc.out, tc.cache, got, tc.want)
				}
			})
		}
	})

	t.Run("missing cache directory", func(t *testing.T) {
		base := t.TempDir()
		cache := filepath.Join(base, "cache")

		cases := []struct {
			name string
			out  string
			want bool
		}{
			{"a child of the missing cache", filepath.Join(cache, "article.md"), true},
			{"the missing cache itself", cache, true},
			{"a child differing only in case", filepath.Join(base, "Cache", "article.md"), true},
			{"the safe-side misjudgement", joinRaw(cache, "..", "article.md"), true},
			{"an outside sibling", filepath.Join(base, "article.md"), false},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := outPathInsideCacheDir(tc.out, cache); got != tc.want {
					t.Fatalf("outPathInsideCacheDir(%q, %q) = %v, want %v", tc.out, cache, got, tc.want)
				}
			})
		}
	})

	t.Run("relative path with the working directory inside the cache", func(t *testing.T) {
		base := t.TempDir()
		cache := filepath.Join(base, "cache")
		if err := os.Mkdir(cache, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Chdir(cache)

		if !outPathInsideCacheDir("article.md", cache) {
			t.Fatal("a relative path under the working directory inside the cache must be inside")
		}
		if outPathInsideCacheDir(filepath.Join(base, "article.md"), cache) {
			t.Fatal("an absolute path outside the cache must be outside")
		}
	})

	t.Run("unreadable cache directory", func(t *testing.T) {
		requireNonRoot(t)
		base := t.TempDir()
		cache := filepath.Join(base, "cache")
		if err := os.Mkdir(cache, 0o700); err != nil {
			t.Fatal(err)
		}
		chmodForTest(t, cache, 0o000)

		if !outPathInsideCacheDir(filepath.Join(cache, "article.md"), cache) {
			t.Fatal("a permission failure must be treated as inside")
		}
	})
}
