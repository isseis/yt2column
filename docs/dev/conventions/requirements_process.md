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
- **Exception: the lightweight process.** A change that meets every condition of
  "Lightweight Process" below, and whose spec comment the user approved in
  `/mkreq`, is implemented without `01_requirements.md`, `02_architecture.md`,
  and `03_implementation_plan.md`: the approved spec comment takes the place of
  the approved documents. The exception reaches no further. A change that
  misses any condition, or an issue without an approved spec comment, takes the
  normal process and every rule above.

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

A change that meets the conditions of "Lightweight Process" below takes a
shorter flow, with no task document:

```
/mkreq <issue> judges that the issue meets the lightweight conditions
  → The user approves the spec comment shown in full (AskUserQuestion)
  → /mkreq posts the spec comment and adds the lightweight label, then stops
  → /runissue <issue> implements the spec on the lightweight branch, checks
    and reviews it, rechecks the conditions on the real diff, and opens one PR
    → Human review (PR review)
    → /runpr merges the PR and closes the issue
```

When the issue does not meet the conditions, when the judgment is in doubt, or
when the user does not choose the lightweight process, `/mkreq` takes the normal
flow above. When `/runissue` finds that the change misses a condition, or that
it needs a decision the spec does not make, it returns the issue to the normal
process: it pushes no implementation commit, posts a revert comment, and
removes the label, and the user runs `/mkreq <issue>` again.

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
A **status transition** is one of four events:
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
  document whose body it did not change (`.claude/commands/fixpr.md`, R3).
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

When a task is withdrawn after review (for example, a feasibility check shows the requirements cannot be met), set the status of every task document that exists to `withdrawn`, whatever its previous status. Set `Comments` to the date and the reason, or a link to the document that records it ("Comments" above), and append a withdrawal line to the change history ("Change history" above; `#未記入` when the line is written before its PR exists), migrating a document without a change history first. A `withdrawn` document is not `approved`, so the commands that require an approved document (`/mkarch`, `/mkplan`, `/runplan`) stop on it. When the task created or changed an ADR, fix the ADR in the withdrawal PR as [adr.md](adr.md), "Changing, retiring, and withdrawing", says.

### Lightweight Process

The lightweight process takes a simple change from an issue to one PR without
task documents. The user approves a spec comment on the issue instead of the
three documents, `/runissue` implements it and opens the PR, and `/runpr` merges
it ("Review Flow" above). It is an exception to "LLM Constraints" above, and its
scope is exactly a change that meets every condition of this section and whose
spec comment the user approved. Anything else takes the normal process.

#### Types

A lightweight change is of one of three types:

- **W (wording):** a fix of a typo, an inconsistent spelling, or a phrasing in
  a comment or a document that does not change its meaning. It is checked by
  reading the diff. A README change is also checked with `/mktrans` and the
  existing document tests.
- **D (deletion):** the deletion of code a tool showed to be unreachable
  (`make deadcode`, `tsc`, `git grep`), and of the tests that check only that
  code. It is checked with `make test` and `make lint`, and with
  `make ext-check` when it changes `extension/`. A test deletion is also
  compared as `CLAUDE.md`, "Deleting a test is a claim that must be checked",
  says.
- **M (mechanical refactoring):** a change in which any mistake is certain to
  be caught by the compiler, the type checker, a linter, or the existing tests.
  Examples are renaming an identifier referenced only from code, applying
  `golangci-lint run --fix`, and moving code or files. It is checked as D is,
  and by running the tool again.

A change qualifies only when it fits exactly one type and meets that type's
limits and the overall limits below. A change that spans two or more types does
not qualify.

#### Limits

Added and deleted lines are counted separately, as the lines of the diff:

- **W:** at most 30 added lines and at most 30 deleted lines.
- **D:** at most 100 deleted lines and at most 10 added lines.
- **M:** at most 50 hand-written added lines; deleted lines are not limited.
  Lines a tool generated are left out of this limit only when the command that
  generated them is recorded and running it again on the state before the
  change gives the same diff.

Overall, the change is one PR, and it changes at most 10 files by hand. A file
whose only changes are tool-generated changes under the M exception is not
counted as a file changed by hand. A change to `README.en.md` that follows a
change to `README.md` counts toward both the lines and the files.

#### Counting lines and files

- **Range:** the diff of the lightweight branch from the commit where it forked
  from `origin/main` (BASE, `git merge-base origin/main HEAD`) to its last
  commit. Hand-written lines and files are counted from the last tool commit
  (BASE when there is none) to the last commit.
- **Tool commits (type M only):** the tool-generated changes are committed
  first on the branch, one commit per tool command, before any hand-written
  commit. Each records its command in one `Lightweight-Generated-By: <command>`
  trailer line. The command must match a `tool` row of the conditions table
  below word for word, and its remaining words must be paths inside the
  repository.
- **Lines:** added and deleted lines are summed separately. A line whose only
  change is its line ending or its whitespace is counted like any other changed
  line (one deleted and one added line). Renames are not detected:
  a rename counts as the deletion of the old file and the addition of the new
  one.
