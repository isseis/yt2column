//go:build test

package codexreview

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLocateFindings checks the path and line checks on the findings of a
// final message: a path outside the root makes the whole message unreadable,
// a root reached through a symbolic link still matches, and InRange holds only
// for a changed regular file that has the end line.
func TestLocateFindings(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	root := filepath.Join(parent, "repo")
	for name, content := range map[string]string{
		"changed.go":    "one\ntwo\nthree\n",
		"unchanged.go":  "one\ntwo\nthree\n",
		"no_newline.go": "one\ntwo",
		"sub/nested.go": "one\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("create dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	// sub/link.go is a tracked symbolic link to a file with enough lines.
	if err := os.Symlink("../changed.go", filepath.Join(root, "sub", "link.go")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	// link reaches the root through a symbolic link, as /var reaches
	// /private/var on macOS.
	link := filepath.Join(parent, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	changed := map[string]struct{}{
		"changed.go":    {},
		"no_newline.go": {},
		"sub/nested.go": {},
		"deleted.go":    {},
		"sub/link.go":   {},
	}
	t.Chdir(root)
	finding := func(path string, end int) rawFinding {
		return rawFinding{path: path, start: 1, end: end, title: "T", body: "B"}
	}
	located := func(path string, end int, inRange bool) Finding {
		return Finding{Path: path, Start: 1, End: end, InRange: inRange, Title: "T", Body: "B"}
	}
	tests := []struct {
		name string
		raw  []rawFinding
		want []Finding // nil: the message is unreadable
	}{
		{"changed_file", []rawFinding{finding(root+"/changed.go", 3)}, []Finding{located("changed.go", 3, true)}},
		{"through_symlink", []rawFinding{finding(link+"/changed.go", 2)}, []Finding{located("changed.go", 2, true)}},
		{"nested_path", []rawFinding{finding(root+"/sub/nested.go", 1)}, []Finding{located("sub/nested.go", 1, true)}},
		{"last_line_without_newline", []rawFinding{finding(root+"/no_newline.go", 2)}, []Finding{located("no_newline.go", 2, true)}},
		{"line_past_end", []rawFinding{finding(root+"/changed.go", 4)}, []Finding{located("changed.go", 4, false)}},
		{"line_past_end_without_newline", []rawFinding{finding(root+"/no_newline.go", 3)}, []Finding{located("no_newline.go", 3, false)}},
		{"unchanged_file", []rawFinding{finding(root+"/unchanged.go", 1)}, []Finding{located("unchanged.go", 1, false)}},
		{"deleted_file", []rawFinding{finding(root+"/deleted.go", 1)}, []Finding{located("deleted.go", 1, false)}},
		{"deleted_file_through_symlink", []rawFinding{finding(link+"/gone/deleted.go", 1)}, []Finding{located("gone/deleted.go", 1, false)}},
		{"symbolic_link", []rawFinding{finding(root+"/sub/link.go", 1)}, []Finding{located("sub/link.go", 1, false)}},
		// The current directory is the root, so a relative path resolved
		// against it would name a changed file.
		{"relative_path", []rawFinding{finding("changed.go", 1)}, nil},
		{"outside_root", []rawFinding{finding(parent+"/other.go", 1)}, nil},
		{"dot_dot_escape", []rawFinding{finding(root+"/../other.go", 1)}, nil},
		{"parent_of_root", []rawFinding{finding(parent, 1)}, nil},
		{"root_itself", []rawFinding{finding(root, 1)}, nil},
		{"sibling_with_root_prefix", []rawFinding{finding(root+"x/a.go", 1)}, nil},
		{"one_outside_among_valid", []rawFinding{finding(root+"/changed.go", 1), finding("/etc/hosts", 1)}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := locateFindings(root, changed, tt.raw)
			if ok != (tt.want != nil) {
				t.Fatalf("locateFindings ok = %v, want %v (findings %#v)", ok, tt.want != nil, got)
			}
			if ok && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings = %#v, want %#v", got, tt.want)
			}
			if !ok && got != nil {
				t.Errorf("an unreadable message returned findings %#v", got)
			}
		})
	}
}
