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
`.claude/commands/_lib/coderabbit-local-review.md`.

These commands do not wait for the review or the merge. The reviewer reviews the
PR and then runs `/runpr <PR>` by hand, which drives it to the merge. It waits for CodeRabbit and CI, resolves the threads, and then asks
one question that covers the merge, the bookkeeping committed just before it
(for a task-document PR, setting the document to `approved`), and the next
command to run (`runpr.md` steps 2 and 4).

## Inputs the calling command must supply

- **TASK_DIR** — the target task directory under the task root.
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
- **ISSUE** (`mkreq.md` only) — the task's issue number. Every other caller
  leaves it out, and Part A reads it from the `対応 issue` line of TASK_DIR's
  `01_requirements.md` on `origin/main`
  (`git show origin/main:<TASK_DIR>/01_requirements.md`); a task whose
  requirements name no issue has none.
- **AFTER_CREATE** (optional) — what the calling command does once the PR
  exists, before CodeRabbit is asked (`runplan.md`: mark `PR を作成した` with the
  PR URL, commit, and push).

## Authorization

Invoking a calling command approves, for its task and its PR only, the network
operations this file names: `gh api user`, `gh pr list`, `gh pr view`,
`git fetch origin main`, `gh issue develop` and the issue operations of
"Choosing the issue to link" (Part A step 4, for the task's issue and its link
sub-issues only), `git push` (including `-u`), `gh pr create`, and
`gh pr comment` for the CodeRabbit request. `coderabbit review` has standing approval (`CLAUDE.md`,
"Tool Execution Safety"). Anything else that touches the network is asked for
first.

## Part A — prepare the branch

