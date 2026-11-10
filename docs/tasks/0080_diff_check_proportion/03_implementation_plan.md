# 実装計画書：`diff` の確かめを、確かめる変更の大きさに見合ったものにする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-10 |
| Review date | 2026-11-10 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。用語（規約・記録した差分の読み取り・差分の記録・記録の再確認・記録し直し・決まった形のコマンド・チェック・壊し方・PR-1・PR-2）は、要件書 3 章と 6 章、設計書の冒頭と同じ意味で使う（PR-1・PR-2 は設計書の冒頭が定める）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

規約・コマンドの定義・`CLAUDE.md`・ADR の文言を変える。これにより、`diff` の確かめの作りを、確かめる変更の大きさに見合ったものにする（要件書 2.1 節）。何をどこに書くかは設計書 3 章が定める。本計画は、その作業の順序と、確かめ方を定める。

### 1.2. 実装の原則

-   **文面は設計書 3 章に従い、英語で書く。** 規約とコマンドの定義は英語の文書である（要件書 5 章）。各ステップは、書く内容を設計書の節で指す。文面は実装の PR で書く。
-   **新しい恒久のテストと道具は足さない**（設計書 1.1・7.1）。前後の関係を述べる AC（AC-10・AC-13）は、本タスクの `_checks/` のシェルスクリプトで確かめる。本計画は新しい規則がマージされる前に承認されるので、今の規約に従う（設計書 7.2、F-007）。
-   **足す文は、文書を読む既存の guard の制約を守る**（設計書 3.12）。
-   **ADR の食い違いの記述を消すことは、編集上の訂正である**（`docs/dev/conventions/adr.md`「Editorial corrections」の 2 つ目の種類）。消すコミットのメッセージに訂正の行（何を何に変え、決定は変わらないことを述べる行）を書き、PR の説明に訂正の段落を書く（同「Recording」。形は `requirements_process.md`「Recording in the PR description」）。

### 1.3. 計画の際に決めたこと（2026-11-10）

1.  **規約の §4 に置く決まった形のコマンドは、PR-1 で 1 回だけ実行し、出力を本計画 §7 に記録して確かめる（設計書 7.1 が計画に委ねた点）。** 実行は 2 通りで行う。1 回目は素の設定で、2 回目は出力の形を変える git の設定を `git -c` で既定と逆にして行い、2 つの出力が同じであることを確かめる。設定の違いが出力に現れる入力を、使い捨てのコミットの組として作る。記録の再確認の比べ方も 1 回通して実行する（ステップ 1-8）。理由：コマンドの例は実行して確かめる必要がある（`CLAUDE.md`「Testing Strategy」）。一方で、設計書 1.1 は Go のテストを足さないと決めている。
    -   採らなかった案：Go の guard で §4 のコマンドを取り出し、一時的なリポジトリで実行する案。設計書 1.1 と食い違う。また、コマンドの誤りは最初に実行した時点で表に出るので、恒久のテストの保守に見合わない。
    -   採らなかった案：実行せず、PR のレビューで読むだけにする案。git の設定に左右されないことは、読んでも確かめられない。
    -   人の答え：1 回実行して記録する案を選んだ。
    -   ADR：なし。本タスクの中で閉じる、確かめ方の選択である。
2.  **AC-13 は、AC-10 と同じ `_checks/` のスクリプトで確かめる。** スクリプトは、`docs/dev/adr/` の差分が issue #789 にリンクする行の削除だけであることを確かめる。PR-2 の実行では、さらに head の `docs/dev/adr/` に issue #789 へのリンクが残らないことも確かめる。理由：設計書の付録 B は AC-13 を `manual` としたが、記述を消すことは前後の関係なので、`diff` の確かめにできる。AC-10 のスクリプトに 1 つチェックを足すだけで済む。
    -   採らなかった案：設計書の付録 B のとおり `manual` だけにする案。
    -   人の答え：AC-10 のスクリプトに含める案を選んだ。
    -   ADR：なし。本タスクの中で閉じる、確かめ方の選択である。
3.  **PR の境界は、設計書 1.2 の決定 2 のとおり、ADR ごとの 2 本とする。** フェーズ 1 が PR-1、フェーズ 2 が PR-2 である（§3.2）。ADR：なし（設計書 1.2 の決定 2）。

### 1.4. 既存コード調査結果

コミット `bf7db6593f8f9b5137f97c1237112fdc4435e81e`（`origin/main`）で確かめた。

