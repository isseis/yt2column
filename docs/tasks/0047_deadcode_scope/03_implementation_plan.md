# 実装計画書：`make deadcode` の解析の範囲をモジュール全体にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的と原則

[要件定義書](01_requirements.md)（以下、要件書）の AC-01〜AC-15 を、[設計書](02_architecture.md)の 1.1・3.2・4.2・7 章に従って実装する。本書の用語（レシピ、`deadcode` コマンド、出力、合否の判定、到達の判断、スタブ、フィクスチャ）は、設計書冒頭の定義に従う。要件書 6 章の用語（到達できない関数、届く、報告、意図した足場、完了条件）も同じ意味で使う。

- 本番の Go のコードは変えない。変えるのは `Makefile` の `deadcode` ターゲット、`CLAUDE.md` の 1 行、`.claude/commands/runplan.md` の step 6 だけである。ほかには、`cmd/yt2column` のテストとテスト補助を加える（設計書 3.4）。
- テストは `make` の子プロセスを `childEnvAllowlist` から作った環境で動かす。LLM の API・Webhook・`yt-dlp` は呼ばない（`CLAUDE.md`「Testing Strategy」）。
- Go のコメント・識別子・文字列は英語とし、受け入れ基準との対応は本書 9 章に記録する。ソースのコメントに `AC-NN` を書かない（`requirements_process.md` §4）。
- テストを加えたら、そのテストが対象とする仕組みを壊して失敗することを確認し、そのことをコミットメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。

### 1.2. 計画の際に決めたこと（2026-10-28）

- **実装は 1 つの PR にまとめる（人間のレビュアーの選択）。** レシピとそれを確かめる 2 種類のテスト（スタブとフィクスチャ）は、同じ契約（設計書 3.2）を確かめる。一緒に読めば一度で検証できる。全体の差分も小さい。AC-01 の記録はレシピが入ってからでないと取れない。
- **フェーズは設計書 8 章の 4 項目の順に、そのまま 4 つにする。** PR は 1 つなので、PR の作成ポイントはフェーズ 4 の後に 1 つだけ置く。
- **子プロセスの環境を作る処理を、`childEnv` から関数に切り出して共用する。** `childEnv`（`cmd/yt2column/test_helpers.go:308-318`）は、許可リストの変数に続けて `YT2COLUMN_TEST_CLI_MODE` を加える。この変数は `make` の子には要らない。許可リストを読む部分だけを新しい関数に切り出し、`childEnv` と `deadcode` のテストの両方から呼ぶ。許可リストを二重に書かないためである（設計書 1.2「子プロセスの環境は、同じパッケージの既存の許可リストから作る」）。
- **一時ファイルの後片付けもスタブのテストで確かめる。** 設計書 3.3 は、レシピが作る一時ファイルを、2 つ目を作れなかったときも含めて消すと定める。設計書 7.1 の表にはこのケースがない。そこで、子の `TMPDIR` を空のディレクトリにし、各ケースの後にそれが空であることを確かめる。2 つ目の `mktemp` だけが失敗するケースも加える。
- **スタブは、空白を含むディレクトリに置く。** 設計書 1.2 は、`DEADCODECMD` を引用符で囲み、空白を含むパスでも動くようにすると定める。そのことを、すべてのスタブのケースで確かめる。
- **`Makefile` を変えたときに、`go test` のキャッシュが古い PASS を返さないようにする。** `make` が読むファイルは、テストのプロセスが開いたファイルとして記録されない。そのため、`Makefile` だけを変えた後の `go test` は、キャッシュの結果を返しうる。新しいテストは、`make` を動かす前に、テストのプロセス自身で `Makefile` を開く（`os.Stat` か読み込み）。Go のテストのキャッシュは、テストが開いたモジュールの中のファイルが変わると、キャッシュの結果を使わないからである（`go help test` の「Tests that open files within the package's module ... only match future runs in which the files and environment variables are unchanged」）。
- **設計書 7.2「環境」の記述を、編集上の訂正として直した。** `go env -w` で書いた設定は、`HOME` から決まる設定ファイルから読まれる。そのため、子の `go list` もこの設定を受け継ぐ（本書の作成時、`go env GOENV` が `$HOME/Library/Application Support/go/env` を示すことを確かめた）。設計は変わらないので、設計書の状態は `approved` のまま、Comments に記録した。

