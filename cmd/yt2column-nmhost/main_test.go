//go:build test

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/nmhost"
	"github.com/isseis/yt2column/internal/udstest"
)

// testExecutable is an absolute executable path outside the temporary
// directory, standing in for the built binary: the test binary itself runs
// under the temporary directory, which Install rejects.
const testExecutable = "/opt/yt2column/build/yt2column-nmhost"

// stdinFake is standard input that records how many bytes were read, so a
// test can tell whether the message body was read.
type stdinFake struct {
	mu   sync.Mutex
	data []byte
	pos  int
}

// message returns standard input holding body after its native-endian length.
func message(body []byte) *stdinFake {
	data := binary.NativeEndian.AppendUint32(nil, uint32(len(body)))
	return &stdinFake{data: append(data, body...)}
}

func (s *stdinFake) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	n := copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}

func (s *stdinFake) consumed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pos
}

// invocation is one run of the host with a recorded environment.
type invocation struct {
	code   int
	stdout []byte
	stderr string
	read   []string // the variable names looked up, in order
}

// invoke runs the host with args, an environment of env, and stdin.
func invoke(t *testing.T, args []string, env map[string]string, stdin io.Reader, executable func() (string, error)) invocation {
	t.Helper()
	var inv invocation
	lookup := func(name string) (string, bool) {
		inv.read = append(inv.read, name)
		value, ok := env[name]
		return value, ok
	}
	var stdout, stderr bytes.Buffer
	inv.code = run(args, lookup, stdin, &stdout, &stderr, executable)
	inv.stdout = stdout.Bytes()
	inv.stderr = stderr.String()
	for _, name := range inv.read {
		if name != "HOME" {
			t.Errorf("run(%q) read %q, want only HOME", args, name)
		}
	}
	return inv
}

// fixedExecutable returns an executable lookup that reports path.
func fixedExecutable(path string) func() (string, error) {
	return func() (string, error) { return path, nil }
}

// reply decodes stdout as exactly one Native Messaging frame and returns its
// members.
func (inv invocation) reply(t *testing.T) map[string]any {
	t.Helper()
	if len(inv.stdout) < 4 {
		t.Fatalf("stdout = %q, want one frame", inv.stdout)
	}
	length := binary.NativeEndian.Uint32(inv.stdout)
	payload := inv.stdout[4:]
	if int(length) != len(payload) {
		t.Fatalf("stdout holds %d bytes after a length of %d; want exactly one frame", len(payload), length)
	}
	var members map[string]any
	if err := json.Unmarshal(payload, &members); err != nil {
		t.Fatalf("decode the reply: %v", err)
	}
	return members
}

// fullEnv is an environment holding every variable the server reads besides
// HOME, so a host that read one would be caught by invoke.
func fullEnv(home string) map[string]string {
	env := map[string]string{
		"YT2COLUMN_LLM_PROVIDER": "deepseek",
		"YT2COLUMN_LLM_MODEL":    "deepseek-chat",
		"DEEPSEEK_API_KEY":       "sk-test-key",
		"SLACK_WEBHOOK_URL":      "https://hooks.slack.com/services/T000/B000/XXXX",
		"XDG_STATE_HOME":         "/xdg-state",
	}
	if home != "" {
		env["HOME"] = home
	}
	return env
}

