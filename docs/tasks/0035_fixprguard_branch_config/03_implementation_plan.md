# 実装計画書：fixprguard の config の比較から、ほかのブランチの `branch.<name>.*` を外す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-22 |
| Review date | 2026-10-22 |
| Reviewer | isseis |
| Comments | 2026-10-22 (editorial): ステップ 3-4 に、0033 の PR-2（#436）がステップ 2-0 を完了したので 0033 の計画の変更を取り下げたことを記録した。2026-10-22 (editorial): ステップ 3-4 の 2 つめの項目の前提を直した（0035 のツールは 0033 の PR-1 を含み、`--reviewer` の繰り返しを受け付ける）。1.3.4 の `[branch ""]` の観察を、失敗するコマンドに限って直した。2026-10-22 (editorial): ステップ 2-3 のテストの配置と名前を実装に合わせた（AC-01 の別の worktree の操作は流れのテストだけに置く。AC-11 の git を起動しないことは `TestBaselineDetectsChange` の層の確かめで、FIFO の行は `TestConfigFileTypeCheckedBeforeGit` で、タイムアウトは `TestConfigListFailureKinds` で確かめる）。1.3.4 に `[branch ""]` の観察を記録した。決定は変えていない。 |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-004 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。作業は設計書 8 章の 4 つの項目を 1 つずつフェーズにし、2 つの PR にまとめる（1.4）。

-   **フェーズ 1（設計書 8 章の 1）:** 設定の値の一覧の分解・分類・ダイジェストを、外部のコマンドを起動しない関数として `configlist.go` に置く。
-   **フェーズ 2（設計書 8 章の 2）:** 基準値の版、場所と種類のダイジェスト、`CheckState` と `Baseline` の順序、一覧の読み取りの失敗の振り分けを変え、`Protocol` を上げる。結合テストと既存のテストの改訂を行う。
-   **フェーズ 3（設計書 8 章の 3）:** ネットワークを要する確かめ（I-01・I-02）と git の最低の版の確かめを行い、その結果を入れてガイド（英語・日本語）を改め、ガイドとコマンドの文書を確かめるガードを加える。
-   **フェーズ 4（設計書 8 章の 4）:** 0030 の `01_requirements.md`・`02_architecture.md` を改め、その記述を確かめるガードを加える。

用語は、要件書 3.1 と設計書の冒頭の定義に従う（設定の値の一覧、外す対象のブランチの設定、項目、比べる項目、基準値のブランチ、場所と種類のダイジェスト、一覧のダイジェスト）。本書の「フェーズ」は実装の区切りを指し、`/fixpr` の「修正のフェーズ」とは別である。「ガイド」は `docs/dev/developer_guide/fixprguard_guide.md` と日本語版の `fixprguard_guide.ja.md` を指す。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの文書（`.claude/commands/`）の文も英語で書く（`CLAUDE.md`「Language」）。例外は、日本語の文書の文字列と一致させる文書のガードのリテラルだけである（先例: `cmd/yt2column/docs_test.go:738`・`:747`）。
-   テストは、対象の実装を壊して失敗することを確かめてからコミットし、壊し方と結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」、ガイド 9 章）。各ステップの「壊して確かめる」は、壊す対象と、失敗するはずのテストを挙げる。
-   **層を区別する入力を使う**（`CLAUDE.md`「A layered path needs inputs only one layer can handle」、設計書 7.1・H-07）。
    -   分類の層: 外す対象の項目を変えた入力では、まず分類なしのダイジェスト（出力全体の SHA-256）または `git config --list --show-origin -z` の出力そのものが変わることを確かめ、そのうえで分類ありの結果が変わらないことを確かめる。
    -   ファイルの内容の層（AC-10）: config のファイルのバイト列が変わり、一覧の出力は変わらないことを確かめ、そのうえで止まらないことを確かめる。
    -   場所と種類の層と一覧の層: 起動した git の記録で区別する。場所と種類で止まる行は git を 1 回も起動せず、一覧で止まる行は `git config --list` を起動して `git rev-parse` を起動しない。
-   Go のファイルを変えたら `make fmt` を実行する。各フェーズの最後に `make test && make lint` を実行する。
-   ソースのコメントに `AC-NN`・`F-NNN` を書かない（要件書との対応は本書 5 章に書く）。

### 1.3. 既存コード調査結果

調査はコミット `82598be` で行った。設計書が確かめたコミット `b8038b4` から、`internal/`・`cmd/`・`.claude/`・`docs/dev/`・`docs/tasks/0030_fixpr_untrusted_threads/`・`CLAUDE.md` に変更はない（`git diff --stat b8038b4 82598be -- docs/tasks/0030_fixpr_untrusted_threads internal cmd .claude docs/dev CLAUDE.md` の出力が空）。そのため、設計書の `file:line` は `82598be` でもそのまま使える。以下の `file:line` も `82598be` のものである。

#### 1.3.1. 変える箇所の一覧

洗い出しは 2 つの検索で行った。Go のソースは、`rg -n -g '*.go'` で `PartConfig`・`configDigest`・`configListDigest`・`digestPath`・`fileDigests`・`baselineVersion`・`"v1`・`gitCalls`・`Protocol` を探した。文書とスクリプトは、リポジトリ全体（`docs/tasks/0035_fixprguard_branch_config/` を除く）で `--protocol 1`・`"protocol":1`・`'v1.` を探した。`--protocol 1` は、下の表の箇所のほかに、`docs/tasks/0033_multiple_trusted_reviewers/`（0033 の設計書と計画）と `docs/tasks/0030_fixpr_untrusted_threads/`（0030 の当時の記録）に現れる。

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
| `docs/tasks/0032_japanese_pr_title/check_rules.sh:524` | 0032 の静的な確認が、`fixpr.md` に `fixprguard edit-pr --protocol 1 --title-file <title-file>` があることを確かめる。CI は実行しないが、手で実行する手順がある（同 `:6-7`） | `--protocol 2` にする。0030 も同じ行を `fixpr.md` に合わせて直した（同 `:521-522` のコメント） | 2-2 |
| `cmd/fixprguard/main_test.go:25` | `baseline = "v1.x"`（解析だけに使う） | `"v2.x"` にする（設計書 7.3） | 2-2 |
| `cmd/fixprguard/test_helpers.go:358-369` | `validBaseline` が `"v1"` で始まるトークンを作り、`ParseBaseline` が受け付けなければテストの準備で失敗する | `"v2"` にし、`baselineVersion` と合わせる必要があることをコメントに書く。なお、このファイルは設計書 2.2 の表に載っていなかったので、この計画の PR で editorial correction として表に加えた（1.4） | 2-2 |
| `internal/fixprguard/test_helpers_git.go:36-57`・`:85-110` | `gitRouter.Calls` は git のサブコマンドの名前だけを記録する（`git config --list` と `git config --get` が同じ `git config`）。すべての git を実物に渡す。`testRepo` のブランチは `feature` だけで、リモートには `feature` だけを push する | `git config --list` を区別して記録する（2-1）。`git config --list` の結果を差し替える仕組みと、別の worktree やブランチを作る補助を加える（2-3。設計書 2.2） | 2-1・2-3 |
| `internal/fixprguard/gitstate_test.go`・`worktree_flow_test.go` | 設計書 7.3 の表の 6 行（`gitstate_test.go:132-134`・`:238`・`:266-276`・`:277-293`、`worktree_flow_test.go:285-313`・`:422-424`・`:440-442`）。`TestBaselineDetectsChange` は git の記録を捨てる（`gitstate_test.go:168`・`:173` の `w, _ :=`） | 設計書 7.3 のとおり改める。`worktree_config` の行は 1.3.3 の注意に従う。`TestBaselineDetectsChange` は記録を受け取り、行ごとに止まった層を確かめる | 2-2・2-3 |
| `cmd/fixprguard/docs_test.go:84-116` | `TestDocumentedInvocationsParse` は `fixpr.md`・`runpr.md` のコードブロックの中の呼び出しだけを読む（`:93-96`）。ガイドとコードブロックの外の `--protocol` は確かめない | `--protocol` の番号を確かめるガードを加える（1.4） | 3-3 |
| ガイドの日本語版と英語版 | 設計書 3.5 の表の箇所。各版で、`--protocol 1` が 9 か所（`:82`・`:92`・`:113`・`:136`・`:146`・`:158`・`:170`・`:182`・`:192`）、`"protocol":1` が 4 か所（`:85`・`:173`・`:185`・`:195`）、現在の番号が 1 か所（`:57`）、基準値の例の `'v1....'` が 3 か所（`:146`・`:158`・`:170`） | 設計書 3.5 のとおり改める | 3-2 |
| `docs/tasks/0033_multiple_trusted_reviewers/03_implementation_plan.md:101-113`・`:292` | 承認済みで未実装の 0033 の計画の 1.3.4 とステップ 2-0 が、インストール済みのツールに `list --protocol 1 …` を渡して振る舞いを確かめる。「`protocol` の番号は変わらない」を前提にする | ステップ 3-4 で扱う（1.4） | 3-4 |
| `docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md`・`02_architecture.md` | 設計書 3.5「0030 の文書」の表の箇所 | 設計書 3.5 のとおり改める | 4-1 |

