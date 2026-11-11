//go:build !js

package writer

import (
	"fmt"
	"os"
	"syscall"
)

// overrideOpenFlags opens an override file read-only without blocking on a
// FIFO that has no writer (O_NONBLOCK) and without making a terminal the
// controlling terminal (O_NOCTTY). The type of the opened file is checked
// right after, so neither flag is relied on for anything but the open itself.
const overrideOpenFlags = os.O_RDONLY | syscall.O_NONBLOCK | syscall.O_NOCTTY

// readOverrideFile opens the override file, following symbolic links, and
// reads it with readOpenedFile. Every failure wraps both ErrInvalidTemplate
// and the underlying os error.
func readOverrideFile(src templateSource) ([]byte, error) {
	file, err := os.OpenFile(src.path, overrideOpenFlags, 0) //nolint:gosec // opens, read-only, the override file the caller named
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidTemplate, src, err)
	}
	defer func() { _ = file.Close() }()
	return readOpenedFile(src, file)
}
