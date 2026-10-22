# 実装計画書：fixprguard の config の比較から、ほかのブランチの `branch.<name>.*` を外す

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

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-004 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。作業は設計書 8 章の 4 つの項目を 1 つずつフェーズにし、2 つの PR にまとめる（1.4）。

-   **フェーズ 1（設計書 8 章の 1）:** 設定の値の一覧の分解・分類・ダイジェストを、外部のコマンドを起動しない関数として `configlist.go` に置く。
-   **フェーズ 2（設計書 8 章の 2）:** 基準値の版、場所と種類のダイジェスト、`CheckState` と `Baseline` の順序、一覧の読み取りの失敗の振り分けを変え、`Protocol` を上げる。結合テストと既存のテストの改訂を行う。
-   **フェーズ 3（設計書 8 章の 3）:** ガイド（英語・日本語）を改め、ガイドの記述と番号を確かめるガードを加える。
-   **フェーズ 4（設計書 8 章の 4）:** 0030 の `01_requirements.md`・`02_architecture.md` を改め、その記述を確かめるガードを加える。

用語は、要件書 3.1 と設計書の冒頭の定義に従う（設定の値の一覧、外す対象のブランチの設定、項目、比べる項目、基準値のブランチ、場所と種類のダイジェスト、一覧のダイジェスト）。本書の「フェーズ」は実装の区切りを指し、`/fixpr` の「修正のフェーズ」とは別である。「ガイド」は `docs/dev/developer_guide/fixprguard_guide.md` と日本語版の `fixprguard_guide.ja.md` を指す。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの文書（`.claude/commands/`）の文も英語で書く（`CLAUDE.md`「Language」）。例外は、日本語の文書の文字列と一致させる文書のガードのリテラルだけである（先例: `cmd/yt2column/docs_test.go:738`・`:747`）。
-   テストは、対象の実装を壊して失敗することを確かめてからコミットし、壊し方と結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」、ガイド 9 章）。各ステップの「壊して確かめる」は、壊す対象と、失敗するはずのテストを挙げる。
-   分類のテストの入力は、分類を外すと結果が変わるものにする。外す対象の項目を変えた入力で、まず分類なしのダイジェスト（出力全体の SHA-256）が変わることを確かめ、そのうえで分類ありのダイジェストが変わらないことを確かめる（`CLAUDE.md`「A layered path needs inputs only one layer can handle」、設計書 7.1）。場所と種類の比較と一覧の比較の区別は、起動した git の記録で行う（設計書 H-07 の対応）。
-   Go のファイルを変えたら `make fmt` を実行する。各フェーズの最後に `make test && make lint` を実行する。
-   ソースのコメントに `AC-NN`・`F-NNN` を書かない（要件書との対応は本書 5 章に書く）。

### 1.3. 既存コード調査結果

調査はコミット `82598be` で行った。設計書が確かめたコミット `b8038b4` から、`internal/`・`cmd/`・`.claude/`・`docs/dev/`・`docs/tasks/0030_fixpr_untrusted_threads/`・`CLAUDE.md` に変更はない（`git diff --stat b8038b4 82598be -- docs/tasks/0030_fixpr_untrusted_threads internal cmd .claude docs/dev CLAUDE.md` の出力が空）。そのため、設計書の `file:line` は `82598be` でもそのまま使える。以下の `file:line` も `82598be` のものである。

#### 1.3.1. 変える箇所の一覧

洗い出しは、`rg -n -g '*.go'` で `PartConfig`・`configDigest`・`configListDigest`・`fileDigests`・`baselineVersion`・`"v1`・`gitCalls`・`Protocol` を、`rg -n` で `.claude/`・`docs/dev/` の `--protocol 1`・`"protocol":1`・`'v1.` を探して行った。

