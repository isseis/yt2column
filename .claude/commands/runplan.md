---
description: "Implement the next phase group of an approved implementation plan and open its PR at a checkpoint"
argument-hint: "<task>"
---

> **Project context (read first)**: Read `.claude/commands/_context.md` — it is
> canonical for every project-specific path, build check, green gate, and
> test-helper placement this command names. Porting instructions live there too;
> this body's parts that need editing are the domain-specific invariant examples
> in step 5 (cache keys, `--dry-run` side effects, temporary-file
> cleanup) and the Go test-helper rules. The review step follows
> `.claude/commands/_lib/review-subagent-pattern.md`; the branch setup and the PR
> handoff follow `.claude/commands/_lib/task-pr-handoff.md`.
>
> **Implementation model requirement.** Before starting a session, check the
> `**実装モデル要件**` value of the PR the work belongs to: the field of the
> `### PR-N 作成ポイント` marker covering it, or, in a plan without PR markers,
> the field of the `PR 構成` section of the plan's §3. `_context.md`, "PR
> marker conventions", says where the value is, how it is read (an old value
> through its table), and which model each reading recommends: start the
> session on Opus for `opus`, and on Sonnet for `sonnet+opus-review` and
> `sonnet`. The value also decides whether the Opus review
> (`_lib/opus-review.md`) runs before the PR opens (step 5a, step 8): for
> `opus`, `sonnet+opus-review`, or an unreadable value it runs when some
> commit of the PR is not covered by a record of an Opus implementation
> (entry 1 of `_lib/opus-review.md` decides); for `sonnet` or no value it
> never runs. Step 4.2 shows the value and the forecast; this command does
> not stop when the session's model and the value disagree.

Your goal is to implement one task under `docs/tasks/` by following its `03_implementation_plan.md`.

Work in order.

1. Identify the target task per `docs/dev/conventions/task_identification.md`.

1.5. Prepare the branch before reading the plan, so it is read from the branch
   the work lands on: follow
   `.claude/commands/_lib/task-pr-handoff.md`, Part A, with **TASK_DIR** = the
   target task directory and **REPORT** = the step 8 summary. On a branch that
   already carries the earlier phase groups of the PR in progress, Part A keeps
   it.

2. Read `03_implementation_plan.md`. If the document status is not `approved`, stop and report.

2.5. Check whether PR boundary design is needed.
   - Count `### フェーズ` headers in the plan. If there are 2 or more and no `### PR-` sections exist, stop and report: PR boundary design is missing (the plan predates `/mkplan` designing them). Ask the user to run `/mkplan2`, which opens a PR for the reviewer, before re-running `/runplan`. Do not invoke `/mkplan2` yourself.
   - If PR markers already exist, skip this step and continue.

3. Read `01_requirements.md`, `02_architecture.md` (both in the target task directory), and `docs/dev/conventions/test_organization.md`. Also read the task's process record from `origin/main` (after a `git fetch origin main`) by following `.claude/commands/_lib/simple-process.md`, "Reading the process record". Stop and report when the record is broken or the record and the documents disagree, as that section says.

4. Select the next unit of work from `03_implementation_plan.md` checkboxes (`[ ]` not started, `[x]` done, `[-]` skipped). Skip the inside of every diff record's code block when reading the plan's markers and checkboxes: nothing there is a marker (`docs/dev/conventions/requirements_process.md` §4, "Diff record form").
- If all phases and all PR markers are complete, skip to step 8 and follow the "If all phases are complete" bullet.
- Scan forward from the last completed item. If the next unchecked items are inside a `### PR-N 作成ポイント` section (PR checkpoint checkboxes), treat this as a PR checkpoint:
  - If its `PR を作成した` box is already checked, the PR exists (its URL is on that line). Check its state with `gh pr view <url> --json state`. If it is open, stop and tell the user to review it and run `/runpr`. If it is merged, `/runpr` has not done its bookkeeping on this branch: stop and report rather than open a second PR. If it is `CLOSED` (not merged), stop and report the cause to the user; do not create a PR, because a second PR from the same branch would leave the closed PR's URL in the plan.
  - Otherwise go to step 5a instead of step 5.
- Otherwise, select the next implementation group: use one phase by default unless it cannot pass `make test` alone (e.g. stub-only or tightly coupled); then extend the group until it can pass. Stop the group at the next `### PR-N 作成ポイント` boundary — do not include the PR checkpoint checkboxes in the implementation group. Briefly note the reason for any grouping before starting work.

