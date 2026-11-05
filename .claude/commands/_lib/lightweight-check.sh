#!/usr/bin/env bash
# lightweight-check.sh rechecks the lightweight process's mechanically checked
# conditions (docs/dev/conventions/requirements_process.md, "Lightweight
# Process") on the real diff of the current branch.
#
# Usage: bash lightweight-check.sh <spec-file>
#
# <spec-file> is the saved body of the approved spec comment. /runissue runs
# the copy of this script that `git show origin/main:` writes out, never the
# work tree's copy.
#
# Contract:
#   - The verdict depends on no value the caller chooses: BASE is the merge
#     base of refs/remotes/origin/main and HEAD, the type is the one letter
#     under the spec's type heading, the tool commits are the commits that
#     carry a trailer right after BASE, and the conditions table is read from
#     origin/main's requirements_process.md, never from the branch it checks.
#   - Git reads neither the global nor the system configuration, nor any GIT_
#     variable of the caller's environment, and every repository setting that
#     could change what is read is overridden on the command line.
#   - FAIL is reported only for a fact about the diff. A failed step, a broken
#     input, or a tool commit that cannot be reproduced is STOP, because FAIL
#     leads to a revert that cannot be undone.
#   - Every temporary file and worktree is removed before the script exits,
#     however it exits.
#   - It uses no network and no bash feature newer than 3.2. It needs a git
#     that accepts --attr-source (GIT_ATTR_SOURCE); with an older one the
#     variable is ignored, so the script checks the option first and stops.
#
# Why this is a bash script and when to decide that again (the revisit
# conditions, two of which TestLightweightCheckRevisitConditions counts):
# docs/dev/adr/0012-lightweight-check-in-bash.md.
#
# Output: lines that start with "lightweight-check: "; the last is the verdict.
# Exit status: 0 PASS, 1 FAIL, 2 STOP.

# Run again in a clean environment, so nothing the caller's environment
# carries (exported functions, SHELLOPTS, BASHOPTS, CDPATH, GIT_ variables)
# reaches the shell or the commands it runs: only PATH, HOME, and TMPDIR are
# kept.
if [ "${LIGHTWEIGHT_CHECK_CLEAN_ENV:-}" != 1 ]; then
	exec env -i LIGHTWEIGHT_CHECK_CLEAN_ENV=1 PATH="${PATH:-}" HOME="${HOME:-}" TMPDIR="${TMPDIR:-}" \
		"${BASH:-bash}" --noprofile --norc "$0" "$@"
fi
unset LIGHTWEIGHT_CHECK_CLEAN_ENV

export LC_ALL=C

# Drop every GIT_ variable the caller's environment carries (GIT_DIR,
# GIT_CONFIG_PARAMETERS, GIT_CONFIG_COUNT, GIT_EXTERNAL_DIFF, ...), then turn
# the global and system configuration and attributes off.
for name in ${!GIT_@}; do
	unset "$name"
done
unset -f git tr mktemp rm sha256sum shasum
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_ATTR_NOSYSTEM=1
export GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0

# The repository settings that could change a diff, a status, a checkout, or
# a hook run. The command line overrides the repository's configuration.
GIT_OVERRIDES=(
	-c core.hooksPath=/dev/null
	-c core.fsmonitor=false
	-c core.untrackedCache=false
	-c core.attributesFile=/dev/null
	-c core.excludesFile=/dev/null
	-c core.autocrlf=false
	-c core.eol=lf
	-c core.safecrlf=false
	-c core.bigFileThreshold=512m
	-c core.sparseCheckout=false
	-c diff.relative=false
	-c diff.ignoreSubmodules=none
	-c status.showUntrackedFiles=all
	-c submodule.recurse=false
	-c log.showSignature=false
)

# The diff options that fix what a diff reads, whatever the configuration.
DIFF_OPTIONS=(
	--no-ext-diff --no-textconv --no-color --diff-algorithm=myers
	--no-renames --ignore-submodules=none --no-relative -z
)

