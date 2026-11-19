# 要件定義書：Fable の部分レビューの記録の後、push の前にできた commit を、PR を開く前に確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、Fable の部分レビューの記録の後、push の前にできた commit（`origin/main` の merge commit を含む）を、PR を開く前に確かめる要件を定義する。

**背景:**

-   タスク 0098 は、タスクの PR を開く前に Fable の部分レビューを行う手順を作った（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）。ADR-0050 は、Fable が見た範囲の後に中身のある commit が足されたら、その commit についてもう一度 Fable の部分レビューを行う、と決めている（`docs/dev/adr/0050-fable-partial-review.md:28`）。
-   有効かの判定は、`origin/main` から取り出した `.claude/commands/_lib/review-records.sh` が決める（`.claude/commands/_lib/fable-review.md:79-135`）。今の手順は、その判定を 2 つの入口でしか走らせない。
    -   **起動の入口:** ローカルの CLI レビューの前に走る（`fable-review.md:137-139`）。タスク文書の PR では、`_lib/task-pr-handoff.md` Part B step 1 がここを通る（`.claude/commands/_lib/task-pr-handoff.md:184-197`）。
    -   **チェックポイントの入口:** `/runplan` の green gate の前に走る（`fable-review.md:234-248`、`.claude/commands/runplan.md:98`）。
-   どちらの入口も、判定の後、push までにできた commit をもう一度判定しない。タスク文書の PR では、記録の commit の後に Part B step 2（CHECKS）の直しや `origin/main` の merge commit ができても、Fable が見ないまま PR が開きうる。`/runplan` でも、チェックポイントの印の後に merge commit ができれば同じことが起きる。判定のスクリプトは、merge commit を、記録を無効にしない commit のどれにも数えない（`.claude/commands/_lib/review-records.sh:109`）。
-   タスク 0098 の実地の確かめで、実際に起きた（[0098 実装計画書](../0098_fable_partial_review/03_implementation_plan.md) §7「PR-3 の実行の記録」のステップ 3-3）。PR #987 では、記録の commit `5de18f65` の後に `origin/main` の merge commit `ac43c9c9` が作られた。この merge は `docs/dev/adr/0046-chief-of-staff.md` の衝突を解いている。判定のスクリプトは、記録の commit では `VALID`、merge の後では `MISSING <merge-base>` を返した。しかし、#987 は Fable を走らせ直さずに開かれた。
-   merge で merge-base が動くと、それより前の見た範囲の行（`Local CLI reviewed:`）と Fable の記録は、START の確かめを通らずに無視される（`review-records.sh:46-52`、`:87-91`）。この場合、判定のスクリプトは `MISSING <新しい merge-base>` を返し、Fable は PR の全体をもう一度見ることになる。`/runpr` の `coverage` は `PARTIAL` になり、人に問う（`review-records.sh:42`）。どちらも安全な側の動きである。
-   PR の説明の「どちらの CLI もレビューしなかった commit」は、このセッションのレビューが全体を見たとき、そのレビューの SUMMARY にある範囲だけを挙げる（`task-pr-handoff.md:250-254`）。記録の後にできた commit を書く規則はない。#987 の PR の説明は、merge commit と空の記録の commit の 2 つを挙げた（0098 実装計画書 §7 のステップ 3-1）。

