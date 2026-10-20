---
description: "Drive a PR through CodeRabbit and CI, record its bookkeeping, merge it, and run the next command"
argument-hint: "[PR | branch | task]"
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

**Input.** `$ARGUMENTS`, when given, is a PR number, a branch name, or a task ID
(e.g. `0015`); with no argument, use the current branch.

**Notes.** `fixpr.md` and `mergepr.md` each perform network operations; each has
its own authorization note — follow it. In particular, `mergepr.md` requires the
user's explicit approval of the squash commit message (step 4 below asks it).

## 1. Resolve the PR, and create it for a standalone change

- Resolve `$ARGUMENTS`:
  - a PR number or a branch: `gh pr view "$ARGUMENTS" --json
    number,url,headRefName`. Pass the argument only when it is set — an empty
    argument would replace `gh pr view`'s current-branch default and fail;
  - a task ID: the current branch's PR, or the open PR whose head branch is
    named for that task (`gh pr list --state open --json number,headRefName,url`,
    then match).
- With no argument, use the current branch:
  `gh pr view --json number,url,headRefName`.
- If a PR is open, use its number in the steps below.
- If none exists, it is a standalone change. Unless `$ARGUMENTS` names a branch
  other than the checked-out one (the review reads the local checkout), review
  it locally first: follow `.claude/commands/_lib/coderabbit-local-review.md`
  with **BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
  **SUMMARY** = the final report. Then set `<target-branch>` to the branch to
  open the PR from (`$ARGUMENTS` when it names a branch, otherwise
  `git rev-parse --abbrev-ref HEAD`), push it, and create the PR naming the head
  branch explicitly (a branch argument other than the checked-out branch must
  not be ignored):

  ```bash
  git push -u origin <target-branch>
  gh pr create --base main --head <target-branch> --title "<title>" --body-file <body-file>
  ```

  Draft a conventional-commit title (`<type>(<scope>): <summary>`, matching the
  style of `git log origin/main`) and a short body from the branch's commits,
  describing the final state, not the history. Write the body to a file with
  the Write tool. Report the URL; let the user adjust the title or body before
  continuing.
- An open PR whose branch does not follow the feature branch convention
  (`_context.md`) was opened by hand, so no command reviewed it locally: run the
  local review above on it too (when its branch is checked out), and push the
  fixes to it.
- Check out the PR's head branch before making any change, so `fixpr`'s commit
  and push land on the PR instead of on whatever branch the worktree has out:
  `gh pr checkout <number>`. (Skip it when the head branch is already current.)

## 2. Decide the PR's bookkeeping and next command

Read them from what the opening command recorded, not from guesses about the
change:

- **Plan PR** — a task's `03_implementation_plan.md` on the PR's branch has this
  PR's URL on a checkpoint's `PR を作成した` line (`runplan.md` step 5a writes it):
  - **PRE_MERGE** = mark that checkpoint's "PR がマージされた" and
    "次のブランチへ切り替えた" as `[x]` (step 5 switches branch right after the
    merge), and the PR's `PR-N マージ済み` in the plan's PR checklist (§6);
    commit once as `docs(<task-id>): record PR-N as merged`. The bookkeeping
    then reaches `main` with the merge, including on the task's final PR.
  - **NEXT** = `/runplan <task>` when a later `### PR-N 作成ポイント` marker
    remains unchecked; none on the task's final PR.
- **Plan PR without markers** — a single-phase plan's PR (`runplan.md` step 8):
  PRE_MERGE and NEXT are none.
- **Task-document PR** — otherwise, when the PR adds or changes task documents
  (`01_requirements.md`, `02_architecture.md`, `03_implementation_plan.md`)
  whose status is `draft`:
  - **PRE_MERGE** = for each of them, set its Document Status to `approved`,
    `Review date` to today, `Reviewer` to the user's GitHub name
    (`gh api user --jq .login`), and keep `Comments` (or `-`); commit alone as
    `docs(<task-id>): approve the <document>`. This is the only way the commands
    set a document to `approved`, and only on the user's answer
    (`docs/dev/developer_guide/requirements_process.md`, "LLM Constraints").
  - **NEXT** = by the latest document approved: `01_requirements.md` →
    `/mkarch <task>`; `02_architecture.md` → `/mkplan <task>`;
    `03_implementation_plan.md` → `/runplan <task>`.
