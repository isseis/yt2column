#!/usr/bin/env bash
# high-review-decision.sh decides whether /runplan runs the high-model review
# before it opens a PR (entry 1 of .claude/commands/_lib/high-review.md).
#
# Usage: bash high-review-decision.sh <PLAN> <TARGET> <BASE>
#
#   PLAN    the plan's repository-relative path,
#           docs/tasks/<task-dir>/03_implementation_plan.md
#   TARGET  N of the PR's "### PR-N 作成ポイント" marker, or "single" for a plan
#           without PR markers
#   BASE    $(git merge-base origin/main HEAD)
#
# Output, on standard output:
#   RUN                  run the high-model review
#   NONE <reason>        do not run it; <reason> is value-low, value-absent,
#                        or all-high
#   SKIP <done-line>     skip it: <done-line> is the "High review: done" (or
#                        legacy "Opus review: done") record line that covers
#                        the branch, followed by one line
#                        "<12-hex-id> <subject>" for each commit after its END,
#                        oldest first
#
# Contract: every failure (a git error, an unreadable plan, a bad argument,
# missing or malformed origin/main:.claude/commands/_context.md, two places
# that could hold the value) prints RUN, writes the reason to standard error,
# and exits 0. When in doubt, the decision falls on the side that runs the
# review. The script reads git's object database only (no work tree file),
# touches no network, and changes nothing.
#
# The rules this script executes are defined elsewhere, and those definitions
# win over this file:
#   - where the value is, how it is read, the old-value table, and the
#     Harness model mapping table: .claude/commands/_context.md, "PR marker
#     conventions" and "Harness model mapping" (both read from origin/main's
#     copy of that file, never held here);
#   - the record-line forms, the excludable commits, and the decision steps:
#     .claude/commands/_lib/high-review.md, "Record lines" and "Entry 1".
#
# Japanese literals: the heading and value-line texts below must equal the
# text of the Japanese plans this script reads (CLAUDE.md, "Go Idioms").

# Run again in a clean environment, so nothing the caller's environment
# carries (exported functions, SHELLOPTS, BASHOPTS, LC_ALL, GIT_ variables,
# BASH_ENV) reaches the shell or git: only PATH, HOME, and TMPDIR are kept.
# The marker is this process's ID, which exec keeps, so a caller cannot skip
# the run by presetting it; [[ is a keyword no function can replace.
if [[ ${HIGH_REVIEW_DECISION_CLEAN_ENV-} != "$$" ]]; then
	exec env -i HIGH_REVIEW_DECISION_CLEAN_ENV=$$ PATH="${PATH:-}" HOME="${HOME:-}" TMPDIR="${TMPDIR:-}" \
		"${BASH:-bash}" --noprofile --norc "$0" "$@" </dev/null
fi
unset HIGH_REVIEW_DECISION_CLEAN_ENV

set -u -o pipefail
export LC_ALL=C

for name in ${!GIT_@}; do
	unset "$name"
done
unset -f git awk grep tr tail printf
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_ATTR_NOSYSTEM=1
export GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0 GIT_NO_REPLACE_OBJECTS=1 GIT_NO_LAZY_FETCH=1

GIT_OVERRIDES=(
	-c core.attributesFile=/dev/null
	-c core.quotePath=true
	-c diff.relative=false
	-c diff.noprefix=false
	-c color.ui=false
	-c log.showSignature=false
	-c i18n.logOutputEncoding=UTF-8
)

g() {
	command git "${GIT_OVERRIDES[@]}" "$@"
}

run_on_failure() {
	printf 'high-review-decision: %s\n' "$1" >&2
	printf 'RUN\n'
	exit 0
}

[ "${BASH_VERSINFO[0]:-0}" -ge 4 ] || run_on_failure "bash 4 or later is required"
[ "$#" -eq 3 ] || run_on_failure "expected 3 arguments, got $#"
PLAN=$1
TARGET=$2
BASE_ARG=$3

[[ $PLAN =~ ^docs/tasks/[^/]+/03_implementation_plan\.md$ ]] ||
	run_on_failure "PLAN is not docs/tasks/<task-dir>/03_implementation_plan.md"
[[ $TARGET == single || $TARGET =~ ^[1-9][0-9]*$ ]] ||
	run_on_failure "TARGET is neither a PR number nor single"

