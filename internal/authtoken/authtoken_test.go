//go:build test

package authtoken

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
)

// TestLoadOrCreateCreates checks the creation path: a missing file is filled
// with 64 lowercase hexadecimal characters, the token carries that value, and
// the file and its directory get the restrictive permissions.
func TestLoadOrCreateCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "yt2column", tokenFileName)

	token, created, err := LoadOrCreate(path, rand.Reader)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if !created {
		t.Errorf("created = false, want true when the token file did not exist")
	}
	if token.IsZero() {
		t.Errorf("IsZero() = true, want false after creation")
	}
	value, err := token.Value().Reveal()
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if len(value) != tokenLength {
		t.Errorf("token length = %d, want %d", len(value), tokenLength)
	}
	if !tokenPattern.MatchString(value) {
		t.Errorf("token %q does not match the accepted form", value)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("token file mode = %o, want %o", got, 0o600)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(content) != value {
		t.Errorf("file content = %q, want the token value", content)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat %s: %v", filepath.Dir(path), err)
	}
	if got := dirInfo.Mode().Perm(); got != dirMode {
		t.Errorf("token directory mode = %o, want %o", got, dirMode)
	}
}

// TestLoadOrCreateReadsAgain checks that a second call reads the existing token
// without regenerating or modifying it.
func TestLoadOrCreateReadsAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), tokenFileName)
	first, created, err := LoadOrCreate(path, rand.Reader)
	if err != nil {
		t.Fatalf("first LoadOrCreate: %v", err)
	}
	if !created {
		t.Fatalf("first LoadOrCreate created = false, want true")
	}
	firstValue, err := first.Value().Reveal()
	if err != nil {
		t.Fatalf("first Reveal: %v", err)
	}
	before := takeSnapshot(t, path)

	second, created, err := LoadOrCreate(path, rand.Reader)
	if err != nil {
		t.Fatalf("second LoadOrCreate: %v", err)
	}
	if created {
		t.Errorf("second LoadOrCreate created = true, want false")
	}
	secondValue, err := second.Value().Reveal()
	if err != nil {
		t.Fatalf("second Reveal: %v", err)
	}
	if firstValue != secondValue {
		t.Errorf("token changed between calls: %q then %q", firstValue, secondValue)
	}
	if !second.Matches(firstValue) {
		t.Errorf("Matches(first token) = false, want true")
	}
	before.assertUnchanged(t, path)
}

