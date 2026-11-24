---
description: "Japanese prose-quality pass on a document"
argument-hint: "<file>"
---

> **Project context**: this command reviews and improves the **Japanese prose
> quality** of a document. It refers to "the document language" and "the
> translation glossary", whose values are defined in
> `.claude/commands/_context.md` (Process convention). Read that file and use its
> values. The review step follows the shared pattern in
> `.claude/commands/_lib/review-subagent-pattern.md`. The rest of this command is
> project-independent.
>
> **Harness.** Every harness-specific operation this command and its shared
> procedures name (such as `select-question` or `shell-run`) is defined in
> `.claude/commands/_lib/harness.md`: read it and the adapter it selects for
> this session before step 1.

## Purpose

Improve the readability of a Japanese document by finding and fixing prose-quality
problems — **not** by changing its technical content, facts, numbers, structure, or
section order. This command targets the recurring problems that machine-assisted
Japanese writing tends to introduce:

1. **直訳調 (literal-translation tone)** — English word order, inanimate subjects,
   overuse of passive voice, and calqued idioms that read as a literal rendering of
   English rather than natural Japanese.
2. **不適切・不自然な語の用法** — a word that is technically wrong or unusual for the
   intended meaning (e.g. borrowing a kanji compound for a sense it does not carry,
   such as 「被覆」for "coverage"), or excessive katakana-English / raw English where a
   natural Japanese word exists.
3. **意味のくみ取りにくい一文** — overlong sentences, ambiguous modification (係り受け),
   missing subjects, or too many clauses packed into one sentence.
4. **未定義語の先行使用 (undefined term used before it is introduced)** — a term,
   abbreviation, or piece of jargon used before it is defined in reading order, so a
   reader going through the document top-to-bottom cannot understand it.
5. **用語の不統一** — the same concept written with more than one Japanese term or
   notation, or a deprecated term (旧称) that the glossary says not to use.
6. **冗長・回りくどい言い回し** — wording that can be made more direct without losing
   meaning.
7. **太字の使いすぎ** — a whole sentence, or a span longer than a sentence, set in
   bold where only a label or a word the reader must not miss should be.

## Preparation

Get the target file path from the argument. If no argument is provided and a task
document is open in the IDE, use that; otherwise ask the user which file to review.

**Scope guard — apply this command only to Japanese-primary documents:**

- This command operates on documents written in the document language (Japanese; see
  `_context.md`). Typical targets: task documents under `docs/tasks/`
  (`01_requirements.md`, `02_architecture.md`, `03_implementation_plan.md`) and
  Japanese guide files.
- Do **not** run it on an English `.md` that is the English half of a bilingual pair
  (its Japanese counterpart is `*.ja.md`); translation quality of those is handled by
  `mktrans`. If the target is such an English file, stop and say so.
- If the file is not in Japanese, stop and report that this command is for Japanese
  documents.

**Lines to check for bold.** Decide once, before any fix, which lines of the target
this branch added or changed. The bold item of the review applies only to those
lines, and only in the first review pass.

1. Get the repository top level with `git rev-parse --show-toplevel`, and make the
   target path relative to it. When the target is outside the top level, the lines
   cannot be decided.
2. Get the merge base with `git merge-base origin/main HEAD`. Do not fetch: a
   caller prepared the branch and has fetched `origin/main`; a standalone run uses
   the local `origin/main` as it is.
3. Check whether the path exists at the merge base with
   `git ls-tree --name-only <base> -- <path>`. Empty output means it does not; one
   line naming the path means it does.
4. When the path is absent, run
   `git diff --name-status --no-renames --no-color <base> --`. When any line starts
   with `D`, the branch deletes a file, and the target may have been moved there, so
   the lines cannot be decided. Otherwise treat every line of the document as added.
5. When the path is present, run
   `env -u GIT_DIFF_OPTS git --no-pager diff --no-color --no-ext-diff --no-textconv --text --unified=0 --inter-hunk-context=0 <base> -- <path>`,
   and read the work-tree line ranges from the lines that start with
   `@@ -a[,b] +c[,d] @@` (ignore the text after the second `@@`). A missing `d`
   means one line, and `d` equal to 0 means no line, a deletion only. Those are the
   added and changed lines.
