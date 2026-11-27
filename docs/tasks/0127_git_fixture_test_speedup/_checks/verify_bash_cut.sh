#!/usr/bin/env bash
# Temporary script of task 0127 (PR-3): verify that reducing bash script tests
# preserves per-script executed line sets in each bash, exercises every mode
# and fail() branch of review-records.sh, detects mutations against kept tests,
# and matches path-bash leaves when the bash 3.2 opt-in env var is enabled.
#
# Usage: verify_bash_cut.sh BASE AFTER
# Narrowing (for the breakage check only; a narrowed run is not a verification):
#   CHECK_ONLY=rr-lines,hrd-lines,mutations,optin-all  checks to run (default: all)
#   CHECK_RR_RUN=<regexp>    -run for review-records.sh tests
#   CHECK_HRD_RUN=<regexp>   -run for high-review-decision.sh tests
#   CHECK_OPTIN_VAR=<name>   opt-in env var for all bin-bash-3 leaves
set -euo pipefail

# Run from a copy outside the work tree: checking out BASE removes this file.
if [ -z "${CHECK_COPY:-}" ]; then
  tmp=$(mktemp -d)
  cp "$0" "$tmp/verify_bash_cut.sh"
  CHECK_COPY=1 CHECK_TMP=$tmp exec bash "$tmp/verify_bash_cut.sh" "$@"
fi

die() { echo "verify_bash_cut: STOP: $*" >&2; exit 1; }
failed=0
check() { # name status(0=pass) detail
  if [ "$2" -eq 0 ]; then echo "check $1: PASS"; else echo "check $1: FAIL${3:+ ($3)}"; failed=1; fi
}

[ "$#" -eq 2 ] || die "usage: verify_bash_cut.sh BASE AFTER"
cd "$(git rev-parse --show-toplevel)"
base=$(git rev-parse --verify "$1^{commit}") || die "unknown BASE"
after=$(git rev-parse --verify "$2^{commit}") || die "unknown AFTER"
T=$CHECK_TMP
[ -z "$(git status --porcelain)" ] || die "the work tree is not clean"

rr_script=".claude/commands/_lib/review-records.sh"
hrd_script=".claude/commands/_lib/high-review-decision.sh"
rr_run=${CHECK_RR_RUN:-'^(TestReviewRecordsCoverage|TestReviewRecordsUnreviewed|TestReviewRecordsCrossCheck|TestReviewRecordsFableAlias|TestReviewRecordsMode)$'}
hrd_run=${CHECK_HRD_RUN:-'^TestHighReviewDecision$'}
optin_var=${CHECK_OPTIN_VAR:-YT2COLUMN_TEST_BASH3_ALL}
only=${CHECK_ONLY:-rr-lines,hrd-lines,mutations,optin-all}
want_check() { case ",$only," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }

orig_sha=$(git rev-parse HEAD)
orig_branch=$(git symbolic-ref --quiet --short HEAD || true)
restore() {
  status=$?
  trap - EXIT
  # One call per path: a pathspec missing at the current HEAD would make git restore neither.
  git checkout -- "$rr_script" 2>/dev/null || true
  git checkout -- "$hrd_script" 2>/dev/null || true
  if [ -n "$orig_branch" ]; then
    git switch --quiet "$orig_branch" || status=1
  else
    git switch --quiet --detach "$orig_sha" || status=1
  fi
  [ "$(git symbolic-ref --quiet --short HEAD || true)" = "$orig_branch" ] || { echo "verify_bash_cut: STOP: not on the original branch" >&2; status=1; }
  [ "$(git rev-parse HEAD)" = "$orig_sha" ] || { echo "verify_bash_cut: STOP: not back at $orig_sha" >&2; status=1; }
  [ -z "$(git status --porcelain)" ] || { echo "verify_bash_cut: STOP: the work tree is not clean after the run" >&2; status=1; }
  rm -rf "$T"
  exit "$status"
}
trap restore EXIT

path_major=$(bash -c 'echo ${BASH_VERSINFO[0]}')
bin_major=$(/bin/bash -c 'echo ${BASH_VERSINFO[0]}')
[ -n "$path_major" ] && [ -n "$bin_major" ] || die "cannot read bash versions"

echo "BASE: $base"
echo "AFTER: $after"
echo "path-bash major: $path_major, /bin/bash major: $bin_major"

# instrument_script replaces the single marker line in-place (preserving every
# line number) with set -T and a DEBUG trap appending "<bash_major>\t<mode>\t<lineno>"
# to trace_file without touching stdout or stderr.
instrument_script() { # script marker trace_file mode_expr
  local script=$1 marker=$2 trace=$3 mode_expr=$4
  local count
  count=$(grep -cxF "$marker" "$script" || true)
  [ "$count" -eq 1 ] || die "$script: expected 1 line matching '$marker', got $count"
  : > "$trace"
  awk -v m="$marker" -v tr="$trace" -v me="$mode_expr" '
    $0 == m {
      printf "%s; _VBC_MODE=%s; set -T; trap '\''builtin printf \"%%s\\t%%s\\t%%s\\n\" \"${BASH_VERSINFO[0]}\" \"$_VBC_MODE\" \"$LINENO\" >> \"%s\"'\'' DEBUG\n", m, me, tr
      next
    }
    { print }
  ' "$script" > "$T/inst.tmp"
  mv "$T/inst.tmp" "$script"
}

