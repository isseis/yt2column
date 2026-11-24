//go:build test

package mergepr

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/isseis/yt2column/internal/cmdrunner"
)

// The Go sources of the take-in tests live here, not in a helper: the
// protocol-number check reads every non-test file of internal/mergepr.

// toolFile is a tool's file with number n and a body line far enough from
// the declaration that the two merge apart.
func toolFile(n int, body string) string {
	return "package tool\n\n// Protocol is the number.\nconst Protocol = " + strconv.Itoa(n) +
		"\n\n// one\n// two\n// three\n// four\n\nfunc Body() string { return \"" + body + "\" }\n"
}

// otherFile is a second tool's file.
func otherFile(n int) string {
	return "package other\n\nconst Protocol = " + strconv.Itoa(n) + "\n"
}

// renameSource is Markdown long enough for git to detect its rename.
func renameSource(tag string) string {
	var b strings.Builder
	for i := range 12 {
		b.WriteString(tag + " line " + strconv.Itoa(i) + "\n")
	}
	return b.String()
}

const (
	testTakeInPR    = "7"
	takeInToolPath  = "internal/tool/tool.go"
	takeInOtherPath = "internal/other/other.go"
)

// baseFiles is the merge base every take-in test starts from.
func baseFiles() map[string]string {
	return map[string]string{
		"README.md":                  "readme\n",
		"doc.md":                     "one\ntwo\nthree\n",
		"main.go":                    "package main\n\nfunc main() {}\n",
		takeInToolPath:               toolFile(3, "base"),
		takeInOtherPath:              otherFile(7),
		".gitignore":                 "*.log\n",
		"keep/x.txt":                 "x\n",
		"docs/dev/adr/0001-first.md": "adr\n",
	}
}

func mustPR(t *testing.T) MergemainPR {
	t.Helper()
	pr, err := ParseMergemainPR(testTakeInPR)
	if err != nil {
		t.Fatalf("ParseMergemainPR: %v", err)
	}
	return pr
}

// wantStop asserts that err is a stop with the reason and the restore need.
func wantStop(t *testing.T, err error, stop MergemainStop, restore RestoreNeed) *MergemainStopError {
	t.Helper()
	got, ok := errors.AsType[*MergemainStopError](err)
	if !ok {
		t.Fatalf("err = %v, want a stop %v", err, stop)
	}
	if got.Stop != stop || got.Restore != restore {
		t.Fatalf("stop = %v restore %v (%s), want %v restore %v", got.Stop, got.Restore, got.Detail, stop, restore)
	}
	return got
}

// begin runs begin and fails the test on a stop.
func begin(t *testing.T, k *takeIn) MergemainBeginResult {
	t.Helper()
	b, err := k.Tool().MergemainBegin(t.Context(), mustPR(t))
	if err != nil {
		t.Fatalf("MergemainBegin: %v", err)
	}
	return b
}

// merge runs begin and merge.
func merge(t *testing.T, k *takeIn) (MergemainBeginResult, MergemainMergeResult, error) {
	t.Helper()
	b := begin(t, k)
	r, err := k.Tool().MergemainMerge(t.Context(), mustPR(t), b.HeadBranch, b.BaseHead, b.Main)
	return b, r, err
}

// commitMerge resolves each Markdown conflict with resolutions and commits
// the merge with git's default message.
func commitMerge(t *testing.T, k *takeIn, resolutions map[string]string) {
	t.Helper()
	k.Apply(resolutions)
	k.Git("commit", "-q", "--no-edit", "--no-verify")
}

func classes(conflicts []PathConflict) map[string]PathConflictClass {
	m := make(map[string]PathConflictClass)
	for _, c := range conflicts {
		m[c.Path] = c.Class
	}
	return m
}

func TestMergemainBeginRejectsInvalidPRArg(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"", "0", "07", "-7", "+7", "7 ", " 7", "7a", "\uff11", "1234567890"} {
		t.Run(strconv.Quote(arg), func(t *testing.T) {
			t.Parallel()
			_, err := ParseMergemainPR(arg)
			wantStop(t, err, MergemainStopPremise, RestoreNotNeeded)
		})
	}
	for _, arg := range []string{"7", "1096", "123456789"} {
		pr, err := ParseMergemainPR(arg)
		if err != nil || pr.String() != arg {
			t.Errorf("ParseMergemainPR(%q) = %q, %v", arg, pr, err)
		}
	}
}

// TestMergemainZeroPRStopsBeforeAnything checks that the zero MergemainPR,
// which no constructor returns, stops every subcommand before git or gh runs.
func TestMergemainZeroPRStopsBeforeAnything(t *testing.T) {
	t.Parallel()
	fail := &fakeRunner{t: t}
	m := newMergemain(fail, fail, func(string) (string, bool) { return "", false })
	var pr MergemainPR
	_, err := m.MergemainBegin(t.Context(), pr)
	wantStop(t, err, MergemainStopPremise, RestoreNotNeeded)
	_, err = m.MergemainMerge(t.Context(), pr, takeInBranch, testHeadOID, testOtherOID)
	wantStop(t, err, MergemainStopPremise, RestoreNotNeeded)
	_, err = m.MergemainRestore(t.Context(), pr, takeInBranch, testHeadOID, testOtherOID)
	wantStop(t, err, MergemainStopOtherChanges, RestoreNotNeeded)
	_, err = m.MergemainPush(t.Context(), pr, testHeadOID, testOtherOID)
	wantStop(t, err, MergemainStopPremise, RestoreNeeded)
	fail.done()
}

func TestMergemainPRViewReadsNarrowFields(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	k.Setup(baseFiles(), map[string]string{"b.md": "b\n"}, map[string]string{"m.md": "m\n"})
	begin(t, k)
	want := []string{"pr", "view", testTakeInPR, "--json", "state,baseRefName,headRefName,headRefOid,isCrossRepository"}
	if len(k.GHCalls) != 1 || !slices.Equal(k.GHCalls[0], want) {
		t.Fatalf("gh calls = %q, want one %q", k.GHCalls, want)
	}
}

