#!/bin/sh
# Static checks for task 0032: PR titles in Japanese, squash subjects in
# English. Each check confirms that a rule file states a property the task's
# implementation plan names (03_implementation_plan.md, sections 2 and 5).
#
# Usage, from the repository root:
#   sh docs/tasks/0032_japanese_pr_title/check_rules.sh [<check-name>...]
# With no argument every check runs; with names, only those. The exit status is
# 0 when every check run passes, 1 when one fails, and 2 on a usage error (an
# unknown check name, an unknown CHECK_REV, or no git repository).
#
# CHECK_REV=<revision> reads the files of that revision's tree instead of the
# working tree.
#
# Text read from the documents is only matched (awk, grep); it is never run as
# a shell command.

set -u

# The checks, in the order they run.
ALL_CHECKS=''

die() {
	printf 'check_rules.sh: %s\n' "$1" >&2
	exit 2
}

# This script's own path, read by check_context_title_section; resolved before
# the cd below makes a relative $0 meaningless.
self=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")

top=$(git rev-parse --show-toplevel 2>/dev/null) || die 'not inside a git repository'
cd "$top" || die "cannot enter $top"

if [ -n "${CHECK_REV:-}" ]; then
	git rev-parse --verify --quiet "${CHECK_REV}^{tree}" >/dev/null ||
		die "unknown revision: $CHECK_REV"
fi

# fail PROPERTY: record that the running check found PROPERTY missing.
fail() {
	printf 'FAIL %s: %s\n' "$current" "$1"
	check_failed=1
}

# show FILE: print FILE from CHECK_REV's tree, or from the working tree.
show() {
	if [ -n "${CHECK_REV:-}" ]; then
		git show "${CHECK_REV}:$1" 2>/dev/null
	else
		cat -- "$1" 2>/dev/null
	fi
}

# repo_grep ARGS...: git grep over the tracked files outside docs/tasks/, in
# CHECK_REV's tree or the working tree.
repo_grep() {
	if [ -n "${CHECK_REV:-}" ]; then
		git grep "$@" "$CHECK_REV" -- . ':(exclude)docs/tasks'
	else
		git grep "$@" -- . ':(exclude)docs/tasks'
	fi
}

