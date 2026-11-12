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

// List lists the transitive dependencies of pkg, a package pattern resolved
// in root (such as ./cmd/yt2column), on each target OS. Empty output, or a
// line that is not an import path, is an error.
func List(root, pkg string) (Deps, error) {
	deps := Deps{pkg: pkg, byOS: map[string][]string{}}
	for _, goos := range targetOSes {
		out, err := runGo(root, goos, "list", "-deps", pkg)
		if err != nil {
			return Deps{}, err
		}
		var paths []string
		for line := range strings.Lines(string(out)) {
			line = strings.TrimSuffix(line, "\n")
			if !importPathLine.MatchString(line) {
				return Deps{}, fmt.Errorf("%w: go list -deps %s (GOOS=%s) printed a line that is not an import path", ErrList, pkg, goos)
			}
			paths = append(paths, line)
		}
		if len(paths) == 0 {
			return Deps{}, fmt.Errorf("%w: go list -deps %s (GOOS=%s) printed nothing", ErrList, pkg, goos)
		}
		deps.byOS[goos] = paths
	}
	return deps, nil
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
