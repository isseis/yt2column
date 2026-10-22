//go:build test

package fixprguard

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	repoOrigin   = "file:.git/config"
	globalOrigin = "file:/home/u/.gitconfig"
	systemOrigin = "file:/etc/gitconfig"
	// includeOrigin is a file read through include.path or includeIf.
	includeOrigin = "file:/home/u/included.cfg"
)

// listed is one entry of git config --list --show-origin -z. A nil value is a
// setting without a value, as git lists a bare "[core] bare".
type listed struct {
	origin, name string
	value        *string
}

func set(origin, name, value string) listed { return listed{origin, name, &value} }

func bare(origin, name string) listed { return listed{origin: origin, name: name} }

// configOutput renders entries as git config --list --show-origin -z does.
func configOutput(entries ...listed) []byte {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.origin + "\x00" + e.name)
		if e.value != nil {
			b.WriteString("\n" + *e.value)
		}
		b.WriteString("\x00")
	}
	return []byte(b.String())
}

// Positions in baseEntries that rows change; TestConfigListDigestClassification
// checks each holds the entry it names.
const (
	iFetchHeads = 3
	iFetchTags  = 4
	iCurrent    = 5
	iMain       = 6
	iOther      = 7
	iEnd        = 9 // the command-line entry, before which rows insert
)

// currentBranchKey is testBranch's remote setting.
var currentBranchKey = branchSection + "." + testBranch + ".remote"

// baseEntries is a listing with a compared entry of every kind and a branch
// setting of a branch other than main and testBranch. The two
// remote.origin.fetch entries pin that a repeated name is accepted.
func baseEntries() []listed {
	return []listed{
		set(systemOrigin, "core.autocrlf", "false"),
		set(repoOrigin, "core.bare", "false"),
		set(repoOrigin, "remote.origin.url", "/srv/remote.git"),
		set(repoOrigin, "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"),
		set(repoOrigin, "remote.origin.fetch", "+refs/tags/*:refs/tags/*"),
		set(repoOrigin, currentBranchKey, "origin"),
		set(repoOrigin, "branch.main.remote", "origin"),
		set(repoOrigin, "branch.other.remote", "origin"),
		set(repoOrigin, "branch.other.merge", "refs/heads/other"),
		set("command line:", "gc.auto", "0"),
	}
}

func mustConfigListDigest(t *testing.T, out []byte, branch string) string {
	t.Helper()
	got, err := configListDigest(out, digestString(branch))
	if err != nil {
		t.Fatalf("configListDigest = %v", err)
	}
	return got
}

