# 実装計画書：Codex の PR レビューと Codex CLI をレビューの体制から外す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-20 |
| Review date | 2026-11-21 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

**目的:** [01_requirements.md](01_requirements.md)（以下、要件定義書）の F-002〜F-012 と、[02_architecture.md](02_architecture.md)（以下、設計書）の設計に従い、Codex の PR レビューと Codex CLI への依存をリポジトリから外し、手動の CodeRabbit PR レビューと CodeRabbit CLI によるローカルレビューへ移行する。F-001（AC-01〜AC-05）は [observations.md](observations.md) と設計書の PR（#1070）で完了済みである。

**実装方針:**

-   設計書 §6.3・§8 の優先順位どおり、フェーズ 1（ローカルの CLI レビュー・PR 前のレビュー・`codexreview` 撤去）→ フェーズ 2（`fixprguard` 拡張・PR レビュー手動化・メンション拒否・自動レビュアーの一覧の更新）→ フェーズ 3（残りの文書・ADR・用語集・chief 確認・フォローアップ issue）の 3 段階で進める。
-   移行の順序（要件定義書 4.4、AC-45）を守る。フェーズ 2（PR-3）のマージ直後に利用者が Codex の GitHub 連携を切り契約を終えるため、フェーズ 3（PR-4）はその後に進める。
-   `fixprguard` のプロトコル番号は `main` の値 + 1 へ上げ、文書とテストの `--protocol` をすべて同じ変更でそろえる（設計書 3.3.5）。
-   テストの削除は設計書 7.4 に従う。`cmd/codexreview` と `internal/codexreview` は製品コードとテストを 1 つの commit で消し、残るパッケージのテスト削除は削除だけの commit に分けて `make coverage-compare BASE=<親>` で確認する。`doctestutil.RunsCommand` のテストカバレッジを維持するため、`cmd/codexreview` 削除より前にコマンド形式テストを `internal/doctestutil/commands_test.go` へ移す。
-   Go のソースコード（`*_test.go`・`test_helpers.go` を含む）のコメント・識別子・文字列リテラルは英語で書く。
-   設計の詳細は設計書の各節を参照し、本書には重複して書かない。

### 1.1 既存コード調査結果

`main` の `dc224b5c` を読んだ結果である。`implementation_handoff.md` は存在しない。`design_handoff.md`（H-01〜H-09）はすべて設計書 付録 A に記録されている。

-   **`_lib/task-pr-handoff.md` Part B（`dc224b5c`）:** タスク 0114 の step 2.5（Entry 4）はまだ `main` に入っていない。Part B は step 1〜7 で構成され、step 2（CHECKS、`:207`）の直後に step 3（push と PR 作成、`:209`）がある。`cmd/yt2column/codex_review_docs_test.go:76` は `3. Push and create the PR for the current branch.` から `\n4. Run AFTER_CREATE` の範囲を切り出している。また `:67-70`（Authorization）と `:185`（step 1）に `codexreview` / Codex CLI の記述がある。
-   **`codexreview` と関連するガード・設定（`dc224b5c`）:**
    -   `cmd/codexreview/`、`internal/codexreview/`（`testutil/` に `helpers.go`・`helpers_test.go` の 2 ファイル）。
    -   `internal/pipeline/pipeline_test.go:578`: `TestFakesCarryBuildTag` が `len(testutilFiles) != 24` を固定値で検証している。`internal/codexreview/testutil/` の 2 ファイルを消すと 22 になるため、同じ変更で更新する必要がある。
    -   `internal/pipeline/pipeline_test.go:630-650`（`TestPackageReferenceListsPackages`）・`internal/pipeline/strictjson_users_test.go:204`: `docs/dev/conventions/package_reference.md` のパッケージ一覧と `internal/strictjson` の利用元を検証している。`package_reference.md:109,117,124,135` の `codexreview` 関連の記述は `codexreview` 撤去と同じ変更で外す必要がある。
    -   `internal/doctestutil/protocol.go:19,24,51,98`（`ToolCodexreview`・`protocolTools`・コメント）、`internal/doctestutil/protocol_test.go:11,19-87`（`TestProtocolFlags` 15 ケース中 12 ケースが `codexreview` を例題ツールに使用）、`internal/doctestutil/commands.go:118-119,200`（コメントの `codex` 例。`RunsCommand` 自体は `cmd/chief/docs_test.go:305` が `herdr` で使うため残す）、`internal/doctestutil/commands_test.go:9,17-18`（`cmd/codexreview` の `TestRunsCodex` にコマンド形式テストを委ねているコメントと `codex` ケース）。
    -   `internal/mergepr/protocolcheck_test.go:141`: `TestMismatchRemedy` が `[]string{"fixprguard", "codexreview", "chief", "mergepr"}` を持つ。
    -   `cmd/fixprguard/docs_test.go:345,393,403-404`: `TestFixprguardProtocolMismatches` とコメントに `codexreview` 参照がある。
    -   `internal/config/envaccess_test.go:76-79`、`internal/cmdrunner/runner.go:3`、`cmd/coveragecompare/main.go:27,64` に `codexreview` を名指す行・コメントがある。
    -   `Makefile:53,73-77`（`install-codexreview`）、`.claude/settings.json:5`（`Bash(codexreview *)`）、`.github/workflows/ci.yml:215`（`doc-guard` の `./cmd/codexreview`）、`CLAUDE.md:189,461-473,766,809,816,849,1085-1113`。
    -   `.claude/commands/fixpr.md:116-118,1271`（Authorization の `codexreview`、Phase 7 の Codex CLI 回数）と `.claude/commands/runplan.md:256,258,261`（step 8 の Codex CLI フォールバック記述）。
-   **`fixprguard` の再利用対象と変更対象（`dc224b5c`）:**
    -   再利用: `internal/fixprguard/github.go` の `runGH`・`resolveRepository`・`graphql`・`strictjson` 呼び出し、`internal/fixprguard/limits.go:281` の `parseDateHeader` と `splitIncluded`・`InteractionState` の `MarshalText` パターン。
    -   プロトコル番号: `internal/fixprguard/types.go:21` に `Protocol` 定数がある。
    -   メンション拒否: `internal/fixprguard/errors.go:56-58,72`（`errCodexMention`・`CodexMention`）、`internal/fixprguard/write.go:76-95,161,339,377,390,404`（`checkPostText` と呼び出し元）、`internal/fixprguard/worktree.go:331`、`cmd/fixprguard/main.go:303-320`（`stop`）。`AppendPR` の `checkPostText(body)`（`write.go:404`）は現在 `errCodexMention`（`ErrInvalidInput` を含めて返す、終了コード 2）を返すため、`ErrCurrentBodyMention`（`ErrInvalidInput` を含めない、`StopCurrentBodyMention = "current-body-mention"`、終了コード 1）へ分ける。
    -   テストヘルパ: `internal/fixprguard/test_helpers.go:24` と `cmd/fixprguard/test_helpers.go:25` に `testSecondReviewer = "chatgpt-codex-connector"`、`internal/fixprguard/types_test.go:51` と `trust_test.go:155` に `"chatgpt-codex-connector"` がある。
    -   `internal/fixprguard/testdata/` は現在存在しない。`review-state` のサンプル用に新設する。
-   **コマンド定義の `<review-mention>` と文書ガードテスト（`dc224b5c`）:**
    -   `.claude/commands/_lib/local-cli-review.md:35,305` に `<review-mention>` があり、`cmd/yt2column/review_records_test.go:1365-1389`（`TestRecordCommitsCheckMention`）が `mentionCheckCommand` で検査している。一方、`.claude/commands/_lib/simple-process.md` には `<review-mention>` がなく、`:343` に Codex の未レビュー変更の記述がある。
    -   `cmd/yt2column/codex_review_docs_test.go`: `TestCodexMentionOnlyInRequestRow`、`TestPostingPathsCheckMention`、`TestPostingCommandsInventory`（`:227` が `gh pr comment <number> --body "<review-request>"` をファイル・節を問わず無条件スキップしている）、`TestCodeRabbitConfigStopsReview`、`TestAgentsCodeReviewRules`。
    -   `cmd/yt2column/fixpr_docs_test.go`: `wantReviewers`（`:62`）、`TestRunprDocContract`（`:158`）、`TestHandoffListingPrintsNoBody`（`:196`）、`TestContextAndCommandsAgree`（`:1129`）、`TestContextReviewerRowsMalformed`（`:1161`）、`TestLocalReviewGuard`（`:1293`）。
    -   `cmd/yt2column/review_records_test.go`: `seenRangeForm`（`:83`）、`TestReviewRecordsCoverage`（`:652`）、`TestReviewRecordsCallSite`（`:1234`）、`TestRecordLinePrefixesDistinct`（`:1318`）、`TestRecordCommitsCheckMention`（`:1365`）。
    -   `cmd/fixprguard/docs_test.go`: `TestGuideCoversStopKindsAndScope`（`:52`）、`TestGuideListsInteractionLimitsValues`（`:82`）、`TestDocumentedInvocationsParse`（`:212`）、`TestDocumentedProtocolMatches`（`:351`）、`TestGuideDescribesMentionCheck`（`:424`、`fixprguard.CodexMention` を参照）。
