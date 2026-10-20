---
description: "Drive a PR through CodeRabbit and CI, record its bookkeeping, merge it, and run the next command"
argument-hint: "[#PR | branch | task]"
---

> **Project context (read first)**: Read `.claude/commands/_context.md`. The
> fix and merge mechanics are in `.claude/commands/fixpr.md` and
> `.claude/commands/mergepr.md`; follow them rather than copying their content
> here. The local CodeRabbit review is
> `.claude/commands/_lib/coderabbit-local-review.md`.

Drive one PR through CodeRabbit review and CI, then ask the user one question
that covers the bookkeeping, the merge, and the next command, and carry it out.

This is the second half of a task PR. The command that opened it (`mkreq`,
`mkarch`, `mkplan`, `mkplan2`, or `runplan`; `_lib/task-pr-handoff.md`)
reviewed it locally and handed it to the human reviewer, who runs this command
after reviewing it. It also drives a standalone change committed on the current
branch without a task (a code change, a development-environment file), and
creates the PR for it.

**Input.** `$ARGUMENTS`, when given, is one of:

- a task ID — a task directory name under `docs/tasks/` on `origin/main` or on
  an open PR's head branch, given as its four-digit number (`0015`) or full name
  (`0015_output_layout`). Check this form first;
- a PR number — `#389` or `389` (a number that is not a task ID);
- a branch name.

With no argument, use the current branch.

**Authorization.** Invoking this command approves, for this PR only, the
network operations it names: `git fetch origin main`, `git pull --ff-only`,
`git push` (including `-u`), `gh pr view`, `gh pr list`, `gh pr checkout`,
`gh pr checks`, `gh pr create` (standalone change only), `gh pr comment` (the
CodeRabbit request), `gh api graphql` (review threads), `gh api user`, and, for
the deferred findings the user moves to an issue in step 4, `gh issue list`,
`gh issue create`, `gh issue comment`, and the thread replies and resolutions
`fixpr.md` Phase 5 makes. It also covers what `fixpr.md` and `mergepr.md`
authorize when run from here, and the next command (NEXT) when the user chooses
to run it in step 4. It does **not** approve the merge itself: only the user's
answer in step 4 does (`CLAUDE.md`, "Tool Execution Safety"). `coderabbit
review` has standing approval.

Every `fixpr.md` and `mergepr.md` run below is given the resolved PR number
(`<number>`), never `$ARGUMENTS`, which may be a task ID or a branch.

## 1. Resolve the PR, and create it for a standalone change

- `git fetch origin main`.
- Resolve the PR:
  - a task ID: the current branch's PR when its head branch is named for that
    task, otherwise the open PR whose head branch is
    (`gh pr list --state open --json number,headRefName,url`, then match the
    feature branch convention in `_context.md`). Stop and report when none or
    more than one matches;
  - a PR number: `gh pr view <number> --json number,url,state,headRefName`;
  - a branch: `gh pr view <branch> --json number,url,state,headRefName`;
  - no argument: `gh pr view --json number,url,state,headRefName` (passing an
    empty argument would replace the current-branch default and fail).
- By the PR's `state`:
  - `OPEN`: use its number in the steps below.
  - `MERGED`: do not open another PR for the same commits. If `mergepr`
    cleanup is pending, run `mergepr cleanup --state <state-file>`. The head
    branch is gone, so read the bookkeeping from `origin/main`: report any step
    2 record missing there (it can be added with a standalone PR), and stop.
  - `CLOSED`: report and stop.
- If none exists, it is a standalone change:
  - Stop and report when the branch to open it from (`$ARGUMENTS` when it names
    a branch, otherwise the current branch) is `main`, a detached `HEAD`, or has
    no commits beyond `origin/main` (`git rev-list --count origin/main..<branch>`).
  - When that branch is the checked-out one, review it locally first: follow
    `.claude/commands/_lib/coderabbit-local-review.md` with **BASE** =
    `$(git merge-base origin/main HEAD)`, **SCOPE** = branch, **SUMMARY** = the
    final report. (The review reads the local checkout, so skip it for another
    branch.)
  - Push it and create the PR naming the head branch explicitly, so a branch
    argument other than the checked-out branch is not ignored:

    ```bash
    git push -u origin <target-branch>
    gh pr create --base main --head <target-branch> --title "<title>" --body-file <body-file>
    ```

    Draft a conventional-commit title (`<type>(<scope>): <summary>`, matching
    the style of `git log origin/main`; English, since the squash commit
    subject is built from it) and a short Japanese body from the branch's
    commits, describing the final state, not the history. Write the body to a
    file with the Write tool. Do not pause for the user to adjust them; they can
    edit them on GitHub, and step 4 is the one question.
