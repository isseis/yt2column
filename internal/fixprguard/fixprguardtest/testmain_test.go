//go:build test

package fixprguardtest

import (
	"os"
	"testing"
)

var isolation Isolation

func TestMain(m *testing.M) {
	os.Exit(Run(m, func(i Isolation) { isolation = i }))
}