g() {
	command git "${GIT_OVERRIDES[@]}" "$@"
}


say() {
	printf 'lightweight-check: %s\n' "$1"
}

# printable writes $1 with each control character replaced by "?", so a path
# cannot add a line of its own to the output.
printable() {
	printf '%s' "$1" | tr '\000-\037\177' '?'
}

stop() {
	if [ -n "$2" ]; then
		say "$2"
	fi
	say "STOP $1"
	exit 2
}

TMP_DIR=
WORKTREE=

# worktree_listed reports whether git still lists $WORKTREE as a worktree.
worktree_listed() {
	local line
	while IFS= read -r line; do
		if [ "$line" = "worktree $WORKTREE" ]; then
			return 0
		fi
	done < <(g worktree list --porcelain 2>/dev/null)
	return 1
}

cleanup() {
	if [ -n "$WORKTREE" ] && [ -d "$WORKTREE" ]; then
		g worktree remove --force "$WORKTREE" >/dev/null 2>&1
	fi
	if [ -n "$TMP_DIR" ]; then
		rm -rf "$TMP_DIR"
	fi
	# Prune only when the script's own worktree is still registered, since
	# a prune also drops other stale worktrees of the user.
	if [ -n "$WORKTREE" ] && worktree_listed; then
		g worktree prune >/dev/null 2>&1
	fi
}
trap cleanup EXIT
trap 'say "STOP signal"; exit 2' HUP INT TERM

# TRIMMED is set by trim to $1 without leading and trailing white space.
TRIMMED=
trim() {
	TRIMMED=$1
	TRIMMED=${TRIMMED#"${TRIMMED%%[![:space:]]*}"}
	TRIMMED=${TRIMMED%"${TRIMMED##*[![:space:]]}"}
}

# FENCE_OPEN tracks fenced code blocks the way the guard tests do
# (internal/doctestutil): a line that starts with ``` opens a block, and a
# line of three or more backquotes alone closes it. track_fence returns 0 when
# the line is inside a block or is a fence.
FENCE_OPEN=0
track_fence() {
	trim "$1"
	if [ "$FENCE_OPEN" = 0 ]; then
		case $TRIMMED in
		'```'*)
			FENCE_OPEN=1
			return 0
			;;
		esac
		return 1
	fi
	case $TRIMMED in
	'```'*)
		case ${TRIMMED//\`/} in
		'') FENCE_OPEN=0 ;;
		esac
		;;
	esac
	return 0
}

# --- Inputs ------------------------------------------------------------------

if [ "$#" -ne 1 ]; then
	stop usage "usage: lightweight-check.sh <spec-file>"
