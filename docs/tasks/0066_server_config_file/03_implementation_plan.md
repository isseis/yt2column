# 実装計画書：サーバが設定ファイルを読むようにする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-07 |
| Review date | 2026-11-08 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下「要件」）の F-001 から F-008 を、[02_architecture.md](02_architecture.md)（以下「設計」）のとおりに実装する手順を定める。設計の内容は本書に写さず、節の番号で参照する。

### 1.2. 実装の原則

-   **設計の優先順位（設計 8 章）の順にフェーズを置く。** フェーズ 1 から 7 は、設計 8 章の 1 から 7 に対応する。あるフェーズのコミットを通すために後の項目の作業の一部が必要なときは、そのフェーズの中に前提として書く（1.3 の 5 つ目と 6 つ目の決定）。
-   **各コミットで `make test`・`make lint`・`make deadcode` が通る。** 一緒に変えないと通らない変更は、各ステップの「コミット」の行に 1 つのコミットとして書く。
-   **テストの削除は、削除だけのコミットにし、`make coverage-compare BASE=<親のコミット>` で確かめる**（`CLAUDE.md`「Deleting a test is a claim that must be checked」、設計 3.6）。比較は 1 回で数分かかるので、10 分以上の時間の上限か、バックグラウンドで実行する。出力を最後の行までコミットメッセージと PR の説明に書く。コミットメッセージを修正するときは、ツリーの ID が変わらないよう、メッセージ以外に触れない。`coverage-compare` は、残る本番の関数の中の行が書き換わると `STOP unmatched` で止まる（`internal/coveragecompare/classify.go:23-48`・`105-131`）。そのため、本番のコードの振る舞いを変えるコミットと、テストを削除するコミットは分ける（ステップ 4-1・4-2）。既存のテストは、できるかぎり削除せずに書き換える。
-   **テストは、守る対象を外して失敗することを確かめてからコミットし、そのことをコミットメッセージに書く**（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。本書は、どのテストが何を検出するかを断定しない。
-   **Go のコメント・識別子・文字列リテラルは英語で書く。** 例外は、文書の日本語の見出しや表の列名と一致しなければならないガードテストのリテラル（`CLAUDE.md`「Go Idioms」の guard-test literal exception）だけである。
-   **ソースのコメントに AC や F の番号を書かない。** AC との対応は本書の 7 章に置く（`requirements_process.md` §4）。
-   **テストの `HOME`:** `LoadServer` まで届くテストは、パッケージによらず `udstest.Dir` の短いディレクトリ（またはその下）を `HOME` にする。macOS では `t.TempDir` のパス（80 バイト以上）にソケットのパスの残り（52 バイト）を足すと上限の 103 バイトを超え、設計 3.2.3 の `LoadServer` の 3 つの段のうち、3 つ目で止まるためである（`internal/config/socketpath.go:13`・`21`）。

### 1.3. 計画の際に決めたこと（2026-11-07）

-   **`MkdirBelow` は、`mkdir` の間だけ umask を `077` にし、作った各ディレクトリを開いて `Fchmod` でモードを `0700` にそろえる（[implementation_handoff.md](implementation_handoff.md) I-01）。** `syscall.Umask(0o077)` で umask を変えて `os.Mkdir(name, 0o700)` を呼び、元の umask に戻す。作ったディレクトリは所有者の読み取りと探索の権限を持つので、`O_RDONLY|O_DIRECTORY|O_NOFOLLOW` で開ける。開いた記述子に `fstat` してディレクトリで自分の所有であることを確かめ、`Fchmod(0o700)` で、Linux で親から継承した setgid のビットも落とす。理由: 設計 3.1 の条件（umask によらず `0700`、作ったものだけを変える、OS ごとのファイルを持たない）を満たす。採らなかった案: Linux の `O_PATH` の記述子は `fchmod` できない。`syscall.Fchmodat` の `AT_SYMLINK_NOFOLLOW` は、Linux では `fchmodat2` のある 6.6 以降のカーネルでだけ使え、それより古いカーネルでは `EOPNOTSUPP` になる。OS ごとの方法（Linux の `/proc/self/fd`、macOS の `O_SEARCH`）は設計の「OS ごとのファイルを持たない」に反する。パスに対する `chmod` は、差し替えられたシンボリックリンクをたどる。受け入れる制約: umask はプロセス全体に効くので、`MkdirBelow` をほかのファイルの作成と並行して呼んではならない。本番で呼ぶのは `config init` だけで、並行する処理はない。テストでは、`MkdirBelow`・`InitServerFile`・`runInit` を呼ぶテストと umask を変えるテストに `t.Parallel` を付けない。`t.Parallel` を付けたテストは、付けていないテストと同時には動かない（Go の `testing` パッケージの規則）。`internal/config`・`internal/privatefile`・`cmd/yt2column-server` には、今 `t.Parallel` が 1 つもない（`rg -c "t.Parallel" internal/config cmd/yt2column-server` の結果は空、コミット `ac6e685e`）。ADR: なし（このタスクの中で閉じる）。
-   **`internal/config` の外のテストは、テスト用のアクセサで設定ファイルを書いてから `LoadServer` を呼ぶ。** `internal/config/test_helpers.go` に、与えた `HOME` の下の設定ファイルのパスにバイト列をモード `0600` で書き、そのパスを返すアクセサを置く（ステップ 3-2）。今 `LoadServer` に環境変数の表を渡しているテスト（1.4）は、すべてこれを使う。理由: サーバの入口を通して値を確かめ続けられ、パスの規則をテストに写さずに済む。`config.Load`（CLI）に切り替える案は、サーバの入口を確かめなくなるので採らない。ADR: なし。
-   **推奨構成の設定のブロックは、設定ファイルの行の形にし、ガードテストは実際の書式解析で読む。** `README.md`・`README.en.md` の「推奨構成」と `docs/dev/production_profile.md` の「本番の構成」のブロックを、`export` の行から設定ファイルの行（`YT2COLUMN_LLM=...`）に変える。`TestProductionProfileDocsContract` は、ブロックを設定ファイルの書式の解析に通し、上のアクセサで書いたファイルから `LoadServer` で読み込んで、既定の構成と比べる。理由: 文書のブロックを、そのまま設定ファイルに写せる実行できる例として確かめられる。`export` のブロックを残して別に例を足す案は、2 つの形を保つことになるので採らない。ADR: なし（[ADR-0016](../../dev/adr/0016-document-test-policy.md) の「実行できる例」の範囲）。
-   **PR は 4 つに分ける。** PR-1 はフェーズ 1（`internal/privatefile`）、PR-2 はフェーズ 2（書式の解析・雛形・`InitServerFile`）、PR-3 はフェーズ 3 から 5（サーバがファイルを読むこと・廃止した変数の検査の削除・`config init`）、PR-4 はフェーズ 6 と 7（拡張と文書）である。理由: `privatefile` は信頼の境界にある部品で、単独でレビューする。フェーズ 3 と 5 を同じ PR にまとめるのは、ファイルがなければ止まるサーバが、まだ存在しない `config init` を案内したまま `main` に現れるのを避けるためである。ADR: なし。
-   **サーバのテストの補助の書き換えは、フェーズ 3 の前提として、フェーズ 3 の中で行う。** 設計 8 章は「テストの補助の書き換え」を 5 番目に置くが、3 番目の項目である `LoadServer` の変更を入れると、環境変数で設定を渡すサーバのテストがすべて通らなくなる。そのため、補助を設定ファイルを書く形に変える作業は、フェーズ 3 のコミットが通るための前提としてフェーズ 3 に置き、フェーズ 5 では `config init` のための補助だけを足す。フェーズの順は設計のままである。ADR: なし。
-   **サーバの入口で廃止した変数を拒否するテストの行は、フェーズ 3 の前提としてフェーズ 3 の最初に削除だけのコミットで消す。** フェーズ 3 の後、サーバは 6 つのキーをファイルから読み、ファイルは廃止した変数の名前を書式で拒否するので、環境の廃止した変数を拒否するテストの行は成り立たない。この時点では CLI のテストが `retiredLLMErrors` を通るので、削除のコミットのカバレッジの比較が意味を持つ。残りの削除はフェーズ 4 で行う。ADR: なし。

### 1.4. 既存コード調査結果

調べたのはコミット `ac6e685e`（`main`）である。行番号はこのコミットのものである。

**作り直さずに使うもの:**

