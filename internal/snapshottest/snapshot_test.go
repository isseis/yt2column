//go:build test

package snapshottest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s := NewStore(t.TempDir())
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close the store: %v", err)
		}
	})
	return s
}

// TestLoadBuildsOnce checks that parallel tests asking for the same kind and
// key share one build, and that each of them gets the built value.
func TestLoadBuildsOnce(t *testing.T) {
	s := newTestStore(t)
	kind := NewKind[int]("counter", nil)
	var builds atomic.Int32
	var wrong atomic.Int32
	t.Run("group", func(t *testing.T) {
		for i := range 16 {
			t.Run(fmt.Sprintf("asker-%d", i), func(t *testing.T) {
				t.Parallel()
				sn := Load(t, s, kind, []string{"k"}, func(_ *testing.T, _ string) int {
					builds.Add(1)
					time.Sleep(20 * time.Millisecond) // let the others arrive while it builds
					return 42
				})
				if sn.Value() != 42 {
					wrong.Add(1)
				}
			})
		}
	})
	if got := builds.Load(); got != 1 {
		t.Errorf("build ran %d times, want 1", got)
	}
	if got := wrong.Load(); got != 0 {
		t.Errorf("%d askers got another value", got)
	}
}

// TestLoadKeyParts checks that lists of parts that join to the same text are
// still different keys.
func TestLoadKeyParts(t *testing.T) {
	s := newTestStore(t)
	kind := NewKind[string]("parts", nil)
	keys := [][]string{{"ab", "c"}, {"a", "bc"}, {"a", "b", "c"}, {"abc"}, {"", "abc"}, {"abc", ""}}
	var builds atomic.Int32
	for _, key := range keys {
		sn := Load(t, s, kind, key, func(_ *testing.T, _ string) string {
			builds.Add(1)
			return fmt.Sprintf("%q", key)
		})
		if want := fmt.Sprintf("%q", key); sn.Value() != want {
			t.Errorf("key %q: got value %q, want %q", key, sn.Value(), want)
		}
	}
	if got := int(builds.Load()); got != len(keys) {
		t.Errorf("build ran %d times for %d different keys", got, len(keys))
	}
}

// TestValueIsCloned checks that a test that writes to the value it got does
// not change what later tests get.
func TestValueIsCloned(t *testing.T) {
	s := newTestStore(t)
	kind := NewKind("slice", slices.Clone[[]string])
	build := func(_ *testing.T, _ string) []string { return []string{"a", "b"} }
	first := Load(t, s, kind, []string{"k"}, build).Value()
	first[0] = "changed"
	snapshot := Load(t, s, kind, []string{"k"}, build)
	second := snapshot.Value()
	second[1] = "changed"
	if got := snapshot.Value(); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("value after writes to two copies = %q, want [a b]", got)
	}
}

