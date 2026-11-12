#!/bin/zsh
# Task 0087's retire script (design section 4, the retry procedure): before a retry, move
# every output of the article's finished current run to
# <root>/failed/<config>/<article>-f<N>/, N one above the highest so far, so no
# output of the previous run remains and its cost stays in the spend.
#
# Usage: retire.sh [--unfinished] <0087 root> <config> <article>
#   --unfinished retires a run whose record has no exit line: a run whose
#   script was killed (kill -9) before it could close the record. Use it only
#   after checking that no CLI of the run is still running (pgrep -f). It
#   appends "retired_unfinished=<time>" to the record; the run keeps no cost
#   total, so the spend counts it at its bound.
#   It moves log/<config>/<article>.{txt,when} to log/, work/<config>/<article>
#   to work/, runcache/<config>/<article> to runcache/, and the article and
#   review log, when they exist, to out/ and review/. A prepayment.tsv line
#   that confirmed the current run ("cur") is rewritten to the new f<N>, so it
#   keeps naming the same run.

emulate -L zsh
set -u
setopt pipefail

die() { print -r -- "retire.sh: $*" >&2; exit 1 }

local unfinished=0
if [[ ${1-} == --unfinished ]]; then
  unfinished=1
  shift
fi
(( $# == 3 )) || die "usage: retire.sh [--unfinished] <0087 root> <config> <article>"
local R=$1 cfg=$2 art=$3
[[ $cfg =~ '^[A-Za-z-]+$' ]] || die "not a config: $cfg"
[[ $art =~ '^[A-Za-z0-9_-]{11}-[12]$' ]] || die "not an article: $art"
local log=$R/log/$cfg/$art.txt when=$R/log/$cfg/$art.when
[[ -f $log && -f $when ]] || die "no current run of $cfg $art"
if (( unfinished )); then
  grep -q '^exit=' $when && die "the current run of $cfg $art has finished; retire it without --unfinished"
  print -r -- "retired_unfinished=$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> $when || die "cannot write $when"
else
  grep -q '^exit=' $when || die "the current run of $cfg $art has not finished (see --unfinished)"
fi

local n=1
while [[ -e $R/failed/$cfg/$art-f$n ]]; do (( n++ )); done
local D=$R/failed/$cfg/$art-f$n
mkdir -p -m 700 $D/log || die "cannot create $D"
mv -- $log $when $D/log/ || die "cannot move the log"
[[ -e $R/work/$cfg/$art ]] && { mv -- $R/work/$cfg/$art $D/work || die "cannot move the artifacts" }
[[ -e $R/runcache/$cfg/$art ]] && { mv -- $R/runcache/$cfg/$art $D/runcache || die "cannot move the cache copy" }
if [[ -e $R/out/$cfg/$art.md ]]; then
  mkdir -m 700 $D/out && mv -- $R/out/$cfg/$art.md $D/out/ || die "cannot move the article"
fi
if [[ -e $R/review/$cfg/$art.md ]]; then
  mkdir -m 700 $D/review && mv -- $R/review/$cfg/$art.md $D/review/ || die "cannot move the review log"
fi
local P=$R/prepayment.tsv
if [[ -f $P ]] && grep -qxF -- "$cfg"$'\t'"$art"$'\tcur' $P; then
  awk -F'\t' -v OFS='\t' -v c="$cfg" -v a="$art" -v n="f$n" '$1 == c && $2 == a && $3 == "cur" { $3 = n } { print }' $P > $P.new &&
    mv -- $P.new $P || die "cannot update $P"
  print -r -- "retire: prepayment.tsv now names $cfg $art f$n"
fi
print -r -- "retire: moved $cfg $art to $D"
