---
description: "Translate README.md (Japanese) into README.en.md (English)"
argument-hint: "README.md"
---

> **Project context**: this command refers to "the translation glossary". Its path
> is defined in `.claude/commands/_context.md` (Process convention: "Translation
> glossary"). Read that file and use its value. The translation pair is defined
> in "Preparation" below, not by a runtime value from `_context.md` —
> `_context.md` describes the project's language-pair convention for reference
> only. The review step follows the shared pattern in
> `.claude/commands/_lib/review-subagent-pattern.md`.

## Preparation

Get the source file path from the argument.
If no argument is provided, ask the user.

This command translates one pair: `README.md` at the repository root
(Japanese, the primary version) into `README.en.md` next to it (English). It is
the only document the project keeps in two languages (`CLAUDE.md`, "Document
language").

- Source `README.md` → output `README.en.md`, **Japanese → English**.
- Any other file, `README.en.md` included: do not translate. Stop and ask the
  user to clarify.

## Standalone and sub-step

This command runs in one of two ways:

- **Standalone**: a person runs `/mktrans README.md`.
- **Sub-step**: another command runs it as one of its own steps (the README
  sync of `_lib/task-pr-handoff.md` Part B, `fixpr.md`, and `runpr.md`). The
  caller passes only `README.md`.

They differ in four ways; every other step is the same.

| | Standalone | Sub-step |
|---|---|---|
| `README.en.md` does not exist | Full translation | Change nothing; the result is `not applicable` |
| The glossary | Add the new terms to it ("Update Glossary") | Leave it unchanged; report the terms that are not in it |
| Committing | Commit as "Commit (standalone only)" says | Never commit; the caller commits `README.en.md` |
| Reporting | Report to the user | Report one of the four results in "Sub-step results" |

A sub-step leaves the glossary alone because `fixpr.md` may change only the
files in scope of the threads it fixes, and the glossary under `docs/` is not in
scope of a thread on `README.md`.

## The sync marker

The first line of `README.en.md` is the sync marker, an HTML comment that names
the `README.md` content the English version was translated from:

```text
<!-- translated-from: README.md <object-id> -->
```

- `<object-id>` is the ID of the blob of that `README.md` content: the full ID
  in lowercase hexadecimal, as long as the repository's object format makes it
  (`git rev-parse --show-object-format`: 40 characters for `sha1`, 64 for
  `sha256`).
- Compute it with `git hash-object -w README.md`. Passing the path applies the
  same attributes (line-ending conversion and the like) as a checkout, and `-w`
  writes the blob into the object database so the later diff can read it.
- The file holds exactly one marker, on its first line. GitHub does not show
  the comment, and it is neither a heading nor a code block, so it does not
  change the structure the README structure test compares.

The marker names a blob, not a commit, so a commit that changes only
`README.en.md` does not move the sync point: a `README.md` change not yet
carried over stays in the diff from the marker's blob.

## Mode Selection

0. **Standalone only: `README.md` is committed.** When `README.md` differs from
   `HEAD` (`git diff --quiet HEAD -- README.md` does not exit `0`), stop and ask
   the user to commit it first: the marker would otherwise name content no
   commit holds, and `README.en.md` would be committed against it. As a
   sub-step, the caller commits `README.md` itself (`fixpr.md` commits it with
   `README.en.md` in Phase 4), so this check does not apply.
1. **`README.en.md` does not exist.** Standalone: go to "Full Translation". As
   a sub-step: change nothing; the result is `not applicable`.
2. **Read the marker.** Stop without translating — the result is `stopped` —
   when any of these holds, and report the reason and its recovery
   ("Recovery"):
   - the first line is not a marker of the form above (`missing`, or
     `malformed` when it starts with `<!-- translated-from:` but does not match
     the form);
   - another line of the file also holds a marker (`more than one`);
   - `git cat-file -t <object-id>` does not print `blob` (`not a blob`, or
     `not found` when the object does not exist in this clone).
3. **Compare.** When `<object-id>` equals `git hash-object README.md`, the
   English version is up to date: change nothing; the result is
   `already in sync`.
4. Otherwise go to "Differential Translation".

---

## Full Translation

Translate the entire source file from scratch.

First check that `README.md` is written in Japanese. When it is not (the README
was English before it became bilingual), stop without translating and tell the
user: an English `README.md` has no English version to make.

### Load Glossary

Read the translation glossary (path in `.claude/commands/_context.md`, Process convention).
Use the terms listed there consistently throughout the translation.

### Translate

Translate strictly following these principles:

- **Accuracy over fluency**: Prioritize precise translation over natural-sounding output.
- **Faithful translation**: Do not delete any content from the source. Do not add any content not present in the source.
- **Structural consistency**: Match chapter headings and sentence structure to the source.

Two parts of the README are not translated word for word, because each version
points at its own counterpart:

- The link at the top of `README.md` to `README.en.md` becomes, in
  `README.en.md`, a link to `README.md`.
- Popup text that `README.md` quotes from the extension's Japanese message
  catalog (`extension/src/core/i18n/ja.ts`) becomes the same message from the
  English catalog (`extension/src/core/i18n/en.ts`), copied, not translated;
  the sentence saying which language the quoted text is in says English.

Write the output to `README.en.md`: the marker on the first line, with
`<object-id>` from `git hash-object -w README.md`, then the translation.

---

## Differential Translation

Translate only what changed in `README.md` since the content the marker names.

### Get the Diff

Run:

```bash
git diff <marker-object-id> "$(git hash-object -w README.md)"
```

### Load Glossary

Read the translation glossary (path in `.claude/commands/_context.md`, Process convention).
Use the terms listed there consistently throughout the translation.

### Translate Only Changed Sections

From the diff, identify which sections (by heading) were added, modified, or removed.

For each changed section:
- **Added lines** (`+`): translate the new source content into the target language.
- **Removed lines** (`-`): identify the corresponding content in the output file and remove it.
- **Modified lines**: translate the new source content and replace the old output content.

Apply these changes to the output file, following the principles and the two
exceptions under "Full Translation", "Translate". Do not touch sections that are
not in the diff. Then replace the marker on the first line with one naming the
blob from the command above.

---

## Update Glossary

Standalone only. If any terms were used during translation that are not in the
glossary, add them to the translation glossary (path in
`.claude/commands/_context.md`, Process convention). Skip this step if no new
terms were introduced. As a sub-step, change nothing here and list those terms
in the result instead.

## Review the Translation (via Subagent)

Run the critical-review subagent procedure in
`.claude/commands/_lib/review-subagent-pattern.md` with these inputs:

- **ARTIFACT**: the translation.
- **PERSONA**: an experienced technical translator and editor. Direct it to
  surface omissions, additions not in the source, mistranslations, and
  inconsistent terminology.
- **FILES**: the source file, the translated output file, and the translation
  glossary (path in `_context.md`) — all as resolved absolute-path strings. For
  a differential translation, also give the diff from "Get the Diff", and
  direct the subagent to review the changed sections against it.
- **CRITERIA**: every item from the Accuracy checklist and the Readability
  checklist below, copied verbatim.

**Accuracy checklist (use verbatim as evaluation criteria in the subagent prompt above):**
- [ ] No content from the source is missing in the translation.
- [ ] No content was added that is not present in the source.
- [ ] All technical terms match the glossary.

**Readability checklist (use verbatim as evaluation criteria in the subagent prompt above):**

The translation principles (Accuracy over fluency, Structural consistency) take precedence. The following checks apply only within those constraints: do not restructure sentences, reorder clauses, or rephrase in ways that would diverge from the source structure.

- [ ] Word choices that are technically correct but unnecessarily obscure are replaced with clearer equivalents that carry the same meaning and preserve the source structure.
- [ ] Terminology is used consistently throughout the translation; the same concept always uses the same term in the target language.
- [ ] Sentence structure follows target-language conventions where the source structure permits it; literal carry-overs from source syntax that produce unnatural output are corrected as long as doing so does not alter meaning or structure.

When a Critical or Major issue is still open after the review passes the shared
pattern allows, stop: the result is `stopped` with the reason
`unresolved review findings`, listing them for a person to decide.

---

## Recovery

Report the recovery for the reason this command stopped:

| Reason | Recovery |
|---|---|
| `missing`, `malformed`, `more than one` | A person finds which `README.md` content `README.en.md` reflects, and writes the blob ID of the `README.md` of that commit on `main` (`git rev-parse <commit>:README.md`) as the marker; the PR review checks it |
| `not a blob`, `not found` | Run `git fetch origin 'refs/pull/*/head'` to bring in the PR head commits, and run this command again. If the blob is still missing, a person writes the marker as in the row above. As a last resort, delete `README.en.md` and run this command standalone for a full translation |
| `unresolved review findings` | List the findings; a person decides |

**Taking in `main`.** When two branches both change `README.md` and sync it,
merging `main` into one of them always conflicts on the marker line of
`README.en.md`. Do not pick either side's marker: the branch's marker would
translate `main`'s change a second time, and `main`'s would translate the
branch's change a second time. Replace `README.en.md` wholesale with `main`'s
version (marker and text), resolve the conflict in `README.md`, and conclude
the merge commit first; then run this command and commit its result
separately. The diff from `main`'s marker is then only the branch's change. An
edit the branch made to `README.en.md` alone (a translation fix) is lost this
way: reapply it after the sync.

## Sub-step results

As a sub-step, end with exactly one of these results, and do not commit.
Before writing `README.en.md`, keep a copy of it outside the work tree; when
this command stops after writing, put the copy back, so `README.en.md` is as
the caller left it (a change the caller made to it, such as a `/fixpr` fix,
stays). What the caller does with each is in the second column.

| Result | The caller |
|---|---|
| `translated` — `README.en.md` was changed; lists the terms not in the glossary | Commits `README.en.md` and reports the terms, as the caller's step says. Continues |
| `already in sync` | Continues |
| `not applicable` (`README.en.md` does not exist) | Continues |
| `stopped` — the reason and its recovery ("Recovery") | Pushes nothing and stops. Reports the reason, its recovery, and the changes left unpushed or uncommitted, which a person decides to keep or discard |

## Commit (standalone only)

Commit only after all review passes are complete and all Critical and Major issues are resolved.

Commit in the following order:
1. Commit the translated file only (do not include glossary changes).
2. If the glossary was updated, commit that as a separate commit.

(No need to wait for user confirmation before committing.)
