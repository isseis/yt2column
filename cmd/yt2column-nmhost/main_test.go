//go:build test

package main

import (
	"bufio"
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
	"regexp"
	"runtime"
	"slices"
	"strconv"
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

// fullEnv is an environment that also holds the server's variables, so that
// a host loading the server's configuration by mistake would succeed and be
// caught only by invoke's read check, which records names whether or not
// they are set.
func fullEnv(home string) map[string]string {
	env := map[string]string{
		"YT2COLUMN_LLM":     "deepseek/deepseek-chat",
		"DEEPSEEK_API_KEY":  "sk-test-key",
		"SLACK_WEBHOOK_URL": "https://hooks.slack.com/services/T000/B000/XXXX",
		"XDG_STATE_HOME":    "/xdg-state",
	}
	if home != "" {
		env["HOME"] = home
	}
	return env
}

// hostEnvs returns fullEnv and fullEnv with every retired LLM variable set as
// well. The CLI and the server reject the retired variables; the host does not
// read the LLM configuration, so it must give the same result with both.
func hostEnvs(home string) []struct {
	name string
	env  map[string]string
} {
	retired := fullEnv(home)
	for _, name := range config.RetiredLLMVarsForTest() {
		retired[name] = "deepseek"
	}
	return []struct {
		name string
		env  map[string]string
	}{
		{"server variables", fullEnv(home)},
		{"retired LLM variables", retired},
	}
}

// TestUsageErrors checks that every argument shape other than the extension's
// origin alone, install <browser>, and uninstall <browser> is a usage error:
// exit code 2, nothing on stdout, stdin unread, no variable read (HOME is
// read only after the browser name is accepted), and a message on stderr. A
// mismatched origin is one of them, so a caller the manifest did not name
// gets no relay. The browser-name grammar itself is TestParseBrowser's.
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
		{"install with two browsers", []string{"install", "chrome", "brave"}},
		{"uninstall with an unknown browser", []string{"uninstall", "safari"}},
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
			if len(inv.read) != 0 {
				t.Errorf("read %q, want no variable", inv.read)
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
// HOME, the relay replies server_unavailable to a message within the limits,
// exits 0, and reads nothing of stdin after the length, and that the log names
// HOME. The length checks still come first: an over-limit length is
// message_too_large, which is also "not sent".
func TestRelayConfigFailure(t *testing.T) {
	overLimit := binary.NativeEndian.AppendUint32(nil, 32<<20+1)
	cases := []struct {
		name  string
		home  string
		stdin *stdinFake
		want  nmhost.HostError
	}{
		{"home unset", "", message([]byte(`{"transcript":"MARK-BODY"}`)), nmhost.HostErrorServerUnavailable},
		{"home unset, length over the limit", "", &stdinFake{data: append(overLimit, "MARK-BODY"...)}, nmhost.HostErrorMessageTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdin := tc.stdin
			inv := invoke(t, []string{nmhost.ExtensionOrigin}, fullEnv(tc.home), stdin, fixedExecutable(testExecutable))
			if inv.code != exitOK {
				t.Errorf("exit code = %d, want %d", inv.code, exitOK)
			}
			want := map[string]any{"v": float64(1), "kind": "error", "error": tc.want.Value()}
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
// response, whether or not the retired LLM variables are set.
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

	for _, variant := range hostEnvs(home) {
		t.Run(variant.name, func(t *testing.T) {
			inv := invoke(t, []string{nmhost.ExtensionOrigin}, variant.env, message([]byte(`{}`)), fixedExecutable(testExecutable))
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
			// The log names the derived socket, so a user can compare it with
			// the server's first log line when the browser passes a different
			// HOME.
			if !strings.Contains(inv.stderr, sock) {
				t.Errorf("stderr = %q, want the derived socket path %s", inv.stderr, sock)
			}
		})
	}
}

// manifestRel returns the Chrome manifest path and Chrome's user data
// directory, relative to home, for the running operating system, and skips on
// one without a layout.
func manifestRel(t *testing.T) (manifest, chromeDir string) {
	t.Helper()
	// The directories are written out as an independent oracle; only the
	// file name comes from the host's constant.
	name := "NativeMessagingHosts/" + nmhost.HostName + ".json"
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
// with the shared-file note on macOS only. Each step gives the same result
// whether or not the retired LLM variables are set.
func TestInstallAndUninstall(t *testing.T) {
	manifest, chromeDir := manifestRel(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, chromeDir), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, filepath.FromSlash(manifest))
	requireNote := func(t *testing.T, stderr string) {
		t.Helper()
		if got, want := strings.Contains(stderr, darwinSharedNote), runtime.GOOS == "darwin"; got != want {
			t.Errorf("stderr = %q; shared-file note present = %v, want %v", stderr, got, want)
		}
	}
	steps := []struct {
		name           string
		args           []string
		manifestExists bool // after the step
	}{
		{"install", []string{"install", "chrome"}, true},
		{"uninstall", []string{"uninstall", "chrome"}, false},
		{"uninstall again", []string{"uninstall", "chrome"}, false},
	}
	for _, variant := range hostEnvs(home) {
		for _, step := range steps {
			name := variant.name + ": " + step.name
			inv := invoke(t, step.args, variant.env, &stdinFake{}, fixedExecutable(testExecutable))
			if inv.code != exitOK {
				t.Fatalf("%s: exit code = %d, want %d; stderr = %q", name, inv.code, exitOK, inv.stderr)
			}
			if len(inv.stdout) != 0 {
				t.Errorf("%s: stdout = %q, want empty", name, inv.stdout)
			}
			if !strings.Contains(inv.stderr, path) {
				t.Errorf("%s: stderr = %q, want the manifest path %s", name, inv.stderr, path)
			}
			requireNote(t, inv.stderr)
			data, err := os.ReadFile(path)
			if !step.manifestExists {
				if !errors.Is(err, os.ErrNotExist) {
					t.Errorf("%s: manifest read error = %v, want it removed", name, err)
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
}

// TestInstallFailures checks the exit codes of an install or uninstall that
// cannot proceed: HOME unusable is a usage error (2); an executable that
// cannot be located, one under the temporary directory, a browser whose user
// data directory is missing, and a file at the manifest path this host did
// not write are failures (1). None writes to stdout.
func TestInstallFailures(t *testing.T) {
	manifest, chromeDir := manifestRel(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, chromeDir), 0o700); err != nil {
		t.Fatal(err)
	}
	// foreignHome holds a file at the manifest path that this host did not
	// write.
	foreignHome := t.TempDir()
	foreign := filepath.Join(foreignHome, filepath.FromSlash(manifest))
	if err := os.MkdirAll(filepath.Dir(foreign), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte(`{"name":"other.host"}`), 0o600); err != nil {
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
		{"uninstall of a foreign manifest", []string{"uninstall", "chrome"}, foreignHome, fixedExecutable(testExecutable), exitFailure},
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

// hostExitCodeRowRE matches a row of the README host exit-code table.
var hostExitCodeRowRE = regexp.MustCompile("^\\| `([0-9]+)` \\|")

// TestReadmeHostExitCodes checks that the README's host exit-code table, under
// "### Installing the native messaging host", lists exactly this command's
// exit codes, so a changed or added code fails until the README follows.
func TestReadmeHostExitCodes(t *testing.T) {
	f, err := os.Open("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	var codes []int
	inSection := false
	for scanner := bufio.NewScanner(f); scanner.Scan(); {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			inSection = line == "### Installing the native messaging host"
			continue
		}
		if m := hostExitCodeRowRE.FindStringSubmatch(line); inSection && m != nil {
			code, _ := strconv.Atoi(m[1])
			codes = append(codes, code)
		}
	}
	slices.Sort(codes)
	if want := []int{exitOK, exitFailure, exitUsage}; !slices.Equal(codes, want) {
		t.Errorf("README host exit-code table lists %v, want %v", codes, want)
	}
}
