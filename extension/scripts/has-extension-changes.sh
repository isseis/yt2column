#!/usr/bin/env bash
# Usage: has-extension-changes.sh <dependency-directory-file>
#
# Reads changed file paths as a NUL-delimited list on standard input and prints
# "true" when CI must run the extension job, "false" otherwise. NUL delimiters
# keep paths that contain control characters (tabs, newlines) intact, so an
# embedded newline cannot split a path into a fragment that matches. The job
# runs for the extension itself, the Makefile that drives it, any workflow,
# go.mod (whose ignore directive keeps extension/node_modules out of ./..., and
# whose go line chooses the toolchain the extension build uses) and go.sum, and
# the files the extension tests read: README.md and README.en.md (their
# browser extension sections, and the minimum Chrome version anywhere in
# them), docs/dev/guides/development.md (its
# rejection-reason table), and internal/server/response.go (which that table
# mirrors).
#
# It also runs for any file under a directory listed in the dependency
# directory file, one directory per line ending in "/", as
# wasm-dep-dirs.sh prints them: the Go packages the article generation modules
# compile, with their embedded files and subdirectories. The trailing "/"
# keeps internal/llm/ from matching internal/llmx/. The script decides from
# the paths alone and never runs go.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: has-extension-changes.sh <dependency-directory-file>" >&2
  exit 2
fi

dep_dirs=()
while IFS= read -r dir || [ -n "$dir" ]; do
  case "$dir" in
    "") ;;
    */) dep_dirs+=("$dir") ;;
    *)
      echo "has-extension-changes: directory \"$dir\" does not end in /" >&2
      exit 2
      ;;
  esac
done <"$1"

# Reads every record, even after a match, so the writer (git diff -z) never
# gets SIGPIPE.
matched=false
while IFS= read -r -d '' path; do
  case "$path" in
    extension/* | Makefile | .github/workflows/* | go.mod | go.sum | README.md | README.en.md | docs/dev/guides/development.md | internal/server/response.go)
      matched=true
      ;;
    *)
      for dir in ${dep_dirs[@]+"${dep_dirs[@]}"}; do
        if [[ "$path" == "$dir"* ]]; then
          matched=true
        fi
      done
      ;;
  esac
done

if [ "$matched" = true ]; then
  echo true
else
  echo false
fi