-   `absoluteHome`（`internal/config/cachedir.go:74-80`）と `appDirName`（同 `:7`）。設定ファイルのパスの規則（設計 3.2.1）が使う。
-   `socketRule` の OS の分類（`internal/config/socketpath.go:30-41`）。規則のない 4 つの OS の分類を、設定ファイルの規則と 1 つの関数で共有する（設計 3.2.1）。`userDirBase`（`cachedir.go:47-70`）も同じ 4 つの OS を並べているが、XDG の変数を読む別の規則なので、本タスクでは触らない。
-   `loadLLM`・`loadSlackWebhook`・`socketPath`・`hasHTTP2Debug`（`internal/config/config.go:480`・`699`、`socketpath.go:49`、`config.go:769`）。`LoadServer`（`config.go:362-383`）の中で `LookupFunc` を渡し分けるだけで、値の検証は書き直さない（設計 3.2.3）。
-   `missingVar`・`missingVarWithReason`・`invalidVar`（`config.go:775-788`）。
-   `stderrWriter` と `splitJoined`（`cmd/yt2column-server/run.go:85-93`・`309-321`）。
-   開き方の前例: `internal/cachelock/cachelock.go:78-82` は `O_NOFOLLOW|O_NONBLOCK` で開き、`File.Stat` で種類を確かめる。`gosec` の抑止もここと同じく、開く 1 行に理由とともに付ける。
-   `udstest.Dir`・`udstest.Listen`（`internal/udstest/udstest.go:33-60`）。短い `HOME` と、Unix ドメインソケットの用意に使う。
-   `TestStartupFailures`（`cmd/yt2column-server/run_test.go:42`）の表の形と、終了コード・設定エラーの行の数・ソケットの有無を確かめる駆動部（`run_test.go:212-250` 付近）。
-   `runChildMode` と `childEnv`（`cmd/yt2column-server/test_helpers.go:539-614`）。子プロセスで `main` を動かすテストの土台。

**まだないもの:** `internal/privatefile`、設定ファイルのパスの規則、書式の解析、雛形、`InitServerFile`、`FileError`・`FormatOverflowError`・`VarError.Source`、`run` の引数の振り分けと `runInit`。

**変えるもの（すべての出現を挙げる）:**

-   **`LoadServer` を呼ぶ場所（`rg -n "LoadServer\(" --glob '*.go'`、`docs/tasks/` の下を除く）:** `cmd/yt2column-server/run.go:133`（本番）、`cmd/yt2column-server/test_helpers.go:145`、`internal/llm/provider/provider_test.go:802`、`cmd/yt2column/docs_test.go:530`・`536`・`621`、`internal/config/socketpath_test.go:175`、`internal/config/config_test.go` の `TestLoadClaudeEffortDefault`（`:329` の副テスト）・`TestLoadServerValid`・`TestLoadServerLLMRejected`・`TestLoadServerWebhookOptional`・`TestServerConfigOutputRedactsWebhook`・`TestLoadServerIgnoresCacheAndYtdlp`・`TestLoadServerSocketPathUnderivable`・`TestLoadServerLLMDefaults`・`TestLoadRejectsRetiredLLMVars`（`:1850` の行）。設計 3.6 の表は `provider_test.go` と `cmd/yt2column/docs_test.go` を挙げていないが、どちらも `LoadServer` に環境変数の表を渡すので書き換える（ステップ 3-3）。
-   **サーバのテストが環境変数で設定を渡す場所:** `cmd/yt2column-server/test_helpers.go` の `serverEnv`（`:113-151`）・`startServer`（`:193`）・`startServerChild`（`:570`）。`e.vars` への書き込みと削除は `run_test.go:79`・`80`・`92`・`103`・`112`・`129`・`147`・`351`・`352`・`430` にある。`relay_e2e_test.go:253` は `HOME` を読むだけで、`signal_test.go`・`server_test.go` は `newServerEnv` を使うだけである。`TestStartupFailures` の「LLM model empty and API key missing」の行（`run_test.go:74-86`）が期待する `configuration: YT2COLUMN_LLM`・`configuration: DEEPSEEK_API_KEY` は、出どころが設定ファイルになると行の形が変わる（設計 4.2「値」）。
-   **廃止した変数の検査（`rg -n "retired|Retired" cmd internal`）:** 本番は `config.go:65-67`（注釈）・`101-109`・`130-141`・`150-166`・`354-361`・`441-452`・`473-482`・`512-523`。テストとテスト用の補助は、`internal/config/test_helpers.go:50-52`・`87-107`、`config_test.go:572`・`582`・`631-636`・`659-720`・`1160`・`1187-1191`・`1447-1448`・`1658-1680`・`1814-1888`、`socketpath_test.go:150-158`・`162`・`170`・`194`・`206`・`226`、`cmd/yt2column/run_test.go:464-471`・`1830-1831`・`2239-2340`、`cmd/yt2column/makefile_test.go:46-52`・`99-208` と `goFilesUnder`（`:208` から。呼ぶのは `TestRetiredModelEnvIsGone` だけ）、`internal/llm/deepseek/testutil/make.go:88-108`（`CheckRetiredModelEnvNotRead`。呼ぶのは `makefile_test.go:110` だけ）、`cmd/yt2column-nmhost/main_test.go:135-153`・`253`・`319`、`cmd/yt2column-server/run_test.go:88-106`・`337-342`。この検索は `cmd/yt2column-server/docs_test.go:53` も出すが、これは共有トークンの版からの更新の節（`ReadmeUpgradeHeading`）についての注釈で、本タスクの対象ではない。
-   **「環境変数」を前提にした文字列とコメント:** `config.go:1-3`（パッケージの注釈）、`config.go:31`、`config.go:175`・`178`（`ErrMissing`・`ErrInvalid` の文）、`run.go:23-28`（終了コードの注釈。`exitFailure` を待ち受けの失敗だけとしている）、`run.go:110-112`（手順 1 と 2 がファイルに触れないという注釈）、`run.go:127`（引数の拒否の行）、`run.go:131-132`（「Each rejected variable is reported on its own line」）、`cmd/yt2column-server/run_test.go:30-32`（`configDocument` の注釈）、`extension/test/secretsAbsent.test.ts:28`（「The server's environment variable a message may tell the user to set.」）。`secretsAbsent.test.ts` の `messageEnvName` と、`:110`・`:182-217` の「environment name」の語は、識別子とテストの説明で、許す名前（`SLACK_WEBHOOK_URL`）は変わらないので変えない。
-   **拡張の案内:** `extension/src/core/i18n/en.ts:27`・`ja.ts:23-24` の `setWebhookStep`。`secretsAbsent.test.ts:31-32` は、メッセージのファイルの文字列リテラルの中でだけ `SLACK_WEBHOOK_URL` を許す。新しい文もこの名前を含むので、このテストはそのまま通る。
-   **文書の中の廃止した変数と、それへのリンク:** `README.md:352`（トラブルシューティングの項目のリンク）・`431-433`（「設定」の段落とリンク）・`467` からの節、`docs/dev/guides/cli.md:69-71`・`255`・`257`（`257` は消す節へのリンク）、`docs/dev/project_overview.md:161`（「次に述べる旧変数の検査だけは例外で」）・`163`、`docs/translation_glossary.md:159`・`165`（「設定エラー」を環境変数の拒否と定義する）・`185`（「設定」を環境変数による設定と定義する）・`186`。
-   **文書のガードテスト:** `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`:34`）、`cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`（`:637`、`checkExportBlock` `:431`・`checkExportLines` `:450`・`checkExportBlockMatchesDefaults` `:524`・`checkProfileTable` `:572`）、`TestLLMDefaultsDocsContract`（`:256`、README の「変数」の列の表を読む）、`TestReadmeLinks`（`:668`、README・`cli.md`・`development.md` のリンク）。README の見出しを探す場所は `internal/doctestutil/readme.go:80-109` にある。
-   **パッケージの一覧のガード:** `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（`:629`）は、`package_reference.md` がすべてのパッケージを挙げることを確かめる。`internal/privatefile` の行はフェーズ 1 で足す。
-   **軽量プロセスの除外の表:** `docs/dev/conventions/requirements_process.md` の「Mechanically checked conditions」は、信頼の境界のパッケージを挙げ、「A PR that adds a package or a file on a trust boundary adds its `exclude` line to this table in the same PR.」とする。秘密を持つファイルを読み書きする `internal/privatefile` は信頼の境界にあるので、フェーズ 1 で `exclude internal/privatefile/` を足す。
-   **テスト用の補助のビルドタグと lint:** `internal/pipeline/pipeline_test.go::TestFakesCarryBuildTag`（`:553`）は、`test_helpers*.go` の最初の行が `//go:build test` か `//go:build test || integration` であることを確かめる。`make lint` は `test,integration` のタグで動き、`gosec` の除外は `_test.go` だけにかかる（`.golangci.yml:191-200`）。そのため、`test_helpers.go` の中の、変数のパスに対するファイル操作には、1 行ごとの `//nolint:gosec` と、テスト専用である理由が要る（前例は `internal/coveragecompare/test_helpers.go:298-470`）。

**Unix だけのビルド:** `internal/config` は `//go:build unix` の `internal/privatefile` を import するので、Unix でだけビルドされる。設計 2.1 がこれを決めている（3 つの実行ファイルはすでに Unix だけを対象にする）。

**`*VarError` の構築の経路（終了コードを型で選ぶ）:** `privatefile` の番兵から `config` の番兵への対応は `errors.Is` の `switch`（設計 4.1）で、`runInit` の終了コードは「`InitServerFile` の失敗が `*VarError` か」で選ぶ（設計 3.3）。本番のコードで `*VarError` を作るのは `missingVar`・`missingVarWithReason`・`invalidVar` の 3 つの関数だけである（`config.go:776`・`782`・`787`。`rg -n "VarError\{" --glob '*.go' internal cmd` の `_test.go` 以外の結果はこの 3 行だけ、コミット `ac6e685e`）。`InitServerFile` がこれらを呼ぶのはパスの導出（`serverFileLocationFor`）だけにする。この経路は、`TestConfigInit` の表（ステップ 5-2）で、`HOME` の誤りが `2`、それ以外のすべての失敗が `1` になることとして確かめる。型をリーフのパッケージに移す案は採らない（`VarError` は CLI と共有する今の公開の型で、フィールドも公開である）。

