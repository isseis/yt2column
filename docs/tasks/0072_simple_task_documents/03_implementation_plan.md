# 実装計画書：開発文書を変更の規模に合わせて簡易にとどめる分岐

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-09 |
| Review date | 2026-11-09 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指し、本計画自身の節は「本計画 §N」と書く。用語は、要件書 6 章の用語と、設計書の冒頭の用語（進め方の記録・`simple-process.md`・要件プロセスの規約・戻す・除外のスレッド・実装の PR・確かめ済みの項目・扱い済みの項目）を、同じ意味で使う。

既存のファイルの行番号は、コミット `3709c2d6`（`main`）で確かめた。設計書が行番号を確かめたコミット `ae6e7b26` から `3709c2d6` までに、本タスクが変えるファイルのうち変わったのは `docs/dev/adr/` の下だけである（`git diff --stat ae6e7b26 3709c2d6 -- .claude docs/dev/conventions CLAUDE.md docs/translation_glossary.md docs/tasks/0000_template docs/dev/guides/fixprguard_guide.md cmd docs/dev/adr` を実行して確かめた。挙がったのは ADR-0012・0034・0035・0039・0040 と ADR の索引だけである）。そのため、設計書が示すコマンドの定義と規約の行番号は、そのまま使える。ADR-0040 は、設計書の PR（#755）で変わった後の内容を前提とする。

申し送りの H-01〜H-13 は、設計書の付録 A がすべて扱っている。本タスクに `implementation_handoff.md` はない。

本計画の PR で、設計書 3.5・3.9 に、戻すときの PR の説明の訂正の段落の扱いを足した（本計画 §1.3 の 2 つ目の項目）。決定の変更なので、設計書は `draft` に戻した。本計画とともに、`/runpr` で承認し直す。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

簡易な進め方（要件書 F-001〜F-005）を、要件プロセスの規約とコマンドの定義に反映する。規則は規約に、簡易な進め方に固有の手順は新しい `simple-process.md` に置き、`/mkreq`・`/mkarch`・`/mkplan`・`mkplan2.md`・`/runplan`・`/runpr`・`/fixpr` と共有の手順（`_lib/task-pr-handoff.md`・`_lib/local-cli-review.md`）から参照する（設計書 1.1・2.2）。最後に ADR-0040 の「決定」の冒頭にある、反映を待つ旨の文を消す（設計書 3.10）。

製品のコード（`internal/`・`cmd/` の本体・拡張）は変えない。Go のコードで変えるのは、`runpr.md` に足す `fixprguard edit-pr` の呼び出しを、既存の文書のテストの表に足すことだけである（設計書 7.1）。

### 1.2. 実装の原則

-   **規則は規約に、手順は `simple-process.md` に、それぞれ 1 か所だけ書く**（設計書 1.1）。コマンドの定義は、規約や `simple-process.md` の節を名指しして参照し、中身を写さない。
-   **通常の進め方の手順は、step の番号と見出しの語で参照する。参照される側（`mkarch.md`・`mkplan.md`・`mkplan2.md`）にも、参照されていることを書く**（設計書 1.1・3.2・3.4）。参照される側の既存の step の番号は変えない。
-   **変えないと設計書が決めた部分は変えない。** 規約の「Lightweight Process」の節の文と表（設計書 3.9）、`fixpr.md`・`runpr.md` の節の見出し（設計書 7.1）、`_lib/task-pr-handoff.md` の Part B（設計書 3.8）、`_lib/settle-decisions.md`・`_lib/review-subagent-pattern.md`・`docs/dev/project_overview.md`（設計書 2.2）である。テストが読むのはこのうち一部だけなので（本計画 §1.4）、残りは PR のレビューで差分を読んで確かめる。
-   **新しい文書のテストを足さない**（設計書 1.2 の H-07 の決定）。既存の表に値を足す変更（本計画 §1.4 の表の最後の 3 行）だけを行う。
-   **コマンドの定義の文は英語で書き、規約・ガイド・用語集・ADR は既存の言語のまま書く**（`CLAUDE.md`「Document language」）。`simple-process.md` は `.claude/commands/` の下なので英語で書く。

### 1.3. 計画の際に決めたこと（2026-11-09）

-   **実装は 3 本の PR に分ける（人の答え、2026-11-09）。** PR-1 は設計書 8 章の優先順位 1・2（規約などの規則と、`/mkreq` の入口）、PR-2 は優先順位 3（`/mkarch`・`/mkplan`・`mkplan2.md`・`/runplan`）、PR-3 は優先順位 4・5（`/runpr`・`/fixpr` と ADR-0040 の仕上げ）である。
    -   **理由:** `/runpr`・`/fixpr` の変更は、戻す手順・除外のスレッド・`fixprguard edit-pr` の呼び出しを含み、開発の道具の信頼の境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）に触れる。ほかの変更と分けて、1 本の PR で読めるようにする。PR を 5 本にすると、往復が増え、途中の状態も長く続く。
    -   **採らなかった案:** 1 本の PR にする案。途中の状態はなくなるが、15 を超えるファイルを 1 回の PR レビューで読むことになる。
    -   **途中の状態:** PR-1 のマージの後、PR-3 のマージまでは、`main` の `/mkreq` が簡易な進め方を尋ねるのに、`/runpr`・`/fixpr` が 3 文書の PR を扱えない。利用者は 1 人なので、PR-3 のマージまでは簡易な進め方を選ばないことで対処する（本計画 §5.1）。
    -   ADR：なし。PR の分け方は本タスクの中で閉じる（`CLAUDE.md`「Design decisions (ADR)」の基準で major ではない）。
-   **戻すときに PR の説明を書き直す場合は、編集上の訂正の段落を、`origin/main..HEAD` のコミットの訂正の行から 1 つの段落に作り直す（人の答え、2026-11-09）。** `/fixpr` が回ごとに足した段落は、この 1 つの段落に置き換わる。規約の「Recording in the PR description」にある、PR の説明の段落をそのまま残す規則（`requirements_process.md:407`）に、戻すときの例外を 1 文で足す。
    -   **理由:** 訂正の行はどれもブランチのコミットにあるので、訂正の中身は失われない。`/runpr` は PR の本文を読まずに済み、信頼の境界が広がらない。
    -   **採らなかった案:** `/fixpr` の Phase 6 のように、エージェントが今の本文を読み、訂正の段落を残して書き直す案。`/runpr` から PR の本文がエージェントに入る。
    -   承認した設計書は本文を Part B の step 3 の決まりで書き直すとだけ述べ、この扱いを定めていなかった。計画書のレビューで分かったので、設計書 3.5・3.9 と付録 C に書き、設計書を `draft` に戻した。
    -   ADR：なし。戻す手順の中の書き方で、ADR-0040 の決定（戻す入口、申し送りへの残し方）は変わらない。