-   **ADR と用語集（`dc224b5c`）:**
    -   `食い違い:` 記述がある ADR: `0013`（`:19`）、`0014`（`:49`）、`0015`（`:27`）、`0042`（`:37`）、`0047`（`:21`）、`0050`（`:38`）。
    -   Codex に触れる残りの ADR: `0040`（`:11`）、`0046`（`:284,473`）。
    -   `docs/translation_glossary.md:119` に「試行」、`.coderabbit.yaml:9-12` にコメントがある。

### 1.2 計画の際に決めたこと（2026-11-20・2026-11-21）

利用者に尋ねて決めたこと（2026-11-21）:

-   **フェーズ 2 の PR 分割（4 本構成）:** 設計書 §6.3 は 3 本の PR を想定しているが、フェーズ 2 を PR-2（`fixprguard review-state` 単独、`Protocol` 据え置き、`sonnet+opus-review`）と PR-3（2 メンション拒否・`Protocol` 引き上げ・コマンド定義とガードテストの切り替え、`opus`）の 2 本に分け、フェーズ 3 を PR-4（`sonnet`、`hold`）とする。高リスクな状態機械と厳密 JSON 検査のレビューをコマンド定義 10 ファイルの書き換えから切り離し、PR-2 を `sonnet+opus-review` で回すためである（設計書 §6.3 の「分け方の細部と、各 PR のステップは計画書で決める」および AC-45 に適合）。PR-2 で `fixprguard` を先行変更し Codex 連携解除が PR-3 マージ後になる点は受け入れる。推奨に従って利用者が選んだ。ADR の判断: ADR なし（本タスク内の PR 分割であり、タスク内で閉じるため）。

設計書が計画書に委ねた細部と計画上の配置（2026-11-20）:

-   **`_lib/task-pr-handoff.md` Part B の新しい手順の番号:** `2.5.` とする。0114 は `dc224b5c` に入っておらず、既存の step 3・step 4 の番号と `codex_review_docs_test.go`・他文書からの `Part B step 3` 参照を保てるためである。ADR の判断: ADR なし（文書内の手順番号であり、タスク内で閉じるため）。
-   **`review-state` の配置:** `internal/fixprguard/review_state.go`・`review_state_test.go`・`testdata/` とする。既存の `limits.go`・`limits_test.go` と同じ構成にそろえるためである。ADR の判断: ADR なし（ファイル配置であり、タスク内で閉じるため）。
-   **`codexreview` 撤去の静的検証スクリプトの配置:** `docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` に置く。`Makefile`・`.claude/settings.json`・`cmd/` と `internal/` の Go ソースに参照が残っていないこと（AC-32〜AC-34・AC-36）を、文書中の生コマンドではなく実行可能なスクリプトで検証するためである。ADR の判断: ADR なし（タスク専用の検証スクリプトであり、タスク内で閉じるため）。
-   **`fixprguard` の拒否メンション一覧の公開 API:** `RejectedMentions() []string` に確定する。既存の `StopKinds() []string`・`InteractionStates()` と同じスライス返却関数にそろえ、`cmd/fixprguard/docs_test.go` や `cmd/yt2column/` のガードテストから一貫して参照するためである。ADR の判断: ADR なし（パッケージ内の公開関数の形であり、タスク内で閉じるため）。

### 1.3 リネーム一覧

本タスクで改名するファイル・識別子・テスト関数をここに集約する。文書内のすべての参照はこの表に従う。

| 旧名 | 新名 | 対象ファイル | フェーズ |
|---|---|---|---|
| `cmd/yt2column/codex_review_docs_test.go` | `cmd/yt2column/review_mention_docs_test.go` | `cmd/yt2column/`（改名だけの commit を分ける） | フェーズ 2 |
| `errCodexMention` | `errMention` | `internal/fixprguard/errors.go`・`write.go`・テスト | フェーズ 2 |
| `CodexMention` | `RejectedMentions() []string`（`@codex`・`@coderabbitai` を返す） | `internal/fixprguard/errors.go`・`write.go`・テスト・文書ガード | フェーズ 2 |
| `TestCodexMentionOnlyInRequestRow` | `TestMentionsOnlyInAllowedRows` | `cmd/yt2column/review_mention_docs_test.go` | フェーズ 2 |
| `TestReplyRejectsCodexMention` | `TestReplyRejectsMention` | `internal/fixprguard/write_test.go` | フェーズ 2 |
| `TestEditPRRejectsCodexMention` | `TestEditPRRejectsMention` | `internal/fixprguard/write_test.go` | フェーズ 2 |

### 1.4 実装の際に決めたこと（2026-11-21）

-   **フェーズ 1 のコミット順:** ステップ 1-3 のテスト移設とカバレッジ比較を先に行う。旧手順を検査するテストが各コミットで通るよう、ステップ 1-1 の手順撤去はステップ 1-4 の製品コード・テスト削除と同じコミットにする。ADR の判断: ADR なし（タスク内のコミット順の変更であるため）。
-   **`review-records.sh` の `flip` にグリーンゲート確認チェックボックスを追加:** `/runplan` step 5a のグリーンゲート確認チェックボックス commit が Part B step 2.5 で未レビュー扱いになり AC-31 を満たせないため、`review-records.sh` の `flip` にグリーンゲート確認チェックボックス（行の残り不変）を追加し、`cmd/yt2column/review_records_test.go` の `TestReviewRecordsCoverage` にテストケースを追加する（設計書 1.4）。ADR の判断: ADR なし（既存スクリプトのチェックボックス判定の漏れ修正であるため）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: ローカルの CLI レビューの変更・PR 前のレビュー追加・`codexreview` の撤去

設計書 §3.2・§3.6・§3.8・§7.4・§8（PR-1）に対応する。

実行順は §1.4 に従い、1-3 を先に行う。

#### ステップ 1-1: ローカルの CLI レビューから Codex CLI を外す（F-006、AC-26〜AC-28）

**変更ファイル:**
- `.claude/commands/_lib/local-cli-review.md`
- `.claude/commands/fixpr.md`
- `.claude/commands/runplan.md`
- `.claude/commands/runpr.md`（冒頭・Authorization の Codex CLI / `codexreview` 記述）
- `cmd/yt2column/fixpr_docs_test.go`（`TestLocalReviewGuard`）
- `cmd/yt2column/review_records_test.go`（`seenRangeForm`・`TestRecordLinePrefixesDistinct`）

**作業内容:**
- [x] `_lib/local-cli-review.md` の step 3（`codexreview` の実行・表・`stop` の扱い）を撤去し、step 2 の失敗をすべて `not run` 記録として進む形に改める（設計書 3.6）。
- [x] `_lib/local-cli-review.md` の step 7 の新しく書く記録行を `(coderabbit)` / `(coderabbit: <kind>)` の形に改め、「Reading the record lines」には古い `Codex local review:`・`(codex)`・`(coderabbit: <kind>; codex: <kind>)` の読み方を残し、外せる条件（古い形を持つ開いたブランチがなくなったとき）を記す（AC-28）。
- [x] `_lib/local-cli-review.md` の step 8 SUMMARY から「Codex CLI が走った回数」と `instructions_changed` の注記を外す。
- [x] `fixpr.md`（Authorization の `codexreview` 記述、Phase 4.5、Phase 7 の Codex CLI 回数）、`runplan.md`（step 7.7・B1・step 8 の Codex CLI フォールバック・回数・instructions 注記）、`runpr.md`（`:9,95-96` の Codex CLI / `codexreview` 記述）を更新する。
- [x] 同じ commit で `cmd/yt2column/fixpr_docs_test.go` の `TestLocalReviewGuard`（step 3 確認を外す）と `cmd/yt2column/review_records_test.go` の `seenRangeForm`（`(coderabbit)` 形式）・`TestRecordLinePrefixesDistinct` の下限を更新し、リポジトリ全体でテストが通る状態を保つ。`TestReviewRecordsCoverage` の `(codex)` を読む既存テストは AC-28 の検証として残す。

**完了基準:**
- `_lib/local-cli-review.md`・`fixpr.md`・`runplan.md` に `codexreview` と Codex CLI フォールバックの記述が残らず、`go test -tags test ./...` が通る。

#### ステップ 1-2: PR を開く直前の未レビュー commit のローカル CLI レビューを追加する（F-007、AC-29〜AC-31）

