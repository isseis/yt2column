# 実装計画書：テストの削除の前後でカバレッジを関数ごとに比べる道具（Go と拡張）

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-02 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.2」などは設計書の節を指す。用語は要件書 6 章と設計書の冒頭の定義（チェックアウト、関数表、行の対応、範囲の補助、作業用のディレクトリ）に従う。既存のコードについての記述は、コミット `a749c29b` で確かめた。設計書が行番号を引いたコミット `4bf1842f` から `a749c29b` までの間に、`Makefile`・`CLAUDE.md`・`cmd/`・`internal/`・`extension/package.json`・`extension/tsconfig.json`・`.golangci.yml` は変わっていない（`git diff --stat 4bf1842f a749c29b -- <これらのパス>` の出力が空）。そのため、設計書の行番号はそのまま使える。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`make coverage-compare BASE=<commit>`（Go）と `make ext-coverage-compare BASE=<commit>`（拡張）を加える。どちらも、BASE と HEAD のチェックアウトでテストの一式を実行してカバレッジを取り、関数ごとに比べ、合格・不合格・停止を決まった最後の行で示す。あわせて、CLAUDE.md の「Deleting a test is a claim that must be checked」をこの道具を使う手順に書き換える。設計は設計書 1〜6 章のとおりである。

### 1.2. 実装の原則

-   設計書 8 章の手順 1〜6 の順で実装する。本書のフェーズ 1〜6 は、手順 1〜6 にそのまま対応する。ただし、手順 4 の Makefile の対象と結びつく文書とガードの変更（手順 5 の一部）は、同じ PR に入れないと `make test` が通らないので、フェーズ 5 に移す（1.3）。
-   型・関数・判定の規則・受け入れる形は、設計書 3〜4 章で決まっている。本書は、それをどのファイルに置き、どのテストで確かめるかと、作業の順を定める。設計の内容は繰り返さず、節を参照する。
-   入力の読み方と判定（フェーズ 1〜3）は、外部のコマンドを起動しない純粋な関数として書き、例を直接与えてテストする（設計書 3.1、AC-35）。git・go・node を起動する部分はフェーズ 4 にまとめる。
-   テストを加えたら、その対象（判定の分岐、受け入れる形の検査）を外して失敗することを確かめ、コミットメッセージにそう書く（`CLAUDE.md`「Testing Strategy」、設計書 7.1 の「壊して確かめる」）。
-   Go と TypeScript のコメント・識別子・文字列リテラル・道具の出力の文言は英語で書く（`CLAUDE.md`「Go Idioms」、要件書 4.5）。

### 1.3. 計画の際に決めたこと（2026-11-02）

-   **PR は 6 つに分ける（人間のレビュアーが決めた）:** 入力の読み方（フェーズ 1）、判定（フェーズ 2）、範囲の補助（フェーズ 3）、`internal` の実行の部分（フェーズ 4）、`cmd`・Makefile とそれに結びつく文書（フェーズ 5）、規則の書き換えと実際の一式での確認（フェーズ 6）である。理由: フェーズ 1〜3 は互いに独立で、外部のコマンドを起動せずに確かめられる（設計書 8 章）。フェーズ 4 は後始末・シグナル・環境の絞り込みという危険の高い処理を含むので、`cmd` と Makefile から分けて詳しく読めるようにする。Makefile に `ext-coverage-compare` を加えると、CLAUDE.md の一覧と `TestExtensionDocumentation` の正規表現も同じ PR で直さないと `make test` が通らない（1.4）。`cmd/coveragecompare/main.go` の `os.Environ` と `allowedEnvRefs` も同じである。そのため、これらはフェーズ 5 に入れる。CLAUDE.md の規則の書き換えは、実際の一式で道具が合格することを確かめた後に入れる（フェーズ 6）。
-   **統合テストの偽の go・node・npm は、テストのバイナリを起動し直して作る（人間のレビュアーが決めた）:** `internal/codexreview/testutil/helpers.go` の `Main`（`:108-`）と同じく、`TestMain` が、偽のコマンドの名前で起動されたかを見て、そうなら偽のコマンドとして振る舞う。理由: 偽の go・node は、`-coverprofile=`・`--test-reporter-destination=` の引数を読んで報告のファイルを書き、範囲の補助の呼び出しにも答える。引数の解析を Go で書けるので、シェルで書くより確実である。偽のコマンドの振る舞いは、偽のコマンドの隣に置くファイルで与える。補助は設計書 2.2 のとおり `internal/coveragecompare/test_helpers.go`（`//go:build test`）に置き、新しいパッケージにはしない。
-   **シグナルの購読を保つ処理は `cmd/coveragecompare` に同じ形で書く（人間のレビュアーが決めた）:** `cmd/codexreview/main.go:50-70` の `heldSignals` と同じ約 8 行を `cmd/coveragecompare/main.go` に書き、`signalrun` には取り出さない。理由: `main` から入れて使う `codexreview` の入口を変えずに済む。重複は数行で、`codexreview` を参照するコメントを置く。
-   **設計書が実装計画に委ねた値:** 次のとおりにする。
    -   BASE を道具に渡す専用の make の変数の名前は `COVERAGE_COMPARE_BASE` とする（設計書 3.2）。`BASE` と違い、利用者の環境にたまたまある名前と重なりにくい。
    -   テストのプロセスグループの `GroupStopGrace` は 5 秒、`WaitDelay` は 1 秒とする（設計書 3.3）。`internal/codexreview/run.go:31`・`:42` と同じ値で、テストの一式が SIGTERM の後に後始末をする時間として、同じ考え方を使う。
    -   抜き書きの上限は 1 つあたり 200 バイト、`test-failed` で出すテストの出力の末尾は 20 行とする（設計書 3.9）。1 行の結果に貼れる長さと、失敗の原因の行（Go の `--- FAIL` の前後、node の `not ok` の詳細）が入る長さを合わせたものである。
-   **実際の一式での確認には、プッシュしない一時的なコミットを使う:** AC-07 の「テストを削除しただけの実際のコミットの組」は、フェーズ 6 で、確認のためのスクリプト（ステップ 6-2）が一時的なクローンの中に作る。リポジトリに、確認のためだけのテストの削除を加えないためである。

### 1.4. 既存コード調査結果

**外部のコマンドを起動する境界（`internal/cmdrunner`）**

| 項目 | 今あるもの | 本書での使い方 |
|---|---|---|
| 起動 | `NewOS(Options)` と `Runner.Run`（`runner.go:25-27`・`:96`・`:101-`）。`Options` は `MaxOutput`・`WaitDelay`・`Dir`・`Env`・`StderrIsFailure`・`ProcessGroup`・`GroupStopGrace`（`:52-60`） | 変えない。git・go・node・npm をすべてこれで起動する（設計書 2.1）。`MaxOutput` は 0（上限なし。AC-32） |
| 失敗 | 0 以外の終了は `*ExitError`（`Code`・`Stdout`・`Stderr`。`:67-75`）。取り消しでは context のエラーと合わせて返す（`:130-132`）。成功したときの標準エラー出力は捨てる | `test-failed` の出力の末尾は `ExitError.Stdout` から取る。取り消しは `internal` に分ける（設計書 4 章） |
| git | `GitEnv`（`git.go:32-44`）と `GitHardening`（`:19-30`） | 変えない。`GitEnv` の結果に `GIT_CONFIG_GLOBAL=/dev/null`・`GIT_CONFIG_NOSYSTEM=1` を加える（設計書 3.2）。`GitEnv` は `GIT_` で始まる変数を落としてから決まった 2 つを加えるので、加える 2 つは `GitEnv` の後に足す |
| 偽の `Runner` | `internal/cmdrunner/testutil` の `Fake`（`mocks.go:34-`）。決めた順の呼び出しを再生する | 使わない。フェーズ 1〜3 は外部のコマンドを起動せず、フェーズ 4 は実際の git と偽の go・node・npm で確かめる（設計書 7.2） |

**入口とシグナル（`internal/signalrun`、`cmd/codexreview`）**

