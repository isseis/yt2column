//go:build test

package fixprguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBeginRejectsDirtyWorkTree(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(t *testing.T, r *testRepo)
	}{
		{"untracked", func(_ *testing.T, r *testRepo) { r.Write("docs/new.md", "x\n") }},
		{"modified", func(_ *testing.T, r *testRepo) { r.Write(testFile, "changed\n") }},
		{"staged", func(_ *testing.T, r *testRepo) {
			r.Write(testFile, "changed\n")
			r.Git("add", testFile)
		}},
		{"deleted", func(t *testing.T, r *testRepo) {
			if err := os.Remove(filepath.Join(r.Dir, testFile)); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			tc.edit(t, r)
			w, _ := newTestWorktree(t)
			if _, err := w.Begin(t.Context()); !errors.Is(err, ErrDirtyWorkTree) {
				t.Errorf("Begin = %v, want ErrDirtyWorkTree", err)
			}
		})
	}
}

func TestInspectInScope(t *testing.T) {
	r := newTestRepo(t)
	base := mustBaseline(t)
	r.Write(testFile, "fixed\n")
	r.Write("docs/notes/c.md", "beside the thread's file\n")
	inspection := mustInspect(t, base)
	if inspection.Changes != 2 || len(inspection.OutOfScope) != 0 {
		t.Errorf("Inspect = %+v, want 2 changes all in scope", inspection)
	}
}

func TestInspectListsOutOfScope(t *testing.T) {
	r := newTestRepo(t)
	base := mustBaseline(t)
	r.Write(testFile, "fixed\n")
	r.Write("internal/x/x.go", "package x\n\nfunc F() {}\n")
	r.Write("docs/other/data.bin", "\x00\x01")
	r.Write("docs/other/bad\x01name.md", "x\n")
	// Beside the thread's file, but an instruction file and a file outside
	// the allow list.
	r.Write("docs/notes/CLAUDE.md", "x\n")
	r.Write("docs/notes/b.ts", "x\n")
	inspection := mustInspect(t, base)
	want := []ScopedChange{
		{Position: 1, Path: "docs/notes/CLAUDE.md", Kind: ChangeAdd, Added: 1},
		{Position: 3, Path: "docs/notes/b.ts", Kind: ChangeAdd, Added: 1},
		{Position: 4, Kind: ChangeAdd, Added: 1},
		{Position: 5, Path: "docs/other/data.bin", Kind: ChangeAdd, Binary: true},
		{Position: 6, Path: "internal/x/x.go", Kind: ChangeModify, Added: 2},
	}
	if inspection.Changes != 6 {
		t.Errorf("Changes = %d, want 6", inspection.Changes)
	}
	if len(inspection.OutOfScope) != len(want) {
		t.Fatalf("OutOfScope = %+v, want %+v", inspection.OutOfScope, want)
	}
	for i := range want {
		if inspection.OutOfScope[i] != want[i] {
			t.Errorf("OutOfScope[%d] = %+v, want %+v", i, inspection.OutOfScope[i], want[i])
		}
	}
}