-   **設計書の行番号は今も正しい。** 設計書はコミット `df655ec6` の行番号を使う。変えるファイル（規約、`.claude/commands/`、`CLAUDE.md`、ADR-0016、ADR-0035）は、`df655ec6` から上記のコミット（`bf7db659`）まで変わっていない（`git diff --stat df655ec6 HEAD -- <それらのパス>` の出力は空）。主な箇所も読んで確かめた（`mkplan.md:98`・`:101`・`:104`・`:111`・`:115`・`:132`・`:133`・`:149`・`:152`・`:159`、`runplan.md:52`・`:65`・`:81`・`:85`・`:162`・`:174`・`:180`・`:233`、`runpr.md:304`・`:325`・`:696`、`weakreview.md:134`・`:295`、`CLAUDE.md:604`・`:713`、`mkreq.md:149-152`）。
-   **消す食い違いの記述。** `git grep -nE 'issues/789([^0-9]|$)' -- docs/dev/adr/` の出力は次の 2 行である。どちらも 1 行の箇条である。
    -   `docs/dev/adr/0016-document-test-policy.md:29`（「決定」の最後の箇条。PR-1 で消す）
    -   `docs/dev/adr/0035-language-policy.md:25`（「決定」の #789 の箇条。PR-2 で消す。同じ「決定」の #726 の記述は残す）
-   **既存のテスト。** `TestADRRecords` は `cmd/yt2column/adr_test.go:434` にある。変える文書を読むほかの guard と、足す文への制約は、設計書 3.12 のとおりである。直す guard はない。
-   **決まった形のコマンドに使う git の引数。** git 2.50.1 で、`--no-ext-diff`・`--no-textconv`・`--no-color`・`--default-prefix`・`-U3`・`--diff-algorithm=myers`・`--no-renames` が受け付けられ、出力が得られることを確かめた（`bf7db659` とその親で実行）。どの引数を使うかは、ステップ 1-1 で §4 に書くときに決める。
-   **スクリプトの実行環境。** 開発環境の `/bin/sh` と `git` で動く POSIX のシェルスクリプトにする。`_checks/` は `make test` と CI で実行されない（`requirements_process.md:790`）。
-   製品のコード（`cmd/`・`internal/`・`extension/`）は変えない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: `diff` の確かめ（設計書 8 章の PR-1）

**変更するファイル:** `docs/dev/conventions/requirements_process.md`、`.claude/commands/mkplan.md`、`.claude/commands/runplan.md`、`.claude/commands/runpr.md`、`.claude/commands/weakreview.md`、`.claude/commands/mkreq.md`、`CLAUDE.md`、`docs/dev/adr/0016-document-test-policy.md`、`docs/tasks/0080_diff_check_proportion/_checks/scope.sh`（新規）、本計画

-   [x] **ステップ 1-1**: 規約を書き換える（AC-01〜AC-05）。
    -   §1 の AC の要件の箇条に、前後の関係を述べる AC の書き方の規則を 1 つの箇条として足す（設計書 3.1）。
    -   §4 の `diff` の確かめの箇条を、小見出しと項目ごとの箇条に書き換える（設計書 3.2 の表の全項目）。
    -   §4 に、差分の記録の形と、決まった形の 2 つのコマンド（パスの差分、ファイルの一覧）を置く。記録のコードブロックの中を計画の印として読まないことも書く（設計書 3.3）。コマンドの引数は、設計書 3.3 が挙げる設定に加えて、出力を変えるほかの git の設定（省略した blob の ID の長さ、ファイルの並び順の指定、近い hunk の結合など）にも左右されないように選ぶ。hunk の見出しの関数名を決める属性（`.gitattributes`・`info/attributes`・`core.attributesFile`）は、`git diff` の引数では固定できない。属性はリポジトリと作業コピーのファイルから来て、記録の再確認は同じリポジトリで行うので、§4 には固定できない限りとして書く。
