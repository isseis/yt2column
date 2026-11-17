//go:build test

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
)

// repoRoot is the repository root relative to this package's directory.
const repoRoot = "../.."

// toolWord is the tool's name, its install target's suffix, and its package
// directory under cmd/.
const toolWord = "numreserve"

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	return doctestutil.ReadDoc(t, filepath.Join(repoRoot, filepath.FromSlash(rel)))
}

// TestInstallTargetInMakefile checks make install-numreserve installs this
// package, the way the user installs the tool from main.
func TestInstallTargetInMakefile(t *testing.T) {
	target := "install-" + toolWord
	lines := strings.Split(readRepoFile(t, "Makefile"), "\n")
	i := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, target+":") })
	if i < 0 || i+1 >= len(lines) || lines[i+1] != "\t$(GOCMD) install ./cmd/"+toolWord {
		t.Errorf("Makefile has no %s target whose recipe is $(GOCMD) install ./cmd/%s", target, toolWord)
	}
}
