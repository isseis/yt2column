# 実装計画書：`/mergemain` は拡張の検査の前に `make ext-install` で依存を入れる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-27 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）および [02_architecture.md](02_architecture.md)（アーキテクチャ設計書。以下、設計書）に基づく実装計画である。既存ファイルの記述は、コミット `0dcdfc0c` の内容で確かめた。`file:line` はこのコミットの行番号を指す。

## 1. 実装の概要 (Implementation Overview)

**目的:** `/mergemain` が拡張の検査が要る取り込みで `make ext-check` の前に `make ext-install` を走らせるよう、コマンド定義と関連文書を更新し、既存のガードテストと実地の確かめで受け入れ基準（AC-01〜AC-11）を検証する。

**実装方針:**

-   設計書 1 章・8 章に従い、変更対象は `.claude/commands/mergemain.md`、`CLAUDE.md`、`docs/dev/adr/0046-chief-of-staff.md`、`.claude/commands/_lib/harness-antigravity.md` の 4 文書のみとする。Go コード、`Makefile`、`.claude/settings.json`、ガードテストは変更しない。
-   上記 4 文書の初期変更は 1 つのコミットにまとめ、`mergemain.md` と `CLAUDE.md` の承認の不整合、および Antigravity での昇格対象の不整合を作らない（設計書 8 章）。
-   AC-07・AC-08 の差分限定は、差分が小さいため一時スクリプト（`_checks/`）を作らず、`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の記録した差分の読み取り（recorded diff reading）で検証する。
-   設計書 7.3 の実地の確かめ（auto mode での `/mergemain` 実行）は、実装 PR を開いた後・マージの前に人が一時的な検証用 PR で実施し、その結果の記録と ADR-0046 の「結果」の更新を同じ実装 PR に追加する。

### 1.1. 既存コード調査結果

コミット `0dcdfc0c` で各ファイルを調べた結果は次のとおりである。`implementation_handoff.md` は存在しない。`design_handoff.md` の H-01〜H-03 はすべて設計書の付録 A に方針が記録済みである。

| ファイル | 確認した箇所（`0dcdfc0c`） | 現状と本タスクでの扱い |
|---|---|---|
| `.claude/commands/mergemain.md` | 承認（`:16-28`）、禁止事項 9（`:78-80`）、止まる理由の対応（`:110`）、手順 7（`:251-267`）、手順 10（`:305-336`） | 承認は依存のインストールを明示的に除外し、禁止事項 9 は `make ext-install` を禁じ、手順 7 は `make ext-preflight` → `make ext-check` を本文の箇条で指示している。設計書 3.1 の 5 か所のみを更新する |
| `CLAUDE.md` | 「Tool Execution Safety」の `/mergemain` の項（`:1259`） | 4 つのネットワーク操作のみを承認している。設計書 3.2 に従い、拡張の検査が要る取り込みでの `make ext-install` の承認を足す |
| `docs/dev/adr/0046-chief-of-staff.md` | 食い違いの文（`:85`）、「結果」の未確認・拒否時の文（`:515`） | `:85` に `mergemain.md`・`CLAUDE.md` との食い違いの文が残っている。`:515` に auto mode の判定が未確認である旨と、人が確認を断ったときの想定がある。ステップ 1-1 で `:85` を消し、ステップ 1-3 の実地の確かめの後に `:515` を実測に合わせて更新する（設計書 3.4） |
| `.claude/commands/_lib/harness-antigravity.md` | 4 章「Sandbox Elevation Rules」の 2（`:48-55`） | 昇格してよいネットワークコマンドの例に `make ext-install` がない。設計書 3.7 に従い 1 項目を足す |
| `.claude/settings.json` | 全体（`:1-54`、`mergemain-restore` の `ask` 規則は `:14`） | `make` の `allow` 規則はない。変更しない（設計書 1.2・3.3） |
| `Makefile` | `ext_preflight`（`:407-413`）、`ext-preflight`（`:417-418`）、`ext-install`（`:425-428`）、`ext-typecheck`〜`ext-build`（`:430-458`） | `ext-install` が `ext_check_versions`、`check-lockfile`、`npm ci --ignore-scripts --registry=https://registry.npmjs.org/ --replace-registry-host=never` を順に実行する。変更しない（要件書 2.3） |
| `cmd/yt2column/settings_test.go` | `unpromptedWords`（`:31-36`）、`promptingModes`（`:40-45`） | `.claude/settings.json` を変えないため変更不要（設計書 3.5） |
| `cmd/mergepr/docs_test.go` | `TestMergemainNoDirectGitMutation`（`:530-535`）、`mergemainLinePrefixes`（`:543-547`）、`unlistedCommandLines`（`:561-584`） | 囲みコードブロックの行のみを読む。`make ext-install` は手順 7 の本文に書くため変更不要（設計書 1.2・3.5） |

