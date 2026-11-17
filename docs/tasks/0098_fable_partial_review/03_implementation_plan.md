# 実装計画書：Fable の部分レビューをタスクのコマンドに組み込み、Codex の PR 再レビューの条件を絞る

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-17 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号と「決定 N」は [02_architecture.md](02_architecture.md)（以下、設計書）の節と 1.2 の決定を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `20c8b986`（`origin/main`）で確かめた。設計書が確かめたコミットは `d76547e1` である。その後に変わったのは、`runpr.md` の step 5 の報告（1302 行目より後）と、`CLAUDE.md` の拡張のコマンドの一覧（258 行目付近で 1 行増えた）だけである（`git diff --stat d76547e1 20c8b986 -- .claude CLAUDE.md cmd docs/dev/guides`）。そのため、設計書が引く行番号は、`CLAUDE.md` の 258 行目より後を除いて今も合う。

用語は、要件書 6 章と設計書の冒頭（Escalation・COVERAGE・Fable の手順・判定のスクリプト・並列の経路・単独の経路・Fable の範囲・範囲の一覧・記録の行・Fable の行・Fable の記録・チェックポイントの印・見た範囲の行・記録の commit・空の commit・中身のない commit・除外できる commit）と同じ意味で使う。本計画は、次の用語を加える。

-   **スクリプトのテスト:** 判定のスクリプトを一時的な git のリポジトリの上で走らせる Go のテスト。`cmd/yt2column/review_records_test.go` に置く（§1.3 の決定 2）。
-   **実地の確かめ:** PR-2 のマージの後、本物のセッションで Fable の部分レビューを走らせて確かめること（設計書 7.4、§1.3 の決定 1）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

コマンドの定義・`_lib/` の共通の手順・`CLAUDE.md`・ガイドの文言を変え、判定のスクリプトとそのテストを加える。これにより、タスクの PR を開くコマンドが、PR を開く前に Fable の部分レビューを行い、`/runpr` が Codex の PR 再レビューを自動で頼む条件が絞られる（要件書 2.1 節）。何をどこに書くかは設計書 3 章が定める。本計画は、作業の順序と確かめ方を定める。

### 1.2. 実装の原則

-   **文面は設計書 3 章に従い、英語で書く。** コマンドの定義、`_lib/`、スクリプト、Go のテストは英語で書く。判定のスクリプトの、計画書の文と一致しなければならないリテラル（`PR を作成した` など）だけは日本語でよい（設計書の決定 22。ステップ 1-10 で `CLAUDE.md` の例外を広げる）。スクリプトのテストは、計画書の見本を `cmd/yt2column/testdata/review_records/` の下のファイルに置き、Go のリテラルに日本語を書かない。各ステップは、書く内容を設計書の節で指す。文面は実装の PR で書く。
-   **判定のスクリプトの規則は、スクリプトのテストで確かめる**（設計書の決定 15、7.1）。各テストの場合は、スクリプトの該当する規則を壊すと失敗することを、実装者が壊して確かめ、commit のメッセージに書く。
-   **コマンドの定義の手順の文だけを定める AC は、PR のレビューで読んで確かめる**（設計書 7.3）。Fable の手順の範囲の基準・指示・待ち方、トリアージの規則、記録を読んで PR の説明を書く手順がこれに当たる。値と構造（判定の規則、行の前置き、テストが読む step、スクリプトの呼び方）は、実行する確かめを持つ。前後の関係を述べる AC（AC-37、AC-40）は、差分の記録で確かめる。
-   **足す文は、コマンドの文書を読む既存の guard の制約を守る**（§1.4）。
-   **シェルスクリプトは bash 3.2 でも動く形で書く**（§1.3 の決定 3）。`#!/usr/bin/env bash` とし、`bash <path>` で走らせる。読むだけで、ネットワークに触れない。`git fetch origin main` は呼ぶ側が先に行う。

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **PR は設計書 8 章の目安どおり 3 本にする。PR-3 の自動送信は `hold` とする。** フェーズ 1 が PR-1（見たかの判定と Escalation）、フェーズ 2 が PR-2（Fable の部分レビュー）、フェーズ 3 が PR-3（ADR の確かめと実地の確かめ）である。実地の確かめのうち、タスク文書の PR の経路は、PR-2 のマージの後に、人がほかのタスクのタスク文書の PR を開いて確かめる。PR-3 の `/runplan` は、実装の PR の経路（最後の phase group の並列の経路と、チェックポイントの入口）を自分で通る。理由: PR-1 は Fable がなくても成り立ち、先に入れると後の PR の `/runpr` で `skip` の経路が働く（設計書 8 章）。実地の確かめは PR-2 が `main` に入った後でしかできず、結果を本タスクに記録するため。
    -   採らなかった案: 2 本にし、ADR の確かめを PR-2 に入れ、実地の確かめを本タスクの PR の外の作業にする案。実地の確かめの結果が本タスクに残らない。
    -   人の答え: 3 本で PR-3 を `hold` にする案を選んだ。
    -   ADR: なし（PR の分け方は、本タスクの中で閉じる）。
