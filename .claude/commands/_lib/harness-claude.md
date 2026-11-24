---
description: "Internal: Claude Code harness adapter implementing the 10 abstract operations (not a command)"
---

# Harness Adapter: Claude Code

This adapter implements the 10 abstract operations defined in
`.claude/commands/_lib/harness.md` for Claude Code sessions (where
`AskUserQuestion`, `Bash`, and `Agent` are all available). Look up every
role-to-model mapping and verification pattern in
`.claude/commands/_context.md` ("Harness model mapping", `Claude Code` rows).

## 1. select-question

Use `AskUserQuestion`:

- Pass 1–4 questions per call, each with 2–4 options.
- Put the recommended option first and mark it as recommended when there is
  one.
- When a reply does not match any listed option, never infer an intent; ask
  again until a valid choice is given.

## 2. file-io

Use `Read`, `Write`, and `Edit` for reading, creating, and modifying files in
the repository and scratchpad.

## 3. scratchpad

Write temporary files (PR titles, PR bodies, commit message drafts for mention
checks, extracted `origin/main` scripts, review outputs) into the Claude Code
session's scratchpad directory outside the git work tree. Never place
scratchpad files inside the tracked repository tree or stage them in git.

## 4. shell-run

Use `Bash`.

- `.claude/settings.json` governs allow, ask, and deny permission rules.
- Follow `CLAUDE.md` ("Tool Execution Safety"),
  `docs/dev/conventions/dev_tools.md` Rule 5 (never install or build installed
  developer tools outside the development exception), and
  `docs/dev/guides/chief_guide.md` section 6 (never invoke mutating `herdr`
  subcommands or `git worktree` add/remove/move operations).
- Follow `.claude/commands/_context.md` ("Passing a title to `gh`") whenever
  passing a PR title to `gh` so shell expansion cannot leak environment
  variables.

## 5. web-lookup

Use `WebSearch` and `WebFetch` to confirm external API, tool, or specification
behavior. If either is unavailable in the session, report that explicitly
instead of skipping silently.

## 6. subagent-inherit

Use `Agent` without setting `model` so the subagent inherits the session's
default model. Provide all required file paths as resolved absolute paths and
instruct the subagent to inspect git ranges directly when needed.

## 7. subagent-role

Look up `Model` in `.claude/commands/_context.md` ("Harness model mapping",
`Claude Code` row for `standard` or `low`).

- Call `Agent` with `model` set to that `Model` value (`sonnet` for `standard`,
  `haiku` for `low`).
- Never use the `fork` subagent type when setting `model`, because `fork`
  ignores `model`.

## 8. pinned-worker

Look up `Model` and `Verification pattern` in `.claude/commands/_context.md`
("Harness model mapping", `Claude Code` row for `high` or `cross-check`).

- **Launch:** call `Agent` with `subagent_type` set to `general-purpose` and
  `model` set to `<Model>`. Never use `fork`. Never write a concrete versioned
  model ID into the prompt.
- **First-line model declaration and verification:** instruct the subagent to
  print `Model: <model-id>` as its very first output line, copied from its own
  system prompt, and begin the structured findings (or `No findings.` / JSON)
  on line 2. Write line 1 alone to a scratchpad file and check it against the
  row's `Verification pattern` without regex or glob evaluation: `.` matches
  only a literal `.`, and a trailing `*` matches the literal prefix followed by
  1–48 characters in `[a-z0-9.-]`.
- **Worktree state check:** take the worktree state check before launch,
  compare all parts on every exit (completion, mismatch, error, timeout, or
  caller stop), and run the post-exit comparison before subsequent commits as
  required by the calling procedure. Keep the check script text and baseline
  hashes in session context (never only in scratchpad). On any difference or
  missing baseline, discard all results and stop Fail-Closed immediately.
- **Timeout and retry:** pair the launch with a 30-minute (1,800-second)
  timer (`timer-wait`). Cancel the timer via `TaskStop` on every exit path.
  Retry at most once on a transient error or usage limit; never retry on model
  mismatch, timeout, or state-check difference.
- **Parallel workers (high-model review):** launch both discovery workers in
  parallel; if either fails model verification or triggers a state-check
  difference, stop all active workers with `TaskStop`, cancel the timer, run
  the exit state check, and stop Fail-Closed.
- **Failure contract (never substitute another model):**
  - High-model review: Fail-Closed stop (no PR, no push, no fix commits).
  - Cross-check partial review / post-merge check: Fail-Open `not run`
    (`unavailable`, `timeout`, `wrong-model`, or `unreadable`), except a
    state-check difference which stops Fail-Closed.
  - PR-review escalation: Fail-Safe `ask` on unavailability, mismatch, timeout,
    or invalid JSON; Fail-Closed stop on state-check difference.
- **Cost recording:** record duration (seconds from notification `duration_ms`
  or wall-clock elapsed time) and total tokens (`subagent_tokens`, or `unknown`
  when absent).
- **Self-verifying implementation model:** Claude Code sessions know their own
  model ID from the system prompt and record `Implementation model:` lines at
  the end of each phase group (`runplan.md` step 7.8).

## 9. timer-wait

Start a background timer with `Bash` running `sleep <seconds>` with
`run_in_background` enabled (1,800 seconds for 30-minute review supervision).
Keep the returned task ID and stop the timer (or an overdue subagent) with
`TaskStop` on every exit path. Because `TaskStop` is a deferred tool, load it
first with `ToolSearch` (`select:TaskStop`) before its first call.

## 10. notify-user

When `PushNotification` is available, send a Japanese notification containing
the PR URL and the next action (`_context.md`, "Reviewer ping"). Always repeat
the PR URL and next action in the final report, which serves as the sole
channel when `PushNotification` is unavailable or not sent.
