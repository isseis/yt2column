# 実装計画書：開発コマンド定義と補助ツールのハーネス非依存化（Antigravity 対応）

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-22 |
| Review date | 2026-11-23 |
| Reviewer | isseis |
| Comments | - |

本計画の `F-NNN`・`AC-NN` は [01_requirements.md](01_requirements.md)（以下、要件定義書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を、`H-NN` は [design_handoff.md](design_handoff.md) の項番を指す。主要用語（ハーネス・ハーネスのアダプタ・上位モデルのレビュー・クロスチェックの部分レビュー・スクラッチパッド）の定義は要件定義書 3.1 に従う。

## 1. 実装の概要 (Implementation Overview)

**目的:** 要件定義書（F-001〜F-008、AC-01〜AC-44）と設計書に従い、`.claude/commands/` 配下の開発コマンド定義・共有手順・判定スクリプトおよび開発ツール（`fixprguard`・`chief`）をハーネス非依存へ移行し、Claude Code と Antigravity CLI（`agy`）の双方で同一の品質関門・承認規則・記録契約を維持する。

**実装方針:**

-   設計書 8 章の実装優先順位に従い、フェーズ 1（`fixprguard` 監視拡張・プロトコル番号引き上げ）→ フェーズ 2（`_context.md`・`chief`・`high-review-decision.sh` 一括改名とパリティ維持）→ フェーズ 3（`review-records.sh` 一般化とハーネスのアダプタ 3 ファイル新設）→ フェーズ 4（共有手順・全コマンド正本の改名とハーネス非依存化、`.agents/skills/` 新設、文書・ガード・ADR 完結）の順に進める。
-   各 PR が単独でグリーンゲート（`make test && make lint`）・`make deadcode`・`make readme-sync-check`・CI `doc-guard` を通る単位で実装する。
-   `fixprguard` と `chief` のプロトコル番号は `CLAUDE.md`「Protocol numbers」に従い実装時の `origin/main` の値 + 1 に設定し、タスク文書に具体数値は書かない。
-   前提タスク `#1059`（タスク 0118：Codex 撤去）の実装完了後に本タスクの実装 PR をマージする（要件定義書 4.4）。関連タスク `#988`（タスク 0114：記録後のマージコミット確認）とは先に `main` に入った側に後から実装する側が合わせる（AC-28、設計書 3.5）。
-   Go ソースコード（`*_test.go`・`test_helpers*.go` を含む）・シェルスクリプト・英語指示文書（`.claude/commands/`・`.agents/skills/`）のコメント・識別子・文字列リテラルは英語で書く（`CLAUDE.md`「Go Idioms」の日本語リテラル例外に該当する箇所を除く）。テストが読む日本語の計画書標本は Go リテラルにせず `testdata/` の標本ファイルに置く。
-   設計の細部は設計書の該当節を参照し、本書には重複記載しない。

### 1.1 既存コード調査結果

`origin/main` のコミット `b93a2202` を調査した結果である。`implementation_handoff.md` は存在しない。`design_handoff.md` の全 5 件（H-01〜H-05）は設計書 1.3 に記録済みである。本タスクに具体的な型で振り分ける設計はなく、型の生成経路の列挙は該当しない（N/A）。

-   **前提・関連タスクの進捗（`b93a2202`）:**
    -   タスク 0118（`#1059`）は PR-1（`#1077`）・PR-2（`#1080`）がマージ済み、PR-3・PR-4 が未マージ（`docs/tasks/0118_codex_removal/03_implementation_plan.md:216-452`）。そのため現時点の `_context.md`（`:298-300`）と `cmd/yt2column/codex_review_docs_test.go` には Codex 記述が残っている。実装時は `#1059` マージ後の `main`（`review_mention_docs_test.go` 等）に合わせる。
    -   タスク 0114（`#988`）は計画書承認済み・全 PR 未作成（`docs/tasks/0114_fable_record_after_merge/03_implementation_plan.md:209-320`）。
-   **`internal/fixprguard`・`cmd/fixprguard`・ガイド（`b93a2202`）:**
    -   `internal/fixprguard/types.go:21` に `const Protocol = 6`。
    -   `internal/fixprguard/ignored.go:38-42` に `watchTopDir = ".claude"`、`watchTopFiles`、`watchNames = []string{"claude.md", "makefile", "gnumakefile", "go.mod", "go.work"}`。`:202,207-211` に `errClaudeGitName` と `ignoredFailures`、`:386-422` に `checkClaudeGitNames`。再利用対象: `foldEqual`（`:133`）、`walkIgnoredDir`（`:429`）。
    -   `internal/fixprguard/scope.go:54` の `instructionFileNames` に 6 ファイル（`claude.md`・`agents.md`・`claude.local.md`・`agent.md`・`gemini.md`・`agents.override.md`）が定義済み。
    -   `internal/fixprguard/ignored_test.go:60` の `TestIsWatched` が `{"agents_md", "AGENTS.md", false}` を持ち、`:144` の `TestWatchListPatternsMatch` が要素数 `28`（`+5` で `33`）を固定検証している。`:290`（`TestCollectTargetsGitFailures`）・`:395-440`（`TestCollectTargetsClaudeGitNames`）・`:957`（`TestCheckStateIgnoredChanges`）に `.claude/` 向けの境界テストがある。
    -   `cmd/fixprguard/docs_test.go:430-440`（`TestDocumentedProtocolMatches`、`runpr.md:220` と `fixprguard_guide.md` 5 章の `"protocol":<N>` 出力例も検査）・`:537-546`（`TestGuideListsWatchList`）・`:591-605`（`TestGuideDescribesIgnoredFiles`）はコードの公開関数から動的に読むため変更不要でそのまま検証に使える。
    -   `docs/dev/guides/fixprguard_guide.md:66`（プロトコル番号）、5 章（`--protocol`・`"protocol":<N>` 出力例）、`:452-459`（7.1 監視の一覧）、`:482,520`（7.2.3・7.2.5 の旧「`AGENTS.md`・`GEMINI.md` 等は一覧外」記述）、`:613,617`（8.1 の `find .claude`）、`:632-646`（8.2 規則表、`:638,642` の `.claude/` 記述含む）に更新が必要。
-   **`internal/chief`・`internal/chief/paneinput`・`cmd/chief`（`b93a2202`）:**
    -   `internal/chief/records.go:29` に `const Protocol = 6`。
    -   `internal/chief/task.go:237-255,274` に `RequirementOpus`・`RequirementSonnetReview`・`RequirementSonnet`、`requirementValues`、`oldValueRow`。
    -   `internal/chief/routing.go:27-34` に `routingRows`（`"runplan-opus"`・`"runplan-sonnet+opus-review"`・`"runplan-sonnet"`）。`ParseRouting`（`:56-104`）は `### Command routing` のみを読み、`ParseOldValues`（`task.go:283-312`）は `### PR marker conventions` のみを読む。
    -   `internal/chief/paneinput/paneinput.go:152-177,352` に `RouteRunplanOpus`・`RouteRunplanSonnetReview`・`RouteRunplanSonnet`、`RouteKeys()`。参照箇所は `internal/chief/state.go:273-278,471` と `internal/chief/skiprecord.go:132`。
    -   `internal/chief/test_helpers.go:38` の `testTools` は `git`・`awk`・`grep`・`tr`・`tail` のみで `"env"` を持たず（`:31` のコメントも `opus-review-decision.sh` を指す）、`:558-561` の `testOldValues` は `"frontier-required"` と `"standard"` の 2 件のみ、`internal/chief/task_test.go:23` の `testContext` は `"standard"` のみである。`high-review-decision.sh` に `exec env -i` を導入する際は `testTools` へ `"env"` を追加し、既存テストの旧値フィクスチャが壊れないよう `testOldValues` と `testContext` を 6 旧値すべてへ広げる必要がある。
    -   `internal/chief/task_test.go` の計画書読み取りテスト関数は `TestParseOldValues`（`:37`）・`TestReadModelRequirement`（`:47`）・`TestParsePlanProgress`（`:106`）・`TestParsePlanRejects`（`:164`）。`internal/chief/testdata/plans/` には新 3 値（`high`・`standard+high-review`・`low`）および `frontier-recommended` の標本が未配置。
    -   `internal/chief/markers_parity_test.go:17,25-34` が `opus-review-decision.sh` と `NONE value-sonnet` を参照している。`docs/dev/chief_architecture.md` には具体のプロトコル番号やモデル固有名の行キーは書かれていない（`docs/dev/guides/chief_guide.md:30` がプロトコル番号を持つ）。
-   **判定スクリプトと関連テスト（`b93a2202`）:**
    -   `.claude/commands/_lib/opus-review-decision.sh`（bash 4+、`markers_parity_test.go` が実行環境確認済み）: `:139-151` に `old_value_table`（`2>/dev/null` でエラー抑制あり）、`:214` に `NONE value-sonnet`、`:253` に `Opus review: done`、`:311` に `claude-opus-*` ハードコード、`:331` に `NONE all-opus`。環境クリーンアップ・`set -o pipefail`・`GIT_NO_REPLACE_OBJECTS=1` は未導入。呼び出し元 `.claude/commands/_lib/opus-review.md:176` でも `--no-replace-objects` をまだ指定していない。
    -   `.claude/commands/_lib/review-records.sh`（bash 3.2 互換、macOS の `/bin/bash` 3.2.57 と `PATH` の bash 5.3.15 で実行可能を確認済み）: `:186,513,597` に `fable` モード、`:195-197` に `claude-fable-[a-z0-9.-]{1,48}`・`not-fable`・`Fable review:` ハードコード。
    -   `cmd/yt2column/test_helpers_bash.go:21-39` の `scriptBashes(t)` は `path-bash` と macOS `/bin/bash`（3.2）を返し、bash 4+ の判定を持たない。`high-review-decision.sh` 用に bash 4+ を要求するヘルパ（`requireBash4`、`//go:build test` の非 `_test.go` ファイルであるため `exec.Command` に最小範囲の `//nolint:gosec` と理由コメントを付与）を同ファイルへ追加して再利用する。
    -   `cmd/yt2column/review_records_test.go`（1,401 行）: `fableReviewPath`（`:43`）、`newRRFixture`（`:98`、`_context.md` を未配置）、`TestReviewRecordsMode`（`:723`）、`readFableForms`（`:743`）、`TestReviewRecordsFable`（`:1154`）、`TestReviewRecordsCallSite`（`:1244`）、`opusReviewPath`（`:1279`）、`TestRecordLinePrefixesDistinct`（`:1330`）、`TestRecordCommitsCheckMention`（`:1377`）。再利用対象: `cmd/yt2column/test_helpers_gitfixture.go`・`cmd/yt2column/testdata/review_records/`。
    -   `cmd/yt2column/fable_state_docs_test.go`: `TestFableStateCheckBlock`（`:46`）が `_lib/fable-review.md` の `## The state check` ブロックを実行検証する。
-   **コマンド定義・文書ガード・仕様文書・ADR（`b93a2202`）:**
    -   `.claude/commands/` の非 `chief` コマンドは 11 ファイル（`fixpr.md`・`japrose.md`・`mergepr.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`mkreq.md`・`mktrans.md`・`runplan.md`・`runpr.md`・`weakreview.md`）。`mergemain.md` は存在しない。
    -   `.claude/commands/_context.md` は `:5`（`Claude Commands`）・`:52`（`PushNotification`）・`:73,98,120`（`_lib/opus-review.md`）・`:210`（`Write tool`）にハーネス固有・改名前の記述を持つ。
    -   `internal/doctestutil/protocol.go:111-129`（`CommandDocs`）は `.claude/commands` 配下の全 `.md` を走査し、5 パッケージ 7 ファイル（`cmd/yt2column/{codex_review_docs_test.go,adr_candidates_docs_test.go,review_records_test.go}`・`cmd/{chief,fixprguard,mergepr,numreserve}/docs_test.go`）から呼ばれる。したがってフェーズ 3 で `.claude/commands/_lib/harness*.md` を追加する時点で、`TestNoDirectHerdrInvocation`・`cmd/numreserve` の `TestDocumentedInvocations`・`TestPostingCommandsInventory` が即座に適用される。
    -   `cmd/yt2column/docpaths_test.go:106-131`（`instructionDocs`）は `CLAUDE.md`・`AGENTS.md`・`.claude/commands`・`docs/dev` を走査する。
    -   `cmd/yt2column/fixpr_docs_test.go:42` が `fixprPhase6 = "## Phase 6 — Wrap: PR description (model: haiku)"` を定数保持し、`TestFixprguardCallSites`（`:861-862`）・`TestFixprPhase6CountFilter`（`:894-942`）・`TestRemovedCommandLiterals`（`:1257`）が参照する。
    -   `CLAUDE.md:48-75`（Document language）、`:161-165`（Language）、`:216-242`（コマンド概要）、`:841,965-966,1009`（`Opus review's decision script`・`opus-review-decision.sh` 参照）。`AGENTS.md`（21 行）に Antigravity 禁止規則の明記が未追加。
    -   `docs/dev/adr/` で `#1072` の食い違い記述を持つ ADR: `0013:56`・`0014:56`・`0035:24`・`0042:23`・`0046:50`・`0047:21`・`0050:35`・`0055:25`。また `0014:33`・`0016:25`・`0042:21,46`・`0050:33,76` に改名前パスまたは設計前の暫定記述がある。
    -   CI の `doc-guard` ジョブ（`.github/workflows/ci.yml:215`）は `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief` を実行する。
    -   本計画書の `**実装モデル要件**` 欄は、PR-2 マージ前の現行 `chief`・`opus-review-decision.sh` で読める現行値（`opus`・`sonnet+opus-review`・`sonnet`）で記述し、PR-2 マージ後は `_context.md` の旧値読み替え表（AC-14）により自動的に新役割名として読まれる形とする。

