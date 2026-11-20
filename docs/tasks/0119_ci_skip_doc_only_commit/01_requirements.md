# 要件定義書：直前コミットで CI 通過済みかつドキュメントのみの変更時に重いジョブをスキップする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-21 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、コード変更を含む PR に対してドキュメントのみ（非コードかつ非ブラウザ拡張）の追加コミットを push したとき、直前の親コミットで必要な CI ジョブが通過済みであれば、`.github/workflows/ci.yml` の重いジョブ（`test`・`lint`・`extension`）をスキップし、常時実行ジョブ（`doc-guard`・`secret-scan`）だけを走らせるための要件を定義する。

対応 issue: [#1073](https://github.com/isseis/yt2column/issues/1073)

**背景:**

-   `/runpr` のマージ直前手順（`.claude/commands/runpr.md:320-378`・`:1617-1649`）では、文書の承認記録・実装計画書のチェックボックス更新・変更履歴の `#未記入` の埋め込み・`diff` 記録の再記録を 1 つの `PRE_MERGE` コミットにまとめて push し、再び `mergepr prepare` で CI の完了を待ってから squash merge する。
-   現在の `.github/workflows/ci.yml:12-72` の `check-changes` ジョブは、PR 全体の差分（`origin/main...HEAD`）をもとに `has-code-changes`（`.github/workflows/ci.yml:35-52`）と `has-extension-changes`（`.github/workflows/ci.yml:63-72`）を出力する。`test`・`lint` は `has-code-changes == 'true'`（`.github/workflows/ci.yml:74-140`）、`extension` は `has-extension-changes == 'true'`（`.github/workflows/ci.yml:141-197`）のときに実行され、`doc-guard` と `secret-scan` は常時実行される（`.github/workflows/ci.yml:201-234`）。
-   そのため、コード変更を含む PR に `docs/tasks/...` のみを変える `PRE_MERGE` コミットや、通常の文書修正コミットを追加 push すると、コードもブラウザ拡張（以下「拡張」）も変わっていないにもかかわらず `test`・`lint`・`extension` の重いジョブが丸ごと再実行される。
-   コミットメッセージに `[skip ci]` や `[no ci]` を付けてワークフロー自体を止める案は、`mergepr` の保証（`gh pr checks --watch --fail-fast` でチェック 0 件の PR を止め、CI が通った `headRefOid` だけを `--match-head-commit` でマージする。`internal/mergepr/mergepr.go:275`、`docs/dev/guides/mergepr_guide.md:30`・`:265`）、必須ステータスチェック、および `PRE_MERGE` の文書編集ミスを検知する `doc-guard`・`secret-scan` の継続実行と衝突するため採らない。

**本書の記述範囲:** 本書は、CI で重いジョブをスキップする条件、スキップ時と通常実行時の観測可能な振る舞い、前提が崩れた場合とエラー時の安全側（fail-safe）の振る舞い、権限の最小化、ADR の作成、および GitHub Actions での実機確認の要件を定める。具体的な実装方式（GitHub API のエンドポイントと参照フィールドの選定、付与する読み取り権限の正確な名前、`pull_request` の合成マージコミットにおける PR head コミットとベースブランチの整合性の検証手段、判定ロジックの配置先（Go ツールかスクリプトか、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) との整合）、既存テスト（`extension/test/repository.test.ts:143-251`）との調整）は `02_architecture.md` で決める。設計へ引き継ぐ論点は [design_handoff.md](design_handoff.md) に申し送る。

### issue #1073 で決めたこと

-   **スキップ方式:** `[skip ci]` / `[no ci]` は使わず、`.github/workflows/ci.yml` の `check-changes` で重いジョブ（`test`・`lint`・`extension`）だけをスキップし、`doc-guard` と `secret-scan` は通常どおり実行する。`mergepr`・`/runpr` の保証とプロトコル番号を変えず、必須ステータスチェックとも両立させるためである。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md) を作る（要件定義の際に決めたことの ADR の項を参照）。
-   **スキップの 4 条件の骨格:** (1) PR の head コミットがマージコミットではない（親コミットがちょうど 1 つ）、(2) 親コミットとの差分が既存の分類規則で非コードかつ非拡張のファイルのみ、(3) 親コミットに対する直前の CI で `test`・`lint`・`extension` がすべて `success` または `skipped` で完了している、(4) ベースブランチ（`main`）の変更などで前提が崩れていない、の 4 つをすべて満たすときにスキップする。必要な情報を取得できない場合や API 呼び出しが失敗した場合は、安全側に倒して通常どおり実行する。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。
-   **権限の最小化:** 親コミットのチェック結果の確認に必要な読み取り権限は、最小限の範囲で `check-changes` ジョブに付与することを検討する。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。

