# Package Reference

This document lists the packages under `cmd/` and `internal/`, plus the
`prompts` package at the repository root, with a short summary of each one's
responsibility. Update it in the same commit that adds, removes, or changes the
responsibility of a package.

The planned layout and the responsibility of each not-yet-created package are
described in [project_overview.md](../project_overview.md) ("想定ディレクトリ構成" and
"パイプライン").

### What a row holds

A row holds only:

- the package's responsibility, in one to three sentences;
- the names of its main exported types, functions, and sentinel errors;
- a link to where its detailed specification lives, at most one link per
  linked document.

A row does not hold unexported names, numbers or limits, exit codes, the list
of environment variables a child process receives, file names, or procedures.

### Where details live

The detailed behavior of a package belongs in:

- the package's and its exported identifiers' doc comments;
- a current document under `docs/dev/` (a specification document or a guide),
  `README.md`, or `prompts/README.md`;
- the section of a completed task's design document that describes the
  behavior as it is now.

When one of these documents already has a detail, link to that document
instead of copying the detail into the row. Move a contract that callers rely
on (accepted and rejected input, the sentinel that reports a failure, an
ordering or atomicity guarantee, how a secret is handled) to the package's or
the exported identifier's doc comment when no document has it. Otherwise the
code is the source of truth, and the detail stays out of the row.

### Kinds of package

- **Pipeline components** run the article pipeline or carry the values it
  passes: the stages, their data types, and the CLI entry point and templates
  that supply them.
- **LLM** packages define the provider-independent client contract, the
  shared HTTP send path and provider factory, and the adapter and parameter
  types of each provider.
- **Shared parts** are packages used by several others, such as configuration
  loading, directory locking, command execution, redaction, secrets, strict
  JSON, nil checks, and signal handling.
- **Development tools** are the commands and packages behind `/mergepr`,
  `/fixpr`, `/runpr`, and the local review and coverage helpers.
- **Test-only packages** are test utilities and packages that only tests use,
  wherever they sit.

A task that adds a package that fits none of these kinds decides whether to
add a kind here or to widen one of these definitions.

## Pipeline Components

