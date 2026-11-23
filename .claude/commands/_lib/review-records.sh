#!/usr/bin/env bash
# review-records.sh reads the review record lines of a branch's commit
# messages and decides, by fixed rules, what a review gate needs to know.
#
# Usage: bash review-records.sh coverage <LAST_REVIEWED> <HEAD> [<PLAN>]
#        bash review-records.sh unreviewed <HEAD> [<PLAN>]
#        bash review-records.sh cross-check <ENTRY> <HEAD> <PLAN>
#        bash review-records.sh fable <ENTRY> <HEAD> <PLAN>   # migration alias
#
# The first argument chooses the mode; any other mode is a failure.
#
# Mode coverage: whether the local CLI review saw every commit after the last
# completed manual PR review (.claude/commands/runpr.md, step 3, and
# .claude/commands/_lib/local-cli-review.md, "Reading the record lines").
#
#   Before a PR opens, task-pr-handoff.md Part B step 2.5 calls coverage
#   with "none" to include the entire branch.
#
#   LAST_REVIEWED  the commit of the last completed manual PR review, 40
#                  hexadecimal characters, or "none" when there is none
#   HEAD           the PR's head, 40 hexadecimal characters
#   PLAN           optional: for an implementation PR, the task's
#                  docs/tasks/<task-dir>/03_implementation_plan.md
#
# Output, on standard output:
#   ALL            every commit judged is known to be seen
#   PARTIAL <n>    <n> commits are not known to be seen; one line
#                  "<12-hex-id> <subject>" follows for each, oldest first
#
# Contract:
#   - The commits judged are those of LAST_REVIEWED..HEAD that
#     refs/remotes/origin/main does not reach (origin/main..HEAD when
#     LAST_REVIEWED is "none"). A LAST_REVIEWED that is not an ancestor of
#     HEAD (the history was rewritten, or HEAD went back behind it) is a
#     failure: the range cannot be judged.
#   - A commit judged is known to be seen when it is any of:
#       * empty: one parent, and no difference from it;
#       * a plan checkbox commit: one parent, changing only PLAN, a regular
#         file before and after, by marking the PR progress checkboxes (the
#         "green gate confirmed", "PR created", "PR merged", and "switched to
#         the next branch" lines) of a PR marker's section, or of the
#         single-PR record of a plan without markers, the way /runplan and
#         /runpr mark them (see checkbox_only below), with the line end (LF or
#         CRLF) of every kept line unchanged; never with PLAN absent;
#       * inside the range of a seen-range line ("Local CLI reviewed:") that
#         passes every check below.
#     Any other commit, a merge commit included, is not known to be seen.
#     The "not run" lines neither cancel a seen range nor add to one, so the
#     script does not read them.
#   - A seen-range line counts only when it is a whole line, from the first
#     column, of the message of a commit in origin/main..HEAD, matching the
#     form exactly, and when: START and END each resolve to exactly one
#     commit by its ID prefix and name no ref (refs/heads, refs/tags,
#     refs/remotes); START is an ancestor of END; END is a proper ancestor
#     of the commit holding the line; START is the merge-base of origin/main
#     and HEAD or a commit of origin/main..HEAD; and END is a commit of
#     origin/main..HEAD. A line that fails a check is ignored.
#   - Inputs: git's object database and the arguments only; no work tree
#     file and nothing on standard input. Of the caller's environment only
#     PATH, HOME, and TMPDIR are kept; its GIT_ variables, exported
#     functions, shell options, and locale change nothing, nor do the global
#     and system git configuration or the attributes. Inputs left: bash
#     reads the caller's BASH_ENV file before this script starts, so what
#     that file prints would reach the output; that is the user's own
#     environment, not a value from outside the trust boundary. Commits and
#     blobs are read with plumbing commands (cat-file, diff-tree --raw,
#     rev-list), so no diff driver, text conversion, or attribute applies.
#     A submodule's change always counts (--ignore-submodules=none, so
#     neither .gitmodules nor the configuration hides one), and neither
#     replace refs nor a grafts file are followed (GIT_NO_REPLACE_OBJECTS,
#     and GIT_GRAFT_FILE set to /dev/null).
#   - Failure: any failed git step, an argument of another form, or an
#     unreadable refs/remotes/origin/main prints "PARTIAL" alone (no
#     count), a diagnostic on standard error, and exits 1. A caller reads a
#     status other than 0, or a first line of another form, as PARTIAL.
#   - It uses no network, writes no file, and uses no bash feature newer
#     than 3.2.
#
# Mode unreviewed: the commits of a PR neither local CLI reviewed, for the PR
# description (.claude/commands/_lib/task-pr-handoff.md, Part B step 3).
#
#   HEAD   the commit to be pushed, 40 hexadecimal characters
#   PLAN   optional, as for coverage
#
# unreviewed <HEAD> [<PLAN>] is coverage none <HEAD> [<PLAN>] with two
# differences, and every other rule, output, and failure as for coverage:
#   - a seen-range line's START also passes under the widened START check
#     (the fable mode's contract below), so a line read before a merge of
#     origin/main moved the merge-base still counts;
#   - a merge commit inside the range of a seen-range line that passes the
#     checks is known to be seen.
#
# Mode cross-check (alias: fable): whether the branch's last cross-check
# partial review record still covers the branch (.claude/commands/_lib/fable-review.md,
# "The validity decision"; renamed to _lib/cross-check-review.md in phase 4).
#
#   ENTRY  launch (the launch entry) or checkpoint (the checkpoint and the
#          pre-push entries); it chooses only whether the rule of the
#          commits after the checkpoint mark applies
#   HEAD   the branch's head, 40 hexadecimal characters
#   PLAN   the task's docs/tasks/<task-dir>/03_implementation_plan.md, which
#          need not exist
#
# Output, on standard output, the same at either ENTRY:
#   VALID            the last cross-check record is valid and no merge commit
#                    after it waits for the merge check
#   MISSING <START>  it is not, or there is none; the cross-check partial
#                    review is to see <START>..HEAD (<START> is 40 hexadecimal
#                    characters)
#   MERGE <n>        the record is valid but for <n> confirmable merge
#                    commits no merge check record names; one line of the
#                    merge commit's 40 hexadecimal characters follows for
#                    each, in the order of git rev-list --reverse
#                    --topo-order
#
# Contract:
#   - Model pattern validation: in cross-check (and alias fable) mode only,
#     read origin/main:.claude/commands/_context.md and validate its entire
#     "### Harness model mapping" table fail-closed before judging records.
#     Collect all non-"-" Verification patterns for Role == cross-check (for
#     cross-check run records) and Role == high (for merge check run records).
#     Match model IDs without regex or glob evaluation ("." matches only a
#     literal "."; a trailing-* pattern matches its literal prefix plus a
#     1..48-char suffix in [a-z0-9.-]). coverage and unreviewed modes do not
#     read _context.md.
#   - The widened START check: a range line's START passes it when START is
#     the merge-base of origin/main and HEAD, a commit of origin/main..HEAD,
#     or a commit origin/main reaches such that origin/main reaches no
#     commit of END ^START. Every other check of a seen-range line above
#     stays. A line read before a merge of origin/main thus keeps counting,
#     while a range that holds a commit of origin/main is still ignored.
#   - A cross-check record is a whole line, from the first column, of the
#     message of a commit in origin/main..HEAD, matching the new
#     ("Cross-check review:") or legacy ("Fable review:") run or not-run form
#     exactly (accepting both "wrong-model" and legacy "not-fable", and any
#     model ID matching a Role == cross-check pattern from origin/main's
#     _context.md), whose START and END pass the checks of a seen-range line
#     above under the widened START check; a record that fails them is
#     ignored. The last record is the last such line of the last commit
#     holding one, in the order of git rev-list --reverse --topo-order.
#   - A confirmable merge commit has exactly two parents, the first the
#     merge-base or a commit of origin/main..HEAD, the second a commit
#     origin/main reaches. Any other commit with other than one parent is an
#     unconfirmable merge commit.
#   - A merge check record is a whole line, from the first column, of the
#     message of a commit Q in origin/main..HEAD, matching the run (any model
#     ID matching a Role == high pattern from origin/main's _context.md) or
#     the not-run form (accepting both "wrong-model" and legacy "not-opus")
#     exactly. It names the merge commit M of its 40 hexadecimal characters
#     only when M is a commit of origin/main..HEAD, a confirmable merge
#     commit, and a proper ancestor of Q; a line naming anything else (no
#     object, an object that is not a commit, another commit) names nothing
#     and is not a failure. Every such line of the branch is read, not only
#     the last.
#   - The last record, with END and the commit H holding it, is judged on
#     every commit of HEAD ^END that origin/main does not reach. A commit
#     with one parent is covered when it is any of:
#       1. without content: it changes nothing, or only README.en.md (a
#          regular file before and after), or only PLAN by marking its
#          checkboxes as a plan checkbox commit does above, or those two
#          together;
#       2. a fix of the same run: an ancestor of H (H included) inside the
#          range of a seen-range line or of a not-run line ("Local CLI
#          review: not run <START12>..<END12> (...)") of H's message, the
#          line passing the checks of a seen-range line above, under the
#          widened START check, with H as the commit holding it;
#       3. at ENTRY checkpoint only: a commit after the last commit K of
#          origin/main..HEAD whose message holds the line "Cross-check
#          review: checkpoint" or legacy "Fable review: checkpoint", unless
#          a commit of HEAD ^K ^origin/main holds a line starting
#          "Implementation model: " (any such line counts, so reading more
#          lines only turns the rule off);
#       4. a fix of a merge check: for a commit Q whose merge check record
#          names M, a descendant of M (M excluded) and an ancestor of Q (Q
#          included) inside the range of a seen-range line of Q's message
#          that passes the checks under the widened START check; a not-run
#          line's range does not count.
#     A confirmable merge commit is confirmed when a merge check record
#     names it, and otherwise waits for the merge check. Every other commit,
#     a commit with one parent no rule covers or an unconfirmable merge
#     commit, invalidates the record.
#   - The result: MISSING <START> when a commit invalidates the record; else
#     MERGE <n> when a confirmable merge commit waits for the merge check;
#     else VALID.
#   - START of MISSING is the merge-base of origin/main and HEAD when there
#     is no record, or when HEAD ^END ^origin/main holds a commit with other
#     than one parent (a confirmed one included). Otherwise it is the first
#     parent of the first commit, in git rev-list --reverse --topo-order,
#     that invalidates the record; when that parent is neither the
#     merge-base nor a commit of origin/main..HEAD, START is the merge-base.
#   - Inputs, environment, and plumbing are as for coverage.
#   - Failure: any failed git step, an argument of another form, an
#     unreadable refs/remotes/origin/main, or a missing or malformed
#     origin/main:.claude/commands/_context.md prints "MISSING" alone, a
#     diagnostic on standard error, and exits 1. A caller reads a status
#     other than 0, or a first line of another form, as MISSING from the
#     merge-base.
#
# Where the rules are defined. The command documents define the record
# lines and what is done with the result, and those definitions win over
# this file: the seen-range and not-run lines' forms in
# .claude/commands/_lib/local-cli-review.md, step 7, the Fable records',
# the checkpoint mark's, and the merge check records' forms and the
# validity rules in .claude/commands/_lib/fable-review.md ("Record lines"
# and "The validity decision"), the form of the "PR created" line in
# .claude/commands/runplan.md, step 5a (AFTER_CREATE), and the rules that
# turn the coverage result into a decision in .claude/commands/runpr.md,
# step 3. When they disagree, the script is fixed. The coverage judging
# rules themselves (which commits are judged, which count as seen, and the
# checks on a seen-range line's START and END) are defined by this header,
# on the basis of the design of task 0098
# (docs/tasks/0098_fable_partial_review/02_architecture.md, 3.6.1), and the
# two differences of unreviewed on the basis of the design of task 0114
# (docs/tasks/0114_fable_record_after_merge/02_architecture.md, 3.2); no
# command document restates them. A change to them changes this header
# and the tests in cmd/yt2column/review_records_test.go together.
#
# Japanese literals: the heading and checkbox texts below must equal the
# text of the Japanese plans this script reads (CLAUDE.md, "Go Idioms").

