# 実装計画書：`/mergemain` は拡張の検査の前に `make ext-install` で依存を入れる

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-27 |
| Review date | 2026-11-27 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）および [02_architecture.md](02_architecture.md)（アーキテクチャ設計書。以下、設計書）に基づく実装計画である。既存ファイルの記述は、コミット `0dcdfc0c` の内容で確かめた。`file:line` はこのコミットの行番号を指す。

## 1. 実装の概要 (Implementation Overview)

**目的:** `/mergemain` が拡張の検査が要る取り込みで `make ext-check` の前に `make ext-install` を走らせるよう、コマンド定義と関連文書を更新し、既存のガードテストと実地の確かめで受け入れ基準（AC-01〜AC-11）を検証する。

**実装方針:**

-   設計書 1 章・8 章に従い、変更対象は `.claude/commands/mergemain.md`、`CLAUDE.md`、`docs/dev/adr/0046-chief-of-staff.md`、`.claude/commands/_lib/harness-antigravity.md` の 4 文書のみとする。Go コード、`Makefile`、`.claude/settings.json`、ガードテストは変更しない。
-   上記 4 文書の初期変更は 1 つのコミットにまとめ、`mergemain.md` と `CLAUDE.md` の承認の不整合、および Antigravity での昇格対象の不整合を作らない（設計書 8 章）。
-   AC-07・AC-08 の差分が定められた範囲に限られることは、差分が小さいため一時スクリプト（`_checks/`）を作らず、`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の記録した差分の読み取り（recorded diff reading）で検証する。
-   設計書 7.3 の実地の確かめ（auto mode での `/mergemain` 実行）は、実装 PR を開いた後・マージの前に人が一時的な検証用 PR で実施し、その結果の記録と ADR-0046 の「結果」の更新を同じ実装 PR に追加する。その手順は実装ステップ（§2）に置かず、チェックボックスのない人の手順として §9.1 に置く。`/runplan` はこれを実行せず、PR の説明にマージ前に人が行う項目として書く。

### 1.1. 既存コード調査結果

コミット `0dcdfc0c` で各ファイルを調べた結果は次のとおりである。`implementation_handoff.md` は存在しない。`design_handoff.md` の H-01〜H-03 はすべて設計書の付録 A に方針が記録済みである。

| ファイル | 確認した箇所（`0dcdfc0c`） | 現状と本タスクでの扱い |
|---|---|---|
| `.claude/commands/mergemain.md` | 承認（`:16-28`）、禁止事項 9（`:78-80`）、止まる理由の対応（`:110`）、手順 7（`:251-267`）、手順 10（`:305-336`） | 承認は依存のインストールを明示的に除外し、禁止事項 9 は `make ext-install` を禁じ、手順 7 は `make ext-preflight` → `make ext-check` を本文の箇条で指示している。設計書 3.1 の 5 か所のみを更新する |
| `CLAUDE.md` | 「Tool Execution Safety」の `/mergemain` の項（`:1259`） | 4 つのネットワーク操作のみを承認している。設計書 3.2 に従い、拡張の検査が要る取り込みでの `make ext-install` の承認を足す |
| `docs/dev/adr/0046-chief-of-staff.md` | 食い違いの文（`:85`）、「結果」の未確認・拒否時の文（`:515`） | `:85` に `mergemain.md`・`CLAUDE.md` との食い違いの文が残っている。`:515` に auto mode の判定が未確認である旨と、人が確認を断ったときの想定がある。ステップ 1-1 で `:85` を消し、マージ前の実地の確かめ（§9.1）の後に `:515` を観測結果に合わせて更新する（設計書 3.4） |
| `.claude/commands/_lib/harness-antigravity.md` | 4 章「Sandbox Elevation Rules」の 2（`:48-55`） | 昇格してよいネットワークコマンドの例に `make ext-install` がない。設計書 3.7 に従い 1 項目を足す |
| `.claude/settings.json` | 全体（`:1-54`、`mergemain-restore` の `ask` 規則は `:14`） | `make` の `allow` 規則はない。変更しない（設計書 1.2・3.3） |
| `Makefile` | `ext_preflight`（`:407-413`）、`ext-preflight`（`:417-418`）、`ext-install`（`:425-428`）、`ext-typecheck`〜`ext-build`（`:430-458`） | `ext-install` が `ext_check_versions`、`check-lockfile`、`npm ci --ignore-scripts --registry=https://registry.npmjs.org/ --replace-registry-host=never` を順に実行する。変更しない（要件書 2.3） |
| `cmd/yt2column/settings_test.go` | `unpromptedWords`（`:31-36`）、`promptingModes`（`:40-45`） | `.claude/settings.json` を変えないため変更不要（設計書 3.5） |
| `cmd/mergepr/docs_test.go` | `TestMergemainNoDirectGitMutation`（`:530-535`）、`mergemainLinePrefixes`（`:543-547`）、`unlistedCommandLines`（`:561-584`） | 囲みコードブロックの行のみを読む。`make ext-install` は手順 7 の本文に書くため変更不要（設計書 1.2・3.5） |