**変更ファイル:**
- `.claude/commands/_lib/task-pr-handoff.md`
- `.claude/commands/_lib/review-records.sh`
- `.claude/commands/runplan.md`
- `.claude/commands/runpr.md`（step 1 の単独 PR 作成）
- `cmd/yt2column/review_records_test.go`（`TestReviewRecordsCallSite`・`TestReviewRecordsCoverage`）
- `cmd/yt2column/testdata/review_records/`（グリーンゲート確認チェックボックス用のフィクスチャ）

**作業内容:**
- [x] `_lib/task-pr-handoff.md` Part B の step 2（CHECKS）と step 3（push と PR 作成）の間に step 2.5 を追加する（設計書 3.2）。`origin/main` の `review-records.sh coverage none <HEAD> [<PLAN>]` を呼び、`ALL` ならレビューせず、`PARTIAL <n>` またはスクリプト失敗時はローカルの CLI レビューを 1 回だけ走らせ、直しがあれば CHECKS を再実行する手順を書く。
- [x] `_lib/task-pr-handoff.md` の Authorization（`codexreview` 記述）、Part B step 1（Codex CLI フォールバック記述）、step 2（CHECKS 一覧の `./cmd/codexreview/...`）、step 3（PR 説明の「Codex CLI が走った回数」）を更新する。
- [x] `_lib/review-records.sh` のヘッダコメントに F-007 の `coverage none` 呼び出しを追記し、`flip` にグリーンゲート確認チェックボックスを追加する（1.4。範囲判定と `(coderabbit|codex)` の正規表現は変えない）。
- [x] `runplan.md` の「Before the PR」の理由を ADR-0042 に合わせ、Part B の CHECKS の規則に「step 2.5 の直しとその後の CHECKS の直しは Opus レビューの判断のやり直しを引き起こさない」旨を追記する（設計書 3.2）。
- [x] 同じ commit で `cmd/yt2column/review_records_test.go` の `TestReviewRecordsCallSite` に `_lib/task-pr-handoff.md` Part B step 2.5 の `coverage none` 呼び出しを追加し、`TestReviewRecordsCoverage` にグリーンゲート確認チェックボックスのテストケースを追加する（1.4）。

**完了基準:**
- 設計書 3.2・6.2 のフローと一致し、`go test -tags test ./...` が通る。

#### ステップ 1-3: `RunsCommand` テストの移設と、残存パッケージの `codexreview` テスト削除（AC-35・AC-36、H-04）

`make coverage-compare BASE=<親>` は `<親>` と `HEAD` の両方で `go test -tags test ./...` を通す必要があるため、`cmd/codexreview`・`internal/codexreview` 削除より前に、リポジトリのテストが通る状態で次の 3 つの commit を順に作る。

**変更ファイル:**
- `internal/doctestutil/commands_test.go`
- `internal/mergepr/protocolcheck_test.go`
- `cmd/fixprguard/docs_test.go`
- `internal/cmdrunner/runner.go`
- `internal/cmdrunner/runner_test.go`

**作業内容:**
- [x] 1 つ目の commit で、`cmd/codexreview/docs_test.go::TestRunsCodex` が担っていた `doctestutil.RunsCommand` のコマンド形式テストを `internal/doctestutil/commands_test.go` へ汎用コマンド名で移し、`cmd/codexreview` を名指すコメント（`:9`）を外す（`doctestutil` のカバレッジ低下を防ぐため）。
- [x] 2 つ目の commit（テスト削除のみ）で、`internal/mergepr/protocolcheck_test.go`（`TestMismatchRemedy` の `"codexreview"` 要素）と `cmd/fixprguard/docs_test.go`（`TestFixprguardProtocolMismatches` の `codexreview` ケースおよびコメント `:345,393`）を外し、`make coverage-compare BASE=<親>` で PASS を確認する（設計書 7.4）。
- [x] 3 つ目の commit で、`internal/cmdrunner/runner.go` の `signalGroup` フォールバック分岐（`codexreview` の削除後に未到達になりうる箇所）を `internal/cmdrunner/runner_test.go` で直接検証できるようにする。

**完了基準:**
- 2 つ目の commit で `make coverage-compare BASE=<親>` が PASS し、`go test -tags test ./...` が通る。

#### ステップ 1-4: `codexreview` の製品コード・テスト・設定・ガード参照を一括撤去する（F-008、AC-32〜AC-37）

**変更ファイル:**
- `cmd/codexreview/`（ディレクトリ削除）
- `internal/codexreview/`（ディレクトリ削除）
- `internal/pipeline/pipeline_test.go`
- `docs/dev/conventions/package_reference.md`
- `internal/config/envaccess_test.go`
- `internal/doctestutil/protocol.go`
- `internal/doctestutil/protocol_test.go`
- `internal/doctestutil/commands.go`
- `internal/cmdrunner/runner.go`
- `cmd/coveragecompare/main.go`
- `Makefile`
- `.claude/settings.json`
- `.github/workflows/ci.yml`
- `CLAUDE.md`（`codexreview` に係る箇所）
- `.claude/commands/_lib/simple-process.md`（CHECKS の `./cmd/codexreview/...`）

**作業内容:**
- [x] 1 つの commit で `cmd/codexreview/` と `internal/codexreview/` を削除し、同時に連動する全ガード・設定を更新して `<親>`・`HEAD` ともにテストが通る状態を保ち、`make coverage-compare BASE=<親>` で PASS（消えた関数が一覧に出るのみ）を確認して結果を commit メッセージに残す（設計書 7.4）。
  - `internal/pipeline/pipeline_test.go` の `TestFakesCarryBuildTag`（ファイル数 24 → 22）と `docs/dev/conventions/package_reference.md`（`cmd/codexreview`・`internal/codexreview`・`internal/codexreview/testutil` 行と `internal/strictjson` 利用元）。
  - `internal/config/envaccess_test.go`（`cmd/codexreview/main.go` 行とコメント）。
  - `internal/doctestutil/protocol.go`（`ToolCodexreview`・`protocolTools` 要素・コメント `:51,98`）、`internal/doctestutil/protocol_test.go`（`TestProtocolFlags` の境界テスト群を削除せず残存ツール `chief`・`mergepr`・`fixprguard` へ書き換え、コメント `:11` 更新）、`internal/doctestutil/commands.go`（コメント `:118-119,200` の `codex` 例を差し替え）。
  - `internal/cmdrunner/runner.go`・`cmd/coveragecompare/main.go` の `codexreview` コメント。
  - `Makefile`（`install-codexreview`）、`.claude/settings.json`（`Bash(codexreview *)`）、`.github/workflows/ci.yml`（`doc-guard` の `./cmd/codexreview`）、`_lib/simple-process.md`（CHECKS の `./cmd/codexreview/...`）、`CLAUDE.md`（`install-codexreview`、`Declare, don't infer` の `codexreview` 例外、Testing Strategy・`doc-guard`・Protocol numbers・Tool Execution Safety の `codexreview` 記述）。

**完了基準:**
- 削除 commit の `make coverage-compare BASE=<親>` が PASS し、`make test && make lint && make deadcode` および CI の `doc-guard` コマンドが通る。

#### ステップ 1-5: フェーズ 1 の ADR 食い違い記述の整理と静的検証スクリプト作成（AC-32〜AC-37・AC-49）

**変更ファイル:**
- `docs/dev/adr/0013-review-setup.md`
- `docs/dev/adr/0014-deterministic-checks-in-go.md`
- `docs/dev/adr/0042-implementation-model-tiers.md`
- `docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh`（新規）

**作業内容:**
- [x] `docs/dev/adr/0014-deterministic-checks-in-go.md` と `docs/dev/adr/0042-implementation-model-tiers.md` の `食い違い:` を外し、`docs/dev/adr/0013-review-setup.md` の `食い違い:` から Codex CLI（`codexreview`）の部分を外す。
- [x] `docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` を作成し、`cmd/codexreview`・`internal/codexreview` が存在しないこと、`Makefile` に `install-codexreview` がないこと、`.claude/settings.json` に `codexreview` の許可がないこと、`.claude/commands/` に `codexreview` 参照がないこと、`cmd/`・`internal/`・`prompts/` 配下の Go ソース（`docs/tasks/` 配下の過去 `_checks/` は除外）に `codexreview` 参照がないことを検証する（実装時に各検査を 1 件以上壊して失敗することを確認する）。

**完了基準:**
- スクリプトが終了コード 0 で通り、`make test && make lint && make deadcode` および `doc-guard` が通る。

### PR-1 作成ポイント: local-cli-review and codexreview removal

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `refactor(0118): Codex CLI をローカルの CLI レビューから外し codexreview を撤去する`

