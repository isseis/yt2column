# 実装計画書：サーバが設定ファイルを読むようにする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-07 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下「要件」）の F-001 から F-008 を、[02_architecture.md](02_architecture.md)（以下「設計」）のとおりに実装する手順を定める。設計の内容は本書に写さず、節の番号で参照する。

### 1.2. 実装の原則

-   **設計の優先順位（設計 8 章）の順にフェーズを置く。** フェーズ 1 から 7 は、設計 8 章の 1 から 7 に対応する。
-   **各コミットで `make test` と `make lint` が通る。** あるフェーズの変更が既存のテストを壊すときは、そのテストの書き換えを同じフェーズに置く（1.3「サーバのテストの補助の書き換えはフェーズ 3 で行う」）。
-   **テストの削除は削除だけのコミットにし、`make coverage-compare BASE=<親のコミット>` の出力をコミットメッセージと PR の説明に書く**（`CLAUDE.md`「Deleting a test is a claim that must be checked」、設計 3.6）。既存のテストは、できるかぎり削除せずに書き換える。書き換えのつもりでテスト関数を消したときも、この手順に従う。
-   **テストは、守っているものを外して失敗することを確かめてからコミットし、そのことをコミットメッセージに書く**（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。本書は、どのテストが何を捕まえるかを主張しない。
-   **Go のコメント・識別子・文字列リテラルは英語で書く。** 例外は、文書の日本語の見出しや表の列名と一致しなければならないガードテストのリテラル（`CLAUDE.md`「Go Idioms」の guard-test literal exception）だけである。
-   **ソースのコメントや注釈に AC や F の番号を書かない。** AC との対応は本書の 7 章に置く（`requirements_process.md` §4）。

### 1.3. 計画の際に決めたこと（2026-11-07）

-   **`MkdirBelow` は、`mkdir` の間だけ umask を `077` にし、作った各ディレクトリを開いて `Fchmod` でモードを `0700` にそろえる（[implementation_handoff.md](implementation_handoff.md) I-01）。** `syscall.Umask(0o077)` で umask を変えて `os.Mkdir(name, 0o700)` を呼び、元の umask に戻す。作ったディレクトリは所有者の読み取りと探索の権限を必ず持つので、`O_RDONLY|O_DIRECTORY|O_NOFOLLOW` で開ける。開いた記述子に `fstat` してディレクトリで自分の所有であることを確かめ、`Fchmod(0o700)` で Linux の親から継承した setgid のビットも落とす。理由: 設計 3.1 の条件（umask によらず `0700`、作ったものだけを変える、OS ごとのファイルを持たない）をすべて満たすのはこの方法だけである。Linux の `O_PATH` の記述子には `fchmod` できず、Go の `syscall.Fchmodat` は Linux で `AT_SYMLINK_NOFOLLOW` を受け付けない。OS ごとの方法（Linux の `/proc/self/fd`、macOS の `O_SEARCH`）は設計の「OS ごとのファイルを持たない」に反し、パスに対する `chmod` は差し替えられたシンボリックリンクをたどる。受け入れること: umask はプロセス全体に効くので、`MkdirBelow` はほかのファイルの作成と並行して呼ばない。呼ぶのは `config init` だけで、並行する処理はない。テストでは、`MkdirBelow`・`InitServerFile`・`runInit` を呼ぶテストに `t.Parallel` を付けない（調査した範囲のテストは、どれも `t.Parallel` を使っていない。2.5）。ADR: なし（このタスクの中で閉じる）。
-   **`internal/config` の外のテストは、テスト用のアクセサで設定ファイルを書いてから `LoadServer` を呼ぶ。** `internal/config/test_helpers.go` に、与えた `HOME` の下の設定ファイルのパスに、与えたキーと値をモード `0600` で書くアクセサを置く。今 `LoadServer` に環境変数の表を渡しているテスト（2.4）は、すべてこれを使う。`HOME` は、ソケットのパスの上限に収まる短いディレクトリ（`udstest.Dir`）にする。理由: サーバの入口を通して値を確かめ続けられ、パスの規則をテストに写さずに済む。`config.Load`（CLI）に切り替える案は、サーバの入口を確かめなくなるので採らない。ADR: なし。
-   **推奨構成の設定のブロックは、設定ファイルの行の形にし、ガードテストは本物の解析で読む。** `README.md`・`README.en.md` の「推奨構成」と `docs/dev/production_profile.md` の「本番の構成」のブロックを、`export` の行から設定ファイルの行（`YT2COLUMN_LLM=...`）に変える。`TestProductionProfileDocsContract` は、ブロックを設定ファイルの書式の解析に通し、上のアクセサで書いたファイルから `LoadServer` で読み込んで、既定の構成と比べる。理由: 文書のブロックを、そのまま設定ファイルに写せる実行できる例として確かめられる。`export` のブロックを残して別に例を足す案は、2 つの形を保つことになるので採らない。ADR: なし（ADR-0016 の「実行できる例」の範囲）。
-   **PR は 4 つに分ける。** PR-1 はフェーズ 1（`internal/privatefile`）、PR-2 はフェーズ 2（書式の解析・雛形・`InitServerFile`）、PR-3 はフェーズ 3 から 5（サーバがファイルを読むこと・廃止した変数の検査の削除・`config init`）、PR-4 はフェーズ 6 と 7（拡張と文書）である。理由: `privatefile` は信頼の境界にある部品で、単独でレビューする。フェーズ 3 と 5 を同じ PR にすると、`main` に「ファイルがなければ止まり、まだない `config init` を案内するサーバ」が現れない。ADR: なし。
-   **サーバのテストの補助の書き換えはフェーズ 3 で行う。** 設計 8 章は「テストの補助の書き換え」を 5 番目に置くが、3 番目の `LoadServer` の変更で、環境変数で設定を渡すサーバのテストがすべて止まる。そのため、補助を設定ファイルを書く形に変える作業はフェーズ 3 に置き、フェーズ 5 では `config init` のための補助だけを足す。フェーズの順は設計のままである。ADR: なし。
-   **サーバの入口で廃止した変数を拒否するテストの行は、フェーズ 3 の最初に削除だけのコミットで消す。** フェーズ 3 の後、サーバは 6 つのキーをファイルから読み、ファイルは廃止した変数の名前を書式で拒否するので、環境の廃止した変数を拒否するテストの行は成り立たない。この時点では CLI のテストが `retiredLLMErrors` を通るので、削除のコミットのカバレッジの比較が意味を持つ。残りの削除はフェーズ 4 で行う。ADR: なし。

### 1.4. 既存コード調査結果

調べたのはコミット `ac6e685e`（`main`）である。行番号はこのコミットのものである。

**作り直さずに使うもの:**

