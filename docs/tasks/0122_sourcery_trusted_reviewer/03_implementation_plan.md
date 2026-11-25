# 実装計画書：Sourcery AI を自動レビュアーの一覧と PR レビュー体制に加える

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-24 |
| Review date | 2026-11-25 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的と実装方針

本書は、[01_requirements.md](01_requirements.md)（F-001〜F-005、AC-01〜AC-31）および [02_architecture.md](02_architecture.md) に基づき、Sourcery AI（`sourcery-ai`）を自動レビュアーの一覧と PR レビュー体制へ組み込むための実装手順と検証計画を定める。設計の詳細（型定義・CheckRun 分類表・フロー・境界チェックリスト）は [02_architecture.md](02_architecture.md) を参照し、本書では重複記述を避ける。

- **実装フェーズの原則:** [02_architecture.md](02_architecture.md) 8 章の優先順位（フェーズ 1〜4）に従う。`fixprguard` の `Protocol` 番号（`internal/fixprguard/types.go:21`、コミット `d29bb3d7` 時点で `8`）はフェーズ 3 の契約変更時にのみ `main` の値 + 1 へ引き上げ、コマンド文書・ガイド・テスト・`CLAUDE.md` 例外を同一 PR で整合させる（`CLAUDE.md`「Protocol numbers」）。`mergepr` の `Protocol`（`internal/mergepr/mergepr.go:63`、コミット `d29bb3d7` 時点で `3`）は CLI 契約が不変のため据え置く。
- **既存基盤の再利用:** `internal/cmdrunner`・`internal/cmdrunner/testutil`・`internal/strictjson`、`internal/fixprguard/review_state.go` の `*reviewReader` メソッド群（`restGet`・`restPages`・`readHead`・`readUser`）および `parseCommitsPage` を再利用し、リポジトリ解決と GitHub Actions CI チェック読み取りを共有パッケージ `internal/ghci` へ集約する。
- **Go ソースの英語規約:** 新設・変更する Go ソース（テストおよび `test_helpers*.go` を含む）のコメント・識別子・文字列リテラルは英語とする（`CLAUDE.md`「Go Idioms」のガードテスト用リテラル例外に該当する日本語文書の見出し・ラベル照合のみ除く）。
- **開発ツールの再インストール:** 開発ツール（`fixprguard`・`mergepr`）のインストールは利用者が行い、エージェントは実行しない（`CLAUDE.md`「Tool Execution Safety」）。

### 1.2. 計画の際に決めたこと（2026-11-24）

- **Sourcery AI の再レビューが見る範囲（`02_architecture.md` 5.4 の未確認の前提）:**
  - **決定:** 現行設計（AC-17・AC-18・`02_architecture.md` 3.2.2）および [ADR-0013](../../dev/adr/0013-review-setup.md) を変更せず、そのまま実装計画を作成する。PR #1097・#1098 の確認結果を本節と [observations.md](observations.md) に記録する。
  - **理由:** 計画前に PR #1097 と PR #1098 を読み取り専用 `gh api` で確認した。初回コミットが 7 日間予算超過で `SKIPPED` になった後の後続コミット（#1097 `516b7e5b` は `03_implementation_plan.md` の +1/-1 行のみ、#1098 `3de3c7bd` は `01_requirements.md` の +7/-3 行のみ）で `SUCCESS` で完了したレビューは、いずれも直近コミットで触れていない先行コミットの変更箇所に指摘を付けていた（#1097 `#discussion_r4665768633` の `docs/dev/adr/0050-fable-partial-review.md` 90 行目、#1098 `#discussion_r4666409559` ほか 3 件の ADR-0013・0015・0047）。この実測は、初回 `SKIPPED` 後の `SUCCESS` 完了が直近コミット単体の増分差分ではなく先行コミットの変更箇所も対象にしていることを示す。一方で、2 件の実測だけでは先行コミットの全ファイルが必ず網羅される公式仕様の保証までは得られないが、[02_architecture.md](02_architecture.md) 5.4 が定めた確認手順の結果を踏まえ、`LAST_REVIEWED` で祖先コミットをレビュー済みと数える現行設計を運用上の前提として維持する判断を利用者と合意した。
  - **ADR の判断:** ADR なし（現行 [ADR-0013](../../dev/adr/0013-review-setup.md) の前提を運用判断として維持し、決定の変更がないため）。
  - **その後の変更:** 2026-11-25 に、Sourcery AI の自動の PR レビューの完了は完了の印に数えないことにした（1.5）。この前提が効くのは、手動依頼への応答として数える完了だけになった。
- **フェーズ 2・フェーズ 3 の PR 分割（6 PR 構成）:**
  - **決定:** [02_architecture.md](02_architecture.md) 8 章の 4 フェーズの順序を保ったまま、フェーズ 2 を 2 PR（`internal/ghci` 新設と `internal/mergepr` 切り替え）、フェーズ 3 を 2 PR（`internal/fixprguard` の非破壊内部追加と、旧 `--jq` テスト単独削除・`review-state` 拡張・`cmd/fixprguard`・`Protocol` 引き上げ・コマンド文書・ガイド 3〜5 章・ガード一括更新）に分け、全体 6 PR とする。`docs/dev/guides/fixprguard_guide.md` のうち `cmd/fixprguard/docs_test.go` と結合しない章（1 章・7.2.4・7.2.6・10 章の残余リスク記述）は PR-6（ステップ 4-1）へ寄せて PR-5 の変更量を抑える。
  - **理由:** 新規共有パッケージ `internal/ghci` と `fixprguard` の新規ロジック（`mentions`・`ci-checks`）を独立 PR で深くレビューでき、`Protocol` 引き上げと `runpr.md` 大幅改定を伴う PR-5 の変更量とレビュー負荷を抑えられるためである。なお `review_state.go` の拡張（`ReviewStateRequest` 変更と `RunSkipped`/`RequestSkipped` 追加）は `cmd/fixprguard`・`TestGuideListsReviewStateValues`・`TestDocumentedInvocationsParse` および単一 `Protocol` 引き上げ制約（[02_architecture.md](02_architecture.md) 1.3・8 章）と結合しており、スタブなしでは PR-4 へ切り出せないため PR-5 へまとめる。`Makefile:364`（`d29bb3d7`）の `deadcode` は `-test` 付きで実行されるため、単体テストを伴う内部追加 PR（PR-2・PR-4）も単独でグリーンゲート（`_context.md` の "Green gate" 参照）を通過できる。
  - **ADR の判断:** ADR なし（実装の分割と順序はこのタスク内に閉じるため）。

### 1.3. 既存コード調査結果

`origin/main` コミット `d29bb3d7` で対象コードを調査した結果をまとめる。

- **`internal/fixprguard/`（既存と変更点）:**
  - `errors.go:79-81`: `RejectedMentions()` が `[]string{"@codex", "@coderabbitai"}` を返す。`write.go:91`（`checkPostText`）がそのまま走査するため、`reply`・`edit-pr`・`append-pr`・`finish` に自動反映される。
  - `types.go:21,89-105`: `const Protocol = 8`。`ParseReviewers` と `trust.go` はすでに複数 `--reviewer` に対応しており、AC-07 の著者判定ロジック自体は変更不要（既存テストを再利用し、未検証の差分ケースのみ `trust_test.go`・`cmd/fixprguard/main_test.go` へ追加する）。
  - `github.go:50-60,99-124`: `resolveRepository` が `gh repo view --json nameWithOwner` を厳格解析している（`github.go:344,495,547`、`limits.go:195`、`write.go:183`、`review_state.go:399` の 6 か所から呼ばれる）。フェーズ 3 で `internal/ghci.ResolveRepo` へ委譲し、`githubTimeout`（`github.go:22`）と既存呼び出し元の `ErrResponseRejected` 変換を維持する（`review_state.go:399-402` の `UnreadableCall` 変換も維持する一方、`mentions.go`・`ci_checks.go` は `ghci.ResolveRepo` を直接呼んで `*ghci.RepoError` から `call-failed`・`rejected`・`limit` を分類する）。`NewGitHubWithoutReviewer` の doc コメントへ `PRReviewState`・`PRMentions`・`CIChecks` を追記する。
  - `review_state.go:44-50,334-348,417-421,437-524,589,655-679,1052-1190`: CheckRun 名 `"CodeRabbit"` と分類表が固定で、依頼コメント 0 件時に早期リターンしている。`isRequestComment` 判定（`589` 行目）を抽出すれば `mentions.go` と完全に共有できる。`restGet`・`restPages`・`readHead`・`readUser` は `*reviewReader` のメソッド（`437,472,497,524` 行目）で `gh api --include` と IMF-fixdate の `Date` ヘッダーを要求するため、`mentions.go` では `*reviewReader` 経由（または共通 REST ヘルパー抽出）で再利用し、`userMember`・`timeMember`・`optionalTimeMember`・`nullableStringMember` も共有する。
  - `construction_test.go:17-41`: `Login`・`PRNumber`・`Baseline`・`Reviewers` の直接構築を AST で制限している。新設する `ObservedBot` は `Profile ReviewProfile` と `Reviewer Login` を公開フィールドで持つため、`Login` は既存の構築制限が効く一方、呼び出し元がゼロ値や不正値の `ObservedBot` を直接構築しうる。`ReviewProfile` は `int` 列挙型でゼロ値 `ProfileUnknown` を不正（`MarshalText` がエラー、入口で `ErrInvalidInput` 拒否、全 `switch` の `default` が安全側）とし、`PRMentions` 入口の検証と単体テストでゼロ値・不正値・重複 `ObservedBot` 拒否を担保する。
  - `test_helpers_review.go:1-408`（`//go:build test`）: 既存の `rsInput`・`rsRun`・`rsCommit` を拡張し、`Profile`・複数依頼・Sourcery CheckRun ヘルパーを追加して再利用する。
- **`cmd/fixprguard/`（既存と変更点）:**
  - `main.go:57-141,205,495-518`: `specs` と `multiFlags`（現在 `flagAccept`・`flagThread`・`flagReviewer`）で全サブコマンドを宣言的に管理している。`flagProfile`・`flagObservedBot`・`flagHead`・`cmdMentions`・`cmdCIChecks` を追加し、`flagRequest` と `flagObservedBot` を `multiFlags` へ加える。
  - `docs_test.go:110-177,210-335,384-457,504-519`: `TestGuideDescribesMentionCheck` は `fixprguard.RejectedMentions()` を参照して `fixprguard_guide.md` の `### 7.1`・`### 7.2` を検査するため、フェーズ 1 でガイドの 4 メンション記載を同時更新する必要がある。`TestGuideDescribesReviewerList` は `specs` で `flagReviewer` を持つサブコマンドの見出しに `--reviewer LOGIN`（`review-state` 以外は `...` 付き）を要求する。`guideTable`（`161-177` 行目）は見出し直後の**最初の**表のみ読むため、応答分類表は 1 表にまとめる。`checkDocumentedReviewer`（`228-247` 行目）は `flagReviewer` 非保持サブコマンドで早期リターンし、`flagRequest` を `inv.values` から読んでいるため、フェーズ 3 で `cmdMentions` 検査と `inv.multi[flagRequest]` 参照へ改める。`documentedInvocations`（`253-270` 行目）は `strings.Fields` で分割するため、`runpr.md` のコードブロック内の繰り返しフラグは 1 回ずつ書き、繰り返し規則を地の文に置く。