### 1.2 計画の際に決めたこと（2026-11-22）

利用者に確認して決めたこと:

-   **フェーズ 4 の PR 分割（5 PR 構成）:** 設計書 8 章のフェーズ順（フェーズ 1〜4）を維持したまま、フェーズ 4 を PR-4（共有手順・全 11 コマンド正本の改名とハーネス非依存化・関連テスト更新）と PR-5（`.agents/skills/` 11 ファイル新設・`CommandDocs` 拡張・構造ガード・関連文書と ADR の最終更新・`_checks/` 検証）の 2 本に分ける（全体で 5 PR 構成）。
    -   **理由:** 11 コマンドと 8 共有手順のハーネス非依存化だけでも約 25〜30 ファイルの変更になるため、新規追加である `.agents/skills/` 群・`CommandDocs` 拡張・関連文書の最終更新を PR-5 に切り離すことでレビュー規模を抑え、各 PR が独立してグリーンゲートを通せるようにするためである。あわせて、設計書 8 章のフェーズ 4 の括弧書きを編集上の訂正（決定の変更なし）として合わせた。
    -   **ADR の判断:** ADR なし（タスク内の PR 分割であり、タスクをまたぐ設計決定ではないため）。
-   **各 PR の `**自動送信**` の設定:** PR-1・PR-2・PR-3 を `hold`、PR-4・PR-5 を `auto` とする。
    -   **理由:** PR-1 着手前には前提タスク `#1059`（タスク 0118）のマージ完了待ち、PR-2 着手前には PR-1 マージ後の `main` からの `make install-fixprguard`、PR-3 着手前には PR-2 マージ後の `/chief` 停止・`main` からの `make install-chief`・`/chief` 再起動という `/runplan` 外の手動作業が入るためである。
    -   **ADR の判断:** ADR なし（本タスクの計画書内の `**自動送信**` 設定であり、タスクをまたぐ設計決定ではないため）。

計画上の配置と検証手段の選択:

-   **新規テストファイルの配置:** `high-review-decision.sh` のフィクスチャテストおよび呼び出し箇所ガードは `cmd/yt2column/high_review_decision_test.go`（新規）に、`.agents/skills/` の 1 対 1・非 `chief` 構造ガードは `cmd/yt2column/skills_docs_test.go`（新規）に置く。既存の `cmd/yt2column/review_records_test.go`（1,401 行）の肥大化を防ぎつつ、既存の `test_helpers_gitfixture.go`・`test_helpers_bash.go` および CI `doc-guard` 対象パッケージ（`./cmd/yt2column`）をそのまま再利用できるためである。ADR の判断: ADR なし（タスク内で閉じるテストファイル配置のため）。
-   **状態チェックテストファイルの改名:** `cmd/yt2column/fable_state_docs_test.go` はフェーズ 4（PR-4）で `cmd/yt2column/cross_check_state_docs_test.go` へ改名する（改名だけの独立コミットと中身変更コミットを分ける）。ADR の判断: ADR なし（テストファイル名でありタスク内で閉じるため）。
-   **AC-02・AC-41（シェルスクリプト数）の静的検証スクリプトと AC-32・AC-14 の差分検証:** AC-02（非アダプタのコマンド定義にハーネス固有ツール呼び出しを残さないこと）と AC-41（第 3 の判定シェルスクリプトを増やさないこと）を検証するスクリプトを `docs/tasks/0120_harness_independent_commands/_checks/check_harness_tools.sh` に置く（設計書 7 章・8 章）。AC-32（`.claude/settings.json` の非変更）および AC-14（既存計画書の非変更）の確認は対象差分が小さいため、各実装 PR の差分記録（Compared paths: `.claude/settings.json`、File list 付き）で行う。ADR の判断: ADR なし（タスク専用の検証手段の選択のため）。

### 1.3 リネーム一覧

本タスクで改名するファイル・Go 識別子・スクリプトモード・理由コード・記録行接頭辞・入力パラメータ・見出しを以下に集約する。文書内のすべての参照はこの表に従う。

| 分類 | 旧名 | 新名 | 対象 | フェーズ |
|---|---|---|---|---|
| ファイル | `.claude/commands/_lib/opus-review-decision.sh` | `.claude/commands/_lib/high-review-decision.sh` | 判定スクリプト | フェーズ 2 |
| ファイル | `.claude/commands/_lib/opus-review.md` | `.claude/commands/_lib/high-review.md` | 共有手順 | フェーズ 4 |
| ファイル | `.claude/commands/_lib/fable-review.md` | `.claude/commands/_lib/cross-check-review.md` | 共有手順 | フェーズ 4 |
| ファイル | `cmd/yt2column/fable_state_docs_test.go` | `cmd/yt2column/cross_check_state_docs_test.go` | 状態チェックテスト | フェーズ 4 |
| Go 識別子 | `RequirementOpus` / `RequirementSonnetReview` / `RequirementSonnet` | `RequirementHigh` / `RequirementStandardReview` / `RequirementLow` | `internal/chief/` | フェーズ 2 |
| Go 識別子 | `RouteRunplanOpus` / `RouteRunplanSonnetReview` / `RouteRunplanSonnet` | `RouteRunplanHigh` / `RouteRunplanStandardReview` / `RouteRunplanLow` | `internal/chief/paneinput/`・`internal/chief/` | フェーズ 2 |
| Go 関数名 | `TestFableStateCheckBlock` / `fableStateParts` | `TestCrossCheckStateCheckBlock` / `crossCheckStateParts` | `cmd/yt2column/cross_check_state_docs_test.go` | フェーズ 4 |
| Go 関数名 | `TestReviewRecordsFable` / `readFableForms` / `fableForms` | `TestReviewRecordsCrossCheck` / `readCrossCheckForms` / `crossCheckForms`（互換別名テスト `TestReviewRecordsFableAlias` を併設） | `cmd/yt2column/review_records_test.go` | フェーズ 3・4 |
| ルーティング行キー | `runplan-opus` / `runplan-sonnet+opus-review` / `runplan-sonnet` | `runplan-high` / `runplan-standard+high-review` / `runplan-low` | `_context.md`・`internal/chief/routing.go` | フェーズ 2 |
| 実装モデル要件 | `opus` / `sonnet+opus-review` / `sonnet` | `high` / `standard+high-review` / `low`（旧 6 値は `_context.md` で読み替え） | `_context.md`・計画作成コマンド・`internal/chief/task.go`・`high-review-decision.sh` | フェーズ 2・4 |
| 判定理由コード | `value-sonnet` / `all-opus` | `value-low` / `all-high`（`value-absent` は維持） | `high-review-decision.sh`・`markers_parity_test.go`・`high-review.md` | フェーズ 2・4 |
| モード名 | `fable` | `cross-check`（`fable` は移行互換別名として維持） | `review-records.sh`・呼び出し元手順・`review_records_test.go` | フェーズ 3・4 |
| 入力パラメータ | `FABLE` | `CROSS_CHECK` | `_lib/local-cli-review.md`・`_lib/task-pr-handoff.md`・`runplan.md`・`_lib/cross-check-review.md` | フェーズ 4 |
| 記録行接頭辞 | `Opus review: done` / `Opus review: kept` | `High review: done` / `High review: kept`（旧行両読み） | `_lib/high-review.md`・`high-review-decision.sh` | フェーズ 2・4 |
| 記録行接頭辞・理由 | `Fable review: run` / `not run`（`not-fable`） / `checkpoint`、`Fable scope: in` / `out`、`Fable finding:` | `Cross-check review: run` / `not run`（`wrong-model`） / `checkpoint`、`Cross-check scope: in` / `out`、`Cross-check finding:`（旧行・`not-fable` 両読み） | `_lib/cross-check-review.md`・`review-records.sh`・`_lib/task-pr-handoff.md`・`runpr.md` | フェーズ 3・4 |
| PR 説明見出し | `## Fable の部分レビュー` / `<details><summary>Fable の行</summary>` | `## クロスチェックの部分レビュー` / `<details><summary>クロスチェックの行</summary>` | `_lib/task-pr-handoff.md`・`_lib/cross-check-review.md` | フェーズ 4 |
| 見出し定数 | `fixprPhase6`（`(model: haiku)`） | `fixprPhase6`（役割 `low` 表記へ更新） | `.claude/commands/fixpr.md`・`cmd/yt2column/fixpr_docs_test.go` | フェーズ 4 |

---

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: `fixprguard` の監視拡張とプロトコル番号更新

設計書 3.6.1・3.8・7.1・7.5・8 章（フェーズ 1）に対応する。

> [!IMPORTANT]
> **着手前提（要件定義書 4.4）:** 前提タスク `#1059`（タスク 0118：Codex 撤去）の実装 PR が `main` にマージされた後に本フェーズに着手し、PR をマージする。

#### ステップ 1-1: `internal/fixprguard` の監視の一覧・ネスト `.git` 検査・プロトコル番号を更新する（F-006、AC-36）

**変更ファイル:**
- `internal/fixprguard/ignored.go`
- `internal/fixprguard/types.go`

**作業内容:**
- [x] `internal/fixprguard/ignored.go` の単一の `watchTopDir` をスライス `watchTopDirs = []string{".claude", ".agents"}` に拡張し、`WatchListPatterns()` と `isWatched()` が `.claude/` と `.agents/` の双方を先頭一致（`foldEqual`）で扱うようにする（設計書 3.6.1）。
- [x] `watchNames` に `"agents.md"`・`"agent.md"`・`"gemini.md"`・`"agents.override.md"` を追加する（`watchTopFiles` の `"claude.local.md"` の既存仕様は維持）。
- [x] 番兵エラー `errAgentsGitName`（`ErrGitFailed` を元のエラーに含め、規則名 `ignored .git name below .agents`）を追加し、`ignoredFailures` に登録する。既存の `errClaudeGitName` を維持したまま、`.claude`・`.agents` 双方の直下走査で `.git`（`foldEqual` 一致）を検出し、走査上限 `lim.walkEntries` を両ディレクトリの合算で適用する。
- [x] `internal/fixprguard/types.go` の `Protocol` を実装時の `origin/main` の値 + 1 へ引き上げる。

**完了基準:**
- `go test -tags test ./internal/fixprguard` がビルドできる（テスト更新はステップ 1-2）。

#### ステップ 1-2: `fixprguard` の単体テスト・呼び出し元・ガイドを更新する（F-006、AC-36）

**変更ファイル:**
- `internal/fixprguard/ignored_test.go`
- `.claude/commands/fixpr.md`
- `.claude/commands/runpr.md`
- `docs/dev/guides/fixprguard_guide.md`
- `docs/dev/adr/0055-harness-independent-commands.md`

**作業内容:**
- [x] `internal/fixprguard/ignored_test.go` を次のとおり更新し、実装時に各新検査を 1 か所ずつ壊して該当テストが失敗することを確認する。
    - `TestIsWatched` の `AGENTS.md` を `true` へ反転し、`.agents/` の境界ケース（`a/.agents/...`・`.agents.bak/...` 等）と、任意階層の 4 指示ファイル（大文字・小文字混在や Unicode 単純ケースフォールディングを含む）の監視検出を追加する。
    - `TestWatchListPatternsMatch` の期待件数を `33` へ更新する。
    - `.claude` 向けの境界ケース（`.git` ファイル・`.GIT`・`a.git`・ネストリポジトリ・リンクされた worktree・最上位ネストリポジトリ・非無視の読取不能サブディレクトリによる `errReadIgnoredDir`・合算 `walkEntries` 上限）を `.agents` にも対称に追加し、`errAgentsGitName` を検証する。
- [x] `.claude/commands/fixpr.md` と `.claude/commands/runpr.md` の全 `fixprguard --protocol` および `runpr.md:220` の `"protocol":<N>` 出力例を `origin/main` の値 + 1 に更新する。
- [x] `docs/dev/guides/fixprguard_guide.md` の `--protocol N` の現在の番号（`:66`）・5 章の全 `--protocol` / `"protocol":<N>` 出力例、7.1 の監視の一覧表と本文、7.2.3・7.2.5 の旧「`AGENTS.md`・`GEMINI.md` 等は一覧外」記述、8.1 の本文と一覧コマンド（`:613,617`。`.agents` が存在しないときもエラーにならない存在確認付きとする）、8.2 の規則表（`ignored .git name below .agents` 追加、および `:638,642` の `.claude/` 記述へ `.agents/` を併記）を更新する。
- [x] `ADR-0055`（`:25`）の `#1072` 食い違い記述から、本 PR で解消した `fixprguard` 監視拡張分を取り除き、残余の食い違いのみを示す形へ書き換える（`docs/dev/conventions/adr.md`「Discrepancy statements」、コミットメッセージに `Editorial correction:` 行を残す）。
- [x] `cmd/fixprguard/docs_test.go` を実行し、`TestGuideListsWatchList`・`TestGuideDescribesIgnoredFiles`・`TestDocumentedProtocolMatches` がすべて通ることを確認する。

