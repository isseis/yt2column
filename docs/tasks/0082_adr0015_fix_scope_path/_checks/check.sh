#!/usr/bin/env bash
# Verify the ADR-0015 change of task 0082 between two commits.
# Usage: check.sh <before-commit> <after-commit>
# Run from inside the repository. Exits 0 when every check passes.
set -u
export LC_ALL=C

adr=docs/dev/adr/0015-untrusted-review-threads.md
taskdir=docs/tasks/0082_adr0015_fix_scope_path
link='(../../tasks/0082_adr0015_fix_scope_path/01_requirements.md'

[ $# -eq 2 ] || { echo "usage: $0 <before> <after>" >&2; exit 2; }
before=$(git rev-parse --verify "$1^{commit}") || exit 2
after=$(git rev-parse --verify "$2^{commit}") || exit 2

tmp=$(mktemp -d) || exit 2
trap 'rm -rf "$tmp"' EXIT

failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

# --- AC-06: only the ADR and the task directory change.
names=$(git diff --no-renames --name-status "$before" "$after") || exit 2
while IFS=$'\t' read -r status path; do
	case "$path" in
	"$adr" | "$taskdir"/*) ;;
	*) fail "AC-06: unexpected change ($status) outside the ADR and the task directory: $path" ;;
	esac
done <<<"$names"

# --- Read the ADR at both commits.
git show "$before:$adr" >"$tmp/old.md" || exit 2
git show "$after:$adr" >"$tmp/new.md" || exit 2
git diff -U0 --no-color "$before" "$after" -- "$adr" | grep -E '^@@' >"$tmp/hunks"

# --- AC-05: exactly three hunks.
nhunks=$(wc -l <"$tmp/hunks" | tr -d ' ')
[ "$nhunks" = 3 ] || fail "AC-05: expected 3 hunks in the ADR, found $nhunks"

# Section of a line number in the new ADR, from its "## " headings.
section_of() {
	awk -v n="$1" '/^## / { sec = $0 } NR == n { print sec; exit }' "$tmp/new.md"
}
line_of() { sed -n "${1}p" "$tmp/new.md"; }
total=$(wc -l <"$tmp/new.md" | tr -d ' ')

# Parse a hunk header into: old_start old_count new_start new_count.
parse_hunk() {
	local h=$1 o n
	o=${h#@@ -}
	o=${o%% +*}
	n=${h#*+}
	n=${n%% @@*}
	local os=${o%%,*} oc=1 ns=${n%%,*} nc=1
	case "$o" in *,*) oc=${o#*,} ;; esac
	case "$n" in *,*) nc=${n#*,} ;; esac
	echo "$os $oc $ns $nc"
}

if [ "$nhunks" = 3 ]; then
	read -r os1 oc1 ns1 nc1 < <(parse_hunk "$(sed -n 1p "$tmp/hunks")")
	read -r os2 oc2 ns2 nc2 < <(parse_hunk "$(sed -n 2p "$tmp/hunks")")
	read -r os3 oc3 ns3 nc3 < <(parse_hunk "$(sed -n 3p "$tmp/hunks")")

	# Hunk 1: one line rewritten inside the decision section.
	[ "$oc1" = 1 ] && [ "$nc1" = 1 ] || fail "AC-02: hunk 1 is not a one-line rewrite ($oc1 -> $nc1 lines)"
	case "$(section_of "$ns1")" in
	"## 決定"*) ;;
	*) fail "AC-02: hunk 1 is not in the decision section" ;;
	esac

	# Hunk 2: one line added at the end of the rejected-alternatives section.
	[ "$oc2" = 0 ] && [ "$nc2" = 1 ] || fail "AC-03: hunk 2 is not a one-line addition ($oc2 -> $nc2 lines)"
	case "$(section_of "$ns2")" in
	"## 採らなかった案"*) ;;
	*) fail "AC-03: hunk 2 is not in the rejected-alternatives section" ;;
	esac
	next=$((ns2 + 1))
	if [ -n "$(line_of "$next")" ]; then
		fail "AC-03: the line after the added item is not blank"
	else
		case "$(section_of "$((next + 1))")" in
		"## 採らなかった案"*) fail "AC-03: the added item is not the last item of its section" ;;
		esac
	fi
	line_of "$ns2" | grep -qE '^-   \*\*' || fail "AC-03: the added item is not a bold list item"
	line_of "$ns2" | grep -qF -- "$link" || fail "AC-03: the added item does not link to the task requirements"

	# Hunk 3: one line added at the end of the history section (the end of the file).
	[ "$oc3" = 0 ] && [ "$nc3" = 1 ] || fail "AC-04: hunk 3 is not a one-line addition ($oc3 -> $nc3 lines)"
	case "$(section_of "$ns3")" in
	"## 経緯"*) ;;
	*) fail "AC-04: hunk 3 is not in the history section" ;;
	esac
	last=$total
	while [ "$last" -gt 0 ] && [ -z "$(line_of "$last")" ]; do last=$((last - 1)); done
	[ "$ns3" = "$last" ] || fail "AC-04: the added item (line $ns3) is not the last item (line $last)"
	line_of "$ns3" | grep -qE '^- [0-9]{4}-[0-9]{2}-[0-9]{2} ' || fail "AC-04: the added item does not start with a date"
	line_of "$ns3" | grep -qF -- "$link" || fail "AC-04: the added item does not link to the task requirements"

	# AC-02: inside the rewritten line, only the leading bold text changes and
	# one text is inserted at a single position.
	sed -n "${os1}p" "$tmp/old.md" >"$tmp/old_line"
	line_of "$ns1" >"$tmp/new_line"
	bold_re='^(-   \*\*[^*]*\*\*)(.*)$'
	if [[ "$(cat "$tmp/old_line")" =~ $bold_re ]] && old_bold=${BASH_REMATCH[1]} && old_rest=${BASH_REMATCH[2]} &&
		[[ "$(cat "$tmp/new_line")" =~ $bold_re ]] && new_bold=${BASH_REMATCH[1]} && new_rest=${BASH_REMATCH[2]}; then
		[ "$old_bold" != "$new_bold" ] || fail "AC-02: the leading bold text was not rewritten"
		d=$((${#new_rest} - ${#old_rest}))
		printf '%s' "$old_rest" >"$tmp/old_rest"
		printf '%s' "$new_rest" >"$tmp/new_rest"
		if [ "$d" -le 0 ]; then
			fail "AC-02: nothing was inserted after the leading bold text"
		else
			# First differing byte gives the insertion point k.
			k=$(cmp "$tmp/old_rest" "$tmp/new_rest" 2>/dev/null | sed -nE 's/.* (byte|char) ([0-9]+).*/\2/p')
			if [ -z "$k" ]; then
				k=${#old_rest}
			else
				k=$((k - 1))
			fi
			case "$k" in
			'' | *[!0-9]*) fail "AC-02: could not locate the insertion point"; k=0 ;;
			esac
			if [ "${new_rest:0:k}" = "${old_rest:0:k}" ] && [ "${new_rest:$((k + d))}" = "${old_rest:k}" ]; then
				echo "info: AC-02: $d bytes inserted at one position (byte $k of the text after the bold part)"
			else
				fail "AC-02: the rest of the line is not the old text with one insertion"
			fi
		fi
	else
		fail "AC-02: the rewritten line does not start with a bold text"
	fi
fi

echo "before: $before"
echo "after:  $after"
echo "check.sh blob: $(git hash-object -- "$0")"
if [ "$failures" -ne 0 ]; then
	echo "RESULT: $failures check(s) failed"
	exit 1
fi
echo "RESULT: all checks passed"