-   **`Process` 行は状態表の値であり、記録の置き場所を 7 つ目に増やさない。** ステップ 1-1 で記録の置き場所の段落（`requirements_process.md:123-133`）に足すのは、次の 2 点である。1 つは、`Process` 行が `Status` と同じく状態表の値であること。もう 1 つは、進め方の理由と判定を覆したことを、今の「…で決めたこと」に置くことである。「Six places」の数は変えない。
    -   **理由:** [ADR-0036](../../dev/adr/0036-task-document-status-records.md) の決定は、記録の置き場所を `Comments` を含む 6 つとし、状態表のほかの行（`Status` など）を置き場所に数えていない。`Process` 行もそれと同じ扱いにすれば、ADR-0036 の決定は変わらない。
    -   ADR：なし。ADR-0036 は変えない。
-   **戻す手順が使うスレッドの情報は、この `/runpr` の実行の中で最後に走った `/fixpr` の Final report から取る。** `/runpr` はスレッドの本文を取得しない（`TestRunprDocContract` の `no_body_fetch`、`cmd/yt2column/fixpr_docs_test.go:178-184`）。step 4 の質問を出す時点で残る未解決のスレッドは、`deferred` のスレッドと除外のスレッドだけである（ほかのスレッドがあれば、step 3 がマージの質問の前に止まる。`.claude/commands/runpr.md:669-702`）。そのため、`/fixpr` の Final report が両方のスレッドについて `threadId`・`databaseId`・`fingerprint`・話題・URL・ファイルのパスを挙げれば、戻す手順はそれで足りる（ステップ 4-2・4-3）。戻すコミットのメッセージには、消した文書の上のスレッドごとに `threadId`・`databaseId`・`fingerprint`・URL と、移した申し送りの項目の番号を書く。途中で止まった戻す手順を別の `/runpr` の実行で再開するときは、`/fixpr` の Final report がないことがあるので、手順 6 はこのメッセージから返信するスレッドを読む。スレッドの本文はメッセージに書かない。
    -   ADR：なし。設計書 3.5 の手順の入力を定めるだけである。
-   **フェーズは設計書 8 章の優先順位 1〜5 にそのまま対応させる。** フェーズ 1〜2 が PR-1、フェーズ 3 が PR-2、フェーズ 4〜5 が PR-3 である。
    -   ADR：なし。
-   **`simple-process.md` の節は、それを呼ぶコマンドを変えるフェーズで書く。** 設計書 8 章は `simple-process.md` を優先順位 2 に置くが、PR の分け方は計画書に任せている（設計書 8 章の最後の文）。本計画は PR を 3 本に分けたので、`simple-process.md` も、呼ぶ側と同じ PR に入るように分ける。フェーズ 2 では、`/mkreq` と、`/mkarch`・`/mkplan`・`/runplan` が使う節（Reading the process record、Recommending a process、Writing the three documents、Reviewing the three documents、Resuming a stopped run、When an excluded decision appears の `/mkreq` の部分）を書く。フェーズ 3・4 では、残りの部分を足す。
    -   **理由:** PR-1 のレビュアーが、まだない `/runpr` の振る舞いを前提にした手順を読まずに済む。各 PR の中で、呼ぶ側と呼ばれる側を組にして読める。
    -   ADR：なし。
-   **`diff` の確かめ（`_checks/` のスクリプト）は作らない。** 前後の関係を述べる受け入れ基準がないためである。設計書が変えないとした部分は、本計画 §1.2 の 3 つ目の原則のとおり、PR のレビューで差分を読んで確かめる。
    -   ADR：なし。
-   **ADR-0040 の変更（フェーズ 5）は、ADR の編集上の訂正として扱う。** 「反映されるまでは、…が仕様を定める」の文を消すのは、`docs/dev/conventions/adr.md`「Editorial corrections」の 2 つ目の種類（食い違いを述べる文の削除）、仕様の場所を規約と `simple-process.md` に向けるのは 3 つ目の種類（古い参照の修正）に当たり、「決定」の意味を変えない。この文は、PR-1・PR-2 のマージの後も、反映が終わっていないので正しい。そのため、PR-3 で消す。`adr.md` は一部だけを直す PR に残りを述べる文への書き換えを求める（`adr.md:342-343`）が、この文は「反映されるまでは要件定義書と設計書が仕様を定める」とするだけで、どこまで反映したかには触れない。PR-1・PR-2 の後も内容は変わらないので、書き換えない。
    -   ADR：なし（ADR-0040 の決定は変えない）。

### 1.4. 既存コード調査結果

製品のコードの変更はない。変えるファイルと、変更に関わる既存の仕組みは次のとおりである（行番号は `3709c2d6`）。

