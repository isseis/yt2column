# 実装計画書：fixprguard の範囲の規則で、下の階層の指示ファイルと Go のビルドの入力を範囲外にする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-22 |
| Review date | 2026-10-22 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-005 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。作業は設計書 8 章の 3 つの項目を 1 つずつフェーズにし、2 つの PR にまとめる（1.4）。

-   **フェーズ 1（設計書 8 章の 1）:** タスク 0034 の状態を確かめて申し送る。`scope.go` の拒否リスト・許可リスト・判定の順序を改め、`scope_test.go` に行を加える。ガイド 6 章（日英）と `TestGuideCoversStopKindsAndScope` を改める。
-   **フェーズ 2（設計書 8 章の 2）:** `Protocol` を `3` に上げ、`fixpr.md`・`runpr.md`・ガイド・`check_rules.sh` の番号と、ガイド 9 章を合わせる。
-   **フェーズ 3（設計書 8 章の 3）:** ガイド 7.2・8 章（日英）、用語集、0030 の文書の注記と、それらの記述を確かめるガード（AC-18・AC-20・AC-21）を加える。

用語は、要件書 6 章と設計書の冒頭の定義に従う（実行関連ファイル、拒否リスト、許可リスト、指示ファイル、Go のビルドの入力、規則 2・3 の位置、規則 1 の位置）。「規則 1〜6」はガイド 6 章の範囲の規則の番号を、「層」は拒否リストと許可リストのそれぞれを指す。許可リストの (a)〜(c) は設計書 1.5 の項目を、「順 1」〜「順 6」は設計書 3.3 の判定の順序を指す。本書の「フェーズ」は実装の区切りを指し、`/fixpr` の「修正のフェーズ」とは別である。「ガイド」は `docs/dev/developer_guide/fixprguard_guide.ja.md`（日本語版）と `fixprguard_guide.md`（英語版）を指す。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの文書（`.claude/commands/`）の文も英語で書く（`CLAUDE.md`「Language」）。例外は、日本語の文書の文字列と一致させる文書のガードのリテラルだけである（先例: `cmd/fixprguard/docs_test.go:256-260`）。
-   テストは、対象の実装を壊して失敗することを確かめてからコミットし、壊し方と結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。各ステップの「壊して確かめる」は、壊す対象と、失敗するはずのテストを挙げる。
-   **片方の層だけが働く入力を使う**（設計書 7.1・H-05）。拒否リストを確かめる行は、許可リストだけなら範囲内になる入力（規則 2・3 の位置の `.md`）か、規則 1 の位置の入力にする。許可リストを確かめる行は、拒否リストに当たらない入力にする。
-   テストの行の値は、`scope.go` のデータからではなく、要件書 AC-01・AC-03 に書かれた値から 1 つずつ作る（設計書 7.1）。
-   ガイドは、日本語版をコミットしてから英語版を `.claude/commands/mktrans.md` の原則で翻訳し、翻訳の後に日本語版と節ごとに突き合わせる（`CLAUDE.md`「Translation Guidelines」）。英語版の用語は設計書 3.7 の表の訳語を使う。用語集への追加はフェーズ 3 だが、訳語は設計書で決まっている。コミットの分け方は 1.4 のとおりである。
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
| `scope_test.go:36-62` | 規則 4 と `.go` の規則の既存の行。これまでは拒否リストだけだったので、規則 2・3 の位置の行は 1 つの層だけを確かめていた。許可リストが入ると、`.md` でも `testdata` の下でもない行は、2 つの層の両方で範囲外になる（`:36-45`・`:47-49`・`:51`・`:53-54`・`:57`）。一方、`.go` の行（`:61-62`）は、拒否リストに当たらず順 4 だけで範囲外になるので、これまでどおり 1 つの層だけを確かめる | 期待は変えず、拒否リストの各項目と順 4 に、片方の層だけが働く行を加える（ステップ 1-3） | 1-3 |
| `cmd/fixprguard/main_test.go:495-527` | `TestInspectExitCode` は、範囲外の行に `other/b.md`（今の規則でも範囲外の場所）だけを使う。出力の要素は `path` と件数だけを確かめる（`:517`、`test_helpers.go:196-201`） | 規則 2 の場所にあり、新しい規則でだけ範囲外になる行を加え、終了コード `3` を確かめる（1.4） | 1-3 |
| `internal/fixprguard/worktree_test.go:51-74` | `TestInspectListsOutOfScope` は `ScopedChange` の全体を比べるが、入力は今の規則でも範囲外のものだけ | 新しい規則でだけ範囲外になるファイルを `want` に加える（1.4） | 1-3 |
| ガイド 6 章（各版 `:204-228`） | 規則 1〜6、`.go` の段落、実行関連ファイルの説明と 6 行の表 | 設計書 3.5「ガイド 6 章」の表のとおり改める | 1-4 |
| `cmd/fixprguard/docs_test.go:45-62` | `TestGuideCoversStopKindsAndScope` がガイドの全体から項目を探す | 設計書 3.5「文書のガード」のとおり改める。表を切り出す関数を加える（1.4） | 1-5 |
| `internal/fixprguard/types.go:17-20` | `Protocol = 2`。コメントは「input and output contract」 | `3` にし、コメントに判定の規則を加える（設計書 3.6） | 2-1 |
| `.claude/commands/fixpr.md`（8 か所）・`runpr.md`（4 か所） | `--protocol 2`（`fixpr.md:44`・`runpr.md:18` はコードブロックの外の文） | `--protocol 3`（設計書 3.6 の表） | 2-1 |
| ガイド 4 章・5 章（各版） | 現在の番号 `2`（`:64`）、`--protocol 2` が 9 か所、`"protocol":2` が 4 か所（`:92`・`:180`・`:192`・`:202`） | `3`（設計書 3.6 の表） | 2-1 |
| `docs/tasks/0032_japanese_pr_title/check_rules.sh:524` | `fixprguard edit-pr --protocol 2 --title-file <title-file>` を求める | `--protocol 3` にする | 2-1 |
| ガイド 9 章（各版 `:287`・`:300`） | 番号を上げる条件は「出力や引数の契約を変える場合」。一覧の更新の項目は「実行に関わるファイル」だけを挙げる | 設計書 1.2・3.5「ガイド 9 章」・3.6 のとおり改める | 2-1 |
| ガイド 7.2（各版 `:244-259`）・8 章（各版 `:281`） | 本タスクの残余のリスクがない | 設計書 3.5「ガイド 7.2」「ガイド 8 章」のとおり加える | 3-1 |
| `cmd/fixprguard/docs_test.go:454-501` | `TestGuideDescribesConfigScope` が、ガイド 7.2（`:496`、`subsection(guide, "### 7.2")`）、1 章の層 (3) の行（`:492`、`"3. **"` で始まる行がちょうど 1 つ）、7.1 の行、8 章の `git-state-changed` の行の語を確かめる | 変えない。改める節（6 章・7.2・8 章）で、このガードの語と「`3. **` で始まる行は 1 つ」を保つ（ステップ 1-4・3-1） | 1-4・3-1 |
| `cmd/fixprguard/docs_test.go:239-336` | `TestGuideDescribesReviewerList` が 9 章の但し書き（`:260`・`:268`）を確かめる | 変えない。ステップ 2-1 で 9 章の隣の文を改めるとき、但し書きを保つ | 2-1 |
| `docs/translation_glossary.md:74-75` の近く | 本タスクの 5 つの用語がない | 設計書 3.7 の表の 5 行を加える | 3-2 |
| 0030 `02_architecture.md:564`・`01_requirements.md:136` | 0030 の時点の規則 | 設計書 3.7「0030 の文書」の注記を加え、`Comments` に記録する | 3-3 |
| `cmd/yt2column/fixpr_docs_test.go:702-735` | `TestGlossaryTerms` が fixprguard の用語の行（`範囲外の変更`・`ガードツール` など）を確かめる | 5 つの用語を加える（AC-21。1.4） | 3-4 |
| `cmd/fixprguard/docs_test.go:408-444` | `passage` と `checkPassages` が、節の語の有無と、消すべき語（`stale`）を確かめる。0030 の文書を読むガードはない | AC-20 は `passage`・`checkPassages` を使ってガイド 7.2 の語を確かめる。AC-18 のガードを加える（1.4） | 3-4 |

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
-   **タスク 0034 の状態（2026-10-22、計画のレビューの時点）:** 要件定義書の PR #429 はマージされた（`09dc151`）。その後、PR #443（ブランチ `isseis/0034-fixprguard-ignored-files-02`、コミット `754bb63`・`554a6fc`）が開き、0034 の要件を改めている。対象を実行関連ファイルの定義の全体から「監視の一覧」に狭め、文書を `draft` に戻す。監視の一覧は Go のビルドの入力を含む。そのため、設計書 3.8 が挙げた具体的な食い違いの一部は、該当しなくなりうる。また、0034 が Go のビルドの入力の一覧を別に持つと、AC-03 の一覧と二重になる。設計もまだない。issue #424 は開いていて、コメントはない。申し送りの中身は、実装のとき（ステップ 1-1）の 0034 の文書から決める。伝える先は issue #424 である（1.4）。