-   [x] **ステップ 1-2**: `mkplan.md` の PR-1 の変更をする（AC-01・AC-02・AC-04〜AC-06・AC-08。設計書 3.4 の「PR-1」の全項目）。過大な確かめの項目は、今の項目と同じ `- [ ]` の 1 行にし、観点を項目の中に書く（AC-09）。
-   [x] **ステップ 1-3**: `runplan.md` の PR-1 の変更をする（AC-03・AC-04・AC-08。設計書 3.5 の「PR-1」の全項目）。
-   [x] **ステップ 1-4**: `runpr.md` の PRE_MERGE と step 4 を変える（AC-03。設計書 3.6）。1 つの AC の行を複数の PR が確かめるとき（本計画の AC-10・AC-13 のように）も、PRE_MERGE が今の PR の記録の有無を確かめられる書き方にする。本計画は、記録を PR ごとの見出し（§7）に分けて置く。`runpr.md` の見出しを変えず、`reviewThreads`・`comments(first:`・自動レビュアーの名前・メンションを書かない（設計書 3.12）。
-   [x] **ステップ 1-5**: 残りのコマンドの定義と `CLAUDE.md` を変える（AC-04・AC-05・AC-08）。
    -   `weakreview.md` の「Weak-model failure checklist」に、過大な確かめの項目を足す（設計書 3.7 の PR-1）。
    -   `mkreq.md` の step 6 の §1 の名指しに、AC の書き方の規則を足す（設計書 3.9）。
    -   `CLAUDE.md`「Testing Strategy」の `diff` の確かめの文を書き換える（設計書 3.8.1）。
-   [x] **ステップ 1-6**: ADR-0016 の「決定」の最後の箇条（食い違いの記述、§1.4）を消す（AC-13。設計書 3.11）。消す前に、`git fetch origin main` を実行してから `git grep -nE 'issues/789([^0-9]|$)' origin/main -- docs/dev/adr/` を実行し、消す記述を確かめる。コミットのメッセージに訂正の行を書き、PR の説明に訂正の段落を書く（§1.2。PR の説明は `/runplan` が `_lib/task-pr-handoff.md` Part B で書く）。消した後の「結果」が今の決定と食い違わないことを読んで確かめる（AC-11）。
-   [x] **ステップ 1-7**: 確認のスクリプト `_checks/scope.sh` を書く（AC-10・AC-13、§1.3 の決定 2）。マージの基点と比べるコミットを引数で受け取り、次のチェックを行う。
    -   2 つのコミットの間で変わったファイルが、AC-10 が挙げるファイルに限られる。具体的には、規約、`mkplan.md`、`runplan.md`、`runpr.md`、`weakreview.md`、`mkreq.md`、`CLAUDE.md`、ADR-0016、ADR-0035、本タスクのディレクトリの下である。`fixpr.md` と guard のファイルは、設計書 1.2 の決定 1 と 3.12 により含めない。ファイルの一覧は、規約の §4 の決まった形のファイルの一覧のコマンドで出す（改名の検出を切り、移した元のパスも一覧に出すため）。
    -   `docs/dev/adr/` の差分が、issue #789 にリンクする行の削除だけである。
    -   PR-1 の実行では、head の ADR-0016 に issue #789 にリンクする行がなく、ADR-0035 が変わらない。PR-2 の実行では、head の `docs/dev/adr/` に issue #789 にリンクする行が残らない。どちらの実行かは、引数で選ぶ。

    違反があれば、スクリプトは該当するチェックを名指しして 0 でない終了コードで終わる。2 つのコミットの完全な ID も出力する。各チェックが少なくとも 1 つの壊し方で失敗することを、使い捨てのブランチで確かめ、結果をコミットのメッセージに書く。ファイルの一覧のチェックの壊し方には、ほかのタスクのディレクトリから本タスクのディレクトリへの移動を含める。ステップ 1-8 か後のレビューで規約の §4 のファイルの一覧のコマンドを直したときは、スクリプトも合わせて直し、壊し方の確かめをやり直してからステップ 1-9 を行う。`make test` のために guard を直す必要が出たときは、許すファイルを広げる前に、計画の決定の変更として人と決める。コメントと文字列リテラルは英語で書く（今の `CLAUDE.md`「Go Idioms」と「Language」）。
