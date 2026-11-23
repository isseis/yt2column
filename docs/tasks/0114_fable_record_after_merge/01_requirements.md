# 要件定義書：Fable の部分レビューの記録の後、push の前にできた commit を、PR を開く前に確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Process | `full` |
| Created | 2026-11-19 |
| Review date | 2026-11-20 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、Fable の部分レビューの記録の後、push の前にできた commit（`origin/main` の merge commit を含む）を、PR を開く前に確かめる要件を定義する。

**背景:**

-   タスク 0098 は、タスクの PR を開く前に Fable の部分レビューを行う手順を作った（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）。ADR-0050 は、Fable が見た範囲の後に中身のある commit が足されたら、その commit についてもう一度 Fable の部分レビューを行う、と決めている（`docs/dev/adr/0050-fable-partial-review.md:28`）。
-   有効かの判定は、`origin/main` から取り出した `.claude/commands/_lib/review-records.sh` が決める（`.claude/commands/_lib/fable-review.md:79-135`）。今の手順は、その判定を 2 つの入口でしか走らせない。
    -   **起動の入口:** ローカルの CLI レビューの前に走る（`fable-review.md:137-139`）。タスク文書の PR では、`_lib/task-pr-handoff.md` Part B step 1 がここを通る（`.claude/commands/_lib/task-pr-handoff.md:184-197`）。
    -   **チェックポイントの入口:** `/runplan` の green gate の前に走る（`fable-review.md:234-248`、`.claude/commands/runplan.md:98`）。
-   どちらの入口も、判定の後、push までにできた commit をもう一度判定しない。タスク文書の PR では、記録の commit の後に Part B step 2（CHECKS）の直しや `origin/main` の merge commit ができても、Fable が見ないまま PR が開きうる。`/runplan` でも、チェックポイントの印の後に merge commit ができれば同じことが起きる。判定のスクリプトは、merge commit を、記録を無効にしない commit のどれにも数えない（`.claude/commands/_lib/review-records.sh:109`）。
-   タスク 0098 の実地の確かめで起きた（[0098 実装計画書](../0098_fable_partial_review/03_implementation_plan.md) §7「PR-3 の実行の記録」のステップ 3-3）。PR #987 では、記録の commit `5de18f65` の後に `origin/main` の merge commit `ac43c9c9` が作られた。この merge は `docs/dev/adr/0046-chief-of-staff.md` の衝突を解いている。判定のスクリプトは、記録の commit では `VALID`、merge の後では `MISSING <merge-base>` を返した。しかし、#987 は Fable を走らせ直さずに開かれた。
-   merge の後に判定のスクリプトが返す START（Fable が見る範囲の起点。出力 `MISSING <START>` の `<START>`）は、merge の前の記録の START によって分かれる。
    -   **記録の START が古い merge-base のとき（#987 の形）:** merge で merge-base が動くと、その記録は START の確かめを通らずに無視される（`review-records.sh:46-52`、`:87-91`）。スクリプトは `MISSING <新しい merge-base>` を返し、Fable は PR の全体を見る。
    -   **記録の START がブランチの commit のとき（単独の経路や、2 回目以降の部分レビューの記録）:** その記録は merge の後も有効なまま残る。記録の後の commit のうち、記録を無効にしない規則のどれにも当たらない最初のものが merge commit なので、スクリプトは `MISSING <merge commit の第 1 親>` を返す（`review-records.sh:570-578`）。この範囲の diff には、merge で入った `main` 側の変更がすべて入る。merge の前の PR の commit は、前の記録がすでに見ている。Fable の指摘のうち、ブランチで変わっていないファイルの指摘は、範囲にないとして捨てられる（`fable-review.md:347-352`）。
-   `/runpr` の `coverage` は、Codex の最後の PR レビューより後の commit だけを判定する（`review-records.sh:10-11`、`:26-28`）。PR を開く前の merge commit は、Codex の最初の PR レビューが見るので、普通は判定の対象にならない。対象になった merge commit は「見ていない」と数えられ、`PARTIAL` になる（`review-records.sh:42`）。そのとき `/runpr` は、escalation の判定によって、人に問うか、PR レビューを頼むか、頼まずに進む（`.claude/commands/runpr.md:700-717`）。`coverage` は、merge で merge-base が動くと、古い merge-base から始まる見た範囲の行も無視する（`review-records.sh:46-52`）。
-   PR の説明の「どちらの CLI もレビューしなかった commit」は、このセッションのレビューがブランチの全体を見たとき、そのレビューの SUMMARY にある範囲だけを挙げる（`task-pr-handoff.md:250-260`）。記録の後にできた commit を書く規則はない。#987 の PR の説明は、merge commit と空の記録の commit の 2 つを挙げた（0098 実装計画書 §7 のステップ 3-1）。

