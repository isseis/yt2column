//go:build test

package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/isseis/yt2column/internal/doctestutil"
	"github.com/isseis/yt2column/internal/nmhost"
)

// Paths from cmd/yt2column to the browser-extension artifacts and documents
// these guards check.
const (
	extensionManifestPath = repositoryRoot + "/extension/static/manifest.json"
	claudePath            = repositoryRoot + "/CLAUDE.md"
	makefilePath          = repositoryRoot + "/Makefile"
	extensionSendPath     = repositoryRoot + "/extension/src/send.ts"
	extensionNativePath   = repositoryRoot + "/extension/src/core/native.ts"
)

// extTargetRE matches a make target definition for an extension target, such as
// "ext-test:" at the start of a line. The Makefile's .PHONY line starts with
// ".PHONY", so it does not match.
var extTargetRE = regexp.MustCompile(`(?m)^(ext-[a-z0-9-]+):`)

// claudeExtTargetRE matches a bullet that lists an extension make target, such
// as "- `make ext-test`" at the start of a line.
var claudeExtTargetRE = regexp.MustCompile("(?m)^- `make (ext-[a-z0-9-]+)`")

// extensionManifest is the subset of extension/static/manifest.json these
// guards read.
type extensionManifest struct {
	Key         string   `json:"key"`
	Permissions []string `json:"permissions"`
}

// TestExtensionDocumentation pins the text that the browser extension's
// documents and the Makefile must keep in sync: the fixed extension ID, the
// ext- target list, and the requested permissions. Each document is read once
// and shared by the subtests. None of these checks invokes node or npm.
func TestExtensionDocumentation(t *testing.T) {
	manifest := readExtensionManifest(t)
	extensionID, err := extensionIDFromManifestKey(manifest.Key)
	if err != nil {
		t.Fatalf("derive the extension ID from the manifest key: %v", err)
	}
	security := doctestutil.ReadDoc(t, securityPath)
	claude := doctestutil.ReadDoc(t, claudePath)
	makefile := doctestutil.ReadDoc(t, makefilePath)

	t.Run("records the fixed extension ID in README and security.md", func(t *testing.T) {
		docs := []struct{ name, text string }{{"security.md", security}}
		for _, v := range doctestutil.ReadmeVersions() {
			docs = append(docs, struct{ name, text string }{v.Path, doctestutil.ReadDoc(t, v.From(repositoryRoot))})
		}
		for _, doc := range docs {
			if !strings.Contains(doc.text, extensionID) {
				t.Errorf("%s does not record the extension ID %s", doc.name, extensionID)
			}
		}
	})

	t.Run("lists every ext- target the Makefile defines, and no others", func(t *testing.T) {
		targets := submatchAll(extTargetRE, makefile, 1)
		if len(targets) == 0 {
			t.Fatal("the Makefile defines no ext- targets")
		}
		listed := submatchAll(claudeExtTargetRE, claude, 1)
		if got, want := sortedUnique(listed), sortedUnique(targets); !slices.Equal(got, want) {
			t.Errorf("CLAUDE.md's extension command list %v does not match the Makefile's %v", got, want)
		}
	})

	t.Run("lists every manifest permission in the extension section of security.md", func(t *testing.T) {
		if len(manifest.Permissions) == 0 {
			t.Fatal("manifest.json requests no permissions")
		}
		section := doctestutil.SectionByHeading(security, "### 8.1. ブラウザ拡張（#110）")
		if section == "" {
			t.Fatal("security.md has no 8.1 extension section")
		}
		declared, ok := declaredPermissions(section)
		if !ok {
			t.Fatal("security.md 8.1 has no 要求する権限 list")
		}
		inManifest := map[string]struct{}{}
		for _, permission := range manifest.Permissions {
			inManifest[permission] = struct{}{}
		}
		inDoc := map[string]struct{}{}
		for _, permission := range declared {
			inDoc[permission] = struct{}{}
		}
		for _, permission := range manifest.Permissions {
			if _, ok := inDoc[permission]; !ok {
				t.Errorf("security.md 8.1 does not list the manifest permission %q", permission)
			}
		}
		for _, permission := range declared {
			if _, ok := inManifest[permission]; !ok {
				t.Errorf("security.md 8.1 lists %q, which the manifest does not request", permission)
			}
		}
	})
}

// sendTimeoutRE matches the extension's declaration of how long it waits for
// a send, in milliseconds, with optional digit separators.
var sendTimeoutRE = regexp.MustCompile(`(?m)^export const sendTimeoutMs = ([0-9][0-9_]*);$`)

// nativeHostNameRE, hostReplyVersionRE and recordDeclarationRE match the
// declarations in extension/src/core/native.ts that the host must agree with.
// recordDeclarationRE captures the name and the body of a Record<..., true>
// constant; recordKeyRE matches one of its "key: true," lines.
var (
	nativeHostNameRE    = regexp.MustCompile(`(?m)^export const nativeHostName = "([^"]*)";$`)
	hostReplyVersionRE  = regexp.MustCompile(`(?m)^export const hostReplyVersion = ([0-9]+);$`)
	recordDeclarationRE = regexp.MustCompile(`(?ms)^export const (\w+): Record<\w+, true> = \{\n(.*?)^\};$`)
	recordKeyRE         = regexp.MustCompile(`(?m)^  (\w+): true,$`)
)

