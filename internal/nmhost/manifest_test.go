//go:build test

package nmhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"testing"
)

// testExecutable is an absolute executable path outside the temporary
// directory. Install does not require it to exist.
const testExecutable = "/opt/yt2column/build/yt2column-nmhost"

// The browser user data directories and manifest paths of the layouts under
// test, relative to home.
const (
	darwinChromeDir     = "Library/Application Support/Google/Chrome"
	darwinBraveDir      = "Library/Application Support/BraveSoftware/Brave-Browser"
	darwinManifest      = darwinChromeDir + "/NativeMessagingHosts/io.github.isseis.yt2column.json"
	linuxChromeDir      = ".config/google-chrome"
	linuxBraveDir       = ".config/BraveSoftware/Brave-Browser"
	linuxChromeManifest = linuxChromeDir + "/NativeMessagingHosts/io.github.isseis.yt2column.json"
	linuxBraveManifest  = linuxBraveDir + "/NativeMessagingHosts/io.github.isseis.yt2column.json"
	ownManifestOrigin   = "chrome-extension://clfmbbcdpnjcefbdihdoahomaifbabkk/"
)

// newHome returns a temporary home holding the given directories, relative
// to it.
func newHome(t *testing.T, dirs ...string) string {
	t.Helper()
	home := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(dir)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// treeOf lists every path under root, relative to it.
func treeOf(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		paths = append(paths, rel)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

// manifestDoc returns the members of a manifest this host writes for
// testExecutable, as a map the caller may change.
func manifestDoc() map[string]any {
	return map[string]any{
		"name":            "io.github.isseis.yt2column",
		"description":     "yt2column: relays the extension's request to the local server",
		"path":            testExecutable,
		"type":            "stdio",
		"allowed_origins": []any{ownManifestOrigin},
	}
}

func marshalDoc(t *testing.T, doc map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// writeFile writes data at path, creating its directory.
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// foreignManifests are files at the manifest path that are not this host's
// manifest. Each breaks exactly one condition of the recognition.
var foreignManifests = []struct {
	name  string
	place func(t *testing.T, path string)
}{
	{"other host name", placeDoc(func(d map[string]any) { d["name"] = "com.example.other" })},
	{"other type", placeDoc(func(d map[string]any) { d["type"] = "socket" })},
	{"other origin", placeDoc(func(d map[string]any) {
		d["allowed_origins"] = []any{"chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/"}
	})},
	{"second origin", placeDoc(func(d map[string]any) {
		d["allowed_origins"] = []any{ownManifestOrigin, "chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/"}
	})},
	{"origin not a string", placeDoc(func(d map[string]any) { d["allowed_origins"] = []any{1} })},
	{"origins not an array", placeDoc(func(d map[string]any) { d["allowed_origins"] = ownManifestOrigin })},
	{"relative path", placeDoc(func(d map[string]any) { d["path"] = "build/yt2column-nmhost" })},
	{"description not a string", placeDoc(func(d map[string]any) { d["description"] = 1 })},
	{"extra member", placeDoc(func(d map[string]any) { d["allowed_extensions"] = []any{} })},
	{"missing member", placeDoc(func(d map[string]any) { delete(d, "description") })},
	{"duplicate member", func(t *testing.T, path string) {
		t.Helper()
		data := marshalDoc(t, manifestDoc())
		writeFile(t, path, append([]byte(`{"name":"com.example.other",`), data[1:]...))
	}},
	{"not JSON", func(t *testing.T, path string) {
		t.Helper()
		writeFile(t, path, []byte("not a manifest\n"))
	}},
	// This host's manifest padded with trailing whitespace: the first 64 KiB
	// still parse as this host's, so only the size cap rejects it.
	{"over 64 KiB", func(t *testing.T, path string) {
		t.Helper()
		writeFile(t, path, append(marshalDoc(t, manifestDoc()), bytes.Repeat([]byte(" "), maxManifestBytes)...))
	}},
	{"symbolic link to this host's manifest", func(t *testing.T, path string) {
		t.Helper()
		target := filepath.Join(t.TempDir(), "manifest.json")
		writeFile(t, target, marshalDoc(t, manifestDoc()))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}},
	{"directory", func(t *testing.T, path string) {
		t.Helper()
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}},
}

func placeDoc(change func(map[string]any)) func(t *testing.T, path string) {
	return func(t *testing.T, path string) {
		t.Helper()
		doc := manifestDoc()
		change(doc)
		writeFile(t, path, marshalDoc(t, doc))
	}
}

// snapshot records what is at path so a test can show it was left unchanged.
type snapshot struct {
	info fs.FileInfo
	data []byte
}

func takeSnapshot(t *testing.T, path string) snapshot {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	var data []byte
	if info.Mode().IsRegular() {
		if data, err = os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
	}
	return snapshot{info: info, data: data}
}

func (s snapshot) requireUnchanged(t *testing.T, path string) {
	t.Helper()
	now := takeSnapshot(t, path)
	if !os.SameFile(s.info, now.info) || s.info.Mode() != now.info.Mode() || !bytes.Equal(s.data, now.data) {
		t.Errorf("%s changed", path)
	}
}

func TestParseBrowser(t *testing.T) {
	for name, want := range map[string]Browser{"chrome": BrowserChrome, "brave": BrowserBrave} {
		got, err := ParseBrowser(name)
		if err != nil || got != want {
			t.Errorf("ParseBrowser(%q) = %v, %v; want %v", name, got, err, want)
		}
	}
	for _, name := range []string{"", "Chrome", "BRAVE", " chrome", "chrome ", "chromium", "google-chrome", "brave-browser"} {
		got, err := ParseBrowser(name)
		if !errors.Is(err, ErrUnknownBrowser) || got != BrowserUnset {
			t.Errorf("ParseBrowser(%q) = %v, %v; want BrowserUnset, ErrUnknownBrowser", name, got, err)
		}
	}
}

func TestManifestLocation(t *testing.T) {
	tests := []struct {
		goos       string
		browser    Browser
		browserDir string
		manifest   string
	}{
		{"darwin", BrowserChrome, darwinChromeDir, darwinManifest},
		// Brave on macOS reads Chrome's directory, so only the existence check
		// looks at Brave's own directory.
		{"darwin", BrowserBrave, darwinBraveDir, darwinManifest},
		{"linux", BrowserChrome, linuxChromeDir, linuxChromeManifest},
		{"linux", BrowserBrave, linuxBraveDir, linuxBraveManifest},
		{"freebsd", BrowserBrave, linuxBraveDir, linuxBraveManifest},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"/"+tt.browserDir, func(t *testing.T) {
			home := "/home/someone"
			loc, err := locate(tt.goos, home, tt.browser)
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(home, filepath.FromSlash(tt.browserDir)); loc.browserDir != want {
				t.Errorf("browser directory = %s, want %s", loc.browserDir, want)
			}
			if want := filepath.Join(home, filepath.FromSlash(tt.manifest)); loc.manifestPath() != want {
				t.Errorf("manifest path = %s, want %s", loc.manifestPath(), want)
			}
		})
	}

	rejected := []struct {
		name    string
		goos    string
		home    string
		browser Browser
		want    error
	}{
		{"unset browser", "darwin", "/home/someone", BrowserUnset, ErrUnknownBrowser},
		{"browser outside the constants", "linux", "/home/someone", BrowserBrave + 1, ErrUnknownBrowser},
		{"relative home", "linux", "home/someone", BrowserChrome, errNotAbsolute},
		{"empty home", "darwin", "", BrowserChrome, errNotAbsolute},
		{"windows", "windows", "/home/someone", BrowserChrome, errUnsupportedOS},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := locate(tt.goos, tt.home, tt.browser); !errors.Is(err, tt.want) {
				t.Errorf("locate error = %v, want %v", err, tt.want)
			}
		})
	}

	// The existence check and the write use different directories for Brave
	// on macOS: Brave's directory alone is enough to install there, and
	// Chrome's directory alone is not.
	t.Run("darwin brave installs into chrome's directory", func(t *testing.T) {
		home := newHome(t, darwinBraveDir)
		path, err := install("darwin", home, BrowserBrave, testExecutable)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, filepath.FromSlash(darwinManifest)); path != want {
			t.Errorf("manifest path = %s, want %s", path, want)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("manifest not written: %v", err)
		}
	})
	t.Run("darwin brave needs brave's directory", func(t *testing.T) {
		home := newHome(t, darwinChromeDir)
		before := treeOf(t, home)
		if _, err := install("darwin", home, BrowserBrave, testExecutable); !errors.Is(err, ErrBrowserNotFound) {
			t.Fatalf("install error = %v, want ErrBrowserNotFound", err)
		}
		if after := treeOf(t, home); !slices.Equal(before, after) {
			t.Errorf("home changed from %v to %v", before, after)
		}
	})
}

