GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
# Pin golangci-lint to the exact version CI enforces (.github/workflows/ci.yml
# and .pre-commit-config.yaml). Using `go run @version` ignores whatever
# golangci-lint is on PATH, so local `make lint` always matches CI. Bump all
# three pins together.
GOLANGCI_VERSION?=v2.13.2
GOLINT=$(GOCMD) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run --build-tags test,integration
GOFUMPTCMD=gofumpt

BINARY=build/yt2column
MAIN_PKG=./cmd/yt2column

# Common gofumpt check and error message
define check_gofumpt
	@if ! command -v $(GOFUMPTCMD) >/dev/null 2>&1; then \
		echo "Error: $(GOFUMPTCMD) is required but not found in PATH"; \
		echo "Please install gofumpt: go install mvdan.cc/gofumpt@latest"; \
		exit 1; \
	fi
endef

# Format files from a list and display what was formatted
# Usage: $(call format_files_from_list,file_list_command)
define format_files_from_list
	TEMP_FILE=$$(mktemp); \
	trap "rm -f \"$$TEMP_FILE\"" EXIT; \
	$(1) | while IFS= read -r file; do \
		if [ -f "$$file" ] && $(GOFUMPTCMD) -d "$$file" | grep -q .; then \
			printf '%s\n' "$$file"; \
		fi; \
	done > "$$TEMP_FILE"; \
	if [ -s "$$TEMP_FILE" ]; then \
		echo "Formatting files:"; \
		while IFS= read -r file; do \
			printf '  %s\n' "$$file"; \
		done < "$$TEMP_FILE"; \
		while IFS= read -r file; do \
			if ! $(GOFUMPTCMD) -w "$$file"; then \
				echo "Error: $(GOFUMPTCMD) failed on $$file"; \
				exit 1; \
			fi; \
		done < "$$TEMP_FILE"; \
	fi
endef

.PHONY: all build clean test test-ci test-integration test-integration-deepseek test-integration-cli lint fmt fmt-all deadcode tidy install-mergepr ext-install ext-typecheck ext-lint ext-fmt-check ext-fmt ext-test ext-build ext-check

all: build

build:
	@mkdir -p build
	$(GOBUILD) -trimpath -ldflags "-s -w" -o $(BINARY) $(MAIN_PKG)

# Install the /mergepr developer tool into $(go env GOPATH)/bin. Run from an
# up-to-date main checkout (docs/dev/developer_guide/mergepr_guide.md).
install-mergepr:
	$(GOCMD) install ./cmd/mergepr

clean:
	$(GOCLEAN)
	rm -rf build
	rm -f coverage.out coverage.html

# Unit tests. Never reach yt-dlp, an LLM API, or a Webhook (fakes only).
test:
	$(GOTEST) -tags test -race -v ./...

# CI variant: same tests plus a coverage profile.
test-ci:
	$(GOTEST) -tags test -race -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -func=coverage.out | tail -1

# Integration test. Runs the real yt-dlp against the network, so it is kept
# out of `make test` by the `integration` build tag. The video defaults below
# can be overridden, e.g.
#   make test-integration YT2COLUMN_TEST_VIDEO_URL=... YT2COLUMN_TEST_VIDEO_ID=...
# An empty YT2COLUMN_TEST_VIDEO_ID skips the video ID check.
YT2COLUMN_TEST_VIDEO_URL ?= https://www.youtube.com/watch?v=EQCUZyB4DqE
YT2COLUMN_TEST_VIDEO_ID ?= EQCUZyB4DqE
# Exported so the test process reads them from its environment; the recipe
# never splices the values into shell text.
export YT2COLUMN_TEST_VIDEO_URL YT2COLUMN_TEST_VIDEO_ID
INTEGRATION_TIMEOUT ?= 10m

test-integration:
	@printf 'test-integration: uses the real yt-dlp and the network (video: %s)\n' "$$YT2COLUMN_TEST_VIDEO_URL"
	$(GOTEST) -tags integration -count=1 -timeout $(INTEGRATION_TIMEOUT) -v ./internal/transcript

# DeepSeek integration test. Calls the real DeepSeek API, which incurs
# charges, so it is kept out of `make test` by the `integration` build tag and
# skips unless the opt-in variable below is set. It reads its API key from
# YT2COLUMN_TEST_DEEPSEEK_API_KEY, never from DEEPSEEK_API_KEY. The model
# defaults to deepseek-flash only when YT2COLUMN_MODEL is undefined; an empty
# value is passed through, and the test fails on it.
YT2COLUMN_MODEL ?= deepseek-flash
# Two Generate calls of at most 15 minutes each, plus margin (see
# internal/llm/deepseek/integration_test.go).
DEEPSEEK_INTEGRATION_TIMEOUT ?= 40m

# Exported to this target's recipe only; the recipe never splices the values
# into shell text.
test-integration-deepseek: export YT2COLUMN_MODEL := $(YT2COLUMN_MODEL)
test-integration-deepseek: export YT2COLUMN_DEEPSEEK_INTEGRATION := 1

test-integration-deepseek:
	@printf 'test-integration-deepseek: calls the real DeepSeek API, which incurs charges (model: %s)\n' "$$YT2COLUMN_MODEL"
	$(GOTEST) -tags integration -count=1 -timeout $(DEEPSEEK_INTEGRATION_TIMEOUT) -v ./internal/llm/deepseek

