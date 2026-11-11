# 実装計画書：実装モデル要件を Opus・Sonnet の選択に直結させ、Opus のレビューを一度に行う

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-11 |
| Review date | 2026-11-11 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。用語（実装モデル要件・正しさのレビュー・弱いモデルの失敗のレビュー・phase group・確認のレビュー・Opus のレビュー・旧い値・読めない値・読み替えの表・共通の手順・記録の行・実装の記録・完了の記録・残した指摘の記録・除いてよい commit・要否の判断・Opus の ID）は、要件書 6 章と設計書の冒頭と同じ意味で使う。本計画は、次の用語を加える。

-   **入口 1・入口 2:** 共通の手順の二つの呼び出し口（設計書 3.5）。入口 1 は要否の判断、入口 2 はレビューである。`CHECKS`・`PR_TEXT`・`LOCAL_REVIEW`・Part B は、`_lib/task-pr-handoff.md` の語である。
-   **判断のスクリプト:** 要否の判断（設計書 3.5.2）を行うシェルスクリプト `.claude/commands/_lib/opus-review-decision.sh`。共通の手順が、`origin/main` のコピーを走らせる（§1.3 の決定 1・6）。
-   **`_checks/` のスクリプト:** 本タスクのディレクトリの `_checks/` に置く、一回限りの確かめのスクリプト。PR のブランチで実行し、結果を §7 に記録する。`make test` と CI は実行しない（`docs/dev/conventions/requirements_process.md` §4）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

コマンドの定義・`_lib/` の共通の手順・`CLAUDE.md`・ADR の文言を変え、判断のスクリプトを加える。これにより、実装モデル要件の値から、セッションのモデルと Opus のレビューの要否が分かるようになる。`/runplan` は PR を開く前に Opus のレビューを行い、`/weakreview` は同じレビューを一度に行えるようになる（要件書 2.1 節）。何をどこに書くかは設計書 3 章が定める。本計画は、作業の順序と確かめ方を定める。

### 1.2. 実装の原則

-   **文面は設計書 3 章に従い、英語で書く。** コマンドの定義、`_lib/`、スクリプトは英語で書く（要件書 5 章）。判断のスクリプトの、計画書の文と一致しなければならないリテラルだけは日本語でよい（§1.3 の決定 5）。`_checks/` のスクリプトは、`CLAUDE.md`「Go Idioms」の `_checks/` の例外に従う。各ステップは、書く内容を設計書の節で指す。文面は実装の PR で書く。
-   **新しい恒久のテストは足さない**（設計書 7.1）。判断のスクリプトの振る舞い、移したチェックリスト、旧い値の残りは、`_checks/` のスクリプトで確かめる。
-   **文書の自然言語の中身だけを定める AC は、PR のレビューで読んで確かめる**（設計書 7.3）。コマンドの定義の手順の文（案内、Opus のレビューの手順、`/weakreview` の振る舞い）がこれに当たる。値と構造（値の読み方、記録の行の形、旧い値の残り）は、実行する確かめを持つ。
-   **足す文は、コマンドの文書を読む既存の guard の制約を守る**（設計書 3.12、§1.4）。
-   **ADR の食い違いの記述を書き換える・消すことは、編集上の訂正である**（`docs/dev/conventions/adr.md`「Editorial corrections」の 2 つ目の種類）。その commit のメッセージに訂正の行を書く。PR の説明の訂正の段落は、`/runplan` が `_lib/task-pr-handoff.md` Part B で書く。
-   **シェルスクリプトは bash で書く。** 前例の `_lib/adr-correction-candidates.sh` と同じく `#!/usr/bin/env bash` とし、`bash <path>` で走らせる。ネットワークには触れない。`git fetch origin main` は呼ぶ側が先に行う。

### 1.3. 計画の際に決めたこと（2026-11-11）

1.  **要否の判断のコマンドの組は、`_lib/` の判断のスクリプトに置き、共通の手順がそれを呼ぶ。確かめは、`_checks/` のスクリプトで fixture のリポジトリを作って各場合を走らせ、出力を §7 に記録する。** 理由: 判断は、計画書を 2 か所から読む、完了の記録の END が `HEAD` の祖先かを見る、除いてよい commit を見分ける、各 commit が実装の記録で覆われるかを見る、と条件が多い。コードブロックをセッションが写して走らせると、写し誤りが起きうる。スクリプトなら、PR の中で同じものを確かめられる。恒久の Go テストは足さないので、設計書 7.1 と合う。設計書は、本計画を加える PR の中で、決定の変更として書き換えた（設計書 1.2 の決定 10・11）。設計書は `draft` に戻し、本計画と一緒に承認を受ける。当初は、PR-2 で編集上の訂正として設計書を直すつもりだった。しかし、決定 6 と、push の前のやり直しを CHECKS で行うこと（ステップ 2-5）は、設計書にない事柄を加えるので、決定の変更として扱った（人の答え）。
    -   採らなかった案: スクリプトに恒久の Go テストを足す案（`_lib/adr-correction-candidates.sh` と同じ形）。設計書 7.1 を変える決定の変更になり、テストの保守の費用も大きい。
    -   採らなかった案: 設計書の字面どおり、コマンドを `opus-review.md` のコードブロックに書く案。セッションの写し誤りを防げず、確かめも、ブロックを抜き出すか、写したものを走らせて試すことになる。
    -   人の答え: スクリプトと一回限りの確かめの案を選んだ。
    -   ADR: なし（タスクの中で閉じる、置き場所と確かめ方の選択である。ADR-0042 の「Opus のレビューの手順は共通の手順に置く」という決定の範囲に収まる）。
2.  **PR は設計書 8 章の目安どおり 3 本にする。** フェーズ 1 が PR-1（共通の手順の入口 2 と `/weakreview`）、フェーズ 2 が PR-2（読む側と `/runplan` への組み込み）、フェーズ 3 が PR-3（書く側）である。理由: PR-2 の変更は互いに名指し合っていて、分けると途中の状態をレビューすることになるため。
    -   採らなかった案: PR-2 を分け、`_context.md` の読み方と `runpr.md` の表示を先に出す案（4 本）。
    -   人の答え: 3 本の案を選んだ。
    -   ADR: なし。