6. Every `git` of steps 1 to 5 must exit 0. A non-zero exit, or output of another
   form in steps 3 to 5, means the lines cannot be decided.

Run each `git` of steps 2 to 5 as
`git -C <top> --literal-pathspecs -c core.quotePath=false ...`. The paths then
resolve from the top level even when `japrose` is called from a subdirectory, a
path holding `[` is not read as a pattern, and the output form does not change with
path quoting.

Decide the lines once, before the fixes: a line that `japrose`'s own expression
fixes changed is not a line the branch added or changed, so the bold item does not
run again on a second pass. `git diff` compares with the work tree, so uncommitted
changes count, and the step works both after the caller committed and when
`japrose` runs alone. When the local `origin/main` is at `HEAD`, as on `main` after
a fetch, the merge base is `HEAD`, so only uncommitted changes are added or changed;
with none, the bold item has no line to check.

## Load the Glossary

Read the translation glossary (path in `_context.md`, Process convention). Use it as
the authority for canonical terminology: the document must use the glossary's terms
and must not use any term the glossary marks as deprecated (旧称). Do **not** invent
new terms; if a clearer term seems warranted but is not in the glossary, flag it as a
finding rather than silently introducing it.

## Review (via Subagent)

Run the critical-review subagent procedure in
`.claude/commands/_lib/review-subagent-pattern.md` in **single-reviewer mode** with
these inputs:

- **ARTIFACT**: the target document.
- **PERSONA**: a technical editor fluent in Japanese, whose job is to find prose that
  is unnatural, hard to follow, literally translated, or terminologically
  inconsistent — and to propose concrete rewrites that preserve the original meaning,
  structure, and section order. It must look for real problems and not rubber-stamp;
  if a category is clean it says so explicitly rather than inventing findings. It must
  prioritize the places where a reader actually stumbles over exhaustive nitpicking.
- **FILES**: the target document; the translation glossary (path in `_context.md`);
  and, when the target is a task document under `docs/tasks/<task>/`, the sibling task
  documents in the same directory that exist (`01_requirements.md`,
  `02_architecture.md`, `03_implementation_plan.md`) — all as resolved absolute-path
  strings. The sibling documents are **read-only reference** so the reviewer can check
  terminology consistency against them (e.g. a plan vs. its architecture); only the
  target document is edited. Also give it the formatting convention
  (`docs/dev/conventions/document_formatting.md`) as a resolved absolute path, and a
  file in the scratchpad that holds the ranges of the lines the branch added or
  changed ("Preparation") and the diff hunks of its step 5. The hunks let the
  reviewer compare a changed line with the line before it and tell whether the
  change is only a link, a name, a heading, or a typo fix. When the ranges could not
  be decided, the file says so and gives the reason instead.
- **CRITERIA**: every item from the Prose-quality checklist and the Hard constraints
  below, copied verbatim.
- **MODEL**: the `standard` role. This review is a fixed-checklist Japanese
  prose/terminology check, not open-ended architectural judgment, so it does not
  need the calling command's model tier (e.g. the `high` role when invoked from
  `mkarch` / `mkplan`) — pin it to the `standard` role regardless of what model is
  driving the caller.

