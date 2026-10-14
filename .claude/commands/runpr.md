> **Project context (read first)**: Read `.claude/commands/_context.md` — it is
> canonical for every project-specific path, build check, green gate, and helper
> this command names. The review and merge mechanics are defined in
> `.claude/commands/fixpr.md` and `.claude/commands/mergepr.md`, and the
> implementation steps and PR-checkpoint bookkeeping are in
> `.claude/commands/runplan.md` (step 5a). Follow those files rather than copying
> their content here.

Drive one already-created PR through review, merge, and the post-merge plan
bookkeeping, then offer to continue with the next PR of the same task.

**Input.** `$ARGUMENTS` is a PR number, a branch name, or a task ID (e.g.
`0015`). Resolve it to a PR with `gh pr view`; with no argument, use the current
branch's PR. A task ID resolves to the current branch's PR, or to the open PR
whose head branch belongs to that task.

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

- If a **new unresolved thread** has appeared since the last `fixpr` run for this
  PR, run `.claude/commands/fixpr.md` for the PR before continuing to wait.
  `fixpr` pushes a fix, so CodeRabbit re-reviews: keep polling.
- When CodeRabbit's state is `SUCCESS` and there are no unresolved threads, go to
  step 2. If its state is `FAILURE`, report and stop.
- If no `CodeRabbit` status context ever appears after a reasonable wait, report
  that and ask the user how to proceed rather than assuming green.

## 2. Merge

Run `.claude/commands/mergepr.md` for the PR. It drafts the squash commit message
from the PR material and pauses for the user's explicit approval before merging;
never merge without that approval.

## 3. Post-merge bookkeeping

Once `mergepr.md` reports the merge:

- Update the local base branch and create the next branch for the next phase
  group from it. Never branch from, or keep committing to, the merged branch:
  after a squash merge its commits are not ancestors of the base.
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
