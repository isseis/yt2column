# 実装計画書：開発の道具の規則を ADR-0014 から規約 dev_tools.md に移す

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

英語の規約 `docs/dev/conventions/dev_tools.md` を作り、ADR-0014 の規則 1〜6 の項目を要約と規約へのリンクに書き直す。`CLAUDE.md` に 2 項目を足し、ADR-0014 を規則の置き場所として指している 3 か所を、規約を指すように直す（設計書 3 章）。

### 1.2. 実装の原則

-   **規則の内容を変えない。** 規約は、ADR-0014 の今の規則と、本 PR が ADR-0014 に足した規則 5 の広げ（要件書 AC-07）を、英語で書き移す。新しい規則も理由も足さない。
-   **ADR-0014 は編集上の訂正だけで変える。** 「経緯」に項目を足さない。外す本文はどれも、規約かほかの正本に同じ意味で書かれていることを確かめてから外す（設計書 3.3 節）。
-   **道具の名前を規約に並べない**（AC-02）。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   **実装モデル要件は `sonnet` とする。** 2026-11-19 に人が選んだ。`mkplan2.md` の基準のどのトリガーにも当たらないためである。変更は文書だけで、方式の選択も危険度の高いステップもない。懸念は、日本語の規則を英語に移すときの意味のずれと、編集上の訂正の分類の誤りである。この 2 つは、AC の `manual` の読みと、ADR-0014 の diff の記録（AC-09）を PR のレビューで読んで捉える。
    -   採らなかった案：`sonnet+opus-review`。基準のトリガーに当たらないので、上乗せになる。
    -   ADR: なし。本タスクの中で閉じる。
-   **差分の範囲（AC-09・AC-14・AC-17・AC-18）は、スクリプトではなく、記録した diff を読んで確かめる。** 変わる文書は小さく（ADR-0014 の 6 項目、`CLAUDE.md` の 2 項目、ガイドの 2 か所）、スクリプトを書くより diff を読むほうが手間が少ないためである（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の「Proportion」）。
    -   ADR: なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `a73b2c4e`（`origin/main`）で確かめた。行番号のうち ADR-0014 のものは、本 PR が `:22` に文を足し、「経緯」に 1 行を足した後も変わらない。

-   ADR-0014 の「決定」は `docs/dev/adr/0014-deterministic-checks-in-go.md:17-25` の 9 項目である。規則 1〜6 は `:17`・`:18`・`:20`・`:24`・`:22`・`:25` に当たる（設計書 3.3 節の表）。変えない 3 項目は `:19`・`:21`・`:23` である。
-   `CLAUDE.md` の「Development Guides」は `CLAUDE.md:15-18`、「Build Commands」の `make install-*` は `CLAUDE.md:181-185`、「Tool Execution Safety」は `CLAUDE.md:990` から始まる。
-   参照の付け替えの 3 か所は、`docs/dev/guides/chief_guide.md:22`、`docs/dev/guides/numreserve_guide.md:11`、`.claude/commands/_lib/adr-correction-candidates.sh:62-64` である。`git grep -n '0014-deterministic\|ADR-0014'` を `docs/tasks` の外で走らせて確かめた。ほかに当たるのは ADR（ADR-0013・ADR-0015・ADR-0037・ADR-0046・ADR-0051・ADR-0053）と、`docs/dev/adr/README.md` の索引の行、`internal/chief/testdata/` と `internal/numreserve/testdata/` のサンプルである。ADR は ADR-0014 の決定を指していて、規則の置き場所を指してはいない。索引の行とサンプルは、変える対象ではない（要件書 2.3 節）。
-   既存のガード：`TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`）は、`CLAUDE.md` と `docs/dev/` の下の文書のリンクが解決することを確かめる。`TestADRRecords`（`cmd/yt2column/adr_test.go:425`）は ADR の形を確かめる。`TestADRCorrectionCandidates`（`cmd/yt2column/adr_candidates_docs_test.go:367`）は `adr-correction-candidates.sh` の動作を確かめる。`TestDocumentedInvocations`（`cmd/numreserve/docs_test.go`）は「Tool Execution Safety」の節にある `numreserve` の呼び出しの形を確かめる。
-   新しいコードとテストのヘルパーは要らない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 規約、ADR-0014、CLAUDE.md、参照（設計書 3 章、AC-01〜AC-18）

#### ステップ 1-1: 規約 dev_tools.md

**変えるファイル:** `docs/dev/conventions/dev_tools.md`（新規）

