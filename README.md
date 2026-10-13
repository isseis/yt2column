# yt2column

A CLI tool that takes a YouTube video URL, generates a magazine-column-style
article from the video's transcript with an LLM, and either writes it as
Markdown to the `--out` path or posts it to the configured webhook with
`--slack`.

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
- `yt-dlp` on `PATH` (or set `YT2COLUMN_YTDLP_PATH`); see
  [Setting up yt-dlp](#setting-up-yt-dlp)
- An API key for the selected provider: `DEEPSEEK_API_KEY` for `deepseek`,
  `ANTHROPIC_API_KEY` for `claude`. When the review step uses a different
  provider, that provider's key is required too. `SLACK_WEBHOOK_URL` is
  required when `--slack` is used.

## Usage

```
yt2column [flags] <video URL>
```

Flags must come before the video URL. Exactly one of `--out` (write the
article to a file outside the cache directory) and `--slack` (post it to the
configured webhook) is required.

| Flag | Meaning |
|---|---|
| `--out <path>` | File to write the article to. Use either this or `--slack`. It must not exist, its parent directory must exist, and it must be outside the cache directory. |
| `--slack` | Post the article to the Slack-compatible Incoming Webhook (e.g. Mattermost) set in `SLACK_WEBHOOK_URL`. Use either this or `--out`. |
| `--refresh` | Ignore the cached transcript, run `yt-dlp` again, and replace the cache. |
| `--keep-cache` | Keep this video's cache after a successful run. |
| `--system-prompt <path>` | Read the system prompt template from this file instead of the built-in one. |
| `--user-prompt <path>` | Read the user prompt template from this file instead of the built-in one. |
| `--no-shorten` | Do not shorten an over-long body, and do not check the final length. |
| `--no-review` | Do not review the article against the source material. |
| `--review-log <path>` | Write the review step's list of changes to this file. Requires the review step; with `--out`, the path must differ from it. |
| `--shorten-system-prompt <path>` | Read the shorten system prompt template from this file instead of the built-in one. |
| `--shorten-user-prompt <path>` | Read the shorten user prompt template from this file instead of the built-in one. |
| `--review-system-prompt <path>` | Read the review system prompt template from this file instead of the built-in one. |
| `--review-user-prompt <path>` | Read the review user prompt template from this file instead of the built-in one. |
| `-h`, `--help` | Show this usage and exit. |

The article is written with file mode `0o644`. The output directory must allow
creating a hard link: publishing uses `link(2)`, so an output path on a
filesystem without hard-link support cannot be used, and a failed link leaves
the finished article in a temporary file next to the output path (the path is
printed to standard error).

### Exit codes

| Code | Meaning |
|---|---|
| `0` | The article was written to `--out` or posted to the configured webhook. Warnings about cache cleanup do not change this. `-h`/`--help` also exits `0`. |
| `1` | A run failure: fetching the transcript, generating the article, or publishing failed; another run held the cache directory; the run was interrupted; or the article was published but writing the review log failed. |
| `2` | A usage or configuration error: bad arguments or video URL, a rejected environment variable, or a stage that could not be built. |

## Post-generation steps

After the article is generated, two steps run by default.

1. **Shorten.** If the body is longer than 4,000 characters, the generation
   model shortens it toward a target of 3,600. If it is still over 4,000
   characters after shortening — or after the review step — the run fails with
   a length error and the article is neither written nor posted.
2. **Review.** The review model compares the article with the video's title,
   channel name, description, and transcript, and returns a list of corrections
   to claims with no basis in the material, meaning-changing paraphrases, wrong
   speaker attributions, copied speech-recognition errors, and sentences whose
   meaning cannot be made out. The writer applies the list to the article. The
   review step can use a provider and a model of its own, set in
   `YT2COLUMN_REVIEW_PROVIDER` and `YT2COLUMN_REVIEW_MODEL`; by default it uses
   the generation provider and model.

Both steps call the LLM (the shorten step only when the body is over the
limit), so the default uses up to two more calls, and more cost, than
generation alone. With `--no-shorten` the shorten step and the final length
check are skipped; with `--no-review` the review step is skipped.
`--no-shorten --no-review` restores the single-call behavior of earlier
versions, including the `--out` format.

An article's model lines start with the generation model and add one pair per
step that ran, in the order shorten, review. For unattended runs, pass
`--review-log <path>` to record, in a separate file, what the review step
changed; the log is written after the article is published, and if it cannot be
written the run exits `1` with a message saying the article is already
published.

The review step's configuration is validated even with `--no-review`, because
the configuration is independent of the flags: if `YT2COLUMN_REVIEW_PROVIDER`
selects a different provider, that provider's variables (its API key, and for
`claude` its effort) are still required.

## Cache and concurrency

