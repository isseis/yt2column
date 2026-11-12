# 実装計画書：/runpr のマージの質問を、画面に収まる 1 つの問いにする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-12 |
| Review date | 2026-11-12 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。用語（マージの質問・deferred のスレッド・deferred の問い・除外スレッド・追跡 issue・詳細ファイル・詳細ディレクトリ・要約・問いの回・承認のメッセージのファイル・番号の答え・NEXT・PRE_MERGE・LAST_REVIEWED・Escalation・LIMITS_NOTE・旧値）は、要件書 6 章と設計書の冒頭と同じ意味で使う。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`runpr.md` の step 4 を組み替え、deferred のスレッドをマージの質問より前の deferred の問いで片付け、マージの質問を要約と詳細ファイルと 1 つの単一選択の問いにする。あわせて、`mergepr.md`・`fixpr.md`・`_lib/simple-process.md`・`_lib/task-pr-handoff.md`・`CLAUDE.md`・`requirements_process.md` の記述と ADR の食い違いの記述を、この組み替えに合わせて直す（要件書 2.1 節）。何をどこに書くかは設計書 3 章が定める。本計画は、作業の順序と確かめ方を定める。

### 1.2. 実装の原則

-   **文面は設計書 3 章に従い、英語で書く。** コマンドの定義と規約は英語の文書である（要件書 5 章）。要約・選択肢・詳細ファイルの文を日本語で出すことは、英語の指示として書く（設計書 3 章の冒頭）。各ステップは、書く内容を設計書の節で指す。文面は実装の PR で書く。
-   **step の番号と見出しは、step 5 の見出しのほかは変えない**（設計書 1.1）。step 4 の中の項目を繰り下げて、deferred の問いを項目 1 に置く。
-   **今の手順は移し、写さない**（設計書 1.1）。追跡 issue に移す手順は、今はマージの答えを受けた後にあるが、その置き場所を項目 1 に変える。戻す手順は、項目 1 と項目 3 の両方から呼ぶ。AC-18 の 4 点のほかは、答えを受けた手順の文を変えない。
-   **新しい道具とテストは足さない。** テストの変更は定数 `runprStep5` の 1 つだけである（設計書 7.1）。
-   **ADR の食い違いの記述を消すか書き直すことは、編集上の訂正である**（`docs/dev/conventions/adr.md`「Editorial corrections」の 2 つ目の種類）。コミットのメッセージに訂正の行を書き、PR の説明に訂正の段落を書く（同「Recording」。形は `requirements_process.md`「Recording in the PR description」）。

### 1.3. 計画の際に決めたこと（2026-11-12）

設計書は、PR を 1 本にすること（8 章）、作業の順序（8 章）、確かめ方（7 章）を決めている。人に尋ねて決める点は残っていなかった。計画の中で次を決めた。

1.  **フェーズを 1 つにし、PR を 1 本にする。** 理由：設計書 8 章が、コマンドの定義・説明の文書・テストの定数が互いに参照し合うので 1 本の PR で行うと決めている。step 5 の見出しと定数 `runprStep5` は同じコミットで変えないと `make test` が落ちる。ADR：なし（PR の分け方で、タスクの中で閉じる）。
2.  **AC-18 と AC-19 は、記録した差分の読み取りで確かめる。比べるパスは、AC-18 では `.claude/commands/runpr.md`、AC-19 では `.claude/commands/mergepr.md` とする。** 理由：設計書 7.2 が決めている。AC-18 が限るのは「答えを受けた手順」の変更の種類で、変わった箇所が 4 点のどれに当たるかは文の意味で決まる（設計書 3.5）。この分類は読んで行う。`runpr.md` の差分は step 4 の組み替えで数百行になるが、PR のレビューで必ず読む差分であり、全体を読める大きさと判断した。レビューで差分が大きいと異議が出たときは、`requirements_process.md` §4「Objection」に従い、計画の決定の変更として、AC-18 の確かめを `_checks/` のシェルスクリプトと `manual` の読み取りに替える。スクリプトが確かめるのは、変えないと定めた手順（PR レビューを依頼する答え、承認なしでマージする答え、マージしない答え、戻す手順 2〜6、PRE_MERGE の記録とマージ）の文が、設計書 3.2・3.5・3.7 が挙げる変更のほかは変わっていないことである。残りの変更を設計書 3.5 の 4 点に分類することは、読んで行う。どちらの AC も変わるファイルを限らないので、ファイルの一覧は記録しない。ADR：なし（確かめ方の選択で、タスクの中で閉じる）。
3.  **AC-22 の手動の確かめは、`/runplan` が、見本の値と、OpenCode のエージェントに表示を出させる指示を用意し、人が OpenCode で行う。結果は §7 に記録してから PR を作る。** 理由：`/runplan` のセッションは OpenCode の画面を見られない。設計書 1.2 は見本の値で確かめると決めているが、誰がいつ行うかは決めていない。PR を作る前に確かめれば、画面に収まらなかったときに、PR を作る前に直し方を人と決められる。ADR：なし（手動の確かめの手順で、タスクの中で閉じる）。
4.  **`requirements_process.md` の「Returning to the full process」の冒頭（`:572-575`）も書き換え、設計書 2.2・3.9 に加える編集上の訂正を、本計画書を追加する PR でする。** この文は、3 文書の PR を通常の進め方へ戻せる入口を、マージの質問の答えと `/runpr` への依頼だけに限っており、deferred の問いからも戻せること（AC-05a、ADR-0040）と食い違う。設計書の変える箇所の一覧から漏れていた。人に尋ね、設計書の訂正として加える案が選ばれた。理由：要件書と ADR-0040 はすでに deferred の問いからも戻せると決めており、設計書の決定は変わらない。採らなかった案：設計書を `draft` に戻す決定の変更として扱う案、別の issue と PR で直す案。ADR：なし（ADR-0040 の決定のとおりに規約を合わせるだけである）。設計書の訂正は、本計画書を追加する PR のコミットのメッセージに訂正の行を書き、PR の説明に訂正の段落を書く（`requirements_process.md`「Recording in the PR description」）。

### 1.4. 既存コード調査結果

コミット `5a3afa134bffd31bf54e05172f1b71590c9d7225`（`origin/main`）で確かめた。

-   **設計書の行番号は今も正しい。** 設計書はコミット `41b2d31e` の行番号を使う。`runpr.md`・`mergepr.md`・`fixpr.md`・`_lib/simple-process.md`・`_lib/task-pr-handoff.md`・`requirements_process.md`・`cmd/` は、`41b2d31e` から上記のコミットまで変わっていない（`git diff --stat 41b2d31e HEAD -- <それらのパス>` の出力は空）。`CLAUDE.md` は変わっており、`/runpr` の行は `CLAUDE.md:207` のまま、その下の流れの説明は `CLAUDE.md:225-230` にある。
-   **AC-20 の対象の文の一覧。** `git grep -n -i "chooses to run\|run NEXT\|runs only when\|run it in step 4" -- ':!docs/tasks' ':!docs/research'` の出力のうち本タスクに関わるものは、`_lib/task-pr-handoff.md:31`、`runpr.md:2`・`:50`・`:91`・`:786`・`:1039`・`:1048`、`CLAUDE.md:207`・`:229` で、どれも設計書 2.2・3.8 の対象に含まれる。ほかの出力（`.pre-commit-config.yaml:11`、`CLAUDE.md:825`、`internal/fixprguard/gitstate.go:933`）は別の事柄である。
-   **戻す入口を述べる文の一覧。** `git grep -n -i "return it to the full process\|returns the PR to the full process\|merge question" 5a3afa13 -- .claude CLAUDE.md docs/dev ':!docs/dev/adr'` の出力（13 行）のうち、戻せるのをマージの質問だけに限る文は `docs/dev/conventions/requirements_process.md:574`（`:572-575` の段落）だけである（§1.3 の決定 4）。`requirements_process.md:108-109` はマージの質問でも戻せると述べるだけで、限らないので変えない。ほかの行は承認、除外スレッド、戻した後の振る舞いを述べ、変更の後も正しい。`_lib/simple-process.md:364-367` は設計書 3.7 が対象にしている（この grep の語では出ない）。
-   **AC-21 の対象の文の一覧。** `git grep -n "runpr.md\` step 4\|step 4's question"` を `.claude`・`CLAUDE.md`・`docs/dev`（ADR を除く）にかけた出力のうち、deferred のスレッドを step 4 で扱うと述べるのは `fixpr.md:145`・`:168` だけである。`fixpr.md:157`・`:184`・`:494`・`:1192` は除外スレッドを述べ、変えない（設計書 3.9）。`_lib/pr-review-escalation.md:18`（step 4 が PR レビューの依頼を示す）、`_lib/local-cli-review.md:17`・`_lib/triage-pattern.md:7`（戻す手順のローカルのレビュー）、`_lib/simple-process.md:24`・`:367`・`:372`（戻す手順）は、変更の後も正しい。`docs/dev/guides/fixprguard_guide.md:227`（戻す手順の `edit-pr`）と `:439`（`/runpr` が `head`・`list`・`reply` を使う）も、変更の後も正しい。`runpr.md` の中では、`:233`（LIMITS_NOTE）はステップ 1-3 で書き換え、`:337`（`#未記入` の行を step 4 の質問で示す）は、マージの質問で示すことが変わらないので変えない。`:563`（「step 4's PR-review question」）は、PR レビューの問いがマージの質問の選択肢になるので、ステップ 1-3 で書き換える。
-   **文書のガードのテスト。** 定数 `runprStep5` は `cmd/yt2column/codex_review_docs_test.go:84` にあり、`postingSections` の「runpr.md step 5」（`:69`）だけが使う。`runprStep1`・`runprStep3`・`runprStep4` は `cmd/yt2column/fixpr_docs_test.go:42-44` にあり、変えない。`postingSections` の「runpr.md step 4」の命令の一覧（`codex_review_docs_test.go:65-68`）は `gh issue comment <issue> --body-file`・`gh issue create `・`fixprguard reply `・`fixprguard edit-pr ` の文字列を探すので、移した手順でもこの書き方を保つ。`TestPostingPathsCheckMention`（`:119`）は節の最初の `**Mention check.**` の文が `grep -qiF` の確認を行うことを、`TestPostingCommandsInventory`（`:200`）は最初の `**Mention check.**` が投稿の命令より前にあることを確かめる（設計書 3.6）。step 4 に足す `fixprguard list` の呼び出しは、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（`:212`）が道具の引数の解析器で読む。
-   **詳細ディレクトリの作り方。** 開発環境（macOS の `mktemp`）で `mktemp -d /tmp/runpr.XXXXXX` を実行し、モード `drwx------` の、17 文字のパス（例 `/tmp/runpr.tBnXHT`）のディレクトリができることを確かめた（2026-11-12）。
-   **スクラッチパッドを使う箇所。** `runpr.md` で「scratchpad」と書く箇所は `:176`（step 1 の PR の作成）、`:875`（戻す手順 5）、`:965`（PRE_MERGE の写し）の 3 つである。設計書 3.2 が写し先を変えるのは `:965` だけで、ほかの 2 つは変えない。`:965` の写しはマージに使うメッセージの元なので、詳細ファイルと同じ場所に置く。`:176` と `:875` は PR のタイトルと本文を書く場所で、AC-18 の限定（戻す手順は変えない）により、本タスクの範囲の外である。
-   製品のコード（`cmd/` の本体・`internal/`・`extension/`）は変えない。`README.md`・`README.en.md` は `/runpr` を述べないので変えない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: マージの質問の組み替えと、説明の文書（設計書 8 章の 1〜5）

