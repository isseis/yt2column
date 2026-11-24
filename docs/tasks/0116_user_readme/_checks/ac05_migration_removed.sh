#!/usr/bin/env bash
# Usage: ac05_migration_removed.sh [--only-a] <commit>
#
# Checks that the earlier-version migration is gone, as the task's AC-05 asks.
# The source is README.md and README.en.md at the merge-base of origin/main and
# <commit>, so the check reads the migration where it still exists; the compared
# side is <commit>.
#
# The checks are:
#   (a) neither the Japanese nor the English migration heading is a heading
#       in any tracked .md file other than docs/tasks/;
#   (b) no line of either README's migration section, and no line of either
#       README's quick-start note that pointed at it, survives in any tracked
#       .md file other than docs/tasks/ (lines are compared with their
#       whitespace collapsed; blank lines, code-fence lines, and lines shorter
#       than 20 characters are skipped);
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
# Check (b) fails when either merge-base README has no migration section or no
# quick-start note, so a vacuous run cannot pass. A git failure aborts with
# exit 2 instead of reading as a clean run. The script exits 0 on success, 1 on
# a failed check, and 2 on a usage or git error.
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

# docs_at prints the tracked .md files at revision $1, other than docs/tasks/,
# which holds past task records that quote the migration on purpose.
docs_at() {
  local rev=$1 tree
  tree=$(git ls-tree -r --name-only "$rev") || return 1
  printf '%s\n' "$tree" | grep -E '\.md$' | grep -v '^docs/tasks/' || true
}

# collapse prints standard input with runs of whitespace turned into one
# space and the ends trimmed.
collapse() {
  sed -E 's/[[:space:]]+/ /g; s/^ //; s/ $//'
}

# old_lines prints, with an S or N marker, the lines of the migration section
# and of the quick-start note that points at it, from the README file $1. $2 is
# the migration heading and $3 the phrase the note uses.
old_lines() {
  awk -v mig="$2" -v note="$3" '
    /^##[[:space:]]/ {
      if ($0 ~ mig) {
        insec = 1
        next
      }
      insec = 0
      inquick = ($0 ~ /^## (クイックスタート|Quick start)[[:space:]]*$/)
      next
    }
    insec { print "S\t" $0 }
    inquick && $0 ~ note { print "N\t" $0 }
  ' "$1"
}

echo "commit:      $commit"
echo "merge-base:  $base"
echo

failed=0

# (a) A migration heading in any document other than docs/tasks/.
DOCS_NOW=$(docs_at "$commit") || {
  echo "FAIL: git ls-tree failed for $commit" >&2
  exit 2
}
echo "=== (a) no migration heading outside docs/tasks/"
heading_re='^#+[[:space:]]+(以前の版からの移行|Migrating from the earlier version)[[:space:]]*$'
a_found=0
while IFS= read -r doc; do
  [ -n "$doc" ] || continue
  content=$(git show "$commit:$doc") || {
    echo "FAIL: cannot read $doc at $commit" >&2
    exit 2
  }
  if printf '%s\n' "$content" | grep -nE -- "$heading_re" >"$work/heading.hits"; then
    while IFS= read -r hit; do
      echo "  $doc:$hit"
    done <"$work/heading.hits"
    a_found=1
  fi
done <<<"$DOCS_NOW"
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

git show "$base:README.md" >"$work/source-ja.md" || {
  echo "FAIL: cannot read the merge-base README.md" >&2
  exit 2
}
git show "$base:README.en.md" >"$work/source-en.md" || {
  echo "FAIL: cannot read the merge-base README.en.md" >&2
  exit 2
}
readme_blob=$(git rev-parse "$base:README.md") || exit 2
echo "README blob: $readme_blob"
echo

DOCS_BASE=$(docs_at "$base") || {
  echo "FAIL: git ls-tree failed for $base" >&2
  exit 2
}

# (b) A line of the removed migration, or of the quick-start note, left
# behind in another document.
echo "=== (b) no old migration line in another document"
old_lines "$work/source-ja.md" '^## 以前の版からの移行[[:space:]]*$' '以前の版からの移行' >"$work/old-ja"
old_lines "$work/source-en.md" '^## Migrating from the earlier version[[:space:]]*$' 'earlier version' >"$work/old-en"
b_source_ok=true
if ! grep -q $'^S\t' "$work/old-ja"; then
  echo "  FAIL: the merge-base README.md has no migration section"
  b_source_ok=false
fi
if ! grep -q $'^N\t' "$work/old-ja"; then
  echo "  FAIL: the merge-base README.md has no quick-start note"
  b_source_ok=false
fi
if ! grep -q $'^S\t' "$work/old-en"; then
  echo "  FAIL: the merge-base README.en.md has no migration section"
  b_source_ok=false
fi
if ! grep -q $'^N\t' "$work/old-en"; then
  echo "  FAIL: the merge-base README.en.md has no quick-start note"
  b_source_ok=false
fi
if [ "$b_source_ok" = false ]; then
  failed=1
else
  {
    cut -f2- "$work/old-ja"
    cut -f2- "$work/old-en"
  } | while IFS= read -r line; do
    normalized=$(printf '%s\n' "$line" | collapse)
    [ -n "$normalized" ] || continue
    case $normalized in
    '```'* | '~~~'*) continue ;;
    esac
    [ "${#normalized}" -ge 20 ] || continue
    printf '%s\n' "$normalized"
  done | sort -u >"$work/old-normalized"
  : >"$work/all-lines"
  while IFS= read -r doc; do
    [ -n "$doc" ] || continue
    content=$(git show "$commit:$doc") || {
      echo "FAIL: cannot read $doc at $commit" >&2
      exit 2
    }
    printf '%s\n' "$content" | collapse | awk -v d="$doc" '{ print d "\t" $0 }' >>"$work/all-lines"
  done <<<"$DOCS_NOW"
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

# pairs_at prints "<file>\t<word>" for every document of $2 at revision $1
# that holds $3, other than README.md, README.en.md, and the glossary.
pairs_at() {
  local rev=$1 docs=$2 word=$3 doc content
  while IFS= read -r doc; do
    [ -n "$doc" ] || continue
    case $doc in
    README.md | README.en.md | docs/translation_glossary.md) continue ;;
    esac
    content=$(git show "$rev:$doc") || return 1
    if grep -qF -- "$word" <<<"$content"; then
      printf '%s\t%s\n' "$doc" "$word"
    fi
  done <<<"$docs"
}

: >"$work/pairs-base"
: >"$work/pairs-now"
for word in "${words[@]}"; do
  pairs_at "$base" "$DOCS_BASE" "$word" >>"$work/pairs-base" || {
    echo "FAIL: cannot read a document at $base" >&2
    exit 2
  }
  pairs_at "$commit" "$DOCS_NOW" "$word" >>"$work/pairs-now" || {
    echo "FAIL: cannot read a document at $commit" >&2
    exit 2
  }
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
glossary=$(git show "$commit:docs/translation_glossary.md") || {
  echo "FAIL: cannot read the glossary at $commit" >&2
  exit 2
}
actual_glossary=$(
  for word in "${words[@]}"; do
    if grep -qF -- "$word" <<<"$glossary"; then
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
