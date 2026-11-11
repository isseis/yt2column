#!/usr/bin/env bash
# One-off check for task 0081, plan step 2-4 (and 3-7): runs the decision
# script, .claude/commands/_lib/opus-review-decision.sh, in fixture
# repositories built for each case, and compares its standard output, its
# standard error being empty or not, and its exit status with what each case
# expects.
#
# Usage: bash decision_cases.sh <compared-commit>
# Run from the top level of the work tree. The decision script, the
# _context.md of every fixture's origin/main, and the real plans are taken
# from <compared-commit>. DECISION_SCRIPT=<file> runs another copy of the
# script instead (used only to check that each case fails when the script is
# broken). Exit 0 when every case passes.
#
# Output: one line per case, PASS or FAIL, with the expected and the received
# summary of the output (its first line, and for SKIP the number and subjects
# of the listed commits). Fixture commit IDs are never printed.
set -euo pipefail

if [ "$#" -ne 1 ]; then
	echo "usage: $0 <compared-commit>" >&2
	exit 2
fi
compared=$(git rev-parse --verify "$1^{commit}")
repo=$(pwd)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

if [ -n "${DECISION_SCRIPT:-}" ]; then
	script=$DECISION_SCRIPT
else
	script=$work/opus-review-decision.sh
	git show "$compared:.claude/commands/_lib/opus-review-decision.sh" >"$script"
fi
git show "$compared:.claude/commands/_context.md" >"$work/context-real.md"
git show "$compared:docs/tasks/0077_openai_llm_client/03_implementation_plan.md" >"$work/plan-0077.md"
git show "$compared:docs/tasks/0071_security_source_link/03_implementation_plan.md" >"$work/plan-0071.md"

# Variants of the real _context.md: the old-value table removed, one row
# malformed, and one old value given two rows.
grep -Ev '^ *\| `(frontier-required|frontier-recommended|standard)` \|' \
	"$work/context-real.md" >"$work/context-notable.md"
sed 's/^\( *\)| `standard` | `sonnet` |$/\1| `standard` | sonnet |/' \
	"$work/context-real.md" >"$work/context-malformed.md"
perl -pe 's/^( *)\| `standard` \| `sonnet` \|\n/$1| `standard` | `opus` |\n$&/' \
	"$work/context-real.md" >"$work/context-duplicate.md"
for v in notable malformed duplicate; do
	if cmp -s "$work/context-real.md" "$work/context-$v.md"; then
		echo "setup: the $v variant of _context.md did not change it" >&2
		exit 2
	fi
done

export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=fixture GIT_AUTHOR_EMAIL=fixture@example.invalid
export GIT_COMMITTER_NAME=fixture GIT_COMMITTER_EMAIL=fixture@example.invalid
export GIT_AUTHOR_DATE='2026-01-01T00:00:00Z' GIT_COMMITTER_DATE='2026-01-01T00:00:00Z'

PLAN=docs/tasks/9999_fixture/03_implementation_plan.md
OPUS=claude-opus-fixture
SONNET=claude-sonnet-fixture
failures=0
n=0

# fixture <context-variant|none> <origin-main-plan-file|none>: a new
# repository whose origin/main holds that _context.md and plan, with HEAD on
# a branch at origin/main.
fixture() {
	n=$((n + 1))
	dir=$work/repo$n
	mkdir -p "$dir"
	cd "$dir"
	git init -q -b main
	mkdir -p .claude/commands docs/tasks/9999_fixture
	echo base >code.txt
	[ "$1" = none ] || cp "$work/context-$1.md" .claude/commands/_context.md
	[ "$2" = none ] || cp "$2" "$PLAN"
	git add -A
	git commit -q -m base
	git update-ref refs/remotes/origin/main HEAD
	git switch -q -c work
}

