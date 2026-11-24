# 実装計画書：Sourcery AI を自動レビュアーの一覧と PR レビュー体制に加える

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-24 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的と実装方針

本書は、[01_requirements.md](01_requirements.md)（F-001〜F-005、AC-01〜AC-31）および [02_architecture.md](02_architecture.md) に基づき、Sourcery AI（`sourcery-ai`）を自動レビュアーの一覧と PR レビュー体制へ組み込むための実装手順と検証計画を定める。設計の詳細（型定義・CheckRun 分類表・フロー・境界チェックリスト）は [02_architecture.md](02_architecture.md) を参照し、本書では重複記述を避ける。

- **実装フェーズの原則:** [02_architecture.md](02_architecture.md) 8 章の優先順位（フェーズ 1〜4）に従う。`fixprguard` の `Protocol` 番号（`internal/fixprguard/types.go:21`、コミット `d29bb3d7` 時点で `8`）はフェーズ 3 の契約変更時にのみ `main` の値 + 1 へ引き上げ、コマンド文書・ガイド・テスト・`CLAUDE.md` 例外を同一 PR で整合させる（`CLAUDE.md`「Protocol numbers」）。`mergepr` の `Protocol`（`internal/mergepr/mergepr.go:63`、コミット `d29bb3d7` 時点で `3`）は CLI 契約が不変のため据え置く。
- **既存基盤の再利用:** `internal/cmdrunner`・`internal/cmdrunner/testutil`・`internal/strictjson`、`internal/fixprguard/review_state.go` の `restGet`・`restPages`・`readHead`・`readUser`・`parseCommitsPage` を再利用し、リポジトリ解決と GitHub Actions CI チェック読み取りを共有パッケージ `internal/ghci` へ集約する。
- **Go ソースの英語規約:** 新設・変更する Go ソース（テストおよび `test_helpers*.go` を含む）のコメント・識別子・文字列リテラルは英語とする（`CLAUDE.md`「Go Idioms」のガードテスト用リテラル例外に該当する日本語文書の見出し・ラベル照合のみ除く）。
- **開発ツールの再インストール:** 開発ツール（`fixprguard`・`mergepr`）のインストールは利用者が行い、エージェントは実行しない（`CLAUDE.md`「Tool Execution Safety」）。

### 1.2. 計画の際に決めたこと（2026-11-24）

- **Sourcery AI の再レビューが見る範囲（`02_architecture.md` 5.4 の未確認の前提）:**
  - **決定:** 現行設計（AC-17・AC-18・`02_architecture.md` 3.2.2）および [ADR-0013](../../dev/adr/0013-review-setup.md) を変更せず、そのまま実装計画を作成する。PR #1097・#1098 の確認結果を本節と [observations.md](observations.md) に記録する。
  - **理由:** 計画前に PR #1097 と PR #1098 を読み取り専用 `gh api` で確認したところ、初回コミットが 7 日間予算超過で `SKIPPED` になった後の後続コミット（#1097 `516b7e5b` は `03_implementation_plan.md` の +1/-1 行のみ、#1098 `3de3c7bd` は `01_requirements.md` の +7/-3 行のみ）で `SUCCESS` 完了したレビューが、いずれも直近コミットで触れていない先行コミットの変更箇所（#1097 `#discussion_r4665768633` の `docs/dev/adr/0050-fable-partial-review.md` 90 行目、#1098 `#discussion_r4666409559` ほか 3 件の ADR-0013・0015・0047）に指摘を付けていた。初回 `SKIPPED` 後の `SUCCESS` 完了は先行コミットを含めてレビューしており、`LAST_REVIEWED` で祖先コミットをレビュー済みと数える前提が実測で裏付けられた。
  - **ADR の判断:** ADR なし（現行 [ADR-0013](../../dev/adr/0013-review-setup.md) の前提が裏付けられ、決定の変更がないため）。
- **フェーズ 2・フェーズ 3 の PR 分割（6 PR 構成）:**
  - **決定:** [02_architecture.md](02_architecture.md) 8 章の 4 フェーズの順序を保ったまま、フェーズ 2 を 2 PR（`internal/ghci` 新設と `internal/mergepr` 切り替え）、フェーズ 3 を 2 PR（`internal/fixprguard` の非破壊内部追加と、`review-state` 拡張・`cmd/fixprguard`・`Protocol` 引き上げ・コマンド文書・ガイド・ガード一括更新）に分け、全体 6 PR とする。
  - **理由:** 新規共有パッケージ `internal/ghci` と `fixprguard` の新規ロジック（`mentions`・`ci-checks`）を独立 PR で深くレビューでき、`Protocol` 引き上げと `runpr.md` 大幅改定を伴う PR の変更量とレビュー負荷を抑えられるためである。`Makefile:364`（`d29bb3d7`）の `deadcode` は `-test` 付きで実行されるため、単体テストを伴う内部追加 PR も単独でグリーンゲートを通過できる。
  - **ADR の判断:** ADR なし（実装の分割と順序はこのタスク内に閉じるため）。

### 1.3. 既存コード調査結果

`origin/main` コミット `d29bb3d7` で対象コードを調査した結果をまとめる。

- **`internal/fixprguard/`（既存と変更点）:**
  - `errors.go:79-81`: `RejectedMentions()` が `[]string{"@codex", "@coderabbitai"}` を返す。`write.go:91`（`checkPostText`）がそのまま走査するため、`reply`・`edit-pr`・`append-pr`・`finish` に自動反映される。
  - `types.go:21,89-105`: `const Protocol = 8`。`ParseReviewers` と `trust.go` はすでに複数 `--reviewer` に対応しており、AC-07 の著者判定ロジック自体は変更不要（`trust_test.go`・`github_test.go`・`write_test.go` へ `sourcery-ai` ケースを追加する）。
  - `github.go:50-60,99-124`: `resolveRepository` が `gh repo view --json nameWithOwner` を厳格解析している（`github.go:344,495,547`、`limits.go:195`、`write.go:183`、`review_state.go:399` の 6 か所から呼ばれる）。フェーズ 3 で `internal/ghci.ResolveRepo` へ委譲し、`githubTimeout`（`github.go:22`）とエラー変換（`ErrResponseRejected`・`Unreadable*`）を維持する。`NewGitHubWithoutReviewer` の doc コメントへ `PRReviewState`・`PRMentions`・`CIChecks` を追記する。
  - `review_state.go:44-50,334-348,417-421,589,655-679,1052-1190`: CheckRun 名 `"CodeRabbit"` と分類表が固定で、依頼コメント 0 件時に早期リターンしている。`isRequestComment` 判定（`589` 行目）を抽出すれば `mentions.go` と完全に共有できる。`restGet`・`restPages`・`readUser`・`userMember`・`timeMember`・`optionalTimeMember`・`nullableStringMember` は `mentions.go` からそのまま再利用可能。
  - `construction_test.go:17-41`: `Login`・`PRNumber`・`Baseline`・`Reviewers` の直接構築を AST で制限している。新設する `ObservedBot` は `Profile ReviewProfile` と `Reviewer Login` を持ち、`Login` は既存の構築制限がそのまま効く。`ReviewProfile` は `int` 列挙型でゼロ値 `ProfileUnknown` を不正（`MarshalText` がエラー、入口で `ErrInvalidInput` 拒否、全 `switch` の `default` が安全側）とするため、新たな構築制限登録は不要である。
  - `test_helpers_review.go:1-408`（`//go:build test`）: 既存の `rsInput`・`rsRun`・`rsCommit` を拡張し、`Profile`・複数依頼・Sourcery CheckRun ヘルパーを追加して再利用する。