#### 1.3.3. 型の構築の経路

本タスクは新しい型を加えず、型で振る舞いを選ぶ設計でもない。拒否リストと許可リストは非公開のパッケージ変数で、パッケージの外からは書き換えられない。今のデータ（`execNames` など）を参照するのは `scope.go` だけである（`git grep` で確かめた、`d90437c`）。公開するのは、一覧の写しを返す 2 つの関数だけである（設計書 3.4）。そのため、構築の経路のガードは要らない。

#### 1.3.4. 実装時に決めること

-   拒否リストと許可リストのデータの変数名と、許可リストを照合する非公開の関数の名前。
-   `scope_test.go` の行の名前と並べ方。設計書 7.1 の表の各行の入力を使う。
-   ガイドの表を切り出す関数の形（1.4 の方針の範囲で）。
-   AC-18・AC-20 のガードが固定する語と、箇所を取り出す鍵。語は書いた文から選ぶ。鍵は、本タスクで書き換えない見出しやセルを選ぶ。
-   新しいテストの関数の名前。5 章の表に書いた名前と変えたときは、同じ PR で本書の表を改める。

### 1.4. 計画の際に決めたこと（2026-10-22）

-   **PR は 2 つにする（PR-1 = フェーズ 1・2、PR-2 = フェーズ 3）:** レビュアーが選んだ。理由: PR-1 は判定の規則というセキュリティのゲートの変更なので、それだけに絞ってレビューする。設計書 8 章は、フェーズ 1 と 2 を同じ PR に入れることを求める。フェーズ 3 は文書と、その記述を確かめるガードだけの変更である。受け入れる影響: PR-1 のマージから PR-2 のマージまでの間、`main` のガイド 7.2 に本タスクの残余のリスクが載っていない。
-   **0034 への申し送りは、ステップ 1-1 で issue #424 にコメントする:** レビュアーが選んだ。理由: PR #429 はマージされたので、0034 の追跡先は issue #424 である（計画のレビューの時点で、0034 の要件を改める PR #443 が開いている。1.3.2）。GitHub への書き込みなので、実装のときに利用者の承認を得て行う（`CLAUDE.md`「Tool Execution Safety」）。受け入れる影響: ステップ 1-1 より前に 0034 の設計が始まると、その設計は設計書 3.8 を知らないまま書かれうる。
-   **ガイド 8 章の段落は、フェーズ 3 に置く:** 設計書 8 章の優先順位は、ガイド 8 章を挙げていない。ガイド 8 章の段落（設計書 3.5「ガイド 8 章」）は、範囲外の変更が残ったときの利用者の手順であり、ガイド 7.2 の残余のリスクと同じく利用者への案内なので、7.2 と同じフェーズにする。
-   **ガイドの表は、太字の見出しの後の最初の表だけを切り出す（設計書 3.5 が実装計画に委ねた切り出し方）:** 6 章を `section` で切り出し、そこから、目印の太字の見出しの行の後に続く、`|` で始まる行の最初のまとまりを表とする。理由: 見出しから次の見出しまでを切り出すと、表の前後の文に項目のコードスパンがあるだけでガードが通る。表の行だけを読めば、表から項目を落としたときに失敗する。切り出しの関数には、`TestCutSection`（`docs_test.go:506`）にならって、表の終わりで止まること、見出しがないときに空を返すことを確かめるテストを付ける。`cmd/yt2column/docs_test.go:377` の `findTable` は使わない。別の `main` パッケージにあるので import できず、表を見出しの行で探すので、見出しが同じ形（`| 形 | 項目 |`）の 2 つの表を区別できないためである。
-   **AC-21 は、既存の `TestGlossaryTerms`（`cmd/yt2column/fixpr_docs_test.go:702`）に行を加えて確かめる（設計書 7.4 が実装計画に委ねた選択）:** このテストは、fixprguard の用語（`範囲外の変更`・`ガードツール` など）の日本語と英語の組をすでに確かめている。同じ形の確かめを `cmd/fixprguard` に作ると、用語集の読み方が 2 か所に分かれる。
-   **AC-18・AC-20 のガードは `cmd/fixprguard/docs_test.go` に置き、既存の部品を使う:** ガイド 7.2 は、すでに `TestGuideDescribesConfigScope` が `subsection(guide, "### 7.2")` で読んでいる（`docs_test.go:496`、`d90437c`）。AC-20 のガードは、同じファイルの `passage`・`checkPassages`（`:408-444`）で 7.2 の語を確かめる。既存の 7.2 の語（`risk72`。`:473`・`:486`）はそのまま残す。0030 の文書の `Status`・`Comments` を確かめる既存のガードはない（`cmd/` と `internal/` の `*_test.go` を `0030_fixpr_untrusted_threads` で探して確かめた）。節は `section`・`subsection`・`cutSection`・`singleLine`（`:180-217`）で取り出す。0030 の F-004 は `####` の見出しなので、`subsection` では F-005 以降まで続く。`cutSection` に `\n#### ` も終わりとして渡す。
-   **AC-14 の確かめに、既存の 2 つのテストの行を加える:** 既存の範囲外の行は、今の規則でも範囲外の場所である。新しい規則でだけ範囲外になる変更が、今までと同じ形で `outOfScope` に入ることを確かめるため、`TestInspectListsOutOfScope`（`worktree_test.go:51`。`ScopedChange` の全体を比べる）の `want` に、そのようなファイルを加える。終了コード `3` は、`TestInspectExitCode`（`main_test.go:495`）に、`testFile`（`docs/notes/a.md`）と同じディレクトリの、許可リストに当たらないファイルの行と、指示ファイルの行を加えて確かめる。範囲外になった理由を出力しないことは、`ScopedChange` の型（`internal/fixprguard/types.go`）と出力の組み立てを変えないことを PR-1 の差分で確かめる（`manual`）。設計書 7.5 は「結合テストは加えない」とし、AC-14 を既存のテストで確かめるとしていた。既存のテストの表に行を加えるだけで、設計の決定は変えないので、設計書 7.5 を editorial correction として改めた（この計画の PR。設計書の `Comments`）。
-   **フェーズ 1 のコミットの分け方:** 設計書 8 章の 1 は、`ExecutionFilePatterns`・`AllowListPatterns` の変更とガイドの表を同じコミットに入れるとする。英語版のガイドは日本語版のコミットの後に翻訳する（`CLAUDE.md`「Translation Guidelines」）ので、次のとおり分ける。`scope.go`・`scope_test.go` の行・日本語版の 6 章・ガードの改訂を 1 つのコミットにし、英語版の 6 章を次のコミットにする。その間は、英語版を対象にしたガードだけが失敗する。PR は squash merge なので、`main` に失敗するコミットは入らない。設計書 8 章の 1 に、英語版がこの順で続くことを editorial correction として書き足した（この計画の PR）。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-06 は、すべて設計書の付録 A に記録されている。
-   `implementation_handoff.md` はない。

