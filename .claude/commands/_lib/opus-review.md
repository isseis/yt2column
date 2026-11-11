---
description: "Internal: shared Opus review, a correctness review and a weak-model failure review run on Opus (not a command)"
---

# Shared procedure: the Opus review

The Opus review runs two reviews of a commit range on Opus, merges their
findings, fixes the Critical and Major ones, and confirms the fixes with a
verification review that also runs on Opus:

- the **correctness review**, which looks for correctness defects (bugs, and
  divergence from the requirements, the architecture, and the plan), with the
  "Correctness checklist" below;
- the **weak-model failure review**, which looks for the failures a
  lower-capability model makes when it follows `runplan.md`: satisfying the
  literal instruction while dropping the judgment it depends on, with the
  "Weak-model failure checklist" below.

Neither review reports cleanup (reuse, simplification, efficiency): that is
`/simplify`'s scope, outside this review.

This file defines the procedure once. It has two entry points: entry 1 decides
whether a caller needs the review, and entry 2 is the review itself.
`weakreview.md` calls entry 2. Entry 1 is not defined yet.

Project-specific values (the build checks, the PR review request whose first
word is `<review-mention>`) are in `.claude/commands/_context.md`. The prompt
assembly, panel mode, and convergence rules are in
`.claude/commands/_lib/review-subagent-pattern.md`; this file says below which
of its rules it uses, which it adds to, and which it replaces.

This file and the prompts it builds never name a concrete model ID (one that
carries a model version). A subagent that is not Opus could copy such an ID
from its prompt when it reports its own model ("Running on Opus").

## Record lines

A record line is a whole line of a commit message, starting at its first
column, in one of the forms below. A line that is indented, or that mentions
a form inside a sentence, is not a record line. Never start a line of a
commit message's prose with one of these prefixes. Only a line in the full
form is read; a line that starts with a prefix but does not match its form is
read as no record.

**Opus.** A model ID is Opus when it starts with `claude-opus-`, and only
then. This is the only rule that decides whether a model is Opus; "Running on
Opus" below uses it.

**Kept finding.** One line for each Critical or Major finding the review moved
past without fixing:

```text
Opus review: kept <path>:<line> (<severity>; <reason>)[ <summary>]
```

- `<path>:<line>`: the finding's location, a repository-relative path and a
  line number in the commit the finding was raised on. A finding that names a
  section or a checklist item but no line takes the line where that section or
  item starts.
- `<severity>`: `critical` or `major`.
- `<reason>`: `rejected by human` (the human rejected the finding, "Fixing
  and verifying" below) or `decision change declined` (fixing it needed a
  decision change, and the human did not approve the change).
- `<summary>`: the finding's gist in your own words, never copied from a
  subagent's text. It is left out when the mention check below removed it.
- Write it when the human answers, in an empty commit that holds only the
  record lines (`git commit --allow-empty -F <file>`), so a session that stops
  and runs again does not ask the human the same question again.

**Mention check.** Before every commit this procedure makes (a fix commit or a
commit of record lines), write the message to a file and run
`grep -qiF -- '<review-mention>' <file>`. Commit only when it exits `1`. Exit
code `0` means the message contains the mention, and any other code means the
check failed. When the first run does not exit `1`, remove every `<summary>`
of a record line and every sentence of the message written from a finding, and
run the check once more; commit when that run exits `1`, and otherwise
return `stopped-error` instead of committing. Never
copy a subagent's text into a commit message, fix commits included.

## Entry 2: the review

### Inputs the calling command must supply

| Input | Meaning |
|---|---|
| RANGE | `<base>..<head>`, both resolved to full commit IDs before the review starts. When FIX is `fix`, `<head>` is `HEAD` at the start: the fix commits go on top of it |
| TASK_DIR | The repository-relative path of the task directory whose documents the reviews read (such as `docs/tasks/0081_implementation_model_choice`), not its bare name |
| FIX | `fix`: fix the Critical and Major findings and commit the fixes. `report`: change nothing and report |
| DECISION_CHANGE | What to do with a finding whose fix needs a change to a decision of the requirements, the architecture, or the plan. The caller gives the rule |

