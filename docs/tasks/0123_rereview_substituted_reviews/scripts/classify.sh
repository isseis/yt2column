#!/usr/bin/env bash
# classify.sh OUTDIR REPO — no network. Prints, per merged PR, the signals
# that its Opus review or Fable partial review ran on another model or not
# at all. Run it in the repository after `git fetch origin main`.
set -euo pipefail
OUT=${1:?OUTDIR}; REPO=${2:?repository path}; HERE=$(cd "$(dirname "$0")" && pwd)
jq -r 'sort_by(.mergedAt)[] | "\(.number)\t\(.mergeCommit.oid)\t\(.mergedAt[0:10])\t\(.title)"' "$OUT/prs.json" |
while IFS=$'\t' read -r n squash date title; do
  for f in "$OUT/pr-$n.commits" "$OUT/pr-$n.body"; do
    [ -f "$f" ] || { echo "classify.sh: missing $f; rerun fetch.sh" >&2; exit 1; }
  done
  sig=$( { awk -f "$HERE/classify.awk" "$OUT/pr-$n.commits"
           { echo "@@@COMMIT squash-body-"; git -C "$REPO" -c log.showSignature=false log -1 --format=%B "$squash" 2>/dev/null || true; } | awk -f "$HERE/classify.awk"
           { echo "@@@COMMIT pr-body----"; cat "$OUT/pr-$n.body"; } | awk -f "$HERE/classify.awk" | grep '^FREETEXT' || true
         } | sort -u )
  if [[ $title =~ ^[a-z]+\([0-9]{4}\) ]]; then
    grep -qE '^(fable-ok|FABLE-)' <<<"$sig" || sig+=$'\n'"NO-FABLE-RECORD - (no Fable line on the branch)"
    if [[ ! $title =~ ^docs ]]; then
      grep -qiE '^(opus-done|OPUS-DONE)' <<<"$sig" || sig+=$'\n'"NO-HIGH-REVIEW-RECORD - (check the plan's requirement: not needed for low or all-high)"
    fi
  fi
  flag=$(printf '%s\n' "$sig" | grep -cE '^[A-Z]' || true)
  printf '#%s %s %s %s  [%s]\n' "$n" "${squash:0:12}" "$date" "$title" "$([ "$flag" -gt 0 ] && echo CANDIDATE || echo ok)"
  printf '%s\n' "$sig" | sed '/^$/d; s/^/    /'
done
