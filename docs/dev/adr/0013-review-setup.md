# ADR-0013: PR レビューは CodeRabbit（手動）と Sourcery AI（自動・手動）を併用し、PR を開いた後の変更はローカルの CLI レビューで見る

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

このリポジトリは公開されており、GitHub のアカウントを持つ人なら誰でも PR を開ける。PR のレビューは、人が書くものではなく、外部のサービスの Bot が書くものを待つ。

レビューには 2 つの経路がある。1 つは PR そのものに対するレビュー（PR レビュー）で、GitHub の PR に投稿される。もう 1 つは、PR を開いたブランチの手元で行うローカルの CLI レビューで、変更のたびに実行できる（[.claude/commands/_lib/local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md)）。どちらも外部のサービスの割り当て（利用できる回数）を使う。割り当てはサービスの契約ごとに別で、量は公表されていないものもある。

## 決定

PR レビューは CodeRabbit（手動）と Sourcery AI（自動・手動）を併用し、PR を開いた後の変更はローカルの CLI レビュー（CodeRabbit CLI）で見る。エスカレーションの判定は、マージの質問で PR レビューを推奨にするかだけを決める。「レビュー済み」は、CodeRabbit と Sourcery AI の完了の印（和集合）と CI で判定する。

### PR レビューの依頼

