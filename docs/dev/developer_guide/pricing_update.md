# Updating the pricing table

`internal/pricing` estimates what an LLM call cost from the providers' published
per-token rates. Those rates live as a constant table in
`internal/pricing/rates.go`, and the estimates the CLI prints come from that
table alone. When a rate changes, or a model is added, the table and the tests
that pin it must be updated together, in one change.

## Confirm the rates at the source

- DeepSeek: <https://api-docs.deepseek.com/quick_start/pricing>
- Anthropic (Claude): <https://platform.claude.com/docs/en/about-claude/pricing>

On the page, confirm for the model:

- the input, cache-read, cache-write, and output rates (per million tokens);
- how its rate set is chosen: one rate (a flat scheme), a weekly peak window, or
  a prompt-length threshold;
- for a time-of-day model, the peak weekdays and the peak minute ranges, which
  the CLI reads in UTC. DeepSeek's window is Monday through Friday, 01:00-04:00
  and 06:00-10:00 UTC.

## Update the table

1. Edit the entry in `rateTable` in `internal/pricing/rates.go`: its rates, and
   the fields of its scheme only. A field the scheme does not use must stay zero;
   `validate` rejects it.
2. Update the `Source: <url> (checked <date>)` comment above the entry, and the
   one above the peak rule of a time-of-day model, to the pages and the date you
   confirmed. `TestRateTableSourceComments` checks that each entry has a source
   line and a checked date.
3. For a new model, add an entry with a scheme and the fields that scheme uses,
   and add its name to the tests that enumerate the models
   (`TestRateTableModels`). Run `TestRateTableValid` (`validate` on the real
   table) and `TestValidateRejects` (each rule on a table that breaks only it).

## Update the hand-calculated tests

The expected amounts are computed by hand from the source rates, never derived
from the table or the code:

- `TestEstimateCategories` in `internal/pricing/pricing_test.go`: the cost of one
  stage of each model. Its comment shows the arithmetic
  (`count * rate / 1e6 USD`); update the arithmetic and the `want` strings
  together with the rates.
- `TestEstimatePeakWindow`: the DeepSeek peak/off-peak boundary, if the window
  moved.
- `TestEstimatePromptTier`: the Claude Haiku prompt-length tiers, if the
  threshold moved.
- `TestAmountFormat` in `internal/pricing/amount_test.go`: the rounding of the
  amount at six and four decimals.

## What else a rate change touches

`TestCostDocsContract` checks the README's "Estimated cost" section against the
code's format constants, not against any list in the document, so a rate change
does not force an edit there. Some spots still depend on the table by hand, so
update them yourself:

- the README's "Estimated cost" section states the DeepSeek peak window
  ("Monday through Friday during 01:00-04:00 and 06:00-10:00 UTC"); a window
  that moves makes that prose stale;
- the README's "Output format" examples show an estimated cost computed from
  the rate table (`0.0094`); `TestOutputLayoutDocsContract` compares them with
  the real output, so a rate change fails that test until the two example
  amounts are updated;
- the README's configuration section and `project_overview.md` name the models,
  which change only when the set of models does.

`package_reference.md`'s `internal/pricing` row lists the package's public types;
a rate change leaves it alone. Check the row by reading it in the PR review.
