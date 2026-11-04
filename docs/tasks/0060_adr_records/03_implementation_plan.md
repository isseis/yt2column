# 実装計画書：主要な設計の決定を ADR として docs/dev に記録する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-04 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.2 節」などは設計書の節を指す。用語（規約、共通の手順、呼び出し元のコマンド、ADR の変更、ADR の判断、ADR の guard、`ADR-NNNN` など）は、要件書 6 章と設計書の冒頭に従う。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-08 は、設計書の付録 A がすべて記録している。

既存のファイルについての記述は、コミット `034ac717`（2026-11-04 の `main`）で確かめた。設計書が行番号を引いたコミット `8749e5d` から `034ac717` までに、`.claude/commands`・`CLAUDE.md`・`docs/dev`・`cmd/yt2column`・`internal/doctestutil`・`.github/workflows` のうち変わったのは、`.claude/commands/runplan.md`（手順 7.5 の観点の 1 項目）、`.claude/commands/weakreview.md`、`CLAUDE.md`（Testing Strategy のテストの削除の項）だけである（`git diff --stat 8749e5d 034ac717 -- <これらのパス>` の出力）。本タスクが変える箇所（設計書 2.2 節の表）の行番号は、`CLAUDE.md` の「Document language」の原則 (a) が 42 行から始まることを含め、変わっていない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

主要な決定を `docs/dev/adr/` の ADR として記録する仕組み（規約、索引、ADR の guard、CLAUDE.md の規則、共通の手順の段）を作り、F-006 の 15 件の主題を ADR にする。何を作るか（置き場所、雛形、状態、索引の形、guard が確かめること、コマンドの変える箇所、ADR の番号の予定）は設計書 1〜6 章で決まっている。本書は、それをどの順序で、どの PR に分け、どう確かめるかを定める。

### 1.2. 実装の原則

-   フェーズは設計書 8 章の 4 つの PR（①〜④）に合わせ、1 つのフェーズを 1 つの PR にする。
-   規約・CLAUDE.md・共通の手順に書く内容は設計書 3 章の節を参照し、本書では繰り返さない。文の最終の文言、guard のテストの表の行、エラーの報告の文は、書くときに決める。
-   加えたテストと確かめのプログラムは、確かめる対象を壊して失敗することを実装のときに確かめ、コミットメッセージにそう書く（CLAUDE.md「Testing Strategy」、設計書 7.3 節）。
-   Go のコメント・識別子・文字列リテラルは英語で書く（CLAUDE.md「Go Idioms」）。ADR の guard（`_test.go` で文書を読む guard テスト）だけは、表のセルの日本語（`状態` の行の名前）をリテラルに持ってよい（guard-test literal exception。設計書 3.6 節）。`_checks/` のプログラムはこの例外の範囲に入らないので、日本語のリテラルを持たず、日本語の見出しや項目は位置で探す（ステップ 1-6）。
-   ADR・索引は日本語、規約・CLAUDE.md・コマンドの定義・`requirements_process.md` は英語で書く（CLAUDE.md「Document language」）。
-   リンクとリンク先は同じコミットに入れるか、リンク先を先にコミットする（設計書 3.7 節）。各コミットで `make test` が通るようにする。
-   各フェーズの終わりに、グリーンゲート（`make test && make lint`）を通す。Go のファイルを変えるフェーズ 1 では `make fmt` と `make deadcode` も通す。本タスクは拡張・README・`docs/dev/guides/development.md`・`.gitignore` を変えないので、`make ext-test` は要らない（CLAUDE.md「Development Notes」）。

### 1.3. 計画の際に決めたこと（2026-11-04）

レビュアーと決めたこと:

-   **AC-17・AC-28・AC-29 のリンク先と AC-31 の前後の比較は、`_checks/` の 1 つの Go のプログラムで確かめる:** 設計書の付録 B は AC-17・AC-28・AC-29 を `manual` としている。しかし、これらは「どのファイルへのリンクがあるか」という値を含むので、文の内容だけを定める AC ではない（`requirements_process.md` §4）。15 本の ADR の経緯から出典のタスクが漏れる誤りも起こりやすい。恒久のテストにはせず、`_checks/adrlinks`（ステップ 1-6）を各 PR で 1 回実行し、7.1 節に記録する。PR を重ねる途中で、まだ作っていない ADR の項目は「未作成」として失敗とは分けて表示する。最後の PR（PR-4）では、未作成の項目がないことを求める。設計書の付録 B の種類を変えるのではなく、`manual` の読み合わせに加えて実行する確かめを足す。設計書はこの点の詳細を実装計画に任せている（付録 B の「確かめ方の種類（詳細は実装計画）」）。

上の決定と設計書から導いたこと（PR のレビューで異論があれば改める）:

-   **ADR のファイル名の `<kebab>` は 1.5 節の表のとおりにする:** 設計書 3.1.3 節が実装計画に任せた名前である。主題を短く表す英語にする。番号はほかのタスクが先に取れば変わる（設計書 3.1.1 節）ので、`_checks/adrlinks` は ADR を番号ではなく `<kebab>` で探す。
-   **規約の節の英語の見出しは 1.6 節の表のとおりにする:** 設計書 3.2.1 節が実装計画に任せた文言である。ADR の guard は、そのうち `## Template`・`## Status`・`## Index` の 3 つを見出しとしてテストに固定する（設計書 3.6 節「規約の節を探す見出し」）。
-   **AC-30 の確かめは、0056 の `task_docs_unchanged.sh` を写して使う:** 0056 のスクリプト（`docs/tasks/0056_legacy_approval_history/_checks/task_docs_unchanged.sh`）は、`sh` と `git` だけを使い、`task` の変数でタスクのディレクトリを決める。`task` の変数と冒頭のコメントのタスク名・使い方の例だけを本タスクのものに変える。4 つの PR のそれぞれで実行する。
-   **F-007 の issue は PR-1 の中で、PR を開く前に作る:** 設計書 8 章のとおり ① の段階で作る。PR-1 の説明に issue の URL を書けるように、PR を開く前に作る。issue の作成と issue #602 のコメントの読み取りはネットワークの操作なので、その時に利用者の承認を得る。
-   **ADR を書くフェーズ（2〜4）は、最初に索引を読み直す:** 本タスクの途中でほかのタスクが同じ主題の ADR を作っていれば、新しく作らずにその ADR を更新する（設計書 3.1.3 節の最後の段落）。