4.2. Show the implementation model guidance when step 4 selected a phase group or a PR checkpoint. The PR is the one whose `### PR-N 作成ポイント` marker covers the selected unit (TARGET = `N`), or, in a plan without PR markers, the plan itself (TARGET = `single`). Show, without stopping whatever they say:
   - the PR's value, read as `_context.md`, "PR marker conventions", says: for an old value, the old value and the value it is read as; for an unreadable value, its text and that it is unreadable; or that there is no value;
   - the model that reading recommends (the same section's table), or that it recommends none;
   - this session's model ID, the one Claude Code gives in its system prompt, or that it is unknown;
   - the forecast of the Opus review: run entry 1 of `.claude/commands/_lib/opus-review.md` now (PLAN = the plan, TARGET, BASE = `$(git merge-base origin/main HEAD)`; step 3 fetched `origin/main`) and show its output. Before this phase group is implemented, its commits are not recorded yet, so the decision before the PR can differ: when the output is `NONE all-opus` or `SKIP`, say that the review runs anyway unless every phase group of the PR, this one included, is recorded as implemented on Opus; when it is `RUN`, say that it may not run if this session is Opus and every other phase group of the PR is recorded as Opus. Step 5a or step 8 decides again before the PR opens.

4.5. Record the starting commit of the phase group: `PHASE_START=$(git rev-parse HEAD)`. Keep it until the reviews in steps 7, 7.5, and 7.7 are done, and pass `${PHASE_START}..HEAD` as their commit range (step 7.7 passes `${PHASE_START}` as BASE, which both CLIs of the local CLI review take as the start of the range), so the range covers exactly this phase group's commits and not the previous group's. Also note this session's model ID (the one Claude Code gives in its system prompt) now, and again at each commit of the phase group (steps 5, 6, 7, 7.5, and 7.7), keeping every ID seen, for the implementation record of step 7.8. When the session does not know its model ID at one of these points, it writes no implementation record.

5. Implement the selected phase group.
- Follow the design in `02_architecture.md`.
- Before writing new files in a package, read at least one existing file of the same kind (e.g. a `_test.go` in the target package) to confirm assertion library, import style, and helper conventions in use. Mismatches with the established style will be caught in review — reading first avoids the rework.
- **State invariants before coding.** For any generated value (IDs/names), flag/mode, or side-effecting operation, write its contract in one line as a code comment above the implementation and implement to that — not to the first approach that compiles. Examples: a cache key → *derived only from the video ID, never from the raw URL* (the same video reached by `youtu.be/…` and `watch?v=…` must hit one entry); `--dry-run` → *no external side effects* (skip every Webhook send and every LLM API call, not just the final post); a yt-dlp working directory → *always removed before returning, even on error* (`defer os.RemoveAll`, not only on the success path).
- Place test helpers per `docs/dev/conventions/test_organization.md`: cross-package helpers under `testutil/`; package-internal helpers in `test_helpers.go` (or `test_helpers_<category>.go`) with `//go:build test`.
- After each file change (Go or otherwise), run `make test && make lint`; for Go file changes also run `make fmt` first. Fix errors before continuing. Exception: errors caused by the phase group's incomplete state (e.g. build or test failures from missing implementations that stubs depend on) need not be fixed until the group is complete; fix only errors unrelated to the in-progress group.
- When `make test` fails on `TestReadmeSyncMarker` because this group changed `README.md`, run the README sync check as `.claude/commands/mktrans.md`, "When a caller runs it", says, and include `README.en.md` in the group's commit; list the terms it reports as missing from the glossary in the step 8 summary and the PR body.
- When removing multiple scattered code sites (e.g. several test functions), delete them one at a time using the exact text read from the file. Do not script bulk deletion (e.g. a brace-counting loop); nested literals make such heuristics over-consume adjacent code. After each removal, check IDE diagnostics for unintended breakage.
- For any newly authored or substantially rewritten artifact in this group (runbook, command example, table, prose, translation, design-doc section), confirm it is correct before committing — do not rely on absence-search of removed terms. Run any documented command and check its exit code and output; cite the source implementation that prose describes; diff a translation against its source. For a design document such as an ADR, also keep the body on the current system and confine removed-design rationale to a bounded history note rather than interleaving it.
- **Before committing each group, self-check** (catches common defects before the step 7 review):
  - **If the implementation approach diverges from `02_architecture.md`, `01_requirements.md`, or the plan step descriptions** (e.g., a simpler alternative is adopted, an API turns out to have compatibility issues), classify the divergence as `docs/dev/conventions/requirements_process.md` "Editing an approved document" does:
    - **Editorial correction** (the documents say the same thing, but a name or a step description is stale): update (1) the plan step *description* text (not only the checkbox) and (2) any architecture document section that no longer matches, in the same commit. Change neither `Comments` nor the change history; write one correction line per correction in that commit's message, in the form `docs/dev/conventions/requirements_process.md`, "Recording in the PR description", gives. The PR description gets the corrections from these lines when the PR is created (`_lib/task-pr-handoff.md` Part B step 3). There is no step for a PR that already exists: this command never works on a branch that has a PR (`_lib/task-pr-handoff.md` Part A step 3 stops on one, and step 4 stops at a checkpoint whose PR exists), so every correction it makes comes before its PR is created. A divergence that is only marked `[x]` without updating the description text will appear as a plan/code inconsistency to reviewers and generate stale review threads.
    - **Excluded decision change in a simple-process task.** Before the "Decision change" check below, when the record is `` `simple` `` and the divergence needs a change excluded by the requirements process guide, "Simple Process" ("Excluded changes"), judge it against that list and, when in doubt, treat it as excluded. On an excluded change, stop before settling or committing it: do not call `_lib/settle-decisions.md`, and if work on it has begun, leave that work out of the commit. Report the content of the change and that it becomes a separate task in the full process, for which an issue is to be created by the human (`/runplan` does not create it). A task with no record or with `` `full` `` goes on as below.
    - **Decision change** (what to build or why changes): stop before implementing it, and settle it with the human: follow `.claude/commands/_lib/settle-decisions.md` with **SCOPE** = this divergence and **RECORD** = a `実装の際に決めたこと（YYYY-MM-DD）` section in the overview chapter of each changed document (the decision and its reason). On the reviewer's answer, update the document, and the later task documents whose content depends on the changed decision, in the same commit, together with the ADR changes (the files `_lib/settle-decisions.md`, "Output", returned). Keep the status of each changed task document `approved` (the answer is the re-approval) and record it as a session re-approval in each changed document: `Comments` `-` and the session re-approval line in the change history, with the reviewer's GitHub name and the not-yet-filled PR number, as `docs/dev/conventions/requirements_process.md`, "Editing an approved document" and "Document Status Format", say (migrate a document without a change history first, as its "Documents without a change history" says; when that section stops, commit nothing of this phase group, leave its changes in the working tree, and report as the section says). The PR description names the change when the PR is created, from that line (`_lib/task-pr-handoff.md` Part B step 3). Do not implement a decision change the reviewer has not approved.
  - **ADR editorial correction** (an ADR's reference or wording is stale, for example after the implementation renamed something it names, and its decision still holds): when this phase group's work makes an editorial correction to an ADR, classify it by `docs/dev/conventions/adr.md`, "Editorial corrections", and write one correction line per correction in the message of the commit that makes it, in the form `docs/dev/conventions/requirements_process.md`, "Recording in the PR description", gives. A revision of the PR's own text (text this branch added to an ADR that `origin/main` does not have yet) is not an editorial correction and gets no correction line. The PR description gets the corrections from these lines when the PR is created (`_lib/task-pr-handoff.md` Part B step 3); as for a task document, this command never works on a branch that has a PR, so no later step is needed. A change to an ADR's decision is a decision change: settle it with the human as the "Decision change" bullet above says.
  - No planning-doc references (`AC-01`, `F-001`) in source comments or strings — put the *why* in plain English.
  - Validators/parsers/env-checks have a happy-path test (all-valid → no error), not only failure cases.
  - A helper guarding a precondition calls its own guard internally; callers shouldn't have to.
  - Sub-test names match what they test (`slack_webhook_missing` tests `SLACK_WEBHOOK_URL`, not `YT2COLUMN_CACHE_DIR`).
  - Reuse existing utilities before writing new ones (e.g. the json3 parser in `internal/transcript`); consolidate duplicate regexes/constants.
  - Test-only behavior stays in test packages; don't branch production code for tests.
  - A Go test file importing a `testutil` that imports the package under test uses `package foo_test` to avoid an import cycle.
  - Any new or modified file with build tags beyond `//go:build test` alone (e.g. `//go:build test && foo`): confirm it is reached by at least one `make lint` invocation, or explicitly document the gap. Ask: "does `make lint` compile this file?"

- **Record each `diff` verification of the ACs this group implements** (`docs/dev/conventions/requirements_process.md` §4, "`diff` verification"), after the commit that implements the AC:
  - For a recorded diff reading, run the fixed commands of §4 ("Fixed commands") on the PR branch, with `HEAD` (the commit that implements the AC) as `<commit>`, and write the diff record in the form of §4 ("Diff record form") in a later commit; the compared paths are the ones the plan names. When `git fetch origin main`, the merge-base, or a fixed command fails, record nothing and stop and report: an empty output cannot be told from an empty diff.
  - When the output turns out too long to be read whole, record nothing and do not change the form of the verification: settle it with the human as a decision change (the "Decision change" bullet above).
  - For a script, confirm that each of its checks fails under at least one breakage, run it on the PR branch, and record its output and both commit IDs in the AC's row. When it fails, fix the implementation or the script (redoing the breakage check for a changed check) and run it again; stop and report when that does not resolve it.
- When complete, update checkboxes (`[x]` done, `[-]` skipped with a note) and commit.

5a. **PR checkpoint** (reached when step 4 directed you here instead of step 5).
- Confirm the current branch is the PR group in progress (`_lib/task-pr-handoff.md`, Part A step 3, second bullet: a convention branch for this task with `/runplan` commits beyond `origin/main` and no PR). Otherwise stop and report; do not push or open a PR from another branch.
- Verify the green gate (defined in `_context.md`) passes. Fix any failures before continuing.
- If no later `### PR-N 作成ポイント` marker remains (the task's final PR), first do the end-of-task check of step 8 ("If all phases are complete"): every AC in `01_requirements.md` is satisfied by the implementation and has at least one `test`, `static`, or `diff` verification, or a `manual` reading where the AC specifies only the natural-language content of a document, as labeled in the plan; each `diff` script was run on this branch and its output and both commit IDs are recorded in the plan, and each recorded diff reading has its diff record. Fix any gap before opening the PR, and report what you checked.
- At every checkpoint, recheck each diff record of the ACs this PR verifies, as §4 of `docs/dev/conventions/requirements_process.md`, "Rechecking a record", says: a later phase group can make a record of an earlier one stale. Re-record each stale one (output and both commit IDs) and commit the plan; stop and report when the recheck stops.
- Verify again that the green gate passes when the steps above made commits, then run "Before the PR" (below step 8) with TARGET = `N` of this checkpoint's `### PR-N 作成ポイント` marker.
- Mark the first PR checkpoint checkbox (the green gate confirmation line) as `[x]` and commit. When "Before the PR" ran the Opus review, put its done record in this commit's message; when the box is already `[x]` (an earlier session marked it and stopped), put the done record in an empty commit (`git commit --allow-empty -F <file>`) instead. Write no done record when the review did not run.
- Open the PR and hand it over: follow `.claude/commands/_lib/task-pr-handoff.md`,
  Part B, with **TASK_DIR** and **REPORT** as in step 1.5, and:
  - **CHECKS** = the green gate, which "Before the PR" left passing (the
    checkbox and record commits after it change only the plan or nothing, so
    they need no rerun). When the README sync of Part B step 1 commits
    `README.en.md`, run the green gate again on that commit. When Part B
    step 2 makes a commit to make the green gate pass (any commit that is not
    excludable, `_lib/opus-review.md`, "Record lines"), redo the decision
    before the push: run entry 1 again for the new `HEAD` ("Before the PR",
    B2). On `RUN`, run the Opus review again (B3, and B4 when it
    made fix commits, with the green gate, the final PR's AC check, and the
    diff-record recheck run again), write the new done record in an empty
    commit, and bring the Opus review result of PR_TEXT up to date with it,
    the new kept findings included. A commit made after `NONE all-opus` is
    not covered by an implementation record, so the decision becomes `RUN`
    unless the value is `sonnet` or absent;
  - **LOCAL_REVIEW** = `done`: step 7.7 reviewed each phase group of this PR,
    and "Before the PR", B1, reviewed the whole branch when a phase group
    was left unreviewed; give that review's SUMMARY to Part B step 3, which
    puts it in the PR body;
  - **PR_TEXT** = a plan PR title (`_context.md`, "PR title and squash
    subject") written from the PR marker's `推奨タイトル`, which is a draft,
    and the marker's `レビュー観点` in the body. The value is the text inside
    its code span, without the backquotes. Write the title this way:
    - When the `推奨タイトル` starts with an accepted prefix that has a
      `<scope>`, keep that prefix; the draft summary is the rest of the value.
    - When it starts with an accepted prefix without a `<scope>` (such as
      `feat: `), keep its `<type>` if it fits the change and otherwise choose
      one from the prefix table that does; the prefix is `<type>(<task-id>): `,
      and the draft summary is the rest of the value.
    - When it starts with no accepted prefix, the prefix is
      `<type>(<task-id>): `, with a `<type>` from the prefix table that fits
      the change, and the whole value is the draft summary.
    - Write the summary in Japanese with the same content as the draft,
      whatever language the draft is in; a draft already in Japanese is the
      summary itself. Do not rewrite the plan: an older plan's value stays as
      written.

    PR_TEXT also carries the Opus review result for the body, written from
    entry 1's output and the record lines, never from a subagent's text
    ("Before the PR", B5).
  - **AFTER_CREATE** = mark the second checkbox (`PR を作成した`) as `[x]` and
    append the PR URL to that line, commit, and `git push`, so the update is in
    the PR before it is merged.
- Then stop. Do not wait for the merge: the reviewer runs `/runpr <PR>`, which
  merges the PR, marks the remaining checkboxes (`PR がマージされた`,
  `次のブランチへ切り替えた`), and switches to the next branch. Run `/runplan`
  again for the next PR. PRs are squash-merged (`_context.md`, "PR merge
  method").

6. Run `make deadcode`. Remove functions made unreachable by this phase group,
   and re-run it until it passes: intentional scaffolding for future phases or
   tasks must be reachable from a test, and do not add scaffolding that no test
   reaches until the PR that uses it. If changes were made, run
   `make fmt && make test && make lint` and commit.

6.5. Run programmatic pre-checks on the changed Go files before spawning the review agent. These checks are deterministic and cheaper than AI review — catch them here rather than in the review loop.

   ```bash
   # Files changed in this phase group — exclude deleted files so rg never
   # receives a path that no longer exists (which would exit 2, masking matches)
   CHANGED=$(git diff origin/main...HEAD --diff-filter=d --name-only | grep '\.go$' || true)

   if [ -n "$CHANGED" ]; then
     # Check 1: no planning-doc identifiers in source
     if echo "$CHANGED" | xargs rg -l '\bAC-[0-9]+[a-z]?\b|\bF-[0-9]+[a-z]?\b' 2>/dev/null; then
       echo "FAIL: planning-doc references found — fix before continuing"
     else
       echo "OK: no planning-doc references"
     fi

     # Check 2: no non-ASCII characters in Go source
     if echo "$CHANGED" | xargs rg -Pn '[^\x00-\x7F]' 2>/dev/null; then
       echo "REVIEW: non-ASCII found — verify each is intentional"
     else
       echo "OK: all ASCII"
     fi
   else
     echo "OK: no Go files changed"
   fi
   ```

   - If Check 1 has any matches: fix them, run `make fmt && make test && make lint`, commit, then continue (these are never intentional in source).
   - If Check 2 has matches: inspect each — test-data literals, error strings, and a literal the guard-test literal exception of `CLAUDE.md`, "Go Idioms", covers (such as a Go program under `_checks/` whose literal must match a Japanese document's text) may legitimately contain non-ASCII, but identifiers and non-test comments must not. Fix any unintentional occurrences, run `make fmt && make test && make lint`, and commit before continuing.

7. Run the critical-review subagent procedure in `.claude/commands/_lib/review-subagent-pattern.md` with these inputs:
   - **ARTIFACT**: this phase group's code changes.
   - **PERSONA**: an experienced senior Go engineer and senior SRE. Direct it to surface bugs, missing test coverage, architecture drift, and unclear code.
   - **FILES**: the architecture document and the implementation plan document (paths in `_context.md`), as resolved absolute-path strings; instruct the subagent to read both in full before evaluating the code. Also list the source files added or modified in this phase group, and the files of any ADR changes `_lib/settle-decisions.md` returned in this phase group with `docs/dev/conventions/adr.md`, as resolved absolute-path strings (read in full). Provide `${PHASE_START}..HEAD` as the commit range and instruct the subagent to run `git diff ${PHASE_START}..HEAD` to see exactly what changed.
   - **CRITERIA**: every item from the phase-group review checklist below, copied verbatim.

   Extra rule: when fixing Critical and Major issues, run the build checks (defined in `_context.md`) and commit before spawning the verification pass.

Phase-group review checklist (use verbatim as evaluation criteria in the subagent prompt above):
- [ ] Implementation is consistent with `02_architecture.md`.
- [ ] Every AC assigned to this phase group by the implementation plan has at least one `test`, `static`, or `diff` verification, or a `manual` reading where the AC specifies only the natural-language content of a document (a mixed AC still needs its test, static, or diff verification), as labeled in the implementation plan's Acceptance Criteria Verification section. Each `diff` verification has its result in the plan's verification row: a script was run on this branch against the merge-base with `origin/main`, with its output and both commit IDs recorded; a recorded diff reading has its diff record in the form of `docs/dev/conventions/requirements_process.md` §4 ("Diff record form").
- [ ] No `diff` verification written in this phase group (a script, or a diff record) is oversized for what it checks: weigh what the reviewer will read (the script, or the recorded output) against the size of the diff it verifies; a script in a language other than shell has the plan's reason why shell is not enough; a script is not copied from or modeled on an earlier task's `_checks/`. This item does not apply to a plan approved before this item reached `main`, nor to its `_checks/`.
- [ ] Test coverage is sufficient: non-trivial logic, error paths, and boundary values are covered.
- [ ] No tests duplicate existing coverage without good reason.
- [ ] No tests are so trivial that they add no verification value.
- [ ] No logic is reimplemented when an existing function in the codebase can be used.
- [ ] Newly authored artifacts (runbooks, command examples, tables, prose, translations) are verified against ground truth, not only by absence-search.
- [ ] All source comments and identifiers are in English. (Non-ASCII flagged by step 6.5 Check 2 has been reviewed and is intentional.)

7.5. Run a second critical-review subagent focused on the tests, following the
   procedure in `.claude/commands/_lib/review-subagent-pattern.md` with:
   - **ARTIFACT**: the test code added or modified in this phase group.
   - **PERSONA**: an experienced Go test reviewer. Direct it to find, in this
     phase group's tests only, coverage that duplicates another test, tests too
     trivial to verify anything, tests that can be simplified, string matching on
     error messages that should be a type check, and tests that break the
     project's testing rules.
   - **FILES**: the test files added or modified in this phase group, plus
     `CLAUDE.md` (for the testing rules) and the implementation plan document
     (paths in `_context.md`), as resolved absolute-path strings (read in full).
     Provide `${PHASE_START}..HEAD` as the commit range and instruct the
     subagent to run `git diff ${PHASE_START}..HEAD`.
   - **CRITERIA**: every item from the test-review checklist below, copied
     verbatim.

   Run it after step 7, so it reviews the tests as they stand after any
   code-review fixes. Fix its Critical and Major findings and include them in the
   same build-check/commit/verification loop as step 7. If the phase group adds
   or changes no tests, skip this step.

Test-review checklist (use verbatim as evaluation criteria in the step 7.5 prompt):
- [ ] No test duplicates another test's coverage without good reason.
- [ ] No test is so trivial that it adds no verification value (for example, a
      struct field asserted to hold the value just assigned to it, or a constant
      asserted equal to its own literal).
- [ ] Related cases that share setup and verification are covered by one table or
      loop; duplicate rows are not repeated as a second table, and cases that
      differ only by fixture data are consolidated.
- [ ] String matching on error messages (`strings.Contains(err.Error(), ...)`,
      `err.Error() == ...`) is a type check (`errors.Is` / `errors.AsType`)
      wherever the verified property is an error type or sentinel. String
      matching is kept only where the wording itself is the observable contract,
      and the test comments why.
- [ ] The tests obey the testing rules in `CLAUDE.md`: every test can fail for its
      stated reason (and the break-to-fail check was performed); a layered path is
      given an input only one layer can handle; no constant-equals-its-own-literal
      or field-holds-its-own-assignment assertion; duplication is checked before
      adding a table test; a deleted test is in a commit of its own whose
      message and the PR description carry the passing output through the last
      line of `make coverage-compare BASE=<parent>` and/or
      `make ext-coverage-compare BASE=<parent>`, both when both kinds of test
      were deleted.
- [ ] Each test asserts observable behavior rather than restating an
      implementation detail.

7.7. Local CLI review and triage. Run it after step 7.5, so the local CLI review (the CodeRabbit CLI, or Codex CLI when the CodeRabbit CLI fails) sees the code and tests as they stand after the subagent-review fixes. Follow `.claude/commands/_lib/local-cli-review.md` with **BASE** = `${PHASE_START}`, **SCOPE** = phase group, **SUMMARY** = the step 8 summary. When it stops, stop this command and report as it says. Put the record lines it returns in SUMMARY, including any `Local CLI review: not run` line, into the message of the first commit made on the branch after step 7.7 returns, which is the commit of step 7.8. Before a commit that carries a `Codex local review:` line, run the mention check of `_lib/local-cli-review.md` step 7. Step 5a and step 8 read these lines from `git log`.

7.8. Implementation record. Make an empty commit (`git commit --allow-empty -F <file>`) whose message holds the implementation record of this phase group (`_lib/opus-review.md`, "Record lines": `Implementation model: <START12>..<END12> <model-id>[, ...]`, with START = `PHASE_START`, END = `HEAD` just before this commit, and every model ID step 4.5 kept) and the record lines step 7.7 returned. Leave the implementation record out when the session did not know its model ID at one of the points of step 4.5; the phase group then stays uncovered and the Opus review runs for it. When the message would hold neither, make no commit. Run the mention check of step 7.7 when the message carries a `Codex local review:` line. Entry 1 of `_lib/opus-review.md` reads the implementation records from `git log` to decide whether the Opus review runs.

8. Decide whether to continue or finish.
- If implementation ran this iteration: summarize implementation, verified ACs, assumptions, and deferred items, the implementation model guidance of step 4.2, the implementation record of step 7.8 (or why none was written), and, when "Before the PR" (step 5a) ran, its decision and the Opus review result of its B5, plus the local CLI review result: each attempt's result (CodeRabbit CLI, Codex CLI as the fallback, or neither) with each failed CLI's failure kind, how many times Codex CLI ran, any range neither CLI reviewed, the counts of fixed / rejected / deferred / unclear findings by CLI with the reason for each rejection and the open question of each `unclear` one, and the note when the reviewed range changes a file Codex reads its instructions from.
- If phases remain: return to step 4 without asking. The next unit is either another phase group of the same PR, or its PR checkpoint (step 5a), where this command opens the PR and stops; the reviewer's checkpoint is the PR review, not a question between phase groups.
- If all phases are complete: verify every AC in `01_requirements.md` is satisfied by the implementation and has at least one `test`, `static`, or `diff` verification, or a `manual` reading where the AC specifies only the natural-language content of a document, as labeled in the plan's Acceptance Criteria Verification section; each `diff` script was run on this branch and its output and both commit IDs are recorded in the plan, and each diff record was rechecked as step 5a says (re-recorded and committed when stale). Report the final status and any gaps. If the plan has no PR markers (a single-phase plan), open the PR with `_lib/task-pr-handoff.md` Part B as step 5a does (with its CHECKS, LOCAL_REVIEW, and Opus review result of PR_TEXT), with these differences: first check `gh pr view` and `git rev-list --count origin/main..HEAD`, and stop if the branch already has a PR or has no commits; then verify the green gate and run "Before the PR" with TARGET = `single`, and after `RUN` write its done record in an empty commit (`git commit --allow-empty -F <file>`), but none after `NONE` or `SKIP`; the title is a plan PR title (`_context.md`, "PR title and squash subject"): with no `推奨タイトル`, the prefix is `<type>(<task-id>): ` with a `<type>` from the prefix table that fits the change, and the summary is in Japanese; the body names the plan's main review points; **AFTER_CREATE** = append to the plan's checklist section (§6) the single-PR record `- [x] PR を作成した: <PR URL>` and `- [ ] PR がマージされた`, commit, and `git push`. `/runpr` reads that record: the PR is the task's final plan PR, so NEXT is none and the merge closes the issue.

## Before the PR

Step 5a, and step 8 for a plan without PR markers, run items B1 to B5 in
order, after the green gate and the end-of-task checks and
before the first commit that marks the PR checkpoint. They put every fix
commit on the branch before the PR opens, so Codex's first PR review sees the
fixed code. These items are not steps of the numbered list above: after B5,
go back to the step that ran them.

B1. **Branch review.** Skip this item when `git log origin/main..HEAD` holds an
   `Opus review: done` line: an earlier run got past the review, and so past
   this item; a fix commit made now would only reopen the decision (the
   earlier SUMMARY is gone, so Part B step 3 gives the ranges of the
   `not run` lines). Step 7.7 reviewed each phase group of this PR. A phase
   group neither CLI reviewed is recorded by a `Local CLI review: not run`
   line in a commit message after it (step 7.7); an earlier session recorded
   it as `CodeRabbit local review: not run`. Read the lines as
   `_lib/local-cli-review.md`, "Reading the record lines", says. Only when
   `git log origin/main..HEAD` shows either line, review the whole branch
   again now: follow `.claude/commands/_lib/local-cli-review.md` with
   **BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
   **SUMMARY** = REPORT. A phase group the Codex CLI fallback reviewed leaves
   no such line and needs no branch review. When the branch review stops,
   stop: nothing is pushed. Copy no record line of this review into a commit
   of your own (the procedure's own fix commits keep theirs). When it made
   commits, run the green gate and the diff-record recheck again. This review
   comes before B2 so that its fix commits are inside the Opus review's
   range.
B2. **Decision.** `git fetch origin main`, then run entry 1 of
   `.claude/commands/_lib/opus-review.md` with PLAN = the plan, TARGET, and
   BASE = `$(git merge-base origin/main HEAD)`.
B3. **Review.** On `RUN`, run entry 2 of `_lib/opus-review.md` with RANGE =
   `<BASE>..<HEAD>` (both resolved to full commit IDs), TASK_DIR = the target
   task directory, FIX = `fix`, and DECISION_CHANGE = the divergence rules of
   step 5's self-check (an editorial correction is corrected in place; an
   excluded decision change of a simple-process task stops as that bullet
   says; a decision change is settled with the human, and when the human does
   not approve it, entry 2 records the finding as kept). When OUTCOME is
   `stopped-unavailable`, `stopped-limit`, or `stopped-error`, stop: do not
   open the PR and push nothing. Report the stop and what the user can do: run
   `/runplan` again once Opus is available (`stopped-unavailable`); look at
   the remaining findings and decide (`stopped-limit`); or, for
   `stopped-error`, its reason, and whether to keep or discard the
   uncommitted fix left in the working tree. Running `/runplan` again comes
   back to this checkpoint and decides again from B2: the review then
   covers every commit of the PR, the fix commits made before the stop
   included, and the findings the human already rejected are read from the
   kept lines rather than asked again. On `NONE` or `SKIP`, run no review.
B4. **After fixes.** When the review made fix commits, run the green gate again
   (a commit that only makes it pass is inside the done record's range), and,
   for the task's final PR, the end-of-task check of step 8, and the
   diff-record recheck. When the end-of-task check or the recheck makes a
   commit that is neither excludable (`_lib/opus-review.md`, "Record lines")
   nor a fix to make the green gate pass, go back to B2.
B5. **Result for the PR body.** Write the Opus review result into PR_TEXT, for
   `_lib/task-pr-handoff.md` Part B step 3, in Japanese:
   - after `RUN`: the range `<BASE>..<HEAD>` as it stands now and the count
     for each severity, from FINDINGS (the done record, written next, repeats
     them); each kept finding's location, gist, and reason, from the
     `Opus review: kept` lines of `git log origin/main..HEAD`; the gist of each
     Minor finding; CATEGORIES and READ;
   - after `SKIP`: the same from the done record and the kept lines, and the
     commits after the done record (entry 1's list), noting that the Minor
     findings and READ belong to the earlier session that ran the review;
   - after `NONE`: that the review did not run and which condition did not
     hold: the value is `sonnet` (`value-sonnet`), there is no value
     (`value-absent`), or every commit is covered by Opus implementation
     records (`all-opus`).

   Describe a finding only by its location and your own summary, never with a
   subagent's text.
