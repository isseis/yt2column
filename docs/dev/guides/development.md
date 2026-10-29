# Development

This document is for developers of yt2column: the `make` targets, a separate
server for development, the extension's development checks, the localhost
server's HTTP API, and the integration tests. Using the browser extension and
the localhost server is described in the [README](../../../README.md), and the
CLI in [cli.md](cli.md).

## Make targets

```sh
make build   # build the three binaries into build/
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
with explicit acceptance criteria. See [CLAUDE.md](../../../CLAUDE.md) and
[docs/dev/conventions/requirements_process.md](../conventions/requirements_process.md).
Design documents are written in Japanese; start with
[docs/dev/project_overview.md](../project_overview.md).

## Running a separate server for development

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

The extension cannot reach such a server. The browser starts the native
messaging host with your real `HOME`, so the host connects only to the socket
of a server started with your real `HOME`. The host manifest is one file per
browser (on macOS, one file shared by Chrome and Brave), so installing the host
from one worktree's build replaces the manifest another worktree installed;
check a build from the extension one worktree at a time.

## Browser extension checks

Building and loading the extension is described in the README's
[Browser extension](../../../README.md#browser-extension). With the same Node.js
version active, one more target runs the checks CI runs:

```sh
make ext-check     # typecheck, lint, format check, tests, then build
```

## Localhost server API

Starting and stopping the localhost server is described in the README's
[Localhost server](../../../README.md#localhost-server). This section is its
HTTP API.

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

When `curl` cannot connect:

- **The server is not running.** `curl` reports `Couldn't connect to server`
  (its message names `localhost port 80`, the URL's host and default port; the
  connection itself goes to the socket). Start the server as above; the first
  log line names the socket it listens on.

An accepted request answers `202` with `{"status":"accepted","requestId":"…"}`.
A rejected request answers a `4xx` or `5xx` with
`{"status":"rejected","reason":"…","requestId":"…"}`, where `reason` is one of:

| reason | status code | extension shows |
|---|---|---|
| `not_found` | 404 | unexpected reply |
| `method_not_allowed` | 405 | unexpected reply |
| `too_many_requests` | 429 | fixed message |
| `body_too_large` | 413 | fixed message |
| `invalid_body` | 400 | fixed message |
| `invalid_video_url` | 400 | fixed message |
| `invalid_transcript` | 400 | fixed message |
| `webhook_not_configured` | 503 | unexpected reply |
| `invalid_key` | 400 | unexpected reply |
| `article_not_ready` | 409 | unexpected reply |
| `article_not_found` | 404 | unexpected reply |
| `article_not_postable` | 409 | unexpected reply |
| `internal_error` | 500 | fixed message (unexpected reply when `requestId` is empty) |

`not_found` and `method_not_allowed` answer a wrong path or method; they start
no job and post no failure notice. The current extension's message declares no
`v` member, so the host relays it to `POST /articles` unchanged and the
extension shows either rejection as an unexpected reply.

### The job API

Posting an article is an operation of its own: the extension's current
`/articles` request generates and posts in one go, and the job API below
generates, returns, and posts on request.

