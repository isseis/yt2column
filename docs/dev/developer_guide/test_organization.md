# Test Helper File Organization

Test helper files follow a two-tier classification system based on their scope and dependencies:

## Classification A: `testutil/` Subdirectory (Cross-Package Helpers)

**Use for**: Test helpers and mocks used across multiple packages or that only use public APIs

```
<package>/
├── <implementation>.go
├── <implementation>_test.go
└── testutil/
    ├── mocks.go              # Lightweight mocks (no external dependencies)
    ├── testify_mocks.go      # testify-based mocks (for complex scenarios)
    ├── mocks_test.go         # Tests for mock implementations
    └── helpers.go            # Test utility functions
```

**File Naming Rules:**
- **`testutil/mocks.go`**: Simple mock implementations without external library dependencies
- **`testutil/testify_mocks.go`**: Advanced mocks using stretchr/testify framework
- **`testutil/mocks_test.go`**: Unit tests for mock implementations
- **`testutil/helpers.go`**: Common test utility functions and setup helpers

**Package Naming:**
- Use a domain-prefixed package name within the `testutil/` subdirectory: `package <domain>testutil`
  - Examples: `package commontestutil`, `package securitytestutil`, `package verificationtestutil`
- Import without an alias: `<module>/internal/<package>/testutil`
- The unique package name eliminates the need for import aliases at call sites, preventing alias drift across the codebase

**Pipeline fakes:** Each pipeline interface (`TranscriptSource`, `LLMClient`,
`Publisher`, ...) gets a fake implementation in the `testutil/mocks.go` of the
package that defines the interface (e.g. `internal/llm/testutil/mocks.go`,
`package llmtestutil`). Unit tests never invoke `yt-dlp`, an LLM API, or a
Webhook; they use these fakes instead.

**testify:** `testutil/testify_mocks.go` applies only once
`github.com/stretchr/testify` is adopted. Adding it is a dependency decision
(see `CLAUDE.md`, "Dependencies"); until then write hand-rolled fakes in
`testutil/mocks.go`.

## Classification B: Package-Level `test_helpers.go` (Internal Helpers)

**Use for**: Test helpers that must remain in the same package due to:
- Adding methods to package-internal types
- Using non-exported (private) package APIs
- Avoiding circular dependencies

```
<package>/
├── <implementation>.go
├── <implementation>_test.go
└── test_helpers.go           # Package-internal test helpers
```

**File Naming Rules:**
- **`test_helpers.go`**: Single file for package-internal test helpers
- If multiple helper categories needed: `test_helpers_<category>.go` (e.g., `test_helpers_group.go`)

**Package Naming:**
- Use the same package name as the production code
- Always include `//go:build test` build tag

## Guidelines for New Test Helpers

When adding new test helper code, follow this decision tree:

1. **Does the helper use only public APIs?**
   - Yes → Continue to step 2 (Classification A)
   - No → Continue to step 4 (likely Classification B)

2. **What type of test helper are you creating?** (Classification A - `testutil/` subdirectory)
   - **Mock implementation** → Choose based on complexity:
     - Simple mock (no external dependencies) → `testutil/mocks.go`
     - Complex mock (using testify/mock) → `testutil/testify_mocks.go`
   - **Helper function** (setup, utilities, fixtures) → `testutil/helpers.go`
   - **Mock tests** → `testutil/mocks_test.go`

3. **Is the helper used by tests in other packages?**
   - Yes → Ensure it uses only public APIs, then place in appropriate `testutil/` file (step 2)
   - No → Continue to step 4

4. **Package-internal considerations** (Classification B - `test_helpers.go`)
   Place in `test_helpers.go` if the helper:
   - Adds methods to package-internal types
   - Uses non-exported (private) package APIs
   - Would create circular dependencies if placed in `testutil/` subdirectory
   - If multiple helper categories exist: use `test_helpers_<category>.go` (e.g., `test_helpers_group.go`)

**Build Tags:**
- All test helper files must include `//go:build test` at the top
- This ensures they are only compiled during test builds, not in production binaries
- **Exception:** a helper that integration tests (built with `-tags integration`
  alone) also use — a `testutil/` file or a `test_helpers_*.go` file — carries
  `//go:build test || integration` instead. It is still excluded from the
  production binary. `TestFakesCarryBuildTag` (`internal/pipeline`) accepts
  exactly these two first lines for every `testutil/` file under `internal/` and
  every `test_helpers*.go` file under `cmd/` and `internal/`.

**Examples:**
- Mock interface implementation → `testutil/mocks.go` or `testutil/testify_mocks.go`
- Test setup helper function → `testutil/helpers.go`
- Method on internal type → `test_helpers.go`
- Factory function using private constructor → `test_helpers.go`