**完了基準:**
- `make test && make lint && make deadcode` および `doc-guard` の全対象パッケージが通る。

#### ステップ 1-3: フェーズ 1 の AC-32・AC-14 差分記録と完了確認（AC-14・AC-32・AC-43）

**変更ファイル:**
- `docs/tasks/0120_harness_independent_commands/03_implementation_plan.md`（§7 の差分記録）

**作業内容:**
- [x] §7.2 の PR-1 差分記録（`Diff record (AC-32)`）に `.claude/settings.json` の Path diff（空）と全変更ファイル一覧（`File list`）を記録し、`.claude/settings.json` および他タスクの計画書が変更されていないことを確認する。

**完了基準:**
- グリーンゲートがパスし、§7.2 の PR-1 差分記録が最新 HEAD と一致する。

### PR-1 作成ポイント: fixprguard watch list and protocol bump

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `feat(0120): fixprguard の監視の一覧を .agents と指示ファイルへ拡張する`

**レビュー観点**: watchTopDirs と 4 指示ファイルの Unicode 単純ケースフォールディング監視 / .agents 配下ネスト .git 検出（errAgentsGitName）と合算 walkEntries 上限 / fixprguard.Protocol 更新と fixpr.md・runpr.md・fixprguard_guide.md の同時整合

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-1・1-2 が独立した高リスク領域（fixprguard の監視境界・Unicode ケースフォールディング・ネスト .git 検査と合算上限）に該当し、Sonnet では foldEqual の適用漏れや .claude/.agents 合算 walkEntries 上限・.agents 不在時処理の境界条件を見落として監視すり抜けや誤検知を招くおそれがあるため。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1090
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 2: `_context.md`・`chief`・`high-review-decision.sh` 原子改名とパリティ維持

設計書 3.3・3.4・3.6.2・3.8・7.1・7.2・7.3・7.5・8 章（フェーズ 2）に対応する。

> [!IMPORTANT]
> **着手前の手動作業:** フェーズ 1 の PR マージ後に、利用者が `main` から `make install-fixprguard` を実行してから着手する。

#### ステップ 2-1: `_context.md` にハーネス別モデル対応表・6 旧値読み替え・改名行キーを反映する（F-003・F-007、AC-12・AC-14・AC-15・AC-16・AC-17・AC-39）

**変更ファイル:**
- `.claude/commands/_context.md`

**作業内容:**
- [x] `### PR marker conventions` の part 1〜5 を設計書 3.3 に従って更新する（正規 3 値 `high`・`standard+high-review`・`low`、旧値読み替え表 6 行、part 5 の `Command routing` と `Harness model mapping` の使い分け、`high-review-decision.sh` への名称更新）。
- [x] `### Command routing` の `/runplan` 用 3 行キーを 1.3 のリネーム一覧どおり改名する（他 3 行と `Claude Code`・`OpenCode` の 2 値は維持）。
- [x] `### Command routing` の直後（`## Domain-specific` より前）に独立した節 `### Harness model mapping` を新設し、設計書 3.3 の固定 5 列・9 行の表と厳密文法規則を記述する。

**完了基準:**
- `_context.md` の新表・旧値表・ルーティング表が設計書 3.3 と一致し、既存パーサ（`contextRow` 等）と衝突しない。

#### ステップ 2-2: `internal/chief`・`paneinput`・`chief` 文書・プロトコル番号を更新する（F-007、AC-12・AC-14・AC-15・AC-37・AC-39・AC-40）

**変更ファイル:**
- `internal/chief/task.go`
- `internal/chief/routing.go`
- `internal/chief/state.go`
- `internal/chief/skiprecord.go`
- `internal/chief/records.go`
- `internal/chief/paneinput/paneinput.go`
- `internal/chief/task_test.go`
- `internal/chief/routing_test.go`
- `internal/chief/send_test.go`
- `internal/chief/snapshot_test.go`
- `internal/chief/maintask_test.go`
- `internal/chief/running_test.go`
- `internal/chief/autosend_test.go`
- `internal/chief/test_helpers.go`
- `internal/chief/testdata/plans/`（新 3 値・`single-low`・`frontier-recommended` 標本追加）
- `internal/chief/testdata/README.md`
- `internal/chief/paneinput/paneinput_test.go`
- `.claude/commands/chief.md`
- `docs/dev/guides/chief_guide.md`
- `docs/dev/chief_architecture.md`（該当箇所があれば更新し、`TestArchitectureValuesMatchTool` との整合を確認）

**作業内容:**
- [x] 1.3 のリネーム一覧どおり、`internal/chief/task.go`（`Requirement*`・`requirementValues`・`oldValueRow`）、`internal/chief/routing.go`（`routingRows`）、`internal/chief/paneinput/paneinput.go`（`RouteRunplan*`・`RouteKeys()`）、`internal/chief/state.go`、`internal/chief/skiprecord.go` を更新する。
- [x] `internal/chief/records.go` の `Protocol` を実装時の `origin/main` の値 + 1 へ上げ、`.claude/commands/chief.md` の全 `chief --protocol` と `docs/dev/guides/chief_guide.md` のプロトコル番号表（`:30`）を更新し、`docs/dev/chief_architecture.md` を確認する。
- [x] `internal/chief/test_helpers.go` の `testTools` に `"env"` など `high-review-decision.sh` が呼ぶ外部コマンドを追加し、`:31` コメントを `high-review-decision.sh` へ更新したうえで、`testOldValues` と `task_test.go` の `testContext` を全 6 旧値へ拡張する。`internal/chief/testdata/plans/` に新 3 値・`single-low`・`frontier-recommended` の標本ファイルを追加し（Go リテラルで書かない）、`internal/chief/testdata/README.md` を更新する。
- [x] `internal/chief` と `paneinput` のテストを更新する。`TestParseOldValues` には実 `_context.md`（`repoContext(t)`）に対する 6 件の厳密一致と `### Harness model mapping` からの非干渉の検証を加える。また、`TestReadModelRequirement`・`TestParsePlanProgress`・`TestParsePlanRejects`・`routing_test.go` で新 3 値・全 6 旧値・異常系（`RequirementUnreadable`・`errTaskDocument`・`errRouting`、未知エージェント `Antigravity` を含む）を検証し、実装時に判定を壊して該当テストが失敗することを確認する。

**完了基準:**
- `go test -tags test ./cmd/chief ./internal/chief/...` のうち `markers_parity_test.go` 以外がすべて通る。

#### ステップ 2-3: `opus-review-decision.sh` を `high-review-decision.sh` へ原子改名・一般化し、パリティテストとフィクスチャテストを整備する（F-004・F-007、AC-18〜AC-20・AC-23・AC-38）

**変更ファイル:**
- `.claude/commands/_lib/opus-review-decision.sh` → `.claude/commands/_lib/high-review-decision.sh`（改名・更新）
- `.claude/commands/_lib/opus-review.md`（呼び出し先スクリプト名・`--no-replace-objects`・理由コード・新旧完了記録行の先行追随）
- `internal/chief/markers_parity_test.go`
- `cmd/yt2column/test_helpers_bash.go`
- `cmd/yt2column/high_review_decision_test.go`（新規）
- `CLAUDE.md`（判定スクリプト名・名称参照箇所）
- `docs/dev/adr/0014-deterministic-checks-in-go.md`
- `docs/dev/adr/0035-language-policy.md`
- `docs/dev/adr/0042-implementation-model-tiers.md`
- `docs/dev/adr/0046-chief-of-staff.md`

**作業内容:**
- [x] `_lib/opus-review-decision.sh` を `_lib/high-review-decision.sh` へ改名し、設計書 3.3・3.4・5.2 に従って更新する。具体的には、環境クリーンアップ（`exec env -i`・`set -o pipefail`・`GIT_NO_REPLACE_OBJECTS=1`）、`value-low`・`value-absent` による早期返却より前の `_context.md` 全体検証と `Launch == agent` の `Role == high` パターン抽出、非正規表現照合、理由コード `value-low`・`all-high`、新旧 `High review: done` / `Opus review: done` の両読み、および失敗時の標準エラー診断＋`RUN`＋終了コード `0` を実装する。
- [x] 同じ PR で `_lib/opus-review.md` 内の呼び出し先コマンド（`:176`）を `git --no-replace-objects show <origin-main-commit>:.claude/commands/_lib/high-review-decision.sh > <decision-script>` へ更新し、出力読み取りを `value-low`・`all-high`・新旧完了記録行へ先行追随する（`_lib/opus-review.md` 本体の改名とアダプタ化はフェーズ 4 で行う）。
- [x] `internal/chief/markers_parity_test.go` を `_lib/high-review-decision.sh`・`RequirementLow`・`NONE value-low` へ更新し、新 3 値・全 6 旧値（`frontier-recommended` 含む）・読めない値・値なし・CRLF・フェンス・重複見出しでパリティ一致を検証する。
- [x] `cmd/yt2column/test_helpers_bash.go` に `requireBash4(t *testing.T) scriptBash`（`PATH` の `bash` が 4 未満・不在なら `t.Fatal`。非 `_test.go` の `//go:build test` ファイルのため `exec.Command` に最小範囲の `//nolint:gosec` と理由コメントを付与）を追加する。
- [x] `cmd/yt2column/high_review_decision_test.go` を新設する（`requireBash4` と `test_helpers_gitfixture.go` を再利用し、計画書標本は `cmd/yt2column/testdata/review_records/` から読み込んで Go ソースに日本語リテラルを書かない）。実装時に各検証規則を壊して失敗することを確認しつつ、次を検証する。
    - (a) 自己検証可能ハーネス（`Launch == agent`）での `NONE all-high`（除外可能コミット、非 `high` モデルの記録範囲にも含まれるコミット→`RUN`、マージコミット→`RUN` を含む）。
    - (b) 自己検証不能ハーネス（`Launch == swarm` の `gemini-4-neon-high`）での `RUN`。
    - (c) 新旧の `done` 記録での `SKIP <done-line>`（`done` 記録の `<END>` 以降にある除外可能コミットの出力、字下げ・不正形式の無視を含む）。
    - (d) パターン照合の境界（`.` の非リテラル扱い・48 文字接尾辞の受理と 49 文字接尾辞の拒否・空接尾辞・空白を含むモデルの ID）、および `value-low`・`value-absent` の計画書であっても `_context.md` が不正（メタ文字・表破損・欠落等）なときは早期返却より前に期待する診断を標準エラーへ出力して `RUN` となること。
    - (e) 環境隔離（`GIT_DIR`・`GIT_WORK_TREE`・`BASH_FUNC_*`・`BASH_ENV`・`refs/replace`）および不正引数・`origin/main` 欠落・`MULTIPLE`・未閉鎖フェンス・`HEAD` での要件引き下げ。
    - (f) `TestHighReviewDecisionCallSite`（`TestReviewRecordsCallSite` と同様に `doctestutil.CommandDocs(t, repositoryRoot)` が返す全ファイルを走査し、`high-review-decision.sh` を呼ぶ箇所が許可された呼び出し元（PR-2 では `_lib/opus-review.md` のみ）に限られ、`origin/main` から `--no-replace-objects` で取り出す契約を満たすこと）。
- [x] `CLAUDE.md` のスクリプト名参照（`:841,965-966,1009`）を更新し、`ADR-0014`（`:33`）・`ADR-0035`（`:24` のスクリプト名部分）・`ADR-0042`（`:23` の残っている食い違い記述）・`ADR-0046`（`:50` の食い違い記述解消）を更新する（編集上の訂正はコミットメッセージに `Editorial correction:` 行を残す）。

**完了基準:**
- `bash -n .claude/commands/_lib/high-review-decision.sh`、`make test && make lint && make deadcode`、および `doc-guard` の全対象パッケージが通る。

#### ステップ 2-4: フェーズ 2 の AC-32・AC-14 差分記録と完了確認（AC-14・AC-32・AC-43）

**変更ファイル:**
- `docs/tasks/0120_harness_independent_commands/03_implementation_plan.md`（§7 の差分記録）

**作業内容:**
- [x] §7.2 の PR-2 差分記録（`Diff record (AC-32)`）に `.claude/settings.json` の Path diff（空）と全変更ファイル一覧（`File list`）を記録する。

**完了基準:**
- グリーンゲートがパスし、§7.2 の PR-2 差分記録が最新 HEAD と一致する。

### PR-2 作成ポイント: chief role tiers and high-review decision script

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `feat(0120): 実装モデル要件を役割名へ移行し chief を追随させ判定スクリプトを改名する`

**レビュー観点**: _context.md の Harness model mapping 厳密文法と 6 旧値読み替えの既存パーサ非干渉 / internal/chief と high-review-decision.sh の全分類パリティ一致 / 判定スクリプトの環境隔離・非正規表現照合・早期返却前検証と opus-review.md 呼び出し先の同一 PR での同時追随

**実装モデル要件**: opus