3.  **判断のスクリプトは、読み替えの表を `origin/main` の `_context.md` から読み、表を自分では持たない。** 表が読めないときは、旧い値を読めない値として扱う（Opus のレビューを行う側に倒れる）。理由: AC-09 は、読み替えを持つ場所を `_context.md` の表だけとする。`origin/main` から読むのは、決定 6 と同じく、レビューを受けるセッションがブランチの上で表を書き換えても判断が外れないようにするためである。表を消した後（要件書 4.5 節）は、旧い値を持つ PR マーカーは残っていないので、振る舞いは変わらない。ADR: なし（AC-09 と ADR-0042 の決定から決まる、実装の形である）。
4.  **ADR-0042 の「決定」の食い違いの記述は、PR-1 と PR-2 で残るずれを述べる形に書き換え、PR-3 で消す。** 理由: `docs/dev/conventions/adr.md`「Discrepancy statements」は、ずれの一部を直す PR に、記述を残るずれだけを述べる形に書き換えることを求める。設計書 3.11.2 の「実装の最後の PR で消す」は、そのまま守る。ADR: なし（ADR-0042 の決定は変えない。編集上の訂正である）。
5.  **guard のテストの日本語のリテラルの例外を、判断のスクリプトに広げる。範囲は、計画書の見出し・欄名・値の行の書き出しと一致しなければならないリテラル（`作成ポイント`、`**実装モデル要件**: `、`PR 構成` など）に限る。** 本計画の PR で、`CLAUDE.md`「Go Idioms」に例外を足し、ADR-0035 の「採らなかった案」と「経緯」を更新した。理由: 判断のスクリプトは日本語の計画書を読むので、リテラルが計画書の文と一致しなければならない。ソースのリテラルを英語に限る規則（`CLAUDE.md`「Language」と「Go Idioms」、ADR-0035）の例外は、Go の guard のテスト・`internal/doctestutil`・`_checks/` のスクリプト・拡張のカタログに限られていて、`_lib/` のスクリプトは含まれない。
    -   採らなかった案: 見出しを、それを引用する英語のコマンドの定義から引数で渡し、スクリプトは ASCII だけにする案。写し誤りがあると値を見つけられず、値がないものとして Opus のレビューを行わない側に倒れる。
    -   採らなかった案: スクリプトをやめ、コマンドを `opus-review.md` に直接書く案（決定 1 で採らなかった案）。
    -   人の答え: 例外をこのスクリプトに広げる案を選んだ。
    -   ADR: [ADR-0035](../../dev/adr/0035-language-policy.md) を更新した（例外の範囲を変えることは、ソースの言語の方針の変更である）。
6.  **`/runplan` は、判断のスクリプトを `origin/main` のコピーで走らせる。** 共通の手順の入口 1 は、`git show origin/main:.claude/commands/_lib/opus-review-decision.sh` で取り出したスクリプトを、作業ツリーの外のファイルに書いて `bash` で走らせる。`/fixpr` が `_lib/adr-correction-candidates.sh` を走らせる形（`.claude/commands/fixpr.md:692-715`）と同じである。取り出せないときや、走らせて失敗したときは `RUN` とする。理由: 値（設計書 1.2 の決定 9）と読み替えの表（決定 3）を `origin/main` から読むのと同じく、レビューを受けるセッションがブランチの上でスクリプトを書き換えても判断が外れないようにするため。PR-2 がマージされる前は、`origin/main` にスクリプトがないので `RUN` になる。これは安全側である。
    -   採らなかった案: ブランチのコピーを走らせ、表を `origin/main` から読む理由を AC-09 だけにする案。
    -   人の答え: `origin/main` のコピーの案を選んだ。
    -   ADR: なし（ADR-0042 の「判断は、レビューを受けるセッションの解釈に任せず、決まったコマンドで行い、値は `origin/main` とブランチの計画書の両方から読む」の範囲に収まる）。
7.  **単一フェーズの計画書を表す TARGET は、英語の `single` にする。** スクリプトの引数は英語で書く（決定 5 の範囲の外）。設計書 3.5.1 も、本計画の PR で `single` に直した。ADR: なし。

### 1.4. 既存コード調査結果

コミット `a4d99f7347960476e433689dc2adc7f7cf5210fa`（`origin/main`）で確かめた。以下の `file:line` はこのコミットの行番号である。

-   **設計書の後に、タスク 0080 が `runplan.md` の step 5a を変えた。** 設計書はコミット `9ba8f68a` の行番号を使う。その後、0080 の PR（#804・#808）が `mkplan.md`・`runplan.md`・`runpr.md`・`weakreview.md`・`CLAUDE.md`・`_lib/local-cli-review.md`・ADR-0016・ADR-0035 を変えた（`git diff --stat 9ba8f68a..a4d99f73 -- .claude CLAUDE.md docs/dev`）。このうち `runplan.md` の step 5a には、差分の記録の再確認（`:86`）と、CHECKS の「ブランチのレビューが commit を作ったら、グリーンゲートと差分の記録の再確認をやり直す」という文（`:90-92`）が加わった。Opus のレビューの直しの commit も、AC の確かめ（`:85`）と差分の記録を古くしうる。ステップ 2-5 で扱う。ほかの変更は設計書の決定に関わらない。設計書の行番号は直さず、各ステップは変える箇所を見出しと文で探す。主な箇所の今の行は次のとおりである。
    -   `_context.md:64`（旧い 3 つの値を述べる 1 項目）
    -   `mkplan2.md:41-46`（step 4 の「Model requirement」）、`:66`・`:84-85`（step 6 の書式と `**判定理由**`）、`:87-97`（step 7 の表の例）、`:111`（step 9 のペルソナ）、`:125-126`（チェックリスト）
    -   `mkplan.md:196-202`（step 8.5）、`:147-174`（「Technical correctness checklist」）
    -   `_lib/simple-process.md:159-164`（「Writing the three documents」の step 2）
    -   `runplan.md:15-20`（冒頭の案内）、`:43-50`（step 4・4.5）、`:82-132`（step 5a）、`:234`（step 7.7）、`:239`（step 8）
    -   `runpr.md:747-758`（step 4 の値の表示）、`:1052-1053`（step 5 の `/clear` の案内）
    -   `_lib/task-pr-handoff.md:44-47`（PR_TEXT）、`:199-252`（Part B step 2・3）
    -   `weakreview.md:3`（`argument-hint`）、`:14-29`（Purpose）、`:31-132`（Scope と Procedure）、`:134-319`（「Weak-model failure checklist」）、`:321-330`（「Output format」）
    -   `CLAUDE.md:206`（`/weakreview` の行）、`:212-223`（PR を作るコマンドの段落）
    -   `docs/dev/adr/0016-document-test-policy.md:25`（`weakreview.md` の「Weak-model failure checklist」へのリンク）、`docs/dev/adr/0042-implementation-model-tiers.md:19`（「決定」の食い違いの記述。issue #760 に完全な URL でリンクする）
-   **0080 への依存は満たされている。** 設計書 8 章は、PR-1 を 0080 の `weakreview.md` を変える PR の後に置く。0080 の計画書の PR-1・PR-2 は、どちらもマージ済みである（`docs/tasks/0080_diff_check_proportion/03_implementation_plan.md:98-99`・`:131-132`）。設計書 3.11.3 の、0080 が消す ADR-0016 と ADR-0035 の記述も、すでに消えている（`git grep -n weakreview docs/dev/adr/` は ADR-0016 の `:25` と ADR-0042 だけを返す）。
-   **移すチェックリストの中の、`weakreview.md` の手順への参照。** 「Weak-model failure checklist」の 2 つの項目が「the step-3 grep」と書く（`weakreview.md:249`・`:305`）。事前の確かめは共通の手順へ移るので、この参照は移した先の手順の名前に合わせて直す必要がある（ステップ 1-1）。
-   **旧い値を書く指示の場所。** `git grep -nE 'frontier-(required|recommended)|実装モデル要件' -- .claude CLAUDE.md` は、`_context.md:62`・`:64`、`mkplan2.md`、`runplan.md:15`・`:17`・`:18`、`runpr.md:747`・`:754`・`:1052` を返す。値の `standard` は、`mkplan2.md:95` の表の例のように、バッククォートのない表のセルとしても現れる。一方で、`standard` という語は「standard error」「standard library」などとしても現れる（`fixpr.md:68` ほか）。そのため、値として探すときは `standard` を単語として探し、値でない使い方を名前の付いた一覧で除く（ステップ 3-6）。
-   **既存の guard（直すものはない）。** 足す文と新しいファイルは、次の guard を通る必要がある。
    -   `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`）: `.claude/commands/` の Markdown の文書の相対リンクと、コードスパンの `docs/dev/` のパスが存在する。`_lib/opus-review.md` へリンクする PR は、同じ PR でそのファイルを作る。`.sh` のファイルは読まない。
    -   `TestCodexMentionOnlyInRequestRow`（`cmd/yt2column/codex_review_docs_test.go:92`）: コマンドの文書に mention を書かない。mention の確かめは、`_lib/task-pr-handoff.md` と同じく `<review-mention>` の形で名指す。
    -   `TestPostingCommandsInventory`（同じファイル）: GitHub に書き込むコマンド（`gh pr create`・`gh pr edit`・`gh pr comment`・`gh api` など）を、新しい文書に書かない。`/weakreview` が使う `gh pr view` は対象外である。
    -   `TestContextAndCommandsAgree`（`cmd/yt2column/fixpr_docs_test.go:992`）: `runpr.md` と `_lib/task-pr-handoff.md` に、レビュアーの名前を書かない。
    -   `runpr.md` の step 4 と step 5 の見出し（`cmd/yt2column/fixpr_docs_test.go:44`、`codex_review_docs_test.go` の `runprStep5`）と、`_lib/task-pr-handoff.md` の `3. Push and create the PR for the current branch.` と `4. Run AFTER_CREATE` の書き出し（`codex_review_docs_test.go` の `postingSections`）を変えない。
    -   `TestADRRecords`（`cmd/yt2column/adr_test.go:434`）: ADR-0016・ADR-0035・ADR-0042 の形。
