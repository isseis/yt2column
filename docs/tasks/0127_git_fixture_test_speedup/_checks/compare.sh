#!/usr/bin/env bash
# Temporary script of task 0127: compare the test names, failures and skips of
# the two packages at BASE and AFTER, run the single selections and the
# shuffled runs at AFTER, and check the range of the changed files.
#
# Usage: compare.sh BASE AFTER MIDDLE_NAME...
#   MIDDLE_NAME: a leaf test name that uses a shared state (the plan's table);
#   it must be in BASE and neither the first nor the last leaf of its
#   top-level test function.
# Narrowing (for the breakage check only; a narrowed run is not a comparison):
#   CHECK_PKGS="./cmd/yt2column ./internal/mergepr"  packages to run
#   CHECK_RUN=<regexp>                                -run of the full runs
#   CHECK_TOPS="TestA TestB"                          top-level functions of the single selections
set -euo pipefail

# Run from a copy outside the work tree: checking out BASE removes this file.
if [ -z "${CHECK_COPY:-}" ]; then
  tmp=$(mktemp -d)
  cp "$0" "$tmp/compare.sh"
  CHECK_COPY=1 CHECK_TMP=$tmp exec bash "$tmp/compare.sh" "$@"
fi

die() { echo "compare: STOP: $*" >&2; exit 1; }
failed=0
check() { # name status(0=pass) detail
  if [ "$2" -eq 0 ]; then echo "check $1: PASS"; else echo "check $1: FAIL${3:+ ($3)}"; failed=1; fi
}

[ "$#" -ge 2 ] || die "usage: compare.sh BASE AFTER MIDDLE_NAME..."
cd "$(git rev-parse --show-toplevel)"
base=$(git rev-parse --verify "$1^{commit}") || die "unknown BASE"
after=$(git rev-parse --verify "$2^{commit}") || die "unknown AFTER"
shift 2
middles=("$@")
pkgs=${CHECK_PKGS:-"./cmd/yt2column ./internal/mergepr"}
mod=$(awk '/^module /{print $2; exit}' go.mod)
[ -n "$mod" ] || die "cannot read the module path"
T=$CHECK_TMP
[ -z "$(git status --porcelain)" ] || die "the work tree is not clean"

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
  [ "$(git symbolic-ref --quiet --short HEAD || true)" = "$orig_branch" ] || { echo "compare: STOP: not on the original branch" >&2; status=1; }
  [ "$(git rev-parse HEAD)" = "$orig_sha" ] || { echo "compare: STOP: not back at $orig_sha" >&2; status=1; }
  [ -z "$(git status --porcelain)" ] || { echo "compare: STOP: the work tree is not clean after the run" >&2; status=1; }
  rm -rf "$T"
  exit "$status"
}
trap restore EXIT

echo "BASE: $base"
echo "AFTER: $after"
echo "packages: $pkgs"

# json_run OUT ARGS...: go test -json; passes only on exit 0 with a package-level
# pass event for every package.
json_run() {
  local out=$1; shift
  # shellcheck disable=SC2086
  go test -tags test -race -count=1 -json "$@" $pkgs > "$out" 2> "$out.err" || return 1
  local p
  for p in $pkgs; do
    jq -r --arg p "$mod/${p#./}" 'select(.Action=="pass" and .Test==null and .Package==$p) | .Package' "$out" | grep -qx "$mod/${p#./}" || return 1
  done
}

full_args=()
[ -z "${CHECK_RUN:-}" ] || full_args+=(-run "$CHECK_RUN")

echo "== full runs =="
git switch --quiet --detach "$base"
json_run "$T/base.json" "${full_args[@]+"${full_args[@]}"}" || { cat "$T/base.json.err" >&2; die "go test failed at BASE"; }
git switch --quiet --detach "$after"
json_run "$T/after.json" "${full_args[@]+"${full_args[@]}"}" || { cat "$T/after.json.err" >&2; die "go test failed at AFTER"; }
echo "full runs: both passed"

names() { jq -r --arg a "$2" 'select(.Action==$a and .Test!=null) | [.Package,.Test] | @tsv' "$1" | LC_ALL=C sort -u; }
for s in base after; do
  names "$T/$s.json" pass > "$T/$s.pass"
  names "$T/$s.json" skip > "$T/$s.skip"
  jq -r 'select(.Action=="fail") | [.Package,(.Test // "")] | @tsv' "$T/$s.json" > "$T/$s.fail"
  # tops: top-level names seen as pass or skip
  cat "$T/$s.pass" "$T/$s.skip" | awk -F'\t' '$2 !~ /\//' | LC_ALL=C sort -u > "$T/$s.tops"
  # leaves: pass names that are not a "/" prefix of another pass name
  awk -F'\t' '{ k[NR]=$0; s=$2; while (match(s, /\/[^\/]*$/)) { s=substr(s,1,RSTART-1); par[$1 FS s]=1 } }
    END { for (i=1;i<=NR;i++) { split(k[i], f, FS); if (!((f[1] FS f[2]) in par)) print k[i] } }' "$T/$s.pass" | LC_ALL=C sort -u > "$T/$s.leaves"
