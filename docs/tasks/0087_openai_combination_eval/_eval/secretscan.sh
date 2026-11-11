#!/bin/zsh
# Task 0087's secret scan (design section 7.4): report whether the value of any API
# key or Webhook URL set in the environment, or a string with the "sk-" key
# prefix, appears in the given paths. It never prints a value: each variable
# is reported as present or absent, and a prefix match by file name only.
#
# Usage: secretscan.sh [path ...]
#   Without paths it scans _eval/ and 04_evaluation.md of task 0087.
# It exits 0 when nothing is found, 1 when something is, and 2 when a path
# does not exist or no variable is set (there would be nothing to compare).

emulate -L zsh
set -u
setopt pipefail

local here=${0:A:h}
local -a targets
if (( $# )); then
  targets=("$@")
else
  targets=($here ${here:h}/04_evaluation.md)
fi

local t
for t in $targets; do
  [[ -e $t ]] || { print -r -- "secretscan: no such path: $t" >&2; exit 2 }
done

local -a vars=(ANTHROPIC_API_KEY OPENAI_API_KEY DEEPSEEK_API_KEY SLACK_WEBHOOK_URL)
local v val set_any=0 found=0
for v in $vars; do
  val=${(P)v-}
  [[ -z $val ]] && continue
  set_any=1
  # The value goes to grep through a pattern file, never on a command line.
  if grep -rqF -f <(print -r -- $val) -- $targets; then
    print -r -- "secretscan: the value of $v is present"
    found=1
  else
    print -r -- "secretscan: the value of $v is absent"
  fi
done
if (( ! set_any )); then
  print -r -- "secretscan: none of ${(j:, :)vars} is set; nothing to compare" >&2
  exit 2
fi
local hits
hits=$(grep -rlE -- 'sk-[A-Za-z0-9_-]{16,}' $targets)
if [[ -n $hits ]]; then
  print -r -- "secretscan: an sk- key prefix is present in:"
  print -r -- $hits
  found=1
else
  print -r -- "secretscan: no sk- key prefix"
fi
exit $found