// declaredRecordKeys returns the keys of the Record constant named name in
// source, sorted, or nil when source declares no such constant.
func declaredRecordKeys(source, name string) []string {
	for _, m := range recordDeclarationRE.FindAllStringSubmatch(source, -1) {
		if m[1] == name {
			return sortedUnique(submatchAll(recordKeyRE, m[2], 1))
		}
	}
	return nil
}

// TestNativeHostContract pins the values the Native Messaging host and the
// extension must agree on: the only origin the host's manifest allows and its
// relay accepts is the one derived from the extension manifest's key; the
// extension sends to the host's name and reads the reply version, kinds and
// host errors the host writes; and the host gives up on the server before the
// extension gives up on the host, so a reply the host writes is never one the
// extension has stopped waiting for.
func TestNativeHostContract(t *testing.T) {
	t.Run("extension origin", func(t *testing.T) {
		id, err := extensionIDFromManifestKey(readExtensionManifest(t).Key)
		if err != nil {
			t.Fatalf("derive the extension ID: %v", err)
		}
		if want := "chrome-extension://" + id + "/"; nmhost.ExtensionOrigin != want {
			t.Errorf("nmhost.ExtensionOrigin = %q, want %q (from the extension manifest's key)", nmhost.ExtensionOrigin, want)
		}
	})
	t.Run("host name and reply values", func(t *testing.T) {
		source := doctestutil.ReadDoc(t, extensionNativePath)
		name := nativeHostNameRE.FindStringSubmatch(source)
		if name == nil {
			t.Fatalf("%s has no nativeHostName declaration", extensionNativePath)
		}
		if name[1] != nmhost.HostName {
			t.Errorf("nativeHostName = %q, want nmhost.HostName %q", name[1], nmhost.HostName)
		}
		wire := nmhost.WireValuesForTest(t)
		version := hostReplyVersionRE.FindStringSubmatch(source)
		if version == nil {
			t.Fatalf("%s has no hostReplyVersion declaration", extensionNativePath)
		}
		if version[1] != strconv.Itoa(wire.Version) {
			t.Errorf("hostReplyVersion = %s, want the host's %d", version[1], wire.Version)
		}
		for _, record := range []struct {
			name string
			want []string
		}{
			{"hostReplyKinds", wire.Kinds},
			{"hostErrorValues", wire.HostErrors},
		} {
			got := declaredRecordKeys(source, record.name)
			if got == nil {
				t.Errorf("%s has no %s declaration", extensionNativePath, record.name)
				continue
			}
			if want := sortedUnique(record.want); !slices.Equal(got, want) {
				t.Errorf("%s = %v, want the host's %v", record.name, got, want)
			}
		}
	})
	t.Run("relay timeout", func(t *testing.T) {
		m := sendTimeoutRE.FindStringSubmatch(doctestutil.ReadDoc(t, extensionSendPath))
		if m == nil {
			t.Fatalf("%s has no sendTimeoutMs declaration", extensionSendPath)
		}
		ms, err := strconv.Atoi(strings.ReplaceAll(m[1], "_", ""))
		if err != nil {
			t.Fatalf("parse sendTimeoutMs %q: %v", m[1], err)
		}
		if extension := time.Duration(ms) * time.Millisecond; nmhost.RelayTimeout >= extension {
			t.Errorf("nmhost.RelayTimeout = %v, want it shorter than the extension's sendTimeoutMs (%v)", nmhost.RelayTimeout, extension)
		}
	})
}

// TestExtensionIDFromManifestKeyRejectsUnparseableKey checks that a value that
// is not a DER public key is rejected. It uses no fixed key value, so it does
// not pin the derivation's output; the recorded ID in the documents does that.
func TestExtensionIDFromManifestKeyRejectsUnparseableKey(t *testing.T) {
	notAKey := base64.StdEncoding.EncodeToString([]byte("this is not a public key"))
	for _, key := range []string{"", "not base64!!", notAKey} {
		if _, err := extensionIDFromManifestKey(key); err == nil {
			t.Errorf("extensionIDFromManifestKey(%q) returned no error", key)
		}
	}
}

