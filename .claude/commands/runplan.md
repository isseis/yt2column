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
> Before starting an implementation session, check the `**実装モデル要件**` field of
> the `### PR-N 作成ポイント` marker covering the work you are about to pick up (see
> `_context.md`, "PR marker conventions"): `frontier-required` means run this session
> on a frontier model; `frontier-recommended` prefers one; `standard` may run on a
> cheaper model, with `weakreview.md` as the follow-up check. This does not change
> any step below.

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

3. Read `01_requirements.md`, `02_architecture.md` (both in the target task directory), and `docs/dev/conventions/test_organization.md`.

4. Select the next unit of work from `03_implementation_plan.md` checkboxes (`[ ]` not started, `[x]` done, `[-]` skipped).
- If all phases and all PR markers are complete, skip to step 8 and follow the "If all phases are complete" bullet.
- Scan forward from the last completed item. If the next unchecked items are inside a `### PR-N 作成ポイント` section (PR checkpoint checkboxes), treat this as a PR checkpoint:
  - If its `PR を作成した` box is already checked, the PR exists (its URL is on that line). Check its state with `gh pr view <url> --json state`. If it is open, stop and tell the user to review it and run `/runpr`. If it is merged, `/runpr` has not done its bookkeeping on this branch: stop and report rather than open a second PR. If it is `CLOSED` (not merged), stop and report the cause to the user; do not create a PR, because a second PR from the same branch would leave the closed PR's URL in the plan.
  - Otherwise go to step 5a instead of step 5.
- Otherwise, select the next implementation group: use one phase by default unless it cannot pass `make test` alone (e.g. stub-only or tightly coupled); then extend the group until it can pass. Stop the group at the next `### PR-N 作成ポイント` boundary — do not include the PR checkpoint checkboxes in the implementation group. Briefly note the reason for any grouping before starting work.

4.5. Record the starting commit of the phase group: `PHASE_START=$(git rev-parse HEAD)`. Keep it until the reviews in steps 7, 7.5, and 7.7 are done, and pass `${PHASE_START}..HEAD` as their commit range (step 7.7 passes `${PHASE_START}` as BASE, which both CLIs of the local CLI review take as the start of the range), so the range covers exactly this phase group's commits and not the previous group's.

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
    - **Editorial correction** (the documents say the same thing, but a name or a step description is stale): update (1) the plan step *description* text (not only the checkbox) and (2) any architecture document section that no longer matches, in the same commit. Change neither `Comments` nor the change history; write a correction line for each corrected document in that commit's message, in the form `docs/dev/conventions/requirements_process.md`, "Recording in the PR description", gives. The PR description gets the corrections from these lines when the PR is created (`_lib/task-pr-handoff.md` Part B step 3). There is no step for a PR that already exists: this command never works on a branch that has a PR (`_lib/task-pr-handoff.md` Part A step 3 stops on one, and step 4 stops at a checkpoint whose PR exists), so every correction it makes comes before its PR is created. A divergence that is only marked `[x]` without updating the description text will appear as a plan/code inconsistency to reviewers and generate stale review threads.
    - **Decision change** (what to build or why changes): stop before implementing it, and settle it with the human: follow `.claude/commands/_lib/settle-decisions.md` with **SCOPE** = this divergence and **RECORD** = a `実装の際に決めたこと（YYYY-MM-DD）` section in the overview chapter of each changed document (the decision and its reason). On the reviewer's answer, update the document, and the later task documents whose content depends on the changed decision, in the same commit; keep their status `approved` (the answer is the re-approval) and record it as a session re-approval in each changed document: `Comments` `-` and the session re-approval line in the change history, with the reviewer's GitHub name and the not-yet-filled PR number, as `docs/dev/conventions/requirements_process.md`, "Editing an approved document" and "Document Status Format", say (migrate a document written under the old rule first, as its "Documents written under the old rule" says). The PR description names the change when the PR is created, from that line (`_lib/task-pr-handoff.md` Part B step 3). Do not implement a decision change the reviewer has not approved.
  - No planning-doc references (`AC-01`, `F-001`) in source comments or strings — put the *why* in plain English.
  - Validators/parsers/env-checks have a happy-path test (all-valid → no error), not only failure cases.
  - A helper guarding a precondition calls its own guard internally; callers shouldn't have to.
  - Sub-test names match what they test (`slack_webhook_missing` tests `SLACK_WEBHOOK_URL`, not `YT2COLUMN_CACHE_DIR`).
  - Reuse existing utilities before writing new ones (e.g. the json3 parser in `internal/transcript`); consolidate duplicate regexes/constants.
  - Test-only behavior stays in test packages; don't branch production code for tests.
  - A Go test file importing a `testutil` that imports the package under test uses `package foo_test` to avoid an import cycle.
  - Any new or modified file with build tags beyond `//go:build test` alone (e.g. `//go:build test && foo`): confirm it is reached by at least one `make lint` invocation, or explicitly document the gap. Ask: "does `make lint` compile this file?"

