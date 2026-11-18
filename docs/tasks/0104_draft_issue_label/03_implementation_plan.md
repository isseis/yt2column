# 実装計画書：`/mkreq` はラベル `draft` が付いた issue からタスクを始めない

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-18 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`/mkreq` が、ラベル `draft` が付いた開いた issue（下書きの issue）から、タスクを始めないようにする（F-001）。あわせて、ラベルを `_context.md` に定義し（F-002）、ADR-0053 の食い違いの記述を消す（AC-09）。ADR-0053 とその索引の行は 3 文書の PR で入るので、本計画では作らない（設計書 3 章）。

### 1.2. 実装の原則

-   **変えるのは、`mkreq.md` の step 1、`_context.md` の 1 行、ADR-0053 の食い違いの記述、本計画だけである（AC-10）。**
-   **Go のコードとテストは変えない。** 文の中身を確かめる文書テストは足さない（設計書 7 章、CLAUDE.md「Document tests check values, structure, and executable examples, never prose」）。
-   **`mkreq.md` と `_context.md` は英語で書く**（CLAUDE.md「Document language」の原則 (b)）。

### 1.3. 計画の際に決めたこと（2026-11-18）

-   **AC-05 は、実装の後に、同じセッションで `/mkreq 928` を実際に走らせて確かめ、結果を §7 に記録する。** 指示文が実際の run で止まることを見るためである。走らせた run が step 1 で止まらなかったときは、step 2 に進まずに打ち切る。こうすれば、`numreserve` は走らず、記録の tag もブランチもできない。人の答え：`/mkreq 928` を実際に走らせる。
    -   ADR：なし。本タスクの確かめ方の選択で、本タスクの中で閉じる。
-   フェーズは 1 つで、実装の PR は 1 本である。設計書が開いたままにした計画の選択は、上のほかにない。

### 1.4. 既存コード調査結果

コミット `bce12d7c1e9e0b3c9944a84a3384c97dc418ccfd`（`origin/main`）で確かめた。

-   `mkreq.md` の step 1 は `.claude/commands/mkreq.md:46-66` にある。issue を読む `gh issue view` は `:48`、閉じた issue で止まる文は `:49`、`url` の確かめは `:49-52` にあり、その後に `git fetch origin` と重複の検索が続く。
-   `mkreq.md` の「Authorization」（`:34-42`）は、`gh issue view` をすでに承認している。新しい承認は要らない。
-   `_context.md` の「Process convention」の表は `.claude/commands/_context.md:35-55` にあり、`Task process record` の行は `:49` にある。この表を読む Go のコードはない。`internal/chief/routing.go` が読むのは「Command routing」の表である。
-   chief の判定は `internal/chief/issue.go:58-60`（`draftLabel`）と `:239-246`（閉じた issue を先に見る）にある。変えない。
-   ラベル `draft` が付いた開いた issue に #928 がある（`gh issue view 928 --json labels,state`、2026-11-18 に確かめた。ステップ 1-4 の前に確かめ直す）。
-   `mkreq.md` と `_context.md` の文を確かめる Go のテストのうち、本タスクの変更に関わるものはない。`cmd/yt2column/fixpr_docs_test.go` と `cmd/numreserve/docs_test.go` は `mkreq.md` を読むが、読むのは `fixprguard` と `numreserve` の呼び出しの形である。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: `/mkreq` の下書きの issue の確かめ（F-001〜F-004）

3 文書の PR のマージの後に `/runplan` で行う。

**変更するファイル:** `.claude/commands/mkreq.md`、`.claude/commands/_context.md`、`docs/dev/adr/0053-draft-issue-label.md`、本計画

-   [ ] **ステップ 1-1**: `_context.md` の「Process convention」の表の、`Task process record` の行の後に、`Draft issue label` の行を 1 つ足す。内容は設計書 3.2 のとおりである（AC-06）。
-   [ ] **ステップ 1-2**: `mkreq.md` の step 1 の、`url` の確かめの後、`git fetch origin` の前に、下書きの issue の確かめを足す。手順・終了コードごとの振る舞い・報告は設計書 3.1 と 4 章のとおりである。ラベルの定義は `_context.md` の `Draft issue label` の行を名前で参照し、ラベルの意味を `mkreq.md` に書き写さない（AC-01〜AC-04）。
-   [ ] **ステップ 1-3**: ADR-0053 の「決定」から、issue #934 にリンクした食い違いの記述を消す。消す前に `git grep -nE 'issues/934([^0-9]|$)' origin/main -- docs/dev/adr/` を走らせ、当たったものが ADR-0053 のその 1 項目だけであることを確かめる（AC-09、`docs/dev/conventions/adr.md`「Discrepancy statements」）。
-   [ ] **ステップ 1-4**: ステップ 1-1〜1-3 をコミットした後、作業ツリーがきれいな状態で、同じセッションから `/mkreq 928` を走らせる。step 1 で止まり、下書きの issue であることと、ラベルを外せば正式になることを報告することを確かめる（AC-05）。走らせた run が step 1 で止まらなかったら、step 2 に進まずに打ち切り、ステップ 1-2 を直してやり直す。走らせる前と止まった後で、`git status --short` が空であること、`git branch --list` の一覧と `git ls-remote origin 'refs/tags/task-number/*'` の一覧が変わらないことを確かめる。結果を §7 の実行の記録に書く。
-   [ ] **ステップ 1-5**: 差分の範囲を、記録した diff の読み取りで確かめる（AC-10）。比べるのは、マージベース（`git merge-base origin/main HEAD`）と、ステップ 1-4 の記録を入れる前の最後のコミットである。読むのは、`git diff --name-status <base> <head>` の一覧と、`.claude/commands/mkreq.md`・`.claude/commands/_context.md`・`docs/dev/adr/0053-draft-issue-label.md` の差分である。本計画は記録を持つので、比べる対象に含めない（`docs/dev/conventions/requirements_process.md`「`diff` verification」）。結果と 2 つのコミットの完全な ID を §7 に書き、ステップ 1-4 の記録とともにコミットする。
-   [ ] **ステップ 1-6**: green gate（`_context.md`）を通す。

