> **Project context (read first)**: Read `.claude/commands/_context.md`. The PR
> creation, review, merge, and branch-switch mechanics are defined once in
> `.claude/commands/_lib/pr-review-merge-loop.md`; this command follows that file
> rather than restating it.

Create a PR for a change already committed on the current branch, drive it
through CodeRabbit review and merge, then switch to a fresh branch for the next
change.

Use this for a change that is **not** a `/runplan` task — for example a
`01_requirements.md`, `02_architecture.md`, or `03_implementation_plan.md`
document, or any other standalone change. There is no `/runplan` step and no
implementation-plan checkbox to update (the plan file may not exist yet).
`runpr.md` is the variant for a `/runplan` task PR, where runplan creates the PR
and the plan's PR-checkpoint checkboxes are updated after the merge.

**Input.** `$ARGUMENTS`, when given, is a PR number or a branch to operate on;
with no argument, use the current branch.

Follow `.claude/commands/_lib/pr-review-merge-loop.md` for the PR with:

- **CREATE_PR** = `true` — create the PR when the target branch has none.
- **POST_MERGE** = none.

Then report the final status. There is no next-PR prompt: create the next change
and run this command again when ready.
