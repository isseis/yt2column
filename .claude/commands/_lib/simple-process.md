---
description: "Internal: the simple process's procedures for the commands that read or write the process record (not a command)"
---

# Shared pattern: the simple process

The simple process writes a task's three documents (`01_requirements.md`,
`02_architecture.md`, `03_implementation_plan.md`) in one PR, the
three-document PR, which `/mkreq` opens. Its rules — the process record and its
values and states, what is never omitted and what may be omitted, the excluded
changes, the return to the full process, and the kinds of handoff item — are in
`docs/dev/conventions/requirements_process.md` ("Process record" under
"Document Status Format", "Simple Process", and "Design handoff"). This file
holds only the procedures the commands follow, and refers to those sections by
name instead of copying them. A calling command says "follow
`_lib/simple-process.md`, <section>".

**Steps of other commands this file uses.** "Writing the three documents" and
"Reviewing the three documents" use steps of `mkreq.md`, `mkarch.md`,
`mkplan.md`, and `mkplan2.md`, named by their number and the words of their
heading. A change that renumbers those steps or changes what they do must check
this file in the same PR.

## Reading the process record

Read the `Process` row of the task's `01_requirements.md` by the "Cell text"
rule of the requirements process guide, from the version the calling command
reads:

| Command | Version read |
|---|---|
| `/mkarch`, `/mkplan` | `origin/main`, right after a `git fetch origin main` and before `_lib/task-pr-handoff.md` Part A, so a simple-process task stops before a branch is created. When `origin/main` has no `01_requirements.md` for the task, leave it to the command's own stop on a requirements document that is not `approved` |
| `/runplan`, and `/runpr` and `/fixpr` on an implementation PR | `origin/main`, after a fetch. After the three-document PR is merged the record does not change, and an edit on the PR's branch is not read |
| `/runpr` and `/fixpr` on a three-document PR | The PR's head (the checkout `runpr.md` step 1 brought up to date), checked against the mismatches below, because `01_requirements.md` is not on `origin/main` yet |
| `/mkreq`, continuing a stopped run | The branch it continues ("Resuming a stopped run" below) |

For `/runpr` on a PR it found `MERGED` (`runpr.md` step 1), compare with the
parent of the PR's squash commit in place of `origin/main`: after the merge
`origin/main` has the PR's `01_requirements.md`, so the comparison with it
would not show which documents the PR added.

Classify what you read by the states of the record in "Process record": no
`Process` row and `` `full` `` are the full process, `` `simple` `` is the
simple process, and a broken record stops the command. When it stops, report
the path of `01_requirements.md`, the version read, and why the row cannot be
read. Never guess a process.

**Mismatches.** Then check the record against the documents. Stop on any of
these, report what does not match and which document is missing or extra, and
decide nothing:

- The record is `` `simple` ``, `origin/main` has `01_requirements.md`, and
  `origin/main` lacks `02_architecture.md` or `03_implementation_plan.md`
  (`/mkarch`, `/mkplan`, `/runplan`, and `/runpr` and `/fixpr` on an
  implementation PR).
- The record is `` `simple` ``, the PR adds `01_requirements.md`, and the PR's
  head lacks `02_architecture.md` or `03_implementation_plan.md` (`/runpr`,
  `/fixpr`).
- The record is missing or `` `full` ``, the PR adds `01_requirements.md`, and
  the same PR adds `02_architecture.md` or `03_implementation_plan.md`
  (`/runpr`, `/fixpr`).
- On a three-document PR, some of the three documents are `approved` and
  others `draft` (`/runpr`): the simple process approves the three in one
  answer.

A mismatch is a stop in both directions: a record that is missing or rewritten
must not silently turn off what the simple process guards.

## Recommending a process

`/mkreq` recommends a process before it asks which one to take. Weigh the
change by these points, with no limit on any count:

1. How many components or commands the change reaches.
2. How many decisions are open.
3. Whether it changes the decision of an existing ADR.
4. Whether it builds a new mechanism, or only changes the wording or a value of
   one that exists.

