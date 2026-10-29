//go:build test

// Package doctestutil holds the Markdown helpers that the repository's guard
// tests share. A guard test checks the values and the structure of a document,
// so these helpers read a document from disk, scan its lines, headings, code
// fences, and tables, and derive the anchors GitHub gives its headings. Every
// file here is built only with -tags test: no production code uses it.
package doctestutil