func TestParseMergemainPRRejectsMalformedValues(t *testing.T) {
	t.Parallel()
	head := strings.Repeat("a", 40)
	valid := `{"state":"OPEN","baseRefName":"main","headRefName":"feature","headRefOid":"` + head + `","isCrossRepository":false}`
	if v, err := parseMergemainPR([]byte(valid)); err != nil || v.headBranch != "feature" || v.headOID != head {
		t.Fatalf("valid PR = %+v, %v", v, err)
	}
	for name, tc := range map[string]struct {
		raw  string
		want error // nil for a rejection by the strict JSON reading
	}{
		"closed":           {strings.Replace(valid, `"OPEN"`, `"CLOSED"`, 1), errPRShape},
		"lowercase state":  {strings.Replace(valid, `"OPEN"`, `"open"`, 1), errPRShape},
		"other base":       {strings.Replace(valid, `"baseRefName":"main"`, `"baseRefName":"dev"`, 1), errPRShape},
		"cross repository": {strings.Replace(valid, `false}`, `true}`, 1), errPRShape},
		"head is main":     {strings.Replace(valid, `"feature"`, `"main"`, 1), errBranchName},
		"head option":      {strings.Replace(valid, `"feature"`, `"-f"`, 1), errBranchName},
		"head with space":  {strings.Replace(valid, `"feature"`, `"a b"`, 1), errBranchName},
		"head with equals": {strings.Replace(valid, `"feature"`, `"a=b"`, 1), errBranchName},
		"head with format": {strings.Replace(valid, `"feature"`, `"a\u200bb"`, 1), errBranchName},
		"head with C1":     {strings.Replace(valid, `"feature"`, `"a\u0085b"`, 1), errBranchName},
		"short head":       {strings.Replace(valid, head, head[:39], 1), errPRShape},
		"uppercase head":   {strings.Replace(valid, head, strings.ToUpper(head), 1), errPRShape},
		"null state":       {strings.Replace(valid, `"OPEN"`, `null`, 1), nil},
		"string bool":      {strings.Replace(valid, `false}`, `"false"}`, 1), nil},
		"missing member":   {strings.Replace(valid, `,"isCrossRepository":false`, ``, 1), nil},
		"duplicate member": {strings.Replace(valid, `{"state":"OPEN",`, `{"state":"OPEN","state":"OPEN",`, 1), nil},
	} {
		_, err := parseMergemainPR([]byte(tc.raw))
		if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Errorf("%s: parseMergemainPR(%s) = %v, want %v", name, tc.raw, err, tc.want)
		}
	}
}

func TestMergemainBeginPremisesAndNothingToTakeIn(t *testing.T) {
	t.Parallel()
	branch := map[string]string{"b.md": "b\n"}
	main := map[string]string{"m.md": "m\n"}
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		baseHead, mainOID := k.Setup(baseFiles(), branch, main)
		b := begin(t, k)
		if b.BaseHead != baseHead || b.Main != mainOID || b.HeadBranch != takeInBranch {
			t.Fatalf("begin = %+v, want %s %s", b, baseHead, mainOID)
		}
		if got := k.Git("rev-parse", "refs/mergemain/"+testTakeInPR+"/main"); got != mainOID {
			t.Fatalf("recorded main = %s, want %s", got, mainOID)
		}
		if status := k.Git("status", "--porcelain"); status != "" {
			t.Fatalf("begin changed the work tree: %q", status)
		}
	})
	for name, tc := range map[string]struct {
		change func(t *testing.T, k *takeIn)
		stop   MergemainStop
	}{
		"closed PR":      {func(_ *testing.T, k *takeIn) { k.PR.State = "CLOSED" }, MergemainStopPremise},
		"GH_REPO set":    {func(_ *testing.T, k *takeIn) { k.Env[ghRepoVariable] = "" }, MergemainStopPremise},
		"second remote":  {func(_ *testing.T, k *takeIn) { k.Git("remote", "add", "upstream", k.Origin) }, MergemainStopPremise},
		"other branch":   {func(_ *testing.T, k *takeIn) { k.Git("switch", "-q", mainBranch) }, MergemainStopPremise},
		"tracked change": {func(t *testing.T, k *takeIn) { writeWork(t, k, "doc.md", "changed\n") }, MergemainStopPremise},
		"untracked file": {func(t *testing.T, k *takeIn) { writeWork(t, k, "new.txt", "n\n") }, MergemainStopPremise},
		"head behind PR": {func(_ *testing.T, k *takeIn) { k.PR.HeadOID = k.Git("rev-parse", "HEAD~1") }, MergemainStopPremise},
		"local commit":   {func(_ *testing.T, k *takeIn) { k.Commit("local", map[string]string{"l.md": "l\n"}) }, MergemainStopPremise},
		"origin's branch is not the PR head": {func(_ *testing.T, k *takeIn) {
			// The PR head names HEAD, but origin's branch moves while gh is
			// read, so only the remote-tracking ref differs.
			k.PR.HeadOID = k.Git("rev-parse", "HEAD")
			k.PR.OnView = func() {
				k.OriginGit("update-ref", headsPrefix+takeInBranch, k.OriginGit("rev-parse", headsPrefix+mainBranch))
			}
		}, MergemainStopPremise},
		"gh default repository elsewhere": {func(_ *testing.T, k *takeIn) {
			k.Git("config", "remote.origin.gh-resolved", "other/repo")
		}, MergemainStopPremise},
		"shallow repository": {func(_ *testing.T, k *takeIn) {
			k.Git("fetch", "-q", "--depth=1", "origin", mainBranch)
		}, MergemainStopPremise},
		"skip-worktree set": {func(_ *testing.T, k *takeIn) { k.Git("update-index", "--skip-worktree", "doc.md") }, MergemainStopPremise},
		"nothing to take in": {func(_ *testing.T, k *takeIn) {
			k.Git("merge", "-q", "--no-edit", "origin/main")
			k.Git("push", "-q", "origin", takeInBranch)
		}, MergemainStopNothingToTakeIn},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			k.Setup(baseFiles(), branch, main)
			tc.change(t, k)
			before := k.Git("rev-parse", "HEAD")
			_, err := k.Tool().MergemainBegin(t.Context(), mustPR(t))
			wantStop(t, err, tc.stop, RestoreNotNeeded)
			if after := k.Git("rev-parse", "HEAD"); after != before {
				t.Fatalf("HEAD moved from %s to %s", before, after)
			}
		})
	}
}

