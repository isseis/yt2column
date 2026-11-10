#!/bin/sh
# Checks that a branch of task 0076 changes the architecture document of task
# 0030 only by adding one note to each section listed below, and changes
# nothing else in it: no line is removed, every added line belongs to a note
# block that starts with "> **注記（タスク 0076）:**" (the other lines of the
# block start with ">", and an added empty line is allowed), the Document
# Status section is byte-identical (the document has no change-history section),
# the file's mode does not change, each listed section gains exactly one note
# before its body (after a task 0057 note when the section has one), no other
# section gains one, and each note links to a document of task 0076 that
# exists in the head.
#
# Usage:
#
#	docs/tasks/0076_fixpr_local_review_guard/_checks/notes_only.sh <upstream> <head>
#
# The merge-base is that of <upstream> and <head>. On a PR branch of the task,
# after git fetch origin main:
#
#	docs/tasks/0076_fixpr_local_review_guard/_checks/notes_only.sh origin/main HEAD
#
# Output and exit codes:
#   0  the document changed only as described (the merge-base and the head on
#      stdout)
#   1  a check failed (the merge-base and the head on stdout, the reason on
#      stderr)
#   2  the arguments or the repository are unusable (not two arguments, a
#      value that is not a commit, no repository, no docs/tasks/ directory, a
#      temporary directory that cannot be made, or a git command that fails)
#
# Run it once on the PR-1 branch of the task, before its PR checkpoint. make
# test and CI do not run it. It reads no environment variable and names no
# secret.
set -eu

doc=docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md
note='> **注記（タスク 0076）:**'
other='> **注記（タスク 0057）:**'
link='../0076_fixpr_local_review_guard/'

if [ "$#" -ne 2 ]; then
	echo "usage: $0 <upstream> <head>" >&2
	exit 2
fi

if ! root=$(git rev-parse --show-toplevel 2>/dev/null); then
	echo "not a git repository" >&2
	exit 2
fi
if [ ! -d "$root/docs/tasks" ]; then
	echo "no docs/tasks/ directory under $root" >&2
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

# The sections that gain one note, one heading per line.
sections='### 3.7. git の状態の基準値（F-004・H-08）
### 5.2. 守りの対応
### 5.3. 残余のリスク'

if ! git -C "$root" show "$base:$doc" >"$tmp/base-doc" 2>/dev/null; then
	echo "git show failed: $doc at the merge-base" >&2
	exit 2
fi
if ! git -C "$root" show "$head:$doc" >"$tmp/head-doc" 2>/dev/null; then
	echo "git show failed: $doc at the head" >&2
	exit 2
fi

# Prints one section of a document: the heading line and the lines after it,
# up to the next heading of the same or a higher level.
extract_section() {
	awk -v heading="$2" '
		function level(s,   n, i) {
			n = 0
			for (i = 1; i <= length(s); i++) {
				if (substr(s, i, 1) == "#") n++
				else break
			}
			return n
		}
		!started {
			if ($0 == heading) {
				started = 1
				lvl = level($0)
				print
			}
			next
		}
		substr($0, 1, 1) == "#" {
			if (level($0) <= lvl) exit
		}
		{ print }
	' "$1"
}

extract_status() {
	awk '
		/^## Document Status$/ { found = 1; print; next }
		found && /^## / { exit }
		found { print }
	' "$1"
}

# (1) the document changed and its mode did not.
if ! git -C "$root" diff --no-renames --name-only "$base" "$head" -- "$doc" >"$tmp/names"; then
	echo "git diff failed: $doc" >&2
	exit 2
fi
if [ ! -s "$tmp/names" ]; then
	echo "$doc did not change" >&2
	exit 1
fi
if ! git -C "$root" diff --no-renames --summary "$base" "$head" -- "$doc" >"$tmp/summary"; then
	echo "git diff failed: $doc" >&2
	exit 2
fi
if [ -s "$tmp/summary" ]; then
	echo "$doc changed its file mode" >&2
	exit 1
