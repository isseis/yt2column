#!/usr/bin/env bash
# Usage: ac04_moved_blocks.sh [--worktree] <commit>
#
# Compares the README blocks that task 0116 moves into the developer guides
# with where they landed at <commit>. The source is README.md at the
# merge-base of origin/main and <commit>, so a later fix to a block on main is
# never silently reverted by comparing against an older copy.
#
# For each block of the fixed list below, the script:
#   - cuts the block out of the source README and out of its target guide at
#     <commit>, both by the block's first and last sentence;
#   - checks that the target block sits under the expected heading;
#   - normalizes both sides (heading marks dropped, paragraph lines joined,
#     whitespace collapsed, one sentence per line, link targets replaced by a
#     placeholder) and prints their remaining diff;
#   - prints the pairs of the source and target link targets, in order.
# Then it checks that the moving is complete: no block's first or last
# sentence is still in README.md at <commit>, and each is in its target.
#
# With --worktree, README.md and the target guides are read from the work
# tree instead of <commit>; the source is still the merge-base README. It
# lets each check be broken by editing a file, without a commit. A recorded
# run passes the commit alone.
#
# The remaining diff and the link pairs are for the reviewer to read; the
# script fails only when a block cannot be found, is under another heading,
# or is still in the README. It exits 0 on success, 1 on a failed check, and
# 2 on a usage or git error.
set -euo pipefail

worktree=false
if [ "${1-}" = --worktree ]; then
  worktree=true
  shift
fi
if [ "$#" -ne 1 ]; then
  echo "usage: ac04_moved_blocks.sh [--worktree] <commit>" >&2
  exit 2
fi
commit=$(git rev-parse --verify "$1^{commit}") || exit 2
base=$(git merge-base origin/main "$commit") || exit 2
readme_blob=$(git rev-parse "$base:README.md") || exit 2

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
git show "$base:README.md" >"$work/source.md"
# compared prints the compared side's copy of the repository file $1.
compared() {
  if [ "$worktree" = true ]; then
    cat -- "$(git rev-parse --show-toplevel)/$1"
  else
    git show "$commit:$1"
  fi
}
compared README.md >"$work/readme-now.md"

# The moved blocks: name, target document, expected heading in the target
# (without its "#" marks), first sentence, last sentence. A sentence is
# matched as a substring of one normalized unit, so it may be a fragment
# that only that unit holds.
names=() targets=() headings=() firsts=() lasts=()
block() {
  names+=("$1") targets+=("$2") headings+=("$3") firsts+=("$4") lasts+=("$5")
}
gen=docs/dev/guides/generation.md
dev=docs/dev/guides/development.md
block intro-cli "$gen" 構成 '補助の経路として CLI も含む。' 'で文字起こしを取得する。'
block diagram "$gen" 構成 'COPIED[("Copied transcript' 'class L2 process'
block paths-and-providers "$gen" 構成 '主経路は、文字起こしパネルからコピーした字幕を使う。' 'プロバイダは差し替えられる。'
block build-requirements "$dev" 拡張のビルドと読み込み '- Go（`go.mod` を参照）。' '（拡張のビルドに使う）'
block post-generation-steps "$gen" 生成の後の工程 '記事を生成した後、2 つの工程が走る。' '拡張は、どの工程も飛ばさない。'
block output-format "$gen" 出力の書式 '記事の情報の並べ方は、出力ごとに異なる。' 'CLI の `--out` のファイルと同じ書式である。'
block estimated-cost "$gen" 推定の料金 '記事の料金は、CLI の `--out` のファイルでは' 'CLI が標準エラーに書く料金の行は'
block extension-build "$dev" 拡張のビルドと読み込み 'Chrome/Brave の拡張（Manifest V3）は `extension/` にある。' '拡張はシークレットモードでは有効にできない。'
block extension-logs "$dev" 拡張のログ '集めたものと結果を見るには' 'API キー・ジョブ ID を書かない。'
block configuration "$gen" 設定 '| 変数 | 説明 | 未設定のとき |' '`YT2COLUMN_LLM` の値に低い effort を付ける'
block recommended-configuration "$gen" 推奨構成 'CLI の環境変数の値:' 'この構成の品質の目標、結果、製品の制約は'

