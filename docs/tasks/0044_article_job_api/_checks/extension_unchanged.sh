#!/bin/sh
# Checks that none of the given commits changed extension/, which this task
# leaves untouched (#503 changes it later). Each argument is a commit ID; the
# check compares the commit with its parent, and a root commit with the empty
# tree. On a change it prints the commit and the files and exits 1.
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

empty_tree=$(git hash-object -t tree /dev/null)
failed=0
for commit in "$@"; do
	if ! git cat-file -e "$commit^{commit}" 2>/dev/null; then
		echo "not a commit: $commit" >&2
		exit 2
	fi
	if parent=$(git rev-parse --verify --quiet "$commit^"); then
		base=$parent
	else
		base=$empty_tree
	fi
	changed=$(git diff --name-only "$base" "$commit" -- extension/)
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
