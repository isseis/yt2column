# 試験運用の観測の記録（0031 codex_pr_review）

[01_requirements.md](01_requirements.md) の F-001（AC-01〜AC-05）の観測の記録。設計（`02_architecture.md`）は、この記録を入力にする。

## 観測の対象

| PR | 内容 | 観測した依頼 |
|---|---|---|
| [#407](https://github.com/isseis/yt2column/pull/407) | 本タスクの要件定義書 | PR を開いたときの自動レビュー（head `292db99`）、`@codex review`（head `15458c3`） |
| [#411](https://github.com/isseis/yt2column/pull/411) | この観測の記録 | PR を開いたときの自動レビュー（head `c6406fd`）。数秒おきに取得して観測した |

## AC-01 の各項目

### 1. 完了の印と、レビューの途中の投稿

-   **指摘があるとき（2026-10-21、PR #407・#411）:** PR レビュー（`chatgpt-codex-connector[bot]`、`state: COMMENTED`）が投稿された。その `commit_id` は、レビューの対象のコミットと一致した（`292db9940bd92564e769430c76f2aea00e13a3b9`、`15458c3db21106971154b4f586b5f4be6f5774ac`、`c6406fd20a47b026754f9a5ade98629f2b473df9`）。レビューの本文には `**Reviewed commit:** \`<SHA の先頭 10 桁>\`` の行がある。
-   **要約コメント:** PR の会話に、`<!-- codex-pull-request-review-summary -->` で始まるコメントが 1 件あり、レビューのたびに更新される（作成 06:14:44Z、更新 07:19:54Z）。その中の `<!-- codex-review-state:v1 {...} -->` に、`code_review.commit_sha`・`code_review.status`（`completed`）・`has_any_findings`・`has_unresolved_findings` がある（根拠: `gh api repos/isseis/yt2column/issues/407/comments`）。
-   **指摘が 0 件のとき:** 未観測。レビューの本文の説明には、指摘がなければ 👍 のリアクションだけになるとある（PR #407 のレビューの本文「About Codex in GitHub」）。公式文書には書かれていない（[Codex GitHub integration](https://learn.chatgpt.com/docs/third-party/github)、2026-10-21 に確認）。
-   **レビューの途中の投稿:** PR に 👀 のリアクション（`chatgpt-codex-connector[bot]`）が付き、レビューの完了の後に外れた（PR #411。9）。公式文書にも「Codex が 👀 でリアクションしてからレビューを投稿する」とある（同上の「Request a Codex review」）。

### 2. 依頼していない push での再レビュー

-   **観測（2026-10-21、PR #407）:** 再レビューの跡はなかった。PR #407 には PR を開いた後に 8 回以上の push があったが、根拠は次のとおりである。ただし、指摘が 0 件の再レビューが 👍 のリアクションだけを残す（1）なら、PR レビューの数だけでは判断できない。そのため、push の直後の動きを PR #411 で直接観測する（下記）。
    -   Codex の PR レビューは、PR を開いたとき（`292db99`）と `@codex review` のとき（`15458c3`）の 2 回だけだった（根拠: `gh api repos/isseis/yt2column/pulls/407/reviews`）。
    -   PR #407 には Codex のリアクションが 1 件も残っていない（`gh api repos/isseis/yt2column/issues/407/reactions` が 0 件）。指摘が 0 件のレビューなら残るはずの 👍 もない。
    -   要約コメント（1）の最後の更新は 07:19:54Z で、`@codex review` のレビュー（07:19:50Z）の直後である。その後の 2 回の push（`d1cc2d1`・`3ba78e2`）では更新されていない。
-   **観測（2026-10-21、PR #411）:** 再レビューはなかった。07:47:42Z に `d49953c` を push し、その後 5 分間、5 秒おきに取得した。👀 も 👍 のリアクションも付かず、要約コメント（1）も更新されず（最後の更新は push の前の 07:46:47Z のまま）、Codex の PR レビューも増えなかった。
    -   同じ間に PR レビューの一覧には 2 件が加わったが、どちらも利用者（`isseis`）のもので、スレッドへの返信が作ったものだった。PR レビューの一覧には Codex 以外の投稿も入る。
-   レビューのタイミングの設定は「PR 作成時のみ」（9）。

### 3. スレッドの著者の値（AC-03）

PR #407 のスレッドの最初のコメント 4 件（`databaseId` 4333625536・4333625547・4333625556・4334061996）で、すべて同じ値だった（2026-10-21）。

| API | 種類 | 名前 |
|---|---|---|
| GraphQL（`author`） | `__typename`: `Bot` | `login`: `chatgpt-codex-connector` |
| REST（`user`） | `type`: `Bot` | `login`: `chatgpt-codex-connector[bot]` |

REST の名前は、GraphQL の名前に `[bot]` を付けた形である（CodeRabbit と同じ。design_handoff.md の H-03）。

### 4. ステータスコンテキストとチェックラン

-   **観測（2026-10-21、PR #407・#411）:** Codex のステータスコンテキストもチェックランも付かなかった。`statusCheckRollup` にあったのは、CI のチェックランと CodeRabbit のステータスコンテキストだけだった（根拠: `gh pr view 407 --json statusCheckRollup`）。

### 5. 割り当て

-   **使用量の画面（2026-10-21、利用者の確認）:** 割り当ては、Codex・Work・Workspace Agents・ChatGPT for Excel で共通である。コードレビューに固有の上限の表示はない。
-   **観測した回数（2026-10-21）:** 約 1 時間 30 分のうちに 3 回のレビュー（06:17:33Z・07:19:50Z・07:46:43Z）が完了し、どれも上限に達しなかった。上限まで試してはいない。
-   **上限に達したときの通知の文面:** 未観測。公式文書にも書かれていない（[Codex GitHub integration](https://learn.chatgpt.com/docs/third-party/github)、2026-10-21 に確認）。

### 6. `AGENTS.md` の節の見出し

-   **公式文書（2026-10-21 に確認）:** 見出しは `## Code Review Rules` である。規則を適用するコードに最も近い `AGENTS.md` に置く（[Codex GitHub integration](https://learn.chatgpt.com/docs/third-party/github) の「Customize what Codex reviews」）。今の `AGENTS.md` の見出しは `## Review guidelines`（`AGENTS.md:5`）で、これとは違う。
-   **PR での観測:** PR #407・#411 の指摘には `AGENTS.md reference: AGENTS.md:L3-L3` の行があり、Codex が `AGENTS.md` を読んだことは分かる。どの節を規則として扱ったかは、PR からは分からない。

### 7. 依頼からレビューの完了までの時間

| 依頼 | 依頼の時点 | 完了（PR レビューの `submitted_at`） | 所要時間 |
|---|---|---|---|
| PR を開いたとき（`292db99`） | 06:14:19Z（PR の `created_at`） | 06:17:33Z | 約 3 分 14 秒 |
| `@codex review`（`15458c3`） | 07:17:00Z（コメントの `created_at`） | 07:19:50Z | 約 2 分 50 秒 |
| PR #411 を開いたとき（`c6406fd`） | 07:44:15Z（`gh pr create` の完了） | 07:46:43Z | 約 2 分 28 秒 |

### 8. 書き込み権限のないアカウントで Codex が動くか

-   **設定の画面（2026-10-21、利用者の確認）:** 誰の `@codex` に応答するかを決める設定はない。
-   **GitHub App（2026-10-21、利用者の確認）:** ChatGPT Codex Connector は、選んだリポジトリだけにインストールされており、yt2column を含む。権限は次のとおり。
    -   Read: checks、commit statuses、metadata
    -   Read and write: actions、code、issues、pull requests、workflows
-   **公式文書（2026-10-21 に確認）:** 誰の `@codex` に応えるかは書かれていない。書かれているのは、自動レビューの設定に push か admin の権限が要ることだけである（[Codex GitHub integration](https://learn.chatgpt.com/docs/third-party/github) の「Set up Codex code review」）。
-   **他のリポジトリでの報告（二次的な根拠）:** [Student-Team-Projects/Most-Overkilled-Whiteboard-for-ArchLinux#76](https://github.com/Student-Team-Projects/Most-Overkilled-Whiteboard-for-ArchLinux/issues/76) の 2026-10-16 のコメントによる。この PR では試していない。
    -   自動レビューは、PR を開いた人が Codex のアカウントを GitHub に接続しているときだけ走る。
    -   Codex のアカウントを接続していない人の `@codex` には、Codex は「To use Codex here, create a Codex account and connect to github」と返信するだけで、実行しない。
    -   接続しているメンバーが `@codex review` と書くと、そのメンバーのアカウントと割り当てで実行される。
    -   レビュー以外の依頼（修正の依頼など）には「To use Codex here, create an environment for this repo」と返信した。
    -   この報告の投稿者たちは、そのリポジトリへの書き込み権限を持っていたとみられる。書き込み権限のない人での試行ではない。
-   **書き込み権限のないアカウントで動くか:** 未確認。

### 9. 設定と、自動レビューの開始の印

-   **設定（2026-10-21、利用者の確認）:** レビューのタイミングは「PR 作成時のみ」、徹底的なコードレビューは無効。
-   **開始の印（PR #411、3 秒おきに取得）:**
    -   07:44:15Z: PR を開いた。
    -   07:44:35Z（20 秒後）: 要約コメント（1）が作られた。
    -   07:44:38Z までに（23 秒以内）: PR に 👀 のリアクションが付いた。
    -   07:46:43Z: PR レビューが投稿された。
    -   07:46:56Z までに: 👀 が外れた。
-   **PR #407:** PR を開いてから 25 秒後に要約コメントが作られた。リアクションは残っていなかった（PR #411 と同じく、完了の後に外れたと考えられる）。

## AC-05 に当たる点

-   未判定（観測が終わってから判定する）。