Besides `/articles`, the server accepts three operations that keep generation
and posting apart, so that a viewer can show the article and post it only when
the user asks. The current extension does not use them yet (#503 moves it);
`/articles` keeps working until then and is removed afterwards.

| operation | request | success response |
|---|---|---|
| generation start | `POST /jobs`, the same JSON object as `/articles` | `202`, `{"status":"accepted","key":"…","requestId":"…"}` |
| status | `GET /jobs/<key>`, no body | `200`, `{"status":"ok","requestId":"…","webhookConfigured":true,"job":{…}}` |
| publish | `POST /jobs/<key>/publish`, no body | `202`, `{"status":"accepted","requestId":"…"}` |

`<key>` is the article key. It is an opaque value: send back the one a
generation start returned, and do not depend on its shape or on how it is
computed. It is also specific to the server process: after a restart, the same
input gets a new key, so use the key a new generation start returns. A status or
publish target whose key is not the accepted shape is rejected with
`invalid_key`; neither operation takes a body, and any job API target with a
query answers `not_found`.

The status response reports the job as one of the codes below. A `running` job
carries `steps`; a `done` job carries `steps` (only `done` or `skipped`),
`article`, and `publish`; a `failed` job carries `steps` and `failure`, the
failure code. A step is `failed` only when the generation failed in that step; a
completion-check failure (`article_invalid`, `article_too_large`) or a failure
before the steps marks none. An unknown key carries no member besides `state`.
The `article` member holds `markdown` (the display Markdown),
`file` (byte for byte what `--out` writes), `cost`, and `durationMs` (from the
accepted start that generated the article, or from the accepted retry after a
failure). `cost` is `{"state":"known","usd":"…"}` or `{"state":"unknown"}`.
`publish` is `{"state":"…"}`, with a `reason` for `failed` and `unpostable`.
`webhookConfigured` says whether this server can post.

```json
{
  "status": "ok",
  "requestId": "0123456789abcdef",
  "webhookConfigured": true,
  "job": {
    "state": "done",
    "steps": { "draft": "done", "shorten": "skipped", "review": "done" },
    "article": {
      "markdown": "# …",
      "file": "---\n…",
      "cost": { "state": "known", "usd": "0.1234" },
      "durationMs": 81234
    },
    "publish": { "state": "not_posted" }
  }
}
```

Sending the same input again returns the same key without calling the LLM
while that article is being generated or is done; a failed key is generated
again. At most two generations run at once, counting the `/articles` jobs. A
generation start reads its body under two reading slots, and the `/articles`
endpoint reads a body only while it holds one of its generation slots, so at
most four bodies are read at once. A start refused for a full reading slot, a
full set of generations, or a full cache with no record to drop answers
`too_many_requests`, and no job starts.

The articles live in the server's memory only. The cache holds at most 100
records; when it is full it drops the oldest completed article or failure, and
never a record that is being generated or posted. A restart loses every article
and every publish state, so a status request then answers `unknown`. An article
whose pre-send checks refuse it has the publish state `unpostable` and cannot be
posted: it still comes back as `done` (it can be displayed and saved), and only
generating it again after a restart or after its record was dropped makes a
postable article.

The publish operation answers `202` without waiting for the Webhook's reply and
does not post an article twice: publishing a `posting` or `posted` article again
sends nothing, and a `failed` post resumes from the first message not delivered
(that one message may arrive twice). Posting needs `SLACK_WEBHOOK_URL`; without
it the operation answers `webhook_not_configured`, the status response carries
`"webhookConfigured": false`, and `/articles` is refused the same way. A server
started without `SLACK_WEBHOOK_URL` still generates articles, but posting (the
job API's publish operation and `/articles`) is refused with
`webhook_not_configured`.

A status request that answers is not logged, because a viewer polls it every few
seconds, so its `requestId` has no line in the log; a rejected status request
logs the rejection. Every other operation logs a line with its `requestId`, and
the job API's jobs log the key.

A request line in the server log carries the `requestId` the response returns
(see the README's [Running the server](../../../README.md#running-the-server)).
The job API's generation and post jobs log the article key instead, so follow
one with `grep "$key" server.log`.

On the first signal, the server cancels the job API's in-flight generations
(their LLM calls stop and their articles are lost, because the cache goes away
with the server), and lets the in-flight `/articles` jobs and the publish jobs
finish.

With the socket from the previous section and the body in `request.json`, the
three operations are:

```sh
curl -sS --unix-socket "$socket" -X POST http://localhost/jobs --data-binary @request.json
curl -sS --unix-socket "$socket" -X GET "http://localhost/jobs/$key"
curl -sS --unix-socket "$socket" -X POST "http://localhost/jobs/$key/publish"
```

`$key` is the key the generation start returned.

### Job API codes

The job API's status response reports each value as one of these codes.

| job state | meaning |
|---|---|
| `unknown` | the server holds no record for the key |
| `running` | the article is being generated |
| `done` | the article is generated and returned |
| `failed` | the generation failed |

| step | step of the generation |
|---|---|
| `draft` | the generation of the draft |
| `shorten` | the shortening of a long body |
| `review` | the review against the source material |

| step state | meaning |
|---|---|
| `pending` | not started |
| `running` | started and not finished |
| `done` | finished |
| `skipped` | not needed for this article |
| `failed` | the step the generation failed in |

| failure | meaning |
|---|---|
| `internal` | an internal failure |
| `transcript_too_long` | the transcript is too long for the prompt |
| `llm_timeout` | an LLM call timed out |
| `llm_empty` | an LLM returned an empty response |
| `llm_truncated` | an LLM response was cut off |
| `llm_failed` | any other LLM failure |
| `article_invalid` | the generated article cannot be published |
| `article_too_large` | the generated article does not fit the status response |

| publish state | meaning |
|---|---|
| `unpostable` | the Slack pre-send checks refuse the article |
| `not_posted` | postable and never posted |
| `posting` | being posted |
| `posted` | every message was delivered |
| `failed` | the last post stopped before the end |

| publish reason | with publish state | meaning |
|---|---|---|
| `other` | `unpostable` | any pre-send refusal but a mention |
| `mention` | `unpostable` | the article contains a mention |
| `internal` | `failed` | the post job ended abnormally |
| `webhook` | `failed` | the Webhook did not acknowledge a message |

## Integration tests

### Integration test

`make test-integration` runs the real `yt-dlp` against the network, so it needs
`yt-dlp` set up as in [Setting up yt-dlp](cli.md#setting-up-yt-dlp) and on `PATH`.
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
does not run it. The model comes from `YT2COLUMN_TEST_DEEPSEEK_MODEL`,
defaulting to `deepseek-flash` when that variable is undefined (an empty value
is passed through and fails the test). The target exports the opt-in variable
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
`YT2COLUMN_TEST_DEEPSEEK_API_KEY`, defaults `YT2COLUMN_TEST_DEEPSEEK_MODEL` to
`deepseek-flash` when undefined, and exports its opt-in variable
`YT2COLUMN_CLI_INTEGRATION=1` for its target alone; unlike it, a missing test
API key fails the test rather than skipping it, so an opted-in run cannot pass
without calling the API.

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
`YT2COLUMN_TEST_DEEPSEEK_MODEL` to `deepseek-flash` when undefined, and it
exports `YT2COLUMN_CLI_SLACK_INTEGRATION=1` for its target alone.
