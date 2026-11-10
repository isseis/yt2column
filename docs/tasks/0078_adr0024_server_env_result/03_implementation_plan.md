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

ADR-0024 の「結果」の最後の項目から未実装を述べる部分を除き（F-001）、その直しを編集上の訂正として記録する（F-002）。

### 1.2. 実装の原則

-   **変えるのは、`docs/dev/adr/0024-env-access-in-config.md` の 1 行と、本タスクのディレクトリの下のファイルだけである**（AC-02・AC-03）。
-   **新しい恒久のテストは足さない**（設計書 7 章）。

### 1.3. 計画の際に決めたこと（2026-11-10）

-   設計書が開いたままにした計画の選択はない。フェーズは 1 つで、実装の PR は 1 本である。そのため PR の作成ポイントは置かない。ADR は作らない（本タスクの中で閉じる計画上の選択である）。

### 1.4. 既存コード調査結果

コミット `3747237fdcb051c436e0487f017c2f9e3a3c4ac7`（`origin/main`）で確かめた。

-   書き直す項目は `docs/dev/adr/0024-env-access-in-config.md:35` にある。「結果」の 4 項目のうち最後のものである。
-   `docs/dev/adr/` の中で「まだコードに」「実装されると」を含む行は、この 1 行だけである（`git grep -n "まだコードに\|実装されると" origin/main -- docs/dev/adr/` の出力は 1 行）。
-   `origin/main` の ADR に、issue #776 にリンクしたずれの記述はない（`git grep -nE 'issues/776([^0-9]|$)' origin/main -- docs/dev/adr/` の出力は空）。
-   この項目の文を確かめる既存のテストはない。コードの変更は要らない。
-   確認のスクリプトの手本：タスク 0071 の `docs/tasks/0071_security_source_link/_checks/main.go` が、git の読み出し（`changedFiles`、`main.go:777`）と訂正の行の確かめ（`checkCorrections`、`main.go:529`）を持ち、日本語の文字列を `_checks/data.json` に置いている。これらを写して使う。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: ADR-0024 の書き直しと確かめ（設計書 3 章・7 章、AC-01〜AC-05）

**変更するファイル:** `docs/dev/adr/0024-env-access-in-config.md`、`docs/tasks/0078_adr0024_server_env_result/_checks/main.go`（新規）、`docs/tasks/0078_adr0024_server_env_result/_checks/data.json`（新規。ステップ 1-2 で日本語の文字列を置くとき）、本計画（§7 の記録）

-   [ ] **ステップ 1-1**: ADR-0024 の「結果」の最後の項目を、次のとおり書き直す（AC-01、設計書 3 章）。

    変更の前：

    ```markdown
    -   [ADR-0018](0018-server-config-file.md) は、サーバが設定ファイルだけから設定を読み、環境からは `HOME` と `GODEBUG` だけを読むと決めている。この決定はまだコードに入っていない。実装されると、サーバが環境から読むのはその 2 つだけになる。
    ```

    変更の後：

    ```markdown
    -   [ADR-0018](0018-server-config-file.md) は、サーバが設定ファイルだけから設定を読み、環境からは `HOME` と `GODEBUG` だけを読むと決めている。サーバが環境から読むのはその 2 つだけである。
    ```

    このコミットには ADR-0024 のほかの変更を入れない。メッセージに、ADR-0024 を名指す訂正の行を 1 行書く（AC-04）。形は `docs/dev/conventions/requirements_process.md`「Recording in the PR description」の訂正の行に従う。
-   [ ] **ステップ 1-2**: 確認のスクリプトを `_checks/main.go` に書く（AC-01〜AC-04。手本は §1.4）。コミットを 2 つ（マージベースと、確かめるコミット）引数で受け取り、作業ツリーを読まずに、次を確かめる。
    -   確かめるコミットの ADR-0024 で、「結果」の最後の項目が `](0018-server-config-file.md)` を含む（AC-01 のリンクの部分）。
    -   ADR-0024 の 2 つの版で、「結果」の最後の項目の行を除いたほかの行が、同じ順序で同じである。「結果」の項目の数が同じである（AC-02）。
    -   2 つのコミットの間で追加・削除・変更されたファイルが、ADR-0024 と本タスクのディレクトリの下だけである（AC-03）。
    -   2 つのコミットの間で ADR-0024 を変えるコミットごとに、メッセージに行頭が `Editorial correction: docs/dev/adr/0024-env-access-in-config.md: ` の行がちょうど 1 行あり、その行は `; no decision changed` で終わり、`@` と `<` を含まない。ADR-0024 を変えないコミットに、その行がない（AC-04）。

    違反があれば 0 でない終了コードで終わり、違反の内容と、確かめた 2 つのコミットの完全な ID と、`main.go`（と `data.json`）の blob の ID を出力する。コメント・識別子・文字列リテラルは英語で書く。「結果」の見出しのような日本語の文字列は、`_checks/` が guard-test literal exception（`CLAUDE.md`「Go Idioms」）の対象ではないので、タスク 0071 と同じく `_checks/data.json` に置く。`make fmt` と `go vet -tags test ./docs/tasks/0078_adr0024_server_env_result/_checks/` を通してからコミットする。