| 箇所 | 今 | 変え方 | ステップ |
|---|---|---|---|
| `internal/fixprguard/configlist.go`・`configlist_test.go` | ない | 新規。`mainBranch`・`configListDigest`（設計書 3.2・3.3） | 1-1 |
| `internal/fixprguard/errors.go:52-54` | 非公開のエラー値は `errNonceExhausted`・`errUnknownEnum`・`errThreadCap` | `errConfigList` を加える（設計書 3.3） | 1-1 |
| `internal/fixprguard/gitstate.go:30` | `baselineVersion = "v1"` | `"v2"` にする（設計書 3.1） | 2-2 |
| `internal/fixprguard/gitstate.go:279-298` | `configDigest` が場所と、2 つの config のファイルの種類と内容を `digestPath` で含める | `gitDirsDigest` に改め、ファイルの種類だけを含める（1.3.3） | 2-2 |
| `internal/fixprguard/gitstate.go:472-482` | `fileDigests` が `configDigest` を呼ぶ | `gitDirsDigest` を呼ぶ。エラーの規則の名前は変えない | 2-2 |
| `internal/fixprguard/gitstate.go:484-494` | メソッド `Worktree.configListDigest` が `w.git` で出力全体の SHA-256 を返す | 出力を得るだけのメソッド `Worktree.configList` に改め、エラーは `cmdrunner` のものをそのまま返す（設計書 3.3） | 2-2 |
| `internal/fixprguard/gitstate.go:514-560` | `CheckState`。一覧の読み取りの失敗は `w.git` の `gitFailure` で `git-failed` | 一覧のダイジェストを `configListDigest` で求め、失敗を設計書 4 章の表の順で振り分ける。コメントを改める（設計書 3.4・3.5） | 2-2 |
| `internal/fixprguard/gitstate.go:695-731` | `Baseline` は `:704` で一覧のダイジェストを求める | `:704` の位置で `configList` を起動し、ダイジェストは `headState` の後に `digestString(branch)` を渡して求める（設計書 3.4） | 2-2 |
| `internal/fixprguard/gitstate.go:58-71`・`:98-104` | `gitHardening` のコメント（「Every other setting … held to the baseline」「git 2.5 and later」）、`Baseline` の型のコメント | 設計書 3.5「コードのコメント」のとおり改める。git の版は 2.12 | 2-2 |
| `internal/fixprguard/types.go:20` | `Protocol = 1` | `2` にする（AC-19） | 2-2 |
| `.claude/commands/fixpr.md:44`・`:241`・`:360`・`:463`・`:506`・`:527`・`:591`・`:638`、`runpr.md:18`・`:127`・`:247`・`:398` | `--protocol 1`（`fixpr.md:44` と `runpr.md:18` はコードブロックの外の文） | すべて `--protocol 2` にする | 2-2 |
| `cmd/fixprguard/main_test.go:25` | `baseline = "v1.x"`（解析だけに使う） | `"v2.x"` にする（設計書 7.3） | 2-2 |
| `cmd/fixprguard/test_helpers.go:358-369` | `validBaseline` が `"v1"` で始まるトークンを作り、`ParseBaseline` が受け付けなければテストの準備で失敗する | `"v2"` にする。設計書 2.2 の表にないので、設計書を editorial correction で直す（1.4） | 2-2 |
| `internal/fixprguard/test_helpers_git.go:36-57` | `gitRouter.Calls` は git のサブコマンドの名前だけを記録する（`git config --list` と `git config --get` が同じ `git config`）。すべての git を実物に渡す | `git config --list` を区別して記録する。`git config --list` だけを指定したエラーで失敗させる差し替えを加える（設計書 2.2） | 2-1 |
| `internal/fixprguard/gitstate_test.go`・`worktree_flow_test.go` | 設計書 7.3 の表の 6 行（`gitstate_test.go:132-134`・`:238`・`:266-276`・`:277-293`、`worktree_flow_test.go:285-313`・`:422-424`・`:440-442`） | 設計書 7.3 のとおり改める | 2-2 |
| `cmd/fixprguard/docs_test.go:84-116` | `TestDocumentedInvocationsParse` はコードブロックの中の呼び出しだけを読む。ガイドの番号と、コードブロックの外の `--protocol` は確かめない | `--protocol` の番号を確かめるガードを加える（1.4） | 3-2 |
| ガイドの日本語版と英語版 | 設計書 3.5 の表の箇所。`--protocol 1` は各 9 か所（`:82`・`:92`・`:113`・`:136`・`:146`・`:158`・`:170`・`:182`・`:192`）、`"protocol":1` は各 4 か所（`:85`・`:173`・`:185`・`:195`）、`'v1....'` は各 3 か所（`:146`・`:158`・`:170`）、現在の番号は `:57` | 設計書 3.5 のとおり改める | 3-1 |
| `docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md`・`02_architecture.md` | 設計書 3.5「0030 の文書」の表の箇所 | 設計書 3.5 のとおり改める | 4-1 |

変えない箇所は次のとおりである。

-   `cmd/fixprguard` のコード（設計書 2.2）。`CheckState` の呼び出し（`cmd/fixprguard/main.go:557`、`worktree.go:130`・`:243`・`:273`・`:326`・`:367`・`:383`・`:502`）は、そのまま新しい比較を使う。
-   `infoDigest`・`hooksDigest` は、今のとおりファイルの内容を含める（設計書 2.3 の表で変わるのは config だけ）。
-   `docs/dev/developer_guide/package_reference.md`。`internal/fixprguard`・`cmd/fixprguard` の行は config の比較の範囲を述べていない（`:14`・`:21`）。
-   `docs/tasks/0033_multiple_trusted_reviewers/` の文書の `--protocol 1`。設計書 9 章のとおり、0033 の実装の PR がその時点の番号に合わせる。

#### 1.3.2. 名前を変えるものの一覧

| 今の名前 | 新しい名前 | 場所 | ステップ |
|---|---|---|---|
| 関数 `configDigest` | `gitDirsDigest` | `internal/fixprguard/gitstate.go` | 2-2 |
| メソッド `Worktree.configListDigest` | `Worktree.configList` | `internal/fixprguard/gitstate.go` | 2-2 |
| （なし） | 関数 `configListDigest`（新規） | `internal/fixprguard/configlist.go` | 1-1 |

新しい関数 `configListDigest` は、今のメソッドと同じ名前である（設計書 3.3）。Go では、パッケージの関数と型のメソッドが同じ名前を持てるので、フェーズ 1 の間は両方が並ぶ。ステップ 2-2 でメソッドを `configList` に改めた後は、`rg -n 'w\.configListDigest|\) configListDigest' internal/fixprguard` が何も返さないことで、古いメソッドが残っていないことを確かめる。古い関数 `configDigest` は、ビルドで確かめる。本書の各ステップ、5 章の表、6 章のチェックリストは、この表の新しい名前で書く。

#### 1.3.3. 場所と種類のダイジェストの作り方

今の `digestPath`（`gitstate.go:375-409`）は、`configDigest`・`infoDigest`・`digestEntry`（hooks）が共有し、通常のファイルの内容の SHA-256 を書く。`gitDirsDigest` だけが内容を除くので、`digestPath` を、種類を書く部分と内容を書く部分に分け、`gitDirsDigest` は種類の部分だけを使う。`infoDigest` と hooks は今の書き方を保つ。info と hooks の内容の比較が保たれることは、既存の `TestBaselineDetectsChange` の `info_exclude`・`info_attributes`・`hook_link_target_content` の行が変更なしで通ることで確かめる。

