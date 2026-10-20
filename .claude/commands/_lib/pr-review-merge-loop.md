---
description: "Internal: shared PR review/merge/branch-switch loop (not a command)"
---

# Shared pattern: the per-PR review, merge, and branch-switch loop

`runpr.md` and `runchange.md` both drive one PR through CodeRabbit review,
CI, `fixpr`, `mergepr`, and the post-merge branch switch; they differ only in
how the PR is created and what the calling command does after the merge. They
are the second half of a task PR: the command that opened it (`runplan.md` or
a task-document command, `_lib/task-pr-handoff.md`) reviewed it locally and
handed it to the human reviewer, who runs one of them after reviewing. This file
defines the common procedure once. A calling command invokes it by saying
"follow `_lib/pr-review-merge-loop.md` for the PR" and supplying the inputs
below.

Project-specific values (build checks, green gate, PR merge method) are in
`.claude/commands/_context.md`. The fix and merge mechanics are in
`.claude/commands/fixpr.md` and `.claude/commands/mergepr.md`. Those are project
files — follow them rather than copying their content here.

## Inputs the calling command must supply

- **CREATE_PR** — `true` when the calling command should create the PR for the
  target branch when none exists yet (`runchange.md`, for a change no command
  opened a PR for); `false` when the PR already exists (`runpr.md`;
  `runplan.md` step 5a creates it).
- **PRE_MERGE** — the bookkeeping commit the calling command proposes to make
  just before the merge, on the user's answer in step 3 (`runchange.md`: set
  the PR's draft task documents to `approved`; `runpr.md`: mark the plan's PR
  checkpoint as merged). None when there is nothing to record.
- **NEXT** — the command the calling command offers to run after the merge, for
  the same task (`runchange.md`: the next task-document command or `/runplan`;
  `runpr.md`: `/runplan` for the next PR). None when the task has no next
  step.

The PR to operate on is `$ARGUMENTS` when given, otherwise the current branch's
PR; the calling command states which argument forms it accepts.

## Procedure

Repeat steps 1–4 for the PR.

### 1. Resolve the PR, and create it when CREATE_PR

- Resolve `$ARGUMENTS`:
  - a PR number or a branch: `gh pr view "$ARGUMENTS" --json
    number,url,headRefName`. Pass the argument only when it is set — an empty
    argument would replace `gh pr view`'s current-branch default and fail;
  - a task ID (when the calling command accepts one): the current branch's PR, or
    the open PR whose head branch is named for that task
    (`gh pr list --state open --json number,headRefName,url`, then match).
- With no argument, use the current branch:
  `gh pr view --json number,url,headRefName`.
- If a PR is open, use its number in the steps below.
- If none exists:
  - with `CREATE_PR`, set `<target-branch>` to the branch to open the PR from
    (`$ARGUMENTS` when it names a branch, otherwise
    `git rev-parse --abbrev-ref HEAD`), confirm the base branch (`main`), push
    that branch and set its upstream, and create the PR naming the head branch
    explicitly (a branch argument other than the checked-out branch must not be
    ignored):

    ```bash
    git push -u origin <target-branch>
    gh pr create --base <base> --head <target-branch> --title "<title>" --body-file <body-file>
    ```

    Draft a conventional-commit title (`<type>(<scope>): <summary>`, matching the
    style of `git log origin/<base>`) and a short body from the branch's commits,
    describing the final state, not the history. Write the body to a file with
    the Write tool and pass it with `--body-file`, so shell metacharacters in the
    body are not interpreted. Report the URL; let the user adjust the title or
    body before continuing.
  - without `CREATE_PR`, report that no PR exists and stop.
- Check out the PR's head branch before making any change, so `fixpr`'s commit
  and push land on the PR instead of on whatever branch the worktree has out:
  `gh pr checkout <number>`. (Skip it when the head branch is already current.)
  Do this in step 1, before the step 2 `fixpr`.

### 2. Wait for CodeRabbit, running fixpr as comments appear

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
  (`no-harm`; `fixpr.md`'s convergence policy defines the floor), go to step 2a.
  If a remaining unresolved thread is not at the floor, report it and stop rather
  than merge. If its state is `FAILURE` or `ERROR`, report and stop.
- If no `CodeRabbit` status context ever appears after a reasonable wait, report
  that and ask the user how to proceed rather than assuming green.

### 2a. Wait for CI

Wait for the PR's checks on the head commit to finish:
`gh pr checks <number> --watch --fail-fast`. (Its `CodeRabbit` row is not the
review signal; step 2 read that from the status context.) If a check fails,
report the failing check and its log URL and stop rather than merge. A fix
pushed afterwards starts a new review, so run the calling command again.

### 3. Confirm, record, and merge

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

### 4. Switch branch, then run NEXT

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
- If the answer in step 3 chose to run NEXT, run it now. It opens its own PR,
  pings the reviewer, and stops; this loop ends there.