**Prose-quality checklist (use verbatim as evaluation criteria in the subagent prompt above):**
- [ ] 直訳調・英語語順の直訳（無生物主語の直訳、過剰な受動態、英語イディオムの逐語訳）が、意味を変えずに自然な日本語へ書き換えられている。
- [ ] 意図した意味に対して技術的に誤った/一般的でない語（例: "coverage" を「被覆」と当てる類）が、普通の日本語の語へ置き換えられている。
- [ ] 不要なカタカナ英語・生の英語が自然な日本語へ置き換えられている。ただし用語集に載る確立した専門用語、本文書で確立した略語、およびバッククォート内のコード識別子・コマンド名・型名は対象外（むやみに言い換えない）。
- [ ] 長すぎる一文・曖昧な係り受け・主語の欠落・詰め込みすぎの文が、意味を保ったまま分割・整理されて読みやすくなっている。
- [ ] 用語・略語・専門語が、読み進める順序で定義より前に使われていない（未定義の先行使用がない）。ある場合は、初出での簡潔な定義、または後続の定義箇所への参照（例:「後述の §X で定義」）で、頭から読んで理解できるようになっている。
- [ ] 同一概念には常に同一の日本語表記が使われ、用語集に一致している。用語集が「旧称」と記す語は使われていない。
- [ ] 冗長・回りくどい言い回しが、意味を変えずに簡潔にされている。
- [ ] 図（Mermaid）のラベル・キャプション中の日本語にも上記が適用されている（ノード識別子・コード識別子は除く）。
- [ ] 確かめる行（「Review」の FILES で渡すファイルにある）の太字が、書式の規約の太字の規則に合っている（太字は、項目や段落の頭のラベルと、読み落とすと誤解を招く語句に限られ、文全体や、文より長いまとまりは太字にしない）。規則の外にある固定のラベルは対象外とし、確かめる行の外の太字と、確かめる行の外までまたがる太字のまとまりは指摘しない。変えた行のうち、変更がリンク・名前・見出し・誤字の直しだけの行の太字も指摘しない。確かめる行を決められないときと、対象がリポジトリの最上位の README（`README.md`・`README.en.md`）のときは、この項目を確かめない。

**Hard constraints (use verbatim; the reviewer must flag any fix that would violate one of these as out of scope):**
- [ ] 技術的な内容・事実・数値・固有名・構造・節の順序・見出し・文書ステータスを変更しない。表現だけを直す。
- [ ] コードブロック（```～```）の内部は変更しない——**ただし Mermaid 図のラベル/キャプションの表示テキストは対象とする**（Mermaid は ```mermaid のフェンス内にあるが、その表示テキストの日本語は本コマンドの対象）。Mermaid のノード/エッジ識別子、およびバッククォート内のコード識別子・コマンド名・関数名・型名は変更しない。編集対象は地の文と Mermaid 図のラベル/キャプションの表示テキストのみ。
- [ ] 用語集の語を保持し、新しい用語を勝手に導入しない。より良い語が必要なら、置換せずに指摘として挙げる。
- [ ] 既に文書内で一貫して使われている確立表現（例: fail-closed、max 合成、profile、Reject/Critical/High 等）を、「カタカナ/英語だから」という理由だけで言い換えない。

## Apply Fixes

From the review findings, apply the fixes to the document in place:

- Fix all Critical and Major findings. Apply Minor fixes at your discretion.
- For an **undefined-term** finding, prefer defining the term briefly at its first
  occurrence; use a pointer to where it is defined later (e.g.「後述の §X で定義」) when
  an inline definition would disrupt the flow. Do **not** reorder sections or move
  content to fix it — the structure-preservation Hard constraint forbids that; use a
  local inline definition or a forward pointer only.
- Never change meaning, numbers, code, or section order to satisfy a prose finding. If
  a finding cannot be fixed without violating a Hard constraint, leave the text and
  note why.
- Preserve Mermaid syntax and code fences exactly; re-check that fence counts stay
  balanced after editing diagram labels.
- For a bold finding, remove only the bold delimiters (`**` or `__`) and keep every
  character as it is. Do not remove the bold of a fixed label the formatting convention puts outside
  its rules, and do not touch a bold span that reaches outside the checked lines.
  Removing the markers is an expression-only fix: do not add subheadings or
  restructure the document to satisfy the formatting convention.
- When the lines could not be decided, do not check the bold item. Say in the report
  that it was not checked and why, and continue the other items. This case reports
  the reason, not a count.
- Otherwise, in the report, give the number of lines the bold item checked. When the
  number is 0, say that there was no line to check.

If any Critical or Major finding required a fix, run a second review pass to verify
the fixes (subject to the three-pass limit in the shared pattern), then stop.

## Commit

If this command was invoked **as a sub-step of another command** (e.g. `mkarch` /
`mkplan`), do **not** commit — return control to the calling command, which owns the
commit. Report the changes you made.

If this command was invoked **standalone**, commit the edited document after all
review passes are complete and all Critical and Major findings are resolved. Use a
`docs(<task-id>): improve Japanese prose` style message that lists the kinds of fixes
applied. (No need to wait for user confirmation before committing.)