対応 issue: [#988](https://github.com/isseis/yt2column/issues/988)（関連: [#924](https://github.com/isseis/yt2column/issues/924)（タスク 0098）、PR [#987](https://github.com/isseis/yt2column/pull/987)（食い違いを観察した PR））

本書は、次のことを決める（「CHECKS の直し」「タスクの PR」「push の直前」の意味は 3.1 で定める）。

-   push の直前に、Fable の記録が有効かをもう一度判定すること。
-   どの commit が記録を無効にし、どれが無効にしないか。
-   記録の後の merge commit を、Fable の代わりに確かめる「merge の後の確認」。
-   Fable を走らせ直すときに Fable が見る範囲。
-   PR の説明への書き方。
-   ADR-0050 の更新。

判定を手順のどこに置くか、判定のスクリプトの規則の書き方、CHECKS の直しを見分ける方法は `02_architecture.md` が決める（[design_handoff.md](design_handoff.md)）。

### issue #988 で決めたこと

-   **扱うタスク：** #988 の食い違いはタスク 0098 では直さず、この issue で扱う。0098 の実地の確かめで見つかり、人と相談して決めた（0098 実装計画書 §7 のステップ 3-3、0098 要件定義書 AC-42 の例外）。
    -   ADR：なし。どのタスクで扱うかの選択で、決定ではない。
-   **ADR-0050 の更新：** 直すときは、ADR-0050 も合わせて直す。記録の後の commit の扱いは ADR-0050 の決定であり、それを変えるためである。
    -   ADR：[ADR-0050](../../dev/adr/0050-fable-partial-review.md) を更新する（下の「要件定義の際に決めたこと」）。

### 要件定義の際に決めたこと（2026-11-19）

-   **進め方：** 通常の進め方（`full`）とする。この変更は、除外される変更に当たらない。通常の進め方を勧めた理由は 3 つある。ADR-0050 の決定を変えること、共有の手順と判定のスクリプト（`_lib/task-pr-handoff.md`・`_lib/fable-review.md`・`_lib/review-records.sh`）に及ぶこと、決めることが複数あることである。除外の判断は次のとおりである。`docs/dev/security.md` の規則と前提を変えない。レビューのスレッドや Bot への依頼の扱いに触れず、`fixprguard`・`codexreview`・`mergepr` も変えないので、開発の道具の信頼境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）を変えない。外部依存も足さない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。人の答え：通常の進め方。
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   **push の直前の再判定：** push の直前に、Fable の記録が有効かをもう一度判定する。有効でなければ、次のどちらかを単独で行う。記録を無効にしているのが、未確認の merge commit（merge の後の確認の記録がまだない merge commit。6 章で定義）だけなら、merge の後の確認を行う。ほかの commit も記録を無効にしているなら、足りない commit について Fable の部分レビューを行う。タスク文書の PR の CHECKS の直しは、`/runplan` の green gate の直しと同じく、記録を無効にしない。merge commit は、merge の後の確認の記録がない限り、記録を無効にする（F-001、F-002、F-005）。今は判定が入口でしか走らないことが、#987 の食い違いの原因だからである。CHECKS の直しを無効にしないのは、文書のガードのテストを通すための直しのたびに Fable が走り直さないようにするためである。これで、タスク文書の PR の扱いが `/runplan` のチェックポイントの扱いと揃う。人の答え：再判定し、Part B の直しは除く。
    -   採らなかった案：(B) CHECKS の直しにも Fable を走らせ直す（今の判定の規則のまま）。CHECKS の直しのたびに、Fable の 1 回分（2〜16 分）の費用がかかる。(C) 再判定して有効でなければ、自動では走らせずに止め、人に問う。人の手間が増えるうえ、人が選べるのは Fable を走らせることだけである。
    -   ADR：[ADR-0050](../../dev/adr/0050-fable-partial-review.md) を更新する。記録の後の commit の扱いという ADR-0050 の決定を変えるので、主要な決定に当たる（CLAUDE.md「Design decisions (ADR)」）。
-   **再判定を入れる PR：** push の直前の再判定は、タスク文書の PR と `/runplan` の PR の両方に入れる（F-001）。`/runplan` のチェックポイントの印の後にできた merge commit にも同じ穴がある。どちらの PR も、push を `_lib/task-pr-handoff.md` Part B で行うので、1 か所で直せる。人の答え：含める。
    -   ADR：ADR-0050 を更新する（上の決定と同じ項目）。
-   **CHECKS の直しの見分け方：** タスク文書の PR でも、`/runplan` のチェックポイントと同じく、Part B step 2 の前に記録が有効かを判定して印を残し、印の後の commit（merge commit を除く）を記録を無効にしない commit とする。印の後の commit かどうかは、判定のスクリプトが commit のデータから決め、セッションが手順の文を読んで決めない（F-002）。commit のデータだけでは、CHECKS の直しとほかの中身のある commit を見分けられないためである。印の後に人やほかのセッションが手で足した中身のある commit は、Fable が見ないまま PR が開く。この危険は、`/runplan` のチェックポイントの印について ADR-0050 がすでに受け入れているものと同じ形で受け入れる。その commit も、PR を開いたときに Codex の PR レビューが見る。人の答え：印の後を除き、危険を受け入れる。
    -   経緯：最初の版は、「CHECKS の直しだけを、判定のスクリプトが commit のデータから見分ける」と書き、人に問わずに決めていた。Fable の部分レビューで、commit のデータだけでは見分けられないと分かったので、人に問い、この形に決めた。
    -   採らなかった案：印の後の commit も無効にし、CHECKS の直しのたびに Fable を走らせ直す（「push の直前の再判定」の採らなかった案 B と同じ）。
    -   ADR：ADR-0050 を更新する。
-   **merge の後の確認：** 記録の後の merge commit は、Fable を走らせ直さず、Opus のサブエージェントが確かめる（merge の後の確認）。範囲に入れるのは、merge で手で解いた箇所（衝突の解決）と、merge で `main` から入った ADR・先行のタスク文書・コマンドの定義の変更を、PR の文書やコードが参照している箇所だけとする。merge の前の merge-base を START に持つ Fable の記録も、merge の後に記録として読む（AC-32）。そうしないと、最初の記録の後の merge（#987 の形）で記録が無視され、Fable が PR の全体を走り直すためである。確かめたら、その merge commit を名指しした記録を残し、その記録がある merge commit は、Fable の記録を無効にしない（F-002、F-005）。merge で新しく見るべきものは、merge commit 自身の変更と、`main` で変わった文書との整合だけで、PR のほかの箇所は Fable の記録がすでに見ている。範囲が決まった確認なので Opus で足り、Fable の 1 回分（2〜16 分、最大 260k トークン）より費用が小さい。並行するタスクが ADR を触る今の運用では、#987 のように衝突を解く merge は稀とは限らない。確認は、衝突を解いたセッション自身ではなく、別のサブエージェントにさせる。セッション自身の盲点を外さないためである。Fable の代わりではなく、merge commit だけを受け持つ別のレビューとして記録するので、「Fable で走らせられないときに Opus で代える」案を採らなかった ADR-0050 の決定とは衝突しない。人の答え：A（Opus のサブエージェントが merge に関わる箇所だけを確かめる）。
    -   経緯：最初の問いでは、「今の安全な側の動き（merge の後は PR の全体を見る）を保ち、判定のスクリプトは変えない」と決めた。セルフレビューで、今の動きが記録の START によって分かれることが分かったので問い直し、「merge の後は、いつも Fable が PR の全体を見る」に決め直した。その後、人が、merge の後に Fable を PR の全体に走らせ直す費用対効果を問うた。PR の全体を見直す費用の大部分は、すでに見た箇所を読み直す分である。そこでこの形に決め直した。
    -   採らなかった案：(B) merge の後に、Fable が PR の全体を見直す（前の決定）。費用の大部分が、すでに見た箇所の読み直しになる。(C) Fable を走らせ直し、範囲に入れる箇所を merge に関わる場所に絞る。判定のスクリプトは変えずに済むが、費用は Fable の 1 回分かかる。(D) merge commit を記録を無効にしない commit に数え、PR を開いたときの Codex の PR レビューに任せる。#987 の穴が残る。(E) 今のまま受け入れ、記録によって範囲が分かれる振る舞いを要件に書く。範囲に `main` 側の変更が入ってノイズになる。(F) Part B step 1 の前に `main` を取り込む手順を足し、記録の後の merge を減らす。merge の要る理由（衝突など）は記録の後にも起こりうるので、減らしても、merge の後の扱いは別に要る。
    -   ADR：ADR-0050 を更新する。merge commit の扱いという ADR-0050 の決定を変え、判定の関門の規則を変えるので、主要な決定に当たる。
-   **merge の後の確認が走らないとき：** Opus を呼べない（OpenCode で動く `/runplan` など）、エラー、利用上限のときは、止めずに `not run` とその理由を、merge commit を名指しして記録し、push に進む（F-005）。Fable の部分レビューが走らないときと扱いを揃える（ADR-0050）。merge は、PR を開いたときに Codex の PR レビューが見る。人の答え：not run と記録して進む。
    -   ADR：ADR-0050 を更新する。
-   **Fable を走らせ直すときの範囲：** 記録の後に、merge commit 以外の中身のある commit があって Fable を走らせ直すとき、その範囲の中に merge commit があれば、Fable は PR の全体（新しい merge-base から HEAD まで）を見る（F-002）。merge の前の記録から範囲を決めると、範囲に merge で入った `main` 側の変更が入り、Fable の指摘が捨てられるノイズになるためである。この決定は、上の決め直しの前に人が答えた「merge の後は PR の全体を見る」を、Fable が走る場合に残したものである。
    -   ADR：ADR-0050 を更新する。
-   **`/runplan` の Opus のレビュー：** `/runplan` で、push の直前の判定の時点に、Opus のレビューの判断の後にできた除外できない commit（push の直前の単独の部分レビューの直しの commit と、merge commit を含む）があれば、Part B step 2 の除外できない commit と同じく、Opus のレビューの判断をやり直す（F-001）。今の `/runplan` は、Part B step 2 の commit で Opus のレビューの判断をやり直す（`runplan.md:106-118`）。Fable の直しも merge commit も同じ種類の commit なので、扱いを揃える。merge commit は、Opus のレビューの判断では除外できない commit である（`.claude/commands/_lib/opus-review.md:102-104`）。人の答え：Part B step 2 と同じ。merge commit のときもやり直す。
    -   ADR：なし。`/runplan` の Opus のレビューの既存の規則（[ADR-0042](../../dev/adr/0042-implementation-model-tiers.md)）を、新しい commit にも当てはめるだけで、決定を変えない。
-   **0098 の文書：** タスク 0098 の承認済みの文書は直さない。ADR-0050 の決定と経緯だけを更新する（F-004）。今の決定とその経緯は ADR が持ち、タスクの文書はそのタスクで決めたことの記録だからである（CLAUDE.md「Design decisions (ADR)」の「Who holds what」）。0098 の文書は、#988 を既知の例外としてすでにリンクしている（0098 要件定義書 AC-42、[0098 設計書](../0098_fable_partial_review/02_architecture.md) 3.10）。issue は「0098 の設計書の該当箇所もあわせて直す」と書いていたが、人と決めてこの形にした。人の答え：直さない。
    -   ADR：ADR-0050 を更新する（経緯に本タスクを足す）。
-   **PR の説明の一覧：** PR の説明の「どちらの CLI もレビューしなかった commit」には、記録の後・push の前にできた commit も含め、どちらの CLI も見ていない commit を挙げる。挙げない commit は、空の commit、計画書の PR の進み具合のチェックボックスの行だけを変える commit、ローカルの CLI レビューの見た範囲の行（`Local CLI reviewed:`）の範囲に入る commit である。中身のない commit の定義は `/runpr` の `coverage` と揃え、`README.en.md` だけを変える commit は挙げる（`review-records.sh:31-42`）。見た範囲の行は、merge で merge-base が動いた後も、範囲に入る commit を見たものとして数える。ここが `coverage` と違う（F-003）。今の規則は、記録の後にできた commit の扱いを決めていない。#987 では、空の記録の commit まで挙がった。`coverage` の定義をそのまま使うと、merge の後は、CLI がレビューした commit まで挙がる（#987 のブランチでは 6 件。Fable の部分レビューの実験）。人の答え：中身のある commit だけ挙げる。中身のない commit の定義は `/runpr` と揃え、`README.en.md` だけを変える commit は挙げる。merge の前に CLI が見た commit は、merge の後も挙げない。
    -   ADR：なし。PR の説明の書き方は、`_lib/task-pr-handoff.md` の 1 つの手順の中で閉じる。
-   **止まった後の扱い：** push の直前の判定や単独の部分レビューで止まったときは、報告して止まるだけとし、やり直しの手段は本タスクで作らない（F-001）。今の Part B step 1 で止まったときも、タスク文書のコマンドを走らせ直すと、Part A が「commit があって PR のないブランチ」で止まる（`task-pr-handoff.md:101-103`）。止まった後の再開は、それと合わせて別に扱う。人の答え：報告だけ、再開はスコープ外。
    -   ADR：なし。本タスクの範囲の選択である。
-   **確かめられない merge commit（2026-11-20、設計の際に決め直した）：** merge の後の確認は、範囲を正しく求められる merge commit（確認できる merge commit。6 章で定義）だけに行う。確認できない merge commit と、範囲を求める git が失敗した（終了コードが 0 でないか、標準エラーに何か書いた）merge commit は、merge commit でない中身のある commit と同じに扱い、Fable が PR の全体（merge-base から HEAD まで）を見る（AC-13、AC-24、AC-26）。親が 3 つ以上の merge commit では `git show --remerge-diff` が差分を出さずに終了コード 0 で終わり、`main` の側で作った merge commit では `main` から入った変更を取り違えるので、確認しても手で入れた変更が範囲に入らず、誰も見ないまま確認済みに数えられるためである。範囲を求める git の失敗で止まらないのは、止まった後の再開の手段がない（上の「止まった後の扱い」）ためである。設計の Fable の部分レビューで、設計書が人に問わずにこの扱いを入れ、要件と食い違っていると分かったので、人に問うた。人の答え：どちらも Fable へ。
    -   採らなかった案：(A) 確認できない merge commit は Fable に回し、範囲を求める git の失敗では止まる。止まった後の再開の手段がない。(B) 要件のとおり、どの merge commit も確認に回し、git の失敗は `not run` と記録して進む。手で入れた変更を誰も見ないまま、merge commit が確認済みに数えられる。
    -   ADR：[ADR-0050](../../dev/adr/0050-fable-partial-review.md) を更新する（決定の節「5. Fable が見た後に足された commit は、判定のスクリプトが分け、push の直前に確かめ直す」の、merge commit の扱い）。判定の関門が確認済みに数える merge commit の範囲を決めるので、主要な決定に当たる。

-   **merge の後の確認の指摘の場所（2026-11-20、計画の際に決め直した）：** merge の後の確認の指摘は、Fable の規則（merge-base から HEAD までに変わったファイル）にある場所に加えて、merge の後の確認の範囲の一覧（AC-26）にあるパスの場所も有効とする（4.2）。merge の後は merge-base が新しい `origin/main` になるので、衝突を `main` 側の内容に寄せて解いたファイルや、解く際に消したファイルは PR の差分から消え、Fable の規則のままでは、確認したい箇所への指摘が INVALID になるためである。計画のレビューで分かり、人に問うた。人の答え：規則を変え、範囲の一覧のパスだけを加える。
    -   採らなかった案：(A′) merge commit が親のどちらかに対して変えたパスも有効とする。`main` 側だけが変えたファイル、つまり PR が触れていないファイルへの指摘までトリアージに入る。(B) Fable の規則のまま受け入れる。確認したい箇所の指摘が、トリアージに入らない。
    -   ADR：なし。merge の後の確認の中の、出力の確かめの細部で、本タスクの中で閉じる（[02_architecture.md](02_architecture.md) 1.2 の決定 13）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   記録の後に merge commit 以外の中身のある commit ができたとき、Fable がそれを見ないまま、タスクの PR が開かれないようにする。未確認の merge commit を含んだまま、PR が開かれないようにする。
-   記録の後の merge commit は、Fable を走らせ直さず、Opus のサブエージェントが merge に関わる箇所だけを確かめる。
-   タスク文書の PR と `/runplan` の PR で、記録を無効にしない commit の扱いを揃える。
-   PR の説明で、どちらの CLI もレビューしなかった commit を、記録の後にできたものも含めて正しく示す。

### 2.2. スコープ (In Scope)

-   `_lib/task-pr-handoff.md` Part B の、push の直前の Fable の記録の再判定と、有効でないときの単独の Fable の部分レビュー（F-001）。
-   `/runplan` で、push の直前の単独の部分レビューの直しと merge commit について、Opus のレビューの判断をやり直すこと（F-001、AC-09）。
-   タスク文書の PR の Part B step 2 の前の印と、印の後の commit を記録を無効にしない commit にすること（F-002）。
-   merge の後の確認と、その記録で merge commit を記録を無効にしない commit にする判定のスクリプトの規則（F-005）。
-   判定のスクリプトの、merge で merge-base が動いた後の記録の START の確かめ（AC-32）と、Fable を走らせ直すときの範囲の START の決め方（F-002）。
-   PR の説明の「どちらの CLI もレビューしなかった commit」の規則（F-003）。
-   ADR-0050 の決定・採らなかった案・結果・経緯の更新（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   **未確認の merge commit の除外：** 未確認の merge commit を、記録を無効にしない commit に数えること（要件定義の際に決めたこと）。
-   **`/runpr` の `coverage` の判定と、マージの質問の変更。** merge commit で `PARTIAL` になる動きは変えない。
-   **PR を開いた後の変更を Fable に見せること。** Fable の部分レビューは PR を開く前のものである（ADR-0050）。PR を開いた後に `/runpr` や `/fixpr` が `main` を取り込んだ merge は、本タスクで扱わない。
-   **タスク 0098 の文書の変更**（要件定義の際に決めたこと）。
-   **Part B の前に `main` を取り込む手順の追加**（要件定義の際に決めたことの採らなかった案 D）。
-   **ローカルの CLI レビューを、記録の後の commit について走らせ直すこと。** 記録の後の commit は、PR を開いたときに Codex の PR レビューが見る。PR の説明には、どちらの CLI もレビューしなかった commit として挙げる（F-003）。ただし、単独の部分レビューの直しは、今の単独の経路のとおり、ローカルの CLI レビューで確かめる（AC-03）。
-   **止まった後の再開の手段**（要件定義の際に決めたこと）。
-   **#987 の PR の扱い。** すでにマージされており、本タスクでは何もしない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

本章では、次の 3 つの語を、それぞれ次の意味で使う。

-   **タスクの PR:** タスク文書の PR（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2` が開く PR）と、`/runplan` の PR。
-   **CHECKS の直し:** `_lib/task-pr-handoff.md` Part B step 2 で、CHECKS を通すために作る commit。`/runplan` では green gate の直しに当たる。
-   **push の直前:** Part B で push と PR の作成（`_lib/task-pr-handoff.md` Part B step 3）を行う前。Part B step 1 の README の同期と step 2 の CHECKS を、直しを含めて終えた後を指す。

#### F-001: push の直前の、Fable の記録の再判定

タスクの PR を push する直前に、Fable の記録が有効かをもう一度判定する。有効でなければ、merge の後の確認（F-005）か、足りない commit についての Fable の部分レビューを単独で行ってから push する。

**Acceptance Criteria**:

- **AC-01**: タスクの PR の push の直前に、`origin/main` から取り出した判定のスクリプトで、ブランチの Fable の記録が有効かを判定する。タスク文書の PR でも、`/runplan` の PR でも判定する。判定の取り出し方と、失敗を「有効でない」と読むことは、今の起動とチェックポイントの入口と同じである（`fable-review.md:79-103`）。
- **AC-02**: 判定が有効（`VALID`）のときは、Fable を走らせず、commit を足さずに push に進む。
- **AC-03**: 判定が Fable の部分レビューを求めるときは、push の前に、判定が示す起点 `<START>` から HEAD までについて、Fable の部分レビューを単独で行う。判定が Fable の部分レビューを求めるのは、記録がないとき、未確認の merge commit 以外の commit が記録を無効にしているとき（未確認の merge commit があるかどうかによらない）、判定が失敗したときである。判定が失敗したときの `<START>` は merge-base である。流れはチェックポイントの単独の経路（`fable-review.md:208-232`）と同じで、指摘のトリアージ、直し、直しのローカルの CLI レビュー、記録の commit の順に進む。Fable に渡す KIND は、タスク文書の PR では `documents`、`/runplan` の PR では `implementation` である。SELF_REJECTED には、そのセッションがそれまでのレビューで却下した指摘を渡す（`fable-review.md:35-47`）。
- **AC-04**: AC-03 の単独の部分レビューか、AC-24 の merge の後の確認が commit を作ったら、Part B step 1 の README の同期と step 2 の CHECKS を、push の前にもう一度走らせる。ここでできた CHECKS の直しと `README.en.md` の commit も、AC-10 と AC-11 のとおり記録を無効にしない。
- **AC-05**: AC-03 か AC-24 と、AC-04 の後に、もう一度判定する。有効なら push に進む。push の前の最後の手順は、この判定である。push するのは、判定した commit である。判定の後、push までの間にブランチの先頭が判定した commit から動いたら、push せずに判定からやり直す。
- **AC-06**: AC-05 の判定が有効でないときは、push せず、PR を開かずに止まる。判定が失敗した（スクリプトを取り出せない、終了コードが 0 でない、1 行目が決まった形でない）ときも止まる。止まったことを報告する。報告には、止まった理由、判定の出力の 1 行目、判定が失敗したときはスクリプトが標準エラーに書いたこと、今の HEAD、止まるまでに作った commit を含める。
- **AC-07**: AC-03 の単独の部分レビューで、Fable が走らない（`not run` の理由のどれか。`fable-review.md:460-464`）ときは、止めずに `not run` とその理由を記録の commit に残し、AC-05 の判定に進む。
- **AC-08**: AC-03 の単独の部分レビューか、AC-24 の merge の後の確認が途中で止まったときは、push せず、PR を開かずに止まる。止まった理由、今の HEAD、止まるまでに作った commit を報告する。途中で止まるのは、作業ツリーの確かめ、状態の確かめ、メンションの確かめ、ローカルの CLI レビューのどれかが止まったときである（`fable-review.md:208-232`）。
- **AC-09**: `/runplan` の PR で、Opus のレビューの判断の後に除外できない commit（AC-03 の単独の部分レビューや AC-24 の merge の後の確認の直しの commit、または merge commit）があれば、Part B step 2 の commit と同じく、push の前に Opus のレビューの判断をやり直す（`runplan.md:106-118`）。たとえば、Opus のレビューを終えた後に `origin/main` を merge し、merge の後の確認が何も直さなかったブランチでも、やり直す。その後に、AC-04 と AC-05 に進む。PR の説明の Opus のレビューの結果も、やり直した判断に合わせる。

#### F-002: 記録を無効にする commit と、無効にしない commit

記録の後、push の直前までにできた commit のうち、どれが Fable の記録を無効にし、どれが無効にしないかを定める。どれも判定のスクリプトが commit のデータ（印の commit の位置を含む）から決め、セッションが手順の文を読んで決めない。

**Acceptance Criteria**:

- **AC-10**: 今まで記録を無効にしなかった commit は、push の直前の判定でも無効にしない。対象は、中身のない commit（`fable-review.md:114-121`）、同じ実行の直し（`:122-125`）、チェックポイントの印の後のチェックポイントの実行の commit（`:126-128`）である。たとえば、記録の後に `README.en.md` だけを変える commit しかないブランチでは、Fable は走らない。
- **AC-11**: タスク文書の PR で、記録が有効なうちに Part B step 2 の前の印が残され、記録の後に印と CHECKS の直ししかないブランチでは、push の直前の判定は有効になり、Fable は走らない。
- **AC-12**: タスク文書の PR で、記録と印の間に中身のある commit（CHECKS の前に作った手作業の commit など）があるブランチでは、push の直前の判定は有効でない。印の後の commit は、手で足した中身のある commit でも記録を無効にしない（受け入れた危険。要件定義の際に決めたこと）。
- **AC-13**: 記録の後に未確認の merge commit（`origin/main` の取り込みを含む）があるブランチでは、タスク文書の PR でも `/runplan` の PR でも、push の直前の判定は有効でない。merge commit がチェックポイントの印や CHECKS の直しの後にあっても、同じである。そのとき、ほかに記録を無効にしている commit がなければ、判定は Fable ではなく merge の後の確認を求め、Fable は走らない（AC-24）。確認できない merge commit（6 章）は、merge commit でない中身のある commit と同じに扱い、判定は Fable の部分レビュー（範囲は merge-base から HEAD まで）を求める。
- **AC-14**: 記録の後に、merge commit 以外の中身のある commit があって Fable を走らせ直すとき、記録の後に merge commit が 1 つでもあれば、Fable が見る範囲は新しい merge-base から HEAD まで（PR の全体）である。merge の前の記録の START が古い merge-base でも、ブランチの commit でも、同じである。たとえば、記録の後に中身のある commit X を足してから `origin/main` を merge したブランチでは、判定は `MISSING <新しい merge-base>` を返す。
- **AC-15**: Fable の起動の後、記録の commit の前に merge commit ができたブランチでも、push の直前の判定は、その merge commit について merge の後の確認を求める。merge の前の記録の START が古い merge-base でも、その記録は無視されない（AC-32）。AC-03 の単独の部分レビューや merge の後の確認の途中で merge commit ができたときは、AC-05 の判定が有効でなくなり、AC-06 のとおり止まる。
- **AC-32**: 記録の START が merge の前の merge-base で、その後の `origin/main` の merge で merge-base が動いたブランチでも、判定のスクリプトはその記録を記録として読む。たとえば、#987 の形のブランチ（START が古い merge-base の記録の commit の後に merge commit が 1 つ）では、判定は merge の後の確認だけを求め、Fable を求めない。

#### F-003: PR の説明の、どちらの CLI もレビューしなかった commit

PR の説明の「どちらの CLI もレビューしなかった commit」（`task-pr-handoff.md:250-260`）に、記録の後にできた commit を正しく示す。

**Acceptance Criteria**:

- **AC-16**: PR の説明の「どちらの CLI もレビューしなかった commit」は、`origin/main..HEAD` の commit のうち、空の commit、計画書の PR の進み具合のチェックボックスの行だけを変える commit、ブランチの見た範囲の行（`Local CLI reviewed:`）の範囲に入る commit のどれでもないものを挙げる。タスク文書の PR でも、`/runplan` の PR でも同じである。記録の後・push の前にできた merge commit、CHECKS の直し、`README.en.md` だけを変える commit も、どちらの CLI も見ていなければ挙げる。
- **AC-17**: 見た範囲の行の範囲に入る commit は、その後に `origin/main` を merge して merge-base が動いても挙げない。たとえば、#987 の形のブランチ（CLI がレビューした commit の後に、空の記録の commit と merge commit が 1 つずつ）では、merge commit だけを挙げる。
- **AC-18**: `/runplan` の PR で、チェックポイントの印の後に merge commit があるブランチでは、その merge commit を挙げる。
- **AC-19**: AC-03 の単独の部分レビューが作った直しの commit のうち、その直しのローカルの CLI レビューで、どちらかの CLI が見たものは挙げない。
- **AC-20**: AC-03 で走った Fable の部分レビューの記録は、PR の説明の `## Fable の部分レビュー` の節に、それより前の記録と並べて、別の記録として示す。

#### F-004: ADR-0050 の更新

**Acceptance Criteria**:

- **AC-21**: ADR-0050 の「決定」は、記録を無効にする commit と無効にしない commit について、本書の決定だけを述べ、前の決定と食い違う文を残さない。述べるのは次の 4 点である。push の直前に記録が有効かをもう一度判定し、有効でなければ足りない commit について Fable の部分レビューを行うこと。タスク文書の PR でも Part B step 2 の前に印を残し、印の後の commit は記録を無効にしないこと（印の後に手で足した commit の危険を受け入れることを含む）。merge commit は、merge の後の確認（Opus のサブエージェントが merge に関わる箇所だけを確かめる）の記録がない限り記録を無効にすること、確認が走らないときは `not run` と記録して進むこと。Fable を走らせ直す範囲に merge commit があれば Fable が PR の全体を見ること。決定のうち、まだ手順と判定のスクリプトに入っていない部分は、#988 にリンクする食い違いの文で示す（`docs/dev/conventions/adr.md`「Discrepancy statements」）。
- **AC-22**: ADR-0050 の「採らなかった案」は、本書の「要件定義の際に決めたこと」の、ADR-0050 を更新する決定の採らなかった案を、理由とともに挙げる。ADR-0050 の「経緯」の末尾には、本タスクの決定の 1 行を、本書へのリンクとともに足す。
- **AC-23**: 本タスクの PR は、タスク 0098 の文書（`docs/tasks/0098_fable_partial_review/` の下）を変えない。`git diff origin/main...HEAD -- docs/tasks/0098_fable_partial_review/` は空である。

#### F-005: merge の後の確認

記録の後の merge commit を、Fable を走らせ直さずに、Opus のサブエージェントが merge に関わる箇所だけを確かめる。

**Acceptance Criteria**:

- **AC-24**: push の直前の判定で、記録を無効にしているのが未確認の merge commit だけのときは、Fable を走らせず、それらの merge commit について merge の後の確認を行う。たとえば、#987 の形のブランチ（記録の commit の後に、衝突を解いた merge commit が 1 つ）では、merge の後の確認が 1 回走り、Fable は走らない。merge の後の確認の範囲を求める git が失敗した（終了コードが 0 でないか、標準エラーに何か書いた）ときは、その merge commit を確認せず、確認の記録も残さずに、merge-base から HEAD までについて Fable の部分レビューを単独で行う（AC-03 の流れ）。
- **AC-25**: merge の後の確認は、Opus のサブエージェントが行う。セッション自身は行わない。サブエージェントが報告するモデル ID で、Opus で走ったかを確かめる。Opus でなかったときは、その結果を使わず、`not run` とする（AC-28）。
- **AC-26**: merge の後の確認の範囲に入れるのは、次の 2 種類の箇所だけである。(1) merge commit で手で解いた箇所（`git` が自動で作る merge の結果と、merge commit の内容が違う箇所）。(2) merge で `main` から入った変更のうち、ADR・先行のタスク文書・コマンドの定義の変更を、PR が変えた文書やコードが参照している箇所。どちらにも当たる箇所がないときも、確認を走らせ、当たる箇所がないことを確かめさせる。
- **AC-27**: 確認の結果は、merge commit を名指しした決まった形の行で、merge の後の確認の記録の commit に残す。判定のスクリプトは、その行が `origin/main..HEAD` の commit にあり、名指しした merge commit がその commit の祖先であるときだけ、その merge commit を記録を無効にしない commit に数える。名指しした commit が merge commit でないとき、ブランチにないとき、行の形が違うときは、数えない。
- **AC-28**: merge の後の確認が走らない（Opus を呼べない、エラー、利用上限、時間切れ、Opus でないモデル、読めない出力）ときは、止めずに `not run` とその理由を、merge commit を名指しした行で merge の後の確認の記録の commit に残し、AC-05 の判定に進む。この記録も、AC-27 のとおり merge commit を記録を無効にしない commit に数えさせる。
- **AC-29**: merge の後の確認のサブエージェントには、Fable の部分レビューと同じ制約を課し、起動の前と終わった後に同じ状態の確かめを行う（`fable-review.md`「Authorization and constraints」「The state check」）。状態が変わっていたら、結果を使わず、push せずに止まり、AC-06 と同じ内容を報告する。
- **AC-30**: merge の後の確認の指摘は、Fable の指摘と同じトリアージの規則で扱う（却下の前に根拠を確かめる。`local-cli-review.md`「Triage rules」）。直しの commit は、ローカルの CLI レビューで確かめる。判定のスクリプトが記録を無効にしない commit に数えるのは、名指しした merge commit の子孫で、確認の記録の commit の祖先（その commit を含む）であり、その commit の見た範囲の行の範囲に入る commit だけである。merge commit より前の commit は、見た範囲の行に入っても数えない。
- **AC-31**: PR の説明は、merge の後の確認のそれぞれについて、走ったか（`not run` ならその理由）、名指しした merge commit、範囲に入れた箇所、直した指摘の数、却下・`deferred`・`unclear` の指摘を、場所とセッション自身の要約で示す。確認のサブエージェントの出力の文は写さない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   記録の後の commit が、記録を無効にしない commit だけのとき、push の直前の判定で増える時間は、判定のスクリプトの 1 回分である。Fable は走らない（AC-02、AC-10、AC-11）。
-   記録の後に記録を無効にする commit が merge commit だけのときは、merge の後の確認（Opus のサブエージェント 1 回）に、その直しの確かめと CHECKS にかかる時間を加えた分だけ、PR を開くまでの時間が延びる。Fable は走らない（要件定義の際に決めたこと）。
-   記録の後に merge commit 以外の中身のある commit があるときは、Fable の部分レビューの 1 回分（これまでの実績で 2〜16 分。ADR-0050 の「結果」）に、その直しの確かめと CHECKS にかかる時間を加えた分だけ延びる。

### 4.2. セキュリティ (Security)

-   `docs/dev/security.md` の条件（`yt-dlp` の起動、API キー、キャッシュ、LLM API への送信、信頼しないテキストの埋め込み）のどれにも当たらないので、N/A とする。開発の道具の手順だけを変え、製品の振る舞いを変えない。
-   判定の関門の性質は保つ。判定のスクリプトは、`origin/main` から取り出して走らせ、作業ツリーのものを使わない。失敗は「有効でない」と読む（ADR-0050、AC-01）。有効と判定する側を広げるのは、タスク文書の PR の印の後の commit だけで、印の後かどうかはスクリプトが決める（AC-11、AC-12）。merge commit は、その merge commit を名指しした merge の後の確認の記録があるときだけ、無効にしない commit に数える（AC-13、AC-27）。印の後に人やほかのセッションが手で足した中身のある commit を Fable が見ない危険は残る。この危険は、`/runplan` のチェックポイントの印で ADR-0050 がすでに受け入れているのと同じ形で、人と決めて受け入れた（要件定義の際に決めたこと）。push するのは判定した commit なので、判定の後にできた commit は、判定されないまま push されない（AC-05）。Fable を走らせ直すときの START の変更（AC-14）は、Fable が見る範囲を決めるだけで、有効と判定する側を広げない。merge の後の確認のサブエージェントには、Fable と同じ制約（作業ツリーとリポジトリの状態を変えない、ネットワークを使わない、秘密のファイルを読まない）を課し、Fable と同じ状態の確かめを行う（AC-29）。その出力の文は、Fable の出力と同じく信頼しない入力として扱い、コミットメッセージにも PR の説明にも写さない。
-   信頼しない入力として新しく読むのは、merge の後の確認のサブエージェントの出力だけである。その受け入れる形とメンションの確かめは、Fable の出力と揃える（AC-29〜AC-31）。ただし、指摘の場所の規則だけは、Fable の規則（merge-base から HEAD までに変わったファイル）に加えて、merge の後の確認の範囲の一覧にあるパスも有効とする（要件定義の際に決めたこと「merge の後の確認の指摘の場所」）。判定のスクリプトの出力の受け入れる形（`fable-review.md:99-103`）と、Fable の出力の受け入れる形（`fable-review.md:330-352`「Checking the output」）は変えない。Fable の出力の文をコミットメッセージにも PR の説明にも写さないこと（`fable-review.md:332-338`）と、メンションの確かめ（`fable-review.md:520-525`）も、単独の経路で今の規則のまま適用する。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   Fable が使えない（エラー・利用上限・時間切れ・Fable でないモデル・エージェントが呼べない）ことで、PR を止めない（AC-07、ADR-0050）。
-   merge の後の確認が走らない（Opus を呼べない、エラー、利用上限、時間切れ）ことでも、PR を止めない（AC-28）。
-   判定のスクリプトが失敗したときは、有効でないものとして Fable を走らせる。最後の判定も失敗したら、止まって報告する（AC-03、AC-06）。

### 4.4. 互換性 (Compatibility)

-   既存の記録の行の形は変えない。新しく足す行は、merge の後の確認の行だけである。読み方を変えるのは、merge で merge-base が動いた後の記録の START の確かめ（AC-32）と、確認の記録がある merge commit（AC-27）だけである。既存のブランチと、マージ済みの PR の記録は、そのまま読める。
-   判定のスクリプトは `origin/main` から取り出して走るので、スクリプトの変更は、本タスクの PR がマージされた後に判定するブランチから効く。
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
-   **有効かの判定:** `review-records.sh fable` の結論。記録が有効か、merge の後の確認を求めるか、Fable の部分レビューを求めるか（`fable-review.md`「The validity decision」。出力の形は設計で決める）。
-   **チェックポイントの印（印）:** `/runplan` のチェックポイントの入口が、記録が有効なときに残す空の commit の行 `Fable review: checkpoint`（`fable-review.md:244-248`）。本書の決定で、タスク文書の PR でも Part B step 2 の前に残す。
-   **中身のない commit:** 親が 1 つで、空の commit、`README.en.md` だけを変える commit、計画書の PR の進み具合のチェックボックスの行だけを変える commit、その 2 つだけを変える commit のどれかである（`fable-review.md:114-121`）。Fable の記録の有効かの判定で使う。
-   **中身のある commit:** 中身のない commit でない commit。merge commit は、いつも中身のある commit である。
-   **CHECKS の直し・タスクの PR・push の直前:** 3.1 の定義による。
-   **merge の後の確認:** 記録の後の merge commit について、Opus のサブエージェントが、merge で手で解いた箇所と、merge で `main` から入った ADR・先行のタスク文書・コマンドの定義の変更を、PR の文書やコードが参照している箇所だけを確かめるレビュー（F-005）。
-   **merge の後の確認の記録:** merge commit を名指しした merge の後の確認の行（`not run` を含む）。Fable の記録ではなく、最後の Fable の記録を変えない。
-   **確認できる merge commit:** 親がちょうど 2 つで、第 1 親がブランチの commit か merge-base であり、第 2 親が `origin/main` から届く commit である merge commit。merge の後の確認が範囲を正しく求められる merge commit である。ほかの merge commit（親が 3 つ以上のもの、`main` の側で作ったものなど）を、確認できない merge commit と呼ぶ。
-   **未確認の merge commit:** `origin/main..HEAD` の確認できる merge commit のうち、最後の Fable の記録が見た範囲の終わり（END）より後にあり、それを名指しした merge の後の確認の記録がまだないもの。Fable の起動の後、記録の commit の前にできた merge commit も含む（AC-15）。
-   **単独の経路:** ローカルの CLI レビューと並列にせず、Fable の部分レビューだけを行う流れ（`fable-review.md`「The standalone path」）。
-   **merge-base:** `git merge-base origin/main HEAD`。
-   ほかの用語は `docs/translation_glossary.md` と統一する。

## 変更履歴

- 2026-11-19 初版（draft）
- 2026-11-19 承認（isseis、#1041）
- 2026-11-20 draft への戻し：merge の後の確認で確かめられない merge commit は、Fable が PR の全体を見る（#1049）
- 2026-11-20 承認（isseis、#1049）
- 2026-11-20 draft への戻し：merge の後の確認の指摘の場所は、Fable の規則（merge-base から HEAD までに変わったファイル）に加えて、範囲の一覧のパスも有効とする（#1060）
- 2026-11-20 承認（isseis、#1060）