# Run again in a clean environment, so nothing the caller's environment
# carries (exported functions, SHELLOPTS, BASHOPTS, LC_ALL, GIT_ variables)
# reaches the shell or git: only PATH, HOME, and TMPDIR are kept. The marker
# is this process's ID, which exec keeps, so a caller cannot skip the run by
# presetting it; [[ is a keyword no function can replace.
if [[ ${REVIEW_RECORDS_CLEAN_ENV-} != "$$" ]]; then
	exec env -i REVIEW_RECORDS_CLEAN_ENV=$$ PATH="${PATH:-}" HOME="${HOME:-}" TMPDIR="${TMPDIR:-}" \
		"${BASH:-bash}" --noprofile --norc "$0" "$@" </dev/null
fi
unset REVIEW_RECORDS_CLEAN_ENV

set -u -o pipefail
export LC_ALL=C

for name in ${!GIT_@}; do
	unset "$name"
done
unset -f git awk printf
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_ATTR_NOSYSTEM=1
export GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0 GIT_NO_REPLACE_OBJECTS=1 GIT_GRAFT_FILE=/dev/null

GIT_OVERRIDES=(
	-c core.attributesFile=/dev/null
	-c core.quotePath=true
	-c diff.relative=false
	-c diff.noprefix=false
	-c color.ui=false
	-c log.showSignature=false
	-c advice.graftFileDeprecated=false
	-c i18n.logOutputEncoding=UTF-8
)

