# Architecture Decision Records (ADR)

An ADR records one subject of major decisions: the decision in effect now, why
it was made, the alternatives that were not taken, and how the decision changed
across tasks. ADRs are living documents. When a decision changes, the same ADR
is rewritten to the current decision and the change is added to its history; a
new ADR does not replace an old one. The task documents stay as the unchanging
record of what each task decided and why.

This document is the convention for writing, changing, and retiring ADRs. The
ADRs and their index are in Japanese (`CLAUDE.md`, "Document language",
principle (a)); this convention is in English because agents follow it
(principle (b)). `CLAUDE.md`, "Design decisions (ADR)", holds the rule that
makes a PR write an ADR, and refers here for how. The index is
[docs/dev/adr/README.md](../adr/README.md).

`TestADRRecords` (`cmd/yt2column/adr_test.go`) reads the definitions in the
"Template", "Status", and "Index" sections below and checks every ADR and the
index against them. Keep each of those sections to the one code block or the
one table it has, and keep their headings: the test looks them up by heading
and fails when it cannot read them.

## Location and naming

- **Location:** directly under `docs/dev/adr/`. The directory holds only the
  ADRs and the index, `README.md`; no other file and no subdirectory.
- **Name:** `NNNN-<kebab>.md`. `NNNN` is a four-digit number, and `<kebab>` is
  a name for the subject made of lowercase ASCII letters and digits joined by
  `-`.
- **Number:** a sequence of its own, unrelated to task numbers. It starts at
  `0001` and runs without gaps. A new ADR takes the number after the highest
  one on `main` and on the PR's branch at the time it is written. An ADR is
  never deleted, so a number is never reused, not even the number of an
  obsolete ADR.
- **Name in text:** a document calls an ADR `ADR-NNNN`.
- **Title:** the text after `ADR-NNNN: ` on the first line. It must not
  contain `|`, because the index puts it in a table cell.
- **Subject:** one ADR holds one subject, and one subject belongs to only one
  active ADR. Subjects that overlap, such as two decisions about the same
  screen, link to each other instead of being merged.

## Template

An ADR has exactly these `##` sections, in this order. Under them, `###`
subheadings may be added; no other `##` heading may be.

```markdown
# ADR-NNNN: <題名>

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

## 決定

## 採らなかった案

## 結果

## 経緯
```

## Status

The `状態` row of the table in the first section holds one of these values. The
table's other rows are used only by an obsolete ADR ("Changing, retiring, and
withdrawing").

| Value | Name | Meaning |
|---|---|---|
| `有効` | active | The decision is approved: the PR that holds the ADR was reviewed and merged. It does not say whether the implementation is finished. |
| `廃止` | obsolete | The subject has ended and the decision is no longer in effect. |

An ADR has no Document Status table and no change history of task-document
form: `/runpr` does not record its approval ("Approval and the PR").

## Index

The index, `docs/dev/adr/README.md`, is in Japanese. It has a short
explanation (that it shows which decisions are in effect now, and that this
convention says how to write an ADR) and one table with one row per ADR, in
number order, of this form. The first column links to the ADR with the text
`ADR-NNNN`, the second holds the ADR's title, and the third its status, as
they are in the ADR.

```markdown
| 番号 | 題名 | 状態 |
|---|---|---|
| [ADR-0001](0001-<kebab>.md) | <ADR の 1 行目の題名> | `有効` |
```

A new row goes at the end of the table. The index lists every ADR, obsolete
ones included; an index with no row is correct only while there is no ADR.

## What counts as a major decision

A decision is major when its effect reaches beyond the task that makes it: it
binds later tasks or more than one component, or reversing it would change
more than one place. Examples are the structure and the boundaries between
components, a protocol, an external dependency or service, a security
assumption, the product's scope, and a policy on the development process and
its tools. A decision that changes the decision of an existing ADR is always
major. A decision that closes inside one task — a test's name, a wording, the
shape of one function, the value of a constant, the order of implementation —
is not major: it stays in the task documents only.

The same criterion is in `CLAUDE.md`, "Design decisions (ADR)". When you change
it, change both in the same PR.

When an agent asks the human about a decision, it adds its judgement of the
decision's ADR: create an ADR, update which ADR, or no ADR because the decision
is not major. The human may correct the judgement together with the decision
(`.claude/commands/_lib/settle-decisions.md`).