-   **スクリプトの前例。** `_lib/adr-correction-candidates.sh` は `#!/usr/bin/env bash` で、ASCII だけで書かれている。`/fixpr` は、その `origin/main` のコピーを `git show` で取り出して走らせる（`fixpr.md:692-715`）。開発環境の git は 2.50.1 である。
-   **Agent ツール。** `model` に `opus` を指定でき、`subagent_type` の `fork` では `model` が無視される（設計書 1.2 の決定 7、3.5.5。このセッションの Agent ツールの説明でも確かめた）。
-   **単一フェーズの計画書の実例。** 0054・0069・0070・0071 の計画書は「PR 構成」の節を持ち、値の行を持たない（設計書 1.2 の決定 5）。0077 の計画書は、旧い 3 つの値を PR マーカーに持つ。
-   製品のコード（`cmd/`・`internal/`・`extension/`）と Go のテストは変えない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 共通の手順の入口 2 と `/weakreview`（設計書 8 章の 1）

**変更するファイル:** `.claude/commands/_lib/opus-review.md`（新規）、`.claude/commands/weakreview.md`、`CLAUDE.md`、`docs/dev/adr/0016-document-test-policy.md`、`docs/dev/adr/0042-implementation-model-tiers.md`、`docs/tasks/0081_implementation_model_choice/_checks/moved_checklist.sh`（新規）、本計画

-   [x] **ステップ 1-1**: `_lib/opus-review.md` を作る（設計書 3.4 のうち Opus かどうかの規則・残した指摘の記録・mention の確かめ、3.5 の冒頭、3.5.3〜3.5.5）。
    -   frontmatter と入口 2。入口 1 は PR-2 で足す。
    -   「Record lines」の節: Opus かどうかの規則（`claude-opus-` で始まる）、残した指摘の記録（`Opus review: kept`）、記録の行の一般の規則、mention の確かめ。
    -   入口 2: 入力（RANGE・TASK_DIR・FIX・DECISION_CHANGE）と出力（OUTCOME・FINDINGS・KEPT・COMMITS・CATEGORIES・READ）、読む文書、決定的な事前の確かめ（`weakreview.md` step 3 の Check 1〜5 を移す）、二人のレビュアー、統合、直しと確認、止まる条件。
    -   「Correctness checklist」を新しく書く。範囲は設計書 3.5.3 の Reviewer A の担当（バグと、AC・設計書・計画書とのずれ）で、テストの網羅と cleanup の項目を入れない（AC-28）。
    -   「Weak-model failure checklist」と事前の確かめを `weakreview.md` から移す。文は変えない。ただし、`weakreview.md` の手順への参照（§1.4 の「the step-3 grep」の 2 か所と、事前の確かめのコードブロックの範囲と task のディレクトリの置き場所）は、移した先の名前（事前の確かめの手順の名前、RANGE・TASK_DIR）に合わせて直す。直す参照の一覧を、ステップ 1-7 の `_checks/` のスクリプトの置き換えの一覧にする。
    -   具体的なモデル ID（版を含む ID）を書かない（設計書 3.5 の冒頭）。
-   [x] **ステップ 1-2**: `weakreview.md` を書き換える（設計書 3.7、6.3）。frontmatter の `argument-hint`、Purpose、引数の解釈と止まる条件、範囲のそろえ方、タスクの特定、直すときの確かめ、PR が開いているときの添え書き、ネットワークの操作、共通の手順の入口 2 の呼び出し、報告を書く。チェックリストと事前の確かめは消す（ステップ 1-1 で移した）。
-   [x] **ステップ 1-3**: `CLAUDE.md` のコマンドの一覧の `/weakreview` の行を書き換える（設計書 3.11.1 の 1 つ目）。
-   [x] **ステップ 1-4**: ADR-0016 の `weakreview.md` の「Weak-model failure checklist」へのリンクを、`_lib/opus-review.md` の同じチェックリストへのリンクに直す（設計書 3.11.3）。commit のメッセージに訂正の行を書く。
-   [x] **ステップ 1-5**: ADR-0042 の「決定」の食い違いの記述を、残るずれを述べる形に書き換える（§1.3 の決定 4）。issue #760 への完全な URL のリンクは残す。残るずれは、ADR-0042 の「決定」の箇条ごとに、PR-1 のブランチの状態（マージの後の `main` になるもの）に照らして確かめる（`_context.md` と `mkplan2.md`・`mkplan.md`・`_lib/simple-process.md` が旧い値を書き、単一フェーズの計画書が値を持たず、`runplan.md` が Opus のレビューを行わず、共通の手順が要否の判断と実装の記録・完了の記録を持たない、など）。commit のメッセージに訂正の行を書く。
-   [x] **ステップ 1-6**: PR-1 のブランチで、ステップ 1-2 の `/weakreview` を実地で確かめる（AC-28〜AC-30、AC-33）。
    -   `--no-fix` で一度実行し、二つのレビューの出力の最初の行を共通の手順が定める確かめで判定して通ったことと、統合した報告の形を確かめる。あわせて、その確かめを、Sonnet の ID を書いた行と、形の合わない行の両方に当て、どちらも通らないことを確かめる。
    -   直すときの止まる条件の 2 つ（作業ツリーが clean でない、範囲の終わりが `HEAD` でない）を、使い捨ての変更と範囲で一度ずつ実行し、レビューを始める前に止まることを確かめる。
    -   §7 の「PR-1 の実行の記録」には、判定の結果（通った・通らなかった）と照らした規則だけを書き、レビューが返したモデル ID の文字列は書かない（後の Opus のレビューが本計画を読むため。設計書 3.5.5）。
    -   Opus で走らせられずに止まったときは、何も push せずに止まって報告し、Opus が使えるようになってからこのステップをやり直す。
    -   Critical と Major の指摘は、step 7 の指摘と同じく、この PR の範囲で直す。