- **`cmd/fixprguard/`（既存と変更点）:**
  - `main.go:57-141,205,495-518`: `specs` と `multiFlags`（現在 `flagAccept`・`flagThread`・`flagReviewer`）で全サブコマンドを宣言的に管理している。`flagProfile`・`flagObservedBot`・`flagHead`・`cmdMentions`・`cmdCIChecks` を追加し、`flagRequest` と `flagObservedBot` を `multiFlags` へ加える。
  - `docs_test.go:110-155,210-335,384-457,504-519`: `TestGuideDescribesMentionCheck` は `fixprguard.RejectedMentions()` を参照して `fixprguard_guide.md` の `### 7.1`・`### 7.2` を検査するため、フェーズ 1 でガイドの 4 メンション記載を同時更新する必要がある。`TestGuideDescribesReviewerList` は `specs` で `flagReviewer` を持つサブコマンドの見出しに `--reviewer LOGIN`（`review-state` 以外は `...` 付き）を要求するため、フェーズ 3 で `review-state` の見出しに `--reviewer LOGIN` を保ち、`mentions` は `--observed-bot` を使う。`documentedInvocations`（`253-270` 行目）は `strings.Fields` で分割するため、`runpr.md` のコードブロック内の繰り返しフラグは 1 回ずつ書き、繰り返し規則を地の文に置く。
- **`internal/mergepr/`・`cmd/mergepr/`（既存と変更点）:**
  - `internal/mergepr/mergepr.go:63,76-111,275-277`: `const Protocol = 3`。`Prepare` が `t.gh(ctx, "pr", "checks", state.number(), "--watch", "--fail-fast")` を呼び、失敗時に `errChecksFailed`（`errors.go:16`）を返す。`Tool` は `mainFetch Runner` を `New` 内で初期化しテスト（`test_helpers.go:166-180` の `newCheckedTool`）で差し替える構造を持つため、`ghci` 用の上限付き `Runner`（`runner.go` に追加、`MaxOutput: 32 MiB`・`WaitDelay: 5s`・`Env: gitEnviron()`）と `ghci.WaitOptions` も同じ形で `Tool` フィールドに保持し、`mergepr_test.go`（`31,267,761,929,975` 行目の `gh pr checks` モック 5 箇所）を `gh repo view`＋GraphQL 応答へ差し替える。
  - `cmd/mergepr/docs_test.go:175-195`: `TestProtocolToolsAgree` は `internal/*/*.go` の `const Protocol =` 宣言と文書ガードの一致を検証する。`internal/ghci` は `Protocol` 定数を持たない共有パッケージのため影響しない。
- **`cmd/yt2column/` の文書ガードとテスト削除対象:**
  - `fixpr_docs_test.go:53-64,160-197,292-839,945-1229`:
    - `wantReviewers = []string{"coderabbitai"}`（`64` 行目）をフェーズ 1 で `[]string{"coderabbitai", "sourcery-ai"}` へ更新し、`TestContextReviewerRowsMalformed` の 1 社・2 メンション前提のテストケースを合わせる。
    - `TestRunprDocContract/listings_print_no_body`（`190-192` 行目）と `TestRunprDocContract/request_comment_agrees_with_review_state`（`194-196` 行目）は旧 `--jq` フィルタを対象とするため、フェーズ 3（ステップ 3-3）で `fixprguard mentions`・`ci-checks` の契約検査へ置換して削除する。**`runJQ`（`721` 行目）と `gojqStrictPrelude`（`717` 行目）は `TestFixprPhase6CountFilter`（`902` 行目）が引き続き使用するため削除しない。** テスト削除は `CLAUDE.md`「Deleting a test is a claim that must be checked」に従い単独コミットで行い、`make coverage-compare BASE=<親コミット>` を実行・記録する。
  - `review_mention_docs_test.go:93-169,238-308`:
    - `TestPostingPathsCheckMention/context_placeholder_definitions`（`165-167` 行目）は `_context.md` の `Rejected mentions` 行に `` `<rejected-mentions>` `` があることを要求している。フェーズ 1 では維持し、フェーズ 3 で `<rejected-mentions>` を廃止する際にこのアサーションも更新する。
    - `TestMentionsOnlyInAllowedRows`（`93-137` 行目）と `TestPostingCommandsInventory`（`276` 行目、`gh pr comment <number> --body "<review-request>"`）をフェーズ 3 で `### PR reviewers` 表と `<post-request>` へ更新する。
- **`internal/chief/`・`internal/pipeline/`・その他:**
  - `internal/chief/screen.go:113-177`: `mergeQuestionText` は `^PR #[1-9][0-9]* をどうしますか（第 [1-9][0-9]* 回。要約と詳細ファイルを確認のうえ）$` のみを `Merge: true` と判定し、他の単一選択の問いは `Merge: false`・`QuestionSingle`・`RelayAllowed` で中継する（`relay.go:69-77`）。依頼先選択 3 択の質問文をこの正規表現に一致させないため、プロダクションコードの変更は不要で、単体テスト追加と既存テスト通過で AC-28 を満たす。
  - `internal/pipeline/pipeline_test.go:578`: `TestFakesCarryBuildTag` は `testutil/` 配下のファイル数を `22` で固定している。本タスクでは新規 `testutil/` ファイルを追加しないため（`internal/cmdrunner/testutil` を再利用）、この件数は変わらない。`TestPackageReferenceListsPackages`（`630` 行目）は新パッケージ追加時に `docs/dev/conventions/package_reference.md` への行追加を要求する（ステップ 2-1 で同時更新）。
  - `.claude/commands/`: コミット `d29bb3d7` 時点で `sourcery` の記載は 0 件。`--protocol 8` / `"protocol":8` は `fixpr.md`（11 箇所）、`runpr.md`（14 箇所）、`fixprguard_guide.md`（26 箇所）に存在する（フェーズ 3 で一括更新）。
  - `CLAUDE.md:178-179,215,249-261,490-502,831-835`: `PR review request` 参照、`/runpr`・PR レビュー体制記述、`Declare, don't infer` の `fixprguard` 例外、および `TestRunprDocContract` の `jq` 依存記述を更新する（`TestHandoffListingPrintsNoBody` はすでに存在しないため合わせて整理する）。

### 1.4. 変更・置換対応表

本タスクで行う名前・構造・テストの置換を一覧化し、以降のステップおよび検証表はすべて本表に従う。

| 旧（変更前） | 新（変更後） | 実施ステップ |
|---|---|---|
| `fixprguard.RejectedMentions()` の `[]string{"@codex", "@coderabbitai"}` | `[]string{"@codex", "@coderabbitai", "@sourcery-ai", "@sourceryai"}` | ステップ 1-1 |
| `_context.md` `Automated reviewer list`: `` `coderabbitai` ``（`wantReviewers = []string{"coderabbitai"}`） | `` `coderabbitai`, `sourcery-ai` ``（`wantReviewers = []string{"coderabbitai", "sourcery-ai"}`） | ステップ 1-2 |
| `internal/fixprguard/github.go` の `resolveRepository` 独自実装 | `internal/ghci.ResolveRepo` への委譲 | ステップ 3-1 |
| `internal/mergepr/mergepr.go:275` の `gh pr checks <N> --watch --fail-fast` | `internal/ghci.ResolveRepo`＋`internal/ghci.WaitPR` | ステップ 2-2 |
| `.claude/commands/runpr.md:711` の `gh pr checks <number> --watch --fail-fast` | `fixprguard ci-checks --protocol <N> --head <head> <number>` | ステップ 3-3 |
| `.claude/commands/runpr.md:442-488` の `gh api --paginate --jq` 3 本と `<rejected-mentions>` | `fixprguard mentions --protocol <N> --observed-bot <observed-bot> --request "<accepted-request>" <number>` | ステップ 3-3 |
| `_context.md` の `Awaited PR reviewer` 行・`PR review request` 行 | Domain-specific 節の `### PR reviewers` 表（5 列: `Reviewer`・`Profile`・`Mode`・`Post request`・`Accepted requests`） | ステップ 3-3 |
| `runpr.md` の `<awaited-reviewer>`・`<review-request>` | `<pr-profile>`・`<pr-reviewer>`・`<post-request>`・`<accepted-request>`・`<observed-bot>` | ステップ 3-3 |
| `TestRunprDocContract/listings_print_no_body`・`TestRunprDocContract/request_comment_agrees_with_review_state` | `TestRunprDocContract` の `mentions`/`ci-checks` 呼び出し・旧経路不在検査、および `internal/fixprguard/mentions_test.go` | ステップ 3-3 |