### 1.2. 計画の際に決めたこと（2026-11-27）

人間のレビュアーが決めたもの:

-   **設計書 7.3 の実地の確かめの用意の仕方**
    -   **決定:** 一時的な検証用 PR を使う。`main` 上で `extension/`・`README.md`・`README.en.md`・`docs/dev/guides/development.md` のいずれかを変更した直近コミットの親から一時ブランチを切り、実装コミットを当てて draft の検証用 PR を作り、確かめの後に閉じて一時ブランチを消す。
    -   **理由:** 実装 PR の履歴に検証用の merge コミットを残さず、`main` の進み具合に依存せずに拡張の検査が要る取り込みを確実に再現できるためである。
    -   **人の答え:** 一時的な検証用 PR を使う（推奨案）。
    -   **ADR:** なし。本タスクの検証手順の中で閉じ、ほかのタスクや構成要素を縛らないためである。
-   **設計書 7.3 の実地の確かめを行うタイミング**
    -   **決定:** 実装 PR を開いた後・マージの前に人が行い、結果の記録（本書 §9）と ADR-0046 の「結果」の更新を同じ実装 PR に追加する。
    -   **理由:** 実地の確かめには人の操作が要るため、PR 作成前に入れると `/runplan` の途中で止まって自動送信を `hold` にする必要がある。PR 作成後・マージ前に置けば、`/runplan` は文書更新・ガードテスト・`diff` 記録・PR 作成までを一貫して進められる。
    -   **人の答え:** PR を開いた後・マージの前に人が行い、記録と ADR-0046 の更新を同じ PR に足す（推奨案）。
    -   **ADR:** なし。本タスクの進め方の中で閉じ、ほかのタスクや構成要素を縛らないためである。

計画で決めたもの:

-   **AC-07・AC-08 の `diff` 検証の形:** 変更箇所が `CLAUDE.md` の 1 箇所と `mergemain.md` の 5 か所に限られ差分が小さいため、一時スクリプト（`_checks/`）ではなく、`requirements_process.md` §4 の記録した差分の読み取り（recorded diff reading）を採る。ADR: なし（本タスクの検証手順の中で閉じるため）。
-   **実地の確かめの記録の置き場所:** 本書の §9「実地の確かめの記録」に置く。ADR: なし（同上）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 文書・コマンド定義の更新と確かめ

設計書 8 章の優先順位に従い、1 つの PR で実施する。ステップ 1-1・1-2 は `/runplan` が実行して PR を開き、ステップ 1-3 は PR 作成後・マージ前に人が実行して同じ PR に追加する。

#### ステップ 1-1: コマンド定義・CLAUDE.md・ADR-0046・Antigravity アダプタの更新

-   **変更対象ファイル:**
    -   `.claude/commands/mergemain.md`
    -   `CLAUDE.md`
    -   `docs/dev/adr/0046-chief-of-staff.md`
    -   `.claude/commands/_lib/harness-antigravity.md`