### Outputs

| Output | Meaning |
|---|---|
| OUTCOME | `completed` (no Critical or Major finding is left unhandled), `reported` (FIX is `report`), `stopped-unavailable` (a review could not run on Opus), or `stopped-limit` (the pass limit stopped the loop), or `stopped-error` with its reason (any other stop this file names) |
| FINDINGS | The merged findings, each with its severity, location, and gist in your own words, and the count for each severity |
| KEPT | The Critical and Major findings moved past without a fix, each with its reason: the kept lines, and the findings DECISION_CHANGE left unfixed |
| COMMITS | The fix commits and the commits of record lines this procedure made |
| CATEGORIES | One line for each category of both checklists: clean, flagged, or not applicable to RANGE |
| READ | A one-line overall read: is RANGE's failure profile consistent with a lower-capability model cutting corners on judgment-heavy rules (expected: fix and move on), or does it suggest the plan or the architecture itself was unclear (escalate: revise the plan before more phase groups run with the same model)? |

### Procedure

1. **Read the documents** the work in RANGE was supposed to follow:
   TASK_DIR's `01_requirements.md`, `02_architecture.md`, and
   `03_implementation_plan.md`, `docs/dev/conventions/test_organization.md`,
   `docs/dev/conventions/requirements_process.md`, and
   `docs/dev/security.md`.

2. **Run the deterministic pre-checks** below over RANGE.

3. **Run the two reviews** in panel mode of
   `_lib/review-subagent-pattern.md` ("Panel mode", steps 1 and 2), as two
   Agent tool calls in one message, with "The review prompts" below:
   - **Reviewer A, the correctness review.** PERSONA: a senior engineer who
     finds correctness defects. Mandate: bugs (logic, error paths, boundary
     values, resource release) and divergence from the ACs of
     `01_requirements.md`, from `02_architecture.md`, and from the step
     descriptions of `03_implementation_plan.md`. Out of scope: reviewer B's
     mandate, test coverage, and cleanup. CRITERIA: the "Correctness
     checklist".
   - **Reviewer B, the weak-model failure review.** PERSONA: a skeptical
     senior Go engineer and senior SRE who has reviewed many PRs produced by
     a lower-capability LLM executing a detailed implementation plan. Their
     default assumption is that any instruction requiring cross-document
     synchronization, an implicit invariant, or resisting a shortcut was
     *not* actually carried out just because the surface artifact
     (checkbox, test, comment) suggests it was — each such claim must be
     checked against the actual diff, not taken on faith. Mandate: the
     "Weak-model failure checklist". Out of scope: reviewer A's mandate and
     cleanup.
   - CRITERIA are pre-routed (panel mode (a)): each reviewer gets only its own
     checklist, copied verbatim, and reports an issue outside its mandate only
     as a one-line OUT-OF-LANE FLAG, when it spots one.

4. **Check that both reviews ran on Opus** ("Running on Opus"). When either
   did not, return `stopped-unavailable`.

5. **Merge the findings** as panel mode's "Synthesize before fixing" says
   (step 3): drop duplicates, take the higher of two severities, rebuild the
   issues that sit between the two mandates, and promote an OUT-OF-LANE FLAG
   that holds up against the owning reviewer's mandate. Use the Critical,
   Major, and Minor scale; do not use `_lib/triage-pattern.md`, whose
   `invalid` judgment would let you reject a finding on your own judgment.
   Write CATEGORIES from the reviewers' category lines, and READ yourself.

6. **When FIX is `report`**, change nothing and return `reported`.

7. **When FIX is `fix`**, fix and verify ("Fixing and verifying"), then
   return `completed`, or the stop that loop returns.

### Deterministic pre-checks