-   [x] **ステップ 1-8**: 規約の §4 に書いた決まった形の 2 つのコマンドを、PR のブランチで実行して確かめる（§1.3 の決定 1）。入力には、設定の違いが出力に現れる、使い捨てのコミットの組を、使い捨てのブランチか worktree に作る（改名、引用が要る非 ASCII のパス、近い 2 つの hunk、複数のファイルを含む。push せず、PR のブランチの HEAD は変えない）。§7 には、コミットの組を作ったコマンドも記録する。素の設定と、`git -c` でステップ 1-1 が挙げた設定をそれぞれ既定と逆にした設定の 2 通りで実行し、2 つの出力が同じであることを確かめる。§4 の固定の引数を外すと、逆にした設定で出力が変わることも確かめる。さらに、記録の再確認の比べ方（記録のコードブロックの中身を取り出し、行末の空白を落として比べる。設計書 6.1）を 1 回通して実行する。使い捨てのブランチで、pre-commit のフックを経てコミットした §4 の形の記録が「同じ」になり、比べたパスを変えると「違う」になることを確かめる。実行したコマンド、git の版、2 つのコミットの ID、出力、比べた結果を §7 の「決まった形のコマンドの実行の記録」に記録する。違えば、§4 のコマンドを直してやり直す。この記録はコマンドの例の確かめであり、どの AC の行の差分の記録でもない（本計画に、記録した差分の読み取りを使う AC の行はない）。そのため、§4 の差分の記録の形では書かない。記録の再確認の対象にならないことも §7 に書く。出力のコードブロックの囲みは、出力の中のどの連続したバッククォートよりも長くする（設計書 3.3）。
-   [x] **ステップ 1-9**: AC-10・AC-13 の確かめとして、`_checks/scope.sh` を、マージの基点と PR-1 の最後の実装のコミットを引数にして実行する。出力・終了コード・2 つのコミットの ID・スクリプトの blob の ID を §7 の「PR-1 の実行の記録」に書く。`make test` と `make lint` を通す。

**確かめ直し。** ステップ 1-9 の比べたコミットより後に、本タスクのディレクトリの外のファイルか `_checks/scope.sh` を変えるコミットが入ったら、ステップ 1-9 をやり直して §7 の記録を改める。PR を作る前のレビューの修正にも、PR を作った後の `/fixpr` の修正や手で入れたコミットにも当てはめる（PRE_MERGE はスクリプトの記録の有無だけを確かめ、スクリプトを実行し直さないため。設計書 3.6）。規約の §4 のコマンドが変わったときは、ステップ 1-8 もやり直す。記録をコミットするコミットは、本計画だけを変える。

**完了の条件:** 設計書 3.1〜3.9 と 3.11 のうち PR-1 の項目がすべて文書にある。§7 にステップ 1-8 と 1-9 の記録がある。`make test` と `make lint` が通る。

### PR-1 作成ポイント: diff verification forms and record procedure

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9

**推奨タイトル**: `docs(0080): diff の確かめに記録した差分の読み取りと作りの基準を加える`

**レビュー観点**: 規約 §4 の決まった形のコマンドが git の設定に左右されないこと（§7 の記録） / runpr.md の PRE_MERGE の記録の再確認と、止まる条件 / mkplan.md の変異の規則と diff のスクリプトの壊し方の規則の使い分け / AC-10・AC-13 のスクリプトの記録

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-3・1-4 が `/runplan` と `/runpr` の手順に新しい停止の条件と記録の再確認を足し、手順の分岐を変えるため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `_checks/` の日本語のリテラル（設計書 8 章の PR-2）

**変更するファイル:** `CLAUDE.md`、`.claude/commands/mkplan.md`、`.claude/commands/runplan.md`、`.claude/commands/weakreview.md`、`docs/dev/adr/0035-language-policy.md`、本計画

-   [ ] **ステップ 2-1**: `CLAUDE.md`「Go Idioms」の guard-test literal exception に、`_checks/` の範囲・リテラル・境界・理由を足す（AC-07。設計書 3.8.2）。
-   [ ] **ステップ 2-2**: 英語だけを求めるコマンドの定義の規則を、「Go Idioms」の例外に当たるリテラルを指摘しない形にする（AC-07）。例外の一覧は写さず、「Go Idioms」を参照する（設計書 1.2）。
    -   `mkplan.md` の 4 か所（設計書 3.4 の「PR-2」）。
    -   `runplan.md` の step 6.5 の Check 2 の扱いの説明（設計書 3.5 の「PR-2」）。
    -   `weakreview.md` の言語の項目（設計書 3.7 の PR-2）。
-   [ ] **ステップ 2-3**: ADR-0035 の「決定」の #789 の箇条（§1.4）を消す（AC-13。設計書 3.11）。#726 の記述は残す。消す前の確かめ、訂正の行、PR の説明の訂正の段落は、ステップ 1-6 と同じである。
-   [ ] **ステップ 2-4**: AC-10・AC-13 の確かめとして、PR-2 の実行で `_checks/scope.sh` を、マージの基点と PR-2 の最後の実装のコミットを引数にして実行する。記録の中身はステップ 1-9 と同じで、§7 の「PR-2 の実行の記録」に書く。`make test` と `make lint` を通す。

