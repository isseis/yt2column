#!/usr/bin/env bash
# check_open_branches.sh verifies that no non-main remote branch on origin
# contains legacy Codex CLI review record lines in commits unreachable from
# origin's main branch.
#
# Usage: bash docs/tasks/0121_retire_legacy_review_records/_checks/check_open_branches.sh
#
# Contract:
#   - Takes no arguments; any argument fails closed with exit 2 and empty stdout.
#   - Never modifies the caller's worktree, index, HEAD, local branches, or
#     refs/remotes/origin/*. All remote fetching happens into a fresh temporary
#     bare repository under TMPDIR that is always removed on EXIT.
#   - Verifies ref set equality between `git ls-remote --heads` and the fetched
#     refs/heads/*, and requires refs/heads/main and every branch ref to point
#     to a commit object.
#   - Buffers all stdout until every branch and commit has been read cleanly.
#   - Escapes untrusted strings (branch names, matched lines, and diagnostics)
#     using escape_untrusted.go built once into the temporary directory.
#   - Exit codes:
#       0: scan succeeded with 0 matches
#       1: scan succeeded with 1 or more matches
#       2: any usage, fetch, verification, build, or read error (empty stdout)

set -u -o pipefail

# Preserve bash 3.2 compatibility: do not use associative arrays or mapfile.
unset -f git awk sed sort cmp rm cat printf mktemp date go 2>/dev/null || true

TMP_WORK=""
ESC_BIN=""

