#!/bin/zsh
# Task 0087's repository-impact check (design section 7.3, requirements section 4.5):
# _eval/ must not change what "go list ./...", "make test", and "make lint"
# cover or report. The script compares, at the merge base with origin/main
# and at HEAD, the package list, the names of the tests "make test" runs
# (listed with "go test -tags test -list"), and the output of "make lint".
#
# Usage: repo_impact.sh   (from a clean work tree; it takes several minutes)

emulate -L zsh
set -u
setopt pipefail

here=${0:A:h}
repo=$(git -C $here rev-parse --show-toplevel) || exit 2
cd $repo || exit 2
[[ -z $(git status --porcelain) ]] || { print -r -- "repo_impact: the work tree is not clean" >&2; exit 2 }
base=$(git merge-base origin/main HEAD) || exit 2
head=$(git rev-parse HEAD) || exit 2
tmp=$(mktemp -d) || exit 2
wt=$tmp/base
cleanup() {
  git -C $repo worktree remove --force $wt > /dev/null 2>&1
  rm -rf -- $tmp
}
trap cleanup EXIT
git worktree add --detach $wt $base > /dev/null 2>&1 || { print -r -- "repo_impact: cannot check out $base" >&2; exit 2 }

# collect <tree> <prefix>: write the three outputs of one tree.
collect() {
  local tree=$1 out=$2
  ( cd $tree && go list ./... ) > $out.list || return 1
  ( cd $tree && go test -tags test -list '.*' ./... | grep -v -e '^ok ' -e '^?' ) > $out.tests || return 1
  ( cd $tree && make lint 2>&1 ) | grep -v -e '^go: downloading ' > $out.lint
  print -r -- "make lint exit=${pipestatus[1]}" >> $out.lint
}

collect $wt $tmp/base || { print -r -- "repo_impact: cannot collect at $base" >&2; exit 2 }
collect $repo $tmp/head || { print -r -- "repo_impact: cannot collect at HEAD" >&2; exit 2 }

fails=0
for kind in list tests lint; do
  if diff $tmp/base.$kind $tmp/head.$kind > $tmp/$kind.diff; then
    print -r -- "repo_impact: same $kind ($(wc -l < $tmp/head.$kind | tr -d ' ') lines)"
  else
    print -r -- "repo_impact: $kind differs:"
    cat $tmp/$kind.diff
    fails=1
  fi
done
print -r -- "repo_impact: base $base, head $head"
exit $fails
