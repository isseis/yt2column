#!/bin/sh
# One-off diff verification for task 0090: which files a change touches, and
# whether the ADR list in issue #773 matches the ADRs that still record a
# discrepancy to the code.
#
# Usage: check.sh <before-commit> <after-commit> [issue-body-file]
#
# Run from the top level of the work tree. It exits 0 when every check holds,
# 1 when a check fails (each failure is named), and 2 on a usage or git error.
set -u

TASK_PREFIX=docs/tasks/0090_extension_only_architecture/

die() {
	printf 'check.sh: %s\n' "$1" >&2
	exit 2
}

if [ "$#" -ne 2 ] && [ "$#" -ne 3 ]; then
	die "usage: check.sh <before-commit> <after-commit> [issue-body-file]"
fi

before=$(git rev-parse --verify --quiet "$1^{commit}") ||
	die "not a commit: $1"
after=$(git rev-parse --verify --quiet "$2^{commit}") ||
	die "not a commit: $2"

tmp=$(mktemp -d) || die "cannot create a temporary directory"
trap 'rm -rf "$tmp"' EXIT

status=0

printf 'before commit: %s\n' "$before"
printf 'after commit: %s\n' "$after"
printf 'script blob: %s\n' "$(git hash-object -- "$0")"

# Check 1: the change stays inside the four locations task 0090 is limited to.

env -u GIT_DIFF_OPTS git --no-pager -c core.quotePath=true \
	diff --no-ext-diff --no-relative --no-renames -O/dev/null --name-only \
	"$before" "$after" >"$tmp/changed" || die "cannot list the changed files"

printf '\nchanged files:\n'
sed 's/^/  /' "$tmp/changed"

printf '\nchanged files outside %s, docs/dev/adr/, docs/translation_glossary.md, docs/research/:\n' "$TASK_PREFIX"
: >"$tmp/outside"
while IFS= read -r changed; do
	[ -n "$changed" ] || continue
	case "$changed" in
	"$TASK_PREFIX"* | docs/dev/adr/* | docs/translation_glossary.md | docs/research/*) ;;
	*) printf '%s\n' "$changed" >>"$tmp/outside" ;;
	esac
done <"$tmp/changed"
if [ -s "$tmp/outside" ]; then
	sed 's/^/  /' "$tmp/outside"
	status=1
else
	printf '  (none)\n'
fi

# Check 2: the issue body names the active ADRs whose decision section still
# links to #773. Only run when the body file is given.

if [ "$#" -eq 3 ]; then
	body=$3
	[ -f "$body" ] || die "no such issue body file: $body"
	printf 'issue body blob: %s\n' "$(git hash-object -- "$body")"

	: >"$tmp/adr-set"
	git ls-tree -r --name-only "$after" -- docs/dev/adr/ >"$tmp/adr-files" ||
		die "cannot list the ADR files at $after"
	while IFS= read -r adr; do
		case "$adr" in
		docs/dev/adr/[0-9][0-9][0-9][0-9]-*.md) ;;
		*) continue ;;
		esac
		git show "$after:$adr" >"$tmp/adr" ||
			die "cannot read $adr at $after"
		# A retired ADR has no decision in effect.
		if ! awk '
			$0 == "## 状態" { inside = 1; next }
			inside && /^## / { exit }
			inside { print }
		' "$tmp/adr" | grep -Eq '^\| 状態 \| `有効` \|$'; then
			continue
		fi
		# A link whose number merely starts with 773 is a different issue.
		if ! awk '
			$0 == "## 決定" { inside = 1; next }
			inside && /^## / { exit }
			inside { print }
		' "$tmp/adr" | grep -Eq 'issues/773([^0-9]|$)'; then
			continue
		fi
		number=${adr#docs/dev/adr/}
		number=${number%%-*}
		printf 'ADR-%s\n' "$number" >>"$tmp/adr-set"
	done <"$tmp/adr-files"
	sort -u -o "$tmp/adr-set" "$tmp/adr-set"

	awk '
		/^#+ / {
			if (inside) exit
			if ($0 ~ /食い違いの記述を置いた/) { inside = 1; next }
		}
		inside { print }
	' "$body" >"$tmp/body-section"
	grep -oE 'ADR-[0-9]{4}' "$tmp/body-section" | sort -u >"$tmp/body-set"

	printf '\nADRs active at %s that link to issue 773 in their decision section:\n' "$after"
	sed 's/^/  /' "$tmp/adr-set"
	printf '\nADRs the issue body names in its discrepancy section:\n'
	sed 's/^/  /' "$tmp/body-set"
	printf '\nADRs only in the ADR files:\n'
	comm -23 "$tmp/adr-set" "$tmp/body-set" | sed 's/^/  /'
	printf '\nADRs only in the issue body:\n'
	comm -13 "$tmp/adr-set" "$tmp/body-set" | sed 's/^/  /'
	cmp -s "$tmp/adr-set" "$tmp/body-set" || status=1
fi

if [ "$status" -eq 0 ]; then
	printf '\nresult: ok\n'
else
	printf '\nresult: FAILED\n'
fi
exit "$status"
