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
# so a path that holds a space stays whole. A path never holds a tab or a
# newline, so each entry is kept on one line as "<path><tab><mode> <blob>": the
# path first, so that a path that holds a space sorts and prints whole.

LC_ALL=C
export LC_ALL

if [ $# -ne 3 ]; then
	echo "usage: golden_md_unchanged.sh <before-commit> <after-commit> <directory>" >&2
	exit 2
fi

before=$1
after=$2
dir=$3
tab=$(printf '\t')

for commit in "$before" "$after"; do
	if ! git rev-parse --verify --quiet "$commit^{commit}" >/dev/null; then
		echo "golden_md_unchanged: $commit is not a commit" >&2
		exit 2
	fi
done

work=$(mktemp -d) || exit 2
trap 'rm -rf "$work"' EXIT

# list writes "<path><tab><mode> <blob>" for each .md file of commit, sorted by
# path.
list() {
	git ls-tree -r -z "$1" -- "$dir" |
		perl -0nE '$/ = "\0"; while (/\G([0-7]+) \S+ (\S+)\t(.*?)\.md\0/gs) { print "$3.md\t$1 $2\n" }' |
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
count=$(wc -l <"$work/before.md" | tr -d ' ')
if cmp -s "$work/before.md" "$work/after.md"; then
	echo "OK   the same $count file(s) with the same content at both commits"
else
	echo "NG   the files differ:"
	sed 's/^/before  /' "$work/before.md" | tr '\t' ' '
	sed 's/^/after   /' "$work/after.md" | tr '\t' ' '
	status=1
fi

# Check 2: the bytes of each file at both commits. The entry at the after commit
# is the first entry whose path up to the tab is the whole path, which a file
# missing from the after commit does not match.
checked=0
while IFS= read -r line; do
	path=${line%	*}
	entry=${line#*	}
	mode=${entry%% *}
	blob=${entry#* }
	checked=$((checked + 1))
	after_blob=$(awk -v want="$path" -v tab="$tab" '
		{
			cut = index($0, tab)
			if (cut == 0) next
			p = substr($0, 1, cut - 1)
			if (p < want) next
			if (p != want) exit
			id = substr($0, cut + 1)
			sub(/^[0-7]+ /, "", id)
			print id
			exit
		}' "$work/after.md")
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