### 1.4. 既存コード調査結果

**ADR の guard が使う既存の関数（`internal/doctestutil`）**

-   `SectionOutsideBlocks(doc, heading string) string`（`internal/doctestutil/markdown.go:44`）は、コードブロックの外の見出しで節を取る。返す節の文には、節の中のコードブロックの行も残る。
-   `FencedBlocks(doc string) []FencedBlock`（`markdown.go:165`）は、コードブロックを順に返す。`ScanDocLines`（`markdown.go:130`）の `DocLine.Block` が、コードブロックの行の印である。
-   `MarkdownTables(doc string) [][][]string`（`tables.go:10`）と `FindTable(doc, header string) [][]string`（`tables.go:63`）は、コードブロックを飛ばさない。設計書 3.6 節のとおり、決まった節の中で、`ScanDocLines` でコードブロックの行を除いてから探す。`StripBackticks`（`tables.go:100`）は、索引の状態の列のバッククォートを取るのに使える。
-   これらの関数のテストは `markdown_test.go`・`tables_test.go` にあるので、重ねない。新しい関数を `internal/doctestutil` に足す必要はない。ADR の guard が要る照合の処理は `cmd/yt2column/adr_test.go` の中に置く（設計書 3 章の冒頭）。

**既存の guard（`cmd/yt2column`）**

-   `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`）は、`instructionDocs`（`:106`）が `docs/dev` と `.claude/commands` を走査して対象を決める。`docs/dev/adr/` と `docs/dev/conventions/adr.md` は、テストを変えずに対象になる。各文書について、リンクの開きの数と取り出したリンクの数が合うことも確かめる（`checkLinkCount`、`cmd/yt2column/docs_test.go:89`）。`<` で始まるリンク先は例として扱われるが、`[ADR-NNNN](NNNN-<kebab>.md)` のように `<` が途中にあるリンクは確かめの対象になる（設計書 3.7 節）。規約・CLAUDE.md・共通の手順にリンクの例を書くときは、コードブロックかコードスパンの中に置く。
-   `cmd/yt2column` のテストのパッケージで、リポジトリのルートは `repositoryRoot`（`cmd/yt2column/makefile_test.go:32`、`"../.."`）である。ADR の guard もこれを使う。
-   `cmd/yt2column` のテストは、CI の `doc-guard` ジョブで、すべての PR で実行される（`.github/workflows/ci.yml:173-187`）。ADR の guard を足しても、CI の設定は変えない。

**本タスクが変えるコマンドの定義を読む既存のテスト**

-   `runpr.md` を読むテストは、`cmd/yt2column/fixpr_docs_test.go`（手順 1・3・4 の節の中の `fixprguard` の呼び出しの句があること。`:156-159`、`:710-713`）、`cmd/yt2column/codex_review_docs_test.go:64`（手順 1 の `gh pr create `）、`cmd/fixprguard/docs_test.go:218` である。どれも句があることを確かめるので、手順 1 の「Taking in `main`」（`.claude/commands/runpr.md:249`）に 1 項目を足しても影響しない。
-   すべてのコマンドの定義を読むテスト（`doctestutil.CommandDocs` を使う、`cmd/yt2column/codex_review_docs_test.go:94`・`:221`、`cmd/codexreview/docs_test.go`、`cmd/fixprguard/docs_test.go:364`）は、`codex` を直接動かすコマンドや `fixprguard` を迂回するコマンドがないことなどを確かめる。本タスクで足す文は、これらのコマンドもレビューを頼む言及も含めない。
-   `mkreq.md`・`mkarch.md`・`mkplan.md`・`runplan.md`・`_lib/settle-decisions.md`・`requirements_process.md` の文言を固定するテストはない（`git grep` で、これらのパスを読む `_test.go` を探して確かめた。上の `CommandDocs` の走査を除く）。
-   `CLAUDE.md` を読むテストは、`cmd/yt2column/extension_docs_test.go:63`（`ext-` の対象）、`cmd/yt2column/makefile_test.go:984`（`TestCoverageCompareDocumented`）、`cmd/yt2column/codex_review_docs_test.go:98` である。足す小節と Quick Links の行は、これらが読む値を変えない。
-   `project_overview.md` を読むテストは、`cmd/yt2column/docs_test.go`（`:132-153` の 16,383、`:231-294` の LLM の既定の表、`:808-874` の `costPrefix`）と `cmd/yt2column-server/docs_test.go:230` である。「決定済みの方針」の項目の末尾にリンクを足しても、これらが読む値は変わらない。

**本タスクが変える箇所（設計書 2.2 節の表の行番号を `034ac717` で確かめた）**

-   `.claude/commands/_lib/settle-decisions.md`：入力は 15〜19 行、手順は 1〜5 の番号付きの項目（21〜43 行）。
-   `.claude/commands/mkreq.md`：手順 6 のコミット（110〜112 行）、手順 7 の FILES（119 行）、`japrose`（126 行）。
-   `.claude/commands/mkarch.md`：74 行（付録「決定の履歴」）、手順 7.5（76 行）、手順 8 の FILES（84 行）、手順 9（126 行）。
-   `.claude/commands/mkplan.md`：手順 7.5（100 行）、手順 8 の FILES（108 行）、手順 9（169 行）。観点を足すのは「Technical correctness checklist」である。
-   `.claude/commands/runplan.md`：手順 5 の「Decision change」（64 行）、手順 7 の FILES（165 行）。
-   `.claude/commands/runpr.md`：「Taking in `main`」（249〜254 行）。
-   `docs/dev/conventions/requirements_process.md`：「Five places hold a record」の段落（96〜108 行）と「Withdrawing a task」（382〜384 行）。151 行の「a decision-history appendix (the past decisions and their reasons)」は、付録の置き場所を述べる文で、AC-19 の後も正しいので変えない。
-   `docs/dev/project_overview.md`：「決定済みの方針」は 7 行の見出しで始まる、文書で最初の `##` の節である。AC-31 の 4 項目は、その節の最上位の箇条の 2〜5 番目（10・11・12・14 行）である。13 行は「LLM（既定は Claude）」の項目の下の入れ子の箇条で、AC-31 の項目ではない。
-   `CLAUDE.md`：Quick Links（5 行）、「## Documents」（22 行）、「Document language」の原則 (a)（42 行）、Development Notes の文書を読む guard の一覧（569 行から）。
-   `.claude/commands/runplan.md:60` の「For a design document such as an ADR, also keep the body on the current system and confine removed-design rationale to a bounded history note」は、設計書の ADR（本文は今の決定、経緯は末尾の節）と合っているので変えない。ほかに `ADR` を書いた指示の文書はない（`grep -rn "ADR\b" CLAUDE.md AGENTS.md .claude docs/dev` の出力が、この 1 行だけ）。

