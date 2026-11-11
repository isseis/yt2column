//go:build js

package writer

import (
	"errors"
	"fmt"
)

// readOverrideFile never touches the file system: a js build has none to
// read an override file from. Every call fails with an error that matches both
// ErrInvalidTemplate and errors.ErrUnsupported, so a caller that names an
// override file gets a clear failure instead of a missing file.
func readOverrideFile(src templateSource) ([]byte, error) {
	return nil, fmt.Errorf("%w: %s: %w", ErrInvalidTemplate, src, errors.ErrUnsupported)
}