### 要件定義の際に決めたこと（2026-11-21）

-   **進め方の選択:** 通常の進め方（`full`）とする。除外判定は「除外に当たらない」（`docs/dev/security.md` の規則・前提（[ADR-0006](../../dev/adr/0006-threat-model.md)）を変えず、[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) の開発ツールの信頼境界にも触れず、外部依存の追加・変更もない）。そのうえで、CI の誤スキップを防ぐ新しい仕組みであり、GitHub API の信頼境界と `pull_request` 合成マージコミットの整合性を設計の panel レビューで確かめる価値が高いため、通常の進め方を選んだ。人の答え：通常の進め方（`full`）。
    -   ADR：なし。本タスクの中で閉じる進め方の選択である。
-   **スキップの単位:** Go 側（`test`・`lint`）と拡張側（`extension`）をまとめて判定する（F-001）。親コミットとの差分が非コードかつ非拡張のファイルのみであり、かつ親コミットの CI で 3 ジョブすべてが通過済みのときだけ、`has-code-changes=false` と `has-extension-changes=false` を両方適用する。系統ごとに別々に判定すると条件分岐と境界ケースが増えるため、issue #1073 の提案どおり単純な形にした。人の答え：まとめて判定する。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。
-   **親コミットの CI 結果の条件と手動再実行:** 親コミットの CI 結果はワークフロー全体の成否ではなくジョブ単位（`check-changes` が `success`、かつ `test`・`lint`・`extension` がすべて `success` または `skipped`）で判定し、連続する文書のみ push でのスキップの連鎖を許す（F-002）。また、手動の再実行（`run_attempt > 1`）時はスキップを適用せず通常の全判定を行う（F-003）。`doc-guard` の失敗を文書修正で直したときや、`/fixpr` の文書修正の後に `PRE_MERGE` を積んだときにも無駄な再実行を避けつつ、GitHub の「Re-run all jobs」で重いジョブを強制的に走らせ直す逃げ道を残すためである。人の答え：ジョブ単位・連続 push の連鎖許可・再実行時はスキップしない。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。
-   **フォークからの PR の扱い:** 同一リポジトリ内の PR のみをスキップ対象とし、フォークからの PR ではスキップしない（F-003）。別リポジトリ由来のコミットやワークフロー実行との取り違えリスクを構造的に排除でき、本リポジトリの通常運用に影響しないためである。人の答え：同一リポジトリの PR のみスキップ対象。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)。
-   **空コミットの扱い:** 親コミットとの差分が 0 件の空コミット（例: `simple-process.md:450-452` の仕上げコミット）も、非コードかつ非拡張のみの変更としてスキップ対象に含める（F-001）。強制再実行の逃げ道は手動再実行（`run_attempt > 1`）で確保できるためである。人の答え：スキップ対象に含める。
    -   ADR：なし。本タスクの中で閉じるエッジケースの振る舞いである。
-   **ADR と更新文書:** 新規 ADR として [ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md) を作成し、ADR の索引（`docs/dev/adr/README.md`）に 1 行を加える（F-005）。`CLAUDE.md` や既存の ADR（[ADR-0016](../../dev/adr/0016-document-test-policy.md) 等）は変更しない。CI の重いジョブをスキップする方針と安全条件は今後の CI 変更やジョブ追加を縛る開発プロセスの方針であり主要な決定に当たる一方、既存の ADR とは主題が別であり、方針の正本を新規 ADR に 1 か所化するためである。人の答え：新規 ADR と ADR 索引のみ。
    -   ADR：[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md) を作る。
