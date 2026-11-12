# 実装計画書：/runpr のマージの質問を、画面に収まる 1 つの問いにする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-12 |
| Review date | - |
| Reviewer | - |
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

-   [ ] **ステップ 1-1**: `runpr.md` step 4 に項目 1（deferred の問い）を足し、今の項目 1〜4 を 2〜5 に繰り下げる（AC-01〜AC-05、AC-21。設計書 3.1・3.2・3.6）。
    -   尋ねる時点、対象、メンションの一覧を読み直すこと、問いの形と選択肢、要約、詳細ファイル、番号の答えの形、答えごとの動きを、設計書 3.1 のとおりに書く。番号の答えの形は、ここで 1 回だけ定める。
    -   詳細ディレクトリの作り方、詳細ファイルの名前、書けないときに止まること、読み戻さないことを、設計書 3.2 のとおりに書く。step 4 で最初の問いを尋ねる前に 1 回作り、その実行の間使う。
    -   今の「When the answer merges, first move the deferred threads not chosen to an issue」の手順（`runpr.md:929-961`）を項目 1 へ移し、設計書 3.6 の 4 点（準備の破棄をしない、追跡 issue を覚えておく、数え直す、止まったときの報告）だけを変える。数え直しの `fixprguard list` の呼び出しは、ステップ 1-2 の戻す手順の呼び出しと同じ形で書く。最初の投稿の命令より前に、その手順の `**Mention check.**` の段落を置く。投稿の命令の書き方（`gh issue comment <issue> --body-file`、`gh issue create `、`fixprguard reply `）は変えない。
-   [ ] **ステップ 1-2**: `runpr.md` step 4 の項目 2〜5 と、答えを受けた手順を書き換える（AC-06〜AC-18。設計書 3.2〜3.5・3.7）。
    -   項目 2：承認の行のない版の本文を、`mergepr prepare` の準備のディレクトリ（詳細ディレクトリではない）にファイルとして書く（設計書 3.2）。
    -   項目 3：詳細ファイル（設計書 3.2 の項目のすべて）、要約の 12 種類の行（設計書 3.3）、question の文と選択肢と自由記述の扱い（設計書 3.4）を書く。今のメッセージの段落（`runpr.md:729-766`）と、3 つの問いの段落（`:766-806`）を置き換える。
    -   答えを受けた手順：deferred のスレッドを直す答え（`:823-828`）は、`mergepr discard` を除いて項目 1 へ移す（設計書 3.5 の 1 点目）。NEXT を実行する選択肢（`:786-787`）は除く。除外スレッドを覆す答え（`:829-833`）は自由記述の答えとして残す。尋ね直すたびに新しい回の詳細ファイルと要約を書くことと、step 2・3 を経て step 4 に戻るときは項目 1 から始め、新しい deferred のスレッドがあれば先に deferred の問いを尋ねることを足す（設計書 2.1・3.5 の 3 点目、6.2）。
    -   戻す手順：項目 1 から入るときは準備を破棄しないことと、戻す手順 1 で `fixprguard list` を呼んで THREADS を未解決のものに限ることを足す（設計書 3.7）。足す `fixprguard list` の呼び出しは、step 3 の呼び出し（`runpr.md:626`）と同じ形で書く。つまり、コードブロックの中の 1 行とし、引数も同じ（`--protocol 6`、`--reviewer <reviewer>`）にする。移した手順の数え直し（ステップ 1-1）の呼び出しも同じ形にする。戻す手順 7 の後は項目 1 から始め直す。手順 2〜6 の文は変えない。
    -   項目 4：PRE_MERGE のときの写し先を、スクラッチパッドから詳細ディレクトリの `approved-<回>.subject`・`approved-<回>.body` に変える（設計書 3.2）。
    -   step 4 の冒頭の「Ask the user once」の段落に、deferred のスレッドがあれば、マージの質問の前に deferred の問いを尋ねることを足す（設計書 3.5）。
    -   マージしたときの報告と止まるときの報告に、詳細ディレクトリと詳細ファイルのパスを書くことを足す（設計書 3.2「報告」）。
-   [ ] **ステップ 1-3**: `runpr.md` の step 1 と step 3 の記述を合わせる（AC-08、AC-21。設計書 3.1・3.3）。
    -   step 1 の LIMITS_NOTE（`:232-234`）の「the message of step 4's question」を、step 4 の各問いの要約と詳細ファイルに出す形にする。
    -   step 2 の LAST_REVIEWED がないときの扱い（`:563` の「step 4's PR-review question」）を、マージの質問の「先に PR レビューを依頼」の選択肢の条件として述べる形にする（設計書 3.3 の 4 行目）。
    -   step 3 の `ask` の持ち越し（`:669`）を、マージの質問の「先に PR レビューを依頼」の選択肢と詳細ファイルへ持ち越す形にする。
    -   step 3 の deferred のスレッドを残す 2 か所（`:672`、`:691`）を、step 4 の deferred の問いのために残す、と書き直す。
