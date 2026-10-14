> **Project context (read first)**: Read `.claude/commands/_context.md`. The
> review, merge, and branch-switch mechanics are defined once in
> `.claude/commands/_lib/pr-review-merge-loop.md`; this command follows that file
> rather than restating it. The implementation steps and PR-checkpoint
> bookkeeping are in `.claude/commands/runplan.md` (step 5a).

Drive one already-created PR through review, merge, and the plan bookkeeping,
then offer to continue with the next PR of the same task.

**Input.** `$ARGUMENTS`, when given, is a PR number, a branch name, or a task ID
(e.g. `0015`); with no argument, use the current branch's PR. The PR is created
by `runplan.md` step 5a, so this command does not create it.

Follow `.claude/commands/_lib/pr-review-merge-loop.md` for the PR with:

- **CREATE_PR** = `false` — `runplan.md` step 5a creates the PR.
- **POST_MERGE** = in the task's `03_implementation_plan.md`, mark the
  just-completed PR checkpoint's remaining checkboxes — the "PR がマージされた"
  and "次のブランチへ切り替えた" items — as `[x]`, and commit. This is the same
  bookkeeping `runplan.md` step 5a defines.

Then ask the user whether to implement the next PR with `/runplan`:

- If yes, run `.claude/commands/runplan.md` for the same task. It implements the
  next phase group and creates the next PR (its step 5a); then follow the shared
  loop again for that PR.
- If the task has no next PR, report the final status and stop.

## Notes

- `fixpr.md` and `mergepr.md` each perform network operations; each has its own
  authorization note — follow it. In particular, `mergepr.md` requires the user's
  explicit approval of the squash commit message.
- If `fixpr` reaches its round cap or leaves a thread it cannot fix, the shared
  loop reports it and stops rather than merging.
