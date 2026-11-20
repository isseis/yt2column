# 実装計画書：Codex の PR レビューと Codex CLI をレビューの体制から外す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-20 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

**目的:** [01_requirements.md](01_requirements.md)（以下、要件定義書）の F-002〜F-012 と、[02_architecture.md](02_architecture.md)（以下、設計書）の設計に従い、Codex の PR レビューと Codex CLI への依存をリポジトリから外し、手動の CodeRabbit PR レビューと CodeRabbit CLI によるローカルレビューへ移行する。F-001（AC-01〜AC-05）は [observations.md](observations.md) と設計書の PR（#1070）で完了済みである。

**実装方針:**

-   設計書 §6.3・§8 の優先順位どおり、フェーズ 1（ローカルの CLI レビュー・PR 前のレビュー・`codexreview` 撤去）→ フェーズ 2（`fixprguard` 拡張・PR レビュー手動化・メンション拒否・自動レビュアー一覧更新）→ フェーズ 3（残りの文書・ADR・用語集・chief 確認・フォローアップ issue）の 3 段階で進める。
-   移行の順序（要件定義書 4.4、AC-45）を守る。フェーズ 2 のマージ直後に利用者が Codex の GitHub 連携を切り契約を終えるため、フェーズ 3 はその後に進める。
-   `fixprguard` のプロトコル番号は `main` の値 + 1 へ上げ、文書とテストの `--protocol` をすべて同じ変更でそろえる（設計書 3.3.5）。
-   テストの削除は設計書 7.4 に従う。`cmd/codexreview` と `internal/codexreview` は製品コードとテストを 1 つの commit で消し、残るパッケージのテスト削除は削除だけの commit に分けて `make coverage-compare BASE=<親>` で確認する。
-   Go のソースコード（`*_test.go`・`test_helpers.go` を含む）のコメント・識別子・文字列リテラルは英語で書く。
-   設計の詳細は設計書の各節を参照し、本書には重複して書かない。

### 1.1 既存コード調査結果

`main` の `dc224b5c` を読んだ結果である。`implementation_handoff.md` は存在しない。`design_handoff.md`（H-01〜H-09）はすべて設計書 付録 A に記録されている。

-   **`_lib/task-pr-handoff.md` Part B（`dc224b5c`）:** タスク 0114 の step 2.5（Entry 4）はまだ `main` に入っていない。Part B は step 1〜7 で構成され、step 2（CHECKS、`:207`）の直後に step 3（push と PR 作成、`:209`）がある。`cmd/yt2column/codex_review_docs_test.go:76` は `3. Push and create the PR for the current branch.` から `\n4. Run AFTER_CREATE` の範囲を切り出している。
-   **`codexreview` と関連するガード・設定（`dc224b5c`）:**
    -   `cmd/codexreview/`、`internal/codexreview/`（`testutil/` に `helpers.go`・`helpers_test.go` の 2 ファイル）。
    -   `internal/pipeline/pipeline_test.go:578`: `TestFakesCarryBuildTag` が `len(testutilFiles) != 24` を固定値で検証している。`internal/codexreview/testutil/` の 2 ファイルを消すと 22 になるため、同じ変更で更新する必要がある。
    -   `internal/pipeline/pipeline_test.go:629-650`・`internal/pipeline/strictjson_users_test.go:204`: `docs/dev/conventions/package_reference.md` のパッケージ一覧と `internal/strictjson` の利用元を検証している。`package_reference.md:109,117,124,135` の `codexreview` 関連の記述は `codexreview` 撤去と同じ変更で外す必要がある。
    -   `internal/doctestutil/protocol.go:19,24`（`ToolCodexreview`・`protocolTools`）、`internal/doctestutil/protocol_test.go`、`internal/doctestutil/commands.go:118-119,200`（コメントの `codex` 例。`RunsCommand` 自体は `cmd/chief/docs_test.go:305` が `herdr` で使うため残す）。
    -   `internal/mergepr/protocolcheck_test.go:141`: `TestMismatchRemedy` が `[]string{"fixprguard", "codexreview", "chief", "mergepr"}` を持つ。
    -   `cmd/fixprguard/docs_test.go:403-404`: `TestFixprguardProtocolMismatches` が `codexreview` のケースを持つ。
    -   `internal/config/envaccess_test.go:76-79`、`internal/cmdrunner/runner.go:3`、`cmd/coveragecompare/main.go:27,64` に `codexreview` を名指す行・コメントがある。
    -   `Makefile:53,73-77`（`install-codexreview`）、`.claude/settings.json:5`（`Bash(codexreview *)`）、`.github/workflows/ci.yml:215`（`doc-guard` の `./cmd/codexreview`）、`CLAUDE.md:189,461-473,766,809,816,849,1085-1113`。
-   **`fixprguard` の再利用対象と変更対象（`dc224b5c`）:**
    -   再利用: `internal/fixprguard/github.go` の `runGH`・`resolveRepository`・`graphql`・`strictjson` 呼び出し、`internal/fixprguard/limits.go:281` の `parseDateHeader` と `splitIncluded`・`InteractionState` の `MarshalText` パターン。
    -   プロトコル番号: `internal/fixprguard/types.go:21` に `Protocol` 定数がある。
    -   メンション拒否: `internal/fixprguard/errors.go:56-58,72`（`errCodexMention`・`CodexMention`）、`internal/fixprguard/write.go:76-95,161,339,377,390,404`（`checkPostText` と呼び出し元）、`internal/fixprguard/worktree.go:331`、`cmd/fixprguard/main.go:303-320`（`stop`）。`AppendPR` の `checkPostText(body)`（`write.go:404`）は現在 `errCodexMention`（`ErrInvalidInput` 包装、終了コード 2）を返すため、`ErrCurrentBodyMention`（`ErrInvalidInput` に包まない、`StopCurrentBodyMention = "current-body-mention"`、終了コード 1）へ分ける。
    -   テストヘルパ: `internal/fixprguard/test_helpers.go:24` と `cmd/fixprguard/test_helpers.go:25` に `testSecondReviewer = "chatgpt-codex-connector"`、`internal/fixprguard/types_test.go:51` と `trust_test.go:155` に `"chatgpt-codex-connector"` がある。
    -   `internal/fixprguard/testdata/` は現在存在しない。`review-state` のサンプル用に新設する。
