---
description: "Drive a /runplan task PR through CodeRabbit and CI, merge it, and update the plan"
argument-hint: "[PR | branch | task]"
---

> **Project context (read first)**: Read `.claude/commands/_context.md`. The
> review, merge, and branch-switch mechanics are defined once in
> `.claude/commands/_lib/pr-review-merge-loop.md`; this command follows that file
> rather than restating it. The implementation steps and the PR checkpoint's
> first two checkboxes are in `.claude/commands/runplan.md` (step 5a); the last
> two are this command's POST_MERGE.

Drive one already-created PR through CodeRabbit review and CI, merge it, and
do the plan bookkeeping, then offer to continue with the next PR of the same
task. `/runplan` opened the PR and handed it to the human reviewer; the reviewer
runs this command after reviewing it. It is to `/runplan` what `/runchange` is
to the task-document commands (`mkreq`, `mkarch`, `mkplan`, `mkplan2`).

**Input.** `$ARGUMENTS`, when given, is a PR number, a branch name, or a task ID
(e.g. `0015`); with no argument, use the current branch's PR. The PR is created
by `runplan.md` step 5a, so this command does not create it.

Follow `.claude/commands/_lib/pr-review-merge-loop.md` for the PR with:

- **CREATE_PR** = `false` — `runplan.md` step 5a creates the PR.
- **POST_MERGE** = on the next branch the shared loop created (always the same
  task, so it always creates one), update `03_implementation_plan.md` and commit
  once:
  - in the just-merged PR checkpoint, mark "PR がマージされた" and
    "次のブランチへ切り替えた" as `[x]`;
  - in the plan's PR checklist (§6), mark that PR's `PR-N マージ済み` as `[x]`.

  The next `/runplan` PR carries this commit to `main`. On the task's final PR
  there is no next `/runplan` PR, so hand the bookkeeping over as its own PR:
  follow `.claude/commands/_lib/task-pr-handoff.md`, Part B, with the
  task-document values of its inputs (title e.g.
  `docs(<task-id>): close out the implementation plan`), and stop.


Then ask the user whether to implement the next PR with `/runplan`:

- If yes, run `.claude/commands/runplan.md` for the same task. It implements the
  next phase groups up to the next PR checkpoint, opens that PR, and hands it to
  the reviewer (its step 5a). Stop there: the reviewer reviews the new PR and
  runs `/runpr` for it.
- If the task has no next PR, report the final status and stop.

## Notes

- `fixpr.md` and `mergepr.md` each perform network operations; each has its own
  authorization note — follow it. In particular, `mergepr.md` requires the user's
  explicit approval of the squash commit message.
- If `fixpr` reaches its round cap or leaves a thread it cannot fix, the shared
  loop reports it and stops rather than merging.