-   **GitHub Actions での実機確認:** GitHub Actions 上での動作（コード変更後の通常実行、続く文書のみコミットでの重いジョブのスキップと `doc-guard`・`secret-scan` の継続実行）を実機で確認し、結果を実装計画書に記録することを要件に含める（F-006）。GitHub Actions はローカルで再現できないため、マージ前に実動作の証跡を残す。人の答え：GitHub Actions 実機確認を要件に含める。
    -   ADR：なし。本タスクの検証手順の選択である。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   コード変更を含む PR に `PRE_MERGE` コミットや文書のみの修正コミットを追加 push したとき、直前の親コミットで必要な重いジョブが通過済みであれば、`test`・`lint`・`extension` をスキップして CI の待ち時間と実行コストを削減する。
-   スキップ時も `doc-guard` と `secret-scan` は通常どおり実行し、文書編集ミスや秘密鍵の混入を検知し続ける。
-   `mergepr` と `/runpr` の保証・プロトコル番号、および必須ステータスチェックとの両立を維持する。
-   前提が崩れた場合や判定に必要な情報を確かめられない場合は、必ず安全側（通常実行）に倒し、未検証コードのマージを防ぐ。

### 2.2. スコープ (In Scope)

-   PR の head コミットの親の数と親コミットとの変更ファイル分類に基づくスキップ適格判定（F-001）。
-   親コミットに対する直前の CI 結果の検証（ジョブ単位・連続 push の連鎖を含む）（F-002）。
-   ベースブランチ（`main`）の整合性、同一リポジトリ判定、手動再実行時のスキップ無効化、および失敗・不備時の安全側フォールバック（F-003）。
-   `check-changes` の出力制御、最小読み取り権限の付与、および常時実行ジョブ（`doc-guard`・`secret-scan`）と既存ジョブ構造の維持（F-004）。
-   新規 ADR（[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)）と ADR 索引の作成・更新、および実装時の食い違いの記述の除去（F-005）。
-   GitHub Actions 上での実機確認と結果の記録（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   `[skip ci]` や `[no ci]` によるワークフロー全体のスキップ（issue #1073 で不採用と決定）。
-   `mergepr`・`/runpr`・`fixprguard` の仕様変更およびプロトコル番号の引き上げ。
-   既存のファイル分類規則（`*.md`・`docs/`・`.claude/`・`LICENSE` を非コードとする規則、および `extension/scripts/has-extension-changes.sh`・`extension/scripts/wasm-dep-dirs.sh` の拡張対象規則）そのものの変更。
-   Go 側（`test`・`lint`）と拡張側（`extension`）を系統別に独立してスキップすること（要件定義の際に決めたこと）。
-   1 回の push で複数コミットをまとめて積んだときに、親コミット（`HEAD~1`）よりさらに前の祖先まで遡って CI 結果を探すこと。
-   `CLAUDE.md` や既存 ADR（[ADR-0016](../../dev/adr/0016-document-test-policy.md) 等）の更新（要件定義の際に決めたこと）。
-   新しい外部 GitHub Action や外部ライブラリ（Go モジュール・npm パッケージ）の追加。
-   GitHub リポジトリ側の必須ステータスチェック設定の変更（リポジトリ設定の手作業）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

以下の要件で「PR head コミット」とは、PR ブランチの先端コミット（GitHub Actions の `pull_request` イベントが示す PR の head コミット）を指し、「親コミット」とはその第 1 親（1 つ前のコミット）を指す。`actions/checkout` が `pull_request` でチェックアウトする合成マージコミットと混同しない（§6 用語集）。

#### F-001: コミット構造と差分の分類によるスキップ適格判定

`check-changes` ジョブは、既存の PR 全体（`origin/main...HEAD`）に対する分類結果のいずれかが `true` である場合（すでに両方 `false` ならスキップ判定は不要）に限り、PR head コミットがスキップ適格であるかを次の 2 条件で判定する。

