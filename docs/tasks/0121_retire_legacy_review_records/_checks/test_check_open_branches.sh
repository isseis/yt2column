#!/usr/bin/env bash
# test_check_open_branches.sh exercises check_open_branches.sh against local
# temporary fake origin repositories without network access.
#
# Usage:
#   /bin/bash docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh
#   bash docs/tasks/0121_retire_legacy_review_records/_checks/test_check_open_branches.sh

set -u -o pipefail
unset CDPATH

SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd -P)
CHECK_SCRIPT="$SCRIPT_DIR/check_open_branches.sh"
TEST_BASH=${BASH:-bash}

if [ ! -f "$CHECK_SCRIPT" ]; then
	printf 'FAIL: check_open_branches.sh not found at %s\n' "$CHECK_SCRIPT" >&2
	exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/test-check-open-branches.XXXXXX")
cleanup() {
	local status=$?
	if [ -n "${WORK:-}" ]; then
		rm -rf -- "$WORK"
	fi
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

# Strip any caller repository-pointing GIT_* variables so fixture git commands
# never touch the caller's repository or index, while keeping GOCACHE so
# escape_untrusted.go builds quickly across subtests.
unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_INDEX_FILE GIT_NAMESPACE GIT_CONFIG GIT_CONFIG_PARAMETERS GIT_CONFIG_COUNT
export GIT_AUTHOR_NAME="Test Author"
export GIT_AUTHOR_EMAIL="test@example.com"
export GIT_COMMITTER_NAME="Test Committer"
export GIT_COMMITTER_EMAIL="test@example.com"
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export LC_ALL=C

PASS_COUNT=0
FAIL_COUNT=0
CASE_SEQ=0
CASE_DIR=""

fail_case() {
	printf 'FAIL [%s]: %s\n' "$CURRENT_CASE" "$1" >&2
	FAIL_COUNT=$((FAIL_COUNT + 1))
}

pass_case() {
	PASS_COUNT=$((PASS_COUNT + 1))
}

# next_case_dir allocates a unique per-case directory in the current shell.
next_case_dir() {
	CASE_SEQ=$((CASE_SEQ + 1))
	CASE_DIR="$WORK/case-$CASE_SEQ"
	mkdir -p "$CASE_DIR"
}

# setup_pair creates a bare fake origin with a main branch and a local clone.
# Sets ORIGIN_DIR and CLONE_DIR.
setup_pair() {
	local base=$1
	ORIGIN_DIR="$base/origin.git"
	CLONE_DIR="$base/clone"
	git init --bare --quiet "$ORIGIN_DIR"
	git -C "$ORIGIN_DIR" symbolic-ref HEAD refs/heads/main
	git clone --quiet "$ORIGIN_DIR" "$CLONE_DIR" 2>/dev/null
	git -C "$CLONE_DIR" checkout -q -B main
	printf 'init\n' >"$CLONE_DIR/file.txt"
	git -C "$CLONE_DIR" add file.txt
	git -C "$CLONE_DIR" commit -q -m "initial commit"
	git -C "$CLONE_DIR" push -q -u origin main
}

commit_raw_msg() {
	local repo=$1 msg_file=$2
	printf 'x\n' >>"$repo/file.txt"
	git -C "$repo" add file.txt
	git -C "$repo" commit -q --cleanup=verbatim -F "$msg_file"
}

# run_check runs CHECK_SCRIPT with a dedicated empty TMPDIR and captures
# RUN_CODE, RUN_OUT, RUN_ERR, and RUN_TMP_LEFTOVERS.
run_check() {
	local cwd=$1
	shift
	local run_dir
	run_dir=$(mktemp -d "$WORK/run.XXXXXX")
	local case_tmp="$run_dir/tmp"
	mkdir -p "$case_tmp"
	local out_f="$run_dir/stdout.txt"
	local err_f="$run_dir/stderr.txt"

	RUN_CODE=0
	(
		cd "$cwd" || exit 99
		TMPDIR="$case_tmp" "$TEST_BASH" "$CHECK_SCRIPT" "$@"
	) >"$out_f" 2>"$err_f" || RUN_CODE=$?

	RUN_OUT_FILE=$out_f
	RUN_ERR_FILE=$err_f
	RUN_OUT=$(cat "$out_f")
	RUN_ERR=$(cat "$err_f")
	RUN_TMP_LEFTOVERS=$(ls -A "$case_tmp")
}

assert_tmp_clean() {
	if [ -n "$RUN_TMP_LEFTOVERS" ]; then
		fail_case "case TMPDIR not empty after exit $RUN_CODE: $RUN_TMP_LEFTOVERS"
		return 1
	fi
	return 0
}

assert_exit() {
	local want=$1
	if [ "$RUN_CODE" -ne "$want" ]; then
		fail_case "expected exit $want, got $RUN_CODE (stdout: $RUN_OUT | stderr: $RUN_ERR)"
		return 1
	fi
	return 0
}

assert_stdout_empty() {
	if [ -s "$RUN_OUT_FILE" ]; then
		fail_case "expected empty stdout, got: $RUN_OUT"
		return 1
	fi
	return 0
}

assert_contains() {
	local file=$1 needle=$2 label=$3
	if ! grep -Fq -- "$needle" "$file"; then
		fail_case "$label missing expected substring [$needle]; actual: $(cat "$file")"
		return 1
	fi
	return 0
}

assert_not_contains_hex() {
	local file=$1 hex_byte=$2 label=$3
	local raw
	raw=$(printf "\\x$hex_byte")
	if grep -Fq -- "$raw" "$file"; then
		fail_case "$label contains raw byte 0x$hex_byte"
		return 1
	fi
	return 0
}

# 1. Exit 0 output contract with 0 non-main branches ((none)) and TMPDIR cleanup.
CURRENT_CASE="exit0_no_non_main_branches"
next_case_dir
setup_pair "$CASE_DIR"
main_oid=$(git -C "$ORIGIN_DIR" rev-parse refs/heads/main)
run_check "$CLONE_DIR"
if assert_exit 0 && assert_tmp_clean; then
	ok=1
	if ! head -n 1 "$RUN_OUT_FILE" | grep -Eq '^checked_at: [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$'; then
		fail_case "bad checked_at line: $(head -n 1 "$RUN_OUT_FILE")"
		ok=0
	fi
	assert_contains "$RUN_OUT_FILE" "origin/main: $main_oid" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "  (none)" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "matches: 0" "stdout" || ok=0
	[ "$ok" -eq 1 ] && pass_case
fi

# 2. Exit 0 output contract with sorted branches, non-matches (main-only legacy,
#    indented lines, current coderabbit lines), and caller repo unchanged.
CURRENT_CASE="exit0_sorted_branches_non_matches_and_caller_unchanged"
next_case_dir
setup_pair "$CASE_DIR"
printf 'legacy on main\n\nCodex local review: on main\nLocal CLI reviewed: 0123456789ab..cdef01234567 (codex)\nLocal CLI review: not run 0123456789ab..cdef01234567 (coderabbit: other; codex: timeout)\n' >"$CASE_DIR/m1.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/m1.msg"
git -C "$CLONE_DIR" push -q origin main

git -C "$CLONE_DIR" checkout -q -b z-branch main
printf 'clean commit\n\n  Codex local review: indented\n  Local CLI reviewed: 0123456789ab..cdef01234567 (codex)\nsee Local CLI review: not run 0123456789ab..cdef01234567 (coderabbit: other; codex: timeout)\ncodex local review: wrong case\nLocal CLI reviewed: 0123456789ab..cdef01234567 (CODEX)\nLocal CLI reviewed: 0123456789ab..cdef01234567 (codex) \nLocal CLI reviewed: 0123456789ab..cdef01234567 (coderabbit)\nLocal CLI review: not run 0123456789ab..cdef01234567 (coderabbit: other)\n' >"$CASE_DIR/z.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/z.msg"
z_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
git -C "$CLONE_DIR" push -q origin z-branch

git -C "$CLONE_DIR" checkout -q -b a-branch main
printf 'clean a\n' >"$CASE_DIR/a.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/a.msg"
a_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
git -C "$CLONE_DIR" push -q origin a-branch

git -C "$CLONE_DIR" checkout -q main
printf 'sentinel-fetch-head\n' >"$CLONE_DIR/.git/FETCH_HEAD"
refs_before=$(git -C "$CLONE_DIR" for-each-ref)
status_before=$(git -C "$CLONE_DIR" status --porcelain)
fetch_head_before=$(cat "$CLONE_DIR/.git/FETCH_HEAD")

run_check "$CLONE_DIR"
if assert_exit 0 && assert_tmp_clean; then
	ok=1
	assert_contains "$RUN_OUT_FILE" "matches: 0" "stdout" || ok=0
	a_line=$(grep -nF "  a-branch ${a_oid:0:12}" "$RUN_OUT_FILE" | cut -d: -f1)
	z_line=$(grep -nF "  z-branch ${z_oid:0:12}" "$RUN_OUT_FILE" | cut -d: -f1)
	if [ -z "$a_line" ] || [ -z "$z_line" ] || [ "$a_line" -ge "$z_line" ]; then
		fail_case "branches not listed in ascending order with 12-hex OIDs: $RUN_OUT"
		ok=0
	fi
	refs_after=$(git -C "$CLONE_DIR" for-each-ref)
	status_after=$(git -C "$CLONE_DIR" status --porcelain)
	fetch_head_after=$(cat "$CLONE_DIR/.git/FETCH_HEAD")
	if [ "$refs_before" != "$refs_after" ] || [ "$status_before" != "$status_after" ] || [ "$fetch_head_before" != "$fetch_head_after" ]; then
		fail_case "caller repository changed during check"
		ok=0
	fi
	[ "$ok" -eq 1 ] && pass_case
fi

# 3. Exit 1 detection: all 3 legacy formats, CRLF line endings, non-tip commits,
#    subject-line match, final line without trailing LF, orphan branch, merged
#    side-branch commit + merge commit message, hyphen-leading branch name, and
#    caller replace ref ignored.
CURRENT_CASE="exit1_three_formats_crlf_nontip_subject_orphan_merge_hyphen_replace"
next_case_dir
setup_pair "$CASE_DIR"

# Format 1 + CRLF at HEAD~1 on branch f1 (clean follow-up commit sits on top).
git -C "$CLONE_DIR" checkout -q -b f1 main
printf 'f1 subject\r\n\r\nCodex local review: rejected finding\r\n' >"$CASE_DIR/f1.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/f1.msg"
f1_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
printf 'clean follow-up on f1\n' >"$CASE_DIR/f1-tip.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/f1-tip.msg"
git -C "$CLONE_DIR" push -q origin f1

# Format 2 + CRLF pushed to a hyphen-leading remote branch refs/heads/-f2.
git -C "$CLONE_DIR" checkout -q -b f2-tmp main
printf 'f2 subject\r\n\r\nLocal CLI reviewed: 0123456789ab..cdef01234567 (codex)\r\n' >"$CASE_DIR/f2.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/f2.msg"
f2_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
git -C "$CLONE_DIR" push -q origin "$f2_oid:refs/heads/-f2"

# Format 3 + CRLF on branch f3, plus a commit whose subject line matches and a
# commit whose matching final line has no trailing newline.
git -C "$CLONE_DIR" checkout -q -b f3 main
printf 'f3 subject\r\n\r\nLocal CLI review: not run 0123456789ab..cdef01234567 (coderabbit: other; codex: timeout)\r\n' >"$CASE_DIR/f3.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/f3.msg"
f3_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
printf 'Codex local review: on subject line\n' >"$CASE_DIR/f3-subj.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/f3-subj.msg"
f3_subj_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
f3_tree=$(git -C "$CLONE_DIR" rev-parse HEAD^{tree})
{
	printf 'tree %s\nparent %s\nauthor Test Author <test@example.com> 1700000000 +0000\ncommitter Test Committer <test@example.com> 1700000000 +0000\n\n' "$f3_tree" "$f3_subj_oid"
	printf 'no-lf subject\n\nLocal CLI reviewed: aaaaaaaaaaaa..bbbbbbbbbbbb (codex)'
} >"$CASE_DIR/f3-nolf.bin"
f3_nolf_oid=$(git -C "$CLONE_DIR" hash-object -t commit -w --stdin <"$CASE_DIR/f3-nolf.bin")
git -C "$CLONE_DIR" update-ref refs/heads/f3 "$f3_nolf_oid"
git -C "$CLONE_DIR" push -q origin f3

# Orphan branch with legacy line.
git -C "$CLONE_DIR" checkout -q --orphan orphan-legacy
git -C "$CLONE_DIR" rm -qf . >/dev/null 2>&1 || true
printf 'orphan\n' >"$CLONE_DIR/orphan.txt"
git -C "$CLONE_DIR" add orphan.txt
printf 'orphan subject\n\nCodex local review: orphan finding\n' >"$CASE_DIR/orphan.msg"
git -C "$CLONE_DIR" commit -q --cleanup=verbatim -F "$CASE_DIR/orphan.msg"
orphan_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
git -C "$CLONE_DIR" push -q origin orphan-legacy

# Merged side branch carrying a legacy line on its non-tip parent commit AND a
# merge commit carrying a legacy line in its own message.
git -C "$CLONE_DIR" checkout -q -b side main
printf 'side legacy\n\nCodex local review: side parent finding\n' >"$CASE_DIR/side1.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/side1.msg"
side1_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
printf 'side clean tip\n' >"$CASE_DIR/side2.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/side2.msg"
git -C "$CLONE_DIR" checkout -q -b merge-branch main
printf 'mb\n' >"$CLONE_DIR/mb.txt"
git -C "$CLONE_DIR" add mb.txt
git -C "$CLONE_DIR" commit -q -m "mb work"
printf 'merge side\n\nLocal CLI reviewed: 111111111111..222222222222 (codex)\n' >"$CASE_DIR/merge.msg"
git -C "$CLONE_DIR" merge -q --no-ff side -F "$CASE_DIR/merge.msg"
merge_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
git -C "$CLONE_DIR" push -q origin merge-branch

# Add a replace ref in caller repo for f1_oid; confirms caller replace refs do
# not hide legacy commits fetched from origin.
clean_f1=$(git -C "$CLONE_DIR" commit-tree "$(git -C "$CLONE_DIR" rev-parse "$f1_oid^{tree}")" -p "$(git -C "$CLONE_DIR" rev-parse main)" -m "clean replacement")
git -C "$CLONE_DIR" replace "$f1_oid" "$clean_f1"
git -C "$CLONE_DIR" checkout -q main

run_check "$CLONE_DIR"
if assert_exit 1 && assert_tmp_clean; then
	ok=1
	assert_contains "$RUN_OUT_FILE" "matches: 8" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "f1 ${f1_oid:0:12} Codex local review: rejected finding" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "-f2 ${f2_oid:0:12} Local CLI reviewed: 0123456789ab..cdef01234567 (codex)" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "f3 ${f3_oid:0:12} Local CLI review: not run 0123456789ab..cdef01234567 (coderabbit: other; codex: timeout)" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "f3 ${f3_subj_oid:0:12} Codex local review: on subject line" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "f3 ${f3_nolf_oid:0:12} Local CLI reviewed: aaaaaaaaaaaa..bbbbbbbbbbbb (codex)" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "orphan-legacy ${orphan_oid:0:12} Codex local review: orphan finding" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "merge-branch ${side1_oid:0:12} Codex local review: side parent finding" "stdout" || ok=0
	assert_contains "$RUN_OUT_FILE" "merge-branch ${merge_oid:0:12} Local CLI reviewed: 111111111111..222222222222 (codex)" "stdout" || ok=0
	[ "$ok" -eq 1 ] && pass_case
fi

# 4. Stale refs/remotes/origin/* not false-positived; unfetched new branch detected.
CURRENT_CASE="stale_ref_ignored_and_unfetched_detected"
next_case_dir
setup_pair "$CASE_DIR"
git -C "$CLONE_DIR" checkout -q -b stale-legacy main
printf 'stale\n\nCodex local review: stale\n' >"$CASE_DIR/stale.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/stale.msg"
git -C "$CLONE_DIR" push -q origin stale-legacy
git -C "$CLONE_DIR" fetch -q origin
git -C "$ORIGIN_DIR" update-ref -d refs/heads/stale-legacy
git -C "$CLONE_DIR" checkout -q main
if ! git -C "$CLONE_DIR" show-ref --verify --quiet refs/remotes/origin/stale-legacy; then
	fail_case "fixture setup error: stale remote tracking ref missing"
else
	run_check "$CLONE_DIR"
	if assert_exit 0 && assert_tmp_clean; then
		second="$CASE_DIR/second"
		git clone --quiet "$ORIGIN_DIR" "$second" 2>/dev/null
		git -C "$second" checkout -q -b unfetched-legacy main
		printf 'unfetched\n\nCodex local review: unfetched\n' >"$CASE_DIR/unfetched.msg"
		commit_raw_msg "$second" "$CASE_DIR/unfetched.msg"
		unfetched_oid=$(git -C "$second" rev-parse HEAD)
		git -C "$second" push -q origin unfetched-legacy

		run_check "$CLONE_DIR"
		if assert_exit 1 && assert_tmp_clean; then
			ok=1
			assert_contains "$RUN_OUT_FILE" "unfetched-legacy ${unfetched_oid:0:12} Codex local review: unfetched" "stdout" || ok=0
			if git -C "$CLONE_DIR" show-ref --verify --quiet refs/remotes/origin/unfetched-legacy; then
				fail_case "caller refs/remotes/origin/unfetched-legacy was created"
				ok=0
			fi
			[ "$ok" -eq 1 ] && pass_case
		fi
	fi
fi

# 5. Linked worktree at a different directory depth with relative origin URL
#    (must resolve against main worktree root, not $PWD) and --separate-git-dir.
CURRENT_CASE="linked_worktree_relative_origin_and_separate_git_dir"
next_case_dir
setup_pair "$CASE_DIR"
git -C "$CLONE_DIR" remote set-url origin ../origin.git
wt="$CASE_DIR/nested/deep/linked-wt"
mkdir -p "$CASE_DIR/nested/deep"
git -C "$CLONE_DIR" worktree add -q -b wt-branch "$wt" main
mkdir -p "$wt/sub"
run_check "$wt/sub"
if assert_exit 0 && assert_tmp_clean; then
	sep_wt="$CASE_DIR/sep-wt"
	sep_git="$CASE_DIR/elsewhere/sep.git"
	mkdir -p "$sep_wt/inner" "$CASE_DIR/elsewhere"
	git init --quiet --separate-git-dir="$sep_git" "$sep_wt"
	git -C "$sep_wt" remote add origin ../origin.git
	run_check "$sep_wt/inner"
	if assert_exit 0 && assert_tmp_clean; then
		assert_contains "$RUN_OUT_FILE" "matches: 0" "stdout" && pass_case
	fi
fi

# 6. Fail-closed cases (exit 2, empty stdout, clean TMPDIR):
#    a) extra argument
CURRENT_CASE="exit2_extra_argument"
next_case_dir
setup_pair "$CASE_DIR"
run_check "$CLONE_DIR" extra-arg
if assert_exit 2 && assert_stdout_empty && assert_tmp_clean; then
	assert_contains "$RUN_ERR_FILE" "check-open-branches: usage:" "stderr" && pass_case
fi

#    b) nonexistent remote URL
CURRENT_CASE="exit2_nonexistent_url"
next_case_dir
setup_pair "$CASE_DIR"
git -C "$CLONE_DIR" remote set-url origin "$CASE_DIR/does-not-exist.git"
run_check "$CLONE_DIR"
if assert_exit 2 && assert_stdout_empty && assert_tmp_clean; then
	assert_contains "$RUN_ERR_FILE" "check-open-branches: fetch:" "stderr" && pass_case
fi

#    c) missing refs/heads/main on origin
CURRENT_CASE="exit2_missing_main"
next_case_dir
setup_pair "$CASE_DIR"
git -C "$CLONE_DIR" checkout -q -b other main
git -C "$CLONE_DIR" push -q origin other
git -C "$ORIGIN_DIR" symbolic-ref HEAD refs/heads/other
git -C "$ORIGIN_DIR" update-ref -d refs/heads/main
run_check "$CLONE_DIR"
if assert_exit 2 && assert_stdout_empty && assert_tmp_clean; then
	assert_contains "$RUN_ERR_FILE" "check-open-branches: main-ref:" "stderr" && pass_case
fi

#    d) non-commit ref under refs/heads (annotated tag written directly to origin)
CURRENT_CASE="exit2_non_commit_ref"
next_case_dir
setup_pair "$CASE_DIR"
tag_oid=$(git -C "$ORIGIN_DIR" mktag <<EOF
object $(git -C "$ORIGIN_DIR" rev-parse refs/heads/main)
type commit
tag fake-tag
tagger Test <test@example.com> 1700000000 +0000

annotated tag under refs/heads
EOF
)
printf '%s\n' "$tag_oid" >"$ORIGIN_DIR/refs/heads/not-a-commit"
run_check "$CLONE_DIR"
if assert_exit 2 && assert_stdout_empty && assert_tmp_clean; then
	if ! grep -Eq '^check-open-branches: (fetch|ref-type):' "$RUN_ERR_FILE"; then
		fail_case "expected fetch or ref-type diagnostic, got: $RUN_ERR"
	else
		pass_case
	fi
fi

#    e) go build failure
CURRENT_CASE="exit2_go_build_failure"
next_case_dir
setup_pair "$CASE_DIR"
bad_dir="$CASE_DIR/bad-checks"
mkdir -p "$bad_dir"
cp "$CHECK_SCRIPT" "$bad_dir/check_open_branches.sh"
printf 'package main\nfunc broken(\n' >"$bad_dir/escape_untrusted.go"
run_dir=$(mktemp -d "$WORK/run.XXXXXX")
case_tmp="$run_dir/tmp"
mkdir -p "$case_tmp"
RUN_CODE=0
(
	cd "$CLONE_DIR" || exit 99
	TMPDIR="$case_tmp" "$TEST_BASH" "$bad_dir/check_open_branches.sh"
) >"$run_dir/stdout.txt" 2>"$run_dir/stderr.txt" || RUN_CODE=$?
RUN_OUT_FILE="$run_dir/stdout.txt"
RUN_ERR_FILE="$run_dir/stderr.txt"
RUN_OUT=$(cat "$RUN_OUT_FILE")
RUN_ERR=$(cat "$RUN_ERR_FILE")
RUN_TMP_LEFTOVERS=$(ls -A "$case_tmp")
if assert_exit 2 && assert_stdout_empty && assert_tmp_clean; then
	assert_contains "$RUN_ERR_FILE" "check-open-branches: go-build:" "stderr" && pass_case
fi

#    f) case-colliding branches on case-insensitive filesystems (ls-remote vs fetched mismatch)
touch "$WORK/casecheck"
if [ -e "$WORK/CASECHECK" ]; then
	CURRENT_CASE="exit2_case_colliding_branches_ls_remote_mismatch"
	next_case_dir
	setup_pair "$CASE_DIR"
	git -C "$CLONE_DIR" checkout -q -b topic-a main
	printf 'topic-a\n' >"$CASE_DIR/ta.msg"
	commit_raw_msg "$CLONE_DIR" "$CASE_DIR/ta.msg"
	ta_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
	git -C "$CLONE_DIR" checkout -q -b topic-b main
	printf 'topic-b\n\nCodex local review: hidden by case collision\n' >"$CASE_DIR/tb.msg"
	commit_raw_msg "$CLONE_DIR" "$CASE_DIR/tb.msg"
	tb_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
	git -C "$CLONE_DIR" push -q origin "$ta_oid:refs/heads/topic" "$tb_oid:refs/heads/Topic-tmp"
	git -C "$ORIGIN_DIR" pack-refs --all
	# Rewrite packed-refs so both refs/heads/Topic and refs/heads/topic exist on origin.
	sed 's|refs/heads/Topic-tmp$|refs/heads/Topic|' "$ORIGIN_DIR/packed-refs" >"$ORIGIN_DIR/packed-refs.new"
	mv "$ORIGIN_DIR/packed-refs.new" "$ORIGIN_DIR/packed-refs"
	run_check "$CLONE_DIR"
	if assert_exit 2 && assert_stdout_empty && assert_tmp_clean; then
		assert_contains "$RUN_ERR_FILE" "check-open-branches: ref-sync:" "stderr" && pass_case
	fi
fi

# 7. Escaping of untrusted characters in matched lines, branch names (exit 0 and exit 1),
#    stderr diagnostics, and pass-through of normal UTF-8 and valid U+FFFD.
CURRENT_CASE="escape_untrusted_stdout_and_stderr"
next_case_dir
setup_pair "$CASE_DIR"

b_c1=$(printf '\302\205')
b_bidi=$(printf '\342\200\256')
b_zwsp=$(printf '\342\200\213')
b_jp=$(printf '\346\227\245\346\234\254\350\252\236')
b_fffd=$(printf '\357\277\275')
tricky_branch="tricky-${b_c1}-${b_bidi}-${b_zwsp}-${b_jp}-${b_fffd}"
want_tricky_branch="tricky-\\xc2\\x85-\\xe2\\x80\\xae-\\xe2\\x80\\x8b-${b_jp}-${b_fffd}"

git -C "$CLONE_DIR" checkout -q -b "$tricky_branch" main
printf 'clean commit on tricky branch\n' >"$CASE_DIR/t0.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/t0.msg"
git -C "$CLONE_DIR" push -q origin "$tricky_branch"

run_check "$CLONE_DIR"
if assert_exit 0 && assert_tmp_clean; then
	ok=1
	assert_contains "$RUN_OUT_FILE" "  $want_tricky_branch " "stdout(exit0)" || ok=0
	if grep -Fq -- "$b_c1" "$RUN_OUT_FILE" || grep -Fq -- "$b_bidi" "$RUN_OUT_FILE" || grep -Fq -- "$b_zwsp" "$RUN_OUT_FILE"; then
		fail_case "exit 0 stdout contains raw forbidden control/format sequence in branch name"
		ok=0
	fi
	if [ "$ok" -eq 1 ]; then
		# Build raw commit object via hash-object so git does not re-encode
		# noncharacters (U+FDD0, U+FFFE) or invalid UTF-8 (0xFF) bytes as Latin-1.
		parent_oid=$(git -C "$CLONE_DIR" rev-parse HEAD)
		tree_oid=$(git -C "$CLONE_DIR" rev-parse HEAD^{tree})
		{
			printf 'tree %s\nparent %s\nauthor Test Author <test@example.com> 1700000000 +0000\ncommitter Test Committer <test@example.com> 1700000000 +0000\n\n' "$tree_oid" "$parent_oid"
			printf 'escape test\n\nCodex local review: t=\011 e=\033 d=\177 c1=\302\205 bidi=\342\200\256 zw=\342\200\213 tag=\363\240\200\201 zl=\342\200\250 zp=\342\200\251 nc=\357\267\220 nc2=\357\277\276 bad=\377 bs=\\ jp=\346\227\245\346\234\254\350\252\236 rep=\357\277\275\n'
		} >"$CASE_DIR/raw_commit.bin"
		t1_oid=$(git -C "$CLONE_DIR" hash-object -t commit -w --stdin <"$CASE_DIR/raw_commit.bin")
		git -C "$CLONE_DIR" update-ref "refs/heads/$tricky_branch" "$t1_oid"
		git -C "$CLONE_DIR" push -q origin "$tricky_branch"

		run_check "$CLONE_DIR"
		if assert_exit 1 && assert_tmp_clean; then
			want_line="${want_tricky_branch} ${t1_oid:0:12} Codex local review: t=\\x09 e=\\x1b d=\\x7f c1=\\xc2\\x85 bidi=\\xe2\\x80\\xae zw=\\xe2\\x80\\x8b tag=\\xf3\\xa0\\x80\\x81 zl=\\xe2\\x80\\xa8 zp=\\xe2\\x80\\xa9 nc=\\xef\\xb7\\x90 nc2=\\xef\\xbf\\xbe bad=\\xff bs=\\\\ jp=${b_jp} rep=${b_fffd}"
			assert_contains "$RUN_OUT_FILE" "$want_line" "stdout(exit1)" || ok=0
			assert_not_contains_hex "$RUN_OUT_FILE" "1b" "stdout(exit1)" || ok=0
			assert_not_contains_hex "$RUN_OUT_FILE" "09" "stdout(exit1)" || ok=0
			assert_not_contains_hex "$RUN_OUT_FILE" "7f" "stdout(exit1)" || ok=0
			assert_not_contains_hex "$RUN_OUT_FILE" "ff" "stdout(exit1)" || ok=0
		else
			ok=0
		fi
	fi
	if [ "$ok" -eq 1 ]; then
		# git's vreportf passes tab, LF, C1, Cf (bidi/zwsp), and backslash through
		# verbatim to stderr; verify escape_untrusted escapes all of them.
		bad_url="$CASE_DIR/missing-$(printf '\011')-${b_c1}-${b_bidi}-${b_zwsp}-\\-repo.git"
		git -C "$CLONE_DIR" remote set-url origin "$bad_url"
		run_check "$CLONE_DIR"
		if assert_exit 2 && assert_stdout_empty && assert_tmp_clean; then
			assert_contains "$RUN_ERR_FILE" '\x09' "stderr" || ok=0
			assert_contains "$RUN_ERR_FILE" '\xc2\x85' "stderr" || ok=0
			assert_contains "$RUN_ERR_FILE" '\xe2\x80\xae' "stderr" || ok=0
			assert_contains "$RUN_ERR_FILE" '\xe2\x80\x8b' "stderr" || ok=0
			assert_contains "$RUN_ERR_FILE" '\\' "stderr" || ok=0
			assert_contains "$RUN_ERR_FILE" '\x0a' "stderr" || ok=0
			assert_not_contains_hex "$RUN_ERR_FILE" "09" "stderr" || ok=0
			if grep -Fq -- "$b_c1" "$RUN_ERR_FILE" || grep -Fq -- "$b_bidi" "$RUN_ERR_FILE" || grep -Fq -- "$b_zwsp" "$RUN_ERR_FILE"; then
				fail_case "stderr contains raw C1/bidi/zwsp sequence"
				ok=0
			fi
		else
			ok=0
		fi
	fi
	[ "$ok" -eq 1 ] && pass_case
fi

# 8. GIT_DIR / GIT_WORK_TREE / CDPATH / GOOS / GOARCH / GIT_REPLACE_REF_BASE /
#    GIT_GRAFT_FILE override resilience (simulating invocation from a hook or
#    cross-compilation shell via a relative script path).
CURRENT_CASE="git_dir_and_env_override_resilience"
next_case_dir
setup_pair "$CASE_DIR"
# Add a non-main branch so escape_untrusted actually runs during the check
# (proving GOOS=js GOARCH=wasm in the caller shell did not build a non-host binary).
git -C "$CLONE_DIR" checkout -q -b env-branch main
printf 'env test\n' >"$CASE_DIR/env.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/env.msg"
git -C "$CLONE_DIR" push -q origin env-branch
git -C "$CLONE_DIR" checkout -q main
refs_before=$(git -C "$CLONE_DIR" for-each-ref)
run_dir=$(mktemp -d "$WORK/run.XXXXXX")
case_tmp="$run_dir/tmp"
mkdir -p "$case_tmp"
script_dir_base=$(basename -- "$SCRIPT_DIR")
RUN_CODE=0
(
	cd "$SCRIPT_DIR/.." || exit 99
	CDPATH=. GOOS=js GOARCH=wasm \
		GIT_DIR="$CLONE_DIR/.git" GIT_WORK_TREE="$CLONE_DIR" \
		GIT_REPLACE_REF_BASE="refs/fake-replace/" GIT_GRAFT_FILE="$CASE_DIR/nonexistent-grafts" \
		TMPDIR="$case_tmp" "$TEST_BASH" "$script_dir_base/check_open_branches.sh"
) >"$run_dir/stdout.txt" 2>"$run_dir/stderr.txt" || RUN_CODE=$?
RUN_OUT_FILE="$run_dir/stdout.txt"
RUN_ERR_FILE="$run_dir/stderr.txt"
RUN_OUT=$(cat "$RUN_OUT_FILE")
RUN_ERR=$(cat "$RUN_ERR_FILE")
RUN_TMP_LEFTOVERS=$(ls -A "$case_tmp")
if assert_exit 0 && assert_tmp_clean; then
	refs_after=$(git -C "$CLONE_DIR" for-each-ref)
	if [ "$refs_before" != "$refs_after" ]; then
		fail_case "caller refs modified when GIT_DIR was set"
	else
		assert_contains "$RUN_OUT_FILE" "matches: 0" "stdout" && pass_case
	fi
fi

# 9. Shallow clone caller succeeds without alternates connectivity error.
CURRENT_CASE="shallow_clone_caller"
next_case_dir
setup_pair "$CASE_DIR"
printf 'second\n' >"$CASE_DIR/m2.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/m2.msg"
git -C "$CLONE_DIR" push -q origin main
git -C "$CLONE_DIR" checkout -q -b feat main~1
printf 'feat\n' >"$CASE_DIR/feat.msg"
commit_raw_msg "$CLONE_DIR" "$CASE_DIR/feat.msg"
git -C "$CLONE_DIR" push -q origin feat

shallow_dir="$CASE_DIR/shallow-clone"
git clone --quiet --depth 1 "file://$ORIGIN_DIR" "$shallow_dir" 2>/dev/null
if [ "$(git -C "$shallow_dir" rev-parse --is-shallow-repository)" != "true" ]; then
	fail_case "fixture setup error: expected shallow clone"
else
	run_check "$shallow_dir"
	if assert_exit 0 && assert_tmp_clean; then
		assert_contains "$RUN_OUT_FILE" "matches: 0" "stdout" && pass_case
	fi
fi

# 10. Genuine partial clones (missing trees/blobs) skip alternates cleanly under
#     both extensions.partialClone and remote.<name>.promisor=true.
CURRENT_CASE="partial_clone_extensions_and_promisor"
next_case_dir
setup_pair "$CASE_DIR"
git -C "$ORIGIN_DIR" config uploadpack.allowFilter true
printf 'extra payload\n' >"$CLONE_DIR/extra.txt"
git -C "$CLONE_DIR" add extra.txt
git -C "$CLONE_DIR" commit -q -m "add extra blob"
git -C "$CLONE_DIR" push -q origin main

# Create both tree:0 partial clones before pushing the next commit so mistaken
# alternates reuse fails fetch connectivity with "missing blob object".
partial_ext="$CASE_DIR/partial-ext"
git clone --quiet --no-checkout --filter=tree:0 "file://$ORIGIN_DIR" "$partial_ext" 2>/dev/null
git -C "$partial_ext" config --unset-all remote.origin.promisor 2>/dev/null || true
git -C "$partial_ext" config --unset-all remote.origin.partialclonefilter 2>/dev/null || true
git -C "$partial_ext" config extensions.partialClone origin

partial_prom="$CASE_DIR/partial-promisor"
git clone --quiet --no-checkout --filter=tree:0 "file://$ORIGIN_DIR" "$partial_prom" 2>/dev/null
git -C "$partial_prom" config --unset-all extensions.partialClone 2>/dev/null || true

printf 'second payload\n' >"$CLONE_DIR/second.txt"
git -C "$CLONE_DIR" add second.txt
git -C "$CLONE_DIR" commit -q -m "add second blob"
git -C "$CLONE_DIR" push -q origin main

if ! git -C "$partial_ext" rev-list --all --objects --missing=print | grep '^?' >/dev/null ||
	! git -C "$partial_prom" rev-list --all --objects --missing=print | grep '^?' >/dev/null; then
	fail_case "fixture setup error: partial clones unexpectedly have no missing objects"
else
	run_check "$partial_ext"
	if assert_exit 0 && assert_tmp_clean; then
		ok=1
		assert_contains "$RUN_OUT_FILE" "matches: 0" "stdout(partial-ext)" || ok=0
		run_check "$partial_prom"
		if assert_exit 0 && assert_tmp_clean; then
			assert_contains "$RUN_OUT_FILE" "matches: 0" "stdout(partial-promisor)" || ok=0
		else
			ok=0
		fi
		[ "$ok" -eq 1 ] && pass_case
	fi
fi

printf 'test_check_open_branches (%s): %d passed, %d failed\n' "$TEST_BASH" "$PASS_COUNT" "$FAIL_COUNT"
if [ "$FAIL_COUNT" -ne 0 ]; then
	exit 1
fi
exit 0