Every run takes an exclusive lock on the cache directory, so runs that share a
cache directory are serialized: a second run fails immediately, without waiting,
while the first holds the lock. Different cache directories do not affect each
other.

If the CLI is killed with SIGKILL, `yt-dlp` may keep running. The lock stays held
until that `yt-dlp` exits, so until then the next run fails as a concurrent run
(the message names the lock file and suggests `lsof` on it); it succeeds once
the leftover `yt-dlp` has exited.

If the cache is corrupt (for example a truncated transcript or `info.json`), the
run fails with a message saying so. Run again with `--refresh` to fetch the
transcript again and replace the cache.

## Webhook publishing

With `--slack`, the article is posted to the Slack-compatible Incoming Webhook
in `SLACK_WEBHOOK_URL` instead of a file. Mattermost is the verified target; a
Slack Incoming Webhook is accepted but is best effort. The URL must parse as an
`https` URL with a host. A value that does not — for example
`https://hooks.slack.com/services/%zz`, `https://hooks.slack.com/%`, or a value
ending in a control character — is rejected as a configuration error with exit
code `2`, and because the variable is validated whenever it is set, this also
stops an `--out` run.

The article is posted as Markdown. A post longer than 16,383 code points — the
limit Mattermost counts — is split into several messages, each at most 16,383
code points and beginning with a `(k/N)` marker. An article that cannot be
split — because a fragment would be only whitespace, or because it would need
more than ten messages — is rejected before anything is sent. Each message is
sent with
`silent` set, so Mattermost shows it without a notification, an unread marker,
or a `New Messages` line; on a Mattermost version that predates `silent`,
notifications can still appear.

Before anything is sent, the article is rejected if it contains syntax a server
would turn into a mention or rewrite: an `@` followed by a name character
(which also rejects YouTube handles, email addresses, and words such as
`@Override`), a `<!` or `<@` sequence, an HTML character reference such as
`&#64;`, or a `<a|b>` link. Such an article is not posted at all; run with
`--out` to inspect what the model generated.

If a post fails partway through a split article, the messages already sent stay
in the channel. There is no retry or resume: running again generates a new
article and posts it from the first message, so the channel can end up with part
of the old article and all of the new one.

## Setting up yt-dlp

yt2column fetches subtitles by running `yt-dlp`, which needs to reach YouTube
reliably. Beyond `yt-dlp` itself that means two optional pieces; without them
`yt-dlp` warns about them, and YouTube is more likely to answer with HTTP 429
(Too Many Requests):