構築の経路について: 本タスクは新しい型を加えない。`Baseline` の構築は、今のとおり `newBaseline` だけで行い（`gitstate.go:115-119`）、既存の `TestRestrictedTypesBuiltByConstructors`（`construction_test.go`）が守る。`configListDigest` の基準値のブランチのダイジェストは、構造体でなく引数で渡し、関数の中で形を確かめる（設計書 3.3）。

#### 1.3.4. 外部の前提の確かめ

-   **手元の版:** git 2.50.1（Apple Git-155）、`gh` 2.96.0（`git --version`・`gh --version`、2026-10-22）。設計書 1.3 の確認と同じ版である。
-   **取り消しとタイムアウトのエラー:** `cmdrunner` の `Run` は、終了コードのある失敗のとき、`ctx.Err()` があれば `errors.Join(ctxErr, *ExitError)` を返す（`internal/cmdrunner/runner.go:96-108`）。出力の上限を超えたときは `ErrOutputTooLarge` を包んだエラーを返し、終了コードを見ない（`:92-94`）。設計書 4 章の判定の順序はこれに基づく。
-   **親の `context`:** `cmd/fixprguard` は `context.Background()` から始め、期限を付けない（`cmd/fixprguard/main.go:146`）。`context.DeadlineExceeded` は、`gitRaw` が付ける `gitTimeout`（`gitstate.go:444-448`）からしか生じない。
-   **lint とテストのみから使う関数:** `make lint` は `golangci-lint run --build-tags test,integration` を実行する（`Makefile:10`・`:208-210`）。`.golangci.yml` は `run.tests` を指定しないので、既定の `true` でテストのファイルも読む。フェーズ 1 の `configListDigest` はテストからだけ使われるが、`unused` に報告されない。
-   **FIFO の作り方:** テストでは `syscall.Mkfifo` を使う。先例は `internal/fixprguard/changes_test.go:113`・`:178` である。
-   **ネットワークを要する確かめ（I-01・I-02）と git の最低の版:** ステップ 2-4 で、実装のときに利用者の承認を得て行う（`CLAUDE.md`「Tool Execution Safety」）。

#### 1.3.5. 実装時に決めること

-   `configListDigest` の中の関数の分け方と、テストの表の行。
-   `gitRouter` の記録の形（`git config --list` を何と書くか）と、差し替えの API の形。
-   `CheckState` の振り分けを書く場所（`configList` の結果を受け取る非公開の関数にするか、`CheckState` の中に書くか。設計書 3.3 は「`CheckState` の中の 1 か所」とだけ定める）。
-   ガードのテストが固定する語。書いた文から選ぶ。
-   新しいテストの関数の名前。5 章の表に書いた名前と変えたときは、同じ PR で本書の表を改める。

### 1.4. 計画の際に決めたこと（2026-10-22）

-   **PR は 2 つにする（PR-1 = フェーズ 1〜3、PR-2 = フェーズ 4）:** 理由: ガイド 7.1 の保証の書き換えは、比較の振る舞いの変更と同じ PR でレビューしたほうが、食い違いを見つけやすい。ガイドの `--protocol` の番号はガードで `Protocol` と結ぶので（下の項目）、`Protocol` を上げる PR に入れなければ `main` でガイドとツールが食い違う。フェーズ 1 だけを先にマージする案は、振る舞いを変えない関数が 1 度だけ使われずにマージされるので採らない。0030 の文書は、ガイドの改訂を下敷きにする文書だけの変更なので、別の PR にする。
-   **I-02 は、通常の PR だけを実際に確かめる:** fork の PR は確かめない。理由: 設計の安全性はこの確認によらず、外れても止まりすぎる方向だけである（設計書 1.3 の表の後の段落）。fork の PR の確認には、公開リポジトリの大きな clone とネットワークを要する。確かめていないことは、ガイド 7.2 に書く（ステップ 3-1）。
-   **PR-1 の `/runpr` の前に、PR-1 のブランチで `make install-fixprguard` を実行する:** PR-1 のブランチの `fixpr.md`・`runpr.md` は `--protocol 2` を渡すので、インストール済みのツールでは終了コード `2` で止まる。再インストールの後、マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は終了コード `2` で止まる（fail-closed）。PR-1 の作成ポイントと 7 章に書く。
-   **ガイドの番号は、ガードのテストで確かめる（設計書 7.5 の 1 つめの項目）:** `cmd/fixprguard/docs_test.go` に、ガイドの現在の番号、ガイドとコマンドの文書のすべての `--protocol N`（コードブロックの外の文を含む）、ガイドの出力の例の `"protocol":N` が `Protocol` と一致することを確かめるテストを加える。理由: 今の `TestDocumentedInvocationsParse` はコードブロックの中しか読まないので、`fixpr.md:44`・`runpr.md:18` の文とガイドの 17 か所が食い違っても通る。文書にだけ書いた確認は実行されない（`mkplan.md` step 6）。
-   **AC-17・AC-18 も、ガードのテストで確かめる:** 対象の記述ごとに、(a)〜(c) を述べる語が含まれることを確かめる。先例は 0033 の `TestGuideDescribesReviewerList` の方法（表の行や節を取り出し、書いた文から選んだ語を確かめる）と、タスクの文書を読む `cmd/yt2column/extension_docs_test.go` である。
-   **設計書 2.2 の表に `cmd/fixprguard/test_helpers.go` を加える（editorial correction）:** `validBaseline`（`:358-369`）は `"v1"` のトークンを `ParseBaseline` に通すので、版を上げると変えなければならない。設計書は `main_test.go:25` だけを挙げている。決定は変わらないので、`Status` を `approved` のまま、`Comments` に記録する（`docs/dev/developer_guide/requirements_process.md`「Editing an approved document」）。本 PR で直す。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-09 は、すべて設計書の付録 A に記録されている。
-   `implementation_handoff.md` の I-01: ステップ 2-4 で、実際の PR に対して `fixprguard` を `GIT_TRACE` 付きで実行し、`ThreadPaths` が起動する `gh` の中の git と、それが読む設定を記録する。ほかのブランチの `branch.<name>.*` を読む git があれば、ガイド 8 章の `response-rejected` の行に反映する（ステップ 3-1）。
-   `implementation_handoff.md` の I-02: ステップ 2-4 で、通常の PR について確かめる（1.4）。`branch.<そのブランチ>.*` のほかに書き込む場合は、そこで止まり、扱い（受け入れるか、要件に戻すか）を利用者に尋ねる。fork の PR は確かめない（1.4）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 設定の値の一覧の分解と分類（設計書 8 章の 1）