**レビュー観点**: codexreview 撤去と関連ガード（TestFakesCarryBuildTag・package_reference.md・protocolcheck）の整合 / PR 作成直前の未レビュー commit 検出（Part B step 2.5）と Opus レビュー判断との干渉防止 / 古い記録行（AC-28）の後方互換とテスト削除 commit の coverage-compare 結果

**実装モデル要件**: opus

**判定理由**: ステップ 1-2・1-3・1-4 が段階的移行とレビュー関門（Part B step 2.5・CI doc-guard・protocolcheck）の変更に当たり、Sonnet では coverage-compare の前提となる commit 分割や関連ガードの同期を落としやすいため。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1077
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 2: `fixprguard` の拡張・PR レビューの手動化・メンションの拒否と自動レビュアー一覧の更新

設計書 §3.1〜§3.5・§3.8・§4・§5・§7・§8（設計書における PR-2）に対応する。高リスクな状態機械（`fixprguard review-state`）のレビューを独立させるため、本書ではフェーズ 2 を PR-2（`review-state` の追加、プロトコル番号は据え置き）と PR-3（メンション拒否・プロトコル番号引き上げ・コマンド定義とガードテストの切り替え）の 2 本に分ける（設計書 6.3 の「分け方の細部と、各 PR のステップは計画書で決める」および AC-45 に適合）。

#### ステップ 2-1: `fixprguard review-state` サブコマンドとテスト・ガイド・`CLAUDE.md` 例外を実装する（F-004、AC-14〜AC-18・AC-54）

**変更ファイル:**
- `internal/fixprguard/review_state.go`（新規）
- `internal/fixprguard/review_state_test.go`（新規）
- `internal/fixprguard/test_helpers_review.go`（新規：`review-state` テスト用フィクスチャとヘルパ）
- `internal/fixprguard/testdata/`（新規：観測の記録から抽出して本文と `details_url` を取り除いたサンプル）
- `cmd/fixprguard/main.go`
- `cmd/fixprguard/main_test.go`
- `cmd/fixprguard/docs_test.go`（`review-state` のガイド値一覧テスト）
- `docs/dev/guides/fixprguard_guide.md`（`review-state` の章）
- `CLAUDE.md`（「Declare, don't infer」に `review-state` の CheckRun 照合例外を追記）

**作業内容:**
- [x] `internal/fixprguard/review_state.go` に設計書 3.3.1〜3.3.3 の型（`PRReviewState`・`ReviewRequest`・`ReviewResponse`・`ReviewMarker`・`RunKind`・`RequestState`・`UnreadableReason`・`IndeterminateReason`、`MarshalText`、ゼロ値規則）と取得・厳密検査・分類・状態遷移のロジックを実装する。既存の `runGH`・`resolveRepository`・`strictjson`・`parseDateHeader` を再利用する。このステップではまだコマンド定義が `review-state` を呼ばないため、`Protocol` は `main` の値のまま据え置く。
- [x] `cmd/fixprguard/main.go` に `review-state` サブコマンド（`--protocol`・`--reviewer`・`--request`・必須 PR 引数）を登録し、終了コード 0（引数不正のみ 2）で JSON を出力する。
- [x] `internal/fixprguard/review_state_test.go` と `cmd/fixprguard/main_test.go` で設計書 7.1・7.2・7.5 の全ケース（分類表の全行、`RunSkipNotice` 除外、`status != COMPLETED` かつ `startedAt == null` の非カウントによる `waiting` 維持、同秒除外、応答後 CheckRun 除外、競合 `competing-runs`、編集済み依頼 `edited-request` と前の依頼が編集済みの場合の `markers` 非生成、15 分上限、依頼なし時の後段スキップ、厳密 JSON 拒否、上限超過 `limit`、`gh` 失敗 `call-failed`、head 変化時の再取得扱い、PR 外 commit の混入除外、偽の著者・app slug 除外、本文・`title`・`details_url` の非出力）をテストする。実装時に分類表の行や検査を壊して失敗することを確認する。
- [x] `docs/dev/guides/fixprguard_guide.md` に `review-state` の章（項目の名前・状態と理由の列挙語）を追加し、`cmd/fixprguard/docs_test.go` にその値一覧を検証するテストを追加する。
- [x] `CLAUDE.md` の「Declare, don't infer」に設計書 3.3.2 の `review-state` 例外（範囲・理由・境界）を追加する。

**完了基準:**
- `make test && make lint && make deadcode` および `doc-guard` が通る（`cmd/fixprguard` の `review-state` 結合テストで本文・`title`・`details_url` が標準出力にもエラーにも現れないことが検証される）。

### PR-2 作成ポイント: fixprguard review-state subcommand

**対象ステップ**: 2-1

**推奨タイトル**: `feat(0118): 手動の PR レビュー状態を判定する fixprguard review-state を追加する`

**レビュー観点**: fixprguard review-state の厳密 JSON 検査・CheckRun 分類表・状態遷移の正確さ / 境界条件（edited-request・competing-runs・startedAt == null・15 分上限・PR 外 commit 除外）のテスト網羅 / 本文・title・details_url の非出力と CLAUDE.md 例外の境界

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 2-1 が独立した高リスク・複雑な状態機械と厳密 JSON 検査（fixprguard review-state）の実装に当たり、Sonnet では厳密 JSON 検査の抜けや境界条件（edited-request・competing-runs・startedAt == null・headRefOid 一貫性）の検証漏れを起こしやすいため。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1080
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 2-2: 旧テストの削除と `fixprguard` のメンション拒否拡張・`current-body-mention`・プロトコル番号引き上げ（F-009、AC-38・AC-39）

**変更ファイル:**
- `cmd/yt2column/fixpr_docs_test.go`（`fixprguard` を変える前のテスト削除 commit で旧テストを外す）
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
- [x] PR-3 の最初の commit（テスト削除のみ、`go test -tags test ./...` が通る状態）で、`cmd/yt2column/fixpr_docs_test.go` から `TestHandoffListingPrintsNoBody`（`handoffListingFixtures`・`handoffReadCalls` を含む）と要約・受領コメントの `listingItem` テストケース・本文ヘルパを削除し、`make coverage-compare BASE=<親>` で PASS を確認する（設計書 7.4。`checkListingConstructors` が `runpr.md` step 3 の現行 jq 構文を検査しているため、`runprListingKeys["summary"]` と `runprCommentKeys["receipt"]` はステップ 2-5 で `runpr.md` 更新と合わせて外す）。
- [x] `errors.go`・`write.go` の `checkPostText` を `@codex` と `@coderabbitai` の 2 つ（ASCII 大小文字無視、単語境界なし、全角やゼロ幅文字は正規化しない）を拒否するよう拡張し、1.3 のリネーム一覧どおり `errCodexMention` → `errMention`、`CodexMention` → `RejectedMentions() []string` に改める。
- [x] `ErrCurrentBodyMention`（`ErrInvalidInput` を含めない）と `StopCurrentBodyMention = "current-body-mention"` を追加し、`StopKinds()`・`StopKind()` に登録する。`AppendPR` で既存の PR 説明にメンションが含まれる場合は `ErrCurrentBodyMention`（終了コード 1）、追記文面自体に含まれる場合は `ErrInvalidInput`（終了コード 2）を返すようにする（設計書 3.3.5・4.2）。
- [x] `internal/fixprguard/types.go` の `Protocol` を `main` の値 + 1 に上げる（設計書 3.3.5）。
- [x] `internal/fixprguard/test_helpers.go`・`cmd/fixprguard/test_helpers.go` の `testSecondReviewer` および `types_test.go`・`trust_test.go` の `"chatgpt-codex-connector"` を Codex 名でない架空の名前に改める。
- [x] `write_test.go`・`worktree_flow_test.go`・`cmd/fixprguard/main_test.go` を更新し、`@coderabbitai` 拒否・`@coderabbit` や全角 `＠` の非拒否・`append-pr` の `current-body-mention`（終了コード 1）を検証する（実装時に判定を壊してテストが失敗することを確認する）。

**完了基準:**
- 最初のテスト削除 commit で `make coverage-compare BASE=<親>` が PASS し、その後の変更で `go test -tags test ./internal/fixprguard` と `go build ./cmd/fixprguard` が通る（`cmd/fixprguard/docs_test.go` と文書ガードはプロトコル番号・`RejectedMentions()` 参照が文書と結合しているため、ステップ 2-5 で更新してグリーンゲートを通す）。

#### ステップ 2-3: `_context.md` と全コマンド定義のメンション確かめ・PR 作成後手順を更新する（F-002・F-009・F-010、AC-06・AC-07・AC-40・AC-43・AC-46）

