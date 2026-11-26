#!/usr/bin/env bash
set -euo pipefail

# Temporary verification script for task 0126 (AC-02, AC-09, AC-12, AC-13, AC-15).
# Modes:
#   estimate          - Run origin/main once per package and estimate parallel lower bound vs 50%.
#   race              - Run -race -count=3 and -race -count=1 -cpu 1,4 in current tree.
#   names <base>      - Compare (Package, Test) pass/skip sets between <base> and current tree.
#   measure <before>  - Measure 3 runs per package in <before> and current tree; compare medians.
#   unchanged <base>  - Verify fixprguard_guide.md and const Protocol are unchanged from <base>.

PKG_INTERNAL="github.com/isseis/yt2column/internal/fixprguard"
PKG_CMD="github.com/isseis/yt2column/cmd/fixprguard"
GUIDE_PATH="docs/dev/guides/fixprguard_guide.md"
TYPES_PATH="internal/fixprguard/types.go"

TMP_DIRS=()
cleanup() {
  local d
  for d in "${TMP_DIRS[@]+"${TMP_DIRS[@]}"}"; do
    rm -rf "$d"
  done
}
trap cleanup EXIT

die() {
  echo "gotest.sh: ERROR: $*" >&2
  exit 1
}

# make_tmp_dir creates a temporary directory in the current shell, registers it
# for EXIT cleanup, and stores its path in the variable named by $1.
make_tmp_dir() {
  local varname="$1"
  local d
  d=$(mktemp -d) || die "mktemp -d failed"
  TMP_DIRS+=("$d")
  printf -v "$varname" '%s' "$d"
}

resolve_commit() {
  local ref="$1"
  git rev-parse --verify "${ref}^{commit}" 2>/dev/null || die "cannot resolve commit: $ref"
}

head_label() {
  local head_commit="$1"
  if [[ -n "$(git status --porcelain)" ]]; then
    printf '%s+dirty\n' "$head_commit"
  else
    printf '%s\n' "$head_commit"
  fi
}

# extract_tree extracts <commit> into a newly created temp dir registered in
# TMP_DIRS and stores the directory path in the variable named by $2.
extract_tree() {
  local commit="$1"
  local varname="$2"
  local dir
  make_tmp_dir dir
  git archive "$commit" | tar -x -C "$dir" || die "failed to extract archive for $commit"
  printf -v "$varname" '%s' "$dir"
}

detect_cores() {
  local n=""
  if command -v nproc >/dev/null 2>&1; then
    n=$(nproc)
  elif command -v sysctl >/dev/null 2>&1; then
    n=$(sysctl -n hw.ncpu)
  fi
  [[ "$n" =~ ^[1-9][0-9]*$ ]] || die "cannot detect CPU core count"
  printf '%s\n' "$n"
}

detect_gomaxprocs() {
  if [[ -n "${GOMAXPROCS:-}" ]]; then
    [[ "$GOMAXPROCS" =~ ^[1-9][0-9]*$ ]] || die "invalid GOMAXPROCS: $GOMAXPROCS"
    printf '%s\n' "$GOMAXPROCS"
    return
  fi
  detect_cores
}

# validate_json_run checks that every line is valid JSON, no Action=="fail"
# exists, and every expected package has a package-level pass event with a
# numeric Elapsed.
validate_json_run() {
  local file="$1"
  shift
  local pkgs_json
  pkgs_json=$(printf '%s\n' "$@" | jq -R . | jq -s .)
  jq -e -s --argjson pkgs "$pkgs_json" '
    . as $events |
    length > 0
    and ([$events[] | select(.Action == "fail")] | length == 0)
    and (
      $pkgs | all(. as $p |
        any($events[]; .Package == $p and .Action == "pass" and (has("Test") | not) and (.Elapsed | type == "number"))
      )
    )
  ' "$file" >/dev/null || die "go test -json validation failed for $file"
}

run_json_test() {
  local workdir="$1"
  local outfile="$2"
  shift 2
  (
    cd "$workdir" && go test "$@"
  ) >"$outfile" || die "go test failed in $workdir with args: $*"
}

