#!/usr/bin/env bash
# AC-22 of task 0117: the change adds no guard test that compares the output
# JSON's fields or the flags with a document. It checks, against the
# merge-base of origin/main and the head:
#   1. among the changed *_test.go files, only cmd/chief/docs_test.go changed;
#   2. the test functions removed from that file are exactly
#      TestGuideValuesMatchTool;
#   3. the test functions added to it are exactly the two the plan's 4.1
#      names.
#
# Usage: check-ac22.sh <base> [<head>]

set -u

base=${1:?usage: check-ac22.sh <base> [<head>]}
head=${2:-HEAD}
doc=cmd/chief/docs_test.go
status=0

changed=$(git diff --name-only "$base" "$head" -- '*_test.go')
extra=$(printf '%s\n' "$changed" | grep -v -x -e "$doc" -e '' || true)
printf 'changed test files:\n%s\n' "${changed:-（なし）}"
if [ -n "$extra" ]; then
	printf 'FAIL: a test file other than %s changed:\n%s\n' "$doc" "$extra"
	status=1
fi

functions() {
	git show "$1:$doc" | sed -n 's/^func \(Test[A-Za-z0-9_]*\)(.*/\1/p' | sort
}
removed=$(comm -23 <(functions "$base") <(functions "$head"))
added=$(comm -13 <(functions "$base") <(functions "$head"))
printf 'removed test functions:\n%s\nadded test functions:\n%s\n' "${removed:-（なし）}" "${added:-（なし）}"

if [ "$removed" != "TestGuideValuesMatchTool" ]; then
	printf 'FAIL: the removed test functions are not exactly TestGuideValuesMatchTool\n'
	status=1
fi
want_added=$(printf '%s\n' TestArchitectureValuesMatchTool TestGuideProtocolMatchesTool)
if [ "$added" != "$want_added" ]; then
	printf 'FAIL: the added test functions are not exactly the two the plan names\n'
	status=1
fi

if [ "$status" -eq 0 ]; then
	printf 'OK\n'
fi
exit "$status"