-   **親コミットの数:** PR head コミットの親コミットがちょうど 1 つであること。マージコミット（親が 2 つ以上）およびルートコミット（親が 0 個）はスキップ対象外とする。
-   **親コミットとの差分:** 親コミットから PR head コミットまでの変更ファイル一覧が、既存の分類規則において**非コードかつ非拡張**のファイルのみであること。
    -   変更ファイルが 0 件（空コミット）の場合は、この条件を満たす。
    -   ファイル名の変更（リネーム）は、変更前と変更後の両方のパスを検査する（コードまたは拡張のファイルを文書配下へリネームした場合、変更前のパスがコード・拡張に当たるためスキップ対象外とする）。
    -   非 ASCII 文字や制御文字（タブ・改行等）を含むパスでも、クォートや行分割によって非コード・非拡張と誤判定されないこと。
    -   `README.md`・`README.en.md`・`docs/dev/guides/development.md`・WASM 依存パッケージ配下のファイル（例: `internal/llm/devfake/articles/*.md`）のように、非コードであっても既存の拡張分類規則（`extension/scripts/has-extension-changes.sh:45-55`）で拡張対象とされるファイルが 1 つでも含まれる場合は、スキップ対象外とする。

**Acceptance Criteria**:
- **AC-01**: PR head コミットの親コミットが 2 つ以上（マージコミット）または 0 個の場合、変更ファイルの内容や親コミットの CI 結果にかかわらず、重いジョブはスキップされない（PR 全体 `origin/main...HEAD` の通常分類の結果が出力される）。
- **AC-02**: PR head コミットの親コミットがちょうど 1 つで、親コミットとの差分が非コードかつ非拡張のファイル（例: `docs/tasks/...`・`.claude/...`・`LICENSE`・`docs/dev/adr/...`）のみ、または変更ファイル 0 件（空コミット）の場合、F-001 の条件を満たす。
- **AC-03**: 親コミットとの差分に、コードファイル（例: `.go`・`.github/workflows/ci.yml`・`Makefile`）、または既存の拡張分類規則で拡張対象となるファイル（例: `extension/` 配下、`README.md`、`README.en.md`、`docs/dev/guides/development.md`、WASM 依存パッケージ配下のファイル）が 1 つでも含まれる場合、重いジョブはスキップされない。
- **AC-04**: 親コミットとの差分に、コードまたは拡張対象ファイルから文書パスへのリネーム（例: `internal/foo/a.go` → `docs/a.md`）、または非 ASCII 文字・タブ・改行を含むコード／拡張対象パスが含まれる場合、非コード・非拡張のみとは判定されず、重いジョブはスキップされない。

#### F-002: 親コミットの直前 CI 結果の検証

`check-changes` ジョブは、F-001 を満たす場合、親コミットに対する本リポジトリの `CI` ワークフロー（`.github/workflows/ci.yml`、`pull_request` イベント）の直前の実行結果を取得し、必要なジョブが通過済みであることを検証する。

-   **検証対象:** 親コミットに対する本リポジトリの `CI` ワークフロー（`pull_request`）の最新の実行。別のワークフロー（例: `deepseek-review.yml`）や別リポジトリの実行結果、古い実行結果は使わない。
-   **通過済みの条件:** 最新の実行が完了しており（実行中・待機中ではない）、次のジョブがすべて存在し、それぞれの結論が条件を満たすこと。
    -   `check-changes`（表示名 `Check changed files`）が `success` であること。
    -   `test`（`Test`）、`lint`（`Code Quality & Lint`）、`extension`（`Browser extension`）の 3 ジョブすべてが `success` または `skipped` であること。
-   **`doc-guard` / `secret-scan` の結論は問わない:** 親コミットで `doc-guard` または `secret-scan` が失敗していても、上の 4 ジョブが条件を満たしていれば通過済みとする（今回の PR head コミットで `doc-guard` と `secret-scan` が改めて実行されるため）。
-   **連続 push の連鎖:** 親コミット自身が前のコミットからスキップされた結果 `test`・`lint`・`extension` が `skipped` であっても、`check-changes` が `success` で F-003 の前提条件も満たすなら、引き続きスキップできる。
-   **複数コミットの一括 push:** 1 回の push で複数コミットが積まれ、親コミットに該当する CI 実行が存在しない場合は、さらに前の祖先を辿らず、通過済みではないとして通常実行する。

