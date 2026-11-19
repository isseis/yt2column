# Document Formatting

This convention defines the formatting of Japanese documents in this
repository: when bold is allowed, and how a section with many items is built so
that a reader can grasp it before the details.

## Target documents

The rules apply to the Japanese documents of `CLAUDE.md`, "Document language",
principle (a): the task documents under `docs/tasks/`, the documents directly
under `docs/dev/`, the ADRs and their index under `docs/dev/adr/`,
`docs/translation_glossary.md`, the guides under `docs/dev/guides/`, the
research records under `docs/research/`, and `prompts/README.md` and
`testdata/README.md`.

The English documents of principle (b) are not target: `CLAUDE.md`, `AGENTS.md`,
the command definitions under `.claude/commands/`, and the conventions under
`docs/dev/conventions/`, including this one. The README of principle (c) is not
target either: `README.md` and its English version `README.en.md`.

## Where the rules apply

The rules apply to a document a change writes from scratch, and to each section
whose change adds to or rewrites the content of its items. Bring such a section
wholly into line with the rules, including the items the change does not touch,
so that one section does not mix two ways of writing.

A change that only fixes a link, a name, or a heading, removes a discrepancy
statement, or corrects a typo does not start this. Such a change is an editorial
correction, and the section does not have to be rewritten for it. Do not rewrite
a section the change does not touch.

## Bold

Use bold for only two purposes:

- **Labels.** A label is a short phrase without a predicate, ending in a noun,
  at the head of an item or a paragraph. It ends with a colon (`:` or `：`) or a
  period (`。` or `.`); the ending mark may be inside the bold or immediately
  after it.
- **Easy-to-miss qualifiers.** Give bold to a word or phrase in body text that a
  reader would misread if they passed over it, such as a negation or an
  exception.

Do not put a whole sentence, or a span longer than a sentence, in bold.

Some bold labels are fixed: a template, a convention, or a tool decides their
form, and a tool may read them. Write those labels in the prescribed form, bold
included. Examples are the implementation plan's `**実装モデル要件**:`, an
acceptance criterion's `**AC-01**`, and a handoff item's `**扱った箇所:**`; the
examples are not exhaustive.

## Sections with many items

A section with many items starts with a summary of one or two sentences that
says what the section settles, so that a reader can grasp the whole before the
details.

Structure the details with subheadings or nested bullet lists. When an item's
rule, its reason, and the place of its source of authority are long, do not cram
them into one paragraph: put them on separate lines or under a subheading. When
all of them are short, one paragraph for the item is fine. The writer judges
which case applies.

The form inside an item — the order of its rule, reason, and source of
authority, for example — is left to the writer. This convention does not decide
it; a document that holds the subject may.

## Short sections and fixed forms

A short section needs neither a summary nor structure: its bullet list may stay
as it is. No numeric threshold decides what is short; the writer judges it.

Where a template or a convention decides the form of a place, follow that form.
The items of an ADR's 「経緯」 (history) section are an example.