// writeWork writes a file of the work tree without staging it.
func writeWork(t *testing.T, k *takeIn, name, content string) {
	t.Helper()
	p := filepath.Join(k.Work, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), testRepoDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readWork(t *testing.T, k *takeIn, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(k.Work, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMergemainMergeNoFastForward(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	// The branch adds nothing, so main is a descendant of the base head.
	_, mainOID := k.Setup(baseFiles(), nil, map[string]string{"m.md": "m\n"})
	k.Git("reset", "-q", "--hard", "origin/main~1")
	k.Git("push", "-q", "-f", "origin", takeInBranch)
	baseHead := k.Git("rev-parse", "HEAD")
	b, r, err := merge(t, k)
	if err != nil {
		t.Fatalf("MergemainMerge: %v", err)
	}
	if b.BaseHead != baseHead || len(r.Conflicts) != 0 {
		t.Fatalf("begin %+v, merge %+v", b, r)
	}
	commitMerge(t, k, nil)
	if parents := k.Git("rev-list", "--parents", "-n", "1", "HEAD"); parents != k.Git("rev-parse", "HEAD")+" "+baseHead+" "+mainOID {
		t.Fatalf("HEAD parents = %s, want a merge commit of %s and %s", parents, baseHead, mainOID)
	}
}

func TestMergemainMergeRefusesIgnoredOverwrite(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	k.Setup(baseFiles(), map[string]string{"b.md": "b\n"}, nil)
	k.Git("switch", "-q", mainBranch)
	writeWork(t, k, "out.log", "main's\n")
	k.Git("add", "-f", "out.log")
	k.Git("commit", "-q", "-m", "track a log")
	k.Git("push", "-q", "origin", mainBranch)
	k.Git("switch", "-q", takeInBranch)
	writeWork(t, k, "out.log", "precious\n")
	_, _, err := merge(t, k)
	wantStop(t, err, MergemainStopPremise, RestoreNotNeeded)
	if got := readWork(t, k, "out.log"); got != "precious\n" {
		t.Fatalf("out.log = %q, the merge overwrote the ignored file", got)
	}
	if _, inMerge, _ := k.Tool().mergeHead(t.Context()); inMerge {
		t.Fatal("a merge started")
	}
}

func TestMergemainMergeChecksArguments(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	k.Setup(baseFiles(), map[string]string{"b.md": "b\n"}, map[string]string{"m.md": "m\n"})
	b := begin(t, k)
	other := k.Git("rev-parse", "HEAD~1")
	for name, args := range map[string][3]string{
		"other main":      {b.HeadBranch, b.BaseHead, other},
		"other base head": {b.HeadBranch, other, b.Main},
		"other branch":    {mainBranch, b.BaseHead, b.Main},
		"short commit":    {b.HeadBranch, b.BaseHead[:12], b.Main},
	} {
		_, err := k.Tool().MergemainMerge(t.Context(), mustPR(t), args[0], args[1], args[2])
		if stop, ok := errors.AsType[*MergemainStopError](err); !ok || stop.Stop != MergemainStopPremise || stop.Restore != RestoreNotNeeded {
			t.Errorf("%s: err = %v, want a premise stop without restore", name, err)
		}
		if _, inMerge, _ := k.Tool().mergeHead(t.Context()); inMerge {
			t.Fatalf("%s: a merge started", name)
		}
	}
}

func TestMergemainConflictClassification(t *testing.T) {
	t.Parallel()
	a, bsrc := renameSource("a"), renameSource("b")
	for name, tc := range map[string]struct {
		base, branch, main map[string]string
		want               map[string]PathConflictClass
		// beforeMerge is true when the stop comes before the merge starts.
		beforeMerge bool
	}{
		"markdown content": {
			branch: map[string]string{"doc.md": "one\nbranch\nthree\n"},
			main:   map[string]string{"doc.md": "one\nmain\nthree\n"},
			want:   map[string]PathConflictClass{"doc.md": PathConflictMarkdown},
		},
		"markdown added on both sides": {
			branch: map[string]string{"new.md": "branch\n"},
			main:   map[string]string{"new.md": "main\n"},
			want:   map[string]PathConflictClass{"new.md": PathConflictMarkdown},
		},
		"protocol line only": {
			branch: map[string]string{takeInToolPath: toolFile(4, "base")},
			main:   map[string]string{takeInToolPath: toolFile(5, "base")},
			want:   map[string]PathConflictClass{takeInToolPath: PathConflictProtocolOnly},
		},
		"protocol and another line": {
			branch: map[string]string{takeInToolPath: toolFile(4, "branch")},
			main:   map[string]string{takeInToolPath: toolFile(5, "main")},
			want:   map[string]PathConflictClass{takeInToolPath: PathConflictOutOfScope},
		},
		"test file": {
			// Only a declaration-like line differs, which would be in scope in
			// a file that is not a test.
			base:   map[string]string{"internal/tool/tool_test.go": "package tool\n\nconst Protocol = 3\n"},
			branch: map[string]string{takeInToolPath: toolFile(4, "base"), "internal/tool/tool_test.go": "package tool\n\nconst Protocol = 4\n"},
			main:   map[string]string{"internal/tool/tool_test.go": "package tool\n\nconst Protocol = 5\n"},
			want:   map[string]PathConflictClass{"internal/tool/tool_test.go": PathConflictOutOfScope},
		},
		"go file": {
			branch: map[string]string{"main.go": "package main\n\nfunc main() { branch() }\n"},
			main:   map[string]string{"main.go": "package main\n\nfunc main() { main2() }\n"},
			want:   map[string]PathConflictClass{"main.go": PathConflictOutOfScope},
		},
		"markdown modify/delete": {
			branch: map[string]string{"doc.md": "one\nbranch\nthree\n"},
			main:   map[string]string{"doc.md": deletedFile},
			want:   map[string]PathConflictClass{"doc.md": PathConflictOutOfScope},
		},
		"markdown renamed to one path on both sides": {
			base:   map[string]string{"a.md": a, "b.md": bsrc},
			branch: map[string]string{"a.md": deletedFile, "c.md": a},
			main:   map[string]string{"b.md": deletedFile, "c.md": bsrc},
			want:   map[string]PathConflictClass{"c.md": PathConflictOutOfScope},
		},
		"markdown renamed onto an addition": {
			base:   map[string]string{"a.md": a},
			branch: map[string]string{"a.md": deletedFile, "c.md": a},
			main:   map[string]string{"c.md": "added on main\n"},
			want:   map[string]PathConflictClass{"c.md": PathConflictOutOfScope},
		},
		"markdown renamed and changed": {
			base:   map[string]string{"a.md": a},
			branch: map[string]string{"a.md": deletedFile, "c.md": strings.Replace(a, "a line 0", "branch line", 1)},
			main:   map[string]string{"a.md": strings.Replace(a, "a line 0", "main line", 1)},
			want:   map[string]PathConflictClass{"c.md": PathConflictOutOfScope},
		},
		"tool file renamed with only its declaration line in conflict": {
			// Apart from the rename, this is a protocol-only conflict, so only
			// the rename check can stop it.
			base:   map[string]string{"internal/third/a.go": thirdFile(2)},
			branch: map[string]string{"internal/third/a.go": deletedFile, "internal/third/b.go": thirdFile(3)},
			main:   map[string]string{"internal/third/a.go": thirdFile(4)},
			want:   map[string]PathConflictClass{"internal/third/b.go": PathConflictOutOfScope},
		},
		"symbolic link": {
			base:   map[string]string{"link.md": symlinkPrefix + "doc.md"},
			branch: map[string]string{"link.md": symlinkPrefix + "README.md"},
			main:   map[string]string{"link.md": symlinkPrefix + "main.go"},
			want:   map[string]PathConflictClass{"link.md": PathConflictOutOfScope},
		},
		"binary markdown": {
			branch: map[string]string{"bin.md": "branch\x00"},
			main:   map[string]string{"bin.md": "main\x00"},
			want:   map[string]PathConflictClass{"bin.md": PathConflictOutOfScope},
		},
		"markdown with merge attribute unset": {
			base:        map[string]string{".gitattributes": "doc.md -merge\n"},
			branch:      map[string]string{"doc.md": "one\nbranch\nthree\n"},
			main:        map[string]string{"doc.md": "one\nmain\nthree\n"},
			want:        map[string]PathConflictClass{"doc.md": PathConflictOutOfScope},
			beforeMerge: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			base := baseFiles()
			maps.Copy(base, tc.base)
			k.Setup(base, tc.branch, tc.main)
			_, r, err := merge(t, k)
			outOfScope := slices.Contains(slices.Collect(maps.Values(tc.want)), PathConflictOutOfScope)
			if outOfScope {
				restore := RestoreNeeded
				if tc.beforeMerge {
					restore = RestoreNotNeeded
				}
				stop := wantStop(t, err, MergemainStopOutOfScope, restore)
				r.Conflicts = stop.Conflicts
			} else if err != nil {
				t.Fatalf("MergemainMerge: %v", err)
			}
			// The whole list is compared: the zero class is out of scope, so a
			// lookup of a path the stop left out would pass.
			if got := classes(r.Conflicts); !maps.Equal(got, tc.want) {
				t.Errorf("conflicts = %v, want %v", got, tc.want)
			}
		})
	}
}

// thirdFile is a third tool's file with number n, long enough for rename
// detection.
func thirdFile(n int) string {
	var b strings.Builder
	b.WriteString("package third\n\nconst Protocol = " + strconv.Itoa(n) + "\n\n")
	for i := range 12 {
		b.WriteString("// line " + strconv.Itoa(i) + "\n")
	}
	return b.String()
}

func TestMergemainProtocolUpdateAndRuleFacts(t *testing.T) {
	t.Parallel()
	t.Run("protocol line conflict resolved to main plus one", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		k.Setup(baseFiles(), map[string]string{takeInToolPath: toolFile(4, "base")}, map[string]string{takeInToolPath: toolFile(5, "base"), "README.md": "readme on main\n"})
		_, r, err := merge(t, k)
		if err != nil {
			t.Fatalf("MergemainMerge: %v", err)
		}
		if !slices.Equal(r.Protocol, []ProtocolBump{{Tool: "tool", Value: 6}}) {
			t.Fatalf("protocol = %+v, want tool 6", r.Protocol)
		}
		if got := readWork(t, k, takeInToolPath); got != toolFile(6, "base") {
			t.Fatalf("tool.go = %q", got)
		}
		if u := k.Git("ls-files", "-u"); u != "" {
			t.Fatalf("still conflicted: %s", u)
		}
		if !r.ReadmeChanged {
			t.Fatal("README change not reported")
		}
	})
	t.Run("same raise on both sides set without a conflict", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		k.Setup(baseFiles(), map[string]string{takeInToolPath: toolFile(4, "branch")}, map[string]string{takeInToolPath: toolFile(4, "base")})
		_, r, err := merge(t, k)
		if err != nil {
			t.Fatalf("MergemainMerge: %v", err)
		}
		if len(r.Conflicts) != 0 || !slices.Equal(r.Protocol, []ProtocolBump{{Tool: "tool", Value: 5}}) {
			t.Fatalf("merge = %+v, want no conflict and tool 5", r)
		}
		if got := k.Git("show", ":"+takeInToolPath); got+"\n" != toolFile(5, "branch") {
			t.Fatalf("staged tool.go = %q", got)
		}
		if r.ReadmeChanged {
			t.Fatal("README change reported for an unchanged README")
		}
	})
	t.Run("raise on the branch only kept", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		k.Setup(baseFiles(), map[string]string{takeInToolPath: toolFile(4, "base")}, map[string]string{"m.md": "m\n"})
		_, r, err := merge(t, k)
		if err != nil {
			t.Fatalf("MergemainMerge: %v", err)
		}
		if got := readWork(t, k, takeInToolPath); !slices.Equal(r.Protocol, []ProtocolBump{{Tool: "tool", Value: 4}}) || got != toolFile(4, "base") {
			t.Fatalf("protocol = %+v, tool.go = %q", r.Protocol, got)
		}
	})
	t.Run("unchanged tool not set", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		k.Setup(baseFiles(), map[string]string{"b.md": "b\n"}, map[string]string{takeInOtherPath: otherFile(8)})
		_, r, err := merge(t, k)
		if err != nil {
			t.Fatalf("MergemainMerge: %v", err)
		}
		if len(r.Protocol) != 0 {
			t.Fatalf("protocol = %+v, want none", r.Protocol)
		}
	})
}