**Acceptance Criteria**:
- **AC-05**: 親コミットに対する本リポジトリの `CI` ワークフロー（`pull_request`）の最新実行において、`check-changes` が `success` かつ `test`・`lint`・`extension` がすべて `success` または `skipped` の場合（`doc-guard` または `secret-scan` の成否を問わない）、F-002 の条件を満たす。
- **AC-06**: 親コミットの最新 `CI` 実行において、`test`・`lint`・`extension` のいずれかが `failure`・`cancelled`・`timed_out`・`action_required`・`neutral`・`stale` または未完了（`in_progress`・`queued` 等）である場合、`check-changes` が `success` でない場合、または必要なジョブの記録が 1 つでも欠けている場合、重いジョブはスキップされない。
- **AC-07**: 親コミットに本リポジトリの `CI` ワークフロー（`pull_request`）の実行が存在しない場合（初回コミット、複数コミットの一括 push の中間コミット等）、または最新より古い実行だけが成功していて最新実行が失敗・未完了である場合、重いジョブはスキップされない。
- **AC-08**: 親コミット自身がスキップ済みのコミット（`check-changes` が `success`、`test`・`lint`・`extension` がすべて `skipped`）であり、今回の追加コミットも非コード・非拡張のみである場合、F-003 の前提条件を満たす限り重いジョブは再びスキップされる。

#### F-003: 前提条件の整合性確認・手動再実行の逃げ道・安全側フォールバック

`check-changes` ジョブは、親コミットの CI 結果が今回の PR head コミットにそのまま成り立つ前提が崩れていないことを確かめ、1 つでも崩れているか確かめられない場合は安全側に倒して通常どおり重いジョブを実行する。

-   **同一リポジトリ・対象ベースブランチ:** PR の head と base が同一リポジトリであり、かつ base ブランチが `main` であること。フォークからの PR ではスキップしない。
-   **手動再実行の逃げ道:** ワークフローの実行回数（`run_attempt`）が 2 以上（例: GitHub UI からの「Re-run all jobs」）のときは、スキップ判定を適用せず PR 全体（`origin/main...HEAD`）の通常分類を使う。
-   **ベースブランチ（`main`）の不変性:** 親コミットの CI 実行が検証した `main` の状態から、今回の実行時点の `main` が進んでいないこと。親コミットの CI 実行以降に `main` へ新たなコミットが入った場合、またはその整合性を確実に検証できない場合はスキップしない。
-   **安全側フォールバック（fail-safe）:** GitHub API の呼び出し失敗（ネットワーク障害・認可エラー・レート制限・タイムアウト・想定外の HTTP ステータス）、応答の解析失敗や規約違反（§3.2）、必要な git 情報の取得失敗など、スキップ判定の途中で何らかの異常が起きた場合は、`check-changes` ジョブ自体を失敗させるのではなく、スキップ不可として通常の `origin/main...HEAD` 分類結果をそのまま出力する。また、スキップしたか通常実行したかとその理由をジョブのログに出力し、後から確認できるようにする。

**Acceptance Criteria**:
- **AC-09**: フォークからの PR（head リポジトリと base リポジトリが異なる、またはリポジトリ情報を取得できない場合）では、ほかの条件をすべて満たしていても重いジョブはスキップされない。
- **AC-10**: 手動再実行（`run_attempt` が 2 以上）では、ほかの条件をすべて満たしていてもスキップが適用されず、`origin/main...HEAD` の通常分類結果が出力される。
- **AC-11**: 親コミットの CI 実行が検証した `main` の状態と今回の実行時点の `main` が異なる場合（`main` に新しいコミットが追加された場合）、または `main` が変わっていないことを確かめられない場合、重いジョブはスキップされない。
- **AC-12**: スキップ判定で用いる外部 API 呼び出し・応答の検査・git 参照のいずれかが失敗した場合、`check-changes` ジョブは異常終了せず、PR 全体（`origin/main...HEAD`）に対する通常分類の結果を出力して安全側に倒す。ただし通常の拡張分類で `wasm-dep-dirs.sh` が失敗したときの既存フォールバック（`has-extension-changes=true`、`.github/workflows/ci.yml:67-70`）はそのまま維持する。
- **AC-13**: `check-changes` ジョブのログに、スキップを適用したか通常分類を使ったか、およびスキップしなかった場合の理由（マージコミット、コード／拡張変更あり、親 CI 未通過、`main` 更新、フォーク PR、手動再実行、取得・検査失敗など）が出力される。

#### F-004: ジョブ出力・常時実行ジョブ・最小権限と既存範囲の維持