// TestLoadOrCreateRejectsInvalidFile checks that every unacceptable file is
// rejected with ErrInvalidTokenFile and the matching reason, and is left as it
// was.
func TestLoadOrCreateRejectsInvalidFile(t *testing.T) {
	valid := bytes.Repeat([]byte("a"), tokenLength)
	cases := []struct {
		name   string
		setup  func(t *testing.T, path string)
		reason string
	}{
		{
			name: "symbolic_link",
			setup: func(t *testing.T, path string) {
				t.Helper()
				target := path + ".target"
				writeTokenFile(t, target, 0o600, valid)
				if err := os.Symlink(target, path); err != nil {
					t.Fatalf("symlink: %v", err)
				}
			},
			reason: reasonSymlink,
		},
		{
			name: "dangling_symbolic_link",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Symlink(path+".missing", path); err != nil {
					t.Fatalf("symlink: %v", err)
				}
			},
			reason: reasonSymlink,
		},
		{
			name: "directory",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			},
			reason: reasonNotRegular,
		},
		{
			// A named pipe would block an open that does not pass O_NONBLOCK;
			// this case fails the test run by timing out if it does.
			name: "named_pipe",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					t.Fatalf("mkfifo: %v", err)
				}
			},
			reason: reasonNotRegular,
		},
		{
			name:   "permissions_0644",
			setup:  func(t *testing.T, path string) { writeTokenFile(t, path, 0o644, valid) },
			reason: reasonPermissions,
		},
		{
			name:   "permissions_0640",
			setup:  func(t *testing.T, path string) { writeTokenFile(t, path, 0o640, valid) },
			reason: reasonPermissions,
		},
		{
			name:   "permissions_0604",
			setup:  func(t *testing.T, path string) { writeTokenFile(t, path, 0o604, valid) },
			reason: reasonPermissions,
		},
		{
			name: "too_short",
			setup: func(t *testing.T, path string) {
				writeTokenFile(t, path, 0o600, bytes.Repeat([]byte("a"), tokenLength-1))
			},
			reason: reasonContent,
		},
		{
			name: "too_long",
			setup: func(t *testing.T, path string) {
				writeTokenFile(t, path, 0o600, bytes.Repeat([]byte("a"), tokenLength+1))
			},
			reason: reasonContent,
		},
		{
			name: "uppercase",
			setup: func(t *testing.T, path string) {
				writeTokenFile(t, path, 0o600, bytes.Repeat([]byte("A"), tokenLength))
			},
			reason: reasonContent,
		},
		{
			name: "not_hexadecimal",
			setup: func(t *testing.T, path string) {
				writeTokenFile(t, path, 0o600, bytes.Repeat([]byte("g"), tokenLength))
			},
			reason: reasonContent,
		},
		{
			name: "trailing_newline",
			setup: func(t *testing.T, path string) {
				t.Helper()
				writeTokenFile(t, path, 0o600, append(bytes.Clone(valid), '\n'))
			},
			reason: reasonContent,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tokenFileName)
			tc.setup(t, path)
			before := takeSnapshot(t, path)

			_, created, err := LoadOrCreate(path, rand.Reader)
			if created {
				t.Errorf("created = true, want false for a rejected file")
			}
			if !errors.Is(err, ErrInvalidTokenFile) {
				t.Fatalf("LoadOrCreate error = %v, want ErrInvalidTokenFile", err)
			}
			var fileErr *FileError
			if !errors.As(err, &fileErr) {
				t.Fatalf("LoadOrCreate error = %v, want *FileError", err)
			}
			if fileErr.Path != path {
				t.Errorf("FileError.Path = %q, want %q", fileErr.Path, path)
			}
			if fileErr.Reason != tc.reason {
				t.Errorf("FileError.Reason = %q, want %q", fileErr.Reason, tc.reason)
			}
			before.assertUnchanged(t, path)
		})
	}
}

// TestLoadOrCreateRejectsWrongOwner checks the owner check. Changing a file's
// owner needs root, so the test runs only where it can.
func TestLoadOrCreateRejectsWrongOwner(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing a file's owner requires root")
	}
	path := filepath.Join(t.TempDir(), tokenFileName)
	writeTokenFile(t, path, 0o600, bytes.Repeat([]byte("a"), tokenLength))
	if err := os.Chown(path, 1, 1); err != nil {
		t.Fatalf("chown: %v", err)
	}

	_, _, err := LoadOrCreate(path, rand.Reader)
	if !errors.Is(err, ErrInvalidTokenFile) {
		t.Fatalf("LoadOrCreate error = %v, want ErrInvalidTokenFile", err)
	}
	var fileErr *FileError
	if !errors.As(err, &fileErr) {
		t.Fatalf("LoadOrCreate error = %v, want *FileError", err)
	}
	if fileErr.Reason != reasonOwner {
		t.Errorf("FileError.Reason = %q, want %q", fileErr.Reason, reasonOwner)
	}
}

// TestCreateDoesNotOverwriteExisting checks the link(2) race: when the final
// name already holds a token, creation returns that token and leaves the file
// alone.
func TestCreateDoesNotOverwriteExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), tokenFileName)
	existing := bytes.Repeat([]byte("b"), tokenLength)
	writeTokenFile(t, path, 0o600, existing)

	// A source that would produce a different token, so a passing test cannot
	// come from the new value matching the old one.
	other := bytes.Repeat([]byte{0x11}, tokenBytes)
	token, created, err := create(path, bytes.NewReader(other))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created {
		t.Errorf("create reported created = true when the name already existed, want false")
	}
	value, err := token.Value().Reveal()
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if value != string(existing) {
		t.Errorf("create returned %q, want the existing file's value %q", value, existing)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(content, existing) {
		t.Errorf("the existing token file was overwritten")
	}
}

