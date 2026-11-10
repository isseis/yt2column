#!/usr/bin/env bash
# adr-correction-candidates.sh lists the ADR files an editorial correction of
# the current fix round may name, and compares the paths /fixpr's fix agent
# returned with that list (.claude/commands/fixpr.md, Phase 3, "Correction
# list").
#
# Usage: bash adr-correction-candidates.sh [<path-file>...]
#
# Each <path-file> holds one path the fix agent returned, as its whole
# content, with no newline added. The path itself never reaches a command
# line: it is only compared, byte for byte, with the list. /fixpr runs the
# copy of this script that `git show refs/remotes/origin/main:` writes out,
# never the work tree's copy, so the fix agent cannot change the check.
#
# Contract:
#   - Inputs: git's output and the bytes of the <path-file>s only. Nothing is
#     read from standard input. Of the caller's environment only PATH, HOME,
#     and TMPDIR are kept; its GIT_ variables, exported functions, shell
#     options, and locale change nothing, and neither do the global and system
#     git configuration, the attributes, or the repository settings that
#     GIT_OVERRIDES fixes. Inputs left: bash reads the caller's BASH_ENV file
#     and imports the caller's exported functions before this script starts,
#     so what that file prints would reach the output (a caller then drops
#     every item, see below), and a function named after a builtin the first
#     lines run (exec, env) could stop the env -i run. Both are the user's own
#     environment, not a value from outside the trust boundary. The
#     repository's core.worktree and core.fileMode still apply.
#   - Location: the repository is judged from the top of its work tree,
#     whatever directory the script runs in.
#   - Reference: refs/remotes/origin/main is checked first, whether or not
#     anything changed, and the script fails when it names no commit. The
#     name is written in full: a local branch named origin/main would win the
#     short name.
#   - The list holds the paths that are, all three:
#       * changed in content between HEAD and the work tree (git's status M:
#         no addition, deletion, type change such as a file replaced by a
#         symbolic link, or rename, and no untracked file);
#       * of the form of docs/dev/conventions/adr.md, "Location and naming":
#         docs/dev/adr/NNNN-<words>.md, the words ASCII lowercase letters and
#         digits joined by single hyphens, matched on the whole path;
#       * in the tree of refs/remotes/origin/main.
#   - Comparison: a <path-file> matches when its content equals a listed path
#     byte for byte. An empty file, a file holding a newline or a NUL, and a
#     file that cannot be read match nothing.
#   - Failure: any failed git step, the last of a pipeline or not, ends the
#     script with a status other than 0.
#   - It uses no network, writes no file, and uses no bash feature newer than
#     3.2. It needs a git that accepts --attr-source (GIT_ATTR_SOURCE); with
#     an older one the variable is ignored, so the script checks the option
#     first and fails.
#
# Output and exit status:
#   - With no argument: the listed paths, one per line (the form excludes a
#     newline in a listed path); nothing when none qualifies. Status 0.
#   - With arguments: the first line is "origin-main <commit>", the full ID
#     refs/remotes/origin/main named; then one line per argument, in order:
#     "match <listed path>" or "no-match". Status 0.
#   - On a failure: a diagnostic on standard error and status 1. A caller
#     drops every ADR item on a status other than 0, on a first line of
#     another form, or on a line count other than the arguments plus one.
#
# Why bash: the script only reads git and prints, with no side effect, so it
# is not a development tool with decisions and side effects that
# docs/dev/adr/0014-deterministic-checks-in-go.md puts in Go; the comparison
# is still made by the script, not by the agent, and TestADRCorrectionCandidates
# (cmd/yt2column/adr_candidates_docs_test.go) checks each property above.

# Run again in a clean environment, so nothing the caller's environment
# carries (exported functions, SHELLOPTS, BASHOPTS, LC_ALL, GIT_ variables)
# reaches the shell or git: only PATH, HOME, and TMPDIR are kept. The marker
# is this process's ID, which exec keeps, so a caller cannot skip the run by
# presetting it; [[ is a keyword no function can replace.
if [[ ${ADR_CANDIDATES_CLEAN_ENV-} != "$$" ]]; then
	exec env -i ADR_CANDIDATES_CLEAN_ENV=$$ PATH="${PATH:-}" HOME="${HOME:-}" TMPDIR="${TMPDIR:-}" \
		"${BASH:-bash}" --noprofile --norc "$0" "$@" </dev/null
fi
unset ADR_CANDIDATES_CLEAN_ENV

set -u -o pipefail
export LC_ALL=C

