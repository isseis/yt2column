# 要件定義書：外部の人が自動レビュアーを介して /fixpr のエージェントに指示を届ける経路を塞ぐ

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-02 |
| Review date | 2026-11-03 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、書き込み権限のない第三者が、コメントから自動レビュアーを介して `/fixpr`・`/runpr` のエージェントに指示を届け、`make` の実行に至る経路を狭めるための要件を定義する。issue は、経路を入口・増幅・被害の 3 か所で塞ぐとした。本書は入口と増幅に手を入れ、被害の側と、塞がない部分を残余のリスクとして記録する。用語の意味は 6 章で定める。

**背景:**

`fixprguard` は、すべてのコメントの著者が信頼する著者であるスレッドを、信頼する著者のスレッドとする。信頼する著者は、自動レビュアーの一覧の名前の Bot と、書き込み権限を持つ人である（`internal/fixprguard/trust.go:62-103`）。自動レビュアーの一覧は、`coderabbitai` と `chatgpt-codex-connector` である（`.claude/commands/_context.md:185`）。信頼する著者のスレッドの本文は、トリアージと修正のエージェントに渡る。

本リポジトリは公開リポジトリである。そのため、次の経路がある（issue #629）。

1.  **入口:** 書き込み権限のない人が、issue や PR へのコメントで自動レビュアーに指示する。自動レビュアーは、攻撃者の文言を含むレビュースレッドを Bot 名義で作りうる。
2.  **信頼:** このスレッドの著者は Bot だけなので、信頼する著者のスレッドになる。本文はエージェントに届き、残る守りは区切りのブロックだけである。区切りのブロックは、エージェントがその中の指示に従わないことを保証しない（`docs/dev/guides/fixprguard_guide.md` 7.2.2）。
3.  **増幅:** 範囲の規則は、信頼する著者のスレッドの `path` が実行関連ファイルなら、その `path` そのものの変更を範囲内とする（`internal/fixprguard/scope.go:232-234`、`docs/dev/guides/fixprguard_guide.md` 6 章の規則 4）。修正のフェーズが `Makefile` を書き換えると、`finish` は利用者の確認なしに、書き換え後の `Makefile` で `make fmt`・`make test`・`make lint` を実行する（`docs/dev/guides/fixprguard_guide.md:162`）。
4.  **被害:** `make` に渡す環境は、`GIT_*` を除いてセッションの環境変数を引き継ぐ（`cmd/fixprguard/main.go:164`、`internal/cmdrunner/git.go:36-45`）。

スレッドとは別に、Bot が PR のブランチに commit を push する経路もある。書き込み権限のない人は push できないが、GitHub App として入っている Bot は push の権限を持ちうる。`/runpr` は、ほかの人が push した commit を `git pull --ff-only` で取り込む（`.claude/commands/runpr.md:180-185`）。`begin` は取り込み後の作業ツリーを基準値にするので、取り込んだ `Makefile` やテストのコードは、範囲の検査も利用者の確認も経ずに `finish` の `make` に届く。

要件定義の際に、次のことを確かめた（2026-11-02）。