**`_checks/` の既存の例**

-   `docs/tasks/0056_legacy_approval_history/_checks/task_docs_unchanged.sh` は、マージの基点と head の間で、自分のタスクのディレクトリの外の `docs/tasks/` が変わっていないことを確かめる。終了コードは 0（変わっていない）・1（変わった）・2（引数やリポジトリが使えない）である。AC-30 にそのまま使える（1.3 節）。
-   `docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go` は、1 行目を `//go:build test` にし、`go run -tags test` で動かし、2 つのコミットの内容を `git` から読む（作業ツリーを読まない）Go のプログラムである。`_checks/adrlinks` はこの形に倣う。

**型による保証:** 本タスクは Go の型を加えない（設計書 3 章の冒頭）。具体的な型で振る舞いを分ける設計もないので、作り方を列挙する確かめは要らない。

### 1.5. ADR のファイル名

設計書 3.1.3 節の予定の番号と主題に、`<kebab>` を割り当てる。番号はほかのタスクが先に取れば変わるが、`<kebab>` は変えない。

| 予定の番号 | 調査の番号 | `<kebab>` | PR |
|---|---|---|---|
| 0001 | A3 | `llm-adapter-stdlib` | PR-2 |
| 0002 | A6 | `post-generation-steps` | PR-2 |
| 0003 | A9 | `llm-config-contract` | PR-2 |
| 0004 | A10 | `production-profile` | PR-2 |
| 0005 | B1 | `server-architecture` | PR-3 |
| 0006 | B2 | `threat-model` | PR-3 |
| 0007 | B4 | `manual-transcript-copy` | PR-3 |
| 0008 | B5 | `launch-path` | PR-3 |
| 0009 | B7 | `viewer-opening` | PR-3 |
| 0010 | B8 | `extension-stores-nothing` | PR-3 |
| 0011 | B11 | `webhook-publishing` | PR-3 |
| 0012 | D1 | `review-setup` | PR-4 |
| 0013 | D2 | `deterministic-checks-in-go` | PR-4 |
| 0014 | D3 | `untrusted-review-threads` | PR-4 |
| 0015 | D4 | `document-test-policy` | PR-4 |

この表は AC-25 の主題と ADR の対応表を兼ねる（設計書の付録 B の AC-25）。1 本の ADR は 1 つの主題だけを持つ（設計書 1.2 節）。

### 1.6. 規約の節の見出し

設計書 3.2.1 節の節の順に、次の `##` の見出しを置く。「guard」の列が「固定」のものは、ADR の guard がテストに固定する見出しである。

| 規約の節（設計書 3.2.1 節） | 見出し | guard |
|---|---|---|
| Location and naming | `## Location and naming` | - |
| Template | `## Template` | 固定 |
| Status | `## Status` | 固定 |
| Index | `## Index` | 固定 |
| What counts as major | `## What counts as a major decision` | - |
| Writing an ADR | `## Writing an ADR` | - |
| History entries | `## History entries` | - |
| Changing, retiring, and withdrawing | `## Changing, retiring, and withdrawing` | - |
| Approval and the PR | `## Approval and the PR` | - |
| Taking in `main` | ``## Taking in `main` `` | - |

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: 規約と手順（設計書 8 章の ①）

#### ステップ 1-1: 規約と空の索引

**変更するファイル:** `docs/dev/conventions/adr.md`（新規）、`docs/dev/adr/README.md`（新規）

-   [ ] `docs/dev/conventions/adr.md` を英語で書く。節は 1.6 節の表の順で、中身は設計書 3.1.1 節・3.1.2 節・3.2.2〜3.2.5 節・3.3 節（CLAUDE.md と両方に書く 3 つの内容と、相手を指す文）・3.5.1 節のとおりにする。
-   [ ] `## Template` の節に、設計書 3.2.2 節の雛形を 1 つだけのコードブロックで置く。`## Status` の節に、設計書 3.2.3 節の状態の表をコードブロックの外に 1 つだけ置く。`## Index` の節に、設計書 3.5.1 節の索引の表の見出しの行を 1 つだけのコードブロックで置く。これら 3 つの節には、ほかのコードブロックと表を置かない（設計書 3.6 節「値の出典」）。
-   [ ] `docs/dev/adr/README.md` を日本語で書く。短い説明（索引が今どの決定が効いているかを示すこと、書き方は規約にあること、規約へのリンク）と、見出しと区切りの行だけの表を置く（設計書 3.5.1 節）。
-   [ ] リンクの例（`[ADR-NNNN](...)`、`NNNN-<kebab>.md` など）は、コードブロックかコードスパンに置く（1.4 節「既存の guard」）。

**完了の条件:** `go test -tags test -run TestInstructionDocPaths ./cmd/yt2column` が通る。

#### ステップ 1-2: ADR の guard

**変更するファイル:** `cmd/yt2column/adr_test.go`（新規。`//go:build test`）