### 1.2. 計画の際に決めたこと（2026-11-27）

人間のレビュアーが決めたもの:

-   **設計書 7.3 の実地の確かめの用意の仕方:**
    -   **決定:** 一時的な検証用 PR を使う。`main` 上で `extension/`・`README.md`・`README.en.md`・`docs/dev/guides/development.md` のいずれかを変更した直近コミットの親から一時ブランチを切り、実装 PR の `.claude/commands/mergemain.md` と `CLAUDE.md` だけを取り込んで draft の検証用 PR を作り、確かめの後に閉じて一時ブランチと検証用の worktree を消す。
    -   **理由:** 実装 PR の履歴に検証用の merge コミットを残さず、`main` の進み具合に依存せずに拡張の検査が要る取り込みを確実に再現できるためである。また、ADR-0046 の変更を含めないことで、親コミット以降の ADR 更新との不要な衝突や ADR チェックによる停止を避けられる（設計書 7.3 が求めるのは実装 PR の `mergemain.md` と `CLAUDE.md` のみ）。
    -   **人の答え:** 一時的な検証用 PR を使う（推奨案）。
    -   **ADR:** なし。本タスクの検証手順の中で閉じ、ほかのタスクや構成要素を縛らないためである。
-   **設計書 7.3 の実地の確かめを行うタイミング:**
    -   **決定:** 実装 PR を開いた後・マージの前に人が行い、結果の記録（本書 §9）と ADR-0046 の「結果」の更新を同じ実装 PR に追加する。
    -   **理由:** 実地の確かめには人の操作が要るため、PR 作成前に入れると `/runplan` の途中で止まって自動送信を `hold` にする必要がある。PR 作成後・マージ前に置けば、`/runplan` は文書更新・ガードテスト・`diff` 記録・PR 作成までを一貫して進められる。そのため、この確かめは `/runplan` が選ぶ実装ステップのチェックボックスにせず、§9.1 の人の手順とする（計画で決めたもの「マージ前の人の手順の置き場所」）。
    -   **人の答え:** PR を開いた後・マージの前に人が行い、記録と ADR-0046 の更新を同じ PR に足す（推奨案）。
    -   **ADR:** なし。本タスクの進め方の中で閉じ、ほかのタスクや構成要素を縛らないためである。

計画で決めたもの:

-   **AC-07・AC-08 の `diff` 検証の形:** 変更箇所が `CLAUDE.md` の 1 か所と `mergemain.md` の 5 か所に限られ差分が小さいため、一時スクリプト（`_checks/`）ではなく、`requirements_process.md` §4 の記録した差分の読み取り（recorded diff reading）を採る。ADR: なし（本タスクの検証手順の中で閉じるため）。
-   **実地の確かめの記録の置き場所:** 本書の §9「実地の確かめの記録」に置く。ADR: なし（同上）。
-   **マージ前の人の手順の置き場所:** 実地の確かめの手順は、実装ステップのチェックボックスにせず、§9.1 にチェックボックスのない番号付きの手順として置く。`/runplan` は、計画のすべてのフェーズが完了したときだけ PR を開き、未完了のチェックボックスは次の作業として選ぶ（`.claude/commands/runplan.md` 手順 4）。人の手順を `[ ]` のまま実装ステップに置くと、`/runplan` がそれを実行しようとし、`自動送信: auto` では人の見ていないセッションが一時ブランチの push・検証用 PR の作成・ブランチの削除を行いうるうえ、設計書 7.3 が求める「人が新しい pane で `/mergemain <PR>` だけを打つ」形にもならない。`/runplan` は、PR の説明にマージ前に人が行う項目として §9.1 を示す（0093 実装計画書の決定 6 と同じ形）。ADR: なし（本タスクの進め方の中で閉じるため）。
-   **ADR-0046「結果」の更新の扱い:** マージ前の実地の確かめの後に ADR-0046 の「結果」を観測結果で更新する変更は、新しい事実を加えるので、`docs/dev/conventions/adr.md` の「Editorial corrections」の 4 種類のどれにも当たらない。同書の「迷ったら決定の変更として扱う」に従い決定の変更として扱い、同じコミットで ADR-0046 の「経緯」に 1 行を足す。訂正行と PR の説明の訂正段落は書かない。ADR: なし（ADR の書き方の規則に従うだけで、新しい方針ではないため）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 文書・コマンド定義の更新と確かめ

設計書 8 章の優先順位に従い、1 つの PR で実施する。フェーズ 1 のステップ（1-1・1-2）はすべて `/runplan` が実行し、完了したら実装 PR を開く。マージ前に人が行う実地の確かめは実装ステップではなく、§9.1 の手順として人が PR 作成後に実施し、同じ PR に追加する。

#### ステップ 1-1: コマンド定義・CLAUDE.md・ADR-0046・Antigravity アダプタの更新

