# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Quick Links

**Project:**
- [Project Overview](docs/dev/project_overview.md) - Decided policies, pipeline, constraints, configuration (Japanese). **Read this before designing anything.**
- [Security Considerations](docs/dev/security.md) - yt-dlp invocation, secrets, network, cache, untrusted text
- [Cache Consistency](docs/dev/cache_consistency.md) - What the transcript cache guarantees across failures and power loss, and why (Japanese). Check its checklist before changing cache code.

**Development Guides:**
- Requirements and Acceptance Criteria Process: [requirements_process.md](docs/dev/developer_guide/requirements_process.md) - Process for implementing new features
- Test Organization Guide: [test_organization.md](docs/dev/developer_guide/test_organization.md) - Test helper file organization
- [Package Reference](docs/dev/developer_guide/package_reference.md) - Detailed package structure
- [mergepr Guide](docs/dev/developer_guide/mergepr_guide.md) - Setup, usage, guarantees, and troubleshooting for the `/mergepr` tool

## Documents

- Documents should be placed under docs
- Default language is Japanese (exceptions: README.md, CLAUDE.md, docs/dev/developer_guide/\*.md)
- Default format is markdown
- Use Mermaid for diagrams, following the conventions in
  [mermaid_reference.md](docs/dev/developer_guide/mermaid_reference.md) (node-label
  quoting, `<br>` line breaks, cylinder `[(...)]` data nodes, the standard classDef
  palette, and Legend blocks).
- Each task lives in `docs/tasks/NNNN_<name>/` (four-digit sequence number).
  `docs/tasks/0000_template/requirements.md` is the starting point for
  `01_requirements.md`.

### Translation Guidelines (Japanese to English)

Create and commit the Japanese version first, then write the English version from
it. The translation principles (accuracy over fluency, faithful translation,
structural consistency) and the glossary workflow live in
[mktrans.md](.claude/commands/mktrans.md), which automates this; the glossary is
[docs/translation_glossary.md](docs/translation_glossary.md).

## Commands

### Build Commands
- `make build` - Build the `yt2column` binary into `build/`
- `make clean` - Clean build artifacts
- `make all` - Default build target
- `make install-mergepr` - Install the `/mergepr` developer tool (`go install ./cmd/mergepr`)

### Test Commands
- `make test` - Run all tests with race detection
- `go test -tags test -v ./...` - Run all tests directly
- `go test -tags test -v ./internal/specific/package` - Run tests for specific package

### Code Quality
- `make lint` - Run golangci-lint (version pinned in `Makefile`; includes `govet`)
- `make fmt` - Format changed Go files with gofumpt (`make fmt-all` for every file)
- `make deadcode` - Report unreachable functions

### Claude Commands (`.claude/commands/`)
Project-specific values these commands depend on are defined once in
[_context.md](.claude/commands/_context.md).
- `/mkarch <task>` - Create `02_architecture.md` (requires approved requirements)
- `/mkplan <task>` - Create `03_implementation_plan.md` (requires approved architecture)
- `/mkplan2 <task>` - Embed PR boundaries into an approved implementation plan
- `/runplan <task>` - Implement the next phase group of an approved plan
- `/weakreview [range]` - Second-pass review of work done by a lower-capability model
- `/fixpr [PR]` - Resolve unresolved PR review threads
- `/mergepr [PR]` - Squash-merge a PR with a generated commit message and clean up its branch
- `/japrose <file>` - Japanese prose-quality pass
- `/mktrans <file>` - Japanese ⇄ English translation

## Architecture Overview

A Go CLI that turns a YouTube video into a magazine-column-style article:

```
URL → TranscriptSource → Transcript → ArticleWriter → Article → Publisher
```

- `TranscriptSource` (`internal/transcript`): fetches subtitles and info.json by
  invoking the external `yt-dlp` command, parses json3, caches per video ID.
- `internal/strictjson`: strict JSON extraction shared by `internal/transcript`
  and `internal/llm/deepseek`; rejects the byte sequences and structural shapes
  `encoding/json` would silently repair.
- `ArticleWriter` (`internal/writer`): provider-independent prompt assembly and
  output post-processing; calls an `LLMClient`.
- `LLMClient` (`internal/llm`, implementations in `internal/llm/<provider>`): thin
  per-provider adapter — system + user prompt in, text out.
