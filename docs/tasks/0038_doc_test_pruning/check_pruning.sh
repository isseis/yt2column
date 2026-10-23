#!/bin/sh
# Static checks for task 0038: names the task deletes or keeps, the documents
# it must leave unchanged, the task records it rewrites, a survey of the tests
# that read documents, and a coverage comparison across two revisions.
#
# Usage:
#   sh docs/tasks/0038_doc_test_pruning/check_pruning.sh removed <PR> <base>
#   sh docs/tasks/0038_doc_test_pruning/check_pruning.sh kept
#   sh docs/tasks/0038_doc_test_pruning/check_pruning.sh unchanged <base>
#   sh docs/tasks/0038_doc_test_pruning/check_pruning.sh records <base>
#   sh docs/tasks/0038_doc_test_pruning/check_pruning.sh survey
#   sh docs/tasks/0038_doc_test_pruning/check_pruning.sh coverage <before> <after>
#
# It checks the repository that holds the script, whatever the current
# directory is. The exit status is 0 when the check passes, 1 when it fails,
# and 2 on any other error (a usage error, an unknown revision, a failed git
# command).
#
# CHECK_REV=<revision> reads the files of that revision's tree instead of the
# working tree; it does not change the two revisions coverage takes.
#
# pruned_names.txt holds one tab-separated row per name. A "removed <PR> <base>"
# run checks the delete rows whose PR is at or before <PR>: the name is gone
# from its file, and for a row marked outside=yes it is gone from the documents
# outside docs/tasks/. For the rows of <PR> itself it also checks that the name
# is declared at the merge base of <base>, so that a row whose name, kind, or
# file is wrong fails instead of passing because nothing matched. A "kept" run
# checks that every keep row is declared.
# Text read from the files is only matched (grep -F / grep -E); it is never
# run as a shell command.

set -u

die() {
	printf 'check_pruning.sh: %s\n' "$1" >&2
	exit 2
}

usage() {
	sed -n '5,13p' "$0" >&2
}

here=$(cd "$(dirname "$0")" 2>/dev/null && pwd) || die 'cannot resolve the script directory'
top=$(cd "$here" 2>/dev/null && git rev-parse --show-toplevel 2>/dev/null) ||
	die 'not inside a git repository'
cd "$top" || die "cannot enter $top"
names="$here/pruned_names.txt"
[ -f "$names" ] || die "cannot read $names"

if [ -n "${CHECK_REV:-}" ]; then
	git rev-parse --verify --quiet "${CHECK_REV}^{commit}" >/dev/null ||
		die "unknown revision: $CHECK_REV"
fi

tmp=$(mktemp -d) || die 'cannot create a temporary directory'
cleanup_tmp() { rm -rf "$tmp"; }
trap cleanup_tmp EXIT
trap 'exit 2' HUP INT TERM

# show_file FILE: print FILE from CHECK_REV's tree or the working tree. The
# status is 0 when it exists, 1 when it does not, and 2 when git fails.
show_file() {
	if [ -n "${CHECK_REV:-}" ]; then
		git rev-parse --verify --quiet "${CHECK_REV}:$1" >/dev/null
		case $? in
		0) git show "${CHECK_REV}:$1" 2>/dev/null || return 2 ;;
		1) return 1 ;;
		*) return 2 ;;
		esac
	elif [ -f "$1" ]; then
		cat -- "$1" || return 2
	else
		return 1
	fi
}