変えない箇所は次のとおりである。

-   `cmd/fixprguard` のコード（設計書 2.2）。`CheckState` の呼び出し（`cmd/fixprguard/main.go:557`、`worktree.go:130`・`:243`・`:273`・`:326`・`:367`・`:383`・`:502`）は、そのまま新しい比較を使う。
-   `infoDigest`・`hooksDigest` は、今のとおりファイルの内容を含める（設計書 2.3 の表で変わるのは config だけ）。
-   `docs/dev/developer_guide/package_reference.md`。`internal/fixprguard`・`cmd/fixprguard` の行は config の比較の範囲を述べていない（`:14`・`:21`）。
-   0033 の設計書（`02_architecture.md:65`）と、0030 の文書の当時の呼び出しの記録。0033 の設計書の番号の記述は、設計書 9 章のとおり 0033 の実装の PR が合わせる。

#### 1.3.2. 名前を変えるものの一覧

| 今の名前 | 新しい名前 | 場所 | ステップ |
|---|---|---|---|
| 関数 `configDigest` | `gitDirsDigest` | `internal/fixprguard/gitstate.go` | 2-2 |
| メソッド `Worktree.configListDigest` | `Worktree.configList` | `internal/fixprguard/gitstate.go` | 2-2 |
| （なし） | 関数 `configListDigest`（新規） | `internal/fixprguard/configlist.go` | 1-1 |

新しい関数 `configListDigest` は、今のメソッドと同じ名前である（設計書 3.3）。Go では、パッケージの関数と型のメソッドが同じ名前を持てるので、フェーズ 1 の間は両方が並ぶ。ステップ 2-2 でメソッドを `configList` に改めた後は、`rg -n 'w\.configListDigest|\) configListDigest' internal/fixprguard` が何も返さないことで、古いメソッドが残っていないことを確かめる。古い関数 `configDigest` は、ビルドで確かめる。本書の各ステップ、5 章の表、6 章のチェックリストは、この表の新しい名前で書く。

#### 1.3.3. 場所と種類のダイジェストの作り方

今の `digestPath`（`gitstate.go:375-409`）は、`configDigest`・`infoDigest`・`digestEntry`（hooks）が共有し、通常のファイルの内容の SHA-256 をダイジェストに含める。`gitDirsDigest` だけが内容を含めないので、`digestPath` を、種類を含める部分と内容を含める部分に分け、`gitDirsDigest` は種類の部分だけを使う。`infoDigest` と hooks は今の書き方を保つ。info と hooks の内容の比較が保たれることは、既存の `TestBaselineDetectsChange` の `info_exclude`・`info_attributes`・`hook_link_target_content` の行が変更なしで通ることで確かめる。

種類は、ないことと通常のファイルであることを区別する。そのため、`begin` の時点でなかった `config.worktree` を後から作ると、内容によらず場所と種類の比較で止まる。worktree の config の内容を一覧の層で確かめるテスト（設計書 7.3 の `worktree_config` の改訂と、AC-03 の worktree の config の行）は、`begin` の前に `config.worktree` を作っておく。

`Baseline` の構築の経路: 本タスクは新しい型を加えない。`Baseline` の構築は、今のとおり `newBaseline` だけで行い（`gitstate.go:115-119`）、既存の `TestRestrictedTypesBuiltByConstructors`（`construction_test.go`）が守る。`configListDigest` の基準値のブランチのダイジェストは、構造体でなく引数で渡し、関数の中で形を確かめる（設計書 3.3）。

#### 1.3.4. 外部の前提の確かめ

-   **手元の版:** git 2.50.1（Apple Git-155）、`gh` 2.96.0（`git --version`・`gh --version`、2026-10-22）。設計書 1.3 の確認と同じ版である。
-   **`context` の取り消しとタイムアウトのエラー:** `cmdrunner` の `Run` は、終了コードのある失敗のとき、`ctx.Err()` があれば `errors.Join(ctxErr, *ExitError)` を返す（`internal/cmdrunner/runner.go:96-108`）。出力の上限を超えたときは、終了コードを調べずに `ErrOutputTooLarge` を包んだエラーを返す（`:92-94`）。設計書 4 章の判定の順序はこれに基づく。テストは `&cmdrunner.ExitError{Code: …}` を作れる（先例: `internal/fixprguard/trust_test.go:198`）。
-   **親の `context`:** `cmd/fixprguard` は `context.Background()` から始め、期限を付けない（`cmd/fixprguard/main.go:146`）。`context.DeadlineExceeded` は、`gitRaw` が付ける `gitTimeout`（`gitstate.go:444-448`）からしか生じない。
-   **環境変数:** `fixprguard` は、git・`gh`・make に渡す環境から `GIT_*` をすべて除く（`cmd/fixprguard/main.go:152-157` の `osEnv` と `GitEnv`、`gitstate.go:87-96`）。そのため、`fixprguard` の実行に `GIT_TRACE` を付けても、`gh` の中の git には届かない。I-01 の確かめは、`gh` を直接実行して行う（ステップ 3-1）。
-   **lint と、テストからだけ使われる関数:** `make lint` は `golangci-lint run --build-tags test,integration` を実行する（`Makefile:10`・`:208-210`）。`.golangci.yml` は `run.tests` を指定しないので、既定の `true` でテストのファイルも読む。フェーズ 1 の `configListDigest` はテストからだけ使われるが、`unused` に報告されない。ただし `test_helpers_git.go` はテストのファイル（`_test.go`）ではないので、そこに加える補助は、加えた時点でテストから使われていなければ `unused` に報告される。また、`_test.go` だけに効く linter の除外は、このファイルには当たらない。
-   **FIFO の作り方:** テストでは `syscall.Mkfifo` を使う。先例は `internal/fixprguard/changes_test.go:113`・`:178` である。
-   **テストのリポジトリのリモート:** `newTestRepo` は `feature` を `origin` に `push -u` する（`test_helpers_git.go:108`）ので、`origin/feature` はあるが `origin/main` はない。別の worktree で `git worktree add -b X` を試すときは、`origin/feature` から作る（ローカルのブランチから作ると `branch.X.*` が書かれない）。
-   **`[branch ""]` の節（実装の際に確かめた、2026-10-22、git 2.50.1）:** `git config --list` は `branch..remote` として出力し、終了コード 0 で終わる。一方、リモートの設定を読むコマンド（`git remote -v`・`git ls-remote`・`git push`、長い形式の `git status`）は `fatal: bad config variable 'branch..remote'` で、終了コード 128 で終わる。`git status --porcelain=v1`・`git rev-parse`・`git symbolic-ref`・`git add`・`git commit`・`git config --get` は成功する。要件書 1 章の「git がこれを使うことはない」とは違い、git はリモートの設定を読むときにこの節で失敗する。比較から外す決定（要件書 AC-02）はそのまま実装した。外した後も、ツールが次に起動する git が失敗して `git-failed` で止まるので、誤りうる方向は止まりすぎだけであり、設計書 1.2 の値のない設定と同じ扱いになる。ステップ 2-3 のテストは、この行だけ `CheckState` を通ることを確かめる。ガイドの `git-failed` の行に、この場合も書く（ステップ 3-2）。
-   **ネットワークを要する確かめ（I-01・I-02）と git の最低の版:** ステップ 3-1 で、実装のときに利用者の承認を得て行う（`CLAUDE.md`「Tool Execution Safety」）。