func TestInstall(t *testing.T) {
	t.Run("writes the manifest", func(t *testing.T) {
		home := newHome(t, linuxBraveDir)
		path, err := install("linux", home, BrowserBrave, testExecutable)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, filepath.FromSlash(linuxBraveManifest)); path != want {
			t.Fatalf("manifest path = %s, want %s", path, want)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("manifest is not JSON: %v", err)
		}
		if want := manifestDoc(); !marshalEqual(t, got, want) {
			t.Errorf("manifest = %v, want %v", got, want)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("manifest mode = %o, want 600", mode)
		}
		hostsDir := filepath.Dir(path)
		dirInfo, err := os.Stat(hostsDir)
		if err != nil {
			t.Fatal(err)
		}
		if mode := dirInfo.Mode().Perm(); mode != 0o700 {
			t.Errorf("created directory mode = %o, want 700", mode)
		}
		// Nothing but the manifest is left: the temporary file was renamed.
		if names := dirNames(t, hostsDir); !slices.Equal(names, []string{HostName + ".json"}) {
			t.Errorf("directory holds %v, want only the manifest", names)
		}
	})

	t.Run("replaces its own manifest", func(t *testing.T) {
		home := newHome(t, linuxChromeDir)
		path := filepath.Join(home, filepath.FromSlash(linuxChromeManifest))
		// An earlier install with another description and binary is still this
		// host's.
		old := manifestDoc()
		old["description"] = "an older description"
		old["path"] = "/usr/local/bin/yt2column-nmhost"
		writeFile(t, path, marshalDoc(t, old))
		before := takeSnapshot(t, path)

		if _, err := install("linux", home, BrowserChrome, testExecutable); err != nil {
			t.Fatal(err)
		}
		after := takeSnapshot(t, path)
		if !bytes.Contains(after.data, []byte(`"path": "`+testExecutable+`"`)) {
			t.Errorf("manifest not replaced: %s", after.data)
		}
		// A rename puts a new file in place; writing over the old one would
		// let the browser read a partly written manifest.
		if os.SameFile(before.info, after.info) {
			t.Error("the manifest was rewritten in place, not replaced by a new file")
		}
	})

	t.Run("public Install uses this system's layout", func(t *testing.T) {
		chromeDir := linuxChromeDir
		if runtime.GOOS == "darwin" {
			chromeDir = darwinChromeDir
		}
		home := newHome(t, chromeDir)
		path, err := Install(home, BrowserChrome, testExecutable)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(home, filepath.FromSlash(chromeDir), hostsDirName, HostName+".json"); path != want {
			t.Errorf("manifest path = %s, want %s", path, want)
		}
	})

	t.Run("no browser directory", func(t *testing.T) {
		for _, goos := range []string{"darwin", "linux"} {
			for _, browser := range []Browser{BrowserChrome, BrowserBrave} {
				home := newHome(t)
				if _, err := install(goos, home, browser, testExecutable); !errors.Is(err, ErrBrowserNotFound) {
					t.Errorf("%s/%d: install error = %v, want ErrBrowserNotFound", goos, browser, err)
				}
				if tree := treeOf(t, home); !slices.Equal(tree, []string{"."}) {
					t.Errorf("%s/%d: install created %v", goos, browser, tree)
				}
			}
		}
	})

	t.Run("browser path is a file", func(t *testing.T) {
		home := newHome(t, ".config")
		writeFile(t, filepath.Join(home, filepath.FromSlash(linuxChromeDir)), []byte("x"))
		if _, err := install("linux", home, BrowserChrome, testExecutable); !errors.Is(err, ErrBrowserNotFound) {
			t.Errorf("install error = %v, want ErrBrowserNotFound", err)
		}
	})

	// A file where a directory of the path should be cannot be examined, so
	// it is reported as it is rather than as a missing browser.
	t.Run("parent of the browser directory is a file", func(t *testing.T) {
		home := newHome(t)
		writeFile(t, filepath.Join(home, ".config"), []byte("x"))
		if _, err := install("linux", home, BrowserChrome, testExecutable); !errors.Is(err, syscall.ENOTDIR) {
			t.Errorf("install error = %v, want ENOTDIR", err)
		}
	})

	for _, tt := range foreignManifests {
		t.Run("foreign: "+tt.name, func(t *testing.T) {
			home := newHome(t, linuxChromeDir)
			path := filepath.Join(home, filepath.FromSlash(linuxChromeManifest))
			tt.place(t, path)
			before := takeSnapshot(t, path)
			if _, err := install("linux", home, BrowserChrome, testExecutable); !errors.Is(err, ErrForeignManifest) {
				t.Fatalf("install error = %v, want ErrForeignManifest", err)
			}
			before.requireUnchanged(t, path)
		})
	}

	t.Run("unstable executable", func(t *testing.T) {
		// TMPDIR is a symbolic link, so the executable is checked against the
		// temporary directory both as given and resolved.
		// The root is resolved so that target is the resolved form of link.
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, "real")
		link := filepath.Join(root, "link")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		home := newHome(t, linuxChromeDir)
		t.Setenv("TMPDIR", link)

		for _, executable := range []string{
			filepath.Join(link, "go-build1", "b001", "exe", "yt2column-nmhost"),
			filepath.Join(target, "go-build1", "b001", "exe", "yt2column-nmhost"),
		} {
			if _, err := install("linux", home, BrowserChrome, executable); !errors.Is(err, ErrUnstableExecutable) {
				t.Errorf("install(%s) error = %v, want ErrUnstableExecutable", executable, err)
			}
		}
		if tree := treeOf(t, filepath.Join(home, filepath.FromSlash(linuxChromeDir))); !slices.Equal(tree, []string{"."}) {
			t.Errorf("rejected install created %v", tree)
		}
		// A sibling whose name starts with the temporary directory's is not
		// under it.
		if _, err := install("linux", home, BrowserChrome, target+"-bin/yt2column-nmhost"); err != nil {
			t.Errorf("install outside the temporary directory: %v", err)
		}
	})

	t.Run("relative executable", func(t *testing.T) {
		home := newHome(t, linuxChromeDir)
		if _, err := install("linux", home, BrowserChrome, "build/yt2column-nmhost"); !errors.Is(err, errNotAbsolute) {
			t.Errorf("install error = %v, want errNotAbsolute", err)
		}
	})

	t.Run("unreadable manifest", func(t *testing.T) {
		skipIfRoot(t)
		home := newHome(t, linuxChromeDir)
		path := filepath.Join(home, filepath.FromSlash(linuxChromeManifest))
		writeFile(t, path, marshalDoc(t, manifestDoc()))
		before := takeSnapshot(t, path)
		chmodForTest(t, path, 0o000)
		_, err := install("linux", home, BrowserChrome, testExecutable)
		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("install error = %v, want a permission error", err)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		before.requireUnchanged(t, path)
	})

	t.Run("unwritable directory", func(t *testing.T) {
		skipIfRoot(t)
		for _, dir := range []string{linuxChromeDir, linuxChromeDir + "/NativeMessagingHosts"} {
			home := newHome(t, dir)
			chmodForTest(t, filepath.Join(home, filepath.FromSlash(dir)), 0o500)
			if _, err := install("linux", home, BrowserChrome, testExecutable); !errors.Is(err, fs.ErrPermission) {
				t.Errorf("%s: install error = %v, want a permission error", dir, err)
			}
		}
	})
}

