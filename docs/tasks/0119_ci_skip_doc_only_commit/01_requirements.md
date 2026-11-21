# 要件定義書：直前コミットで CI 通過済みかつドキュメントのみの変更時に重いジョブをスキップする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Process | `full` |
| Created | 2026-11-21 |
| Review date | 2026-11-21 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、コード変更を含む PR に対してドキュメントのみ（非コードかつ非ブラウザ拡張、以下「ブラウザ拡張」を「拡張」と呼ぶ。分類の定義は §6）の追加コミットを push したとき、直前の親コミットで必要な CI ジョブが通過済みであれば、`.github/workflows/ci.yml` の重いジョブ（`test`・`lint`・`extension`）をスキップし、常時実行ジョブ（`doc-guard`・`secret-scan`）だけを走らせるための要件を定義する。

対応 issue: [#1073](https://github.com/isseis/yt2column/issues/1073)

**背景:**

-   `/runpr` のマージ直前手順（`.claude/commands/runpr.md:320-378`・`:1617-1653`）では、文書の承認記録・実装計画書のチェックボックス更新・変更履歴の `#未記入` の埋め込み・`diff` 記録の再記録を 1 つの `PRE_MERGE` コミットにまとめて push し、再び `mergepr prepare` で CI の完了を待ってから squash merge する。
-   現在の `.github/workflows/ci.yml:12-72` の `check-changes` ジョブは、PR 全体の差分（`origin/main...HEAD`）をもとに `has-code-changes`（`.github/workflows/ci.yml:35-52`）と `has-extension-changes`（`.github/workflows/ci.yml:63-72`）を出力する。`test`・`lint` は `has-code-changes == 'true'`（`.github/workflows/ci.yml:74-140`）、`extension` は `has-extension-changes == 'true'`（`.github/workflows/ci.yml:141-197`）のときに実行され、`doc-guard` と `secret-scan` は常時実行される（`.github/workflows/ci.yml:201-234`）。
-   そのため、コード変更を含む PR に `docs/tasks/...` のみを変える `PRE_MERGE` コミットや、通常の文書修正コミットを追加 push すると、コードも拡張も変わっていないにもかかわらず `test`・`lint`・`extension` の重いジョブが丸ごと再実行される。
-   コミットメッセージに `[skip ci]` や `[no ci]` を付けてワークフロー自体を止める案は採らない。`mergepr` の保証（`gh pr checks --watch --fail-fast` でチェック 0 件の PR を止め、CI が通った `headRefOid`（PR の先端コミット）だけを `--match-head-commit` でマージする仕様。`internal/mergepr/mergepr.go:275`、`docs/dev/guides/mergepr_guide.md:30`・`:265`）、必須ステータスチェック、および `doc-guard`（文書の値・構造・同期ガード）と `secret-scan`（追跡対象ファイル内の PEM 秘密鍵の検査）を毎回実行する方針（[ADR-0016](../../dev/adr/0016-document-test-policy.md)、`.github/workflows/ci.yml:198-234`）と衝突するためである。

**本書の記述範囲:** 本書は、CI で重いジョブをスキップする条件、スキップ時と通常実行時の観測可能な振る舞い、前提が崩れた場合とエラー時の安全側（fail-safe）の振る舞い、権限の最小化、ADR の作成、および GitHub Actions での実機確認の要件を定める。具体的な実装方式は `02_architecture.md` で決める。設計で決める対象は、GitHub API のエンドポイントと参照フィールドの選定、付与する読み取り権限の正確な名前、`pull_request` の合成マージコミットにおける PR head コミットとベースブランチの整合性の検証手段（各用語は §3.1・§6 で定義）、判定ロジックの配置先（Go ツールかスクリプトか、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) との整合）、および既存テスト（`extension/test/repository.test.ts:143-251`）との調整である。設計へ引き継ぐ論点は [design_handoff.md](design_handoff.md) に申し送る。

### issue #1073 で決めたこと

-   **スキップ方式:** `[skip ci]` / `[no ci]` は使わず、`.github/workflows/ci.yml` の `check-changes` で重いジョブ（`test`・`lint`・`extension`）だけをスキップし、`doc-guard` と `secret-scan` は通常どおり実行する。`mergepr`・`/runpr` の保証とプロトコル番号を変えず、必須ステータスチェックとも両立させるためである。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md) を作る（後述の「要件定義の際に決めたこと」の「ADR と更新文書」を参照）。
-   **スキップの 4 条件の骨格:** (1) PR head コミットがマージコミットではない（親コミットがちょうど 1 つ）、(2) 親コミットとの差分が既存の分類規則で非コードかつ非拡張のファイルのみ、(3) 親コミットに対する直前の CI で `test`・`lint`・`extension` がすべて `success` または `skipped` で完了している、(4) ベースブランチ（`main`）の変更などで前提が崩れていない、の 4 つをすべて満たすときにスキップする。必要な情報を取得できない場合や API 呼び出しが失敗した場合は、安全側に倒して通常どおり実行する。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。
-   **権限の最小化:** 親コミットのチェック結果の確認に必要な読み取り権限は、最小限の範囲で `check-changes` ジョブに付与することを検討する。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。

### 要件定義の際に決めたこと（2026-11-21）