func TestConfigListDigestClassification(t *testing.T) {
	base := baseEntries()
	insert := func(at int, e ...listed) []listed { return slices.Insert(slices.Clone(base), at, e...) }
	replace := func(at int, e listed) []listed {
		c := slices.Clone(base)
		c[at] = e
		return c
	}
	remove := func(at int) []listed { return slices.Delete(slices.Clone(base), at, at+1) }
	for at, name := range map[int]string{
		iFetchHeads: "remote.origin.fetch", iFetchTags: "remote.origin.fetch", iCurrent: currentBranchKey,
		iMain: "branch.main.remote", iOther: "branch.other.remote", iEnd: "gc.auto",
	} {
		if base[at].name != name {
			t.Fatalf("base[%d] = %q, want %q", at, base[at].name, name)
		}
	}
	// Names that overlap testBranch: a longer subsection, and one differing
	// only in case.
	extended := testBranch + ".a"
	recased := strings.ToUpper(testBranch[:1]) + testBranch[1:]
	// Each row changes base; ignored rows must leave the digest unchanged and
	// compared rows must change it.
	for _, tc := range []struct {
		name    string
		branch  string // the baseline branch; testBranch when empty
		before  []listed
		after   []listed
		ignored bool
	}{
		// Branch settings of other branches, from any origin.
		{name: "other_branch_added", after: insert(iOther+1, set(repoOrigin, "branch.x.remote", "origin")), ignored: true},
		{name: "other_branch_changed", after: replace(iOther, set(repoOrigin, "branch.other.remote", "upstream")), ignored: true},
		{name: "other_branch_removed", after: remove(iOther), ignored: true},
		{name: "other_branch_section_added", after: insert(iEnd, set(repoOrigin, "branch.x.remote", "origin"), set(repoOrigin, "branch.x.merge", "refs/heads/x")), ignored: true},
		{name: "other_branch_section_removed", after: slices.Delete(slices.Clone(base), iOther, iOther+2), ignored: true},
		{name: "empty_subsection_added", after: insert(iEnd, set(repoOrigin, "branch..remote", "origin")), ignored: true},
		{name: "other_branch_without_value", after: insert(iEnd, bare(repoOrigin, "branch.x.remote")), ignored: true},
		{name: "other_branch_value_with_lf", after: insert(iEnd, set(repoOrigin, "branch.x.description", "line\nline")), ignored: true},
		{name: "other_branch_in_global", after: insert(1, set(globalOrigin, "branch.x.remote", "origin")), ignored: true},
		{name: "other_branch_in_system", after: insert(1, set(systemOrigin, "branch.x.remote", "origin")), ignored: true},
		{name: "other_branch_in_include", after: insert(iEnd, set(includeOrigin, "branch.x.remote", "origin")), ignored: true},
		{name: "other_branch_in_worktree_config", after: insert(iEnd, set("file:.git/worktrees/w/config.worktree", "branch.x.remote", "origin")), ignored: true},
		// The subsection is feature.a, not feature: the overlap with the
		// current branch's name does not make it the current branch.
		{name: "subsection_extends_current_branch", after: insert(iEnd, set(repoOrigin, "branch."+extended+".remote", "origin")), ignored: true},
		{name: "subsection_differs_in_case", after: insert(iEnd, set(repoOrigin, "branch."+recased+".remote", "origin")), ignored: true},
		{name: "main_differs_in_case", after: insert(iEnd, set(repoOrigin, "branch.Main.remote", "origin")), ignored: true},

		// The current branch, main, two-level branch settings, and every
		// other section stay compared.
		{name: "current_branch_added", after: insert(iCurrent+1, set(repoOrigin, "branch."+testBranch+".merge", "refs/heads/"+testBranch))},
		{name: "current_branch_changed", after: replace(iCurrent, set(repoOrigin, currentBranchKey, "upstream"))},
		{name: "current_branch_removed", after: remove(iCurrent)},
		{name: "current_branch_without_value", after: replace(iCurrent, bare(repoOrigin, currentBranchKey))},
		{name: "current_branch_with_dot", branch: extended, after: insert(iEnd, set(repoOrigin, "branch."+extended+".pushremote", "upstream"))},
		{name: "main_added", after: insert(iMain+1, set(repoOrigin, "branch.main.pushremote", "upstream"))},
		{name: "main_changed", after: replace(iMain, set(repoOrigin, "branch.main.remote", "upstream"))},
		{name: "main_removed", after: remove(iMain)},
		{name: "two_level_branch_added", after: insert(iEnd, set(repoOrigin, "branch.autosetupmerge", "always"))},
		{name: "branch_prefixed_section_added", after: insert(iEnd, set(repoOrigin, "branchx.a.b", "c"))},
		{name: "remote_push_default_added", after: insert(iEnd, set(repoOrigin, "remote.pushdefault", "upstream"))},
		{name: "remote_added", after: insert(iEnd, set(repoOrigin, "remote.upstream.url", "/srv/other.git"))},
		{name: "include_added", after: insert(iEnd, set(repoOrigin, "include.path", "included.cfg"))},
		{name: "compared_value_changed_after_lf", before: insert(iEnd, set(repoOrigin, "core.x", "a\nb")), after: insert(iEnd, set(repoOrigin, "core.x", "a\nc"))},
		{name: "empty_value_to_no_value", before: insert(iEnd, set(repoOrigin, "core.x", "")), after: insert(iEnd, bare(repoOrigin, "core.x"))},
		{name: "origin_only_changed", after: replace(1, set(globalOrigin, "core.bare", "false"))},
		{name: "multi_value_order_swapped", after: func() []listed {
			c := slices.Clone(base)
			c[iFetchHeads], c[iFetchTags] = c[iFetchTags], c[iFetchHeads]
			return c
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			branch := tc.branch
			if branch == "" {
				branch = testBranch
			}
			before := tc.before
			if before == nil {
				before = base
			}
			beforeOut, afterOut := configOutput(before...), configOutput(tc.after...)
			// Without classification the change is visible, so an unchanged
			// digest can only come from the classification.
			if bytes.Equal(beforeOut, afterOut) {
				t.Fatal("the row does not change the output")
			}
			same := mustConfigListDigest(t, beforeOut, branch) == mustConfigListDigest(t, afterOut, branch)
			if same != tc.ignored {
				t.Errorf("digest unchanged = %v, want %v", same, tc.ignored)
			}
		})
	}
}

func TestConfigListDigestRejects(t *testing.T) {
	valid := string(configOutput(set(repoOrigin, "core.bare", "false")))
	// Each input carries "secret" in its origin, name or value, so the error
	// text can be checked for it. Each row breaks one rule only, but two pairs
	// share a check: an empty name has no "." either, and output with no NUL
	// after an origin ends after it.
	for _, tc := range []struct {
		name string
		out  string
	}{
		{"empty_output", ""},
		{"no_nul_at_all", "file:secret"},
		{"empty_origin", "\x00core.secret\nv\x00"},
		{"empty_origin_after_entry", valid + "\x00core.secret\nv\x00"},
		{"ends_after_origin", valid + "file:secret\x00"},
		{"value_without_final_nul", valid + "file:x\x00core.bare\nsecret"},
		{"empty_name", "file:secret\x00\nv\x00"},
		{"name_without_dot", "file:x\x00secret\nv\x00"},
		{"empty_section", "file:x\x00.secret\nv\x00"},
		{"empty_variable", "file:x\x00secret.\nv\x00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := configListDigest([]byte(tc.out), digestString(testBranch))
			if !errors.Is(err, errConfigList) || !errors.Is(err, ErrGitFailed) {
				t.Fatalf("configListDigest = %v, want errConfigList wrapping ErrGitFailed", err)
			}
			if got != "" {
				t.Errorf("digest = %q, want none on rejection", got)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Errorf("error %q carries a value from the input", err)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		digest string
	}{
		{"empty", ""},
		{"63_digits", strings.Repeat("a", digestLength-1)},
		{"65_digits", strings.Repeat("a", digestLength+1)},
		{"uppercase", strings.ToUpper(digestString(testBranch))},
		{"not_hex", strings.Repeat("g", digestLength)},
	} {
		t.Run("branch_digest/"+tc.name, func(t *testing.T) {
			got, err := configListDigest([]byte(valid), tc.digest)
			if !errors.Is(err, errConfigList) {
				t.Fatalf("configListDigest = %v, want errConfigList", err)
			}
			if got != "" {
				t.Errorf("digest = %q, want none on rejection", got)
			}
		})
	}
}