-   **変更対象ファイル:**
    -   `.claude/commands/mergemain.md`
    -   `CLAUDE.md`
    -   `docs/dev/adr/0046-chief-of-staff.md`
    -   `.claude/commands/_lib/harness-antigravity.md`
-   **作業内容:**
    -   [x] `.claude/commands/mergemain.md` の 5 か所（承認の節、禁止事項の 9、止まる理由の対応の「検査の準備がない」の行、手順 7、手順 10 の報告の項目）を設計書 3.1 の仕様どおり更新する（手順 7 の `make ext-install` は囲みコードブロックではなく本文に書き、`make ext-preflight` を除く）
    -   [x] `CLAUDE.md` の「Tool Execution Safety」の `/mergemain` の項（`CLAUDE.md:1259`）に、拡張の検査が要る取り込みでの `make ext-install` の承認を足す（設計書 3.2）
    -   [x] `docs/dev/adr/0046-chief-of-staff.md` の「決定」にある食い違いの文（`0046-chief-of-staff.md:85`）を消す（設計書 3.4）
    -   [x] `.claude/commands/_lib/harness-antigravity.md` の 4 章「Sandbox Elevation Rules」の 2（`harness-antigravity.md:48-55`）の昇格してよいコマンドの種類に、`make ext-install`（コマンドの定義が指示するときに限る）をネットワーク通信の例として足す（設計書 3.7）
    -   [x] 上記 4 ファイルを 1 つのコミットにまとめる（設計書 8 章）
-   **完了条件:**
    -   変更ファイルが上記 4 文書のみで、`.claude/settings.json`・`Makefile`・Go コード・ガードテストに変更がないこと。

#### ステップ 1-2: ガードテストの実行と `diff` 記録の作成

-   **変更対象ファイル:**
    -   `docs/tasks/0130_mergemain_ext_install/03_implementation_plan.md`（§8 の Diff record の記入）
-   **作業内容:**
    -   [x] `make test` と `make lint` を実行し、既存のガードテスト（`cmd/mergepr/docs_test.go`、`cmd/yt2column/settings_test.go`、`cmd/yt2column/skills_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go` を含む）がすべて通ることを確かめる（設計書 3.5・7.1）
    -   [x] `requirements_process.md` §4「`diff` verification」の固定コマンドで AC-07（`CLAUDE.md`）と AC-08（`.claude/commands/mergemain.md`）の差分を取得し、本書の §8 の各 Diff record に記入してコミットする
    -   [x] PR を開く前に、`requirements_process.md` §4「Rechecking a record」の手順で Diff record を最新コミットと照合し、一致することを確かめる
    -   [x] 実装 PR の説明に、マージ前に人が行う項目として §9.1 の実地の確かめ（AC-01・AC-05・AC-06 の確かめ、§9.2 の記録、ADR-0046「結果」と「経緯」の更新、検証用 PR・一時 worktree・一時ブランチの片付け）を書く。`/runplan` は §9.1 の手順を実行しない
-   **完了条件:**
    -   `make test && make lint` が通り、§8 の Diff record が照合に合格していること。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 実装順序

設計書 8 章の順序どおり、単一フェーズ・単一 PR で進める。

1.  **マイルストーン 1（実装 PR の作成）:** ステップ 1-1（4 文書の更新を 1 コミット）→ ステップ 1-2（`make test && make lint` と AC-07・AC-08 の Diff record 記入）→ 実装 PR の作成。
2.  **マイルストーン 2（実地の確かめとマージ準備）:** 人が §9.1 の手順（一時的な検証用 PR での実地の確かめ、§9.2 の記録、ADR-0046「結果」と「経緯」の更新、実装 PR 説明の追記、`make test && make lint` の再確認、検証用 PR・一時 worktree・一時ブランチの片付け）を実施 → `/runpr` でのマージ。

### 3.2. PR 構成

本計画は単一フェーズであるため、PR マーカーは設けず、1 本の PR で実施する。ステップ 1-1・1-2 を `/runplan` が実行して PR を開き、§9.1 の手順はマージの前に人が実施して同じ PR に追加する。

**実装モデル要件**: high