-   **文書ガードテスト（`dc224b5c`）:**
    -   `cmd/yt2column/codex_review_docs_test.go`: `TestCodexMentionOnlyInRequestRow`、`TestPostingPathsCheckMention`、`TestPostingCommandsInventory`、`TestCodeRabbitConfigStopsReview`、`TestAgentsCodeReviewRules`。
    -   `cmd/yt2column/fixpr_docs_test.go`: `wantReviewers`（`:62`）、`TestRunprDocContract`（`:158`）、`TestHandoffListingPrintsNoBody`（`:196`）、`TestLocalReviewGuard`（`:1293`）。
    -   `cmd/yt2column/review_records_test.go`: `seenRangeForm`（`:83`）、`TestReviewRecordsCoverage`（`:652`）、`TestReviewRecordsCallSite`（`:1234`）、`TestRecordLinePrefixesDistinct`（`:1318`）。
    -   `cmd/fixprguard/docs_test.go`: `TestGuideCoversStopKindsAndScope`（`:52`）、`TestGuideListsInteractionLimitsValues`（`:82`）、`TestDocumentedInvocationsParse`（`:212`）、`TestDocumentedProtocolMatches`（`:351`）、`TestGuideDescribesMentionCheck`（`:424`）。
-   **ADR と用語集（`dc224b5c`）:**
    -   `食い違い:` 記述がある ADR: `0013`（`:19`）、`0014`（`:49`）、`0015`（`:27`）、`0042`（`:37`）、`0047`（`:21`）、`0050`（`:38`）。
    -   Codex に触れる残りの ADR: `0040`（`:11`）、`0046`（`:284,473`）。
    -   `docs/translation_glossary.md:119` に「試行」、`.coderabbit.yaml:9-12` にコメントがある。

### 1.2 計画の際に決めたこと（2026-11-20）

利用者に尋ねるべき未決の計画判断はなかった（設計書と要件定義書で主要な決定は確定済み）。設計書が計画書に委ねた細部と計画上の配置を次のとおり決めた。

-   **`_lib/task-pr-handoff.md` Part B の新しい手順の番号を `2.5.` とする:** 0114 は `dc224b5c` に入っておらず、既存の step 3・step 4 の番号と `codex_review_docs_test.go`・他文書からの `Part B step 3` 参照を保てるためである。ADR の判断: ADR なし（文書内の手順番号であり、タスク内で閉じるため）。
-   **`review-state` の配置を `internal/fixprguard/review_state.go`・`review_state_test.go`・`testdata/` とする:** 既存の `limits.go`・`limits_test.go` と同じ構成にそろえるためである。ADR の判断: ADR なし（ファイル配置であり、タスク内で閉じるため）。
-   **`codexreview` 撤去の静的検証スクリプトを `docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` に置く:** `Makefile`・`.claude/settings.json`・Go ソースコメントからの残存参照の不在（AC-33・AC-34・AC-36）を文書中の生コマンドではなく実行可能なスクリプトで検証するためである。ADR の判断: ADR なし（タスク専用の検証スクリプトであり、タスク内で閉じるため）。

### 1.3 リネーム一覧

本タスクで改名するファイル・識別子・テスト関数をここに集約する。文書内のすべての参照はこの表に従う。

| 旧名 | 新名 | 対象ファイル | フェーズ |
|---|---|---|---|
| `cmd/yt2column/codex_review_docs_test.go` | `cmd/yt2column/review_mention_docs_test.go` | `cmd/yt2column/`（改名だけの commit を分ける） | フェーズ 2 |
| `errCodexMention` | `errMention` | `internal/fixprguard/errors.go`・`write.go`・テスト | フェーズ 2 |
| `CodexMention`（単一定数） | `RejectedMentions()`（`@codex`・`@coderabbitai` を返す関数、または対応する定数群） | `internal/fixprguard/errors.go`・`write.go`・テスト・文書ガード | フェーズ 2 |
| `TestCodexMentionOnlyInRequestRow` | `TestMentionsOnlyInAllowedRows` | `cmd/yt2column/review_mention_docs_test.go` | フェーズ 2 |
| `TestReplyRejectsCodexMention` | `TestReplyRejectsMention` | `internal/fixprguard/write_test.go` | フェーズ 2 |
| `TestEditPRRejectsCodexMention` | `TestEditPRRejectsMention` | `internal/fixprguard/write_test.go` | フェーズ 2 |

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: ローカルの CLI レビューの変更・PR 前のレビュー追加・`codexreview` の撤去

設計書 §3.2・§3.6・§3.8・§7.4・§8（PR-1）に対応する。

#### ステップ 1-1: ローカルの CLI レビューから Codex CLI を外す（F-006、AC-26〜AC-28）

**変更ファイル:**
- `.claude/commands/_lib/local-cli-review.md`
- `.claude/commands/fixpr.md`
- `.claude/commands/runplan.md`

**作業内容:**
- [ ] `_lib/local-cli-review.md` の step 3（`codexreview` の実行・表・`stop` の扱い）を撤去し、step 2 の失敗をすべて `not run` 記録として進む形に改める（設計書 3.6）。
- [ ] `_lib/local-cli-review.md` の step 7 の新しく書く記録行を `(coderabbit)` / `(coderabbit: <kind>)` の形に改め、「Reading the record lines」には古い `Codex local review:`・`(codex)`・`(coderabbit: <kind>; codex: <kind>)` の読み方を残し、外せる条件（古い形を持つ開いたブランチがなくなったとき）を記す（AC-28）。
- [ ] `_lib/local-cli-review.md` の step 8 SUMMARY から「Codex CLI が走った回数」と `instructions_changed` の注記を外す。
- [ ] `fixpr.md` Phase 4.5 と `runplan.md` step 7.7・B1 から Codex CLI フォールバックの記述を外す。

**完了基準:**
- `_lib/local-cli-review.md` に `codexreview` の呼び出しが残らず、古い記録行の読み方が保たれている。

#### ステップ 1-2: PR を開く直前の未レビュー commit のローカル CLI レビューを追加する（F-007、AC-29〜AC-31）

**変更ファイル:**
- `.claude/commands/_lib/task-pr-handoff.md`
- `.claude/commands/_lib/review-records.sh`
- `.claude/commands/runplan.md`

**作業内容:**
- [ ] `_lib/task-pr-handoff.md` Part B の step 2（CHECKS）と step 3（push と PR 作成）の間に step 2.5 を追加する（設計書 3.2）。`origin/main` の `review-records.sh coverage none <HEAD> [<PLAN>]` を呼び、`ALL` ならレビューせず、`PARTIAL <n>` またはスクリプト失敗時はローカルの CLI レビューを 1 回だけ走らせ、直しがあれば CHECKS を再実行する手順を書く。
- [ ] `_lib/task-pr-handoff.md` Part B step 2 の CHECKS 一覧から `./cmd/codexreview/...` を外し、step 3 の PR 説明の項目から「Codex CLI が走った回数」を外す。
- [ ] `_lib/review-records.sh` のヘッダコメントに F-007 の `coverage none` 呼び出しを追記する（判定ロジックと `(coderabbit|codex)` の正規表現は変えない）。
- [ ] `runplan.md` の「Before the PR」の理由を ADR-0042 に合わせ、Part B の CHECKS の規則に「step 2.5 の直しとその後の CHECKS の直しは Opus レビューの判断のやり直しを引き起こさない」旨を追記する（設計書 3.2）。