## Writing an ADR

- **状態:** the `状態` row, as "Status" says.
- **背景:** only the problem the decision answers and the assumptions that
  bind it ("What 背景 holds" below).
- **決定:** only the decision in effect now; past decisions go in 経緯. When a
  document holds the current specification of the subject (for example
  `docs/dev/security.md` for the threat model, `docs/dev/production_profile.md`
  for the production profile, `CLAUDE.md` and `.claude/commands/_context.md`
  for the development process), summarize the decision and link to that
  document instead of restating the specification: the specification has one
  source. The ADR holds the reasons, the alternatives, and the history.
- **採らなかった案:** each alternative with the reason it was not taken. When
  the source document gives no reason, write that no reason is recorded
  (`理由は記録されていない`); do not supply one.
- **結果:** the constraints the decision puts on later tasks and components,
  and the costs accepted.
- **経緯:** "History entries" below.

The split between an ADR and a document that holds the current specification,
above, is also in `CLAUDE.md`, "Design decisions (ADR)", with the rest of the
split between the ADRs, the task documents, `docs/dev/project_overview.md`, and
`CLAUDE.md`. When you change it, change both in the same PR.

**Who 背景 and 決定 are for.** A later developer reads an ADR to learn the
decision on a subject they are about to change, and why it was made. Write
背景 and 決定 for that reader, as the next two paragraphs say.

**What 背景 holds.** Write only the problem the decision answers and the
assumptions that bind it. Do not write the content of an earlier rule, the
figures of an investigation, or where a rule came from, unless it is itself a
ground of the decision: an earlier decision goes in 経緯, and a rejected form
in 採らなかった案. A figure the decision rests on stays in 背景, such as a
measured slowdown that decides which build checks a time limit.

**What 決定 holds.** Write one item for each rule a later change must keep.
Give each item a short reason and the place of its specification: a document
and the heading of its section, or a package or a file, without line numbers
("Links" below). When the source document gives no reason for an item, write
`理由は記録されていない`; do not supply one. Do not copy into 決定 a list that
the document holding the specification has (a list of syntax, a list of
exceptions), and do not write implementation details that go stale while the
decision stays the same (the order of function calls, a list of call sites,
the details of a procedure). When the decision, the document holding its
specification, and the code disagree with one another, keep in 決定 a
statement of the discrepancy (a discrepancy statement), linked to the issue
that handles it by the issue's full URL. "Editorial corrections", its
"Discrepancy statements" paragraph, says who adds and removes one.

**Links.** When an ADR states how the code behaves or what a document
specifies, it links to the document that holds the specification, or to the
task document that recorded it. Use relative links without line numbers: an
ADR is a living document, and line numbers go stale. To point at a section,
name its heading in the text instead of linking to an anchor;
`TestInstructionDocPaths` drops anchors, so it would not notice an anchor that
no longer matches.

A task document or `docs/dev/project_overview.md` points at an ADR with a
relative link whose text is `ADR-NNNN`. A title, when given, goes outside the
link, so a later change of the title does not leave a stale link text:

```markdown
[ADR-0001](../../dev/adr/0001-<kebab>.md)（<題名>）
```

## History entries

Each entry of 経緯 is one bullet line of this form:

```markdown
- YYYY-MM-DD <その時点の決定または変更の要旨>（[<出典の文書の呼び名>](<出典の文書への相対リンク>)）
```

- **One entry per event:** each time the decision is made or changed, one
  entry, with the date, the gist of the decision or the change at that time,
  and a link to the document that recorded it.
- **No entry for an editorial correction** ("Editorial corrections"). When a
  rewrite to the criteria moves into 経緯 an earlier decision that the ADR or
  its source documents already hold, and 経緯 has no entry for that decision,
  put it there as the entry of that decision, in the form above, dated as
  "Date of an entry copied from an existing decision" below says. It is not an
  entry for a new event, so it goes in its place by date, not at the end as
  "Order" says for a new entry. When 経緯 already has its entry, only remove it
  from where it was.
- **A disputed correction in a PR with no task document.** When a change
  becomes a decision change because the PR's reviewer disputed it
  ("Editorial corrections") and the PR has no task document, the entry links
  to the PR, whose description records the decision.