-   **進め方の選択:** 通常の進め方（`full`）とする。除外判定は「除外に当たらない」（`docs/dev/security.md` の規則・前提（[ADR-0006](../../dev/adr/0006-threat-model.md)）を変えず、[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) の開発ツールの信頼境界にも触れず、外部依存の追加・変更もない）。そのうえで、CI の誤スキップを防ぐ新しい仕組みであり、GitHub API の信頼境界と `pull_request` 合成マージコミットの整合性を設計の panel レビューで確かめる価値が高いため、通常の進め方を選んだ。人の答え：通常の進め方（`full`）。
    -   ADR：なし。本タスクの中で閉じる進め方の選択である。
-   **スキップの単位:** Go 側（`test`・`lint`）と拡張側（`extension`）をまとめて判定する（F-001）。親コミットとの差分が非コードかつ非拡張のファイルのみであり、かつ親コミットの CI で 3 ジョブすべてが通過済みのときだけ、`has-code-changes=false` と `has-extension-changes=false` の両方を出力する。系統ごとに別々に判定すると条件分岐と境界ケースが増えるため、issue #1073 の提案どおり単純な形にした。人の答え：まとめて判定する。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。
-   **親コミットの CI 結果の条件と手動再実行:** 親コミットの CI 結果は同一 PR に紐付く実行に限定する。ワークフロー実行自体が完了状態（実行中・待機中でない）であることを前提に、ワークフロー全体の結論ではなくジョブ単位の結論（`check-changes` が `success`、かつ `test`・`lint`・`extension` がすべて `success` または `skipped`）で判定し、連続する文書のみの push でのスキップの連鎖を許す（F-002）。また、手動再実行（`run_attempt > 1`）時はスキップを適用せず PR 全体の通常分類を行う（F-003）。別 PR の実行との取り違えやジョブ一覧が揃う前の競合を防ぎつつ、`doc-guard` の失敗を文書修正で直したときや `/fixpr` の文書修正の後に `PRE_MERGE` を積んだときの無駄な再実行を避けるためである。あわせて、GitHub の「Re-run all jobs」で重いジョブを強制的に走らせ直す逃げ道を残すためでもある。人の答え：ジョブ単位・連続 push の連鎖許可・再実行時はスキップしない、同一 PR の実行のみ＋ワークフロー実行自体が完了状態。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。
-   **フォークからの PR の扱い:** 同一リポジトリ内の PR のみをスキップ対象とし、フォークからの PR ではスキップしない（F-003）。別リポジトリ由来のコミットやワークフロー実行との取り違えリスクを構造的に排除でき、本リポジトリの通常運用に影響しないためである。人の答え：同一リポジトリの PR のみスキップ対象。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。
-   **空コミットの扱い:** 親コミットとの差分が 0 件の空コミット（例: `.claude/commands/_lib/simple-process.md:450-452` の仕上げコミット）も、非コードかつ非拡張のみの変更としてスキップ対象に含める（F-001）。強制再実行の逃げ道は手動再実行（`run_attempt > 1`）で確保できるためである。人の答え：スキップ対象に含める。
    -   ADR：なし。本タスクの中で閉じるエッジケースの振る舞いである。
-   **ADR と更新文書:** 新規 ADR として [ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md) を作成し、ADR の索引（`docs/dev/adr/README.md`）に 1 行を加える（F-005）。CI スキップ方針そのものの記載は ADR-0054 に限り、`CLAUDE.md` や [ADR-0016](../../dev/adr/0016-document-test-policy.md) 等の既存 ADR には追記しない。一方、設計（`02_architecture.md`）で Go ツール等の実装方式を選んだ際に規約上必要となる更新（[ADR-0024](../../dev/adr/0024-env-access-in-config.md) や `CLAUDE.md` の例外、`package_reference.md` 等）の要否は `02_architecture.md` に委ねる（[design_handoff.md](design_handoff.md) H-03）。方針の正本を新規 ADR の 1 か所にまとめつつ、設計段階での実装方式の選択を不必要に縛らないためである。人の答え：新規 ADR と ADR 索引のみ（設計依存の規約更新は `02_architecture.md` に委ねる）。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md) を作る。
-   **GitHub Actions での実機確認:** GitHub Actions 上での動作（コード変更後の通常実行、続く文書のみのコミットでの重いジョブのスキップと `doc-guard`・`secret-scan` の継続実行）を実機で確認し、結果を実装計画書に記録することを要件に含める（F-006）。GitHub Actions はローカルで再現できないため、マージ前に実動作の証跡を残す。人の答え：GitHub Actions 実機確認を要件に含める。
    -   ADR：なし。本タスクの検証手順の選択である。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   コード変更を含む PR に `PRE_MERGE` コミットや文書のみの修正コミットを追加 push したとき、直前の親コミットで必要な重いジョブが通過済みであれば、`test`・`lint`・`extension` をスキップして CI の待ち時間と実行コストを削減する。
-   スキップ時も `doc-guard`（文書の値・構造・同期ガード）と `secret-scan`（追跡対象ファイル内の PEM 秘密鍵の検査）を通常どおり実行し続ける。
-   `mergepr` と `/runpr` の保証・プロトコル番号、および必須ステータスチェックとの両立を維持する。
-   前提が崩れた場合や判定に必要な情報を確かめられない場合は、必ず安全側（通常実行）に倒し、未検証コードのマージを防ぐ。