- **`internal/mergepr/`・`cmd/mergepr/`（既存と変更点）:**
  - `internal/mergepr/mergepr.go:63,76-111,275-277`: `const Protocol = 3`。`Prepare` が `t.gh(ctx, "pr", "checks", state.number(), "--watch", "--fail-fast")` を呼び、失敗時に `errChecksFailed`（`errors.go:16`）を返す。`Tool` は `mainFetch Runner` を `New` 内で初期化しテスト（`test_helpers.go:166-180` の `newCheckedTool`）で差し替える構造を持つため、`ghci` 用の上限付き `Runner`（`runner.go` に追加、`MaxOutput: 32 MiB`・`WaitDelay: 5s`・`Env: gitEnviron()`）と `ghci.WaitOptions` も同じ形で `Tool` フィールドに保持し、`mergepr_test.go`（`31,267,761,929,975` 行目の `gh pr checks` モック 5 箇所および `921` 行目のコメント）を `gh repo view`＋GraphQL 応答へ差し替える。
  - `cmd/mergepr/docs_test.go:175-195`: `TestProtocolToolsAgree` は `internal/*/*.go` の `const Protocol =` 宣言と文書ガードの一致を検証する。`internal/ghci` は `Protocol` 定数を持たない共有パッケージのため影響しない。
- **`cmd/yt2column/` の文書ガードとテスト削除対象:**
  - `fixpr_docs_test.go:53-64,160-197,218-231,292-839,945-1229`:
    - `wantReviewers = []string{"coderabbitai"}`（`64` 行目）をフェーズ 1 で `[]string{"coderabbitai", "sourcery-ai"}` へ更新し、`TestContextReviewerRowsMalformed` の 1 社・2 メンション前提のテストケースを合わせる。
    - `TestRunprDocContract/listings_print_no_body`（`190-192` 行目）と `TestRunprDocContract/request_comment_agrees_with_review_state`（`194-196` 行目）は旧 `--jq` フィルタを対象とするため、フェーズ 3 で削除する。ただし `checkGhAPICalls`（`606` 行目）が担っていた「`runpr.md` step 3 への無許可 `gh api` 呼び出し拒否」は `no_unchecked_gh_api` サブテストとして維持する。`bodyMarker`（`218` 行目）・`leftPlaceholder`（`231` 行目）・`checkGhAPICalls`（`606` 行目）・`gojqStrictPrelude`（`717` 行目）・`runJQ`（`721` 行目）は他テストが引き続き使用するため削除しない。テスト削除は `CLAUDE.md`「Deleting a test is a claim that must be checked」に従い、`ReviewStateRequest` 変更や `runpr.md` 改定前のグリーンな `BASE`（ステップ 3-1 完了直後）の上で**単独コミット**として行い、`make coverage-compare BASE=<親コミット>` を実行・記録する。
  - `review_mention_docs_test.go:93-169,238-308`:
    - `TestPostingPathsCheckMention/context_placeholder_definitions`（`165-167` 行目）は `_context.md` の `Rejected mentions` 行に `` `<rejected-mentions>` `` があることを要求している。フェーズ 1 では維持し、フェーズ 3 で `<rejected-mentions>` を廃止する際にこのアサーションも更新する。
    - `TestMentionsOnlyInAllowedRows`（`93-137` 行目）と `TestPostingCommandsInventory`（`276` 行目、`gh pr comment <number> --body "<review-request>"`）をフェーズ 3 で `### PR reviewers` 表（行数と許可列の検証を維持）と `<post-request>` へ更新する。
- **`internal/chief/`・`internal/pipeline/`・その他:**
  - `internal/chief/screen.go:113-177`: `mergeQuestionText` は `^PR #[1-9][0-9]* をどうしますか（第 [1-9][0-9]* 回。要約と詳細ファイルを確認のうえ）$` のみを `Merge: true` と判定し、他の単一選択の問いは `Merge: false`・`QuestionSingle`・`RelayAllowed` で中継する（`relay.go:69-77`）。依頼先選択 3 択の質問文をこの正規表現に一致させないため、プロダクションコードの変更は不要で、単体テスト追加と既存テスト通過で AC-28 を満たす。
  - `internal/pipeline/pipeline_test.go:578`: `TestFakesCarryBuildTag` は `testutil/` 配下のファイル数を `22` で固定している。本タスクでは新規 `testutil/` ファイルを追加しないため（`internal/cmdrunner/testutil` を再利用）、この件数は変わらない。`TestPackageReferenceListsPackages`（`630` 行目）は新パッケージ追加時に `docs/dev/conventions/package_reference.md` への行追加を要求する（ステップ 2-1 で同時更新）。
  - `.claude/commands/`: コミット `d29bb3d7` 時点で `sourcery` の記載は 0 件。`--protocol 8` / `"protocol":8` は `fixpr.md`（12 箇所）、`runpr.md`（14 箇所）、`fixprguard_guide.md`（24 箇所＋`66` 行目の本文 `` `8` ``）に存在する（フェーズ 3 で一括更新）。また `.claude/commands/_lib/pr-review-escalation.md`（`2,7-9,21-22,55-62` 行目）と `.claude/commands/fixpr.md`（`463,610-611,1073-1074` 行目の `@` なし表記案内）も合わせて更新対象に含める。
  - `CLAUDE.md:178-179,215,249-261,490-502,833-837`: `PR review request` 参照、`/runpr`・PR レビュー体制記述、`Declare, don't infer` の `fixprguard` 例外、および `TestRunprDocContract` の `jq` 依存記述を更新する（`TestHandoffListingPrintsNoBody` はすでに存在しないため合わせて整理する）。

### 1.4. 変更・置換対応表

本タスクで行う名前・構造・テストの置換を一覧化し、以降のステップおよび検証表はすべて本表に従う。

| 旧（変更前） | 新（変更後） | 実施ステップ |
|---|---|---|
| `fixprguard.RejectedMentions()` の `[]string{"@codex", "@coderabbitai"}` | `[]string{"@codex", "@coderabbitai", "@sourcery-ai", "@sourceryai"}` | ステップ 1-1 |
| `_context.md` `Automated reviewer list`: `` `coderabbitai` ``（`wantReviewers = []string{"coderabbitai"}`） | `` `coderabbitai`, `sourcery-ai` ``（`wantReviewers = []string{"coderabbitai", "sourcery-ai"}`） | ステップ 1-2 |
| `internal/fixprguard/github.go` の `resolveRepository` 独自実装 | `internal/ghci.ResolveRepo` への委譲 | ステップ 3-1 |
| `internal/mergepr/mergepr.go:275` の `gh pr checks <N> --watch --fail-fast` | `internal/ghci.ResolveRepo`＋`internal/ghci.WaitPR` | ステップ 2-2 |
| `.claude/commands/runpr.md:711` の `gh pr checks <number> --watch --fail-fast` | `fixprguard ci-checks --protocol <N> --head <head> <number>` | ステップ 3-5 |
| `.claude/commands/runpr.md:442-488` の `gh api --paginate --jq` 3 本と `<rejected-mentions>` | `fixprguard mentions --protocol <N> --observed-bot <observed-bot> --request "<accepted-request>" <number>` | ステップ 3-5 |
| `_context.md` の `Awaited PR reviewer` 行・`PR review request` 行 | Domain-specific 節の `### PR reviewers` 表（5 列: `Reviewer`・`Profile`・`Mode`・`Post request`・`Accepted requests`） | ステップ 3-5 |
| `runpr.md` の `<awaited-reviewer>`・`<review-request>` | `<pr-profile>`・`<pr-reviewer>`・`<post-request>`・`<accepted-request>`・`<observed-bot>` | ステップ 3-5 |
| `TestRunprDocContract/listings_print_no_body`・`TestRunprDocContract/request_comment_agrees_with_review_state` | `TestRunprDocContract/no_unchecked_gh_api`・`mentions`/`ci-checks` 呼び出し・旧経路不在検査、および `internal/fixprguard/mentions_test.go` | ステップ 3-1 / 3-2 / 3-5 |

---

### 1.5. 計画の変更の際に決めたこと（2026-11-25）