### 1.6. ステップ 1-1 の確かめの結果

確かめた日: 2026-10-22（`origin/main` は `ad45375`）。

-   **0034 の状態:** 要件定義書を改める PR #443 はマージされた（`9f83469`）。`origin/main` の `docs/tasks/0034_fixprguard_ignored_files/` には `01_requirements.md`（`approved`）と `design_handoff.md` だけがあり、設計書も実装もない。0034 の開いている PR はない。issue #424 は開いている。
-   **設計書 3.8 の前提との違い:** 0034 の対象は、実行関連ファイルの定義全体から「監視の一覧」（0034 要件書 3 章）に改められ、0034 は範囲の規則の定義に依らなくなった（0034 要件書 2.3）。そのため、設計書 3.8 の「定義の中身」の 3 つの食い違い（`.vscode/settings.json`・`.DS_Store` とスワップファイル・`extension/node_modules/` の下の `.bin/`）は該当しない。0034 の要件書 4.5 は、Go のビルドの入力の拡張子の一覧を「先に実装するほうが置き、後のほうが再利用する」とした。監視の一覧は `claude.md` と Go のビルドの入力をどの階層でも含むので、設計書 5.3 の「自分自身を無視させる `.gitignore`」のうち、これらのファイルについては 0034 が閉じる。
-   **申し送り:** `Protocol` の `3` は本タスクが使い、0034 は `4` 以上を使うこと、Go のビルドの入力の拡張子は本タスクが `scope.go` の `goBuildInputSuffixes` に置き 0034 が再利用すること、0034 の要件書が引く `scope.go` の行番号が変わること、用語（実行関連ファイル、`.go` の規則の許可リストの (c) への移動）を、利用者の承認を得て issue #424 にコメントした: https://github.com/isseis/yt2column/issues/424#issuecomment-6337255297
-   **ステップ 2-2 の確かめ直し（2026-10-22、`origin/main` は `ad45375` の後に 0031 の計画書の PR #445 だけが入った）:** 0034 の実装は `main` に入っていない。0034 の設計書の PR #447（ブランチ `isseis/0034-fixprguard-ignored-files-03`、`draft`）が開いた。設計書は `Protocol` を `4` にし、Go のビルドの入力の拡張子に本タスクが `scope.go` に置く変数を使い、`scope.go` の行番号が変わることを書いている（同 1.2・3.2・3.9）。申し送りの内容と食い違わないので、追加の申し送りはしない（利用者が確認した）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 範囲の規則とガイド 6 章（設計書 8 章の 1）

