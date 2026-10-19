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
| `--artifacts <path>` | Save each stage's output (the generated article, the shortened article, and the review list with the applied article) to this directory, and reuse it to resume. The directory must exist and be outside the cache directory. |
| `--after <stage>` | Run only the stages after `stage` (`generation`, `shorten`, or `review`), reusing that stage's saved output from `--artifacts`. Requires `--artifacts`; `--after review` does not accept `--review-log`. |
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
| `1` | A run failure: fetching the transcript, generating the article, or publishing failed; another run held the cache directory; the run was interrupted; an `--artifacts` directory was unusable or a saved artifact was missing, broken, or from another run; or the article was published but writing the review log failed. |
| `2` | A usage or configuration error: bad arguments or video URL, a rejected environment variable, a forbidden `--artifacts`/`--after` combination, or a stage that could not be built. |

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

With `--artifacts <dir>`, a run also saves each stage's output — the generated
article, the shortened article, and the review list with the applied article —
to its own file (`<video ID>.generation.json`, `.shorten.json`, `.review.json`)
as the stage completes and before the article is published, so a later stage's
failure still leaves the earlier stages on disk. `--after <stage>` restarts from
a saved stage and runs only the stages after it, reusing the saved output
instead of calling the LLM for the skipped stages; `--after review` calls no LLM
and only checks the final length and publishes. A resumed run saves the stages
it does execute, so the `--artifacts` directory must be writable. The directory
lives outside the cache directory and is still there on the next run, which is
what makes the resume work; see [Cache and concurrency](#cache-and-concurrency).

The review step's configuration is validated even with `--no-review`, because
the configuration is independent of the flags: if `YT2COLUMN_REVIEW_PROVIDER`
selects a different provider, that provider's variables (its API key, and for
`claude` its effort) are still required. A resumed run validates only the
settings of the stages it executes, so `--after review` requires no LLM
settings at all.

## Token usage

Every run reports the token usage the LLM API returned, one line per stage, on
standard error. A successful run writes the lines after its other summary lines;
a failed run writes them before the line that names the failure. A stage that
neither ran nor reused an artifact writes no line. The line is:

```text
yt2column: tokens: <stage> <origin> known input=<n> cache_read=<n> cache_write=<n> output=<n> <model-field>
yt2column: tokens: <stage> <origin> unknown reason=<reason> <model-field>
```

- `<stage>` is `generation`, `shorten`, or `review`.
- `<origin>` is `run` when this run called the stage's LLM, or `reused` when the
  stage came from a `--after` artifact.
- `known` means the four counts are known; they are, in order, `input=`
  (neither read from nor written to the prompt cache), `cache_read=` (read from
  the prompt cache), `cache_write=` (written to the prompt cache; always 0 for
  DeepSeek, which has no such category), and `output=` (the output, reasoning
  and thinking included). For a `reused` stage they are the counts the saved
  artifact recorded.
- `unknown` means the usage could not be read or did not meet the rules. No
  number is written, and `reason=` gives one space-free word: `not-read` (no
  usage could be read: a call that failed before its usage was read, or a client
  that reports none), `recorded-unknown` (a saved artifact stored the stage as
  unknown), `invalid-rule` (a rule outside the set of known rules), or, for the
  usage member or one of the consumed members that broke a rule, `usage:missing`,
  `usage:duplicate`, `usage:not-an-object`,
  `usage:not-a-non-negative-integer` and their `usage.<member>:missing`,
  `usage.<member>:duplicate`, `usage.<member>:not-a-non-negative-integer` forms.
