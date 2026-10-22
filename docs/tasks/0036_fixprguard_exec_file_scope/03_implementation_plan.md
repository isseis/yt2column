# 実装計画書：fixprguard の範囲の規則で、下の階層の指示ファイルと Go のビルドの入力を範囲外にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-22 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-005 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。作業は設計書 8 章の 3 つの項目を 1 つずつフェーズにし、2 つの PR にまとめる（1.4）。

-   **フェーズ 1（設計書 8 章の 1）:** タスク 0034 の状態を確かめて申し送る。`scope.go` の拒否リスト・許可リスト・判定の順序を改め、`scope_test.go` に行を加える。ガイド 6 章（日英）と `TestGuideCoversStopKindsAndScope` を改める。
-   **フェーズ 2（設計書 8 章の 2）:** `Protocol` を `3` に上げ、`fixpr.md`・`runpr.md`・ガイド・`check_rules.sh` の番号と、ガイド 9 章を合わせる。
-   **フェーズ 3（設計書 8 章の 3）:** ガイド 7.2・8 章（日英）、用語集、0030 の文書の注記と、それらの記述を確かめるガード（AC-18・AC-20・AC-21）を加える。

用語は、要件書 6 章と設計書の冒頭の定義に従う（実行関連ファイル、拒否リスト、許可リスト、指示ファイル、Go のビルドの入力、規則 2・3 の位置、規則 1 の位置）。許可リストの (a)〜(c) は設計書 1.5 の項目を、「順 1」〜「順 6」は設計書 3.3 の判定の順序を指す。本書の「フェーズ」は実装の区切りを指し、`/fixpr` の「修正のフェーズ」とは別である。「ガイド」は `docs/dev/developer_guide/fixprguard_guide.ja.md`（日本語版）と `fixprguard_guide.md`（英語版）を指す。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの文書（`.claude/commands/`）の文も英語で書く（`CLAUDE.md`「Language」）。例外は、日本語の文書の文字列と一致させる文書のガードのリテラルだけである（先例: `cmd/fixprguard/docs_test.go:256-260`）。
-   テストは、対象の実装を壊して失敗することを確かめてからコミットし、壊し方と結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。各ステップの「壊して確かめる」は、壊す対象と、失敗するはずのテストを挙げる。
-   **片方の層だけが働く入力を使う**（設計書 7.1・H-05）。拒否リストを確かめる行は、許可リストだけなら範囲内になる入力（規則 2・3 の位置の `.md`）か、規則 1 の位置の入力にする。許可リストを確かめる行は、拒否リストに当たらない入力にする。
-   テストの行の値は、`scope.go` のデータからではなく、要件書 AC-01・AC-03 に書かれた値から 1 つずつ作る（設計書 7.1）。
-   ガイドは、日本語版をコミットしてから英語版を `.claude/commands/mktrans.md` の原則で翻訳する（`CLAUDE.md`「Translation Guidelines」）。英語版の用語は設計書 3.7 の表の訳語を使う。用語集への追加はフェーズ 3 だが、訳語は設計書で決まっている。日本語版だけをコミットした時点では英語版のガードが失敗するが、フェーズの最後に通ればよい（`runplan.md` step 5 の例外）。
-   Go のファイルを変えたら `make fmt` を実行する。各フェーズの最後に `make test && make lint` を実行する。
-   ソースのコメントに `AC-NN`・`F-NNN` を書かない（要件書との対応は本書 5 章に書く）。

### 1.3. 既存コード調査結果

調査はコミット `d90437c` で行った。設計書が確かめたコミット `3ed049b` から、`internal/`・`cmd/`・`.claude/`・`docs/dev/`・`docs/translation_glossary.md`・`docs/tasks/0030_fixpr_untrusted_threads/`・`docs/tasks/0032_japanese_pr_title/`・`CLAUDE.md` に変更はない（`git diff --stat 3ed049b d90437c -- <これらのパス>` の出力が空）。そのため、設計書の `file:line` は `d90437c` でもそのまま使える。以下の `file:line` も `d90437c` のものである。

#### 1.3.1. 変える箇所の一覧