# Commits on the branch.
code_commit() { # <subject>: changes a file other than the plan
	echo "$1" >>code.txt
	git add code.txt
	git commit -q -m "$1"
}
plan_commit() { # <plan-file> <subject>: the plan with a checkbox line added
	cp "$1" "$PLAN"
	printf -- "- [x] %s\n" "$2" >>"$PLAN"
	git add "$PLAN"
	git commit -q -m "$2"
}
readme_commit() { # <subject>
	echo "$1" >>README.en.md
	git add README.en.md
	git commit -q -m "$1"
}
record_commit() { # <subject> <record line>...: an empty commit
	local subject=$1
	shift
	printf '%s\n\n' "$subject" >"$work/msg"
	printf '%s\n' "$@" >>"$work/msg"
	git commit -q --allow-empty -F "$work/msg"
}
id12() { git rev-parse --short=12 "$1"; }
base12() { id12 refs/remotes/origin/main; }
impl_line() { # <start-rev> <end-rev> <model-ids>
	echo "Implementation model: $(id12 "$1")..$(id12 "$2") $3"
}
done_line() { # <start-rev> <end-rev>
	echo "Opus review: done $(id12 "$1")..$(id12 "$2") (critical 0, major 1, minor 2)"
}

# Plan writers.
marker_plan() { # <file> <N>:<value>... ; value "-" leaves out the field
	local spec num value
	{
		printf '# plan\n\n## 2. steps\n\n### phase 1: a\n\n- [x] step\n\n'
		for spec in "${@:2}"; do
			num=${spec%%:*}
			value=${spec#*:}
			printf '### PR-%s 作成ポイント: pr %s\n\n**steps**: 1-1\n\n' "$num" "$num"
			[ "$value" = - ] || printf '**実装モデル要件**: %s\n\n' "$value"
			printf '**reason-field**: reason\n\n- [ ] box\n\n'
		done
		printf '## 3. order\n\n### 3.2 PR 構成\n\n| PR | value |\n|---|---|\n'
	} >"$1"
}
single_plan() { # <file> <heading> <value|-> [<chapter of the section>]
	local chapter=${4:-3}
	{
		printf '# plan\n\n## 2. steps\n\n### phase 1: a\n\n- [x] step\n\n'
		[ "$chapter" = 2 ] && printf '%s\n\n' "$2"
		[ "$chapter" = 2 ] && [ "$3" != - ] && printf '**実装モデル要件**: %s\n\n' "$3"
		printf '## 3. order\n\n### 3.1. milestones\n\ntext\n\n'
		if [ "$chapter" = 3 ]; then
			printf '%s\n\none PR\n\n' "$2"
			[ "$3" = - ] || printf '**実装モデル要件**: %s\n\n**reason-field**: reason\n\n' "$3"
		fi
		printf '## 4. tests\n\ntext\n'
	} >"$1"
}

# summary prints the first line, and for SKIP the subjects of the listed
# commits, without the IDs.
summary() {
	local first rest
	first=$(head -n 1 "$1")
	case $first in
	SKIP\ *)
		rest=$(tail -n +2 "$1" | sed 's/^[0-9a-f]\{12\} //' | paste -sd ',' -)
		echo "SKIP [$rest]"
		;;
	*) echo "$first" ;;
	esac
}

# expect <case> <expected summary> <ok|fail> [<plan> <target> [<base>]]: runs
# the script in the current fixture (BASE defaults to the merge-base); "fail"
# expects a reason on standard error, "ok" expects none.
expect() {
	local name=$1 want=$2 kind=$3 plan=${4:-$PLAN} target=${5:-1} status=0 got errs
	local base=${6:-}
	[ -n "$base" ] || base=$(git merge-base refs/remotes/origin/main HEAD 2>/dev/null || echo none)
	bash "$script" "$plan" "$target" "$base" >"$work/out" 2>"$work/err" || status=$?
	got=$(summary "$work/out")
	errs=ok
	[ -s "$work/err" ] && errs=fail
	if [ "$got" = "$want" ] && [ "$status" = 0 ] && [ "$errs" = "$kind" ]; then
		echo "PASS $name: $want ($kind)"
	else
		echo "FAIL $name: expected '$want' ($kind, exit 0), got '$got' ($errs, exit $status)"
		failures=$((failures + 1))
	fi
	cd "$repo"
}

