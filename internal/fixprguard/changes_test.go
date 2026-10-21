//go:build test

package fixprguard

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// enumerate lists the changes against HEAD with a Worktree that runs no
// other command.
func enumerate(t *testing.T, r *testRepo) []change {
	t.Helper()
	w, _ := newTestWorktree(t)
	changes, err := w.enumerateChanges(t.Context(), r.Head())
	if err != nil {
		t.Fatalf("enumerateChanges: %v", err)
	}
	return changes
}

func TestEnumerateChanges(t *testing.T) {
	r := newTestRepo(t)
	r.Write("docs/notes/new.md", "x\ny")
	r.Write("docs/notes/staged.md", "s\n")
	r.Git("add", "docs/notes/staged.md")
	r.Write(testFile, "one\nTWO\nthree\n")
	if err := os.Remove(filepath.Join(r.Dir, "docs/notes/b.md")); err != nil {
		t.Fatal(err)
	}
	r.Git("mv", "internal/x/x.go", "internal/x/renamed.go")
	if err := os.Chmod(filepath.Join(r.Dir, "internal/x/x_test.go"), 0o700); err != nil { //nolint:gosec // an executable bit on a test file
		t.Fatal(err)
	}
	r.Write("docs/bin.dat", "a\x00b")
	if err := os.Symlink("a.md", filepath.Join(r.Dir, "docs/notes/link.md")); err != nil {
		t.Fatal(err)
	}
	// git does not list an untracked FIFO.
	if err := syscall.Mkfifo(filepath.Join(r.Dir, "docs/fifo"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := enumerate(t, r)
	type summary struct {
		kind    ChangeKind
		mode    string
		added   int
		deleted int
		binary  bool
	}
	want := map[string]summary{
		"docs/bin.dat":          {kind: ChangeAdd, mode: modeRegular, binary: true},
		testFile:                {kind: ChangeModify, mode: modeRegular, added: 2, deleted: 1},
		"docs/notes/b.md":       {kind: ChangeDelete, deleted: 1},
		"docs/notes/link.md":    {kind: ChangeOther, mode: modeSymlink},
		"docs/notes/new.md":     {kind: ChangeAdd, mode: modeRegular, added: 2},
		"docs/notes/staged.md":  {kind: ChangeAdd, mode: modeRegular, added: 1},
		"internal/x/renamed.go": {kind: ChangeAdd, mode: modeRegular, added: 1},
		"internal/x/x.go":       {kind: ChangeDelete, deleted: 1},
		"internal/x/x_test.go":  {kind: ChangeModify, mode: modeExecutable},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d changes, want %d: %+v", len(got), len(want), got)
	}
	for i, c := range got {
		if i > 0 && got[i-1].path >= c.path {
			t.Errorf("changes not in path order at %q", c.path)
		}
		w, ok := want[c.path]
		if !ok {
			t.Errorf("unexpected change %q", c.path)
			continue
		}
		s := summary{kind: c.kind, mode: c.mode, added: c.added, deleted: c.deleted, binary: c.binary}
		if c.kind == ChangeOther && w.mode == "" {
			s.mode = ""
		}
		if s != w {
			t.Errorf("%s = %+v, want %+v", c.path, s, w)
		}
		if (c.kind == ChangeDelete) != (c.digest == "") {
			t.Errorf("%s digest = %q", c.path, c.digest)
		}
	}

	t.Run("content_mode_and_link_change_the_id", func(t *testing.T) {
		before := judgedSetID(got)
		for _, edit := range []func(){
			func() { r.Write(testFile, "one\nTWO\nfour\n") },
			func() { _ = os.Chmod(filepath.Join(r.Dir, testFile), 0o700) }, //nolint:gosec // an executable bit on a test file
			func() {
				_ = os.Remove(filepath.Join(r.Dir, "docs/notes/link.md"))
				_ = os.Symlink("b.md", filepath.Join(r.Dir, "docs/notes/link.md"))
			},
		} {
			edit()
			after := judgedSetID(enumerate(t, r))
			if after == before {
				t.Errorf("judged set ID unchanged after an edit")
			}
			before = after
		}
	})
}

func TestEnumerateChangesFIFOReplacingTrackedFile(t *testing.T) {
	// git cannot hash a FIFO, so the enumeration fails closed instead of
	// judging the path.
	r := newTestRepo(t)
	if err := os.Remove(filepath.Join(r.Dir, "Makefile")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(r.Dir, "Makefile"), 0o600); err != nil {
		t.Fatal(err)
	}
	w, _ := newTestWorktree(t)
	if _, err := w.enumerateChanges(t.Context(), r.Head()); !errors.Is(err, ErrGitFailed) {
		t.Errorf("enumerateChanges = %v, want ErrGitFailed", err)
	}
}

func TestChangeCaps(t *testing.T) {
	t.Run("files", func(t *testing.T) {
		r := newTestRepo(t)
		for i := range maxChanges {
			r.Write("docs/many/"+strconv.Itoa(i)+".md", "x\n")
		}
		if got := enumerate(t, r); len(got) != maxChanges {
			t.Fatalf("got %d changes, want %d", len(got), maxChanges)
		}
		r.Write("docs/many/extra.md", "x\n")
		w, _ := newTestWorktree(t)
		_, err := w.enumerateChanges(t.Context(), r.Head())
		if !errors.Is(err, ErrLimitExceeded) || StopKind(err) != StopChangeCap {
			t.Errorf("enumerateChanges over the cap = %v, want the change cap", err)
		}
	})
	t.Run("digest_size", func(t *testing.T) {
		r := newTestRepo(t)
		r.Write("docs/exact.md", strings.Repeat("x", maxDigestBytes))
		r.Write("docs/over.md", strings.Repeat("x", maxDigestBytes+1))
		kinds := map[string]ChangeKind{}
		for _, c := range enumerate(t, r) {
			kinds[c.path] = c.kind
		}
		if kinds["docs/exact.md"] != ChangeAdd || kinds["docs/over.md"] != ChangeOther {
			t.Errorf("kinds = %v, want exact add and over other", kinds)
		}
	})
}

func TestParseRawOutputRejected(t *testing.T) {
	const rec = ":100644 100644 aaaaaaa 0000000 M\x00a.md\x00"
	for _, tc := range []struct {
		name  string
		parse func([]byte) error
		out   string
	}{
		{"raw_odd_fields", func(b []byte) error { _, err := parseRaw(b); return err }, ":100644 100644 a b M\x00"},
		{"raw_short_meta", func(b []byte) error { _, err := parseRaw(b); return err }, ":100644 M\x00a.md\x00"},
		{"raw_duplicate", func(b []byte) error { _, err := parseRaw(b); return err }, rec + rec},
		{"numstat_no_path", func(b []byte) error { _, err := parseNumstat(b); return err }, "1\t2\x00"},
		{"numstat_not_number", func(b []byte) error { _, err := parseNumstat(b); return err }, "x\t2\ta.md\x00"},
		{"numstat_duplicate", func(b []byte) error { _, err := parseNumstat(b); return err }, "1\t2\ta.md\x001\t2\ta.md\x00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.parse([]byte(tc.out)); !errors.Is(err, ErrGitFailed) {
				t.Errorf("parse = %v, want ErrGitFailed", err)
			}
		})
	}
}

func TestOpenRegularRefusesFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		f, err := openRegular(fifo, true)
		if err == nil {
			_ = f.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrGitFailed) {
			t.Errorf("openRegular(FIFO) = %v, want ErrGitFailed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("openRegular blocked on a FIFO")
	}
}

func TestParseJudgedSetID(t *testing.T) {
	valid := strings.Repeat("0a", digestLength/2)
	if id, err := ParseJudgedSetID(valid); err != nil || string(id) != valid {
		t.Errorf("ParseJudgedSetID(valid) = %q, %v", id, err)
	}
	for _, s := range []string{"", valid[1:], valid + "0", strings.ToUpper(valid), valid[:63] + "g"} {
		if _, err := ParseJudgedSetID(s); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("ParseJudgedSetID(%q) = %v, want ErrInvalidInput", s, err)
		}
	}
}