---

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: F-002・F-003 基礎（`fixprguard` の `Protocol` 据え置き）

#### ステップ 1-1: `internal/fixprguard` の `RejectedMentions()` 拡張と単体テスト更新

- **対象ファイル:**
  - `internal/fixprguard/errors.go`
  - `internal/fixprguard/trust_test.go`
  - `internal/fixprguard/github_test.go`
  - `internal/fixprguard/write_test.go`
  - `internal/fixprguard/worktree_flow_test.go`
  - `cmd/fixprguard/main_test.go`
- **作業内容:**
  - [ ] `internal/fixprguard/errors.go` の `RejectedMentions()` を `[]string{"@codex", "@coderabbitai"}` から `[]string{"@codex", "@coderabbitai", "@sourcery-ai", "@sourceryai"}` へ変更する（[02_architecture.md](02_architecture.md) 3.2.1）。
  - [ ] `internal/fixprguard/trust_test.go`・`github_test.go`・`write_test.go`・`cmd/fixprguard/main_test.go` に、`--reviewer coderabbitai --reviewer sourcery-ai` での著者判定テストを追加する（`Bot`/`sourcery-ai` の権限照会なし信頼・`begin` の `authorType: "Bot"`・混在スレッド、`Sourcery-AI`・`sourcery-ai[bot]`・`User`/`sourcery-ai` の非自動レビュアー扱い。AC-07）。
  - [ ] `internal/fixprguard/write_test.go`（`TestCheckPostText`・`TestReplyRejectsMention`・`TestEditPRRejectsMention`・`TestAppendPRRejects`）・`worktree_flow_test.go`（`TestFinishRejectsBodyFile`）・`cmd/fixprguard/main_test.go`（`TestPostTextMentionExitsUsage`）に `@sourcery-ai`・`@sourceryai`（大文字小文字混在・部分文字列・既存 PR 本文の `current-body-mention`）の拒否ケースを追加する（AC-11）。
  - [ ] 壊して確かめる検証: `RejectedMentions()` から `@sourcery-ai` または `@sourceryai` を一時的に外し、`write_test.go`（`TestAppendPRRejects` の `current-body-mention` 経路を含む）と `worktree_flow_test.go` が失敗することを確かめ、コミットメッセージに記録する。
- **完了条件:**
  - `go test -tags test ./internal/fixprguard ./cmd/fixprguard` が通ること（ステップ 1-2 のガイド更新後に `TestGuideDescribesMentionCheck` も含めて通る）。

#### ステップ 1-2: `_context.md` の一覧・拒否メンション拡張と文書ガード・ガイド更新

- **対象ファイル:**
  - `.claude/commands/_context.md`
  - `cmd/yt2column/fixpr_docs_test.go`
  - `docs/dev/guides/fixprguard_guide.md`
- **作業内容:**
  - [ ] `.claude/commands/_context.md` の `Automated reviewer list` 行の値を `` `coderabbitai` `` から `` `coderabbitai`, `sourcery-ai` `` へ、`Rejected mentions` 行の値を `` `@codex`, `@coderabbitai` `` から `` `@codex`, `@coderabbitai`, `@sourcery-ai`, `@sourceryai` `` へ更新する（`<mention-check>`・`<rejected-mentions>` の説明はフェーズ 3 まで維持する。AC-06・AC-10）。
  - [ ] `cmd/yt2column/fixpr_docs_test.go` の `wantReviewers` を `[]string{"coderabbitai"}` から `[]string{"coderabbitai", "sourcery-ai"}` へ更新し、`TestContextReviewerRowsMalformed` の壊した写しケースを 2 レビュアー・4 メンション前提へ更新する（AC-09）。
  - [ ] `docs/dev/guides/fixprguard_guide.md` の 4 章・5 章（書き込み系 4 サブコマンド）・7.1・7.2.1・7.2.3・7.2.6・8 章（`current-body-mention`）・10 章（`sourcery-ai` と 4 メンション該当箇所）をフェーズ 1 の状態に合わせて更新する（`TestGuideDescribesMentionCheck` 通過、AC-26 先行分）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` がすべて通ること。
  - **マージ後の手動作業:** 利用者が `main` から `make install-fixprguard` を実行する（`02_architecture.md` 8 章）。

---

### フェーズ 2: 基盤と `mergepr` の CI 分離

#### ステップ 2-1: 共有パッケージ `internal/ghci` の新設と `package_reference.md` 更新

- **対象ファイル:**
  - `internal/ghci/ghci.go`（新規）
  - `internal/ghci/ghci_test.go`（新規）
  - `docs/dev/conventions/package_reference.md`
- **作業内容:**
  - [ ] `internal/ghci/ghci.go` を作成し、`ResolveRepo`・`ReadPR`・`WaitPR`、状態・理由・失敗項目・オプション型（`State`・`UnreadableReason`・`FailingCheck`・`Request`・`Result`・`WaitOptions`）、番兵 `ErrInvalidRequest` を実装する（依存は `internal/cmdrunner`・`internal/strictjson`・標準ライブラリのみ。厳格 JSON・16 MiB 上限・`app.slug == "github-actions"` 限定・同一ワークフロー最新 `createdAt` 採用・空 `QUEUED` CheckSuite 除外・即時失敗・`ExpectedHead` 一致・厳格 GitHub URL 検証の仕様は [02_architecture.md](02_architecture.md) 3.2.4・5.3 参照）。
  - [ ] `internal/ghci/ghci_test.go` を作成し、`internal/cmdrunner/testutil` を用いて `ResolveRepo`・`ReadPR`・`WaitPR` の全状態遷移・境界値・外部 App 無視（`sourcery-ai` の `FAILURE`/`QUEUED`/100 件超 `hasNextPage`、`coderabbitai` の `SKIPPED`）・同一ワークフロー再実行・0 CheckRun `STARTUP_FAILURE`・`ExpectedHead` 不一致・`NoCheckGrace` と `Timeout`・厳格 JSON 拒否・`ErrInvalidRequest`・禁止テキスト非出力マーカーを網羅検証する（[02_architecture.md](02_architecture.md) 7.1・7.3、AC-19）。
  - [ ] `docs/dev/conventions/package_reference.md` の `Shared Parts` へ `internal/ghci` 行を追加し、`internal/strictjson` 行（`110` 行目）の参照元へ `internal/ghci` を追記する（`TestPackageReferenceListsPackages` 通過）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` がすべて通ること。

#### ステップ 2-2: `internal/mergepr` の CI 待機分離と `mergepr.md`・ガイド・テスト更新

- **対象ファイル:**
  - `internal/mergepr/runner.go`
  - `internal/mergepr/mergepr.go`
  - `internal/mergepr/test_helpers.go`
  - `internal/mergepr/mergepr_test.go`
  - `.claude/commands/mergepr.md`
  - `docs/dev/guides/mergepr_guide.md`
