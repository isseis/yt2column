#!/usr/bin/env bash
# Static check for task 0120.
#   (1) no harness-specific tool call remains in the command definitions
#       and shared procedures (adapters and chief.md excluded);
#   (2) the only shell scripts under .claude/commands/ are the three
#       record-reading scripts.
# Usage: check_harness_tools.sh   (run from anywhere inside the repository)
set -u
root=$(git rev-parse --show-toplevel) || exit 2
cd "$root/.claude/commands" || exit 2
status=0

# Names that cannot be read as ordinary English: any word-bounded use is a hit.
unique='AskUserQuestion|WebFetch|WebSearch|TaskStop|ToolSearch|PushNotification|ask_question|invoke_subagent|swarm_wait|read_board|run_command|read_file|write_file|edit_file|manage_task|search_web'
# Names that are also English words: a hit only as a code span, as a call
# (Name(), or as "the Name tool".
plain='Bash|Write|Read|Edit|Agent|swarm|schedule'

while IFS= read -r f; do
  case "$f" in
    ./_lib/harness*.md | ./chief.md) continue ;;
  esac
  hits=$(grep -nE "\b($unique)\b|\`($plain)\`|\b($plain)\(" "$f") || true
  # "the Name tool" may wrap across a line break, so it is read on the joined text.
  wrapped=$(tr -s '[:space:]' ' ' <"$f" | grep -oE "\b($plain) tool\b") || true
  [ -z "$wrapped" ] || hits="$hits"$'\n'"joined text: $wrapped"
  if [ "$f" = ./_context.md ] && [ -n "$hits" ]; then
    # Only _context.md's harness tables and their Launch description name the
    # tool launch kinds and the Agent column.
    hits=$(printf '%s\n' "$hits" |
      grep -vE '^[0-9]+:\| *`(Claude Code|Antigravity|OpenCode)` *\|' |
      grep -vE '^[0-9]+:- \*\*Launch\*\*: ' |
      grep -vE '^[0-9]+:\| *Row *\| *Command *\| *Agent *\|') || true
  fi
  if [ -n "$hits" ]; then
    echo "FAIL (1): harness tool name in $f:"
    echo "$hits"
    status=1
  fi
done < <(find . -type f -name '*.md' | sort)

scripts=$(find . -type f -name '*.sh' | sort | tr '\n' ' ')
want='./_lib/adr-correction-candidates.sh ./_lib/high-review-decision.sh ./_lib/review-records.sh '
if [ "$scripts" != "$want" ]; then
  echo "FAIL (2): shell scripts = $scripts; want $want"
  status=1
fi

[ "$status" -eq 0 ] && echo "OK: no harness tool call outside the adapters; scripts = $scripts"
exit "$status"
