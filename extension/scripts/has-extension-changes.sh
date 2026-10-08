#!/usr/bin/env bash
# Reads changed file paths as a NUL-delimited list on standard input and prints
# "true" when CI must run the extension job, "false" otherwise. NUL delimiters
# keep paths that contain control characters (tabs, newlines) intact, so the
# patterns' ^ anchors still match them. The job runs for the extension itself,
# the Makefile that drives it, any workflow, and go.mod (whose ignore directive
# keeps extension/node_modules out of ./...).
set -euo pipefail

# grep reads all of its input (no -q), so a writer piping into this script
# never gets SIGPIPE. Exit status 1 is "no match"; anything above is an error
# and must not read as "false".
status=0
grep -zE \
  -e '^extension/' \
  -e '^Makefile$' \
  -e '^\.github/workflows/' \
  -e '^go\.mod$' \
  >/dev/null || status=$?

case "$status" in
  0) echo true ;;
  1) echo false ;;
  *) exit "$status" ;;
esac
