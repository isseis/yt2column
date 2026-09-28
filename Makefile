GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
# Pin golangci-lint to the exact version CI enforces (.github/workflows/ci.yml
# and .pre-commit-config.yaml). Using `go run @version` ignores whatever
# golangci-lint is on PATH, so local `make lint` always matches CI. Bump all
# three pins together.
GOLANGCI_VERSION?=v2.13.2
GOLINT=$(GOCMD) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run --build-tags test
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

.PHONY: all build clean test test-ci lint fmt fmt-all deadcode tidy

all: build

build:
	@mkdir -p build
	$(GOBUILD) -trimpath -ldflags "-s -w" -o $(BINARY) $(MAIN_PKG)

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

lint:
	$(GOLINT)

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
