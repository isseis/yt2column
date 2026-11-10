#!/bin/sh
# Checks that a PR of task 0080 changes only the files the task may change,
# that its ADR changes only delete the lines linking issue 789, and that the
# PR leaves no such line in the ADRs it resolves.
#
# Usage: scope.sh <base> <commit> pr1|pr2
#   <base>    the merge-base with origin/main
#   <commit>  the compared commit of the PR
#   pr1|pr2   which PR of the task is checked
#
# It runs from the top level of the work tree, whatever the current
# directory. It exits 0 when every check holds, 1 when a check fails (each
# failure is named), and 2 on a usage or git error.

set -u

if [ "$#" -ne 3 ]; then
	echo "usage: scope.sh <base> <commit> pr1|pr2" >&2
	exit 2
fi
case "$3" in
pr1 | pr2) mode=$3 ;;
*)
	echo "usage: scope.sh <base> <commit> pr1|pr2" >&2
	exit 2
	;;
esac

top=$(git rev-parse --show-toplevel) && cd "$top" || {
	echo "error: not inside a git work tree" >&2
	exit 2
}

base=$(git rev-parse --verify --quiet "$1^{commit}") || {
	echo "error: cannot resolve base $1" >&2
	exit 2
}
commit=$(git rev-parse --verify --quiet "$2^{commit}") || {
	echo "error: cannot resolve commit $2" >&2
	exit 2
}
echo "base:   $base"
echo "commit: $commit"
echo "mode:   $mode"

adr16=docs/dev/adr/0016-document-test-policy.md
adr35=docs/dev/adr/0035-language-policy.md
issue_re='issues/789([^0-9]|$)'
failed=0

fail() {
	echo "FAIL $1: $2"
	failed=1
}

# links_issue <pathspec>: exit 0 when a line under the pathspec at the
# compared commit links issue 789, 1 when none does; stop on a git error.
links_issue() {
	git grep -qE "$issue_re" "$commit" -- "$1"
	rc=$?
	if [ "$rc" -gt 1 ]; then
		echo "error: git grep failed on $1" >&2
		exit 2
	fi
	return "$rc"
}

# Check 1: the changed files are limited to the files the task may change.
# The file list is the fixed file list command of requirements_process.md
# section 4, so a moved file shows both its old and its new path.
files=$(env -u GIT_DIFF_OPTS git --no-pager -c core.quotePath=true \
	diff --no-ext-diff --no-relative --no-renames -O/dev/null --name-only "$base" "$commit") || {
	echo "error: git diff --name-only failed" >&2
	exit 2
}
outside=$(printf '%s\n' "$files" | while IFS= read -r f; do
	[ -n "$f" ] || continue
	case "$f" in
	(docs/dev/conventions/requirements_process.md | \
		.claude/commands/mkplan.md | \
		.claude/commands/runplan.md | \
		.claude/commands/runpr.md | \
		.claude/commands/weakreview.md | \
		.claude/commands/mkreq.md | \
		CLAUDE.md | \
		"$adr16" | \
		"$adr35" | \
		docs/tasks/0080_diff_check_proportion/*) ;;
	(*) printf '%s\n' "$f" ;;
	esac
done)
if [ -n "$outside" ]; then
	fail file-scope "files outside the task's scope changed:"
	printf '%s\n' "$outside" | sed 's/^/  /'
else
	echo "ok   file-scope"
fi

# Check 2: the diff under docs/dev/adr/ only deletes lines linking issue 789.
adr_diff=$(env -u GIT_DIFF_OPTS git --no-pager --literal-pathspecs -c core.quotePath=true \
	diff --no-color --no-ext-diff --no-textconv --text --default-prefix --no-relative \
	--no-renames -U0 "$base" "$commit" -- docs/dev/adr) || {
	echo "error: git diff of docs/dev/adr failed" >&2
	exit 2
}
bad=$(printf '%s\n' "$adr_diff" | awk -v re="$issue_re" '
	/^diff --git / { hunk = 0; next }
	!hunk && /^(old mode|new mode) / { print "mode change: " $0; next }
	/^@@ / { hunk = 1; next }
	hunk && /^\+/ { print "added: " substr($0, 2); next }
	hunk && /^-/ && substr($0, 2) !~ re { print "removed: " substr($0, 2) }
')
if [ -n "$bad" ]; then
	fail adr-deletions-only "docs/dev/adr/ has changes other than deleting lines that link issue 789:"
	printf '%s\n' "$bad" | sed 's/^/  /'
else
	echo "ok   adr-deletions-only"
fi

# Check 3: the ADRs this PR resolves no longer link issue 789.
if [ "$mode" = pr1 ]; then
	if links_issue "$adr16"; then
		fail adr-resolved "$adr16 still links issue 789"
	else
		echo "ok   adr-resolved ($adr16)"
	fi
	git diff --quiet --no-ext-diff --no-textconv "$base" "$commit" -- "$adr35"
	case $? in
	0) echo "ok   adr35-unchanged" ;;
	1) fail adr35-unchanged "$adr35 changed in PR-1" ;;
	*)
		echo "error: git diff failed on $adr35" >&2
		exit 2
		;;
	esac
else
	if links_issue docs/dev/adr; then
		fail adr-resolved "docs/dev/adr/ still links issue 789"
	else
		echo "ok   adr-resolved (docs/dev/adr/)"
	fi
fi

if [ "$failed" -ne 0 ]; then
	echo "result: FAIL"
	exit 1
fi
echo "result: PASS"