**完了の条件:** `mkreq.md`・`_context.md`・ADR-0053 の変更がコミットされている。§7 に AC-05 の実行の記録と AC-10 の diff の読み取りの記録がある。green gate が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `mkreq.md` の step 1 の確かめ、`_context.md` の行、ADR-0053 の食い違いの記述の削除、§7 の記録 |

### 3.2 PR 構成

フェーズは 1 つなので、PR の作成ポイントは置かない。実装の PR は 1 本で、フェーズ 1 の完了の後に `/runplan` が開く。

**実装モデル要件**: sonnet

**判定理由**: 指示文 2 つへの小さな追記と ADR の 1 項目の削除だけで、どの条件にも当たらない

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** N/A。Go のコードを変えない。
-   **実際に走らせる確かめ:** ステップ 1-4 で `/mkreq 928` を 1 回走らせる（AC-05）。
-   **既存のテスト:** `TestADRRecords` と `TestInstructionDocPaths`（`cmd/yt2column`）が、ADR-0053 の形とリンク先を確かめる。green gate に含まれる。
-   **読んで確かめる:** `mkreq.md` と `_context.md` の文の中身（AC-01〜AC-04・AC-06）と ADR-0053 の「決定」の内容（AC-08）は、PR のレビューで読む。
-   **後方互換のテスト:** N/A。ラベル `draft` の付いていない issue での振る舞いは変わらない（要件書 4.4）。そのような issue で `/mkreq` を走らせるとタスク番号が決まるので、確かめのためには走らせない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| ステップ 1-4 の `/mkreq 928` が step 1 で止まらず、タスク番号を決める | step 1 で止まらなかったら step 2 に進まずに打ち切る（ステップ 1-4）。万一番号が決まっても欠番は誤りではない（ADR-0051） |
| ステップ 1-4 の前に、#928 のラベル `draft` が外される | ステップ 1-4 の前に `gh issue view 928 --json labels,state` で確かめ直す。外れていたら、ラベル `draft` が付いたほかの開いた issue を使い、§7 にその番号を書く |
| 食い違いの記述が、本計画の外の ADR にもある | ステップ 1-3 の `git grep` で当たったものを読み、ADR-0053 のほかに当たれば止まって人に尋ねる |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1（ステップ 1-1〜1-6）
-   [ ] PR を作成した
-   [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | 実装の PR のレビューで、`mkreq.md` の step 1 が、確かめの位置（閉じているかと `url` の確かめの後、`git fetch origin` の前）、下書きの issue で止まること、報告の内容、`[task]` と止まった run の続きでも止まることを述べるかを読む |
| AC-02 | `manual` | 実装の PR のレビューで、`mkreq.md` の step 1 が `grep -qxF -- draft` の終了コードで決め、名前がちょうど `draft` のラベルだけを印とすることを読む |
| AC-03 | `manual` | 実装の PR のレビューで、確かめが閉じた issue の確かめの後にあり、閉じた issue では今までどおり止まることを読む |
| AC-04 | `manual` | 実装の PR のレビューで、ラベルを読めないときと `grep` が `0`・`1` のほかで終わるときに止まることを読む |
| AC-05 | `manual` | ステップ 1-4 で `/mkreq 928` を走らせ、結果を下の実行の記録に書く |
| AC-06 | `manual` | 実装の PR のレビューで、`_context.md` の `Draft issue label` の行が要件書 AC-06 の 4 点を述べるかを読む |
| AC-07 | `test` | `cmd/yt2column/adr_test.go::TestADRRecords`、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（green gate） |
| AC-08 | `manual` | 3 文書の PR のレビューで、ADR-0053 の「決定」が要件書 AC-08 の項目を述べ、`_context.md` にリンクするかを読む |
| AC-09 | `diff` | ステップ 1-5 の diff の読み取りで、ADR-0053 の差分が食い違いの記述の 1 行の削除だけであることを確かめ、下の記録に書く |
| AC-10 | `diff` | ステップ 1-5 の diff の読み取りで確かめ、下の記録に書く |

実行の記録（ステップ 1-4・1-5）：実装のときに書く。

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-10 の検証がすべて済み、§7 に記録がある。
-   green gate が通る。

## 9. 次のステップ (Next Steps)

-   実装の PR のマージの後、`/runpr` が issue #934 を閉じる。
