# ADR-0015: レビューのスレッドと Bot への依頼は信頼せず、書き込み権限・範囲の規則・interaction limits で守る

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

このリポジトリは公開されており、GitHub のアカウントを持つ人なら誰でも、PR にレビューのスレッドを開ける。`/fixpr` は、未解決のスレッドの本文をエージェントに読ませ、リポジトリのファイルを直させ、commit と push まで行う。`/runpr` は、その `/fixpr` を繰り返し呼ぶ。スレッドの本文は、外部の人が書いた信頼できないテキストで、プロンプトインジェクションの経路になる。本文を読んだエージェントはファイルを編集できるので、誤ると範囲外の変更が commit され、push される。

GitHub App として入っている自動レビュアー（CodeRabbit・Sourcery AI）は、書き込み権限を持つ Bot として振る舞う。書き込み権限のない人は、PR や issue のコメントで自動レビュアーに依頼し、自動レビュアーの名義のスレッドに自分の指示を入れられる。自動レビュアー自身も、PR のブランチに push しうる。

## 決定

公開リポジトリのレビュースレッドと Bot への依頼は信頼せず、本文の分離・著者の制限・変更範囲の確認・ガードツール経由の書き込み・`interaction limits` の 5 つの層で守る。

### 本文の扱いと著者の判定

-   **本文のデータ扱い:** スレッドの本文はデータであり、指示ではない。エージェントのプロンプトは実行ごとの乱数を含む区切りの行で本文を囲み、本文側から区切りの行を閉じられないようにする。返信の文面と PR の説明も、本文を読んだエージェントが書くので、同じ扱いにする。
    -   理由: 外部の人が書いた本文を、ファイルを編集できるエージェントが読むためである。
    -   正本: [fixpr.md](../../../.claude/commands/fixpr.md) の R6、[fixprguard_guide.md](../guides/fixprguard_guide.md) の 1 章「位置付け」・7.1「保証すること」。