**変更ファイル:**
- `.claude/commands/_context.md`
- `.claude/commands/_lib/task-pr-handoff.md`
- `.claude/commands/_lib/local-cli-review.md`
- `.claude/commands/fixpr.md`
- `.claude/commands/_lib/fable-review.md`
- `.claude/commands/_lib/opus-review.md`
- `.claude/commands/_lib/simple-process.md`

**作業内容:**
- [x] `_context.md` を設計書 3.1 の表どおり更新する（`Automated reviewer list` = `coderabbitai`、`Awaited PR reviewer` = `coderabbitai`、`PR review request` = `@coderabbitai review`、新規 `Rejected mentions` = `@codex`・`@coderabbitai`、置き換え名 `<mention-check>`・`<rejected-mentions>` の定義）。
- [x] `_lib/task-pr-handoff.md` Part B step 5（Codex 開始待機・依頼投稿）を撤去し、冒頭・通知・報告の Codex 記述を外す（AC-06・AC-07）。
- [x] `_lib/task-pr-handoff.md`・`_lib/local-cli-review.md`（`:35,305`）・`fixpr.md`・`_lib/fable-review.md`・`_lib/opus-review.md` の `<review-mention>` による確かめを `<mention-check>` に置き換える。
- [x] `fixpr.md` の `--protocol` を `main` の値 + 1 に更新し、`append-pr` の `current-body-mention` 停止の報告手順（AC-39）、`replyBody` 規則、Codex PR レビュー記述（`:59,128-131`）、禁止メンションの記述（`:458,605,1071`）を更新する。
- [x] `_lib/fable-review.md` の冒頭理由を ADR-0050 の新しい理由に改めて Codex CLI 指摘の記述を外し、`_lib/simple-process.md:343` の Codex 未レビュー変更の記述を改める。

**完了基準:**
- 依頼のコメント以外の全投稿経路（記録 commit 経路を含む）が `<mention-check>` を参照し、Part B に PR 作成後のコメント投稿手順が存在しない。

#### ステップ 2-4: `runpr.md` と `_lib/pr-review-escalation.md` を手動 PR レビューと推奨のみのエスカレーションへ更新する（F-003〜F-005・F-009・F-010、AC-08〜AC-25・AC-41・AC-42・AC-44・AC-52〜AC-55）

**変更ファイル:**
- `.claude/commands/runpr.md`
- `.claude/commands/_lib/pr-review-escalation.md`
- `.claude/commands/_lib/review-records.sh`（ヘッダの LAST_REVIEWED 説明）

**作業内容:**
- [x] `runpr.md` のフロントマター・冒頭・Authorization・step 2 等の Codex PR レビュー記述（`:2,48,75,231`）と step 1（単独 PR 作成）の Codex 開始待機・依頼投稿を外す（AC-06・AC-07）。
- [x] `runpr.md` step 3 を設計書 3.4.1 に従って書き換える。見出しを更新し、Codex 要約コメント・受領コメント・状態規則 1〜7・自動依頼を撤去する。`fixprguard review-state` の状態ごとの処理、`<rejected-mentions>` を使うメンションの 3 つの一覧（CodeRabbit の観測済み 2 行を除外し、Codex 投稿は `by_bot` とする）、`LAST_REVIEWED` の算出（設計書 3.3.4）、interaction limits 維持（AC-42）、`fixprguard list` によるスレッド処理（AC-10・AC-44）を書く。
- [x] `runpr.md` step 4 を設計書 3.4.2・3.4.3・3.5 に従って書き換える。「先に PR レビューを依頼」選択肢の表示条件・説明・位置、ラウンドごとの `head` 照合、要約の 15 行（Codex 名なし、手動の PR レビュー行の追加、本文に指摘や未レビューファイルがありうる旨の表示）、詳細ファイル更新、選択時の `gh pr comment` 依頼と `review-state` 待機、停止報告表を書く。自動エスカレーション依頼を撤去し、答えを推奨に写す規則 1〜5 を 1 か所に置く。
- [x] `runpr.md` step 5 の最終報告から受領コメントの行を外し、全 `fixprguard` 呼び出しの `--protocol` を `main` の値 + 1 に更新する。
- [x] `_lib/pr-review-escalation.md` の冒頭・Inputs（LAST_REVIEWED なしでも merge-base..HEAD で判定する）・基準 4 のレビュー関門の範囲（`fixprguard review-state` 等を含む）を更新し、`_lib/review-records.sh` ヘッダの LAST_REVIEWED 説明を合わせる。

**完了基準:**
- 設計書 3.4・3.5・6.1 のすべての分岐と報告項目が `runpr.md` と `_lib/pr-review-escalation.md` に反映されている。

#### ステップ 2-5: 文書ガードテスト・ガイド・ADR 食い違い記述を更新する（AC-06・AC-13・AC-25・AC-40・AC-41・AC-43・AC-46・AC-49・AC-56）

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
- [x] `cmd/yt2column/codex_review_docs_test.go` を `cmd/yt2column/review_mention_docs_test.go` に改名する commit を独立して作る（設計書 7.4）。
- [x] `review_mention_docs_test.go` を設計書 7.3 に従って更新する。
  - `TestMentionsOnlyInAllowedRows`: `fixprguard.RejectedMentions()` の全値が `_context.md` の 2 行以外に出ないことを確かめる。
  - `TestPostingPathsCheckMention`: `<mention-check>` とその展開が `Rejected mentions` の全値を含むことを確かめる。
  - `TestPostingCommandsInventory`: `gh pr comment <number> --body "<review-request>"` の例外を `runpr.md` step 4 の 1 か所のみに制限し、Part B・`runpr.md` step 1・step 3 等に出たら失敗させる（実装時に許可外セクションへ置いて失敗を確かめる）。
  - `TestAgentsCodeReviewRules`: コメントの理由を CodeRabbit に更新する（H-09）。
  - `TestCodeRabbitConfigStopsReview`: そのまま維持する。
- [x] `cmd/yt2column/fixpr_docs_test.go` の `wantReviewers` を `[]string{"coderabbitai"}` に更新し、`runprListingKeys["summary"]`・`runprCommentKeys["receipt"]` を外して、`TestContextAndCommandsAgree`（`awaited` = `coderabbitai`、`PR review request`・`Rejected mentions` 照合）、`TestContextReviewerRowsMalformed`（新構成と `Rejected mentions` 行の異常系）、`TestRunprDocContract`（新しい step 3 見出し・メンション 2 種・CodeRabbit の観測 2 行・`review-state` との依頼コメント判定一致テスト）を更新する。
- [x] `cmd/fixprguard/docs_test.go` に `review-state` の呼び出し形（`runpr.md` の呼び出し検証）と `TestGuideDescribesMentionCheck`（`fixprguard.RejectedMentions()` の 2 値）を追加・更新し、`docs/dev/guides/fixprguard_guide.md` の `current-body-mention`、2 つのメンション、新しいプロトコル番号を更新する。
- [x] `docs/dev/adr/0013`・`0015`・`0047`・`0050` の `食い違い:` 記述を外し、`docs/dev/adr/0047-runpr-merge-question.md:45`（`## 結果` の要約の最大行数）を 14 行から 15 行へ編集上の訂正として更新する（AC-47・AC-49）。

**完了基準:**
- `make test && make lint && make deadcode` および `doc-guard` が通る（`cmd/fixprguard` の `append-pr` 結合テストおよび全ガードテストを含む）。

### PR-3 作成ポイント: manual CodeRabbit PR review and mention rejection

