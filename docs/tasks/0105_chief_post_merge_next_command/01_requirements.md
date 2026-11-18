# 要件定義書：chief は `main` にいる worktree をタスクのない workspace と読まず、マージを見た PR からタスクを特定する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-18 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、chief が、ブランチ `main` をチェックアウトした worktree の workspace を「タスクのない workspace」と読んで `/mkreq` を示す誤りを直す要件を定義する。あわせて、chief がその workspace で開いているのを見て、マージを見た PR から、その worktree のタスクを特定する要件を定義する。

**背景:**

2026-11-18、issue-982 の workspace で、PR #991（タスク 0102 の設計書）のマージの後、chief の `tick` の行が `task: ""`・文書 `requirements`・進み具合 `awaiting_creation`・次のコマンド `/mkreq` になった。issue の筆者の期待は `/mkplan 0102` だった。issue-982 の worktree の reflog と `main` の履歴から、次のことが分かった。

1.  #991 は、要件定義書と設計書の承認を記録しないままマージされた。`origin/main` の 2 つの文書は `draft` のままで、後の #997（790652e3）が承認を記録した。そのコミットメッセージにも「PR #991 merged without recording one」とある。
2.  マージの後始末（`mergepr cleanup`）は、worktree の今のブランチが `main` でなく、`main` がほかの worktree でチェックアウトされていなければ、`main` に切り替えて `origin/main` へ fast-forward する（`internal/mergepr/mergepr.go:472-491`）。reflog では、20:25 に `isseis/0102-chief-text-table-02` から `main` へ移っている。この後、`/runpr` の手順 5 は、次のブランチを作るか `origin/main` で detached にする（`.claude/commands/runpr.md:1346-1353`）。ところが worktree は、20:46 に別のセッションが `isseis/0102-chief-text-table-03` に切り替えるまで、`main` のままだった。
3.  chief は、タスクをチェックアウトされたブランチの名前だけから決める（`internal/chief/task.go:789-823` の `Read`）。`main` は機能ブランチの名前の形をとらないので、タスクのない workspace と読まれる。`decideNext` は、ブランチがあってタスクがなければ `/mkreq` を返す（`internal/chief/state.go:308-314`）。行の文書は、`/mkreq` を提案する行では要件定義書になる（`internal/chief/position.go:250-253`）。これが issue の行の値である。
4.  一方、マージの後の自動送信は、chief が開いているのを見た PR の観測の記録（`internal/chief/records.go:1148-1162` の `Observation`。PR を見たときのタスクとブランチを持つ）から、タスク 0102 を知っていた。送らなかった理由は 2 つあり、どちらも今の規則のとおりである。1 つ目は、`main` が 0102 の機能ブランチでないことである（`internal/chief/autosend.go:212-237` の規則）。2 つ目は、設計書が approved でないことで、当時の `origin/main` の内容を正しく読んだ結果である。

issue が挙げた未確認のことの答えは次のとおりである。

-   タスクの特定はブランチの名前だけによる。`main` にいるときは、タスクのない workspace として扱う（上の 3）。
-   approved の読み取りは、マージの後の `origin/main` を見ている（`internal/chief/task.go:847-890` の `readDocuments` は `origin/main` から読む）。当時の設計書は `draft` だったので、`/mkplan` が出ないことは正しい。

タスク 0088 の設計は、ブランチのない状態（detached HEAD）を「要確認」とし、タスクがないものとはしないと決めていた（[0088 設計書](../0088_chief_of_staff/02_architecture.md) の 1.3.1「実装の際に決めたこと（2026-11-14）」、`02_architecture.md:64`）。`/runpr` が detached にした worktree には、続けるべきタスクがありうるためである。チェックアウトされた `main` も同じく、マージの後に `mergepr` が置く場所で、続けるべきタスクがありうる。ところが、この規則から漏れていた。

