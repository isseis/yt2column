//go:build test

package main

import (
	"os"
	"testing"
)

// TestMain checks whether this process is a server child started by a signal
// test, and if so runs the server instead of the tests.
func TestMain(m *testing.M) {
	if mode, ok := os.LookupEnv(childModeEnv); ok {
		os.Exit(runChildMode(mode))
	}
	os.Exit(m.Run())
}