#### 1.3.5. 実装時に決めること

-   `configListDigest` の中の関数の分け方と、テストの表の行。
-   `gitRouter` の記録の形（`git config --list` を何と書くか）と、差し替えの API の形。
-   `CheckState` の振り分けを書く場所（`configList` の結果を受け取る非公開の関数にするか、`CheckState` の中に書くか。設計書 3.3 は「`CheckState` の中の 1 か所」とだけ定める）。
-   ガードのテストが固定する語と、箇所を取り出す鍵。語は書いた文から選ぶ。鍵は、本タスクで書き換えない見出しやセルを選ぶ。
-   新しいテストの関数の名前。5 章の表に書いた名前と変えたときは、同じ PR で本書の表を改める。

### 1.4. 計画の際に決めたこと（2026-10-22）

-   **PR は 2 つにする（PR-1 = フェーズ 1〜3、PR-2 = フェーズ 4）:** 理由: ガイド 7.1 の保証の書き換えは、比較の振る舞いの変更と同じ PR でレビューしたほうが、食い違いを見つけやすい。ガイドの `--protocol` の番号はガードで `Protocol` と結ぶので（下の項目）、`Protocol` を上げる PR に入れなければ `main` でガイドとツールが食い違う。フェーズ 1 だけを先にマージする案は、どこからも使われない関数がそのままマージされることになるので採らない。0030 の文書は、ガイドの改訂を下敷きにする文書だけの変更なので、別の PR にする。
-   **I-02 は、通常の PR だけを実際に確かめる:** fork の PR は確かめない。理由: 設計の安全性はこの確認によらず、外れても止まりすぎる方向だけである（設計書 1.3 の表の後の段落）。fork の PR の確認には、公開リポジトリの大きな clone とネットワークを要する。確かめていないことは、ガイド 7.2 に書く（ステップ 3-2）。
-   **PR-1 の `/runpr` の前に、PR-1 のブランチで `make install-fixprguard` を実行する:** PR-1 のブランチの `fixpr.md`・`runpr.md` は `--protocol 2` を渡すので、インストール済みのツールでは終了コード `2` で止まる。再インストールしてからマージされるまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は終了コード `2` で止まる（fail-closed）。PR-1 の作成ポイントと 7 章に書く。
-   **`--protocol` の番号は、ガードのテストで確かめる（設計書 7.5 の 1 つめの項目）:** `cmd/fixprguard/docs_test.go` に、ガイドの現在の番号、ガイドと `.claude/commands/` のすべての文書のすべての `--protocol N`（コードブロックの外の文を含む）、ガイドの出力の例の `"protocol":N` が `Protocol` と一致することを確かめるテストを加える。理由: 今の `TestDocumentedInvocationsParse` は `fixpr.md`・`runpr.md` のコードブロックの中しか読まない。そのため、両文書の文の 2 か所と、ガイドの各版の 14 か所（両版で 28 か所）が食い違っても通る。文書に書いただけの確認は、誰も実行しないので守られない（`mkplan.md` step 6）。基準値の例の `'v2....'` は、版の文字列が公開されていないのでガードでは確かめず、ステップ 3-2 で `baselineVersion` と突き合わせる。
-   **AC-17・AC-18 も、ガードのテストで確かめる:** 対象の記述ごとに、要件書 AC-17 の (a)〜(c)（(a) 外す対象のブランチの設定は比べないこと、(b) 設定の値の一覧を変えない編集は検出しないこと、(c) 外す対象のブランチの設定の書き換えは残余のリスクであること）を述べる語が含まれ、書き改める前の範囲の記述（たとえばガイド 7.1 の今の文、0030 設計書 3.7 のファイルの内容のダイジェストの記述）が残っていないことを確かめる。文書の節や表の行を取り出して語を確かめる先例は、`cmd/yt2column/docs_test.go` の `sectionByHeading`・`bulletContaining`（`:730-747` で使う）と、`cmd/yt2column/fixpr_docs_test.go` の `requireSection`・`requirePhrases` である。これらは別の `main` パッケージにあり、import できないので、`cmd/fixprguard/docs_test.go` に取り出しの関数を置き、ステップ 3-3 と 4-2 で共有する。
-   **0033 の計画の確かめの番号は、PR-1 で合わせる（ステップ 3-4）:** 0033 の計画の 1.3.4 とステップ 2-0 は、インストール済みのツールに `--protocol 1` を渡す確かめを、`protocol` の番号が変わらない前提で書いている。PR-1 のマージの後に 0033 のステップ 2-0 を実行すると、番号の不一致で違う診断が出る。再インストールを求める案内が繰り返されるだけで、先に進めなくなる。`--protocol` のガードは `docs/tasks/` を読まないので、この食い違いを見つけない。PR-1 が番号を変えるので、PR-1 で 0033 の計画を editorial correction で直す。
-   **設計書 2.2 の表に `cmd/fixprguard/test_helpers.go` を加えた（editorial correction、この計画の PR）:** `validBaseline`（`:358-369`）は `"v1"` のトークンを `ParseBaseline` に通すので、版を上げると変えなければならない。設計書は `main_test.go:25` だけを挙げていた。決定は変わらないので、`Status` を `approved` のまま、`Comments` に記録した（`docs/dev/developer_guide/requirements_process.md`「Editing an approved document」）。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-09 は、すべて設計書の付録 A に記録されている。
-   `implementation_handoff.md` の I-01: ステップ 3-1 で、`ThreadPaths` が起動するのと同じ `gh api graphql` と権限の照会（`threads.go:180`・`trust.go:140`）を、`GIT_TRACE` に出力先のファイルを指定して直接実行する。そして、起動された git と、その git がブランチの設定を読むかどうかを記録する。`fixprguard` を通して実行しない理由は 1.3.4 の「環境変数」である。ほかのブランチの `branch.<name>.*` を読む git があれば、ガイド 8 章の `response-rejected` の行に反映する（ステップ 3-2）。
-   `implementation_handoff.md` の I-02: ステップ 3-1 で、通常の PR について確かめる（1.4）。`branch.<そのブランチ>.*` のほかに書き込む場合は、そこで止まり、扱い（受け入れるか、要件に戻すか）を利用者に尋ねる。fork の PR は確かめない（1.4）。

### 1.6. ステップ 3-1 の確かめの結果（2026-10-22）

手元の版は git 2.50.1（Apple Git-155）、`gh` 2.96.0 である。利用者の承認を得て行った。

