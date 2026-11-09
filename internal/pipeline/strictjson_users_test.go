//go:build test

package pipeline

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// goSource is one Go source file found by the walk, with the package (the
// slash-separated directory relative to the walk root) it belongs to.
type goSource struct {
	pkg  string
	path string
}

// goSources returns every Go source file under root/cmd and root/internal,
// with its slash-separated path relative to root. It returns a file whose name
// ends in _test.go and a file whose build constraints exclude it from the
// current build, because the caller decides what to read. It skips a directory
// named testdata, a directory or file whose name starts with "_" or ".",
// everything below such a directory, and a file outside cmd/ and internal/.
// The name rules apply only to the entries below cmd/ and internal/, not to
// root, so a root whose own path starts with "." or "_" is still walked.
func goSources(root string) ([]goSource, error) {
	var sources []goSource
	for _, base := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, base), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipWalkName(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if skipWalkName(d.Name()) || !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			sources = append(sources, goSource{pkg: filepath.ToSlash(filepath.Dir(rel)), path: rel})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return sources, nil
}

// skipWalkName reports whether the walk skips an entry with this name: the
// testdata directory and every name that starts with "_" or ".".
func skipWalkName(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".")
}

// TestGoSources checks which files the walk returns and which it skips.
func TestGoSources(t *testing.T) {
	root := filepath.Join(t.TempDir(), "_root")
	writeSourceFiles(t, root, map[string]string{
		"cmd/tool/main.go":            "package main",
		"cmd/tool/main_windows.go":    "package main",
		"cmd/tool/tagged.go":          "//go:build test\n\npackage main",
		"cmd/tool/ignored.go":         "//go:build ignore\n\npackage main",
		"cmd/tool/x_test.go":          "package main",
		"cmd/tool/_skip.go":           "package main",
		"cmd/tool/.skip.go":           "package main",
		"cmd/tool/testdata/sample.go": "package sample",
		"cmd/tool/_hidden/sample.go":  "package sample",
		"cmd/tool/.hidden/sample.go":  "package sample",
		"top.go":                      "package top",
		"other/lib.go":                "package other",
		"internal/lib/lib.go":         "package lib",
		"internal/lib/lib_test.go":    "package lib",
		"internal/lib/testdata/t.go":  "package t",
		"internal/lib/_nested/n.go":   "package n",
	})

	got, err := goSources(root)
	if err != nil {
		t.Fatalf("goSources returned error: %v", err)
	}
	want := []goSource{
		{pkg: "cmd/tool", path: "cmd/tool/ignored.go"},
		{pkg: "cmd/tool", path: "cmd/tool/main.go"},
		{pkg: "cmd/tool", path: "cmd/tool/main_windows.go"},
		{pkg: "cmd/tool", path: "cmd/tool/tagged.go"},
		{pkg: "cmd/tool", path: "cmd/tool/x_test.go"},
		{pkg: "internal/lib", path: "internal/lib/lib.go"},
		{pkg: "internal/lib", path: "internal/lib/lib_test.go"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("goSources returned %v, want %v", got, want)
	}

	empty := t.TempDir()
	writeSourceFiles(t, empty, map[string]string{
		"cmd/.keep":      "",
		"internal/.keep": "",
	})
	got, err = goSources(empty)
	if err != nil {
		t.Fatalf("goSources on an empty tree returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("goSources on an empty tree returned %v, want none", got)
	}
}

// writeSourceFiles writes each path (slash-separated, relative to root) with
// its content, creating the parent directories.
func writeSourceFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create the directory of %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}
