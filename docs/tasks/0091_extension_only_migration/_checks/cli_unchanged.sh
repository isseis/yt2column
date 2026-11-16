#!/bin/sh
# Checks that the branch changes no file under cmd/yt2column/ other than a
# _test.go file (AC-26: the CLI's behavior stays as it is). It compares HEAD with
# the merge-base of origin/main and HEAD, so run it after `git fetch origin main`
# and with the work committed: an uncommitted change is not read.
#
# Output and exit codes:
#   0  no such file changed (the merge-base and HEAD IDs are on stdout)
#   1  at least one changed (the files are on stderr)
#   2  the repository or origin/main is unusable
#
# Not run by make test or CI; it reads no environment variable and names no
# secret.
set -eu

root=$(git rev-parse --show-toplevel) || exit 2
base=$(git -C "$root" merge-base origin/main HEAD) || exit 2
head=$(git -C "$root" rev-parse HEAD) || exit 2
changed=$(git -C "$root" diff --no-renames --name-only "$base" "$head" -- cmd/yt2column/) || exit 2
changed=$(printf '%s\n' "$changed" | grep -v -e '^$' -e '_test\.go$' || true)

echo "merge-base $base head $head"
if [ -n "$changed" ]; then
	echo "non-test files under cmd/yt2column/ changed:" >&2
	printf '%s\n' "$changed" >&2
	exit 1
fi
echo "ok: no non-test file under cmd/yt2column/ changed"