### 1.3. 既存コード調査結果

調査の基点はコミット `5dbf3a484ca366b7a99e4a92ef79f72b52bb06b8`。この節の `file:line` はその版を指す。設計書が確かめたコミット `9dbc1f4` から基点までの間に、`docs/tasks/` の外のファイルの変更はない（本書の作成時、`git diff --stat 9dbc1f4 HEAD -- ':!docs/tasks'` の出力が空であることを確かめた）。そのため、設計書の `file:line` も基点の版で有効である。

| 対象 | 現状と必要な変更 | 作業 |
|---|---|---|
| `deadcode` ターゲット | `Makefile:244-245` が `deadcode -test -tags test $(MAIN_PKG)` を実行する。行頭に `@` はない。`DEADCODECMD` を加え、設計書 1.1・3.2・4.2 のレシピに置き換える。`MAIN_PKG`（`Makefile:14`）は `build`（`:61`）が使うので残す。`.PHONY`（`:53`）には `deadcode` がすでにある | 1-2 |
| 倣う既存の形 | `GOFUMPTCMD=gofumpt`（`Makefile:11`）、`check_gofumpt`（`:20-26`）、`format_files_from_list` の `mktemp` と `trap ... EXIT`（`:31-33`） | 1-2 |
| `Makefile` のテストの置き場所 | `cmd/yt2column/makefile_test.go` が `Makefile` のターゲットを確かめる。`repositoryRoot`（`:27`）を再利用する。`deadcode` を確かめるテストはない | 1-1、2-1 |
| 子の環境 | `childEnvAllowlist`（`cmd/yt2column/test_helpers.go:79-82`）と `childEnv`（`:308-318`）。`childEnv` は `YT2COLUMN_TEST_CLI_MODE` を加える。`TestMain`（`cmd/yt2column/main_test.go:22-46`）がプロキシの変数を閉じたポートに向ける。`internal/maketestutil/make.go:40-43` にも同じ内容の許可リストがある。ただし `RunTarget` は `make` が失敗するとテストを失敗させ、`GOTEST` しか差し替えないので、使わない（設計書 1.2） | 1-1 |
| スタブの書き方の手本 | `maketestutil.RunTarget` は、スタブを `t.TempDir()` に書き、`//nolint:gosec` を `os.WriteFile`（`internal/maketestutil/make.go:121`）と `exec.Command`（`:129`）の行だけに付ける。新しいテストも同じく、行ごとに理由を書いて付ける | 1-1、2-1 |
| `make -n` の検査 | `extension/test/repository.test.ts:229-258` は、`node` と `npm` のない `PATH` で `make -n build test lint deadcode` を実行し、終了コード 0、標準エラーが空、出力に `node`・`npm`・`npx` がないことを確かめる。設計書 3.4 のとおり、変えずに通る。`make ext-test` で確かめる | 4-1 |
| 文書 | `CLAUDE.md:80` は `` - `make deadcode` - Report unreachable functions `` だけである。`.claude/commands/runplan.md:117` は「Remove functions made unreachable by this phase group; keep intentional scaffolding for future phases or tasks.」と書き、足場の扱いと `make deadcode` が通ることを書いていない。`CLAUDE.md` を読む Go のガード（`cmd/yt2column/extension_docs_test.go:26` の `claudePath` など）は `ext-` のターゲットの行と見出しを読み、`:80` の行は読まない。`runplan.md` を読むテストはない（基点の版で、`*_test.go` と `*.test.ts` に `runplan.md` を含むものはない） | 3-1 |
| 変えないもの | `.claude/commands/_context.md:168` と `.claude/commands/weakreview.md:8` は `make deadcode` の名前だけを挙げる（要件書 2.3）。これまでのタスクの文書も変えない（要件書 2.3） | - |