### 2.2. スコープ (In Scope)

-   PR head コミットの親の数と親コミットとの差分に基づく変更ファイル分類によるスキップ適格判定（F-001）。
-   親コミットに対する同一 PR の直前 CI 結果の検証（ワークフロー完了確認・ジョブ単位判定・連続 push の連鎖を含む）（F-002）。
-   ベースブランチ（`main`）の整合性、同一リポジトリ判定、手動再実行時のスキップ無効化、および失敗・不備時の安全側フォールバック（F-003）。
-   `check-changes` の出力制御、最小読み取り権限の付与、および常時実行ジョブ（`doc-guard`・`secret-scan`）と既存ジョブ構造の維持（F-004）。
-   新規 ADR（[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)）と ADR 索引の作成・更新、および実装時の食い違いの記述の除去（F-005）。
-   GitHub Actions 上での実機確認と結果の記録（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   `[skip ci]` や `[no ci]` によるワークフロー全体のスキップ（issue #1073 で不採用と決定）。
-   `mergepr`・`/runpr`・`fixprguard` の仕様変更およびプロトコル番号の引き上げ。
-   既存のファイル分類規則（`*.md`・`docs/`・`.claude/`・`LICENSE` を非コードとする規則、および `extension/scripts/has-extension-changes.sh`・`extension/scripts/wasm-dep-dirs.sh` の拡張対象規則）そのものの変更。
-   Go 側（`test`・`lint`）と拡張側（`extension`）を系統別に独立してスキップすること（要件定義の際に決めたこと）。
-   1 回の push で複数コミットをまとめて積んだときに、親コミット（PR head コミットの第 1 親）よりさらに前の祖先まで遡って CI 結果を探すこと。
-   CI スキップ方針そのものを `CLAUDE.md` や既存 ADR（[ADR-0016](../../dev/adr/0016-document-test-policy.md) 等）へ記載すること（要件定義の際に決めたこと。設計で選択した実装方式に伴う規約更新の要否は `02_architecture.md` で判断する）。
-   新しい外部 GitHub Action や外部ライブラリ（Go モジュール・npm パッケージ）の追加。
-   GitHub リポジトリ側の必須ステータスチェック設定の変更（リポジトリ設定の手作業）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

以下の要件で「PR head コミット」とは、PR ブランチの先端コミット（GitHub Actions の `pull_request` イベントが示す PR の head コミット）を指し、「親コミット」とはその第 1 親（1 つ前のコミット）を指す。`actions/checkout` が `pull_request` でチェックアウトする合成マージコミット（`HEAD`）とは区別する（§6 用語集）。

#### F-001: コミット構造と差分の分類によるスキップ適格判定

`check-changes` ジョブは、PR 全体（`origin/main...HEAD`）に対する既存の通常分類の結果のいずれかが `true` である場合（すでに両方 `false` ならスキップ判定は不要）に限り、PR head コミットがスキップ適格であるかを次の 2 条件で判定する。

-   **親コミットの数:** PR head コミットの親コミットがちょうど 1 つであること。マージコミット（親が 2 つ以上）およびルートコミット（親が 0 個）はスキップ対象外とする。
-   **親コミットとの差分:** 親コミットから PR head コミットまでの変更ファイル一覧が、既存の分類規則において**非コードかつ非拡張**のファイルのみであること。
    -   変更ファイルが 0 件（空コミット）の場合は、この条件を満たす。
    -   ファイル名の変更（リネーム）があった場合は、変更前と変更後の両方のパスを検査する（コードまたは拡張対象ファイルを `docs/` 配下等の非コード・非拡張のパスへリネームした場合、変更前のパスがコード・拡張対象に当たるためスキップ対象外とする）。
    -   非 ASCII 文字や制御文字（タブ・改行等）を含むパスでも、クォートや行分割によって非コード・非拡張と誤判定されないこと。
    -   `README.md`・`README.en.md`・`docs/dev/guides/development.md`・WASM 依存パッケージ配下のファイル（例: `internal/llm/devfake/articles/*.md`）のように、非コードであっても既存の拡張分類規則（`extension/scripts/has-extension-changes.sh:45-55`）で拡張対象とされるファイルが 1 つでも含まれる場合は、スキップ対象外とする。
    -   WASM 依存パッケージの列挙（`extension/scripts/wasm-dep-dirs.sh`）または拡張変更分類（`extension/scripts/has-extension-changes.sh`）が失敗した場合は、拡張対象でないことを確かめられないためスキップ対象外とし、通常分類の既存フォールバック（`has-extension-changes=true`、`.github/workflows/ci.yml:67-70`）を維持する。