**完了基準:**
- 設計書 3.2・6.2 のフローと一致し、`review-records.sh` の既存テストが通る。

#### ステップ 1-3: `codexreview` の製品コードとテストを撤去する（F-008、AC-32・AC-33・AC-34・AC-36）

**変更ファイル:**
- `cmd/codexreview/`（ディレクトリ削除）
- `internal/codexreview/`（ディレクトリ削除）
- `Makefile`
- `.claude/settings.json`
- `.github/workflows/ci.yml`
- `internal/pipeline/pipeline_test.go`
- `docs/dev/conventions/package_reference.md`
- `internal/config/envaccess_test.go`
- `internal/cmdrunner/runner.go`
- `cmd/coveragecompare/main.go`
- `CLAUDE.md`（`codexreview` に係る箇所）
- `.claude/commands/_lib/simple-process.md`（CHECKS の `./cmd/codexreview/...`）

**作業内容:**
- [ ] `cmd/codexreview/` と `internal/codexreview/` を 1 つの commit で削除し、その commit で `make coverage-compare BASE=<親>` を実行して PASS（消えた関数が一覧に出るのみ）を確認し、結果を commit メッセージに残す（設計書 7.4）。
- [ ] `Makefile` から `install-codexreview` を、`.claude/settings.json` から `Bash(codexreview *)` を、`.github/workflows/ci.yml` の `doc-guard` と `_lib/simple-process.md` の CHECKS から `./cmd/codexreview` を外す。
- [ ] `internal/pipeline/pipeline_test.go` の `TestFakesCarryBuildTag` のファイル数を 24 から 22 へ更新し、`docs/dev/conventions/package_reference.md` から `cmd/codexreview`・`internal/codexreview`・`internal/codexreview/testutil` の行と `internal/strictjson` の利用元の記述を外す。
- [ ] `internal/config/envaccess_test.go` の `cmd/codexreview/main.go` 行とコメント、`internal/cmdrunner/runner.go`・`cmd/coveragecompare/main.go` の `codexreview` コメントを外す。
- [ ] `CLAUDE.md` から `install-codexreview`、`Declare, don't infer` の `codexreview` 例外、Testing Strategy・`doc-guard`・Protocol numbers・Tool Execution Safety の `codexreview` 記述を外す。

**完了基準:**
- `make test && make lint && make deadcode` および CI の `doc-guard` コマンドが通る。

#### ステップ 1-4: 残るパッケージの `codexreview` 参照と文書ガードを更新する（AC-28・AC-35・AC-37）

**変更ファイル:**
- `internal/doctestutil/protocol.go`
- `internal/doctestutil/protocol_test.go`
- `internal/doctestutil/commands.go`
- `internal/mergepr/protocolcheck_test.go`
- `cmd/fixprguard/docs_test.go`
- `cmd/yt2column/fixpr_docs_test.go`
- `cmd/yt2column/review_records_test.go`

**作業内容:**
- [ ] `internal/doctestutil/protocol.go` から `ToolCodexreview` と `protocolTools` の要素を外し、`internal/doctestutil/commands.go` のコメントの `codex` 例を差し替える。
- [ ] `internal/doctestutil/protocol_test.go`、`internal/mergepr/protocolcheck_test.go`（`TestMismatchRemedy`）、`cmd/fixprguard/docs_test.go`（`TestFixprguardProtocolMismatches`）の `codexreview` ケースを削除する（設計書 7.4 に従い、テスト削除のみの commit に分けて `make coverage-compare BASE=<親>` で PASS を確認する）。
- [ ] `cmd/yt2column/fixpr_docs_test.go` の `TestLocalReviewGuard` から step 3 の確認を外す。
- [ ] `cmd/yt2column/review_records_test.go` の `seenRangeForm`（`(coderabbit)` の形）、`TestRecordLinePrefixesDistinct` の下限、`TestReviewRecordsCallSite`（`_lib/task-pr-handoff.md` の `coverage none` 呼び出し）を更新する。`TestReviewRecordsCoverage` の `(codex)` を読む既存テストは AC-28 の検証として残す。

**完了基準:**
- `go test -tags test ./internal/doctestutil ./internal/mergepr ./cmd/fixprguard ./cmd/yt2column` が通り、テスト削除 commit の `make coverage-compare` が PASS する。

#### ステップ 1-5: フェーズ 1 の ADR 食い違い記述の整理と静的検証スクリプト作成（AC-32〜AC-37・AC-49）

**変更ファイル:**
- `docs/dev/adr/0013-review-setup.md`
- `docs/dev/adr/0014-deterministic-checks-in-go.md`
- `docs/dev/adr/0042-implementation-model-tiers.md`
- `docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh`（新規）

**作業内容:**
- [ ] `docs/dev/adr/0014-deterministic-checks-in-go.md` と `docs/dev/adr/0042-implementation-model-tiers.md` の `食い違い:` を外し、`docs/dev/adr/0013-review-setup.md` の `食い違い:` から Codex CLI（`codexreview`）の部分を外す。
- [ ] `docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` を作成し、`cmd/codexreview`・`internal/codexreview` の不在、`Makefile` の `install-codexreview` 不在、`.claude/settings.json` の `codexreview` 許可不在、Go ソースの `codexreview` 参照不在を検証する（実装時に 1 件以上壊して失敗することを確認する）。

**完了基準:**
- スクリプトが終了コード 0 で通り、`make test && make lint && make deadcode` が通る。

---

### フェーズ 2: `fixprguard` の拡張・PR レビューの手動化・メンションの拒否と自動レビュアー一覧の更新

設計書 §3.1〜§3.5・§3.8・§4・§5・§7・§8（PR-2）に対応する。

#### ステップ 2-1: `fixprguard` のメンション拒否拡張・`current-body-mention`・プロトコル番号引き上げ（F-009、AC-38・AC-39）

**変更ファイル:**
- `internal/fixprguard/errors.go`
- `internal/fixprguard/write.go`
- `internal/fixprguard/types.go`
- `internal/fixprguard/write_test.go`
- `internal/fixprguard/worktree_flow_test.go`
- `internal/fixprguard/test_helpers.go`
- `internal/fixprguard/types_test.go`
- `internal/fixprguard/trust_test.go`
- `cmd/fixprguard/main.go`
- `cmd/fixprguard/main_test.go`
- `cmd/fixprguard/test_helpers.go`

