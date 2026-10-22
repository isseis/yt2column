#!/bin/sh
# Static check for task 0037: no file outside docs/tasks/ keeps a wording
# that says /runpr always runs the next command after the merge, or that
# recommends running it (03_implementation_plan.md, sections 1.3.4 and 5).
# It confirms only that the old wordings are gone, not that the new ones are
# there.
#
# Usage, from anywhere inside the repository:
#   sh docs/tasks/0037_runpr_merge_recommendation/check_wording.sh
# The exit status is 0 when no wording is left, 1 when one is, and 2 on any
# other error (an unknown CHECK_REV, no git repository, a failed git command).
#
# CHECK_REV=<revision> reads the files of that revision's tree instead of the
# working tree (tracked files plus untracked files that are not ignored).
#
# Each file is matched with its newlines, tabs, and runs of spaces folded into
# one space, so a wording split across lines is found too. Text read from the
# files is only matched (grep -F); it is never run as a shell command.

set -u

die() {
	printf 'check_wording.sh: %s\n' "$1" >&2
	exit 2
}

top=$(git rev-parse --show-toplevel 2>/dev/null) || die 'not inside a git repository'
cd "$top" || die "cannot enter $top"

if [ -n "${CHECK_REV:-}" ]; then
	git rev-parse --verify --quiet "${CHECK_REV}^{tree}" >/dev/null ||
		die "unknown revision: $CHECK_REV"
fi

tmp=$(mktemp -d) || die 'cannot create a temporary directory'
trap 'rm -rf "$tmp"' EXIT
trap 'exit 2' HUP INT TERM

# The wordings that must not be left, one per line.
cat >"$tmp/wordings" <<'EOF'
and run the next command
and carry it out
starts the next command
the next command to run
pre-merge bookkeeping and the next command
then run NEXT
run NEXT (recommended
EOF

# The files to search, outside docs/tasks/.
if [ -n "${CHECK_REV:-}" ]; then
	git -c core.quotePath=false ls-tree -r --name-only "$CHECK_REV" >"$tmp/all" ||
		die "cannot list the files of $CHECK_REV"
else
	git -c core.quotePath=false ls-files --cached --others --exclude-standard >"$tmp/all" ||
		die 'cannot list the files of the working tree'
fi
grep -v '^docs/tasks/' "$tmp/all" | sort -u >"$tmp/files"

# fold_file FILE: write FILE, from CHECK_REV's tree or the working tree, to
# $tmp/folded with its whitespace folded. A tracked file deleted from the
# working tree is written empty.
fold_file() {
	: >"$tmp/raw" || return 1
	if [ -n "${CHECK_REV:-}" ]; then
		git show "${CHECK_REV}:$1" >"$tmp/raw" || return 1
	elif [ -f "$1" ]; then
		cat -- "$1" >"$tmp/raw" || return 1
	fi
	LC_ALL=C tr '\n\t' '  ' <"$tmp/raw" | LC_ALL=C tr -s ' ' >"$tmp/folded"
}

found=0
while IFS= read -r f; do
	fold_file "$f" || die "cannot read $f"
	while IFS= read -r w; do
		if LC_ALL=C grep -q -F -- "$w" "$tmp/folded"; then
			printf 'FAIL "%s": %s\n' "$w" "$f"
			found=1
		fi
	done <"$tmp/wordings"
done <"$tmp/files"

if [ "$found" -eq 0 ]; then
	echo 'ok   no old wording left'
fi
exit "$found"