-   [ ] 規約・索引・`docs/dev/adr/` の項目を入力として、設計書 3.6 節「確かめること」の表のすべての食い違いを報告する照合の処理を書く。入力は文字列とファイル名の一覧にし、ファイルを読む部分と分ける。規約の定義は、1.6 節の「固定」の 3 つの見出しの節から、設計書 3.6 節「値の出典」のとおりに読む。
-   [ ] `TestADRRecords` を書く。リポジトリの実際の規約・索引・ADR を読み、照合の処理が報告した食い違いをすべて失敗にする。規約の定義が読めないときは、テストを飛ばさずに失敗にする（設計書 3.6 節「読めないときは失敗にする」）。
-   [ ] `TestCheckADRRecords` を書く。照合の処理を、正しい入力と、設計書 3.6 節「失敗の報告」が挙げる食い違いの種類ごとに 1 つずつ壊した入力で確かめ、報告された種類を確かめる（設計書 7.1 節）。規約の節・コードブロック・表が 0 個の入力と 2 個の入力、索引に行がなく ADR もない入力（一致）、索引に行がなく ADR がある入力（不一致）を含める。
-   [ ] 失敗の種類は、文字列の照合ではなく、照合の処理が返す種類の値で確かめる（CLAUDE.md「Error Testing」）。
-   [ ] `TestADRRecords` が実際の文書を壊して失敗することを確かめる。壊し方は設計書 7.3 節の 5 つ（索引の行を消す、ADR の状態の値を変える、`##` の見出しを変える、番号を重複させる、規約の雛形の見出しを消す）とし、ADR がまだない PR-1 では、壊すための一時的な ADR を作業ツリーに置いて確かめる。結果をコミットメッセージに書く。
-   [ ] ステップ 1-1 と同じコミットに入れるか、その後のコミットに入れる（設計書 8 章）。

**完了の条件:** `make fmt`・`go test -tags test -run 'TestADRRecords|TestCheckADRRecords' ./cmd/yt2column`・`make lint`・`make deadcode` が通る。

#### ステップ 1-3: CLAUDE.md と `requirements_process.md`

**変更するファイル:** `CLAUDE.md`、`docs/dev/conventions/requirements_process.md`

-   [ ] `CLAUDE.md` の「## Documents」の章に、小節「Design decisions (ADR)」を足す。中身は設計書 3.3 節の規則（AC-14）・基準（AC-15）・分担（AC-16）で、両方に書く 3 つの内容の直後に、規約を指す文を置く。
-   [ ] `CLAUDE.md` の Quick Links の「Project:」に、索引（`docs/dev/adr/README.md`）と規約（`docs/dev/conventions/adr.md`）を足す（AC-17）。索引の説明には、設計の前に今の決定と理由を探す入口であることを書く。
-   [ ] `CLAUDE.md` の「Document language」の原則 (a) の一覧に、`docs/dev/adr/` の ADR と索引を足す（設計書 3.3 節）。
-   [ ] `CLAUDE.md` の Development Notes の、文書を読む guard の一覧（「the other text-sync guards that read documents」）に ADR の guard を足す（設計書 3.3 節）。
-   [ ] `requirements_process.md` の「Five places hold a record about a task document」の段落を、設計書 3.5.2 節のとおり 6 つ目の置き場所として ADR を挙げる形に改める。段落の冒頭の数を、挙げた置き場所の数に合わせる（AC-18）。
-   [ ] `requirements_process.md` の「Withdrawing a task」に、規約の取り下げの規則を参照する 1 文を足す（設計書 3.5.2 節）。

**完了の条件:** `go test -tags test ./cmd/yt2column` が通る。

#### ステップ 1-4: 共通の手順と呼び出し元のコマンド

**変更するファイル:** `.claude/commands/_lib/settle-decisions.md`、`.claude/commands/mkreq.md`、`.claude/commands/mkarch.md`、`.claude/commands/mkplan.md`、`.claude/commands/runplan.md`、`.claude/commands/runpr.md`

-   [ ] `settle-decisions.md` に、設計書 3.4.1 節の表の 5 つの変更（手順 1 の前の段、手順 2、手順 5、手順 5 の後の段、「Output」の節）を加える。段の中身は規約と CLAUDE.md を参照し、書式・状態・基準を書き写さない（AC-23）。入力は足さない。
-   [ ] `mkreq.md`・`mkarch.md`・`mkplan.md` を、設計書 3.4.2 節の表のとおりに変える（コミット、FILES、`japrose` の対象に ADR の変更を足し、自己レビューの観点に 1 項目を足す）。足す観点の 1 項目の中身は、設計書 3.4.2 節「足す観点の 1 項目」のとおりにする。
-   [ ] `mkarch.md:74` を、設計書 3.4.2 節「`mkarch.md:74` の改め方」のとおりに改める（AC-19）。
-   [ ] `runplan.md` の手順 5 の「Decision change」の「in the same commit」の対象と、手順 7 の FILES に、ADR の変更を足す（設計書 3.4.2 節）。
-   [ ] `runpr.md` の「Taking in `main`」に、規約の ``## Taking in `main` `` の節を参照する 1 項目を足す（設計書 3.1.2 節）。
-   [ ] `mkplan2.md`・`fixpr.md`・`mergepr.md` は変えない（設計書 2.2 節）。
-   [ ] 足す文に、`codex` を直接動かすコマンドと、レビューを頼む言及を書かない（1.4 節「本タスクが変えるコマンドの定義を読む既存のテスト」）。

**完了の条件:** 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline`）が通る。

#### ステップ 1-5: F-007 の issue

**変更するファイル:** なし（GitHub の issue）

-   [ ] 利用者の承認を得て、issue #602 のコメント（2026-11-04）の調査の 35 件を読む。
-   [ ] 利用者の承認を得て、issue を 1 つ作る。本文は日本語で、issue #602 を参照し、F-006 に入らない 20 件（B6 を含む）を、調査の番号・主題・出典のタスクとともに挙げる（AC-32）。B6 を 0058 と 0059 の完了の後に書くこと、0058 と 0059 の完了の後に B2・B8（0058）と B11（0059）の ADR の経緯へ項目を足すことを書く（AC-33）。投稿の前に、`_lib/task-pr-handoff.md` の「Mention check」を本文のファイルに行う。
-   [ ] 20 件が、35 件から F-006 の 15 件を除いたものと一致することを、番号の一覧で照らす。issue の URL を本書 7.1 節に書く。

**完了の条件:** issue が作られ、7.1 節に URL がある。

#### ステップ 1-6: 1 回だけの確かめ

**変更するファイル:** `docs/tasks/0060_adr_records/_checks/task_docs_unchanged.sh`（新規）、`docs/tasks/0060_adr_records/_checks/adrlinks/main.go`（新規。`//go:build test`）

