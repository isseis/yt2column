#!/usr/bin/env bash
# Checks that step 2 ("## 2. One round") of the /chief definition differs
# between two commits only in its item 2 and in the --protocol values, and
# that item 2 did change.
#
# Usage, from the top level of the work tree:
#   bash docs/tasks/0102_chief_text_table/_checks/ac03_step2_diff.sh <base> <commit>
# It prints one line per check and exits 0 only when every check passes.
set -euo pipefail

base=$1
commit=$2
def=.claude/commands/chief.md
# Bracket expressions rather than backslashes: awk -v processes escapes.
item2='^2[.] [*][*]The table[*][*]'
item3='^3[.] [*][*]Under the table:[*][*]'

# step2 prints the lines of step 2 at a commit: from its heading to the
# next level-2 heading.
step2() {
	git show "$1:$def" | awk '/^## 2\. One round$/ { on = 1; print; next } on && /^## / { exit } on'
}

# outside prints step 2 without item 2, with every protocol number replaced.
outside() {
	awk -v a="$item2" -v b="$item3" '$0 ~ a { skip = 1 } $0 ~ b { skip = 0 } !skip' |
		sed -E 's/--protocol [0-9]+/--protocol N/g'
}

# inside prints item 2 alone.
inside() {
	awk -v a="$item2" -v b="$item3" '$0 ~ a { on = 1 } $0 ~ b { on = 0 } on'
}

fail=0
b=$(step2 "$base")
c=$(step2 "$commit")
for side in b c; do
	text=${!side}
	if ! grep -q "$item2" <<<"$text" || ! grep -q "$item3" <<<"$text"; then
		echo "FAIL: step 2, item 2, or item 3 not found at the $side side"
		exit 1
	fi
done

if d=$(diff <(outside <<<"$b") <(outside <<<"$c")); then
	echo "OK: step 2 outside item 2 is unchanged but for --protocol values"
else
	echo "FAIL: step 2 changed outside item 2:"
	printf '%s\n' "$d"
	fail=1
fi

if [ "$(inside <<<"$b")" != "$(inside <<<"$c")" ]; then
	echo "OK: item 2 changed"
else
	echo "FAIL: item 2 did not change"
	fail=1
fi
exit "$fail"
