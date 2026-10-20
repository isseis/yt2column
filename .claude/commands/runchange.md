---
description: "Drive a task-document PR, or a committed standalone change, through CodeRabbit and CI and merge it"
argument-hint: "[PR | branch]"
---

> **Project context (read first)**: Read `.claude/commands/_context.md`. The PR
> creation, review, merge, and branch-switch mechanics are defined once in
> `.claude/commands/_lib/pr-review-merge-loop.md` and the local CodeRabbit review
> in `.claude/commands/_lib/coderabbit-local-review.md`; this command follows
> those files rather than restating them.

Drive a PR through CodeRabbit review, CI, and merge, then switch to a fresh
branch for the next change. Use it for:

- a task-document PR, which `mkreq.md`, `mkarch.md`, `mkplan.md`, or
  `mkplan2.md` opened and handed to the reviewer (`_lib/task-pr-handoff.md`);
  the reviewer runs this command after reviewing it;
- a standalone change committed on the current branch without an
  implementation plan (a code change, a development-environment file), for
  which this command creates the PR.

There is no implementation-plan checkbox to update. `runpr.md` is the variant
for a `/runplan` task PR.

**Input.** `$ARGUMENTS`, when given, is a PR number or a branch to operate on;
with no argument, use the current branch.

First resolve whether the target branch has a PR (`gh pr view` as the shared
loop's step 1 does). When it has none, review the change locally before the PR
is created: follow `.claude/commands/_lib/coderabbit-local-review.md` with
**BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
**SUMMARY** = the final status report. Skip this when the PR exists and its
branch follows the feature branch convention (`_context.md`): a task-document
command ran the same review before opening it (`_lib/task-pr-handoff.md`
Part B), and the PR-side CodeRabbit review covers commits pushed since. A PR
opened by hand gets the local review, with the fixes pushed to it. Also skip it
when `$ARGUMENTS` names a PR or branch other than the checked-out one, since the
review reads the local checkout.

Follow `.claude/commands/_lib/pr-review-merge-loop.md` for the PR with:

- **CREATE_PR** = `true` — create the PR when the target branch has none.
- **PRE_MERGE** = for each task document the PR adds or changes
  (`01_requirements.md`, `02_architecture.md`, `03_implementation_plan.md`)
  whose status is `draft`: set its Document Status to `approved`, `Review date`
  to today, `Reviewer` to the user's GitHub name (`gh api user --jq .login`),
  and keep `Comments` (or `-`). Commit it alone as
  `docs(<task-id>): approve the <document>`. This is the only way the commands
  set a document to `approved`, and only on the user's answer
  (`docs/dev/developer_guide/requirements_process.md`, "LLM Constraints"). None
  for a PR without a draft task document.
- **NEXT** = by the latest document this PR approves: `01_requirements.md` →
  `/mkarch <task>`; `02_architecture.md` → `/mkplan <task>`;
  `03_implementation_plan.md` → `/runplan <task>`. None when the PR approves no
  task document.

Then report the final status.
