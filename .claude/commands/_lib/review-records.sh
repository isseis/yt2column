#!/usr/bin/env bash
# review-records.sh reads the review record lines of a branch's commit
# messages and decides, by fixed rules, what a review gate needs to know.
#
# Usage: bash review-records.sh coverage <LAST_REVIEWED> <HEAD> [<PLAN>]
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
#     LAST_REVIEWED is "none").
#   - A commit judged is known to be seen when it is any of:
#       * empty: one parent, and no difference from it;
#       * a plan checkbox commit: one parent, changing only PLAN, a regular
#         file before and after, by marking the PR progress checkboxes (the
#         "PR created", "PR merged", and "switched to the next branch"
#         lines) of a PR marker's section, or of the single-PR record of a
#         plan without markers, the way /runplan and /runpr mark them (see
#         checkbox_only below); never with PLAN absent;
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
#     and system git configuration or the attributes. Commits and blobs are
#     read with plumbing commands (cat-file, diff-tree --raw, rev-list), so
#     no diff driver, text conversion, or attribute applies.
#   - Failure: any failed git step, an argument of another form, or an
#     unreadable refs/remotes/origin/main prints "PARTIAL" alone (no
#     count), a diagnostic on standard error, and exits 1. A caller reads a
#     status other than 0, or a first line of another form, as PARTIAL.
#   - It uses no network, writes no file, and uses no bash feature newer
#     than 3.2.
#
# The rules this script executes are defined in the command documents, and
# those definitions win over this file: the seen-range line's form in
# .claude/commands/_lib/local-cli-review.md, step 7, and the rules in
# .claude/commands/runpr.md, step 3. When they disagree, the script is fixed.
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
export GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0

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
	if [ "$MODE" = coverage ]; then
		printf 'PARTIAL\n'
	fi
	exit 1
}

HEX40='^[0-9a-f]{40}$'
SEEN_RE='^Local CLI reviewed: ([0-9a-f]{12})\.\.([0-9a-f]{12}) \((coderabbit|codex)\)$'
PLAN_RE='^docs/tasks/[^/]+/03_implementation_plan\.md$'

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