-   `signalrun.Run`（`signalrun.go:42-`）は、最初のシグナルで context を取り消し、購読を外す。2 回目のシグナルが後始末の前にプロセスを終わらせないように、`cmd/codexreview/main.go:50-70` は `heldSignals` を実行の間ずっと購読する。`cmd/coveragecompare` は同じ処理を持つ（1.3）。
-   `cmd/codexreview/main.go:30-37` は終了コード 2 を使わない理由を書いている。`ExitCode` も 2 を使わない（設計書 3.9）。

**JSON の読み取り（`internal/strictjson`）**

-   `ParseObject`（`strictjson.go:72`）、`Object.Collect`・`CollectOnly`（`:90`・`:109`）、`Value.AsString`・`AsArray`・`AsObject`・`AsNonNegativeInt64`（`:200`・`:297`・`:284`・`:257`）がある。`go test -json` の各行（拡張されうるので `Collect`）、範囲の補助の出力（型のとおりでなければ拒否するので `CollectOnly`）、`extension/package.json` の `packageManager` を、これらで読む。新しい関数は要らない。

**Makefile と、それを読むテスト**

| 項目 | 今あるもの | 変えること |
|---|---|---|
| `.PHONY` | `Makefile:57` | `coverage-compare`・`ext-coverage-compare` を加える（ステップ 5-2） |
| go の呼び方 | `GOCMD=go`（`Makefile:1`）。ほかの対象は `$(GOCMD)`・`$(GOTEST)` を使う | 新しいレシピも `$(GOCMD)` を使う |
| 拡張の前提の確認 | `ext_check_versions`（`:309-324`）、`ext_preflight`（`:326-332`） | `ext-coverage-compare` は呼ばない（設計書 3.2）。`ext_check_versions` に、道具が同じ確認を持つことを示すコメントを加える |
| `ext-` の一覧のガード | `cmd/yt2column/extension_docs_test.go:37`（Makefile の `^ext-…:`）と `:41`（CLAUDE.md の `` ^- `make (ext-…)` ``）を、`TestExtensionDocumentation` の部分テスト（`:75-82`）が比べる | `:41` を、対象の名前の後ろの空白と引数を受け入れる形にする（設計書 3.11）。Makefile に `ext-coverage-compare:` が加わると、CLAUDE.md に載せるまでこの部分テストが失敗する |
| make を実行するテスト | `cmd/yt2column/makefile_test.go::TestMakeBuildBuildsEveryBinary`（`:479-`）は、`PATH` と `HOME` だけの環境で `make -n` を実行する。`internal/maketestutil` の `RunTarget`（`make.go:106-`）は `GOTEST` を記録するスタブに置き換えて対象を実行する | 新しいレシピは `GOTEST` を使わないので `RunTarget` はそのまま使えない。`make -n` の形と、`PATH` の先頭に偽の go を置く形（設計書 7.2・7.3）で確かめる |
| 道具の npm の使い方のガード | `extension/test/repository.test.ts:116-128` は、Makefile が `npm` を `ci`・`run`・`pkg`・`--version` でだけ使うことを確かめる | 変えない。新しいレシピは `npm` を書かない（`npm --version` は道具が起動する） |

**環境の読み取りとパッケージの一覧のガード**

-   `internal/config/envaccess_test.go` の `allowedEnvRefs`（`:65-77`）は、`internal/config` の外で環境を読む製品のコードを、ファイルごとに数で許す。`cmd/codexreview/main.go` は `{"call:os.Environ": 1}`（`:73-76`）。`cmd/coveragecompare/main.go` にも同じ 1 つを許す（ステップ 5-1）。`os.TempDir` が読む `TMPDIR` は数えない（`:20-26`）ので、`os.MkdirTemp("", …)` はこの検査に当たらない。`internal/coveragecompare` は環境を `Options.Environ` で受け取り、自分では読まない。
-   `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（`:629-`）は、本体のコードを持つパッケージに `package_reference.md` の行がないと失敗する。`internal/coveragecompare` の行はフェーズ 1 で、`cmd/coveragecompare` の行はフェーズ 5 で加える。
-   `TestFakesCarryBuildTag`（`pipeline_test.go:553-`）は、`test_helpers*.go` の 1 行目が `//go:build test`（または `test || integration`）であることを確かめる。数を固定しているのは `testutil` のファイル（25）だけ（`:577`）で、本書は `testutil` のファイルを加えない。

**リンタ**

-   `.golangci.yml` の除外（`:181-198`）は `_test.go` だけに当たり、`test_helpers.go` には `gosec` が働く。偽のコマンドのためにテストのバイナリを起動し直す呼び出し（G204）と、実行できるリンクやファイルの作成には、`internal/codexreview/testutil/helpers.go:237`・`:356` と同じく、その行だけに理由を添えた `//nolint:gosec` を付ける。
-   `depguard` は、`internal/coveragecompare` と `cmd/coveragecompare` に `os/exec` を禁じていない（禁じているのは `internal/nmhost` と `cmd/yt2column-nmhost` だけ。`:132-151`）。本体は `os/exec` を直接使わず `cmdrunner` を通すが、テストの補助は偽のコマンドの起動に使う。

**拡張**

-   `extension/package.json:12` の `test` スクリプトは `node --test "test/**/*.test.ts"` である。道具の指定と同じであることを確かめるテストを置く（ステップ 4-4）。
-   `typescript` は `devDependencies` にある（`package.json:23`。`^6.0.3`）。`extension/test/typecheck.test.ts:10` は `node_modules/typescript` を直接使っている。範囲の補助は `import ts from "typescript"` で読む。新しい npm のパッケージは加えない。
-   `extension/tsconfig.json` の `include` は `scripts` を含み、`eslint.config.js:273-275` は `scripts/**/*.ts` に `src/` と同じ規則を当てる。範囲の補助は `make ext-typecheck`・`make ext-lint`・`make ext-fmt-check` の対象になる。
-   `extension/scripts/check-lockfile.ts` は、関数を `export` し、`if (import.meta.main)` の中で実行する形である（`:58-`）。範囲の補助も同じ形にし、テストは関数を直接呼ぶものと、`node` で起動して終了コードと出力を見るものを置く。

**CLAUDE.md**

-   「Deleting a test is a claim that must be checked」は `CLAUDE.md:490-491`、「Test Commands」の一覧は `:137-`、「Extension Commands」は `:192-204` にある。「Test Commands」の一覧を Makefile と比べるガードは今ない（`cmd/` と `internal/` のテストに、`Test Commands` を読むものはない）。AC-38 の値を確かめるため、`make coverage-compare` の行を確かめるガードを加える（ステップ 5-3）。

**再利用しないもの**

-   0038・0052 の一度きりの道具、`internal/codexreview` の祖先の確認・`git status`・状態の読み取りは、設計書 3.2・3.12 のとおり再利用しない。
-   `internal/codexreview/testutil` の `Main`・`NewRepo` は codex の偽物に結びついた形（振る舞いのファイルの形、`PATH` に git だけを置く処理）なので、import せず、同じ考え方の補助を `internal/coveragecompare/test_helpers.go` に書く（1.3）。

**具体的な型の構築の経路（mkplan 手順 5）**

-   本書の設計は、具体的な型で振る舞いを例外にしたり分岐したりしない。Go と拡張の違いは、パッケージの中の `target` インターフェースの 2 つの実装に閉じる（設計書 3.1）。停止の種類・結果の種類・`RunState` などの列挙型は、ゼロ値が最も安全な解釈（`StopInternal`・`OutcomeStop`・`RanUnknown`）で、`switch` の `default` を `internal` として扱う（設計書 3.1）。そのため、構築の経路をリーフパッケージに閉じ込める必要はない。`Result` はテストからも組み立てる（`Write` のテスト）ので、フィールドは公開のままにする（設計書 3.1）。

**`implementation_handoff.md`**

-   このタスクにはない。`design_handoff.md` の H-01〜H-09 は、すべて設計書 10 章に記録されている。

## 2. 実装手順 (Implementation Steps)

ファイルの分け方は設計書 2.2 が実装計画に委ねたものである。`internal/coveragecompare` のファイルは、各ステップの「変更するファイル」のとおりに分ける。新しいテストの関数名は予定の名前で、実装で変えたときは本書 7 章を同じ PR で直す。

### フェーズ 1: 入力の読み方（設計書 8 章の手順 1）

