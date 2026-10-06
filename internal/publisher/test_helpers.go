//go:build test

package publisher

import (
	"errors"
	"io"
	"os"
	"testing"
)

var errInjectedWrite = errors.New("injected write failure")

// newFilePublisherWithSeams returns a publisher for path whose temporary-file
// writer and link function are replaced; a nil seam keeps the production value.
func newFilePublisherWithSeams(
	t *testing.T,
	path string,
	wrapWriter func(io.Writer) io.Writer,
	link func(oldname, newname string) error,
) *FilePublisher {
	t.Helper()
	p, err := NewFilePublisher(path)
	if err != nil {
		t.Fatalf("NewFilePublisher: %v", err)
	}
	if wrapWriter != nil {
		p.wrapWriter = wrapWriter
	}
	if link != nil {
		p.link = link
	}
	return p
}

// failAfter returns a writer wrapper that passes through the first n bytes and
// then fails; onFail, if not nil, runs when the failure is first produced.
func failAfter(n int, onFail func()) func(io.Writer) io.Writer {
	return func(w io.Writer) io.Writer {
		return &failingWriter{w: w, left: n, onFail: onFail}
	}
}

type failingWriter struct {
	w      io.Writer
	left   int
	onFail func()
}

func (f *failingWriter) Write(b []byte) (int, error) {
	if f.left >= len(b) {
		f.left -= len(b)
		return f.w.Write(b)
	}
	n, err := f.w.Write(b[:f.left])
	f.left = 0
	if err != nil {
		return n, err
	}
	if f.onFail != nil {
		f.onFail()
	}
	return n, errInjectedWrite
}

// requireNonRoot fails the test when it runs as root, where permission
// failures cannot be produced. Such a run is an unsupported environment, so it
// fails instead of being skipped silently.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Fatal("unsupported test environment: permission-failure tests require a non-root user")
	}
}

// chmodForTest changes path's mode and restores the original mode at cleanup,
// so the test temp directory can be removed afterwards.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	original := info.Mode().Perm()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, original)
	})
}

// cancelAfterFirstWrite returns a writer wrapper that calls cancel once the
// first write has gone through and fails any later write, so a publisher that
// does not stop writing after its context ends reports the injected error
// instead of the context's.
func cancelAfterFirstWrite(cancel func()) func(io.Writer) io.Writer {
	return func(w io.Writer) io.Writer {
		return &cancelingWriter{w: w, cancel: cancel}
	}
}

type cancelingWriter struct {
	w      io.Writer
	cancel func()
	done   bool
}

func (c *cancelingWriter) Write(b []byte) (int, error) {
	if c.done {
		return 0, errInjectedWrite
	}
	n, err := c.w.Write(b)
	c.done = true
	c.cancel()
	return n, err
}
