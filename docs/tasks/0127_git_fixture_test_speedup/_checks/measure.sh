#!/usr/bin/env bash
# Temporary script of task 0127: measure the wall time of the two packages at
# BASE and AFTER, alternating the two, and print the per-package medians and
# ratios plus the machine information.
#
# Usage: measure.sh BASE AFTER
# Narrowing (for the breakage check only; a narrowed run is not a measurement):
#   CHECK_PKGS="./cmd/yt2column ./internal/mergepr"  packages to run
#   CHECK_RUN=<regexp>                                -run value (default: none)
#   CHECK_ROUNDS=3                                    rounds per commit (odd)
set -euo pipefail

# Run from a copy outside the work tree: checking out BASE removes this file.
if [ -z "${CHECK_COPY:-}" ]; then
  tmp=$(mktemp -d)
  cp "$0" "$tmp/measure.sh"
  CHECK_COPY=1 CHECK_TMP=$tmp exec bash "$tmp/measure.sh" "$@"
fi

fail() { echo "measure: FAIL: $*" >&2; exit 1; }

[ "$#" -eq 2 ] || fail "usage: measure.sh BASE AFTER"
cd "$(git rev-parse --show-toplevel)"
base=$(git rev-parse --verify "$1^{commit}") || fail "unknown BASE"
after=$(git rev-parse --verify "$2^{commit}") || fail "unknown AFTER"
pkgs=${CHECK_PKGS:-"./cmd/yt2column ./internal/mergepr"}
rounds=${CHECK_ROUNDS:-3}
[ $((rounds % 2)) -eq 1 ] || fail "CHECK_ROUNDS must be odd"
mod=$(awk '/^module /{print $2; exit}' go.mod)
[ -n "$mod" ] || fail "cannot read the module path"

[ -z "$(git status --porcelain)" ] || fail "the work tree is not clean"

# Restore the original branch (or commit) however the script ends.
orig_sha=$(git rev-parse HEAD)
orig_branch=$(git symbolic-ref --quiet --short HEAD || true)
restore() {
  status=$?
  trap - EXIT
  if [ -n "$orig_branch" ]; then
    git switch --quiet "$orig_branch" || status=1
  else
    git switch --quiet --detach "$orig_sha" || status=1
  fi
  [ "$(git symbolic-ref --quiet --short HEAD || true)" = "$orig_branch" ] || { echo "measure: STOP: not on the original branch" >&2; status=1; }
  [ "$(git rev-parse HEAD)" = "$orig_sha" ] || { echo "measure: FAIL: not back at $orig_sha" >&2; status=1; }
  [ -z "$(git status --porcelain)" ] || { echo "measure: FAIL: the work tree is not clean after the run" >&2; status=1; }
  rm -rf "$CHECK_TMP"
  exit "$status"
}
trap restore EXIT

echo "== machine =="
echo "os: $(uname -srm)"
if cores=$(sysctl -n hw.ncpu 2>/dev/null); then :; else cores=$(nproc); fi
[ -n "$cores" ] || fail "cannot read the core count"
echo "cores: $cores"
echo "path-bash: $(command -v bash) $(bash -c 'echo $BASH_VERSION')"
echo "bin-bash: /bin/bash $(/bin/bash -c 'echo $BASH_VERSION')"
echo "go: $(go version)"
echo "GOMAXPROCS: ${GOMAXPROCS:-unset}"
echo "BASE: $base"
echo "AFTER: $after"
echo "packages: $pkgs"
echo "rounds: $rounds"

results=$CHECK_TMP/results.txt
: > "$results"

run_once() { # label sha round
  local label=$1 sha=$2 round=$3 log=$CHECK_TMP/run.log
  git switch --quiet --detach "$sha"
  local args=(-tags test -race -count=1)
  [ -z "${CHECK_RUN:-}" ] || args+=(-run "$CHECK_RUN")
  # shellcheck disable=SC2086
  go test "${args[@]}" $pkgs > "$log" 2>&1 || { cat "$log" >&2; fail "go test failed at $label round $round"; }
  local p secs
  for p in $pkgs; do
    secs=$(sed -nE "s#^ok[[:space:]]+${mod}/${p#./}[[:space:]]+([0-9.]+)s\$#\\1#p" "$log")
    [ "$(printf '%s\n' "$secs" | grep -c .)" -eq 1 ] || { cat "$log" >&2; fail "cannot read the time of $p at $label round $round"; }
    echo "$label $round $p $secs" >> "$results"
    echo "$label round $round $p ${secs}s"
  done
}

echo "== runs =="
i=1
while [ "$i" -le "$rounds" ]; do
  run_once BASE "$base" "$i"
  run_once AFTER "$after" "$i"
  i=$((i + 1))
done

echo "== medians =="
for p in $pkgs; do
  med() { awk -v l="$1" -v p="$p" '$1==l && $3==p {print $4}' "$results" | sort -n | awk '{a[NR]=$1} END{print a[int((NR+1)/2)]}'; }
  b=$(med BASE)
  a=$(med AFTER)
  awk -v p="$p" -v b="$b" -v a="$a" 'BEGIN{ if (b+0 <= 0) exit 1; printf "%s BASE %.3fs AFTER %.3fs ratio %.3f\n", p, b, a, a/b }' || fail "bad median for $p"
done
echo "measure: DONE"