-   `absoluteHome`（`internal/config/cachedir.go:74-80`）と `appDirName`（同 `:7`）。設定ファイルのパスの規則（設計 3.2.1）が使う。
-   `socketRule` の OS の分類（`internal/config/socketpath.go:30-41`）。`windows`・`plan9`・`js`・`wasip1` に規則がない分類を、設定ファイルの規則と 1 つの関数で共有する（設計 3.2.1）。`userDirBase`（`cachedir.go:47-70`）も同じ 4 つの OS を並べているが、XDG の変数を読む別の規則なので、本タスクでは触らない。
-   `loadLLM`・`loadSlackWebhook`・`socketPath`・`hasHTTP2Debug`（`internal/config/config.go:480`・`699`、`socketpath.go:49`、`config.go:769`）。`LoadServer`（`config.go:362-383`）の中で `LookupFunc` を渡し分けるだけで、値の検証は書き直さない（設計 3.2.3）。
-   `missingVar`・`missingVarWithReason`・`invalidVar`（`config.go:775-788`）。
-   `stderrWriter` と `splitJoined`（`cmd/yt2column-server/run.go:85-93`・`309-321`）。設定エラーの行の書き出しはそのまま使う。
-   開き方の前例: `internal/cachelock/cachelock.go:78-82` は `O_NOFOLLOW|O_NONBLOCK` で開き、`File.Stat` で種類を確かめる。`gosec` の抑止もここと同じく、開く 1 行に理由とともに付ける。
-   `udstest.Dir`（`internal/udstest/udstest.go:33-45`）。ソケットのパスの上限に収まる短い `HOME` を作る。
-   `TestStartupFailures`（`cmd/yt2column-server/run_test.go:42`）の表の形。設定エラーの行・終了コード・ソケットが作られないことを、この表に行を足して確かめる。

**足りないもの:** `internal/privatefile`、設定ファイルのパスの規則、書式の解析、雛形、`InitServerFile`、`FileError`・`FormatOverflowError`・`VarError.Source`、`run` の引数の振り分けと `runInit`。

**変えるもの（すべての出現を挙げる）:**

-   **`LoadServer` を呼ぶ場所（`rg -n "LoadServer\(" --glob '*.go'`、`docs/tasks/` の下を除く）:** `cmd/yt2column-server/run.go:133`（本番）、`cmd/yt2column-server/test_helpers.go:145`、`internal/llm/provider/provider_test.go:802`、`cmd/yt2column/docs_test.go:530`・`536`・`621`、`internal/config/socketpath_test.go:175`、`internal/config/config_test.go` の `TestLoadClaudeEffortDefault`（`:329` の副テスト）・`TestLoadServerValid`・`TestLoadServerLLMRejected`・`TestLoadServerWebhookOptional`・`TestServerConfigOutputRedactsWebhook`・`TestLoadServerIgnoresCacheAndYtdlp`・`TestLoadServerSocketPathUnderivable`・`TestLoadServerLLMDefaults`・`TestLoadRejectsRetiredLLMVars`（`:1850` の行）。設計 3.6 の表は `provider_test.go` と `cmd/yt2column/docs_test.go` を挙げていないが、どちらも `LoadServer` に環境変数の表を渡すので書き換える（ステップ 3-3）。
-   **サーバのテストが環境変数で設定を渡す場所:** `cmd/yt2column-server/test_helpers.go` の `serverEnv`（`:113-151`）・`startServer`（`:193`）・`startServerChild` と `childEnv`（`:570-614`）、`run_test.go`・`relay_e2e_test.go`・`signal_test.go`・`server_test.go` の `e.vars[...]` への書き込み。子プロセスのテスト（`runChildMode`、`:539-558`）は `os.LookupEnv` を `run` に渡すので、子の `HOME` の下に設定ファイルを書けば同じ形で動く。
-   **廃止した変数の検査（`rg -n "retired|Retired" cmd internal`）:** 本番は `config.go:65-67`（注釈）・`101-109`・`130-141`・`150-166`・`354-361`・`441-452`・`473-482`・`512-523`。テストとテスト用の補助は、`internal/config/test_helpers.go:50-52`・`87-107`、`config_test.go:572`・`582`・`631-636`・`659-720`・`1160`・`1187-1191`・`1447-1448`・`1658-1680`・`1814-1888`、`socketpath_test.go:150-158`・`162`・`170`・`194`・`206`・`226`、`cmd/yt2column/run_test.go:464-471`・`1830-1831`・`2239-2340`、`cmd/yt2column/makefile_test.go:46-52`・`99-208`、`internal/llm/deepseek/testutil/make.go:88-108`（`CheckRetiredModelEnvNotRead`。呼ぶのは `makefile_test.go:110` だけで、テストを消すと到達できなくなる）、`cmd/yt2column-nmhost/main_test.go:135-153`・`253`・`319`、`cmd/yt2column-server/run_test.go:88-106`・`337-342`。`internal/maketestutil/make_test.go:12` の `YT2COLUMN_MODEL` は変数名の形の例として使っているだけなので変えない。
-   **「環境変数」と書いた文字列とコメント:** `config.go:1-3`（パッケージの注釈）、`config.go:31`（「Names of the environment variables Load reads.」）、`config.go:175`・`178`（`ErrMissing`・`ErrInvalid` の文）、`run.go:110-112`（手順 1 と 2 がファイルに触れないという注釈）、`run.go:127`（引数の拒否の行）、`cmd/yt2column-server/run_test.go:30-32`（`configDocument` の注釈）、`extension/test/secretsAbsent.test.ts:28`（「The server's environment variable a message may tell the user to set.」）。
-   **拡張の案内:** `extension/src/core/i18n/en.ts:27`・`ja.ts:23-24` の `setWebhookStep`。`secretsAbsent.test.ts:31-32` は、メッセージのファイルの文字列リテラルの中でだけ `SLACK_WEBHOOK_URL` を許す。新しい文もこの名前を含むので、このテストはそのまま通る。
-   **文書のガードテスト:** `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`:34`）、`cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`（`:636`、`checkExportBlock` `:431`・`checkExportLines` `:450`・`checkExportBlockMatchesDefaults` `:524`・`checkProfileTable` `:572`）、`TestLLMDefaultsDocsContract`（`:254`、README の「変数」の列の表を読む）、`TestReadmeLinks`（README の見出しへのリンク。廃止した変数の節を消すと、それを指す `README.md:352` のリンクも消す）。README の見出しを探す場所は `internal/doctestutil/readme.go:80-109` にある。
-   **パッケージの一覧のガード:** `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（`:625`）は、`package_reference.md` がすべてのパッケージを挙げることを確かめる。`internal/privatefile` の行はフェーズ 1 で足す。
-   **軽量プロセスの除外の表:** `docs/dev/conventions/requirements_process.md` の「Mechanically checked conditions」は、信頼の境界のパッケージを挙げ、「A PR that adds a package or a file on a trust boundary adds its `exclude` line to this table in the same PR.」とする。秘密を持つファイルを読み書きする `internal/privatefile` は信頼の境界にあるので、フェーズ 1 で `exclude internal/privatefile/` を足す。
-   **テスト用の補助のビルドタグ:** `internal/pipeline` の `TestFakesCarryBuildTag` は、`test_helpers*.go` の最初の行が `//go:build test` か `//go:build test || integration` であることを確かめる。新しい `internal/privatefile/test_helpers.go` は `//go:build test` で始める。

**構築の経路（具体的な型での分岐）:** 設計は具体的な型で振る舞いを選ぶ。`privatefile` の番兵から `config` の番兵への対応は `errors.Is` の `switch`（設計 4.1）で、`run` の終了コードは「`InitServerFile` の失敗が `*VarError` か」で選ぶ（設計 3.3）。`*VarError` を作るのは `missingVar`・`missingVarWithReason`・`invalidVar` の 3 つの関数だけで（`config.go:776`・`782`・`787`。`rg -n "VarError\{" --glob '*.go' internal cmd` の本番のコードの結果はこの 3 行だけ、コミット `ac6e685e`）、`InitServerFile` が返す `*VarError` はパスの導出（`serverFileLocationFor`）からだけ来る。この組み立ての経路は、`runInit` の表のテスト（ステップ 5-3）で、`HOME` の誤りが `2`、それ以外のすべての失敗が `1` になることとして確かめる。型をリーフのパッケージに移す案は採らない（`VarError` は CLI と共有する今の公開の型で、フィールドも公開である）。