- When complete, update checkboxes (`[x]` done, `[-]` skipped with a note) and commit.

5a. **PR checkpoint** (reached when step 4 directed you here instead of step 5).
- Confirm the current branch is the PR group in progress (`_lib/task-pr-handoff.md`, Part A step 3, second bullet: a convention branch for this task with `/runplan` commits beyond `origin/main` and no PR). Otherwise stop and report; do not push or open a PR from another branch.
- Verify the green gate (defined in `_context.md`) passes. Fix any failures before continuing.
- If no later `### PR-N 作成ポイント` marker remains (the task's final PR), first do the end-of-task check of step 8 ("If all phases are complete"): every AC in `01_requirements.md` is satisfied by the implementation and has at least one `test`, `static`, or `diff` verification, or a `manual` reading where the AC specifies only the natural-language content of a document, as labeled in the plan; each `diff` script was run on this branch and its output and both commit IDs are recorded in the plan. Fix any gap before opening the PR, and report what you checked.
- Mark the first PR checkpoint checkbox (the green gate confirmation line) as `[x]` and commit.
- Open the PR and hand it over: follow `.claude/commands/_lib/task-pr-handoff.md`,
  Part B, with **TASK_DIR** and **REPORT** as in step 1.5, and:
  - **CHECKS** = the green gate verified above (the checkbox commit after it
    changes only the plan, so it needs no rerun; run it again when the branch
    review below made commits). When the README sync of
    Part B step 1 commits `README.en.md`, run the green gate again on that
    commit;
  - **LOCAL_REVIEW** = `done` — step 7.7 reviewed each phase group of this PR.
    A phase group neither CLI reviewed is recorded by a `Local CLI review: not
    run` line in a commit message after it (step 7.7); an earlier session
    recorded it as `CodeRabbit local review: not run`. Read the lines as
    `_lib/local-cli-review.md`, "Reading the record lines", says. Only when
    `git log origin/main..HEAD` shows either line, review the whole branch
    again: follow `.claude/commands/_lib/local-cli-review.md` now with
    **BASE** = `$(git merge-base origin/main HEAD)`, **SCOPE** = branch,
    **SUMMARY** = REPORT. A phase group the Codex CLI fallback reviewed leaves
    no such line and needs no branch review. When the branch review stops,
    stop before Part B: nothing is pushed. Copy no record line of this review
    into a commit of your own (the procedure's own fix commits keep theirs):
    its SUMMARY goes to Part B step 3, which puts it in the PR body;
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
   - If Check 2 has matches: inspect each — test-data literals and error strings may legitimately contain non-ASCII, but identifiers and non-test comments must not. Fix any unintentional occurrences, run `make fmt && make test && make lint`, and commit before continuing.

7. Run the critical-review subagent procedure in `.claude/commands/_lib/review-subagent-pattern.md` with these inputs:
   - **ARTIFACT**: this phase group's code changes.
   - **PERSONA**: an experienced senior Go engineer and senior SRE. Direct it to surface bugs, missing test coverage, architecture drift, and unclear code.
   - **FILES**: the architecture document and the implementation plan document (paths in `_context.md`), as resolved absolute-path strings; instruct the subagent to read both in full before evaluating the code. Also list the source files added or modified in this phase group as resolved absolute-path strings (read in full). Provide `${PHASE_START}..HEAD` as the commit range and instruct the subagent to run `git diff ${PHASE_START}..HEAD` to see exactly what changed.
   - **CRITERIA**: every item from the phase-group review checklist below, copied verbatim.

   Extra rule: when fixing Critical and Major issues, run the build checks (defined in `_context.md`) and commit before spawning the verification pass.

Phase-group review checklist (use verbatim as evaluation criteria in the subagent prompt above):
- [ ] Implementation is consistent with `02_architecture.md`.
- [ ] Every AC assigned to this phase group by the implementation plan has at least one `test`, `static`, or `diff` verification, or a `manual` reading where the AC specifies only the natural-language content of a document (a mixed AC still needs its test, static, or diff verification), as labeled in the implementation plan's Acceptance Criteria Verification section. Each `diff` script was run on this branch against the merge-base with `origin/main`, and its output and both commit IDs are recorded in the plan's verification row.
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
      adding a table test; a deleted test's coverage was confirmed unchanged.
- [ ] Each test asserts observable behavior rather than restating an
      implementation detail.

7.7. Local CLI review and triage. Run it after step 7.5, so the local CLI review (the CodeRabbit CLI, or Codex CLI when the CodeRabbit CLI fails) sees the code and tests as they stand after the subagent-review fixes. Follow `.claude/commands/_lib/local-cli-review.md` with **BASE** = `${PHASE_START}`, **SCOPE** = phase group, **SUMMARY** = the step 8 summary. When it stops, stop this command and report as it says. Put the record lines it returns in SUMMARY, including any `Local CLI review: not run` line, into the message of the first commit made on the branch after step 7.7 returns: a commit of the next phase group, or the green-gate checkbox commit of step 5a. When no commit follows before Part B (step 8's path for a plan without PR markers), make one that holds only the lines (`git commit --allow-empty -F <file>`). Before a commit that carries a `Codex local review:` line, run the mention check of `_lib/local-cli-review.md` step 7. Step 5a and step 8 read these lines from `git log`.

8. Decide whether to continue or finish.
- If implementation ran this iteration: summarize implementation, verified ACs, assumptions, and deferred items, plus the local CLI review result: each attempt's result (CodeRabbit CLI, Codex CLI as the fallback, or neither) with each failed CLI's failure kind, how many times Codex CLI ran, any range neither CLI reviewed, the counts of fixed / rejected / deferred / unclear findings by CLI with the reason for each rejection and the open question of each `unclear` one, and the note when the reviewed range changes a file Codex reads its instructions from.
- If phases remain: return to step 4 without asking. The next unit is either another phase group of the same PR, or its PR checkpoint (step 5a), where this command opens the PR and stops; the reviewer's checkpoint is the PR review, not a question between phase groups.
- If all phases are complete: verify every AC in `01_requirements.md` is satisfied by the implementation and has at least one `test`, `static`, or `diff` verification, or a `manual` reading where the AC specifies only the natural-language content of a document, as labeled in the plan's Acceptance Criteria Verification section; each `diff` script was run on this branch and its output and both commit IDs are recorded in the plan. Report the final status and any gaps. If the plan has no PR markers (a single-phase plan), open the PR with `_lib/task-pr-handoff.md` Part B as step 5a does (including its LOCAL_REVIEW rule: the whole branch is reviewed again only when a `not run` line is present), with these differences: first check `gh pr view` and `git rev-list --count origin/main..HEAD`, and stop if the branch already has a PR or has no commits; the title is a plan PR title (`_context.md`, "PR title and squash subject"): with no `推奨タイトル`, the prefix is `<type>(<task-id>): ` with a `<type>` from the prefix table that fits the change, and the summary is in Japanese; the body names the plan's main review points; **AFTER_CREATE** = append to the plan's checklist section (§6) the single-PR record `- [x] PR を作成した: <PR URL>` and `- [ ] PR がマージされた`, commit, and `git push`. `/runpr` reads that record: the PR is the task's final plan PR, so NEXT is none and the merge closes the issue.
