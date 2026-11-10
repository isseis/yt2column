# 実装計画書：実装モデル要件を Opus・Sonnet の選択に直結させ、Opus のレビューを一度に行う

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。用語（実装モデル要件・正しさのレビュー・弱いモデルの失敗のレビュー・phase group・確認のレビュー・Opus のレビュー・旧い値・読めない値・読み替えの表・共通の手順・記録の行・実装の記録・完了の記録・残した指摘の記録・除いてよい commit・要否の判断・Opus の ID）は、要件書 6 章と設計書の冒頭と同じ意味で使う。本計画は、次の用語を加える。

-   **判断のスクリプト:** 要否の判断（設計書 3.5.2）を行うシェルスクリプト `.claude/commands/_lib/opus-review-decision.sh`。共通の手順が呼ぶ（§1.3 の決定 1）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

コマンドの定義・`_lib/` の共通の手順・`CLAUDE.md`・ADR の文言を変え、判断のスクリプトを加える。これにより、実装モデル要件の値から、セッションのモデルと Opus のレビューの要否が分かるようにし、`/runplan` が PR を開く前に Opus のレビューを行い、`/weakreview` が同じレビューを一度に行えるようにする（要件書 2.1 節）。何をどこに書くかは設計書 3 章が定める。本計画は、作業の順序と確かめ方を定める。

### 1.2. 実装の原則

-   **文面は設計書 3 章に従い、英語で書く。** コマンドの定義と `_lib/` は英語の文書である（要件書 5 章）。各ステップは、書く内容を設計書の節で指す。文面は実装の PR で書く。
-   **新しい恒久のテストは足さない**（設計書 7.1）。判断のスクリプトの振る舞いと、移したチェックリスト、旧い値の残りは、本タスクの `_checks/` の一回限りのスクリプトで確かめ、出力を §7 に記録する。`_checks/` は `make test` と CI で実行されない（`docs/dev/conventions/requirements_process.md` §4）。
-   **文書の自然言語の中身だけを定める AC は、PR のレビューで読んで確かめる**（設計書 7.3）。コマンドの定義の手順の文（案内、Opus のレビューの手順、`/weakreview` の振る舞い）がこれに当たる。値と構造（読み方、記録の行の形、旧い値の残り）は、実行する確かめを持つ。
-   **足す文は、コマンドの文書を読む既存の guard の制約を守る**（設計書 3.12、§1.4）。
-   **ADR の食い違いの記述を書き換える・消すことは、編集上の訂正である**（`docs/dev/conventions/adr.md`「Editorial corrections」の 2 つ目の種類）。その commit のメッセージに訂正の行を書く。PR の説明の訂正の段落は、`/runplan` が `_lib/task-pr-handoff.md` Part B で書く。

### 1.3. 計画の際に決めたこと（2026-11-11）

1.  **要否の判断のコマンドの組は、`_lib/` の判断のスクリプトに置き、共通の手順がそれを呼ぶ。確かめは、`_checks/` の一回限りのスクリプトで fixture のリポジトリを作って各場合を走らせ、出力を §7 に記録する。** 理由: 判断は、計画書を 2 か所から読む、完了の記録の END が `HEAD` の祖先かを見る、除いてよい commit を見分ける、各 commit が実装の記録で覆われるかを見る、と条件が多い。コードブロックをセッションが写して走らせると、写し誤りが起きうる。スクリプトなら、PR の中で同じものを確かめられる。恒久の Go テストは足さないので、設計書 7.1 と合う。設計書 2.2 の表と 3.5.2 にスクリプトのファイルを足すことは、決定を変えない編集上の訂正として、PR-2 で行う（ステップ 2-9）。
    -   採らなかった案: スクリプトに恒久の Go テストを足す案（`_lib/adr-correction-candidates.sh` と同じ形）。設計書 7.1 を変える決定の変更になり、テストの保守の費用も大きい。
    -   採らなかった案: 設計書の字面どおり、コマンドを `opus-review.md` のコードブロックに書く案。セッションの写し誤りを防げず、確かめもブロックを抜き出すか写しを試すことになる。
    -   人の答え: スクリプトと一回限りの確かめの案を選んだ。
    -   ADR: なし（タスクの中で閉じる、置き場所と確かめ方の選択である。ADR-0042 の「Opus のレビューの手順は共通の手順に置く」という決定の範囲に収まる）。
2.  **PR は設計書 8 章の目安どおり 3 本にする。** フェーズ 1 が PR-1（共通の手順の入口 2 と `/weakreview`）、フェーズ 2 が PR-2（読む側と `/runplan` への組み込み）、フェーズ 3 が PR-3（書く側）である。理由: PR-2 の変更は互いに名指し合っていて、分けると途中の状態をレビューすることになるため。
    -   採らなかった案: PR-2 を分け、`_context.md` の読み方と `runpr.md` の表示を先に出す案（4 本）。
    -   人の答え: 3 本の案を選んだ。
    -   ADR: なし。