**作業内容:**
- [ ] `errors.go`・`write.go` の `checkPostText` を `@codex` と `@coderabbitai` の 2 つ（ASCII 大小文字無視、単語境界なし、全角やゼロ幅文字は正規化しない）を拒否するよう拡張し、1.3 のリネーム一覧どおり `errCodexMention` → `errMention` 等を改名する。
- [ ] `ErrCurrentBodyMention`（`ErrInvalidInput` に包まない）と `StopCurrentBodyMention = "current-body-mention"` を追加し、`StopKinds()`・`StopKind()` に登録する。`AppendPR` で既存の PR 説明にメンションが含まれる場合は `ErrCurrentBodyMention`（終了コード 1）、追記文面自体に含まれる場合は `ErrInvalidInput`（終了コード 2）を返すようにする（設計書 3.3.5・4.2）。
- [ ] `internal/fixprguard/types.go` の `Protocol` を `main` の値 + 1 に上げる（設計書 3.3.5）。
- [ ] `internal/fixprguard/test_helpers.go`・`cmd/fixprguard/test_helpers.go` の `testSecondReviewer` および `types_test.go`・`trust_test.go` の `"chatgpt-codex-connector"` を Codex 名でない架空の名前に改める。
- [ ] `write_test.go`・`worktree_flow_test.go`・`cmd/fixprguard/main_test.go` を更新し、`@coderabbitai` 拒否・`@coderabbit` や全角 `＠` の非拒否・`append-pr` の `current-body-mention`（終了コード 1）を検証する（実装時に判定を壊してテストが失敗することを確認する）。

**完了基準:**
- `go test -tags test ./internal/fixprguard ./cmd/fixprguard` が通る。

#### ステップ 2-2: `fixprguard review-state` サブコマンドとテストを実装する（F-004、AC-14〜AC-18・AC-54）

**変更ファイル:**
- `internal/fixprguard/review_state.go`（新規）
- `internal/fixprguard/review_state_test.go`（新規）
- `internal/fixprguard/testdata/`（新規：観測の記録から抽出・サニタイズした fixture。本文と `details_url` を含めない）
- `cmd/fixprguard/main.go`
- `cmd/fixprguard/main_test.go`
- `CLAUDE.md`（「Declare, don't infer」に `review-state` の CheckRun 照合例外を追記）

**作業内容:**
- [ ] `internal/fixprguard/review_state.go` に設計書 3.3.1〜3.3.3 の型（`ReviewState`・`ReviewRequest`・`ReviewResponse`・`ReviewMarker`・`RunKind`・`RequestState`・`UnreadableReason`・`IndeterminateReason`、`MarshalText`、ゼロ値規則）と取得・厳密検査・分類・状態遷移のロジックを実装する。既存の `runGH`・`resolveRepository`・`strictjson`・`parseDateHeader` を再利用する。
- [ ] `cmd/fixprguard/main.go` に `review-state` サブコマンド（`--protocol`・`--reviewer`・`--request`・必須 PR 引数）を登録し、終了コード 0（引数不正のみ 2）で JSON を出力する。
- [ ] `internal/fixprguard/review_state_test.go` と `cmd/fixprguard/main_test.go` で設計書 7.1・7.2・7.5 の全ケース（表の全分類行、`RunSkipNotice` 除外、同秒除外、応答後 CheckRun 除外、競合 `competing-runs`、編集済み依頼 `edited-request`、15 分上限、依頼なし時の後段スキップ、厳密 JSON 拒否、上限超過 `limit`、`gh` 失敗 `call-failed`、head 変化時の再取得扱い、PR 外 commit の混入除外、偽の著者・app slug 除外、本文・`title`・`details_url` の非出力）をテストする（実装時に分類表の行や検査を壊して失敗することを確認する）。
- [ ] `CLAUDE.md` の「Declare, don't infer」に設計書 3.3.2 の `review-state` 例外（範囲・理由・境界）を追加する。

**完了基準:**
- `go test -tags test ./internal/fixprguard ./cmd/fixprguard` が通り、本文・`title`・`details_url` が標準出力にもエラーにも現れないことがテストで検証される。

#### ステップ 2-3: `_context.md` と全コマンド定義のメンション確かめ・PR 作成後手順を更新する（F-002・F-009・F-010、AC-06・AC-07・AC-40・AC-43・AC-46）

**変更ファイル:**
- `.claude/commands/_context.md`
- `.claude/commands/_lib/task-pr-handoff.md`
- `.claude/commands/fixpr.md`
- `.claude/commands/_lib/fable-review.md`
- `.claude/commands/_lib/opus-review.md`
- `.claude/commands/_lib/simple-process.md`

**作業内容:**
- [ ] `_context.md` を設計書 3.1 の表どおり更新する（`Automated reviewer list` = `coderabbitai`、`Awaited PR reviewer` = `coderabbitai`、`PR review request` = `@coderabbitai review`、新規 `Rejected mentions` = `@codex`・`@coderabbitai`、置き換え名 `<mention-check>`・`<rejected-mentions>` の定義）。
- [ ] `_lib/task-pr-handoff.md` Part B step 5（Codex 開始待機・依頼投稿）を撤去し、冒頭・Authorization・通知・報告の Codex 記述を外す（AC-06・AC-07）。
- [ ] `_lib/task-pr-handoff.md`・`fixpr.md`・`_lib/fable-review.md`・`_lib/opus-review.md`・`_lib/simple-process.md` の `<review-mention>` を `<mention-check>` に置き換え、`fixpr.md` の `--protocol` を `main` の値 + 1 に更新し、`append-pr` の `current-body-mention` 停止の報告手順（AC-39）と `replyBody` 規則を更新する。
- [ ] `_lib/fable-review.md` の冒頭理由を ADR-0050 の新しい理由に改め、Codex CLI 指摘の記述を外す。

**完了基準:**
- 依頼のコメント以外の全投稿経路が `<mention-check>` を参照し、Part B に PR 作成後のコメント投稿手順が存在しない。

#### ステップ 2-4: `runpr.md` と `_lib/pr-review-escalation.md` を手動 PR レビューと推奨のみのエスカレーションへ更新する（F-003〜F-005・F-009・F-010、AC-08〜AC-25・AC-41・AC-42・AC-44・AC-52〜AC-55）

**変更ファイル:**
- `.claude/commands/runpr.md`
- `.claude/commands/_lib/pr-review-escalation.md`
- `.claude/commands/_lib/review-records.sh`（ヘッダの LAST_REVIEWED 説明）

