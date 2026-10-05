//go:build test

package transcript

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Paths to the committed testdata fixtures, relative to this package directory.
const (
	testdataRealSubtitles        = "../../testdata/2tcCWM-sRBw.ja.json3"
	testdataRealInfo             = "../../testdata/2tcCWM-sRBw.info.json"
	testdataInvalidUTF8Subtitles = "../../testdata/invalid_utf8.json3"
	testdataSurrogateSubtitles   = "../../testdata/unpaired_surrogate.json3"
	testdataInvalidUTF8Info      = "../../testdata/invalid_utf8.info.json"
	testdataSurrogateInfo        = "../../testdata/unpaired_surrogate.info.json"
	testdataRealVideoID          = "2tcCWM-sRBw"
)

// fakeCommandCall records one fakeCommandExecutor.Run call.
type fakeCommandCall struct {
	ctx    context.Context
	name   string
	args   []string
	env    []string
	files  []*os.File
	stderr io.Writer
}

// fakeCommandExecutor is a commandExecutor for tests. behavior may inspect the
// call and write files or standard error output; a nil behavior succeeds
// without output. A timeout or a cancellation is simulated by returning a
// non-context failure while the call's context is done, matching the real
// executor, which returns raw results.
type fakeCommandExecutor struct {
	calls    []fakeCommandCall
	behavior func(call fakeCommandCall) error
}

// Run implements commandExecutor.
func (f *fakeCommandExecutor) Run(ctx context.Context, name string, args, env []string, inherited []*os.File, stderr io.Writer) error {
	call := fakeCommandCall{ctx: ctx, name: name, args: args, env: env, files: inherited, stderr: stderr}
	f.calls = append(f.calls, call)
	if f.behavior == nil {
		return nil
	}
	return f.behavior(call)
}

// callCount returns how many times Run was called.
func (f *fakeCommandExecutor) callCount() int {
	return len(f.calls)
}

// lastCall returns the most recent recorded call.
func (f *fakeCommandExecutor) lastCall(t *testing.T) fakeCommandCall {
	t.Helper()
	if len(f.calls) == 0 {
		t.Fatal("no command was run")
	}
	return f.calls[len(f.calls)-1]
}

// newTestSource builds a YtDlpSource whose executor is fake. YtDlpPath points
// to a path that does not exist, so forgetting to install the fake cannot run
// the real yt-dlp. modify may adjust the options before construction.
func newTestSource(t *testing.T, cacheDir string, fake *fakeCommandExecutor, modify func(*Options)) *YtDlpSource {
	t.Helper()
	options := Options{
		CacheDir:  cacheDir,
		YtDlpPath: filepath.Join(cacheDir, "missing-yt-dlp"),
		Timeout:   time.Minute,
	}
	if modify != nil {
		modify(&options)
	}
	source, err := NewYtDlpSource(options)
	if err != nil {
		t.Fatalf("NewYtDlpSource(%+v) error = %v", options, err)
	}
	source.exec = fake
	return source
}

// generation is the content of one cache slot in a test.
type generation struct {
	subtitles string
	info      string
}

// placeSlot writes gen into the named slot of the video's cache.
func placeSlot(t *testing.T, dir, id, slot string, gen generation) {
	t.Helper()
	slotDir := slotDirPath(dir, id, slot)
	if err := os.MkdirAll(slotDir, 0o700); err != nil {
		t.Fatalf("create slot %s: %v", slotDir, err)
	}
	writeSlotFiles(t, slotDir, id, gen.subtitles, gen.info)
}

// placePointer writes the pointer of the video.
func placePointer(t *testing.T, dir, id, content string) {
	t.Helper()
	writeTestFile(t, pointerPath(dir, id), content)
}

// placePointerTmp writes the pointer temporary file of the video.
func placePointerTmp(t *testing.T, dir, id, content string) {
	t.Helper()
	writeTestFile(t, pointerTmpPath(dir, id), content)
}

// writeSlotFiles writes the fixed subtitle and info files into an existing slot
// directory.
func writeSlotFiles(t *testing.T, slotDir, id, subtitles, info string) {
	t.Helper()
	writeTestFile(t, subtitlesPath(slotDir, id), subtitles)
	writeTestFile(t, infoPath(slotDir, id), info)
}