**Acceptance Criteria**:
- **AC-01**: PR head コミットの親コミットが 2 つ以上（マージコミット）または 0 個の場合、変更ファイルの内容や親コミットの CI 結果にかかわらず、本機能のスキップは適用されず、PR 全体（`origin/main...HEAD`）に対する通常分類の結果が出力される。
- **AC-02**: PR head コミットの親コミットがちょうど 1 つで、親コミットとの差分が非コードかつ非拡張のファイル（例: `docs/tasks/...`・`.claude/...`・`LICENSE`・`docs/dev/adr/...`）のみ、または変更ファイル 0 件（空コミット）の場合、F-002・F-003 の条件も満たせば `check-changes` は `has-code-changes=false` と `has-extension-changes=false` を出力して重いジョブをスキップする。
- **AC-03**: 親コミットとの差分に、コードファイル（例: `internal/foo/a.go`・`.github/workflows/ci.yml`・`Makefile`）、または既存の拡張分類規則で拡張対象となるファイル（例: `extension/` 配下、`README.md`、`README.en.md`、`docs/dev/guides/development.md`、WASM 依存パッケージ配下のファイル）が 1 つでも含まれる場合、本機能のスキップは適用されず、PR 全体（`origin/main...HEAD`）に対する通常分類の結果が出力される（例: 拡張対象外の Go ファイル変更では通常分類どおり `has-code-changes=true`・`has-extension-changes=false` となる）。
- **AC-04**: 親コミットとの差分に、コードまたは拡張対象ファイルから文書パスへのリネーム（例: `internal/foo/a.go` → `docs/a.md`）、または非 ASCII 文字・タブ・改行を含むコード／拡張対象パスが含まれる場合、非コード・非拡張のみとは判定されず、本機能のスキップは適用されず PR 全体（`origin/main...HEAD`）に対する通常分類の結果が出力される。

#### F-002: 親コミットの直前 CI 結果の検証

`check-changes` ジョブは、F-001 を満たす場合、親コミットに対する本リポジトリ・同一 PR の `CI` ワークフロー（`.github/workflows/ci.yml`、`pull_request` イベント）の直前の実行結果を取得し、必要なジョブが通過済みであることを検証する。

-   **検証対象:** 親コミットに対する、本リポジトリかつ**同一 PR** に紐付く `CI` ワークフロー（`pull_request` イベント）の最新の実行。別のワークフロー（例: `deepseek-review.yml`）、`pull_request` 以外のイベント、別 PR または別リポジトリの実行結果、PR との紐付けが空または曖昧な実行結果、最新より古い実行結果は使わない。
-   **通過済みの条件:** 最新の実行自体が完了状態（`completed`。実行中・待機中ではない）かつそのワークフロー実行の結論が `success` または `failure`（`cancelled`・`timed_out`・`startup_failure`・`stale`・`action_required` 等ではない）であり、次の 4 ジョブがすべて重複なく存在し、それぞれの結論が条件を満たすこと。ワークフロー実行の結論から `cancelled` 等を除外するのは、`check-changes` の成功後にワークフローがキャンセルされた場合に未実行の重いジョブが `skipped` となって通過済みと誤認されるのを防ぐためである。
    -   `check-changes`（表示名 `Check changed files`、`.github/workflows/ci.yml:12-13`）が `success` であること。
    -   `test`（`Test`、`.github/workflows/ci.yml:74-75`）、`lint`（`Code Quality & Lint`、`.github/workflows/ci.yml:98-99`）、`extension`（`Browser extension`、`.github/workflows/ci.yml:141-142`）の 3 ジョブすべてが `success` または `skipped` であること。
-   **`doc-guard` / `secret-scan` の扱い:** ワークフロー実行自体が完了状態かつ結論が `success` または `failure` であれば、親コミットで `doc-guard` または `secret-scan` が失敗していても（ワークフロー全体の結論が `failure` でも）、上の 4 ジョブが条件を満たしていれば通過済みとする（今回の PR head コミットで `doc-guard` と `secret-scan` が改めて実行されるため）。
-   **連続 push の連鎖:** 親コミット自身の CI でも、さらに前のコミットの通過結果を引き継いで重いジョブがスキップされた結果 `test`・`lint`・`extension` が `skipped` であっても、実行自体が `success` または `failure` で完了しており `check-changes` が `success` で F-003 の前提条件も満たすなら、引き続きスキップできる。
-   **複数コミットの一括 push:** 1 回の push で複数コミットが積まれ、親コミットに該当する CI 実行が存在しない場合は、さらに前の祖先を辿らず、通過済みではないとして通常実行する。

**Acceptance Criteria**:
- **AC-05**: 親コミットに対する本リポジトリ・同一 PR の `CI` ワークフロー（`pull_request`）の最新実行が結論 `success` または `failure` で完了しており、`check-changes` が `success` かつ `test`・`lint`・`extension` がすべて `success` または `skipped` の場合（`doc-guard` または `secret-scan` の成否を問わない）、F-001・F-003 の条件も満たせば `check-changes` は `has-code-changes=false` と `has-extension-changes=false` を出力して重いジョブをスキップする。
- **AC-06**: 親コミットの最新 `CI` 実行自体が未完了（実行中・待機中）である場合、ワークフロー実行の結論が `cancelled`・`timed_out`・`startup_failure` 等（`success`・`failure` 以外）である場合、`test`・`lint`・`extension` のいずれかが `failure`・`cancelled`・`timed_out`・`action_required`・`neutral`・`stale` または未完了である場合、`check-changes` が `success` でない場合、または 4 ジョブのいずれかが欠損・重複している場合、本機能のスキップは適用されず、PR 全体（`origin/main...HEAD`）に対する通常分類の結果が出力される。
- **AC-07**: 親コミットに本リポジトリ・同一 PR の `CI` ワークフロー（`pull_request`）の実行が存在しない場合（初回コミット、複数コミットの一括 push の中間コミット、別ワークフロー（例: `deepseek-review.yml`）や `pull_request` 以外のイベントや別 PR の実行しかない場合、PR 紐付けが空または曖昧な場合）、または最新より古い実行だけが成功していて最新実行が失敗・キャンセル・未完了である場合、本機能のスキップは適用されず、PR 全体（`origin/main...HEAD`）に対する通常分類の結果が出力される。
- **AC-08**: 親コミット自身がスキップ済みのコミット（実行自体が `success` または `failure` で完了し、`check-changes` が `success`、`test`・`lint`・`extension` がすべて `skipped`）であり、今回の追加コミットも非コード・非拡張のみである場合、F-003 の前提条件を満たす限り重いジョブは再びスキップされる。

