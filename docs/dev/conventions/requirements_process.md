# Requirements and Acceptance Criteria Process

When implementing new features or security-critical functionality, follow this process to prevent implementation gaps.

## 0. Review Workflow

### LLM Constraints (Critical)

- LLMs must **always create `01_requirements.md`, `02_architecture.md`, and `03_implementation_plan.md` in draft status (`draft`)**. They must never be created as approved (`approved`).
- A document becomes `approved` only on the reviewer's explicit instruction. Approval is given through `/runpr`'s merge question: when the reviewer chooses to approve the document there, an LLM sets the status (recording the reviewer's name and the date, and the approval line of the change history: "Document Status Format" below). The reviewer does not edit the status to `approved` directly on the PR, because `/runpr` records the approval only for `draft` documents. An LLM never sets `approved` on its own judgment.
- Do not begin creating `02_architecture.md` until the status of `01_requirements.md` is `approved`.
- Do not begin creating `03_implementation_plan.md` until the status of `02_architecture.md` is `approved`.
- Do not begin writing any implementation code until the status of `03_implementation_plan.md` is `approved`.
- If a document with a non-`approved` status is found, do not proceed with subsequent work even if instructed to do so — wait until the status is `approved`.

#### Editing an approved document

Two kinds of edit reach an already-approved document, a decision change and an
editorial correction, and they are handled differently; a decision change found
in a `/runplan` session is handled as a session re-approval:

- **Decision change** — the document now says something different about what to
  build or why. Write the decision itself into the document's body, and set the
  status back to `draft` (which also sets `Review date` and `Reviewer` to `-`,
  as the status format requires). Set `Comments` as "Comments" below says, and
  add a `draft` revert line to the change history ("Change history" below). The
  reviewer approves the document again through `/runpr`, whose approval line
  records the approver; the past approvals stay in the change history. A revert
  made before the PR exists is named in the PR description when the PR is
  created ("Recording in the PR description" below); one `/fixpr` makes after
  the PR exists is not added to the PR description. When the
  document was written under the old rule, migrate it first ("Documents written
  under the old rule" below).
- **Session re-approval** — the exception for a decision change `/runplan` finds
  during implementation: it asks the reviewer in the session, and the
  reviewer's explicit answer is the re-approval. Write the decision into the
  body and record the decision and its reason in a `実装の際に決めたこと（YYYY-MM-DD）`
  section in the overview chapter of the changed document. The status stays
  `approved` and `Comments` is `-`. Add a session re-approval line to the change
  history, and name the decision change in the PR description ("Recording in the
  PR description" below). Migrate a document written under the old rule first.
- **Editorial correction** — the document says the same thing, but a name, a
  cross-reference, a table cell, or a line number has gone stale (a function the
  same task renames, a test the plan reassigned, a phase summary that no longer
  matches the section it summarizes). **Keep the status, and change neither
  `Comments` nor the change history** (a document written under the old rule is
  not migrated for an editorial correction). Record the correction in the message
  of the commit that makes it, as a correction line, and in the PR description,
  as a correction paragraph ("Recording in the PR description" below). Each says
  what was changed to what and that no decision changed, in the writer's own
  words: never quote the text of a review thread or of the PR description.

The distinction exists because the alternative is status thrash: on go-safe-cmd-runner task 0172,
`02_architecture.md` moved between `approved` and `draft` three times in one day,
every time for a wording fix, and each round cost a re-approval that decided
nothing. An LLM classifying its own edit as editorial must say so explicitly in
the PR description and the commit message, so the PR's reviewer can disagree.
The reviewer reads it once during the review, and the PR keeps it afterwards, so
the document does not carry it: a correction recorded in `Comments` stayed there
after every review and grew the field without telling a later reader anything
about the document's current status.

### Review Flow

```
LLM creates 01_requirements.md from an issue with /mkreq (status: draft)
  → Human review and revision (PR review)
  → Reviewer approves in /runpr's merge question; the LLM records approved
  → LLM creates 02_architecture.md with /mkarch (status: draft)
    → Human review and revision (PR review)
    → Reviewer approves in /runpr's merge question; the LLM records approved
    → LLM creates 03_implementation_plan.md with /mkplan (status: draft)
      → Human review and revision (PR review)
      → Reviewer approves in /runpr's merge question; the LLM records approved
      → Proceed to writing implementation code with /runplan
```

Each document goes through its own PR: `/mkreq`, `/mkarch`, or `/mkplan` (which also designs the PR boundaries) opens it and pings the reviewer; `/mkplan2`, which adds PR boundaries to an older approved plan, returns the plan to `draft` and opens a PR the same way. After reviewing it, the reviewer runs `/runpr`, which asks one question at the merge: approve the document and merge, with the next command offered. On that answer it commits the approval and merges; it runs the next command right away only when the user chooses to (`.claude/commands/runpr.md`, steps 4 and 5).

### Document Status Format

Include the following section at the top of `01_requirements.md`, `02_architecture.md`, and `03_implementation_plan.md`:

```markdown
## Document Status

| Item | Value |
|---|---|
| Status | `draft`, `approved`, or `withdrawn` |
| Created | YYYY-MM-DD |
| Review date | YYYY-MM-DD (or `-` when draft) |
| Reviewer | Name (or `-` when draft) |
| Comments | The reason for the current status (or `-` if none) |
```

Five places hold a record about a task document, chosen by who reads it and
when. `Comments` holds what the reviewer needs to know about the current status
now. The change history holds the status transitions, for every later reader.
A `…で決めたこと（YYYY-MM-DD）` section of the body holds a decision and its
reason. The PR description holds what that PR's reviewer reads once, such as an
editorial correction. The commit message holds the correction line the PR
description is written from. A **status transition** is one of four events:
**approval** (`/runpr` sets the document to `approved`, the first time or after
a return to `draft`), **`draft` revert** (a decision change returns an approved
document to `draft`), **session re-approval** (`/runplan`'s reviewer approves a
decision change in the session and the document stays `approved`), and
**withdrawal** (the document is set to `withdrawn`). The revisions of a newly
created `draft` during its own review are not a status transition.

#### Comments

`Comments` holds only the reason for the current status that the reviewer needs
to know now:

| Status | `Comments` |
|---|---|
| `approved` | `-` |
| `draft`, newly created | `-` |
| `draft`, returned by a decision change | `決定の変更で draft に戻した：<変更の要旨>` |
| `draft`, returned along with an earlier document's decision change, its own body unchanged | `<文書のファイル名> の決定の変更に伴って draft に戻した` |
| `withdrawn` | `YYYY-MM-DD 取り下げ：<理由、または理由を記録した文書へのリンク>` |

- Rewrite `Comments` at every status transition, keeping nothing of the
  previous value: approval and a session re-approval set it to `-`, and a `draft` revert or a
  withdrawal sets the value above. The past belongs in the change history.
- Separate two or more items in `Comments` with `<br>`, so the table stays one
  row and each item shows on its own line.
- When a second decision change reaches a document that is already `draft` from
  a decision change (for example, `/fixpr` returns a document to `draft` in one
  round and changes it again in the next), the status does not change: append
  the new value (the form of the table above) to `Comments` after a `<br>`,
  unless the same value is already there. In the change history, when the
  current PR has already added a `draft` revert line for the document (the line
  is not on `main` yet), append the new summary to that line's summary after a
  `；`, unless the same summary is already there (so a rerun does not add it
  twice). When there is no such line (the document reached `main` as `draft`
  from an earlier PR), add a new `draft` revert line for the current PR.

#### Change history

The change history is the section that starts with the heading line
`## 変更履歴` outside a code block and is the last section of the document. A
`## 変更履歴` inside a code block (an example, or a quotation of this rule) is
not the change history; the definition of a document written under the old rule
and the PR-number fill-in of `/runpr` follow this definition.

- **Content:** the status transitions only, one list item per transition, in
  the order they happened; a new line is appended at the end. Never record an
  editorial correction here.
- **Placement:** the last section of the document. When `02_architecture.md` has
  a decision-history appendix (the past decisions and their reasons), the change
  history follows it; the two are separate sections.
- **When it is created:** not when the document is created (the requirements
  template has none). The command that writes the first line creates the
  section at the end of the document; usually this is `/runpr` recording the
  first approval.
- **Lines:**

  | Transition | Line |
  |---|---|
  | Approval | `- YYYY-MM-DD 承認（<承認者>、#<PR 番号>）` |
  | `draft` revert | `- YYYY-MM-DD draft への戻し：<変更の要旨>（#<PR 番号>）` |
  | `draft` revert along with an earlier document's decision change | `- YYYY-MM-DD draft への戻し：<文書のファイル名> の決定の変更に伴う（#<PR 番号>）` |
  | Session re-approval | `- YYYY-MM-DD セッション中の再承認：<変更の要旨>（<承認者>、#<PR 番号>）` |
  | Withdrawal | `- YYYY-MM-DD withdrawn：<理由、または理由を記録した文書へのリンク>（#<PR 番号>）` |

  For example (an illustration of each form, not one document's sequence):

  ```markdown
  ## 変更履歴

  - 旧規則の Comments：<移した旧 Comments の文、内容を変えずに>
  - 2026-11-01 承認（isseis、#606）
  - 2026-11-03 draft への戻し：<変更の要旨>（#610）
  - 2026-11-04 draft への戻し：02_architecture.md の決定の変更に伴う（#610）
  - 2026-11-05 セッション中の再承認：<変更の要旨>（isseis、#612）
  - 2026-11-07 withdrawn：<理由、または理由を記録した文書へのリンク>（#615）
  ```

- **Approver:** the GitHub user name, the value of `gh api user --jq .login`,
  in both the approval line and the session re-approval line.
- **PR number:** every line that reaches `main` carries the number of the PR
  that added it; the migrated item below is the only exception. A line written
  after its PR exists (by `/fixpr` or `/runpr`) writes the number. A line written
  before its PR exists (for example by `/runplan`, `/mkarch`, `/mkplan2`, or a
  manual withdrawal) writes the not-yet-filled marker `#未記入` in place of
  `#<PR 番号>`. `/runpr`'s PRE_MERGE replaces the marker in the change-history
  lines of task documents that the PR added, and only in those (`.claude/commands/runpr.md`, PRE_MERGE); a marker
  already on `main` is shown to the user, never filled in automatically,
  because the PR that added it is not known for certain.
- **`draft` revert along with an earlier document's decision change:** only
  `/fixpr` writes this line, when a changed handoff item returns a later
  document whose body it did not change (`.claude/commands/fixpr.md`, R3).
- **Migrated item:** the old `Comments` of a document written under the old rule
  becomes the first item, starting with the prefix `旧規則の Comments：`. It has
  no date, transition, or PR number, and it is the only exception to one line per
  transition.

#### Documents written under the old rule

A document **written under the old rule** is one that has no change history and
whose `Comments` is not `-`: its `Comments` was written by the rule before this
one, which recorded corrections and past approvals there. It stays valid as it
is. The first time a status transition is written to it under this rule, the
command that writes the transition first does the following, in order:

1. Create `## 変更履歴` at the end of the document, and put the text of the old
   `Comments` cell in it as the first item, after the prefix
   `旧規則の Comments：`. The cell's text is the bytes of the line after its
   leading `| Comments | ` and before its trailing ` |`; do not split it on `|`.
   Copy it byte for byte, `\|` and `<br>` included.
2. Rewrite `Comments` to the reason for the status after the transition
   ("Comments" above).
3. Append the transition's line.
4. Before committing, check mechanically that the copy equals the cell: take
   the cell text of the removed `Comments` line and the text of the added
   item (the bytes after `- 旧規則の Comments：` to the end of the line) from `git diff` into two files and compare them
   with `cmp`. An old `Comments` can be several kilobytes long, and a long text
   an LLM copies can change. When they differ, do not commit.

An editorial correction alone does not migrate the document and does not change
its `Comments`.

#### Recording in the PR description

- **Correction line (commit message):** one line per correction, in English:

  ```text
  Editorial correction: <document path>: <what was changed to what>; no decision changed
  ```

  Only a line that starts with `Editorial correction: ` at its first column is a
  correction line; an indented line or a mention inside a sentence is not. Do
  not start any other line of a commit message with `Editorial correction: `.
  `<document path>` is the repository-relative path of the task document. The
  line contains no `@` or `<`; a command may check it more strictly before it
  posts it (`.claude/commands/fixpr.md`, Phase 4). Its description is written in the
  writer's own words, never quoting a review thread or the PR description.
- **Correction paragraph (PR description):** the PR description records the
  corrections of the correction lines in a paragraph that is not a heading, so it
  never nests under whatever heading ends the description. It carries the date
  and the step that wrote it, so each round adds its own paragraph:

  ```markdown
  **編集上の訂正（<書いた手順>、YYYY-MM-DD）:**

  - `<文書のパス>`: <何を何に直したか>（決定の変更なし）
  ```

  `<書いた手順>` is `PR の作成` or `/fixpr`. Each item is a correction line put
  into Japanese, one item for each correction line. When a PR is created, the paragraph is
  written from the correction lines of `git log origin/main..HEAD`; after that,
  `/fixpr` appends a paragraph for the correction lines of the commits of its
  round. A paragraph already in the description is kept as it is. When the
  range has no correction line, no paragraph is written.
- **Decision changes (PR description):** when a PR is created, its description
  names the decision changes that the `draft` revert lines and the session
  re-approval lines the branch added to change histories show
  (`git diff origin/main...HEAD`).

#### Withdrawing a task

When a task is withdrawn after review (for example, a feasibility check shows the requirements cannot be met), set the status of every task document that exists to `withdrawn`, whatever its previous status. Set `Comments` to the date and the reason, or a link to the document that records it ("Comments" above), and append a withdrawal line to the change history ("Change history" above; `#未記入` when the line is written before its PR exists), migrating a document written under the old rule first. A `withdrawn` document is not `approved`, so the commands that require an approved document (`/mkarch`, `/mkplan`, `/runplan`) stop on it.

---

## 1. Requirements Document (`docs/tasks/XXXX_feature/01_requirements.md`)

**Mandatory for each functional requirement:**
- Define the requirement clearly (what, why, how)
- **Add explicit acceptance criteria** in a dedicated section
- Each acceptance criterion must be:
  - Specific and measurable
  - Independently verifiable
  - Focused on behavior, not implementation
- Assign a **document-wide unique identifier** (`AC-01`, `AC-02`, …) to each acceptance criterion. Rules for managing identifiers:
  - On initial creation, assign sequentially across all requirements (e.g., `F-001` gets `AC-01`–`AC-05`, `F-002` gets `AC-06`–`AC-08`)
  - **Deletion**: remove the criterion and leave the identifier unused — gaps are permitted
  - **Addition**: append the next available number, or use a suffix (`AC-01a`) when inserting between existing identifiers
  - Never renumber existing identifiers — doing so would break references in the implementation plan and test code

**Example format:**
```markdown
#### F-XXX: Feature Name

[Feature description]

**Acceptance Criteria**:
- **AC-01**: [Specific observable behavior #1]
- **AC-02**: [Specific observable behavior #2]
- **AC-03**: [Error handling requirement]
- **AC-04**: [Security requirement]
- **AC-05**: [Edge case handling]
```

**Untrusted input boundaries:** when a requirement consumes untrusted input (network responses, external-command output, or files derived from them), state, for each boundary, the accepted form, the sentinel error a rejection maps to, any size/record limit, and the rule that input the standard library would silently repair, discard, or ignore is rejected rather than accepted wherever the accepted form requires that value. State observable behavior only; how a malformed shape is detected (which API, which pre-check) is decided in `02_architecture.md`. State which members the requirement consumes; an unconsumed member is ignored and accepted only when the requirement explicitly declares it extensible, and an unknown or repeated member the requirement does not declare extensible is a rejection reason. Whole-document checks the accepted form requires, such as raw-byte encoding and a single top-level value with no trailing data, still apply to the whole input. Concrete malformed inputs found during review may be added to the acceptance criteria as examples, but they are not a claim of completeness.

**Design handoff:** when requirements review surfaces implementation-level concerns (library pitfalls, mechanism choices, test construction), record them in `docs/tasks/XXXX_feature/design_handoff.md` instead of expanding the requirements. `02_architecture.md` records, for each handoff item, the approach taken or why it does not apply.

The type-derived checklist below is applied when designing each boundary in `02_architecture.md`. It is an aid against oversights, not a completeness guarantee.

- **Bytes:** is the raw input valid UTF-8 before decoding? (`encoding/json` can succeed by replacing invalid bytes with U+FFFD, so check with `utf8.Valid`.) Are `\uD800`-`\uDFFF` escapes in JSON strings correctly paired? (An unpaired surrogate escape passes `utf8.Valid`, and `encoding/json` replaces it with U+FFFD without returning an error.)
- **Top level:** is the expected JSON kind required? Are `null` and other kinds rejected? Is the input exactly one top-level value, with any non-whitespace trailing data (a second value or junk) rejected? (`json.Decoder.Decode` can succeed after reading only the first value and leave the trailing bytes unread, so confirm that decoding reaches EOF.)
- **Arrays:** is each element the expected kind? Are `null` and non-object elements rejected? Apply this element-kind check (and the type checks inside each element) recursively to nested arrays as well (e.g. `events[].segs`).
- **Numbers:** in addition to sign and integrality, is the value representable in the target Go type (e.g. `int64`)? Is an unrepresentable value (e.g. `9223372036854775808`) rejected? This applies to every numeric field the logic reads, not only the most obvious one.
- **Required vs optional fields:** which fields are mandatory, and how are missing/empty values handled?
- **Identity:** is an identifier that must match the request (e.g. an `id`) verified?
- **Consumed vs unconsumed members:** does the requirement state which members it consumes and which unconsumed members it explicitly declares extensible? Are only the declared-extensible members ignored and accepted, so a boundary the requirement does not declare extensible rejects an unknown or repeated unconsumed member? (Whole-document checks the accepted form requires, such as raw-byte encoding and a single top-level value, still apply.)
- **Size/record limits:** is there a finite limit, with an oversized input rejected via the corresponding sentinel?
- **Partial results:** is no partial result returned on rejection?
- **External-process output capture:** is the capture bounded while still draining the remainder? (Stopping the read at the cap can block the child on a full pipe.) Is the wait after a timeout or cancellation bounded even when a descendant process inherits the pipe and keeps it open? (`exec.CommandContext` kills only the direct child, so set `exec.Cmd.WaitDelay`.)

## 2. Architecture Design Document (`docs/tasks/XXXX_feature/02_architecture.md`)

**Purpose**: High-level design focusing on system structure, component interactions, and design decisions.

**Required sections:**
1. **Design Overview (設計の全体像)**
   - Design principles (設計原則)
   - Concept model with Mermaid diagrams

2. **System Structure (システム構成)**
   - Overall architecture with Mermaid flowcharts
   - Component placement (コンポーネント配置)
   - Data flow with sequence diagrams
   - **Use Mermaid diagram style**: Follow the conventions in [mermaid_reference.md](mermaid_reference.md)
   - **Cylinder nodes for data**: Use `[(data)]` syntax for data sources in flowcharts

3. **Component Design (コンポーネント設計)**
   - Data structure extensions (interfaces, types)
   - High-level interface definitions
   - Component responsibilities

4. **Error Handling Design (エラーハンドリング設計)**
   - Error type definitions (interfaces only)
   - Error message design patterns

5. **Security Considerations (セキュリティ考慮事項)**
   - Security design patterns
   - Threat models with Mermaid diagrams

6. **Processing Flow Details**
   - Key processing flows with sequence/flowchart diagrams

7. **Test Strategy**
   - Unit test strategy
   - Integration test strategy
   - Security test strategy

8. **Implementation Priorities**
   - Phase breakdown
   - Ordered implementation steps

9. **Future Extensibility**
   - Design considerations for future enhancements

**Content guidelines:**
- **Focus on high-level design**: Use diagrams and natural language descriptions
- **Code examples**: Only include high-level code (interfaces, type definitions, error types)
- **Avoid implementation details**: Concrete code belongs in the implementation itself, not the design doc
- **Language**: Japanese (default)
- **Format**: Markdown with Mermaid diagrams

**Reference**: none yet — the first completed task's `02_architecture.md` becomes the reference.

## 3. Implementation Plan (`docs/tasks/XXXX_feature/03_implementation_plan.md`)

**Purpose**: Track implementation progress with actionable tasks and checkboxes.

**Required sections:**
1. **Implementation Overview**
   - Purpose (目的)
   - Implementation principles

2. **Implementation Steps**
   - Organized by phases derived from the architecture document
   - Each step includes:
     - **Files to modify**: Specific file paths
     - **Work content**: What to do (with checkboxes)
     - **Success criteria**: How to verify completion
   - Use checkboxes `[ ]` for tracking: `- [ ] Task description`
   - Mark completed items: `- [x] Completed task`
   - Mark partially completed: `- [-] Partially done (with note)`

3. **Implementation Order and Milestones**
   - Milestone definitions with deliverables

4. **Test Strategy**
   - Unit test coverage goals
   - Integration test scenarios
   - Backward compatibility testing

5. **Risk Management**
   - Technical risks with mitigation strategies
   - Schedule risks with buffer plans

6. **Implementation Checklist**
   - Phase-by-phase checklist with checkboxes
   - Overall completion tracking

7. **Success Criteria**
   - Functional completeness metrics
   - Quality metrics (test coverage, etc.)
   - Security verification requirements
   - Documentation completeness

8. **Next Steps**
   - Post-implementation activities

**Content guidelines:**
- **Focus on tracking**: Use checkboxes extensively for progress tracking
- **Avoid duplication**: Reference other documents instead of repeating content
  - Don't duplicate architecture diagrams or design details
  - Reference sections like "See 02_architecture.md Section 3.2 for design details"
- **Actionable tasks**: Each checkbox should represent a concrete, completable action
- **Update during implementation**: Mark tasks as complete in real-time
- **AC traceability**: Include an explicit "Acceptance Criteria Verification" section
  mapping each AC to the verification that proves it (`diff` when the AC
  states a relation between the state before and after the change; `manual`
  when the AC specifies only the natural-language content of a document; a
  mixed AC still needs its test, static, or diff verification) (see § 4)
- **Language**: Japanese (default)

**Reference**: none yet — the first completed task's `03_implementation_plan.md` becomes the reference.

## 4. Acceptance Tests

**Create appropriate test coverage:**
- Place tests in standard test files (`*_test.go`)
- Follow normal test naming conventions based on what is being tested
- Tests can be unit tests, integration tests, or any appropriate type
- Each acceptance criterion must have at least one `test`, `static`, or `diff` verification (see "Acceptance Criteria Verification" in `03_implementation_plan.md`); a `static` check alone is sufficient only for criteria that are purely about textual presence or absence in code or configuration, never for the presence or absence of a word in a document. An AC that specifies only the natural-language content of a document is verified by a `manual` reading in the PR review instead; a mixed AC still needs the test, static, or diff verification for its value or structure
- A `diff` verification is for an AC that states a relation between the state before and after the change: something is preserved, unchanged, or still present in the same order (for example, a document's existing headings keep their wording and order). Such a property belongs to the change, not to the resulting state, so it is not a permanent test: a permanent test would have to pin a copy of the old values, which `CLAUDE.md` forbids ("Document tests check values, structure, and executable examples, never prose"), or read git history, which makes it depend on the checkout and blocks later changes once the "before" no longer means anything. Instead, a script committed under the task directory (`docs/tasks/NNNN_<name>/_checks/`; `go list ./...` ignores a directory whose name starts with `_`) compares the merge-base with the PR's head. It is run once on the PR branch, and its output and both commit IDs are recorded in the implementation plan's verification row. `make test` and CI do not run it, and it is not maintained after the merge
- Tests must verify the actual behavior, not just the happy path
- Link each verification to its acceptance criterion in the implementation plan

**Traceability in implementation plan:**
Document the verification for each acceptance criterion in `03_implementation_plan.md`
(`test`, `static`, `diff` for an AC that states a before/after relation, or
`manual` for an AC that specifies prose alone):

```markdown
**AC-01: [First acceptance criterion]**
- Verification: `internal/package/subpackage_test.go::TestFunctionName`
- Implementation: `internal/package/subpackage.go:123-145`
- Verification method: [How to verify]

**AC-02: [Second acceptance criterion]**
- Verification: `internal/package/integration_test.go::TestIntegrationScenario`
- Implementation: `internal/package/another.go:67-89`
- Verification method: [How to verify]
```

**Example test:**
```go
// TestFoo verifies that [behavior].
func TestFoo(t *testing.T) {
    // Test implementation that verifies the specific criterion
}
```

Traceability between this test and its acceptance criteria is recorded in
`03_implementation_plan.md` (see "Traceability in implementation plan" above),
not as `AC-NN`/`F-NNN` references in source comments — `runplan.md`'s
pre-commit checks reject such references in Go source.

## 5. Pre-Commit Checklist

Before considering a feature complete:
- [ ] All acceptance criteria defined in requirements document
- [ ] Architecture design document created with high-level design
- [ ] Implementation plan created and updated during development
- [ ] Acceptance criteria verification section present in implementation plan
- [ ] At least one test or `static` verification per acceptance criterion (or a `diff` verification for an AC that states a before/after relation, or a `manual` reading for an AC that specifies only the natural-language content of a document)
- [ ] All acceptance tests pass
- [ ] Security requirements explicitly tested

## 6. Background

This process was established after a critical security gap was discovered in a feature: a requirement explicitly stated that certain files should be subject to checksum verification to detect tampering, yet the implementation omitted this check entirely. The gap occurred because:

1. Requirements lacked explicit acceptance criteria
2. No verification phase traced each criterion to a test
3. No tests specifically validated the security requirement

This process ensures such gaps do not recur by requiring explicit acceptance criteria for every functional requirement, and traceability between each criterion and its test.