-   [x] **ステップ 1-7**: `_checks/moved_checklist.sh` を書いて実行する（AC-28）。マージの基点のコミットと、比べるコミットの 2 つを引数で受け取り、基点の `weakreview.md` の「Weak-model failure checklist」の項目と事前の確かめのコードブロックが、比べるコミットの `_lib/opus-review.md` に同じ文で、同じ順にあることを確かめる。見出しの階層の違い、コードブロックの字下げ（リストの入れ子による）の違い、ステップ 1-1 の置き換えの一覧にある置き換えだけを許す。各チェックが少なくとも 1 つの壊し方で失敗することを確かめ、その結果を commit のメッセージに書く。実行の出力・終了コード・2 つのコミットの ID を §7 の「PR-1 の実行の記録」に書く。ステップ 1-6 の直しの後に行う。
-   [x] **ステップ 1-8**: AC-13 の差分の記録を書く（§7 の「差分の記録」の PR-1 の欄）。

**確かめ直し。** ステップ 1-7 の比べたコミットより後に `_lib/opus-review.md` か `_checks/moved_checklist.sh` を変える commit が入ったら（PR を作った後の `/fixpr` の修正を含む）、ステップ 1-7 をやり直して §7 の記録を改める。

**完了の条件:** 設計書 3.4（残した指摘の記録と Opus かどうかの規則）、3.5.3〜3.5.5、3.7、3.11.1 の 1 つ目、3.11.3 の項目がすべて文書にある。§7 にステップ 1-6・1-7 の記録と AC-13 の PR-1 の記録がある。`bash -n` が `_checks/moved_checklist.sh` に通る（`shellcheck` があれば、それも通る）。`make test` と `make lint` が通る。

### PR-1 作成ポイント: Opus review procedure and weakreview

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8

**推奨タイトル**: `feat(0081): Opus のレビューの共通の手順を作り、/weakreview で二つのレビューを一度に行う`

**レビュー観点**: 正しさのレビューのチェックリストが cleanup とテストの網羅を含まないか / Opus で走ったことの確かめと止まる条件 / 直しで指摘を退けない規則と残した指摘の記録 / 移したチェックリストが移す前と同じか

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-1 が、繰り返しの上限と止まる条件を持つ直しと確認のループ（状態機械に近い手順）を新しく書く。1-2・1-6・1-7 がこれに依るので、PR の中で隔離はできない。Sonnet で実装し、旧い値の意味に従って `/weakreview` で確かめるときは、このブランチで書き換え中の `weakreview.md` ではなく、`origin/main` の `weakreview.md` で行う。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 読む側と `/runplan` への組み込み（設計書 8 章の 2）

**変更するファイル:** `.claude/commands/_context.md`、`.claude/commands/_lib/opus-review.md`、`.claude/commands/_lib/opus-review-decision.sh`（新規）、`.claude/commands/runplan.md`、`.claude/commands/runpr.md`、`.claude/commands/_lib/task-pr-handoff.md`、`CLAUDE.md`、`docs/dev/adr/0042-implementation-model-tiers.md`、`docs/tasks/0081_implementation_model_choice/_checks/decision_cases.sh`（新規）、本計画

-   [ ] **ステップ 2-1**: `_context.md` の PR marker conventions の `:64` の項目を、設計書 3.1 の 5 つ（値・値の場所・読み方・読み替えの表・読んだ結果から勧めるモデルへの対応）に置き換える（AC-09、AC-37 の読む側）。1 の「値を書くコマンド」には、この PR では `mkplan2.md` step 4 だけを挙げる。単一フェーズの計画書を書く `mkplan.md` は、PR-3 で足す（ステップ 3-4）。値の読み方の節は、それを実行する形として判断のスクリプトを名指す。`_context.md` は英語の文書なので、読み替えの表の見出しも英語で書く（判断のスクリプトは表の見出しと照合しない）。
-   [ ] **ステップ 2-2**: 判断のスクリプト `_lib/opus-review-decision.sh` を書く（設計書 3.5.1、3.5.2、§1.3 の決定 1・3・5・7）。
    -   引数は PLAN・TARGET（`### PR-N 作成ポイント` の `N` か `single`）・BASE。出力は `RUN`・`NONE <reason>`・`SKIP <done-line>` の 1 行と、`SKIP` のときに続く commit の一覧。どの失敗でも `RUN` を出力する。
    -   値は、`origin/main` の PLAN と `HEAD` の PLAN（`git show HEAD:<PLAN>`）の両方から、設計書 3.1 の場所と読み方で読む。計画書のフェンスで囲まれたコードブロックの中は読まない（差分の記録の中身を値の行として読まないため）。旧い値は、`origin/main` の `_context.md` の読み替えの表で読む。
    -   完了の記録と実装の記録は、記録の行の形（設計書 3.4）に完全に合う行だけを読む。除いてよい commit の定義は、設計書の冒頭のとおりである。
    -   冒頭のコメントで、規則の正本（値の読み方と読み替えの表は `_context.md`、記録の行の形と Opus かどうかの規則は `_lib/opus-review.md` の「Record lines」）を名指す。
-   [ ] **ステップ 2-3**: `_lib/opus-review.md` に、実装の記録（`Implementation model:`）と完了の記録（`Opus review: done`）の形、除いてよい commit の定義、入口 1 を足す（設計書 3.4、3.5.1、3.5.2）。入口 1 は、判断のスクリプトの `origin/main` のコピーを作業ツリーの外に取り出して `bash` で走らせること（§1.3 の決定 6）と、取り出せないとき・出力が決まった形でないとき・終了コードが 0 でないときは `RUN` とすることを定める。「Record lines」の節は、判断のスクリプトがそれを読む実行の形であることを名指し、記録の行の種類ごとに具体的な例を一つずつ置く。例のモデル ID は、版を含む実在の ID ではなく、規則だけを満たす架空の ID にする（設計書 3.5.5）。
-   [ ] **ステップ 2-4**: `_checks/decision_cases.sh` を書いて実行する（AC-07、AC-09〜AC-12、AC-18〜AC-20）。作業ツリーの判断のスクリプトを、場合ごとに作る fixture のリポジトリで走らせ、標準出力（`SKIP` の後の commit の一覧を含む）と終了コードが期待どおりかを確かめる。
    -   場合は、設計書 3.1 の読み方の各規則、3.4 の記録の行の各規則、3.5.2 の各条件（除いてよい commit、merge commit、記録の範囲の重なり、完了の記録の条件）、ステップ 2-2 のコードブロックを読まない規則、設計書 4.2 の表のうち判断に関わる行を覆う。判断のコマンドの失敗は、`origin/main` の参照がない、PLAN が読めない、git のリポジトリでない、などで起こす。
    -   各 fixture の `origin/main` の `_context.md` は、比べるコミットの実物（`git show <commit>:.claude/commands/_context.md`）から作る。表がない場合と、表の形が崩れた場合だけ、手で作る。
    -   実物の計画書でも走らせる。比べるコミットの 0077 の計画書（旧い値）と、値の行を持たない単一フェーズの計画書（0071）である。
    -   fixture の記録の行は、比べるコミットの `_lib/opus-review.md` の「Record lines」の例から作る。モデル ID は、Opus かどうかの規則だけを満たす架空の ID（`claude-opus-` で始まるものと、そうでないもの）にし、版を含む実在の ID を書かない。PR-2 と PR-3 の Opus のレビューがこのスクリプトを読むためである（設計書 3.5.5）。
    -   fixture は `mktemp -d` の下に作り、`trap` で消す。git は、利用者の設定を読まないように（`GIT_CONFIG_GLOBAL=/dev/null`・`GIT_CONFIG_NOSYSTEM=1`）、決まった名前と日時で走らせる。出力は、場合ごとの PASS か FAIL と、期待した値と得た値の最初の語にし、fixture の commit の ID は出さない。
    -   各場合が、判断のスクリプトの対応する箇所を壊すと失敗することを確かめ、その結果を commit のメッセージに書く。clean な作業ツリーで実行し、出力・終了コード・コミットの ID・2 つのスクリプトの blob の ID を §7 の「PR-2 の実行の記録」に書く。