#### F-003: 前提条件の整合性確認・手動再実行の逃げ道・安全側フォールバック

`check-changes` ジョブは、親コミットの CI 結果が今回の PR head コミットにもそのまま有効とみなせる前提が崩れていないことを確かめ、前提が 1 つでも崩れている場合、または崩れていないことを確かめられない場合は安全側に倒して通常どおり重いジョブを実行する。

-   **同一リポジトリ・対象ベースブランチ:** PR の head と base が同一リポジトリであり、かつ base ブランチが `main` であること。フォークからの PR、または base ブランチが `main` でない PR（および取得・検証できない場合）ではスキップしない。
-   **手動再実行の逃げ道:** ワークフローの実行回数（`run_attempt`）が 2 以上（例: GitHub UI からの「Re-run all jobs」）のときは、スキップ判定を適用せず PR 全体（`origin/main...HEAD`）の通常分類を使う。
-   **ベースブランチ（`main`）の不変性:** 親コミットの CI 実行が検証した `main` の状態から、今回の実行時点の `main` が進んでいないこと。親コミットの CI 実行以降に `main` へ新たなコミットが入った場合、またはその整合性を確実に検証できない場合はスキップしない。
-   **安全側フォールバック（fail-safe）:** GitHub API の呼び出し失敗（ネットワーク障害・認可エラー・レート制限・タイムアウト・想定外の HTTP ステータス）、応答の解析失敗や規約違反（§3.2）、必要な git 情報の取得失敗、WASM 依存列挙または拡張変更分類の失敗など、スキップ判定の途中で何らかの異常が起きた場合は、`check-changes` ジョブ自体を失敗させるのではなく、スキップ不可として通常の `origin/main...HEAD` 分類結果をそのまま出力する。また、スキップしたか通常実行したかとその理由をジョブのログに出力し、後から確認できるようにする。

**Acceptance Criteria**:
- **AC-09**: フォークからの PR（head リポジトリと base リポジトリが異なる、またはリポジトリ情報を取得・検証できない場合）、または base ブランチが `main` でない（もしくは取得・検証できない）PR では、ほかの条件をすべて満たしていても本機能のスキップは適用されず、PR 全体（`origin/main...HEAD`）に対する通常分類の結果が出力される。
- **AC-10**: 手動再実行（`run_attempt` が 2 以上）の場合、または `run_attempt` を正の整数として読み取れない場合は、ほかの条件をすべて満たしていても本機能のスキップは適用されず、PR 全体（`origin/main...HEAD`）に対する通常分類の結果が出力される。
- **AC-11**: 親コミットの CI 実行が検証した `main` の状態と今回の実行時点の `main` が異なる場合（`main` に新しいコミットが追加された場合）、または `main` が変わっていないことを確かめられない場合、本機能のスキップは適用されず、PR 全体（`origin/main...HEAD`）に対する通常分類の結果が出力される。
- **AC-12**: スキップ判定で用いる外部 API 呼び出し・応答の検査・git 参照・WASM 依存列挙（`wasm-dep-dirs.sh`）・拡張変更分類（`has-extension-changes.sh`）のいずれかが失敗した場合、`check-changes` ジョブは異常終了せず、スキップ不可として PR 全体（`origin/main...HEAD`）に対する通常分類の結果を出力して安全側に倒す。特に `wasm-dep-dirs.sh` が失敗したときは、空の依存一覧でスキップ判定を行わず、通常分類の既存フォールバック（`has-extension-changes=true`、`.github/workflows/ci.yml:67-70`）を維持する。
- **AC-13**: `check-changes` ジョブのログに、スキップを適用したか通常分類を使ったか、およびスキップしなかった場合の理由（マージコミット、コード／拡張変更あり、親 CI 未通過、`main` 更新、フォーク PR、非 `main` ベース、手動再実行、取得・検査失敗など）が出力される。

#### F-004: ジョブ出力・常時実行ジョブ・最小権限と既存範囲の維持

