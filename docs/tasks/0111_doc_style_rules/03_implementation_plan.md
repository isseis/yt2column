# 実装計画書：日本語の文書の太字を絞り、項目の多い節はまとめを先に書いて構造化する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-19 |
| Review date | 2026-11-19 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。要件書・設計書・本計画を、以下「3 文書」と呼ぶ。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

書式の規約 `docs/dev/conventions/document_formatting.md` を作り、`CLAUDE.md` と `adr.md` からリンクし、`japrose` に太字の確認を足し、ADR-0039 の書式の項目のリンクを規約へ替える（設計書 3.1〜3.5 節）。

### 1.2. 実装の原則

-   規約とコマンドの定義は英語で書く（`CLAUDE.md`「Document language」の原則 (b)）。`japrose` の基準の項目は、今の基準と同じく日本語で書く。
-   本タスクで新しく書く文は、書式の規約に従う。差分は AC-13 が挙げる箇所に限る。

### 1.3. 計画の際に決めたこと（2026-11-19）

計画で新たに人に確かめた判断はない。フェーズと PR を 1 つにすること、および変更の範囲を記録した diff で確かめることは、設計書の 7 章と 8 章で決まっており、計画で選ぶ判断ではない。

### 1.4. 既存コード調査結果

コミット `21aef492`（`origin/main`）で確かめた。

