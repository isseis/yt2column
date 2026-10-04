//go:build test

package writer

import (
	"os"
	"path/filepath"
	"testing"
)

// overrideTargets lists, for each template, how to build Options that
// override only that template with the file at path.
var overrideTargets = []struct {
	name    string
	options func(path string) Options
}{
	{systemTemplateName, func(path string) Options { return Options{SystemTemplatePath: path} }},
	{userTemplateName, func(path string) Options { return Options{UserTemplatePath: path} }},
}

// writeOverrideFile writes content to a file in a fresh test temp directory
// and returns its path.
func writeOverrideFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "override.tmpl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// requireNonRoot fails the test when it runs as root, where permission
// failures cannot be produced. Such a run is an unsupported environment, so it
// fails instead of being skipped silently.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Fatal("unsupported test environment: permission-failure tests require a non-root user")
	}
}