-   **作業内容:**
    -   [ ] `.claude/commands/mergemain.md` の 5 か所（承認の節、禁止事項の 9、止まる理由の対応の「検査の準備がない」の行、手順 7、手順 10 の報告の項目）を設計書 3.1 の仕様どおり更新する（手順 7 の `make ext-install` は囲みコードブロックではなく本文に書き、`make ext-preflight` を除く）
    -   [ ] `CLAUDE.md` の「Tool Execution Safety」の `/mergemain` の項（`CLAUDE.md:1259`）に、拡張の検査が要る取り込みでの `make ext-install` の承認を足す（設計書 3.2）
    -   [ ] `docs/dev/adr/0046-chief-of-staff.md` の「決定」にある食い違いの文（`0046-chief-of-staff.md:85`）を消す（設計書 3.4）
    -   [ ] `.claude/commands/_lib/harness-antigravity.md` の 4 章「Sandbox Elevation Rules」の 2（`harness-antigravity.md:48-55`）の昇格してよいコマンドの種類に、`make ext-install`（コマンドの定義が指示するときに限る）をネットワーク通信の例として足す（設計書 3.7）
    -   [ ] 上記 4 ファイルを 1 つのコミットにまとめる（設計書 8 章）
-   **完了条件:**
    -   変更ファイルが上記 4 文書のみで、`.claude/settings.json`・`Makefile`・Go コード・ガードテストに変更がないこと。

#### ステップ 1-2: ガードテストの実行と `diff` 記録の作成

-   **変更対象ファイル:**
    -   `docs/tasks/0130_mergemain_ext_install/03_implementation_plan.md`（§8 の Diff record の記入）
-   **作業内容:**
    -   [ ] `make test` と `make lint` を実行し、既存のガードテスト（`cmd/mergepr/docs_test.go`、`cmd/yt2column/settings_test.go`、`cmd/yt2column/skills_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go` を含む）がすべて通ることを確かめる（設計書 3.5・7.1）
    -   [ ] `requirements_process.md` §4「`diff` verification」の固定コマンドで AC-07（`CLAUDE.md`）と AC-08（`.claude/commands/mergemain.md`）の差分を取得し、本書の §8 の各 Diff record に記入してコミットする
    -   [ ] PR を開く前に、`requirements_process.md` §4「Rechecking a record」の手順で Diff record が最新コミットと一致することを照合する
-   **完了条件:**
    -   `make test && make lint` が通り、§8 の Diff record が照合に合格していること。

#### ステップ 1-3: 実地の確かめと ADR-0046 の仕上げ（PR 作成後・マージ前に人が実施）

-   **変更対象ファイル:**
    -   `docs/tasks/0130_mergemain_ext_install/03_implementation_plan.md`（§9 の記入）
    -   `docs/dev/adr/0046-chief-of-staff.md`（「結果」の更新）
-   **作業内容:**
    -   [ ] `main` 上で `extension/`・`README.md`・`README.en.md`・`docs/dev/guides/development.md` のいずれかを変更した直近コミットの親から一時ブランチを切り、ステップ 1-1 の実装コミットを当てて draft の検証用 PR を作成する
    -   [ ] 利用者の設定（`~/.claude/settings.json` や worktree の `.claude/settings.local.json`）に `make` に当たる許可規則がないことと、worktree に `extension/node_modules` がないことを確かめたうえで、新規または `/clear` 直後の auto mode の Claude Code pane で `/mergemain <検証用 PR>` のみを実行する（設計書 7.3）
    -   [ ] Claude Code のセッション記録（transcript）で `make ext-install` が許可の確認なしに実際に走ったこと、`make ext-check` まで通ったこと、報告に「拡張の依存」の項目が出たことを確かめる。あわせて、確認を出すモードで人が断ったときの振る舞い（および判定が拒んだ回があれば手順 9 への遷移）を観測する（設計書 7.3）
    -   [ ] 観測結果を本書の §9「実地の確かめの記録」に記入し、設計書 3.4 に従い `docs/dev/adr/0046-chief-of-staff.md` の「結果」（`0046-chief-of-staff.md:515`）の「未確認である」の文と人が断ったときの振る舞いの文を更新して、実装 PR にコミット・push する（確認が出るか拒まれた場合は、設計書 7.3 に従い設計書と ADR-0046 に戻って方針を決め直す）
    -   [ ] `make test` と `make lint` が通ること、および §8 の Diff record が最新コミットと一致することを確かめる
    -   [ ] 検証用 PR を閉じ、一時ブランチ（ローカル・リモート）を削除する
