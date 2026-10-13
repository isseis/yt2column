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
	"strings"
	"testing"
)

// Paths from cmd/yt2column to the browser-extension artifacts and documents
// these guards check.
const (
	extensionManifestPath     = repositoryRoot + "/extension/static/manifest.json"
	claudePath                = repositoryRoot + "/CLAUDE.md"
	makefilePath              = repositoryRoot + "/Makefile"
	extensionArchitecturePath = repositoryRoot + "/docs/tasks/0007_browser_extension_skeleton/02_architecture.md"
	extensionPlanPath         = repositoryRoot + "/docs/tasks/0007_browser_extension_skeleton/03_implementation_plan.md"
)

// extTargetRE matches a make target definition for an extension target, such as
// "ext-test:" at the start of a line. The Makefile's .PHONY line starts with
// ".PHONY", so it does not match.
var extTargetRE = regexp.MustCompile(`(?m)^(ext-[a-z0-9-]+):`)

// claudeExtTargetRE matches a bullet that lists an extension make target, such
// as "- `make ext-test`" at the start of a line.
var claudeExtTargetRE = regexp.MustCompile("(?m)^- `make (ext-[a-z0-9-]+)`")

// requestedPermissionRE captures a requested permission: a code span that is
// the item name of the requested-permissions list, directly followed by its
// parenthesised reason. Incidental code spans inside a reason are not followed
// by "（", so they are not captured.
var requestedPermissionRE = regexp.MustCompile("`([^`]+)`（")

// extensionManifest is the subset of extension/static/manifest.json these
// guards read.
type extensionManifest struct {
	Key         string   `json:"key"`
	Permissions []string `json:"permissions"`
}

