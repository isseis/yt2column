//go:build test

package main

import (
	"reflect"
	"testing"

	"github.com/isseis/yt2column/internal/godepstest"
	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/nmhost"
)

// TestNoDevDependency checks that this production binary does not link the
// fake LLM or the development server. godepstest reads the sources of the packages it lists, so a
// change to them, for any target OS, invalidates the cached result. The
// fake's path comes from its type, so a rename cannot empty the check.
func TestNoDevDependency(t *testing.T) {
	godepstest.Check(t, "./cmd/yt2column-nmhost",
		[]string{reflect.TypeFor[nmhost.RelayOptions]().PkgPath()},
		[]string{
			reflect.TypeFor[devfake.Scenario]().PkgPath(),
			// A main package has no type to take its path from, so it is
			// looked up; the lookup fails when the package is gone.
			godepstest.MustPackagePath(t, "./cmd/yt2column-devserver"),
		})
}