#### ステップ 1-1: 公開の型とエラーの型

**変更するファイル:** `internal/coveragecompare/coveragecompare.go`（新規）、`internal/coveragecompare/errors.go`（新規）、`docs/dev/conventions/package_reference.md`

-   [ ] 設計書 3.1 の公開の型（`Target`・`Outcome`・`StopKind`・`Options`・`Result`・`FuncID`・`Measure`・`RunState`・`DeletedFunc`・`Unit`・`Coverage`・`LostFunc`・`Point`・`Input`・`StopReason`）と、設計書 4 章の `StopError` を書く。`Run`・`Write`・`ExitCode` はフェーズ 2・4 で加える。
-   [ ] パッケージのコメントに、道具の目的と、外部のコマンドを起動するのは `Run` の経路だけであることを書く。
-   [ ] `package_reference.md` に `internal/coveragecompare` の行を加える。行の内容は、このフェーズで加えるもの（型と入力の読み方）に合わせ、フェーズ 2・4 で書き足す。

#### ステップ 1-2: Go の入力（プロファイルと `go test -json`）と製品のコードの見分け方

**変更するファイル:** `internal/coveragecompare/profile.go`、`internal/coveragecompare/gotestjson.go`、`internal/coveragecompare/product.go`（いずれも新規）と、それぞれの `_test.go`

-   [ ] プロファイルを、設計書 3.4 の「プロファイルの受け入れる形」「重複したブロック」のとおりに読む。モジュールのパスは `go.mod` の `module` の行から得て、ファイルをリポジトリのルートからの相対パスにする（AC-27）。外れた行は `StopError{Kind: StopUnreadable}` に、時点と行の番号と抜き書きを付けて返す。
-   [ ] `go test -json` の出力を、設計書 3.4 の「テストの実行の結果の読み方」のとおりに読み、テストの数と失敗（`fail` の `Test` か `Package`、`build-fail` の `ImportPath`）を返す。終了コードによる `test-failed` の判定は、フェーズ 4 の実行の側で行う。
-   [ ] Go の製品のコードの見分け方を、設計書 3.4 の「製品のコードの見分け方」のとおりに書く（`go/build/constraint`。`test` を偽、`-tags test` のビルドの値で評価する）。
-   [ ] テストを書く（設計書 7.1 の「入力の読み方」「製品のコードの見分け方」）。プロファイルは、`mode:` の行がない・2 つある・1 行目でない、形の崩れた行、負や `int` に収まらない数、始まりが終わりより後ろ、モジュールの外・空・絶対・`.`/`..` を含むパス、同じ位置で文の数が違うブロック、複数のテストのバイナリから出た同じブロックのまとめ方（どれかが 1 以上なら実行された）を含める。`go test -json` は、サブテストを数えないこと、`fail`・`build-fail` の行、JSON のオブジェクトでない行、テストが 0 の出力を含める。製品のコードは、`4bf1842f` の 7 種類の `//go:build` の行と `integration || test`、`//go:build` の行がないファイル、`_test.go` を含める。

#### ステップ 1-3: 拡張の入力（lcov と TAP）

**変更するファイル:** `internal/coveragecompare/lcov.go`、`internal/coveragecompare/tap.go`（いずれも新規）と、それぞれの `_test.go`

-   [ ] lcov を、要件書 AC-29 と設計書 3.5 の「lcov の読み方」のとおりに読む。`SF` のパスに `extension/` を前に付け、絶対パス・`..` を含むパス・空のパスを拒否する。製品のコードの見分け方（`src/`・`scripts/` で始まるか、パスの区切りの単位で比べる）もここに置く。
-   [ ] TAP を、設計書 3.5 の「テストの実行の結果の読み方」のとおりに読み、`# tests` の値と、`# fail`・`# cancelled` の値、`not ok` の行の名前を返す。
-   [ ] テストを書く（設計書 7.1）。lcov は、要件書 AC-29 の拒否の条件をそれぞれ 1 行以上（知らないレコード、閉じていないまとまり、まとまりの外のレコード、重複した `SF`、`FN` と `FNDA` の数が違う・名前が違う、符号・小数・範囲外の数）と、読み飛ばすレコードの受理、`SF` のパスの拒否と `src/`・`scripts/` の見分け（`srcx/` のような前方一致を外すこと）を含める。TAP は、`# tests`・`# fail`・`# cancelled` がない・2 つ以上ある・負や数でない、字下げされた `# tests` を数えないこと、字下げの深さによらない `not ok` を含める。

#### ステップ 1-4: 差分

**変更するファイル:** `internal/coveragecompare/diff.go`（新規）、`internal/coveragecompare/diff_test.go`

-   [ ] `git diff` の出力を、設計書 3.7 の「差分の受け入れる形」のとおりに、ファイルごとの hunk の並び（BASE と HEAD のパス、新しいファイル・削除したファイル、hunk の見出しの値と `-`・`+` の行）に読む。行の対応への変換はフェーズ 2 で行う。
-   [ ] テストを書く（設計書 7.1）。受け入れる見出しの各形、hunk の後ろの関数名の文脈、`\ No newline at end of file`、拒否する形（知らない見出しの行、`Binary files ... differ`、引用符で囲まれたパス、空白を含むパス、hunk の見出しの数と行の数が合わない場合）を含める。

#### ステップ 1-5: 実際の出力の例

**変更するファイル:** `internal/coveragecompare/testdata/`（新規）、`internal/coveragecompare/testdata_test.go`（新規）

-   [ ] `v24.21.0` の実際の lcov の一部と TAP の末尾、`go1.27.1` の実際の `go test -json` の数行とプロファイルの一部を、このリポジトリを測った出力から取って置く（設計書 7.1 の「実際の出力の例」）。取ったコミットと版を、`testdata/` の中の短い説明のファイルに書く。
-   [ ] それぞれを 1〜4 のステップの読み方で読み、受け入れられることと、読んだ値（テストの数、`FN` の数など）が例の中身と合うことを確かめるテストを書く。

**フェーズ 1 の完了の条件:** `make fmt`、`make test`、`make lint`、`make deadcode` が通る。このフェーズの関数は本番のコードから呼ばれないが、`make deadcode` はテストも到達の起点に含める（`CLAUDE.md`「Code Quality」）。

### フェーズ 2: 判定（設計書 8 章の手順 2）

#### ステップ 2-1: 関数表

**変更するファイル:** `internal/coveragecompare/gofuncs.go`、`internal/coveragecompare/table.go`（いずれも新規）と、それぞれの `_test.go`

-   [ ] Go のソースから、関数の宣言ごとの範囲と名前（メソッドはレシーバの型を添える）を `go/parser` で求める（設計書 3.6 の「Go の関数」）。
-   [ ] 1 つの時点の関数表を、Go はソースの範囲とプロファイルから、拡張は範囲の補助の出力（フェーズ 3 で読む形を、ここでは Go の値として受け取る）と lcov から作る（設計書 3.5 の「`FN` の行と構文木の対応」、3.6）。関数の宣言の外のブロックはどの関数にも属させない。報告に出ないファイルの関数は「記録なし」にする。同じ (ファイル, 行) から始まる Go の関数の宣言は `unmatched` の条件として集める（その場で止めない。設計書 3.6）。拡張の `FN` の行から始まる範囲がないときは `unreadable` にする。
-   [ ] 読むファイルの集合（両方の報告と差分の製品のコードのファイルの和）と、`Measure` の数え方を、設計書 3.6 の「読むファイル」「関数の数」のとおりに書く。
-   [ ] テストを書く。Go は、メソッドとレシーバ、関数の宣言の外のブロック（AC-21）、同じ行の 2 つの宣言（AC-15）、報告に出ないファイル、丸めない網羅率の分子と分母。拡張は、同じ行から始まる関数のまとまり、`fieldGroups` の行の `<instance_members_initializer>`、`FN` に当たる範囲がない場合、`anonymous_<番号>` の名前が変わっても結果が変わらないこと（AC-17）、`FN` のない行の名前が `<unknown>` になること。

#### ステップ 2-2: 行の対応と関数の分類

**変更するファイル:** `internal/coveragecompare/mapping.go`、`internal/coveragecompare/classify.go`（いずれも新規）と、それぞれの `_test.go`

