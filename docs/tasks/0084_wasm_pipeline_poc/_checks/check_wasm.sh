#!/usr/bin/env bash
# One-off check of AC-01 and AC-02 of task 0084. Run from the repository root.
set -euo pipefail

export GOOS=js GOARCH=wasm
core=(./internal/transcript ./internal/writer ./internal/llm/claude ./internal/pricing)
module=github.com/isseis/yt2column

# AC-01: the four packages build, and the two with js-only files pass vet.
go build "${core[@]}"
go vet ./internal/writer ./internal/transcript
echo "OK AC-01: build of 4 packages and vet of writer and transcript for js/wasm"

# AC-02: no dependency is the yt-dlp package, and no repository package in the
# dependency closure imports os/exec.
deps=$(go list -deps "${core[@]}")
if grep -qx "$module/internal/ytdlpsource" <<<"$deps"; then
	echo "FAIL AC-02: internal/ytdlpsource is a dependency" >&2
	exit 1
fi
execusers=$(go list -deps -f '{{.ImportPath}} {{join .Imports " "}}' "${core[@]}" |
	awk -v m="$module/" 'index($1, m) == 1 { for (i = 2; i <= NF; i++) if ($i == "os/exec") print $1 }')
if [ -n "$execusers" ]; then
	echo "FAIL AC-02: repository packages import os/exec: $execusers" >&2
	exit 1
fi
echo "OK AC-02: no ytdlpsource dependency, no repository package imports os/exec"
