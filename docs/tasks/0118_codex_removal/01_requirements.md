# 要件定義書：Codex の PR レビューと Codex CLI をレビューの体制から外す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-20 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、Codex の PR レビュー（GitHub の連携）と Codex CLI をレビューの体制から外し、Codex なしで PR が回るようにするための要件を定義する。

対応 issue: [#1059](https://github.com/isseis/yt2column/issues/1059)（関連: [#1057](https://github.com/isseis/yt2column/issues/1057)）

**背景:**

利用者は Codex の契約を終えることにした。今のレビューの体制は、次の 2 か所で Codex に依存している（[ADR-0013](../../dev/adr/0013-review-setup.md)）。

-   **Codex の PR レビュー:**
    -   **開いたときの依頼:** PR を開くコマンドは、Codex の自動レビューが始まらなければ `@codex review` で 1 回だけ依頼する（`.claude/commands/_lib/task-pr-handoff.md:301`）。
    -   **`/runpr` の待機:** `/runpr` の step 3 は、Codex の要約コメントが PR をレビューし終えるまで待つ（`.claude/commands/runpr.md:392`、`runpr.md:401`）。要約コメントが結果のない完了を示したときは、受領コメントで利用者に確かめる（タスク 0108）。
    -   **追加の依頼:** 最後の PR レビューの後の変更が条件に当たれば、PR レビューを自動でもう一度頼む（`.claude/commands/_lib/pr-review-escalation.md:34`）。マージの質問は「先に PR レビューを依頼」を選択肢に持つ（`runpr.md:1412`）。
-   **Codex CLI:** ローカルの CLI レビューは、CodeRabbit CLI が失敗したときに `codexreview` を通して Codex CLI を走らせる（`.claude/commands/_lib/local-cli-review.md:153`）。両方が失敗すると `Local CLI review: not run` を記録して進む（`local-cli-review.md:287`）。

判断の材料は #1057 の集計にある。47 件の PR（#984〜#1054）で、Codex の PR レビューのインラインの指摘は計 4 件だった。そのうち 1 件は誤読として退けたので、本物は 3 件（#1033、#1040、#1054）である。指摘の大半は、PR を開く前の Fable の部分レビュー（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）が出している。

CodeRabbit の PR レビューは、今は `.coderabbit.yaml` で自動レビューを止めている（`.coderabbit.yaml:9-14`）。以前は時間あたりの上限に頻繁に当たり、機能していなかった（`docs/tasks/0031_codex_pr_review/01_requirements.md` の 1 節）。スターが 10 未満のこのリポジトリでは、手動の `@coderabbitai review` でしか動かないことも観測している（PR #404）。

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。いつ誰に PR レビューを頼むか、`/runpr` が何を待つか、CodeRabbit CLI が失敗したときにどう進むか、どのメンションを拒否するか、何を撤去するか、である。次の事項は設計（`02_architecture.md`）で決める。設計は、F-001 の観測の記録がそろってから始める。

-   CodeRabbit の PR レビューの完了を判定する具体的な方法（どの API のどの値を見るか）と、待つ時間の上限。
-   コマンドの文書・道具・ガードテストのどこをどう変えるか、どの PR に分けるか。

要件の起草で挙がった設計の事項は、[design_handoff.md](design_handoff.md) に申し送る。

**issue #1059 で決めたこと:**

-   **Codex の PR レビューと Codex CLI の両方を外す:** 契約を終えるためである。
-   **順序:** このタスクの変更をマージしてから、利用者が GitHub の連携を切り、契約を終える。このタスク自身の PR は、Codex の PR レビューが動いている間の `/runpr` で回す。先に連携を切ると、今の `/runpr` の step 3 が依頼の投稿と待機を繰り返すためである（4.4）。
-   **`docs/tasks/` は変えない:** タスク文書は、各タスクが何を決めたかの記録だからである。

**要件定義の際に決めたこと（2026-11-20）:**

-   **プロセスは `full` にする:** 推奨は `full` で、利用者もそれを選んだ。除外に当たると判断したためである。このタスクは、開発用の道具の信頼の境界を変える（レビューの関門、`fixprguard` のメンションの拒否、自動レビュアーの一覧）。ほかにも、10 を超えるコマンドと道具に及び、ADR-0013・0014・0015・0042 の決定を変える。
-   **PR レビューは、利用者が選んだときだけ CodeRabbit に手動で頼む（F-002〜F-004）:** PR を開いたときにも、`/runpr` の中でも、自動では頼まない。マージの質問で利用者が「先に PR レビューを依頼」を選んだときだけ、`@coderabbitai review` で頼む。「なくす」案（推奨だった）より、信頼の境界を変える PR などで外部の目を足す手段を残すことを利用者が選んだ。CodeRabbit の自動レビューを有効に戻す案は採らない。上限に当たって機能しなかった実績があるためである。ADR の判断: [ADR-0013](../../dev/adr/0013-review-setup.md) を更新する。
-   **CodeRabbit の振る舞いは、設計の前に観測する（F-001）:** 完了の印や上限に当たったときの見え方は、0031 より前の記録（`docs/tasks/0031_codex_pr_review/01_requirements.md` の 1 節）から 1 か月たっており、確かめていない。0031 より前の判定（`StatusContext` と「between」の行）をそのまま使う案は採らない。上限に当たっても CheckRun が `pass` になる観測（PR #404）があり、形が変わっていれば `/runpr` が止まるか、誤ってレビュー済みとしうるためである。ADR の判断: ADR なし。観測は判断の材料で、決定そのものではないためである。
-   **エスカレーションは、勧めるだけにする（F-005）:** 判定は残し、当たれば、マージの質問で「先に PR レビューを依頼」を推奨にする。自分では依頼しない。範囲は、最後に完了した CodeRabbit の PR レビューの後とし、それがなければ PR の全体とする。PR レビューを手動に限る決定に合わせつつ、信頼の境界を変える PR で利用者の目を引くためである。撤去する案は採らない。どの PR でもレビューを推奨しなくなり、判断の材料が減るためである。ADR の判断: [ADR-0013](../../dev/adr/0013-review-setup.md) を更新する。
-   **CodeRabbit CLI が失敗したら `not run` を記録して進む（F-006）:** 代わりのレビューは置かない。今も両方の CLI が失敗したときはこの形なので、新しい仕組みが要らないためである。見ていない範囲は、今と同じく PR の説明とマージの質問に出る。Opus のサブエージェントを代わりに置く案は採らない。#1057 の Opus への置き換えと重なるためである。止めて利用者に聞く案も採らない。理由は、推奨に従った利用者の選択で、記録されていない。ADR の判断: [ADR-0013](../../dev/adr/0013-review-setup.md) を更新する。
-   **Fable が走らない PR の危険は受け入れる:** OpenCode で実装した PR と単独の PR では、外部のレビューが CodeRabbit CLI だけになる（CLI が失敗すればなくなる）。#1057 の集計では、Codex の本物の指摘 3 件のうち 2 件がこの種類の PR だった。それでも、補う仕組みは #1057 の並走の結果を見てから、別のタスクで決めたほうが手戻りが少ない。そのため、この危険を受け入れて記録する。ADR の判断: [ADR-0013](../../dev/adr/0013-review-setup.md) の「受け入れた危険」に加える。
-   **Opus のレビューの直しの commit を、ローカルの CLI レビューにかける（F-007）:** [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) は、直しの commit をかけ直さないと決めている。最初の Codex の PR レビューが直しを見るという理由だったが、その理由がなくなる。かけないと、直しを Claude 以外の系統のレビューが一度も見ないことになる。CodeRabbit CLI の割り当ては PR レビューとは別で、増えるのは 1 PR に 1 回である。ADR の判断: [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) を更新する。
-   **`@codex` と `@coderabbitai` の両方を拒否する（F-009）:** 利用者のアカウントからの投稿で、依頼の文面のほかは、どちらのメンションも含めない。推奨は `@coderabbitai` に移すことだったが、利用者は両方を選んだ。連携を切った後に `@codex` が何かを動かすことはないと見込めるが、拒否を残しても費用はほとんどないためである。`/runpr` が確かめる「利用者以外のメンション」も両方を対象にする。CodeRabbit も、利用者のアカウントからのメンションで修正の PR を作るなどの操作ができ、書き込み権限のない人の依頼を防ぐ interaction limits の確かめも残すためである。ADR の判断: [ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) を更新する。
-   **信頼する自動レビュアーの一覧から `chatgpt-codex-connector` を外す（F-010）:** 連携を切った後は Codex のスレッドはできないので、使っていない Bot を信頼したままにしないためである。ADR の判断: [ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) を更新する（上の項目と同じ更新）。
-   **`codexreview` は、プロトコル番号ごと撤去する（F-008）:** Codex CLI を走らせる道具なので、Codex CLI を外せば要らなくなる。番号を持つ道具は `fixprguard`・`chief`・`mergepr` の 3 つになる。ほかの道具の番号を上げるかは、`CLAUDE.md` の「Protocol numbers」の規則に従って設計で決める（[design_handoff.md](design_handoff.md) の H-03）。ADR の判断: [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) を更新する。
-   **chief の読み取りのサンプルは残す（F-012）:** chief のコードは Codex の文言で分岐していない。Codex が現れるのはテストとサンプルだけで、サンプルは実際の画面の記録である。`/runpr` の報告の形が変わって読み取りの形が変わるときだけ、新しいサンプルを足す。ADR の判断: ADR なし。このタスクの中で閉じるためである。
-   **Codex のために足した `/runpr` の仕組みは撤去する（F-003）:** 要約コメントの待機、受領コメント（0108）による利用者の確認がそれに当たる。Codex にしかない形の仕組みなので、PR レビューを CodeRabbit に移すと要らなくなるためである。ADR の判断: [ADR-0013](../../dev/adr/0013-review-setup.md) を更新する（上の PR レビューの項目と同じ更新）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   Codex の GitHub の連携を切り、Codex CLI がなくなっても、タスクの PR と単独の PR を、作成からマージまで回せる。
-   PR を開いた後の外部のレビューを、利用者が選んだときだけ CodeRabbit に頼める。
-   Codex への依存を、コマンドの定義・道具・設定・現在の仕様を書く文書から除く。

### 2.2. スコープ (In Scope)

-   CodeRabbit の PR レビューの観測（F-001）。
-   PR を開くコマンドと `/runpr` から、Codex の PR レビューの依頼・待機・判定を除くこと（F-002、F-003）。
-   マージの質問から CodeRabbit の PR レビューを手動で頼み、完了を待つこと（F-004）。
-   エスカレーションを、勧めるだけにすること（F-005）。
-   ローカルの CLI レビューから Codex CLI を除くこと（F-006）。
-   Opus のレビューの直しの commit を、ローカルの CLI レビューにかけること（F-007）。
-   `codexreview` の撤去（F-008）。
-   メンションの拒否と、利用者以外のメンションの確かめを、2 つのメンションに広げること（F-009）。
-   信頼する自動レビュアーの一覧と、完了を待つ PR レビュアーの変更（F-010）。
-   現在の仕様を書く文書と ADR の更新（F-011）。
-   chief の読み取りの扱い（F-012）。

### 2.3. スコープ外 (Out of Scope)

-   **GitHub の連携を切ることと、Codex の契約を終えること:** リポジトリの外で、利用者が行う。順序は 4.4 で定める。
-   **`docs/tasks/` の文書:** 各タスクの記録なので変えない（issue #1059 で決めたこと）。
-   **Fable が走らない PR を補う仕組み:** 受け入れる危険として記録する（要件定義の際に決めたこと）。補うかは、#1057 の並走の後に別のタスクで決める。
-   **#1057 の Fable から Opus への置き換え:** 別のタスクで行う。並走期間は、このタスクの後に始める。
-   **CodeRabbit の自動の PR レビュー:** 有効に戻さない（要件定義の際に決めたこと）。
-   **書き込み権限のない人の `@coderabbitai` の観測:** interaction limits（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）が、書き込み権限のない人のコメントを止めている。この防御に頼るので、観測しない。
-   **`docs/research/` の記録:** 調べた時点の記録なので変えない。
-   **`AGENTS.md`:** Codex に固有の記述がなく、ローカルの CLI レビューを含む自動のレビュアーへの指示として使い続けるので、変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 用語

-   **手動の PR レビュー:** 利用者がマージの質問で選んだときに、`/runpr` が CodeRabbit に頼む PR レビュー（F-004）。
-   **メンションの対象:** `@codex` と `@coderabbitai` の 2 つ（F-009）。
-   **現在の仕様を書く文書:** `CLAUDE.md`、`.claude/commands/` の下のコマンドの定義（`_context.md` と `_lib/` を含む）、`docs/dev/` の下の文書（ADR を含む）、`docs/translation_glossary.md`、設定ファイル（`.coderabbit.yaml`、`.claude/settings.json`、`Makefile`、`.github/workflows/`）。`docs/tasks/` と `docs/research/` は含まない。

### 3.2. 機能一覧

#### F-001: CodeRabbit の PR レビューの観測

設計の前に、試験の PR で CodeRabbit の PR レビューの振る舞いを観測し、記録する。

**Acceptance Criteria**:
- **AC-01**: 本タスクのディレクトリの下に観測の記録がある。記録は、次の各項目について、観測した日付、PR の番号、観測した値（API の応答の該当する欄）を持つ。
    1. 利用者の `@coderabbitai review` から、レビューが終わるまでに CodeRabbit が投稿するもの（コメント、レビュー、`StatusContext` や CheckRun）と、その順序。
    2. 完了したことと、レビューしたコミットを示す値。指摘がないときとあるときの両方。
    3. 時間あたりの上限に当たったときに、CodeRabbit が示すもの。観測できなかったときは、そのことと、代わりに CodeRabbit の公式の文書から分かることを、出典とともに記録する。
    4. スレッドの著者の値（GraphQL の `author.__typename` と `author.login`、REST の `user.type` と `user.login`）。
    5. `.coderabbit.yaml` の自動レビューが止まったまま、PR を開いたときと push したときに、CodeRabbit が自分でレビューを始めないこと。
- **AC-02**: 試験の PR への `@coderabbitai review` の投稿は利用者が行い、エージェントは行わない。記録には、誰が投稿したかを書く。
- **AC-03**: `02_architecture.md` が CodeRabbit の完了の判定として使う値は、すべて AC-01 の記録にある値である。

#### F-002: PR を開くときに PR レビューを頼まない

PR を開くコマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2`・`/runplan`、単独の変更の PR を作る `/runpr`）は、PR を開いた後に、どの自動レビュアーにも PR レビューを頼まず、その開始も待たない。

**Acceptance Criteria**:
- **AC-04**: これらのコマンドが PR を作ってから報告するまでの間に、コマンドは PR へのコメントを投稿しない。
- **AC-05**: これらのコマンドの最終の報告と利用者への通知は、今と同じく PR の URL と次の行動（PR をレビューして `/runpr <PR>` を実行する）を示す。PR レビューを待つ旨は書かない。

#### F-003: `/runpr` は PR レビューを待たない

`/runpr` は、CI と、信頼する著者のスレッドの処理だけを待って、マージの質問に進む。Codex の要約コメントと受領コメントは読まない。

**Acceptance Criteria**:
- **AC-06**: CI が成功し、未解決のスレッドがない PR では、`/runpr` は PR レビューの完了の印を待たずに、マージの質問を出す。
- **AC-07**: `/runpr` は、Codex の要約コメントや受領コメントを待たず、その有無で止まらない。受領コメントによる「指摘なしとして進めますか」の問いは出さない。
- **AC-08**: 信頼する著者のスレッドが PR にあれば、`/runpr` は今と同じく `/fixpr` で処理してから、マージの質問に進む。
- **AC-09**: マージの質問の要約の行は、Codex の名前を出さない。PR を開いた後の変更をローカルの CLI レビューが見たかと、Fable の部分レビューの記録は、今と同じく示す。

#### F-004: 手動の PR レビュー

マージの質問で利用者が「先に PR レビューを依頼」を選んだときだけ、`/runpr` は CodeRabbit に PR レビューを頼み、その完了を待ってから、マージの質問に戻る。

**Acceptance Criteria**:
- **AC-10**: 「先に PR レビューを依頼」は、PR の head を、完了した手動の PR レビューがまだ見ていないとき、マージの質問の選択肢に出る。head を見た完了の印があるときは出ない。
- **AC-11**: 利用者が「先に PR レビューを依頼」を選んだときだけ、`/runpr` は `@coderabbitai review` を本文とするコメントを 1 件投稿する。それ以外のときは、どの自動レビュアーにも依頼しない。
- **AC-12**: 依頼の後、`/runpr` は CodeRabbit の完了の印を待つ。完了の印が届けば、その間に開かれたスレッドを `/fixpr` で処理してから、マージの質問に戻る。
- **AC-13**: 時間あたりの上限に当たったことが分かったとき、または待つ時間の上限を過ぎても完了の印がないときは、レビュー済みとせずに止まる。報告は、そのことと依頼のコメントの URL を示し、次の手を示す（上限が戻ってから `/runpr` を再実行する、またはレビューなしでマージに進む）。
- **AC-14**: 止まった後の再実行では、応答を待っている依頼があれば、再び依頼しない。上限やエラーの知らせを受けた依頼と、待つ時間の上限を過ぎた依頼は終わったものとし、利用者が再び選べば新しく依頼する。

#### F-005: エスカレーションは勧めるだけにする

エスカレーションの判定は残す。その結果は、マージの質問の推奨を決めるだけにし、PR レビューを依頼しない。

**Acceptance Criteria**:
- **AC-15**: エスカレーションが判定する範囲は、最後に完了した手動の PR レビューが見たコミットから head までとする。完了した手動の PR レビューがないときは、PR の全体（`origin/main` との merge-base から head まで）とする。
- **AC-16**: 範囲の変更が信頼の境界・セキュリティ・耐久性の基準に当たるとき、マージの質問は「先に PR レビューを依頼」を推奨にする。
- **AC-17**: ほかの基準にだけ当たり、その範囲のすべての commit をローカルの CLI レビューが見たと記録から分からないとき、マージの質問は「先に PR レビューを依頼」を推奨にする。すべて見たと分かるときは、「マージ」を推奨にする。
- **AC-18**: どの基準にも当たらないときは、「マージ」を推奨にする。
- **AC-19**: どの判定の結果のときも、`/runpr` はエスカレーションの結果だけで PR レビューを依頼しない（依頼は AC-11 だけ）。

#### F-006: ローカルの CLI レビューは CodeRabbit CLI だけにする

ローカルの CLI レビューは CodeRabbit CLI だけで行う。失敗したときは、代わりのレビューを走らせず、`not run` を記録して進む。

**Acceptance Criteria**:
- **AC-20**: CodeRabbit CLI が失敗したとき、ローカルの CLI レビューは Codex CLI を含むほかのレビューを走らせない。見ていない範囲を `not run` の記録の行に残し、呼び出し元の手順を止めずに進む。
- **AC-21**: 見ていない範囲は、今と同じく PR の説明（レビューを受けていない commit の一覧）とマージの質問の要約の行に出る。
- **AC-22**: このタスクの変更がマージされる前の commit にある `Codex local review:` の行と、`(codex)` を持つ `Local CLI reviewed:` の行は、それを持つブランチで今と同じ意味に読まれる。すなわち、レビューした範囲と指摘として読まれ、見ていない範囲として扱われない。

#### F-007: Opus のレビューの直しの commit をローカルの CLI レビューにかける

`/runplan` で Opus のレビューが直しの commit を作ったときは、PR を開く前に、その commit をローカルの CLI レビューにかける。

**Acceptance Criteria**:
- **AC-23**: Opus のレビューが直しの commit を 1 つ以上作った `/runplan` では、PR を開く前に、ローカルの CLI レビューが直しの commit の範囲をレビューし、その結果をほかのローカルの CLI レビューと同じ形でブランチに記録する。
- **AC-24**: そのレビューで CodeRabbit CLI が失敗したときは、F-006 のとおり `not run` を記録して PR を開く。PR の説明は、その範囲をレビューを受けていない commit として示す。
- **AC-25**: Opus のレビューが直しの commit を作らなかったとき、このためのローカルの CLI レビューは走らない。

#### F-008: `codexreview` の撤去

`codexreview` の道具と、それを使う・確かめる・許可するものを撤去する。

**Acceptance Criteria**:
- **AC-26**: リポジトリに `cmd/codexreview` と `internal/codexreview` がない。
- **AC-27**: `Makefile` に `install-codexreview` のターゲットがない。
- **AC-28**: `.claude/settings.json` の許可に、`codexreview` を実行する規則がない。
- **AC-29**: マージの前のプロトコル番号の確かめ（`mergepr`）と、`CLAUDE.md` の「Protocol numbers」が番号を持つ道具として挙げるものは、`fixprguard`・`chief`・`mergepr` の 3 つであり、`codexreview` を含まない。
- **AC-30**: CI の `doc-guard` のジョブと `make test`・`make lint`・`make deadcode` は、撤去の後も成功する。

#### F-009: 2 つのメンションの拒否と確かめ

利用者のアカウントから投稿する文面は、手動の PR レビューの依頼のほかは、`@codex` も `@coderabbitai` も含まない。`/runpr` は、利用者以外が書いたどちらのメンションも確かめる。

**Acceptance Criteria**:
- **AC-31**: `fixprguard` は、レビューのスレッドへの返信と PR の説明の更新で、`@codex` または `@coderabbitai` を含む文面を、GitHub に書き込む前に拒否する。比べ方は、今の `@codex` の拒否と同じく、大文字と小文字を区別せず、文面のどこにあっても拒否する（`internal/fixprguard/write.go:91`）。
- **AC-32**: コマンドの定義が投稿の前に行うメンションの確かめは、2 つのメンションのどちらかを含む文面を投稿しない。例外は、AC-11 の依頼の文面だけである。
- **AC-33**: `/runpr` は、PR のコメントに利用者以外が書いた `@codex` または `@coderabbitai` を見つけたとき、今の `@codex` のときと同じく、レビューを待たずに止まって報告する。
- **AC-34**: interaction limits の確かめ（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）は、今と同じく行う。

#### F-010: 自動レビュアーの一覧と完了を待つ PR レビュアー

**Acceptance Criteria**:
- **AC-35**: 信頼する自動レビュアーの一覧（`.claude/commands/_context.md` の「Automated reviewer list」）は `coderabbitai` だけである。`chatgpt-codex-connector` のスレッドは、ほかの信頼しない著者のスレッドと同じく扱われる。
- **AC-36**: 完了を待つ PR レビュアー（`_context.md` の「Awaited PR reviewer」）は `coderabbitai` であり、PR レビューの依頼の文面（「PR review request」）は `@coderabbitai review` である。

#### F-011: 現在の仕様を書く文書と ADR

**Acceptance Criteria**:
- **AC-37**: 現在の仕様を書く文書（3.1）は、Codex の PR レビュー、Codex CLI、`codexreview`、受領コメントを、今の振る舞いとして書かない。過去の決定・採らなかった案・受け入れた危険の出典として名前を挙げること、F-009 の `@codex` の拒否、AC-22 の古い記録の行の読み方は、これに当たらない。確かめは、PR のレビューで読むこと（`manual`）とする。
- **AC-38**: 用語集は、Codex CLI を含む定義（「試行」など）を、CodeRabbit CLI だけの定義に改める。
- **AC-39**: ADR-0013・0014・0015・0042 の `## 決定` は、本書の決定と一致する。実装が終わるまで残る食い違いは、各 ADR の `## 決定` に、#1059 への食い違いの記述として示す。実装が食い違いを解いた PR で、その記述を外す。

#### F-012: chief の読み取り

**Acceptance Criteria**:
- **AC-40**: `internal/chief/testdata/` の今のサンプルは残り、chief のテストはそれで成功する。
- **AC-41**: このタスクが `/runpr` の報告または NEXT 行の形を変え、chief の読み取りの形に当たるときは、変えた後の `/runpr` の実際の報告を新しいサンプルとして足し、chief がそれを読めることをテストで確かめる。形が変わらないときは、サンプルを足さない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。待つ時間が短くなる変更で、性能の要件はない。手動の PR レビューの待つ時間の上限は、F-001 の観測を見て設計で決める。

### 4.2. セキュリティ (Security)

-   条件付きのガイド（`docs/dev/security.md`）のきっかけ（外部のコマンドの起動、API キー、キャッシュ、LLM API への送信、信頼しないテキストのプロンプトへの埋め込み）に当たる機能はない。N/A。
-   開発用の道具の信頼の境界を、次のように変える。
    -   **投稿する文面:** 利用者のアカウントからの投稿が自動レビュアーを動かさないよう、F-009 で拒否するメンションを 2 つにする。
    -   **信頼する著者:** F-010 で自動レビュアーの一覧を 1 つにする。
    -   **外部の人の入口:** interaction limits の確かめは残す（AC-34）。
    -   **外部へ送るもの:** Codex CLI がなくなるので、ローカルの CLI レビューが差分を OpenAI のサービスへ送ることはなくなる。CodeRabbit CLI が CodeRabbit のサービスへ送ることは変わらない。
-   受領コメント（0108）の本文を出さない絞り込みは、受領コメントの読み取りごと撤去する。手動の PR レビューのコメントを読むときも、要約コメントの本文をセッションに出さない今の原則（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）を保つ。どの値を読むかは設計で決める（[design_handoff.md](design_handoff.md) の H-01）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   手動の PR レビューが届かないときは、止めて報告し、レビュー済みとしない（AC-13）。
-   CodeRabbit CLI が失敗しても、手順を止めない（AC-20）。

### 4.4. 互換性 (Compatibility)

-   **移行の順序:**
    1.  このタスクの PR は、Codex の連携が動いている間に、その時点の `main` の `/runpr` で回す。
    2.  F-002・F-003 を入れる PR がマージされた後は、Codex の自動レビューが PR に投稿されても、`/runpr` は待たない。
    3.  利用者は、このタスクの最後の PR がマージされてから、Codex の連携を切り、契約を終える。
-   **ほかのタスクのブランチ:** このタスクの変更がマージされる前に作られ、`Codex local review:` などの行を持つブランチも、マージの後の `/runpr`・`/fixpr`・`/runplan` で回せる（AC-22）。
-   **入れ直す道具:** 番号を変えた道具は、マージの後に利用者が `main` から入れ直す（`CLAUDE.md` の「Protocol numbers」）。利用者の環境に残った `codexreview` のバイナリは、どのコマンドからも呼ばれない。消すかは利用者に任せる。

### 4.5. 保守性 (Maintainability)

-   Codex に固有の判定（要約コメント、受領コメント、`codexreview` の最終メッセージの解析）を保守しなくてよくなる。
-   CodeRabbit の完了の判定は、F-001 の記録に依存する。形が変わったときは、`/runpr` が止まり（AC-13）、設計を直す。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md` の「Protocol numbers」に従う。タスク文書には上げた後の番号を書かず、「`main` の値 + 1」と書く。
-   `CLAUDE.md` の「Tool Execution Safety」に従う。試験の PR への依頼の投稿（F-001）と、ネットワークに触れるほかの操作は、利用者の承認を得て行う。
-   テストの削除は、`CLAUDE.md` の「Testing Strategy」の「Deleting a test is a claim that must be checked」に従う。ただし、撤去する道具そのものの `_test.go` を、その製品のコードと一緒に消す場合の扱いは設計で決める（[design_handoff.md](design_handoff.md) の H-04）。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **PR レビュー**、**ローカルの CLI レビュー**、**完了を待つ PR レビュアー**、**自動レビュアーの一覧**、**完了の印**、**試行**、**割り当て**は、用語集の定義に従う（試行は AC-38 で改める）。
-   **手動の PR レビュー**、**メンションの対象**、**現在の仕様を書く文書**は、3.1 で定める。
