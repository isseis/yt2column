#!/usr/bin/env bash
# Reads changed file paths as a NUL-delimited list on standard input and prints
# "true" when CI must run the extension job, "false" otherwise. NUL delimiters
# keep paths that contain control characters (tabs, newlines) intact, so an
# embedded newline cannot split a path into a fragment that matches. The job
# runs for the extension itself, the Makefile that drives it, any workflow, and
# go.mod (whose ignore directive keeps extension/node_modules out of ./...).
set -euo pipefail

# Reads every record, even after a match, so the writer (git diff -z) never
# gets SIGPIPE.
matched=false
while IFS= read -r -d '' path; do
  case "$path" in
    extension/* | Makefile | .github/workflows/* | go.mod) matched=true ;;
  esac
done

if [ "$matched" = true ]; then
  echo true
else
  echo false
fi
