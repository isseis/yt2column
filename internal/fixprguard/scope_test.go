//go:build test

package fixprguard

import (
	"slices"
	"strings"
	"testing"
)

type scopeCase struct {
	name    string
	changed string
	kind    ChangeKind
	threads []ThreadPath
	want    bool
}

func trustedThread(p string) []ThreadPath { return []ThreadPath{{Path: p}} }

func namedThread(p string) []ThreadPath { return []ThreadPath{{Path: p, Named: true}} }

// denyListAtPathItself returns, for each path, the pair that shows it is on the
// deny list where the allow list has no effect: as the thread's path itself it
// is in scope for a trusted thread and out of scope for a named one. A path
// that is not on the deny list would be in scope for both.
func denyListAtPathItself(paths ...string) []scopeCase {
	var cases []scopeCase
	for _, p := range paths {
		cases = append(cases,
			scopeCase{"trusted_itself:" + p, p, ChangeAdd, trustedThread(p), true},
			scopeCase{"named_itself:" + p, p, ChangeAdd, namedThread(p), false},
		)
	}
	return cases
}

func TestScopeRules(t *testing.T) {
	trusted, named := trustedThread, namedThread
	cases := []scopeCase{
		// Rules 1 to 3.
		{"path_itself", "docs/a.md", ChangeModify, trusted("docs/a.md"), true},
		{"same_directory", "docs/b.md", ChangeAdd, trusted("docs/a.md"), true},
		{"same_directory_delete", "docs/b.md", ChangeDelete, trusted("docs/a.md"), true},
		{"subdirectory", "docs/sub/b.md", ChangeModify, trusted("docs/a.md"), false},
		{"parent_directory", "b.md", ChangeModify, trusted("docs/a.md"), false},
		{"other_directory", "internal/b.md", ChangeModify, trusted("docs/a.md"), false},
		{"task_directory_depth", "docs/tasks/0030_x/v1/notes.md", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), true},
		{"other_task_directory", "docs/tasks/0031_y/a.md", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), false},
		{"task_number_not_digits", "docs/tasks/003x_x/v1/a.md", ChangeAdd, trusted("docs/tasks/003x_x/a.md"), false},
		{"task_number_short", "docs/tasks/030_x/v1/a.md", ChangeAdd, trusted("docs/tasks/030_x/a.md"), false},
		{"task_without_name", "docs/tasks/0030_/v1/a.md", ChangeAdd, trusted("docs/tasks/0030_/a.md"), false},
		{"no_threads", "docs/a.md", ChangeModify, nil, false},
		{"second_thread", "internal/b.md", ChangeModify, []ThreadPath{{Path: "docs/a.md"}, {Path: "internal/a.md"}}, true},

		// Rule 4: execution-related files.
		{"makefile_is_path", "Makefile", ChangeModify, trusted("Makefile"), true},
		{"makefile_beside_path", "docs/Makefile", ChangeAdd, trusted("docs/a.md"), false},
		{"makefile_lowercase", "docs/makefile", ChangeAdd, trusted("docs/a.md"), false},
		{"gnumakefile", "docs/GNUmakefile", ChangeAdd, trusted("docs/a.md"), false},
		{"mk_suffix", "docs/rules.mk", ChangeAdd, trusted("docs/a.md"), false},
		{"go_mod", "docs/go.mod", ChangeAdd, trusted("docs/a.md"), false},
		{"go_sum", "go.sum", ChangeModify, trusted("README.md"), false},
		{"go_work", "docs/go.work", ChangeAdd, trusted("docs/a.md"), false},
		{"go_work_sum", "docs/go.work.sum", ChangeAdd, trusted("docs/a.md"), false},
		{"tool_versions", "docs/.tool-versions", ChangeAdd, trusted("docs/a.md"), false},
		{"mise", "docs/.mise.toml", ChangeAdd, trusted("docs/a.md"), false},
		{"claude_dir", ".claude/commands/b.md", ChangeModify, trusted(".claude/commands/a.md"), false},
		{"github_dir", ".github/workflows/b.yml", ChangeModify, trusted(".github/workflows/a.yml"), false},
		{"extension_scripts", "extension/scripts/b.mjs", ChangeModify, trusted("extension/scripts/a.mjs"), false},
		{"root_dotfile", ".golangci.yml", ChangeModify, trusted("README.md"), false},
		{"root_claude_md", "CLAUDE.md", ChangeModify, trusted("README.md"), false},
		{"root_agents_md_case", "agents.MD", ChangeAdd, trusted("README.md"), false},
		{"nested_dotfile_not_root_rule", "docs/.keep", ChangeAdd, trusted("docs/a.md"), false},
		{"extension_package_json", "extension/package.json", ChangeModify, trusted("extension/README.md"), false},
		{"extension_tsconfig", "extension/tsconfig.build.json", ChangeModify, trusted("extension/README.md"), false},
		{"extension_other_file", "extension/notes.md", ChangeModify, trusted("extension/README.md"), true},
		{"named_execution_path_itself", "Makefile", ChangeModify, named("Makefile"), false},
		{"task_directory_makefile", "docs/tasks/0030_x/Makefile", ChangeAdd, trusted("docs/tasks/0030_x/a.md"), false},

		// The .go rule.
		{"go_same_package", "internal/x/x_test.go", ChangeAdd, trusted("internal/x/x.go"), true},
		{"go_beside_non_go_path", "docs/x_test.go", ChangeAdd, trusted("docs/a.md"), false},
		{"go_under_task_directory", "docs/tasks/0030_x/v1/x.go", ChangeAdd, trusted("docs/tasks/0030_x/a.md"), false},
		{"go_other_package", "internal/y/y.go", ChangeModify, trusted("internal/x/x.go"), false},
		{"go_upper_case_suffix", "docs/x.GO", ChangeAdd, trusted("docs/a.md"), false},
		{"named_go_path_itself", "internal/x/x.go", ChangeModify, named("internal/x/x.go"), true},
		{"named_go_same_package", "internal/x/x_test.go", ChangeAdd, named("internal/x/x.go"), false},
		{"named_same_directory", "docs/b.md", ChangeAdd, named("docs/a.md"), true},

		// Rule 6 and the kinds that are never in scope.
		{"other_kind", "docs/a.md", ChangeOther, trusted("docs/a.md"), false},
		{"unknown_kind", "docs/a.md", ChangeUnknown, trusted("docs/a.md"), false},

		// Path checks.
		{"absolute", "/docs/a.md", ChangeModify, trusted("/docs/a.md"), false},
		{"dot_dot", "../a.md", ChangeModify, trusted("../a.md"), false},
		{"not_clean", "docs//a.md", ChangeModify, trusted("docs//a.md"), false},
		{"control_character", "docs/a\x01.md", ChangeModify, trusted("docs/a\x01.md"), false},
		{"c1_control_character", "docs/a\u0085.md", ChangeModify, trusted("docs/a\u0085.md"), false},
		{"invalid_utf8", "docs/a\xff.md", ChangeModify, trusted("docs/a\xff.md"), false},
		{"invalid_thread_path", "b.md", ChangeModify, trusted("../a.md"), false},
		{"non_ascii_exact", "docs/é.md", ChangeModify, trusted("docs/é.md"), true},
		{"non_ascii_beside_path", "docs/é.md", ChangeAdd, trusted("docs/a.md"), false},
		// U+212A KELVIN SIGN folds to k on some file systems.
		{"kelvin_sign", "docs/K.md", ChangeAdd, trusted("docs/k.md"), false},

		// Instruction files: each name as a .md beside or under the thread's
		// path, which the allow list alone would admit.
		{"instruction_claude_md", "internal/writer/CLAUDE.md", ChangeAdd, trusted("internal/writer/README.md"), false},
		{"instruction_agents_md", "internal/writer/AGENTS.md", ChangeAdd, trusted("internal/writer/README.md"), false},
		{"instruction_gemini_md", "internal/writer/Gemini.md", ChangeAdd, trusted("internal/writer/README.md"), false},
		{"instruction_agent_md", "internal/writer/agent.md", ChangeAdd, trusted("internal/writer/README.md"), false},
		// Also a row of issue #425.
		{"instruction_claude_local_md", "CLAUDE.local.md", ChangeAdd, trusted("README.md"), false},
		{"instruction_agents_override_md", "docs/tasks/0030_x/sub/agents.override.md", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), false},
		// Folded for the deny list; the lower-case .md would pass the
		// case-sensitive allow list.
		{"instruction_mixed_case", "internal/x/Claude.md", ChangeAdd, trusted("internal/x/README.md"), false},

		// Paths with an element that starts with ".", as a .md beside or under
		// the thread's path.
		// Also a row of issue #425.
		{"dot_directory_under_task", "docs/tasks/0030_x/.claude/commands/evil.md", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), false},
		{"dot_file_under_task", "docs/tasks/0030_x/.notes.md", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), false},
		{"dot_file_same_directory", "docs/.notes.md", ChangeAdd, trusted("docs/a.md"), false},

		// The allow list: what rules 2 and 3 still admit.
		{"allow_md_under_task", "docs/tasks/0030_x/02_architecture.md", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), true},
		{"allow_md_under_task_named", "docs/tasks/0030_x/02_architecture.md", ChangeAdd, named("docs/tasks/0030_x/01_requirements.md"), true},
		{"allow_txt_under_task", "docs/tasks/0030_x/sub/notes.txt", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), true},
		{"allow_txt_under_task_named", "docs/tasks/0030_x/sub/notes.txt", ChangeAdd, named("docs/tasks/0030_x/01_requirements.md"), true},
		{"allow_testdata", "internal/x/testdata/b.json", ChangeAdd, trusted("internal/x/testdata/a.json"), true},
		{"allow_testdata_named", "internal/x/testdata/b.json", ChangeAdd, named("internal/x/testdata/a.json"), true},
		{"allow_testdata_deep_under_task", "docs/tasks/0030_x/a/testdata/b/c.json", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), true},

		// Outside the allow list: out of scope beside the thread's path.
		{"outside_ts_test", "extension/src/a.test.ts", ChangeAdd, trusted("extension/src/a.ts"), false},
		{"outside_tmpl", "prompts/user.tmpl", ChangeModify, trusted("prompts/system.tmpl"), false},
		{"outside_sh_under_task", "docs/tasks/0032_japanese_pr_title/check_rules.sh", ChangeModify, trusted("docs/tasks/0032_japanese_pr_title/01_requirements.md"), false},
		{"outside_json", "internal/x/data.json", ChangeAdd, trusted("internal/x/x.go"), false},
		{"outside_no_extension", "LICENSE", ChangeModify, trusted("README.md"), false},
		{"outside_delete", "extension/src/b.ts", ChangeDelete, trusted("extension/src/a.ts"), false},
		{"outside_mdx", "docs/b.mdx", ChangeAdd, trusted("docs/a.md"), false},
		{"outside_md_then_sh", "docs/b.md.sh", ChangeAdd, trusted("docs/a.md"), false},
		{"outside_testdata_substring", "internal/x/mytestdata/b.json", ChangeAdd, trusted("internal/x/mytestdata/a.json"), false},
		{"outside_testdata_file_name", "internal/x/testdata", ChangeAdd, trusted("internal/x/a.md"), false},
		// The allow list does not fold case.
		{"outside_upper_md", "docs/B.MD", ChangeAdd, trusted("docs/a.md"), false},
		{"outside_upper_testdata", "internal/x/TestData/b.json", ChangeAdd, trusted("internal/x/TestData/a.json"), false},

		// Rule 1 does not consult the allow list.
		{"outside_allow_list_itself", "extension/src/a.ts", ChangeModify, trusted("extension/src/a.ts"), true},
		{"outside_allow_list_itself_named", "extension/src/a.ts", ChangeModify, named("extension/src/a.ts"), true},

		// The .go rule takes .go files before the allow list: under testdata/
		// a .go file beside a non-.go path stays out, and its case folds.
		{"go_under_testdata", "internal/x/testdata/gen.go", ChangeAdd, trusted("internal/x/testdata/a.json"), false},
		{"go_under_task_testdata", "docs/tasks/0030_x/testdata/gen.go", ChangeAdd, trusted("docs/tasks/0030_x/01_requirements.md"), false},
		{"go_upper_case_same_package", "internal/x/y.GO", ChangeAdd, trusted("internal/x/x.go"), true},
		{"go_upper_case_under_testdata", "internal/x/testdata/gen.GO", ChangeAdd, trusted("internal/x/testdata/a.json"), false},

		// The rows of issue #425 (the CLAUDE.local.md and .claude/ rows are
		// above).
		{"issue_claude_md", "internal/writer/CLAUDE.md", ChangeAdd, trusted("internal/writer/writer.go"), false},
		{"issue_agents_md", "internal/writer/AGENTS.md", ChangeAdd, trusted("internal/writer/writer.go"), false},
		{"issue_named_claude_md_under_task", "docs/tasks/0030_x/CLAUDE.md", ChangeAdd, named("docs/tasks/0030_x/01_requirements.md"), false},
		{"issue_syso", "internal/writer/foo.syso", ChangeAdd, trusted("internal/writer/writer.go"), false},
		{"issue_asm", "internal/writer/foo_amd64.s", ChangeAdd, trusted("internal/writer/writer.go"), false},
		{"issue_c", "internal/writer/foo.c", ChangeAdd, trusted("internal/writer/writer.go"), false},
		{"issue_gitattributes", "internal/writer/.gitattributes", ChangeAdd, trusted("internal/writer/writer.go"), false},
		{"issue_gitignore", "internal/writer/.gitignore", ChangeAdd, trusted("internal/writer/writer.go"), false},
	}
	cases = slices.Concat(cases,
		// Instruction files, in any directory and any case.
		denyListAtPathItself(
			"internal/writer/CLAUDE.md", "internal/writer/agents.md", "internal/writer/claude.local.md",
			"internal/writer/Agent.md", "internal/writer/gemini.md", "internal/writer/agents.override.md",
			"internal/x/Claude.MD",
		),
		// Paths with an element that starts with ".", in any directory.
		denyListAtPathItself("internal/writer/.gitignore", "internal/.config/foo.md"),
		// Go build inputs, in any directory and any case.
		denyListAtPathItself(
			"internal/writer/foo.c", "internal/writer/foo.h", "internal/writer/foo.cc", "internal/writer/foo.cpp",
			"internal/writer/foo.cxx", "internal/writer/foo.hh", "internal/writer/foo.hpp", "internal/writer/foo.hxx",
			"internal/writer/foo.m", "internal/writer/foo.s", "internal/writer/foo.sx", "internal/writer/foo.f",
			"internal/writer/foo.for", "internal/writer/foo.f90", "internal/writer/foo.swig",
			"internal/writer/foo.swigcxx", "internal/writer/foo.syso",
			"internal/writer/foo.HPP", "internal/writer/foo.S", "internal/writer/foo.F",
		),
		// The definitions kept from before, and examples of the ones the "."
		// rule and the instruction-file names now cover.
		denyListAtPathItself(
			"Makefile", "docs/GNUmakefile", "internal/x/go.mod", "go.sum", "go.work", "go.work.sum",
			"build/rules.mk", "extension/scripts/has-extension-changes.sh",
			"extension/package.json", "extension/package-lock.json", "extension/eslint.config.js",
			"extension/tsconfig.json", "extension/tsconfig.core.json", "extension/tsconfig.build.json",
			".github/workflows/ci.yml", ".claude/commands/a.md", ".tool-versions", ".mise.toml",
			".golangci.yml", "CLAUDE.md", "extension/.npmrc", "extension/.node-version",
			"extension/.prettierrc.json", "extension/.prettierignore",
		),
		// The rows of issue #425 as the thread's path itself.
		denyListAtPathItself(
			"internal/writer/AGENTS.md", "CLAUDE.local.md", "docs/tasks/0030_x/.claude/commands/evil.md",
			"docs/tasks/0030_x/CLAUDE.md", "internal/writer/foo_amd64.s", "internal/writer/.gitattributes",
		),
	)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inScope(tc.changed, tc.kind, tc.threads); got != tc.want {
				t.Errorf("inScope(%q, %v) = %v, want %v", tc.changed, tc.kind, got, tc.want)
			}
		})
	}
}

