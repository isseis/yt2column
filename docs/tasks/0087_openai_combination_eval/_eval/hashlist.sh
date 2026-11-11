#!/bin/zsh
# Task 0087's hash list (design section 3.3): one line per regular file of each tree,
# "<label>/<relative path>\t<mode>\t<SHA-256>", sorted by path in the C
# locale, and a diff against a previous list.
#
# Usage: hashlist.sh <out-file> <previous-file|-> <label>=<dir> ...
#   The evaluation's trees are base=~/yt2column-eval/0018/base/B3,
#   work=~/yt2column-eval/0018/work/B3, and cache=~/yt2column-eval/cache; the
#   run script reads the copies' expected hashes under these labels.
#   With a previous file it exits 1 when the new list differs from it.
#
# It assumes macOS (shasum, BSD stat), the machine the evaluation runs on.

emulate -L zsh
set -u
setopt pipefail

die() { print -r -- "hashlist.sh: $*" >&2; exit 2 }

(( $# >= 3 )) || die "usage: hashlist.sh <out-file> <previous-file|-> <label>=<dir> ..."
local out=$1 prev=$2
shift 2
[[ -e $out ]] && die "already exists: $out"
[[ $prev == - || -f $prev ]] || die "no previous list $prev"

local tmp=$out.partial
: > $tmp || die "cannot write $tmp"
local spec label dir odd f rel mode sha
for spec in "$@"; do
  label=${spec%%=*} dir=${spec#*=}
  [[ -n $label && $label != $spec && -d $dir ]] || die "not <label>=<dir>: $spec"
  odd=$(cd -- $dir && find . ! -type f ! -type d -print | head -n 1) || die "cannot walk $dir"
  [[ -z $odd ]] && odd=$(cd -- $dir && find . -name $'*[\t\n]*' -print | head -n 1)
  [[ -z $odd ]] || die "$dir holds an entry that is not a plainly named file or directory"
  for f in ${(f)"$(cd -- $dir && find . -type f -print)"}; do
    rel=${f#./}
    mode=$(stat -f %Lp -- $dir/$rel) || die "cannot stat $dir/$rel"
    sha=$(shasum -a 256 -- $dir/$rel | cut -d' ' -f1) || die "cannot hash $dir/$rel"
    print -r -- "$label/$rel"$'\t'"$mode"$'\t'"$sha"
  done >> $tmp || die "cannot write $tmp"
done
LC_ALL=C sort -o $out $tmp || die "cannot sort into $out"
rm -f -- $tmp
print -r -- "hashlist: wrote $out ($(wc -l < $out | tr -d ' ') files)"

[[ $prev == - ]] && exit 0
if diff -- $prev $out; then
  print -r -- "hashlist: no difference from $prev"
else
  print -r -- "hashlist: $out differs from $prev"
  exit 1
fi