// slotDirFromArgs returns the -P value of recorded yt-dlp arguments.
func slotDirFromArgs(t *testing.T, args []string) string {
	t.Helper()
	for i, arg := range args {
		if arg == "-P" && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("recorded args have no -P value: %q", args)
	return ""
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeSparseFile creates a file of size bytes without writing its content.
func writeSparseFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	file, err := os.Create(path) //nolint:gosec // the path names a file inside a test temp directory
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	if err := file.Truncate(size); err != nil {
		_ = file.Close()
		t.Fatalf("truncate %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

// placeRealCache places the committed real fixtures as a valid cache (slot a,
// pointer a) and returns the fixture bytes.
func placeRealCache(t *testing.T, dir, id string) (subtitles, info []byte) {
	t.Helper()
	subtitles = readTestdataFile(t, testdataRealSubtitles)
	info = readTestdataFile(t, testdataRealInfo)
	placeSlot(t, dir, id, slotNameA, generation{subtitles: string(subtitles), info: string(info)})
	placePointer(t, dir, id, slotNameA)
	return subtitles, info
}

// The interrupted cache states of the architecture document, section 6.3. Each
// helper writes the fixed names that the state leaves behind and uses distinct
// generations where the test must detect a mixed transcript.

// placeStateS1 builds S1: pointer a, slot b empty or partially written.
func placeStateS1(t *testing.T, dir, id string, old, writeProgress generation) {
	t.Helper()
	placeSlot(t, dir, id, slotNameA, old)
	placePointer(t, dir, id, slotNameA)
	placeSlot(t, dir, id, slotNameB, writeProgress)
}

// placeStateS2 builds S2: pointer a, slot b complete, pointer temporary file.
func placeStateS2(t *testing.T, dir, id string, old, complete generation) {
	t.Helper()
	placeSlot(t, dir, id, slotNameA, old)
	placePointer(t, dir, id, slotNameA)
	placeSlot(t, dir, id, slotNameB, complete)
	placePointerTmp(t, dir, id, slotNameB)
}

// placeStateS3 builds S3: pointer b, slot a stale, slot b complete.
func placeStateS3(t *testing.T, dir, id string, stale, complete generation) {
	t.Helper()
	placeSlot(t, dir, id, slotNameA, stale)
	placeSlot(t, dir, id, slotNameB, complete)
	placePointer(t, dir, id, slotNameB)
}

// placeStateS4 builds S4: no pointer, slot a arbitrary, pointer temporary file.
func placeStateS4(t *testing.T, dir, id string, preview generation) {
	t.Helper()
	placeSlot(t, dir, id, slotNameA, preview)
	placePointerTmp(t, dir, id, slotNameA)
}

// placeStateS5 builds S5: a regular pointer with invalid content and a slot.
func placeStateS5(t *testing.T, dir, id string, content generation) {
	t.Helper()
	placeSlot(t, dir, id, slotNameA, content)
	placePointer(t, dir, id, "x")
}

// placeStateS6 builds S6: no pointer, slot a remaining, pointer temporary file.
func placeStateS6(t *testing.T, dir, id string, remaining generation) {
	t.Helper()
	placeSlot(t, dir, id, slotNameA, remaining)
	placePointerTmp(t, dir, id, slotNameB)
}

// readTestdataFile returns the bytes of a committed testdata fixture.
func readTestdataFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the path names a committed testdata fixture
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// sampleAllowlistEnv returns every allowlisted variable set to a distinct
// value, so a dropped, renamed, or swapped entry fails an exact comparison.
func sampleAllowlistEnv() map[string]string {
	return map[string]string{ //nolint:gosec // the credentials are dummy values in a test fixture
		"PATH":             "/usr/bin:/bin",
		"HOME":             "/home/example",
		"TMPDIR":           "/tmp/example",
		"XDG_CONFIG_HOME":  "/home/example/.config",
		"XDG_CACHE_HOME":   "/home/example/.cache",
		envHTTPProxy:       "http://proxy.example:3128",
		envHTTPSProxy:      "http://user:pass@proxy.example:8080",
		envNOProxy:         "localhost,127.0.0.1",
		envALLProxy:        "socks5://proxy.example:1080",
		envHTTPProxyLower:  "http://lower-proxy.example:3128",
		envHTTPSProxyLower: "http://lower-proxy.example:8443",
		envNOProxyLower:    "",
		envALLProxyLower:   "socks5://lower-proxy.example:1080",
		"LANG":             "ja_JP.UTF-8",
		"LC_ALL":           "C.UTF-8",
		"LC_CTYPE":         "ja_JP.UTF-8",
		"SSL_CERT_FILE":    "/etc/ssl/cert.pem",
		"SSL_CERT_DIR":     "/etc/ssl/certs",
	}
}

// infoOracle decodes the consumed metadata fields with encoding/json,
// independent of the parser under test.
func infoOracle(t *testing.T, data []byte) videoInfo {
	t.Helper()
	var document struct {
		Title       string `json:"title"`
		Channel     string `json:"channel"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("info oracle decode: %v", err)
	}
	return videoInfo{Title: document.Title, ChannelName: document.Channel, Description: document.Description}
}

// testVideoID returns id after checking it is a valid video ID, so a typo in a
// test constant cannot silently turn a fixed name into a non-candidate.
func testVideoID(t *testing.T, id string) string {
	t.Helper()
	if !videoIDPattern.MatchString(id) {
		t.Fatalf("test constant %q is not a valid video ID", id)
	}
	return id
}

// unsetenvForTest removes name for the test and restores its previous state at
// cleanup, so a test can build a parent environment without an allowlisted
// variable even when the test process has it set.
func unsetenvForTest(t *testing.T, name string) {
	t.Helper()
	original, ok := os.LookupEnv(name)
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(name, original)
			return
		}
		_ = os.Unsetenv(name)
	})
	_ = os.Unsetenv(name)
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

// chmodForTest changes path's mode and restores the original mode at cleanup,
// so the test temp directory can be removed afterwards.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	original := info.Mode().Perm()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, original)
	})
}