done
# keep only the tops that BASE has
restrict() { awk -F'\t' 'NR==FNR { t[$0]=1; next } { top=$2; sub(/\/.*/, "", top); if ((($1) FS top) in t) print }' "$T/base.tops" "$1"; }
for s in base after; do
  restrict "$T/$s.leaves" > "$T/$s.leaves.r"
  restrict "$T/$s.skip" > "$T/$s.skip.r"
done
# A leaf name can hold a commit ID that changes every run (-g plus 12 hex digits);
# replace it by one mark for the comparison of the name sets only.
norm() { sed -E 's/-g[0-9a-f]{12}(\/|$)/-g<id>\1/g' "$1" | LC_ALL=C sort -u; }
echo "BASE leaves: $(wc -l < "$T/base.leaves.r"), AFTER leaves: $(wc -l < "$T/after.leaves.r"), BASE tops: $(wc -l < "$T/base.tops")"

echo "== names, fail and skip (compare) =="
[ -s "$T/base.leaves.r" ] || die "BASE has no leaf"
if diff <(norm "$T/base.leaves.r") <(norm "$T/after.leaves.r") > "$T/names.diff"; then
  check "names" 0
else
  check "names" 1 "$(grep -c '^[<>]' "$T/names.diff") differing names"; head -40 "$T/names.diff"
fi
if [ ! -s "$T/base.fail" ] && [ ! -s "$T/after.fail" ]; then check "fail" 0; else check "fail" 1 "fail events present"; fi
if diff <(norm "$T/base.skip.r") <(norm "$T/after.skip.r") > "$T/skip.diff"; then
  check "skip" 0
else
  check "skip" 1 "skip sets differ"; head -40 "$T/skip.diff"
fi

echo "== bash 3 leaves =="
bin_major=$(/bin/bash -c 'echo ${BASH_VERSINFO[0]}')
[ -n "$bin_major" ] || die "cannot read the version of /bin/bash"
if [ "$bin_major" = 3 ]; then
  bad=""
  for f in TestReviewRecordsCoverage TestReviewRecordsUnreviewed TestReviewRecordsCrossCheck TestReviewRecordsFableAlias; do
    n=$(awk -F'\t' -v f="$f" 'index($2, f "/bin-bash-3/")==1' "$T/base.leaves.r" | wc -l)
    [ "$n" -gt 0 ] || bad="$bad $f"
  done
  if [ -z "$bad" ]; then check "bin-bash-3" 0; else check "bin-bash-3" 1 "no bin-bash-3 leaf under:$bad"; fi
else
  echo "bin-bash-3: not applicable (/bin/bash is version $bin_major)"
fi

echo "== single selections (AFTER) =="
git switch --quiet --detach "$after"
quote_pattern() { # leaf name -> go test -run pattern, one anchored element per "/"
  local name=$1 part out="" first=1
  local IFS='/' -
  set -f
  for part in $name; do
    part=$(printf '%s' "$part" | sed -E 's/[][\.+*?(){}|^$]/\\&/g')
    if [ "$first" -eq 1 ]; then out="^$part\$"; first=0; else out="$out/^$part\$"; fi
  done
  printf '%s' "$out"
}
run_single() { # package name
  local pkg=$1 name=$2 pat out="$T/single.json"
  pat=$(quote_pattern "$name")
  if go test -tags test -race -count=1 -json -run "$pat" "./${pkg#"$mod"/}" > "$out" 2> "$out.err" \
     && jq -r --arg n "$name" --arg p "$pkg" 'select(.Action=="pass" and .Test==$n and .Package==$p) | .Test' "$out" | grep -qxF "$name" \
     && jq -r --arg p "$pkg" 'select(.Action=="pass" and .Test==null and .Package==$p) | .Package' "$out" | grep -qxF "$pkg"; then
    echo "select ok: $pkg $name"
    return 0
  fi
  echo "select FAILED: $pkg $name"
  return 1
}
if [ -n "${CHECK_TOPS:-}" ]; then
  tops=$CHECK_TOPS
else
  tops=""
  for f in cmd/yt2column/review_records_test.go cmd/yt2column/high_review_decision_test.go internal/mergepr/mergemain_test.go; do
    p="./$(dirname "$f")"
    case " $pkgs " in *" $p "*) ;; *) continue ;; esac
    tops="$tops $(sed -nE 's/^func (Test[A-Za-z0-9_]*)\(.*/\1/p' "$f")"
  done