fi
SPEC=$1
case $SPEC in
/*) ;;
*) SPEC=$PWD/$SPEC ;;
esac
if [ ! -f "$SPEC" ] || [ ! -r "$SPEC" ]; then
	stop spec "the spec file is not a readable file"
fi

TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/lightweight-check.XXXXXX") || {
	TMP_DIR=
	stop tmp "cannot create a temporary directory"
}
# An absolute, physical path, so the cleanup still finds it after the cd
# below and git lists the worktree under the same path.
RESOLVED_TMP_DIR=$(cd -- "$TMP_DIR" && pwd -P) || stop tmp "cannot resolve the temporary directory"
TMP_DIR=$RESOLVED_TMP_DIR

TOP=$(g rev-parse --show-toplevel) || stop git "git rev-parse --show-toplevel failed"
cd "$TOP" || stop git "cannot change to the repository root"
# Every later git reads attributes from the empty tree instead of the work
# tree or the index (GIT_ATTR_SOURCE, the --attr-source option's variable,
# which subprocesses of git inherit), so a .gitattributes the branch adds at
# any depth cannot change which files count as binary, and no checkout or add
# of the reproduction runs a filter (such as a Git LFS smudge) the
# repository's configuration defines.
EMPTY_TREE=$(g hash-object -t tree /dev/null) || stop git "git hash-object failed"
g --attr-source="$EMPTY_TREE" rev-parse --git-dir >/dev/null 2>&1 ||
	stop git "this git does not accept --attr-source"
export GIT_ATTR_SOURCE=$EMPTY_TREE

# The repository's local attributes file is read even with GIT_ATTR_SOURCE,
# and it could mark a text file binary (a false FAIL) or a binary file text.
LOCAL_ATTRIBUTES=$(g rev-parse --git-path info/attributes) || stop git "git rev-parse --git-path failed"
if [ -s "$LOCAL_ATTRIBUTES" ]; then
	stop local-attributes "the repository has a local attributes file ($LOCAL_ATTRIBUTES)"
fi

# The SHA-256 of the spec file's bytes, so the caller can compare the file
# this script read with the spec on GitHub.
if command -v sha256sum >/dev/null 2>&1; then
	SPEC_SHA=$(sha256sum <"$SPEC") || stop sha256 "sha256sum failed"
elif command -v shasum >/dev/null 2>&1; then
	SPEC_SHA=$(shasum -a 256 <"$SPEC") || stop sha256 "shasum failed"
else
	stop sha256 "neither sha256sum nor shasum is on PATH"
fi
SPEC_SHA=${SPEC_SHA%%[!0-9a-f]*}
if [ "${#SPEC_SHA}" -ne 64 ]; then
	stop sha256 "the SHA-256 output has an unexpected form"
fi

# The type is the one line that is not blank under the one type heading line
# (TYPE_HEADING, outside code blocks), up to the next heading line; it must be
# W, D, or M exactly. Lines are compared without their trailing CR, as the spec
# procedure (_lib/lightweight-spec.md) compares them.
TYPE_HEADING='### 類型'
read_type() {
	local line headings=0 under=0 values=0 value= inside
	FENCE_OPEN=0
	while IFS= read -r line || [ -n "$line" ]; do
		line=${line%$'\r'}
		inside=1
		track_fence "$line" || inside=0
		if [ "$line" = "$TYPE_HEADING" ]; then
			if [ "$inside" = 1 ]; then
				stop type "the type heading is inside a code block"
			fi
			headings=$((headings + 1))
			under=1
			continue
		fi
		if [ "$under" = 1 ]; then
			if [ "$inside" = 0 ] && [[ $line =~ $HEADING_LINE ]]; then
				under=0
			elif [ -n "$TRIMMED" ]; then
				values=$((values + 1))
				value=$line
			fi
		fi
	done <"$SPEC"
	if [ "$headings" -ne 1 ]; then
		stop type "the spec has $headings type headings, want 1"
	fi
	if [ "$values" -ne 1 ]; then
		stop type "the type heading has $values lines that are not blank, want 1"
	fi
	case $value in
	W | D | M) TYPE=$value ;;
	*) stop type "the type is not W, D, or M" ;;
	esac
}
HEADING_LINE='^#{1,6} '
TYPE=
read_type

MAIN=$(g rev-parse --verify --quiet 'refs/remotes/origin/main^{commit}') ||
	stop git "refs/remotes/origin/main is not a commit"
HEAD_COMMIT=$(g rev-parse --verify --quiet 'HEAD^{commit}') || stop git "HEAD is not a commit"
HEAD_TREE=$(g rev-parse --verify --quiet 'HEAD^{tree}') || stop git "HEAD has no tree"
BASES=$(g merge-base --all "$MAIN" "$HEAD_COMMIT") || stop git "git merge-base failed"
case $BASES in
*[!0-9a-f]*) stop git "the branch has more than one merge base with origin/main" ;;
'') stop git "the branch has no merge base with origin/main" ;;
esac
BASE=$BASES
if [ "$BASE" = "$HEAD_COMMIT" ]; then
	stop empty "the branch has no commit after the merge base"
fi

g status --porcelain=v1 -z --untracked-files=all --ignore-submodules=none >"$TMP_DIR/status" ||
	stop git "git status failed"
if [ -s "$TMP_DIR/status" ]; then
	stop dirty "the work tree has changes or untracked files"
fi

# --- The conditions table ----------------------------------------------------

TABLE_DOC=docs/dev/conventions/requirements_process.md
TABLE_HEADING='#### Mechanically checked conditions'
g cat-file blob "$MAIN:$TABLE_DOC" >"$TMP_DIR/conditions.md" ||
	stop git "cannot read $TABLE_DOC of origin/main"

LIMIT_W_ADDED=
LIMIT_W_DELETED=
LIMIT_D_ADDED=
LIMIT_D_DELETED=
LIMIT_M_ADDED=
LIMIT_M_DELETED=
LIMIT_FILES=
EXCLUDES=()
TOOLS=()

RE_LIMIT='^limit ([WDM]) (added|deleted) ([0-9]{1,9}|none)$'
RE_FILES='^limit files ([0-9]{1,9})$'
RE_EXCLUDE='^(exclude|exclude-absent) ([A-Za-z0-9._/-]+)$'
RE_TOOL='^tool [!-~]+( [!-~]+)*$'

table_row() {
	local row=$1 path value old
	case $SEEN_ROWS in
	*"|$row|"*) stop table "the conditions table repeats a row" ;;
	esac
	SEEN_ROWS="$SEEN_ROWS$row|"
	if [[ $row =~ $RE_LIMIT ]]; then
		# Each limit variable is named in the code, not built from the row's
		# text, so no row can choose what is assigned.
		value=${BASH_REMATCH[3]}
		case ${BASH_REMATCH[1]}_${BASH_REMATCH[2]} in
		W_added) old=$LIMIT_W_ADDED LIMIT_W_ADDED=$value ;;
		W_deleted) old=$LIMIT_W_DELETED LIMIT_W_DELETED=$value ;;
		D_added) old=$LIMIT_D_ADDED LIMIT_D_ADDED=$value ;;
		D_deleted) old=$LIMIT_D_DELETED LIMIT_D_DELETED=$value ;;
		M_added) old=$LIMIT_M_ADDED LIMIT_M_ADDED=$value ;;
		M_deleted) old=$LIMIT_M_DELETED LIMIT_M_DELETED=$value ;;
		*) stop table "the conditions table has a limit row of an unknown kind" ;;
		esac
		if [ -n "$old" ]; then
			stop table "the conditions table has two rows for one limit"
		fi
	elif [[ $row =~ $RE_FILES ]]; then
		if [ -n "$LIMIT_FILES" ]; then
			stop table "the conditions table has two rows for the files limit"
		fi
		LIMIT_FILES=${BASH_REMATCH[1]}
	elif [[ $row =~ $RE_EXCLUDE ]]; then
		path=${BASH_REMATCH[2]}
		case /$path in
		//* | *//* | */./* | */../* | */. | */..)
			stop table "the conditions table has a path that is not from the repository root"
			;;
		esac
		EXCLUDES+=("${path%/}")
	elif [[ $row =~ $RE_TOOL ]]; then
		TOOLS+=("${row#tool }")
	else
		stop table "the conditions table has a row of no known form"
	fi
}

