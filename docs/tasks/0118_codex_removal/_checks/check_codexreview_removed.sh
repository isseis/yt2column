#!/usr/bin/env bash
# Static verification that codexreview has been removed from the repository.
# Usage: bash docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh [root]
set -euo pipefail

ROOT="${1:-$(cd "$(dirname "$0")/../../../.." && pwd)}"
fail=0

check_no_path() {
	local rel="$1"
	if [ -e "$ROOT/$rel" ] || [ -L "$ROOT/$rel" ]; then
		printf 'FAIL: %s still exists\n' "$rel" >&2
		fail=1
	fi
}

check_absent() {
	local desc="$1"
	shift
	local out rc
	set +e
	out=$("$@" 2>&1)
	rc=$?
	set -e
	if [ "$rc" -eq 1 ]; then
		return 0
	fi
	if [ "$rc" -eq 0 ]; then
		printf 'FAIL: %s\n%s\n' "$desc" "$out" >&2
	else
		printf 'FAIL: %s (grep exited %d)\n%s\n' "$desc" "$rc" "$out" >&2
	fi
	fail=1
}

if [ ! -f "$ROOT/Makefile" ] || [ ! -f "$ROOT/.claude/settings.json" ] || [ ! -d "$ROOT/.claude/commands" ] || [ ! -d "$ROOT/cmd" ] || [ ! -d "$ROOT/internal" ] || [ ! -d "$ROOT/prompts" ]; then
	printf 'FAIL: %s does not look like the repository root\n' "$ROOT" >&2
	exit 1
fi

check_no_path "cmd/codexreview"
check_no_path "internal/codexreview"
check_absent "Makefile still mentions install-codexreview" grep -nF -- 'install-codexreview' "$ROOT/Makefile"
check_absent ".claude/settings.json still mentions codexreview" grep -inF -- 'codexreview' "$ROOT/.claude/settings.json"
check_absent ".claude/commands/ still mentions codexreview" grep -rniF -- 'codexreview' "$ROOT/.claude/commands"
check_absent "Go sources under cmd/, internal/, or prompts/ still mention codexreview" grep -rniF --include='*.go' -- 'codexreview' "$ROOT/cmd" "$ROOT/internal" "$ROOT/prompts"

if [ "$fail" -ne 0 ]; then
	exit 1
fi
printf 'PASS: codexreview removed\n'
