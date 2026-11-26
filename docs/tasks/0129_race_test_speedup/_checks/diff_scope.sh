#!/usr/bin/env bash
# Temporary check for AC-02, AC-04, AC-05, AC-06, AC-07, AC-08 and AC-10 of
# task 0129 (not committed as a test).
#
# Usage: diff_scope.sh      (run after `git fetch origin main`)
#
# Compares the merge-base of origin/main and HEAD ("before") with HEAD
# ("after"). Prints one PASS or FAIL line per check, and exits 0 only when
# every check passed. A check that cannot be evaluated is a FAIL.
set -u
export LC_ALL=C

before=$(git merge-base origin/main HEAD) || { echo "FAIL: no merge-base"; exit 2; }
after=$(git rev-parse --verify HEAD) || { echo "FAIL: no HEAD"; exit 2; }
echo "before (merge-base): $before"
echo "after  (HEAD):       $after"

task=docs/tasks/0129_race_test_speedup/
adr=docs/dev/adr/0038-deadcode-and-race-free-timing.md
status=0
fail() { echo "FAIL: $1"; status=1; }
pass() { echo "PASS: $1"; }

# show <commit> <path>: the file's content; the caller sees a failure.
show() { git show "$1:$2" 2>/dev/null; }
# plain: drop comment-only and blank lines, and squeeze white space.
plain() { grep -v -E '^[[:space:]]*(//.*)?$' | sed -E 's/[[:space:]]+/ /g'; }

# funcbody <file-content-on-stdin> <name>: the lines of func <name> up to its
# closing brace; empty when the function is not there.
funcbody() {
  awk -v name="$1" '
    $0 ~ "^func " name "\\(" { on = 1 }
    on { print }
    on && /^}/ { exit }'
}

t=internal/transcript/copied_format_test.go
n=internal/transcript/copied_format_norace_test.go

# AC-02: the build without -race parses the same inputs.
if b=$(show "$before" $t | plain) && a=$(show "$after" $t | plain | sed 's/copiedLargeInputSize/copiedLargeInputLimit/g') \
   && [ -n "$b" ] && [ "$a" = "$b" ]; then
  pass "AC-02 $t is the same up to the constant's name"
else fail "AC-02 $t differs from before beyond the constant's name"; fi
if b=$(show "$before" $n | plain) && a=$(show "$after" $n | plain) && [ -n "$b" ]; then
  d=$(diff <(printf '%s\n' "$b") <(printf '%s\n' "$a") | grep -E '^[<>]' | sed -E 's/^([<>]) +/\1 /')
  if [ "$d" = "> copiedLargeInputSize = copiedLargeInputLimit" ]; then
    pass "AC-02 $n only adds copiedLargeInputSize = copiedLargeInputLimit"
  else fail "AC-02 $n differs from before: $d"; fi
else fail "AC-02 cannot read $n"; fi

# AC-04: the five tests and every subtest call t.Parallel() first.
parallel_check() { # file func
  local body
  body=$(show "$after" "$1" | funcbody "$2")
  if [ -z "$body" ]; then fail "AC-04 $2 not found in $1"; return; fi
  if ! printf '%s\n' "$body" | awk '
      NR == 2 { if ($0 !~ /^\tt\.Parallel\(\)$/) bad = 1 }
      prev ~ /t\.Run\(.*func\(t \*testing\.T\) \{$/ {
        line = $0; sub(/^\t+/, "", line); if (line != "t.Parallel()") bad = 1 }
      { prev = $0 }
      END { exit bad }'; then
    fail "AC-04 $2: the function or a subtest does not start with t.Parallel()"
  else pass "AC-04 $2 and its subtests start with t.Parallel()"; fi
}
parallel_check cmd/yt2column/deadcode_test.go TestMakeDeadcodeScope
parallel_check cmd/yt2column/deadcode_test.go TestMakeDeadcodeVerdict
parallel_check internal/ytdlpsource/json3_test.go TestParseSubtitlesLimits
parallel_check internal/ytdlpsource/json3_test.go TestParseSubtitlesArrayElementLimit
parallel_check internal/strictjson/strictjson_test.go TestAsArrayElementLimit

# AC-05, AC-10: those three files change only by added t.Parallel() lines.
for f in cmd/yt2column/deadcode_test.go internal/ytdlpsource/json3_test.go internal/strictjson/strictjson_test.go; do
  if d=$(git diff -U0 --no-renames "$before" "$after" -- "$f") && [ -n "$d" ] \
     && ! printf '%s\n' "$d" | grep -E '^[+-]' | grep -v -E '^(\+\+\+|---)' | grep -v -E '^\+[[:space:]]*t\.Parallel\(\)$' | grep -q .; then
    pass "AC-05/AC-10 $f changes only by added t.Parallel() lines"
  else fail "AC-05/AC-10 $f has a change other than added t.Parallel() lines (or none)"; fi
done

# AC-06: the two tests that measure time or allocation stay serial.
serial_check() {
  local body
  body=$(show "$after" "$1" | funcbody "$2")
  if [ -z "$body" ]; then fail "AC-06 $2 not found in $1"
  elif printf '%s\n' "$body" | grep -q 't\.Parallel()'; then fail "AC-06 $2 calls t.Parallel()"
  else pass "AC-06 $2 does not call t.Parallel()"; fi
}
serial_check internal/transcript/copied_format_test.go TestParseCopiedLargeInputs
serial_check internal/publisher/display_render_test.go TestNeutralizedDisplayMarkdownLinearTime

# AC-07: the linear-time test's file is unchanged.
f=internal/publisher/display_render_test.go
if git cat-file -e "$before:$f" 2>/dev/null && git cat-file -e "$after:$f" 2>/dev/null \
   && git diff --quiet "$before" "$after" -- "$f"; then pass "AC-07 $f is unchanged"
else fail "AC-07 $f changed or cannot be read"; fi

# AC-08: every changed file is a test file, a build-tagged test file, the
# task's directory, or ADR-0038.
if list=$(git diff --name-status --no-renames -z "$before" "$after" | tr '\0' '\n') ; then
  scope_ok=1
  while IFS= read -r st && IFS= read -r path; do
    case "$path" in
      *_test.go|"$task"*|"$adr") continue ;;
    esac
    if [ "$st" = D ]; then commit=$before; else commit=$after; fi
    first=$(show "$commit" "$path" | head -n 1)
    if [ "$first" != "//go:build test" ]; then echo "  out of scope: $st $path"; scope_ok=0; fi
  done < <(printf '%s\n' "$list")
  if [ -z "$list" ]; then fail "AC-08 no changed files"
  elif [ "$scope_ok" -eq 1 ]; then pass "AC-08 every changed file is in scope"
  else fail "AC-08 a changed file is out of scope"; fi
else fail "AC-08 cannot list the changed files"; fi
echo "changed files:"; git diff --name-status --no-renames "$before" "$after" | sed 's/^/  /'

[ "$status" -eq 0 ] && echo "RESULT: PASS" || echo "RESULT: FAIL"
exit "$status"