-   **出力とスキップ動作:** F-001〜F-003 をすべて満たすとき、`check-changes` は `has-code-changes=false` および `has-extension-changes=false` を出力する（既存の出力名・値 `.github/workflows/ci.yml:15-17`・`:47-51`・`:71` を変えない）。これにより `test`・`lint`・`extension` は `skipped` となる一方、`doc-guard` と `secret-scan` は `check-changes` に依存せず通常どおり実行される（`.github/workflows/ci.yml:201-234`）。
-   **権限の最小化:** ワークフロー全体の `permissions`（`contents: read`、`.github/workflows/ci.yml:7-8`）は広げない。親コミットの CI 結果確認に必要な読み取り権限は `check-changes` ジョブのみに限定して付与し、書き込み権限はいずれのジョブにも付与しない。
-   **既存ワークフロー構造の維持:** `.github/workflows/ci.yml` の変更は `check-changes` ジョブの定義に限る。

**Acceptance Criteria**:
- **AC-14**: F-001〜F-003 をすべて満たす場合、`check-changes` は `has-code-changes=false` と `has-extension-changes=false` の両方を出力し、`test`・`lint`・`extension` がスキップされる。このときも `doc-guard` と `secret-scan` は通常どおり実行される。
- **AC-15**: `.github/workflows/ci.yml` のワークフロー全体の `permissions` は `contents: read` のままであり、追加される権限は `check-changes` ジョブに限定された読み取り権限（`: read`）のみである。書き込み権限（`: write`）はいずれのジョブにも付与されない。
- **AC-16**: 実装 PR における `.github/workflows/ci.yml` の差分は `check-changes` ジョブの定義に限られる。
- **AC-17**: `check-changes` は既存の出力名（`has-code-changes`・`has-extension-changes`）と値（`true` / `false`）を維持し、スキップ条件を満たさないすべてのケースで、既存の `origin/main...HEAD` に対する通常分類（`wasm-dep-dirs.sh` 失敗時の `has-extension-changes=true` フォールバックを含む）と同じ出力を返す。

#### F-005: 新規 ADR（ADR-0054）の作成と整合

-   直前コミットで CI 通過済みかつ非コード・非拡張のみの変更時に重いジョブをスキップする方針と安全条件を [ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md) として作成し、`docs/dev/adr/README.md` に番号順で 1 行追加する。
-   本要件定義書の PR 時点では、`.github/workflows/ci.yml` が未実装である旨の食い違いの記述を ADR-0054 の `## 決定` に置き、issue #1073 の完全な URL へのリンクを記載する（`docs/dev/conventions/adr.md`「Writing an ADR」「Discrepancy statements」）。
-   実装 PR でこの食い違いを解消し、同じ実装 PR で ADR-0054 から食い違いの記述を取り除く。

**Acceptance Criteria**:
- **AC-18**: `docs/dev/adr/0054-ci-skip-doc-only-commit.md` が存在し、状態が `有効` で、`docs/dev/adr/README.md` に番号順で索引行があり、`TestADRRecords` が通る。
- **AC-19**: 実装 PR において、ADR-0054 の `## 決定` は実装済みの振る舞いと一致し、未実装を示す食い違いの記述が残っていない。

#### F-006: GitHub Actions での実機確認と記録

GitHub Actions 上での挙動を実機で確認し、確認日・PR 番号・ワークフロー実行の URL と結果を `03_implementation_plan.md` の手動確認の節に記録する。

**Acceptance Criteria**:
- **AC-20**: GitHub Actions 上で次の 2 つのケースを実機確認し、確認日・PR 番号・それぞれの実行 URL とジョブ結果（`check-changes`・`test`・`lint`・`extension`・`doc-guard`・`secret-scan`）を `03_implementation_plan.md` に記録している。
    1.  コード変更を含むコミットの push で `test`・`lint`（および該当すれば `extension`）と `doc-guard`・`secret-scan` が通常どおり実行されて成功すること。
    2.  その直後に非コードかつ非拡張のファイルのみを変更するコミットを追加 push したとき、`check-changes` が `has-code-changes=false`・`has-extension-changes=false` を出力して `test`・`lint`・`extension` が `skipped` になり、かつ `doc-guard` と `secret-scan` が実行されて成功すること。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

`check-changes` ジョブは、GitHub API 応答、GitHub Actions コンテキスト入力、git コマンド出力、および拡張分類スクリプトの出力を読み取ってスキップ可否を判定する。これらを信頼できない入力として境界で厳密に検査し、標準ライブラリやパーサが黙って補正・破棄・無視しうる不正な入力を補正せず拒否する。