TOP=$(g rev-parse --show-toplevel 2>/dev/null) || run_on_failure "not in a git work tree"
cd "$TOP" || run_on_failure "cannot enter the top of the work tree"

OM=$(g rev-parse --verify --quiet 'refs/remotes/origin/main^{commit}') ||
	run_on_failure "refs/remotes/origin/main names no commit"
HEAD_ID=$(g rev-parse --verify --quiet 'HEAD^{commit}') || run_on_failure "HEAD names no commit"
BASE=$(g rev-parse --verify --quiet "${BASE_ARG}^{commit}") || run_on_failure "BASE names no commit"
# BASE must be the merge-base itself: a narrower range (such as a phase
# group's PHASE_START) would leave commits out of the decision.
MB=$(g merge-base "$OM" "$HEAD_ID") || run_on_failure "origin/main and HEAD have no merge-base"
[ "$BASE" = "$MB" ] || run_on_failure "BASE is not the merge-base of origin/main and HEAD"

# --- Step 0: validate origin/main:_context.md before any early return -------

# Contract: validate origin/main's _context.md and its entire Harness model
# mapping table fail-closed before any early value-low/value-absent return,
# and extract only Role == high && Launch == agent verification patterns so a
# non-self-verifying harness (Launch == swarm) can never satisfy all-high.
CONTEXT_DOC=$(g cat-file blob "$OM:.claude/commands/_context.md" 2>/dev/null) ||
	run_on_failure "cannot read origin/main:.claude/commands/_context.md"

parse_harness_mapping() {
	awk '
		function fail(msg) {
			if (!failed) {
				printf "ERR\t%s\n", msg
				failed = 1
			}
		}
		failed { next }
		{ sub(/\r$/, "") }
		{
			t = $0
			if (t ~ /^ ? ? ?[^ \t]/ || t == "") sub(/^ +/, "", t); else t = ""
			c = substr(t, 1, 1)
			if (substr(t, 1, 3) == "```" || substr(t, 1, 3) == "~~~") {
				len = 0
				while (substr(t, len + 1, 1) == c) len++
				rest = substr(t, len + 1)
				if (!fence) {
					if (table_state == 1) table_state = 2
					fence = 1; fchar = c; flen = len; next
				}
				if (c == fchar && len >= flen && rest ~ /^[ \t]*$/) { fence = 0; next }
			}
		}
		fence { next }
		/^#+[ \t]/ {
			if ($0 ~ /^### Harness model mapping[ \t]*$/) {
				sec_count++
				if (sec_count > 1) fail("duplicate Harness model mapping section")
				in_sec = 1
				table_state = 0
				row_num = 0
			} else {
				in_sec = 0
			}
			next
		}
		!in_sec { next }
		/^[ \t]*\|/ {
			if ($0 !~ /^\|/) { fail("indented table row in Harness model mapping"); next }
			if (table_state == 2) { fail("multiple tables in Harness model mapping"); next }
			table_state = 1
			row_num++
			if (row_num == 1) {
				if ($0 != "| Harness | Role | Launch | Model | Verification pattern |") {
					fail("invalid Harness model mapping header")
				}
				next
			}
			if (row_num == 2) {
				if ($0 != "|---|---|---|---|---|") {
					fail("invalid Harness model mapping separator")
				}
				next
			}
			if ($0 !~ /^\| / || $0 !~ / \|$/) {
				fail("malformed Harness model mapping row")
				next
			}
			inner = substr($0, 3, length($0) - 4)
			n = split(inner, cells, " \\| ")
			if (n != 5) {
				fail("Harness model mapping row must have 5 columns")
				next
			}
			for (i = 1; i <= 5; i++) {
				cell = cells[i]
				if (length(cell) < 3 || substr(cell, 1, 1) != "`" || substr(cell, length(cell), 1) != "`") {
					fail("Harness model mapping cell must be a single code span")
					next
				}
				val[i] = substr(cell, 2, length(cell) - 2)
				if (index(val[i], "`") > 0 || index(val[i], "|") > 0) {
					fail("Harness model mapping cell contains forbidden character")
					next
				}
			}
			harness = val[1]
			role = val[2]
			launch = val[3]
			model = val[4]
			pat = val[5]
			if (harness != "Claude Code" && harness != "Antigravity" && harness != "OpenCode") {
				fail("unknown Harness in Harness model mapping: " harness)
				next
			}
			if (role != "high" && role != "standard" && role != "low" && role != "cross-check") {
				fail("unknown Role in Harness model mapping: " role)
				next
			}
			if (launch != "agent" && launch != "swarm" && launch != "none") {
				fail("unknown Launch in Harness model mapping: " launch)
				next
			}
			if (model !~ /^[ -~]+$/ || model !~ /[^ ]/) {
				fail("invalid Model in Harness model mapping")
				next
			}
			pair = harness SUBSEP role
			if (seen[pair]++) {
				fail("duplicate (Harness, Role) in Harness model mapping: " harness "/" role)
				next
			}
			if (launch == "none") {
				if (pat != "-") {
					fail("Launch none row must have Verification pattern -")
					next
				}
			} else {
				if (pat == "-") {
					fail("Launch " launch " row cannot have Verification pattern -")
					next
				}
				if (substr(pat, length(pat), 1) == "*") {
					pfx = substr(pat, 1, length(pat) - 1)
					if (length(pfx) < 1 || length(pfx) > 63 || pfx !~ /^[a-z0-9][a-z0-9.-]*$/) {
						fail("invalid prefix Verification pattern in Harness model mapping")
						next
					}
				} else {
					if (length(pat) < 1 || length(pat) > 64 || pat !~ /^[a-z0-9][a-z0-9.-]*$/) {
						fail("invalid literal Verification pattern in Harness model mapping")
						next
					}
				}
			}
			if (role == "high" && launch == "agent") {
				high_pats[++high_count] = pat
			}
			next
		}
		table_state == 1 { table_state = 2 }
		END {
			if (failed) exit 0
			if (fence) fail("unclosed code fence in _context.md")
			else if (sec_count == 0) fail("missing Harness model mapping section")
			else if (sec_count > 1) fail("duplicate Harness model mapping section")
			else if (row_num < 3) fail("missing or empty Harness model mapping table")
			else if (high_count == 0) fail("no high-role agent verification pattern in Harness model mapping")
			if (failed) exit 0
			for (i = 1; i <= high_count; i++) {
				printf "PAT\t%s\n", high_pats[i]
			}
		}
	' <<<"$CONTEXT_DOC"
}