p=$work/plan

# --- The value: the PR marker ---------------------------------------------
marker_plan "$p" 1:opus
fixture real "$p"; code_commit c1
expect value-opus-uncovered RUN ok

marker_plan "$p" 1:sonnet
fixture real "$p"; code_commit c1
expect value-sonnet "NONE value-sonnet" ok

marker_plan "$p" 1:-
fixture real "$p"; code_commit c1
expect value-field-missing "NONE value-absent" ok

marker_plan "$p" 1:sonnet+opus-review
fixture real "$p"; code_commit c1
expect value-sonnet-opus-review-uncovered RUN ok

marker_plan "$p" "1:  sonnet	"
fixture real "$p"; code_commit c1
expect value-surrounding-space "NONE value-sonnet" ok

marker_plan "$p" "1:sonnet (note)"
fixture real "$p"; code_commit c1
expect value-unreadable RUN ok

marker_plan "$p" 1:
fixture real "$p"; code_commit c1
expect value-empty-is-unreadable RUN ok

marker_plan "$p" 1:Sonnet
fixture real "$p"; code_commit c1
expect value-case-differs-is-unreadable RUN ok

marker_plan "$p" 1:sonnet 10:opus
fixture real "$p"; code_commit c1
expect marker-pr1-not-pr10 "NONE value-sonnet" ok

marker_plan "$p" 1:- 2:opus
fixture real "$p"; code_commit c1
expect marker-section-ends-at-next-heading "NONE value-absent" ok

marker_plan "$p" 2:sonnet
fixture real "$p"; code_commit c1
expect marker-missing-at-head RUN fail

marker_plan "$p" 1:sonnet 1:sonnet
fixture real none; plan_commit "$p" "add the plan"; code_commit c1
expect marker-twice RUN fail

marker_plan "$p" 1:sonnet
sed 's/^\*\*reason-field\*\*: reason$/**実装モデル要件**: sonnet/' "$p" >"$p.2"
fixture real "$p.2"; code_commit c1
expect value-line-twice RUN fail

marker_plan "$p" 1:sonnet
awk '{ print } /^\*\*reason-field\*\*/ { print "\n```text\n### PR-1 作成ポイント: fake\n**実装モデル要件**: opus\n```" }' "$p" >"$p.2"
fixture real "$p.2"; code_commit c1
expect fenced-block-not-read "NONE value-sonnet" ok

marker_plan "$p" 1:sonnet
sed 's/^\*\*実装モデル要件\*\*: sonnet$/  **実装モデル要件**: opus/' "$p" >"$p.2"
fixture real "$p.2"; code_commit c1
expect value-not-in-first-column "NONE value-absent" ok

# --- The value: origin/main and HEAD --------------------------------------
marker_plan "$p" 1:opus
marker_plan "$p.2" 1:sonnet
fixture real "$p"; plan_commit "$p.2" "lower the value"; code_commit c1
expect origin-main-opus-head-sonnet RUN ok

marker_plan "$p" 1:opus
marker_plan "$p.2" 1:-
fixture real "$p"; plan_commit "$p.2" "remove the value"; code_commit c1
expect origin-main-opus-head-absent RUN ok

marker_plan "$p" 1:sonnet
marker_plan "$p.2" 1:-
fixture real "$p"; plan_commit "$p.2" "remove the value"; code_commit c1
expect origin-main-sonnet-head-absent "NONE value-sonnet" ok

marker_plan "$p" 1:sonnet
fixture real none; plan_commit "$p" "add the plan"; code_commit c1
expect origin-main-without-plan "NONE value-sonnet" ok

marker_plan "$p" 2:opus
marker_plan "$p.2" 1:sonnet 2:opus
fixture real "$p"; plan_commit "$p.2" "add PR-1"; code_commit c1
expect origin-main-without-marker "NONE value-sonnet" ok

