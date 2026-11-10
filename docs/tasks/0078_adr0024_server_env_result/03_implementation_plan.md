# 実装計画書：ADR-0024「結果」の、サーバが環境から読む変数の記述の更新

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-10 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

ADR-0024 の「結果」の最後の項目を、ADR-0018 の決定が実装された今の状態を述べる文に書き直し（F-001）、その直しを編集上の訂正として記録する（F-002）。

### 1.2. 実装の原則

-   **変えるのは `docs/dev/adr/0024-env-access-in-config.md` の 1 行と、本タスクのディレクトリの下だけである。** コード・ほかの ADR・ADR の索引は変えない（AC-02・AC-03）。
-   **新しい恒久のテストは足さない**（設計書 7 章）。

### 1.3. 計画の際に決めたこと（2026-11-10）

-   設計書が開いたままにした計画の選択はない。フェーズは 1 つで、実装の PR は 1 本である。そのため PR の作成ポイントは置かない。ADR は作らない（本タスクの中で閉じる計画の形である）。

### 1.4. 既存コード調査結果

コミット `3747237fdcb051c436e0487f017c2f9e3a3c4ac7`（`origin/main`）で確かめた。

-   書き直す項目は `docs/dev/adr/0024-env-access-in-config.md:35` にある。「結果」の 4 項目のうち最後のものである。
-   `docs/dev/adr/` の中で「まだコードに」「実装されると」を含む行は、この 1 行だけである（`git grep -n "まだコードに\|実装されると" origin/main -- docs/dev/adr/` の出力は 1 行）。
-   `origin/main` の ADR に、issue #776 にリンクしたずれの記述はない（`git grep -nE 'issues/776([^0-9]|$)' origin/main -- docs/dev/adr/` の出力は空）。
-   この項目の文を確かめる既存のテストはない。コードの変更は要らない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: ADR-0024 の書き直しと確かめ（設計書 3 章・7 章、AC-01〜AC-05）

**変更するファイル:** `docs/dev/adr/0024-env-access-in-config.md`、`docs/tasks/0078_adr0024_server_env_result/_checks/main.go`（新規）、本計画（§7 の記録）

-   [ ] **ステップ 1-1**: ADR-0024 の「結果」の最後の項目を、次のとおり書き直す（AC-01）。言い回しは、設計書 3 章の 3 つの点を満たす範囲で、実装のときに整えてよい。
    -   変更の前：`-   [ADR-0018](0018-server-config-file.md) は、サーバが設定ファイルだけから設定を読み、環境からは `HOME` と `GODEBUG` だけを読むと決めている。この決定はまだコードに入っていない。実装されると、サーバが環境から読むのはその 2 つだけになる。`
    -   変更の後：`-   [ADR-0018](0018-server-config-file.md) の決定により、サーバは設定を設定ファイルだけから読み、サーバのコードがプロセスの環境から読むのは `HOME` と `GODEBUG` だけである（[security.md](../security.md) §8.2）。`

    このコミットのメッセージに、ADR-0024 を名指す訂正の行を 1 行書く（AC-04）。形は `docs/dev/conventions/requirements_process.md`「Recording in the PR description」の訂正の行に従い、何を何に直したかと、決定が変わらないことを書く。このコミットには ADR-0024 のほかの変更を入れない。