-   [ ] **ステップ 1-3**: スクリプトが狙った理由で失敗できることを確かめる。ステップ 1-2 のコミットから使い捨てのブランチを作り、次の壊し方を 1 件ずつ一時的なコミットにし、そのコミットを引数にして走らせて、狙った項目で失敗することを確かめる。
    -   AC-01：「結果」の最後の項目から ADR-0018 へのリンクを除く。
    -   AC-02：「決定」の 1 行を変える。「経緯」に項目を 1 つ足す。「結果」の最後の項目を消す。「結果」に 5 つ目の項目を足す。
    -   AC-03：ほかの ADR の 1 行を変える。索引 `docs/dev/adr/README.md` の 1 行を変える。`docs/dev/adr/` に新しいファイルを足す。
    -   AC-04：訂正の行のないメッセージで ADR-0024 を変える。訂正の行を 2 行にする。`; no decision changed` を欠いた訂正の行にする。ADR-0024 を変えないコミットに訂正の行を書く。

    使い捨てのブランチは push せず、確かめた後に消す。壊し方ごとの結果は、ステップ 1-4 のコミットのメッセージに書く。
-   [ ] **ステップ 1-4**: 確認のスクリプトを、マージベースとその時点の最後のコミットを引数にして実行する。出力・終了コード・2 つのコミットの完全な ID を本計画 §7 に書き、本計画の進み具合とともにコミットする。このコミットは本タスクのディレクトリの下しか変えないので、§7 の記録はこのコミットの後の head にも当てはまる。`make test` と `make lint` を通す。

**PR を作る直前の確かめ直し。** `/runplan` のレビュー（手順 6〜7.7）の後、PR を作る前（`.claude/commands/_lib/task-pr-handoff.md` Part B の手順 1・2 の修正の後も含む）に、次を行う。`/runplan` がこれを実装のまとまりとして拾わないよう、チェックボックスにはしない。

1.  `git fetch origin main` の後で、`origin/main` を取り込む必要があるかを確かめる。取り込んだときは、ステップ 1-4 をやり直す。
2.  ステップ 1-4 の後に、本タスクのディレクトリの外を変えるコミット、または `_checks/` を変えるコミットが入ったときは、ステップ 1-4 をやり直して §7 の記録を改める。`_checks/` が変わったときは、ステップ 1-3 もやり直す。

**完了の条件:** 確認のスクリプトが終了コード 0 で終わり、その記録が本計画 §7 にある。壊し方ごとの結果が、ステップ 1-4 のコミットのメッセージにある。`make test` と `make lint` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 書き直した ADR-0024、`_checks/` のスクリプト、§7 の実行の記録 |

PR は 1 本である。`/runplan` が PR を作るとき、訂正の段落を、ブランチのコミットの訂正の行から PR の説明に書く（`.claude/commands/_lib/task-pr-handoff.md` Part B step 3。AC-05）。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** N/A。コードを変えない。
-   **確認のスクリプト:** `_checks/main.go` を、ステップ 1-4 と PR を作る直前の確かめ直しで実行する（AC-01 のリンクの部分・AC-02・AC-03・AC-04）。このスクリプトは `make test` にも CI にも組み込まない。AC-04 はコミットメッセージの性質で、変更の前後の関係ではないが、マージベースからの範囲に属するので同じスクリプトで確かめる（設計書 7 章）。
-   **読んで確かめる:** 書き直した文の内容（AC-01）と PR の説明の訂正の段落（AC-05）は、PR のレビューで読む。
-   **後方互換のテスト:** N/A。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| PR のレビュアーが、編集上の訂正という分類に異を唱える | 規約に従い決定の変更として扱い、ADR-0024 の「経緯」に項目を足し、要件書の決定を改める（`docs/dev/conventions/adr.md`「Editorial corrections」の「Classifying a change」）。その場合、AC-02 の「経緯に項目が足されない」と AC-04 も改める |
| 記録の後にコミットが増え、§7 の記録が head に当てはまらなくなる | PR を作る直前の確かめ直し（フェーズ 1 の末尾）でやり直す |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1（ステップ 1-1〜1-4）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `diff`・`manual` | リンクの部分は `docs/tasks/0078_adr0024_server_env_result/_checks/main.go`（ステップ 1-2）。下の実行の記録。文の内容は、PR のレビューで、`HOME` と `GODEBUG` だけを読むことを今の事実として述べ、未実装を前提にする記述がないことを読んで確かめる |
| AC-02 | `diff` | `_checks/main.go`。下の実行の記録 |
| AC-03 | `diff` | 同上 |
| AC-04 | `diff` | 同上 |
| AC-05 | `manual` | PR の説明に、ADR-0024 の訂正を述べる訂正の段落があることを読む |

実行の記録（ステップ 1-4）：未記入。

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-05 の検証がすべて済み、§7 に記録がある。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   実装の PR のマージの後、`/runpr` が issue #776 を閉じる。