marker_plan "$p" 1:sonnet 1:opus
marker_plan "$p.2" 1:sonnet
fixture real "$p"; plan_commit "$p.2" "dedupe"; code_commit c1
expect origin-main-marker-twice RUN fail

# --- The value: old values and the table ----------------------------------
marker_plan "$p" 1:standard
fixture real "$p"; code_commit c1
expect old-standard "NONE value-sonnet" ok

marker_plan "$p" 1:frontier-recommended
fixture real "$p"; code_commit c1
expect old-frontier-recommended RUN ok

marker_plan "$p" 1:frontier-required
fixture real "$p"; code_commit c1
expect old-frontier-required RUN ok

marker_plan "$p" 1:standard
fixture notable "$p"; code_commit c1
expect old-without-table RUN ok

marker_plan "$p" 1:standard
fixture malformed "$p"; code_commit c1
expect old-malformed-row RUN ok

marker_plan "$p" 1:standard
fixture duplicate "$p"; code_commit c1
expect old-two-rows RUN ok

marker_plan "$p" 1:standard
fixture none "$p"; code_commit c1
expect old-without-context RUN ok

marker_plan "$p" 1:
fixture notable "$p"; code_commit c1
expect empty-value-without-table RUN ok

# --- The value: a single-phase plan ---------------------------------------
single_plan "$p" '### 3.2 PR 構成' sonnet
fixture real "$p"; code_commit c1
expect single-heading-3.2 "NONE value-sonnet" ok "$PLAN" single

single_plan "$p" '### 3.2. PR 構成' sonnet
fixture real "$p"; code_commit c1
expect single-heading-3.2-dot "NONE value-sonnet" ok "$PLAN" single

single_plan "$p" '### 3.2. PR 構成' opus
fixture real "$p"; code_commit c1
expect single-opus-uncovered RUN ok "$PLAN" single

single_plan "$p" '### 3.2 PR 構成' opus 2
fixture real "$p"; code_commit c1
expect single-section-outside-chapter-3 "NONE value-absent" ok "$PLAN" single

single_plan "$p" '### 3.2 PR 構成' sonnet
perl -pe 's/^\*\*reason-field\*\*: reason\n/$&\n#### 3.2.1 note\n\n**実装モデル要件**: opus\n/' "$p" >"$p.2"
fixture real "$p.2"; code_commit c1
expect single-section-ends-at-subheading "NONE value-sonnet" ok "$PLAN" single

single_plan "$p" '### 3.2 PR 構成' sonnet
perl -pe 's/^### 3.1. milestones\n/### 3.0 PR 構成\n\n**実装モデル要件**: sonnet\n/' "$p" >"$p.2"
fixture real "$p.2"; code_commit c1
expect single-section-twice RUN fail "$PLAN" single

single_plan "$p" '### 3.2 PR 構成' sonnet
cat "$p" >"$p.2"; marker_plan "$p.3" 1:sonnet; grep '^### PR-' "$p.3" >>"$p.2"
fixture real "$p.2"; code_commit c1
expect single-on-plan-with-markers RUN fail "$PLAN" single

marker_plan "$p" 1:sonnet
perl -pe 's/^(### PR-1 \S+): pr 1\n/$1\n/; s/\n/\r\n/' "$p" >"$p.2"
fixture real "$p.2"; code_commit c1
expect marker-crlf-lines "NONE value-sonnet" ok

marker_plan "$p" 1:sonnet
awk '{ print } /^\*\*reason-field\*\*/ { print "\n~~~text\n**実装モデル要件**: opus\n~~~" }' "$p" >"$p.2"
fixture real "$p.2"; code_commit c1
expect tilde-fence-not-read "NONE value-sonnet" ok

marker_plan "$p" 1:sonnet
printf '\n```text\nnot closed\n' >>"$p"
fixture real "$p"; code_commit c1
expect unclosed-fence RUN fail

# The plan arguments below name the fixture plan by a different path form.
marker_plan "$p" 1:sonnet
fixture real "$p"; code_commit c1
expect plan-path-not-canonical RUN fail "./$PLAN" 1

