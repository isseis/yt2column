#!/usr/bin/env bash
#
# correction_lines.sh BASE
#
# Verifies how the PR records the rewrite of ADR-0014 as an editorial
# correction, over BASE..HEAD of the PR branch, with BASE the merge-base of
# origin/main and HEAD.
#
# A correction line is a whole line of a commit message that starts in its
# first column with "Editorial correction: ". The checks:
#
#   1. The range changes ADR-0014, and at least one commit that changes it has
#      a correction line that names ADR-0014's path. No correction line is
#      required of each such commit: a later commit that revises text the PR
#      itself added is not an editorial correction.
#   2. Every correction line of the range has the fixed form: a repository
#      path ending in .md, a non-empty description, and the closing
#      "; no decision changed".
#   3. No correction line of the range contains "<" or "@".
#
# It prints one line per check (ok or fail, with a reason) and exits 1 when a
# check fails. On a failure of git (a bad BASE, an unreadable commit) it prints
# a diagnostic on standard error and exits 2: a range that cannot be read is
# not a range with no correction line.

set -u -o pipefail
export LC_ALL=C

adr_path='docs/dev/adr/0014-deterministic-checks-in-go.md'

usage() {
	echo "usage: correction_lines.sh BASE" >&2
	exit 2
}

[ "$#" -eq 1 ] || usage
base=$1

if ! head=$(git rev-parse --verify 'HEAD^{commit}'); then
	echo "correction_lines: cannot read HEAD" >&2
	exit 2
fi
if ! git rev-parse --verify "${base}^{commit}" >/dev/null; then
	echo "correction_lines: cannot read base $base" >&2
	exit 2
fi
if ! commits=$(git rev-list "$base..$head"); then
	echo "correction_lines: cannot list $base..$head" >&2
	exit 2
fi

# One pass over the range: check 1 needs the commit that holds a line, and
# checks 2 and 3 read every line by itself, so no line has to be kept.
adr_commits=0
named_by_change=0
form_failures=0
forbidden=0
line_count=0
commit_count=0
while IFS= read -r commit; do
	[ -n "$commit" ] || continue
	commit_count=$((commit_count + 1))
	changed=$(git show --pretty=format: --name-only "$commit") || {
		echo "correction_lines: cannot read commit $commit" >&2
		exit 2
	}
	changes_adr=0
	while IFS= read -r path; do
		if [ "$path" = "$adr_path" ]; then
			changes_adr=1
		fi
	done <<<"$changed"
	if [ "$changes_adr" -eq 1 ]; then
		adr_commits=$((adr_commits + 1))
	fi
	message=$(git show -s --format=%B "$commit") || {
		echo "correction_lines: cannot read the message of $commit" >&2
		exit 2
	}
	while IFS= read -r line; do
		case $line in
		'Editorial correction: '*) ;;
		*) continue ;;
		esac
		line_count=$((line_count + 1))

		# Check 1: a line of a commit that changes the ADR names its path.
		if [ "$changes_adr" -eq 1 ] && [[ $line == *"$adr_path"* ]]; then
			named_by_change=1
		fi

		# Check 2: path, description, and closing phrase.
		rest=${line#'Editorial correction: '}
		path=${rest%%: *}
		description=${rest#*: }
		if [[ ! $path =~ ^[A-Za-z0-9._/-]+\.md$ ]]; then
			echo "fail: no repository .md path before the colon: $line"
			form_failures=$((form_failures + 1))
		elif [[ $description != *'; no decision changed' ]]; then
			echo "fail: no closing \"; no decision changed\": $line"
			form_failures=$((form_failures + 1))
		else
			what=${description%; no decision changed}
			if [ -z "${what// /}" ]; then
				echo "fail: empty description: $line"
				form_failures=$((form_failures + 1))
			fi
		fi

		# Check 3: no "<" and no "@".
		if [[ $line == *'<'* || $line == *'@'* ]]; then
			echo "fail: contains '<' or '@': $line"
			forbidden=$((forbidden + 1))
		fi
	done <<<"$message"
done <<<"$commits"

status=0

if [ "$adr_commits" -eq 0 ]; then
	echo "fail: no commit of $base..$head changes $adr_path"
	status=1
elif [ "$named_by_change" -ne 1 ]; then
	echo "fail: no commit that changes $adr_path has an editorial correction line naming it"
	status=1
else
	echo "ok: a commit that changes $adr_path has an editorial correction line naming it"
fi

if [ "$form_failures" -ne 0 ]; then
	status=1
else
	echo "ok: every editorial correction line has the fixed form"
fi

if [ "$forbidden" -ne 0 ]; then
	status=1
else
	echo "ok: no editorial correction line contains '<' or '@'"
fi

echo "commits in $base..$head: $commit_count"
echo "commits that change $adr_path: $adr_commits"
echo "editorial correction lines: $line_count"

exit "$status"