-   **I-01:** このリポジトリの worktree で、`GIT_TRACE` に出力先のファイルを指定して、次の 3 つを直接実行した。
    -   対照: `gh repo view --json nameWithOwner`。trace に `git remote -v` と `git config --get-regexp '^remote\..*\.gh-resolved$'` が出た。
    -   `ThreadPaths` と同じ形の `gh api graphql --input -`（owner・name・PR #429 を変数に明示したスレッドの照会）。応答は成功した。
    -   `gh api --include repos/isseis/yt2column/collaborators/coderabbitai/permission`。HTTP 200 で成功した。

    後の 2 つは、trace のファイルを作らなかった。つまり git を 1 回も起動しなかった。対照の `gh repo view` が起動する `git remote -v` は、ほかのブランチの設定を読み込むが、宛先の決定には使わない。ただし、値のない設定や `[branch ""]` の節があると失敗する（設計書 V-4、1.3.4）。ガイド 8 章の `response-rejected` の行には、`gh` の中の git（`gh repo view` の `git remote -v`）が値のない設定で失敗する場合だけを書く。
-   **I-02:** スクラッチのディレクトリに `gh repo clone isseis/yt2column` で新しく clone し、その clone で `gh pr checkout 429`（通常の PR）を実行した。前後の `git config --list --show-origin` の違いは、`file:.git/config` の `branch.isseis/0034-fixprguard-ignored-files-01.remote=origin` と `.merge=refs/heads/isseis/0034-fixprguard-ignored-files-01` の 2 行だけだった。`branch.<そのブランチ>.*` のほかには書き込まなかったので、利用者に尋ねる場合に当たらない。clone は消した。開いている PR は #429 だけで、そのブランチは手元の別の worktree で checkout されている。ステップ 3-1 の「手元のどの worktree でも checkout されていないブランチ」には当たらないが、新しい clone の中だけで checkout するので手元の worktree に影響しないとして、利用者が #429 を選んだ。fork の PR は確かめていない（1.4）。
-   **git の最低の版:** git のリポジトリの `Documentation/RelNotes/2.8.0.adoc`・`2.11.0.adoc`・`2.12.0.adoc` と、各タグの文書で確かめた。
    -   `git config --show-origin`: 2.8.0 のリリースノートに「"git config" learnt a new "--show-origin" option」とある。`v2.8.0` の `git-config.txt` に `--show-origin` があり、`v2.7.0` にはない。
    -   `git status --porcelain=v1`: `v2.11.0` の `git-status.txt` に `--porcelain[=<version>]` と `--porcelain=v1` があり、`v2.10.0` にはない。
    -   `protocol.<name>.allow`: 2.12.0 のリリースノートに「Finer-grained control of what protocols are allowed for transports ... via a new configuration mechanism」とある。`v2.12.0` の `config.txt` に `protocol.allow`・`protocol.<name>.allow` があり、`v2.11.0` にはない。

    よって、設計書 1.3 の最低の版 2.12 は変わらない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 設定の値の一覧の分解と分類（設計書 8 章の 1）

#### ステップ 1-1: `configListDigest`

**変更するファイル**: `internal/fixprguard/configlist.go`（新規）、`configlist_test.go`（新規）、`errors.go`

-   [x] `errConfigList`・`mainBranch`・`configListDigest` を、設計書 3.3 の定義とコメントのとおりに加える。受理する形、分類、一覧のダイジェストは設計書 3.2 のとおりにする。外部のコマンドを起動せず、ファイルも読まない。
-   [x] `configlist_test.go` に、設計書 7.1 の「受理する形」「値の区別」「分類」「層の区別」の項目を確かめるテストを加える。入力は git を起動せずにバイト列で与える。拒否は `errors.Is(err, errConfigList)` と `errors.Is(err, ErrGitFailed)` で確かめ、拒否のときにダイジェストを返さないことも確かめる。エラーの文字列が入力の名前・値・読み込み元を含まないことを確かめる（AC-16）。
-   [x] 拒否の行は、その行が狙う確認だけが拒否する入力にする（たとえば「名前が空」の行と「`.` を含まない名前」の行が、互いの確認で拒否されないようにする）。設計書 7.1 の例に加え、空の読み込み元の行を置く（設計書 3.2「読み込み元は、空でないバイト列」）。確認が重なって分けられない場合は、その確認を 1 つにまとめ、まとめたことをテストのコメントに書く。
-   [x] 分類の行には、要件書 AC-01〜AC-09 の例（`branch.feature.a.remote`、`[branch "Feature"]`、`branch..remote`、`branch.main.*`、`branch.autoSetupMerge`、`file:/etc/gitconfig` を読み込み元とする項目、`remote.pushDefault`、複数の値の順序）を含める。境界として、LF を含む値、`branch` で始まる別のセクション（`branchx.a.b`。比べる項目）、大文字・小文字だけが `main` と違う `branch.Main.*`（外す対象）、形の違うブランチのダイジェスト（空、63 桁・65 桁、大文字の 16 進数、16 進数でない文字）も置く。外す対象の項目の追加・変更・削除の行では、分類なしのダイジェストが変わることを先に確かめる（1.2）。

**壊して確かめる**: 次で、それぞれ括弧の中の行が失敗すること。

-   分類を外す（すべての項目を比べる）（AC-01〜AC-04 の行）。
-   基準値のブランチの照合を外す（AC-05 の行）。`main` の照合を外す（AC-06 の行）。
-   サブセクションの取り出しを「最初の `.` と最後の `.` の間」から「2 つめの `.` まで」に変える（`branch.feature.a.remote` の行）。セクションの照合を前方一致にする（`branchx.a.b` の行）。
-   項目の区切りをダイジェストに含めない（値を持たない項目と空の値の項目の行）。
-   ブランチのダイジェストの形の確かめを外す（形の違うダイジェストの行）。
-   形の確かめ（最後の NUL、空の出力、空の読み込み元、空の名前、`.` を含まない名前、空のセクション、空の変数の名前）を 1 つずつ外す（それぞれの拒否の行）。
-   `errConfigList` に名前を付けて包む（エラーの文字列の行）。

**完了条件**: `go test -tags test ./internal/fixprguard` と `make lint` が通る。

### フェーズ 2: 基準値と比較の順序（設計書 8 章の 2）

#### ステップ 2-1: テストの git の記録

**変更するファイル**: `internal/fixprguard/test_helpers_git.go`

-   [x] `gitRouter` の記録で、`git config --list` を `git config --get` と区別できるようにする（1.3.5）。ほかのサブコマンドの記録は変えない（既存のテストが `"git add"`・`"git commit"`・`"git push"` で比べている。`worktree_flow_test.go:291-295`・`:437`）。

**完了条件**: `go test -tags test ./internal/fixprguard` が通る（この時点では既存のテストの期待値を変えない）。

#### ステップ 2-2: 基準値の版、場所と種類のダイジェスト、比較の順序

版を上げると、既存のテスト、`cmd/fixprguard` のテストのヘルパー、コマンドの文書の `--protocol` が同時に食い違うので、このステップは 1 つのコミットにする（設計書 8 章の 2）。

**変更するファイル**: `internal/fixprguard/gitstate.go`、`types.go`、`gitstate_test.go`、`worktree_flow_test.go`、`cmd/fixprguard/main_test.go`、`cmd/fixprguard/test_helpers.go`、`.claude/commands/fixpr.md`、`.claude/commands/runpr.md`、`docs/tasks/0032_japanese_pr_title/check_rules.sh`

