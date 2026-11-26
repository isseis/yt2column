#!/usr/bin/env bash
# Compares the internal/chief tests of the merge-base (before) with the work
# tree (after): elapsed time ratio (AC-01), set of passing test names with no
# fail or skip (AC-03), and the list of changed files (AC-05).
# Run from the work tree after `git fetch origin main`, as a non-root user.
# Exit: 0 all checks pass, 1 some check fails, 2 cannot decide.
set -u

die() { echo "compare.sh: cannot decide: $*" >&2; exit 2; }

[ "$(id -u)" -ne 0 ] || die "run as a non-root user"
cd "$(git rev-parse --show-toplevel)" || die "no work tree"
base=$(git merge-base origin/main HEAD) || die "no merge-base"
head=$(git rev-parse HEAD) || die "no HEAD"
if [ -n "$(git status --porcelain)" ]; then clean=no; else clean=yes; fi

tmp=$(mktemp -d) || die "mktemp"
trap 'rm -rf "$tmp"' EXIT

mkdir "$tmp/before" || die "mkdir"
git archive "$base" | tar -x -C "$tmp/before" || die "git archive"

# The tests may fail; the verdict is read from the JSON, not from the status.
(cd "$tmp/before" && go test -tags test -race -count=1 -json ./internal/chief >"$tmp/before.json" 2>"$tmp/before.err")
go test -tags test -race -count=1 -json ./internal/chief >"$tmp/after.json" 2>"$tmp/after.err"

# Package-level pass line: the only line without a Test member.
elapsed() {
	local out
	out=$(jq -r 'select(.Action == "pass" and (has("Test") | not)) | .Elapsed' "$1") || return 1
	[ "$(printf '%s\n' "$out" | grep -c .)" -eq 1 ] || return 1
	printf '%s\n' "$out"
}
names() { jq -r 'select(.Action == "pass" and has("Test")) | .Test' "$1" | LC_ALL=C sort; }
bad() { jq -r 'select((.Action == "fail" or .Action == "skip") and has("Test")) | .Test' "$1"; }

eb=$(elapsed "$tmp/before.json") || die "no single package pass line before"
ea=$(elapsed "$tmp/after.json") || die "no single package pass line after"
names "$tmp/before.json" >"$tmp/before.names" || die "names before"
names "$tmp/after.json" >"$tmp/after.names" || die "names after"
[ -s "$tmp/before.names" ] || die "no passing tests before"
bad "$tmp/before.json" >"$tmp/before.bad" || die "bad before"
bad "$tmp/after.json" >"$tmp/after.bad" || die "bad after"

fail=0
ratio=$(awk -v a="$ea" -v b="$eb" 'BEGIN { printf "%.3f", a / b }') || die "ratio"
if awk -v r="$ratio" 'BEGIN { exit !(r <= 0.5) }'; then r1=PASS; else r1=FAIL; fail=1; fi

if cmp -s "$tmp/before.names" "$tmp/after.names" && [ ! -s "$tmp/before.bad" ] && [ ! -s "$tmp/after.bad" ]; then r3=PASS; else r3=FAIL; fail=1; fi

allowed='^(internal/chief/[^/]*_test\.go|internal/chief/test_helpers\.go|docs/tasks/0128_chief_test_speedup/.*)$'
{ git diff --name-only "$base" && git ls-files --others --exclude-standard; } >"$tmp/changed" || die "changed files"
if grep -Evq "$allowed" "$tmp/changed"; then r5=FAIL; fail=1; else r5=PASS; fi

echo "before (merge-base): $base"
echo "after  (HEAD):       $head (work tree clean: $clean)"
echo "elapsed before: ${eb}s, after: ${ea}s, ratio: $ratio"
echo "passing tests before: $(wc -l <"$tmp/before.names" | tr -d ' '), after: $(wc -l <"$tmp/after.names" | tr -d ' ')"
echo "fail/skip before: $(grep -c . "$tmp/before.bad"), after: $(grep -c . "$tmp/after.bad")"
echo "changed files: $(grep -c . "$tmp/changed")"
grep -Ev "$allowed" "$tmp/changed" | cat -v
echo "AC-01 (ratio <= 0.5): $r1"
echo "AC-03 (same passing names, no fail/skip): $r3"
echo "AC-05 (only allowed files changed): $r5"
exit "$fail"