# --- Records ----------------------------------------------------------------
marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1; code_commit c2
record_commit "record" "$(impl_line "$s" HEAD "$OPUS")"
plan_commit "$p" "checkbox"
readme_commit "readme sync"
record_commit "empty"
expect all-opus-with-excludable "NONE all-opus" ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "$(impl_line "$s" HEAD "$OPUS, $SONNET")"
expect record-mixed-models RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "$(impl_line "$s" HEAD "$SONNET")"
expect record-sonnet RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "$(impl_line "$s" HEAD "$OPUS")"
code_commit c2
expect commit-after-record RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "$(impl_line "$s" HEAD "$OPUS")" "$(impl_line "$s" HEAD "$SONNET")"
expect overlapping-opus-and-sonnet RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1; m=$(git rev-parse HEAD); code_commit c2
record_commit "record" "$(impl_line "$s" "$m" "$OPUS")" "$(impl_line "$m" HEAD "$OPUS")"
expect two-opus-records "NONE all-opus" ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "  $(impl_line "$s" HEAD "$OPUS")"
expect record-indented RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "Implementation model: $(git rev-parse --short=10 "$s")..$(git rev-parse --short=10 HEAD) $OPUS"
expect record-short-ids RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "Implementation model: $(id12 "$s")..$(id12 HEAD)"
expect record-without-model RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "$(impl_line "$s" HEAD "$OPUS,$OPUS")"
expect record-comma-without-space RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "$(impl_line "$s" HEAD "$OPUS") trailing"
expect record-trailing-word RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1
record_commit "record" "Implementation model: 0123456789ab..$(id12 HEAD) $OPUS"
expect record-unknown-start RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD)
echo plan-and-code >>code.txt; cp "$p" "$PLAN"; echo x >>"$PLAN"; git add -A; git commit -q -m "plan and code"
expect plan-and-code-not-excludable RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD)
git switch -q -c side; echo side >side.txt; git add side.txt; git commit -q -m side1; git switch -q work; code_commit c1
git merge -q --no-edit side
record_commit "record" "$(impl_line "$s" HEAD "$OPUS")"
expect merge-commit-covered RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
s=$(git rev-parse HEAD); code_commit c1; old=$(git rev-parse HEAD)
git reset -q --hard "$s"; code_commit "c1 rewritten"
record_commit "record of the old c1" "$(impl_line "$s" "$old" "$OPUS")"
expect rewritten-out-of-range RUN ok

# --- Done records -----------------------------------------------------------
marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1
record_commit "checkbox and done" "$(done_line refs/remotes/origin/main HEAD)"
plan_commit "$p" "checkbox 2"
readme_commit "readme sync"
expect done-then-excludable "SKIP [checkbox and done,checkbox 2,readme sync]" ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1
record_commit "checkbox and done" "$(done_line refs/remotes/origin/main HEAD)"
code_commit "green-gate fix"
expect done-then-code RUN ok

marker_plan "$p" 1:sonnet
fixture real "$p"
code_commit c1
record_commit "checkbox and done" "$(done_line refs/remotes/origin/main HEAD)"
expect done-with-sonnet-value "NONE value-sonnet" ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1
record_commit "old done" "$(done_line refs/remotes/origin/main HEAD)"
code_commit c2
record_commit "new done" "$(done_line refs/remotes/origin/main HEAD)"
expect last-done-wins "SKIP [new done]" ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1
record_commit "new done first" "$(done_line refs/remotes/origin/main HEAD)"
code_commit c2
record_commit "stale done last" "$(done_line refs/remotes/origin/main HEAD~2)"
expect last-done-is-newest-commit RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1
record_commit "done" "Opus review: done 0123456789ab..$(id12 HEAD) (critical 0, major 1, minor 2)"
expect done-unknown-start RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1; c1=$(git rev-parse HEAD); code_commit c2
record_commit "done" "$(done_line "$c1" HEAD)"
expect done-start-after-base RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1; git switch -q -c gone; code_commit gone1; g=$(git rev-parse HEAD); git switch -q work
record_commit "done" "$(done_line refs/remotes/origin/main "$g")"
expect done-end-not-ancestor RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1
record_commit "done" "Opus review: done $(base12)..$(id12 HEAD) (critical 0, major 1)"
expect done-malformed RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1
record_commit "done" "x Opus review: done $(base12)..$(id12 HEAD) (critical 0, major 1, minor 2)"
expect done-not-at-line-start RUN ok

