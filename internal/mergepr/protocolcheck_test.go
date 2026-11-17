//go:build test

package mergepr

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// protocolSource returns a tool file that declares the number n. Declaration
// lines are built only in _test.go files, which the check never reads.
func protocolSource(n string) string {
	return "package tool\n\nconst Protocol = " + n + "\n"
}

func toolPath(name string) string {
	return "internal/" + name + "/" + name + ".go"
}

// snapshot returns the files of a commit: a README, one file per tool
// declaring its number, and extra path/content pairs.
func snapshot(tools map[string]string, extra ...string) map[string]string {
	files := map[string]string{"README.md": "readme\n"}
	for name, n := range tools {
		files[toolPath(name)] = protocolSource(n)
	}
	for i := 0; i+1 < len(extra); i += 2 {
		files[extra[i]] = extra[i+1]
	}
	return files
}

func present(n int) protocolValue { return protocolValue{State: protocolPresent, Value: n} }

var (
	absent     = protocolValue{State: protocolAbsent}
	unreadable = protocolValue{State: protocolUnreadable}
)

type wantReading struct {
	value protocolValue
	kind  unreadableKind
}

func TestClassifyProtocolLines(t *testing.T) {
	one := func(text string) []protocolLine { return []protocolLine{{Path: "internal/alpha/alpha.go", Text: text}} }
	val := func(n int) map[string]wantReading { return map[string]wantReading{"alpha": {value: present(n)}} }
	bad := map[string]wantReading{"alpha": {value: unreadable, kind: unreadableValue}}
	dup := map[string]wantReading{"alpha": {value: unreadable, kind: unreadableDuplicate}}
	none := map[string]wantReading{}
	tests := []struct {
		name  string
		lines []protocolLine
		want  map[string]wantReading
	}{
		{"accepted", one("const Protocol = 1"), val(1)},
		{"nine digits", one("const Protocol = 999999999"), val(999999999)},
		{"ten digits", one("const Protocol = 1000000000"), bad},
		{"leading zero", one("const Protocol = 01"), bad},
		{"zero", one("const Protocol = 0"), bad},
		{"negative", one("const Protocol = -1"), bad},
		{"underscore", one("const Protocol = 1_0"), bad},
		{"hexadecimal", one("const Protocol = 0x1"), bad},
		{"typed", one("const Protocol int = 1"), bad},
		{"in a const block", one("\tProtocol = 1"), bad},
		{"two names", one("const Protocol, Other = 1, 2"), bad},
		{"trailing comment", one("const Protocol = 1 // note"), bad},
		{"trailing carriage return", one("const Protocol = 1\r"), bad},
		{"two spaces", one("const  Protocol = 1"), bad},
		{"struct field", one("\tProtocol int `json:\"protocol\"`"), none},
		{"struct tag holding =", one("\tProtocol int `x:\"a=b\"`"), none},
		{"quoted tag holding =", one("\tProtocol int \"a=b\""), none},
		{"composite literal element", one("\t\tProtocol: Protocol,"), none},
		{"short variable declaration", one("\tProtocol := 1"), none},
		{"equality", one("\tProtocol == 1"), none},
		{"inequality", one("\tProtocol != 1"), none},
		{"less or equal", one("\tProtocol <= 1"), none},
		{"greater or equal", one("\tProtocol >= 1"), none},
		{"comment holding =", one("\tProtocol int // a = b"), none},
		{"longer identifier", one("const ProtocolVersion = 2"), none},
		{"word inside a line", one("\treturn x.Protocol = 1"), none},
		{"two lines in one file", []protocolLine{
			{Path: "internal/alpha/alpha.go", Text: "const Protocol = 1"},
			{Path: "internal/alpha/alpha.go", Text: "const Protocol = 1"},
		}, dup},
		{"two lines in two files", []protocolLine{
			{Path: "internal/alpha/a.go", Text: "const Protocol = 1"},
			{Path: "internal/alpha/b.go", Text: "const Protocol = 2"},
		}, dup},
		{"two tools", []protocolLine{
			{Path: "internal/alpha/a.go", Text: "const Protocol = 1"},
			{Path: "internal/beta/b.go", Text: "const Protocol = 7"},
		}, map[string]wantReading{"alpha": {value: present(1)}, "beta": {value: present(7)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := classifyProtocolLines(tt.lines)
			if err != nil {
				t.Fatalf("classifyProtocolLines error = %v", err)
			}
			gotWant := make(map[string]wantReading, len(got))
			for pkg, r := range got {
				gotWant[pkg] = wantReading{value: r.value, kind: r.kind}
			}
			if !maps.Equal(gotWant, tt.want) {
				t.Errorf("classifyProtocolLines = %v, want %v", gotWant, tt.want)
			}
		})
	}
}

func TestClassifyProtocolLinesRejectsOtherPaths(t *testing.T) {
	for _, path := range []string{"internal/alpha/sub/s.go", "internal/alpha/alpha_test.go", "cmd/alpha/main.go", "internal/alpha.go", "internal/alpha/README.md"} {
		t.Run(path, func(t *testing.T) {
			_, err := classifyProtocolLines([]protocolLine{{Path: path, Text: "const Protocol = 1"}})
			if e, ok := errors.AsType[*protocolUnreadableError](err); !ok || e.Kind != unreadableCommits {
				t.Errorf("classifyProtocolLines error = %v, want an unreadableCommits error", err)
			}
		})
	}
}

func TestMismatchRemedy(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	for _, tool := range []string{"fixprguard", "codexreview", "chief", "mergepr"} {
		t.Run(tool, func(t *testing.T) {
			target := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(installCommand(tool)[len("make "):]) + `:`)
			if !target.Match(makefile) {
				t.Errorf("Makefile has no target for %q", installCommand(tool))
			}
			// The message is what the operator follows, so the command it
			// shows is the observable contract here.
			e := &protocolMismatchError{Mismatches: []protocolMismatch{{Package: tool, Main: present(1), Head: 3}}}
			if !regexp.MustCompile(regexp.QuoteMeta(installCommand(tool)) + `\b`).MatchString(e.Error()) {
				t.Errorf("mismatch error does not show %q:\n%s", installCommand(tool), e.Error())
			}
		})
	}
	rewrites := []struct {
		name string
		m    protocolMismatch
		want bool
	}{
		{"head equals origin/main", protocolMismatch{Main: present(3), Head: 3}, true},
		{"head above the expected value", protocolMismatch{Main: present(3), Head: 5}, false},
		{"head below origin/main", protocolMismatch{Main: present(3), Head: 2}, false},
		{"new tool", protocolMismatch{Main: absent, Head: 2}, false},
	}
	for _, tt := range rewrites {
		t.Run(tt.name, func(t *testing.T) {
			if got := offersHistoryRewrite(tt.m); got != tt.want {
				t.Errorf("offersHistoryRewrite = %v, want %v", got, tt.want)
			}
		})
	}
}