**判定理由**: ステップ 2-1〜2-3 がパネルモード条件（段階的移行とセキュリティ関門：_context.md・chief・high-review-decision.sh の一括改名と 6 旧値互換、非正規表現照合・環境隔離）に該当し、Sonnet では chief との分類パリティ崩れや value-low 早期返却前の検証順序漏れ・Launch == swarm での all-high 拒否漏れを起こしやすいため。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1094
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 3: `review-records.sh` 一般化とハーネスのアダプタ 3 ファイル新設

設計書 3.1・3.5・4.1・4.2・5.2・6.1・6.3・7.3・7.5・8 章（フェーズ 3）に対応する。

> [!IMPORTANT]
> **着手前の手動作業:** フェーズ 2 の PR マージ後に、利用者が動いている `/chief` を止め、`main` を更新して `make install-chief` を実行し、`/chief` を起動し直してから着手する（`docs/dev/guides/chief_guide.md` 2 章）。あわせて、未マージの並行ブランチがあれば `git merge origin/main` で新しい呼び出し元を取り込む（設計書 8 章）。

#### ステップ 3-1: `review-records.sh` に `cross-check` モード・`_context.md` パターン検証・新旧両読みを実装する（F-005、AC-28〜AC-30）

**変更ファイル:**
- `.claude/commands/_lib/review-records.sh`

**作業内容:**
- [x] bash 3.2 互換を保ったまま、新モード `cross-check <ENTRY> <HEAD> <PLAN>` を追加し、移行互換の別名として `fable` を同一処理へ振り分ける（設計書 3.5・6.3）。`coverage` モードは `_context.md` を読まず、既存動作と失敗出力（`PARTIAL`・終了コード `1`）を変更しない。
- [x] `cross-check` / `fable` モードで `origin/main:.claude/commands/_context.md` の `### Harness model mapping` 表全体を設計書 3.3 の厳密文法で検証し、`Role == cross-check` の有効パターンを収集する。表の欠落・形式不正・有効パターン 0 件のときは、標準出力に `MISSING`、標準エラーに診断を出力し、終了コード `1` で終了する。
- [x] 実施記録・未実施記録・チェックポイント行について、1.3 のリネーム一覧どおり新旧両方の接頭辞および `wrong-model` / `not-fable` を同義として受理し、`<model-id>` を非正規表現照合（完全一致またはリテラル前方一致＋接尾辞 1〜48 文字）で検証する。
- [x] タスク 0114（`#988`）が実装時点で既に `main` にマージされている場合は、設計書 3.5 の条件付き整合（マージ後確認記録の `Role == high` パターン検証等）をあわせて反映する。

**完了基準:**
- `bash -n .claude/commands/_lib/review-records.sh` および `/bin/bash`（3.2）での構文検査が通る。

#### ステップ 3-2: `review_records_test.go` のフィクスチャテストを更新・拡張する（F-005、AC-28・AC-30）

**変更ファイル:**
- `cmd/yt2column/review_records_test.go`

**作業内容:**
- [x] `newRRFixture` に実リポジトリの `.claude/commands/_context.md` を配置し、`coverage` モードについては `_context.md` 不在でも動作するテストケースを追加する。
- [x] 既存の `fable` テストを `TestReviewRecordsCrossCheck`（`cross-check` モードの全パターンを bash 3.2・bash 4+ の双方で実行）と、最小限の互換別名テスト `TestReviewRecordsFableAlias`（`fable` モードの代表ケース: `VALID`・`MISSING <base>`・失敗時の `MISSING` と終了コード `1` を両 bash で実行）へ整理する。新旧記録行・チェックポイント行・`wrong-model` / `not-fable`・`agent-cannot-call`、および Claude Code（`claude-fable-*`）と Antigravity（`claude-opus-5-5-medium`）のモデルの ID が受理されることを検証する（`_lib/fable-review.md` の表読み取りはフェーズ 4 で `_lib/cross-check-review.md` へ一本化する）。
- [x] 設計書 7.5 のセキュリティ・境界テストを追加し、48 文字の接尾辞を受理する一方、49 文字の接尾辞や次の不正入力では `MISSING`（終了コード `1`）で拒否することを検証する: パターン長超過（リテラル 65 文字・前方一致 65 文字（接頭辞 64 文字＋`*`））、`Launch != none` での `-`、未知の `Harness`・`Role`・`Launch`、メタ文字、途中ワイルドカード、`.` の非リテラル扱い、空接尾辞、空白を含むモデルの ID、未登録モデル、重複する `(Harness, Role)`、別役割行の破損、表の欠落。実装時に各検証を壊して失敗することを確認する。
- [x] `TestReviewRecordsMode` に未知モードの拒否（`Cross-check` 等）を含めて更新する。

**完了基準:**
- `go test -tags test -v -run 'TestReviewRecords' ./cmd/yt2column` で bash 3.2・bash 4+ の全サブテストが通る。

#### ステップ 3-3: ハーネスのアダプタ 3 ファイル（`harness.md`・`harness-claude.md`・`harness-antigravity.md`）を新設し、Phase 6 隔離ガードと差分記録を追加する（F-001・F-004〜F-006、AC-03〜AC-08・AC-14・AC-21・AC-22・AC-26・AC-27・AC-32・AC-33・AC-34）

**変更ファイル:**
- `.claude/commands/_lib/harness.md`（新規）
- `.claude/commands/_lib/harness-claude.md`（新規）
- `.claude/commands/_lib/harness-antigravity.md`（新規）
- `cmd/yt2column/fixpr_docs_test.go`（`TestHarnessAntigravityPhase6Redirect` 追加）
- `docs/dev/adr/0014-deterministic-checks-in-go.md`
- `docs/dev/adr/0050-fable-partial-review.md`
- `docs/dev/adr/0055-harness-independent-commands.md`
- `docs/tasks/0120_harness_independent_commands/03_implementation_plan.md`（§7 の差分記録）

**作業内容:**
- [x] `.claude/commands/_lib/harness.md` を英語で新設し、設計書 3.1 の決定的ハーネス判別表（Antigravity は `ask_question`・`run_command` の 2 条件、Claude Code は `AskUserQuestion`・`Bash`・`Agent` の 3 条件）、10 種の抽象操作カタログ、および OpenCode フォールバック規則を記述する。
- [x] `.claude/commands/_lib/harness-claude.md` を英語で新設し、10 種の抽象操作の Claude Code 実装（`AskUserQuestion`・`Agent`＋出力 1 行目 `Model:` 検証・30 分タイマー＋`TaskStop`・`PushNotification` 等）を記述する。
- [x] `.claude/commands/_lib/harness-antigravity.md` を英語で新設し、10 種の抽象操作の Antigravity 実装を設計書 3.1・4.1・4.2・5.2・6.1 に従って記述する。`CommandDocs` の既存信頼境界ガード（`TestNoDirectHerdrInvocation`・`cmd/numreserve` `TestDocumentedInvocations`・`TestPostingCommandsInventory`）に抵触しないよう、禁止コマンド一覧と PR タイトルの安全な渡し方は生の `herdr <subcmd>` コードスパン・未許可形 `numreserve ...`・`gh pr create/edit --title ...` を書かず、`docs/dev/guides/chief_guide.md` 6 章・`CLAUDE.md`「Tool Execution Safety」・`_context.md`「Passing a title to `gh`」への参照で記述する。
- [x] `cmd/yt2column/fixpr_docs_test.go` に `TestHarnessAntigravityPhase6Redirect` を追加し、`harness-antigravity.md` 内のすべてのコードフェンスおよびインラインコード（環境変数前置や複数行折り返しを含む）から `gh ... --json ... body` の呼び出しを抽出し、1 件以上検出されること、およびその全件がスクラッチパッドへの直接リダイレクト形であり標準出力へ本文を漏らさないことを検証する（実装時にリダイレクトを外して失敗すること、および 0 件一致で失敗することを確認する）。
- [x] `ADR-0014`（`:56`）・`ADR-0050`（`:35`）・`ADR-0055`（`:25`）の `#1072` 食い違い記述から、本 PR で解消した `review-records.sh` 一般化およびハーネスのアダプタ新設分を取り除き、残余の食い違いのみを示す形へ書き換える（コミットメッセージに `Editorial correction:` 行を残す）。
- [x] §7.2 の PR-3 差分記録（`Diff record (AC-32)`）に `.claude/settings.json` の Path diff（空）と全変更ファイル一覧（`File list`）を記録する。

**完了基準:**
- `make test && make lint && make deadcode`・`doc-guard`（`TestHighReviewDecisionCallSite` を含む `CommandDocs` 経由の全 8 呼び出しテスト・`TestInstructionDocPaths`・`TestHarnessAntigravityPhase6Redirect` を含む）が通り、§7.2 の PR-3 差分記録が最新 HEAD と一致する。

### PR-3 作成ポイント: generalized review-records script and harness adapters

**対象ステップ**: 3-1 / 3-2 / 3-3

**推奨タイトル**: `feat(0120): review-records.sh を一般化しハーネスのアダプタを追加する`

**レビュー観点**: review-records.sh の bash 3.2 互換非正規表現照合と cross-check・fable 新旧両読み / _context.md 破損・未知パターン時の Fail-Closed 挙動と coverage モード非干渉 / ハーネスのアダプタ 3 ファイルの決定規則・swarm 検証・fixpr Phase 6 隔離と CommandDocs 整合

**実装モデル要件**: opus

**判定理由**: ステップ 3-1・3-3 がパネルモード条件（セキュリティ関門と段階的移行：review-records.sh の bash 3.2 非正規表現照合・新旧記録両読み、および Antigravity swarm モデル検証と fixpr Phase 6 隔離を担うアダプタ新設）に該当し、Sonnet では bash 4 固有構文の混入や正規表現誤用による照合すり抜け・CommandDocs 信頼境界違反記述の混入を起こしやすいため。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1101
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 4: 共有手順・全コマンド正本の改名とハーネス非依存化、`.agents/skills/` 新設、および文書・ガード完結

設計書 3.1・3.2・3.4・3.5・3.6.3・3.7・3.8・7.3・7.4・8 章（フェーズ 4）に対応する。レビュー規模と独立性を両立させるため、前半（ステップ 4-1〜4-4）で共有手順・全 11 コマンド正本・関連テストを一括で切り替え、後半（ステップ 4-5〜4-8）で `.agents/skills/` 新設・`CommandDocs` 拡張・構造ガード・関連文書と ADR の最終更新・`_checks/` 検証を完結させる（1.2 の決定）。

#### ステップ 4-1: 共有手順（`_lib/`）と `_context.md` の残余ツール記述を改名・一般化し、抽象操作と拡張状態チェックへ移行する（F-001・F-004〜F-006、AC-01〜AC-08・AC-21〜AC-29・AC-31・AC-36）

**変更ファイル:**
- `.claude/commands/_context.md`（`:5,52,73,98,120,210` のツール名・旧手順パス更新）
- `.claude/commands/_lib/opus-review.md` → `.claude/commands/_lib/high-review.md`（改名・更新）
- `.claude/commands/_lib/fable-review.md` → `.claude/commands/_lib/cross-check-review.md`（改名・更新）
- `.claude/commands/_lib/local-cli-review.md`
- `.claude/commands/_lib/pr-review-escalation.md`
- `.claude/commands/_lib/review-subagent-pattern.md`
- `.claude/commands/_lib/settle-decisions.md`
- `.claude/commands/_lib/simple-process.md`
- `.claude/commands/_lib/task-pr-handoff.md`

**作業内容:**
- [ ] `.claude/commands/_context.md` の `:5`（見出しの Claude 限定表現の一般化）、`:52`（`PushNotification` → `notify-user`）、`:73,98,120`（`_lib/opus-review.md` → `_lib/high-review.md`）、`:210`（`Write tool` → `file-io`/`scratchpad`、`_lib/harness.md` 参照）を更新する。
- [ ] `_lib/opus-review.md` を `_lib/high-review.md` へ改名し、1.3 のリネーム一覧どおり `High review: done` / `kept`（旧行も両読み）へ更新する。あわせて、`pinned-worker`（役割 `high`）への委譲、共通状態チェックの参照、フェーズ 2 で入れた `--no-replace-objects` の維持、および失敗時の Fail-Closed 停止を反映する（設計書 3.4・4.1・6.2）。
- [ ] `_lib/fable-review.md` を `_lib/cross-check-review.md` へ改名し、1.3 のリネーム一覧どおり `CROSS_CHECK`・`cross-check` モード呼び出し・`Cross-check ...:` 記録行・`wrong-model`（旧行と `not-fable` も両読み）へ更新する。あわせて、`.agents/skills/` を `command`・その他の `.agents/` を `agent-config` とする分類、`pinned-worker`（役割 `cross-check`）への委譲、および `## The state check` の `watched` 拡張（`.agents/`・4 指示ファイル・`.claude` / `.agents` 配下の `.git` 検出・一時ファイル名 `cross-check-index.XXXXXX`）を反映する（設計書 3.5・5.2）。タスク 0114（`#988`）がマージ済みなら、マージ後確認の `high` 役割への一般化も反映する（AC-28）。
- [ ] `_lib/local-cli-review.md`・`_lib/pr-review-escalation.md`・`_lib/review-subagent-pattern.md`・`_lib/settle-decisions.md`・`_lib/simple-process.md`・`_lib/task-pr-handoff.md` の 6 ファイルからハーネス固有ツール名とモデル固有名を除去し、抽象操作へ置き換える。あわせて、`_lib/pr-review-escalation.md`（`high` 固定 `pinned-worker`・状態チェック・起動や割り当てやモデルや JSON やタイムアウトの失敗時は Fail-Safe の `ask`、状態チェック差分または不正ヘルパー検出時は Fail-Closed 停止）、`_lib/review-subagent-pattern.md`（段階指定と確認パスの役割指定化）、`_lib/task-pr-handoff.md`（`CROSS_CHECK`・新 PR 説明見出し・新旧記録行および `High review: kept` / `scope:` / `finding:` の両読み）を更新する。