-   [ ] **ステップ 2-5**: `runplan.md` を書き換える（設計書 3.6）。
    -   冒頭の案内（3.6.1）。
    -   step 4 の後、step 4.5 の前の案内の表示（3.6.2）。step 4.5 でのモデル ID の確かめ（3.6.3）。
    -   step 7.7 の後の実装の記録の空の commit（3.6.3）。step 7.7 の「最初の commit」の記述との関係を合わせる。
    -   step 5a と step 8 の、ブランチ全体のローカル CLI レビュー・要否の判断・Opus のレビュー・グリーンゲート・完了の記録・PR_TEXT・LOCAL_REVIEW（3.6.4 の 1〜6）。Opus のレビューが直しの commit を作ったときは、グリーンゲートに加えて、最後の PR のときの AC の確かめ（今の `:85`）と差分の記録の再確認（今の `:86`）もやり直す（§1.4）。それらがビルドの直しでも除いてよい commit でもない commit を作ったときは、要否の判断からやり直す（設計書 3.6.4 の 4）。単一フェーズの計画書の step 8 では、完了の記録を `RUN` のときだけ書く。
    -   push の前のやり直し（3.6.4 の後半）を、step 5a と step 8 が Part B に渡す CHECKS の定義に書く。CHECKS はグリーンゲートであり、Part B step 2 がグリーンゲートを通すために commit を作ったときは、push の前に要否の判断をやり直し、`RUN` のときは Opus のレビュー・完了の記録・PR_TEXT の Opus のレビューの結果も新しい `HEAD` に合わせる。やり直したレビューが直しの commit を作ったときは、グリーンゲート・最後の PR のときの AC の確かめ・差分の記録の再確認もやり直す（設計書 1.2 の決定 11）。今の CHECKS の「ブランチのレビューが commit を作ったら」という文（`:90-92`）は、新しい順序に合わせて書き換える。
    -   再び実行したときの振る舞い（3.6.4 の後半）。
-   [ ] **ステップ 2-6**: `runpr.md` の step 4 と step 5 の `/clear` の案内を書き換える（設計書 3.8）。見出しは変えない。
-   [ ] **ステップ 2-7**: `_lib/task-pr-handoff.md` の PR_TEXT と Part B step 3 の本文の項目を書き換える（設計書 3.9）。step 3 と step 4 の書き出しは変えない。
-   [ ] **ステップ 2-8**: `CLAUDE.md` の PR を作るコマンドの段落に、`/runplan` が条件に応じて Opus のレビューを行うことを足す（設計書 3.11.1 の 2 つ目）。
-   [ ] **ステップ 2-9**: ADR-0042 の食い違いの記述を、残るずれを述べる形に書き換える（§1.3 の決定 4）。残るずれは、PR-2 のブランチの状態に照らして確かめる（`mkplan2.md`・`mkplan.md`・`_lib/simple-process.md` が旧い値を書き、単一フェーズの計画書が値を持たない、など）。issue #760 への完全な URL のリンクは残す。commit のメッセージに訂正の行を書く。
-   [ ] **ステップ 2-10**: AC-13 の差分の記録を書く（§7 の「差分の記録」の PR-2 の欄）。

**確かめ直し。** ステップ 2-4 の比べたコミットより後に、判断のスクリプト・`_checks/decision_cases.sh`・`_context.md`・`_lib/opus-review.md` の「Record lines」を変える commit が入ったら（PR を作った後の `/fixpr` の修正を含む）、ステップ 2-4 をやり直して §7 の記録を改める。

**完了の条件:** 設計書 3.1、3.4（実装の記録と完了の記録）、3.5.1、3.5.2、3.6、3.8、3.9、3.11.1 の 2 つ目の項目がすべて文書にある。§7 にステップ 2-4 の記録と AC-13 の PR-2 の記録がある。`bash -n` が判断のスクリプトと `_checks/decision_cases.sh` に通る（`shellcheck` があれば、それも通る）。`make test` と `make lint` が通る。

### PR-2 作成ポイント: review decision and runplan integration

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 / 2-10

**推奨タイトル**: `feat(0081): 実装モデル要件の新しい値を読み、/runplan が PR を開く前に Opus のレビューを行う`

**レビュー観点**: 判断のスクリプトが迷うとき・失敗したときに RUN に倒れるか / 値・読み替えの表・スクリプトを origin/main から読むこと / step 5a と step 8 の順序と push の前のやり直し / 再び実行したときの振る舞い

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-1・2-2 は、旧い値から新しい値への読み替えを、値を書く側（PR-3）より先に、読む側へ段階的に入れる移行である。これは `mkplan.md` step 8 の panel mode が挙げる migration の契機に当たる。ステップ 2-5 も、push の前のやり直しを含む順序を持つ。2-2 と 2-5 は、3 本の PR の決定（§1.3 の決定 2）のもとでは隔離できないので、2-4 の fixture の実行で判断の論理を細かく確かめる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 書く側（設計書 8 章の 3）

**変更するファイル:** `.claude/commands/mkplan2.md`、`.claude/commands/mkplan.md`、`.claude/commands/_lib/simple-process.md`、`.claude/commands/_context.md`、`docs/dev/adr/0042-implementation-model-tiers.md`、`docs/tasks/0081_implementation_model_choice/_checks/old_values.sh`（新規）、`docs/tasks/0081_implementation_model_choice/_checks/decision_cases.sh`、本計画

-   [ ] **ステップ 3-1**: `mkplan2.md` の step 4 の「Model requirement」、step 6 の書式と `**判定理由**` の規則、step 7 の表の例、step 9 のペルソナとチェックリストを新しい値にする（設計書 3.2）。値の意味と判定理由の書き方は step 4 だけが定め、step 6 は step 4 を名指す（AC-05）。
-   [ ] **ステップ 3-2**: `mkplan.md` の step 8.5 に、単一フェーズの計画書の「PR 構成」の節と値の 2 行を書くことを足す。「Technical correctness checklist」に、その項目を足す（設計書 3.3）。
-   [ ] **ステップ 3-3**: `_lib/simple-process.md` の「Writing the three documents」に、単一フェーズの計画書の値を書くことを足す（設計書 3.3）。
-   [ ] **ステップ 3-4**: `_context.md` の PR marker conventions の値の項目に、値を書くコマンドとして `mkplan.md` の step 8.5（単一フェーズの計画書）を足す（設計書 3.1 の 1、AC-37 の書く側）。
-   [ ] **ステップ 3-5**: ADR-0042 の「決定」の食い違いの記述を消す（設計書 3.11.2、§1.3 の決定 4）。commit のメッセージに訂正の行を書く。
-   [ ] **ステップ 3-6**: `_checks/old_values.sh` を書いて実行する（AC-01、AC-04、AC-05、AC-39）。比べるコミットの `.claude/commands/` の下で、`frontier-required`・`frontier-recommended` と、単語としての `standard` を探す。`standard` の値でない使い方（`standard error`・`standard output`・`standard input`・`standard library`・`standard-library`・`standard classDef` など）は、スクリプトの中の名前の付いた一覧で除く。残ったすべての箇所が `_context.md` の PR marker conventions の節にあることを確かめ、ほかの箇所はすべて表示して失敗する。値でない使い方を除く一覧に語句を足すときは、その語句の前後の文を実行の記録に示せばよい。値としての使い方を `_context.md` の外に許す必要が出たときは、計画の決定の変更として人と決める（`runplan.md` step 5 の「Decision change」）。各チェックが少なくとも 1 つの壊し方で失敗することを確かめ、その結果を commit のメッセージに書く。実行の出力・終了コード・コミットの ID を §7 の「PR-3 の実行の記録」に書く。
-   [ ] **ステップ 3-7**: `_checks/decision_cases.sh` に、ステップ 3-2 の `mkplan.md` step 8.5 が書かせる形の単一フェーズの計画書の場合を足し、PR-3 のブランチで実行する（AC-06・AC-07・AC-08。ステップ 3-4 が `_context.md` を変えるので、フェーズ 2 の「確かめ直し」にも当たる）。結果を §7 の「PR-3 の実行の記録」に書く。
-   [ ] **ステップ 3-8**: AC-13 の差分の記録を書く（§7 の「差分の記録」の PR-3 の欄）。