**判定理由**: ステップ 1-1 は、`CLAUDE.md` と `mergemain.md` の承認を広げると同時に `mergemain.md` の禁止事項 9 を狭め、`harness-antigravity.md` の昇格の一覧も変える、開発ツールの信頼の境界のゲートを変えるステップであり、`.claude/commands/mkplan2.md` の `high` の契機（`mkplan.md` 手順 8 の security-gate の変更、振る舞いを同時に広げて狭める変更）に当たるため。変更は 4 文書の小さな編集で、上位モデルで実装する費用は小さい。

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   **単体テスト・ガードテスト:** Go コードの変更はなく、追加する単体テストやテストヘルパーファイルはない（`docs/dev/conventions/test_organization.md` に該当する追加なし）。`make test && make lint` により、既存の文書・設定ガードテスト（`cmd/mergepr/docs_test.go`、`cmd/yt2column/settings_test.go`、`cmd/yt2column/skills_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`）がすべて通ることを確かめる（設計書 3.5・7.1）。
-   **結合テスト:** なし（設計書 7.2）。
-   **実地の確かめ:** 設計書 7.3 に従い、マージ前に人が §9.1 の手順で一時的な検証用 PR を用いて実施し、§9.2 に記録する。確認を出すモード（`default` など）の pane では確認画面で断ったときの振る舞いを確かめ、auto mode の pane では `make ext-install` の確認なし実行（transcript 確認）・`make ext-check` 通過・報告項目を確かめる。Antigravity での実地の確かめは求めず、機会があれば §9.2 に追記する（設計書 7.3）。
-   **文書の確かめ・`diff` 検証:** 文言の意味は PR レビュー（`manual`）で読む。AC-07・AC-08 の差分が定められた範囲に限られることは §8 の Diff record（recorded diff reading）で検証し、一時スクリプト（`_checks/`）は作らない。
-   **後方互換性:** chief が `/mergemain` を送るのはブランチの `mergemain.md` が `origin/main` と同じときに限られるため（要件書 4.4）、本実装が `main` に入った後に取り込まれるブランチから新しい手順が適用される。既存のガードテストと `mergepr` のプロトコル番号（4）は変わらない。

## 5. リスク管理 (Risk Management)

### 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| auto mode の判定が `make ext-install` に確認を出すか拒む | AC-05 を満たせず、chief の自動送信で止まる | §9.1 の手順でマージ前に実地で確かめる。確認が出るか拒まれた場合はマージせず、設計書 7.3 に従い設計書と ADR-0046 に戻って方針を決め直す |
| `mergemain.md` の記述が `TestMergemainCommandLinesAreListed` に抵触する | `make test` と CI の `doc-guard` が落ちる | 設計書 1.2・3.5 に従い、`make ext-install` は囲みコードブロックではなく手順 7 の本文に書き、ステップ 1-2 の `make test` で確かめる |
| §9.1 の追加コミットで Diff record と乖離する | `/runpr` の PRE_MERGE 照合で止まる | §9.1 で変えるファイルは本計画書と ADR-0046 のみとする。やむを得ず `CLAUDE.md` または `mergemain.md` を修正した場合は Diff record を取り直す |
| 検証用 PR・一時 worktree・一時ブランチが残る | 不要な PR・ブランチ・worktree が残る | §9.1 の完了条件に検証用 PR のクローズと一時 worktree・一時ブランチの削除を含める |

### スケジュールリスク

-   §9.1 の手順は人の手動操作とネットワーク（`registry.npmjs.org`）への接続を要する。PR 作成後・マージ前の工程として切り出すことで、ステップ 1-1・1-2 の実施と PR 作成が人の待機で止まらないようにする。§9.1 は `/runplan` が選ぶチェックボックスを持たないので、`自動送信: auto` で送られた `/runplan` がこれを実行することはない。

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] フェーズ 1 完了
    -   [x] ステップ 1-1: `mergemain.md`・`CLAUDE.md`・ADR-0046・`harness-antigravity.md` の更新を 1 コミットで完了
    -   [x] ステップ 1-2: `make test && make lint` 通過、AC-07・AC-08 の Diff record 記入と照合完了、PR の説明に §9.1 の人の項目を記載、実装 PR 作成
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1200
- [x] PR がマージされた

## 7. 完了基準 (Success Criteria)

-   **機能完了基準:** 要件書の AC-01〜AC-11 と設計書 1.2・3.7 の Antigravity アダプタ変更がすべて実装・検証されていること。
-   **品質基準:** `make test && make lint` がすべて通り、§8 の Diff record が `requirements_process.md` §4「Rechecking a record」の照合に合格していること。
-   **セキュリティ検証要件:** `.claude/settings.json`・`Makefile`・`mergepr` が変更されていないこと、`cmd/yt2column/settings_test.go` と `cmd/mergepr/docs_test.go` のガードテストが通ること、ADR-0046 の食い違いの文が消え「結果」が実地の確かめの結果に合わせて更新されていること。
-   **文書完了基準:** 本計画書のチェックボックスと §8 の Diff record がすべて埋まっていること。マージの前に、人が §9.1 を終え、§9.2 の実地の確かめの記録が埋まり、実装 PR の説明に実地の確かめの結果が記されていること。