-   **完了条件:**
    -   auto mode で `make ext-install` が確認なしに実行され `make ext-check` まで通った記録が §9 にあり、ADR-0046 の「結果」が更新され、`make test && make lint` と Diff record の照合が通り、検証用 PR と一時ブランチが削除されていること。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 実装順序

設計書 8 章の順序どおり、単一フェーズ・単一 PR で進める。

1.  **マイルストーン 1（実装 PR の作成）:** ステップ 1-1（4 文書の更新を 1 コミット）→ ステップ 1-2（`make test && make lint` と AC-07・AC-08 の Diff record 記入）→ 実装 PR の作成。
2.  **マイルストーン 2（実地の確かめとマージ準備）:** ステップ 1-3（一時的な検証用 PR での実地の確かめ、§9 の記録、ADR-0046「結果」の更新、`make test && make lint` の再確認、検証用 PR・一時ブランチの片付け）→ `/runpr` でのマージ。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト・ガードテスト:** Go コードの変更はなく、追加する単体テストやテストヘルパーファイルはない（`docs/dev/conventions/test_organization.md` に該当する追加なし）。`make test && make lint` により、既存の文書・設定ガードテスト（`cmd/mergepr/docs_test.go`、`cmd/yt2column/settings_test.go`、`cmd/yt2column/skills_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`）がすべて通ることを確かめる（設計書 3.5・7.1）。
-   **結合テスト:** なし（設計書 7.2）。
-   **実地の確かめ:** 設計書 7.3 に従い、ステップ 1-3 で一時的な検証用 PR を用いて auto mode の Claude Code pane で `/mergemain` を実行し、`make ext-install` の確認なし実行（transcript 確認）・`make ext-check` 通過・報告項目・確認拒否時の振る舞いを確かめ、§9 に記録する。Antigravity での実地の確かめは求めず、機会があれば §9 に追記する（設計書 7.3）。
-   **文書の確かめ・`diff` 検証:** 文言の意味は PR レビュー（`manual`）で読む。AC-07・AC-08 の差分限定は §8 の Diff record（recorded diff reading）で検証し、一時スクリプト（`_checks/`）は作らない。
-   **後方互換性:** chief が `/mergemain` を送るのはブランチの `mergemain.md` が `origin/main` と同じときに限られるため（要件書 4.4）、本実装が `main` に入った後に取り込まれるブランチから新しい手順が適用される。既存のガードテストと `mergepr` のプロトコル番号（4）は変わらない。

## 5. リスク管理 (Risk Management)

### 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| auto mode の判定が `make ext-install` に確認を出すか拒む | AC-05 を満たせず、chief の自動送信で止まる | ステップ 1-3 でマージ前に実地で確かめる。確認が出るか拒まれた場合はマージせず、設計書 7.3 に従い設計書と ADR-0046 に戻って方針を決め直す |
| `mergemain.md` の記述が `TestMergemainCommandLinesAreListed` に抵触する | `make test` と CI の `doc-guard` が落ちる | 設計書 1.2・3.5 に従い、`make ext-install` は囲みコードブロックではなく手順 7 の本文に書き、ステップ 1-2 の `make test` で確かめる |
| ステップ 1-3 の追加コミットで Diff record と乖離する | `/runpr` の PRE_MERGE 照合で止まる | ステップ 1-3 で変えるファイルは本計画書と ADR-0046 のみとする。やむを得ず `CLAUDE.md` または `mergemain.md` を修正した場合は Diff record を取り直す |
| 検証用 PR や一時ブランチが GitHub に残る | リポジトリに不要な PR・ブランチが残る | ステップ 1-3 の完了条件に検証用 PR のクローズと一時ブランチの削除を含める |

