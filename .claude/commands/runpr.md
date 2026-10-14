> **Project context (read first)**: Read `.claude/commands/_context.md` — it is
> canonical for every project-specific path, build check, green gate, and helper
> this command names. The review and merge mechanics are defined in
> `.claude/commands/fixpr.md` and `.claude/commands/mergepr.md`, and the
> implementation steps and PR-checkpoint bookkeeping are in
> `.claude/commands/runplan.md` (step 5a). Follow those files rather than copying
> their content here.

Drive one already-created PR through review, merge, and the post-merge plan
bookkeeping, then offer to continue with the next PR of the same task.

**Input.** `$ARGUMENTS`, when given, is a PR number, a branch name, or a task ID
(e.g. `0015`). Resolve it to a PR:

- a PR number or branch: `gh pr view "$ARGUMENTS" --json number,url,headRefName`.
  Pass the argument only when it is set — an empty argument would replace
  `gh pr view`'s current-branch default and fail;
- a task ID: the current branch's PR, or the open PR whose head branch is named
  for that task (`gh pr list --state open --json number,headRefName,url`, then
  match the task);
- no argument: `gh pr view --json number,url,headRefName` for the current branch.

Use the resolved PR's number in the steps below.

The PR this loop starts on is created by `runplan.md` step 5a; this command does
not create the first PR.

Repeat steps 1–4 for each PR.

## 1. Wait for CodeRabbit, running fixpr as comments appear

Poll until CodeRabbit has finished and no review thread is left to fix.

- Read CodeRabbit's status from the PR's **status contexts**, not from
  `gh pr checks`:

  ```bash
  gh pr view <number> --json statusCheckRollup
  ```

  Find the entry whose `context` (a `StatusContext`) or `name` is `CodeRabbit`
  and read its `state` (`PENDING` → `SUCCESS` / `FAILURE`). `gh pr checks` can
  report a separate check run whose conclusion is `pass` with the description
  `Review skipped` while the review is still in progress, so it is not a
  reliable signal. Wait between polls rather than spinning.

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
  (`no-harm`; `fixpr.md`'s convergence policy defines the floor), go to step 2. If
  a remaining unresolved thread is not at the floor, report it and stop rather
  than merge. If its state is `FAILURE`, report and stop.
- If no `CodeRabbit` status context ever appears after a reasonable wait, report
  that and ask the user how to proceed rather than assuming green.

## 2. Merge

Run `.claude/commands/mergepr.md` for the PR. It drafts the squash commit message
from the PR material and pauses for the user's explicit approval before merging;
never merge without that approval.

## 3. Post-merge bookkeeping

Once `mergepr.md` reports the merge:

- Confirm its cleanup finished and the local base branch was updated before
  creating the next branch. `mergepr.md` can stop after the merge with cleanup
  pending, in which case run `mergepr cleanup --state <state-file>` and only then
  continue. Then update the local base branch and create the next branch for the
  next phase group from it. Never branch from, or keep committing to, the merged
  branch: after a squash merge its commits are not ancestors of the base.
- In the task's `03_implementation_plan.md`, mark the just-completed PR
  checkpoint's remaining checkboxes — the "PR がマージされた" and
  "次のブランチへ切り替えた" items — as `[x]`, and commit. This is the same
  bookkeeping `runplan.md` step 5a defines; follow that definition.

## 4. Offer to continue

Ask the user whether to implement the next PR with `/runplan`.

- If yes, run `.claude/commands/runplan.md` for the same task. It implements the
  next phase group and creates the next PR (its step 5a); then return to step 1
  for that PR.
- If the task has no next PR, say so, report the final status, and stop.

## Notes

- `fixpr.md` and `mergepr.md` each perform network operations; each has its own
  authorization note — follow it. In particular, `mergepr.md` requires the user's
  explicit approval of the squash commit message.
- If `fixpr` reaches its round cap or leaves a thread it cannot fix, report the
  remaining threads and ask the user instead of merging with them open.
