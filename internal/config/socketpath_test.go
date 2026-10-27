//go:build test

package config

import (
	"errors"
	"maps"
	"strings"
	"testing"
)

// The parts of the socket path below HOME and the longest accepted path, in
// bytes (sun_path minus its terminating NUL), written out so a change to the
// derivation or the limits fails these tests instead of moving their
// expectation.
const (
	darwinSocketTail  = "/Library/Application Support/yt2column/run/server.sock"
	unixSocketTail    = "/.local/state/yt2column/run/server.sock"
	darwinSocketLimit = 103
	linuxSocketLimit  = 107
	// otherUnixSocketLimit is the BSDs' limit, given to every Unix other than
	// Linux and macOS.
	otherUnixSocketLimit = 103
)

// homeMark is part of every HOME the tests build, so an error that carried
// HOME's value would be detected.
const homeMark = "MARK-SOCKET-HOME-Q7"

// homeOfLength returns an absolute HOME of exactly n bytes that contains
// homeMark, so a test can place the derived path at a chosen length.
func homeOfLength(t *testing.T, n int) string {
	t.Helper()
	home := "/" + homeMark + strings.Repeat("h", n-1-len(homeMark))
	if len(home) != n {
		t.Fatalf("homeOfLength(%d) built %d bytes", n, len(home))
	}
	return home
}

// TestSocketPath fixes the socket path rule: the path for each operating
// system, the systems with no rule, the HOME shapes that are rejected, and the
// length limit at its boundary. Every rejection names HOME, wraps the expected
// sentinel, and does not hold HOME's value.
func TestSocketPath(t *testing.T) {
	darwinAtLimit := homeOfLength(t, darwinSocketLimit-len(darwinSocketTail))
	darwinOverLimit := homeOfLength(t, darwinSocketLimit-len(darwinSocketTail)+1)
	unixAtLimit := homeOfLength(t, linuxSocketLimit-len(unixSocketTail))
	unixOverLimit := homeOfLength(t, linuxSocketLimit-len(unixSocketTail)+1)
	otherAtLimit := homeOfLength(t, otherUnixSocketLimit-len(unixSocketTail))
	otherOverLimit := homeOfLength(t, otherUnixSocketLimit-len(unixSocketTail)+1)

	cases := []struct {
		name    string
		goos    string
		env     map[string]string
		want    string
		wantErr error
	}{
		{name: "darwin", goos: "darwin", env: map[string]string{"HOME": "/Users/u"}, want: "/Users/u" + darwinSocketTail},
		{name: "linux", goos: "linux", env: map[string]string{"HOME": "/home/u"}, want: "/home/u" + unixSocketTail},
		{name: "freebsd uses the unix rule", goos: "freebsd", env: map[string]string{"HOME": "/home/u"}, want: "/home/u" + unixSocketTail},
		{name: "windows", goos: "windows", env: map[string]string{"HOME": "/u"}, wantErr: ErrMissing},
		{name: "plan9", goos: "plan9", env: map[string]string{"HOME": "/u"}, wantErr: ErrMissing},
		{name: "js", goos: "js", env: map[string]string{"HOME": "/u"}, wantErr: ErrMissing},
		{name: "wasip1", goos: "wasip1", env: map[string]string{"HOME": "/u"}, wantErr: ErrMissing},
		{name: "darwin home unset", goos: "darwin", env: map[string]string{}, wantErr: ErrMissing},
		{name: "darwin home empty", goos: "darwin", env: map[string]string{"HOME": ""}, wantErr: ErrMissing},
		{name: "darwin home relative", goos: "darwin", env: map[string]string{"HOME": homeMark + "/rel"}, wantErr: ErrMissing},
		{name: "linux home unset", goos: "linux", env: map[string]string{}, wantErr: ErrMissing},
		{name: "linux home empty", goos: "linux", env: map[string]string{"HOME": ""}, wantErr: ErrMissing},
		{name: "linux home relative", goos: "linux", env: map[string]string{"HOME": homeMark + "/rel"}, wantErr: ErrMissing},
		{name: "darwin at the limit", goos: "darwin", env: map[string]string{"HOME": darwinAtLimit}, want: darwinAtLimit + darwinSocketTail},
		{name: "darwin one byte over", goos: "darwin", env: map[string]string{"HOME": darwinOverLimit}, wantErr: ErrInvalid},
		{name: "linux at the limit", goos: "linux", env: map[string]string{"HOME": unixAtLimit}, want: unixAtLimit + unixSocketTail},
		{name: "linux one byte over", goos: "linux", env: map[string]string{"HOME": unixOverLimit}, wantErr: ErrInvalid},
		{name: "freebsd at the limit", goos: "freebsd", env: map[string]string{"HOME": otherAtLimit}, want: otherAtLimit + unixSocketTail},
		{name: "freebsd one byte over", goos: "freebsd", env: map[string]string{"HOME": otherOverLimit}, wantErr: ErrInvalid},
		{
			name: "darwin ignores the XDG variables and TMPDIR",
			goos: "darwin",
			env: map[string]string{
				"HOME": "/Users/u", "XDG_CONFIG_HOME": "/xdg-config", "XDG_STATE_HOME": "/xdg-state",
				"XDG_RUNTIME_DIR": "/run/user/1", "TMPDIR": "/tmp-dir",
			},
			want: "/Users/u" + darwinSocketTail,
		},
		{
			name: "linux ignores the XDG variables and TMPDIR",
			goos: "linux",
			env: map[string]string{
				"HOME": "/home/u", "XDG_CONFIG_HOME": "/xdg-config", "XDG_STATE_HOME": "/xdg-state",
				"XDG_RUNTIME_DIR": "/run/user/1", "TMPDIR": "/tmp-dir",
			},
			want: "/home/u" + unixSocketTail,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := socketPath(tc.goos, envLookup(tc.env))
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("socketPath(%q, ...) error = %v, want nil", tc.goos, err)
				}
				if got != tc.want {
					t.Errorf("socketPath(%q, ...) = %q, want %q", tc.goos, got, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("socketPath(%q, ...) = %q, want an error naming HOME", tc.goos, got)
			}
			if err.Name != "HOME" {
				t.Errorf("socketPath(%q, ...) error names %q, want HOME", tc.goos, err.Name)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("socketPath(%q, ...) error = %v, want it to wrap %v", tc.goos, err, tc.wantErr)
			}
			if strings.Contains(err.Error(), homeMark) {
				t.Errorf("socketPath(%q, ...) error = %q, contains HOME's value", tc.goos, err)
			}
		})
	}
}

