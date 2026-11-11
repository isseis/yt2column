---
description: "Opus review of a commit range: a correctness review and a weak-model failure review in one pass, fixing by default"
argument-hint: "[range | PR] [--no-fix | --fix]"
---

> **Project context (read first)**: Read `.claude/commands/_context.md`. It is the
> single source of truth for the task root, guide paths, document/status
> conventions, build checks (`make fmt`/`make test`/`make lint`/`make deadcode`),
> the green gate, source layout, and test-helper placement. Where this command
> names such a path or command, treat the entry in `_context.md` as canonical.
> The review itself is the shared procedure in
> `.claude/commands/_lib/opus-review.md`.

## Purpose

`runplan.md` is written assuming the executing model reliably applies every
rule it states. In practice, a lower-capability model following the same
`runplan.md` steps tends to satisfy the *literal* instruction (checkbox
ticked, code compiles, tests pass) while missing the *judgment* the
instruction actually depends on: noticing an implicit invariant, keeping
several documents in sync, resisting a shortcut, or applying an exception
correctly.

This command runs the Opus review of `_lib/opus-review.md` over a commit
range, in one pass and on Opus whatever model runs the session: a correctness
review (bugs, and divergence from the requirements, the architecture, and the
plan) and a weak-model failure review (the failure patterns above), merged
into one report. It needs no other review to run alongside it, and it does not
review cleanup (`/simplify`'s scope). By default it fixes the Critical and
Major findings and commits the fixes on the current branch without pushing;
`--no-fix` makes it report only.

Use it after work was implemented by a lower-capability model, before the
merge.

## Arguments

`$ARGUMENTS` holds at most one range and at most one flag, in any order:

| Argument | Meaning |
|---|---|
| (no range) | The current branch: `origin/main...HEAD` |
| `<A>..<B>` or `<A>...<B>` | A commit range, each end a revision `git rev-parse` resolves to a commit |
| `<N>` (digits only) | A PR number |
| `--no-fix` | Report only; change nothing |
| `--fix` | Accepted and does nothing: fixing is the default |

Stop without changing anything, and show the accepted arguments above, when
`$ARGUMENTS` holds both `--no-fix` and `--fix`, a flag twice, two ranges, or
any other token. Do not guess what an unaccepted argument meant.

FIX is `report` with `--no-fix`, and `fix` otherwise.

## Network operations

Running this command approves `git fetch origin main` and `gh pr view`, the
only network operations it runs. Both only read; this command writes nothing
to GitHub and never pushes.

## Procedure

1. **Parse the arguments** as "Arguments" says.

2. **Resolve RANGE** to the form `_lib/opus-review.md` takes, `<base>..<head>`
   with both ends as full commit IDs. A three-dot range given to `git log`
   would be the symmetric difference, which includes commits of both sides,
   so it is never passed on.
   - No range: run `git fetch origin main`; RANGE is
     `$(git merge-base origin/main HEAD)..$(git rev-parse HEAD)`.
   - Match `...` before `..`. An end left out of a range (`HEAD~3..`) is
     `HEAD`, as in `git log`.
   - `<A>..<B>`: resolve each end with
     `git rev-parse --verify --end-of-options '<A>^{commit}'` (the same for
     `<B>`); RANGE is `<A's ID>..<B's ID>`.
   - `<A>...<B>`: resolve both ends the same way; RANGE is
     `$(git merge-base <A's ID> <B's ID>)..<B's ID>`.
   - `<N>`: run `gh pr view <N> --json baseRefName,headRefOid`. Stop when the
     base is not `main`, or when the head commit is not in the local
     repository (`git cat-file -e '<headRefOid>^{commit}'` fails; tell the
     user to fetch the PR's branch). Otherwise run `git fetch origin main`;
     RANGE is `$(git merge-base origin/main <headRefOid>)..<headRefOid>`.
   - Stop and report when a command fails, or when the range is empty
     (`git rev-list --count <RANGE>` is `0`).

3. **Identify the task.** The arguments name a range, not a task, so
   `docs/dev/conventions/task_identification.md` applies only through its
   rule 2. Take the first of these that gives one task:
   - the task directory the files RANGE changes lie under
     (`git diff --name-only <RANGE> -- docs/tasks/`), when they lie under
     exactly one;
   - when they lie under none, RANGE's `<head>` is `HEAD`, and the current
     branch's name follows the feature branch name convention
     (`_context.md`): the directory under `docs/tasks/` whose name, with each
     `_` replaced by `-`, equals the name's `<slug>`, when exactly one does.
     A range that does not end at `HEAD` says nothing about the current
     branch;
   - the task of a file open in the IDE (its rule 2), when RANGE changes no
     task directory or the file's task is one of those it changes;
   - otherwise ask with AskUserQuestion, offering the task directories RANGE
     changes as the options when they fit in its options, and otherwise
     asking for the task's number. Do not stop as its rule 3 says: this command takes no task
     argument.

   TASK_DIR is the task directory's repository-relative path (such as
   `docs/tasks/0081_implementation_model_choice`).

4. **When FIX is `fix`, check before reviewing.** Stop, change nothing, and
   report the reason when any of these fails:
   - A branch is checked out (`git symbolic-ref -q --short HEAD` prints its
     name), and it is not `main`.
   - RANGE's `<head>` is `HEAD` (`git rev-parse HEAD`). A fix committed on a
     `HEAD` other than the range's end would sit on commits outside the
     range.
   - The working tree is clean (`git status --short` prints nothing).

5. **Run the review**: follow `_lib/opus-review.md`, entry 2, with RANGE,
   TASK_DIR, FIX, and **DECISION_CHANGE** = do not fix the finding; report it
   in KEPT with the decision it needs, and leave it to the human. Do not push.
   This command writes no record that the review finished: its range is
   whatever the user gave, so it need not match a PR's range.

6. **When the review made commits (COMMITS is not empty), check for an open
   PR**: run
   `gh pr view <current-branch> --json state,url`. When its state is `OPEN`,
   add to the report:
   - the commits are not pushed, so the first PR review, which ran when the
     PR was opened, does not see them;
   - `/fixpr` run without a push stops on the unpushed commits when its round
     commits nothing, and pushes these fixes together with its own when it
     commits; `/runpr` and `/mergepr` merge the head on GitHub, so an unpushed
     fix does not reach the squash merge;
   - before `/runpr`, the user decides whether to push the fixes or drop them;
     whether a pushed change gets another PR review is decided by
     `_lib/pr-review-escalation.md`.

   When `gh pr view` fails, for any reason (a branch with no PR also makes it
   fail), report that the PR's state could not be checked, and add the note
   all the same, introduced as applying if a PR is open for the branch.

7. **Report** in Japanese: RANGE and TASK_DIR, the outputs of
   `_lib/opus-review.md` (OUTCOME, FINDINGS grouped by severity, KEPT,
   COMMITS, CATEGORIES, READ), and the note of step 6. On
   `stopped-unavailable`, report that the review could not run on Opus and
   that the user can run this command again once Opus is available. On
   `stopped-limit`, report the remaining findings for the user to decide on.
   On `stopped-error`, report its reason and what the run left (commits, and
   an uncommitted fix in the working tree, if any).