#### ステップ 1-1: タスク 0034 の状態の確かめと申し送り

**変更するファイル**: `docs/tasks/0036_fixprguard_exec_file_scope/03_implementation_plan.md`（結果の記録）

-   [x] 利用者の承認を得て `git fetch` と `gh pr list`・`gh pr view` を実行し、`origin/main` の `docs/tasks/0034_fixprguard_ignored_files/` と、0034 の開いている PR（1.3.2 の時点では #443）の文書を読む（`CLAUDE.md`「Tool Execution Safety」）。
-   [x] 0034 の実装が `main` に入っていたら、止まって、0034 が拒否リストをどう使っているかを調べ、扱いを利用者と決め直す（設計書 3.8）。
-   [x] 設計書 3.8 の項目を確認項目の一覧として使い、その時点の 0034 の文書に当てはめて、issue #424 へのコメントの案を日本語で書く。少なくとも次を含める: `Protocol` の `3` は本タスクが使い、0034 は `4` 以上を使うこと。0034 の文書が参照する `scope.go` の行番号が変わること。0034 が Go のビルドの入力の一覧を持つなら、AC-03 の一覧（できれば `scope.go` のデータ）と揃える必要があること。設計書 3.8 への参照も含める。0034 の文書が設計書 3.8 の前提から変わっていたら、違いを本書 1.6 に記録する。
-   [x] コメントの案を利用者に示し、承認を得てから `gh issue comment 424 --body-file <file>` で投稿する（1.4）。
-   [x] 確かめた 0034 の状態と、コメントの URL を本書 1.6 に記録する。記録は実装の記録なので、`Status` は変えない。

**完了条件**: 0034 の実装が `main` に入っていないことを確かめ、コメントの URL が本書に記録されている。

#### ステップ 1-2: 拒否リスト・許可リスト・判定の順序

**変更するファイル**: `internal/fixprguard/scope.go`

-   [x] 拒否リストのデータを設計書 3.1 の表のとおりにし、`.` で始まる要素の照合を加える。照合は今のとおり `strings.ToLower` の後に行う。
-   [x] 許可リストのデータ（設計書 3.2）と、(a)・(b) を大文字小文字を区別して照合する非公開の関数を加える。
-   [x] `inScopeOf` を設計書 3.3 の順 1〜6 にする。`inScope` は変えない。
-   [x] `ExecutionFilePatterns` を設計書 3.4 の項目を出力するように改め、`AllowListPatterns` を加える。コメントは設計書 3.4 のとおりにする。どちらも、判定の関数が照合に使うのと同じパッケージ変数から組み立てる（今の `ExecutionFilePatterns` も `isExecutionRelated` と同じ変数を使っている。`scope.go:60-75`・`:103-126`）。ただし、データを持たない `.*`（`ExecutionFilePatterns`）と `*.go`（`AllowListPatterns`）だけはリテラルで出力する。ガイドのガードは、出力した項目がガイドにあることしか確かめないので、出力からこのリテラルを消しても通ってしまう。そこで、2 つの関数の出力にこの 2 つが含まれることを確かめる小さなテストを `scope_test.go` に置く。
-   [x] ファイルの冒頭のコメント（`:11-28`）とデータのコメント（`:36-38`・`:56-59`）を、新しい規則に合わせて英語で書き直す。

**完了条件**: `go build ./...` が通る（テストの期待はステップ 1-3 で改める）。

