#!/bin/sh
# One-off diff verification for task 0079 (AC-04, AC-11, AC-12, the removed
# names, and the ADR-0013 discrepancy statement).
# Usage: check.sh <base-commit> <head-commit>
set -eu
[ $# -eq 2 ] || { echo "usage: $0 <base> <head>" >&2; exit 2; }
base=$(git rev-parse --verify "$1^{commit}")
head=$(git rev-parse --verify "$2^{commit}")
types=internal/codexreview/types.go
state=internal/codexreview/state.go
proc=.claude/commands/_lib/local-cli-review.md
adr=docs/dev/adr/0013-review-setup.md
echo "base $base"
echo "head $head"
echo "$0 blob $(git hash-object "$0")"
# Pathspecs below are relative to the root.
cd "$(git rev-parse --show-toplevel)"
fail() { echo "FAIL $*"; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for rev in base head; do
	eval "c=\$$rev"
	git show "$c:$types" >"$tmp/$rev.types"
	git show "$c:$state" >"$tmp/$rev.state"
	git show "$c:$proc" >"$tmp/$rev.proc"
	git show "$c:$adr" >"$tmp/$rev.adr"
done

# AC-11: the Protocol definition line.
grep -x 'const Protocol = [0-9]*' "$tmp/base.types" >"$tmp/base.protocol" || fail "protocol: base has no Protocol line"
grep -x 'const Protocol = [0-9]*' "$tmp/head.types" >"$tmp/head.protocol" || fail "protocol: head has no Protocol line"
cmp -s "$tmp/base.protocol" "$tmp/head.protocol" || fail "protocol: the Protocol line differs"
echo "OK   protocol: $(cat "$tmp/head.protocol") in both"

# AC-11: the JSON names of Result's fields, in order.
jsonnames() { awk '/^type Result struct \{$/{body=1; next} body && /^\}$/{exit} body' "$1" | sed -n 's/.*`json:"\([^"]*\)".*/\1/p'; }
jsonnames "$tmp/base.types" >"$tmp/base.json"
jsonnames "$tmp/head.types" >"$tmp/head.json"
[ -s "$tmp/head.json" ] || fail "result-names: no JSON names read"
cmp -s "$tmp/base.json" "$tmp/head.json" || fail "result-names: the JSON names of Result differ"
echo "OK   result-names: $(tr '\n' ' ' <"$tmp/head.json")in both"

# AC-04: the git status arguments (base declares them in a var block, head
# alone, so only the value is compared).
statusargs() { sed -n 's/^.*statusArgs *= *\(\[\]string{.*}\)$/\1/p' "$1"; }
statusargs "$tmp/base.state" >"$tmp/base.status"
statusargs "$tmp/head.state" >"$tmp/head.status"
[ "$(wc -l <"$tmp/head.status")" -eq 1 ] || fail "status-args: head has no single statusArgs value"
cmp -s "$tmp/base.status" "$tmp/head.status" || fail "status-args: the statusArgs value differs"
echo "OK   status-args: the statusArgs value is the same"

# AC-12: join the stop bullet of step 3 into one line, cut it at the
# rewritten sentence, and require the rest of the file to be the same and
# the cut part to differ.
cutstop() {
	awk -v cut="$2" '
		/^   - \*\*`stop`\*\*/ { inb = 1; found = 1; para = $0; next }
		inb && /^$/ { inb = 0; i = index(para, "A `state_changed` caused by")
			if (i == 0) { bad = 1; exit }
			print substr(para, 1, i - 1); print substr(para, i) >cut; print; next }
		inb { sub(/^ +/, ""); para = para " " $0; next }
		{ print }
		END { if (bad || !found) exit 1 }
	' "$1" >"$3" || return 1
	[ -s "$2" ]
}
cutstop "$tmp/base.proc" "$tmp/base.cut" "$tmp/base.rest" || fail "procedure: base stop bullet not recognized"
cutstop "$tmp/head.proc" "$tmp/head.cut" "$tmp/head.rest" || fail "procedure: head stop bullet not recognized"
cmp -s "$tmp/base.rest" "$tmp/head.rest" || fail "procedure: text outside the rewritten sentence differs"
! cmp -s "$tmp/base.cut" "$tmp/head.cut" || fail "procedure: the rewritten sentence did not change"
echo "OK   procedure: only the text from \"A \`state_changed\` caused by\" to the end of the stop bullet differs"

# The ADR-0013 discrepancy statement: removing its three sentences from the
# base gives the head.
stmt='この決定は、まだ `codexreview` と共通手順に反映されていない。今の `codexreview` は、ローカルのすべてのブランチとタグ、stash の一覧も比べる。このずれは [#778](https://github.com/isseis/yt2column/issues/778) で扱う。'
STMT=$stmt awk 'BEGIN { s = ENVIRON["STMT"] } { i = index($0, s); if (i) { n++; $0 = substr($0, 1, i - 1) substr($0, i + length(s)) } print } END { if (n != 1) exit 1 }' "$tmp/base.adr" >"$tmp/base.adr.removed" || fail "adr: the statement is not in exactly one base line"
cmp -s "$tmp/base.adr.removed" "$tmp/head.adr" || fail "adr: the head is not the base without the statement"
echo "OK   adr: the head is the base without the three sentences"

# The removed names: none is left in the package, and gitState has no refs
# or stash field.
if git grep -qwE 'itemStash|refsArgs|stashArgs|errRefsUnreadable|parseRefs' "$head" -- internal/codexreview; then
	fail "removed-names: a removed name is still in internal/codexreview"
fi
awk '/^type gitState struct \{$/{body=1; next} body && /^\}$/{exit} body' "$tmp/head.state" >"$tmp/head.gitstate"
[ -s "$tmp/head.gitstate" ] || fail "removed-names: head has no gitState body"
# Field names are the words before the type; comments are dropped first.
if sed 's://.*$::' "$tmp/head.gitstate" | grep -qwE 'refs|stash'; then
	fail "removed-names: gitState still has a refs or stash field"
fi
echo "OK   removed-names: itemStash, refsArgs, stashArgs, errRefsUnreadable, parseRefs, and the refs and stash fields are gone"
echo "OK every check passed"
