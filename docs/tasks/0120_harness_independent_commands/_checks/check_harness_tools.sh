#!/usr/bin/env bash
# AC-02 / AC-41(4) static check for task 0120.
#   (1) no harness-specific tool call remains in the command definitions
#       and shared procedures (adapters and chief.md excluded);
#   (2) the only shell scripts under .claude/commands/ are the three
#       record-reading scripts.
# Usage: check_harness_tools.sh   (run from anywhere inside the repository)
set -u
root=$(git rev-parse --show-toplevel) || exit 2
cd "$root/.claude/commands" || exit 2
status=0

# Tool names that must not be called directly. Plain English verbs
# (read, write, edit) are matched only in their snake_case tool forms.
tools='Bash|Write|Read|Edit|WebFetch|WebSearch|AskUserQuestion|Agent|TaskStop|ToolSearch|PushNotification|ask_question|invoke_subagent|swarm|swarm_wait|read_board|run_command|read_file|write_file|edit_file|schedule|manage_task|search_web'

while IFS= read -r f; do
  case "$f" in
    ./_lib/harness*.md | ./chief.md) continue ;;
  esac
  # A tool name counts only when written as a code span (`Name`) or as a
  # call (Name( ), which is how a procedure names a tool it invokes.
  hits=$(grep -nE "\`($tools)\`|\b($tools)\(" "$f" |
    grep -vE '^[0-9]+:\| *Row *\| *Command *\| *Agent *\|' |
    grep -vE '`(agent|swarm|none)`' |
    grep -vE '^[0-9]+: *```bash') || true
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