### 1.5. 実装への申し送りへの対応

| 項目 | 対応 |
|---|---|
| I-01 `MkdirBelow` が作ったディレクトリのモードを umask によらず `0700` にする | 1.3 の 1 つ目の決定。実装はステップ 1-3、umask `0400`・`0100` を含むテストはステップ 1-4 |

## 2. 実装の手順

各ステップの完了条件は、特に書かないかぎり `make fmt`・`make test`・`make lint`・`make deadcode` が通ることである。テスト用のアクセサや補助は、それを最初に使うステップで足す。

### フェーズ 1: `internal/privatefile`（設計 8 章の 1）

**コミット:** 関数ごとに、その関数とテストを同じコミットにする。1 つ目はステップ 1-1・1-2 と、ステップ 1-4 の `Read` のテスト（`TestRead`・`TestReadUnreadable`・`TestReadChecksDescriptor`・`TestOwnerAndModeCheck`）、2 つ目は `Create` とそのテスト、3 つ目は `MkdirBelow` とそのテストである。テストのない関数を残したコミットは `make deadcode` と `unused` で止まるためである。テスト用の差し込み口の変数と、それを差し替える関数は、それを最初に使うテストと同じコミットで足す。

#### ステップ 1-1: パッケージ・エラーの型・テスト用の差し込み口

**変えるファイル:** `internal/privatefile/privatefile.go`（新規）、`internal/privatefile/test_helpers.go`（新規、`//go:build test`）、`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/requirements_process.md`

- [x] `//go:build unix` の `privatefile.go` に、設計 3.1 の定数・番兵・`UnsafeReason`（`String()` を持つ）・`UnsafeError` を置く。パッケージの注釈に、設定に依存しないことと、`MkdirBelow` が umask を一時的に変えるのでほかのファイルの作成と並行して呼ばないことを書く。
- [x] 所有者とモードの判定を、`fs.FileInfo` と実効ユーザーの ID を受け取る非公開の関数にする。`Read` の 2 つの検査（`Lstat` と `fstat`）がこれを共有する（設計 7.1）。
- [x] テスト用の差し込み口は、`privatefile.go` の非公開のパッケージ変数（関数の値。既定は何もしない）にする。差し込む場所は、`Read` の `Lstat` の後、`Create` の `Lstat` の後と書き込み・`Sync`・`link`・一時ファイルの削除の前、`MkdirBelow` の各 `mkdir` の後である。`test_helpers.go` には、その変数を差し替え、`t.Cleanup` で元に戻す関数だけを置く。
- [x] `package_reference.md` に `internal/privatefile` の行を足し（`TestPackageReferenceListsPackages`）、`requirements_process.md` の「Mechanically checked conditions」の表に `exclude internal/privatefile/` を足す。

#### ステップ 1-2: `Read`

**変えるファイル:** `internal/privatefile/privatefile.go`

- [x] 設計 3.1「`Read` の手順」と 6.1 のとおりに実装する。`Lstat` の結果は拒否にだけ使い、読んでよいと決めるのは開いた記述子の `fstat` の結果だけにする。開いた後のどの拒否でも記述子を閉じる。
- [x] `os.OpenFile` の行に、`cachelock.go:82` と同じく、その 1 行だけを対象にした `//nolint:gosec` と理由を付ける。

#### ステップ 1-3: `Create` と `MkdirBelow`

**変えるファイル:** `internal/privatefile/privatefile.go`

- [x] `Create` を設計 3.1「`Create` の手順」と 6.3 のとおりに実装する。一時ファイルの接頭辞は、パスの最後の要素の前に `.` を、後に `.tmp-` を付けたもの（設定ファイルでは `.config.env.tmp-`）とする。`privatefile` が設定ファイルの名前を持たないためである。
- [x] `MkdirBelow` を設計 3.1 の振る舞いと 1.3 の 1 つ目の決定の手順で実装する。umask は `mkdir` の直後に戻し、`defer` でも必ず戻す。作った要素を開く行にも、1 行だけの `//nolint:gosec` と理由を付ける。

#### ステップ 1-4: `privatefile` のテスト

**変えるファイル:** `internal/privatefile/privatefile_test.go`（新規）

- [x] 設計 7.1「`internal/privatefile`」の場合を、`TestRead`・`TestReadUnreadable`・`TestReadChecksDescriptor`・`TestOwnerAndModeCheck`・`TestCreate`・`TestCreateExisting`・`TestCreateRaces`・`TestCreateFailureLeavesNothing`・`TestMkdirBelow`・`TestMkdirBelowSwap` に分けて書く。Unix ドメインソケットの用意は `udstest.Dir` と `udstest.Listen` を使う（パスの長さの上限のため）。
- [x] `TestCreate` は、所有者のビットを落とす umask（`0400`・`0200`）を含めて、作ったファイルのモードがちょうど `0600` で中身が一致することを確かめる。`TestMkdirBelow` は、umask `0400`・`0100`・`0000`・`0077` で作ったディレクトリのモードがちょうど `0700` になること（I-01）、すでにある要素（例: モード `0755`）を変えないこと、親が setgid のビットを持つときにも作った要素が `0700` になること（Linux で setgid のディレクトリを用意できないときは `t.Skip`）を確かめる。
- [x] `TestReadChecksDescriptor` は、`Lstat` の後の差し込み口で、通常のファイルを、モード `0644` のファイル・ディレクトリ・FIFO・正しいファイルへのシンボリックリンクに差し替える場合と、消す場合を試す（設計 7.3）。FIFO の場合は `Read` を goroutine で呼んで短い期限で待ち、後始末で FIFO を書き込み用に開いて、止まった goroutine を解く。
- [x] `TestCreateRaces` は、`Create` の `Lstat` の後の差し込み口でパスにファイルを置き、`ErrExist` になってそのファイルが変わらず、`.config.env.tmp-` で始まる名前が残らないことを確かめる。`TestCreateFailureLeavesNothing` は、書き込み・`Sync`・`link` の前の差し込み口で失敗させて、パスに何もなく一時ファイルが残らないことを確かめ、一時ファイルの削除を失敗させた場合に、成功ならその名前を返し、失敗ならエラーの文がその名前を含むことを確かめる。`TestMkdirBelowSwap` は、`mkdir` の後の差し込み口で作った名前をシンボリックリンクに替え、「作れなかった」になり、リンク先のモードが変わらないことを確かめる。
- [x] 所有者の判定は、別の所有者のファイルを root でないと作れないので、`TestOwnerAndModeCheck` で関数を直接呼んで確かめる。
- [x] 権限を外したディレクトリを用意したテストは、用意の直後に、モードを戻す `t.Cleanup` を登録する（`t.TempDir` の削除より先に動く）。root では結果が変わるテスト（モード `0000`、探索できない親）は、`os.Geteuid() == 0` のとき `t.Skip` する（設計 7.1）。
- [x] 特別なビットのモード（setuid・setgid・sticky）は `t.TempDir` の下のファイルに付け、`Lstat` で付いたことを確かめ、付かなければ `t.Fatalf` にする（macOS の `/tmp` の下では、グループが `wheel` のファイルに setgid を付けられない）。umask を変えるテストは、`t.TempDir` を umask を変える前に作り、呼んだ直後に umask を戻す。どのテストにも `t.Parallel` を付けない。

**完了条件:** 共通の完了条件と `make build`（テストのタグなしで、差し込み口を含めてビルドできること）。

### PR-1 作成ポイント: private file package

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4

**推奨タイトル**: `feat(0066): 所有者だけが読めるファイルを検査して読み書きする privatefile を追加する`

**レビュー観点**: 検査と読み込みが同じ記述子を使い、Lstat の結果で許可しないこと / Create が既存の名前を上書きせず書きかけを残さないこと / MkdirBelow の umask の扱いと作ったものだけを変えること / 差し込み口を使った競合と失敗のテスト

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-2・1-3 は信頼の境界のファイル操作（設計 5.1 の脅威の対策）で、mkplan のパネルモードの条件（セキュリティのゲート）に当たる

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/715
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 書式の解析・雛形・`InitServerFile`（設計 8 章の 2）

#### ステップ 2-1: 設定ファイルのパスの規則

**変えるファイル:** `internal/config/configpath.go`（新規）、`internal/config/socketpath.go`、`internal/config/configpath_test.go`（新規）

- [x] OS の分類を 1 つの非公開の関数にし、`socketRule` と設定ファイルの規則の両方がそれを使うように `socketpath.go` を変える（設計 3.2.1）。
- [x] `serverFileLocation` を設計 3.2.1 のとおりに置き、`serverFileLocationFor(goos string, lookup LookupFunc)` にする（`socketPath(goos, lookup)` と同じ形。本番の呼び出しは `runtime.GOOS` を渡す）。理由の定数 `reasonConfigPathAbsent` に、設計 3.2.1 の文「no absolute configuration file path could be derived from HOME」を置く。
- [x] `TestServerFileLocation` を書く: `goos`（`darwin`・`linux`・`freebsd`・規則のない 4 つ）と、`HOME` が未設定・空・相対の場合の表で、パス・基点・作ってよいディレクトリを確かめる。`XDG_CONFIG_HOME` を設定してもパスが変わらないこと、`HOME` のほかに変数を読まないこと（読んだ名前を記録する `LookupFunc`）、エラーが `HOME` の値を含まないことも確かめる。今の `TestSocketPath`（`socketpath_test.go:45`）は、変えずに通ることを確かめる。