**変更するファイル:** `.claude/commands/runpr.md`、`.claude/commands/mergepr.md`、`.claude/commands/fixpr.md`、`.claude/commands/_lib/simple-process.md`、`.claude/commands/_lib/task-pr-handoff.md`、`CLAUDE.md`、`docs/dev/conventions/requirements_process.md`、`cmd/yt2column/codex_review_docs_test.go`、`docs/dev/adr/0047-runpr-merge-question.md`、`docs/dev/adr/0046-chief-of-staff.md`、本計画

-   [x] **ステップ 1-1**: `runpr.md` step 4 に項目 1（deferred の問い）を足し、今の項目 1〜4 を 2〜5 に繰り下げる（AC-01〜AC-05、AC-21。設計書 3.1・3.2・3.6）。
    -   尋ねる時点、対象、メンションの一覧を読み直すこと、問いの形と選択肢、要約、詳細ファイル、番号の答えの形、答えごとの動きを、設計書 3.1 のとおりに書く。番号の答えの形は、ここで 1 回だけ定める。
    -   詳細ディレクトリの作り方、詳細ファイルの名前、書けないときに止まること、読み戻さないことを、設計書 3.2 のとおりに書く。step 4 で最初の問いを尋ねる前に 1 回作り、その実行の間使う。
    -   今の「When the answer merges, first move the deferred threads not chosen to an issue」の手順（`runpr.md:929-961`）を項目 1 へ移し、設計書 3.6 の 4 点（準備の破棄をしない、追跡 issue を覚えておく、数え直す、止まったときの報告）だけを変える。数え直しの `fixprguard list` の呼び出しは、ステップ 1-2 の戻す手順の呼び出しと同じ形で書く。最初の投稿の命令より前に、その手順の `**Mention check.**` の段落を置く。投稿の命令の書き方（`gh issue comment <issue> --body-file`、`gh issue create `、`fixprguard reply `）は変えない。
-   [x] **ステップ 1-2**: `runpr.md` step 4 の項目 2〜5 と、答えを受けた手順を書き換える（AC-06〜AC-18。設計書 3.2〜3.5・3.7）。
    -   項目 2：承認の行のない版の本文を、`mergepr prepare` の準備のディレクトリ（詳細ディレクトリではない）にファイルとして書く（設計書 3.2）。
    -   項目 3：詳細ファイル（設計書 3.2 の項目のすべて）、要約の 12 種類の行（設計書 3.3）、question の文と選択肢と自由記述の扱い（設計書 3.4）を書く。今のメッセージの段落（`runpr.md:729-766`）と、3 つの問いの段落（`:766-806`）を置き換える。
    -   答えを受けた手順：deferred のスレッドを直す答え（`:823-828`）は、`mergepr discard` を除いて項目 1 へ移す（設計書 3.5 の 1 点目）。NEXT を実行する選択肢（`:786-787`）は除く。除外スレッドを覆す答え（`:829-833`）は自由記述の答えとして残す。尋ね直すたびに新しい回の詳細ファイルと要約を書くことと、step 2・3 を経て step 4 に戻るときは項目 1 から始め、新しい deferred のスレッドがあれば先に deferred の問いを尋ねることを足す（設計書 2.1・3.5 の 3 点目、6.2）。
    -   戻す手順：項目 1 から入るときは準備を破棄しないことと、戻す手順 1 で `fixprguard list` を呼んで THREADS を未解決のものに限ることを足す（設計書 3.7）。足す `fixprguard list` の呼び出しは、step 3 の呼び出し（`runpr.md:626`）と同じ形で書く。つまり、コードブロックの中の 1 行とし、引数も同じ（`--protocol 6`、`--reviewer <reviewer>`）にする。移した手順の数え直し（ステップ 1-1）の呼び出しも同じ形にする。戻す手順 7 の後は項目 1 から始め直す。手順 2〜6 の文は変えない。
    -   項目 4：PRE_MERGE のときの写し先を、スクラッチパッドから詳細ディレクトリの `approved-<回>.subject`・`approved-<回>.body` に変える（設計書 3.2）。
    -   step 4 の冒頭の「Ask the user once」の段落に、deferred のスレッドがあれば、マージの質問の前に deferred の問いを尋ねることを足す（設計書 3.5）。
    -   マージしたときの報告と止まるときの報告に、詳細ディレクトリと詳細ファイルのパスを書くことを足す（設計書 3.2「報告」）。
-   [x] **ステップ 1-3**: `runpr.md` の step 1 と step 3 の記述を合わせる（AC-08、AC-21。設計書 3.1・3.3）。
    -   step 1 の LIMITS_NOTE（`:232-234`）の「the message of step 4's question」を、step 4 の各問いの要約と詳細ファイルに出す形にする。
    -   step 2 の LAST_REVIEWED がないときの扱い（`:563` の「step 4's PR-review question」）を、マージの質問の「先に PR レビューを依頼」の選択肢の条件として述べる形にする（設計書 3.3 の 4 行目）。
    -   step 3 の `ask` の持ち越し（`:669`）を、マージの質問の「先に PR レビューを依頼」の選択肢と詳細ファイルへ持ち越す形にする。
    -   step 3 の deferred のスレッドを残す 2 か所（`:672`、`:691`）を、step 4 の deferred の問いのために残す、と書き直す。
-   [x] **ステップ 1-4**: `runpr.md` の step 5、description、導入の段落、Authorization を書き換え、テストの定数を合わせる（AC-16、AC-20、AC-21。設計書 3.8）。
    -   step 5 の見出しを `## 5. Switch branch, then show NEXT` にし、NEXT を実行する分岐（`:1039-1041`）を除く。「do not run NEXT」の句（`:1048`）も除く。最終の報告に、マージを決めた回の詳細ファイルのパスを書くことを足す。
    -   description（`:2`）、導入の段落（`:48-50`）、Authorization の NEXT の句（`:90-91`）と追跡 issue の句（`:82-83`）を、設計書 3.8 のとおりに書き直す。
    -   `cmd/yt2column/codex_review_docs_test.go:84` の `runprStep5` を新しい見出しにする。`make fmt` を実行する。
-   [x] **ステップ 1-5**: `mergepr.md` step 3 の「When this command runs inside `runpr.md`」で始まる文（`:122-126`）だけを書き直す（AC-19。設計書 3.9）。同じ step のほかの文は変えない。
-   [x] **ステップ 1-6**: ほかのコマンドの定義と文書を書き換える（AC-05、AC-05a、AC-15、AC-20、AC-21。設計書 3.8・3.9）。
    -   `fixpr.md:144-146` と `:167-168`。
    -   `_lib/simple-process.md:334-339`（除外スレッド・「fix all」・マージの質問の選択肢）、`:364-367`（戻す手順を呼ぶ箇所に項目 1 を足す）、`:380-384`（THREADS を、`fixprguard list` で未解決のものに限る）。
    -   `_lib/task-pr-handoff.md:27-32`、`CLAUDE.md:207` と `:225-230`、`requirements_process.md:89`。
    -   `requirements_process.md:572-575` に、deferred の問いの答えでも戻せることを書き足す（§1.3 の決定 4、設計書 3.9）。
-   [x] **ステップ 1-7**: ADR の食い違いの記述を直す（設計書 3.10）。直す前に、`git grep -n "0089" HEAD -- docs/dev/adr/` を実行し、直す記述を確かめる。経緯の行（ADR-0040・ADR-0046・ADR-0047 の 2026-11-12 の行）とリンクは変えない。
    -   ADR-0047 の「決定」の最後の段落（「この決定は、まだ…反映していない。…解消する。」）を消す。「決定」の冒頭の「実装の後は `runpr.md` と `mergepr.md` の step 3 が持つ」は、実装の後の状態を述べる形（`runpr.md` と `mergepr.md` の step 3 が持つ）に書き直す（同じ食い違いの記述の一部として扱う）。
    -   ADR-0046 の「決定」の食い違いの記述（`docs/dev/adr/0046-chief-of-staff.md:41`）から、0089 の実装を待つ部分（「0089 の実装の後は」「0089 の実装より前は、この答えを中継しない」）を除き、残る食い違い（0088 の実装を待つこと、AC-30・AC-46 が対象を失ったこと）を述べる形に書き直す。
    -   両方とも訂正の行をコミットのメッセージに書き、PR の説明に訂正の段落を書く（§1.2。PR の説明は `/runplan` が `_lib/task-pr-handoff.md` Part B で書く）。直した後の ADR が、それぞれの「決定」と食い違わないことを読んで確かめる。
