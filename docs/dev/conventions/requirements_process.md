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
- **Exception: the simple process.** In a task whose `01_requirements.md`
  records the simple process (its `Process` row is `` `simple` ``: "Process
  record" below), `/mkreq` writes `02_architecture.md` and
  `03_implementation_plan.md` before `01_requirements.md` is approved, in the
  same PR as `01_requirements.md` ("Simple Process" below). This lifts, for
  `/mkreq` writing the three documents of such a task (in one run, or in a run
  that continues a stopped one), the two rules above about when
  `02_architecture.md` and `03_implementation_plan.md` may begin, and the rule
  above about not proceeding past a document that is not `approved`. The
  exception reaches no further: no implementation code is written until all
  three documents are `approved`.

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
  document has no change history, migrate it first ("Documents without a change
  history" below).
- **Session re-approval** — the exception for a decision change `/runplan` finds
  during implementation: it asks the reviewer in the session, and the
  reviewer's explicit answer is the re-approval. Write the decision into the
  body and record the decision and its reason in a `実装の際に決めたこと（YYYY-MM-DD）`
  section in the overview chapter of the changed document. The status stays
  `approved` and `Comments` is `-`. Add a session re-approval line to the change
  history, and name the decision change in the PR description ("Recording in the
  PR description" below). Migrate a document without a change history first.
- **Editorial correction** — the document says the same thing, but a name, a
  cross-reference, a table cell, or a line number has gone stale (a function the
  same task renames, a test the plan reassigned, a phase summary that no longer
  matches the section it summarizes). **Keep the status, and change neither
  `Comments` nor the change history** (a document without a change history is
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

The flow above is the **full process**. `/mkreq` asks each time it writes a
`01_requirements.md` whether the task takes the full process or the **simple
process**, and records the answer ("Process record" below). The simple process
writes the three documents in one PR ("Simple Process" below):

```
LLM asks in /mkreq which process to take; the reviewer chooses the simple process
  → LLM confirms the requirements, design, and plan decisions with the reviewer
  → LLM creates 01_requirements.md, 02_architecture.md, and
    03_implementation_plan.md with /mkreq, one commit each, and opens one PR
    (status: draft)
    → Human review and revision (PR review)
    → Reviewer approves in /runpr's merge question; the LLM records approved
      for all three documents
    → Proceed to writing implementation code with /runplan
```

Before that PR is merged, the reviewer can return it to the full process
instead, in the same merge question: the PR then holds only
`01_requirements.md`, and the task continues with `/mkarch` in the full process
above.

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

`01_requirements.md` alone also has a `Process` row, right after `Status`
("Process record" below).

Six places hold a record about a task document, chosen by who reads it and
when. `Comments` holds what the reviewer needs to know about the current status
now. The change history holds the status transitions, for every later reader.
A `…で決めたこと（YYYY-MM-DD）` section of the body holds a decision and its
reason. The PR description holds what that PR's reviewer reads once, such as an
editorial correction. The commit message holds the correction line the PR
description is written from. An ADR under `docs/dev/adr/` holds a major
decision across tasks: its current form, its reasons, the alternatives not
taken, and its history; the task document that makes or changes the decision
records it in its `…で決めたこと` section as well and links to the ADR
([adr.md](adr.md), and `CLAUDE.md`, "Design decisions (ADR)").
The `Process` row of `01_requirements.md` is a value of the status table, like
`Status`, not a seventh place: the reason for the chosen process, the
exclusion judgment, and a human's override of that judgment are decisions, and
they go in the `要件定義の際に決めたこと（YYYY-MM-DD）` section of
`01_requirements.md` ("Process record" below).
A **status transition** is one of four events:
**approval** (`/runpr` sets the document to `approved`, the first time or after
a return to `draft`), **`draft` revert** (a decision change returns an approved
document to `draft`), **session re-approval** (`/runplan`'s reviewer approves a
decision change in the session and the document stays `approved`), and
**withdrawal** (the document is set to `withdrawn`). The revisions of a newly
created `draft` during its own review are not a status transition.

#### Process record

The **process record** is the `Process` row of the status table of
`01_requirements.md`. It records which process the task takes: the full process
or the simple process ("Review Flow" above). `02_architecture.md` and
`03_implementation_plan.md` have no such row.

```markdown
| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | YYYY-MM-DD |
| Review date | - |
| Reviewer | - |
| Comments | - |
```

- **Values:** `` `simple` `` (the simple process) and `` `full` `` (the full
  process), backquotes included, and no other. The value is the row's cell text
  ("Cell text" in "Change history" below). The row is written right after
  `Status`, but a reader does not depend on its position.
- **States of the record:**

  | Record | Meaning | How a command treats the task |
  |---|---|---|
  | No `Process` row | A `01_requirements.md` written before the simple process existed | The full process |
  | `` `full` `` | The full process | The full process |
  | `` `simple` `` | The simple process | The simple-process branches of the commands |
  | Two or more `Process` rows, a cell text that is neither value, or a status table that cannot be read | A broken record | Stop and report; never guess either process |

- **Who writes it:** `/mkreq` writes the row when it writes
  `01_requirements.md`, in either process, with the value the human chose.
  `/runpr` rewrites it to `` `full` `` when it returns a three-document PR to the
  full process ("Simple Process" below). No other command changes it, and it is
  not edited by hand. Once `01_requirements.md` is on `main`, the record does
  not change: `/runpr` stops on a PR whose diff adds, removes, or changes the
  `Process` row of a `01_requirements.md` that `origin/main` has.
- **Where the reasons go:** the reason for the chosen process, the judgment of
  whether the change is excluded ("Simple Process" below) with its reason, a
  human's override of that judgment, and a return to the full process with its
  reason are written in the `要件定義の際に決めたこと（YYYY-MM-DD）` section of
  the overview chapter of `01_requirements.md`.

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
not the change history; the definition of a document without a change history
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
  | Approval copied from the status table (written only by the migration) | `- <Review date のセルの文> 承認（<Reviewer のセルの文>、状態表から写した）` |

  For example (an illustration of each form, not one document's sequence):

  ```markdown
  ## 変更履歴

  - 旧規則の Comments：<移した旧 Comments の文、内容を変えずに>
  - 2026-10-20 承認（isseis、状態表から写した）
  - 2026-11-01 承認（isseis、#606）
  - 2026-11-03 draft への戻し：<変更の要旨>（#610）
  - 2026-11-04 draft への戻し：02_architecture.md の決定の変更に伴う（#610）
  - 2026-11-05 セッション中の再承認：<変更の要旨>（isseis、#612）
  - 2026-11-07 withdrawn：<理由、または理由を記録した文書へのリンク>（#615）
  ```

- **Cell text:** the status table is the first table outside a code block
  after the `## Document Status` heading. The cell text of one of its rows is
  the bytes of the line after its leading `| <Item> | ` (for example
  `| Comments | ` or `| Reviewer | `) and before its trailing ` |`; do not
  split it on `|`. A row has a cell text only when its line starts with that
  leading text and ends with ` |`, and only when it is the one row of its item
  in the table. Wherever this rule copies a cell text into the change
  history, copy it byte for byte, backquotes, `\|`, and `<br>` included, and
  do not correct a value that looks mistaken.
- **Approver:** the GitHub user name, the value of `gh api user --jq .login`,
  in both the approval line and the session re-approval line. The copied
  approval line below is the exception: it carries the cell text of
  `Reviewer` as it is.
- **PR number:** every line that reaches `main` carries the number of the PR
  that added it; the migrated item and the copied approval line below are the
  only two exceptions. A line written after its PR exists (by `/fixpr` or
  `/runpr`) writes the number. A line written before its PR exists (for example
  by `/runplan`, `/mkarch`, `/mkplan2`, or a manual withdrawal) writes the
  not-yet-filled marker `#未記入` in place of `#<PR 番号>`. `/runpr`'s PRE_MERGE
  replaces the marker in each change-history line the PR adds, to a new or an
  existing task document, and only in those (`.claude/commands/runpr.md`,
  PRE_MERGE); a marker already on `main` is shown to the user, never filled in
  automatically, because the PR that added it is not known for certain. To fill
  one in, find the commit that added the line (`git log -S '#未記入' --
  <document>`) and its PR (`gh pr list --state merged --search <SHA>`), and fix
  the marker in a PR of its own.
- **`draft` revert along with an earlier document's decision change:** only
  `/fixpr` writes this line, when a changed handoff item returns a later
  document whose body it did not change (`.claude/commands/fixpr.md`, R3). On
  a three-document PR whose documents are approved, a decision change in one
  of them returns the other two as well, an earlier one included, each with
  this line naming the changed document ("Simple Process" below).
- **Migrated item:** the old `Comments` of a document without a change history,
  when it is not `-`, becomes the first item, starting with the prefix
  `旧規則の Comments：`. It has no date, transition, or PR number, and it is the
  only exception to one line per transition.
- **Copied approval line:** the line that keeps the approval recorded in the
  status table of an `approved` document without a change history. Only the
  migration ("Documents without a change history" below) writes it, from the
  cell texts of `Review date` and `Reviewer`. It comes right after the
  migrated item when there is one, and is the first item otherwise; this item
  alone defines its position. Its marker `状態表から写した` stands in place of `#<PR 番号>`,
  because the status table does not tell which PR recorded the approval: it
  carries neither a PR number nor `#未記入`, and since the marker has no `#`,
  `/runpr`'s PR-number fill-in leaves it alone. It records one approval in one
  line, so it follows one line per transition.

#### Documents without a change history

A **document without a change history** is a task document that has no change
history as "Change history" above defines it. A `## 変更履歴` line inside a code
block does not count, including a code block indented inside a list item, so
checking only whether some line of the document equals `## 変更履歴` is not enough:
a code block can hold that line at the start of a line. Whether its `Comments`
is `-` does not matter. A document written before the change history existed
keeps corrections and past approvals in `Comments` and its latest approval in
the status table; it stays valid as it is.

**The status table before the transition** is the status table of the document
at `HEAD` (`git show HEAD:<path>`), never the one in the working tree. The
migration is committed together with the change history it creates, so `HEAD`
holds the values before the transition, while the working tree's table may
already have been rewritten (for example, while a chained revert or a second
decision change is handled). The status before the transition is `approved`
when the cell text of `Status` equals `` `approved` ``, backquotes included. A
document that is not at `HEAD` is treated as not `approved`.

The first time a status transition is written to a document without a change
history under this rule, whoever writes it, a command or a person, does the
following, in order:

1. **Check that the status table can be copied.** Check that the status
   table before the transition gives a cell text ("Cell text" in "Change
   history" above) for `Status` and for `Comments`, and that the `Comments`
   cell text is not empty, not only whitespace (U+0020, tab, or U+3000), and,
   when it is not `-`, does not contain `#未記入`. When the status before the
   transition is `approved`, also check that it gives a cell text for
   `Review date` and for `Reviewer`, and that each is not empty, not only
   whitespace, not `-`, and does not contain `#未記入`. A value with the marker
   could be rewritten by `/runpr`'s PR-number fill-in once it is in a
   change-history line. When any of these fails, stop ("Stopping" below). A
   document that is not at `HEAD` has no status table before the transition:
   it skips this step and steps 3 and 4.
2. Create `## 変更履歴` at the end of the document.
3. When the `Comments` cell text before the transition is not `-`, put it in as
   the migrated item ("Change history" above). When it is `-`, there is no
   migrated item.
4. When the status before the transition is `approved`, put in the copied
   approval line ("Change history" above, which also gives its position). When
   it is `draft` or `withdrawn`, there is no copied approval line.
5. Rewrite `Comments` to the reason for the status after the transition
   ("Comments" above).
6. Append the transition's line.
7. Before committing, run the step 7 check below, after the last edit of the
   document, and commit the document as checked, without editing it again
   (stage it after the check, so the index holds the checked version). When the
   check fails, stop ("Stopping" below).

Whoever writes a status transition to a document without a change history runs
the step 7 check, as a step of this rule, even when an earlier step was skipped
by mistake; a command definition may say which of its participants runs it.

**The step 7 check.** It checks mechanically that the change history starts
with exactly the items the status table before the transition gives. An LLM can
"correct" a value it copies (a `Reviewer` of `isesis` into `isseis`), and an old
`Comments` can be several kilobytes long, which an LLM can change while copying.

- **Input:** the document at `HEAD` (`git show HEAD:<path>`) and the document in
  the working tree that will be committed. Do not use the output of
  `git diff`: a change staged in the index is missing from it, and the check
  would pass on nothing. Take the texts into files and compare them with
  commands, without reading them into the conversation.
- **Documents checked:** among the task documents the change touches (the files
  under `docs/tasks/` that differ between `HEAD` and the working tree and whose
  version at `HEAD` has a status table), every one that has no change history at
  `HEAD` and, in the working tree, has a change history or has a status-table
  row whose cell text differs from `HEAD`'s. When a cell that can be taken at
  `HEAD` cannot be taken from the working tree's status table (the table or the
  row is missing, or the row appears twice or more), it counts as different. So a transition written to the
  status table without a change history is checked, and fails below. A document
  that is not at `HEAD` is not checked: the first transition of a document
  created in this change is written by a later commit, which has it at `HEAD`.
- **Expected head:** the items steps 3 and 4 put in, built from the status
  table at `HEAD` in the forms and the order "Change history" above gives: the
  migrated item when the cell text of `Comments` is not `-`, and the copied
  approval line when the status is `approved`. When neither applies, the
  expected head is empty. When step 1's conditions do not hold, the check
  fails.
- **Comparison:** when the working tree's document has no change history, the
  check fails. Otherwise take the item lines of its change history (the lines
  of the section that start with `- `). Their first lines, as many as the
  expected head has, must equal the expected head byte for byte (compare the
  two files with `cmp`), and no later item line may have the form of a
  migrated item (its prefix) or of a copied approval line (its marker and the
  closing `）` at the end of the line). Only the change history is compared,
  so a line of the body that quotes this rule does not count.

**A second transition in the same change.** When the document in the working
tree already has a change history, do not migrate it again (for example, when
`/fixpr`'s chained revert and a body change reach the same document in one
round).

**Stopping.** When step 1 stops, do not write the transition. On either stop,
commit nothing of the change that contains the transition: neither the body
rewrite of the decision change nor the other documents returned to `draft` along
with it. Leave the changes in the working tree, and report to the user:

- the document's path and the names of the rows step 1 found unusable
  (`Status`, `Comments`, `Review date`, or `Reviewer`), or, when the step 7
  check failed, the part that failed;
- what was left: the uncommitted changes in the working tree, and the commits
  already on the branch;
- that the user decides how to fix the status table, by the kinds of edit in
  "Editing an approved document" above.

What to do next is the user's decision. A command definition says only what
stopping means for that command: what it does not commit and what it leaves.

**An editorial correction** alone does not migrate the document: it changes
neither its `Comments` nor its change history.

**A newly created `draft`** has no change history and its `Comments` is `-`, so
at its first approval by `/runpr` steps 3 and 4 put in nothing, and the change
history is the approval line alone, as "When it is created" in "Change history"
above says.

#### Recording in the PR description

- **Correction line (commit message):** one line per correction, in English:

  ```text
  Editorial correction: <document path>: <what was changed to what>; no decision changed
  ```

  Only a line that starts with `Editorial correction: ` at its first column is a
  correction line; an indented line or a mention inside a sentence is not. Do
  not start any other line of a commit message with `Editorial correction: `.
  `<document path>` is the repository-relative path of the task document or of
  the ADR file (`docs/dev/conventions/adr.md`, "Location and naming"). When
  rewriting an ADR's title changes its row in the index,
  `docs/dev/adr/README.md`, that ADR's correction line describes the index
  change too; no correction line names the index. Which changes to an ADR are
  editorial corrections, and when one may be made, is in
  `docs/dev/conventions/adr.md`, "Editorial corrections". The
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

  `<書いた手順>` is `PR の作成`, `/fixpr`, or `通常の進め方へ戻す`. Each item is a correction line put
  into Japanese, one item for each correction line. The paragraph carries every
  correction line of the range, whichever document it names. When a PR is created, the paragraph is
  written from the correction lines of `git log origin/main..HEAD`; after that,
  `/fixpr` appends a paragraph for the correction lines of the commits of its
  round. A paragraph already in the description is kept as it is, with one
  exception: when `/runpr` returns a three-document PR to the full process
  ("Simple Process" below), it rewrites the description and writes the
  corrections as one paragraph, built from the correction lines of
  `git log origin/main..HEAD` with `通常の進め方へ戻す` as `<書いた手順>`, which
  replaces every paragraph already there (the correction lines stay in the
  branch's commits, so nothing is lost). When the
  range has no correction line, no paragraph is written.
- **Decision changes (PR description):** when a PR is created, its description
  names the decision changes that the `draft` revert lines and the session
  re-approval lines the branch added to change histories show
  (`git diff origin/main...HEAD`).

#### Withdrawing a task

When a task is withdrawn after review (for example, a feasibility check shows the requirements cannot be met), set the status of every task document that exists to `withdrawn`, whatever its previous status. Set `Comments` to the date and the reason, or a link to the document that records it ("Comments" above), and append a withdrawal line to the change history ("Change history" above; `#未記入` when the line is written before its PR exists), migrating a document without a change history first. A `withdrawn` document is not `approved`, so the commands that require an approved document (`/mkarch`, `/mkplan`, `/runplan`) stop on it. When the task created or changed an ADR, fix the ADR in the withdrawal PR as [adr.md](adr.md), "Changing, retiring, and withdrawing", says.

### Simple Process

The simple process writes the three task documents of a small change, each kept
as short as the change allows, in one PR: the **three-document PR**. `/mkreq`
confirms the requirements, design, and plan decisions with the human in one
run, writes the three documents as `draft`, and opens the PR; the reviewer
approves all three in one answer to `/runpr`'s merge question, and `/runplan`
implements the plan in later PRs ("Review Flow" above). It is an exception to
"LLM Constraints" above, and to the required sections and Mermaid diagrams of
sections 1 to 3 below as far as "May be omitted" says. It applies only to a task whose process record is
`` `simple` `` ("Process record" above): a task whose `01_requirements.md` was
written before the simple process existed takes the full process. The
command-side procedures are in `.claude/commands/_lib/simple-process.md`.

**Choosing the process.** Every time `/mkreq` writes a `01_requirements.md`, it
asks the human which process to take, after showing the process it recommends
with the reason and its judgment of whether the change is excluded (below) with
the reason. The human decides; there is no limit on lines or files and no
script that judges it.

**Never omitted.** The simple process keeps:

- the acceptance criteria of `01_requirements.md`, each with an identifier
  unique in the document;
- the verification of each acceptance criterion in `03_implementation_plan.md`
  and its traceability ("AC traceability" below);
- the ADR judgment of each decision in the three documents, and the ADR that a
  major decision creates or updates (`CLAUDE.md`, "Design decisions (ADR)");
- the judgment of whether `docs/dev/security.md` applies; when it does not, the
  security section says N/A with the reason;
- the PR boundaries of `03_implementation_plan.md`, as `/mkplan` writes them,
  with no limit on the number of implementation PRs;
- the steps of the Japanese prose pass (`japrose`) over the three documents
  and the ADR changes, and of the local CLI review, before the PR is opened.

**May be omitted.** A document of the simple process may omit:

- the body of a required section that does not apply: the heading stays, with
  N/A and the reason in one line;
- the Mermaid diagrams of `02_architecture.md`, which are written when they are
  judged necessary;
- the review panel: one reviewer may self-review the three documents, unless
  the agent judges the risk high, in which case each document gets the review
  its own command gives it.

**Excluded changes.** A change that does any of the following is excluded:
it cannot take the simple process unless the human overrides the judgment.

- It changes a rule of `docs/dev/security.md` or its premise (the threat model,
  [ADR-0006](../adr/0006-threat-model.md)).
- It changes a trust boundary of the development tools
  ([ADR-0015](../adr/0015-untrusted-review-threads.md)): a change to
  `fixprguard`, `codexreview`, or `mergepr`, and also a change that leaves them
  as they are but affects the boundary, such as how the text of a review thread
  or a request to a Bot is handled. The judgment is whether the boundary is
  affected, not whether a named tool changes.
- It adds or changes an external dependency: a Go module, an npm package of the
  extension, the Go toolchain, an external development tool such as
  `golangci-lint`, a CI action, or an external service, added, upgraded, or
  replaced.

A change that only brings the description of a security document in line with
what the code does now, without changing a rule or a premise, is not excluded.
The agent and the human judge exclusion by reading the change, with no limit on
lines or files and no script. When the agent is in doubt, it judges the change
excluded and recommends the full process. The human may override the judgment
and choose the simple process; `01_requirements.md` records the override
("Process record" above).

When a decision found later needs an excluded change:

- while `/mkreq` confirms the design or plan decisions, it shows this with the
  reason and asks whether to switch to the full process; the human may go on,
  as with an override, which is recorded;
- while the three-document PR is under review, `/fixpr` does not make that
  fix; the human chooses in `/runpr`'s merge question to override the judgment
  and have it fixed, which is recorded, or to return to the full process;
- after the three-document PR is merged, `/runplan` stops, and so does `/fixpr`
  on an implementation PR: the change becomes a separate task in the full
  process.

**Returning to the full process.** Before the three-document PR is merged, the
reviewer can return it to the full process, only by answering `/runpr`'s merge
question or by telling `/runpr` to; a review thread asking for it never
returns it, and no other command changes the process record. After the return:

- the PR holds `01_requirements.md` alone, without `02_architecture.md` and
  `03_implementation_plan.md`;
- the process record is `` `full` ``, and `01_requirements.md` records the
  return and its reason;
- an ADR change made only for a design or plan decision leaves the PR, and an
  ADR changed for both a requirements decision and a design or plan decision
  keeps only the requirements part;
- the design and plan decisions the human confirmed stay as confirmed items of
  the handoff files ("Design handoff" below), which `/mkarch` and `/mkplan`
  read; the addressed items become ordinary items;
- the concerns of the review threads left unresolved on the deleted documents
  become ordinary items of the handoff files, in `/runpr`'s own words;
- `/runpr` asks its merge question again and can merge it as a requirements
  PR, after which `/mkarch` comes next.

After the three-document PR is merged, the task never returns to the full
process. A later decision change follows "Editing an approved document" above,
as in the full process; one that needs an excluded change is handled as the
list above says.

---

## 1. Requirements Document (`docs/tasks/XXXX_feature/01_requirements.md`)

**Mandatory for each functional requirement:**
- Define the requirement clearly (what, why, how)
- **Add explicit acceptance criteria** in a dedicated section
- Each acceptance criterion must be:
  - Specific and measurable
  - Independently verifiable
  - Focused on behavior, not implementation
- **Write an AC that states a relation between the state before and after the change as a limited diff.** Such an AC (something is preserved, unchanged, or still present in the same order) takes the form "the diff of X is limited to Y and Z", and does not enumerate the parts that must be kept: each part listed becomes a piece of checking code (§ 4, "`diff` verification"). The one exception is an AC for a move or a split, which may take the form "the content of A is in B unchanged", because a move or a split cannot be stated as a limited diff.
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

**Design handoff:** when review surfaces concerns that belong to a later document (mechanism choices, library pitfalls, test construction), record them in a handoff file instead of expanding the reviewed document, routed as `.claude/commands/fixpr.md` R2 says: design- and architecture-level concerns raised on `01_requirements.md` go to `docs/tasks/XXXX_feature/design_handoff.md`, whose items `02_architecture.md` records, each with the approach taken or why it does not apply; implementation-level concerns raised on `01_requirements.md` or `02_architecture.md` go to `implementation_handoff.md`, whose items `03_implementation_plan.md` records in the same way.

A handoff item (`H-NN` in `design_handoff.md`, `I-NN` in `implementation_handoff.md`) is a `## H-NN: <要旨>` or `## I-NN: <要旨>` section in Japanese, of one of three kinds:

- **Ordinary item:** a concern raised in review, in the form `/fixpr` writes it (`.claude/commands/fixpr.md`, R5): what the concern is, why it matters, a candidate approach, and the related F-/AC- IDs. The consuming document records the approach taken or why it does not apply.
- **Addressed item:** in a three-document PR ("Simple Process" above), a concern raised on an earlier document of the PR that a later document of the same PR handles. It is an ordinary item with one more line at its end, naming where the later document handles it:

  ```markdown
  **扱った箇所:** <文書のファイル名> の <節の番号と見出し>
  ```

  The later document needs no further record for it. When the PR returns to the full process, the line is removed and the item becomes an ordinary item.
- **Confirmed item:** a design or plan decision the human confirmed in a `/mkreq` run of the simple process (an item of the `設計の際に決めたこと` or `計画の際に決めたこと` section, which records the decisions `_lib/settle-decisions.md` asked the human about), moved to `design_handoff.md` (a design decision) or `implementation_handoff.md` (a plan decision) when the task returns or switches to the full process. Its heading ends with `（確かめ済み）`, and its body has these lines (`## I-NN` in `implementation_handoff.md`):

  ```markdown
  ## H-NN: <決定の要旨>（確かめ済み）

  **決定:** <決めたこと>
  **理由:** <理由>
  **決めた日:** YYYY-MM-DD
  **人の答え:** <人が選んだ選択肢、または答えの要旨>
  **ADR:** <ADR の判断（なし、または作るか更新する ADR とその理由）>
  ```

  The human has already confirmed it: `/mkarch` or `/mkplan` adopts the decision and its ADR judgment without asking again, and records it in its `…で決めたこと` section. It asks again only when `01_requirements.md` changed after the item was written and the item's premise no longer holds.

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
- A `diff` verification is for an AC that states a relation between the state before and after the change; it is a script or a recorded diff reading, as "`diff` verification" below says
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

### `diff` verification

- **What it verifies.** An AC that states a relation between the state before and after the change: something is preserved, unchanged, or still present in the same order. Section 1 gives the form such an AC takes, for example "the diff of section S is limited to rewriting the one line of item X". The property belongs to the change, not to the resulting state, so it is not a permanent test: a permanent test would have to pin a copy of the old values, which `CLAUDE.md` forbids ("Document tests check values, structure, and executable examples, never prose"), or read git history, which makes it depend on the checkout and blocks later changes once the "before" no longer means anything.
- **Two forms.** A `diff` verification is either a **script** or a **recorded diff reading**, and both carry the `diff` label. Both compare the merge-base with a commit of the PR (the **compared commit**), are done once per PR on the PR branch, and leave their result with both commit IDs in the implementation plan's verification row. `make test` and CI run neither, and neither is maintained after the merge.
- **Proportion.** Build the verification in proportion to the diff it checks: weigh what the reviewer has to read (the script, or the recorded output) against the size of the diff it verifies.
- **Script.** A script is committed under the task directory (`docs/tasks/NNNN_<name>/_checks/`; `go list ./...` ignores a directory whose name starts with `_`). Its default language is shell (`git diff`, `cmp`, `sed`, and the like); another language is used only when the plan states why shell is not enough. Whoever writes it confirms that each of its checks fails under at least one breakage, runs it on the PR branch, and records its output and both commit IDs in the plan's verification row.
- **Recorded diff reading.** For a document change whose diff is small, the PR review may read the recorded diff instead of a script. Whether the diff is small is the plan author's judgment, with no numeric threshold, and the reviewer may object to it ("Objection" below). The plan states that the AC uses a recorded diff reading and names the compared paths. It needs no breakage step: there is no code to break.
- **Compared paths.** The compared paths may be limited to the paths the AC names. They never include the implementation plan, which holds the record: each recording would change the diff it records. An AC about the plan itself is verified by a script. For an AC that limits which files change, the record also holds the file list of the whole diff, so a change outside the compared paths is not missed.
- **Recording.** `/runplan` runs the fixed commands below after the commit that implements the AC and writes the diff record, in the form below, into the plan's verification row in a later commit. Committing the record advances the PR's head; the record stays valid as long as the compared paths have the same content at the final head.
- **Recheck.** The record is rechecked before the PR is created (`/runplan`) and before the merge (`/runpr`, PRE_MERGE), by "Rechecking a record" below. The recheck runs nothing but `git fetch origin main`, the fixed commands, and the comparison: it never evaluates a string from the record in the shell, and it never runs a `_checks/` script.
- **Output too long to read.** Output too long to be read whole means the diff is not small. Found while planning, the plan uses a script; found while recording, `/runplan` records nothing and settles the change of verification with the human as a decision change ("Editing an approved document"); found in the review, the reviewer objects. The output is never truncated.
- **Objection.** When the reviewer objects that a diff recorded for reading is not small, the AC's verification is replaced by a script before the merge. Changing the form of a verification changes what the approved plan decides, so it is a decision change ("Editing an approved document"): the plan returns to `draft` and is approved again at the merge. Whoever adds the script runs and records it as "Script" above says; `/runpr` stops at the merge while the record is missing.

#### Fixed commands

The record holds no command: both commands are fixed here, so rerunning one gives the same output whatever the git configuration, except for the one part named after the commands. Their arguments fix color, the path prefixes, the number of context lines, the merging of nearby hunks, the diff algorithm, the abbreviation of blob IDs, the order of files, the quoting of path names, rename detection, external diff and text conversion programs, a binary marking (the output is always text), the relative-path setting, and the form of an empty context line; `env -u GIT_DIFF_OPTS` removes the environment variable that would override the number of context lines. Rename detection is off, so a moved file shows as the deletion of its old path and the addition of its new path. Run them from the top level of the work tree, with `<base>` the merge-base (`git merge-base origin/main HEAD`, after `git fetch origin main`) and `<commit>` the compared commit, both as full commit IDs.

Path diff, with each compared path passed literally as its own single-quoted argument after `--` (a path that holds a single quote or a newline cannot be passed this way: stop and report it):

```bash
env -u GIT_DIFF_OPTS git --no-pager --literal-pathspecs -c core.quotePath=true -c diff.suppressBlankEmpty=false \
  diff --no-color --no-ext-diff --no-textconv --text --default-prefix --no-relative --full-index \
  -U3 --inter-hunk-context=0 --diff-algorithm=myers --indent-heuristic --no-renames -O/dev/null \
  <base> <commit> -- '<path>' ...
```

File list, only for an AC that limits which files change:

```bash
env -u GIT_DIFF_OPTS git --no-pager -c core.quotePath=true \
  diff --no-ext-diff --no-relative --no-renames -O/dev/null --name-only <base> <commit>
```

One part of the output stays outside these arguments: the function name after the `@@` of a hunk header comes from the `diff` attribute (`.gitattributes`, `$GIT_DIR/info/attributes`, `core.attributesFile`) and the `diff.<driver>.funcname` or `diff.<driver>.xfuncname` setting of the driver it selects, which no `git diff` argument fixes. The record is made and rechecked in the same repository, so the attributes are normally the same; a recheck in a checkout whose attributes differ can see a difference in hunk headers alone, and it is treated like any other difference (re-recorded).

#### Diff record form

The diff records follow the verification table of the plan's Acceptance Criteria Verification section, one block per AC (and, when several PRs verify one AC row, one per PR, under a line naming the PR). The plan author writes the first line and the compared paths; `/runplan` writes the rest:

`````markdown
Diff record (AC-NN):

- Compared paths: `docs/dev/security.md`
- Merge-base: `<full commit ID>`
- Compared commit: `<full commit ID>`

Path diff:

````text
<the whole output of the path diff command>
````

File list (only for an AC that limits which files change):

````text
<the whole output of the file list command>
````
`````

- The compared paths are listed one per code span. The plan author names them; `/runplan` does not change them.
- The label line before a code block (`Path diff:`, or the line starting `File list`) says which command's output the block holds.
- Each output is the command's whole output, never truncated, in a code block that starts at the beginning of the line. Its fence is a run of backquotes longer than any run of backquotes in the output, and at least three, because a document's diff can hold a fence of its own.
- Nothing inside a record's code block is read as a plan marker or a checkbox (a PR checkpoint marker, `- [ ]`, the not-yet-filled PR number marker, and the like): a command that reads the plan's markers skips those code blocks.

#### Rechecking a record

`/runplan` (before it creates the PR) and `/runpr` (PRE_MERGE) recheck each diff record the same way, and compare mechanically, never by eye:

1. Run `git fetch origin main`, then compute `<base>` as `git merge-base origin/main HEAD`, with `HEAD` as `<commit>`.
2. Run the path diff command with the record's compared paths, and the file list command when the record has a file list, writing each output to a file outside the work tree.
3. Copy the lines strictly between the opening and the closing fence of each of the record's code blocks, unedited, to another file outside the work tree: `awk -v a=<opening fence line> -v b=<closing fence line> 'NR > a && NR < b' <plan>`. An empty code block gives an empty file.
4. Drop the trailing whitespace of every line of both files (`sed 's/[[:space:]]*$//'`) and compare them with `cmp`. The repository's pre-commit hook removes trailing whitespace from the plan, so a committed record can differ from the command's output there alone.
5. When every comparison exits `0`, the record stays valid. When one exits `1`, re-record: replace the outputs and both commit IDs with the new ones. When a command or a comparison fails (any other exit status), or the record lacks a compared path or a commit ID, stop and report: an output that cannot be compared cannot be told from an unchanged one.

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