#### ステップ 2-2: 書式の解析とエラーの型

**変えるファイル:** `internal/config/serverfile.go`（新規）、`internal/config/serverfile_test.go`（新規）

- [x] 設計 3.2.2 の `serverFileKeys`・`serverFileMaxSize`・`serverFileMaxReported`・`serverFile`・`lookup`・`parseServerFile` を、設計 3.2.2 と 6.2 の規則で実装する。行は `bytes.Split` で分ける。
- [x] 設計 4.1 の番兵（`ErrConfigFile*`）・`FileError`・`FormatOverflowError` を置く。`Unwrap` は番兵 1 つだけを返す。`Error()` の文の形は設計 4.2 の表に従い、英語の文は実装で決める。
- [x] `TestParseServerFileAccepts` を書く: 受理する形（空行、コメントの行、空白を伴わない `#` と `=` を含む値、空の値、LF で終わらない最後の行、64 KiB ちょうど）の表で、`lookup` が返す値と `ok` を確かめる。
- [x] `TestParseServerFileRejects` を書く: 要件 F-003「拒否するもの」のそれぞれと、UTF-8 として正しくないファイルの表で、`errors.Is(err, ErrConfigFileFormat)`、行番号（ファイル全体の拒否では 0）、理由、重複のときのキーの名前を確かめる。`Error()` の文が、パスと、行の拒否では `line <n>`、重複ではキーの名前を含み、ファイル全体の拒否では行番号を含まないことも確かめる。どの拒否の文も、ファイルの値・行・未知のキーの文字列を含まないことを、表のすべての行について確かめる。
- [x] `TestParseServerFileOverflow` を書く: 拒否が 21 行以上のとき、`*FileError` が行番号の小さい順にちょうど 20 個で、`*FormatOverflowError` が残りの数を持つことを確かめる。
- [x] `TestParseServerFileDuplicate` を書く: 最初の出現がほかの理由で拒否されていても、2 回目の出現が「重複」になることを確かめる（設計 3.2.2「重複」）。

#### ステップ 2-3: 雛形と `InitServerFile`

**変えるファイル:** `internal/config/serverfile.go`、`internal/config/serverfile_test.go`、`internal/privatefile/privatefile.go`（モードを数値で示す `UnixMode` を公開し、失敗の後に消せなかった一時ファイルを `LeftoverError` で示す）、`internal/privatefile/test_helpers.go`（`Create` を `link` の前で失敗させる、ほかのパッケージのテスト用の補助）

- [x] `serverFileTemplate`・`InitServerFile`・`InitResult` を設計 3.2.5 のとおりに置く。雛形の英語の文面は実装で決める。
- [x] `privatefile` の番兵から `config` の番兵への対応を、設計 4.1 の箇条のとおりに `errors.Is` の `switch` で書く。`default` は `Read` では「読めない」、`Create`・`MkdirBelow` では「作れなかった」にする。`privatefile` の型は `FileError` の外に出さない。
- [x] `TestServerFileTemplate` を書く: 雛形を `parseServerFile` に通すと拒否がなく値を 1 つも持たないこと、すべての行が空行かコメントの行であること、`serverFileKeys` のそれぞれが `# <キー>=` の形のコメントの行にあることを確かめる。
- [x] `TestInitServerFile` を書く: `t.TempDir` の `HOME` で、作ったファイルのパス・中身（雛形と一致）・モード `0600`、作ったディレクトリのモード `0700` を確かめる。基点がない、すでにある（通常のファイル・リンク先のないシンボリックリンク・ディレクトリ）、作れない（親に書けない）の各場合の番兵と `Path` を確かめ、作れない場合に、それまでに作った親が残ることを確かめる。読んだ変数が `HOME` だけであることを、6 つのキーに値を入れた記録する `LookupFunc` で確かめる。`HOME` の誤りでは `*VarError` を返し、何も作らないことを確かめる。親に書けない場合は、モードを戻す `t.Cleanup` を登録し、root では `t.Skip` する。
- [x] `TestFileErrorMapping` を書く: `privatefile` の各番兵と `UnsafeReason` が設計 4.1 の表のとおりの `config` の番兵に対応し、`FileError` の文がパスと設計 4.2 の分類ごとの内容（ファイルがない場合の `yt2column-server config init`、シンボリックリンク、通常のファイルでない、所有者が違う、モードと `chmod 600`、読めない）を含むことを確かめる。

### PR-2 作成ポイント: config file parser and template

**対象ステップ**: 2-1 / 2-2 / 2-3

**推奨タイトル**: `feat(0066): 設定ファイルのパスの規則・書式の解析・雛形を追加する`

**レビュー観点**: 書式の解析が要件 F-003 の受理する形だけを受け付け、直さずに拒否すること / 拒否の文が値・行・未知のキーを含まないこと / privatefile の番兵から config の番兵への対応と default の倒れ方 / OS の分類を socketRule と共有すること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-2 は信頼できない入力の境界の解析で、条件付きの検査（拒否の分類・値を出さない）の複数に当たる

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/723
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `LoadServer` の組み立ての変更と `VarError.Source`（設計 8 章の 3）

このフェーズは、1.3 の 5 つ目と 6 つ目の決定のとおり、前提としてテストの補助の書き換え（設計 8 章の 5 の一部）と、サーバの入口の廃止した変数のテストの行の削除（設計 8 章の 4 の一部）を含む。

#### ステップ 3-1: サーバの入口で廃止した変数を拒否するテストの行を削除する

**変えるファイル:** `internal/config/config_test.go`、`cmd/yt2column-server/run_test.go`

**コミット:** このステップだけの、削除だけのコミット。

- [x] `TestLoadRejectsRetiredLLMVars` の `LoadServer` の行（`config_test.go:1850-1856`）、`TestStartupFailures` の廃止した変数の 2 行（`run_test.go:88-106`）、`lastRetiredLLMVar`（`run_test.go:337-342`）を消し、`make coverage-compare BASE=<親のコミット>` で確かめる（1.2）。

#### ステップ 3-2: 設定ファイルへの切り替え

**変えるファイル:** `internal/config/config.go`・`test_helpers.go`・`config_test.go`・`socketpath_test.go`、`cmd/yt2column-server/run.go`・`test_helpers.go`・`run_test.go`、`internal/llm/provider/provider_test.go`、`cmd/yt2column/docs_test.go`

**コミット:** このステップの全体を 1 つのコミットにする。`LoadServer` だけを変えると、環境変数で設定を渡すテストがすべて止まるためである。

- [x] 設計 3.2.4 の `VarSource`・`VarSourceUnspecified`・`VarSourceConfigFile` と `VarError.Source` を足し、`Error()` を `Source` の `switch` にする（`default` は今の形）。理由の定数 `reasonNotSet`（「is not set」）・`reasonEmpty`（「is empty」）を足す（設計 3.2.3）。
- [x] `ErrMissing` の文 `"environment variable is unset or empty"` と `ErrInvalid` の文 `"environment variable has an invalid value"` を、出どころを言わない文に変える（設計 3.2.4）。`config.go:31` のコメントに、これらの名前が設定ファイルのキーでもあることを足す。
- [x] `LoadServer` を設計 3.2.3 の 3 つの段に組み替え、注釈を設計 3.2.3 の注釈に置き換える。ファイルから読んだキーの `*VarError` に `Source` を付け、`ErrMissing` の `Reason` を選ぶ（設計 3.2.3・3.2.4）。`ServerConfig` の注釈（`config.go:317-320`）に、6 つのキーを設定ファイルから読むことを足す。`run.go:131-132` の注釈を、拒否ごとに 1 行を書く形に直す。
- [x] `internal/config/test_helpers.go` に、1.3 の 2 つ目の決定のアクセサを足す。与えた `HOME` の下の設定ファイルのパスに、与えたバイト列を書いてパスを返す。親は `os.MkdirAll` で作ってから `os.Chmod` で `0700` にし、ファイルは `os.Chmod` で `0600` にする（umask に頼らない。`privatefile.MkdirBelow` は umask を変えるので使わない）。すでにあるファイルは中身を置き換える。キーと値の表から行を作る小さな関数と、変数の表を設定ファイルのキーとそれ以外に分け、キーをファイルに書いて残りの環境を返すアクセサ（`ServerLookupForTest`）も置く。`internal/config` の外のテストは、このアクセサで `LoadServer` に渡す。ファイル操作の各行に、テスト専用である理由を付けた `//nolint:gosec` を付ける。
- [x] `serverEnv` を、環境変数の表（`HOME`・`GODEBUG` など）と、設定ファイルのキーと値の表を持つ形にする。生のバイト列・モード・ファイルがないことなどの状態は、テストが書いたファイルを変えて作る（`TestStartupFailures` では行ごとの `fileSetup`）。ファイルは、`setup` の後、`run` の直前（子プロセスでは起動の直前）に 1 回だけ書く。`HOME` が未設定か相対のときは書かない。`socketPath` は、ファイルを書かずに `HOME` からソケットのパスを求める（`LoadServer` を呼ばない）。`childEnv` は 6 つのキーを子の環境に渡さない。
- [x] 1.4「サーバのテストが環境変数で設定を渡す場所」の書き込みと削除を、設定ファイルの状態への操作に変える。`run_test.go:74-86` の行の期待を、設定ファイルのキーの行の形に合わせる。
- [x] 1.4「`LoadServer` を呼ぶ場所」の `internal/config` のテスト、`provider_test.go:790-810`、`cmd/yt2column/docs_test.go:493-499`・`530-536`・`619-622` を、`udstest.Dir` の `HOME` の下にアクセサでファイルを書いてから `LoadServer` を呼ぶ形にする（1.2「テストの `HOME`」）。`docsProfileEnv` の固定の `HOME` も `udstest.Dir` に変える。`TestLoadServerSocketPathUnderivable`（`config_test.go:989`）の「長すぎる」行は、`udstest.Dir` の下に延ばした `HOME` を作ってファイルを置く。「未設定」の行は `LoadServer` の段 1（設計 3.2.3）で止まるので、期待する理由を設定ファイルのパスの理由に変える。これらのほかは、確かめる内容を変えない。