### スケジュールリスク

-   ステップ 1-3 は人の手動操作とネットワーク（`registry.npmjs.org`）への接続を要する。PR 作成後・マージ前の工程として切り出すことで、ステップ 1-1・1-2 の実施と PR 作成が人の待機で止まらないようにする。

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1 完了
    -   [ ] ステップ 1-1: `mergemain.md`・`CLAUDE.md`・ADR-0046・`harness-antigravity.md` の更新を 1 コミットで完了
    -   [ ] ステップ 1-2: `make test && make lint` 通過、AC-07・AC-08 の Diff record 記入と照合完了、実装 PR 作成
    -   [ ] ステップ 1-3: 一時的な検証用 PR による実地の確かめ完了、§9 記入、ADR-0046「結果」更新、`make test && make lint` 再確認、検証用 PR・一時ブランチ削除

## 7. 完了基準 (Success Criteria)

-   **機能完了基準:** 要件書の AC-01〜AC-11 と設計書 1.2・3.7 の Antigravity アダプタ変更がすべて実装・検証されていること。
-   **品質基準:** `make test && make lint` がすべて通り、§8 の Diff record が `requirements_process.md` §4「Rechecking a record」の照合に合格していること。
-   **セキュリティ検証要件:** `.claude/settings.json`・`Makefile`・`mergepr` が変更されていないこと、`cmd/yt2column/settings_test.go` と `cmd/mergepr/docs_test.go` のガードテストが通ること、ADR-0046 の食い違いの文が消え「結果」が実地の確かめの結果に合わせて更新されていること。
-   **文書完了基準:** 本計画書のチェックボックス・§8 の Diff record・§9 の実地の確かめの記録がすべて埋まっており、実装 PR の説明に実地の確かめの結果が記されていること。

