#!/usr/bin/env bash
# Temporary check for AC-03 of task 0129 (not committed as a test).
#
# Usage: break_parse.sh <commit> [go-test-timeout]
#
# Extracts <commit> with git archive into a temporary directory, so the work
# tree is never touched. In that tree it first runs the seven large-input
# subtests unbroken (all must pass), then applies each patch under
# breakages/ one at a time and runs only the subtest of the path that patch
# breaks, with -race. A breakage counts as caught only when its subtest fails
# with the allocation-bound error ("parse allocated ... want at most ... per
# byte"): a timeout, a parse error, or a wrong segment count does not count.
# Exits 0 only when the unbroken run passes and all seven breakages are caught.
set -u

commit=${1:?usage: break_parse.sh <commit> [go-test-timeout]}
gotimeout=${2:-90m}
here=$(cd "$(dirname "$0")" && pwd)
repo=$(git -C "$here" rev-parse --show-toplevel) || exit 2
tmp=$(mktemp -d "${TMPDIR:-/tmp}/break_parse.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT

full=$(git -C "$repo" rev-parse --verify "$commit^{commit}") || { echo "unknown commit: $commit"; exit 2; }
git -C "$repo" archive "$full" | tar -x -C "$tmp" || { echo "cannot extract $full"; exit 2; }
echo "commit: $full"
echo "go: $(go version)"

# patch file -> subtest name (spaces become underscores in -run patterns)
names=(
  "01_shortest_form_a_lines:shortest_form_A_lines"
  "02_one_long_form_a_line:one_long_form_A_line"
  "03_textless_form_b_timestamps:textless_form_B_timestamps"
  "04_trailing_zero_length_lines:trailing_zero-length_lines"
  "05_one_byte_prefix_lines:one-byte_prefix_lines"
  "06_form_a_headings:form_A_headings_between_segments"
  "07_form_b_headings:one-byte_form_B_headings"
)

status=0
run_test() { # $1 = -run pattern, $2 = output file
  (cd "$tmp" && go test -tags test -race -count=1 -v -timeout "$gotimeout" -run "$1" ./internal/transcript >"$2" 2>&1)
}

echo "== unbroken tree =="
if run_test '^TestParseCopiedLargeInputs$' "$tmp.control.out"; then
  grep -E '^\s+copied_format_test.go:[0-9]+: [0-9]+ bytes' "$tmp.control.out"
  echo "unbroken: PASS"
else
  tail -n 20 "$tmp.control.out"
  echo "unbroken: FAIL"
  status=1
fi

for entry in "${names[@]}"; do
  patch=${entry%%:*}
  sub=${entry#*:}
  echo "== breakage $patch (subtest $sub) =="
  if ! (cd "$tmp" && git apply --whitespace=nowarn "$here/breakages/$patch.patch"); then
    echo "$patch: cannot apply"
    status=1
    continue
  fi
  start=$(date +%s)
  run_test "^TestParseCopiedLargeInputs\$/^${sub}\$" "$tmp.out"
  elapsed=$(( $(date +%s) - start ))
  (cd "$tmp" && git apply -R --whitespace=nowarn "$here/breakages/$patch.patch")
  line=$(grep -E 'parse allocated [0-9]+ bytes for [0-9]+ input bytes, want at most [0-9]+ per byte' "$tmp.out" | head -n 1)
  if grep -qE "^\s+--- FAIL: TestParseCopiedLargeInputs/${sub} " "$tmp.out" \
     && [ -n "$line" ] \
     && ! grep -qE 'parse error =|parsed into [0-9]+ segments|test timed out|^panic:' "$tmp.out"; then
    echo "$patch: CAUGHT in ${elapsed}s: $line"
  else
    echo "$patch: NOT CAUGHT by the allocation check (${elapsed}s)"
    tail -n 15 "$tmp.out"
    status=1
  fi
done

rm -f "$tmp.control.out" "$tmp.out"
if [ "$status" -eq 0 ]; then echo "RESULT: PASS (unbroken passes; 7 of 7 caught)"; else echo "RESULT: FAIL"; fi
exit "$status"
