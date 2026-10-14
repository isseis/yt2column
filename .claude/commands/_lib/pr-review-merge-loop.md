# Shared pattern: the per-PR review, merge, and branch-switch loop

`runpr.md` and `rundoc.md` both drive one PR through CodeRabbit review,
`fixpr`, `mergepr`, and the post-merge branch switch; they differ only in how
the PR is created and what the calling command does after the merge. This file
defines the common procedure once. A calling command invokes it by saying
"follow `_lib/pr-review-merge-loop.md` for the PR" and supplying the inputs
below.

Project-specific values (build checks, green gate, PR merge method) are in
`.claude/commands/_context.md`. The fix and merge mechanics are in
`.claude/commands/fixpr.md` and `.claude/commands/mergepr.md`. Those are project
files — follow them rather than copying their content here.

## Inputs the calling command must supply

- **CREATE_PR** — `true` when the calling command should create the PR for the
  target branch when none exists yet (`rundoc.md`); `false` when the PR already
  exists (`runpr.md`; `runplan.md` step 5a creates it).
- **POST_MERGE** — what the calling command does after the merge and branch
  switch, beyond the shared procedure (`runpr.md` updates the task plan's
  PR-checkpoint checkboxes; `rundoc.md` nothing).

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

### 2. Wait for CodeRabbit, running fixpr as comments appear

Poll until CodeRabbit has finished and nothing is left to fix.

- Read CodeRabbit's status from the PR's **status contexts**, not from
  `gh pr checks`:

  ```bash
  gh pr view <number> --json statusCheckRollup
  ```

  Find the entry whose `context` (a `StatusContext`) or `name` is `CodeRabbit`
  and read its `state` (`PENDING` → `SUCCESS` / `FAILURE`). `gh pr checks` can
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
  (`no-harm`; `fixpr.md`'s convergence policy defines the floor), go to step 3.
  If a remaining unresolved thread is not at the floor, report it and stop rather
  than merge. If its state is `FAILURE`, report and stop.
- If no `CodeRabbit` status context ever appears after a reasonable wait, report
  that and ask the user how to proceed rather than assuming green.

### 3. Merge

Run `.claude/commands/mergepr.md` for the PR. It drafts the squash commit message
from the PR material and pauses for the user's explicit approval before merging;
never merge without that approval.

### 4. Switch branch, then apply POST_MERGE

Once `mergepr.md` reports the merge:

- Confirm its cleanup finished and the local base branch was updated before
  creating the next branch. `mergepr.md` can stop after the merge with cleanup
  pending, in which case run `mergepr cleanup --state <state-file>` and only then
  continue.
- Update the local base branch and create the next branch for the next change
  from it. Never branch from, or keep committing to, the merged branch: after a
  squash merge its commits are not ancestors of the base. If there is no next
  change, check out the updated base branch instead.
- Then apply the calling command's POST_MERGE action, if it was supplied.
