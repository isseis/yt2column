#!/usr/bin/env bash
# One-off check for task 0081, plan step 1-7: the "Weak-model failure
# checklist" and the deterministic pre-check block of weakreview.md at BASE
# are in _lib/opus-review.md at HEAD with the same text, in the same order.
# Allowed differences: the heading level, the indentation of the code block
# (it was nested in a list item), and the replacements listed below, which
# rename references to weakreview.md's own step and placeholders.
#
# Usage: bash moved_checklist.sh <base-commit> <compared-commit>
# Run from the top level of the work tree. Exit 0 when every check passes.
set -euo pipefail

if [ "$#" -ne 2 ]; then
	echo "usage: $0 <base-commit> <compared-commit>" >&2
	exit 2
fi
base=$1
head=$2
old_path=.claude/commands/weakreview.md
new_path=.claude/commands/_lib/opus-review.md

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

git show "$base:$old_path" >"$tmp/old.md"
git show "$head:$new_path" >"$tmp/new.md"

# The replacements, one "old|new" pair per line.
cat >"$tmp/replacements" <<'EOF'
RANGE="<resolved range>"|RANGE="<RANGE>"
docs/tasks/<task-dir>/|<TASK_DIR>/
step-3|pre-check
EOF

# section FILE: the lines after the "Weak-model failure checklist" heading,
# of any level, up to the next heading or the end of the file, without the
# blank lines at its end.
section() {
	awk '/^#+ Weak-model failure checklist$/ { f = 1; next } f && /^#+ / { exit }
		f { line[++n] = $0; if (NF) last = n }
		END { for (i = 1; i <= last; i++) print line[i] }' "$1"
}

# codeblock FILE: the lines inside the file's only ```bash block, with the
# indentation common to its non-blank lines removed. Fails unless the file
# has exactly one such block.
codeblock() {
	local n
	n=$(grep -cE '^ *```bash$' "$1" || true)
	if [ "$n" != 1 ]; then
		echo "FAIL: $1 has $n bash code blocks, want 1" >&2
		return 1
	fi
	awk '/^ *```bash$/ { f = 1; next } f && /^ *```$/ { exit } f' "$1" >"$1.block"
	awk 'NF { match($0, /^ */); if (min == "" || RLENGTH < min) min = RLENGTH }
		END { print min + 0 }' "$1.block" >"$1.indent"
	awk -v n="$(cat "$1.indent")" '{ print substr($0, n + 1) }' "$1.block"
}

# replace FILE: FILE with every replacement applied, as fixed strings.
replace() {
	awk -v list="$tmp/replacements" '
		BEGIN { while ((getline l < list) > 0) { i = index(l, "|"); k++; from[k] = substr(l, 1, i - 1); to[k] = substr(l, i + 1) } }
		{
			for (j = 1; j <= k; j++) {
				out = ""; s = $0
				while ((p = index(s, from[j])) > 0) { out = out substr(s, 1, p - 1) to[j]; s = substr(s, p + length(from[j])) }
				$0 = out s
			}
			print
		}' "$1"
}

status=0

# Check 1: each replacement's old text occurs in the base, and none is left
# in the compared file.
while IFS='|' read -r from to; do
	if ! grep -qF -- "$from" "$tmp/old.md"; then
		echo "FAIL replacement: '$from' does not occur in $old_path at base"
		status=1
	elif grep -qF -- "$from" "$tmp/new.md"; then
		echo "FAIL replacement: '$from' is still in $new_path"
		status=1
	else
		echo "PASS replacement: '$from' -> '$to'"
	fi
done <"$tmp/replacements"

# Check 2: the checklist is the same.
section "$tmp/old.md" | replace /dev/stdin >"$tmp/old.checklist"
section "$tmp/new.md" >"$tmp/new.checklist"
if [ ! -s "$tmp/old.checklist" ] || [ ! -s "$tmp/new.checklist" ]; then
	echo "FAIL checklist: section not found"
	status=1
elif diff -u "$tmp/old.checklist" "$tmp/new.checklist"; then
	echo "PASS checklist: $(wc -l <"$tmp/new.checklist" | tr -d ' ') lines identical"
else
	echo "FAIL checklist: differs (diff above)"
	status=1
fi

# Check 3: the pre-check block is the same.
if codeblock "$tmp/old.md" >"$tmp/old.raw" && codeblock "$tmp/new.md" >"$tmp/new.block"; then
	replace "$tmp/old.raw" >"$tmp/old.block"
	if [ ! -s "$tmp/old.block" ] || [ ! -s "$tmp/new.block" ]; then
		echo "FAIL pre-check block: block is empty"
		status=1
	elif diff -u "$tmp/old.block" "$tmp/new.block"; then
		echo "PASS pre-check block: $(wc -l <"$tmp/new.block" | tr -d ' ') lines identical"
	else
		echo "FAIL pre-check block: differs (diff above)"
		status=1
	fi
else
	status=1
fi

exit "$status"