-   [x] `baselineVersion` を `v2` にする（設計書 3.1）。
-   [x] `configDigest` を `gitDirsDigest` に改め、1.3.3 のとおりファイルの種類だけを含める（設計書 3.1・3.3）。
-   [x] `Worktree.configListDigest` を `Worktree.configList` に改める（設計書 3.3）。
-   [x] `CheckState` を、設計書 3.4 の手順 1〜4 と 4 章の表のとおりにする。`configListDigest` が返す `errConfigList` は `ErrGitFailed` を包むので、そのまま返さず `config` の変化にする（4 章の表の 6 行）。
-   [x] `Baseline` を、設計書 3.4 の「`Baseline`（`begin`）」のとおりにする。`begin` での一覧の読み取りの失敗は、今の `gitFailure` の規則の名前で `git-failed` にする（設計書 4 章）。
-   [x] `gitHardening`・`Baseline` の型・`CheckState` のコメントを、設計書 3.5「コードのコメント」のとおり改める。
-   [x] `Protocol` を `2` にし、`fixpr.md`・`runpr.md` の `--protocol 1` を、コードブロックの外の文（`fixpr.md:44`・`runpr.md:18`）も含めてすべて `--protocol 2` にする（設計書 3.5）。`check_rules.sh:524` も合わせ、`sh docs/tasks/0032_japanese_pr_title/check_rules.sh fixpr_title` が通ることを確かめる。
-   [x] `cmd/fixprguard/main_test.go:25` と `cmd/fixprguard/test_helpers.go` の `validBaseline` の版を `v2` にする（1.3.1）。
-   [x] 設計書 7.3 の表の 6 つの既存のテストを、表の「改訂」のとおりに改める。`TestBaselineMalformedRejected` の `version` の行は、前の版の `v1` を拒否する行にする（AC-15）。`worktree_config` の行は、`begin` の前に `extensions.worktreeConfig` を真にし、`config.worktree` も作っておく（1.3.3）。
-   [x] `rg -n 'w\.configListDigest|\) configListDigest' internal/fixprguard` が何も返さないことを確かめる（1.3.2）。

**壊して確かめる**: 次で、それぞれ括弧の中のテストが失敗すること。ステップ 2-3 のテストを加えた後にもう一度行う。

-   一覧の比較を `git rev-parse` の後に移す（`TestGitNotLaunchedAfterConfigChange`・`TestFinishStopsWhenHooksChangeConfig`・`TestPushRechecks` の `config_changed`）。
-   版を `v1` に戻す（`TestBaselineMalformedRejected` の前の版の行）。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 2-3: 結合テスト

**変更するファイル**: `internal/fixprguard/test_helpers_git.go`、`gitstate_test.go`、`worktree_flow_test.go`、`cmd/fixprguard/main_test.go`

-   [x] `test_helpers_git.go` に、`git config --list` の呼び出しだけを、テストが渡した出力とエラーの組で差し替え、ほかの git は実物に渡す仕組みを加える（設計書 2.2）。差し替えは、呼び出しを記録してから結果を返し、git を起動しない。エラーの値は、テストが作って渡す（このファイルは `_test.go` でないので、linter の除外が当たらない。1.3.4）。別の worktree と、`testBranch` でない名前のブランチ（`.` を含む名前、見分けやすい名前）を作る補助も加える。
-   [x] 設計書 7.2 の表の各行の入力で、`begin` の後に設定を変え、`compareState` の結果を確かめる。止まる行（AC-05〜AC-09・AC-11・AC-13）は `TestBaselineDetectsChange` に加え、止まらない行（AC-01〜AC-04・AC-10）は別のテストにまとめる。AC-03 には `includeIf` で読まれるファイルの行も置き、worktree の config の行は `begin` の前に `config.worktree` を作っておく（1.3.3）。AC-08 の読み込み元を移す行は、読み込みの順序も変わることをテストのコメントに書く（読み込み元だけの違いはステップ 1-1 で確かめる）。
-   [x] `TestBaselineDetectsChange` で git の記録を受け取り、行ごとに止まった層を確かめる（1.2）。場所と種類の行（`git_file_points_elsewhere`・`commondir_added` など）では git が 1 回も起動されないこと、一覧の行では `git config --list` が起動され、`git rev-parse` が起動されないことを確かめる。
-   [x] 止まらない行は、1.2 の層の区別に従う。AC-01〜AC-04 は、変更の前後で `git config --list --show-origin -z` の出力が変わったことを先に確かめる。AC-10 は、config のファイルのバイト列が変わり、一覧の出力が変わらないことを先に確かめる。
-   [x] AC-01 の流れのテストを `worktree_flow_test.go` に加える。別の worktree での操作（`origin/feature` からの `git worktree add -b X`、`git push -u`、`git worktree remove` と `git branch -d X`）を、1 つの回の `inspect`・`finish`・`push` の間に 1 つずつはさみ、どれも止まらないことを確かめる。各操作の後に、一覧の出力が変わったことも確かめる。
-   [x] AC-12 のテストを加える。`begin` の後の構文の誤りは、実物の git で確かめる（`TestConfigListUnreadable`）。タイムアウトは、差し替えで `errors.Join(context.DeadlineExceeded, &cmdrunner.ExitError{…})` を返して確かめる（次の項目の振り分けのテストの 3 行）。どちらも `config` で止まり、`git rev-parse` を起動しないことを、記録で確かめる。
-   [x] 設計書 4 章の表の振り分けを、差し替えで確かめる。`CheckState` では、1 行（上限超過）・2 行（`errors.Join(context.Canceled, &cmdrunner.ExitError{…})`）・5 行が `git-failed`（`errors.Is(err, ErrGitFailed)` で、`*GitStateError` でない）、3 行・4 行・6 行（終了コード 0 で受理する形でない出力）が `config` になること。`begin` では 3・4・6 行も `git-failed` になり、基準値を返さないこと。
-   [x] AC-14 の、リポジトリの config を FIFO に置き換えると git を 1 回も起動せずに `config` で止まるテスト（`TestConfigFileTypeCheckedBeforeGit`）を加える（設計書 V-7）。このテストは、差し替えで `git config --list` をすぐに失敗させておく。種類の確かめを外して壊したときに、実物の git が FIFO の読み取りで `gitTimeout`（2 分）まで待たないようにするためである。AC-11 の `.git` ファイルの書き換えで git を 1 回も起動しないことは、上の `TestBaselineDetectsChange` の層の確かめで確かめる。
-   [x] AC-15: `cmd/fixprguard/main_test.go` に、`inspect`・`finish`・`push` について、`validBaseline` の版だけを `v1` に替えたトークンを渡すと終了コード `2` になる行を加える。ほかの引数は `parse` が受け付ける値にし、同じ行を `v2` のトークンで渡すと `parse` が受け付けることも確かめる（先例: `TestProtocolMismatch`、`main_test.go:42-62`）。git と `gh` の fake には手順を持たせない（先例: `TestAcceptThreadFlagParsing`、`main_test.go:100-128`）。
-   [x] AC-16: `cmd/fixprguard/main_test.go` に、今のブランチと外す対象のブランチに見分けやすい名前を付け、今のブランチの設定を変えて `inspect` を止めるテストを加える。まず、終了コードが `1`、`stop` が `git-state-changed`、`parts` が `config` を含むこと、見分けやすい名前が `git config --list` の出力に現れることを確かめる。そのうえで、基準値のトークン、止まったときの標準出力、標準エラーが、ブランチの名前、設定の名前と値、読み込み元のパスを含まないことを確かめる。

**壊して確かめる**: 次で、それぞれ括弧の中のテストが失敗すること。

-   分類を外す（止まらない行のテストと、AC-01 の流れのテスト）。
-   `gitDirsDigest` にファイルの内容を含める（AC-10 の行と、AC-01・AC-02 の止まらない行）。
-   場所と種類のダイジェストから種類を外す（FIFO の行。git を起動しないことの確かめで失敗する）。
-   `CheckState` で一覧の読み取りの失敗を `git-failed` にする（AC-12 のテストと振り分けの 3・4 行）。
-   `CheckState` が `configListDigest` のエラーをそのまま返す（振り分けの 6 行）。
-   `CheckState` の判定の順序で、`ExitError` を `context` のエラーより先に判定する（振り分けの 2 行）。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

