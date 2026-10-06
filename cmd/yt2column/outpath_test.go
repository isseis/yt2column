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
		other := filepath.Join(cache, "other")
		if err := os.Mkdir(other, 0o700); err != nil {
			t.Fatal(err)
		}
		realCache := filepath.Join(base, "real", "cache")
		if err := os.MkdirAll(realCache, 0o700); err != nil {
			t.Fatal(err)
		}
		linkDir := filepath.Join(base, "linkdir")
		if err := os.Symlink(filepath.Join(base, "real"), linkDir); err != nil {
			t.Fatal(err)
		}
		dangling := filepath.Join(base, "dangling")
		if err := os.Symlink(filepath.Join(base, "missing"), dangling); err != nil {
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
			{"a cache path through a symbolic link", filepath.Join(linkDir, "cache", "article.md"), filepath.Join(linkDir, "cache"), true},
			{"a real path against a symlinked cache", filepath.Join(realCache, "article.md"), filepath.Join(linkDir, "cache"), true},
			{"a dangling symbolic link component", filepath.Join(dangling, "article.md"), cache, false},
			{"an outside sibling", filepath.Join(base, "article.md"), cache, false},
			{"the parent of the cache", base, cache, false},
			{"a non-directory in the middle", filepath.Join(afile, "article.md"), cache, false},
			{"dot dot after a non-directory", joinRaw(afile, "..", "other", "article.md"), cache, true},
			{"an existing sibling under a cache with a missing tail", filepath.Join(other, "article.md"), filepath.Join(cache, "new"), false},
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

	t.Run("unreadable directory on the way to --out", func(t *testing.T) {
		requireNonRoot(t)
		base := t.TempDir()
		cache := filepath.Join(base, "cache")
		if err := os.Mkdir(cache, 0o700); err != nil {
			t.Fatal(err)
		}
		blocked := filepath.Join(base, "blocked")
		if err := os.Mkdir(blocked, 0o700); err != nil {
			t.Fatal(err)
		}
		chmodForTest(t, blocked, 0o000)

		// Name comparison alone would report "outside"; only treating the
		// permission failure as unresolvable yields "inside".
		if !outPathInsideCacheDir(filepath.Join(blocked, "article.md"), cache) {
			t.Fatal("a permission failure must be treated as inside")
		}
	})
}
