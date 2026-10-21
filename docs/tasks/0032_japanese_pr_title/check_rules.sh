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
		/^```/ { fence = !fence }
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
# TEXT matches ERE.
lacks() {
	_found=$(printf '%s\n' "$1" | command grep -E -- "$2") || return 0
	fail "$3"
	printf '%s\n' "$_found" | sed 's/^/    /'
}

CONTEXT=.claude/commands/_context.md
TITLE_HEADING='### PR title and squash subject'
REGEX_LABEL='Squash subject regular expression:'
# The accepted <type> values, as the requirements (AC-13) list them, sorted.
TYPES='build chore ci docs feat fix perf refactor revert style test'

# title_section: print the title section of _context.md, or fail.
title_section() {
	_ctx=$(show "$CONTEXT") || {
		fail "cannot read $CONTEXT"
		return 1
	}
	printf '%s\n' "$_ctx" | section "$TITLE_HEADING"
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
	sec=$(title_section) || return
	[ -n "$sec" ] || {
		fail "no section '$TITLE_HEADING' in $CONTEXT"
		return
	}
	one=$(printf '%s\n' "$sec" | flat)
	# The six items of the item table.
	has "$sec" '^\| PR title \| `<type>\(<scope>\): <Japanese summary>`' \
		'item: PR title is <type>(<scope>): <Japanese summary>'
	has "$sec" '^\| Accepted prefix \|' 'item: accepted prefix'
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
	has "$one" '`--title "\$\(cat <title-file>\)"`' \
		'the title is passed by command substitution from the file'
	has "$one" 'a PR title and a `推奨タイトル` never contain backquotes' \
		'no backquotes in a PR title or a 推奨タイトル'
}

check_context_subject_regex() {
	sec=$(title_section) || return
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
	[ "$got" = "$TYPES" ] || fail "<type> alternatives are '$got', expected '$TYPES'"
}

# subject_ok FILE: the squash-subject check the title section describes: FILE
# holds exactly one line, and it matches the expression in the C locale.
subject_ok() {
	[ "$(awk 'END { print NR }' "$1")" = 1 ] &&
		LC_ALL=C command grep -Eqx -- "$re" "$1"
}

check_subject_regex_samples() {
	sec=$(title_section) || return
	re=$(printf '%s\n' "$sec" | subject_regex)
	[ -n "$re" ] || {
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
	rm -rf "$tmp"
}

ALL_CHECKS="$ALL_CHECKS context_title_section context_subject_regex subject_regex_samples"

check_context_plan_pr() {
	_ctx=$(show "$CONTEXT") || {
		fail "cannot read $CONTEXT"
		return
	}
	sec=$(printf '%s\n' "$_ctx" | section '### Plan PR')
	[ -n "$sec" ] || {
		fail "no section '### Plan PR' in $CONTEXT"
		return
	}
	one=$(printf '%s\n' "$sec" | flat)
	has "$one" '`03_implementation_plan\.md` has the PR.s URL on a `PR を作成した` line' \
		'definition: the PR URL on a plan'"'"'s PR を作成した line'
	has "$one" 'in the tree of the PR.s head' 'the tree read is the PR head'
	has "$one" 'Count only a line where the whole URL matches' 'only a whole-URL match counts'
	has "$one" 'Stop[^.]*when the matching lines are in the plans of two or more tasks' \
		'stop when two or more tasks'"'"' plans match'
	has "$one" 'Stop[^.]*when the tree cannot be read' 'stop when the tree cannot be read'
	has "$one" 'final PR\*\* when no `### PR-N 作成ポイント` marker after' 'the final-PR rule'
}

ALL_CHECKS="$ALL_CHECKS context_plan_pr"

for name in "$@"; do
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
	"check_$name"
	if [ "$check_failed" -eq 0 ]; then
		printf 'ok   %s\n' "$name"
	else
		failed=1
	fi
done
exit "$failed"