- Check out the PR's head branch and bring it up to date before making any
  change, so `fixpr`'s and step 4's commits land on the PR's latest head:
  `gh pr checkout <number>` (even when the branch is already checked out), then
  `git pull --ff-only`. A reviewer may have pushed since the branch was last
  checked out. If the branch is checked out in another worktree, the checkout
  fails: stop and tell the user to run this command in that worktree.

## 2. Decide the PR's bookkeeping and next command

Read them from what the opening command recorded on the PR's branch, not from
guesses about the change. Run this step again whenever step 3 or 4 changes the
head; a record already made counts as made, never as absent.

- **Kind.**
  - **Plan PR** — a task's `03_implementation_plan.md` has this PR's URL on a
    `PR を作成した` line (`runplan.md` writes it, at a PR checkpoint or, for a
    plan without PR markers, in its single-PR record). It is the task's
    **final PR** when no `### PR-N 作成ポイント` marker after it is unchecked.
  - **Task-document PR** — not a plan PR, and the PR adds or changes a task
    document (`01_requirements.md`, `02_architecture.md`,
    `03_implementation_plan.md`), whatever its status.
  - **Standalone change** — anything else.
- **PRE_MERGE** — what step 4 commits just before the merge, in one commit
  (`docs(<task-id>): record the review of PR #<number>`). It leaves out what is
  already recorded; when nothing is left, it is none.
  - **Approvals** (any kind): every task document the PR adds or changes whose
    status is `draft` (including one `fixpr` returned to `draft` as a decision
    change). Set its Document Status to `approved`, `Review date` to today,
    `Reviewer` to the user's GitHub name (`gh api user --jq .login`), and keep
    `Comments` (or `-`). This is the only way the commands set a document to
    `approved`, and only on the user's answer
    (`docs/dev/developer_guide/requirements_process.md`, "LLM Constraints").
  - **Checkpoint** (plan PR): "PR がマージされた" on that PR's record, and in
    the plan's PR checklist (§6) its `PR-N マージ済み`. On a checkpoint, also
    "次のブランチへ切り替えた": `[x]` when NEXT is set (step 5 creates the next
    branch only then), `[-]` (not applicable) on the final PR.
- **NEXT** — the command offered after the merge:
  - plan PR: `/runplan <task>` unless it is the final PR; none on the final
    PR;
  - task-document PR: only for a document this PR's PRE_MERGE approves (one
    that was `draft` here), by the latest of them: `01_requirements.md` →
    `/mkarch <task>`; `02_architecture.md` → `/mkplan <task>`;
    `03_implementation_plan.md` → `/runplan <task>`. None when the next
    command's document already exists on `origin/main` (for `/runplan`, when
    the plan has no unchecked step left), and none when the user leaves the
    document `draft`. An editorial PR on an approved document therefore offers
    nothing;
  - standalone change: none.

## 3. Wait for CodeRabbit and CI, running fixpr as comments appear

First make sure CodeRabbit reviews the head commit, including commits the
reviewer pushed after the PR was opened. CodeRabbit normally reviews each push
on its own, so poll `statusCheckRollup` (below) for about two minutes; only if
no `CodeRabbit` status context appears, ask for a review with
`gh pr comment <number> --body "@coderabbitai review"`.

Poll until CodeRabbit has finished and nothing is left to fix.

