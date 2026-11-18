#!/usr/bin/env bash
# Checks the state the removal of the localhost server, the Native Messaging
# host, and the development server leaves (AC-30, AC-31, AC-33):
#
#   dirs      the removed package directories do not exist
#   kept      internal/job, internal/privatefile, internal/cachelock,
#             internal/signalrun, and internal/redact still exist
#   notag     no Go file of internal/config carries a !js build tag
#   wasmdeps  GOOS=js GOARCH=wasm go list -deps ./internal/config does not
#             list internal/privatefile
#   builddev  make -n build-dev fails (the target is gone)
#   config    Makefile, .golangci.yml, .github/workflows/ci.yml, and
#             extension/scripts/has-extension-changes.sh name no removed
#             package
#   docgo     every package a go command example of the tracked documents
#             names is found by go list
#   docmake   every target a make command example of the tracked documents
#             names passes make -n
#
# The tracked documents are README.md, README.en.md, CLAUDE.md, AGENTS.md,
# the files under docs/dev/ other than the ADRs, and the files under
# .claude/commands/. A command example is a fenced code block line or an
# inline code span that holds `go <subcommand>` or `make`.
#
# Run it from anywhere inside the repository, on a committed tree. It runs go
# and make (make -n only) and touches no network.
#
# Output and exit codes:
#   0  every check passed ("ok <name>" per check, then "ok: all checks passed")
#   1  a check failed ("FAIL <name>: ..." lines on stdout)
#   2  the repository is unusable
set -euo pipefail

root=$(git rev-parse --show-toplevel) || exit 2
cd "$root"

# failed is set once any check fails; bad is reset by each check.
failed=0
bad=0
fail() {
	echo "FAIL $1: $2"
	failed=1
	bad=1
}
# done_check prints "ok <name>" when the check that just ran found nothing.
done_check() {
	[ "$bad" = 1 ] || echo "ok $1"
	bad=0
}

removed_pkgs=(
	cmd/yt2column-server
	cmd/yt2column-nmhost
	cmd/yt2column-devserver
	internal/server
	internal/articlecache
	internal/serversock
	internal/nmhost
)
kept_pkgs=(
	internal/job
	internal/privatefile
	internal/cachelock
	internal/signalrun
	internal/redact
)

# dirs
for p in "${removed_pkgs[@]}"; do
	if [ -e "$p" ]; then
		fail dirs "$p exists"
	fi
done
done_check dirs

# kept
for p in "${kept_pkgs[@]}"; do
	if ! ls "$p"/*.go >/dev/null 2>&1; then
		fail kept "$p has no Go file"
	fi
done
done_check kept

# notag
while IFS= read -r f; do
	if grep -qE '^//go:build.*!js' "$f"; then
		fail notag "$f carries a !js build tag"
	fi
done < <(git ls-files 'internal/config/*.go')
done_check notag

# wasmdeps
if ! deps=$(GOOS=js GOARCH=wasm go list -deps ./internal/config); then
	fail wasmdeps "go list -deps failed"
elif grep -qx 'github.com/isseis/yt2column/internal/privatefile' <<<"$deps"; then
	fail wasmdeps "internal/config depends on internal/privatefile in a js/wasm build"
fi
done_check wasmdeps

# builddev
if make -n build-dev >/dev/null 2>&1; then
	fail builddev "make -n build-dev succeeds"
fi
done_check builddev

# config
pattern=$(printf '%s\n' "${removed_pkgs[@]}" | paste -sd '|' -)
for f in Makefile .golangci.yml .github/workflows/ci.yml extension/scripts/has-extension-changes.sh; do
	if hits=$(grep -nE "($pattern)([^a-z0-9-]|$)" "$f"); then
		fail config "$f names a removed package: $hits"
	fi
done
done_check config

# The command examples: fenced code block lines and inline code spans.
examples=$(
	git ls-files README.md README.en.md CLAUDE.md AGENTS.md '.claude/commands/*' 'docs/dev/*' |
		grep -v '^docs/dev/adr/' |
		while IFS= read -r f; do
			awk '/^[[:space:]]*```/ { inblock = !inblock; next } inblock { print }' "$f"
			grep -oE '`[^`]+`' "$f" | sed 's/^`//; s/`$//' || true
		done
)

# docgo: the ./ and module paths after go <subcommand>, up to a shell
# operator or a comment. placeholder_pkgs are the example paths a document
# writes for "some package"; they name no package on purpose.
placeholder_pkgs=(./internal/specific/package)
go_pkgs=$(
	grep -oE '(^|[[:space:](])go (build|test|list|vet|install|run|generate)( [^;&|#`)]*)?' <<<"$examples" |
		tr ' ' '\n' |
		grep -E '^(\./|github\.com/isseis/yt2column)' |
		grep -vxF -f <(printf '%s\n' "${placeholder_pkgs[@]}") |
		sort -u || true
)
if [ -z "$go_pkgs" ]; then
	fail docgo "found no go command example"
fi
while IFS= read -r p; do
	[ -n "$p" ] || continue
	if ! go list -tags test,integration "$p" >/dev/null 2>&1; then
		fail docgo "go list finds no package for $p"
	fi
done <<<"$go_pkgs"
done_check docgo

# docmake: the target words right after make and its options, up to the
# first other word (a VAR=value argument, a shell operator, or a comment).
make_targets=$(
	grep -oE '(^|[[:space:](])make( [^;&|#`)]*)?' <<<"$examples" |
		awk '{
			for (i = 1; i <= NF; i++) {
				if ($i == "make") { started = 1; continue }
				if (!started) continue
				if ($i ~ /^-/) continue
				if ($i ~ /^[a-z][a-z0-9-]*$/) { print $i; continue }
				break
			}
			started = 0
		}' |
		sort -u || true
)
if [ -z "$make_targets" ]; then
	fail docmake "found no make command example"
fi
while IFS= read -r t; do
	[ -n "$t" ] || continue
	if ! make -n "$t" >/dev/null 2>&1; then
		fail docmake "make -n $t fails"
	fi
done <<<"$make_targets"
done_check docmake

if [ "$failed" != 0 ]; then
	exit 1
fi
echo "ok: all checks passed"