#### ステップ 1-3: 範囲の規則のテスト

**変更するファイル**: `internal/fixprguard/scope_test.go`、`internal/fixprguard/worktree_test.go`、`cmd/fixprguard/main_test.go`

-   [x] `TestScopeRules` に、設計書 7.1 の表の各行の入力を加える。AC-01 の 6 つの名前と AC-03 の 17 種類の拡張子は、要件書の値から 1 つずつ行にする（1.2）。AC-12 の表の 10 行と、各行を規則 1 の位置にした組（信頼する著者のスレッドと名指しのスレッド）を含める（AC-13）。
-   [x] 設計書 3.1 の表の拒否リストの項目のうち、片方の層だけが働く行をまだ持たないすべての項目（1.3.1 の `scope_test.go:36-62` の行の項目。`gnumakefile`・`go.sum`・`go.work`・`go.work.sum`・`extension/` の直下の 6 つなど）に、規則 1 の位置の組（信頼する著者のスレッドなら範囲内、名指しなら範囲外）を加える。値は設計書 3.1 の表から取る。
-   [x] 順 4 だけで範囲外になる行（規則 3 の位置の `testdata` の下の `.go` のファイル）と、`.` で始まる要素の規則だけで範囲外になる行（規則 2 の位置の `.` で始まる `.md`）を加える。`nested_dotfile_not_root_rule`（`:52`）の期待は `false` にする（設計書 7.3）。ほかの既存の行の期待は変えない。
-   [x] 新しい照合の境界の行を加える。(b): `testdata` を部分に含むだけの要素、ファイルの名前が `testdata`、`testdata` が深い位置の要素。(a): `.md` に似た拡張子（`.mdx`、`.md.sh` など）。(c): 大文字の `.GO` が同じディレクトリの `.go` の `path` で範囲内になること、`testdata` の下の `.GO` が範囲外になること。
-   [x] `TestInspectListsOutOfScope` の `want` に、新しい規則でだけ範囲外になるファイルを加える（変更の数と既存の要素の `Position` も合わせて改める）。`TestInspectExitCode` に、`testFile` と同じディレクトリの、許可リストに当たらないファイル（たとえば `.ts`）と指示ファイル（たとえば `CLAUDE.md`）の行を加え、終了コード `3` を確かめる（1.4）。

**壊して確かめる**: 次で、それぞれ括弧の中の行が失敗すること。

-   設計書 7.2 の表の 1〜7 行の壊し方（その表の「失敗するはずの行」）。
-   拒否リストのデータから項目を 1 つずつ消す（その項目の規則 1 の位置の名指しの行）。
-   順 4 を外す（`testdata` の下の `.go` の行）。`isGoFile` で大文字小文字を区別する（`testdata` の下の `.GO` の行）。
-   (b) を部分一致で照合する（`testdata` を部分に含む要素の行）。許可リストのデータから `.txt` または `testdata` を消す（AC-08 の行）。
-   許可リストを順 2 より前に照合する（AC-10 の行）。
-   `.` で始まる要素の照合を、ファイルの名前だけにする（`.claude/commands/evil.md`・`internal/.config/foo.md` の行）。ディレクトリの名前だけにする（`.notes.md` の行）。
-   順 6 の照合を外す（`TestInspectExitCode` と `TestInspectListsOutOfScope` の許可リストの行）。指示ファイルの名前のデータから `claude.md` を消す（`TestInspectExitCode` の指示ファイルの行）。

**完了条件**: `go test -tags test ./internal/fixprguard ./cmd/fixprguard -run 'TestScopeRules|TestInspectListsOutOfScope|TestInspectExitCode'` が通る。