g() {
	command git "${GIT_OVERRIDES[@]}" "$@"
}

MODE=${1-}

fail() {
	printf 'review-records: %s\n' "$1" >&2
	case $MODE in
	coverage | unreviewed) printf 'PARTIAL\n' ;;
	cross-check | fable) printf 'MISSING\n' ;;
	esac
	exit 1
}

HEX40='^[0-9a-f]{40}$'
SEEN_RE='^Local CLI reviewed: ([0-9a-f]{12})\.\.([0-9a-f]{12}) \((coderabbit|codex)\)$'
PLAN_RE='^docs/tasks/[^/]+/03_implementation_plan\.md$'
NOT_RUN_RE='^Local CLI review: not run ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(.*\)$'
CROSS_CHECK_RUN_RE='^(Cross-check|Fable) review: run ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(model ([^;]+); fixed [0-9]+, rejected [0-9]+, deferred [0-9]+, unclear [0-9]+, invalid [0-9]+; [0-9]+s, tokens ([0-9]+|unknown)\)$'
CROSS_CHECK_NOT_RUN_RE='^(Cross-check|Fable) review: not run ([0-9a-f]{12})\.\.([0-9a-f]{12}) \((no-target|unavailable|timeout|wrong-model|not-fable|unreadable|agent-cannot-call)(; [0-9]+s, tokens ([0-9]+|unknown))?\)$'
CHECKPOINT_LINE='Cross-check review: checkpoint'
LEGACY_CHECKPOINT_LINE='Fable review: checkpoint'
MERGE_RUN_RE='^Merge check: run ([0-9a-f]{40}) \(model ([^;]+); fixed [0-9]+, rejected [0-9]+, deferred [0-9]+, unclear [0-9]+, invalid [0-9]+; [0-9]+s, tokens ([0-9]+|unknown)\)$'
MERGE_NOT_RUN_RE='^Merge check: not run ([0-9a-f]{40}) \((unavailable|timeout|wrong-model|not-opus|unreadable|agent-cannot-call)(; [0-9]+s, tokens ([0-9]+|unknown))?\)$'
SUFFIX_RE='^[a-z0-9.-]+$'
MODIFIED_RE='^:100644 100644 [0-9a-f]{40} [0-9a-f]{40} M$'
TAB=$(printf '\t')
IMPLEMENTATION_PREFIX='Implementation model: '

# parse_harness_mapping validates the entire "### Harness model mapping" table
# of _context.md fail-closed and emits CC<TAB><pat> for Role == cross-check
# (excluding "-") and HI<TAB><pat> for Role == high (excluding "-"), or
# ERR<TAB><msg> on any violation.
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
			if (role == "cross-check" && pat != "-") {
				cc_pats[++cc_count] = pat
			}
			if (role == "high" && pat != "-") {
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
			else if (cc_count == 0) fail("no cross-check verification pattern in Harness model mapping")
			else if (high_count == 0) fail("no high-role verification pattern in Harness model mapping")
			if (failed) exit 0
			for (i = 1; i <= cc_count; i++) {
				printf "CC\t%s\n", cc_pats[i]
			}
			for (i = 1; i <= high_count; i++) {
				printf "HI\t%s\n", high_pats[i]
			}
		}
	'
}

