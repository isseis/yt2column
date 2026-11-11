#!/usr/bin/env bash
# opus-review-decision.sh decides whether /runplan runs the Opus review before
# it opens a PR (entry 1 of .claude/commands/_lib/opus-review.md).
#
# Usage: bash opus-review-decision.sh <PLAN> <TARGET> <BASE>
#
#   PLAN    the plan's repository-relative path,
#           docs/tasks/<task-dir>/03_implementation_plan.md
#   TARGET  N of the PR's "### PR-N 作成ポイント" marker, or "single" for a plan
#           without PR markers
#   BASE    $(git merge-base origin/main HEAD)
#
# Output, on standard output:
#   RUN                  run the Opus review
#   NONE <reason>        do not run it; <reason> is value-sonnet, value-absent,
#                        or all-opus
#   SKIP <done-line>     skip it: <done-line> is the "Opus review: done" record
#                        line that covers the branch, followed by one line
#                        "<12-hex-id> <subject>" for each commit after its END,
#                        oldest first
#
# Contract: every failure (a git error, an unreadable plan, a bad argument,
# two places that could hold the value) prints RUN, writes the reason to
# standard error, and exits 0. When in doubt, the decision falls on the side
# that runs the review. The script reads git's object database only (no work
# tree file), touches no network, and changes nothing.
#
# The rules this script executes are defined elsewhere, and those definitions
# win over this file:
#   - where the value is, how it is read, and the old-value table:
#     .claude/commands/_context.md, "PR marker conventions" (the table is read
#     from origin/main's copy of that file, never held here);
#   - the record-line forms, the Opus rule, the excludable commits, and the
#     decision steps: .claude/commands/_lib/opus-review.md, "Record lines" and
#     "Entry 1".
#
# Japanese literals: the heading and value-line texts below must equal the
# text of the Japanese plans this script reads (CLAUDE.md, "Go Idioms").

set -u
export LC_ALL=C

run_on_failure() {
	printf 'opus-review-decision: %s\n' "$1" >&2
	echo RUN
	exit 0
}

[ "${BASH_VERSINFO[0]:-0}" -ge 4 ] || run_on_failure "bash 4 or later is required"
[ "$#" -eq 3 ] || run_on_failure "expected 3 arguments, got $#"
PLAN=$1
TARGET=$2
BASE_ARG=$3

[[ $PLAN =~ ^docs/tasks/[^/]+/03_implementation_plan\.md$ ]] ||
	run_on_failure "PLAN is not docs/tasks/<task-dir>/03_implementation_plan.md"
[[ $TARGET == single || $TARGET =~ ^[1-9][0-9]*$ ]] ||
	run_on_failure "TARGET is neither a PR number nor single"

TOP=$(git rev-parse --show-toplevel 2>/dev/null) || run_on_failure "not in a git work tree"
cd "$TOP" || run_on_failure "cannot enter the top of the work tree"

OM=$(git rev-parse --verify --quiet 'refs/remotes/origin/main^{commit}') ||
	run_on_failure "refs/remotes/origin/main names no commit"
HEAD_ID=$(git rev-parse --verify --quiet 'HEAD^{commit}') || run_on_failure "HEAD names no commit"
BASE=$(git rev-parse --verify --quiet "${BASE_ARG}^{commit}") || run_on_failure "BASE names no commit"
# BASE must be the merge-base itself: a narrower range (such as a phase
# group's PHASE_START) would leave commits out of the decision.
MB=$(git merge-base "$OM" "$HEAD_ID") || run_on_failure "origin/main and HEAD have no merge-base"
[ "$BASE" = "$MB" ] || run_on_failure "BASE is not the merge-base of origin/main and HEAD"

# --- Step 1: read the value -------------------------------------------------