3.  **判断のスクリプトは、読み替えの表を `origin/main` の `_context.md` から読み、表を自分では持たない。** 表が読めないときは、旧い値を読めない値として扱う（Opus のレビューを行う側に倒れる）。理由: AC-09 は、読み替えを持つ場所を `_context.md` の表だけとする。`origin/main` から読むのは、値を `origin/main` からも読む設計書 1.2 の決定 9 と同じく、レビューを受けるセッションがブランチの上で表を書き換えても判断が外れないようにするためである。表を消した後（要件書 4.5 節）は、旧い値を持つ PR マーカーは残っていないので、振る舞いは変わらない。ADR: なし（AC-09 と ADR-0042 の決定から決まる、実装の形である）。
4.  **ADR-0042 の「決定」の食い違いの記述は、PR-1 と PR-2 で残るずれを述べる形に書き換え、PR-3 で消す。** 理由: `docs/dev/conventions/adr.md`「Discrepancy statements」は、ずれの一部を直す PR に、記述を残るずれだけを述べる形に書き換えることを求める。設計書 3.11.2 の「実装の最後の PR で消す」は、そのまま守る。ADR: なし（ADR-0042 の決定は変えない。編集上の訂正である）。

### 1.4. 既存コード調査結果

コミット `a4d99f7347960476e433689dc2adc7f7cf5210fa`（`origin/main`）で確かめた。以下の `file:line` はこのコミットの行番号である。

-   **設計書の行番号は、一部のファイルで古くなっている。** 設計書はコミット `9ba8f68a` の行番号を使う。その後、タスク 0080 の PR（#804・#808）が `mkplan.md`・`runplan.md`・`runpr.md`・`weakreview.md`・`CLAUDE.md`・`_lib/local-cli-review.md`・ADR-0016・ADR-0035 を変えた（`git diff --stat 9ba8f68a..a4d99f73 -- .claude CLAUDE.md docs/dev`）。設計書の決定には関わらないので、設計書は直さない。各ステップは、変える箇所を見出しと文で探す。主な箇所の今の行は次のとおりである。
    -   `_context.md:64`（旧い 3 つの値を述べる 1 項目）
    -   `mkplan2.md:41-46`（step 4 の「Model requirement」）、`:66`・`:84-85`（step 6 の書式と `**判定理由**`）、`:87-97`（step 7 の表の例）、`:111`（step 9 のペルソナ）、`:125-126`（チェックリスト）
    -   `mkplan.md:195-201`（step 8.5）、`:147-174`（「Technical correctness checklist」）
    -   `_lib/simple-process.md:159-164`（「Writing the three documents」の step 2）
    -   `runplan.md:15-20`（冒頭の案内）、`:43-50`（step 4・4.5）、`:82-132`（step 5a）、`:234`（step 7.7）、`:239`（step 8）
    -   `runpr.md:747-758`（step 4 の値の表示）、`:1052-1053`（step 5 の `/clear` の案内）
    -   `_lib/task-pr-handoff.md:44-47`（PR_TEXT）、`:201-260`（Part B step 3）
    -   `weakreview.md:3`（`argument-hint`）、`:14-29`（Purpose）、`:31-132`（Scope と Procedure）、`:134-319`（「Weak-model failure checklist」）、`:321-330`（「Output format」）
    -   `CLAUDE.md:206`（`/weakreview` の行）、`:212-223`（PR を作るコマンドの段落）
    -   `docs/dev/adr/0016-document-test-policy.md:25`（`weakreview.md` の「Weak-model failure checklist」へのリンク）、`docs/dev/adr/0042-implementation-model-tiers.md:19`（「決定」の食い違いの記述）
