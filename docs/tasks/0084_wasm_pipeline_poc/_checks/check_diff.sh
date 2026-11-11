#!/usr/bin/env bash
# One-off check of AC-04, AC-05 and AC-06 of task 0084, comparing the merge-base
# with origin/main to HEAD. Run from the repository root after `git fetch origin main`.
set -euo pipefail
export LC_ALL=C

base=$(git merge-base origin/main HEAD)
fail() { echo "FAIL: $*" >&2; exit 1; }

# norm drops the package, build-constraint and import lines and the package
# qualifiers a move adds, so what is left is the declarations themselves.
norm() {
	awk '/^import \(/{i=1;next} i&&/^\)/{i=0;next} i{next} /^package |^\/\/go:build |^import "/{next} {print}' |
		perl -pe 's/\b(transcript|ytdlpsource)\.//g'
}
show_old() { git show "$base:$1" | norm; }
show_new() { git show "HEAD:$1" | norm; }
squeeze() { sed -E 's/^[[:space:]]+//; s/[[:space:]]+/ /g' | grep -v '^$' | sort; }

# 1. The set of changed files is the designed one.
allowed='^(internal/(transcript|ytdlpsource)/|internal/writer/template(_open|_open_js)?\.go$|internal/(job|cachelock|config)/|cmd/yt2column/(run_test|test_helpers_integration)\.go$|docs/tasks/0018_article_quality/_material_check/main\.go$|Makefile$|\.github/workflows/ci\.yml$|docs/dev/conventions/package_reference\.md$|CLAUDE\.md$|docs/dev/guides/development\.md$|\.claude/commands/(runplan|mkplan)\.md$|docs/dev/(project_overview|cache_consistency)\.md$|docs/dev/adr/(0019|0033)-|docs/tasks/0084_wasm_pipeline_poc/)'
extra=$(git diff --name-only --no-renames "$base" HEAD | grep -Ev "$allowed" || true)
[ -z "$extra" ] || fail "files outside the design changed: $extra"
echo "OK files: every changed file is one the design names"

# 2. Moved files differ from their originals only in the designed places.
moved=0
while read -r status old new; do
	[ "${status:0:1}" = R ] || continue
	moved=$((moved + 1))
	residual=$(diff <(show_old "$old") <(show_new "$new") | grep '^[<>]' || true)
	case "$new" in
	internal/ytdlpsource/cache.go) want=$(cat <<'X'
< 	if !ok || !videoIDPattern.MatchString(id) {
> 	if !ok {
> 		return "", false
> 	}
> 	// NormalizedVideoURL reports false exactly when id does not match the
> 	// video ID pattern, so this is the same check without the private regexp.
> 	if _, valid := NormalizedVideoURL(id); !valid {
X
		) ;;
	internal/ytdlpsource/ytdlp.go) want=$(cat <<'X'
< 		transcript, hit, err := s.readCached(id, normalizedURL)
> 		cached, hit, err := s.readCached(id, normalizedURL)
< 			return transcript, nil
> 			return cached, nil
X
		) ;;
	internal/ytdlpsource/json3_test.go)
		# Only the sentinel guard test and its helper are gone; sentinel_test.go holds them.
		removed=$(grep '^<' <<<"$residual" | sed 's/^< //' | squeeze)
		kept=$(git show HEAD:internal/transcript/sentinel_test.go | norm |
			awk '/^\/\/ parsePackageFile parses/{exit} {print}' | squeeze)
		[ -z "$(grep '^>' <<<"$residual" || true)" ] || fail "$new gained lines"
		[ "$removed" = "$kept" ] || fail "$new: removed lines differ from sentinel_test.go"
		continue ;;
	*) want= ;;
	esac
	[ "$residual" = "$want" ] || fail "$new differs from $old beyond the designed rewrites:
$residual"
done < <(git diff --name-status -M "$base" HEAD -- internal)
[ "$moved" -eq 12 ] || fail "expected 12 moved files, saw $moved"
echo "OK moved files: 12 files differ only in the designed places"

