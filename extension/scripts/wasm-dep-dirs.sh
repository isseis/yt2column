#!/usr/bin/env bash
# Prints the directories of this module's packages that the two article
# generation modules (cmd/yt2column-wasm and cmd/yt2column-wasm-dev) depend on
# in their js/wasm build, one per line, relative to the repository root and
# ending in "/" (for example "internal/llm/"). Standard library packages and
# other modules are left out: only the main module's packages are printed.
# CI's check-changes job gives the list to has-extension-changes.sh, so a
# change to a Go package the extension's build compiles runs the extension job.
#
# Contract: it exits non-zero, printing nothing on standard output, when go
# list fails, prints no directory, or prints a directory outside the
# repository. go list runs without -e, which would report a broken package as
# a success. CI treats a non-zero exit as "run the extension job".
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd -P)
cd "$root"

dirs=$(GOOS=js GOARCH=wasm go list -deps \
  -f '{{if and .Module .Module.Main}}{{.Dir}}{{end}}' \
  ./cmd/yt2column-wasm ./cmd/yt2column-wasm-dev)

out=""
while IFS= read -r dir; do
  [ -z "$dir" ] && continue
  case "$dir" in
    "$root"/*) out+="${dir#"$root"/}/"$'\n' ;;
    *)
      echo "wasm-dep-dirs: $dir is outside $root" >&2
      exit 1
      ;;
  esac
done <<<"$dirs"

if [ -z "$out" ]; then
  echo "wasm-dep-dirs: go list printed no directory" >&2
  exit 1
fi
printf '%s' "$out"