### フェーズ 3: ガイド（設計書 8 章の 3）

#### ステップ 3-1: ネットワークを要する確かめと git の最低の版

ガイドに書く内容の前提を確かめる。設計書 8 章の 3 のガイドの改訂の前に置く。

**変更するファイル**: `docs/tasks/0035_fixprguard_branch_config/03_implementation_plan.md`（結果の記録）、必要なら `internal/fixprguard/gitstate.go`

-   [x] 利用者の承認を得てから行う（`CLAUDE.md`「Tool Execution Safety」）。
-   [x] I-01: `GIT_TRACE` に出力先のファイルを指定して、`ThreadPaths` と同じ `gh api graphql`（スレッドの照会）と権限の照会の `gh api` を直接実行する（1.5）。2 つの呼び出しは owner と name を明示して渡すので、`gh` が git を一度も起動しないこともありうる。そのため、まず同じ出力先で git を起動することが分かっている `gh repo view`（設計書 V-6）を実行し、trace に git の記録が出ることを確かめる（対照）。そのうえで、2 つの呼び出しが成功したこと（出力があること）と、起動された git とそれがブランチの設定を読むかを記録する。git の記録がなければ、「git を起動しなかった」を結果とする。
-   [x] I-02: スクラッチのディレクトリに、このリポジトリを新しく clone する。手元のどの worktree でも checkout されていないブランチの PR を選び、その clone で `gh pr checkout` を実行する。前後の、その clone の `git config --list --show-origin` を比べて、書き込まれた設定の名前を記録する。共有の `.git/config` には書き込まない（ほかのセッションの `fixprguard` が止まるため）。`branch.<そのブランチ>.*` のほかに書き込む場合は、止まって利用者に尋ねる（1.5）。終わったら clone を消す。
-   [x] git の最低の版（2.12）の根拠（設計書 1.3「git の最低の版」の 3 つの機能が入った版）を、git のリリースノートで確かめ直す。違えば、最低の版を変えることは設計書の決定の変更なので、止まって `.claude/commands/runplan.md` の決定の変更の手順（`docs/dev/developer_guide/requirements_process.md`「Editing an approved document」）に従う。利用者の承認を得てから、設計書と、ステップ 2-2 で書いた `gitHardening` のコメントを直し、ステップ 3-2 に進む。
-   [x] 結果を本書に追記する（1.6）（コマンド、`gh`・git の版、日付）。追記は実装の記録なので、`Status` は変えない。
-   [-] （前提は外れなかったので不要）I-02 で前提が外れ、利用者が要件に戻すと決めた場合は、ステップ 2-3 の AC-01 のテストを見直す。I-01 の結果は、ガイドの記述だけに効く（1.5）。

**完了条件**: 3 つの確かめの結果が本書に記録されている。

#### ステップ 3-2: ガイドの改訂

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`

-   [x] 日本語版を、設計書 3.5 の「ガイド（英語・日本語）」の表のすべての箇所について改める。8 章の `response-rejected` の行と 7.2 には、ステップ 3-1 の I-01・I-02 の結果（確かめた範囲と、fork の PR を確かめていないこと）を反映する。
-   [x] 書いた記述を、ツールの振る舞いと突き合わせる。比べる範囲は `configListDigest`、止まり方は設計書 4 章の表とステップ 2-3 のテスト、最低の版はステップ 3-1 の結果、基準値の例の `'v2....'` は `baselineVersion` と比べる。
-   [x] 日本語版をコミットしてから、英語版を `.claude/commands/mktrans.md` の原則で翻訳する（`CLAUDE.md`「Translation Guidelines」）。

**完了条件**: 両方の版の、設計書 3.5 の表のすべての箇所が改まっている（ステップ 3-3 のガードで確かめる）。

#### ステップ 3-3: ガイドとコマンドの文書のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [x] `--protocol` の番号のガードを加える（1.4）。両方のガイドと `.claude/commands/` の下のすべての Markdown の `--protocol <数字>`、ガイドの `"protocol":<数字>`、ガイド 4 章の現在の番号が、`fixprguard.Protocol` と一致することを確かめる。両方のガイドと `fixpr.md`・`runpr.md` では、1 つ以上見つかることも確かめる。
-   [x] 文書の節や表の行を取り出す関数を置く（1.4）。ステップ 4-2 でも使う。
-   [x] AC-18 のガードを加える。両方のガイドの、要件書 AC-18 が挙げる箇所（1 章の層 (3)、5 章の `inspect`・`push`、7.1 の行、7.2、8 章の `git-state-changed` の行）のそれぞれが、(a)〜(c) を述べる語を含み、書き改める前の範囲の記述を含まないことを確かめる。7.1 の行は、外す対象を除いた config を比べること、比べる前に起動する git が `git config --list` だけであることを述べる語を含むことも確かめる。日本語版を確かめるリテラルは日本語で書く（1.2）。

**壊して確かめる**: 次で、それぞれのガードが失敗すること。一時的に書き換えて確かめ、戻す。

-   ガイドの例の 1 か所、`fixpr.md:44` の文、ガイド 4 章の番号を、それぞれ `1` に戻す。
-   両方のガイドのそれぞれで、AC-18 の箇所の (a)・(b)・(c) の文を 1 つずつ消す。7.1 の行に、書き改める前の文を戻す。

**完了条件**: `go test -tags test ./cmd/fixprguard` が通る。

#### ステップ 3-4: 0033 の計画の番号

**変更するファイル**: `docs/tasks/0033_multiple_trusted_reviewers/03_implementation_plan.md`

-   [x] この時点で 0033 のステップ 2-0 が完了していなければ、ステップ 2-0（`:292`）の手順の `--protocol 1` を、実行の時点の `Protocol` の番号を渡す形に改め、「`protocol` の番号は変わらない」の前提の文を、0035 で番号が変わったことに合わせる（1.4）。1.3.4（`:101-113`）の観察の記録（当時のツールの出力）は、記録として残し、書き換えない。0033 の計画の `Status` は保ち、`Comments` に editorial correction として記録する。完了していれば、何もしない。（記録: このブランチでは、いったん未完了として 0033 の計画を改めた。その後、0033 の PR-2（#436）がステップ 2-0 を完了した状態で `main` にマージされた。そのため、`main` を取り込んだときに、0033 の計画は `main` の版に戻した。完了した確かめの記録は、当時のツールに渡した番号のまま残す。）
-   [x] 0033 のステップ 2-0 の確かめ（`--reviewer` の繰り返しを受け付けるか）の意味は、番号を合わせても変わらないことを確かめる。0033 の PR-1（#434）は本タスクのブランチの起点より前にマージされているので、0035 のツールは `--reviewer` の繰り返しを受け付ける。0033 の PR-1 の後のツールとして、PR 番号の文法の診断を返す（`list --protocol 2 --reviewer coderabbitai --reviewer chatgpt-codex-connector x` で終了コード `2`、`fixprguard: invalid input: PR number grammar`。`--protocol 1` では番号の不一致の診断になる。2026-10-22、このブランチでビルドしたツールで確かめた）。0033 の設計書の番号の記述（`02_architecture.md:65`）は、設計書 9 章のとおり 0033 の実装の PR が合わせるので、ここでは変えない。

**完了条件**: 0033 の計画が、未実行の確かめで `Protocol` と違う番号を渡していない。

#### ステップ 3-5: PR-1 の確かめ

**変更するファイル**: なし

-   [x] `make test && make lint` を実行する。
-   [x] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行する。

**完了条件**: 上の 2 つが通る。

### PR-1 作成ポイント: fixprguard ignores other branches' config

**対象ステップ**: 1-1 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0035): fixprguard の config の比較から、ほかのブランチの branch 設定を外す`