// TestUsageErrors checks that every argument shape other than the extension's
// origin alone, install <browser>, and uninstall <browser> is a usage error:
// exit code 2, nothing on stdout, stdin unread, and a message on stderr. A
// mismatched origin is one of them, so a caller the manifest did not name
// gets no relay.
func TestUsageErrors(t *testing.T) {
	origin := nmhost.ExtensionOrigin
	cases := []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"other origin", []string{"chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/"}},
		{"origin without the trailing slash", []string{strings.TrimSuffix(origin, "/")}},
		{"origin with a suffix", []string{origin + "x"}},
		{"origin in upper case", []string{strings.ToUpper(origin)}},
		{"origin and another argument", []string{origin, "--parent-window=0"}},
		{"install without a browser", []string{"install"}},
		{"install with an unknown browser", []string{"install", "firefox"}},
		{"install with a browser in upper case", []string{"install", "Chrome"}},
		{"install with two browsers", []string{"install", "chrome", "brave"}},
		{"uninstall with a padded browser", []string{"uninstall", "brave "}},
		{"unknown command", []string{"register", "chrome"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdin := message([]byte(`{}`))
			inv := invoke(t, tc.args, fullEnv("/home/u"), stdin, fixedExecutable(testExecutable))
			if inv.code != exitUsage {
				t.Errorf("exit code = %d, want %d", inv.code, exitUsage)
			}
			if len(inv.stdout) != 0 {
				t.Errorf("stdout = %q, want empty", inv.stdout)
			}
			if stdin.consumed() != 0 {
				t.Errorf("stdin: %d bytes read, want 0", stdin.consumed())
			}
			if !strings.Contains(inv.stderr, "usage:") {
				t.Errorf("stderr = %q, want the usage", inv.stderr)
			}
		})
	}
}

// TestRelayLengthUnreadable checks that a relay whose stdin ends before the
// length exits 1 with nothing on stdout.
func TestRelayLengthUnreadable(t *testing.T) {
	inv := invoke(t, []string{nmhost.ExtensionOrigin}, fullEnv("/home/u"), &stdinFake{data: []byte{1, 0}}, fixedExecutable(testExecutable))
	if inv.code != exitFailure {
		t.Errorf("exit code = %d, want %d", inv.code, exitFailure)
	}
	if len(inv.stdout) != 0 {
		t.Errorf("stdout = %q, want empty", inv.stdout)
	}
}

// TestRelayConfigFailure checks that when no socket path can be derived from
// HOME, the relay replies server_unavailable, exits 0, and reads nothing of
// stdin after the length, and that the log names HOME.
func TestRelayConfigFailure(t *testing.T) {
	for name, home := range map[string]string{"home unset": "", "home relative": "home/u"} {
		t.Run(name, func(t *testing.T) {
			stdin := message([]byte(`{"transcript":"MARK-BODY"}`))
			inv := invoke(t, []string{nmhost.ExtensionOrigin}, fullEnv(home), stdin, fixedExecutable(testExecutable))
			if inv.code != exitOK {
				t.Errorf("exit code = %d, want %d", inv.code, exitOK)
			}
			want := map[string]any{"v": float64(1), "kind": "error", "error": nmhost.HostErrorServerUnavailable.Value()}
			if got := inv.reply(t); !maps.Equal(got, want) {
				t.Errorf("reply = %v, want %v", got, want)
			}
			if stdin.consumed() != 4 {
				t.Errorf("stdin: %d bytes read, want only the 4-byte length", stdin.consumed())
			}
			if !strings.Contains(inv.stderr, "HOME") {
				t.Errorf("stderr = %q, want the cause naming HOME", inv.stderr)
			}
		})
	}
}