**対象ステップ**: 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0118): 手動の CodeRabbit PR レビューと 2 メンション拒否へ切り替え Codex の PR レビューを外す`

**レビュー観点**: 2 つのメンションの二重拒否（checkPostText・`<mention-check>`）と current-body-mention 停止 / runpr.md step 3・4 の手動依頼・エスカレーション推奨化・プロトコル番号引き上げ / TestPostingCommandsInventory の配置制限と TestRunprDocContract の依頼コメント判定一致

**実装モデル要件**: opus

**判定理由**: ステップ 2-2〜2-5 が信頼の境界とレビュー関門（2 メンション拒否・プロトコル番号引き上げ・runpr.md 書き換え・文書ガード）の段階的移行に当たり、Sonnet では投稿経路のメンション検査漏れ、エスカレーション推奨規則の抜け、ガードテストの不整合を起こしやすいため。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1084
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

> [!IMPORTANT]
> **PR-3 マージ直後の手動作業（要件定義書 4.4、設計書 6.3）:**
> 1. 利用者が `main` から `make install-fixprguard` で `fixprguard` を入れ直す（PR-3 のレビュー中もブランチから入れ直す）。
> 2. 利用者が Codex の GitHub 連携を切り、契約を終える。
> 3. 完了後に PR-4（フェーズ 3）へ進む。

---

### フェーズ 3: 残りの文書・ADR・用語集の更新と chief の確認

設計書 §3.7・§3.8・§3.9・§6.3・§8（設計書における PR-3、本書では PR-4）に対応する。

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
- [x] `.coderabbit.yaml` のコメントを、手動の PR レビューと CodeRabbit CLI のみの説明に改める（`reviews.auto_review.enabled: false` の設定値は変えない）。
- [x] `CLAUDE.md`（Claude Commands の Codex PR レビュー段落等）、`docs/dev/conventions/dev_tools.md`、`docs/dev/conventions/requirements_process.md`、`docs/dev/guides/fixprguard_guide.md`、`docs/dev/guides/mergepr_guide.md` から、今の振る舞いとして書かれた Codex PR レビュー・Codex CLI・`codexreview`・受領コメントの記述を改める（AC-47 の例外に当たる過去出典・`@codex` 拒否・AC-28 古い記録行は残す）。
- [x] `docs/translation_glossary.md` の「試行」の定義を CodeRabbit CLI だけの定義に改め、「手動の PR レビュー」「完了の印」を追加するか用語集の規則に従って判断する（AC-48。どちらも文書で使うため追加した）。

**完了基準:**
- `make test && make lint` および `doc-guard` が通り、現在の仕様を書く文書に AC-47 違反がない。

#### ステップ 3-2: ADR の Codex 記述を点検・更新する（F-011、H-07、AC-47）

**変更ファイル:**
- `docs/dev/adr/0015-untrusted-review-threads.md`
- `docs/dev/adr/0040-simple-task-documents.md`
- `docs/dev/adr/0046-chief-of-staff.md`
- `docs/dev/adr/0047-runpr-merge-question.md`
- `docs/dev/adr/0050-fable-partial-review.md`

**作業内容:**
- [x] 設計書 3.9 に従い、ADR-0040（`:11`）と ADR-0046（`:285`）の Codex に触れる箇所を「過去の出典」「今の振る舞い」「理由が崩れる箇所」に分類する。
- [x] 今の振る舞いとしての記述は `docs/dev/conventions/adr.md` の「Editorial corrections」に従って直し、commit メッセージに `Editorial correction:` 行を残す。決定の理由が崩れる箇所があると判断した場合は、変更前に利用者へ確認する。同じ点検で、ADR-0015 の `背景`、ADR-0047 の `採らなかった案`、ADR-0050 の `決定` に残る、今の振る舞いとして読める Codex の記述も編集上の訂正として直す。

**完了基準:**
- ADR-0015・0040・0046・0047・0050 が AC-47 と ADR 規約を満たす。

#### ステップ 3-3: chief のテスト確認と古い記録行の整理用 issue 作成（F-012、AC-50・AC-51、設計書 3.6）

**変更ファイル:**
- なし（`/runpr` の報告・NEXT 行の形が chief の読み取りに影響する変更になった場合のみ `internal/chief/testdata/` とテストを追加する）

**作業内容:**
- [x] `go test -tags test ./cmd/chief ./internal/chief` を実行し、`internal/chief/testdata/` の既存サンプルがすべて残り成功することを確認する（AC-50）。
- [x] フェーズ 2 の `runpr.md` 変更で `/runpr` の報告・NEXT 行の形が chief の読み取り形式に当たらないことを確認し、当たらないならサンプルを足さない（当たる場合は AC-51 に従い実報告サンプルとテストを追加する）。
- [x] 古い記録行（`Codex local review:`、`(codex)`、`not run` の旧形）の読み方規則を、該当ブランチがなくなった後に外すためのフォローアップ issue を GitHub に作成する（設計書 3.6・6.3）: https://github.com/isseis/yt2column/issues/1087

**完了基準:**
- chief テストが成功し、フォローアップ issue の URL が記録されている。

### PR-4 作成ポイント: remaining docs, glossary, ADRs, and chief verification

**対象ステップ**: 3-1 / 3-2 / 3-3

**推奨タイトル**: `docs(0118): 仕様文書・用語集・ADR の Codex 記述を更新し chief の読み取りを確認する`

**レビュー観点**: 現在の仕様を書く文書・設定コメント・用語集に AC-47・AC-48 違反が残っていないこと / ADR-0015・0040・0046・0047・0050 の Codex 記述が編集上の訂正と決定の変更で正しく分類されていること / chief の既存サンプル維持とフォローアップ issue の作成

**実装モデル要件**: sonnet

**判定理由**: opus・sonnet+opus-review のいずれの契機にも該当しない（残りの仕様文書・用語集・ADR の更新と既存テストの確認であり、新しい機構や信頼の境界の変更を含まないため）。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 対象フェーズ | 主な成果物 | 完了条件 |
|---|---|---|---|
| M1: ローカル CLI レビュー移行と `codexreview` 撤去 | フェーズ 1（PR-1: ステップ 1-1〜1-5） | Codex CLI なしの `_lib/local-cli-review.md`、Part B step 2.5、`codexreview` 撤去、`check_codexreview_removed.sh`、ADR-0013/0014/0042 更新 | `make test && make lint && make deadcode`・`doc-guard`・`coverage-compare`・`check_codexreview_removed.sh` が通る |
| M2: 手動 PR レビュー移行と境界更新 | フェーズ 2（PR-2: ステップ 2-1、PR-3: ステップ 2-2〜2-5） | `fixprguard review-state`（PR-2）、2 メンション拒否・`current-body-mention`・プロトコル番号引き上げ・`_context.md`・`runpr.md`・`task-pr-handoff.md`・ガードテスト・ガイド・ADR-0013/0015/0047/0050 更新（PR-3） | PR-2・PR-3 それぞれで `make test && make lint && make deadcode`・`doc-guard`・`coverage-compare` が通る。PR-3 マージ直後に利用者が `fixprguard` を入れ直し、Codex 連携を切る |
| M3: 仕様文書整合と完了確認 | フェーズ 3（PR-4: ステップ 3-1〜3-3） | `.coderabbit.yaml`・`CLAUDE.md`・規約・ガイド・用語集・ADR-0015/0040/0046/0047/0050 更新、chief テスト確認、フォローアップ issue | `make test && make lint`・`doc-guard` が通り、全 AC の検証が完了する |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | Codex CLI 撤去、Part B step 2.5 追加、`codexreview` 一括撤去、関連ガード更新、検証スクリプト、ADR-0013/0014/0042 整理 | opus | auto |
| PR-2 | 2-1 | `fixprguard review-state` サブコマンド（厳密 JSON 検査・CheckRun 分類・状態遷移・テスト・ガイド・`CLAUDE.md` 例外、`Protocol` 据え置き） | sonnet+opus-review | auto |
| PR-3 | 2-2 / 2-3 / 2-4 / 2-5 | 旧テスト削除、`fixprguard` メンション拒否拡張・`current-body-mention`・プロトコル番号引き上げ、`_context.md`・`runpr.md`・エスカレーション・文書ガード・ガイド・ADR-0013/0015/0047/0050 更新 | opus | auto |
| PR-4 | 3-1 / 3-2 / 3-3 | `.coderabbit.yaml`・`CLAUDE.md`・規約・ガイド・用語集・ADR-0015/0040/0046/0047/0050 更新、chief テスト確認、フォローアップ issue 作成（PR-3 マージ後に利用者が `fixprguard` 入れ直しと Codex 連携解除を行ってから着手） | sonnet | hold |

## 4. テスト戦略 (Test Strategy)

設計書 §7 に従う。新しい `testutil/` パッケージは作らず、パッケージ内ヘルパ（既存の `internal/fixprguard/test_helpers.go`・`cmd/fixprguard/test_helpers.go` と、カテゴリ別の `internal/fixprguard/test_helpers_review.go`、いずれも `//go:build test`）を使う（`docs/dev/conventions/test_organization.md` に準拠）。