# locate_value prints one of: VALUE<TAB><text>, ABSENT, NOSECTION, MULTIPLE.
# It skips fenced code blocks (``` or ~~~, nested ones included), so a diff record's content is
# never read as a heading or as a value line; an unclosed fence, or (for
# "single") a PR marker heading, prints MULTIPLE, a failure.
locate_value() {
	awk -v target="$TARGET" '
		function trim(s) { sub(/^[ \t\r]+/, "", s); sub(/[ \t\r]+$/, "", s); return s }
		{ sub(/\r$/, "") }
		# A fence closes only on a line of the same character, at least as
		# long as the opening run, with nothing after it; a fence-like line
		# inside a fence is content.
		{
			t = $0
			sub(/^[ \t]+/, "", t)
			c = substr(t, 1, 1)
			if (substr(t, 1, 3) == "```" || substr(t, 1, 3) == "~~~") {
				len = 0
				while (substr(t, len + 1, 1) == c) len++
				rest = substr(t, len + 1)
				if (!fence) { fence = 1; fchar = c; flen = len; next }
				if (c == fchar && len >= flen && rest ~ /^[ \t]*$/) { fence = 0; next }
			}
		}
		fence { next }
		target == "single" && /^### PR-[0-9]+ 作成ポイント/ { markers++ }
		/^#+[ \t]/ {
			level = match($0, /[^#]/) - 1
			if (target == "single") {
				if (level <= 2) {
					in3 = ($0 ~ /^## 3([.]|[ \t])/)
					insec = 0
					next
				}
				text = $0
				sub(/^#+[ \t]+/, "", text)
				sub(/^[0-9]+([.][0-9]+)*[.]?[ \t]+/, "", text)
				insec = (in3 && trim(text) == "PR 構成")
			} else {
				insec = ($0 ~ ("^### PR-" target " 作成ポイント(:|[ \t]*$)"))
			}
			if (insec) sections++
			next
		}
		insec && index($0, "**実装モデル要件**: ") == 1 {
			values++
			value = trim(substr($0, length("**実装モデル要件**: ") + 1))
		}
		END {
			if (fence || markers) print "MULTIPLE"
			else if (sections == 0) print "NOSECTION"
			else if (sections > 1 || values > 1) print "MULTIPLE"
			else if (values == 0) print "ABSENT"
			else printf "VALUE\t%s\n", value
		}
	'
}

# old_value_table prints "<old> <new>" for each row of the old-value table in
# origin/main's _context.md: a two-cell row, inside "### PR marker
# conventions", whose second cell is one of the three values. An old value
# with two rows is dropped, so it reads as unreadable.
old_value_table() {
	git cat-file blob "$OM:.claude/commands/_context.md" 2>/dev/null | awk '
		/^#+[ \t]/ { insec = ($0 ~ /^### PR marker conventions[ \t]*$/); next }
		insec && match($0, /^[ \t]*\| `[^`]+` \| `(opus|sonnet\+opus-review|sonnet)` \|[ \t]*$/) {
			line = $0
			sub(/^[ \t]*\| `/, "", line)
			old = line; sub(/`.*/, "", old)
			new = line; sub(/^[^`]*` \| `/, "", new); sub(/`.*/, "", new)
			count[old]++
			map[old] = new
		}
		END { for (o in map) if (count[o] == 1) print o, map[o] }
	'
}

# classify reads one copy of the plan and prints opus, sonnet+opus-review,
# sonnet, unreadable, absent, or (for the failures) NOSECTION or MULTIPLE.
classify() {
	local result text old new
	result=$(locate_value)
	case $result in
	NOSECTION | MULTIPLE | ABSENT)
		[ "$result" = ABSENT ] && echo absent || echo "$result"
		return
		;;
	esac
	text=${result#VALUE$'\t'}
	case $text in
	opus | sonnet+opus-review | sonnet)
		echo "$text"
		return
		;;
	esac
	# An empty table still yields one empty line, which must not match an
	# empty value.
	while read -r old new; do
		if [ -n "$old" ] && [ "$old" = "$text" ]; then
			echo "$new"
			return
		fi
	done <<<"$OLD_TABLE"
	echo unreadable
}

OLD_TABLE=$(old_value_table)

HEAD_PLAN=$(git cat-file blob "$HEAD_ID:$PLAN" 2>/dev/null) || run_on_failure "cannot read PLAN at HEAD"
HEAD_READ=$(printf '%s\n' "$HEAD_PLAN" | classify)
case $HEAD_READ in
NOSECTION)
	if [ "$TARGET" = single ]; then
		HEAD_READ=absent
	else
		run_on_failure "HEAD's plan has no PR-$TARGET marker"
	fi
	;;
MULTIPLE) run_on_failure "HEAD's plan has more than one place for the value" ;;
esac

# origin/main's copy counts only when it has the plan and the target place;
# without them the decision rests on HEAD alone.
OM_READ=absent
if OM_PLAN=$(git cat-file blob "$OM:$PLAN" 2>/dev/null); then
	OM_READ=$(printf '%s\n' "$OM_PLAN" | classify)
	case $OM_READ in
	NOSECTION) OM_READ=absent ;;
	MULTIPLE) run_on_failure "origin/main's plan has more than one place for the value" ;;
	esac
fi

needs_review() {
	case $1 in opus | sonnet+opus-review | unreadable) return 0 ;; esac
	return 1
}
if ! needs_review "$HEAD_READ" && ! needs_review "$OM_READ"; then
	if [ "$HEAD_READ" = sonnet ] || [ "$OM_READ" = sonnet ]; then
		echo "NONE value-sonnet"
	else
		echo "NONE value-absent"
	fi
	exit 0
fi

# --- Commits, messages, and excludable commits ------------------------------

COMMITS=$(git rev-list "$BASE..$HEAD_ID") || run_on_failure "git rev-list failed"

# message prints a commit's message; the commit object is read directly, so
# no log configuration (such as signature display) changes it.
message() {
	git cat-file commit "$1" | awk 'body { print; next } /^$/ { body = 1 }'
}

parent_count() {
	git cat-file commit "$1" | awk '/^$/ { exit } /^parent / { n++ } END { print n + 0 }'
}

# excludable: one parent, and either empty or changing only PLAN and
# README.en.md.
excludable() {
	local c=$1 path paths
	[ "$(parent_count "$c")" = 1 ] || return 1
	paths=$(
		set -o pipefail
		git diff-tree -z --no-commit-id --name-only -r --no-renames "$c" | tr '\0' '\n'
	) || return 1
	while IFS= read -r path; do
		[ -z "$path" ] || [ "$path" = "$PLAN" ] || [ "$path" = README.en.md ] || return 1
	done <<<"$paths"
	return 0
}

# An ID is printable ASCII without a space or a comma.
ID='[!-+.-~-]+'
IMPL_RE="^Implementation model: ([0-9a-f]{12})\\.\\.([0-9a-f]{12}) ($ID(, $ID)*)\$"
DONE_RE='^Opus review: done ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(critical [0-9]+, major [0-9]+, minor [0-9]+\)$'

resolve12() {
	git rev-parse --verify --quiet "$1^{commit}" 2>/dev/null
}

# --- Step 2: a done record that still covers the branch ---------------------

# The last done record is the last matching line of the newest commit that
# has one. Its START must be BASE or an ancestor of it, its END an ancestor of
# HEAD, and every commit after END must be excludable.
DONE_LINE=""
for c in $COMMITS; do
	line=$(message "$c" | grep -E -- "$DONE_RE" | tail -n 1)
	if [ -n "$line" ]; then
		DONE_LINE=$line
		break
	fi
done
if [ -n "$DONE_LINE" ] && [[ $DONE_LINE =~ $DONE_RE ]]; then
	START=$(resolve12 "${BASH_REMATCH[1]}") || START=""
	END=$(resolve12 "${BASH_REMATCH[2]}") || END=""
	if [ -n "$START" ] && [ -n "$END" ] &&
		git merge-base --is-ancestor "$START" "$BASE" &&
		git merge-base --is-ancestor "$END" "$HEAD_ID"; then
		AFTER=$(git rev-list --reverse "$END..$HEAD_ID") || run_on_failure "git rev-list failed"
		ok=1
		for c in $AFTER; do
			excludable "$c" || {
				ok=0
				break
			}
		done
		if [ "$ok" = 1 ]; then
			echo "SKIP $DONE_LINE"
			for c in $AFTER; do
				git log -1 --no-show-signature --format='%h %s' --abbrev=12 "$c" ||
					run_on_failure "git log failed"
			done
			exit 0
		fi
	fi
fi

# --- Step 3: every commit covered by Opus implementation records ------------

# COVERED holds each commit inside an Opus record's range; TAINTED each commit
# inside a non-Opus record's range.
declare -A COVERED=() TAINTED=()
for c in $COMMITS; do
	while IFS= read -r line; do
		[[ $line =~ $IMPL_RE ]] || continue
		START=$(resolve12 "${BASH_REMATCH[1]}") || continue
		END=$(resolve12 "${BASH_REMATCH[2]}") || continue
		opus=1
		IFS=',' read -r -a ids <<<"${BASH_REMATCH[3]}"
		for id in "${ids[@]}"; do
			id=${id# }
			[[ $id == claude-opus-* ]] || opus=0
		done
		RANGE_COMMITS=$(git rev-list "$START..$END") || run_on_failure "git rev-list failed"
		for r in $RANGE_COMMITS; do
			if [ "$opus" = 1 ]; then COVERED[$r]=1; else TAINTED[$r]=1; fi
		done
	done < <(message "$c")
done

for c in $COMMITS; do
	[ "$(parent_count "$c")" = 1 ] || {
		echo RUN
		exit 0
	}
	excludable "$c" && continue
	if [ -z "${COVERED[$c]:-}" ] || [ -n "${TAINTED[$c]:-}" ]; then
		echo RUN
		exit 0
	fi
done
echo "NONE all-opus"
