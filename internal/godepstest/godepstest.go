//go:build test

// Package godepstest lists a package's transitive dependencies with go list
// and checks them, so a test can guard which packages a binary links. It runs
// go list without build tags (as make build does), once for each target OS,
// with the toolchain download and the user's go settings turned off. Every
// failure to list is an error, never a skip: a guard that cannot list must not
// pass.
package godepstest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// targetOSes are the operating systems the binaries are built for. A
// dependency an OS-specific file adds shows up under its OS only.
var targetOSes = []string{"darwin", "linux"}

// childGoEnv turns off the toolchain download and the user's go settings in
// the child go command, as make coverage-compare does, so the listing uses
// the local toolchain and this module only.
var childGoEnv = []string{"GOTOOLCHAIN=local", "GOENV=off", "GOFLAGS=", "GOPROXY=off", "GOWORK=off"}

// importPathLine is what one line of go list output must look like. A line
// that does not match means the output is not a package list.
var importPathLine = regexp.MustCompile(`^[A-Za-z0-9_.~+/-]+$`)

var (
	// ErrList reports that go list could not be run or its output was not a
	// package list.
	ErrList = errors.New("go list failed")
	// ErrMissing reports that a package that must be a dependency is not.
	ErrMissing = errors.New("required dependency missing")
	// ErrForbidden reports that a forbidden package is a dependency.
	ErrForbidden = errors.New("forbidden dependency present")
)

// Deps is a package's transitive dependencies on each target OS.
type Deps struct {
	pkg  string
	byOS map[string][]string
}

// childEnv returns the process environment with goos and childGoEnv
// overriding any value it already has.
func childEnv(goos string) []string {
	override := append(slices.Clone(childGoEnv), "GOOS="+goos)
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.ContainsFunc(override, func(o string) bool { return strings.HasPrefix(o, name+"=") })
	})
	return append(env, override...)
}

// runGo runs the go command found on PATH in dir and returns its standard
// output.
func runGo(dir, goos string, args ...string) ([]byte, error) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("%w: go is not on PATH: %w", ErrList, err)
	}
	cmd := exec.Command(goPath, args...) //nolint:gosec // go from PATH with arguments the test fixes
	cmd.Dir = dir
	cmd.Env = childEnv(goos)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: go %s (GOOS=%s): %w: %s", ErrList, strings.Join(args, " "), goos, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Root returns the directory of the module the current directory is in.
func Root() (string, error) {
	out, err := runGo("", targetOSes[0], "list", "-m", "-f", "{{.Dir}}")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" || strings.Contains(root, "\n") {
		return "", fmt.Errorf("%w: go list -m printed no single module directory", ErrList)
	}
	return root, nil
}

// listFormat prints a package's import path and, for a package outside the
// standard library, a tab and its directory.
const listFormat = "{{.ImportPath}}{{if not .Standard}}\t{{.Dir}}{{end}}"

// List lists the transitive dependencies of pkg, a package pattern resolved
// in root (such as ./cmd/yt2column), on each target OS, and reads the sources
// of every dependency outside the standard library (trackSources). Empty
// output, or a line that is not an import path, is an error.
func List(root, pkg string) (Deps, error) {
	deps := Deps{pkg: pkg, byOS: map[string][]string{}}
	dirs := map[string]struct{}{}
	for _, goos := range targetOSes {
		out, err := runGo(root, goos, "list", "-deps", "-f", listFormat, pkg)
		if err != nil {
			return Deps{}, err
		}
		var paths []string
		for line := range strings.Lines(string(out)) {
			path, dir, hasDir := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
			if !importPathLine.MatchString(path) || (hasDir && dir == "") {
				return Deps{}, fmt.Errorf("%w: go list -deps %s (GOOS=%s) printed a line that is not an import path", ErrList, pkg, goos)
			}
			paths = append(paths, path)
			if hasDir {
				dirs[dir] = struct{}{}
			}
		}
		if len(paths) == 0 {
			return Deps{}, fmt.Errorf("%w: go list -deps %s (GOOS=%s) printed nothing", ErrList, pkg, goos)
		}
		deps.byOS[goos] = paths
	}
	if err := trackSources(dirs); err != nil {
		return Deps{}, err
	}
	return deps, nil
}