# 3. test_helpers.go: the two halves together equal the original plus the designed edits.
want=$(cat <<'X'
> // fixtures, relative to this package directory.
> // readTestdataFile returns the bytes of a committed testdata fixture.
> // testdataCopiedTranscripts is the glob of the committed copied-transcript
> const testdataCopiedTranscripts = "../../testdata/*.transcript_copy.txt"
> data, err := os.ReadFile(path) //nolint:gosec // the path names a committed testdata fixture
> func readTestdataFile(t *testing.T, path string) []byte {
< if !videoIDPattern.MatchString(id) {
> if _, ok := NormalizedVideoURL(id); !ok {
> if err != nil {
> return data
> t.Fatalf("read %s: %v", path, err)
< testdataCopiedTranscripts = "../../testdata/*.transcript_copy.txt"
> t.Helper()
> }
> }
X
)
got=$(diff <(show_old internal/transcript/test_helpers.go | squeeze) \
	<( { show_new internal/transcript/test_helpers.go; show_new internal/ytdlpsource/test_helpers.go; } | squeeze) |
	grep '^[<>]' || true)
[ "$got" = "$want" ] || fail "test_helpers.go split differs:
$got"
echo "OK test_helpers.go: split differs only by the duplicated readTestdataFile and testVideoID"

# 4. Callers changed only by import lines and qualifiers; envaccess_test.go also by the path string.
for f in cmd/yt2column/run_test.go cmd/yt2column/test_helpers_integration.go internal/cachelock/cachelock_test.go \
	internal/job/job.go internal/job/job_test.go docs/tasks/0018_article_quality/_material_check/main.go; do
	diff <(show_old "$f") <(show_new "$f") >/dev/null || fail "$f changed beyond imports and qualifiers"
done
# -w: gofmt realigns the map literal around the longer path.
diff -w <(show_old internal/config/envaccess_test.go | sed 's#internal/transcript/ytdlp\.go#internal/ytdlpsource/ytdlp.go#g') \
	<(show_new internal/config/envaccess_test.go) >/dev/null || fail "envaccess_test.go changed beyond the path string"
echo "OK callers: only import lines, qualifiers, and the envaccess path string changed"

# 5. writer: what left template.go is exactly the body of template_open.go.
old_t=$(git show "$base:internal/writer/template.go" | norm | squeeze)
new_t=$(git show HEAD:internal/writer/template.go | norm | squeeze)
open_t=$(show_new internal/writer/template_open.go | squeeze)
gone=$(comm -23 <(echo "$old_t") <(echo "$new_t"))
[ -z "$(comm -13 <(echo "$old_t") <(echo "$new_t"))" ] || fail "template.go gained lines"
[ "$gone" = "$open_t" ] || fail "template_open.go differs from what left template.go"
echo "OK writer: template_open.go holds exactly the lines that left template.go"

# 6. Makefile and ci.yml.
mk=$(git diff -U0 "$base" HEAD -- Makefile | grep '^[+-]' | grep -v '^+++\|^---')
mk_want=$(cat <<'X'
-.PHONY: all build clean test test-ci test-parse-norace readme-sync-check test-integration test-integration-deepseek test-integration-claude test-integration-openai test-integration-cli test-integration-slack lint fmt fmt-all deadcode tidy install-mergepr install-fixprguard install-codexreview ext-install ext-typecheck ext-lint ext-fmt-check ext-fmt ext-test ext-build ext-check coverage-compare ext-coverage-compare
+.PHONY: all build clean test test-ci test-parse-norace readme-sync-check test-integration test-integration-deepseek test-integration-claude test-integration-openai test-integration-cli test-integration-slack lint wasm-check fmt fmt-all deadcode tidy install-mergepr install-fixprguard install-codexreview ext-install ext-typecheck ext-lint ext-fmt-check ext-fmt ext-test ext-build ext-check coverage-compare ext-coverage-compare
-	$(GOTEST) -tags integration -count=1 -timeout $(INTEGRATION_TIMEOUT) -v ./internal/transcript
+	$(GOTEST) -tags integration -count=1 -timeout $(INTEGRATION_TIMEOUT) -v ./internal/ytdlpsource
-lint:
+lint: wasm-check
+# The packages the WebAssembly core is built from (the transcript types and
+# CopiedSource, the article writer, the Claude adapter, and the pricing table)
+# must keep compiling for js/wasm. This checks only that they build; behavior
+# that fails at run time in a browser (file system access) is not found here.
+# The two vet runs also compile the js-only files, which `make test` skips.
+WASM_PACKAGES = ./internal/transcript ./internal/writer ./internal/llm/claude ./internal/pricing
+WASM_VET_PACKAGES = ./internal/writer ./internal/transcript
+
+wasm-check:
+	GOOS=js GOARCH=wasm $(GOCMD) build $(WASM_PACKAGES)
+	GOOS=js GOARCH=wasm $(GOCMD) vet $(WASM_VET_PACKAGES)
+
X
)
[ "$mk" = "$mk_want" ] || fail "Makefile diff differs from the design:
$mk"
ci=$(git diff -U0 "$base" HEAD -- .github/workflows/ci.yml | grep '^[+-]' | grep -v '^+++\|^---')
[ -z "$(grep '^-' <<<"$ci" || true)" ] || fail "ci.yml lost lines"
grep -qx '+        run: make wasm-check' <<<"$ci" || fail "ci.yml step does not run make wasm-check"
[ "$(grep -c '^+      - name:' <<<"$ci")" -eq 1 ] || fail "ci.yml gained other than one step"
echo "OK Makefile and ci.yml: only the designed lines changed"

# 7. AC-06: no dependency change.
[ -z "$(git diff --name-only "$base" HEAD -- .golangci.yml go.mod go.sum)" ] || fail ".golangci.yml, go.mod, or go.sum changed"
echo "OK AC-06: .golangci.yml, go.mod, go.sum unchanged"
