---
description: "Create 01_requirements.md for a task from a GitHub issue and open its PR for review"
argument-hint: "<issue> [task]"
---

> **Project context (read first)**: Read `.claude/commands/_context.md` — it is
> canonical for every project-specific path, document name, status value, branch
> name, and reviewer this command names. The review step follows
> `.claude/commands/_lib/review-subagent-pattern.md`; the branch setup and the PR
> handoff follow `.claude/commands/_lib/task-pr-handoff.md`.

Your goal is to turn one GitHub issue into the first version of a task's
`01_requirements.md`, review it, open its PR, and hand the PR to the human
reviewer. The issue comes first: a requirements document is not written without
one. This command does not wait for the merge. The reviewer reviews the PR and
then runs `/runpr <PR>` by hand, which drives the review threads and the
merge.

**Input.** `$ARGUMENTS` is `<issue> [task]`:

- `<issue>` (required) — an issue number (`330`, `#330`) or its URL. If it is
  missing, stop and ask the user to re-run with one.
- `[task]` (optional) — an existing task directory to write into (`0030` or
  `0030_<name>`). Without it, a new task directory is created.

**Authorization.** Invoking this command approves, for this issue and this PR
only, `gh issue view` (also for the issues the issue references),
`gh repo view`, `gh pr list` for the duplicate search, and the
network operations `_lib/task-pr-handoff.md` names ("Authorization"). Nothing
else that touches the network runs without asking.

Work in the following order.

1. Check the starting state.
   - Read the issue:
     `gh issue view <issue> --json number,title,body,state,url,comments`.
     If it is closed, stop and report. If its `url` is not under this
     repository's (`gh repo view --json url`), another repository's issue URL
     was given: stop and report rather than using its number. From here on,
     use the `number` it returns (`N`) wherever the issue is named — searches, commits, the PR
     body — never the raw argument, which may be `#330` or a URL.
   - `git fetch origin`. Search for a requirements document that already names
     issue `N` as its own (`対応 issue: [#N]`) on `origin/main`, on every
     `origin` branch, and on every local branch
     (`git grep -l '対応 issue: \[#N\]' $(git for-each-ref --format='%(refname)' refs/heads refs/remotes/origin) -- docs/tasks`).
     The branches cover open PRs, a branch pushed by a run that stopped before
     opening its PR, and an unpushed branch in another worktree. If one
     exists, stop and report it: revising an existing requirements document
     goes through its PR (`/fixpr`, `/runpr`), not this command.

2. Resolve the task directory.
   - With `[task]`: resolve it under the task root on `origin/main` as
     `docs/dev/conventions/task_identification.md` rule 1 describes. Stop if
     it does not exist there or already contains `01_requirements.md`.
   - Without it: choose the name `NNNN_<name>` (the directory is created in
     step 3). `NNNN` is one above the highest task number in use, looking at
     `origin/main` (`git ls-tree -d --name-only origin/main docs/tasks/`), at
     local and `origin` branches (`git branch -a`), and at the head branches of
     open PRs (`<user>/NNNN-...`), so parallel worktrees do not take the same
     number. `<name>` is a short English snake_case name for
     the issue's subject, in the style of the existing directory names (e.g.
     `output_layout`, `native_messaging_relay`).

3. Create the feature branch: follow `.claude/commands/_lib/task-pr-handoff.md`,
   Part A, with **TASK_DIR** = the step 2 directory,
   **REPORT** = the step 9 report, **ISSUE** = the issue number from step 1
   (the requirements document that names it does not exist yet), and the
   task-document values of the other
   inputs. A new task's branch has sequence number `01`
   (e.g. `isseis/0027-remove-context-menu-01`). Then create the task directory
   chosen in step 2 on it.

4. Read the input material.
   - The requirements process guide, the requirements template
     (`docs/tasks/0000_template/requirements.md`), and the project overview.
   - The issue's body and comments, and every issue or task it references, as far
     as they bear on what to build.
   - The conditional guide when the conditional-guide trigger applies (both in
     `_context.md`, Domain-specific).
   - The code and the earlier task documents the issue touches, so the document
     states the current behavior correctly. A claim about current behavior cites
     `file:line` or the task document it comes from.