具象型による振る舞いの選択と、名前の変更は、本タスクにない。CI（`.github/workflows/ci.yml`）は `deadcode` コマンドを入れないので、2-1 のテストは CI ではスキップされる（設計書 1.2）。1-1 のテストは CI の `test` と `doc-guard` のジョブで走る（どちらも `./cmd/yt2column` を含む）。

**外部の前提の確かめ（本書の作成時、手元の macOS、GNU Make 3.81、`deadcode` は `golang.org/x/tools` v0.47.0 を go1.27.1 でビルドしたもの）:**

- 設計書 7.2 のフィクスチャの構成を一時ディレクトリに作り、`deadcode` コマンドを直接実行した。`-test -tags test,integration ./...` の報告は 0 件だった。`-test` を外すと `Helper`・`OnlyTest`・`OnlyIntegration` が、`integration` を外すと `OnlyIntegration` が、今のレシピの形（`-test -tags test ./cmd/cli`）では `OnlySrv`・`OnlyTest`・`OnlyIntegration` が、`cmd/srv` を除くと `OnlySrv` が報告された。設計書 7.2「入力の分離」の前提のとおりである。
- GNU Make 3.81 で `make -s -C <dir> -f <絶対パスの Makefile> <target>` を実行すると、「Entering directory」の行は出なかった。Linux の GNU Make 4.x では `-s` がこの行を止める（設計書 7.2）。

### 1.4. 申し送りの扱い

`design_handoff.md` の H-01〜H-04 は、設計書の付録 A に記録されている。`implementation_handoff.md` はない。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: 合否の判定のテストとレシピ（設計書 8 章の 1）

#### ステップ 1-1: 子の環境の関数と、合否の判定のテストを書く

**変えるファイル**: `cmd/yt2column/test_helpers.go`、`cmd/yt2column/deadcode_test.go`（新設、`//go:build test`）

- [ ] `childEnv` から、`childEnvAllowlist` の変数を読む部分を関数に切り出す。`childEnv` はその関数を呼ぶ形にし、振る舞いは変えない（1.2）。
- [ ] `deadcode_test.go` に、`make deadcode DEADCODECMD=<スタブ>` を `repositoryRoot` で実行するテスト `TestMakeDeadcodeVerdict` を加える。ケースは設計書 7.1 の表のすべての行と、1.2 で加えたケース（2 つ目の `mktemp` だけが失敗する）である。各ケースで、`make` の終了コード（0 か 0 以外か）、標準出力と標準エラーのそれぞれの中身、スタブが実行されたかどうかと受け取った引数を確かめる。
- [ ] スタブは、テストが決めたバイト列を標準出力と標準エラーに書き、決めた終了コードで終わり、受け取った引数をファイルに記録するシェルスクリプトとする（設計書 7.1）。空白を含むディレクトリに置く（1.2）。
- [ ] 子の環境は、切り出した関数の結果に、ケースごとの `PATH` と `TMPDIR` を加えて作る。すべての `make` の起動に同じ作り方を使う。`TMPDIR` はケースごとの空のディレクトリとし、ケースの後に空であることを確かめる（1.2）。
- [ ] 成功のケースは `-s` を付けずに `make` を実行し、標準出力も標準エラーも空であることを確かめる（設計書 1.2「成功したときは何も出さない」）。
- [ ] `mktemp` が失敗するケースは、失敗する `mktemp` を `PATH` の先頭に置く。2 つ目だけが失敗するケースでは、1 回目は本物の `mktemp`（`PATH` を変える前に `exec.LookPath` で絶対パスを得る）に任せる。
- [ ] 「見つからない」ケースは、`make` と `sh` へのシンボリックリンクだけを置いたディレクトリを `PATH` にして、`make` を絶対パスで実行する（設計書 7.1、`repository.test.ts:229-258` と同じ形）。
- [ ] テストのプロセスで `Makefile` を開いてから `make` を実行する（1.2）。
- [ ] スタブを書く `os.WriteFile` と、`exec.Command` の行に、`gosec` の指摘があれば、その行だけに理由を付けた `//nolint:gosec` を置く（`internal/maketestutil/make.go:121`・`:129` と同じ形）。