- [ ] 設計書 3.2 節の見出しと内容で、英語の規約を書く（AC-01〜AC-06）。規則 1〜6 の内容は、ADR-0014 の `:17`・`:18`・`:20`・`:22`・`:24`・`:25` と要件書 1 章の規則 1〜6 から写し、意味を変えない
- [ ] 道具の名前を一覧として並べず、`CLAUDE.md` の「Build Commands」へリンクする（AC-02）。chief の例外の範囲を書かない（AC-03）。理由を書かない（AC-05）

**完了の条件:** 規約があり、`go test -tags test ./cmd/yt2column -run TestInstructionDocPaths` が通る。

#### ステップ 1-2: ADR-0014 の書き直し

**変えるファイル:** `docs/dev/adr/0014-deterministic-checks-in-go.md`

- [ ] 「決定」の規則 1〜6 の 6 項目を、設計書 3.3 節の表のとおり、要約・短い理由・規約の節への参照に書き直す（AC-08）。見出しの名前は、ステップ 1-1 で書いた規約の見出しと揃える
- [ ] 外す本文ごとに、それが規約かほかの正本（`CLAUDE.md`、ガイド）に同じ意味で書かれていることを確かめる。どこにもない本文は外さない（設計書 3.3 節）
- [ ] `:19`・`:21`・`:23` の 3 項目と、「背景」「採らなかった案」「結果」「経緯」は変えない（AC-09）
- [ ] コミットメッセージに、項目ごとの訂正行（`Editorial correction: docs/dev/adr/0014-deterministic-checks-in-go.md: ...; no decision changed`）を書く（AC-10）

**完了の条件:** `go test -tags test ./cmd/yt2column -run 'TestADRRecords|TestInstructionDocPaths'` が通り、コミットメッセージに訂正行がある。

#### ステップ 1-3: CLAUDE.md

**変えるファイル:** `CLAUDE.md`

- [ ] 「Development Guides」の `Package Reference` の行の後に、規約へのリンクの 1 行を足す（設計書 3.4 節、AC-12）
- [ ] 「Tool Execution Safety」の `codexreview` の項目の前に、設計書 3.4 節の 1 項目を足す。`numreserve` の語は書かない（AC-13）

**完了の条件:** `go test -tags test ./cmd/yt2column ./cmd/numreserve` が通る。

#### ステップ 1-4: 参照の付け替え

**変えるファイル:** `docs/dev/guides/chief_guide.md`、`docs/dev/guides/numreserve_guide.md`、`.claude/commands/_lib/adr-correction-candidates.sh`

- [ ] 2 つのガイドの「（ADR-0014 の『道具は `main` からインストールして使う』）」を、規約の `Install from main` の節への参照に替える（設計書 3.5 節、AC-15）
- [ ] スクリプトのコメントの `docs/dev/adr/0014-deterministic-checks-in-go.md` を `docs/dev/conventions/dev_tools.md` に替え、折り返しを整える。コードは変えない（AC-16）

**完了の条件:** `go test -tags test ./cmd/yt2column -run 'TestADRCorrectionCandidates|TestInstructionDocPaths'` が通る。

#### ステップ 1-5: 仕上げ

- [ ] `make test` と `make lint` を通す（AC-11）
- [ ] 7 章の diff の記録（AC-09・AC-14・AC-17・AC-18）を書く

**完了の条件:** グリーンゲートが通り、diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 規約、ADR-0014 の書き直し、`CLAUDE.md` の 2 項目、参照の付け替え、diff の記録 |