These are mechanical and cheaper than AI review — catch them here, not in the
subagent pass. Run them over the changed files in RANGE, with RANGE and
TASK_DIR put in for `<RANGE>` and `<TASK_DIR>`. First confirm that
TASK_DIR is a directory (`test -d <TASK_DIR>`): with a bare directory name,
Check 2 would find no document and print `OK`. When it is not, return
`stopped-error` before the reviews. Write the block to a file in the
scratchpad and run it with `bash <file>` from the top level of the work tree
(the block's paths are relative to it), not in the session's shell: in zsh, the block's `path` variable would replace the command
search path. The block's last line exits `1` when no Go file changed; that is
not a failure.

```bash
RANGE="<RANGE>"
CHANGED_GO=$(git diff $RANGE --diff-filter=d --name-only | grep '\.go$' || true)
CHANGED_ALL=$(git diff $RANGE --diff-filter=d --name-only || true)

# Check 1: planning-doc references leaked into source (should live only in the plan doc)
if [ -n "$CHANGED_GO" ]; then
  echo "$CHANGED_GO" | xargs rg -n '\bAC-[0-9]+[a-z]?\b|\bF-[0-9]+[a-z]?\b' 2>/dev/null \
    && echo "FLAG: planning-doc references in Go source" || echo "OK: no planning-doc references"
fi

# Check 2: draft-status documents with commits already built on top of them
NON_APPROVED=""
for f in 01_requirements.md 02_architecture.md 03_implementation_plan.md; do
  path="<TASK_DIR>/$f"
  rg -n '^\| Status \|' "$path"
  if [ -f "$path" ] && ! rg -q '^\| Status \|.*approved' "$path"; then
    NON_APPROVED="$NON_APPROVED $f"
  fi
done
# Any non-`approved` status for a document whose successor exists (e.g. a
# 03_implementation_plan.md exists while 02_architecture.md is still draft)
# means implementation started before its gate — FLAG regardless of what
# the commit history says about being told to proceed.
if [ -n "$NON_APPROVED" ] && [ -f "<TASK_DIR>/03_implementation_plan.md" ]; then
  echo "FLAG: non-approved doc(s) with a successor doc present:$NON_APPROVED"
else
  echo "OK: no non-approved doc precedes a successor doc"
fi

# Check 3: Dockerfile base images pinned by tag instead of digest
CHANGED_DOCKERFILES=$(git diff $RANGE --diff-filter=d --name-only | grep -i dockerfile || true)
if [ -n "$CHANGED_DOCKERFILES" ]; then
  echo "$CHANGED_DOCKERFILES" | xargs rg -n '^FROM .*:[a-zA-Z0-9._-]+\s*$|^FROM .*:[a-zA-Z0-9._-]+\s+[Aa][Ss]\s+\S+\s*$' 2>/dev/null \
    && echo "FLAG: tag-pinned FROM (expect @sha256: digest)" || echo "OK: no tag-only FROM lines"
else
  echo "OK: no Dockerfile changes"
fi

# Check 4: build-tagged files actually reachable by `make lint`
[ -n "$CHANGED_GO" ] && echo "$CHANGED_GO" | xargs rg -l '^//go:build' 2>/dev/null
# For each hit with a tag other than a bare `test`, confirm the Makefile's
# lint target actually passes that tag (grep the Makefile / CI config for
# -tags). A file only ever compiled under a tag no invocation passes is a
# silent dead corner.

# Check 5: non-ASCII in Go source (same as runplan.md step 6.5 — re-verify,
# since a weak model's self-check in runplan step 5 may have been superficial)
[ -n "$CHANGED_GO" ] && echo "$CHANGED_GO" | xargs rg -Pn '[^\x00-\x7F]' 2>/dev/null
```

Note every FLAG before the reviews start, and give the FLAGs to both
reviewers — do not let a reviewer re-discover what a grep already found.

### The review prompts

Build each prompt as `_lib/review-subagent-pattern.md`, "Procedure", says
(persona, files, criteria, and the Severity / Location / Problem / Suggestion
output format), and add these to every prompt, of the discovery reviews and of
the verification review alike:

- The first line of the output is `Model: <model-id>`, where `<model-id>` is
  copied from the subagent's own system prompt, which states the model it runs
  on. Name no model ID in the prompt.
- Report no cleanup finding (reuse, simplification, efficiency). The
  weak-model checklist's item on reimplemented logic is not cleanup: it looks
  for a weak model rebuilding an existing function under a new name, and stays
  in reviewer B's scope.
- The pre-check FLAGs, to verify in context rather than re-derive.
- RANGE, with the instruction to run `git diff <RANGE>` and `git log <RANGE>`
  itself.
- The kept lines, when there are any: the `Opus review: kept` lines in
  `git log <base>..HEAD` when FIX is `fix` (this run's kept lines included),
  and in `git log <RANGE>` when FIX is `report`, which commits nothing and may
  run on a branch other than the range's: a finding with the same gist near the
  same place in the same file was already decided by the human and is not
  raised again. Fixes shift line numbers, so a matching location alone does
  not decide it.
- One line for each category of the reviewer's checklist, stating whether it
  was clean, flagged, or not applicable to RANGE.

FILES: the documents of step 1 and every file RANGE changes and does not
delete (`git diff --name-only --diff-filter=d <RANGE>`), all as resolved
absolute paths.

### Running on Opus

- Run every review of this procedure (both discovery reviews and every
  verification review) with the Agent tool's `model` set to `opus`. Do not
  use the `fork` subagent type: a fork ignores `model`. Panel mode's and
  "Model tiering"'s permission to run a verification review on a cheaper
  model does not apply.
- Check each review's output: write its first line, alone, to a file in the
  scratchpad, and run `grep -Eqx 'Model: claude-opus-[!-~]+' <file>`. The
  review ran on Opus only when the check exits `0`; any other exit code (a
  line of another form, another model, a failed check) means it did not.
- When an Agent tool call fails, run it once more. When the second call fails
  too, the review did not run on Opus.
- When a review did not run on Opus, never run it on the session's model
  instead: return `stopped-unavailable`. The fix commits and the kept lines
  made before the stop stay on the branch.
- This check keeps the review's quality; it is not a security boundary. A
  model that is not Opus can still copy an Opus ID it finds in its context,
  which is why no concrete model ID is written in this file or in a prompt.

### Fixing and verifying

This replaces "Fix all Critical and Major issues" and "Apply Minor fixes at
your discretion" of `_lib/review-subagent-pattern.md`, "Procedure". Its pass
limit applies as it is.

1. A merged finding that matches a kept line already in
   `git log <base>..HEAD` (this run's kept lines included)
   (same gist, near the same place in the same file) was already decided by
   the human: do not ask about it again and do not fix it. List it in KEPT.
2. When you think a Critical or Major finding is wrong, do not reject it on
   your own judgment. Before fixing, ask the human with AskUserQuestion,
   giving its location, its gist, and your view. Fix it unless the human
   rejects it. For each finding the human rejects, commit a kept line with
   the reason `rejected by human` at once ("Record lines").
3. A finding whose fix needs a change to a decision of the requirements, the
   architecture, or the plan follows DECISION_CHANGE. When the caller's rule
   asks the human and the human does not approve the change, commit a kept
   line with the reason `decision change declined`.
4. Fix every other Critical and Major finding. Run the build checks
   (`_context.md`) and commit, after the mention check ("Record lines"). When
   the build checks fail and cannot be made to pass within the finding's
   scope, return `stopped-error`, leaving the uncommitted fix in the working
   tree for the caller's report.
5. Do not fix a Minor finding: report it in FINDINGS. Do not record it as an
   issue, as "Convergence" would; this procedure writes nothing to GitHub.
6. When nothing was fixed (the human rejected every finding, or every finding
   was kept), run no verification review.
7. Otherwise run one verification review on Opus ("Running on Opus") over
   `<base>..HEAD`, which now holds the fix commits. Give it the persona of
   the reviewer, or of both reviewers, whose findings you fixed, the list of
   those findings, and the checklist or checklists they came from, with "The
   review prompts" added. Its mandate is to confirm that each fix is correct
   and introduced no new defect within those checklists.
8. When it reports a Critical or Major finding, go back to item 1 for those
   findings. The discovery reviews count as the first pass. After the third
   pass, go on only as "Procedure" says (the remaining findings are concrete,
   scoped to RANGE, and fixable without widening the scope); otherwise return
   `stopped-limit`, listing the remaining findings. Count a new finding of a
   verification review in FINDINGS.

## Correctness checklist

(Reviewer A's criteria. Test coverage and cleanup are out of scope: report a
missing test or a possible simplification only when it hides a defect, as the
defect.)

**Bugs**
- [ ] Each changed function, script, and procedure step computes what its
      callers and its documentation expect for every input it can receive,
      not only the inputs the tests use. Trace at least one non-trivial input
      through the changed code.
- [ ] Every error is checked and carries its cause; no failure is swallowed,
      turned into a success, or reported as a different failure; state is
      left consistent on the error path.
- [ ] Boundary values behave: empty input, zero, one, the maximum, the first
      and last element, a missing file or ref, and off-by-one ranges.
- [ ] Every resource acquired in the range (a file, a process, a temporary
      directory, a lock, a connection) is released on every exit path.
- [ ] Code that shares state between goroutines, processes, or steps of a
      procedure has no race and no ordering assumption the code does not
      enforce.
- [ ] Every command, script, and procedure step written in the range runs as
      written: its quoting, its exit-code handling, and the paths, names, and
      sections it refers to exist; a step's condition agrees with the other
      steps that read or write the same state.

**Divergence from the approved documents**
- [ ] Every AC this range claims to implement (the plan's Acceptance Criteria
      Verification section) is met by the change, read against the AC's own
      text in `01_requirements.md`, not against a summary of it.
- [ ] The change does what `02_architecture.md` and the plan's step
      descriptions say: no behavior differs from them unless the difference
      was recorded as `runplan.md` step 5 requires (an editorial correction,
      or a decision change the human approved).
- [ ] The change implements nothing the documents put out of scope or assign
      to a later PR.

## Weak-model failure checklist

(Use verbatim as evaluation criteria in the subagent prompt. Each item names
the concrete thing to check, not just the underlying rule, since these are
exactly the checks a weaker executing model would have applied only
superficially.)

**Invariant discipline**
- [ ] For every generated identifier (IDs, names, per-call tokens), the code
      enforces uniqueness/length constraints the plan or architecture implies
      — not just "looks unique in the happy path". Check the actual generation
      site, not just its call sites.
- [ ] Every flag or mode documented as suppressing side effects (`--dry-run`
      and equivalents) is checked at *every* side-effecting call site it
      should gate (deletes, unfollows, network sends), not only the ones
      exercised by the tests actually added in this range.
- [ ] Every session-scoped or resource-scoped block (`WithSession`-style
      helpers, opened clients, acquired locks) releases its resource on
      **every exit path**, including panics and early returns/errors — not
      only the success path. Read the actual `defer`/`recover` placement,
      don't infer it from a comment saying it does.

**Unearned mechanism** (a weak model closes a finding by building something,
and the thing it builds is rarely load-bearing)
- [ ] Every optimization in this range (cache, pre-filter, fast path, pooled
      buffer) is justified against an absolute budget stated in the commit
      message, not against a relative delta versus the previous commit
      ("1.9x slower" with no conversion to real wall time is a finding).
      Check whether the stage optimized is even a significant fraction of
      its path's cost, and whether the cached step leaves the expensive
      setup still running per call.
- [ ] For each mechanism added in this range, ask what deleting it would
      change. If the answer is only a benchmark number, it is a finding —
      especially where the regression it offsets was introduced by an
      earlier commit in this same range (the model creating and then
      solving its own problem).
- [ ] No optimization added here carries an unstated correctness
      obligation: byte-wise case folding where the real path folds under
      Unicode rules, an ASCII assumption over arbitrary input, a fast path
      that must agree with the slow path, a cache whose key omits an input.
      Read the fast path against the slow one; do not trust a comment.
- [ ] No behavior change and its motivating optimization share a commit.

**Inference where declaration belongs**
- [ ] No code path is selected by inspecting the content of caller-supplied
      data (`strings.Contains(x, ":")`, `HasPrefix`, length checks) where
      the intent could be carried in the type. The tell: the intent is
      documented in a comment next to the data's definition while the
      routing lives in a condition in a different file, so nothing ties the
      two together and a value that happens to contain the sniffed
      character silently takes the wrong path.
- [ ] Any `switch` added over a kind/mode enum has a `default` branch, and
      that branch fails secure rather than falling through to the most
      permissive case.
- [ ] Any invariant this range claims ("patterns are always validated",
      "this is only ever built by X") is enforced by the type — unexported
      fields plus a constructor — not by convention. If an exported field
      was kept as an extension point, the range shows the count of its
      actual uses.
- [ ] No helper added here silently repairs a malformed caller input
      (trimming, defaulting, coercing) where rejecting it would surface a
      programming mistake.

**Tests that cannot fail**
- [ ] Every test added in this range can fail for the reason it names.
      Spot-check by nil-ing the collaborator or reverting the branch it
      covers; a test that still passes is a finding. Highest-yield targets:
      tests over a layered path (two mechanisms that produce the same
      output) whose assertions are only "output differs from input" or
      "output contains the placeholder" — those cannot say which layer ran.
- [ ] No test asserts that a constant equals its own literal, that a field
      holds what was just assigned to it, or that two literals are equal.
- [ ] No table test added for a helper duplicates rows the caller's table
      already covers with the same literals, where the helper differs from
      the caller only by a loop.
- [ ] Any test deleted in this range is the subject of a commit that holds
      nothing but test deletions, and that commit's message and the PR
      description carry the passing output (through its last line) of
      `make coverage-compare BASE=<that commit's parent>` and/or
      `make ext-coverage-compare BASE=<that commit's parent>` — both when both
      kinds of test were deleted.
- [ ] No `diff` verification added in this range (a `_checks/` script, or a
      diff record in the plan) is oversized for what it checks: weigh what
      the reviewer will read (the script, or the recorded output) against the
      size of the diff it verifies, and a script in a language other than
      shell has the plan's reason why shell is not enough, and a script is not
      copied from or modeled on an earlier task's `_checks/`. This item does
      not apply to a plan approved before this item reached `main`, nor to its
      `_checks/`.

**Cross-document synchronization**
- [ ] If this diff's implementation diverges from `02_architecture.md` or
      the corresponding step description in `03_implementation_plan.md`
      (a simpler approach was substituted, an assumed API turned out
      different), the plan's *description text* (not only its checkbox) and
      the architecture section were updated in the same commit range. A
      `[x]` with stale prose describing the old approach is a finding.
- [ ] `03_implementation_plan.md`'s Acceptance Criteria Verification section
      still names real test locations (`path::TestName`) or real static
      commands for every AC this range claims to close — not a placeholder
      or a description of a test that does not exist under that name. An AC
      that specifies only the natural-language content of a document may name
      a `manual` reading in the PR review instead; a mixed AC still needs the
      test or static verification for its value or structure.

**Approval-gate integrity**
- [ ] No implementation commit in this range is dated/ordered before its
      governing document (`01_requirements.md`/`02_architecture.md`/
      `03_implementation_plan.md`) reached `approved` status. If any
      commit message or plan history suggests the model proceeded because
      a user asked it to despite a draft-status gate, flag it regardless of
      the user request — the gate has no override clause.

**Traceability leakage**
- [ ] No `AC-NN` / `F-NNN` planning identifiers appear in Go source comments,
      identifiers, or string literals (confirm the pre-check grep result was
      accurate, i.e. no escaped/split variant like `AC` + `-01` across a
      line break defeats it).
- [ ] Conversely, every AC this range claims to satisfy has a corresponding
      entry in the plan's Acceptance Criteria Verification table — a test
      added in the diff with no plan-side traceability entry is a finding
      (the model implemented the behavior but skipped the paperwork the
      process depends on for future audits).

**Shortcut resistance**
- [ ] Any removal of 3+ scattered same-shaped code sites (test functions,
      duplicated blocks) in this range was done as discrete edits, not a
      generated brace-counting / regex-based bulk deletion — check for
      accidental over-deletion of adjacent code (a truncated function, a
      dangling brace, an orphaned comment) as the tell.
- [ ] Any newly authored non-code artifact in this range (runbook step,
      command example, table, translated text) was checked against ground
      truth (the command was actually run, the cited implementation exists
      at the cited location, the translation matches its source) — not
      verified only by confirming an old term is absent.

**PR/phase boundary judgment**
- [ ] The set of changes in this range corresponds to a coherent phase
      group per `03_implementation_plan.md` — it doesn't stop mid-phase in
      a state that cannot pass `make test` on its own, and it doesn't bundle
      unrelated later-phase work that the plan intended to separate.

**Security-adjacent omissions** (judgment-heavy; a weak model satisfies the
ticket but misses the neighboring risk — check `docs/dev/security.md`'s
attack-vector list against this diff even where no AC mentions it explicitly)
- [ ] Any new or modified network response handling (HTTP client reads,
      pagination loops) bounds response size and iteration count — an
      unbounded `io.Copy`/`Decode` or a pagination loop whose only stop
      condition is a value an untrusted server controls is a finding, per
      the response-size rule in `docs/dev/security.md`.
- [ ] Any new error path that wraps an HTTP client error, request, or
      response object does not carry `Authorization` headers or session
      tokens through to a log line or notification payload — check what the
      wrapped struct actually serializes to, not whether the code "looks
      like" it redacts.
- [ ] Any new external-endpoint resolution (redirect following, webhook target
      validation, YouTube URL / video-ID validation) fails closed on
      mismatch/ambiguity rather than falling back to a default or a
      best-effort continue.
- [ ] Any new retry/backoff loop has a bounded ceiling (attempts or elapsed
      time), not an open-ended loop relying on the remote side to eventually
      succeed or fail.
- [ ] Any new interruption/partial-failure path (timeout mid-operation,
      crash-and-resume) is treated as "outcome unknown, safe to retry" and
      not silently promoted to either "succeeded" or "failed" — check this
      against the operation's actual idempotency guarantee, not the
      surrounding code's comment.

**Language and convention discipline**
- [ ] Every Go comment, identifier, and string literal added in this range
      is English, except a literal the guard-test literal exception of
      `CLAUDE.md`, "Go Idioms", covers; any non-ASCII found by the pre-check
      grep is in a test-data literal, an intentionally localized error
      string, or such an excepted literal (a Go program under `_checks/`
      whose literal must match a Japanese document's text), not an identifier or a
      non-test comment (a weak model tends to apply this rule uniformly
      instead of case-by-case).
- [ ] Any new test file's assertion style (testify `assert`/`require` vs.
      hand-rolled `if`/`t.Errorf`), package naming (`testutil` vs.
      package-internal `test_helpers.go`), and import style match the
      convention of an existing file in the same package — evidence the
      model actually read a sibling file rather than generating from a
      generic template.
- [ ] No logic is reimplemented where an existing function in the codebase
      already does it — check for a near-duplicate helper, not just an
      identical one (weak models often rename rather than reuse).
