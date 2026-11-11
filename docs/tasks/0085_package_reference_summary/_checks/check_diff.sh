#!/bin/sh
# Checks the scope of an implementation PR of task 0085 between two commits:
#
#   1. every changed path is inside the allowed set: the package reference,
#      existing Go files under cmd/, internal/, and prompts/, the ADRs, the
#      two documents the task edits, and this task's directory;
#   2. no Go file is added or deleted;
#   3. every Go file's diff is comment-only (lines starting with // and blank
#      lines), except internal/pipeline/strictjson_users_test.go, which the
#      task changes to read the strictjson row from every table.
#
# Usage:
#
#	docs/tasks/0085_package_reference_summary/_checks/check_diff.sh <base> <head>
#
# <base> is the merge-base of the PR branch with origin/main and <head> is the
# PR's commit. On a PR branch, after git fetch origin main:
#
#	base=$(git merge-base origin/main HEAD)
#	docs/tasks/0085_package_reference_summary/_checks/check_diff.sh "$base" HEAD
#
# Run it once on each PR branch of the task, before its PR checkpoint. make
# test and CI do not run it. It reads no environment variable and names no
# secret.
#
# Output and exit codes:
#   0  every check passed (the base, the head, the counts, and a result line)
#   1  a check failed (the reason on stderr)
#   2  the arguments or the repository are unusable (not two commits, base not
#      an ancestor of head, no repository, or a git command that fails)
set -eu
export LC_ALL=C

doc=docs/dev/conventions/package_reference.md
task=docs/tasks/0085_package_reference_summary
test_file=internal/pipeline/strictjson_users_test.go

if [ "$#" -ne 2 ]; then
	echo "usage: $0 <base> <head>" >&2
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
if ! base=$(git -C "$root" rev-parse --verify "$1^{commit}"); then
	echo "cannot resolve $1" >&2
	exit 2
fi
if ! head=$(git -C "$root" rev-parse --verify "$2^{commit}"); then
	echo "cannot resolve $2" >&2
	exit 2
fi
if ! git -C "$root" merge-base --is-ancestor "$base" "$head"; then
	echo "$base is not an ancestor of $head" >&2
	exit 2
fi
echo "base: $base"
echo "head: $head"

if ! changed=$(git -C "$root" diff --no-renames --name-status "$base" "$head"); then
	echo "git diff failed" >&2
	exit 2
fi

ret=0
files=0
go_files=0

# A Go diff is comment-only when every added or removed line is blank or starts
# with //; the file headers and the hunk headers are not content.
check_go_comment_only() {
	git -C "$root" diff --no-renames --unified=0 --no-color "$base" "$head" -- "$1" |
		awk '
		/^\+\+\+ / || /^--- / { next }
		/^\+/ || /^-/ {
			line = substr($0, 2)
			if (line ~ /^[[:space:]]*$/) next
			if (line ~ /^[[:space:]]*\/\//) next
			printf "non-comment line in %s: %s\n", file, $0
			bad = 1
		}
		END { exit bad ? 1 : 0 }
		' file="$1"
}

old_ifs=$IFS
IFS='
'
for entry in $changed; do
	IFS=$old_ifs
	status=${entry%%	*}
	path=${entry#*	}
	if [ -z "$path" ]; then
		continue
	fi
	files=$((files + 1))

	case "$path" in
	"$doc" | CLAUDE.md | docs/dev/guides/pricing_update.md) ;;
	docs/dev/adr/*.md) ;;
	"$task"/*) ;;
	cmd/* | internal/* | prompts/*)
		case "$path" in
		*.go) ;;
		*)
			echo "a non-Go file under cmd/, internal/, or prompts/: $path" >&2
			ret=1
			continue
			;;
		esac
		;;
	*)
		echo "a path outside the allowed set ($status): $path" >&2
		ret=1
		continue
		;;
	esac

	case "$path" in
	*.go)
		go_files=$((go_files + 1))
		case "$status" in
		A)
			echo "a Go file is added: $path" >&2
			ret=1
			continue
			;;
		D)
			echo "a Go file is deleted: $path" >&2
			ret=1
			continue
			;;
		esac
		if [ "$path" = "$test_file" ]; then
			continue
		fi
		if ! check_go_comment_only "$path"; then
			echo "the Go diff of $path is not comment-only" >&2
			ret=1
		fi
		;;
	esac
done
IFS=$old_ifs

echo "changed files: $files"
echo "Go files checked: $go_files (comment-only except $test_file)"

if [ "$ret" -ne 0 ]; then
	echo "FAIL: the PR changes paths or Go lines outside the task's scope"
	exit 1
fi
echo "OK: the PR stays in scope and its Go diffs are comment-only"