**完了の条件**: `go test -tags test ./cmd/yt2column -run 'TestMakeDeadcodeVerdict' -count=1` を実行し、今のレシピで失敗することを確かめる（ステップ 1-2 の前の状態）。

#### ステップ 1-2: `Makefile` のレシピを変える

**変えるファイル**: `Makefile`

- [ ] `GOFUMPTCMD` の近くに `DEADCODECMD=deadcode` を `=` で加える（設計書 1.2・3.2）。
- [ ] `deadcode` ターゲットのレシピを、設計書 1.1・3.2・4.2・6 章のとおりに書き換える。表示する行は設計書 4.2 の文面とする。`$(shell ...)` など、`Makefile` を読むときに評価される式を加えない（設計書 3.2）。レシピに、`check_gofumpt` と同じ程度のコメントを付ける。
- [ ] `TestMakeDeadcodeVerdict` を壊して確かめる。少なくとも次の 3 つで、それぞれ対応するケースが失敗することを確かめ、結果をコミットメッセージに書く。
  - 合否の判定を終了コードだけにする（出力のバイト数を見ない）。報告あり・標準エラーだけ・改行だけのケースが失敗する。
  - 合否の判定をバイト数だけにする（終了コードを見ない）。終了コードだけのケースが失敗する。
  - 引数から `-test` を外す。引数のケースが失敗する。
- [ ] 理由の行を選ぶ順を逆にしたとき（出力があれば終了コードより優先する）に「終了コードと報告」のケースが失敗することと、後片付けの `trap` を外したときに `TMPDIR` を確かめるケースが失敗することも確かめ、コミットメッセージに書く。

**完了の条件**: `go test -tags test ./cmd/yt2column -run 'TestMakeDeadcodeVerdict' -count=1` が通る。`make lint` が通る。

### フェーズ 2: フィクスチャと解析の範囲のテスト（設計書 8 章の 2）

#### ステップ 2-1: 本物の `deadcode` コマンドで解析の範囲を確かめるテストを書く

**変えるファイル**: `cmd/yt2column/deadcode_test.go`

- [ ] テスト `TestMakeDeadcodeScope` を加える。`exec.LookPath("deadcode")` で見つからなければ、理由を示して `t.Skip` する（設計書 1.2）。スキップの判断は `PATH` の検索だけで行い、環境変数を読まない。
- [ ] フィクスチャを、テストのコードの中の内容から、実行のたびに `t.TempDir()` の下に書き出す（設計書 1.2・7.2 のパッケージの構成）。`go.mod` は `require` と `toolchain` の行を持たず、`go` の行はリポジトリの `go.mod` の `go` の行以下とする。
- [ ] `make -s -C <フィクスチャ> -f <リポジトリの Makefile の絶対パス> deadcode` を、ステップ 1-1 の関数から作った環境で実行する。前提の確認のための `deadcode` コマンドの直接の実行にも、同じ環境を使う（設計書 7.2「環境」）。
- [ ] 設計書 7.2 の順に確かめる。(1) 書き出したままのフィクスチャで、`make` が終了コード 0 で終わり、何も出力しない。(2) 「入力の分離」の 4 つの前提（起点を除いた直接の実行で、その関数が報告される）。(3) 「届く経路」の表の 4 つの関数が `make deadcode` で報告されない。これは (1) で示されるが、関数ごとにサブテストで名前を挙げる。(4) 「報告されるべき関数」の 3 つの行を 1 つずつ加え、`make` が 0 以外で終わり、報告に加えた関数のファイル、行、名前が含まれる。(5) コンパイルできないファイルを加え、`make` が 0 以外で終わり、`deadcode` コマンドのエラーが標準エラーに出る。(4)・(5) は、ケースごとに新しく書き出したフィクスチャを使う。
- [ ] 失敗のメッセージに、`go version -m` で読んだ `deadcode` コマンドのモジュールの版と、設計書が前提にした版（v0.47.0）を含める（設計書 7.2）。
- [ ] 壊して確かめる。`Makefile` の引数から `-test` を外す、`integration` を外す、`./...` を `$(MAIN_PKG)` に戻す、の 3 つで、それぞれ (3) の対応する関数のサブテストが失敗することを確かめ、結果をコミットメッセージに書く。