// TestRelayUsesDerivedSocket checks that the relay connects to the socket
// path derived from HOME, reading no other variable, and relays the server's
// response.
func TestRelayUsesDerivedSocket(t *testing.T) {
	home := udstest.Dir(t)
	sock := home + config.SocketPathTailForTest(runtime.GOOS)
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		t.Fatal(err)
	}
	listener := udstest.Listen(t, sock)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"requestId":"r1"}`))
	})}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { _ = srv.Close() })

	inv := invoke(t, []string{nmhost.ExtensionOrigin}, fullEnv(home), message([]byte(`{}`)), fixedExecutable(testExecutable))
	if inv.code != exitOK {
		t.Fatalf("exit code = %d, want %d; stderr = %q", inv.code, exitOK, inv.stderr)
	}
	want := map[string]any{
		"v": float64(1), "kind": "response", "status": float64(http.StatusAccepted),
		"body": base64.StdEncoding.EncodeToString([]byte(`{"requestId":"r1"}`)),
	}
	if got := inv.reply(t); !maps.Equal(got, want) {
		t.Errorf("reply = %v, want %v", got, want)
	}
}

// manifestRel returns the Chrome manifest path and Chrome's user data
// directory, relative to home, for the running operating system, and skips on
// one without a layout.
func manifestRel(t *testing.T) (manifest, chromeDir string) {
	t.Helper()
	const name = "NativeMessagingHosts/io.github.isseis.yt2column.json"
	switch runtime.GOOS {
	case "darwin":
		chromeDir = "Library/Application Support/Google/Chrome"
		return chromeDir + "/" + name, chromeDir
	case "linux":
		chromeDir = ".config/google-chrome"
		return chromeDir + "/" + name, chromeDir
	default:
		t.Skipf("no manifest layout under test for %s", runtime.GOOS)
		return "", ""
	}
}

// TestInstallAndUninstall checks that install writes the Chrome manifest
// pointing at the executable the lookup reported, and that install and
// uninstall exit 0, leave stdout empty, and write the manifest path to stderr,
// with the shared-file note on macOS only.
func TestInstallAndUninstall(t *testing.T) {
	manifest, chromeDir := manifestRel(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, chromeDir), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, filepath.FromSlash(manifest))
	requireNote := func(t *testing.T, stderr string) {
		t.Helper()
		if got, want := strings.Contains(stderr, "Chrome and Brave share this manifest file"), runtime.GOOS == "darwin"; got != want {
			t.Errorf("stderr = %q; shared-file note present = %v, want %v", stderr, got, want)
		}
	}
	steps := []struct {
		name string
		args []string
	}{
		{"install", []string{"install", "chrome"}},
		{"uninstall", []string{"uninstall", "chrome"}},
		{"uninstall again", []string{"uninstall", "chrome"}},
	}
	for _, step := range steps {
		inv := invoke(t, step.args, fullEnv(home), &stdinFake{}, fixedExecutable(testExecutable))
		if inv.code != exitOK {
			t.Fatalf("%s: exit code = %d, want %d; stderr = %q", step.name, inv.code, exitOK, inv.stderr)
		}
		if len(inv.stdout) != 0 {
			t.Errorf("%s: stdout = %q, want empty", step.name, inv.stdout)
		}
		if !strings.Contains(inv.stderr, path) {
			t.Errorf("%s: stderr = %q, want the manifest path %s", step.name, inv.stderr, path)
		}
		requireNote(t, inv.stderr)
		data, err := os.ReadFile(path)
		if step.name != "install" {
			if !errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s: manifest read error = %v, want it removed", step.name, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("install: read the manifest: %v", err)
		}
		var got struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("install: decode the manifest: %v", err)
		}
		if got.Path != testExecutable {
			t.Errorf("install: manifest path = %q, want %q", got.Path, testExecutable)
		}
	}
}

// TestInstallFailures checks the exit codes of an install that cannot
// proceed: HOME unusable is a usage error (2); an executable that cannot be
// located, one under the temporary directory, and a browser whose user data
// directory is missing are failures (1). None writes to stdout.
func TestInstallFailures(t *testing.T) {
	_, chromeDir := manifestRel(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, chromeDir), 0o700); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		args       []string
		home       string
		executable func() (string, error)
		want       int
	}{
		{"home unset", []string{"install", "chrome"}, "", fixedExecutable(testExecutable), exitUsage},
		{"home relative", []string{"uninstall", "chrome"}, "home/u", fixedExecutable(testExecutable), exitUsage},
		{
			"executable unknown",
			[]string{"install", "chrome"},
			home,
			func() (string, error) { return "", errors.New("no executable") }, exitFailure,
		},
		{"executable under the temporary directory", []string{"install", "chrome"}, home, fixedExecutable(filepath.Join(os.TempDir(), "yt2column-nmhost")), exitFailure},
		{"browser not found", []string{"install", "brave"}, home, fixedExecutable(testExecutable), exitFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := invoke(t, tc.args, fullEnv(tc.home), &stdinFake{}, tc.executable)
			if inv.code != tc.want {
				t.Errorf("exit code = %d, want %d; stderr = %q", inv.code, tc.want, inv.stderr)
			}
			if len(inv.stdout) != 0 {
				t.Errorf("stdout = %q, want empty", inv.stdout)
			}
		})
	}
}