- **作業内容:**
  - [ ] `internal/mergepr/runner.go` に `ghci` 用の `cmdrunner.Runner`（`MaxOutput: 32 MiB`・`WaitDelay: 5s`・`Env: gitEnviron()`）を追加する。
  - [ ] `internal/mergepr/mergepr.go` の `Tool` へ `ghci` 用 runner と `WaitOptions`（既定: `PollInterval: 30s`・`Timeout: 60m`・`PerCallTimeout: 2m`・`NoCheckGrace: 5m`）を保持し、`Prepare`（`275` 行目）の `gh pr checks <N> --watch --fail-fast` を `ghci.ResolveRepo`＋`ghci.WaitPR`（`ExpectedHead: state.HeadRefOID`）へ置き換える。`StatePassed` 以外は `errChecksFailed`（失敗ジョブ名・URL または理由コードのみ）を返す（`Protocol = 3` は据え置く。[02_architecture.md](02_architecture.md) 3.2.4・4.2）。
  - [ ] `internal/mergepr/test_helpers.go`（`newCheckedTool` でテスト用 runner・即時 `Sleep` 注入）と `internal/mergepr/mergepr_test.go` の `gh pr checks` モック（5 箇所）を更新し、`Sourcery review` が `FAILURE` でも `github-actions` が成功なら通過すること、`github-actions` 失敗・`head-changed`・`unreadable` で `errChecksFailed` を返すことを検証する（AC-19）。
  - [ ] 壊して確かめる検証: `Prepare` の `ExpectedHead` 引き渡しや `StatePassed` 判定を一時的に壊し、`mergepr_test.go` が失敗することを確かめ、コミットメッセージに記録する。
  - [ ] `.claude/commands/mergepr.md:47` の CI 待機記述（`gh pr checks --watch --fail-fast`）を `github-actions` チェック監視へ更新し、`docs/dev/guides/mergepr_guide.md`（4 章・5.2 節 `prepare`・6.1・6.2 節の `github-actions` 限定監視と `.github/workflows/deepseek-review.yml` の前提・7 章の古いバイナリ判別法と `make install-mergepr` 対処）を更新する（[02_architecture.md](02_architecture.md) 5.4・8 章）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode`（`cmd/mergepr/docs_test.go` を含む）がすべて通ること。
  - **マージ後の手動作業:** 利用者が `main` から `make install-mergepr` を実行する（`02_architecture.md` 8 章）。

---

### フェーズ 3: `fixprguard` の `Protocol` 引き上げと F-003 完了・F-004 実装

#### ステップ 3-1: `internal/fixprguard` の非破壊内部追加（`ReviewProfile`・`mentions`・`ci-checks`・`ResolveRepo` 委譲）と `CLAUDE.md` 例外追記

- **対象ファイル:**
  - `internal/fixprguard/github.go`
  - `internal/fixprguard/review_state.go`
  - `internal/fixprguard/mentions.go`（新規）
  - `internal/fixprguard/mentions_test.go`（新規）
  - `internal/fixprguard/ci_checks.go`（新規）
  - `internal/fixprguard/ci_checks_test.go`（新規）
  - `CLAUDE.md`
- **作業内容:**
  - [ ] `internal/fixprguard/github.go` の `resolveRepository` を `internal/ghci.ResolveRepo` への委譲へ整理し、`githubTimeout` と既存エラー変換（`ErrResponseRejected`、`types_test.go` の既存テスト互換）を維持する。`NewGitHubWithoutReviewer` の doc コメントへ `PRReviewState`・`PRMentions`・`CIChecks` を反映する。
  - [ ] `internal/fixprguard/review_state.go` へ `ReviewProfile` 型（`ProfileUnknown`・`ProfileCodeRabbit`・`ProfileSourcery`、`ParseReviewProfile`・`MarshalText`・`ReviewProfiles`）と共通の依頼コメント判定ヘルパー `isRequestComment` を追加する（既存の `ReviewStateRequest`・`PRReviewState` 契約と `Protocol` はこのステップでは変更しない）。
  - [ ] `internal/fixprguard/mentions.go` を新規作成し、`ObservedBot`・`ParseObservedBot`・`MentionScanRequest`・`MentionScanState`・`MentionSource`・`MentionScanStage`・`MentionFinding`・`MentionScanResult`・`(*GitHub).PRMentions` を実装する（[02_architecture.md](02_architecture.md) 3.2.3・5.3 の 3 エンドポイント厳格 JSON・`body: null` / `submitted_at: null` 許容・4 メンション検査・CodeRabbit 定型 2 種と Sourcery レビューガイド／7 日間予算超過コメント除外・`isRequestComment` 共有・本文非出力）。
  - [ ] `internal/fixprguard/ci_checks.go` を新規作成し、`CIChecksRequest`・`CIChecksResult`・`(*GitHub).CIChecks`（`ghci.ResolveRepo`＋`ghci.ReadPR` の薄いラッパー、`githubTimeout` 適用）を実装する（[02_architecture.md](02_architecture.md) 3.2.4）。
  - [ ] `internal/fixprguard/mentions_test.go` と `internal/fixprguard/ci_checks_test.go` を作成し、[02_architecture.md](02_architecture.md) 7.1・7.3 の全観点（4 メンション×3 エンドポイント×著者種別、観測済み定型除外と非定型・`User` なりすまし・他利用者名の予算超過コメントの非除外、手動依頼 3 種一致、厳格 JSON 拒否と `stage`、上限超過、禁止テキスト非出力マーカー、`ci-checks` の状態伝播と `--head` 不一致）を検証し、新規公開関数すべてがテストから到達可能であることを確認する（AC-05・AC-13・AC-19）。
  - [ ] 壊して確かめる検証: `mentions.go` の `user.type == "Bot"` 照合または定型 1 行目照合を一時的に緩めて `mentions_test.go` が失敗することを確かめ、コミットメッセージに記録する。
  - [ ] `CLAUDE.md` の `Declare, don't infer` の `fixprguard` 例外（`490-502` 行目）に `internal/fixprguard/mentions.go` の検査範囲と境界（1 行目の観測済み定型照合・依頼文面一致・4 メンション部分一致のみ、本文非出力）を追記する（AC-29 先行分）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` がすべて通ること（`Protocol` 未変更のため既存 CLI と文書ガードがすべて通る）。

#### ステップ 3-2: `internal/fixprguard` `review-state` 拡張・`cmd/fixprguard` 統合・`Protocol` の `main` の値 + 1 引き上げ

- **対象ファイル:**
  - `internal/fixprguard/types.go`
  - `internal/fixprguard/review_state.go`
  - `internal/fixprguard/test_helpers_review.go`
  - `internal/fixprguard/review_state_test.go`
  - `cmd/fixprguard/main.go`
  - `cmd/fixprguard/main_test.go`
- **作業内容:**
  - [ ] `internal/fixprguard/types.go` の `Protocol` を `origin/main` の値 + 1 へ引き上げる（`CLAUDE.md`「Protocol numbers」）。
  - [ ] `internal/fixprguard/review_state.go` を [02_architecture.md](02_architecture.md) 3.2.2 の仕様どおり拡張する（`RunSkipped`・`RequestSkipped`・`AutoReviewState`・`AutoReview`・`ReviewStateRequest` の `Profile` と複数 `Requests`・`PRReviewState` の `Profile` 常時設定と `AutoReview`・GraphQL `$checkName` と `name` 完全一致照合・プロファイル別分類表・`ProfileSourcery` の依頼 0 件時読み取り・15 分応答上限・同一／より古いコミット競合判定・4 段階優先順位 `AutoReview`・自動完了の印と `ReviewURL` 解決）。
  - [ ] `internal/fixprguard/test_helpers_review.go`（`//go:build test`）と `review_state_test.go` を拡張し、`ProfileCodeRabbit` 既存ケース維持＋`ProfileSourcery` の全状態・優先順位・未開始 `startedAt: null`・手動 `waiting` かつ `Response == nil` 時の未開始 CheckRun 非誤認・15 分応答上限・同一コミット競合と後続コミット非競合・空 `QUEUED` CheckSuite 無視・`name` 不一致 `rejected`・`Profile` 常時設定・禁止テキスト非出力を検証する（AC-05・AC-16〜AC-20・AC-24）。
  - [ ] `cmd/fixprguard/main.go` に `cmdMentions`・`cmdCIChecks`・`flagProfile`・`flagObservedBot`・`flagHead` を追加し、`flagRequest` と `flagObservedBot` を `multiFlags` に登録して `specs` とハンドラ（`reviewState`・`mentions`・`ciChecks`）を実装する（[02_architecture.md](02_architecture.md) 2.2・4.2）。
  - [ ] `cmd/fixprguard/main_test.go` の `TestReviewStateRun` を更新し、`mentions`・`ci-checks` の引数解析・終了コード `0`/`2`・`GH_REPO`/`GH_HOST` 拒否・JSON 出力を検証するテストを追加する。
  - [ ] 壊して確かめる検証: `review_state.go` の `name` 照合または `ProfileSourcery` の `commitIndex` 競合判定を一時的に壊して `review_state_test.go` が失敗することを確かめ、コミットメッセージに記録する。
