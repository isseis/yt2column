#!/usr/bin/env bash
# review-records.sh reads the review record lines of a branch's commit
# messages and decides, by fixed rules, what a review gate needs to know.
#
# Usage: bash review-records.sh coverage <LAST_REVIEWED> <HEAD> [<PLAN>]
#        bash review-records.sh fable <ENTRY> <HEAD> <PLAN>
#
# The first argument chooses the mode; any other mode is a failure.
#
# Mode coverage: whether the local CLI review saw every commit after Codex's
# last PR review (.claude/commands/runpr.md, step 3, and
# .claude/commands/_lib/local-cli-review.md, "Reading the record lines").
#
#   LAST_REVIEWED  the commit of Codex's last PR review, 40 hexadecimal
#                  characters, or "none" when there is none
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
#         "PR created", "PR merged", and "switched to the next branch"
#         lines) of a PR marker's section, or of the single-PR record of a
#         plan without markers, the way /runplan and /runpr mark them (see
#         checkbox_only below), with the line end (LF or CRLF) of every kept
#         line unchanged; never with PLAN absent;
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
#     neither .gitmodules nor the configuration hides one), and replace refs
#     are not followed (GIT_NO_REPLACE_OBJECTS).
#   - Failure: any failed git step, an argument of another form, or an
#     unreadable refs/remotes/origin/main prints "PARTIAL" alone (no
#     count), a diagnostic on standard error, and exits 1. A caller reads a
#     status other than 0, or a first line of another form, as PARTIAL.
#   - It uses no network, writes no file, and uses no bash feature newer
#     than 3.2.
#
# Mode fable: whether the branch's last Fable record still covers the branch
# (.claude/commands/_lib/fable-review.md, "The validity decision").
#
#   ENTRY  launch (the launch entry) or checkpoint (the checkpoint entry)
#   HEAD   the branch's head, 40 hexadecimal characters
#   PLAN   the task's docs/tasks/<task-dir>/03_implementation_plan.md, which
#          need not exist
#
# Output, on standard output:
#   VALID            the last Fable record is valid
#   MISSING <START>  it is not, or there is none; Fable is to see
#                    <START>..HEAD (<START> is 40 hexadecimal characters)
#
# Contract:
#   - A Fable record is a whole line, from the first column, of the message
#     of a commit in origin/main..HEAD, matching the run or the not-run form
#     exactly, whose START and END pass the checks of a seen-range line
#     above; a record that fails them is ignored. The last record is the
#     last such line of the last commit holding one, in the order of
#     git rev-list --reverse --topo-order.
#   - The last record, with END and the commit H holding it, is valid when
#     every commit of HEAD ^END that origin/main does not reach is any of:
#       * without content: one parent, and it changes nothing, or only
#         README.en.md (a regular file before and after), or only PLAN by
#         marking its checkboxes as a plan checkbox commit does above, or
#         those two together;
#       * a fix of the same run: an ancestor of H (H included) inside the
#         range of a seen-range line or of a not-run line ("Local CLI
#         review: not run <START12>..<END12> (...)") of H's message, the
#         line passing the checks of a seen-range line above with H as the
#         commit holding it;
#       * at ENTRY checkpoint only: a commit after the last commit K of
#         origin/main..HEAD whose message holds the line "Fable review:
#         checkpoint", unless a commit of HEAD ^K ^origin/main holds a line
#         starting "Implementation model: " (any such line counts, so
#         reading more lines only turns the rule off).
#     A merge commit is never any of them.
#   - START of MISSING is the merge-base of origin/main and HEAD when there
#     is no record. Otherwise it is the first parent of the first commit,
#     in git rev-list --reverse --topo-order, that none of the rules
#     covers; when that parent is neither the merge-base nor a commit of
#     origin/main..HEAD, START is the merge-base.
#   - Inputs, environment, and plumbing are as for coverage.
#   - Failure: any failed git step, an argument of another form, or an
#     unreadable refs/remotes/origin/main prints "MISSING" alone, a
#     diagnostic on standard error, and exits 1. A caller reads a status
#     other than 0, or a first line of another form, as MISSING from the
#     merge-base.
#
# Where the rules are defined. The command documents define the record
# lines and what is done with the result, and those definitions win over
# this file: the seen-range and not-run lines' forms in
# .claude/commands/_lib/local-cli-review.md, step 7, the Fable records' and
# the checkpoint mark's forms and the validity rules in
# .claude/commands/_lib/fable-review.md ("Record lines" and "The validity
# decision"), the form of the "PR created" line in
# .claude/commands/runplan.md, step 5a (AFTER_CREATE), and the rules that
# turn the coverage result into a decision in .claude/commands/runpr.md,
# step 3. When they disagree, the script is fixed. The coverage judging
# rules themselves (which commits are judged, which count as seen, and the
# checks on a seen-range line's START and END) are defined by this header,
# on the basis of the design of task 0098
# (docs/tasks/0098_fable_partial_review/02_architecture.md, 3.6.1); no
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
export GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0 GIT_NO_REPLACE_OBJECTS=1

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

MODE=${1-}

