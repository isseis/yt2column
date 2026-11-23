---
description: "Internal: deterministic harness detection, abstract operation catalog, and OpenCode fallback rules (not a command)"
---

# Harness Adapter Entry

This document is the shared entry point for harness-independent command
definitions and shared procedures under `.claude/commands/`. It defines how to
detect the active harness deterministically, lists the 10 abstract operations
that commands reference, and specifies the fallback behavior for OpenCode (or
any other harness without a dedicated adapter).

Model identifiers, launch mechanisms, and verification patterns live solely in
`.claude/commands/_context.md` ("Harness model mapping"). Neither this file nor
the harness adapters hardcode concrete versioned model IDs in subagent or worker
prompts.

## Deterministic Harness Detection

Evaluate the session's declared built-in tools in the exact order below and
load the first matching adapter. Do not guess the harness from prose, paths, or
environment variables.

| Order | Harness | Required built-in tools | Adapter to load |
|---|---|---|---|
| 1 | `Antigravity` | Both `ask_question` and `run_command` are available (`swarm` availability is checked individually by `subagent-role` and `pinned-worker`) | `.claude/commands/_lib/harness-antigravity.md` |
| 2 | `Claude Code` | All three of `AskUserQuestion`, `Bash`, and `Agent` are available | `.claude/commands/_lib/harness-claude.md` |
| 3 | `OpenCode` (or other) | Neither row 1 nor row 2 matches | None (follow "OpenCode Fallback Rules" below) |

Keeping `swarm` out of row 1 ensures that an Antigravity context where `swarm`
is unavailable (such as inside a nested `invoke_subagent`) still loads
`harness-antigravity.md` and enforces its sandbox and prohibited-command rules.
When `subagent-role` or `pinned-worker` runs without `swarm`, it follows the
safe fallback or stop rules in `harness-antigravity.md`.

## Abstract Operation Catalog

Command bodies and shared procedures reference the 10 operations below instead
of naming harness-specific built-in tools directly. Both
`harness-claude.md` and `harness-antigravity.md` implement these 10 sections
with identical numbers and names.

| # | Operation | Purpose |
|---|---|---|
| 1 | `select-question` | Ask the user structured multiple-choice questions (1–4 questions per call, 2–4 options per question, recommended option first; re-ask rather than guessing if the reply matches no option). |
| 2 | `file-io` | Read, create, and edit files in the work tree or scratchpad. |
| 3 | `scratchpad` | Create and read session-scoped temporary files outside the git work tree (never tracked or committed). |
| 4 | `shell-run` | Run shell commands with timeouts, sandbox rules, and prohibited-command enforcement. |
| 5 | `web-lookup` | Look up external specifications or documentation (report explicitly if unavailable rather than silently skipping). |
| 6 | `subagent-inherit` | Spawn a general subagent that inherits the session's model (self-review, panel review, `/fixpr` Triage/Fix, `runplan` code and test reviews, `mktrans` translation check). |
| 7 | `subagent-role` | Spawn a role-tiered general subagent (`standard` or `low` from `_context.md` "Harness model mapping", used by `japrose`, `fixpr` Phase 6, and verification passes). |
| 8 | `pinned-worker` | Spawn a strictly model-pinned and model-verified worker (`high` or `cross-check` from `_context.md` "Harness model mapping", used by the high-model review, cross-check partial review, post-merge check, and PR-review escalation). |
| 9 | `timer-wait` | Supervise a 30-minute (1,800-second) timeout and cancel timers on every exit path. |
| 10 | `notify-user` | Notify the user when long-running work finishes or opens a PR (falls back to the final report when no push-notification tool exists, per `_context.md` "Reviewer ping"). |

## Self-Verification of Implementation Model

Whether a session may write `Implementation model:` record lines in commit
messages is governed by `_context.md` ("Harness model mapping"):

- When the harness has `Launch` = `agent` for `Role` = `high` (Claude Code),
  the session can read its own model ID from its system prompt and writes
  `Implementation model:` lines as required by `runplan.md`.
- When the harness has `Launch` = `swarm` or `none` (Antigravity, OpenCode),
  the session cannot self-verify its model ID from its system prompt and writes
  no `Implementation model:` line. For PRs whose implementation model
  requirement is `high`, `standard+high-review`, or unreadable, the high-model
  review therefore runs before the PR opens unless a valid done record already
  covers the branch.

## OpenCode Fallback Rules

When neither the Antigravity nor the Claude Code detection row matches (for
example, OpenCode running `/runpr` or a `low`-tier `/runplan`), apply the
following rules for the 10 operations:

1. `select-question`: use OpenCode's native question mechanism if available, or
   ask in plain text with numbered options and re-ask if the answer matches no
   option.
2. `file-io`: use OpenCode's built-in file read/write/edit tools.
3. `scratchpad`: write temporary files to a directory outside the git work
   tree (never tracked or committed).
4. `shell-run`: use OpenCode's built-in shell execution tool, obeying all
   prohibitions in `CLAUDE.md` ("Tool Execution Safety"), `AGENTS.md`,
   `docs/dev/conventions/dev_tools.md` Rule 5, and
   `docs/dev/guides/chief_guide.md` section 6.
5. `web-lookup`: use OpenCode's web search/fetch tool if available; if none is
   available, report that explicitly rather than skipping silently.
6. `subagent-inherit`: use OpenCode's default subagent mechanism (for example,
   `/fixpr` Triage and Fix phases).
7. `subagent-role`: use OpenCode's default subagent mechanism.
8. `pinned-worker`: unavailable (`Launch` is `none` in `_context.md`). Never
   substitute the session's model; follow each gate's safe contract:
   - **High-model review**: Fail-Closed stop (open no PR, push nothing, make no
     fix commits, report unavailability).
   - **Cross-check partial review**: Fail-Open `not run` with reason
     `agent-cannot-call`.
   - **Post-merge check**: Fail-Open `not run` with reason `agent-cannot-call`.
   - **PR-review escalation**: Fail-Safe `ask` (recommend requesting a manual
     PR review in the merge question).
9. `timer-wait`: use whatever wait mechanism the harness provides when needed.
10. `notify-user`: no notification tool is assumed; report the PR URL and next
    action in the final report (`_context.md`, "Reviewer ping").