// readValues reads rev with the production check built in dir and returns
// each tool's value.
func readValues(t *testing.T, dir, rev string) map[string]protocolValue {
	t.Helper()
	readings, err := newProtocolCheck(newGitRunner(dir)).readRevision(t.Context(), rev)
	if err != nil {
		t.Fatalf("readRevision(%s) error = %v", rev, err)
	}
	values := make(map[string]protocolValue, len(readings))
	for pkg, r := range readings {
		values[pkg] = r.value
	}
	return values
}

func TestReadRevisionProtocols(t *testing.T) {
	r := newGitRepo(t)
	const accented = "caf\u00e9"
	head := r.Commit("tools", map[string]string{
		toolPath("alpha"):                  protocolSource("1"),
		"internal/alpha/sub/sub.go":        protocolSource("9"),
		"internal/alpha/alpha_test.go":     protocolSource("2"),
		"internal/beta/beta_tag.go":        "//go:build test\n\n" + protocolSource("4"),
		"internal/" + accented + "/x.go":   protocolSource("3"),
		"internal/gamma/gamma.go":          "package gamma\n\nvar x = T{\n\tProtocol: Protocol,\n}\n",
		"cmd/delta/main.go":                protocolSource("5"),
		"internal/epsilon/testdata/fix.go": protocolSource("6"),
	})
	want := map[string]protocolValue{"alpha": present(1), "beta": present(4), accented: present(3)}
	other := r.Commit("other", snapshot(map[string]string{"alpha": "8"}))
	if got := readValues(t, r.Dir, head); !maps.Equal(got, want) {
		t.Fatalf("baseline = %v, want %v", got, want)
	}

	t.Run("no declaration line", func(t *testing.T) {
		bare := r.withT(t).Commit("bare", map[string]string{"README.md": "x\n", "internal/alpha/alpha.go": "package alpha\n"})
		if got := readValues(t, r.Dir, bare); len(got) != 0 {
			t.Errorf("values = %v, want none", got)
		}
	})
	t.Run("missing revision", func(t *testing.T) {
		_, err := newProtocolCheck(newGitRunner(r.Dir)).readRevision(t.Context(), "0123456789abcdef0123456789abcdef01234567")
		if err == nil {
			t.Error("readRevision error = nil, want a failure, not an empty result")
		}
	})
	t.Run("user configuration", func(t *testing.T) {
		config := "[grep]\n\tlineNumber = false\n\tfullName = false\n\tcolumn = true\n\tpatternType = perl\n\textendedRegexp = true\n" +
			"[color]\n\tui = always\n\tgrep = always\n[core]\n\tquotePath = true\n"
		path := filepath.Join(r.Home, ".gitconfig")
		if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
			t.Fatalf("write .gitconfig: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })
		if got := readValues(t, r.Dir, head); !maps.Equal(got, want) {
			t.Errorf("values = %v, want %v", got, want)
		}
	})
	t.Run("replace ref", func(t *testing.T) {
		rt := r.withT(t)
		rt.Git("replace", head, other)
		t.Cleanup(func() { rt.Git("replace", "-d", head) })
		if got := readValues(t, r.Dir, head); !maps.Equal(got, want) {
			t.Errorf("values = %v, want %v", got, want)
		}
	})
	t.Run("GIT_DIR", func(t *testing.T) {
		elsewhere := newGitRepo(t)
		elsewhere.Commit("elsewhere", snapshot(map[string]string{"alpha": "8"}))
		t.Setenv("GIT_DIR", filepath.Join(elsewhere.Dir, ".git"))
		if got := readValues(t, r.Dir, head); !maps.Equal(got, want) {
			t.Errorf("values = %v, want %v", got, want)
		}
	})
	t.Run("subdirectory", func(t *testing.T) {
		sub := filepath.Join(r.Dir, "internal", "nested")
		if err := os.MkdirAll(sub, 0o750); err != nil {
			t.Fatalf("create subdirectory: %v", err)
		}
		if got := readValues(t, sub, head); !maps.Equal(got, want) {
			t.Errorf("values = %v, want %v", got, want)
		}
	})
}

// wantMismatch is one expected mismatch: the tool, origin/main's value, the
// head value, and the commits that changed it.
type wantMismatch struct {
	pkg     string
	main    protocolValue
	head    int
	commits []string
}

// historyWant is a built history and the expected result: a pass with the
// changed tools when mismatches is nil.
type historyWant struct {
	head, base string
	changed    []string
	mismatches []wantMismatch
}

func TestCheckProtocolHistories(t *testing.T) {
	baseTools := map[string]string{"alpha": "1", "beta": "6"}
	// start makes origin/main a commit with alpha 1 and beta 6.
	start := func(r *gitRepo) string {
		m0 := r.Commit("main", snapshot(baseTools))
		r.SetMain(m0)
		return m0
	}
	pass := func(head string, changed ...string) historyWant {
		return historyWant{head: head, base: mainBranch, changed: changed}
	}
	fail := func(head string, ms ...wantMismatch) historyWant {
		return historyWant{head: head, base: mainBranch, mismatches: ms}
	}
	tests := []struct {
		name  string
		build func(r *gitRepo) historyWant
	}{
		{"untouched PR", func(r *gitRepo) historyWant {
			m0 := start(r)
			return pass(r.Commit("docs", snapshot(baseTools, "docs/a.md", "a\n"), m0))
		}},
		{"raised by one", func(r *gitRepo) historyWant {
			m0 := start(r)
			return pass(r.Commit("raise", snapshot(map[string]string{"alpha": "2", "beta": "6"}), m0), "alpha")
		}},
		{"raised by two", func(r *gitRepo) historyWant {
			m0 := start(r)
			p1 := r.Commit("raise", snapshot(map[string]string{"alpha": "3", "beta": "6"}), m0)
			return fail(p1, wantMismatch{"alpha", present(1), 3, []string{p1}})
		}},
		{"lowered", func(r *gitRepo) historyWant {
			m0 := r.Commit("main", snapshot(map[string]string{"alpha": "5"}))
			r.SetMain(m0)
			p1 := r.Commit("lower", snapshot(map[string]string{"alpha": "3"}), m0)
			return fail(p1, wantMismatch{"alpha", present(5), 3, []string{p1}})
		}},
		{"raised and restored", func(r *gitRepo) historyWant {
			m0 := start(r)
			p1 := r.Commit("raise", snapshot(map[string]string{"alpha": "2", "beta": "6"}), m0)
			p2 := r.Commit("restore", snapshot(baseTools), p1)
			return fail(p2, wantMismatch{"alpha", present(1), 1, []string{p1, p2}})
		}},
		{"same number merged first", func(r *gitRepo) historyWant {
			m0 := start(r)
			raised := snapshot(map[string]string{"alpha": "2", "beta": "6"})
			m1 := r.Commit("A raises alpha", raised, m0)
			r.SetMain(m1)
			b1 := r.Commit("B raises alpha", raised, m0)
			b2 := r.Commit("merge main", raised, b1, m1)
			return fail(b2, wantMismatch{"alpha", present(2), 2, []string{b1}})
		}},
		{"merged main only", func(r *gitRepo) historyWant {
			m0 := start(r)
			m1 := r.Commit("A raises alpha", snapshot(map[string]string{"alpha": "2", "beta": "6"}), m0)
			r.SetMain(m1)
			c1 := r.Commit("docs", snapshot(baseTools, "docs/a.md", "a\n"), m0)
			return pass(r.Commit("merge main", snapshot(map[string]string{"alpha": "2", "beta": "6"}, "docs/a.md", "a\n"), c1, m1))
		}},
		{"new tool", func(r *gitRepo) historyWant {
			m0 := start(r)
			return pass(r.Commit("add gamma", snapshot(map[string]string{"alpha": "1", "beta": "6", "gamma": "1"}), m0), "gamma")
		}},
		{"new tool not at one", func(r *gitRepo) historyWant {
			m0 := start(r)
			p1 := r.Commit("add gamma", snapshot(map[string]string{"alpha": "1", "beta": "6", "gamma": "2"}), m0)
			return fail(p1, wantMismatch{"gamma", absent, 2, []string{p1}})
		}},
		{"existing package gains the constant", func(r *gitRepo) historyWant {
			m0 := r.Commit("main", snapshot(baseTools, "internal/delta/delta.go", "package delta\n"))
			r.SetMain(m0)
			return pass(r.Commit("add protocol", snapshot(baseTools, "internal/delta/delta.go", "package delta\n", "internal/delta/protocol.go", protocolSource("1")), m0), "delta")
		}},
		{"existing package gains the constant not at one", func(r *gitRepo) historyWant {
			m0 := r.Commit("main", snapshot(baseTools, "internal/delta/delta.go", "package delta\n"))
			r.SetMain(m0)
			p1 := r.Commit("add protocol", snapshot(baseTools, "internal/delta/delta.go", "package delta\n", "internal/delta/protocol.go", protocolSource("4")), m0)
			return fail(p1, wantMismatch{"delta", absent, 4, []string{p1}})
		}},
		{"re-added tool", func(r *gitRepo) historyWant {
			m0 := r.Commit("main", snapshot(map[string]string{"alpha": "1", "gamma": "2"}))
			m1 := r.Commit("remove gamma", snapshot(map[string]string{"alpha": "1"}), m0)
			r.SetMain(m1)
			return pass(r.Commit("re-add gamma", snapshot(map[string]string{"alpha": "1", "gamma": "1"}), m1), "gamma")
		}},
		{"re-added tool at its old number plus one", func(r *gitRepo) historyWant {
			m0 := r.Commit("main", snapshot(map[string]string{"alpha": "1", "gamma": "2"}))
			m1 := r.Commit("remove gamma", snapshot(map[string]string{"alpha": "1"}), m0)
			r.SetMain(m1)
			p1 := r.Commit("re-add gamma", snapshot(map[string]string{"alpha": "1", "gamma": "3"}), m1)
			return fail(p1, wantMismatch{"gamma", absent, 3, []string{p1}})
		}},
		{"removed tool", func(r *gitRepo) historyWant {
			m0 := start(r)
			return pass(r.Commit("remove beta", snapshot(map[string]string{"alpha": "1"}), m0), "beta")
		}},
		{"removed tool next to a mismatch", func(r *gitRepo) historyWant {
			m0 := start(r)
			p1 := r.Commit("remove beta, raise alpha", snapshot(map[string]string{"alpha": "3"}), m0)
			return fail(p1, wantMismatch{"alpha", present(1), 3, []string{p1}})
		}},
		{"branch older than the tool", func(r *gitRepo) historyWant {
			m0 := r.Commit("main", snapshot(map[string]string{"alpha": "1"}))
			m1 := r.Commit("add gamma", snapshot(map[string]string{"alpha": "1", "gamma": "1"}), m0)
			r.SetMain(m1)
			return pass(r.Commit("docs", snapshot(map[string]string{"alpha": "1"}, "docs/a.md", "a\n"), m0))
		}},
		{"local branch named origin/main", func(r *gitRepo) historyWant {
			m0 := start(r)
			raised := snapshot(map[string]string{"alpha": "2", "beta": "6"})
			decoy := r.Commit("decoy", raised, m0)
			r.Git("update-ref", "refs/heads/origin/main", decoy)
			return pass(r.Commit("raise", raised, m0), "alpha")
		}},
		{"two tools mismatch", func(r *gitRepo) historyWant {
			m0 := start(r)
			p1 := r.Commit("raise both", snapshot(map[string]string{"alpha": "3", "beta": "8"}), m0)
			return fail(p1,
				wantMismatch{"alpha", present(1), 3, []string{p1}},
				wantMismatch{"beta", present(6), 8, []string{p1}})
		}},
		{"unreadable midway then raised", func(r *gitRepo) historyWant {
			m0 := start(r)
			p1 := r.Commit("typo", snapshot(map[string]string{"alpha": "02", "beta": "6"}), m0)
			return pass(r.Commit("fix", snapshot(map[string]string{"alpha": "2", "beta": "6"}), p1), "alpha")
		}},
		{"unreadable midway then restored", func(r *gitRepo) historyWant {
			m0 := start(r)
			p1 := r.Commit("typo", snapshot(map[string]string{"alpha": "02", "beta": "6"}), m0)
			p2 := r.Commit("restore", snapshot(baseTools), p1)
			return fail(p2, wantMismatch{"alpha", present(1), 1, []string{p1, p2}})
		}},
		{"unreadable value rewritten on a branch from an old main", func(r *gitRepo) historyWant {
			// An old main commit held an unreadable value; the branch rewrote
			// it to another unreadable form, then merged main's fix. The only
			// change is unreadable to unreadable, which still counts.
			m0 := r.Commit("old main", snapshot(map[string]string{"alpha": "02"}))
			m1 := r.Commit("main fixes alpha", snapshot(map[string]string{"alpha": "1"}), m0)
			r.SetMain(m1)
			p1 := r.Commit("rewrite", snapshot(map[string]string{"alpha": "03"}), m0)
			p2 := r.Commit("merge main", snapshot(map[string]string{"alpha": "1"}), p1, m1)
			return fail(p2, wantMismatch{"alpha", present(1), 1, []string{p1}})
		}},
		{"commits with empty messages", func(r *gitRepo) historyWant {
			// git rev-list prints no line for an empty subject in a plain
			// %s format; the second empty commit raises the number.
			m0 := start(r)
			p1 := r.Commit("", snapshot(baseTools, "docs/a.md", "a\n"), m0)
			p2 := r.Commit("", snapshot(map[string]string{"alpha": "3", "beta": "6"}, "docs/a.md", "a\n"), p1)
			p3 := r.Commit("docs", snapshot(map[string]string{"alpha": "3", "beta": "6"}, "docs/a.md", "b\n"), p2)
			return fail(p3, wantMismatch{"alpha", present(1), 3, []string{p2}})
		}},
		{"root commit merged", func(r *gitRepo) historyWant {
			m0 := start(r)
			root := r.Commit("unrelated", map[string]string{toolPath("alpha"): protocolSource("2")})
			b1 := r.Commit("docs", snapshot(baseTools, "docs/a.md", "a\n"), m0)
			return pass(r.Commit("merge unrelated", snapshot(map[string]string{"alpha": "2", "beta": "6"}, "docs/a.md", "a\n"), b1, root), "alpha")
		}},
		{"other base untouched", func(r *gitRepo) historyWant {
			m0 := start(r)
			w := pass(r.Commit("docs", snapshot(baseTools, "docs/a.md", "a\n"), m0))
			w.base = "release"
			return w
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newGitRepo(t)
			want := tt.build(r)
			mainOID := r.Git("rev-parse", testMainRef)
			summary, err := r.Check("", want.head, want.base)
			if want.mismatches == nil {
				if err != nil {
					t.Fatalf("check error = %v, want a pass", err)
				}
				if !slices.Equal(summary.Changed, want.changed) {
					t.Errorf("Changed = %v, want %v", summary.Changed, want.changed)
				}
				return
			}
			if !errors.Is(err, errProtocolMismatch) {
				t.Fatalf("check error = %v, want errProtocolMismatch", err)
			}
			e, ok := errors.AsType[*protocolMismatchError](err)
			if !ok {
				t.Fatalf("check error %T is not *protocolMismatchError", err)
			}
			if e.MainOID != mainOID || e.HeadOID != want.head {
				t.Errorf("compared %s with %s, want %s with %s", e.MainOID, e.HeadOID, mainOID, want.head)
			}
			if len(e.Mismatches) != len(want.mismatches) {
				t.Fatalf("mismatches = %+v, want %d", e.Mismatches, len(want.mismatches))
			}
			for i, w := range want.mismatches {
				got := e.Mismatches[i]
				var commits []string
				for _, c := range got.Changes {
					commits = append(commits, c.Commit)
				}
				if got.Package != w.pkg || got.Main != w.main || got.Head != w.head || !slices.Equal(commits, w.commits) {
					t.Errorf("mismatch %d = {%s %v %d %v}, want {%s %v %d %v}",
						i, got.Package, got.Main, got.Head, commits, w.pkg, w.main, w.head, w.commits)
				}
			}
		})
	}
}

func TestCheckProtocolSummary(t *testing.T) {
	r := newGitRepo(t)
	m0 := r.Commit("main", snapshot(map[string]string{"alpha": "1", "beta": "6"}))
	r.SetMain(m0)
	head := r.Commit("raise", snapshot(map[string]string{"alpha": "2", "beta": "6"}), m0)
	summary, err := r.Check("", head, mainBranch)
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	wantTools := map[string]protocolValue{"alpha": present(1), "beta": present(6)}
	if summary.MainOID != m0 || summary.HeadOID != head || !maps.Equal(summary.MainTools, wantTools) || !slices.Equal(summary.Changed, []string{"alpha"}) {
		t.Errorf("summary = %+v, want main %s head %s tools %v changed [alpha]", summary, m0, head, wantTools)
	}
}

func TestCheckProtocolUnreadable(t *testing.T) {
	baseTools := map[string]string{"alpha": "1"}
	tests := []struct {
		name string
		// build returns the directory to run in (the repository when
		// empty), the head, and the base.
		build func(r *gitRepo) (dir, head, base string)
		want  unreadableKind
	}{
		{"head not in the repository", func(r *gitRepo) (string, string, string) {
			r.SetMain(r.Commit("main", snapshot(baseTools)))
			return "", "0123456789abcdef0123456789abcdef01234567", mainBranch
		}, unreadableCommits},
		{"head not a commit ID", func(r *gitRepo) (string, string, string) {
			r.SetMain(r.Commit("main", snapshot(baseTools)))
			return "", "HEAD", mainBranch
		}, unreadableCommits},
		{"no origin/main", func(r *gitRepo) (string, string, string) {
			return "", r.Commit("main", snapshot(baseTools)), mainBranch
		}, unreadableCommits},
		{"changed but gone from the head", func(r *gitRepo) (string, string, string) {
			m0 := r.Commit("main", snapshot(baseTools))
			m1 := r.Commit("docs", snapshot(baseTools, "docs/a.md", "a\n"), m0)
			r.SetMain(m1)
			b1 := r.Commit("add gamma", snapshot(map[string]string{"alpha": "1", "gamma": "1"}), m0)
			// The merge resolves the conflict on gamma as absent.
			return "", r.Commit("merge main", snapshot(baseTools, "docs/a.md", "a\n"), b1, m1), mainBranch
		}, unreadableHeadMissing},
		{"head value not accepted", func(r *gitRepo) (string, string, string) {
			m0 := r.Commit("main", snapshot(baseTools))
			r.SetMain(m0)
			return "", r.Commit("typo", snapshot(map[string]string{"alpha": "02"}), m0), mainBranch
		}, unreadableValue},
		{"origin/main value not accepted on an untouched PR", func(r *gitRepo) (string, string, string) {
			m0 := r.Commit("main", snapshot(map[string]string{"alpha": "1 // note"}))
			r.SetMain(m0)
			return "", r.Commit("docs", snapshot(map[string]string{"alpha": "1 // note"}, "docs/a.md", "a\n"), m0), mainBranch
		}, unreadableValue},
		{"two declarations at the head", func(r *gitRepo) (string, string, string) {
			m0 := r.Commit("main", snapshot(baseTools))
			r.SetMain(m0)
			return "", r.Commit("dup", snapshot(map[string]string{"alpha": "2"}, "internal/alpha/more.go", protocolSource("2")), m0), mainBranch
		}, unreadableDuplicate},
		{"other base with a change", func(r *gitRepo) (string, string, string) {
			m0 := r.Commit("main", snapshot(baseTools))
			r.SetMain(m0)
			return "", r.Commit("raise", snapshot(map[string]string{"alpha": "2"}), m0), "release"
		}, unreadableBase},
		{"number changed on the base branch", func(r *gitRepo) (string, string, string) {
			// The release branch raised the number; the PR into it does not
			// touch it, but its commits are counted from origin/main.
			m0 := r.Commit("main", snapshot(baseTools))
			r.SetMain(m0)
			rel := r.Commit("release raises alpha", snapshot(map[string]string{"alpha": "2"}), m0)
			return "", r.Commit("docs", snapshot(map[string]string{"alpha": "2"}, "docs/a.md", "a\n"), rel), "release"
		}, unreadableBase},
		{"shallow clone", func(r *gitRepo) (string, string, string) {
			m0 := r.Commit("main", snapshot(baseTools))
			m1 := r.Commit("raise", snapshot(map[string]string{"alpha": "2"}), m0)
			return r.ShallowClone(m1), m1, mainBranch
		}, unreadableHistory},
		{"parent object missing", func(r *gitRepo) (string, string, string) {
			m0 := r.Commit("main", snapshot(baseTools))
			r.SetMain(m0)
			p1 := r.Commit("raise", snapshot(map[string]string{"alpha": "2"}), m0)
			p2 := r.Commit("docs", snapshot(map[string]string{"alpha": "2"}, "docs/a.md", "a\n"), p1)
			r.RemoveObject(p1)
			return "", p2, mainBranch
		}, unreadableHistory},
		{"no tool on origin/main", func(r *gitRepo) (string, string, string) {
			m0 := r.Commit("main", map[string]string{"README.md": "x\n"})
			r.SetMain(m0)
			return "", r.Commit("docs", map[string]string{"README.md": "y\n"}, m0), mainBranch
		}, unreadableNoTools},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newGitRepo(t)
			dir, head, base := tt.build(r)
			_, err := r.Check(dir, head, base)
			if !errors.Is(err, errProtocolUnreadable) {
				t.Fatalf("check error = %v, want errProtocolUnreadable", err)
			}
			if e, ok := errors.AsType[*protocolUnreadableError](err); !ok || e.Kind != tt.want {
				t.Errorf("check error = %#v, want kind %v", err, tt.want)
			}
		})
	}
}