-   [ ] 差分の hunk から、BASE の各行を「HEAD の行 N に対応する」「削除された」「書き換えられた」に分け、追加された行の BASE での位置を記録する（設計書 3.7 の「行の対応」）。
-   [ ] 関数の分類（削除した関数・残る関数・対応が取れない）と、対応の相手がない場合・HEAD にだけある関数・BASE の報告にないファイルの扱いを、設計書 3.7 の表と箇条のとおりに書く。
-   [ ] 純粋な削除の hunk の位置のずらしを、設計書 3.7 の「純粋な削除の位置」のとおりに書く。受け入れられる位置がない・1 つ・2 つ以上で分類が同じ・2 つ以上で分類が違う、の 4 つの場合を分ける。
-   [ ] テストを書く（設計書 7.1 の「比較と分類」「範囲の境界」）。途中の関数の削除で後ろの行がずれた場合、ファイルごとの削除、範囲の中が書き換えられた関数、一部の行だけが削除された関数、範囲の中への追加、始まりの行の直前と終わりの行の後ろへの追加（範囲の外）と始まりの行の後ろへの追加（範囲の中）、HEAD にだけある関数、名前を変えて移したファイル、どのテストも読み込まない新しいファイル、範囲の外の変更（import・パッケージの変数・関数の前のコメント）、閉じ括弧の行を前の関数から削除したとみなす差分、同じ内容の関数が並んで削除した関数が決まらない差分。

#### ステップ 2-3: 比較と判定の順序

**変更するファイル:** `internal/coveragecompare/compare.go`、`internal/coveragecompare/judge.go`（いずれも新規）と、それぞれの `_test.go`

-   [ ] Go と拡張の比較を、設計書 3.8 のとおりに書く（Go は分数のまま比べ、HEAD の相手が「記録なし」なら `unmatched`。拡張は行のまとまりの `FN` の数と実行された数で比べる。外側の関数の手がかりを `StopReason.Outer` に入れる）。削除した関数の `RunState` を決める。
-   [ ] 2 つの時点の読んだ報告・関数表・差分から、`nothing-measured`・`unmatched`・不合格・合格の順に判定し、`Result` の判定に関わる部分を作る関数を書く（設計書 3.8 の「測れていることの確認」、3.9 の最後の段落）。関数表を作る途中で集めた `unmatched` の条件は、`nothing-measured` の判定の後に出す（設計書 3.6）。
-   [ ] テストを書く（設計書 7.1 の「比較と分類」）。不合格: 網羅率が下がった Go の関数、丸めると同じになる小さな減少（例: `1000/1001` から `999/1001`）、実行回数が 0 になった拡張の関数、HEAD の報告から消えたファイル、同じ行で実行された数が減った場合。`unmatched`: 同じ行で実行された数が増えた場合と関数の数が変わった場合、HEAD の相手が「記録なし」の Go の関数、`FN` が HEAD の lcov から消えた行と、外側の関数の手がかり。合格: BASE で実行回数 0 の関数を含む行が変わらない場合、どのテストも読み込まなかったファイルの関数の削除（`NotRunAtBase`）、文を持たない関数（`NoStatements`）。判定の順序: テストの数が 0 の時点と製品の関数の数が 0 の時点（BASE・HEAD のそれぞれ）、`nothing-measured` の条件と `unmatched` の条件が両方あるとき `nothing-measured` になること、`unmatched` と不合格が両方あるとき `unmatched` になること。
-   [ ] 要件書 AC-33・AC-34 の各項目を、読んだ報告と差分の例から判定まで通して確かめる表のテスト（`TestJudgeScenarios`）を書く。上の各テストと同じ行を書き写さず、AC-33・AC-34 の項目ごとに 1 行を置き、期待する結果（停止の種類、不合格、合格と削除した関数の一覧）を確かめる。

#### ステップ 2-4: 結果の出し方と終了コード

**変更するファイル:** `internal/coveragecompare/output.go`（新規）、`internal/coveragecompare/output_test.go`

-   [ ] `Write` と `ExitCode` を、設計書 3.9 のとおりに書く。抜き書きの上限と出力の末尾の行の数は 1.3 の値にする。名前・パス・抜き書き・BASE の指定は `%q` で出す。
-   [ ] テストを書く（設計書 7.1 の「結果の出し方」）。各停止の種類の最後の行、合格のときの各項目と並び、不合格の `lost` の行、不合格と止まったときに `no compared function lost coverage` が出ないこと、各停止の種類で出す内容（`test-failed` の時点・名前・末尾の行・実行し直すコマンド、`unreadable` の入力・行・抜き書き、`unmatched` の関数と外側の関数、`internal` の処理と元のエラー）、抜き書きの上限の境界、制御文字の引用、`RunState` の添え書き、`ExitCode` の 3 つの値と、範囲外の `Outcome` の扱い。

**フェーズ 2 の完了の条件:** `make fmt`、`make test`、`make lint`、`make deadcode` が通る。`package_reference.md` の `internal/coveragecompare` の行に、判定と `Write`・`ExitCode` を書き足す。

### フェーズ 3: 範囲の補助（設計書 8 章の手順 3）

#### ステップ 3-1: `function-ranges.ts`

**変更するファイル:** `extension/scripts/function-ranges.ts`（新規）、`extension/test/functionRanges.test.ts`（新規）

-   [ ] 設計書 3.5 の「範囲の補助」のとおりに書く。引数は `--root <dir> -- <パス>...` だけを受け入れ、出力は設計書 3.1 の `FunctionRanges` の形の JSON 1 つである。読めないファイル・`--root` の外を指すパス・引数の形の誤りでは、何も出さずに 0 以外で終わる。`check-lockfile.ts` と同じく、関数を `export` し、`import.meta.main` の中で起動する（1.4）。
-   [ ] テストを書く（設計書 7.1 の「範囲の補助」）。関数の宣言、関数式、アロー関数、入れ子の関数、同じ行から始まる 2 つの関数、メソッド、コンストラクタ、アクセサ、前に付くコメントを含めない始まりの行、インスタンスのフィールドのまとまり（静的なフィールドを含めないこと）、`--root` の木を読むこと（同じパスで中身の違う 2 つの木で違う範囲が出る）、読めないファイル・`--root` の外のパス・引数の誤りで 0 以外で終わり標準出力が空であること。

#### ステップ 3-2: 範囲の補助の出力を読む

**変更するファイル:** `internal/coveragecompare/ranges.go`（新規）、`internal/coveragecompare/ranges_test.go`

-   [ ] 範囲の補助の標準出力を `internal/strictjson` で読み、フェーズ 2 の関数表が受け取る Go の値にする（設計書 3.5）。型のとおりでない場合、与えたパスがちょうど 1 回ずつ与えた順に出ていない場合、行が 1 より小さい・終わりが始まりより前の場合は `unreadable` にする。
-   [ ] テストを書く。受け入れる例、知らないメンバー、重複したメンバー、パスの欠け・重複・順序の違い、数でない行、負の行。

**フェーズ 3 の完了の条件:** `make fmt`、`make test`、`make lint`、`make deadcode`、`make ext-check` が通る。

### フェーズ 4: `internal` の実行の部分（設計書 8 章の手順 4 のうち `internal/coveragecompare`）

#### ステップ 4-1: git の起動、前提の確認、利用者の状態

**変更するファイル:** `internal/coveragecompare/git.go`、`internal/coveragecompare/precheck.go`、`internal/coveragecompare/userstate.go`（いずれも新規）

-   [ ] git の起動の環境と引数を、設計書 3.2 の「git の起動の仕方」のとおりに 1 か所で作る（1.4 の `GitEnv` の後に 2 つを加える）。
-   [ ] 前提の確認を、設計書 3.2 の表の順（`usage`・`not-ancestor`・`dirty`・`toolchain`）で書く。BASE と HEAD を 40 桁のコミットの ID に一度だけ解決し、そのあとの git の操作はすべてその ID を使う（設計書 3.2 の「コミットの ID の固定」）。
-   [ ] toolchain の確認を、Go と拡張について設計書 3.2 の「toolchain の確認」のとおりに書く（ファイルは `git show <ID>:<path>` で読む。`go/version` で比べる。lockfile が BASE と HEAD で違えば止める）。
-   [ ] 利用者の状態（HEAD、今のブランチ、`git status` の出力、作業ツリーのパスの集合）を記録し、後始末の後に比べる（設計書 3.3 の「利用者の状態の確認」）。