2.  **スクリプトのテストは `cmd/yt2column/review_records_test.go` に置き、`scriptBashes`（`cmd/yt2column/test_helpers_bash.go:21`）で、PATH の bash と、macOS の `/bin/bash` 3.2 の両方で走らせる。** 前例は `_lib/adr-correction-candidates.sh` のテスト（`cmd/yt2column/adr_candidates_docs_test.go:473`）である。`cmd/yt2column` のテストは、CI の `doc-guard` でも走る。ADR: なし。
3.  **判定のスクリプトは bash 3.2 でも動く形で書く。** 理由: スクリプトは、失敗すると安全な側（`coverage` は `PARTIAL`、`fable` は `MISSING`）に倒れる（設計書 3.6.3）。bash 4 を要すると、macOS の `/bin/bash` 3.2 で走ったときに、Escalation がいつも `ask` に寄り、Fable が毎回走るのに、誰も気づかない。
    -   採らなかった案: `_lib/opus-review-decision.sh` と同じく bash 4 を要する案。
    -   人の答え: bash 3.2 でも動く案を選んだ。
    -   ADR: なし（スクリプトの書き方で、本タスクの中で閉じる）。
4.  **コマンドの定義が判定のスクリプトを `origin/main` のコピーでだけ走らせることを、guard のテスト `TestReviewRecordsCallSite` で確かめる。** 前例は `TestADRCorrectionCandidatesCallSite`（`cmd/yt2column/adr_candidates_docs_test.go:577`）である。理由: 作業ツリーのコピーを走らせると、レビューを受けるブランチの変更で判定が変わり、関門が外れる。これは、`CLAUDE.md`「Testing Strategy」がテストを残すとする、エージェントが信頼の境界で走らせるコマンドに当たる。
    -   採らなかった案: テストを足さず、PR のレビューで読む案。
    -   人の答え: テストを足す案を選んだ。
    -   ADR: なし（テストの形で、本タスクの中で閉じる）。
5.  **PR-1 の `runpr.md` のマージの質問の要約の行は、Fable の部分を `なし` とだけ示し、PR-2 で Fable の記録の読み方を足す。** 理由: Fable の行の読み方は、PR-2 で作る Fable の手順の「Reading the record lines」に置く（設計書 3.2）。PR-1 の時点では、どのブランチにも Fable の記録はない（設計書 8 章の 1）。ADR: なし。
6.  **AC-32 は、既存の記録の行の前置きと新しい記録の行の前置きを手順の文書から読んで比べる guard のテスト `TestRecordLinePrefixesDistinct` で確かめる。** 前置きの値を比べるテストで、文の意味は読まない（`CLAUDE.md`「Testing Strategy」の、文書の値を比べるテスト）。記録の行だけを持つ空の commit が除外できる commit であることは、今の定義（`.claude/commands/_lib/opus-review.md:102-106`）のままで、本タスクは変えない。ADR: なし。

### 1.4. 既存コード調査結果

コミット `20c8b986` で確かめた。以下の `file:line` はこのコミットの行番号である。

-   **変えるファイルの今の箇所。** 設計書 2.1 と 2.2 の表のとおりである。主な箇所は次のとおり。
    -   `_lib/local-cli-review.md`：入力の節の GUARD の文（`:40-41`）、Terms の未レビューの範囲（`:73-79`）、step 4・5（`:191-239`）、step 6 の 2 回目の試行の条件（`:241-243`）、step 7 の表（`:257-262`）、「Reading the record lines」（`:306-321`）。
    -   `_lib/task-pr-handoff.md`：Part B step 1（`:184-199`。README の同期の commit が記録の行を載せる記述は `:193-196`）、step 3（`:203-271`）。
    -   `runplan.md`：step 4（`:50`）、step 5a（`:95-149`）、step 7.7（`:252`）、step 7.8（`:254`）、step 8 の単一フェーズの PR（`:259`）、「Before the PR」の B1（`:270-287`）。
    -   `runpr.md`：step 1 の単独の変更（`:129-131`）、step 3 の Escalation（`:665-678`）、step 4 の要約（`:976-1008`）、戻しのローカルの CLI レビュー（`:1131`）。
    -   `_lib/simple-process.md`：「Reviewing the three documents」の後の文（`:229-230`）、finish の commit（`:449-456`）。
    -   `_lib/pr-review-escalation.md`：「Criteria」（`:32-56`）、「Procedure」（`:58-81`）。
    -   `docs/dev/guides/fixprguard_guide.md:422`（Phase 4.5 の commit の一覧）。
    -   `CLAUDE.md`：「Claude Commands」のタスクの PR の流れの段落と Codex の PR レビューの段落、「Go Idioms」の `opus-review-decision.sh` の例外の項目（`:800` 付近から。最後の文が「Boundary: no other `.claude/commands/` script takes this exception.」）。