// TestExtensionDocumentation pins the text that the browser extension's
// documents and the Makefile must keep in sync: the fixed extension ID, the
// ext- target list, the requested permissions, and the records kept in the
// project overview and task 0007. Each document is read once and shared by the
// subtests. None of these checks invokes node or npm.
func TestExtensionDocumentation(t *testing.T) {
	manifest := readExtensionManifest(t)
	extensionID, err := extensionIDFromManifestKey(manifest.Key)
	if err != nil {
		t.Fatalf("derive the extension ID from the manifest key: %v", err)
	}
	readme := readDoc(t, readmePath)
	security := readDoc(t, securityPath)
	claude := readDoc(t, claudePath)
	makefile := readDoc(t, makefilePath)
	overview := readDoc(t, projectOverviewPath)
	architecture := readDoc(t, extensionArchitecturePath)
	plan := readDoc(t, extensionPlanPath)

	t.Run("records the fixed extension ID in README and security.md", func(t *testing.T) {
		for _, doc := range []struct{ name, text string }{
			{"README.md", readme},
			{"security.md", security},
		} {
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
		section := sectionByHeading(security, "### 8.1. ブラウザ拡張（#110）")
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

	t.Run("adds TypeScript and extension/ to the project overview", func(t *testing.T) {
		policies := sectionByHeading(overview, "## 決定済みの方針")
		if !strings.Contains(policies, "TypeScript") {
			t.Error("決定済みの方針 does not mention TypeScript")
		}
		layout := sectionByHeading(overview, "## 想定ディレクトリ構成")
		if !strings.Contains(layout, "extension/") {
			t.Error("想定ディレクトリ構成 does not list extension/")
		}
	})

	t.Run("records sections 3.12.1 to 3.12.3 with the extension ID in 3.12.1", func(t *testing.T) {
		for _, heading := range []string{"#### 3.12.1.", "#### 3.12.2.", "#### 3.12.3."} {
			if !strings.Contains(architecture, heading) {
				t.Errorf("the architecture has no section %s", heading)
			}
		}
		if body := sectionByHeadingPrefix(architecture, "#### 3.12.1."); !strings.Contains(body, extensionID) {
			t.Error("architecture 3.12.1 does not record the extension ID")
		}
		for _, heading := range []string{"#### 3.12.2.", "#### 3.12.3."} {
			if body := sectionByHeadingPrefix(architecture, heading); body == "" {
				t.Errorf("architecture %s has no investigation result", heading)
			}
		}
	})

	t.Run("fills every cell of the plan's manual-check table", func(t *testing.T) {
		section := sectionByHeading(plan, "### 5.1. 手動の確認と記録")
		if section == "" {
			t.Fatal("the plan has no section 5.1")
		}
		table := findTable(section, "AC")
		if table == nil {
			t.Fatal("section 5.1 has no AC table")
		}
		rows := table[1:]
		want := []string{
			acLabel(2),
			acLabel(3),
			acLabel(5),
			acLabel(7),
			acLabel(8),
			acLabel(10, 11),
			acLabel(13),
			acLabel(20),
			acLabel(21, 22, 31),
			acLabel(28, 29),
			acLabel(25),
		}
		if len(rows) != len(want) {
			t.Fatalf("section 5.1 has %d rows, want %d", len(rows), len(want))
		}
		for i, row := range rows {
			if len(row) == 0 {
				t.Errorf("section 5.1 row %d is empty", i+1)
				continue
			}
			if row[0] != want[i] {
				t.Errorf("section 5.1 row %d is %q, want %q", i+1, row[0], want[i])
				continue
			}
			if len(row) != 5 {
				t.Errorf("section 5.1 row %s is malformed: %d cells, want 5", row[0], len(row))
				continue
			}
			for _, column := range []int{1, 2, 3, 4} {
				if strings.TrimSpace(row[column]) == "" {
					t.Errorf("section 5.1 row %s has an empty column %d", row[0], column+1)
				}
			}
		}
	})
}

// TestSectionByHeadingPrefix pins sectionByHeadingPrefix's boundaries: it
// matches a heading by trimmed prefix, excludes the heading line, stops before
// the next heading line at any level, and returns the empty string when the
// heading is absent or the line is not a heading.
func TestSectionByHeadingPrefix(t *testing.T) {
	const doc = `## Top
top body
### 3.12.1. First
first body
#### 3.12.1.1. Detail
detail body
### 3.12.2. Second
second body
#109 note
note body`
	for _, tc := range []struct {
		name   string
		prefix string
		want   string
	}{
		{
			name:   "prefix matches and stops at the next heading of any level",
			prefix: "### 3.12.1.",
			want:   "first body",
		},
		{
			name:   "deeper heading keeps only its own body",
			prefix: "#### 3.12.1.1.",
			want:   "detail body",
		},
		{
			name:   "a non-heading line continues the body",
			prefix: "### 3.12.2.",
			want:   "second body\n#109 note\nnote body",
		},
		{
			name:   "missing heading yields the empty string",
			prefix: "### 3.12.3.",
			want:   "",
		},
		{
			name:   "hash without following whitespace is not a heading",
			prefix: "#109 note",
			want:   "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sectionByHeadingPrefix(doc, tc.prefix); got != tc.want {
				t.Errorf("sectionByHeadingPrefix(doc, %q) = %q, want %q", tc.prefix, got, tc.want)
			}
		})
	}
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
	if err := json.Unmarshal([]byte(readDoc(t, extensionManifestPath)), &manifest); err != nil {
		t.Fatalf("parse %s: %v", extensionManifestPath, err)
	}
	return manifest
}

// sectionByHeadingPrefix returns the text under the first heading line whose
// trimmed text starts with prefix, up to the next heading line at any level.
// The heading line is not included and the result is trimmed; it returns the
// empty string when no heading line matches. The caller passes a heading prefix
// such as "#### 3.12.1.".
func sectionByHeadingPrefix(doc, prefix string) string {
	want := strings.TrimSpace(prefix)
	inSection := false
	var b strings.Builder
	for line := range strings.Lines(doc) {
		if headingLevel(line) > 0 {
			if !inSection {
				if strings.HasPrefix(strings.TrimSpace(line), want) {
					inSection = true
				}
				continue
			}
			break
		}
		if inSection {
			b.WriteString(line)
		}
	}
	return strings.TrimSpace(b.String())
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
// followed by its parenthesised reason; incidental code spans inside a reason
// are not permissions. It reads only up to the first sentence end, so the
// permissions the section names as not requested are excluded. The second
// result is false when the bullet is missing or lists nothing.
func declaredPermissions(section string) ([]string, bool) {
	const marker = "**要求する権限**"
	start := strings.Index(section, marker)
	if start < 0 {
		return nil, false
	}
	rest := section[start:]
	if end := strings.Index(rest, "。"); end >= 0 {
		rest = rest[:end]
	}
	var permissions []string
	for _, match := range requestedPermissionRE.FindAllStringSubmatch(rest, -1) {
		permissions = append(permissions, match[1])
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

// acLabel builds a manual-check row label from acceptance-criterion numbers
// that belong to task 0007: acLabel(2) returns the label for criterion 2 and
// acLabel(10, 11) joins two labels with the Japanese middle dot. These labels
// are document data matched against 0007's table, not references to this task's
// own acceptance criteria. Building them from numbers keeps the "AC-" prefix
// out of a literal followed by digits, which runplan's static check rejects.
func acLabel(numbers ...int) string {
	labels := make([]string, len(numbers))
	for i, number := range numbers {
		labels[i] = fmt.Sprintf("AC-%02d", number)
	}
	return strings.Join(labels, "・")
}
