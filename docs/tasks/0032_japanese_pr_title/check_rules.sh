#!/bin/sh
# Static checks for task 0032: PR titles in Japanese, squash subjects in
# English. Each check confirms that a rule file states a property the task's
# implementation plan names (03_implementation_plan.md, sections 2 and 5).
#
# Usage, from the repository root:
#   sh docs/tasks/0032_japanese_pr_title/check_rules.sh [<check-name>...]
# With no argument every check runs; with names, only those. The exit status is
# 0 when every check run passes, 1 when one fails, and 2 on a usage error (an
# unknown check name, an unknown CHECK_REV, or no git repository).
#
# CHECK_REV=<revision> reads the files of that revision's tree instead of the
# working tree.
#
# Text read from the documents is only matched (awk, grep); it is never run as
# a shell command.

set -u

# The checks, in the order they run.
ALL_CHECKS=''

die() {
	printf 'check_rules.sh: %s\n' "$1" >&2
	exit 2
}

top=$(git rev-parse --show-toplevel 2>/dev/null) || die 'not inside a git repository'
cd "$top" || die "cannot enter $top"

if [ -n "${CHECK_REV:-}" ]; then
	git rev-parse --verify --quiet "${CHECK_REV}^{tree}" >/dev/null ||
		die "unknown revision: $CHECK_REV"
fi

# fail PROPERTY: record that the running check found PROPERTY missing.
fail() {
	printf 'FAIL %s: %s\n' "$current" "$1"
	check_failed=1
}

# show FILE: print FILE from CHECK_REV's tree, or from the working tree.
show() {
	if [ -n "${CHECK_REV:-}" ]; then
		git show "${CHECK_REV}:$1" 2>/dev/null
	else
		cat -- "$1" 2>/dev/null
	fi
}

# repo_grep ARGS...: git grep over the tracked files outside docs/tasks/, in
# CHECK_REV's tree or the working tree.
repo_grep() {
	if [ -n "${CHECK_REV:-}" ]; then
		git grep "$@" "$CHECK_REV" -- . ':(exclude)docs/tasks'
	else
		git grep "$@" -- . ':(exclude)docs/tasks'
	fi
}

# section HEADING: print the Markdown section of stdin whose heading line is
# exactly HEADING (for example '### Plan PR'), up to the next heading of the
# same or a higher level. Lines inside code fences are never headings.
section() {
	H=$1 awk '
		BEGIN { h = ENVIRON["H"]; n = index(h, " ") - 1 }
		/^```/ { fence = !fence }
		on && !fence && /^#+ / { if (index($0, " ") - 1 <= n) exit }
		on { print }
		!fence && $0 == h { on = 1 }'
}

# between START END: print the lines of stdin from the first line matching the
# extended regular expression START up to, not including, the next line
# matching END.
between() {
	S=$1 E=$2 awk '
		BEGIN { s = ENVIRON["S"]; e = ENVIRON["E"] }
		on && $0 ~ e { exit }
		on { print; next }
		$0 ~ s { on = 1; print }'
}

# flat: join stdin into one line with single spaces, so a phrase matches even
# where the Markdown source wraps it.
flat() {
	tr '\n\t' '  ' | tr -s ' '
	echo
}

# has TEXT ERE PROPERTY: fail with PROPERTY unless TEXT matches ERE.
has() {
	printf '%s\n' "$1" | command grep -Eq -- "$2" || fail "$3"
}

# lacks TEXT ERE PROPERTY: fail with PROPERTY, and the matching lines, when
# TEXT matches ERE.
lacks() {
	_found=$(printf '%s\n' "$1" | command grep -E -- "$2") || return 0
	fail "$3"
	printf '%s\n' "$_found" | sed 's/^/    /'
}

for name in "$@"; do
	case " $ALL_CHECKS " in
	*" $name "*) ;;
	*) die "unknown check: $name" ;;
	esac
done

[ $# -gt 0 ] || set -- $ALL_CHECKS

failed=0
for name in "$@"; do
	current=$name
	check_failed=0
	"check_$name"
	if [ "$check_failed" -eq 0 ]; then
		printf 'ok   %s\n' "$name"
	else
		failed=1
	fi
done
exit "$failed"