func TestMergemainReportsADRConflictFacts(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		branch, main  map[string]string
		mainChanged   bool
		wantConflicts []string
	}{
		"same number on both sides": {
			branch:        map[string]string{"docs/dev/adr/0002-branch.md": "b\n"},
			main:          map[string]string{"docs/dev/adr/0002-main.md": "m\n"},
			mainChanged:   true,
			wantConflicts: []string{"0002"},
		},
		"branch number free": {
			branch:      map[string]string{"docs/dev/adr/0003-branch.md": "b\n"},
			main:        map[string]string{"docs/dev/adr/0002-main.md": "m\n"},
			mainChanged: true,
		},
		"main changes no ADR": {
			branch: map[string]string{"docs/dev/adr/0001-first.md": "changed\n"},
			main:   map[string]string{"m.md": "m\n"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			k.Setup(baseFiles(), tc.branch, tc.main)
			_, r, err := merge(t, k)
			if err != nil {
				t.Fatalf("MergemainMerge: %v", err)
			}
			if r.ADRMainChanged != tc.mainChanged || !slices.Equal(r.ADRConflictNumbers, tc.wantConflicts) {
				t.Fatalf("ADR facts = %v %q, want %v %q", r.ADRMainChanged, r.ADRConflictNumbers, tc.mainChanged, tc.wantConflicts)
			}
		})
	}
}

func TestMergemainExtCheckHint(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		main map[string]string
		want bool
	}{
		"extension file":      {map[string]string{"extension/src/a.ts": "a\n"}, true},
		"development guide":   {map[string]string{"docs/dev/guides/development.md": "d\n"}, true},
		"English README":      {map[string]string{"README.en.md": "e\n"}, true},
		"unrelated file":      {map[string]string{"docs/other.md": "o\n"}, false},
		"extension-like name": {map[string]string{"extensions.md": "x\n"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			k.Setup(baseFiles(), map[string]string{"b.md": "b\n"}, tc.main)
			_, r, err := merge(t, k)
			if err != nil {
				t.Fatalf("MergemainMerge: %v", err)
			}
			if r.ExtCheckHint != tc.want {
				t.Fatalf("ext-check hint = %v, want %v", r.ExtCheckHint, tc.want)
			}
		})
	}
}

// conflictSetup makes a Markdown conflict on doc.md.
func conflictSetup(k *takeIn) (baseHead, mainOID string) {
	return k.Setup(baseFiles(), map[string]string{"doc.md": "one\nbranch\nthree\n"}, map[string]string{"doc.md": "one\nmain\nthree\n"})
}

// assertRestored checks the state a restore must leave: HEAD at the base
// head, no tracked change, and no merge in progress.
func assertRestored(t *testing.T, k *takeIn, baseHead string) {
	t.Helper()
	if head := k.Git("rev-parse", "HEAD"); head != baseHead {
		t.Fatalf("HEAD = %s, want %s", head, baseHead)
	}
	if status := k.Git("status", "--porcelain", "--untracked-files=no"); status != "" {
		t.Fatalf("tracked changes left: %q", status)
	}
	if _, inMerge, _ := k.Tool().mergeHead(t.Context()); inMerge {
		t.Fatal("a merge is still in progress")
	}
}