## 8. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 要件の要約 | 対応ステップ | 検証種別 | 検証方法・対象 |
|---|---|---|---|---|
| AC-01 | 拡張の検査が要る取り込みで `make test`・`make lint` の後、`make ext-check` の前に毎回 `make ext-install` を走らせる | ステップ 1-1、§9.1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 7 を読み、§9.1 の実地の確かめ（§9.2）で `extension/node_modules` なしから `make ext-check` まで通ることを確認する |
| AC-02 | 拡張の検査が要らない取り込みでは `make ext-install` を走らせない | ステップ 1-1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 7 の分岐条件と「要らない」ときの記述を読む |
| AC-03 | `make ext-install` の失敗・打ち切り時は `make ext-check` を走らせず戻して「検査の準備がない」で止まり、報告の詳細に 2 点を書く | ステップ 1-1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の止まる理由の対応・手順 7・手順 10 を読む |
| AC-04 | `make ext-install` 通過後の `make ext-check` 失敗時は戻して「検査の失敗」で止まる | ステップ 1-1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 7 の `make ext-check` 失敗時の記述を読む |
| AC-05 | auto mode で `make ext-install` が許可の確認待ちや判定による拒否なく実行される | ステップ 1-1、§9.1 | `manual` | §9.1 の実地の確かめ（§9.2）で、auto mode の pane での `/mergemain` 実行時に確認が出ず、transcript に `make ext-install` の実行出力が残ることを確認する |
| AC-06 | 「解消した」の報告に `make ext-install` を走らせたかどうかを書く | ステップ 1-1、§9.1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 10 を読み、§9.1 の実地の確かめ（§9.2）で「拡張の依存」項目が出力されることを確認する |
| AC-07 | `CLAUDE.md` と `mergemain.md` に `make ext-install` の承認を書き、`CLAUDE.md` の差分はその項に限る | ステップ 1-1、1-2 | `manual`, `diff` | `manual`: PR レビューで承認の文言を読む。`diff`: 下の `Diff record (AC-07)`（recorded diff reading、比較対象 `CLAUDE.md`）で差分が `/mergemain` の項に限ることを検証する |
| AC-08 | 禁止事項から手順 7 の `make ext-install` を外しつつ手順 7 外と別手段を禁じ、`mergemain.md` の差分を 5 か所に限る | ステップ 1-1、1-2 | `manual`, `diff`, `static` | `manual`: PR レビューで禁止事項の 9 を読む。`diff`: 下の `Diff record (AC-08)`（recorded diff reading、比較対象 `.claude/commands/mergemain.md`）で差分が設計書 3.1 の 5 か所に限ることを検証する。`static`: `make test`（`cmd/mergepr/docs_test.go::TestMergemainCommandLinesAreListed`、`TestMergemainNoDirectGitMutation`） |
| AC-09 | 止まって戻したとき `extension/node_modules` を消さず、それ以外は 0115 AC-28 の状態にする | ステップ 1-1 | `manual` | PR レビューの変更ファイル一覧で `internal/mergepr/mergemain_restore.go` と `Makefile` を変更していないことを確認する（設計書 4.2・7.5） |
| AC-10 | ブランチが `Makefile` 等を変えていないとき、変わる既存の無視ファイルは `extension/node_modules` と `make ext-check` の出力に限る | ステップ 1-1 | `manual` | PR レビューの変更ファイル一覧で `Makefile`・`extension/package.json`・`extension/scripts/check-lockfile.ts` を変更していないことを確認する（設計書 5.2・7.5） |
| AC-11 | `make ext-install` 通過後に止まって戻したとき、報告の詳細に 2 点を書く | ステップ 1-1 | `manual` | PR レビューで `.claude/commands/mergemain.md` の手順 10 の「止まった」の記述を読む |
| （設計書 1.2・3.7） | Antigravity アダプタの昇格可能コマンド一覧に `make ext-install`（定義が指示するときに限る）を 1 項目足す | ステップ 1-1、1-2 | `manual`, `static` | `manual`: PR レビューで `.claude/commands/_lib/harness-antigravity.md` の差分が 4 章の昇格の一覧の 1 項目に限ることを確認する。`static`: `make test` で既存のガードテスト（`cmd/yt2column/skills_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`）が壊れていないことを確認する（非回帰確認） |

Diff record (AC-07):

- Compared paths: `CLAUDE.md`
- Merge-base: `781a2373c4c617cb8061e24b6723842ce7364f22`
- Compared commit: `69eaf2d61a05cea9cb9f29e681b31a26af67bf81`

Path diff:

````text
diff --git a/CLAUDE.md b/CLAUDE.md
index b35691a2b3fa60a089686c78491204fbde3da26a..3479fb49425bc81b7904da0617644a23b9c9e3f0 100644
--- a/CLAUDE.md
+++ b/CLAUDE.md
@@ -1256,7 +1256,7 @@ for the agents that read it, and the Antigravity skill entries point here.
   `git push` may be executed without approval. Changing, disabling, or
   deleting the record ruleset that protects these tags also requires the
   user's explicit approval