// TestCopyTo checks that a copy keeps execute permission and symbolic links,
// is independent of the snapshot, and is refused over an existing path.
func TestCopyTo(t *testing.T) {
	s := newTestStore(t)
	kind := NewKind[struct{}]("tree", nil)
	snapshot := Load(t, s, kind, []string{"k"}, func(t *testing.T, dir string) struct{} {
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sub", "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec // the test needs an executable file
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data"), []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("sub/run.sh", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		return struct{}{}
	})

	first := filepath.Join(t.TempDir(), "copy")
	snapshot.CopyTo(t, first)
	info, err := os.Stat(filepath.Join(first, "sub", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("copied script mode = %v, want the execute bit", info.Mode().Perm())
	}
	target, err := os.Readlink(filepath.Join(first, "link"))
	if err != nil || target != "sub/run.sh" {
		t.Errorf("copied link = %q, %v; want sub/run.sh", target, err)
	}

	if err := os.WriteFile(filepath.Join(first, "data"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(t.TempDir(), "copy")
	snapshot.CopyTo(t, second)
	got, err := os.ReadFile(filepath.Join(second, "data"))
	if err != nil || string(got) != "original" {
		t.Errorf("data of a second copy = %q, %v; want original", got, err)
	}

	// CopyTo would end the test with t.Fatalf, so the refusal is checked on
	// the function it calls.
	if err := copyDir(snapshot.dir, first); !errors.Is(err, fs.ErrExist) {
		t.Errorf("copy over an existing destination = %v, want fs.ErrExist", err)
	}
}

const (
	childEnv = "SNAPSHOTTEST_CHILD_MODE"
	childKey = "child"
)

// TestChildScenario is the test the parent TestLoadPropagatesFailureAndSkip runs
// in a child process, since t.Fatal in a builder ends the process with a
// failure. It does nothing unless the parent set childEnv. The builder test
// asks for a snapshot whose build ends in the way the mode names; the
// latecomer test then asks for the same snapshot.
func TestChildScenario(t *testing.T) {
	mode := os.Getenv(childEnv)
	if mode == "" {
		t.Skip("runs only as a child of TestLoadPropagatesFailureAndSkip")
	}
	s := newTestStore(t)
	kind := NewKind[int]("scenario", nil)
	build := func(t *testing.T, dir string) int {
		switch mode {
		case "fatal":
			t.Fatal("the build fails")
		case "errorf":
			t.Errorf("the build fails and returns")
		case "skip":
			t.Skip("the build skips")
		case "errorf-skip":
			t.Errorf("the build fails")
			t.Skip("and then skips")
		case "goexit":
			runtime.Goexit()
		case "link-absolute":
			if err := os.Symlink("/etc/hosts", filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
		case "link-outside":
			if err := os.Symlink("../elsewhere", filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
		}
		return 1
	}
	t.Run("builder", func(t *testing.T) {
		if mode != "goexit" {
			Load(t, s, kind, []string{childKey}, build)
			return
		}
		// A Goexit that neither fails nor skips the builder's test would
		// end the whole test process, so the builder runs on a goroutine of
		// its own; the builder's test itself neither fails nor skips.
		done := make(chan struct{})
		go func() {
			defer close(done)
			Load(t, s, kind, []string{childKey}, build)
		}()
		<-done
	})
	t.Run("latecomer", func(t *testing.T) { Load(t, s, kind, []string{childKey}, build) })
}

var childResultLine = regexp.MustCompile(`--- (FAIL|SKIP|PASS): TestChildScenario/(builder|latecomer) `)

// TestLoadPropagatesFailureAndSkip checks, for each way a build can end, how
// the test that built the state and a test that came later end.
func TestLoadPropagatesFailureAndSkip(t *testing.T) {
	tests := []struct {
		mode               string
		builder, latecomer string
		exitFails          bool
		// latecomerMsg, when set, is text of the latecomer's failure message
		// that tells the end-before-result case from a type mismatch.
		latecomerMsg string
	}{
		{"fatal", "FAIL", "FAIL", true, ""},
		{"errorf", "FAIL", "FAIL", true, ""},
		{"skip", "SKIP", "SKIP", false, ""},
		{"errorf-skip", "FAIL", "FAIL", true, ""},
		{"goexit", "PASS", "FAIL", true, "failed to build in"},
		{"link-absolute", "FAIL", "FAIL", true, "holds a link outside itself"},
		{"link-outside", "FAIL", "FAIL", true, "holds a link outside itself"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestChildScenario$", "-test.v", "-test.count=1") //nolint:gosec // re-runs this test binary
			cmd.Env = append(os.Environ(), childEnv+"="+tt.mode)
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("child timed out:\n%s", out)
			}
			if exitErr, ok := errors.AsType[*exec.ExitError](err); tt.exitFails != ok {
				t.Errorf("child exit error = %v, want a failure exit: %v\n%s", exitErr, tt.exitFails, out)
			} else if !ok && err != nil {
				t.Fatalf("run child: %v", err)
			}
			if tt.latecomerMsg != "" && !bytes.Contains(out, []byte(tt.latecomerMsg)) {
				t.Errorf("child output lacks %q\n%s", tt.latecomerMsg, out)
			}
			got := map[string]string{}
			for _, m := range childResultLine.FindAllSubmatch(out, -1) {
				got[string(m[2])] = string(m[1])
			}
			if got["builder"] != tt.builder || got["latecomer"] != tt.latecomer {
				t.Errorf("builder/latecomer = %q/%q, want %q/%q\n%s",
					got["builder"], got["latecomer"], tt.builder, tt.latecomer, out)
			}
		})
	}
}

// TestCloseRemovesBuiltDirectories checks that Close removes what a build
// wrote, and that closing twice is not an error.
func TestCloseRemovesBuiltDirectories(t *testing.T) {
	s := NewStore(t.TempDir())
	kind := NewKind[string]("closing", nil)
	var built string
	Load(t, s, kind, []string{"k"}, func(_ *testing.T, dir string) string {
		built = dir
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return ""
	})
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := os.Stat(built); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("directory after Close: %v, want not exist", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("second close: %v", err)
	}
}

// TestLoadNilInterfaceValue checks that a build returning a nil value of an
// interface type is not read as a value of another type.
func TestLoadNilInterfaceValue(t *testing.T) {
	s := newTestStore(t)
	kind := NewKind[error]("nil-error", nil)
	sn := Load(t, s, kind, []string{"k"}, func(_ *testing.T, _ string) error { return nil })
	if sn.Value() != nil {
		t.Errorf("value = %v, want nil", sn.Value())
	}
}