# section HEADING: print the Markdown section of stdin whose heading line is
# exactly HEADING (for example '### Plan PR'), up to the next heading of the
# same or a higher level. Lines inside code fences are never headings.
section() {
	H=$1 awk '
		BEGIN { h = ENVIRON["H"]; n = index(h, " ") - 1 }
		/^[ \t]*(```|~~~)/ { fence = !fence }
		on && !fence && /^#+ / { if (index($0, " ") - 1 <= n) exit }
		on { print }
		!fence && $0 == h { on = 1 }'
}

# between START END: print the lines of stdin from the first line matching the
# extended regular expression START up to, not including, the next line
# matching END.
between() {
	S=$1 E=$2 awk '
		BEGIN { s = ENVIRON["S"]; e = ENVIRON["E"] }
		on && $0 ~ e { exit }
		on { print; next }
		$0 ~ s { on = 1; print }'
}

# flat: join stdin into one line with single spaces, so a phrase matches even
# where the Markdown source wraps it.
flat() {
	tr '\n\t' '  ' | tr -s ' '
	echo
}

# has TEXT ERE PROPERTY: fail with PROPERTY unless TEXT matches ERE.
has() {
	printf '%s\n' "$1" | command grep -Eq -- "$2" || fail "$3"
}

# lacks TEXT ERE PROPERTY: fail with PROPERTY, and the matching lines, when
# TEXT matches ERE. An invalid ERE (grep status above 1) also fails.
lacks() {
	_found=$(printf '%s\n' "$1" | command grep -E -- "$2")
	case $? in
	0)
		fail "$3"
		printf '%s\n' "$_found" | sed 's/^/    /'
		;;
	1) ;;
	*) fail "grep failed on the pattern for: $3" ;;
	esac
}

# load FILE: set ctx to the text of FILE, or fail and return 1. It runs in the
# caller's shell, so a failure is recorded.
load() {
	ctx=$(show "$1") || {
		fail "cannot read $1"
		return 1
	}
}

CONTEXT=.claude/commands/_context.md
TITLE_HEADING='### PR title and squash subject'
REGEX_LABEL='Squash subject regular expression:'
# The accepted <type> values, as the requirements (AC-13) list them, sorted.
TYPES='build chore ci docs feat fix perf refactor revert style test'

# load_section FILE HEADING: set sec to the section HEADING of FILE, or fail and
# return 1 when FILE cannot be read or has no such section.
load_section() {
	load "$1" || return 1
	sec=$(printf '%s\n' "$ctx" | section "$2")
	[ -n "$sec" ] || {
		fail "no section '$2' in $1"
		return 1
	}
}

# subject_regex: print the squash-subject expression from the title section on
# stdin: the one line of the code block that follows the label line after a
# blank line. Prints nothing unless the label occurs exactly once and its block
# has exactly one line.
subject_regex() {
	L=$REGEX_LABEL awk '
		BEGIN { l = ENVIRON["L"] }
		$0 == l { n++; st = 1; next }
		st == 1 && $0 == "" { st = 2; next }
		st == 2 && $0 == "```" { st = 3; next }
		st == 3 { if ($0 == "```") st = 0; else { re = $0; st = 4 }; next }
		st == 4 { ok = ($0 == "```"); st = 0; next }
		st { st = 0 }
		END { if (n == 1 && ok) print re }'
}

# regex_types: print the sorted <type> alternatives of the expression on stdin.
regex_types() {
	sed -n 's/^(\([a-z|]*\)).*/\1/p' | tr '|' '\n' | sort | tr '\n' ' ' | sed 's/ $//'
}

# table_types: print the sorted <type> values of the accepted-prefix table in
# the title section on stdin.
table_types() {
	command grep -E '^\| `<type>` \|' | sed 's/^| `<type>` |//' |
		command grep -oE '`[a-z]+`' | tr -d '`' | sort | tr '\n' ' ' | sed 's/ $//'
}

check_context_title_section() {
	load_section "$CONTEXT" "$TITLE_HEADING" || return 0
	one=$(printf '%s\n' "$sec" | flat)
	# The six items of the item table.
	has "$sec" '^\| PR title \| `<type>\(<scope>\): <Japanese summary>`' \
		'item: PR title is <type>(<scope>): <Japanese summary>'
	has "$sec" '^\| Accepted prefix \| The prefix table below \|' \
		'item: accepted prefix refers to the prefix table'
	has "$sec" '^\| Task-document PR title \| `docs\(<task-id>\): <Japanese summary>`' \
		'item: task-document PR title is docs(<task-id>): <Japanese summary>'
	has "$sec" '^\| Plan PR title \|.*`<type>\(<task-id>\): `' \
		'item: plan PR title falls back to <type>(<task-id>): '
	has "$sec" '^\| `推奨タイトル` \|.*not a squash subject.*`<type>\(<task-id>\): <Japanese summary>`' \
		'item: 推奨タイトル is a PR title, not a squash subject, of form <type>(<task-id>): <Japanese summary>'
	has "$sec" '^\| Squash subject \| `<type>\(<scope>\): <English summary> \(#<number>\)` or `<type>: <English summary> \(#<number>\)`.*printable ASCII' \
		'item: squash subject forms, English, printable ASCII'
	# The accepted <type> values and the <scope> characters.
	rows=$(printf '%s\n' "$sec" | command grep -cE '^\| `<type>` \|')
	[ "$rows" -eq 1 ] || fail "expected one <type> row in the prefix table, found $rows"
	got=$(printf '%s\n' "$sec" | table_types)
	[ "$got" = "$TYPES" ] || fail "<type> table is '$got', expected '$TYPES'"
	has "$sec" '^\| `<scope>` \| One or more ASCII lowercase letters, digits, and `-`' \
		'<scope> is one or more ASCII lowercase letters, digits, and -'
	# A prefix that does not match is not repaired.
	has "$one" 'Do not repair it; treat the title as having no prefix' \
		'a non-matching prefix is not repaired but treated as absent'
	# A PR title always has a <scope>.
	has "$one" 'never uses the scope-less `<type>: ` form' \
		'a PR title a command writes always has a <scope>'
	# How a title is passed, and no backquotes.
	has "$one" 'Write the title with the Write tool to a file' \
		'the title is written to a file with the Write tool'
	has "$one" '`--title "\$\(cat "<title-file>"\)"`' \
		'the title is passed by command substitution from the file'
	has "$one" 'a PR title and a `推奨タイトル` never contain backquotes' \
		'no backquotes in a PR title or a 推奨タイトル'
	# The documented subject check is the one subject_ok runs, line for line.
	doc=$(printf '%s\n' "$sec" | between "^f='<subject-file>'\$" '^```$' | sed 1d)
	run=$(between '^subject_ok\(\) \{$' '^}$' <"$self" | sed 1,2d)
	[ -n "$doc" ] && [ "$doc" = "$run" ] ||
		fail 'the documented subject check differs from the commands subject_ok runs'
}

check_context_subject_regex() {
	load_section "$CONTEXT" "$TITLE_HEADING" || return 0
	# A subject expression is recognized by its (#<number>) tail; if that tail
	# is ever written differently, change this pattern with it.
	n=$(printf '%s\n' "$sec" | command grep -c '^(.*\\(#\[0-9\]+\\)$')
	[ "$n" -eq 1 ] || fail "expected one subject expression in the title section, found $n"
	re=$(printf '%s\n' "$sec" | subject_regex)
	[ -n "$re" ] || {
		fail "no single one-line code block after '$REGEX_LABEL'"
		return
	}
	got=$(printf '%s\n' "$re" | regex_types)
	want=$(printf '%s\n' "$sec" | table_types)
	[ "$got" = "$want" ] ||
		fail "<type> alternatives of the expression '$got' differ from the table '$want'"
}

# subject_ok FILE: the squash-subject check of "Checking a squash subject",
# with R set to the expression. check_context_title_section compares the lines
# after f=$1 with the documented commands, so keep them identical to that code
# block, indentation included.
subject_ok() {
f=$1
[ "$(awk 'END { print NR }' "$f")" = 1 ] &&
	[ -z "$(LC_ALL=C tr -d '\n -~' <"$f")" ] &&
	LC_ALL=C command grep -Eqx -- "$R" "$f"
}

check_subject_regex_samples() {
	load_section "$CONTEXT" "$TITLE_HEADING" || return 0
	R=$(printf '%s\n' "$sec" | subject_regex)
	[ -n "$R" ] || {
		fail "no subject expression to try"
		return
	}
	tmp=$(mktemp -d) || {
		fail 'mktemp failed'
		return
	}
	i=0
	# sample accept|reject PRINTF-FORMAT: write a subject file and check it.
	sample() {
		i=$((i + 1))
		# The format is a fixed literal of this script, never document text.
		# shellcheck disable=SC2059
		printf "$2" >"$tmp/$i"
		if subject_ok "$tmp/$i"; then _r=accept; else _r=reject; fi
		[ "$_r" = "$1" ] || fail "expected $1, got $_r: $(od -c "$tmp/$i" | sed -n 1p)"
	}
	n=0
	for t in $TYPES; do
		n=$((n + 1))
		if [ $((n % 2)) -eq 0 ]; then
			sample accept "$t(0032): write the rule (#415)\n"
		else
			sample accept "$t: write the rule (#7)\n"
		fi
	done
	sample accept 'feat(server-2): add a thing (#9)\n'
	sample accept 'docs(0032): draft the plan (#413)\n'
	sample accept 'fix: tidy (#1)'
	# Rejected in the plan's trial (1.3.2).
	sample reject 'docs\357\274\2100032\357\274\211: draft (#1)\n'
	sample reject 'docs(0032)\357\274\232 draft (#1)\n'
	sample reject 'd\320\276cs(0032): draft (#1)\n'
	sample reject 'docs(0032): \350\250\210\347\224\273\343\202\222\350\265\267\350\215\211\343\201\231\343\202\213 (#1)\n'
	sample reject 'Feat(0032): draft (#1)\n'
	sample reject 'feat(0032)!: draft (#1)\n'
	sample reject 'docs(0032):  draft (#1)\n'
	sample reject 'docs(0032): draft\343\200\200plan (#1)\n'
	sample reject 'docs(0032): draft  (#1)\n'
	# The design's other examples (3.1).
	sample reject 'docs(): draft (#1)\n'
	sample reject 'docs(Task): draft (#1)\n'
	sample reject 'docs(0032):\302\240draft (#1)\n'
	sample reject 'docs(0032\342\200\220x): draft (#1)\n'
	sample reject 'docs(0032\342\210\222x): draft (#1)\n'
	# Shapes the expression and the one-line check must refuse.
	sample reject 'feature(0032): draft (#1)\n'
	sample reject 'docs(0032): draft\n'
	sample reject 'docs(0032): draft (#1) \n'
	sample reject 'docs(0032):\tdraft (#1)\n'
	sample reject 'docs(0032): draft (#1)\nsecond line\n'
	sample reject 'docs(0032): draft (#1)\n\n'
	sample reject 'docs(0032): draft\r (#1)\n'
	sample reject 'docs(0032): draft (#1)\r\n'
	sample reject '\357\273\277docs(0032): draft (#1)\n'
	sample reject 'docs(0032):draft (#1)\n'
	sample reject 'feat!: draft (#1)\n'
	sample reject 'docs(0032):\343\200\200draft (#1)\n'
	sample reject 'docs(\357\274\220032): draft (#1)\n'
	sample reject 'docs(\343\201\202): draft (#1)\n'
	sample reject 'docs(0032): (#1)\n'
	sample reject 'docs(0032): draft (#)\n'
	sample reject ' docs(0032): draft (#1)\n'
	# The byte check alone: with a grep that accepts everything first on PATH
	# (command grep finds it there), only the tr line can reject these.
	mkdir "$tmp/stub" && printf '#!/bin/sh\nexit 0\n' >"$tmp/stub/grep" &&
		chmod +x "$tmp/stub/grep" || fail 'cannot write the stub grep'
	# byte_sample accept|reject PRINTF-FORMAT: sample, with the stub grep.
	byte_sample() {
		_path=$PATH
		PATH=$tmp/stub:$PATH
		sample "$@"
		PATH=$_path
	}
	byte_sample accept 'docs(0032): draft the plan (#413)\n'
	byte_sample reject 'docs(0032): draft (#1)\r\n'
	byte_sample reject 'docs(0032): draft\r (#1)\n'
	byte_sample reject '\357\273\277docs(0032): draft (#1)\n'
	byte_sample reject 'docs(0032):\tdraft (#1)\n'
	byte_sample reject 'docs(0032):\302\240draft (#1)\n'
	byte_sample reject 'docs(0032):\343\200\200draft (#1)\n'
	rm -rf "$tmp"
}

ALL_CHECKS="$ALL_CHECKS context_title_section context_subject_regex subject_regex_samples"

check_context_plan_pr() {
	load_section "$CONTEXT" '### Plan PR' || return 0
	one=$(printf '%s\n' "$sec" | flat)
	has "$one" '`03_implementation_plan\.md` has the PR.s URL on a `PR を作成した` line' \
		'definition: the PR URL on a plan'"'"'s PR を作成した line'
	has "$one" 'in the tree of the PR.s head' 'the tree read is the PR head'
	has "$one" 'Its task ID is the number of the task directory that holds that plan' \
		'the task ID is the number of the plan'"'"'s task directory'
	has "$one" '\*\*No match\*\*: not a plan PR' 'no match means not a plan PR'
	has "$one" 'Count only a line where the whole URL matches' 'only a whole-URL match counts'
	has "$one" 'Stop[^.]*when the matching lines are in the plans of two or more tasks' \
		'stop when two or more tasks'"'"' plans match'
	has "$one" 'Stop[^.]*when the tree cannot be read' 'stop when the tree cannot be read'
	has "$one" 'final PR\*\* when no `### PR-N 作成ポイント` marker after' 'the final-PR rule'
}

ALL_CHECKS="$ALL_CHECKS context_plan_pr"

check_context_marker_conventions() {
	load_section "$CONTEXT" '### PR marker conventions' || return 0
	line=$(printf '%s\n' "$sec" | command grep -E '^- `\*\*推奨タイトル\*\*`')
	has "$line" 'is a PR title, not a squash subject' \
		'推奨タイトル is a PR title, not a squash subject'
	has "$line" '"PR title and squash subject"' 'refers to the title section'
	lacks "$sec" 'Conventional commit scope format|<concise English title>' \
		'the old feat(<task-id>) marker line is gone'
}

ALL_CHECKS="$ALL_CHECKS context_marker_conventions"

# A sentence tying a title to English. A sentence ends at a period followed by
# a space, so a path such as _context.md inside it does not end it.
EN_TITLE='[Tt]itles?([^.]|\.[^ ])*English|English([^.]|\.[^ ])*[Tt]itles?'

check_claude_language() {
	load_section CLAUDE.md '## Language' || return 0
	gh=$(printf '%s\n' "$sec" | between '^- \*\*What lives on GitHub' '^- \*\*|^$' | flat)
	repo=$(printf '%s\n' "$sec" | between '^- \*\*What stays in the git repository' '^- \*\*|^$' | flat)
	has "$gh" '^- \*\*What lives on GitHub: Japanese\*\* — [^.]*PR titles' \
		'PR titles are among the Japanese things on GitHub'
	# No sentence of the GitHub bullet ties a title to English, and the old
	# wording is gone from the whole section.
	lacks "$gh" "$EN_TITLE" \
		'no exception keeps the PR title in English'
	lacks "$(printf '%s\n' "$sec" | flat)" 'PR title, which stays|stays an English|built from it' \
		'the old English-title exception is gone'
	has "$gh" '`\.claude/commands/_context\.md`, "PR title and squash subject"' \
		'refers to the title section'
	has "$repo" 'squash commit.s subject and body are English' \
		'the squash subject and body are English'
	has "$repo" '`/mergepr` writes the subject from the PR material' \
		'/mergepr writes the subject from the PR material'
	has "$repo" 'not by copying the PR title' '/mergepr does not copy the PR title'
	has "$gh" 'bot commands such as `@coderabbitai review`' 'the bot-command exception remains'
}

ALL_CHECKS="$ALL_CHECKS claude_language"

check_glossary_terms() {
	load docs/translation_glossary.md || return 0
	has "$ctx" '^\| 接頭辞 \| prefix \|' 'term: 接頭辞 = prefix'
	has "$ctx" '^\| 受け付ける接頭辞 \| accepted prefix \|' 'term: 受け付ける接頭辞 = accepted prefix'
	has "$ctx" '^\| 要約 \| summary \|' 'term: 要約 = summary'
	has "$ctx" '^\| プランの PR \| plan PR \|' 'term: プランの PR = plan PR'
	has "$ctx" '^\| 材料 \| material \|.*と、必要なときに読む diff）' \
		'term: 材料 includes the diff read when needed'
	has "$ctx" '^\| 材料 \| material \|.*PR のタイトルは含めない' \
		'term: 材料 excludes the PR title'
}

ALL_CHECKS="$ALL_CHECKS glossary_terms"

# The way every gh call in the commands passes a title (_context.md,
# "Passing a title to gh").
TITLE_ARG='--title "$(cat "<title-file>")"'

check_handoff_pr_text() {
	load .claude/commands/_lib/task-pr-handoff.md || return 0
	def=$(printf '%s\n' "$ctx" | between '^- \*\*PR_TEXT\*\*' '^- \*\*' | flat)
	b3=$(printf '%s\n' "$ctx" | between '^3\. Push and create the PR' '^4\. ' | flat)
	[ -n "$def" ] && [ -n "$b3" ] || {
		fail 'PR_TEXT or Part B step 3 not found'
		return 0
	}
	has "$def" 'for a task-document command, the "Task-document PR title" of `_context\.md`, "PR title and squash subject"' \
		'PR_TEXT refers to the task-document PR title of the title section'
	has "$def" 'for `runplan\.md`, the title its step 5a writes from the PR marker.s `推奨タイトル`' \
		'PR_TEXT for runplan.md is the title step 5a writes from 推奨タイトル'
	lacks "$b3" "$EN_TITLE" 'step 3 does not make the title English'
	has "$b3" 'title and the body to files in the scratchpad with the Write tool' \
		'step 3 writes the title to a file with the Write tool'
	has "$b3" "$(quote_ere "$TITLE_ARG")" \
		'step 3 passes the title by command substitution'
	has "$b3" 'follows `_context\.md`, "PR title and squash subject"' \
		'step 3 refers to the title section for the form'
}

ALL_CHECKS="$ALL_CHECKS handoff_pr_text"

check_runplan_title() {
	load .claude/commands/runplan.md || return 0
	s5a=$(printf '%s\n' "$ctx" | between '^5a\. ' '^[0-9]' | flat)
	s8=$(printf '%s\n' "$ctx" | between '^8\. ' '^[0-9]' | flat)
	[ -n "$s5a" ] && [ -n "$s8" ] || {
		fail 'step 5a or step 8 not found'
		return 0
	}
	has "$s5a" 'the PR marker.s `推奨タイトル`, which is a draft' 'step 5a treats 推奨タイトル as a draft'
	has "$s5a" 'starts with an accepted prefix that has a `<scope>`, keep that prefix' \
		'step 5a: an accepted prefix with a <scope> is kept'
	has "$s5a" 'starts with an accepted prefix without a `<scope>`[^.]*the prefix is `<type>\(<task-id>\): `' \
		'step 5a: an accepted prefix without a <scope> becomes <type>(<task-id>): '
	has "$s5a" 'starts with no accepted prefix, the prefix is `<type>\(<task-id>\): `' \
		'step 5a: no accepted prefix gives <type>(<task-id>): '
	has "$s5a" 'Write the summary in Japanese' 'step 5a writes the summary in Japanese'
	has "$s5a" 'Do not rewrite the plan' 'step 5a does not rewrite the plan'
	has "$s8" 'the title is a plan PR title \(`_context\.md`, "PR title and squash subject"\)' \
		'step 8 refers to the plan PR title of the title section'
}

ALL_CHECKS="$ALL_CHECKS runplan_title"

# quote_ere TEXT: print TEXT as an ERE matching it literally.
quote_ere() {
	printf '%s' "$1" | sed 's/[][\.*^$()|+?{}]/\\&/g'
}

check_runpr_title() {
	load .claude/commands/runpr.md || return 0
	s1=$(printf '%s\n' "$ctx" | section '## 1. Resolve the PR, and create it for a standalone change' | flat)
	s2=$(printf '%s\n' "$ctx" | section "## 2. Decide the PR's bookkeeping and next command" | flat)
	s4=$(printf '%s\n' "$ctx" | section '## 4. Confirm, record, and merge' |
		between '^2\. Ask with one' '^3\. ' | flat)
	[ -n "$s1" ] && [ -n "$s2" ] && [ -n "$s4" ] || {
		fail 'step 1, step 2, or step 4 item 2 not found'
		return 0
	}
	has "$s1" 'Draft a PR title in the form `_context\.md`, "PR title and squash subject", gives' \
		'step 1 writes the title in the PR title form of the title section'
	has "$s1" 'Japanese summary after a prefix whose `<type>\(<scope>\)` matches the style of `git log origin/main`' \
		'step 1: Japanese summary, <type>(<scope>) as in git log origin/main'
	has "$s1" "$(quote_ere "$TITLE_ARG")" 'step 1 passes the title from a file'
	lacks "$s1" "$EN_TITLE" 'step 1 does not make the title English'
	has "$s2" '\*\*Plan PR\*\* — a plan PR as `_context\.md`, "Plan PR", defines it' \
		'step 2 refers to the plan PR judgment of _context.md'
	has "$s4" 'the record of which rule decided the subject.s prefix' \
		'step 4 item 2 shows the record of the prefix rule'
}

ALL_CHECKS="$ALL_CHECKS runpr_title"

check_fixpr_title() {
	load_section .claude/commands/fixpr.md '## Phase 6 — Wrap: PR description (model: haiku)' || return 0
	one=$(printf '%s\n' "$sec" | sed 's/^> \{0,1\}//' | flat)
	has "$one" 'read the section "PR title and squash subject" of `\.claude/commands/_context\.md`' \
		'the prompt names the title section of _context.md to read'
	has "$one" 'write the new title in its PR title form' 'the new title is in the PR title form'
	has "$one" 'Only if the description is significantly stale' 'the stale-description condition stays'
	has "$one" 'drafted a concrete final title and body' 'the concrete-text condition stays'
	has "$one" "$(quote_ere "$TITLE_ARG")" 'gh pr edit takes the title from a file'
	lacks "$one" "$EN_TITLE" 'the prompt does not keep the title English'
}

ALL_CHECKS="$ALL_CHECKS fixpr_title"

check_mkplan2_recommended_title() {
	load .claude/commands/mkplan2.md || return 0
	has "$ctx" '^   \*\*推奨タイトル\*\*: `<type>\(<task-id>\): <Japanese summary>`$' \
		'the inserted marker template is <type>(<task-id>): <Japanese summary>'
	has "$(printf '%s\n' "$ctx" | flat)" \
		'- `\*\*推奨タイトル\*\*` is the Japanese title `runplan\.md` step 5a writes the PR title from, in the `推奨タイトル` form of `_context\.md`, "PR title and squash subject"' \
		'the field description refers to the title section'
	has "$ctx" '^- \[ \] `推奨タイトル` values match the `推奨タイトル` form of `_context\.md`, "PR title and squash subject", and contain no backquotes' \
		'the review checklist checks the form and no backquotes'
	lacks "$ctx" 'concise English title|conventional commit titles? in English' \
		'no English 推奨タイトル remains'
}

ALL_CHECKS="$ALL_CHECKS mkplan2_recommended_title"

for name in "$@"; do
	case $name in
	'' | *[!a-z0-9_]*) die "unknown check: '$name'" ;;
	esac
	case " $ALL_CHECKS " in
	*" $name "*) ;;
	*) die "unknown check: $name" ;;
	esac
done

[ $# -gt 0 ] || set -- $ALL_CHECKS

failed=0
for name in "$@"; do
	current=$name
	check_failed=0
	"check_$name" || fail "the check stopped with status $?"
	if [ "$check_failed" -eq 0 ]; then
		printf 'ok   %s\n' "$name"
	else
		failed=1
	fi
done
exit "$failed"