**完了基準:**
- 上記 9 ファイルの相互参照が新ファイル名・新パラメータ名で一致し、設計書 3.1・3.4・3.5・4.1 の契約を満たす。

#### ステップ 4-2: 全 11 コマンド正本をハーネス非依存化・新役割名・新パスへ更新する（F-001・F-003〜F-005、AC-01〜AC-08・AC-12・AC-13・AC-17・AC-19・AC-20・AC-22・AC-24・AC-29・AC-30）

**変更ファイル:**
- `.claude/commands/mkreq.md`
- `.claude/commands/mkarch.md`
- `.claude/commands/mkplan.md`
- `.claude/commands/mkplan2.md`
- `.claude/commands/runplan.md`
- `.claude/commands/runpr.md`
- `.claude/commands/fixpr.md`
- `.claude/commands/mergepr.md`
- `.claude/commands/weakreview.md`
- `.claude/commands/japrose.md`
- `.claude/commands/mktrans.md`
- （実装時点で `main` に `chief.md` 以外の利用者起動コマンドが追加されていれば同様に対象へ含める）

**作業内容:**
- [ ] 全 11 コマンドのハーネス固有ツール名・モデル固有名を `_lib/harness.md` の抽象操作と役割名（`high`・`standard`・`low`・`cross-check`）へ置換し、`_lib/high-review.md`・`_lib/cross-check-review.md`・`CROSS_CHECK` を参照するよう更新する。
- [ ] `mkplan2.md`・`mkplan.md`・`mkreq.md` の `**実装モデル要件**` の値を `high`・`standard+high-review`・`low` に、`判定理由` の規則を AC-13 に更新する。
- [ ] `runplan.md` と `runpr.md` に AC-17（新旧値表示・`_context.md` に基づくハーネス別推奨モデルと上位モデルのレビュー見込み案内）、AC-19/AC-20（Antigravity では `Implementation model:` 行を書かない規則）、新旧記録行（`Cross-check ...:`・`Fable ...:`・`High review:`・`Opus review:`）とクロスチェック要約の両読みを反映する。
- [ ] `fixpr.md` Phase 6 の見出しを役割 `low` 表記へ改め（`TestFixprPhase6CountFilter` の件数取得コマンド契約は維持）、`japrose.md` の `MODEL` 指定を `standard` 役割へ改め、`weakreview.md` の引数仕様・作業ツリー検査・`done` 非記録（AC-24）を維持して `_lib/high-review.md` へ接続する。

**完了基準:**
- 全 11 コマンドが抽象操作・新役割名・新パスで整合している。

#### ステップ 4-3: スクリプト・状態チェック・`fixpr`・`chief` テストと関連 ADR リンクを更新する（F-004〜F-006・F-008、AC-06・AC-12・AC-18・AC-23〜AC-25・AC-29・AC-30・AC-36・AC-44）

**変更ファイル:**
- `cmd/yt2column/fable_state_docs_test.go` → `cmd/yt2column/cross_check_state_docs_test.go`（改名コミットと中身更新コミットを分ける）
- `cmd/yt2column/review_records_test.go`
- `cmd/yt2column/fixpr_docs_test.go`
- `cmd/yt2column/high_review_decision_test.go`
- `cmd/chief/docs_test.go`
- `docs/dev/adr/0013-review-setup.md`
- `docs/dev/adr/0016-document-test-policy.md`
- `docs/dev/adr/0042-implementation-model-tiers.md`
- `docs/dev/adr/0047-runpr-merge-question.md`
- `docs/dev/adr/0050-fable-partial-review.md`
- `docs/dev/adr/0055-harness-independent-commands.md`

**作業内容:**
- [ ] `cmd/yt2column/fable_state_docs_test.go` を `cmd/yt2column/cross_check_state_docs_test.go` へ改名する独立コミットを作る。そのうえで `_lib/cross-check-review.md` の `## The state check` を対象に `TestCrossCheckStateCheckBlock` へ更新し、git 無視された `.agents/skills/evil/SKILL.md` と全 4 指示ファイル（`AGENTS.md`・`AGENT.md`・`GEMINI.md`・`AGENTS.override.md`）で `status` 不変のまま `watched` ハッシュが変わること、および `.claude/` と `.agents/` 双方の配下の `.git` エントリ検出（`watched` 変化または非ゼロ終了）を検証する。
- [ ] `cmd/yt2column/review_records_test.go` の参照パスを `_lib/cross-check-review.md`・`_lib/high-review.md` へ更新し、`readCrossCheckForms` が新表から新形式を読み取る形へ一本化する。あわせて、`TestReviewRecordsCallSite`（`cross-check` モードのみ許可）、`TestRecordLinePrefixesDistinct`（文書から抽出した全接頭辞と旧互換接頭辞 `Opus review:`・`Fable review:`・`Fable scope:`・`Fable finding:` の全組で相互非衝突を検証）、および `TestRecordCommitsCheckMention`（`high-review.md` を含むメンション確認）を更新する。
- [ ] `cmd/yt2column/fixpr_docs_test.go` の `fixprPhase6` 定数を新しい見出しに合わせ、`TestFixprPhase6CountFilter`・`TestFixprguardCallSites`・`TestRemovedCommandLiterals` がすべて通ることを確認する。
- [ ] `cmd/yt2column/high_review_decision_test.go` の `TestHighReviewDecisionCallSite` が全 `CommandDocs` 走査を維持したまま許可呼び出し元を `_lib/opus-review.md` から `_lib/high-review.md` へ切り替える。また、`cmd/chief/docs_test.go` に `TestMarkerFormsCarryModelRequirement` を追加し、`mkplan2.md` step 6 から抽出した 3 値（`high`・`standard+high-review`・`low`）が `mkplan2.md` step 6 と `mkplan.md` step 8.5（`<value>` 置換）の双方で `chief.ParsePlan` により対応する `Requirement` として読み取られることを検証する。実装時に各新規ガードを壊して失敗することを確認する。
- [ ] `ADR-0016:25` の `_lib/opus-review.md` リンクを `_lib/high-review.md` へ訂正する。あわせて、`ADR-0013`・`ADR-0047` の食い違い記述、`ADR-0042`（`:21,23,46`）・`ADR-0050`（`:33,35,76`）の旧パス・食い違い記述・設計前の暫定記述を解消し、`ADR-0055`（`:25`）の食い違い記述から PR-4 で解消したコマンド・共有手順・状態チェック分を取り除いて PR-5 の残余のみへ書き換える（編集上の訂正はコミットメッセージに `Editorial correction:` 行を残す）。

**完了基準:**
- `make test && make lint && make deadcode` および `doc-guard` の全対象パッケージが通る。

#### ステップ 4-4: 前半（PR-4）の AC-32・AC-14 差分記録と完了確認（AC-14・AC-32・AC-43）

**変更ファイル:**
- `docs/tasks/0120_harness_independent_commands/03_implementation_plan.md`（§7 の差分記録）

**作業内容:**
- [ ] §7.2 の PR-4 差分記録（`Diff record (AC-32)`）に `.claude/settings.json` の Path diff（空）と全変更ファイル一覧（`File list`）を記録する。

**完了基準:**
- グリーンゲートがパスし、§7.2 の PR-4 差分記録が最新 HEAD と一致する。

### PR-4 作成ポイント: harness-independent commands and shared procedures

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4

**推奨タイトル**: `refactor(0120): 共有手順と全開発コマンド正本をハーネス非依存へ移行する`

**レビュー観点**: 共有手順・全 11 コマンド正本からのハーネス固有ツール名とモデル固有名の除去 / High review・Cross-check review 記録行の新旧両読みと接頭辞非衝突・メンション確認 / cross_check_state_docs_test.go 改名コミット分離と fixpr・chief テストおよび ADR 追随

**実装モデル要件**: opus

**判定理由**: ステップ 4-1〜4-3 がパネルモード条件（段階的移行とセキュリティ関門：8 共有手順・全 11 コマンド正本の一括改名とハーネス非依存化、共通状態チェックの .agents/・.git 拡張、新旧記録行接頭辞の非衝突・メンション確認）に該当し、Sonnet では多ファイル横断の旧名・固有ツール名取りこぼしや状態チェックの watched 拡張漏れ・Fail-Closed 停止条件の弱化を起こしやすいため。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 4-5: `.agents/skills/<cmd>/SKILL.md`（11 ファイル）を新設する（F-002、AC-09〜AC-11）

**変更ファイル:**
- `.agents/skills/mkreq/SKILL.md`（新規）
- `.agents/skills/mkarch/SKILL.md`（新規）
- `.agents/skills/mkplan/SKILL.md`（新規）
- `.agents/skills/mkplan2/SKILL.md`（新規）
- `.agents/skills/runplan/SKILL.md`（新規）
- `.agents/skills/runpr/SKILL.md`（新規）
- `.agents/skills/fixpr/SKILL.md`（新規）
- `.agents/skills/mergepr/SKILL.md`（新規）
- `.agents/skills/weakreview/SKILL.md`（新規）
- `.agents/skills/japrose/SKILL.md`（新規）
- `.agents/skills/mktrans/SKILL.md`（新規）
- （実装時点で `main` に `chief.md` 以外の利用者起動コマンドが増えていれば同様に作成する。`chief` は作成しない）

**作業内容:**
- [ ] 設計書 3.2 に従い、英語の YAML フロントマター（`name: <cmd>`・空でない `description`）と 4 点の参照指示（`_context.md`・`_lib/harness.md` 経由のアダプタ・対応する `.claude/commands/<cmd>.md` 正本・`AGENTS.md`/`CLAUDE.md` 禁止規則）のみを持つ薄い入口を全 11 ファイル作成する（手順本文は一切複製しない）。

**完了基準:**
- `chief` を除くすべての開発コマンドに対応する `.agents/skills/<cmd>/SKILL.md` が存在し、`.agents/skills/chief/` が存在しない。

#### ステップ 4-6: `CommandDocs`・`docpaths_test.go` 拡張と 1 対 1 スキル構造ガードを追加する（F-002・F-008、AC-01・AC-09〜AC-11・AC-42）

**変更ファイル:**
- `internal/doctestutil/protocol.go`
- `internal/doctestutil/protocol_test.go`
- `cmd/yt2column/docpaths_test.go`
- `cmd/yt2column/skills_docs_test.go`（新規）

**作業内容:**
- [ ] `internal/doctestutil/protocol.go` の `CommandDocs` の走査対象に `.agents/skills` を追加し、`internal/doctestutil/protocol_test.go` に `TestCommandDocs` を追加して、`t.TempDir()` 上の `.claude/commands/*.md` と `.agents/skills/<cmd>/SKILL.md` の双方が返ること（`.agents/skills` 欠落・空時の失敗を含む）を検証する。
- [ ] `cmd/yt2column/docpaths_test.go` の `instructionDocs` に `.agents/skills` を追加し、返却一覧に `.agents/skills/` 配下のファイルが含まれることを明示的に検証する。あわせて、`TestInstructionDocPaths` が全スキルファイルの相対リンクと `docs/dev/` 参照の実在を検査することを確認する。
- [ ] `cmd/yt2column/skills_docs_test.go` を新設し、`TestSkillsMatchCommands` を実装する。同テストでは、`.claude/commands/*.md`（`_` 始まりと `chief.md` を除く）の集合と `.agents/skills/<cmd>/SKILL.md` の集合の完全一致、`.agents/skills/` 配下に余分なファイル・ディレクトリや `chief/` が存在しないこと、および各 `SKILL.md` の YAML フロントマター（`name == <cmd>`・空でない `description`）と対応する正本・`_context.md`・`_lib/harness.md` への参照構造を検証する。
- [ ] 実装時に、スキル 1 件削除・`chief` 追加・リンク破壊・禁止メンション混入のそれぞれで該当テスト（`TestSkillsMatchCommands`・`TestInstructionDocPaths`・`CommandDocs` 呼び出しテスト）が失敗することを確認する。

**完了基準:**
- `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/doctestutil` がすべて通る。

#### ステップ 4-7: `CLAUDE.md`・`AGENTS.md`・用語集・残り ADR を更新する（F-006・F-008、AC-34・AC-35・AC-41・AC-44）

**変更ファイル:**
- `CLAUDE.md`
- `AGENTS.md`
- `docs/translation_glossary.md`
- `docs/dev/adr/0014-deterministic-checks-in-go.md`
- `docs/dev/adr/0035-language-policy.md`
- `docs/dev/adr/0055-harness-independent-commands.md`

