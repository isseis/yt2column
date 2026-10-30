---
description: "Internal: shared review-finding triage scales (not a command)"
---

# Shared pattern: review-finding triage

`fixpr.md` (Phase 2) and `_lib/local-cli-review.md` (used by `runplan.md` step 7.7, `runplan.md` step 5a, `runpr.md` step 1, and `_lib/task-pr-handoff.md` Part B step 1) both classify review findings
on the same scales. This file defines the scales once. A calling command invokes
it by saying "follow `_lib/triage-pattern.md`" and supplies the findings; it
keeps its own input and output shapes (thread JSON, reply JSON, clusters).

Project-specific values (the task root `docs/tasks/NNNN_<name>/`) are in
`.claude/commands/_context.md`. The shared rules R1-R5 (off-level handling,
handoff routing and lifecycle, reply requirements) stay in `fixpr.md`; this file
references them and does not repeat them.

## Per finding

Read the source at the cited location for context, then judge the finding
against `CLAUDE.md` (design principles, Go idioms, testing rules, Performance).
Classify two independent dimensions.

### `verdict` — is the suggestion right for this codebase?

- `"valid"` — fix clearly improves correctness, clarity, or convention alignment
- `"off-level"` — the finding is on a process document and asks it to carry
  detail that belongs to a later phase (see "Abstraction-level check" below)
- `"invalid"` — suggestion is wrong or inapplicable in this context
- `"unclear"` — genuinely uncertain

### Abstraction-level check

The `off-level` verdict and this check apply only to a finding on a process
document under `docs/tasks/NNNN_<name>/`, including prose or documentation a
`/runplan` phase wrote or changed there. For a finding on any other file (code,
tests, commands, other docs), use only `valid`, `invalid`, or `unclear`.

Do this check before choosing `valid`. Apply the shared rules R1 and R2 in
`fixpr.md` ("Shared rules") to decide whether the comment asks the document to
carry detail below its own level. If so, the verdict is `"off-level"`, **not**
`"valid"`, even when the comment is technically correct: the fix is to raise the
document's abstraction level. The underlying concern still counts for
`severity`; an off-level finding can be `must-fix` if the concern is a real
defect. How an off-level finding is fixed and answered (R1-R5, `behaviorGap`)
is defined in `fixpr.md`.

### `severity` — how much does the raised issue actually matter?

Judge the underlying concern on its merits, not just whether you will act on it.

- `"must-fix"` — a real bug, or a correctness/security defect
- `"worth-fixing"` — a legitimate improvement (clarity, convention, robustness)
  but not a bug
- `"no-harm"` — cosmetic only, OR the finding is invalid/inapplicable: safe to
  ignore

Calibration: rate by the WORST outcome when the affected code path executes, not
by how often it executes. A defect that breaks shell/code syntax or makes a
command fail (e.g. an indented heredoc terminator, a malformed quote) is
`must-fix` even if its branch is rarely taken — rarity lowers likelihood, not
the kind of defect. Do not downgrade a real bug to `worth-fixing` just because
it sits in a guarded or seldom-run branch.

### `topic`

One concise Japanese phrase naming what the finding raised, since it appears in
the report to the user (e.g. 「ヒアドキュメント終端のインデント」、
「プロンプト内の古い Go バージョン」), so the summary shows what each finding
was about at a glance.

### `replyBody`

A concise Japanese sentence, since it is posted as the reply on GitHub
(`CLAUDE.md`, "Language"):

- valid: describe the fix (used as the reply after applying)
- off-level: empty string (the fix phase writes it after applying)
- invalid: explain why the suggestion does not apply
- unclear: empty string
