# ADR-0012: PR レビューは作成時の 1 回だけ Codex に頼み、その後の変更はローカルの CLI レビューで見る

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

このリポジトリは公開されており、GitHub のアカウントを持つ人なら誰でも PR を開ける。PR のレビューは、人が書くものではなく、外部のサービスの Bot が書くものを待つ。「レビュー済み」の判定は、その Bot が投稿した要約コメントの形と、CI の結果で行う。

レビューには 2 つの経路がある。1 つは PR そのものに対するレビュー（PR レビュー）で、GitHub の PR に投稿される。もう 1 つは、PR を開いたブランチの手元で行うローカルの CLI レビューで、変更のたびに実行できる（`.claude/commands/_lib/local-cli-review.md`）。どちらも外部のサービスの割り当て（利用できる回数）を使う。割り当てはサービスの契約ごとに別で、量は公表されていないものもある。

この決定の仕様の正本は [CLAUDE.md](../../../CLAUDE.md) の「Commands」の章（「Codex does the PR review」以降）、[.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Awaited PR reviewer」と「PR review request」と「Automated reviewer list」の行、[local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md)、[pr-review-escalation.md](../../../.claude/commands/_lib/pr-review-escalation.md)、[.coderabbit.yaml](../../../.coderabbit.yaml) である。

## 決定

-   **PR レビューは Codex（`chatgpt-codex-connector`）が行う。** 依頼は、PR を開いたときの自動レビューを待ち、約 2 分たっても始まらなければ `@codex review` をコメントして 1 回だけ頼む（`_context.md` の「PR review request」）。push のたびの自動レビューは使わない。
-   **CodeRabbit の自動 PR レビューは止める。** `.coderabbit.yaml` の `reviews.auto_review.enabled` を `false` にする。同じ PR を 2 つのサービスがレビューすると、両方の割り当てを 1 回のレビューに使うためである。
-   **PR を開いた後の変更は、ローカルの CLI レビューで見る。** CodeRabbit CLI を使い、それが失敗したときは Codex CLI を `codexreview` ツール越しに使う。指摘の記録は、CLI ごとに分けた `CodeRabbit local review:`・`Codex local review:` の行として、修正のコミット（修正がないときはその次のコミット）のメッセージと、セッションの SUMMARY に残す（[ADR-0013](0013-deterministic-checks-in-go.md)）。
-   **PR レビューを追加で頼むのは、ローカルの CLI レビューで代われない種類の変更のときと、利用者が求めるときだけにする。** 前者の判断は [pr-review-escalation.md](../../../.claude/commands/_lib/pr-review-escalation.md) に従う。Codex CLI のフォールバックは PR レビューと同じ割り当てを使うので、フォールバックを実行すること自体が割り当てを使う。
-   **「レビュー済み」は、Codex の要約コメントと CI で判定する。** `/runpr` は、1 行目が `<!-- codex-pull-request-review-summary -->` の要約コメントを待ち、その状態の行が示すコミットを見る。待ち時間と読む値の形は [.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 3 に従う。Codex のモデル・推論の強さ・徹底的なレビューは、リポジトリの外の利用者の設定で決まり、リポジトリは指定しない。
-   **信頼する自動レビュアーの一覧と、完了を待つ PR レビュアーは別の値にする。** 一覧は [.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Automated reviewer list」に 1 か所だけ置き、各名前は GraphQL の `author.login` の形（`[bot]` を付けない）で持つ。完了を待つレビュアーは「Awaited PR reviewer」の行が決め、REST の `user.login` の形はその名前に `[bot]` を付けて作る。一覧は「誰のスレッドを処理するか」（[ADR-0014](0014-untrusted-review-threads.md)）を決め、完了を待つレビュアーは「何をもってレビュー済みとするか」を決める。

## 採らなかった案

-   **CodeRabbit の PR レビューのままにする、または Codex と並べて比べてから切り替える。** CodeRabbit の PR レビューは時間あたりの上限に頻繁に当たり、機能していなかった。同じ PR で両方を待つことはできないので、比較の期間を設けられない（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   **push のたびに PR レビューを走らせる、またはスマート検出を使う。** push のたびのレビューは割り当てを消費する。スマート検出は試験的な機能で、いつ走るかを `/runpr` から予測できず、レビュー済みの判定を組めない（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   **Codex の「徹底的なコードレビュー」を有効にする。** 指摘が尽きるまで探し続けるので、`/fixpr` の方針で止める設計と合わず、待つ時間と割り当てが増える（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1、§5）。
-   **Codex のレビューが届かないときに CodeRabbit へ自動で切り替える。** CodeRabbit の PR レビューは機能しておらず、2 つの判定の方法を保守することにもなる（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   **ローカルの CLI レビューも Codex に切り替える。** CLI の割り当ては PR レビューと別であり、CodeRabbit CLI の `--base-commit` や構造化出力など、今の手順がそのまま使える（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   **Codex CLI を Claude Code の Bash 権限の前方一致で直接実行する。** `Bash(codex review *)` のような許可は、後ろに `--dangerously-bypass-approvals-and-sandbox` を付けた実行まで許す。作業ツリーにスクリプトを置く方法も、レビュー対象のブランチのコミットで中身を書き換えられる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **範囲を一時 worktree で実現する。** 後片付けと指摘のパスの読み替えが要るうえ、Codex は作業ディレクトリの外も読めるので、秘密のファイルから遠ざける利点も薄い（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **Codex のセッションの記録にある構造化された `review_output` を読む。** 公開された仕様ではなく、実行ごとに 2 つできるファイルから 1 つを選ぶ手順が要る（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **Codex にモデルと推論の強さを指定する。** モデルの名前をツールに書くと、名前が変わるたびにツールを直すことになる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **`--ephemeral` を付ける。** 観測しておらず、利用者が普段 Codex を使うときにも残る記録が、本タスクで新たに増えるリスクではない（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **Codex にレビューの指示（PROMPT）やプロジェクトの規則の指定を渡す。** Codex は指示しなくても `AGENTS.md` と `CLAUDE.md` を読み、`AGENTS.md` の内容をレビューの指示に入れる。CodeRabbit CLI の `-c CLAUDE.md` が担う役割は、これで足りる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。

## 結果

-   1 つの PR は、開いたときに 1 回だけ自動でレビューされる。push のたびのレビューはなく、以後の変更はローカルの CLI レビューで見る。
-   Codex の割り当ては、利用者の他の ChatGPT の利用と共有である。量は公表されていないので、レビューを頼む回数は、PR の作成時と、エスカレーションか利用者の依頼のときだけに限る。
-   `codexreview` は Codex CLI に利用者の設定・規則のファイル・プラグインを読ませず、環境変数も決めたものだけを渡す。レビューの対象の差分と、Codex がレビュー中に読むものは OpenAI のサービスへ送られる（[CLAUDE.md](../../../CLAUDE.md)「Tool Execution Safety」）。
-   「レビュー済み」の判定は、Codex の要約コメントの形に依存する。形が変わったときは、`/runpr` が止まるので、`runpr.md` の step 3 を直す。

## 経緯

- 2026-10-21 PR レビューを CodeRabbit の GitHub 連携から Codex に切り替え、PR を開いたときの 1 回だけにした。切り替えの前に振る舞いを観測して記録し、その結果でレビュー済みの判定を決めた。CodeRabbit の自動 PR レビューを止め、ローカルの CLI レビューは CodeRabbit のまま残した（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md)）
- 2026-10-28 ローカルの CLI レビューで CodeRabbit CLI が失敗したときに、Codex CLI を `codexreview` 越しに使うフォールバックを加えた。指摘の記録の行を CLI ごとに分け、異系統のレビューを受けていない範囲を示す行を加えた（[0049 要件定義書](../../tasks/0049_codex_local_review/01_requirements.md)）