- Read CodeRabbit's status from the PR's **status contexts**, not from
  `gh pr checks`:

  ```bash
  gh pr view <number> --json statusCheckRollup
  ```

  Use only the `StatusContext` entry whose `context` is `CodeRabbit`, and read
  its `state` (`PENDING` → `SUCCESS` / `FAILURE` / `ERROR`). Ignore a
  `CheckRun` named `CodeRabbit`: it can report `pass` with the description
  `Review skipped` while the review is still in progress, which is also why
  `gh pr checks` is not the signal. Wait between polls rather than spinning.
- Fetch the unresolved review threads, following every page:

  ```bash
  gh api graphql -F owner=<owner> -F repo=<repo> -F number=<number> -f query='
    query($owner:String!, $repo:String!, $number:Int!, $after:String) {
      repository(owner:$owner, name:$repo) {
        pullRequest(number:$number) {
          reviewThreads(first:100, after:$after) {
            pageInfo { hasNextPage endCursor }
            nodes {
              id
              isResolved
              comments(first:10) {
                nodes { databaseId author { login } path line url body }
              }
            }
          }
        }
      }
    }'
  ```

  Filter to `isResolved=false`, and repeat the call with `-F after=<endCursor>`
  from the previous page until `hasNextPage` is false; otherwise a thread past the
  first 100 would be left open while the flow proceeds to merge. (Each thread's
  comments are capped at the first 10, as `fixpr.md` also notes; read more only
  when a thread needs it.)
- **While any thread is unresolved**, run `.claude/commands/fixpr.md` for
  `<number>` when there is an unresolved thread ID you have not yet passed to
  `fixpr` (including one that appears while CodeRabbit is still `PENDING`).
  Record the set of unresolved thread IDs each time; re-run `fixpr` when the
  set holds an ID not seen before — not merely when the count rises, since
  resolving one thread while another is added keeps the count the same.
  `fixpr` pushes a fix, so CodeRabbit re-reviews: keep polling.
- When CodeRabbit's state is `SUCCESS` and no unseen thread is left, look at the
  unresolved threads `fixpr` reported:
  - `deferred` (past its round cap): keep them for step 4;
  - anything else — `must-fix`, `unclear`, a valid fix not applied, a thread a
    person opened — stop rather than merge, and report each one with its URL
    and why it is open, so the user can fix, re-classify, or answer it.
  Also stop and report when CodeRabbit's state is `FAILURE` or `ERROR`.
- If no `CodeRabbit` status context ever appears after a reasonable wait, report
  that and ask the user how to proceed rather than assuming green.
- Wait for the PR's checks on the head commit to finish:
  `gh pr checks <number> --watch --fail-fast`. If a check fails, report the
  failing check and its log URL and stop rather than merge. A fix pushed
  afterwards starts a new review, so run this command again.

## 4. Confirm, record, and merge

Ask the user once, so a reviewer who has already reviewed the PR answers once
rather than once per decision.

1. Run `.claude/commands/mergepr.md` steps 1–2 for `<number>` (prepare, draft
   the squash commit message). When PRE_MERGE has approvals, draft the message
   with a line naming them (e.g. "Approves 01_requirements.md"), and keep a
   second version without that line for the option that merges without them.
   For the task's final plan PR, the body ends with `Closes #<issue>` for the
   issue the requirements document names as its own (`対応 issue`); a task
   without one gets no such line.