func TestMergemainRestoreSafeAndRefusesOtherChanges(t *testing.T) {
	t.Parallel()
	restore := func(t *testing.T, k *takeIn, b MergemainBeginResult) error {
		t.Helper()
		_, err := k.Tool().MergemainRestore(t.Context(), mustPR(t), b.HeadBranch, b.BaseHead, b.Main)
		return err
	}
	t.Run("merge in progress with Markdown edits", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		baseHead, _ := conflictSetup(k)
		b, _, err := merge(t, k)
		if err != nil {
			t.Fatalf("MergemainMerge: %v", err)
		}
		writeWork(t, k, "doc.md", "one\nresolved\nthree\n")
		k.Git("add", "doc.md")
		writeWork(t, k, "README.md", "edited, untouched by the merge\n")
		writeWork(t, k, "ignored.log", "kept\n")
		if err := restore(t, k, b); err != nil {
			t.Fatalf("MergemainRestore: %v", err)
		}
		assertRestored(t, k, baseHead)
		if readWork(t, k, "ignored.log") != "kept\n" {
			t.Fatal("restore removed an ignored file")
		}
	})
	t.Run("merge stopped out of scope", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		baseHead, _ := k.Setup(baseFiles(), map[string]string{"main.go": "package main\n\nfunc main() { b() }\n"}, map[string]string{"main.go": "package main\n\nfunc main() { m() }\n"})
		b, _, err := merge(t, k)
		wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
		if err := restore(t, k, b); err != nil {
			t.Fatalf("MergemainRestore: %v", err)
		}
		assertRestored(t, k, baseHead)
	})
	t.Run("protocol conflict the merge resolved", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		baseHead, _ := k.Setup(baseFiles(), map[string]string{takeInToolPath: toolFile(4, "base")}, map[string]string{takeInToolPath: toolFile(5, "base")})
		b, _, err := merge(t, k)
		if err != nil {
			t.Fatalf("MergemainMerge: %v", err)
		}
		if err := restore(t, k, b); err != nil {
			t.Fatalf("MergemainRestore: %v", err)
		}
		assertRestored(t, k, baseHead)
	})
	t.Run("merge commit not pushed", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		baseHead, _ := conflictSetup(k)
		b, _, err := merge(t, k)
		if err != nil {
			t.Fatalf("MergemainMerge: %v", err)
		}
		commitMerge(t, k, map[string]string{"doc.md": "one\nresolved\nthree\n"})
		if err := restore(t, k, b); err != nil {
			t.Fatalf("MergemainRestore: %v", err)
		}
		assertRestored(t, k, baseHead)
	})
	for name, change := range map[string]func(t *testing.T, k *takeIn, b MergemainBeginResult){
		"other branch": func(_ *testing.T, k *takeIn, _ MergemainBeginResult) {
			k.Git("merge", "--abort")
			k.Git("switch", "-q", mainBranch)
		},
		"go change during the merge": func(t *testing.T, k *takeIn, _ MergemainBeginResult) {
			writeWork(t, k, "main.go", "package main\n\n// someone's work\nfunc main() {}\n")
		},
		"go change staged during the merge": func(t *testing.T, k *takeIn, _ MergemainBeginResult) {
			writeWork(t, k, "main.go", "package main\n\n// someone's work\nfunc main() {}\n")
			k.Git("add", "main.go")
		},
		"untracked file": func(t *testing.T, k *takeIn, _ MergemainBeginResult) {
			writeWork(t, k, "notes.txt", "someone's\n")
		},
		"ignored file where the base head tracks one": func(t *testing.T, k *takeIn, _ MergemainBeginResult) {
			// The merge commit removes keep/x.txt, and an ignored file takes
			// its place, which a reset to the base head would overwrite.
			commitMerge(t, k, map[string]string{"doc.md": "one\nresolved\nthree\n", "keep/x.txt": deletedFile})
			writeWork(t, k, ".git/info/exclude", "keep/\n")
			writeWork(t, k, "keep/x.txt", "someone's\n")
		},
		"MERGE_HEAD is another commit": func(_ *testing.T, k *takeIn, b MergemainBeginResult) {
			// The index holds only Markdown changes against the computed
			// merge, so only the MERGE_HEAD comparison can stop this.
			k.Git("merge", "--abort")
			other := k.Git("commit-tree", b.Main+"^{tree}", "-p", b.Main, "-m", "another main")
			k.Git("merge", "-q", "--no-ff", "--no-commit", "-s", "ours", other)
		},
		"commit after the merge commit": func(t *testing.T, k *takeIn, _ MergemainBeginResult) {
			commitMerge(t, k, map[string]string{"doc.md": "one\nresolved\nthree\n"})
			k.Commit("more", map[string]string{"more.md": "m\n"})
		},
		"tracked change after the merge commit": func(t *testing.T, k *takeIn, _ MergemainBeginResult) {
			commitMerge(t, k, map[string]string{"doc.md": "one\nresolved\nthree\n"})
			writeWork(t, k, "doc.md", "someone's\n")
		},
		"merge commit with swapped parents": func(t *testing.T, k *takeIn, b MergemainBeginResult) {
			commitMerge(t, k, map[string]string{"doc.md": "one\nresolved\nthree\n"})
			swapped := k.Git("commit-tree", "HEAD^{tree}", "-p", b.Main, "-p", b.BaseHead, "-m", "swapped")
			k.Git("reset", "-q", "--hard", swapped)
		},
		"merge commit pushed": func(t *testing.T, k *takeIn, _ MergemainBeginResult) {
			commitMerge(t, k, map[string]string{"doc.md": "one\nresolved\nthree\n"})
			k.Git("push", "-q", "origin", takeInBranch)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			conflictSetup(k)
			b, _, err := merge(t, k)
			if err != nil {
				t.Fatalf("MergemainMerge: %v", err)
			}
			change(t, k, b)
			before := k.Git("rev-parse", "HEAD")
			status := k.Git("status", "--porcelain=v1", "--untracked-files=all", "--ignored")
			wantStop(t, restore(t, k, b), MergemainStopOtherChanges, RestoreNotNeeded)
			if after := k.Git("rev-parse", "HEAD"); after != before {
				t.Fatalf("HEAD moved from %s to %s", before, after)
			}
			if after := k.Git("status", "--porcelain=v1", "--untracked-files=all", "--ignored"); after != status {
				t.Fatalf("the work tree changed:\n%s\nwant\n%s", after, status)
			}
		})
	}
}

// pushFlow runs begin, merge, and the commit of a Markdown conflict.
func pushFlow(t *testing.T, k *takeIn) MergemainBeginResult {
	t.Helper()
	conflictSetup(k)
	b, _, err := merge(t, k)
	if err != nil {
		t.Fatalf("MergemainMerge: %v", err)
	}
	commitMerge(t, k, map[string]string{"doc.md": "one\nbranch\nmain\nthree\n"})
	return b
}

func push(t *testing.T, k *takeIn, b MergemainBeginResult) (MergemainPushResult, error) {
	t.Helper()
	return k.Tool().MergemainPush(t.Context(), mustPR(t), b.BaseHead, b.Main)
}

// preReceive installs a pre-receive hook in origin.
func preReceive(t *testing.T, k *takeIn, script string) {
	t.Helper()
	dir := filepath.Join(k.Origin, "hooks")
	if err := os.MkdirAll(dir, testRepoDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pre-receive"), []byte("#!/bin/sh\n"+script+"\n"), 0o700); err != nil { //nolint:gosec // the hook must be executable
		t.Fatal(err)
	}
}