ステップ 1-1 を先に行う。ステップ 1-2 と 1-4 は、規約の見出しの名前を参照するためである。

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-5 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（文書だけの変更で、方式の選択も危険度の高いステップもない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   新しいテストは足さない（設計書 7 章）。規約は、コードと照合する値を持たない。
-   既存のガードが次を確かめる。リンクの解決は `TestInstructionDocPaths`、ADR の形は `TestADRRecords`、スクリプトの動作は `TestADRCorrectionCandidates`、「Tool Execution Safety」の `numreserve` の呼び出しの形は `TestDocumentedInvocations` が受け持つ。どれも `make test` と CI の `doc-guard` ジョブで走る。
-   規約の文、ADR-0014 の各項目、`CLAUDE.md` の 2 項目は、PR のレビューで読む（`manual`）。
-   差分の範囲は、記録した diff を読む（`diff`、7 章）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 規則を英語に移すときに意味がずれる（範囲が広がる、条件が落ちる） | ステップ 1-1 で ADR-0014 の各項目から写す。PR のレビューで、規約と ADR-0014 の今の項目を並べて読む（AC-01〜AC-06 の `manual`） |
| ADR-0014 から外した本文が、規約にもほかの正本にもなくなる | ステップ 1-2 で外す本文ごとに置き場所を確かめ、ない本文は外さない。AC-09 の diff の記録を PR のレビューで読む |
| 編集上の訂正の分類を誤り、決定の変更を訂正として扱う | `docs/dev/conventions/adr.md`「Editorial corrections」のとおり、迷ったら決定の変更として扱う。レビューアは PR の説明の訂正の段落で分類に異を唱えられる |
| 規約の見出しを変えたのに、ADR-0014 やガイドの参照が古い名前のまま残る | ステップ 1-1 を先に行い、1-2 と 1-4 で見出しの名前を揃える。見出しの名前は本文で示すので、`TestInstructionDocPaths` は確かめない。PR のレビューで読む |
| 「Tool Execution Safety」に足す項目が `numreserve` の呼び出しの形と見なされ、`TestDocumentedInvocations` が失敗する | 項目に `numreserve` の語を書かない（ステップ 1-3） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-5）
- [ ] PR を作成した
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` / `static` | PR のレビューで、規約が英語で規則 1〜6 を節として持つことを読む。規約のリンクの解決は `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` |
| AC-02 | `manual` | PR のレビューで、規約が道具の名前を一覧として並べず、`CLAUDE.md` の「Build Commands」へリンクすることを読む |
| AC-03 | `manual` | PR のレビューで、規則 2 の節が chief の例外の範囲を書かず、ADR-0014 と ADR-0046 へリンクすることを読む |
| AC-04 | `manual` | PR のレビューで、規則 5 の節が要件書 AC-04 の 3 点を書き、`CLAUDE.md`「Tool Execution Safety」へリンクすることを読む |
| AC-05 | `manual` | PR のレビューで、規約が理由を書かず、ADR-0014 へリンクすることを読む |
| AC-06 | `manual` | PR のレビューで、規約がプロトコル番号の規則を書かず、`CLAUDE.md`「Protocol numbers」へリンクすることを読む |
| AC-07 | `manual` | 本 PR（3 文書の PR）のレビューで、ADR-0014 の `:22` の項目と「経緯」の最後の項目を読む |
| AC-08 | `manual` | PR のレビューで、ADR-0014 の規則 1〜6 の項目が要約・理由・規約の節への参照だけを持つことを読む |
| AC-09 | `diff` | 記録した diff を読む。下の Diff record (AC-09) |
| AC-10 | `manual` | PR のレビューで、ステップ 1-2 のコミットの訂正行と、PR の説明の訂正の段落を読む |
| AC-11 | `test` | `cmd/yt2column/adr_test.go::TestADRRecords`（`make test`） |
| AC-12 | `manual` / `static` | PR のレビューで、「Development Guides」の 1 行を読む。リンクの解決は `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` |
| AC-13 | `manual` / `test` | PR のレビューで、「Tool Execution Safety」の 1 項目を読む。節の `numreserve` の呼び出しの形は `cmd/numreserve/docs_test.go::TestDocumentedInvocations` |
| AC-14 | `diff` | 記録した diff を読む。下の Diff record (AC-14) |
| AC-15 | `manual` / `static` | PR のレビューで、2 つのガイドの参照を読む。リンクの解決は `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` |
| AC-16 | `test` / `diff` | `cmd/yt2column/adr_candidates_docs_test.go::TestADRCorrectionCandidates`（`make test`）。コメントだけの変更であることは Diff record (AC-17) の差分で読む |
| AC-17 | `diff` | 記録した diff を読む。下の Diff record (AC-17) |
| AC-18 | `diff` | 記録した diff を読む。下の Diff record (AC-18) |

Diff record (AC-09):

- Compared paths: `docs/dev/adr/0014-deterministic-checks-in-go.md`

Diff record (AC-14):

- Compared paths: `CLAUDE.md`

Diff record (AC-17):

- Compared paths: `.claude/commands/_lib/adr-correction-candidates.sh`

Diff record (AC-18):

- Compared paths: `docs/dev/guides/chief_guide.md` `docs/dev/guides/numreserve_guide.md`

## 8. 成功の基準 (Success Criteria)

-   7 章のすべての AC の検証が通り、グリーンゲートが通る。

## 9. 次のステップ (Next Steps)

-   3 つの文書が承認されたら、`/runplan 0112` でフェーズ 1 を実装し、PR を開く。