#### ステップ 1-1: `configListDigest`

**変更するファイル**: `internal/fixprguard/configlist.go`（新規）、`configlist_test.go`（新規）、`errors.go`

-   [ ] `errConfigList`・`mainBranch`・`configListDigest` を、設計書 3.3 の定義とコメントのとおりに加える。受理する形、分類、一覧のダイジェストは設計書 3.2 のとおりにする。外部のコマンドを起動せず、ファイルも読まない。
-   [ ] `configlist_test.go` に、設計書 7.1 の「受理する形」「値の区別」「分類」「層の区別」の項目を確かめるテストを加える。入力は git を起動せずにバイト列で与える。拒否は `errors.Is(err, errConfigList)` と `errors.Is(err, ErrGitFailed)` で確かめ、拒否のときにダイジェストを返さないことも確かめる。エラーの文字列が入力の名前・値・読み込み元を含まないことを確かめる（AC-16）。
-   [ ] 分類のテストの入力には、要件書 AC-01〜AC-09 の例（`branch.feature.a.remote`、`[branch "Feature"]`、`branch..remote`、`branch.main.*`、`branch.autoSetupMerge`、`file:/etc/gitconfig` を読み込み元とする項目、`remote.pushDefault`、複数の値の順序）を含める。外す対象の項目の追加・変更・削除の行では、分類なしのダイジェストが変わることを先に確かめる（1.2）。

**壊して確かめる**: 次で、それぞれ括弧の中の行が失敗すること。

-   分類を外す（すべての項目を比べる）（AC-01〜AC-04 の行）。
-   基準値のブランチの照合を外す（AC-05 の行）。`main` の照合を外す（AC-06 の行）。
-   サブセクションの取り出しを「最初の `.` と最後の `.` の間」から「2 つめの `.` まで」に変える（`branch.feature.a.remote` の行）。
-   項目の区切りをダイジェストに含めない（値を持たない項目と空の値の項目の行）。
-   ブランチのダイジェストの形の確かめを外す（空の文字列の行）。
-   形の確かめの 1 つ（最後の NUL、空の名前、`.` を含まない名前）を 1 つずつ外す（それぞれの拒否の行）。

**完了条件**: `go test -tags test ./internal/fixprguard` と `make lint` が通る。

### フェーズ 2: 基準値と比較の順序（設計書 8 章の 2）

#### ステップ 2-1: テストの git の記録と差し替え

**変更するファイル**: `internal/fixprguard/test_helpers_git.go`

-   [ ] `gitRouter` の記録で、`git config --list` を `git config --get` と区別できるようにする（1.3.5）。ほかのサブコマンドの記録は変えない（既存のテストが `"git add"`・`"git commit"`・`"git push"` で比べている。`worktree_flow_test.go:291-295`・`:437`）。
-   [ ] `git config --list` の呼び出しだけを、テストが指定したエラーで失敗させ、ほかの git は実物に渡す差し替えを加える（設計書 2.2、7.1 の「エラーの振り分け」、7.2 の AC-12）。差し替えは呼び出しを記録してからエラーを返し、git を起動しない。

**完了条件**: `go test -tags test ./internal/fixprguard` が通る（この時点では既存のテストの期待値を変えない）。

#### ステップ 2-2: 基準値の版、場所と種類のダイジェスト、比較の順序

版を上げると、既存のテスト、`cmd/fixprguard` のテストのヘルパー、コマンドの文書の `--protocol` が同時に食い違うので、このステップは 1 つのコミットにする（設計書 8 章の 2）。

**変更するファイル**: `internal/fixprguard/gitstate.go`、`types.go`、`gitstate_test.go`、`worktree_flow_test.go`、`cmd/fixprguard/main_test.go`、`cmd/fixprguard/test_helpers.go`、`.claude/commands/fixpr.md`、`.claude/commands/runpr.md`

-   [ ] `baselineVersion` を `v2` にする（設計書 3.1）。
-   [ ] `configDigest` を `gitDirsDigest` に改め、1.3.3 のとおりファイルの種類だけを含める（設計書 3.1・3.3）。
-   [ ] `Worktree.configListDigest` を `Worktree.configList` に改める（設計書 3.3）。
-   [ ] `CheckState` を、設計書 3.4 の手順 1〜4 と 4 章の表のとおりにする。
-   [ ] `Baseline` を、設計書 3.4 の「`Baseline`（`begin`）」のとおりにする。`begin` での一覧の読み取りの失敗は、今の `gitFailure` の規則の名前で `git-failed` にする（設計書 4 章）。
-   [ ] `gitHardening`・`Baseline` の型・`CheckState` のコメントを、設計書 3.5「コードのコメント」のとおり改める。
-   [ ] `Protocol` を `2` にし、`fixpr.md`・`runpr.md` の `--protocol 1` を、コードブロックの外の文（`fixpr.md:44`・`runpr.md:18`）も含めてすべて `--protocol 2` にする（設計書 3.5）。
-   [ ] `cmd/fixprguard/main_test.go:25` と `cmd/fixprguard/test_helpers.go` の `validBaseline` の版を `v2` にする（1.3.1）。
-   [ ] 設計書 7.3 の表の 6 つの既存のテストを、表の「改訂」のとおりに改める。`TestBaselineMalformedRejected` の `version` の行は、前の版の `v1` を拒否する行にする（AC-15）。
-   [ ] `rg -n 'w\.configListDigest|\) configListDigest' internal/fixprguard` が何も返さないことを確かめる（1.3.2）。

