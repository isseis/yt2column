# 実装計画書：ADR-0024「結果」の、サーバが環境から読む変数の記述の更新

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-10 |
| Review date | 2026-11-10 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

ADR-0024 の直し（「結果」の最後の項目の書き直し（F-001）と、「経緯」の項目 1 つ（F-002））は、決定の変更として 3 文書の PR（#780）で入れた（要件書 1 章）。本計画は、その直しを `_checks/` のスクリプトで確かめる。

### 1.2. 実装の原則

-   **実装の PR が変えるのは、本タスクのディレクトリの下のファイルだけである。** ADR-0024 は変えない。
-   **新しい恒久のテストは足さない**（設計書 7 章）。

### 1.3. 計画の際に決めたこと（2026-11-10）

-   設計書が開いたままにした計画の選択はない。フェーズは 1 つで、実装の PR は 1 本である。そのため PR の作成ポイントは置かない。ADR は作らない（本タスクの中で閉じる計画上の選択である）。
-   確かめる 2 つのコミットは、PR #780 が `main` に入ったスカッシュのコミットとその親とする。マージの後に確かめるのは、`requirements_process.md`「4. Acceptance Tests」の例外である。理由と代わりの手当ては設計書 7 章にある。

### 1.3.1. 実装の際に決めたこと（2026-11-10）

-   **確認のスクリプトを、Go の 547 行のプログラムから、`git diff` と `cmp` を使う 50 行ほどのシェルスクリプト（`_checks/check.sh`）に替える。壊し方は、確かめる項目ごとに失敗することが分かる数にとどめる（人の答え、2026-11-10）。**
    -   **理由:** 確かめる ADR-0024 の差分は、1 行の書き換えと 1 行の追加だけで、PR #780 のレビューでも読んでいる。シェルスクリプトでも AC-01 のリンクの部分・AC-02・AC-03・AC-06 の形を同じく確かめられる。Go の版は、確かめる変更の大きさに比べて、PR のレビューで読む量が大きすぎた。手本にしたタスク 0071 のプログラムの構造を写したことが、大きさの元になった。
    -   **採らなかった案:** Go の版をそのまま使う案。確かめる性質は変わらないが、レビューの負担が残る。
    -   見出しの日本語の文字列は使わない。「経緯」はファイルの最後の `## ` の節で、「結果」の最後の項目はその見出しの 2 行上にあることを、構造で確かめる。そのため `data.json` は要らない。
    -   ADR：なし。本タスクの確かめ方の選択で、本タスクの中で閉じる。同じことがほかのタスクで起きないための規約とコマンドの直しは、別のタスクで扱う。

### 1.4. 既存コード調査結果

コミット `3747237fdcb051c436e0487f017c2f9e3a3c4ac7`（`origin/main`）で確かめた。

-   書き直す項目は `docs/dev/adr/0024-env-access-in-config.md:35` にある。「結果」の 4 項目のうち最後のものである。
-   `docs/dev/adr/` の中で「まだコードに」「実装されると」を含む行は、この 1 行だけである（`git grep -n "まだコードに\|実装されると" origin/main -- docs/dev/adr/` の出力は 1 行）。
-   `origin/main` の ADR に、issue #776 にリンクしたずれの記述はない（`git grep -nE 'issues/776([^0-9]|$)' origin/main -- docs/dev/adr/` の出力は空）。
-   この項目の文を確かめる既存のテストはない。コードの変更は要らない。
-   確認のスクリプトの手本：タスク 0056 の `docs/tasks/0056_legacy_approval_history/_checks/task_docs_unchanged.sh` が、`git diff` で変わったファイルを確かめるシェルスクリプトである（§1.3.1）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: ADR-0024 の直しの確かめ（設計書 7 章、AC-01〜AC-03・AC-06）

PR #780 のマージの後に `/runplan` で行う。

**変更するファイル:** `docs/tasks/0078_adr0024_server_env_result/_checks/check.sh`（新規）、本計画（§7 の記録）

-   [x] **ステップ 1-1**: 確認のスクリプトを `_checks/check.sh` に書く（手本は §1.4、§1.3.1）。コミットを 2 つ（変更の前と後）引数で受け取り、ADR-0024 をその 2 つのコミットで読み、次を確かめる。
    -   後のコミットの ADR-0024 で、「結果」の最後の項目が ADR-0018 へのリンク（`0018-server-config-file.md`）を含む（AC-01 のリンクの部分）。
    -   ADR-0024 の 2 つの版で、「結果」の最後の項目と「経緯」の最後に足された項目を除いたほかの行が、同じ順序で同じである。「結果」の項目の数が同じである（AC-02）。
    -   2 つのコミットの間で追加・削除・変更されたファイルが、ADR-0024 と本タスクのディレクトリの下だけである（AC-03）。
    -   後の版の「経緯」が、前の版の「経緯」の項目をそのまま並べた後に、項目をちょうど 1 つ持つ。その項目が `docs/dev/conventions/adr.md`「History entries」の形で、本タスクの要件定義書にリンクする（AC-06 の形）。

    違反があれば 0 でない終了コードで終わり、違反の内容と、確かめた 2 つのコミットの完全な ID と、`check.sh` の blob の ID を出力する。コメントと文字列リテラルは英語で書き、日本語の見出しの文字列は使わない（§1.3.1）。
