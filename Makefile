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

.PHONY: all build clean test test-ci test-integration test-integration-deepseek lint fmt fmt-all deadcode tidy install-mergepr

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

# golangci-lint compiles the integration tests only together with the `test`
# helpers; vet the `-tags integration` build that `make test-integration` and
# `make test-integration-deepseek` run, so a compile error there fails lint too.
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
	@$(call format_files_from_list,find . -name '*.go' -not -path './vendor/*')

deadcode:
	deadcode -test -tags test $(MAIN_PKG)

tidy:
	$(GOCMD) mod tidy