5. Settle the open decisions with the human: follow
   `.claude/commands/_lib/settle-decisions.md` with
   - **SCOPE** = what the requirements must fix and the issue leaves open: scope
     and out-of-scope boundaries, observable behavior (normal, error, and edge
     cases), user-visible messages and rejection reasons, compatibility, and any
     exception to a policy in the project overview, `CLAUDE.md`, or an earlier
     task. A choice of mechanism (API, data structure, test method) belongs to
     the design: record it for `design_handoff.md` (step 6).
   - **RECORD** = the `要件定義の際に決めたこと（YYYY-MM-DD）` list (step 6).

6. Write the first version of `01_requirements.md`.
   - Start from the template. Write in Japanese. Status `draft`, `Created` today,
     `Review date` and `Reviewer` `-`.
   - In 概要, link the issue (`対応 issue: [#N](<url>)`, plus prerequisite and
     related issues), state what the document decides and what it leaves to
     `02_architecture.md`, and record the decisions in two lists: those the
     issue already made (`issue #N で決めたこと`) and those made in step 5
     (`要件定義の際に決めたこと（YYYY-MM-DD）`), each with its reason.
     `docs/tasks/0029_transcript_panel_read/01_requirements.md` is an example of
     this shape.
   - Follow the requirements process guide, section 1: acceptance criteria with
     document-wide unique IDs that state observable behavior, and the
     untrusted-input boundary rules where the requirement consumes untrusted
     input.
   - Put implementation-level concerns that came up in `design_handoff.md`
     (`H-NN` items) instead of the requirements.
   - Commit the task directory:
     `docs(NNNN): draft requirements for <concise English summary>`, with
     `Refs #<issue>` in the body.

7. Self-review. Run the critical-review procedure in
   `.claude/commands/_lib/review-subagent-pattern.md` in single-reviewer mode:
   - **ARTIFACT**: the requirements document.
   - **PERSONA**: an experienced requirements engineer and senior SRE, who reads
     the document as the architect who must design from it.
   - **FILES**: the requirements document, `design_handoff.md` if it exists, the
     requirements template, the requirements process guide, the project
     overview, and `CLAUDE.md`, plus the conditional guide when the trigger
     applies, as resolved absolute paths. Include the issue text by saving it to
     the scratchpad and passing that path.
   - **CRITERIA**: the checklist below, verbatim.

   Then run the Japanese prose-quality pass by invoking `japrose`
   (`.claude/commands/japrose.md`) on the requirements document, after the
   Critical and Major issues of the review are resolved; as a sub-step it does
   not commit. When both passes are done, commit the fixes once.

   **Requirements checklist (use verbatim as evaluation criteria):**
   - [ ] The status is `draft`; `Review date` and `Reviewer` are `-`.
   - [ ] Every section of the template is present, or explicitly marked as not applicable.
   - [ ] The issue is linked, and every point the issue asks for is either covered by a requirement or listed as out of scope with a reason.
   - [ ] Nothing is required beyond the issue unless it is recorded as a decision with its reason.
   - [ ] Every decision is recorded with its reason, and none rests on an assumption that was not confirmed with the reviewer.
   - [ ] Every functional requirement has acceptance criteria; each criterion is specific, measurable, independently verifiable, and states observable behavior, not implementation.
   - [ ] Acceptance-criterion IDs are unique across the document and sequential on first creation.
   - [ ] Error and edge-case behavior is stated, including what the user sees.
   - [ ] For each untrusted input the requirements consume: the accepted form, the sentinel error a rejection maps to, any size or record limit, the consumed members and the ones declared extensible, and rejection (not repair) of input the standard library would silently repair, discard, or ignore.
   - [ ] No mechanism (API, selector, data structure, test method) is fixed unless the issue or a recorded decision fixed it; other implementation concerns are in `design_handoff.md`.
   - [ ] Every claim about current behavior cites `file:line` or the task document it comes from.
   - [ ] The document does not claim what a test would catch.
   - [ ] Any exception to a policy in the project overview, `CLAUDE.md`, or an earlier task is stated with the policy, its location, and the reason.
   - [ ] The security section applies the conditional guide when the trigger applies, and says `N/A` otherwise.
   - [ ] The same concept always uses the same Japanese term, consistent with the translation glossary.

8. Open the PR and hand it over: follow `.claude/commands/_lib/task-pr-handoff.md`,
   Part B, with the same inputs as step 3.

9. Report: the PR URL, the task directory, the decisions made in step 5, the
   review results, and the next action for the reviewer (review the PR, then run
   `/runpr <PR>`).