**作業内容:**
- [ ] `CLAUDE.md` の 4 項目（AC-41）を更新する: 「Document language」原則 (b) への `.agents/skills/` 追加、「Language」対話言語規則のハーネス非依存化、「Commands」節の一般化名称（上位モデルのレビュー・クロスチェックの部分レビュー等）と `.agents/skills/` 説明、および「Tool Execution Safety」の禁止規則が Antigravity にも適用される旨の明記（AC-34）。
- [ ] `AGENTS.md` に AC-34 の禁止規則 3 点（インストール型ツールのインストール・ビルド禁止、`herdr` 変更系と `git worktree` 追加・削除・移動禁止、記録タグ直接操作と未承認ネットワーク・マージ・実 API 呼び出し禁止）と `_lib/harness-antigravity.md` 参照を明記する（英語）。
- [ ] `docs/translation_glossary.md` に本タスクの一般化用語（ハーネス、ハーネスのアダプタ、上位モデルのレビュー、クロスチェックの部分レビュー、スクラッチパッド）を追加し、定義・英語表記を要件定義書 3.1・6 章と照合する。
- [ ] `ADR-0014`（`:56`）・`ADR-0035`（`:24`）・`ADR-0055`（`:25`）から残っている `#1072` の食い違い記述を除去する（編集上の訂正はコミットメッセージに `Editorial correction:` 行を残す）。

**完了基準:**
- `docs/dev/adr/` に `issues/1072` の食い違い記述が残らず、`TestADRRecords`・`TestInstructionDocPaths`・`TestReadmeSyncMarker` がすべて通る。

#### ステップ 4-8: 静的検証スクリプト作成・実行および最終差分記録（AC-02・AC-14・AC-32・AC-41・AC-43）

**変更ファイル:**
- `docs/tasks/0120_harness_independent_commands/_checks/check_harness_tools.sh`（新規）
- `docs/tasks/0120_harness_independent_commands/03_implementation_plan.md`（§7 の静的検証結果・差分記録）

**作業内容:**
- [ ] `docs/tasks/0120_harness_independent_commands/_checks/check_harness_tools.sh` を作成し、次の 2 点を検査する。実装時は各検査について違反（ツール呼び出し 1 件の一時挿入、仮の `.sh` ファイル 1 件の一時追加）で失敗することを確認し、実行結果と対象コミット ID を §7.1 に記録する。
    - (1) `.claude/commands/` 配下の非アダプタ・非 `chief` のコマンド定義（`_lib/harness*.md` と `chief.md` を除く）に AC-02 のハーネス固有ツール呼び出しが残っていないこと（`_context.md` の `### Command routing` の `Agent` 列ヘッダ、`### Harness model mapping` の `Launch` 列の `agent` / `swarm` 値、通常の英語動詞、` ```bash ` コードフェンス開始行による誤検知は除外する）。
    - (2) `.claude/commands/` 配下全体の `*.sh` を再帰走査し、ファイル集合が `_lib/adr-correction-candidates.sh`・`_lib/high-review-decision.sh`・`_lib/review-records.sh` の 3 本のみであること（AC-41 (4)）。
- [ ] §7.2 の PR-5 差分記録（`Diff record (AC-32)`）に `.claude/settings.json` の Path diff（空）と全変更ファイル一覧（`File list`）を記録する。
- [ ] `make test && make lint && make deadcode && make readme-sync-check` および CI `doc-guard` コマンド（`go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`）をすべて実行して成功を確認する。

**完了基準:**
- `_checks/check_harness_tools.sh` が終了コード `0` で通り、全テスト・静的検査・差分記録が完了する。

### PR-5 作成ポイント: Antigravity skills, doc guards, and final documentation

**対象ステップ**: 4-5 / 4-6 / 4-7 / 4-8

**推奨タイトル**: `feat(0120): Antigravity スキルと文書ガードを追加し関連文書を更新する`

**レビュー観点**: .agents/skills/ 11 ファイルの薄い入口構造と chief 非配置の 1 対 1 ガード / CommandDocs・docpaths_test.go 拡張によるスキル群の信頼境界検査適用 / CLAUDE.md・AGENTS.md・用語集・残 ADR 更新と _checks/check_harness_tools.sh 検証

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 4-6・4-8 が独立した高リスク検証領域（CommandDocs・docpaths_test.go の信頼境界テスト適用範囲の拡張、.agents/skills/ 1 対 1 構造ガード、および誤検知除外を含む _checks/check_harness_tools.sh）に該当し、Sonnet では余分なエントリやフロントマターの検査漏れ・除外規則の過大化によるツール呼び出し残存の見逃しを起こしやすいため。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 対象フェーズ | 主な成果物 | 完了条件 |
|---|---|---|---|
| M1: `fixprguard` 監視拡張 | フェーズ 1（ステップ 1-1〜1-3） | `.agents/`・4 指示ファイル監視、`errAgentsGitName`、`fixprguard.Protocol` 更新、ガイド・テスト更新 | `make test && make lint && make deadcode`・`doc-guard`・AC-32 差分記録が通る。マージ後に利用者が `make install-fixprguard` を実行 |
| M2: 役割名移行と `chief`・判定スクリプト一括改名 | フェーズ 2（ステップ 2-1〜2-4） | `_context.md`（`Harness model mapping`・6 旧値・新行キー）、`internal/chief`・`paneinput`・`chief.Protocol` 更新、`high-review-decision.sh` 一括改名、パリティ・フィクスチャテスト、関係 ADR 更新 | `make test && make lint && make deadcode`・`doc-guard`・パリティテスト・AC-32 差分記録が通る。マージ後に利用者が `/chief` 停止→`make install-chief`→再起動を実行 |
| M3: `review-records.sh` 一般化とアダプタ新設 | フェーズ 3（ステップ 3-1〜3-3） | `review-records.sh`（`cross-check`・`fable` 別名・`_context.md` 検証）、bash 3.2/4+ テスト、`harness.md`・`harness-claude.md`・`harness-antigravity.md`、Phase 6 隔離ガード | `make test && make lint && make deadcode`・bash 3.2/4+ テスト・`doc-guard`・AC-32 差分記録が通る |
| M4: 全コマンド非依存化・スキル新設・関連文書の最終更新 | フェーズ 4（ステップ 4-1〜4-8） | `_lib/high-review.md`・`_lib/cross-check-review.md`・全 11 コマンド非依存化・状態チェック/スクリプト/fixpr/chief テスト、`.agents/skills/` 11 ファイル、`CommandDocs`・`docpaths_test.go`・`skills_docs_test.go`、`CLAUDE.md`/`AGENTS.md`/用語集/残 ADR・`_checks/check_harness_tools.sh` | 全テスト・`make lint`・`make deadcode`・`make readme-sync-check`・`doc-guard`・`_checks/check_harness_tools.sh`・AC-32 差分記録が通り、全 44 AC の検証が完了する |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | `fixprguard` の監視の一覧・ネスト `.git` 検査拡張、プロトコル番号・ガイド・テスト更新、AC-32・AC-14 差分記録 | sonnet+opus-review | hold |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 | `_context.md`（モデル対応表・6 旧値・行キー）、`internal/chief`・プロトコル番号更新、`high-review-decision.sh` 一括改名とパリティ・フィクスチャテスト、関係 ADR 更新、差分記録 | opus | hold |
| PR-3 | 3-1 / 3-2 / 3-3 | `review-records.sh` 一般化（`cross-check`・`fable` 別名・bash 3.2 検証）、bash 3.2/4+ テスト、ハーネスのアダプタ 3 ファイル新設、Phase 6 隔離ガード、差分記録 | opus | hold |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 | 共有手順 8 本・`_context.md`・全 11 コマンド正本の改名とハーネス非依存化、状態チェック・スクリプト・`fixpr`・`chief` テストと関係 ADR 更新、差分記録 | opus | auto |
| PR-5 | 4-5 / 4-6 / 4-7 / 4-8 | `.agents/skills/` 11 ファイル新設、`CommandDocs`・`docpaths_test.go`・1 対 1 スキルガード、`CLAUDE.md`・`AGENTS.md`・用語集・残 ADR 更新、`_checks/check_harness_tools.sh` と最終差分記録 | sonnet+opus-review | auto |

---

## 4. テスト戦略 (Test Strategy)

設計書 7 章および `docs/dev/conventions/test_organization.md` に従う。新しい `testutil/` パッケージは作らず、既存のパッケージ内テストヘルパ（`internal/fixprguard/test_helpers.go`、`internal/chief/test_helpers.go`、`cmd/yt2column/test_helpers*.go`、いずれも `//go:build test`）を再利用する。

-   **Go 単体テスト（設計書 7.1）:**
    -   `internal/fixprguard/ignored_test.go`: `TestIsWatched`・`TestWatchListPatternsMatch`（33 件）・`TestCollectTargetsWatchedDirGitNames`（`.claude/`・`.agents/` 配下の `.git` / `.GIT` 境界による `errClaudeGitName` / `errAgentsGitName` および読取不能サブディレクトリによる `errReadIgnoredDir`）、`.agents/` と任意階層 4 指示ファイル（Unicode 単純ケースフォールディング含む）の `PartIgnored` 検出、合算 `walkEntries` 上限。
    -   `internal/chief/{task_test.go,routing_test.go,send_test.go,snapshot_test.go,maintask_test.go,running_test.go,autosend_test.go}`・`internal/chief/paneinput/paneinput_test.go`: 実 `_context.md` に対する `TestParseOldValues` の 6 件一致検証、`TestReadModelRequirement`・`TestParsePlanProgress`・`TestParsePlanRejects`（新 3 値・全 6 旧値・異常系 `RequirementUnreadable` / `errTaskDocument`、`testdata/plans/` 標本使用）、新 3 行キー・未知エージェント拒否（`errRouting`）・`### Harness model mapping` 非干渉。
-   **パリティテスト（設計書 7.2）:**
    -   `internal/chief/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript`: `internal/chief` と `_lib/high-review-decision.sh` の分類結果（`RUN`・`NONE value-low`・`NONE value-absent`）を全フィクスチャで照合する（`testTools` に `"env"` を追加）。
-   **シェルスクリプトフィクスチャテスト（設計書 7.3）:**
    -   `cmd/yt2column/high_review_decision_test.go`（`cmd/yt2column/test_helpers_bash.go::requireBash4` と `cmd/yt2column/testdata/review_records/` 標本を使用、Go ソースに日本語リテラルを書かない）: `high-review-decision.sh` の自己検証可能ハーネス（`Launch == agent`）による `NONE all-high`（除外可能コミット・非 `high` モデルの記録範囲に含まれるコミット・マージコミットを含む）、自己検証不能ハーネス（`Launch == swarm`）の `RUN`、新旧 `done` 記録での `SKIP <done-line>`、`_context.md` 不正時に早期返却より前に期待する診断を出して `RUN` を返すこと、環境隔離・不正入力、呼び出し箇所の `origin/main` 取得契約（`--no-replace-objects`）。
    -   `cmd/yt2column/review_records_test.go`: `review-records.sh` の `cross-check` モード（全マトリクス）・互換別名 `fable` モード（代表ケースのスモークテスト）・`coverage` モード（`_context.md` 非依存）を bash 3.2（`/bin/bash`）と bash 4+ の双方で実行検証する。
    -   `cmd/yt2column/cross_check_state_docs_test.go`: `_lib/cross-check-review.md` の `## The state check` ブロックを実行し、既存項目に加えて git 無視された `.agents/` 配下・全 4 指示ファイル（`AGENTS.md`・`AGENT.md`・`GEMINI.md`・`AGENTS.override.md`、`status` 不変の確認付き）・`.claude/` および `.agents/` 配下 `.git` エントリの検出を検証する。