### 1.5. 実装への申し送りへの対応

| 項目 | 対応 |
|---|---|
| I-01 `MkdirBelow` が作ったディレクトリのモードを umask によらず `0700` にする | 1.3 の 1 つ目の決定。実装はステップ 1-3、umask `0400`・`0100` を含むテストはステップ 1-4 |

## 2. 実装の手順

各ステップの完了条件は、特に書かないかぎり `make fmt`・`make test`・`make lint` が通ることである。

### フェーズ 1: `internal/privatefile`（設計 8 章の 1）

#### ステップ 1-1: パッケージの骨組みとエラーの型

**変えるファイル:** `internal/privatefile/privatefile.go`（新規）、`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/requirements_process.md`

- [ ] `//go:build unix` の `privatefile.go` に、設計 3.1 の定数（`Mode`・`DirMode`）、番兵（`ErrNotExist`・`ErrUnsafe`・`ErrTooLarge`・`ErrExist`・`ErrBaseMissing`）、`UnsafeReason`（`String()` を持つ）、`UnsafeError` を置く。パッケージの注釈に、設定のことを知らないことと、`MkdirBelow` が umask を一時的に変えるのでほかのファイルの作成と並行して呼ばないことを書く。
- [ ] 所有者とモードの判定を、`fs.FileInfo`（`fstat` または `Lstat` の結果）と実効ユーザーの ID を受け取る小さな非公開の関数にする。`Read` の 2 つの検査（`Lstat` と `fstat`）がこの関数を共有する（設計 7.1）。
- [ ] `package_reference.md` に `internal/privatefile` の行を足す（`TestPackageReferenceListsPackages`）。
- [ ] `requirements_process.md` の「Mechanically checked conditions」の表に `exclude internal/privatefile/` を足す。

#### ステップ 1-2: `Read`

**変えるファイル:** `internal/privatefile/privatefile.go`、`internal/privatefile/test_helpers.go`（新規、`//go:build test`）

- [ ] 設計 3.1「`Read` の手順」と 6.1 のとおりに実装する。`Lstat` の結果は拒否にだけ使い、読んでよいと決めるのは開いた記述子の `fstat` の結果だけにする。開いた後のどの拒否でも記述子を閉じる。
- [ ] `os.OpenFile` の行に、`cachelock.go:82` と同じく、その 1 行だけを対象にした `//nolint:gosec` と理由を付ける。
- [ ] `test_helpers.go` に、`Lstat` の後に呼ばれるフックの差し込み口を置く（設計 7.3）。本番のコードからは、テスト用のビルドでないとき何もしない関数として呼ぶ。

#### ステップ 1-3: `Create` と `MkdirBelow`

**変えるファイル:** `internal/privatefile/privatefile.go`

- [ ] `Create` を設計 3.1「`Create` の手順」と 6.3 のとおりに実装する。一時ファイルの接頭辞は `.config.env.tmp-` とする。成功の後に一時ファイルの名前を消せなかったときはその名前を返し、失敗の後に消せなかったときはその名前を失敗のエラーの文に含める。
- [ ] `MkdirBelow` を、設計 3.1 の振る舞いと 1.3 の 1 つ目の決定の手順で実装する。umask は `defer` で必ず元に戻す。`EEXIST` の要素は変えずに次へ進む。作った要素を開いたときに、ディレクトリでないか自分の所有でなければ「作れなかった」とする。

#### ステップ 1-4: `privatefile` のテスト

**変えるファイル:** `internal/privatefile/privatefile_test.go`（新規）

- [ ] 設計 7.1「`internal/privatefile`」に挙げた場合を、`TestRead`・`TestReadUnreadable`・`TestReadChecksDescriptor`・`TestOwnerAndModeCheck`・`TestCreate`・`TestCreateExisting`・`TestCreateFailureLeavesNothing`・`TestMkdirBelow` に分けて書く。`TestMkdirBelow` は、umask が `0400`・`0100`・`0000`・`0077` のそれぞれで、作ったディレクトリのモードがちょうど `0700` になることを確かめる（I-01）。umask を変えるテストは、元の umask を `t.Cleanup` で戻す。
- [ ] `TestReadChecksDescriptor` は、ステップ 1-2 のフックで、`Lstat` の後に通常のファイルをモード `0644` のファイル、ディレクトリ、FIFO に差し替えて、拒否になることを確かめる（設計 7.3）。別の所有者のファイルは root でないと作れないので、所有者の判定は `TestOwnerAndModeCheck` で関数を直接呼んで確かめる。
- [ ] 実効ユーザーが root のときに結果が変わるテスト（モード `0000`、探索できない親）は、`os.Geteuid() == 0` のとき `t.Skip` する（設計 7.1）。
- [ ] ファイルの用意でモードを決めるときは、umask に頼らず `os.Chmod` で明示する。どのテストにも `t.Parallel` を付けない。

**完了条件:** 上の共通の完了条件に加えて、`make deadcode` が何も報告しない。

### フェーズ 2: 書式の解析・雛形・`InitServerFile`（設計 8 章の 2）

#### ステップ 2-1: 設定ファイルのパスの規則

**変えるファイル:** `internal/config/configpath.go`（新規）、`internal/config/socketpath.go`、`internal/config/configpath_test.go`（新規）

- [ ] OS の分類を 1 つの非公開の関数にし、`socketRule` と設定ファイルの規則の両方がそれを使うように `socketpath.go` を変える（設計 3.2.1）。
- [ ] `serverFileLocation` と `serverFileLocationFor` を設計 3.2.1 のとおりに置く。理由の定数 `reasonConfigPathAbsent = "no absolute configuration file path could be derived from HOME"` を足す。
- [ ] `TestServerFileLocation` を書く: `goos` の文字列（`darwin`・`linux`・`freebsd`・規則のない 4 つ）と、`HOME` が未設定・空・相対の場合の表で、パス・基点・作ってよいディレクトリを確かめる。`XDG_CONFIG_HOME` を設定してもパスが変わらないこと、`HOME` のほかに変数を読まないこと（読んだ名前を記録する `LookupFunc` で確かめる）、エラーが `HOME` の値を含まないことも確かめる。今の `TestSocketPath`（`socketpath_test.go:45`）は、分類の共有の後も変えずに通ることを確かめる。

#### ステップ 2-2: 書式の解析とエラーの型

**変えるファイル:** `internal/config/serverfile.go`（新規）、`internal/config/serverfile_test.go`（新規）

- [ ] 設計 3.2.2 の `serverFileKeys`・`serverFileMaxSize`・`serverFileMaxReported`・`serverFile`・`lookup`・`parseServerFile` を、設計 3.2.2 と 6.2 の規則で実装する。行は `bytes.Split` で分ける。
- [ ] 設計 4.1 の番兵（`ErrConfigFile*`）・`FileError`・`FormatOverflowError` を置く。`Unwrap` は番兵 1 つだけを返す。`Error()` の文の形は設計 4.2 の表に従い、英語の文は実装で決める。
- [ ] `TestParseServerFileAccepts` を書く: 受理する形（空行、コメントの行、空白を伴わない `#` と `=` を含む値、空の値、LF で終わらない最後の行、64 KiB ちょうど）の表で、`lookup` が返す値と `ok` を確かめる。
- [ ] `TestParseServerFileRejects` を書く: 要件 F-003「拒否するもの」のそれぞれと、UTF-8 として正しくないファイルの表で、`errors.Is(err, ErrConfigFileFormat)`、行番号（ファイル全体の拒否では 0）、理由、重複のときのキーの名前を確かめる。どの行の拒否の文も、ファイルの値・行・未知のキーの文字列を含まないことを、表のすべての行について確かめる。
- [ ] `TestParseServerFileOverflow` を書く: 拒否が 21 行以上のとき、`*FileError` が行番号の小さい順にちょうど 20 個で、`*FormatOverflowError` が残りの数を持つことを確かめる。
- [ ] `TestParseServerFileDuplicate` を書く: 最初の出現がほかの理由で拒否されていても、2 回目の出現が「重複」になることを確かめる（設計 3.2.2「重複」）。