#### ステップ 3-3: `internal/config` の新しいテスト

**変えるファイル:** `internal/config/config_test.go`

- [x] `TestVarErrorSource`（ゼロ値の `Source` が今の `<Name>: <Reason>` の形で、`VarSourceConfigFile` が設定ファイルのキーとして示す形であること）を書く。
- [x] `TestLoadServerReadsFile`（要件 AC-04 の 2 つの例）、`TestLoadServerValueRejections`（拒否ごとに 1 つの要素、`Source` が設定ファイル、文が「environment variable」を含まず値を含まない）、`TestLoadServerEmptyValues`（使うキーの空の値は「is empty」、使わないプロバイダのキーの空の値は受理）を書く。
- [x] `TestLoadServerIgnoresEnvSettings` を書く: `ANTHROPIC_API_KEY` を持たない設定ファイル（例: 雛形）と、環境の `ANTHROPIC_API_KEY` で、「is not set」の設定ファイルのキーの拒否になること、環境の `YT2COLUMN_LLM` が生成の選択を変えないこと、環境に 6 つのキーと廃止した 5 つの変数を空の値を含めて入れても結果がファイルだけで決まることを確かめる。
- [x] `TestLoadServerFileRejections` を書く: ファイルがない・シンボリックリンク・ディレクトリ・モードの誤り・読めない・書式の誤り・大きさの上限（65,536 バイトは受理、65,537 バイトは行番号 0 の書式の拒否）が、それぞれの番兵の `*FileError` になり、ゼロ値の `ServerConfig` を返すことを確かめる。読めない場合は、モードを戻す `t.Cleanup` を登録し、root では `t.Skip` する。
- [x] `TestLoadServerReadsOnlyHomeAndGodebug`（環境から読む名前が `HOME` と `GODEBUG` だけ）を書き、`TestLoadHTTP2Debug`（`config_test.go:1011`）に `LoadServer` の行を足す。
- [x] `TestLoadForIgnoresServerFile` を書く: `HOME` の下に、6 つのキーの値を持つ設定ファイル、書式の誤りのあるファイル、モード `0644` のファイルがあっても、`Load`・`LoadFor` の結果が環境変数だけで決まることを確かめる。

#### ステップ 3-4: サーバの新しいテスト

**変えるファイル:** `cmd/yt2column-server/run_test.go`

- [x] `TestStartupFailures` に、設定ファイルがない、シンボリックリンク、ディレクトリ、モード `0644`、読めない（root では飛ばす）、書式の誤り（1 行）、大きさの上限を超える、値の拒否の行を足す。それぞれについて、終了コードが `2` であること、設定エラーの行が出ること、ソケットのディレクトリ・起動のロック・ソケットが作られないこと、設定ファイルとその親のモードと中身が変わらないことを確かめる。今の `HOME` の誤りの行とソケットのパスが長すぎる行は、そのまま通ることを確かめる。
- [x] `TestFormatRejectionLines`（21 行以上の書式の拒否で、標準エラーの設定エラーの行がちょうど 21 行）と、`TestModeRejectionChmodCommand`（パスに空白を含む `HOME` で、モードの拒否の行から `chmod` のコマンドを取り出して `sh -c` で実行すると、ファイルのモードが `0600` になる。設計 4.2「直し方のコマンドの引用」）を書く。
- [x] `TestServerIgnoresEnvSettings` を書く: 環境に 6 つのキーと廃止した 5 つの変数（空の値を含む）を入れ、ファイルが正しければサーバが待ち受け、標準エラーにそれらの変数について何も書かないことを確かめる。`ANTHROPIC_API_KEY` を持たないファイルと環境の `ANTHROPIC_API_KEY` では、そのキーが設定ファイルにないことを示す設定エラーで止まることを確かめる。

### フェーズ 4: 廃止した変数の検査の削除（設計 8 章の 4）

#### ステップ 4-1: 検査をやめる（振る舞いの変更）

**変えるファイル:** `internal/config/config.go`・`config_test.go`、`cmd/yt2column/run_test.go`

**コミット:** このステップだけのコミット。振る舞いを変えるコミットで、テストの削除のコミットではない（`loadLLM` の行が書き換わるので `coverage-compare` は使えない。1.2）。コミットメッセージにも、これが振る舞いの変更のコミットであり、テストの削除のコミットではないことを書く。

- [x] `retiredLLMErrors` と、`loadLLM` の中のその呼び出しを消す（`config.go:482` は空の `errs` で始める）。呼ぶテストがなくなる `RetiredLLMVarErrorForTest`（`test_helpers.go`）も、`make deadcode` を通すため、このコミットで消す。
- [x] 拒否を確かめるテストを、無視することを確かめる形に書き換える。`TestLoadRejectsRetiredLLMVars` を `TestLoadForIgnoresRetiredVars` にする（廃止した 5 つの変数を、空の値と DeepSeek を選ぶ値で環境に入れても、`Load` と `LoadFor`（`Need{}` を含む）が拒否せず、結果が廃止した変数のない環境と同じであること）。`TestRunRejectsRetiredLLMVars`（`cmd/yt2column/run_test.go:2239`）を、CLI が止まらず、その値を使わないことを確かめる形にする。`cmd/yt2column/run_test.go:464-471` の行、`TestLoadErrorsOmitValues`（`config_test.go:631-636`）・`TestLoadReportsAllInvalid`（`:659-720`）・`TestLoadFor`（`:1187-1191`）の廃止した変数の部分は、このコミットでは消さずに、拒否を期待しない形に直す。不要になった部分は、ステップ 4-2 の削除だけのコミットで消す。`withRetiredLLMVars` の呼び出し（`socketpath_test.go`）と `hostEnvs`（`cmd/yt2column-nmhost/main_test.go:142-151`）は、このコミットでは変えない。

#### ステップ 4-2: 使われなくなったテストと定義を削除する

**変えるファイル:** `internal/config/config.go`・`config_test.go`・`socketpath_test.go`・`test_helpers.go`、`cmd/yt2column/run_test.go`・`makefile_test.go`、`internal/llm/deepseek/testutil/make.go`、`cmd/yt2column-nmhost/main_test.go`

**コミット:** このステップだけの、削除だけのコミット。`make coverage-compare BASE=<親のコミット>` で確かめる（1.2）。

- [x] 本番の定義: `retiredLLMVar`・`retiredLLMVars`、`reasonRetiredLLM`・`reasonRetiredReviewLLM`・`reasonRetiredClaudeEffort`（どれも関数ではない）。
- [x] テスト用の補助とテスト: `RetiredLLMVarsForTest`（`test_helpers.go:87-97`。`RetiredLLMVarErrorForTest` はステップ 4-1 で消す）、`withRetiredLLMVars`（`socketpath_test.go:150-158`）と `hostEnvs` の廃止した変数の組（これらが参照する定義が消えるので、廃止した変数の名前をテストの中に直接書くか、組ごと消す。どちらにするかは実装で決める。組ごと消す場合は、その削除もこのコミットに含める）、`CheckRetiredModelEnvNotRead`（`make.go:88-108`）、`retiredLLMEnvs`・`retiredLLMEnvMentioned`・`retiredLLMEnvRE`・`goFilesUnder`（`makefile_test.go`）、`TestRetiredClaudeEffortReason`、`retiredLLMVarsWant`・`TestRetiredLLMVars`、`TestMakeTargetsDoNotReadRetiredModelEnv`、`TestRetiredLLMEnvMentioned`、`TestRetiredModelEnvIsGone`、`markRetired`（`config_test.go:572`・`582`）。使われなくなった import も消す。

#### ステップ 4-3: 注釈を直す

**変えるファイル:** `internal/config/config.go`・`test_helpers.go`・`config_test.go`・`socketpath_test.go`、`cmd/yt2column/run_test.go`、`cmd/yt2column-nmhost/main_test.go`

