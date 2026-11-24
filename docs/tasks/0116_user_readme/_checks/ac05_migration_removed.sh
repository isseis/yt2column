#!/usr/bin/env bash
# Usage: ac05_migration_removed.sh [--only-a] <commit>
#
# Checks that the earlier-version migration is gone, as the task's AC-05 asks.
# The source README is README.md at the merge-base of origin/main and <commit>,
# so the check reads the migration where it still exists; the compared side is
# <commit>.
#
# The checks are:
#   (a) neither the Japanese nor the English migration heading is a heading
#       in any tracked .md file other than docs/tasks/;
#   (b) no line of the README's migration section, and no line of the
#       quick-start note that pointed at it, survives in any tracked .md file
#       other than docs/tasks/ (lines are compared with their whitespace
#       collapsed; blank lines, code-fence lines, and lines shorter than 20
#       characters are skipped);
#   (c) the (file, word) pairs of the words only the migration used
#       (yt2column-nmhost, NativeMessagingHosts, config.env, sharedToken) do
#       not grow from the merge-base when README.md, README.en.md, the
#       translation glossary, and docs/tasks/ are left out, and the
#       translation glossary keeps exactly the two pairs that define the
#       retired terms.
#
# --only-a runs check (a) alone. Phase 5 runs it after the migration is gone
# from the merge-base too, which would make (b) and (c) vacuous.
#
# The script fails when the merge-base README has no migration section or no
# quick-start note, so a vacuous run cannot pass. It exits 0 on success, 1 on a
# failed check, and 2 on a usage or git error.
set -euo pipefail
# Count characters, not bytes, whatever the caller's locale. C.UTF-8, not
# en_US.UTF-8, whose collation on macOS treats distinct Japanese strings as
# equal.
export LC_ALL=C.UTF-8

only_a=false
if [ "${1-}" = --only-a ]; then
  only_a=true
  shift
fi
if [ "$#" -ne 1 ]; then
  echo "usage: ac05_migration_removed.sh [--only-a] <commit>" >&2
  exit 2
fi
commit=$(git rev-parse --verify "$1^{commit}") || exit 2
base=$(git merge-base origin/main "$commit") || exit 2

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

git show "$base:README.md" >"$work/source.md" || exit 2
readme_blob=$(git rev-parse "$base:README.md") || exit 2

# docs_at prints the tracked .md files at revision $1, other than docs/tasks/,
# which holds past task records that quote the migration on purpose.
docs_at() {
  git ls-tree -r --name-only "$1" | grep -E '\.md$' | grep -v '^docs/tasks/' || true
}

# collapse prints standard input with runs of whitespace turned into one
# space and the ends trimmed.
collapse() {
  sed -E 's/[[:space:]]+/ /g; s/^ //; s/ $//'
}

echo "commit:      $commit"
echo "merge-base:  $base"
echo "README blob: $readme_blob"
echo

failed=0

# (a) A migration heading in any document other than docs/tasks/.
echo "=== (a) no migration heading outside docs/tasks/"
heading_re='^#+[[:space:]]+(以前の版からの移行|Migrating from the earlier version)[[:space:]]*$'
a_found=0
while IFS= read -r doc; do
  [ -n "$doc" ] || continue
  if git show "$commit:$doc" | grep -nE -- "$heading_re" >"$work/heading.hits"; then
    while IFS= read -r hit; do
      echo "  $doc:$hit"
    done <"$work/heading.hits"
    a_found=1
  fi
done < <(docs_at "$commit")
if [ "$a_found" -eq 0 ]; then
  echo "  (none)"
else
  failed=1
fi
echo

if [ "$only_a" = true ]; then
  if [ "$failed" -ne 0 ]; then
    echo "ac05: FAIL"
    exit 1
  fi
  echo "ac05: PASS"
  exit 0
fi

# (b) A line of the removed migration, or of the quick-start note, left
# behind in another document.
echo "=== (b) no old migration line in another document"
awk '
  /^##[[:space:]]/ {
    if ($0 ~ /^## 以前の版からの移行[[:space:]]*$/) {
      inmigration = 1
      next
    }
    inmigration = 0
    inquick = ($0 ~ /^## クイックスタート[[:space:]]*$/)
    next
  }
  inmigration { print }
  inquick && /以前の版からの移行/ { print }