cleanup() {
	local status=$?
	if [ -n "$TMP_WORK" ]; then
		command rm -rf -- "$TMP_WORK"
	fi
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

# die_safe writes a fixed safe ASCII diagnostic to stderr and exits 2 without
# emitting anything on stdout. Used before ESC_BIN is built or if escaping fails.
die_safe() {
	command printf 'check-open-branches: %s: %s\n' "$1" "$2" >&2
	exit 2
}

# die_escaped escapes diagnostic text from stdin through ESC_BIN and exits 2.
die_escaped_file() {
	local step=$1 err_file=$2 escaped=""
	if [ -n "$ESC_BIN" ] && [ -x "$ESC_BIN" ]; then
		if escaped=$("$ESC_BIN" -trim-final-lf <"$err_file" 2>/dev/null); then
			if [ -z "$escaped" ]; then
				escaped="command failed"
			fi
			command printf 'check-open-branches: %s: %s\n' "$step" "$escaped" >&2
			exit 2
		fi
	fi
	die_safe "$step" "command failed (diagnostic unavailable)"
}

die_escaped_msg() {
	local step=$1 msg=$2 escaped=""
	if [ -n "$ESC_BIN" ] && [ -x "$ESC_BIN" ]; then
		if escaped=$(command printf '%s' "$msg" | "$ESC_BIN" 2>/dev/null); then
			command printf 'check-open-branches: %s: %s\n' "$step" "$escaped" >&2
			exit 2
		fi
	fi
	die_safe "$step" "diagnostic unavailable"
}

escape_str() {
	local out
	if ! out=$(command printf '%s' "$1" | "$ESC_BIN" 2>/dev/null); then
		die_safe "escape" "escape helper failed"
	fi
	ESCAPED=$out
}

if [ "$#" -ne 0 ]; then
	die_safe "usage" "expected no arguments, got $#"
fi

SCRIPT_DIR=$(cd -- "$(dirname -- "$0")" && pwd -P) || die_safe "init" "cannot resolve script directory"
ESC_SRC="$SCRIPT_DIR/escape_untrusted.go"
if [ ! -f "$ESC_SRC" ]; then
	die_safe "init" "escape_untrusted.go not found next to script"
fi

TMP_BASE=${TMPDIR:-/tmp}
if [ "$TMP_BASE" != "/" ]; then
	TMP_BASE=${TMP_BASE%/}
fi
TMP_WORK=$(command mktemp -d "${TMP_BASE}/check-open-branches.XXXXXX" 2>/dev/null) || die_safe "mktemp" "cannot create temporary directory"

ESC_BIN="$TMP_WORK/escape-untrusted"
if ! GO111MODULE=off GOTOOLCHAIN=local GOENV=off GOFLAGS= GOPROXY=off GOWORK=off \
	command go build -o "$ESC_BIN" "$ESC_SRC" >/dev/null 2>&1; then
	ESC_BIN=""
	die_safe "go-build" "failed to build escape helper"
fi
if [ ! -x "$ESC_BIN" ]; then
	ESC_BIN=""
	die_safe "go-build" "escape helper binary is not executable"
fi

ERR_FILE="$TMP_WORK/git.err"
TMP_REPO="$TMP_WORK/origin.git"

# git_caller runs git against the caller repository while keeping user/system
# config (for URL resolution and repo discovery) and blocking hooks/fsmonitor.
git_caller() {
	(
		export GIT_TERMINAL_PROMPT=0 GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE=/dev/null
		exec command git -c core.hooksPath=/dev/null -c core.fsmonitor=false "$@"
	)
}

# git_tmp_fetch runs git against the temporary bare repository during the fetch
# phase: keeps user/system config and network/auth environment, while stripping
# repository-pointing GIT_* variables so hooks or outer GIT_DIR cannot redirect it.
git_tmp_fetch() {
	(
		unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_INDEX_FILE GIT_NAMESPACE
		export GIT_TERMINAL_PROMPT=0 GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE=/dev/null
		exec command git -c core.hooksPath=/dev/null -c core.fsmonitor=false "$@"
	)
}

# git_tmp_inspect runs read-only inspection against the temporary bare
# repository in a clean, deterministic git and C-locale environment.
git_tmp_inspect() {
	(
		local name
		for name in ${!GIT_@}; do
			unset "$name"
		done
		export LC_ALL=C
		export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_ATTR_NOSYSTEM=1
		export GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0 GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE=/dev/null
		exec command git \
			-c core.attributesFile=/dev/null \
			-c core.hooksPath=/dev/null \
			-c core.fsmonitor=false \
			-c core.quotePath=true \
			-c diff.relative=false \
			-c diff.noprefix=false \
			-c color.ui=false \
			-c log.showSignature=false \
			-c advice.graftFileDeprecated=false \
			-c i18n.logOutputEncoding=UTF-8 \
			--git-dir="$TMP_REPO" "$@"
	)
}

# Resolve caller repository metadata.
ORIGIN_URL=$(git_caller remote get-url origin 2>"$ERR_FILE") || die_escaped_file "origin-url" "$ERR_FILE"
if [ -z "$ORIGIN_URL" ]; then
	die_safe "origin-url" "remote origin URL is empty"
fi

# Resolve a relative local origin URL against the main worktree root (the first
# `worktree` line of `git worktree list --porcelain`), matching git's own rule.
is_relative_local_url() {
	local u=$1 first_seg
	case $u in
	/* | *://*) return 1 ;;
	./* | ../*) return 0 ;;
	esac
	first_seg=${u%%/*}
	case $first_seg in
	*:*) return 1 ;;
	esac
	return 0
}

RAW_COMMON_DIR=$(git_caller rev-parse --git-common-dir 2>"$ERR_FILE") || die_escaped_file "git-common-dir" "$ERR_FILE"
CALLER_COMMON_DIR=$(cd -- "$RAW_COMMON_DIR" && pwd -P) || die_safe "git-common-dir" "cannot resolve common git directory"

if is_relative_local_url "$ORIGIN_URL"; then
	WT_LIST="$TMP_WORK/worktrees.txt"
	if ! git_caller worktree list --porcelain >"$WT_LIST" 2>"$ERR_FILE"; then
		die_escaped_file "worktree-list" "$ERR_FILE"
	fi
	MAIN_WT=""
	while IFS= read -r wt_line; do
		case $wt_line in
		"worktree "*)
			MAIN_WT=${wt_line#"worktree "}
			break
			;;
		esac
	done <"$WT_LIST"
	if [ -z "$MAIN_WT" ]; then
		die_safe "worktree-list" "cannot determine main worktree path"
	fi
	# In a --separate-git-dir repo without core.worktree, `worktree list` reports
	# the git-common-dir itself; resolve against --show-toplevel when inside a worktree.
	MAIN_WT_REAL=$(cd -- "$MAIN_WT" 2>/dev/null && pwd -P || printf '%s' "$MAIN_WT")
	if [ "$MAIN_WT_REAL" = "$CALLER_COMMON_DIR" ]; then
		if TOP_WT=$(git_caller rev-parse --show-toplevel 2>/dev/null) && [ -n "$TOP_WT" ]; then
			MAIN_WT=$TOP_WT
		fi
	fi
	ORIGIN_URL="${MAIN_WT%/}/$ORIGIN_URL"
fi

IS_SHALLOW=$(git_caller rev-parse --is-shallow-repository 2>"$ERR_FILE") || die_escaped_file "shallow-check" "$ERR_FILE"

USE_ALTERNATES=0
if [ "$IS_SHALLOW" = "false" ] && [ ! -e "$CALLER_COMMON_DIR/shallow" ] && [ -d "$CALLER_COMMON_DIR/objects" ]; then
	# Do not register alternates for partial clones (extensions.partialClone or
	# any remote.<name>.promisor=true), because missing objects fail fetch connectivity.
	PARTIAL_EXT=$(git_caller config --get extensions.partialClone 2>/dev/null || true)
	PROMISOR_OUT=$(git_caller config --bool --get-regexp '^remote\..*\.promisor$' 2>/dev/null || true)
	HAS_PROMISOR=0
	while IFS= read -r p_line; do
		case $p_line in
		*" true")
			HAS_PROMISOR=1
			break
			;;
		esac
	done <<<"$PROMISOR_OUT"
	if [ -z "$PARTIAL_EXT" ] && [ "$HAS_PROMISOR" -eq 0 ]; then
		USE_ALTERNATES=1
	fi
fi

# Initialize empty temporary bare repository.
if ! git_tmp_fetch init --bare --quiet "$TMP_REPO" >/dev/null 2>"$ERR_FILE"; then
	die_escaped_file "init-bare" "$ERR_FILE"
fi

if [ "$USE_ALTERNATES" -eq 1 ]; then
	command printf '%s/objects\n' "$CALLER_COMMON_DIR" >"$TMP_REPO/objects/info/alternates" || die_safe "alternates" "cannot write alternates file"
fi

# Fetch all remote heads without tags, then query ls-remote --heads to verify
# the exact branch set (guards against case-insensitive filesystems and races).
if ! git_tmp_fetch --git-dir="$TMP_REPO" fetch --quiet --no-tags -- "$ORIGIN_URL" '+refs/heads/*:refs/heads/*' >/dev/null 2>"$ERR_FILE"; then
	die_escaped_file "fetch" "$ERR_FILE"
fi

LS_REMOTE_RAW="$TMP_WORK/ls-remote.raw"
if ! git_tmp_fetch --git-dir="$TMP_REPO" ls-remote --heads -- "$ORIGIN_URL" >"$LS_REMOTE_RAW" 2>"$ERR_FILE"; then
	die_escaped_file "ls-remote" "$ERR_FILE"
fi

# Inspection phase: strict C locale and isolated git configuration.
export LC_ALL=C

TAB=$(command printf '\t')
CR=$(command printf '\r')
HEX40='^[0-9a-f]{40}$'
LS_REMOTE_NORM="$TMP_WORK/ls-remote.norm"
: >"$LS_REMOTE_NORM"
while IFS= read -r ls_line || [ -n "$ls_line" ]; do
	[ -n "$ls_line" ] || continue
	case $ls_line in
	*"$TAB"*)
		ls_oid=${ls_line%%"$TAB"*}
		ls_ref=${ls_line#*"$TAB"}
		;;
	*)
		die_escaped_msg "ls-remote" "malformed ls-remote line: $ls_line"
		;;
	esac
	if [[ ! $ls_oid =~ $HEX40 ]]; then
		die_escaped_msg "ls-remote" "invalid object id in ls-remote: $ls_oid"
	fi
	case $ls_ref in
	refs/heads/*) ;;
	*)
		die_escaped_msg "ls-remote" "unexpected ref in ls-remote --heads: $ls_ref"
		;;
	esac
	command printf '%s\t%s\n' "$ls_ref" "$ls_oid" >>"$LS_REMOTE_NORM"
done <"$LS_REMOTE_RAW"

LS_REMOTE_SORTED="$TMP_WORK/ls-remote.sorted"
command sort "$LS_REMOTE_NORM" >"$LS_REMOTE_SORTED" || die_safe "sort" "failed to sort ls-remote output"

FETCHED_RAW="$TMP_WORK/fetched.raw"
if ! git_tmp_inspect for-each-ref --format="%(refname)${TAB}%(objectname)${TAB}%(objecttype)" refs/heads >"$FETCHED_RAW" 2>"$ERR_FILE"; then
	die_escaped_file "for-each-ref" "$ERR_FILE"
fi

FETCHED_NORM="$TMP_WORK/fetched.norm"
BRANCHES_FILE="$TMP_WORK/branches.tsv"
: >"$FETCHED_NORM"
: >"$BRANCHES_FILE"
MAIN_OID=""

while IFS= read -r f_line || [ -n "$f_line" ]; do
	[ -n "$f_line" ] || continue
	f_ref=${f_line%%"$TAB"*}
	f_rest=${f_line#*"$TAB"}
	if [ "$f_rest" = "$f_line" ]; then
		die_safe "for-each-ref" "malformed for-each-ref output"
	fi
	f_oid=${f_rest%%"$TAB"*}
	f_type=${f_rest#*"$TAB"}
	if [ "$f_type" = "$f_rest" ]; then
		die_safe "for-each-ref" "malformed for-each-ref output"
	fi
	if [[ ! $f_oid =~ $HEX40 ]]; then
		die_escaped_msg "for-each-ref" "invalid object id: $f_oid"
	fi
	if [ "$f_type" != "commit" ]; then
		die_escaped_msg "ref-type" "ref $f_ref has non-commit type $f_type"
	fi
	command printf '%s\t%s\n' "$f_ref" "$f_oid" >>"$FETCHED_NORM"
	if [ "$f_ref" = "refs/heads/main" ]; then
		MAIN_OID=$f_oid
	else
		command printf '%s\t%s\n' "${f_ref#refs/heads/}" "$f_oid" >>"$BRANCHES_FILE"
	fi
done <"$FETCHED_RAW"

FETCHED_SORTED="$TMP_WORK/fetched.sorted"
command sort "$FETCHED_NORM" >"$FETCHED_SORTED" || die_safe "sort" "failed to sort fetched refs"

if ! command cmp -s "$LS_REMOTE_SORTED" "$FETCHED_SORTED"; then
	die_safe "ref-sync" "fetched refs/heads set does not match origin ls-remote --heads"
fi

if [ -z "$MAIN_OID" ]; then
	die_safe "main-ref" "refs/heads/main is missing on origin"
fi

BRANCHES_SORTED="$TMP_WORK/branches.sorted"
command sort "$BRANCHES_FILE" >"$BRANCHES_SORTED" || die_safe "sort" "failed to sort non-main branches"

# Three legacy record line patterns (column 1 fixed, POSIX ERE, after stripping
# at most one trailing CR from CRLF lines).
PAT_CODEX_REVIEW='^Codex local review:'
PAT_SEEN_CODEX='^Local CLI reviewed: [0-9a-f]{12}\.\.[0-9a-f]{12} \(codex\)$'
PAT_NOT_RUN_CODEX='^Local CLI review: not run [0-9a-f]{12}\.\.[0-9a-f]{12} \(.*codex:.*\)$'

BRANCH_LINES="$TMP_WORK/out-branches.txt"
MATCH_LINES="$TMP_WORK/out-matches.txt"
COMMITS_FILE="$TMP_WORK/commits.txt"
COMMIT_OBJ="$TMP_WORK/commit.obj"
: >"$BRANCH_LINES"
: >"$MATCH_LINES"

BRANCH_COUNT=0
MATCH_COUNT=0

while IFS= read -r b_line || [ -n "$b_line" ]; do
	[ -n "$b_line" ] || continue
	b_name=${b_line%%"$TAB"*}
	b_oid=${b_line#*"$TAB"}
	BRANCH_COUNT=$((BRANCH_COUNT + 1))

	escape_str "$b_name"
	b_esc=$ESCAPED
	command printf '  %s %s\n' "$b_esc" "${b_oid:0:12}" >>"$BRANCH_LINES"

	if ! git_tmp_inspect rev-list --reverse "$b_oid" "^$MAIN_OID" >"$COMMITS_FILE" 2>"$ERR_FILE"; then
		die_escaped_file "rev-list" "$ERR_FILE"
	fi

	while IFS= read -r c_oid || [ -n "$c_oid" ]; do
		[ -n "$c_oid" ] || continue
		if [[ ! $c_oid =~ $HEX40 ]]; then
			die_escaped_msg "rev-list" "invalid commit id: $c_oid"
		fi
		if ! git_tmp_inspect cat-file commit "$c_oid" >"$COMMIT_OBJ" 2>"$ERR_FILE"; then
			die_escaped_file "cat-file" "$ERR_FILE"
		fi

		in_body=0
		while IFS= read -r msg_line || [ -n "$msg_line" ]; do
			if [ "$in_body" -eq 0 ]; then
				if [ -z "$msg_line" ]; then
					in_body=1
				fi
				continue
			fi
			# Strip at most one trailing CR so CRLF messages do not evade matching.
			msg_line=${msg_line%"$CR"}
			if [[ $msg_line =~ $PAT_CODEX_REVIEW ]] || [[ $msg_line =~ $PAT_SEEN_CODEX ]] || [[ $msg_line =~ $PAT_NOT_RUN_CODEX ]]; then
				MATCH_COUNT=$((MATCH_COUNT + 1))
				escape_str "$msg_line"
				command printf '%s %s %s\n' "$b_esc" "${c_oid:0:12}" "$ESCAPED" >>"$MATCH_LINES"
			fi
		done <"$COMMIT_OBJ"
	done <"$COMMITS_FILE"
done <"$BRANCHES_SORTED"

CHECKED_AT=$(command date -u '+%Y-%m-%dT%H:%M:%SZ') || die_safe "date" "cannot format UTC timestamp"

OUT_FILE="$TMP_WORK/stdout.txt"
{
	command printf 'checked_at: %s\n' "$CHECKED_AT"
	command printf 'origin/main: %s\n' "$MAIN_OID"
	command printf 'branches:\n'
	if [ "$BRANCH_COUNT" -eq 0 ]; then
		command printf '  (none)\n'
	else
		command cat "$BRANCH_LINES"
	fi
	command printf 'matches: %d\n' "$MATCH_COUNT"
	if [ "$MATCH_COUNT" -gt 0 ]; then
		command cat "$MATCH_LINES"
	fi
} >"$OUT_FILE" || die_safe "output" "failed to buffer output"

command cat "$OUT_FILE" || exit 2
if [ "$MATCH_COUNT" -gt 0 ]; then
	exit 1
fi
exit 0