-   [ ] 0056 の `task_docs_unchanged.sh` を写し、`task` の変数と冒頭のコメントのタスク名・使い方の例だけを本タスクのものに変える。実行の権限を付ける（AC-30、1.3 節）。
-   [ ] `_checks/adrlinks` を書く。`go run -tags test ./docs/tasks/0060_adr_records/_checks/adrlinks/ <merge-base> <head>` で動かし、ファイルを `git` から読む（作業ツリーを読まない）。0059 の `_checks/residue` の形に倣う（1.4 節）。確かめることは次の 4 つで、それぞれ `OK`・`NG`・`PENDING` を表示する。終了コードは、`NG` があれば 1、確かめを実行できなければ 2、それ以外は 0 とする。
    -   AC-17: head の `CLAUDE.md` の `## Quick Links` の節に、`docs/dev/adr/README.md` と `docs/dev/conventions/adr.md` への相対リンクがある。
    -   AC-28: head の `docs/dev/adr/` で、1.5 節の `<kebab>` の ADR を名前で探し、見つかった ADR の「経緯」の節に、要件書 F-006 の表の括弧のない出典のタスクのそれぞれのディレクトリへのリンクがある。見つからない ADR は `PENDING` とする。
    -   AC-29: B7 の ADR に、`docs/tasks/0050_viewer_tab/02_architecture.md` へのリンクがある。ADR がなければ `PENDING` とする。
    -   AC-31: `docs/dev/project_overview.md` の AC-31 の 4 項目について、head の項目の文から `ADR-NNNN` のリンクを取り除いたものが、マージの基点の項目の文から同じく取り除いたものと同じであり、head の項目のリンク先が、要件書 AC-31 の表の主題の ADR（1.5 節の `<kebab>`）である。リンク先の ADR が head にまだなく、項目にリンクもなければ `PENDING` とする。リンク先の ADR が head にあるのに項目にリンクがなければ `NG` とする。
-   [ ] `_checks/adrlinks` は日本語の文字列リテラルを持たない（1.2 節）。ADR の「経緯」の節は ADR の 6 番目の `##` の節として、`project_overview.md` の 4 項目は最初の `##` の節の最上位の箇条の 2〜5 番目として、位置で探す（1.4 節）。項目の文は、最上位の箇条の行だけとし、入れ子の箇条を含めない。
-   [ ] `_checks/adrlinks` の各確かめが、対象を壊して `NG` になることを確かめる（Quick Links の行を消す、経緯のリンクを 1 つ消す、`project_overview.md` の項目の文を 1 文字変える）。PR-1 では ADR がないので、AC-28・AC-29 の確かめは、一時的な ADR をコミットした一時的なブランチで確かめる。結果をコミットメッセージに書く。
-   [ ] PR のブランチで、ほかの作業がすべて終わった後に、2 つの確かめを `origin/main` に対して実行する。出力・終了コード・マージの基点と head のコミット ID を 7.1 節に書く。記録を書くコミットが本書だけを変えることも書く。

**完了の条件:** `task_docs_unchanged.sh` が 0 で終わる。`_checks/adrlinks` が 0 で終わり、AC-17 が `OK`、ほかが `PENDING` である。

### PR-1 作成ポイント: ADR convention, guard, and procedure

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `docs(0060): ADR の規約と索引、guard、決定を確かめる手順への段を加える`

**レビュー観点**: 規約・CLAUDE.md・共通の手順の分担（規則は規約にだけあり、CLAUDE.md と両方に書く 3 つの内容に相手を指す文があり、手順は規約を書き写さない） / ADR の guard が規約の定義を読み、読めないときに飛ばさず失敗すること、壊して失敗することの記録 / 共通の手順の新しい段と出力を、4 つのコマンドがコミット・レビュー・`japrose` で使うこと / F-007 の issue の 20 件が調査の 35 件から F-006 の 15 件を除いたものであること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-4 が、4 つのコマンドが従う共通の手順と、以後のすべての PR に効く CLAUDE.md の規則を変える（影響の広い、隔離すべき変更）

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: LLM の ADR（設計書 8 章の ②）

#### ステップ 2-1: A 系の 4 本の ADR

**変更するファイル:** `docs/dev/adr/0001-llm-adapter-stdlib.md`・`0002-post-generation-steps.md`・`0003-llm-config-contract.md`・`0004-production-profile.md`（新規。番号はほかのタスクが先に取れば変わる）、`docs/dev/adr/README.md`

-   [ ] 索引を読み直し、A3・A6・A9・A10 の主題の有効な ADR がすでにないことを確かめる（1.3 節）。あれば、新しく作らずにその ADR を更新する。
-   [ ] 主題ごとに、要件書 F-006 の表の括弧のない出典のタスクの `01_requirements.md`・`02_architecture.md`（A10 は 0008・0018 の `02_evaluation.md` も）と、今の `main` のコード・正本の文書（`production_profile.md` など）を読み、規約の雛形で ADR を書く（設計書 6.2 節）。
-   [ ] 「決定」の節は今の `main` のコードの振る舞いと照らす（AC-26）。食い違いを見つけたら、利用者の承認を得て issue を作り、「決定」の節に食い違いと issue へのリンクを書く（設計書 4 章）。
-   [ ] 「採らなかった案」は出典の文書にあるものだけを書き、理由のない案は「理由は記録されていない」と書く（AC-08・AC-27）。
-   [ ] 「経緯」は出典のタスクごとに少なくとも 1 項目を置き、日付を設計書 3.2.4 節の順で選ぶ（AC-09・AC-28）。
-   [ ] 索引に 4 行を足す。
-   [ ] PR の説明に、各 ADR の「決定」と「採らなかった案」の出典（文書の節）を挙げる（設計書 7.5 節）。

#### ステップ 2-2: `project_overview.md` の LLM の 2 項目

**変更するファイル:** `docs/dev/project_overview.md`