-   [x] **ステップ 1-8**: `make test` と `make lint` を通す。ステップ 1-1〜1-7 の文書の変更によってガードのテストが落ちたときは、テストを変えずに文書の書き方を直す（設計書 7.1。テストの変更は `runprStep5` だけ）。
-   [x] **ステップ 1-9**: AC-18 と AC-19 の差分の記録を、§7 の「差分の記録」に書く（§1.3 の決定 2）。ステップ 1-8 の後で、`runpr.md` か `mergepr.md` を変えた最後のコミットを比べるコミットにして、`requirements_process.md` §4「Fixed commands」のパスの差分のコマンドを実行し、「Diff record form」の形で記録する。記録は本計画だけを変える別のコミットにする。
-   [x] **ステップ 1-10**: AC-22 の手動の確かめを行う（§1.3 の決定 3。設計書 1.2・7.2）。
    -   設計書 7.2 の 3 つの表示（(1) 要約 11 行と選択肢 4 つ、(2) 要約 12 行と選択肢 3 つ、(3) deferred のスレッドが 4 件の deferred の問い）について、見本の値と、PR のブランチの `runpr.md` の書式どおりに要約と問いを出すよう OpenCode のエージェントに頼む指示を用意する。
    -   `AskUserQuestion` で人に頼み、答えを待つ。人は、高さ 24 行の OpenCode の pane でその指示を実行して表示を出し、表示ごとに、すべての選択肢が画面の中に見えて選べるか、自由記述の欄があって入力できるか（AC-01・AC-15・AC-17 の自由記述の答えに要る）を確かめて答える。答えが得られないときは、PR を作らずに止まる。
    -   人の答えを基に、`/runplan` が日付、OpenCode の版、見本の値、表示ごとの結果（収まったか、選べたか、自由記述の欄があったか、所見）を §7 の「AC-22 の記録」に書き、本計画だけを変えるコミットにする。
    -   収まらなかったときは、直さずに止まる。選択肢の description を除くなどの直しは設計書 3.1・3.4 の表を変えるので、計画の決定の変更として人と決める（`requirements_process.md`「Editing an approved document」）。

**確かめ直し。** ステップ 1-9 の比べたコミットより後に `runpr.md` か `mergepr.md` を変えるコミットが入ったら（PR を作る前のレビューの修正、`/fixpr` の修正、手で入れたコミットを含む）、`/runplan` と `/runpr` は `requirements_process.md` §4「Rechecking a record」のとおり記録を確かめ直す。`runpr.md` の要約と問いの書式が変わったときは、ステップ 1-10 もやり直す。

**完了の条件:** 設計書 2.2 の表のすべてのファイルに、設計書 3 章の変更がある。§7 に AC-18・AC-19 の差分の記録と AC-22 の記録がある。`make test` と `make lint` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（ステップ 1-1〜1-3） | `runpr.md` step 4 の組み替え（deferred の問い、詳細ファイルと要約、1 つの問い、答えを受けた手順、戻す入口）と、step 1・step 3 の記述 |
| M2（ステップ 1-4〜1-7） | step 5・description・Authorization、定数 `runprStep5`、`mergepr.md`・`fixpr.md`・`_lib/` の 2 ファイル・`CLAUDE.md`・`requirements_process.md`、ADR-0047・ADR-0046 の食い違いの記述 |
| M3（ステップ 1-8〜1-10） | `make test`・`make lint`、AC-18・AC-19 の差分の記録、AC-22 の記録 |

順序は設計書 8 章のとおりである。

### 3.2 PR 構成