**レビュー観点**: 一覧の分類が、今のブランチの設定、`main` の設定、`branch` 以外の設定を比べ続け、受理する形に合わない出力を拒否すること / `CheckState` の起動の順序（種類の比較 → `git config --list` → `git rev-parse`）と、一覧の読み取りの失敗の振り分け（設計書 4 章。特に 6 行の `config`） / 止まらない行のテストが、層を外すと失敗する入力を使っていること / `Protocol` 2 とコマンドの文書・ガイド・0033 の計画の番号、`/runpr` の前の再インストール。コードと文書の変更が多いので、コミットの順にレビューする

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2 は、git を起動する前の比較の順序という、ガードツールの保証（ガイド 7.1）を変えるセキュリティのゲートの変更である（`mkplan.md` step 8 の panel mode の契機）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/437
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**`/runpr` の前**: 利用者は、PR-1 のブランチの作業ツリーで `make install-fixprguard` を実行してから `/runpr` を実行する（1.4）。マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は終了コード `2` で止まる。

**マージの後**: 利用者は、マージの後の `origin/main` を含む作業ツリーで `make install-fixprguard` を実行し直す。作業中のほかのブランチには、次の `/fixpr`・`/runpr` の前に `main` を取り込む（設計書 3.5「移行の手順」）。

### フェーズ 4: 0030 の文書（設計書 8 章の 4）

#### ステップ 4-1: 0030 の文書の改訂

**変更するファイル**: `docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md`、`02_architecture.md`

-   [x] 設計書 3.5「0030 の文書」の表のすべての箇所を、要件書 AC-17 の (a)〜(c) を述べる形に改める。0030 `02_architecture.md` 2 章ではなく 1.2 の push の決定を対象とすることは、設計書 3.5 の表の後の段落のとおりである。
-   [x] 両文書の `Status` を `approved` のままにし、`Comments` に `YYYY-MM-DD (decision): タスク 0035（#431）の要件で決めた変更` の形で記録する（設計書 3.5、要件書 5 章）。日付は改訂した日にする。
-   [x] 改めた記述を、ガイド（フェーズ 3）と設計書 3.4・5 章と突き合わせ、比べる範囲と残余のリスクの述べ方が一致することを確かめる。

**完了条件**: 設計書 3.5 の表のすべての箇所が改まっている（ステップ 4-2 のガードで確かめる）。

#### ステップ 4-2: 0030 の文書のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] 0030 の両文書の、設計書 3.5 の表の箇所のそれぞれが (a)〜(c) を述べる語を含み、書き改める前の範囲の記述を含まないこと、両文書の `Comments` が `(decision)` とタスク 0035 を含むこと、`Status` が `approved` であることを確かめるガードを加える。箇所の取り出しには、ステップ 3-3 の関数を使う。

**壊して確かめる**: 両文書のそれぞれで、箇所の (a)・(b)・(c) の文を 1 つずつ消す、書き改める前の文を戻す、`Comments` の記録を消す、のそれぞれでガードが失敗すること。

**完了条件**: 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。

#### ステップ 4-3: PR-2 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。

**完了条件**: 上が通る。

### PR-2 作成ポイント: task 0030 documents describe the config scope

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `docs(0035): 0030 の要件書と設計書の config の比較の範囲を改める`

**レビュー観点**: 設計書 3.5 の表のすべての箇所が (a)〜(c) を述べ、古い範囲の記述が残っていないこと / ガイド（PR-1）と 0030 の文書の述べ方が一致すること / `Status` を `approved` のまま `Comments` に `(decision)` を記録したこと

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
| M3 | I-01・I-02・最低の版の確かめの結果 | ステップ 3-1 の完了条件 |
| M4 | ガイド、ガード、0033 の計画の番号 | PR-1 のマージ |
| M5 | 0030 の文書とガード | PR-2 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | `configlist.go`、`gitstate.go` の比較の順序と振り分け、基準値 `v2`、`Protocol` 2、コマンドの文書の `--protocol`、結合テスト、I-01・I-02 の確かめ、ガイド、番号と AC-18 のガード、0033 の計画の番号 | frontier-required |
| PR-2 | 4-1 / 4-2 / 4-3 | 0030 の要件書と設計書の改訂、AC-17 のガード | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。

-   フェーズ 1 は外部のコマンドに依存しないので最初に置く。フェーズ 2 はその関数を使う。
-   フェーズ 2 の中では、記録の区別（2-1）を、期待値を変える 2-2 より先に置く。2-2 の既存のテストの改訂が、`git config --list` を区別した記録を使うためである。`git config --list` の差し替えと補助は、それを使うテストと同じ 2-3 で加える（`test_helpers_git.go` は `_test.go` でないので、使われない補助は `unused` に報告される。1.3.4）。
-   I-01・I-02 と最低の版の確かめ（3-1）は、その結果を書くガイド（3-2）の前提なので、フェーズ 3 の最初に置く。
-   0030 の文書（フェーズ 4）は、ガイドの改訂の後に置く（設計書 8 章の 4。要件書は承認済み）。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の項目を、ステップ 1-1（受理する形・値の区別・分類・層の区別）とステップ 2-3（エラーの振り分け）に割り当てた。システムの config を読み込み元とする外す対象の項目（AC-03）と、読み込み元だけの違い（AC-08）は、実際のリポジトリでは再現できないので、ユニットテストで確かめる（設計書 7.1）。

### 4.2. 結合テスト

設計書 7.2 の表の各行を、ステップ 2-3 で一時ディレクトリの実際のリポジトリに対して確かめる（ガイド 9 章）。リポジトリ、リモート、HOME は `t.TempDir` の下にあり、実行ごとに作り直される（`test_helpers_git.go:85-110`）。`gh` と make は fake を使う。`gh pr checkout` は再現せず、`git config` による直接の書き込みで代える（設計書 7.2 の AC-01）。V-4 の振る舞い（値のない項目で `git ls-remote` が失敗すること）は git の振る舞いなので固定しない（設計書 7.2）。

### 4.3. セキュリティテスト

設計書 7.6 の対応のとおりである。経路 1 は AC-08 と AC-14、経路 4 は AC-12、経路 5 は AC-14 の FIFO の行、`.git` ファイルの書き換えは AC-11 で確かめる。7.4 の壊す確認は、ステップ 1-1・2-2・2-3 の「壊して確かめる」に割り当てた。

### 4.4. 文書のガード

ステップ 3-3 と 4-2 のガードは `cmd/fixprguard/docs_test.go` に置き、CI の `doc-guard` のジョブが実行する。

### 4.5. 後方互換性