#### ステップ 4-2: チェックアウト、子のプロセスの環境、後始末

**変更するファイル:** `internal/coveragecompare/checkout.go`、`internal/coveragecompare/childenv.go`（いずれも新規）

-   [ ] 作業用のディレクトリを `os.MkdirTemp` で作り、その下に BASE と HEAD のチェックアウトを `git clone --shared --no-checkout` と `git -c core.hooksPath=/dev/null checkout --detach <ID>` で作る（設計書 3.3）。
-   [ ] 拡張のとき、各チェックアウトの `extension/node_modules` を実際のディレクトリとして作り、利用者の `extension/node_modules` の各項目へのシンボリックリンクを置く。BASE のテストの前と HEAD のテストの後に `extension/node_modules/.package-lock.json` を比べ、違えば `internal` にする（設計書 3.3 の「拡張の依存パッケージ」）。
-   [ ] go と node に渡す環境を、設計書 3.3 の「子のプロセスの環境」のとおりに、名前で選んだ変数と道具が決める変数（`TMPDIR`・`GOTMPDIR` は作業用のディレクトリの下の `tmp`）から作る関数を書く。
-   [ ] 後始末は、取り消されない context で作業用のディレクトリを消す。`Run` がどの終わり方でも戻る前に行う（設計書 3.3 の「後始末」）。

#### ステップ 4-3: テストの実行、`target` の 2 つの実装、`Run`

**変更するファイル:** `internal/coveragecompare/target.go`、`internal/coveragecompare/target_go.go`、`internal/coveragecompare/target_ext.go`、`internal/coveragecompare/run.go`（いずれも新規）、`docs/dev/conventions/package_reference.md`

-   [ ] 設計書 3.1 の `target` インターフェースを置き、Go と拡張の実装に、フェーズ 1〜3 の読み方と判定の関数を束ねる。テストの実行は、設計書 3.4・3.5 のコマンドを、`cmdrunner` の `ProcessGroup`・`GroupStopGrace`・`WaitDelay`（1.3 の値）と `MaxOutput` 0 で起動する。範囲の補助は `StderrIsFailure` を付けて HEAD のチェックアウトの補助を起動する（設計書 3.5）。
-   [ ] 差分を、設計書 3.7 の「差分の取り方」の引数で、固定したコミットの ID の間で取る。
-   [ ] `Run` を、設計書 2.3 の流れと 3.9 の最後の段落の順で書く。BASE のテストが失敗したら HEAD のテストを実行しない。`internal` は起きた時点で止める。どの終わり方でも、後始末と利用者の状態の確認の後に `Result` を返す。`StopError` でないエラーは `internal` にする（設計書 4 章）。取り消された context では、取り消されたことを `internal` の理由にする。
-   [ ] `package_reference.md` の `internal/coveragecompare` の行に、`Run` と実行の部分を書き足す。

#### ステップ 4-4: 統合テストと偽のコマンド

**変更するファイル:** `internal/coveragecompare/test_helpers.go`（新規、`//go:build test`）、`internal/coveragecompare/main_test.go`（新規。`TestMain`）、`internal/coveragecompare/run_test.go`、`internal/coveragecompare/precheck_test.go`、`internal/coveragecompare/childenv_test.go`（いずれも新規）

-   [ ] 1.3 の方式で、偽の `go`・`node`・`npm` を書く。`TestMain` は、偽のコマンドの名前で起動されたら振る舞いのファイルに従って答え、テストを実行しない。偽のコマンドは、起動された引数と環境と作業ディレクトリを記録し、テストの実行・`go env GOVERSION`・`node --version`・`npm --version`・範囲の補助の呼び出しに、振る舞いのファイルのとおりに答える（報告のファイルは、引数の `-coverprofile=`・`--test-reporter-destination=` の場所に書く）。偽のコマンドを置く `PATH` には、偽のコマンドと実際の `git` だけを置き、実際の `go`・`node`・`npm` が見つからないことを、テストの前に確かめる（AC-35）。
-   [ ] 一時的な git のリポジトリを作る補助を書く。署名を切り、利用者の大域の設定を読まない（`GIT_CONFIG_GLOBAL` を空のファイルに向ける）。
-   [ ] `test_helpers.go` の `gosec` に当たる行（テストのバイナリの起動し直し、実行できるリンクの作成）だけに、理由を添えた `//nolint:gosec` を付ける（1.4）。
-   [ ] 前提の確認のテスト（`TestRunPreconditions`。設計書 7.2 の「前提の確認」）を書く。BASE がない・コミットとして解決できない・HEAD と同じ・祖先でない、変更・ステージした変更・追跡されていないファイルがある（git が無視するファイルだけなら止まらない）、BASE または HEAD の `go.mod` の `go` の行が新しい・読めない、`.node-version` と違う、`packageManager` と違う、`node_modules` がない、lockfile が違う。各行で、停止の種類と、偽のコマンドにテストの実行の記録がないことを確かめる。2 つの条件に同時に当たる行で、表の順の早いほうで止まることも確かめる（AC-23）。
-   [ ] 後始末と利用者の状態のテスト（`TestRunCleansUp`。設計書 7.2 の「後始末と利用者の状態」）を書く。Go と拡張のそれぞれで、合格・不合格・テストの失敗・読めない報告・取り消された context の各場合に、作業用のディレクトリが残らず（偽のコマンドが記録した `TMPDIR` の親が消えている）、作業ツリーの一覧・ブランチ・HEAD・`git status` が変わらないことを確かめる。偽のテストが利用者のリポジトリにブランチを作る場合に `internal` で止まること（`TestRunDetectsUserStateChange`）、`.package-lock.json` が途中で変わる場合に `internal` で止まることも確かめる。
-   [ ] 実行の流れのテスト（`TestRunEndToEnd`）を書く。Go と拡張のそれぞれで、偽の報告から合格・不合格になり、`Result` の `Toolchain`・`Measure`・コミットの ID・ツリーの ID が偽のコマンドの答えと一時的なリポジトリの値に一致すること、テストのコマンドの引数が設計書 3.4・3.5 のとおりであること、偽のテストが BASE と HEAD のチェックアウト（固定したコミットの ID）で起動されたこと、拡張の各チェックアウトの `node_modules` が実際のディレクトリで、その項目が利用者の `node_modules` を指すことを確かめる。BASE のテストの失敗で HEAD のテストが起動されないこと（`TestRunStopsOnBaseFailure`）も確かめる。
-   [ ] 子のプロセスの環境のテスト（`TestChildEnv`）を書く（設計書 7.1 の「子のプロセスの環境」）。`NODE_OPTIONS`・`GOFLAGS`・`GOENV`・`GIT_DIR`・`DEEPSEEK_API_KEY` を含む環境で `Run` を通し、偽のコマンドが記録した環境にそれらがなく、`GOTOOLCHAIN=local`・`GOENV=off`・`GOFLAGS=`・`GOPROXY=off`・`GOWORK=off` と `TMPDIR` があることを確かめる。git の起動の環境に `GIT_CONFIG_GLOBAL=/dev/null`・`GIT_CONFIG_NOSYSTEM=1` があることは、実行の結果から観測できないので、git の環境を作る関数を直接確かめる。
-   [ ] 利用者のリポジトリの `.git/config` に `diff.noprefix=true`・`diff.interHunkContext=3`・`diff.relative=true` を書いても、差分を読めて同じ結果になることを確かめる（`TestDiffIgnoresRepoDiffConfig`。設計書 3.7）。
-   [ ] 入力の大きさに上限がないことを確かめる（`TestRunAcceptsLargeOutput`、AC-32）。偽の go が、正しい `-json` の行を 20 MiB を超えて出しても、判定まで進むことを確かめる。
-   [ ] 道具の拡張のテストの対象の指定が、`extension/package.json` の `test` スクリプトの指定と同じであることを確かめる（`TestExtTestGlobMatchesPackageJSON`。設計書 3.5・7.1）。`package.json` は `internal/strictjson` で読み、`node --test` の後ろの引数を取り出して比べる。