- `<model-field>` is `model=<model>`, the model name, or the fixed word
  `nomodel` when the stage reported no model that passed the model-name rules.
  The model name is escaped the way `redact.Line` escapes every standard-error
  line, so a `"` or a `\` in it is escaped; to recover it, wrap everything after
  `model=` in double quotes and pass the result to `strconv.Unquote`. A model
  name that contains a configured secret (or its last eight characters) is
  replaced by the redaction marker and cannot be recovered.

For example:

```text
yt2column: tokens: generation reused known input=46 cache_read=0 cache_write=0 output=85 model=deepseek-flash
yt2column: tokens: shorten run known input=1800 cache_read=0 cache_write=0 output=2400 model=deepseek-flash
yt2column: tokens: review run unknown reason=usage.output_tokens:not-a-non-negative-integer model=claude-opus-5-5
yt2column: tokens: generation run unknown reason=not-read nomodel
```

An invalid `usage` alone does not fail the run: the call still succeeds, and
whether the article is published and the exit code is 0 depend on the rest of
the processing. A failed call fails the run (the exit code is non-zero), but its
line is not always `unknown`: a truncation, an unexpected finish reason, or an
empty response still carries the rejected response's model and usage, so the
line can be `known`. `reason=not-read` is written only when no usage could be
read.

A run that ends before it calls any LLM (a usage or configuration error, a
transcript-fetch failure, a rejected `--after` input, an unusable `--artifacts`
directory, or a stage that could not be built) writes no token line. A run that
is stopped without writing its summary (a second signal, `kill -9`, or a panic)
writes none either.

The counts are also stored, as the `usage` element, in a `--artifacts` file. A
stage's call time is stored next to it, as the `call_time` element: the moment
the stage's LLM call returned, in UTC and whole seconds. The artifact format is
version 3: a version 1 or version 2 file is rejected on resume, so after
updating, use a new `--artifacts` directory or move the old one away.

## Estimated cost

Every run that calls or reuses at least one LLM stage also reports an estimated
cost on standard error: one line per stage, then two total lines. A successful
run writes them after the other summary lines; a failed run writes them before
the line that names the failure. A stage that neither ran nor reused an artifact
writes no cost line, and a run that writes no token line writes no cost line
either. The lines are:

```text
yt2column: cost: <stage> <origin> known usd=<amount> <basis> at=<time>
yt2column: cost: <stage> <origin> unknown reason=<reason> at=<time>
yt2column: cost-total: <total> known usd=<amount>
yt2column: cost-total: <total> unknown
```

- `<stage>` and `<origin>` are the same words as the token-usage line:
  `generation`, `shorten`, or `review`, and `run` or `reused`.
- `known` is followed by the amount as `usd=` and six decimals, then `basis`,
  the rate basis the estimate applied: `flat` (the model has one rate),
  `off-peak` or `peak` (the two windows of DeepSeek's time-of-day rate), or
  `short-prompt` or `long-prompt` (Claude Haiku's two prompt-length tiers).
  `invalid` in this position, like the one under `unknown`, marks a broken
  invariant and does not occur in a correct build.
- `unknown` means the cost could not be estimated. No number is written, and
  `reason` gives one word: `usage-unknown` (the stage's token counts are
  unknown), `no-model` (the stage reported no model name), or `unpriced-model`
  (the model name is not in the rate table). `invalid` means the CLI broke an
  invariant of the pricing package; it does not occur in a correct build.
- `at` is the call time the estimate used, in UTC as `YYYY-MM-DDTHH:MM:SSZ`. A
  reused stage shows the time its artifact recorded; `none` marks a broken
  invariant and does not occur in a correct build.
- The `cost-total` lines are two totals: `run` is this run's cost, from the
  stages this run called, and `article` is the whole article's cost, from those
  stages plus the ones reused from `--after`. A total is `unknown` when any
  stage in its set is unknown, and the `article` total is also `unknown` when
  the generation stage is neither run nor reused (an article always has a
  generation stage). Without `--after` the two totals are equal. The totals
  round the exact sum of the stages, not the already-rounded stage amounts, so a
  total can differ from the sum of the printed stage lines in the last digit.

For example:

```text
yt2column: cost: generation reused known usd=0.000158 peak at=2026-10-16T01:12:09Z
yt2column: cost: shorten run known usd=0.001710 off-peak at=2026-10-16T04:02:41Z
yt2column: cost: review run unknown reason=unpriced-model at=2026-10-16T04:03:15Z
yt2column: cost-total: run unknown
yt2column: cost-total: article unknown
```

Both the `--out` file and the Slack post carry the article's cost as a line of
the heading metadata, with four decimals and the word `USD`, or `unknown`:

```text
- Estimated cost: 0.0023 USD
- Estimated cost: unknown
```

The estimate is an estimate, not the billed amount. It can differ from the
provider's invoice:

- DeepSeek's peak window is a weekly rule in UTC: the alternate rate applies on
  Monday through Friday during 01:00-04:00 and 06:00-10:00 UTC. The CLI judges
  the window from its own clock in UTC and does not know Chinese public
  holidays, so a call on a holiday is billed at the off-peak rate but estimated
  as peak: the estimate leans high.
- A call that crosses a window boundary can be judged at a different minute than
  the provider billed it, because the CLI's time is when the response was read
  and validated, and the provider's billing time is not documented.
- A CLI on a machine whose clock is wrong judges the window wrongly.
- The rate table can be stale, for example when the provider changes a rate or
  when the `deepseek-flash` alias starts to point at a model with different
  rates. Each entry records its source and checked date, and the update
  procedure is a developer document.

Check the actual charge in the provider's console. A reused stage is priced with
the rate table of the CLI that resumes, not the one that saved the artifact, so
a run that resumes after a rate change reports a different article cost for the
same saved text: the artifact records the call time, not the rate.

A Slack post from the localhost server cannot show why an unknown cost is
unknown, because the server's logs and HTTP responses do not change. To find the
reason, run the CLI with the same model setting and read the `reason` word on
its cost line.

## Cache and concurrency

Every run takes an exclusive lock on the cache directory, so runs that share a
cache directory are serialized: a second run fails immediately, without waiting,
while the first holds the lock. Different cache directories do not affect each
other.

The artifacts saved with `--artifacts` live outside the cache directory, so
removing the cache does not remove them and they are still there for a resume.
The CLI neither prunes nor deletes them; delete the directory yourself when it
is no longer needed.

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

## Localhost server

`yt2column-server` accepts the transcript the browser extension collects and
posts the generated article to the same Webhook as the CLI. `make build` builds
both `build/yt2column` and `build/yt2column-server`.

It reads the same LLM variables as the CLI and requires `SLACK_WEBHOOK_URL`. It
never calls `yt-dlp` and reads neither `YT2COLUMN_CACHE_DIR` nor
`YT2COLUMN_YTDLP_PATH`. It listens on a Unix domain socket only, never on a TCP
port, at a path derived from `HOME` alone:

- macOS: `$HOME/Library/Application Support/yt2column/run/server.sock`
- other Unix: `$HOME/.local/state/yt2column/run/server.sock`

No other variable (such as `XDG_STATE_HOME` or `TMPDIR`) changes the path. The
server needs no shared token: the socket directory has mode `0700` and the
socket mode `0600`, so only processes running as your user can connect, and the
server trusts them. It checks only the request path and method, not the
`Host`, `Origin`, `Content-Type`, or `Authorization` headers.

### Running the server

Start it with the provider and Webhook variables set:

```sh
./build/yt2column-server 2>>server.log
```

The server writes nothing to standard output; the logs go to standard error,
so `2>>server.log` keeps a record. The first log line names the socket. Every
request and job log line carries the `requestId` the response returns, so you
can find a request's log with `grep <requestId> server.log`.

On start the server creates the socket directory (`run`) with mode `0700` when
it is missing, takes the start lock (the file `.yt2column.lock` in the socket
directory, which stays), removes a stale socket left by a server that was
killed, and creates the socket with mode `0600`. Only one server per `HOME`
(normally one per user) can run: the start lock is held until the server has stopped and its in-flight jobs
have finished. The server repairs nothing: a socket directory that is not
yours, is a symbolic link, or does not have exactly mode `0700`, and a name at
the socket path that is not a socket, stop the start until you fix them by
hand.

The socket path must fit the operating system's limit (107 bytes on Linux, 103
on macOS and other Unix). On macOS the part after `HOME` is 54 bytes, so `HOME` can be at
most 49 bytes; a longer one stops the start with exit code `2`.

Exit codes:

| Code | Meaning |
|---|---|
| `0` | The server stopped after a signal. |
| `1` | Preparing the socket directory, taking the start lock, creating the socket, or serving failed (for example, another server is already running). |
| `2` | The server was given arguments, or the configuration (including `HOME` and the socket path derived from it) or building the components failed before listening. |

### Stopping the server

The server logs to standard error. The first `SIGINT`, `SIGTERM`, or `SIGHUP`
(one ignored at startup is not subscribed) stops accepting connections, removes
the socket, and lets the in-flight jobs finish, which can take up to about 45
minutes (three LLM calls of 15 minutes each); a second signal exits at once.
Until the jobs have finished, a new server refuses to start.

### Checking the server with curl

Write a request body to `request.json` (a video URL and the copied transcript):

```json
{"videoUrl":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","transcript":"0:00\nHello\n"}
```

Send it to the socket. The path contains a space on macOS, so keep it in
double quotes:

```sh
socket="$HOME/.local/state/yt2column/run/server.sock"
if [ "$(uname -s)" = Darwin ]; then socket="$HOME/Library/Application Support/yt2column/run/server.sock"; fi
curl -sS --unix-socket "$socket" -X POST http://localhost/articles \
  -H 'Content-Type: application/json' \
  --data-binary @request.json
```

An accepted request answers `202` with `{"status":"accepted","requestId":"…"}`.
A rejected request answers a `4xx` or `5xx` with
`{"status":"rejected","reason":"…","requestId":"…"}`, where `reason` is one of:

| reason | status code |
|---|---|
| `not_found` | 404 |
| `method_not_allowed` | 405 |
| `too_many_requests` | 429 |
| `body_too_large` | 413 |
| `invalid_body` | 400 |
| `invalid_video_url` | 400 |
| `invalid_transcript` | 400 |
| `internal_error` | 500 |

`not_found` and `method_not_allowed` answer a wrong path or method; they start
no job and post no failure notice.

### Troubleshooting

- **The server is not running.** `curl` reports `Couldn't connect to server`
  (its message names `localhost port 80`, the URL's host and default port; the
  connection itself goes to the socket). Start the server as above; the first
  log line names the socket it listens on.
- **`another server holds the socket directory lock`.** Another server is
  running, or a stopped one is still waiting for its in-flight jobs. The
  message names the lock file; `lsof` on it shows which process holds it.
- **`another process accepts connections on the socket path`.** A process other
  than a server listens on the socket path. Stop it, or check the file at that
  path.
- **`... nothing was repaired or removed, so fix it by hand`.** The socket
  directory, its parent, or the name at the socket path is not in the accepted
  form (the message names the path and the reason). Fix it by hand, for
  example by restoring mode `0700` on the socket directory, and start again.
- **`HOME: the socket path derived from HOME exceeds the operating system's
  limit`.** `HOME` is too long for the socket path; see the limit above.

### Upgrading from the shared-token version

- Stop the old server before starting this one. The old server listened on a
  TCP port and does not take the start lock, so the new one cannot detect it.
- The server no longer reads the shared token file. It can be deleted:
  - macOS: `$HOME/Library/Application Support/yt2column/server-token`
  - other Unix: `${XDG_CONFIG_HOME:-$HOME/.config}/yt2column/server-token`
- The server no longer reads the variable that chose its port; it can be
  removed from your environment.

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

`yt2column-server` reads the LLM variables and requires `SLACK_WEBHOOK_URL`; it
reads neither `YT2COLUMN_CACHE_DIR` nor `YT2COLUMN_YTDLP_PATH`, because it never
calls `yt-dlp`. It derives its socket path from `HOME` alone (see
[Localhost server](#localhost-server)).

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
make build   # build both binaries into build/
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

### Running a separate server for development

The socket path depends only on `HOME`, and one server per `HOME` can run. To
run a server built from a worktree beside your own, give it its own `HOME` in a
short directory under `/tmp` (on macOS, the default directory of `mktemp -d`
makes the socket path too long), and send to its socket:

```sh
dev_home="$(mktemp -d /tmp/y2c.XXXXXX)"
HOME="$dev_home" ./build/yt2column-server 2>>"$dev_home/server.log" &
server_pid=$!
socket="$dev_home/.local/state/yt2column/run/server.sock"
if [ "$(uname -s)" = Darwin ]; then socket="$dev_home/Library/Application Support/yt2column/run/server.sock"; fi
until [ -S "$socket" ] || ! kill -0 "$server_pid" 2>/dev/null; do sleep 0.1; done
if [ -S "$socket" ]; then
  curl -sS --unix-socket "$socket" -X GET http://localhost/articles
else
  echo "server exited at startup; $dev_home/server.log:" >&2; cat "$dev_home/server.log" >&2
fi
kill "$server_pid" 2>/dev/null; wait "$server_pid"; rm -rf "$dev_home"
```

The loop stops waiting when the server exits at startup (a configuration error,
or another server holding the start lock for that `HOME`); its log is then
printed instead of sending the request, because the last line removes it. The
last line works whether or not the server is still running.

Each worktree's server with a different `HOME` runs at the same time as the
others. The server still reads the LLM and Webhook variables from your
environment: with the real ones, an accepted request generates an article and
posts it. With fake values (any API key, and an `https` Webhook URL such as
`https://example.invalid/hook`), send only requests that are rejected before a
job starts, such as the wrong method above (`method_not_allowed`) or a wrong
path (`not_found`); an accepted request would try to call the LLM API and post
a failure notice.

### Browser extension

A Chrome/Brave extension (Manifest V3) lives in `extension/`; it is written in
TypeScript and built with Node.js. It is not published to a store — you build it
and load the build as an unpacked extension. It collects a selection, the tab
URL, and the tab title, and reads the video metadata — the video title, the
channel name, and the description — from the video page. It sends the selection,
the URL, and the video metadata to the localhost server, and shows the response.
The tab title is shown for checking (and must not be empty); it is not sent.

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

To update from an earlier build, run `make ext-build` to rebuild
`extension/dist/` first, then click the extension's reload button on
`chrome://extensions/` (Brave: `brave://extensions/`). `extension/dist/` is
build output that pulling new source does not update, so reloading without
rebuilding keeps running the earlier build, which may send without asking for
confirmation.

The extension ID is fixed to `clfmbbcdpnjcefbdihdoahomaifbabkk`, so it is the
same on every machine and from every directory.

Before sending, set the shared token on the extension's options page:
right-click the toolbar icon, then **Options** (or **Details** → **Extension
options** on `chrome://extensions/`). Paste the token the server printed at
startup and save it; if you changed the server's port, set it on the same page
too. Without a saved token the extension does not send and says how to set one.
The value itself is never shown again, only whether one is set.

If you restricted the extension's site access, allow `127.0.0.1`: on
`chrome://extensions/` open **Details** → **Site access** and allow
`127.0.0.1`. Without it the request fails and the extension shows "Could not get
a response from the server", which looks the same as a server that is not
running.

To send, open the transcript panel on a video page, select the caption lines,
and click the extension's toolbar icon to open the popup. The popup shows what
was collected, the video metadata, and a **Send to yt2column** button. It sends
when you press the button. While sending the button is disabled, and after a
send (whatever the result) it stays disabled: reopen the popup to send again.

The extension reads the video metadata from the page without changing it, so a
collapsed description is read only up to what the page shows. When the
description is collapsed, the popup says "The video description is collapsed,
so only its beginning will be sent." with these steps:

1. Click "...more" below the video to show the full description.
2. Reopen this popup, then send.

The button stays usable: pressing it sends the beginning that the page shows.

Some values may not be available: when a video title, channel name, or
description is missing or blank on the page, that value is left out of the
request, and when the page cannot be read at all — or the tab has moved to
another video since the selection was read — all three are left out. The
extension still sends the selection and the URL, and the article is generated
from them.

How to read the result:

- **Accepted** (`202`): the server took the request. It shows a `requestId` and
  says the result of generating and posting the article will arrive in Slack.
- **Rejected**: it shows the reason and a `requestId`.
- **Could not get a response** or **could not interpret the response**: it does
  not resend automatically, and it tells you to check Slack before sending
  again, because the server may already be processing the request.

The `requestId` matches the extension's display with the server log and with a
failure notice in Slack. A successfully generated article carries no
`requestId`, so match it by the video or the time; a failure with no `requestId`
is matched by the time. Do not close the popup while it is sending: if you
close it the result is not shown, and sending again could post the same article
twice.

The selection is sent exactly as copied from the transcript panel; the extension
does not trim it. You can select the whole panel: the server skips the lines the
panel adds around the captions — the lines before the first timestamp (such as
the panel's title), chapter heading lines, and, at the end, blank lines and the
`Sync to video time` (`動画の時間と同期`) button label. When the timestamp and
the text are on separate lines, a heading directly after a timestamp with no
text is taken as that timestamp's text. Other lines are still rejected: an
unknown line among captions that put the timestamp and the text on one line, an
empty or whitespace-only line among the captions, a tab or other control
character in any line, and, before the first timestamp or where a timestamp line
is expected, a line containing an ASCII digit, a colon, and a digit in a row
(such as a malformed timestamp).

To see what was collected and the send result, open the popup's console from
the moment the popup opens: right-click the toolbar icon and choose
**Inspect popup** (shown while Developer mode is on; pin the extension first if
its icon is in the Extensions menu). The log is lost when the popup closes, so
after closing it, find the result of a send in Slack and in the server log by
the time.

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