#### ステップ 2-3: 雛形と `InitServerFile`

**変えるファイル:** `internal/config/serverfile.go`、`internal/config/serverfile_test.go`

- [ ] `serverFileTemplate`・`InitServerFile`・`InitResult` を設計 3.2.5 のとおりに置く。雛形の英語の文面は実装で決める。
- [ ] `privatefile` の番兵から `config` の番兵への対応を、設計 4.1 の箇条のとおりに `errors.Is` の `switch` で書く。`default` は `Read` では「読めない」、`Create`・`MkdirBelow` では「作れなかった」にする。`privatefile` の型は `FileError` の外に出さない。
- [ ] `TestServerFileTemplate` を書く: 雛形を `parseServerFile` に通すと拒否がなく、値を 1 つも持たないこと、すべての行が空行かコメントの行であること、`serverFileKeys` のそれぞれが `# <キー>=` の形のコメントの行にあることを確かめる。
- [ ] `TestInitServerFile` を書く: `t.TempDir` の `HOME` で、作ったファイルのパス・中身（雛形と一致）・モード `0600`、作ったディレクトリのモード `0700` を確かめる。基点がない、すでにある（通常のファイル・リンク先のないシンボリックリンク・ディレクトリ）、作れない、の各場合の番兵と `Path` を確かめる。読んだ変数が `HOME` だけであることを、6 つのキーに値を入れた記録する `LookupFunc` で確かめる。`HOME` の誤りでは `*VarError` を返し、何も作らないことを確かめる。
- [ ] `TestFileErrorMapping` を書く: `privatefile` の各番兵と `UnsafeReason` が、設計 4.1 の表のとおりの `config` の番兵に対応し、`FileError` の文がパスと設計 4.2 の分類ごとの理由（シンボリックリンク・通常のファイルでない・所有者が違う・モードと `chmod 600`・読めない）を含むことを確かめる。

**完了条件:** 共通の完了条件と `make deadcode`。

### PR の区切り（フェーズ 1 と 2）

フェーズ 1 の後と、フェーズ 2 の後に PR を作る（3.2）。

### フェーズ 3: `LoadServer` の組み立ての変更と `VarError.Source`（設計 8 章の 3）

#### ステップ 3-1: サーバの入口で廃止した変数を拒否するテストの行を削除する

**変えるファイル:** `internal/config/config_test.go`、`cmd/yt2column-server/run_test.go`

- [ ] 削除だけのコミットで、`TestLoadRejectsRetiredLLMVars` の `LoadServer` の行（`config_test.go:1850-1856`）、`TestStartupFailures` の廃止した変数の 2 行（`run_test.go:88-106`）、`lastRetiredLLMVar`（`run_test.go:337-342`）を消す。`make coverage-compare BASE=<親のコミット>` の出力を、最後の行まで、コミットメッセージに書く（1.2）。

#### ステップ 3-2: `VarError.Source` と番兵の文

**変えるファイル:** `internal/config/config.go`、`internal/config/config_test.go`

- [ ] 設計 3.2.4 の `VarSource`・`VarSourceUnspecified`・`VarSourceConfigFile` と `VarError.Source` を足し、`Error()` を `Source` の `switch` にする（`default` は今の形）。理由の定数 `reasonNotSet = "is not set"`・`reasonEmpty = "is empty"` を足す。
- [ ] `ErrMissing` の文を `"environment variable is unset or empty"` から `"a setting is unset or empty"` に、`ErrInvalid` の文を `"environment variable has an invalid value"` から `"a setting has an invalid value"` に変える。
- [ ] `config.go:31` のコメントを `// Names of the environment variables Load reads.` から `// Names of the environment variables Load reads, which are also the keys of the server's configuration file.` に変える。
- [ ] `TestVarErrorSource` を書く: ゼロ値の `Source` が今の `<Name>: <Reason>` の形で、`VarSourceConfigFile` が設定ファイルのキーとして示す形であることを確かめる。

#### ステップ 3-3: 設定ファイルを書くテスト用のアクセサ

**変えるファイル:** `internal/config/test_helpers.go`

- [ ] 1.3 の 2 つ目の決定のアクセサを足す。与えた `HOME` の下の、`serverFileLocationFor` が返すパスに、キーと値を 1 行ずつモード `0600` で書く（親は `0700` で作る。`privatefile.MkdirBelow` は umask を変えるので使わず、`os.MkdirAll` と `os.Chmod` で作る）。値を書かない行（値のない `KEY=`）も書けるようにする。
- [ ] 同じファイルに、設定ファイルのパスの規則（OS ごとの `HOME` より下の部分）、`serverFileKeys`、`serverFileMaxSize`、`ErrConfigFile*` の番兵の対応を確かめるための、ガードテスト用のアクセサを足す（フェーズ 7 で使う。今のアクセサと同じく値をコードから取る）。
- [ ] 雛形をバイト列で返すアクセサと、バイト列を書式の解析に通して受理したキーと値を返すアクセサを足す（ステップ 5-3、ステップ 7-3）。

#### ステップ 3-4: `LoadServer` の変更

**変えるファイル:** `internal/config/config.go`

- [ ] `LoadServer` を設計 3.2.3 の 3 つの段に組み替える。段 3 の `loadLLM`・`loadSlackWebhook` には `serverFile` の `lookup` を、`socketPath`・`hasHTTP2Debug` には引数の `lookup` を渡す。
- [ ] 段 3 の `*VarError` のうちファイルから読んだキーのものに `Source = VarSourceConfigFile` を付け、`ErrMissing` のときは `serverFile` がキーを持つかで `Reason` を `reasonNotSet` か `reasonEmpty` にする（設計 3.2.3・3.2.4）。複数の拒否は今までどおり 1 段の `errors.Join` にする。
- [ ] `LoadServer` の注釈を設計 3.2.3 の注釈に置き換える。`ServerConfig` の注釈（`config.go:317-320`）に、6 つのキーを設定ファイルから読むことを足す。

#### ステップ 3-5: `internal/config` のテストを設定ファイルに合わせる

**変えるファイル:** `internal/config/config_test.go`、`internal/config/socketpath_test.go`、`internal/config/test_helpers.go`

- [ ] 1.4「`LoadServer` を呼ぶ場所」の `internal/config` のテストを、ステップ 3-3 のアクセサでファイルを書く形に書き換える。確かめる内容は変えない。
- [ ] `TestLoadServerReadsFile`（AC-04: 要件 AC-04 の 2 つの例）、`TestLoadServerValueRejections`（AC-05: 拒否ごとに 1 つの要素、`Source` が設定ファイル、行が「environment variable」を含まず値を含まない）、`TestLoadServerIgnoresEnvSettings`（AC-06・AC-07: 環境に 6 つのキーと廃止した 5 つの変数を空の値を含めて入れても、結果がファイルだけで決まること）、`TestLoadServerEmptyValues`（AC-11a: 使うキーの空の値は「is empty」、使わないプロバイダのキーの空の値は受理）、`TestLoadServerFileRejections`（ファイルがない・読んではならない・読めない・書式の誤りが、それぞれの番兵の `*FileError` になり、ゼロ値の `ServerConfig` を返すこと）、`TestLoadServerReadsOnlyHomeAndGodebug`（環境から読む名前が `HOME` と `GODEBUG` だけであること）を書く。
- [ ] `TestLoadHTTP2Debug`（`config_test.go:1011`）に、`LoadServer` が環境の `GODEBUG` を読むことの行を足す（AC-09）。
- [ ] `TestLoadForIgnoresServerFile` を書く: `HOME` の下に 6 つのキーの値を持つ設定ファイルがあっても、`Load`・`LoadFor` の結果が環境変数だけで決まることを確かめる（AC-10 のファイルの部分）。