**フェーズ 4 の完了の条件:** `make fmt`、`make test`、`make lint`、`make deadcode` が通る。`test_helpers.go` は `//go:build test` なので、`make lint`（`test,integration` のタグで解析する）と `make deadcode`（同じタグ）がコンパイルする。

### フェーズ 5: 入口・Makefile と、それに結びつく文書（設計書 8 章の手順 4 の残りと手順 5 の一部）

#### ステップ 5-1: `cmd/coveragecompare`

**変更するファイル:** `cmd/coveragecompare/main.go`（新規）、`cmd/coveragecompare/main_test.go`（新規）、`internal/config/envaccess_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] 設計書 2.2 の表のとおりに入口を書く。コマンドラインは `go --base <rev>` または `ext --base <rev>` だけを受け入れ、ほかの形は `Result{Outcome: OutcomeStop, Stop: StopUsage}` を `Write` で出して終了コード 3 で終わる。`signalrun.Run` の下で動かし、`heldSignals` を実行の間ずっと購読する（1.3）。
-   [ ] テストを書く（`TestRun`）。受け入れる 2 つの形、引数の数の違い、知らない対象、`--base` の欠け・重複・空の値、`--base=<rev>` の形、位置の違う引数が、`usage` の最後の行と終了コード 3 になることを確かめる（AC-01・AC-02）。
-   [ ] `allowedEnvRefs` に `"cmd/coveragecompare/main.go": {"call:os.Environ": 1}` を、`cmd/codexreview/main.go` の行と同じ形のコメントとともに加える（設計書 3.11）。
-   [ ] `package_reference.md` に `cmd/coveragecompare` の行を加える。

#### ステップ 5-2: Makefile の 2 つの対象

**変更するファイル:** `Makefile`、`cmd/yt2column/makefile_test.go`

-   [ ] `coverage-compare` と `ext-coverage-compare` を、設計書 3.2 の「`make` の対象」のとおりに加える。HEAD の `go.mod` の確認、道具の一時的なディレクトリへのビルド、シグナルの転送、0・1・3 以外の終了コードでの `internal` の最後の行、一時的なディレクトリの削除を、2 つの対象で共通の `define` に置く。BASE は `$(origin BASE)` が `command line` のときだけ `COVERAGE_COMPARE_BASE` に入れて `export` し、レシピのシェルが `"$$COVERAGE_COMPARE_BASE"` で道具に渡す。`.PHONY` に 2 つを加える。
-   [ ] `ext_check_versions` に、道具（`internal/coveragecompare` の toolchain の確認）が同じ値を確かめることを示すコメントを加え、道具の側にも `ext_check_versions` を示すコメントを置く（設計書 3.2）。
-   [ ] レシピのテストを書く（`TestCoverageCompareRecipes`。設計書 7.2 の「レシピの `go.mod` の確認」、7.3）。2 つの対象のそれぞれで、次を確かめる。`go list -m` が失敗する偽の go で `toolchain` の最後の行と 0 以外の終了。シェルの特殊文字を含む一意の BASE の値で、`make -n` の出力にその値が現れず `"$COVERAGE_COMPARE_BASE"` が現れること。`BASE` を環境だけに置いたとき、道具に空の値が渡ること（偽の go が `build -o <path>` で、受け取った引数を記録する代わりの道具を書く）。ビルドが失敗したとき、`internal` の最後の行と 0 以外の終了。代わりの道具が 2 で終わったとき、レシピが `internal` の最後の行を出すこと。代わりの道具が 0・1・3 で終わったとき、レシピが最後の行を書き足さないこと。make を起動するたびに、`TestMakeBuildBuildsEveryBinary` と同じく環境を `PATH`・`HOME` に絞る。

#### ステップ 5-3: CLAUDE.md の「Commands」とガード

**変更するファイル:** `CLAUDE.md`、`cmd/yt2column/extension_docs_test.go`、`cmd/yt2column/makefile_test.go`

-   [ ] 「Test Commands」に `make coverage-compare BASE=<commit>` を、「Extension Commands」に `make ext-coverage-compare BASE=<commit>` を、説明とともに加える（設計書 3.10 の「Commands」）。「Extension Commands」の冒頭の文は変えない。
-   [ ] `claudeExtTargetRE`（`extension_docs_test.go:41`）を、対象の名前の直後に `` ` `` か空白が来る形にする。対象の名前の読み取りは変えない（設計書 3.11）。直前のコメントも合わせる。
-   [ ] 「Test Commands」の節に `make coverage-compare` の行があり、Makefile がその対象を定義することを確かめるガード（`TestCoverageCompareDocumented`）を加える（AC-38）。節は見出しで探す（`doctestutil.SectionByHeading`）。
-   [ ] `claudeExtTargetRE` を元の形に戻すと `TestExtensionDocumentation` が失敗すること、CLAUDE.md の `make coverage-compare` の行を消すと `TestCoverageCompareDocumented` が失敗することを確かめる（コミットしない）。

**フェーズ 5 の完了の条件:** `make fmt`、`make test`、`make lint`、`make deadcode` が通る。`extension/test/repository.test.ts` の npm のガードのために `make ext-test` も通す（`CLAUDE.md`「Development Notes」の、Makefile の変更と拡張のテストの関係）。

### フェーズ 6: 規則の書き換えと実際の一式での確認（設計書 8 章の手順 5 の残りと手順 6）

#### ステップ 6-1: CLAUDE.md の「Deleting a test is a claim that must be checked」

**変更するファイル:** `CLAUDE.md`

-   [ ] `CLAUDE.md:490-491` を、設計書 3.10 の「Deleting a test is a claim that must be checked」の 6 項目を満たす英語の文に書き換える（AC-37）。
-   [ ] `docs/translation_glossary.md` に、本書で増えた用語で表に載せるべきもの（「関数表」など）がないかを確かめる。

#### ステップ 6-2: 実際の一式での確認のスクリプト

**変更するファイル:** `docs/tasks/0055_coverage_compare/_checks/realrun/main.go`（新規）

-   [ ] AC-06・AC-07 を確かめる一度きりのスクリプトを Go で書く（`go list ./...` は `_` で始まるディレクトリを無視するので、`make test` と CI は実行しない。`requirements_process.md` §4）。スクリプトは、今のリポジトリの HEAD を一時的なディレクトリにクローンし、その中に `extension/node_modules` を利用者のものへの項目ごとのリンクとして置き（設計書 3.3 と同じ形。`git status` を空に保つため）、引数で与えた変更（テストのファイルの削除、または `git apply` する差分）を 1 つのコミットにする。そのクローンで `make coverage-compare BASE=HEAD~1` と `make ext-coverage-compare BASE=HEAD~1` を実行し、両方の最後の行が `coverage-compare: PASS` であること、`tests`・`files`・`functions` の値が両方の時点で 0 より大きいこと、`toolchain` の行が入っている go と node の版と一致することを確かめ、出力をそのまま出す。終わったら一時的なディレクトリを消す。
-   [ ] 削除するテストは、実装のときに、削除しても製品の関数のカバレッジが変わらないと見込めるもの（Go と拡張それぞれ 1 つ）を選び、選んだ理由を本書 7 章の AC-07 の行に書く。

#### ステップ 6-3: 実行と記録

**変更するファイル:** `docs/tasks/0055_coverage_compare/03_implementation_plan.md`

-   [ ] ステップ 6-2 のスクリプトを PR のブランチで一度実行し、出力と、元にしたコミットの ID と一時的なコミットの ID、日付、go と node の版を本書 7 章の AC-06・AC-07 の行に記録する。不合格や停止のときは、原因を調べ、道具の誤りなら直してから実行し直す。
-   [ ] シグナルの手動の確認（設計書 7.2 の「シグナル」）を、実際の `make coverage-compare` の実行中に、端末の Ctrl-C、`make` のプロセスだけへの SIGTERM、`make` のプロセスだけへの SIGHUP の 3 つで行う。それぞれで、最後の行が `coverage-compare: STOP internal` であること、作業用のディレクトリとレシピの一時的なディレクトリが残らないこと（実行前後の `$TMPDIR` の一覧で確かめる）を、本書 7 章の AC-03 の行に記録する。
-   [ ] 合格の出力を PR の説明に貼る（設計書 7.2 の「実際のテストの一式での実行」）。