fi
[ -n "$tops" ] || die "no top-level test function to select"
sel_failed=0
for top in $tops; do
  awk -F'\t' -v t="$top" '$2==t || index($2, t "/")==1' "$T/base.leaves.r" > "$T/top.leaves"
  if [ ! -s "$T/top.leaves" ]; then echo "select FAILED: $top has no leaf in BASE"; sel_failed=1; continue; fi
  for pos in 1 "$(wc -l < "$T/top.leaves" | tr -d ' ')"; do
    line=$(sed -n "${pos}p" "$T/top.leaves")
    run_single "${line%%$'\t'*}" "${line#*$'\t'}" || sel_failed=1
    [ "$pos" = 1 ] && [ "$(wc -l < "$T/top.leaves" | tr -d ' ')" = 1 ] && break
  done
done
check "single-first-last" "$sel_failed"

mid_failed=0
for name in "${middles[@]+"${middles[@]}"}"; do
  # The leaves that use one shared state span several top-level functions, so
  # "neither first nor last" is judged among all leaves of the package.
  pkg=$(awk -F'\t' -v n="$name" '$2==n {print $1; exit}' "$T/base.leaves.r")
  if [ -z "$pkg" ]; then echo "middle FAILED: $name is not in BASE"; mid_failed=1; continue; fi
  awk -F'\t' -v p="$pkg" '$1==p' "$T/base.leaves.r" > "$T/pkg.leaves"
  n=$(wc -l < "$T/pkg.leaves" | tr -d ' ')
  ln=$(awk -F'\t' -v n="$name" '$2==n {print NR; exit}' "$T/pkg.leaves")
  if [ "$ln" -le 1 ] || [ "$ln" -ge "$n" ]; then echo "middle FAILED: $name is the first or the last leaf of $pkg"; mid_failed=1; continue; fi
  run_single "$pkg" "$name" || mid_failed=1
done
[ "${#middles[@]}" -gt 0 ] || { echo "middle FAILED: no middle name given"; mid_failed=1; }
check "single-middle" "$mid_failed"

echo "== shuffle (AFTER) =="
shuf_failed=0
for r in 1 2; do
  if json_run "$T/shuffle$r.json" -shuffle=on "${full_args[@]+"${full_args[@]}"}"; then
    seeds=$(jq -r 'select(.Output != null and (.Output | test("^-test\\.shuffle [0-9]+"))) | "\(.Package) \(.Output | rtrimstr("\n"))"' "$T/shuffle$r.json" | sort -u)
    count=$(printf '%s\n' "$seeds" | grep -c . || true)
    np=$(echo $pkgs | wc -w | tr -d ' ')
    echo "shuffle run $r seeds:"; printf '%s\n' "$seeds"
    [ "$count" -eq "$np" ] || { echo "shuffle run $r: expected $np seed lines, read $count"; shuf_failed=1; }
  else
    cat "$T/shuffle$r.json.err"; echo "shuffle run $r: go test failed"; shuf_failed=1
  fi
done
check "shuffle" "$shuf_failed"

echo "== changed files (BASE..AFTER) =="
git -c core.quotePath=true diff --no-ext-diff --no-relative --no-renames -O/dev/null --name-only "$base" "$after" > "$T/files.txt"
cat "$T/files.txt"
tagged() { # first line of a file at the given commit is the test build tag
  [ "$(git show "$1:$2" 2>/dev/null | head -n 1)" = "//go:build test" ]
}
range_failed=0
snap_ok=""
while IFS= read -r f; do
  [ -n "$f" ] || continue
  case "$f" in *'"'*|*\\*) echo "range FAILED: quoted path $f"; range_failed=1; continue ;; esac
  rev=$after
  git cat-file -e "$after:$f" 2>/dev/null || rev=$base  # a deleted file is read at BASE
  ok=1
  case "$f" in
    cmd/yt2column/*|internal/mergepr/*)
      case "$f" in *_test.go) ;; *) tagged "$rev" "$f" || ok=0 ;; esac ;;
    internal/snapshottest/*)
      if [ -z "$snap_ok" ]; then
        snap_ok=yes
        while IFS= read -r g; do
          case "$g" in *_test.go) ;; *) tagged "$after" "$g" || snap_ok=no ;; esac
        done < <(git ls-tree -r --name-only "$after" -- internal/snapshottest)
      fi
      [ "$snap_ok" = yes ] || ok=0 ;;
    internal/pipeline/pipeline_test.go|docs/dev/conventions/package_reference.md) ;;
    docs/tasks/0127_git_fixture_test_speedup/*) ;;
    *) ok=0 ;;
  esac
  [ "$ok" -eq 1 ] || { echo "range FAILED: $f"; range_failed=1; }
done < "$T/files.txt"
[ -s "$T/files.txt" ] || { echo "range FAILED: no changed file"; range_failed=1; }
check "range" "$range_failed"

if [ "$failed" -eq 0 ]; then echo "compare: PASS"; else echo "compare: FAIL"; exit 1; fi
