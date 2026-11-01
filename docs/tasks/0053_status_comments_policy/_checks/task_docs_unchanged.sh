#!/bin/sh
# Checks that a branch of task 0053 changes no task document of another task:
# every path under docs/tasks/ that differs between the merge-base and the
# head lies under docs/tasks/0053_status_comments_policy/. Renames are not
# detected (--no-renames), so moving another task's file into this task's
# directory counts as deleting it from its own place. The pathspecs are
# repository-root-relative (:(top)), so the script may be run from any
# directory inside the repository, and a directory whose name only starts
# with this task's directory name is another task's.
#
# Usage:
#
#	docs/tasks/0053_status_comments_policy/_checks/task_docs_unchanged.sh <upstream> <head>
#
# The merge-base is that of <upstream> and <head>. On a PR branch of the task,
# after git fetch origin main:
#
#	docs/tasks/0053_status_comments_policy/_checks/task_docs_unchanged.sh origin/main HEAD
#
# Output and exit codes:
#   0  no other task's document changed (the merge-base and the head on stdout)
#   1  another task's document changed (the merge-base and the head on stdout,
#      the changed paths on stderr)
#   2  the arguments or the repository are unusable (not two arguments, a
#      value that is not a commit, no repository, or no docs/tasks/ directory)
#
# Run it once on each PR branch of the task, before its PR checkpoint. make
# test and CI do not run it. It reads no environment variable and names no
# secret.
set -eu

task=docs/tasks/0053_status_comments_policy

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
head=$(git -C "$root" rev-parse --verify "$2^{commit}")
if ! base=$(git -C "$root" merge-base "$1" "$head"); then
	echo "no merge-base of $1 and $2" >&2
	exit 2
fi
echo "merge-base: $base"
echo "head: $head"

changed=$(git -C "$root" diff --no-renames --name-only "$base" "$head" -- \
	':(top)docs/tasks/' ":(top,exclude)$task/")
if [ -n "$changed" ]; then
	echo "changed outside $task/:" >&2
	printf '%s\n' "$changed" >&2
	exit 1
fi
echo "docs/tasks/: only $task/ changed"
