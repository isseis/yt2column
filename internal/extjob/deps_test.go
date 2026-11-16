//go:build test

package extjob

import (
	"reflect"
	"testing"

	"github.com/isseis/yt2column/internal/godepstest"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

// TestModuleDependencies checks, on js/wasm (the platform the article
// generation modules are built for, whose js-only files a native listing does
// not see), that the release module links the Claude backend and neither the
// fake LLM nor the development backend, and that the development module
// links the development backend and neither the Claude adapter nor the
// release backend. Each check requires the backend its module must link, so
// a listing that missed the module's dependencies cannot pass by holding
// nothing forbidden. The backend packages import this package, so their paths
// are looked up, a lookup that fails when a package is gone.
func TestModuleDependencies(t *testing.T) {
	claudeBackend := godepstest.MustPackagePath(t, "./internal/extjob/claudebackend")
	devBackend := godepstest.MustPackagePath(t, "./internal/extjob/devbackend")
	claudeAdapter := godepstest.MustPackagePath(t, "./internal/llm/claude")
	fake := reflect.TypeFor[devfake.Scenario]().PkgPath()

	t.Run("release", func(t *testing.T) {
		godepstest.CheckWasm(t, "./cmd/yt2column-wasm",
			[]string{claudeBackend},
			[]string{fake, devBackend})
	})
	t.Run("development", func(t *testing.T) {
		godepstest.CheckWasm(t, "./cmd/yt2column-wasm-dev",
			[]string{devBackend},
			[]string{claudeAdapter, claudeBackend})
	})
}