estimate_pkg() {
  local tree="$1"
  local rel_pkg="$2"
  local full_pkg="$3"
  local serial_json="$4"
  local procs="$5"
  local worktmp
  make_tmp_dir worktmp
  local json_file="$worktmp/out.json"

  run_json_test "$tree" "$json_file" -tags test -race -count=1 -timeout 0 -json "$rel_pkg"
  validate_json_run "$json_file" "$full_pkg"

  local report
  report=$(jq -r -s \
    --arg pkg "$full_pkg" \
    --argjson serial "$serial_json" \
    --argjson procs "$procs" '
    ([.[] | select(.Package == $pkg and .Action == "pass" and (has("Test") | not))] | last) as $pkg_ev |
    [.[] | select(.Package == $pkg and .Action == "pass" and has("Test") and (.Test | contains("/") | not))] as $tops |
    if ($pkg_ev == null) or (($pkg_ev.Elapsed // 0) <= 0) then
      error("missing or non-positive package elapsed")
    elif ($tops | length == 0) then
      error("no top-level pass tests")
    else . end |
    ([$tops[].Test]) as $top_names |
    if ($serial | all(. as $s | $top_names | index($s) != null) | not) then
      error("missing expected serial test")
    else . end |
    [$tops[] | select(.Test as $t | ($serial | index($t)) == null)] as $par_tops |
    if ($par_tops | length == 0) then
      error("no parallelizable top-level tests")
    else . end |
    ($par_tops | max_by(.Elapsed)) as $longest |
    ([$tops[] | select(.Test as $t | ($serial | index($t)) != null) | .Elapsed] | add // 0) as $serial_sum |
    ([$tops[].Elapsed] | add) as $tests_sum |
    ($longest.Elapsed + $serial_sum) as $bound_serial |
    ($tests_sum / $procs) as $bound_cpu |
    (if $bound_serial > $bound_cpu then $bound_serial else $bound_cpu end) as $lower |
    ($pkg_ev.Elapsed * 0.5) as $target |
    (if $lower <= $target then "PASS" else "FAIL" end) as $status |
    "package=\($pkg)",
    "  pkg_elapsed=\($pkg_ev.Elapsed)s",
    "  target_50pct=\($target)s",
    "  longest_parallel=\($longest.Test) (\($longest.Elapsed)s)",
    "  serial_sum=\($serial_sum)s",
    "  bound_serial=\($bound_serial)s",
    "  tests_sum=\($tests_sum)s",
    "  bound_cpu=\($bound_cpu)s (GOMAXPROCS=\($procs))",
    "  lower_bound=\($lower)s",
    "  status=\($status)"
  ' "$json_file") || die "estimate calculation failed for $full_pkg"

  printf '%s\n' "$report"
  if grep -q '^  status=FAIL$' <<<"$report"; then
    return 1
  fi
}

cmd_estimate() {
  [[ $# -eq 0 ]] || die "estimate takes no arguments"
  local commit procs tree status=0
  commit=$(resolve_commit "origin/main")
  procs=$(detect_gomaxprocs)
  extract_tree "$commit" tree

  echo "mode=estimate"
  echo "commit=$commit"
  echo "os=$(uname -srm)"
  echo "cores=$(detect_cores) gomaxprocs=$procs"
  echo "go=$(go version)"

  estimate_pkg "$tree" "./internal/fixprguard" "$PKG_INTERNAL" \
    '["TestGitEnvKeepsGitOnTheWorktree"]' "$procs" || status=1
  estimate_pkg "$tree" "./cmd/fixprguard" "$PKG_CMD" \
    '["TestGhTargetEnvRejected","TestGhRunsWithoutGitVariables","TestListRunnerRejectsStderr"]' "$procs" || status=1

  return "$status"
}

cmd_race() {
  [[ $# -eq 0 ]] || die "race takes no arguments"
  local head worktmp j1 j2
  head=$(head_label "$(resolve_commit "HEAD")")
  make_tmp_dir worktmp
  j1="$worktmp/count3.json"
  j2="$worktmp/cpu14.json"

  echo "mode=race"
  echo "head=$head"

  run_json_test "." "$j1" -tags test -race -count=3 -json ./internal/fixprguard ./cmd/fixprguard
  validate_json_run "$j1" "$PKG_INTERNAL" "$PKG_CMD"
  jq -e -s '[.[] | select(has("Test") and .Action == "pass")] | length > 0' "$j1" >/dev/null || die "no test pass events in count=3 run"
  echo "count3=PASS"

  run_json_test "." "$j2" -tags test -race -count=1 -cpu 1,4 -json ./internal/fixprguard ./cmd/fixprguard
  validate_json_run "$j2" "$PKG_INTERNAL" "$PKG_CMD"
  jq -e -s '[.[] | select(has("Test") and .Action == "pass")] | length > 0' "$j2" >/dev/null || die "no test pass events in cpu=1,4 run"
  echo "cpu14=PASS"
}

cmd_names() {
  [[ $# -eq 1 ]] || die "usage: gotest.sh names <base>"
  local base_commit head_commit head_out base_tree worktmp base_json head_json
  base_commit=$(resolve_commit "$1")
  head_commit=$(resolve_commit "HEAD")
  head_out=$(head_label "$head_commit")
  extract_tree "$base_commit" base_tree
  make_tmp_dir worktmp
  base_json="$worktmp/base.json"
  head_json="$worktmp/head.json"

  run_json_test "$base_tree" "$base_json" -tags test -race -count=1 -timeout 0 -json ./internal/fixprguard ./cmd/fixprguard
  validate_json_run "$base_json" "$PKG_INTERNAL" "$PKG_CMD"

  run_json_test "." "$head_json" -tags test -race -count=1 -timeout 0 -json ./internal/fixprguard ./cmd/fixprguard
  validate_json_run "$head_json" "$PKG_INTERNAL" "$PKG_CMD"

  jq -r -s '
    [.[] | select(has("Test") and .Action == "pass") | "\(.Package)::\(.Test)"] | unique | .[]
  ' "$base_json" | LC_ALL=C sort >"$worktmp/base_pass.txt"

  jq -r -s '
    [.[] | select(has("Test") and .Action == "pass") | "\(.Package)::\(.Test)"] | unique | .[]
  ' "$head_json" | LC_ALL=C sort >"$worktmp/head_pass.txt"

  jq -r -s '
    [.[] | select(has("Test") and .Action == "skip") | "\(.Package)::\(.Test)"] | unique | .[]
  ' "$head_json" | LC_ALL=C sort >"$worktmp/head_skip.txt"

  local base_count head_count
  base_count=$(wc -l <"$worktmp/base_pass.txt" | tr -d ' ')
  head_count=$(wc -l <"$worktmp/head_pass.txt" | tr -d ' ')
  [[ "$base_count" -gt 0 ]] || die "base pass test set is empty"

  LC_ALL=C comm -23 "$worktmp/base_pass.txt" "$worktmp/head_pass.txt" >"$worktmp/missing.txt"
  LC_ALL=C comm -12 "$worktmp/base_pass.txt" "$worktmp/head_skip.txt" >"$worktmp/skipped.txt"
  LC_ALL=C comm -13 "$worktmp/base_pass.txt" "$worktmp/head_pass.txt" >"$worktmp/added.txt"

  local missing_count skipped_count added_count
  missing_count=$(wc -l <"$worktmp/missing.txt" | tr -d ' ')
  skipped_count=$(wc -l <"$worktmp/skipped.txt" | tr -d ' ')
  added_count=$(wc -l <"$worktmp/added.txt" | tr -d ' ')

  echo "mode=names"
  echo "base=$base_commit"
  echo "head=$head_out"
  echo "base_pass=$base_count"
  echo "head_pass=$head_count"
  echo "missing=$missing_count"
  if [[ "$missing_count" -gt 0 ]]; then
    sed 's/^/  MISSING: /' "$worktmp/missing.txt"
  fi
  echo "became_skip=$skipped_count"
  if [[ "$skipped_count" -gt 0 ]]; then
    sed 's/^/  SKIPPED: /' "$worktmp/skipped.txt"
  fi
  echo "added=$added_count"
  if [[ "$added_count" -gt 0 ]]; then
    sed 's/^/  ADDED: /' "$worktmp/added.txt"
  fi

  if [[ "$missing_count" -gt 0 || "$skipped_count" -gt 0 ]]; then
    echo "status=FAIL"
    return 1
  fi
  echo "status=PASS"
}

measure_pkg() {
  local before_tree="$1"
  local rel_pkg="$2"
  local full_pkg="$3"
  local worktmp
  make_tmp_dir worktmp
  local before_vals=()
  local after_vals=()
  local i jf elapsed

  for i in 1 2 3; do
    jf="$worktmp/before-$i.json"
    run_json_test "$before_tree" "$jf" -tags test -race -count=1 -timeout 0 -json "$rel_pkg"
    validate_json_run "$jf" "$full_pkg"
    elapsed=$(jq -r -s --arg pkg "$full_pkg" '
      [.[] | select(.Package == $pkg and .Action == "pass" and (has("Test") | not))] | last | .Elapsed
    ' "$jf")
    before_vals+=("$elapsed")
  done

  for i in 1 2 3; do
    jf="$worktmp/after-$i.json"
    run_json_test "." "$jf" -tags test -race -count=1 -timeout 0 -json "$rel_pkg"
    validate_json_run "$jf" "$full_pkg"
    elapsed=$(jq -r -s --arg pkg "$full_pkg" '
      [.[] | select(.Package == $pkg and .Action == "pass" and (has("Test") | not))] | last | .Elapsed
    ' "$jf")
    after_vals+=("$elapsed")
  done

  local report
  report=$(jq -r -n \
    --arg pkg "$full_pkg" \
    --argjson before "[${before_vals[0]},${before_vals[1]},${before_vals[2]}]" \
    --argjson after "[${after_vals[0]},${after_vals[1]},${after_vals[2]}]" '
    ($before | sort | .[1]) as $bmed |
    ($after | sort | .[1]) as $amed |
    if $bmed <= 0 then error("non-positive before median") else . end |
    (($amed / $bmed) * 100) as $pct |
    (if $amed <= ($bmed * 0.5) then "PASS" else "FAIL" end) as $status |
    "package=\($pkg)",
    "  before_runs=\($before | map("\(.)s") | join(", "))",
    "  before_median=\($bmed)s",
    "  after_runs=\($after | map("\(.)s") | join(", "))",
    "  after_median=\($amed)s",
    "  ratio_pct=\(($pct * 100 | round) / 100)%",
    "  status=\($status)"
  ') || die "measure calculation failed for $full_pkg"

  printf '%s\n' "$report"
  if grep -q '^  status=FAIL$' <<<"$report"; then
    return 1
  fi
}

cmd_measure() {
  [[ $# -eq 1 ]] || die "usage: gotest.sh measure <before>"
  local before_commit head_commit head_out before_tree status=0
  before_commit=$(resolve_commit "$1")
  head_commit=$(resolve_commit "HEAD")
  head_out=$(head_label "$head_commit")
  extract_tree "$before_commit" before_tree

  echo "mode=measure"
  echo "before=$before_commit"
  echo "head=$head_out"
  echo "os=$(uname -srm)"
  echo "cores=$(detect_cores) gomaxprocs=$(detect_gomaxprocs)"
  echo "go=$(go version)"

  measure_pkg "$before_tree" "./internal/fixprguard" "$PKG_INTERNAL" || status=1
  measure_pkg "$before_tree" "./cmd/fixprguard" "$PKG_CMD" || status=1

  return "$status"
}

extract_protocol_line() {
  local src="$1"
  local matches
  matches=$(grep -E '^const Protocol = [0-9]+$' <<<"$src" || true)
  local count
  count=$(printf '%s\n' "$matches" | grep -c . || true)
  [[ "$count" -eq 1 ]] || die "expected exactly 1 const Protocol line, got $count"
  printf '%s\n' "$matches"
}

cmd_unchanged() {
  [[ $# -eq 1 ]] || die "usage: gotest.sh unchanged <base>"
  local base_commit head_commit head_out
  base_commit=$(resolve_commit "$1")
  head_commit=$(resolve_commit "HEAD")
  head_out=$(head_label "$head_commit")

  git cat-file -e "${base_commit}:${GUIDE_PATH}" 2>/dev/null || die "missing $GUIDE_PATH at $base_commit"
  git cat-file -e "${head_commit}:${GUIDE_PATH}" 2>/dev/null || die "missing $GUIDE_PATH at $head_commit"
  [[ -f "$GUIDE_PATH" ]] || die "missing $GUIDE_PATH in working tree"
  git cat-file -e "${base_commit}:${TYPES_PATH}" 2>/dev/null || die "missing $TYPES_PATH at $base_commit"
  git cat-file -e "${head_commit}:${TYPES_PATH}" 2>/dev/null || die "missing $TYPES_PATH at $head_commit"
  [[ -f "$TYPES_PATH" ]] || die "missing $TYPES_PATH in working tree"

  local status=0
  local guide_status="PASS"
  if ! git diff --quiet "$base_commit" "$head_commit" -- "$GUIDE_PATH" || ! git diff --quiet "$base_commit" -- "$GUIDE_PATH"; then
    guide_status="FAIL"
    status=1
  fi

  local base_proto head_proto wt_proto proto_status="PASS"
  base_proto=$(extract_protocol_line "$(git show "${base_commit}:${TYPES_PATH}")")
  head_proto=$(extract_protocol_line "$(git show "${head_commit}:${TYPES_PATH}")")
  wt_proto=$(extract_protocol_line "$(cat "$TYPES_PATH")")
  if [[ "$base_proto" != "$head_proto" || "$base_proto" != "$wt_proto" ]]; then
    proto_status="FAIL"
    status=1
  fi

  echo "mode=unchanged"
  echo "base=$base_commit"
  echo "head=$head_out"
  echo "guide=$guide_status ($GUIDE_PATH)"
  echo "protocol=$proto_status (base: $base_proto | head: $head_proto | worktree: $wt_proto)"
  if [[ "$status" -ne 0 ]]; then
    echo "status=FAIL"
    return 1
  fi
  echo "status=PASS"
}

main() {
  [[ $# -ge 1 ]] || die "usage: gotest.sh <estimate|race|names|measure|unchanged> [args...]"
  local mode="$1"
  shift
  case "$mode" in
    estimate) cmd_estimate "$@" ;;
    race) cmd_race "$@" ;;
    names) cmd_names "$@" ;;
    measure) cmd_measure "$@" ;;
    unchanged) cmd_unchanged "$@" ;;
    *) die "unknown mode: $mode" ;;
  esac
}

main "$@"
