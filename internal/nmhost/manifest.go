package nmhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/isseis/yt2column/internal/strictjson"
)

// HostName is the Native Messaging host name: the manifest's "name", the
// manifest file's base name, and the name the extension sends to.
const HostName = "io.github.isseis.yt2column"

// ExtensionOrigin is the only origin in the manifest's "allowed_origins" and
// the only first argument the relay accepts.
const ExtensionOrigin = "chrome-extension://clfmbbcdpnjcefbdihdoahomaifbabkk/"

const (
	// manifestDescription is the manifest's "description". Recognizing this
	// host's own manifest does not compare it, so changing it does not turn an
	// installed manifest into a foreign one.
	manifestDescription = "yt2column: relays the extension's request to the local server"
	// manifestType is the manifest's "type": the browser talks to the host
	// over its standard input and output.
	manifestType = "stdio"
	// maxManifestBytes caps the existing manifest read to recognize it. The
	// manifest this host writes is a few hundred bytes.
	maxManifestBytes = 64 << 10
	// hostsDirMode is the mode of the NativeMessagingHosts directory and its
	// parents when Install creates them.
	hostsDirMode = 0o700
	// hostsDirName is the browsers' per-user manifest directory name.
	hostsDirName = "NativeMessagingHosts"
)

// manifestMembers are the members of the manifest this host writes, exactly.
var manifestMembers = []string{"name", "description", "path", "type", "allowed_origins"}

// Browser is the browser whose manifest is installed. The zero value is
// BrowserUnset, which every operation rejects.
type Browser int

// The browsers whose manifest location is known.
const (
	BrowserUnset Browser = iota
	BrowserChrome
	BrowserBrave
)

// Errors of the manifest operations. ErrBrowserNotFound, ErrForeignManifest,
// and ErrUnstableExecutable are returned wrapped with %w together with the
// path they concern.
var (
	// ErrUnknownBrowser reports a browser name ParseBrowser does not accept,
	// or BrowserUnset (a usage error).
	ErrUnknownBrowser = errors.New("nmhost: unknown browser; use chrome or brave")
	// ErrBrowserNotFound reports that the browser's user data directory does
	// not exist, so the browser is not used under this home.
	ErrBrowserNotFound = errors.New("nmhost: the browser's user data directory does not exist")
	// ErrForeignManifest reports a file at the manifest path that is not a
	// manifest this host wrote. It is neither replaced nor removed.
	ErrForeignManifest = errors.New("nmhost: the file at the manifest path was not written by this host; check it by hand")
	// ErrUnstableExecutable reports an executable under the temporary
	// directory, such as the binary go run builds and removes.
	ErrUnstableExecutable = errors.New("nmhost: the executable is under the temporary directory; build it and install from the built binary")
)

// errNotAbsolute reports a relative home or executable path. The manifest's
// "path" must be absolute, and a relative home would resolve against the
// current directory.
var errNotAbsolute = errors.New("nmhost: the path is not absolute")

// errUnsupportedOS reports an operating system with no known manifest
// location.
var errUnsupportedOS = errors.New("nmhost: no Native Messaging manifest location is known for this operating system")

// ParseBrowser accepts exactly "chrome" or "brave".
func ParseBrowser(name string) (Browser, error) {
	switch name {
	case "chrome":
		return BrowserChrome, nil
	case "brave":
		return BrowserBrave, nil
	default:
		return BrowserUnset, ErrUnknownBrowser
	}
}

// manifestLocation is where a browser's manifest goes under one home.
type manifestLocation struct {
	// browserDir is the browser's user data directory. Its existence is the
	// test that the browser is used; Install never creates it.
	browserDir string
	// hostsDir is the directory holding the manifest file.
	hostsDir string
}

// manifestPath is the manifest file's path.
func (l manifestLocation) manifestPath() string {
	return filepath.Join(l.hostsDir, HostName+".json")
}