作業は 1 本の PR で行う（§1.3 の決定 1）。`/runplan` は、フェーズ 1 を終えたときにこの PR を作る。推奨するタイトルは `docs(0089): runpr のマージの質問を要約と詳細ファイルと 1 つの問いにし、deferred のスレッドを前の問いで片付ける` である。レビューの観点は次のとおりである。AC-18 の差分の記録で、答えを受けた手順の変更が 4 点に限られること。移した追跡 issue の手順で、メンションの確認と止まる条件が保たれ、数え直しと戻す手順の THREADS が正しいこと。要約が 12 行と 80 桁に収まること。AC-22 の記録があること。

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-1・1-2 は、マージの承認の前の答えの分岐、step 2・3 への戻り、追跡 issue への投稿とメンションの確認の置き場所を組み替える、状態の遷移が複雑な手順で、Sonnet では止まる条件やメンションの確認を移し落としやすいため。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** 足さない（設計書 7.1）。定数 `runprStep5` を直す。`make test` と `make lint` を通す。文書のガードのテスト（`TestPostingPathsCheckMention`、`TestPostingCommandsInventory`、`TestFixprguardCallSites`、`TestRunprDocContract`、`TestDocumentedInvocationsParse`、`TestADRRecords`）は、`make test` と CI の `doc-guard` のジョブに含まれる。
-   **前後の関係の確かめ:** AC-18・AC-19 は、記録した差分の読み取りで確かめる（ステップ 1-9）。`make test` にも CI にも組み込まない。
-   **手動の確かめ:** AC-22（ステップ 1-10）。
-   **読んで確かめる（manual）:** AC-01〜AC-17、AC-20、AC-21 の文の内容は、PR のレビューで設計書 3 章の該当する節と照合しながら読む（設計書 7.2）。
-   **結合テスト・後方互換のテスト:** N/A。コマンドの定義の文書だけを変える。この PR を `/runpr` で回すとき、マージの質問が新しい形で出ることを見る（設計書 7.3）。後方互換の仕組みは設けない（要件書 4.4）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 移した追跡 issue の手順で、メンションの確認が投稿の命令より後になる | 設計書 3.6 の順序をステップ 1-1 に書き、`TestPostingCommandsInventory` を `make test` で通す |
| 手順を移すときに、止まる条件や報告の中身を落とす | AC-18 の差分の記録を読み、移した手順を今の `runpr.md:929-961` と照合する。Opus のレビューを行う（§3.2） |
| 要約が 12 行・80 桁を超えるか、OpenCode で選択肢が画面から外れる | 設計書 3.3 の行の形をそのまま書き、ステップ 1-10 で確かめる。収まらなければ止まって人と決める |
| この PR を `/runpr` で回すとき、`/runpr` を始めたブランチの `runpr.md` と、PR のブランチの `mergepr.md`・`simple-process.md` が食い違うことがある | どちらの形でも誤ったマージにはならないので、移行の仕組みは設けない（要件書 4.4、設計書 9 章） |
| 日程：AC-22 の手動の確かめに人の時間が要る | ステップ 1-10 で答えを待ち、得られなければ PR を作らずに止まる。ほかのステップは先に終えておける |

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] フェーズ 1 完了（ステップ 1-1〜1-10）
-   [ ] PR をマージした

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の意味は次のとおりである。`static` は `make test` に含まれるガードのテスト、`diff` は記録した差分の読み取り、`manual` は PR のレビューで読むか手で操作して確かめるものである。AC-01〜AC-17、AC-20、AC-21 は、コマンドの定義の文が `/runpr` の振る舞いを述べるもので、その意味はテストにせず読んで確かめる（設計書 7.1、`CLAUDE.md`「Testing Strategy」）。

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | `manual` | `runpr.md` step 4 項目 1 を設計書 3.1・3.2 と照合して読む | 1-1 |
| AC-01a | `manual` | 項目 1 が step 4 の先頭、`mergepr prepare` より前にあることを、設計書 3.1「尋ねる時点」と照合して読む | 1-1 |
| AC-02 | `static`・`manual` | `cmd/yt2column/codex_review_docs_test.go::TestPostingPathsCheckMention`・`::TestPostingCommandsInventory`（`make test`）。この 2 つが確かめるのは、step 4 の節の最初の `**Mention check.**` が確認の命令を含み、節の投稿の命令より前にあることだけで、移した手順があることは確かめない。移した手順があること、返信と止まる条件は、設計書 3.6 と今の `runpr.md:929-961` と照合して読む | 1-1 |
| AC-03 | `manual` | 項目 1 の答えの表を設計書 3.1・6.1 と照合して読む | 1-1 |
| AC-04 | `manual` | 同上 | 1-1 |
| AC-05 | `manual` | 項目 1 の対象と `_lib/simple-process.md` を設計書 3.1・3.9 と照合して読む | 1-1・1-6 |
| AC-05a | `static`・`manual` | `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（コードブロックの中の `fixprguard` の行が道具の引数の解析器で読めること。足した `list` の呼び出しがあることは確かめない。`make test`）。呼び出しがあること、戻す手順の入口と THREADS、`requirements_process.md:572-575` を設計書 3.7・3.9 と照合して読む | 1-2・1-6 |
| AC-06 | `manual` | 項目 3 の詳細ファイルの項目を設計書 3.2 と照合して読む | 1-2 |
| AC-07 | `manual` | 要約の行の形を設計書 3.3 と照合して読む。画面に収まることはステップ 1-10 | 1-2・1-10 |
| AC-08 | `manual` | 要約の 7〜10 行目と step 1 の LIMITS_NOTE を設計書 3.3 と照合して読む | 1-2・1-3 |
| AC-09 | `manual` | 要約の 6 行目を設計書 3.3 の表と照合して読む | 1-2 |
| AC-10 | `manual` | question の文と選択肢を設計書 3.4 と照合して読む | 1-2 |
| AC-11 | `manual` | 尋ね直しのたびの書き直しと、step 2・3 から戻ったときに項目 1 から始め、新しい deferred のスレッドがあれば先に deferred の問いを尋ねることを、設計書 2.1・3.5・6.2 と照合して読む | 1-2 |
| AC-12 | `manual` | 承認のメッセージのファイルの扱い、PRE_MERGE のときの写しを設計書 3.2・3.4 と照合して読む | 1-2 |
| AC-13 | `manual` | 問いが 1 つの単一選択であることを設計書 3.4 と照合して読む | 1-2 |
| AC-14 | `manual` | 選択肢の表を設計書 3.4 と照合して読む | 1-2 |
| AC-15 | `manual` | 除外スレッドが残るときの選択肢と自由記述を設計書 3.4 と照合して読む | 1-2・1-6 |
| AC-16 | `static`・`manual` | `cmd/yt2column/codex_review_docs_test.go::TestPostingPathsCheckMention`（「runpr.md step 5」の節を新しい見出しで探す。`make test`）。NEXT を実行する選択肢と分岐がないことを設計書 3.4・3.8 と照合して読む | 1-2・1-4 |
| AC-17 | `manual` | 要約の 11 行目と自由記述の扱いを設計書 3.3・3.4 と照合して読む | 1-2 |
| AC-18 | `diff`・`manual` | 記録した差分の読み取り（比べるパス `.claude/commands/runpr.md`）。下の「差分の記録」の各変更を、設計書 3.5 の 4 点のどれに当たるかに分けて読む | 1-9 |
| AC-19 | `diff`・`manual` | 記録した差分の読み取り（比べるパス `.claude/commands/mergepr.md`）。下の「差分の記録」。新しい文が 2 点を述べることは、設計書 3.9 と照合して読む | 1-5・1-9 |
| AC-20 | `manual` | 設計書 3.8 が挙げる文を読む | 1-4・1-6 |
| AC-21 | `manual` | 設計書 3.1・3.7・3.8・3.9 が挙げる文を読む | 1-1・1-3・1-4・1-6 |
| AC-22 | `manual` | 下の「AC-22 の記録」（AC 自体が手で確かめることを定める） | 1-10 |

### 差分の記録

ステップ 1-9 で `/runplan` が書く。

Diff record (AC-18):

- Compared paths: `.claude/commands/runpr.md`
- Merge-base: `f72fd25d5b7971a3dc8598caa13ae9019c5b7454`
- Compared commit: `d413285efd2ba0288a440a4ca1bbad68cb67d4a0`

Path diff:

````text
diff --git a/.claude/commands/runpr.md b/.claude/commands/runpr.md
index 4a0bbbd5f93f4cc0b3cdf99b420c858756fcb96f..3858457ff6d6574d1b86d1172a560ee20f694a16 100644
--- a/.claude/commands/runpr.md
+++ b/.claude/commands/runpr.md
@@ -1,5 +1,5 @@
 ---
-description: "Drive a PR through Codex's PR review and CI, record its bookkeeping, merge it, and offer the next command, which it runs only when the user chooses"
+description: "Drive a PR through Codex's PR review and CI, record its bookkeeping, merge it, and show the next command, which it does not run"
 argument-hint: "[#PR | branch | task]"
 ---

@@ -45,9 +45,10 @@ this command's own text; read them from `_context.md` when you run it.
 return steps of step 4, by number and heading; a change that renumbers them or
 changes what they do checks that file in the same PR.

-Drive one PR through Codex's PR review and CI, then ask the user one question
-that decides the bookkeeping and the merge and offers the next command (NEXT)
-when there is one, which this command runs only when the user chooses to.
+Drive one PR through Codex's PR review and CI, then ask the user the question
+that decides the bookkeeping and the merge (the merge question), preceded by
+the deferred question when deferred review threads are left. This command
+shows the next command (NEXT) when there is one, and never runs it.

 This is the second half of a task PR. The command that opened it (`mkreq`,
 `mkarch`, `mkplan`, `mkplan2`, or `runplan`; `_lib/task-pr-handoff.md`)
@@ -80,7 +81,7 @@ the repository's interaction limits, a `GET`), and `git ls-remote` — the issue
 `_lib/task-pr-handoff.md` Part A step 4 makes to link the next branch (step
 5), `git ls-remote` to check whether the next branch exists on `origin` when
 step 5 stops before it is ready, and, for the deferred findings the user
-moves to an issue in step 4, `gh issue list`, `gh issue create`, `gh issue comment`, and the thread replies
+moves to an issue in step 4's deferred question, `gh issue list`, `gh issue create`, `gh issue comment`, and the thread replies
 and resolutions. When the user returns a three-document PR to the full
 process in step 4, it also approves the `git push` of the return, the
 operations `fixprguard` makes for `edit-pr` (`gh pr edit`, rewriting the PR's
@@ -88,9 +89,8 @@ title and body), and the thread replies and resolutions of the return. After the
 `gh issue view`, `gh api` (listing sub-issues), and `gh issue close` for its
 `対応 issue` and that issue's link sub-issues (step 5, and step 1's `MERGED`
 case). It also covers what `fixpr.md` and `mergepr.md` authorize when run from
-here, and the next command (NEXT) when the user chooses to run it in step 4.
-It does **not** approve the merge itself: only the user's answer in step 4
-does (`CLAUDE.md`, "Tool Execution Safety"). `coderabbit review` and
+here. It does **not** approve running the next command (NEXT), nor the merge
+itself: only the user's answer to step 4's merge question approves the merge (`CLAUDE.md`, "Tool Execution Safety"). `coderabbit review` and
 `codexreview` have standing approval: the local CLI review runs `codexreview`,
 which runs Codex CLI and sends the diff and what Codex reads to OpenAI's
 service, only when the CodeRabbit CLI fails.
@@ -230,8 +230,9 @@ Every `fixpr.md` and `mergepr.md` run below is given the resolved PR number
   - 期限が近い: show now a warning with the state name, `expiresOn` (UTC),
     and the guide's chapter 10, where the limit is renewed. Keep that one line
     as **LIMITS_NOTE** for the rest of the run, and include it in every report
-    after this step: each stop report, the message of step 4's question, and
-    the final report of step 5.
+    after this step: each stop report, the summary and detail file of each question
+    of step 4 (the state and the date in the summary, the full text in the
+    detail file), and the final report of step 5.
   - 制限なし: the state name, the guide's chapter 10, and what the user does.
   - 制限が弱い: the state name, `expiresOn`, that the limit must be changed to
     `collaborators_only`, the guide's chapter 10, and what the user does.
@@ -560,8 +561,9 @@ finished review observed had and every review in progress lacked, so that a
   split the history), or when `git merge-base` exits with a code other than `0`
   or `1`. The summary comment is rewritten, so an earlier run's markers count
   only while the current summary comment still shows one: LAST_REVIEWED may
-  come out older than it was, never newer. When it is none, step 4's PR-review
-  question treats the PR as having changes Codex has not reviewed, and
+  come out older than it was, never newer. When it is none, step 4's merge
+  question treats the PR as having changes Codex has not reviewed (the "先に PR
+  レビューを依頼" option is shown), and
   Escalation's decision is `ask`, as for a LAST_REVIEWED that is not an
   ancestor of the head.
 - **Requesting the review.** Post a request with
@@ -666,11 +668,13 @@ finished review observed had and every review in progress lacked, so that a
     per PR on this command's own judgement; a later `request` is treated as
     `ask`, so the budget is spent only on the first and on what the user chooses;
   - `skip`: continue;
-  - `ask`: continue, and carry the reason into step 4 (the PR-review question).
+  - `ask`: continue, and carry the reason into step 4: the merge question's
+    "先に PR レビューを依頼" option and its detail file.
 - When the PR is reviewed and no unseen thread is left, look at the
   unresolved threads `fixpr` reported:
-  - `deferred` (past its round cap): keep them for step 4;
-  - excluded, on a three-document PR (step 2): keep them for step 4, with the
+  - `deferred` (past its round cap): keep them for step 4's deferred question;
+  - excluded, on a three-document PR (step 2): keep them for step 4's merge
+    question, with the
     `threadId`, `databaseId`, `fingerprint`, topic, URL, path, and reason the
     latest `fixpr` Final report gave. Only a thread that report lists as
     excluded counts; any other is sorted by the other items of this list.
@@ -688,7 +692,7 @@ finished review observed had and every review in progress lacked, so that a
     and why it is open, so the user can fix, re-classify, or answer it.
 - **Untrusted threads.** While an untrusted thread is unresolved, do not
   merge. When the untrusted threads are the only reason to stop (the
-  `deferred` and excluded ones wait for step 4 and do not count), ask the user once, with
+  `deferred` ones wait for step 4's deferred question and the excluded ones for its merge question, and do not count), ask the user once, with
   one `AskUserQuestion` call, which of them to process (none included). Show
   each thread's URL and its reasons (every `findings` entry of the latest
   `list`); do not fetch or read their content: the user checks them on GitHub.
@@ -711,134 +715,375 @@ finished review observed had and every review in progress lacked, so that a

 ## 4. Confirm, record, and merge

-Ask the user once, so a reviewer who has already reviewed the PR answers once
-rather than once per decision.
+Ask the user as few times as the PR allows, so a reviewer who has already
+reviewed the PR answers once rather than once per decision: when step 3 left
+`deferred` threads, the deferred question (item 1) comes first and settles
+them; then the merge question (item 3) is asked once per arrival. Every arrival at this
+step, including each return to it from step 2 or step 3 after a fix, a PR
+review, or a push that changed the head, starts at item 1.
+
+Each time a question of this step is asked, the deferred question or the merge
+question, the first time and every time it is asked again, read the three
+mention lists of step 3, "Reading the PR", just before it, and apply step 3's
+"Mentions by other accounts" to them. When it stops, stop here: no question,
+no merge.
+
+**Summary and detail file.** Each question shows the conversation a short
+summary of fixed lines, and puts everything long in a detail file outside the
+work tree, so the question and its options stay on the screen. Write the
+summary, the question, the options, and the detail file in Japanese, one item
+per line, each summary line within 80 display columns (count a full-width
+character as 2); the line shapes below are the examples to follow.
+
+- **Details directory.** Right before the first question of this run, make one
+  details directory with `mktemp -d /tmp/runpr.XXXXXX` (a directory only the
+  user can read, with a name nobody can guess, outside the work tree, so
+  `git status --short` does not change). Use it for every question of this run
+  and make no second one. Do not use the scratchpad for it: its path is too
+  long for the summary line that shows it.
+- **Round.** Count each question asked, the deferred question and the merge
+  question alike, and each asking again as a new **round**, from 1.
+- **Detail file.** Write each round's detail file in the details directory,
+  right before asking: `deferred-<round>.md` for the deferred question and
+  `merge-<round>.md` for the merge question. Check first that no file of that
+  name exists (`test -e`). Never overwrite or reuse an earlier round's file, so
+  the path the summary shows is always this round's. Start every detail file
+  with the PR number, the head commit, the round, the kind of question, and a
+  sentence saying that editing the file changes neither the merge message nor
+  anything else this command does.
+- **Never read back.** This command never reads a detail file, never decides
+  from one, and never merges from one. The merge uses the approval message
+  files only (item 2). It never deletes the details directory, so the user can
+  read the files after the merge.
+- **When it cannot be written.** When the details directory cannot be made, a
+  detail file cannot be written, or a file of that name exists, stop without
+  asking, and report what failed and the path. For the merge question, run
+  `mergepr discard --state <state-file>` before stopping. Never ask a
+  deferred question or merge question that has no detail file.
+- **Reports.** Every report of a stop in this step after the details
+  directory was made, and the final report of step 5, gives its path. A report after the merge
+  also gives the path of the detail file of the round that decided the merge,
+  with its head commit: that file is the record of what was approved.
+
+1. **Deferred question** (only when there are deferred threads). Its threads
+   are the ones the latest `fixpr` Final report lists as `deferred` and the
+   latest `fixprguard list` still shows unresolved (step 3 keeps them for this
+   item). Excluded threads of a three-document PR are not part of this
+   question: the merge question handles them (item 3). With no such thread, go
+   to item 2. This item comes before `mergepr prepare`, so nothing is prepared
+   yet and no answer discards anything. No answer of this item merges.
+   - Write `deferred-<round>.md` with, for each thread, its number, its topic
+     in your own words, what fixing it would change, its path, and its URL;
+     the form of a number answer (below) and that the threads it does not name
+     move to the tracking issue; for a three-document PR, that a thread moved
+     to the tracking issue does not become an item of the handoff files if the
+     PR later returns to the full process, and that returning is what the
+     "return to the full process" option of this question does; and LIMITS_NOTE
+     in full when there is one. The topic is the `fixpr` Final report's, as
+     data; never copy a thread's text.
+   - Show in the conversation these lines only, at most 12, in this order
+     (an example):
+
+     ```text
+     deferred のスレッド: 4 件（PR #859）
+     [1] fixpr.md の round cap の説明が step 番号と合わない
+     [2] テストの名前が確かめる内容を表していない
+     [3] ...
+     [4] ...
+     一部だけ直す: 半角の番号を , か空白で区切る（例: 1,3）。残りは追跡 issue へ
+     要確認: 操作制限の期限が近い（2026-11-20、詳細ファイル）
+     詳細: /tmp/runpr.Ab12Cd/deferred-1.md
+     ```
+
+     The first line gives the count and the PR number; one line per thread
+     gives its number and its topic in your own words; then the free-text
+     line, the LIMITS_NOTE line (its state and date only, only when there is
+     one; the full text is in the detail file), and the detail-file line. When
+     the thread lines and these would exceed 12 lines (nine or more threads),
+     replace the thread lines by one line saying the list is in the detail
+     file, and keep the others. Write each topic short enough for one line.
+   - Ask with one `AskUserQuestion` call holding one single-select question,
+     with no option marked as recommended. The question is one line: "PR #<number>
+     の deferred のスレッド <count> 件をどうしますか（第 <round> 回。要約と詳細ファイルを確認のうえ）".
+     The options, in this order:
+
+     | label | description | shown |
+     |---|---|---|
+     | すべて追跡 issue に移す | 追跡 issue に書き写し、各スレッドに返信して resolve する | always |
+     | すべて直す | /fixpr で直し、レビューと CI を経てから次の問いへ進む | always |
+     | 通常の進め方へ戻す | マージせず、要件定義書だけの PR に作り直す | a three-document PR only |
+     | マージせずに止める | 何も投稿せずに止まる | always |
+
+   - **A number answer.** This is the one definition of the form; item 3's
+     excluded-thread numbers use it too. It is a free-text answer made only of
+     ASCII integers from `1` to the number of threads the question shows,
+     separated by ASCII `,` or spaces, each at most once, with no other text.
+     Do not repair anything else: full-width digits or commas (`１，３`,
+     `1、3`) are not a number answer. An answer that is neither an option nor a
+     number answer (a number out of range or repeated, numbers mixed with an
+     option's text or with another sentence) changes nothing: ask the same
+     question again as a new round.
+   - **What each answer does.**
+     - "すべて追跡 issue に移す": move every deferred thread ("Moving threads to
+       the tracking issue" below), which also counts them again; when that
+       does not stop, go to item 2.
+     - A number answer: move the threads it does not name, with the same
+       procedure; if it stops, stop (do not run `fixpr`). Otherwise run
+       `fixpr.md` for `<number>` with the named threads' IDs as its caller
+       override (pass them in the instructions you follow; `fixpr` reads them
+       from this session), and return to step 2. A number answer that names
+       every thread is the same as "すべて直す". When `fixpr` stops after the
+       move, report, besides `fixpr`'s report, the tracking issue's URL, the
+       URLs of the moved threads, and the URLs of the threads left unresolved
+       to be fixed: the next run asks about those again.
+     - "すべて直す": run `fixpr.md` for `<number>` with the IDs of every
+       deferred thread as its caller override, and return to step 2.
+     - "通常の進め方へ戻す": run the return steps (item 3, "When the answer
+       returns the PR to the full process"), entering from this item: there is
+       no preparation yet, so the first return step does not discard one.
+     - "マージせずに止める": post nothing, stop, and report. The deferred
+       threads stay unresolved.
+
+     After step 2 and step 3, a fix or a return comes back to item 1: when
+     deferred threads are still unresolved, ask the deferred question again;
+     otherwise go to item 2.
+
+   **Moving threads to the tracking issue.** Used by the "move" answers of
+   this item, so no deferred thread is silently dropped. Look for this PR's
+   tracking issue
+   (`gh issue list --state open --search "PR #<number> のレビュー指摘の追跡 in:title"`),
+   unless this run already found or created one: keep that issue's number for
+   the rest of the run and use it from then on, without searching again (a
+   search may not find an issue just created, and a second tracking issue must
+   not be created). Skip any finding whose thread URL the issue or its
+   comments already list. Write the issue comment or body to a file outside the
+   work tree, and run the mention check on it before posting it:
+   **Mention check.** Before posting, run `grep -qiF -- '<review-mention>' <file>`
+   on each file of text to post. Post only when it exits `1` for every file: exit
+   code `0` means the text contains the mention, and any other code means the
+   check failed. Never rewrite the text to get past the check.
+   When the check fails, do not post: stop and report, with the tracking
+   issue's URL when there is one. Nothing is prepared, so there is nothing to
+   discard.
+   Add the findings to the issue as a comment
+   (`gh issue comment <issue> --body-file <file>`), or create it
+   (`gh issue create --title "PR #<number> のレビュー指摘の追跡" --body-file <file>`)
+   when there is none. Write in Japanese, listing each finding's topic, the
+   change it asks for, and its thread URL. Then reply to each of those threads
+   in Japanese with the issue URL and resolve it, as `fixpr.md` Phase 5 does:
+   write the reply to a file outside the work tree, run the mention check on
+   it, and only when it passes run, with the `databaseId` and `fingerprint` the
+   latest `fixpr` report gave for the thread,
+
+   ```
+   fixprguard reply --protocol 6 --reviewer <reviewer> --thread-id <threadId> --comment-id <databaseId> --fingerprint <fingerprint> --body-file <reply-file> <number>
+   ```

-Each time this step's question is asked, the first time and every time it is
-asked again, read the three mention lists of step 3, "Reading the PR", just
-before it, and apply step 3's "Mentions by other accounts" to them. When it
-stops, stop here: no question, no merge.
+   If `reply` stops for any thread, or a reply fails the mention check, stop
+   and report each such thread with its URL and stop kind, and the tracking
+   issue's URL. The tracking issue already lists it, so a rerun of this
+   command must not add it to the issue again. For `reply-not-resolved`, the
+   reply may be posted: ask the user to resolve the thread by hand, never
+   replying again. Otherwise count again: run

-1. Run `.claude/commands/mergepr.md` steps 1–2 for `<number>` (prepare, draft
+   ```
+   fixprguard list --protocol 6 --reviewer <reviewer> <number>
+   ```
+
+   and when any thread that was to be moved is still unresolved (for example
+   one a former run stopped on with `reply-not-resolved`, which the steps above
+   skip because the issue already lists it), stop and report its URL and the
+   tracking issue's URL: do not go on to item 2 or call `fixpr` with a thread
+   left that the merge question would no longer show.
+
+2. Run `.claude/commands/mergepr.md` steps 1–2 for `<number>` (prepare, draft
    the squash commit message). When PRE_MERGE has approvals, draft the message
    with a line naming them (e.g. "Approves 01_requirements.md"), and keep a
-   second version without that line for the option that merges without them.
+   second version of the body without that line, written as a file in the
+   prepared directory next to the subject and body files, for the option that
+   merges without them. These files, and the copy item 4 makes of them, are the
+   **approval message files**: the only source of the message the merge uses.
    For the task's final plan PR, the body ends with `Closes #<issue>` for the
    issue the requirements document names as its own (`対応 issue`); a task
    without one gets no such line.