| 箇所 | 今 | 変え方 | ステップ |
|---|---|---|---|
| `internal/fixprguard/scope.go:39-54` | `execNames`（`.tool-versions`・`.mise.toml` を含む）、`execSuffixes`、`execDirs`（`.claude/`・`.github/` を含む）、`execTopLevel`、`execExtensionFiles`（`.` で始まる 4 つを含む） | 設計書 3.1 のデータにする。`execTopLevel` を消し、指示ファイルの名前と Go のビルドの入力の拡張子のデータを加える | 1-2 |
| `scope.go:102-126` | `isExecutionRelated`。最上位の `.` で始まるファイルと `execTopLevel` を `:119` で照合する | `:119` の照合を消し、`.` で始まる要素の照合を加える（設計書 3.1）。照合は今のとおり `strings.ToLower` の後に行う | 1-2 |
| `scope.go:56-75` | `ExecutionFilePatterns` が `*.go`・`/.*`・`/claude.md`・`/agents.md` を出力する | 設計書 3.4 の項目を出力する。`AllowListPatterns` を加える | 1-2 |
| `scope.go:168-190` | `inScopeOf`。規則 2・3 の場所にあれば範囲内 | 設計書 3.3 の順 1〜6 にする。許可リストの (a)・(b) を照合する非公開の関数を 1 つ加える（設計書 3.4） | 1-2 |
| `scope.go:11-28`・`:36-38`・`:56-59` | 今の規則を述べるコメント | 新しい規則に合わせる（設計書 2.2） | 1-2 |
| `internal/fixprguard/scope_test.go:9-92` | `TestScopeRules` の 56 行。`nested_dotfile_not_root_rule`（`:52`）は `true` を期待する | 設計書 7.1 の行を加え、`:52` の期待を `false` にする（設計書 7.3） | 1-3 |
| `cmd/fixprguard/main_test.go:495-527` | `TestInspectExitCode` は、範囲外の行に `other/b.md`（今の規則でも範囲外の場所）だけを使う | 規則 2 の場所にあり、新しい規則でだけ範囲外になる行を加える（1.4） | 1-3 |
| ガイド 6 章（各版 `:204-228`） | 規則 1〜6、`.go` の段落、実行関連ファイルの説明と 6 行の表 | 設計書 3.5「ガイド 6 章」の表のとおり改める | 1-4 |
| `cmd/fixprguard/docs_test.go:45-62` | `TestGuideCoversStopKindsAndScope` がガイドの全体から項目を探す | 設計書 3.5「文書のガード」のとおり改める。表を切り出す関数を加える（1.4） | 1-5 |
| `internal/fixprguard/types.go:17-20` | `Protocol = 2`。コメントは「input and output contract」 | `3` にし、コメントに判定の規則を加える（設計書 3.6） | 2-1 |
| `.claude/commands/fixpr.md`（8 か所）・`runpr.md`（4 か所） | `--protocol 2`（`fixpr.md:44`・`runpr.md:18` はコードブロックの外の文） | `--protocol 3`（設計書 3.6 の表） | 2-1 |
| ガイド 4 章・5 章（各版） | 現在の番号 `2`（`:64`）、`--protocol 2` が 9 か所、`"protocol":2` が 4 か所（`:92`・`:180`・`:192`・`:202`） | `3`（設計書 3.6 の表） | 2-1 |
| `docs/tasks/0032_japanese_pr_title/check_rules.sh:524` | `fixprguard edit-pr --protocol 2 --title-file <title-file>` を求める | `--protocol 3` にする | 2-1 |
| ガイド 9 章（各版 `:287`・`:300`） | 番号を上げる条件は「出力や引数の契約を変える場合」。一覧の更新の項目は「実行に関わるファイル」だけを挙げる | 設計書 1.2・3.5「ガイド 9 章」・3.6 のとおり改める | 2-1 |
| ガイド 7.2（各版 `:244-259`）・8 章（各版 `:281`） | 本タスクの残余のリスクがない | 設計書 3.5「ガイド 7.2」「ガイド 8 章」のとおり加える | 3-1 |
| `docs/translation_glossary.md:74-75` の近く | 本タスクの 5 つの用語がない | 設計書 3.7 の表の 5 行を加える | 3-2 |
| 0030 `02_architecture.md:564`・`01_requirements.md:136` | 0030 の時点の規則 | 設計書 3.7「0030 の文書」の注記を加え、`Comments` に記録する | 3-3 |
| `cmd/yt2column/fixpr_docs_test.go:702-735` | `TestGlossaryTerms` が fixprguard の用語の行（`範囲外の変更`・`ガードツール` など）を確かめる | 5 つの用語を加える（AC-21。1.4） | 3-4 |
| `cmd/fixprguard/docs_test.go` | ガイド 7.2 と 0030 の注記を確かめるガードがない | 2 つのガードを加える（AC-18・AC-20。1.4） | 3-4 |

変えない箇所は次のとおりである。

-   `cmd/fixprguard` の本番のコードと、`inspect` の流れ・出力の形（設計書 2.2）。`inScope` の型と、`internal/fixprguard/worktree.go:141` の呼び出し。
-   `internal/fixprguard/worktree_test.go`・`worktree_flow_test.go`・`cmd/fixprguard/main_test.go` の既存の行。設計書 7.3 の試作で、すべて通ることを確かめた。`TestProtocolMismatch`（`main_test.go:43`）と `cmd/fixprguard/test_helpers.go:213` は `fixprguard.Protocol` を参照するので、番号を上げても変えない。
-   `.claude/commands/fixpr.md`・`runpr.md` の手順（番号だけを変える）。範囲の規則は言い換えていない（`fixpr.md:476` は終了コード `3` の扱いだけを述べる）。
-   `docs/tasks/` の下の `.md` の文書にある `--protocol` の番号と、ガイド 3 章の「例: タスク 0035 で `1` から `2`」（`:52`）、基準値の例の `'v2....'`（設計書 3.6「変えないもの」）。0032 の計画書の `:444`（0030 の後のツールが `check_rules.sh` を実行関連ファイルに分類しないというリスクの記録）も、当時の記録として変えない。
-   `docs/dev/developer_guide/package_reference.md`（設計書 2.2）。

#### 1.3.2. 外部の前提の確かめ