-   [ ] 「LLM（既定は Claude）」の項目の末尾に A3・A10 の ADR へ、「LLM プロバイダは差し替え可能にする」の項目の末尾に A3 の ADR へ、`[ADR-NNNN](adr/NNNN-<kebab>.md)` の形のリンクを足す。項目の要約の文は変えない（設計書 3.5.3 節）。
-   [ ] ステップ 2-1 の ADR と同じコミットか、その後のコミットに入れる（1.2 節）。

#### ステップ 2-3: 1 回だけの確かめ

**変更するファイル:** 本書 7.1 節

-   [ ] PR のブランチで、ほかの作業がすべて終わった後に、`task_docs_unchanged.sh` と `_checks/adrlinks` を `origin/main` に対して実行し、ステップ 1-6 と同じく 7.1 節に記録する。`_checks/adrlinks` は、A 系の 4 本の AC-28 と、LLM の 2 項目の AC-31 が `OK` である。

**完了の条件（ステップ 2-1〜2-3）:** グリーンゲートが通る（`TestADRRecords` が 4 本の ADR と索引を照らす）。2 つの確かめが 0 で終わる。

### PR-2 作成ポイント: LLM ADRs

**対象ステップ**: 2-1 / 2-2 / 2-3

**推奨タイトル**: `docs(0060): LLM のアダプタと設定、生成後の段階、本番の構成の ADR を加える`

**レビュー観点**: 各 ADR の「決定」が今の `main` のコードと正本の文書に合っていること / 「採らなかった案」と理由が出典の文書にあるものだけであること / 経緯の項目が出典のタスクをすべて持ち、日付が設計書 3.2.4 節の順で選ばれていること / `project_overview.md` の 2 項目の要約の文が変わっていないこと

**実装モデル要件**: standard

**判定理由**: 規約と雛形に沿って既存の文書を写す作業で、どの判定の条件にも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 構成と拡張の ADR（設計書 8 章の ③）

#### ステップ 3-1: B 系の 7 本の ADR

**変更するファイル:** `docs/dev/adr/0005-server-architecture.md`〜`0011-webhook-publishing.md`（新規。1.5 節の 7 本）、`docs/dev/adr/README.md`

-   [ ] ステップ 2-1 の作業（索引の読み直し、出典とコードの照らし合わせ、採らなかった案と経緯の書き方、索引の行、PR の説明の出典）を、B1・B2・B4・B5・B7・B8・B11 の 7 本に行う。B4 は 0029 の `observation_gate_review.md` も出典として読む。
-   [ ] B2・B8・B11 は、括弧に入れた 0058・0059 を経緯に入れず、書く時点の `main` のコードと文書の状態で書く（設計書 6.2 節）。0058・0059 がすでにマージされていて、書く時点の `main` がそれらの決定を含むときは、F-007 の issue の該当の項目を、ADR の今の経緯と照らして直す（設計書 3.1.3 節）。
-   [ ] B7 の ADR に、`docs/tasks/0050_viewer_tab/02_architecture.md` の付録 B（B.1〜B.3）の採らなかった案をすべて写し、付録 B へリンクする（AC-29）。
-   [ ] B5 と B7 のように内容の重なる主題は、ADR どうしのリンクで扱う（設計書 1.2 節）。

#### ステップ 3-2: `project_overview.md` の実行形式と字幕取得

**変更するファイル:** `docs/dev/project_overview.md`

-   [ ] 「実行形式」の項目の末尾に B1・B5 の ADR へ、「字幕取得」の項目の末尾に B4 の ADR へ、ステップ 2-2 と同じ形のリンクを足す。項目の要約の文は変えない。

#### ステップ 3-3: 1 回だけの確かめ

**変更するファイル:** 本書 7.1 節

-   [ ] ステップ 2-3 と同じく、2 つの確かめを実行して記録する。`_checks/adrlinks` は、A 系と B 系の 11 本の AC-28、AC-29、AC-31 の 4 項目が `OK` である。

**完了の条件（ステップ 3-1〜3-3）:** グリーンゲートが通る。2 つの確かめが 0 で終わる。

### PR-3 作成ポイント: architecture and extension ADRs

**対象ステップ**: 3-1 / 3-2 / 3-3

**推奨タイトル**: `docs(0060): サーバの構成、脅威モデル、拡張の起動とビューアなどの ADR を加える`

**レビュー観点**: 各 ADR の「決定」が今の `main` のコードと正本の文書（`security.md` §8 など）に合っていること / B7 の ADR が 0050 の付録 B の採らなかった案をすべて含むこと / B2・B8・B11 の経緯に 0058・0059 が入っていないこと（書く時点の `main` に入っていた場合は F-007 の issue との整合） / `project_overview.md` の 2 項目の要約の文が変わっていないこと

**実装モデル要件**: standard

**判定理由**: PR-2 と同じ作業で、どの判定の条件にも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: プロセスの ADR（設計書 8 章の ④）

#### ステップ 4-1: D 系の 4 本の ADR

**変更するファイル:** `docs/dev/adr/0012-review-setup.md`〜`0015-document-test-policy.md`（新規。1.5 節の 4 本）、`docs/dev/adr/README.md`

-   [ ] ステップ 2-1 の作業を、D1〜D4 の 4 本に行う。D 系の正本の文書は、CLAUDE.md・`.claude/commands/_context.md`・`docs/dev/guides/` の手引きである。「決定」の節はそれらを要約してリンクし、CLAUDE.md の規則そのものを ADR に写さない（要件書 1 章「上の決定から導いたこと」）。

#### ステップ 4-2: 1 回だけの確かめ

**変更するファイル:** 本書 7.1 節

-   [ ] ステップ 2-3 と同じく、2 つの確かめを実行して記録する。`_checks/adrlinks` は、すべての確かめが `OK` で、`PENDING` がない。

**完了の条件（ステップ 4-1・4-2）:** グリーンゲートが通る。2 つの確かめが 0 で終わり、`PENDING` がない。

### PR-4 作成ポイント: process ADRs

**対象ステップ**: 4-1 / 4-2

**推奨タイトル**: `docs(0060): レビューの体制、開発ツール、スレッドの扱い、文書のテストの方針の ADR を加える`

**レビュー観点**: 各 ADR の「決定」が CLAUDE.md・`_context.md`・手引きの今の規則を要約してリンクし、規則を写していないこと / 「採らなかった案」と理由が出典の文書にあるものだけであること / 経緯の項目が出典のタスクをすべて持つこと / `_checks/adrlinks` に `PENDING` がないこと

