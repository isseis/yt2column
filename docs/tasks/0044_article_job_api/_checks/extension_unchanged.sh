#!/bin/sh
# Checks that none of the given commits changed extension/, which this task
# leaves untouched (#503 changes it later). Each argument is a commit ID; the
# check compares the commit with its parent, and a root commit with the empty
# tree. The pathspec is repository-root-relative, so the script may be run from
# any directory inside the repository.
#
# Output and exit codes:
#   0  every commit left extension/ unchanged (one line per commit on stdout)
#   1  at least one commit changed extension/ (the commit and its files on
#      stderr; the clean commits still print their lines on stdout)
#   2  the arguments or the repository are unusable (no commit given, a value
#      that is not a commit, no repository, or no extension/ directory)
#
# Run it once on the task's final PR branch, with the squash commits of every
# task PR and the branch's own commits:
#
#	docs/tasks/0044_article_job_api/_checks/extension_unchanged.sh \
#		$(git rev-list --reverse origin/main..HEAD) <squash commits>
#
# make test and CI do not run it. It reads no environment variable and names no
# secret.
set -eu

if [ "$#" -eq 0 ]; then
	echo "usage: $0 <commit>..." >&2
	exit 2
fi

if ! root=$(git rev-parse --show-toplevel 2>/dev/null); then
	echo "not a git repository" >&2
	exit 2
fi
if [ ! -d "$root/extension" ]; then
	echo "no extension/ directory under $root" >&2
	exit 2
fi

empty_tree=$(git -C "$root" hash-object -t tree /dev/null)
failed=0
for commit in "$@"; do
	if ! git -C "$root" cat-file -e "$commit^{commit}" 2>/dev/null; then
		echo "not a commit: $commit" >&2
		exit 2
	fi
	if parent=$(git -C "$root" rev-parse --verify --quiet "$commit^"); then
		base=$parent
	else
		base=$empty_tree
	fi
	changed=$(git -C "$root" diff --name-only "$base" "$commit" -- ':(top)extension/')
	if [ -n "$changed" ]; then
		echo "commit $commit changed extension/:" >&2
		printf '%s\n' "$changed" >&2
		failed=1
		continue
	fi
	echo "$commit: extension/ unchanged"
done

if [ "$failed" -ne 0 ]; then
	exit 1
fi
