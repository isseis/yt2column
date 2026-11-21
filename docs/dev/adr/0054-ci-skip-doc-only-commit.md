# ADR-0054: 直前コミットで CI 通過済みかつ非コード・非ブラウザ拡張のみの変更時は check-changes で重いジョブをスキップする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

`/runpr` のマージ直前手順では、文書の承認記録や実装計画書のチェックボックス更新などをまとめた `PRE_MERGE` コミットを push し、再び `mergepr prepare` で CI の完了を待ってから squash merge する（[runpr.md](../../../.claude/commands/runpr.md)）。コード変更を含む PR に、このような文書のみの追加コミットを push すると、`.github/workflows/ci.yml` の `check-changes` ジョブが PR 全体（`origin/main...HEAD`）の差分を見て分類するため、コードもブラウザ拡張も変わっていないにもかかわらず `test`・`lint`・`extension` の重いジョブが丸ごと再実行される。

一方で、`mergepr` は「PR に CI チェックが 1 つ以上あり、そのすべてが通った `headRefOid`（PR の先端コミット）だけを `--match-head-commit` でマージする」ことを保証している（[mergepr_guide.md](../guides/mergepr_guide.md)）。また、`PRE_MERGE` コミットや文書修正は文書を編集するため、`doc-guard`（文書の値・構造・同期ガード、[ADR-0016](0016-document-test-policy.md)）と `secret-scan`（追跡対象ファイル内の PEM 秘密鍵の検査）は引き続き毎回実行する必要がある。

## 決定

直前の親コミットで必要な重いジョブが通過済みであり、今回の追加コミットが非コードかつ非ブラウザ拡張の変更のみである場合は、`[skip ci]` ではなく `.github/workflows/ci.yml` の `check-changes` ジョブから Go 製の CI 専用ツール（`cmd/ciskip` + `internal/ciskip`）を呼び、重いジョブ（`test`・`lint`・`extension`）だけをスキップする。詳細な要件は [0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) に、設計は [0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) に、ワークフローの仕様は [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) に置く。

### スキップ方針と安全条件

-   **スキップ方式:** `[skip ci]` / `[no ci]` は使わず、スキップ条件を満たすときに `check-changes` ジョブが `has-code-changes=false` および `has-extension-changes=false` を出力して `test`・`lint`・`extension` のみをスキップし、`doc-guard` と `secret-scan` は通常どおり実行する。
    -   理由：`mergepr` の保証とプロトコル番号を変えず、必須ステータスチェックとも両立し、`doc-guard` と `secret-scan` を引き続き毎回実行するため（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-004）。
-   **スキップの単位:** Go 側（`test`・`lint`）とブラウザ拡張側（`extension`）をまとめて判定し、親コミットとの差分が既存の分類規則で非コードかつ非ブラウザ拡張のファイルのみであり、かつ親コミットで 3 ジョブすべてが通過済みのときだけ両出力を `false` にする。
    -   理由：系統別に独立判定すると条件分岐と境界ケースが増えるため（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-001）。
-   **親コミットの CI 結果の条件:** PR head コミット（PR ブランチの先端コミット）の親がちょうど 1 つ（マージコミットでない）であることを前提とする。そのうえで、親コミットに対する本リポジトリ・同一 PR の `CI` ワークフロー（`pull_request`）の最新実行が完了状態かつ結論が `success` または `failure`（`cancelled` 等ではない）であり、かつ `check-changes` が `success`、`test`・`lint`・`extension` がすべて `success` または `skipped` で完了していることを要求する。
    -   理由：別 PR の実行との取り違えやジョブ一覧が揃う前の競合、およびワークフローのキャンセルによる未実行ジョブの誤認を防ぎつつ、`doc-guard` や `secret-scan` の失敗を文書修正で直した場合や、文書のみを連続して push した場合でも無駄な再実行を避けるため（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-001・F-002）。