func TestMergemainPushSuccessRejectedAndUnknown(t *testing.T) {
	t.Parallel()
	t.Run("pushed", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		b := pushFlow(t, k)
		head := k.Git("rev-parse", "HEAD")
		r, err := push(t, k, b)
		if err != nil {
			t.Fatalf("MergemainPush: %v", err)
		}
		if r.NewHead != head || k.OriginGit("rev-parse", headsPrefix+takeInBranch) != head {
			t.Fatalf("pushed %s, origin has %s, want %s", r.NewHead, k.OriginGit("rev-parse", headsPrefix+takeInBranch), head)
		}
	})
	t.Run("rejected", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		b := pushFlow(t, k)
		preReceive(t, k, "exit 1")
		_, err := push(t, k, b)
		wantStop(t, err, MergemainStopPushRejected, RestoreNeeded)
		if got := k.OriginGit("rev-parse", headsPrefix+takeInBranch); got != b.BaseHead {
			t.Fatalf("origin moved to %s", got)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		b := pushFlow(t, k)
		// The hook moves the branch elsewhere and fails, so origin is
		// neither the base head nor the pushed commit afterwards.
		preReceive(t, k, "printf '%s\\n' "+b.Main+" > refs/heads/"+takeInBranch+"\nexit 1")
		_, err := push(t, k, b)
		wantStop(t, err, MergemainStopPushUnknown, RestoreNotNeeded)
	})
}

func TestMergemainPushRejectsNonHeadOrForce(t *testing.T) {
	t.Parallel()
	// The push's arguments are the observable contract here: the checks
	// before it stop every input on which a force push would behave
	// differently, so only the command shows that none is used.
	t.Run("push command", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		b := pushFlow(t, k)
		head := k.Git("rev-parse", "HEAD")
		rec := &recordingRunner{run: k.GitRunner()}
		if _, err := k.ToolWith(rec).MergemainPush(t.Context(), mustPR(t), b.BaseHead, b.Main); err != nil {
			t.Fatalf("MergemainPush: %v", err)
		}
		var pushes [][]string
		for _, c := range rec.calls {
			if i := slices.Index(c, "push"); i >= 0 && i == len(c)-7 {
				pushes = append(pushes, c[i:])
			}
		}
		want := []string{"push", "-q", "--no-follow-tags", "--recurse-submodules=no", "--", "origin", head + ":refs/heads/" + takeInBranch}
		if len(pushes) != 1 || !slices.Equal(pushes[0], want) {
			t.Fatalf("pushes = %q, want one %q", pushes, want)
		}
	})
	for name, change := range map[string]func(k *takeIn){
		"PR head branch is main": func(k *takeIn) { k.PR.Branch = mainBranch },
		"current branch is not the PR's": func(k *takeIn) {
			k.Git("switch", "-q", "-c", "elsewhere")
		},
		"GH_REPO set":      func(k *takeIn) { k.Env[ghRepoVariable] = "other/repo" },
		"second remote":    func(k *takeIn) { k.Git("remote", "add", "upstream", k.Origin) },
		"push URL differs": func(k *takeIn) { k.Git("config", "remote.origin.pushurl", k.Origin+"-elsewhere") },
		"origin moved": func(k *takeIn) {
			k.PR.HeadOID = k.Git("rev-parse", "HEAD^1")
			k.OriginGit("update-ref", headsPrefix+takeInBranch, k.Git("rev-parse", "HEAD^2"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			b := pushFlow(t, k)
			change(k)
			before := k.OriginGit("rev-parse", headsPrefix+takeInBranch)
			_, err := push(t, k, b)
			wantStop(t, err, MergemainStopPremise, RestoreNeeded)
			if after := k.OriginGit("rev-parse", headsPrefix+takeInBranch); after != before {
				t.Fatalf("origin moved from %s to %s", before, after)
			}
		})
	}
}

func TestMergemainPushChecksParentsAndMain(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		change func(t *testing.T, k *takeIn, b *MergemainBeginResult)
		stop   MergemainStop
	}{
		"first parent is not the base head": {func(_ *testing.T, k *takeIn, b *MergemainBeginResult) {
			swapped := k.Git("commit-tree", "HEAD^{tree}", "-p", b.Main, "-p", b.BaseHead, "-m", "swapped")
			k.Git("reset", "-q", "--hard", swapped)
		}, MergemainStopPremise},
		"PR head is not the base head": {func(_ *testing.T, k *takeIn, b *MergemainBeginResult) {
			k.PR.HeadOID = b.Main
		}, MergemainStopPremise},
		"recorded main is an older main": {func(_ *testing.T, k *takeIn, b *MergemainBeginResult) {
			// The ref names an older main of origin's history, so the merge
			// commit's parents and the ancestry still match.
			k.Git("update-ref", "refs/mergemain/"+testTakeInPR+"/main", b.Main+"~1")
		}, MergemainStopPremise},
		"recorded main outside origin's history": {func(_ *testing.T, k *takeIn, b *MergemainBeginResult) {
			fake := k.Git("commit-tree", b.Main+"^{tree}", "-p", b.Main, "-m", "not on origin")
			k.Git("update-ref", "refs/mergemain/"+testTakeInPR+"/main", fake)
			merged := k.Git("commit-tree", "HEAD^{tree}", "-p", b.BaseHead, "-p", fake, "-m", "merge a main origin never had")
			k.Git("reset", "-q", "--hard", merged)
			b.Main = fake
		}, MergemainStopPremise},
		"merge in progress": {func(_ *testing.T, k *takeIn, _ *MergemainBeginResult) {
			// A merge started on top of the right merge commit, which leaves
			// the work tree clean, so only the MERGE_HEAD check can stop it.
			child := k.Git("commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "child")
			k.Git("merge", "-q", "--no-ff", "--no-commit", "-s", "ours", child)
		}, MergemainStopPremise},
		"tracked change": {func(t *testing.T, k *takeIn, _ *MergemainBeginResult) {
			writeWork(t, k, "doc.md", "someone's\n")
		}, MergemainStopOtherChanges},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			b := pushFlow(t, k)
			tc.change(t, k, &b)
			restore := RestoreNeeded
			if tc.stop == MergemainStopOtherChanges {
				restore = RestoreNotNeeded
			}
			before := k.OriginGit("rev-parse", headsPrefix+takeInBranch)
			_, err := push(t, k, b)
			wantStop(t, err, tc.stop, restore)
			if got := k.OriginGit("rev-parse", headsPrefix+takeInBranch); got != before {
				t.Fatalf("origin moved from %s to %s", before, got)
			}
		})
	}
}

// amendMerge rewrites the merge commit with files changed, keeping its
// parents.
func amendMerge(t *testing.T, k *takeIn, files map[string]string) {
	t.Helper()
	k.Apply(files)
	k.Git("commit", "-q", "--amend", "--no-edit", "--no-verify")
}