-   **出力とスキップ動作:** F-001〜F-003 をすべて満たすとき、`check-changes` は `has-code-changes=false` および `has-extension-changes=false` を出力する。これにより `test`・`lint`・`extension` は `skipped` となる一方、`doc-guard` と `secret-scan` は `check-changes` に依存せず通常どおり実行される（`.github/workflows/ci.yml:201-234`）。
-   **権限の最小化:** ワークフロー全体の `permissions`（`contents: read`、`.github/workflows/ci.yml:7-8`）は広げない。親コミットの CI 結果確認に必要な読み取り権限は `check-changes` ジョブのみに限定して付与し、書き込み権限はいずれのジョブにも付与しない。
-   **既存ワークフロー構造の維持:** `test`・`lint`・`extension`・`doc-guard`・`secret-scan` のジョブ定義・実行条件・ステップは変更しない。

**Acceptance Criteria**:
- **AC-14**: F-001〜F-003 をすべて満たす場合、`check-changes` は `has-code-changes=false` と `has-extension-changes=false` の両方を出力し、`test`・`lint`・`extension` がスキップされる。このときも `doc-guard` と `secret-scan` は通常どおり実行される。
- **AC-15**: `.github/workflows/ci.yml` のワークフロー全体の `permissions` は `contents: read` のままであり、追加される権限は `check-changes` ジョブに限定された読み取り権限（`: read`）のみである。書き込み権限（`: write`）はいずれのジョブにも付与されない。
- **AC-16**: 実装 PR における `.github/workflows/ci.yml` の差分は `check-changes` ジョブの定義（権限とステップ）に限られ、`test`・`lint`・`extension`・`doc-guard`・`secret-scan` の各ジョブ定義およびトリガ（`on:`）・ワークフロー全体の `permissions:` は変更されない。
- **AC-17**: 既存の CI 関連テスト（`make test`・`make lint`・`make ext-test`、特に `extension/test/repository.test.ts`・`extension/test/ciChanges.test.ts`・`internal/transcript/makefile_test.go`）がすべて通る。

#### F-005: 新規 ADR（ADR-0054）の作成と整合

-   CI で直前コミット通過済みかつ非コード・非拡張のみの変更時に重いジョブをスキップする方針と安全条件を [ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md) として作成し、`docs/dev/adr/README.md` に番号順で 1 行追加する。
-   本要件定義書の PR 時点では、`.github/workflows/ci.yml` がまだ未実装である旨の食い違いの記述を ADR-0054 の `## 決定` に置き、issue #1073 の完全な URL にリンクする（`docs/dev/conventions/adr.md`「Writing an ADR」「Discrepancy statements」）。
-   実装 PR でこの食い違いを解消し、同じ実装 PR で ADR-0054 から食い違いの記述を取り除く。

**Acceptance Criteria**:
- **AC-18**: `docs/dev/adr/0054-ci-skip-doc-only-commit.md` が存在し、状態が `有効` で、`docs/dev/adr/README.md` に番号順で索引行があり、`TestADRRecords` が通る。
- **AC-19**: 実装 PR のマージ後、ADR-0054 の `## 決定` は実装済みの振る舞いと一致し、未実装を示す食い違いの記述が残っていない。

#### F-006: GitHub Actions での実機確認と記録

GitHub Actions 上での挙動を実機で確認し、確認日・PR 番号・ワークフロー実行の URL と結果を `03_implementation_plan.md` の手動確認の節に記録する。

**Acceptance Criteria**:
- **AC-20**: GitHub Actions 上で次の 2 つのケースを実機確認し、それぞれの実行 URL とジョブ結果（`check-changes`・`test`・`lint`・`extension`・`doc-guard`・`secret-scan`）を `03_implementation_plan.md` に記録している。
    1.  コード変更を含むコミットの push で `test`・`lint`（および該当すれば `extension`）と `doc-guard`・`secret-scan` が通常どおり実行されて成功すること。
    2.  その直後に非コードかつ非拡張のファイルのみを変更するコミットを追加 push したとき、`check-changes` が `has-code-changes=false`・`has-extension-changes=false` を出力して `test`・`lint`・`extension` が `skipped` になり、かつ `doc-guard` と `secret-scan` が実行されて成功すること。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

`check-changes` ジョブは、外部 API（GitHub API）の応答および git コマンドの出力を読み取ってスキップ可否を判定する。これらを信頼できない入力として境界で厳密に検査し、標準ライブラリやパーサが黙って補正・破棄・無視しうる不正な入力を補正せず拒否する。

