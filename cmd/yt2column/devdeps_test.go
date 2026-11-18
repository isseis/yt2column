//go:build test

package main

import (
	"reflect"
	"testing"

	"github.com/isseis/yt2column/internal/godepstest"
	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/writer"
)

// TestNoDevDependency checks that this production binary does not link the
// fake LLM or the development backend of the article generation module. A
// main package cannot be imported, so the development article generation
// module and the development command get no entry. godepstest reads the
// sources of the packages it lists, so a change to them, for any target OS,
// invalidates the cached result. The fake's path comes from its type, and the
// backend's from a lookup that fails when the package is gone, so a rename
// cannot empty the check.
func TestNoDevDependency(t *testing.T) {
	godepstest.Check(t, "./cmd/yt2column",
		[]string{reflect.TypeFor[writer.Article]().PkgPath()},
		[]string{
			reflect.TypeFor[devfake.Scenario]().PkgPath(),
			// The development backend exports no type, so its path is looked
			// up; the lookup fails when the package is gone.
			godepstest.MustPackagePath(t, "./internal/extjob/devbackend"),
		})
}
