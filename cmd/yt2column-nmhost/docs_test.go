//go:build test

package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/nmhost"
)

// repositoryRoot is the path from cmd/yt2column-nmhost to the repository root.
const repositoryRoot = "../.."

// manifestSpan matches a code span that holds a manifest path under $HOME.
var manifestSpan = regexp.MustCompile("`(\\$HOME/[^`]*" + regexp.QuoteMeta(nmhost.HostName) + "\\.json)`")

// TestReadmeMigrationManifestPaths checks that the migration section of each
// README version names exactly the manifest files uninstall removes, for
// Chrome and Brave on macOS and on the other Unix systems, so a user who can
// no longer run the host can remove them by hand. The paths come from the
// host's own location rule under a placeholder home written as $HOME.
func TestReadmeMigrationManifestPaths(t *testing.T) {
	const home = "/home-placeholder"
	var want []string
	for _, goos := range []string{"darwin", "linux"} {
		for _, browser := range []nmhost.Browser{nmhost.BrowserChrome, nmhost.BrowserBrave} {
			path, err := nmhost.ManifestPathForTest(goos, home, browser)
			if err != nil {
				t.Fatalf("manifest path of browser %d on %s: %v", browser, goos, err)
			}
			path = "$HOME" + strings.TrimPrefix(path, home)
			if !slices.Contains(want, path) {
				want = append(want, path)
			}
		}
	}
	slices.Sort(want)
	for _, v := range doctestutil.ReadmeVersions() {
		t.Run(v.Path, func(t *testing.T) {
			heading := doctestutil.ReadmeMigrationHeading.In(t, v.Lang)
			section := doctestutil.SectionOutsideBlocks(doctestutil.ReadDoc(t, v.From(repositoryRoot)), heading)
			if section == "" {
				t.Fatalf("%s has no %q section", v.Path, heading)
			}
			var got []string
			for _, match := range manifestSpan.FindAllStringSubmatch(section, -1) {
				got = append(got, match[1])
			}
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("%s %s names the manifest files %q, want %q", v.Path, heading, got, want)
			}
		})
	}
}
