# Re-measuring the Claude output limit

When a request leaves the output limit to the provider (`MaxOutputTokens` 0),
the Claude adapter sends its own default `max_tokens`, the constant
`defaultMaxOutputTokens` in `internal/llm/claude/request.go`. All three writer
steps (generation, shorten, review) use that default. The value rests on
measured output sizes and output rates, so it can go stale when what produces
the output changes. This guide says when to measure again, how, and what to
update. It copies neither the value nor the measurements: they live in the code
and in the record named under "Current record".

## When to re-measure

Re-measure when any of the three steps of the production configuration changes:

- its template;
- its effort, including a change to `defaultClaudeEffort` in
  `internal/config/config.go`;
- its model.

## Cost and approval

Measuring calls the real Anthropic API and is billed. Get a person's approval
before running it (`CLAUDE.md`, "Tool Execution Safety"). For how to run an
evaluation set of videos, follow
`docs/tasks/0008_column_prompt/02_evaluation.md` §2.3 and
`docs/tasks/0018_article_quality/02_evaluation.md` §3.

## How to measure

The rate is computed per LLM call, so each CLI run must call the LLM exactly
once. Run each step separately and reuse the earlier step's saved output with
`--after`. Give every run `--artifacts <dir>` (an existing directory outside
the cache directory), `--keep-cache`, and `--out` with a fresh path:

- `--keep-cache` keeps the video's cache after a successful run. Without it,
  the next run fetches the transcript again, which adds network time to the
  measured time, and a changed transcript makes the resume fail.
- `--out` writes the article to a file, so a measurement never posts to the
  Webhook.
- Runs that share a cache directory run one at a time.

The steps:

1. Generation: `--no-review --no-shorten`.
2. Shorten: `--after generation --no-review`. The shorten step runs only when
   the generated body is over the length limit, so a run whose body is within
   it calls no LLM and gives no shorten measurement.
3. Review, after a run whose shorten step ran: `--after shorten`.
4. Review, after a run whose shorten step did not run: `--after generation
   --no-shorten`. `--after shorten` cannot be used there, because no shorten
   artifact was saved and loading it fails. This form skips the shorten step
   and the final length check, so use it only when the shorten step did not
   run.

## What to collect

For each run:

- the output token count of the step: `output=` on the
  `tokens: <stage> run` line of the step that called the LLM (README, "Token
  usage"; a resumed run also writes `reused` lines for the earlier stages), or
  `usage.output` in the step's saved file under the `--artifacts` directory;
- the wall-clock time of the CLI invocation, for example with `time`, as the
  recorded measurements were taken;
- whether the run was truncated: the CLI reports an error that wraps
  `llm.ErrTruncated` ("generation truncated by the output token limit").

## How to judge

Check the value against the three conditions of
`docs/tasks/0041_claude_effort_max_tokens/01_requirements.md` F-002:

- (a) The value is at least twice the largest observed output.
- (b) Compute each run's rate as its output tokens divided by its wall-clock
  time. The time includes work besides the LLM call, so the rate comes out
  lower than the real one, which errs on the safe side. Producing the value at
  the slowest rate must take at most half of `provider.LLMTimeout`.
- (c) For each model F-002 (c) names (today `claude-opus-5-5` and
  `claude-sonnet-5-5`), the model accepts the value as `max_tokens` in a
  non-streaming request: it is at most the model's
  maximum output in the Anthropic documentation, or an existing observation
  shows the model accepting it.

A truncated run's output is known only to be at least the limit it was sent;
record it as such.

When no value meets both (a) and (b), do not pick one: changing the call
itself (streaming, or `provider.LLMTimeout`) is a decision of its own, so raise
it as a new task.

## What to update

When the value no longer meets the conditions, update in one change:

1. the constant `defaultMaxOutputTokens` in `internal/llm/claude/request.go`;
2. the named measurements in `internal/llm/provider/provider_test.go` that
   `TestClaudeDefaultMaxTokensFitsBudget` checks the value against:
   `largestObservedOutputTokens`, `slowestRateTokens`, and
   `slowestRateDuration`, with their source comment;
3. the record of the basis: the `02_architecture.md` of the task that makes the
   change;
4. "Current record" below, to point at that record;
5. the README's "Configuration" notes on `max` and on the default effort, when
   the new measurements change what they say.

## Current record

The current basis is recorded in
`docs/tasks/0041_claude_effort_max_tokens/02_architecture.md` §1.4.