// TestDeclaredPermissions pins declaredPermissions's parsing: it captures each
// requested permission, ignores a code span inside a reason (which is not
// followed by a parenthesised reason itself), stops at the first sentence end
// so a later bullet's code span is not captured, and reports not-ok when the
// bullet or its list is missing.
func TestDeclaredPermissions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		section string
		want    []string
		ok      bool
	}{
		{
			name:    "captures each requested permission",
			section: "**要求する権限**（設計書 3.1 の manifest）: `activeTab`（タブを読む）、`storage`（保存する）。要求しない権限はない。",
			want:    []string{"activeTab", "storage"},
			ok:      true,
		},
		{
			name:    "ignores a code span inside a reason",
			section: "**要求する権限**: `storage`（`chrome.storage.session` で渡す）。",
			want:    []string{"storage"},
			ok:      true,
		},
		{
			name:    "ignores a nested code span followed by its own parenthesis",
			section: "**要求する権限**: `storage`（`activeTab`（使用例）には使わない）。",
			want:    []string{"storage"},
			ok:      true,
		},
		{
			name:    "keeps permissions after a period inside a reason",
			section: "**要求する権限**: `a`（理由。続き）、`b`（別の理由）。",
			want:    []string{"a", "b"},
			ok:      true,
		},
		{
			name:    "a top-level period ends the list",
			section: "**要求する権限**: `a`（理由）。この後 `b`（別）は要求しない。",
			want:    []string{"a"},
			ok:      true,
		},
		{
			name:    "stops before a later bullet's code span",
			section: "**要求する権限**: `activeTab`（タブを読む）。\n- **秘密鍵**: `key`（公開鍵）だけを使う。",
			want:    []string{"activeTab"},
			ok:      true,
		},
		{
			name:    "missing marker yields not ok",
			section: "### 8.1. ブラウザ拡張\n\n要求する権限はない。",
			want:    nil,
			ok:      false,
		},
		{
			name:    "marker without an item yields not ok",
			section: "**要求する権限**: なし。",
			want:    nil,
			ok:      false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := declaredPermissions(tc.section)
			if ok != tc.ok {
				t.Fatalf("declaredPermissions(...) ok = %v, want %v", ok, tc.ok)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("declaredPermissions(...) = %v, want %v", got, tc.want)
			}
		})
	}
}

// readExtensionManifest reads and parses the extension manifest, failing the
// test on error.
func readExtensionManifest(t *testing.T) extensionManifest {
	t.Helper()
	var manifest extensionManifest
	if err := json.Unmarshal([]byte(doctestutil.ReadDoc(t, extensionManifestPath)), &manifest); err != nil {
		t.Fatalf("parse %s: %v", extensionManifestPath, err)
	}
	return manifest
}

// extensionIDFromManifestKey derives the Chrome extension ID from a manifest
// "key": the first 16 bytes of the SHA-256 of the DER SubjectPublicKeyInfo, in
// hex, with each hex digit rewritten so that 0 becomes 'a' and f becomes 'p'.
// It returns an error when the value is not a parseable public key, which is
// how a pasted private key is rejected.
func extensionIDFromManifestKey(key string) (string, error) {
	der, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("decode the manifest key: %w", err)
	}
	publicKey, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return "", fmt.Errorf("parse the manifest key as a public key: %w", err)
	}
	der, err = x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("re-encode the manifest key: %w", err)
	}
	sum := sha256.Sum256(der)
	var id strings.Builder
	for _, b := range sum[:16] {
		id.WriteByte('a' + (b >> 4))
		id.WriteByte('a' + (b & 0x0f))
	}
	return id.String(), nil
}

// declaredPermissions returns the permissions listed on the requested-permissions
// bullet of a security.md extension section. A listed permission is a code span
// at the top level of the list, directly followed by its parenthesised reason;
// code spans inside a reason, including nested ones, are not permissions. The
// list ends at the first top-level sentence end, so a period inside a reason
// (at a parenthesis depth greater than zero) does not end it, and the
// permissions the section names as not requested are excluded. The second
// result is false when the bullet is missing or lists nothing.
func declaredPermissions(section string) ([]string, bool) {
	const marker = "**要求する権限**"
	start := strings.Index(section, marker)
	if start < 0 {
		return nil, false
	}
	rest := section[start:]
	var permissions []string
	depth := 0
loop:
	for i := 0; i < len(rest); {
		r, size := utf8.DecodeRuneInString(rest[i:])
		switch r {
		case '。':
			if depth == 0 {
				break loop
			}
			i += size
		case '（':
			depth++
			i += size
		case '）':
			if depth > 0 {
				depth--
			}
			i += size
		case '`':
			end := strings.IndexByte(rest[i+1:], '`')
			if end < 0 {
				i = len(rest)
				continue
			}
			name := rest[i+1 : i+1+end]
			next := i + 1 + end + 1
			if depth == 0 && strings.HasPrefix(rest[next:], "（") {
				permissions = append(permissions, name)
			}
			i = next
		default:
			i += size
		}
	}
	if len(permissions) == 0 {
		return nil, false
	}
	return permissions, true
}

// submatchAll returns the given capture group of every match of re in doc.
func submatchAll(re *regexp.Regexp, doc string, group int) []string {
	var matches []string
	for _, match := range re.FindAllStringSubmatch(doc, -1) {
		matches = append(matches, match[group])
	}
	return matches
}

// sortedUnique returns the distinct values of values in ascending order.
func sortedUnique(values []string) []string {
	seen := map[string]struct{}{}
	var unique []string
	for _, value := range values {
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			unique = append(unique, value)
		}
	}
	slices.Sort(unique)
	return unique
}