run_traced() { # sha script marker trace mode_expr run_re
  local sha=$1 script=$2 marker=$3 trace=$4 mode_expr=$5 run_re=$6
  git switch --quiet --detach "$sha"
  instrument_script "$script" "$marker" "$trace" "$mode_expr"
  local rc=0
  env -u "$optin_var" go test -tags test -count=1 -run "$run_re" ./cmd/yt2column > "$trace.out" 2>&1 || rc=$?
  git checkout -- "$script"
  if [ "$rc" -ne 0 ]; then
    tail -n 40 "$trace.out" >&2
    die "go test failed for $script at $sha"
  fi
  [ -s "$trace" ] || die "empty trace for $script at $sha"
}

extract_lines() { # trace major out
  awk -F'\t' -v v="$2" '$1==v {print $3}' "$1" | sort -nu > "$3"
}

if want_check rr-lines; then
  echo "== review-records.sh line coverage =="
  run_traced "$base" "$rr_script" "unset REVIEW_RECORDS_CLEAN_ENV" "$T/rr-base.trace" '${1:-none}' "$rr_run"
  run_traced "$after" "$rr_script" "unset REVIEW_RECORDS_CLEAN_ENV" "$T/rr-after.trace" '${1:-none}' "$rr_run"

  git switch --quiet --detach "$after"
  part_line=$(grep -n "printf 'PARTIAL\\\\n'" "$rr_script" | cut -d: -f1 || true)
  miss_line=$(grep -n "printf 'MISSING\\\\n'" "$rr_script" | cut -d: -f1 || true)
  [ "$(printf '%s\n' "$part_line" | grep -c .)" -eq 1 ] || die "cannot locate PARTIAL line in $rr_script"
  [ "$(printf '%s\n' "$miss_line" | grep -c .)" -eq 1 ] || die "cannot locate MISSING line in $rr_script"

  check_rr_bash() { # label major
    local label=$1 major=$2
    extract_lines "$T/rr-base.trace" "$major" "$T/rr-base-$label.lines"
    extract_lines "$T/rr-after.trace" "$major" "$T/rr-after-$label.lines"
    local bc ac
    bc=$(wc -l < "$T/rr-base-$label.lines" | tr -d ' ')
    ac=$(wc -l < "$T/rr-after-$label.lines" | tr -d ' ')
    echo "$label lines: BASE=$bc AFTER=$ac"
    if [ "$bc" -eq 0 ] || [ "$ac" -eq 0 ]; then
      check "rr-lines-$label" 1 "no lines recorded"
      return
    fi
    if diff -u "$T/rr-base-$label.lines" "$T/rr-after-$label.lines" > "$T/rr-$label.diff"; then
      check "rr-lines-$label" 0
    else
      check "rr-lines-$label" 1 "line sets differ"
      head -n 40 "$T/rr-$label.diff"
    fi

    # Verify all modes and both fail() branches were exercised on AFTER.
    awk -F'\t' -v v="$major" '$1==v {print $2}' "$T/rr-after.trace" | sort -u > "$T/rr-after-$label.modes"
    local missing_modes=""
    for m in coverage unreviewed cross-check fable; do
      grep -qxF "$m" "$T/rr-after-$label.modes" || missing_modes="$missing_modes $m"
    done
    local fail_ok=0
    grep -qxF "$part_line" "$T/rr-after-$label.lines" && grep -qxF "$miss_line" "$T/rr-after-$label.lines" || fail_ok=1
    if [ -z "$missing_modes" ] && [ "$fail_ok" -eq 0 ]; then
      check "rr-modes-$label" 0
    else
      check "rr-modes-$label" 1 "missing modes:${missing_modes:- none}, fail_branches_missing=$fail_ok"
    fi
  }

  check_rr_bash path-bash "$path_major"
  if [ "$bin_major" = 3 ]; then
    check_rr_bash bin-bash-3 "$bin_major"
  else
    echo "bin-bash-3: not applicable (/bin/bash is version $bin_major)"
  fi
fi

if want_check hrd-lines; then
  echo "== high-review-decision.sh line coverage =="
  run_traced "$base" "$hrd_script" "unset HIGH_REVIEW_DECISION_CLEAN_ENV" "$T/hrd-base.trace" "hrd" "$hrd_run"
  run_traced "$after" "$hrd_script" "unset HIGH_REVIEW_DECISION_CLEAN_ENV" "$T/hrd-after.trace" "hrd" "$hrd_run"
  extract_lines "$T/hrd-base.trace" "$path_major" "$T/hrd-base.lines"
  extract_lines "$T/hrd-after.trace" "$path_major" "$T/hrd-after.lines"
  bc=$(wc -l < "$T/hrd-base.lines" | tr -d ' ')
  ac=$(wc -l < "$T/hrd-after.lines" | tr -d ' ')
  echo "high-review-decision lines: BASE=$bc AFTER=$ac"
  if [ "$bc" -eq 0 ] || [ "$ac" -eq 0 ]; then
    check "hrd-lines" 1 "no lines recorded"
  elif diff -u "$T/hrd-base.lines" "$T/hrd-after.lines" > "$T/hrd.diff"; then
    check "hrd-lines" 0
  else
    check "hrd-lines" 1 "line sets differ"
    head -n 40 "$T/hrd.diff"
  fi