**確かめ直し。** フェーズ 1 の「確かめ直し」と同じ条件で、ステップ 2-4 をやり直す。

**完了の条件:** 設計書 3.4・3.5・3.7・3.8.2・3.11 のうち PR-2 の項目がすべて文書にある。§7 にステップ 2-4 の記録がある。`make test` と `make lint` が通る。

### PR-2 作成ポイント: Japanese literals in task checks

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `docs(0080): _checks のスクリプトに日本語のリテラルの例外を広げる`

**レビュー観点**: Go Idioms の例外の範囲と境界（コメントと識別子は英語のまま） / 各コマンドが例外の一覧を写さず Go Idioms を参照していること / ADR-0035 の #726 の記述が残っていること

**実装モデル要件**: standard

**判定理由**: 設計書が文面の置き場所を決めた文言の変更だけで、どの判定の条件にも当たらないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1、PR-1） | 規約の §1・§4、`mkplan.md`・`runplan.md`・`runpr.md`・`weakreview.md`・`mkreq.md`・`CLAUDE.md`「Testing Strategy」の変更、ADR-0016 の食い違いの記述の削除、`_checks/scope.sh`、§7 の記録 |
| M2（フェーズ 2、PR-2） | `CLAUDE.md`「Go Idioms」、`mkplan.md`・`runplan.md`・`weakreview.md` の英語だけの規則の変更、ADR-0035 の #789 の記述の削除、§7 の記録 |

PR-1 を先にする（設計書 8 章）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 | `diff` の確かめの 2 つの形、作りの基準、記録と記録の再確認の手順、過大な確かめの項目、ADR-0016 の食い違いの記述の削除 | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 | `_checks/` の日本語のリテラルの例外、英語だけの規則の参照、ADR-0035 の #789 の記述の削除 | standard |

