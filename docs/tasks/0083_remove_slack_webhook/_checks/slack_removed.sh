#!/bin/sh
# Checks that a commit carries no part of the Slack Webhook integration test
# and that the server's configuration template names no Webhook key:
#
#   1. the Makefile does not name the test-integration-slack target;
#   2. cmd/, internal/, the Makefile, and .github/ do not name
#      YT2COLUMN_TEST_SLACK_WEBHOOK_URL or YT2COLUMN_SLACK_INTEGRATION;
#   3. the file yt2column-server config init creates has no line that holds
#      SLACK_WEBHOOK_URL.
#
#	slack_removed.sh <commit>
#
# Exits 1 when a check fails, 2 when the argument is wrong or a step cannot
# run (git cannot read the commit, the build or config init fails), and 0
# otherwise. Checks 1 and 2 read blobs with git; check 3 builds the server
# from a git archive of the commit in a temporary directory and runs config
# init there with HOME set to an empty temporary directory, so the working
# tree and the real configuration file are never touched. Run it from the top
# level of the work tree. make test and CI do not run it.

LC_ALL=C
export LC_ALL

if [ $# -ne 1 ]; then
	echo "usage: slack_removed.sh <commit>" >&2
	exit 2
fi

commit=$1
if ! git rev-parse --verify --quiet "$commit^{commit}" >/dev/null; then
	echo "slack_removed: $commit is not a commit" >&2
	exit 2
fi

work=$(mktemp -d) || exit 2
trap 'rm -rf "$work"' EXIT

echo "commit: $commit"
status=0

# Check 1: the target's name appears nowhere in the Makefile, so neither the
# target nor a .PHONY entry for it remains.
if ! git show "$commit:Makefile" >"$work/Makefile" 2>/dev/null; then
	echo "slack_removed: cannot read the Makefile at $commit" >&2
	exit 2
fi
if grep -n 'test-integration-slack' "$work/Makefile" >"$work/target.txt"; then
	echo "NG   the Makefile names test-integration-slack:"
	sed 's/^/     /' "$work/target.txt"
	status=1
else
	echo "OK   the Makefile does not name test-integration-slack"
fi

# Check 2: git grep exits 0 when it finds a line, 1 when it finds none, and
# above 1 when it fails.
git grep -n -e YT2COLUMN_TEST_SLACK_WEBHOOK_URL -e YT2COLUMN_SLACK_INTEGRATION \
	"$commit" -- cmd internal Makefile .github >"$work/vars.txt" 2>"$work/vars.err"
case $? in
0)
	echo "NG   the opt-in or test Webhook variable is named:"
	sed 's/^/     /' "$work/vars.txt"
	status=1
	;;
1)
	echo "OK   cmd/, internal/, Makefile, and .github/ name neither YT2COLUMN_TEST_SLACK_WEBHOOK_URL nor YT2COLUMN_SLACK_INTEGRATION"
	;;
*)
	cat "$work/vars.err" >&2
	echo "slack_removed: git grep failed at $commit" >&2
	exit 2
	;;
esac

# Check 3: the template config init writes.
mkdir "$work/src" "$work/home" || exit 2
if ! git archive "$commit" | tar -x -C "$work/src"; then
	echo "slack_removed: cannot extract $commit" >&2
	exit 2
fi
if ! (cd "$work/src" && go build -o "$work/yt2column-server" ./cmd/yt2column-server) >"$work/build.log" 2>&1; then
	cat "$work/build.log" >&2
	echo "slack_removed: cannot build yt2column-server at $commit" >&2
	exit 2
fi
if ! env -i PATH="$PATH" HOME="$work/home" "$work/yt2column-server" config init >"$work/init.log" 2>&1; then
	cat "$work/init.log" >&2
	echo "slack_removed: config init failed" >&2
	exit 2
fi
template=$(find "$work/home" -type f -name config.env)
if [ -z "$template" ] || [ "$(printf '%s\n' "$template" | wc -l | tr -d ' ')" -ne 1 ]; then
	echo "slack_removed: config init did not create exactly one config.env" >&2
	exit 2
fi
lines=$(wc -l <"$template" | tr -d ' ')
if grep -n 'SLACK_WEBHOOK_URL' "$template" >"$work/template.txt"; then
	echo "NG   the template config init writes holds SLACK_WEBHOOK_URL:"
	sed 's/^/     /' "$work/template.txt"
	status=1
else
	echo "OK   the template config init writes ($lines lines) holds no SLACK_WEBHOOK_URL"
fi

exit "$status"
