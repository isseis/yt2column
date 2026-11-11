#!/usr/bin/env bash
# One-off check for task 0081, plan step 3-6: under .claude/commands/ of the
# compared commit, the old implementation model requirement values
# (frontier-required, frontier-recommended, and the word "standard") appear
# only in the PR marker conventions section of _context.md, which holds the
# old-value table.
#
# Usage: bash old_values.sh <compared-commit>
# Run from the top level of the work tree. Exit 0 when the check passes.
set -euo pipefail

if [ "$#" -ne 1 ]; then
	echo "usage: $0 <compared-commit>" >&2
	exit 2
fi
commit=$(git rev-parse --verify "$1^{commit}")
ctx=.claude/commands/_context.md

# Uses of the word "standard" that are not a requirement value.
not_a_value='standard (error|output|input|library)|standard-library|standard classDef'

# Line range of the PR marker conventions section of _context.md: from its
# heading to the next heading of the same or a higher level.
range=$(git show "$commit:$ctx" | awk '
	/^### PR marker conventions/ { start = NR; next }
	start && !end && /^#{1,3} / { end = NR - 1 }
	END { if (start) print start "," (end ? end : NR) }')
if [ -z "$range" ]; then
	echo "FAIL: no PR marker conventions section in $ctx"
	exit 1
fi
first=${range%,*}
last=${range#*,}

# Check 1: the section holds the three old values (so the range is the right one).
section=$(git show "$commit:$ctx" | sed -n "${first},${last}p")
status=0
for v in frontier-required frontier-recommended standard; do
	if ! printf '%s\n' "$section" | grep -qE "^[[:space:]]*\| \`$v\` \|"; then
		echo "FAIL: the old-value table of $ctx has no row for $v"
		status=1
	fi
done

# Check 2: every other hit is outside .claude/commands/ or inside that range.
# git grep exits 1 when nothing matches; any other failure is a failure of this check.
rc=0
hits=$(git grep -nwE 'frontier-required|frontier-recommended|standard' "$commit" -- .claude/commands) || rc=$?
if [ "$rc" -gt 1 ]; then
	echo "FAIL: git grep failed with status $rc"
	exit 1
fi
stray=0
while IFS= read -r hit; do
	[ -n "$hit" ] || continue
	rest=${hit#"$commit":}
	file=${rest%%:*}
	rest=${rest#*:}
	line=${rest%%:*}
	text=${rest#*:}
	# Drop the non-value phrases; keep the hit only if a value-like word is left.
	left=$(printf '%s\n' "$text" | sed -E "s/($not_a_value)//g")
	printf '%s\n' "$left" | grep -qwE 'frontier-required|frontier-recommended|standard' || continue
	if [ "$file" = "$ctx" ] && [ "$line" -ge "$first" ] && [ "$line" -le "$last" ]; then
		continue
	fi
	echo "FAIL: old value outside the allowed place: $file:$line: $text"
	stray=1
done <<<"$hits"
[ "$stray" -eq 0 ] || status=1

if [ "$status" -eq 0 ]; then
	echo "PASS: old values appear only in the old-value table of $ctx (lines $first-$last)"
fi
exit "$status"