前の版の基準値は受け付けない（AC-15）。前の版のバイナリと新しい文書、新しいバイナリと前の文書の組み合わせは、`--protocol` の不一致で終了コード `2` で止まる（AC-19）。既存の `TestProtocolMismatch`（`cmd/fixprguard/main_test.go:42-62`）が、`Protocol` と違う番号を拒否することを確かめる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は振る舞いを確かめる Go のテスト、`static` は文書を読む Go のガードのテスト、`manual` は PR の上での観察を表す。新しいテストの名前は仮の名前で、実装で変えたときは本書の表を同じ PR で改める（1.3.5）。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | test | `internal/fixprguard/configlist_test.go::TestConfigListDigestClassification`、`internal/fixprguard/gitstate_test.go::TestOtherBranchConfigIgnored`（`git config`）、`internal/fixprguard/worktree_flow_test.go::TestRoundIgnoresOtherBranchConfig`（`git worktree add -b`・`git push -u`・`git worktree remove`・`git branch -d`） |
| AC-02 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestOtherBranchConfigIgnored`（節の追加と削除、`[branch ""]`） |
| AC-03 | test | `configlist_test.go::TestConfigListDigestClassification`（システムの config を含む読み込み元）、`gitstate_test.go::TestOtherBranchConfigIgnored`（グローバル、worktree の config、`include.path`・`includeIf` の先） |
| AC-04 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestOtherBranchConfigIgnored`（`branch.<b>.a.remote`、大文字・小文字だけが違う名前） |
| AC-05 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestBaselineDetectsChange`（今のブランチ、`.` を含む名前、`[branch.Feature]`） |
| AC-06 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestBaselineDetectsChange`（`branch.main.*`） |
| AC-07 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestBaselineDetectsChange`（`branch.autoSetupMerge`） |
| AC-08 | test | `configlist_test.go::TestConfigListDigestClassification`（読み込み元だけの変化）、`gitstate_test.go::TestBaselineDetectsChange`（既存の `config`・`global_config`・`included_config`・`linked_worktree_common_config` と、加える `remote.pushDefault`・`remote.<name>.*`・`include.path`・読み込み元の移動の行） |
| AC-09 | test | `configlist_test.go::TestConfigListDigestClassification`、`gitstate_test.go::TestBaselineDetectsChange`（複数の値の順序） |
| AC-10 | test | `gitstate_test.go::TestOtherBranchConfigIgnored`（コメント・空行・字下げだけの追加。ファイルのバイト列が変わり、一覧の出力が変わらないことを先に確かめる） |
| AC-11 | test | `gitstate_test.go::TestBaselineDetectsChange`（既存の `git_file_points_elsewhere`・`commondir_added`。git を起動しない） |
| AC-12 | test | `gitstate_test.go::TestConfigListUnreadable`（構文の誤り。`git rev-parse` を起動しない）、`::TestConfigListFailureKinds`（設計書 4 章の表の 1〜6 行の振り分け。タイムアウトを含む。`CheckState` と `begin`） |
| AC-13 | test | `gitstate_test.go::TestBaselineDetectsChange`（既存の `branch_switch_same_commit` と、加える detached HEAD の行） |
| AC-14 | test | `gitstate_test.go::TestGitNotLaunchedAfterConfigChange`（改訂した 2 行）・`::TestConfigFileTypeCheckedBeforeGit`（FIFO）、`worktree_flow_test.go::TestFinishStopsWhenHooksChangeConfig`・`::TestPushRechecks`（`config_changed`）、`cmd/fixprguard/main_test.go::TestInspectChecksGitStateBeforeGh`（既存） |
| AC-15 | test | `gitstate_test.go::TestBaselineMalformedRejected`（前の版の行）、`cmd/fixprguard/main_test.go::TestPreviousBaselineRejected`（終了コード `2`。`v2` のトークンの対照の行） |
| AC-16 | test + static + manual | `cmd/fixprguard/main_test.go::TestConfigStopOmitsNames`、`configlist_test.go`（エラーの文字列）、`cmd/fixprguard/docs_test.go::TestGuideCoversStopKindsAndScope`（既存。止まり方の種類） |
| AC-17 | static | `cmd/fixprguard/docs_test.go::TestTask0030DocsDescribeConfigScope` |
| AC-18 | static | `cmd/fixprguard/docs_test.go::TestGuideDescribesConfigScope` |
| AC-19 | static + test | `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（既存）・`::TestDocumentedProtocolMatches`（ガイドとコマンドの文書のすべての番号）、`cmd/fixprguard/main_test.go::TestProtocolMismatch`（既存） |

-   AC-17・AC-18 は、文書が何を述べるかを定めるので、`static` で確かめる。
-   AC-16 の「`parts` の値の集合は変わらない」は、`Part*` の定数を加えも消しもしないことを、PR-1 の差分で確かめる（`manual`）。ステップ 2-3 のテストは、止まったときの `parts` を既存の値で確かめる。
-   AC-19 のうち「前の版の値より大きい」ことは、PR-1 の差分で `1` から `2` への変更を見て確かめる（`manual`）。定数をそれ自身のリテラルと比べるテストは書かない（`CLAUDE.md`「Testing Strategy」）。

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
- [ ] PR-2 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 対策 |
|---|---|
| 止まらない行のテストが、層を外しても通る入力で書かれる | 1.2 の層の区別に従い、外す対象の変更では一覧の出力が、AC-10 ではファイルのバイト列が変わることを先に確かめる（ステップ 1-1・2-3） |
| `digestPath` を分けるときに、info や hooks の内容の比較まで外れる | 既存の `info_exclude`・`info_attributes`・`hook_link_target_content` の行で確かめる（1.3.3） |
| `CheckState` が `errConfigList` をそのまま返し、`config` でなく `git-failed` になる | 振り分けの 6 行のテストと壊す確認で確かめる（ステップ 2-3） |
| `CheckState` の振り分けで、`context` の取り消しを `config` の変化として扱う | `ExitError` より先に `context` のエラーを判定する。振り分けの 2 行のテストと壊す確認で確かめる（ステップ 2-3） |
| PR-1 の `/runpr` が、インストール済みのツールの番号の不一致で止まる | PR-1 の作成ポイントの「`/runpr` の前」のとおり再インストールする（1.4）。忘れても終了コード `2` で止まるだけで、誤った判定はしない |
| 再インストールの後、ほかの worktree のセッションが止まる | `main` を取り込むまで終了コード `2` で止まる（fail-closed）。ガイド 3 章の移行の手順に書く（設計書 3.5） |
| I-02 で、通常の PR の `gh pr checkout` が `branch.<そのブランチ>.*` のほかにも書き込む | ステップ 3-1 で止まり、扱いを利用者に尋ねる（1.5） |
| git の最低の版（2.12）の根拠が、リリースノートと食い違う | ステップ 3-1 で止まり、決定の変更として利用者の承認を得てから設計書を直す |
| 0033 の計画の確かめが、本タスクの後に古い番号を渡す | ステップ 3-4 で合わせる。`--protocol` のガードは `docs/tasks/` を読まないので、ステップ 3-4 を飛ばしても検出されない |
| 0033 の設計書の番号の記述が古くなる | 設計書 9 章のとおり、0033 の実装の PR が合わせる |

### 7.2. スケジュールのリスク

-   ステップ 3-1 はネットワークと利用者の承認を要し、PR-1 に含まれる。承認を待つ間も、ステップ 3-2 のうち I-01・I-02 の結果によらない箇所は進められる。
-   要件書 2.2 のとおり、本タスクは 0034（PR #429）より先にマージする。0034 の作業は、PR-1 のマージまで止まる。
-   CodeRabbit の PR レビューは 1 時間あたりの回数が限られるので、PR を開いてからレビューが届くまで待つことがある（`runpr.md` step 3）。

## 8. 成功基準 (Success Criteria)

-   [ ] 5 章のすべての `test` と `static` の検証が通る。
-   [ ] `make test && make lint` と、PR-1・PR-2 の CI の `doc-guard` のジョブが通る。
-   [ ] 各ステップの「壊して確かめる」の結果が、コミットメッセージに書かれている。
-   [ ] ステップ 3-1 の確かめの結果が本書に記録され、ガイドに反映されている。
-   [ ] ガイドの日本語版と英語版の記述が一致する。

## 9. 次のステップ (Next Steps)

-   承認の後、`/runplan 0035` で PR-1 から実装する。PR-1 の `/runpr` の前に、PR-1 のブランチで `make install-fixprguard` を実行する。
-   PR-1 のマージの後、`main` で `make install-fixprguard` を実行し直す。0034（PR #429）を再開するときは、基準値の版を `v3` に上げて要素を足し、0034 の要件案の `gitstate.go` の行番号を合わせる（設計書 9 章）。
-   PR-2 は本タスクの最後のプランの PR なので、マージの後に issue #431 を閉じる。