どちらの PR も 1 つの ADR の食い違いを一度に解く。そのため、記述を途中の形に書き換える手間がない（設計書 1.2 の決定 2）。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** 足さない（設計書 7.1）。各 PR で `make test` と `make lint` を通す。設計書 3.12 の guard と `TestADRRecords` は、`make test` と CI の `doc-guard` のジョブに含まれる。
-   **前後の関係の確かめ:** `_checks/scope.sh` を PR ごとに 1 回実行する（AC-10・AC-13）。`make test` にも CI にも組み込まない。
-   **コマンドの例の確かめ:** 規約の §4 の決まった形のコマンドを、ステップ 1-8 で 1 回実行して記録する（§1.3 の決定 1）。
-   **読んで確かめる（manual）:** AC-01〜AC-09 と AC-11 の文の内容は、PR のレビューで設計書 3 章の該当する節と照合しながら読む（設計書 7.3）。
-   **結合テスト・後方互換のテスト:** N/A。実行時の振る舞いを変えない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 足した文が、文書を読む guard の制約に触れる | 設計書 3.12 の制約をステップ 1-4 に書き、各 PR で `make test` を通す |
| 決まった形のコマンドが git の設定で出力を変える | ステップ 1-8 で、設定を変えた 2 回の出力を比べる |
| PR-1 のマージの後、PR-2 を新しい `/runpr` の PRE_MERGE に通すと、スクリプトの記録がない場合に止まる | ステップ 2-4 で、PR-2 の記録を §7 に PR-1 の記録と分けて書く。ステップ 1-4 で、1 つの AC の行を複数の PR が確かめる場合も扱える書き方にする。PR-1 の `/runpr` は PR のブランチの新しい `runpr.md` で動くが、§7 はスクリプトの記録を持つので、今の規則と新しい規則のどちらでも止まらない |
| 食い違いの記述を、一部だけ消すか、別の記述まで消す | `_checks/scope.sh` が、ADR の差分が #789 にリンクする行の削除だけであることを確かめる |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR-1 のレビューで、規約の §4 と `mkplan.md` を設計書 3.2・3.4 と照合して読む |
| AC-02 | `manual` | PR-1 のレビューで、規約の §4 と `mkplan.md` を設計書 3.2〜3.4 と照合して読む |
| AC-03 | `manual` | PR-1 のレビューで、規約の §4・`runplan.md`・`runpr.md` を設計書 3.2・3.3・3.5・3.6・6.1・6.2 と照合して読む。決まった形のコマンドの例は、下の「決まった形のコマンドの実行の記録」で確かめる（ステップ 1-8） |
| AC-04 | `manual` | PR-1 のレビューで、設計書 3.2・3.4・3.5・3.8.1 の変える箇所の一覧と照合し、`diff` の確かめをスクリプトだけとして述べる文が残っていないことを読んで確かめる（設計書 7.3） |
| AC-05 | `manual` | PR-1 のレビューで、規約の §1・§4、`mkplan.md`、`CLAUDE.md`「Testing Strategy」、`mkreq.md` を設計書 3.1・3.2・3.4・3.8.1・3.9 と照合して読む |
| AC-06 | `manual` | PR-1 のレビューで、`mkplan.md` を設計書 3.4 と照合して読む |
| AC-07 | `manual` | PR-2 のレビューで、`CLAUDE.md`「Go Idioms」と `mkplan.md`・`runplan.md`・`weakreview.md` を設計書 3.4・3.5・3.7・3.8.2 と照合して読む |
| AC-08 | `manual` | PR-1 のレビューで、3 つのチェックリストの項目を設計書 3.4・3.5・3.7 と照合して読む |
| AC-09 | `manual` | PR-1 のレビューで、`mkplan.md` の足した項目が「Technical correctness checklist」の中の `- [ ]` の 1 行であることを読んで確かめる（設計書 3.10）。`_lib/simple-process.md:198-204`（コミット `bf7db659`）がこのチェックリストをそのまま写して渡す。`_lib/simple-process.md` が変わらないことは `_checks/scope.sh` の AC-10 のチェックが示す |
| AC-10 | `diff` | `docs/tasks/0080_diff_check_proportion/_checks/scope.sh`（ステップ 1-7）。PR-1 ではステップ 1-9、PR-2 ではステップ 2-4 が実行し、それぞれ下の「PR-1 の実行の記録」「PR-2 の実行の記録」に記録する |
| AC-11 | `static`・`manual` | `cmd/yt2column/adr_test.go::TestADRRecords`（`make test`）。PR-1 のレビューで、ADR-0016 の「結果」が今の決定と食い違わないことを読む |
| AC-12 | `static` | `cmd/yt2column/adr_test.go::TestADRRecords`（`make test`）。内容は要件書の PR（#791）で入った |
| AC-13 | `diff` | `_checks/scope.sh`（§1.3 の決定 2）。AC-10 と同じく、PR-1 ではステップ 1-9、PR-2 ではステップ 2-4 の記録。PR-2 の記録が、head の `docs/dev/adr/` に issue #789 へのリンクが残らないことを示す。要件書の PR の時点の部分（両 ADR の食い違いの記述と、issue #789 の本文が両 ADR を名指すこと）は、#791 で入った |

どの AC も、記録した差分の読み取りを使わない。`diff` の行（AC-10・AC-13）は、どちらの PR でも対象である（PR-2 ではステップ 2-4 が確かめる）。どちらの PR でもスクリプトの記録を持つ。

決まった形のコマンドの実行の記録（ステップ 1-8。コマンドの例の確かめで、AC の行の差分の記録ではない）:

-   この記録はコマンドの例の確かめで、AC の行の差分の記録ではない。§4 の差分の記録の形で書かず、記録の再確認の対象にもならない。
-   実行した環境：`git version 2.50.1 (Apple Git-155)`。PR のブランチのコミット `6c9f888351c4845be54b7b855a335c853c7f3d19` から、使い捨ての worktree（detached HEAD。push しない）にコミットの組を作った。§4 のコマンドは、レビューの修正（`--text` と `env -u GIT_DIFF_OPTS` を足した）の後の形で実行した。
-   コミットの組を作ったコマンド（worktree の中で実行した）:

    ```sh
    mkdir -p t
    awk 'BEGIN { for (i = 1; i <= 30; i++) print (i % 5 == 0 ? "" : "line " i) }' > t/a.md
    printf 'b one\nb two\nb three\n' > t/b.md
    printf 'こんにちは\n世界\n' > t/日本語.md
    printf 'old one\nold two\n' > t/old.md
    printf '{\n  foo\n}\n{\n  bar\n}\n' > t/c.md
    printf 'a\n  a\nb\n}\n\n}\n  b\n' > t/d.md
    git add t && git commit -q --no-verify -m "throwaway base"
    git mv t/old.md t/new.md
    sed -i '' -e 's/^line 7$/line 7 changed/' -e 's/^line 17$/line 17 changed/' t/a.md
    printf 'b one\nb two changed\nb three\n' > t/b.md
    printf 'こんにちは\n世界!\n' > t/日本語.md
    printf '{\n  bar\n}\n{\n  foo\n}\n' > t/c.md
    printf 'a\n\n  b\nb\na\n  a\nb\n}\n\n}\n  b\n' > t/d.md
    git commit -q --no-verify -am "throwaway change"
    ```

    改名（`t/old.md` → `t/new.md`）、引用が要る非 ASCII のパス（`t/日本語.md`）、7 行目と 17 行目を変えた近い 2 つの hunk（`t/a.md`）、差分のアルゴリズムで出力が変わる変更（`t/c.md`）、indent heuristic で出力が変わる変更（`t/d.md`）、複数のファイルを含む。