- [x] `defaultProvider` の注釈（`config.go:65-66`）から「The retired variables' rejection names it too, so it is written once.」を消す。`LoadFor` の注釈（`:448-452`）から廃止した変数の例外の文を消す。`loadLLM` の注釈（`:473-474`）の契約から廃止した変数の部分を消す。パッケージの注釈（`:1-3`）を、CLI は環境変数から、サーバは設定ファイルから読むことに合わせて直す（設計 3.2.3）。
- [x] `test_helpers.go:50-52`、`cmd/yt2column/run_test.go:1828-1831`、`main_test.go:135-137`・`253`・`319`、`config_test.go:1160`・`1445-1448`、`socketpath_test.go:161-163`・`206` の注釈の、廃止した変数を拒否するという記述を消す。

### フェーズ 5: 引数の振り分けと `runInit`（設計 8 章の 5）

#### ステップ 5-1: 引数の振り分けと `runInit`

**変えるファイル:** `cmd/yt2column-server/run.go`

**コミット:** このステップとステップ 5-2 を 1 つのコミットにする。

- [ ] `run` の最初で、設計 3.3 のとおりに引数を振り分ける。引数の拒否の行（`run.go:127`）を、受け付ける形（引数なし、または `config init`）を示し、`README.en.md` へ案内する行に置き換える。
- [ ] `runInit` を設計 3.3 と 6.3 のとおりに実装する。`*VarError` は `exitUsage`、それ以外の失敗は `exitFailure`、成功は `exitOK` を返す。成功のときは作ったパスの 1 行を、一時ファイルが残ったときはその名前の 1 行を足して、`stderrWriter` を通して標準エラーに書く。標準出力には何も書かない。
- [ ] 終了コードの注釈（`run.go:23-28`）に、`config init` の失敗が `exitFailure` になることを足す。`run` の注釈の「Steps 1 and 2 touch no network and no file」（`:110-112`）を、手順 1 が設定ファイルを読み、何も書かないことに合わせて直す。

#### ステップ 5-2: 引数と `config init` のテスト

**変えるファイル:** `cmd/yt2column-server/run_test.go`、`cmd/yt2column-server/test_helpers.go`、`internal/config/test_helpers.go`

- [ ] `TestStartupFailures` の「an argument」の行（`run_test.go:64-70`）は、削除せずに、新しい行の文に合わせて期待を直す。`configDocument` の注釈（`:30-31`）も合わせる。
- [ ] `TestRunArgs` を書く: 引数なし（ファイルが正しければ待ち受ける）、`config init`（雛形を作る）、要件 AC-29 の例（`config`、`init`、`config init extra`、`--help`、`config Init`）の表で、終了コードを確かめ、拒否のときに空の `HOME` の下に何も作らないことを確かめる。この表の `serverEnv` は、ファイルを書かない状態で始める。
- [ ] `TestConfigInit` を書く: 空の `HOME` では、終了コードが `0` で、作ったファイルが雛形と一致してモードが `0600` であり、標準エラーにパスの 1 行があり、ソケットのディレクトリ・起動のロック・ソケットが作られていないこと。すでにある（通常のファイル・リンク先のないシンボリックリンク・ディレクトリ）ときは、終了コードが `1` で、既存のものが変わらないこと。基点（macOS では `$HOME/Library/Application Support`、ほかでは `$HOME`）がないときは、終了コードが `1` で、何も作らないこと。作れないとき（親に書けない）は、終了コードが `1` で、書きかけがなく、作った親が残ること。一時ファイルの削除に失敗したとき（`privatefile` の差し込み口）は、終了コードが `0` で、その名前の行があること。`HOME` の誤りでは、終了コードが `2` で、何も作らないこと。ソケットのパスの上限を超える長い `HOME` でも作れること。親に書けない場合は、モードを戻す `t.Cleanup` を登録し、root では `t.Skip` する。雛形と比べるため、`internal/config/test_helpers.go` に雛形をバイト列で返すアクセサを足す。
- [ ] `TestConfigInitStdout` を書く: 子プロセスで `main` を `config init` の引数で動かし、標準出力が空で、終了コードが `0` であることを確かめる（`run` は標準出力を受け取らないので、子プロセスで確かめる）。`test_helpers.go` に、ソケットを待たずに子プロセスを最後まで動かす補助を足す。
- [ ] `TestConfigInitThenStart` を書く: ファイルを書かない状態の `serverEnv` で `config init` を実行し、そのファイルのままサーバを起動すると、書式と検査を通って `ANTHROPIC_API_KEY` がないことを示す設定エラーで止まり（AC-26）、そのファイルにキーを書き足すとサーバが待ち受けることを確かめる（設計 7.2）。
- [ ] このステップのテストに `t.Parallel` を付けない（1.3）。

### PR-3 作成ポイント: server reads the config file

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2

**推奨タイトル**: `feat(0066): サーバが設定ファイルを読み、config init で雛形を作るようにする`

**レビュー観点**: LoadServer が 6 つのキーをファイルからだけ読み、環境の値が効かないこと / 振る舞いを変えるコミットと削除だけのコミットの分け方と coverage-compare の出力 / テストの補助がファイルの状態を setup の後に 1 回だけ書くこと / config init の終了コードと標準エラー

**実装モデル要件**: frontier-required

**判定理由**: ステップ 3-2 は振る舞いの切り替えと多数のテストの書き換えを 1 つのコミットで行う移行で、ステップ 4-1 は CLI とサーバの拒否を同時にやめる。mkplan のパネルモードの条件（移行・多数のテストの更新）に当たる

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: 拡張のビューアの案内（設計 8 章の 6）

#### ステップ 6-1: カタログの文

**変えるファイル:** `extension/src/core/i18n/en.ts`、`extension/src/core/i18n/ja.ts`、`extension/test/secretsAbsent.test.ts`

- [ ] `en.ts:27` と `ja.ts:23-24` の `setWebhookStep` を、`yt2column-server` の設定ファイルに `SLACK_WEBHOOK_URL` を書いて `yt2column-server` を起動し直す、という案内に変える。環境変数という語を使わず、設定ファイルのパスは書かない（設計 3.4）。文面は実装で決める。
- [ ] `secretsAbsent.test.ts:28` の注釈を、許す名前が設定ファイルのキーであることに合わせて直す。

**完了条件:** `make ext-check` が通る。

### フェーズ 7: 文書とガードテスト（設計 8 章の 7）

#### ステップ 7-1: 廃止した変数の記述の削除

**変えるファイル:** `README.md`、`README.en.md`、`docs/dev/guides/cli.md`、`docs/dev/project_overview.md`、`docs/translation_glossary.md`

**コミット:** 節の削除と、それを指すリンクの削除を 1 つのコミットにする（`TestReadmeLinks`）。

- [ ] `README.md` の「設定」の節の廃止した変数の段落（`:431-433`）、「廃止した LLM の変数からの移行」の節（`:467` から）、それを指すトラブルシューティングの項目（`:352`）を消す。`/mktrans README.md` で `README.en.md` に同期する。
- [ ] `cli.md:69-71`・`255`・`257`、`project_overview.md:161` の「次に述べる旧変数の検査だけは例外で」の部分と `:163`、`translation_glossary.md:159`・`186` の行を消す。
- [ ] `docs/tasks/0066_server_config_file/_checks/` に、merge-base と HEAD の文書を比べて、上の節・段落・用語集の行が HEAD で消え、ほかの見出しが同じ順で残っていることを確かめるスクリプトを置く（`diff` の検証。`requirements_process.md` §4）。PR-4 のブランチで 1 回実行し、出力と 2 つのコミットの ID を 7 章の AC-30・AC-33 の行に書く。

#### ステップ 7-2: README とほかの文書の書き換え

**変えるファイル:** `README.md`、`README.en.md`、`docs/dev/project_overview.md`、`docs/dev/security.md`、`docs/dev/production_profile.md`、`docs/dev/guides/development.md`、`docs/dev/guides/cli.md`、`docs/dev/conventions/package_reference.md`、`CLAUDE.md`、`docs/translation_glossary.md`、`cmd/yt2column/docs_test.go`、`internal/config/test_helpers.go`

**コミット:** 推奨構成と本番の構成のブロックの形の変更と、このステップの最後の項目（`checkExportBlock` の書き換え）を 1 つのコミットにする。

- [ ] `README.md` を、要件 AC-30・AC-31 の内容に書き換える。設定ファイルの節（パス・書式・検査・ACL・`config init` と終了コードの表・一時ファイルの接頭辞 `.config.env.tmp-`）と移行の手順を足し、クイックスタート・サーバの起動・設定・推奨構成のサーバの手順を、設定ファイルの形にする。`/mktrans README.md` で `README.en.md` に同期し、2 つを同じコミットにする。
- [ ] 要件 AC-33 に挙げた節を書き換える。`development.md` の「開発用に別のサーバを動かす」は、専用の `HOME` の下に設定ファイルを置いてから起動する手順にする。`package_reference.md` の `cmd/yt2column-server` と `internal/config` の行を直す。`CLAUDE.md` の Architecture Overview のサーバを支えるパッケージに `internal/privatefile` を足し、設定と秘密の記述を直す。`translation_glossary.md` の「設定エラー」（`:165`）と「設定」（`:185`）の定義を環境変数に限らない形にし、「設定ファイル」「雛形」を足す。
- [ ] `TestProductionProfileDocsContract` の `checkExportBlock`・`checkExportLines`・`exportLineRE` を、設定ファイルの行のブロックを読む形に変える。`internal/config/test_helpers.go` に、バイト列を書式の解析に通して受理したキーと値を返すアクセサを足す。ブロックがこれで受理されることを確かめ、短い `HOME` の下にファイルとして書いて `LoadServer` で読み込み、既定の構成と比べる（1.3）。