**完了の条件**: `go test -tags test ./cmd/yt2column -run 'TestMakeDeadcodeScope' -v -count=1` が、手元でスキップされずに通る。`make lint` が通る。

### フェーズ 3: 文書（設計書 8 章の 3）

#### ステップ 3-1: `CLAUDE.md` と `runplan.md` の step 6 を書き換える

**変えるファイル**: `CLAUDE.md`、`.claude/commands/runplan.md`

- [ ] `CLAUDE.md:80` の `` - `make deadcode` - Report unreachable functions `` を、次の 2 点を述べる 1 項目に書き換える（AC-14）。解析の範囲（モジュール全体と、そのテスト。ビルドタグ `test`・`integration`）と、報告が 1 件以上あれば失敗すること。ほかの項目と同じく英語で、1〜2 行に収める。
- [ ] `.claude/commands/runplan.md:117` の step 6 を、次の 2 点を加えて書き換える（AC-15）。`make deadcode` が通ること。意図した足場を残すならテストから届くようにし、届かないなら、使う PR まで加えないこと。このフェーズ群で到達できなくなった関数を削除することと、変えたら `make fmt && make test && make lint` を実行してコミットすることは残す。

**完了の条件**: `make test` が通る（`CLAUDE.md` を読むガードを含む）。

### フェーズ 4: 完了の確認と記録（設計書 8 章の 4）

#### ステップ 4-1: グリーンゲートと完了の条件を実行し、結果を記録する

**変えるファイル**: `docs/tasks/0047_deadcode_scope/03_implementation_plan.md`（本書の 9 章）

- [ ] `make fmt`、`make test`、`make lint` を実行し、通ることを確かめる。
- [ ] `Makefile` を変えたので、`make ext-test` を実行し、`repository.test.ts` の `make -n` の検査が通ることを確かめる（設計書 7.3）。
- [ ] このリポジトリで `make deadcode` を変数の指定なしに実行する。実行したコマンドとその終了コード、`command -v deadcode` の結果、`go version -m "$(command -v deadcode)"` の結果（モジュールの版と、ビルドした Go の版）、実行したコミットを、本書の 9 章の AC-01 の行の下に記録する（設計書 7.3）。
- [ ] `go test -tags test ./cmd/yt2column -run 'TestMakeDeadcodeScope' -v -count=1` を実行し、スキップされずに PASS したことを、出力の要約（各サブテストの PASS の行）と実行したコミットとともに、本書の 9 章に記録する（設計書 7.3）。

**完了の条件**: グリーンゲート（`make test && make lint`）と `make ext-test` が通り、上の 2 つの記録が本書にある。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序と前提

| 本書のフェーズ | 設計書 8 章の項目 |
|---|---|
| フェーズ 1 | 1. 合否の判定のテストとスタブを書き、`Makefile` のレシピと `DEADCODECMD` を変える |
| フェーズ 2 | 2. フィクスチャと解析の範囲のテストを書く |
| フェーズ 3 | 3. `CLAUDE.md` と `runplan.md` の step 6 を直す |
| フェーズ 4 | 4. `make deadcode`、解析の範囲のテストの単独の実行、`make test`、`make lint`、`make ext-test` を実行し、記録する |