**フェーズ 6 の完了の条件:** `make test`、`make lint` が通る（CLAUDE.md を変えるので文書のガードを含む）。ステップ 6-3 の記録が本書にある。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: 入力の読み方 | プロファイル・`go test -json`・lcov・TAP・差分の読み方、製品のコードの見分け方、実際の出力の例 | フェーズ 1 の PR がマージされた |
| M2: 判定 | 関数表、行の対応、分類、比較、判定の順序、`Write`・`ExitCode` | フェーズ 2 の PR がマージされた |
| M3: 範囲の補助 | `function-ranges.ts` とその出力の読み方 | フェーズ 3 の PR がマージされた |
| M4: 実行の部分 | 前提の確認、チェックアウトと後始末、子のプロセスの環境、`Run`、統合テスト | フェーズ 4 の PR がマージされた |
| M5: 入口 | `cmd/coveragecompare`、2 つの `make` の対象、CLAUDE.md の「Commands」とガード | フェーズ 5 の PR がマージされた |
| M6: 手順の書き換え | CLAUDE.md の規則、実際の一式での確認とシグナルの確認の記録 | フェーズ 6 の PR がマージされた |

M1〜M3 は互いに独立である。M2 の拡張の関数表は範囲の補助の出力を Go の値として受け取るので、M3 より前に作れる。M4 は M1〜M3 の上に組み立てる。M5 は M4 の `Run` を呼ぶ。M6 は M5 の `make` の対象を実行する。

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどのテストで確かめるかは本書 7 章に示す。

-   **ユニットテスト（フェーズ 1〜3）:** 入力の読み方と判定は、テストの中で組み立てた例と `testdata/` の実際の出力の例を直接与えて確かめる。外部のコマンドを起動しない。範囲の補助は `make ext-test` で動く `node --test` のテストで確かめる。
-   **統合テスト（フェーズ 4・5）:** 実際の git の一時的なリポジトリと、テストのバイナリが演じる偽の go・node・npm で `Run` を通す。偽のコマンドを置く `PATH` には実際の go・node・npm を置かないので、道具のテストが実際のテストの一式を実行することはない（AC-35）。Makefile のレシピは `make -n` と偽の go で確かめる。
-   **実際の一式での確認（フェーズ 6）:** ステップ 6-2 のスクリプトを一度実行する。`make test` には入れない（AC-35）。
-   **後方互換:** 既存の `make` の対象は変えない（要件書 4.4）。`TestExtensionDocumentation` の正規表現の変更は、今の一覧の行（引数のない形）も読めることを、既存のテストが今の CLAUDE.md で通ることで確かめる。
-   **壊して確かめる:** 各停止の種類と各不合格のテストは、その判定の処理を外すと失敗することを実装のときに確かめ、コミットメッセージに書く（設計書 7.1、H-07）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 測れていない比較を合格にする（テストの一部だけが動く、報告が空、読めない部分を飛ばす） | テストの削除で実行されなくなった関数を見逃す | 受け入れる形を狭く定めて外れたら止める（フェーズ 1）。`nothing-measured` を `unmatched` より先に判定する（ステップ 2-3）。子のプロセスの環境から `NODE_OPTIONS`・`GOFLAGS` を落とす（ステップ 4-2、`TestChildEnv`） |
| 後始末の漏れ、利用者の状態の変更 | 一時的なディレクトリが残る、利用者のブランチや作業ツリーが変わる | `Run` のどの終わり方でも後始末をし、状態を比べる（ステップ 4-3、`TestRunCleansUp`）。レシピがシグナルを転送する（ステップ 5-2）。シグナルの伝わり方は手動で確かめる（ステップ 6-3） |
| 実際の一式での振る舞いが、偽のコマンドでのテストと違う（node の lcov の形、`--shared` のクローンでのテストの失敗） | 実際の実行で `unreadable` や `test-failed` で止まる | `testdata/` に実際の出力の例を置く（ステップ 1-5）。ステップ 6-2 のスクリプトで実際の一式を通す。止まった場合は原因を調べ、道具を直してから実行し直す |
| node の版を上げて lcov や TAP の形が変わる | 道具が `unreadable` で止まり、使えなくなる | 止まることは合格にならないので見逃しは起きない（設計書 3.5、H-06）。版を上げる変更で道具を直す |
| フェーズ 4 の PR が大きく、レビューの見落としが起きる | 後始末や環境の絞り込みの誤りの見落とし | 実行の部分を `cmd` と Makefile から分けた（1.3）。コミットを、前提の確認・チェックアウトと環境・`Run`・統合テストの順に分ける |
| 予定の遅れ（実際の一式の実行に時間がかかる） | フェーズ 6 の完了が遅れる | 1 回の実行は Go で数分（設計書 3.4 の実測で全体で約 2 分の一式を 2 回）。agent はコマンドの時間の上限を 10 分以上にして実行する（設計書 3.10） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（ステップ 1-1〜1-5）
- [ ] フェーズ 2 完了（ステップ 2-1〜2-4）
- [ ] フェーズ 3 完了（ステップ 3-1〜3-2）
- [ ] フェーズ 4 完了（ステップ 4-1〜4-4）
- [ ] フェーズ 5 完了（ステップ 5-1〜5-3）
- [ ] フェーズ 6 完了（ステップ 6-1〜6-3）

**横断の確認（`make lint` と `make test` では検出できないもの）:**

- [ ] `docs/translation_glossary.md` と本書・設計書の用語（関数表、行の対応、範囲の補助、作業用のディレクトリ、停止の種類）が一致している（PR のレビューで読む。ステップ 6-1）。

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストのファイルは、特に書かない限り `internal/coveragecompare/` の下にある。

