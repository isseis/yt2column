//go:build test

package main

import (
	"os"
	"path/filepath"
	"slices"
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
		linkFile := filepath.Join(base, "linkfile")
		if err := os.Symlink(afile, linkFile); err != nil {
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
			{"dot dot after a symlink to a non-directory", joinRaw(linkFile, "..", "other", "article.md"), cache, true},
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
		// The kernel cannot resolve ".." below an unsearchable directory, so
		// collapsing it lexically would wrongly report "outside".
		if !outPathInsideCacheDir(joinRaw(blocked, "..", "article.md"), cache) {
			t.Fatal("a permission failure before dot dot must be treated as inside")
		}
	})
}

func TestOutPathInsideCacheDirCaseVariant(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "cache")
	variant := filepath.Join(base, "Cache")
	if err := os.Mkdir(variant, 0o700); err != nil {
		t.Fatal(err)
	}
	// On a case-insensitive filesystem the two spellings are the same
	// directory, so the distinction under test cannot exist.
	if _, err := os.Stat(cache); err == nil {
		t.Skip("filesystem is case-insensitive")
	}
	if !outPathInsideCacheDir(filepath.Join(variant, "article.md"), cache) {
		t.Fatal("an existing case variant of the missing cache tail must be inside")
	}
}

func TestAliasesCacheDescendant(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "cache")
	slot := filepath.Join(cache, "dQw4w9WgXcQ.a")
	if err := os.MkdirAll(slot, 0o700); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(base, "other")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	slotAlias := filepath.Join(base, "slot-alias")
	if err := os.Symlink(slot, slotAlias); err != nil {
		t.Fatal(err)
	}
	otherAlias := filepath.Join(base, "other-alias")
	if err := os.Symlink(other, otherAlias); err != nil {
		t.Fatal(err)
	}

	// The alias names the cache slot through an unrelated path; the identity
	// comparison must still see the descendant.
	if !aliasesCacheDescendant(slotAlias, cache) {
		t.Fatal("an alias of a cache descendant was not recognized")
	}
	if aliasesCacheDescendant(otherAlias, cache) {
		t.Fatal("an unrelated directory was reported as a cache descendant")
	}

	// A directory nested inside a slot, reached through an alias at an
	// unrelated path, is a cache descendant too: RemoveCache's recursive
	// removal would delete it through the slot, so the identity walk must
	// descend into a matched slot.
	nested := filepath.Join(slot, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	nestedAlias := filepath.Join(base, "nested-alias")
	if err := os.Symlink(nested, nestedAlias); err != nil {
		t.Fatal(err)
	}
	if !aliasesCacheDescendant(nestedAlias, cache) {
		t.Fatal("an alias of a directory nested inside a slot was not recognized")
	}

	// A symbolic link at a fixed-name slot must not be followed: otherwise a
	// single entry such as <id>.a -> the outside would make every --out fail.
	linkSlot := filepath.Join(cache, "aaaaaaaaaaa.a")
	if err := os.Symlink(base, linkSlot); err != nil {
		t.Fatal(err)
	}
	if aliasesCacheDescendant(base, cache) {
		t.Fatal("a symlink cache entry made an outside path a cache descendant")
	}

	// A directory whose name is not a fixed-name slot is ignored, even when
	// its identity would otherwise compare as the directory being checked.
	plainDir := filepath.Join(cache, "not-a-slot")
	if err := os.Mkdir(plainDir, 0o700); err != nil {
		t.Fatal(err)
	}
	plainAlias := filepath.Join(base, "plain-alias")
	if err := os.Symlink(plainDir, plainAlias); err != nil {
		t.Fatal(err)
	}
	if aliasesCacheDescendant(plainAlias, cache) {
		t.Fatal("a non-cache-named directory was reported as a cache descendant")
	}
}

func TestInsideResolvedMissingCacheTailIgnoresSlotAlias(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "cache")
	slot := filepath.Join(cache, "dQw4w9WgXcQ.a")
	if err := os.MkdirAll(slot, 0o700); err != nil {
		t.Fatal(err)
	}
	// A slot-shaped entry that is an existing sibling under the cache's
	// existing prefix. An alias of it must not make an unrelated path look
	// inside when the full cache path (cache plus a missing tail) does not
	// exist yet, because no slot can exist below a missing component.
	alias := filepath.Join(base, "slot-alias")
	if err := os.Symlink(slot, alias); err != nil {
		t.Fatal(err)
	}
	if insideResolved(alias, nil, cache, []string{"new"}) {
		t.Fatal("a slot alias made a path look inside a cache with a missing tail")
	}
	// Without a missing tail the same alias is inside, so the test would
	// catch a change that stopped reporting slot aliases at all.
	if !insideResolved(alias, nil, cache, nil) {
		t.Fatal("a slot alias was not recognized when the cache path resolves completely")
	}
}

func TestIsAncestorFollowsAliases(t *testing.T) {
	base := t.TempDir()
	cache := filepath.Join(base, "cache")
	if err := os.MkdirAll(filepath.Join(cache, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(base, link); err != nil {
		t.Fatal(err)
	}

	// The same directories reached by another path spelling: the identity walk
	// must still see the ancestor.
	if !isAncestor(cache, filepath.Join(link, "cache", "sub")) {
		t.Fatal("a descendant reached through an alias was not recognized")
	}
	if isAncestor(cache, filepath.Join(base, "other")) {
		t.Fatal("an unrelated directory was reported as a descendant")
	}
	got, ok := pathFrom(link, cache)
	if !ok || !slices.Equal(got, []string{"cache"}) {
		t.Fatalf("pathFrom(link, cache) = %v, %v; want [cache], true", got, ok)
	}
}
