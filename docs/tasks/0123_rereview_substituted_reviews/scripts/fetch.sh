#!/usr/bin/env bash
# fetch.sh SINCE OUTDIR — read-only network: lists PRs merged into main since
# SINCE (YYYY-MM-DD) and saves each PR's branch commit messages and body.
set -euo pipefail
SINCE=${1:?SINCE (YYYY-MM-DD)}; OUT=${2:?OUTDIR}
mkdir -p "$OUT"
gh pr list --state merged --base main --search "merged:>=$SINCE" --limit 300 \
  --json number,title,mergedAt,mergeCommit,headRefName,headRefOid > "$OUT/prs.json"
if [ "$(jq length "$OUT/prs.json")" -ge 300 ]; then
  echo "fetch.sh: 300 PRs reached; the list may be incomplete, split the period" >&2
  exit 1
fi
for n in $(jq -r '.[].number' "$OUT/prs.json"); do
  gh api --paginate "repos/{owner}/{repo}/pulls/$n/commits" \
    --jq '.[] | "@@@COMMIT \(.sha)\n\(.commit.message)"' > "$OUT/pr-$n.commits"
  gh pr view "$n" --json body --jq .body > "$OUT/pr-$n.body"
done
echo "fetched $(jq length "$OUT/prs.json") PRs into $OUT"