#### ステップ 3-6: サーバのテストの補助と、`config` の外のテストを書き換える

**変えるファイル:** `cmd/yt2column-server/test_helpers.go`、`cmd/yt2column-server/run_test.go`・`relay_e2e_test.go`・`signal_test.go`・`server_test.go`、`internal/llm/provider/provider_test.go`、`cmd/yt2column/docs_test.go`

- [ ] `serverEnv` を、環境変数（`HOME`・`GODEBUG` など）と設定ファイルのキーの 2 つの表を持つ形にする。`run` を呼ぶ前・`socketPath` を求める前・子プロセスを起動する前に、ステップ 3-3 のアクセサでその時点の `HOME` の下にファイルを書く。`TestStartupFailures` の `HOME` を延ばす行のように、`setup` が `HOME` を変えた後でもファイルがその `HOME` の下にあるようにするためである。
- [ ] 1.4「サーバのテストが環境変数で設定を渡す場所」の `e.vars[...]` への書き込みのうち 6 つのキーのものを、ファイルの表への書き込みに変える。`childEnv` は、6 つのキーを子の環境に渡さない。
- [ ] `provider_test.go:790-810` と `cmd/yt2column/docs_test.go:493-499`・`530-536`・`619-622` を、短い `HOME` の下にアクセサでファイルを書いてから `LoadServer` を呼ぶ形にする。`docsProfileEnv` の固定の `HOME`（`/tmp/yt2column-docs-test`）は、テストごとの `udstest.Dir` に変える。
- [ ] `TestStartupFailures` に、設定ファイルがない、モードが `0644`、書式の誤り（1 行）、値の拒否の行を足し、それぞれ終了コード `2`、設定エラーの行、ソケットのディレクトリ・起動のロック・ソケットがないこと、設定ファイルとその親のモードと中身が変わらないことを確かめる（AC-05・AC-12・AC-14・AC-18・AC-20）。HOME の誤りの行（`:110` 付近）とソケットのパスが長すぎる行は、今のまま通ることを確かめる（AC-03）。
- [ ] `TestFormatRejectionLines` を書く: 21 行以上の書式の拒否で、標準エラーの設定エラーの行がちょうど 21 行になることを確かめる。
- [ ] `TestModeRejectionChmodCommand` を書く: パスに空白を含む `HOME` で、モードの拒否の行から `chmod` のコマンドを取り出して `sh -c` で実行し、ファイルのモードが `0600` になることを確かめる（設計 4.2「直し方のコマンドの引用」）。
- [ ] `TestServerIgnoresEnvSettings` を書く: 環境に 6 つのキーと廃止した 5 つの変数（空の値を含む）を入れ、ファイルが正しければサーバが待ち受け、標準エラーにそれらの変数について何も書かないことを確かめる（AC-07）。環境の `ANTHROPIC_API_KEY` だけではファイルの欠落の設定エラーで止まることも確かめる（AC-06）。

**完了条件:** 共通の完了条件。`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./internal/llm/provider` が通る。

### フェーズ 4: 廃止した変数の検査の削除（設計 8 章の 4）

#### ステップ 4-1: 検査のコードとテストを削除する

**変えるファイル:** `internal/config/config.go`・`config_test.go`・`socketpath_test.go`・`test_helpers.go`、`cmd/yt2column/run_test.go`・`makefile_test.go`、`internal/llm/deepseek/testutil/make.go`、`cmd/yt2column-nmhost/main_test.go`

- [ ] 削除だけのコミットで、次を消す。本番: `retiredLLMVar`・`retiredLLMVars`、`reasonRetiredLLM`・`reasonRetiredReviewLLM`・`reasonRetiredClaudeEffort`、`retiredLLMErrors` と `loadLLM` の中の呼び出し（`config.go:482` は `var errs []error` にする）。テスト用の補助: `RetiredLLMVarsForTest`・`RetiredLLMVarErrorForTest`（`test_helpers.go:87-107`）、`withRetiredLLMVars`（`socketpath_test.go:150-158`、呼び出しは引数の表をそのまま渡す形にする）、`CheckRetiredModelEnvNotRead`（`make.go:88-108`）、`retiredLLMEnvs`・`retiredLLMEnvMentioned`・`retiredLLMEnvRE`（`makefile_test.go:46-52`・`116-130`）。テスト: `TestRetiredClaudeEffortReason`、`retiredLLMVarsWant`・`TestRetiredLLMVars`、`TestLoadRejectsRetiredLLMVars`、`TestRunRejectsRetiredLLMVars`、`TestMakeTargetsDoNotReadRetiredModelEnv`、`TestRetiredLLMEnvMentioned`、`TestRetiredModelEnvIsGone`。テストの行: `TestLoadErrorsOmitValues` の廃止した変数の行（`config_test.go:631-636`）と `markRetired`、`TestLoadReportsAllInvalid` の廃止した変数の 3 行（`:659-720` の該当行）、`TestLoadFor` の廃止した変数の確かめ（`:1187-1191`）、`cmd/yt2column/run_test.go:464-471` の行、`hostEnvs` の廃止した変数の組（`main_test.go:142-145`・`151`）。
- [ ] `make coverage-compare BASE=<親のコミット>` を、10 分以上の時間の上限で実行し、出力を最後の行まで、コミットメッセージと PR の説明に書く（1.2）。

#### ステップ 4-2: 注釈を直す

**変えるファイル:** `internal/config/config.go`、`internal/config/test_helpers.go`、`cmd/yt2column/run_test.go`、`cmd/yt2column-nmhost/main_test.go`、`internal/config/config_test.go`、`internal/config/socketpath_test.go`

- [ ] `defaultProvider` の注釈を `// defaultProvider is the provider part of the default selection. The\n// retired variables' rejection names it too, so it is written once.` から `// defaultProvider is the provider part of the default selection.` に変える。
- [ ] `LoadFor` の注釈から `The one exception to reading only what the run needs: whatever need is, LoadFor checks whether each retired LLM variable is present (never its value) and rejects the run if one is, so a leftover setting is noticed even by a run that calls no LLM, YT2COLUMN_CLAUDE_EFFORT included when no stage uses claude.` を消す。
- [ ] `loadLLM` の注釈の `Contract: the retired variables are checked first, whatever need is, and their values are never read; every other variable is read only for a stage the run needs.` を `Contract: every variable is read only for a stage the run needs.` に変える。
- [ ] パッケージの注釈（`config.go:1-3`）を `// Package config loads and validates the process configuration from\n// environment variables. It is the only package that names the secret\n// variables; every other package receives already-validated values.` から `// Package config loads and validates the process configuration: the CLI's\n// from environment variables, and the server's from its configuration file.\n// It is the only package that names the secret variables; every other package\n// receives already-validated values.` に変える。
- [ ] `test_helpers.go:50-52` の見出しの注釈から「and the retired-variable rejection」を消し、`run_test.go:1828-1831`・`main_test.go:135-137`・`253`・`319`・`config_test.go:1160`・`1445-1448`・`socketpath_test.go:161-163`・`206` の注釈の、廃止した変数に触れる部分を消す。`rg -n "retired|Retired" cmd internal` が、`internal/maketestutil/make_test.go` のほかに何も出さないことを確かめる。