## 8. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 要件の要約 | 対応ステップ | 検証種別 | 検証方法・対象 |
|---|---|---|---|---|
| AC-01 | 拡張の検査が要る取り込みで `make test`・`make lint` の後、`make ext-check` の前に毎回 `make ext-install` を走らせる | ステップ 1-1、1-3 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 7 を読み、ステップ 1-3 の実地の確かめ（§9）で `extension/node_modules` なしから `make ext-check` まで通ることを確認する |
| AC-02 | 拡張の検査が要らない取り込みでは `make ext-install` を走らせない | ステップ 1-1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 7 の分岐条件と「要らない」ときの記述を読む |
| AC-03 | `make ext-install` の失敗・打ち切り時は `make ext-check` を走らせず戻して「検査の準備がない」で止まり、報告の詳細に 2 点を書く | ステップ 1-1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の止まる理由の対応・手順 7・手順 10 を読む |
| AC-04 | `make ext-install` 通過後の `make ext-check` 失敗時は戻して「検査の失敗」で止まる | ステップ 1-1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 7 の `make ext-check` 失敗時の記述を読む |
| AC-05 | auto mode で `make ext-install` が許可の確認待ちや判定による拒否なく実行される | ステップ 1-1、1-3 | `manual` | ステップ 1-3 の実地の確かめ（§9）で、auto mode の pane での `/mergemain` 実行時に確認が出ず、transcript に `make ext-install` の実行出力が残ることを確認する |
| AC-06 | 「解消した」の報告に `make ext-install` を走らせたかどうかを書く | ステップ 1-1、1-3 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 10 を読み、ステップ 1-3 の実地の確かめ（§9）で「拡張の依存」項目が出力されることを確認する |
| AC-07 | `CLAUDE.md` と `mergemain.md` に `make ext-install` の承認を書き、`CLAUDE.md` の差分はその項に限る | ステップ 1-1、1-2 | `manual`, `diff` | `manual`: PR レビューで承認の文言を読む。`diff`: 下の `Diff record (AC-07)`（recorded diff reading、比較対象 `CLAUDE.md`）で差分が `/mergemain` の項に限ることを検証する |
| AC-08 | 禁止事項から手順 7 の `make ext-install` を外しつつ手順 7 外と別手段を禁じ、`mergemain.md` の差分を 5 か所に限る | ステップ 1-1、1-2 | `manual`, `diff`, `static` | `manual`: PR レビューで禁止事項の 9 を読む。`diff`: 下の `Diff record (AC-08)`（recorded diff reading、比較対象 `.claude/commands/mergemain.md`）で差分が設計書 3.1 の 5 か所に限ることを検証する。`static`: `make test`（`cmd/mergepr/docs_test.go::TestMergemainCommandLinesAreListed`、`TestMergemainNoDirectGitMutation`） |
| AC-09 | 止まって戻したとき `extension/node_modules` を消さず、それ以外は 0115 AC-28 の状態にする | ステップ 1-1、1-2 | `manual` | PR レビューの変更ファイル一覧で `internal/mergepr/mergemain_restore.go` と `Makefile` を変更していないことを確認する（設計書 4.2・7.5） |
| AC-10 | ブランチが `Makefile` 等を変えていないとき、変わる既存の無視ファイルは `extension/node_modules` と `make ext-check` の出力に限る | ステップ 1-1、1-2 | `manual` | PR レビューの変更ファイル一覧で `Makefile`・`extension/package.json`・`extension/scripts/check-lockfile.ts` を変更していないことを確認する（設計書 5.2・7.5） |
| AC-11 | `make ext-install` 通過後に止まって戻したとき、報告の詳細に 2 点を書く | ステップ 1-1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 10 の「止まった」の記述を読む |
| （設計書 1.2・3.7） | Antigravity アダプタの昇格可能コマンド一覧に `make ext-install`（定義が指示するときに限る）を 1 項目足す | ステップ 1-1、1-2 | `manual`, `static` | `manual`: PR レビューで `.claude/commands/_lib/harness-antigravity.md` の差分が 4 章の昇格の一覧の 1 項目に限ることを確認する。`static`: `make test`（`cmd/yt2column/skills_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`） |

Diff record (AC-07):

- Compared paths: `CLAUDE.md`

Diff record (AC-08):

- Compared paths: `.claude/commands/mergemain.md`

## 9. 実地の確かめの記録

ステップ 1-3 の実施後、人がここに記録を記入する（設計書 7.3）。

-   **実施日:** （未実施）
-   **対象コミット（実装 PR の head）:** （未実施）
-   **検証用 PR:** （未実施）
-   **前提の確認:** 利用者の設定に `make` の許可規則がないこと／worktree に `extension/node_modules` がないこと
-   **実施回数と各回の結果:**
    -   （回数・確認画面の有無・transcript での `make ext-install` 実行出力の有無・`make ext-check` の結果・報告の「拡張の依存」項目の有無を記録する）
-   **合わせて観測したこと:**
    -   確認を出すモードで人が断ったときの振る舞い: （未実施）
    -   auto mode の判定が拒んだ回の手順 9 への遷移（該当があれば）: （未実施）
-   **Antigravity での確認（任意・機会があれば記録）:** （未実施）

## 10. 次のステップ (Next Steps)

1.  本計画書の PR レビューを受け、`/runpr` で承認（`approved` への更新とマージ）を行う。
2.  `/runplan 0130` を実行し、ステップ 1-1・1-2 を実施して実装 PR を作成する。
3.  実装 PR のマージ前に人がステップ 1-3（実地の確かめ、§9 の記録、ADR-0046「結果」の更新）を実施し、検証用 PR と一時ブランチを片付けたうえで `/runpr` でマージする。