func TestMergemainPushRejectsOutOfScope(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(t *testing.T, k *takeIn){
		"go file changed": func(t *testing.T, k *takeIn) {
			amendMerge(t, k, map[string]string{"main.go": "package main\n\nfunc main() { evil() }\n"})
		},
		"protocol line of a tool the branch did not change": func(t *testing.T, k *takeIn) {
			amendMerge(t, k, map[string]string{takeInOtherPath: otherFile(9)})
		},
		"markdown made a symbolic link": func(t *testing.T, k *takeIn) {
			amendMerge(t, k, map[string]string{"doc.md": symlinkPrefix + "README.md"})
		},
		"markdown removed": func(t *testing.T, k *takeIn) {
			amendMerge(t, k, map[string]string{"doc.md": deletedFile})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			b := pushFlow(t, k)
			change(t, k)
			_, err := push(t, k, b)
			wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
		})
	}
	t.Run("out-of-scope conflict committed as it is", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		// A modify/delete conflict whose changed side is kept: the committed
		// tree equals the computed merge's, so only the conflict list can
		// stop this.
		k.Setup(baseFiles(), map[string]string{"main.go": "package main\n\nfunc main() { b() }\n"}, map[string]string{"main.go": deletedFile})
		b, _, err := merge(t, k)
		wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
		k.Git("add", "-A")
		k.Git("commit", "-q", "--no-edit", "--no-verify")
		_, err = push(t, k, b)
		stop := wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
		if got := classes(stop.Conflicts); !maps.Equal(got, map[string]PathConflictClass{"main.go": PathConflictOutOfScope}) {
			t.Fatalf("conflicts = %v", got)
		}
	})
}

func TestMergemainPushChecksProtocolValues(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		branch, main map[string]string
		commitValue  int // 0 keeps what the merge set
		// commitBody, when set, replaces the tool's file in the merge commit.
		commitBody string
		// chmod makes the tool's file executable in the merge commit, so
		// only its mode differs from the merge of the parents.
		chmod bool
		ok    bool
	}{
		"mode of a changed tool's file changed": {
			branch: map[string]string{takeInToolPath: toolFile(4, "base")},
			main:   map[string]string{takeInToolPath: toolFile(5, "base")},
			chmod:  true,
		},
		"body of a changed tool edited": {
			branch:     map[string]string{takeInToolPath: toolFile(4, "base")},
			main:       map[string]string{takeInToolPath: toolFile(5, "base")},
			commitBody: toolFile(6, "evil"),
		},
		"value the merge set": {
			branch: map[string]string{takeInToolPath: toolFile(4, "base")},
			main:   map[string]string{takeInToolPath: toolFile(5, "base")},
			ok:     true,
		},
		"value changed to another integer": {
			branch:      map[string]string{takeInToolPath: toolFile(4, "base")},
			main:        map[string]string{takeInToolPath: toolFile(5, "base")},
			commitValue: 9,
		},
		"value left at the automatic merge's": {
			branch:      map[string]string{takeInToolPath: toolFile(4, "branch")},
			main:        map[string]string{takeInToolPath: toolFile(4, "base")},
			commitValue: 4,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			k.Setup(baseFiles(), tc.branch, tc.main)
			b, _, err := merge(t, k)
			if err != nil {
				t.Fatalf("MergemainMerge: %v", err)
			}
			if tc.commitValue != 0 {
				content := readWork(t, k, takeInToolPath)
				i := strings.Index(content, "const Protocol = ")
				end := i + strings.IndexByte(content[i:], '\n')
				k.Apply(map[string]string{takeInToolPath: content[:i] + "const Protocol = " + strconv.Itoa(tc.commitValue) + content[end:]})
			}
			if tc.commitBody != "" {
				k.Apply(map[string]string{takeInToolPath: tc.commitBody})
			}
			if tc.chmod {
				if err := os.Chmod(filepath.Join(k.Work, takeInToolPath), 0o700); err != nil { //nolint:gosec // the test needs an executable file
					t.Fatal(err)
				}
			}
			commitMerge(t, k, nil)
			_, err = push(t, k, b)
			if tc.ok {
				if err != nil {
					t.Fatalf("MergemainPush: %v", err)
				}
				return
			}
			wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
		})
	}
}

// TestMergemainBeginMergeRestoreFlow runs the stop path of /mergemain end to
// end: a merge the agent gives up on is restored to the base head, and a new
// begin then starts from the same state.
func TestMergemainBeginMergeRestoreFlow(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	baseHead, _ := conflictSetup(k)
	b, _, err := merge(t, k)
	if err != nil {
		t.Fatalf("MergemainMerge: %v", err)
	}
	// The agent gives up on the conflict without resolving it.
	if _, err := k.Tool().MergemainRestore(t.Context(), mustPR(t), b.HeadBranch, b.BaseHead, b.Main); err != nil {
		t.Fatalf("MergemainRestore: %v", err)
	}
	assertRestored(t, k, baseHead)
	if again := begin(t, k); again != b {
		t.Fatalf("begin after the restore = %+v, want %+v", again, b)
	}
}

func TestMergemainBeginRefusesSubdirectory(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	k.Setup(baseFiles(), map[string]string{"b.md": "b\n"}, map[string]string{"m.md": "m\n"})
	sub := k.ToolWith(cmdrunner.NewOS(cmdrunner.Options{Dir: filepath.Join(k.Work, "keep"), Env: k.env}))
	_, err := sub.MergemainBegin(t.Context(), mustPR(t))
	wantStop(t, err, MergemainStopPremise, RestoreNotNeeded)
	if len(k.GHCalls) != 0 {
		t.Fatalf("gh ran from a subdirectory: %q", k.GHCalls)
	}
	// Restore stops with its one reason, other_changes.
	_, err = sub.MergemainRestore(t.Context(), mustPR(t), takeInBranch, k.Git("rev-parse", "HEAD"), k.Git("rev-parse", "origin/main"))
	wantStop(t, err, MergemainStopOtherChanges, RestoreNotNeeded)
}

