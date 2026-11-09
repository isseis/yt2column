#!/bin/sh
# Checks that a branch of task 0066 removed the documents' description of the
# five retired LLM variables and kept every other heading:
#
#   - none of the five retired variable names appears at the head in README.md,
#     README.en.md, docs/dev/guides/cli.md, or docs/dev/project_overview.md,
#     while each of these documents names at least one of them at the
#     merge-base (so the check reads the right files);
#   - the two glossary rows of the retired variables (the term and the README
#     heading) are in docs/translation_glossary.md at the merge-base and not at
#     the head;
#   - every heading of those five documents at the merge-base, except the
#     removed migration heading of each README version, is still a heading at
#     the head, with the same text and in the same order. Headings the branch
#     added may sit anywhere between them.
#
# A heading is a line starting with "#" outside a fenced code block (a line
# starting with three backquotes opens or closes one).
#
# Usage:
#
#	docs/tasks/0066_server_config_file/_checks/retired_docs_removed.sh <upstream> <head>
#
# The merge-base is that of <upstream> and <head>. On the task's PR branch,
# after git fetch origin main:
#
#	docs/tasks/0066_server_config_file/_checks/retired_docs_removed.sh origin/main HEAD
#
# Output and exit codes:
#   0  every check passed (the merge-base, the head, and one line per check on
#      stdout)
#   1  a check failed (the failures on stderr)
#   2  the arguments or the repository are unusable (not two arguments, a
#      value that is not a commit, no repository, or a git command that fails)
#
# Run it once on the PR branch, before the PR checkpoint. make test and CI do
# not run it. It reads no environment variable and names no secret.
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
head=$(git -C "$root" rev-parse --verify "$2^{commit}") || exit 2
base=$(git -C "$root" merge-base "$1" "$head") || {
	echo "no merge-base of $1 and $2" >&2
	exit 2
}
echo "merge-base: $base"
echo "head: $head"

retired='YT2COLUMN_LLM_PROVIDER|YT2COLUMN_MODEL|YT2COLUMN_REVIEW_PROVIDER|YT2COLUMN_REVIEW_MODEL|YT2COLUMN_CLAUDE_EFFORT'
docs='README.md README.en.md docs/dev/guides/cli.md docs/dev/project_overview.md'
glossary=docs/translation_glossary.md
removed_headings='### 廃止した LLM の変数からの移行
### Migrating from the retired LLM variables'

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
status=0

# show <commit> <path> <file>: writes the document at the commit to file.
show() {
	if ! git -C "$root" show "$1:$2" >"$3"; then
		echo "cannot read $2 at $1" >&2
		exit 2
	fi
}

# headings <file>: prints the file's headings, one per line.
headings() {
	awk '/^```/ { fence = !fence; next } !fence && /^#/ { print }' "$1"
}

for doc in $docs; do
	show "$base" "$doc" "$tmp/base"
	show "$head" "$doc" "$tmp/head"
	if ! grep -Eq "$retired" "$tmp/base"; then
		echo "$doc names no retired variable at the merge-base" >&2
		status=1
	fi
	if grep -En "$retired" "$tmp/head" >&2; then
		echo "$doc still names a retired variable at the head (lines above)" >&2
		status=1
	else
		echo "$doc: no retired variable at the head"
	fi
	headings "$tmp/base" | grep -vxF "$removed_headings" >"$tmp/base.h" || true
	headings "$tmp/head" >"$tmp/head.h"
	# Every remaining base heading, in order, must be found in the head's
	# headings after the previous one found.
	if missing=$(awk 'NR == FNR { want[++n] = $0; next }
		i < n && $0 == want[i + 1] { i++ }
		END { if (i < n) { print want[i + 1]; exit 1 } }' "$tmp/base.h" "$tmp/head.h"); then
		echo "$doc: the other headings are kept in order"
	else
		echo "$doc: the heading \"$missing\" is missing at the head or out of order" >&2
		status=1
	fi
done

show "$base" "$glossary" "$tmp/base"
show "$head" "$glossary" "$tmp/head"
for row in '| 廃止した LLM の変数 |' '| 廃止した LLM の変数からの移行 |'; do
	if ! grep -qF -- "$row" "$tmp/base"; then
		echo "$glossary has no row \"$row\" at the merge-base" >&2
		status=1
	elif grep -qF -- "$row" "$tmp/head"; then
		echo "$glossary still has the row \"$row\" at the head" >&2
		status=1
	else
		echo "$glossary: the row \"$row\" is removed"
	fi
done

exit "$status"
