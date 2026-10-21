# 要件定義書：PR レビューを CodeRabbit から Codex の GitHub 連携に切り替える案の評価

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-21 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、PR レビュー（GitHub の PR 上で自動レビュアーが行うレビュー）を CodeRabbit から Codex の GitHub 連携に切り替える案を評価し、評価の結果に従って切り替えるための要件を定義する（[#403](https://github.com/isseis/yt2column/issues/403)）。評価は 3 段階で行う。試験運用の観測、同じ PR での CodeRabbit との比較、継続するかの判断である。ローカルの CLI レビュー（`coderabbit review`）は、CodeRabbit のまま残す。

**背景:**

CodeRabbit の PR レビューには、時間あたりの上限がある。Essentials のプランでは、1 開発者あたり 1 時間に 5 回である。手動の `@coderabbitai review` も数える（issue #403）。この上限のため、PR レビューは PR を開いたときの 1 回だけにし、`.coderabbit.yaml` で増分レビューを止めている（`.coderabbit.yaml:9-14`）。以後の変更はローカルの CLI でレビューする（`CLAUDE.md:99-104`）。2 回目以降の PR レビューは、エスカレーションの判断か利用者の依頼があったときだけ依頼する（`.claude/commands/_lib/pr-review-escalation.md:14-18`）。それでも、PR レビューの割り当てそのものがボトルネックである点は変わらない。

現在の `/runpr` は、PR レビューを CodeRabbit 専用の方法で扱う。

-   **依頼:** `gh pr comment <number> --body "@coderabbitai review"` で依頼する（`runpr.md:187-196`）。PR を開くコマンドも、同じ方法で依頼する（`_lib/task-pr-handoff.md:116-121`）。
-   **レビュー済みの判定:** 次の 2 つがそろったとき、そのコミットをレビュー済みとする（`runpr.md:153-185`）。PR の `StatusContext` のうち `CodeRabbit` のものが `SUCCESS` であること。CodeRabbit の PR レビューの本文か要約コメントに、「between `<sha>` and `<headRefOid>`.」の行があること。応答がなければ、依頼から約 15 分で止めて報告する（`runpr.md:192-196`）。
-   **ラウンド上限:** `/fixpr` のラウンド上限は、スレッドの最初のコメントの著者が `Bot` である場合だけに働く（`fixpr.md:42-47`・`fixpr.md:183`）。

Codex の GitHub 連携の公式文書から分かったことは、次のとおりである（issue #403）。手動の依頼は PR コメントの `@codex review` で行い、個人設定で自動レビュー（Automatic review）を有効にできる。投稿者は `chatgpt-codex-connector` ボットで、ファイル・行に紐づくコメントを標準の GitHub レビューとして投稿する。GitHub には P0 と P1 の指摘だけを投稿する。レビューの規則は `AGENTS.md` で変えられる。ChatGPT のプランが必要で、API キーの契約では使えない。GitHub 経由のレビューの割り当ての数値は、公式文書に記載がない。

`AGENTS.md` はすでにリポジトリにあり、`## Review guidelines` の節を持つ（`AGENTS.md:5`）。issue は `AGENTS.md` を新規に作り、`## Code Review Rules` の節を置くとしている。

対応 issue: [#403](https://github.com/isseis/yt2column/issues/403)（前提: [#405](https://github.com/isseis/yt2column/issues/405)、その前提: [#395](https://github.com/isseis/yt2column/issues/395)（タスク `0030_fixpr_untrusted_threads`）。関連: [#394](https://github.com/isseis/yt2column/issues/394)）

**前提の関係:** #395 は、信頼する自動レビュアーを `_context.md` の 1 つの名前（`coderabbitai`）で判定する（`docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md` の AC-01・AC-35）。このままでは、Codex が開いたスレッドは信頼されず、`/fixpr` は自動では処理しない。名前を複数持てるようにする対応は #405 で行う。依存関係は #395 → #405 → #403 である。本書の F-002 から F-006 は、#405 の完了を前提にする。F-001（試験運用の観測）は、#405 を待たずに行える。#405 は F-001 の観測値（AC-03）をもとに設計する。

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。何を観測して記録するか、いつ誰にレビューを依頼するか、何をもってレビュー済みとするか、比較の期間に何をするか、継続するかの判断をどう記録し、その後どうするか、である。次の事項は設計（`02_architecture.md`）で決める。設計は、F-001 の観測の記録がそろってから始める。

-   Codex のレビュー済みを判定する具体的な手段（どの API のどの値を見るか）。
-   比較の期間かどうかを、コマンドがどこから知るか。
-   観測と比較の記録の文書の名前と形式。
-   コマンドの文書のどこに何を書くか。

要件の起草とレビューで挙がった設計の事項は、[design_handoff.md](design_handoff.md) に申し送る。

**issue #403 で決めたこと:**

-   **切り替えるのは PR レビューだけ:** ローカルの CLI レビューは CodeRabbit のまま残す。CLI の割り当ては PR レビューの割り当てと別であり、`--base-commit`・構造化出力・`findings --clear` など、今の手順がそのまま使えるためである。
-   **エスカレーションの方針は残す:** PR レビューは PR を開いたときの 1 回だけとし、2 回目以降はエスカレーションの判断か利用者の依頼で行う。Codex 側の割り当てを実測するまでは、割り当てに余裕があると見なせないためである。
-   **先に試験運用で観測する:** 未確認項目 1〜5 を観測して記録してから、コマンドを変える。判定の方法が観測の結果で決まるためである。
-   **CodeRabbit と比べてから継続を決める:** 指摘の質（見落とし・誤検知）、所要時間、割り当ての消費を比べる。
-   **スレッドの著者の値を記録する:** GraphQL の `author.__typename` と `author.login`、REST の `user.type` と `user.login` を記録する。CodeRabbit では、GraphQL の `login`（`coderabbitai`）と REST の `login`（`coderabbitai[bot]`）の形が違ったためである。#405 はこの値をもとに設計する。
-   **待つ PR レビュアーを信頼の一覧と分ける:** どの Bot の PR レビューを待つかは、信頼する自動レビュアーの一覧（#405）とは別に定める。REST の名前は直接書かず、`_context.md` から作る（0030 の AC-35 に合わせる）。役割が違うためである。信頼の一覧は「誰のスレッドを処理するか」を決め、待つ PR レビュアーは「何をもってレビュー済みとするか」を決める。

**要件定義の際に決めたこと（2026-10-21）:**

-   **観測は設計の前に行う:** 要件では観測の項目と記録の内容を定め、観測そのものは `02_architecture.md` を書く前に済ませる（F-001）。試験運用はまだ行っていない。判定の手段は観測の結果で決まるので、観測の前に設計すると書き直しになるためである。
-   **比較は同じ PR で両方にレビューさせて行う:** 比較の期間は、同じ PR を CodeRabbit と Codex の両方にレビューさせる（F-004）。同じ変更に対する指摘を並べなければ、見落としを直接比べられないためである。切り替えの後の Codex と過去の CodeRabbit を比べる案と、PR ごとに交互に使う案は採らない。割り当ては二重に使わずに済むが、PR が違うので見落としを比べられないためである。
-   **比較の期間は、マージの前に両方のレビューを待つ:** 比較の期間の `/runpr` は、CodeRabbit と Codex の両方のレビューが済み、両方のスレッドを処理してからマージの質問に進む（F-004）。Codex だけを待つ案は採らない。CodeRabbit の指摘がマージの後に届くと、処理されないまま残るためである。
-   **比較する PR は 5 件:** そのうち、文書だけを変える PR とコードを変える PR を、それぞれ 1 件以上含める（F-004）。種類によって指摘の傾向が違いうるためである。
-   **継続は記録を読んで人が判断する:** 比較の記録を読んで、人が継続するかを判断し、判断とその理由を記録する（F-005）。数値の基準は事前に決めない。5 件では、基準の値を決める根拠が乏しいためである。
-   **継続しないときは、切り替えの PR を revert する:** 判定の方法は Codex 用の 1 つだけにし、両方の判定の方法を残して切り替えられるようにはしない（F-005）。使わない経路を `runpr.md` に残して保守し続けることになるためである（`CLAUDE.md` の YAGNI）。
-   **Codex の自動レビューは使わない:** Codex の個人設定の Automatic review は有効にしない。レビューは `@codex review` の依頼でだけ行う（F-002）。自動レビューが push のたびに走ると（未確認項目 2）、「PR を開いたときの 1 回だけ」の方針と割り当ての節約が崩れるためである。
-   **`AGENTS.md` は既存のものを更新する:** 新規には作らない。Codex が読む節の見出しは、試験運用で確かめてから決める（F-001・F-006）。`AGENTS.md` はすでにあり、`## Review guidelines` の節を持つ（`AGENTS.md:5`）が、issue は `## Code Review Rules` としているためである。
-   **Codex のレビューが届かないときは止めて報告する:** レート制限の通知や、依頼の後に応答がない場合は、レビュー済みとせず、止めて報告する（F-003）。CodeRabbit に自動で切り替える案は採らない。両方の判定の方法が必要になり、継続しないときの戻し方（revert）の決定と矛盾するためである。
-   **観測と比較の記録はタスクのディレクトリに置く:** 本タスクのディレクトリの下の文書に記録する（F-001・F-004・F-005）。設計と同じ PR の流れでレビューでき、判断の根拠がリポジトリに残るためである。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   Codex の PR レビューの振る舞い（未確認項目 1〜5）を観測して記録し、判定の方法を観測の結果に基づいて決められるようにする。
-   PR レビューを Codex で行い、`/runpr` が初回のレビューの完了を正しく判定できるようにする。
-   同じ PR での CodeRabbit との比較の結果を記録し、継続するかを判断する。

### 2.2. スコープ (In Scope)

-   試験運用での観測と、その記録（F-001）。
-   Codex への PR レビューの依頼（F-002）。
-   Codex のレビュー済みの判定と、レビューが届かないときの停止（F-003）。
-   比較の期間の振る舞いと、比較の記録（F-004）。
-   継続するかの判断の記録と、判断の後の変更（F-005）。
-   文書: `AGENTS.md`・`CLAUDE.md`・`.coderabbit.yaml`・`.claude/commands/_context.md`・`runpr.md`・`_lib/task-pr-handoff.md`・`_lib/pr-review-escalation.md`・`fixpr.md`（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   ローカルの CLI レビュー（`_lib/coderabbit-local-review.md`）。issue で CodeRabbit のまま残すと決めたためである。
-   信頼する自動レビュアーを複数の名前にすること（`_context.md` の一覧、`fixprguard` の `--reviewer`、文書のガードのテスト）。#405 で行う。
-   CodeRabbit のレート制限の通知を、CodeRabbit のレビュー済みと数えない修正。#394 で行う。比較の期間に CodeRabbit のレビューを待つときは、今の CodeRabbit の判定（`runpr.md:153-185`）をそのまま使う。
-   Codex CLI の導入、`codex login`、リポジトリの Codex 連携とコードレビューの有効化。リポジトリの外の設定であり、利用者が行う（issue #403 の「進め方 1」）。
-   エスカレーションの方針の見直し（初回のみの方針をやめること）。Codex の割り当てを実測した後に、別の issue で扱う。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 試験運用での観測と記録

実際の PR で `@codex review` を依頼し、Codex の振る舞いを観測して記録する。この記録が設計の入力になる。

**Acceptance Criteria**:
- **AC-01**: 次の観測の結果が、本タスクのディレクトリの下の文書に記録されている。
    1.  指摘が 0 件のとき、Codex が何を投稿するか（PR レビュー、コメント、リアクションだけ、など）。
    2.  依頼していない push で、Codex が再びレビューするか（自動レビューが無効のとき）。
    3.  Codex のスレッドの著者の値（AC-03）。
    4.  ステータスコンテキストやチェックランが付くか。付く場合、名前と状態の値と、branch protection に影響するか。
    5.  1 時間あたりに受け付けるレビューの回数（上限に達するまで観測できなければ、観測した範囲の回数と、上限に達しなかったこと）と、上限に達したときに投稿される通知の文面。
    6.  Codex のレビューの規則として読まれる、`AGENTS.md` の節の見出し。
- **AC-02**: 各観測の記録に、観測した PR の番号、日付、依頼したコミットの SHA、根拠（API の応答の該当部分か、PR の画面で見た内容）が含まれている。
- **AC-03**: 指摘が 1 件以上あるレビューについて、スレッドの最初のコメントの著者の値として、GraphQL の `author.__typename` と `author.login`、REST の `user.type` と `user.login` が記録されている。
- **AC-04**: 指摘が 0 件のレビューと 1 件以上のレビューを、それぞれ 1 回以上観測している。
- **AC-05**: 観測の結果が本書の要件と矛盾する場合（例: 自動レビューが無効でも push のたびにレビューされる、Codex のスレッドの著者が `Bot` でない）、その点が記録に挙げられ、本書の該当する要件が見直されるまで設計を始めない。

#### F-002: Codex への PR レビューの依頼

PR レビューは、Codex に `@codex review` で依頼する。

**Acceptance Criteria**:
- **AC-06**: PR を開くコマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2`・`/runplan`）と、単独の変更で PR を作る `/runpr` は、PR を開いた後、`@codex review` のコメントで Codex にレビューを依頼する。
- **AC-07**: `/runpr` が 2 回目以降の PR レビューを依頼するとき（エスカレーションの判断が `request` のとき、または利用者が依頼を選んだとき）、`@codex review` で Codex に依頼する。比較の期間も、2 回目以降の依頼は Codex にだけ行う。
- **AC-08**: 依頼していない push で、Codex のレビューは走らない。Codex の個人設定の Automatic review は有効にしない。
- **AC-09**: コマンドの文書に、CodeRabbit に PR レビューを依頼する手順（`@coderabbitai review`）は、比較の期間の初回の依頼（AC-14）を除いて残らない。

#### F-003: Codex のレビュー済みの判定

`/runpr` は、Codex がそのコミットのレビューを完了したことを確かめてから、レビュー済みとする。

**Acceptance Criteria**:
- **AC-10**: Codex が指摘のあるレビューを完了したコミットと、指摘が 0 件のレビューを完了したコミットは、どちらもレビュー済みと判定される。指摘が 0 件のときに何を完了の印とするかは、AC-01 の 1 の観測で決める。
- **AC-11**: 次のものは、レビュー済みと判定されない。
    -   Codex のレート制限の通知（AC-01 の 5 で観測した文面）や、エラーの通知。
    -   head より前のコミットに対するレビュー（初回のレビューか依頼したレビューを待っているとき）。
    -   `chatgpt-codex-connector` 以外のアカウントが投稿したレビューやコメント。
- **AC-12**: `/runpr` は、Codex のレート制限の通知かエラーの通知を見つけたら、レビューを待ち続けずに止め、通知の URL と内容の要約を報告する。
- **AC-13**: 依頼から約 15 分たっても依頼したコミットがレビュー済みにならなければ、`/runpr` は止め、Codex が最後に投稿したもの（なければ、投稿がないこと）を報告する。マージの質問には進まない。

#### F-004: 比較の期間

比較の期間は、同じ PR を CodeRabbit と Codex の両方にレビューさせ、結果を比較の記録に残す。

**Acceptance Criteria**:
- **AC-14**: 比較の期間に開いた PR では、CodeRabbit と Codex の両方が、PR を開いたときの head をレビューする。CodeRabbit が自動でレビューしなければ、PR を開くコマンドが `@coderabbitai review` で依頼する（今の `_lib/task-pr-handoff.md:116-121` と同じ）。
- **AC-15**: 比較の期間の `/runpr` は、CodeRabbit と Codex の両方の初回のレビューが済み、両方が開いたスレッドの処理が済むまで、マージの質問に進まない。どちらかのレビューが届かなければ、AC-12・AC-13（Codex）と `runpr.md:192-196`（CodeRabbit）のとおりに止めて報告する。
- **AC-16**: 比較の期間の後は、`/runpr` は Codex のレビューだけを待ち、CodeRabbit の PR レビューを待たない。
- **AC-17**: 比較の記録には、比較の期間にマージした 5 件の PR が含まれ、そのうち文書だけを変える PR とコードを変える PR が、それぞれ 1 件以上ある。
- **AC-18**: 比較の記録には、PR ごとに次のものが含まれている。
    -   PR の番号と種類（文書かコードか）。
    -   CodeRabbit と Codex のそれぞれの指摘の数と、そのうち修正したもの（妥当）と誤検知のそれぞれの数。
    -   片方だけが挙げた妥当な指摘（もう片方の見落とし）の一覧。各項目に、スレッドの URL と、CodeRabbit の重大度か Codex の P0・P1 を付ける。Codex は P0・P1 しか投稿しないので、CodeRabbit の軽微な指摘を Codex の見落としと数えない。
    -   依頼からレビューの完了までの時間（それぞれ）。
    -   割り当ての消費（それぞれの依頼の回数）と、レート制限に達したかどうか。

#### F-005: 継続の判断と、その後の変更

比較の記録を読んで人が継続するかを判断し、判断に従って比較の期間を終える。

**Acceptance Criteria**:
- **AC-19**: 継続するかの判断、判断した人、日付、理由が、本タスクのディレクトリの下の文書に記録されている。理由は、比較の記録（AC-18）の項目を根拠に挙げる。
- **AC-20**: 継続すると判断した後は、PR を開いても CodeRabbit の PR レビューは走らず（`.coderabbit.yaml` で自動レビューを止める）、AC-16 のとおり `/runpr` は Codex だけを待つ。
- **AC-21**: 継続しないと判断した後は、F-002・F-003・F-004 のための変更を revert し、PR レビューが CodeRabbit に戻る。戻した後の `runpr.md` と `_lib/task-pr-handoff.md` は、Codex への依頼もレビュー済みの判定も含まない。

#### F-006: 文書

**Acceptance Criteria**:
- **AC-22**: `/runpr` がどの Bot の PR レビューを待つかは、`.claude/commands/_context.md` の 1 か所で定められている。この値は、信頼する自動レビュアーの一覧（#405）とは別の項目である。`runpr.md` と `_lib/task-pr-handoff.md` は、REST の名前（例: `chatgpt-codex-connector[bot]`）を直接書かず、その値を参照する。
- **AC-23**: `AGENTS.md` の、Codex がレビューの規則として読む節（AC-01 の 6 で確かめた見出し）に、レビューの規則がある。その節は、`CLAUDE.md` の内容を写さず、参照する。`AGENTS.md` を新たに作らず、既存のものを更新する。
- **AC-24**: `CLAUDE.md`・`_lib/pr-review-escalation.md`・`fixpr.md` の、PR レビューを行う Bot の名前と説明が、PR レビューが Codex であること（比較の期間は両方であること）に合っている。ローカルの CLI レビューを CodeRabbit と呼ぶ記述は残る。
- **AC-25**: `make test`・`make lint` と、CI の `doc-guard` のジョブが通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。比較の期間に、待つ時間が CodeRabbit と Codex のうち遅い方に合わせて長くなることは、F-004 で受け入れる。

### 4.2. セキュリティ (Security)

本タスクは、ネットワークへの送信（Codex の GitHub 連携がリポジトリの内容を読む）と、信頼できないテキスト（Codex が書くレビューの本文）の扱いに関わる。`docs/dev/security.md` の観点から、次を要件とする。

-   **Codex のスレッドの本文は信頼できないデータとして扱う:** Codex のスレッドは、#395（タスク 0030）の守りと同じ扱いを受ける。本文はデータとしてプロンプトに渡し、変更の範囲を確かめる。Codex を信頼する自動レビュアーに加えるのは #405 で行い、本タスクでは信頼の判定を変えない。
-   **判定に使う投稿の著者を確かめる:** レビュー済みの判定に使うのは、`_context.md` で定めた Bot（AC-22）の投稿だけである（AC-11）。ほかのアカウントの投稿が、完了の印に似た文面を含んでいても、レビュー済みとしない。公開リポジトリでは誰でも PR にコメントを書けるためである。
-   **送るのは公開されている内容だけ:** Codex の GitHub 連携が読むのは、公開リポジトリの PR の内容である。秘密（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`SLACK_WEBHOOK_URL`）をコミットしない規則（`CLAUDE.md` の「Development Notes」）は変わらない。
-   **依頼のコメントの範囲:** コマンドが PR に投稿するのは、レビューの依頼（`@codex review`、比較の期間は `@coderabbitai review` も）だけである。Codex に修正やブランチの作成を依頼するコメントは投稿しない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   Codex のレビューが届かないときは、レビュー済みとせず止めて報告する（AC-12・AC-13）。CodeRabbit への自動の切り替えはしない。
-   比較の期間は、CodeRabbit の判定の既知の問題（レート制限の通知をレビュー済みと数える。#394）が残る。比較の期間の `/runpr` は CodeRabbit のレビューも待つので、この問題の影響を受ける。#394 の修正は本タスクの範囲外とする（2.3）。

### 4.4. 互換性 (Compatibility)

-   ローカルの CLI レビュー（`coderabbit review`）の手順は変えない。
-   `/fixpr` のラウンド上限（`fixpr.md:42-47`）は、Codex のスレッドにも同じく働く。これは、Codex のスレッドの著者が `Bot` であることを前提にする（AC-01 の 3 で確かめる。違えば AC-05）。

### 4.5. 保守性 (Maintainability)

-   待つ PR レビュアーは 1 か所で定める（AC-22）。
-   判定の方法は Codex 用の 1 つだけにする。CodeRabbit の判定の方法は、比較の期間が終われば `runpr.md` から消える（AC-16・AC-20）。継続しないときは revert で戻す（AC-21）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   F-002 から F-006 は #405 の完了を前提にする。F-001 は前提なしに行える。
-   Codex の GitHub 連携には ChatGPT のプランが必要である。API キーの契約では使えない（issue #403）。
-   Codex の連携と設定（リポジトリの連携、コードレビューの有効化、Automatic review を無効のままにすること）は、リポジトリの外の設定であり、利用者が行う。
-   エスカレーションの方針（`_lib/pr-review-escalation.md`）は、Codex の割り当てを実測するまで残す（issue #403）。

## 6. 用語集 (Glossary)

-   **PR レビュー**: 自動レビュアーが GitHub の PR 上で行うレビュー。ローカルの CLI レビュー（`coderabbit review`）と区別する。
-   **ローカルの CLI レビュー**: `coderabbit` CLI でローカルの差分をレビューすること（`_lib/coderabbit-local-review.md`）。本タスクでは変えない。
-   **レビュー済み**: 待つ PR レビュアーが、あるコミットのレビューを完了した状態（`runpr.md` の「Reviewed」）。
-   **待つ PR レビュアー**: `/runpr` がレビュー済みの判定に使う Bot。信頼する自動レビュアーの一覧（#405）とは別に定める（AC-22）。
-   **比較の期間**: 同じ PR を CodeRabbit と Codex の両方にレビューさせる期間。切り替えを入れてから、継続するかを判断するまで（F-004・F-005）。
-   **割り当て**: 自動レビュアーが一定時間に受け付けるレビューの回数の上限（英語では quota）。`docs/translation_glossary.md` の「枠（slot）」は localhost サーバの同時処理数の単位であり、別の概念なので本書では使わない。
-   用語は `docs/translation_glossary.md` と統一する。