issue [#1140](https://github.com/isseis/yt2column/issues/1140) と、#1117（本書）のクロスチェックの部分レビューの再レビュー（タスク 0123）の指摘について、利用者と決めたことを記録する。

- **Sourcery AI の自動の PR レビューの完了を完了の印に含めない（[01_requirements.md](01_requirements.md)「issue #1140 で決めたこと（2026-11-25）」、[02_architecture.md](02_architecture.md) 1.4）:**
  - **決定:** ステップ 3-3 の `review_state.go` は、`ProfileSourcery` の `Markers` を手動依頼に応答した完了だけにし、依頼に応答しない自動の完了は `AutoReview` の `completed` に示すだけにする。ステップ 3-5 の `runpr.md` と `_lib/pr-review-escalation.md` は、`LAST_REVIEWED` を両社の手動の完了の印の和集合から求め、要約の kind 16 で自動の完了を「レビュー済みに数えない」と添えて示す。自動の完了は `markers` を変えないので、step 4 の質問前の再読込は `head` 上の `autoReview` の変化でも step 3 へ戻り、その自動の PR レビューのスレッドを処理する。PR の数と分け方は変えない。
  - **理由:** 要件定義書の同節のとおり、自動の PR レビューの完了を数えると、エスカレーションの判定と要約の「未レビュー分」の行が、自動の PR レビューが成功した PR で飛ばされるためである。変更はどちらも PR-5 の範囲に収まり、PR-3 と PR-4 には影響しない。
  - **ADR の判断:** [ADR-0013](../../dev/adr/0013-review-setup.md) を更新する（要件定義書の同節の決定。本書を draft に戻した文書の PR で更新する）。
- **ADR の食い違いの記述は、それを解消する PR で取り除く（[02_architecture.md](02_architecture.md) 1.4）:**
  - **決定:** `#1095` への食い違いの記述は、ステップ 4-2 でまとめて取り除かず、それを解消する PR で取り除くか、残る部分だけに書き換える。PR-1 で解消した ADR-0015 の分（自動レビュアーの一覧の記述を削除し、拒否するメンションと `/runpr` のメンション検査の記述を残る部分だけに書き換える）は、本書を draft に戻した文書の PR で行う。ADR-0013「レビュー済みの判定」の CI 待機の記述から `mergepr prepare` の部分を外すのはステップ 2-2（PR-3）、残りの記述をすべて取り除くのはステップ 3-5（PR-5）とする。ステップ 4-2 は、記述が残っていないことを確かめるだけにする。あわせて、ADR-0013 と ADR-0047 の `#1072` の食い違いの記述を残すという指示を除く。
  - **理由:** 要件定義書の AC-27 と `docs/dev/conventions/adr.md`「Editorial corrections」の「Discrepancy statements」は、食い違いを解消した PR が同じ PR で記述を取り除くか、残る部分に書き換えると定めており、これまでの配置（PR-6 でまとめて削除）はこれと食い違っていた。`#1072` の記述は #1115 がすでに取り除いており、残す指示は何も指していなかった。
  - **ADR の判断:** ADR なし（タスク内の作業の配置を、既存の規約に合わせる変更であるため）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: F-002・F-003 基礎（`fixprguard` の `Protocol` 据え置き）

#### ステップ 1-1: `internal/fixprguard` の `RejectedMentions()` 拡張と単体テスト更新

- **対象ファイル:**
  - `internal/fixprguard/errors.go`
  - `internal/fixprguard/trust_test.go`
  - `internal/fixprguard/write_test.go`
  - `internal/fixprguard/worktree_flow_test.go`
  - `cmd/fixprguard/main_test.go`
- **作業内容:**
  - [x] `internal/fixprguard/errors.go` の `RejectedMentions()` を `[]string{"@codex", "@coderabbitai"}` から `[]string{"@codex", "@coderabbitai", "@sourcery-ai", "@sourceryai"}` へ変更する（[02_architecture.md](02_architecture.md) 3.2.1）。
  - [x] 既存テスト（`internal/fixprguard/trust_test.go::TestAssessAuthors`・`TestAssessUnverifiableAuthor`、`github_test.go::TestBeginAuthorType`、`write_test.go::TestReplyTrustsEveryReviewer`、`cmd/fixprguard/main_test.go::TestSecondReviewerBeginReply`）を再利用し、未検証の差分ケースのみ追加する：`TestAssessAuthors` へ 3 者混在スレッド（`coderabbitai`＋`sourcery-ai`＋write 権限 `User`）と大文字小文字違い `Sourcery-AI`、`TestAssessUnverifiableAuthor` へ `sourcery-ai[bot]`（`ReasonNoAuthorName`）、`cmd/fixprguard/main_test.go` へ複数 `--reviewer` の `inspect` CLI ケースを追加する（AC-07）。
  - [x] `internal/fixprguard/write_test.go`（`TestCheckPostText` のリテラル表・`mentionTexts`・`TestReplyRejectsMention`・`TestEditPRRejectsMention`・`TestAppendPRRejects`）・`worktree_flow_test.go`（`TestFinishRejectsBodyFile`）・`cmd/fixprguard/main_test.go`（`TestPostTextMentionExitsUsage`）に、`RejectedMentions()` の反復に頼らないリテラル指定で `@sourcery-ai`・`@sourceryai`（大文字小文字混在・部分文字列・既存 PR 本文の `current-body-mention`）の拒否ケースを追加する（AC-11）。
  - [x] 壊して確かめる検証: `RejectedMentions()` から `@sourcery-ai`・`@sourceryai` をそれぞれ一時的に外し、`TestCheckPostText`・`TestReplyRejectsMention`・`TestEditPRRejectsMention`・`TestAppendPRRejects`（`current-body-mention` 経路を含む）・`TestFinishRejectsBodyFile`・`TestPostTextMentionExitsUsage` が失敗することを確かめ、コミットメッセージに記録する。
- **完了条件:**
  - `go test -tags test ./internal/fixprguard` および `cmd/fixprguard` の CLI テストが通ること（`TestGuideDescribesMentionCheck` はステップ 1-2 のガイド更新後に通過）。

#### ステップ 1-2: `_context.md` の一覧・拒否メンション拡張と `fixpr.md`・文書ガード・ガイド更新

- **対象ファイル:**
  - `.claude/commands/_context.md`
  - `.claude/commands/fixpr.md`
  - `cmd/yt2column/fixpr_docs_test.go`
  - `docs/dev/guides/fixprguard_guide.md`
- **作業内容:**
  - [x] `.claude/commands/_context.md` の `Automated reviewer list` 行（`362` 行目）の値を `` `coderabbitai` `` から `` `coderabbitai`, `sourcery-ai` `` へ（説明の単数表現も複数へ）、`Rejected mentions` 行（`365` 行目）の値を `` `@codex`, `@coderabbitai` `` から `` `@codex`, `@coderabbitai`, `@sourcery-ai`, `@sourceryai` `` へ更新する（説明の `@` なし表記例へ `Sourcery AI` を追記し、`<mention-check>`・`<rejected-mentions>` 定義はフェーズ 3 まで維持する。AC-06・AC-10・AC-12）。
  - [x] `.claude/commands/fixpr.md` の `@` なし表記案内（`463,610-611,1073-1074` 行目）を `Codex, CodeRabbit, or Sourcery AI` へ更新する（AC-12）。
  - [x] `cmd/yt2column/fixpr_docs_test.go` の `wantReviewers` を `[]string{"coderabbitai"}` から `[]string{"coderabbitai", "sourcery-ai"}` へ更新し、`TestContextReviewerRowsMalformed` の壊した写しケースを 2 レビュアー・4 メンション前提へ更新する（AC-09）。
  - [x] `docs/dev/guides/fixprguard_guide.md` の 4 章・5 章（書き込み系 4 サブコマンド）・7.1・7.2.1・7.2.3・7.2.6・8 章（`current-body-mention`）・10 章（`sourcery-ai` と 4 メンション該当箇所）をフェーズ 1 の状態に合わせて更新する（`TestGuideDescribesMentionCheck` 通過、AC-26 先行分）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` がすべて通ること。
  - **マージ後の手動作業:** 利用者が `main` から `make install-fixprguard` を実行する（`02_architecture.md` 8 章）。

### PR-1 作成ポイント: fixprguard reviewer list and rejected mentions

**対象ステップ**: 1-1 / 1-2

**推奨タイトル**: `feat(0122): fixprguard の拒否するメンションと自動レビュアーの一覧に Sourcery AI を追加`

**レビュー観点**: `RejectedMentions()` の 4 メンション拒否と `current-body-mention` 停止のリテラル検証 / `_context.md`・`fixpr.md`・`fixpr_docs_test.go`・`fixprguard_guide.md` のフェーズ 1 整合（`Protocol` と `<rejected-mentions>` 維持） / 既存著者判定テストの再利用と未検証の差分ケースの網羅

**実装モデル要件**: high

**判定理由**: ステップ 1-1・1-2 はセキュリティゲート（`fixprguard` の拒否するメンションと自動レビュアーの一覧）を段階移行の初段として変更するパネルレビュー相当の境界変更であり、標準モデル（Sonnet 等）では `RejectedMentions()` の反復に依存して壊れ得ないテストを書いたり、フェーズ 3 まで維持すべき `<rejected-mentions>` を早期に削ったりする恐れがある。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1128
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 2: 基盤と `mergepr` の CI 分離

#### ステップ 2-1: 共有パッケージ `internal/ghci` の新設と `package_reference.md` 更新

- **対象ファイル:**
  - `internal/ghci/ghci.go`（新規）
  - `internal/ghci/ghci_test.go`（新規）
  - `docs/dev/conventions/package_reference.md`
  - `CLAUDE.md`
- **作業内容:**
  - [x] `internal/ghci/ghci.go` を作成し、`ResolveRepo`・`ReadPR`・`WaitPR`、状態・理由・失敗項目・オプション型（`State`・`UnreadableReason`・`FailingCheck`・`Request`・`Result`・`WaitOptions`）、`States()`・`UnreadableReasons()`（`UnreadableNone` を除く）、番兵 `ErrInvalidRequest`、および型付きエラー `RepoError` を実装する。`RepoError` は `ResolveRepo` の失敗種別（`Reason UnreadableReason`: `UnreadableCall`・`UnreadableRejected`・`UnreadableLimit`）を呼び出し元が `errors.AsType[*RepoError]` で安全に分類できるようにし、`Error()` は外部出力を含まない固定文言のみとする。依存は `internal/cmdrunner`・`internal/strictjson`・標準ライブラリのみとし、厳格 JSON・16 MiB 上限・`app.slug == "github-actions"` 限定・同一ワークフロー最新 `createdAt` 採用・空 `QUEUED` CheckSuite 除外・即時失敗・`ExpectedHead` 一致・厳格 GitHub URL 検証の仕様は [02_architecture.md](02_architecture.md) 3.2.4・5.3 に従う。
  - [x] `internal/ghci/ghci_test.go` を作成し、`internal/cmdrunner/testutil` を用いて `ResolveRepo`（正常・`run == nil` の `ErrInvalidRequest`・`RepoError` の `call-failed`/`rejected`/`limit`）・`ReadPR`・`WaitPR` の全状態遷移と境界値を網羅検証する。検証対象には、外部 App 無視（`sourcery-ai` の `FAILURE`/`QUEUED`/100 件超 `hasNextPage`、`coderabbitai` の `SKIPPED`）、同一ワークフロー再実行、0 CheckRun `STARTUP_FAILURE`、`ExpectedHead` 不一致、`NoCheckGrace` と `Timeout`、厳格 JSON 拒否、`ErrInvalidRequest`、禁止テキスト非出力マーカー、列挙型安全側（`States()`・`UnreadableReasons()`・範囲外値 `99` の `MarshalText` 拒否・ゼロ値 `Result{}` の `"state":"unreadable"`）を含め、新規公開関数すべてがテストから到達可能であることを確認する（[02_architecture.md](02_architecture.md) 7.1・7.3、AC-19）。
  - [x] `CLAUDE.md` の `Declare, don't infer` に `internal/ghci` の例外（`app.slug`・`status`・`conclusion`・ワークフロー名・`createdAt`・`url`、GraphQL エラーの `type`・`extensions.code` の照合範囲と、その理由・境界）を追記する（PR #1143 のレビューで人が決定。`fixprguard review-state` と `internal/ciskip` の例外と同じ扱いにする）。
  - [x] 壊して確かめる検証: `app.slug == "github-actions"` フィルタと `ExpectedHead` 一致検証をそれぞれ一時的に無効化し、`TestReadPR`（`Sourcery review` 失敗無視ケース・`head-changed` ケース）と `TestWaitPR`（`head-changed` 即時終了ケース）が失敗することを確かめ、コミットメッセージに記録する。
  - [x] `docs/dev/conventions/package_reference.md` の `Shared Parts` へ `internal/ghci` 行を追加し、`internal/strictjson` 行（`110` 行目）の参照元へ `internal/ghci` を追記する（`TestPackageReferenceListsPackages` 通過）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` がすべて通ること。

### PR-2 作成ポイント: internal ghci shared package

**対象ステップ**: 2-1

**推奨タイトル**: `feat(0122): GitHub Actions CI チェック監視の共有パッケージ internal/ghci を新設`

**レビュー観点**: `app.slug == "github-actions"` 限定と他 App の `hasNextPage` 読み飛ばし / 同一ワークフロー再実行・空 `QUEUED` CheckSuite・`ExpectedHead` 不一致・`NoCheckGrace` と `Timeout` の状態遷移 / 厳格 JSON・`RepoError` 失敗分類・列挙型安全側と `make deadcode` 到達性

**実装モデル要件**: high

**判定理由**: ステップ 2-1 は外部 API 応答の厳格検証と信頼境界（`internal/ghci` の GraphQL 解析・URL 検証・ポーリング状態機械・`app.slug` 分離）を新設するパネルレビュー相当のセキュリティゲートであり、標準モデル（Sonnet 等）では `hasNextPage` 判定順序の誤りや厳格 JSON・URL 検証の抜け漏れ、外部 App の失敗混入を起こしやすい。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1143
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 2-2: `internal/mergepr` の CI 待機分離と `mergepr.md`・ガイド・テスト更新

- **対象ファイル:**
  - `internal/mergepr/runner.go`
  - `internal/mergepr/mergepr.go`
  - `internal/mergepr/test_helpers.go`
  - `internal/mergepr/mergepr_test.go`
  - `.claude/commands/mergepr.md`
  - `docs/dev/guides/mergepr_guide.md`
  - `docs/dev/adr/0013-review-setup.md`
- **作業内容:**
  - [ ] `internal/mergepr/runner.go` に `ghci` 用の `cmdrunner.Runner`（`MaxOutput: 32 MiB`・`WaitDelay: 5s`・`Env: gitEnviron()`）を追加する。
  - [ ] `internal/mergepr/mergepr.go` の `Tool` へ `ghci` 用 runner と `WaitOptions`（既定: `PollInterval: 30s`・`Timeout: 60m`・`PerCallTimeout: 2m`・`NoCheckGrace: 5m`）を保持し、`Prepare`（`275` 行目）の `gh pr checks <N> --watch --fail-fast` を `ghci.ResolveRepo`＋`ghci.WaitPR`（`ExpectedHead: state.HeadRefOID`）へ置き換える。`StatePassed` 以外は `errChecksFailed`（失敗ジョブ名・URL または理由コードのみ）を返す（`Protocol = 3` は据え置く。[02_architecture.md](02_architecture.md) 3.2.4・4.2）。
  - [ ] `internal/mergepr/test_helpers.go`（`newCheckedTool` でテスト用 runner・即時 `Sleep` 注入）と `internal/mergepr/mergepr_test.go` の `gh pr checks` モック（5 箇所）および `921` 行目の古いコメントを更新し、`Sourcery review` が `FAILURE` でも `github-actions` が成功なら通過すること、`github-actions` 失敗・`head-changed`・`unreadable` で `errChecksFailed` を返すことを検証する（AC-19）。
  - [ ] 壊して確かめる検証: `Prepare` の `ExpectedHead` 引き渡しと `StatePassed` 判定をそれぞれ一時的に壊し、`TestPrepare`・`TestPrepareRejectsFailedChecks` が失敗することを確かめ、コミットメッセージに記録する。
  - [ ] `.claude/commands/mergepr.md:47` の CI 待機記述（`gh pr checks --watch --fail-fast`）を `github-actions` チェック監視へ更新し、`docs/dev/guides/mergepr_guide.md`（2 章 `30` 行目の前提・4 章・5.2 節 `prepare`・6.1・6.2 節の `github-actions` 限定監視と `.github/workflows/deepseek-review.yml` の前提・7 章の古いバイナリ判別法と `make install-mergepr` 対処）を更新する（[02_architecture.md](02_architecture.md) 5.4・8 章）。
  - [ ] [ADR-0013](../../dev/adr/0013-review-setup.md)「レビュー済みの判定」の `#1095` への食い違いの記述から、このステップで解消する `mergepr prepare`（`internal/mergepr/mergepr.go`・`.claude/commands/mergepr.md`）の部分を外し、残る部分（`fixprguard review-state` と `runpr.md` の判定、`runpr.md` の CI 待機）だけに書き換える。編集上の訂正として、コミットメッセージに `Editorial correction:` 行を記録する（`docs/dev/conventions/adr.md`「Editorial corrections」、AC-27、1.5）。
  - [ ] 文書の事実照合: 既存インストール済みバイナリ（ビルド・再インストールは行わない）に対する `go version -m` の実出力（`build vcs.revision` 行）と、`.github/workflows/deepseek-review.yml` の実際のワークフロー名（`Code Review`）・トリガー（`workflow_dispatch` のみ有効、`pull_request`/`issue_comment` はコメントアウト）を照合し、`mergepr_guide.md` の記載と一致させる。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode`（`cmd/mergepr/docs_test.go` を含む）がすべて通ること。
  - **マージ後の手動作業:** 利用者が `main` から `make install-mergepr` を実行する（`02_architecture.md` 8 章）。

### PR-3 作成ポイント: mergepr CI check separation

**対象ステップ**: 2-2

**推奨タイトル**: `feat(0122): mergepr prepare の CI 待機を GitHub Actions 限定へ分離`

**レビュー観点**: `Prepare` の `ghci.ResolveRepo`＋`WaitPR`（`ExpectedHead: state.HeadRefOID`）切り替えと `errChecksFailed` 変換 / 出力上限 32 MiB の `ghci` 用 runner と `Protocol = 3` 据え置き / `mergepr.md`・`mergepr_guide.md` の `go version -m` と `deepseek-review.yml` 事実照合 / ADR-0013 の `#1095` の食い違いの記述から `mergepr prepare` の部分だけを外す書き換え（`Editorial correction:` 記録）

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 2-2 はマージ前ゲートの `mergepr prepare` を `gh pr checks` から `internal/ghci` へ切り替える単独の高リスク変更であり、標準モデル（Sonnet 等）では `ExpectedHead` の引き渡し漏れや非 `StatePassed` のエラー変換誤り、`go version -m` 判別手順の事実誤認を起こしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 3: `fixprguard` の `Protocol` 引き上げと F-003 完了・F-004 実装

#### ステップ 3-1: `internal/fixprguard` の非破壊内部追加（`ReviewProfile`・`mentions`・`ci-checks`・`ResolveRepo` 委譲）と `CLAUDE.md` 例外追記

- **対象ファイル:**
  - `internal/fixprguard/github.go`
  - `internal/fixprguard/review_state.go`
  - `internal/fixprguard/review_state_test.go`
  - `internal/fixprguard/mentions.go`（新規）
  - `internal/fixprguard/mentions_test.go`（新規）
  - `internal/fixprguard/ci_checks.go`（新規）
  - `internal/fixprguard/ci_checks_test.go`（新規）
  - `CLAUDE.md`
- **作業内容:**
  - [ ] `internal/fixprguard/github.go` の `resolveRepository` を `internal/ghci.ResolveRepo` へ委譲する形に整理し、`githubTimeout` と既存エラー変換（`ErrResponseRejected`、`types_test.go` の既存テスト互換、および `review_state.go:399-402` の既存挙動）を維持する。あわせて未使用になる `validRepoName`・`maxRepoNameLength` を削除し、`NewGitHubWithoutReviewer` の doc コメントへ `PRReviewState`・`PRMentions`・`CIChecks` を反映する。
  - [ ] `internal/fixprguard/review_state.go` へ `ReviewProfile` 型（`ProfileUnknown`・`ProfileCodeRabbit`・`ProfileSourcery`、`ParseReviewProfile`・`MarshalText`・`ReviewProfiles`）と共通の依頼コメント判定ヘルパー `isRequestComment` を追加し、`internal/fixprguard/review_state_test.go` にこれらの単体テスト（`ProfileUnknown(0)`・範囲外値 `99` の `MarshalText` 拒否、`ReviewProfiles()` 到達性を含む）を追加する（既存の `ReviewStateRequest`・`PRReviewState` 契約と `Protocol` はこのステップでは変更しない）。
  - [ ] `internal/fixprguard/mentions.go` を新規作成し、`ObservedBot`・`ParseObservedBot`・`MentionScanRequest`・`MentionScanState`・`MentionSource`・`MentionScanStage`・`MentionFinding`・`MentionScanResult`・`MentionScanStates`・`MentionSources`・`MentionScanStages`・`(*GitHub).PRMentions` を実装する（[02_architecture.md](02_architecture.md) 3.2.3・5.3 の 3 エンドポイント厳格 JSON・`ghci.ResolveRepo` の `*ghci.RepoError` 分類・`body: null` / `submitted_at: null` 許容・4 メンション検査・CodeRabbit 定型 2 種と Sourcery レビューガイド／7 日間予算超過コメント除外・`isRequestComment` 共有・本文非出力）。
  - [ ] `internal/fixprguard/ci_checks.go` を新規作成し、`CIChecksRequest`・`CIChecksResult`・`(*GitHub).CIChecks`（`ghci.ResolveRepo` の `*ghci.RepoError` 分類＋`ghci.ReadPR` の薄いラッパー、`githubTimeout` 適用）を実装する（[02_architecture.md](02_architecture.md) 3.2.4）。
  - [ ] `internal/fixprguard/mentions_test.go` と `internal/fixprguard/ci_checks_test.go` を作成し、[02_architecture.md](02_architecture.md) 7.1・7.3 の全観点を検証する。`mentions_test.go` では 4 メンション×3 エンドポイント×著者種別、観測済み定型除外と非定型・`User` なりすまし・他利用者名の予算超過コメントの非除外、手動依頼 3 種一致、厳格 JSON 拒否と `stage`、上限超過、禁止テキスト非出力マーカー、列挙型 `MarshalText` の範囲外・不正ゼロ値拒否、ゼロ値 `MentionScanResult{}` の `"state":"unreadable"`、直接構築したゼロ値・不正値・重複 `ObservedBot` の `ErrInvalidInput` 拒否を確かめる。`ci_checks_test.go` では `ResolveRepo` 失敗分類・状態伝播・`--head` 不一致を確かめ、`MentionScanStates`・`MentionSources`・`MentionScanStages` を含む新規公開関数すべてがテストから到達可能であることを確認する（AC-05・AC-13・AC-19）。
  - [ ] 壊して確かめる検証: `mentions.go` の `user.type == "Bot"` 照合と定型 1 行目照合をそれぞれ一時的に緩め、`TestPRMentionsObservedExclusions` が失敗することを確かめ、コミットメッセージに記録する。
  - [ ] `CLAUDE.md` の `Declare, don't infer` の `fixprguard` 例外（`490-502` 行目）に `internal/fixprguard/mentions.go` の検査範囲と境界（1 行目の観測済み定型照合・依頼文面一致・4 メンション部分一致のみ、本文非出力）を追記する（AC-29 先行分）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` がすべて通ること（`Protocol` 未変更のため既存 CLI と文書ガードがすべて通る）。

### PR-4 作成ポイント: fixprguard internal mentions and ci-checks

**対象ステップ**: 3-1

**推奨タイトル**: `feat(0122): fixprguard 内部に ReviewProfile・mentions・ci-checks を追加`

**レビュー観点**: `mentions.go` の 3 エンドポイント厳格 JSON・4 種定型除外のなりすまし防止（`user.type == "Bot"`・1 行目完全一致）・本文非出力 / `ObservedBot` 不正構築と列挙型安全側検証 / `resolveRepository` 委譲の互換維持と `CLAUDE.md` 例外追記

**実装モデル要件**: high

**判定理由**: ステップ 3-1 は信頼できない PR コメント・レビューを扱うセキュリティゲート（`mentions.go` の厳格 JSON・定型除外・なりすまし防止・本文非出力）を新設するパネルレビュー相当の変更であり、標準モデル（Sonnet 等）では `User` なりすまし除外の抜けや `body: null`/`submitted_at: null` の厳格処理誤り、`*ghci.RepoError` の分類漏れを起こしやすい。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 3-2: 旧 `--jq` サブテストの単独削除と `make coverage-compare` 検証

- **対象ファイル:**
  - `cmd/yt2column/fixpr_docs_test.go`
- **作業内容:**
  - [ ] **先行・単独テスト削除コミット（`CLAUDE.md`「Deleting a test is a claim that must be checked」準拠）:** `ReviewStateRequest` 契約変更や `runpr.md` 改定を行う前のグリーンな `BASE`（PR-4 / ステップ 3-1 マージ後、`mentions_test.go` がすでに `main` にある状態）の上で、旧 `--jq` フィルタを対象としていた以下を単独コミットで削除し、`make coverage-compare BASE=<親コミット>`（タイムアウト 10 分以上を設定）を実行して `coverage-compare: PASS` の出力をコミットメッセージと PR 説明に記録する。`checkGhAPICalls` の不変条件（`runpr.md` step 3 への無許可 `gh api` 呼び出し拒否）は同コミット内で `TestRunprDocContract/no_unchecked_gh_api` サブテストへ引き継ぎ、`bodyMarker`（`218` 行目）・`leftPlaceholder`（`231` 行目）・`checkGhAPICalls`（`606` 行目）・`gojqStrictPrelude`（`717` 行目）・`runJQ`（`721` 行目）は残す：
    - [ ] `cmd/yt2column/fixpr_docs_test.go` から `TestRunprDocContract/listings_print_no_body` サブテストを削除する（`checkGhAPICalls` 呼び出しは `no_unchecked_gh_api` サブテストへ移す）
    - [ ] `cmd/yt2column/fixpr_docs_test.go` から `TestRunprDocContract/request_comment_agrees_with_review_state` サブテストを削除する
    - [ ] 上記 2 サブテスト専用で未使用になるヘルパー・定数・型（`fakeAwaited`・`fakeUser`・`fakeRequest`・`crObservedHeader1`・`crObservedHeader2`・`paginatedListing`・`stderrToFile`・`runprListingKeys`・`mentionAuthorKeys`・`runprRequiredKeys`・`mentionWriter` とその定数群・`listingItem`・`listingFixture`・`marked`・`timeFieldSeconds`・`fakeTimeBase`・`fakeIDBase`・`runprListingFixtures`・`listingReplacer`・`checkListings`・`checkRequestAgreementWithReviewState`・`checkListingConstructors`・`jqObjectBody`・`splitTopLevel`・`skipJQString`・`checkListingOutput`）を削除する
- **完了条件:**
  - `make test && make lint && make deadcode` および `make coverage-compare BASE=<親コミット>` がすべて `PASS` すること。

#### ステップ 3-3: `internal/fixprguard` `review-state` 状態機械拡張と `Protocol` の `main` の値 + 1 引き上げ

- **対象ファイル:**
  - `internal/fixprguard/types.go`
  - `internal/fixprguard/review_state.go`
  - `internal/fixprguard/test_helpers_review.go`
  - `internal/fixprguard/review_state_test.go`
- **作業内容:**
  - [ ] `internal/fixprguard/types.go` の `Protocol` を `origin/main` の値 + 1 へ引き上げる（`CLAUDE.md`「Protocol numbers」）。
  - [ ] `internal/fixprguard/review_state.go` を [02_architecture.md](02_architecture.md) 3.2.2 の仕様どおり拡張する（`RunSkipped`・`RequestSkipped`・`AutoReviewState`・`AutoReview`・`ReviewStateRequest` の `Profile` と複数 `Requests`・`PRReviewState` の `Profile` 常時設定と `AutoReview`・GraphQL `$checkName` と `name` 完全一致照合・プロファイル別分類表・`ProfileSourcery` の依頼 0 件時読み取り・15 分応答上限・同一／より古いコミット競合判定・4 段階優先順位 `AutoReview`・手動依頼に応答した完了だけを `Markers` に入れる規則と `ReviewURL` 解決。自動の完了は `AutoReview` の `completed` だけに示す。1.5）。
  - [ ] `internal/fixprguard/test_helpers_review.go`（`//go:build test`）と `review_state_test.go` を拡張し、`ProfileCodeRabbit` 既存ケース維持＋`ProfileSourcery` の全状態・優先順位・未開始 `startedAt: null`・手動 `waiting` かつ `Response == nil` 時の未開始 CheckRun 非誤認・15 分応答上限・同一コミット競合と後続コミット非競合・空 `QUEUED` CheckSuite 無視・`name` 不一致 `rejected`・`Profile` 常時設定・自動の完了が `Markers` に入らず `AutoReview` の `completed` になること（1.5）を検証する。あわせて `TestReviewStateEnumsRejectOutsideValues`（`ReviewProfile`・`AutoReviewState(99)`・ゼロ値 `PRReviewState{}` の `json.Marshal` 失敗・`PRReviewState{Profile: ProfileCodeRabbit}` の `"state":"indeterminate"`・ゼロ値 `AutoReview{}` の `"state":"none"`/`"kind":"unknown"`）、`TestReviewStateObserved`（`observations.md` の Sourcery 観測値フィクスチャ追加）、`TestReviewStateNonPRCommitIgnored`（`ProfileSourcery` ケース追加）、`TestReviewStateOutputHasNoText`（`ProfileSourcery` `AutoReview` の禁止テキスト非出力ケース追加）を更新する（AC-05・AC-16〜AC-20・AC-24）。
  - [ ] 壊して確かめる検証: `review_state.go` の `name` 照合、`ProfileSourcery` の `commitIndex` 競合判定、および自動の完了を `Markers` から外す条件をそれぞれ一時的に壊し、`TestReviewStateSourceryAuto`・`TestReviewStateSourceryManual` が失敗することを確かめ、コミットメッセージに記録する。
- **完了条件:**
  - `go test -tags test ./internal/fixprguard` がすべて通ること。

#### ステップ 3-4: `cmd/fixprguard` CLI 統合と CLI テスト更新

- **対象ファイル:**
  - `cmd/fixprguard/main.go`
  - `cmd/fixprguard/main_test.go`
- **作業内容:**
  - [ ] `cmd/fixprguard/main.go` に `cmdMentions`・`cmdCIChecks`・`flagProfile`・`flagObservedBot`・`flagHead` を追加し、`flagRequest` と `flagObservedBot` を `multiFlags` に登録して `specs` とハンドラ（`reviewState`・`mentions`・`ciChecks`）を実装する（[02_architecture.md](02_architecture.md) 2.2・4.2）。
  - [ ] `cmd/fixprguard/main_test.go` の `validArgs()`・`TestGhTargetEnvRejected`・`TestReviewStateRun` を更新し、`TestMentionsRun`・`TestCIChecksRun` を追加して引数解析・終了コード `0`/`2`・`GH_REPO`/`GH_HOST` 拒否・JSON 出力を検証する（AC-13・AC-16・AC-19・AC-20）。
- **完了条件:**
  - `go test -tags test ./internal/fixprguard` および `cmd/fixprguard` の CLI テスト（`docs_test.go` はステップ 3-5 の文書更新と合わせて通過）が通ること。

#### ステップ 3-5: `_context.md`・`runpr.md`・`fixpr.md`・`_lib/pr-review-escalation.md`・ガイド 3〜5 章・文書ガード・`CLAUDE.md` 例外の一括更新

- **対象ファイル:**
  - `.claude/commands/_context.md`
  - `.claude/commands/runpr.md`
  - `.claude/commands/fixpr.md`
  - `.claude/commands/_lib/pr-review-escalation.md`
  - `docs/dev/guides/fixprguard_guide.md`
  - `CLAUDE.md`
  - `cmd/fixprguard/docs_test.go`
  - `cmd/yt2column/fixpr_docs_test.go`
  - `cmd/yt2column/review_mention_docs_test.go`
  - `docs/dev/adr/0013-review-setup.md`
  - `docs/dev/adr/0015-untrusted-review-threads.md`
  - `docs/dev/adr/0047-runpr-merge-question.md`
- **作業内容:**
  - [ ] `.claude/commands/_context.md` の Domain-specific 節から `Awaited PR reviewer` 行・`PR review request` 行および `Rejected mentions` 行の `<rejected-mentions>` 定義を削除し、`Rejected mentions` 行（`365` 行目）の例外参照を `### PR reviewers` 表の `Post request` へ、`Guard tool` 行（`366` 行目）を `mentions`・`ci-checks`・自動の PR レビューを含む記述へ更新し、`### PR reviewers` 表（5 列・`coderabbitai` と `sourcery-ai` の 2 行）を新設する（[02_architecture.md](02_architecture.md) 3.1、AC-12・AC-15）。
  - [ ] `.claude/commands/runpr.md` を [02_architecture.md](02_architecture.md) 3.3・4.2・6.1・6.2 の仕様どおり更新する。冒頭では frontmatter・説明（`2,11-20,53-54` 行目の旧 `--jq` 例外と CodeRabbit 専用表現）・プレースホルダー・認可リストを更新し、全 `fixprguard` 呼び出しの `--protocol` を `main` の値 + 1 へ揃える。step 3 では旧 `<stderr-dir>`/`mktemp -d`・`gh api user --jq .login`（step 2 `350` 行目は維持）を削除し、`fixprguard mentions`・両社 `fixprguard review-state`・`TIMED_OUT_AUTO`／`FIRST_SEEN_AUTO`・両社の手動の完了の印の和集合による `LAST_REVIEWED`（自動の完了は数えない。1.5）・`fixprguard ci-checks --head` を組み込む。step 4 では質問前再読込と `mentions` 再確認（`732-735,776-779` 行目。`head` 上の `autoReview` の変化でも step 3 へ戻る。1.5）・要約最大 16 行（kind 16 の自動の完了には「レビュー済みに数えない」と添える）・マージの質問表示条件と推奨・依頼先 3 択と `mergepr discard` 前の投稿前確認・`Manual PR review stops` 表への `skipped` 追加を反映し、レビュアー名と 4 メンションは直書きしない（AC-08・AC-13〜AC-25）。
  - [ ] `.claude/commands/fixpr.md` の全 `fixprguard` 呼び出しと説明の `--protocol`（12 箇所）を `main` の値 + 1 へ更新する。
  - [ ] `.claude/commands/_lib/pr-review-escalation.md`（`2,7-9,21-22,55-62` 行目）を CodeRabbit（手動）と Sourcery AI（自動・手動）の併用、両社の手動の PR レビューの完了の印の和集合による `LAST_REVIEWED`（Sourcery AI の自動の PR レビューの完了は数えない。1.5）、および `fixprguard mentions`・`ci-checks` へ更新する（AC-18・AC-22）。
  - [ ] `docs/dev/guides/fixprguard_guide.md` のガード結合章（3 章のプロトコル番号記載、4 章の新 `--protocol`・`--profile`・`--observed-bot`・`--request` 複数回・`--head`・`mentions`/`ci-checks` の終了コード `0`、5 章の `review-state` 拡張と `mentions`・`ci-checks` 節新設、および文書内すべての `--protocol`・`"protocol"` 例）を更新する。なお、`guideTable` 互換のため `**応答の分類。**` 直下は 1 表にまとめる（非結合章の 1 章・7.2.4・7.2.6・10 章はステップ 4-1 で更新。AC-26）。
  - [ ] `CLAUDE.md` の `178-179` 行目（廃止する `PR review request` 行参照を `_context.md` の `PR reviewers` 表へ更新）および `Declare, don't infer` の `fixprguard` 例外（`490-502` 行目：`review_state.go` の CheckRun `name` 照合とプロファイル別分類表）を更新する（AC-29）。
  - [ ] `cmd/fixprguard/docs_test.go` を更新する。`TestDocumentedInvocationsParse` の `runpr.md` 期待サブコマンドへ `cmdMentions`・`cmdCIChecks` を追加し、`checkDocumentedReviewer`（`228-247` 行目：`flagReviewer` 非保持でも `cmdMentions` を検査するよう早期リターン条件を改め、`flagRequest` 参照を `inv.values` から `inv.multi` へ切り替え）と `TestCheckDocumentedReviewer` を新プレースホルダー（`<pr-profile>`・`<pr-reviewer>`・`<accepted-request>`・`<observed-bot>`）へ対応させる。さらに `TestGuideListsReviewStateValues` へ `ReviewProfile`・`AutoReviewState`・`RunSkipped`・`RequestSkipped` を追加し、`mentions`・`ci-checks` のガイド値表検査（`TestGuideListsMentionsValues`・`TestGuideListsCIChecksValues`）を新設する（AC-08・AC-15・AC-26）。
  - [ ] このステップで解消する `#1095` への食い違いの記述を、ADR から取り除く。対象は、[ADR-0013](../../dev/adr/0013-review-setup.md) の「PR レビューの依頼」「PR レビューを勧める判定」「レビュー済みの判定」（ステップ 2-2 で書き換えた残り）「信頼する自動レビュアーの一覧と、PR レビューの待機・依頼の対象」、[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) の「ガードツール経由の書き込みとメンション拒否」「`interaction limits` とメンション検査」、[ADR-0047](../../dev/adr/0047-runpr-merge-question.md) の「マージの質問の形式と選択肢」の記述である。取り除く前に `git grep -nE 'issues/1095([^0-9]|$)' origin/main -- docs/dev/adr/` で記述を探し、このステップが解消しない記述が残っていればそれだけを残す。編集上の訂正として、ADR ごとにコミットメッセージへ `Editorial correction:` 行を記録する（`docs/dev/conventions/adr.md`「Editorial corrections」、AC-27、1.5）。
  - [ ] `cmd/yt2column/fixpr_docs_test.go` と `cmd/yt2column/review_mention_docs_test.go` を更新する。`fixpr_docs_test.go` では `checkContextReviewers`・`TestContextReviewerRowsMalformed` の `### PR reviewers` 5 列表と旧行 0 件検査、`TestContextAndCommandsAgree` の全 `Reviewer` 直書き禁止、`TestFixprguardCallSites` への `mentions`・`ci-checks` 追加、`TestRunprDocContract/no_unchecked_gh_api` の `step3ReadCalls` 空化による `runpr.md` step 3 への `gh api` 呼び出し全面禁止と `gh pr checks --watch` 不在検査を反映する。`review_mention_docs_test.go` では `TestMentionsOnlyInAllowedRows` の `### PR reviewers` 許可化（期待行数と `Post request`・`Accepted requests` 列限定を固定）、`TestPostingPathsCheckMention` からの `<rejected-mentions>` 要求削除、`TestPostingCommandsInventory` の `<post-request>` 更新を行う（AC-08・AC-09・AC-12・AC-15・AC-25）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` がすべて通ること。
  - **PR レビュー・マージ前後の手動作業:** 利用者が PR の `/runpr` 前にこの PR のブランチから `make install-fixprguard` を実行し、マージ直後に `main` から `make install-fixprguard` を再実行する（`fixprguard_guide.md` 3 章・`CLAUDE.md`「Protocol numbers」）。

### PR-5 作成ポイント: fixprguard protocol bump and runpr multi-reviewer support

**対象ステップ**: 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0122): fixprguard のプロトコル更新と runpr の複数レビュアー・自動の PR レビュー対応を実装`

**レビュー観点**: 先行単独テスト削除コミットの `make coverage-compare PASS` と `no_unchecked_gh_api` 引き継ぎ / `review-state` の複数プロファイル・自動レビュー状態機械・`name` 照合・同一コミット競合判定 / `Protocol` 引き上げと `_context.md`・`runpr.md`・`fixpr.md`・`_lib/pr-review-escalation.md`・ガイド 3〜5 章・全ガードの一括整合 / 自動の完了を完了の印に数えない `LAST_REVIEWED` / ADR-0013・0015・0047 の `#1095` の食い違いの記述の削除（`Editorial correction:` 記録）

**実装モデル要件**: high

**判定理由**: ステップ 3-2〜3-5 はセキュリティゲートの `Protocol` 引き上げ・段階移行完了・単独テスト削除（`make coverage-compare`）・`review-state` 状態機械拡張・`runpr.md` 大幅改定を同時に行うパネルレビュー相当の移行 PR であり、標準モデル（Sonnet 等）では削除コミット順序誤りや `checkDocumentedReviewer`/`guideTable` のガード破綻、自動レビュー時間切れ・競合判定の抜けを起こしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 4: 仕様文書・ADR・`chief` 確認の仕上げ

#### ステップ 4-1: `CLAUDE.md`・`docs/translation_glossary.md`・`fixprguard_guide.md` 非結合章の更新

- **対象ファイル:**
  - `CLAUDE.md`
  - `docs/translation_glossary.md`
  - `docs/dev/guides/fixprguard_guide.md`
- **作業内容:**
  - [ ] `CLAUDE.md` の `215,249-261` 行目（`/runpr` と PR レビュー体制を CodeRabbit 手動＋Sourcery AI 自動・手動の併用へ更新）、`833-837` 行目（`jq` が必要なテスト記述から `TestRunprDocContract`・`TestHandoffListingPrintsNoBody` を除き `TestFixprPhase6CountFilter` に合わせる）を更新する（AC-29）。
  - [ ] `docs/translation_glossary.md` の `完了を待つ PR レビュアー`・`手動の PR レビュー`・`完了の印` の定義を CodeRabbit＋Sourcery AI 併用と `### PR reviewers` 表に合わせて更新し、`自動の PR レビュー`（`automatic PR review`）を追加する（AC-30）。
  - [ ] `docs/dev/guides/fixprguard_guide.md` のうち PR-5 のガード非結合章（1 章の位置付け・7.2.4 `513` 行目の基準値なし一覧への `review-state`/`mentions`/`ci-checks` 追記・7.2.6・10 章の残余リスク〔`02_architecture.md` 5.4：予算超過 1 行目除外・Sourcery PR 本文更新と `current-body-mention`/上書き競合・ruleset `26400325` 監視〕）を更新する（AC-26）。
- **完了条件:**
  - 変更した記述が実装後の `_context.md`・`runpr.md`・`fixprguard` と一致し、`cmd/fixprguard/docs_test.go` が通ること。

#### ステップ 4-2: ADR-0013・ADR-0015・ADR-0047 に `#1095` の食い違いの記述が残っていないことの確認

- **対象ファイル:**
  - `docs/dev/adr/0013-review-setup.md`
  - `docs/dev/adr/0015-untrusted-review-threads.md`
  - `docs/dev/adr/0047-runpr-merge-question.md`
- **作業内容:**
  - [ ] `git grep -nE 'issues/1095([^0-9]|$)' origin/main -- docs/dev/adr/` で、3 つの ADR の `## 決定` に本タスクの `#1095` の食い違いの記述が残っていないことを確かめる。記述は、それを解消した PR（PR-3・PR-5、PR-1 の分は本書を draft に戻した文書の PR）が取り除く（1.5）。
  - [ ] 残っている記述があれば（取り除き忘れ）、このステップで取り除き、ADR ごとにコミットメッセージへ `Editorial correction:` 行を記録する（`docs/dev/conventions/adr.md`「Editorial corrections」、AC-27）。
- **完了条件:**
  - `go test -tags test ./cmd/yt2column -run TestADRRecords` が通り、3 ADR に `#1095` への食い違い記述が残っていないこと。

#### ステップ 4-3: `internal/chief` の単一選択中継テスト確認と全体検証

- **対象ファイル:**
  - `internal/chief/screen_test.go`
- **作業内容:**
  - [ ] `internal/chief/screen_test.go` に、`internal/chief/testdata/screens/` の既存単一選択サンプルから `plainSample` で派生させたテストケース `TestPickerQuestionRelayable`（画面全体の Go リテラル埋め込みは行わない、`CLAUDE.md` 準拠）を追加し、依頼先選択の 3 択の問い（質問文 `PR #<number> の PR レビューをどのレビュアーに依頼しますか`、選択肢 `CodeRabbit`・`Sourcery AI`・`マージの質問に戻る`）が `parseClaudeScreen` と `parseOpenCodeScreen` の双方で `Merge: false`・`QuestionSingle`・`RelayAllowed`・全選択肢 `Relayable` と判定されることを確かめる（AC-28）。
  - [ ] 既存の `internal/chief` 全テスト、およびリポジトリ全体の `make test`・`make lint`・`make deadcode`・CI の `doc-guard` と同一コマンド（`go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`）を実行してすべて成功することを確認する（AC-28・AC-31）。
- **完了条件:**
  - `make fmt && make test && make lint && make deadcode` と `doc-guard` 相当コマンドがすべて成功すること。

### PR-6 作成ポイント: docs, ADR cleanup, and chief relay test

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `docs(0122): CLAUDE.md・用語集・ADR 整合更新と chief 中継テストを追加`

**レビュー観点**: `CLAUDE.md`・`docs/translation_glossary.md`・`fixprguard_guide.md` 非結合章と実装後の最終仕様の一致 / ADR-0013・0015・0047 に `#1095` の食い違いの記述が残っていないことの確認 / `TestPickerQuestionRelayable` の両パーサ検証と全体 `doc-guard` 通過

**実装モデル要件**: low

**判定理由**: いずれのトリガーにも該当しない（仕様文書・用語集・ガイド非結合章の更新、ADR の食い違いの記述が残っていないことの確認、既存サンプル派生による `internal/chief` 単体テスト 1 件の追加に留まるため）。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

1. **M1（フェーズ 1 完了）:** `sourcery-ai` のスレッド自動処理と 4 メンション拒否（`RejectedMentions()`・`_context.md`・`fixprguard_guide.md`・文書ガード）が `Protocol` 据え置きで有効になる。利用者が `main` から `make install-fixprguard` を実行する。
2. **M2（フェーズ 2 完了）:** 共有パッケージ `internal/ghci` が整備され、`mergepr prepare` が `app.slug == "github-actions"` のみ監視する形へ切り替わる。利用者が `main` から `make install-mergepr` を実行する。
3. **M3（フェーズ 3 完了）:** `fixprguard` の `Protocol` が `main` の値 + 1 へ上がり、`review-state` 拡張・`mentions`・`ci-checks`・`_context.md` `### PR reviewers` 表・`runpr.md` 複数レビュアー運用・ガイド 3〜5 章・文書ガードがすべて整合する。利用者が `main` から `make install-fixprguard` を再実行する。
4. **M4（フェーズ 4 完了）:** `CLAUDE.md`・用語集・`fixprguard_guide.md` 非結合章・ADR 3 件・`internal/chief` テストが最終状態と一致し、全 AC（AC-01〜AC-31）と CI が通過する。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 | `fixprguard` の 4 メンション拒否・自動レビュアーの一覧・`fixpr.md` 案内・ガイド・文書ガード（`Protocol` 据え置き） | high | auto |
| PR-2 | 2-1 | 共有パッケージ `internal/ghci` 新設と `package_reference.md` 更新（PR-1 マージ後の `make install-fixprguard` のため `hold`） | high | hold |
| PR-3 | 2-2 | `internal/mergepr` の CI 待機分離と `mergepr.md`・`mergepr_guide.md`・テスト更新（`Protocol = 3` 据え置き）、ADR-0013 の食い違いの記述から `mergepr prepare` の部分を外す | standard+high-review | auto |
| PR-4 | 3-1 | `internal/fixprguard` への `ReviewProfile`・`mentions`・`ci-checks`・`ResolveRepo` 委譲の非破壊追加と `CLAUDE.md` 例外（PR-3 マージ後の `make install-mergepr` のため `hold`） | high | hold |
| PR-5 | 3-2 / 3-3 / 3-4 / 3-5 | 旧 `--jq` テスト単独削除・`review-state` 拡張・`cmd/fixprguard`・`Protocol` 引き上げ・`_context.md`・`runpr.md`・`fixpr.md`・`_lib/pr-review-escalation.md`・ガイド 3〜5 章・全ガード一括更新、ADR 3 件の `#1095` の食い違いの記述の削除 | high | auto |
| PR-6 | 4-1 / 4-2 / 4-3 | `CLAUDE.md`・用語集・ガイド非結合章更新、ADR 3 件に `#1095` の食い違いの記述が残っていないことの確認、`internal/chief` 単一選択中継テスト追加と全体検証（PR-5 マージ後の `make install-fixprguard` のため `hold`） | low | hold |

---

## 4. テスト戦略 (Test Strategy)

詳細なテストケース設計は [02_architecture.md](02_architecture.md) 7 章に従う。

- **単体テスト:**
  - `internal/fixprguard`: 著者判定（`trust_test.go`・`github_test.go`・`write_test.go`）、4 メンション拒否（`write_test.go`・`worktree_flow_test.go`）、`review-state` 複数プロファイルと自動レビュー状態機械（`review_state_test.go`）、厳格メンション検査（`mentions_test.go`）、`ci-checks` ラッパー（`ci_checks_test.go`）。
  - `internal/ghci`: `ResolveRepo`・`ReadPR`・`WaitPR` の全状態・再試行・上限・外部 App 無視・厳格 JSON 拒否（`ghci_test.go`）。
  - `internal/mergepr`: `Prepare` の `ghci` 連携と `Sourcery review` 失敗時の非停止（`mergepr_test.go`）。
  - `cmd/fixprguard`: 新サブコマンドとフラグ解析・終了コード・環境変数拒否（`main_test.go`）。
  - `internal/chief`: 依頼先 3 択の単一選択中継判定（`screen_test.go` へ追加、および既存の `relay_test.go` を含む `internal/chief` 全テスト通過）。
- **文書ガード・契約テスト:**
  - `cmd/yt2column/fixpr_docs_test.go`・`review_mention_docs_test.go`・`adr_test.go`、`cmd/fixprguard/docs_test.go`、`cmd/mergepr/docs_test.go`、`internal/pipeline/pipeline_test.go` を各フェーズのグリーンゲートで実行する。
- **セキュリティテスト:**
  - コメント本文・CheckRun `title`・`details_url`・`output.summary` に固有マーカーを埋め込み、標準出力・標準エラーへの非出力を機械的に検証する。
  - なりすまし `User`・`user: null`・大文字小文字違い・重複 JSON キー・不正 UTF-8・単独サロゲート・後続ゴミデータ・サイズ上限超過の拒否を検証する。
  - `fixprguard_guide.md` 7.1 の保証と `mergepr_guide.md` 6.1 の保証を変える箇所は、実装を一時的に壊してテストが失敗することを確認し、コミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。
- **テスト削除の検証:**
  - ステップ 3-2 で削除する旧 `--jq` サブテスト 2 件と専用ヘルパーは、`ReviewStateRequest` 変更と `runpr.md` 改定前のグリーンな `BASE` の上で単独コミットに分離し、`make coverage-compare BASE=<親コミット>` の `PASS` 出力をコミットメッセージと PR 説明に残す。
- **テストヘルパー構成（`docs/dev/conventions/test_organization.md` 準拠）:**
  - パッケージをまたぐ fake は既存の `internal/cmdrunner/testutil` を再利用し、新規 `testutil/` ファイルは追加しない（`TestFakesCarryBuildTag` の 22 ファイル固定を維持）。
  - パッケージ内部ヘルパーは既存の `internal/fixprguard/test_helpers_review.go`・`internal/mergepr/test_helpers.go`・`cmd/fixprguard/test_helpers.go`（いずれも `//go:build test`）を拡張して再利用する。`internal/ghci` はまず `ghci_test.go` 内に収め、分離が必要になった場合のみ `internal/ghci/test_helpers.go`（`//go:build test`）を設ける。

---

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 緩和策 |
|---|---|---|
| フェーズ 1（PR-1）マージ後・フェーズ 3 完了（PR-5）マージ前の移行期に、旧 `--jq` 経路が `sourcery-ai[bot]` の定型コメントを `by_bot` 確認へ回す | PR ごと・`/runpr` 実行ごとに 1 回 Bot 確認が出る | 安全側の挙動であり運用継続に支障なし。PR-5 マージで解消する（[02_architecture.md](02_architecture.md) 8 章） |
| フェーズ 2 完了（PR-3）マージ後に `mergepr` を再インストールし忘れる（`Protocol = 3` 据え置きのため自動検出されない） | `Sourcery review` の失敗・未完了で `mergepr prepare` が止まる／待ち続ける | PR-4 を `自動送信: hold` とし、マージ後の手動作業として計画と PR 説明に明記する。`mergepr_guide.md` 7 章に `go version -m` による判別法と対処を記載する |
| PR-5（ステップ 3-3）の `fixprguard` `Protocol` 引き上げ時に並行ブランチが同じ番号を使う | マージ前の `Protocol` 衝突またはツール不一致 | `CLAUDE.md`「Protocol numbers」に従い、PR 作成時に `origin/main` の値 + 1 を設定し、マージ時は `mergepr` の自動検査で確認する |
| PR-5 先頭（ステップ 3-2）の旧 `--jq` サブテスト削除時に、意図せずプロダクションコードのカバレッジが下がる | `make coverage-compare` が `FAIL` になる | 先行 PR（PR-4 / ステップ 3-1）で `mentions_test.go` をマージ済みとした上で、`ReviewStateRequest`・`runpr.md` 変更前のグリーンな `BASE` に対して削除コミットを単独作成し、`make coverage-compare` を実行する |
| 本タスク自身の PR で Sourcery AI が PR 説明末尾に `@sourcery-ai` を引用して追記し、`append-pr` が `current-body-mention` で止まる | `/fixpr` Phase 6 の追記が自動完了しない | 安全側の停止として `fixprguard_guide.md` 8 章どおり `append-pr` を繰り返さず、利用者が PR 説明を GitHub 上で確認して対処する（[02_architecture.md](02_architecture.md) 5.4） |
| 食い違いを解消した PR が、ADR の `#1095` の食い違いの記述を取り除き忘れる | 解消済みの食い違いが ADR に残り、AC-27 と `docs/dev/conventions/adr.md` の規則に反する | 取り除く作業を解消するステップ（ステップ 2-2・3-5）に置き、各 PR のレビュー観点に挙げる。ステップ 4-2 で残っていないことを確かめる（1.5） |

---

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2）
- [x] PR-2 マージ済み（対象ステップ: 2-1）
- [ ] PR-3 マージ済み（対象ステップ: 2-2）
- [ ] PR-4 マージ済み（対象ステップ: 3-1）
- [ ] PR-5 マージ済み（対象ステップ: 3-2 / 3-3 / 3-4 / 3-5）
- [ ] PR-6 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）

---

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

各 AC と実装ステップ・検証手段の対応を以下に示す。種別は `test`（実行可能な動作テスト）、`static`（ガードテスト・`make` ターゲット）、`manual`（自然言語文書・手順プロンプトの PR レビュー確認）とする。

| AC | 対象ステップ | 種別 | 検証場所・コマンド | 検証内容 |
|---|---|---|---|---|
| AC-01 | 設計前完了（PR #1102＋本 PR 追記） | `manual` | `docs/tasks/0122_sourcery_trusted_reviewer/observations.md` | 10 項目＋CI 補足・#1097/#1098 再レビュー範囲の観測記録と出典の目視確認（PR #1102 承認済み） |
| AC-02 | 設計前完了（PR #1102） | `manual` | `docs/tasks/0122_sourcery_trusted_reviewer/observations.md` | 手動依頼投稿者が利用者 `isseis` である記録の目視確認（PR #1102 承認済み） |
| AC-03 | 設計前完了（PR #1102） | `manual` | `docs/tasks/0122_sourcery_trusted_reviewer/observations.md` | `02_architecture.md` と同一 PR（#1102）でコミット済み・試験 PR 未作成の確認 |
| AC-04 | 設計前完了（PR #1102＋本 PR 確認） | `manual` | `docs/tasks/0122_sourcery_trusted_reviewer/observations.md` | 前提との照合・項目 8 ruleset 確認・#1097/#1098 再レビュー範囲確認の利用者合意確認 |
| AC-05 | 3-1 / 3-3 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateObserved`, `internal/fixprguard/mentions_test.go::TestPRMentionsObservedExclusions` | 分類表・定型除外ヘッダーの実装値が `observations.md` の観測値と一致することを確認 |
| AC-06 | 1-2 | `test` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `TestContextReviewerRowsMalformed` | `_context.md` の `Automated reviewer list` が `[bot]` なしでちょうど `coderabbitai` と `sourcery-ai` の 2 つであることを検証 |
| AC-07 | 1-1 | `test` | `internal/fixprguard/trust_test.go::TestAssessAuthors`, `TestAssessUnverifiableAuthor`, `internal/fixprguard/github_test.go::TestBeginAuthorType`, `internal/fixprguard/write_test.go::TestReplyTrustsEveryReviewer`, `cmd/fixprguard/main_test.go::TestSecondReviewerBeginReply`, `TestInspectTrustsEveryReviewer` | `Bot`/`sourcery-ai` の無照会信頼・`authorType: "Bot"`・3 者混在スレッド・`inspect` CLI・大文字小文字違い `Sourcery-AI`・`sourcery-ai[bot]`（`ReasonNoAuthorName`）・`User`/`sourcery-ai` の非自動レビュアー扱いを検証 |
| AC-08 | 1-2 / 3-5 | `static` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`, `TestCheckDocumentedReviewer` | コマンド文書に `coderabbitai`・`sourcery-ai`（`[bot]` 含む）が直書きされずプレースホルダー経由で渡されることを検証 |
| AC-09 | 1-2 / 3-5 | `test`, `static` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `TestContextReviewerRowsMalformed`, `make test` | 文書ガードが 2 社を期待値として照合し、`make test` が成功することを検証 |
| AC-10 | 1-1 / 1-2 | `test` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `TestContextReviewerRowsMalformed`, `cmd/yt2column/review_mention_docs_test.go::TestMentionsOnlyInAllowedRows` | `_context.md` の `Rejected mentions` と `fixprguard.RejectedMentions()` が 4 メンションで一致することを検証 |
| AC-11 | 1-1 | `test` | `internal/fixprguard/write_test.go::TestCheckPostText`, `TestReplyRejectsMention`, `TestEditPRRejectsMention`, `TestAppendPRRejects`, `internal/fixprguard/worktree_flow_test.go::TestFinishRejectsBodyFile`, `cmd/fixprguard/main_test.go::TestPostTextMentionExitsUsage` | 書き込み系 4 サブコマンドが 4 メンションを ASCII 大文字小文字無視・部分文字列で拒否し、`append-pr` が既存本文で `current-body-mention` 停止することを検証 |
| AC-12 | 1-2 / 3-5 | `static`, `manual` | `cmd/yt2column/review_mention_docs_test.go::TestMentionsOnlyInAllowedRows`, `TestPostingPathsCheckMention`, `TestPostingCommandsInventory`（`fixpr.md`・`_context.md` の `@` なし表記案内は `manual`） | `<mention-check>` の 4 メンション拒否、許可箇所以外への 4 メンション記載禁止、`@` なし表記案内の更新を検証 |
| AC-13 | 3-1 / 3-2 / 3-4 / 3-5 | `test`, `static`, `manual` | `internal/fixprguard/mentions_test.go::TestPRMentions`, `TestPRMentionsObservedExclusions`, `TestPRMentionsErrors`, `cmd/fixprguard/main_test.go::TestMentionsRun`, `cmd/yt2column/fixpr_docs_test.go::TestFixprguardCallSites`, `TestRunprDocContract`（`runpr.md` の Bot 確認質問手順は `manual`） | 4 メンション×3 経路の著者別扱い・定型 4 種除外・なりすまし拒否・本文非出力・`runpr.md` の `fixprguard mentions` 呼び出しと無許可 `gh api` 禁止・Bot 確認手順を検証 |
| AC-14 | 3-5（既存維持） | `static`, `test` | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（`head_before_checkout`）, `TestFixprguardCallSites`, `cmd/fixprguard/main_test.go::TestInteractionLimitsRun` | `runpr.md` step 1 で `head` → `interaction-limits` → `gh pr checkout` の順序と停止判定が維持されることを検証 |
| AC-15 | 3-5 | `test`, `static` | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`, `TestContextReviewerRowsMalformed`, `cmd/yt2column/review_mention_docs_test.go::TestMentionsOnlyInAllowedRows`, `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`, `TestCheckDocumentedReviewer` | `_context.md` の `### PR reviewers` 表の 5 列・2 行・依頼文面定義とコマンド文書の直書き禁止を検証 |
| AC-16 | 3-3 / 3-4 / 3-5 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateSourceryAuto`, `cmd/fixprguard/main_test.go::TestReviewStateRun`（`runpr.md` の `TIMED_OUT_AUTO`/`FIRST_SEEN_AUTO` 手順は `manual`） | `head` 上の進行中自動 CheckRun の `AutoRunning`・未開始 `startedAt: ""`・15 分超過 `AutoTimedOut`・祖先未完了非待機を検証し、step 4 非再待機手順をレビューで確認 |
| AC-17 | 3-3 | `test` | `internal/fixprguard/review_state_test.go::TestReviewState`, `TestReviewStateSourceryAuto`, `TestReviewStateSourceryManual`, `TestReviewStateObserved`, `TestReviewStateNonPRCommitIgnored`, `TestReviewStateOutputHasNoText` | CodeRabbit（手動）と Sourcery AI（手動、`@sourcery-ai review`/`@sourceryai review`。自動の完了は完了の印に含めず `AutoReview` の `completed` に示す）の完了の印判定、`name`/`app.slug`/非 PR コミット・本文非出力を検証 |
| AC-18 | 3-3 / 3-5 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateSourceryAuto`, `TestReviewStateSourceryManual`, `TestReviewStateLimits`（`runpr.md` の両社和集合・祖先算出手順・`_lib/pr-review-escalation.md` は `manual`） | 両社の `markers` 出力と `unreadable (limit)` 時の動作を検証し、`runpr.md`・`_lib/pr-review-escalation.md` の `LAST_REVIEWED` 和集合規則をレビューで確認 |
| AC-19 | 2-1 / 2-2 / 3-1 / 3-3 / 3-4 / 3-5 | `test`, `static`, `manual` | `internal/ghci/ghci_test.go::TestReadPR`, `TestWaitPR`, `internal/fixprguard/ci_checks_test.go::TestCIChecks`, `cmd/fixprguard/main_test.go::TestCIChecksRun`, `internal/mergepr/mergepr_test.go::TestPrepare`, `TestPrepareRejectsFailedChecks`, `cmd/mergepr/docs_test.go`, `internal/fixprguard/review_state_test.go::TestReviewStateSourceryAuto`, `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（`mergepr.md`・`mergepr_guide.md` は `manual`） | 自動レビュー異常時の `AutoSkipped`/`AutoUnrecognized`/`AutoTimedOut`、`internal/ghci`・`ci-checks`・`mergepr prepare` の `github-actions` 限定監視、`runpr.md` からの `gh pr checks --watch` 廃止、`mergepr` 文書整合を検証 |
| AC-20 | 3-1 / 3-3 / 3-4 / 3-5 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateSourceryManual`, `TestReviewStateUnreadable`, `TestReviewStateLimits`, `internal/fixprguard/mentions_test.go::TestPRMentionsErrors`, `cmd/fixprguard/main_test.go::TestReviewStateRun` | 手動依頼の `skipped`/`rate-limited`/`unrecognized`/`timed-out`・`indeterminate`・`unreadable` 各理由を検証し、`runpr.md` の停止・再試行・同時待機手順をレビューで確認 |
| AC-21 | 3-5 | `manual` | `.claude/commands/runpr.md` step 4（PR レビュー確認） | マージの質問の最大 4 択・単一選択維持と、`head` 未レビュー残存時の「先に PR レビューを依頼」表示条件を確認（手順文書の自然言語仕様） |
| AC-22 | 3-5 | `manual` | `.claude/commands/runpr.md` step 4・`.claude/commands/_lib/pr-review-escalation.md` | `LAST_REVIEWED == head` 時の「未レビューの変更: なし」・推奨「マージ」、未レビュー時のエスカレーション推奨規則、および自動の PR レビューが `head` を完了していても `LAST_REVIEWED != head` ならエスカレーションと要約の「未レビュー分」の行が働くことを確認（手順文書の自然言語仕様） |
| AC-23 | 3-5 / 4-3 | `static`, `test`, `manual` | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory`, `internal/chief/screen_test.go::TestPickerQuestionRelayable` | `gh pr comment <number> --body "<post-request>"` が `runpr.md` step 4 に 1 か所のみであること・3 択質問の中継可能性を検証し、投稿前確認・重複投稿抑止手順をレビューで確認 |
| AC-24 | 3-3 / 3-5 | `test`, `manual` | `internal/fixprguard/review_state_test.go::TestReviewStateOutputHasNoText`（`runpr.md` 要約 16 行・表示優先順位は `manual`） | 本文・`title`・`details_url` 非出力を検証し、`runpr.md` の CodeRabbit・Sourcery AI 各 1 行要約（自動の完了の「レビュー済みに数えない」表示を含む）と URL 優先順位をレビューで確認 |
| AC-25 | 3-5 | `static` | `cmd/yt2column/review_mention_docs_test.go::TestPostingCommandsInventory` | PR 作成コマンド群に自動レビュアーへの依頼コメント投稿コマンドが存在しないことを検証 |
| AC-26 | 1-2 / 3-5 / 4-1 | `test`, `manual` | `cmd/fixprguard/docs_test.go::TestGuideCoversStopKindsAndScope`, `TestGuideDescribesMentionCheck`, `TestGuideDescribesReviewerList`, `TestGuideListsReviewStateValues`, `TestGuideListsMentionsValues`, `TestGuideListsCIChecksValues`, `TestDocumentedProtocolMatches` | `docs/dev/guides/fixprguard_guide.md` の値表・コマンド文法・4 メンション・`--protocol` 一致を検証し、文章記述をレビューで確認 |
| AC-27 | 2-2 / 3-5 / 4-2 | `test`, `manual` | `cmd/yt2column/adr_test.go::TestADRRecords` | ADR 構造・索引整合を検証し、ADR-0013・0015・0047 の `## 決定` 一致と、`#1095` の食い違いの記述を解消した PR（PR-3・PR-5、PR-1 の分は本書を draft に戻した文書の PR）で取り除いたことをレビューで確認する。ステップ 4-2 で残っていないことを確かめる |
| AC-28 | 4-3 | `test` | `internal/chief/screen_test.go::TestPickerQuestionRelayable`, `go test -tags test ./internal/chief` | 依頼先選択 3 択の中継判定および既存 `internal/chief` 全テスト通過を検証 |
| AC-29 | 3-1 / 3-5 / 4-1 | `manual` | `CLAUDE.md`（PR レビュー体制・`Declare, don't infer` 例外・`jq` 記述） | `CLAUDE.md` の該当箇所が実装と一致していることを PR レビューで確認（自然言語仕様） |
| AC-30 | 4-1 | `manual` | `docs/translation_glossary.md` | 用語集の該当定義が複数レビュアー併用と自動の PR レビューへ更新されていることを PR レビューで確認（自然言語用語集） |
| AC-31 | 全ステップ / 4-3 | `test`, `static` | `make test`, `make lint`, `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief` | 全単体テスト・リンター・WASM ビルド検査・`doc-guard` ジョブ相当の通過を検証 |

### 7.1. 横断検索チェックリスト（`make lint`・`make test` で検出できない文書整合）

- [ ] ステップ 3-5 完了時点でその時点の更新対象（`.claude/commands/`・`docs/dev/guides/fixprguard_guide.md` 3〜5 章・`CLAUDE.md:178-179`）に、さらにステップ 4-1・4-2 完了時点で仕様文書全体（`docs/dev/`・`CLAUDE.md`・`.claude/commands/`・`docs/translation_glossary.md`。過去の `docs/tasks/` と `internal/chief/testdata/` を除く）に、廃止済みの `Awaited PR reviewer`・`PR review request`・`<awaited-reviewer>`・`<review-request>`・`<rejected-mentions>`・`cr_observed`・`gh pr checks --watch`（`mergepr`・`runpr` の CI 待機記述）の古い記述が残っていないことを PR レビューで目視確認する（`manual`）。
- [ ] ステップ 2-2・3-5 の各 PR で、その PR が解消した `#1095` の食い違いの記述が取り除かれている（一部だけ解消したときは残る部分だけに書き換えられている）こと、およびステップ 4-2 の完了時点で ADR-0013・ADR-0015・ADR-0047 に `#1095` の食い違いの記述が残っていないことを PR レビューで目視確認する（`manual`）。

---

## 8. 成功基準 (Success Criteria)

- **機能完全性:** F-001〜F-005 の全 31 件の受け入れ基準（AC-01〜AC-31）が実装・検証済みであること。
- **品質:** すべての実装 PR で `make fmt`・`make test`・`make lint`・`make deadcode`・CI `doc-guard` が通り、テスト削除コミットで `make coverage-compare` が `PASS` していること。
- **セキュリティ:** 本文・`title`・`details_url`・`output.summary` の非出力、4 メンション拒否、厳格 JSON 検証（要件定義書 3.2 項目 4 の `jq` 例外解消を含む）、直書き禁止ガードがテストで実証され、保証変更箇所の壊して確かめる検証がコミットメッセージに記録されていること。
- **文書整合:** `_context.md`・`runpr.md`・`fixpr.md`・`mergepr.md`・`_lib/pr-review-escalation.md`・`fixprguard_guide.md`・`mergepr_guide.md`・`package_reference.md`・`CLAUDE.md`・`docs/translation_glossary.md`・ADR-0013・ADR-0015・ADR-0047 がすべて実装と一致していること。

---

## 9. 次のステップ (Next Steps)

1. 本計画書（`03_implementation_plan.md`）の PR レビューを受け、`/runpr` で承認（`approved`）とマージを行う。
2. マージ後、`/runplan 0122` で PR-1（ステップ 1-1）から PR-6 まで順に実装・検証を進める。
3. 各開発ツールの更新 PR マージ後に、利用者が手順どおり `make install-fixprguard`（PR-1 マージ後および PR-5 の `/runpr` 前・マージ直後）および `make install-mergepr`（PR-3 マージ後）を実行する（これらの直後となる PR-2・PR-4・PR-6 は `自動送信: hold`）。

## 変更履歴

- 2026-11-24 承認（isseis、#1117）
- 2026-11-25 draft への戻し：02_architecture.md の決定の変更に伴う（#1143）
- 2026-11-25 承認（isseis、#1143）
- 2026-11-25 draft への戻し：自動の PR レビューの完了を完了の印に含めない実装と、ADR の食い違いの記述を解消する PR で取り除く配置（#1158）
- 2026-11-25 承認（isseis、#1158）