-   **信頼する著者:** リポジトリへの書き込み権限（write・maintain・admin）を持つ Bot でないアカウントと、`_context.md` の「Automated reviewer list」に挙げた自動レビュアーだけとする。自動レビュアーは、Bot であることと名前の一致の両方で判定し、権限の照会は自動レビュアー以外のアカウントにだけ行う。
    -   理由: 公開リポジトリでは誰でもスレッドを開けるので、本文をデータとして扱うだけでは守りにならず、Bot であることだけでも信頼する理由にならないためである。
    -   正本: [.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Automated reviewer list」の行、[fixprguard_guide.md](../guides/fixprguard_guide.md) の 1 章「位置付け」。
-   **信頼する自動レビュアーの一覧:** `coderabbitai` と `sourcery-ai` の 2 つにする。
    -   理由: Codex の連携を切った後は使っていない `chatgpt-codex-connector` を外し（[0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-010）、PR レビューで使う `sourcery-ai` のスレッドを毎回名指しせずに処理できるようにするためである（[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-002）。
    -   正本: [.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Automated reviewer list」の行。
    -   **食い違い:** `_context.md` とガードテスト（`cmd/yt2column/fixpr_docs_test.go`）はまだ `coderabbitai` のみである。この食い違いは https://github.com/isseis/yt2column/issues/1095 の実装で解消する。
-   **スレッド全体の判定:** スレッドのすべてのコメントの著者を確かめ、1 件でも想定外の著者があれば、スレッド全体を想定外とする。コメント数が上限を超えるスレッドと取得できないコメントがあるスレッドも想定外とする。想定外のスレッドはトリアージ・修正・返信・解決をせず、本文を取得せず、`/runpr` を含むどのセッションにも渡さない。URL と理由だけを報告し、利用者が内容を確かめて名指ししたものだけを処理する。
    -   理由: 想定外の著者のコメントだけを除くと会話の文脈が欠けて指摘を誤解するおそれがあり、トリアージだけでも本文がエージェントに渡るためである。
    -   正本: [fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章 `begin`・7.1「保証すること」、[.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 3。

### 変更範囲と書き込みの制限

-   **修正の範囲:** スレッドの `path` そのもの（許可リストは当てない）に加え、同じディレクトリおよびその `path` が属するタスクディレクトリ内で許可リスト（`*.md`・`*.txt`・`testdata/` 以下のファイル・`.go` の規則に該当するファイル）に該当するファイルに限る。ただし実行関連ファイルは、スレッドの著者と `path` にかかわらず範囲外とする。範囲外の変更が出たら build checks・commit・push の前に利用者に一覧を示して確かめ、commit は判定したパスだけを stage する。ローカルの CLI レビューの修正は対象外とする。
    -   理由: `path` そのものに許可リストを当てると `.ts` や `.sh` の修正のたびに確認が入る一方、信頼する著者のスレッドを経ても指示は届きうるので、著者の確認とは別の守りを残し、コードを実行する build checks の前に確認するためである。
    -   正本: [fixprguard_guide.md](../guides/fixprguard_guide.md) の 6 章・7.1「保証すること」、[.claude/commands/fixpr.md](../../../.claude/commands/fixpr.md) の Phase 3.5・Phase 4。
-   **ガードツール経由の書き込みとメンション拒否:** GitHub のレビュースレッドへの返信・スレッドの解決・PR の説明の更新はガードツールの `reply`・`edit-pr`・`append-pr` で行い（シェルを介さず標準入力で本文を渡す）、`push` もガードツール、PR 作成は `gh`、マージは `mergepr` が行う。`@codex`・`@coderabbitai`・`@sourcery-ai`・`@sourceryai` のいずれかを含む文面は、返信・PR のタイトルと説明の更新・コミットを作成する `finish` のコミット本文では書き込み前に拒否する。コマンドが投稿する文面は、手動の PR レビューを頼む依頼コメント（`@coderabbitai review`・`@sourcery-ai review`）だけがメンションを含み、ほかは投稿前に 4 つのメンションのいずれもないことを確かめる（[ADR-0014](0014-deterministic-checks-in-go.md)）。
    -   理由: 本文を読んだエージェントが書く文面をシェルに渡すとコマンドとして実行されうるほか、利用者アカウントの投稿が自動レビュアー（CodeRabbit・Sourcery AI。Sourcery AI は `@sourcery-ai` と `@sourceryai` の両方に反応する）を動かすのを防ぐためである。`@codex` を残すのは費用がほとんどないためである（[0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md) の F-009、[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-003）。
    -   正本: [fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章「サブコマンド」・7.1「保証すること」、[.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Rejected mentions」の行、[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」。
    -   **食い違い:** `fixprguard`・`_context.md`・コマンド文書の拒否するメンションはまだ `@codex` と `@coderabbitai` の 2 つであり、手動の PR レビューの依頼文面も `@coderabbitai review` のみである。この食い違いは https://github.com/isseis/yt2column/issues/1095 の実装で解消する。
-   **fix round の push と Phase 4.5 の状態比較:** `/fixpr` の Phase 4.5（ローカルの CLI レビューの修正後）を含め、fix round の push はすべてガードツールが行う。Phase 4.5 では手順が定める git 起動と push の前に git の状態を開始時（`begin`）に記録した基準値と比べ（範囲検査と判定した集合との一致確認は行わない）、違えば止める。基準値を持たない呼び出し元は基準値の入力を渡さない。修正を確定する Phase 4 のコミットでは、セッションからの引数を信じずガードツールが git の状態から対象を求める。
    -   理由: Phase 4 の後に書き換えられた hooks や config を実行させないためである。Phase 4.5 にはスレッドの `path` も、セッションとは別に変更対象を判定する仕組みもなく、セッション自身も信頼できないテキストの影響を受けうる。
    -   正本: [fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章 `check`・`push-review`・7.1「保証すること」、[local-cli-review.md](../../../.claude/commands/_lib/local-cli-review.md)、[0076 要件定義書](../../tasks/0076_fixpr_local_review_guard/01_requirements.md) §3、[0076 設計書](../../tasks/0076_fixpr_local_review_guard/02_architecture.md) §3。
-   **fork の PR の拒否:** fork の PR では、`/fixpr` と `/runpr` は何もせずに止まる。
    -   理由: head を外部の人が管理すると、チェックアウトした `CLAUDE.md` や `.claude/commands/` そのものが攻撃者のものになり、本 ADR のどの守りも働かないためである。
    -   正本: [.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 1、[fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章 `head`・`begin`。

### GitHub の制限と他アカウントのメンション検査

-   **`interaction limits` とメンション検査:** 4 つの設定（アカウント全体の `interaction limits`、PR 作成の制限、issue 作成の制限、ブランチへの push の制限）を利用者が設定する。`/runpr` は `fixprguard interaction-limits` で制限の状態を読み、PR の作成時刻と比べ、利用者以外の `@codex`・`@coderabbitai`・`@sourcery-ai`・`@sourceryai` のメンションがないことを確かめる（CodeRabbit の観測済み定型コメント 2 種と、`sourcery-ai[bot]` の観測済みレビューガイドコメントは除外する）。確かめられない・制限がない・制限が弱い・PR が制限設定より前に作られたときは、レビューを待たずに止まる。
    -   理由: 書き込み権限のない人が自動レビュアーに指示する入口を塞ぎ、期限で自動解除される `interaction limits` が外れていた間に作られたスレッドを見逃さないためである。自動レビュアー自身の観測済み定型コメントを除外するのは、通常の PR ごとに誤検知で止まらないようにするためである（[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md) の F-003）。
    -   正本: [fixprguard_guide.md](../guides/fixprguard_guide.md) の 10 章「GitHub の設定」、[.claude/commands/runpr.md](../../../.claude/commands/runpr.md) の step 1・step 3。
    -   **食い違い:** `runpr.md` step 3 のメンション検査はまだ `@codex` と `@coderabbitai` の 2 つを対象とし、除外は CodeRabbit の定型 2 種のみである。この食い違いは https://github.com/isseis/yt2column/issues/1095 の実装で解消する。
-   **制限状態の厳密判定:** 制限状態の判定は、GitHub の応答を厳密に検査する Go のサブコマンドで行い、「今」は同じ応答の `Date` ヘッダ（GitHub の時刻）から取る。応答の検査は 1 つの番兵エラーにまとめ、部分的な結果を使わない。
    -   理由: `gh api --jq` は重複した項目を黙って後勝ちにするため厳密に検査できず、ローカルの時計では時計のずれがそのまま判定のずれになるためである。
    -   正本: [fixprguard_guide.md](../guides/fixprguard_guide.md) の 5 章 `interaction-limits`・10 章「GitHub の設定」、[0057 設計書](../../tasks/0057_reviewer_relay_guard/02_architecture.md) §1.2。

## 採らなかった案

採らなかった案は、プロンプト任せや部分的な処理で守りを弱める案、日常運用の確認負担を過度に増やす案、および厳密な判定を欠く案である。

### 著者判定・範囲規則・ツール設計

-   本文をデータとして明示するだけで、著者を確かめない。公開リポジトリでは誰でもスレッドを開けるので、プロンプトの書き方だけでは守りにならない（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1）。
-   想定外の著者のスレッドもトリアージだけ行う。トリアージだけでも本文がエージェントに渡るので、守りにならない（同上）。
-   `/fixpr` の実行中に、スレッドごとに利用者へ聞く。`/runpr` の自動の繰り返しのたびに利用者を待たせることになる（同上）。
-   想定外の著者のコメントだけを除いて処理する。会話の文脈が欠けて指摘を誤って理解するおそれがある（同上）。
-   範囲外の変更があれば止める。コードの修正では別のパッケージを変えることもあり、正当な修正まで止まる（同上）。
-   修正の範囲をスレッドの `path` のファイルだけに限る。同じパッケージのテストを直すたびに確認が入り、`/runpr` の繰り返しのたびに利用者を待たせる（同上）。
-   範囲外の変更を捨てる。何が書き換えられたかを利用者が確かめられなくなる（同上）。
-   会話コメントの著者まで確かめて止める。自動レビュアーが何をもとにスレッドを書いたかは会話コメントの著者だけからは決まらないため、本文をデータとして扱うことと範囲の確認で抑え、残余リスクとして記録する（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) §1・§4.2）。
-   `--reviewer` を名前の数だけ文書に書き、一覧を変えるたびに文書を変える。1 つのプレースホルダーを名前ごとに繰り返す形とすれば、渡し忘れても安全側に倒れる（[0033 設計書](../../tasks/0033_multiple_trusted_reviewers/02_architecture.md) §1.2）。
-   範囲の規則の変更で `--protocol` を上げない。古いガードツールと新しい文書が食い違ったまま、違う規則で黙って判定しうる（[0036 要件定義書](../../tasks/0036_fixprguard_exec_file_scope/01_requirements.md) §1）。
-   スレッドの `path` そのもの（規則 1）にも許可リストを当てる。スレッドが指す `.ts` や `.sh` のファイルを直すたびに利用者の確認が入り、`/runpr` の繰り返しのたびに利用者を待たせる（[0082 要件定義書](../../tasks/0082_adr0015_fix_scope_path/01_requirements.md) §1）。
-   ローカルの CLI レビューの commit と push の前に比べるだけで、push は `git push` のままにする。push 先の OID 確認と明示した refspec が付かず、`push` をガードツールが行う決定と食い違う（[0076 要件定義書](../../tasks/0076_fixpr_local_review_guard/01_requirements.md) §1）。
-   ローカルの CLI レビューの修正も `inspect` → `finish` → `push` に通す。スレッドがないため新しい範囲の規則が要り、`finish` のコミットメッセージ規則とも合わない（同上）。
-   既存の `push` が HEAD と `--commit` の一致で Phase 4 と Phase 4.5 を見分ける。呼び出し元のデータから振る舞いを選ぶことになり、型や引数で振る舞いを宣言する方針に反する（[0076 設計書](../../tasks/0076_fixpr_local_review_guard/02_architecture.md) §1.2）。
-   共有の手順を変えず、`fixpr.md` から共有手順の各 step の前に状態を比べるよう指示する。共有手順の構成が変わっても気づかれずずれる（同上）。

### 制限設定と自動レビュアー・メンションの扱い

-   すべてのブランチを push の制限の対象にする。PR のマージ（`main` の更新）も制限に当たり `/mergepr` の `gh pr merge` が止まる。`main` への直接 push は既存の ruleset がすでに禁じている（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md) §1）。
-   `interaction limits` を `gh api --jq` で読む。`jq` は重複した項目を黙って後勝ちにするので厳密に検査できない（[0057 設計書](../../tasks/0057_reviewer_relay_guard/02_architecture.md) §1.2）。
-   制限の状態をローカルの時計で判定する。時計のずれが判定のずれになる。同じ応答の `Date` ヘッダを使えば 1 回の取得で制限と時刻がそろう（同上）。
-   制限が外れているか確かめられないときに警告して処理を続ける。制限が外れていた間に自動レビュアーが作ったスレッドは本来のレビューと見分けられず、手作業で探すと見逃しうる（同上）。
-   疑わしいコメントを一律に削除する。正当なレビューも消え、取り消せない（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md) §1）。
-   信頼する自動レビュアーの一覧から CodeRabbit を外す。Web から CodeRabbit のレビューを頼む使い方を残したいうえ、書き込み権限のない人は制限が有効な間はどちらの Bot にも指示できないため、一覧から外しても減る危険は小さい（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md) §1「実装の際に決めたこと（2026-11-04）」）。
-   拒否するメンションに `@sourcery-ai` だけを加えて `@sourceryai` を入れない、またはどちらも加えない。公式文書と PR #1094 の観測から Sourcery AI は `@sourceryai`（ハイフンなし）でも起動するため、どちらか一方でも拒否・検出対象から外すと誤投稿の抑止や外部メンションの検出に穴が残る（[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md)「要件定義の際に決めたこと（2026-11-23）」）。

## 結果

本決定により、想定外の著者のスレッドの遮断と書き込み・変更範囲の多層防御を維持しつつ、CodeRabbit と Sourcery AI のスレッドを安全に自動処理できる。

-   **想定外の著者のスレッドの遮断:** 想定外の著者が開いたスレッドは、利用者が内容を確かめて名指しするまで処理されず、本文は `/fixpr` にも `/runpr` のセッションにも渡らない。
-   **多層防御:** 信頼する著者のスレッドでも、本文を読んだエージェントが指示に従うことは保証できないため、範囲の確認とガードツール経由の書き込みを別の守りとして重ねる。修正フェーズの範囲外の変更は、コードを実行する build checks の前に利用者の確認に回る。
-   **`interaction limits` の運用:** アカウント全体に効き、ほかの公開リポジトリにも及ぶ。期限は最長 6 か月で切れる前に利用者が更新する。`/runpr` は 4 つの設定のうち `interaction limits` だけを読み、制限がない・弱い・確かめられない・PR が設定より前に作られたときはマージしない。PR・issue 作成の制限とブランチへの push の制限は利用者が設定画面で確かめる。
-   **自動レビュアーの一覧とメンション拒否:** 信頼する自動レビュアーの一覧は CodeRabbit と Sourcery AI の 2 つになり、両者のスレッドを `/fixpr` と `/runpr` が自動処理できる。4 つの拒否するメンション（`@codex`・`@coderabbitai`・`@sourcery-ai`・`@sourceryai`）はコマンド文書と `CLAUDE.md` で許可された行以外に書けず、`/runpr` は両社の観測済み定型コメントだけをメンション検査から除外する。
-   **Phase 4.5 の残余リスク:** ローカルの CLI レビューの修正は範囲外のファイルを変えても利用者の確認に回らず、比較が検出するのは git の状態（hooks・config・info・対象のファイル・ブランチ・HEAD の系列）の書き換えだけである。

## 経緯

- 2026-10-21 `/fixpr` と `/runpr` で、スレッドの本文を信頼できないデータとして扱い、すべてのコメントの著者を書き込み権限と自動レビュアーの一覧で判定することにした。想定外のスレッドは処理せず、修正の範囲を `path` の周辺に限り、範囲外の変更は commit の前に利用者へ確かめる（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md)）
- 2026-10-21 信頼する自動レビュアーを複数名にできるようにし、`--reviewer` の一覧を `_context.md` の 1 か所に置いた。一覧の名前は GraphQL の `author.login` の形で照合する（[0033 要件定義書](../../tasks/0033_multiple_trusted_reviewers/01_requirements.md)）
- 2026-10-22 範囲の規則で、実行関連ファイルをスレッドの著者と `path` にかかわらず範囲外とし、`.go` を許可リストに移した。規則を変えたので `--protocol` を `3` に上げた（[0036 要件定義書](../../tasks/0036_fixprguard_exec_file_scope/01_requirements.md)）
- 2026-11-02 外部の人が自動レビュアーを介して指示を届ける経路を塞ぐため、Codex を外部の指示に応じうるものとして扱い、interaction limits と PR・issue の作成の制限、ブランチの push の制限を設けた。実行関連ファイルは信頼する著者のスレッドでも範囲外とし、`/runpr` が制限の状態と PR の作成の時刻、利用者以外のメンションを確かめることにした（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md)）
- 2026-11-03 interaction limits をアカウント全体の設定とし、制限が外れているとき、弱いとき、確かめられないときは、警告して続けずに止めることにした。push の制限の対象から `main` を外し、PR のマージを止めないようにした（[0057 設計書](../../tasks/0057_reviewer_relay_guard/02_architecture.md) §1.2）
- 2026-11-04 信頼する自動レビュアーの一覧に CodeRabbit を残し、一覧から外す要件（F-002）を取り下げた。Web から CodeRabbit のレビューを頼む使い方を残すためである（[0057 要件定義書](../../tasks/0057_reviewer_relay_guard/01_requirements.md) §1「実装の際に決めたこと（2026-11-04）」）
- 2026-11-09 `/fixpr` のローカルの CLI レビューの push も道具が行い、手順が定める git の起動と push の前に git の状態を基準値と比べることにした。範囲の検査と、判定した集合との一致の確認は行わない（[0076 要件定義書](../../tasks/0076_fixpr_local_review_guard/01_requirements.md)）
- 2026-11-10 Phase 4.5 の比較は道具の新しいサブコマンド `check` に、push は `push-review` に行わせ、比べる呼び出しの多くは、呼び出し元が渡す入力としてローカルの CLI レビューの共有の手順に加えることにした（[0076 設計書](../../tasks/0076_fixpr_local_review_guard/02_architecture.md)）
- 2026-11-10 修正の範囲の決定を、規則 1（スレッドの `path` そのもの）に許可リストを当てず、実行関連ファイルでなければ名前や拡張子を問わず範囲に入れる形に直した。タスク 0036 の決定と道具の実装に合わせたもので、本 ADR を作ったときの要約が、`path` そのものにも許可リストが当たる書き方になっていたためである（[0082 要件定義書](../../tasks/0082_adr0015_fix_scope_path/01_requirements.md)）
- 2026-11-20 利用者のアカウントからの投稿で拒否するメンションと、`/runpr` が確かめる利用者以外のメンションを、`@codex` と `@coderabbitai` の 2 つにした。手動の PR レビューの依頼 `@coderabbitai review` だけがメンションを含む。信頼する自動レビュアーの一覧から `chatgpt-codex-connector` を外した（[0118 要件定義書](../../tasks/0118_codex_removal/01_requirements.md)「要件定義の際に決めたこと（2026-11-20）」）
- 2026-11-23 信頼する自動レビュアーの一覧に `sourcery-ai` を加え、拒否するメンションと `/runpr` が確かめるメンションに `@sourcery-ai` と `@sourceryai` を加えて 4 つにし、`/runpr` のメンション検査から `sourcery-ai[bot]` の観測済みレビューガイドコメントを除外することにした（[0122 要件定義書](../../tasks/0122_sourcery_trusted_reviewer/01_requirements.md)「要件定義の際に決めたこと（2026-11-23）」）
