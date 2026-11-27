//go:build test

package fixprguard

import (
	"os"
	"testing"

	"github.com/isseis/yt2column/internal/fixprguard/fixprguardtest"
)

// isolation is set by TestMain before any test runs. The process's working
// directory is an empty directory outside every git repository, so a test
// reads the package directory (testdata, sources) through isolation.PackageDir.
var isolation fixprguardtest.Isolation

func TestMain(m *testing.M) {
	os.Exit(fixprguardtest.Run(m, func(i fixprguardtest.Isolation) { isolation = i }))
}