| 対象 | 今あるもの | 本タスクですること |
|---|---|---|
| `docs/dev/conventions/requirements_process.md` | 「LLM Constraints」（`:7-21`）、「Review Flow」（`:70-105`）、「Document Status Format」（`:107-`）と記録の置き場所の段落（`:123-133`）、「Recording in the PR description」の訂正の段落の規則（`:403-408`）、「Lightweight Process」（`:418-614`）、「Design handoff」の段落（`:649`） | 設計書 3.9 のとおり足す（ステップ 1-1） |
| `docs/tasks/0000_template/requirements.md` | 状態表は `Status`・`Created`・`Review date`・`Reviewer`・`Comments` の 5 行（`:5-11`） | `Status` の次に `` `full` `` の `Process` 行を足す（ステップ 1-2） |
| `docs/translation_glossary.md` | 「簡易な進め方」「通常の進め方」「進め方の記録」の行がない | 3 行を足す（ステップ 1-3） |
| `.claude/commands/_context.md` | 申し送りの 2 行（`:46-47`）は書く者を `fixpr.md` だけとする。実装モデルの行（`:67`）は割り当てる者を `mkplan2.md`（`mkplan.md` の step 8.5）とする | 設計書 2.2 のとおり改め、「Task process record」の行を足す（ステップ 1-4） |
| `CLAUDE.md` | 「Requirements and Acceptance Criteria」の手順の 2（`:776`）と「the next one is not started until」（`:781`）、「Claude Commands」の `/mkreq`・`/mkarch`・`/mkplan` の行 | 設計書 3.9 のとおり足す（ステップ 1-5） |
| `.claude/commands/_lib/simple-process.md` | ない | 新規（ステップ 2-1・3-5・4-1） |
| `.claude/commands/mkreq.md` | step 1 の重複の検索（`:62-69`）は、issue を名指しする要件定義書が見つかれば止まる。step 5 の SCOPE（`:226-234`）。Authorization（`:41-50`） | 設計書 3.3 のとおり変える（ステップ 2-2） |
| `.claude/commands/_lib/task-pr-handoff.md` | Part A の step 3（`:100-115`）は、タスク文書のコマンドが呼んだとき、独自のコミットを持つ決まりどおりの名前のブランチで止まる | `/mkreq` が続ける場合を足す（ステップ 2-3） |
| `.claude/commands/mkarch.md`・`mkplan.md` | step 3 の承認の確かめ（`mkarch.md:26-29`、`mkplan.md:28-32`）、step 6.5・5.5 の決定の確かめ、申し送りの扱い（`mkarch.md:67`、`mkplan.md:81`） | 設計書 3.4 のとおり変える（ステップ 3-1・3-2） |
| `.claude/commands/mkplan2.md` | 冒頭の `:16` が、`mkplan.md` が steps 4–9 を使うことを書く | `/mkreq` が `simple-process.md` を通して steps 4–8 を使うことを足す（ステップ 3-3） |
| `.claude/commands/runplan.md` | step 2 の承認の確かめ（`:35`）、step 3 の読む文書（`:41`）、step 5 の自己点検の「Decision change」（`:64`） | 設計書 3.7 のとおり変える（ステップ 3-4） |
| `.claude/commands/runpr.md` | Authorization（`:68-97`）、step 2 の Kind・PRE_MERGE・NEXT（`:279-386`）、step 3 の未解決のスレッドの分類（`:669-702`）、step 4 のマージの選択肢（`:762-775`）と、残したスレッドを issue に移して `fixprguard reply` で返信する手順（`:799-831`）。step 4 の `**Mention check.**` は今 `:805` の 1 つだけである | 設計書 3.5 のとおり変える（ステップ 4-2） |
| `.claude/commands/fixpr.md` | 「People are always answered」（`:133-139`）、「Naming an untrusted thread」（`:150-158`）、R2（`:220-228`）、R5（`:246-255`）、Phase 1（`:268-`）、Phase 2（`:309-`）、Final report の `deferred` のスレッドの項目（`:903-906`。パスを挙げない） | 設計書 3.6 のとおり変える（ステップ 4-3） |
| `.claude/commands/_lib/local-cli-review.md` | 「Six callers run it:」と呼び出し元の一覧（`:9-20`） | `/runpr` の戻す場合を足し、数を改める（ステップ 4-4） |
| `docs/dev/guides/fixprguard_guide.md` | `edit-pr` の節（`:201-209`）。`edit-pr` を呼ぶのが `/fixpr` だけであることを前提にした記述がある（`:406` の、git の状態の比較の説明など） | `/runpr` が戻すときに `edit-pr` を使うことを足し、前提の記述を改める（ステップ 4-5） |
| `docs/dev/adr/0040-simple-task-documents.md` | 「決定」の冒頭（`:17`）に、反映を待つ旨の文と `issues/743` へのリンクがある | 消して、仕様の場所を向け直す（ステップ 5-1） |
| `cmd/yt2column/fixpr_docs_test.go::TestFixprguardCallSites`（`:709`） | `runpr.md` の step 4 の呼び出しは `fixprguard reply` だけ | `{"runpr.md", runpr, runprStep4, "fixprguard edit-pr "}` の行を足す（ステップ 4-6） |
| `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（`:212`） | `runpr.md` の `want` は `cmdHead`・`cmdInteractionLimits`・`cmdList`・`cmdReply` | `cmdEditPR` を足す（ステップ 4-6）。この検査はコードブロックの中の呼び出しだけを読む（`documentedInvocations`、`cmd/fixprguard/docs_test.go:183-210`） |
| `cmd/yt2column/codex_review_docs_test.go::TestPostingCommandsInventory`（`:213`） | `postingSections` の `runpr.md step 4` の行（`:66-69`）は `gh issue comment`・`gh issue create`・`fixprguard reply` を挙げる。`postingCommand`（`:145`）は `fixprguard edit-pr` に当たる。このテストは、投稿の呼び出しが、その節の最初の `**Mention check.**` より後にあり、かつその節が挙げたコマンドであることを確かめる。節が挙げたコマンドが実際にあることまでは確かめない | `"fixprguard edit-pr "` を足す（ステップ 4-6） |

次の既存のテストは、本タスクの変更を読み、文の書き方を縛る（設計書 7.1）。どれも `make test` で走る。

-   **`.claude/commands/` の下のすべての文書を読むテスト**（`internal/doctestutil/protocol.go:85` の `CommandDocs`）。`simple-process.md` と、変えるコマンドの定義のすべてが対象になる。
    -   `cmd/yt2column/codex_review_docs_test.go::TestCodexMentionOnlyInRequestRow`（`:101`）: メンションを書かない。
    -   `TestPostingCommandsInventory`（`:213`）: 投稿の呼び出しは、`postingSections` が挙げた節の中で、その節の最初の `**Mention check.**` より後に置く。
    -   `TestPostingPathsCheckMention`（`:128`）: 投稿の節の最初の `**Mention check.**` の最初の文は、`` `grep -qiF -- '<review-mention>' <file>` `` を含む。
    -   `cmd/fixprguard/docs_test.go::TestDocumentedProtocolMatches`（`:350`）と、`cmd/codexreview` の同じ名前のテスト・`TestDocumentedInvocationsParse`・`TestNoDirectCodexInvocation`: `--protocol` の値と、`codex` を直接呼ばないこと。
    -   `cmd/yt2column/lightweight_docs_test.go::TestLightweightCheckCallSite`（`:366`）。
-   `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（`:140`）: 指示の文書の相対リンクと、`docs/dev/` の下の `.md` を名指すコードスパン（`docpaths_test.go:24`・`:37-62`）の先が実在する。アンカーは確かめない（`:33-36`）。
-   `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（`:988`）: `fixpr.md`・`runpr.md`・`runissue.md`・`mkreq.md`・`_lib/task-pr-handoff.md` がレビュアーの名前を書かない。`simple-process.md` は表に足さない（設計書 7.1）が、レビュアーの名前は書かない。
-   `cmd/yt2column/fixpr_docs_test.go::TestRemovedCommandLiterals`（`:1105`）: `fixpr.md` のどこにも `git add -A`・`git commit -m`・`comments(first:`・`reviewThreads` を書かず、`gh pr edit` と `--title "` を同じ行に書かず、Phase 1〜6 に `git push` を書かない（Phase 4.5 は除く）。
-   `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（`:156`）: `runpr.md` のどこにも `reviewThreads`・`comments(first:` を書かない（`no_body_fetch`）。step 3 の `gh api` の呼び出しは、許した形だけにする（`listings_print_no_body`、`step3ReadCalls`）。
-   `cmd/yt2column/fixpr_docs_test.go` の節の見出しの定数（`:31-43`）と `cmd/yt2column/codex_review_docs_test.go` の `runprStep5`・`handoffLinkIssue`（`:88-91`）: これらの見出しは変えない。`runpr.md` の step 2 の見出しはテストが読まないので、PR のレビューで変えていないことを確かめる。
-   `cmd/yt2column/lightweight_docs_test.go` の各テスト: 規約の「Lightweight Process」のうち読むのは「Mechanically checked conditions」の表だけである（`lightweight_docs_test.go:65-120`）。節のほかの文は、PR のレビューで変えていないことを確かめる。
-   `CLAUDE.md` を読むテスト（`TestCoverageCompareDocumented`、`TestExtensionDocumentation` など）: ステップ 1-5 は、これらが読む値（`make` の目標の名前など）を変えない。
-   `cmd/fixprguard/docs_test.go` のガイドを読むテスト（`TestGuideDescribesReviewerList`・`TestDocumentedProtocolMatches`・`TestGuideDescribesMentionCheck` など）: ステップ 4-5 の変更を読む。
-   `cmd/yt2column/adr_test.go::TestADRRecords`（`:434`）: ADR-0040 の構造と索引を読む。

## 2. 実装のステップ (Implementation Steps)

各ステップの「何を書くか」は、設計書の該当する節が定める。ここには、変えるファイル、設計書の節、守ることだけを書く。

### フェーズ 1: 規約・テンプレート・用語集・`_context.md`・`CLAUDE.md`（設計書 8 章の 1）

**変更するファイル:** `docs/dev/conventions/requirements_process.md`、`docs/tasks/0000_template/requirements.md`、`docs/translation_glossary.md`、`.claude/commands/_context.md`、`CLAUDE.md`

-   [ ] **ステップ 1-1**: 要件プロセスの規約を、設計書 3.9 の「要件プロセスの規約」の項目のとおりに変える。
    -   「LLM Constraints」に簡易な進め方の例外を足す（AC-13）。
    -   「Review Flow」に簡易な進め方の流れを、通常の流れと並べて足す。
    -   「Document Status Format」に `Process` 行の値と 4 つの記録の状態（設計書 3.1 の表）を書く。記録の置き場所の段落には、本計画 §1.3 の 3 つ目の項目の 2 点だけを足し、「Six places」の数は変えない。
    -   「Recording in the PR description」の、訂正の段落をそのまま残す規則に、戻すときの例外を 1 文で足す（設計書 3.9、本計画 §1.3 の 2 つ目の項目）。
    -   「Design handoff」の段落に、申し送りの項目の 3 つの種類と形を書く。
    -   新しい節「Simple Process」を「Lightweight Process」の前に置く。省かないものと省いてよいもの（AC-05・AC-06）、除外の一覧（AC-14・AC-15）、戻すことの要約、"normal process" の意味を述べる 1 文を書く。
    -   「Lightweight Process」の節の文と表は変えない。
-   [ ] **ステップ 1-2**: 要件定義書のテンプレートの状態表で、`Status` の次に `` | Process | `full` | `` の行を足す（設計書 3.1 の例と 3.9）。
-   [ ] **ステップ 1-3**: 用語集に「簡易な進め方」（simple process）、「通常の進め方」（full process）、「進め方の記録」（process record）の 3 行を、既存の行の形で足す。
-   [ ] **ステップ 1-4**: `_context.md` を設計書 2.2 の行のとおりに変える。「Process convention」の表に「Task process record」の行を足し、値の定めは規約の「Document Status Format」を参照させる（値そのものは書かない）。申し送りの 2 行の書く者と項目の種類を改め、実装モデルの行に `/mkreq` を足す。レビュアーの行は変えない。
-   [ ] **ステップ 1-5**: `CLAUDE.md` を設計書 3.9 の「`CLAUDE.md`」のとおりに変える。「Requirements and Acceptance Criteria」の手順の 2 と `:781` の文に簡易な進め方の例外と規約への参照を足し、「Claude Commands」の `/mkreq`・`/mkarch`・`/mkplan` の行に簡易な進め方に触れる語句を足す。

**完了の条件:** `make test` と `make lint` が通る。フェーズ 1 の変更をコミットしてある。

### フェーズ 2: `simple-process.md`・`/mkreq`・`task-pr-handoff.md` の Part A（設計書 8 章の 2）

**変更するファイル:** `.claude/commands/_lib/simple-process.md`（新規）、`.claude/commands/mkreq.md`、`.claude/commands/_lib/task-pr-handoff.md`

-   [ ] **ステップ 2-1**: `simple-process.md` を作る。フロントマターの `description` は、ほかの `_lib/` のファイルにならい「Internal: … (not a command)」の形にする。冒頭に、`mkarch.md`・`mkplan.md`・`mkplan2.md` の step を参照していることと、それらの step の番号か中身を変える PR は本ファイルも見直すことを書く（設計書 3.2）。このフェーズで書く節は次のとおりである（本計画 §1.3）。
    -   Reading the process record（設計書 3.1 の読む版の表と食い違いの確かめ方。値と状態は規約を参照する）。
    -   Recommending a process（設計書 3.2 の推奨の観点 4 つ。数の上限は書かない）。
    -   Writing the three documents（設計書 3.2 の 4 項目。参照する step は番号と見出しの語を併せて書く）。
    -   Reviewing the three documents（設計書 3.2 のチェックリストの渡し方と読み替え、リスクが高いときのレビュー）。
    -   Resuming a stopped run（設計書 3.3 の続けてよい条件と続ける手順）。
    -   When an excluded decision appears のうち `/mkreq` の実行中の扱い（設計書 3.3、6.3 の最初の枝）。
-   [ ] **ステップ 2-2**: `mkreq.md` を設計書 3.3 のとおりに変える。次の箇条は、設計書 3.3 の項目をすべて挙げたものである。
    -   step 5 の SCOPE に、進め方の選択を「常に開いた決定」として足す。最初の質問の回で推奨と除外の判定を示すこと、除外のときの選択肢、選択肢でない答えを尋ね直すこと、軽量プロセスの仕様のコメントを投稿して止まる実行では尋ねないことを書く。
    -   質問の回の順序（進め方と要件、設計、計画）と、設計か計画の決定で除外に当たる決定が出たときの扱いを書く。
    -   簡易な進め方では、`simple-process.md` の Writing・Reviewing の節に従って 3 文書を書き、1 本ずつコミットすることを書く。コミットの件名は今の 3 つの形のままとし、ADR の変更はその決定を記録した文書のコミットに入れる。通常の進め方でも、要件定義書の状態表に `` `full` `` の `Process` 行を書くことを書く。
    -   レビューと推敲（AC-07）: 3 文書を書いた後に Reviewing の節のレビューを行い、3 文書と ADR の変更に `japrose` を行い、直しを 1 回でコミットする。ローカルの CLI レビューは Part B の step 1 で行う。
    -   step 1 の重複の検索で見つかったときに、`simple-process.md` の Resuming の節の条件を満たせば続けることと、step 2 で新しい番号を選ばないことを書く。
    -   step 9 の報告に、簡易な進め方の実行が PR を開く前に止まったときの、走らせ直すコマンド `/mkreq <issue>` を足す。
    -   Authorization の段落が、続けてよい条件の確かめに使うネットワークの操作（`origin` のブランチの取得、ブランチの PR の一覧）を含むことを確かめ、欠けていれば足す。
-   [ ] **ステップ 2-3**: `_lib/task-pr-handoff.md` の Part A の step 3 に、呼び出し元が `/mkreq` で、`simple-process.md` の Resuming の節の条件を満たしたブランチなら、そのブランチを使う場合を足す（設計書 3.8）。Part B は変えない。

**完了の条件:** `make test` と `make lint` が通る。`simple-process.md` は投稿のコマンドを書かない。

### PR-1 作成ポイント: process rules and the mkreq entry point

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 / 2-3

**推奨タイトル**: `docs(0072): 簡易な進め方の規則と /mkreq の分岐を足す`

**レビュー観点**: 規約・テンプレートの進め方の記録の値と 4 つの状態が設計書 3.1 と一致するか / `/mkreq` の進め方の質問と続ける条件が AC-01〜AC-04・AC-12 を満たすか / `simple-process.md` が参照する step の番号と見出しの語が今の定義と合うか / 「Lightweight Process」の節の文を変えていないか

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-2・2-3 の続ける場合は、ブランチ・PR・worktree の状態の組み合わせで分岐する複雑な手順である

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `/mkarch`・`/mkplan`・`mkplan2.md`・`/runplan`（設計書 8 章の 3）

**変更するファイル:** `.claude/commands/mkarch.md`、`.claude/commands/mkplan.md`、`.claude/commands/mkplan2.md`、`.claude/commands/runplan.md`、`.claude/commands/_lib/simple-process.md`

-   [ ] **ステップ 3-1**: `mkarch.md` を設計書 3.4 のとおりに変える。
    -   step 1 の中に、タスクを決めた後の手順として、`git fetch origin main` を行い、`simple-process.md` の Reading の節で `origin/main` の記録を読むことを足す。記録が `` `simple` `` ならば文書を作らずに止まる。記録が壊れていれば止まる。食い違いがあれば、それを報告して止まる（AC-22）。既存の step の番号は変えない（`simple-process.md` が番号で参照するため）。
    -   step 6.5 に、確かめ済みの項目を問い直さずに採ることを足す。
    -   冒頭に、`/mkreq` が `simple-process.md` を通して step を参照していることを 1 文で書く。
-   [ ] **ステップ 3-2**: `mkplan.md` に、ステップ 3-1 と同じ 3 つの変更を加える（確かめ済みの項目は step 5.5 で採る）。
-   [ ] **ステップ 3-3**: `mkplan2.md` の冒頭（`:16`）に、`/mkreq` が `simple-process.md` を通して steps 4–8 を使い、step 9 のチェックリストを 3 文書のレビューに渡すことを足す（設計書 2.2）。
-   [ ] **ステップ 3-4**: `runplan.md` を設計書 3.7 のとおりに変える。step 3 で `origin/main` の記録を読み、壊れているか食い違いがあれば止まる。step 5 の「Decision change」の前に、記録が `` `simple` `` のタスクで決定の変更が除外に当たるかを判定し、当たるか迷うときは `settle-decisions.md` を呼ばずに止まる。報告に書くことは設計書 3.7 のとおりである。
-   [ ] **ステップ 3-5**: `simple-process.md` の When an excluded decision appears の節に、`/runplan` の扱い（設計書 6.3 の「マージの後」の枝）を足す。

**完了の条件:** `make test` と `make lint` が通る。

### PR-2 作成ポイント: process record in mkarch, mkplan, and runplan

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `docs(0072): /mkarch・/mkplan・/runplan が進め方の記録を読むようにする`

**レビュー観点**: `/mkarch`・`/mkplan` が Part A の前に `origin/main` を取得して記録を読み、ブランチを作らずに止まるか / 記録のない要件定義書を通常の進め方として扱うか（AC-21） / 既存の step の番号を変えていないか / `/runplan` の除外の判定が「Decision change」の前に置かれているか

**実装モデル要件**: standard

**判定理由**: 既存の止まる手順に、記録の読み取りと 1 つの判定を足すだけで、どの昇格の条件にも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: `/runpr`・`/fixpr`・`fixprguard_guide.md` と呼び出しの表のテスト（設計書 8 章の 4）

**変更するファイル:** `.claude/commands/_lib/simple-process.md`、`.claude/commands/runpr.md`、`.claude/commands/fixpr.md`、`.claude/commands/_lib/local-cli-review.md`、`docs/dev/guides/fixprguard_guide.md`、`cmd/yt2column/fixpr_docs_test.go`、`cmd/fixprguard/docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`

-   [ ] **ステップ 4-1**: `simple-process.md` に、Identifying the PR kind（設計書 3.2 の判定。題名と本文は読まない）と Returning to the full process（設計書 3.5 の「戻す手順の中身」と再開の判定）の節を足し、When an excluded decision appears の節に 3 文書の PR と実装の PR の扱い（設計書 6.3 の残りの枝）を足す。投稿と push のコマンドは書かない（それらは `runpr.md` の step 4 に置く）。
-   [ ] **ステップ 4-2**: `runpr.md` を設計書 3.5 のとおりに変える。
    -   step 2: Kind の中で 3 文書の PR と実装の PR を `simple-process.md` で見分け、食い違いと `origin/main` の `Process` 行を変える差分で止まる（`MERGED` の場合の基準は設計書 3.1）。戻す手順の途中で止まった PR の再開の判定を足す。再開では step 4 の手順の番号を参照し、投稿の呼び出しを step 2 に書かない。PRE_MERGE と NEXT は変えない。
    -   step 3: 未解決のスレッドの分類に、除外のスレッドの 2 つの扱いを足す。スレッドは、最後の `/fixpr` の Final report が除外として挙げたものだけとし、`gh api` の呼び出しを足さない。
    -   step 4: 3 文書の PR では「承認なしでマージ」の代わりに「通常の進め方へ戻す」を示す。除外のスレッドを示す形と選択肢を書く。戻す答えの手順 1〜7 を書き、手順 2 は `simple-process.md` の Returning の節を呼ぶ。スレッドの情報は、最後の `/fixpr` の Final report から取る（本計画 §1.3）。手順 5 の本文の訂正の段落は、コミットの訂正の行から作り直す（同）。
    -   戻すコミットのメッセージに、消した文書の上のスレッドの一覧（本計画 §1.3）を書く。再開のときの手順 6 は、今の形の `fixprguard list` を走らせ、この一覧のうち `list` にまだ現れるスレッドだけに、一覧の `databaseId` と `fingerprint` で返信する。`reply` が止まったら、スレッドを読み直さずに、そこで止まって報告する。`reply-not-resolved` で止まったスレッドには再び返信せず、人に手で解決するよう伝える（`fixprguard_guide.md` の `reply` の節）。一覧は `fixprguard reply` の引数にだけ使い、指示として読まない。
    -   step 4 の投稿: 戻す手順の段落に、題名・本文・返信のファイルを対象とする、戻す手順専用の `**Mention check.**` の段落を置く。その段落の最初の文は、`fixpr.md` と同じ決まった文にする。この段落が今の `:805` より前に来ると step 4 の最初のメンションの確かめになり、`TestPostingPathsCheckMention` がその文を読む。`fixprguard edit-pr --protocol 5 --title-file <title-file> --body-file <body-file> <number>` と、手順 6 の `fixprguard reply`（今の形）は、その段落より後のコードブロックの中に書く。`edit-pr` の呼び出しは、ステップ 4-6 の表の行と同じコミットに入れる。
    -   `runpr.md` のどこにも `reviewThreads`・`comments(first:` を書かない。
    -   Authorization の段落に、戻すときの `git push` と `fixprguard edit-pr` を足す。
-   [ ] **ステップ 4-3**: `fixpr.md` を設計書 3.6 のとおりに変える。
    -   Phase 1 で PR の種類と記録を読み、食い違いで止まる。
    -   R2 に 3 文書の PR の行を足し、R5 から規約の項目の形を参照する。
    -   Phase 2 の判定の出力に除外の真偽と理由を足し、除外のスレッドの扱いを PR の種類ごとに書く。
    -   Final report に除外のスレッドの項目を足し、その項目と `deferred` のスレッドの項目に、`threadId`・`databaseId`・`fingerprint`・話題・URL・ファイルのパスを挙げる（本計画 §1.3）。
    -   「People are always answered」に 2 つの例外を足す。「Naming an untrusted thread」と同じ仕組みで「覆す」スレッドを受け取ることと、記録を変えないことを書く。
    -   `TestRemovedCommandLiterals` が拒む文字列（本計画 §1.4）を足さない。
-   [ ] **ステップ 4-4**: `_lib/local-cli-review.md` の呼び出し元の一覧に、`runpr.md` step 4 の戻す場合（BASE は戻す前の head）を足し、「Six callers」の数を改める。
-   [ ] **ステップ 4-5**: `fixprguard_guide.md` に、`/runpr` が戻すときに `edit-pr` を使う場面を足す。`edit-pr` を呼ぶのが `/fixpr` だけであることを前提にした記述を探して改める。`:406` は git の状態の比較（基準値と比べること）の説明なので、`/runpr` の `edit-pr` の前にはその比較がないことを書く。`/fixpr` の Phase 6 の訂正の段落の数の確かめとは別のことである。
-   [ ] **ステップ 4-6**: 呼び出しの表のテストに、`runpr.md` の `fixprguard edit-pr` を足す（本計画 §1.4 の表の最後の 3 行）。`runpr.md` の呼び出しと 3 つの表の行は、同じコミットに入れる（片方だけのコミットでは `TestPostingCommandsInventory` か `TestDocumentedInvocationsParse` が失敗するため）。そのコミットで、次の 3 つの壊し方を 1 つずつ試し、それぞれ名指したテストが失敗することを確かめて、コミットのメッセージに書く（`CLAUDE.md`「Testing Strategy」）。
    -   `runpr.md` から呼び出しを消す → `TestFixprguardCallSites` と `TestDocumentedInvocationsParse`。
    -   呼び出しを step 4 の最初の `**Mention check.**` より前に移す → `TestPostingCommandsInventory`。
    -   `postingSections` の `runpr.md step 4` の行から `"fixprguard edit-pr "` を外す → `TestPostingCommandsInventory`。

    `make fmt` を通す。

**完了の条件:** `make test` と `make lint` が通る。ステップ 4-6 の壊して確かめた記録がコミットのメッセージにある。

### フェーズ 5: ADR-0040 の仕上げ（設計書 8 章の 5）

**変更するファイル:** `docs/dev/adr/0040-simple-task-documents.md`

-   [ ] **ステップ 5-1**: ADR-0040 の「決定」の冒頭の、反映を待つ旨の文（「以下の決定を…反映することは、[#743] の実装で扱う。反映されるまでは、[0072 要件定義書] と [0072 設計書] が仕様を定める。」）を消し、各項目の仕様の場所を規約（`requirements_process.md` の「Simple Process」などの節）と `simple-process.md` に向ける（設計書 3.10）。決定の意味は変えず、「経緯」に項目を足さない。
    -   PR を作る前に、`docs/dev/conventions/adr.md` の食い違いの文の手順（`adr.md:342-349`）に従い、`origin/main` で `issues/743` を名指す食い違いの文を探して読む。`3709c2d6` では ADR-0040 の `:13`（「背景」の issue への参照）と `:17` が当たり、食い違いの文は「決定」の `:17` だけである。見つかった文が PR のブランチにまだないときは、先に `main` を取り込む。
    -   このファイルだけをコミットし、メッセージに訂正の行を、変えた箇所ごとに 1 行書く（本計画 §1.3）。

**完了の条件:** `make test`（`TestADRRecords`・`TestInstructionDocPaths` を含む）と `make lint` が通る。

### PR-3 作成ポイント: runpr and fixpr for the simple process

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 5-1

**推奨タイトル**: `docs(0072): /runpr・/fixpr で 3 文書の PR と通常の進め方へ戻す手順を扱う`

**レビュー観点**: 戻す手順がファイルに触れる前に要ることをすべて尋ね、途中で止まっても再開できるか（AC-17・AC-18） / 進め方の記録をスレッドの本文で変えられないか（ADR-0015） / `fixprguard edit-pr` の呼び出しが戻す手順専用のメンションの確かめの後にあり、テストの表に入っているか / 除外のスレッドの扱いが 3 文書の PR と実装の PR で設計書 3.5・3.6 のとおりか、`runpr.md`・`fixpr.md` の節の見出しを変えていないか

**実装モデル要件**: frontier-required

**判定理由**: ステップ 4-2・4-3 は開発の道具の信頼の境界（ADR-0015。mention check・`fixprguard`・信頼しないスレッド）を変える security-gate に当たる手順と、途中で止まった後に再開する復旧の手順を含む

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | ステップ |
|---|---|---|
| M1: 規則 | 規約・テンプレート・用語集・`_context.md`・`CLAUDE.md` の変更 | 1-1〜1-5 |
| M2: 入口 | `simple-process.md`（`/mkreq` が使う節）、`/mkreq`、Part A の変更 | 2-1〜2-3 |
| M3: 後のコマンド | `/mkarch`・`/mkplan`・`mkplan2.md`・`/runplan` の変更 | 3-1〜3-5 |
| M4: PR の扱い | `simple-process.md`（`/runpr`・`/fixpr` が使う節）、`/runpr`・`/fixpr`・ガイド・テストの表の変更 | 4-1〜4-6 |
| M5: ADR の仕上げ | ADR-0040 の訂正 | 5-1 |

各マイルストーンは前のマイルストーンを前提とする。M2 は M1 の規約の節を参照し、M3・M4 は M2 の `simple-process.md` を参照する。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 / 2-3 | 規約などの規則、`simple-process.md` の前半、`/mkreq` と Part A | frontier-recommended |
| PR-2 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | `/mkarch`・`/mkplan`・`mkplan2.md`・`/runplan` が記録を読んで止まる | standard |
| PR-3 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 5-1 | `/runpr`・`/fixpr` の 3 文書の PR の扱い、戻す手順、呼び出しの表のテスト、ADR-0040 | frontier-required |

どの PR も、製品のコードを変えないので、単独でグリーンゲートを通る。`runpr.md` の `fixprguard edit-pr` の呼び出しと、それを読むテストの表は、PR-3 の同じコミットに入れる。

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

-   新しいテストは足さない（設計書 1.2・7.1）。既存の表に値を足すのは、ステップ 4-6 の 3 つのテストだけである。
-   各 PR で `make test` と `make lint` を走らせる。コマンドの定義と規約を読む既存のテスト（本計画 §1.4 の箇条書き）が、変えた定義を読む。
-   ステップ 4-6 で足した行は、壊し方ごとに名指したテストが失敗することを確かめる。

### 4.2. 読んで確かめる

-   設計書 7.2 のとおり、AC-01〜AC-23 の文の意味を、各 PR のレビューで読んで確かめる。どの AC をどの PR で読むかは、本計画 §7 の「ステップ」の列が示す。
-   AC に対応しない変更も、PR のレビューで設計書の節と照らして読む（本計画 §7 の最後の表）。

### 4.3. 結合テスト・後方互換のテスト

N/A。製品の実行時の振る舞いは変わらない（設計書 7.3）。進行中のタスク（`Process` 行のない要件定義書）の扱いは AC-21 として読んで確かめる。

## 5. リスク管理 (Risk Management)

### 5.1. 技術のリスク

| リスク | 影響 | 対策 |
|---|---|---|
| PR-1 のマージから PR-3 のマージまでの間に、簡易な進め方を選ぶ | `/runpr` が 3 文書の PR に「通常の進め方へ戻す」を示さず、`/fixpr` が除外のスレッドを判定しない。規約は PR-1 で、戻すときの訂正の段落の例外を述べるが、それを行う `/runpr` は PR-3 まで変わらない | PR-3 のマージまで簡易な進め方を選ばない（本計画 §1.3）。PR-1 と PR-2 の PR の説明に、このことを書く |
| `simple-process.md` が参照する `mkarch.md`・`mkplan.md`・`mkplan2.md` の step の番号が、別のタスクの変更でずれる | 簡易な進め方の `/mkreq` が別の step を使う | 参照に番号と見出しの語を併せて書き、参照される側に参照されていることを書く（設計書 5.2）。各 PR の作成の前に、`main` を取り込んだ後の step の番号を確かめ直す |
| 戻す手順の投稿の置き場所が、投稿の節のテストの決まりに合わない | `TestPostingCommandsInventory` か `TestPostingPathsCheckMention` が失敗する | ステップ 4-2 のとおり、戻す手順専用のメンションの確かめの段落を置き、その後に呼び出しを書く |
| 実装の途中で、設計書の決定と食い違う必要が見つかる | 文書と定義が食い違う | `runplan.md` step 5 の「Decision change」に従い、人に諮ってから設計書を直す |

### 5.2. 日程のリスク

| リスク | 影響 | 対策 |
|---|---|---|
| タスク 0075（軽量プロセスの削除）の実装が先にマージされる | `mkreq.md`・`requirements_process.md`・`CLAUDE.md`・`_context.md`・`runpr.md` で衝突する。ステップ 1-1 の "normal process" の 1 文が意味を失う | 0075 の要件書の決定（後からマージする側が軽量プロセスへの参照を取り除く）に従う。"normal process" の 1 文を書かなくてよくなるなど設計書と食い違うときは、`runplan.md` step 5 のとおり人に諮る |
| タスク 0073・0074 の変更が、`fixpr.md`・`runpr.md`・ADR を先に変える | 衝突と、step の番号のずれ | PR ごとに `main` を取り込み、`make test` を走らせ直す |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 / 2-3）
- [ ] PR-2 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
- [ ] PR-3 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 5-1）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

AC-01〜AC-23 は、コマンドの定義と規約の文の意味を定める。設計書 7.2 のとおり、PR のレビューで読んで確かめる（`manual`）。このうち AC-03・AC-22 は、記録の値（`` `simple` ``・`` `full` ``）と状態表の行という、値と構造を含む。値を確かめるテストを足さないことは、設計書 1.2（H-07）で決めた。値が 2 つだけで、読むのはエージェントであり、誤るおそれも害も小さいためである（`CLAUDE.md`「Testing Strategy」の最後の段落）。そのため、AC-03・AC-22 の値の部分も、設計書 3.1 と照らして読む。

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | manual | `mkreq.md` step 5 の進め方の質問と、`simple-process.md` の Recommending の節を読む | 2-1・2-2 |
| AC-02 | manual | `simple-process.md` の Recommending の節の「迷うときは当たる」と、`mkreq.md` の除外のときの選択肢と記録を読む | 2-1・2-2 |
| AC-03 | manual | 規約の「Document Status Format」の `Process` 行、テンプレートの `Process` 行、`mkreq.md` が記録を書く手順を読む。値（バッククォートを含む）と行の位置が、規約・テンプレート・設計書 3.1 の例で一致することを照らす | 1-1・1-2・2-2 |
| AC-04 | manual | `simple-process.md` の Writing の節と、`mkreq.md` の書く順とコミットを読む | 2-1・2-2 |
| AC-05 | manual | 規約の「Simple Process」の省かないものと、`simple-process.md` の Writing の節を読む | 1-1・2-1 |
| AC-06 | manual | 規約の「Simple Process」の省いてよいものと、`simple-process.md` の Reviewing の節を読む | 1-1・2-1 |
| AC-07 | manual | `mkreq.md` のレビューと推敲の手順を読む | 2-2 |
| AC-08 | manual | `simple-process.md` の Writing の節が `mkplan2.md` の steps 4–8 を使うことを読む | 2-1・3-3 |
| AC-09 | manual | `runpr.md` step 2 の PRE_MERGE と NEXT が 3 文書の PR でも変わらないことを読む | 4-2 |
| AC-10 | manual | `fixpr.md` の R2 の 3 文書の PR の行と、規約の扱い済みの項目の形を読む | 1-1・4-3 |
| AC-11 | manual | `mkarch.md`・`mkplan.md` の、Part A の前の記録の読み取りと止まる報告を読む | 3-1・3-2 |
| AC-12 | manual | `simple-process.md` の Resuming の節、`mkreq.md` の続ける場合と報告、Part A の step 3 を読む | 2-1・2-2・2-3 |
| AC-13 | manual | 規約の「LLM Constraints」の例外を読む | 1-1 |
| AC-14 | manual | 規約の「Simple Process」の除外の一覧を読む | 1-1 |
| AC-15 | manual | 規約の「Simple Process」と `simple-process.md` の Recommending の節に、数の上限と判定のスクリプトがないことを読む | 1-1・2-1 |
| AC-16 | manual | `simple-process.md` の When an excluded decision appears の節と、`mkreq.md`・`runplan.md`・`fixpr.md`・`runpr.md` の該当箇所を読む | 2-1・2-2・3-4・3-5・4-1〜4-3 |
| AC-17 | manual | `runpr.md` step 4 の戻す答えが人の答えだけで始まることと、`fixpr.md` が記録を変えないことを読む | 4-2・4-3 |
| AC-18 | manual | `simple-process.md` の Returning の節と、規約の確かめ済みの項目の形を読む | 1-1・4-1 |
| AC-19 | manual | `runpr.md` step 2 の NEXT が、戻した後の PR で `/mkarch` になることを読む | 4-2 |
| AC-20 | manual | 規約の「Simple Process」と `runpr.md` step 2 の、`origin/main` の `Process` 行を変える差分で止まる手順を読む | 1-1・4-2 |
| AC-21 | manual | 規約の記録の状態（行がないとき）と、`mkarch.md`・`mkplan.md`・`runplan.md` の読み取りを読む | 1-1・3-1・3-2・3-4 |
| AC-22 | manual | 規約の記録の状態（壊れているとき）と、`simple-process.md` の Reading の節を読む。設計書 3.1 の記録の状態の表と 4 つの食い違いのそれぞれについて、それを担うコマンド（`mkarch.md`・`mkplan.md`・`runplan.md`・`runpr.md` step 2・`fixpr.md` Phase 1）が止まって報告することを読む | 1-1・2-1・3-1・3-2・3-4・4-2・4-3 |
| AC-23 | manual | 規約・`CLAUDE.md`・用語集の変更を読む | 1-1・1-3・1-5 |
| AC-24 | test | `make test`（本計画 §1.4 の箇条書きのテストを含む）と `make lint` を、各 PR の head で走らせる。PR-3 では、ステップ 4-6 で足した `TestFixprguardCallSites`・`TestDocumentedInvocationsParse`・`TestPostingCommandsInventory` の行によって、戻すときの PR の題名と本文の書き込みが、`runpr.md` step 4 の中で、メンションの確かめの後に、`fixprguard edit-pr` の受け付ける形で書かれていることを確かめる | 各フェーズの完了の条件、4-6 |

AC に対応しない次の変更は、PR のレビューで設計書の節と照らして読む。

| 変更 | 照らす節 | ステップ |
|---|---|---|
| `_context.md` の 3 つの行 | 設計書 2.2 | 1-4 |
| `local-cli-review.md` の呼び出し元の一覧と数 | 設計書 3.5 の戻す答えの手順 3 | 4-4 |
| `fixprguard_guide.md` の `edit-pr` の場面と前提の記述（ガイドを読むテストは `make test` で走る） | 設計書 2.2・3.5 | 4-5 |
| ADR-0040 の「決定」の冒頭と仕様の場所（リンクのアンカーはテストが確かめない） | 設計書 3.10 | 5-1 |
| 規約の「Recording in the PR description」の例外 | 設計書 3.5・3.9 | 1-1 |

## 8. 成功の基準 (Success Criteria)

-   **機能の完全性:** 本計画 §7 のすべての AC の検証が済んでいる。PR-3 のマージの後、簡易な進め方が `/mkreq` から 3 文書の PR のマージまで、規約とコマンドの定義だけで進められる。
-   **品質:** 各 PR で `make test` と `make lint` が通る。ステップ 4-6 の 3 つの壊し方を確かめた記録がある。
-   **セキュリティ:** 進め方の記録と戻すことが、人の答えだけで変わる（設計書 5.1）。PR の題名と本文の書き込みは、メンションの確かめの後に `fixprguard` を通る。
-   **文書:** ADR-0040 が、反映を待つ旨の文を持たず、仕様の場所として規約と `simple-process.md` を指す。

## 9. 次のステップ (Next Steps)

-   本計画と設計書の承認の後、`/runplan 0072` で PR-1 から順に実装する。
-   PR-3 のマージの後、issue #743 を閉じる（`/runpr` が閉じる。`Closes #N` では自動で閉じない）。
-   PR-3 のマージの後に要件定義書を書くタスクから、簡易な進め方を選べる。

## 変更履歴

- 2026-11-09 承認（isseis、#759）