- ステップ 1-1 のテストは、ステップ 1-2 の前には失敗する。リポジトリの pre-commit フックが全テストを実行する場合は、1-1 と 1-2 を 1 つのコミットにまとめ、壊し方の確認の結果をコミットメッセージに書く。
- 2-1 は 1-2 のレシピを前提にする。3-1 はほかのステップと独立している。4-1 はすべてのステップの後に行う。

### 3.2. マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（PR のマージ） | モジュール全体を解析し、報告があれば失敗する `make deadcode`。そのテスト、文書、AC-01 と解析の範囲のテストの記録。本タスクの完了 |

## 4. テスト戦略 (Test Strategy)

### 4.1. 検証の分担

| 層 | 確かめること | 走る場所 | ステップ |
|---|---|---|---|
| スタブのテスト（`TestMakeDeadcodeVerdict`） | 合否の判定、表示、引数、`deadcode` コマンドが見つからないとき、一時ファイルを作れないときと後片付け | 手元と CI（`test`・`doc-guard` のジョブ） | 1-1 |
| フィクスチャのテスト（`TestMakeDeadcodeScope`） | 解析の範囲（どの起点から届く関数を報告しないか、どの関数を報告するか）、本物の報告と解析の失敗での終了コード | 手元だけ（CI ではスキップ） | 2-1 |
| 既存の `make -n` の検査 | `deadcode` ターゲットが `make -n` で何も実行せず、`node` を要らないこと | 手元と CI（`make ext-test`） | 4-1 |
| 完了の記録 | このリポジトリで報告が 0 件であること（AC-01）、フィクスチャのテストがスキップされずに通ったこと | 手元で 1 回 | 4-1 |
| PR のレビュー | `CLAUDE.md` と `runplan.md` の文面（AC-14・AC-15） | PR | 3-1 |

単体のテストの対象の Go のコードはない。互換性は、CI（Linux、GNU Make 4.x、`/bin/sh` は dash）と手元（macOS、GNU Make 3.81）の両方でスタブのテストが走ることで確かめる（要件書 4.4）。

### 4.2. 実行範囲と外部の前提

- テストは `-tags test` で作る。新しいファイルは `cmd/yt2column` のほかのテストと同じく `//go:build test` とする。`make lint` は `--build-tags test,integration` で解析する（`Makefile:10`）。
- テストが使う外部のコマンドは `make`、`sh`、`mktemp`、`go`（フィクスチャのテストだけ）、`deadcode`（同）である。どれも、開発環境と CI の `ubuntu-latest` にある（`deadcode` は手元だけ）。新しい道具は加えない。
- `go test -count=1` で、キャッシュの結果を使わずに実行する。

## 5. 横断検索 (Cross-Search Checklist)

名前の変更や削除はない。`MAIN_PKG` は残す（1.3）。`deadcode` の振る舞いを述べる文書は `CLAUDE.md:80` と `runplan.md:117` だけで、ほかの記述は名前を挙げるだけである（1.3）。そのため、`make lint` と `make test` で見つからないものとして検索する項目はない。文面の整合は、9 章の `manual` で確かめる。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| フィクスチャのテストが CI ではスキップされ、レシピの解析の範囲が後で壊れても CI が気づかない | 解析の範囲の退行を見逃す | 引数はスタブのテストが CI で確かめる。手元で `deadcode` コマンドがあれば、`make test` がフィクスチャのテストを走らせる。本タスクでは、スキップされずに通った記録を残す（4-1） |
| `deadcode` コマンドの版で到達の判断が変わる | 手元でフィクスチャのテストが失敗する | 失敗のメッセージに使った版と前提の版を出す（2-1、設計書 9 章） |
| 開発者が `go env -w` で書いた設定（`GOFLAGS` など）が、子の `go list` に効く | フィクスチャのテストが手元の設定によって失敗しうる | 設計書 7.2 に記録した（1.2）。失敗のメッセージに `make` と `deadcode` コマンドの出力を含めるので、原因を読み取れる |
| テレメトリなどが解析の成功のときに標準エラーへ書く | `make deadcode` が失敗する | 要件書 F-002 と設計書付録 A の H-01 のとおり、失敗として扱う |
| GNU Make 3.81 と 4.x で表示が違う | スタブのテストが一方の環境だけで失敗する | 成功のケースは、`-C` を使わない実行で何も出ないことを確かめる。失敗のケースは、期待する行を含むかどうかで確かめる。CI（4.x）と手元（3.81）の両方で走らせる |
| 書き込みのエラーを `deadcode` コマンドが無視する | 一時ファイルの置き場所が満杯のとき、`make deadcode` が成功してしまう | 設計書 4.3 で受け入れたリスクである。本書では扱わない |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（1-1 / 1-2）
- [ ] フェーズ 2 完了（2-1）
- [ ] フェーズ 3 完了（3-1）
- [ ] フェーズ 4 完了（4-1）、9 章の記録を書いた