- a JavaScript runtime ([deno](https://deno.com/) is the one `yt-dlp` enables
  by default), and
- [curl_cffi](https://github.com/lexiforest/curl_cffi) for browser
  impersonation.

On macOS the Homebrew formula brings both (it depends on `deno` and bundles
`curl_cffi`):

```sh
brew install yt-dlp
command -v yt-dlp                   # must be the Homebrew one, not an older copy earlier on PATH
yt-dlp --list-impersonate-targets   # lists targets whose Source is curl_cffi
```

If another `yt-dlp` comes first on `PATH`, remove it or point
`YT2COLUMN_YTDLP_PATH` at the one to use.

On other platforms, see the yt-dlp
[installation](https://github.com/yt-dlp/yt-dlp/wiki/Installation),
[EJS (JavaScript runtime)](https://github.com/yt-dlp/yt-dlp/wiki/EJS) and
[impersonation](https://github.com/yt-dlp/yt-dlp#impersonation) documentation.

## Configuration

| Variable | Description | When unset |
|---|---|---|
| `YT2COLUMN_LLM_PROVIDER` | LLM provider; `deepseek` or `claude` | `deepseek` |
| `YT2COLUMN_MODEL` | LLM model name (e.g. `deepseek-flash`, `claude-opus-5-5`) | Required (an error) |
| `YT2COLUMN_REVIEW_PROVIDER` | Review step's LLM provider; same values as `YT2COLUMN_LLM_PROVIDER` | The generation provider |
| `YT2COLUMN_REVIEW_MODEL` | Review step's LLM model name (required when `YT2COLUMN_REVIEW_PROVIDER` is set) | The generation model |
| `DEEPSEEK_API_KEY` | DeepSeek API key | Required when the generation or review provider is `deepseek` (an error) |
| `ANTHROPIC_API_KEY` | Anthropic API key | Required when the generation or review provider is `claude` (an error) |
| `YT2COLUMN_CLAUDE_EFFORT` | Claude effort; `low`, `medium`, `high`, `xhigh`, or `max` | Required when the generation or review provider is `claude` (an error) |
| `ANTHROPIC_WORKSPACE_ID` | Anthropic workspace ID, for an API key not bound to a workspace | No workspace ID is sent |
| `SLACK_WEBHOOK_URL` | Slack-compatible Incoming Webhook URL (Mattermost is the verified target) | Required with `--slack`; optional with `--out` (no value) |
| `YT2COLUMN_CACHE_DIR` | Cache directory for subtitles and video metadata | `$HOME/Library/Caches/yt2column` on macOS; `$XDG_CACHE_HOME/yt2column` or `$HOME/.cache/yt2column` on other Unix |
| `YT2COLUMN_YTDLP_PATH` | Path to `yt-dlp` | `yt-dlp` on `PATH` |

The value of an environment variable is never trimmed or otherwise corrected;
an empty value is treated as set and rejected, not as unset.

Setting `YT2COLUMN_REVIEW_PROVIDER` requires `YT2COLUMN_REVIEW_MODEL`; setting
only `YT2COLUMN_REVIEW_MODEL` keeps the generation provider. With both unset,
the review step reuses the generation provider and model.

Only the variables of the selected providers are read and checked: a provider's
variables are read when the generation or the review provider is that provider.
The other provider's variables are ignored whatever their value, so exporting
`ANTHROPIC_API_KEY=` for another tool does not stop a run that uses only
`deepseek`; when `YT2COLUMN_LLM_PROVIDER` itself is invalid and
`YT2COLUMN_REVIEW_PROVIDER` is unset, no provider-specific variable is checked,
but errors for common variables such as `YT2COLUMN_MODEL` are still reported.

With `claude`, `max` is accepted but in this use case it spends the whole output
budget on reasoning and returns no article text, so the run fails as truncated.
Use `low` through `xhigh` for column generation.

## Development

```sh
make build   # build/yt2column
make test    # unit tests (no network, no yt-dlp)
make lint    # golangci-lint (pinned version)
make fmt     # gofumpt on changed files
make test-integration  # real yt-dlp + network; see below
make test-integration-deepseek  # real DeepSeek API; see below
make test-integration-claude  # real Anthropic API; see below
make test-integration-cli  # the CLI with the real DeepSeek API; see below
make test-integration-slack  # posts fixed articles to a real Mattermost test Webhook; see below
make test-integration-cli-slack  # the CLI with the real DeepSeek API and a real Webhook; see below
```

Development follows a requirements → architecture → implementation-plan process
with explicit acceptance criteria. See [CLAUDE.md](CLAUDE.md) and
[docs/dev/developer_guide/requirements_process.md](docs/dev/developer_guide/requirements_process.md).
Design documents are written in Japanese; start with
[docs/dev/project_overview.md](docs/dev/project_overview.md).

### Browser extension

A Chrome/Brave extension (Manifest V3) lives in `extension/`; it is written in
TypeScript and built with Node.js. It is not published to a store — you build it
and load the build as an unpacked extension. At this point it never sends
anything: it only collects a selection, the tab URL, and the tab title, and
shows them.

Node.js 24 is required, and every `ext-*` target refuses to run when
`node --version` does not match `extension/.node-version`. The version file sits
inside `extension/`, so a version manager that reads the current directory (such
as [fnm](https://github.com/Schniz/fnm)) must be activated from there; run the
targets from the repository root with that version still active:

```sh
cd extension && fnm use && cd ..
make ext-install   # install the pinned dev dependencies (npm ci; needs network)
make ext-build     # build extension/dist/
make ext-check     # typecheck, lint, format check, tests, then build
```

Load the build in Chrome or Brave:

1. Open `chrome://extensions/` (Brave: `brave://extensions/`).
2. Turn on **Developer mode**.
3. Click **Load unpacked** and select the `extension/dist` directory.

The extension ID is fixed to `clfmbbcdpnjcefbdihdoahomaifbabkk`, so it is the
same on every machine and from every directory.

There are two ways to launch the extension, and both collect the same three
values:

- **Context menu:** on a video page, select the transcript text, right-click,
  and choose the extension's item. The result opens in a window.
- **Toolbar icon:** click the icon to open the popup, which shows what would be
  collected and why nothing would be collected when the page or selection does
  not qualify. The popup closes when you click elsewhere.

To see what was collected, open the console before launching: the service
worker's console via the **service worker** link on `chrome://extensions/`, and
the popup's console via right-click → **Inspect**.

### Integration test

`make test-integration` runs the real `yt-dlp` against the network, so it needs
`yt-dlp` set up as in [Setting up yt-dlp](#setting-up-yt-dlp) and on `PATH`.
It fetches a real video (by default
`https://www.youtube.com/watch?v=EQCUZyB4DqE`) and sends YouTube two requests
two minutes apart, so a run takes a few minutes. Override the video with
`make test-integration YT2COLUMN_TEST_VIDEO_URL=... YT2COLUMN_TEST_VIDEO_ID=...`
(an empty `YT2COLUMN_TEST_VIDEO_ID` skips the video ID check). Leave some
time between runs to stay clear of YouTube's rate limit.

### DeepSeek integration test

`make test-integration-deepseek` calls the real DeepSeek API, so it incurs
charges. It needs `YT2COLUMN_TEST_DEEPSEEK_API_KEY` set to a DeepSeek API key
for testing; this is separate from the production `DEEPSEEK_API_KEY`, which the
test never reads. A missing or empty `YT2COLUMN_TEST_DEEPSEEK_API_KEY` skips
the test with a message naming the variable, so setting only `DEEPSEEK_API_KEY`
does not run it. The model comes from `YT2COLUMN_MODEL`, defaulting to
`deepseek-flash` when that variable is undefined (an empty value is passed
through and fails the test). The target exports the opt-in variable
`YT2COLUMN_DEEPSEEK_INTEGRATION=1` for the integration test alone; without it
the test skips with a message naming the variable, so a plain
`go test -tags integration` or an IDE run does not call the API. A run makes two
generations — an ordinary one and one truncated by `MaxOutputTokens` — each
bounded by a 15-minute timeout, so it can take a few minutes under API load.

### Claude integration test

`make test-integration-claude` calls the real Anthropic Messages API, so it
incurs charges. It needs `YT2COLUMN_TEST_ANTHROPIC_API_KEY` set to an Anthropic
API key for testing; this is separate from the production `ANTHROPIC_API_KEY`,
which the test never reads. A missing or empty `YT2COLUMN_TEST_ANTHROPIC_API_KEY`
skips the test with a message naming the variable, so setting only
`ANTHROPIC_API_KEY` does not run it. The model and effort come from
`YT2COLUMN_TEST_CLAUDE_MODEL` and `YT2COLUMN_TEST_CLAUDE_EFFORT`, defaulting to
`claude-haiku-5-5` and `low` when undefined (an empty value is passed through
and fails the test). A workspace ID is read from
`YT2COLUMN_TEST_ANTHROPIC_WORKSPACE_ID` when set, never from the production
`ANTHROPIC_WORKSPACE_ID`; unset means no workspace ID is sent, while an empty or
malformed value fails the test. The target exports the opt-in variable
`YT2COLUMN_CLAUDE_INTEGRATION=1` for the integration test alone; without it the
test skips with a message naming the variable, so a plain
`go test -tags integration` or an IDE run does not call the API. A run makes two
generations — an ordinary one and one truncated by `MaxOutputTokens` — each
bounded by a 5-minute timeout, so the target's `-timeout` is 15 minutes.

### CLI integration test

`make test-integration-cli` runs the CLI's own assembly from a seeded transcript
cache to the `--out` file, calling the real DeepSeek API (so it incurs charges)
and never starting `yt-dlp`. Like the DeepSeek integration test it reads
`YT2COLUMN_TEST_DEEPSEEK_API_KEY`, defaults `YT2COLUMN_MODEL` to `deepseek-flash`
when undefined, and exports its opt-in variable `YT2COLUMN_CLI_INTEGRATION=1`
for its target alone; unlike it, a missing test API key fails the test rather
than skipping it, so an opted-in run cannot pass without calling the API.

### Webhook integration test

`make test-integration-slack` posts two fixed articles (no LLM call) to the real
Incoming Webhook of a Mattermost test channel, so it needs
`YT2COLUMN_TEST_SLACK_WEBHOOK_URL` set to that channel's Webhook URL. This is
separate from the production `SLACK_WEBHOOK_URL`, which the test never reads.
The target exports the opt-in variable `YT2COLUMN_SLACK_INTEGRATION=1` for the
integration test alone; without it the test skips with a message naming the
variable, so a plain `go test -tags integration` or an IDE run does not post. A
missing or invalid test Webhook URL, or a `GODEBUG` that enables `http2debug`,
fails the test rather than skipping it and posts nothing. Message text never
includes the URL, and failure messages redact the URL, its path, query, and
userinfo, and the last eight characters of the URL; a transport failure can
still name the destination host and port, which are not treated as secret.

### CLI webhook integration test

`make test-integration-cli-slack` runs the CLI's own assembly with `--slack`,
generating the article with the real DeepSeek API (so it incurs charges) and
posting it to the real test Webhook; it never starts `yt-dlp`. It needs both
`YT2COLUMN_TEST_DEEPSEEK_API_KEY` and `YT2COLUMN_TEST_SLACK_WEBHOOK_URL` and
fails when either is missing. Like the other CLI targets it defaults
`YT2COLUMN_MODEL` to `deepseek-flash` when undefined, and it exports
`YT2COLUMN_CLI_SLACK_INTEGRATION=1` for its target alone.