# CLI integration test. Runs cmd/yt2column from a seeded transcript cache to
# the --out file with the real DeepSeek API, which incurs charges; yt-dlp is
# never started. Like the DeepSeek integration test it is kept out of
# `make test` by the `integration` build tag, reads its API key from
# YT2COLUMN_TEST_DEEPSEEK_API_KEY, and uses the YT2COLUMN_MODEL default above;
# unlike it, the test fails rather than skips when that key is missing.
# One Generate call of at most 15 minutes (provider.LLMTimeout), plus margin.
CLI_INTEGRATION_TIMEOUT ?= 20m

# Exported to this target's recipe only; the recipe never splices the values
# into shell text.
test-integration-cli: export YT2COLUMN_MODEL := $(YT2COLUMN_MODEL)
test-integration-cli: export YT2COLUMN_CLI_INTEGRATION := 1

test-integration-cli:
	@printf 'test-integration-cli: calls the real DeepSeek API, which incurs charges (model: %s)\n' "$$YT2COLUMN_MODEL"
	$(GOTEST) -tags integration -count=1 -timeout $(CLI_INTEGRATION_TIMEOUT) -v ./cmd/yt2column

# golangci-lint compiles the integration tests only together with the `test`
# helpers; vet the `-tags integration` build that `make test-integration`,
# `make test-integration-deepseek`, and `make test-integration-cli` run, so a
# compile error there fails lint too.
lint:
	$(GOLINT)
	$(GOCMD) vet -tags integration ./...

fmt:
	$(call check_gofumpt)
	@CHANGED=$$(mktemp); \
	{ git diff --name-only HEAD 2>/dev/null; git diff --name-only --cached; git ls-files --others --exclude-standard; } \
		| grep '\.go$$' | sort -u > "$$CHANGED" || true; \
	if [ -s "$$CHANGED" ]; then \
		$(call format_files_from_list,cat "$$CHANGED"); \
	else \
		echo "No changed Go files to format"; \
	fi; \
	rm -f "$$CHANGED"

fmt-all:
	$(call check_gofumpt)
	@$(call format_files_from_list,find . -name '*.go' -not -path './vendor/*' -not -path './extension/*')

deadcode:
	deadcode -test -tags test $(MAIN_PKG)

tidy:
	$(GOCMD) mod tidy

# Browser extension (extension/). Node.js is invoked only inside these
# recipes, never at parse time, so the Go targets above do not need Node.js.
EXT_DIR=extension

# Fails unless node and npm are exactly the versions pinned in
# extension/.node-version and the packageManager field of package.json.
define ext_check_versions
	@want_node=$$(cat $(EXT_DIR)/.node-version); \
	have_node=$$(node --version 2>/dev/null); have_node=$${have_node#v}; \
	if [ "$$have_node" != "$$want_node" ]; then \
		echo "Error: Node.js $$want_node is required (extension/.node-version), found '$$have_node'"; \
		exit 1; \
	fi; \
	want_npm=$$(cd $(EXT_DIR) && npm pkg get packageManager | tr -d '"'); want_npm=$${want_npm#npm@}; \
	have_npm=$$(npm --version 2>/dev/null); \
	if [ "$$have_npm" != "$$want_npm" ]; then \
		echo "Error: npm $$want_npm is required (packageManager in extension/package.json), found '$$have_npm'"; \
		exit 1; \
	fi
endef

# Version check plus installed dependencies. Never installs them: reaching
# the registry is left to an explicit `make ext-install`.
define ext_preflight
	$(call ext_check_versions)
	@if [ ! -d $(EXT_DIR)/node_modules ]; then \
		echo "Error: $(EXT_DIR)/node_modules is missing; run 'make ext-install' first"; \
		exit 1; \
	fi
endef

# Installs exactly the lockfile. npm ci fails when package.json and the
# lockfile disagree, and --ignore-scripts keeps dependency install scripts
# from running even if .npmrc is overridden.
ext-install:
	$(call ext_check_versions)
	cd $(EXT_DIR) && npm run --silent check-lockfile
	cd $(EXT_DIR) && npm ci --ignore-scripts --no-audit --no-fund

ext-typecheck:
	$(call ext_preflight)
	cd $(EXT_DIR) && npm run --silent typecheck

ext-lint:
	$(call ext_preflight)
	cd $(EXT_DIR) && npm run --silent lint

ext-fmt-check:
	$(call ext_preflight)
	cd $(EXT_DIR) && npm run --silent fmt-check

# Rewrites files; CI runs ext-fmt-check instead.
ext-fmt:
	$(call ext_preflight)
	cd $(EXT_DIR) && npm run --silent fmt

ext-test:
	$(call ext_preflight)
	cd $(EXT_DIR) && npm run --silent test

# Rebuilds dist/ from scratch and checks it (scripts/check-dist.ts).
ext-build:
	$(call ext_preflight)
	cd $(EXT_DIR) && npm run --silent build

# Everything CI runs after ext-install, in the same order.
ext-check:
	@$(MAKE) --no-print-directory ext-typecheck
	@$(MAKE) --no-print-directory ext-lint
	@$(MAKE) --no-print-directory ext-fmt-check
	@$(MAKE) --no-print-directory ext-test
	@$(MAKE) --no-print-directory ext-build
