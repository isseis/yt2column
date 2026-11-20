# ADR-0054: 直前コミットで CI 通過済みかつ非コード・非ブラウザ拡張のみの変更時は check-changes で重いジョブをスキップする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

`/runpr` のマージ直前手順では、文書の承認記録や実装計画書のチェックボックス更新などをまとめた `PRE_MERGE` コミットを push し、再び `mergepr prepare` で CI の完了を待ってから squash merge する（[runpr.md](../../../.claude/commands/runpr.md)）。コード変更を含む PR に、このような文書のみの追加コミットを push すると、`.github/workflows/ci.yml` の `check-changes` ジョブが PR 全体（`origin/main...HEAD`）の差分を見て分類するため、コードもブラウザ拡張も変わっていないにもかかわらず `test`・`lint`・`extension` の重いジョブが丸ごと再実行される。

一方で、`mergepr` は「PR に CI チェックが 1 つ以上あり、そのすべてが通った `headRefOid`（PR の先端コミット）だけを `--match-head-commit` でマージする」ことを保証している（[mergepr_guide.md](../guides/mergepr_guide.md)）。また、`PRE_MERGE` コミットや文書修正は文書を編集するため、`doc-guard`（文書の値・構造・同期ガード、[ADR-0016](0016-document-test-policy.md)）と `secret-scan`（追跡対象ファイル内の PEM 秘密鍵の検査）は引き続き毎回実行する必要がある。

## 決定

直前の親コミットで必要な重いジョブが通過済みであり、今回の追加コミットが非コードかつ非ブラウザ拡張の変更のみである場合は、`[skip ci]` ではなく `.github/workflows/ci.yml` の `check-changes` ジョブで重いジョブ（`test`・`lint`・`extension`）だけをスキップする。詳細な要件は [0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) に、ワークフローの仕様は [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) に置く。

-   **スキップ方式:** `[skip ci]` / `[no ci]` は使わず、スキップ条件を満たすときに `check-changes` ジョブが `has-code-changes=false` および `has-extension-changes=false` を出力して `test`・`lint`・`extension` のみをスキップし、`doc-guard` と `secret-scan` は通常どおり実行する。`mergepr` の保証とプロトコル番号を変えず、必須ステータスチェックとも両立し、`doc-guard` と `secret-scan` を引き続き毎回実行するためである（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-004）。
-   **スキップの単位:** Go 側（`test`・`lint`）とブラウザ拡張側（`extension`）をまとめて判定し、親コミットとの差分が既存の分類規則で非コードかつ非ブラウザ拡張のファイルのみであり、かつ親コミットで 3 ジョブすべてが通過済みのときだけ、両出力（`has-code-changes` と `has-extension-changes`）を `false` にする。系統別に独立判定すると条件分岐と境界ケースが増えるためである（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-001）。
-   **親コミットの CI 結果の条件:** PR head コミットの親がちょうど 1 つ（マージコミットでない）であることを前提とする。そのうえで、親コミットに対する本リポジトリ・同一 PR の `CI` ワークフロー（`pull_request`）の最新実行が完了状態かつ結論が `success` または `failure`（`cancelled` 等ではない）であり、かつ `check-changes` が `success`、`test`・`lint`・`extension` がすべて `success` または `skipped` で完了していることを要求する。別 PR の実行との取り違えやジョブ一覧が揃う前の競合、およびワークフローのキャンセルによる未実行ジョブの誤認を防ぎつつ、`doc-guard` や `secret-scan` の失敗を文書修正で直した場合や、文書のみを連続して push した場合でも無駄な再実行を避けるためである（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-001・F-002）。
-   **前提条件と安全側フォールバック:** 同一リポジトリ内の `main` 向け PR で、手動再実行（`run_attempt > 1`）ではなく、かつ親コミットの CI 実行後に `main` が進んでいないことを確かめられたときだけスキップする。フォークからの PR、非 `main` ベース、手動再実行、`main` の更新、および判定に必要な情報取得、API 応答の検査、またはブラウザ拡張の変更分類に失敗したときは、ジョブを失敗させずに通常分類（`origin/main...HEAD`）へフォールバックする。未検証のコードや古い `main` との合成結果が誤って CI を通過することを防ぎつつ、手動再実行（「Re-run all jobs」）を強制再実行の逃げ道として残すためである（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-003）。
-   **最小権限:** 親コミットの CI 結果の確認に要する読み取り権限は `check-changes` ジョブに限り付与し、ワークフロー全体や他ジョブの権限を広げず、書き込み権限は付与しない。CI の最小権限の原則を守るためである（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-004）。
-   **食い違い:** この決定はまだ [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) に実装されていない。`check-changes` ジョブは現在も `origin/main...HEAD` のみで分類しており、直前コミットの CI 結果に基づくスキップは行わない。この食い違いは https://github.com/isseis/yt2column/issues/1073 の実装で解消する。

