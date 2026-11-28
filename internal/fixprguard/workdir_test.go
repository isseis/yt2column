//go:build test

package fixprguard

import (
	"errors"
	"strings"
	"testing"
)

func TestNewWorktreeDir(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/repo", "/tmp/work/repo"} {
		t.Run("accept/"+path, func(t *testing.T) {
			t.Parallel()
			d, err := NewWorktreeDir(path)
			if err != nil {
				t.Fatalf("NewWorktreeDir(%q) = %v, want nil", path, err)
			}
			if got, want := d.joinName(".git"), path+"/.git"; got != want {
				t.Errorf("NewWorktreeDir(%q).joinName(\".git\") = %q, want %q", path, got, want)
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
			t.Parallel()
			d, err := NewWorktreeDir(tc.path)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("NewWorktreeDir(%q) err = %v, want ErrInvalidInput", tc.path, err)
			}
			if d.usable() {
				t.Errorf("NewWorktreeDir(%q) returned usable WorktreeDir", tc.path)
			}
			// Not echoing the rejected path is part of NewWorktreeDir's
			// observable contract, so err.Error() is checked directly here.
			if strings.Contains(err.Error(), marker) {
				t.Errorf("NewWorktreeDir(%q) error %q echoes input path", tc.path, err.Error())
			}
		})
	}
}

func TestWorktreeDirOperations(t *testing.T) {
	t.Parallel()
	absDir, err := NewWorktreeDir("/tmp/work")
	if err != nil {
		t.Fatalf("NewWorktreeDir: %v", err)
	}
	joinNames := []string{".git", "docs/a.md", "dir/"}
	relLocations := []string{".git/hooks", "../other/.git"}
	relNames := []string{"docs/a.md", "dir/", ".claude"}

	for _, mode := range []struct {
		name      string
		dir       WorktreeDir
		prefix    string
		rejectRel []string
	}{
		{
			name:   "current_dir",
			dir:    CurrentDir(),
			prefix: "",
		},
		{
			name:      "absolute_dir",
			dir:       absDir,
			prefix:    "/tmp/work/",
			rejectRel: []string{"/tmp/work", "/tmp/work-other/docs/a.md", "/elsewhere/docs/a.md", "docs/a.md"},
		},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			for _, name := range joinNames {
				if got, want := mode.dir.joinName(name), mode.prefix+name; got != want {
					t.Errorf("joinName(%q) = %q, want %q", name, got, want)
				}
			}
			for _, loc := range relLocations {
				if got, want := mode.dir.resolveLocation(loc), mode.prefix+loc; got != want {
					t.Errorf("resolveLocation(%q) = %q, want %q", loc, got, want)
				}
			}
			if got := mode.dir.resolveLocation("/abs/hooks"); got != "/abs/hooks" {
				t.Errorf("resolveLocation(\"/abs/hooks\") = %q, want \"/abs/hooks\"", got)
			}
			for _, rel := range relNames {
				osPath := mode.prefix + rel
				got, ok := mode.dir.relName(osPath)
				if !ok || got != rel {
					t.Errorf("relName(%q) = (%q, %t), want (%q, true)", osPath, got, ok, rel)
				}
			}
			for _, bad := range mode.rejectRel {
				if got, ok := mode.dir.relName(bad); ok {
					t.Errorf("relName(%q) = (%q, true), want (_, false)", bad, got)
				}
			}
		})
	}
}