# decl_present NAME KIND FILE: succeed when the declaration is present.
#
# The match is a line regex, not a parse, and is known to be loose. Every row
# in pruned_names.txt is matched correctly; a new row must be checked by hand
# against these limits:
#   - func matches only a plain function ("func name("); a method
#     ("func (r T) name(") or a generic function ("func name[T any](") is
#     never found.
#   - var and const also match any indented "name =" line, including an
#     assignment inside a function body, and miss a typed spec in a grouped
#     block ("\tname T = ...") or a spec without a value.
# A miss makes removed pass and kept fail; a false match does the opposite.
decl_present() {
	name=$1
	kind=$2
	file=$3
	content=$(show_file "$file")
	rc=$?
	case $rc in
	0) ;;
	1) return 1 ;;
	*) die "cannot read $file" ;;
	esac
	case $kind in
	func)
		printf '%s\n' "$content" |
			LC_ALL=C grep -E -- "^func[[:space:]]+$name\\(" >/dev/null
		;;
	var | const)
		printf '%s\n' "$content" |
			LC_ALL=C grep -E -- "^(var|const)[[:space:]]+$name([[:space:]=])|^[[:space:]]+$name[[:space:]]*=" >/dev/null
		;;
	type)
		printf '%s\n' "$content" |
			LC_ALL=C grep -E -- "^type[[:space:]]+$name([[:space:]]|$)" >/dev/null
		;;
	node)
		found=1
		for q in '"' "'" '`'; do
			if printf '%s\n' "$content" |
				LC_ALL=C grep -F -- "it($q$name$q" >/dev/null; then
				found=0
				break
			fi
		done
		return "$found"
		;;
	subtest)
		printf '%s\n' "$content" |
			LC_ALL=C grep -F -- "t.Run(\"$name\"" >/dev/null
		;;
	text)
		printf '%s\n' "$content" | LC_ALL=C grep -F -- "$name" >/dev/null
		;;
	*)
		die "unknown kind in pruned_names.txt: $kind"
		;;
	esac
}

# outside_hit NAME: succeed when an outside document names it.
outside_hit() {
	if [ -n "${CHECK_REV:-}" ]; then
		LC_ALL=C git grep -qF -- "$1" "$CHECK_REV" -- \
			CLAUDE.md README.md AGENTS.md docs/dev .claude/commands
	else
		LC_ALL=C git grep -qF -- "$1" -- \
			CLAUDE.md README.md AGENTS.md docs/dev .claude/commands
	fi
}

read_names() {
	while IFS="$(printf '\t')" read -r r_name r_kind r_file r_outside r_pr r_action; do
		case $r_name in
		'' | '#'*) continue ;;
		esac
		if [ -z "$r_kind" ] || [ -z "$r_file" ] ||
			[ -z "$r_outside" ] || [ -z "$r_pr" ] || [ -z "$r_action" ]; then
			die "malformed row in pruned_names.txt: $r_name"
		fi
		case $r_outside in
		yes | no) ;;
		*) die "unknown outside value for $r_name: $r_outside" ;;
		esac
		case $r_action in
		delete | keep) ;;
		*) die "unknown action for $r_name: $r_action" ;;
		esac
		case $r_pr in
		'' | *[!0-9]*) die "unknown PR for $r_name: $r_pr" ;;
		esac
	done <"$names"
}

cmd_removed() {
	[ $# -eq 2 ] || die 'usage: removed <PR> <base>'
	limit=$1
	case $limit in
	'' | *[!0-9]*) die "invalid PR: $limit" ;;
	esac
	base=$(merge_base "$2") || exit 2
	read_names
	fail=0
	while IFS="$(printf '\t')" read -r r_name r_kind r_file r_outside r_pr r_action; do
		case $r_name in
		'' | '#'*) continue ;;
		esac
		[ "$r_action" = delete ] || continue
		[ "$r_pr" -le "$limit" ] || continue
		if [ "$r_pr" -eq "$limit" ]; then
			(CHECK_REV=$base decl_present "$r_name" "$r_kind" "$r_file")
			rc=$?
			case $rc in
			0) ;;
			1)
				printf 'FAIL removed %s: %s is not declared in %s at the base, so the row cannot show its deletion\n' \
					"$limit" "$r_name" "$r_file"
				fail=1
				;;
			*) die "cannot read $r_file at the base" ;;
			esac
		fi
		if decl_present "$r_name" "$r_kind" "$r_file"; then
			printf 'FAIL removed %s: %s is still declared in %s\n' \
				"$limit" "$r_name" "$r_file"
			fail=1
		fi
		if [ "$r_outside" = yes ]; then
			outside_hit "$r_name"
			rc=$?
			case $rc in
			0)
				printf 'FAIL removed %s: the documents outside docs/tasks/ still name %s\n' \
					"$limit" "$r_name"
				fail=1
				;;
			1) ;;
			*) die "cannot search the outside documents for $r_name" ;;
			esac
		fi
	done <"$names"
	if [ "$fail" -eq 0 ]; then
		printf 'ok   removed %s: no name the PR deletes is left\n' "$limit"
	fi
	return "$fail"
}

