//go:build test

package coveragecompare

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestFakePathHasNoRealToolchain checks that the commands Run starts by name
// resolve, on the test process's PATH, to the fakes, so no test of the
// package can run the repository's real test suites; only git's programs
// are real.
func TestFakePathHasNoRealToolchain(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve the test binary: %v", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatalf("resolve the test binary: %v", err)
	}
	for _, name := range fakeNames {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("look up %s: %v", name, err)
		}
		if filepath.Dir(path) != fakeBinDir {
			t.Errorf("%s resolves to %s, want it in %s", name, path, fakeBinDir)
		}
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatalf("resolve %s: %v", path, err)
		}
		if target != executable {
			t.Errorf("%s is %s, want the test binary %s", name, target, executable)
		}
	}
	entries, err := os.ReadDir(fakeBinDir)
	if err != nil {
		t.Fatalf("read %s: %v", fakeBinDir, err)
	}
	if got, want := len(entries), len(fakeNames)+len(gitNames); got != want {
		t.Errorf("%s holds %d entries, want the %d fakes and the %d git programs", fakeBinDir, got, want, len(gitNames))
	}
}
