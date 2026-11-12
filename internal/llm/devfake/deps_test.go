//go:build test

package devfake_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/godepstest"
	"github.com/isseis/yt2column/internal/llm"
)

// TestDependencies checks that the fake depends on no server-only package and
// no binary, so it stays usable after the server is gone, and on neither the
// network nor external commands. It lists the dependencies of the package
// this test belongs to, so a change to them invalidates the cached result.
func TestDependencies(t *testing.T) {
	llmPath := reflect.TypeFor[llm.GenerateRequest]().PkgPath()
	module, ok := strings.CutSuffix(llmPath, "/internal/llm")
	if !ok {
		t.Fatalf("llm package path %q does not end in /internal/llm; the forbidden paths below would name no package", llmPath)
	}
	godepstest.Check(t, "./internal/llm/devfake",
		[]string{llmPath},
		[]string{
			module + "/internal/server",
			module + "/internal/articlecache",
			module + "/internal/serversock",
			module + "/internal/privatefile",
			module + "/internal/nmhost",
			module + "/cmd/...",
			"net",
			"os/exec",
		})
}