// locate returns the manifest location of browser under home for goos.
// Contract: hostsDir is the NativeMessagingHosts directory directly under
// browserDir, except for Brave on darwin, which reads Chrome's directory, so
// there browserDir is Brave's and hostsDir is Chrome's.
func locate(goos, home string, browser Browser) (manifestLocation, error) {
	if !filepath.IsAbs(home) {
		return manifestLocation{}, fmt.Errorf("%w: home %s", errNotAbsolute, home)
	}
	var chrome, brave string
	switch goos {
	case "darwin":
		support := filepath.Join(home, "Library", "Application Support")
		chrome = filepath.Join(support, "Google", "Chrome")
		brave = filepath.Join(support, "BraveSoftware", "Brave-Browser")
	case "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		config := filepath.Join(home, ".config")
		chrome = filepath.Join(config, "google-chrome")
		brave = filepath.Join(config, "BraveSoftware", "Brave-Browser")
	default:
		return manifestLocation{}, fmt.Errorf("%w: %s", errUnsupportedOS, goos)
	}
	switch browser {
	case BrowserChrome:
		return manifestLocation{browserDir: chrome, hostsDir: filepath.Join(chrome, hostsDirName)}, nil
	case BrowserBrave:
		hostsBase := brave
		if goos == "darwin" {
			hostsBase = chrome
		}
		return manifestLocation{browserDir: brave, hostsDir: filepath.Join(hostsBase, hostsDirName)}, nil
	default:
		return manifestLocation{}, ErrUnknownBrowser
	}
}

// Install writes the manifest of browser, pointing at executable, under home,
// using the directory layout of runtime.GOOS. It returns the manifest path.
// On darwin, BrowserChrome and BrowserBrave share one manifest path.
func Install(home string, browser Browser, executable string) (string, error) {
	return install(runtime.GOOS, home, browser, executable)
}

// install is Install for goos.
// Contract: it changes nothing but the manifest file and, when missing, the
// NativeMessagingHosts directory and its parents; a rejection changes
// nothing, and a file that is not this host's manifest is never replaced.
func install(goos, home string, browser Browser, executable string) (string, error) {
	loc, err := locate(goos, home, browser)
	if err != nil {
		return "", err
	}
	if err := checkExecutable(executable); err != nil {
		return "", err
	}
	if info, err := os.Stat(loc.browserDir); err != nil || !info.IsDir() {
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", ErrBrowserNotFound, loc.browserDir)
	}
	path := loc.manifestPath()
	state, err := readManifestState(path)
	if err != nil {
		return "", err
	}
	switch state {
	case manifestAbsent, manifestOwn:
	default: // manifestForeign and every value outside the constants
		return "", fmt.Errorf("%w: %s", ErrForeignManifest, path)
	}
	content, err := manifestContent(executable)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(loc.hostsDir, hostsDirMode); err != nil {
		return "", err
	}
	if err := replaceFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}

// Uninstall removes the manifest of browser under home when it is one this
// host wrote. removed is false when there was no manifest.
func Uninstall(home string, browser Browser) (path string, removed bool, err error) {
	return uninstall(runtime.GOOS, home, browser)
}

// uninstall is Uninstall for goos.
// Contract: it removes only this host's manifest file, never a directory and
// never a file this host did not write.
func uninstall(goos, home string, browser Browser) (string, bool, error) {
	loc, err := locate(goos, home, browser)
	if err != nil {
		return "", false, err
	}
	path := loc.manifestPath()
	state, err := readManifestState(path)
	if err != nil {
		return path, false, err
	}
	switch state {
	case manifestAbsent:
		return path, false, nil
	case manifestOwn:
		if err := os.Remove(path); err != nil {
			return path, false, err
		}
		syncDir(loc.hostsDir)
		return path, true, nil
	default: // manifestForeign and every value outside the constants
		return path, false, fmt.Errorf("%w: %s", ErrForeignManifest, path)
	}
}

// checkExecutable rejects an executable the manifest must not point at: a
// relative path, or one under the temporary directory (a binary go run
// builds and removes after the run). The caller resolves symbolic links, so
// the temporary directory is compared both as given and resolved.
func checkExecutable(executable string) error {
	if !filepath.IsAbs(executable) {
		return fmt.Errorf("%w: executable %s", errNotAbsolute, executable)
	}
	temp := os.TempDir()
	dirs := []string{temp}
	if resolved, err := filepath.EvalSymlinks(temp); err == nil {
		dirs = append(dirs, resolved)
	}
	for _, dir := range dirs {
		if isWithin(filepath.Clean(dir), filepath.Clean(executable)) {
			return fmt.Errorf("%w: %s", ErrUnstableExecutable, executable)
		}
	}
	return nil
}