#### ステップ 1-4: ガイド 6 章

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`

-   [x] 日本語版の 6 章を、設計書 3.5「ガイド 6 章」の表のすべての行のとおり改める。実行関連ファイルの表と許可リストの表は、設計書 3.5 の例の構成にし、項目は `ExecutionFilePatterns`・`AllowListPatterns` の出力と突き合わせる。判定の順序は、ステップ 1-2 の `inScopeOf` と突き合わせる。
-   [x] 6 章の用語を「実行関連ファイル」にそろえ、初出に「コマンドの実行に関わるファイル（実行関連ファイル）」と書く（設計書 3.5 の「用語」の行）。7 章・9 章の用語はフェーズ 2・3 でそろえる。英語版はすでに "execution-related file" を使っている（`fixprguard_guide.md:211`・`:219`・`:300`）。
-   [x] 判定の順序の箇条書きは、`3. **` で始まる行を作らない形にする（`TestGuideDescribesConfigScope` がその行を 1 つだけ求める。1.3.1）。
-   [x] 日本語版をコミットしてから、英語版を翻訳し、節ごとに突き合わせる（1.2・1.4）。

**完了条件**: 両方の版の 6 章が、設計書 3.5「ガイド 6 章」の表のすべての行のとおりになっている（表の項目はステップ 1-5 のガードで確かめる）。

#### ステップ 1-5: ガイドの表のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [x] 6 章から表を切り出す関数と、そのテストを加える（1.4）。
-   [x] `TestGuideCoversStopKindsAndScope` を設計書 3.5「文書のガード」のとおり改める。`ExecutionFilePatterns` の項目は実行関連ファイルの表に、`AllowListPatterns` の項目は許可リストの表にあり、`*.go` が実行関連ファイルの表にないことを、両方の版で確かめる。`stop` の種類は今までどおりガイドの全体から探す。テストのコメントも改める。

**壊して確かめる**: 次で、それぞれ `TestGuideCoversStopKindsAndScope` が失敗すること。一時的に書き換えて確かめ、戻す。

-   どちらかの版の許可リストの表から `*.txt` を消す。
-   どちらかの版の実行関連ファイルの表に `*.go` を置く。
-   実行関連ファイルの表から 1 つの項目を消し、同じコードスパンを表の外の文に残す。
-   許可リストの表から `*.md` を消し、6 章の表の外の文に `*.md` のコードスパンを残す。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

### フェーズ 2: `Protocol` の番号（設計書 8 章の 2）

#### ステップ 2-1: `Protocol` を `3` に上げる

ガードが `Protocol` と文書の番号の一致を求めるので、このステップは 1 つのコミットにする（設計書 3.6「同じ commit に入れるもの」）。ガイド 9 章の文の改訂は、番号と別のコミットにしてよい。

**変更するファイル**: `internal/fixprguard/types.go`、`.claude/commands/fixpr.md`、`.claude/commands/runpr.md`、`docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`、`docs/tasks/0032_japanese_pr_title/check_rules.sh`

-   [x] `Protocol` を `3` にし、コメントに判定の規則を加える（設計書 3.6「上げる値」）。コメントは、`// Protocol is the version of the tool's input and output contract. The command` / `// documents pass it back with --protocol, so an installed binary that predates` / `// the documents stops instead of judging by older rules.` から、`// Protocol is the version of the tool's input and output contract and of the` / `// scope rules inspect applies. The command documents pass it back with` / `// --protocol, so an installed binary that predates the documents stops instead` / `// of judging by older rules.` に改める（` / ` は行の区切り）。
-   [x] 設計書 3.6 の表の箇所の番号を、すべて `3` にする。そのうえで、設計書 3.6 の 2 つの正規表現でリポジトリ全体を `git grep` し、`2` が残るのが設計書 3.6「変えないもの」の箇所だけであることを確かめる（設計書 3.6「番号を固定している箇所の探し方」）。
-   [x] `sh docs/tasks/0032_japanese_pr_title/check_rules.sh fixpr_title` が通ることを確かめる。
-   [x] ガイド 9 章（日本語版をコミットしてから英語版）を、設計書 1.2 の 4 つめの項目と 3.5「ガイド 9 章」のとおり改める。番号を上げる条件に「範囲の規則を変える場合」を加え、一覧の更新の項目に許可リストを加えて、用語を「実行関連ファイル」にする。`TestGuideDescribesReviewerList` が確かめる但し書きの文（1.3.1）は変えない。英語版は日本語版と節ごとに突き合わせる。9 章の文の中身は PR-1 のレビューで読み合わせる（`manual`）。

**壊して確かめる**: ガイドの `--protocol 3` の 1 か所を一時的に `2` に戻して、`TestDocumentedProtocolMatches` が失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 2-2: PR-1 の確かめ

**変更するファイル**: なし

-   [x] `make test && make lint` を実行する。
-   [x] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行する。
-   [x] 本タスクの前のツールで、PR-1 のブランチの `fixpr.md` に書いた形の `--protocol 3` の呼び出しが終了コード `2` で止まることを確かめる（AC-16 の `manual`。1.3.2）。まず `go version -m "$(command -v fixprguard)"` の `vcs.revision` が PR-1 の前の `origin/main` のコミットであることを確かめる。そうでなければ、PR-1 の前の `origin/main` の `cmd/fixprguard` をスクラッチのディレクトリにビルドして使う。
-   [x] PR を開く前に、利用者の承認を得て 0034 の状態を確かめ直す（設計書 3.8。ステップ 1-1 と同じ読み取り）。0034 の実装が `main` に入っていたら、止まって利用者と決め直す。ステップ 1-1 の後に 0034 の文書が設計書 3.8 にかかわる形で変わっていたら、違いを本書 1.6 に記録し、追加の申し送りの要否を利用者に確かめる。

**完了条件**: 上の 4 つが済んでいる。

### PR-1 作成ポイント: scope rules with deny and allow lists

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2

**推奨タイトル**: `feat(0036): fixprguard の範囲の規則に拒否リストと許可リストを入れ、下の階層の指示ファイルと Go のビルドの入力を範囲外にする`

**レビュー観点**: 判定の順序（設計書 3.3 の順 1〜6）と大文字小文字の扱いの組み合わせで範囲が広がらないこと / テストの行が、片方の層だけが働く入力を使い、要件書の値から作られていること / ガイド 6 章の 2 つの表とガードが `ExecutionFilePatterns`・`AllowListPatterns` と一致し、表の外の文では通らないこと / `Protocol` 3 とコマンドの文書・ガイド・`check_rules.sh` の番号、`/runpr` の前の再インストール

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-2 は、0030 の守りの層 (3) にあたる範囲の規則を変える、セキュリティのゲートの変更である（`mkplan.md` step 8 の panel mode の契機）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/449
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**`/runpr` の前**: 利用者は、PR-1 のブランチの作業ツリーで `make install-fixprguard` を実行してから `/runpr` を実行する（設計書 3.6「マージと入れ直しの順序」）。マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は終了コード `2` で止まる。

**マージの後**: 利用者は、マージの後の `origin/main` を含む作業ツリーで `make install-fixprguard` を実行し直す。作業中のほかのブランチには、次の `/fixpr`・`/runpr` の前に `main` を取り込む。

### フェーズ 3: 残余のリスク・用語集・0030 の文書（設計書 8 章の 3）

