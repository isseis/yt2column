#!/bin/sh
# Checks that a branch of task 0057 changes four task documents of tasks 0030
# and 0036 only by adding one note per section listed below, and changes
# nothing else in them: no line is removed, every added line belongs to a note
# block that starts with "> **注記（タスク 0057）:**" (the other lines of the
# block start with ">", and an added empty line is allowed), the Document
# Status table and the change-history section (when there is one) are
# byte-identical, no file's mode changes, and each listed section gains exactly
# one note before its body.
#
# Usage:
#
#	docs/tasks/0057_reviewer_relay_guard/_checks/notes_only.sh <upstream> <head>
#
# The merge-base is that of <upstream> and <head>. On a PR branch of the task,
# after git fetch origin main:
#
#	docs/tasks/0057_reviewer_relay_guard/_checks/notes_only.sh origin/main HEAD
#
# Output and exit codes:
#   0  the four documents changed only as described (the merge-base and the
#      head on stdout)
#   1  a check failed (the merge-base and the head on stdout, the reason on
#      stderr)
#   2  the arguments or the repository are unusable (not two arguments, a
#      value that is not a commit, no repository, no docs/tasks/ directory, a
#      temporary directory that cannot be made, or a git command that fails)
#
# Run it once on the PR-3 branch of the task, before its PR checkpoint. make
# test and CI do not run it. It reads no environment variable and names no
# secret.
set -eu

task=docs/tasks/0057_reviewer_relay_guard
note='> **注記（タスク 0057）:**'

if [ "$#" -ne 2 ]; then
	echo "usage: $0 <upstream> <head>" >&2
	exit 2
fi

if ! root=$(git rev-parse --show-toplevel 2>/dev/null); then
	echo "not a git repository" >&2
	exit 2
fi
if [ ! -d "$root/docs/tasks" ]; then
	echo "no docs/tasks/ directory under $root" >&2
	exit 2
fi

for ref in "$1" "$2"; do
	if ! git -C "$root" cat-file -e "$ref^{commit}" 2>/dev/null; then
		echo "not a commit: $ref" >&2
		exit 2
	fi
done
if ! head=$(git -C "$root" rev-parse --verify "$2^{commit}"); then
	echo "cannot resolve $2" >&2
	exit 2
fi
if ! base=$(git -C "$root" merge-base "$1" "$head"); then
	echo "no merge-base of $1 and $2" >&2
	exit 2
fi
echo "merge-base: $base"
echo "head: $head"

if ! tmp=$(mktemp -d); then
	echo "cannot make a temporary directory" >&2
	exit 2
fi
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

# The documents, and the number of notes each gains. The fields are separated
# by a tab.
documents='
docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md	2
docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md	3
docs/tasks/0036_fixprguard_exec_file_scope/01_requirements.md	4
docs/tasks/0036_fixprguard_exec_file_scope/02_architecture.md	3
'
# The sections that gain one note, with the same separator.
sections='
docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md	## 1. 概要 (Overview)
docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md	#### F-004: 変更範囲と作業ツリーの確認
docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md	### 3.6. 変更範囲の確認と commit・push（F-004・H-05・H-10）
docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md	### 5.1. 脅威モデル
docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md	### 5.3. 残余のリスク
docs/tasks/0036_fixprguard_exec_file_scope/01_requirements.md	## 1. 概要 (Overview)
docs/tasks/0036_fixprguard_exec_file_scope/01_requirements.md	#### F-001: 実行関連ファイルの定義を広げる
docs/tasks/0036_fixprguard_exec_file_scope/01_requirements.md	#### F-003: issue の表の各行を範囲外にする
docs/tasks/0036_fixprguard_exec_file_scope/01_requirements.md	## 6. 用語集 (Glossary)
docs/tasks/0036_fixprguard_exec_file_scope/02_architecture.md	### 3.3. 判定の順序（H-02・H-06）
docs/tasks/0036_fixprguard_exec_file_scope/02_architecture.md	### 3.7. 用語集と 0030 の文書（AC-18・AC-21）
docs/tasks/0036_fixprguard_exec_file_scope/02_architecture.md	### 7.1. ユニットテスト（`scope_test.go`）
'

# Writes a revision of a document to a file; a git failure is unusable input.
show_file() {
	if ! git -C "$root" show "$1:$2" >"$3" 2>/dev/null; then
		echo "git show failed: $2" >&2
		exit 2
	fi
}

# Prints one section of the document in a file: the heading line and the lines
# after it, up to the next heading of the same or a higher level.
extract_section() {
	awk -v heading="$2" '
		function level(s,   n, i, c) {
			n = 0
			for (i = 1; i <= length(s); i++) {
				c = substr(s, i, 1)
				if (c == "#") n++
				else break
			}
			return n
		}
		!started {
			if ($0 == heading) {
				started = 1
				lvl = level($0)
				print
			}
			next
		}
		substr($0, 1, 1) == "#" {
			if (level($0) <= lvl) exit
		}
		{ print }
	' "$1"
}