**実装モデル要件**: standard

**判定理由**: PR-2 と同じ作業で、どの判定の条件にも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: 規約と手順 | 規約、空の索引、ADR の guard、CLAUDE.md、`requirements_process.md`、共通の手順とコマンドの定義、F-007 の issue、`_checks/` の 2 つの確かめ | PR-1 がマージされた |
| M2: LLM の ADR | A 系の 4 本、索引の 4 行、`project_overview.md` の 2 項目のリンク | PR-2 がマージされた |
| M3: 構成と拡張の ADR | B 系の 7 本、索引の 7 行、`project_overview.md` の 2 項目のリンク | PR-3 がマージされた |
| M4: プロセスの ADR | D 系の 4 本、索引の 4 行 | PR-4 がマージされた |

M2〜M4 は M1 の規約・索引・guard に依存する。M2 → M3 → M4 は、この順にマージする。番号は抜けなく続く必要があり、予定の番号は前の PR の番号に続くためである（設計書 8 章）。ADR を書く作業は並行して進めてよいが、PR を開くのは前の PR のマージの後にする。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | 規約、空の索引、ADR の guard、CLAUDE.md、`requirements_process.md`、共通の手順とコマンドの定義、F-007 の issue、`_checks/` | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 | A 系の 4 本の ADR、`project_overview.md` の LLM の 2 項目 | standard |
| PR-3 | 3-1 / 3-2 / 3-3 | B 系の 7 本の ADR、`project_overview.md` の実行形式と字幕取得 | standard |
| PR-4 | 4-1 / 4-2 | D 系の 4 本の ADR | standard |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどの確かめで確かめるかは本書 7 章に示す。

