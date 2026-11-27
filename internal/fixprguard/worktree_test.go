//go:build test

package fixprguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWorktreePanicsOnZeroWorktreeDir(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewWorktree with zero WorktreeDir did not panic")
		}
		// Match the panic message to distinguish the usable() guard panic from
		// an accidental runtime panic on the nil runners.
		if !strings.Contains(fmt.Sprint(r), "WorktreeDir") {
			t.Errorf("panic = %v, want message naming WorktreeDir", r)
		}
	}()
	NewWorktree(WorktreeDir{}, nil, nil, nil)
}

// TestCommitMessageEndsInOneLF: the message finish commits with a body file
// ends in exactly one LF, whether the body ends in one or not.
// TestFinishCommitBody shows git stores the message as built.
func TestCommitMessageEndsInOneLF(t *testing.T) {
	t.Parallel()
	const line = "Editorial correction: docs/x.md: a; no decision changed"
	subject := CommitSubject(mustPR(t)) + "\n\n"
	for _, tc := range []struct {
		name, body, want string
	}{
		{name: "ends_with_lf", body: line + "\n", want: subject + line + "\n"},
		{name: "no_trailing_lf", body: line + "\n" + line, want: subject + line + "\n" + line + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := commitMessage(FinishRequest{PR: mustPR(t), BodyFile: writeTextFile(t, tc.body)})
			if err != nil || got != (finishMessage{text: tc.want, pinned: true}) {
				t.Errorf("commitMessage = %+v, %v, want %q pinned", got, err, tc.want)
			}
		})
	}
}

func TestBeginRejectsDirtyWorkTree(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			r := newTestRepo(t)
			tc.edit(t, r)
			w, _ := newTestWorktree(t, r.Dir, r.Home)
			if _, err := w.Begin(t.Context()); !errors.Is(err, ErrDirtyWorkTree) {
				t.Errorf("Begin = %v, want ErrDirtyWorkTree", err)
			}
		})
	}
}

func TestInspectInScope(t *testing.T) {
	t.Parallel()
	r := newTestRepo(t)
	base := mustBaseline(t, r.Dir, r.Home)
	r.Write(testFile, "fixed\n")
	r.Write("docs/notes/c.md", "beside the thread's file\n")
	inspection := mustInspect(t, r.Dir, r.Home, base)
	if inspection.Changes != 2 || len(inspection.OutOfScope) != 0 {
		t.Errorf("Inspect = %+v, want 2 changes all in scope", inspection)
	}
}

func TestInspectListsOutOfScope(t *testing.T) {
	t.Parallel()
	r := newTestRepo(t)
	base := mustBaseline(t, r.Dir, r.Home)
	r.Write(testFile, "fixed\n")
	r.Write("internal/x/x.go", "package x\n\nfunc F() {}\n")
	r.Write("docs/other/data.bin", "\x00\x01")
	r.Write("docs/other/bad\x01name.md", "x\n")
	// Beside the thread's file, but an instruction file and a file outside
	// the allow list.
	r.Write("docs/notes/CLAUDE.md", "x\n")
	r.Write("docs/notes/b.ts", "x\n")
	inspection := mustInspect(t, r.Dir, r.Home, base)
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
