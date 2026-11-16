//go:build test

package llmhttptest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeGo writes one Go file into a new directory and returns the directory.
func writeGo(t *testing.T, name, source string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return dir
}

func TestParseDirCalls(t *testing.T) {
	dir := writeGo(t, "a.go", `package a

func direct() { target() }
func selector(x T) { x.target() }
func inClosure() { func() { target() }() }
func reference() { f := target; _ = f }
func other() { different() }
func otherSelector(x T) { x.different() }
`)
	funcs := ParseDir(t, dir)
	want := []string{"direct", "inClosure", "selector"}
	if got := Callers(funcs, "target"); !slices.Equal(got, want) {
		t.Errorf("Callers(target) = %v, want %v (a reference without a call must not count)", got, want)
	}
}

func TestParseDirSkipsTestFilesAndReadsConstrainedFiles(t *testing.T) {
	dir := t.TempDir()
	for name, source := range map[string]string{
		"x_js.go":   "//go:build js\n\npackage a\n\nfunc onlyJS() {}\n",
		"x_test.go": "package a\n\nfunc inTest() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	funcs := ParseDir(t, dir)
	if len(Named(funcs, "onlyJS")) != 1 {
		t.Error("a file that only a js build compiles was not read")
	}
	if len(Named(funcs, "inTest")) != 0 {
		t.Error("a _test.go file was read")
	}
}

func TestSetsHeader(t *testing.T) {
	dir := writeGo(t, "a.go", `package a

import "net/http"

func match(r *http.Request)        { r.Header.Set("Name", "Value") }
func wrongValue(r *http.Request)   { r.Header.Set("Name", "Other") }
func wrongName(r *http.Request)    { r.Header.Set("Other", "Value") }
func notHeader(r *http.Request)    { r.Set("Name", "Value") }
func notSet(r *http.Request)       { r.Header.Add("Name", "Value") }
func notLiteral(r *http.Request, v string) { r.Header.Set("Name", v) }
`)
	funcs := ParseDir(t, dir)
	want := map[string]bool{
		"match": true, "wrongValue": false, "wrongName": false,
		"notHeader": false, "notSet": false, "notLiteral": false,
	}
	for name, wantSet := range want {
		decls := Named(funcs, name)
		if len(decls) != 1 {
			t.Fatalf("declarations of %s = %d, want 1", name, len(decls))
		}
		if got := decls[0].SetsHeader("Name", "Value"); got != wantSet {
			t.Errorf("%s: SetsHeader(Name, Value) = %t, want %t", name, got, wantSet)
		}
	}
}

func TestTopLevelCallAndFirstStatementCalling(t *testing.T) {
	dir := writeGo(t, "a.go", `package a

func inOrder() {
	setup()
	_, _ = x.send()
}

func reversed() {
	_, _ = x.send()
	setup()
}

func conditional(ok bool) {
	if ok {
		setup()
	}
	_, _ = x.send()
}

func inClosure() {
	func() { setup() }()
	_, _ = x.send()
}

func neverSends() { setup() }

func sendsInsideAnIf(ok bool) {
	setup()
	if ok {
		x.send()
	}
}
`)
	funcs := ParseDir(t, dir)
	cases := []struct {
		name      string
		wantSetup int
		wantSend  int
	}{
		{name: "inOrder", wantSetup: 0, wantSend: 1},
		{name: "reversed", wantSetup: 1, wantSend: 0},
		{name: "conditional", wantSetup: noIndex, wantSend: 1},
		{name: "inClosure", wantSetup: noIndex, wantSend: 1},
		{name: "neverSends", wantSetup: 0, wantSend: noIndex},
		{name: "sendsInsideAnIf", wantSetup: 0, wantSend: 1},
	}
	for _, tc := range cases {
		decls := Named(funcs, tc.name)
		if len(decls) != 1 {
			t.Fatalf("declarations of %s = %d, want 1", tc.name, len(decls))
		}
		if got := decls[0].TopLevelCall("setup"); got != tc.wantSetup {
			t.Errorf("%s: TopLevelCall(setup) = %d, want %d", tc.name, got, tc.wantSetup)
		}
		if got := decls[0].FirstStatementCalling("send"); got != tc.wantSend {
			t.Errorf("%s: FirstStatementCalling(send) = %d, want %d", tc.name, got, tc.wantSend)
		}
	}
}