#### ステップ 7-3: 文書のガードテスト

**変えるファイル:** `cmd/yt2column-server/docs_test.go`、`cmd/yt2column/docs_test.go`、`internal/doctestutil/readme.go`、`internal/config/test_helpers.go`

- [ ] `TestServerDocsContract` に、設計 3.5 の表の値の照合を足す: 両 README の設定ファイルのパス（OS ごとの行）、`config init` の終了コードの表（`exitOK`・`exitFailure`・`exitUsage` の 3 つの行）、6 つのキーの名前、大きさの上限（`serverFileMaxSize` から作る文字列）、`security.md` §8.2 の設定ファイルのパス。`internal/config/test_helpers.go` に、設定ファイルのパスの `HOME` より下の部分、`serverFileKeys`、`serverFileMaxSize` を返すアクセサを足す。新しい見出しや表の列の場所は `internal/doctestutil/readme.go` に足す。
- [ ] 文書を書き換えた後に、`TestLLMDefaultsDocsContract`・`TestEffortValuesDocsContract`・`TestReadmeLinks`・`TestReadmeSyncMarker` と、`make ext-test`（README と `development.md` を読む拡張のテスト）が通ることを確かめる。

**完了条件:** 共通の完了条件、`make readme-sync-check`、`make ext-test`。

### PR-4 作成ポイント: extension message and documents

**対象ステップ**: 6-1 / 7-1 / 7-2 / 7-3

**推奨タイトル**: `docs(0066): 拡張の案内と文書を設定ファイルに合わせる`

**レビュー観点**: README の設定ファイルの節と移行の手順の正しさ / 推奨構成のブロックを実際の書式解析で確かめるガードテスト / 廃止した変数の記述の削除と _checks の結果 / README.en.md の同期

**実装モデル要件**: standard

**判定理由**: ガードテストの書き換えはあるが、競合する実装の案も高リスクの手順もない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン

### 3.1. マイルストーン

| マイルストーン | フェーズ | 成果物 |
|---|---|---|
| M1 | 1 | 検査して読み、安全に作る `internal/privatefile` |
| M2 | 2 | 設定ファイルのパスの規則・書式の解析・雛形・`InitServerFile`（まだ本番のコードからは呼ばれない） |
| M3 | 3〜5 | サーバが設定ファイルだけを読み、廃止した変数の検査がなくなり、`config init` で雛形を作れる |
| M4 | 6〜7 | 拡張の案内と文書が設定ファイルに合う |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 | `internal/privatefile`（`Read`・`Create`・`MkdirBelow`）とそのテスト | frontier-required |
| PR-2 | 2-1 / 2-2 / 2-3 | 設定ファイルのパスの規則、書式の解析、`FileError`、雛形、`InitServerFile` | frontier-recommended |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2 | サーバが設定ファイルを読む切り替え、廃止した変数の検査の削除、引数の振り分けと `config init` | frontier-required |
| PR-4 | 6-1 / 7-1 / 7-2 / 7-3 | 拡張の案内、README とほかの文書、文書のガードテスト | standard |

PR の分け方の理由は 1.3 の 4 つ目の決定にある。

## 4. テスト戦略

### 4.1. 単体テスト

-   `internal/privatefile` は、`t.TempDir` の下の実際のファイルで試す（ステップ 1-4）。タイミングに頼るテストは書かず、検査と読み込みが同じ記述子を使うことや、競合と失敗の経路は、テスト用の差し込み口で確かめる（設計 7.3、ステップ 1-1）。
-   `internal/config` は、書式の解析を表で試し、`LoadServer` は `udstest.Dir` の `HOME` に、`InitServerFile` は `t.TempDir` の `HOME` に実際のファイルを置いて試す（ステップ 2-1 から 2-3、3-2、3-3、4-1）。
-   拒否の文が値・行・未知のキーを含まないことは、表のすべての行について確かめる（ステップ 2-2、3-3）。

### 4.2. 結合テスト

-   `cmd/yt2column-server` の `run` を通すテスト（`TestStartupFailures`・`TestRunArgs`・`TestConfigInit`・`TestConfigInitThenStart`・`TestServerIgnoresEnvSettings`）と、子プロセスのテスト（`TestConfigInitStdout`、今の `signal_test.go`）と、ホストとサーバの結合のテスト（`relay_e2e_test.go`）は、テスト用の `HOME` の下に設定ファイルを置いて動かす（ステップ 3-2、3-4、5-2）。
-   LLM の API と Webhook は呼ばない。今のテストと同じく、偽の部品（`testDeps`）を使う。

### 4.3. 互換性のテスト

-   CLI の設定が変わらないこと（AC-10）は、`Load`・`LoadFor` の今のテストがそのまま通ることと、`TestLoadForIgnoresServerFile`（ステップ 3-3）、`TestLoadForIgnoresRetiredVars`（ステップ 4-1）で確かめる。
-   ホストが変わらないことは、`cmd/yt2column-nmhost` の今のテストがそのまま通ることで確かめる。
-   環境変数からの移行は互換の仕組みを持たない（要件 4.4）ので、移行のテストは置かない。

### 4.4. 権限と実行環境

-   root では結果が変わるテストは、`os.Geteuid() == 0` のときに `t.Skip` する。対象は、モード `0000`・探索できない親・読めないファイル・書けない親を使う場合で、ステップ 1-4、2-3、3-3、3-4、5-2 にある。CI は root でない利用者で動く。
-   権限を外したディレクトリは、用意の直後に、モードを戻す `t.Cleanup` を登録する。
-   umask を変えるテストと、`MkdirBelow`・`InitServerFile`・`runInit` を呼ぶテストには `t.Parallel` を付けない（1.3）。

## 5. リスク管理

| リスク | 影響 | 対策 |
|---|---|---|
| umask を変えている間に、同じプロセスのほかの処理がファイルを作る | そのファイルのモードが意図より狭くなる | `MkdirBelow` を本番で呼ぶのは `config init` だけで、並行する処理がない。注釈で並行して呼ばないことを示し、テストに `t.Parallel` を付けない（1.3、ステップ 1-1） |
| `LoadServer` の変更で、環境変数で設定を渡すテストが一度に止まる | フェーズ 3 のコミットが通らない | テストの補助とすべての呼び出し元の書き換えを、`LoadServer` の変更と同じコミットにする（ステップ 3-2）。呼び出し元は 1.4 に挙げた |
| 削除のコミットのカバレッジの比較が `STOP` か `FAIL` になる | テストの削除が確かめられない | 振る舞いを変えるコミット（ステップ 4-1）と削除だけのコミット（ステップ 3-1・4-2）を分ける。`STOP` か `FAIL` のときは削除の範囲を見直し、比較の出力をそのまま記録する |
| macOS と Linux で、OS が返すエラーや、モードとパスの長さの振る舞いが違う | 一方の OS でだけテストが通らない | テストは番兵と `errors.Is` で確かめ、OS のエラーの文を照合しない（`CLAUDE.md`「Error Testing」）。`HOME` は `udstest.Dir` にし（1.2）、特別なビットは付いたことを確かめる（ステップ 1-4）。CI は Linux で、macOS では開発の環境で `make test` を通す |
| PR-3 が大きい | レビューに時間がかかる | 削除のコミット・切り替えのコミット・新しいテストのコミットを分け、PR の説明でコミットごとの見どころを示す |
| PR-3 のマージから PR-4 のマージまでの間、`main` のサーバは設定ファイルだけを読むのに、README と拡張の案内は環境変数のままである | その間、案内に従って環境変数を設定してもサーバに効かない | 利用者はリポジトリの外にいない（要件 4.4）。PR-4 は PR-3 のマージの直後に進める。それまでは、本書と設計 3.2 で設定ファイルの形を確かめられる |
| 日程: PR-3 の切り替えのコミット（ステップ 3-2）が、書き換えるテストの多さで長引く | PR-3 とその後の PR が遅れる | 切り替えのコミットでは、書き換えだけを行い、新しいテスト（ステップ 3-3・3-4）を後のコミットに分ける。PR-1・PR-2 は PR-3 と独立にレビューとマージを進められる |