- **Order:** the order in which the decisions reached `main`; a new entry goes
  at the end. Decisions take effect in merge order, so this is oldest first.
- **Date of a new entry:** the day the entry is written, not the merge day,
  which is unknown when the entry is written. When taking in `main` moves the
  PR's entry after an entry dated later, change the PR's entry to the day `main`
  was taken in, so the dates never run backwards.
- **Date of an entry copied from an existing decision:** the first of these
  that the source document records:
  1. The date in the heading of the section that states the decision (such as
     `実装の際に決めたこと（YYYY-MM-DD）`).
  2. The date of the approval line in the source document's change history.
     For a document approved more than once, the first approval after the
     change that wrote the decision into the body.
  3. The `Review date` of the source document's status table.
  4. When none of these exists, the date of the first commit that added the
     source document to `main`: the last line of
     `git log --diff-filter=A --format=%cs -- <path>` (renames are not
     followed). Add `（日付は文書を加えたコミットの日付）` after the gist.

  Do not use `Created`: it is the day writing started, not the day of the
  decision.

## Changing, retiring, and withdrawing

- **The decision changes:** rewrite 決定 of the same ADR to the new decision
  and add an entry to 経緯; the past decision stays in 経緯. Put the past
  decision in 採らなかった案 as well only when the document that changed it
  says why it is no longer taken. Do not create a new ADR and retire the old
  one for a changed decision on the same subject.
- **The subject ends** (another subject may take its place): set the
  `状態` row to `廃止`, add the rows `廃止の日付`, `廃止の理由`, and `代わりの ADR`
  (a link to the ADR of the subject that takes its place, or `なし`) to the same
  table, add an entry to 経緯, and set the index row's status to `廃止`. Never
  delete the ADR.
- **The task that made the decision is withdrawn**
  (`docs/dev/conventions/requirements_process.md`, "Withdrawing a task"): in
  the withdrawal PR, revert the part of the decision that the withdrawn task
  added and add an entry to 経緯. When a later task changed the same ADR's
  decision after the withdrawn task, keep that change and revert only the
  withdrawn task's part; when the two cannot be separated, ask the human. When
  the withdrawn task created the ADR, set it to `廃止` instead (do not delete
  it).

## Approval and the PR