-   **単体テスト（`internal/fixprguard`）:** `review_state_test.go` で CheckRun 分類表の全行・状態規則・境界値（`status != COMPLETED` かつ `startedAt == null` の `waiting` 維持、同秒除外、15 分上限、編集済み依頼の `edited-request` と前の依頼が編集済みの場合の `markers` 非生成、競合 CheckRun、上限、厳密 JSON 拒否、PR 外 commit 混入）を検証する。`write_test.go`・`worktree_flow_test.go` で 2 メンション拒否と `ErrCurrentBodyMention` を検証する。各テストは実装時に該当ロジックを壊して失敗することを確かめる。
-   **結合テスト（`cmd/fixprguard`）:** 偽の `gh` で `review-state` と `append-pr` の標準出力 JSON・終了コード・本文・`title`・`details_url` 非出力を検証する。
-   **文書ガードテスト（`cmd/yt2column`・`cmd/fixprguard`・`internal/pipeline`・`internal/doctestutil`・`internal/mergepr`）:**
    -   `review_mention_docs_test.go`: 全投稿経路の `<mention-check>`、`gh pr comment <number> --body "<review-request>"` の出現を `runpr.md` step 4 のみに限定する `TestPostingCommandsInventory`、`_context.md` の 2 行限定のメンション出現、`.coderabbit.yaml` の自動レビュー無効化、`AGENTS.md` 見出し。
    -   `fixpr_docs_test.go`: `wantReviewers` = `coderabbitai`、`TestContextAndCommandsAgree`・`TestContextReviewerRowsMalformed`、`TestRunprDocContract` のメンションフィルタ実行と `review-state` の依頼コメント判定の一致確認、`TestLocalReviewGuard`。
    -   `review_records_test.go`: `(coderabbit)` 形式・`coverage none` 呼び出し・`TestRecordCommitsCheckMention`・グリーンゲート確認チェックボックス（AC-31）・`(codex)` 後方互換（AC-28）。
    -   `cmd/fixprguard/docs_test.go`: `review-state` 呼び出し形・プロトコル番号・ガイドの 2 メンション・状態表・理由表・停止種類。
-   **テスト削除のカバレッジ比較（H-04）:** `cmd/codexreview` 削除前に `TestRunsCodex` のコマンド形式テストを `internal/doctestutil/commands_test.go` へ移したうえで、`cmd/codexreview`・`internal/codexreview` の一括削除 commit と、残るパッケージのテスト削除 commit ごとに `make coverage-compare BASE=<親>` を実行し PASS を確認する。
-   **セキュリティテスト（設計書 7.5）:** 本文・`title`・`details_url` に目印文字列を入れたサンプルで出力への非漏洩を検証し、偽の app slug・CheckRun 名・`User`/`Bot` なりすましを拒否することを検証する。
-   **後方互換性テスト:** `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage` で `(codex)` 行の読み取り（AC-28）を、`internal/chief` テストで既存サンプルの維持（AC-50）を検証する。

## 5. リスク管理 (Risk Management)