-   **単体テスト:** ADR の guard の照合の処理を、文字列を入力にした表のテスト（`TestCheckADRRecords`）で、食い違いの種類ごとに確かめる（ステップ 1-2）。`internal/doctestutil` の既存の関数は、すでにテストがあるので重ねない。
-   **実際の文書に対する確かめ:** `TestADRRecords` が、リポジトリの規約・索引・ADR を読んで照らす。`make test` と CI の `doc-guard` ジョブで、すべての PR で実行される。リンクの実在は既存の `TestInstructionDocPaths` が確かめる。
-   **結合テスト:** N/A（設計書 7.2 節）。
-   **後方互換:** 既存のテストは変えない。既存のコマンドの定義を読むテストが、足した文の後も通ることを、各フェーズのグリーンゲートで確かめる（1.4 節）。
-   **1 回だけの確かめ:** `task_docs_unchanged.sh`（AC-30）と `_checks/adrlinks`（AC-17・AC-28・AC-29・AC-31）を各 PR で実行し、7.1 節に記録する。
-   **壊して確かめる:** ADR の guard と `_checks/adrlinks` の各確かめは、対象を壊して失敗することを実装のときに確かめ、コミットメッセージに書く（ステップ 1-2・1-6）。
-   **読んで確かめる:** 文の内容だけを定める AC と、guard が確かめない部分は、PR のレビューで読む（設計書 7.5 節）。AC-26・AC-27 のために、PR-2〜PR-4 の説明に各 ADR の出典を挙げる。
-   **カバレッジ:** 数値の目標は置かない。本タスクはテストを削除しないので、`make coverage-compare` は使わない。
-   **セキュリティ:** N/A（設計書 7.6 節）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| ほかのタスクが本タスクの途中で ADR を作り、番号や主題が重なる | 予定の番号が使えない。同じ主題の ADR が 2 本になる | ADR を書くフェーズの最初に索引を読み直す（1.3 節）。番号は 1.5 節の予定から変わってよく、`_checks/adrlinks` は `<kebab>` で ADR を探す。`main` を取り込むときは規約の ``## Taking in `main` `` の手順で直す |
| ADR に出典にない採らなかった案や理由を書き足す | AC-27 を満たさない | PR の説明に各 ADR の出典を挙げ、レビュアーが照らせるようにする（ステップ 2-1） |
| ADR の「決定」が今のコードと食い違う | AC-26 を満たさない | 書く時点の `main` のコードと照らし、食い違いは issue にする（ステップ 2-1） |
| 0058・0059 が本タスクの途中でマージされる | B2・B8・B11 の書く時点の状態が変わる。F-007 の issue の項目が古くなる | ステップ 3-1 で、書く時点の `main` で書き、issue の項目を照らして直す |
| ADR の guard の規約の定義の読み方が、規約の書き方と合わない | PR-1 のグリーンゲートが通らない | 規約と guard を同じ PR に入れ、guard は規約と同じコミットかその後に入れる（ステップ 1-2） |
| 足した文が、コマンドの定義を読む既存のテストの検査に当たる | 文書のガードが失敗する | ステップ 1-4 の完了の条件で文書のガードを通す。`codex` のコマンドとレビューを頼む言及を書かない |
| PR-3 が 7 本の ADR で大きい | レビューに時間がかかる | 設計書 1.2 節で決めた分け方に従う。ADR は 1 本ずつコミットし、レビュアーが 1 本ずつ読めるようにする |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（`make test` で実行するテスト）、`static`（`_checks/` の、PR ごとに 1 回実行するプログラム）、`diff`（`_checks/` の、マージの基点と PR の head を比べるスクリプトやプログラム）、`manual`（PR のレビューで読む、issue を確かめる）である。`static` と `diff` の実行の記録は 7.1 節に書く。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | test | `cmd/yt2column/adr_test.go::TestADRRecords`（実際の文書）、`cmd/yt2column/adr_test.go::TestCheckADRRecords`（名前の形、`docs/dev/adr/` の直下のほかのファイルとサブディレクトリ、1 行目の番号） | 1-2 |
| AC-02 | test・manual | test: `TestADRRecords`、`TestCheckADRRecords`（番号の重複と抜け）。manual: 廃止した番号を使い回していないこと（設計書 3.6 節「確かめないこと」） | 1-2 |
| AC-03 | test | `TestADRRecords`、`TestCheckADRRecords`（索引にない ADR、ファイルのない索引の行、リンク先、題名と状態の不一致、題名の `\|`、行のない索引） | 1-2 |
| AC-04 | manual | PR-1〜PR-4 のレビューで、索引と ADR が日本語で書かれていることを読む | 1-1・2-1・3-1・4-1 |
| AC-05 | test | `TestADRRecords`、`TestCheckADRRecords`（`##` の見出しの文言と順序、規約の雛形の見出しの数） | 1-2 |
| AC-06 | test・manual | test: `TestADRRecords`、`TestCheckADRRecords`（状態の表と値、規約の状態の数）。manual: PR-1 のレビューで、規約の状態の意味の文を読む | 1-1・1-2 |
| AC-07 | manual | PR-1 のレビューで規約を、PR-2〜PR-4 のレビューで各 ADR の「決定」の節を読む | 1-1・2-1・3-1・4-1 |
| AC-08 | manual | 同上（「採らなかった案」の節） | 1-1・2-1・3-1・4-1 |
| AC-09 | manual | 同上（「経緯」の節の項目の形・日付・順） | 1-1・2-1・3-1・4-1 |
| AC-10 | manual | PR-1 のレビューで、規約の廃止の規則を読む | 1-1 |
| AC-11 | manual | PR-1〜PR-4 のレビューで読む。リンクの実在は `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（既存） | 1-1・2-1・3-1・4-1 |
| AC-12 | test・manual | test: `TestADRRecords`（規約の `## Template`・`## Status`・`## Index` の節が読めないと失敗する）。manual: PR-1 のレビューで、規約が英語で書かれていることを読む | 1-1・1-2 |
| AC-13 | manual | PR-1 のレビューで、規約の各節を読む | 1-1 |
| AC-14 | manual | PR-1 のレビューで、CLAUDE.md の「Design decisions (ADR)」を読む | 1-3 |
| AC-15 | manual | 同上（基準と、規約の基準と同じ内容であること） | 1-3 |
| AC-16 | manual | 同上（分担の 5 項目） | 1-3 |
| AC-17 | static・manual | static: `docs/tasks/0060_adr_records/_checks/adrlinks/main.go`（AC-17 の確かめ）。manual: PR-1 のレビューで、索引の説明の文を読む | 1-3・1-6 |
| AC-18 | manual | PR-1 のレビューで、`requirements_process.md` の段落を読む。置き場所の数は文で挙げられているので、数と置き場所の対応も読んで確かめる | 1-3 |
| AC-19 | manual | PR-1 のレビューで、`mkarch.md` の箇条を読む | 1-4 |
| AC-20 | manual | PR-1 のレビューで、`settle-decisions.md` の新しい段を読む | 1-4 |
| AC-21 | manual | 同上（手順 2 と手順 5） | 1-4 |
| AC-22 | manual | 同上（手順 5 の後の段） | 1-4 |
| AC-23 | manual | PR-1 のレビューで、`settle-decisions.md` が規約を参照して書き写していないこと、4 つのコマンドの定義が同じ手順を書き写していないことを読む | 1-4 |
| AC-24 | manual | PR-1 のレビューで、4 つのコマンドの定義のコミット・FILES・`japrose` の対象と「Output」の節を読む | 1-4 |
| AC-25 | manual | PR-2〜PR-4 のレビューで、1.5 節の表の主題と ADR が一対一であることを読む | 2-1・3-1・4-1 |
| AC-26 | manual | PR-2〜PR-4 のレビューで、PR の説明の出典と照らす。食い違いの issue があれば PR の説明に挙げる | 2-1・3-1・4-1 |
| AC-27 | manual | 同上（「採らなかった案」と理由） | 2-1・3-1・4-1 |
| AC-28 | static・manual | static: `_checks/adrlinks`（AC-28 の確かめ。PR-4 で `PENDING` がない）。manual: 各項目の要旨と日付を読む | 1-6・2-3・3-3・4-2 |
| AC-29 | static・manual | static: `_checks/adrlinks`（AC-29 の確かめ）。manual: PR-3 のレビューで、付録 B（B.1〜B.3）の採らなかった案がすべて写されていることを読む | 1-6・3-1・3-3 |
| AC-30 | diff | `docs/tasks/0060_adr_records/_checks/task_docs_unchanged.sh`（PR-1〜PR-4 の各 PR で実行） | 1-6・2-3・3-3・4-2 |
| AC-31 | diff | `_checks/adrlinks`（AC-31 の確かめ。PR-3 の時点で 4 項目が `OK`）。リンクの実在は `TestInstructionDocPaths`（既存） | 1-6・2-2・2-3・3-2・3-3 |
| AC-32 | manual | ステップ 1-5 で、issue の 20 件を調査の 35 件と照らし、URL を 7.1 節に書く。PR-1 のレビューで issue を読む | 1-5 |
| AC-33 | manual | 同上（B6 と経緯の追加の 2 点） | 1-5 |

### 7.1. 実行の記録

（ステップ 1-5・1-6・2-3・3-3・4-2 で記入する。各 PR の `task_docs_unchanged.sh` と `_checks/adrlinks` の出力・終了コード・マージの基点と head のコミット ID、F-007 の issue の URL を書く。）

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 7 章のすべての行の確かめが済み、7.1 節に 4 つの PR の確かめの記録と F-007 の issue の URL がある。`_checks/adrlinks` が PR-4 で `PENDING` を表示しない。
-   **品質:** 各 PR でグリーンゲート（`make test && make lint`）が通り、PR-1 で `make deadcode` も通る。ADR の guard と `_checks/adrlinks` は、対象を壊して失敗することをコミットメッセージに記録している。
-   **セキュリティ:** N/A（要件書 4.2）。
-   **文書:** 15 本の ADR と索引があり、CLAUDE.md・`requirements_process.md`・共通の手順・コマンドの定義・`project_overview.md` が、ADR を記録の置き場所として扱っている。

## 9. 次のステップ (Next Steps)

-   PR-4 のマージの後、`/runpr` が issue #602 を閉じる案内をする（本タスクの最後の PR のため）。
-   F-007 の issue で、残りの 20 件の ADR と、B2・B8・B11 の経緯への 0058・0059 の項目を扱う（設計書 9 章）。