- **Files:** the number of changed paths. A rename counts as two files, the old
  path and the new one.
- **Changes that cannot be counted:** a binary file, a symbolic link, a
  submodule, or a mode change (including between `100644` and `100755`) makes
  the change miss the conditions.
- The counts must not depend on the user's git configuration (diff algorithm,
  external diff, textconv, color, ignored submodules, `diff.relative`, log
  signatures, the untracked-file display, hooks).

#### Changes that always take the normal process

Whatever their size, these changes take the normal process:

- A change of wording that changes a meaning. In `CLAUDE.md`,
  `.claude/commands/`, and `docs/dev/conventions/`, only a typo fix qualifies;
  any other change takes the normal process.
- Renaming something referenced by a string: for example a JSON tag, a protocol
  member, a storage key, a message catalog key, a `chrome.*` name, a literal of
  a document test, or an identifier a document refers to by name.
- A change to user-visible wording beyond a typo fix.
- A change within the scope of `docs/dev/security.md` (one that meets a
  condition under which that guide applies).
- A change that touches a trust boundary: `fixprguard`, `codexreview`,
  `mergepr`, secrets, the invocation of `yt-dlp`, and the cache.
- Adding a dependency.
- A change that makes an exception to a policy.
- A change that leaves a design choice open.
- An edit of a task document (anything under `docs/tasks/`).
- A change of behavior, a bug fix included.

#### Mechanically checked conditions

```text
limit W added 30
limit W deleted 30
limit D added 10
limit D deleted 100
limit M added 50
limit M deleted none
limit files 10
exclude docs/tasks/
exclude docs/dev/security.md
exclude cmd/fixprguard/
exclude internal/fixprguard/
exclude cmd/codexreview/
exclude internal/codexreview/
exclude cmd/mergepr/
exclude internal/mergepr/
exclude internal/secret/
exclude internal/redact/
exclude internal/config/
exclude internal/transcript/
exclude internal/cmdrunner/
exclude internal/cachelock/
exclude internal/articlecache/
exclude cmd/yt2column-nmhost/
exclude internal/nmhost/
exclude internal/serversock/
exclude extension/static/manifest.json
exclude testdata/
exclude prompts/
exclude go.mod
exclude-absent go.sum
exclude-absent go.work
exclude extension/package.json
exclude extension/package-lock.json
exclude extension/.npmrc
exclude .golangci.yml
exclude-absent .gitmodules
exclude .github/workflows/
exclude .claude/settings.json
exclude .pre-commit-config.yaml
exclude .coderabbit.yaml
exclude .gitattributes
exclude Makefile
exclude AGENTS.md
exclude .github/copilot-instructions.md
exclude .github/instructions/
exclude docs/dev/conventions/requirements_process.md
exclude .claude/commands/_context.md
exclude .claude/commands/_lib/lightweight-spec.md
exclude .claude/commands/_lib/lightweight-check.sh
exclude .claude/commands/_lib/task-pr-handoff.md
exclude cmd/coveragecompare/
exclude internal/coveragecompare/
exclude cmd/yt2column/codex_review_docs_test.go
exclude cmd/yt2column/fixpr_docs_test.go
exclude cmd/yt2column/lightweight_docs_test.go
exclude internal/doctestutil/
tool gofumpt -w
tool git mv
```

The block above is the conditions table: the conditions checked mechanically on
the real diff. The recheck script reads it from `origin/main`, never from the
branch it checks, so a branch cannot loosen it. Each line is one condition:

- `limit <type> added|deleted <number | none>` is the limit on hand-written
  lines. Each of W, D, and M has exactly one `added` line and one `deleted`
  line; `none` means no limit.
- `limit files <number>` is the limit on files changed by hand, on exactly one
  line.
- `exclude <path>` is a path no lightweight change may touch, and it must exist
  in the repository. A path ending in `/` is a directory; any other path is a
  file. Paths are written from the repository root and matched as a whole path
  or as a directory above it, never as a string prefix (`go.mod.bak` does not
  match `go.mod`). A rename touches both its old and its new path.
- `exclude-absent <path>` is a path that does not exist now and whose creation
  is itself excluded.
- `tool <word> [<word> ...]` is a command a tool commit may record. The
  recorded command matches when its leading words equal the row's words one
  for one (`gofumpt -w` does not match `gofumpt -wx`).
- Any other line, a missing line, or a repeated line makes the table invalid:
  the check stops rather than read it more widely.

A PR that adds a package or a file on a trust boundary adds its `exclude` line
to this table in the same PR.

The recheck script checks these conditions on the real diff: the line limits
of the type, the number of files changed by hand, that no excluded path
changed, that there is no change that cannot be counted ("Counting lines and
files" above), and that each tool commit is reproduced by its recorded command.

The remaining conditions are checked by the subagent review of `/runissue`:
that the change fits exactly one type; that it is none of the changes above
that a path does not decide (a wording that changes a meaning, a name
referenced by a string, the scope of `docs/dev/security.md`, a trust boundary,
a policy exception, an open design choice, a change of behavior); and that it
stays inside the spec.

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