| AC | 種類 | 確かめ方 | 実装 |
|---|---|---|---|
| AC-01 | test | `run_test.go::TestRunEndToEnd`（Go と拡張の合格・不合格）、`output_test.go::TestExitCode`、`cmd/coveragecompare/main_test.go::TestRun`。実際の一式: `docs/tasks/0055_coverage_compare/_checks/realrun/main.go`（ステップ 6-3 の記録） | ステップ 4-3・5-1・5-2 |
| AC-02 | test | `precheck_test.go::TestRunPreconditions`（`usage`・`not-ancestor`・`dirty` の行）、`cmd/coveragecompare/main_test.go::TestRun` | ステップ 4-1・5-1 |
| AC-03 | test・manual | `run_test.go::TestRunCleansUp`・`::TestRunDetectsUserStateChange`。シグナルの伝わり方は手動の確認（ステップ 6-3 の記録） | ステップ 4-1・4-2・4-3・5-2 |
| AC-04 | test | `precheck_test.go::TestRunPreconditions`（拡張の `toolchain` の行） | ステップ 4-1 |
| AC-05 | test | `precheck_test.go::TestRunPreconditions`（Go の `toolchain` の行）、`childenv_test.go::TestChildEnv`（`GOTOOLCHAIN=local`・`GOPROXY=off`）、`cmd/yt2column/makefile_test.go::TestCoverageCompareRecipes`（HEAD の `go.mod` の確認） | ステップ 4-1・4-2・5-2 |
| AC-06 | test | `run_test.go::TestRunEndToEnd`（`Toolchain`、同じ `node_modules` へのリンク）、`::TestRunCleansUp`（`.package-lock.json` の変化で `internal`）。実際の一式: `_checks/realrun/main.go`（`toolchain` の行と入っている版の一致。ステップ 6-3 の記録） | ステップ 4-2・4-3 |
| AC-07 | test | `docs/tasks/0055_coverage_compare/_checks/realrun/main.go`（一度だけ実行する。ステップ 6-3 の記録。削除したテストとその理由もここに書く） | ステップ 6-2 |
| AC-08 | test | `gotestjson_test.go::TestParseGoTestJSON`（`fail`・`build-fail`）、`tap_test.go::TestParseTAP`（`# fail`・`not ok`）、`run_test.go::TestRunStopsOnBaseFailure`・`::TestRunCleansUp`（終了コードだけが 0 でない場合）、`output_test.go::TestWrite`（時点とテストの名前） | ステップ 1-2・1-3・4-3 |
| AC-09 | test | `judge_test.go::TestJudge`（テストの数が 0、製品の関数の数が 0。BASE と HEAD のそれぞれ） | ステップ 2-3 |
| AC-10 | test | `compare_test.go::TestCompareGo`（下がった関数、丸めると同じになる減少）、`output_test.go::TestWrite`（`lost` の行の `分子/分母`） | ステップ 2-3・2-4 |
| AC-11 | test | `profile_test.go::TestParseProfile`（複数のテストのバイナリの同じブロックのまとめ方）、`run_test.go::TestRunEndToEnd`（`-coverpkg=./...` を含むテストのコマンド） | ステップ 1-2・4-3 |
| AC-12 | test | `compare_test.go::TestCompareExt` | ステップ 2-3 |
| AC-13 | test | `compare_test.go::TestCompareExt`（HEAD の報告にないファイル）、`classify_test.go::TestClassify`（BASE の報告にないファイル） | ステップ 2-2・2-3 |
| AC-14 | test | `compare_test.go::TestCompareExt`（同じ行の関数の数と実行された数）、`table_test.go::TestBuildExtTable`（同じ行のまとまり） | ステップ 2-1・2-3 |
| AC-15 | test | `table_test.go::TestBuildGoTable`（同じ行の 2 つの宣言） | ステップ 2-1 |
| AC-16 | test | `product_test.go::TestIsGoProduct`、`lcov_test.go::TestParseLCOV`（`src/`・`scripts/` の見分け）、`table_test.go::TestBuildGoTable`・`::TestBuildExtTable`（製品でないファイルの関数を表に入れない） | ステップ 1-2・1-3・2-1 |
| AC-17 | test | `table_test.go::TestBuildExtTable`（`anonymous_<番号>` の変化）、`compare_test.go::TestCompareExt` | ステップ 2-1・2-3 |
| AC-18 | test | `classify_test.go::TestClassify`、`judge_test.go::TestJudgeScenarios` | ステップ 2-2 |
| AC-19 | test | `classify_test.go::TestClassify`（行のずれ）、`judge_test.go::TestJudgeScenarios` | ステップ 2-2 |
| AC-20 | test | `classify_test.go::TestClassify`、`compare_test.go::TestCompareGo`・`::TestCompareExt`（HEAD の報告にない相手） | ステップ 2-2・2-3 |
| AC-21 | test | `classify_test.go::TestClassify`（範囲の外の変更）、`table_test.go::TestBuildGoTable`（関数の宣言の外のブロック） | ステップ 2-1・2-2 |
| AC-22 | test | `compare_test.go::TestCompareGo`・`::TestCompareExt`（`RunState`）、`output_test.go::TestWrite`（`not run at BASE`） | ステップ 2-3・2-4 |
| AC-23 | test | `output_test.go::TestWrite`（各最後の行）、`judge_test.go::TestJudge`（テストの後の順序）、`precheck_test.go::TestRunPreconditions`（テストの前の順序）、`run_test.go::TestRunStopsOnBaseFailure`、`cmd/yt2column/makefile_test.go::TestCoverageCompareRecipes`（レシピが出す最後の行） | ステップ 2-3・2-4・4-1・4-3・5-2 |
| AC-24 | test | `output_test.go::TestWrite`（合格の各項目）、`run_test.go::TestRunEndToEnd`（実行から得た値） | ステップ 2-4・4-3 |
| AC-25 | test | `output_test.go::TestWrite`（不合格と止まったとき） | ステップ 2-4 |
| AC-26 | test | `output_test.go::TestWrite`（抜き書きの上限、出力の末尾の行の数） | ステップ 2-4 |
| AC-27 | test | `profile_test.go::TestParseProfile`（モジュールのパスの除去）、`lcov_test.go::TestParseLCOV`（`extension/` の付加）、`output_test.go::TestWrite` | ステップ 1-2・1-3・2-4 |
| AC-28 | test | `profile_test.go::TestParseProfile` | ステップ 1-2 |
| AC-29 | test | `lcov_test.go::TestParseLCOV`、`testdata_test.go::TestParseRealSamples` | ステップ 1-3・1-5 |
| AC-30 | test | `gotestjson_test.go::TestParseGoTestJSON`、`tap_test.go::TestParseTAP`、`testdata_test.go::TestParseRealSamples` | ステップ 1-2・1-3・1-5 |
| AC-31 | test | `diff_test.go::TestParseDiff`、`run_test.go::TestDiffIgnoresRepoDiffConfig` | ステップ 1-4・4-3 |
| AC-32 | test | `run_test.go::TestRunAcceptsLargeOutput` | ステップ 4-3 |
| AC-33 | test | `judge_test.go::TestJudgeScenarios`（AC-33 の項目ごとに 1 行）。各項目の詳しい場合は、AC-09・AC-08・AC-29・AC-28・AC-10・AC-12・AC-14・AC-20 の行のテスト | ステップ 2-3 |
| AC-34 | test | `judge_test.go::TestJudgeScenarios`（AC-34 の 3 つの場合と、削除した関数の一覧） | ステップ 2-3 |
| AC-35 | test | `run_test.go::TestFakePathHasNoRealToolchain`（偽のコマンドの `PATH` に実際の go・node・npm がない）。道具のテストは `make test`（`internal/coveragecompare`、`cmd/coveragecompare`、`cmd/yt2column`）と `make ext-test`（`extension/test/functionRanges.test.ts`）で動く | ステップ 3-1・4-4 |
| AC-36 | test | `precheck_test.go::TestRunPreconditions`（テストの実行の記録がないこと）、`cmd/yt2column/makefile_test.go::TestCoverageCompareRecipes` | ステップ 4-1・5-2 |
| AC-37 | manual | PR のレビューで、CLAUDE.md の「Deleting a test is a claim that must be checked」が要件書 AC-37 の 5 項目と設計書 3.10 の時間の上限の項目を満たすことを読む | ステップ 6-1 |
| AC-38 | test・manual | `cmd/yt2column/extension_docs_test.go::TestExtensionDocumentation`（`ext-coverage-compare` が「Extension Commands」の一覧にある）、`cmd/yt2column/makefile_test.go::TestCoverageCompareDocumented`（`make coverage-compare` が「Test Commands」にある）。説明の文は PR のレビューで読む | ステップ 5-3 |

設計書が決めたことのうち AC を持たないものは、次のテストで確かめる。この表には行を置かない。

-   子のプロセスの環境（設計書 3.3）: `childenv_test.go::TestChildEnv`。
-   拡張のテストの対象の指定が `extension/package.json` と同じであること（設計書 3.5）: `run_test.go::TestExtTestGlobMatchesPackageJSON`。
-   BASE の値がレシピのシェルの文法として解釈されないこと、環境だけの `BASE` が道具に届かないこと（設計書 3.2・7.3）: `cmd/yt2column/makefile_test.go::TestCoverageCompareRecipes`。
-   範囲の補助の形（設計書 3.5）: `extension/test/functionRanges.test.ts`、`ranges_test.go::TestParseRanges`。

## 8. 成功基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-38 が、7 章の確かめ方で満たされている。
-   **品質:** `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。加えたテストは、対象を壊して失敗することを確かめ、コミットメッセージに記録している。
-   **安全:** 道具がネットワークを使わず（`GOTOOLCHAIN=local`・`GOPROXY=off`、npm は `--version` だけ）、秘密の変数をテストに渡さず（`TestChildEnv`）、BASE の値をシェルの文法として解釈しない（`TestCoverageCompareRecipes`）ことを確かめている。
-   **文書:** CLAUDE.md の規則と「Commands」が書き換わり、本書 7 章の `manual` の行を PR のレビューで読んでいる。ステップ 6-3 の実際の一式での確認とシグナルの確認の記録が本書にある。

## 9. 次のステップ (Next Steps)

-   本書が承認された後、フェーズ 1 から順に `/runplan 0055` で実装する。
-   最後の PR のマージの後、issue #618 を閉じる（`/runpr` が提案する）。#619 の軽量プロセスは、2 つの `make` の対象を類型 D「削除」の確かめ方として使える（設計書 9 章）。