// TestLoadOrCreateRandomError checks that a failure to draw the token bytes is
// reported and creates no file.
func TestLoadOrCreateRandomError(t *testing.T) {
	path := filepath.Join(t.TempDir(), tokenFileName)
	_, created, err := LoadOrCreate(path, iotest.ErrReader(errors.New("no randomness")))
	if err == nil {
		t.Fatalf("LoadOrCreate error = nil, want a failure")
	}
	if created {
		t.Errorf("created = true, want false when generation failed")
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("a token file exists after a failed creation: %v", statErr)
	}
}

// TestMatches checks the constant-time comparison, including the cases that
// must never match: an empty value and the zero Token.
func TestMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), tokenFileName)
	token, _, err := LoadOrCreate(path, rand.Reader)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	value, err := token.Value().Reveal()
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	different := []byte(value)
	if different[0] == '0' {
		different[0] = '1'
	} else {
		different[0] = '0'
	}

	tests := []struct {
		name      string
		presented string
		want      bool
	}{
		{"exact", value, true},
		{"empty", "", false},
		{"shorter", value[:len(value)-1], false},
		{"longer", value + "0", false},
		{"different", string(different), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := token.Matches(tc.presented); got != tc.want {
				t.Errorf("Matches(%q) = %v, want %v", tc.presented, got, tc.want)
			}
		})
	}

	var zero Token
	if !zero.IsZero() {
		t.Errorf("zero Token IsZero() = false, want true")
	}
	if zero.Matches(value) {
		t.Errorf("zero Token matched a non-empty value")
	}
	if zero.Matches("") {
		t.Errorf("zero Token matched an empty value")
	}
	if token.IsZero() {
		t.Errorf("a created Token IsZero() = true, want false")
	}
	if _, err := zero.Value().Reveal(); err == nil {
		t.Errorf("zero Token Value() revealed a value, want the zero secret")
	}
}

// TestFileErrorMessageOmitsContent checks that a rejection names the path and a
// fixed reason and never the file's content.
func TestFileErrorMessageOmitsContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), tokenFileName)
	content := "S3cret-not-a-token"
	writeTokenFile(t, path, 0o600, []byte(content))

	_, _, err := LoadOrCreate(path, rand.Reader)
	var fileErr *FileError
	if !errors.As(err, &fileErr) {
		t.Fatalf("LoadOrCreate error = %v, want *FileError", err)
	}
	message := err.Error()
	for _, want := range []string{path, fileErr.Reason} {
		if !strings.Contains(message, want) {
			t.Errorf("FileError message %q does not contain %q", message, want)
		}
	}
	if strings.Contains(message, content) {
		t.Errorf("FileError message %q contains the file content", message)
	}
}

// writeTokenFile writes data to path with exactly perm, restoring the mode
// after the write so the umask cannot change it.
func writeTokenFile(t *testing.T, path string, perm os.FileMode, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, perm); err != nil { //nolint:gosec // the path is inside the test's temporary directory
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

// snapshot records a path's mode and, for a regular file, its content, so a
// test can check that a rejected file was left untouched.
type snapshot struct {
	mode os.FileMode
	data []byte
}

// takeSnapshot records the state of path.
func takeSnapshot(t *testing.T, path string) snapshot {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	s := snapshot{mode: info.Mode()}
	if info.Mode().IsRegular() {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		s.data = data
	}
	return s
}

// assertUnchanged fails when path's mode or content differs from s.
func (s snapshot) assertUnchanged(t *testing.T, path string) {
	t.Helper()
	after := takeSnapshot(t, path)
	if after.mode != s.mode {
		t.Errorf("%s mode changed from %v to %v; a rejected file must not be modified", path, s.mode, after.mode)
	}
	if !bytes.Equal(after.data, s.data) {
		t.Errorf("%s content changed; a rejected file must not be modified", path)
	}
}