| 境界 | 受理する形 | 上限 | 消費するメンバと拡張性 | 失敗・拒否時の扱い | 主な AC |
|---|---|---|---|---|---|
| GitHub API 応答（親コミットの CI 実行・ジョブ情報、ベース整合性情報） | HTTPS で取得しリダイレクトに従わない HTTP 200 応答の有効な UTF-8 JSON 文書で、単一のトップレベル値を持ち末尾に余分なデータがないこと。消費するメンバがすべて存在し、期待する型・閉じた列挙値（状態・結論）・形式（40 桁小文字 16 進の SHA 等）に一致すること。重複キー・欠損・型不一致・不正な UTF-8・切り詰め・必須ジョブの重複を補正せず拒否する | 応答本文サイズ（展開後を含む）とページ数／件数に上限を設ける（具体的な上限値は `02_architecture.md` で定める） | 消費する論理メンバは、ワークフロー実行の識別子・イベント種別・対象コミット SHA・実行状態・実行結論・紐付く PR・ベース整合性の確認項目・最新順の比較キー、および各ジョブの名前・状態・結論とする。GitHub API 応答は付加フィールドを多数含み将来の追加もあるため、設計で宣言する消費メンバ以外の未消費メンバは**拡張可能**（無視して受理）と宣言する。一方、文書全体の UTF-8・単一トップレベル値・末尾データなし・重複キーの検査は応答全体に適用する | すべての拒否（番兵エラーを設ける場合は設計で定める拒否種別）を、単一の安全側判定「スキップ不可（PR 全体の通常分類へフォールバック）」に写像する。非 200 応答の生本文やトークン等の秘密情報をログに出さない | AC-05〜AC-08・AC-11・AC-12 |
| GitHub Actions コンテキスト入力（PR 番号、head/base リポジトリ、base ブランチ名、`run_attempt`、PR head SHA 等） | 各項目が期待する形式に完全に一致すること（PR 番号と `run_attempt` は前後の空白や符号のない正の 10 進整数、base ブランチ名はちょうど `main`、PR head SHA は 40 桁小文字 16 進文字列、head/base リポジトリは空でない識別子）。前後の空白除去や大文字小文字の正規化などの補正を行わない | 各コンテキスト値の長さ上限（設計で定める） | 設計で宣言する必須入力のみを消費する（拡張可能と宣言するメンバなし） | 欠損・形式不一致・上限超過はすべて「スキップ不可（PR 全体の通常分類へフォールバック）」に写像する | AC-09・AC-10・AC-12 |
| git コマンド出力（PR head コミットの親一覧、親コミットから PR head コミットまでの変更ファイルパス一覧、ベース参照） | 終了コード `0` であること。コミット OID は 40 桁小文字 16 進文字列。変更ファイルパス一覧は、リネーム前後の両パス、空白、非 ASCII 文字、制御文字（タブ・改行等）を損なわず区切りを曖昧にしない形で読み取ること（具体的なオプションは `02_architecture.md` で定める） | 変更ファイル一覧は 1 パスずつ分類し、コード・拡張対象パスが 1 件出た時点でスキップ不可とするため件数による切り捨ては行わない（設計で出力サイズ等の上限を設ける場合、超過時は安全側へ倒す） | 固定形式の行・パス要素のみを消費する（拡張可能と宣言するメンバなし） | 終了コード非ゼロまたは想定外の形式はすべて「スキップ不可（PR 全体の通常分類へフォールバック）」に写像する | AC-01〜AC-04・AC-12 |
| 拡張分類スクリプト出力（`wasm-dep-dirs.sh`・`has-extension-changes.sh`） | 終了コード `0` で、`wasm-dep-dirs.sh` は空でないディレクトリ一覧（末尾 `/`）、`has-extension-changes.sh` はちょうど `true` または `false` を返すこと（`extension/scripts/wasm-dep-dirs.sh:10-13`、`extension/scripts/has-extension-changes.sh:23-62`） | 既存スクリプトの契約に従う | 出力行の固定形式のみを消費する（拡張可能と宣言するメンバなし） | 非ゼロ終了・空出力・`true`/`false` 以外の出力はすべてスキップ不可とし、通常分類の既存フォールバック（`has-extension-changes=true`）を維持する | AC-03・AC-12・AC-17 |

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   スキップ判定（親コミットの確認・差分分類・GitHub API 照会）は、`test`・`lint`・`extension` の再実行時間に比べて十分に短い時間（数秒〜十数秒程度）で完了すること。API 呼び出しにはタイムアウトを設け、応答遅延でジョブが無制限に待たないようにする。

### 4.2. セキュリティ (Security)

-   条件付きガイド（`docs/dev/security.md`）のトリガ（外部コマンド `yt-dlp` の起動、LLM の API キー、キャッシュディレクトリ、LLM API へのネットワーク送信、信頼できないテキストのプロンプトまたは記事への埋め込み）には該当しないため、同ガイドの適用は **N/A** である。
-   CI 固有のセキュリティ要件として次を守る。
    -   **最小権限:** 追加する権限は `check-changes` ジョブ限定の読み取り権限のみとし、ワークフロー全体や他ジョブの権限を広げない。書き込み権限を付与しない（AC-15）。
    -   **フォーク PR の排除:** フォークからの PR ではスキップを適用しない（AC-09）。
    -   **API 通信とトークンの保護:** GitHub API への通信は HTTPS に限り、HTTP リダイレクト（3xx）に従わない（認証ヘッダの他ホストへの流出防止）。応答本文は展開後を含めサイズ上限を設け（高圧縮ファイル爆弾対策）、非 200 応答の本文や `GITHUB_TOKEN` 等の認証情報をログやエラー出力に出さない。
    -   **インジェクション防止:** PR タイトル・ブランチ名・コミットメッセージ・ファイルパス等の外部由来の文字列を、シェルコマンドや YAML の `${{ ... }}` 直接展開で実行しない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   すべての不確実な状況（API 失敗、レート制限、応答の形式不備、ジョブ欠損・重複、`main` 更新、複数コミット一括 push、未完了の親 CI、拡張分類スクリプトの失敗）は安全側（スキップ不可＝通常実行）に倒し、誤スキップ（未検証コードや古い `main` との合成結果が CI を通過すること）を防ぐ（AC-06・AC-07・AC-11・AC-12）。
