---
description: "Internal: shared local CodeRabbit review pattern (not a command)"
---

# Shared pattern: local CodeRabbit CLI review and triage

`runplan.md` (step 7.7), `runchange.md`, and `_lib/doc-pr-handoff.md` (Part B)
review committed changes with the
`coderabbit` CLI before the PR-side review, then triage and fix the findings.
This file defines the procedure once. A calling command invokes it by saying
"follow `_lib/coderabbit-local-review.md`" and supplying the inputs below.

Project-specific values (build checks) are in `.claude/commands/_context.md`.
Findings are classified as `.claude/commands/_lib/triage-pattern.md` defines.
The convergence rules are in `.claude/commands/_lib/review-subagent-pattern.md`.

## Inputs the calling command must supply

- **BASE** — the commit the review range starts from (`coderabbit` takes it as
  `--base-commit`), so the range covers exactly the commits under review.
- **SCOPE** — what BASE..HEAD is called in the calling command's vocabulary
  (e.g. "phase group"); used for the run cap and the summary.
- **SUMMARY** — where the calling command reports the result (e.g. the step 8
  summary).

## Authorization

`coderabbit review` sends the diff to an external service. `CLAUDE.md`
("Tool Execution Safety") grants it standing approval as a development tool used
on every change, so run it without asking the user.

## Procedure

1. Confirm `git status --short` is empty (`--committed` ignores uncommitted
   changes), then run:

   ```bash
   coderabbit review --agent --committed --base-commit "<BASE>" -c CLAUDE.md \
     > "<scratchpad>/coderabbit-<BASE>.txt"
   ```

   If the command fails (authentication, usage limit, network), do not retry and
   do not stop the run: skip the rest of this procedure and report the failure in
   SUMMARY.

2. Triage each finding yourself, in this session. Read the code at the cited
   location and classify it as `triage-pattern.md` defines (`verdict`,
   `severity` with its calibration rule, `topic`, `replyBody`):
   - When a finding concerns a process document under the task root
     (`docs/tasks/NNNN_<name>/`), including prose written there in this run, also
     apply that file's abstraction-level check and use the `off-level` verdict
     where it applies; handle an `off-level` finding as `fixpr.md` "Shared rules"
     (R1-R5) describe. For any other file, the verdicts are `valid`, `invalid`,
     and `unclear`.
   - A finding that repeats one already fixed by an earlier review of the same
     changes is closed as such; do not fix it twice.
   - For a performance finding, measure the cost before adding a mechanism
     (`CLAUDE.md`, Performance).

3. Act on the triage:
   - Fix every `must-fix`. Fix `worth-fixing` at your discretion.
   - Leave `no-harm` and `invalid` findings unchanged; note one line of reason
     for each.
   - Record each `unclear` finding in SUMMARY as unresolved (finding, location,
     why it is uncertain); do not drop it.
   - Apply the Convergence rules in `review-subagent-pattern.md`: prefer deleting
     to adding when closing a finding, and do not chase a mirror case of a rule
     just fixed.
   - After fixing, run the build checks (`_context.md`) and commit.

4. Run `coderabbit review --agent ...` at most twice per SCOPE. Run the second
   time only if the first led to a fix; from the second run on, act on
   `must-fix` only. After the second run, stop once every remaining `must-fix`
   finding is addressed; record the remaining `worth-fixing` findings as
   `deferred` in SUMMARY, and record the reason for each `no-harm` and `invalid`
   finding. The reviewer returns findings whenever it is asked, so "it found
   something" is not a reason for another run.

5. After triage, run `coderabbit review findings --clear` so stored findings are
   not surfaced again. Record each rejected finding and its reason (a line in the
   commit message of the fix commit, or in SUMMARY) so the later PR-side
   CodeRabbit review and `/fixpr` do not re-litigate it.