-   **Go のビルドの入力の拡張子:** 手元の Go 1.27.1 で `go help filetype` を実行し、設計書 1.3 の表と同じ拡張子が載っていることを確かめた（2026-10-22）。Fortran の拡張子は設計書 1.3 のとおり `go/build` による。
-   **インストール済みのツールの振る舞い（AC-16）:** インストール済みの `fixprguard`（`go version -m` の `vcs.revision` は `adcba30`、`Protocol` は `2`）で `fixprguard head --protocol 3 1` を実行すると、`fixprguard: usage: --protocol does not match this binary's 2 (see docs/dev/developer_guide/fixprguard_guide.md)` を出力し、終了コード `2` で終わった（2026-10-22）。引数の解析（`cmd/fixprguard/main.go:232-233`）で止まるので、ネットワークには接続しない。ステップ 2-2 で、PR-1 のブランチの文書から同じことを確かめる。
-   **`check_rules.sh` の実行:** `sh docs/tasks/0032_japanese_pr_title/check_rules.sh fixpr_title` は、`d90437c` で `ok   fixpr_title` を出力し、終了コード `0` で終わった。
-   **タスク 0034 の状態（2026-10-22）:** 要件定義書の PR #429 はマージされ（`09dc151`）、0034 の `01_requirements.md` は `approved` である。設計書はまだない。ブランチ `isseis/0034-fixprguard-ignored-files-02` は `origin/main` の上に自分のコミットを持たない。issue #424 は開いていて、コメントはない。設計書 3.8 は #429 が開いていた時点の記述だが、申し送りの中身は変わらない。伝える先は issue #424 にする（1.4）。

#### 1.3.3. 型の構築の経路

本タスクは新しい型を加えない。拒否リストと許可リストは非公開のパッケージ変数で、`scope.go` の外からは書き換えられない。公開するのは、一覧の写しを返す 2 つの関数だけである（設計書 3.4）。そのため、構築の経路のガードは要らない。

#### 1.3.4. 実装時に決めること

-   拒否リストと許可リストのデータの変数名と、許可リストを照合する非公開の関数の名前。
-   `scope_test.go` の行の名前と並べ方。設計書 7.1 の表の各行の入力を使う。
-   ガイドの表を切り出す関数の形（1.4 の方針の範囲で）。
-   AC-18・AC-20 のガードが固定する語と、箇所を取り出す鍵。語は書いた文から選ぶ。鍵は、本タスクで書き換えない見出しやセルを選ぶ。
-   新しいテストの関数の名前。5 章の表に書いた名前と変えたときは、同じ PR で本書の表を改める。

### 1.4. 計画の際に決めたこと（2026-10-22）

-   **PR は 2 つにする（PR-1 = フェーズ 1・2、PR-2 = フェーズ 3）:** レビュアーが選んだ。理由: PR-1 は判定の規則というセキュリティのゲートの変更なので、それだけに絞ってレビューする。設計書 8 章は、フェーズ 1 と 2 を同じ PR に入れることを求める。フェーズ 3 は文書と、その記述を確かめるガードだけの変更である。受け入れること: PR-1 のマージから PR-2 のマージまで、`main` のガイド 7.2 に本タスクの残余のリスクが載っていない。
-   **0034 への申し送りは、ステップ 1-1 で issue #424 にコメントする:** レビュアーが選んだ。理由: PR #429 はマージされたので、0034 の追跡先は issue #424 である。GitHub への書き込みなので、実装のときに利用者の承認を得て行う（`CLAUDE.md`「Tool Execution Safety」）。受け入れること: ステップ 1-1 より前に 0034 の設計が始まると、設計書 3.8 を知らないまま書かれうる。
-   **ガイド 8 章の段落は、フェーズ 3 に置く:** 設計書 8 章の優先順位は 8 章を挙げていない。8 章の段落（設計書 3.5「ガイド 8 章」）は、範囲外の変更が残ったときの利用者の手順であり、7.2 の残余のリスクと同じく利用者への案内なので、7.2 と同じフェーズにする。
-   **ガイドの表は、太字の見出しの後の最初の表だけを切り出す（設計書 3.5 が実装計画に委ねた切り出し方）:** 6 章を `section` で切り出し、そこから、目印の太字の見出しの行の後に続く、`|` で始まる行の最初のまとまりを表とする。理由: 見出しから次の見出しまでを切り出すと、表の前後の文に項目のコードスパンがあるだけでガードが通る。表の行だけを読めば、表から項目を落としたときに失敗する。切り出しの関数には、`TestCutSection`（`docs_test.go:506`）にならって、表の終わりで止まること、見出しがないときに空を返すことを確かめるテストを付ける。
-   **AC-21 は、既存の `TestGlossaryTerms`（`cmd/yt2column/fixpr_docs_test.go:702`）に行を加えて確かめる（設計書 7.4 が実装計画に委ねた選択）:** このテストは、fixprguard の用語（`範囲外の変更`・`ガードツール` など）の日本語と英語の組をすでに確かめている。同じ形の確かめを `cmd/fixprguard` に作ると、用語集の読み方が 2 か所に分かれる。
-   **AC-18・AC-20 は、`cmd/fixprguard/docs_test.go` に新しいガードを置く:** 0030 の文書の `Status`・`Comments` と、ガイド 7.2 の項目を確かめる既存のガードはない（`cmd/` と `internal/` の `*_test.go` を `0030_fixpr_untrusted_threads` と `### 7.2` で探して確かめた）。節を取り出すには、同じファイルの `section`・`subsection`・`cutSection`・`singleLine`（`docs_test.go:180-217`）を使う。
-   **AC-14 の確かめに、`TestInspectExitCode` の行を加える:** 既存の範囲外の行（`other/b.md`）は、今の規則でも範囲外の場所である。新しい規則でだけ範囲外になる変更が、コマンドの出力の `outOfScope` に同じ形で入ることを確かめるため、`testFile`（`docs/notes/a.md`）と同じディレクトリの、許可リストに当たらないファイルの行と、指示ファイルの行を加える。設計書 7.5 は「結合テストは加えない」とし、AC-14 を既存のテストで確かめるとしていた。既存のテストの表に行を加えるだけで、設計の決定は変えないので、設計書 7.5 を editorial correction として改めた（この計画の PR。設計書の `Comments`）。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-06 は、すべて設計書の付録 A に記録されている。
-   `implementation_handoff.md` はない。