fi

if want_check mutations; then
  echo "== mutations (AFTER) =="
  git switch --quiet --detach "$after"
  run_mutation() { # name awk_prog test_re
    local name=$1 prog=$2 test_re=$3 log="$T/mut-$1.out"
    cp "$rr_script" "$T/rr.orig"
    awk "$prog" "$T/rr.orig" > "$rr_script"
    if cmp -s "$T/rr.orig" "$rr_script"; then
      git checkout -- "$rr_script"
      check "mut-$name" 1 "mutation did not change $rr_script"
      return
    fi
    local rc=0
    go test -tags test -count=1 -v -run "$test_re" ./cmd/yt2column > "$log" 2>&1 || rc=$?
    git checkout -- "$rr_script"
    if [ "$rc" -ne 0 ] && grep -q -- '--- FAIL:' "$log"; then
      check "mut-$name" 0
    else
      check "mut-$name" 1 "expected test failure (rc=$rc)"
      tail -n 20 "$log"
    fi
  }
  run_mutation drop-log-encoding \
    '$0 ~ /-c i18n\.logOutputEncoding=UTF-8/ { next } { print }' \
    '^TestReviewRecordsCoverage$/^path-bash$/^git_config$/^repository$/^i18n\.logOutputEncoding$'
  run_mutation break-widen \
    '{ gsub(/\[ "\$widen" = widen \] \|\| return 1/, "return 1"); print }' \
    '^TestReviewRecordsUnreviewed$/^path-bash$'
  run_mutation drop-fable \
    '$0 ~ /^[[:space:]]*cross-check \| fable\)$/ { sub(/ \| fable/, ""); } { print }' \
    '^TestReviewRecordsFableAlias$/^path-bash$'
fi

if want_check optin-all; then
  echo "== bash 3.2 opt-in all leaves (AFTER) =="
  if [ "$bin_major" = 3 ]; then
    git switch --quiet --detach "$after"
    leaves_from() { # json_file bash_name out
      jq -r 'select(.Action=="pass" and .Test!=null) | .Test' "$1" | LC_ALL=C sort -u > "$1.pass"
      awk '{ k[NR]=$0; s=$0; while (match(s, /\/[^\/]*$/)) { s=substr(s,1,RSTART-1); par[s]=1 } }
        END { for (i=1;i<=NR;i++) if (!(k[i] in par)) print k[i] }' "$1.pass" |
        awk -v b="/$2/" 'index($0, b) > 0' | LC_ALL=C sort -u > "$3"
    }
    env -u "$optin_var" go test -tags test -count=1 -json -run "$rr_run" ./cmd/yt2column > "$T/def.json" 2> "$T/def.err" || die "default run failed at AFTER"
    env "$optin_var=1" go test -tags test -count=1 -json -run "$rr_run" ./cmd/yt2column > "$T/optin.json" 2> "$T/optin.err" || die "opt-in run failed at AFTER"
    leaves_from "$T/def.json" "path-bash" "$T/def-path.leaves"
    leaves_from "$T/def.json" "bin-bash-3" "$T/def-bin3.leaves"
    leaves_from "$T/optin.json" "path-bash" "$T/optin-path.leaves"
    leaves_from "$T/optin.json" "bin-bash-3" "$T/optin-bin3.leaves"
    def_p=$(wc -l < "$T/def-path.leaves" | tr -d ' ')
    def_3=$(wc -l < "$T/def-bin3.leaves" | tr -d ' ')
    opt_p=$(wc -l < "$T/optin-path.leaves" | tr -d ' ')
    opt_3=$(wc -l < "$T/optin-bin3.leaves" | tr -d ' ')
    echo "default leaves: path-bash=$def_p, bin-bash-3=$def_3; opt-in leaves: path-bash=$opt_p, bin-bash-3=$opt_3"
    sed 's#/bin-bash-3/#/path-bash/#g' "$T/optin-bin3.leaves" > "$T/optin-bin3.norm"
    if [ "$opt_p" -gt 0 ] && [ "$def_3" -gt 0 ] && [ "$def_3" -lt "$def_p" ] &&
       diff -u "$T/optin-path.leaves" "$T/optin-bin3.norm" > "$T/optin.diff"; then
      check "optin-all" 0
    else
      check "optin-all" 1 "default or opt-in leaf sets do not match expectations"
      head -n 40 "$T/optin.diff" 2>/dev/null || true
    fi
  else
    echo "optin-all: not applicable (/bin/bash is version $bin_major)"
  fi
fi

if [ "$failed" -eq 0 ]; then echo "verify_bash_cut: PASS"; else echo "verify_bash_cut: FAIL"; exit 1; fi