// trackSources reads each directory and every Go file in it from the test
// process itself. go test reuses a cached result while the test binary and
// the files the test process opened are unchanged; the files the child go
// list reads are not among them, and a file for another target OS is not in
// the binary, so without this a forbidden import added to such a file would
// pass on the cached result. Reading them makes a change to any of them, or
// a file added to or removed from one of the directories, invalidate it.
func trackSources(dirs map[string]struct{}) error {
	for dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("%w: read package directory: %w", ErrList, err)
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			if _, err := os.ReadFile(filepath.Join(dir, entry.Name())); err != nil { //nolint:gosec // a directory go list reported for this module, read only to key the test cache
				return fmt.Errorf("%w: read package source: %w", ErrList, err)
			}
		}
	}
	return nil
}

// PackagePath returns the import path of the one package pkg (a pattern such
// as ./cmd/yt2column-devserver) names, resolved in root. It is how a test names
// a main package, which has no type to take the path from: the lookup fails
// when the package does not exist, so a path that was renamed away cannot turn
// a Forbid into a check of nothing.
func PackagePath(root, pkg string) (string, error) {
	out, err := runGo(root, targetOSes[0], "list", "-f", "{{.ImportPath}}", pkg)
	if err != nil {
		return "", err
	}
	path := strings.TrimSuffix(string(out), "\n")
	if !importPathLine.MatchString(path) {
		return "", fmt.Errorf("%w: go list %s did not print one import path", ErrList, pkg)
	}
	return path, nil
}

// MustPackagePath is PackagePath from the module root, failing tb when it
// cannot name the package.
func MustPackagePath(tb testing.TB, pkg string) string {
	tb.Helper()
	root, err := Root()
	if err != nil {
		tb.Fatal(err)
	}
	path, err := PackagePath(root, pkg)
	if err != nil {
		tb.Fatal(err)
	}
	return path
}

// matches reports whether path is pattern, or, for a pattern ending in
// "/...", the package before it or one under it.
func matches(pattern, path string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "/..."); ok {
		return path == prefix || strings.HasPrefix(path, prefix+"/")
	}
	return path == pattern
}

// Require reports every path that is not a dependency on some target OS. It
// is checked before Forbid, so a listing that missed the packages it should
// hold cannot pass by holding nothing forbidden.
func (d Deps) Require(paths ...string) error {
	var errs []error
	for _, goos := range targetOSes {
		for _, path := range paths {
			if !slices.Contains(d.byOS[goos], path) {
				errs = append(errs, fmt.Errorf("%w: %s does not depend on %s (GOOS=%s)", ErrMissing, d.pkg, path, goos))
			}
		}
	}
	return errors.Join(errs...)
}

// Forbid reports every dependency that matches a pattern on some target OS.
// A pattern is an import path, or an import path followed by "/..." for that
// package and every package under it.
func (d Deps) Forbid(patterns ...string) error {
	var errs []error
	for _, goos := range targetOSes {
		for _, path := range d.byOS[goos] {
			for _, pattern := range patterns {
				if matches(pattern, path) {
					errs = append(errs, fmt.Errorf("%w: %s depends on %s (GOOS=%s)", ErrForbidden, d.pkg, path, goos))
				}
			}
		}
	}
	return errors.Join(errs...)
}

// Check lists pkg's dependencies from the module root and fails tb unless
// every required path is a dependency and no dependency matches a forbidden
// pattern. required must not be empty: it is what proves the listing worked.
func Check(tb testing.TB, pkg string, required, forbidden []string) {
	tb.Helper()
	if len(required) == 0 {
		tb.Fatal("godepstest.Check needs at least one required dependency")
	}
	root, err := Root()
	if err != nil {
		tb.Fatal(err)
	}
	deps, err := List(root, pkg)
	if err != nil {
		tb.Fatal(err)
	}
	if err := deps.Require(required...); err != nil {
		tb.Fatal(err)
	}
	if err := deps.Forbid(forbidden...); err != nil {
		tb.Fatal(err)
	}
}