## 8. 成功基準 (Success Criteria)

- 機能: 要件書の AC-01〜AC-15 のすべてが、9 章の検証で確かめられている。
- 品質: グリーンゲートと `make ext-test` が通る。追加したテストの壊し方の確認が、コミットメッセージに記録されている。
- セキュリティ: N/A（要件書 4.2、設計書 5 章）。
- 文書: `CLAUDE.md` と `runplan.md` の step 6 が更新され、9 章に AC-01 とフィクスチャのテストの記録がある。

## 9. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストの名前は本書で決めたものである。実装で変えたときは、この表に書き戻す。`TestMakeDeadcodeScope` は CI ではスキップされるので、4-1 の記録を合わせて検証とする。

| AC | 種別 | 検証 | 実装 |
|---|---|---|---|
| AC-01 | static | `make deadcode` をこのリポジトリで実行し、結果を下に記録する（4-1） | 1-2 |
| AC-02 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeScope`（`OnlySrv` が報告されないことと、今のレシピの形と `cmd/srv` を除いた解析で報告されること） | 1-2 |
| AC-03 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeScope`（`Helper`） | 1-2 |
| AC-04 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeScope`（`OnlyTest`） | 1-2 |
| AC-05 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeScope`（`internal/lib` に加えた関数） | 1-2 |
| AC-06 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeScope`（`cmd/srv` に加えた関数） | 1-2 |
| AC-07 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeScope`（ビルドタグ `integration` のファイルに加えた関数） | 1-2 |
| AC-08 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeScope`（`OnlyIntegration`） | 1-2 |
| AC-09 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeScope`（`OnlySrv`。`Makefile` はフィクスチャの `main` パッケージの名前を持たない） | 1-2 |
| AC-10 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeVerdict`（報告なし）、`TestMakeDeadcodeScope`（書き出したままのフィクスチャ） | 1-2 |
| AC-11 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeVerdict`（報告あり）、`TestMakeDeadcodeScope`（報告されるべき関数） | 1-2 |
| AC-12 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeVerdict`（解析の失敗）、`TestMakeDeadcodeScope`（コンパイルできないファイル） | 1-2 |
| AC-13 | test | `cmd/yt2column/deadcode_test.go::TestMakeDeadcodeVerdict`（見つからない） | 1-2 |
| AC-14 | manual | `CLAUDE.md` の「Code Quality」の `make deadcode` の項目を PR のレビューで読む | 3-1 |
| AC-15 | manual | `.claude/commands/runplan.md` の step 6 を PR のレビューで読む | 3-1 |

**AC-01 の記録（4-1 で書く）:** 未実施。

**`TestMakeDeadcodeScope` の記録（4-1 で書く）:** 未実施。

## 10. 次のステップ (Next Steps)

- 本書の承認の後、`/runplan 0047` で実装する。
- PR のマージの後、`/runpr` の案内に従い、タスクの issue #361 を閉じる。