-   PR を開いたときには、コマンドからどの自動レビュアーにも PR レビューを頼まない。ただし、Sourcery AI の GitHub App 側の自動の PR レビューは有効のまま運用する。
    -   理由: Codex の契約を終えたためであり（[0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-002）、Sourcery AI はすでに自動の PR レビューが動いているため、そのまま活かして二重依頼を避けるためである（[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-004）。
    -   正本: [0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-002、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-004。
-   手動の PR レビューは、利用者がマージの質問で「先に PR レビューを依頼」を選び、続く依頼先を選ぶ単一選択の問いで指定したレビュアーに 1 件だけ頼む（CodeRabbit には `@coderabbitai review`、Sourcery AI には `@sourcery-ai review`）。利用者が手で投稿した `@sourceryai review` も Sourcery AI への手動の依頼として認識する。
    -   理由: 信頼の境界を変える PR などで外部の目を足す手段を残しつつ、CodeRabbit と Sourcery AI のどちらにも `/runpr` から依頼できるようにするためである。
    -   正本: [0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-004、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-004、[.claude/commands/_context.md](../../../.claude/commands/_context.md)。
    -   **食い違い:** `_context.md` と `runpr.md` はまだ CodeRabbit のみを手動の依頼の対象としている。この食い違いは https://github.com/isseis/yt2column/issues/1095 の実装で解消する。
-   CodeRabbit の自動の PR レビューは止めたままにする。
    -   内容: `.coderabbit.yaml` の `reviews.auto_review.enabled` を `false` にする。
    -   理由: 割り当てを頻繁に使い切り、機能していなかったためである（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
    -   正本: [.coderabbit.yaml](../../../.coderabbit.yaml)。

### ローカルの CLI レビュー

-   PR を開いた後の変更は、ローカルの CLI レビューで見る。
    -   内容: CodeRabbit CLI を使う。指摘の記録は、`CodeRabbit local review:` の行として、修正のコミット（修正がないときはその次のコミット）のメッセージと、セッションの SUMMARY に残す（[ADR-0014](0014-deterministic-checks-in-go.md)）。
    -   理由: PR レビューとは別の割り当てで、変更のたびにレビューできるためである。
    -   正本: 手順と記録の行の形は [local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md)。
-   CodeRabbit CLI が失敗したときは、代わりのレビューを走らせず、見ていない範囲を `Local CLI review: not run` の行に記録して進む。
    -   理由: これまでも、2 つの CLI が両方失敗したときはこの形で記録していたので、新しい仕組みが要らないためである。見ていない範囲は、PR の説明とマージの質問に出る。
    -   正本: [0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-006。
-   PR を開く前に、どのローカルの CLI レビューも見ていない中身のあるコミットを、まとめてもう 1 回ローカルの CLI レビューにかける。
    -   内容: 上位モデルのレビューの直し（[ADR-0042](0042-implementation-model-tiers.md)）、テスト通過の関門（`green gate`）・終わりの確かめ・`CHECKS` の直し、クロスチェックの部分レビュー（[ADR-0050](0050-fable-partial-review.md)）の記録の後のコミットを含む。
    -   理由: これらは、最初の Codex の PR レビューが見ることを理由にかけ直していなかったが、その PR レビューがなくなり、かけないと別系統の CLI レビューが一度も見ないためである。
    -   正本: [0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-007。
-   **例外（push の直前の判定の再チェック）:** `step 2.6` が救済を走らせた後の再チェックでできるコミット（`AFTER_REMEDY` と `CHECKS` の直し）は、かけ直さない。このコミットは、PR を開いた後の PR レビュー（Sourcery AI の自動と、依頼したときの CodeRabbit）と、PR の説明の未レビュー一覧で扱う。
    -   理由: 救済のたびに、ブランチの全体を push の直前に再レビューすることになり、利用上限で失敗しやすいためである（タスク 0114 の実地の確かめ）。
    -   正本: [0114 設計書](../../tasks/0114_fable_record_after_merge/02_architecture.md) の 1.2.2（実装の際に決めたこと）、[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md) の step 2.6。

### PR レビューを勧める判定

-   エスカレーションの判定は、マージの質問で「先に PR レビューを依頼」を推奨にするかだけを決め、自分では PR レビューを頼まない。
    -   範囲: 最後に完了した PR レビュー（CodeRabbit の手動または Sourcery AI の自動・手動の完了の印の和集合から求める `LAST_REVIEWED`）が見たコミットの後とし、完了した PR レビューがなければ PR の全体とする。
    -   推奨にするとき: 変更が信頼の境界（開発ツールのものを含む）・セキュリティ・耐久性に触れるとき。ほかの基準（振る舞いや契約の変更、モジュールの境界をまたぐ変更、要件や設計の決定に触れる変更、新しい仕組み）にだけ当たるときは、その変更のすべてをローカルの CLI レビューが見たと分からなければ推奨にし、見ていれば推奨にしない。判定はハーネスの `high` 役割モデルに固定したワーカーで行う。`high` 役割ワーカーを呼べないハーネス（OpenCode など）のとき、判定が迷ったとき、答えが決まった形でないとき、判定を走らせられなかったときは、安全側に倒して推奨にする（[ADR-0055](0055-harness-independent-commands.md)、[0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md) の AC-07）。
    -   「見た」とは、ローカルの CLI レビューがそのコミットを見たことが、ブランチの記録から分かることである。
    -   理由: CodeRabbit の PR レビューを手動に限る決定に合わせつつ、未レビューの変更に信頼の境界を変える変更などが含まれるときに利用者の目を引くためである。基準を信頼の境界・セキュリティ・耐久性とそれ以外に分けるのは、次の実績があるためである。PR を作成した後に手作業で頼んだ Fable のレビューの指摘を直し、その後に PR 再レビューを頼んだ 7 件のうち、新しい指摘が 0 件だったのは 6 件で、残る 1 件は信頼の境界のロジックを書き換えた PR だった。実績と違う種類の変更にこの分け方を当てはめることは、受け入れる危険とする。
    -   正本: [0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-005、[0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md) の F-001、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-004、基準は [pr-review-escalation.md](../../../.claude/commands/_lib/pr-review-escalation.md)。
    -   **食い違い:** `pr-review-escalation.md` は、まだ Claude Code の `Agent` ツールと `model: "opus"` を直接指定している（https://github.com/isseis/yt2column/issues/1072 の実装で解消する）。また、`runpr.md` の `LAST_REVIEWED` はまだ CodeRabbit の手動の PR レビューのみを起点としている（https://github.com/isseis/yt2column/issues/1095 の実装で解消する）。
-   ローカルの CLI レビューの試行がレビューした範囲を、`Local CLI reviewed:` の記録の行としてブランチに残す。
    -   呼び出し元が指定したときは、ローカルの CLI レビューの手順が、記録の行だけを持つ空のコミットを作る。
    -   範囲のすべてのコミットをレビューしたかどうかは、`origin/main` から取り出す判定のスクリプト `.claude/commands/_lib/review-records.sh` が決める。結果は `ALL` か `PARTIAL` で、スクリプトが失敗したときは `PARTIAL` とする。
    -   空のコミット（親との差分がないコミット）と、計画書の PR の進み具合のチェックボックスの行だけを変えるコミットは、レビューする中身がないので見たとみなす。
    -   理由: 見なかった範囲と指摘だけの記録からは、見たことを読めないためである。判定をセッションやサブエージェントの解釈に任せず、ブランチの変更でも変わらないスクリプトで行うのは、見ていない範囲を見落とさないためである。空のコミットを作るのは、`/fixpr` の Phase 4.5 で指摘がなく直しもないときにも、見たことがブランチに残るようにするためである。
    -   正本: 形と規則は [0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) の 3.3.4〜3.3.5 と 3.6。
-   推奨にするかは、エスカレーションのワーカー／サブエージェントが返す当たった基準の番号と、レビューしたかどうかの判定の結果から、`runpr.md` が決まった規則で決める。
    -   この信頼の境界には、レビューの関門（PR レビューを推奨にするかを決める仕組み）を変える変更を含める。対象は、エスカレーションの基準と手順、`runpr.md` の直近のレビュー済みコミット（`LAST_REVIEWED`）の定義と対応の規則、判定のスクリプトとその呼び方、ローカルの CLI レビューの試行の結果と範囲を決める手順、記録の行の形と読み方、`fixprguard` である。
    -   理由: 推奨にするかの判断までサブエージェントに任せると、見ていない範囲があるのに推奨にしない誤りが残るためである。関門を変える変更を含めるのは、PR を開いた後に関門を緩める変更が入っても、推奨されずに済んでしまわないようにするためである。ほかのコマンドの定義の変更を含めないのは、このリポジトリの PR の多くがコマンドの定義を変えるためである。
    -   正本: [0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 12・13、1.5 の決定 1。

### レビュー済みの判定

-   PR レビューの「レビュー済み」（`LAST_REVIEWED`）は、CodeRabbit（手動）と Sourcery AI（自動・手動）それぞれの完了の印の和集合と CI で判定する。
    -   **CodeRabbit の完了の印:** CodeRabbit の GitHub App（slug が `coderabbitai`）が作る CheckRun `CodeRabbit` のうち、依頼のコメントの後に始まり、`completed`・`success`・`output.title` が `Finished` のもの。示すコミットはその CheckRun の `head_sha` である。`neutral` で `Review rate limited` のものを割り当ての知らせとし、push のときの知らせ（`skipped` で `Review skipped: automatic reviews are disabled`）は数えず、それ以外の終わり方はエラーの知らせとする。
    -   **Sourcery AI の完了の印と自動の PR レビューの扱い:** Sourcery AI の GitHub App（slug が `sourcery-ai`）が作る CheckRun `Sourcery review` のうち、観測で確定した成功完了の形（`COMPLETED`・`SUCCESS`・`title` が `✅ Completed successfully`）に一致するものを完了の印とし、自動実行の完了と手動の依頼への応答の完了の両方を数える。`head` コミット上で自動の PR レビューが進行中のときは、完了するか上限時間（15 分）に達するまで待機する。自動の PR レビューがスキップ・割り当て切れ・失敗・未認識・時間切れになっても `/runpr` は止めず、完了の印に数えないまま要約に状態と URL を示してマージの質問へ進む。`/runpr`（`fixprguard ci-checks`）および `/mergepr`・`/runpr` step 4（`mergepr prepare`）の CI 待機では、共有パッケージ `internal/ghci` を使い、リポジトリの CI チェック（`app.slug == "github-actions"`）のみを成否判定の対象とする。`Sourcery review` や `CodeRabbit` など外部レビュアーの CheckRun が失敗・未完了でも、CI 失敗とみなして処理は止めない。停止するのは、手動の依頼が割り当て切れ・スキップ・失敗・時間切れになったときだけである（`/runpr` の再実行時点で既に終わっていた依頼であれば、要約に示して進む）。
    -   **判定の場所:** `fixprguard` のサブコマンド `review-state` が、GitHub の応答を厳密な JSON として検査し、状態・SHA・URL・時刻だけを出す。
    -   **本文だけの指摘と、レビューしなかったファイル:** 本文を読まずに分からないものは分からないものとして扱い、完了した PR レビューの要約ではレビューの URL と、本文に指摘やレビューしなかったファイルの知らせがありうることを示す。
    -   理由: 観測では完了・割り当て切れ・レビューしたコミットを本文なしに示すのは CheckRun だけで、CheckRun を作れるのはその GitHub App だけである。判定を Go に置くのは `gh api --jq` の重複後勝ちを避けテストで検証するためである（[ADR-0014](0014-deterministic-checks-in-go.md) の 1、[ADR-0015](0015-untrusted-review-threads.md)）。どちらか 1 社の完了で `LAST_REVIEWED` とし、自動の PR レビューのスキップで止めないのは、自動の PR レビューを活かしつつ割り当て切れや上限到達でマージを塞がないためである。`mergepr prepare` 側も `internal/ghci` で CI 待機を外部レビュアーの CheckRun から分離するのは、`/runpr` step 4 で `mergepr prepare` を呼ぶ際に `gh pr checks --watch --fail-fast` が `Sourcery review` の失敗で停止し、未完了では完了まで待ち続けてしまうためである（[0122 設計書](../../tasks/0122_sourcery_trusted_reviewer/02_architecture.md)「設計の際に決めたこと（2026-11-24）」）。
    -   正本: [0118 設計書](../../tasks/0118_codex_removal/02_architecture.md) の 1.3・3.3、[0118 の観測の記録](../../tasks/0118_codex_removal/observations.md)、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-001・F-004、[0122 設計書](../../tasks/0122_sourcery_trusted_reviewer/02_architecture.md) の 1.3・3.2.4。
    -   **食い違い:** `fixprguard review-state` と `runpr.md` はまだ CodeRabbit 単体（手動のみ）を判定している。また、`runpr.md` の CI 待機および `mergepr prepare`（`internal/mergepr/mergepr.go`・`.claude/commands/mergepr.md`）もまだ `gh pr checks` を使っており、外部レビュアーの CheckRun をリポジトリの CI チェックから分離していない。この食い違いは https://github.com/isseis/yt2column/issues/1095 の実装で解消する。
-   自動レビュアーの投稿は信頼しない入力として読む。
    -   内容: 完了の印は、GitHub App の slug が対象レビュアー（`coderabbitai`・`sourcery-ai`）の CheckRun だけを数え、それが示すコミットとその祖先だけをレビュー済みにする。手動の依頼のコメントは著者の種類が `User` で著者名が利用者のものだけを数える。観測した形に合わないときは完了としない。CheckRun は PR のコミット一覧にあるコミットのものだけを読み、読み取りの途中で head が変わったときは一覧を捨てて読み直すので、示すコミットはかならず出力する head かその祖先である。
    -   理由: 形を直したり読み替えたりして完了とすると、レビューされていない変更をレビュー済みとしうるためである。
    -   正本: [0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-004、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-004。
-   PR レビューのコメントの本文は、セッションに出さない。
    -   出すもの: 形に合うかの真偽値、16 進と確かめた短い SHA、URL、時刻など、判定に要る決まった値だけ。
    -   理由: 信頼しないテキストの扱い（[ADR-0015](0015-untrusted-review-threads.md)）を保つためである。
    -   正本: [0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の 4.2、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の 3.2。
-   信頼する自動レビュアーの一覧と、PR レビューの待機・依頼の対象は役割を分けて定める。
    -   内容: 一覧は [.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Automated reviewer list」に 1 か所だけ置き、各名前は GraphQL の `author.login` の形（`[bot]` を付けない）で持つ。PR レビューの待機・依頼の対象と依頼文面は `_context.md` の別項目で定め、REST の `user.login` の形はその名前に `[bot]` を付けて作る。一覧は「誰のスレッドを処理するか」（[ADR-0015](0015-untrusted-review-threads.md)）を決め、PR レビューの待機・依頼の対象は「何をもってレビュー済みとし、誰に手動の依頼をするか」を決める。
    -   理由: 名前を一覧にすると、どの名前に `[bot]` を付け、どの依頼文面で誰を待つかが一覧だけでは決まらないためである。
    -   正本: [0033 要件定義書](../../tasks/0033_multiple_trusted_reviewers/01_requirements.md) の「自動レビュアーの一覧と完了を待つ PR レビュアーを分ける」、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-004。
    -   **食い違い:** `_context.md` の「Awaited PR reviewer」と「PR review request」はまだ単一の `coderabbitai` のみである。この食い違いは https://github.com/isseis/yt2column/issues/1095 の実装で解消する。

## 採らなかった案

採らなかった案の多くは、割り当てを余分に使うか、レビュー済みの判定や見たかの判定を予測できない・解釈に任せるものになるために退けた。

### PR レビューの依頼

-   CodeRabbit の PR レビューのままにする、または Codex と並べて比べてから切り替える。
    -   理由: CodeRabbit の PR レビューは割り当てを頻繁に使い切り、機能していなかった。同じ PR で両方を待つことはできないので、比較の期間を設けられない（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   push のたびに PR レビューを走らせる、またはスマート検出を使う。
    -   理由: push のたびのレビューは割り当てを消費する。スマート検出は試験的な機能で、いつ走るかを `/runpr` から予測できず、レビュー済みの判定を組めない（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   Codex の「徹底的なコードレビュー」を有効にする。
    -   理由: 指摘が尽きるまで探し続けるので、`/fixpr` の方針で止める設計と合わず、待つ時間と割り当てが増える（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1、§5）。
-   Codex のレビューが届かないときに CodeRabbit へ自動で切り替える。
    -   理由: CodeRabbit の PR レビューは機能しておらず、2 つの判定の方法を保守することにもなる（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。

### ローカルの CLI レビュー

-   ローカルの CLI レビューも Codex に切り替える。
    -   理由: CLI の割り当ては PR レビューと別であり、CodeRabbit CLI の `--base-commit` や構造化出力など、今の手順がそのまま使える（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   Codex CLI を Claude Code の Bash 権限の前方一致で直接実行する。
    -   理由: `Bash(codex review *)` のような許可は、後ろに `--dangerously-bypass-approvals-and-sandbox` を付けた実行まで許す。作業ツリーにスクリプトを置く方法も、レビュー対象のブランチのコミットで中身を書き換えられる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   範囲を一時 worktree で実現する。
    -   理由: 後片付けと指摘のパスの読み替えが要るうえ、Codex は作業ディレクトリの外も読めるので、秘密のファイルから遠ざける利点も薄い（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   Codex のセッションの記録にある構造化された `review_output` を読む。
    -   理由: 公開された仕様ではなく、実行ごとに 2 つできるファイルから 1 つを選ぶ手順が要る（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   Codex にモデルと推論の強さを指定する。
    -   理由: モデルの名前をツールに書くと、名前が変わるたびにツールを直すことになる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   `--ephemeral` を付ける。
    -   理由: 観測しておらず、利用者が普段 Codex を使うときにも残る記録が、本タスクで新たに増えるリスクではない（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   Codex にレビューの指示（PROMPT）やプロジェクトの規則の指定を渡す。
    -   理由: Codex は指示しなくても `AGENTS.md` と `CLAUDE.md` を読み、`AGENTS.md` の内容をレビューの指示に入れる。CodeRabbit CLI の `-c CLAUDE.md` が担う役割は、これで足りる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   `codexreview` の比較に、ローカルのすべてのブランチとタグ、stash の一覧も含める。
    -   理由: 0049 で採った形である。ほかの worktree の操作でも止まり、利用者の判断を待つか、やり直して Codex の割り当てをもう 1 回使うことになる。止まらない CodeRabbit CLI の側とも揃わない（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「背景」と「issue #778 で決めたこと」）。
-   `codexreview` と `/mergepr` の排他。
    -   理由: 2 つのツールにまたがる約束が増え、ほかのセッションの commit による停止は減らない（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「issue #778 で決めたこと」）。
-   `/mergepr` の形の変化（`refs/remotes/origin/*` への fast-forward とブランチの削除）だけを許す。
    -   理由: ブランチの削除を見逃すと、push していない commit の消失に気づけない（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「issue #778 で決めたこと」）。
-   ほかの worktree の ref の変化なら、自動でやり直す。
    -   理由: Codex の割り当てを余分に使う（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「issue #778 で決めたこと」）。
-   比べなくなったものを結果から外し（`changed_refs` を削り）、`--protocol` を 2 に上げる。
    -   理由: 入れ直しの前後で番号の合わない組み合わせの間は、Codex CLI のフォールバックが働かなくなる（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「要件定義の際に決めたこと（2026-11-10）」）。

### PR レビューの追加の依頼

-   PR レビューを追加で自動で頼む基準に、振る舞いや契約の変更・モジュールの境界・要件や設計の決定・新しい仕組みを入れたままにする。
    -   理由: Fable の部分レビューの修正の多くがこれらに当たり、割り当ての節約にならない（[0098 要件定義書](../../tasks/0098_fable_partial_review/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   見た範囲の記録を足さずに、見たかを判定する。
    -   理由: 今の記録は見なかった範囲と指摘しか残さないので、判定はいつも「すべては見ていない」になる（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）。
-   見たかの突き合わせを、セッションが手順の文に従って行う。
    -   理由: 判定が解釈に左右され、見ていない範囲を見落としうる（同上）。
-   見たかの判定の値を Escalation のサブエージェントに渡し、頼むかをサブエージェントに決めさせる。
    -   理由: 見ていない範囲があるのに頼まない誤りが残り、サブエージェントの答え方の定義とも食い違う（同上）。
-   レビューの関門を変える変更を信頼の境界に含めない、または `.claude/` の全体を含める。
    -   理由: 前者は関門を緩める変更が頼まれずに済みうる。後者は割り当てを多く使う（同上）。

### レビュー済みの判定（結果のない完了）

以下の出典は、どれも [0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md) である。

-   受領コメントが決まった形に合えば、利用者に尋ねずに完了とする。
    -   理由: 理由は記録されていない（「issue #1005 で決めたこと」）。
-   受領コメントの 3 行（1 行目・結果の行・`Reviewed commit:` の行）をそのまま絞り込んでセッションに出す。
    -   理由: 本文の一部がエージェントに渡り、ADR-0015 の更新と、本文を出さないことを確かめるガードテストの例外が要る（「要件定義の際に決めたこと（2026-11-19）」。以下同じ）。
-   受領コメントの指摘ありの形も受け入れ、スレッドの処理に回す。
    -   理由: 指摘ありなのに `has_any_findings` が `null` という状態は、理由がいっそう分からない。
-   受領コメントの結果の行を、観測した文とちょうど一致するときだけ受け入れる。
    -   理由: `Didn't find any major issues.` の後ろの文が観測した 2 件で違い、Codex が後ろの文を変えるたびに止まる。
-   受領コメントが 2 件以上あるときに、最新の 1 件を使う。
    -   理由: 要約コメントが 2 件以上あるときに選ばずに止める規則と合わない。
-   受領コメントがまだなければすぐ止める。
    -   理由: 受領コメントが要約コメントの更新より遅れて投稿されると、その間のポーリングで止まる。
-   形に合う受領コメントも、今までどおり想定外の投稿に数える。
    -   理由: 受領コメントが要約コメントの完了より先に届くと止まり、再実行が依頼を投稿し直して割り当てを使う。
-   形に合わない受領コメントも、想定外の投稿に数えない。
    -   理由: 観測していない上限やエラーの知らせが受領コメントの印を持つと、URL を示されずに期限まで待って止まる。
-   利用者が認めた完了を、squash コミットや PR へのコメントに記録する。
    -   理由: 前者は `mergepr.md` の変更が要り、後者は GitHub への書き込みが増える。
-   利用者が認めた完了を LAST_REVIEWED に含めない。
    -   理由: LAST_REVIEWED が「なし」になり、マージの質問が PR レビューの依頼を勧めて、割り当てを余分に使う。
-   承認を Completed と同じに扱い、要約コメント・受領コメントが変わらない間は判定し直さない。
    -   理由: 新しい依頼の後に前の承認を使い回しうるうえ、承認の直後に届いたコメントで尋ねる条件が崩れても止まらない。

### Codex の撤去（0118）

以下の出典は、どれも [0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の「要件定義の際に決めたこと（2026-11-20）」である。

-   Codex の PR レビューと Codex CLI を続ける。
    -   理由: 利用者が Codex の契約を終えることにしたためである。47 件の PR で、Codex の PR レビューの本物の指摘は 3 件だった。
-   PR を開いた後の外部のレビューをなくし、手動の依頼も設けない。
    -   理由: 信頼の境界を変える PR などで、外部の目を足す手段を残すことを利用者が選んだためである。
-   CodeRabbit の自動 PR レビューを有効に戻し、PR を開いたときの 1 回を CodeRabbit に移す。
    -   理由: 割り当てを頻繁に使い切り、機能しなかった実績があるためである。
-   0031 より前の CodeRabbit の判定（`StatusContext` と「between」の行）を、観測せずにそのまま使う。
    -   理由: 利用者が PR #404 で、割り当てを使い切っても CheckRun が `pass` になるのを見ており（リポジトリに記録はない）、形が変わっていれば `/runpr` が止まるか、誤ってレビュー済みとしうるためである。
-   CodeRabbit CLI が失敗したときに、Opus のサブエージェントのレビューを代わりに走らせる。
    -   理由: [#1057](https://github.com/isseis/yt2column/issues/1057) の Fable から Opus への置き換えと重なるためである。
-   CodeRabbit CLI が失敗したときに、止めて利用者に聞く。
    -   理由: 人を待たせずに進め、見ていない範囲を PR の説明とマージの質問で示せば足りるためである。
-   エスカレーションの判定を撤去する。
    -   理由: どの PR でも PR レビューを推奨しなくなり、利用者の判断の材料が減るためである。

### Sourcery AI の併用（0122）

以下の出典は、個別に明記したものを除き、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の「要件定義の際に決めたこと（2026-11-23）」である。

-   Sourcery AI を信頼する自動レビュアーの一覧にだけ加え、`/runpr` の手動の PR レビュー依頼・完了待機の対象には含めない。
    -   理由: 利用者が `/runpr` からも Sourcery AI の完了待機と手動の依頼を行えるようにすることを選んだためである。
-   手動の PR レビューの対象を CodeRabbit から Sourcery AI に切り替え、併用しない。
    -   理由: 信頼の境界の変更などで CodeRabbit と Sourcery AI の両方を使い分けられるようにするためである。
-   Sourcery AI の自動の PR レビューを無効化する、または自動実行の完了を `LAST_REVIEWED` に数えない。
    -   理由: すでに動いている Sourcery AI の自動の PR レビューを活かし、二重依頼を避けるためである。
-   CodeRabbit と Sourcery AI の両方が完了したコミットだけを `LAST_REVIEWED` とする。
    -   理由: 自動の PR レビューのない CodeRabbit が未実行の間は常に未レビュー扱いになり、日常の PR 運用が重くなるためである。
-   自動の PR レビューがスキップ・失敗・時間切れになったときも `/runpr` を止める。
    -   理由: 自動の PR レビューの週間枠や再レビュー上限でマージが塞がれるのを防ぐためである。
-   CI 待機と外部レビュアーの CheckRun の分離を `runpr.md` step 3 だけにとどめて `mergepr prepare` は `gh pr checks --watch --fail-fast` のままにする、または CI 待機を `gh pr checks --json` や `--required` で済ませる。
    -   理由: `mergepr prepare` で `gh pr checks` を使い続けると、`Sourcery review` が失敗のままマージへ進んだときに step 4 で止まり、未完了のままでは step 4 で待ち続ける。また、`main` の ruleset には必須チェック設定がないため `--required` は使えず、`gh pr checks --json` は `app.slug` を返さず、`headRefOid` の一致確認、ジョブ生成前の `CheckSuite` 状態の取得、重複 JSON キーの拒否も満たせない（[0122 設計書](../../tasks/0122_sourcery_trusted_reviewer/02_architecture.md)「設計の際に決めたこと（2026-11-24）」）。

## 結果

PR レビューは CodeRabbit（手動）と Sourcery AI（自動・手動）を併用し、PR を開いた後の変更はローカルの CLI レビュー（CodeRabbit CLI）でも見る。受け入れた危険は下の「受け入れた危険」にまとめる。

### レビューの回数と送るもの

-   コマンドから PR を開いたときの PR レビュー依頼はないが、Sourcery AI の GitHub App 側の自動の PR レビューは PR 作成・push 時に自動で動作し、`/runpr` は進行中のレビューが終わるのを待って完了を `LAST_REVIEWED` に数える。手動の PR レビューは、利用者がマージの質問から選んだときだけ CodeRabbit または Sourcery AI に頼む。
-   CodeRabbit の PR レビューの割り当ては時間あたりの回数で決まり、手動の依頼も数える。手動の依頼が割り当てを使い切ったときはレビュー済みとせず、`/runpr` が止まる。Sourcery AI の自動の PR レビューが上限などでスキップされても `/runpr` は止めず、要約に状態を示してマージの質問へ進む。
-   ローカルの CLI レビューの対象の差分は、CodeRabbit のサービスへ送られる。OpenAI のサービスへは送らない。

### 費用と依存

-   見た範囲を記録のコミットでブランチに残すので、`/fixpr` の回ごとに記録のコミットを push し、CI がもう 1 回走る。この費用は受け入れる（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 21）。
-   PR レビューの「レビュー済み」の判定は、CodeRabbit と Sourcery AI の投稿・CheckRun の形に依存する。形が変わったときは、手動の依頼では `/runpr` が止まり、自動の PR レビューでは完了に数えられないので、判定を直す。
-   完了した PR レビューがない PR では、エスカレーションの判定は PR の全体を見る。

### 受け入れた危険

-   クロスチェックの部分レビューが走らない PR（OpenCode で実装した PR、単独の PR）では、外部のレビューが CodeRabbit CLI だけになる。
    -   #1057 の集計では、Codex の PR レビューの本物の指摘 3 件のうち 2 件が、この種類の PR だった。
    -   補う仕組みは、[#1057](https://github.com/isseis/yt2column/issues/1057) の並走の結果を見てから、別のタスクで決める（[0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md)「要件定義の際に決めたこと（2026-11-20）」）。
-   CodeRabbit CLI が失敗した範囲は、実装したハーネスのモデルとは別系統の CLI レビューを一度も受けずにマージされうる。その範囲は PR の説明とマージの質問に示し、利用者は手動の PR レビューを選べる（同上）。
-   見たかどうかは、ローカルの CLI レビューがその範囲をレビューしたかどうかだけで決める。次の 2 つの危険は、どちらも受け入れる（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 14、5.2）。
    -   `unclear`（要確認）や `deferred`（先送り）の指摘が残っていても、ほかの基準にだけ当たる変更では PR レビューを推奨しないことがある。
    -   見た範囲の記録の行はレビューを受ける側のセッションが書くので、レビューしていない範囲を形の正しい行で書けば、推奨されずに済んでしまう。

## 経緯

- 2026-10-21 PR レビューを CodeRabbit の GitHub 連携から Codex に切り替え、PR を開いたときの 1 回だけにした。切り替えの前に振る舞いを観測して記録し、その結果でレビュー済みの判定を決めた。CodeRabbit の自動 PR レビューを止め、ローカルの CLI レビューは CodeRabbit のまま残した（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md)）
- 2026-10-28 ローカルの CLI レビューで CodeRabbit CLI が失敗したときに、Codex CLI を `codexreview` 越しに使うフォールバックを加えた。指摘の記録の行を CLI ごとに分け、異系統のレビューを受けていない範囲を示す行を加えた（[0049 要件定義書](../../tasks/0049_codex_local_review/01_requirements.md)）
- 2026-11-10 `codexreview` が Codex CLI の前後で比べるものを、この worktree の `git status`、HEAD、HEAD のブランチの ref に絞った。0049 で比べていたほかのブランチ、タグ、stash は比べないことにし、結果の形と `--protocol` の番号は変えないことにした（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md)）
- 2026-11-17 `/runpr` が PR レビューを追加で自動で頼むのを、信頼の境界・セキュリティ・耐久性に触れる変更だけに絞った。ほかの基準に当たる変更は、ローカルの CLI レビューがすべて見ていれば頼まず、見ていない範囲があれば利用者に尋ねることにした（[0098 要件定義書](../../tasks/0098_fable_partial_review/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）
- 2026-11-17 ローカルの CLI レビューが見た範囲を記録の行に残し、Codex の最後の PR レビューの後の commit をすべて見たかを、`origin/main` の判定のスクリプトで決めることにした。自動で頼むかを決まった規則で決め、レビューの関門を変える変更を信頼の境界に含めた（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）
- 2026-11-18 信頼の境界に含めるレビューの関門に、ローカルの CLI レビューの試行の結果と範囲を決める手順、`runpr.md` の LAST_REVIEWED の定義、Escalation の手順を足した（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md)「実装の際に決めたこと（2026-11-18）」）
- 2026-11-19 要約コメントが `completed` で `has_any_findings` が `null` のときの扱いを決めた。受領コメントの決まった形を本文を出さずに判定して利用者に確かめ、利用者が認めたときだけ、その実行の中で完了とみなして LAST_REVIEWED に含める。形に合う受領コメントは想定外の投稿に数えない（[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md)「issue #1005 で決めたこと」「要件定義の際に決めたこと（2026-11-19）」）
- 2026-11-19 受領コメントの本文をセッションに出さない決まりの範囲を、一覧の読み取りとした。形に合わない受領コメントで想定外の投稿として止まったときの報告は、これまでの想定外の投稿と同じく本文を読んで要約する（[0108 設計書](../../tasks/0108_codex_null_findings_receipt/02_architecture.md)「設計の際に決めたこと（2026-11-19）」）
- 2026-11-20 Codex の PR レビューと Codex CLI をレビューの体制から外すことにした。PR レビューは、利用者がマージの質問で選んだときだけ CodeRabbit に手動で頼む。エスカレーションの判定は推奨を決めるだけにし、CodeRabbit CLI が失敗したら `not run` を記録して進む。PR を開く前に、まだ見ていない commit をまとめてもう 1 回ローカルの CLI レビューにかける。要約コメントの待機と受領コメントによる確認は撤去する（[0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md)「要件定義の際に決めたこと（2026-11-20）」）
- 2026-11-20 手動の PR レビューの完了の印を、CodeRabbit の GitHub App が作る CheckRun `CodeRabbit` の `completed`・`success`・`Finished` とし、判定を `fixprguard review-state` に置いた。本文だけの指摘とレビューしなかったファイルは分からないものとして扱い、マージの質問でありうることを示すことにした（[0118 設計書](../../tasks/0118_codex_removal/02_architecture.md)「設計の際に決めたこと（2026-11-20）」）
- 2026-11-21 エスカレーションの判定を Opus 固定からハーネスの `high` 役割ワーカー固定（呼べないハーネスでは安全側の推奨）へ改め、上位モデルのレビューとクロスチェックの部分レビューの名称変更に追随した（[0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md)「要件定義の際に決めたこと（2026-11-21）」）
- 2026-11-23 PR レビューに Sourcery AI（自動・手動）を加え、CodeRabbit（手動）と併用することにした。Sourcery AI の自動の PR レビューは有効のまま維持し、進行中のレビューの完了を待って完了判定（`LAST_REVIEWED` は両社の和集合）に含める。自動の PR レビューのスキップ・失敗・時間切れでは止めず、手動の依頼の失敗時だけ止める（[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md)「要件定義の際に決めたこと（2026-11-23）」）
- 2026-11-24 リポジトリの CI チェック（`app.slug == "github-actions"`）のみの成否を判定する共有パッケージ `internal/ghci` を設け、`/runpr` step 3（`fixprguard ci-checks`）だけでなく `mergepr prepare`（`/mergepr`・`/runpr` step 4）の CI 待機も外部レビュアーの CheckRun から分離することにした（[0122 設計書](../../tasks/0122_sourcery_trusted_reviewer/02_architecture.md)「設計の際に決めたこと（2026-11-24）」）
- 2026-11-24 push の直前の判定（`step 2.6`）が救済を走らせた後の再チェックでできる commit は、ローカルの CLI レビューにかけ直さない例外を足した（[0114 要件定義書](../../tasks/0114_fable_record_after_merge/01_requirements.md)）
