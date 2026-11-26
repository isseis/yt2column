//go:build test

package fixprguard

import (
	"errors"
	"strings"
	"testing"
)

func TestNewWorktreeDir(t *testing.T) {
	for _, path := range []string{"/repo", "/tmp/work/repo", "/a/b/c"} {
		t.Run("accept/"+path, func(t *testing.T) {
			d, err := NewWorktreeDir(path)
			if err != nil {
				t.Fatalf("NewWorktreeDir(%q) = %v, want nil", path, err)
			}
			if !d.usable() {
				t.Errorf("NewWorktreeDir(%q).usable() = false, want true", path)
			}
		})
	}

	const marker = "secret-marker-path"
	for _, tc := range []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"dot", "."},
		{"relative", marker + "/repo"},
		{"dot_relative", "./" + marker},
		{"root", "/"},
		{"trailing_slash", "/" + marker + "/"},
		{"double_slash", "/a//" + marker},
		{"leading_double_slash", "//" + marker},
		{"parent_elem", "/a/../" + marker},
		{"dot_elem", "/a/./" + marker},
	} {
		t.Run("reject/"+tc.name, func(t *testing.T) {
			d, err := NewWorktreeDir(tc.path)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("NewWorktreeDir(%q) err = %v, want ErrInvalidInput", tc.path, err)
			}
			if d.usable() {
				t.Errorf("NewWorktreeDir(%q) returned usable WorktreeDir", tc.path)
			}
			if strings.Contains(err.Error(), marker) {
				t.Errorf("NewWorktreeDir(%q) error %q echoes input path", tc.path, err.Error())
			}
		})
	}

	var zero WorktreeDir
	if zero.usable() {
		t.Errorf("zero WorktreeDir.usable() = true, want false")
	}
}

func TestWorktreeDirOperations(t *testing.T) {
	t.Run("current_dir", func(t *testing.T) {
		d := CurrentDir()
		if !d.usable() {
			t.Fatal("CurrentDir().usable() = false, want true")
		}
		for _, tc := range []struct {
			name string
			want string
		}{
			{".git", ".git"},
			{"docs/a.md", "docs/a.md"},
			{"dir/", "dir/"},
		} {
			if got := d.joinName(tc.name); got != tc.want {
				t.Errorf("CurrentDir().joinName(%q) = %q, want %q", tc.name, got, tc.want)
			}
		}
		for _, tc := range []struct {
			loc  string
			want string
		}{
			{".git/hooks", ".git/hooks"},
			{"../other/.git", "../other/.git"},
			{"/abs/hooks", "/abs/hooks"},
		} {
			if got := d.resolveLocation(tc.loc); got != tc.want {
				t.Errorf("CurrentDir().resolveLocation(%q) = %q, want %q", tc.loc, got, tc.want)
			}
		}
		for _, osPath := range []string{"docs/a.md", "dir/", ".claude"} {
			got, ok := d.relName(osPath)
			if !ok || got != osPath {
				t.Errorf("CurrentDir().relName(%q) = (%q, %t), want (%q, true)", osPath, got, ok, osPath)
			}
		}
	})

	t.Run("absolute_dir", func(t *testing.T) {
		d, err := NewWorktreeDir("/tmp/work")
		if err != nil {
			t.Fatalf("NewWorktreeDir: %v", err)
		}
		for _, tc := range []struct {
			name string
			want string
		}{
			{".git", "/tmp/work/.git"},
			{"docs/a.md", "/tmp/work/docs/a.md"},
			{"dir/", "/tmp/work/dir/"},
		} {
			if got := d.joinName(tc.name); got != tc.want {
				t.Errorf("joinName(%q) = %q, want %q", tc.name, got, tc.want)
			}
		}
		for _, tc := range []struct {
			loc  string
			want string
		}{
			{".git/hooks", "/tmp/work/.git/hooks"},
			{"../other/.git", "/tmp/work/../other/.git"},
			{"/abs/hooks", "/abs/hooks"},
		} {
			if got := d.resolveLocation(tc.loc); got != tc.want {
				t.Errorf("resolveLocation(%q) = %q, want %q", tc.loc, got, tc.want)
			}
		}
		for _, tc := range []struct {
			osPath string
			want   string
			wantOK bool
		}{
			{"/tmp/work/docs/a.md", "docs/a.md", true},
			{"/tmp/work/dir/", "dir/", true},
			{"/tmp/work/.claude", ".claude", true},
			{"/tmp/work", "", false},
			{"/tmp/work-other/docs/a.md", "", false},
			{"/elsewhere/docs/a.md", "", false},
			{"docs/a.md", "", false},
		} {
			got, ok := d.relName(tc.osPath)
			if ok != tc.wantOK || (tc.wantOK && got != tc.want) {
				t.Errorf("relName(%q) = (%q, %t), want (%q, %t)", tc.osPath, got, ok, tc.want, tc.wantOK)
			}
		}
	})
}
