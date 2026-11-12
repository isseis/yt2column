//go:build test

package devfake_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/articlecache"
	"github.com/isseis/yt2column/internal/godepstest"
	"github.com/isseis/yt2column/internal/llm"
	"github.com/isseis/yt2column/internal/nmhost"
	"github.com/isseis/yt2column/internal/privatefile"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/serversock"
)

// TestDependencies checks that the fake depends on no server-only package and
// no binary, so it stays usable after the server is gone, and on neither the
// network nor external commands. It lists the dependencies of the package
// this test belongs to, so a change to them invalidates the cached result.
func TestDependencies(t *testing.T) {
	llmPath := reflect.TypeFor[llm.GenerateRequest]().PkgPath()
	module, ok := strings.CutSuffix(llmPath, "/internal/llm")
	if !ok {
		t.Fatalf("llm package path %q does not end in /internal/llm; the cmd pattern below would name no package", llmPath)
	}
	// The server-only paths come from their types, so a rename cannot leave
	// a pattern that names no package. This test package's imports are not
	// in the listing: go list -deps runs without -test.
	godepstest.Check(t, "./internal/llm/devfake",
		[]string{llmPath},
		[]string{
			reflect.TypeFor[server.Options]().PkgPath(),
			reflect.TypeFor[articlecache.Cache]().PkgPath(),
			reflect.TypeFor[serversock.Lock]().PkgPath(),
			reflect.TypeFor[privatefile.UnsafeError]().PkgPath(),
			reflect.TypeFor[nmhost.RelayOptions]().PkgPath(),
			module + "/cmd/...",
			"net",
			"os/exec",
		})
}
