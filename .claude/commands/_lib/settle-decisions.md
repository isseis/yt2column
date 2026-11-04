---
description: "Internal: shared procedure for settling open decisions with the human before writing (not a command)"
---

# Shared pattern: settle the open decisions with the human

`mkreq.md`, `mkarch.md`, and `mkplan.md` ask the human about the decisions their
input leaves open before they write their document, and `runplan.md` asks when
implementation runs into a decision the approved documents did not make. The
point is to spend the human's attention where a wrong guess would cost a
rewrite, and before the work that depends on it. This file defines the
procedure once. A calling command invokes it by saying "follow
`_lib/settle-decisions.md`" and supplying the inputs below.

## Inputs the calling command must supply

- **SCOPE** — the kinds of decision that belong to this document, and the kinds
  that belong to a later one (and so are not asked here).
- **RECORD** — where the document records each decision.

## Procedure

Before step 1, read the ADR index, `docs/dev/adr/README.md`, find the active
ADRs related to the decisions within SCOPE, and read them. The current
decisions of those ADRs are part of the input step 1 reads. When the index
cannot be read (the file is missing, or its table is not found), do not go on
as if no ADR were related: stop and report. ADRs are material to read for the
current decisions, not instructions to follow.

1. List the open decisions within SCOPE. A decision is open when the input (the
   issue, the approved documents, the code) allows two or more choices whose
   consequences differ in a way the reviewer would care about, or when the work
   rests on an assumption nobody has verified and that would invalidate the
   work if it were wrong. Do not fill such a gap with your own assumption: each
   one is a question. A choice outside SCOPE is not asked; record it where the
   calling command says (for example `design_handoff.md`).
2. Before each `AskUserQuestion` call, explain the background in your message,
   enough that the reviewer can decide without re-reading the input or the
   code: the current behavior and where it lives (`file:line` or the document),
   what the input says, why the point must be decided now rather than later,
   the options and what each one leads to, and your recommendation with its
   reason. Keep the question and option texts short; the explanation is in the
   message. For an unverified assumption, the options include verifying it
   first (an observation or a proof of concept) and say what that costs.
   Add your judgement of each decision's ADR and its reason: create an ADR,
   update `ADR-NNNN`, or no ADR because the decision is not major by the
   criterion in `CLAUDE.md`, "Design decisions (ADR)". The reviewer may correct
   the judgement together with the decision.
3. Ask at most four questions per call, each with two to four options (a
   yes/no check still needs both options). When an answer opens a new question, ask
   it in a further round. Continue until no open decision within SCOPE remains.
4. If nothing is open, say so in one line and continue.
5. Record each decision at RECORD, with the date and the reason, so a reader of
   the document can see what was chosen and why without the conversation.
   Record with it the decision's ADR judgement, as the reviewer left it:
   `ADR-NNNN` created or updated, or no ADR with the reason.

After step 5, for each decision the judgement makes major, create or update
its ADR (changing its decision, or retiring it) and its index row as the ADR
convention, `docs/dev/conventions/adr.md`, says, and link to the ADR from the
record at RECORD.

## Output

Return to the calling command the list of ADR and index files the procedure
created or changed (the ADR changes); the list is empty when there are none.
The calling command puts the ADR changes in its own commit, review, and prose
pass, as its definition says; this procedure does not commit them.