- **完了条件:**
  - `go test -tags test ./internal/fixprguard` と `cmd/fixprguard` の CLI テスト（`docs_test.go` はステップ 3-3 の文書更新と合わせて通過）が通ること。

#### ステップ 3-3: `_context.md`・`runpr.md`・`fixpr.md`・文書ガード・ガイド・`CLAUDE.md` 例外の一括更新と旧 `jq` サブテスト削除

- **対象ファイル:**
  - `.claude/commands/_context.md`
  - `.claude/commands/runpr.md`
  - `.claude/commands/fixpr.md`
  - `docs/dev/guides/fixprguard_guide.md`
  - `CLAUDE.md`
  - `cmd/fixprguard/docs_test.go`
  - `cmd/yt2column/fixpr_docs_test.go`
  - `cmd/yt2column/review_mention_docs_test.go`
- **作業内容:**
  - [ ] `.claude/commands/_context.md` の Domain-specific 節から `Awaited PR reviewer` 行・`PR review request` 行および `Rejected mentions` 行の `<rejected-mentions>` 定義を削除し、`### PR reviewers` 表（5 列・`coderabbitai` と `sourcery-ai` の 2 行）を新設する（[02_architecture.md](02_architecture.md) 3.1、AC-12・AC-15）。
  - [ ] `.claude/commands/runpr.md` を [02_architecture.md](02_architecture.md) 3.3・4.2・6.1・6.2 の仕様どおり更新する（冒頭プレースホルダーと認可リスト更新、全 `fixprguard` 呼び出しの `--protocol` を `main` の値 + 1 へ更新、step 3 の `fixprguard mentions`・両社 `fixprguard review-state`・`TIMED_OUT_AUTO`／`FIRST_SEEN_AUTO`・`LAST_REVIEWED` 和集合・`fixprguard ci-checks --head`、step 4 の質問前再読込・要約最大 16 行・マージの質問表示条件と推奨・依頼先 3 択と `mergepr discard` 前の投稿前確認・`Manual PR review stops` 表への `skipped` 追加。レビュアー名と 4 メンションは直書きしない。AC-08・AC-13〜AC-25）。
  - [ ] `.claude/commands/fixpr.md` の全 `fixprguard` 呼び出しと説明の `--protocol` を `main` の値 + 1 へ更新する。
  - [ ] `docs/dev/guides/fixprguard_guide.md` の 1 章・4 章（新 `--protocol`・`--profile`・`--observed-bot`・`--request` 複数回・`--head`・`mentions`/`ci-checks` の終了コード `0`）・5 章（`review-state` 拡張、`mentions`・`ci-checks` 節新設、全 `"protocol"` 例更新）・7.1・7.2・10 章を更新する（AC-26）。
  - [ ] `CLAUDE.md` の `Declare, don't infer` の `fixprguard` 例外（`490-502` 行目）へ `review_state.go` の CheckRun `name` 照合とプロファイル別分類表を反映する（AC-29）。
  - [ ] `cmd/fixprguard/docs_test.go` を更新し、`TestDocumentedInvocationsParse` の `runpr.md` 期待サブコマンドへ `cmdMentions`・`cmdCIChecks` を追加、`checkDocumentedReviewer` と `TestCheckDocumentedReviewer` を新プレースホルダー（`<pr-profile>`・`<pr-reviewer>`・`<accepted-request>`・`<observed-bot>`）へ対応させ、`TestGuideListsReviewStateValues`（`ReviewProfile`・`AutoReviewState`・`RunSkipped`・`RequestSkipped` 追加）および `mentions`・`ci-checks` のガイド値表検査（`TestGuideListsMentionsValues`・`TestGuideListsCIChecksValues`）を追加する（AC-08・AC-15・AC-26）。
  - [ ] `cmd/yt2column/fixpr_docs_test.go` と `cmd/yt2column/review_mention_docs_test.go` を更新する（`checkContextReviewers` と `TestContextReviewerRowsMalformed` の `### PR reviewers` 5 列表・旧行 0 件検査、`TestContextAndCommandsAgree` の全 `Reviewer` 直書き禁止、`TestFixprguardCallSites` への `mentions`・`ci-checks` 追加、`TestRunprDocContract` への旧 `gh api --paginate`・`gh pr checks --watch` 不在検査追加、`TestMentionsOnlyInAllowedRows` の `### PR reviewers` 許可化、`TestPostingPathsCheckMention` から `<rejected-mentions>` 要求削除、`TestPostingCommandsInventory` の `<post-request>` 更新。AC-08・AC-09・AC-12・AC-15・AC-25）。
  - [ ] **単独テスト削除コミット（`CLAUDE.md`「Deleting a test is a claim that must be checked」準拠）:** 新しい契約テストと `internal/fixprguard/mentions_test.go` が揃った状態の後に、旧 `--jq` フィルタを対象としていた以下を単独コミットで削除し、`make coverage-compare BASE=<親コミット>`（10 分以上の制限時間）を実行して `coverage-compare: PASS` の出力をコミットメッセージと PR 説明に記録する（`runJQ`・`gojqStrictPrelude` は `TestFixprPhase6CountFilter` 用に残す）：
    - [ ] `cmd/yt2column/fixpr_docs_test.go` から `TestRunprDocContract/listings_print_no_body` サブテストを削除する
    - [ ] `cmd/yt2column/fixpr_docs_test.go` から `TestRunprDocContract/request_comment_agrees_with_review_state` サブテストを削除する
    - [ ] 上記 2 サブテスト専用で不要になった未公開ヘルパー（`runprListingFixtures`・`checkListings`・`checkRequestAgreementWithReviewState`・`checkListingConstructors`・`jqObjectBody`・`splitTopLevel`・`skipJQString`・`checkListingOutput` 等、`TestFixprPhase6CountFilter` が使わないもの）を削除する
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` および `make coverage-compare` がすべて通ること。
  - **PR レビュー・マージ前後の手動作業:** 利用者が PR の `/runpr` 前にこの PR のブランチから `make install-fixprguard` を実行し、マージ直後に `main` から `make install-fixprguard` を再実行する（`fixprguard_guide.md` 3 章・`CLAUDE.md`「Protocol numbers」）。

---

### フェーズ 4: 仕様文書・ADR・`chief` 確認の仕上げ

#### ステップ 4-1: `CLAUDE.md` と `docs/translation_glossary.md` の更新

- **対象ファイル:**
  - `CLAUDE.md`
  - `docs/translation_glossary.md`
- **作業内容:**
  - [ ] `CLAUDE.md` の `178-179` 行目（`PR review request` 行参照を `_context.md` の `PR reviewers` 表へ更新）、`215,249-261` 行目（`/runpr` と PR レビュー体制を CodeRabbit 手動＋Sourcery AI 自動・手動の併用へ更新）、`831-835` 行目（`jq` が必要なテスト記述から `TestRunprDocContract`・`TestHandoffListingPrintsNoBody` を除き `TestFixprPhase6CountFilter` に合わせる）を更新する（AC-29）。
  - [ ] `docs/translation_glossary.md` の `完了を待つ PR レビュアー`・`手動の PR レビュー`・`完了の印` の定義を CodeRabbit＋Sourcery AI 併用と `### PR reviewers` 表に合わせて更新し、`自動の PR レビュー`（`automatic PR review`）を追加する（AC-30）。
