#!/bin/sh
# One-off diff verification for task 0078 (AC-01 link, AC-02, AC-03, AC-06 form).
# Usage: check.sh <base-commit> <head-commit>
# The ADR's last level-2 section is the history; the item two lines above its
# heading is the last consequence item. The check replaces that item in both
# versions, drops the head's last line, and requires the rest to be identical.
set -eu
[ $# -eq 2 ] || { echo "usage: $0 <base> <head>" >&2; exit 2; }
base=$(git rev-parse --verify "$1^{commit}")
head=$(git rev-parse --verify "$2^{commit}")
adr=docs/dev/adr/0024-env-access-in-config.md
task=docs/tasks/0078_adr0024_server_env_result/
link='](../../tasks/0078_adr0024_server_env_result/01_requirements.md)'
echo "base $base"
echo "head $head"
echo "$0 blob $(git hash-object "$0")"
fail() { echo "FAIL $*"; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

outside=$(git diff --no-renames --name-status "$base" "$head" | grep -v "	$task" || true)
[ "$outside" = "M	$adr" ] || fail "scope: changes outside the task directory: $outside"
echo "OK   scope: only $adr changed outside the task directory"

git show "$base:$adr" >"$tmp/base"
git show "$head:$adr" >"$tmp/head"
# Print the line number of the last consequence item, or fail.
item() { awk '/^## /{h=NR} {l[NR]=$0} END{if (h<3 || l[h-1]!="" || substr(l[h-2],1,4)!="-   ") exit 1; print h-2}' "$1"; }
nb=$(item "$tmp/base") || fail "consequence: base layout not recognized"
nh=$(item "$tmp/head") || fail "consequence: head layout not recognized"
sed "${nb}d" "$tmp/base" >"$tmp/base.rest"
sed -e "${nh}d" -e '$d' "$tmp/head" >"$tmp/head.rest"
cmp -s "$tmp/base.rest" "$tmp/head.rest" || fail "consequence: lines other than the last consequence item and the last line differ"
echo "OK   consequence: only the last consequence item and the appended last line differ"

new=$(sed -n "${nh}p" "$tmp/head")
[ "$new" != "$(sed -n "${nb}p" "$tmp/base")" ] || fail "consequence-link: the last consequence item did not change"
case $new in *"](0018-server-config-file.md)"*) ;; *) fail "consequence-link: no link to 0018-server-config-file.md" ;; esac
echo "OK   consequence-link: the rewritten item links to 0018-server-config-file.md"

last=$(tail -n 1 "$tmp/head")
echo "$last" | grep -Eq '^- [0-9]{4}-[0-9]{2}-[0-9]{2} ' || fail "history-entry: the appended line is not a dated history item"
case $last in *"$link"*) ;; *) fail "history-entry: the appended item does not link to the requirements" ;; esac
echo "OK   history-entry: one dated history item appended, linking to the requirements"
echo "OK every check passed"
