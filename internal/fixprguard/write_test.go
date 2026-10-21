//go:build test

package fixprguard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// writeTextFile writes content to a new file under a temporary directory and
// returns its path.
func writeTextFile(t *testing.T, content string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "text.md")
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestBodyFileChecks(t *testing.T) {
	symlink := func(t *testing.T) string {
		target := writeTextFile(t, "reply")
		link := filepath.Join(t.TempDir(), "link.md")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		return link
	}
	fifo := func(t *testing.T) string {
		name := filepath.Join(t.TempDir(), "fifo")
		if err := syscall.Mkfifo(name, 0o600); err != nil {
			t.Fatal(err)
		}
		return name
	}
	file := func(content string) func(*testing.T) string {
		return func(t *testing.T) string { return writeTextFile(t, content) }
	}
	atCap := strings.Repeat("a", maxTextFileBytes)
	for _, tc := range []struct {
		name    string
		path    func(*testing.T) string
		title   bool
		wantErr bool
	}{
		{name: "regular_file", path: file("Fixed in the next commit.\n")},
		{name: "at_size_cap", path: file(atCap)},
		{name: "over_size_cap", path: file(atCap + "a"), wantErr: true},
		{name: "symbolic_link", path: symlink, wantErr: true},
		{name: "fifo", path: fifo, wantErr: true},
		{name: "directory", path: func(t *testing.T) string { return t.TempDir() }, wantErr: true},
		{name: "missing", path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "none") }, wantErr: true},
		{name: "invalid_utf8", path: file("ok \xff"), wantErr: true},
		{name: "empty", path: file(""), wantErr: true},
		{name: "title_one_line", path: file("fix: a title"), title: true},
		{name: "title_newline", path: file("fix: a title\n"), title: true, wantErr: true},
		{name: "title_two_lines", path: file("fix: a title\nmore"), title: true, wantErr: true},
		{name: "title_carriage_return", path: file("fix: a\rtitle"), title: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := tc.path(t)
			read := readTextFile
			if tc.title {
				read = readTitleFile
			}
			got, err := read(name)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidInput) {
					t.Errorf("read = %v, want ErrInvalidInput", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("read = %v", err)
			}
			want, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("read returned %d bytes, want the file's %d bytes", len(got), len(want))
			}
		})
	}
}