### 1.6. ステップ 1-1 の確かめの結果

（ステップ 1-1 の実装のときに記録する。）

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 範囲の規則とガイド 6 章（設計書 8 章の 1）

#### ステップ 1-1: タスク 0034 の状態の確かめと申し送り

**変更するファイル**: `docs/tasks/0036_fixprguard_exec_file_scope/03_implementation_plan.md`（結果の記録）

-   [ ] `origin/main` の `docs/tasks/0034_fixprguard_ignored_files/` と、0034 の開いている PR を確かめる。0034 の実装が `main` に入っていたら、止まって、0034 が拒否リストをどう使っているかを調べ、扱いを利用者と決め直す（設計書 3.8）。
-   [ ] 設計書 3.8 の内容（定義の中身の変化と 3 つの具体的な食い違い、0034 の守りが本タスクを補う点、行番号、`Protocol` の `3` を本タスクが使い 0034 は `4` 以上を使うこと）を、issue #424 へのコメントの案として日本語で書く。設計書 3.8 への参照を含める。
-   [ ] 利用者の承認を得てから、`gh issue comment 424 --body-file <file>` で投稿する（1.4）。
-   [ ] 確かめた 0034 の状態と、コメントの URL を本書 1.6 に記録する。記録は実装の記録なので、`Status` は変えない。

**完了条件**: 0034 の実装が `main` に入っていないことを確かめ、コメントの URL が本書に記録されている。

#### ステップ 1-2: 拒否リスト・許可リスト・判定の順序

**変更するファイル**: `internal/fixprguard/scope.go`

-   [ ] 拒否リストのデータを設計書 3.1 の表のとおりにし、`.` で始まる要素の照合を加える。照合は今のとおり `strings.ToLower` の後に行う。
-   [ ] 許可リストのデータ（設計書 3.2）と、(a)・(b) を大文字小文字を区別して照合する非公開の関数を加える。
-   [ ] `inScopeOf` を設計書 3.3 の順 1〜6 にする。`inScope` は変えない。
-   [ ] `ExecutionFilePatterns` を設計書 3.4 の項目を出力するように改め、`AllowListPatterns` を加える。コメントは設計書 3.4 のとおりにする。
-   [ ] ファイルの冒頭のコメント（`:11-28`）とデータのコメント（`:36-38`・`:56-59`）を、新しい規則に合わせて英語で書き直す。

**完了条件**: `go build ./...` が通る（テストの期待はステップ 1-3 で改める）。

#### ステップ 1-3: 範囲の規則のテスト

**変更するファイル**: `internal/fixprguard/scope_test.go`、`cmd/fixprguard/main_test.go`

-   [ ] `TestScopeRules` に、設計書 7.1 の表の各行の入力を加える。AC-01 の 6 つの名前と AC-03 の 17 種類の拡張子は、要件書の値から 1 つずつ行にする（1.2）。AC-12 の表の 10 行と、各行を規則 1 の位置にした組（信頼する著者のスレッドと名指しのスレッド）を含める（AC-13）。
-   [ ] `nested_dotfile_not_root_rule`（`:52`）の期待を `false` にする（設計書 7.3）。ほかの既存の行の期待は変えない。
-   [ ] `TestInspectExitCode`（`main_test.go:495`）に、`testFile` と同じディレクトリの、許可リストに当たらないファイル（たとえば `.ts`）と、指示ファイル（たとえば `CLAUDE.md`）の行を加え、終了コード `3` と `outOfScope` の要素を確かめる（1.4）。

**壊して確かめる**: 設計書 7.2 の表の 1〜7 行の壊し方で、それぞれ「失敗するはずの行」が失敗すること。加えて、順 6 の照合を外すと `TestInspectExitCode` の許可リストの行が、指示ファイルの名前のデータから `claude.md` を消すと同じテストの指示ファイルの行が、失敗すること。

**完了条件**: `go test -tags test ./internal/fixprguard ./cmd/fixprguard -run 'TestScopeRules|TestInspectExitCode'` が通る。

