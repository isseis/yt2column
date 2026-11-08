//go:build test

package config

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

// The configuration file's rule below HOME, written out so a change to the
// derivation fails these tests instead of moving their expectation.
const (
	darwinServerFileBaseTail = "/Library/Application Support"
	darwinServerFileTail     = "/Library/Application Support/yt2column/config.env"
	unixServerFileTail       = "/.config/yt2column/config.env"
)

// TestServerFileLocation fixes the configuration file's rule: the path, the
// base that must exist, and the directories config init may create for each
// operating system; the systems with no rule; and the HOME shapes that are
// rejected. The rule reads HOME and nothing else, so the XDG variables do not
// move the file, and a rejection names HOME without holding its value.
func TestServerFileLocation(t *testing.T) {
	// Every variable a user directory rule might read, set to an absolute
	// value, so reading any of them could move the path.
	others := map[string]string{
		"XDG_CONFIG_HOME": "/xdg/config",
		"XDG_STATE_HOME":  "/xdg/state",
		"XDG_DATA_HOME":   "/xdg/data",
		"TMPDIR":          "/tmp/elsewhere",
	}
	withHome := func(home string) map[string]string {
		env := maps.Clone(others)
		env["HOME"] = home
		return env
	}

	cases := []struct {
		name     string
		goos     string
		env      map[string]string
		wantPath string
		wantBase string
		wantDirs []string
		wantErr  bool
	}{
		{
			name: "darwin", goos: "darwin", env: withHome("/Users/u"),
			wantPath: "/Users/u" + darwinServerFileTail, wantBase: "/Users/u" + darwinServerFileBaseTail,
			wantDirs: []string{"yt2column"},
		},
		{
			name: "linux", goos: "linux", env: withHome("/home/u"),
			wantPath: "/home/u" + unixServerFileTail, wantBase: "/home/u",
			wantDirs: []string{".config", "yt2column"},
		},
		{
			name: "freebsd uses the unix rule", goos: "freebsd", env: withHome("/home/u"),
			wantPath: "/home/u" + unixServerFileTail, wantBase: "/home/u",
			wantDirs: []string{".config", "yt2column"},
		},
		{
			name: "HOME with a trailing separator is cleaned", goos: "linux", env: withHome("/home/u/"),
			wantPath: "/home/u" + unixServerFileTail, wantBase: "/home/u",
			wantDirs: []string{".config", "yt2column"},
		},
		{name: "windows", goos: "windows", env: withHome("/" + homeMark), wantErr: true},
		{name: "plan9", goos: "plan9", env: withHome("/" + homeMark), wantErr: true},
		{name: "js", goos: "js", env: withHome("/" + homeMark), wantErr: true},
		{name: "wasip1", goos: "wasip1", env: withHome("/" + homeMark), wantErr: true},
		{name: "darwin home unset", goos: "darwin", env: maps.Clone(others), wantErr: true},
		{name: "darwin home empty", goos: "darwin", env: withHome(""), wantErr: true},
		{name: "darwin home relative", goos: "darwin", env: withHome(homeMark + "/rel"), wantErr: true},
		{name: "linux home unset", goos: "linux", env: maps.Clone(others), wantErr: true},
		{name: "linux home empty", goos: "linux", env: withHome(""), wantErr: true},
		{name: "linux home relative", goos: "linux", env: withHome(homeMark + "/rel"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var read []string
			loc, err := serverFileLocationFor(tc.goos, recordingLookup(tc.env, &read))
			for _, name := range read {
				if name != "HOME" {
					t.Errorf("read %s; the rule reads HOME alone", name)
				}
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("got %+v, want an error", loc)
				}
				if err.Name != "HOME" || !errors.Is(err, ErrMissing) || err.Reason != reasonConfigPathAbsent {
					t.Errorf("got %#v, want the HOME rejection with reasonConfigPathAbsent", err)
				}
				if strings.Contains(err.Error(), homeMark) {
					t.Errorf("error %q holds HOME's value", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if loc.path != tc.wantPath || loc.base != tc.wantBase || !slices.Equal(loc.dirs, tc.wantDirs) {
				t.Errorf("got path %q base %q dirs %q, want %q %q %q",
					loc.path, loc.base, loc.dirs, tc.wantPath, tc.wantBase, tc.wantDirs)
			}
		})
	}
}