#### ステップ 4-3: CLI が廃止した変数を無視することのテスト

**変えるファイル:** `internal/config/config_test.go`

- [ ] `TestLoadForIgnoresRetiredVars` を書く: 廃止した 5 つの変数を、空の値と DeepSeek を選ぶ値で環境に入れても、`Load` と `LoadFor`（`Need{}` を含む）が拒否せず、結果が廃止した変数のない環境と同じであることを確かめる（AC-10）。

### フェーズ 5: 引数の振り分けと `runInit`（設計 8 章の 5）

#### ステップ 5-1: 引数の振り分け

**変えるファイル:** `cmd/yt2column-server/run.go`、`cmd/yt2column-server/run_test.go`

- [ ] `run` の最初で、設計 3.3 のとおりに引数を振り分ける。引数の拒否の行 `"%s: takes no arguments; the configuration is read from environment variables (see README.en.md)"` を、受け付ける形（引数なし、または `config init`）を示す行に置き換える。英語の文は実装で決め、`README.en.md` への案内は残す。
- [ ] `run` の注釈の `Steps 1 and 2 touch no network and no file; a failure there stops the start with exitUsage.` を `Steps 1 and 2 touch no network; step 1 reads the configuration file and writes nothing. A failure there stops the start with exitUsage.` に変える。`configDocument` の注釈（`run_test.go:30-31`）を新しい行に合わせる。
- [ ] `TestRunArgs` を書く: 引数なし（サーバとして待ち受ける）、`config init`（雛形を作る）、要件 AC-29 の例（`config`、`init`、`config init extra`、`--help`、`config Init`）の表で、終了コードと、拒否のときに何も作らないこと（`HOME` の下に何もないこと）を確かめる。今の `TestStartupFailures` の「an argument」の行は、`TestRunArgs` に移すか、新しい行の文に合わせる。

#### ステップ 5-2: `runInit`

**変えるファイル:** `cmd/yt2column-server/run.go`

- [ ] `runInit` を設計 3.3 と 6.3 のとおりに実装する。`*VarError` は `exitUsage`、それ以外の失敗は `exitFailure`、成功は `exitOK` を返す。成功のときは作ったパスの 1 行、一時ファイルが残ったときはその名前の 1 行を、`stderrWriter` を通して標準エラーに書く。標準出力には何も書かない。

#### ステップ 5-3: `config init` のテスト

**変えるファイル:** `cmd/yt2column-server/run_test.go`、`cmd/yt2column-server/test_helpers.go`

- [ ] `TestConfigInit` を書く: 何もない `HOME` で終了コード `0`、作ったファイルが雛形と一致しモード `0600`、標準エラーにパスの 1 行、標準出力が空、ソケットのディレクトリ・起動のロック・ソケットがないことを確かめる。すでにある（通常のファイル・リンク先のないシンボリックリンク・ディレクトリ）ときは終了コード `1` で変わらないこと、基点（macOS では `$HOME/Library/Application Support`、ほかでは `$HOME`）がないときは終了コード `1` で何も作らないこと、作れないとき（親に書けない）は終了コード `1` で書きかけがないこと、`HOME` の誤りでは終了コード `2` で何も作らないこと、ソケットのパスの上限を超える長い `HOME` でも作れることを確かめる。親に書けない場合は root では結果が変わるので、`os.Geteuid() == 0` のとき `t.Skip` する。
- [ ] `TestConfigInitThenStart` を書く: `config init` で作ったファイルのままサーバを起動すると、書式と検査を通って `ANTHROPIC_API_KEY` がないことを示す設定エラーで止まり（AC-26）、そのファイルにキーを書き足すとサーバが待ち受けることを確かめる（設計 7.2）。
- [ ] `config init` の作ったファイルに書き足すための補助が要るなら、`test_helpers.go` に置く。このステップのテストに `t.Parallel` を付けない（1.3）。

### PR の区切り（フェーズ 3 から 5）

フェーズ 5 の後に PR を作る（3.2）。

### フェーズ 6: 拡張のビューアの案内（設計 8 章の 6）

#### ステップ 6-1: カタログの文

**変えるファイル:** `extension/src/core/i18n/en.ts`、`extension/src/core/i18n/ja.ts`、`extension/test/secretsAbsent.test.ts`

- [ ] `en.ts:27` の `setWebhookStep` を `"Set SLACK_WEBHOOK_URL and restart yt2column-server."` から `"Write SLACK_WEBHOOK_URL in the yt2column-server configuration file and restart yt2column-server."` に変える。`ja.ts:23-24` の `setWebhookStep` も、設定ファイルに `SLACK_WEBHOOK_URL` を書いて `yt2column-server` を起動し直す、という同じ意味の文に変える（設計 3.4）。
- [ ] `secretsAbsent.test.ts:28` の注釈を `// The server's environment variable a message may tell the user to set.` から `// The server's configuration key a message may tell the user to set.` に変える。

**完了条件:** `make ext-check` が通る。

### フェーズ 7: 文書とガードテスト（設計 8 章の 7）

#### ステップ 7-1: README

**変えるファイル:** `README.md`、`README.en.md`

- [ ] `README.md` を、要件 AC-30・AC-31 の内容に書き換える。設定ファイルの節（パス・書式・検査・ACL・`config init` と終了コードの表・一時ファイルの接頭辞 `.config.env.tmp-`）、移行の手順を足す。クイックスタート・サーバの起動・設定・推奨構成のサーバの手順を、設定ファイルの形にする。推奨構成のブロックは設定ファイルの行の形にする（1.3）。「設定」の節の廃止した変数の段落（`README.md:431-433` 付近）、「廃止した LLM の変数からの移行」の節（`:467-525` 付近）、それを指すトラブルシューティングの項目（`:352` 付近）を消す。
- [ ] `/mktrans README.md` で `README.en.md` に同期し、2 つを 1 つのコミットにする。

#### ステップ 7-2: ほかの文書

**変えるファイル:** `docs/dev/project_overview.md`、`docs/dev/security.md`、`docs/dev/production_profile.md`、`docs/dev/guides/development.md`、`docs/dev/guides/cli.md`、`docs/dev/conventions/package_reference.md`、`CLAUDE.md`、`docs/translation_glossary.md`

- [ ] 要件 AC-33 に挙げた節を書き換える。`project_overview.md` は「設定（環境変数）」の節の廃止した変数の段落（`:163`）を消す。`production_profile.md` の「本番の構成」のブロックは設定ファイルの行の形にする（1.3）。`development.md` の「開発用に別のサーバを動かす」は、専用の `HOME` の下に設定ファイルを置いてから起動する手順にする。`cli.md` の廃止した変数の記述（`:70-71`・`:255`）を消す。`package_reference.md` の `cmd/yt2column-server` と `internal/config` の行を直す。`CLAUDE.md` の Architecture Overview のサーバを支えるパッケージに `internal/privatefile` を足し、設定と秘密の記述を直す。`translation_glossary.md` の「廃止した LLM の変数」（`:159`）と「廃止した LLM の変数からの移行」（`:186`）の行を消し、「設定エラー」の定義を環境変数に限らない形にし、「設定ファイル」「雛形」を足す。