fi
if [ "$(grep -cF -- "$note" "$tmp/base-doc" || true)" -ne 0 ]; then
	echo "$doc already holds a task 0076 note in the merge-base" >&2
	exit 1
fi

ret=0

# (2) no removed line, and every added line belongs to a note block that
# starts with the note marker.
if ! git -C "$root" diff --no-renames --unified=0 --no-color "$base" "$head" -- "$doc" >"$tmp/diff"; then
	echo "git diff failed: $doc" >&2
	exit 2
fi
if ! awk -v marker="$note" '
	/^@@/ { hunk = 1; first = 1; next }
	!hunk { next }
	/^\\/ { next }
	/^-/ { print "removed: " $0; bad = 1; next }
	/^\+/ {
		line = substr($0, 2)
		if (line == "") next
		if (first) {
			if (index(line, marker) != 1) {
				print "added block does not start with the note: " $0
				bad = 1
			}
			first = 0
		} else if (substr(line, 1, 1) != ">") {
			print "added without >: " $0
			bad = 1
		}
	}
	END { exit bad ? 1 : 0 }
' "$tmp/diff" >&2; then
	echo "$doc removes a line or adds a line outside a note block" >&2
	ret=1
fi

# (3) the Document Status section is byte-identical.
extract_status "$tmp/base-doc" >"$tmp/status-base"
extract_status "$tmp/head-doc" >"$tmp/status-head"
if ! cmp -s "$tmp/status-base" "$tmp/status-head"; then
	echo "$doc changed its Document Status section" >&2
	ret=1
fi

# (4) one note per listed section and none elsewhere; (5) the note comes
# before the body, after a task 0057 note; (6) the note links to a task 0076
# document that exists in the head.
total=$(grep -cF -- "$note" "$tmp/head-doc" || true)
count_sections=0
while IFS= read -r heading; do
	[ -n "$heading" ] || continue
	count_sections=$((count_sections + 1))
	extract_section "$tmp/head-doc" "$heading" >"$tmp/section"
	count=$(grep -cF -- "$note" "$tmp/section" || true)
	if [ "$count" -ne 1 ]; then
		echo "$doc: section \"$heading\" holds $count task 0076 notes, not 1" >&2
		ret=1
		continue
	fi
	if ! awk -v marker="$note" -v other="$other" '
		NR > 1 && !body {
			if ($0 == "") next
			if (index($0, marker) == 1) { seen = 1; next }
			if (index($0, other) == 1) { if (seen) late = 1; next }
			if (substr($0, 1, 1) != ">") body = 1
		}
		END { exit (seen && !late) ? 0 : 1 }
	' "$tmp/section"; then
		echo "$doc: the note of section \"$heading\" comes after the body or before a task 0057 note" >&2
		ret=1
	fi
	line=$(grep -F -- "$note" "$tmp/section")
	targets=$(printf '%s\n' "$line" | awk -v prefix="]($link" '
		{
			s = $0
			while ((i = index(s, prefix)) > 0) {
				s = substr(s, i + 2)
				j = index(s, ")")
				if (j == 0) break
				print substr(s, 1, j - 1)
				s = substr(s, j + 1)
			}
		}
	')
	if [ -z "$targets" ]; then
		echo "$doc: the note of section \"$heading\" has no link to a task 0076 document" >&2
		ret=1
		continue
	fi
	for target in $targets; do
		path="docs/tasks/${target#../}"
		if ! git -C "$root" cat-file -e "$head:$path" 2>/dev/null; then
			echo "$doc: the note of section \"$heading\" links to $target, which the head does not have" >&2
			ret=1
		fi
	done
done <<EOF
$sections
EOF
if [ "$total" -ne "$count_sections" ]; then
	echo "$doc holds $total task 0076 notes, not one per listed section ($count_sections)" >&2
	ret=1
fi

if [ "$ret" -ne 0 ]; then
	exit 1
fi

echo "$doc adds one task 0076 note per listed section, before the body, and changes nothing else"