# normalize prints the document on standard input as "<heading>\t<unit>"
# lines: <unit> is one sentence of prose (paragraph lines joined, whitespace
# collapsed, split after each "。"), one table row, one list item's
# sentences, or one line of a fenced code block, verbatim; <heading> is the
# text of the nearest heading above, without its "#" marks. Headings
# themselves are not units, so a changed heading level leaves no diff.
normalize() {
  awk '
    function flush(   s, n, i, parts) {
      if (para == "") return
      s = para; para = ""
      gsub(/[ \t]+/, " ", s); sub(/^ /, "", s); sub(/ $/, "", s)
      n = split(s, parts, "。")
      for (i = 1; i <= n; i++) {
        unit = parts[i]
        sub(/^ /, "", unit)
        if (i < n) unit = unit "。"
        if (unit != "") print heading "\t" unit
      }
    }
    /^```/ { flush(); fence = !fence; print heading "\t" $0; next }
    fence { print heading "\t" $0; next }
    /^#+ / { flush(); heading = $0; sub(/^#+ /, "", heading); next }
    /^[ \t]*$/ { flush(); next }
    /^\|/ { flush(); print heading "\t" $0; next }
    /^(- |[0-9]+\. )/ { flush(); para = $0; next }
    { para = (para == "" ? $0 : para " " $0) }
    END { flush() }
  '
}

# cut prints the units from the first one holding $2 through the first one
# after it holding $3, from the normalized file $1. It prints nothing when
# either sentence is missing.
cut_block() {
  awk -F '\t' -v first="$2" -v last="$3" '
    !started && index($2, first) { started = 1 }
    started { lines[++n] = $0 }
    started && index($2, last) { done = 1; exit }
    END { if (done) for (i = 1; i <= n; i++) print lines[i] }
  ' "$1"
}

# links prints the target of every inline link in the units on standard
# input, one per line, in order.
links() {
  grep -oE '\]\([^)[:space:]]+\)' | sed -E 's/^\]\((.*)\)$/\1/' || true
}

# text drops the heading column and replaces each link target with a
# placeholder, so the diff shows only the text.
text() {
  cut -f2- | sed -E 's/\]\([^)[:space:]]+\)/](<link>)/g'
}

normalize <"$work/source.md" >"$work/source.norm"
normalize <"$work/readme-now.md" >"$work/readme-now.norm"

if [ "$worktree" = true ]; then
  echo "compared:    the work tree (merge-base taken with $commit)"
else
  echo "commit:      $commit"
fi
echo "merge-base:  $base"
echo "README blob: $readme_blob"
echo

failed=0
count=0
for k in "${!names[@]}"; do
  name=${names[k]} target=${targets[k]} heading=${headings[k]}
  first=${firsts[k]} last=${lasts[k]}
  count=$((count + 1))
  echo "=== block $count: $name -> $target ($heading)"
  if ! compared "$target" 2>/dev/null | normalize >"$work/target.norm"; then
    echo "FAIL: $target does not exist on the compared side"
    failed=1
    continue
  fi
  cut_block "$work/source.norm" "$first" "$last" >"$work/src.blk"
  cut_block "$work/target.norm" "$first" "$last" >"$work/dst.blk"
  if [ ! -s "$work/src.blk" ]; then
    echo "FAIL: the merge-base README has no block from \"$first\" to \"$last\""
    failed=1
    continue
  fi
  if [ ! -s "$work/dst.blk" ]; then
    echo "FAIL: $target has no block from \"$first\" to \"$last\""
    failed=1
    continue
  fi
  got_heading=$(head -n 1 "$work/dst.blk" | cut -f1)
  src_heading=$(head -n 1 "$work/src.blk" | cut -f1)
  echo "headings: README \"$src_heading\" -> $target \"$got_heading\""
  if [ "$got_heading" != "$heading" ]; then
    echo "FAIL: the block is under \"$got_heading\" in $target, want \"$heading\""
    failed=1
  fi
  echo "--- remaining diff (README -> $target):"
  if diff -u <(text <"$work/src.blk") <(text <"$work/dst.blk") | tail -n +3; then
    echo "(none)"
  fi
  echo "--- link pairs (README -> $target):"
  paste -d ' ' <(links <"$work/src.blk" | sed 's/$/ ->/') <(links <"$work/dst.blk") | sed 's/^/  /'
  echo
done

echo "=== completeness on the compared side"
for k in "${!names[@]}"; do
  name=${names[k]}
  for sentence in "${firsts[k]}" "${lasts[k]}"; do
    if cut -f2- "$work/readme-now.norm" | grep -qF -- "$sentence"; then
      echo "FAIL: $name: README.md still holds \"$sentence\""
      failed=1
    fi
  done
done
echo "blocks compared: $count"

if [ "$failed" -ne 0 ]; then
  echo "ac04: FAIL"
  exit 1
fi
echo "ac04: PASS"