const (
	parseRev    = "1111111111111111111111111111111111111111"
	parseParent = "2222222222222222222222222222222222222222"
)

func TestParseGrepOutputRejectsMalformed(t *testing.T) {
	tests := []struct {
		name string
		out  string
	}{
		{"empty", ""},
		{"no final newline", parseRev + ":internal/a/a.go\x001\x00const Protocol = 1"},
		{"other revision", parseParent + ":internal/a/a.go\x001\x00const Protocol = 1\n"},
		{"no path separator", parseRev + ":internal/a/a.go 1 const Protocol = 1\n"},
		{"no line number", parseRev + ":internal/a/a.go\x00const Protocol = 1\n"},
		{"line number not digits", parseRev + ":internal/a/a.go\x001a\x00const Protocol = 1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseGrepOutput([]byte(tt.out), parseRev); !errors.Is(err, errUnexpectedGitOutput) {
				t.Errorf("parseGrepOutput error = %v, want errUnexpectedGitOutput", err)
			}
		})
	}
}

func TestParseRevList(t *testing.T) {
	good := "commit " + parseRev + " " + parseParent + "\n\x00subject\n"
	got, err := parseRevList([]byte(good))
	if err != nil || len(got) != 1 || got[0].oid != parseRev || !slices.Equal(got[0].parents, []string{parseParent}) || got[0].subject != "subject" {
		t.Fatalf("parseRevList = %+v, %v; want one commit with its parent and subject", got, err)
	}
	tests := []struct {
		name string
		out  string
	}{
		{"no final newline", "commit " + parseRev + "\n\x00s"},
		{"odd line count", "commit " + parseRev + "\n\x00s\ncommit " + parseParent + "\n"},
		{"subject line without NUL", "commit " + parseRev + "\ns\n"},
		{"header in the subject place", "commit " + parseRev + "\ncommit " + parseParent + "\n"},
		{"header without the commit word", parseRev + "\n\x00s\n"},
		{"abbreviated ID", "commit 1111111\n\x00s\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseRevList([]byte(tt.out)); !errors.Is(err, errUnexpectedGitOutput) {
				t.Errorf("parseRevList error = %v, want errUnexpectedGitOutput", err)
			}
		})
	}
}