対応 issue: [#988](https://github.com/isseis/yt2column/issues/988)（関連: [#924](https://github.com/isseis/yt2column/issues/924)（タスク 0098）、PR [#987](https://github.com/isseis/yt2column/pull/987)（食い違いを観察した PR））

本書は、次のことを決める。

-   push の直前に、Fable の記録が有効かをもう一度判定すること。
-   どの commit が記録を無効にし、どれが無効にしないか。
-   merge の後に Fable が見る範囲。
-   PR の説明への書き方。
-   ADR-0050 の更新。

判定を手順のどこに置くか、判定のスクリプトを変えるか、Part B step 2 の直しを見分ける方法は `02_architecture.md` が決める（[design_handoff.md](design_handoff.md)）。

### issue #988 で決めたこと

-   **#988 の食い違いはタスク 0098 では直さず、この issue で扱う。** 0098 の実地の確かめで見つかり、人と相談して決めた（0098 実装計画書 §7 のステップ 3-3、0098 要件定義書 AC-42 の例外）。
    -   ADR：なし。どのタスクで扱うかの選択で、決定ではない。
-   **直すときは、ADR-0050 も合わせて直す。** 記録の後の commit の扱いは ADR-0050 の決定であり、それを変えるためである。
    -   ADR：[ADR-0050](../../dev/adr/0050-fable-partial-review.md) を更新する（下の「要件定義の際に決めたこと」）。

### 要件定義の際に決めたこと（2026-11-19）

-   **進め方は通常の進め方（`full`）とする。この変更は、除外される変更に当たらない。** 通常の進め方を勧めた理由は 3 つある。ADR-0050 の決定を変えること、共有の手順の 2〜3 個（`_lib/task-pr-handoff.md`・`_lib/fable-review.md`、設計によっては `_lib/review-records.sh`）に及ぶこと、決めることが複数あることである。除外の判断は次のとおりである。`docs/dev/security.md` の規則と前提を変えない。レビューのスレッドや Bot への依頼の扱いに触れず、`fixprguard`・`codexreview`・`mergepr` も変えないので、開発の道具の信頼境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）を変えない。外部依存も足さない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。人の答え：通常の進め方。
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   **push の直前に、Fable の記録が有効かをもう一度判定する。有効でなければ、足りない commit について Fable の部分レビューを単独で行う。Part B step 2 の直しの commit は、`/runplan` の green gate の直しと同じく、記録を無効にしない。merge commit は、いつも記録を無効にする（F-001、F-002）。** 今は判定が入口でしか走らないことが、#987 の食い違いの原因だからである。直しの commit を無効にしないのは、CHECKS の直し（文書のガードのテストを通すための直し）のたびに Fable が走り直さないようにするためである。これで、タスク文書の PR の扱いが `/runplan` のチェックポイントの扱いと揃う。人の答え：再判定し、Part B の直しは除く。
    -   採らなかった案：(B) Part B step 2 の直しにも Fable を走らせ直す（今の判定の規則のまま）。CHECKS の直しのたびに、Fable の 1 回分（2〜16 分）の費用がかかる。(C) 再判定して有効でなければ、自動では走らせずに止め、人に問う。人の手間が増えるうえ、人が選べるのは Fable を走らせることだけである。
    -   ADR：[ADR-0050](../../dev/adr/0050-fable-partial-review.md) を更新する。記録の後の commit の扱いという ADR-0050 の決定を変えるので、主要な決定に当たる（CLAUDE.md「Design decisions (ADR)」）。
-   **push の直前の再判定は、タスク文書の PR と `/runplan` の PR の両方に入れる（F-001）。** `/runplan` のチェックポイントの印の後にできた merge commit にも同じ穴がある。どちらの PR も、push を `_lib/task-pr-handoff.md` Part B で行うので、1 か所で直せる。人の答え：含める。
    -   ADR：ADR-0050 を更新する（上の決定と同じ項目）。
-   **merge の後は、今の安全な側の動きを保つ。merge の前の記録は無視され、Fable は PR の全体（新しい merge-base から HEAD まで）をもう一度見る。判定のスクリプトの規則は、merge commit について変えない。`/runpr` の `coverage` が `PARTIAL` になって人に問う動きも変えない（F-002）。** 判定の関門の規則に新しい仕組みを足すと、誤って有効と判定する危険な側の誤りの余地が増える。記録の後の merge は稀なので、Fable の 1 回分の費用は受け入れる。人の答え：今の安全な側を保つ。
    -   採らなかった案：(B) merge の前の記録を有効なまま残し、merge で入った枝の側の変更（衝突の解決など）だけを Fable に見せる。判定のスクリプトに新しい規則が要る。(C) Part B step 1 の前に `main` を取り込む手順を足し、記録の後の merge を減らす。merge の要る理由（衝突など）は記録の後にも起こりうるので、減らしても、A の扱いは別に要る。
    -   ADR：ADR-0050 を更新する。
-   **タスク 0098 の承認済みの文書は直さない。ADR-0050 の決定と経緯だけを更新する（F-004）。** 今の決定とその経緯は ADR が持ち、タスクの文書はそのタスクで決めたことの記録だからである（CLAUDE.md「Design decisions (ADR)」の「Who holds what」）。0098 の文書は、#988 を既知の例外としてすでにリンクしている（0098 要件定義書 AC-42、0098 設計書の 8 章）。issue は「0098 の設計書の該当箇所もあわせて直す」と書いていたが、人と決めてこの形にした。人の答え：直さない。
    -   ADR：ADR-0050 を更新する（経緯に本タスクを足す）。
-   **PR の説明の「どちらの CLI もレビューしなかった commit」には、記録の後・push の前にできた、中身のある commit（merge commit を含む）を挙げる。中身のない commit は挙げない（F-003）。** 今の規則は、記録の後にできた commit の扱いを決めていない。#987 では、空の記録の commit まで挙がった。人の答え：中身のあるものだけ書く。
    -   ADR：なし。PR の説明の書き方は、`_lib/task-pr-handoff.md` の 1 つの手順の中で閉じる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   記録の後に中身のある commit ができたとき、Fable がそれを見ないまま、タスクの PR が開かれないようにする。
-   タスク文書の PR と `/runplan` の PR で、記録を無効にしない commit の扱いを揃える。
-   PR の説明で、どちらの CLI もレビューしなかった commit を、記録の後にできたものも含めて正しく示す。

### 2.2. スコープ (In Scope)

-   `_lib/task-pr-handoff.md` Part B の、push の直前の Fable の記録の再判定と、有効でないときの単独の Fable の部分レビュー（F-001）。
-   タスク文書の PR の Part B step 2 の直しの commit を、記録を無効にしない commit にすること（F-002）。手段は設計で決める。
-   PR の説明の「どちらの CLI もレビューしなかった commit」の規則（F-003）。
-   ADR-0050 の決定・採らなかった案・経緯の更新（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   **判定のスクリプトの、merge commit と merge-base の扱いの変更。** merge commit はいつも記録を無効にし、merge の前の記録は無視される、今の規則のままにする（要件定義の際に決めたこと）。
-   **`/runpr` の `coverage` の判定と、マージの質問の変更。** merge commit で `PARTIAL` になって人に問う動きは変えない（同上）。
-   **PR を開いた後の変更を Fable に見せること。** Fable の部分レビューは PR を開く前のものである（ADR-0050）。PR を開いた後に `/runpr` や `/fixpr` が `main` を取り込んだ merge は、本タスクで扱わない。
-   **タスク 0098 の文書の変更**（要件定義の際に決めたこと）。
-   **Part B の前に `main` を取り込む手順の追加**（要件定義の際に決めたことの採らなかった案 C）。
-   **ローカルの CLI レビューを、記録の後の commit について走らせ直すこと。** 記録の後の commit は、PR を開いたときに Codex の PR レビューが見る。PR の説明には、どちらの CLI もレビューしなかった commit として挙げる（F-003）。
-   **#987 の PR の扱い。** すでにマージされており、本タスクでは何もしない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

本章では、次の 2 つを次の意味で使う。

-   **タスクの PR:** タスク文書の PR（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2` が開く PR）と、`/runplan` の PR。
-   **push の直前:** Part B で push と PR の作成（`_lib/task-pr-handoff.md` Part B step 3）を行う前。Part B step 1 の README の同期と step 2 の CHECKS を、直しを含めて終えた後を指す。

#### F-001: push の直前の、Fable の記録の再判定

タスクの PR を push する直前に、Fable の記録が有効かをもう一度判定する。有効でなければ、足りない commit について Fable の部分レビューを単独で行ってから push する。

**Acceptance Criteria**:

- **AC-01**: タスクの PR の push の直前に、`origin/main` から取り出した判定のスクリプトで、ブランチの Fable の記録が有効かを判定する。タスク文書の PR でも、`/runplan` の PR でも判定する。判定の取り出し方と、失敗を「有効でない」と読むことは、今の起動とチェックポイントの入口と同じである（`fable-review.md:79-103`）。
- **AC-02**: 判定が有効（`VALID`）のときは、Fable を走らせず、commit を足さずに push に進む。記録の後に中身のない commit か、Part B step 2 の直しの commit しかないブランチでは、Fable の部分レビューは PR につき 1 回だけである。
- **AC-03**: 判定が有効でない（`MISSING <START>`、または判定の失敗）ときは、push の前に、`<START>` から HEAD までについて、Fable の部分レビューを単独で行う。判定が失敗したときの `<START>` は merge-base である。流れはチェックポイントの単独の経路（`fable-review.md:208-232`）と同じで、指摘のトリアージ、直し、直しのローカルの CLI レビュー、記録の commit の順に進む。その後でもう一度判定し、有効なら push に進む。
- **AC-04**: AC-03 の 2 度目の判定が有効でないときは、push せず、PR を開かずに止まる。止まったことと、その理由を報告する。判定が失敗した（スクリプトの終了コードが 0 でない、1 行目が決まった形でない、スクリプトを取り出せない）ときは、スクリプトが標準エラーに書いたことも報告する。
- **AC-05**: AC-03 の単独の部分レビューで、Fable が走らない（`not run` の理由のどれか。`fable-review.md:460-464`）ときは、止めずに `not run` とその理由を記録の commit に残し、AC-03 の 2 度目の判定に進む。
- **AC-06**: AC-03 の単独の部分レビューが途中で止まった（状態の確かめ、メンションの確かめ、ローカルの CLI レビューが止まった）ときは、push せず、PR を開かずに止まる。

#### F-002: 記録を無効にする commit と、無効にしない commit

記録の後、push の直前までにできた commit のうち、どれが Fable の記録を無効にし、どれが無効にしないかを定める。

**Acceptance Criteria**:

- **AC-07**: タスク文書の PR で、Fable の記録の後に Part B step 2（CHECKS）の直しとして作った commit は、記録を無効にしない。その commit しか記録の後にないブランチでは、push の直前の判定は有効になり、Fable は走らない。
- **AC-08**: 記録の後に merge commit（`origin/main` の取り込みを含む）があるブランチでは、タスク文書の PR でも `/runplan` の PR でも、push の直前の判定は有効でない。merge commit がチェックポイントの印や Part B step 2 の直しの後にあっても、同じである。
- **AC-09**: `origin/main` を merge して merge-base が動いたブランチでは、AC-03 で Fable が見る範囲は、新しい merge-base から HEAD まで（PR の全体）である。merge の前の Fable の記録と見た範囲の行は、範囲を狭めない。
- **AC-10**: 今まで記録を無効にしなかった commit は、push の直前の判定でも無効にしない。対象は、中身のない commit（`fable-review.md:114-121`）、同じ実行の直し（`:122-125`）、チェックポイントの印の後のチェックポイントの実行の commit（`:126-128`）である。たとえば、記録の後に `README.en.md` だけを変える commit しかないブランチでは、Fable は走らない。
- **AC-11**: タスク文書の PR で、記録の後に Part B step 2 の直しの commit と、それ以外の中身のある commit（直しの前の手作業の commit など、merge commit 以外）の両方があるブランチでは、push の直前の判定は有効でない。Part B step 2 の直しの扱いは、ほかの commit を無効にしない commit に変えない。

#### F-003: PR の説明の、どちらの CLI もレビューしなかった commit

PR の説明の「どちらの CLI もレビューしなかった commit」（`task-pr-handoff.md:250-260`）に、記録の後にできた commit を正しく示す。

**Acceptance Criteria**:

- **AC-12**: このセッションのローカルの CLI レビューがブランチの全体を見た後に、どちらの CLI も見ずに作られ、push される commit のうち、中身のある commit を、PR の説明の「どちらの CLI もレビューしなかった commit」に挙げる。merge commit と Part B step 2 の直しの commit も、ここに含む。
- **AC-13**: AC-12 の commit のうち、中身のない commit（空の commit、`README.en.md` だけを変える commit、計画書の PR の進み具合のチェックボックスの行だけを変える commit）は挙げない。たとえば、#987 の形のブランチ（記録の commit の後に merge commit が 1 つ）では、merge commit だけを挙げ、空の記録の commit は挙げない。
- **AC-14**: AC-03 の単独の部分レビューで、直しの 2 回目の試行がどちらかの CLI でレビューした直しの commit は、AC-12 の一覧に挙げない。
- **AC-15**: AC-03 で走った Fable の部分レビューの記録は、PR の説明の `## Fable の部分レビュー` の節に、それより前の記録と並べて、別の記録として示す。

#### F-004: ADR-0050 の更新

**Acceptance Criteria**:

- **AC-16**: ADR-0050 の「決定」は、次の 4 点を述べる。push の直前に記録が有効かをもう一度判定し、有効でなければ足りない commit について Fable の部分レビューを行うこと。タスク文書の PR の Part B step 2 の直しは記録を無効にしないこと。merge commit はいつも記録を無効にすること。merge の後は Fable が PR の全体をもう一度見ること。
- **AC-17**: ADR-0050 の「採らなかった案」は、本書の「要件定義の際に決めたこと」の採らなかった案を、理由とともに挙げる。ADR-0050 の「経緯」の末尾には、本タスクの決定の 1 行を、本書へのリンクとともに足す。
- **AC-18**: 本タスクの PR は、タスク 0098 の文書（`docs/tasks/0098_fable_partial_review/` の下）を変えない。`git diff origin/main...HEAD -- docs/tasks/0098_fable_partial_review/` は空である。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   記録の後の commit が中身のない commit と Part B step 2 の直しだけのとき、push の直前の判定で増える時間は、判定のスクリプトの 1 回分である。Fable は走らない（AC-02、AC-07）。
-   merge commit のあるときは、Fable の部分レビューの 1 回分（これまでの実績で 2〜16 分。ADR-0050 の「結果」）と、その直しの確かめの分だけ、PR を開くまでの時間が延びる。この費用は受け入れる（要件定義の際に決めたこと）。

### 4.2. セキュリティ (Security)

-   `docs/dev/security.md` の条件（`yt-dlp` の起動、API キー、キャッシュ、LLM API への送信、信頼しないテキストの埋め込み）のどれにも当たらないので、N/A とする。開発の道具の手順だけを変え、製品の振る舞いを変えない。
-   判定の関門の性質は変えない。判定のスクリプトは、`origin/main` から取り出して走らせ、作業ツリーのものを使わない。失敗は「有効でない」と読む（ADR-0050、AC-01）。誤って有効と判定する側に、規則を広げない。merge commit を無効にしない commit に数えない（AC-08）。
-   Fable の出力の文の扱い（コミットメッセージにも PR の説明にも写さない）と、メンションの確かめは、単独の経路で今の規則のまま適用する（`fable-review.md:520-525`）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   Fable が使えない（エラー・利用上限・時間切れ・Fable でないモデル・エージェントが呼べない）ことで、PR を止めない（AC-05、ADR-0050）。
-   判定のスクリプトが失敗したときは、有効でないものとして Fable を走らせる。2 度目も失敗したら、止まって報告する（AC-03、AC-04）。

### 4.4. 互換性 (Compatibility)

-   コミットメッセージの記録の行の形と読み方は変えない。既存のブランチと、マージ済みの PR の記録は、そのまま読める。
-   `/runpr` の `coverage` の判定は変えない（2.3）。

### 4.5. 保守性 (Maintainability)

-   有効かの判定の規則の正本は、今のとおり `fable-review.md`「The validity decision」と、判定のスクリプトの冒頭の 2 か所に置く。それらと食い違う規則を、`task-pr-handoff.md` に書き写さない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR-0050 の、今の決定のうち本書が変えないもの（範囲の 4 つの基準、30 分の上限、モデル ID の確かめ、状態の確かめ、記録の行の形、判定のスクリプトを `origin/main` から取り出すこと）に従う。
-   判定のスクリプトを変えるときは、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の、`review-records.sh` を Bash で置く例外の範囲で変える。

## 6. 用語集 (Glossary)

-   **Fable の記録:** `Fable review: run` か `Fable review: not run` の行（`fable-review.md`「Record lines」）。
-   **記録の commit:** Fable の記録と、並列のローカルの CLI レビューの見た範囲の行を持つ commit。
-   **有効かの判定:** `review-records.sh fable` の出力が `VALID` か `MISSING <START>` か（`fable-review.md`「The validity decision」）。
-   **中身のない commit:** 親が 1 つで、何も変えない commit。`README.en.md` だけを変える commit、計画書の PR の進み具合のチェックボックスの行だけを変える commit、その 2 つを合わせた commit も含む（`fable-review.md:114-121`）。
-   **単独の経路:** ローカルの CLI レビューと並列にせず、Fable の部分レビューだけを行う流れ（`fable-review.md`「The standalone path」）。
-   **merge-base:** `git merge-base origin/main HEAD`。
-   ほかの用語は `docs/translation_glossary.md` と統一する。

## 変更履歴

- 2026-11-19 初版（draft）