-   **ローカルの CLI レビューの呼び出し元は 6 つで、設計書 3.3.5 の表と一致する**（`local-cli-review.md:12-23`。ほかに、`runpr.md:130`・`:1131`、`task-pr-handoff.md` Part B step 1、`fixpr.md` Phase 4.5、`runplan.md:252`・B1）。RECORD を渡す箇所は、この表の行ごとに 1 つずつである（ステップ 1-4）。
-   **記録の行を読む手順。** `git log origin/main..HEAD` から記録の行を読むのは、`runplan.md` step 5a（B1 の done record と `not run` の行）、`task-pr-handoff.md` Part B step 3、`_lib/opus-review.md` の入口 1（`opus-review-decision.sh` が `Implementation model:` と `Opus review: done` の行だけを読む。`opus-review.md:44-46`）である。`internal/chief` は commit メッセージの記録の行を読まない（設計書 3.11）。
-   **スクリプトの前例。** `_lib/adr-correction-candidates.sh` は `#!/usr/bin/env bash` で、bash 3.2 でも動き、`/fixpr` が `origin/main` のコピーを取り出して走らせる（`fixpr.md` Phase 3）。そのテストは `cmd/yt2column/adr_candidates_docs_test.go` にあり、一時的なリポジトリに `refs/remotes/origin/main` を作り、`scriptBashes` の各 bash で走らせ、git の設定に左右されないことも確かめる（`:522`）。呼び方の guard は `TestADRCorrectionCandidatesCallSite`（`:577`）で、コマンドの文書の、スクリプトを名指すコードブロックの行とコードスパンを、決まった 3 行の形と名前だけに限る。`_lib/opus-review-decision.sh` は bash 4 を要し、`opus-review.md:167-188` が同じ 3 行の形で走らせる。開発環境の git は 2.50.1、`/bin/bash` は 3.2.57 である。
-   **既存の guard（直すのは `TestLocalReviewGuard` だけ）。** 設計書 3.11 の表のとおりである。足す文と新しいファイルは、ほかに次の guard を通る必要がある。
    -   `TestLocalReviewGuard`（`cmd/yt2column/fixpr_docs_test.go:1155`）: 確かめる step の一覧 `{1, 3, 5, 6}` に 7 を足す（ステップ 1-9）。
    -   `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`）: `.claude/commands/` の Markdown の相対リンクと、コードスパンの `docs/dev/` のパスが存在する。`_lib/fable-review.md` へリンクする文は、そのファイルを作る PR-2 で足す。
    -   `TestCodexMentionOnlyInRequestRow`・`TestPostingPathsCheckMention`・`TestPostingCommandsInventory`（`cmd/yt2column/codex_review_docs_test.go:92`・`:119`・`:200`）: 新しい文書にメンションも GitHub に書き込むコマンドも書かない。メンションの確かめは `<review-mention>` の形で名指す。
    -   `TestRunprDocContract`・`TestHandoffListingPrintsNoBody`（`fixpr_docs_test.go:157`・`:195`）: `runpr.md` step 3 に足す `headRefOid` の読み取りは、本文を出さない `--jq` の形にする。`runpr.md` の step の見出しと、`task-pr-handoff.md` の `3. Push and create the PR for the current branch.` と `4. Run AFTER_CREATE` の書き出しを変えない。
    -   `TestNoDirectCodexInvocation`（`cmd/codexreview/docs_test.go:375`）: 新しい文書は `codex` を直接走らせない。
    -   `TestADRRecords`（`cmd/yt2column/adr_test.go:434`）: PR-3 で ADR を直すときの形。