fail() {
	printf 'review-records: %s\n' "$1" >&2
	case $MODE in
	coverage) printf 'PARTIAL\n' ;;
	fable) printf 'MISSING\n' ;;
	esac
	exit 1
}

HEX40='^[0-9a-f]{40}$'
SEEN_RE='^Local CLI reviewed: ([0-9a-f]{12})\.\.([0-9a-f]{12}) \((coderabbit|codex)\)$'
PLAN_RE='^docs/tasks/[^/]+/03_implementation_plan\.md$'
NOT_RUN_RE='^Local CLI review: not run ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(.*\)$'
FABLE_RUN_RE='^Fable review: run ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(model claude-fable-[a-z0-9.-]{1,48}; fixed [0-9]+, rejected [0-9]+, deferred [0-9]+, unclear [0-9]+, invalid [0-9]+; [0-9]+s, tokens ([0-9]+|unknown)\)$'
FABLE_NOT_RUN_RE='^Fable review: not run ([0-9a-f]{12})\.\.([0-9a-f]{12}) \((no-target|unavailable|timeout|not-fable|unreadable|agent-cannot-call)(; [0-9]+s, tokens ([0-9]+|unknown))?\)$'
CHECKPOINT_LINE='Fable review: checkpoint'
MODIFIED_RE='^:100644 100644 [0-9a-f]{40} [0-9a-f]{40} M$'
TAB=$(printf '\t')
IMPLEMENTATION_PREFIX='Implementation model: '

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
# whether the line passes the checks of a seen-range line.
checked_range() {
	local holder=$1
	RANGE_START=$(resolve12 "$2") || return 1
	RANGE_END=$(resolve12 "$3") || return 1
	g merge-base --is-ancestor "$RANGE_START" "$RANGE_END" || return 1
	[ "$RANGE_END" != "$holder" ] || return 1
	g merge-base --is-ancestor "$RANGE_END" "$holder" || return 1
	[ "$RANGE_START" = "$MERGE_BASE" ] || in_list "$RANGE_START" "$BRANCH" || return 1
	in_list "$RANGE_END" "$BRANCH"
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

	# seen holds every commit inside the range of a seen-range line that
	# passes the checks.
	local seen="" msg range
	for holder in $BRANCH; do
		msg=$(message "$holder") || fail "cannot read a commit message"
		while IFS= read -r line; do
			[[ $line =~ $SEEN_RE ]] || continue
			checked_range "$holder" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" || continue
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

fable() {
	[ "$#" -eq 3 ] || fail "fable expects 3 arguments, got $#"
	local entry=$1 head=$2 c line holder msg
	PLAN=$3
	[ "$entry" = launch ] || [ "$entry" = checkpoint ] || fail "ENTRY is neither launch nor checkpoint"
	[[ $head =~ $HEX40 ]] || fail "HEAD is not 40 hexadecimal characters"
	[[ $PLAN =~ $PLAN_RE ]] || fail "PLAN is not docs/tasks/<task-dir>/03_implementation_plan.md"

	load_branch "$head"
	local ordered
	ordered=$(g rev-list --reverse --topo-order "$head" "^$ORIGIN_MAIN") || fail "git rev-list failed"

	# The last Fable record whose range passes the checks.
	local rec_end="" rec_holder=""
	for holder in $ordered; do
		msg=$(message "$holder") || fail "cannot read a commit message"
		while IFS= read -r line; do
			[[ $line =~ $FABLE_RUN_RE ]] || [[ $line =~ $FABLE_NOT_RUN_RE ]] || continue
			checked_range "$holder" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" || continue
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
		checked_range "$rec_holder" "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" || continue
		range=$(g rev-list "$RANGE_END" "^$RANGE_START") || fail "git rev-list of a range failed"
		fixes=$fixes$'\n'$range
	done < <(printf '%s\n' "$msg")

	# after_mark holds the commits after the last checkpoint mark, at the
	# checkpoint entry, unless one of them holds an implementation record.
	local after_mark="" mark=""
	if [ "$entry" = checkpoint ]; then
		for holder in $ordered; do
			if has_line "$holder" "$CHECKPOINT_LINE" exact; then
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

	local judged parents parent start
	judged=$(g rev-list --reverse --topo-order "$head" "^$rec_end" "^$ORIGIN_MAIN") || fail "git rev-list failed"
	for c in $judged; do
		parents=$(parent_count "$c") || fail "cannot read a commit"
		if [ "$parents" = 1 ]; then
			parent=$(g rev-parse --verify --quiet "$c^1") || fail "cannot read a parent"
			contentless "$c" "$parent" && continue
			in_list "$c" "$fixes" && continue
			in_list "$c" "$after_mark" && continue
		fi
		start=$(g rev-parse --verify --quiet "$c^1") || start=$MERGE_BASE
		if [ "$start" != "$MERGE_BASE" ] && ! in_list "$start" "$BRANCH"; then
			start=$MERGE_BASE
		fi
		printf 'MISSING %s\n' "$start" || exit 1
		return
	done
	printf 'VALID\n' || exit 1
}

case $MODE in
coverage)
	shift
	coverage "$@"
	;;
fable)
	shift
	fable "$@"
	;;
*)
	fail "unknown mode: ${MODE:-(none)}"
	;;
esac