**壊して確かめる**: 次で、それぞれ括弧の中のテストが失敗すること。ステップ 2-3 のテストを加えた後にもう一度行う。

-   一覧の比較を `git rev-parse` の後に移す（`TestGitNotLaunchedAfterConfigChange`・`TestFinishStopsWhenHooksChangeConfig`・`TestPushRechecks` の `config_changed`）。
-   版を `v1` に戻す（`TestBaselineMalformedRejected` の前の版の行）。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 2-3: 結合テスト

**変更するファイル**: `internal/fixprguard/gitstate_test.go`、`worktree_flow_test.go`、`configlist_test.go`、`cmd/fixprguard/main_test.go`

-   [ ] 設計書 7.2 の表の各行の入力で、`begin` の後に設定を変え、`compareState` の結果を確かめるテストを加える。止まる行（AC-05〜AC-09・AC-11・AC-13）は `parts` を、止まらない行（AC-01〜AC-04・AC-10）は `compareState` が成功することを確かめる。既存の `TestBaselineDetectsChange` の表に入る行はそこに加え、止まらない行は別のテストにまとめる。
-   [ ] 止まらない行のテストでは、外す対象の設定の変更が、変更の前後で `git config --list --show-origin -z` の出力そのものを変えていることを確かめる（1.2。外す対象の変更が出力に現れない入力では、分類を確かめたことにならない）。
-   [ ] 別の worktree での `git worktree add -b X`・`git push -u`・`git worktree remove`・`git branch -d X` のそれぞれの後に、`inspect`・`finish`・`push` を順に実行して、どれも止まらないことを確かめる流れのテストを `worktree_flow_test.go` に加える（AC-01）。
-   [ ] AC-12 のテストを加える。`begin` の後の構文の誤りは実物の git で、タイムアウトはステップ 2-1 の差し替えで `context.DeadlineExceeded` と `*cmdrunner.ExitError` を `errors.Join` で返して確かめる。どちらも `config` で止まり、`git rev-parse` を起動しないことを、記録で確かめる。
-   [ ] 設計書 7.1 の「エラーの振り分け」を、ステップ 2-1 の差し替えで確かめる。4 章の表の 1・2・5 行が `git-failed`（`errors.Is(err, ErrGitFailed)` で、`*GitStateError` でない）、3・4 行が `config` になること。`begin` では 3・4・6 行も `git-failed` になることを確かめる。
-   [ ] AC-14 の、リポジトリの config を FIFO に置き換えると git を 1 回も起動せずに `config` で止まる行を、`TestGitNotLaunchedAfterConfigChange` に加える（設計書 V-7）。AC-11 の `.git` ファイルの書き換えにも、git を 1 回も起動しないことの確かめを加える。
-   [ ] AC-15: `cmd/fixprguard/main_test.go` に、`v1.` に続く 7 つの要素の、それ以外は正しい形のトークンを `--baseline` に渡すと、`inspect`・`finish`・`push` が終了コード `2` になるテストを加える。
-   [ ] AC-16: `cmd/fixprguard/main_test.go` に、今のブランチと外す対象のブランチに見分けやすい名前を付け、今のブランチの設定を変えて `inspect` を止めるテストを加える。基準値のトークン、止まったときの標準出力、標準エラーが、ブランチの名前、設定の名前と値、読み込み元のパスを含まないことを確かめる。

**壊して確かめる**: 次で、それぞれ括弧の中のテストが失敗すること。

-   分類を外す（止まらない行のテストと、AC-01 の流れのテスト）。
-   場所と種類のダイジェストから種類を外す（FIFO の行）。
-   `CheckState` で一覧の読み取りの失敗を `git-failed` にする（AC-12 のテスト）。
-   `CheckState` の判定の順序で、`ExitError` を `context` のエラーより先に判定する（振り分けのテストの取り消しの行）。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 2-4: ネットワークを要する確かめと git の最低の版

**変更するファイル**: `docs/tasks/0035_fixprguard_branch_config/03_implementation_plan.md`（結果の記録）

-   [ ] 利用者の承認を得てから行う（`CLAUDE.md`「Tool Execution Safety」）。
-   [ ] I-01: 開いている PR に対して、`GIT_TRACE` に出力先のファイルを指定して `fixprguard list` を実行する。`list` も `ThreadPaths` と同じ `gh api graphql` と権限の照会を使う（`threads.go:180`・`trust.go:140`）。起動された git と、それがブランチの設定を読むかを記録する。
-   [ ] I-02: このリポジトリの PR を、一時的な worktree で `gh pr checkout` し、前後の `git config --list --show-origin` を比べて、書き込まれた設定の名前を記録する。`branch.<そのブランチ>.*` のほかに書き込む場合は、止まって利用者に尋ねる（1.5）。終わったら一時的な worktree とブランチを消す。
-   [ ] git の最低の版（2.12）の根拠（設計書 1.3「git の最低の版」の 3 つの機能が入った版）を、git のリリースノートで確かめ直す。違えば、設計書を直してからステップ 3-1 に進む。
-   [ ] 結果を本書の 1.5 に追記する（コマンド、`gh`・git の版、日付）。