-   **`CLAUDE.md` の、テストを残す信頼の境界のコマンドの列挙**（`CLAUDE.md:653-657`）には、`adr-correction-candidates.sh` の呼び方の guard も挙がっていない。本計画は、その前例に揃え、列挙を変えない。
-   **Agent ツール。** `model` に `fable` を指定でき、`subagent_type` の `fork` では `model` が無視される（設計書の決定 5 の確かめた事実。`opus-review.md:394-395`）。
-   製品のコード（`internal/` の製品のパッケージ・`extension/`）と、Go で書いた開発の道具（`fixprguard`・`codexreview`・`mergepr`・`coveragecompare`・`chief`）は変えない。`fixpr.md`・`mkreq.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`_lib/opus-review.md`・`_lib/opus-review-decision.sh`・`_lib/triage-pattern.md`・ADR の索引は変えない（設計書 2.2）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 見たかの判定と Escalation（設計書 8 章の 1）

**変更するファイル:** `.claude/commands/_lib/review-records.sh`（新規）、`cmd/yt2column/review_records_test.go`（新規）、`cmd/yt2column/testdata/review_records/`（新規）、`.claude/commands/_lib/local-cli-review.md`、`.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/runplan.md`、`.claude/commands/runpr.md`、`.claude/commands/_lib/simple-process.md`、`.claude/commands/_lib/pr-review-escalation.md`、`cmd/yt2column/fixpr_docs_test.go`、`docs/dev/guides/fixprguard_guide.md`、`CLAUDE.md`、本計画

-   [ ] **ステップ 1-1**: `_lib/review-records.sh` を作り、`coverage` のモードを書く（設計書 3.6.1、3.6.3）。最初の引数でモードを選び、`coverage` 以外のモードは失敗で終わる（`fable` のモードはステップ 2-2 で足す）。見た範囲の行の確かめ（START と END の解決、祖先の関係、範囲がブランチの中にあること）、計画書の PR の進み具合のチェックボックスの行だけを変える commit の見分け（PR マーカーの節のものと単一 PR の記録のもの）、merge commit、入力の形の確かめ、失敗のときの `PARTIAL` を、設計書 3.6.1 のとおりに書く。
-   [ ] **ステップ 1-2**: スクリプトのテスト `TestReviewRecordsCoverage` を書く（設計書 7.1 の `coverage` の場合と、失敗の場合）。一時的なリポジトリに `refs/remotes/origin/main` を作り、`scriptBashes` の各 bash で走らせる。各場合が、スクリプトの該当する規則を壊すと失敗することを確かめ、commit のメッセージに書く。
-   [ ] **ステップ 1-3**: `_lib/local-cli-review.md` を変える（設計書 3.3.1 の RECORD、3.3.2 の 3 つ目の項目、3.3.4 の 2 つ目と 3 つ目の項目、3.3.5）。
    -   入力 RECORD と、その既定の `commit`。
    -   step 4・5 で、指摘を `invalid` か `no-harm` で却下するときに、確かめた方法と結果を理由として記録すること（AC-21）。トリアージの規則の節にまとめるのは、ステップ 2-3 で行う。
    -   step 7 の表に見た範囲の行（`Local CLI reviewed:`）を足し、「Reading the record lines」にその読み方を足す。
    -   RECORD が `commit` のときの記録の commit（メンションの確かめを通す）と、GUARD のときの扱い（step 7 で GUARD を走らせ、入力の節の GUARD の地点の一覧に step 7 を足し、Terms の HEAD3 を記録の commit の後の HEAD とする）。
-   [ ] **ステップ 1-4**: 呼び出し元に RECORD を渡す（設計書 3.3.5 の表）。
    -   `task-pr-handoff.md` Part B step 1 は `commit` を渡し、README の同期の commit が記録の行を載せる記述を消す。
    -   `runplan.md` step 7.7 は `return`、B1 は `commit` を渡す。step 7.8 は、step 7.7 の後に必ず commit を作る形に直す（設計書 3.5.2 の 3 つ目の項目）。
    -   `_lib/simple-process.md` の戻しの step 3 は `return` を渡す。`runpr.md` step 1 と `fixpr.md` Phase 4.5 は何も渡さない（`fixpr.md` は変えない）。
-   [ ] **ステップ 1-5**: `_lib/pr-review-escalation.md` の「Criteria」を書き換える（設計書 3.7、決定 12・13）。基準の番号は変えずに基準 4 を広げ、ほかの基準は呼び出し元が記録で決めると書き、「Do not request one for」と「Size is not a criterion」の段落を合わせる。「Procedure」、返す JSON の形、「迷うときは `ask`」の指示は変えない（AC-37）。
-   [ ] **ステップ 1-6**: `runpr.md` step 3 の Escalation を変える（設計書 3.8.1）。`git fetch origin main`、`headRefOid` と手元の HEAD の比較（本文を出さない `--jq` の形）、判定のスクリプトの `coverage` のモードを `origin/main` のコピーで走らせること、対応の規則（設計書 3.7 の 3 つ目の項目）を書く。
-   [ ] **ステップ 1-7**: `runpr.md` step 4 のマージの質問の要約に、Codex が見ていない変更があるときの行を足し、要約の行の数の上限を 13 にする（設計書 3.8.2）。この PR では、Fable の部分は `なし` とだけ示す（§1.3 の決定 5）。選択肢の表、問いの文、詳細ファイルの扱いは変えない（AC-40）。
-   [ ] **ステップ 1-8**: guard のテスト `TestReviewRecordsCallSite` を `cmd/yt2column/review_records_test.go` に書く（§1.3 の決定 4）。コマンドの文書の、判定のスクリプトを名指すコードブロックの行とコードスパンを、`origin/main` の commit の読み取り・取り出し・実行の決まった形と、名前だけに限る。決まった形から外した行を足すと失敗することを確かめ、commit のメッセージに書く。
-   [ ] **ステップ 1-9**: `TestLocalReviewGuard` の確かめる step に 7 を足す（設計書 3.11）。step 7 から GUARD の文を消すと失敗することを確かめ、commit のメッセージに書く。
-   [ ] **ステップ 1-10**: `docs/dev/guides/fixprguard_guide.md` の Phase 4.5 の commit の一覧に記録の commit を足し（設計書 3.9）、`CLAUDE.md` を変える。「Claude Commands」の Codex の PR レビューの段落では、`/runpr` が PR レビューを追加で頼む条件を、信頼の境界・セキュリティ・耐久性に触れる変更（レビューの関門の変更を含む）に絞った形にする（AC-41 のうち Escalation の部分。設計書 3.9）。「Go Idioms」の `opus-review-decision.sh` の例外を、判定のスクリプトの、計画書の文と一致しなければならないリテラルにも広げ、最後の Boundary の文を合わせる（設計書の決定 22）。
-   [ ] **ステップ 1-11**: AC-37 と AC-40 の差分の記録を書く（§7 の「差分の記録」の PR-1 の欄）。

**完了の条件:** 設計書 3.3.1（RECORD）、3.3.2（却下の根拠）、3.3.4（見た範囲の行）、3.3.5、3.5.2 の 3 つ目、3.6.1、3.6.3、3.7、3.8.1、3.8.2（Fable の部分を除く）、3.9 の `fixprguard_guide.md` と `CLAUDE.md` の Escalation の部分、決定 22 の項目がすべて文書とスクリプトにある。`bash -n` が判定のスクリプトに通る（`shellcheck` があれば、それも通る）。§7 に AC-37 と AC-40 の PR-1 の記録がある。`make test` と `make lint` が通る。

### PR-1 作成ポイント: review coverage check and narrower escalation

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11

**推奨タイトル**: `feat(0098): ローカルの CLI レビューが見た範囲を記録し、Codex の PR 再レビューを自動で頼む条件を絞る`

**レビュー観点**: 判定のスクリプトが見たと誤って判定しうる筋書き（祖先の関係・範囲の外の行・計画書のほかの行） / Escalation の対応の規則が `skip` に倒れる条件 / 記録の commit と GUARD の地点が `fixpr.md` Phase 4.5 を壊さないか / スクリプトを `origin/main` のコピーでだけ走らせること

**実装モデル要件**: opus

**判定理由**: ステップ 1-1・1-6 は、Codex の PR 再レビューを省いてよいかを決めるレビューの関門（`mkplan.md` step 8 のパネルの条件の security-gate）で、Sonnet が祖先の関係や見た範囲の確かめを取り違えると、見ていない commit を `ALL` と判定し、PR 再レビューを頼まないままマージさせうる。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: Fable の部分レビュー（設計書 8 章の 2）

**変更するファイル:** `.claude/commands/_lib/fable-review.md`（新規）、`.claude/commands/_lib/review-records.sh`、`cmd/yt2column/review_records_test.go`、`cmd/yt2column/testdata/review_records/`、`.claude/commands/_lib/local-cli-review.md`、`.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/runplan.md`、`.claude/commands/runpr.md`、`.claude/commands/_lib/simple-process.md`、`CLAUDE.md`、本計画

-   [ ] **ステップ 2-1**: `_lib/fable-review.md` を作る（設計書 3.1 の全体と 3.2）。入口と入力・出力、有効な記録の判定の呼び方、範囲の基準と範囲の一覧（基準ごとの当たる例と当たらない例）、Fable への指示、起動・待ち方・上限、モデルと出力の形の確かめ、状態の確かめ、単独の経路、「Record lines」と「Reading the record lines」、確かめていない振る舞い（設計書 3.1.9）の扱いを書く。具体的なモデル ID は書かない（設計書 3.1.4・3.1.6）。
-   [ ] **ステップ 2-2**: 判定のスクリプトに `fable` のモードを足し（設計書 3.1.2、3.6.2）、スクリプトのテスト `TestReviewRecordsFable` を書く（設計書 7.1 の `fable` の場合と、失敗の場合）。各場合が、スクリプトの該当する規則を壊すと失敗することを確かめ、commit のメッセージに書く。
-   [ ] **ステップ 2-3**: `_lib/local-cli-review.md` を変える（設計書 3.3.1 の FABLE、3.3.2、3.3.3、3.3.4 の 1 つ目の項目、3.3.5 の Fable の行）。入力 FABLE、step 1 の起動の入口、最初の試行のトリアージの前の回収の入口（最初の試行が `none` のときも通る）、新しい節「Triage rules」（今の step 4・5 の規則とステップ 1-3 の却下の根拠に、Fable の指摘をまとめる規則を足す。step 4・5 と単独の経路が名指す）、step 6 の 2 回目の試行の条件、Terms の未レビューの範囲、記録の commit に Fable の行を載せることと、その前の状態の確かめを書く。冒頭の呼び出し元の一覧に、Fable の単独の経路を足す。
-   [ ] **ステップ 2-4**: `_lib/task-pr-handoff.md` を変える（設計書 3.4）。Part B step 1 で FABLE（KIND `documents`・TASK_DIR・セルフレビューで却下した指摘）を渡し、Part B step 3 の PR の説明の項目に、Fable の部分レビューの節（設計書 3.2.4）と、`unclear` と `deferred` の Fable の指摘をレビュアーの確かめる点に挙げることを足す。
-   [ ] **ステップ 2-5**: `runplan.md` を変える（設計書 3.5）。step 4 で PR を開く前の最後の phase group を見分けて step 8 の要約に示すこと、最後の phase group の step 7.7 で FABLE（KIND `implementation`）を渡すこと、step 7.8 の空の commit に Fable の行を載せること、step 5a と step 8 の最初のチェックポイントの入口（判定・単独の経路・チェックポイントの印・有効な記録がない間は進まないこと）、OpenCode で動くときの `not run (agent-cannot-call)` を書く。
-   [ ] **ステップ 2-6**: `runpr.md` step 4 の要約の行の Fable の部分を、Fable の手順の「Reading the record lines」で読む形にする（設計書 3.8.2 の 2 つ目の項目）。
-   [ ] **ステップ 2-7**: `_lib/simple-process.md:229-230` の文に、Fable の部分レビューも Part B step 1 で走ることを足す（設計書 3.9）。`CLAUDE.md`「Claude Commands」のタスクの PR を作るコマンドの段落に、PR を開く前に Fable の部分レビューをローカルの CLI レビューと並列に行うこと（`/runplan` では PR ごとに 1 回）を足す（AC-41 のうち Fable の部分）。
-   [ ] **ステップ 2-8**: guard のテスト `TestRecordLinePrefixesDistinct` を書く（§1.3 の決定 6）。`_lib/local-cli-review.md` step 7 の表、`_lib/fable-review.md` の「Record lines」、`_lib/opus-review.md` の「Record lines」から記録の行の前置きを読み、どの 2 つも一致せず、一方が他方の先頭にもならないことを確かめる。前置きを既存のものと重ねると失敗することを確かめ、commit のメッセージに書く。
-   [ ] **ステップ 2-9**: AC-40 の差分の記録を書く（§7 の「差分の記録」の PR-2 の欄）。

**完了の条件:** 設計書 3.1、3.2、3.3（ステップ 1-3 で済んだものを除く）、3.4、3.5、3.6.2、3.8.2 の Fable の部分、3.9 の `simple-process.md` と `CLAUDE.md` の Fable の部分の項目がすべて文書とスクリプトにある。`bash -n` が判定のスクリプトに通る（`shellcheck` があれば、それも通る）。§7 に AC-40 の PR-2 の記録がある。`make test` と `make lint` が通る。

### PR-2 作成ポイント: Fable partial review

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9

**推奨タイトル**: `feat(0098): タスクの PR を開く前に Fable の部分レビューを並列に行う`

**レビュー観点**: Fable の記録が有効かの判定が、PR の実装の一部を見ないまま有効と判定しうる筋書き / 状態の確かめの出口と、出口の後の比べの漏れ / Fable の出力の文が記録の行や PR の説明に写らないこと / まとめたトリアージと 2 回目の試行の条件

**実装モデル要件**: opus

**判定理由**: ステップ 2-1 のバックグラウンドのサブエージェントとタイマーの待ち合わせと状態の確かめは前例のない設計の判断を含み、ステップ 2-2 は PR を開いてよいかの関門（security-gate）である。Sonnet では、出口での状態の比べの漏れや、記録の後の commit を無視する誤りで、Fable を飛ばして PR を開きうる。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: ADR の確かめと実地の確かめ（設計書 8 章の 3）

**変更するファイル:** 本計画、必要なときは `docs/dev/adr/0050-fable-partial-review.md`・`docs/dev/adr/0013-review-setup.md`・`docs/dev/adr/0014-deterministic-checks-in-go.md`

PR-2 のマージの後、PR-3 の `/runplan` の前に、人がほかのタスクのタスク文書の PR を、本物のセッション（Claude Code）で開く。PR-3 の `/runplan` は、その PR の記録を読んで §7 に書く。

-   [ ] **ステップ 3-1**: タスク文書の PR の経路の実地の確かめを記録する（設計書 7.4）。上の PR のブランチの記録の行と PR の説明から、Fable がローカルの CLI レビューと並列に起動したこと、見た範囲の行と Fable の行を持つ記録の commit ができたこと、PR の説明に Fable の部分レビューの節ができたことを確かめ、§7 の「PR-3 の実行の記録」に書く。Fable の出力の文とモデル ID の文字列は書かず、確かめの結果だけを書く。
-   [ ] **ステップ 3-2**: 実装の PR の経路の実地の確かめを記録する。PR-3 自身の `/runplan` で、最後の phase group の step 7.7 が Fable を並列に起動したこと、チェックポイントの入口が `VALID` と判定してチェックポイントの印を残したことを確かめ、§7 に書く。PR-3 は Claude Code の Sonnet で走る（実装モデル要件 `sonnet+opus-review`）ので、設計書 3.1.9 の Sonnet のセッションでの呼び出しと通知も、ここで確かめる。
-   [ ] **ステップ 3-3**: セキュリティの実地の確かめを、使い捨てのブランチで 1 回ずつ行う（設計書 7.4）。記録の行の要約にメンションを含めたとき、メンションの確かめで自由な文が除かれること。Fable の起動の後に hooks のディレクトリにファイルを置いたとき、状態の確かめで止まること。結果を §7 に書き、使い捨てのブランチと置いたファイルを消す。
-   [ ] **ステップ 3-4**: 設計書 3.1.9 の確かめていない振る舞いのうち、ステップ 3-1〜3-3 で観察できたものを §7 に書く。設計書と違う振る舞いが見つかったときは、`runplan.md` の決定の変更として人と決める。
-   [ ] **ステップ 3-5**: ADR-0050・ADR-0013・ADR-0014 の「決定」を、実装した手順と照らす（AC-42、設計書 3.10）。食い違いがあれば、編集上の訂正か決定の変更として直す（`docs/dev/conventions/adr.md`）。照らした結果を §7 に書く。

**完了の条件:** §7 にステップ 3-1〜3-5 の記録がある。`make test` と `make lint` が通る。

### PR-3 作成ポイント: ADR check and live verification

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `docs(0098): Fable の部分レビューを実地で確かめ、ADR の決定と照らす`

**レビュー観点**: 実地の確かめの記録が、Fable の出力の文やモデル ID を含まないか / 3.1.9 の確かめていない振る舞いの観察と、設計との食い違いの扱い / ADR の「決定」と実装した手順の一致

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 3-3 は状態の確かめとメンションの確かめを実地で試すセキュリティの確かめで、Sonnet が結果を読み違えたり、Fable の出力の文を記録に写したりする誤りが起きうる。また、`sonnet` では OpenCode で走り、Fable を呼べないので、ステップ 3-2 の確かめができない。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 前提 |
|---|---|---|
| M1（PR-1） | 判定のスクリプトの `coverage` のモードとそのテスト、見た範囲の行と記録の commit、Escalation の新しい条件、マージの質問の要約の行 | 本計画の承認 |
| M2（PR-2） | Fable の手順、判定のスクリプトの `fable` のモード、まとめたトリアージ、タスク文書の PR と実装の PR への組み込み | PR-1 のマージ（Fable の行の読み方と記録の commit が、PR-1 の見た範囲の行と記録の commit に載る） |
| M3（PR-3） | 実地の確かめの記録、ADR の確かめ | PR-2 のマージと、人が開くタスク文書の PR |

判定のスクリプトは `origin/main` のコピーで走るので、PR-1 がマージされるまでは `coverage` が `PARTIAL` に、PR-2 がマージされるまでは `fable` が `MISSING` になる（設計書 3.6.3）。PR-2 の `/runpr` は、PR-1 のスクリプトで Escalation を判定する。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 | 判定のスクリプトの `coverage` のモードとテスト、RECORD と見た範囲の行と記録の commit、Escalation の条件、マージの質問の要約の行、`CLAUDE.md` とガイド | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 | Fable の手順、`fable` のモードとテスト、まとめたトリアージ、`task-pr-handoff.md`・`runplan.md`・`runpr.md`・`simple-process.md`・`CLAUDE.md` への組み込み | opus | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | 実地の確かめの記録と ADR の確かめ | sonnet+opus-review | hold |

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 判定のスクリプトの規則は、スクリプトのテスト（`TestReviewRecordsCoverage`・`TestReviewRecordsFable`）が、一時的なリポジトリの上で、設計書 7.1 の場合ごとに確かめる。bash は PATH のものと `/bin/bash` 3.2 の両方で走らせる。git の設定に左右されないことは、前例（`adr_candidates_docs_test.go` の `checkACIgnoresGitConfig`）に揃えて確かめる。
-   **guard のテスト:** スクリプトの呼び方（`TestReviewRecordsCallSite`）、記録の行の前置き（`TestRecordLinePrefixesDistinct`）、GUARD の地点（`TestLocalReviewGuard`）。既存の guard（§1.4）が、変わる文書でも通ること。
-   **差分の記録:** AC-37 と AC-40 の前後の関係は、差分を記録して PR のレビューで読む（§7）。
-   **読んで確かめる:** 手順の文（設計書 7.3）は PR のレビューで読む。
-   **結合・実地の確かめ:** 並列の実行、時間切れのタイマー、モデル ID の確かめ、状態の確かめは、手順の文だけでは確かめられないので、PR-3 の実地の確かめで行う（設計書 7.4）。
-   **後方互換:** 本タスクの前に作られたブランチの記録は書き換えない。既存の記録の行の読み方が変わらないことは、`TestRecordLinePrefixesDistinct` と、既存の読み手の文書を変えないこと（§1.4）で確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 判定のスクリプトの誤りで、見ていない commit を `ALL` とする、または Fable の記録を誤って `VALID` とする | スクリプトのテストで規則ごとに壊して確かめる。失敗は安全な側に倒す（設計書 3.6）。PR-1・PR-2 は `opus` で実装する |
| PR-1 と PR-2 の間、`/runpr` の要約の Fable の部分がいつも `なし` になる | 意図した途中の状態である（§1.3 の決定 5）。PR-1 の時点でどのブランチにも Fable の記録はない |
| 記録の commit を push するので、`/fixpr` の回ごとに CI が 1 回増える | 設計書の決定 21 で受け入れた |
| Fable が返らず、PR を開くまで 30 分待つ | 設計書の決定 4 の上限で `not run (timeout)` とする |
| 設計書 3.1.9 の確かめていない振る舞いが、設計と違う | PR-3 の実地の確かめで観察し、違えば決定の変更として人と決める（ステップ 3-4） |
| 実地の確かめに要るタスク文書の PR が、すぐには開かれない | PR-3 を `hold` にし、人がその PR を開いてから `/runplan` を送る |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

確かめの種類は次のとおりである。

-   `test`: Go のテストが、判定のスクリプトの振る舞いを確かめる。
-   `static`: guard のテストが、コマンドの文書の値と構造を確かめる。
-   `diff`: 差分の記録を PR のレビューで読む（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」）。
-   `manual`: PR のレビューで、その文書の該当箇所を読んで確かめる。コマンドの定義の手順の文だけを定める AC は、`manual` だけで確かめる（§1.2）。実地の確かめの結果も、PR-3 のレビューで読む。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `task-pr-handoff.md` Part B step 1 と `mk*.md` が Part B を呼ぶ順序を読む。実地の確かめ（ステップ 3-1） | manual | ステップ 2-4 | PR-2、PR-3 |
| AC-02 | `local-cli-review.md` の起動と回収の入口、`fable-review.md` の待ち方を読む。実地の確かめ（ステップ 3-1） | manual | ステップ 2-1・2-3・2-4 | PR-2、PR-3 |
| AC-03 | `simple-process.md` と `task-pr-handoff.md` Part B step 1、`fable-review.md` の基準 2 を読む | manual | ステップ 2-1・2-4・2-7 | PR-2 |
| AC-04 | `cmd/yt2column/review_records_test.go::TestReviewRecordsFable`（記録がないときの `MISSING <merge-base>`）、`runplan.md` step 4・7.7 を読む | test、manual | ステップ 2-2・2-5 | PR-2 |
| AC-05 | `runplan.md` step 7.7 と `local-cli-review.md` の回収の入口を読む。実地の確かめ（ステップ 3-2） | manual | ステップ 2-3・2-5 | PR-2、PR-3 |
| AC-06 | `cmd/yt2column/review_records_test.go::TestReviewRecordsFable`（足りない commit があるときの `MISSING <START>`）、`fable-review.md` の単独の経路と `runplan.md` のチェックポイントの入口を読む | test、manual | ステップ 2-1・2-2・2-5 | PR-2 |
| AC-07 | `cmd/yt2column/review_records_test.go::TestReviewRecordsFable`（中身のない commit、同じ実行の直し、チェックポイントの印の前後、印の後の実装の記録、計画書のほかの行を変える commit） | test | ステップ 2-2 | PR-2 |
| AC-08 | `cmd/yt2column/review_records_test.go::TestReviewRecordsFable`（チェックポイントの印の後の commit が記録を無効にしないこと）、`runplan.md` のチェックポイントの入口を読む | test、manual | ステップ 2-2・2-5 | PR-2 |
| AC-09 | `cmd/yt2column/review_records_test.go::TestReviewRecordsFable`（有効な記録があるときの `VALID`）、`fable-review.md` の止まったときの扱いを読む | test、manual | ステップ 2-1・2-2 | PR-2 |
| AC-10 | `fable-review.md` の指示の制約と状態の確かめを読む。実地の確かめ（ステップ 3-3） | manual | ステップ 2-1 | PR-2、PR-3 |
| AC-11〜AC-15 | `fable-review.md` の範囲の基準と範囲の一覧を読む | manual | ステップ 2-1 | PR-2 |
| AC-16〜AC-19 | `fable-review.md` の Fable への指示を読む | manual | ステップ 2-1 | PR-2 |
| AC-20 | `local-cli-review.md` の「Triage rules」を読む | manual | ステップ 2-3 | PR-2 |
| AC-21 | `local-cli-review.md` step 4・5（PR-2 では「Triage rules」）と step 7 を読む | manual | ステップ 1-3・2-3 | PR-1、PR-2 |
| AC-22 | `local-cli-review.md` step 6 と Terms を読む | manual | ステップ 2-3 | PR-2 |
| AC-23 | `task-pr-handoff.md` Part B step 3 を読む | manual | ステップ 2-4 | PR-2 |
| AC-24〜AC-27 | `fable-review.md` の起動・待ち方・確かめを読む。実地の確かめ（ステップ 3-1・3-2） | manual | ステップ 2-1 | PR-2、PR-3 |
| AC-28〜AC-30 | `fable-review.md` の「Record lines」と記録の commit のメンションの確かめを読む。実地の確かめ（ステップ 3-3） | manual | ステップ 2-1・2-3 | PR-2、PR-3 |
| AC-31 | `task-pr-handoff.md` Part B step 3 を読む。実地の確かめ（ステップ 3-1） | manual | ステップ 2-4 | PR-2、PR-3 |
| AC-32 | `cmd/yt2column/review_records_test.go::TestRecordLinePrefixesDistinct` | static | ステップ 2-8 | PR-2 |
| AC-33 | `runpr.md` step 4 の要約の行の読み方を読む | manual | ステップ 1-7・2-6 | PR-1、PR-2 |
| AC-34 | `pr-review-escalation.md` の「Criteria」と `runpr.md` step 3 の対応の規則を読む | manual | ステップ 1-5・1-6 | PR-1 |
| AC-35 | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`、`cmd/yt2column/review_records_test.go::TestReviewRecordsCallSite`、`runpr.md` step 3 の対応の規則を読む | test、static、manual | ステップ 1-1・1-2・1-6・1-8 | PR-1 |
| AC-36 | `runpr.md` step 3 の対応の規則を読む | manual | ステップ 1-6 | PR-1 |
| AC-37 | 差分の記録（AC-37）。比べるパス: `.claude/commands/_lib/pr-review-escalation.md` | diff | ステップ 1-5 | PR-1 |
| AC-38 | `runpr.md` step 3 を読む | manual | ステップ 1-6 | PR-1 |
| AC-39 | `cmd/yt2column/review_records_test.go::TestReviewRecordsCoverage`（要約の前半の値）、`runpr.md` step 4 の要約の行を読む | test、manual | ステップ 1-7・2-6 | PR-1、PR-2 |
| AC-40 | 差分の記録（AC-40）。比べるパス: `.claude/commands/runpr.md` | diff | ステップ 1-6・1-7・2-6 | PR-1、PR-2 |
| AC-41 | `CLAUDE.md`「Claude Commands」を読む | manual | ステップ 1-10・2-7 | PR-1、PR-2 |
| AC-42 | ADR の「決定」と実装した手順を照らした記録（ステップ 3-5）を読む。ADR を直したときは `cmd/yt2column/adr_test.go::TestADRRecords` が形を確かめる | manual | ステップ 3-5 | PR-3 |
| AC-43 | `fable-review.md` の「Record lines」の `Fable finding:` の行を読む | manual | ステップ 2-1 | PR-2 |
| AC-44 | `task-pr-handoff.md` Part B step 3 の PR の説明の形を読む。実地の確かめ（ステップ 3-1） | manual | ステップ 2-4 | PR-2、PR-3 |

