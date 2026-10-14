> **Project context (read first)**: Read `.claude/commands/_context.md` — it is
> canonical for every project-specific path, build check, green gate, and helper
> this command names. The review and merge mechanics are defined in
> `.claude/commands/fixpr.md` and `.claude/commands/mergepr.md`. Follow those
> files rather than copying their content.

Create a PR for a change already committed on the current branch, drive it
through CodeRabbit review and merge, then switch to a fresh branch for the next
change.

Use this for a change that is **not** a `/runplan` task — for example a
`01_requirements.md`, `02_architecture.md`, or `03_implementation_plan.md`
document, or any other standalone change. There is no `/runplan` step and no
implementation-plan checkbox to update (the plan file may not exist yet).
`runpr.md` is the variant for a `/runplan` task PR, where runplan creates the PR
and the plan's PR-checkpoint checkboxes are updated after the merge.

**Input.** `$ARGUMENTS`, if given, is a PR number or branch to operate on; with
no argument, use the current branch.

## 1. Create the PR (reuse an existing one)

- Determine the target: `$ARGUMENTS` when it is given (a PR number or branch),
  otherwise the current branch. Resolve its PR — pass the argument to `gh pr
  view`, and omit it only to fall back to the current branch:

  ```bash
  gh pr view "$ARGUMENTS" --json number,url,headRefName
  ```

  If a PR is open, reuse it, use its number in the steps below, and go to step 2.
- Otherwise the target branch has no PR yet. Confirm the base branch (`main`; the
  repository's base for a normal PR), push the branch and set its upstream so
  `gh pr create` does not prompt:

  ```bash
  git push -u origin HEAD
  ```

- Draft a conventional-commit title (`<type>(<scope>): <summary>`, matching the
  style of `git log origin/<base>`) and a short body from the branch's commits,
  describing the final state, not the history. Write the body to a file with the
  Write tool and pass it with `--body-file` so shell metacharacters in the body
  are not interpreted:

  ```bash
  gh pr create --base <base> --title "<title>" --body-file <body-file>
  ```

- Report the PR URL. If the user wants to adjust the title or body, do so before
  continuing.

## 2. Wait for CodeRabbit, running fixpr as comments appear

Same test as `runpr.md` step 1:

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

- Fetch the unresolved review threads, following every page (`endCursor` until
  `hasNextPage` is false), the same way `runpr.md` step 1 does. Filter to
  `isResolved=false`.
- If a **new unresolved thread** has appeared since the last `fixpr` run for this
  PR, run `.claude/commands/fixpr.md` for the PR before continuing to wait;
  `fixpr` pushes a fix, so CodeRabbit re-reviews and you keep polling.
- Proceed to step 3 when CodeRabbit's state is `SUCCESS` and either no thread is
  unresolved or `fixpr` reported that every remaining thread is at its severity
  floor (`no-harm`; `fixpr.md`'s convergence policy defines the floor). If an
  unresolved thread is not at the floor (a `must-fix`, or a thread `fixpr` could
  not fix), report it and stop rather than merge. If its state is `FAILURE`,
  report and stop. If no `CodeRabbit` status context ever appears after a
  reasonable wait, report that and ask the user how to proceed rather than
  assuming green.

## 3. Merge

Run `.claude/commands/mergepr.md` for the PR. It drafts the squash commit message
from the PR material and pauses for the user's explicit approval before merging;
never merge without that approval.

## 4. Switch branch

Once `mergepr.md` reports the merge, confirm its cleanup finished and the local
base branch was updated before creating the next branch. `mergepr.md` can stop
after the merge with cleanup pending, in which case run
`mergepr cleanup --state <state-file>` and only then continue. Then update the
local base branch and create a new branch for the next change from it. Do not
keep committing to the merged branch: after a squash merge its commits are not
ancestors of the base. This is the branch handling of `runplan.md` step 5a,
without its plan-checkbox update. If there is no next change, check out the
updated base branch instead.

Then report the final status. There is no next-PR prompt: create the next change
and run this command again when ready.
