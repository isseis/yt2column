#!/bin/zsh
# Task 0087's retire script (design section 4, the retry procedure): before a retry, move
# every output of the article's finished current run to
# <root>/failed/<config>/<article>-f<N>/, N one above the highest so far, so no
# output of the previous run remains and its cost stays in the spend.
#
# Usage: retire.sh <0087 root> <config> <article>
#   It moves log/<config>/<article>.{txt,when} to log/, work/<config>/<article>
#   to work/, runcache/<config>/<article> to runcache/, and the article and
#   review log, when they exist, to out/ and review/.

emulate -L zsh
set -u
setopt pipefail

die() { print -r -- "retire.sh: $*" >&2; exit 1 }

(( $# == 3 )) || die "usage: retire.sh <0087 root> <config> <article>"
local R=$1 cfg=$2 art=$3
[[ $cfg =~ '^[A-Za-z-]+$' ]] || die "not a config: $cfg"
[[ $art =~ '^[A-Za-z0-9_-]{11}-[12]$' ]] || die "not an article: $art"
local log=$R/log/$cfg/$art.txt when=$R/log/$cfg/$art.when
[[ -f $log && -f $when ]] || die "no current run of $cfg $art"
grep -q '^exit=' $when || die "the current run of $cfg $art has not finished"

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
print -r -- "retire: moved $cfg $art to $D"