-   2 つのコミット：`<base>` = `06d4d60087c08e178a41889db7df5baa2abb626a`、`<commit>` = `44bfc763608d004b9bad54d3eda9fb365a0a3889`。比べたパスは `t/a.md`・`t/b.md`・`t/c.md`・`t/d.md`・`t/日本語.md`・`t/old.md`・`t/new.md`。
-   既定と逆にした設定（2 回目の実行で与えた）：環境変数 `GIT_DIFF_OPTS=--unified=0` と、`git -c` の `color.ui=always`・`color.diff=always`・`diff.noprefix=true`・`diff.mnemonicPrefix=true`・`diff.srcPrefix=SRC/`・`diff.dstPrefix=DST/`・`diff.context=10`・`diff.interHunkContext=10`・`diff.algorithm=histogram`・`core.abbrev=12`・`diff.orderFile=<t/new.md と t/b.md を先に並べるファイル>`・`core.quotePath=false`・`diff.renames=true`・`diff.external=<引数を出すだけのスクリプト>`・`diff.relative=true`・`diff.suppressBlankEmpty=true`・`diff.indentHeuristic=false`・`core.attributesFile=<t/b.md diff=up と t/a.md -diff>`・`diff.up.textconv=<英字を大文字にするスクリプト>`。
-   比べた結果：
    -   パスの差分のコマンドの出力は、素の設定と逆にした設定で同じだった（`cmp` の終了コード 0）。ファイルの一覧のコマンドも同じだった（同 0）。
    -   逆にした設定のまま、§4 の固定の部分を 1 つずつ外して実行した。パスの差分のコマンドでは、`--no-color`・`--no-ext-diff`・`--no-textconv`・`--text`・`--default-prefix`・`--full-index`・`-U3`・`--inter-hunk-context=0`・`--diff-algorithm=myers`・`--indent-heuristic`・`--no-renames`・`-O/dev/null`・`-c core.quotePath=true`・`-c diff.suppressBlankEmpty=false`・`env -u GIT_DIFF_OPTS` のどれを外しても、出力が素の設定のものと違った。`--no-relative` を外しても同じだった（作業ツリーの最上位で実行したため。`diff.relative` は下のディレクトリで実行したときに効く）。ファイルの一覧のコマンドでは、`--no-renames`・`-O/dev/null`・`-c core.quotePath=true` を外すと違い、`--no-ext-diff` と `--no-relative` を外しても同じだった（`--name-only` は外部の差分のプログラムを呼ばず、`--no-relative` は上と同じ理由による）。
    -   記録の再確認の比べ方：上の出力を §4 の差分の記録の形でファイル（`t/record.md`）に書き、pre-commit のフックを経てコミットした（フックが、空のコンテキストの行の行末の空白 4 行を消した）。§4「Rechecking a record」の 3〜4 のとおり、コードブロックの中身を `awk` で囲みの行を除いて取り出し、両方の行末の空白を落として `cmp` で比べると、終了コードは 0（同じ）だった。続けて比べたパス `t/a.md` に 1 行足してコミットし、HEAD と比べ直すと、終了コードは 1（違う）だった。
