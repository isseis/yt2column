# CLI

The CLI is the development path: use it to develop and check the LLM call and
the posting, while the primary path is the one in
[Quick start](../../../README.md#quick-start).

This document describes what only the CLI uses or shows. The environment
variables the CLI shares with the localhost server are in the README's
[Configuration](../../../README.md#configuration); the variables only the CLI
reads are in [Configuration](#configuration) below.

## Requirements

**For the CLI only:**

- `yt-dlp` on `PATH` (or set `YT2COLUMN_YTDLP_PATH`); see
  [Setting up yt-dlp](#setting-up-yt-dlp)
- `SLACK_WEBHOOK_URL`, only when `--slack` is used

The other requirements, such as Go and the API key of the selected provider,
are in the README's [Requirements](../../../README.md#requirements).

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

What the shorten and review steps do, and why they cost more, is in the
README's [Post-generation steps](../../../README.md#post-generation-steps).

With `--no-shorten` the shorten step and the final length
check are skipped; with `--no-review` the review step is skipped.
`--no-shorten --no-review` restores the single-call behavior of earlier
versions, including the `--out` format.

An article's model entries start with the generation model and add one entry per
step that ran, in the order shorten, review (see [Output format](#output-format)).
For unattended runs, pass
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
the configuration is independent of the flags: `YT2COLUMN_REVIEW_LLM` is still
checked, and if it selects a different provider, that provider's variables are
checked too. Its API key is required, and for `claude` a set effort must still
be valid (an unset one selects `high`). A resumed run validates only the
settings of the stages it executes, so `--after review` requires no LLM
settings at all. The one exception is the retired LLM variables
(`YT2COLUMN_LLM_PROVIDER`, `YT2COLUMN_MODEL`, `YT2COLUMN_REVIEW_PROVIDER`,
`YT2COLUMN_REVIEW_MODEL`): every run, `--after review` included, stops when
one of them is set (see [Configuration](../../../README.md#configuration)).

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

## Output format

The article is the same text in both outputs, but each output arranges the
information differently: the reader's part (title, channel name, body, source
link) comes first, and the development part (models, estimated cost) comes last
in a Slack post and in the front matter of a file.

The Slack post is shown in the README's
[The Slack post](../../../README.md#the-slack-post).

### The `--out` file

The file starts with a YAML front matter, then the title, the channel name (when
there is one), and the body:

```text
---
channel: "Example Channel"
source: "https://www.youtube.com/watch?v=abcdefghijk"
model: "deepseek-flash"
model_version: "v4"
shorten_model: "deepseek-flash"
shorten_model_version: "v4"
review_model: "claude-haiku-5-5"
review_model_version: "2026-01-01"
estimated_cost_usd: "0.0094"
---

# Caching transcripts by video ID

Example Channel

The cache key comes from the video ID alone.

## One key per video

Two URLs for one video share one entry.

出典: <https://www.youtube.com/watch?v=abcdefghijk>
```

- The keys appear in this order. `channel` appears only when the video has a
  channel name; `shorten_*` only when the article went through the shorten step (run now or
  reused with `--after`); `review_*` only when it went through the review step.
- Every value is a double-quoted string, quoted with Go's `strconv.Quote`, so a
  value that contains a quote, a backslash, `:`, `#`, or `---` cannot change the
  structure. `estimated_cost_usd` is a string too: four decimals, or `unknown`
  (see [Estimated cost](#estimated-cost)). An empty model version is written as
  `(none)`.
- The channel name appears again under the title, so a Markdown viewer that
  hides the front matter still shows it. A name that starts like a Markdown block
  (`# `, `- `, `> `, and so on) gets a backslash at the start (after the digits
  for a numbered-list marker) so that the rendered line is the name; leading
  whitespace is dropped.
- This format replaces the earlier one, which listed the models and the cost
  under the title (`- Model: …`).

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

Both outputs carry the article's cost, with four decimals, or `unknown`: the
`--out` file as the front matter key `estimated_cost_usd` (`"0.0023"` or
`"unknown"`), and the Slack post as the end of its footnote, which the README's
[Estimated cost](../../../README.md#estimated-cost) shows.

See [Output format](#output-format) for both layouts.

Why the estimate can differ from the provider's invoice is in the README's
[Estimated cost](../../../README.md#estimated-cost). A reused stage is priced with
the rate table of the CLI that resumes, not the one that saved the artifact, so
a run that resumes after a rate change reports a different article cost for the
same saved text: the artifact records the call time, not the rate.

A Slack post from the localhost server cannot show why an unknown cost is
unknown. To find the
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

The channel name in a saved `info.json` is checked before the LLM is called: a
name that is only whitespace, contains a control character (or a bidirectional
control character such as U+202E), or is longer than 200 code points makes the
run fail, where earlier versions accepted it. Run with
`--refresh` to fetch the video again and replace the cache. The localhost
server rejects such a `channelName` with `invalid_body`.

## Webhook publishing

With `--slack`, the article is posted to the Slack-compatible Incoming Webhook
in `SLACK_WEBHOOK_URL` instead of a file. The rules for the URL, the split into
several messages, `silent`, and the check for syntax that would become a
mention are in the README's
[Webhook publishing](../../../README.md#webhook-publishing).

A URL those rules reject is a configuration error with exit code `2`, and
because the variable is validated whenever it is set, this also stops an
`--out` run.

When a mention target is posted with U+200B inserted, the file written by
`--out` and the article itself are not changed.

A rejected article's error gives the line and column in the post (the lines
differ from a file written by `--out`, which starts with front matter). Running
again generates a different article, which may not have the problem; run with
`--out` to inspect what the model generated.

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

The CLI reads the variables in the README's
[Configuration](../../../README.md#configuration), and these two, which only the
CLI reads:

| Variable | Description | When unset |
|---|---|---|
| `YT2COLUMN_CACHE_DIR` | Cache directory for subtitles and video metadata | `$HOME/Library/Caches/yt2column` on macOS; `$XDG_CACHE_HOME/yt2column` or `$HOME/.cache/yt2column` on other Unix |
| `YT2COLUMN_YTDLP_PATH` | Path to `yt-dlp` | `yt-dlp` on `PATH` |

`yt2column-server` reads neither `YT2COLUMN_CACHE_DIR` nor `YT2COLUMN_YTDLP_PATH`,
because it never calls `yt-dlp`.

`SLACK_WEBHOOK_URL` is required with `--slack`; optional with `--out` (no
value).

A model ID with surrounding white space is rejected, before any request, when
the step's client is built: the CLI builds its clients before the run starts.

With `YT2COLUMN_REVIEW_LLM` unset, the review step uses generation's provider and
model also in a resumed run that only reviews; a resumed run that only reviews
and has `YT2COLUMN_REVIEW_LLM` set does not read `YT2COLUMN_LLM`.

The retired LLM variables stop every run, including runs that read no LLM
setting such as `--after review`; a CLI run exits with `2` before it starts.
See the README's
[Migrating from the retired LLM variables](../../../README.md#migrating-from-the-retired-llm-variables).
