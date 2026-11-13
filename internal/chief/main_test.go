//go:build test

package chief

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates PATH before any test runs (isolateTestPath).
func TestMain(m *testing.M) {
	dirs, err := isolateTestPath()
	code := 1
	if err != nil {
		fmt.Fprintf(os.Stderr, "chief tests: %v\n", err)
	} else {
		code = m.Run()
	}
	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}