marker_plan "$p" 1:opus
fixture real "$p"
code_commit c1
record_commit "checkbox and done" "$(done_line refs/remotes/origin/main HEAD)"
git switch -q -c side; plan_commit "$p" "side checkbox"; git switch -q work; record_commit "empty"
git merge -q --no-edit side
expect done-then-merge RUN ok

marker_plan "$p" 1:sonnet
fixture real "$p"; code_commit c1; code_commit c2
expect base-not-merge-base RUN fail "$PLAN" 1 "$(git rev-parse HEAD~1)"

# --- Failures -------------------------------------------------------------
marker_plan "$p" 1:sonnet
fixture real "$p"; code_commit c1
git update-ref -d refs/remotes/origin/main
bash "$script" "$PLAN" 1 HEAD~1 >"$work/out" 2>"$work/err" && st=0 || st=$?
if [ "$(summary "$work/out")" = RUN ] && [ -s "$work/err" ] && [ "$st" = 0 ]; then
	echo "PASS no-origin-main: RUN (fail)"
else
	echo "FAIL no-origin-main: expected 'RUN' (fail, exit 0), got '$(summary "$work/out")' (exit $st)"
	failures=$((failures + 1))
fi
cd "$repo"

single_plan "$p" '### 3.2 PR 構成' sonnet
fixture real none; code_commit c1
expect plan-missing-at-head RUN fail "$PLAN" single

marker_plan "$p" 1:sonnet
fixture real "$p"; code_commit c1
expect target-not-a-number RUN fail "$PLAN" '1|2'

marker_plan "$p" 1:sonnet
fixture real "$p"; code_commit c1; git switch -q -c other main; code_commit other1; git switch -q work
bash "$script" "$PLAN" 1 other >"$work/out" 2>"$work/err" && st=0 || st=$?
if [ "$(summary "$work/out")" = RUN ] && [ -s "$work/err" ] && [ "$st" = 0 ]; then
	echo "PASS base-not-ancestor: RUN (fail)"
else
	echo "FAIL base-not-ancestor: expected 'RUN' (fail, exit 0), got '$(summary "$work/out")' (exit $st)"
	failures=$((failures + 1))
fi
cd "$repo"

mkdir -p "$work/notrepo"
cd "$work/notrepo"
bash "$script" "$PLAN" 1 HEAD >"$work/out" 2>"$work/err" && st=0 || st=$?
if [ "$(summary "$work/out")" = RUN ] && [ -s "$work/err" ] && [ "$st" = 0 ]; then
	echo "PASS not-a-repository: RUN (fail)"
else
	echo "FAIL not-a-repository: expected 'RUN' (fail, exit 0), got '$(summary "$work/out")' (exit $st)"
	failures=$((failures + 1))
fi
cd "$repo"

# --- Real plans -------------------------------------------------------------
# 0077: PR-1 standard, PR-2 frontier-required, PR-3 frontier-recommended.
fixture real "$work/plan-0077.md"; code_commit c1
expect real-0077-pr1-standard "NONE value-sonnet" ok
fixture real "$work/plan-0077.md"; code_commit c1
expect real-0077-pr2-frontier-required RUN ok "$PLAN" 2
fixture real "$work/plan-0077.md"; code_commit c1
expect real-0077-pr3-frontier-recommended RUN ok "$PLAN" 3
# 0071: a single-phase plan whose PR 構成 section has no value line.
fixture real "$work/plan-0071.md"; code_commit c1
expect real-0071-single "NONE value-absent" ok "$PLAN" single

echo "cases: $n fixtures, failures: $failures"
[ "$failures" = 0 ]