-   **移行順序の破綻（AC-45）:** PR-3 より前に Codex 連携を切ると現行 `/runpr` が止まり続け、PR-1 より前に `codexreview` を消すとローカルの CLI レビューが壊れる。対策: PR-1 → PR-2 → PR-3 → 利用者による連携解除 → PR-4 の順序を厳守する。
-   **PR-3 のレビュー中の Codex 自動投稿:** PR-3 を開く時点では Codex 連携が有効なため、PR-3 上で Codex のスレッド（Untrusted threads）と要約コメント（`by_bot` メンション）に対する確認が発生する（設計書 6.3）。対策: 想定内の挙動として利用者が応答して進める。
-   **`fixprguard` のプロトコル番号引き上げと並行ブランチ:** 番号を上げると、入れ直し後、`main` を取り込んでいない並行ブランチの `/fixpr`・`/runpr` は番号不一致で止まる。対策: PR-3 のブランチとマージ後に `make install-fixprguard` を案内し、他ブランチは `main` を取り込む。
-   **`TestFakesCarryBuildTag` と `package_reference.md` の同期漏れ:** `internal/codexreview` 撤去時に固定ファイル数（24→22）と `package_reference.md` の更新が漏れると `make test` が落ちる。対策: ステップ 1-4 の同じ commit で更新し、グリーンゲートで確認する。
-   **`append-pr` の既存 PR 説明との衝突（AC-39）:** CodeRabbit が PR 説明にメンションを含む文を書き足した場合に `append-pr` が止まる。対策: `ErrCurrentBodyMention`（`current-body-mention`、終了コード 1）として区別し、`fixpr.md` が原因を報告して再試行しない手順とする。

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
-   [x] PR-2 マージ済み（対象ステップ: 2-1）
-   [x] PR-3 マージ済み（対象ステップ: 2-2 / 2-3 / 2-4 / 2-5）
-   [x] PR-3 マージ後に利用者が `fixprguard` 入れ直し・Codex 連携解除を完了（PR-4 の `自動送信: hold` 解除条件。2026-11-23 に利用者が完了を確認）
-   [ ] PR-4 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 検証方法 | 種別 | 対応ステップ | フェーズ |
|---|---|---|---|---|
| AC-01 | PR #1070 の `docs/tasks/0118_codex_removal/observations.md` で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-02 | PR #1070 の `docs/tasks/0118_codex_removal/observations.md` §1 で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-03 | PR #1070 の `docs/tasks/0118_codex_removal/observations.md` §1・§2 で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-04 | PR #1070 の `docs/tasks/0118_codex_removal/observations.md` §4 で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-05 | PR #1070 の `02_architecture.md` と `observations.md` の照合で確認済み | manual | 完了済み（#1070） | 設計 PR |
| AC-06 | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory`（`gh pr comment` 依頼例外を `runpr.md` step 4 のみに限定し、Part B・`runpr.md` step 1 に出たら失敗させる） | static | ステップ 2-3・2-4・2-5 | フェーズ 2 |
| AC-07 | `_lib/task-pr-handoff.md` Part B と `runpr.md` step 1 の通知・報告文を PR レビューで確認 | manual | ステップ 2-3・2-4 | フェーズ 2 |
| AC-08 | `internal/fixprguard/review_state_test.go::TestReviewState`（依頼なし時の `none` と後段スキップ）、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse` | test / static | ステップ 2-1・2-4・2-5 | フェーズ 2 |
| AC-09 | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（要約・受領コメントのフィルタが存在しないこと） | static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-10 | `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（`runpr.md` の `fixprguard list` 呼び出し維持）、`internal/fixprguard/trust_test.go` | test / static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-11 | `runpr.md` step 4 の要約 15 行の定義を PR レビューで確認（Codex 名の不在とローカル CLI・Fable 行の維持） | manual | ステップ 2-4 | フェーズ 2 |
| AC-12 | `runpr.md` step 4 の選択肢条件（LAST_REVIEWED != head）を PR レビューで確認、`internal/fixprguard/review_state_test.go::TestReviewState`（`markers` の算出） | test / manual | ステップ 2-1・2-4 | フェーズ 2 |
| AC-13 | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory`（`gh pr comment <number> --body "<review-request>"` を `runpr.md` step 4 の 1 か所のみに限定） | static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-14 | `internal/fixprguard/review_state_test.go::TestReviewState`（`waiting`・`completed`）、`cmd/fixprguard/main_test.go::TestReviewStateRun` | test | ステップ 2-1・2-4 | フェーズ 2 |
| AC-15 | `internal/fixprguard/review_state_test.go::TestReviewState`（app slug・`User`/`Bot` なりすましの拒否）、`TestReviewStateReads`（CheckRun 名フィルタ） | test | ステップ 2-1 | フェーズ 2 |
| AC-16 | `internal/fixprguard/review_state_test.go::TestReviewState`（`competing-runs`・`edited-request`）、`TestReviewStateUnreadable`（`rejected`・非 16 進 SHA）、`TestReviewStateNonPRCommitIgnored`（PR 外 commit 除外）、`TestReviewStateCommitPages`（ページ間 head 照合）、`TestReviewStateOutputHasNoText`（本文非出力）、`cmd/fixprguard/main_test.go::TestReviewStateRun` | test | ステップ 2-1・2-4 | フェーズ 2 |
| AC-17 | `internal/fixprguard/review_state_test.go::TestReviewState`（依頼前の CheckRun 除外・同秒除外・前の依頼の応答範囲） | test | ステップ 2-1 | フェーズ 2 |
| AC-18 | `internal/fixprguard/review_state_test.go::TestReviewState`（`rate-limited`・`unrecognized`・`timed-out`）、`cmd/fixprguard/main_test.go::TestReviewStateRun` | test | ステップ 2-1・2-4 | フェーズ 2 |
| AC-19 | `internal/fixprguard/review_state_test.go::TestReviewStateOutputHasNoText`・`cmd/fixprguard/main_test.go::TestReviewStateRun`（本文非出力）、`runpr.md` step 4 要約行の PR レビュー | test / manual | ステップ 2-1・2-4 | フェーズ 2 |
| AC-20 | `runpr.md` step 4 の手動 PR レビュー要約行（未レビューファイルがありうる旨と URL 表示）を PR レビューで確認（設計書 1.3 の決定） | manual | ステップ 2-4 | フェーズ 2 |
| AC-21 | `_lib/pr-review-escalation.md`・`runpr.md` の範囲定義と `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`（`none` 指定時の `origin/main..HEAD`） | test / manual | ステップ 2-4 | フェーズ 2 |
| AC-22 | `runpr.md` step 4 の答えを推奨に写す規則 2 を PR レビューで確認 | manual | ステップ 2-4 | フェーズ 2 |
| AC-23 | `runpr.md` step 4 の答えを推奨に写す規則 3 と `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage` | test / manual | ステップ 2-4 | フェーズ 2 |
| AC-24 | `runpr.md` step 4 の答えを推奨に写す規則 5 を PR レビューで確認 | manual | ステップ 2-4 | フェーズ 2 |
| AC-25 | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory`（エスカレーション経路を含む `runpr.md` step 4 以外への依頼投稿を拒否） | static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-26 | `cmd/yt2column/fixpr_docs_test.go::TestLocalReviewGuard`、`docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-1・1-5 | フェーズ 1 |
| AC-27 | `_lib/local-cli-review.md` step 7・`_lib/task-pr-handoff.md` Part B step 3・`runpr.md` step 4 の要約行を PR レビューで確認、`cmd/yt2column/review_records_test.go::TestRecordLinePrefixesDistinct` | static / manual | ステップ 1-1・1-2・2-4 | フェーズ 1・2 |
| AC-28 | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`（`(codex)` 行を見た範囲として読む既存テストの維持） | test | ステップ 1-1 | フェーズ 1 |
| AC-29 | `cmd/yt2column/review_records_test.go::TestReviewRecordsCallSite`（`_lib/task-pr-handoff.md` Part B step 2.5 の `coverage none` 呼び出し）、`TestReviewRecordsCoverage` | test / static | ステップ 1-2 | フェーズ 1 |
| AC-30 | `_lib/task-pr-handoff.md` Part B step 2.5・step 3 の失敗時 `not run` 記録と PR 説明出力を PR レビューで確認 | manual | ステップ 1-2 | フェーズ 1 |
| AC-31 | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`（空 commit・グリーンゲート確認を含むチェックボックス変更のみの commit を `ALL` とするテスト） | test | ステップ 1-2 | フェーズ 1 |
| AC-32 | `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`、`TestFakesCarryBuildTag`、`docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-4・1-5 | フェーズ 1 |
| AC-33 | `internal/mergepr/protocolcheck_test.go::TestMismatchRemedy`、`docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-3・1-4・1-5 | フェーズ 1 |
| AC-34 | `docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-4・1-5 | フェーズ 1 |
| AC-35 | `internal/mergepr/protocolcheck_test.go::TestWorktreeProtocolTools`、`TestMismatchRemedy`、`internal/doctestutil/protocol_test.go` | test / static | ステップ 1-3・1-4 | フェーズ 1 |
| AC-36 | `internal/config/envaccess_test.go::TestEnvAccessConfined`、`docs/tasks/0118_codex_removal/_checks/check_codexreview_removed.sh` | static | ステップ 1-3・1-4・1-5 | フェーズ 1 |
| AC-37 | `make test`、`make lint`、`make deadcode`、CI の `doc-guard` コマンド | test / static | ステップ 1-3・1-4・1-5 | フェーズ 1 |
| AC-38 | `internal/fixprguard/write_test.go`（`checkPostText`・`TestReplyRejectsMention`・`TestEditPRRejectsMention`・`AppendPR`）、`internal/fixprguard/worktree_flow_test.go`、`cmd/fixprguard/main_test.go` | test | ステップ 2-2 | フェーズ 2 |
| AC-39 | `internal/fixprguard/write_test.go`（`ErrCurrentBodyMention`）、`cmd/fixprguard/main_test.go`（`current-body-mention` 終了コード 1）、`cmd/fixprguard/docs_test.go::TestGuideCoversStopKindsAndScope` | test / static | ステップ 2-2・2-3・2-5 | フェーズ 2 |
| AC-40 | `cmd/yt2column/review_mention_docs_test.go::TestPostingPathsCheckMention`、`TestPostingCommandsInventory`、`cmd/yt2column/review_records_test.go::TestRecordCommitsCheckMention` | static | ステップ 2-3・2-5 | フェーズ 2 |
| AC-41 | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（2 メンション分類・CodeRabbit 観測 2 行除外・`by_user`/`by_bot`・依頼コメント判定の `review-state` 一致） | test | ステップ 2-4・2-5 | フェーズ 2 |
| AC-42 | `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（`runpr.md` の `interaction-limits` 維持）、`internal/fixprguard/limits_test.go` | test / static | ステップ 2-4・2-5 | フェーズ 2 |
| AC-43 | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（`wantReviewers` = `coderabbitai` のみ） | static | ステップ 2-3・2-5 | フェーズ 2 |
| AC-44 | `internal/fixprguard/trust_test.go`（一覧外の Bot が `ReasonOtherBot` になる既存テスト）、`cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree` | test / static | ステップ 2-3・2-5 | フェーズ 2 |
| AC-45 | 本書のフェーズ・PR 順序（PR-1 → PR-2 → PR-3 → 連携解除 → PR-4）の遵守を、PR #1077・#1080・#1084 のマージ記録と、利用者による連携解除の完了の確認で確かめる（連携解除はリポジトリの外の作業なので、test・static・diff では表せない） | manual | 全フェーズ | フェーズ 1〜3 |
| AC-46 | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`、`cmd/yt2column/review_mention_docs_test.go::TestMentionsOnlyInAllowedRows` | static | ステップ 2-3・2-5 | フェーズ 2 |
| AC-47 | 現在の仕様を書く文書一式を PR レビューで確認（AC-47 で `manual` と指定） | manual | ステップ 1-1・1-2・1-4・2-3・2-4・3-1・3-2 | フェーズ 1〜3 |
| AC-48 | `docs/translation_glossary.md` の改訂内容を PR レビューで確認 | manual | ステップ 3-1 | フェーズ 3 |
| AC-49 | `docs/dev/adr/0013`・`0014`・`0015`・`0042`・`0047`・`0050` の `食い違い:` 削除と、変更した `## 決定`（ADR-0050）を PR レビューで確認 | manual | ステップ 1-5・2-5・3-2 | フェーズ 1〜3 |
| AC-50 | `go test -tags test ./cmd/chief ./internal/chief` | test | ステップ 3-3 | フェーズ 3 |
| AC-51 | `runpr.md` step 5 の変更差分が chief の読み取り形式に当たらないことを確認（当たる場合は新サンプル追加と `go test -tags test ./internal/chief`） | test / manual | ステップ 3-3 | フェーズ 3 |
| AC-52 | `runpr.md` step 4 の手動 PR レビュー要約行（URL と本文に指摘がありうる旨）を PR レビューで確認 | manual | ステップ 2-4 | フェーズ 2 |
| AC-53 | `internal/fixprguard/review_state_test.go::TestReviewState`（終わった依頼の状態返却）、`runpr.md` step 4 要約行の PR レビュー | test / manual | ステップ 2-1・2-4 | フェーズ 2 |
| AC-54 | `internal/fixprguard/review_state_test.go::TestReviewState`（応答後の CheckRun を完了の印に数えないこと） | test | ステップ 2-1 | フェーズ 2 |
| AC-55 | `runpr.md` step 4 の答えを推奨に写す規則 1・4 を PR レビューで確認 | manual | ステップ 2-4 | フェーズ 2 |
| AC-56 | `cmd/yt2column/review_mention_docs_test.go`（`TestMentionsOnlyInAllowedRows`・`TestPostingPathsCheckMention`・`TestPostingCommandsInventory`・`TestCodeRabbitConfigStopsReview`） | static | ステップ 2-5 | フェーズ 2 |

## 8. 成功基準 (Success Criteria)

-   **機能的完全性:** AC-06〜AC-56 がすべて検証を通り、PR-3 マージ後に利用者が Codex 連携を切った状態で PR-4 が `/runpr` で回る。
-   **品質基準:** 各フェーズの完了時に `make test && make lint && make deadcode` および CI の `doc-guard` コマンドが通る。テスト削除 commit はすべて `make coverage-compare` が PASS する。
-   **セキュリティ検証:** `review-state` とメンションフィルタが本文・`title`・`details_url` を出力しないこと、2 つのメンションの二重拒否（`<mention-check>` と `checkPostText`）が全経路で機能することがテストとガードで検証される。
-   **文書整合性:** 現在の仕様を書く文書と ADR に食い違いが残らず、`docs/dev/guides/fixprguard_guide.md` と `docs/translation_glossary.md` が更新されている。

## 9. 次のステップ (Next Steps)

1.  本計画書の PR をレビューし、`/runpr` で承認・マージする。
2.  承認後、`/runplan 0118` でフェーズ 1 から順に実装する。

## 変更履歴

- 2026-11-21 承認（isseis、#1074）
- 2026-11-21 セッション中の再承認：フェーズ 1 のテスト移設と削除のコミット順を変更（isseis、#1077）
- 2026-11-21 セッション中の再承認：review-records.sh の flip にグリーンゲート確認チェックボックスを追加（isseis、#1077）