**作業内容:**
- [ ] `runpr.md` step 1（単独 PR 作成）から Codex 開始待機・依頼投稿を外す（AC-06・AC-07）。
- [ ] `runpr.md` step 3 を設計書 3.4.1 に従って書き換える。見出しを更新し、Codex 要約コメント・受領コメント・状態規則 1〜7・自動依頼を撤去する。`fixprguard review-state` の状態ごとの処理、`<rejected-mentions>` を使うメンション 3 一覧（CodeRabbit の観測済み 2 行を除外し、Codex 投稿は `by_bot` とする）、`LAST_REVIEWED` の算出（設計書 3.3.4）、interaction limits 維持（AC-42）、`fixprguard list` によるスレッド処理（AC-10・AC-44）を書く。
- [ ] `runpr.md` step 4 を設計書 3.4.2・3.4.3・3.5 に従って書き換える。「先に PR レビューを依頼」選択肢の表示条件・説明・位置、ラウンドごとの `head` 照合、要約の 15 行（Codex 名なし、手動 PR レビュー行の追加、本文指摘・未レビューファイルありうる旨の表示）、詳細ファイル更新、選択時の `gh pr comment` 依頼と `review-state` 待機、停止報告表を書く。自動エスカレーション依頼を撤去し、答えを推奨に写す規則 1〜5 を 1 か所に置く。
- [ ] `runpr.md` step 5 の最終報告から受領コメントの行を外し、全 `fixprguard` 呼び出しの `--protocol` を `main` の値 + 1 に更新する。
- [ ] `_lib/pr-review-escalation.md` の冒頭・Inputs（LAST_REVIEWED なしでも merge-base..HEAD で判定する）・基準 4 のレビュー関門の範囲（`fixprguard review-state` 等を含む）を更新し、`_lib/review-records.sh` ヘッダの LAST_REVIEWED 説明を合わせる。

**完了基準:**
- 設計書 3.4・3.5・6.1 のすべての分岐と報告項目が `runpr.md` と `_lib/pr-review-escalation.md` に反映されている。

#### ステップ 2-5: 文書ガードテスト・ガイド・ADR 食い違い記述を更新する（AC-41・AC-43・AC-46・AC-49・AC-56）

**変更ファイル:**
- `cmd/yt2column/codex_review_docs_test.go` → `cmd/yt2column/review_mention_docs_test.go`（改名 commit と中身変更 commit を分ける）
- `cmd/yt2column/fixpr_docs_test.go`
- `cmd/fixprguard/docs_test.go`
- `docs/dev/guides/fixprguard_guide.md`
- `docs/dev/adr/0013-review-setup.md`
- `docs/dev/adr/0015-untrusted-review-threads.md`
- `docs/dev/adr/0047-runpr-merge-question.md`
- `docs/dev/adr/0050-fable-partial-review.md`

**作業内容:**
- [ ] `cmd/yt2column/codex_review_docs_test.go` を `cmd/yt2column/review_mention_docs_test.go` に改名する commit を独立して作る（設計書 7.4）。
- [ ] `review_mention_docs_test.go` を設計書 7.3 に従って更新する。`TestMentionsOnlyInAllowedRows`（2 つのメンションが `_context.md` の 2 行以外に出ないこと）、`TestPostingPathsCheckMention`（`<mention-check>` とその展開が `Rejected mentions` 全値を含むこと）、`TestAgentsCodeReviewRules` のコメント理由を CodeRabbit に更新（H-09）、`TestCodeRabbitConfigStopsReview` 維持。
- [ ] `cmd/yt2column/fixpr_docs_test.go` で `TestHandoffListingPrintsNoBody` と要約・受領コメントのフィルタテストを削除する（削除のみの commit に分けて `make coverage-compare BASE=<親>` で PASS を確認する）。`wantReviewers` を `[]string{"coderabbitai"}` に更新し、`TestRunprDocContract` を新しい step 3 見出し・メンション 2 種・CodeRabbit の観測 2 行・`review-state` との依頼コメント判定一致テスト（設計書 7.3）に合わせる。
- [ ] `cmd/fixprguard/docs_test.go` に `review-state` の呼び出し形・ガイドの `review-state` 状態・理由一覧・`TestGuideDescribesMentionCheck`（2 つのメンション）を追加し、`docs/dev/guides/fixprguard_guide.md` に `review-state` の章、`current-body-mention`、2 つのメンション、新しいプロトコル番号を書く。
- [ ] `docs/dev/adr/0013`・`0015`・`0047`・`0050` の `食い違い:` 記述を外す（AC-49）。

**完了基準:**
- `make test && make lint && make deadcode` および `doc-guard` が通る。テスト削除 commit の `make coverage-compare` が PASS する。

> [!IMPORTANT]
> **フェーズ 2 マージ直後の手動作業（要件定義書 4.4、設計書 6.3）:**
> 1. 利用者が `main` から `make install-fixprguard` で `fixprguard` を入れ直す（フェーズ 2 の PR レビュー中もブランチから入れ直す）。
> 2. 利用者が Codex の GitHub 連携を切り、契約を終える。
> 3. 完了後にフェーズ 3 へ進む。

---

### フェーズ 3: 残りの文書・ADR・用語集の更新と chief の確認

設計書 §3.7・§3.8・§3.9・§6.3・§8（PR-3）に対応する。

#### ステップ 3-1: 残りの仕様文書・ガイド・設定コメント・用語集を更新する（F-011、AC-47・AC-48）

**変更ファイル:**
- `.coderabbit.yaml`
- `CLAUDE.md`（Codex PR レビューの段落など残り）
- `docs/dev/conventions/dev_tools.md`
- `docs/dev/conventions/requirements_process.md`
- `docs/dev/guides/fixprguard_guide.md`（残りの Codex 記述）
- `docs/dev/guides/mergepr_guide.md`
- `docs/translation_glossary.md`

**作業内容:**
- [ ] `.coderabbit.yaml` のコメントを、手動の PR レビューと CodeRabbit CLI のみの説明に改める（`reviews.auto_review.enabled: false` の設定値は変えない）。
- [ ] `CLAUDE.md`（Claude Commands の Codex PR レビュー段落等）、`docs/dev/conventions/dev_tools.md`、`docs/dev/conventions/requirements_process.md`、`docs/dev/guides/fixprguard_guide.md`、`docs/dev/guides/mergepr_guide.md` から、今の振る舞いとして書かれた Codex PR レビュー・Codex CLI・`codexreview`・受領コメントの記述を改める（AC-47 の例外に当たる過去出典・`@codex` 拒否・AC-28 古い記録行は残す）。
- [ ] `docs/translation_glossary.md` の「試行」の定義を CodeRabbit CLI だけの定義に改め、「手動の PR レビュー」「完了の印」を追加するか用語集の規則に従って判断する（AC-48）。

**完了基準:**
- `make test && make lint` および `doc-guard` が通り、現在の仕様を書く文書に AC-47 違反がない。

#### ステップ 3-2: ADR-0040・ADR-0046 の Codex 記述を点検・更新する（F-011、H-07、AC-47）