**確かめ直し。** ステップ 3-6 か 3-7 の比べたコミットより後に `.claude/commands/` か `_checks/` のスクリプトを変える commit が入ったら（PR を作った後の `/fixpr` の修正を含む）、そのステップをやり直して §7 の記録を改める。

**完了の条件:** 設計書 3.1 の 1（書く側）、3.2、3.3、3.11.2 の項目がすべて文書にある。§7 にステップ 3-6・3-7 の記録と AC-13 の PR-3 の記録がある。`bash -n` が `_checks/old_values.sh` と `_checks/decision_cases.sh` に通る（`shellcheck` があれば、それも通る）。`make test` と `make lint` が通る。

### PR-3 作成ポイント: writing the new values

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8

**推奨タイトル**: `feat(0081): /mkplan2 と /mkplan が実装モデル要件の新しい値を書く`

**レビュー観点**: 値の意味と判定理由の書き方が mkplan2.md step 4 だけにあるか / 単一フェーズの計画書の PR 構成の節と値の 2 行 / 旧い値が _context.md の外に残らないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-1〜3-4 は、PR-2 が始めた移行の書く側の半分で、後のすべての計画書が従う値の意味と契機（`mkplan2.md` step 4）を書き換える。ステップ 3-7 は判断のスクリプトの確かめをやり直す。PR-3 は、PR-2 のマージの後は `sonnet+opus-review` と読まれ、PR-2 で入れた Opus のレビューの仕組みを初めて実地で使う PR になる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（フェーズ 1） | `_lib/opus-review.md` の入口 2、`/weakreview` | `/weakreview` 一つで、Opus の二つのレビューを行い、既定で直せる |
| M2（フェーズ 2） | 入口 1、判断のスクリプト、`_context.md` の読み方、`/runplan`・`/runpr` | 旧い値を読み替えの表で読み、Sonnet で実装した phase group を含む PR で `/runplan` が Opus のレビューを行う。この PR のマージの後に作ったブランチから効く（設計書 8 章） |
| M3（フェーズ 3） | `/mkplan2`・`/mkplan` が新しい値を書く | 新しい計画書が新しい値と、単一フェーズの計画書の値を持つ |

順序は設計書 8 章のとおりで、新しい値を読むコマンドを、書くコマンドより先に入れる。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 | `_lib/opus-review.md` の入口 2、`/weakreview`、ADR-0016 の訂正 | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 / 2-10 | 判断のスクリプト、入口 1、`_context.md` の読み方、`/runplan`・`/runpr`・`_lib/task-pr-handoff.md` | frontier-required |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 | `/mkplan2`・`/mkplan`・`_lib/simple-process.md` が新しい値を書く、旧い値の残りの確かめ | frontier-recommended |

実装モデル要件は、本計画を書いた時点の `/mkplan2` の旧い値で書いた。PR-2 のマージの後は、`_context.md` の読み替えの表で読まれる（設計書 8 章）。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 足さない（設計書 7.1、§1.2）。製品のコードは変わらない。
-   **`_checks/` のスクリプト:** 3 つのスクリプトを、それぞれの PR のブランチで実行し、出力を §7 に記録する。各チェックが壊し方で失敗することは、実装する人が壊して確かめ、commit のメッセージに書く。
    -   `moved_checklist.sh`（PR-1）: 移したチェックリストと事前の確かめが、移す前と同じである。
    -   `decision_cases.sh`（PR-2、PR-3）: 判断のスクリプトの各場合の出力。
    -   `old_values.sh`（PR-3）: 旧い値が、`_context.md` の PR marker conventions の外に残っていない。