- `Publisher` (`internal/publisher`): Slack Incoming Webhook and local file.
- `internal/pipeline` orchestrates the stages; `internal/config` reads environment
  variables; `cmd/yt2column` wires everything together.

See [Project Overview](docs/dev/project_overview.md) for the decided policies and
constraints, and [Package Reference](docs/dev/developer_guide/package_reference.md)
for the packages that exist today.

### Key Design Patterns
- **Separation of Concerns**: Each package has a single responsibility. Core logic
  stays out of `cmd/` so it can later back a Slack bot or a scheduled job.
- **Interface-based Design**: Every pipeline stage (`TranscriptSource`,
  `LLMClient`, `Publisher`) is an interface with a fake for tests.
- **Provider details stay in their adapter package**: request/response shapes of
  a provider API (e.g. DeepSeek's `thinking` parameter and `reasoning_content`)
  live only under `internal/llm/<provider>`. If a provider SDK is added later
  (`google.golang.org/genai` for `internal/llm/gemini`, the Anthropic SDK for
  `internal/llm/claude`), it is imported only there; `depguard` in
  `.golangci.yml` already enforces that.
- **Delegate the fragile part**: subtitle retrieval is delegated to `yt-dlp`. Do
  not reimplement YouTube scraping in Go.
- **YAGNI**: Use simple and clear approach to satisfy the requirement. Don't take complex approach for not-yet-planned features. No LangChain-style frameworks or multi-provider abstraction libraries.
- **DRY**: Don't repeat yourself. Before adding new code, check the codebase and prefer reusing existing implementations.
- **Declare, don't infer**: Do not choose behavior by inspecting the content of a
  string supplied by a caller (`strings.Contains`/`HasPrefix` over caller data to
  pick a code path). Put the choice in the type: an enum field whose zero value is
  the interpretation that assumes least about its input, dispatched by a `switch`
  whose `default` fails secure. Changing a struct's shape is cheap here — the
  project has no consumers outside the repository, so "it would change the exported
  API" is not on its own a reason to keep inferring.
- **Enforce invariants with the type, not with convention**: if a value must pass
  validation before use (e.g. a video ID), make the compiler the thing that
  guarantees it — unexported fields plus a mandatory constructor — rather than
  relying on every caller remembering to validate. Before preserving an exported
  field as an extension point, count its real uses; an extension point nobody uses
  costs the guarantee and buys nothing.
- **Reject, don't normalize**: when code can either quietly repair a caller's
  malformed input or reject it, reject it. Silent repair makes a wrong definition
  indistinguishable from a right one, so the mistake never surfaces and propagates
  into whatever someone copies next. Prefer a `validate` method returning a
  sentinel error, rejection at the construction boundary, and a test over the real
  defaults that fails the build when they are edited into an invalid shape.
- **Empty is an error**: an empty LLM response, a missing subtitle file after a
  successful `yt-dlp` exit, or a truncated generation is reported as an error,
  never published.

### Performance

- **A benchmark regression is not by itself a defect.** Justify an optimization
  against an *absolute* budget — wall time of a real run — never against a relative
  delta versus the previous commit. A run is dominated by `yt-dlp` and the LLM API
  call (seconds to tens of seconds); local processing that costs milliseconds is
  not worth a mechanism.
- **Profile before optimizing** to confirm the cost is where you assume. If the
  stage you are about to optimize is a small fraction of its path's total cost,
  stop.
- **An optimization that adds a correctness obligation** — encoding assumptions,
  cache invalidation, a fast path that must agree with the slow path — must clear a
  much higher bar. State the obligation in the commit message and pin it with a
  test that fails if the optimization is later changed unsafely.
- **Optimizations go in their own commit**, separable by revert, and never in the
  same commit as the behavior change that motivated them.
- **Do not close a review finding by adding a mechanism until the finding's premise
  is verified.** Measure whether the reported cost is real harm in absolute terms
  first.

### Testing Strategy

Unit tests never invoke `yt-dlp`, an LLM API, or a Webhook. Each pipeline
interface has a fake (see the Test Organization Guide); `ArticleWriter` is tested
with a fake `LLMClient`, HTTP code with `net/http/httptest`, and the json3 /
info.json parsers with real samples under `testdata/`.

- **Error Testing**: Use `errors.Is` / `errors.AsType[T]` to validate error types,
  never string matching on error messages.
- **Every test must be able to fail for its stated reason.** Before committing a
  test, disable the thing it claims to cover — nil the collaborator, revert the
  branch, break the default — and confirm it fails. Say in the commit message that
  you did. This is checked by breaking the code, never by arguing it in prose: a
  design or planning document must not claim that a test "would fail if the
  implementation were X", because that is a statement about code nobody has written
  and no reviewer can check.
- **A layered path needs inputs only one layer can handle.** Where two mechanisms
  can produce the same output, an input both layers match proves nothing about
  either. Construct the input so only the layer under test can act, and assert
  first that the other layer alone leaves it untouched.
- **Do not assert that a constant equals its own literal**, or that a struct field
  holds what was just assigned to it. These pass unconditionally and reach nothing.
- **Check for duplication before adding a table test for a helper.** If the helper
  differs from its caller only by a loop, and the caller's table already covers the
  same rows with the same literals, one table is enough.
- **Deleting a test is a claim that must be checked**: confirm `go tool cover -func`
  is unchanged function by function afterwards, and say so.

See [Test Organization Guide](docs/dev/developer_guide/test_organization.md) for test helper file structure.

## Development Notes

- Go version: see `go.mod`.
- After editing go files, make sure to run `make fmt` to format the files.
- After editing files, make sure to run `make test` and `make lint` and fix errors.
- `yt-dlp` must be on `PATH` (or set `YT2COLUMN_YTDLP_PATH`) to run the CLI; it is
  not needed for `make test`.
- Secrets (`DEEPSEEK_API_KEY`, `SLACK_WEBHOOK_URL`, ...) go in `.envrc` / `.env`,
  which are git-ignored. Never commit them and never print them.
- Real captured data committed under `testdata/` must come from a video licensed
  for redistribution and must not identify whoever captured it; full-page browser
  snapshots are never committed and live outside the repository. See
  [Security Considerations §7](docs/dev/security.md) before adding any.

### Dependencies

Keep external libraries to a minimum. HTTP, JSON, CLI flags, and configuration use
the standard library. The allowed modules are listed in the `depguard` `deps` rule
in `.golangci.yml`; adding one means adding it there, and the commit message (and
the task's architecture document, when there is one) must state why the standard
library is not enough. Currently no module outside the standard library is
allowed: the initial LLM provider (DeepSeek) exposes an OpenAI-compatible HTTP API
that is called with `net/http` and `encoding/json`.

## Go Idioms

Modern-idiom drift (`any` over `interface{}`, `for range n`, `min`/`max`, the
`slices`/`maps` packages, `strings.Cut`, `slices.SortFunc` over `sort.Slice`, …) is
enforced mechanically by the `modernize`, `intrange`, `copyloopvar`, and
`usestdlibvars` linters in `.golangci.yml`, most of them auto-fixable with
`golangci-lint run --fix`. Run `make lint` rather than working from a list here.

Only the conventions a linter does not check are worth stating:

- Prefer `errors.AsType[T]` over `errors.As` — it eliminates the `var target T`
  declaration. This is a Go 1.26 API, so it is not what habit produces:
  ```go
  // Before
  var pathErr *fs.PathError
  if errors.As(err, &pathErr) { ... }

  // After
  if pathErr, ok := errors.AsType[*fs.PathError](err); ok { ... }
  ```
- Use `map[T]struct{}` rather than `map[T]bool` for set semantics.
- In tests, prefer `t.Cleanup` over manual `defer` chains and `t.TempDir` over
  `os.MkdirTemp` + `defer os.RemoveAll`.
- Go comments, identifiers, and string literals are English.

## Requirements and Acceptance Criteria

When implementing new features, follow the process documented in [Requirements Process Guide](docs/dev/developer_guide/requirements_process.md).

**Quick summary:**
1. Create `01_requirements.md` with explicit acceptance criteria
2. Create `02_architecture.md` with high-level design (Mermaid diagrams)
3. Create `03_implementation_plan.md` with progress tracking (checkboxes) and AC traceability
4. Write tests for each acceptance criterion
5. Link tests to acceptance criteria in the implementation plan

Each document is created as `draft`; the next one is not started until a human
reviewer sets the previous one to `approved`.

## Tool Execution Safety

**CRITICAL**
- Don't run following commands without user's explicit approval
  - commands interacting with network, e.g. git pull
  - merging pull requests on GitHub
  - running the CLI against real services (LLM API calls, Slack Webhook posts)
- `git commit` and `git push` may be executed without explicit approval
