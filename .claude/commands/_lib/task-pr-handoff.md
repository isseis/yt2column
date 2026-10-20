---
description: "Internal: shared branch setup and PR handoff for the commands that produce a task PR (not a command)"
---

# Shared pattern: task branch and PR handoff

The commands that produce a task PR — the task-document commands `mkreq.md`,
`mkarch.md`, `mkplan.md`, and `mkplan2.md`, and `runplan.md` at a PR
checkpoint — each work on their own feature branch and end by opening the PR
and handing it to the human reviewer. This file defines those two parts once. A
calling command invokes them by saying "follow `_lib/task-pr-handoff.md`,
Part A" (before it writes anything) and "Part B" (after its last commit), and
supplies the inputs below.

Project-specific values (feature branch name, reviewer ping, task root, green
gate) are in `.claude/commands/_context.md`. The local review is
`.claude/commands/_lib/coderabbit-local-review.md`; the PR creation mechanics are
`.claude/commands/_lib/pr-review-merge-loop.md` step 1.

These commands do not wait for the review or the merge. The reviewer reviews the
PR and then runs, by hand, the command that drives it to the merge:
`/runchange <PR>` for a task-document PR, `/runpr <PR>` for a `/runplan` PR.
That command asks CodeRabbit to review the latest commits, waits for CodeRabbit
and CI, resolves the threads, and merges.

## Inputs the calling command must supply

- **TASK_DIR** — the target task directory under the task root.
- **PREREQ** — the task documents whose `approved` status the command requires
  (`mkreq.md`: none; `mkarch.md`: `01_requirements.md`; `mkplan.md`:
  `01_requirements.md` and `02_architecture.md`; `mkplan2.md` and `runplan.md`:
  `03_implementation_plan.md`).
- **REPORT** — where the calling command gives its final report.
- **CHECKS** — what runs before the push: for a task-document command, the
  documentation guards
  (`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline`);
  for `runplan.md`, the green gate (`_context.md`).
- **LOCAL_REVIEW** — `run` for a task-document command; `done` for
  `runplan.md`, which reviews each phase group locally (its step 7.7).
- **PR_TEXT** — the PR title and what the body must name: for a task-document
  command, the title of the commit that added the document; for `runplan.md`,
  the PR marker's `推奨タイトル` and its `レビュー観点`.
- **DRIVER** — the command the reviewer runs next: `/runchange` for a
  task-document command, `/runpr` for `runplan.md`.
- **AFTER_CREATE** (optional) — what the calling command does once the PR
  exists, before CodeRabbit is asked (`runplan.md`: mark `PR を作成した` with the
  PR URL, commit, and push).

## Authorization

Invoking a calling command approves, for its task and its PR only, the network
operations this file names: `gh api user`, `gh pr list`, `gh pr view`,
`git fetch origin main`, `git push` (including `-u`), `gh pr create`, and
`gh pr comment` for the CodeRabbit request. `coderabbit review` has standing approval (`CLAUDE.md`,
"Tool Execution Safety"). Anything else that touches the network is asked for
first.

## Part A — prepare the branch

1. Check the working tree with `git status --short`. The only uncommitted change
   allowed is the reviewer's approval of PREREQ: the Document Status table of a
   PREREQ document set to `approved`, with its review date, reviewer, and
   comments. Any other change, or any change when PREREQ is none, is a reason
   to stop and report. An allowed change is committed on its own in step 5.
2. `git fetch origin main`.
3. Use the current branch when its name follows the feature branch convention
   (`_context.md`) for TASK_DIR and either:
   - `HEAD` is `origin/main` (`git rev-parse HEAD` equals
     `git rev-parse origin/main`: the branch the PR loop created after the
     previous merge, with nothing merged since); or
   - the calling command is `runplan.md`, the branch has commits beyond
     `origin/main`, all of them made by `runplan.md` (implementation and
     checkbox commits, not an unopened task-document change), and it has no PR
     yet (`gh pr view <branch>` finds none): the PR group in progress, continued
     with its next phase group.

   A convention-named branch for TASK_DIR in any other state (commits beyond
   `origin/main` with a PR, or with no PR when the caller is a task-document
   command) is a reason to stop and report.
4. Otherwise create a new branch from `origin/main`:
   - Get the GitHub user name with `gh api user --jq .login`.
   - Choose `NN` one above the highest sequence number this task's branches have
     used, looking at local branches, `origin` branches, and the head branches
     of all PRs (`gh pr list --state all --limit 1000 --json headRefName`); merged
     branches are deleted, so the PR list is what remembers them. With none,
     `NN` is `01`.
   - `git switch -c <branch> origin/main`. The allowed uncommitted change from
     step 1 comes along.
5. If step 1 found the reviewer's approval, commit it alone
   (`docs(NNNN): approve the <document>`), before any work that depends on it.
   The approval then predates that work in the branch history.

## Part B — open the PR and hand it over

Run this after the calling command's last commit, with a clean working tree.

1. When LOCAL_REVIEW is `run`, review the branch with the local CodeRabbit CLI:
   follow `.claude/commands/_lib/coderabbit-local-review.md` with
   **BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
   **SUMMARY** = REPORT.
2. Run CHECKS. Fix and commit if they fail.
3. Push and create the PR for the current branch. Ignore the calling command's
   `$ARGUMENTS` here (it is a task or an issue, not a PR or a branch): run
   `git push -u origin <current-branch>` and
   `gh pr create --base main --head <current-branch> --title "<title>" --body-file <file>`,
   writing the body with the Write tool. Do not pause for the user to adjust the
   title or body; the reviewer can edit them on GitHub. The title is PR_TEXT's.
   The body:
   - summarizes the change, and states the approvals this PR commits (step 5 of
     Part A), if any;
   - lists the decisions made with the reviewer during this run, if any;
   - names the points the reviewer should check (for `runplan.md`, the
     `レビュー観点`), including any `unclear` or `deferred` findings of the local
     review;
   - contains `Refs #<issue>` for the issue the requirements document names as
     its own (`対応 issue`), never `Closes`: the issue stays open until the task is
     implemented.
4. Run AFTER_CREATE, if supplied.
5. Make sure CodeRabbit reviews the PR. It normally starts a review on its own
   when a PR is opened or pushed to. Poll `gh pr view <number> --json
   statusCheckRollup` for about two minutes; if no `CodeRabbit` status context
   appears, ask for one with
   `gh pr comment <number> --body "@coderabbitai review"`.
6. Ping the human reviewer (`_context.md`, "Reviewer ping") with the PR URL and
   the next action: review the PR, then run `DRIVER <PR>`. If the ping tool is
   unavailable or the ping is not sent, the report in step 7 carries the same
   message.
7. In REPORT, give the PR URL, the review results, and the same next action.
   Then stop: do not wait for CodeRabbit and do not merge.