// isWithin reports whether path is dir or below it. path is a clean absolute
// path. A dir it cannot be compared with, such as a relative TMPDIR, counts as
// containing it, so the check never passes by default.
func isWithin(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// manifestJSON is the manifest this host writes; the field order is the
// member order of the file.
type manifestJSON struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// manifestContent returns the manifest file's bytes for executable.
func manifestContent(executable string) ([]byte, error) {
	data, err := json.MarshalIndent(manifestJSON{
		Name:           HostName,
		Description:    manifestDescription,
		Path:           executable,
		Type:           manifestType,
		AllowedOrigins: []string{ExtensionOrigin},
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// manifestState is what is at the manifest path. The zero value is
// manifestForeign, so an unset state never lets a file be replaced or removed.
type manifestState int

const (
	manifestForeign manifestState = iota
	manifestAbsent
	manifestOwn
)

// readManifestState reports what is at path. A file that cannot be recognized
// as this host's manifest is manifestForeign; an error is returned only when
// the file cannot be examined at all.
func readManifestState(path string) (manifestState, error) {
	// O_NOFOLLOW refuses a symbolic link at the name instead of reading its
	// target; O_NONBLOCK makes opening a named pipe return at once. The type
	// is checked on the opened file, so no name swap can pass between a check
	// and the open.
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) //nolint:gosec // path is the fixed manifest name under the user's own browser directory
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return manifestAbsent, nil
	case errors.Is(err, syscall.ELOOP):
		return manifestForeign, nil
	case err != nil:
		return manifestForeign, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return manifestForeign, err
	}
	if !info.Mode().IsRegular() {
		return manifestForeign, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
	if err != nil {
		return manifestForeign, err
	}
	if len(data) > maxManifestBytes || !isOwnManifest(data) {
		return manifestForeign, nil
	}
	return manifestOwn, nil
}

// isOwnManifest reports whether data is a manifest this host wrote: exactly
// the five members, the fixed name, type, and allowed origins, a string
// description, and an absolute path. The description and the path are not
// compared, so a changed description or a moved binary can be reinstalled.
func isOwnManifest(data []byte) bool {
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return false
	}
	// A missing member is the zero Value, whose accessors are documented to
	// fail, so each accessor below also rejects its member's absence.
	members, err := object.CollectOnly(manifestMembers...)
	if err != nil {
		return false
	}
	name, err := members["name"].AsString()
	if err != nil || name != HostName {
		return false
	}
	kind, err := members["type"].AsString()
	if err != nil || kind != manifestType {
		return false
	}
	if _, err := members["description"].AsString(); err != nil {
		return false
	}
	path, err := members["path"].AsString()
	if err != nil || !filepath.IsAbs(path) {
		return false
	}
	values, err := members["allowed_origins"].AsArray()
	if err != nil {
		return false
	}
	origins := make([]string, 0, len(values))
	for _, value := range values {
		origin, err := value.AsString()
		if err != nil {
			return false
		}
		origins = append(origins, origin)
	}
	return slices.Equal(origins, []string{ExtensionOrigin})
}

// replaceFile puts content at path through a synced temporary file in the
// same directory and a rename, then syncs the directory. The temporary file
// is created with mode 0600 and removed on any failure before the rename.
func replaceFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, HostName+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	fail := func(err error) error {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if _, err := file.Write(content); err != nil {
		return fail(err)
	}
	if err := file.Sync(); err != nil {
		return fail(err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	syncDir(dir)
	return nil
}

// syncDir makes a directory entry change durable. A failure is ignored: the
// rename or removal has already taken effect.
func syncDir(dir string) {
	d, err := os.Open(dir) //nolint:gosec // dir is the NativeMessagingHosts directory under the user's own home
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
