# yt2column

A CLI tool that takes a YouTube video URL, generates a magazine-column-style
article from the video's transcript with an LLM, and posts it as Markdown to
Slack (or other destinations) via a Webhook.

```
URL → TranscriptSource → Transcript → ArticleWriter → Article → Publisher
```

- Subtitles are fetched with [yt-dlp](https://github.com/yt-dlp/yt-dlp).
- The initial LLM provider is DeepSeek (OpenAI-compatible API, called with the
  Go standard library); providers are pluggable.
- Designed for local execution.

> **Status:** early development — no release yet.

## Requirements

- Go (see `go.mod`)
- `yt-dlp` on `PATH` (or set `YT2COLUMN_YTDLP_PATH`)
- A DeepSeek API key and a Slack Incoming Webhook URL

## Configuration

| Variable | Description |
|---|---|
| `YT2COLUMN_LLM_PROVIDER` | `deepseek` (default) |
| `YT2COLUMN_MODEL` | LLM model name (e.g. `deepseek-flash`) |
| `DEEPSEEK_API_KEY` | DeepSeek API key |
| `SLACK_WEBHOOK_URL` | Slack Incoming Webhook URL |
| `YT2COLUMN_CACHE_DIR` | Cache directory for subtitles and video metadata |
| `YT2COLUMN_YTDLP_PATH` | Path to `yt-dlp` (defaults to the one on `PATH`) |

## Development

```sh
make build   # build/yt2column
make test    # unit tests (no network, no yt-dlp)
make lint    # golangci-lint (pinned version)
make fmt     # gofumpt on changed files
```

Development follows a requirements → architecture → implementation-plan process
with explicit acceptance criteria. See [CLAUDE.md](CLAUDE.md) and
[docs/dev/developer_guide/requirements_process.md](docs/dev/developer_guide/requirements_process.md).
Design documents are written in Japanese; start with
[docs/dev/project_overview.md](docs/dev/project_overview.md).
