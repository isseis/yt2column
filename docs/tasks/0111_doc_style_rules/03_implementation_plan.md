# 実装計画書：日本語の文書の太字を絞り、項目の多い節はまとめを先に書いて構造化する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

書式の規約 `docs/dev/conventions/document_formatting.md` を作り、`CLAUDE.md` と `adr.md` からリンクし、`japrose` に太字の確認を足す（設計書 3.1〜3.4 節）。

### 1.2. 実装の原則

-   規約とコマンドの定義は英語で書く（`CLAUDE.md`「Document language」の原則 (b)）。`japrose` の基準の項目は、今の基準と同じく日本語で書く。
-   本タスクで新しく書く文は、書式の規約に従う。既存の文の、変えない部分は直さない（AC-02・AC-13）。

### 1.3. 計画の際に決めたこと（2026-11-19）

計画で新たに人に確かめた判断はない。フェーズと PR を 1 つにすることと、変更の範囲を記録した diff で確かめることは、設計書の 7 章と 8 章にあるとおりで、選ぶ余地のある判断ではない。

### 1.4. 既存コード調査結果

コミット `21aef492`（`origin/main`）で確かめた。

-   `CLAUDE.md` の「Documents」は `CLAUDE.md:27-41` にあり、Mermaid の項目は `:34-37` にある。
-   `adr.md` の「Writing an ADR」は `docs/dev/conventions/adr.md:128` から始まる。「What 決定 holds」は `:163-175`、「Links」は `:177` にある。`TestADRRecords` が読む「Template」「Status」「Index」の節は変えない。
-   `japrose.md` の「Purpose」は `.claude/commands/japrose.md:14-36`、「Preparation」は `:38-54`、「Review」の FILES は `:76-82`、Prose-quality checklist は `:90-98`、Hard constraints は `:100-104`、「Apply Fixes」は `:106-123` にある。
-   `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`）は、`CLAUDE.md` と `.claude/commands/`・`docs/dev/` の下の文書のリンクの先があることを確かめる。新しい規約も対象になる。
-   `docs/dev/conventions/` の下の文書の一覧を持つガードやテストはない。`docs/dev/conventions/package_reference.md` はパッケージの一覧で、規約の一覧ではない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 規約の新設と参照、japrose の太字の確認（設計書 3 章、AC-01〜AC-14）

#### ステップ 1-1: 書式の規約

**変えるファイル:** `docs/dev/conventions/document_formatting.md`（新規）

- [ ] 設計書 3.1 節のとおり、対象の文書、当てる範囲、太字の規則、項目の多い節の規則を英語で書く

**完了の条件:** 規約が AC-01〜AC-05 の内容を述べる。

#### ステップ 1-2: `CLAUDE.md` と `adr.md` からの参照

**変えるファイル:** `CLAUDE.md`、`docs/dev/conventions/adr.md`

- [ ] 設計書 3.2 節のとおり、`CLAUDE.md` の「Documents」の Mermaid の項目の次に、規約へリンクする 1 項目を足す
- [ ] 設計書 3.3 節のとおり、`adr.md` の「What 決定 holds」と「Links」の間に、規約へリンクし、「決定」「採らなかった案」「結果」を挙げ、短い ADR の扱いを述べる 1 段落を足す

**完了の条件:** `CLAUDE.md` が AC-06 の、`adr.md` が AC-07 の内容を述べる。

#### ステップ 1-3: `japrose` の太字の確認

**変えるファイル:** `.claude/commands/japrose.md`

- [ ] 設計書 3.4.1 節のとおり、「Preparation」に、確かめる行を決める手順と、決められないときの扱いを足す
- [ ] 設計書 3.4.2 節のとおり、「Review」の FILES に、確かめる行の範囲を書いたファイルを足す
- [ ] 設計書 3.4.3 節のとおり、Prose-quality checklist に太字の項目を日本語で 1 つ足し、「Purpose」の問題の一覧に太字の使いすぎを足す
- [ ] 設計書 3.4.4 節のとおり、「Apply Fixes」に、太字の直し方と、確かめなかったときの報告を足す

**完了の条件:** `japrose.md` が AC-08〜AC-11 の内容を述べる。

#### ステップ 1-4: 仕上げ

- [ ] `go test -tags test ./cmd/yt2column` を通す（AC-14）
- [ ] `make test` と `make lint` を通す
- [ ] 7 章の AC-12・AC-13 の diff の記録を書く

