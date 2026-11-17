#!/usr/bin/env bash
# AC-19: the only change to what /chief shows under the table is that the
# per-row list of each question's gist and pane (rows in 質問で停止) is gone;
# every other item stays, in the same order.
#
# Usage: ac19_under_the_table.sh <base-commit> <head-commit>
# It compares the items of "Under the table" in .claude/commands/chief.md at
# the two commits and prints PASS or FAIL with what differs.
set -euo pipefail

doc=.claude/commands/chief.md
removed_prefix='for each row in 質問で停止'

# items prints the items of "Under the table" at commit $1, one per line,
# each item's continuation lines joined to it by one space.
items() {
	git show "$1:$doc" | awk '
		/^3\. \*\*Under the table:\*\*/ { inside = 1; next }
		inside && /^[0-9]+\. / { exit }
		inside && /^   - / { if (item != "") print item; item = substr($0, 6); next }
		inside && /^     [^ ]/ { sub(/^ +/, ""); item = item " " $0; next }
		END { if (item != "") print item }
	'
}

base=$(items "$1")
head=$(items "$2")
printf 'base %s items:\n%s\n\nhead %s items:\n%s\n\n' "$1" "$base" "$2" "$head"

fail=0
if [ -z "$base" ] || [ -z "$head" ]; then
	echo "FAIL: no items read at one of the commits"
	exit 1
fi
if ! printf '%s\n' "$base" | grep -q "^$removed_prefix"; then
	echo "FAIL: the base has no item starting with \"$removed_prefix\""
	fail=1
fi
if printf '%s\n' "$head" | grep -q "^$removed_prefix"; then
	echo "FAIL: the head still has the item starting with \"$removed_prefix\""
	fail=1
fi
kept=$(printf '%s\n' "$base" | grep -v "^$removed_prefix" || true)
if [ "$kept" != "$head" ]; then
	echo "FAIL: the other items differ from the base's, or their order does:"
	diff <(printf '%s\n' "$kept") <(printf '%s\n' "$head") || true
	fail=1
fi
if [ "$fail" -ne 0 ]; then
	exit 1
fi
echo "PASS: only the per-row question list was removed; the other $(printf '%s\n' "$head" | wc -l | tr -d ' ') items stay in order"