-   **CodeRabbit:** 応答する相手を限る設定は `chat.allow_non_org_members` だけで、organization のリポジトリにしか効かない（[CodeRabbit configuration reference](https://docs.coderabbit.ai/reference/configuration)）。本リポジトリは個人のリポジトリなので、設定では限れない。また、`.coderabbit.yaml` は自動レビューを無効にしている（`.coderabbit.yaml:13-14`）。それでも CodeRabbit は、[PR #617](https://github.com/isseis/yt2column/pull/617)・[PR #620](https://github.com/isseis/yt2column/pull/620) で、PR が開いた約 10 秒後にレビュースレッドを 3 件ずつ作った（`gh api graphql` でスレッドの最初のコメントの著者を数えた）。
-   **Codex:** 公式の文書（[Codex code review in GitHub](https://learn.chatgpt.com/docs/third-party/github.md)）には、誰が `@codex` で Codex を起動できるかの制限が書かれていない。一方、同じ文書には、`review` 以外の言葉を添えたメンションは cloud chat を始め、権限があれば修正を push しうる、と書かれている。
-   **秘密の置き場所:** 秘密は `.envrc` に置かれる（`CLAUDE.md`「Development Notes」）。`.envrc` はこの worktree の最上位にもある。同じ OS ユーザーのプロセスは `.envrc` を読める（`docs/dev/security.md` §8、`:95`）ので、`make` が実行するコードも読める。要件定義のセッションの環境変数を名前だけで確かめたところ、issue が挙げたパターン（`*_API_KEY`・`*_TOKEN`・`*_WEBHOOK_URL`・`*_SECRET`）に当たるのは `CLAUDE_CODE_MESSAGING_TOKEN` だけだった。push に使える `SSH_AUTH_SOCK` は、パターンに当たらない。環境変数は、セッションの起動のしかた（direnv が読み込むかなど）で変わる。

対応 issue: [#629](https://github.com/isseis/yt2column/issues/629)（統合した issue: [#427](https://github.com/isseis/yt2column/issues/427)、[#397](https://github.com/isseis/yt2column/issues/397)、[#428](https://github.com/isseis/yt2column/issues/428) の項目 2。関連: [#395](https://github.com/isseis/yt2column/issues/395)（タスク `0030_fixpr_untrusted_threads`）、[#425](https://github.com/isseis/yt2column/issues/425)（タスク `0036_fixprguard_exec_file_scope`）、[#405](https://github.com/isseis/yt2column/issues/405)）

**本書の記述範囲:** 本書が定めるのは、観測できる振る舞いである。具体的には、範囲の規則がどの変更を範囲外にするか、どの Bot のスレッドを信頼するか、利用者が行う GitHub の設定、`/runpr` が示す警告と止まる条件、ガイドに書く内容である。次のことは設計（`02_architecture.md`）で決める。

-   interaction limits の状態を読む処理を置く場所（`fixprguard` のサブコマンドか、コマンドの文書の `gh` の呼び出しか）と、応答の検査の手段
-   期限の比較に使う現在の時刻の取り方
-   ガイドへの記述の載せ方と、文書のガードの更新
-   テストの手段

要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #629 で決めたこと:**

-   **経路を、入口・増幅・被害の 3 か所で塞ぐ:** 本書の F-001〜F-004 が入口と増幅を扱う。要件定義の際に見つかった Bot の push の経路は、F-003 の push の制限で塞ぐ。被害の側は、要件定義の際の決定（下記の「`make` に渡す環境から秘密を落とさない」）で、仕組みを足さず残余のリスクとした。
    -   理由（issue）: 1 か所だけを塞いでも、ほかの 2 か所が残れば経路は開いたままである。
-   **コマンドの側の守りは残す:** タスク 0030 の F-001・F-002（想定外の著者のスレッドを処理しない守り）は変えない。
    -   理由: GitHub の設定が外れたときの安全策である。
-   **#405（複数の自動レビュアー）で Bot を増やすときは、この経路を Bot ごとに評価する:** 本書はその手順を定めないが、ガイドの残余のリスクに書く（AC-21）。
    -   理由: 新しい Bot が外部の人の指示に応じるかは、Bot ごとに違う。

**要件定義の際に決めたこと（2026-11-02）:**

-   **Bot の挙動は、要件定義の際にベンダーの文書で確かめる:** 結果は「背景」に書いた。
    -   理由: 確かめていない前提の上に要件を立てないためである。
-   **CodeRabbit を自動レビュアーの一覧から外す（F-002）:** CodeRabbit のスレッドは、`other-bot` として想定外の著者のスレッドになる。利用者が名指ししない限り処理されない。
    -   理由: CodeRabbit は、個人のリポジトリでは応答する相手を設定で限れない。PR レビューは Codex が行い（`CLAUDE.md`「Claude Commands」）、CodeRabbit は手順の上で PR にスレッドを作る役を持たない。
    -   受け入れる負担: CodeRabbit が意図せず作ったスレッド（PR #617・#620 の例）は、`/fixpr` が自動では直さず、想定外の著者のスレッドとして利用者の判断に回る。
-   **Codex は、外部の人の指示に応じうるものとして扱う:** Codex は自動レビュアーの一覧に残す。入口の守りは interaction limits（F-003）に頼り、制限が外れている間を残余のリスクとする（4.2）。
    -   理由: Codex は PR レビューを行う唯一の自動レビュアーで、一覧から外せない。文書では応じないことを確かめられないので、安全側に倒す。
-   **interaction limits と作成の制限を使う（F-003）:** interaction limits の `collaborators_only` を有効にする。あわせて、PR の作成と issue の作成を collaborators に限る、期限のない設定（作成の制限）も使う。設定は利用者が行う。
    -   理由: 書き込み権限のない人のコメントを止められるのは interaction limits だけである。ただし最長 6 か月で解除されるので、期限のない作成の制限を重ねる。
    -   受け入れる負担: 外部の人は、本リポジトリに issue（脆弱性の報告を含む）・PR・コメントを作れなくなる。
    -   interaction limits は、利用者のアカウント全体の設定（アカウントの Settings → Moderation）で有効にする（2026-11-03 の決定の変更）。リポジトリの interaction limits の取得の応答は、アカウント全体の制限を `origin` が `user` の制限として返す（設計の際に観測した。`02_architecture.md` 1.3）。
        -   理由: 利用者は設計の際にアカウント全体の設定で有効にし、そのまま使うことを選んだ。
        -   受け入れる負担: 制限は、利用者のほかの公開リポジトリにも効く。更新もアカウントの設定画面で行う。
-   **`/runpr` が interaction limits の状態を確かめ、外れていれば止まる（F-004。2026-11-03 の決定の変更）:** 期限まで 14 日以内のときは警告して処理を続ける。制限がないとき、制限が弱いとき、状態を確かめられないとき、PR が今の制限を設定した時刻より前に作られたときは、警告して、Codex の待ちと `fixpr` の前に止まる。
    -   理由: 期限で自動的に解除されるので、気づく仕組みが要る。制限が外れていた間に Codex が作ったスレッドは、Codex の本来のレビューのスレッドと見分けられない。警告だけで続けると、そのスレッドの本文が修正のフェーズのエージェントに届く。利用者が期限切れの間のスレッドやコメントを手で探すと、見逃しうる。
    -   **止まった後は、PR を作り直す。** 利用者が制限を更新し、開いている PR を閉じて、同じブランチから新しい PR を開く。新しい PR には期限切れの間のスレッドもコメントもない。ブランチの中身は、push の制限により利用者の push だけでできている。古い PR は記録として残す。コメントを削除する案は採らない。正当なレビューも消え、取り消せず、外部の人が先に自分のコメントを消せば削除の範囲を決められないためである。
    -   **PR が制限の設定より前に作られたかを、時刻で比べる。** 応答は制限を設定した時刻を返さないので、`expires_at` の 6 か月前を設定の時刻とみなす（見逃しのない側に丸める方法は設計で決める）。PR の作成の時刻がそれより前なら、その PR は制限のない期間を経た可能性があるとして止まる。これで、制限を更新した後に作り直しを忘れても止まる。そのため、interaction limits の期間は常に 6 か月にする（F-003）。
        -   受け入れる負担: 期限の前に更新した場合も、そのとき開いていた PR は作り直しになる。PR ごとに Codex の PR レビューを 1 回使い、実装計画の PR の記録を書き直す。そのため、更新は開いている PR がないときに行うのがよい（ガイドに書く）。
        -   「確かめられない」で止まったときは、制限が実際には有効で、PR より前に設定されていたと利用者が設定画面で確かめられれば、PR を作り直さずに `/runpr` を実行し直す。
    -   **利用者以外の `@codex` のメンションがあれば、状態にかかわらず止まる（AC-27）。** `/runpr` は、PR の会話のコメント・行コメント・PR レビューの本文に含まれるメンションを、すでに一覧にしている（`.claude/commands/runpr.md:301-336`）。外部の人が Codex に指示した痕跡を、制限の状態と別に見つけるためである。外部の人がメンションを消した、または編集で外した後は見つけられないので、時刻の比較と重ねる。
        -   人（`User`）が書いたメンションでは止まる。Bot（Codex のほか）が書いたメンションでは、URL を示して、続けるかを利用者に問う。CodeRabbit は設定に反して自動でレビューのコメントを書くことがあり（PR #617・#620）、変更した行の `@codex review` を引用すると、PR を作り直しても止まり続けるためである。制限が有効な間は、外部の人は Bot に指示できず、制限が外れていた間の中継は、時刻の比較が止める。
-   **規則 4 の例外を、信頼する著者のスレッドについても外す（F-001）:** 実行関連ファイルは、`path` そのものでも、どの著者のスレッドでも範囲外とする。
    -   これは、次の既存の決定を改める。
        -   タスク 0030 の要件定義書 F-004（`docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md:146`）と設計書 3.6 の規則 4（`docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md:573`）
        -   タスク 0036 の要件定義書の AC-05・AC-13（`docs/tasks/0036_fixprguard_exec_file_scope/01_requirements.md:159`・`:211`）
    -   理由: 範囲外の変更は利用者の確認に回る（`docs/dev/guides/fixprguard_guide.md` 6 章）。build checks の前に利用者が見ることが目的なので、手間は小さい。
-   **`.go` のファイルの規則は変えない:** 信頼する著者のスレッドの `path` が `.go` のファイルなら、そのファイル（規則 1）と同じディレクトリの `.go` のファイル（`internal/fixprguard/scope.go:244-246`）は確認なしに範囲に入り、`make test` がそれを実行する。cgo の前文も書ける（`docs/dev/guides/fixprguard_guide.md` 7.2.3）。この経路は残余のリスクとして記録する（4.2）。F-001 の後も、増幅の段は狭まるだけで、閉じない。
    -   理由: 狭めると、日常の修正（テストのコードの修正）のたびに確認が入る。入口は F-002・F-003 で塞ぐ。
-   **build checks のテストが実行するガイドのコードは、残余のリスクとする（2026-11-03 の決定の変更）:** `make test` は、`cmd/fixprguard/docs_test.go` の `TestGuideIgnoredCommands` で、`docs/dev/guides/fixprguard_guide.md` 8.1 の最初の `sh` のブロックを、セッションの環境を引き継いで実行する（`cmd/fixprguard/docs_test.go:495-561`）。ガイドは実行関連ファイルではないので、信頼する著者のスレッドの `path` がガイドか、同じディレクトリの `.md` なら、修正のフェーズが書き換えたブロックを、`finish` が確認なしに実行する。ガイドを実行関連ファイルにせず、テストも変えない。この経路は残余のリスクとして記録する（4.2）。設計のレビューで見つかった。
    -   理由: 同じディレクトリの `.go` のファイルを `make test` が実行する経路（上の決定）と同じ種類であり、同じ扱いにする。ガイドを実行関連ファイルにすると、ガイドを直すたびに確認が入る。テストを変えると、ガイドの手順を実際に動かして確かめる価値を失う。
-   **`make` に渡す環境から秘密を落とさない:** issue の案（#428 の項目 2）は採らない。`make` が実行するコードが秘密を使えることは、残余のリスクとして記録する（4.2）。
    -   理由: 主な秘密は `.envrc` からファイルとして読めるので、環境変数を落としても被害はほとんど減らない（「背景」）。環境変数だけにある秘密（`CLAUDE_CODE_MESSAGING_TOKEN`）と `SSH_AUTH_SOCK` は残るが、仕組みと保守の手間に見合わないと判断した。
-   **Bot が PR のブランチに push する経路は、GitHub の ruleset で塞ぐ（F-003 の 4 つ目の設定）:** 既定のブランチ（`main`）を除くすべてのブランチに「Restrict creations」と「Restrict updates」をかけ、bypass を Repository admin（利用者）だけにする。`/runpr` が取り込んだ commit を検出する仕組みは加えず、CodeRabbit の GitHub App も外さない。
    -   理由: GitHub の文書は、「Restrict updates」を「bypass の権限を持つ人だけが、当たるブランチに push できる」と定める（[Available rules for rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets)）。bypass に指定できるのは役割・チーム・GitHub App などで（[Creating rulesets for a repository](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/creating-rulesets-for-a-repository)）、指定しない App の push は拒否される。interaction limits と違って期限がない。push した人をコマンドの側で見分けるには、署名の検証などの大きな仕組みが要る。
    -   確かめていないこと: 個人の公開リポジトリで ruleset が効くこと、利用者の push と `/mergepr` の squash merge・ブランチの削除が止まらないことは、文書に書かれていない。設定の後の PR で確かめる（AC-10・AC-11）。
    -   既定のブランチを対象から外す（2026-11-03 の決定の変更）。設計の際に、すべてのブランチを対象にして設定したところ、PR のマージ（`main` の更新）も「Restrict updates」に当たり、Web の画面は bypass を選ばない限りマージを止めた。`/mergepr` の `gh pr merge` も、`--admin` を付けなければ止まる見込みである。
        -   理由: `main` は、既存の ruleset（`main`）の `pull_request` の規則が、bypass なしで直接の push を禁じている（設計の際に読んだ。`02_architecture.md` 1.3）。そのため、Bot が `main` に直接 push することは、もともとできない。対象に `main` を含めても、PR のマージのたびに bypass を求めるだけで、本タスクの経路を塞ぐ効果は増えない。`/mergepr` に `--admin` を足す案は、`cmd/mergepr` の変更で範囲が広がり、将来 `main` に加える規則も黙って飛ばすので採らない。
        -   受け入れるリスク: GitHub App として入っている Bot が、API で PR を `main` にマージする経路は、push の制限では止まらない（4.2）。
    -   AC-11 が成り立たなければ（Codex のレビューや `/mergepr` が止まるなど）、作業を止め、決定の変更として本書に戻す。interaction limits についても同じである。
    -   「Restrict deletions」はかけない。Bot によるブランチの削除はコードを持ち込まないので、本タスクの経路に当たらない。`/mergepr` のブランチの削除に影響しないようにするためでもある。
-   **CodeRabbit が設定に反して自動レビューを行う件は、本書の範囲外とする:** 調査と対応は別の issue で扱う。issue は、本書の PR を開いた後に、利用者に確かめてから作る。
    -   理由: F-002 で CodeRabbit のスレッドは信頼されなくなり、push は ruleset で拒否されるので、本タスクの経路には影響しない。原因の調査は本タスクの目的と別である。

以下の決定は、要件定義書を作成する際に、安全側の既定として選んだ。PR のレビューで確かめてほしい。

-   **`protocol` の番号を上げる（F-005）:** 本タスクの前のツールは、新しい文書から呼ばれると終了コード `2` で止まる。
    -   理由: 前のツールは、信頼する著者のスレッドの `path` そのものなら実行関連ファイルを範囲内とする。番号が同じなら、入れ直していない利用者は、古い規則で判定されていることに気づけない（`docs/dev/guides/fixprguard_guide.md` 3 章）。タスク 0036 と同じ扱いである。
-   **interaction limits の制限が `collaborators_only` より弱ければ、制限がないときと同じく扱う（AC-15・AC-25）:** `existing_users`・`contributors_only` は、書き込み権限のない人のコメントを許すので、警告して止まる。
    -   理由: 守りたいのは、書き込み権限のない人のコメントを止めることである。
-   **`/runpr` が状態を読む時点と日付の形（AC-12〜AC-14）:** 対象の PR を特定した後、Codex のレビューを待ち始める前に、interaction limits の状態を 1 回読む。PR がすでにマージされていれば読まない。期限の日付は UTC の `YYYY-MM-DD` で示す。
    -   理由: 修正のフェーズ（`/fixpr`）が信頼する著者のスレッドを処理する前に、利用者が状態を知れるようにする。マージ済みの PR では、スレッドを処理しない。日付は UTC にそろえ、時差による読み違いを避ける。
-   **警告と止まる条件は、`/fixpr` の単独の実行では使わない:** `/runpr` だけが確かめる。
    -   理由: PR は `/runpr` が駆動する（`CLAUDE.md`「Claude Commands」）。`/fixpr` を単独で実行する場合の扱いは、残余のリスクとして記録する（4.2）。
-   **0030 と 0036 の文書に、本タスクで決めた変更を反映する（AC-22）:** 対象は、規則 4 の例外を述べる箇所（0030 の要件定義書 F-004、設計書 3.6、0036 の要件定義書・設計書）と、issue が挙げた 0030 の設計書 5.1・5.3 である。5.1・5.3 には、第三者が自動レビュアーを介してスレッドを作れること、実行関連ファイルが `path` そのものでも範囲外になったこと、`make` に渡す環境を変えないことを書く。本タスクの要件の承認で決まった内容を反映するだけなので、文書の状態は `approved` のままとする。0030 の文書がタスク 0034・0035 の決定を反映したときと同じ扱いである。
    -   理由: ガイドは、設計の背景として 0030 の設計書を挙げる（`docs/dev/guides/fixprguard_guide.md:5`）。古い規則のまま残すと、読み手が今の規則を誤解する。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   書き込み権限のない人が、自動レビュアーを介して、信頼する著者のスレッドを作れないようにする（GitHub の設定が有効な間）。
-   利用者以外（GitHub App として入っている Bot を含む）が、本リポジトリのブランチに push できないようにする。
-   修正のフェーズが変えた実行関連ファイル（ガイド 6 章の実行関連ファイルの表にある名前に当たるパス）は、どの著者のスレッドからの修正でも、利用者の確認なしに build checks で使われないようにする。
-   GitHub の設定が外れたときに、`/runpr` の利用者が気づけるようにし、制限が外れていた間に作られたスレッドを `/runpr` が処理しないようにする。
-   塞がない経路（`.go` のファイル、秘密、Unicode の別名など）を、残余のリスクとして文書に残す。

### 2.2. スコープ (In Scope)

-   範囲の規則で、実行関連ファイルを常に範囲外にする（F-001）。
-   CodeRabbit を自動レビュアーの一覧から外す（F-002）。
-   interaction limits・作成の制限・push の制限を設定する手順を文書にし、利用者が設定する（F-003）。
-   `/runpr` が interaction limits の状態を確かめ、外れていれば止まる（F-004）。
-   `protocol` の番号を上げる（F-005）。
-   ガイド・0030 と 0036 の文書・用語集を合わせる（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   **`make` に渡す環境から秘密を落とすこと:** 1 章の決定により採らない。残余のリスクとして 4.2 に記録する。
-   **CodeRabbit が設定に反して自動レビューを行う件:** 別の issue で扱う（1 章の決定）。
-   **Bot が PR のブランチに push した commit の、コマンドの側での検出:** 行わない。push は ruleset で拒否する（1 章の決定）。
-   **`.go` のファイルの規則:** 変えない（1 章の決定）。
-   **`/fixpr` の単独の実行での警告と止まる条件:** 使わない（1 章の決定）。
-   **#428 の残りの項目（基準値の健全性検査、`reply`・`edit-pr` の fork の検査、`inspect` の確認の明示）、#426、#446:** それぞれの issue で扱う。
-   **書き込み権限を持つ人のコメントの扱い:** 今までどおり信頼する。
-   **製品のセキュリティの文書（`docs/dev/security.md`）:** 対象は製品であり、本タスクは開発の手順の守りなので変えない（タスク 0030 と同じ扱い）。
-   **Codex の設定:** Codex の自動レビューの設定はリポジトリの外にあり（`CLAUDE.md`「Claude Commands」）、変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

以下で「判定」とは、`fixprguard inspect` が、変更したパス・変更の種類・スレッドの `path`・名指しの有無から、その変更が範囲に入るかを決めることをいう。判定の結果は、`inspect` の `outOfScope` に現れる（範囲外ならその要素になり、`inspect` は終了コード `3` を返す）。各 AC の例は、断りがなければ、変更が通常のファイルの変更である場合を示す。

#### F-001: 実行関連ファイルを常に範囲外にする

実行関連ファイルの変更は、スレッドの著者と `path` にかかわらず範囲外とする。実行関連ファイルの定義（`docs/dev/guides/fixprguard_guide.md` 6 章の表）は変えない。

**Acceptance Criteria**:
- **AC-01**: 信頼する著者のスレッドの `path` が実行関連ファイルで、変更したパスがその `path` そのものであるとき、変更は範囲外になる。少なくとも次の各行で範囲外になる。

    | 変更したパス（= スレッドの `path`） |
    |---|
    | `Makefile` |
    | `.golangci.yml` |
    | `.github/workflows/ci.yml` |
    | `internal/writer/CLAUDE.md` |
    | `extension/package.json` |
    | `go.mod` |

- **AC-02**: 名指しされた想定外の著者のスレッドでも、AC-01 の各行は範囲外である（今の振る舞いと同じ）。
- **AC-03**: 実行関連ファイルでないファイルの判定は変わらない。少なくとも次の各行の判定が、本タスクの前と同じである。

    | 変更したパス | スレッドの `path` | 名指し | 判定 |
    |---|---|---|---|
    | `internal/writer/writer.go` | `internal/writer/writer.go` | なし | 範囲内 |
    | `internal/writer/writer_test.go` | `internal/writer/writer.go` | なし | 範囲内 |
    | `internal/writer/writer_test.go` | `internal/writer/writer.go` | あり | 範囲外 |
    | `internal/writer/notes.md` | `internal/writer/writer.go` | なし | 範囲内 |
    | `scripts/check.sh` | `scripts/check.sh` | なし | 範囲内 |

- **AC-04**: AC-01 の変更で `inspect` が終了コード `3` を返した後、利用者が続けることを選べば、`finish` は今までどおりその変更を commit できる。

#### F-002: CodeRabbit を自動レビュアーの一覧から外す

自動レビュアーの一覧を `chatgpt-codex-connector` だけにする。

**Acceptance Criteria**:
- **AC-05**: `.claude/commands/_context.md` の「Automated reviewer list」の名前は `chatgpt-codex-connector` だけである。`/fixpr`・`/runpr` が `fixprguard` に渡す `--reviewer` は `chatgpt-codex-connector` だけである。
- **AC-06**: コメントの著者に `coderabbitai` の Bot を含むスレッドは、`/fixpr` の実行で想定外の著者のスレッドになり、理由 `other-bot`（アカウント `coderabbitai`）として利用者に示される。そのスレッドの本文は取得されず、名指しされない限り処理されない。
- **AC-07**: 利用者が AC-06 のスレッドを名指しすると、そのスレッドは名指しされた想定外の著者のスレッドとして処理される。範囲の規則は、名指しされた想定外の著者のスレッドの規則（`path` そのもの以外の `.go` のファイルと、実行関連ファイルは範囲外）を使う。
- **AC-08**: ガイドの説明と例は、`coderabbitai` を信頼する自動レビュアーとして挙げない。`--reviewer` の例は、自動レビュアーの一覧にある名前を使う。

#### F-003: GitHub の設定で、書き込み権限のない人の操作と Bot の push を制限する

利用者は、本リポジトリに次の 4 つを設定する。エージェントは設定を変えない。

1.  interaction limits の `collaborators_only`（期限は常に 6 か月。アカウント全体の設定で有効にする。1 章の決定）
2.  作成の制限のうち、PR の作成を collaborators に限る設定
3.  作成の制限のうち、issue の作成を collaborators に限る設定
4.  push の制限: 既定のブランチ（`main`）を除くすべてのブランチを対象にした ruleset で、「Restrict creations」と「Restrict updates」をかけ、bypass を Repository admin だけにする（bypass の方式は「Always allow」。「For pull requests only」では利用者の push が止まる）

**Acceptance Criteria**:
- **AC-09**: `docs/dev/guides/` の下のガイドに、4 つの設定の目的、設定する手順、interaction limits を期限の前に更新する手順、設定を変えるのは利用者だけであることが書かれている。
- **AC-10**: 本タスクの最後の PR を開く前に、利用者が 4 つの設定を行っている。その PR の `/runpr` は、interaction limits の状態を「有効」と報告する（AC-13）。作成の制限と push の制限は、利用者がリポジトリの設定画面で確かめる。
- **AC-11**: AC-10 の設定の後に、次の 3 つが今までどおり成り立つ。
    - Codex の PR レビューが、`/runpr` が待つ要約のコメントの完了の状態に達する（`.claude/commands/runpr.md` step 3）。
    - 利用者の `git push` が、PR のブランチの作成と更新に成功する。
    - `/mergepr` の squash merge とブランチの削除が成功する。

#### F-004: `/runpr` が interaction limits の状態を確かめ、外れていれば止まる

`/runpr` は、本リポジトリの interaction limits の状態を読み、上から順に調べて、最初に当たる状態に分類する。「設定の時刻」は、`expires_at` の 6 か月前とみなす時刻である（1 章の決定）。

| 順 | 状態 | 条件 | 振る舞い |
|---|---|---|---|
| 1 | 確かめられない | 取得の失敗（時間切れを含む）、または応答が 4.2 の受け入れる形でない | 警告して止まる |
| 2 | 制限なし | 制限がないことを示す応答、または `expires_at` が今以前 | 警告して止まる |
| 3 | 制限が弱い | `limit` が `existing_users` または `contributors_only` | 警告して止まる |
| 4 | 制限より前の PR | `limit` が `collaborators_only` で、PR の作成の時刻が設定の時刻より前 | 警告して止まる |
| 5 | 期限が近い | `limit` が `collaborators_only` で、`expires_at` が今から 14 日以内 | 警告して続ける |
| 6 | 有効 | `limit` が `collaborators_only` で、`expires_at` が今から 14 日より先 | 警告しない |

**Acceptance Criteria**:
- **AC-12**: `/runpr` は、対象の PR を特定した後、Codex のレビューを待ち始める前に、1 回の実行につき 1 回、interaction limits の状態を読む。PR がすでにマージされていれば読まない。読む操作は `/runpr` の「Authorization」に挙げられている。
- **AC-13**: 状態が「有効」なら、`/runpr` は警告を出さず、最終の報告に期限の日付（UTC の `YYYY-MM-DD`）を 1 行で示す。
- **AC-14**: 状態が「期限が近い」なら、`/runpr` は警告を日本語で示し、処理を続ける。警告は、状態の名前、期限の日付（UTC の `YYYY-MM-DD`）、F-003 の手順があるガイドの場所を含む。警告は、確かめた時点と、`/runpr` の最後の報告（途中で止まったときは、止まった時点の報告）の両方に示す。状態を読む前に止まったときは、状態を示さない。
- **AC-15**: 状態が「制限が弱い」のときの警告は、`collaborators_only` に変える必要があることを示す。
- **AC-16**: 状態が「確かめられない」なら、`/runpr` は AC-25 のとおり止まり、確かめられなかったことと、利用者が設定画面で確かめる必要があることを示す。警告に応答の本文を含めない。
- **AC-17**: `/runpr` は、F-003 の 4 つの設定を変える操作と、PR を閉じる・作り直す操作を行わない。
- **AC-18**: 期限の判定の境界: `expires_at` が今からちょうど 14 日後のときは「期限が近い」、14 日と 1 秒後のときは「有効」になる。`expires_at` が今と同じ時刻のときは「制限なし」になる。
- **AC-25**: 状態が「確かめられない」「制限なし」「制限が弱い」「制限より前の PR」なら、`/runpr` は、確かめた時点で止まる。Codex のレビューを待たず、`fixpr` を実行せず、マージしない。止まったときの報告は、状態の名前、（あれば）期限の日付、F-003 の手順があるガイドの場所、利用者が行うこと（状態に応じて、制限を確かめて更新し、PR を作り直す。「確かめられない」なら、制限を確かめて実行し直す）を日本語で示す。
- **AC-26**: 状態が「制限より前の PR」のときの報告は、PR を作り直す必要があることを示す。境界: PR の作成の時刻が設定の時刻と同じなら「制限より前の PR」にならず、1 秒前なら「制限より前の PR」になる。
- **AC-27**: 利用者以外のアカウントが書いた、`@codex` のメンションを含む PR の会話のコメント・行コメント・PR レビューが 1 つでもあれば、`/runpr` は、制限の状態にかかわらず、`fixpr` を実行する前とマージの前に、そのコメントの URL を示す。本文は示さない。書いたのが人（`User`）なら止まる。Bot なら、続けるかを利用者に問い、利用者が続けると答えたときだけ続ける。Codex の名義のコメントは対象にしない。

#### F-005: `protocol` の番号を上げる

F-001 で範囲の規則が変わるので、古いツールと新しい文書の組み合わせを止める。

**Acceptance Criteria**:
- **AC-19**: `fixprguard` の `protocol` の番号は `4` から `5` になる。`/fixpr`・`/runpr` の文書とガイドの例は `--protocol 5` を渡す。本タスクの前のツールに `--protocol 5` を渡すと、終了コード `2` で止まる。本タスクの後のツールに `--protocol 4` を渡すと、終了コード `2` で止まる。

#### F-006: 文書を合わせる

規則・自動レビュアーの一覧・残余のリスクの変更を、ガイドと過去のタスクの文書と用語集に反映する。

**Acceptance Criteria**:
- **AC-20**: `docs/dev/guides/fixprguard_guide.md` 6 章の規則 4 と「判定の順序」は、実行関連ファイルが、著者と `path` にかかわらず範囲外であることを示す。
- **AC-21**: ガイドの 7.2（保証しないこと）に、4.2 の残余のリスクがすべて書かれている。
- **AC-22**: 次の文書に、本タスクで決めた変更と本書への参照が加わり、どの文書の状態も `approved` のままである（1 章の決定）。
    - タスク 0030 の `01_requirements.md` F-004 と `02_architecture.md` 3.6（規則 4 の例外を外したこと）
    - タスク 0030 の `02_architecture.md` 5.1・5.3（第三者が自動レビュアーを介してスレッドを作れること、実行関連ファイルが `path` そのものでも範囲外になったこと、`make` に渡す環境を変えないこと、F-003 の設定）
    - タスク 0036 の `01_requirements.md` と `02_architecture.md` のうち、規則 4 の例外を述べる箇所
- **AC-23**: `docs/translation_glossary.md` の「実行関連ファイル」の説明は F-001 の規則に合う。「interaction limits」「作成の制限」「push の制限」の項目が加わっている。
- **AC-24**: `make test`・`make lint`・`doc-guard` が通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   `/runpr` の 1 回の実行で、interaction limits を読む操作は高々 1 回である（AC-12）。

### 4.2. セキュリティ (Security)

条件付きのセキュリティガイド（`docs/dev/security.md`）のトリガーのうち、次のものに当たる。

-   **API キーなどの秘密を扱う:** `make` に渡す環境の秘密の扱いを決める（1 章の決定）。同書 §2（`docs/dev/security.md:16`）の、秘密をログや出力に出さない扱いを、F-004 の警告に当てはめる。警告は、`gh` の資格情報や応答の本文を含めない（AC-16）。
-   **信頼できないテキストをプロンプトに埋め込む:** レビュースレッドの本文がエージェントのプロンプトに入る。同書 §6 の原則「生成結果を他のコマンドやコードとして実行しない」（`docs/dev/security.md:71`）を、タスク 0030 と同じく本タスクにも当てはめる。F-001 は、信頼できないテキストに由来しうる実行関連ファイルの変更を、利用者の確認なしに `make` で使わないようにする。
-   **ネットワークでデータを送る:** `/runpr` が GitHub の API を読む（F-004）。

同書は製品を対象にした文書なので、変えない（2.3）。

**信頼できない入力: interaction limits の取得の応答。** GitHub の `GET /repos/{owner}/{repo}/interaction-limits`（[REST API endpoints for repository interactions](https://docs.github.com/en/rest/interactions/repos)）の応答を、次のとおり扱う。

-   受け入れる形は次の 2 つだけで、1 を先に調べる。
    1.  制限がないことを示す応答。GitHub の REST API の文書は「制限がなければ空の応答」とする。どの HTTP の状態とバイト列を受け入れるかは、実際の応答を観測して設計で 1 つに決める（design_handoff.md の H-01）。決めた形とバイト単位で一致するものだけが当たる。
    2.  HTTP の状態が `200` で、本文が 1 つの JSON のオブジェクトである応答。
-   2 の形で使う項目は `limit` と `expires_at` で、どちらも必須の文字列である。`limit` は `existing_users`・`contributors_only`・`collaborators_only` のどれかである。`expires_at` は、時差を含む RFC 3339 の日時である。ほかの項目（`origin` など）は使わず、拡張可能として無視する。GitHub が項目を加えうるためである。
-   次のものは、受け入れずに「確かめられない」とする。
    -   有効な UTF-8 でない本文、JSON の文字列の対になっていないサロゲートのエスケープ
    -   オブジェクトでない最上位の値、`null`、2 つ目の値や後続のデータ
    -   使う項目の欠落、重複、文字列でない値
    -   一覧にない `limit` の値
    -   解釈できない `expires_at`
    -   64 KiB を超える本文
    -   1 の形でも 2 の形でもない HTTP の状態、取得の失敗、時間切れ（時間の上限は設計で決める）
-   標準ライブラリやツールが黙って修復・破棄・無視する入力（不正なバイトの置き換え、重複した項目の後勝ち、後続のデータの読み残し）は、修復せずに「確かめられない」とする。この規則は、読む処理をどの手段で実装しても守る（design_handoff.md の H-02）。
-   拒否はすべて「確かめられない」の 1 つの状態にまとめる。Go のコードでは 1 つの番兵のエラーにまとめる（名前は設計で決める）。拒否のときに部分的な結果を使わない。

**残余のリスク。** 次の経路は塞がない。ガイドの 7.2 に書く（AC-21）。

-   **制限が外れている間の Codex の経路。** interaction limits の期限が切れてから利用者が更新するまでの間は、書き込み権限のない人が Codex に指示しうる。`/runpr` は止まり（AC-25〜AC-27）、PR の作り直しを求めるが、`/fixpr` を単独で実行する場合は、止まらず警告も出ない。interaction limits を 6 か月より短い期間で設定すると、設定の時刻の見積もりが実際より早くなり、制限のない期間を経た PR を「制限より前の PR」と判定し損ないうる。Codex 名義のスレッドは、信頼する著者のスレッドになる。Codex が外部の人の指示に応じるかは、確かめていない。
-   **`.go` のファイル。** 信頼する著者のスレッドの `path` が `.go` のファイルなら、そのファイル（規則 1）と同じディレクトリの `.go` のファイルは確認なしに範囲に入り、`make test` がそれを実行する。cgo の前文も書ける（1 章の決定）。上の経路でスレッドが作られると、第三者の文言がこの実行に至りうる。F-001 の後も、増幅の段は狭まるだけで閉じない。
-   **Unicode の畳み込みによる別名。** 大文字小文字を区別しない macOS のボリュームでは、`makeﬁle`（U+FB01）のような名前が `Makefile` として開かれうる（`docs/dev/guides/fixprguard_guide.md` 7.2.3）。そのような名前のファイルが PR の diff にあり、信頼する著者のスレッドが付けば、規則 1 で範囲に入る。
-   **`make` が実行するコードが使える秘密。** `make` が実行するコードは、作業ツリーの `.envrc` の秘密、引き継いだ環境変数（`CLAUDE_CODE_MESSAGING_TOKEN`・`SSH_AUTH_SOCK` を含む）、`gh` の資格情報を使える（1 章の決定）。
-   **push の制限の bypass。** Repository admin の役割を持つアカウントは、push の制限を受けない。利用者が bypass に App を加えれば、その App は push できる。
-   **Bot による `main` へのマージ。** push の制限は既定のブランチを対象にしない（1 章の決定）。既存の ruleset（`main`）は直接の push を禁じるが、PR を経たマージは止めない。GitHub App として入っている Bot が、API で PR を `main` にマージすれば、その変更は `main` に入り、利用者が `main` を取り込んだブランチに届く。
-   **build checks のテストが実行するガイドのコード。** `make test` は、ガイド 8.1 の最初の `sh` のブロックを、セッションの環境を引き継いで実行する（1 章の決定）。信頼する著者のスレッドの `path` がガイドか、同じディレクトリの `.md` なら、修正のフェーズが書き換えたブロックを、`finish` が確認なしに実行する。
-   **設定を外した場合と、設定する前の期間。** `/runpr` が確かめるのは interaction limits だけで、作成の制限と push の制限が外されても警告しない。また、利用者が F-003 の設定を行うまで（遅くとも本タスクの最後の PR を開くまで）は、これまでどおり経路が開いている。
-   **名指しした CodeRabbit のスレッド。** 利用者が名指しすれば、その本文はエージェントに届く。名指しの前に、利用者が内容を確かめる。
-   **確認への慣れ。** 実行関連ファイルの変更は、正当な修正でも毎回確認に回る。確認に慣れると、危険な変更を見落としうる。
-   **新しい自動レビュアー。** 自動レビュアーの一覧に Bot を加えるときは、その Bot が外部の人の指示に応じるかを評価する（#405）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   interaction limits を読めないことや時間切れで、`/runpr` は止まる（AC-16・AC-25）。GitHub の障害の間や、`gh` の出力の形が変わって確かめられない状態が続く間は、PR を進められない。このときは、利用者が GitHub で PR を確かめてから `/mergepr` で直接マージできる（`/runpr` を経ない、今までの手段）。止まらない案は、確かめられない間に期限切れの間のスレッドを処理しうるので採らない（1 章の決定）。

### 4.4. 互換性 (Compatibility)

-   古いツールと新しい文書、新しいツールと古い文書の組み合わせは、どちらも終了コード `2` で止まり、古い規則で判定しない（AC-19）。
-   すでに PR にある CodeRabbit のスレッドは、本タスクの後の `/fixpr` では想定外の著者のスレッドになる（AC-06）。

### 4.5. 保守性 (Maintainability)

-   自動レビュアーの一覧は、今までどおり `.claude/commands/_context.md` の 1 か所で定める。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md`「Tool Execution Safety」に従う。interaction limits を読む操作はネットワークの操作なので、`/runpr` の「Authorization」に挙げる（AC-12）。F-003 の設定を変える操作は、エージェントは行わない（AC-17）。
-   外部のモジュールを加えない（`CLAUDE.md`「Dependencies」）。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。本書で新たに使う用語は次のとおり。

-   **interaction limits:** GitHub のリポジトリまたはアカウントの設定で、許可した範囲の外のアカウントによる、コメント・issue・PR の作成を一定の期間制限するもの。`collaborators_only` は、書き込み権限を持つ人（collaborators）以外のアカウントを制限する。期限は最長 6 か月で、期限が来ると自動的に解除される。
-   **作成の制限:** PR の作成と issue の作成を collaborators に限る、期限のないリポジトリの設定。コメントは制限しない。
-   **push の制限:** 既定のブランチを除くすべてのブランチを対象にした ruleset で、ブランチの作成と更新を bypass の権限を持つ相手（Repository admin）に限る設定。

既存の用語（信頼する著者、信頼する著者のスレッド、想定外の著者のスレッド、自動レビュアー、自動レビュアーの一覧、実行関連ファイル、名指し）は、用語集の定義に従う。本書は「名指しされた想定外の著者のスレッド」を略さずに書く。

## 変更履歴

- 2026-11-02 承認（isseis、#631）
- 2026-11-03 draft への戻し：push の制限の対象から既定のブランチを外し、interaction limits をアカウント全体の設定とし、build checks のテストが実行するガイドのコードを残余のリスクに加えた；制限が外れているか確かめられないとき、PR が制限の設定より前に作られたとき、利用者以外の @codex のメンションがあるときに、/runpr を止めるようにした（#634）
- 2026-11-03 承認（isseis、#634）
