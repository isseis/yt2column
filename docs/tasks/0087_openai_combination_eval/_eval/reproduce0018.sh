#!/bin/zsh
# Task 0087's reproduction of 0018 (design section 7.1): run the aggregation's cost
# and duration functions on 0018's B3 and C03 logs and compare the result with
# the values of the 0018 evaluation record section 4.8. evaltool also checks that
# B3's averages equal baseline.json's.
#
# Usage: reproduce0018.sh   (the 0018 directory is ~/yt2column-eval/0018, or
#   $YT2COLUMN_EVAL_ROOT/0018)

emulate -L zsh
set -u
setopt pipefail

here=${0:A:h}
repo=$(git -C $here rev-parse --show-toplevel) || exit 2
E=${YT2COLUMN_EVAL_ROOT:-$HOME/yt2column-eval}

# The 0018 evaluation record section 4.8: B3 3.615544 in total, 0.361554 on average,
# 0.610696 at most, 1:53.4 and 3:06.2; C03 3.572464, 1:52.1 and 3:05.2 (its
# average and maximum cost follow from its section 4.7.3 table).
expected=$'table reproduce0018 keys=1
config\tcost_sum_usd\tcost_avg_usd\tcost_max_usd\tcost_max_article\tduration_avg\tduration_max\tduration_max_article
B3\t3.615544\t0.361554\t0.610696\troNZ-rLxhUE-1\t1:53.4\t3:06.2\troNZ-rLxhUE-1
C03\t3.572464\t0.357246\t0.602652\troNZ-rLxhUE-1\t1:52.1\t3:05.2\troNZ-rLxhUE-1
end'

got=$(cd $repo && go run ./${here#$repo/}/evaltool reproduce0018 -base0018 $E/0018 -baseline $here/baseline.json) || {
  print -r -- "reproduce0018: evaltool failed" >&2
  exit 1
}
print -r -- $got
if [[ $got == $expected ]]; then
  print -r -- "reproduce0018: matches the 0018 evaluation record"
else
  diff <(print -r -- $expected) <(print -r -- $got)
  print -r -- "reproduce0018: differs from the 0018 evaluation record"
  exit 1
fi