-   **0080 への依存は満たされている。** 設計書 8 章は、PR-1 を 0080 の `weakreview.md` を変える PR の後に置く。0080 の計画書の PR-1・PR-2 は、どちらもマージ済みである（`docs/tasks/0080_diff_check_proportion/03_implementation_plan.md:98-99`・`:131-132`）。設計書 3.11.3 の、0080 が消す ADR-0016 と ADR-0035 の記述も、もうない（`git grep -n weakreview docs/dev/adr/` は ADR-0016 の `:25` と ADR-0042 だけを返す）。
-   **旧い値を書く指示の場所。** `git grep -nE 'frontier-(required|recommended)|実装モデル要件' -- .claude CLAUDE.md` は、`_context.md:62`・`:64`、`mkplan2.md`、`runplan.md:15-19`、`runpr.md:747`・`:754`・`:1052` を返す。`standard` は、ほかに「standard error」などの語として現れるので、値として探すときは `` `standard` `` と `**実装モデル要件**: standard` の形に限る。
-   **既存の guard（直すものはない）。** 足す文と新しいファイルは、次の guard を通る必要がある。
    -   `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`）: `.claude/commands/` の文書の相対リンクと、コードスパンの `docs/dev/` のパスが存在する。`_lib/opus-review.md` へリンクする PR は、同じ PR でそのファイルを作る。
    -   `TestCodexMentionOnlyInRequestRow`（`cmd/yt2column/codex_review_docs_test.go:92`）: コマンドの文書に mention を書かない。mention の確かめは、`_lib/task-pr-handoff.md` と同じく `<review-mention>` の形で名指す。
    -   `TestPostingCommandsInventory`（同じファイル）: GitHub に書き込むコマンド（`gh pr create`・`gh pr edit`・`gh pr comment`・`gh api` など）を、新しい文書に書かない。`/weakreview` が使う `gh pr view` は対象外である。
    -   `runpr.md` の step 4 と step 5 の見出し（`cmd/yt2column/fixpr_docs_test.go:44`、`codex_review_docs_test.go` の `runprStep5`）と、`_lib/task-pr-handoff.md` の `3. Push and create the PR for the current branch.` と `4. Run AFTER_CREATE` の書き出し（`codex_review_docs_test.go` の `postingSections`）を変えない。
    -   `TestADRRecords`（`cmd/yt2column/adr_test.go:434`）: ADR-0016 と ADR-0042 の形。
-   **スクリプトの前例と実行環境。** `_lib/` には、`/fixpr` が実行するシェルスクリプト `_lib/adr-correction-candidates.sh` がある。判断のスクリプトと `_checks/` のスクリプトも、開発環境の `/bin/sh` と `git`（2.50.1 で確かめた）で動く POSIX のシェルスクリプトにする。ネットワークには触れない。`git fetch origin main` は呼ぶ側が先に行う。
-   **Agent ツール。** `model` に `opus` を指定でき、`subagent_type` の `fork` では `model` が無視される（設計書 1.2 の決定 7、3.5.5。このセッションの Agent ツールの説明でも確かめた）。
-   製品のコード（`cmd/`・`internal/`・`extension/`）と Go のテストは変えない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 共通の手順の入口 2 と `/weakreview`（設計書 8 章の 1）

**変更するファイル:** `.claude/commands/_lib/opus-review.md`（新規）、`.claude/commands/weakreview.md`、`CLAUDE.md`、`docs/dev/adr/0016-document-test-policy.md`、`docs/dev/adr/0042-implementation-model-tiers.md`、`docs/tasks/0081_implementation_model_choice/_checks/moved_checklist.sh`（新規）、本計画

-   [ ] **ステップ 1-1**: `_lib/opus-review.md` を作る（設計書 3.4 のうち Opus かどうかの規則・残した指摘の記録・mention の確かめ、3.5 の冒頭、3.5.3〜3.5.5）。
    -   frontmatter と、入口 2。入口 1 は PR-2 で足すので、この PR では入口 2 だけを定める。
    -   「Record lines」の節: Opus かどうかの規則（`claude-opus-` で始まる）、残した指摘の記録（`Opus review: kept`）、記録の行の一般の規則、mention の確かめ。
    -   入口 2: 入力（RANGE・TASK_DIR・FIX・DECISION_CHANGE）と出力（OUTCOME・FINDINGS・KEPT・COMMITS・CATEGORIES・READ）、読む文書、決定的な事前の確かめ（`weakreview.md` step 3 の Check 1〜5 を移す）、二人のレビュアー、統合、直しと確認、止まる条件。
    -   「Correctness checklist」を新しく書く。範囲は設計書 3.5.3 の Reviewer A の担当（バグと、AC・設計書・計画書とのずれ）で、テストの網羅と cleanup の項目を入れない（AC-28）。
    -   「Weak-model failure checklist」を `weakreview.md` から移す。項目の文は変えない（ステップ 1-6 で確かめる）。
    -   具体的なモデル ID（版を含む ID）を書かない（設計書 3.5 の冒頭）。