-   **差分の記録:** AC-13（0073・0076・0077 の計画書が変わらない）を、PR ごとに記録した差分の読み取りで確かめる。比べるパスは 3 つの計画書で、差分は空になるはずなので、読む量は小さい。
-   **実地の確かめ:** PR-1 で `/weakreview` を実際に走らせる（ステップ 1-6）。直す既定の経路（直して commit し、確認のレビューを走らせる）は、費用を抑えるために走らせず、読んで確かめる。`/runplan` の Opus のレビューの実地の確かめは、PR-2 のマージの後に作ったブランチで初めて行われる（設計書 7.4、8 章）。PR-3 の値（`frontier-recommended`、`sonnet+opus-review` と読まれる）を Sonnet で実装すれば、PR-3 がその最初の例になる。
-   **既存の guard:** `make test` が、§1.4 の guard を走らせる。
-   **後方互換:** 旧い値を持つ計画書の読み替えは、`decision_cases.sh` の旧い値の場合と、0077 の実物の場合で確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 判断のスクリプトの誤りで、Opus のレビューを行うべき PR で行わない | 迷うとき・失敗したときは `RUN` に倒す（設計書 1.1）。`decision_cases.sh` で各規則と失敗の場合を確かめる |
| 読み替えの表の形を読み違え、旧い `standard` の PR で要らない Opus のレビューを行う | 安全側の誤りだが費用がかかる。`decision_cases.sh` は、実物の `_context.md` と 0077 の計画書で確かめる |
| セッションが値・表・スクリプトをブランチの上で書き換え、レビューを外す | 三つとも `origin/main` から読む（設計書 1.2 の決定 9、§1.3 の決定 3・6）。品質を保つ仕組みで、セキュリティの境界ではない（設計書 5.2） |
| Opus でないモデルがレビューを走らせ、Opus の ID を写して報告する | 共通の手順と prompt と本計画の記録に、具体的なモデル ID を書かない（設計書 3.5.5、ステップ 1-6）。残る危うさとして受け入れる |
| PR-2 の前に作ったブランチの `/runplan` は、新しい手順を使わない | 設計書 8 章のとおり受け入れる。PR-2 のマージの後に作るブランチから効く |
| PR-2 のブランチで新しいセッションが `/runplan` を始めると、ブランチの `runplan.md` を読み、`origin/main` にスクリプトがないので `RUN` になる | 安全側に倒れた結果として受け入れる。PR-2 の Opus のレビューを行う |
| PR-1 のマージの後、PR-3 のマージまで、ADR-0042 の食い違いの記述がずれる | 各 PR で、残るずれを述べる形に書き換える（§1.3 の決定 4） |
| 新しい文が既存の guard を壊す | §1.4 の制約を守り、各ステップの後に `make test` を通す |
| PR-1 を Sonnet で実装したとき、確かめに使う `/weakreview` が、このブランチで書き換え中のものになる | PR-1 の `判定理由` のとおり、`origin/main` の `weakreview.md` で確かめる |
| 実地の確かめ（ステップ 1-6）の費用 | 一度だけ、`--no-fix` で行う。費用は要件書 1 節の目安（約 $1.5〜4）の範囲である |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 / 2-10）
-   [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の意味は次のとおりである。

-   `test`（一回限り）: `_checks/` のスクリプトで、対象を実行して振る舞いを確かめる。PR のブランチで実行し、出力・終了コード・コミットとスクリプトの ID を下の実行の記録に書く。`make test` と CI は実行しない。
-   `static`（一回限り）: `_checks/old_values.sh` で、旧い値という決まった値（列挙された 3 つの値の名前）が残っていないことを確かめる。文の語を探すのではない。記録は `test` と同じである。
-   `diff`: `docs/dev/conventions/requirements_process.md` §4 のとおり。
-   `manual`: PR のレビューで、その文書の該当箇所を読んで確かめる。コマンドの定義の手順の文だけを定める AC は、`manual` だけで確かめる（§1.2）。

`test` と `static` の実行の記録があることは、各フェーズの完了の条件であり、PR のレビューで確かめる。各フェーズの「確かめ直し」は、その commit を作る人が行う。PR を作る前は `/runplan` のセッション、PR を作った後は `/fixpr` の回か手で commit する人である。`/runpr` の PRE_MERGE は `_checks/` のスクリプトを走らせないので（`docs/dev/conventions/requirements_process.md` §4）、レビュアーは `/runpr` の質問に答える前に、記録に書いた比べたコミットより後にスクリプトの対象を変える commit がないかを確かめる。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `_checks/old_values.sh`、`mkplan2.md` の書式と説明を読む | static、manual | ステップ 3-1 | PR-3 |
| AC-02 | `mkplan2.md` step 4 を読む | manual | ステップ 3-1 | PR-3 |
| AC-03 | `mkplan2.md` step 6 の `**判定理由**` の規則を読む | manual | ステップ 3-1 | PR-3 |
| AC-04 | `_checks/old_values.sh`（表の例とチェックリスト）、step 7 と step 9 を読む | static、manual | ステップ 3-1 | PR-3 |
| AC-05 | `_checks/old_values.sh`、値の意味と判定理由の書き方が step 4 だけにあることを読む | static、manual | ステップ 3-1 | PR-3 |
| AC-06 | `_checks/decision_cases.sh`（`mkplan.md` step 8.5 が書かせる形の場合）、`mkplan.md` step 8.5 とチェックリストを読む | test、manual | ステップ 3-2・3-7 | PR-3 |
| AC-07 | `_checks/decision_cases.sh`（単一フェーズの計画書の「PR 構成」の節から値を読む場合）、`runplan.md` と `runpr.md` を読む | test、manual | ステップ 2-2・2-5・2-6・3-7 | PR-2、PR-3 |
| AC-08 | `_checks/decision_cases.sh`（ステップ 3-7 の場合）、`_lib/simple-process.md` を読む | test、manual | ステップ 3-3・3-7 | PR-3 |
| AC-09 | `_checks/decision_cases.sh`（旧い値を実物の `_context.md` の表で読む場合と、表がない・崩れた場合）、`_checks/old_values.sh`、`_context.md` を読む | test、static、manual | ステップ 2-1・2-2・3-6 | PR-2、PR-3 |
| AC-10 | `_checks/decision_cases.sh`（旧い値の場合）、`runplan.md` と `runpr.md` の示し方を読む | test、manual | ステップ 2-2・2-5・2-6 | PR-2 |
| AC-11 | `_checks/decision_cases.sh`（読めない値の場合）、`runplan.md` と `runpr.md` を読む | test、manual | ステップ 2-2・2-5・2-6 | PR-2 |
| AC-12 | `_checks/decision_cases.sh`（値の欄がない場合）、`runplan.md` と `runpr.md` を読む | test、manual | ステップ 2-2・2-5・2-6 | PR-2 |
| AC-13 | 記録した差分の読み取り（下の「差分の記録」） | diff | 全ステップ | PR-1、PR-2、PR-3 |
| AC-14 | `runplan.md` の冒頭の案内を読む | manual | ステップ 2-5 | PR-2 |
| AC-15 | `runplan.md` の案内の表示を読む | manual | ステップ 2-5 | PR-2 |
| AC-16 | `runplan.md` の冒頭の案内を読む | manual | ステップ 2-5 | PR-2 |
| AC-17 | `runpr.md` の step 4 と step 5 を読む | manual | ステップ 2-6 | PR-2 |
| AC-18 | `_checks/decision_cases.sh`（実装の記録を読む場合と、形の違う行の場合）、`runplan.md` を読む | test、manual | ステップ 2-2・2-3・2-5 | PR-2 |
| AC-19 | `_checks/decision_cases.sh`（`RUN` の各場合）、`runplan.md` step 5a と step 8 を読む | test、manual | ステップ 2-2・2-5 | PR-2 |
| AC-20 | `_checks/decision_cases.sh`（`NONE` の各場合）、`runplan.md` を読む | test、manual | ステップ 2-2・2-5 | PR-2 |
| AC-21 | `_lib/opus-review.md` 入口 2 と `runplan.md` の RANGE を読む | manual | ステップ 1-1・2-5 | PR-1、PR-2 |
| AC-22 | `_lib/opus-review.md` の統合・直し・確認・上限と、`runplan.md` の上限で止まるときの振る舞いを読む | manual | ステップ 1-1・2-5 | PR-1、PR-2 |
| AC-23 | `_lib/opus-review.md` の退けない規則と、`runplan.md` の PR を開く前に尋ねることを読む | manual | ステップ 1-1・2-5 | PR-1、PR-2 |
| AC-24 | `_lib/opus-review.md` の DECISION_CHANGE と `runplan.md` を読む | manual | ステップ 1-1・2-5 | PR-1、PR-2 |
| AC-25 | `_lib/opus-review.md` の止まる条件と `runplan.md` の止まり方を読む。モデル ID の確かめを実行した記録（ステップ 1-6） | manual | ステップ 1-1・1-6・2-5 | PR-1、PR-2 |
| AC-26 | `runplan.md` step 5a と step 8 の順序を読む | manual | ステップ 2-5 | PR-2 |
| AC-27 | `_lib/task-pr-handoff.md` と `runplan.md` の PR_TEXT を読む | manual | ステップ 2-5・2-7 | PR-2 |
| AC-28 | `_checks/moved_checklist.sh`（弱いモデルの失敗のチェックリストが移す前と同じ。出力と 2 つのコミットの ID は「PR-1 の実行の記録」に書く）、「Correctness checklist」を読む、実地の確かめ（ステップ 1-6） | diff、manual | ステップ 1-1・1-6・1-7 | PR-1 |
| AC-29 | `_lib/opus-review.md` のモデルの指定と確かめを読む。モデル ID の確かめを実行した記録（ステップ 1-6） | manual | ステップ 1-1・1-6 | PR-1 |
| AC-30 | `_lib/opus-review.md` の統合と報告を読む、実地の確かめ（ステップ 1-6） | manual | ステップ 1-1・1-6 | PR-1 |
| AC-31 | `weakreview.md` と `_lib/opus-review.md` の直しの規則を読む | manual | ステップ 1-1・1-2 | PR-1 |
| AC-32 | `weakreview.md` の引数の解釈を読む | manual | ステップ 1-2 | PR-1 |
| AC-33 | `weakreview.md` の直すときの確かめを読む。止まる条件を実行した記録（ステップ 1-6） | manual | ステップ 1-2・1-6 | PR-1 |
| AC-34 | `weakreview.md` の添え書きを読む | manual | ステップ 1-2 | PR-1 |
| AC-35 | `weakreview.md` の Purpose を読む | manual | ステップ 1-2 | PR-1 |
| AC-36 | `weakreview.md` の範囲の指定とそろえ方を読む | manual | ステップ 1-2 | PR-1 |
| AC-37 | `_context.md` の PR marker conventions を読む | manual | ステップ 2-1・3-4 | PR-2、PR-3 |
| AC-38 | `CLAUDE.md` の `/weakreview` の行と `weakreview.md` の `argument-hint` を読む | manual | ステップ 1-2・1-3 | PR-1 |
| AC-39 | `_checks/old_values.sh`、「`weakreview.md` で確かめる」という案内の文が消えたことを読む | static、manual | ステップ 2-5・3-1・3-6 | PR-2、PR-3 |
| AC-40 | `cmd/yt2column/adr_test.go::TestADRRecords`（ADR の形）、ADR-0042 の「決定」を読む | static、manual | ステップ 1-5・2-9・3-5 | PR-1、PR-2、PR-3 |

### 差分の記録

AC-13 は、記録した差分の読み取りで確かめる。比べるパスは次の 3 つで、PR ごとに記録する。

PR-1:

Diff record (AC-13):

- Compared paths: `docs/tasks/0073_adr_context_decision_review/03_implementation_plan.md` `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md` `docs/tasks/0077_openai_llm_client/03_implementation_plan.md`
- Merge-base: `04dc4824145e2243bbf8dcfa82c5ff853c43eb11`
- Compared commit: `49b7627aff9c347c111f2fe29bd6ea6a35ded09b`

Path diff:

```text
```

PR-2:

Diff record (AC-13):

- Compared paths: `docs/tasks/0073_adr_context_decision_review/03_implementation_plan.md` `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md` `docs/tasks/0077_openai_llm_client/03_implementation_plan.md`

PR-3:

Diff record (AC-13):

- Compared paths: `docs/tasks/0073_adr_context_decision_review/03_implementation_plan.md` `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md` `docs/tasks/0077_openai_llm_client/03_implementation_plan.md`

### PR-1 の実行の記録

**ステップ 1-6（`/weakreview` の実地の確かめ）。** このブランチで、書き換えた `weakreview.md` の手順に従い、`--no-fix` で実行した（範囲 `04dc4824145e..e47b01579f48`、タスクはブランチの名前から特定した）。

-   二つのレビューの出力の最初の行を、`_lib/opus-review.md`「Running on Opus」の確かめ（最初の行だけを書いたファイルに `grep -Eqx 'Model: claude-opus-[!-~]+'`）で判定した。どちらも通った（終了コード 0）。照らした規則は「Record lines」の Opus かどうかの規則（`claude-opus-` で始まる）である。確認のレビュー（2 回）の最初の行も、同じ確かめで通った。
-   同じ確かめを、Sonnet の ID を書いた行（`Model: claude-sonnet-` で始まる行）と、形の合わない 4 つの行（`:` の後に空白がない、`model:` が小文字、`claude-opus-` の後が空、ID の後に語が続く）に当てた。どれも通らなかった（終了コード 1）。
-   統合した報告の形: 二つのレビューの指摘を統合し、重複を除いて高い重大度をとった結果は、Critical 1・Major 3・Minor 9 だった。各チェックリストの分類の行（clean・flagged・該当なし）と全体の見立ての 1 行を得た。
-   直すときの止まる条件は、`/weakreview` を呼び出してではなく、手順の step 4 の確かめのコマンドを書いたとおりに並べたスクリプトで確かめた（報告の文はそのスクリプトのもので、`weakreview.md` の文ではない）。範囲の終わりを `HEAD~1` にしたときは「range end is not HEAD」で、使い捨ての追跡されないファイルを置いたときは「working tree not clean」で、どちらもレビューを始める前に止まった（終了コード 1）。clean な作業ツリーで範囲の終わりが `HEAD` のときは通った。
-   実地で見つかった手順の不具合を直した。タスクの文書を変えない範囲でタスクを特定できなかった（ブランチの名前からの特定を足した、835e7bc6）。事前の確かめのブロックを zsh で走らせると `path` の変数が `PATH` を書き換えた（`bash` で走らせると書いた、e47b0157）。
-   `--no-fix` の実行は `reported` で終わり、確認のレビューを走らせない。Critical と Major の指摘は、本ステップの「この PR の範囲で直す」に従い、セッションが共通の手順の「Fixing and verifying」に沿って直し（bf281603、b99db37e）、確認のレビューを Opus で 2 回走らせた。2 回目で Critical と Major が残らなかった。残った Minor（コミットメッセージに位置する指摘の、残した指摘の記録の形がない、ほか）は PR の説明に書く。

**ステップ 1-7（`_checks/moved_checklist.sh`）。** `bash docs/tasks/0081_implementation_model_choice/_checks/moved_checklist.sh 04dc4824145e2243bbf8dcfa82c5ff853c43eb11 1121fb36b589ee11e681024a13f68e48be602a30` の出力（終了コード 0）:

```text
PASS replacement: 'RANGE="<resolved range>"' -> 'RANGE="<RANGE>"'
PASS replacement: 'docs/tasks/<task-dir>/' -> '<TASK_DIR>/'
PASS replacement: 'step-3' -> 'pre-check'
PASS checklist: 185 lines identical
PASS pre-check block: 48 lines identical
```

マージの基点 `04dc4824145e2243bbf8dcfa82c5ff853c43eb11`、比べたコミット `1121fb36b589ee11e681024a13f68e48be602a30`。各チェックを壊して失敗することは、49b7627a のメッセージに書いた。

### PR-2 の実行の記録

（ステップ 2-4 で書く。）

### PR-3 の実行の記録

（ステップ 3-6 と 3-7 で書く。）

## 8. 成功の基準 (Success Criteria)

-   要件書の AC-01〜AC-40 が、§7 の確かめで満たされている。
-   §7 に、`_checks/` のスクリプトの実行の記録と、AC-13 の 3 つの差分の記録がある。
-   各 PR で `make test` と `make lint` が通る。
-   ADR-0042 の「決定」に食い違いの記述が残らず、ADR の索引の行がある。
-   セキュリティ: 要件書 4.2 節のとおり、条件付きのガイドの契機に当たらない。GitHub に書き込むコマンドを新しく加えない（§1.4）。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後に作ったブランチで `/runplan` を初めて実行するとき、値が `sonnet+opus-review` か `opus` の PR で、Opus のレビューが行われ、PR の説明に結果が載ることを見る（設計書 7.4）。
-   PR がまだ作られていない PR マーカーに旧い値を持つ計画書（0073・0076・0077 と、本計画）がなくなったら、読み替えの表を消せる（要件書 4.5 節）。

## 変更履歴

- 2026-11-11 承認（isseis、#816）