-   パスの差分のコマンドの出力（素の設定。逆にした設定でも同じ）:

    ````text
    diff --git a/t/a.md b/t/a.md
    index 8cd36f5ec454e8366b8ecd49b92142894afb1d05..caed7a715362c02286c84b616de121ed91086779 100644
    --- a/t/a.md
    +++ b/t/a.md
    @@ -4,7 +4,7 @@ line 3
     line 4

     line 6
    -line 7
    +line 7 changed
     line 8
     line 9

    @@ -14,7 +14,7 @@ line 13
     line 14

     line 16
    -line 17
    +line 17 changed
     line 18
     line 19

    diff --git a/t/b.md b/t/b.md
    index 442b8db476daa867dc7f3c33e635f4bd2c054511..81c2880205ad75598bdc21c3d88638f6b3c4fa7c 100644
    --- a/t/b.md
    +++ b/t/b.md
    @@ -1,3 +1,3 @@
     b one
    -b two
    +b two changed
     b three
    diff --git a/t/c.md b/t/c.md
    index 66b501b6de03e643fed9649ba933e19dbab4a56d..9bc351d1f2fb45fc4120af0a6e9139a0ad0c4aad 100644
    --- a/t/c.md
    +++ b/t/c.md
    @@ -1,6 +1,6 @@
     {
    -  foo
    +  bar
     }
     {
    -  bar
    +  foo
     }
    diff --git a/t/d.md b/t/d.md
    index d674c28d912b35623f632c96756c36bb4a464d61..541151ff704da20bdf15c820ae2e8f5e606c9f0e 100644
    --- a/t/d.md
    +++ b/t/d.md
    @@ -1,3 +1,7 @@
    +a
    +
    +  b
    +b
     a
       a
     b
    diff --git a/t/new.md b/t/new.md
    new file mode 100644
    index 0000000000000000000000000000000000000000..445cea22ee57b8ce2a2434dc79312b0cb63b33eb
    --- /dev/null
    +++ b/t/new.md
    @@ -0,0 +1,2 @@
    +old one
    +old two
    diff --git a/t/old.md b/t/old.md
    deleted file mode 100644
    index 445cea22ee57b8ce2a2434dc79312b0cb63b33eb..0000000000000000000000000000000000000000
    --- a/t/old.md
    +++ /dev/null
    @@ -1,2 +0,0 @@
    -old one
    -old two
    diff --git "a/t/\346\227\245\346\234\254\350\252\236.md" "b/t/\346\227\245\346\234\254\350\252\236.md"
    index dfbbd52b717b5087823badc37e4af30654679998..051eaf9e7b0191ee09ded5f6962dcaf1ff47b9aa 100644
    --- "a/t/\346\227\245\346\234\254\350\252\236.md"
    +++ "b/t/\346\227\245\346\234\254\350\252\236.md"
    @@ -1,2 +1,2 @@
     こんにちは
    -世界
    +世界!
    ````

-   ファイルの一覧のコマンドの出力（同）:

    ````text
    t/a.md
    t/b.md
    t/c.md
    t/d.md
    t/new.md
    t/old.md
    "t/\346\227\245\346\234\254\350\252\236.md"
    ````

PR-1 の実行の記録（ステップ 1-9）:

-   コマンド：`docs/tasks/0080_diff_check_proportion/_checks/scope.sh <base> <commit> pr1`（作業ツリーの最上位で実行した）
-   マージの基点（`git merge-base origin/main HEAD`。`git fetch origin main` の後）：`bc6866d5edb490d56bd02593ecd786d6947e6074`
-   比べたコミット（PR-1 の最後の実装のコミット）：`baec77a570d237f36f479c56c611f6c1818561a1`
-   スクリプトの blob の ID：`8ca3335b9983cbe34354b2fc88dff842345849df`
-   終了コード：0
-   出力:

    ```text
    base:   bc6866d5edb490d56bd02593ecd786d6947e6074
    commit: baec77a570d237f36f479c56c611f6c1818561a1
    mode:   pr1
    ok   file-scope
    ok   adr-deletions-only
    ok   adr-resolved (docs/dev/adr/0016-document-test-policy.md)
    ok   adr35-unchanged
    result: PASS
    ```

PR-2 の実行の記録（ステップ 2-4）:

-   （PR-2 の実装で記録する）

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-13 の検証がすべて済み、§7 に記録がある。
-   PR-2 のマージの後の `main` で、両 ADR に issue #789 の食い違いの記述がない（AC-13）。
-   各 PR で `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`/runpr` が issue #789 を閉じる。
-   新しい規則は、PR-1 のマージの後に書く計画から当てはめる（F-007）。

## 変更履歴

- 2026-11-10 承認（isseis、#800）