| Package | Responsibility |
|---|---|
| `cmd/yt2column` | Entry point of the CLI (`yt2column [flags] <video URL>`): `main` builds the stages and runs one job through `internal/job`, writing the article to `--out` and the run's summaries to standard error. Usage and exit codes: [CLI guide](../guides/cli.md). |
| `internal/artifact` | Persists and resumes the article writer's intermediate artifacts, one file per video ID and stage: `Parse`, `Marshal`, `NewStore`, `Save`, `Load`; `ErrMissing`, `ErrMalformed`, `ErrMismatch`. Format: [0077 design §3.4.2](../../tasks/0077_openai_llm_client/02_architecture.md). |
| `internal/extjob` | One browser extension job of the article generation module: `StartInput`, `Prepare`/`Prepared` (the input's validation and `InputDigest`), `Run`/`Result`, `Failure`, `Steps`, `Observer`, `Deps`, `Backend`, `BackendInfo` (built by `ClaudeInfo` and `FakeInfo`), and `NewModule`/`Module` with the four JSON entry points `CheckSettingsJSON`, `PrepareJSON`, `GenerateJSON`, and `BuildInfoJSON`; `ErrInvalidInput`. It classifies `llm.ErrAuthRejected` as `FailureAPIKeyRejected`. Design: [0091 design §3.3](../../tasks/0091_extension_only_migration/02_architecture.md). |
| `internal/extjob/claudebackend` | The release `extjob.Backend`: `New` builds the production profile's Claude clients with `claude.New` (the same options as `internal/llm/provider`, which it cannot import into a js build) and checks keys with `claude.CheckKey`; `ErrNotClaude`. |
| `internal/extjob/devbackend` | The development `extjob.Backend` of a `devfake.Scenario`: `New`. It calls no network and does not import the Claude adapter. |
| `internal/extjob/jsmodule` | The `syscall/js` glue both article generation modules share, built only for js: `Serve` registers an `extjob.Module`'s four JSON entry points as `checkSettings`, `prepare`, `generate`, and `buildInfo` on the `globalThis` object named `GlobalName`, and keeps the program running. It parses nothing; the JSON is `internal/extjob`'s. |
| `cmd/yt2column-wasm` | Entry point of the release article generation module, built only for js/wasm: it serves `extjob.NewModule(claudebackend.New())` with `jsmodule.Serve`. It must not depend on the fake LLM or the development backend (`internal/extjob`'s `TestModuleDependencies`). |
| `cmd/yt2column-wasm-dev` | Entry point of the development article generation module, not a production artifact: it parses the scenario the build embeds in five `main` variables with `devfake.ParseScenario` and, when it parses, serves `extjob.NewModule(devbackend.New(scenario))` with `jsmodule.Serve`. A native build only parses the same variables and exits 0 or 1, for the extension build to check a scenario first. It must not depend on the Claude adapter or the release backend. |
| `cmd/yt2column-extjob` | Entry point of the development command that runs the article generation module's JSON entry points natively on an input file, not a production binary: `-llm=claude` (the API key and workspace ID through `config.ReadExtensionSettings`) or `-llm=fake` with the five scenario arguments, required. Usage: [development.md](../guides/development.md). |
| `internal/job` | Runs one job end to end, from the output checks and the cache lock through publishing: `Run`, `Request`, `Result`, `*ReviewLogError`. Output rules: [CLI guide](../guides/cli.md). |
| `internal/pipeline` | Runs the three stages in order: `Pipeline`, `Stage`, `StageError`, `ErrNilStage`. |
| `internal/pricing` | Estimates an LLM call's cost from its token counts and the providers' published rates: `Estimate`, `Totals`, `StageCost`, `Amount`, `RateBasis`, `UnknownReason`. Updating the rates: [pricing update guide](../guides/pricing_update.md). |
| `internal/publisher` | Defines the publishing stage: `Publisher`, `FilePublisher`, `FileContent`, `ReviewLogWriter`, `FormatArticleCost`, and the mention neutralization of the display Markdown (`NeutralizedDisplay`, `NeutralizedDisplayMarkdown`, `NeutralizeHeading`); `ErrOutputExists`, `*KeptFileError`. File format: [CLI guide](../guides/cli.md); neutralization rule: [security.md](../security.md). |
| `internal/transcript` | Defines the transcript stage (`Transcript`, `TranscriptSource`) with `CopiedSource`; `ValidateVideoURL` and `NormalizedVideoURL` are the video URL rules. The `yt-dlp` path and its cache live in `internal/ytdlpsource`. |
| `internal/writer` | Defines the article-writing stage: `Article`, `ArticleWriter`, `ProgressWriter`, `New`, `NewProgressWriter`, `RunUsage`, `CheckChannelName`. The template contract: [prompts/README.md](../../../prompts/README.md). |
| `internal/ytdlpsource` | Implements `transcript.TranscriptSource` with `YtDlpSource`: runs `yt-dlp` without a shell and parses its json3 and info.json output. It is the development path of the CLI and is not built into the WebAssembly core. The cache's guarantees: [cache consistency](../cache_consistency.md); the `yt-dlp` boundary: [security.md](../security.md). |
| `prompts` | Embeds the default prompt templates and returns them: `System`/`User`, `ShortenSystem`/`ShortenUser`, `ReviewSystem`/`ReviewUser`. [README](../../../prompts/README.md) documents the contract. |

## LLM

| Package | Responsibility |
|---|---|
| `internal/llm` | Defines the provider-independent `LLMClient` contract: `GenerateRequest` (with `Validate`) and `GenerateResponse`, the common sentinels `ErrInvalidRequest`, `ErrTruncated`, `ErrUnexpectedFinishReason`, `ErrEmptyResponse`, and `ErrAuthRejected`, and the usage types `TokenCounts` and `Usage`. Only the Claude adapter returns `ErrAuthRejected` (for a 401); the DeepSeek and OpenAI adapters do not, so a caller that classifies with it pairs it with the Claude adapter. |
| `internal/llm/claude` | Implements `llm.LLMClient` against the Anthropic Messages API: `New`; `CheckKey` and `KeyCheckOptions`, which call the Models API once to check an API key (a 401 matches `llm.ErrAuthRejected`); `ErrHTTPStatus`, `ErrInvalidResponse`, `ErrTransport`. It has files only a `js/wasm` build compiles. |
| `internal/llm/claudeparam` | The Claude adapter's parameter types `Effort` and `WorkspaceID`, used by the adapter and `internal/config`; standard library only. |
| `internal/llm/deepseek` | Implements `llm.LLMClient` against the DeepSeek Chat Completions API: `New`; `ErrHTTPStatus`, `ErrInvalidResponse`, `ErrTransport`, `ErrPaddedModel`. |
| `internal/llm/deepseekparam` | The DeepSeek adapter's parameter type `Effort`, used by the adapter and `internal/config`; standard library only. |
| `internal/llm/devfake` | The fake LLM of the development builds, not a production adapter: `NewScenario` checks a `Scenario` (`Article`, `Failure`, `FailAt`, delay), and `ParseScenario` builds one from the five spellings of `ScenarioSpelling` (the article, the failure, the failing call, the delay in seconds, and the settings check result), rejecting an empty or unknown value unchanged; `Scenario.Spelling` reports the five spellings back; `Scenario.NewJob` returns the clients of one generation job, which return fixed articles or the selected failure, and `Scenario.CheckKeyResult` reports the selected `SettingsCheck`; `Articles`, `Failures`, `FailAts`, and `SettingsChecks` list the values a caller may select; `ModelName`, `TitleMarker`, `MaxDelay`; `ErrInvalidScenario`, `ErrSimulatedAPIFailure`, `ErrUnexpectedCall`. Its rejected-key failure and its key-rejected settings check wrap `llm.ErrAuthRejected`. It depends on `internal/llm` and the standard library only, and no production binary links it. |
| `internal/llm/llmhttp` | Sends one HTTP request for an LLM adapter: `Post` (a POST that returns the body of a 200 response) and `Status` (a request that returns only the status code, without reading the body), `NewClient`, `CheckAPIKey`, `MaxResponseBytes`; `ErrZeroAPIKey`, `ErrInvalidAPIKey`. It defines no send-failure sentinel of its own and reports a failure with the adapter's sentinel. It has a file only a `js/wasm` build compiles. |
| `internal/llm/openai` | Implements `llm.LLMClient` against the OpenAI Responses API: `New`; `ErrHTTPStatus`, `ErrInvalidResponse`, `ErrTransport`. The default output limit: [OpenAI output limit guide](../guides/openai_output_limit.md). |
| `internal/llm/openaiparam` | The OpenAI adapter's parameter type `Effort` (with `EffortNone` to turn reasoning off), used by the adapter and `internal/config`; standard library only. |
| `internal/llm/provider` | Builds the `llm.LLMClient` a `config.LLMConfig` selects: `New` for generation and `NewReview` for review, rejecting an unknown provider. |

## Shared Parts

| Package | Responsibility |
|---|---|
| `internal/cachelock` | Serializes the processes that share one directory with an exclusive `flock`: `Acquire`, `File`, `Close`; `*LockedError`, `ErrLocked`. |
| `internal/cmdrunner` | The shared execution boundary for external commands: `Runner`, `InputRunner`, `NewOS`, `GitEnv`, `GitHardening`; `MaxOutput`, `ErrOutputTooLarge`, `ErrUnexpectedStderr`, `*ExitError`. The `yt-dlp` boundary: [security.md](../security.md). |
| `internal/config` | Reads and validates the process configuration: `Load`, `LoadFor`, `LoadExtension`, `ExtensionLookup`, `ReadExtensionSettings`, `LLMConfig`, `Need`, `ExtensionSettings`; `*VarError`, `ErrMissing`, `ErrInvalid`. Reference: [README](../../../README.md#設定). |
| `internal/nilcheck` | Reports whether a value is nil, including a typed nil held in an interface (`IsNil`). |
| `internal/privatefile` | Reads and creates files only their owner may read (`internal/chief` uses it): `Read`, `Create`, `Remove`, `MkdirBelow`; `*UnsafeError`, `ErrUnsafe`, `ErrNotExist`, `ErrExist`, `ErrTooLarge`, `ErrBaseMissing`. Rules: [security.md §8.2](../security.md). |
| `internal/redact` | Keeps secret values out of output: `Line`, `SecretStrings`. |
| `internal/secret` | Holds a secret (API key) and guarantees it never appears in `fmt`, `log/slog`, or JSON output. |
| `internal/signalrun` | `Run` calls a program's main function under a context that the first subscribed signal cancels; a second signal terminates the process. |
| `internal/strictjson` | Extracts values from JSON documents that passed strict checks: `ParseArray`, `Object.CollectOnly`, `Object.CollectExactCase`, `Object.Has`, `Value.AsBool`, `Value.IsNull`, `Value.AsNonNegativeInt64`. The rejected inputs: [0003 design §3.5](../../tasks/0003_deepseek_llm_client/02_architecture.md). Shared by `internal/ytdlpsource`, `internal/llm/deepseek`, `internal/llm/claude`, `internal/llm/openai`, `internal/writer`, `internal/artifact`, `internal/extjob`, `internal/fixprguard`, `internal/codexreview`, `internal/coveragecompare`, `internal/chief`, and `internal/numreserve` |
| `internal/tasknaming` | The name forms of task branches, task directories, and ADR files, and the four-digit number they carry: `Number` (`String`, `Int`), `ParseNumber`, `ParseBranch`, `ParseTaskDir`, `ParseADRFile`; `ErrInvalid`. Shared by `internal/chief`, `internal/numreserve`, and the ADR guard test. |

## Development Tools

| Package | Responsibility |
|---|---|
| `cmd/chief` | Entry point of the `chief` developer tool: `main` reads the environment once (`chief.ReadEnv`), parses `--protocol`, the subcommand (`start`, `snapshot`, `tick`, `wait`, `send`, `relay`, `reset`, `detail`, `release`, `targets`, `create`, `remove` today), and its flags (`targets` and `create` take `--issue` as text, which `internal/chief` checks), and prints one JSON value with the exit code of the 0088 design (3.1.1) and the 0092 design (3.1). |
| `cmd/codexreview` | Entry point of the `codexreview` developer tool, which runs Codex CLI's local review when the CodeRabbit CLI fails: `main` parses `--protocol` and `--base`, runs `codexreview.Run`, and prints its `Result` as JSON. |
| `cmd/coveragecompare` | Entry point of the coverage comparison tool behind `make coverage-compare` and `make ext-coverage-compare`: it parses `go --base <rev>` or `ext --base <rev>` and runs `coveragecompare.Run`. |
| `cmd/fixprguard` | Entry point of the `fixprguard` guard tool behind `/fixpr` and `/runpr`: it parses the subcommands and orders the `internal/fixprguard` calls. Usage: [fixprguard guide](../guides/fixprguard_guide.md). |
| `cmd/mergepr` | Entry point of the `/mergepr` developer tool: it prepares a PR for message drafting, then merges and cleans up after the user approves the message. Usage: [mergepr guide](../guides/mergepr_guide.md). |
| `cmd/numreserve` | Entry point of the `numreserve` developer tool: `main` takes one word (`task` or `adr`), runs `numreserve.Tool.Reserve`, and prints the number, or one stop line with its reason word, with the exit code of the design (3.1). The design: [0100 design](../../tasks/0100_number_reservation/02_architecture.md). |
| `internal/chief` | The decisions and side effects of the `chief` developer tool: the screen readers, which read an agent's pane screen and Claude Code's transcript into typed results (`Screen`, `Question`, `InputLine`, `RunprNext`, `RunprReport`, `ModelSwitch`, `HumanInput`, `SessionReport`); the sanitizing and read caps for text shown to the human (`Sanitize`, `CapHead`); the one path that runs herdr, git, and gh (`Commands`); and the readers of Herdr (`Herdr`), of a task's documents and PR markers (`Tasks`, `ParsePlan`), and of the routing table (`ParseRouting`, `ReadRouting`); the state directory and the lock (`Store`, `OpenStore`, `Owner`), the git and GitHub reader of a worktree (`GitReader`), the classification of each task workspace and its pending items (`TakeSnapshot`, `Snapshot`, `Row`, `PendingItem`), with each row's urgency, turn, and position (`Urgency`, `Turn`, `Position`, `PositionDocument`, `Progress`), the command its working agents run (`RunningCommand`, `RunningShown`, `RunningUnknown`), and the value names the output gives them (`OutputValueNames`); the place a task is made in and the workspaces at a path (`TaskSpace`, `NewTaskSpace`), the read of an issue from GitHub (`GitReader.ReadIssue`, `Issue`, `IssueStatus`), the registered worktrees and the ancestry of a commit (`GitReader.Worktrees`, `GitReader.IsAncestor`, `WorktreeEntry`), the task that answers an issue (`Tasks.IssueIndex`, `IssueIndex`, `Tasks.ReadDir`); the removal proposal of a workspace chief made whose task is done, with the checks before a removal (`RemovalCandidate`, `RemovalProposal`, `RemovalCheck`, `CheckResult`, `PendingRemoval`, `Row.RemovalRow`); the only writer of input to a pane (`PaneWriter`), the creation of a task's worktree and workspace with its `/mkreq` and the workspaces `/mkreq` can go to (`Create`, `CreateOutcome`, `CreateRefusal`, `CreateStage`, `CreateResult`, `Made`, `MadeKind`, `NotReady`, `Targets`, `TargetsOutcome`, `TargetWorkspace`), the only maker of a workspace (`WorkspaceMaker`, `NewWorkspaceMaker`), the removal of a finished task's workspace and worktree (`Remove`, `RemoveOutcome`, `RemoveRefusal`, `RemoveStage`, `RemoveResult`) by the only closer of a workspace and remover of a worktree (`WorkspaceCloser`), the exclusion between a creation and the automatic sends (`Store.LockCreation`, `CreationLock`, `ErrCreationBusy`), the send of a command with its checks and the reset after `/runpr` (`Send`, `ParseSendRequest`, `ActionResult`), the send of a reset skipped for want of a report and offered to the human (`SendOfferedReset`, `ParseOfferedResetRequest`, `ResetItem`), the send records, stops, and per-workspace exclusion (`WorkspaceRecords`, `HeldWorkspace`, `Release`), the record of a worktree chief made (`CreatedRecord`, `HeldWorkspace.KeepCreated`, `WorkspaceRecords.Created`, `HeldWorkspace.ClearRecords`), a PR's detail (`Detail`, `CollectDetail`, `PRDetail`), the wait for a change of the task agents (`Wait`, `WaitBaseline`), and the round of the wait with its observations and the automatic send after a merge (`Tick`, `Store.BeginRound`, `Observation`, `AutoSendResult`). The design: [0088 design](../../tasks/0088_chief_of_staff/02_architecture.md); the urgency, position, and running command: [0093 design](../../tasks/0093_chief_status_sheet/02_architecture.md). |
| `internal/chief/paneinput` | The only types that describe an input `chief` may send to an agent's pane, each built only by a validating constructor: `Command` (`NewMkreq`, `NewMkarch`, `NewMkplan`, `NewRunplan`, `NewRunpr`; its kind and argument read back by `Command.Parts`), `CommandKind` (named by `CommandKind.Name` and read from a name by `ParseCommandKind`), `Prompt` (`ClearSession`, `NewSession`, `SwitchModel`, `TypeCommand`), `Route`, `RouteKey`, `TaskID`, `IssueNumber`. The design: [0088 design §3.1.3](../../tasks/0088_chief_of_staff/02_architecture.md). |
| `internal/codexreview` | The checks behind the `codexreview` tool: `Run`, `Result`, `Kind`, `Protocol`. |
| `internal/coveragecompare` | The coverage comparison tool's core: `Run`, `Write`, `ExitCode`, `StopError`, and the input parsers. |
| `internal/fixprguard` | The deterministic checks for `/fixpr` and `/runpr`: `GitHub`, `NewGitHub`, `ParseReviewers`, `CheckState`, `Worktree.Baseline`. Usage: [fixprguard guide](../guides/fixprguard_guide.md). |
| `internal/mergepr` | Implements the `/mergepr` mechanics: prepares the drafting material after CI passes, squash-merges with `--match-head-commit`, and updates the local base and deletes the local head branch only while it still points at the merged head. Before CI is awaited and again right before the merge, it fetches `main` and runs the protocol-number check, which stops a PR that changed a tool's `Protocol` to anything but `origin/main`'s value plus one. Besides `New`, `Tool`, and `Discard`, it exports `Protocol`, `ShownCommands`, and `WorktreeProtocolTools`, and, for the editorial correction lines a squash body carries, `Drop`, `DropCase`, `Drops`, `ParseDrop`, `RejectedDrop`, `ParseRejectedDrop`, `RejectKind`, `RejectedLine`, `CorrectionMismatchError`, and `CorrectionsNotCarriedError`. Usage: [mergepr guide](../guides/mergepr_guide.md). |
| `internal/numreserve` | Decides a collision-free task or ADR number by creating a record tag on GitHub: `New`, `Tool.Reserve`, `Reservation`, `Space` (`ParseSpace`), `StopKind` (`StopKinds`, `Reason`), `StopError`, `ErrUnderTest`. It collects the numbers in use from `origin`, `origin/main`, the local branches, and the work tree, creates the tag with the REST API, and bounds itself. The design: [0100 design](../../tasks/0100_number_reservation/02_architecture.md). |

## Test-Only Packages

| Package | Responsibility |
|---|---|
| `internal/cmdrunner/testutil` | Test double `Fake` for `cmdrunner.InputRunner`: replays a scripted command sequence, records standard input, and runs a side-effect function per step. |
| `internal/codexreview/testutil` | The fake Codex CLI and clean `PATH`, `HOME`, and `TMPDIR` a `codexreview` test runs with: `Main`, `NewFake`, `Fake.Set`, `Starts`, `Args`, `Env`. |
| `internal/doctestutil` | The Markdown helpers the document guard tests share: `ReadDoc`, `ScanDocLines`, `FencedBlocks`, `SectionByHeading`, `ParseStructure`, `CompareStructures`, `FindTable`, `ProtocolFlags`, `ProtocolCallers`, `ProtocolTools`, and the readers of the commands a document writes (`CommandTexts`, `Invocations`, `PromptFree`, `RunsCommand`). |
| `internal/godepstest` | Lists a package's transitive dependencies with `go list -deps` on each target OS and checks them, failing rather than skipping when it cannot list: `Check`, `Root`, `List`, `PackagePath`, `MustPackagePath`, `Deps.Require`, `Deps.Forbid`. |
| `internal/llm/claude/testutil` | What the Anthropic integration test needs: `SettingsFrom` decides from the environment whether it runs, skips, or fails. |
| `internal/llm/deepseek/testutil` | What the DeepSeek integration tests share: `SettingsFrom` for the opt-in, and `RunMakeTarget`, `CheckChargedTarget`, `FirstLineIs` for the make targets that keep them out of `make test`. |
| `internal/llm/llmhttp/llmhttptest` | The test servers and loopback listeners shared by `internal/llm/llmhttp` and the LLM adapter tests: a request recorder, fixed-response and stalling servers, and a blackhole proxy; and `ParseDir`, which reads a package's Go source (including files only a `js/wasm` build compiles) to check which function calls which. |
| `internal/llm/openai/testutil` | What the OpenAI integration test needs: `SettingsFrom` decides from the environment whether it runs, skips, or fails. |
| `internal/llm/testutil` | Test doubles for `llm.LLMClient`: `FakeLLMClient` and a scripted client. |
| `internal/loopbacktest` | The rule a test endpoint must meet so a loopback constructor cannot be pointed at an external host: `ValidateURL`. |
| `internal/maketestutil` | Checks the make targets that run charged integration tests: `RunTarget`, `CheckChargedTarget`, `RecordedNames`, `FirstLineIs`. |
| `internal/pricing/testutil` | Formats a stage's estimated cost as one log line: `CostSummary`. |
| `internal/publisher/testutil` | Test double `FakePublisher` for the publisher and the pipeline. |
| `internal/transcript/testutil` | Test fixtures the other packages share: `FakeTranscriptSource` and fake `yt-dlp` scripts. |
| `internal/udstest` | Short directories and listeners for tests that use Unix domain sockets: `Dir`, `Listen`. |
| `internal/writer/testutil` | Test double `FakeArticleWriter` for `writer.ProgressWriter`, and `DefaultOptions`, the default `writer.Options` a run uses. |