// recordingLookup returns a LookupFunc over env that appends every name it is
// asked for to *read, so a test can check which variables a loader reads.
func recordingLookup(env map[string]string, read *[]string) LookupFunc {
	return func(name string) (string, bool) {
		*read = append(*read, name)
		value, ok := env[name]
		return value, ok
	}
}

// requireReadOnlyHome fails the test unless read is non-empty and names HOME
// only.
func requireReadOnlyHome(t *testing.T, fn string, read []string) {
	t.Helper()
	if len(read) == 0 {
		t.Fatalf("%s read no variable, want HOME", fn)
	}
	for _, name := range read {
		if name != "HOME" {
			t.Errorf("%s read %q, want only HOME", fn, name)
		}
	}
}

// withRetiredLLMVars returns a copy of env with every retired LLM variable set,
// which the CLI and the server reject and the host must not read.
func withRetiredLLMVars(env map[string]string) map[string]string {
	out := maps.Clone(env)
	for _, v := range retiredLLMVars {
		out[v.name] = "deepseek"
	}
	return out
}

// TestLoadHost checks that LoadHost reads HOME and nothing else, derives the
// same socket path as LoadServer from an environment that sets every variable
// the server reads (and the retired LLM variables the server would reject),
// and returns a *VarError naming HOME
// and the zero HostConfig when no path can be derived. TestSocketPath fixes
// the derivation itself.
func TestLoadHost(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		env := validServerEnv()
		var read []string
		cfg, err := LoadHost(recordingLookup(withRetiredLLMVars(env), &read))
		if err != nil {
			t.Fatalf("LoadHost() error = %v, want nil", err)
		}
		requireReadOnlyHome(t, "LoadHost", read)
		server, err := LoadServer(envLookup(env))
		if err != nil {
			t.Fatalf("LoadServer() error = %v, want nil", err)
		}
		if cfg.SocketPath() == "" || cfg.SocketPath() != server.SocketPath() {
			t.Errorf("LoadHost().SocketPath() = %q, want LoadServer's %q", cfg.SocketPath(), server.SocketPath())
		}
	})
	cases := []struct {
		name     string
		env      map[string]string
		sentinel error
	}{
		{"home unset", map[string]string{}, ErrMissing},
		{"too long", map[string]string{"HOME": homeOfLength(t, linuxSocketLimit)}, ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var read []string
			cfg, err := LoadHost(recordingLookup(withRetiredLLMVars(tc.env), &read))
			assertVarError(t, err, tc.sentinel, "HOME")
			requireReadOnlyHome(t, "LoadHost", read)
			if cfg != (HostConfig{}) {
				t.Errorf("LoadHost() returned a non-zero HostConfig on error: %+v", cfg)
			}
		})
	}
}

// TestLoadHostHome checks that LoadHostHome reads HOME and nothing else,
// returns it even when the socket path derived from it would exceed every OS
// limit, ignores the retired LLM variables, and rejects an unset, empty, or relative HOME with an ErrMissing that
// names HOME and does not hold its value.
func TestLoadHostHome(t *testing.T) {
	longHome := homeOfLength(t, linuxSocketLimit)
	cases := []struct {
		name    string
		env     map[string]string
		want    string
		wantErr bool
	}{
		{name: "absolute", env: map[string]string{"HOME": "/home/u"}, want: "/home/u"},
		{name: "socket path too long", env: map[string]string{"HOME": longHome}, want: longHome},
		{name: "unset", env: map[string]string{}, wantErr: true},
		{name: "empty", env: map[string]string{"HOME": ""}, wantErr: true},
		{name: "relative", env: map[string]string{"HOME": homeMark + "/rel"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var read []string
			got, err := LoadHostHome(recordingLookup(withRetiredLLMVars(tc.env), &read))
			requireReadOnlyHome(t, "LoadHostHome", read)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("LoadHostHome() error = %v, want nil", err)
				}
				if got != tc.want {
					t.Errorf("LoadHostHome() = %q, want %q", got, tc.want)
				}
				return
			}
			assertVarError(t, err, ErrMissing, "HOME")
			if got != "" {
				t.Errorf("LoadHostHome() = %q on error, want empty", got)
			}
			if strings.Contains(err.Error(), homeMark) {
				t.Errorf("LoadHostHome() error = %q, contains HOME's value", err)
			}
		})
	}
}
