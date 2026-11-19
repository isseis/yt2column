# ADR-0013: PR レビューは作成時の 1 回だけ Codex に頼み、その後の変更はローカルの CLI レビューで見る

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

このリポジトリは公開されており、GitHub のアカウントを持つ人なら誰でも PR を開ける。PR のレビューは、人が書くものではなく、外部のサービスの Bot が書くものを待つ。

レビューには 2 つの経路がある。1 つは PR そのものに対するレビュー（PR レビュー）で、GitHub の PR に投稿される。もう 1 つは、PR を開いたブランチの手元で行うローカルの CLI レビューで、変更のたびに実行できる（[.claude/commands/_lib/local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md)）。どちらも外部のサービスの割り当て（利用できる回数）を使う。割り当てはサービスの契約ごとに別で、量は公表されていないものもある。

## 決定

-   **PR レビューは Codex（`chatgpt-codex-connector`）が行う。** 依頼は、PR を開いたときの自動レビューを待ち、約 2 分たっても始まらなければ `@codex review` をコメントして 1 回だけ頼む。push のたびの自動レビューは使わない。割り当てが利用者のほかの ChatGPT の利用と共有で、量が公表されていないためである。規則は [CLAUDE.md](../../../CLAUDE.md) の「Claude Commands」と [.claude/commands/_context.md](../../../.claude/commands/_context.md) の「PR review request」の行、[.claude/commands/_lib/task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md) の Part B step 5 にある。
-   **CodeRabbit の自動 PR レビューは止める。** `.coderabbit.yaml` の `reviews.auto_review.enabled` を `false` にする。同じ PR を 2 つのサービスがレビューすると、両方の割り当てを 1 回のレビューに使うためである。規則は [.coderabbit.yaml](../../../.coderabbit.yaml) にある。
-   **PR を開いた後の変更は、ローカルの CLI レビューで見る。** CodeRabbit CLI を使い、それが失敗したときは Codex CLI を `codexreview` ツール越しに使う。指摘の記録は、CLI ごとに分けた `CodeRabbit local review:`・`Codex local review:` の行として、修正のコミット（修正がないときはその次のコミット）のメッセージと、セッションの SUMMARY に残す（[ADR-0014](0014-deterministic-checks-in-go.md)）。PR レビューとは別の割り当てで、変更のたびにレビューできるためである。手順と記録の行の形は [local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md) にある。
-   **`codexreview` は、Codex CLI の実行の前後で、レビューする worktree の `git status`、HEAD（HEAD がいるブランチの名前と、HEAD が指す commit）、HEAD のブランチの ref だけを比べ、違えば `state_changed` で止める。** ほかのブランチ、タグ、stash は比べない。これらは同じリポジトリのすべての worktree が共有するので、比べると、並行する `/mergepr` の後片付けやほかのセッションの commit でも止まるためである。残す 3 つは、レビューの範囲と、手順が後で作る commit の内容を Codex に変えさせない、という目的に直接関わる。比べるものは、[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「F-001: 比べるものの範囲」が正本であり、処理は [internal/codexreview](../../../internal/codexreview) にある。共通手順は [local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md) の手順 3 で `state_changed` を `stop` として扱う。
-   **比べるものを絞っても、`codexreview` の結果の形と `--protocol` の番号は変えない。** 比べなくなったものが結果に現れなくなるだけで、`changed_refs` には HEAD のブランチの ref だけが入りうる。番号を上げると、入れ直したツールを古い文書のブランチが呼ぶ間と、古いツールを新しい文書が呼ぶ間は、Codex CLI のフォールバックが働かず、記録だけになるためである（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「F-002: 結果に現れる値」）。
-   **PR レビューを追加で頼むのは、ローカルの CLI レビューで代われない種類の変更のときと、利用者が求めるときだけにする。** 前者の判断は [pr-review-escalation.md](../../../.claude/commands/_lib/pr-review-escalation.md) に従う。Codex CLI のフォールバックは PR レビューと同じ割り当てを使うので、フォールバックを実行すること自体が割り当てを使う。規則は [CLAUDE.md](../../../CLAUDE.md) の「Commands」にある。
-   **`/runpr` が PR レビューを追加で自動で頼むのは、最後の PR レビューの後の変更が、信頼の境界（開発用のツールのものを含む）・セキュリティ・耐久性に触れるときだけにする。振る舞いや契約の変更、モジュールの境界をまたぐ変更、要件や設計の決定に触れる変更、新しい仕組みは、その変更のすべてをローカルの CLI レビューが見ていれば頼まず、見ていない範囲があれば利用者に尋ねる。** PR を作成した後に手作業で頼んだ Fable のレビューの指摘を直し、その後に頼んだ PR 再レビューは、7 件中 6 件で新しい指摘が 0 件で、例外は信頼の境界のロジックを書き換えた PR だったためである。この実績は Fable の指摘の修正についてのもので、この条件が今扱うのは主に `/fixpr` による Codex の指摘の修正である（Fable の部分レビューは PR を開く前に移した。[ADR-0050](0050-fable-partial-review.md)）。実績と違う種類の変更にこの条件を当てはめることは、受け入れる危険とする。「見た」とは、ローカルの CLI レビューがその commit を見たことが、ブランチの記録から分かることである。分からない commit があれば利用者に尋ねる。仕様は [0098 要件定義書](../../tasks/0098_fable_partial_review/01_requirements.md) の F-007 にある。
-   **ローカルの CLI レビューの試行がレビューした範囲を、`Local CLI reviewed:` の記録の行としてブランチに残す。呼び出し元が指定したときは、ローカルの CLI レビューの手順が、記録の行だけを持つ空の commit を作る。Codex の最後の PR レビューの後の commit をすべてレビューしたかどうかは、`origin/main` から取り出す判定のスクリプト `.claude/commands/_lib/review-records.sh` が決める。結果は `ALL` か `PARTIAL` で、スクリプトが失敗したときは `PARTIAL` とする。** 見なかった範囲と指摘だけの記録からは、見たことを読めないためである。判定をセッションやサブエージェントの解釈に任せず、ブランチの変更でも変わらないスクリプトで行うのは、見ていない範囲を見落とさないためである。空の commit を作るのは、`/fixpr` の Phase 4.5 で指摘がなく直しもないときにも、見たことがブランチに残るようにするためである。空の commit（親との差分がない commit）と、計画書の PR の進み具合のチェックボックスの行だけを変える commit は、レビューする中身がないので見たとみなす。形と規則は [0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) の 3.3.4〜3.3.5 と 3.6 にある。
-   **`/runpr` が追加の PR レビューを自動で頼むかどうかは、Escalation のサブエージェントが返す当たった基準の番号と、レビューしたかどうかの判定の結果から、`runpr.md` が決まった規則で決める。セキュリティと耐久性の基準を、信頼の境界・セキュリティ・耐久性に触れる変更に広げ、それに当たれば頼み、ほかの基準だけなら、すべて見ていれば頼まず、見ていなければ利用者に尋ねる。この信頼の境界には、レビューの関門（PR レビューを追加で頼むかを決める仕組み。Escalation の基準と手順、`runpr.md` の LAST_REVIEWED の定義と対応の規則、判定のスクリプトとその呼び方、ローカルの CLI レビューの試行の結果と範囲を決める手順、記録の行の形と読み方、`fixprguard`）を変える変更を含める。** 頼むかどうかの判断までサブエージェントに任せると、見ていない範囲があるのに頼まない誤りが残るためである。関門を変える変更を含めるのは、PR を開いた後に関門を緩める変更が入っても、頼まずに済んでしまわないようにするためである。ほかのコマンドの定義の変更を含めないのは、このリポジトリの PR の多くがコマンドの定義を変え、含めると割り当てを多く使うためである（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 12・13、1.5 の決定 1）。
-   **「レビュー済み」は、Codex の要約コメントと CI で判定する。** `/runpr` は、1 行目が `<!-- codex-pull-request-review-summary -->` の要約コメントを待ち、その状態の行が示すコミットを見る。待ち時間と読む値の形は [.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 3 に従う。レビューの完了を、投稿されたコメントの形から決めるためである。
-   **要約コメントが `completed` で `has_any_findings` が `null`（結果のない完了）のときは、Codex の受領コメント（1 行目が `<!-- codex-review-receipt:v1 -->`）を使って利用者に確かめる。受領コメントが 1 件だけで、指摘なしの決まった形に合い、レビューしたコミットが要約コメントのコミットで依頼したコミットであるときだけ、`/runpr` は利用者に尋ね、利用者が「指摘なしとして進む」を選んだときだけ、そのコミットのレビューを完了とみなす。受領コメントが依頼から約 15 分たっても来ないとき、2 件以上あるとき、形が合わないときは、今までどおり判定できない状態として止まる。決まった形に合う受領コメントは、想定外の投稿に数えない。** 結果のない完了でも、レビューの結果が受領コメントにあった例（PR #998）があるためである。利用者に尋ねる理由は記録されていない。形に合う受領コメントを想定外の投稿に数えないのは、受領コメントは `has_any_findings` が真偽値のレビューでも投稿され、要約コメントの完了より先に届くと、`/runpr` が止まってしまうためである。形に合わないものを数えたままにするのは、観測していない上限やエラーの知らせを止めて示すためである。仕様は [.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 3 と、[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md) の F-001〜F-004 にある。
-   **受領コメントの形は `--jq` の絞り込みの中で判定し、セッションには、形に合うかの真偽値と、16 進と確かめた短い SHA、URL、時刻だけを出す。本文の文字列は出さない。利用者は URL から GitHub で本文を確かめる。** 要約コメントの最初の 2 行のほかは本文をセッションに出さない、という原則を保つためである（[ADR-0015](0015-untrusted-review-threads.md)）。絞り込みが本文を出さないことは、信頼の境界で実行するコマンドとして、ガードテストで確かめる。仕様は [.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 3 と、[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md) の F-002・F-006 にある。
-   **利用者が認めた完了は、その実行の中だけで完了の印として扱い、LAST_REVIEWED を決める完了の印に含める。承認は、承認したときの最新の依頼と、要約コメント・受領コメントに結び付け、それらが変わるか、尋ねる条件が崩れたら効かなくなる。永続的には記録せず、その実行の報告と step 4 のマージの質問に示すだけにする。再実行では、もう一度利用者に尋ねる。** 受領コメントがレビューしたコミットを示しているので、含めないと PR レビューの依頼を余分に勧めて割り当てを使うためである。承認が効かなくなる条件を広く取るのは、承認の根拠が後のコメントで崩れうるので、止まる側に倒すためである。記録を足さないのは、要約コメントと受領コメントが GitHub に残り、マージは利用者がマージの質問に答えて決めるためである。仕様は [.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 3 と、[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md) の F-005 にある。
-   **食い違い:** `runpr.md` の step 3 は、まだ結果のない完了で受領コメントを確かめずに止まり、要約コメントの完了より先に届いた受領コメントを想定外の投稿に数える。この ADR の決定に合っていない（[#1005](https://github.com/isseis/yt2column/issues/1005)）。
-   **Codex のモデル・推論の強さ・徹底的なレビューは、リポジトリの外の利用者の設定で決め、リポジトリでは指定しない。** 名前が変わってもツールや文書を直さずに済み、指摘が尽きるまで探し続ける設定も避けられるためである。規則は [CLAUDE.md](../../../CLAUDE.md) の「Commands」にある。
-   **信頼する自動レビュアーの一覧と、完了を待つ PR レビュアーは別の値にする。** 一覧は [.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Automated reviewer list」に 1 か所だけ置き、各名前は GraphQL の `author.login` の形（`[bot]` を付けない）で持つ。完了を待つレビュアーは「Awaited PR reviewer」の行が決め、REST の `user.login` の形はその名前に `[bot]` を付けて作る。一覧は「誰のスレッドを処理するか」（[ADR-0015](0015-untrusted-review-threads.md)）を決め、完了を待つレビュアーは「何をもってレビュー済みとするか」を決める。名前を一覧にすると、どの名前に `[bot]` を付けるかが決まらなくなるためである。規則は [0033 要件定義書](../../tasks/0033_multiple_trusted_reviewers/01_requirements.md) の「自動レビュアーの一覧と完了を待つ PR レビュアーを分ける」にある。

## 採らなかった案

-   **CodeRabbit の PR レビューのままにする、または Codex と並べて比べてから切り替える。** CodeRabbit の PR レビューは時間あたりの上限に頻繁に当たり、機能していなかった。同じ PR で両方を待つことはできないので、比較の期間を設けられない（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   **push のたびに PR レビューを走らせる、またはスマート検出を使う。** push のたびのレビューは割り当てを消費する。スマート検出は試験的な機能で、いつ走るかを `/runpr` から予測できず、レビュー済みの判定を組めない（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   **Codex の「徹底的なコードレビュー」を有効にする。** 指摘が尽きるまで探し続けるので、`/fixpr` の方針で止める設計と合わず、待つ時間と割り当てが増える（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1、§5）。
-   **PR レビューを追加で自動で頼む基準に、振る舞いや契約の変更・モジュールの境界・要件や設計の決定・新しい仕組みを入れたままにする。** Fable の部分レビューの修正の多くがこれらに当たり、割り当ての節約にならない（[0098 要件定義書](../../tasks/0098_fable_partial_review/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **見た範囲の記録を足さずに、見たかを判定する。** 今の記録は見なかった範囲と指摘しか残さないので、判定はいつも「すべては見ていない」になる（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）。
-   **見たかの突き合わせを、セッションが手順の文に従って行う。** 判定が解釈に左右され、見ていない範囲を見落としうる（同上）。
-   **見たかの判定の値を Escalation のサブエージェントに渡し、頼むかをサブエージェントに決めさせる。** 見ていない範囲があるのに頼まない誤りが残り、サブエージェントの答え方の定義とも食い違う（同上）。
-   **レビューの関門を変える変更を信頼の境界に含めない、または `.claude/` の全体を含める。** 前者は関門を緩める変更が頼まれずに済みうる。後者は割り当てを多く使う（同上）。
-   **Codex のレビューが届かないときに CodeRabbit へ自動で切り替える。** CodeRabbit の PR レビューは機能しておらず、2 つの判定の方法を保守することにもなる（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   **ローカルの CLI レビューも Codex に切り替える。** CLI の割り当ては PR レビューと別であり、CodeRabbit CLI の `--base-commit` や構造化出力など、今の手順がそのまま使える（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md) §1）。
-   **Codex CLI を Claude Code の Bash 権限の前方一致で直接実行する。** `Bash(codex review *)` のような許可は、後ろに `--dangerously-bypass-approvals-and-sandbox` を付けた実行まで許す。作業ツリーにスクリプトを置く方法も、レビュー対象のブランチのコミットで中身を書き換えられる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **範囲を一時 worktree で実現する。** 後片付けと指摘のパスの読み替えが要るうえ、Codex は作業ディレクトリの外も読めるので、秘密のファイルから遠ざける利点も薄い（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **Codex のセッションの記録にある構造化された `review_output` を読む。** 公開された仕様ではなく、実行ごとに 2 つできるファイルから 1 つを選ぶ手順が要る（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **Codex にモデルと推論の強さを指定する。** モデルの名前をツールに書くと、名前が変わるたびにツールを直すことになる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **`--ephemeral` を付ける。** 観測しておらず、利用者が普段 Codex を使うときにも残る記録が、本タスクで新たに増えるリスクではない（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **Codex にレビューの指示（PROMPT）やプロジェクトの規則の指定を渡す。** Codex は指示しなくても `AGENTS.md` と `CLAUDE.md` を読み、`AGENTS.md` の内容をレビューの指示に入れる。CodeRabbit CLI の `-c CLAUDE.md` が担う役割は、これで足りる（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **受領コメントが決まった形に合えば、利用者に尋ねずに完了とする。** 理由は記録されていない（[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md)「issue #1005 で決めたこと」）。
-   **結果のない完了で、受領コメントの 3 行（1 行目・結果の行・`Reviewed commit:` の行）をそのまま絞り込んでセッションに出す。** 本文の一部がエージェントに渡り、ADR-0015 の更新と、本文を出さないことを確かめるガードテストの例外が要る（[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **受領コメントの指摘ありの形も受け入れ、スレッドの処理に回す。** 指摘ありなのに `has_any_findings` が `null` という状態は、理由がいっそう分からない（同上）。
-   **受領コメントの結果の行を、観測した文とちょうど一致するときだけ受け入れる。** `Didn't find any major issues.` の後ろの文が観測した 2 件で違い、Codex が後ろの文を変えるたびに止まる（同上）。
-   **受領コメントが 2 件以上あるときに、最新の 1 件を使う。** 要約コメントが 2 件以上あるときに選ばずに止める規則と合わない（同上）。
-   **結果のない完了で、受領コメントがまだなければすぐ止める。** 受領コメントが要約コメントの更新より遅れて投稿されると、その間のポーリングで止まる（同上）。
-   **利用者が認めた完了を、squash コミットや PR へのコメントに記録する。** 前者は `mergepr.md` の変更が要り、後者は GitHub への書き込みが増える（同上）。
-   **利用者が認めた完了を LAST_REVIEWED に含めない。** LAST_REVIEWED が「なし」になり、マージの質問が PR レビューの依頼を勧めて、割り当てを余分に使う（同上）。
-   **形に合う受領コメントも、今までどおり想定外の投稿に数える。** 受領コメントが要約コメントの完了より先に届くと止まり、再実行が依頼を投稿し直して割り当てを使う（同上）。
-   **形に合わない受領コメントも、想定外の投稿に数えない。** 観測していない上限やエラーの知らせが受領コメントの印を持つと、URL を示されずに期限まで待って止まる（[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **承認を Completed と同じに扱い、要約コメント・受領コメントが変わらない間は判定し直さない。** 新しい依頼の後に前の承認を使い回しうるうえ、承認の直後に届いたコメントで尋ねる条件が崩れても止まらない（同上）。
-   **`codexreview` の比較に、ローカルのすべてのブランチとタグ、stash の一覧も含める。** 0049 で採った形である。ほかの worktree の操作でも止まり、利用者の判断を待つか、やり直して Codex の割り当てをもう 1 回使うことになる。止まらない CodeRabbit CLI の側とも揃わない（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「背景」と「issue #778 で決めたこと」）。
-   **`codexreview` と `/mergepr` の排他。** 2 つのツールにまたがる約束が増え、ほかのセッションの commit による停止は減らない（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「issue #778 で決めたこと」）。
-   **`/mergepr` の形の変化（`refs/remotes/origin/*` への fast-forward とブランチの削除）だけを許す。** ブランチの削除を見逃すと、push していない commit の消失に気づけない（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「issue #778 で決めたこと」）。
-   **ほかの worktree の ref の変化なら、自動でやり直す。** Codex の割り当てを余分に使う（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「issue #778 で決めたこと」）。
-   **比べなくなったものを結果から外し（`changed_refs` を削り）、`--protocol` を 2 に上げる。** 入れ直しの前後で番号の合わない組み合わせの間は、Codex CLI のフォールバックが働かなくなる（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「要件定義の際に決めたこと（2026-11-10）」）。

## 結果

-   1 つの PR は、開いたときに 1 回だけ自動でレビューされる。push のたびのレビューはなく、以後の変更はローカルの CLI レビューで見る。
-   Codex の割り当ては、利用者の他の ChatGPT の利用と共有である。量は公表されていないので、レビューを頼む回数は、PR の作成時と、エスカレーションか利用者の依頼のときだけに限る。
-   `codexreview` は Codex CLI に利用者の設定・規則のファイル・プラグインを読ませず、環境変数も決めたものだけを渡す。レビューの対象の差分と、Codex がレビュー中に読むものは OpenAI のサービスへ送られる（[CLAUDE.md](../../../CLAUDE.md)「Tool Execution Safety」）。
-   Codex が読み取り専用の sandbox を破って、HEAD のブランチ以外のブランチ、タグ、stash を書き換えても、`codexreview` は気づかない。sandbox が効いている限り起こらない。起きても、この worktree の `git status`、HEAD、HEAD のブランチの ref の変化は、今と同じく見つかる。ただし、push していないほかのブランチの commit が失われても、`codexreview` からは分からない（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md) の「4.2. セキュリティ (Security)」）。
-   見た範囲を記録の commit でブランチに残すので、`/fixpr` の回ごとに記録の commit を push し、CI がもう 1 回走る。この費用は受け入れる（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 21）。
-   見たかどうかは、ローカルの CLI レビューがその範囲をレビューしたかどうかだけで決める。このため、`unclear` や `deferred` の指摘が残っていても、ほかの条件にだけ当たる変更では PR レビューを頼まないことがある。また、見た範囲の記録の行はレビューを受ける側のセッションが書くので、レビューしていない範囲を形の正しい行で書けば、頼まずに済んでしまう。この 2 つの危険は、どちらも受け入れる（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 14、5.2）。
-   「レビュー済み」の判定は、Codex の要約コメントの形に依存する。形が変わったときは、`/runpr` が止まるので、`runpr.md` の step 3 を直す。結果のない完了では、受領コメントの形にも依存する。受領コメントの形が変わったときも止まる。
-   結果のない完了で受領コメントが来ないとき（PR #984 の形）は、すぐには止まらず、依頼から約 15 分たってから止まる。待つ間に依頼はしない。
-   利用者が認めた完了は実行の中だけで持つので、再実行のたびに利用者に尋ねる。Codex 自身が実際にはレビューしないまま指摘なしの受領コメントを投稿した場合は、利用者の確認でも見分けられない。`completed` と真偽値がそろっても中身のあるレビューを保証しない、今の危険と同じ種類として受け入れる（[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md) の 4.2）。

## 経緯

- 2026-10-21 PR レビューを CodeRabbit の GitHub 連携から Codex に切り替え、PR を開いたときの 1 回だけにした。切り替えの前に振る舞いを観測して記録し、その結果でレビュー済みの判定を決めた。CodeRabbit の自動 PR レビューを止め、ローカルの CLI レビューは CodeRabbit のまま残した（[0031 要件定義書](../../tasks/0031_codex_pr_review/01_requirements.md)）
- 2026-10-28 ローカルの CLI レビューで CodeRabbit CLI が失敗したときに、Codex CLI を `codexreview` 越しに使うフォールバックを加えた。指摘の記録の行を CLI ごとに分け、異系統のレビューを受けていない範囲を示す行を加えた（[0049 要件定義書](../../tasks/0049_codex_local_review/01_requirements.md)）
- 2026-11-10 `codexreview` が Codex CLI の前後で比べるものを、この worktree の `git status`、HEAD、HEAD のブランチの ref に絞った。0049 で比べていたほかのブランチ、タグ、stash は比べないことにし、結果の形と `--protocol` の番号は変えないことにした（[0079 要件定義書](../../tasks/0079_codexreview_state_scope/01_requirements.md)）
- 2026-11-17 `/runpr` が PR レビューを追加で自動で頼むのを、信頼の境界・セキュリティ・耐久性に触れる変更だけに絞った。ほかの基準に当たる変更は、ローカルの CLI レビューがすべて見ていれば頼まず、見ていない範囲があれば利用者に尋ねることにした（[0098 要件定義書](../../tasks/0098_fable_partial_review/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）
- 2026-11-17 ローカルの CLI レビューが見た範囲を記録の行に残し、Codex の最後の PR レビューの後の commit をすべて見たかを、`origin/main` の判定のスクリプトで決めることにした。自動で頼むかを決まった規則で決め、レビューの関門を変える変更を信頼の境界に含めた（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）
- 2026-11-18 信頼の境界に含めるレビューの関門に、ローカルの CLI レビューの試行の結果と範囲を決める手順、`runpr.md` の LAST_REVIEWED の定義、Escalation の手順を足した（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md)「実装の際に決めたこと（2026-11-18）」）
- 2026-11-19 要約コメントが `completed` で `has_any_findings` が `null` のときの扱いを決めた。受領コメントの決まった形を本文を出さずに判定して利用者に確かめ、利用者が認めたときだけ、その実行の中で完了とみなして LAST_REVIEWED に含める。受領コメントは想定外の投稿に数えない（[0108 要件定義書](../../tasks/0108_codex_null_findings_receipt/01_requirements.md)「issue #1005 で決めたこと」「要件定義の際に決めたこと（2026-11-19）」）