-   [ ] **ステップ 1-2**: `weakreview.md` を書き換える（設計書 3.7、6.3）。frontmatter の `argument-hint`、Purpose、引数の解釈と止まる条件、範囲のそろえ方、直すときの確かめ、PR が開いているときの添え書き、ネットワークの操作、共通の手順の入口 2 の呼び出し、報告を書く。チェックリストと事前の確かめは消す（ステップ 1-1 で移した）。
-   [ ] **ステップ 1-3**: `CLAUDE.md` のコマンドの一覧の `/weakreview` の行を書き換える（設計書 3.11.1 の 1 つ目）。
-   [ ] **ステップ 1-4**: ADR-0016 の `weakreview.md` の「Weak-model failure checklist」へのリンクを、`_lib/opus-review.md` の同じチェックリストへのリンクに直す（設計書 3.11.3）。commit のメッセージに訂正の行を書く。
-   [ ] **ステップ 1-5**: ADR-0042 の「決定」の食い違いの記述を、`/weakreview` のずれが直ったことを除き、残るずれ（`_context.md` とほかのコマンドの定義が旧い値を書き、`/runplan` が Opus のレビューを行わない）を述べる形に書き換える（§1.3 の決定 4）。commit のメッセージに訂正の行を書く。
-   [ ] **ステップ 1-6**: 確認のスクリプト `_checks/moved_checklist.sh` を書いて実行する（AC-28）。マージの基点と比べるコミットを引数で受け取り、基点の `weakreview.md` の「Weak-model failure checklist」の項目と事前の確かめのコードブロックが、比べるコミットの `_lib/opus-review.md` に同じ文で、同じ順にあることを確かめる。見出しの階層の違いは許す。各チェックが少なくとも 1 つの壊し方で失敗することを確かめ、その結果を commit のメッセージに書く。実行の出力・終了コード・2 つのコミットの ID を §7 の「PR-1 の実行の記録」に書く。
-   [ ] **ステップ 1-7**: PR-1 のブランチで、ステップ 1-2 の `/weakreview` を `--no-fix` で一度実行し、実地で確かめる（AC-28〜AC-30）。二つのレビューの出力の最初の行が Opus の ID であることを、共通の手順が定める確かめで判定したことと、統合した報告の形を確かめる。あわせて、その確かめを、Sonnet の ID の行と形の違う行に当てて、どちらも通らないことを確かめる。結果を §7 の「PR-1 の実行の記録」に書く。Critical と Major の指摘は、step 7 の指摘と同じく、この PR の範囲で直す。
-   [ ] **ステップ 1-8**: AC-13 の差分の記録を書く（§7 の「Diff record (AC-13)」の PR-1 の欄）。

**完了の条件:** 設計書 3.4（残した指摘の記録と Opus かどうかの規則）、3.5.3〜3.5.5、3.7、3.11.1 の 1 つ目、3.11.3 の項目がすべて文書にある。§7 にステップ 1-6・1-7 の記録と AC-13 の PR-1 の記録がある。`make test` と `make lint` が通る。

### フェーズ 2: 読む側と `/runplan` への組み込み（設計書 8 章の 2）

**変更するファイル:** `.claude/commands/_context.md`、`.claude/commands/_lib/opus-review.md`、`.claude/commands/_lib/opus-review-decision.sh`（新規）、`.claude/commands/runplan.md`、`.claude/commands/runpr.md`、`.claude/commands/_lib/task-pr-handoff.md`、`CLAUDE.md`、`docs/dev/adr/0042-implementation-model-tiers.md`、`docs/tasks/0081_implementation_model_choice/02_architecture.md`、`docs/tasks/0081_implementation_model_choice/_checks/decision_cases.sh`（新規）、本計画

-   [ ] **ステップ 2-1**: `_context.md` の PR marker conventions の `:64` の項目を、設計書 3.1 の 5 つ（値・値の場所・読み方・読み替えの表・読んだ結果から勧めるモデルへの対応）に置き換える（AC-09、AC-37 の読む側）。1 の「値を書くコマンド」には、この PR では `mkplan2.md` step 4 だけを挙げる。単一フェーズの計画書を書く `mkplan.md` は、PR-3 で足す（ステップ 3-4）。
-   [ ] **ステップ 2-2**: 判断のスクリプト `_lib/opus-review-decision.sh` を書く（設計書 3.5.1、3.5.2、§1.3 の決定 1・3）。
    -   引数は PLAN・TARGET・BASE（設計書 3.5.1）。出力は `RUN`・`NONE <reason>`・`SKIP <done-line>` の 1 行と、`SKIP` のときに続く commit の一覧。どの失敗でも `RUN` を出力する。
    -   値は、`origin/main` の PLAN と作業ツリーの PLAN の両方から、設計書 3.1 の場所と読み方で読む。計画書のフェンスで囲まれたコードブロックの中は読まない（差分の記録の中身を値の行として読まないため）。旧い値は、`origin/main` の `_context.md` の読み替えの表で読む（§1.3 の決定 3）。
    -   完了の記録と実装の記録は、記録の行の形（設計書 3.4）に完全に合う行だけを読む。除いてよい commit の定義は、設計書の冒頭のとおりである。