-   `CLAUDE.md` の「Documents」は `CLAUDE.md:27-41` にあり、Mermaid の項目は `:34-37` にある。
-   `adr.md` の「Writing an ADR」は `docs/dev/conventions/adr.md:128` から始まる。「What 決定 holds」は `:163-175`、「Links」は `:177` にある。`TestADRRecords` が読む「Template」「Status」「Index」の節は変えない。
-   `japrose.md` の「Purpose」は `.claude/commands/japrose.md:14-36`、「Preparation」は `:38-54`、「Review」の FILES は `:76-82`、Prose-quality checklist は `:90-98`、Hard constraints は `:100-104`、「Apply Fixes」は `:106-123` にある。
-   `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`）は、`CLAUDE.md` と `.claude/commands/`・`docs/dev/` の下の文書のリンクの先があることを確かめる。新しい規約、`adr.md`、`japrose.md`、ADR-0039 も対象になる。`cmd/numreserve` のガードは `adr.md` とコマンドの定義を読む。
-   `go test -tags test ./cmd/yt2column ./cmd/numreserve` は、コミット `9442e9ab` に 3 文書の見直しと ADR-0039 の項目を加えた作業ツリーで通った（`ok`、2 パッケージ）。
-   設計書 3.4.1 節の最後の段落が頼る `git` の終了状態は、コミット `21aef492` で確かめた。
-   `japrose` を呼ぶのは、`.claude/commands/mkreq.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`_lib/simple-process.md` と、それを説明する `_lib/task-pr-handoff.md` である（`grep -l japrose .claude/commands/*.md .claude/commands/_lib/*.md`、コミット `9442e9ab`）。
-   `docs/dev/conventions/` の下の文書の一覧を持つガードやテストはない。`docs/dev/conventions/package_reference.md` はパッケージの一覧で、規約の一覧ではない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 規約の新設と参照、japrose の太字の確認（設計書 3 章、AC-01〜AC-17。AC-16 は実装の PR の側）

#### ステップ 1-1: 書式の規約

**変えるファイル:** `docs/dev/conventions/document_formatting.md`（新規）

- [x] 設計書 3.1 節のとおり、対象の文書、当てる範囲、太字の規則、項目の多い節の規則を英語で書く

**完了の条件:** 規約が AC-01〜AC-05 の内容を述べる。

#### ステップ 1-2: `CLAUDE.md` と `adr.md` からの参照

**変えるファイル:** `CLAUDE.md`、`docs/dev/conventions/adr.md`

- [x] 設計書 3.2 節のとおり、`CLAUDE.md` の「Documents」の Mermaid の項目の次に、規約へリンクする 1 項目を足す
- [x] 設計書 3.3 節のとおり、`adr.md` の「What 決定 holds」と「Links」の間に、規約へリンクし、「決定」「採らなかった案」「結果」を挙げ、短い節の扱いと、編集上の訂正として書き直してよいことを述べる 1 段落を足す
- [x] 設計書 3.5 節のとおり、ADR-0039 の書式の項目のずれの記述を消し（訂正の 2 種目）、正本へのリンクの先と文字を、本タスクの要件書の F-001・F-002 から書式の規約と `adr.md` の「Writing an ADR」へ替える（3 種目）。訂正の行をコミットメッセージに書く

**完了の条件:** `CLAUDE.md` が AC-06 の、`adr.md` が AC-07・AC-15 の内容を述べ、ADR-0039 の項目が規約へリンクする（AC-16）。

#### ステップ 1-3: `japrose` の太字の確認

**変えるファイル:** `.claude/commands/japrose.md`

- [x] 設計書 3.4.1 節のとおり、「Preparation」に、確かめる行を決める手順と、決められないときの扱いを足す
- [x] 設計書 3.4.2 節のとおり、「Review」の FILES に、書式の規約と、確かめる行の範囲と差分のハンクを書いたファイルを足す
- [x] 設計書 3.4.3 節のとおり、Prose-quality checklist に太字の項目を日本語で 1 つ足し、「Purpose」の問題の一覧に太字の使いすぎを足す
- [x] 設計書 3.4.4 節のとおり、「Apply Fixes」に、太字の直し方、確かめなかったときの報告、確かめた行の数の報告を足す

**完了の条件:** `japrose.md` が AC-08〜AC-11・AC-17 の内容を述べる。

#### ステップ 1-4: 仕上げ

- [x] `go test -tags test ./cmd/yt2column ./cmd/numreserve` を通す（AC-14）
- [x] `make test` と `make lint` を通す
- [x] 7 章の AC-12・AC-13 の diff の記録を書く

**完了の条件:** テストが通り、diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 書式の規約、`CLAUDE.md` と `adr.md` からの参照、ADR-0039 のリンク、`japrose` の太字の確認、diff の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-4 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（文書と 1 つのコマンドの定義の追記で、方式の選択も危険度の高いステップもない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   新しいテストは足さない。変わるのは文書と、コマンドの定義の文だけで、文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。
-   既存のガード（`go test -tags test ./cmd/yt2column ./cmd/numreserve`）が、リンクの先があること、ADR の記録の形、`numreserve` の呼び出しの形を確かめる（AC-14）。
-   変更の範囲は、記録した diff の読み取りで確かめる（AC-12・AC-13）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| `japrose` が確かめる行の外の太字まで外す | 基準の項目と「Apply Fixes」の両方に、確かめる行の外を変えないことを書く（設計書 3.4.3・3.4.4 節）。パスをリポジトリの最上位からの相対パスにし、決められないときは確かめない側に倒す（設計書 3.4.1 節）。PR のレビューで読む |
| `japrose` が固定のラベルの太字を外し、道具が値を読めなくなる | 基準の項目と「Apply Fixes」に、固定のラベルを対象外とすることを書く。固定のラベルの例を規約に挙げる |
| 規約と `adr.md` の段落が同じ規則を重ねて書き、後で食い違う | `adr.md` にはリンクと当てはめだけを書く（設計書 1 章の設計原則）。PR のレビューで読む |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] フェーズ 1（ステップ 1-1〜1-4）
- [ ] PR を作成した
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR のレビューで、規約があり英語で書かれ、対象が原則 (a) の日本語の文書で、英語の文書と README は対象でないと述べることを読んで確かめる |
| AC-02 | `manual` | 同じく、規約が、新しく書く文書と、変更が項目の中身を足すか書き換える節に当てて節全体を合わせ、リンク・名前・見出し・ずれの記述・誤字だけの直しはきっかけにせず、触れない節の書き直しを求めないと述べることを読んで確かめる |
| AC-03 | `manual` | 同じく、規約が、太字にしてよいもの（コロンか句点で終わるラベルの定義を含む）・しないもの・規則の外の固定のラベルを述べることを読んで確かめる |
| AC-04 | `manual` | 同じく、規約が、まとめを先に書くこと、小見出しか入れ子の箇条書きで構造化すること、項目の中の形を書き手に任せることを述べることを読んで確かめる |
| AC-05 | `manual` | 同じく、規約が、短い節の扱い（まとめも構造化も求めず、閾値なしで書き手が判断）と、テンプレートか規約が決める形の扱いを述べることを読んで確かめる |
| AC-06 | `manual` | 同じく、`CLAUDE.md` の「Documents」に規約へリンクする項目があることを読んで確かめる。リンクの先があることは AC-14 が確かめる |
| AC-07 | `manual` | 同じく、`adr.md` の「Writing an ADR」が規約へリンクし、書式は節全体・中身の基準は変える部分だけと述べ、3 つの節を挙げ、短い節の扱いを述べることを読んで確かめる |
| AC-08 | `manual` | 同じく、`japrose.md` の Prose-quality checklist に太字の項目が 1 つあり、「Review」の FILES に書式の規約があることを読んで確かめる |
| AC-09 | `manual` | 同じく、`japrose.md` が次の 5 点を述べることを読んで確かめる。(1) `git` を最上位から、最上位からのパスで呼ぶ。(2) 確かめる行を merge-base との差分から決める。(3) merge-base になく、ブランチがほかのファイルを消していない文書は、すべての行を確かめる。(4) 行は直しの前に 1 回だけ決め、太字は 1 回目の確認でだけ確かめる。(5) レビュアーに範囲とハンクを渡し、リンク・名前・見出し・誤字の直しだけの行と、それ以外の行の太字は指摘も変更もしない |
| AC-10 | `manual` | 同じく、`japrose.md` が、3 つの「決められないとき」（`git` の失敗か想定しない出力、作業ツリーの外、ブランチがほかのファイルを消すか動かすとき）に太字の項目を確かめず、理由とともに報告に書き、ほかの項目を続けると述べることを読んで確かめる |
| AC-11 | `manual` | 同じく、`japrose.md` が、太字の印だけを外して文字を変えず、固定のラベルの太字を外さないと述べることを読んで確かめる |
| AC-12 | `diff` | 下の記録のファイルの一覧を、PR のレビューで読む（記録した diff の読み取り） |
| AC-13 | `diff` | 下の記録の 4 つのファイルの差分を、PR のレビューで読む（記録した diff の読み取り） |
| AC-14 | `test` | `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`、`cmd/yt2column/adr_test.go::TestADRRecords`、`cmd/numreserve` のガード（`go test -tags test ./cmd/yt2column ./cmd/numreserve`） |
| AC-15 | `manual` | PR のレビューで、`adr.md` の段落が、既存の ADR の書き直しを求めないが編集上の訂正の 1 種目として行ってよいと述べることを読んで確かめる |
| AC-16 | `manual` | 3 文書の PR のレビューで、ADR-0039 の「決定」と「経緯」に項目があり、要件書の F-001・F-002 へリンクし、#1011 へリンクするずれの記述を持つことを読んで確かめる。実装の PR では、ずれの記述が消え、リンクが規約と `adr.md` へ替わったことを AC-13 の記録で、訂正の行をコミットメッセージで確かめる |
| AC-17 | `manual` | PR のレビューで、`japrose.md` が、確かめた行の数と、0 行のときはそのことを報告すると述べることを読んで確かめる |

AC-12 は、下の記録のファイルの一覧で確かめる。その記録の比べるパスは、本計画を含まない 5 つのファイルである。

Diff record (AC-12):

- Compared paths: `docs/dev/conventions/document_formatting.md` `CLAUDE.md` `docs/dev/conventions/adr.md` `.claude/commands/japrose.md` `docs/dev/adr/0039-adr-writing-criteria.md`
- Merge-base: `b93d1111834de21a5dbe9de2093f2f80d26606bf`
- Compared commit: `d0feba07b2a6a903d24ce0e8c77093fcd40fca0f`

Path diff:

````text
diff --git a/.claude/commands/japrose.md b/.claude/commands/japrose.md
index 23e12d0dee70887ecd0e8f78195fb5e14dd30446..d0f8f74a7276f54d3ba1982303f5d79803c9ddca 100644
--- a/.claude/commands/japrose.md
+++ b/.claude/commands/japrose.md
@@ -34,6 +34,8 @@ Japanese writing tends to introduce:
    notation, or a deprecated term (旧称) that the glossary says not to use.
 6. **冗長・回りくどい言い回し** — wording that can be made more direct without losing
    meaning.
+7. **太字の使いすぎ** — a whole sentence, or a span longer than a sentence, set in
+   bold where only a label or a word the reader must not miss should be.

 ## Preparation

@@ -52,6 +54,46 @@ document is open in the IDE, use that; otherwise ask the user which file to revi
 - If the file is not in Japanese, stop and report that this command is for Japanese
   documents.

+**Lines to check for bold.** Decide once, before any fix, which lines of the target
+this branch added or changed. The bold item of the review applies only to those
+lines, and only in the first review pass.
+
+1. Get the repository top level with `git rev-parse --show-toplevel`, and make the
+   target path relative to it. When the target is outside the top level, the lines
+   cannot be decided.
+2. Get the merge base with `git merge-base origin/main HEAD`. Do not fetch: a
+   caller prepared the branch and has fetched `origin/main`; a standalone run uses
+   the local `origin/main` as it is.
+3. Check whether the path exists at the merge base with
+   `git ls-tree --name-only <base> -- <path>`. Empty output means it does not; one
+   line naming the path means it does.
+4. When the path is absent, run
+   `git diff --name-status --no-renames --no-color <base> --`. When any line starts
+   with `D`, the branch deletes a file, and the target may have been moved there, so
+   the lines cannot be decided. Otherwise treat every line of the document as added.
+5. When the path is present, run
+   `env -u GIT_DIFF_OPTS git --no-pager diff --no-color --no-ext-diff --no-textconv --text --unified=0 --inter-hunk-context=0 <base> -- <path>`,
+   and read the work-tree line ranges from the lines that start with
+   `@@ -a[,b] +c[,d] @@` (ignore the text after the second `@@`). A missing `d`
+   means one line, and `d` equal to 0 means no line, a deletion only. Those are the
+   added and changed lines.
+6. Every `git` of steps 1 to 5 must exit 0. A non-zero exit, or output of another
+   form in steps 3 to 5, means the lines cannot be decided.
+
+Run each `git` of steps 2 to 5 as
+`git -C <top> --literal-pathspecs -c core.quotePath=false ...`. The paths then
+resolve from the top level even when `japrose` is called from a subdirectory, a
+path holding `[` is not read as a pattern, and the output form does not change with
+path quoting.
+
+Decide the lines once, before the fixes: a line that `japrose`'s own expression
+fixes changed is not a line the branch added or changed, so the bold item does not
+run again on a second pass. `git diff` compares with the work tree, so uncommitted
+changes count, and the step works both after the caller committed and when
+`japrose` runs alone. When the local `origin/main` is at `HEAD`, as on `main` after
+a fetch, the merge base is `HEAD`, so only uncommitted changes are added or changed;
+with none, the bold item has no line to check.
+
 ## Load the Glossary

 Read the translation glossary (path in `_context.md`, Process convention). Use it as
@@ -79,7 +121,13 @@ these inputs:
   `02_architecture.md`, `03_implementation_plan.md`) — all as resolved absolute-path
   strings. The sibling documents are **read-only reference** so the reviewer can check
   terminology consistency against them (e.g. a plan vs. its architecture); only the
-  target document is edited.
+  target document is edited. Also give it the formatting convention
+  (`docs/dev/conventions/document_formatting.md`) as a resolved absolute path, and a
+  file in the scratchpad that holds the ranges of the lines the branch added or
+  changed ("Preparation") and the diff hunks of its step 5. The hunks let the
+  reviewer compare a changed line with the line before it and tell whether the
+  change is only a link, a name, a heading, or a typo fix. When the ranges could not
+  be decided, the file says so and gives the reason instead.
 - **CRITERIA**: every item from the Prose-quality checklist and the Hard constraints
   below, copied verbatim.
 - **MODEL**: `sonnet`. This review is a fixed-checklist Japanese prose/terminology
@@ -96,6 +144,7 @@ these inputs:
 - [ ] 同一概念には常に同一の日本語表記が使われ、用語集に一致している。用語集が「旧称」と記す語は使われていない。
 - [ ] 冗長・回りくどい言い回しが、意味を変えずに簡潔にされている。
 - [ ] 図（Mermaid）のラベル・キャプション中の日本語にも上記が適用されている（ノード識別子・コード識別子は除く）。
+- [ ] 確かめる行（「Review」の FILES で渡すファイルにある）の太字が、書式の規約の太字の規則に合っている（太字は、項目や段落の頭のラベルと、読み落とすと誤解を招く語句に限られ、文全体や、文より長いまとまりは太字にしない）。規則の外にある固定のラベルは対象外とし、確かめる行の外の太字と、確かめる行の外までまたがる太字のまとまりは指摘しない。変えた行のうち、変更がリンク・名前・見出し・誤字の直しだけの行の太字も指摘しない。確かめる行を決められないときと、対象がリポジトリの最上位の README（`README.md`・`README.en.md`）のときは、この項目を確かめない。

 **Hard constraints (use verbatim; the reviewer must flag any fix that would violate one of these as out of scope):**
 - [ ] 技術的な内容・事実・数値・固有名・構造・節の順序・見出し・文書ステータスを変更しない。表現だけを直す。
@@ -118,6 +167,16 @@ From the review findings, apply the fixes to the document in place:
   note why.
 - Preserve Mermaid syntax and code fences exactly; re-check that fence counts stay
   balanced after editing diagram labels.
+- For a bold finding, remove only the bold delimiters (`**` or `__`) and keep every
+  character as it is. Do not remove the bold of a fixed label the formatting convention puts outside
+  its rules, and do not touch a bold span that reaches outside the checked lines.
+  Removing the markers is an expression-only fix: do not add subheadings or
+  restructure the document to satisfy the formatting convention.
+- When the lines could not be decided, do not check the bold item. Say in the report
+  that it was not checked and why, and continue the other items. This case reports
+  the reason, not a count.
+- Otherwise, in the report, give the number of lines the bold item checked. When the
+  number is 0, say that there was no line to check.

 If any Critical or Major finding required a fix, run a second review pass to verify
 the fixes (subject to the three-pass limit in the shared pattern), then stop.
diff --git a/CLAUDE.md b/CLAUDE.md
index 69115868f9cb3e1c5b35f90854f42df725be4121..44e9e6b64222aee5cf0e42b87f2c9395022df950 100644
--- a/CLAUDE.md
+++ b/CLAUDE.md
@@ -35,6 +35,10 @@ This file provides guidance to Claude Code (claude.ai/code) when working with co
   [mermaid_reference.md](docs/dev/conventions/mermaid_reference.md) (node-label
   quoting, `<br>` line breaks, cylinder `[(...)]` data nodes, the standard classDef
   palette, and Legend blocks).
+- Japanese documents use bold and structure a section with many items as
+  [document_formatting.md](docs/dev/conventions/document_formatting.md) says
+  (labels and easy-to-miss qualifiers only; a summary first, then subheadings or
+  nested lists).
 - Each task lives in `docs/tasks/NNNN_<name>/` (four-digit sequence number).
   `docs/tasks/0000_template/requirements.md` is the starting point for
   `01_requirements.md`.
diff --git a/docs/dev/adr/0039-adr-writing-criteria.md b/docs/dev/adr/0039-adr-writing-criteria.md
index 963d0c65820d401cd2535537162940360348ac3e..cd978c78bd1ddd03f76f88f58b90e6e7debe7774 100644
--- a/docs/dev/adr/0039-adr-writing-criteria.md
+++ b/docs/dev/adr/0039-adr-writing-criteria.md
@@ -21,7 +21,7 @@ ADR は、後の開発者が、変えようとしている主題の決定とそ
 -   **`/mergepr` で squash マージするときは、PR のコミットにある訂正の行を、タスク文書と ADR のどちらのものも、要約せず 1 行ずつそのまま squash コミットの本文に引き継ぐ。** 行数にも 1 行の長さにも上限を設けない。人は承認のときに訂正の行を消せず、要件定義書が定める場合に限って落とせる。落としたときは、落としたことを本文に記録する。squash マージの後は PR のコミットが `main` からたどれなくなり、PR の説明はリポジトリの外にあって書き換えられるので、訂正の記録を `main` の履歴からたどれるようにするためである。行を受け付ける条件と落とし方の仕様は、[0106 要件定義書](../../tasks/0106_squash_editorial_corrections/01_requirements.md) の「3. 機能要件」にある。今の `/mergepr` の手順と規約は、まだ訂正の行を squash コミットの本文に引き継がない。このずれは、[#747](https://github.com/isseis/yt2column/issues/747) で扱う。
 -   **ずれの記述は、ずれを直す PR が同じ PR で消す。** ずれの記述が事実でない期間を作らないためである。ずれを直す人が記述を見つけられるように、issue から ADR を、ADR から issue をたどれるようにする。ずれの記述を足す PR は、マージを頼む前と `main` を取り込むときに、最新の `main` に照らして、そのずれの記述がまだ事実かどうかを確かめ直す。規則は [adr.md](../conventions/adr.md) の「Writing an ADR」「Editorial corrections」にある。
 -   **`main` の上で `TestADRRecords` が失敗したときの、番号を付け替える直しは、その直しだけの PR で行う。** 番号の付け替えは ADR の識別子を変え、ほかの文書の参照にも及ぶ。関係のない PR のレビュアーは、これを承認する立場にないためである。規則は [adr.md](../conventions/adr.md) の「Taking in `main`」にある。
--   **ADR の書式（太字の使い方と、項目の多い節の組み立て方）は、日本語の文書の書式の規約に従う。既存の ADR を書式の規約に合わせる書き直しは求めないが、編集上の訂正の 1 種目（基準に合わせる書き直し）として、どの PR も行ってよい。** 書式の規則を、ADR に限らず日本語の文書で 1 つにするためである。書き直しを訂正として認めるのは、訂正がコミットと PR の説明に記録され、レビュアーが確かめられるためである。規則は [0111 要件定義書](../../tasks/0111_doc_style_rules/01_requirements.md) の F-001 と F-002 にある。今の [adr.md](../conventions/adr.md) の「Writing an ADR」と「Editorial corrections」は、まだ書式の規約を含まず、書式の規約もまだない。このずれは、[#1011](https://github.com/isseis/yt2column/issues/1011) で扱う。
+-   **ADR の書式（太字の使い方と、項目の多い節の組み立て方）は、日本語の文書の書式の規約に従う。既存の ADR を書式の規約に合わせる書き直しは求めないが、編集上の訂正の 1 種目（基準に合わせる書き直し）として、どの PR も行ってよい。** 書式の規則を、ADR に限らず日本語の文書で 1 つにするためである。書き直しを訂正として認めるのは、訂正がコミットと PR の説明に記録され、レビュアーが確かめられるためである。規則は [書式の規約](../conventions/document_formatting.md) と [adr.md](../conventions/adr.md) の「Writing an ADR」にある。

 ## 採らなかった案

diff --git a/docs/dev/conventions/adr.md b/docs/dev/conventions/adr.md
index 8ca3d754e9a56a38d305d2d12434757c4e80cf76..f681d0e23230697f663240b09e6baa1fab0309ee 100644
--- a/docs/dev/conventions/adr.md
+++ b/docs/dev/conventions/adr.md
@@ -174,6 +174,19 @@ statement of the discrepancy (a discrepancy statement), linked to the issue
 that handles it by the issue's full URL. "Editorial corrections", its
 "Discrepancy statements" paragraph, says who adds and removes one.

+**Format.** An ADR follows the bold and many-item-section rules of the
+Japanese document formatting convention
+([document_formatting.md](document_formatting.md)). The sections that tend to
+hold many items are 決定, 採らなかった案, and 結果; a short section may stay a
+bullet list, as the convention says. The formatting applies to the whole
+section, while the content criteria for what each section holds keep their
+current scope: a PR that changes the decision of an ADR written before these
+criteria brings only the part it changes to the content criteria; the formatting
+still applies to the whole section. Rewriting an existing ADR to the formatting
+convention is not required, but any PR may do it as the first kind of editorial
+correction ("Editorial corrections" below), a rewrite to the criteria, which now
+include this formatting convention.
+
 **Links.** When an ADR states how the code behaves or what a document
 specifies, it links to the document that holds the specification, or to the
 task document that recorded it. Use relative links without line numbers: an
diff --git a/docs/dev/conventions/document_formatting.md b/docs/dev/conventions/document_formatting.md
new file mode 100644
index 0000000000000000000000000000000000000000..2ab88cc3d79e6923fd70589dc601e507f9925a21
--- /dev/null
+++ b/docs/dev/conventions/document_formatting.md
@@ -0,0 +1,75 @@
+# Document Formatting
+
+This convention defines the formatting of Japanese documents in this
+repository: when bold is allowed, and how a section with many items is built so
+that a reader can grasp it before the details.
+
+## Target documents
+
+The rules apply to the Japanese documents of `CLAUDE.md`, "Document language",
+principle (a): the task documents under `docs/tasks/`, the documents directly
+under `docs/dev/`, the ADRs and their index under `docs/dev/adr/`,
+`docs/translation_glossary.md`, the guides under `docs/dev/guides/`, the
+research records under `docs/research/`, and `prompts/README.md` and
+`testdata/README.md`.
+
+The English documents of principle (b) are not target: `CLAUDE.md`, `AGENTS.md`,
+the command definitions under `.claude/commands/`, and the conventions under
+`docs/dev/conventions/`, including this one. The README of principle (c) is not
+target either: `README.md` and its English version `README.en.md`.
+
+## Where the rules apply
+
+The rules apply to a document a change writes from scratch, and to each section
+whose change adds to or rewrites the content of its items. Bring such a section
+wholly into line with the rules, including the items the change does not touch,
+so that one section does not mix two ways of writing.
+
+A change that only fixes a link, a name, or a heading, removes a discrepancy
+statement, or corrects a typo does not start this. Such a change is an editorial
+correction, and the section does not have to be rewritten for it. Do not rewrite
+a section the change does not touch.
+
+## Bold
+
+Use bold for only two purposes:
+
+- **Labels.** A label is a short phrase without a predicate, ending in a noun,
+  at the head of an item or a paragraph. It ends with a colon (`:` or `：`) or a
+  period (`。` or `.`); the ending mark may be inside the bold or immediately
+  after it.
+- **Easy-to-miss qualifiers.** Give bold to a word or phrase in body text that a
+  reader would misread if they passed over it, such as a negation or an
+  exception.
+
+Do not put a whole sentence, or a span longer than a sentence, in bold.
+
+Some bold labels are fixed: a template, a convention, or a tool decides their
+form, and a tool may read them. Write those labels in the prescribed form, bold
+included. Examples are the implementation plan's `**実装モデル要件**:`, an
+acceptance criterion's `**AC-01**`, and a handoff item's `**扱った箇所:**`; the
+examples are not exhaustive.
+
+## Sections with many items
+
+A section with many items starts with a summary of one or two sentences that
+says what the section settles, so that a reader can grasp the whole before the
+details.
+
+Structure the details with subheadings or nested bullet lists. When an item's
+rule, its reason, and the place of its source of authority are long, do not cram
+them into one paragraph: put them on separate lines or under a subheading. When
+all of them are short, one paragraph for the item is fine. The writer judges
+which case applies.
+
+The form inside an item — the order of its rule, reason, and source of
+authority, for example — is left to the writer. This convention does not decide
+it; a document that holds the subject may.
+
+## Short sections and fixed forms
+
+A short section needs neither a summary nor structure: its bullet list may stay
+as it is. No numeric threshold decides what is short; the writer judges it.
+
+Where a template or a convention decides the form of a place, follow that form.
+The items of an ADR's 「経緯」 (history) section are an example.
````

File list:

````text
.claude/commands/japrose.md
CLAUDE.md
docs/dev/adr/0039-adr-writing-criteria.md
docs/dev/conventions/adr.md
docs/dev/conventions/document_formatting.md
docs/tasks/0111_doc_style_rules/03_implementation_plan.md
````

Diff record (AC-13):

- Compared paths: `CLAUDE.md` `docs/dev/conventions/adr.md` `.claude/commands/japrose.md` `docs/dev/adr/0039-adr-writing-criteria.md`
- Merge-base: `b93d1111834de21a5dbe9de2093f2f80d26606bf`
- Compared commit: `d0feba07b2a6a903d24ce0e8c77093fcd40fca0f`

Path diff:

````text
diff --git a/.claude/commands/japrose.md b/.claude/commands/japrose.md
index 23e12d0dee70887ecd0e8f78195fb5e14dd30446..d0f8f74a7276f54d3ba1982303f5d79803c9ddca 100644
--- a/.claude/commands/japrose.md
+++ b/.claude/commands/japrose.md
@@ -34,6 +34,8 @@ Japanese writing tends to introduce:
    notation, or a deprecated term (旧称) that the glossary says not to use.
 6. **冗長・回りくどい言い回し** — wording that can be made more direct without losing
    meaning.
+7. **太字の使いすぎ** — a whole sentence, or a span longer than a sentence, set in
+   bold where only a label or a word the reader must not miss should be.

 ## Preparation

@@ -52,6 +54,46 @@ document is open in the IDE, use that; otherwise ask the user which file to revi
 - If the file is not in Japanese, stop and report that this command is for Japanese
   documents.

+**Lines to check for bold.** Decide once, before any fix, which lines of the target
+this branch added or changed. The bold item of the review applies only to those
+lines, and only in the first review pass.
+
+1. Get the repository top level with `git rev-parse --show-toplevel`, and make the
+   target path relative to it. When the target is outside the top level, the lines
+   cannot be decided.
+2. Get the merge base with `git merge-base origin/main HEAD`. Do not fetch: a
+   caller prepared the branch and has fetched `origin/main`; a standalone run uses
+   the local `origin/main` as it is.
+3. Check whether the path exists at the merge base with
+   `git ls-tree --name-only <base> -- <path>`. Empty output means it does not; one
+   line naming the path means it does.
+4. When the path is absent, run
+   `git diff --name-status --no-renames --no-color <base> --`. When any line starts
+   with `D`, the branch deletes a file, and the target may have been moved there, so
+   the lines cannot be decided. Otherwise treat every line of the document as added.
+5. When the path is present, run
+   `env -u GIT_DIFF_OPTS git --no-pager diff --no-color --no-ext-diff --no-textconv --text --unified=0 --inter-hunk-context=0 <base> -- <path>`,
+   and read the work-tree line ranges from the lines that start with
+   `@@ -a[,b] +c[,d] @@` (ignore the text after the second `@@`). A missing `d`
+   means one line, and `d` equal to 0 means no line, a deletion only. Those are the
+   added and changed lines.
+6. Every `git` of steps 1 to 5 must exit 0. A non-zero exit, or output of another
+   form in steps 3 to 5, means the lines cannot be decided.
+
+Run each `git` of steps 2 to 5 as
+`git -C <top> --literal-pathspecs -c core.quotePath=false ...`. The paths then
+resolve from the top level even when `japrose` is called from a subdirectory, a
+path holding `[` is not read as a pattern, and the output form does not change with
+path quoting.
+
+Decide the lines once, before the fixes: a line that `japrose`'s own expression
+fixes changed is not a line the branch added or changed, so the bold item does not
+run again on a second pass. `git diff` compares with the work tree, so uncommitted
+changes count, and the step works both after the caller committed and when
+`japrose` runs alone. When the local `origin/main` is at `HEAD`, as on `main` after
+a fetch, the merge base is `HEAD`, so only uncommitted changes are added or changed;
+with none, the bold item has no line to check.
+
 ## Load the Glossary

 Read the translation glossary (path in `_context.md`, Process convention). Use it as
@@ -79,7 +121,13 @@ these inputs:
   `02_architecture.md`, `03_implementation_plan.md`) — all as resolved absolute-path
   strings. The sibling documents are **read-only reference** so the reviewer can check
   terminology consistency against them (e.g. a plan vs. its architecture); only the
-  target document is edited.
+  target document is edited. Also give it the formatting convention
+  (`docs/dev/conventions/document_formatting.md`) as a resolved absolute path, and a
+  file in the scratchpad that holds the ranges of the lines the branch added or
+  changed ("Preparation") and the diff hunks of its step 5. The hunks let the
+  reviewer compare a changed line with the line before it and tell whether the
+  change is only a link, a name, a heading, or a typo fix. When the ranges could not
+  be decided, the file says so and gives the reason instead.
 - **CRITERIA**: every item from the Prose-quality checklist and the Hard constraints
   below, copied verbatim.
 - **MODEL**: `sonnet`. This review is a fixed-checklist Japanese prose/terminology
@@ -96,6 +144,7 @@ these inputs:
 - [ ] 同一概念には常に同一の日本語表記が使われ、用語集に一致している。用語集が「旧称」と記す語は使われていない。
 - [ ] 冗長・回りくどい言い回しが、意味を変えずに簡潔にされている。
 - [ ] 図（Mermaid）のラベル・キャプション中の日本語にも上記が適用されている（ノード識別子・コード識別子は除く）。
+- [ ] 確かめる行（「Review」の FILES で渡すファイルにある）の太字が、書式の規約の太字の規則に合っている（太字は、項目や段落の頭のラベルと、読み落とすと誤解を招く語句に限られ、文全体や、文より長いまとまりは太字にしない）。規則の外にある固定のラベルは対象外とし、確かめる行の外の太字と、確かめる行の外までまたがる太字のまとまりは指摘しない。変えた行のうち、変更がリンク・名前・見出し・誤字の直しだけの行の太字も指摘しない。確かめる行を決められないときと、対象がリポジトリの最上位の README（`README.md`・`README.en.md`）のときは、この項目を確かめない。

 **Hard constraints (use verbatim; the reviewer must flag any fix that would violate one of these as out of scope):**
 - [ ] 技術的な内容・事実・数値・固有名・構造・節の順序・見出し・文書ステータスを変更しない。表現だけを直す。
@@ -118,6 +167,16 @@ From the review findings, apply the fixes to the document in place:
   note why.
 - Preserve Mermaid syntax and code fences exactly; re-check that fence counts stay
   balanced after editing diagram labels.
+- For a bold finding, remove only the bold delimiters (`**` or `__`) and keep every
+  character as it is. Do not remove the bold of a fixed label the formatting convention puts outside
+  its rules, and do not touch a bold span that reaches outside the checked lines.
+  Removing the markers is an expression-only fix: do not add subheadings or
+  restructure the document to satisfy the formatting convention.
+- When the lines could not be decided, do not check the bold item. Say in the report
+  that it was not checked and why, and continue the other items. This case reports
+  the reason, not a count.
+- Otherwise, in the report, give the number of lines the bold item checked. When the
+  number is 0, say that there was no line to check.

 If any Critical or Major finding required a fix, run a second review pass to verify
 the fixes (subject to the three-pass limit in the shared pattern), then stop.
diff --git a/CLAUDE.md b/CLAUDE.md
index 69115868f9cb3e1c5b35f90854f42df725be4121..44e9e6b64222aee5cf0e42b87f2c9395022df950 100644
--- a/CLAUDE.md
+++ b/CLAUDE.md
@@ -35,6 +35,10 @@ This file provides guidance to Claude Code (claude.ai/code) when working with co
   [mermaid_reference.md](docs/dev/conventions/mermaid_reference.md) (node-label
   quoting, `<br>` line breaks, cylinder `[(...)]` data nodes, the standard classDef
   palette, and Legend blocks).
+- Japanese documents use bold and structure a section with many items as
+  [document_formatting.md](docs/dev/conventions/document_formatting.md) says
+  (labels and easy-to-miss qualifiers only; a summary first, then subheadings or
+  nested lists).
 - Each task lives in `docs/tasks/NNNN_<name>/` (four-digit sequence number).
   `docs/tasks/0000_template/requirements.md` is the starting point for
   `01_requirements.md`.
diff --git a/docs/dev/adr/0039-adr-writing-criteria.md b/docs/dev/adr/0039-adr-writing-criteria.md
index 963d0c65820d401cd2535537162940360348ac3e..cd978c78bd1ddd03f76f88f58b90e6e7debe7774 100644
--- a/docs/dev/adr/0039-adr-writing-criteria.md
+++ b/docs/dev/adr/0039-adr-writing-criteria.md
@@ -21,7 +21,7 @@ ADR は、後の開発者が、変えようとしている主題の決定とそ
 -   **`/mergepr` で squash マージするときは、PR のコミットにある訂正の行を、タスク文書と ADR のどちらのものも、要約せず 1 行ずつそのまま squash コミットの本文に引き継ぐ。** 行数にも 1 行の長さにも上限を設けない。人は承認のときに訂正の行を消せず、要件定義書が定める場合に限って落とせる。落としたときは、落としたことを本文に記録する。squash マージの後は PR のコミットが `main` からたどれなくなり、PR の説明はリポジトリの外にあって書き換えられるので、訂正の記録を `main` の履歴からたどれるようにするためである。行を受け付ける条件と落とし方の仕様は、[0106 要件定義書](../../tasks/0106_squash_editorial_corrections/01_requirements.md) の「3. 機能要件」にある。今の `/mergepr` の手順と規約は、まだ訂正の行を squash コミットの本文に引き継がない。このずれは、[#747](https://github.com/isseis/yt2column/issues/747) で扱う。
 -   **ずれの記述は、ずれを直す PR が同じ PR で消す。** ずれの記述が事実でない期間を作らないためである。ずれを直す人が記述を見つけられるように、issue から ADR を、ADR から issue をたどれるようにする。ずれの記述を足す PR は、マージを頼む前と `main` を取り込むときに、最新の `main` に照らして、そのずれの記述がまだ事実かどうかを確かめ直す。規則は [adr.md](../conventions/adr.md) の「Writing an ADR」「Editorial corrections」にある。
 -   **`main` の上で `TestADRRecords` が失敗したときの、番号を付け替える直しは、その直しだけの PR で行う。** 番号の付け替えは ADR の識別子を変え、ほかの文書の参照にも及ぶ。関係のない PR のレビュアーは、これを承認する立場にないためである。規則は [adr.md](../conventions/adr.md) の「Taking in `main`」にある。
--   **ADR の書式（太字の使い方と、項目の多い節の組み立て方）は、日本語の文書の書式の規約に従う。既存の ADR を書式の規約に合わせる書き直しは求めないが、編集上の訂正の 1 種目（基準に合わせる書き直し）として、どの PR も行ってよい。** 書式の規則を、ADR に限らず日本語の文書で 1 つにするためである。書き直しを訂正として認めるのは、訂正がコミットと PR の説明に記録され、レビュアーが確かめられるためである。規則は [0111 要件定義書](../../tasks/0111_doc_style_rules/01_requirements.md) の F-001 と F-002 にある。今の [adr.md](../conventions/adr.md) の「Writing an ADR」と「Editorial corrections」は、まだ書式の規約を含まず、書式の規約もまだない。このずれは、[#1011](https://github.com/isseis/yt2column/issues/1011) で扱う。
+-   **ADR の書式（太字の使い方と、項目の多い節の組み立て方）は、日本語の文書の書式の規約に従う。既存の ADR を書式の規約に合わせる書き直しは求めないが、編集上の訂正の 1 種目（基準に合わせる書き直し）として、どの PR も行ってよい。** 書式の規則を、ADR に限らず日本語の文書で 1 つにするためである。書き直しを訂正として認めるのは、訂正がコミットと PR の説明に記録され、レビュアーが確かめられるためである。規則は [書式の規約](../conventions/document_formatting.md) と [adr.md](../conventions/adr.md) の「Writing an ADR」にある。

 ## 採らなかった案

diff --git a/docs/dev/conventions/adr.md b/docs/dev/conventions/adr.md
index 8ca3d754e9a56a38d305d2d12434757c4e80cf76..f681d0e23230697f663240b09e6baa1fab0309ee 100644
--- a/docs/dev/conventions/adr.md
+++ b/docs/dev/conventions/adr.md
@@ -174,6 +174,19 @@ statement of the discrepancy (a discrepancy statement), linked to the issue
 that handles it by the issue's full URL. "Editorial corrections", its
 "Discrepancy statements" paragraph, says who adds and removes one.

+**Format.** An ADR follows the bold and many-item-section rules of the
+Japanese document formatting convention
+([document_formatting.md](document_formatting.md)). The sections that tend to
+hold many items are 決定, 採らなかった案, and 結果; a short section may stay a
+bullet list, as the convention says. The formatting applies to the whole
+section, while the content criteria for what each section holds keep their
+current scope: a PR that changes the decision of an ADR written before these
+criteria brings only the part it changes to the content criteria; the formatting
+still applies to the whole section. Rewriting an existing ADR to the formatting
+convention is not required, but any PR may do it as the first kind of editorial
+correction ("Editorial corrections" below), a rewrite to the criteria, which now
+include this formatting convention.
+
 **Links.** When an ADR states how the code behaves or what a document
 specifies, it links to the document that holds the specification, or to the
 task document that recorded it. Use relative links without line numbers: an
````

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-17 がすべて確かめられている。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   次に日本語の文書を書くタスクで、`japrose` の太字の確認が働くことを観察する。
-   既存の ADR を書式の規約に合わせる書き直しは、編集上の訂正として、#729 などで行える。

## 変更履歴

- 2026-11-19 承認（isseis、#1020）
