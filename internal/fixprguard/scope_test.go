//go:build test

package fixprguard

import (
	"testing"
)

func TestScopeRules(t *testing.T) {
	trusted := func(p string) []ThreadPath { return []ThreadPath{{Path: p}} }
	named := func(p string) []ThreadPath { return []ThreadPath{{Path: p, Named: true}} }
	for _, tc := range []struct {
		name    string
		changed string
		kind    ChangeKind
		threads []ThreadPath
		want    bool
	}{
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
		{"nested_dotfile_not_root_rule", "docs/.keep", ChangeAdd, trusted("docs/a.md"), true},
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
		{"dot_dot", "docs/../docs/a.md", ChangeModify, trusted("docs/../docs/a.md"), false},
		{"not_clean", "docs//a.md", ChangeModify, trusted("docs//a.md"), false},
		{"control_character", "docs/a\x01.md", ChangeModify, trusted("docs/a\x01.md"), false},
		{"c1_control_character", "docs/a\u0085.md", ChangeModify, trusted("docs/a\u0085.md"), false},
		{"invalid_utf8", "docs/a\xff.md", ChangeModify, trusted("docs/a\xff.md"), false},
		{"invalid_thread_path", "docs/b.md", ChangeModify, trusted("docs/../a.md"), false},
		{"non_ascii_exact", "docs/é.md", ChangeModify, trusted("docs/é.md"), true},
		{"non_ascii_beside_path", "docs/é.md", ChangeAdd, trusted("docs/a.md"), false},
		// U+212A KELVIN SIGN folds to k on some file systems.
		{"kelvin_sign", "docs/K.md", ChangeAdd, trusted("docs/k.md"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := inScope(tc.changed, tc.kind, tc.threads); got != tc.want {
				t.Errorf("inScope(%q, %v) = %v, want %v", tc.changed, tc.kind, got, tc.want)
			}
		})
	}
}