#### ステップ 7-3: 文書のガードテスト

**変えるファイル:** `cmd/yt2column-server/docs_test.go`、`cmd/yt2column/docs_test.go`、`internal/doctestutil/readme.go`

- [ ] `TestServerDocsContract` に、設計 3.5 の表の値の照合を足す: 両 README の設定ファイルのパス（OS ごとの行）、`config init` の終了コードの表（`exitOK`・`exitFailure`・`exitUsage` の 3 つの行）、6 つのキーの名前、大きさの上限（`serverFileMaxSize` から作る文字列）、`security.md` §8.2 の設定ファイルのパス。値はステップ 3-3 のアクセサから取る。新しい見出しや表の列の場所は `internal/doctestutil/readme.go` に足す。
- [ ] `TestProductionProfileDocsContract` の `checkExportBlock`・`checkExportLines` を、設定ファイルの行のブロックを読む形に変える。ブロックをステップ 3-3 の解析のアクセサに通して受理されることを確かめ、そのキーと値を短い `HOME` の下にファイルとして書いて `LoadServer` で読み込み、既定の構成と比べる（1.3）。`exportLineRE` と、`export` の形を前提にした注釈とテストの失敗の文を、新しい形に合わせる。
- [ ] 文書を書き換えた後に、`TestLLMDefaultsDocsContract`・`TestEffortValuesDocsContract`・`TestReadmeLinks`・`TestReadmeSyncMarker` と、`make ext-test`（README と `development.md` を読む拡張のテスト）が通ることを確かめる。

**完了条件:** 共通の完了条件、`make readme-sync-check`、`make ext-test`。

### PR の区切り（フェーズ 6 と 7）

フェーズ 7 の後に PR を作る（3.2）。

## 3. 実装の順序とマイルストーン

### 3.1. マイルストーン

| マイルストーン | フェーズ | 成果物 |
|---|---|---|
| M1 | 1 | 検査して読み、安全に作る `internal/privatefile` |
| M2 | 2 | 設定ファイルのパスの規則・書式の解析・雛形・`InitServerFile`（まだどこからも呼ばれない） |
| M3 | 3〜5 | サーバが設定ファイルだけを読み、廃止した変数の検査がなくなり、`config init` で雛形を作れる |
| M4 | 6〜7 | 拡張の案内と文書が設定ファイルに合う |

## 4. テスト戦略

### 4.1. 単体テスト

-   `internal/privatefile` は、`t.TempDir` の下の実際のファイルで試す（ステップ 1-4）。タイミングに頼るテストは書かず、検査と読み込みが同じ記述子を使うことは `Lstat` の後のフックで確かめる（設計 7.3）。
-   `internal/config` は、書式の解析を表で試し、`LoadServer` と `InitServerFile` は `t.TempDir` や `udstest.Dir` の `HOME` に実際のファイルを置いて試す（ステップ 2-1 から 2-3、3-5、4-3）。
-   拒否の文が値・行・未知のキーを含まないことは、表のすべての行について確かめる（ステップ 2-2、3-5）。

### 4.2. 結合テスト

-   `cmd/yt2column-server` の `run` を通すテスト（`TestStartupFailures`・`TestRunArgs`・`TestConfigInit`・`TestConfigInitThenStart`・`TestServerIgnoresEnvSettings`）と、ホストとサーバの結合のテスト（`relay_e2e_test.go`）は、テスト用の `HOME` の下に設定ファイルを置いて動かす（ステップ 3-6、5-3）。
-   LLM の API と Webhook は呼ばない。今のテストと同じく、偽の部品（`testDeps`）を使う。

### 4.3. 互換性のテスト

-   CLI の設定が変わらないこと（AC-10）は、`Load`・`LoadFor` の今のテストがそのまま通ることと、ステップ 3-5 の `TestLoadForIgnoresServerFile`、ステップ 4-3 の `TestLoadForIgnoresRetiredVars` で確かめる。
-   ホストは変わらないことは、`cmd/yt2column-nmhost` の今のテストがそのまま通ることで確かめる。
-   環境変数からの移行は互換の仕組みを持たない（要件 4.4）ので、移行のテストは置かない。

### 4.4. 権限と実行環境

-   root では結果が変わるテストは、`os.Geteuid() == 0` のときに `t.Skip` する（ステップ 1-4、5-3）。CI は root でない利用者で動く。
-   umask を変えるテストと、`MkdirBelow` を呼ぶテストには `t.Parallel` を付けない（1.3）。

## 5. リスク管理

| リスク | 影響 | 対策 |
|---|---|---|
| umask を変えている間に、同じプロセスのほかの処理がファイルを作る | そのファイルのモードが意図より狭くなる | `MkdirBelow` を呼ぶのは `config init` だけで、並行する処理がない。注釈で並行して呼ばないことを示し、テストに `t.Parallel` を付けない（1.3、ステップ 1-1） |
| `LoadServer` の変更で、環境変数で設定を渡すテストが一度に止まる | フェーズ 3 のコミットが通らない | テストの補助とすべての呼び出し元の書き換えをフェーズ 3 に置く（1.3、ステップ 3-6）。呼び出し元は 1.4 に挙げた |
| 削除のコミットのカバレッジの比較が `STOP` か `FAIL` になる | テストの削除が確かめられない | 削除だけのコミットにし、本番のコードとそれだけを試すテストを同じコミットで消す（ステップ 3-1、4-1）。`FAIL` のときは削除の範囲を見直し、比較の出力をそのまま記録する |
| macOS と Linux で、OS が返すエラーやモードの振る舞いが違う | 一方の OS でだけテストが通らない | テストは番兵と `errors.Is` で確かめ、OS のエラーの文を照合しない（`CLAUDE.md`「Error Testing」）。CI は Linux で、macOS は開発の環境で `make test` を通す |
| PR-3 が大きい | レビューに時間がかかる | 削除のコミット・書き換えのコミット・新しいテストのコミットを分け、PR の説明でコミットごとの見どころを示す |

## 6. 実装チェックリスト

- [ ] フェーズ 1: `internal/privatefile`（ステップ 1-1 から 1-4）
- [ ] フェーズ 2: 書式の解析・雛形・`InitServerFile`（ステップ 2-1 から 2-3）
- [ ] フェーズ 3: `LoadServer` の変更とテストの書き換え（ステップ 3-1 から 3-6）
- [ ] フェーズ 4: 廃止した変数の検査の削除（ステップ 4-1 から 4-3）
- [ ] フェーズ 5: 引数の振り分けと `runInit`（ステップ 5-1 から 5-3）
- [ ] フェーズ 6: 拡張の案内（ステップ 6-1）
- [ ] フェーズ 7: 文書とガードテスト（ステップ 7-1 から 7-3）
- [ ] すべての受け入れ基準の検証（7 章）が通った

## 7. 受け入れ基準の検証

種別は `test`（実行して誤った振る舞いで失敗する）、`static`（ガードテスト・`make` のターゲット）、`manual`（PR のレビューでの読み取り）である。本タスクに `diff` の検証はない（前後の関係を述べる AC がない）。

