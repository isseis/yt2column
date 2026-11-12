#!/bin/zsh
# Task 0087's self-test (plan step 1-5): the evaltool package's tests, vet,
# golangci-lint at the Makefile's version, and gofumpt; hashlist.sh's change
# detection; and secretscan.sh's detection with fake values. It calls no API
# and reads no evaluation directory.
#
# Usage: selftest.sh   (from anywhere inside the repository)

emulate -L zsh
set -u
setopt pipefail

here=${0:A:h}
repo=$(git -C $here rev-parse --show-toplevel) || exit 2
pkg=./${here#$repo/}/evaltool
fails=0

step() {
  local name=$1
  shift
  if "$@"; then
    print -r -- "selftest: ok   $name"
  else
    print -r -- "selftest: FAIL $name"
    fails=$(( fails + 1 ))
  fi
}

# expect_status <code> <command...>: run the command and require its exit
# status, discarding its output.
expect_status() {
  local want=$1
  shift
  "$@" > /dev/null 2>&1
  (( $? == want ))
}

cd $repo || exit 2
lint_version=$(sed -n 's/^GOLANGCI_VERSION?=//p' Makefile)
[[ -n $lint_version ]] || { print -r -- "selftest: no GOLANGCI_VERSION in the Makefile" >&2; exit 2 }

step "go test $pkg" go test -count=1 $pkg
step "go vet $pkg" go vet $pkg
step "golangci-lint $lint_version $pkg" go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$lint_version run $pkg
# gofumpt_clean: gofumpt must succeed and list no file.
gofumpt_clean() {
  local out
  out=$(gofumpt -l ${pkg#./}) || return 1
  [[ -z $out ]]
}
step "gofumpt $pkg" gofumpt_clean

tmp=$(mktemp -d) || exit 2
trap 'rm -rf -- $tmp' EXIT
mkdir -p $tmp/tree/sub && print a > $tmp/tree/sub/f && print b > $tmp/tree/g && chmod 600 $tmp/tree/g
H=$here/hashlist.sh
step "hashlist: first list" expect_status 0 $H $tmp/1.tsv - t=$tmp/tree
step "hashlist: unchanged tree gives an empty diff" expect_status 0 $H $tmp/2.tsv $tmp/1.tsv t=$tmp/tree
print c >> $tmp/tree/g
step "hashlist: changed content gives a diff" expect_status 1 $H $tmp/3.tsv $tmp/1.tsv t=$tmp/tree
print b > $tmp/tree/g
step "hashlist: restored content gives an empty diff" expect_status 0 $H $tmp/4.tsv $tmp/1.tsv t=$tmp/tree
chmod 644 $tmp/tree/g
step "hashlist: changed mode gives a diff" expect_status 1 $H $tmp/5.tsv $tmp/1.tsv t=$tmp/tree

X=$here/secretscan.sh
fake=fake-value-for-selftest-$$
mkdir $tmp/scan $tmp/clean
print -r -- "a line with $fake in it" > $tmp/scan/leak.txt
# The prefix sample is built at run time so this file does not hold one.
pad=abcdefghijklmnopqrst
print -r -- "a line with sk-$pad in it" > $tmp/scan/prefix.txt
print -r -- "nothing to see" > $tmp/clean/ok.txt
clean_env=(env -u ANTHROPIC_API_KEY -u OPENAI_API_KEY -u DEEPSEEK_API_KEY -u SLACK_WEBHOOK_URL
  -u YT2COLUMN_TEST_ANTHROPIC_API_KEY -u YT2COLUMN_TEST_OPENAI_API_KEY
  -u YT2COLUMN_TEST_DEEPSEEK_API_KEY -u YT2COLUMN_TEST_SLACK_WEBHOOK_URL)
step "secretscan: finds a key value in a file" expect_status 1 $clean_env OPENAI_API_KEY=$fake $X $tmp/scan/leak.txt
step "secretscan: finds the sk- prefix" expect_status 1 $clean_env OPENAI_API_KEY=$fake $X $tmp/scan/prefix.txt
step "secretscan: passes a clean file" expect_status 0 $clean_env OPENAI_API_KEY=$fake $X $tmp/clean
step "secretscan: finds a test key value" expect_status 1 $clean_env YT2COLUMN_TEST_OPENAI_API_KEY=$fake $X $tmp/scan/leak.txt
step "secretscan: fails with no key variable" expect_status 2 $clean_env $X $tmp/clean
step "secretscan: fails on a missing path" expect_status 2 $clean_env OPENAI_API_KEY=$fake $X $tmp/missing
print -r -- x > $tmp/clean/unreadable.txt && chmod 000 $tmp/clean/unreadable.txt
step "secretscan: fails when grep cannot read a file" expect_status 2 $clean_env OPENAI_API_KEY=$fake $X $tmp/clean
chmod 600 $tmp/clean/unreadable.txt
step "secretscan: never prints the value" test -z "$($clean_env OPENAI_API_KEY=$fake $X $tmp/scan 2>&1 | grep -F -- $fake)"

if (( fails )); then
  print -r -- "selftest: $fails check(s) failed"
  exit 1
fi
print -r -- "selftest: all checks passed"