#### ステップ 1-4: ガイド 6 章

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`

-   [ ] 日本語版の 6 章を、設計書 3.5「ガイド 6 章」の表のすべての行のとおり改める。実行関連ファイルの表と許可リストの表は、設計書 3.5 の例の構成にし、項目は `ExecutionFilePatterns`・`AllowListPatterns` の出力と突き合わせる。判定の順序は、ステップ 1-2 の `inScopeOf` と突き合わせる。
-   [ ] 6 章の用語を「実行関連ファイル」にそろえ、初出に「コマンドの実行に関わるファイル（実行関連ファイル）」と書く（設計書 3.5 の「用語」の行）。7 章・9 章の用語はフェーズ 2・3 でそろえる。
-   [ ] 日本語版をコミットしてから、英語版を翻訳する（1.2）。

**完了条件**: 両方の版の 6 章が、設計書 3.5「ガイド 6 章」の表のすべての行のとおりになっている（表の項目はステップ 1-5 のガードで確かめる）。

#### ステップ 1-5: ガイドの表のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] 6 章から表を切り出す関数と、そのテストを加える（1.4）。
-   [ ] `TestGuideCoversStopKindsAndScope` を設計書 3.5「文書のガード」のとおり改める。`ExecutionFilePatterns` の項目は実行関連ファイルの表に、`AllowListPatterns` の項目は許可リストの表にあり、`*.go` が実行関連ファイルの表にないことを、両方の版で確かめる。`stop` の種類は今までどおりガイドの全体から探す。テストのコメントも改める。

**壊して確かめる**: 次で、それぞれ `TestGuideCoversStopKindsAndScope` が失敗すること。一時的に書き換えて確かめ、戻す。

-   どちらかの版の許可リストの表から `*.txt` を消す。
-   どちらかの版の実行関連ファイルの表に `*.go` を置く。
-   実行関連ファイルの表から 1 つの項目を消し、同じコードスパンを表の外の文に残す。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

### フェーズ 2: `Protocol` の番号（設計書 8 章の 2）

#### ステップ 2-1: `Protocol` を `3` に上げる

ガードが `Protocol` と文書の番号の一致を求めるので、このステップは 1 つのコミットにする（設計書 3.6「同じ commit に入れるもの」）。ガイド 9 章の文の改訂は、番号と別のコミットにしてよい。

**変更するファイル**: `internal/fixprguard/types.go`、`.claude/commands/fixpr.md`、`.claude/commands/runpr.md`、`docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`、`docs/tasks/0032_japanese_pr_title/check_rules.sh`

-   [ ] `Protocol` を `3` にし、コメントに判定の規則を加える（設計書 3.6「上げる値」）。
-   [ ] 設計書 3.6 の表の箇所の番号を、すべて `3` にする。そのうえで、設計書 3.6 の 2 つの正規表現でリポジトリ全体を `git grep` し、`2` が残るのが設計書 3.6「変えないもの」の箇所だけであることを確かめる（設計書 3.6「番号を固定している箇所の探し方」）。
-   [ ] `sh docs/tasks/0032_japanese_pr_title/check_rules.sh fixpr_title` が通ることを確かめる。
-   [ ] ガイド 9 章（日本語版をコミットしてから英語版）を、設計書 1.2 の 4 つめの項目と 3.5「ガイド 9 章」のとおり改める。番号を上げる条件に「範囲の規則を変える場合」を加え、一覧の更新の項目に許可リストを加えて、用語を「実行関連ファイル」にする。

**壊して確かめる**: ガイドの `--protocol 3` の 1 か所を一時的に `2` に戻して、`TestDocumentedProtocolMatches` が失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 2-2: PR-1 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。
-   [ ] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行する。
-   [ ] 再インストールの前のツールで、PR-1 のブランチの `fixpr.md` に書いた形の `--protocol 3` の呼び出しが終了コード `2` で止まることを確かめる（AC-16 の `manual`。1.3.2）。
-   [ ] PR を開く前に、0034 の状態を確かめ直す（設計書 3.8）。0034 の実装が `main` に入っていたら、止まって利用者と決め直す。

**完了条件**: 上の 4 つが済んでいる。

### PR-1 作成ポイント: scope rules with deny and allow lists

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2

**推奨タイトル**: `feat(0036): fixprguard の範囲の規則に拒否リストと許可リストを入れ、下の階層の指示ファイルと Go のビルドの入力を範囲外にする`

**レビュー観点**: 判定の順序（設計書 3.3 の順 1〜6）と大文字小文字の扱いの組み合わせで範囲が広がらないこと / テストの行が、片方の層だけが働く入力を使い、要件書の値から作られていること / ガイド 6 章の 2 つの表とガードが `ExecutionFilePatterns`・`AllowListPatterns` と一致し、表の外の文では通らないこと / `Protocol` 3 とコマンドの文書・ガイド・`check_rules.sh` の番号、`/runpr` の前の再インストール

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-2 は、0030 の守りの層 (3) である範囲の規則を変えるセキュリティのゲートの変更である（`mkplan.md` step 8 の panel mode の契機）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**`/runpr` の前**: 利用者は、PR-1 のブランチの作業ツリーで `make install-fixprguard` を実行してから `/runpr` を実行する（設計書 3.6「マージと入れ直しの順序」）。マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は終了コード `2` で止まる。

**マージの後**: 利用者は、マージの後の `origin/main` を含む作業ツリーで `make install-fixprguard` を実行し直す。作業中のほかのブランチには、次の `/fixpr`・`/runpr` の前に `main` を取り込む。

### フェーズ 3: 残余のリスク・用語集・0030 の文書（設計書 8 章の 3）

#### ステップ 3-1: ガイド 7.2 と 8 章

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`