- **Anything else** (a standalone change): PRE_MERGE and NEXT are none.

## 3. Wait for CodeRabbit and CI, running fixpr as comments appear

First make sure CodeRabbit reviews the head commit, including commits the
reviewer pushed after the PR was opened. CodeRabbit normally reviews each push
on its own, so poll `statusCheckRollup` (below) for about two minutes; only if
no `CodeRabbit` status context appears (a `StatusContext` entry; a check run of
that name does not count) ask for a review with
`gh pr comment <number> --body "@coderabbitai review"`.

Poll until CodeRabbit has finished and nothing is left to fix.

- Read CodeRabbit's status from the PR's **status contexts**, not from
  `gh pr checks`:

  ```bash
  gh pr view <number> --json statusCheckRollup
  ```

  Find the entry whose `context` (a `StatusContext`) or `name` is `CodeRabbit`
  and read its `state` (`PENDING` → `SUCCESS` / `FAILURE` / `ERROR`). `gh pr checks` can
  report a separate check run whose conclusion is `pass` with the description
  `Review skipped` while the review is still in progress, so it is not a reliable
  signal. Wait between polls rather than spinning.
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
- **While any thread is unresolved**, run `.claude/commands/fixpr.md` for the PR
  when there is an unresolved thread ID you have not yet passed to `fixpr`
  (including one that appears while CodeRabbit is still `PENDING`). Record the set
  of unresolved thread IDs each time; re-run `fixpr` when the set holds an ID not
  seen before — not merely when the count rises, since resolving one thread while
  another is added keeps the count the same. `fixpr` pushes a fix, so CodeRabbit
  re-reviews: keep polling.
- When CodeRabbit's state is `SUCCESS` and either no thread is unresolved or
  `fixpr` reported that every remaining thread is at its severity floor
  (`no-harm`; `fixpr.md`'s convergence policy defines the floor), wait for CI.
  If a remaining unresolved thread is not at the floor, report it and stop rather
  than merge. If CodeRabbit's state is `FAILURE` or `ERROR`, report and stop.
- If no `CodeRabbit` status context ever appears after a reasonable wait, report
  that and ask the user how to proceed rather than assuming green.
- Wait for the PR's checks on the head commit to finish:
  `gh pr checks <number> --watch --fail-fast`. (Its `CodeRabbit` row is not the
  review signal; the status context above is.) If a check fails, report the
  failing check and its log URL and stop rather than merge. A fix pushed
  afterwards starts a new review, so run this command again.

## 4. Confirm, record, and merge

Ask the user one question, so a reviewer who has already reviewed the PR answers
once rather than once per decision.

1. Run `.claude/commands/mergepr.md` steps 1–2 (prepare, draft the squash commit
   message). When PRE_MERGE is set, the message says what it records (e.g.
   "Approves 01_requirements.md").
2. Ask with `AskUserQuestion`. In your message, show what `mergepr.md` step 3
   requires (PR URL, base branch, CI result, the full subject and body), the
   PRE_MERGE change (which lines of which file, and for an approval the
   reviewer name and date it writes), and NEXT (for `/runplan`, the next PR
   marker's `実装モデル要件`, so the user can decide whether this session's model
   fits). Offer, as applicable:
   - record PRE_MERGE, merge, and run NEXT (recommended);
   - record PRE_MERGE and merge, without running NEXT;
   - merge without PRE_MERGE (for an approval: the document stays `draft`, and
     NEXT is not offered, since it requires the approval);
   - revise the message, or stop.

   An answer that merges is the explicit approval `mergepr.md` step 3 asks for.
   Do not merge on anything less.
3. When the answer records PRE_MERGE: copy the approved subject and body files
   out of the prepared directory (to the scratchpad), run
   `mergepr discard --state <state-file>`, make the PRE_MERGE commit, push it,
   and run `mergepr prepare` again (it waits for CI on the new head). Confirm
   that `git diff --name-only <previous headRefOid> <new headRefOid>` lists only
   the files PRE_MERGE changes, then merge with the approved message files
   without asking again: the answer approved this message for exactly this
   head. Do not wait for a CodeRabbit review of that commit; it changes only
   status or checkbox lines.
4. Merge with `mergepr.md` step 4, and report as its step 5 says.

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