Judge whether the change is excluded against the list of excluded changes in
the requirements process guide, "Simple Process", by reading the change. When
in doubt, judge it excluded. When it is excluded, recommend the full process.
Otherwise recommend the process the points above favor: the simple process for
a change that reaches few places, leaves few decisions open, changes no ADR's
decision, and builds no new mechanism.

## Writing the three documents

`/mkreq` follows this in the simple process, after the human chose it.

1. **Confirm every decision before writing.** Settle the requirements
   decisions (`mkreq.md` step 5, "Settle the open decisions"), then the design
   decisions (`mkarch.md` steps 4 to 6.5, from "Read the remaining required
   input documents" to "Settle the open design decisions"), then the plan
   decisions (`mkplan.md` steps 4 to 5.5, from "Read the remaining required
   input documents" to "Settle the open planning decisions"), in that order,
   because a later decision depends on the earlier answers. Where those steps
   say "the approved requirements" or "the approved architecture" in setting
   what to investigate and which decisions are open, read "the requirements
   decisions confirmed in this run" and "the design decisions confirmed in this
   run". Write nothing before the last round is answered.
2. **Write, one commit per document.** Write `01_requirements.md` (`mkreq.md`
   step 6, "Write the first version of `01_requirements.md`"), with the
   `Process` row `` `simple` ``; then `02_architecture.md` (`mkarch.md` step 7,
   "Create `02_architecture.md`"); then `03_implementation_plan.md`
   (`mkplan.md` steps 6 and 7, "Create `03_implementation_plan.md`" and "Apply
   test helper planning rules"), with its PR boundaries designed by
   `mkplan2.md` steps 4 to 8 ("Analyse the implementation steps and design PR
   groupings" to "Update §6"). Do not run `mkplan2.md` step 9 (the PR boundary
   review): its checklist goes to "Reviewing the three documents" below.
   Commit each document by itself, with the subject its step gives
   (`mkreq.md` step 6, `mkarch.md` step 7.5, `mkplan.md` step 7.5), and put
   each ADR change in the commit of the document that records its decision.
3. **An error in an earlier document.** When writing a later document shows
   that an earlier one of this run is wrong (the case `mkplan.md` step 6 calls
   "When the architecture is wrong"), fix the earlier `draft` document in this
   run. When the fix changes a decision the human confirmed, ask the human
   first.
4. **What may be left out.** Leave out only what the requirements process
   guide, "Simple Process", says may be omitted, and keep everything it says is
   never omitted.

## Reviewing the three documents

`/mkreq` follows this after the three documents are committed. Run the
critical-review procedure in `.claude/commands/_lib/review-subagent-pattern.md`
in single-reviewer mode:

- **ARTIFACT**: the three documents of the three-document PR.
- **PERSONA**: an experienced requirements engineer, software architect, and
  senior SRE, who reads the three documents together as the implementer who
  must build from them, and looks especially for places where they disagree.
- **FILES**: the three documents, `design_handoff.md` and
  `implementation_handoff.md` if they exist, the requirements template, the
  requirements process guide, the Mermaid reference guide, the test
  organization guide, the project overview, and `CLAUDE.md`, plus the
  conditional guide when its trigger applies, and the files of the ADR changes
  with, when they are not empty, `docs/dev/conventions/adr.md`, as resolved
  absolute paths. Include the issue text by saving it to the scratchpad and
  passing that path.
- **CRITERIA**: under one heading per document, these checklists, copied
  verbatim: for `01_requirements.md`, the requirements checklist of `mkreq.md`
  step 7 ("Self-review"); for `02_architecture.md`, the two checklists of
  `mkarch.md` step 8 (the critical-review procedure in panel mode); for
  `03_implementation_plan.md`, the three checklists of `mkplan.md` step 8 (the
  critical-review procedure) and the PR boundary review checklist of
  `mkplan2.md` step 9. Tell the reviewer to read these items as follows:
  - "`01_requirements.md` is `approved`" and "`02_architecture.md` is
    `approved`" mean that the three documents are all `draft` and the process
    record is `` `simple` ``.
  - "All required sections ... are present" is met by a section whose heading
    stays with N/A and its reason.
  - A Mermaid diagram item applies only to a diagram the document has.

The risk is high when `mkplan.md` step 8 would use panel mode for this plan,
or when the human overrode an exclusion judgment made in doubt. Then, instead
of the single review above, give each document the review of its own command:
`mkreq.md` step 7 for `01_requirements.md`, `mkarch.md` step 8 (panel mode)
for `02_architecture.md`, and `mkplan.md` step 8 and `mkplan2.md` step 9 for
`03_implementation_plan.md`, with the same reading of the items.

Then run `japrose` on the three documents and on each ADR of the ADR changes,
after the Critical and Major issues of the review are resolved; as a sub-step
it does not commit. Commit the fixes of both passes once. The local CLI review
runs later, in `_lib/task-pr-handoff.md` Part B step 1.

## Resuming a stopped run

A simple-process `/mkreq` run that stopped before it opened the PR leaves a
branch with some of the three documents. `/mkreq <issue>` run again continues
it when `mkreq.md` step 1 finds a requirements document that names the issue
and every one of these holds:

- `origin/main` has no such requirements document.
- The search found it on exactly one branch. A local branch and the `origin`
  branch of the same name count as one.
- The branch's name follows the feature branch name of `_context.md` for the
  task directory of that requirements document.
- The process record of that requirements document on the branch is
  `` `simple` ``.
- The branch has no PR, open or closed
  (`gh pr list --head <branch> --state all --json number`).
- When the branch is both local and on `origin`, the `origin` branch's commit
  is the local commit or an ancestor of it. `_lib/task-pr-handoff.md` Part A
  creates the branch on `origin`, and the document commits are not pushed
  before Part B, so the local branch is usually ahead. Stop when the two have
  diverged or `origin` is ahead.
- No other worktree has the branch checked out (`git worktree list`).
- The working tree is clean. When documents being written are left in it, stop
  with the changes left as they are, list their files, and leave it to the
  human to commit or discard them.

When any of them does not hold, stop as `mkreq.md` step 1 does for an existing
requirements document, and report each condition that failed.

When all of them hold:

1. Show the branch's name and its commits (`git log --oneline origin/main..<branch>`).
2. TASK_DIR is the directory of the requirements document found; `mkreq.md`
   step 2 chooses no new number. Check out the branch before
   `_lib/task-pr-handoff.md` Part A, which keeps it (its step 3).
3. For each of the three documents the branch lacks, settle its decisions
   again and write and commit it, as "Writing the three documents" says; the
   process is not asked again. A document already committed is not written
   again.
4. Run "Reviewing the three documents" and its prose pass again over all three
   documents, whether or not the stopped run had done them, and then open the
   PR (`mkreq.md` step 8).

## When an excluded decision appears

**While `/mkreq` confirms the design or plan decisions** ("Writing the three
documents", step 1): when a decision needs an excluded change (the requirements
process guide, "Simple Process"), show it and the reason, and ask whether to
switch to the full process, with two options: switch to the full process, or
override the judgment and go on in the simple process. An answer that is
neither is not a choice: ask again.

- **Switch:** write `01_requirements.md` alone, with the `Process` row
  `` `full` ``, and record the switch and its reason in its
  `要件定義の際に決めたこと（YYYY-MM-DD）` section. Write each design decision
  and each plan decision already confirmed as a confirmed item of
  `design_handoff.md` or `implementation_handoff.md` (the requirements process
  guide, "Design handoff"), so `/mkarch` and `/mkplan` adopt them later.
  Keep the ADR changes of the requirements decisions only: undo, in the working
  tree, the ADR changes made only for a design or plan decision, and keep only
  the requirements part of an ADR changed for both. The confirmed item keeps
  the ADR judgment for `/mkarch` or `/mkplan` to act on.
  Continue as the full process does: `mkreq.md` steps 6 to 9.
- **Go on:** record the override and its reason in the
  `要件定義の際に決めたこと（YYYY-MM-DD）` section of `01_requirements.md`, and
  continue with "Writing the three documents".
