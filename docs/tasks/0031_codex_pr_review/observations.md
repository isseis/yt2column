# 試験運用の観測の記録（0031 codex_pr_review）

[01_requirements.md](01_requirements.md) の F-001（AC-01〜AC-05）の観測の記録。設計（`02_architecture.md`）は、この記録を入力にする。

## 観測の対象

| PR | 内容 | 観測した依頼 |
|---|---|---|
| [#407](https://github.com/isseis/yt2column/pull/407) | 本タスクの要件定義書 | PR を開いたときの自動レビュー（head `292db99`）、`@codex review`（head `15458c3`） |
| [#413](https://github.com/isseis/yt2column/pull/413) | 指摘が 0 件のときの観測のための PR（文書に空行を 1 行加えるだけ。マージせずに閉じた） | PR を開いたときの自動レビュー（head `55d3ebb`）。3 秒おきに取得して観測した |
| [#411](https://github.com/isseis/yt2column/pull/411) | この観測の記録 | PR を開いたときの自動レビュー（head `c6406fd`）、`@codex review`（head `f7e6ef5`）。どちらも数秒おきに取得して観測した |

## AC-01 の各項目

### 1. 完了の印と、レビューの途中の投稿

-   **指摘があるとき（2026-10-21、PR #407・#411）:** PR レビュー（`chatgpt-codex-connector[bot]`、`state: COMMENTED`）が投稿された。その `commit_id` は、レビューの対象のコミットと一致した（`292db9940bd92564e769430c76f2aea00e13a3b9`、`15458c3db21106971154b4f586b5f4be6f5774ac`、`c6406fd20a47b026754f9a5ade98629f2b473df9`、`f7e6ef57d9fb4d826a9b7062aa7642ed3b8dbd8e`）。レビューの本文には `**Reviewed commit:** \`<SHA の先頭 10 桁>\`` の行がある。
-   **要約コメント:** PR の会話に、`<!-- codex-pull-request-review-summary -->` で始まるコメントが 1 件あり、レビューのたびに更新される（作成 06:14:44Z、更新 07:19:54Z）。その中の `<!-- codex-review-state:v1 {...} -->` に、`code_review.commit_sha`・`code_review.status`・`has_any_findings`・`has_unresolved_findings` がある（根拠: `gh api repos/isseis/yt2column/issues/407/comments`）。PR #411 の `@codex review` では、レビューの間は `status: running`・`commit_sha: f7e6ef5…`・`has_any_findings: null` で、完了すると `status: completed`・`has_any_findings: true` に変わった（9）。
-   **指摘が 0 件のとき（2026-10-21、PR #413）:** PR レビューは投稿されなかった（PR レビューの一覧も、行へのコメントも 0 件）。要約コメントが `status: completed`・`commit_sha: 55d3ebbe7e6d16ec9bca0e238428d1ff2e11f2f9`（レビューの対象のコミット）・`has_any_findings: false`・`has_unresolved_findings: false` に更新され、PR に 👍 のリアクションが付いた（9）。レビューの本文の説明にも、指摘がなければ 👍 のリアクションだけになるとある（PR #407 のレビューの本文「About Codex in GitHub」）。
-   **完了の印の候補:** 指摘の有無にかかわらず残るのは、要約コメントの `codex-review-state` である。`commit_sha` が対象のコミットで、`status` が `completed` になる。PR レビューの `commit_id` は、指摘があるときにしか残らない。公式文書には書かれていない（[Codex GitHub integration](https://learn.chatgpt.com/docs/third-party/github)、2026-10-21 に確認）。
-   **レビューの途中の投稿:** PR に 👀 のリアクション（`chatgpt-codex-connector[bot]`）が付き、レビューの完了の後に外れた（PR #411。9）。`@codex review` のときは、依頼のコメントにも 👀 が付き、同じく完了の後に外れた（依頼したコミットは、PR を開いたときが `c6406fd`、`@codex review` のときが `f7e6ef5`）。要約コメントは `status: running` に更新された（上記）。公式文書にも「Codex が 👀 でリアクションしてからレビューを投稿する」とある（同上の「Request a Codex review」）。

### 2. 依頼していない push での再レビュー

-   **観測（2026-10-21、PR #407）:** 再レビューの跡はなかった。PR #407 には PR を開いた後に 8 回以上の push があったが、根拠は次のとおりである。ただし、指摘が 0 件の再レビューが 👍 のリアクションだけを残す（1）なら、PR レビューの数だけでは判断できない。そのため、push の直後の動きを PR #411 で直接観測する（下記）。
    -   Codex の PR レビューは、PR を開いたとき（`292db99`）と `@codex review` のとき（`15458c3`）の 2 回だけだった（根拠: `gh api repos/isseis/yt2column/pulls/407/reviews`）。
    -   PR #407 には Codex のリアクションが 1 件も残っていない（`gh api repos/isseis/yt2column/issues/407/reactions` が 0 件）。指摘が 0 件のレビューなら残るはずの 👍 もない。
    -   要約コメント（1）の最後の更新は 07:19:54Z で、`@codex review` のレビュー（07:19:50Z）の直後である。その後の 2 回の push（`d1cc2d1`・`3ba78e2`）では更新されていない。
-   **観測（2026-10-21、PR #411）:** 再レビューはなかった。07:47:42Z に `d49953c` を push し、その後 5 分間、5 秒おきに取得した。👀 も 👍 のリアクションも付かず、要約コメント（1）も更新されず（最後の更新は push の前の 07:46:47Z のまま）、Codex の PR レビューも増えなかった。
    -   同じ間に PR レビューの一覧には 2 件が加わったが、どちらも利用者（`isseis`）のもので、スレッドへの返信が作ったものだった。PR レビューの一覧には Codex 以外の投稿も入る。
-   レビューのタイミングの設定は「PR 作成時のみ」（9）。

### 3. スレッドの著者の値（AC-03）

PR #407 のスレッドの最初のコメント 4 件（`databaseId` 4333625536・4333625547・4333625556・4334061996）で、すべて同じ値だった（2026-10-21）。レビューの対象のコミット（`original_commit_id`）は、最初の 3 件が `292db99`（PR を開いたときの自動レビュー）、4 件めが `15458c3`（`@codex review`）である（根拠: `gh api repos/isseis/yt2column/pulls/comments/<databaseId>`。2026-10-22 に確認）。

| API | 種類 | 名前 |
|---|---|---|
| GraphQL（`author`） | `__typename`: `Bot` | `login`: `chatgpt-codex-connector` |
| REST（`user`） | `type`: `Bot` | `login`: `chatgpt-codex-connector[bot]` |

REST の名前は、GraphQL の名前に `[bot]` を付けた形である（CodeRabbit と同じ。design_handoff.md の H-03）。

### 4. ステータスコンテキストとチェックラン

-   **観測（2026-10-21、PR #407・#411）:** Codex のステータスコンテキストもチェックランも付かなかった。Codex がレビューした 4 つのコミット（PR #407 の `292db99`・`15458c3`、PR #411 の `c6406fd`・`f7e6ef5`）のそれぞれで、チェックランは GitHub Actions の 6 件（Test、Code Quality & Lint、Browser extension、Documentation guard、Check changed files、Secret scan）だけ、ステータスコンテキストは `CodeRabbit` だけだった（根拠: `gh api repos/isseis/yt2column/commits/<SHA>/check-runs` と `.../commits/<SHA>/status`）。

### 5. 割り当て

-   **使用量の画面（2026-10-21、利用者の確認）:** 割り当ては、Codex・Work・Workspace Agents・ChatGPT for Excel で共通である。コードレビューに固有の上限の表示はない。
-   **観測した回数（2026-10-21）:** 約 1 時間 30 分のうちに 3 回のレビュー（06:17:33Z・07:19:50Z・07:46:43Z。それぞれ PR #407 の `292db99`、PR #407 の `15458c3`、PR #411 の `c6406fd` のレビュー）が完了し、どれも上限に達しなかった。上限まで試してはいない。
-   **上限に達したときの通知の文面:** 未観測。公式文書にも書かれていない（[Codex GitHub integration](https://learn.chatgpt.com/docs/third-party/github)、2026-10-21 に確認）。上限まで試すことはしない。割り当ては ChatGPT のほかの用途と共通なので（上記）、使い切るとほかの作業にも響くためである。
-   **扱い:** 通知の文面が分からないので、文面で通知を見分けることはできない。完了を待つ PR レビュアーの投稿のうち、完了の印でもレビューの途中の投稿でもないものは、要件定義書の AC-12 のとおり想定外の投稿として止めて報告する。レート制限の通知は、この扱いで止まる。

### 6. `AGENTS.md` の節の見出し

-   **公式文書（2026-10-21 に確認）:** 見出しは `## Code Review Rules` である。規則を適用するコードに最も近い `AGENTS.md` に置く（[Codex GitHub integration](https://learn.chatgpt.com/docs/third-party/github) の「Customize what Codex reviews」）。今の `AGENTS.md` の見出しは `## Review guidelines`（`AGENTS.md:5`）で、これとは違う。
-   **PR での観測:** PR #407・#411 の指摘（レビューの対象のコミットは `292db99`・`15458c3`・`c6406fd`・`f7e6ef5`）には `AGENTS.md reference: AGENTS.md:L3-L3` の行があり、Codex が `AGENTS.md` を読んだことは分かる。どの節を規則として扱ったかは、PR からは分からない。

### 7. 依頼からレビューの完了までの時間

観測は 2026-10-21 に行った。各行に、PR と依頼の種類と依頼したコミットを書く。時刻の根拠は各 PR の `gh api` の応答である。ただし、依頼の時点が「`gh pr create` の完了」の行は、手元で `gh pr create` が終わった時刻である。

| 依頼 | 依頼の時点 | 完了（PR レビューの `submitted_at`） | 所要時間 |
|---|---|---|---|
| PR #407 を開いたとき（`292db99`） | 06:14:19Z（PR の `created_at`） | 06:17:33Z | 約 3 分 14 秒 |
| PR #407 の `@codex review`（`15458c3`） | 07:17:00Z（コメントの `created_at`） | 07:19:50Z | 約 2 分 50 秒 |
| PR #411 を開いたとき（`c6406fd`） | 07:44:15Z（`gh pr create` の完了） | 07:46:43Z | 約 2 分 28 秒 |
| PR #411 の `@codex review`（`f7e6ef5`） | 11:00:51Z | 11:03:45Z | 約 2 分 54 秒 |
| PR #413 を開いたとき（`55d3ebb`、指摘 0 件） | 11:22:47Z（`gh pr create` の完了） | 11:23:57Z（要約コメントの `completed`） | 約 1 分 10 秒 |

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
-   **書き込み権限のないアカウントでの試行（2026-10-21、PR #411）:** `isseis-gh`（yt2column での権限は `read`。Codex のアカウントを GitHub に接続していない）で試した。
    -   07:56:59Z: `isseis-gh` が `@codex review` とコメントした（コメント 6311012555）。
    -   07:57:03Z（4 秒後）: `chatgpt-codex-connector[bot]`（`Bot`）が「To use Codex here, [create a Codex account and connect to github](https://chatgpt.com/codex/cloud/settings/connectors).」と返信した（コメント 6311013571）。
    -   その後 08:04:12Z まで 5 秒おきに取得したが、👀 のリアクション、要約コメントの更新、Codex の PR レビューは、どれもなかった。
    -   利用者（`isseis`）が ChatGPT の使用量の画面で確かめたところ、このコメントで割り当ては消費されていなかった。
    -   コメントの時点の head は `9387aeb` だった。07:54:51Z に push され、次の push は 08:04:28Z である（根拠: `gh api repos/isseis/yt2column/activity?ref=refs/heads/isseis/0031-codex-pr-review-02&activity_type=push`。2026-10-22 に確認）。Codex は実行されなかったので、観測の結論には影響しない。
-   **結論:** Codex のアカウントを接続していない、書き込み権限のないアカウントの `@codex` では、Codex は実行されず、利用者の割り当ても消費されない。Codex は断りのコメントを返す。Codex のアカウントを接続している外部のアカウントの `@codex` は、確かめていない。
-   **片付け:** 試行のコメント（6311012555）の削除は、利用者が行う。

### 9. 設定と、自動レビューの開始の印

-   **設定（2026-10-21、利用者の確認）:** レビューのタイミングは「PR 作成時のみ」、徹底的なコードレビューは無効。
-   **開始の印（PR #411、head `c6406fd`、3 秒おきに取得）:**
    -   07:44:15Z: PR を開いた。
    -   07:44:35Z（20 秒後）: 要約コメント（1）が作られた。
    -   07:44:38Z までに（23 秒以内）: PR に 👀 のリアクションが付いた。
    -   07:46:43Z: PR レビューが投稿された。
    -   07:46:56Z までに: 👀 が外れた。
-   **`@codex review` のときの開始の印（PR #411、5 秒おきに取得）:**
    -   11:00:51Z: 利用者が `@codex review` とコメントした（コメント 6314314542）。
    -   11:01:04Z（13 秒後）: 依頼のコメントに 👀 が付いた。
    -   11:01:08Z: 要約コメントが `status: running`・`commit_sha: f7e6ef5…` に更新された。
    -   11:01:10Z: PR にも 👀 が付いた。
    -   11:03:45Z: PR レビューが投稿された。11:03:49Z に要約コメントが `status: completed` に更新された。
    -   11:03:58Z までに: 両方の 👀 が外れた。
-   **指摘が 0 件のときの開始と完了の印（PR #413、3 秒おきに取得）:**
    -   11:22:47Z: PR を開いた。
    -   11:22:59Z（12 秒後）: 要約コメントが `status: running`・`commit_sha: 55d3ebb…` で作られた。
    -   11:23:01Z（14 秒後）: PR に 👀 が付いた。
    -   11:23:57Z（70 秒後）: 要約コメントが `status: completed`・`has_any_findings: false` に更新された。
    -   11:24:00Z（73 秒後）: 👀 が外れ、👍 が付いた。
-   **PR #407（head `292db99`）:** PR を開いてから 25 秒後に要約コメントが作られた。リアクションは残っていなかった（PR #411 と同じく、完了の後に外れたと考えられる）。

## AC-05 に当たる点

-   **項目 8:** 当たらない。Codex のアカウントを接続していない、書き込み権限のないアカウントの `@codex` では、Codex は動かず、割り当ても消費されなかった。ゲートの判定の範囲は、この観測を受けて要件定義書で絞った（01_requirements.md の 1 節の決定、2026-10-21）。
-   **指摘が 0 件の完了の印（AC-05 の 2 つめ）:** 当たらない。要約コメントの `codex-review-state` が、対象のコミットと `status: completed` を持つ（1）。
-   **AC-04:** 満たした。指摘が 0 件のレビュー（PR #413）と 1 件以上のレビュー（PR #407・#411）を観測した。
-   **開始の印（AC-05 の 5 つめ）:** 当たらない。👀 のリアクションと、要約コメントの `status: running` がある（9）。
-   **要件との矛盾（AC-05 の 1 つめ）:** 当たらない。「PR 作成時のみ」の設定で、依頼していない push ではレビューされなかった（2）。Codex のスレッドの著者は `Bot` だった（3）。
-   **まとめ:** AC-05 のどの項目にも当たらない。設計（`02_architecture.md`）に進める。上限の通知の文面（5）は未観測で、AC-12 で扱う。

## 設計への入力

設計で決める事項（[design_handoff.md](design_handoff.md)）に、観測から分かったことを対応させる。

| 申し送り | 観測から分かったこと |
|---|---|
| H-01（完了の印） | 指摘の有無にかかわらず残るのは、要約コメントの `codex-review-state`（`commit_sha`・`status`・`has_any_findings`）である（1）。PR レビューは指摘があるときだけ投稿され、その `commit_id` は対象のコミットと一致する。PR レビューの一覧には、利用者がスレッドに返信したときのものも入る（2）。外部の人の `@codex` への断りのコメントも、同じ Bot が投稿する（8） |
| H-03（REST の名前） | GraphQL の `chatgpt-codex-connector` に `[bot]` を付けた形が REST の名前である（3） |
| H-05（ステータス・チェック） | Codex のステータスコンテキストもチェックランも付かない（4） |
| H-08（待つ時間） | 完了までの時間は約 1 分 10 秒〜3 分 14 秒（7）。開始の印（👀、要約コメントの `running`）は 12〜23 秒で現れた（9） |
| AC-26（`AGENTS.md`） | 公式文書の見出しは `## Code Review Rules` で、今の `## Review guidelines` とは違う（6） |