# env -i already dropped the caller's variables; drop every GIT_ variable
# (GIT_DIR, GIT_WORK_TREE, GIT_CONFIG_PARAMETERS, ...) and the functions the
# script relies on again, as a second guard.
for name in ${!GIT_@}; do
	unset "$name"
done
unset -f git cmp printf
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_ATTR_NOSYSTEM=1
export GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0

# The repository settings that could change what the diff reports; the
# command line overrides the repository's configuration.
GIT_OVERRIDES=(
	-c core.fsmonitor=false
	-c core.untrackedCache=false
	-c core.attributesFile=/dev/null
	-c core.autocrlf=false
	-c core.eol=lf
	-c core.safecrlf=false
	-c core.bigFileThreshold=512m
	-c core.sparseCheckout=false
	-c diff.relative=false
	-c diff.ignoreSubmodules=none
	-c submodule.recurse=false
)

g() {
	command git "${GIT_OVERRIDES[@]}" "$@"
}

fail() {
	printf 'adr-correction-candidates: %s\n' "$1" >&2
	exit 1
}

# The ADR file name form, with every character listed instead of a range, so
# no locale can widen it.
DIGIT='[0123456789]'
WORD='[abcdefghijklmnopqrstuvwxyz0123456789]+'
ADR_RE="^docs/dev/adr/${DIGIT}${DIGIT}${DIGIT}${DIGIT}-${WORD}(-${WORD})*[.]md\$"

# keep_adr_form reads NUL-terminated paths and prints those of the ADR file
# form, one per line.
keep_adr_form() {
	local path
	while IFS= read -r -d '' path; do
		if [[ $path =~ $ADR_RE ]]; then
			printf '%s\n' "$path"
		fi
	done
}

command -v cmp >/dev/null 2>&1 || fail "cmp is not on PATH"

# The argument files are named relative to the caller's directory, so make
# them absolute before moving to the top of the work tree.
args=()
for arg in "$@"; do
	case $arg in
	/*) args+=("$arg") ;;
	*) args+=("$PWD/$arg") ;;
	esac
done

top=$(g rev-parse --show-toplevel) || fail "not inside a git work tree"
cd "$top" || fail "cannot enter the top of the work tree"

origin_main=$(g rev-parse --verify --quiet 'refs/remotes/origin/main^{commit}') ||
	fail "refs/remotes/origin/main does not name a commit"

# Every later git reads attributes from the empty tree instead of the work
# tree or the index, so no .gitattributes (an encoding or a filter) can make
# an ADR whose bytes did not change count as changed, or run a filter.
empty_tree=$(g hash-object -t tree /dev/null) || fail "git hash-object failed"
g --attr-source="$empty_tree" rev-parse --git-dir >/dev/null 2>&1 ||
	fail "this git does not accept --attr-source"
export GIT_ATTR_SOURCE=$empty_tree

# The repository's local attributes file is read even with GIT_ATTR_SOURCE.
local_attributes=$(g rev-parse --git-path info/attributes) || fail "git rev-parse --git-path failed"
if [ -s "$local_attributes" ]; then
	fail "the repository has a local attributes file ($local_attributes)"
fi

# Rename detection is turned off so a renamed file counts as a deletion and an
# addition, never as M. git does not report a rename as M either way, so no
# test can tell it is off; it stays as a guard.
changed=$(g diff --no-ext-diff --no-textconv --no-renames --no-relative \
	--ignore-submodules=none -z --name-only --diff-filter=M HEAD | keep_adr_form) ||
	fail "git diff against HEAD failed"

# The tree is read from the commit resolved above, so the header names the
# commit whose tree was read even when a fetch moves the reference meanwhile.
on_main=$(g ls-tree -z --name-only "$origin_main" -- docs/dev/adr/ | keep_adr_form) ||
	fail "git ls-tree of refs/remotes/origin/main failed"

# A path of the form holds no space, newline, or glob character, so the
# lists split on newlines alone, with globbing off.
set -f
IFS=$'\n'
listed=()
for path in $changed; do
	for main_path in $on_main; do
		if [ "$path" = "$main_path" ]; then
			listed+=("$path")
			break
		fi
	done
done

if [ "${#args[@]}" -eq 0 ]; then
	for path in ${listed[@]+"${listed[@]}"}; do
		printf '%s\n' "$path" || exit 1
	done
	exit 0
fi

printf 'origin-main %s\n' "$origin_main" || exit 1
for file in "${args[@]}"; do
	result=no-match
	if [ -f "$file" ] && [ -r "$file" ]; then
		for path in ${listed[@]+"${listed[@]}"}; do
			if printf '%s' "$path" | cmp -s -- - "$file"; then
				result="match $path"
				break
			fi
		done
	fi
	printf '%s\n' "$result" || exit 1
done