-   [ ] 日本語版の 7.2 に、設計書 3.5「ガイド 7.2」の項目（要件書 4.2 の 4 項目と、設計書 5.3 が加える 5 項目）を加える。今の `.gitignore` の項目（`:255`）は残す。
-   [ ] 日本語版の 8 章の段落（`:281`）に、設計書 3.5「ガイド 8 章」の内容を加える。
-   [ ] 7 章・8 章に「コマンドの実行に関わるファイル」「実行に関わるファイル」が残っていないことを確かめる（6 章の初出の併記を除く）。
-   [ ] 書いた各項目を、設計書 5.3 と、PR-1 の `scope.go` の振る舞い（`TestScopeRules` の対応する行）と突き合わせる。
-   [ ] 日本語版をコミットしてから、英語版を翻訳する（1.2）。

**完了条件**: 両方の版の 7.2 と 8 章が、設計書 3.5 のとおりになっている（7.2 の 4 項目はステップ 3-4 のガードで確かめる）。

#### ステップ 3-2: 用語集

**変更するファイル**: `docs/translation_glossary.md`

-   [ ] 設計書 3.7 の表の 5 つの用語を、fixprguard の用語のまとまり（`:74-75` の近く）に加える。訳語と説明は、PR-1 でガイドと `ExecutionFilePatterns`・`AllowListPatterns` のコメントに使った語と突き合わせる。

**完了条件**: 5 つの用語の行がある（ステップ 3-4 のガードで確かめる）。

#### ステップ 3-3: 0030 の文書の注記

**変更するファイル**: `docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md`、`01_requirements.md`

-   [ ] 設計書 3.7「0030 の文書」の 2 か所に、範囲の規則が本タスクで変わったこと、`.go` の規則が許可リストの (c) になったこと、本タスクの要件書と設計書への参照を、短い引用ブロックで加える。規則の本文は書き換えない。
-   [ ] 両文書の `Status` を `approved` のままにし、`Comments` に `YYYY-MM-DD (editorial): …` の形で、決定を変えない注記であることを記録する（設計書 3.7）。日付は注記を加えた日にする。
-   [ ] 注記の内容を、ガイド 6 章（PR-1）と突き合わせる。

**完了条件**: 2 か所に注記があり、`Comments` に記録がある（ステップ 3-4 のガードで確かめる）。