| 境界 | 受理する形 | 上限 | 消費するメンバと拡張性 | 失敗・拒否時の扱い | 主な AC |
|---|---|---|---|---|---|
| GitHub API 応答（親コミットの CI 実行・ジョブ情報、ベース整合性情報） | 有効な UTF-8 の JSON 文書で、単一のトップレベル値を持ち末尾に余分なデータがないこと。設計（`02_architecture.md`）で定める消費メンバがすべて存在し、期待する型・列挙値（結論・状態）・形式（40 桁小文字 16 進のコミット SHA 等）に一致すること。重複キー・欠損・型不一致・不正な UTF-8・切り詰めを補正せず拒否する | 応答サイズ・ページ数／件数の上限を設ける（具体的な上限値は `02_architecture.md` で定める） | 設計で定めるエンドポイントごとの消費メンバのみを使用する。GitHub API は応答に多数の付加フィールドを含み将来の追加もあるため、宣言した消費メンバ以外の未消費メンバは**拡張可能**（無視して受理）と宣言する。一方、文書全体の UTF-8・単一トップレベル値・末尾データなし・重複キーの検査は応答全体に適用する | すべての拒否（番兵エラーを設ける場合は設計で定める拒否種別）を、単一の安全側判定「スキップ不可（PR 全体の通常分類へフォールバック）」に写像する。部分受理や補正を行わず、エラー詳細にトークン等の秘密を含めない | AC-05〜AC-08・AC-11・AC-12 |
| git コマンド出力（親コミット一覧、`HEAD~1..HEAD` の変更ファイルパス一覧、ベース参照） | 終了コード `0` であること。コミット OID は 40 桁小文字 16 進文字列。変更ファイルパス一覧は `core.quotePath=false`・`--no-renames`・NUL 区切り（`-z`）の生のパス列とし、空白・非 ASCII・制御文字・リネーム前後を損なわずに読み取る | 変更ファイル数の上限を設けるかは `02_architecture.md` で定める（上限を超える場合は安全側へ倒す） | 固定形式の行・NUL 区切り要素のみを消費する（拡張可能と宣言するメンバなし） | 終了コード非ゼロまたは想定外の形式はすべて「スキップ不可（PR 全体の通常分類へフォールバック）」に写像する | AC-01〜AC-04・AC-12 |

-   GitHub Actions のコンテキスト値（PR 番号、head/base リポジトリ、base ブランチ名、`run_attempt`、head SHA 等）は、環境変数経由で受け渡し、シェル展開によるインジェクションを起こさない形で扱う（具体的な受け渡し方法は `02_architecture.md` で定める）。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   スキップ判定（親コミットの確認・差分分類・GitHub API 照会）は、`test`・`lint`・`extension` の再実行時間に比べて十分に短い時間（数秒〜十数秒程度）で完了すること。API 呼び出しにはタイムアウトを設け、応答遅延でジョブが無制限に待たないようにする。

### 4.2. セキュリティ (Security)

-   条件付きガイド（`docs/dev/security.md`）のトリガ（外部コマンド `yt-dlp` の起動、LLM の API キー、キャッシュディレクトリ、LLM API へのネットワーク送信、信頼できないテキストのプロンプトまたは記事への埋め込み）には該当しないため、同ガイドの適用は **N/A** である。
-   CI 固有のセキュリティ要件として次を守る。
    -   **最小権限:** 追加する権限は `check-changes` ジョブ限定の読み取り権限のみとし、ワークフロー全体や他ジョブの権限を広げない。書き込み権限を付与しない（AC-15）。
    -   **フォーク PR の排除:** フォークからの PR ではスキップを適用しない（AC-09）。
    -   **秘密の非出力:** `GITHUB_TOKEN` 等の認証情報をログやエラー出力に出さない。
    -   **インジェクション防止:** PR タイトル・ブランチ名・コミットメッセージ・ファイルパス等の外部由来の文字列を、シェルコマンドや YAML の `${{ ... }}` 直接展開で実行しない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   すべての不確実な状況（API 失敗、レート制限、応答の形式不備、ジョブ欠損、`main` 更新、複数コミット一括 push、未完了の親 CI）は安全側（スキップ不可＝通常実行）に倒し、誤スキップ（未検証コードや古い `main` との合成結果が CI を通過すること）を防ぐ（AC-06・AC-07・AC-11・AC-12）。