# load_harness_patterns reads origin/main:.claude/commands/_context.md (must
# be a regular file), validates the entire Harness model mapping table
# fail-closed, and populates CROSS_CHECK_PATS and HIGH_PATS.
load_harness_patterns() {
	local entry mode type context_doc mapping_out mline
	local err_pfx="ERR${TAB}" cc_pfx="CC${TAB}" hi_pfx="HI${TAB}"
	entry=$(g ls-tree "$ORIGIN_MAIN" -- .claude/commands/_context.md) ||
		fail "cannot inspect origin/main tree for _context.md"
	[ -n "$entry" ] || fail "cannot read origin/main:.claude/commands/_context.md"
	case $entry in
	*$'\n'*) fail "origin/main:.claude/commands/_context.md matched multiple tree entries" ;;
	esac
	IFS=' ' read -r mode type _ < <(printf '%s\n' "$entry")
	case "$mode $type" in
	"100644 blob" | "100755 blob") ;;
	*) fail "origin/main:.claude/commands/_context.md is not a regular file (mode $mode)" ;;
	esac
	context_doc=$(g cat-file blob "$ORIGIN_MAIN:.claude/commands/_context.md" 2>/dev/null) ||
		fail "cannot read origin/main:.claude/commands/_context.md"
	mapping_out=$(printf '%s\n' "$context_doc" | parse_harness_mapping) ||
		fail "failed to parse Harness model mapping"
	CROSS_CHECK_PATS=""
	HIGH_PATS=""
	while IFS= read -r mline; do
		case $mline in
		"$err_pfx"*)
			fail "${mline#"$err_pfx"}"
			;;
		"$cc_pfx"*)
			CROSS_CHECK_PATS=$CROSS_CHECK_PATS$'\n'${mline#"$cc_pfx"}
			;;
		"$hi_pfx"*)
			HIGH_PATS=$HIGH_PATS$'\n'${mline#"$hi_pfx"}
			;;
		"") ;;
		*)
			fail "unexpected parser output for Harness model mapping"
			;;
		esac
	done < <(printf '%s\n' "$mapping_out")
	[ -n "$CROSS_CHECK_PATS" ] || fail "no cross-check verification pattern in Harness model mapping"
	[ -n "$HIGH_PATS" ] || fail "no high-role verification pattern in Harness model mapping"
}

# matches_pattern checks id against a validated literal or trailing-* pattern
# without regex or glob evaluation: "." matches only a literal ".", and a
# trailing-* suffix must match ^[a-z0-9.-]{1,48}$.
matches_pattern() {
	local id=$1 pat=$2 pfx suffix
	case $pat in
	*'*')
		pfx=${pat%\*}
		[ "${#id}" -gt "${#pfx}" ] || return 1
		[ "${id:0:${#pfx}}" = "$pfx" ] || return 1
		suffix=${id:${#pfx}}
		[ "${#suffix}" -ge 1 ] && [ "${#suffix}" -le 48 ] || return 1
		[[ $suffix =~ $SUFFIX_RE ]] || return 1
		return 0
		;;
	*)
		[ "$id" = "$pat" ]
		;;
	esac
}

# matches_any_pattern reports whether $1 matches at least one newline-separated
# pattern in $2.
matches_any_pattern() {
	local id=$1 list=$2 pat
	while IFS= read -r pat; do
		[ -n "$pat" ] || continue
		if matches_pattern "$id" "$pat"; then
			return 0
		fi
	done < <(printf '%s\n' "$list")
	return 1
}

# in_list reports whether $1 is one of the newline-separated IDs in $2.
in_list() {
	case $'\n'"$2"$'\n' in
	*$'\n'"$1"$'\n'*) return 0 ;;
	esac
	return 1
}

# message prints a commit's message; the commit object is read directly, so
# no log configuration changes it.
message() {
	g cat-file commit "$1" | awk 'body { print; next } /^$/ { body = 1 }'
}

parent_count() {
	g cat-file commit "$1" | awk '/^$/ { exit } /^parent / { n++ } END { print n + 0 }'
}

# resolve12 prints the one commit whose ID starts with $1, which must name no
# ref; it fails otherwise. The prefix is never resolved as a name, so a ref
# whose name looks like an ID cannot stand in for the commit.
resolve12() {
	local prefix=$1 objects object found=""
	if in_list "refs/heads/$prefix" "$REFS" || in_list "refs/tags/$prefix" "$REFS" ||
		in_list "refs/remotes/$prefix" "$REFS" || in_list "refs/$prefix" "$REFS"; then
		return 1
	fi
	objects=$(g rev-parse --disambiguate="$prefix") || return 1
	for object in $objects; do
		[ "$(g cat-file -t "$object" 2>/dev/null)" = commit ] || continue
		[ -z "$found" ] || return 1
		found=$object
	done
	[ -n "$found" ] || return 1
	printf '%s\n' "$found"
}

