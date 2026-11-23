#!/usr/bin/env bash
# Verify that .github/workflows/ci.yml changes between <base> and <commit>
# stay strictly within the check-changes job definition:
#   (a) both "  check-changes:" and "  test:" appear exactly once in that order
#       in both commits;
#   (b) all bytes before "  check-changes:" and all bytes from "  test:" to EOF
#       match byte-for-byte between <base> and <commit>;
#   (c) no sibling job or top-level key (^ ? ?[^ #]) is inserted between
#       "  check-changes:" and "  test:" in <commit>.
#
# Usage: bash check-ac16-ci-diff.sh <base-commit> <compared-commit>
set -eu -o pipefail
export LC_ALL=C

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <base-commit> <compared-commit>" >&2
  exit 2
fi

WORKFLOW=".github/workflows/ci.yml"
BASE=$(git --no-replace-objects rev-parse --verify "${1}^{commit}")
HEAD=$(git --no-replace-objects rev-parse --verify "${2}^{commit}")

TMP=$(mktemp -d "${TMPDIR:-/tmp}/check-ac16.XXXXXX")
trap 'rm -rf "$TMP"' EXIT

git --no-replace-objects cat-file blob "${BASE}:${WORKFLOW}" > "$TMP/base.yml"
git --no-replace-objects cat-file blob "${HEAD}:${WORKFLOW}" > "$TMP/head.yml"

find_anchors() {
  local file=$1 label=$2
  local c_lines t_lines c_count t_count c_line t_line

  c_lines=$(grep -n '^  check-changes:$' "$file" | cut -d: -f1 || true)
  t_lines=$(grep -n '^  test:$' "$file" | cut -d: -f1 || true)

  c_count=$(printf '%s\n' "$c_lines" | awk 'NF { n++ } END { print n + 0 }')
  t_count=$(printf '%s\n' "$t_lines" | awk 'NF { n++ } END { print n + 0 }')

  if [ "$c_count" -ne 1 ]; then
    echo "FAIL ($label): expected exactly 1 '  check-changes:' line, found $c_count" >&2
    return 1
  fi
  if [ "$t_count" -ne 1 ]; then
    echo "FAIL ($label): expected exactly 1 '  test:' line, found $t_count" >&2
    return 1
  fi

  c_line=$c_lines
  t_line=$t_lines
  if [ "$c_line" -ge "$t_line" ]; then
    echo "FAIL ($label): '  check-changes:' (line $c_line) must precede '  test:' (line $t_line)" >&2
    return 1
  fi
  printf '%s %s\n' "$c_line" "$t_line"
}

BASE_ANCHORS=$(find_anchors "$TMP/base.yml" "base")
HEAD_ANCHORS=$(find_anchors "$TMP/head.yml" "compared")
read -r BASE_START BASE_TEST <<< "$BASE_ANCHORS"
read -r HEAD_START HEAD_TEST <<< "$HEAD_ANCHORS"

slice_before() {
  local file=$1 line=$2 out=$3
  if [ "$line" -le 1 ]; then
    : > "$out"
  else
    head -n $((line - 1)) "$file" > "$out"
  fi
}

slice_before "$TMP/base.yml" "$BASE_START" "$TMP/base.before"
slice_before "$TMP/head.yml" "$HEAD_START" "$TMP/head.before"
if ! cmp -s "$TMP/base.before" "$TMP/head.before"; then
  echo "FAIL: bytes before '  check-changes:' differ between base and compared commit" >&2
  exit 1
fi

tail -n +"$BASE_TEST" "$TMP/base.yml" > "$TMP/base.after"
tail -n +"$HEAD_TEST" "$TMP/head.yml" > "$TMP/head.after"
if ! cmp -s "$TMP/base.after" "$TMP/head.after"; then
  echo "FAIL: bytes from '  test:' to EOF differ between base and compared commit" >&2
  exit 1
fi

INTRUDERS=$(awk -v s="$HEAD_START" -v e="$HEAD_TEST" \
  'NR > s && NR < e && /^ ? ?[^ #]/ { printf "%d:%s\n", NR, $0 }' "$TMP/head.yml")
if [ -n "$INTRUDERS" ]; then
  echo "FAIL: unexpected top-level key or sibling job between '  check-changes:' and '  test:':" >&2
  printf '%s\n' "$INTRUDERS" >&2
  exit 1
fi

echo "PASS: ${WORKFLOW} diff is confined to check-changes"
echo "  base:     ${BASE} (check-changes:${BASE_START}, test:${BASE_TEST})"
echo "  compared: ${HEAD} (check-changes:${HEAD_START}, test:${HEAD_TEST})"