-   [ ] **ステップ 2-3**: `_lib/opus-review.md` に、実装の記録（`Implementation model:`）と完了の記録（`Opus review: done`）の形、除いてよい commit の定義、入口 1（入力・出力、判断のスクリプトの呼び方、出力が決まった形でないときと終了コードが 0 でないときは `RUN` とすること）を足す（設計書 3.4、3.5.1、3.5.2）。
-   [ ] **ステップ 2-4**: 確認のスクリプト `_checks/decision_cases.sh` を書いて実行する（AC-07、AC-09〜AC-12、AC-18〜AC-20）。一時的なディレクトリに、`origin/main` に当たる参照を持つ fixture のリポジトリを場合ごとに作り、判断のスクリプトを走らせて、出力が期待どおりかを確かめる。場合は、設計書 6.1 の図 4 の各枝と、設計書 4.2 の表のうち判断に関わる行（値が読めない、値の欄がない、ブランチの上で値が下げられた・消された、記録の行が決まった形でない、判断のコマンドが失敗した）を覆う。値の場所は、PR マーカーと、単一フェーズの計画書の「PR 構成」の節の両方の書き方（`### 3.2 PR 構成` と `### 3.2. PR 構成`）を覆う。旧い値は、読み替えの表があるときとないときを覆う。各場合が、判断のスクリプトの対応する箇所を壊すと失敗することを確かめ、その結果を commit のメッセージに書く。実行の出力・終了コード・コミットの ID・2 つのスクリプトの blob の ID を §7 の「PR-2 の実行の記録」に書く。
-   [ ] **ステップ 2-5**: `runplan.md` を書き換える（設計書 3.6）。
    -   冒頭の案内（3.6.1）。
    -   step 4 の後、step 4.5 の前の案内の表示（3.6.2）。step 4.5 でのモデル ID の確かめ（3.6.3）。
    -   step 7.7 の後の実装の記録の空の commit（3.6.3）。step 7.7 の「最初の commit」の記述との関係を合わせる。
    -   step 5a と step 8 の、ブランチ全体のローカル CLI レビュー・要否の判断・Opus のレビュー・グリーンゲート・完了の記録・PR_TEXT・LOCAL_REVIEW（3.6.4 の 1〜6）。push の前のやり直しと、再び実行したときの振る舞い（3.6.4 の後半）。
-   [ ] **ステップ 2-6**: `runpr.md` の step 4 と step 5 の `/clear` の案内を書き換える（設計書 3.8）。見出しは変えない。
-   [ ] **ステップ 2-7**: `_lib/task-pr-handoff.md` の PR_TEXT と Part B step 3 の本文の項目を書き換える（設計書 3.9）。step 3 と step 4 の書き出しは変えない。
-   [ ] **ステップ 2-8**: `CLAUDE.md` の PR を作るコマンドの段落に、`/runplan` が条件に応じて Opus のレビューを行うことを足す（設計書 3.11.1 の 2 つ目）。
-   [ ] **ステップ 2-9**: 設計書 2.2 の表と 3.5.2 に、判断のスクリプトのファイルを足す（§1.3 の決定 1）。commit のメッセージに訂正の行を書く。
-   [ ] **ステップ 2-10**: ADR-0042 の食い違いの記述を、残るずれ（`mkplan2.md` と `mkplan.md` が旧い値を書き、単一フェーズの計画書が値を持たない）を述べる形に書き換える（§1.3 の決定 4）。commit のメッセージに訂正の行を書く。
-   [ ] **ステップ 2-11**: AC-13 の差分の記録を書く（§7 の「Diff record (AC-13)」の PR-2 の欄）。

**確かめ直し。** ステップ 2-4 の比べたコミットより後に判断のスクリプトか `_checks/decision_cases.sh` を変える commit が入ったら、ステップ 2-4 をやり直して §7 の記録を改める。

**完了の条件:** 設計書 3.1、3.4（実装の記録と完了の記録）、3.5.1、3.5.2、3.6、3.8、3.9、3.11.1 の 2 つ目の項目がすべて文書にある。§7 にステップ 2-4 の記録と AC-13 の PR-2 の記録がある。`make test` と `make lint` が通る。

### フェーズ 3: 書く側（設計書 8 章の 3）

**変更するファイル:** `.claude/commands/mkplan2.md`、`.claude/commands/mkplan.md`、`.claude/commands/_lib/simple-process.md`、`.claude/commands/_context.md`、`docs/dev/adr/0042-implementation-model-tiers.md`、`docs/tasks/0081_implementation_model_choice/_checks/old_values.sh`（新規）、本計画