A PR that makes or changes a major decision creates or updates the ADR in the
same PR. The rule applies to every PR, including one that no command opens. The
review and merge of that PR approve the ADR; no other approval step exists. An
ADR is added only by the PR that makes its decision. Apart from that PR and a
PR that changes its decision, only two kinds of PR change an ADR: any PR may
make the editorial corrections of "Editorial corrections" below, and a PR of
its own renumbers ADRs when `TestADRRecords` fails on `main` ("Taking in
`main`").

The same rule is in `CLAUDE.md`, "Design decisions (ADR)". When you change it,
change both in the same PR.

## Editorial corrections

An editorial correction changes how an ADR is written without changing its
decision. Any PR, including one that makes or changes no ADR decision, may
make an editorial correction of these four kinds, and only these, to any ADR,
an obsolete (`廃止`) one included:

1. **A rewrite to the criteria** of "Writing an ADR". It may move content that
   the ADR or its source documents already hold to another section, such as
   an earlier decision out of 背景 into 経緯 or a rejected form into
   採らなかった案, and it adds no new fact. It may rewrite the title without
   changing its meaning; the index row then changes in the same PR.
2. **Removing a discrepancy statement** once the discrepancy is fixed, or
   rewriting it to state what remains when only part of it is fixed
   ("Discrepancy statements" below).
3. **Fixing a stale reference**: a link, a name, or a heading that no longer
   matches what it refers to.
4. **Fixing a typo** that does not change the meaning.

The second and third kinds are editorial only when the result matches the
ADR's 決定 as it stands. When the fix changed the behavior or the
specification that 決定 states, or a renamed thing is itself part of the
decision (an environment variable, a protocol member), the change is a
decision change. Writing an existing decision into the document that holds
its specification does not change 決定.

Renumbering an ADR when taking in `main` ("Taking in `main`", step 2) is also
recorded as an editorial correction, but it has its own rule and is not one
of the four kinds.

A change to the meaning of 決定 is not an editorial correction: only a PR that
makes or changes the decision makes it ("Approval and the PR"). When you are
not sure whether a change is editorial, treat it as a decision change.

**Recording.** Record each editorial correction in the message of the commit
that makes it, as a correction line, and in the PR description, as a
correction paragraph, in the forms of
`docs/dev/conventions/requirements_process.md`, "Recording in the PR
description"; the document path in them is the ADR's repository-relative
path. The line says what was changed to what and that no decision changed.
The PR description names the change as an editorial correction, so the PR's
reviewer can dispute the classification. An editorial correction adds no
経緯 entry ("History entries").

**After the PR is created.** Make editorial corrections to an ADR only in
commits made before the PR is created. After that, the commands do not carry
an ADR's correction lines into the PR description (#732), so the reviewer
would not see the classification. A correction already recorded may still be
reworded, as long as its correction line still describes it. Revising text
that the PR itself added to an ADR, and is not yet on `main`, is not an
editorial correction and is not limited by this paragraph.

**Classifying a change.**

- When one PR changes an ADR's decision and also makes editorial corrections
  to the same ADR, record the decision change in 経緯 and the corrections in
  the commit message and the PR description.
- When the PR's reviewer disputes that a change is editorial, treat it as a
  decision change: add a 経緯 entry, and record the decision and its ADR
  judgement where the PR records its decisions
  (`.claude/commands/_lib/settle-decisions.md`): in the task document, or in
  the PR description when the PR has no task document ("History entries").
- A PR that changes the decision of an ADR written before these criteria
  brings only the part it changes to the criteria.

**Discrepancy statements.**

- Adding a discrepancy statement is not an editorial correction. Only a PR
  that makes or changes the ADR's decision adds one; that PR makes sure the
  body of the issue the statement links to names the ADR. A PR that finds a
  discrepancy in an ADR whose decision it does not change opens such an issue
  instead.
- Before asking for the merge, and again whenever it takes in `main`, the PR
  that adds a discrepancy statement checks it against the document and the
  code on the `main` it has fetched or taken in. When the discrepancy is gone
  there, it removes the statement or rewrites it to state what remains. The
  statement was never on `main`, so this revises the PR's own change and
  needs no correction line.
- The PR that fixes a discrepancy removes its statement in the same PR, or
  rewrites it to state what remains when the PR fixes only part. Before the
  PR is created, find the statements on `origin/main`:
  `git grep -nE 'issues/<N>([^0-9]|$)' origin/main -- docs/dev/adr/`, where
  `<N>` is the number of the issue the PR handles. Read each hit: remove only
  a statement in 決定 that describes this discrepancy. When the statement is
  not on the PR's branch yet, take in `main` first. A statement that reaches
  `main` after the PR is created is left to a later PR ("After the PR is
  created").
- A statement that a later PR finds stale may be removed by that PR, as the
  second kind above.

## Taking in `main`

When a PR's branch takes in `main` and `docs/dev/adr/` changed on `main` after
the merge base, do the following.

1. **Check the subject first.** For each ADR the PR created, check whether the
   index on the `main` taken in has an active ADR on the same subject. When it
   does, do not create a new ADR: update the ADR on `main` instead.
2. **Renumber.** When only the number collides (a different subject), move the
   PR's ADR to the next free number: its file name, its first line, and its
   index row, and every `ADR-<old number>` and every link to a file starting
   `<old number>-` in the PR's diff (`git diff origin/main...HEAD`), task
   documents included. Record the renumbering as an editorial correction in
   the commit message (`docs/dev/conventions/requirements_process.md`,
   "Recording in the PR description").
3. **Reconcile changes to the same ADR.** When both the PR and `main` changed
   an ADR, read it again, even when git merged the two without a conflict.
   Check that 決定 matches the decision of the last 経緯 entry, and fix it when
   it does not. The PR's 経緯 entries go after `main`'s ("History entries").

**When `TestADRRecords` fails on `main`.** Two PRs that both pass CI before
taking in `main`, and merge without an index conflict, can put a duplicate or
missing number on `main`. Every PR's `doc-guard` job then fails. Fix it,
renumbering the ADRs so that no number is duplicated or missing, in a PR of
its own that fixes only that, not inside an unrelated PR: that PR's reviewer
is not in a position to approve a change of an ADR's number. This fix is not
one of the four kinds of editorial correction that any PR may make
("Editorial corrections").
