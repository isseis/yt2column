# 観測の記録：CodeRabbit の PR レビュー（0118 の F-001）

本書は、[01_requirements.md](01_requirements.md) の F-001（AC-01〜AC-05）が求める観測の記録である。`02_architecture.md` が完了の印や割り当ての知らせとして使う値は、すべて本書にある（AC-05）。

## 1. 観測の条件

-   **観測した日:** 2026-11-20（時刻はすべて UTC）。
-   **試験の PR:** [#1064](https://github.com/isseis/yt2column/pull/1064)。ブランチは `isseis/0118-coderabbit-probe`、ベースは `main` の `e999fa2c`。2026-11-20T04:46:32Z に、マージせずに閉じ、ブランチを消した（AC-03）。
-   **試験の PR の commit:**
    -   **commit 1（`7cefaa5e`）:** `tmp-coderabbit-probe/README.md` だけを加える。指摘が出にくい短い文書である。
    -   **commit 2（`f24ee60a`）:** わざと欠陥を入れた Go のファイル（`tmp-coderabbit-probe/probe/probe.go`）と、4.4 MB のテキストのファイル（`tmp-coderabbit-probe/large.txt`）を加える。
-   **依頼のコメント:** すべて利用者（`isseis`、`User`）が GitHub の画面から投稿した。本文は `@coderabbitai review` である。エージェントは依頼のコメントを投稿していない（AC-02）。
-   **エージェントが行った操作:** 試験の PR の作成、commit 2 の push、スレッドへの返信（項目 10）、PR を閉じてブランチを消すこと。どれも利用者の承認を得て行った（5 節の制約）。
-   **CodeRabbit の設定:** `.coderabbit.yaml` は `reviews.auto_review.enabled: false` である。要約コメントの「Run configuration」には `Plan: Advanced` と出た。一方、割り当ては「free OSS reviews」として「up to 1 included review per hour」と示された（項目 3）。
-   **別の PR での観測:** 項目 3 の一部は、利用者が別のタスクの PR [#1063](https://github.com/isseis/yt2column/pull/1063) に投稿した依頼のコメントでも観測した。その PR はこの観測のためのものではなく、閉じていない。

依頼のコメントは 4 件あった。

| 依頼 | 時刻 | PR | head | 結果 |
|---|---|---|---|---|
| R1 | 03:20:06Z | #1064 | `7cefaa5e` | 完了（指摘なし） |
| R0 | 03:27:39Z | #1063 | `a7bc3dcb` | 割り当て切れ |
| R2 | 03:58:32Z | #1064 | `f24ee60a` | 割り当て切れ |
| R3 | 04:26:01Z | #1064 | `f24ee60a` | 完了（指摘あり） |

読んだ API は次のとおりである。REST の `issues/<n>/comments`、`pulls/<n>/reviews`、`pulls/<n>/comments`、`issues/<n>/timeline`、`commits/<sha>/check-runs`（`filter=latest` と `filter=all`）、`commits/<sha>/status`、`commits/<sha>/statuses`、GraphQL の `pullRequest`（`reviewThreads`、`reviews`、`comments`、`commits` の `statusCheckRollup`）である。

## 2. Codex の投稿（CodeRabbit の観測と分ける）

試験の PR を開いたとき、Codex の連携はまだ動いていた（AC-03）。

-   **要約コメント:** `chatgpt-codex-connector[bot]`（`Bot`）が、03:18:05Z に 1 行目が `<!-- codex-pull-request-review-summary -->` の要約コメントを投稿し、03:19:14Z に書き換えた。本文には `@codex` の文字列が含まれた。
-   **ほかの投稿:** Codex は push でも依頼でも、それ以上は投稿しなかった。PR レビューのオブジェクトも、受領コメントも作らなかった。

本書のほかの節は、Codex の投稿を含まない。

## 3. 観測した項目（AC-01）

### 3.1. 項目 1：依頼からレビューの終わりまでに投稿・変更するものと順序

依頼 R3（指摘あり）の順序は次のとおりである。R1（指摘なし）も、PR レビューとスレッドがないことのほかは同じだった。

| 時刻 | 投稿・変更 | 値 |
|---|---|---|
| 04:26:01Z | 依頼のコメント | `User` `isseis` |
| 04:26:08Z | 依頼への返信のコメントを新しく投稿 | 本文は `<details>` の `<summary>Action performed</summary>` と「Review triggered.」 |
| 04:26:25Z | CheckRun `CodeRabbit` を作る | `status: in_progress`、`output.title: "Review in progress"` |
| 04:29:12Z | 要約コメント（PR を開いたときに作ったもの）を同じ場所で書き換える | `id` 7029283771 のまま、`updated_at` だけが変わる |
| 04:29:14Z | レビューのスレッドを 3 件作る | 項目 4 |
| 04:29:15Z | PR レビューを 1 件作る | `state: COMMENTED`、`commit_id: f24ee60a…` |
| 04:29:18Z | CheckRun を終える | `status: completed`、`conclusion: success`、`output.title: "Finished"` |
| 04:29:19Z | 依頼への返信を同じ場所で書き換える | 本文は「Review finished.」 |

-   **PR の説明:** 最初に完了したレビュー（R1）の後に、PR の説明の末尾へ `<!-- This is an auto-generated comment: release notes by coderabbit.ai -->` で囲んだ「Summary by CodeRabbit」を書き足した（03:22 ごろ）。
-   **`StatusContext`:** どの head にも作らなかった。`commits/<sha>/status` は `state: pending`、`statuses` は空である。0031 より前の判定が見ていた `StatusContext` の `CodeRabbit` は、今は存在しない。
-   **CheckRun の識別:**
    -   **app:** CheckRun の app は `id: 347564`、`slug: coderabbitai`、`owner.login: coderabbitai` である。
    -   **`external_id`:** `coderabbit-review-progress-v1:<64 桁の 16 進>` の形だった。
    -   **CheckRun の数:** 1 つの commit に、依頼のたびに新しい CheckRun ができた。`f24ee60a` の上には、`skipped`（03:23:35Z）、`neutral`（03:58:46Z）、`success`（04:26:25Z）の 3 つがある。
-   **`filter` の違い:** REST の `commits/<sha>/check-runs` は、既定（`filter=latest`）では同じ名前の最新の 1 つだけを返し、`filter=all` で 3 つすべてを返した。GraphQL の `statusCheckRollup.contexts` も、名前ごとに最新の 1 つだけを返した。

### 3.2. 項目 2：完了したことと、レビューしたコミットを示す値

| 値 | 指摘なし（R1） | 指摘あり（R3） |
|---|---|---|
| CheckRun `CodeRabbit` | `head_sha: 7cefaa5e…`、`completed`、`success`、`Finished`、`started_at` 03:20:24Z（依頼の 18 秒後）、`completed_at` 03:22:24Z | `head_sha: f24ee60a…`、`completed`、`success`、`Finished`、`started_at` 04:26:25Z（依頼の 24 秒後）、`completed_at` 04:29:18Z |
| CheckRun の `output.summary` | `Result: Success - Review completed` で始まる表 | 同じ |
| PR レビューのオブジェクト | なし | 1 件。`user.type: Bot`、`commit_id: f24ee60a…`、`state: COMMENTED` |
| 依頼への返信 | 「Review finished.」 | 「Review finished.」 |
| 要約コメントの中の値 | `<!-- final_review_risk_coverage:{"sourceCommitId":"7cefaa5e…","coveredCommitId":"7cefaa5e…","kind":"reviewed"} -->` と、「between `e999fa2c…` and `7cefaa5e…`.」の行 | `change_assessment_commit:"f24ee60a…"` など |

-   **CheckRun だけが持つ値:** レビューしたコミット（`head_sha`）と完了を、本文を読まずに示すのは CheckRun だけである。依頼への返信はコミットを示さない。要約コメントの値は本文の中にある。
-   **PR レビューのオブジェクト:** 指摘があるときだけできる。指摘のない完了の印にはならない。
-   **依頼と応答を結ぶ値:** 依頼への返信は `<!-- CodeRabbit review command invocation: v2:<64 桁の 16 進> -->` を持つ。この値がどの依頼のコメントから作られたかは、公開されていない。依頼への返信の `created_at` は依頼の 4〜7 秒後だった。

### 3.3. 項目 3：割り当てを使い切ったとき、エラーのとき

割り当て切れは、R0（PR #1063、03:27:39Z）と R2（PR #1064、03:58:32Z）で観測した。どちらも R1 のレビューから 1 時間の内だった。

| 値 | R0（#1063） | R2（#1064） |
|---|---|---|
| CheckRun `CodeRabbit` | `head_sha: a7bc3dcb…`、`completed`、`conclusion: neutral`、`output.title: "Review rate limited"`、`started_at` = `completed_at` = 03:27:51Z | `head_sha: f24ee60a…`、`completed`、`neutral`、`Review rate limited`、03:58:46Z |
| `output.summary` | `Result: Rate limited - Review rate limited` で始まり、手順の表はすべて `Skipped` | 同じ |
| 依頼への返信 | `<summary>Action not completed</summary>` と「Review rate limited.」 | 同じ |
| 要約コメント | `<!-- This is an auto-generated comment: rate limited by coderabbit.ai -->` の節。見出しは「Review limit reached」、本文は「You've used all free OSS reviews for now.」「Next included review available in 52 minutes.」 | 同じ節で「21 minutes」 |

-   **「between」の行:** 割り当て切れの要約コメントも「Reviewing files that changed from the base of the PR and between `e999fa2c…` and `a7bc3dcb…`.」の行を持っていた（R0）。そのため、この行は完了の印に使えない。
-   **割り当ての大きさ:** 完了したレビュー（R3）の本文は「Your plan provides up to 1 included review per hour; 0 remain after this review.」と示した。割り当ては PR をまたいで共有される（R1 の後の R0 が、別の PR で割り当て切れになった）。
-   **エラーのとき（観測できなかった）:** 公式の文書から分かることは次のとおりである。
    -   CheckRun の名前は `CodeRabbit` で、失敗のときは `Review failed` か、原因を示す説明（`Summary could not be published` など）になる。
    -   `reviews.fail_commit_status` が既定の `false` なら、GitHub の CheckRun は `neutral` で終わる。取り消しは `Review cancelled` である。
    -   スキップのときは `Review skipped`、`Review paused`、`No files to review`、`Too many files to review`、`Incremental review skipped` などの説明になる。
    -   出典は https://docs.coderabbit.ai/pr-reviews/status-check.md と https://docs.coderabbit.ai/management/rate-limits.md （2026-11-20 に読んだ）。
    -   エラーも `neutral` で終わりうるので、`conclusion` だけでは割り当て切れと区別できない。区別には `output.title` が要る。

### 3.4. 項目 4：著者の値

| 投稿 | GraphQL `author` | REST `user` |
|---|---|---|
| 要約コメント、依頼への返信、自動修正の状態のコメント | `__typename: Bot`、`login: coderabbitai` | `type: Bot`、`login: coderabbitai[bot]`（`id` 136622811） |
| PR レビュー | `Bot`、`coderabbitai` | `Bot`、`coderabbitai[bot]` |
| スレッドのコメント | `Bot`、`coderabbitai` | `Bot`、`coderabbitai[bot]` |
| CheckRun | （GraphQL の `checkSuite.app.slug: coderabbitai`） | `app.id: 347564`、`app.slug: coderabbitai` |

### 3.5. 項目 5：自動レビューを止めたまま、PR を開いたときと push したとき

-   **PR を開いたとき（03:17:55Z）:**
    -   CodeRabbit は 03:18:00Z に要約コメントを投稿した。本文は「Review skipped」の節（`<!-- This is an auto-generated comment: skip review by coderabbit.ai -->`）で、「Auto reviews are disabled on this repository. … To trigger a single review, invoke the `@coderabbitai review` command.」と書いてある。
    -   同時に、CheckRun `CodeRabbit` を `conclusion: skipped`、`output.title: "Review skipped: automatic reviews are disabled"` で作った（03:18:01Z）。
    -   レビューは始めなかった。
-   **push したとき（commit 2、03:23:16Z）:**
    -   CodeRabbit は 03:23:34Z に要約コメントを書き換え、「Review skipped」の節を足した。
    -   新しい head に、同じ `skipped` の CheckRun を作った（03:23:35Z）。
    -   レビューは始めなかった。
-   **要件の決定との関係:** どちらも CodeRabbit が自分でレビューを始めたのではないので、AC-04 の止める条件には当たらない。ただし、依頼のない CheckRun（`skipped`）は PR を開いたときにも push のときにもできるので、完了の印の判定はこれを数えない必要がある。

### 3.6. 項目 6：`@coderabbitai` と `@codex` の文字列

-   **`@coderabbitai` を含むもの:** 要約コメントは、「Review skipped」の節を持つ間（PR を開いた直後と push の直後）、`@coderabbitai` を含んだ。共有の案内（tips）の URL にも `%40coderabbitai` がある。
-   **含まないもの:**
    -   依頼への返信、PR レビュー、スレッドのコメント、自動修正の状態のコメント、PR の説明に書き足した要約は、どちらの文字列も含まなかった。
    -   完了した後の要約コメントも、どちらの文字列も含まなかった。
    -   `@codex` は、CodeRabbit のどの投稿にもなかった。
-   **1 行目:** CodeRabbit の投稿は、1 行目がすべて `<!-- coderabbit-ai-generated:v1 -->` だった。例外が 2 つある。
    -   **書き換えた自動修正の状態のコメント:** 1 行目が `<!-- This is an auto-generated comment: coding agent status by CodeRabbit -->` に変わった。
    -   **利用者が「Fix comments」のチェックボックスを押した後の PR レビューの本文:** 改行が CRLF になり、1 行目は `<!-- coderabbit-ai-generated:v1 -->` の後に CR が付いた。

### 3.7. 項目 7：指摘をスレッドにせず本文だけに書くこと

-   **R3 の数の食い違い:** R3 の PR レビューの本文は「**Actionable comments posted: 4**」と書いたが、スレッドは 3 件だった。同じ行（26 行目）への 2 つの指摘が 1 つのスレッドにまとまったと読める。
-   **PR #852 の記録:** `internal/chief/testdata/gh/pr-view-852-reviews.json` では、本文の中の `<summary>` に「Additional comments: 1」があり、指摘がスレッドにならずに本文だけにあった。
-   **値の有無:** 本文だけにある指摘の有無や数を、本文を読まずに示す値（API の欄）はなかった。数は本文の文言の中にしかなく、その文言の種類も公開されていない。そのため、AC-52 の場合に当たる。

### 3.8. 項目 8：一部のファイルをレビューしなかったことを示す値

-   **R3 の本文:** 「Files selected for processing: 2」（`large.txt` と `probe.go`）の後に、「Files not reviewed because the changes were too large to process: 1」と `tmp-coderabbit-probe/large.txt` を示した。
-   **値の有無:** API の欄にこれを示す値はなかった。CheckRun の `conclusion` は `success` で、`output.title` は `Finished` のままだった。分かるのは、本文の英語の見出しだけである。

### 3.9. 項目 9：指摘を直す commit を PR のブランチに作る機能

-   **利用者の操作:** 利用者が 04:37 ごろ、R3 の PR レビューの「Fix comments: Fix CodeRabbit comments on this PR」のチェックボックスを押した。
-   **CodeRabbit の投稿:** 04:37:59Z に「Coding task started … for 3 unresolved review comments.」のコメントを投稿した。
-   **push の結果:** CodeRabbit の作業の画面には、リポジトリのブランチが保護されていて push できないと出た（利用者が見た）。
    -   04:45:19Z に、同じコメントが「⚠️ Coding task changes are ready, but delivery needs attention」に書き換わった。
    -   PR の head は `f24ee60a` のままで、新しいブランチもできなかった。
-   **ルールセット:** ブランチの作成と更新を制限するルールセット（`Restrict branch creation/updates`、`main` 以外のすべてのブランチ）が適用されていた。そのルールセットの bypass は管理者の役割だけである。
-   **要件の決定との関係:** 今のブランチの push の制限の下では、CodeRabbit は PR のブランチに push できなかった。また、この機能は利用者がチェックボックスを押したときだけ動いた。AC-04 の止める条件には当たらない。
-   **公式の文書:** 「Fix comments」は、利用者の GitHub の認可で commit を push するか、積み重ねた PR を開く。「Repository permissions and branch protections still apply.」とある（https://docs.coderabbit.ai/finishing-touches/autofix.md 、https://docs.coderabbit.ai/code/deliver-changes.md 、2026-11-20 に読んだ）。

### 3.10. 項目 10：レビューの後にスレッドへ返信したとき

-   **返信:** エージェントが利用者のアカウントで、R3 のスレッドの 1 つ（`discussion_r4638479103`）に、メンションを含まない返信を投稿した（04:30:14Z）。本文は「この PR は観測のための試験の PR で、マージしません。この指摘は直さずに閉じます。」である。
    -   GitHub は、この返信を `User` `isseis` の PR レビュー（`COMMENTED`、本文は空、`commit_id: f24ee60a…`）として記録した。
-   **CodeRabbit の応答:** 04:30:36Z に、返信のコメントへ 🚀 の反応（`rocket`）を付けた。04:37Z までに、返信のコメント、PR レビュー、CheckRun のどれも作らなかった。反応はコミットを示さない。
-   **公式の文書:** CodeRabbit が始めたスレッドの返信には、メンションがなくても既定で答える。了解だけの返信には 🚀 の反応を付ける、とある（https://docs.coderabbit.ai/guide/chat.md 、2026-11-20 に読んだ）。返信の内容によっては、返信のコメントを投稿しうる。

### 3.11. 項目 11：`AGENTS.md` を読むか（公式の文書から）

-   **CodeRabbit の PR レビュー:** 読む。「code guidelines」は、`**/AGENTS.md`、`**/CLAUDE.md` などを既定で見つけ、レビューの基準として使う。止めるには `knowledge_base.code_guidelines.enabled: false` とする（https://docs.coderabbit.ai/knowledge-base/code-guidelines.md 、2026-11-20 に読んだ）。
-   **CodeRabbit CLI:** `AGENTS.md` や `CLAUDE.md` を自分で見つけるとは書いていない（https://docs.coderabbit.ai/cli/index.md ）。追加の指示は `-c` で渡す（https://docs.coderabbit.ai/configuration/local-reviews.md ）。今のローカルの CLI レビューは `-c CLAUDE.md` を渡している（`.claude/commands/_lib/local-cli-review.md:134`）。

## 4. 本書の決定との食い違い（AC-04）

観測の結果に、要件定義書の決定と食い違うものはなかった。

-   **項目 5:** CodeRabbit は自分でレビューを始めなかった。
-   **項目 9:** CodeRabbit は PR のブランチに push できなかった。

そのため、設計に進む。
