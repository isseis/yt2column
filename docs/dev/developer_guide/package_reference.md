# Package Reference

This document lists the packages under `cmd/` and `internal/`, plus the
`prompts` package at the repository root, with the responsibility of each. Update it in the same commit that adds, removes, or
changes the responsibility of a package.

The planned layout and the responsibility of each not-yet-created package are
described in [project_overview.md](../project_overview.md) ("想定ディレクトリ構成" and
"パイプライン").

| Package | Responsibility |
|---|---|
| `cmd/mergepr` | Entry point of the `/mergepr` developer tool: prepares a PR for message drafting, then merges and cleans up after the user approves the message |
| `internal/mergepr` | Implements the `/mergepr` mechanics: prepares the drafting material after CI passes, squash-merges with `--match-head-commit`, and updates the local base and deletes the local head branch only while it still points at the merged head |
| `internal/secret` | Holds a secret (API key, Webhook URL) and guarantees it never appears in `fmt`, `log/slog`, or JSON output |
| `internal/strictjson` | Extracts values from JSON documents that passed strict checks: rejects the byte sequences and structural shapes `encoding/json` would silently repair (invalid UTF-8, unpaired surrogate escapes, trailing data, duplicate or `null` consumed members). Shared by `internal/transcript` and `internal/llm/deepseek` |
| `internal/nilcheck` | Reports whether a value is nil, including a typed nil held in an interface (`IsNil`). Used by `internal/pipeline` to reject unset stages and by `internal/writer` to reject a missing `LLMClient` |
| `internal/transcript` | Defines the transcript stage (`Transcript`/`Segment`, `TranscriptSource`) and provides its `YtDlpSource` implementation: video URL validation (`NormalizedVideoURL` validates a video ID and builds the normalized URL from it), strict json3 and info.json parsers, the `yt-dlp` execution boundary (a shell-free command executor with a fixed environment allowlist and a capped, drained standard error output), the per-video cache (`Fetch`, `RemoveCache`, `PruneCache`), and the stage's sentinel errors |
| `internal/llm` | Defines the provider-independent `LLMClient` interface and its `GenerateRequest`/`GenerateResponse` types, the provider-common sentinel errors (`ErrInvalidRequest`/`ErrTruncated`/`ErrUnexpectedFinishReason`/`ErrEmptyResponse`), and `GenerateRequest.Validate` |
| `internal/llm/deepseek` | Implements `llm.LLMClient` against the DeepSeek Chat Completions API: construction validation, a fixed production endpoint that only the test helper replaces (with a loopback URL), redirects not followed, and strict validation of the response body before anything is returned |
| `internal/writer` | Defines the article-writing stage: `Article` and the `ArticleWriter` interface, and implements it. `New` rejects a nil or typed-nil `LLMClient`, takes optional per-template override paths in `Options`, reads each override file once (following symbolic links, refusing non-regular files without blocking, capped in size), and checks the embedded and override templates alike: size, UTF-8, not blank, parses, no `define`/`block`, and only allowlisted syntax (the four template data fields and the builtin functions that cannot grow a string) |
| `prompts` | Embeds the default system and user prompt templates and returns them with `System` and `User`; `README.md` documents the template contract for override-file authors |
| `internal/publisher` | Defines the publishing stage: the `Publisher` interface |
| `internal/pipeline` | Runs the three stages in order: `Pipeline`, `Stage`, `StageError`, `ErrNilStage` |
| `internal/transcript/testutil` | Test double (`FakeTranscriptSource`) for `transcript.TranscriptSource`; built only with `-tags test` |
| `internal/llm/testutil` | Test double (`FakeLLMClient`) for `llm.LLMClient`; built only with `-tags test` |
| `internal/writer/testutil` | Test double (`FakeArticleWriter`) for `writer.ArticleWriter`; built only with `-tags test` |
| `internal/publisher/testutil` | Test double (`FakePublisher`) for `publisher.Publisher`; built only with `-tags test` |