-   `check-changes` ジョブ自体は、API 障害などスキップ判定側の失敗で赤にならない（通常分類で継続し、CI 全体の可用性を損なわない）（AC-12）。

### 4.4. 互換性 (Compatibility)

-   `mergepr` の保証（CI 完了待ち、`--match-head-commit` によるマージ、チェック 0 件での停止、`docs/dev/guides/mergepr_guide.md:30`・`:265`）および `/runpr` の `PRE_MERGE` 手順（`.claude/commands/runpr.md:1617-1649`）とそのまま互換であること。両ツールの変更やプロトコル番号の引き上げを要しない。
-   GitHub の必須ステータスチェックにおいて、ジョブが `skipped` となった場合の挙動（条件付き `if:` でスキップされたジョブが必須チェックを通過する既存の `test`・`lint`・`extension` の仕組み）をそのまま維持する。
-   既存の `check-changes` の出力名（`has-code-changes`・`has-extension-changes`）および値（`true` / `false`）を変えない。

### 4.5. 保守性 (Maintainability)

-   スキップ判定のロジックは、ユニットテストで各条件（正常スキップ、マージコミット、リネーム、非 ASCII／制御文字パス、拡張対象ファイル、親 CI の各種結論・欠損・未完了、`main` 更新、フォーク PR、手動再実行、API 失敗・不正応答）を独立に検証できる構成にする（実装手段は [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) を踏まえて `02_architecture.md` で決める）。
-   将来 `.github/workflows/ci.yml` に新しい重いジョブ（`check-changes` の出力で制御されるジョブ）を追加するときは、親コミットの通過済み確認の対象ジョブ一覧にも同じ PR で追加する必要がある（[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)「結果」）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。新しい外部ライブラリ・新しい GitHub Action を追加しない。
-   `CLAUDE.md` の「Declare, don't infer」に従い、スキップ可否をコミットメッセージの文字列（`[skip ci]` や `PRE_MERGE` の文言等）で判定しない。判定は親コミット数・変更ファイルパス・GitHub API が宣言するジョブ結論等の構造化された事実のみに基づく。
-   `mergepr`・`/runpr`・`fixprguard` を変更せず、プロトコル番号を上げない。
-   `docs/dev/conventions/requirements_process.md` の通常の進め方に従い、`01_requirements.md` の承認後に `02_architecture.md`、`03_implementation_plan.md`、実装の順に進める。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **重いジョブ:** `.github/workflows/ci.yml` のうち、`check-changes` の出力で実行を制御される `test`・`lint`・`extension` の 3 ジョブ。
-   **常時実行ジョブ:** `check-changes` の出力に依存せず、すべての PR で常に実行される `doc-guard` と `secret-scan` の 2 ジョブ（`.github/workflows/ci.yml:201-234`）。
-   **`PRE_MERGE` コミット:** `/runpr` のマージ直前手順（`.claude/commands/runpr.md:320-378`）で、文書の承認記録・実装計画書のチェックボックス更新・変更履歴の `#未記入` の埋め込み・`diff` 記録の再記録をまとめて記録するコミット。
-   **PR head コミット:** PR ブランチの先端のコミット。GitHub Actions の `pull_request` イベントでは、`actions/checkout` がチェックアウトする合成マージコミット（`refs/pull/<N>/merge`、親が `base` と PR head の 2 つ）とは別に、`github.event.pull_request.head.sha` が指す実コミットである。
-   **親コミット:** PR head コミットの第 1 親（1 つ前のコミット）。
-   **通常分類:** 既存の `check-changes` が行う、PR 全体の差分（`origin/main...HEAD`）に対する `has-code-changes` と `has-extension-changes` の分類。
-   **安全側フォールバック（fail-safe）:** スキップ条件を満たすことを確実に証明できない場合（前提崩れ・情報取得失敗・形式不備等）に、スキップせず通常分類の結果を採用して重いジョブを実行すること。
-   **食い違いの記述:** ADR の決定と、その仕様を持つ文書やコードが食い違うときに、`## 決定` に残す記述（`docs/dev/conventions/adr.md`「Writing an ADR」）。