-   **前提条件と安全側フォールバック:** 同一リポジトリ内の `main` 向け PR で、手動再実行（`run_attempt > 1`）ではなく、かつ親コミットの CI 実行後に `main` が進んでいないことを確かめられたときだけスキップする。フォークからの PR、非 `main` ベース、手動再実行、または `main` の更新に該当するときや、判定に必要な情報取得・API 応答の検査・ブラウザ拡張の変更分類に失敗したときは、ジョブを失敗させずに通常分類（`origin/main...HEAD`）へフォールバックする。
    -   理由：未検証のコードや古い `main` との合成結果が誤って CI を通過することを防ぎつつ、手動再実行（「Re-run all jobs」）を強制再実行の逃げ道として残すため（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-003）。
-   **最小権限:** 親コミットの CI 結果とベース整合性の確認に要する読み取り権限（`actions: read` および `checks: read`）は `check-changes` ジョブに限り付与し、ワークフロー全体や他ジョブの権限を広げず、書き込み権限は付与しない。
    -   理由：CI の最小権限の原則を守るため（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md) F-004、[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）。

### 実装構造と API 検証方式

-   **`main` 不変性の検証方式:** 通常分類でコードまたはブラウザ拡張の変更が検出されて `ciskip` が起動するたびに、`pull_request` イベントでチェックアウトされる合成マージコミットの第 1 親（`HEAD^1`、その実行が実際に検証した `main` コミット）を固定形式の `::notice` アノテーション（`title=ci-base-sha`）として記録する。次回のスキップ判定時には、親コミットの `check-changes`（表示名 `Check changed files`）ジョブのチェック実行アノテーション（Checks API）から読み戻して今回の `HEAD^1` および `origin/main` と照合する。
    -   理由：GitHub Actions REST API は過去の合成マージコミットの `HEAD^1` を返さず、`pull_requests[].base.sha` も古くなりうるため、明示的に記録した SHA を照合することで偽陽性を防ぎつつ、PR ブランチが `main` を取り込んでいない一般的なケースでもスキップを有効にするため（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）。
-   **判定ロジックの配置先:** [ADR-0014](0014-deterministic-checks-in-go.md) の方針に従い、Go 製の CI 専用ツール `cmd/ciskip` + `internal/ciskip` として実装し、`.github/workflows/ci.yml` の `check-changes` ジョブから `go run ./cmd/ciskip` で呼び出す。コンパイル済みバイナリの事前ビルド・配布は行わない。インストール型ツールではなく CI 専用のため、`make install-*` やプロトコル番号の対象外とする。既存の通常分類ステップはそのまま残し、後段の最終出力ステップでスキップ判定を適用する。
    -   理由：`check-changes` は `wasm-dep-dirs.sh` のためにすでに Go をセットアップ済みであり、`internal/strictjson` と `internal/cmdrunner` を再利用して厳密な検証と包括的なユニットテストを書けるため。また、`go.mod` は外部依存がなく `cmd/ciskip` の依存パッケージも少ないため、`GOCACHE` が空の状態から `go run` で都度ビルドしても数秒程度（4 vCPU 換算で約 4〜6 秒）で終わり、スキップ適用時は常時実行の `doc-guard`（約 1 分）と並列に走るため PR 全体の待ち時間を増やさない（文書のみ PR で通常分類が両方 `false` のときは `ciskip` 自体を起動しない）（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）。
-   **GitHub API の呼び出し方式:** 既存の Go 製開発ツールと同じく `internal/cmdrunner` 経由で `gh api --hostname github.com -i` を呼び出す（`GH_HOST`・`GH_REPO` 等を環境から除外し、`git`・`bash` へは `GH_TOKEN`・`GITHUB_TOKEN` を渡さない）。HTTP リダイレクト（3xx）を拒否できない点と HTTP 層で応答サイズ上限を指定できない点は `gh` の制限として受け入れ、呼び出し側で出力読み込み上限（`cmdrunner.Options.MaxOutput`）とタイムアウト・プロセスグループ終了を課すことでメモリ枯渇とハングを防ぐ。
    -   理由：GitHub API の認証を `gh` の認証機構へ委譲でき、`ciskip` 自身が `GITHUB_TOKEN`／`GH_TOKEN` を直接読み込んで保持・ヘッダ付与・ログやエラー出力からの秘匿を自前管理せずに済むため（[ADR-0024](0024-env-access-in-config.md) の方針変更も不要になる）。あわせて既存の `cmdrunner`・`gh` パターンを再利用して実装の複雑さを抑えられるため（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）。
-   **食い違い:** この決定はまだ [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) に実装されておらず、`cmd/ciskip` および `internal/ciskip` も未作成である。`check-changes` ジョブは現在も `origin/main...HEAD` のみで分類しており、直前コミットの CI 結果に基づくスキップは行わない。この食い違いは https://github.com/isseis/yt2column/issues/1073 の実装で解消する。

## 採らなかった案

要件定義と設計で比較し、安全性・運用性・保守性の観点から採らなかった選択肢と理由をまとめる。

-   **`[skip ci]` / `[no ci]` の付与案:** コミットメッセージに `[skip ci]` / `[no ci]` を付ける方式は、`mergepr` の保証（チェック 0 件で `prepare` が止まる前提、および CI 通過済みの `headRefOid` だけをマージする前提）、必須ステータスチェック、`PRE_MERGE` 時の `doc-guard`・`secret-scan` の継続実行と衝突する（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「issue #1073 で決めたこと」）。
-   **系統別の独立スキップ判定案:** Go 側（`test`・`lint`）とブラウザ拡張側（`extension`）を系統別に独立してスキップ判定すると、条件分岐と境界ケースが増える。`PRE_MERGE` や通常の `docs/`・`.claude/` 修正は非コードかつ非ブラウザ拡張のみであり、まとめて判定する単純な形で十分に対象にできる（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。
-   **親コミットのワークフロー全体の成功要求案:** 親コミットのワークフロー全体の成功を要求すると、`test`・`lint`・`extension` が成功して `doc-guard` だけが失敗した PR で文書を直したとき、重いジョブまで再実行されてしまう（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。
-   **手動再実行でのスキップ適用案:** 手動再実行（`run_attempt > 1`）でもスキップを適用すると、重いジョブを強制的に走らせ直したいときの逃げ道がなくなる（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。
-   **フォークからの PR へのスキップ適用案:** フォークからの PR もスキップ対象に含めると、別リポジトリ由来のコミットやワークフロー実行との取り違えリスクを負うことになり、本リポジトリの通常運用にも利点がない（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）。
-   **`main` 不変性の git 祖先判定のみ案:** `origin/main` が親コミットの祖先である場合だけスキップする方式であれば `checks: read` は不要になるが、PR 作成後に他 PR が `main` にマージされた場合、ブランチ自体に最新 `main` を取り込んでいなければスキップが効かず、実運用での削減効果が下がる（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）。
-   **シェルスクリプト（`.github/scripts/`）＋ `curl` / `jq` 実装案:** 厳密な JSON 検査（不正 UTF-8・不対サロゲート・末尾データ・消費メンバの重複キー拒否）と包括的ユニットテストをシェルで書くのは保守性が低く、[ADR-0014](0014-deterministic-checks-in-go.md) の方針にも合わない（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）。
-   **`ciskip` のコンパイル済みバイナリを事前ビルドして再利用する案:** `check-changes` では `wasm-dep-dirs.sh`（`go list`）のために Go のセットアップ自体を省けず、事前ビルド済みバイナリの取得・展開にも数秒かかるため、`go run` の数秒のビルドを省く実質の短縮幅が小さい。一方で、リポジトリ格納による履歴肥大化・レビュー不能化・ソースとの食い違い、または GitHub Releases 等での別途クロスビルド・配布・チェックサム検証や `ciskip` 自身を修正する PR での検証切り替えといった運用コストとリスクが増える。将来さらに数秒の短縮が必要になった場合も、バイナリ配布ではなく `actions/cache` による `GOCACHE` キャッシュで `go run` のまま短縮できる（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）。
-   **Go `net/http` 直接呼び出し案:** リダイレクト拒否と HTTP 層のサイズ上限を直接制御できる一方、`gh` の認証機構へ委譲できず、`ciskip` 自身が `GITHUB_TOKEN`（または `GH_TOKEN`）を直接受け取って扱う必要がある。環境変数からのトークン読み込みは [ADR-0024](0024-env-access-in-config.md) の方針と衝突し、トークンの保持・`Authorization` ヘッダの付与・ログやエラー出力への非露出や独自の HTTP クライアント管理を自前で担うことになる。認証を `gh` へ委譲し、本リポジトリの他の開発ツールと同じ `gh` + `cmdrunner` の仕組みを使って出力上限とタイムアウトでメモリ枯渇を防ぐ方が単純で一貫性がある（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）。

## 結果

本決定により、既存の `mergepr`・`/runpr` と常時実行ジョブ（`doc-guard`・`secret-scan`）の保証を保ちつつ文書のみの追加 push での CI 待ち時間を削減できる一方、`check-changes` の読み取り権限が加わり、`gh` の仕様上の制限が残る。

-   `mergepr` と `/runpr` の仕様・プロトコル番号を変えずに、`PRE_MERGE` コミットや文書のみの追加 push 後の CI 待ち時間と実行コストを削減できる。
-   `doc-guard`（文書の値・構造・同期ガード）と `secret-scan`（追跡対象ファイル内の PEM 秘密鍵の検査）はスキップ時も毎回実行される。
-   `check-changes` ジョブに `actions: read` と `checks: read` の読み取り権限が加わり、通常分類でコードまたはブラウザ拡張の変更がある実行の Summary の Annotations 欄に `ci-base-sha` の notice が 1 件表示される。
-   `gh` コマンドの仕様上、HTTP 3xx リダイレクトの拒否と HTTP 層での応答サイズ上限の指定は行えないという制限がある（呼び出し側の `cmdrunner.Options.MaxOutput` とタイムアウトでメモリ枯渇とハングを防ぐ）。
-   `::notice` アノテーションを `GITHUB_TOKEN` + `checks: read` で Checks API から読み戻せるかは実装 PR の実機確認（F-006）で確かめる（読み戻せない場合は安全側で通常実行に倒れる）。
-   将来 `.github/workflows/ci.yml` に `check-changes` の出力で制御される新しい重いジョブを追加する場合は、同じ PR 内で、親コミットの通過確認対象となるジョブの一覧（`internal/ciskip`）も更新する必要がある。
-   1 回の push で複数コミットをまとめて積んだ場合や、`README.md`・`README.en.md`・`docs/dev/guides/development.md` を含む文書修正を積んだ場合、および `main` が進んだ場合は、重いジョブはスキップされず、通常どおり実行される。

## 経緯

- 2026-11-21 `[skip ci]` を採らず、`.github/workflows/ci.yml` の `check-changes` で、直前コミットで CI 通過済みかつ非コード・非ブラウザ拡張のみの変更時に重いジョブ（`test`・`lint`・`extension`）をまとめてスキップする方針と安全条件（同一 PR 限定・ワークフロー完了確認＋ジョブ単位判定・連続 push の連鎖許可・手動再実行時はスキップしない・フォーク PR 除外・`main` 不変性・安全側フォールバック・最小読み取り権限）を定めた（[0119 要件定義書](../../tasks/0119_ci_skip_doc_only_commit/01_requirements.md)「issue #1073 で決めたこと」「要件定義の際に決めたこと（2026-11-21）」）
- 2026-11-21 設計で、`main` 不変性の検証方式（`::notice` による `HEAD^1` 記録＋Checks API 読み戻し、`checks: read` 追加）、判定ロジックの配置先（Go 製の CI 専用ツール `cmd/ciskip` + `internal/ciskip`）、および GitHub API 呼び出し方式（`internal/cmdrunner` 経由の `gh api`、リダイレクト拒否不可・HTTP 層サイズ上限なしを制限として受け入れ、呼び出し側の読み込み上限とタイムアウトでメモリ枯渇を防止）を確定した（[0119 設計書](../../tasks/0119_ci_skip_doc_only_commit/02_architecture.md) §1.2）