MAPPING_OUT=$(parse_harness_mapping) || run_on_failure "failed to parse Harness model mapping"
HIGH_AGENT_PATTERNS=()
while IFS= read -r mline; do
	case $mline in
	ERR$'\t'*)
		run_on_failure "${mline#ERR$'\t'}"
		;;
	PAT$'\t'*)
		HIGH_AGENT_PATTERNS+=("${mline#PAT$'\t'}")
		;;
	"") ;;
	*)
		run_on_failure "unexpected parser output for Harness model mapping"
		;;
	esac
done <<<"$MAPPING_OUT"
[ "${#HIGH_AGENT_PATTERNS[@]}" -gt 0 ] ||
	run_on_failure "no high-role agent verification pattern in Harness model mapping"

# matches_pattern checks id against a validated literal or trailing-* pattern
# without regex or glob evaluation: "." matches only a literal ".", and a
# trailing-* suffix must match ^[a-z0-9.-]{1,48}$.
matches_pattern() {
	local id=$1 pat=$2 pfx suffix
	if [[ $pat == *'*' ]]; then
		pfx=${pat%\*}
		((${#id} > ${#pfx})) || return 1
		[[ "${id:0:${#pfx}}" == "$pfx" ]] || return 1
		suffix=${id:${#pfx}}
		((${#suffix} >= 1 && ${#suffix} <= 48)) || return 1
		[[ $suffix =~ ^[a-z0-9.-]+$ ]] || return 1
		return 0
	fi
	[[ "$id" == "$pat" ]]
}

matches_high_agent() {
	local id=$1 pat
	for pat in "${HIGH_AGENT_PATTERNS[@]}"; do
		if matches_pattern "$id" "$pat"; then
			return 0
		fi
	done
	return 1
}

# --- Step 1: read the value -------------------------------------------------

# locate_value prints one of: VALUE<TAB><text>, ABSENT, NOSECTION, MULTIPLE.
# It skips fenced code blocks (``` or ~~~, nested ones included), so a diff
# record's content is never read as a heading or as a value line; an unclosed
# fence, or (for "single") a PR marker heading, prints MULTIPLE, a failure.
locate_value() {
	awk -v target="$TARGET" '
		function trim(s) { sub(/^[ \t\r]+/, "", s); sub(/[ \t\r]+$/, "", s); return s }
		{ sub(/\r$/, "") }
		# A fence marker is indented by at most three spaces (no tab). A fence
		# closes only on a line of the same character, at least as long as
		# the opening run, with nothing after it; a fence-like line inside a
		# fence is content. A value line inside a fence in the target section
		# is a failure, since a fence the parse got wrong could hide it.
		{
			t = $0
			if (t ~ /^ ? ? ?[^ \t]/ || t == "") sub(/^ +/, "", t); else t = ""
			c = substr(t, 1, 1)
			if (substr(t, 1, 3) == "```" || substr(t, 1, 3) == "~~~") {
				len = 0
				while (substr(t, len + 1, 1) == c) len++
				rest = substr(t, len + 1)
				if (!fence) { fence = 1; fchar = c; flen = len; next }
				if (c == fchar && len >= flen && rest ~ /^[ \t]*$/) { fence = 0; next }
			}
		}
		fence && insec && index($0, "**実装モデル要件**: ") == 1 { fenced++ }
		fence { next }
		target == "single" && /^### PR-[0-9]+ 作成ポイント/ { markers++ }
		/^#+[ \t]/ {
			level = match($0, /[^#]/) - 1
			if (target == "single") {
				if (level <= 2) {
					in3 = ($0 ~ /^## 3([.]|[ \t])/)
					insec = 0
					next
				}
				text = $0
				sub(/^#+[ \t]+/, "", text)
				sub(/^[0-9]+([.][0-9]+)*[.]?[ \t]+/, "", text)
				insec = (in3 && trim(text) == "PR 構成")
			} else {
				insec = ($0 ~ ("^### PR-" target " 作成ポイント(:|[ \t]*$)"))
			}
			if (insec) sections++
			next
		}
		insec && index($0, "**実装モデル要件**: ") == 1 {
			values++
			value = trim(substr($0, length("**実装モデル要件**: ") + 1))
		}
		END {
			if (fence || markers || fenced) print "MULTIPLE"
			else if (sections == 0) print "NOSECTION"
			else if (sections > 1 || values > 1) print "MULTIPLE"
			else if (values == 0) print "ABSENT"
			else printf "VALUE\t%s\n", value
		}
	'
}

# old_value_table prints "<old> <new>" for each row of the old-value table in
# origin/main's _context.md: a two-cell row, inside "### PR marker
# conventions" outside fenced code blocks, whose second cell is one of the
# three canonical values. An old value with a blank or with two rows is
# dropped, so it reads as unreadable.
old_value_table() {
	awk '
		{ sub(/\r$/, "") }
		{
			t = $0
			if (t ~ /^ ? ? ?[^ \t]/ || t == "") sub(/^ +/, "", t); else t = ""
			c = substr(t, 1, 1)
			if (substr(t, 1, 3) == "```" || substr(t, 1, 3) == "~~~") {
				len = 0
				while (substr(t, len + 1, 1) == c) len++
				rest = substr(t, len + 1)
				if (!fence) { fence = 1; fchar = c; flen = len; next }
				if (c == fchar && len >= flen && rest ~ /^[ \t]*$/) { fence = 0; next }
			}
		}
		fence { next }
		/^#+[ \t]/ { insec = ($0 ~ /^### PR marker conventions[ \t]*$/); next }
		insec && match($0, /^[ \t]*\| `[^`]+` \| `(high|standard\+high-review|low)` \|[ \t]*$/) {
			line = $0
			sub(/^[ \t]*\| `/, "", line)
			old = line; sub(/`.*/, "", old)
			if (old ~ /[ \t]/) next
			new = line; sub(/^[^`]*` \| `/, "", new); sub(/`.*/, "", new)
			count[old]++
			map[old] = new
		}
		END { for (o in map) if (count[o] == 1) print o, map[o] }
	' <<<"$CONTEXT_DOC"
}

# classify reads one copy of the plan and prints high, standard+high-review,
# low, unreadable, absent, or (for the failures) NOSECTION or MULTIPLE.
classify() {
	local result text old new
	result=$(locate_value)
	case $result in
	NOSECTION | MULTIPLE | ABSENT)
		[ "$result" = ABSENT ] && printf 'absent\n' || printf '%s\n' "$result"
		return
		;;
	esac
	text=${result#VALUE$'\t'}
	case $text in
	high | standard+high-review | low)
		printf '%s\n' "$text"
		return
		;;
	esac
	# An empty table still yields one empty line, which must not match an
	# empty value.
	while read -r old new; do
		if [ -n "$old" ] && [ "$old" = "$text" ]; then
			printf '%s\n' "$new"
			return
		fi
	done <<<"$OLD_TABLE"
	printf 'unreadable\n'
}

OLD_TABLE=$(old_value_table) || run_on_failure "failed to parse old-value table"

# check_regular_plan_file verifies that PLAN at rev is a regular file (mode
# 100644 or 100755). When required=1, an absent path fails closed; when
# required=0, an absent path returns 1. Any non-regular entry (such as a
# symlink, mode 120000) or git error fails closed.
check_regular_plan_file() {
	local rev=$1 label=$2 required=$3 entry mode type
	entry=$(g ls-tree "$rev" -- "$PLAN") || run_on_failure "cannot inspect $label tree for PLAN"
	if [ -z "$entry" ]; then
		if [ "$required" = 1 ]; then
			run_on_failure "cannot read PLAN at $label"
		fi
		return 1
	fi
	[[ $entry != *$'\n'* ]] || run_on_failure "PLAN at $label matched multiple tree entries"
	IFS=' ' read -r mode type _ <<<"$entry"
	case "$mode $type" in
	"100644 blob" | "100755 blob") return 0 ;;
	*) run_on_failure "PLAN at $label is not a regular file (mode $mode)" ;;
	esac
}

check_regular_plan_file "$HEAD_ID" HEAD 1
HEAD_PLAN=$(g cat-file blob "$HEAD_ID:$PLAN" 2>/dev/null) || run_on_failure "cannot read PLAN at HEAD"
HEAD_READ=$(printf '%s\n' "$HEAD_PLAN" | classify) || run_on_failure "cannot classify PLAN at HEAD"
case $HEAD_READ in
NOSECTION)
	if [ "$TARGET" = single ]; then
		HEAD_READ=absent
	else
		run_on_failure "HEAD's plan has no PR-$TARGET marker"
	fi
	;;
MULTIPLE) run_on_failure "HEAD's plan has more than one place for the value" ;;
esac

# origin/main's copy counts only when it has the plan and the target place;
# without them the decision rests on HEAD alone. Distinguish an absent path
# from a present path whose blob cannot be read or is not a regular file (fail
# closed).
OM_READ=absent
if check_regular_plan_file "$OM" origin/main 0; then
	OM_PLAN=$(g cat-file blob "$OM:$PLAN" 2>/dev/null) || run_on_failure "cannot read PLAN at origin/main"
	OM_READ=$(printf '%s\n' "$OM_PLAN" | classify) || run_on_failure "cannot classify PLAN at origin/main"
	case $OM_READ in
	NOSECTION) OM_READ=absent ;;
	MULTIPLE) run_on_failure "origin/main's plan has more than one place for the value" ;;
	esac
fi

needs_review() {
	case $1 in high | standard+high-review | unreadable) return 0 ;; esac
	return 1
}
if ! needs_review "$HEAD_READ" && ! needs_review "$OM_READ"; then
	if [ "$HEAD_READ" = low ] || [ "$OM_READ" = low ]; then
		printf 'NONE value-low\n'
	else
		printf 'NONE value-absent\n'
	fi
	exit 0
fi

# --- Commits, messages, and excludable commits ------------------------------

COMMITS=$(g rev-list "$BASE..$HEAD_ID") || run_on_failure "git rev-list failed"

# message prints a commit's message; the commit object is read directly, so
# no log configuration (such as signature display) changes it.
message() {
	g cat-file commit "$1" | awk 'body { print; next } /^$/ { body = 1 }'
}

parent_count() {
	g cat-file commit "$1" | awk '!body && /^$/ { body = 1; next } !body && /^parent / { n++ } END { print n + 0 }'
}

# excludable: one parent, and either empty or changing only PLAN and
# README.en.md. Preserves NUL delimiters from diff-tree -z and fails closed
# on any git error instead of treating it as a normal non-excludable commit.
excludable() {
	local c=$1 pc path prev="" have_prev=0 excl=1
	pc=$(parent_count "$c") || run_on_failure "cannot read commit $c"
	[ "$pc" = 1 ] || return 1
	while IFS= read -r -d '' path; do
		if [ "$have_prev" = 1 ]; then
			if [ "$prev" != "$PLAN" ] && [ "$prev" != README.en.md ]; then
				excl=0
			fi
		fi
		prev=$path
		have_prev=1
	done < <(
		g diff-tree -z --no-commit-id --name-only -r --no-renames "$c"
		printf '%d\0' "$?"
	)
	[ "$have_prev" = 1 ] && [ "$prev" = 0 ] || run_on_failure "git diff-tree failed for $c"
	[ "$excl" = 1 ]
}

# An ID is printable ASCII without a space or a comma.
ID='[!-+.-~-]+'
IMPL_RE="^Implementation model: ([0-9a-f]{12})\\.\\.([0-9a-f]{12}) ($ID(, $ID)*)\$"
DONE_RE='^(High|Opus) review: done ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(critical [0-9]+, major [0-9]+, minor [0-9]+\)$'

resolve12() {
	g rev-parse --verify --quiet "$1^{commit}" 2>/dev/null
}

# --- Step 2: a done record that still covers the branch ---------------------

# The last done record is the last matching line of the newest commit that
# has one. Its START must be BASE or an ancestor of it, its END an ancestor of
# HEAD, and every commit after END must be excludable.
DONE_LINE=""
for c in $COMMITS; do
	msg=$(message "$c") || run_on_failure "cannot read commit $c"
	last_done=""
	while IFS= read -r line; do
		if [[ $line =~ $DONE_RE ]]; then
			last_done=$line
		fi
	done <<<"$msg"
	if [ -n "$last_done" ]; then
		DONE_LINE=$last_done
		break
	fi
done
if [ -n "$DONE_LINE" ] && [[ $DONE_LINE =~ $DONE_RE ]]; then
	START=$(resolve12 "${BASH_REMATCH[2]}") || START=""
	END=$(resolve12 "${BASH_REMATCH[3]}") || END=""
	if [ -n "$START" ] && [ -n "$END" ] &&
		g merge-base --is-ancestor "$START" "$BASE" &&
		g merge-base --is-ancestor "$END" "$HEAD_ID"; then
		AFTER=$(g rev-list --reverse "$END..$HEAD_ID") || run_on_failure "git rev-list failed"
		ok=1
		for c in $AFTER; do
			excludable "$c" || {
				ok=0
				break
			}
		done
		if [ "$ok" = 1 ]; then
			AFTER_LINES=()
			for c in $AFTER; do
				entry=$(g log -1 --no-show-signature --format='%h %s' --abbrev=12 "$c") ||
					run_on_failure "git log failed"
				AFTER_LINES+=("$entry")
			done
			printf 'SKIP %s\n' "$DONE_LINE"
			for entry in "${AFTER_LINES[@]}"; do
				printf '%s\n' "$entry"
			done
			exit 0
		fi
	fi
fi

# --- Step 3: every commit covered by self-verifying high records ------------

# COVERED holds each commit inside a self-verifying high-role record's range;
# TAINTED holds each commit inside a record's range that includes any model ID
# not matching HIGH_AGENT_PATTERNS.
declare -A COVERED=() TAINTED=()
for c in $COMMITS; do
	msg=$(message "$c") || run_on_failure "cannot read commit $c"
	while IFS= read -r line; do
		[[ $line =~ $IMPL_RE ]] || continue
		START=$(resolve12 "${BASH_REMATCH[1]}") || continue
		END=$(resolve12 "${BASH_REMATCH[2]}") || continue
		all_high=1
		IFS=',' read -r -a ids <<<"${BASH_REMATCH[3]}"
		for id in "${ids[@]}"; do
			id=${id# }
			matches_high_agent "$id" || all_high=0
		done
		RANGE_COMMITS=$(g rev-list "$START..$END") || run_on_failure "git rev-list failed"
		for r in $RANGE_COMMITS; do
			if [ "$all_high" = 1 ]; then COVERED[$r]=1; else TAINTED[$r]=1; fi
		done
	done <<<"$msg"
done

for c in $COMMITS; do
	pc=$(parent_count "$c") || run_on_failure "cannot read commit $c"
	[ "$pc" = 1 ] || {
		printf 'RUN\n'
		exit 0
	}
	excludable "$c" && continue
	if [ -z "${COVERED[$c]:-}" ] || [ -n "${TAINTED[$c]:-}" ]; then
		printf 'RUN\n'
		exit 0
	fi
done
printf 'NONE all-high\n'