extract_status() {
	awk '
		/^## Document Status$/ { found = 1; print; next }
		found && /^## / { exit }
		found { print }
	' "$1"
}

extract_history() {
	awk '
		/^## 変更履歴$/ { found = 1; print; next }
		found && /^## / { exit }
		found { print }
	' "$1"
}

ret=0
while IFS='	' read -r doc expected; do
	[ -n "$doc" ] || continue
	show_file "$base" "$doc" "$tmp/base-doc"
	show_file "$head" "$doc" "$tmp/head-doc"
	# (1) the document changed between the merge-base and the head.
	if ! git -C "$root" diff --no-renames --name-only "$base" "$head" -- "$doc" >"$tmp/names"; then
		echo "git diff failed: $doc" >&2
		exit 2
	fi
	if [ ! -s "$tmp/names" ]; then
		echo "$doc did not change" >&2
		ret=1
		continue
	fi
	# The base holds no note of this task, so a note in the head is an addition.
	if [ "$(grep -cF -- "$note" "$tmp/base-doc" || true)" -ne 0 ]; then
		echo "$doc already holds a task 0057 note in the merge-base" >&2
		ret=1
		continue
	fi
	# No file mode changes.
	if ! git -C "$root" diff --no-renames --summary "$base" "$head" -- "$doc" >"$tmp/summary"; then
		echo "git diff failed: $doc" >&2
		exit 2
	fi
	if [ -s "$tmp/summary" ]; then
		echo "$doc changed its file mode" >&2
		ret=1
		continue
	fi
	# (2) no removed line, and every added line belongs to a note block that
	# starts with the note marker.
	if ! git -C "$root" diff --no-renames --unified=0 --no-color "$base" "$head" -- "$doc" >"$tmp/diff"; then
		echo "git diff failed: $doc" >&2
		exit 2
	fi
	if ! awk -v marker="$note" '
		/^@@/ { hunk = 1; first = 1; next }
		!hunk { next }
		/^\\/ { next }
		/^-/ { print "removed: " $0; bad = 1; next }
		/^\+/ {
			line = substr($0, 2)
			if (line == "") next
			if (first) {
				if (index(line, marker) != 1) {
					print "added block does not start with the note: " $0
					bad = 1
				}
				first = 0
			} else if (substr(line, 1, 1) != ">") {
				print "added without >: " $0
				bad = 1
			}
		}
		END { exit bad ? 1 : 0 }
	' "$tmp/diff" >&2; then
		echo "$doc removes a line or adds a line outside a note block" >&2
		ret=1
		continue
	fi
	# (3) the Document Status table and the change-history section are
	# byte-identical.
	extract_status "$tmp/base-doc" >"$tmp/status-base"
	extract_status "$tmp/head-doc" >"$tmp/status-head"
	if ! cmp -s "$tmp/status-base" "$tmp/status-head"; then
		echo "$doc changed its Document Status table" >&2
		ret=1
		continue
	fi
	extract_history "$tmp/base-doc" >"$tmp/history-base"
	extract_history "$tmp/head-doc" >"$tmp/history-head"
	if ! cmp -s "$tmp/history-base" "$tmp/history-head"; then
		echo "$doc changed its change history" >&2
		ret=1
		continue
	fi
	# (4) every note in the document is in a listed section, exactly one per
	# section.
	if [ "$(grep -cF -- "$note" "$tmp/head-doc" || true)" -ne "$expected" ]; then
		echo "$doc does not hold exactly $expected task 0057 notes" >&2
		ret=1
		continue
	fi
done <<EOF
$documents
EOF

while IFS='	' read -r doc heading; do
	[ -n "$doc" ] || continue
	show_file "$head" "$doc" "$tmp/section-doc"
	count=$(extract_section "$tmp/section-doc" "$heading" | grep -cF -- "$note" || true)
	if [ "$count" -ne 1 ]; then
		echo "$doc: section \"$heading\" holds $count task 0057 notes, not 1" >&2
		ret=1
	fi
	if ! extract_section "$tmp/section-doc" "$heading" | awk -v marker="$note" '
		NR > 1 && !body {
			if ($0 == "") next
			if (index($0, marker) == 1) { seen = 1; next }
			if (substr($0, 1, 1) != ">") body = 1
		}
		END { exit seen ? 0 : 1 }
	'; then
		echo "$doc: the note of section \"$heading\" is missing or comes after the body" >&2
		ret=1
	fi
done <<EOF
$sections
EOF

if [ "$ret" -ne 0 ]; then
	exit 1
fi

echo "the four documents add one task 0057 note per listed section, before the body, and change nothing else"