#### ステップ 3-4: 文書のガード

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`、`cmd/fixprguard/docs_test.go`

-   [ ] AC-21: `TestGlossaryTerms` に、設計書 3.7 の 5 つの用語の日本語と英語の組を加える（1.4）。日本語のリテラルは用語集のセルと一致させる（1.2）。
-   [ ] AC-20: 両方のガイドの 7.2 に、要件書 4.2 の残余のリスクの 4 項目を述べる語があることを確かめるガードを加える。
-   [ ] AC-18: 0030 の 2 か所（`02_architecture.md` の 3.6、`01_requirements.md` の F-004）のそれぞれに、本タスクの文書への参照を含む注記があり、両文書の `Status` が `approved` で、`Comments` に `(editorial)` と本タスクの記録があることを確かめるガードを加える。

**壊して確かめる**: 次で、それぞれのガードが失敗すること。一時的に書き換えて確かめ、戻す。

-   用語集から 5 つの用語の行を 1 つずつ消す。
-   どちらかの版の 7.2 から 4 項目を 1 つずつ消す。同じ文を 7.1 に移す（節の取り出しが 7.2 で止まることの確かめ）。
-   0030 のどちらかの文書の注記を消す。注記を別の節に移す。`Status` を `draft` にする。`Comments` の記録を消す。

**完了条件**: 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。

#### ステップ 3-5: PR-2 の確かめ

**変更するファイル**: なし

-   [ ] `make fmt`、`make test && make lint` を実行する。

**完了条件**: 上が通る。

### PR-2 作成ポイント: residual risks, glossary, and task 0030 notes

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `docs(0036): fixprguard のガイドに範囲の規則の残余のリスクを書き、用語集と 0030 の文書を合わせる`

**レビュー観点**: ガイド 7.2 の各項目が、PR-1 の規則の振る舞いと設計書 5.3 に一致すること（ガードは語の有無だけを確かめる） / 0030 の注記が規則の本文を書き換えず、`Status` を保って `Comments` に editorial と記録したこと / 用語集の訳語が、ガイドの英語版とコードのコメントの語と一致すること

**実装モデル要件**: standard

**判定理由**: 文書と、その記述を確かめるガードだけの変更で、`mkplan2.md` step 4 の契機のどれにも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の目安 |
|---|---|---|
| M1 | 0034 への申し送り | ステップ 1-1 の完了条件 |
| M2 | 新しい範囲の規則とテスト | ステップ 1-3 の完了条件 |
| M3 | ガイド 6 章と表のガード | ステップ 1-5 の完了条件 |
| M4 | `Protocol` 3 と文書の番号 | PR-1 のマージ |
| M5 | ガイド 7.2・8 章、用語集、0030 の注記とガード | PR-2 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 | 0034 への申し送り、`scope.go` の拒否リスト・許可リスト・判定の順序、`TestScopeRules`・`TestInspectExitCode` の行、ガイド 6 章と表のガード、`Protocol` 3 と文書の番号、ガイド 9 章 | frontier-required |
| PR-2 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | ガイド 7.2・8 章、用語集、0030 の注記、AC-18・AC-20・AC-21 のガード | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。

-   0034 への申し送り（1-1）は、0034 の設計が始まる前に届くように、最初に置く（1.4）。
-   規則の変更（1-2）は、テスト（1-3）とガイドの表（1-4・1-5）の前提である。`ExecutionFilePatterns` の出力が変わるとガードがガイドとの一致を求めるので、ガイドの表は同じフェーズに入れる（設計書 8 章の 1）。
-   `Protocol` の引き上げ（2-1）は、規則の変更と同じ PR に入れる。別の PR にすると、規則が変わってから番号が上がるまで、入れ直していないツールが古い規則で黙って判定する（設計書 8 章の後の段落）。
-   フェーズ 3 は、PR-1 の規則とガイド 6 章の記述を下敷きにするので、最後に置く。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の表の各行を、ステップ 1-3 で `TestScopeRules` に入れる。テストはネットワークも外部コマンドも使わない。設計書 7.2 の壊し方は、ステップ 1-3 と 1-5 の「壊して確かめる」に割り当てた。

### 4.2. 結合テスト

範囲の判定は `inspect` の中の純粋な関数なので、新しい結合テストは加えない（設計書 7.5）。コマンドの出力の形は、既存の `TestInspectExitCode` に新しい規則の行を加えて確かめる（1.4）。`TestInspectListsOutOfScope`（`worktree_test.go:51`）は変えない。

### 4.3. セキュリティテスト

設計書 5.2 の表の各経路は、ステップ 1-3 の行で確かめる（設計書 7.6）。拒否リストを大文字小文字の違いで逃れる入力は H-06 の `internal/x/Claude.MD` の行で、許可リストに大文字小文字の違いで入り込む入力は AC-09 の行で確かめる。

### 4.4. 文書のガード

ステップ 1-5・3-4 のガードは `cmd/fixprguard/docs_test.go` と `cmd/yt2column/fixpr_docs_test.go` に置き、CI の `doc-guard` のジョブが実行する。ガードが確かめるのは記述の有無と構造だけなので、中身が正しいことは PR のレビューで読み合わせる（設計書 7.4）。

### 4.5. 後方互換性

本タスクの前のツールと新しい文書、新しいツールと前の文書の組み合わせは、`--protocol` の不一致で終了コード `2` で止まる（設計書 3.6）。既存の `TestProtocolMismatch`（`cmd/fixprguard/main_test.go:43`）が、`Protocol` と違う番号を拒否することを確かめる。インストール済みのツールでの確認は、ステップ 2-2 で行う。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は振る舞いを確かめる Go のテスト、`static` は文書を読む Go のガードのテストかコミット済みのスクリプト、`manual` は PR の上での観察を表す。新しいテストの名前は仮の名前で、実装で変えたときは本書の表を同じ PR で改める（1.3.4）。`scope_test.go` の括弧の中は、設計書 7.1 の表の行を示す。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | test | `internal/fixprguard/scope_test.go::TestScopeRules`（指示ファイルの名前の行。6 つの名前ごとの規則 2・3 の位置の `.md` と、規則 1 の位置の組）、`cmd/fixprguard/main_test.go::TestInspectExitCode`（指示ファイルの行） |
| AC-02 | test | `scope_test.go::TestScopeRules`（`.` で始まる要素の行） |
| AC-03 | test | `scope_test.go::TestScopeRules`（17 種類の拡張子ごとの、規則 1 の位置の組） |
| AC-04 | test | `scope_test.go::TestScopeRules`（今の定義の行。1.2 で消した項目の例を含む） |
| AC-05 | test | `scope_test.go::TestScopeRules`（AC-01〜AC-04 の規則 1 の位置の名指しの行） |
| AC-06 | test | `scope_test.go::TestScopeRules`（許可リストの範囲内の行と、`internal/x/testdata/gen.go` の行） |
| AC-07 | test | `scope_test.go::TestScopeRules`（許可リストの外の行）、`main_test.go::TestInspectExitCode`（許可リストに当たらないファイルの行。終了コード `3`） |
| AC-08 | test | `scope_test.go::TestScopeRules`（許可リストの範囲内の行。(a)・(b) は名指しの有無の両方） |
| AC-09 | test | `scope_test.go::TestScopeRules`（`docs/B.MD`・`internal/x/TestData/b.json` の行） |
| AC-10 | test | `scope_test.go::TestScopeRules`（`extension/src/a.ts` の規則 1 の位置の組） |
| AC-11 | test | `scope_test.go::TestScopeRules`（既存のパスの検査・ASCII・変更の種類の行、`:70-84`）、`internal/fixprguard/changes_test.go::TestEnumerateChanges`（既存。名前の変更の分割と、通常のファイル以外の変更） |
| AC-12 | test | `scope_test.go::TestScopeRules`（要件書 AC-12 の表の 10 行） |
| AC-13 | test | `scope_test.go::TestScopeRules`（AC-12 の各行を規則 1 の位置にした組） |
| AC-14 | test | `main_test.go::TestInspectExitCode`（加える 2 行。`outOfScope` の要素の形）、`internal/fixprguard/worktree_test.go::TestInspectListsOutOfScope`（既存） |
| AC-15 | static | `cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract/out_of_scope_stop`（既存。`fixpr.md` の Phase 3.5 の手順を変えないことを確かめる） |
| AC-16 | test + static + manual | `main_test.go::TestProtocolMismatch`（既存）、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`・`::TestDocumentedProtocolMatches`（既存）、`docs/tasks/0032_japanese_pr_title/check_rules.sh fixpr_title`（ステップ 2-1 で実行）。インストール済みの前のツールでの確認（ステップ 2-2） |
| AC-17 | static + manual | `docs_test.go::TestGuideCoversStopKindsAndScope`（表の項目）。説明の文は PR-1 のレビューで読み合わせる |
| AC-18 | static + manual | `docs_test.go::TestTask0030NotesScopeChange`（注記・`Status`・`Comments`）。注記の中身は PR-2 のレビューで読み合わせる |
| AC-19 | static | `docs_test.go::TestGuideCoversStopKindsAndScope`、`docs_test.go::TestGuideTable`（表の切り出しの関数のテスト） |
| AC-20 | static + manual | `docs_test.go::TestGuideListsScopeResidualRisks`。各項目の説明は PR-2 のレビューで読み合わせる |
| AC-21 | static + manual | `cmd/yt2column/fixpr_docs_test.go::TestGlossaryTerms`（5 つの用語の行を加える）。訳語と説明は PR-2 のレビューで読み合わせる |