-   [ ] **ステップ 3-1**: `mkplan2.md` の step 4 の「Model requirement」、step 6 の書式と `**判定理由**` の規則、step 7 の表の例、step 9 のペルソナとチェックリストを新しい値にする（設計書 3.2）。値の意味と判定理由の書き方は step 4 だけが定め、step 6 は step 4 を名指す（AC-05）。
-   [ ] **ステップ 3-2**: `mkplan.md` の step 8.5 に、単一フェーズの計画書の「PR 構成」の節と値の 2 行を書くことを足す。「Technical correctness checklist」に、その項目を足す（設計書 3.3）。
-   [ ] **ステップ 3-3**: `_lib/simple-process.md` の「Writing the three documents」に、単一フェーズの計画書の値を書くことを足す（設計書 3.3）。
-   [ ] **ステップ 3-4**: `_context.md` の PR marker conventions の値の項目に、値を書くコマンドとして `mkplan.md` の step 8.5（単一フェーズの計画書）を足す（設計書 3.1 の 1、AC-37 の書く側）。
-   [ ] **ステップ 3-5**: ADR-0042 の「決定」の食い違いの記述を消す（設計書 3.11.2、§1.3 の決定 4）。commit のメッセージに訂正の行を書く。
-   [ ] **ステップ 3-6**: 確認のスクリプト `_checks/old_values.sh` を書いて実行する（AC-01、AC-04、AC-05、AC-39）。比べるコミットの `.claude/commands/` の下で、旧い 3 つの値が値として現れる箇所（§1.4 の探し方）を挙げ、すべてが `_context.md` の読み替えの表と、旧い値を読むときの扱いを述べる箇所にあることを確かめる。許す箇所は `_context.md` の PR marker conventions の節だけを既定とし、実装でほかの箇所が要るときは、その箇所を本計画のこのステップに書き足してからスクリプトに足す。各チェックが少なくとも 1 つの壊し方で失敗することを確かめ、その結果を commit のメッセージに書く。実行の出力・終了コード・コミットの ID を §7 の「PR-3 の実行の記録」に書く。
-   [ ] **ステップ 3-7**: AC-13 の差分の記録を書く（§7 の「Diff record (AC-13)」の PR-3 の欄）。

**完了の条件:** 設計書 3.1 の 1（書く側）、3.2、3.3、3.11.2 の項目がすべて文書にある。§7 にステップ 3-6 の記録と AC-13 の PR-3 の記録がある。`make test` と `make lint` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（フェーズ 1） | `_lib/opus-review.md` の入口 2、`/weakreview` | `/weakreview` 一つで、Opus の二つのレビューを行い、既定で直せる |
| M2（フェーズ 2） | 入口 1、判断のスクリプト、`_context.md` の読み方、`/runplan`・`/runpr` | 旧い値を読み替えの表で読み、Sonnet で実装した phase group を含む PR で `/runplan` が Opus のレビューを行う。この PR のマージの後に作ったブランチから効く（設計書 8 章） |
| M3（フェーズ 3） | `/mkplan2`・`/mkplan` が新しい値を書く | 新しい計画書が新しい値と、単一フェーズの計画書の値を持つ |

順序は設計書 8 章のとおりで、新しい値を読むコマンドを、書くコマンドより先に入れる。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 足さない（設計書 7.1、§1.2）。製品のコードは変わらない。
-   **一回限りのスクリプト:** `_checks/` の 3 つのスクリプトを、各 PR のブランチで 1 回実行し、出力を §7 に記録する。各チェックが壊し方で失敗することは、実装する人が壊して確かめ、commit のメッセージに書く。
    -   `moved_checklist.sh`（PR-1）: 移したチェックリストと事前の確かめが、移す前と同じである。
    -   `decision_cases.sh`（PR-2）: 判断のスクリプトの各場合の出力。
    -   `old_values.sh`（PR-3）: 旧い値が、読み替えの表と、旧い値を読む箇所の外に残っていない。