read_table() {
	local line state=before headings=0 inside
	SEEN_ROWS='|'
	FENCE_OPEN=0
	while IFS= read -r line || [ -n "$line" ]; do
		inside=1
		track_fence "$line" || inside=0
		case $state in
		heading)
			if [ -z "$TRIMMED" ]; then
				continue
			fi
			if [ "$line" != '```text' ]; then
				stop table "the conditions heading is not followed directly by a text code block"
			fi
			state=rows
			continue
			;;
		rows)
			if [ "$FENCE_OPEN" = 0 ]; then
				if [ "$line" != '```' ]; then
					stop table "the conditions table does not end with a closing fence alone"
				fi
				state=after
				continue
			fi
			if [ -z "$line" ]; then
				stop table "the conditions table has a blank row"
			fi
			table_row "$line"
			continue
			;;
		esac
		if [ "$inside" = 0 ] && [ "$TRIMMED" = "$TABLE_HEADING" ]; then
			headings=$((headings + 1))
			if [ "$state" = before ]; then
				state=heading
			fi
		fi
	done <"$TMP_DIR/conditions.md"
	if [ "$headings" -ne 1 ]; then
		stop table "the conditions heading appears $headings times outside code blocks, want 1"
	fi
	if [ "$state" != after ]; then
		stop table "the conditions table is missing or not closed"
	fi
	local var
	for var in LIMIT_W_ADDED LIMIT_W_DELETED LIMIT_D_ADDED LIMIT_D_DELETED \
		LIMIT_M_ADDED LIMIT_M_DELETED LIMIT_FILES; do
		if [ -z "${!var}" ]; then
			stop table "the conditions table has no row for $var"
		fi
	done
	if [ "${#EXCLUDES[@]}" -eq 0 ]; then
		stop table "the conditions table has no exclude row"
	fi
}
read_table

