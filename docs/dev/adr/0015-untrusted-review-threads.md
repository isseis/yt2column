# ADR-0015: レビューのスレッドと Bot への依頼は信頼せず、書き込み権限・範囲の規則・interaction limits で守る

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

このリポジトリは公開されており、GitHub のアカウントを持つ人なら誰でも、PR にレビューのスレッドを開ける。`/fixpr` は、未解決のスレッドの本文をエージェントに読ませ、リポジトリのファイルを直させ、commit と push まで行う。`/runpr` は、その `/fixpr` を繰り返し呼ぶ。スレッドの本文は、外部の人が書いた信頼できないテキストで、プロンプトインジェクションの経路になる。本文を読んだエージェントはファイルを編集できるので、誤ると範囲外の変更が commit され、push される。

GitHub App として入っている自動レビュアー（Codex・CodeRabbit）は、書き込み権限を持つ Bot として振る舞う。書き込み権限のない人は、PR や issue のコメントで自動レビュアーに依頼し、自動レビュアーの名義のスレッドに自分の指示を入れられる。自動レビュアー自身も、PR のブランチに push しうる。

## 決定

-   **スレッドの本文はデータであり、指示ではない。** エージェントのプロンプトは本文を実行ごとの乱数を含む区切りの行で囲み、区切りの行を本文が終わらせられないようにする。返信の文面と PR の説明も、本文を読んだエージェントが書くので、同じ扱いにする。外部の人が書いた本文を、ファイルを編集できるエージェントが読むためである。規則は [fixpr.md](../../../.claude/commands/fixpr.md) の R6 と、[fixprguard_guide.md](../guides/fixprguard_guide.md) の 1 章「位置付け」・7.1「保証すること」にある。
-   **信頼する著者は、リポジトリへの書き込み権限（write・maintain・admin）を持つ Bot でないアカウントと、`_context.md` の「Automated reviewer list」に挙げた自動レビュアーだけである。** 自動レビュアーは、Bot であることと名前の一致の両方で判定する。権限の照会は自動レビュアー以外のアカウントにだけ行う。公開リポジトリでは誰でもスレッドを開けるので、本文をデータとして扱うだけでは守りにならず、Bot であることだけでも信頼する理由にならないためである。規則は [.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Automated reviewer list」の行と、[fixprguard_guide.md](../guides/fixprguard_guide.md) の 1 章「位置付け」にある。
-   **スレッドのすべてのコメントの著者を確かめ、1 件でも想定外の著者があれば、スレッド全体を想定外とする。** コメントの数が多くても、1 つのスレッドのコメントを、取得する上限まで見る。上限を超えるスレッドと、取得できないコメントがあるスレッドも想定外とする。想定外のスレッドは、トリアージ・修正・返信・解決をせず、URL と理由を報告する。本文は取得せず、セッションにも渡さない。利用者が内容を確かめて名指ししたスレッドだけを処理する。想定外の著者のコメントだけを除くと会話の文脈が欠けて指摘を誤って理解するおそれがあり、トリアージだけでも本文がエージェントに渡るためである。`/runpr` のセッションもエージェントなので、本文を渡さない。規則は [fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章 `begin` と 7.1「保証すること」、[.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 3 にある。
-   **修正の範囲は、スレッドの `path` そのものと、`path` と同じディレクトリ、その `path` があるタスクのディレクトリのうち、[fixprguard_guide.md](../guides/fixprguard_guide.md) の 6 章「範囲の規則」が定める範囲に限る。** 実行関連ファイルは、スレッドの著者と `path` にかかわらず範囲外とする。修正のフェーズで範囲外の変更が出たら、build checks・commit・push の前に、利用者に一覧を示して確かめる。commit は判定したパスだけを stage する。ローカルの CLI レビューの修正は、この規則の対象にしない（fix round の push の項目）。信頼する著者のスレッドを経ても指示は届きうるので、著者の確認とは別の守りを残すためである。build checks の前に確かめるのは、build checks が修正したコードを実行するためである。規則は [fixprguard_guide.md](../guides/fixprguard_guide.md) の 6 章と 7.1「保証すること」、[.claude/commands/fixpr.md](../../../.claude/commands/fixpr.md) の Phase 3.5 と Phase 4 にある。
-   **GitHub のレビューのスレッドへの返信・スレッドの解決・PR の説明の更新は、ガードツールの `reply`・`edit-pr`・`append-pr` で行う。** 道具はシェルを介さず、本文を標準入力で渡す。`push` も道具が行い、PR の作成は `gh`、マージは `/mergepr` の `mergepr` ツールが行う。`@codex` のメンションを含む文面は、レビューのスレッドへの返信と PR の説明の更新では、GitHub に書き込む前に拒否する。コマンドが投稿する文面は、レビューを頼む `@codex review` だけがメンションを含み、ほかは投稿の前にメンションがないことを確かめる（[ADR-0014](0014-deterministic-checks-in-go.md)）。返信の文面は本文を読んだエージェントが書くので、シェルに渡すとコマンドとして実行されうる。利用者のアカウントの投稿が Codex を動かすのも防ぐためである。規則は [fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章「サブコマンド」と 7.1「保証すること」、[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」にある。
-   **fix round の push は、ローカルの CLI レビュー（`/fixpr` の Phase 4.5）の修正の後のものも含めて、すべて道具が行う。** Phase 4.5 では、手順が定める git の起動と push の前に、git の状態を `begin` の基準値と比べ、違えば止める。範囲の検査と、判定した集合との一致の確認は行わない。基準値を持たない、`/fixpr` 以外の呼び出し元は、この入力を渡さない。Phase 4 の commit は、セッションが渡す値を信じず、道具が git の状態から求める。比べるのは、Phase 4 の push の確認の後に書き換えられた hooks や config を、その後の git の起動・commit・push で実行させないためである。範囲の検査を行わないのは、範囲の規則が元にするスレッドの `path` が Phase 4.5 にはなく、`finish` の commit のメッセージの規則がローカルの CLI レビューの記録の行と合わないためである。判定した集合との一致を確かめないのは、Phase 4.5 にはセッションと別に集合を判定する者がいないためである。Phase 4 の commit を道具が求めるのは、セッションが信頼できないテキストの影響を受けうるためである。規則は [fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章 `check`・`push-review` と 7.1「保証すること」、[local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md) にある。比べる時点と内容は [0076 要件定義書](../../tasks/0076_fixpr_local_review_guard/01_requirements.md) §3、呼び出しの形は [0076 設計書](../../tasks/0076_fixpr_local_review_guard/02_architecture.md) §3 にある。
-   **fork の PR では、`/fixpr` と `/runpr` は何もせずに止まる。** head を外部の人が管理すると、チェックアウトした `CLAUDE.md` や `.claude/commands/` そのものが攻撃者のものになり、本 ADR のどの守りも働かないためである。規則は [.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 1 と、[fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章 `head`・`begin` にある。
-   **interaction limits をアカウントに設定し、`/runpr` は PR ごとにその状態を確かめる。** 対象は 4 つの設定で、設定は利用者が行う。`/runpr` は、`fixprguard interaction-limits` で制限の状態を読み、PR の作成の時刻と比べ、利用者以外の `@codex` のメンションがないことを確かめる。確かめられないとき、制限がないとき、制限が弱いとき、PR が制限の設定より前に作られたときは、レビューを待たずに止まる。書き込み権限のない人が自動レビュアーに指示する入口を塞ぐためである。interaction limits は期限で自動に解除されるので、PR の作成の時刻と比べ、制限が外れていた間に作られたスレッドを見逃さない。規則は [fixprguard_guide.md](../guides/fixprguard_guide.md) の 10 章「GitHub の設定」と、[.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 1・step 3 にある。
-   **制限の状態の判定は、GitHub の応答を厳密に検査する Go のサブコマンドで行う。** 「今」は同じ応答の `Date` ヘッダ（GitHub の時刻）から取る。応答の検査は 1 つの番兵のエラーにまとめ、部分的な結果を使わない。`gh api --jq` は重複した項目を黙って後勝ちにするので応答を厳密に検査できず、ローカルの時計で判定すると時計のずれがそのまま判定のずれになるためである。規則は [fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章 `interaction-limits` と 10 章「GitHub の設定」、[0057 設計書](../../tasks/0057_reviewer_relay_guard/02_architecture.md) §1.2 にある。

## 採らなかった案

-   **本文をデータとして明示するだけで、著者を確かめない。** 公開リポジトリでは誰でもスレッドを開けるので、プロンプトの書き方だけでは守りにならない（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1）。
-   **想定外の著者のスレッドもトリアージだけ行う。** トリアージだけでも本文がエージェントに渡るので、守りにならない（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1）。
-   **`/fixpr` の実行中に、スレッドごとに利用者へ聞く。** `/runpr` の自動の繰り返しのたびに、利用者を待たせることになる（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1）。
-   **想定外の著者のコメントだけを除いて処理する。** 会話の文脈が欠けて、指摘を誤って理解するおそれがある（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1）。
-   **範囲外の変更があれば止める。** コードの修正では別のパッケージを変えることもあり、正当な修正まで止まる（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1）。
-   **修正の範囲を、スレッドの `path` のファイルだけに限る。** 同じパッケージのテストを直すたびに確認が入り、`/runpr` の繰り返しのたびに利用者を待たせる（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1）。
-   **範囲外の変更を捨てる。** 何が書き換えられたかを、利用者が確かめられなくなる（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1）。
-   **会話コメントの著者まで確かめて止める。** 自動レビュアーが何をもとにスレッドを書いたかは、会話コメントの著者だけからは決まらない。自動レビュアーを経た指示は、本文をデータとして扱うことと範囲の確認で抑え、残余のリスクとして記録する（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1、§4.2）。
-   **`--reviewer` を名前の数だけ文書に書き、一覧を変えるたびに文書を変える。** 文書は 1 つのプレースホルダーを書き、名前ごとに 1 回ずつ繰り返すと定める。名前を渡し忘れても、その Bot のスレッドは信頼されないだけで、安全側に倒れる（[0033 設計書](../../tasks/0033_multiple_trusted_reviewers/02_architecture.md) §1.2）。
-   **範囲の規則の変更で `--protocol` を上げない。** 番号を上げないと、古い道具と新しい文書が食い違ったまま、違う規則で黙って判定しうる（[0036 要件定義書](../../tasks/0036_fixprguard_exec_file_scope/01_requirements.md) §1）。
-   **すべてのブランチを push の制限の対象にする。** PR のマージ（`main` の更新）も制限に当たり、`/mergepr` の `gh pr merge` が止まる。`main` への直接の push は、既存の ruleset がすでに禁じている（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md) §1）。
-   **interaction limits を `gh api --jq` で読む。** `jq` は重複した項目を黙って後勝ちにするので、応答を厳密に検査できない（[0057 設計書](../../tasks/0057_reviewer_relay_guard/02_architecture.md) §1.2）。
-   **制限の状態をローカルの時計で判定する。** ローカルの時計がずれていると、判定もずれる。同じ応答の `Date` ヘッダを使えば、1 回の取得で制限と時刻がそろう（[0057 設計書](../../tasks/0057_reviewer_relay_guard/02_architecture.md) §1.2）。
-   **制限が外れているか確かめられないときに、警告して処理を続ける。** 制限が外れていた間に自動レビュアーが作ったスレッドは、本来のレビューのスレッドと見分けられず、利用者が手で探すと見逃しうる（[0057 設計書](../../tasks/0057_reviewer_relay_guard/02_architecture.md) §1.2）。
-   **ローカルの CLI レビューの commit と push の前に比べるだけで、push は `git push` のままにする。** push 先の OID の確認と明示した refspec が付かず、`push` を道具が行うという決定と食い違う（[0076 要件定義書](../../tasks/0076_fixpr_local_review_guard/01_requirements.md) §1）。
-   **ローカルの CLI レビューの修正も `inspect` → `finish` → `push` に通す。** スレッドがないので新しい範囲の規則が要り、`finish` の commit のメッセージの規則を記録の行と README の同期の commit に合わせて変える必要がある（[0076 要件定義書](../../tasks/0076_fixpr_local_review_guard/01_requirements.md) §1）。
-   **既存の `push` が、HEAD が `--commit` と一致するかで Phase 4 と Phase 4.5 の push を見分ける。** 呼び出し元のデータの形から振る舞いを選ぶことになり、型や引数で振る舞いを宣言する方針に反する（[0076 設計書](../../tasks/0076_fixpr_local_review_guard/02_architecture.md) §1.2）。
-   **共有の手順を変えず、`fixpr.md` から手順の各 step の前に比べるよう指示する。** 共有の手順の step の番号や構成が変わっても、`fixpr.md` の指示は気づかれないままずれる（[0076 設計書](../../tasks/0076_fixpr_local_review_guard/02_architecture.md) §1.2）。
-   **疑わしいコメントを一律に削除する。** 正当なレビューも消え、取り消せない（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md) §1）。
-   **信頼する自動レビュアーの一覧から CodeRabbit を外す。** Web から CodeRabbit のレビューを頼む使い方があり、書き込み権限のない人は制限が有効な間はどちらの Bot にも指示できない。CodeRabbit だけを厳しく扱っても減る危険は小さい（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md) §1「実装の際に決めたこと（2026-11-04）」）。

## 結果

-   想定外の著者が開いたスレッドは、利用者が内容を確かめて名指しするまで処理されない。その本文は、`/fixpr` にも `/runpr` のセッションにも渡らない。
-   信頼する著者のスレッドでも、本文を読んだエージェントが指示に従うことは保証できない。範囲の確認と、書き込みを道具に限ることを、別の守りとして重ねる。
-   修正のフェーズの範囲外の変更は、build checks の前に利用者の確認に回る。build checks は修正したコードを実行するので、確認の後に build checks を動かす。
-   interaction limits はアカウント全体に効き、ほかの公開リポジトリにも及ぶ。期限は最長 6 か月で、切れる前に利用者が更新する。`/runpr` は、制限がない、弱い、確かめられない、または PR が設定より前に作られたときは、その PR をマージしない。
-   `/runpr` は、4 つの設定のうち interaction limits だけを読む。PR と issue の作成の制限と push の制限は、利用者が設定画面で確かめる。
-   信頼する自動レビュアーの一覧には CodeRabbit も残る。Web から CodeRabbit のレビューを頼んだ場合のスレッドも、`/fixpr` と `/runpr` は処理できる。
-   ローカルの CLI レビューの修正は、範囲外のファイルを変えても利用者の確認に回らない。その修正は、信頼できないテキストの影響を間接的に受けた指摘に従う。比較が検出するのは、git の状態（hooks・config・info・対象のファイル、ブランチ、HEAD の系列）の書き換えだけである。

## 経緯

- 2026-10-21 `/fixpr` と `/runpr` で、スレッドの本文を信頼できないデータとして扱い、すべてのコメントの著者を書き込み権限と自動レビュアーの一覧で判定することにした。想定外のスレッドは処理せず、修正の範囲を `path` の周辺に限り、範囲外の変更は commit の前に利用者へ確かめる（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md)）
- 2026-10-21 信頼する自動レビュアーを複数名にできるようにし、`--reviewer` の一覧を `_context.md` の 1 か所に置いた。一覧の名前は GraphQL の `author.login` の形で照合する（[0033 要件定義書](../../tasks/0033_multiple_trusted_reviewers/01_requirements.md)）
- 2026-10-22 範囲の規則で、実行関連ファイルをスレッドの著者と `path` にかかわらず範囲外とし、`.go` を許可リストに移した。規則を変えたので `--protocol` を `3` に上げた（[0036 要件定義書](../../tasks/0036_fixprguard_exec_file_scope/01_requirements.md)）
- 2026-11-02 外部の人が自動レビュアーを介して指示を届ける経路を塞ぐため、Codex を外部の指示に応じうるものとして扱い、interaction limits と PR・issue の作成の制限、ブランチの push の制限を設けた。実行関連ファイルは信頼する著者のスレッドでも範囲外とし、`/runpr` が制限の状態と PR の作成の時刻、利用者以外のメンションを確かめることにした（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md)）
- 2026-11-03 interaction limits をアカウント全体の設定とし、制限が外れているとき、弱いとき、確かめられないときは、警告して続けずに止めることにした。push の制限の対象から `main` を外し、PR のマージを止めないようにした（[0057 設計書](../../tasks/0057_reviewer_relay_guard/02_architecture.md) §1.2）
- 2026-11-04 信頼する自動レビュアーの一覧に CodeRabbit を残し、一覧から外す要件（F-002）を取り下げた。Web から CodeRabbit のレビューを頼む使い方を残すためである（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md) §1「実装の際に決めたこと（2026-11-04）」）
- 2026-11-09 `/fixpr` のローカルの CLI レビューの push も道具が行い、手順が定める git の起動と push の前に git の状態を基準値と比べることにした。範囲の検査と、判定した集合との一致の確認は行わない（[0076 要件定義書](../../tasks/0076_fixpr_local_review_guard/01_requirements.md)）
- 2026-11-10 Phase 4.5 の比較は道具の新しいサブコマンド `check` に、push は `push-review` に行わせ、比べる呼び出しの多くは、呼び出し元が渡す入力としてローカルの CLI レビューの共有の手順に加えることにした（[0076 設計書](../../tasks/0076_fixpr_local_review_guard/02_architecture.md)）