**完了条件**: 3 つの確かめの結果が本書に記録されている。

### フェーズ 3: ガイド（設計書 8 章の 3）

#### ステップ 3-1: ガイドの改訂

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`

-   [ ] 日本語版を、設計書 3.5 の「ガイド（英語・日本語）」の表のすべての箇所について改める。8 章の `response-rejected` の行と 7.2 には、ステップ 2-4 の I-01・I-02 の結果（確かめた範囲と、fork の PR を確かめていないこと）を反映する。
-   [ ] 書いた記述を、ツールの振る舞いと突き合わせる。比べる範囲は `configListDigest`、止まり方は設計書 4 章の表とステップ 2-3 のテスト、最低の版はステップ 2-4 の結果と比べる。
-   [ ] 日本語版をコミットしてから、英語版を `.claude/commands/mktrans.md` の原則で翻訳する（`CLAUDE.md`「Translation Guidelines」）。

**完了条件**: 両方の版の、設計書 3.5 の表のすべての箇所が改まっている（ステップ 3-2 のガードで確かめる）。

#### ステップ 3-2: ガイドとコマンドの文書のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] `--protocol` の番号のガードを加える（1.4）。両方のガイドと `fixpr.md`・`runpr.md` のすべての `--protocol <数字>`、ガイドの `"protocol":<数字>`、ガイド 4 章の現在の番号が、`fixprguard.Protocol` と一致することを確かめる。各文書で 1 つ以上見つかることも確かめる。
-   [ ] AC-18 のガードを加える。両方のガイドの、要件書 AC-18 が挙げる箇所（1 章の層 (3)、5 章の `inspect`・`push`、7.1 の行、7.2、8 章の `git-state-changed` の行）のそれぞれが、(a)〜(c) を述べる語を含むことを確かめる。7.1 の行は、外す対象を除いた config を比べること、比べる前に起動する git が `git config --list` だけであることを述べる語を含むことも確かめる。箇所は、節の見出しや表の行の最初のセルで取り出す。日本語版を確かめるリテラルは日本語で書く（1.2）。

**壊して確かめる**: 次で、それぞれのガードが失敗すること。一時的に書き換えて確かめ、戻す。

-   ガイドの例の 1 か所、`fixpr.md:44` の文、ガイド 4 章の番号を、それぞれ `1` に戻す。
-   両方のガイドのそれぞれで、AC-18 の箇所の (a)・(b)・(c) の文を 1 つずつ消す。

**完了条件**: `go test -tags test ./cmd/fixprguard` が通る。

#### ステップ 3-3: PR-1 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。
-   [ ] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行する。

**完了条件**: 上の 2 つが通る。

### PR-1 作成ポイント: fixprguard ignores other branches' config

**対象ステップ**: 1-1 / 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3

**推奨タイトル**: `feat(0035): fixprguard の config の比較から、ほかのブランチの branch 設定を外す`

**レビュー観点**: 一覧の分類が今のブランチ・`main`・`branch` でない設定を比べ続け、受理する形に合わない出力を拒否すること / `CheckState` の起動の順序（種類の比較 → `git config --list` → `git rev-parse`）と、一覧の読み取りの失敗の振り分け（設計書 4 章） / 止まらない行のテストが、分類を外すと失敗する入力を使っていること / `Protocol` 2 とコマンドの文書・ガイドの番号、`/runpr` の前の再インストールの要否

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2 は、git を起動する前の比較の順序という、ガードツールの保証（ガイド 7.1）を変えるセキュリティのゲートの変更である（`mkplan.md` step 8 の panel mode の契機）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**`/runpr` の前**: 利用者は、PR-1 のブランチの作業ツリーで `make install-fixprguard` を実行してから `/runpr` を実行する（1.4）。マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は終了コード `2` で止まる。

**マージの後**: 利用者は、マージの後の `origin/main` を含む作業ツリーで `make install-fixprguard` を実行し直す。作業中のほかのブランチには、次の `/fixpr`・`/runpr` の前に `main` を取り込む（設計書 3.5「移行の手順」）。

### フェーズ 4: 0030 の文書（設計書 8 章の 4）

#### ステップ 4-1: 0030 の文書の改訂

**変更するファイル**: `docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md`、`02_architecture.md`

-   [ ] 設計書 3.5「0030 の文書」の表のすべての箇所を、要件書 AC-17 の (a)〜(c) を述べる形に改める。0030 `02_architecture.md` 2 章ではなく 1.2 の push の決定を対象とすることは、設計書 3.5 の表の後の段落のとおりである。
-   [ ] 両文書の `Status` を `approved` のままにし、`Comments` に `YYYY-MM-DD (decision): タスク 0035（#431）の要件で決めた変更` の形で記録する（設計書 3.5、要件書 5 章）。日付は改訂した日にする。
-   [ ] 改めた記述を、ガイド（フェーズ 3）と設計書 3.4・5 章と突き合わせ、比べる範囲と残余のリスクの述べ方が一致することを確かめる。

**完了条件**: 設計書 3.5 の表のすべての箇所が改まっている（ステップ 4-2 のガードで確かめる）。

#### ステップ 4-2: 0030 の文書のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] 0030 の両文書の、設計書 3.5 の表の箇所のそれぞれが (a)〜(c) を述べる語を含むこと、両文書の `Comments` が `(decision)` とタスク 0035 を含むこと、`Status` が `approved` であることを確かめるガードを加える。箇所の取り出しと語の選び方は、ステップ 3-2 の AC-18 のガードと同じにし、取り出しの関数は共有する。