// TestPatternRuleLiterals: ".*" and "*.go" stand for rules that hold no data,
// so the pattern functions write them as literals. The guide guard only checks
// that what the functions print is in the guide, and would pass if a literal
// went missing. Every printed pattern must also be one the matchers enforce,
// so the printed lists cannot drift from the rules.
func TestPatternRuleLiterals(t *testing.T) {
	if !slices.Contains(ExecutionFilePatterns(), ".*") {
		t.Errorf("ExecutionFilePatterns() = %q, want it to list .*", ExecutionFilePatterns())
	}
	if !slices.Contains(AllowListPatterns(), "*.go") {
		t.Errorf("AllowListPatterns() = %q, want it to list *.go", AllowListPatterns())
	}
	for _, pattern := range ExecutionFilePatterns() {
		if p := samplePath(pattern); !isExecutionRelated(p) {
			t.Errorf("ExecutionFilePatterns lists %q, but %q is not execution-related", pattern, p)
		}
	}
	for _, pattern := range AllowListPatterns() {
		p := samplePath(pattern)
		if pattern == "*.go" {
			if !isGoFile(p) {
				t.Errorf("AllowListPatterns lists *.go, but %q is not a .go file", p)
			}
			continue
		}
		if !inAllowList(p) {
			t.Errorf("AllowListPatterns lists %q, but %q is not on the allow list", pattern, p)
		}
	}
}

// samplePath returns a path below the repository root that pattern, in the
// guide's notation, describes.
func samplePath(pattern string) string {
	switch {
	case pattern == ".*":
		return "a/.x"
	case strings.HasPrefix(pattern, "*"):
		return "a/b" + strings.TrimPrefix(pattern, "*")
	case strings.HasSuffix(pattern, "/") && strings.Contains(strings.TrimSuffix(pattern, "/"), "/"):
		return pattern + "x"
	case strings.HasSuffix(pattern, "/"):
		return "a/" + pattern + "x"
	case strings.Contains(pattern, "/"):
		return pattern
	default:
		return "a/" + pattern
	}
}