-   **文書・構造ガードテスト（設計書 7.4）:**
    -   `internal/doctestutil/protocol_test.go::TestCommandDocs`（`.agents/skills` 返却の直接検証）および `CommandDocs` 経由の全 8 呼び出しテスト（5 パッケージ、新設の `cmd/yt2column/high_review_decision_test.go::TestHighReviewDecisionCallSite` を含む）。
    -   `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（`.agents/skills` 含有検証とリンク実在）。
    -   `cmd/yt2column/skills_docs_test.go::TestSkillsMatchCommands`（1 対 1 対応・余分なエントリや `chief/` の不在・YAML フロントマター・参照構造）。
    -   `cmd/yt2column/fixpr_docs_test.go`（`TestFixprPhase6CountFilter`・`TestHarnessAntigravityPhase6Redirect`）。
    -   `cmd/yt2column/review_records_test.go`（`TestReviewRecordsCallSite`・`TestRecordLinePrefixesDistinct` の新旧全接頭辞ペア検査・`TestRecordCommitsCheckMention`）。
    -   `cmd/fixprguard/docs_test.go`、`cmd/chief/docs_test.go`（`TestMarkerFormsCarryModelRequirement` 追加）、`cmd/yt2column/adr_test.go::TestADRRecords`。
-   **セキュリティテスト（設計書 7.5）:**
    -   `_context.md` パターン不正入力・非正規表現照合（`.` リテラル一致・48 文字受理 vs 49 文字拒否・パターン長超過・メタ文字拒否・`Launch != none` での `-` 拒否・表破損時の Fail-Closed）、自己検証不能ハーネスでの `all-high` 拒否、`.agents/` ネスト `.git` 隠蔽拒否。
-   **後方互換性テスト:**
    -   6 旧値の読み替え（`internal/chief/task_test.go`・`markers_parity_test.go`・`high_review_decision_test.go`）、旧記録行（`Opus review:`・`Fable ...:`・`not-fable`）と旧 `fable` モード別名の受理（`high_review_decision_test.go`・`review_records_test.go`）。

---

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 緩和策 |
|---|---|---|
| 前提タスク `#1059`（タスク 0118）未完了のまま着手・マージ | `_context.md`・`runpr.md`・`codex_review_docs_test.go` 等で大規模な競合が発生する | フェーズ 1 の開始を `#1059` 完了後とし、`**自動送信**` を `hold` に設定する（1.2） |
| `fixprguard`・`chief` のプロトコル番号引き上げに伴う再インストール漏れ | 古いインストール済みバイナリが `--protocol` 不一致で停止する | 各フェーズ完了後の手動作業を明記し、後続 PR の `**自動送信**` を `hold` にする。`CLAUDE.md`「Protocol numbers」に従いマージ前に `origin/main` の値 + 1 を再確認する |
| `opus-review-decision.sh` → `high-review-decision.sh` 一括改名の未マージブランチへの影響 | 改名 PR マージ直後に `main` 未取り込みの OpenCode / `low` ブランチが安全側 `RUN` で停止する | 設計書 8 章の復旧手順（`git merge origin/main`）を PR 説明に明記し、レビュー素通りが起きない Fail-Closed 特性を保つ |
| `review-records.sh` への bash 4 固有構文の混入 | macOS の `/bin/bash`（3.2）で失敗する | `scriptBashes` による `/bin/bash` 3.2 サブテストの通過をフェーズ 3・4 の完了条件に含める |
| 関連タスク `#988`（タスク 0114）とのマージ順序の前後 | `review-records.sh` や `_lib/cross-check-review.md` の入口構成に差分が出る | フェーズ 3・4 着手時に `origin/main` の状態を確認し、`#988` が先に入っていれば設計書 3.5 の条件付き整合（マージ後確認の `high` 役割化と `high` パターン検証）を適用する |
| `CommandDocs` の既存信頼境界ガードとの意図しない衝突（フェーズ 3 の `_lib/harness*.md` およびフェーズ 4 の `.agents/skills/`） | アダプタやスキル内のコマンド例・語句が `TestNoDirectHerdrInvocation`・`cmd/numreserve` `TestDocumentedInvocations`・`TestPostingCommandsInventory` 等に抵触する | ステップ 3-3 で生コマンド例を避け既存ガイド参照で記述し、ステップ 3-3・4-6 それぞれのグリーンゲートで全 `doc-guard` パッケージを確認する |

