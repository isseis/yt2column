---
description: "Create a PR for a committed non-runplan change, review it, and merge it"
argument-hint: "[PR | branch]"
---

> **Project context (read first)**: Read `.claude/commands/_context.md`. The PR
> creation, review, merge, and branch-switch mechanics are defined once in
> `.claude/commands/_lib/pr-review-merge-loop.md` and the local CodeRabbit review
> in `.claude/commands/_lib/coderabbit-local-review.md`; this command follows
> those files rather than restating them.

Create a PR for a change already committed on the current branch, drive it
through CodeRabbit review and merge, then switch to a fresh branch for the next
change.

Use this for a change that is **not** a `/runplan` task — a document such as
`01_requirements.md`, `02_architecture.md`, or `03_implementation_plan.md`, a
standalone code change, a development-environment file, or any other change made
without an implementation plan. There is no `/runplan` step and no
implementation-plan checkbox to update (the plan file may not exist yet).
`runpr.md` is the variant for a `/runplan` task PR, where runplan creates the PR
and the plan's PR-checkpoint checkboxes are updated after the merge.

**Input.** `$ARGUMENTS`, when given, is a PR number or a branch to operate on;
with no argument, use the current branch.

Before the PR is created, review the change locally. Ask once whether to run
the CodeRabbit review, as the Authorization section of
`.claude/commands/_lib/coderabbit-local-review.md` defines, then follow that file
with **BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
**SUMMARY** = the final status report. If the target branch already has a PR,
push any fix commits to it. Skip this when `$ARGUMENTS` names a PR or branch
other than the checked-out one, since the review reads the local checkout.

Follow `.claude/commands/_lib/pr-review-merge-loop.md` for the PR with:

- **CREATE_PR** = `true` — create the PR when the target branch has none.
- **POST_MERGE** = none.

Then report the final status. There is no next-PR prompt: create the next change
and run this command again when ready.