-   [ ] **ステップ 1-4**: `runpr.md` の step 5、description、導入の段落、Authorization を書き換え、テストの定数を合わせる（AC-16、AC-20、AC-21。設計書 3.8）。
    -   step 5 の見出しを `## 5. Switch branch, then show NEXT` にし、NEXT を実行する分岐（`:1039-1041`）を除く。「do not run NEXT」の句（`:1048`）も除く。最終の報告に、マージを決めた回の詳細ファイルのパスを書くことを足す。
    -   description（`:2`）、導入の段落（`:48-50`）、Authorization の NEXT の句（`:90-91`）と追跡 issue の句（`:82-83`）を、設計書 3.8 のとおりに書き直す。
    -   `cmd/yt2column/codex_review_docs_test.go:84` の `runprStep5` を新しい見出しにする。`make fmt` を実行する。
-   [ ] **ステップ 1-5**: `mergepr.md` step 3 の「When this command runs inside `runpr.md`」で始まる文（`:122-126`）だけを書き直す（AC-19。設計書 3.9）。同じ step のほかの文は変えない。
-   [ ] **ステップ 1-6**: ほかのコマンドの定義と文書を書き換える（AC-05、AC-05a、AC-15、AC-20、AC-21。設計書 3.8・3.9）。
    -   `fixpr.md:144-146` と `:167-168`。
    -   `_lib/simple-process.md:334-339`（除外スレッド・「fix all」・マージの質問の選択肢）、`:364-367`（戻す手順を呼ぶ箇所に項目 1 を足す）、`:380-384`（THREADS を、`fixprguard list` で未解決のものに限る）。
    -   `_lib/task-pr-handoff.md:27-32`、`CLAUDE.md:207` と `:225-230`、`requirements_process.md:89`。
    -   `requirements_process.md:572-575` に、deferred の問いの答えでも戻せることを書き足す（§1.3 の決定 4、設計書 3.9）。
-   [ ] **ステップ 1-7**: ADR の食い違いの記述を直す（設計書 3.10）。直す前に、`git grep -n "0089" HEAD -- docs/dev/adr/` を実行し、直す記述を確かめる。経緯の行（ADR-0040・ADR-0046・ADR-0047 の 2026-11-12 の行）とリンクは変えない。
    -   ADR-0047 の「決定」の最後の段落（「この決定は、まだ…反映していない。…解消する。」）を消す。「決定」の冒頭の「実装の後は `runpr.md` と `mergepr.md` の step 3 が持つ」は、実装の後の状態を述べる形（`runpr.md` と `mergepr.md` の step 3 が持つ）に書き直す（同じ食い違いの記述の一部として扱う）。
    -   ADR-0046 の「決定」の食い違いの記述（`docs/dev/adr/0046-chief-of-staff.md:41`）から、0089 の実装を待つ部分（「0089 の実装の後は」「0089 の実装より前は、この答えを中継しない」）を除き、残る食い違い（0088 の実装を待つこと、AC-30・AC-46 が対象を失ったこと）を述べる形に書き直す。
    -   両方とも訂正の行をコミットのメッセージに書き、PR の説明に訂正の段落を書く（§1.2。PR の説明は `/runplan` が `_lib/task-pr-handoff.md` Part B で書く）。直した後の ADR が、それぞれの「決定」と食い違わないことを読んで確かめる。
-   [ ] **ステップ 1-8**: `make test` と `make lint` を通す。ステップ 1-1〜1-7 の文書の変更によってガードのテストが落ちたときは、テストを変えずに文書の書き方を直す（設計書 7.1。テストの変更は `runprStep5` だけ）。
-   [ ] **ステップ 1-9**: AC-18 と AC-19 の差分の記録を、§7 の「差分の記録」に書く（§1.3 の決定 2）。ステップ 1-8 の後で、`runpr.md` か `mergepr.md` を変えた最後のコミットを比べるコミットにして、`requirements_process.md` §4「Fixed commands」のパスの差分のコマンドを実行し、「Diff record form」の形で記録する。記録は本計画だけを変える別のコミットにする。
-   [ ] **ステップ 1-10**: AC-22 の手動の確かめを行う（§1.3 の決定 3。設計書 1.2・7.2）。
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

-   [ ] フェーズ 1 完了（ステップ 1-1〜1-10）
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

Diff record (AC-19):

- Compared paths: `.claude/commands/mergepr.md`

### AC-22 の記録

ステップ 1-10 で書く。

## 8. 成功の基準 (Success Criteria)

-   設計書 2.2 の表のすべてのファイルに、設計書 3 章の変更がある。
-   `make test` と `make lint` が通る。
-   AC-18・AC-19 の差分の記録と AC-22 の記録が §7 にある。
-   セキュリティ：step 4 の節で、メンションの確認が投稿の命令より前にある（`TestPostingCommandsInventory`）。移した追跡 issue の手順が、メンションの確認と `fixprguard reply` と止まる条件を保っている（AC-18 の差分の記録を読んで確かめる）。
-   ADR-0047・ADR-0046 に、0089 の実装を待つ食い違いの記述が残っていない。

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/runplan 0089` でフェーズ 1 を実装し、PR を作る。
-   マージの後、0088 の実装（#837・#838）が、ADR-0046 のとおり 0088 の AC-30・AC-46 を計画の決定の変更として外す。
