//go:build test

package main

import (
	"reflect"
	"testing"

	"github.com/isseis/yt2column/internal/godepstest"
	"github.com/isseis/yt2column/internal/llm/devfake"
	"github.com/isseis/yt2column/internal/server"
)

// TestNoDevDependency checks that this production binary does not link the
// fake LLM. godepstest reads the sources of the packages it lists, so a
// change to them, for any target OS, invalidates the cached result. The
// fake's path comes from its type, so a rename cannot empty the check.
func TestNoDevDependency(t *testing.T) {
	godepstest.Check(t, "./cmd/yt2column-server",
		[]string{reflect.TypeFor[server.Options]().PkgPath()},
		[]string{reflect.TypeFor[devfake.Scenario]().PkgPath()})
}