The calling command runs Part A before it reads any task document, so it reads
the documents (their status, the plan's checkboxes) from the branch the work
lands on, not from a stale checkout.

1. Check the working tree with `git status --short`. It must be clean; stop and
   report otherwise. (Approving the previous document is not a working-tree
   edit: `/runpr` commits it at the merge, on the reviewer's answer.)
2. `git fetch origin main`.
3. Use the current branch when its name follows the feature branch convention
   (`_context.md`) for TASK_DIR and either:
   - the branch has no commits of its own (`HEAD` is an ancestor of
     `origin/main`: the branch `/runpr` created after the previous merge, even
     if more was merged since); fast-forward it with
     `git merge --ff-only origin/main`; or
   - the calling command is `runplan.md`, the branch has commits beyond
     `origin/main`, none of them a task-document command's commit (a subject
     starting `docs(<task-id>): draft` or a PR-boundary commit of `mkplan2.md`),
     and it has no PR yet (`gh pr view <branch>` finds none): the PR group in
     progress, continued with its next phase group.

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
   - When the task has an issue (ISSUE), create the branch as one linked to
     it, so the PR opened from it appears in the issue's Development section
     without a closing keyword in its body. An issue holds at most ten linked
     pull requests, so first choose the issue to link (LINK_ISSUE) by
     "Choosing the issue to link" below, then run
     `gh issue develop <LINK_ISSUE> --name <branch> --base main --checkout`. This
     creates the branch on `origin` at the head of `main` and checks it out
     with upstream tracking. The repository's "Auto-close issues with merged
     linked pull requests" setting is off, so merging such a PR leaves the
     issue open; `/runpr` closes it after the task's final plan PR
     (`runpr.md` step 5). Confirm `git rev-parse HEAD` equals
     `git rev-parse origin/main` after a `git fetch origin main`; if `main`
     moved in between, fast-forward with `git merge --ff-only origin/main`.
   - Without an issue: `git switch -c <branch> origin/main`.

### Choosing the issue to link

A task usually needs more than ten PRs only when its plan has many PR
checkpoints. The PRs beyond ten are linked to a sub-issue of ISSUE made for
holding them (a **link sub-issue**), not to ISSUE itself.

1. Count an issue's links: its linked PRs, open or closed, plus its linked
   branches that have no PR yet (a branch's entry is replaced by its PR's
   once the PR is opened):

   ```bash
   gh api graphql -F owner='{owner}' -F repo='{repo}' -F n=<issue> -f query='
     query($owner: String!, $repo: String!, $n: Int!) {
       repository(owner: $owner, name: $repo) {
         issue(number: $n) {
           closedByPullRequestsReferences(first: 20, includeClosedPrs: true) {
             nodes { headRefName }
           }
           linkedBranches(first: 20) { nodes { ref { name } } }
         }
       }
     }'
   ```

   The count is the PR nodes plus the branch nodes whose name is no PR
   node's `headRefName`.
2. When ISSUE has fewer than ten links, LINK_ISSUE is ISSUE.
3. Otherwise list ISSUE's sub-issues
   (`gh api repos/{owner}/{repo}/issues/<issue>/sub_issues --paginate`) and
   take the open link sub-issues, those titled `#<issue> の PR リンク (<k>)`.
   LINK_ISSUE is the one with the highest `<k>` when it has fewer than ten
   links (step 1).
4. When there is none, or that one is full, create the next one with `<k>` one
   above the highest (`2` for the first) and add it as a sub-issue of ISSUE:

   ```bash
   gh issue create --title "#<issue> の PR リンク (<k>)" --body-file <file>
   gh api -X POST repos/{owner}/{repo}/issues/<issue>/sub_issues \
     -F sub_issue_id="$(gh api repos/{owner}/{repo}/issues/<new> --jq .id)"
   ```

   The body, in Japanese, says that the issue exists only to hold the task's
   PRs beyond the ten the parent issue can link, that the task is tracked on
   the parent issue, and that `/runpr` closes it together with the parent.
   `sub_issue_id` is the issue's numeric `id`, not its number. LINK_ISSUE is
   the new issue.

## Part B — open the PR and hand it over

Run this after the calling command's last commit, with a clean working tree.

1. When LOCAL_REVIEW is `run`, review the branch with the local CodeRabbit CLI:
   follow `.claude/commands/_lib/coderabbit-local-review.md` with
   **BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
   **SUMMARY** = REPORT.
2. Run CHECKS. If they fail, fix, commit, and run them again; do not go on to
   the push until they pass.
3. Push and create the PR for the current branch. Ignore the calling command's
   `$ARGUMENTS` here (it is a task or an issue, not a PR or a branch): run
   `git push -u origin <current-branch>` and
   `gh pr create --base main --head <current-branch> --title "<title>" --body-file <file>`,
   writing the body with the Write tool. Do not pause for the user to adjust the
   title or body; the reviewer can edit them on GitHub. The title is PR_TEXT's,
   in English (the squash commit subject is built from it). The body is in
   Japanese (`CLAUDE.md`, "Language"):
   - summarizes the change;
   - lists the decisions made with the reviewer during this run, if any;
   - names the points the reviewer should check (for `runplan.md`, the
     `レビュー観点`), including every `unclear` or `deferred` finding of the
     local reviews: this run's, and those earlier sessions recorded in commit
     messages on the branch (`git log origin/main..HEAD`, the
     `CodeRabbit local review:` lines `_lib/coderabbit-local-review.md` step 5
     writes);
   - contains `Refs #<issue>` for the issue the requirements document names as
     its own (`対応 issue`), never a closing keyword (`Closes`, `Fixes`,
     `Resolves`): the issue stays open until the task is implemented. The
     Development link comes from the branch (Part A step 4), not from the
     body. A task whose requirements name no issue gets no such line.
4. Run AFTER_CREATE, if supplied.
5. Make sure CodeRabbit reviews the PR. Poll for about two minutes; if the
   head commit is not reviewed and CodeRabbit's status is not `PENDING`
   (`runpr.md` step 3, "Reviewed": a skipped review also reports
   `SUCCESS`), ask for a review with
   `gh pr comment <number> --body "@coderabbitai review"`. Do not wait for it
   to finish; `/runpr` does.
6. Ping the human reviewer (`_context.md`, "Reviewer ping") in Japanese with the
   PR URL and the next action: review the PR, then run `/runpr <PR>`. If the ping tool is
   unavailable or the ping is not sent, the report in step 7 carries the same
   message.
7. In REPORT, give the PR URL, the review results, and the same next action.
   Then stop: do not wait for CodeRabbit and do not merge.