# plan_lines prints each line of the blob $1 of a plan as "<kind>\t<line>",
# where <kind> is M for a line of a PR marker's section, S for a line of the
# checklist chapter (## 6) of a plan without PR markers, and O for any other
# line, a line inside a fenced code block and every heading and fence line
# included. A section runs from its heading to the next heading of any level
# (.claude/commands/_context.md, "PR marker conventions", part 2). An
# unclosed fence fails.
plan_lines() {
	g cat-file blob "$1" | awk '
		{
			sub(/\r$/, "")
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
				printf "%s\t%s\n", k, line[i]
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
# Any other change, to PLAN or to another path, is not.
checkbox_only() {
	local commit=$1 parent=$2 raw old new
	[ -n "$PLAN" ] || return 1
	raw=$(g diff-tree -r -z --raw --no-abbrev --no-renames --no-commit-id "$parent" "$commit" | tr '\0' '\n') || return 1
	[ "$raw" = ":100644 100644 $(g rev-parse --verify --quiet "$parent:$PLAN") $(g rev-parse --verify --quiet "$commit:$PLAN") M
$PLAN" ] || return 1
	old=$(plan_lines "$parent:$PLAN") || return 1
	new=$(plan_lines "$commit:$PLAN") || return 1
	{
		printf '%s\n' "$old" | awk '{ print "A\t" $0 }'
		printf '%s\n' "$new" | awk '{ print "B\t" $0 }'
	} | awk -F '\t' '
		{
			k = $2
			s = $0
			sub(/^[AB]\t[MSO]\t/, "", s)
			if ($1 == "A") { na++; ak[na] = k; al[na] = s } else { nb++; bk[nb] = k; bl[nb] = s }
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
					if (ak[i] != bk[i]) exit 1
					if (al[i] == bl[i]) continue
					if (ak[i] == "O" || !flip(al[i], bl[i])) exit 1
					changed++
				}
				exit changed ? 0 : 1
			}
			if (nb != na + 2) exit 1
			k = 1
			while (k <= na && ak[k] == bk[k] && al[k] == bl[k]) k++
			if (bk[k] != "S" || bk[k + 1] != "S") exit 1
			if (!created(bl[k]) || bl[k + 1] != "- [ ] PR がマージされた") exit 1
			for (i = k; i <= na; i++) if (ak[i] != bk[i + 2] || al[i] != bl[i + 2]) exit 1
			exit 0
		}
	'
}

# empty_commit reports whether commit $1 has no difference from its parent $2.
empty_commit() {
	local raw
	raw=$(g diff-tree -r -z --raw --no-renames --no-commit-id "$2" "$1" | tr '\0' '\n') || return 1
	[ -z "$raw" ]
}

coverage() {
	[ "$#" -eq 2 ] || [ "$#" -eq 3 ] || fail "coverage expects 2 or 3 arguments, got $#"
	local last=$1 head=$2 c line holder start end
	PLAN=${3-}
	[[ $last == none || $last =~ $HEX40 ]] || fail "LAST_REVIEWED is neither none nor 40 hexadecimal characters"
	[[ $head =~ $HEX40 ]] || fail "HEAD is not 40 hexadecimal characters"
	if [ -n "$PLAN" ]; then
		[[ $PLAN =~ $PLAN_RE ]] || fail "PLAN is not docs/tasks/<task-dir>/03_implementation_plan.md"
	fi

	local top
	top=$(g rev-parse --show-toplevel) || fail "not inside a git work tree"
	cd "$top" || fail "cannot enter the top of the work tree"

	local origin_main
	origin_main=$(g rev-parse --verify --quiet 'refs/remotes/origin/main^{commit}') ||
		fail "refs/remotes/origin/main does not name a commit"
	[ "$(g cat-file -t "$head" 2>/dev/null)" = commit ] || fail "HEAD names no commit"
	if [ "$last" != none ]; then
		[ "$(g cat-file -t "$last" 2>/dev/null)" = commit ] || fail "LAST_REVIEWED names no commit"
	fi
	REFS=$(g for-each-ref --format='%(refname)') || fail "git for-each-ref failed"

	local merge_base branch judged
	merge_base=$(g merge-base "$origin_main" "$head") || fail "origin/main and HEAD have no merge-base"
	branch=$(g rev-list "$head" "^$origin_main") || fail "git rev-list of the branch failed"
	if [ "$last" = none ]; then
		judged=$(g rev-list --reverse "$head" "^$origin_main") || fail "git rev-list failed"
	else
		judged=$(g rev-list --reverse "$head" "^$last" "^$origin_main") || fail "git rev-list failed"
	fi

	# seen holds every commit inside the range of a seen-range line that
	# passes the checks.
	local seen="" msg range
	for holder in $branch; do
		msg=$(message "$holder") || fail "cannot read a commit message"
		while IFS= read -r line; do
			[[ $line =~ $SEEN_RE ]] || continue
			start=$(resolve12 "${BASH_REMATCH[1]}") || continue
			end=$(resolve12 "${BASH_REMATCH[2]}") || continue
			g merge-base --is-ancestor "$start" "$end" || continue
			[ "$end" != "$holder" ] || continue
			g merge-base --is-ancestor "$end" "$holder" || continue
			[ "$start" = "$merge_base" ] || in_list "$start" "$branch" || continue
			in_list "$end" "$branch" || continue
			range=$(g rev-list "$end" "^$start") || fail "git rev-list of a seen range failed"
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

case $MODE in
coverage)
	shift
	coverage "$@"
	;;
*)
	fail "unknown mode: ${MODE:-(none)}"
	;;
esac