- **完了条件:**
  - 変更した記述が実装後の `_context.md`・`runpr.md`・`fixprguard` と一致していること。

#### ステップ 4-2: ADR-0013・ADR-0015・ADR-0047 の `#1095` 食い違い記述の削除

- **対象ファイル:**
  - `docs/dev/adr/0013-review-setup.md`
  - `docs/dev/adr/0015-untrusted-review-threads.md`
  - `docs/dev/adr/0047-runpr-merge-question.md`
- **作業内容:**
  - [ ] `docs/dev/adr/0013-review-setup.md`（`27,55,76,89` 行目の `#1095` 食い違い記述を削除。`55` 行目の `#1072` 食い違いは別タスクのため残す）を更新する（AC-27）。
  - [ ] `docs/dev/adr/0015-untrusted-review-threads.md`（`30,43,56` 行目の `#1095` 食い違い記述を削除）を更新する（AC-27）。
  - [ ] `docs/dev/adr/0047-runpr-merge-question.md`（`21` 行目の `#1095` 食い違い記述を削除。`#1072` 食い違いは別タスクのため残す）を更新する（AC-27）。
  - [ ] 上記 ADR 3 件の編集は実装完了に伴う食い違い記述の削除（決定の変更なし）のため、`docs/dev/conventions/adr.md`「Editorial corrections」に従いコミットメッセージへ `Editorial correction:` 行を記録する。
- **完了条件:**
  - `go test -tags test ./cmd/yt2column -run TestADRRecords` が通り、3 ADR に `#1095` への食い違い記述が残っていないこと。

#### ステップ 4-3: `internal/chief` の単一選択中継テスト確認と全体検証

- **対象ファイル:**
  - `internal/chief/screen_test.go`
- **作業内容:**
  - [ ] `internal/chief/screen_test.go` に、依頼先選択の 3 択の問い（質問文 `PR #<number> の PR レビューをどのレビュアーに依頼しますか`、選択肢 `CodeRabbit`・`Sourcery AI`・`マージの質問に戻る`）が `Merge: false`・`QuestionSingle`・`RelayAllowed` と判定され `relayable` となることを確かめる単体テスト `TestPickerQuestionRelayable` を追加する（AC-28）。
  - [ ] 既存の `internal/chief` 全テスト、およびリポジトリ全体の `make test`・`make lint`・`make deadcode`・CI の `doc-guard` と同一コマンド（`go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`）を実行してすべて成功することを確認する（AC-28・AC-31）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` と `doc-guard` 相当コマンドがすべて成功すること。

---

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

1. **M1（フェーズ 1 完了）:** `sourcery-ai` のスレッド自動処理と 4 メンション拒否（`RejectedMentions()`・`_context.md`・`fixprguard_guide.md`・文書ガード）が `Protocol` 据え置きで有効になる。利用者が `main` から `make install-fixprguard` を実行する。
2. **M2（フェーズ 2 完了）:** 共有パッケージ `internal/ghci` が整備され、`mergepr prepare` が `app.slug == "github-actions"` のみ監視する形へ切り替わる。利用者が `main` から `make install-mergepr` を実行する。
3. **M3（フェーズ 3 完了）:** `fixprguard` の `Protocol` が `main` の値 + 1 へ上がり、`review-state` 拡張・`mentions`・`ci-checks`・`_context.md` `### PR reviewers` 表・`runpr.md` 複数レビュアー運用・ガイド・文書ガードがすべて整合する。利用者が `main` から `make install-fixprguard` を再実行する。
4. **M4（フェーズ 4 完了）:** `CLAUDE.md`・用語集・ADR 3 件・`internal/chief` テストが最終状態と一致し、全 AC（AC-01〜AC-31）と CI が通過する。

---

## 4. テスト戦略 (Test Strategy)

詳細なテストケース設計は [02_architecture.md](02_architecture.md) 7 章に従う。

- **単体テスト:**
  - `internal/fixprguard`: 著者判定（`trust_test.go`・`github_test.go`・`write_test.go`）、4 メンション拒否（`write_test.go`・`worktree_flow_test.go`）、`review-state` 複数プロファイルと自動レビュー状態機械（`review_state_test.go`）、厳格メンション検査（`mentions_test.go`）、`ci-checks` ラッパー（`ci_checks_test.go`）。
  - `internal/ghci`: `ResolveRepo`・`ReadPR`・`WaitPR` の全状態・再試行・上限・外部 App 無視・厳格 JSON 拒否（`ghci_test.go`）。
  - `internal/mergepr`: `Prepare` の `ghci` 連携と `Sourcery review` 失敗時の非停止（`mergepr_test.go`）。
  - `cmd/fixprguard`: 新サブコマンドとフラグ解析・終了コード・環境変数拒否（`main_test.go`）。
  - `internal/chief`: 依頼先 3 択の単一選択中継判定（`screen_test.go`・`relay_test.go`）。
- **文書ガード・契約テスト:**
  - `cmd/yt2column/fixpr_docs_test.go`・`review_mention_docs_test.go`・`adr_test.go`、`cmd/fixprguard/docs_test.go`、`cmd/mergepr/docs_test.go`、`internal/pipeline/pipeline_test.go` を各フェーズのグリーンゲートで実行する。
- **セキュリティテスト:**
  - コメント本文・CheckRun `title`・`details_url`・`output.summary` に固有マーカーを埋め込み、標準出力・標準エラーへの非出力を機械的に検証する。
  - なりすまし `User`・`user: null`・大文字小文字違い・重複 JSON キー・不正 UTF-8・単独サロゲート・後続ゴミデータ・サイズ上限超過の拒否を検証する。
  - `fixprguard_guide.md` 7.1 の保証と `mergepr_guide.md` 6.1 の保証を変える箇所は、実装を一時的に壊してテストが失敗することを確認し、コミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。
- **テスト削除の検証:**
  - ステップ 3-3 で削除する旧 `--jq` サブテスト 2 件と専用ヘルパーは単独コミットに分離し、`make coverage-compare BASE=<親コミット>` の `PASS` 出力をコミットメッセージと PR 説明に残す。
- **テストヘルパー構成（`docs/dev/conventions/test_organization.md` 準拠）:**
  - クロス・パッケージの fake は既存の `internal/cmdrunner/testutil` を再利用し、新規 `testutil/` ファイルは追加しない（`TestFakesCarryBuildTag` の 22 ファイル固定を維持）。
  - パッケージ内部ヘルパーは既存の `internal/fixprguard/test_helpers_review.go`・`internal/mergepr/test_helpers.go`・`cmd/fixprguard/test_helpers.go`（いずれも `//go:build test`）を拡張して再利用する。`internal/ghci` はまず `ghci_test.go` 内に収め、分離が必要になった場合のみ `internal/ghci/test_helpers.go`（`//go:build test`）を設ける。

