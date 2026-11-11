#!/bin/sh
# Checks that a set of .md files is the same at two commits, by name and by
# content: their names and modes, and their bytes.
#
#	golden_md_unchanged.sh <before-commit> <after-commit> <directory>
#
# Exits 1 when the sets differ, 2 when an argument is wrong or git cannot read
# a commit or a file, and 0 otherwise. Reads blobs rather than a checkout, so
# the working tree is never touched. Run it from the top level of the work tree.
# make test and CI do not run it.
#
# git ls-tree separates the path with a tab and -z ends each entry with a NUL,
# so a path that holds a space stays whole.

LC_ALL=C
export LC_ALL

if [ $# -ne 3 ]; then
	echo "usage: golden_md_unchanged.sh <before-commit> <after-commit> <directory>" >&2
	exit 2
fi

before=$1
after=$2
dir=$3

for commit in "$before" "$after"; do
	if ! git rev-parse --verify --quiet "$commit^{commit}" >/dev/null; then
		echo "golden_md_unchanged: $commit is not a commit" >&2
		exit 2
	fi
done

work=$(mktemp -d) || exit 2
trap 'rm -rf "$work"' EXIT

# list writes "<mode> <blob> <path>" for each .md file of commit at the NUL of
# the entry, then sorts by path.
list() {
	git ls-tree -r -z "$1" -- "$dir" |
		perl -0nE '$/ = "\0"; while (/\G([0-7]+) \S+ (\S+)\t(.*?)\.md\0/gs) { print "$1 $2 $3.md\n" }' |
		LC_ALL=C sort
}

echo "before: $before"
echo "after:  $after"
echo "files:  $dir/*.md"

if ! list "$before" >"$work/before.md" || ! list "$after" >"$work/after.md"; then
	echo "golden_md_unchanged: cannot list the files under $dir" >&2
	exit 2
fi
if [ ! -s "$work/before.md" ]; then
	echo "golden_md_unchanged: no .md file under $dir at $before" >&2
	exit 2
fi

status=0

# Check 1: the same file names, with the same modes and blob IDs, at both
# commits.
awk '{ print $3, $1, $2 }' "$work/before.md" >"$work/before.names"
awk '{ print $3, $1, $2 }' "$work/after.md" >"$work/after.names"
count=$(wc -l <"$work/before.names" | tr -d ' ')
if cmp -s "$work/before.names" "$work/after.names"; then
	echo "OK   the same $count file(s) with the same content at both commits"
else
	echo "NG   the files differ:"
	sed 's/^/before  /' "$work/before.names"
	sed 's/^/after   /' "$work/after.names"
	status=1
fi

# Check 2: the bytes of each file at both commits.
checked=0
while read -r mode blob path; do
	checked=$((checked + 1))
	after_blob=$(awk -v p="$path" '$3 == p { print $2 }' "$work/after.md")
	if [ -z "$after_blob" ]; then
		echo "NG   $path is missing at $after"
		status=1
		continue
	fi
	if ! git cat-file blob "$blob" >"$work/before.blob" 2>/dev/null ||
		! git cat-file blob "$after_blob" >"$work/after.blob" 2>/dev/null; then
		echo "golden_md_unchanged: cannot read $path" >&2
		exit 2
	fi
	if cmp -s "$work/before.blob" "$work/after.blob"; then
		continue
	fi
	echo "NG   $path changed at $after"
	status=1
done <"$work/before.md"

if [ "$status" -eq 0 ]; then
	echo "OK   all $checked file(s) are byte for byte the same"
else
	echo "NG   the .md files changed"
fi
exit "$status"