-   **差分の記録:** AC-13（0073・0076・0077 の計画書が変わらない）を、PR ごとに記録した差分の読み取りで確かめる。比べるパスは 3 つの計画書で、差分は空になるはずなので、読む量は小さい。
-   **実地の確かめ:** PR-1 で `/weakreview --no-fix` を一度実行する（ステップ 1-7）。`/runplan` の Opus のレビューの実地の確かめは、PR-2 のマージの後に作ったブランチで初めて行われる（設計書 7.4、8 章）。本タスクの PR-3 の値が `sonnet` に当たるときは、本タスクの中では行われない。
-   **既存の guard:** `make test` が、§1.4 の guard を走らせる。
-   **後方互換:** 旧い値を持つ計画書の読み替えは、`decision_cases.sh` の旧い値の場合で確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 判断のスクリプトの誤りで、Opus のレビューを行うべき PR で行わない | 迷うとき・失敗したときは `RUN` に倒す（設計書 1.1）。`decision_cases.sh` で図 4 の各枝と失敗の場合を確かめる |
| セッションが判断のスクリプトや計画書をブランチの上で書き換え、レビューを外す | 値と読み替えの表は `origin/main` からも読む（設計書 1.2 の決定 9、§1.3 の決定 3）。スクリプト自体の書き換えは防がない。品質を保つ仕組みで、セキュリティの境界ではない（設計書 5.2） |
| Opus でないモデルがレビューを走らせ、Opus の ID を写して報告する | 共通の手順と prompt に具体的なモデル ID を書かない（設計書 3.5.5）。残る危うさとして受け入れる |
| PR-2 の前に作ったブランチの `/runplan` は、新しい手順を使わない | 設計書 8 章のとおり受け入れる。PR-2 のマージの後に作るブランチから効く |
| PR-1 のマージの後、PR-3 のマージまで、ADR-0042 の食い違いの記述がずれる | 各 PR で、残るずれを述べる形に書き換える（§1.3 の決定 4） |
| 新しい文が既存の guard を壊す | §1.4 の制約を守り、各ステップの後に `make test` を通す |
| 実地の確かめ（ステップ 1-7）の費用 | 一度だけ、`--no-fix` で行う。費用は要件書 1 節の目安（約 $1.5〜4）の範囲である |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1 完了（ステップ 1-1〜1-8）
-   [ ] フェーズ 2 完了（ステップ 2-1〜2-11）
-   [ ] フェーズ 3 完了（ステップ 3-1〜3-7）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`manual` は、PR のレビューで、その文書の該当箇所を読んで確かめることをいう。コマンドの定義の手順の文だけを定める AC は、`manual` だけで確かめる（§1.2）。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `_checks/old_values.sh`（`mkplan2.md` に旧い値が書く値として残らない）、`mkplan2.md` の書式と説明を読む | static、manual | ステップ 3-1 | PR-3 |
| AC-02 | `mkplan2.md` step 4 を読む | manual | ステップ 3-1 | PR-3 |
| AC-03 | `mkplan2.md` step 6 の `**判定理由**` の規則を読む | manual | ステップ 3-1 | PR-3 |
| AC-04 | `_checks/old_values.sh`（表の例とチェックリストに旧い値が残らない）、step 7 と step 9 を読む | static、manual | ステップ 3-1 | PR-3 |
| AC-05 | `_checks/old_values.sh`、値の意味と判定理由の書き方が step 4 だけにあることを読む | static、manual | ステップ 3-1 | PR-3 |
| AC-06 | `mkplan.md` step 8.5 とチェックリストを読む | manual | ステップ 3-2 | PR-3 |
| AC-07 | `_checks/decision_cases.sh`（単一フェーズの計画書の「PR 構成」の節から値を読む場合）、`runplan.md` と `runpr.md` を読む | test、manual | ステップ 2-2・2-5・2-6 | PR-2 |
| AC-08 | `_lib/simple-process.md` を読む | manual | ステップ 3-3 | PR-3 |
| AC-09 | `_checks/decision_cases.sh`（旧い値を `_context.md` の表で読む場合と、表がない場合）、`_checks/old_values.sh`（表の外に旧い値が残らない）、`_context.md` を読む | test、static、manual | ステップ 2-1・2-2・3-6 | PR-2、PR-3 |
| AC-10 | `_checks/decision_cases.sh`（旧い値の場合）、`runplan.md` と `runpr.md` の示し方を読む | test、manual | ステップ 2-2・2-5・2-6 | PR-2 |
| AC-11 | `_checks/decision_cases.sh`（読めない値の場合）、`runplan.md` と `runpr.md` を読む | test、manual | ステップ 2-2・2-5・2-6 | PR-2 |
| AC-12 | `_checks/decision_cases.sh`（値の欄がない場合）、`runplan.md` と `runpr.md` を読む | test、manual | ステップ 2-2・2-5・2-6 | PR-2 |
| AC-13 | 記録した差分の読み取り（下の「Diff record (AC-13)」） | diff | 全ステップ | PR-1、PR-2、PR-3 |
| AC-14 | `runplan.md` の冒頭の案内を読む | manual | ステップ 2-5 | PR-2 |
| AC-15 | `runplan.md` の案内の表示を読む | manual | ステップ 2-5 | PR-2 |
| AC-16 | `runplan.md` の冒頭の案内を読む | manual | ステップ 2-5 | PR-2 |
| AC-17 | `runpr.md` の step 4 と step 5 を読む | manual | ステップ 2-6 | PR-2 |
| AC-18 | `_checks/decision_cases.sh`（実装の記録を読む場合と、形の違う行の場合）、`runplan.md` を読む | test、manual | ステップ 2-2・2-3・2-5 | PR-2 |
| AC-19 | `_checks/decision_cases.sh`（`RUN` の各場合）、`runplan.md` step 5a と step 8 を読む | test、manual | ステップ 2-2・2-5 | PR-2 |
| AC-20 | `_checks/decision_cases.sh`（`NONE` の各場合）、`runplan.md` を読む | test、manual | ステップ 2-2・2-5 | PR-2 |
| AC-21 | `_lib/opus-review.md` 入口 2 と `runplan.md` の RANGE を読む | manual | ステップ 1-1・2-5 | PR-1、PR-2 |
| AC-22 | `_lib/opus-review.md` の統合・直し・確認・上限を読む | manual | ステップ 1-1 | PR-1 |
| AC-23 | `_lib/opus-review.md` の退けない規則を読む | manual | ステップ 1-1 | PR-1 |
| AC-24 | `_lib/opus-review.md` の DECISION_CHANGE と `runplan.md` を読む | manual | ステップ 1-1・2-5 | PR-1、PR-2 |
| AC-25 | `_lib/opus-review.md` の止まる条件と `runplan.md` の止まり方を読む。モデル ID の確かめの実行の記録（ステップ 1-7） | manual | ステップ 1-1・1-7・2-5 | PR-1、PR-2 |
| AC-26 | `runplan.md` step 5a と step 8 の順序を読む | manual | ステップ 2-5 | PR-2 |
| AC-27 | `_lib/task-pr-handoff.md` と `runplan.md` の PR_TEXT を読む | manual | ステップ 2-5・2-7 | PR-2 |
| AC-28 | `_checks/moved_checklist.sh`（弱いモデルの失敗のチェックリストが移す前と同じ）、「Correctness checklist」を読む、実地の確かめ（ステップ 1-7） | diff、manual | ステップ 1-1・1-6・1-7 | PR-1 |
| AC-29 | `_lib/opus-review.md` のモデルの指定と確かめを読む、実地の確かめ（ステップ 1-7） | manual | ステップ 1-1・1-7 | PR-1 |
| AC-30 | `_lib/opus-review.md` の統合と報告を読む、実地の確かめ（ステップ 1-7） | manual | ステップ 1-1・1-7 | PR-1 |
| AC-31 | `weakreview.md` と `_lib/opus-review.md` の直しの規則を読む | manual | ステップ 1-1・1-2 | PR-1 |
| AC-32 | `weakreview.md` の引数の解釈を読む | manual | ステップ 1-2 | PR-1 |
| AC-33 | `weakreview.md` の直すときの確かめを読む | manual | ステップ 1-2 | PR-1 |
| AC-34 | `weakreview.md` の添え書きを読む | manual | ステップ 1-2 | PR-1 |
| AC-35 | `weakreview.md` の Purpose を読む | manual | ステップ 1-2 | PR-1 |
| AC-36 | `weakreview.md` の範囲の指定とそろえ方を読む | manual | ステップ 1-2 | PR-1 |
| AC-37 | `_context.md` の PR marker conventions を読む | manual | ステップ 2-1・3-4 | PR-2、PR-3 |
| AC-38 | `CLAUDE.md` の `/weakreview` の行と `weakreview.md` の `argument-hint` を読む | manual | ステップ 1-2・1-3 | PR-1 |
| AC-39 | `_checks/old_values.sh`、「`weakreview.md` で確かめる」という案内の文が消えたことを読む | static、manual | ステップ 2-5・3-1・3-6 | PR-2、PR-3 |
| AC-40 | `cmd/yt2column/adr_test.go::TestADRRecords`（ADR の形）、ADR-0042 の「決定」を読む | static、manual | ステップ 1-5・2-10・3-5 | PR-1、PR-2、PR-3 |

