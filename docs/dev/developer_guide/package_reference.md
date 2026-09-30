# Package Reference

This document lists the packages under `cmd/` and `internal/` and the
responsibility of each. Update it in the same commit that adds, removes, or
changes the responsibility of a package.

The planned layout and the responsibility of each not-yet-created package are
described in [project_overview.md](../project_overview.md) ("想定ディレクトリ構成" and
"パイプライン").

| Package | Responsibility |
|---|---|
| `internal/secret` | Holds a secret (API key, Webhook URL) and guarantees it never appears in `fmt`, `log/slog`, or JSON output |
| `internal/transcript` | Defines the transcript stage (`Transcript`/`Segment`, `TranscriptSource`) and provides video URL validation, strict json3 and info.json parsers, and the stage's sentinel errors |
| `internal/llm` | Defines the provider-independent `LLMClient` interface and its `GenerateRequest`/`GenerateResponse` types |
| `internal/writer` | Defines the article-writing stage: `Article` and the `ArticleWriter` interface |
| `internal/publisher` | Defines the publishing stage: the `Publisher` interface |
| `internal/pipeline` | Runs the three stages in order: `Pipeline`, `Stage`, `StageError`, `ErrNilStage` |
| `internal/transcript/testutil` | Test double (`FakeTranscriptSource`) for `transcript.TranscriptSource`; built only with `-tags test` |
| `internal/llm/testutil` | Test double (`FakeLLMClient`) for `llm.LLMClient`; built only with `-tags test` |
| `internal/writer/testutil` | Test double (`FakeArticleWriter`) for `writer.ArticleWriter`; built only with `-tags test` |
| `internal/publisher/testutil` | Test double (`FakePublisher`) for `publisher.Publisher`; built only with `-tags test` |
