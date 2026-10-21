# 試験運用の観測の記録（0031 codex_pr_review）

[01_requirements.md](01_requirements.md) の F-001（AC-01〜AC-05）の観測の記録。設計（`02_architecture.md`）は、この記録を入力にする。

## 観測の対象

| PR | 内容 | 観測した依頼 |
|---|---|---|
| [#407](https://github.com/isseis/yt2column/pull/407) | 本タスクの要件定義書 | PR を開いたときの自動レビュー（head `292db99`）、`@codex review`（head `15458c3`） |

## AC-01 の各項目

### 1. 完了の印と、レビューの途中の投稿

-   **指摘があるとき（2026-10-21、PR #407）:** PR レビュー（`chatgpt-codex-connector[bot]`、`state: COMMENTED`）が投稿された。その `commit_id` は、レビューの対象のコミットと一致した（`292db9940bd92564e769430c76f2aea00e13a3b9`、`15458c3db21106971154b4f586b5f4be6f5774ac`）。レビューの本文には `**Reviewed commit:** \`<SHA の先頭 10 桁>\`` の行がある。
-   **要約コメント:** PR の会話に、`<!-- codex-pull-request-review-summary -->` で始まるコメントが 1 件あり、レビューのたびに更新される（作成 06:14:44Z、更新 07:19:54Z）。その中の `<!-- codex-review-state:v1 {...} -->` に、`code_review.commit_sha`・`code_review.status`（`completed`）・`has_any_findings`・`has_unresolved_findings` がある（根拠: `gh api repos/isseis/yt2column/issues/407/comments`）。
-   **指摘が 0 件のとき:** 未観測。レビューの本文の説明には、指摘がなければ 👍 のリアクションだけになるとある（PR #407 のレビューの本文「About Codex in GitHub」）。
-   **レビューの途中の投稿:** 未観測（9 と合わせて観測する）。

### 2. 依頼していない push での再レビュー

-   **観測（2026-10-21、PR #407）:** 再レビューはなかった。PR #407 には PR を開いた後に 8 回以上の push があったが、Codex の PR レビューは、PR を開いたとき（`292db99`）と `@codex review` のとき（`15458c3`）の 2 回だけだった（根拠: `gh api repos/isseis/yt2column/pulls/407/reviews`）。レビューのタイミングの設定は「PR 作成時のみ」（9）。

### 3. スレッドの著者の値（AC-03）

PR #407 のスレッドの最初のコメント 4 件（`databaseId` 4333625536・4333625547・4333625556・4334061996）で、すべて同じ値だった（2026-10-21）。

| API | 種類 | 名前 |
|---|---|---|
| GraphQL（`author`） | `__typename`: `Bot` | `login`: `chatgpt-codex-connector` |
| REST（`user`） | `type`: `Bot` | `login`: `chatgpt-codex-connector[bot]` |

REST の名前は、GraphQL の名前に `[bot]` を付けた形である（CodeRabbit と同じ。design_handoff.md の H-03）。

### 4. ステータスコンテキストとチェックラン

-   **観測（2026-10-21、PR #407）:** Codex のステータスコンテキストもチェックランも付かなかった。`statusCheckRollup` にあったのは、CI のチェックランと CodeRabbit のステータスコンテキストだけだった（根拠: `gh pr view 407 --json statusCheckRollup`）。

### 5. 割り当て

-   **使用量の画面（2026-10-21、利用者の確認）:** 割り当ては、Codex・Work・Workspace Agents・ChatGPT for Excel で共通である。コードレビューに固有の上限の表示はない。
-   **上限に達したときの通知の文面:** 未観測。

### 6. `AGENTS.md` の節の見出し

-   未観測。PR #407 の指摘には `AGENTS.md reference: AGENTS.md:L3-L3` の行があり、Codex が `AGENTS.md` を読んだことは分かる。どの見出しの節をレビューの規則として読むかは、公式文書で確かめる。

### 7. 依頼からレビューの完了までの時間

| 依頼 | 依頼の時点 | 完了（PR レビューの `submitted_at`） | 所要時間 |
|---|---|---|---|
| PR を開いたとき（`292db99`） | 06:14:19Z（PR の `created_at`） | 06:17:33Z | 約 3 分 14 秒 |
| `@codex review`（`15458c3`） | 07:17:00Z（コメントの `created_at`） | 07:19:50Z | 約 2 分 50 秒 |

### 8. 書き込み権限のないアカウントで Codex が動くか

-   **設定の画面（2026-10-21、利用者の確認）:** 誰の `@codex` に応答するかを決める設定はない。
-   **GitHub App（2026-10-21、利用者の確認）:** ChatGPT Codex Connector は、選んだリポジトリだけにインストールされており、yt2column を含む。権限は次のとおり。
    -   Read: checks、commit statuses、metadata
    -   Read and write: actions、code、issues、pull requests、workflows
-   **書き込み権限のないアカウントで動くか:** 未確認。

### 9. 設定と、自動レビューの開始の印

-   **設定（2026-10-21、利用者の確認）:** レビューのタイミングは「PR 作成時のみ」、徹底的なコードレビューは無効。
-   **開始の印:** PR #407 では、PR を開いてから 25 秒後に要約コメント（1）が作られた。リアクションは残っていなかった。途中でリアクションが付いて消えたかは、未観測。

## AC-05 に当たる点

-   未判定（観測が終わってから判定する）。