-   [ ] **ステップ 1-2**: 確認のスクリプトを `_checks/main.go` に書く（AC-02・AC-03・AC-04）。コミットを 2 つ（マージベースと head）引数で受け取り、作業ツリーを読まずに、次を確かめる。
    -   ADR-0024 の 2 つの版で、「結果」の最後の項目の行を除いたほかの行が、同じ順序で同じである。「結果」の項目の数が同じである（AC-02）。
    -   `docs/dev/adr/` の下で 2 つのコミットの間に変わったファイルが、ADR-0024 だけである（AC-03）。
    -   2 つのコミットの間のコミットのメッセージに、行頭が `Editorial correction: docs/dev/adr/0024-env-access-in-config.md: ` の行がちょうど 1 行ある（AC-04）。

    違反では 0 でない終了コードで終わり、違反の内容と、確かめた 2 つのコミットの完全な ID を出力する。コメント・識別子・文字列リテラルは英語で書く。ただし、「結果」の見出しを探す文字列は日本語の文書に合わせる必要がある。`_checks/` は guard-test literal exception（`CLAUDE.md`「Go Idioms」）の対象ではないので、タスク 0071 と同じく `_checks/data.json` に置くか、Go のソースで `\u` のエスケープで書く。どちらにするかは実装で決める。`make fmt` と `go vet -tags test ./docs/tasks/0078_adr0024_server_env_result/_checks/` を通してからコミットする。
-   [ ] **ステップ 1-3**: スクリプトが狙った理由で失敗できることを確かめる。使い捨てのブランチで、次の壊し方を 1 件ずつ一時的なコミットにし、そのコミットを head として走らせて失敗することを確かめる。
    -   AC-02：ADR-0024 の「決定」の 1 行を変える。別のコミットで、「経緯」に項目を 1 つ足す。
    -   AC-03：ほかの ADR の 1 行を変える。
    -   AC-04：訂正の行のないメッセージで ADR-0024 を直したブランチを作る。

    使い捨てのブランチは push せず、確かめた後に消す。壊し方ごとの結果は、ステップ 1-4 のコミットのメッセージに書く。
-   [ ] **ステップ 1-4**: PR の head で確認のスクリプトを実行し、出力・終了コード・2 つのコミットの完全な ID を本計画 §7 に書いて、本計画の進み具合とともにコミットする。`make test` と `make lint` を通す。

**完了の条件:** 確認のスクリプトが終了コード 0 で終わり、その記録が本計画 §7 にある。`make test` と `make lint` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 書き直した ADR-0024、`_checks/main.go`、§7 の実行の記録 |

PR は 1 本である。`/runplan` が PR を作るとき、訂正の段落を、ブランチのコミットの訂正の行から PR の説明に書く（`.claude/commands/_lib/task-pr-handoff.md` Part B step 3。AC-05）。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** N/A。コードを変えない。
-   **確認のスクリプト:** `_checks/main.go` を PR の head で 1 回実行する（AC-02・AC-03・AC-04）。`make test` と CI は実行しない。
-   **読んで確かめる:** 書き直した文の内容（AC-01）と PR の説明の訂正の段落（AC-05）は、PR のレビューで読む。
-   **後方互換のテスト:** N/A。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| PR のレビュアーが、編集上の訂正という分類に異を唱える | 規約に従い決定の変更として扱い、ADR-0024 の「経緯」に項目を足し、要件書の決定を改める（`docs/dev/conventions/adr.md`「Editorial corrections」の「Classifying a change」）。その場合 AC-02 の「経緯に項目が足されない」と AC-04 も改める |
| PR を作る前に `main` で ADR-0024 が変わる | `main` を取り込み、ステップ 1-4 をやり直す |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1（ステップ 1-1〜1-4）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR のレビューで、書き直した項目が ADR-0018 へのリンクを持ち、`HOME` と `GODEBUG` だけを読むことを今の事実として述べ、未実装を前提にする記述がないことを読む（ステップ 1-1） |
| AC-02 | `diff` | `docs/tasks/0078_adr0024_server_env_result/_checks/main.go`（ステップ 1-2）。下の実行の記録 |
| AC-03 | `diff` | 同上 |
| AC-04 | `diff` | 同上 |
| AC-05 | `manual` | PR の説明に、ADR-0024 の訂正を述べる訂正の段落があることを読む |

実行の記録（ステップ 1-4）：未記入。

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-05 の検証がすべて済み、§7 に記録がある。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   実装の PR のマージの後、`/runpr` が issue #776 を閉じる。
