//go:build test

package devfake_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/godepstest"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/privatefile"
)

// TestDependencies checks that the fake depends on no binary, on
// internal/privatefile, which only a native build compiles, and on neither the
// network nor external commands. godepstest reads the sources of the packages
// it lists, so a change to them, for any target OS, invalidates the cached
// result.
func TestDependencies(t *testing.T) {
	llmPath := reflect.TypeFor[llm.GenerateRequest]().PkgPath()
	module, ok := strings.CutSuffix(llmPath, "/internal/llm")
	if !ok {
		t.Fatalf("llm package path %q does not end in /internal/llm; the cmd pattern below would name no package", llmPath)
	}
	// The package path comes from its type, so a rename cannot leave a
	// pattern that names no package. This test package's imports are not
	// in the listing: go list -deps runs without -test.
	godepstest.Check(t, "./internal/llm/devfake",
		[]string{llmPath},
		[]string{
			reflect.TypeFor[privatefile.UnsafeError]().PkgPath(),
			module + "/cmd/...",
			"net",
			"os/exec",
		})
}