`runpr.md` の差分の記録（AC-40）には、マージの質問の外の変更（step 3 の Escalation、戻しの RECORD）も現れる。レビュアーは、マージの質問の節の差分が、要約の行の追加とその読み方と行の数の上限に限られることを読む。

### 差分の記録

Diff record (AC-37):

- Compared paths: `.claude/commands/_lib/pr-review-escalation.md`

PR-1:

Diff record (AC-40):

- Compared paths: `.claude/commands/runpr.md`

PR-2:

Diff record (AC-40):

- Compared paths: `.claude/commands/runpr.md`

### PR-3 の実行の記録

（ステップ 3-1〜3-5 の結果を書く）

## 8. 成功の基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-44 が、§7 の確かめで満たされる。
-   **品質:** スクリプトのテストが PATH の bash と `/bin/bash` 3.2 の両方で通り、`make test` と `make lint` が通る。
-   **セキュリティ:** Fable の出力の文が commit メッセージと PR の説明に写らないこと、状態の確かめとメンションの確かめが働くことを、PR のレビューと実地の確かめ（ステップ 3-3）で確かめる。
-   **文書:** `CLAUDE.md`、`fixprguard_guide.md`、ADR-0050・ADR-0013・ADR-0014 が、実装した手順と一致する。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、効果の分類と集計（[#936](https://github.com/isseis/yt2column/issues/936)）が、PR の説明の `Fable の行` のコードブロックを読む。
-   実地の確かめで設計と違う振る舞いが見つかり、本タスクの中で扱わなかったものは、issue にする。