## 採らなかった案

-   **`[skip ci]` / `[no ci]` の付与案:** コミットメッセージに `[skip ci]` / `[no ci]` を付ける方式は、`mergepr` の保証（チェック 0 件で `prepare` が止まる前提、および CI 通過済みの `headRefOid` だけをマージする前提）、必須ステータスチェック、`PRE_MERGE` 時の `doc-guard`・`secret-scan` の継続実行と衝突する（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「issue #1073 で決めたこと」）。
-   **系統別の独立スキップ判定案:** Go 側（`test`・`lint`）とブラウザ拡張側（`extension`）を系統別に独立してスキップ判定すると、条件分岐と境界ケースが増える。`PRE_MERGE` や通常の `docs/`・`.claude/` 修正は非コードかつ非ブラウザ拡張のみであり、まとめて判定する単純な形で十分に対象にできる（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。
-   **親コミットのワークフロー全体の成功要求案:** 親コミットのワークフロー全体の成功を要求すると、`test`・`lint`・`extension` が成功して `doc-guard` だけが失敗した PR で文書を直したとき、重いジョブまで再実行されてしまう（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。
-   **手動再実行でのスキップ適用案:** 手動再実行（`run_attempt > 1`）でもスキップを適用すると、重いジョブを強制的に走らせ直したいときの逃げ道がなくなる（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。
-   **フォークからの PR へのスキップ適用案:** フォークからの PR もスキップ対象に含めると、別リポジトリ由来のコミットやワークフロー実行との取り違えリスクを負うことになり、本リポジトリの通常運用にも利点がない（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。

## 結果

-   `mergepr` と `/runpr` の仕様・プロトコル番号を変えずに、`PRE_MERGE` コミットや文書のみの追加 push 後の CI 待ち時間と実行コストを削減できる。
-   `doc-guard`（文書の値・構造・同期ガード）と `secret-scan`（追跡対象ファイル内の PEM 秘密鍵の検査）はスキップ時も毎回実行される。
-   将来 `.github/workflows/ci.yml` に `check-changes` の出力で制御される新しい重いジョブを追加する場合は、同じ PR で親コミットで通過済みかを確認する対象ジョブの一覧も更新する必要がある。
-   1 回の push で複数コミットをまとめて積んだ場合や、`README.md`・`README.en.md`・`docs/dev/guides/development.md` を含む文書修正を積んだ場合、および `main` が進んだ場合は、重いジョブはスキップされず、通常どおり実行される。

## 経緯

- 2026-11-21 `[skip ci]` を採らず、`.github/workflows/ci.yml` の `check-changes` で、直前コミットで CI 通過済みかつ非コード・非ブラウザ拡張のみの変更時に重いジョブ（`test`・`lint`・`extension`）をまとめてスキップする方針と安全条件（同一 PR 限定・ワークフロー完了確認＋ジョブ単位判定・連続 push の連鎖許可・手動再実行時はスキップしない・フォーク PR 除外・`main` 不変性・安全側フォールバック・最小読み取り権限）を定めた（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「issue #1073 で決めたこと」「要件定義の際に決めたこと（2026-11-21）」）