---

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 緩和策 |
|---|---|---|
| フェーズ 1 マージ後・フェーズ 3 マージ前の移行期に、旧 `--jq` 経路が `sourcery-ai[bot]` の定型コメントを `by_bot` 確認へ回す | PR ごと・`/runpr` 実行ごとに 1 回 Bot 確認が出る | 安全側の挙動であり運用継続に支障なし。フェーズ 3 マージで解消する（[02_architecture.md](02_architecture.md) 8 章） |
| フェーズ 2 マージ後に `mergepr` を再インストールし忘れる（`Protocol = 3` 据え置きのため自動検出されない） | `Sourcery review` の失敗・未完了で `mergepr prepare` が止まる／待ち続ける | マージ後の手動作業として計画と PR 説明に明記し、`mergepr_guide.md` 7 章に `go version -m` による判別法と対処を記載する |
| フェーズ 3 の `fixprguard` `Protocol` 引き上げ時に並行ブランチが同じ番号を使う | マージ前の `Protocol` 衝突またはツール不一致 | `CLAUDE.md`「Protocol numbers」に従い、PR 作成時に `origin/main` の値 + 1 を設定し、マージ時は `mergepr` の自動検査で確認する |
| ステップ 3-3 の旧 `--jq` サブテスト削除時に、意図せずプロダクトコードのカバレッジが下がる | `make coverage-compare` が `FAIL` になる | 先に `mentions_test.go`・`review_state_test.go` で同等以上の経路をカバーしたうえで削除コミットを単独作成し、`make coverage-compare` を実行する |
| 本タスク自身の PR で Sourcery AI が PR 説明末尾に `@sourcery-ai` を引用して追記し、`append-pr` が `current-body-mention` で止まる | `/fixpr` Phase 6 の追記が自動完了しない | 安全側の停止として `fixprguard_guide.md` 8 章どおり `append-pr` を繰り返さず、利用者が PR 説明を GitHub 上で確認して対処する（[02_architecture.md](02_architecture.md) 5.4） |

---

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（ステップ 1-1・1-2）
- [ ] フェーズ 2 完了（ステップ 2-1・2-2）
- [ ] フェーズ 3 完了（ステップ 3-1・3-2・3-3）
- [ ] フェーズ 4 完了（ステップ 4-1・4-2・4-3）

---

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

各 AC と実装ステップ・検証手段の対応を以下に示す。種別は `test`（実行可能な動作テスト）、`static`（ガードテスト・`make` ターゲット）、`manual`（自然言語文書・手順プロンプトの PR レビュー確認）とする。

| AC | 対象ステップ | 種別 | 検証場所・コマンド | 検証内容 |
|---|---|---|---|---|
| AC-01 | 設計前完了（PR #1102＋本 PR 追記） | `manual` | `docs/tasks/0122_sourcery_trusted_reviewer/observations.md` | 10 項目＋CI 補足・#1097/#1098 再レビュー範囲の観測記録と出典の目視確認（PR #1102 承認済み） |
| AC-02 | 設計前完了（PR #1102） | `manual` | `docs/tasks/0122_sourcery_trusted_reviewer/observations.md` | 手動依頼投稿者が利用者 `isseis` である記録の目視確認（PR #1102 承認済み） |
| AC-03 | 設計前完了（PR #1102） | `manual` | `docs/tasks/0122_sourcery_trusted_reviewer/observations.md` | `02_architecture.md` と同一 PR（#1102）でコミット済み・試験 PR 未作成の確認 |
| AC-04 | 設計前完了（PR #1102＋本 PR 確認） | `manual` | `docs/tasks/0122_sourcery_trusted_reviewer/observations.md` | 前提との照合・項目 8 ruleset 確認・#1097/#1098 再レビュー範囲確認の利用者合意確認 |
| AC-05 | 3-1 / 3-2 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateObserved`, `internal/fixprguard/mentions_test.go::TestPRMentionsObservedExclusions` | 分類表・定型除外ヘッダーの実装値が `observations.md` の観測値と一致することを確認 |
| AC-06 | 1-2 | `test` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `TestContextReviewerRowsMalformed` | `_context.md` の `Automated reviewer list` が `[bot]` なしでちょうど `coderabbitai` と `sourcery-ai` の 2 つであることを検証 |
| AC-07 | 1-1 | `test` | `internal/fixprguard/trust_test.go::TestAssessAuthors`, `internal/fixprguard/github_test.go::TestBeginSourceryAuthorType`, `internal/fixprguard/write_test.go::TestReplyTrustsEveryReviewer`, `cmd/fixprguard/main_test.go::TestSecondReviewerBeginReply` | `Bot`/`sourcery-ai` の無照会信頼・`authorType: "Bot"`・混在スレッド・大文字小文字違いや `User`/`sourcery-ai` の非自動レビュアー扱いを検証 |
| AC-08 | 1-2 / 3-3 | `static` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`, `TestCheckDocumentedReviewer` | コマンド文書に `coderabbitai`・`sourcery-ai`（`[bot]` 含む）が直書きされずプレースホルダー経由で渡されることを検証 |
| AC-09 | 1-2 / 3-3 | `test`, `static` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `TestContextReviewerRowsMalformed`, `make test` | 文書ガードが 2 社を期待値として照合し、`make test` が成功することを検証 |
| AC-10 | 1-1 / 1-2 | `test` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `TestContextReviewerRowsMalformed`, `cmd/yt2column/review_mention_docs_test.go::TestMentionsOnlyInAllowedRows` | `_context.md` の `Rejected mentions` と `fixprguard.RejectedMentions()` が 4 メンションで一致することを検証 |
| AC-11 | 1-1 | `test` | `internal/fixprguard/write_test.go::TestCheckPostText`, `TestReplyRejectsMention`, `TestEditPRRejectsMention`, `TestAppendPRRejects`, `internal/fixprguard/worktree_flow_test.go::TestFinishRejectsBodyFile`, `cmd/fixprguard/main_test.go::TestPostTextMentionExitsUsage` | 書き込み系 4 サブコマンドが 4 メンションを ASCII 大文字小文字無視・部分文字列で拒否し、`append-pr` が既存本文で `current-body-mention` 停止することを検証 |
| AC-12 | 1-2 / 3-3 | `static` | `cmd/yt2column/review_mention_docs_test.go::TestMentionsOnlyInAllowedRows`, `TestPostingPathsCheckMention`, `TestPostingCommandsInventory` | `<mention-check>` の 4 メンション拒否と、許可箇所以外への 4 メンション記載禁止を検証 |
| AC-13 | 3-1 / 3-2 / 3-3 | `test`, `static` | `internal/fixprguard/mentions_test.go::TestPRMentions`, `TestPRMentionsObservedExclusions`, `TestPRMentionsErrors`, `cmd/fixprguard/main_test.go::TestMentionsRun`, `cmd/yt2column/fixpr_docs_test.go::TestFixprguardCallSites`, `TestRunprDocContract` | 4 メンション×3 経路の著者別扱い・定型 4 種除外・なりすまし拒否・本文非出力・`runpr.md` からの `fixprguard mentions` 呼び出しを検証 |
| AC-14 | 3-3（既存維持） | `static`, `test` | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（`head_before_checkout`）, `TestFixprguardCallSites`, `cmd/fixprguard/main_test.go::TestInteractionLimitsRun` | `runpr.md` step 1 で `head` → `interaction-limits` → `gh pr checkout` の順序と停止判定が維持されることを検証 |
| AC-15 | 3-3 | `test` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `TestContextReviewerRowsMalformed`, `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`, `TestCheckDocumentedReviewer` | `_context.md` の `### PR reviewers` 表の 5 列・2 行・依頼文面定義とコマンド文書の直書き禁止を検証 |
| AC-16 | 3-2 / 3-3 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateSourceryAuto`, `cmd/fixprguard/main_test.go::TestReviewStateRun`（`runpr.md` の `TIMED_OUT_AUTO`/`FIRST_SEEN_AUTO` 手順は `manual`） | `head` 上の進行中自動 CheckRun の `AutoRunning`・未開始 `startedAt: ""`・15 分超過 `AutoTimedOut`・祖先未完了非待機を検証し、step 4 非再待機手順をレビューで確認 |
| AC-17 | 3-2 | `test` | `internal/fixprguard/review_state_test.go::TestReviewState`, `TestReviewStateSourceryAuto`, `TestReviewStateSourceryManual`, `TestReviewStateObserved`, `TestReviewStateNonPRCommitIgnored`, `TestReviewStateOutputHasNoText` | CodeRabbit（手動）と Sourcery AI（自動・手動、`@sourcery-ai review`/`@sourceryai review`）の完了の印判定、`name`/`app.slug`/非 PR コミット・本文非出力を検証 |
| AC-18 | 3-2 / 3-3 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateSourceryAuto`, `TestReviewStateSourceryManual`, `TestReviewStateLimits`（`runpr.md` の両社和集合・祖先算出手順は `manual`） | 両社の `markers` 出力と `unreadable (limit)` 時の動作を検証し、`runpr.md` の `LAST_REVIEWED` 和集合規則をレビューで確認 |
| AC-19 | 2-1 / 2-2 / 3-1 / 3-2 / 3-3 | `test`, `static`, `manual` | `internal/ghci/ghci_test.go::TestReadPR`, `TestWaitPR`, `internal/fixprguard/ci_checks_test.go::TestCIChecks`, `internal/mergepr/mergepr_test.go::TestPrepare`, `TestPrepareRejectsFailedChecks`, `internal/fixprguard/review_state_test.go::TestReviewStateSourceryAuto`, `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract` | 自動レビュー異常時の `AutoSkipped`/`AutoUnrecognized`/`AutoTimedOut`、`internal/ghci`・`ci-checks`・`mergepr prepare` の `github-actions` 限定監視、`runpr.md` からの `gh pr checks --watch` 廃止を検証 |
| AC-20 | 3-1 / 3-2 / 3-3 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateSourceryManual`, `TestReviewStateUnreadable`, `TestReviewStateLimits`, `internal/fixprguard/mentions_test.go::TestPRMentionsErrors`, `cmd/fixprguard/main_test.go::TestReviewStateRun` | 手動依頼の `skipped`/`rate-limited`/`unrecognized`/`timed-out`・`indeterminate`・`unreadable` 各理由を検証し、`runpr.md` の停止・再試行・同時待機手順をレビューで確認 |
| AC-21 | 3-3 | `manual` | `.claude/commands/runpr.md` step 4（PR レビュー確認） | マージの質問の最大 4 択・単一選択維持と、`head` 未レビュー残存時の「先に PR レビューを依頼」表示条件を確認（手順文書の自然言語仕様） |
| AC-22 | 3-3 | `manual` | `.claude/commands/runpr.md` step 4（PR レビュー確認） | `LAST_REVIEWED == head` 時の「未レビューの変更: なし」・推奨「マージ」および未レビュー時のエスカレーション推奨規則を確認（手順文書の自然言語仕様） |
| AC-23 | 3-3 / 4-3 | `static`, `test`, `manual` | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory`, `internal/chief/screen_test.go::TestPickerQuestionRelayable` | `gh pr comment <number> --body "<post-request>"` が `runpr.md` step 4 に 1 か所のみであること・3 択質問の中継可能性を検証し、投稿前確認・重複投稿抑止手順をレビューで確認 |
| AC-24 | 3-2 / 3-3 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateOutputHasNoText`（`runpr.md` 要約 16 行・表示優先順位は `manual`） | 本文・`title`・`details_url` 非出力を検証し、`runpr.md` の CodeRabbit・Sourcery AI 各 1 行要約と URL 優先順位をレビューで確認 |
| AC-25 | 3-3 | `static` | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory` | PR 作成コマンド群に自動レビュアーへの依頼コメント投稿コマンドが存在しないことを検証 |
| AC-26 | 1-2 / 3-3 | `test`, `manual` | `cmd/fixprguard/docs_test.go::TestGuideCoversStopKindsAndScope`, `TestGuideDescribesMentionCheck`, `TestGuideDescribesReviewerList`, `TestGuideListsReviewStateValues`, `TestGuideListsMentionsValues`, `TestGuideListsCIChecksValues`, `TestDocumentedProtocolMatches` | `docs/dev/guides/fixprguard_guide.md` の値表・コマンド文法・4 メンション・`--protocol` 一致を検証し、文章記述をレビューで確認 |
| AC-27 | 4-2 | `test`, `manual` | `cmd/yt2column/adr_test.go::TestADRRecords` | ADR 構造・索引整合を検証し、ADR-0013・0015・0047 の `## 決定` 一致と `#1095` 食い違い削除をレビューで確認 |
| AC-28 | 4-3 | `test` | `internal/chief/screen_test.go::TestPickerQuestionRelayable`, `go test -tags test ./internal/chief` | 依頼先選択 3 択の中継判定および既存 `internal/chief` 全テスト通過を検証 |
| AC-29 | 3-1 / 3-3 / 4-1 | `manual` | `CLAUDE.md`（PR レビュー体制・`Declare, don't infer` 例外・`jq` 記述） | `CLAUDE.md` の該当箇所が実装と一致していることを PR レビューで確認（自然言語仕様） |
| AC-30 | 4-1 | `manual` | `docs/translation_glossary.md` | 用語集の該当定義が複数レビュアー併用と自動の PR レビューへ更新されていることを PR レビューで確認（自然言語用語集） |
| AC-31 | 全ステップ / 4-3 | `static` | `make test`, `make lint`, `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief` | 全単体テスト・リンター・WASM ビルド検査・`doc-guard` ジョブ相当の通過を検証 |