func TestMergemainRefusesOtherMergeDrivers(t *testing.T) {
	t.Parallel()
	conflicting := func(k *takeIn, base map[string]string) {
		k.Setup(base, map[string]string{"main.go": "package main\n\nfunc main() { b() }\n"}, map[string]string{"main.go": "package main\n\nfunc main() { m() }\n"})
	}
	t.Run("committed union driver", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		base := baseFiles()
		base[".gitattributes"] = "main.go merge=union\n"
		conflicting(k, base)
		_, _, err := merge(t, k)
		stop := wantStop(t, err, MergemainStopOutOfScope, RestoreNotNeeded)
		if got := classes(stop.Conflicts); !maps.Equal(got, map[string]PathConflictClass{"main.go": PathConflictOutOfScope}) {
			t.Fatalf("conflicts = %v", got)
		}
		if _, inMerge, _ := k.Tool().mergeHead(t.Context()); inMerge {
			t.Fatal("a merge started")
		}
	})
	for name, change := range map[string]func(t *testing.T, k *takeIn){
		"info attributes": func(t *testing.T, k *takeIn) {
			writeWork(t, k, ".git/info/attributes", "main.go merge=keep\n")
		},
		"default driver": func(_ *testing.T, k *takeIn) {
			k.Git("config", "merge.default", "keep")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			k := newTakeIn(t)
			conflicting(k, baseFiles())
			k.Git("config", "merge.keep.driver", "true")
			change(t, k)
			_, _, err := merge(t, k)
			wantStop(t, err, MergemainStopPremise, RestoreNotNeeded)
		})
	}
	t.Run("push of a union merge after main drops the rule", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		base := baseFiles()
		base[".gitattributes"] = "main.go merge=union\n"
		// The merge commit no longer has the rule, but the merge of the
		// parents takes the base head's attributes, which still have it.
		k.Setup(base, map[string]string{"main.go": "package main\n\nfunc main() { b() }\n"},
			map[string]string{"main.go": "package main\n\nfunc main() { m() }\n", ".gitattributes": "*.log -diff\n"})
		b := begin(t, k)
		k.Git("merge", "-q", "--no-ff", "--no-edit", b.Main)
		_, err := push(t, k, b)
		wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
	})
	t.Run("push of a union merge main's rule allows", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		// Main adds the rule, so the base head's attributes do not have it
		// and the merge commit's do.
		k.Setup(baseFiles(), map[string]string{"main.go": "package main\n\nfunc main() { b() }\n"},
			map[string]string{"main.go": "package main\n\nfunc main() { m() }\n", ".gitattributes": "*.log\nmain.go merge=union\n"})
		b := begin(t, k)
		k.Git("merge", "-q", "--no-ff", "--no-commit", "-s", "ours", b.Main)
		k.Git("checkout", b.Main, "--", ".gitattributes")
		writeWork(t, k, "main.go", "package main\n\nfunc main() { b() }\nfunc main() { m() }\n")
		k.Git("add", "main.go")
		k.Git("commit", "-q", "--no-edit", "--no-verify")
		_, err := push(t, k, b)
		wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
	})
	t.Run("push of a union merge", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		base := baseFiles()
		base[".gitattributes"] = "main.go merge=union\n"
		conflicting(k, base)
		b := begin(t, k)
		// The merge the tool refuses, made by hand: git's union driver
		// resolves the Go conflict, and the tree matches the computed merge.
		k.Git("merge", "-q", "--no-ff", "--no-edit", b.Main)
		_, err := push(t, k, b)
		wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
	})
}

func TestMergemainBeginAcceptsGHDefaultOfOrigin(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	k.Setup(baseFiles(), map[string]string{"b.md": "b\n"}, map[string]string{"m.md": "m\n"})
	k.Git("config", "remote.origin.gh-resolved", "base")
	begin(t, k)
}

func TestMergemainBlockingPathIgnoresLetterCase(t *testing.T) {
	t.Parallel()
	tracked := map[string]struct{}{"Local/notes.txt": {}}
	for _, ignored := range []string{"local/notes.txt", "LOCAL/", "local/notes.txt/x"} {
		if _, ok := blockingPath([]string{ignored}, tracked); !ok {
			t.Errorf("blockingPath(%q) = false, want the case variant to block", ignored)
		}
	}
	if _, ok := blockingPath([]string{"other/notes.txt"}, tracked); ok {
		t.Error("blockingPath blocked an unrelated path")
	}
}

func TestMergemainReadsNoAttributesOutsideTheBaseHead(t *testing.T) {
	t.Parallel()
	conflicting := func(k *takeIn) {
		k.Setup(baseFiles(), map[string]string{"main.go": "package main\n\nfunc main() { b() }\n"}, map[string]string{"main.go": "package main\n\nfunc main() { m() }\n"})
	}
	t.Run("ignored .gitattributes at merge", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		conflicting(k)
		writeWork(t, k, ".git/info/exclude", ".gitattributes\n")
		writeWork(t, k, ".gitattributes", "main.go merge=union\n")
		_, _, err := merge(t, k)
		stop := wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
		if got := classes(stop.Conflicts); !maps.Equal(got, map[string]PathConflictClass{"main.go": PathConflictOutOfScope}) {
			t.Fatalf("conflicts = %v", got)
		}
	})
	t.Run("untracked .gitattributes at push", func(t *testing.T) {
		t.Parallel()
		k := newTakeIn(t)
		conflicting(k)
		b, _, err := merge(t, k)
		wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
		writeWork(t, k, "main.go", "package main\n\nfunc main() { b() }\nfunc main() { m() }\n")
		k.Git("add", "main.go")
		k.Git("commit", "-q", "--no-edit", "--no-verify")
		writeWork(t, k, ".gitattributes", "main.go merge=union\n")
		_, err = push(t, k, b)
		wantStop(t, err, MergemainStopOutOfScope, RestoreNeeded)
	})
}

func TestMergemainMergeIgnoresConfiguredMergeOptions(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	conflictSetup(k)
	k.Git("config", "branch."+takeInBranch+".mergeOptions", "-Xours")
	_, r, err := merge(t, k)
	if err != nil {
		t.Fatalf("MergemainMerge: %v", err)
	}
	if got := classes(r.Conflicts); !maps.Equal(got, map[string]PathConflictClass{"doc.md": PathConflictMarkdown}) {
		t.Fatalf("conflicts = %v, want the doc.md conflict -Xours would hide", got)
	}
}

func TestMergemainPushAfterOriginHasItAsksNoRestore(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	b := pushFlow(t, k)
	// origin has the merge commit, but GitHub still names the base head as
	// the PR head.
	k.PR.HeadOID = b.BaseHead
	k.Git("push", "-q", "origin", "HEAD:refs/heads/landed")
	k.OriginGit("update-ref", headsPrefix+takeInBranch, k.Git("rev-parse", "HEAD"))
	_, err := push(t, k, b)
	wantStop(t, err, MergemainStopPremise, RestoreNotNeeded)
}

func TestMergemainPushAfterItLandedAsksNoRestore(t *testing.T) {
	t.Parallel()
	k := newTakeIn(t)
	b := pushFlow(t, k)
	// An earlier push landed but its result was not read: origin and the
	// PR head are the merge commit, and the remote-tracking ref is not.
	k.Git("push", "-q", "origin", "HEAD:refs/heads/landed")
	k.OriginGit("update-ref", headsPrefix+takeInBranch, k.Git("rev-parse", "HEAD"))
	_, err := push(t, k, b)
	wantStop(t, err, MergemainStopPremise, RestoreNotNeeded)
}

// TestNewMergemainIgnoresTheCallersGitEnvironment verifies that the tool the
// command builds reads the repository of the work tree even when the caller's
// environment names another one. Every git call drops the GIT_* variables, so
// a GIT_DIR, GIT_WORK_TREE, or GIT_CONFIG_GLOBAL set by the caller cannot
// point a take-in check at another repository.
func TestNewMergemainIgnoresTheCallersGitEnvironment(t *testing.T) {
	r := newGitRepo(t)
	head := r.Commit("initial", map[string]string{"a.md": "a\n"})
	r.Git("update-ref", "refs/heads/"+mainBranch, head)
	t.Chdir(r.Dir)
	elsewhere := t.TempDir()
	t.Setenv("GIT_DIR", filepath.Join(elsewhere, "git"))
	t.Setenv("GIT_WORK_TREE", elsewhere)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(elsewhere, "config"))
	tool := NewMergemain(os.LookupEnv)
	got, err := tool.git.resolveCommit(t.Context(), "HEAD")
	if err != nil {
		t.Fatalf("resolveCommit with the caller's GIT_* set: %v", err)
	}
	if got != head {
		t.Errorf("resolveCommit = %s, want %s", got, head)
	}
}