**壊して確かめる**: 両文書のそれぞれで、箇所の (a)・(b)・(c) の文を 1 つずつ消す、`Comments` の記録を消す、のそれぞれでガードが失敗すること。

**完了条件**: 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。

#### ステップ 4-3: PR-2 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。

**完了条件**: 上が通る。

### PR-2 作成ポイント: task 0030 documents describe the config scope

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `docs(0035): 0030 の要件書と設計書の config の比較の範囲を改める`

**レビュー観点**: 設計書 3.5 の表のすべての箇所が (a)〜(c) を述べていること / ガイド（PR-1）と 0030 の文書の述べ方が一致すること / `Status` を `approved` のまま `Comments` に `(decision)` を記録したこと

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
| M1 | `configListDigest` とそのユニットテスト | ステップ 1-1 の完了条件 |
| M2 | 新しい比較、`Protocol` 2、結合テスト | ステップ 2-3 の完了条件 |
| M3 | I-01・I-02・最低の版の確かめの結果 | ステップ 2-4 の完了条件 |
| M4 | ガイドとガード | PR-1 のマージ |
| M5 | 0030 の文書とガード | PR-2 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3 | `configlist.go`、`gitstate.go` の比較の順序と振り分け、基準値 `v2`、`Protocol` 2、コマンドの文書の `--protocol`、結合テスト、ガイド、番号と AC-18 のガード | frontier-required |
| PR-2 | 4-1 / 4-2 / 4-3 | 0030 の要件書と設計書の改訂、AC-17 のガード | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。

-   フェーズ 1 は外部のコマンドに依存しないので最初に置く。フェーズ 2 はその関数を使う。
-   フェーズ 2 の中では、テストの記録の変更（2-1）を、期待値を変える 2-2 より先に置く。2-2 の既存のテストの改訂が、`git config --list` を区別した記録を使うためである。
-   I-01・I-02 の確かめ（2-4）は、その結果を書くガイド（3-1）より先に置く。
-   0030 の文書（フェーズ 4）は、ガイドの改訂の後に置く（設計書 8 章の 4。要件書は承認済み）。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の項目を、ステップ 1-1（受理する形・値の区別・分類・層の区別）とステップ 2-3（エラーの振り分け）に割り当てた。システムの config を読み込み元とする外す対象の項目（AC-03）は、実際のリポジトリのテストでは差し替えられないので、ユニットテストだけで確かめる（設計書 7.1）。

### 4.2. 結合テスト

設計書 7.2 の表の各行を、ステップ 2-3 で一時ディレクトリの実際のリポジトリに対して確かめる（ガイド 9 章）。`gh` と make は fake を使う。`gh pr checkout` は再現せず、`git config` による直接の書き込みで代える（設計書 7.2 の AC-01）。V-4 の振る舞い（値のない項目で `git ls-remote` が失敗すること）は git の振る舞いなので固定しない（設計書 7.2）。

### 4.3. セキュリティテスト

設計書 7.6 の対応のとおりである。経路 1 は AC-08 と AC-14、経路 4 は AC-12、経路 5 は AC-14 の FIFO の行、`.git` ファイルの書き換えは AC-11 で確かめる。7.4 の壊す確認は、ステップ 1-1・2-2・2-3 の「壊して確かめる」に割り当てた。

### 4.4. 文書のガード

ステップ 3-2 と 4-2 のガードは `cmd/fixprguard/docs_test.go` に置き、CI の `doc-guard` のジョブが実行する。

### 4.5. 後方互換性

