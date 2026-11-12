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
// fake LLM. It lists the dependencies of this package, the one its test
// belongs to, so a change to them invalidates the cached result. The fake's
// path comes from its type, so a rename cannot empty the check.
func TestNoDevDependency(t *testing.T) {
	godepstest.Check(t, "./cmd/yt2column",
		[]string{reflect.TypeFor[writer.Article]().PkgPath()},
		[]string{reflect.TypeFor[devfake.Scenario]().PkgPath()})
}
