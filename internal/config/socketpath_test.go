//go:build test

package config

import (
	"errors"
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
	unixSocketLimit   = 107
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
	unixAtLimit := homeOfLength(t, unixSocketLimit-len(unixSocketTail))
	unixOverLimit := homeOfLength(t, unixSocketLimit-len(unixSocketTail)+1)

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