# plan_lines prints each line of the blob $1 of a plan as
# "<kind>\t<end>\t<line>", where <kind> is M for a line of a PR marker's
# section, S for a line of the checklist chapter (## 6) of a plan without PR
# markers, and O for any other line, a line inside a fenced code block and
# every heading and fence line included; <end> is 1 when the line ended in
# CR (a CRLF line) and 0 otherwise; and <line> is the line without the CR. A section runs from its heading to the next heading of any level
# (.claude/commands/_context.md, "PR marker conventions", part 2). An
# unclosed fence fails.
plan_lines() {
	g cat-file blob "$1" | awk '
		{
			end[NR] = sub(/\r$/, "") ? 1 : 0
			line[NR] = $0
			kind[NR] = "O"
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
			if (fence) next
			if ($0 ~ /^#+[ \t]/) {
				sec = ""
				if ($0 ~ /^### PR-[0-9]+ 作成ポイント(:|[ \t]*$)/) { sec = "M"; markers++ }
				else if ($0 ~ /^## 6([.]|[ \t])/) sec = "S"
				next
			}
			if (sec != "") kind[NR] = sec
		}
		END {
			if (fence) exit 1
			for (i = 1; i <= NR; i++) {
				k = kind[i]
				if (k == "S" && markers) k = "O"
				printf "%s\t%s\t%s\n", k, end[i], line[i]
			}
		}
	'
}

# checkbox_only reports whether commit $1, with parent $2, changes PLAN only
# by marking its PR progress checkboxes:
#   - in place, in a PR marker's section or the single-PR record, with the
#     same list prefix before and after:
#       the "green gate confirmed" box from "[ ]" to "[x]", the rest of the
#       line unchanged (runplan.md step 5a);
#       the "PR created" box from "[ ]" to "[x]" with ": <PR URL>" appended
#       (runplan.md step 5a);
#       the "PR merged" box from "[ ]" to "[x]" (runpr.md);
#       the "switched to the next branch" box from "[ ]" to "[x]" or "[-]",
#       the rest of the line unchanged (runpr.md);
#   - or by adding, in the checklist chapter of a plan without PR markers,
#     a checked "PR created" line with its URL followed by an unchecked "PR
#     merged" line (runplan.md step 8).
# Every line kept, the marked ones included, keeps its line end, so a commit
# that also rewrites line ends is not one.
# Any other change, to PLAN or to another path, is not.
checkbox_only() {
	local commit=$1 parent=$2 raw
	[ -n "$PLAN" ] || return 1
	raw=$(g diff-tree -r -z --raw --no-abbrev --no-renames --ignore-submodules=none --no-commit-id "$parent" "$commit" | tr '\0' '\n') || return 1
	[ "$raw" = ":100644 100644 $(g rev-parse --verify --quiet "$parent:$PLAN") $(g rev-parse --verify --quiet "$commit:$PLAN") M
$PLAN" ] || return 1
	plan_marked "$commit" "$parent"
}

# plan_marked reports whether PLAN, a regular file in commit $1 and in its
# parent $2, changes only by the markings checkbox_only describes.
plan_marked() {
	local commit=$1 parent=$2 old new
	old=$(plan_lines "$parent:$PLAN") || return 1
	new=$(plan_lines "$commit:$PLAN") || return 1
	{
		printf '%s\n' "$old" | awk '{ print "A\t" $0 }'
		printf '%s\n' "$new" | awk '{ print "B\t" $0 }'
	} | awk -F '\t' '
		{
			k = $2
			s = $0
			sub(/^[AB]\t[MSO]\t[01]\t/, "", s)
			if ($1 == "A") { na++; ak[na] = k; ae[na] = $3; al[na] = s } else { nb++; bk[nb] = k; be[nb] = $3; bl[nb] = s }
		}
		# flip reports whether old line a becomes new line b by a marking.
		function flip(a, b,    p, ta, tb, url) {
			if (!match(a, /^-[ ]+\[ \] /)) return 0
			p = substr(a, 1, RLENGTH - 4)
			if (substr(b, 1, length(p)) != p) return 0
			ta = substr(a, length(p) + 1)
			tb = substr(b, length(p) + 1)
			if (substr(ta, 1, length("[ ] グリーンゲート")) == "[ ] グリーンゲート") {
				return tb == "[x]" substr(ta, 4)
			}
			if (ta == "[ ] PR を作成した") {
				if (substr(tb, 1, length("[x] PR を作成した: ")) != "[x] PR を作成した: ") return 0
				url = substr(tb, length("[x] PR を作成した: ") + 1)
				return url ~ /^https:\/\/github[.]com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+\/pull\/[0-9]+$/
			}
			if (ta == "[ ] PR がマージされた") return tb == "[x] PR がマージされた"
			if (substr(ta, 1, length("[ ] 次のブランチへ切り替えた")) == "[ ] 次のブランチへ切り替えた") {
				return tb == "[x]" substr(ta, 4) || tb == "[-]" substr(ta, 4)
			}
			return 0
		}
		function created(s) {
			if (substr(s, 1, length("- [x] PR を作成した: ")) != "- [x] PR を作成した: ") return 0
			return substr(s, length("- [x] PR を作成した: ") + 1) ~ /^https:\/\/github[.]com\/[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+\/pull\/[0-9]+$/
		}
		END {
			if (nb == na) {
				changed = 0
				for (i = 1; i <= na; i++) {
					if (ak[i] != bk[i] || ae[i] != be[i]) exit 1
					if (al[i] == bl[i]) continue
					if (ak[i] == "O" || !flip(al[i], bl[i])) exit 1
					changed++
				}
				exit changed ? 0 : 1
			}
			if (nb != na + 2) exit 1
			k = 1
			while (k <= na && ak[k] == bk[k] && ae[k] == be[k] && al[k] == bl[k]) k++
			if (bk[k] != "S" || bk[k + 1] != "S") exit 1
			if (!created(bl[k]) || bl[k + 1] != "- [ ] PR がマージされた") exit 1
			for (i = k; i <= na; i++) if (ak[i] != bk[i + 2] || ae[i] != be[i + 2] || al[i] != bl[i + 2]) exit 1
			exit 0
		}
	'
}

# contentless reports whether commit $1, with parent $2, has no content for
# the Fable record: it changes nothing, or only README.en.md, or only PLAN by
# the markings checkbox_only describes, or those two, each a regular file
# modified in place. The raw diff is read without -z, so git quotes a path
# holding a tab or a newline and such a path matches neither name.
contentless() {
	local commit=$1 parent=$2 raw line meta path plan_changed=0
	raw=$(g diff-tree -r --raw --no-abbrev --no-renames --ignore-submodules=none --no-commit-id "$parent" "$commit") || return 1
	while IFS= read -r line; do
		[ -n "$line" ] || continue
		meta=${line%%"$TAB"*}
		path=${line#*"$TAB"}
		[[ $meta =~ $MODIFIED_RE ]] || return 1
		if [ "$path" = README.en.md ]; then
			continue
		fi
		if [ -n "$PLAN" ] && [ "$path" = "$PLAN" ]; then
			plan_changed=1
			continue
		fi
		return 1
	done <<EOF_RAW
$raw
EOF_RAW
	[ "$plan_changed" = 0 ] || plan_marked "$commit" "$parent"
}

# empty_commit reports whether commit $1 has no difference from its parent $2.
empty_commit() {
	local raw
	raw=$(g diff-tree -r -z --raw --no-renames --ignore-submodules=none --no-commit-id "$2" "$1" | tr '\0' '\n') || return 1
	[ -z "$raw" ]
}

# load_branch enters the top of the work tree and sets, for the head $1,
# ORIGIN_MAIN, REFS, MERGE_BASE, and BRANCH (the commits of
# origin/main..head).
load_branch() {
	local head=$1 top
	top=$(g rev-parse --show-toplevel) || fail "not inside a git work tree"
	cd "$top" || fail "cannot enter the top of the work tree"
	ORIGIN_MAIN=$(g rev-parse --verify --quiet 'refs/remotes/origin/main^{commit}') ||
		fail "refs/remotes/origin/main does not name a commit"
	[ "$(g cat-file -t "$head" 2>/dev/null)" = commit ] || fail "HEAD names no commit"
	REFS=$(g for-each-ref --format='%(refname)') || fail "git for-each-ref failed"
	MERGE_BASE=$(g merge-base "$ORIGIN_MAIN" "$head") || fail "origin/main and HEAD have no merge-base"
	BRANCH=$(g rev-list "$head" "^$ORIGIN_MAIN") || fail "git rev-list of the branch failed"
}

# checked_range sets RANGE_START and RANGE_END to the commits the 12-hex
# prefixes $2 and $3 of a line in the message of commit $1 name, and reports
# whether the line passes the checks of a seen-range line; with $4 "widen"
# START passes the widened START check instead (the fable mode's contract).
checked_range() {
	local holder=$1 widen=${4-} all inside
	RANGE_START=$(resolve12 "$2") || return 1
	RANGE_END=$(resolve12 "$3") || return 1
	g merge-base --is-ancestor "$RANGE_START" "$RANGE_END" || return 1
	[ "$RANGE_END" != "$holder" ] || return 1
	g merge-base --is-ancestor "$RANGE_END" "$holder" || return 1
	in_list "$RANGE_END" "$BRANCH" || return 1
	if [ "$RANGE_START" = "$MERGE_BASE" ] || in_list "$RANGE_START" "$BRANCH"; then
		return 0
	fi
	# Widened: a START behind a merge of origin/main passes only when the
	# range holds no commit origin/main reaches, so a range made after the
	# merge, which holds main's commits, is still ignored.
	[ "$widen" = widen ] || return 1
	g merge-base --is-ancestor "$RANGE_START" "$ORIGIN_MAIN" || return 1
	all=$(g rev-list --count "$RANGE_END" "^$RANGE_START") || return 1
	inside=$(g rev-list --count "$RANGE_END" "^$RANGE_START" "^$ORIGIN_MAIN") || return 1
	[ "$all" = "$inside" ]
}

# confirmable reports whether commit $1 is a confirmable merge commit:
# exactly two parents, the first the merge-base or a branch commit, the
# second a commit origin/main reaches. Only for such a merge commit can the
# merge check take the changes made by hand and the changes main brought in;
# any other is judged like a commit with content.
confirmable() {
	local first second
	[ "$(parent_count "$1")" = 2 ] || return 1
	first=$(g rev-parse --verify --quiet "$1^1") || return 1
	second=$(g rev-parse --verify --quiet "$1^2") || return 1
	if [ "$first" != "$MERGE_BASE" ] && ! in_list "$first" "$BRANCH"; then
		return 1
	fi
	g merge-base --is-ancestor "$second" "$ORIGIN_MAIN"
}

coverage() {
	[ "$#" -eq 2 ] || [ "$#" -eq 3 ] || fail "coverage expects 2 or 3 arguments, got $#"
	local last=$1 head=$2 c line holder
	PLAN=${3-}
	[[ $last == none || $last =~ $HEX40 ]] || fail "LAST_REVIEWED is neither none nor 40 hexadecimal characters"
	[[ $head =~ $HEX40 ]] || fail "HEAD is not 40 hexadecimal characters"
	if [ -n "$PLAN" ]; then
		[[ $PLAN =~ $PLAN_RE ]] || fail "PLAN is not docs/tasks/<task-dir>/03_implementation_plan.md"
	fi

	load_branch "$head"
	if [ "$last" != none ]; then
		[ "$(g cat-file -t "$last" 2>/dev/null)" = commit ] || fail "LAST_REVIEWED names no commit"
		g merge-base --is-ancestor "$last" "$head" ||
			fail "LAST_REVIEWED is not an ancestor of HEAD"
	fi

	local judged
	if [ "$last" = none ]; then
		judged=$(g rev-list --reverse "$head" "^$ORIGIN_MAIN") || fail "git rev-list failed"
	else
		judged=$(g rev-list --reverse "$head" "^$last" "^$ORIGIN_MAIN") || fail "git rev-list failed"
	fi
	judge_seen "$judged" narrow
}

# unreviewed is coverage none with the two differences the header names.
unreviewed() {
	[ "$#" -eq 1 ] || [ "$#" -eq 2 ] || fail "unreviewed expects 1 or 2 arguments, got $#"
	local head=$1 judged
	PLAN=${2-}
	[[ $head =~ $HEX40 ]] || fail "HEAD is not 40 hexadecimal characters"
	if [ -n "$PLAN" ]; then
		[[ $PLAN =~ $PLAN_RE ]] || fail "PLAN is not docs/tasks/<task-dir>/03_implementation_plan.md"
	fi
	load_branch "$head"
	judged=$(g rev-list --reverse "$head" "^$ORIGIN_MAIN") || fail "git rev-list failed"
	judge_seen "$judged" widen
}

# judge_seen prints the result of coverage for the commits $1 judged, oldest
# first. $2 chooses the two points where unreviewed differs: "narrow"
# (coverage) or "widen" (unreviewed: the widened START check, and a merge
# commit inside a seen range is seen).
judge_seen() {
	local judged=$1 how=$2 c line holder
	# seen holds every commit inside the range of a seen-range line that
	# passes the checks.
	local seen="" msg range
	for holder in $BRANCH; do
		msg=$(message "$holder") || fail "cannot read a commit message"
		while IFS= read -r line; do
			[[ $line =~ $SEEN_RE ]] || continue
			checked_range "$holder" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "$how" || continue
			range=$(g rev-list "$RANGE_END" "^$RANGE_START") || fail "git rev-list of a seen range failed"
			seen=$seen$'\n'$range
		done < <(printf '%s\n' "$msg")
	done

	local unseen="" count=0 parents
	for c in $judged; do
		parents=$(parent_count "$c") || fail "cannot read a commit"
		if [ "$parents" = 1 ]; then
			local parent
			parent=$(g rev-parse --verify --quiet "$c^1") || fail "cannot read a parent"
			empty_commit "$c" "$parent" && continue
			checkbox_only "$c" "$parent" && continue
			in_list "$c" "$seen" && continue
		elif [ "$how" = widen ]; then
			in_list "$c" "$seen" && continue
		fi
		unseen=$unseen$'\n'$c
		count=$((count + 1))
	done

	if [ "$count" -eq 0 ]; then
		printf 'ALL\n' || exit 1
		return
	fi
	local out
	out="PARTIAL $count"
	for c in $unseen; do
		line=$(g log -1 --no-show-signature --format='%h %s' --abbrev=12 "$c") || fail "git log failed"
		out=$out$'\n'$line
	done
	printf '%s\n' "$out" || exit 1
}

# has_line reports whether the message of commit $1 has a line equal to $2
# (when $3 is "exact") or starting with $2 (when $3 is "prefix").
has_line() {
	local msg line
	msg=$(message "$1") || fail "cannot read a commit message"
	while IFS= read -r line; do
		case $3 in
		exact) [ "$line" = "$2" ] && return 0 ;;
		prefix) [ "${line#"$2"}" != "$line" ] && return 0 ;;
		esac
	done < <(printf '%s\n' "$msg")
	return 1
}

cross_check() {
	[ "$#" -eq 3 ] || fail "$MODE expects 3 arguments, got $#"
	local entry=$1 head=$2 c line holder msg start12 end12 model_id
	PLAN=$3
	[ "$entry" = launch ] || [ "$entry" = checkpoint ] || fail "ENTRY is neither launch nor checkpoint"
	[[ $head =~ $HEX40 ]] || fail "HEAD is not 40 hexadecimal characters"
	[[ $PLAN =~ $PLAN_RE ]] || fail "PLAN is not docs/tasks/<task-dir>/03_implementation_plan.md"

	load_branch "$head"
	load_harness_patterns
	local ordered
	ordered=$(g rev-list --reverse --topo-order "$head" "^$ORIGIN_MAIN") || fail "git rev-list failed"

	# The last cross-check (or legacy Fable) record whose range and model pass
	# the checks.
	local rec_end="" rec_holder=""
	for holder in $ordered; do
		msg=$(message "$holder") || fail "cannot read a commit message"
		while IFS= read -r line; do
			if [[ $line =~ $CROSS_CHECK_RUN_RE ]]; then
				start12=${BASH_REMATCH[2]}
				end12=${BASH_REMATCH[3]}
				model_id=${BASH_REMATCH[4]}
				matches_any_pattern "$model_id" "$CROSS_CHECK_PATS" || continue
			elif [[ $line =~ $CROSS_CHECK_NOT_RUN_RE ]]; then
				start12=${BASH_REMATCH[2]}
				end12=${BASH_REMATCH[3]}
			else
				continue
			fi
			checked_range "$holder" "$start12" "$end12" widen || continue
			rec_end=$RANGE_END
			rec_holder=$holder
		done < <(printf '%s\n' "$msg")
	done
	if [ -z "$rec_holder" ]; then
		printf 'MISSING %s\n' "$MERGE_BASE" || exit 1
		return
	fi

	# fixes holds every commit inside a seen-range or not-run line of the
	# record's own commit.
	local fixes="" range
	msg=$(message "$rec_holder") || fail "cannot read a commit message"
	while IFS= read -r line; do
		[[ $line =~ $SEEN_RE ]] || [[ $line =~ $NOT_RUN_RE ]] || continue
		checked_range "$rec_holder" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" widen || continue
		range=$(g rev-list "$RANGE_END" "^$RANGE_START") || fail "git rev-list of a range failed"
		fixes=$fixes$'\n'$range
	done < <(printf '%s\n' "$msg")

	# after_mark holds the commits after the last checkpoint mark, at the
	# checkpoint entry, unless one of them holds an implementation record.
	local after_mark="" mark=""
	if [ "$entry" = checkpoint ]; then
		for holder in $ordered; do
			if has_line "$holder" "$CHECKPOINT_LINE" exact || has_line "$holder" "$LEGACY_CHECKPOINT_LINE" exact; then
				mark=$holder
			fi
		done
		if [ -n "$mark" ]; then
			after_mark=$(g rev-list "$head" "^$mark" "^$ORIGIN_MAIN") || fail "git rev-list after the mark failed"
			for c in $after_mark; do
				if has_line "$c" "$IMPLEMENTATION_PREFIX" prefix; then
					after_mark=""
					break
				fi
			done
		fi
	fi

	# confirmed holds every merge commit a merge check record names, and
	# merge_fixes every commit rule 4 covers.
	local confirmed="" merge_fixes="" named m
	for holder in $ordered; do
		msg=$(message "$holder") || fail "cannot read a commit message"
		named=""
		while IFS= read -r line; do
			if [[ $line =~ $MERGE_RUN_RE ]]; then
				m=${BASH_REMATCH[1]}
				model_id=${BASH_REMATCH[2]}
				matches_any_pattern "$model_id" "$HIGH_PATS" || continue
			elif [[ $line =~ $MERGE_NOT_RUN_RE ]]; then
				m=${BASH_REMATCH[1]}
			else
				continue
			fi
			# Only a branch commit can be named. BRANCH lists commits only, so
			# a name of no object, or of another object, names nothing; testing
			# it before any lookup keeps a name the repository lacks from making
			# git fetch it (a partial clone's lazy fetch).
			in_list "$m" "$BRANCH" || continue
			[ "$m" != "$holder" ] || continue
			g merge-base --is-ancestor "$m" "$holder" || continue
			confirmable "$m" || continue
			confirmed=$confirmed$'\n'$m
			named=$named$'\n'$m
		done < <(printf '%s\n' "$msg")
		[ -n "$named" ] || continue
		while IFS= read -r line; do
			[[ $line =~ $SEEN_RE ]] || continue
			checked_range "$holder" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" widen || continue
			range=$(g rev-list "$RANGE_END" "^$RANGE_START") || fail "git rev-list of a range failed"
			# M itself is inside the range of its descendants' test, but it
			# has two parents, which no rule covers.
			for c in $range; do
				for m in $named; do
					g merge-base --is-ancestor "$m" "$c" || continue
					g merge-base --is-ancestor "$c" "$holder" || continue
					merge_fixes=$merge_fixes$'\n'$c
					break
				done
			done
		done < <(printf '%s\n' "$msg")
	done

	local judged parents parent start invalid="" merged=0 unconfirmed="" count=0
	judged=$(g rev-list --reverse --topo-order "$head" "^$rec_end" "^$ORIGIN_MAIN") || fail "git rev-list failed"
	for c in $judged; do
		parents=$(parent_count "$c") || fail "cannot read a commit"
		if [ "$parents" = 1 ]; then
			parent=$(g rev-parse --verify --quiet "$c^1") || fail "cannot read a parent"
			contentless "$c" "$parent" && continue
			in_list "$c" "$fixes" && continue
			in_list "$c" "$after_mark" && continue
			in_list "$c" "$merge_fixes" && continue
		else
			merged=1
			if confirmable "$c"; then
				in_list "$c" "$confirmed" && continue
				unconfirmed=$unconfirmed$'\n'$c
				count=$((count + 1))
				continue
			fi
		fi
		[ -n "$invalid" ] || invalid=$c
	done

	if [ -n "$invalid" ]; then
		# After a merge, a range from the record would hold the changes main
		# brought in, so the cross-check partial review sees the whole PR.
		if [ "$merged" = 1 ]; then
			start=$MERGE_BASE
		else
			start=$(g rev-parse --verify --quiet "$invalid^1") || start=$MERGE_BASE
			# Defensive only: with no merge commit after END, the commits after
			# it form one line of single parents down to END, so this parent
			# is END or a commit after it, both branch commits.
			if [ "$start" != "$MERGE_BASE" ] && ! in_list "$start" "$BRANCH"; then
				start=$MERGE_BASE
			fi
		fi
		printf 'MISSING %s\n' "$start" || exit 1
		return
	fi
	if [ "$count" -gt 0 ]; then
		local out="MERGE $count"
		for c in $unconfirmed; do
			out=$out$'\n'$c
		done
		printf '%s\n' "$out" || exit 1
		return
	fi
	printf 'VALID\n' || exit 1
}

case $MODE in
coverage)
	shift
	coverage "$@"
	;;
unreviewed)
	shift
	unreviewed "$@"
	;;
cross-check | fable)
	shift
	cross_check "$@"
	;;
*)
	fail "unknown mode: ${MODE:-(none)}"
	;;
esac