// runnerFunc adapts a function to Runner.
type runnerFunc func() ([]byte, error)

func (f runnerFunc) Run(context.Context, string, ...string) ([]byte, error) { return f() }

func TestReadRevisionExitStatus(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr bool
	}{
		{"exit 1 without output means no line", &cmdrunner.ExitError{Code: 1}, false},
		{"exit 1 with output", &cmdrunner.ExitError{Code: 1, Stdout: []byte("x")}, true},
		{"exit 128", &cmdrunner.ExitError{Code: 128}, true},
		{"not an exit", errors.New("start failed"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newProtocolCheck(runnerFunc(func() ([]byte, error) { return nil, tt.err }))
			got, err := c.readRevision(t.Context(), parseRev)
			if (err != nil) != tt.wantErr || (!tt.wantErr && len(got) != 0) {
				t.Errorf("readRevision = %v, %v; want error %v", got, err, tt.wantErr)
			}
		})
	}
}

func TestProtocolErrorsRender(t *testing.T) {
	// Every kind has its own description; the zero kind is the only invalid one.
	for k := unreadableKind(1); k <= unreadableNoTools; k++ {
		if k.String() == unreadableKind(0).String() {
			t.Errorf("kind %d has no description", k)
		}
		e := &protocolUnreadableError{Kind: k, Rev: parseRev, Path: "internal/a/a.go", Line: "const Protocol = 01"}
		if !errors.Is(e, errProtocolUnreadable) {
			t.Errorf("kind %d does not wrap errProtocolUnreadable", k)
		}
	}
	m := &protocolMismatchError{Mismatches: []protocolMismatch{{Package: "alpha", Main: present(2), Head: 2}}}
	if !errors.Is(m, errProtocolMismatch) {
		t.Error("mismatch error does not wrap errProtocolMismatch")
	}
}