#### ステップ 3-1: ガイド 7.2 と 8 章

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`

-   [ ] 日本語版の 7.2 に、設計書 3.5「ガイド 7.2」の項目（要件書 4.2 の 4 項目と、設計書 5.3 が加える 5 項目）を加える。今の `.gitignore` の項目（`:255`）と、`TestGuideDescribesConfigScope` が 7.2 で確かめる語（1.3.1）は残す。
-   [ ] 日本語版の 8 章の段落（`:281`）に、設計書 3.5「ガイド 8 章」の内容を加える。8 章の `git-state-changed` の行は変えず、`3. **` で始まる行を作らない（1.3.1）。
-   [ ] 7 章・8 章に「コマンドの実行に関わるファイル」「実行に関わるファイル」が残っていないことを確かめる（6 章の初出の併記を除く）。
-   [ ] 書いた各項目を、設計書 5.3 と、PR-1 の `scope.go` の振る舞い（`TestScopeRules` の対応する行）と突き合わせる。8 章の段落の中身は PR-2 のレビューで読み合わせる（`manual`）。
-   [ ] 日本語版をコミットしてから、英語版を翻訳し、節ごとに突き合わせる（1.2）。

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
-   [ ] AC-20: 両方のガイドの 7.2 に、要件書 4.2 の残余のリスクの 4 項目を述べる語があることを確かめるガードを、`passage`・`checkPassages` で加える（1.4）。`checkPassages` は `configScope` の 3 つの語を必ず求めるので、確かめる語の一覧を渡せる形に改めて使う。`TestGuideDescribesConfigScope` が確かめる語は変えない。
-   [ ] AC-18: 0030 の 2 か所（`02_architecture.md` の 3.6、`01_requirements.md` の F-004。F-004 は `####` の見出しで止める。1.4）のそれぞれに、本タスクの文書への参照を含む注記があり、両文書の `Status` が `approved` であることを確かめるガードを加える。`Comments` は、`(editorial)` と本タスクへの参照を同じ 1 つの記録に含むことを確かめる（0030 `02_architecture.md` の `Comments` には、ほかのタスクの `(editorial)` の記録がすでにある）。

**壊して確かめる**: 次で、それぞれのガードが失敗すること。一時的に書き換えて確かめ、戻す。

-   用語集から 5 つの用語の行を 1 つずつ消す。
-   どちらかの版の 7.2 から 4 項目を 1 つずつ消す。同じ文を 7.1 に移す。同じ文を 8 章に移す（節の取り出しの始まりと終わりの確かめ）。
-   0030 のどちらかの文書の注記を消す。注記を次の節（3.7、F-005）に移す。`Status` を `draft` にする。本タスクの記録を消す。本タスクの記録の `(editorial)` だけを `(decision)` に替える。

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

範囲の判定は `inspect` の中の純粋な関数なので、新しい結合テストは加えない（設計書 7.5）。新しい規則で範囲外になった変更の形と終了コードは、既存の `TestInspectListsOutOfScope`（`worktree_test.go:51`）と `TestInspectExitCode`（`main_test.go:495`）に行を加えて確かめる（1.4）。

### 4.3. セキュリティテスト

設計書 5.2 の表の各経路は、ステップ 1-3 の行で確かめる（設計書 7.6）。拒否リストを大文字小文字の違いで逃れる入力は H-06 の `internal/x/Claude.MD` の行で、許可リストに大文字小文字の違いで入り込む入力は AC-09 の行で確かめる。

### 4.4. 文書のガード

ステップ 1-5・3-4 のガードは `cmd/fixprguard/docs_test.go` と `cmd/yt2column/fixpr_docs_test.go` に置き、CI の `doc-guard` のジョブが実行する。ガードが確かめるのは記述の有無と構造だけなので、中身が正しいことは PR のレビューで読み合わせる（設計書 7.4）。

### 4.5. 後方互換性