### 7.1. 横断検索チェックリスト（`make lint`・`make test` で検出できない文書整合）

- [ ] ステップ 3-3・4-1・4-2 の完了時点で、仕様文書（`docs/dev/`・`CLAUDE.md`・`.claude/commands/`・`docs/translation_glossary.md`。過去の `docs/tasks/` と `internal/chief/testdata/` を除く）に廃止済みの `Awaited PR reviewer`・`PR review request`・`<awaited-reviewer>`・`<review-request>`・`<rejected-mentions>`・`cr_observed`・`gh pr checks --watch`（`mergepr`・`runpr` の CI 待機記述）の古い記述が残っていないことを PR レビューで目視確認する（`manual`）。
- [ ] ステップ 4-2 の完了時点で、ADR-0013・ADR-0015・ADR-0047 に `#1095` の未実装食い違い記述が残っていないこと、かつ別タスク（`#1072`）の食い違い記述を誤って消していないことを PR レビューで目視確認する（`manual`）。

---

## 8. 成功基準 (Success Criteria)

- **機能完全性:** F-001〜F-005 の全 31 件の受け入れ基準（AC-01〜AC-31）が実装・検証済みであること。
- **品質:** すべての実装 PR で `make fmt`・`make test`・`make lint`・`make deadcode`・CI `doc-guard` が通り、テスト削除コミットで `make coverage-compare` が `PASS` していること。
- **セキュリティ:** 本文・`title`・`details_url`・`output.summary` の非出力、4 メンション拒否、厳格 JSON 検証（要件定義書 3.2 項目 4 の `jq` 例外解消を含む）、直書き禁止ガードがテストで実証され、保証変更箇所の壊して確かめる検証がコミットメッセージに記録されていること。
- **文書整合:** `_context.md`・`runpr.md`・`fixpr.md`・`mergepr.md`・`fixprguard_guide.md`・`mergepr_guide.md`・`package_reference.md`・`CLAUDE.md`・`docs/translation_glossary.md`・ADR-0013・ADR-0015・ADR-0047 がすべて実装と一致していること。

---

## 9. 次のステップ (Next Steps)

1. 本計画書（`03_implementation_plan.md`）の PR レビューを受け、`/runpr` で承認（`approved`）とマージを行う。
2. マージ後、`/runplan 0122` で PR-1 から順に実装・検証を進める。
3. 各開発ツールの更新 PR マージ後に、利用者が手順どおり `make install-fixprguard`（PR-1・PR-5 前後）および `make install-mergepr`（PR-3 後）を実行する。