-   [x] **ステップ 1-2**: スクリプトが狙った理由で失敗できることを確かめる。PR #780 のスカッシュのコミットから使い捨てのブランチを作り、次の壊し方を 1 件ずつ一時的なコミットにし、スカッシュのコミットの親とその一時的なコミットを引数にして走らせて、狙った項目で失敗することを確かめる。
    -   AC-01：「結果」の最後の項目から ADR-0018 へのリンクを除く。
    -   AC-02：「決定」の 1 行を変える。「経緯」の既存の項目を 1 つ変える。「結果」の最後の項目を消す。「結果」に 5 つ目の項目を足す。
    -   AC-03：ほかの ADR の 1 行を変える。索引 `docs/dev/adr/README.md` の 1 行を変える。`docs/dev/adr/` に新しいファイルを足す。
    -   AC-06：足した「経緯」の項目を消す。「経緯」の最後にもう 1 つ項目を足す。足した項目のリンク先をほかの文書にする。

    使い捨てのブランチは push せず、確かめた後に消す。壊し方ごとの結果は、ステップ 1-3 のコミットのメッセージに書く。
-   [x] **ステップ 1-3**: 確認のスクリプトを、PR #780 のスカッシュのコミットの親とそのスカッシュのコミットを引数にして実行する。出力・終了コード・2 つのコミットの完全な ID を本計画 §7 に書き、本計画の進み具合とともにコミットする。確かめる 2 つのコミットは `main` の上で決まっているので、この後に実装のブランチにコミットが増えても §7 の記録は変わらない。`make test` と `make lint` を通す。

**PR を作る直前の確かめ直し。** `/runplan` のレビュー（手順 6〜7.7）の後、PR を作る前（`.claude/commands/_lib/task-pr-handoff.md` Part B の手順 1・2 の修正の後も含む）に、ステップ 1-3 の後で `_checks/` を変えるコミットが入ったときは、ステップ 1-2 と 1-3 をやり直して §7 の記録を改める。`/runplan` がこれを実装のまとまりとして拾わないよう、チェックボックスにはしない。

**完了の条件:** 確認のスクリプトが終了コード 0 で終わり、その記録が本計画 §7 にある。壊し方ごとの結果が、ステップ 1-3 のコミットのメッセージにある。`make test` と `make lint` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `_checks/` のスクリプト、§7 の実行の記録 |

実装の PR は 1 本である。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** N/A。コードを変えない。
-   **確認のスクリプト:** `_checks/check.sh` を、ステップ 1-3 で 1 回実行する（AC-01 のリンクの部分・AC-02・AC-03・AC-06 の形）。このスクリプトは `make test` にも CI にも組み込まない。
-   **読んで確かめる:** 書き直した文の内容（AC-01）と「経緯」の項目の言い回し（AC-06）は、PR #780 のレビューで読む。
-   **後方互換のテスト:** N/A。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| スクリプトに渡すコミットを取り違える | §7 に 2 つのコミットの完全な ID を書き、スカッシュのコミットが PR #780 のものであることを、そのメッセージと親の関係で確かめてから実行する |

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] フェーズ 1（ステップ 1-1〜1-3）
-   [x] PR を作成した: https://github.com/isseis/yt2column/pull/787
-   [x] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `diff`・`manual` | リンクの部分は `docs/tasks/0078_adr0024_server_env_result/_checks/check.sh`（ステップ 1-1）。下の実行の記録。文の内容は、PR #780 のレビューで、`HOME` と `GODEBUG` だけを読むことを今の事実として述べ、未実装を前提にする記述がないことを読んで確かめる |
| AC-02 | `diff` | `_checks/check.sh`。下の実行の記録 |
| AC-03 | `diff` | 同上 |
| AC-06 | `diff`・`manual` | 形とリンク先は `_checks/check.sh`。下の実行の記録。要旨の言い回しは、PR #780 のレビューで読んで確かめる |

AC-04 と AC-05 は欠番である（要件書 F-002）。

実行の記録（ステップ 1-3、§1.3.1 でスクリプトを替えた後のやり直し）:

-   確かめたコミット：前が `b633a13858855ec254898bba18cc3a0f45a447a2`、後が `9a30d0d38e83144c27ce4fd468d44fea01a94cfd`。後者は PR #780 のスカッシュのコミットで、subject が `docs(0078): update ADR-0024's server environment consequence and add the task documents (#780)`、親が前者であることを確かめた。
-   スクリプトの blob：`check.sh` が `f17e585acc3db4339c742e5c0d159ae083712903`。
-   実行したコマンドと出力（終了コード 0）：

```
$ sh docs/tasks/0078_adr0024_server_env_result/_checks/check.sh b633a13858855ec254898bba18cc3a0f45a447a2 9a30d0d38e83144c27ce4fd468d44fea01a94cfd
base b633a13858855ec254898bba18cc3a0f45a447a2
head 9a30d0d38e83144c27ce4fd468d44fea01a94cfd
docs/tasks/0078_adr0024_server_env_result/_checks/check.sh blob f17e585acc3db4339c742e5c0d159ae083712903
OK   scope: only docs/dev/adr/0024-env-access-in-config.md changed outside the task directory
OK   consequence: only the last consequence item and the appended last line differ
OK   consequence-link: the rewritten item links to 0018-server-config-file.md
OK   history-entry: one dated history item appended, linking to the requirements
OK every check passed
```

-   壊し方ごとの結果は、ステップ 1-3 のコミットのメッセージにある。

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-03 と AC-06 の検証がすべて済み、§7 に記録がある。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   実装の PR のマージの後、`/runpr` が issue #776 を閉じる。

## 変更履歴

- 2026-11-10 承認（isseis、#780）
- 2026-11-10 セッション中の再承認：確認のスクリプトを短いシェルスクリプトに替え、壊し方を確かめる項目ごとの数にとどめた（isseis、#787）