-   AC-17 の説明の文（3 つの部分の説明、大文字小文字の扱い）は自然言語の記述なので、`manual` で確かめる（設計書 3.5・7.4）。表の項目は AC-19 のガードが確かめる。
-   AC-16 のうち「番号が上がる」ことは、PR-1 の差分で `2` から `3` への変更を見て確かめる（`manual`）。定数をそれ自身のリテラルと比べるテストは書かない（`CLAUDE.md`「Testing Strategy」）。

### 5.1. 横断の検索

`make test`・`make lint` とガードが検出しないものだけを挙げる。

-   [ ] ステップ 2-1 の `git grep`（設計書 3.6 の 2 つの正規表現）で、`2` の番号が設計書 3.6「変えないもの」の箇所にだけ残っている。
-   [ ] ガイドの両方の版で、「コマンドの実行に関わるファイル」（英語版は対応する旧い言い方）が 6 章の初出の併記にだけ残り、ほかは「実行関連ファイル」になっている（設計書 3.5 の「用語」の行）。
-   [ ] `.claude/commands/` と `docs/dev/` に、消した表記（`/.*`・`/claude.md`・`/agents.md`）と「`.go` の規則を実行関連ファイルの一覧に含める」記述が残っていない。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2）
- [ ] PR-2 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 対策 |
|---|---|
| テストの行が、拒否リストと許可リストのどちらでも範囲外になる入力で書かれ、片方の層を外しても通る | 1.2 の層の区別に従い、設計書 7.1 の表の入力を使う。ステップ 1-3 の壊す確認で確かめる |
| テストの行を `scope.go` のデータから作り、データから項目を消すと行も消える | 行を要件書の値から作る（1.2）。ステップ 1-3 の壊す確認で確かめる |
| 判定の順序を誤り、`.go` のファイルに許可リストの (b) が効く | 設計書 7.2 の「順 4 と順 6 を入れ替える」の壊す確認で確かめる |
| ガイドの表のガードが、表の外の文のコードスパンで通る | 表の行だけを切り出す（1.4）。ステップ 1-5 の 3 つめの壊す確認で確かめる |
| `Protocol` の番号の固定箇所の漏れ（CI が実行しない `check_rules.sh` など） | ステップ 2-1 の `git grep` と `check_rules.sh` の実行 |
| PR-1 の `/runpr` が、インストール済みのツールの番号の不一致で止まる | PR-1 の作成ポイントの「`/runpr` の前」のとおり再インストールする。忘れても終了コード `2` で止まるだけで、古い規則では判定しない |
| 0034 の実装が先に `main` に入り、本タスクの定義の変更が 0034 の対象を変える | ステップ 1-1 と 2-2 で確かめ、入っていたら止まって利用者と決め直す（設計書 3.8） |
| 確認が増え、利用者が「続ける」と答えることに慣れる | 設計書 5.3 の残余のリスクとしてガイド 7.2 に書き（ステップ 3-1）、ガイド 6 章の判定の順序で理由を特定できるようにする（ステップ 1-4） |

### 7.2. スケジュールのリスク

-   ステップ 1-1 は利用者の承認を要する。承認を待つ間も、ステップ 1-2 以降は進められる。
-   PR-1 のマージから PR-2 のマージまで、`main` のガイド 7.2 に本タスクの残余のリスクが載っていない（1.4）。PR-2 は PR-1 のマージの後すぐに作る。
-   CodeRabbit の PR レビューは 1 時間あたりの回数が限られるので、PR を開いてからレビューが届くまで待つことがある（`runpr.md` step 3）。

## 8. 成功基準 (Success Criteria)

-   [ ] 5 章のすべての `test` と `static` の検証が通る。
-   [ ] `make test && make lint` と、PR-1・PR-2 の CI の `doc-guard` のジョブが通る。
-   [ ] 各ステップの「壊して確かめる」の結果が、コミットメッセージに書かれている。
-   [ ] 0034 への申し送りが issue #424 にあり、その URL が本書に記録されている。
-   [ ] ガイドの日本語版と英語版の記述が一致する。

## 9. 次のステップ (Next Steps)

-   承認の後、`/runplan 0036` で PR-1 から実装する。PR-1 の `/runpr` の前に、PR-1 のブランチで `make install-fixprguard` を実行する。
-   PR-1 のマージの後、`main` で `make install-fixprguard` を実行し直す。
-   PR-2 は本タスクの最後のプランの PR なので、マージの後に issue #425 を閉じる。