## 6. 実装チェックリスト

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4）
- [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 5-1 / 5-2）
- [ ] PR-4 マージ済み（対象ステップ: 6-1 / 7-1 / 7-2 / 7-3）
- [ ] すべての受け入れ基準の検証（7 章）が通った

## 7. 受け入れ基準の検証

種別は `test`（実行して誤った振る舞いで失敗する）、`static`（ガードテスト・`make` のターゲット）、`diff`（`_checks/` のスクリプトで merge-base と HEAD を比べる）、`manual`（PR のレビューでの読み取り）である。「実装」はコードを書くステップ、「テスト」はテストを書くステップである。

| AC | 検証 | 種別 | 実装 | テスト |
|---|---|---|---|---|
| AC-01・AC-02 | `internal/config/configpath_test.go::TestServerFileLocation` | test | 2-1 | 2-1 |
| AC-03 | `internal/config/configpath_test.go::TestServerFileLocation`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（`HOME` の誤りとソケットのパスの上限の行）、`cmd/yt2column-server/run_test.go::TestConfigInit`（`HOME` の誤りと長い `HOME`） | test | 2-1、3-2、5-1 | 2-1、3-4、5-2 |
| AC-04 | `internal/config/config_test.go::TestLoadServerReadsFile` | test | 3-2 | 3-3 |
| AC-05 | `internal/config/config_test.go::TestLoadServerValueRejections`、`internal/config/config_test.go::TestVarErrorSource`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（値の拒否の行） | test | 3-2 | 3-3、3-4 |
| AC-06 | `internal/config/config_test.go::TestLoadServerIgnoresEnvSettings`、`cmd/yt2column-server/run_test.go::TestServerIgnoresEnvSettings`（キーのないファイルと環境のキー） | test | 3-2 | 3-3、3-4 |
| AC-07 | `internal/config/config_test.go::TestLoadServerIgnoresEnvSettings`、`cmd/yt2column-server/run_test.go::TestServerIgnoresEnvSettings` | test | 3-2 | 3-3、3-4 |
| AC-09 | `internal/config/config_test.go::TestLoadHTTP2Debug`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（今の `GODEBUG` の行、`run_test.go:145`） | test | 3-2 | 3-3 |
| AC-10 | `internal/config/config_test.go::TestLoadForIgnoresServerFile`、`internal/config/config_test.go::TestLoadForIgnoresRetiredVars`、`cmd/yt2column/run_test.go::TestRunRejectsRetiredLLMVars` を書き換えたテスト、`Load`・`LoadFor` の今のテスト（`internal/config/config_test.go::TestLoadValid`・`TestLoadFor` ほか） | test | 4-1 | 3-3、4-1 |
| AC-11 | `internal/config/serverfile_test.go::TestParseServerFileAccepts` | test | 2-2 | 2-2 |
| AC-11a | `internal/config/serverfile_test.go::TestParseServerFileAccepts`（空の値の受理）、`internal/config/config_test.go::TestLoadServerEmptyValues` | test | 2-2、3-2 | 2-2、3-3 |
| AC-12 | `internal/config/serverfile_test.go::TestParseServerFileRejects`、`internal/config/config_test.go::TestLoadServerFileRejections`（大きさの上限の境界を含む）、`cmd/yt2column-server/run_test.go::TestStartupFailures`（書式の誤りと大きさの行、ファイルが変わらないこと） | test | 2-2、3-2 | 2-2、3-3、3-4 |
| AC-13 | `internal/config/serverfile_test.go::TestParseServerFileRejects`（文の内容）・`TestParseServerFileOverflow`・`TestParseServerFileDuplicate`、`cmd/yt2column-server/run_test.go::TestFormatRejectionLines` | test | 2-2 | 2-2、3-4 |
| AC-14 | `internal/privatefile/privatefile_test.go::TestRead`、`internal/config/serverfile_test.go::TestFileErrorMapping`（パスと `yt2column-server config init`）、`internal/config/config_test.go::TestLoadServerFileRejections`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（ファイルがない行） | test | 1-2、2-3、3-2 | 1-4、2-3、3-3、3-4 |
| AC-15・AC-16 | `internal/privatefile/privatefile_test.go::TestRead`（シンボリックリンク 2 種、ディレクトリ、FIFO、ソケット）・`TestReadChecksDescriptor`（シンボリックリンクと FIFO への差し替え）、`internal/config/serverfile_test.go::TestFileErrorMapping`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（シンボリックリンクとディレクトリの行） | test | 1-2、2-3 | 1-4、2-3、3-4 |
| AC-17 | `internal/privatefile/privatefile_test.go::TestOwnerAndModeCheck`、`internal/config/serverfile_test.go::TestFileErrorMapping`（所有者が違うことを示す文） | test | 1-1、2-3 | 1-4、2-3 |
| AC-18 | `internal/privatefile/privatefile_test.go::TestRead`（各モード）、`cmd/yt2column-server/run_test.go::TestStartupFailures`（モード `0644` の行）、`cmd/yt2column-server/run_test.go::TestModeRejectionChmodCommand` | test | 1-2、2-3 | 1-4、3-4 |
| AC-19 | `internal/privatefile/privatefile_test.go::TestReadUnreadable`、`internal/config/config_test.go::TestLoadServerFileRejections`、`cmd/yt2column-server/run_test.go::TestStartupFailures`（読めない行） | test | 1-2、2-3 | 1-4、3-3、3-4 |
| AC-19a | `internal/privatefile/privatefile_test.go::TestReadChecksDescriptor` | test | 1-2 | 1-4 |
| AC-20 | `cmd/yt2column-server/run_test.go::TestStartupFailures`（AC-03・AC-05・AC-12・AC-14 から AC-19 の各行で、ソケットのディレクトリ・起動のロック・ソケットがなく、設定ファイルと親が変わらないこと。AC-19a の止まり方は `Read` の中で AC-15 から AC-18 と同じ拒否になるので、`TestReadChecksDescriptor` で確かめる） | test | 3-2 | 3-4 |
| AC-21 | `internal/privatefile/privatefile_test.go::TestCreate`（所有者のビットを落とす umask を含む）、`internal/config/serverfile_test.go::TestInitServerFile`、`cmd/yt2column-server/run_test.go::TestConfigInit`、`cmd/yt2column-server/run_test.go::TestConfigInitStdout` | test | 1-3、2-3、5-1 | 1-4、2-3、5-2 |
| AC-22 | `internal/config/serverfile_test.go::TestServerFileTemplate`、`internal/config/serverfile_test.go::TestInitServerFile`（`HOME` だけを読む） | test | 2-3 | 2-3 |
| AC-23 | `internal/privatefile/privatefile_test.go::TestMkdirBelow`・`TestMkdirBelowSwap`、`cmd/yt2column-server/run_test.go::TestConfigInit`（基点がない場合） | test | 1-3、5-1 | 1-4、5-2 |
| AC-24 | `internal/privatefile/privatefile_test.go::TestCreateExisting`・`TestCreateRaces`、`cmd/yt2column-server/run_test.go::TestConfigInit`（すでにある場合） | test | 1-3、5-1 | 1-4、5-2 |
| AC-25 | `internal/privatefile/privatefile_test.go::TestCreateFailureLeavesNothing`、`internal/config/serverfile_test.go::TestInitServerFile`（作った親が残る）、`cmd/yt2column-server/run_test.go::TestConfigInit`（作れない場合） | test | 1-3、5-1 | 1-4、2-3、5-2 |
| AC-26 | `internal/config/serverfile_test.go::TestServerFileTemplate`、`cmd/yt2column-server/run_test.go::TestConfigInitThenStart` | test | 2-3、5-1 | 2-3、5-2 |
| AC-27・AC-28・AC-29 | `cmd/yt2column-server/run_test.go::TestRunArgs` | test | 5-1 | 5-2 |
| AC-30・AC-31 | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（パス・終了コード・キー・上限）、`cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`（推奨構成のブロック）、`cmd/yt2column/docs_test.go::TestReadmeLinks`、`docs/tasks/0066_server_config_file/_checks/` の削除の確かめ（実行の結果と 2 つのコミットの ID は PR-4 で記入）、文の内容は PR のレビューで読む | static・diff・manual | 7-1、7-2 | 7-1、7-2、7-3 |
| AC-32 | `cmd/yt2column/readme_sync_test.go::TestReadmeSyncMarker`（`make readme-sync-check`）、`make ext-test` | static | 7-1、7-2 | - |
| AC-33 | `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`（`production_profile.md` のブロック）、`cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`security.md` §8.2 のパス）、`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`、`docs/tasks/0066_server_config_file/_checks/` の削除の確かめ、文の内容は PR のレビューで読む | static・diff・manual | 1-1、7-1、7-2 | 7-1、7-2、7-3 |
| AC-34 | `cmd/yt2column-server/docs_test.go::TestServerDocsContract` | static | 7-3 | 7-3 |
| AC-35 | `make ext-check`（`extension/test/secretsAbsent.test.ts` を含む）、案内の文の内容は PR のレビューで読む | static・manual | 6-1 | - |

要件 4.2「拒否の分類」の番兵ごとの `errors.Is` は、`TestFileErrorMapping`（ステップ 2-3）、`TestLoadServerFileRejections`（ステップ 3-3）、`TestConfigInit`（ステップ 5-2）で確かめる。

## 8. 成功基準

-   **機能:** 7 章のすべての検証が通る。
-   **品質:** `make test`・`make lint`・`make deadcode`・`make build`・`make ext-check` が通る。テストを削除したコミットは、`make coverage-compare` が `PASS` で、その出力がコミットメッセージと PR の説明にある。
-   **セキュリティ:** 設計 5.1 の脅威の表のそれぞれに、7 章の検証がある。拒否の文と雛形が値・秘密を含まないことを、テストで確かめている。
-   **文書:** 要件 AC-30 から AC-33 の文書が設定ファイルに合い、`README.en.md` が同期している。

## 9. 次のステップ

-   本書の承認の後、`/runplan 0066` で PR-1 から順に実装する。
-   最後の PR のマージの後、issue #576 を閉じる（`/runpr` が案内する）。
-   実際の `HOME` で、`config init` から起動までを 1 度確かめる（`brew services` での常駐は #578 で扱う）。

## 変更履歴

- 2026-11-08 承認（isseis、#711）