**変更ファイル:**
- `docs/dev/adr/0040-simple-task-documents.md`
- `docs/dev/adr/0046-chief-of-staff.md`

**作業内容:**
- [ ] 設計書 3.9 に従い、ADR-0040（`:11`）と ADR-0046（`:284,473`）の Codex に触れる箇所を「過去の出典」「今の振る舞い」「理由が崩れる箇所」に分類する。
- [ ] 今の振る舞いとしての記述は `docs/dev/conventions/adr.md` の「Editorial corrections」に従って直し、commit メッセージに `Editorial correction:` 行を残す。決定の理由が崩れる箇所があると判断した場合は、変更前に利用者へ確認する。

**完了基準:**
- ADR-0040・0046 が AC-47 と ADR 規約を満たす。

#### ステップ 3-3: chief のテスト確認と古い記録行の整理用 issue 作成（F-012、AC-50・AC-51、設計書 3.6）

**変更ファイル:**
- なし（`/runpr` の報告・NEXT 行の形が chief の読み取りに影響する変更になった場合のみ `internal/chief/testdata/` とテストを追加する）

**作業内容:**
- [ ] `go test -tags test ./cmd/chief ./internal/chief` を実行し、`internal/chief/testdata/` の既存サンプルがすべて残り成功することを確認する（AC-50）。
- [ ] フェーズ 2 の `runpr.md` 変更で `/runpr` の報告・NEXT 行の形が chief の読み取りに当たらないことを確認し、当たらないならサンプルを足さない（当たる場合は AC-51 に従い実報告サンプルとテストを追加する）。
- [ ] 古い記録行（`Codex local review:`、`(codex)`、`not run` の旧形）の読み方規則を、該当ブランチがなくなった後に外すためのフォローアップ issue を GitHub に作成する（設計書 3.6・6.3）。

**完了基準:**
- chief テストが成功し、フォローアップ issue の URL が記録されている。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 対象フェーズ | 主な成果物 | 完了条件 |
|---|---|---|---|
| M1: ローカル CLI レビュー移行と `codexreview` 撤去 | フェーズ 1（ステップ 1-1〜1-5） | Codex CLI なしの `_lib/local-cli-review.md`、Part B step 2.5、`codexreview` 撤去、`check_codexreview_removed.sh`、ADR-0013/0014/0042 更新 | `make test && make lint && make deadcode`・`doc-guard`・`coverage-compare`・`check_codexreview_removed.sh` が通る |
| M2: 手動 PR レビュー移行と境界更新 | フェーズ 2（ステップ 2-1〜2-5） | `fixprguard review-state`・2 メンション拒否・`current-body-mention`・プロトコル番号引き上げ、`_context.md`・`runpr.md`・`task-pr-handoff.md`・ガードテスト・ガイド・ADR-0013/0015/0047/0050 更新 | `make test && make lint && make deadcode`・`doc-guard`・`coverage-compare` が通る。マージ直後に利用者が `fixprguard` を入れ直し、Codex 連携を切る |
| M3: 仕様文書整合と完了確認 | フェーズ 3（ステップ 3-1〜3-3） | `.coderabbit.yaml`・`CLAUDE.md`・規約・ガイド・用語集・ADR-0040/0046 更新、chief テスト確認、フォローアップ issue | `make test && make lint`・`doc-guard` が通り、全 AC の検証が完了する |

## 4. テスト戦略 (Test Strategy)

設計書 §7 に従う。新しい `testutil/` パッケージは作らず、既存の `internal/fixprguard/test_helpers.go`・`cmd/fixprguard/test_helpers.go`（`//go:build test`）のみ更新する（`docs/dev/conventions/test_organization.md` に準拠）。

-   **単体テスト（`internal/fixprguard`）:** `review_state_test.go` で CheckRun 分類表の全行・状態規則・境界値（同秒・15 分・編集済み依頼・競合 CheckRun・上限・厳密 JSON 拒否・PR 外 commit 混入）を検証する。`write_test.go`・`worktree_flow_test.go` で 2 メンション拒否と `ErrCurrentBodyMention` を検証する。各テストは実装時に該当ロジックを壊して失敗することを確かめる。
-   **結合テスト（`cmd/fixprguard`）:** 偽の `gh` で `review-state` と `append-pr` の標準出力 JSON・終了コード・本文・`title`・`details_url` 非出力を検証する。
-   **文書ガードテスト（`cmd/yt2column`・`cmd/fixprguard`・`internal/pipeline`・`internal/doctestutil`・`internal/mergepr`）:**
    -   `review_mention_docs_test.go`: 全投稿経路の `<mention-check>`、`_context.md` の 2 行限定のメンション出現、`.coderabbit.yaml` の自動レビュー無効化、`AGENTS.md` 見出し。
    -   `fixpr_docs_test.go`: `wantReviewers` = `coderabbitai`、`TestRunprDocContract` のメンションフィルタ実行と `review-state` の依頼コメント判定の一致確認、`TestLocalReviewGuard`。
    -   `review_records_test.go`: `(coderabbit)` 形式・`coverage none` 呼び出し・`(codex)` 後方互換（AC-28）。
    -   `cmd/fixprguard/docs_test.go`: `review-state` 呼び出し形・プロトコル番号・ガイドの 2 メンション・状態表・理由表・停止種類。
-   **テスト削除のカバレッジ比較（H-04）:** `cmd/codexreview`・`internal/codexreview` の一括削除 commit と、残るパッケージのテスト削除 commit ごとに `make coverage-compare BASE=<親>` を実行し PASS を確認する。
-   **セキュリティテスト（設計書 7.5）:** 本文・`title`・`details_url` に目印文字列を入れた fixture で出力への非漏洩を検証し、偽の app slug・CheckRun 名・`User`/`Bot` なりすましを拒否することを検証する。
-   **後方互換性テスト:** `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage` で `(codex)` 行の読み取り（AC-28）を、`internal/chief` テストで既存サンプルの維持（AC-50）を検証する。

## 5. リスク管理 (Risk Management)