-   `check-changes` ジョブ自体は、API 障害などスキップ判定側の失敗で失敗（異常終了）しない（通常分類で継続し、CI 全体の可用性を損なわない）（AC-12）。

### 4.4. 互換性 (Compatibility)

-   `mergepr` の保証（CI 完了待ち、`--match-head-commit` によるマージ、チェック 0 件での停止、`docs/dev/guides/mergepr_guide.md:30`・`:265`）および `/runpr` の `PRE_MERGE` 手順（`.claude/commands/runpr.md:1617-1653`）との互換性をそのまま維持すること。両ツールの変更やプロトコル番号の引き上げは不要である。
-   GitHub の必須ステータスチェックにおいて、ジョブが `skipped` となった場合の挙動（条件付き `if:` でスキップされたジョブの扱い。`.github/workflows/ci.yml:77-78`・`:101-102`・`:144-145`）をそのまま維持する。
-   既存の `check-changes` の出力名（`has-code-changes`・`has-extension-changes`、`.github/workflows/ci.yml:15-17`）および値（`true` / `false`）を変えない（AC-17）。

### 4.5. 保守性 (Maintainability)

-   スキップ判定のロジックは、ユニットテストで各条件（正常スキップ、マージコミット、リネーム、非 ASCII／制御文字パス、拡張対象ファイル、親 CI の各種結論・欠損・重複・未完了、`main` 更新、フォーク PR、非 `main` ベース、手動再実行、API 失敗・不正応答）を独立に検証できる構成にする（実装手段は [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) を踏まえて `02_architecture.md` で決める）。
-   将来 `.github/workflows/ci.yml` に新しい重いジョブ（`check-changes` の出力で制御されるジョブ）を追加するときは、親コミットの CI で通過を確認する対象ジョブ一覧にも同じ PR で追加する必要がある（[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)「結果」）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。新しい外部ライブラリ・新しい GitHub Action を追加しない。
-   `CLAUDE.md` の「Declare, don't infer」に従い、スキップ可否をコミットメッセージの文字列（`[skip ci]` や `PRE_MERGE` の文言等）で判定しない。判定は親コミット数・変更ファイルパス・GitHub API が宣言するジョブ結論等の構造化された事実のみに基づく。
-   `mergepr`・`/runpr`・`fixprguard` を変更せず、プロトコル番号を上げない。
-   `docs/dev/conventions/requirements_process.md` の通常の進め方に従い、`01_requirements.md` の承認後に `02_architecture.md`、`03_implementation_plan.md`、実装の順に進める。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **重いジョブ:** `.github/workflows/ci.yml` のうち、`check-changes` の出力で実行を制御される `test`・`lint`・`extension` の 3 ジョブ（`.github/workflows/ci.yml:74-197`）。
-   **常時実行ジョブ:** `check-changes` の出力に依存せず、すべての PR で常に実行される `doc-guard` と `secret-scan` の 2 ジョブ（`.github/workflows/ci.yml:201-234`）。
-   **コードファイル／非コードファイル:** `.github/workflows/ci.yml:39-44` の既存分類規則において、`*.md`・`docs/` 配下・`.claude/` 配下・`LICENSE` に該当するファイルを「非コードファイル」、それ以外を「コードファイル」と呼ぶ。
-   **拡張対象ファイル／非拡張ファイル:** `extension/scripts/has-extension-changes.sh:44-55` の既存分類規則において、`extension/*`・`Makefile`・`.github/workflows/*`・`go.mod`・`go.sum`・`README.md`・`README.en.md`・`docs/dev/guides/development.md` および `extension/scripts/wasm-dep-dirs.sh` が列挙する WASM 依存パッケージ配下に該当するファイルを「拡張対象ファイル」、それ以外を「非拡張ファイル」と呼ぶ。
-   **`PRE_MERGE` コミット:** `/runpr` のマージ直前手順（`.claude/commands/runpr.md:320-378`）で、文書の承認記録・実装計画書のチェックボックス更新・変更履歴の `#未記入` の埋め込み・`diff` 記録の再記録をまとめて記録するコミット。
-   **PR head コミット:** PR ブランチの先端のコミット。GitHub Actions の `pull_request` イベントでは、`actions/checkout` がチェックアウトする合成マージコミット（`refs/pull/<N>/merge`、親が `base` と PR head の 2 つ。`HEAD` が指す）とは別に、`github.event.pull_request.head.sha` が指す実コミットである。
-   **親コミット:** PR head コミットの第 1 親（1 つ前のコミット）。
-   **通常分類:** 既存の `check-changes` が行う、PR 全体の差分（`origin/main...HEAD`）に対する `has-code-changes` と `has-extension-changes` の分類。
-   **安全側フォールバック（fail-safe）:** スキップ条件を満たすことを確実に証明できない場合（前提崩れ・情報取得失敗・形式不備等）に、スキップせず通常分類の結果を採用して重いジョブを実行すること。
-   **食い違いの記述:** ADR の決定と、その仕様を持つ文書やコードが食い違うときに、`## 決定` に残す記述（`docs/dev/conventions/adr.md`「Writing an ADR」）。

## 変更履歴

- 2026-11-21 承認（isseis、#1075）