# --- Tool commits ------------------------------------------------------------

# message_of writes the message of commit $1 to $2: what follows the first
# empty line of the raw commit object. git cat-file is plumbing, so no log
# setting (such as log.showSignature) can add a line to it.
message_of() {
	local line body=0
	g cat-file commit "$1" >"$TMP_DIR/commit" || return 1
	: >"$2"
	while IFS= read -r line || [ -n "$line" ]; do
		if [ "$body" = 1 ]; then
			printf '%s\n' "$line" >>"$2"
		elif [ -z "$line" ]; then
			body=1
		fi
	done <"$TMP_DIR/commit"
}

# parents_of sets PARENTS to the parent lines of commit $1's raw object.
parents_of() {
	local line
	PARENTS=
	g cat-file commit "$1" >"$TMP_DIR/commit" || return 1
	while IFS= read -r line; do
		case $line in
		'') break ;;
		'parent '*) PARENTS="$PARENTS${line#parent } " ;;
		esac
	done <"$TMP_DIR/commit"
}

TRAILER='Lightweight-Generated-By: '
RE_PATH_WORD='^[A-Za-z0-9._/-]+$'

# check_tool_command sets TOOL_WORDS to the words of the trailer command $1
# when its leading words equal a tool row's words one for one and each of the
# remaining words is a path inside the repository. Otherwise it stops.
check_tool_command() {
	local cmd=$1 row matched= i word
	local -a row_words
	read -r -a TOOL_WORDS <<<"$cmd"
	for row in "${TOOLS[@]}"; do
		read -r -a row_words <<<"$row"
		if [ "${#TOOL_WORDS[@]}" -lt "${#row_words[@]}" ]; then
			continue
		fi
		matched=1
		for i in "${!row_words[@]}"; do
			if [ "${TOOL_WORDS[$i]}" != "${row_words[$i]}" ]; then
				matched=
				break
			fi
		done
		if [ -n "$matched" ]; then
			break
		fi
	done
	if [ -z "$matched" ]; then
		stop tool-allowlist "a trailer command matches no tool row of the conditions table"
	fi
	for ((i = ${#row_words[@]}; i < ${#TOOL_WORDS[@]}; i++)); do
		word=${TOOL_WORDS[$i]}
		if ! [[ $word =~ $RE_PATH_WORD ]]; then
			stop tool-allowlist "a trailer command has a word that is not a plain path"
		fi
		case /$word/ in
		//* | */../*)
			stop tool-allowlist "a trailer command has a word that is not a path inside the repository"
			;;
		esac
	done
}

COMMITS=$(g rev-list --reverse --topo-order "$BASE..$HEAD_COMMIT") || stop git "git rev-list failed"
TOOL=$BASE
TOOL_COMMITS=()
TOOL_COMMANDS=()
SEEN_HAND=
for commit in $COMMITS; do
	message_of "$commit" "$TMP_DIR/message" || stop git "git cat-file commit failed"
	trailers=0
	command_line=
	while IFS= read -r line; do
		case $line in
		"$TRAILER"*)
			trailers=$((trailers + 1))
			command_line=${line#"$TRAILER"}
			;;
		esac
	done <"$TMP_DIR/message"
	if [ "$trailers" = 0 ]; then
		SEEN_HAND=1
		continue
	fi
	if [ "$trailers" -gt 1 ]; then
		stop tool-order "a commit has $trailers trailer lines, want 1"
	fi
	if [ "$TYPE" != M ]; then
		stop tool-order "a type $TYPE branch has a tool commit"
	fi
	if [ -n "$SEEN_HAND" ]; then
		stop tool-order "a tool commit follows a hand-written commit"
	fi
	parents_of "$commit" || stop git "git cat-file commit failed"
	if [ "$PARENTS" != "$TOOL " ]; then
		stop tool-order "a tool commit is not the child of BASE or of the previous tool commit"
	fi
	check_tool_command "$command_line"
	TOOL=$commit
	TOOL_COMMITS+=("$commit")
	TOOL_COMMANDS+=("$command_line")
done

say "type=$TYPE spec-sha256=$SPEC_SHA base=$BASE tool=$TOOL tree=$HEAD_TREE"

# Each tool commit must be what its command makes of its parent: run the
# command, as words and without a shell, in a temporary worktree of the parent
# and compare the resulting tree with the commit's.
for i in "${!TOOL_COMMITS[@]}"; do
	commit=${TOOL_COMMITS[$i]}
	check_tool_command "${TOOL_COMMANDS[$i]}"
	WORKTREE=$TMP_DIR/worktree
	g worktree add --quiet --detach "$WORKTREE" "$commit^" >/dev/null 2>&1 ||
		stop git "git worktree add failed"
	# Resolve the tool before changing into the worktree, so an empty or
	# relative PATH entry cannot run a file of the worktree.
	tool_path=$(command -v -- "${TOOL_WORDS[0]}") || stop tool-failed "the tool ${TOOL_WORDS[0]} is not on PATH"
	case $tool_path in
	/*) ;;
	*) stop tool-failed "the tool ${TOOL_WORDS[0]} does not resolve to an absolute path" ;;
	esac
	(cd "$WORKTREE" && "$tool_path" "${TOOL_WORDS[@]:1}") </dev/null >"$TMP_DIR/tool-output" 2>&1 ||
		stop tool-failed "the command of tool commit $commit failed"
	g -C "$WORKTREE" add -A >/dev/null 2>&1 || stop git "git add in the worktree failed"
	got=$(g -C "$WORKTREE" write-tree) || stop git "git write-tree in the worktree failed"
	want=$(g rev-parse --verify --quiet "$commit^{tree}") || stop git "git rev-parse failed"
	g worktree remove --force "$WORKTREE" >/dev/null 2>&1 || stop git "git worktree remove failed"
	WORKTREE=
	if [ "$got" != "$want" ]; then
		stop tool-mismatch "running the command of tool commit $commit again gives tree $got, not $want"
	fi
done

# --- Facts about the diff ----------------------------------------------------

DETAILS=()
R_BINARY=
R_SYMLINK=
R_SUBMODULE=
R_MODE=
R_EXCLUDED=

# excluded reports whether path $1 is an excluded path or under one, compared
# byte for byte as whole path elements, never as a string prefix.
excluded() {
	local path=$1 exclude
	for exclude in "${EXCLUDES[@]}"; do
		if [ "$path" = "$exclude" ]; then
			return 0
		fi
		case $path in
		"$exclude"/*) return 0 ;;
		esac
	done
	return 1
}

g diff --raw "${DIFF_OPTIONS[@]}" "$BASE" "$HEAD_COMMIT" >"$TMP_DIR/raw" || stop git "git diff --raw failed"
while IFS= read -r -d '' header && IFS= read -r -d '' path; do
	header=${header#:}
	old_mode=${header%% *}
	rest=${header#* }
	new_mode=${rest%% *}
	shown=$(printable "$path")
	if [ "$old_mode" = 120000 ] || [ "$new_mode" = 120000 ]; then
		R_SYMLINK=1
		DETAILS+=("symlink $shown")
	elif [ "$old_mode" = 160000 ] || [ "$new_mode" = 160000 ]; then
		R_SUBMODULE=1
		DETAILS+=("submodule $shown")
	elif [ "$old_mode" != 000000 ] && [ "$new_mode" != 000000 ] && [ "$old_mode" != "$new_mode" ]; then
		R_MODE=1
		DETAILS+=("mode $shown $old_mode $new_mode")
	fi
	if excluded "$path"; then
		R_EXCLUDED=1
		DETAILS+=("excluded $shown")
	fi
done <"$TMP_DIR/raw"

g diff --numstat "${DIFF_OPTIONS[@]}" "$BASE" "$HEAD_COMMIT" >"$TMP_DIR/numstat-all" ||
	stop git "git diff --numstat failed"
while IFS= read -r -d '' record; do
	if [ "${record%%$'\t'*}" = - ]; then
		R_BINARY=1
		rest=${record#*$'\t'}
		DETAILS+=("binary $(printable "${rest#*$'\t'}")")
	fi
done <"$TMP_DIR/numstat-all"

g diff --numstat "${DIFF_OPTIONS[@]}" "$TOOL" "$HEAD_COMMIT" >"$TMP_DIR/numstat-hand" ||
	stop git "git diff --numstat failed"
ADDED=0
DELETED=0
FILES=0
while IFS= read -r -d '' record; do
	FILES=$((FILES + 1))
	added=${record%%$'\t'*}
	rest=${record#*$'\t'}
	deleted=${rest%%$'\t'*}
	case $added$deleted in
	-*) continue ;;
	*[!0-9]*) stop git "git diff --numstat gave a count that is not a number" ;;
	esac
	ADDED=$((ADDED + added))
	DELETED=$((DELETED + deleted))
done <"$TMP_DIR/numstat-hand"

case $TYPE in
W) LIMIT_ADDED=$LIMIT_W_ADDED LIMIT_DELETED=$LIMIT_W_DELETED ;;
D) LIMIT_ADDED=$LIMIT_D_ADDED LIMIT_DELETED=$LIMIT_D_DELETED ;;
M) LIMIT_ADDED=$LIMIT_M_ADDED LIMIT_DELETED=$LIMIT_M_DELETED ;;
*) stop type "the type is not W, D, or M" ;;
esac

# over reports whether count $1 is above limit $2, where "none" is no limit.
over() {
	[ "$2" != none ] && [ "$1" -gt "$((10#$2))" ]
}

for detail in "${DETAILS[@]}"; do
	say "$detail"
done
say "added=$ADDED/$LIMIT_ADDED deleted=$DELETED/$LIMIT_DELETED files=$FILES/$LIMIT_FILES"

REASONS=
add_reason() {
	REASONS=${REASONS:+$REASONS,}$1
}
if over "$ADDED" "$LIMIT_ADDED"; then
	say "added $ADDED exceeds the limit $LIMIT_ADDED"
	add_reason added
fi
if over "$DELETED" "$LIMIT_DELETED"; then
	say "deleted $DELETED exceeds the limit $LIMIT_DELETED"
	add_reason deleted
fi
if over "$FILES" "$LIMIT_FILES"; then
	say "files $FILES exceeds the limit $LIMIT_FILES"
	add_reason files
fi
[ -n "$R_BINARY" ] && add_reason binary
[ -n "$R_SYMLINK" ] && add_reason symlink
[ -n "$R_SUBMODULE" ] && add_reason submodule
[ -n "$R_MODE" ] && add_reason mode
[ -n "$R_EXCLUDED" ] && add_reason excluded

if [ -n "$REASONS" ]; then
	say "FAIL $REASONS"
	exit 1
fi
say PASS
exit 0