### PR-1 の実行の記録

（ステップ 1-6 と 1-7 で書く。）

### PR-2 の実行の記録

（ステップ 2-4 で書く。）

### PR-3 の実行の記録

（ステップ 3-6 で書く。）

### 差分の記録

AC-13 は、記録した差分の読み取りで確かめる。比べるパスは次の 3 つで、PR ごとに記録する。

PR-1:

Diff record (AC-13):

- Compared paths: `docs/tasks/0073_adr_context_decision_review/03_implementation_plan.md` `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md` `docs/tasks/0077_openai_llm_client/03_implementation_plan.md`

PR-2:

Diff record (AC-13):

- Compared paths: `docs/tasks/0073_adr_context_decision_review/03_implementation_plan.md` `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md` `docs/tasks/0077_openai_llm_client/03_implementation_plan.md`

PR-3:

Diff record (AC-13):

- Compared paths: `docs/tasks/0073_adr_context_decision_review/03_implementation_plan.md` `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md` `docs/tasks/0077_openai_llm_client/03_implementation_plan.md`

## 8. 成功の基準 (Success Criteria)

-   要件書の AC-01〜AC-40 が、§7 の確かめで満たされている。
-   §7 に、3 つの `_checks/` のスクリプトの実行の記録と、AC-13 の 3 つの差分の記録がある。
-   各 PR で `make test` と `make lint` が通る。
-   ADR-0042 の「決定」に食い違いの記述が残らず、ADR の索引の行がある。
-   セキュリティ: 要件書 4.2 節のとおり、条件付きのガイドの契機に当たらない。GitHub に書き込むコマンドを新しく加えない（§1.4）。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後に作ったブランチで `/runplan` を初めて実行するとき、値が `sonnet+opus-review` か `opus` の PR で、Opus のレビューが行われ、PR の説明に結果が載ることを見る（設計書 7.4）。
-   PR がまだ作られていない PR マーカーに旧い値を持つ計画書（0073・0076・0077）がなくなったら、読み替えの表を消せる（要件書 4.5 節）。
