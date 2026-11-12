#!/bin/zsh
# Task 0087's rate comparison (design section 3.4 item 3): compare the
# claude-opus-5-5 rates of internal/pricing/rates.go at the evaluation's
# commit with those at 2fc2e2a, the commit of B3's CLI (0018 evaluation
# record section 4.1.2). Only the rate set is compared, not the line it sits on.
#
# Usage: rates-compare.sh <evaluation commit>   (run inside the repository)
# It exits 0 when the rates are the same and 1 when they differ.

emulate -L zsh
set -u
setopt pipefail

die() { print -r -- "rates-compare.sh: $*" >&2; exit 2 }

(( $# == 1 )) || die "usage: rates-compare.sh <evaluation commit>"
local B3_COMMIT=2fc2e2a

# rates_of <commit>: the claude-opus-5-5 entry's "base:" rate set, without
# its comment and white space.
rates_of() {
  git show "$1:internal/pricing/rates.go" |
    awk '/model:[ \t]*"claude-opus-5-5"/ { f = 1 } f && /base:/ { sub(/\/\/.*/, ""); gsub(/[ \t]/, ""); print; exit }'
}

local eval_rates b3_rates
eval_rates=$(rates_of $1) || die "cannot read rates.go at $1"
b3_rates=$(rates_of $B3_COMMIT) || die "cannot read rates.go at $B3_COMMIT"
[[ -n $eval_rates && -n $b3_rates ]] || die "no claude-opus-5-5 rate set found"
print -r -- "evaluation commit $(git rev-parse --short=12 $1): $eval_rates"
print -r -- "B3 commit $B3_COMMIT: $b3_rates"
if [[ $eval_rates == $b3_rates ]]; then
  print -r -- "rates-compare: same"
else
  print -r -- "rates-compare: differ"
  exit 1
fi