前の版の基準値は受け付けない（AC-15）。前の版のバイナリと新しい文書、新しいバイナリと前の文書の組み合わせは、`--protocol` の不一致で終了コード `2` で止まる（AC-19）。既存の `TestProtocolMismatch`（`cmd/fixprguard/main_test.go:42-62`）が、`Protocol` と違う番号を拒否することを確かめる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は振る舞いを確かめる Go のテスト、`static` は文書を読む Go のガードのテスト、`manual` は PR の上での観察を表す。新しいテストの名前は仮の名前で、実装で変えたときは本書の表を同じ PR で改める（1.3.5）。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | test | `internal/fixprguard/configlist_test.go::TestConfigListDigestClassification`、`internal/fixprguard/gitstate_test.go::TestOtherBranchConfigIgnored`（`git worktree add -b`・`git push -u`・`git worktree remove`・`git branch -d`・`git config`）、`internal/fixprguard/worktree_flow_test.go::TestRoundIgnoresOtherBranchConfig` |
| AC-02 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestOtherBranchConfigIgnored`（節の追加と削除、`[branch ""]`） |
| AC-03 | test | `configlist_test.go::TestConfigListDigestClassification`（システムの config を含む読み込み元）、`gitstate_test.go::TestOtherBranchConfigIgnored`（グローバル、worktree の config、include 先） |
| AC-04 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestOtherBranchConfigIgnored`（`branch.<b>.a.remote`、大文字・小文字だけが違う名前） |
| AC-05 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestBaselineDetectsChange`（今のブランチ、`.` を含む名前、`[branch.Feature]`） |
| AC-06 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestBaselineDetectsChange`（`branch.main.*`） |
| AC-07 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestBaselineDetectsChange`（`branch.autoSetupMerge`） |
| AC-08 | test | `configlist_test.go::TestConfigListDigestClassification`（読み込み元の変化）、`gitstate_test.go::TestBaselineDetectsChange`（既存の `config`・`global_config`・`included_config`・`linked_worktree_common_config` と、加える `remote.pushDefault`・`remote.<name>.*`・`include.path`・読み込み元の移動の行） |
| AC-09 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestBaselineDetectsChange`（複数の値の順序） |
| AC-10 | test | `gitstate_test.go::TestOtherBranchConfigIgnored`（コメント・空行・字下げだけの追加） |
| AC-11 | test | `gitstate_test.go::TestBaselineDetectsChange`（既存の `git_file_points_elsewhere`・`commondir_added`）、`::TestGitNotLaunchedAfterConfigChange`（`.git` ファイルの行。git を 1 回も起動しない） |
| AC-12 | test | `gitstate_test.go::TestConfigListUnreadable`（構文の誤り、タイムアウト。`git rev-parse` を起動しない）、`::TestConfigListFailureKinds`（設計書 4 章の表の振り分け） |
| AC-13 | test | `gitstate_test.go::TestBaselineDetectsChange`（既存の `branch_switch_same_commit` と、加える detached HEAD の行） |
| AC-14 | test | `gitstate_test.go::TestGitNotLaunchedAfterConfigChange`（改訂した 2 行と FIFO の行）、`worktree_flow_test.go::TestFinishStopsWhenHooksChangeConfig`・`::TestPushRechecks`（`config_changed`）、`cmd/fixprguard/main_test.go::TestInspectChecksGitStateBeforeGh`（既存） |
| AC-15 | test | `gitstate_test.go::TestBaselineMalformedRejected`（前の版の行）、`cmd/fixprguard/main_test.go::TestPreviousBaselineRejected`（終了コード `2`） |
| AC-16 | test | `cmd/fixprguard/main_test.go::TestConfigStopOmitsNames`、`configlist_test.go`（エラーの文字列） |
| AC-17 | static | `cmd/fixprguard/docs_test.go::TestTask0030DocsDescribeConfigScope` |
| AC-18 | static | `cmd/fixprguard/docs_test.go::TestGuideDescribesConfigScope` |
| AC-19 | static + test | `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（既存）・`::TestDocumentedProtocolMatches`（ガイドとコマンドの文書のすべての番号）、`cmd/fixprguard/main_test.go::TestProtocolMismatch`（既存） |

AC-17・AC-18 は、文書が何を述べるかを定めるので、`static` で確かめる。AC-19 のうち「前の版の値より大きい」ことは、PR-1 の差分で `1` から `2` への変更を見て確かめる（`manual`）。定数をそれ自身のリテラルと比べるテストは書かない（`CLAUDE.md`「Testing Strategy」）。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3）
- [ ] PR-2 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 対策 |
|---|---|
| 分類のテストが、外す対象の変更を出力に現さない入力で通ってしまう | 止まらない行では、変更の前後で出力そのものが変わることを先に確かめる（ステップ 1-1・2-3） |
| `digestPath` を分けるときに、info や hooks の内容の比較まで外れる | 既存の `info_exclude`・`info_attributes`・`hook_link_target_content` の行で確かめる（1.3.3） |
| `CheckState` の振り分けで、取り消しを `config` の変化として扱う | `ExitError` より先に `context` のエラーを判定する。振り分けのテストと壊す確認で確かめる（ステップ 2-3） |
| PR-1 の `/runpr` が、インストール済みのツールの番号の不一致で止まる | PR-1 の作成ポイントの「`/runpr` の前」のとおり再インストールする（1.4）。忘れても終了コード `2` で止まるだけで、誤った判定はしない |
| 再インストールの後、ほかの worktree のセッションが止まる | `main` を取り込むまで終了コード `2` で止まる（fail-closed）。ガイド 3 章の移行の手順に書く（設計書 3.5） |
| I-02 で、通常の PR の `gh pr checkout` が `branch.<そのブランチ>.*` のほかにも書き込む | ステップ 2-4 で止まり、扱いを利用者に尋ねる（1.5） |
| 0033 の文書の `--protocol 1` が、本タスクの後に古くなる | 設計書 9 章のとおり、0033 の実装の PR が合わせる。0033 の実装で `TestDocumentedProtocolMatches` が食い違いを見つける |

### 7.2. スケジュールのリスク

-   ステップ 2-4 はネットワークと利用者の承認を要する。承認を待つ間も、ステップ 3-1 のうち I-01・I-02 の結果によらない箇所は進められる。
-   要件書 2.2 のとおり、本タスクは 0034（PR #429）より先にマージする。0034 の作業は、PR-1 のマージまで止まる。
-   CodeRabbit の PR レビューは 1 時間あたりの回数が限られるので、PR を開いてからレビューが届くまで待つことがある（`runpr.md` step 3）。

## 8. 成功基準 (Success Criteria)

-   [ ] 5 章のすべての `test` と `static` の検証が通る。
-   [ ] `make test && make lint` と、PR-1・PR-2 の CI の `doc-guard` のジョブが通る。
-   [ ] 各ステップの「壊して確かめる」の結果が、コミットメッセージに書かれている。
-   [ ] ステップ 2-4 の確かめの結果が本書に記録され、ガイドに反映されている。
-   [ ] ガイドの日本語版と英語版の記述が一致する。

## 9. 次のステップ (Next Steps)

-   承認の後、`/runplan 0035` で PR-1 から実装する。PR-1 の `/runpr` の前に、PR-1 のブランチで `make install-fixprguard` を実行する。
-   PR-1 のマージの後、`main` で `make install-fixprguard` を実行し直す。0034（PR #429）を再開するときは、基準値の版を `v3` に上げて要素を足し、0034 の要件案の `gitstate.go` の行番号を合わせる（設計書 9 章）。
-   PR-2 は本タスクの最後のプランの PR なので、マージの後に issue #431 を閉じる。