-   **移行順序の破綻（AC-45）:** フェーズ 2 より前に Codex 連携を切ると現行 `/runpr` が止まり続け、フェーズ 1 より前に `codexreview` を消すとローカル CLI レビューが壊れる。対策: フェーズ 1 → フェーズ 2 → 利用者による連携解除 → フェーズ 3 の順序を厳守する。
-   **フェーズ 2 の PR レビュー中の Codex 自動投稿:** フェーズ 2 を開く時点では Codex 連携が有効なため、PR-2 上で Codex のスレッド（Untrusted threads）と要約コメント（`by_bot` メンション）に対する確認が発生する（設計書 6.3）。対策: 想定内の挙動として利用者が応答して進める。
-   **`fixprguard` のプロトコル番号引き上げと並行ブランチ:** 番号を上げると、入れ直し後、`main` を取り込んでいない並行ブランチの `/fixpr`・`/runpr` は番号不一致で止まる。対策: フェーズ 2 のブランチとマージ後に `make install-fixprguard` を案内し、他ブランチは `main` を取り込む。
-   **`TestFakesCarryBuildTag` と `package_reference.md` の同期漏れ:** `internal/codexreview` 撤去時に固定ファイル数（24→22）と `package_reference.md` の更新が漏れると `make test` が落ちる。対策: ステップ 1-3 の同一フェーズで同時に更新し、グリーンゲートで確認する。
-   **`append-pr` の既存 PR 説明との衝突（AC-39）:** CodeRabbit が PR 説明にメンションを含む文を書き足した場合に `append-pr` が止まる。対策: `ErrCurrentBodyMention`（`current-body-mention`、終了コード 1）として区別し、`fixpr.md` が原因を報告して再試行しない手順とする。

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1 完了（ステップ 1-1〜1-5）
-   [ ] フェーズ 2 完了（ステップ 2-1〜2-5）
-   [ ] フェーズ 2 マージ後に利用者が `fixprguard` 入れ直し・Codex 連携解除を完了
-   [ ] フェーズ 3 完了（ステップ 3-1〜3-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 検証方法 | 種別 | 対応ステップ | フェーズ |
|---|---|---|---|---|
| AC-01 | PR #1070 の `docs/tasks/0118_codex_removal/observations.md` で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-02 | PR #1070 の `docs/tasks/0118_codex_removal/observations.md` §1 で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-03 | PR #1070 の `docs/tasks/0118_codex_removal/observations.md` §1・§2 で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-04 | PR #1070 の `docs/tasks/0118_codex_removal/observations.md` §4 で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-05 | PR #1070 の `02_architecture.md` と `observations.md` の照合で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-06 | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory`（Part B・`runpr.md` step 1 に `gh pr comment` がないこと） | static | ステップ 2-3・2-4・2-5 | フェーズ 2 |
| AC-07 | `_lib/task-pr-handoff.md` Part B と `runpr.md` step 1 の通知・報告文を PR レビューで確認 | manual | ステップ 2-3・2-4 | フェーズ 2 |
| AC-08 | `internal/fixprguard/review_state_test.go::TestReviewState`（依頼なし時の `none` と後段スキップ）、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse` | test / static | ステップ 2-2・2-4・2-5 | フェーズ 2 |
| AC-09 | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（要約・受領コメントのフィルタが存在しないこと） | static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-10 | `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（`runpr.md` の `fixprguard list` 呼び出し維持）、`internal/fixprguard/trust_test.go` | test / static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-11 | `runpr.md` step 4 の要約 15 行の定義を PR レビューで確認（Codex 名の不在とローカル CLI・Fable 行の維持） | manual | ステップ 2-4 | フェーズ 2 |
| AC-12 | `runpr.md` step 4 の選択肢条件（LAST_REVIEWED != head）を PR レビューで確認、`internal/fixprguard/review_state_test.go::TestReviewState`（`markers` の算出） | test / manual | ステップ 2-2・2-4 | フェーズ 2 |
| AC-13 | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory`（`gh pr comment <number> --body "<review-request>"` の配置箇所） | static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-14 | `internal/fixprguard/review_state_test.go::TestReviewState`（`waiting`・`completed`）、`cmd/fixprguard/main_test.go::TestReviewState` | test | ステップ 2-2・2-4 | フェーズ 2 |
| AC-15 | `internal/fixprguard/review_state_test.go::TestReviewState`（app slug・CheckRun 名・`User`/`Bot` なりすましの拒否） | test | ステップ 2-2 | フェーズ 2 |
| AC-16 | `internal/fixprguard/review_state_test.go::TestReviewState`（`competing-runs`・`edited-request`・`rejected`・非 16 進 SHA・PR 外 commit 除外・本文非出力）、`cmd/fixprguard/main_test.go::TestReviewState` | test | ステップ 2-2・2-4 | フェーズ 2 |
| AC-17 | `internal/fixprguard/review_state_test.go::TestReviewState`（依頼前の CheckRun 除外・同秒除外・前の依頼の応答範囲） | test | ステップ 2-2 | フェーズ 2 |
| AC-18 | `internal/fixprguard/review_state_test.go::TestReviewState`（`rate-limited`・`unrecognized`・`timed-out`）、`cmd/fixprguard/main_test.go::TestReviewState` | test | ステップ 2-2・2-4 | フェーズ 2 |
| AC-19 | `internal/fixprguard/review_state_test.go::TestReviewState`・`cmd/fixprguard/main_test.go::TestReviewState`（本文非出力）、`runpr.md` step 4 要約行の PR レビュー | test / manual | ステップ 2-2・2-4 | フェーズ 2 |
| AC-20 | `runpr.md` step 4 の手動 PR レビュー要約行（未レビューファイルがありうる旨と URL 表示）を PR レビューで確認（設計書 1.3 の決定） | manual | ステップ 2-4 | フェーズ 2 |
| AC-21 | `_lib/pr-review-escalation.md`・`runpr.md` の範囲定義と `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`（`none` 指定時の `origin/main..HEAD`） | test / manual | ステップ 2-4 | フェーズ 2 |
| AC-22 | `runpr.md` step 4 の推奨マッピング規則 2 を PR レビューで確認 | manual | ステップ 2-4 | フェーズ 2 |
| AC-23 | `runpr.md` step 4 の推奨マッピング規則 3 と `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage` | test / manual | ステップ 2-4 | フェーズ 2 |
| AC-24 | `runpr.md` step 4 の推奨マッピング規則 5 を PR レビューで確認 | manual | ステップ 2-4 | フェーズ 2 |
| AC-25 | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory`（エスカレーション経路に依頼投稿がないこと） | static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-26 | `cmd/yt2column/fixpr_docs_test.go::TestLocalReviewGuard`、`docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-1・1-4・1-5 | フェーズ 1 |
| AC-27 | `_lib/local-cli-review.md` step 7・`_lib/task-pr-handoff.md` Part B step 3・`runpr.md` step 4 の要約行を PR レビューで確認、`cmd/yt2column/review_records_test.go::TestRecordLinePrefixesDistinct` | static / manual | ステップ 1-1・1-4・2-4 | フェーズ 1・2 |
| AC-28 | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`（`(codex)` 行を見た範囲として読む既存テストの維持） | test | ステップ 1-1・1-4 | フェーズ 1 |
| AC-29 | `cmd/yt2column/review_records_test.go::TestReviewRecordsCallSite`（`_lib/task-pr-handoff.md` Part B step 2.5 の `coverage none` 呼び出し）、`TestReviewRecordsCoverage` | test / static | ステップ 1-2・1-4 | フェーズ 1 |
| AC-30 | `_lib/task-pr-handoff.md` Part B step 2.5・step 3 の失敗時 `not run` 記録と PR 説明出力を PR レビューで確認 | manual | ステップ 1-2 | フェーズ 1 |
| AC-31 | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`（空 commit・チェックボックス変更のみの commit を `ALL` とする既存テスト） | test | ステップ 1-2・1-4 | フェーズ 1 |
| AC-32 | `internal/pipeline/pipeline_test.go::TestPackageReferenceCoversAllPackages`、`TestFakesCarryBuildTag`、`docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-3・1-5 | フェーズ 1 |
| AC-33 | `internal/mergepr/protocolcheck_test.go::TestMismatchRemedy`、`docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-3・1-4・1-5 | フェーズ 1 |
| AC-34 | `docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-3・1-5 | フェーズ 1 |
| AC-35 | `internal/mergepr/protocolcheck_test.go::TestWorktreeProtocolTools`、`TestMismatchRemedy`、`internal/doctestutil/protocol_test.go` | test / static | ステップ 1-3・1-4 | フェーズ 1 |
| AC-36 | `internal/config/envaccess_test.go::TestEnvAccessConfined`、`docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-3・1-5 | フェーズ 1 |
| AC-37 | `make test`、`make lint`、`make deadcode`、CI の `doc-guard` コマンド | test / static | ステップ 1-3・1-4・1-5 | フェーズ 1 |
| AC-38 | `internal/fixprguard/write_test.go`（`checkPostText`・`TestReplyRejectsMention`・`TestEditPRRejectsMention`・`AppendPR`）、`internal/fixprguard/worktree_flow_test.go`、`cmd/fixprguard/main_test.go` | test | ステップ 2-1 | フェーズ 2 |
| AC-39 | `internal/fixprguard/write_test.go`（`ErrCurrentBodyMention`）、`cmd/fixprguard/main_test.go`（`current-body-mention` 終了コード 1）、`cmd/fixprguard/docs_test.go::TestGuideCoversStopKindsAndScope` | test / static | ステップ 2-1・2-3・2-5 | フェーズ 2 |
| AC-40 | `cmd/yt2column/review_mention_docs_test.go::TestPostingPathsCheckMention`、`TestPostingCommandsInventory` | static | ステップ 2-3・2-5 | フェーズ 2 |
| AC-41 | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（2 メンション分類・CodeRabbit 観測 2 行除外・`by_user`/`by_bot`・依頼コメント判定の `review-state` 一致） | test | ステップ 2-4・2-5 | フェーズ 2 |
| AC-42 | `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（`runpr.md` の `interaction-limits` 維持）、`internal/fixprguard/limits_test.go` | test / static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-43 | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（`wantReviewers` = `coderabbitai` のみ） | static | ステップ 2-3・2-5 | フェーズ 2 |
| AC-44 | `internal/fixprguard/trust_test.go`（一覧外の Bot が `ReasonOtherBot` になる既存テスト）、`cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree` | test / static | ステップ 2-3・2-5 | フェーズ 2 |
| AC-45 | 本書のフェーズ・PR 順序（PR-1 → PR-2 → 連携解除 → PR-3）の遵守を各 PR マージ時に確認 | manual | 全フェーズ | フェーズ 1〜3 |
| AC-46 | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`、`cmd/yt2column/review_mention_docs_test.go::TestMentionsOnlyInAllowedRows` | static | ステップ 2-3・2-5 | フェーズ 2 |
| AC-47 | 現在の仕様を書く文書一式を PR レビューで確認（AC-47 で `manual` と指定） | manual | ステップ 1-3・2-3・2-4・3-1・3-2 | フェーズ 1〜3 |
| AC-48 | `docs/translation_glossary.md` の改訂内容を PR レビューで確認 | manual | ステップ 3-1 | フェーズ 3 |
| AC-49 | `docs/dev/adr/0013`・`0014`・`0015`・`0042`・`0047`・`0050` の `食い違い:` 削除を PR レビューで確認 | manual | ステップ 1-5・2-5 | フェーズ 1・2 |
| AC-50 | `go test -tags test ./cmd/chief ./internal/chief` | test | ステップ 3-3 | フェーズ 3 |
| AC-51 | `runpr.md` step 5 の変更差分が chief の読み取り形式に当たらないことを確認（当たる場合は新サンプル追加と `go test -tags test ./internal/chief`） | test / manual | ステップ 3-3 | フェーズ 3 |
| AC-52 | `runpr.md` step 4 の手動 PR レビュー要約行（URL と本文に指摘がありうる旨）を PR レビューで確認 | manual | ステップ 2-4 | フェーズ 2 |
| AC-53 | `internal/fixprguard/review_state_test.go::TestReviewState`（終わった依頼の状態返却）、`runpr.md` step 4 要約行の PR レビュー | test / manual | ステップ 2-2・2-4 | フェーズ 2 |
| AC-54 | `internal/fixprguard/review_state_test.go::TestReviewState`（応答後の CheckRun を完了の印に数えないこと） | test | ステップ 2-2 | フェーズ 2 |
| AC-55 | `runpr.md` step 4 の推奨マッピング規則 1・4 を PR レビューで確認 | manual | ステップ 2-4 | フェーズ 2 |
| AC-56 | `cmd/yt2column/review_mention_docs_test.go`（`TestMentionsOnlyInAllowedRows`・`TestPostingPathsCheckMention`・`TestPostingCommandsInventory`・`TestCodeRabbitConfigStopsReview`） | static | ステップ 2-5 | フェーズ 2 |

## 8. 成功基準 (Success Criteria)

-   **機能的完全性:** AC-06〜AC-56 がすべて検証を通り、フェーズ 2 マージ後に利用者が Codex 連携を切った状態でフェーズ 3 の PR が `/runpr` で回る。
-   **品質基準:** 各フェーズの完了時に `make test && make lint && make deadcode` および CI の `doc-guard` コマンドが通る。テスト削除 commit はすべて `make coverage-compare` が PASS する。
-   **セキュリティ検証:** `review-state` とメンションフィルタが本文・`title`・`details_url` を出力しないこと、2 つのメンションの二重拒否（`<mention-check>` と `checkPostText`）が全経路で機能することがテストとガードで検証される。
-   **文書整合性:** 現在の仕様を書く文書と ADR に食い違いが残らず、`docs/dev/guides/fixprguard_guide.md` と `docs/translation_glossary.md` が更新されている。

## 9. 次のステップ (Next Steps)

1.  本計画書の PR をレビューし、`/runpr` で承認・マージする。
2.  承認後、`/runplan 0118` でフェーズ 1 から順に実装する。