-- Invoking `/mergemain` (whether typed by a human or sent by `chief`) approves, for that PR only, the network operations it runs through `mergepr`: `gh pr view`, `git fetch origin`, `git ls-remote` on the PR's head branch, and `git push` (non-force) to the PR's head branch. It approves no merge (`gh pr merge`), no PR review request, no write to a PR or review thread, no `numreserve` call, and no push to any branch other than the PR's head branch
+- Invoking `/mergemain` (whether typed by a human or sent by `chief`) approves, for that PR only, the network operations it runs through `mergepr`: `gh pr view`, `git fetch origin`, `git ls-remote` on the PR's head branch, and `git push` (non-force) to the PR's head branch. When the take-in requires extension checks, it also approves running `make ext-install` (including fetching dependencies from `registry.npmjs.org`), only in step 7 of `mergemain.md`, before any restore, and only as the bare command (no arguments, variables, or chaining); it never approves it after a restore. It approves no merge (`gh pr merge`), no PR review request, no write to a PR or review thread, no `numreserve` call, and no push to any branch other than the PR's head branch
 - Invoking `/chief` approves, for that session, the read-only network operations
   the `chief` tool runs while it shows the status, watches, makes a task's
   workspace, removes one, or sends a command the human chose: `git fetch origin
````

Diff record (AC-08):

- Compared paths: `.claude/commands/mergemain.md`
- Merge-base: `781a2373c4c617cb8061e24b6723842ce7364f22`
- Compared commit: `ae87023f38825f952c4171b97ca0281bd42d634d`

Path diff:

````text
diff --git a/.claude/commands/mergemain.md b/.claude/commands/mergemain.md
index e3356a3fcd26569225fa31c73238b82bfa0a7680..f10a6fb84b4f900c06d3e0e82ea2e517b4c3ab45 100644
--- a/.claude/commands/mergemain.md
+++ b/.claude/commands/mergemain.md
@@ -24,8 +24,13 @@ for this PR only, the four network operations performed through `mergepr`:
 - `git ls-remote` on the PR's head branch
 - `git push` (non-force) to the PR's head branch

-It approves no other network operation, no merge, no PR write, and no dependency
-installation (`CLAUDE.md`, "Tool Execution Safety").
+In addition, when step 7 determines that extension checks are needed for this
+take-in, it approves running `make ext-install` (including fetching
+dependencies from `registry.npmjs.org`) strictly in step 7's bare standalone
+form (with no arguments, variables, or chaining).
+
+It approves no other network operation, no merge, no PR write, and no other
+dependency installation (`CLAUDE.md`, "Tool Execution Safety").

 ## Tool contract and line protocol

@@ -76,8 +81,13 @@ Never do any of the following during `/mergemain`:
    state, head branch, head commit, base branch, and whether the head belongs to
    another repository.
 9. Never run `git merge`, `git pull`, `git reset`, or `git push` directly (and never run
-   `git clean`, `git merge --abort`, or `make ext-install`). Perform taking in,
-   restoring, and pushing strictly through the `mergepr` take-in subcommands.
+   `git clean` or `git merge --abort`). Perform taking in, restoring, and
+   pushing strictly through the `mergepr` take-in subcommands. Never run
+   `make ext-install` outside step 7 (including after restoring in step 9),
+   never install dependencies by any other means (such as `npm ci`,
+   `npm install`, or `npx` under `extension/`), and when `make ext-install`
+   is denied or fails, never try another way to install dependencies or ask
+   the human yourself (the harness's own permission prompt is excluded).

 **Temporary files.** Never create temporary or working files inside the git work
 tree. Put any temporary files in the session scratchpad outside the work tree
@@ -107,7 +117,7 @@ step 9 before reporting:
 - `ADR の衝突` (ADR number/subject collision, discrepancy statement requiring re-check, or irreconcilable same-ADR edit)
 - `README の同期の失敗` (`mktrans.md` sub-step stopped or sync check failed unexpectedly)
 - `検査の失敗` (`make test`, `make lint`, `make ext-check`, or `git commit --no-edit` failed)
-- `検査の準備がない` (`make ext-preflight` failed)
+- `検査の準備がない` (`make ext-install` failed, timed out, or was denied by permission evaluation or the human)

 ## Procedure

@@ -261,11 +271,37 @@ applied to each, and a short summary of the resolution for step 10.

      If any listed path is under `extension/` or equals `README.md`,
      `README.en.md`, or `docs/dev/guides/development.md`:
-     1. Run `make ext-preflight` (never run `make ext-install`). If it fails,
-        run step 9 and go to step 10 with reason `検査の準備がない`.
+     1. Run `make ext-install` from the repository top level as a bare,
+        standalone command (no arguments, variables, or chaining) in the
+        foreground with a 10-minute timeout (`_lib/harness.md`, `shell-run`),
+        whether or not `extension/node_modules` already exists and without
+        stopping because the PR branch modifies `extension/package.json`,
+        `extension/package-lock.json`, `extension/scripts/check-lockfile.ts`,
+        or `Makefile`. If it reaches the timeout and the harness moves it to
+        the background, stop that background execution using the harness's
+        stop operation (for example, `TaskStop` in Claude Code) and treat it
+        as timed out. Record the outcome for step 10 as one of: succeeded,
+        failed, timed out, or denied. If it did not succeed, do not run
+        `make ext-check` and do not retry it (an adapter's elevated rerun
+        inside `shell-run` after a sandbox failure does not count as a retry;
+        use the post-elevation result as the `shell-run` outcome, except if
+        that elevated rerun is denied after the sandboxed attempt already ran,
+        record the outcome as failed so step 10 warns that
+        `extension/node_modules` may be missing or incomplete): run
+        step 9 and go to step 10 with reason `検査の準備がない`.
+        - When permission evaluation denies it, proceed to step 9 without
+          asking the human.
+        - When the human denies it at a permission prompt, regardless of any
+          follow-up instruction the human gives (including text entered when
+          denying), do not alter the take-in procedure or install
+          dependencies by any other means: proceed to step 9 and report with
+          reason `検査の準備がない`.
      2. Run `make ext-check`. If it fails, run step 9 and go to step 10 with
         reason `検査の失敗`.

+     Otherwise, run neither `make ext-install` nor `make ext-check`, and
+     record the `make ext-install` outcome for step 10 as not run.
+
 8. **Push (`mergemain-push`).**
    Run as a standalone command:

@@ -323,12 +359,36 @@ applied to each, and a short summary of the resolution for step 10.
         none.
       - **詳細**: brief summary of what failed, whether the branch and work tree
         were restored to `<base-head>`, and (when `--protocol` mismatched) the
-        reinstall instruction (`make install-mergepr` from `main`).
+        reinstall instruction (`make install-mergepr` from `main`). Also
+        include, according to step 7's `make ext-install` outcome:
+        - when `make ext-install` failed or timed out: (1) that
+          `make ext-install` failed and whether it timed out, which stage
+          failed (version check, `check-lockfile`, or `npm ci`), and the last
+          few lines of the error; (2) that `extension/node_modules` may be
+          missing or incomplete, so the user must fix the cause and run
+          `make ext-install`; and (3) if timed out, that the user must verify
+          no `npm` process remains running under `extension/` before
+          reinstalling;
+        - when `make ext-install` was denied: that it did not run because
+          permission evaluation denied it or the human denied it at a prompt
+          (with the reason given by permission evaluation, if any), and that
+          `extension/node_modules` remains as it was before `/mergemain`
+          started;
+        - when `make ext-install` succeeded and step 9 restored the branch and
+          work tree (for example after `make ext-check` failed or
+          `push_rejected`): (1) that `make ext-install` ran; and (2) that
+          `extension/node_modules` matches the post-merge lockfile and may
+          differ from the restored branch's lockfile, so the user must re-run
+          `make ext-install` before working on the extension;
+        - when `make ext-install` succeeded and the branch was not restored
+          (for example `push_unknown` or step 9 overrode the stop reason to
+          `ほかの変更があり戻さなかった`): that `make ext-install` ran.
     - **When `解消した`:**
       - **変更したファイルと適用した規則**: for each file `/mergemain` modified
         (every conflicted file and every file modified by a take-in rule), state
         the rule applied (`README`・`ADR`・`プロトコル番号`・`両側の変更を保つ`)
         and a concise summary of how it was resolved, or `なし` if none.
+      - **拡張の依存**: state whether `make ext-install` was run.
       - **新しい head**: the pushed `new_head` commit ID.
       - **道具の入れ直し案内** (when any `protocol_tool:` line was reported):
         list each tool whose protocol number changed and state that the user
````

## 9. 実地の確かめの記録

### 9.1. マージ前に人が行う手順（`/runplan` は実行しない）

実装 PR を開いた後・マージの前に、人が次の手順を行う（§1.2「設計書 7.3 の実地の確かめを行うタイミング」「マージ前の人の手順の置き場所」）。`/runplan` が選ばないよう、チェックボックスは置かない。

-   **変更するファイル:**
    -   `docs/tasks/0130_mergemain_ext_install/03_implementation_plan.md`（§9.2 の記入）
    -   `docs/dev/adr/0046-chief-of-staff.md`（「結果」の更新と「経緯」の 1 行）
-   **手順:**
    1.  `main` 上で `extension/`・`README.md`・`README.en.md`・`docs/dev/guides/development.md` のいずれかを変更した直近コミットの親から一時ブランチを切り、実装 PR の `.claude/commands/mergemain.md` と `CLAUDE.md` のみを取り込んでコミットし、draft の検証用 PR を作成する（親コミット以降に `main` 側で `CLAUDE.md` や `mergemain.md` が変わっている場合は、`/mergemain` の手順 5 で `.md` 衝突解消が入ることも想定内とする）。
    2.  検証用の一時 worktree を作成して一時ブランチをチェックアウトし、利用者の設定（`~/.claude/settings.json` や worktree の `.claude/settings.local.json`）に `make` に当たる許可規則がないことと、その worktree に `extension/node_modules` がないことを確かめる（設計書 7.3）。
    3.  まず、確認を出すモード（`default` など）の Claude Code pane で `/mergemain <検証用 PR>` を実行し、`make ext-install` の確認画面で断ったときの振る舞い（エージェントの番が終わるか、および次の入力後に手順 9 で戻して「検査の準備がない」で止まるか）を観測し、検証用 worktree が `<base-head>` に戻り `extension/node_modules` がない状態であることを確かめる（設計書 1.2・7.3）。
    4.  続いて、新規または `/clear` 直後の auto mode の Claude Code pane で `/mergemain <検証用 PR>` のみを実行し、Claude Code のセッション記録（transcript）で `make ext-install` が許可の確認なしに実際に走ったこと、`make ext-check` まで通ったこと、報告に「拡張の依存」の項目が出たこと（および判定が拒んだ回があれば手順 9 への遷移）を確かめる（設計書 7.3）。
    5.  観測結果を §9.2 に記入し、設計書 3.4 に従い `docs/dev/adr/0046-chief-of-staff.md` の「結果」（`0046-chief-of-staff.md:515`）の「未確認である」の文と人が断ったときの振る舞いの文を更新する。この更新は決定の変更として扱い（§1.2「ADR-0046「結果」の更新の扱い」）、同じコミットで ADR-0046 の「経緯」に、確かめた日付・確かめた結果・本計画書へのリンクを書いた 1 行を足す。実装 PR にコミット・push する（確認が出るか拒まれた場合は、設計書 7.3 に従い設計書と ADR-0046 に戻って方針を決め直す）。
    6.  実装 PR の説明に、実地の確かめの結果を追記する。
    7.  `make test` と `make lint` が通ること、および §8 の Diff record が最新コミットと一致することを確かめる。
    8.  検証用 PR を閉じ、検証用の一時 worktree と一時ブランチ（ローカル・リモート）を削除する。
-   **完了条件:** auto mode で `make ext-install` が確認なしに実行され `make ext-check` まで通った記録が §9.2 にあり、ADR-0046 の「結果」と「経緯」、実装 PR の説明が更新され、`make test && make lint` と Diff record の照合が通り、検証用 PR・一時 worktree・一時ブランチが削除されていること。

### 9.2. 記録

§9.1 の実施後、人がここに記録を記入する（設計書 7.3）。

-   **実施日:** 2026-11-28
-   **対象コミット（実装 PR の head）:** `05196bad`
-   **検証用 PR:** #1203（draft。一時ブランチ `isseis/tmp-0130-verify` は、`69d08df7`（#1177）の親 `eb865fe6` から切り、実装 PR の `mergemain.md` と `CLAUDE.md` だけを取り込んだ `5db89a61` から始めた。取り込んだ `origin/main` は `e99525ec`）
-   **前提の確認:** `~/.claude/settings.json` の `permissions` に `make` に当たる規則はなく、プロジェクトの `.claude/settings.json` にあるのは `deny` の `Bash(make install-*)` だけで、`.claude/settings.local.json` はなかった。検証用の worktree に `extension/node_modules` はなかった。
-   **実施回数と各回の結果:**
    -   **1 回目（default のモード、確認の画面で人が断る）:** `make ext-install` の確認の画面が出て、人が断った。`make ext-install` は走らず、`make ext-check` も走らなかった。結果は「止まった」、理由の種類は「検査の準備がない」で、報告の「詳細」に、確認で拒否されて実行しなかったことと、`extension/node_modules` が始める前のままであることが出た。
    -   **2 回目（auto mode、`/clear` の直後の pane で `/mergemain 1203` だけを入力）:** 確認の画面は出なかった。transcript に `make ext-install` の実行の出力（`npm ci --ignore-scripts --registry=https://registry.npmjs.org/ …`、`added 141 packages`）があり、`make ext-check` は終了状態 0 で通った。結果は「解消した」で、報告に「拡張の依存: make ext-install を実行し、成功しました。」が出た。`mergemain-push` は `status: ok` で、新しい head は `92bbf1c5`。
