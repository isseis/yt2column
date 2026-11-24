//go:build test

package doctestutil

import "path/filepath"

// Lang is the language of one version of a bilingual document. The zero
// value is not a language.
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