| AC | 検証 | 種別 | 実装 |
|---|---|---|---|
| AC-01・AC-02 | `internal/config/configpath_test.go::TestServerFileLocation` | test | ステップ 2-1 |
| AC-03 | `internal/config/configpath_test.go::TestServerFileLocation`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（`HOME` の誤りとソケットのパスの上限の行）、`cmd/yt2column-server/run_test.go::TestConfigInit`（`HOME` の誤りと長い `HOME`） | test | ステップ 2-1、3-6、5-3 |
| AC-04 | `internal/config/config_test.go::TestLoadServerReadsFile` | test | ステップ 3-4 |
| AC-05 | `internal/config/config_test.go::TestLoadServerValueRejections`、`internal/config/config_test.go::TestVarErrorSource`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（値の拒否の行） | test | ステップ 3-2、3-4 |
| AC-06 | `internal/config/config_test.go::TestLoadServerIgnoresEnvSettings`、`cmd/yt2column-server/run_test.go::TestServerIgnoresEnvSettings` | test | ステップ 3-4 |
| AC-07 | `internal/config/config_test.go::TestLoadServerIgnoresEnvSettings`、`cmd/yt2column-server/run_test.go::TestServerIgnoresEnvSettings` | test | ステップ 3-4 |
| AC-09 | `internal/config/config_test.go::TestLoadHTTP2Debug`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（`GODEBUG` の行） | test | ステップ 3-4 |
| AC-10 | `internal/config/config_test.go::TestLoadForIgnoresServerFile`、`internal/config/config_test.go::TestLoadForIgnoresRetiredVars`、`Load`・`LoadFor` の今のテスト（`internal/config/config_test.go::TestLoadValid`・`TestLoadFor` ほか） | test | ステップ 3-5、4-1、4-3 |
| AC-11 | `internal/config/serverfile_test.go::TestParseServerFileAccepts` | test | ステップ 2-2 |
| AC-11a | `internal/config/serverfile_test.go::TestParseServerFileAccepts`（空の値の受理）、`internal/config/config_test.go::TestLoadServerEmptyValues` | test | ステップ 2-2、3-4 |
| AC-12 | `internal/config/serverfile_test.go::TestParseServerFileRejects`、`internal/config/config_test.go::TestLoadServerFileRejections`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（書式の誤りの行、ファイルが変わらないこと） | test | ステップ 2-2、3-4、3-6 |
| AC-13 | `internal/config/serverfile_test.go::TestParseServerFileRejects`・`TestParseServerFileOverflow`・`TestParseServerFileDuplicate`、`cmd/yt2column-server/run_test.go::TestFormatRejectionLines` | test | ステップ 2-2、3-6 |
| AC-14 | `internal/privatefile/privatefile_test.go::TestRead`、`internal/config/config_test.go::TestLoadServerFileRejections`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（ファイルがない行：パスと `yt2column-server config init` を示す） | test | ステップ 1-2、3-4、3-6 |
| AC-15・AC-16 | `internal/privatefile/privatefile_test.go::TestRead`（シンボリックリンク 2 種、ディレクトリ、FIFO、ソケット）、`internal/config/config_test.go::TestLoadServerFileRejections`、`internal/config/serverfile_test.go::TestFileErrorMapping`（示す文） | test | ステップ 1-2、2-3 |
| AC-17 | `internal/privatefile/privatefile_test.go::TestOwnerAndModeCheck`、`internal/config/serverfile_test.go::TestFileErrorMapping`（所有者が違うことを示す文） | test | ステップ 1-1、2-3 |
| AC-18 | `internal/privatefile/privatefile_test.go::TestRead`（各モード）、`cmd/yt2column-server/run_test.go::TestStartupFailures`（モード `0644` の行）、`cmd/yt2column-server/run_test.go::TestModeRejectionChmodCommand` | test | ステップ 1-2、3-6 |
| AC-19 | `internal/privatefile/privatefile_test.go::TestReadUnreadable`、`internal/config/config_test.go::TestLoadServerFileRejections` | test | ステップ 1-2 |
| AC-19a | `internal/privatefile/privatefile_test.go::TestReadChecksDescriptor` | test | ステップ 1-2 |
| AC-20 | `cmd/yt2column-server/run_test.go::TestStartupFailures`（設定エラーの各行で、ソケットのディレクトリ・起動のロック・ソケットがなく、設定ファイルと親が変わらないこと） | test | ステップ 3-6 |
| AC-21 | `internal/privatefile/privatefile_test.go::TestCreate`、`internal/config/serverfile_test.go::TestInitServerFile`、`cmd/yt2column-server/run_test.go::TestConfigInit` | test | ステップ 1-3、2-3、5-2 |
| AC-22 | `internal/config/serverfile_test.go::TestServerFileTemplate`、`internal/config/serverfile_test.go::TestInitServerFile`（`HOME` だけを読む） | test | ステップ 2-3 |
| AC-23 | `internal/privatefile/privatefile_test.go::TestMkdirBelow`、`cmd/yt2column-server/run_test.go::TestConfigInit`（基点がない場合） | test | ステップ 1-3、5-2 |
| AC-24 | `internal/privatefile/privatefile_test.go::TestCreateExisting`、`cmd/yt2column-server/run_test.go::TestConfigInit`（すでにある場合） | test | ステップ 1-3、5-2 |
| AC-25 | `internal/privatefile/privatefile_test.go::TestCreateFailureLeavesNothing`、`cmd/yt2column-server/run_test.go::TestConfigInit`（作れない場合） | test | ステップ 1-3、5-2 |
| AC-26 | `internal/config/serverfile_test.go::TestServerFileTemplate`、`cmd/yt2column-server/run_test.go::TestConfigInitThenStart` | test | ステップ 2-3、5-3 |
| AC-27・AC-28・AC-29 | `cmd/yt2column-server/run_test.go::TestRunArgs` | test | ステップ 5-1 |
| AC-30・AC-31 | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（パス・終了コード・キー・上限）、`cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`（推奨構成のブロック）、`cmd/yt2column/docs_test.go::TestReadmeLinks`（消した節へのリンクが残らない）、文の内容は PR のレビューで読む | static・manual | ステップ 7-1、7-3 |
| AC-32 | `cmd/yt2column/readme_sync_test.go::TestReadmeSyncMarker`（`make readme-sync-check`）、`make ext-test` | static | ステップ 7-1 |
| AC-33 | `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`（`production_profile.md` のブロック）、`cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`security.md` §8.2 のパス）、`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`、文の内容は PR のレビューで読む | static・manual | ステップ 1-1、7-2、7-3 |
| AC-34 | `cmd/yt2column-server/docs_test.go::TestServerDocsContract` | static | ステップ 7-3 |
| AC-35 | `make ext-check`（`extension/test/secretsAbsent.test.ts` を含む）、案内の文の内容は PR のレビューで読む | static・manual | ステップ 6-1 |

要件 4.2「拒否の分類」の番兵ごとの `errors.Is` は、`TestFileErrorMapping`（ステップ 2-3）、`TestLoadServerFileRejections`（ステップ 3-5）、`TestConfigInit`（ステップ 5-3）で確かめる。

## 8. 成功基準

-   **機能:** 7 章のすべての検証が通る。
-   **品質:** `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。テストを削除したコミットは、`make coverage-compare` が `PASS` で、その出力がコミットメッセージと PR の説明にある。
-   **セキュリティ:** 設計 5.1 の脅威の表のそれぞれに、7 章の検証がある。拒否の文と雛形が値・秘密を含まないことを、テストで確かめている。
-   **文書:** 要件 AC-30 から AC-33 の文書が設定ファイルに合い、`README.en.md` が同期している。

## 9. 次のステップ

-   本書の承認の後、`/runplan 0066` で PR-1 から順に実装する。
-   最後の PR のマージの後、issue #576 を閉じる（`/runpr` が案内する）。
-   ブラウザ拡張の利用者として、実際の `HOME` で `config init` から起動までを 1 度確かめる（`brew services` での常駐は #578 で扱う）。