対応 issue: [#996](https://github.com/isseis/yt2column/issues/996)（関連: [#982](https://github.com/isseis/yt2column/issues/982)（タスク 0102。事象の起きた workspace）、[#839](https://github.com/isseis/yt2column/issues/839)（タスク 0092。chief が作った workspace の記録））

本書は、`main` にいる worktree の行のタスク、文書、次のコマンド、送信の振る舞いを決める。観測の記録をセッションの間どう持ち続けるか、理由の文言、送信の直前の確かめ方、タスクの文書のディレクトリの見つけ方は `02_architecture.md` が決める（[design_handoff.md](design_handoff.md)）。

### issue #996 で決めたこと

-   **マージの後に workspace が `main` にいても、そのタスクを特定できるようにする。** マージの後に `mergepr` が worktree を `main` に置くので、そのたびにタスクが分からなくなるためである。
    -   ADR：[ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する（要件定義の際に決めたことの ADR の項を参照）。
-   **設計書が `origin/main` で approved なら、次のコマンドに `/mkplan <task>` を提案する。** 機能ブランチにいるときと同じ次のコマンドを示すためである。設計書が approved でなければ、今の規則のとおり、次のコマンドは決まらない（F-001）。
    -   ADR：なし。次のコマンドの規則（0088 設計書の 3.1.4）を変えないためである。

### 要件定義の際に決めたこと（2026-11-18）

-   **進め方は通常の進め方（`full`）とする。この変更は、簡易な進め方をとれない変更（除外される変更）に当たらない。** chief がタスクを決める情報源に観測の記録を足す新しい仕組みで、ADR-0046 の記録の使い道の決定を変えるためである。除外の判断：`docs/dev/security.md` の規則と前提を変えない。`fixprguard`・`codexreview`・`mergepr` を変えず、レビューのスレッドや Bot への依頼の扱いにも触れないので、開発の道具の信頼境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）を変えない。外部依存も足さない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。人の答え：通常の進め方。
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   **`main` にいる worktree は、chief の同じセッションがマージを見た PR の記録からタスクを特定する。そのような記録がなければ、detached HEAD と同じく「要確認」とし、`/mkreq` を示さない（F-001・F-002）。** 記録から特定できれば issue の期待を満たし、特定できなくても、続けるべきタスクがありうる worktree に `/mkreq` を示さないためである。記録がないときに今のとおり `/mkreq` を示す案と、記録を使わずに要確認にするだけの案は採らなかった。人の答え：記録から特定し、なければ要確認。
    -   ADR：[ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する。chief がタスクを決める情報源と、記録の使い道を変えるので、主要な決定に当たる（CLAUDE.md「Design decisions (ADR)」）。
-   **記録から特定するのは、マージを見た PR だけとする。開いているのしか見ていない PR の記録では特定しない（AC-07）。** 開いている PR のタスクの `origin/main` の文書は、その PR を作ったコマンドを次のコマンドとして示しうるので、作成済みの文書のコマンドを提案するおそれがあるためである。開いている PR の記録も使う案は採らなかった。人の答え：マージを見た PR だけ。
    -   ADR：ADR-0046 の同じ更新に含める。
-   **この扱いは、チェックアウトされたブランチの名前がちょうど `main` の worktree だけに当てる。detached HEAD の worktree と、`main` でも機能ブランチでもないブランチの worktree の扱いは変えない（F-003）。** `mergepr` がマージの後に worktree を置くのは `main` であり、範囲を必要な所に限るためである。detached HEAD にも記録からの特定を当てる案は、0088 の決定を変え、完了したタスクの行や削除の提案の扱いにも及ぶので採らなかった。人の答え：`main` だけ。
    -   ADR：ADR-0046 の同じ更新に含める。
-   **`main` にいる worktree へは、記録からタスクを特定できても、マージの後の自動送信をしない。次のコマンドは提案として示し、人が選べば送る（F-001・F-003）。** 自動送信の条件を広げずに、issue の期待（提案する）を満たすためである（AC-10）。人の答え：送らず提案だけ。
    -   ADR：ADR-0046 の同じ更新に含める。
-   **記録から特定したタスクの次のコマンドは、人が選べば chief が送る（AC-04）。** 提案を chief から送れるようにして、issue の期待（提案する）を満たすためである。ただし、記録は自動送信には使わない。行には特定したタスクを示し、人はそれを見てから選ぶ。ADR-0046 は「記録は、送るコマンドとタスクを決めるためには使わない」と決めている（`docs/dev/adr/0046-chief-of-staff.md:38`）ので、本タスクでこの決定に例外を足す。表示だけにして送れなくする案は、提案を chief から送れなくなるので採らなかった。人の答え：人が選べば送れる。
    -   ADR：ADR-0046 を更新する（「決定」の記録の項に例外を足す）。
-   **記録が示すタスクが 2 つ以上に食い違えば、タスクを特定せず「要確認」とする（AC-06）。** 1 つの worktree を別のタスクに使い回したときに、推し量ってどれかを選ばないためである（CLAUDE.md「Reject, don't normalize」）。最後に見た PR のタスクを採る案は採らなかった。人の答え：食い違えば要確認。
    -   ADR：ADR-0046 の同じ更新に含める。
-   **記録のない `main` の workspace は、detached HEAD の worktree と同じく要確認として表のほかの行の後に並べ、chief から `/mkreq` を送れなくする。人がその pane で `/mkreq` を打つ（AC-07・AC-08）。** detached HEAD の worktree と同じ扱いにするためである。人の答え：それでよい（detached と同じく要確認にして表の最後に並べ、`/mkreq` は人が pane で打つ）。
    -   ADR：ADR-0046 の「結果」に書く。
-   **記録から特定したタスクが完了していれば、行はそのタスクの「タスク完了」とする。削除の提案の規則は変えない（AC-03）。** 機能ブランチにいるときと同じ規則を当てるためである。削除の提案は、今の規則のとおり、HEAD が detached でないことを理由に止まったまま示される。完了したタスクでも要確認にする案は採らなかった。人の答え：タスク完了、削除は今のまま。
    -   ADR：なし。ADR-0046 の削除の決定（HEAD が `origin/main` から届くことなどの確かめ）を変えないためである。
-   **文書が `draft` のままマージされたときの行の理由は、今の理由（「設計書が approved でない」など）のままとし、文言を足さない（AC-02）。** 今の理由は正しく、範囲を広げないためである。人の答え：今の理由のまま。
    -   ADR：なし。理由の文言は本タスクの中で閉じる。
-   **記録からの特定は、chief のセッションの間だけ行う。chief のセッションとは、chief の pane の Claude Code のセッションで、chief の記録はこれごとに分かれる（`internal/chief/records.go:177-182` の `Owner`）。chief の pane で `/clear` をするか Claude Code を起動し直すとセッションが変わり、それより前に見たマージでは特定しない。同じセッションで `/chief` を打ち直しても、特定は続く（AC-07）。** 新しい永続的な記録を作らず、分からないときは安全な側に倒すためである。セッションを越えて保つ案は、セッションに属さない記録と、それを消す規則が要るので採らなかった。人の答え：同じセッションの間（区切りは chief の pane のセッション）。
    -   ADR：ADR-0046 の同じ更新に含め、「結果」に書く。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   マージの後に `main` にいる worktree の行に、`/mkreq` を誤って示さない。
-   chief がマージを見た PR の記録から、`main` にいる worktree のタスクを特定し、機能ブランチにいるときと同じ規則で文書の位置と次のコマンドを示す。

### 2.2. スコープ (In Scope)

-   チェックアウトされたブランチが `main` の worktree の行の、タスク・文書の位置・状態・次のコマンドの決め方（F-001・F-002）。
-   その行の次のコマンドを人が選んで送ること（AC-04・AC-05）と、`/mkreq` の送信を拒むこと（AC-08）。
-   特定に使う観測の記録の受け付ける形（4.2）。
-   変えない振る舞いの確認（F-003）。
-   `docs/dev/guides/chief_guide.md` と ADR-0046 の更新（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   **#991 が承認を記録しないままマージされたこと、`/runpr` の手順 5 の後も worktree が `main` に残ったことの原因。** 本 issue は chief の判断を対象にしている。`/runpr` を実行したエージェントの記録は、この worktree の Claude Code の transcript になく、原因はまだ確かめていない。本タスクを入れても、#991 と同じ事象（承認を記録しないままのマージ）では `/mkplan` は示されない（AC-02）。別の issue にするかは、本書のレビューで決める。
-   **`mergepr cleanup` が worktree を `main` へ切り替える振る舞い。** 変えない。`mergepr` を変えると、開発の道具の信頼境界（ADR-0015）にかかわる除外される変更になる。
-   **detached HEAD の worktree のタスクの特定。** 0088 の決定（要確認）のままとする（要件定義の際に決めたこと）。
-   **`main` にいる worktree への自動送信。** 送らない（要件定義の際に決めたこと）。
-   **chief のセッションを越えた特定。** 行わない（要件定義の際に決めたこと）。
-   **文書が `draft` のままマージされたことを示す文言。** 足さない（要件定義の際に決めたこと）。
-   **削除の提案の規則。** 変えない（要件定義の際に決めたこと）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

本章では、次の語を使う。

-   **`main` にいる worktree**：HEAD がブランチを指し、そのブランチの名前がバイト列でちょうど `main` である worktree。
-   **chief のセッション**：要件定義の際に決めたことのとおり、chief の pane の Claude Code のセッション。
-   **マージを見た PR**：今の chief のセッションの `tick` が、その worktree で開いているのを見て、その後の `tick` で `MERGED` になったのを見た PR。「見た」の条件は、今の自動送信がマージを見たとする条件と同じである。ただし、自動送信が観測を次のラウンドへ引き継ぐのをやめる期限（マージを見てから 10 分。`internal/chief/autosend.go:22` の `carryWindow`）は当てない。一度見たマージの記録は、セッションが続く間、有効とする（H-01）。一方、ラウンドが 15 分以上あくと（`observeWindow`）、後のラウンドは前のラウンドの観測を読まないので、その間にあったマージは見たことにならない（`internal/chief/tick.go:49-70`、`internal/chief/autosend.go:21`）。
-   **記録が示すタスク**：マージを見た PR について、PR が開いていたときに chief が記録したタスク（`internal/chief/tick.go:56`）。

#### F-001: `main` にいる worktree のタスクを、マージを見た PR から特定する

`main` にいる worktree について、マージを見た PR が 1 つ以上あり、それらの記録が示すタスクがすべて同じタスク T のとき、chief はその worktree のタスクを T とする。行の状態・文書の位置・次のコマンドは、T の機能ブランチにいるときと同じ規則で、`origin/main` の T の文書から決める。

**Acceptance Criteria**:

-   **AC-01**: `main` にいる worktree で、マージを見た PR の記録が示すタスクが T だけで、`origin/main` の T の設計書が approved で計画書がないとき、`tick` の行は次のようになる。
    -   タスクは T。
    -   文書の位置は計画書（`plan`）、進み具合は作成待ち（`awaiting_creation`）。
    -   次のコマンドは `/mkplan T`。
-   **AC-02**: AC-01 と同じ条件で、`origin/main` の T の設計書が `draft` のとき、行のタスクは T で、状態は要確認になる。理由は、機能ブランチにいるときと同じ「設計書が approved でない」である。次のコマンドは示さず、`/mkreq` も示さない（issue #996 の事象の条件）。
-   **AC-03**: AC-01 と同じ条件で、`origin/main` の T の計画書のすべての PR がマージ済みのとき、行のタスクは T で、状態はタスク完了になる。T の workspace を chief が作っていれば、削除の提案は、HEAD が detached でないことで止まったものとして示す。
-   **AC-04**: AC-01 の行で、人が次のコマンドを選んで送らせると、chief は `/mkplan T` をその workspace の送り先の pane へ送る。送る前の確かめ（送り先の pane、モデル、送信の記録）は、機能ブランチにいる行へ送るときと同じものを経る。
-   **AC-05**: AC-04 で、人が選んだ後、送る直前に、その行のタスクか次のコマンドが人に示したものと変わっていれば、chief は何も送らずに拒み、行が変わったことを理由に示す。変わる場合は、たとえば次のどれかである。
    -   chief のセッションが変わった。
    -   別のタスクの記録が増えた。
    -   記録を読めなくなった。
-   **AC-06**: `main` にいる worktree で、マージを見た PR が 2 つ以上あり、記録が示すタスクが 2 つ以上に食い違うとき、chief はタスクを特定しない。行は要確認になり、理由に記録の示すタスクが食い違うことを示す。次のコマンドは示さず、`/mkreq` も示さない。

#### F-002: タスクを特定できない `main` の worktree を要確認とする

`main` にいる worktree で、F-001 でタスクを特定できないとき、chief はその worktree をタスクのない workspace とせず、要確認とする。

**Acceptance Criteria**:

-   **AC-07**: `main` にいる worktree で、マージを見た PR がないとき、行は要確認になる。理由には、worktree が `main` にいてタスクを特定できないことを示し、次のコマンドは示さない。その行は、detached HEAD だけによる要確認の行と同じく、使っていないか残った worktree の行として、表のほかの行の後に並ぶ。マージを見た PR がないのは、たとえば次の場合である。
    -   今のセッションが、その worktree で開いている PR を見ていない。chief のセッションが変わった後と、`tick` を走らせずに `snapshot` だけを走らせたとき（`/chief once`）を含む。
    -   見た PR がまだ開いている。
    -   見た PR がマージされずに閉じられた。
    -   開いているのを見た後、15 分以上ラウンドがあかずにマージを見られなかった（3.1 の定義）。
-   **AC-08**: `main` にいる worktree へ `/mkreq` を送らせると、F-001 でタスクを特定したかによらず、chief は送らずに拒み、タスクのない workspace でないことを理由に示す。
-   **AC-09**: そのラウンドの特定に使う観測の記録に読めないものが 1 つでもあるとき、`main` にいる worktree の行はタスクを特定せず、要確認になる。読めない記録には、4.2 の形に合わない記録を含む。読めない記録はどの worktree のものか分からないので、ほかの worktree の記録でも同じである。理由には記録を読めないことを示す。次のコマンドは示さず、`/mkreq` も示さない。例：記録の `task` が `"../x"` や `"0102 "` のとき。

#### F-003: ほかの worktree の扱いと自動送信を変えない

**Acceptance Criteria**:

-   **AC-10**: `main` にいる worktree へは、F-001 でタスクを特定できても、マージの後の自動送信をしない。その PR の自動送信の結果には、今と同じく、ブランチがそのタスクの機能ブランチでないことを理由に示す（`internal/chief/autosend.go:212-237`）。
-   **AC-11**: 次の 3 つの worktree の行の、タスク・状態・文書の位置・次のコマンド・削除の提案の決め方は変えない。どれにも、マージを見た PR の記録は使わない。
    -   機能ブランチにいる worktree：タスクはブランチの名前から決める（`internal/chief/task.go:789-823`）。
    -   detached HEAD の worktree：今の扱い（要確認、または chief が作った完了したタスクの削除の提案の行。`internal/chief/state.go:290-302`、`internal/chief/removal.go:395-399`）のまま。
    -   `main` でも機能ブランチでもないブランチにいる worktree：タスクのない workspace として `/mkreq` を示す（`internal/chief/state.go:308-314`）。

#### F-004: ガイドと ADR

**Acceptance Criteria**:

-   **AC-12**: `docs/dev/guides/chief_guide.md` の進行の位置の説明（今の `chief_guide.md:82`）に、次の 4 点を書く。
    -   `main` にいる worktree のタスクは、chief の同じセッションがマージを見た PR から特定される。
    -   特定できなければ要確認になる。
    -   chief の pane のセッションが変わるか、`snapshot` だけを走らせたときは、特定されない。
    -   そこへは自動送信せず、`/mkreq` も送れない。
-   **AC-13**: ADR-0046 の「決定」・「採らなかった案」・「結果」・「経緯」が、本書の要件定義の際に決めたことと合っている（本 PR で更新済み）。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   新しい GitHub や git のネットワークの呼び出しを足さない。chief の記録と `origin/main` の文書だけを読む。

### 4.2. セキュリティ (Security)

-   条件付きのガイド（`docs/dev/security.md`）の発動条件（外部コマンド `yt-dlp`、API キー、キャッシュ、LLM API、信頼できない文字列のプロンプトや記事への埋め込み）には当たらない：N/A。
-   **信頼できない入力の境界：観測の記録。** 記録はリポジトリの外の状態のディレクトリにあり、同じ利用者のほかのプロセスが書ける（ADR-0046 の「結果」）。今は、記録のタスクを自動送信の照合（文字列の比較）にしか使っていない。本タスクではこれを、`origin/main` から読む文書と送るコマンドのタスクの源にする。そのため、特定に使う記録に次の形を求める。
    -   使う項目は、worktree・PR の番号・タスクと、マージを見たことである。ほかの項目は今の読み取りの規則のままとし、知らない項目を持つ記録は今と同じく読めない記録とする（`internal/chief/records.go:159-175`）。
    -   タスクは、4 桁の数字のタスク番号の形だけを受け付ける。前後の空白、区切り、ほかの文字を含むものは、直さずに拒む（AC-09）。
    -   不正な UTF-8 の列、対のないサロゲート、同じ名前の項目の繰り返しを含む記録は、標準ライブラリなら置き換えたり後の項目を採ったりして通すが、直さずに拒む。
    -   1 つの記録の大きさの上限は、今の記録の上限（`internal/chief/records.go:103` の `recordLimit`、512 KiB）とする。
    -   これらの形の求めは、`main` にいる worktree の特定に使う読み取りに当てる。今の自動送信の読み取りにも当てるかは設計で決める（H-06）。
    -   拒んだ記録は、記録の誤りの sentinel（`internal/chief/records.go:32` の `errRecord`）と `errors.Is` で判定できる誤りとして扱う。そのラウンドの `main` の行は AC-09 のとおり要確認とし、読めた記録だけで特定しない。
-   偽った記録が `main` にいる worktree に別のタスクを示すことは、残る危険として受け入れる。そのとき、行はそのタスクの次のコマンドを示しうるが、自動送信はせず（AC-10）、人が行のタスクを見て選んだときだけ送る（AC-04）。送られるのも、そのタスクの `origin/main` の文書が示す次のコマンドだけである。
-   chief が送りうる入力の範囲（ADR-0046）は変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   分からないときは要確認とし、次のコマンドを推し量らない（AC-06・AC-07・AC-09）。

### 4.4. 互換性 (Compatibility)

-   chief の出力の形（`snapshot`・`tick` の JSON）の項目は変えない。値の組み合わせだけが変わる。プロトコル番号を上げるかは設計で決める（[design_handoff.md](design_handoff.md) H-04）。

### 4.5. 保守性 (Maintainability)

-   ブランチをチェックアウトした worktree のタスクの特定の規則は、ブランチの名前からの特定と、マージを見た PR からの特定の 2 つに限る。これとは別に、chief が作った workspace の記録から削除の提案の行のタスクを探す規則（ADR-0046）は、変えない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR-0046 の決定に従う。本書で変える決定は、要件定義の際に決めたことと同じ PR の ADR-0046 の更新に書く。
-   CLAUDE.md の「Declare, don't infer」：ブランチの名前は git が返す値で、chief は今もそれを機能ブランチの名前の形で読んでタスクを決めている（`internal/chief/task.go:741-751` の `branchTask`）。本タスクは、その読み方に、バイト列で `main` と等しいかの比較を 1 つ足すだけで、名前の中身からほかの振る舞いを推し量らない。`mergepr` も、今のブランチをベースのブランチの名前と比べている（`internal/mergepr/mergepr.go:477`）。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **`main` にいる worktree**・**chief のセッション**・**マージを見た PR**・**記録が示すタスク**：3.1 の冒頭で定義する。
-   **要確認**：chief の行の状態の 1 つ（`needs_check`）。
-   **worktree と workspace**：条件は worktree（git の作業ツリー）について述べ、表の行は workspace（Herdr の workspace）ごとに示す。chief のタスクの workspace は worktree を 1 つ持つ。
