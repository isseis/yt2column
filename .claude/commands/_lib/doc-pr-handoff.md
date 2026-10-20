---
description: "Internal: shared branch setup and PR handoff for the task-document commands (not a command)"
---

# Shared pattern: task-document branch and PR handoff

`mkreq.md`, `mkarch.md`, `mkplan.md`, and `mkplan2.md` each produce one task
document, and each does it on its own feature branch and ends by opening the PR
and handing it to the human reviewer. This file defines those two parts once. A
calling command invokes them by saying "follow `_lib/doc-pr-handoff.md`, Part A"
(before it writes anything) and "Part B" (after its last commit), and supplies
the inputs below.

Project-specific values (feature branch name, reviewer ping, task root) are in
`.claude/commands/_context.md`. The local review is
`.claude/commands/_lib/coderabbit-local-review.md`; the PR creation mechanics are
`.claude/commands/_lib/pr-review-merge-loop.md` step 1.

The commands do not wait for the review or the merge. The reviewer reviews the
PR and then runs `/runchange <PR>` by hand, which drives the CodeRabbit threads
and the merge.

## Inputs the calling command must supply

- **TASK_DIR** — the target task directory under the task root.
- **PREREQ** — the task documents whose `approved` status the command requires
  (`mkreq.md`: none; `mkarch.md`: `01_requirements.md`; `mkplan.md`:
  `01_requirements.md` and `02_architecture.md`; `mkplan2.md`:
  `03_implementation_plan.md`).
- **REPORT** — where the calling command gives its final report.

## Authorization

Invoking a calling command approves, for its task and its PR only, the network
operations this file names: `gh api user`, `gh pr list`, `git fetch origin main`,
`git push`, `gh pr create`, and `gh pr comment` for the CodeRabbit request.
`coderabbit review` has standing approval (`CLAUDE.md`, "Tool Execution
Safety"). Anything else that touches the network is asked for first.

## Part A — prepare the branch

1. Check the working tree with `git status --short`. The only uncommitted change
   allowed is the reviewer's approval of PREREQ: the Document Status table of a
   PREREQ document set to `approved`, with its review date, reviewer, and
   comments. Any other change, or any change when PREREQ is none, is a reason
   to stop and report. An allowed change stays uncommitted for now: it is
   committed together with the new document, so the PR records the approval and
   the document that depends on it.
2. `git fetch origin main`.
3. If the current branch's name follows the feature branch convention
   (`_context.md`) for TASK_DIR and `git rev-list --count origin/main..HEAD` is
   `0`, use it (the branch `/runchange` switched to after the previous merge).
4. Otherwise create a new branch from `origin/main`:
   - Get the GitHub user name with `gh api user --jq .login`.
   - Choose `NN` one above the highest sequence number this task's branches have
     used, looking at local branches, `origin` branches, and the head branches
     of all PRs (`gh pr list --state all --limit 1000 --json headRefName`); merged
     branches are deleted, so the PR list is what remembers them. With none,
     `NN` is `01`.
   - `git switch -c <branch> origin/main`. The allowed uncommitted change from
     step 1 comes along.

## Part B — review, open the PR, and hand it over

Run this after the calling command's last commit, with a clean working tree.

1. Review the branch with the local CodeRabbit CLI: follow
   `.claude/commands/_lib/coderabbit-local-review.md` with
   **BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
   **SUMMARY** = REPORT.
2. Run the documentation guards:
   `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline`.
   Fix and commit if they fail.
3. Push and create the PR as `.claude/commands/_lib/pr-review-merge-loop.md`
   step 1 describes for `CREATE_PR`, without pausing for the user to adjust the
   title or body (the reviewer can edit them on GitHub). The title is the title
   of the commit that added the document. The body:
   - summarizes the document, and states which PREREQ documents the PR approves;
   - lists the decisions made with the reviewer during this run, if any;
   - names the points the reviewer should check, including any `unclear` or
     `deferred` findings from step 1;
   - contains `Refs #<issue>` for the issue the requirements document names as
     its own (`対応 issue`), never `Closes`: the issue stays open until the task is
     implemented.
4. Ask CodeRabbit to review: `gh pr comment <number> --body "@coderabbitai review"`.
5. Ping the human reviewer (`_context.md`, "Reviewer ping") with the PR URL and
   the next action: review the PR, then run `/runchange <PR>`.
6. In REPORT, give the PR URL, the review results, and the same next action.
   Then stop: do not wait for CodeRabbit and do not merge.
