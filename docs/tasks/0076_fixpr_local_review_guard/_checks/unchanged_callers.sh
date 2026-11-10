#!/bin/sh
# Checks that a branch of task 0076 leaves the other callers of the shared
# local CLI review procedure, and its record lines, byte-identical: the whole
# of .claude/commands/runplan.md and .claude/commands/_lib/task-pr-handoff.md,
# the standalone-change item of runpr.md step 1 (from the line
# "- If none exists, it is a standalone change:" up to the next line that
# starts "- " at the same indent), and step 7 of
# .claude/commands/_lib/local-cli-review.md (from the line that starts
# "7. **Record lines.**" up to the line that starts "8. **SUMMARY.**").
# The item and the step must be found exactly once in both revisions, so a
# changed start line cannot pass by being missing on both sides.
#
# Usage:
#
#	docs/tasks/0076_fixpr_local_review_guard/_checks/unchanged_callers.sh <upstream> <head>
#
# The merge-base is that of <upstream> and <head>. On a PR branch of the task,
# after git fetch origin main:
#
#	docs/tasks/0076_fixpr_local_review_guard/_checks/unchanged_callers.sh origin/main HEAD
#
# Output and exit codes:
#   0  every target is unchanged (the merge-base and the head on stdout)
#   1  a check failed (the merge-base and the head on stdout, the reason on
#      stderr)
#   2  the arguments or the repository are unusable (not two arguments, a
#      value that is not a commit, no repository, a temporary directory that
#      cannot be made, or a git command that fails)
#
# Run it once on the PR-1 branch of the task, before its PR checkpoint. make
# test and CI do not run it. It reads no environment variable and names no
# secret.
set -eu

if [ "$#" -ne 2 ]; then
	echo "usage: $0 <upstream> <head>" >&2
	exit 2
fi

if ! root=$(git rev-parse --show-toplevel 2>/dev/null); then
	echo "not a git repository" >&2
	exit 2
fi

for ref in "$1" "$2"; do
	if ! git -C "$root" cat-file -e "$ref^{commit}" 2>/dev/null; then
		echo "not a commit: $ref" >&2
		exit 2
	fi
done
if ! head=$(git -C "$root" rev-parse --verify "$2^{commit}"); then
	echo "cannot resolve $2" >&2
	exit 2
fi
if ! base=$(git -C "$root" merge-base "$1" "$head"); then
	echo "no merge-base of $1 and $2" >&2
	exit 2
fi
echo "merge-base: $base"
echo "head: $head"

if ! tmp=$(mktemp -d); then
	echo "cannot make a temporary directory" >&2
	exit 2
fi
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

# Writes a revision of a file; a git failure is unusable input.
show_file() {
	if ! git -C "$root" show "$1:$2" >"$3" 2>/dev/null; then
		echo "git show failed: $2 at $1" >&2
		exit 2
	fi
}

# Prints the lines of a file from the line equal to $2 up to, not including,
# the next line that starts with $3, and prints the number of such start
# lines to $4.
extract_block() {
	awk -v start="$2" -v next_prefix="$3" -v countfile="$4" '
		$0 == start { found++; in_block = 1; print; next }
		in_block && index($0, next_prefix) == 1 { in_block = 0 }
		in_block { print }
		END { print found + 0 > countfile }
	' "$1"
}

# Same as extract_block, with a start prefix instead of a whole line.
extract_prefixed() {
	awk -v start="$2" -v next_prefix="$3" -v countfile="$4" '
		index($0, start) == 1 { found++; in_block = 1; print; next }
		in_block && index($0, next_prefix) == 1 { in_block = 0 }
		in_block { print }
		END { print found + 0 > countfile }
	' "$1"
}

ret=0

for file in .claude/commands/runplan.md .claude/commands/_lib/task-pr-handoff.md; do
	show_file "$base" "$file" "$tmp/base"
	show_file "$head" "$file" "$tmp/head"
	if ! cmp -s "$tmp/base" "$tmp/head"; then
		echo "$file changed" >&2
		ret=1
	fi
done

# compare_block <file> <label> <extractor> <start> <next prefix>
compare_block() {
	show_file "$base" "$1" "$tmp/base"
	show_file "$head" "$1" "$tmp/head"
	"$3" "$tmp/base" "$4" "$5" "$tmp/count-base" >"$tmp/block-base"
	"$3" "$tmp/head" "$4" "$5" "$tmp/count-head" >"$tmp/block-head"
	for side in base head; do
		count=$(cat "$tmp/count-$side")
		if [ "$count" -ne 1 ]; then
			echo "$1: $2 found $count times in the $side, not once" >&2
			ret=1
			return
		fi
	done
	if ! cmp -s "$tmp/block-base" "$tmp/block-head"; then
		echo "$1: $2 changed" >&2
		ret=1
	fi
}

compare_block .claude/commands/runpr.md "the standalone-change item of step 1" extract_block \
	"- If none exists, it is a standalone change:" "- "
compare_block .claude/commands/_lib/local-cli-review.md "step 7 (record lines)" extract_prefixed \
	"7. **Record lines.**" "8. **SUMMARY.**"

if [ "$ret" -ne 0 ]; then
	exit 1
fi

echo "runplan.md, task-pr-handoff.md, runpr.md's standalone-change item, and local-cli-review.md step 7 are unchanged"