cmd_kept() {
	[ $# -eq 0 ] || die 'usage: kept'
	read_names
	fail=0
	while IFS="$(printf '\t')" read -r r_name r_kind r_file r_outside r_pr r_action; do
		case $r_name in
		'' | '#'*) continue ;;
		esac
		[ "$r_action" = keep ] || continue
		if ! decl_present "$r_name" "$r_kind" "$r_file"; then
			printf 'FAIL kept: %s is not declared in %s\n' "$r_name" "$r_file"
			fail=1
		fi
	done <"$names"
	if [ "$fail" -eq 0 ]; then
		echo 'ok   kept: every kept name is declared'
	fi
	return "$fail"
}

# docguard_text REV: print .github/workflows/ci.yml of REV, or of the working
# tree when REV is empty.
docguard_text() {
	if [ -n "$1" ]; then
		git show "$1:.github/workflows/ci.yml" 2>/dev/null || return 2
	else
		cat -- .github/workflows/ci.yml || return 2
	fi
}

# docguard_line REV: print the one doc-guard go test line of that revision.
docguard_line() {
	text=$(docguard_text "$1") || return 2
	printf '%s\n' "$text" |
		LC_ALL=C awk '
			/^  doc-guard:/ { on = 1 }
			on && /^  [A-Za-z0-9_-]+:/ && !/^  doc-guard:/ { exit }
			on { print }
		' |
		LC_ALL=C grep -E 'go test -tags test \./cmd/yt2column \./cmd/yt2column-server \./cmd/fixprguard \./internal/pipeline'
}

# merge_base BASE: print the merge base of BASE and the checked revision
# (CHECK_REV, or HEAD for the working tree), so that a BASE that moved on after
# the branch forked (another task merged into main) does not count that task's
# changes as this branch's.
merge_base() {
	b=$(git rev-parse --verify --quiet "$1^{commit}") ||
		die "unknown revision: $1"
	git merge-base "$b" "${CHECK_REV:-HEAD}" 2>/dev/null ||
		die "no merge base between $1 and ${CHECK_REV:-HEAD}"
}