func TestUninstall(t *testing.T) {
	t.Run("removes its own manifest", func(t *testing.T) {
		home := newHome(t, linuxBraveDir)
		installed, err := install("linux", home, BrowserBrave, testExecutable)
		if err != nil {
			t.Fatal(err)
		}
		path, removed, err := uninstall("linux", home, BrowserBrave)
		if err != nil || !removed || path != installed {
			t.Fatalf("uninstall = %s, %v, %v; want %s, true, nil", path, removed, err, installed)
		}
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("manifest still present: %v", err)
		}
		// The directory may hold other hosts' manifests, so it stays.
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			t.Errorf("NativeMessagingHosts removed: %v", err)
		}
	})

	// On macOS one file serves both browsers, so uninstalling for one removes
	// what an install for the other wrote.
	t.Run("darwin shares the manifest", func(t *testing.T) {
		home := newHome(t, darwinChromeDir, darwinBraveDir)
		if _, err := install("darwin", home, BrowserBrave, testExecutable); err != nil {
			t.Fatal(err)
		}
		if _, removed, err := uninstall("darwin", home, BrowserChrome); err != nil || !removed {
			t.Fatalf("uninstall chrome = %v, %v; want removed", removed, err)
		}
		if _, removed, err := uninstall("darwin", home, BrowserBrave); err != nil || removed {
			t.Errorf("uninstall brave afterwards = %v, %v; want nothing to remove", removed, err)
		}
	})

	t.Run("public Uninstall uses this system's layout", func(t *testing.T) {
		chromeDir := linuxChromeDir
		if runtime.GOOS == "darwin" {
			chromeDir = darwinChromeDir
		}
		home := newHome(t, chromeDir)
		installed, err := Install(home, BrowserChrome, testExecutable)
		if err != nil {
			t.Fatal(err)
		}
		if path, removed, err := Uninstall(home, BrowserChrome); err != nil || !removed || path != installed {
			t.Errorf("Uninstall = %s, %v, %v; want %s, true, nil", path, removed, err, installed)
		}
	})

	t.Run("no manifest", func(t *testing.T) {
		home := newHome(t)
		path, removed, err := uninstall("linux", home, BrowserChrome)
		if err != nil || removed {
			t.Errorf("uninstall = %v, %v; want not removed, nil", removed, err)
		}
		if want := filepath.Join(home, filepath.FromSlash(linuxChromeManifest)); path != want {
			t.Errorf("manifest path = %s, want %s", path, want)
		}
		if tree := treeOf(t, home); !slices.Equal(tree, []string{"."}) {
			t.Errorf("uninstall created %v", tree)
		}
	})

	for _, tt := range foreignManifests {
		t.Run("foreign: "+tt.name, func(t *testing.T) {
			home := newHome(t)
			path := filepath.Join(home, filepath.FromSlash(linuxChromeManifest))
			tt.place(t, path)
			before := takeSnapshot(t, path)
			if _, removed, err := uninstall("linux", home, BrowserChrome); !errors.Is(err, ErrForeignManifest) || removed {
				t.Fatalf("uninstall = %v, %v; want not removed, ErrForeignManifest", removed, err)
			}
			before.requireUnchanged(t, path)
		})
	}

	t.Run("parent of the manifest directory is a file", func(t *testing.T) {
		home := newHome(t)
		writeFile(t, filepath.Join(home, ".config"), []byte("x"))
		if _, removed, err := uninstall("linux", home, BrowserChrome); !errors.Is(err, syscall.ENOTDIR) || removed {
			t.Errorf("uninstall = %v, %v; want not removed, ENOTDIR", removed, err)
		}
	})

	t.Run("rejected browser", func(t *testing.T) {
		if _, _, err := uninstall("linux", newHome(t), BrowserUnset); !errors.Is(err, ErrUnknownBrowser) {
			t.Errorf("uninstall error = %v, want ErrUnknownBrowser", err)
		}
	})

	t.Run("unremovable manifest", func(t *testing.T) {
		skipIfRoot(t)
		home := newHome(t, linuxChromeDir)
		path, err := install("linux", home, BrowserChrome, testExecutable)
		if err != nil {
			t.Fatal(err)
		}
		chmodForTest(t, filepath.Dir(path), 0o500)
		if _, removed, err := uninstall("linux", home, BrowserChrome); !errors.Is(err, fs.ErrPermission) || removed {
			t.Errorf("uninstall = %v, %v; want not removed, a permission error", removed, err)
		}
	})

	t.Run("unreadable manifest", func(t *testing.T) {
		skipIfRoot(t)
		home := newHome(t, linuxChromeDir)
		path, err := install("linux", home, BrowserChrome, testExecutable)
		if err != nil {
			t.Fatal(err)
		}
		chmodForTest(t, path, 0o000)
		if _, removed, err := uninstall("linux", home, BrowserChrome); !errors.Is(err, fs.ErrPermission) || removed {
			t.Errorf("uninstall = %v, %v; want not removed, a permission error", removed, err)
		}
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("manifest removed: %v", err)
		}
	})
}

// marshalEqual compares two decoded JSON values by their encoding.
func marshalEqual(t *testing.T, a, b any) bool {
	t.Helper()
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(ja, jb)
}

// dirNames lists the names in dir.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
