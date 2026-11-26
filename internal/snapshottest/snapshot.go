//go:build test

// Package snapshottest lets tests build an expensive directory state (a git
// repository with history, for example) once per test process and copy it
// into each test's own directory, instead of rebuilding it in every test.
// Built only with the test tag.
package snapshottest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// outcome is how a snapshot's build ended. The zero value is "not recorded":
// the build ended before it could record a result, which a later test reads
// as a failure.
type outcome int

const (
	outcomeUnrecorded outcome = iota
	outcomeBuilt
	outcomeFailed
	outcomeSkipped
)

// entry is the shared state of one kind and key: the directory the build
// wrote, the value it returned, and how it ended. done is closed once, by the
// builder, after every other field is set; readers touch the fields only
// after done is closed.
type entry struct {
	done    chan struct{}
	outcome outcome
	builder string // name of the test that ran the build
	dir     string
	value   any
	link    string // relative path of a link that leaves dir, when there is one
}

// Store keeps the directories that tests build once, one per kind and key,
// under a root the caller owns. It is safe for concurrent use by parallel
// tests.
type Store struct {
	root string

	mu      sync.Mutex
	entries map[string]*entry
	dirs    []string
}

// NewStore returns a store whose directories live under root.
func NewStore(root string) *Store {
	return &Store{root: root, entries: make(map[string]*entry)}
}

// Close removes every directory the store built.
func (s *Store) Close() error {
	s.mu.Lock()
	dirs := s.dirs
	s.dirs = nil
	s.mu.Unlock()
	var errs []error
	for _, dir := range dirs {
		errs = append(errs, os.RemoveAll(dir))
	}
	return errors.Join(errs...)
}

// Kind names one kind of shared state and fixes the type of its value.
// clone returns a copy of a value that shares no memory with it; it is nil
// for a value type that holds no slice, map, or pointer.
type Kind[V any] struct {
	name  string
	clone func(V) V
}

// NewKind returns a kind with the name the failure messages print.
func NewKind[V any](name string, clone func(V) V) Kind[V] {
	return Kind[V]{name: name, clone: clone}
}

// Snapshot is one directory built once and the value its build returned.
// It gives no path to the directory: a test can only copy it.
type Snapshot[V any] struct {
	kind  Kind[V]
	dir   string
	value V
}

// Load returns the snapshot of kind and key. The first test that asks for
// them builds it by calling build on its own goroutine, with its own t and
// a new directory dir; the others wait until that build ends. The parts of
// key are joined so that two different lists never make the same key.
//
// Everything the snapshot keeps must be under dir, and neither the files nor
// the value may refer to a path outside it: the builder's t.TempDir and
// t.Cleanup go away with the builder. Load fails the build when a symbolic
// link in dir points outside dir.
//
// A test that asks for a snapshot must not use t.Setenv: build reads the
// process environment, and a test that calls t.Setenv cannot run in parallel
// with the tests that might become the builder, so the environment a build
// sees would depend on which test asked first.
//
// The snapshot is built once per test process, so -count=N reuses it from the
// second run on; to look for flakiness in a build, run the process once per
// attempt.
func Load[V any](t *testing.T, s *Store, kind Kind[V], key []string,
	build func(t *testing.T, dir string) V,
) Snapshot[V] {
	t.Helper()
	id := kind.name + "\x00" + joinKey(key)

	s.mu.Lock()
	e, found := s.entries[id]
	if !found {
		e = &entry{done: make(chan struct{}), builder: t.Name()}
		s.entries[id] = e
	}
	s.mu.Unlock()

	if !found {
		runBuild(t, s, e, build)
	} else {
		<-e.done
	}

	switch e.outcome {
	case outcomeBuilt:
		v, ok := e.value.(V)
		if !ok {
			t.Fatalf("the shared %s was built with another value type in %s", kind.name, e.builder)
		}
		return Snapshot[V]{kind: kind, dir: e.dir, value: v}
	case outcomeSkipped:
		t.Skipf("the shared %s was skipped in %s", kind.name, e.builder)
	default:
		if e.link != "" {
			t.Fatalf("the shared %s holds a link outside itself: %s", kind.name, e.link)
		}
		t.Fatalf("the shared %s failed to build in %s", kind.name, e.builder)
	}
	return Snapshot[V]{}
}

// runBuild runs build for the test that found no entry and records how it
// ended. The record is written in a deferred function so that t.Fatal and
// t.Skip (runtime.Goexit) are recorded too; the entry is complete, and done
// closed, before the builder's goroutine leaves.
func runBuild[V any](t *testing.T, s *Store, e *entry, build func(t *testing.T, dir string) V) {
	t.Helper()
	finished := false
	defer func() {
		switch {
		case finished && !t.Failed():
			e.outcome = outcomeBuilt
		case t.Failed():
			// Failure is checked before skip: a test that fails and then
			// skips is reported by go test as failed.
			e.outcome = outcomeFailed
		case t.Skipped():
			e.outcome = outcomeSkipped
		default:
			// The build ended before recording a result (a panic or a
			// Goexit that neither failed nor skipped).
			e.outcome = outcomeFailed
		}
		close(e.done)
	}()

	dir, err := os.MkdirTemp(s.root, "snapshot-*")
	if err != nil {
		t.Fatalf("create the directory of the shared state: %v", err)
	}
	s.mu.Lock()
	s.dirs = append(s.dirs, dir)
	s.mu.Unlock()
	e.dir = dir

	value := build(t, dir)
	if link, err := findOutsideLink(dir); err != nil {
		t.Errorf("check the links of the shared state: %v", err)
	} else if link != "" {
		e.link = link
		t.Errorf("the shared state holds a link outside itself: %s", link)
	}
	e.value = value
	finished = true
}

// joinKey joins the parts so that two different lists never give the same
// key: every part is written with its length.
func joinKey(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(strconv.Itoa(len(p)))
		b.WriteByte(':')
		b.WriteString(p)
	}
	return b.String()
}

// findOutsideLink returns the path, relative to dir, of the first symbolic
// link under dir whose target is absolute or leaves dir, or "" when there is
// none. os.CopyFS writes a link's target as it was written, so such a link
// would point a copy at the original or at a path the copy does not own.
func findOutsideLink(dir string) (string, error) {
	var found string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if filepath.IsAbs(target) {
			found = rel
			return fs.SkipAll
		}
		resolved, err := filepath.Rel(dir, filepath.Join(filepath.Dir(path), target))
		if err != nil {
			return err
		}
		if resolved == ".." || strings.HasPrefix(resolved, ".."+string(filepath.Separator)) {
			found = rel
			return fs.SkipAll
		}
		return nil
	})
	return found, err
}

// copyDir copies dir to dst, which must not exist.
func copyDir(dir, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("copy destination %s: %w", dst, fs.ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.CopyFS(dst, os.DirFS(dir))
}

// CopyTo copies the snapshot's directory to dst, which must not exist.
func (sn Snapshot[V]) CopyTo(t *testing.T, dst string) {
	t.Helper()
	if err := copyDir(sn.dir, dst); err != nil {
		t.Fatalf("copy the shared %s: %v", sn.kind.name, err)
	}
}

// Value returns the value the build returned, cloned with the kind's clone.
func (sn Snapshot[V]) Value() V {
	if sn.kind.clone == nil {
		return sn.value
	}
	return sn.kind.clone(sn.value)
}