cmd_unchanged() {
	[ $# -eq 1 ] || die 'usage: unchanged <base>'
	base=$(merge_base "$1") || exit 2
	fail=0
	if [ -n "${CHECK_REV:-}" ]; then
		changed=$(git diff --name-only "$base" "$CHECK_REV" -- \
			docs/tasks/0029_transcript_panel_read)
	else
		changed=$(git diff --name-only "$base" -- \
			docs/tasks/0029_transcript_panel_read)
	fi
	rc=$?
	[ "$rc" -le 1 ] || die 'cannot compare the withdrawn task documents'
	if [ -n "$changed" ]; then
		printf 'FAIL unchanged: the withdrawn task 0029 documents changed:\n%s\n' "$changed"
		fail=1
	fi

	docguard_line "$base" >"$tmp/base-ci"
	rc=$?
	[ "$rc" -le 1 ] || die 'cannot read the base .github/workflows/ci.yml'
	if [ -z "${CHECK_REV:-}" ]; then
		docguard_line '' >"$tmp/target-ci"
	else
		docguard_line "$CHECK_REV" >"$tmp/target-ci"
	fi
	rc=$?
	[ "$rc" -le 1 ] || die 'cannot read the target .github/workflows/ci.yml'
	base_count=$(LC_ALL=C grep -c . "$tmp/base-ci")
	target_count=$(LC_ALL=C grep -c . "$tmp/target-ci")
	if [ "$base_count" -ne 1 ] || [ "$target_count" -ne 1 ]; then
		echo 'FAIL unchanged: cannot find exactly one doc-guard go test line'
		fail=1
	elif ! cmp -s "$tmp/base-ci" "$tmp/target-ci"; then
		echo 'FAIL unchanged: the doc-guard go test line changed'
		diff -u "$tmp/base-ci" "$tmp/target-ci"
		fail=1
	fi
	if [ "$fail" -eq 0 ]; then
		echo 'ok   unchanged: task 0029 and the doc-guard command are unchanged'
	fi
	return "$fail"
}

# section_body FILE START_RE END_RE: print the lines between the two markers.
section_body() {
	show_file "$1" | awk -v start="$2" -v stop="$3" '
		$0 ~ start { on = 1; next }
		on && $0 ~ stop { exit }
		on { print }
	'
}

cmd_records() {
	[ $# -eq 1 ] || die 'usage: records <base>'
	base=$(merge_base "$1") || exit 2
	if [ -n "${CHECK_REV:-}" ]; then
		changed=$(git diff --name-only "$base" "$CHECK_REV" -- docs/tasks)
	else
		changed=$(git diff --name-only "$base" -- docs/tasks)
	fi
	rc=$?
	[ "$rc" -le 1 ] || die 'cannot compare the task documents'
	printf '%s\n' "$changed" |
		LC_ALL=C grep -E '^docs/tasks/[^/]+/(02_architecture|03_implementation_plan)\.md$' |
		LC_ALL=C grep -v -E '^docs/tasks/(0038|0029)_' >"$tmp/changed" || true

	fail=0
	while IFS= read -r f; do
		[ -n "$f" ] || continue
		status=$(show_file "$f" | LC_ALL=C grep -m 1 '^| Status |')
		comments=$(show_file "$f" | LC_ALL=C grep -m 1 '^| Comments |')
		case $status in
		*'`approved`'*) ;;
		*)
			printf 'FAIL records: %s does not keep Status approved\n' "$f"
			fail=1
			;;
		esac
		case $comments in
		*'(decision)'*) ;;
		*)
			printf 'FAIL records: %s has no (decision) record\n' "$f"
			fail=1
			;;
		esac
		case $comments in
		*0038*) ;;
		*)
			printf 'FAIL records: %s does not name task 0038 in its (decision) record\n' "$f"
			fail=1
			;;
		esac
	done <"$tmp/changed"

	for pair in \
		'docs/tasks/0035_fixprguard_branch_config/03_implementation_plan.md|^## 5[.]|^## ' \
		'docs/tasks/0035_fixprguard_branch_config/02_architecture.md|^### 7[.]5[.]|^## |^### '; do
		f=${pair%%|*}
		rest=${pair#*|}
		start=${rest%%|*}
		stop=${rest#*|}
		if [ -n "${CHECK_REV:-}" ] && ! git cat-file -e "${CHECK_REV}:$f" 2>/dev/null; then
			continue
		fi
		section=$(section_body "$f" "$start" "$stop")
		n=$(printf '%s\n' "$section" | LC_ALL=C grep -c -- 'AC-18')
		if [ "$n" -eq 0 ]; then
			printf 'FAIL records: %s has no AC-18 line in the checked section\n' "$f"
			fail=1
			continue
		fi
		bad=$(printf '%s\n' "$section" |
			LC_ALL=C grep -- 'AC-18' | LC_ALL=C grep -v 'manual')
		if [ -n "$bad" ]; then
			printf 'FAIL records: %s does not check AC-18 by manual:\n%s\n' "$f" "$bad"
			fail=1
		fi
	done

	if [ "$fail" -eq 0 ]; then
		echo 'ok   records: the changed task documents record the decision, and task 0035 AC-18 is manual'
	fi
	return "$fail"
}

cmd_survey() {
	[ $# -eq 0 ] || die 'usage: survey'
	if [ -n "${CHECK_REV:-}" ]; then
		git -c core.quotePath=false ls-tree -r --name-only "$CHECK_REV" >"$tmp/all" ||
			die "cannot list the files of $CHECK_REV"
	else
		git -c core.quotePath=false ls-files --cached --others --exclude-standard >"$tmp/all" ||
			die 'cannot list the files of the working tree'
	fi
	if LC_ALL=C grep -q '^"' "$tmp/all"; then
		die 'a file name git quotes is listed; rename it to survey it'
	fi
	# The files that may read a document: Go tests, test-tagged helpers, and
	# the extension's node tests.
	: >"$tmp/candidates"
	while IFS= read -r f; do
		case $f in
		*_test.go | extension/test/*)
			printf '%s\n' "$f" >>"$tmp/candidates"
			;;
		*)
			if show_file "$f" | head -n 1 |
				LC_ALL=C grep -Eq '^//go:build (.*[[:space:]])?test([[:space:]].*)?$'; then
				printf '%s\n' "$f" >>"$tmp/candidates"
			fi
			;;
		esac
	done <"$tmp/all"
	while IFS= read -r f; do
		case $f in
		*.go | *.ts | *.js) ;;
		*) continue ;;
		esac
		if show_file "$f" |
			LC_ALL=C grep -Eq -e '\.md' -e 'docs/' -e '\.claude/' \
				-e 'README' -e 'CLAUDE' -e 'AGENTS' -e 'prompts/'; then
			printf '%s\n' "$f"
		fi
	done <"$tmp/candidates" | LC_ALL=C sort -u
}

cmd_coverage() {
	[ $# -eq 2 ] || die 'usage: coverage <before> <after>'
	before=$(git rev-parse --verify --quiet "$1^{commit}") ||
		die "unknown revision: $1"
	after=$(git rev-parse --verify --quiet "$2^{commit}") ||
		die "unknown revision: $2"
	cov=$(mktemp -d) || die 'cannot create a temporary directory'
	wt_before="$cov/before"
	wt_after="$cov/after"
	cleanup_cov() {
		git worktree remove --force "$wt_before" >/dev/null 2>&1
		git worktree remove --force "$wt_after" >/dev/null 2>&1
		rm -rf "$cov" "$tmp"
	}
	trap cleanup_cov EXIT
	trap 'exit 2' HUP INT TERM

	git worktree add --detach "$wt_before" "$before" >/dev/null 2>&1 ||
		die "cannot check out $1"
	git worktree add --detach "$wt_after" "$after" >/dev/null 2>&1 ||
		die "cannot check out $2"

	run_cover() {
		(
			cd "$1" &&
				go test -tags test -count=1 -coverpkg=./... \
					-coverprofile="$2" ./...
		) >"$3" 2>&1
	}
	run_cover "$wt_before" "$cov/before.out" "$cov/before.log" || {
		cat "$cov/before.log" >&2
		die "the before run ($1) failed; no comparison is made"
	}
	run_cover "$wt_after" "$cov/after.out" "$cov/after.log" || {
		cat "$cov/after.log" >&2
		die "the after run ($2) failed; no comparison is made"
	}
	go tool cover -func="$cov/before.out" >"$cov/before.func" ||
		die 'cannot read the before coverage profile'
	go tool cover -func="$cov/after.out" >"$cov/after.func" ||
		die 'cannot read the after coverage profile'
	LC_ALL=C sort "$cov/before.func" >"$cov/before.sorted"
	LC_ALL=C sort "$cov/after.func" >"$cov/after.sorted"
	if diff -u "$cov/before.sorted" "$cov/after.sorted" >"$cov/diff"; then
		echo "ok   coverage: $1 and $2 cover the same functions"
		return 0
	fi
	echo "FAIL coverage: $1 and $2 differ"
	cat "$cov/diff"
	return 1
}

case ${1:-} in
removed)
	shift
	cmd_removed "$@"
	;;
kept)
	shift
	cmd_kept "$@"
	;;
unchanged)
	shift
	cmd_unchanged "$@"
	;;
records)
	shift
	cmd_records "$@"
	;;
survey)
	shift
	cmd_survey "$@"
	;;
coverage)
	shift
	cmd_coverage "$@"
	;;
-h | --help)
	usage
	exit 2
	;;
'')
	usage
	die 'no subcommand'
	;;
*)
	die "unknown subcommand: $1"
	;;
esac