本タスクの前のツールと新しい文書、新しいツールと前の文書の組み合わせは、`--protocol` が一致せず、終了コード `2` で止まる（設計書 3.6）。既存の `TestProtocolMismatch`（`cmd/fixprguard/main_test.go:43`）が、`Protocol` と違う番号を拒否することを確かめる。インストール済みのツールでの確認は、ステップ 2-2 で行う。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は振る舞いを確かめる Go のテスト、`static` は文書を読む Go のガードのテストかコミット済みのスクリプト、`manual` は PR のレビューでの確認を表す。新しいテストの名前は仮の名前で、実装で変えたときは本書の表を同じ PR で改める（1.3.4）。`scope_test.go` の括弧の中は、設計書 7.1 の表の行を示す。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | test | `internal/fixprguard/scope_test.go::TestScopeRules`（指示ファイルの名前の行。6 つの名前ごとの規則 2・3 の位置の `.md` と、規則 1 の位置の組）、`cmd/fixprguard/main_test.go::TestInspectExitCode`（指示ファイルの行） |
| AC-02 | test | `scope_test.go::TestScopeRules`（`.` で始まる要素の行） |
| AC-03 | test | `scope_test.go::TestScopeRules`（17 種類の拡張子ごとの、規則 1 の位置の組） |
| AC-04 | test | `scope_test.go::TestScopeRules`（今の定義の行。設計書 1.2 で消した項目の例と、拒否リストの各項目の規則 1 の位置の組を含む） |
| AC-05 | test | `scope_test.go::TestScopeRules`（AC-01〜AC-04 の規則 1 の位置の名指しの行） |
| AC-06 | test | `scope_test.go::TestScopeRules`（許可リストの範囲内の行、`internal/x/testdata/gen.go` と規則 3 の位置の `testdata` の下の `.go` の行、(a)〜(c) の境界の行） |
| AC-07 | test | `scope_test.go::TestScopeRules`（許可リストの外の行）、`main_test.go::TestInspectExitCode`（許可リストに当たらないファイルの行。終了コード `3`） |
| AC-08 | test | `scope_test.go::TestScopeRules`（許可リストの範囲内の行。(a)・(b) は名指しの有無の両方） |
| AC-09 | test | `scope_test.go::TestScopeRules`（`docs/B.MD`・`internal/x/TestData/b.json` の行） |
| AC-10 | test | `scope_test.go::TestScopeRules`（`extension/src/a.ts` の規則 1 の位置の組） |
| AC-11 | test | `scope_test.go::TestScopeRules`（既存のパスの検査・ASCII・変更の種類の行、`:70-84`）、`internal/fixprguard/changes_test.go::TestEnumerateChanges`（既存。名前の変更の分割と、通常のファイル以外の変更） |
| AC-12 | test | `scope_test.go::TestScopeRules`（要件書 AC-12 の表の 10 行） |
| AC-13 | test | `scope_test.go::TestScopeRules`（AC-12 の各行を規則 1 の位置にした組） |
| AC-14 | test + manual | `internal/fixprguard/worktree_test.go::TestInspectListsOutOfScope`（新しい規則でだけ範囲外になるファイルを加えた `want`。`ScopedChange` の全体を比べる）、`main_test.go::TestInspectExitCode`（加える 2 行。終了コード `3`）。理由を出力しないことは、PR-1 の差分で `ScopedChange` と出力の組み立てが変わらないことを確かめる（1.4） |
| AC-15 | static | `cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract/out_of_scope_stop`・`::TestRunprDocContract/stop_without_merge`（どちらも既存。`fixpr.md`・`runpr.md` の手順を変えないことを確かめる） |
| AC-16 | test + static + manual | `main_test.go::TestProtocolMismatch`（既存）、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`・`::TestDocumentedProtocolMatches`（既存）、`docs/tasks/0032_japanese_pr_title/check_rules.sh fixpr_title`（ステップ 2-1 で実行）。インストール済みの前のツールでの確認（ステップ 2-2） |
| AC-17 | static + manual | `docs_test.go::TestGuideCoversStopKindsAndScope`（表の項目）。説明の文は PR-1 のレビューで読み合わせる |
| AC-18 | static + manual | `docs_test.go::TestTask0030NotesScopeChange`（注記・`Status`・`Comments`）。注記の中身は PR-2 のレビューで読み合わせる |
| AC-19 | static | `docs_test.go::TestGuideCoversStopKindsAndScope`、`docs_test.go::TestGuideTable`（表の切り出しの関数のテスト）、`internal/fixprguard/scope_test.go::TestPatternRuleLiterals`（`.*`・`*.go` の出力。ステップ 1-2） |
| AC-20 | static + manual | `docs_test.go::TestGuideListsScopeResidualRisks`。各項目の説明は PR-2 のレビューで読み合わせる |
| AC-21 | static + manual | `cmd/yt2column/fixpr_docs_test.go::TestGlossaryTerms`（5 つの用語の行を加える）。訳語と説明は PR-2 のレビューで読み合わせる |

-   AC-17 の説明の文（3 つの部分の説明、大文字小文字の扱い）は自然言語の記述なので、`manual` で確かめる（設計書 3.5・7.4）。表の項目は AC-19 のガードが確かめる。ただし、ガードは `ExecutionFilePatterns` の出力だけを読むので、設計書 1.2 でデータから消し、表に例として残す項目（`.tool-versions`・`.claude/`・最上位の `CLAUDE.md` など）が表に載ることも `manual` で確かめる（設計書 3.5 は逆向きのガードを置かない）。
-   AC-16 のうち「番号が上がる」ことは、PR-1 の差分で `2` から `3` への変更を見て確かめる（`manual`）。定数をそれ自身のリテラルと比べるテストは書かない（`CLAUDE.md`「Testing Strategy」）。

### 5.1. 横断の検索

`make test`・`make lint` とガードが検出しないものだけを挙げる。どれも `manual` で、実行した検索とその結果を、そのステップのコミットメッセージに書く。

-   [x] ステップ 2-1 の `git grep`（設計書 3.6 の 2 つの正規表現）で、`2` の番号が設計書 3.6「変えないもの」の箇所にだけ残っている。
-   [x] ガイドの日本語版で、「コマンドの実行に関わるファイル」と「実行に関わるファイル」が 6 章の初出の併記にだけ残り、ほかは「実行関連ファイル」になっている（設計書 3.5 の「用語」の行）。英語版は今の "execution-related file" のままでよい。
-   [x] `.claude/commands/` と `docs/dev/` に、消した表記（`/.*`・`/claude.md`・`/agents.md`）と「`.go` の規則を実行関連ファイルの一覧に含める」記述が残っていない。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2）
- [ ] PR-2 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 対策 |
|---|---|
| テストの行が、拒否リストと許可リストのどちらでも範囲外になる入力で書かれ、片方の層を外しても通る。既存の規則 4 の行も、許可リストが入るとこうなる（1.3.1） | 1.2 の層の区別に従い、設計書 7.1 の表の入力を使い、拒否リストの各項目に規則 1 の位置の組を加える。ステップ 1-3 の壊す確認で確かめる |
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
