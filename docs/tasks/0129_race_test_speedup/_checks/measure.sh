#!/usr/bin/env bash
# Temporary check for AC-13 and AC-14 of task 0129 (not committed as a test).
#
# Usage: MEASURE_OUT=<file outside the work tree> measure.sh <before> <after>
#
# Environment (all optional except MEASURE_OUT):
#   MEASURE_OUT    file the whole output is also written to (required; must be
#                  outside the work tree).
#   MEASURE_ROUNDS number of measured rounds (default 3).
#   MEASURE_TESTS  space-separated test names to measure alone (default: all
#                  eight); lets a breakage check run a short measurement.
#   MEASURE_FULL   1 (default) also measures the whole run; 0 skips it.
#
# The script checks out <before> and <after> alternately in the work tree, so
# it starts only on a clean work tree, copies itself out of the tree first (a
# checkout can remove this file), and on every exit path checks the original
# branch out again and verifies it. For each commit it first runs every
# command once to warm the build cache (not recorded). Per round and commit it
# records the load average, then times each test alone
# (`go test -tags test -race -count=1 -run '^T$' pkg`) and the whole run
# (`go test -tags test -race -count=1 ./...`).
# The AC-14 verdict: for each compared test, the median of "after" must be less
# than the minimum of "before". The linear-time test and
# TestParseSubtitlesArrayElementLimit are measured and recorded but not
# compared. The whole-run time is recorded only. Any failed run, unreadable
# time, missing precondition or failed restore makes the result FAIL.
set -u

if [ -z "${MEASURE_SELF_COPY:-}" ]; then
  # Re-run from a copy outside the work tree, so a checkout cannot pull the
  # script out from under bash.
  copy=$(mktemp "${TMPDIR:-/tmp}/measure.XXXXXX") || exit 2
  cp "$0" "$copy" || exit 2
  MEASURE_SELF_COPY=$copy exec bash "$copy" "$@"
fi
trap 'rm -f "$MEASURE_SELF_COPY"' EXIT

before=${1:?usage: MEASURE_OUT=<file> measure.sh <before> <after>}
after=${2:?usage: MEASURE_OUT=<file> measure.sh <before> <after>}
rounds=${MEASURE_ROUNDS:-3}
full=${MEASURE_FULL:-1}
out=${MEASURE_OUT:-}
[ -n "$out" ] || { echo "MEASURE_OUT is required"; exit 2; }