2. Ask with one `AskUserQuestion` call. In your message, show what
   `mergepr.md` step 3 requires (PR URL, base branch, CI result, the full
   subject and body, and the version without the approval line when it
   applies), the PRE_MERGE change (which lines of which files, and for an
   approval the reviewer name and date it writes), the `deferred` threads (each
   with its number, topic, what fixing it would change, and its URL), and NEXT
   (for `/runplan`, the next PR marker's `実装モデル要件`, so the user can decide
   whether this session's model fits). Each question has two to four options:
   - **Deferred threads** (only when there are some): with one to three,
     `multiSelect` with one option per thread plus "fix none"; with more,
     single-select "fix all" / "fix none", and a subset by number in the
     free-text answer.
   - **Merge**, applying when nothing is chosen to fix. Offer those of these
     that apply, and always the last:
     - record PRE_MERGE, merge, and run NEXT (recommended; only when NEXT is
       set);
     - record PRE_MERGE and merge, without running NEXT (or "merge" when
       PRE_MERGE is none);
     - merge without the approvals (only for a task-document PR whose PRE_MERGE
       has approvals): the documents stay `draft`, the message without the
       approval line is used, NEXT is none, and step 5 detaches. A plan PR is
       never merged without its PRE_MERGE: its record would be missing on
       `main`, and implementation would merge under a `draft` document;
     - do not merge (revising the message goes through the free-text answer).

   An answer that merges is the explicit approval `mergepr.md` step 3 asks for.
   Do not merge on anything less. When the answer does not merge, run
   `mergepr discard --state <state-file>` and stop.

   When the answer chooses deferred threads to fix now, run
   `mergepr discard --state <state-file>`, run `fixpr.md` for `<number>` with
   those thread IDs as its caller override (pass them in the instructions you
   follow; `fixpr` reads them from this session), and return to step 2; then
   ask step 4 again. Do not merge on this answer: the fix changes the head, so
   the message and the merge are confirmed again on the new head.

   When the answer merges, first move the deferred threads not chosen to an
   issue, so none is silently dropped. Look for this PR's tracking issue
   (`gh issue list --state open --search "PR #<number> のレビュー指摘の追跡 in:title"`).
   Add the findings to it as a comment (`gh issue comment`), or create it
   (`gh issue create --title "PR #<number> のレビュー指摘の追跡" --body-file <file>`)
   when there is none. Write in Japanese, listing each finding's topic, the
   change it asks for, and its thread URL. Then reply to each of those threads
   in Japanese with the issue URL and resolve it, as `fixpr.md` Phase 5 does.
   Then continue below.
3. When the answer records PRE_MERGE:
   - Note the prepared `headRefOid`, and copy the approved subject and body
     files to the scratchpad (`mergepr.md` keeps them in the prepared
     directory, which `discard` deletes). Then run
     `mergepr discard --state <state-file>`.
   - Make the PRE_MERGE commit and push it. If the push is rejected (someone
     pushed meanwhile), `git pull --rebase` the bookkeeping commit onto the new
     head and return to step 2: the new commits need review, and step 4 asks
     again.
   - Wait until `gh pr view <number> --json headRefOid` reports the pushed
     commit (`git rev-parse HEAD`), so `prepare` does not read the old head.
   - Run `mergepr prepare <number>` again. If it fails (a check fails on the
     new head), report and stop; a rerun of this command finds PRE_MERGE
     already recorded.
   - Confirm that `git diff --name-only <noted headRefOid> HEAD` lists only the
     files PRE_MERGE changes; otherwise discard the preparation and stop.
   - Wait for CodeRabbit's status context on the new head to reach `SUCCESS`
     (step 3), then fetch the unresolved threads again. If a new thread
     appeared on the bookkeeping commit, do not merge: discard the preparation,
     return to step 2, and ask step 4 again.
   - Otherwise merge with the approved message files without asking again: the
     answer approved this message for exactly this head.
4. Merge with `mergepr.md` step 4, and report as its step 5 says. If the merge
   is refused because the head moved, discard the preparation and return to
   step 2 rather than re-running `mergepr.md` from its step 1: the new commits
   need review, and step 4 asks again.

## 5. Switch branch, then run NEXT

Once `mergepr.md` reports the merge:

- Confirm its cleanup finished. `mergepr.md` can stop after the merge with
  cleanup pending, in which case run `mergepr cleanup --state <state-file>` and
  only then continue. When another worktree has the base branch checked out,
  the cleanup leaves the local base branch stale; do not rely on it.
- `git fetch origin main`. When NEXT is set, create the next branch from
  `origin/main` and name it by `_lib/task-pr-handoff.md` Part A step 4; NEXT
  (now or later) works on it (Part A step 3). Never branch from the local base
  branch or the merged branch (after a squash merge the merged branch's
  commits are not ancestors of the base). When NEXT is none, detach at
  `origin/main` (`git switch --detach origin/main`); do not check out the base
  branch, which may be checked out in another worktree.
- If the answer in step 4 chose to run NEXT, run it now. It opens its own PR,
  pings the reviewer, and stops; this command ends there. Otherwise report the
  final status.