-   **合わせて観測したこと:**
    -   確認を出すモードで人が断ったときの振る舞い: エージェントの番が終わって入力待ちになった。人が「続けて」と入力すると、依存を別の道で入れずに手順 9 へ進んだ。`mergepr mergemain-restore` は `.claude/settings.json` の `ask` の規則により確認を出し、人が許すと、ブランチと作業ツリーを `5db89a61` に戻して止まった。push はなく、作業ツリーに変更はなく、`extension/node_modules` もないままだった。
    -   auto mode の判定が拒んだ回の手順 9 への遷移（該当があれば）: 該当なし（判定が拒んだ回はなかった）。
-   **Antigravity での確認（任意・機会があれば記録）:** 行っていない。

## 10. 次のステップ (Next Steps)

1.  本計画書の PR レビューを受け、`/runpr` で承認（`approved` への更新とマージ）を行う。
2.  `/runplan 0130` を実行し、ステップ 1-1・1-2 を実施して実装 PR を作成する。
3.  実装 PR のマージ前に人が §9.1 の手順（実地の確かめ、§9.2 の記録、ADR-0046「結果」と「経緯」の更新、実装 PR 説明の追記）を実施し、検証用 PR・一時 worktree・一時ブランチを片付けたうえで `/runpr` でマージする。

## 変更履歴

- 2026-11-27 承認（isseis、#1199）