root=$(git rev-parse --show-toplevel) || { echo "not in a git work tree"; exit 2; }
cd "$root" || exit 2
case $(cd "$(dirname "$out")" 2>/dev/null && pwd)/ in
  "$root"/*) echo "MEASURE_OUT must be outside the work tree"; exit 2 ;;
esac
: >"$out" || exit 2
say() { printf '%s\n' "$*" | tee -a "$out"; }

before=$(git rev-parse --verify "$before^{commit}") || { say "unknown commit"; exit 2; }
after=$(git rev-parse --verify "$after^{commit}") || { say "unknown commit"; exit 2; }

fail_early() { say "RESULT: FAIL ($1)"; exit 2; }
[ -z "$(git status --porcelain)" ] || fail_early "the work tree is not clean"
command -v deadcode >/dev/null 2>&1 || fail_early "deadcode is not on PATH"
command -v perl >/dev/null 2>&1 || fail_early "perl is not on PATH"
orig_branch=$(git symbolic-ref --short -q HEAD) || fail_early "HEAD is not on a branch"
orig_commit=$(git rev-parse HEAD)

restored=0
restore() {
  [ "$restored" -eq 1 ] && return
  restored=1
  git checkout -q "$orig_branch" 2>/dev/null
  if [ "$(git symbolic-ref --short -q HEAD)" = "$orig_branch" ] && [ "$(git rev-parse HEAD)" = "$orig_commit" ] \
     && [ -z "$(git status --porcelain)" ]; then
    say "restored: on $orig_branch at $orig_commit"
  else
    say "RESTORE FAILED: check the branch and work tree before doing anything else"
    exit 3
  fi
}
trap 'restore; rm -f "$MEASURE_SELF_COPY"' EXIT
trap 'exit 130' INT TERM

now() { perl -MTime::HiRes=time -e 'printf "%.3f\n", time'; }

all_tests=(
  "TestParseCopiedLargeInputs ./internal/transcript"
  "TestRunAcceptsLargeOutput ./internal/coveragecompare"
  "TestMakeDeadcodeVerdict ./cmd/yt2column"
  "TestMakeDeadcodeScope ./cmd/yt2column"
  "TestParseSubtitlesLimits ./internal/ytdlpsource"
  "TestParseSubtitlesArrayElementLimit ./internal/ytdlpsource"
  "TestAsArrayElementLimit ./internal/strictjson"
  "TestNeutralizedDisplayMarkdownLinearTime ./internal/publisher"
)
# Not compared in the AC-14 verdict.
not_compared=" TestNeutralizedDisplayMarkdownLinearTime TestParseSubtitlesArrayElementLimit "
tests=()
for entry in "${all_tests[@]}"; do
  name=${entry%% *}
  if [ -z "${MEASURE_TESTS:-}" ] || [[ " $MEASURE_TESTS " == *" $name "* ]]; then tests+=("$entry"); fi
done
[ "${#tests[@]}" -gt 0 ] || fail_early "no test selected"

say "machine: $(sysctl -n machdep.cpu.brand_string 2>/dev/null), $(sysctl -n hw.ncpu 2>/dev/null) cores, $(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1073741824 )) GiB, $(sw_vers -productName 2>/dev/null) $(sw_vers -productVersion 2>/dev/null)"
say "power: $(pmset -g batt 2>/dev/null | head -n 1)"
say "go: $(go version)"
say "before: $before"
say "after:  $after"
say "single command: go test -tags test -race -count=1 -run '^<Test>\$' <package>"
say "whole command:  go test -tags test -race -count=1 ./...  (measured: $full)"
say "rounds: $rounds"

bad=0
# timed <label> <command...>: runs it, sets ELAPSED, returns its status.
timed() {
  local start end
  start=$(now)
  "$@" >"$out.run" 2>&1
  local rc=$?
  end=$(now)
  ELAPSED=$(awk -v a="$start" -v b="$end" 'BEGIN { printf "%.2f", b - a }')
  return $rc
}
run_single() { # name pkg
  timed go test -tags test -race -count=1 -v -run "^$1\$" "$2" || return 1
  grep -q "^--- PASS: $1 " "$out.run"
}
run_full() { timed go test -tags test -race -count=1 ./...; }

times_file=$(mktemp "${TMPDIR:-/tmp}/measure-times.XXXXXX") || exit 2
trap 'restore; rm -f "$MEASURE_SELF_COPY" "$times_file" "$out.run"' EXIT

checkout() { git checkout -q --detach "$1" || { say "checkout $1 failed"; bad=1; return 1; }; }

warm() {
  checkout "$1" || return
  say "warm-up (not recorded) at $1"
  for entry in "${tests[@]}"; do
    run_single ${entry} || { say "warm-up failed: $entry"; bad=1; }
  done
  if [ "$full" = 1 ]; then run_full || { say "warm-up of the whole run failed"; bad=1; }; fi
}
warm "$before"; warm "$after"

for round in $(seq 1 "$rounds"); do
  for which in before after; do
    commit=$before; [ "$which" = after ] && commit=$after
    [ "$bad" -eq 0 ] || break 2
    checkout "$commit" || break 2
    say "round $round $which $commit load: $(uptime | sed 's/.*load averages*: //')"
    for entry in "${tests[@]}"; do
      name=${entry%% *}
      if run_single ${entry}; then
        say "  $which $name ${ELAPSED}s"
        echo "$which $name $ELAPSED" >>"$times_file"
      else
        say "  $which $name FAILED (${ELAPSED}s); the output tail follows"; tail -n 15 "$out.run" | tee -a "$out"; bad=1
      fi
    done
    if [ "$full" = 1 ]; then
      if run_full; then say "  $which whole ${ELAPSED}s"; echo "$which WHOLE $ELAPSED" >>"$times_file"
      else say "  $which whole FAILED (${ELAPSED}s)"; tail -n 15 "$out.run" | tee -a "$out"; bad=1; fi
    fi
  done
done

restore
# The checkout is back; restore() exits 3 itself when it failed.

if [ "$bad" -ne 0 ]; then say "RESULT: FAIL (a run failed)"; exit 1; fi

verdict=0
say "AC-14 verdict: median(after) < min(before) per compared test"
for entry in "${tests[@]}"; do
  name=${entry%% *}
  if [[ "$not_compared" == *" $name "* ]]; then say "  $name: recorded, not compared"; continue; fi
  line=$(awk -v n="$name" '
    $2 == n { v[$1] = v[$1] " " $3 }
    END {
      split(v["before"], b, " "); split(v["after"], a, " ")
      nb = length(b); na = length(a)
      if (nb == 0 || na == 0) { print "unreadable"; exit }
      min = b[1]; for (i = 2; i <= nb; i++) if (b[i] + 0 < min + 0) min = b[i]
      for (i = 1; i <= na; i++) s[i] = a[i] + 0
      for (i = 1; i <= na; i++) for (j = i + 1; j <= na; j++) if (s[j] < s[i]) { t = s[i]; s[i] = s[j]; s[j] = t }
      med = (na % 2) ? s[(na + 1) / 2] : (s[na / 2] + s[na / 2 + 1]) / 2
      printf "%s %s %d\n", min, med, (med + 0 < min + 0) ? 1 : 0
    }' "$times_file")
  if [ "$line" = unreadable ] || [ -z "$line" ]; then say "  $name: no times, FAIL"; verdict=1; continue; fi
  set -- $line
  if [ "$3" = 1 ]; then say "  $name: before min $1s, after median ${2}s: PASS"
  else say "  $name: before min $1s, after median ${2}s: FAIL (not shorter)"; verdict=1; fi
done
if [ "$full" = 1 ]; then
  for which in before after; do
    say "whole run $which: $(awk -v w=$which '$1 == w && $2 == "WHOLE" { printf "%ss ", $3 }' "$times_file")"
  done
fi
if [ "$verdict" -eq 0 ]; then say "RESULT: PASS"; exit 0; else say "RESULT: FAIL (AC-14)"; exit 1; fi