' "$work/source.md" >"$work/old-lines"
while IFS= read -r line; do
  normalized=$(printf '%s\n' "$line" | collapse)
  [ -n "$normalized" ] || continue
  case $normalized in
  '```'* | '~~~'*) continue ;;
  esac
  [ "${#normalized}" -ge 20 ] || continue
  printf '%s\n' "$normalized"
done <"$work/old-lines" | sort -u >"$work/old-normalized"
if [ ! -s "$work/old-normalized" ]; then
  echo "  FAIL: the merge-base README has no migration lines to compare"
  failed=1
else
  : >"$work/all-lines"
  while IFS= read -r doc; do
    [ -n "$doc" ] || continue
    git show "$commit:$doc" | collapse | awk -v d="$doc" '{ print d "\t" $0 }' >>"$work/all-lines"
  done < <(docs_at "$commit")
  b_found=0
  while IFS= read -r want; do
    while IFS= read -r hit; do
      [ -n "$hit" ] || continue
      echo "  $hit"
      b_found=1
    done < <(awk -F '\t' -v w="$want" '$2 == w { print $1 ": " $2 }' "$work/all-lines")
  done <"$work/old-normalized"
  if [ "$b_found" -eq 0 ]; then
    echo "  (none)"
  else
    failed=1
  fi
fi
echo

# (c) The migration-only words did not spread, and the glossary keeps only
# the two pairs that define the retired terms.
echo "=== (c) migration-only words"
words=(yt2column-nmhost NativeMessagingHosts config.env sharedToken)

# pairs_at prints "<file>\t<word>" for every tracked .md file at revision $1
# that holds $2, other than README.md, README.en.md, the translation glossary,
# and docs/tasks/.
pairs_at() {
  local rev=$1 word=$2 doc content
  while IFS= read -r doc; do
    [ -n "$doc" ] || continue
    case $doc in
    README.md | README.en.md | docs/translation_glossary.md) continue ;;
    esac
    content=$(git show "$rev:$doc")
    if grep -qF -- "$word" <<<"$content"; then
      printf '%s\t%s\n' "$doc" "$word"
    fi
  done < <(docs_at "$rev")
}

: >"$work/pairs-base"
: >"$work/pairs-now"
for word in "${words[@]}"; do
  pairs_at "$base" "$word" >>"$work/pairs-base"
  pairs_at "$commit" "$word" >>"$work/pairs-now"
done
sort -u "$work/pairs-base" -o "$work/pairs-base"
sort -u "$work/pairs-now" -o "$work/pairs-now"
if comm -13 "$work/pairs-base" "$work/pairs-now" >"$work/pairs-new" && [ -s "$work/pairs-new" ]; then
  echo "  FAIL: these (file, word) pairs are new:"
  sed 's/^/    /' "$work/pairs-new"
  failed=1
else
  echo "  compared pairs (unchanged from the merge-base):"
  sed 's/\t/: /' "$work/pairs-now" | sed 's/^/    /'
fi

# The glossary keeps the definitions of the retired terms, so exactly these
# two words may remain there.
expected_glossary=$(printf '%s\n' config.env yt2column-nmhost | sort)
actual_glossary=$(
  for word in "${words[@]}"; do
    if git show "$commit:docs/translation_glossary.md" | grep -qF -- "$word"; then
      printf '%s\n' "$word"
    fi
  done | sort
)
if [ "$actual_glossary" = "$expected_glossary" ]; then
  echo "  glossary pairs (as expected):"
  printf '%s\n' "$actual_glossary" | sed 's/^/    docs\/translation_glossary.md: /'
else
  echo "  FAIL: the glossary holds words other than the expected definitions:"
  printf '    want: %s\n' "$(printf '%s' "$expected_glossary" | tr '\n' ' ')"
  printf '    got:  %s\n' "$(printf '%s' "$actual_glossary" | tr '\n' ' ')"
  failed=1
fi
echo

if [ "$failed" -ne 0 ]; then
  echo "ac05: FAIL"
  exit 1
fi
echo "ac05: PASS"
