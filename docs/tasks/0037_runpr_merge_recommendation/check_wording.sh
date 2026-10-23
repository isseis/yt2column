#!/bin/sh
# Static check for task 0037: no file outside docs/tasks/ keeps a wording
# that says /runpr always runs the next command after the merge, or that
# recommends running it (03_implementation_plan.md, sections 1.3.4 and 5).
# It confirms only that the old wordings are gone, not that the new ones are
# there.
#
# Usage:
#   sh docs/tasks/0037_runpr_merge_recommendation/check_wording.sh
# It checks the repository that holds the script, whatever the current
# directory is.
# The exit status is 0 when no wording is left, 1 when one is, and 2 on any
# other error (an unknown CHECK_REV, no git repository, a failed git command).
#
# CHECK_REV=<revision> reads the files of that revision's tree instead of the
# working tree (tracked files plus untracked files that are not ignored).
#
# Each file is matched with its ASCII whitespace (CR, LF, tab, form feed,
# vertical tab, space) folded into one space, so a wording split across lines
# is found too. The match is case-sensitive and strips no comment or quote
# prefix, so a wording broken by "//" or ">" at a line start is not found; the
# plan's manual cross-search covers those. A file name git has to quote is
# rejected (exit 2) rather than skipped. Text read from the files is only
# matched (grep -F); it is never run as a shell command.

set -u

die() {
	printf 'check_wording.sh: %s\n' "$1" >&2
	exit 2
}

here=$(dirname "$0")
top=$(cd "$here" 2>/dev/null && git rev-parse --show-toplevel 2>/dev/null) ||
	die 'not inside a git repository'
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
if LC_ALL=C grep -q '^"' "$tmp/all"; then
	die 'a file name git quotes is listed; rename it to check it'
fi
LC_ALL=C grep -v '^docs/tasks/' "$tmp/all" >"$tmp/kept"
[ $? -le 1 ] || die 'cannot filter the file list'
sort -u "$tmp/kept" >"$tmp/files" || die 'cannot sort the file list'

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
	LC_ALL=C tr '\r\n\t\f\v' '     ' <"$tmp/raw" >"$tmp/spaced" || return 1
	LC_ALL=C tr -s ' ' <"$tmp/spaced" >"$tmp/folded"
}

found=0
while IFS= read -r f; do
	fold_file "$f" || die "cannot read $f"
	while IFS= read -r w; do
		LC_ALL=C grep -q -F -- "$w" "$tmp/folded"
		case $? in
		0)
			printf 'FAIL "%s": %s\n' "$w" "$f"
			found=1
			;;
		1) ;;
		*) die "cannot match $f" ;;
		esac
	done <"$tmp/wordings"
done <"$tmp/files"

if [ "$found" -eq 0 ]; then
	echo 'ok   no old wording left'
fi
exit "$found"