-2. Ask with one `AskUserQuestion` call. In your message, show what
-   `mergepr.md` step 3 requires (PR URL, base branch, CI result, the full
-   subject and body, the record of which rule decided the subject's prefix,
-   and the version without the approval line when it applies), the PRE_MERGE change (which lines of which files, and for an
-   approval the reviewer name and date it writes, and the PR number fill-in
-   lines separately, since the merge without the approvals commits only
-   those), each diff record PRE_MERGE re-records (with the AC's text, the
-   difference between the recorded and the new output, and your judgment
-   that the AC holds on the new output), every editorial correction any `fixpr` run of this session reported
-   it could not append to the PR description (each run's Final report,
-   accumulated across rounds), and, for each task document this PR changes
-   that exists on `origin/main`, the lines of its change history (the rule's
-   "Change history" definition) in `git show origin/main:<document>` that hold
-   the marker `#未記入` (fixing such a marker needs a separate PR, never part
-   of PRE_MERGE, as the rule's "PR number" bullet says), the `deferred` threads (each
-   with its number, topic, what fixing it would change, and its URL), the
-   excluded threads of a three-document PR (step 3; each numbered in the same
-   sequence, marked as excluded, with its topic, the reason, and its URL), and
-   NEXT.
-   When NEXT is `/runplan`, also show the `実装モデル要件` of the PR that
-   NEXT will open, and the model to start the new session with. That PR is
-   the one of the plan's first `### PR-N 作成ポイント` marker whose
-   `PR を作成した` box is unchecked (for a plan PR, a marker after this PR's
-   own; for a task-document PR that first approves the plan, its first
-   marker), or, in a plan without PR markers, the plan itself, whose value is
-   in the `PR 構成` section of its §3. Read the value as `_context.md`, "PR
-   marker conventions", says, and show the model from that section's table:
-   for an old value, show the old value and the value it is read as; for an
-   unreadable value, show its text as written and that it is unreadable, and
-   that no model is recommended; when there is no value, say so and, as far
-   as known, why (the marker has no `実装モデル要件` field, or the plan
-   without PR markers has no `PR 構成` section or no value line in it), and
-   that no model is recommended. The value and the model let the user decide
-   whether this session's model fits and, since step 5 suggests `/clear`
-   before NEXT, which model to start the new session with. Step 5 reuses the
-   value and the model (or their absence) shown here, without reading the
-   plan again; when step 4 is asked more than once, it uses the ones shown in
-   the question that decided the merge. Give LIMITS_NOTE (step 1) when there is one. Each question has two
-   to four options:
-   - **Deferred threads** (only when there are some, counting the excluded
-     threads): with one to three,
-     `multiSelect` with one option per thread plus "fix none"; with more,
-     single-select "fix all" / "fix none", and a subset by number in the
-     free-text answer. An excluded thread's option is labeled as excluded,
-     and "fix all" does not include the excluded threads: one is fixed only
-     when chosen by itself or by its number. Choosing one overrides its
-     exclusion judgment.
-   - **PR review** (only when LAST_REVIEWED is behind the head or none, so
-     changes exist that Codex has not reviewed): single-select "merge without another PR
-     review" / "request a PR review first". Show the Escalation reason in the
-     message, and mark "request a PR review first" as recommended when
-     Escalation said `ask`. This is how the user asks for a review at will.
-   - **Merge**, applying when nothing is chosen to fix. Offer those of these
-     that apply, in this order, and always the last:
-     - record PRE_MERGE and merge, without running NEXT (or "merge" when
-       PRE_MERGE is none). Mark this option, and no other Merge option, as
-       recommended, whether NEXT is set or not and whatever the PR's kind;
-     - record PRE_MERGE, merge, and run NEXT in this session (only when NEXT is
-       set);
-     - merge without the approvals, recording only the PR numbers ("merge
-       without the approvals" when the fill-in is none; only for a
-       task-document PR whose PRE_MERGE has approvals): the documents stay
-       `draft`, PRE_MERGE is reduced to its PR number fill-in (none when no
-       marker is left to fill), the message without the approval line is used,
-       NEXT is none, and step 5 detaches. The fill-in still runs, so this PR's
-       lines do not reach `main` with the marker. A plan PR is
-       never merged without its PRE_MERGE: its record would be missing on
-       `main`, and implementation would merge under a `draft` document.
-       A three-document PR is never offered this option: merged with its
-       documents `draft`, it would leave a task that `/mkarch`, `/mkplan`,
-       and `/runplan` all stop on;
-     - return to the full process (only for a three-document PR, whether or
-       not PRE_MERGE has already approved its documents);
-     - do not merge (revising the message goes through the free-text answer).
-
-     While an excluded thread of a three-document PR is unresolved, the Merge
-     options are only "return to the full process" and "do not merge": the
-     three documents are never merged with one left open.
+3. **Merge question.** Write the subject file, the body file, and the version
+   without the approval line, and check the subject as `mergepr.md` step 2
+   does. Then write `merge-<round>.md` and ask one question, as follows.
+   - **Detail file.** It holds what `mergepr.md` step 3 requires and what the
+     summary only points to: the PR URL, the base branch, and the CI result;
+     the full subject and body, and the body without the approval line when
+     that option is shown, built by concatenating the approval message files
+     into the detail file with `cat` (never retype them), together with the
+     absolute paths of the files the merge takes them from; the record of
+     which rule decided the subject's prefix; the PRE_MERGE change (which lines
+     of which files, the reviewer name and date an approval writes, and the PR
+     number fill-in lines separately, since the merge without the approvals
+     commits only those); each diff record PRE_MERGE re-records, with the AC's
+     text, the difference between the recorded and the new output, and your
+     judgment that the AC holds on the new output; every editorial correction
+     any `fixpr` run of this session reported it could not append to the PR
+     description (each run's Final report, accumulated across rounds); for
+     each task document this PR changes that exists on `origin/main`, the
+     lines of its change history (the rule's "Change history" definition) in
+     `git show origin/main:<document>` that hold the marker `#未記入` (fixing
+     such a marker needs a separate PR, never part of PRE_MERGE, as the rule's
+     "PR number" bullet says); the excluded threads of a three-document PR
+     (step 3), numbered 1, 2, ... in order, each marked as excluded, with its
+     topic, the reason, and its URL; the Escalation decision and its reason,
+     when there are changes Codex has not reviewed; NEXT, and, when NEXT is
+     `/runplan`, the `実装モデル要件` of the PR that NEXT will open and the
+     model to start the new session with (below); and LIMITS_NOTE in full when
+     there is one.
+   - **NEXT and the model.** When NEXT is `/runplan`, the PR NEXT will open is
+     the one of the plan's first `### PR-N 作成ポイント` marker whose
+     `PR を作成した` box is unchecked (for a plan PR, a marker after this PR's
+     own; for a task-document PR that first approves the plan, its first
+     marker), or, in a plan without PR markers, the plan itself, whose value is
+     in the `PR 構成` section of its §3. Read the value as `_context.md`, "PR
+     marker conventions", says, and take the model from that section's table:
+     for an old value, the old value and the value it is read as; for an
+     unreadable value, its text as written, that it is unreadable, and that no
+     model is recommended; when there is no value, that and, as far as known,
+     why (the marker has no `実装モデル要件` field, or the plan without PR
+     markers has no `PR 構成` section or no value line in it), and that no
+     model is recommended. The value and the model let the user decide whether
+     this session's model fits and, since step 5 suggests `/clear` before NEXT,
+     which model to start the new session with. Step 5 reuses the value and
+     the model (or their absence) shown here, without reading the plan again;
+     when this question is asked more than once, it uses the ones shown in the
+     round that decided the merge.
+   - **Summary.** Show in the conversation these lines only, in this order,
+     each kind at most once (so at most 12 lines, whatever the PR), with the
+     condition in parentheses (an example of each line is given, and the
+     example's words are the fixed text):
+
+     ```text
+     PR #859 → main ／ CI: 成功                                  (always)
+     件名: docs(0089): draft the architecture ... (#859)         (always)
+     PRE_MERGE: 承認 1 件・PR 番号の記入 2 行（2 ファイル）       (always; or "PRE_MERGE: なし")
+     未レビューの変更: あり（Escalation: ask）                    (always; or "未レビューの変更: なし")
+     除外スレッド: 2 件（番号で判定を覆せる）                      (a three-document PR with unresolved excluded threads)
+     NEXT: /runplan 0083（要件 sonnet → Sonnet）                  (always; the forms below)
+     要確認: diff 記録の再記録 1 件（詳細ファイル）                (when PRE_MERGE re-records a diff record)
+     要確認: PR の説明に追記できなかった訂正 2 件（詳細ファイル）  (when there are such corrections)
+     要確認: #未記入 の行 1 件（詳細ファイル）                    (when there are such lines)
+     要確認: 操作制限の期限が近い（2026-11-20、詳細ファイル）      (when there is a LIMITS_NOTE)
+     自由記述: メッセージの修正 ／ 除外スレッドの判定を半角の番号で覆す (always; the second half only with the excluded-threads line)
+     詳細: /tmp/runpr.Ab12Cd/merge-3.md                          (always)
+     ```
+
+     - Line 2 shows the subject in full and has no width limit; the other
+       lines hold only fixed text and short values (numbers, branch, counts,
+       dates, task ID, the requirement value, the model name, the path), so
+       each stays within 80 columns. Write the subject line from the checked
+       subject file; the merge uses the file, not this line.
+     - Line 3 gives the count of approvals and the number of PR-number lines
+       and files; which line of which file is in the detail file. Line 4 says
+       there are changes Codex has not reviewed when LAST_REVIEWED is behind
+       the head or none, with the Escalation decision (`ask`, `skip`, ...) on
+       the same line; the reason is in the detail file.
+     - Kinds 7 to 10 appear only for what exists. Kinds 7 to 9 give a kind and
+       a count; kind 10 gives the state and the expiry date, not a count.
+     - Kind 6 is `NEXT: なし` when there is none; `NEXT: /mkarch 0089` for
+       `/mkarch` or `/mkplan`; for `/runplan`,
+       `NEXT: /runplan 0083（要件 sonnet → Sonnet）` for a value that is one of
+       the three, `NEXT: /runplan 0083（要件 standard＝sonnet → Sonnet）` for an
+       old value (the old value, `＝`, the value it is read as),
+       `NEXT: /runplan 0083（要件の値を読めない、推奨モデルなし）` for an
+       unreadable value, and `NEXT: /runplan 0083（要件の値がない、推奨モデルなし）`
+       when there is no value. The unreadable text and the reason there is no
+       value are in the detail file.
+     - Never show in the conversation the body, the record of the prefix rule,
+       or the version without the approval line: they are in the detail file.
+   - **Question.** Ask with one `AskUserQuestion` call holding one single-select
+     question: "PR #<number> をどうしますか（第 <round> 回。要約と詳細ファイルを確認のうえ）",
+     in one line, with none of the summary or the detail file in it. Offer the
+     options that apply, in this order; "recommended" means the label carries
+     the recommended mark:
+
+     | label | description | shown | recommended |
+     |---|---|---|---|
+     | 先に PR レビューを依頼 | Codex に PR レビューを頼み、終わったらこの問いに戻る | LAST_REVIEWED is behind the head or none (changes Codex has not reviewed), and Escalation said `ask` | yes |
+     | PRE_MERGE を記録してマージ ("マージ" when PRE_MERGE is none) | 要約の件名と詳細ファイルの本文でマージする | always | unless the first option is shown and recommended |
+     | 先に PR レビューを依頼 | (as above) | the same changes exist, and Escalation did not say `ask` | no |
+     | 承認なしでマージ ("承認なしでマージ、PR 番号の記入だけ" when its fill-in is not none) | 文書は draft のまま、承認の行のない本文でマージする | a task-document PR whose PRE_MERGE has approvals, not a plan PR, and not a three-document PR | no |
+     | 通常の進め方へ戻す | マージせず、要件定義書だけの PR に作り直す | a three-document PR | no |
+     | マージしない | 何もせずに止まる | always, last | no |
+
+     "先に PR レビューを依頼" is shown once, as the first or the third option,
+     never both. "承認なしでマージ" is not shown for a plan PR, whose record
+     would be missing on `main` and under whose `draft` document implementation
+     would merge, nor for a three-document PR, which merged with its documents
+     `draft` would leave a task that `/mkarch`, `/mkplan`, and `/runplan` all
+     stop on, so at most four options are shown. Mark no other Merge option as
+     recommended, whether NEXT is set or not and whatever the PR's kind. There
+     is no option that merges and runs NEXT: NEXT is only shown.
+
+     "承認なしでマージ" (the merge without the approvals, recording only the PR
+     numbers): the documents stay `draft`, PRE_MERGE is reduced to its PR
+     number fill-in (none when no marker is left to fill), the message without
+     the approval line is used, NEXT is none, and step 5 detaches. The fill-in
+     still runs, so this PR's lines do not reach `main` with the marker.
+
+     While an excluded thread of a three-document PR is unresolved, show only
+     "先に PR レビューを依頼" (only when changes Codex has not reviewed exist;
+     recommended when Escalation said `ask`), "通常の進め方へ戻す", and
+     "マージしない", in this order: the three documents are never merged with
+     one left open.
+   - **Free-text answers.** A free-text answer never merges, and does not count
+     as choosing an option. It is one of these:
+     - A number answer (item 1 defines the form), when there are excluded
+       threads: it names the excluded threads, by their numbers in the detail
+       file, whose exclusion the user overrides. Pass each to `fixpr.md`
+       as `<threadId>=<fingerprint>` with the fingerprint of the `fixpr`
+       Final report the detail file showed (`fixpr.md`, "Overriding an
+       exclusion"): the user's answer is what overrides the judgment, so
+       `fixpr` fixes it without judging it again and records the override.
+       Run `mergepr discard --state <state-file>`, run `fixpr.md` for
+       `<number>` with them, and return to step 2.
+     - An answer made only of digits and separators that is not a number answer
+       (a number out of range or repeated, or any number when there are no
+       excluded threads): change nothing and ask this question again as a new
+       round.
+     - Any other free text revises the message: rewrite the subject and body
+       files (and the version without the approval line), check the subject
+       again as `mergepr.md` step 2 does, and ask this question again as a new
+       round, writing a new `merge-<round>.md` from the new files. The
+       preparation stays, so item 1 and item 2 are not run again. Whatever the
+       text, it only revises and asks again, never merges (not even "fix the
+       subject and merge").
+
+   Each time this question is asked again, whatever the reason, write a new
+   round's detail file and summary from the head and the approval message files
+   as they are then. The ways back to it are the same as before: after a PR
+   review, a fix, a failed push of PRE_MERGE, or a head that moved, the path
+   goes through step 2 and step 3 and starts at item 1, so a new deferred
+   thread is asked about first.

    An answer that merges is the explicit approval `mergepr.md` step 3 asks for.
-   Do not merge on anything less. When the free-text answer revises the
-   message, rewrite the subject and body files, check the subject again as
-   `mergepr.md` step 2 does, and ask step 4 again with the revised message.
+   Do not merge on anything less. A merging answer merges with the approval
+   message files of the round that asked (the version without the approval
+   line for the merge without the approvals), never with the detail file.
    When the answer otherwise does not merge, run
-   `mergepr discard --state <state-file>` and stop.
+   `mergepr discard --state <state-file>`, stop, and report.

    When the answer requests a PR review first, run
    `mergepr discard --state <state-file>`, ask Codex for a PR review as step 3,
    "Requesting the review", says, with
    `gh pr comment <number> --body "<review-request>"` (the user's request, so
    it does not count against Escalation's one), wait for it and handle what it
-   opens as step 3 says, and return to step 2; then ask step 4 again. Do not
-   merge on this answer.
-
-   When the answer chooses deferred threads to fix now, run
-   `mergepr discard --state <state-file>`, run `fixpr.md` for `<number>` with
-   those thread IDs as its caller override (pass them in the instructions you
-   follow; `fixpr` reads them from this session), and return to step 2; then
-   ask step 4 again. Do not merge on this answer: the fix changes the head, so
-   the message and the merge are confirmed again on the new head. Pass each
-   excluded thread chosen as an override thread instead, as
-   `<threadId>=<fingerprint>` with the fingerprint of the `fixpr` Final report
-   the question showed (`fixpr.md`, "Overriding an exclusion"): the user's
-   answer is what overrides the judgment, so `fixpr` fixes it without judging
-   it again and records the override.
+   opens as step 3 says, and return to step 2; then ask step 4 again, from its
+   item 1. Do not merge on this answer.

    When the answer returns the PR to the full process (only a three-document
-   PR is offered it), do not merge. Run the return steps in order; each stop
+   PR is offered it, in item 1 and in this item), do not merge. Run the return steps in order; each stop
    below ends this command, and a later run finishes the return (step 2, "A
    return in progress"):

-   1. **Ask first.** First run `mergepr discard --state <state-file>`. Before
-      touching any file, settle everything the return needs. An answer that
+   1. **Ask first.** When entering from this item, first run
+      `mergepr discard --state <state-file>`; when entering from item 1 there
+      is no preparation, so discard nothing. Before touching any file, settle
+      everything the return needs. An answer that
       chooses the return together with anything else (threads to fix, a PR
       review) is not a choice: ask step 4 again, from its item 1. REASON
       is the answer's free text as written, or `理由の記載なし` when it gives
@@ -846,7 +1091,18 @@ stops, stop here: no question, no merge.
       three documents; when one ADR change cannot be split that way, ask the
       user with one `AskUserQuestion` call, and without an answer change
       nothing and stop. THREADS come from the latest `fixpr` Final report
-      (`_lib/simple-process.md`, "Returning to the full process", "Inputs").
+      (`_lib/simple-process.md`, "Returning to the full process", "Inputs"),
+      limited to the threads the latest `fixprguard list` still shows
+      unresolved: run
+
+      ```
+      fixprguard list --protocol 6 --reviewer <reviewer> <number>
+      ```
+
+      and keep only the threads whose `threadId` it lists, taking each one's
+      `databaseId`, `fingerprint`, and the rest from the Final report. A thread
+      moved to the tracking issue in item 1 is resolved and drops out, so the
+      return neither writes it into a handoff file nor replies to it again.
    2. **The return commit.** Follow `_lib/simple-process.md`, "Returning to the
       full process", with REASON, ADR_SPLIT, and THREADS.
    3. **Local review.** Follow `.claude/commands/_lib/local-cli-review.md` with
@@ -921,49 +1177,21 @@ stops, stop here: no question, no merge.
       A later run pushes it (step 2) after a failure that does not move the
       head; for a rejected push (the head moved), report the recovery of
       return step 4.
-   7. Return to step 2, then step 3, and ask step 4 again. The PR is no longer
+   7. Return to step 2, then step 3, and ask step 4 again, from its item 1.
+      The PR is no longer
       a three-document PR, so step 3 runs `fixpr` once more for the excluded
       threads left on `01_requirements.md`, which are now handled as in the
       full process.

-   When the answer merges, first move the deferred threads not chosen to an
-   issue, so none is silently dropped. Look for this PR's tracking issue
-   (`gh issue list --state open --search "PR #<number> のレビュー指摘の追跡 in:title"`).
-   Skip any finding whose thread URL the issue or its comments already list.
-   Write the issue comment or body to a file outside the work tree, and run
-   the mention check on it before posting it:
-   **Mention check.** Before posting, run `grep -qiF -- '<review-mention>' <file>`
-   on each file of text to post. Post only when it exits `1` for every file: exit
-   code `0` means the text contains the mention, and any other code means the
-   check failed. Never rewrite the text to get past the check.
-   When the check fails, do not post: run `mergepr discard --state <state-file>`,
-   stop, and report, without merging.
-   Add the others to the issue as a comment
-   (`gh issue comment <issue> --body-file <file>`), or create it
-   (`gh issue create --title "PR #<number> のレビュー指摘の追跡" --body-file <file>`)
-   when there is none. Write in Japanese, listing each finding's topic, the
-   change it asks for, and its thread URL. Then reply to each of those threads
-   in Japanese with the issue URL and resolve it, as `fixpr.md` Phase 5 does:
-   write the reply to a file outside the work tree, run the mention check on
-   it, and only when it passes run, with the `databaseId` and `fingerprint` the
-   latest `fixpr` report gave for the thread,
-
-   ```
-   fixprguard reply --protocol 6 --reviewer <reviewer> --thread-id <threadId> --comment-id <databaseId> --fingerprint <fingerprint> --body-file <reply-file> <number>
-   ```
-
-   If `reply` stops for any thread, or a reply fails the mention check, do not
-   merge: run
-   `mergepr discard --state <state-file>`, stop, and report each such thread
-   with its URL and stop kind. The tracking issue already lists it, so a rerun
-   of this command must not add it to the issue again. For
-   `reply-not-resolved`, the reply may be posted: ask the user to resolve the
-   thread by hand, never replying again. Otherwise continue below.
-3. When the answer records PRE_MERGE (including the merge without the
+4. When the answer records PRE_MERGE (including the merge without the
    approvals, when its PR number fill-in is not none):
-   - Note the prepared `headRefOid`, and copy the approved subject and body
-     files to the scratchpad (`mergepr.md` keeps them in the prepared
-     directory, which `discard` deletes). Then run
+   - Note the prepared `headRefOid`, and copy the approval message files of the
+     round that decided (the subject file and the body file; for the merge
+     without the approvals, the subject file and the body file without the
+     approval line) to the details directory as `approved-<round>.subject` and
+     `approved-<round>.body` (`mergepr.md` keeps them in the prepared
+     directory, which `discard` deletes; the details directory is not the
+     scratchpad, which a session without one cannot write). Then run
      `mergepr discard --state <state-file>`.
    - Make the PRE_MERGE commit and push it. If the push is rejected (someone
      pushed meanwhile), `git pull --rebase` the bookkeeping commit onto the new
@@ -983,14 +1211,16 @@ stops, stop here: no question, no merge.
      merge: discard the preparation and return to step 3, which handles it
      (`fixpr`, or the untrusted-thread question), then step 2, and ask step 4
      again.
-   - Otherwise merge with the approved message files without asking again: the
-     answer approved this message for exactly this head.
-4. Merge with `mergepr.md` step 4, and report as its step 5 says. If the merge
-   is refused because the head moved, discard the preparation and return to
-   step 2 rather than re-running `mergepr.md` from its step 1: the new commits
-   need review, and step 4 asks again.
+   - Otherwise merge with the approved message files (the copies) without
+     asking again: the answer approved this message for exactly this head.
+5. Merge with `mergepr.md` step 4, and report as its step 5 says, adding the
+   path of the details directory and of the detail file of the round that
+   decided the merge, with its head commit (the report convention above). If
+   the merge is refused because the head moved, discard the preparation and
+   return to step 2 rather than re-running `mergepr.md` from its step 1: the
+   new commits need review, and step 4 asks again.

-## 5. Switch branch, then run or show NEXT
+## 5. Switch branch, then show NEXT

 Once `mergepr.md` reports the merge:

@@ -1032,24 +1262,23 @@ Once `mergepr.md` reports the merge:
   merged, which issues are still open, and that running `/runpr <PR>` again
   closes them (step 1's `MERGED` case); a close that failed after its comment
   was posted posts the comment once more on that rerun.
-- End by the answer in step 4 and how this step went. The next branch is
+- End by the answer in step 4 and how this step went. This command never runs
+  NEXT: it shows it. The next branch is
   **ready** once `_lib/task-pr-handoff.md` Part A step 4 has run to its end; a
   branch that exists on `origin` or locally is not ready by that alone. (An
   answer that does not merge stopped in step 4, so this step does not run.)
-  - NEXT is set, the next branch is ready, and the answer chose to run NEXT:
-    run it now. It opens its own PR, pings the reviewer, and stops; this
-    command ends there.
-  - NEXT is set, the next branch is ready, and the answer merged without
-    running NEXT: report the final status with the `/clear` guidance below.
+  - NEXT is set and the next branch is ready: report the final status with the
+    `/clear` guidance below.
   - NEXT is none (including a merge without the approvals): report the final
     status, without the `/clear` guidance.
   - NEXT is set, but this step stopped before the next branch was ready:
-    report as "Stopping before the next branch is ready" below says, whichever
-    Merge option the answer chose, and do not run NEXT.
+    report as "Stopping before the next branch is ready" below says.

   Every final report of this step, the stop report below included, is in
-  Japanese, and gives LIMITS_NOTE when there is one, or else the expiry date
-  step 1 kept for the 有効 state, on one line. The `/clear` guidance gives:
+  Japanese, gives the path of the details directory and, when the PR merged,
+  the path of the detail file of the round that decided the merge and its head
+  commit (step 4), and gives LIMITS_NOTE when there is one, or else the expiry
+  date step 1 kept for the 有効 state, on one line. The `/clear` guidance gives:
   1. that the user should run `/clear` and then NEXT;
   2. NEXT with its arguments, ready to run as written (for example
      `/runplan 0015`);
````

Diff record (AC-19):

- Compared paths: `.claude/commands/mergepr.md`
- Merge-base: `f72fd25d5b7971a3dc8598caa13ae9019c5b7454`
- Compared commit: `d413285efd2ba0288a440a4ca1bbad68cb67d4a0`

Path diff:

```text
diff --git a/.claude/commands/mergepr.md b/.claude/commands/mergepr.md
index dd42d15fe67f7f1dfcc7aaa77c5a27a7cc95babd..63906b2bbe5d2f3029b3f5ec1b813c3a88ba61d9 100644
--- a/.claude/commands/mergepr.md
+++ b/.claude/commands/mergepr.md
@@ -120,10 +120,11 @@ around it with your own shell commands. Work in order; do not skip a step.
    the subject's prefix (step 2), and ask whether to merge with this
    message. Revise it as asked; after revising the subject, check it again as
    step 2 does. Do not merge without an explicit yes. When this
-   command runs inside `runpr.md`, its step 4 asks
-   this question together with the pre-merge bookkeeping and whether to run
-   the next command `runpr.md` offers (when there is one), and says how to
-   re-prepare after the bookkeeping commit.
+   command runs inside `runpr.md`, its step 4 asks this question together with
+   the pre-merge bookkeeping, shows the subject in the conversation, and puts
+   the body and the record of the prefix rule in a detail file outside the work
+   tree; the question does not ask whether to run the next command. `runpr.md`
+   also says how to re-prepare after the bookkeeping commit.
 4. **Merge.** Run
    `mergepr merge --state <state-file> --subject-file <subject-file> --body-file <body-file>`.
    It requires the PR to be still open on the prepared base, then merges with
```

### AC-22 の記録

- 日付: 2026-11-12
- 確かめた人: isseis（OpenCode の pane で、本計画の手順どおりに行った）
- OpenCode の版: 人の答えに含まれなかったため記録しない。
- 見本の値: 設計書 7.2 の 3 つの表示（(1) 要約 11 行と選択肢 4 つ、(2) 要約 12 行と選択肢 3 つ、(3) deferred のスレッド 4 件の deferred の問い）。PR #859、CI 成功、詳細ディレクトリ `/tmp/runpr.Ab12Cd` などの見本の値を、OpenCode のエージェントへの指示（`/runplan` が用意）で与えた。
- 結果: 高さ 24 行の pane では、表示は収まらなかった。人が普段使っている環境（pane の高さは答えに含まれなかった）では十分に収まり、人はこの結果で受け入れた。3 つの表示ごとの結果（収まったか、選べたか、自由記述の欄があったか）は、人の答えが 1 つにまとめており、分けて記録できない。
- 所見: 設計書 9 章は、収まらなかったときに選択肢の description を除いて行を減らせるとする。人が普段の環境で収まるとして受け入れたので、直さずに PR を作る。24 行の pane で収めることは、この確かめの結果としては満たしていない。レビューで人が判断する。

## 8. 成功の基準 (Success Criteria)

-   設計書 2.2 の表のすべてのファイルに、設計書 3 章の変更がある。
-   `make test` と `make lint` が通る。
-   AC-18・AC-19 の差分の記録と AC-22 の記録が §7 にある。
-   セキュリティ：step 4 の節で、メンションの確認が投稿の命令より前にある（`TestPostingCommandsInventory`）。移した追跡 issue の手順が、メンションの確認と `fixprguard reply` と止まる条件を保っている（AC-18 の差分の記録を読んで確かめる）。
-   ADR-0047・ADR-0046 に、0089 の実装を待つ食い違いの記述が残っていない。

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/runplan 0089` でフェーズ 1 を実装し、PR を作る。
-   マージの後、0088 の実装（#837・#838）が、ADR-0046 のとおり 0088 の AC-30・AC-46 を計画の決定の変更として外す。

## 変更履歴

- 2026-11-12 承認（isseis、#864）