**完了の条件:** テストが通り、diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 書式の規約、`CLAUDE.md` と `adr.md` からの参照、`japrose` の太字の確認、diff の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-4 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（文書と 1 つのコマンドの定義の追記で、方式の選択も危険の高いステップもない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   新しいテストは足さない。変わるのは文書と、コマンドの定義の文だけで、文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。
-   既存のガード（`go test -tags test ./cmd/yt2column`）が、リンクの先があることと ADR の記録の形を確かめる（AC-14）。
-   変更の範囲は、記録した diff の読み取りで確かめる（AC-12・AC-13）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| `japrose` が確かめる行の外の太字まで外す | 基準の項目と「Apply Fixes」の両方に、確かめる行の外を変えないことを書く（設計書 3.4.3・3.4.4 節）。PR のレビューで読む |
| `japrose` が固定のラベルの太字を外し、道具が値を読めなくなる | 基準の項目と「Apply Fixes」に、固定のラベルを対象外とすることを書く。固定のラベルの例を規約に挙げる |
| 規約と `adr.md` の段落が同じ規則を重ねて書き、後で食い違う | `adr.md` にはリンクと当てはめだけを書く（設計書 1 章の設計原則）。PR のレビューで読む |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-4）
- [ ] PR を作成した
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR のレビューで、規約があり英語で書かれ、対象が原則 (a) の日本語の文書で英語の文書は対象でないと述べることを読んで確かめる |
| AC-02 | `manual` | 同じく、規約が、新しく書く文書と変更が足すか変える部分に当て、変えない部分の書き直しを求めないと述べることを読んで確かめる |
| AC-03 | `manual` | 同じく、規約が、太字にしてよいもの・しないもの・規則の外の固定のラベルを述べることを読んで確かめる |
| AC-04 | `manual` | 同じく、規約が、まとめを先に書くこと、小見出しか入れ子の箇条書きで構造化すること、項目の中の形を書き手に任せることを述べることを読んで確かめる |
| AC-05 | `manual` | 同じく、規約が、短い節の扱い（閾値なしで書き手が判断）と、テンプレートが決める形の扱いを述べることを読んで確かめる |
| AC-06 | `manual` | 同じく、`CLAUDE.md` の「Documents」に規約へリンクする項目があることを読んで確かめる。リンクの先があることは AC-14 が確かめる |
| AC-07 | `manual` | 同じく、`adr.md` の「Writing an ADR」が規約へリンクし、3 つの節を挙げ、短い ADR の扱いを述べることを読んで確かめる |
| AC-08 | `manual` | 同じく、`japrose.md` の Prose-quality checklist に太字の項目が 1 つあることを読んで確かめる |
| AC-09 | `manual` | 同じく、`japrose.md` が、確かめる行を merge-base との差分から決め、merge-base にない文書はすべての行とし、それ以外の行の太字を指摘も変更もしないと述べることを読んで確かめる |
| AC-10 | `manual` | 同じく、`japrose.md` が、確かめる行を決められないときは太字の項目を確かめず、報告に書き、ほかの項目を続けると述べることを読んで確かめる |
| AC-11 | `manual` | 同じく、`japrose.md` が、太字の印だけを外して文字を変えず、固定のラベルの太字を外さないと述べることを読んで確かめる |
| AC-12 | `diff` | 下の記録のファイルの一覧を、PR のレビューで読む（記録した diff の読み取り） |
| AC-13 | `diff` | 下の記録の 3 つのファイルの差分を、PR のレビューで読む（記録した diff の読み取り） |
| AC-14 | `test` | `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` と `cmd/yt2column/adr_test.go::TestADRRecords`（`go test -tags test ./cmd/yt2column`） |

AC-12 の記録はファイルの一覧を持ち、AC-12 はその一覧で確かめる。比べるパスは、本計画を含まない 4 つのファイルとする。

Diff record (AC-12):

- Compared paths: `docs/dev/conventions/document_formatting.md` `CLAUDE.md` `docs/dev/conventions/adr.md` `.claude/commands/japrose.md`

Diff record (AC-13):

- Compared paths: `CLAUDE.md` `docs/dev/conventions/adr.md` `.claude/commands/japrose.md`

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-14 がすべて確かめられている。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   次に日本語の文書を書くタスクで、`japrose` の太字の確認が働くことを観察する。
-   既存の文書を書式の規約に合わせる必要が出たら、別の issue にする。