---

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
-   [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4）
-   [x] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4）
-   [ ] PR-5 マージ済み（対象ステップ: 4-5 / 4-6 / 4-7 / 4-8）

---

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

すべての受け入れ基準（AC-01〜AC-44）と検証手段の対応を以下に示す。各 `test`・`static` 検証では、実装時に対応する実装箇所を意図的に壊して失敗することを確認し、コミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。

| AC | 種別 | 検証場所 / 成果物 | 対象ステップ | 検証内容 |
|---|---|---|---|---|
| AC-01 | `test` / `manual` | `cmd/yt2column/skills_docs_test.go::TestSkillsMatchCommands`、PR レビュー | 4-1, 4-2, 4-5, 4-6 | 非 `chief` 開発コマンド集合とスキル集合の 1 対 1 一致、および手順正本が `.claude/commands/` に 1 か所のみ存在することの確認 |
| AC-02 | `static` / `manual` | `docs/tasks/0120_harness_independent_commands/_checks/check_harness_tools.sh`、PR レビュー | 4-1, 4-2, 4-8 | 非アダプタ・非 `chief` コマンド定義（`_context.md` 含む）にハーネス固有ツール呼び出しが存在しないことの確認と、抽象操作による記述の確認 |
| AC-03 | `test` / `manual` | `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`、PR レビュー | 3-3, 4-1, 4-2 | アダプタ 3 ファイルへのリンク実在検証、および決定的判別規則・OpenCode フォールバック記述の確認 |
| AC-04 | `manual` | PR レビュー（`_lib/harness*.md`・コマンド正本） | 3-3, 4-1, 4-2 | 選択式の質問（最大 4 問・2〜4 択・先頭推奨・選択肢外聞き直し）の自然言語手順確認 |
| AC-05 | `manual` | PR レビュー（`_context.md`・`_lib/harness*.md`・コマンド正本） | 3-3, 4-1, 4-2 | スクラッチパッド（作業ツリー外、Antigravity `<appDataDir>/brain/<conversation-id>/scratch/`）の手順確認 |
| AC-06 | `test` / `manual` | `cmd/yt2column/fixpr_docs_test.go::TestFixprPhase6CountFilter`・`TestFixprguardCallSites`・`TestRemovedCommandLiterals`・`TestHarnessAntigravityPhase6Redirect`、PR レビュー | 3-3, 4-1〜4-3 | `fixpr` Phase 6 の役割表記・件数フィルタ・Antigravity 直接リダイレクト隔離の検証、および他コマンドの段階指定手順確認 |
| AC-07 | `manual` | PR レビュー（`_lib/pr-review-escalation.md`・`_lib/harness*.md`） | 3-3, 4-1 | エスカレーション判定の `high` 役割固定ワーカー化・固定 3 メンバ JSON、起動や割り当てやモデルや JSON やタイムアウトの失敗時は `ask`、状態チェック差分・不正ヘルパー検出時は Fail-Closed 停止となる手順の確認 |
| AC-08 | `manual` | PR レビュー（`_context.md`・`_lib/harness*.md`・コマンド正本） | 3-3, 4-1 | ハーネス別の待機・タイマー（Antigravity は `schedule`、シェル `sleep` 禁止）・通知代替手順の確認 |
| AC-09 | `test` / `manual` | `cmd/yt2column/skills_docs_test.go::TestSkillsMatchCommands`、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`、PR レビュー | 4-5, 4-6 | `.agents/skills/<cmd>/SKILL.md` の 1 対 1 存在・YAML フロントマター・参照リンク実在・英語薄い入口記述の確認 |
| AC-10 | `test` / `manual` | `cmd/yt2column/skills_docs_test.go::TestSkillsMatchCommands`、PR レビュー | 4-5, 4-6 | 各 `SKILL.md` が正本・`_context.md`・アダプタへの参照のみを持ち手順本文を複製しないことの確認 |
| AC-11 | `test` | `cmd/yt2column/skills_docs_test.go::TestSkillsMatchCommands` | 4-5, 4-6 | `.agents/skills/chief/` および `.agents/skills/chief/SKILL.md` が存在しないことの検証 |
| AC-12 | `test` / `manual` | `internal/chief/task_test.go::TestReadModelRequirement`・`TestParsePlanProgress`、`internal/chief/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript`、`cmd/chief/docs_test.go::TestMarkerFormsCarryModelRequirement`、PR レビュー | 2-1, 2-2, 4-2, 4-3 | 新 3 値（`high`・`standard+high-review`・`low`）の読み取りと計画作成コマンドの書式確認 |
| AC-13 | `manual` | PR レビュー（`mkplan2.md`・`mkplan.md`・`mkreq.md`） | 4-2 | `**判定理由**` の書き方からモデル固有名必須記述が外れ、役割基準で書かれていることの確認 |
| AC-14 | `test` / `diff` | `internal/chief/task_test.go::TestParseOldValues`・`TestReadModelRequirement`、`internal/chief/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript`、`cmd/yt2column/high_review_decision_test.go::TestHighReviewDecision`、§7.2 各 PR の `Diff record (AC-32)` 内 `File list` | 1-3, 2-1〜2-4, 3-3, 4-4, 4-8 | 6 旧値すべての読み替え検証、および他タスクの計画書が全実装 PR の変更ファイル一覧に含まれないことの差分記録確認 |
| AC-15 | `test` | `internal/chief/task_test.go::TestReadModelRequirement`・`TestParsePlanRejects`、`internal/chief/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript`、`cmd/yt2column/high_review_decision_test.go::TestHighReviewDecision` | 2-1〜2-3 | 未知値→`RequirementUnreadable`/`RUN`、値なし→`RequirementAbsent`/`NONE value-absent` の検証 |
| AC-16 | `test` / `manual` | `cmd/yt2column/high_review_decision_test.go::TestHighReviewDecision`、`cmd/yt2column/review_records_test.go::TestReviewRecordsCrossCheck`、`internal/chief/routing_test.go`、`internal/chief/task_test.go::TestParseOldValues`、PR レビュー（`_context.md`） | 2-1〜2-3, 3-1, 3-2 | 実 `_context.md` の `### Harness model mapping` 表の厳密文法・`high` / `cross-check` のモデルの ID（`fable`・`claude-opus-5-5-medium`）・既存パーサ非干渉の自動検証と、全 9 行のモデル／起動手段の目視確認 |
| AC-17 | `manual` | PR レビュー（`runplan.md`・`runpr.md`・`_context.md` part 5） | 2-1, 4-2 | 新旧値表示・ハーネス別推奨モデル・上位モデルのレビュー見込み案内の手順確認 |
| AC-18 | `test` | `cmd/yt2column/high_review_decision_test.go::TestHighReviewDecision`・`TestHighReviewDecisionCallSite`、`internal/chief/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript` | 2-3, 4-1, 4-3 | `origin/main` からの `--no-replace-objects` 取り出し実行、`_context.md` 早期返却前検証と動的読取、モデル固有名除去、環境隔離、エラー時 `RUN`＋標準エラー診断＋終了コード `0` の検証 |
| AC-19 | `test` / `manual` | `cmd/yt2column/high_review_decision_test.go::TestHighReviewDecision`、PR レビュー（`runplan.md`） | 2-3, 4-2 | 自己検証可能ハーネス（`Launch == agent`、`claude-opus-*`）の `Implementation model:` 記録による `NONE all-high`（除外可能コミット・非 `high` モデルの記録範囲に含まれるコミット・マージコミットを含む）検証と記録手順確認 |
| AC-20 | `test` / `manual` | `cmd/yt2column/high_review_decision_test.go::TestHighReviewDecision`、PR レビュー（`runplan.md`・`_lib/harness-antigravity.md`） | 2-3, 3-3, 4-2 | Antigravity で `Implementation model:` 非記録、`Launch == swarm` パターン（`gemini-4-neon-high`）でも `NONE all-high` にならず `RUN` となることの検証 |
| AC-21 | `test` / `manual` | `cmd/yt2column/cross_check_state_docs_test.go::TestCrossCheckStateCheckBlock`、PR レビュー（`_lib/high-review.md`・`_lib/harness*.md`） | 3-3, 4-1, 4-3 | 共通状態チェックの実行検証、および Claude Code / Antigravity 双方の起動・モデル検証・待機・停止手順の確認 |
| AC-22 | `manual` | PR レビュー（`_lib/high-review.md`・`_lib/harness*.md`・`runplan.md`・`weakreview.md`） | 3-3, 4-1, 4-2 | 5 つの失敗条件すべてで別モデル代替せず Fail-Closed 停止（PR 作成・push なし）となる手順確認 |
| AC-23 | `test` / `manual` | `cmd/yt2column/high_review_decision_test.go::TestHighReviewDecision`、`cmd/yt2column/review_records_test.go::TestRecordLinePrefixesDistinct`、PR レビュー（`_lib/task-pr-handoff.md`・`_lib/cross-check-review.md`） | 2-3, 4-1, 4-3 | `High review: done` / 旧 `Opus review: done` の両読み・厳密文法・接頭辞非衝突の検証、および PR 説明・決着済み論点での `High review: kept` / 旧 `Opus review: kept` 両読み手順確認 |
| AC-24 | `test` / `manual` | `cmd/yt2column/review_records_test.go::TestRecordCommitsCheckMention`、PR レビュー（`_lib/high-review.md`・`weakreview.md`） | 4-1〜4-3 | コミット前のメンション確認検証、および出力非転記・利用者確認・`/weakreview` 引数仕様維持の確認 |
| AC-25 | `test` / `manual` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCallSite`・`TestRecordLinePrefixesDistinct`・`TestRecordCommitsCheckMention`、`cmd/yt2column/cross_check_state_docs_test.go::TestCrossCheckStateCheckBlock`、PR レビュー | 4-1〜4-3 | `_lib/cross-check-review.md`・`CROSS_CHECK`・状態チェック・呼び出し箇所・メンション確認の検証と共通手順確認 |
| AC-26 | `manual` | PR レビュー（`_lib/harness-claude.md`） | 3-3 | Claude Code アダプタの `Agent` 起動・`Model:` 1 行目検証・30 分タイマー・`TaskStop` 手順確認 |
| AC-27 | `manual` | PR レビュー（`_lib/harness-antigravity.md`・`_lib/cross-check-review.md`） | 3-3, 4-1 | Antigravity アダプタの `invoke_subagent` 禁止・`models` 事前確認・`swarm add` と JSON `"model"` 検証・`schedule` 30 分・`read_board`・`kill`・1 回再試行・`tokens unknown` の手順確認 |
| AC-28 | `test` / `manual` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCrossCheck`、PR レビュー（`_lib/harness.md`・`_lib/cross-check-review.md`） | 3-1〜3-3, 4-1 | `agent-cannot-call` 記録の受理検証、OpenCode フォールバックおよび `#988` 条件付き整合の確認 |
| AC-29 | `test` / `manual` | `cmd/yt2column/review_records_test.go::TestRecordLinePrefixesDistinct`・`TestReviewRecordsCrossCheck`、PR レビュー（`_lib/cross-check-review.md`・`_lib/task-pr-handoff.md`・`runpr.md`） | 3-1, 3-2, 4-1〜4-3 | 新記録行接頭辞・`wrong-model` の検証、PR 説明見出し・`/runpr` 要約・`.agents/` の `<kind>` 分類の確認 |
| AC-30 | `test` / `manual` | `cmd/yt2column/review_records_test.go::TestReviewRecordsCrossCheck`・`TestReviewRecordsFableAlias`・`TestReviewRecordsCoverage`・`TestReviewRecordsMode`・`TestReviewRecordsCallSite`、PR レビュー（`_lib/task-pr-handoff.md`・`runpr.md`） | 3-1, 3-2, 4-1〜4-3 | `cross-check` モード・`fable` 別名・`_context.md` パターン非正規表現検証・新旧両読み・失敗時 `MISSING`（終了コード `1`）・`coverage` 不変の bash 3.2/4+ 検証、および PR 説明・`/runpr` での新旧 `scope:`/`finding:` 両読み手順確認 |
| AC-31 | `test` / `manual` | `cmd/yt2column/review_records_test.go::TestRecordCommitsCheckMention`、PR レビュー（`_lib/cross-check-review.md`・`_lib/task-pr-handoff.md`） | 4-1, 4-3 | メンション確認検証、および 30 件上限・形式不一致→`unreadable`・範囲外→`invalid`・出力非転記の手順確認 |
| AC-32 | `diff` | §7.2 の各実装 PR 差分記録（`Diff record (AC-32)`、Compared paths: `.claude/settings.json`、`File list` 付き） | 1-3, 2-4, 3-3, 4-4, 4-8 | すべての実装 PR で `.claude/settings.json` が変更されていないことを機械的に再確認 |
| AC-33 | `manual` | PR レビュー（`_lib/harness-antigravity.md`） | 3-3 | Antigravity サンドボックス実行規則（標準優先・単一コマンド分離昇格・`toolSummary` 一致・PR タイトルの安全な渡し方の優先）の記載確認 |
| AC-34 | `manual` | PR レビュー（`AGENTS.md`・`CLAUDE.md`・`_lib/harness-antigravity.md`） | 3-3, 4-7 | 禁止規則 3 点が Antigravity セッションにも適用される旨の 3 文書記載確認 |
| AC-35 | `test` / `manual` | `cmd/yt2column/adr_test.go::TestADRRecords`、PR レビュー（`ADR-0055`・`ADR-0014`） | 4-7 | ADR 構造整合検証、および機械的 `deny` 不在と残余の危険の記載確認 |
| AC-36 | `test` | `internal/fixprguard/ignored_test.go`、`cmd/fixprguard/docs_test.go::TestGuideListsWatchList`・`TestGuideDescribesIgnoredFiles`・`TestDocumentedProtocolMatches`、`cmd/yt2column/cross_check_state_docs_test.go::TestCrossCheckStateCheckBlock` | 1-1, 1-2, 4-1, 4-3 | `fixprguard` と状態チェックの `.agents/`・全 4 指示ファイル・`.claude/`/`.agents/` ネスト `.git` 監視、および `fixprguard` プロトコル番号・ガイド・呼び出し箇所の同時整合検証 |
| AC-37 | `test` | `internal/chief/task_test.go::TestParseOldValues`・`TestReadModelRequirement`・`TestParsePlanProgress`・`TestParsePlanRejects` | 2-2 | `internal/chief` の新 3 値・6 旧値・異常系（`RequirementUnreadable` / `errTaskDocument`）検証 |
| AC-38 | `test` | `internal/chief/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript` | 2-3 | `internal/chief` と `high-review-decision.sh` の全分類パリティ一致検証 |
| AC-39 | `test` | `internal/chief/routing_test.go`、`internal/chief/paneinput/paneinput_test.go`、`internal/chief/snapshot_test.go`、`internal/chief/send_test.go`、`internal/chief/task_test.go::TestParseOldValues` | 2-1, 2-2 | 改名後 3 行キー・2 エージェント維持・異常系 `errRouting`・`### Harness model mapping` 非干渉の検証 |
| AC-40 | `test` | `cmd/chief/docs_test.go::TestDocumentedProtocolMatches`・`TestGuideProtocolMatchesTool`・`TestArchitectureValuesMatchTool` | 2-2 | `internal/chief` プロトコル番号と `chief.md`・`chief_guide.md`・`chief_architecture.md` の同時整合検証 |
| AC-41 | `static` / `manual` | `docs/tasks/0120_harness_independent_commands/_checks/check_harness_tools.sh`、PR レビュー（`CLAUDE.md`・`AGENTS.md`・`docs/dev/`・`docs/translation_glossary.md`） | 2-3, 4-7, 4-8 | `.claude/commands/` 配下全体の `*.sh` 集合に第 3 の判定スクリプトが増えていないことの検証、および一般化名称・用語集照合・`CLAUDE.md` 4 項目更新の確認 |
| AC-42 | `test` | `internal/doctestutil/protocol_test.go::TestCommandDocs`、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`、`CommandDocs` 経由の全 8 テスト（`TestHighReviewDecisionCallSite` 含む）、`cmd/yt2column/skills_docs_test.go::TestSkillsMatchCommands` | 4-6 | `.agents/skills/` 全ファイルの (1) パス解決、(2) 信頼境界検査、(3) 1 対 1 対応と `chief` 不在の検証 |
| AC-43 | `test` / `static` | `make test`・`make lint`・`make deadcode`・`make readme-sync-check`・CI `doc-guard` コマンド | 1-2, 2-3, 3-3, 4-3, 4-8 | 全単体テスト・文書ガード・静的検査・README 同期確認の成功 |
| AC-44 | `test` / `manual` | `cmd/yt2column/adr_test.go::TestADRRecords`、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`、PR レビュー（関連 8 ADR・ADR-0016・索引） | 1-2, 2-3, 3-3, 4-3, 4-7 | ADR 構造・索引一致・リンク実在検証、および各 PR での段階的書き換えと全 `#1072` 食い違い記述の解消確認 |

### 7.1 静的検証結果（`_checks/check_harness_tools.sh`）

実装時（ステップ 4-8）にスクリプトの実行結果と対象コミット ID をここへ記録する。

### 7.2 差分の記録（AC-32・AC-14）

各実装 PR で `/runplan` が `docs/dev/conventions/requirements_process.md` §4「`diff` verification」の固定コマンドにより、`Path diff`（Compared paths: `.claude/settings.json`）と `File list`（全変更ファイル一覧。本計画書自身は Compared paths に含めない）の両方を記録する。各 PR の `File list` は AC-32（`.claude/settings.json` 非変更）と AC-14（他タスクの `03_implementation_plan.md` 非変更）の双方を検証する。

PR-1:

Diff record (AC-32):

- Compared paths: `.claude/settings.json`
- Merge-base: `d7467569603ebee9da629419ae299d430fb920f2`
- Compared commit: `6dee92cb368e44d3e140203296c888b4e502b1de`

Path diff:

````text
````

File list:

````text
.claude/commands/fixpr.md
.claude/commands/runpr.md
docs/dev/adr/0055-harness-independent-commands.md
docs/dev/guides/fixprguard_guide.md
docs/tasks/0120_harness_independent_commands/03_implementation_plan.md
internal/fixprguard/ignored.go
internal/fixprguard/ignored_test.go
internal/fixprguard/types.go
````

PR-2:

Diff record (AC-32):

- Compared paths: `.claude/settings.json`
- Merge-base: `d74acaeb62c0aeebc1a85a2593324aac97aa3029`
- Compared commit: `9c438744fe629f4a17d8b5a290b6bdc3fbcc48ff`

Path diff:

````text
````

File list:

````text
.claude/commands/_context.md
.claude/commands/_lib/high-review-decision.sh
.claude/commands/_lib/opus-review-decision.sh
.claude/commands/_lib/opus-review.md
.claude/commands/chief.md
.claude/commands/runplan.md
CLAUDE.md
cmd/yt2column/high_review_decision_test.go
cmd/yt2column/review_records_test.go
cmd/yt2column/test_helpers_bash.go
docs/dev/adr/0014-deterministic-checks-in-go.md
docs/dev/adr/0035-language-policy.md
docs/dev/adr/0042-implementation-model-tiers.md
docs/dev/adr/0046-chief-of-staff.md
docs/dev/guides/chief_guide.md
docs/tasks/0120_harness_independent_commands/03_implementation_plan.md
internal/chief/autosend_test.go
internal/chief/maintask_test.go
internal/chief/markers_parity_test.go
internal/chief/paneinput/paneinput.go
internal/chief/paneinput/paneinput_test.go
internal/chief/records.go
internal/chief/routing.go
internal/chief/routing_test.go
internal/chief/running_test.go
internal/chief/send_test.go
internal/chief/skiprecord.go
internal/chief/snapshot_test.go
internal/chief/state.go
internal/chief/task.go
internal/chief/task_test.go
internal/chief/test_helpers.go
internal/chief/testdata/README.md
internal/chief/testdata/plans/marker-high.md
internal/chief/testdata/plans/marker-low.md
internal/chief/testdata/plans/marker-old-frontier-recommended.md
internal/chief/testdata/plans/marker-standard-review.md
internal/chief/testdata/plans/single-low.md
````

PR-3:

Diff record (AC-32):

- Compared paths: `.claude/settings.json`
- Merge-base: `4aee4aa65011cb421b630e489dbb3779372ee900`
- Compared commit: `ed7dac285c4fdcb3a4e693e8d970584e3394baf3`

Path diff:

````text
````

File list:

````text
.claude/commands/_context.md
.claude/commands/_lib/harness-antigravity.md
.claude/commands/_lib/harness-claude.md
.claude/commands/_lib/harness.md
.claude/commands/_lib/review-records.sh
cmd/yt2column/fixpr_docs_test.go
cmd/yt2column/review_records_test.go
docs/dev/adr/0014-deterministic-checks-in-go.md
docs/dev/adr/0050-fable-partial-review.md
docs/dev/adr/0055-harness-independent-commands.md
docs/tasks/0120_harness_independent_commands/03_implementation_plan.md
````

PR-4:

Diff record (AC-32):

- Compared paths: `.claude/settings.json`
- File list: required (all changed paths of the PR)

PR-5:

Diff record (AC-32):

- Compared paths: `.claude/settings.json`
- File list: required (all changed paths of the PR)

---

## 8. 完了基準 (Success Criteria)

-   **機能完了:** AC-01〜AC-44 すべてに対応する実装と検証が完了している。
-   **品質・静的検査:** `make test`・`make lint`・`make deadcode`・`make readme-sync-check`、および CI `doc-guard` コマンド（`go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`）がすべて通る。macOS 手元環境で `review-records.sh` の `/bin/bash`（3.2）および bash 4+ サブテストがすべて通る。
-   **セキュリティ検証:** 設計書 7.5 の全セキュリティテスト（`_context.md` パターン非正規表現照合・不正入力 Fail-Closed、自己検証不能ハーネスの `all-high` 拒否、`.agents/` ネスト `.git` 拒否、`CommandDocs` 経由の `.agents/skills/` 信頼境界検査、`fixpr` Phase 6 直接リダイレクト隔離）が通り、各実装 PR で `.claude/settings.json` が未変更である（AC-32）。
-   **文書完結:** `CLAUDE.md`・`AGENTS.md`・`docs/dev/guides/{fixprguard_guide,chief_guide}.md`・`docs/dev/chief_architecture.md`・`docs/translation_glossary.md`・関連 ADR（`0013`・`0014`・`0016`・`0035`・`0042`・`0046`・`0047`・`0050`・`0055`・索引）がすべて整合し、`#1072` の食い違い記述が残っていない。

---

## 9. 次のステップ (Next Steps)

1.  本計画書（`03_implementation_plan.md`）の PR レビューを受け、`/runpr` で承認・マージする。
2.  前提タスク `#1059`（タスク 0118：Codex 撤去）の実装 PR がすべて `main` にマージされたことを確認する。
3.  `/runplan 0120` でフェーズ 1（PR-1）から順に実装・PR 作成・マージを進める（PR-1 マージ後に `make install-fixprguard`、PR-2 マージ後に `/chief` 停止→`make install-chief`→再起動を手動で行う）。
4.  本タスク完了後、後続タスク `#1057`（計画書・実装 PR の部分レビュー置き換え）および `#1082`（Antigravity からのローカル `claude -p --model fable` 呼び出し設計）へ進む。

## 変更履歴

- 2026-11-23 承認（isseis、#1085）
