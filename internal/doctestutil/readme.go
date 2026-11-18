//go:build test

package doctestutil

import (
	"path/filepath"
	"testing"
)

// Lang is the language of one version of a bilingual document. The zero
// value is not a language: a Locator asked for it fails the test.
type Lang int

// The languages of the README's two versions.
const (
	LangUnknown Lang = iota
	LangJA
	LangEN
)

// ReadmeVersion is one version of the README: its path from the repository
// root, with forward slashes, and its language.
type ReadmeVersion struct {
	Path string
	Lang Lang
}

// ReadmeVersions returns README.md (LangJA), the primary version, and
// README.en.md (LangEN), its English version, in that order. A guard test of a
// value that stays in the README checks it in each, naming the version's path
// in its failure.
func ReadmeVersions() []ReadmeVersion {
	return []ReadmeVersion{
		{Path: "README.md", Lang: LangJA},
		{Path: "README.en.md", Lang: LangEN},
	}
}

// From returns the version's path joined to root, the path from a test's
// package directory to the repository root (such as "../.."), so the test can
// read the file.
func (v ReadmeVersion) From(root string) string {
	return filepath.Join(root, filepath.FromSlash(v.Path))
}

// Locator is a README heading line, table column name, or item label that a
// guard test looks for, spelled once per language. Each spelling must equal
// the text of that version of the README.
type Locator struct {
	JA string
	EN string
}

// In returns the spelling for lang. It fails t for a language that is not
// LangJA or LangEN and for an empty spelling, so a missing translation fails
// instead of matching nothing silently; after reporting it returns "".
func (l Locator) In(t testing.TB, lang Lang) string {
	t.Helper()
	var spelling string
	switch lang {
	case LangJA:
		spelling = l.JA
	case LangEN:
		spelling = l.EN
	default:
		// The empty-spelling check below would also reject it; failing here
		// names the cause and keeps the switch failing secure on its own.
		t.Fatalf("locator %+v: no spelling for language %d", l, int(lang))
		return ""
	}
	if spelling == "" {
		t.Fatalf("locator %+v: empty spelling for language %d", l, int(lang))
		return ""
	}
	return spelling
}

// The README locators. A guard test finds a section, a table, or a labelled
// line of either version by these.
var (
	ReadmeRecommendedConfigHeading = Locator{JA: "### 推奨構成", EN: "### Recommended configuration"}

	// ReadmeVariableColumn and ReadmeUnsetColumn head the configuration
	// table's first and third columns.
	ReadmeVariableColumn = Locator{JA: "変数", EN: "Variable"}
	ReadmeUnsetColumn    = Locator{JA: "未設定のとき", EN: "When unset"}
	// ReadmeEffortProviderColumn and ReadmeEffortValuesColumn head the first
	// two columns of the configuration section's table of accepted effort
	// values. The values column finds the table: no other table has it.
	ReadmeEffortProviderColumn = Locator{JA: "プロバイダ", EN: "Provider"}
	ReadmeEffortValuesColumn   = Locator{JA: "受理する effort", EN: "Accepted efforts"}
)
