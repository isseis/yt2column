# 実装計画書：-race 下の巨大入力テストの軽量化と独立したテストの並列化による Go テストの高速化

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-26 |
| Review date | 2026-11-26 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指し、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。用語は設計書の冒頭と要件書 6 章のもの（大きな入力のテスト、大きな出力のテスト、deadcode のテスト、線形の時間のテスト、境界のテスト、対象のテスト、壊した解析、単独の実行、全体の実行、確保量）を同じ意味で使う。`design_handoff.md` の H-01〜H-08 は、設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

本計画は、`make test` の `-race` 付きの実行で時間のかかっている対象のテストを短くする（要件書 2.1）。どのテストもテストのファイルの変更だけで短くし、本番の挙動とテストの検証の強さは保つ（F-004）。

-   大きな入力のテストは、`-race` 付きのビルドの入力を 1 MiB にする（F-001）。
-   大きな出力のテストは、大きな出力を BASE の時点だけで 9 行にする（AC-12）。
-   deadcode のテストと境界のテストは、並列にする（F-002）。

変更の前と後で、壊した解析の確認・カバレッジ・時間を比べ、本計画書に記録する（AC-03、AC-11、AC-13、AC-14）。

### 1.2. 実装の原則

-   変えるのは、`_test.go` のファイル、`//go:build test` のファイル、本タスクのディレクトリの下、ADR-0038 だけである（AC-08、設計書 1.1）。壊した解析は、`break_parse.sh` が一時ディレクトリに展開した木にだけ当て、作業ツリーには入れない。テストが失敗できることを確かめるための壊し方は、作業ツリーに一時的に入れ、確かめた後に戻す。どちらもコミットはしない。
-   時間か確保量を測るテスト（大きな入力のテストと線形の時間のテスト）は、`t.Parallel()` を呼ばない（AC-06、設計書 3.1.4）。
-   変えたテストは、確かめる対象を壊すと失敗することを確かめ、そのことをコミットのメッセージに書く（`CLAUDE.md`「Testing Strategy」、設計書 7.1）。`_checks/` のスクリプトの壊し方は、コミットのメッセージではなく、7 章の一時スクリプトの記録に書く（`docs/dev/conventions/requirements_process.md`「Temporary scripts (`_checks/`)」）。
-   Go のコメント、識別子、文字列リテラルは英語で書く。

### 1.3. 計画の際に決めたこと（2026-11-26）

人間のレビュアーが決めたもの:

-   **PR の分け方:** フェーズを 3 つに分け、実装の PR は 1 本にする。
    -   理由: AC-13・AC-14 の計測では、後のコミットを「前のコミットにこのタスクの変更だけを加えたもの」にする必要がある（設計書 3.6）。PR を分けると、PR の間に `origin/main` の変更が入り、前と後のコミットを組み立て直す手間がかかる。
    -   採らなかった案: PR-1 で大きな入力・大きな出力のテストと ADR を、PR-2 で並列化と計測を行う 2 本の PR。
    -   ADR: なし。本タスクの中で閉じる。
-   **時間の計測（AC-13・AC-14）:** 一時スクリプト `_checks/measure.sh` で行う。スクリプトは、前と後のコミットを交互にチェックアウトし、キャッシュを温める実行・負荷の記録・実時間の計測・AC-14 の判定を行う（設計書 3.6・6.2）。
    -   理由: 前後を交互に 3 回以上測ると、実行は約 60 回になる。手で実行すると、順序の偏りやブランチの戻し忘れが入りうる。
    -   採らなかった案: `/runplan` が設計書 6.2 の手順を手で実行して記録する案。
    -   ADR: なし。本タスクの中で閉じる。
-   **壊した解析の確認（AC-03）:** 一時スクリプト `_checks/break_parse.sh` と、7 つの壊し方の差分のファイル（`_checks/breakages/` の下）で行う。
    -   理由: 7 つの壊し方を前後 2 つのコミットで実行する計 14 回の確認を、同じ手順で繰り返せる。
    -   採らなかった案: `/runplan` が作業ツリーに壊し方を 1 つずつ手で加えて実行し、記録する案。計画では推奨したが、人はスクリプトを選んだ。
    -   ADR: なし。本タスクの中で閉じる。
-   **CI の 3 回の実行（AC-09）:** PR を作った後、エージェントが人の承認を得て、`gh run view` でジョブの ID を読み、`gh run rerun <run-id> --job <job-id>` で `Test` ジョブを 2 回再実行し、3 回の結果を 7.6 節に記録する。`/runplan` には PR を作った後のステップがない。そのため、再実行は、`/runplan` の報告の後に、人が同じセッションのエージェントに指示したときに行う（7.6 節）。
    -   理由: CI は `pull_request` でだけ走る（`.github/workflows/ci.yml:3-5`）ので、PR を作る前には実行できない。再実行はネットワークへの書き込みなので、人の承認が要る。
    -   採らなかった案: レビュアーが GitHub の画面で再実行し、記録も書く案。
    -   ADR: なし。本タスクの中で閉じる。

計画で決めたもの（人間のレビュアーに諮らず、理由とともに記す）:

-   **差分の範囲の確認:** AC-02・AC-04・AC-05・AC-06・AC-07・AC-08・AC-10 は、一時スクリプト `_checks/diff_scope.sh` が、マージベースと後のコミットを比べて確かめる。変わるのが Go のコードなので、記録した差分を読む方法（文書の小さな変更のためのもの）ではなくスクリプトにする（`requirements_process.md`「`diff` verification」）。ADR: なし。
-   **スクリプトの木の作り方:** `break_parse.sh` は、前と後のコミットを `git archive` で一時ディレクトリに展開し、そこで壊し方を当てて実行する。作業ツリーを変えないためである。`measure.sh` は、作業ツリーで前と後のコミットをチェックアウトする。設計書 3.6・6.2 が、それぞれのコミットをチェックアウトして測ると決めているためである。エージェントは `git worktree add` を使わない（`docs/dev/guides/chief_guide.md` 6 節）。ADR: なし。
-   **大きな出力のテストの壊し方:** ツールの読み取りの経路に、1 行の上限（1 MiB 未満）と、全体の上限（8,388,608 バイトを超える出力を拒む）を、1 つずつ一時的に加える。どちらでもテストが失敗することを確かめる（設計書 7.1）。全体の上限をちょうど 8 MiB にするのは、テストが失敗することで、出力が 8 MiB を超えることも確かめられるためである。ADR: なし。
-   **フェーズの順:** 設計書 8 章の優先順位のとおりにする。並列化（ステップ 2-2・2-3）は、テストのコードを変える最後のステップである。その後のフェーズ 3 は、確認と計測のスクリプトと記録を足すだけである。差分の範囲のスクリプト（ステップ 3-3）を最後にするのは、AC-08 の変えたファイルの一覧に、ほかのスクリプトと記録を含めるためである。ADR: なし。
-   **ステップの書き方:** 書き方 A（`- [ ] **ステップ N-M**`）で書く。chief が確実に読める書き方だからである（タスク 0113 の要件書 1 章）。ADR: なし。

### 1.4. 既存コード調査結果

コミット `7fbf234e`（`origin/main`）で確かめた。設計書が確かめた `aa9274fb` から、`internal/transcript`・`internal/coveragecompare`・`cmd/yt2column/deadcode_test.go`・`internal/ytdlpsource`・`internal/strictjson`・`internal/publisher/display_render_test.go`・ADR-0038・`Makefile` を変えたコミットはない（`git log aa9274fb..7fbf234e -- <これらのパス>` の出力が空）。そのため、設計書の `file:line` はこのコミットでも同じ行を指す。

-   **大きな入力のテスト:**
    -   `copiedLargeInputLimit` を参照するのは、`internal/transcript/copied_format_test.go` の 3 か所だけである。定義（`:1043`）、7 つの入力の表（`:1067-1073`）、大きすぎないことの確認（`:1077-1078`）である（`grep -rn copiedLargeInputLimit internal/transcript/`）。
    -   4 MiB と書いたコメントは、`:1024`（記事生成モジュールの入力の上限。変えない）、`:1047`、`:1049` にある。`:1040-1041` は、ビルドごとに定める定数の説明である。
    -   7 つの入力を作る関数（`:1108-1146`）は、大きさを引数に取る。関数そのものは変えなくてよい。
    -   ビルドタグのファイル（`copied_format_race_test.go`・`copied_format_norace_test.go`）は、どちらも `const` のブロックを 1 つ持つ。
    -   `copied_format_race_test.go` のコメントにあるとおり、`make lint` は `race` のタグのファイルをコンパイルしない（`copied_format_race_test.go:18-21`）。
    -   `internal/transcript/makefile_test.go` の `TestMakeTestParseNorace` は、テストの名前と `make test-parse-norace` の引数を確かめる。本タスクは、そのどちらも変えない。
-   **壊した解析:** `internal/transcript/copied_format.go` は本番のコードで、本タスクでは変えない。そのため、前と後のコミットで同じ内容であり、同じ壊し方の差分を当てられる。壊す場所は設計書 3.1.3 の表のとおりである。
-   **大きな出力のテスト:**
    -   `LargeStdout` を立てるのは、`TestRunAcceptsLargeOutput`（`internal/coveragecompare/run_test.go:851-854`）だけである。このテストは、`b.Suites` のすべての時点（`"base"` と `"head"`、`run_test.go:58-61`）に立てる。
    -   大きな出力は、`test_helpers.go:433-440` が `fakeLargeLines` 行だけ書く。
    -   ツールは、子のプロセスの標準出力を `cmdrunner` でまとめて受け取り（`internal/coveragecompare/target.go:79-97` の `runSuite`）、`parseGoTestJSON`（`internal/coveragecompare/gotestjson.go:57`）が `splitLines` で行に分ける。読み取りに `bufio.Scanner` は使っていない（`grep -n 'bufio' internal/coveragecompare/*.go` が当たるのは `test_helpers.go:125` のコメントだけ）。
-   **deadcode のテスト:**
    -   `t.Setenv`・`t.Chdir`・`os.Setenv` を呼ぶ `cmd/yt2column` のファイルは、`outpath_test.go`・`run_test.go`・`main_test.go` で、`deadcode_test.go` にはない（`grep -rln`）。扱いは設計書 3.3 のとおりである。
    -   `deadcodeScope`（`deadcode_test.go:610-617`）のフィールドは作った後に書かれない。メソッドは読むだけである（`:621-641`）。
    -   `isolation` のフィクスチャは、親の `t.TempDir()` の下に作られる（`deadcode_test.go:720`、`:853`）。`t.TempDir()` を消す Cleanup は、並列にしたサブテストがすべて終わってから走る（`go doc testing.T.Cleanup`）。
-   **境界のテスト:**
    -   上限は定数である（`internal/ytdlpsource/json3.go:14`・`:16`、`internal/strictjson/strictjson.go:25`）。
    -   `internal/ytdlpsource` で `t.Setenv` などを呼ぶのは、`exec_test.go`・`ytdlp_test.go`・`test_helpers.go` である。対象の 3 つのテストは呼ばない。並列のトップレベルのテストは直列のトップレベルのテストが終わってから再開するので、これらと重ならない（`go doc testing.T.Parallel`）。`internal/strictjson` には、呼ぶファイルがない。
-   **Lint:** `.golangci.yml` は `paralleltest` も `tparallel` も有効にしていない（`.golangci.yml:7-30`）。`t.Parallel()` の付け方を lint は確かめない。
-   **CI:** `Test` ジョブは `make test-ci` と `make test-parse-norace` を実行する（`.github/workflows/ci.yml:150-169`）。CI は `pull_request` でだけ走る（`ci.yml:3-5`）。`gh run rerun` の `--job` は、`gh run view <run-id> --json jobs` の `databaseId` を取る（`gh run rerun --help`、gh の手元の版で確かめた）。
-   **ADR-0038:** 「食い違い」の項は、`## 決定` の 5 つ目の項の下にある（`docs/dev/adr/0038-deadcode-and-race-free-timing.md:24`）。
-   **計測の機械:** 手元の機械は Apple M3 Pro（12 コア）、メモリ 18 GiB、`go1.27.1 darwin/arm64` である。`deadcode` は `PATH` にある（`which deadcode`）。設計書の計測の機械と同じである。
-   新しいテストのヘルパーは要らない（`docs/dev/conventions/test_organization.md`）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 壊した解析と大きな入力のテスト（設計書 3.1・3.8、AC-01〜AC-03、AC-06）

- [x] **ステップ 1-1**: 壊した解析のスクリプトと、変更の前の確認
    - 追加するファイル: `docs/tasks/0129_race_test_speedup/_checks/break_parse.sh`、`_checks/breakages/` の下の 7 つの差分のファイル
    - 差分のファイルは、設計書 3.1.3 の表の経路と単位（行や見出しの行など、経路が 1 回ずつ処理するまとまり）ごとに 1 つ作る。単位の位置から入力の終わりまで（trailing zero-length lines では入力の始まりまで）のバイト数と同じ長さのバイト列を、中身を写さずに確保する
    - スクリプトは、与えられたコミットを `git archive` で一時ディレクトリに展開する。そこで差分を 1 つずつ当て、その入力のサブテストだけを `-race` 付きで実行する。確保量の確認のエラーで失敗したものだけを「失敗した」と数える（設計書 3.1.3「失敗の理由の確認」）。壊さない木でも同じサブテストを実行し、通ることを確かめる
    - `-timeout` は、4 MiB の one long form A line の壊し方を先に 1 回測ってから決める（設計書 3.1.3）。1 MiB で 38 秒なので、4 MiB では約 10 分かかる見込みである。この計測とスクリプトの実行は、バックグラウンドで行う
    - スクリプトをコミットしてから、各判定が少なくとも 1 つの壊し方で失敗することを確かめ、7.1 節に書く
    - `git fetch origin main` の後に、マージベース（変更の前のコミット）で実行し、出力を 7.1 節に書く。7 つのどれかが失敗しなければ、壊し方を直す（設計書 6.1）
    - 完了の条件: スクリプトがコミットされ、7.1 節に壊し方の記録と変更の前の実行の記録がある
- [x] **ステップ 1-2**: `-race` 付きのビルドの入力を小さくする
    - 変えるファイル: `internal/transcript/copied_format_test.go`、`copied_format_race_test.go`、`copied_format_norace_test.go`
    - 設計書 3.1.1 のとおり、入力の大きさを決める定数 `copiedLargeInputSize` をビルドタグのファイルに置く。`-race` 付きでは `1 << 20` にし、`-race` なしでは `copiedLargeInputLimit` にする。`-race` 付きのファイルの doc コメントには、1 MiB を選んだ根拠（設計書 3.1.2 の壊した解析の結果）を書く
    - 7 つの入力の表と、大きすぎないことの確認で、`copiedLargeInputLimit` を `copiedLargeInputSize` に替える。入力を作る関数と `wantSegments` の式の形は変えない
    - 4 MiB と書いたコメント（`:1047`、`:1049`）と、ビルドごとに定める定数の説明（`:1040-1041`）を、ビルドごとの大きさに合わせて直す。`:1024` は変えない。コメントには、設計書 3.1.2 で測った値を書き、AC や F の番号は書かない。PR のレビューで、その値を設計書 3.1.2 と比べて読む
    - 入力を作る関数の 1 つを、`copiedLargeInputSize` を 1 バイト超える入力を返すように一時的に壊し、`-race` 付きのビルドで大きすぎないことの確認が失敗することを確かめる。コミットのメッセージにそのことを書く
    - `make test-parse-norace` と、`-race` 付きの `go test -tags test -race -count=1 -v -run '^TestParseCopiedLargeInputs$' ./internal/transcript` を通す。後者のログの、7 つの入力のバイト数の行を 7.4 節に書く（AC-01）
    - コミットの後に、`break_parse.sh` をそのコミットでバックグラウンドで実行し、出力を 7.1 節に書く（AC-03）。7 つのどれかが失敗しなければ、作業を止めて人に知らせる。大きさか壊し方の見直しは、設計に戻ることになる（設計書 6.1）
    - 完了の条件: 2 つの実行が通り、7.1 節・7.4 節に記録がある
- [x] **ステップ 1-3**: ADR-0038 の食い違いの項を消す
    - 変えるファイル: `docs/dev/adr/0038-deadcode-and-race-free-timing.md`
    - 設計書 3.8 のとおり、`git grep -nE 'issues/1166([^0-9]|$)' origin/main -- docs/dev/adr/` で項を探す。決定の中の、この食い違いの項だけを消す。決定の文は変えず、経緯の行も加えない
    - コミットのメッセージに訂正の行（`Editorial correction: `）を書き、PR の説明に訂正の段落を書く（`requirements_process.md`「Recording in the PR description」）
    - 完了の条件: 項が消え、訂正の行を含むコミットがある

### フェーズ 2: 大きな出力のテストと並列化（設計書 3.2〜3.4、AC-04・AC-05・AC-10・AC-12）

- [x] **ステップ 2-1**: 大きな出力のテストを小さくする
    - 変えるファイル: `internal/coveragecompare/run_test.go`、`internal/coveragecompare/test_helpers.go`
    - 設計書 3.2 のとおり、`fakeLargeLines` を 9 にし、`LargeStdout` を `"base"` の時点のスイートだけに立てる。確かめること（合格で、BASE のテストが 2 件）は変えない
    - テストの doc コメントと `fakeLargeLines` のコメントの「over 20 MiB」を、「over 8 MiB」と BASE の時点だけで出すことに直す。AC の番号は書かない。PR のレビューで、その値を設計書 3.2 の計算と比べて読む
    - 1.3 節の 2 つの壊し方（1 行の上限、全体の上限）を 1 つずつ入れ、それぞれでテストが失敗することを確かめる。コミットのメッセージにそのことを書く
    - 完了の条件: `go test -tags test -race -run '^TestRunAcceptsLargeOutput$' ./internal/coveragecompare` が通る
- [x] **ステップ 2-2**: deadcode のテストを並列にする
    - 変えるファイル: `cmd/yt2column/deadcode_test.go`
    - 設計書 3.3 の「並列にするもの」のとおり、2 つのテスト関数とすべてのサブテストの先頭に `t.Parallel()` を加える。それ以外は変えない（AC-05）
    - 各テスト関数について、確かめる対象を 1 つ壊し（たとえば `Makefile` の `deadcode` のレシピ）、テストが失敗することを確かめる。コミットのメッセージにそのことを書く
    - 完了の条件: `go test -tags test -race -count=1 -run '^TestMakeDeadcode(Verdict|Scope)$' ./cmd/yt2column` が、`deadcode` のある手元の機械で通る
- [x] **ステップ 2-3**: 境界のテストを並列にする
    - 変えるファイル: `internal/ytdlpsource/json3_test.go`、`internal/strictjson/strictjson_test.go`
    - 設計書 3.4 のとおり、3 つのテスト関数とすべてのサブテストの先頭に `t.Parallel()` を加える。入力の作り方と確かめる値は変えない（AC-10）
    - 各テスト関数について、確かめる上限を 1 つ壊し（たとえば上限の定数を一時的に変える）、テストが失敗することを確かめる。コミットのメッセージにそのことを書く
    - 完了の条件: 2 つのパッケージの `go test -tags test -race -count=1` が通る。並列にしたテストの安定は、ステップ 3-1 の全体の実行 3 回と CI の 3 回で確かめる（AC-09）

### フェーズ 3: 変更の後の確認と計測（設計書 3.6・3.7・7.1、AC-02・AC-04〜AC-11・AC-13・AC-14）

- [x] **ステップ 3-1**: 変更の後のテストの実行とカバレッジ
    - `go test -tags test -race -count=1 ./...` を 3 回続けて実行し、`make test` と `make lint` を通す。それぞれの終了コードと、実行したコミットを 7.5 節に書く（AC-09）。1 回の全体の実行は約 5 分かかる（設計書 2.1）ので、バックグラウンドで実行する
    - `make coverage-compare BASE=<マージベース>` を、バックグラウンドか 10 分以上の時間制限で実行する（`CLAUDE.md`「Testing Strategy」）。最後の行が `coverage-compare: PASS` であることを確かめる。`FAIL` のときは、設計書 3.7 のとおり、変更のない作業ツリーで実行し直し、すべての実行を記録する
    - 設計書 3.7 のとおり、大きな入力のテストだけを `-race` 付きで実行したプロファイルを、前と後のコミットで取る。前のコミットのプロファイルは、マージベースを `git archive` で一時ディレクトリに展開した木で取り、作業ツリーをチェックアウトしない。`go tool cover -func` の出力を比べ、どの関数の割合も下がっていないことを確かめる
    - 2 つの比較の出力と、それぞれの実行したコミットを 7.7 節に書く（AC-11）。7.5 節・7.7 節の記録をコミットしてから、次のステップに進む（`measure.sh` は変更のない作業ツリーでだけ始まる）
    - 完了の条件: 7.5 節・7.7 節に記録があり、すべて通っている
- [ ] **ステップ 3-2**: 時間の計測のスクリプトと計測
    - 追加するファイル: `docs/tasks/0129_race_test_speedup/_checks/measure.sh`
    - スクリプトは、設計書 3.6・6.2 のとおりに計測する。前と後のコミットを交互にチェックアウトし、温める実行を記録から外し、各回の前に負荷を記録する。対象のテストの単独の実行と全体の実行を、それぞれ 3 回以上測る。設計書 3.6 の記録の項目を出力し、AC-14 の判定を出す
    - スクリプトは、変更のない作業ツリーでだけ始める。出力は作業ツリーの外のファイルに書く。チェックアウトで自分のファイルが消えても最後まで動き、どの終わり方でも元のブランチに戻す。`deadcode` が `PATH` にないとき、どれかの実行が失敗したとき、値を読めないときは、判定を通ったと報告しない
    - スクリプトをコミットしてから、各判定が少なくとも 1 つの壊し方で失敗することを確かめ、7.3 節に書く。スクリプトは変更のない作業ツリーでだけ始まるので、壊し方は作業ツリーの外に写したスクリプトに入れて実行する。確認のために、対象のテストと回数を減らして実行できるようにする。元のブランチに戻ることも、この確認で確かめる
    - 計測の前に、同じ機械でほかの重い処理が走っていないかを人に尋ねる。答えを待ち、走っていないという答えを得てから計測する。答えを推測しない。人の答えを 7.3 節に書く。`/runplan` のセッションからは、ほかの workspace の負荷を確かめられないためである
    - バックグラウンドで実行する。実行の間（約 1 時間）は、作業ツリーのファイルを変えず、作業ツリーかブランチを変える git のコマンドも実行しない。終わるのを待ってから記録を書く
    - スクリプトが強制終了などで止まり、元のブランチに戻れなかったときは、次の作業の前に `git status` で現在のブランチを確かめ、このタスクのブランチに戻す。新しいセッションで `/runplan` を続けるときも、始める前に同じことを確かめる。戻さずに `/runplan` を始めると、別のブランチを作ってしまう
    - 出力と 2 つのコミットを 7.0 節に書く
    - AC-14 の判定で短くならなかった対象のテストがあれば、結果を記録して作業を止め、人に知らせる。扱いは設計に戻って決め直す（要件書 AC-14）
    - 完了の条件: 7.0 節・7.3 節に記録があり、AC-14 の判定が通っている
- [ ] **ステップ 3-3**: 差分の範囲のスクリプト
    - 追加するファイル: `docs/tasks/0129_race_test_speedup/_checks/diff_scope.sh`
    - スクリプトは、マージベースと後のコミットの差分で、7.2 節の判定を行う。判定できないときは、通ったと報告しない
    - スクリプトをコミットしてから、各判定が少なくとも 1 つの壊し方で失敗することを確かめ、7.2 節に書く
    - `git fetch origin main` の後に実行し、出力と 2 つのコミットを 7.0 節に書く。ほかのスクリプトを足し終えた後に実行するので、AC-08 の変えたファイルの一覧は PR の内容と同じになる
    - ADR-0038 の差分を記録した差分の読み取りの形で 7.0 節に書く（ステップ 1-3）
    - 完了の条件: すべての判定が通り、7.0 節・7.2 節に記録がある

PR を開く前の上位モデルのレビューで、テストのファイル、`_checks/` のスクリプト、壊し方の差分を直したときは、その直しが影響する記録を、PR を開く前に取り直す。取り直す対象は、`break_parse.sh` の後のコミットでの実行と 7.4 節（`internal/transcript` を直したとき）、7.5 節・7.7 節（テストを直したとき）、`measure.sh`（対象のテストかスクリプトを直したとき。計測の前の人への問いも含む）、`diff_scope.sh`（どの直しでも）である。

### PR-1 作成ポイント: race test speedup and its verification records

**対象ステップ**: 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3

**推奨タイトル**: `test(0129): -race 下の巨大入力テストを軽くし、独立したテストを並列にする`

**レビュー観点**: 並列にしたテストが書き換えられる共有の状態を持たないこと（設計書 3.3・3.4） / 壊した解析と壊し方の確認が、確保量の確認の失敗だけを数えていること / 3 つの一時スクリプトが fail closed で、作業ツリーを元に戻すこと / 7 章の記録（マージの前に書く 7.6 節の CI の 3 回を含む）が AC-03・AC-09・AC-11・AC-13・AC-14 を満たしていること

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 3-2 の `measure.sh` が作業ツリーのチェックアウトと元への戻しを伴い、ステップ 1-1 の `break_parse.sh` が失敗の理由を見分ける必要があり、ステップ 2-2・2-3 の並列化が並行性に関わる。`high` 役割より軽いモデルが実装すると、途中で止まったときにブランチを戻し損ねて作業ツリーを前のコミットに残すか、タイムアウトやセグメントの数による失敗を確保量の失敗と数えて AC-03 を満たしたように見せるか、共有の状態を見落として結果の不安定なテストを持ち込むおそれがある

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `break_parse.sh` と、前後の確認の記録。1 MiB の入力にした大きな入力のテスト。ADR-0038 の食い違いの項の削除 |
| M2（フェーズ 2） | 9 行・BASE だけにした大きな出力のテスト。並列にした deadcode のテストと境界のテスト |
| M3（フェーズ 3） | `diff_scope.sh`・`measure.sh` とその記録。AC-09・AC-11 の記録 |

フェーズの順は、設計書 8 章の優先順位（変更の前の確認 → 大きな入力のテストと ADR → 大きな出力のテスト → 並列化 → 変更の後の確認）のとおりである。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 | 大きな入力・大きな出力のテストの縮小、deadcode のテストと境界のテストの並列化、ADR-0038 の項の削除、3 つの一時スクリプトと前後の確認・計測の記録 | standard+high-review | auto |

## 4. テスト戦略 (Test Strategy)

-   新しいテスト関数は加えない。変えるのは、対象のテストの入力の大きさ・時点・`t.Parallel()` の呼び出しだけである（1.2 節）。
-   変えたテストが失敗できることは、各ステップの壊し方で確かめ、コミットのメッセージに書く（ステップ 2-1〜2-3）。大きな入力のテストは `break_parse.sh` で確かめる（AC-03）。
-   変更の前と後の関係（入力が同じ、差分が `t.Parallel()` だけ、変えたファイルの範囲）は、`diff_scope.sh` で確かめる。
-   本番のコードのカバレッジは、`make coverage-compare` と、`-race` 付きのビルドのプロファイルの比較で確かめる（AC-11）。
-   安定は、手元の全体の実行 3 回と、CI の `Test` ジョブ 3 回で確かめる（AC-09）。線形の時間のテストの安定も、これで確かめる（設計書 7.1）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 壊した解析が、確保量ではなくタイムアウトやセグメントの数で失敗し、AC-03 を満たしたように見える | `break_parse.sh` は、確保量の確認のエラーで失敗したものだけを数える（設計書 3.1.3） |
| 4 MiB の one long form A line の壊し方が、`go test` のタイムアウトに先に当たる | 先に 1 回測ってから `-timeout` を決める（ステップ 1-1） |
| 並列にしたテストが共有の状態を持ち、まれにしか出ない失敗を持ち込む | 設計書 3.3・3.4 の確認のとおりにだけ並列にする。手元と CI でそれぞれ 3 回実行する（AC-09）。PR を開く前に、上位モデルのレビューを行う（3.2 節） |
| `measure.sh` のチェックアウトが、途中で止まったときに作業ツリーを前のコミットに残す | スクリプトは、変更のない作業ツリーでだけ始め、どの終わり方でも元のブランチに戻す。実行の間、エージェントは作業ツリーを変えない（ステップ 3-2）。壊し方の確認で、戻すことも確かめる |
| PR を開く前の上位モデルのレビューの直しで、記録が古くなる | フェーズ 3 の終わりの段落のとおり、直しが影響する記録を、PR を開く前に取り直す |
| 機械の負荷のばらつきで、差の小さい `TestParseSubtitlesLimits` が AC-14 を満たさない | 計測の前に重い処理がないことを人に確かめる。満たさなければ止めて設計に戻る（要件書 AC-14、設計書 2.1） |
| 並列にしたテストで、CI のメモリが足りなくなる | 設計書 3.4 の試作では小さい。CI の 3 回の実行で確かめる（AC-09） |
| PR のレビューの直しか `origin/main` の取り込みで、テストが変わり、記録が古くなる | 変えた人が、7 章の記録のうち影響を受けるもの（スクリプトの実行、7.4 節〜7.7 節）を実行し直して書き直す（`requirements_process.md`「Temporary script record」の「Rewriting」） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 を作った後、マージの前に、7.6 節の CI の 3 回の実行を記録した
- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`diff` の行の結果（スクリプトの出力と 2 つのコミットの ID、記録した差分）は 7.0 節に置く。7.1〜7.3 節の実行の記録は、そこを指す。

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | `internal/transcript/copied_format_test.go::TestParseCopiedLargeInputs` の `-race` 付きの実行。7 つの入力のバイト数の行を 7.4 節に書く。大きすぎないことの確認が失敗できることは、ステップ 1-2 のコミットのメッセージに書く |
| AC-01 | `manual` | PR のレビューで、設計書 3.1.2 に、1 MiB とその大きさでの `-race` 付きの実行時間があることを読む |
| AC-02 | `diff` | `_checks/diff_scope.sh` の「`-race` なしの入力」の判定。結果は 7.0 節 |
| AC-02 | `test` | `make test-parse-norace`（期限 3 秒と 1 バイトあたり 128 バイトの確認を含む） |
| AC-03 | `test` | `_checks/break_parse.sh` を前と後のコミットで実行する。記録は 7.1 節 |
| AC-04 | `diff` | `_checks/diff_scope.sh` の「並列化」の判定。結果は 7.0 節 |
| AC-04 | `test` | 7.5 節の `-race` 付きの全体の実行 3 回 |
| AC-05 | `diff` | `_checks/diff_scope.sh` の「`t.Parallel()` だけ」の判定（`cmd/yt2column/deadcode_test.go`）。結果は 7.0 節 |
| AC-06 | `test` | `_checks/diff_scope.sh` の「測るテストは直列」の判定。後のコミットの状態を確かめる。結果は 7.0 節 |
| AC-07 | `diff` | `_checks/diff_scope.sh` の「変えないファイル」の判定（`internal/publisher/display_render_test.go`）。結果は 7.0 節 |
| AC-07 | `manual` | PR のレビューで、設計書 3.5 に小さくしない理由があることを読む |
| AC-08 | `diff` | `_checks/diff_scope.sh` の「変えたファイルの範囲」の判定。結果は 7.0 節 |
| AC-08 | `diff` | ADR-0038 の差分を記録した差分の読み取り（比べるパス: `docs/dev/adr/0038-deadcode-and-race-free-timing.md`）。食い違いの項だけが消え、決定の文が変わっていないことを PR のレビューで読む（設計書 3.8）。記録は 7.0 節 |
| AC-09 | `test` | 手元の `-race` 付きの全体の実行 3 回、`make test`、`make lint`（7.5 節）。CI の `Test` ジョブ 3 回（7.6 節） |
| AC-10 | `diff` | `_checks/diff_scope.sh` の「`t.Parallel()` だけ」の判定（`internal/ytdlpsource/json3_test.go`、`internal/strictjson/strictjson_test.go`）。結果は 7.0 節 |
| AC-11 | `test` | `make coverage-compare BASE=<マージベース>`。出力は 7.7 節 |
| AC-11 | `test` | 大きな入力のテストの `-race` 付きのプロファイルの前後の比較（設計書 3.7、ステップ 3-1）。2 つの出力と実行したコミットは 7.7 節 |
| AC-12 | `test` | `internal/coveragecompare/run_test.go::TestRunAcceptsLargeOutput`。1.3 節の 2 つの壊し方で失敗することを、ステップ 2-1 のコミットのメッセージに書く |
| AC-13 | `test` | `_checks/measure.sh` の計測。結果は 7.0 節 |
| AC-13 | `manual` | PR のレビューで、7.0 節に、機械、コマンド、各回の結果があることを読む |
| AC-14 | `diff` | `_checks/measure.sh` の AC-14 の判定。結果は 7.0 節 |

### 7.0. `diff` の行の結果

#### `diff_scope.sh`（AC-02・AC-04〜AC-08・AC-10）

（`/runplan` が差分の範囲の確認で書く。マージベースと後のコミットの完全な ID、スクリプトの出力の全体）

#### `measure.sh`（AC-13・AC-14）

（`/runplan` が時間の計測で書く。重い処理がないという人の答え、前と後のコミットの完全な ID、スクリプトの出力の全体）

#### ADR-0038 の記録した差分（AC-08）

Diff record (AC-08):

- Compared paths: `docs/dev/adr/0038-deadcode-and-race-free-timing.md`

### 7.1. 一時スクリプトの記録（`_checks/break_parse.sh`）

-   **スクリプト:** `docs/tasks/0129_race_test_speedup/_checks/break_parse.sh` と、`_checks/breakages/` の下の 7 つの差分のファイル。与えられたコミットの木で、7 つの壊し方を 1 つずつ当て、壊した入力のサブテストが確保量の確認で失敗することを確かめる。AC-03 を確かめる。
-   **判定:**
    -   壊さない木で、7 つのサブテストが通ること
    -   7 つの壊し方のそれぞれで、その入力のサブテストが確保量の確認のエラーで失敗すること
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる。壊し方は、コミットした内容の上で作業ツリーに入れ、確かめた後に戻す。
-   **予定の実行:**
    -   変更の前の確認として、`/runplan` がマージベース（変更の前のコミット）で 1 回実行する。
    -   大きな入力のテストを小さくしたコミットの後に、`/runplan` がそのコミットで 1 回実行する。
    -   その後にスクリプトか大きな入力のテストが変わったとき（PR を開く前の上位モデルのレビューの直しを含む）は、変えた人が後のコミットで実行し直す。前のコミットでの実行も、スクリプトが変わったときは実行し直す（`requirements_process.md`「Temporary script record」の「Rewriting」）。
-   **テスト:** コミットしない。
-   **壊し方の記録:** スクリプトの内容は、コミット `d5e4b8f49815512ec30379a30392797248a716b6`（後の実行と同じ内容）である。そのコミットで、`go test` の `-timeout` を 1 秒にして実行した（`break_parse.sh <commit> 1s`）。
    -   「壊さない木で 7 つのサブテストが通ること」の判定: 壊さない木の実行がタイムアウトで失敗し、`unbroken: FAIL` と `RESULT: FAIL` になった。
    -   「確保量の確認のエラーで失敗すること」の判定: 7 つの壊し方すべてがタイムアウトで失敗し（`parse allocated` の行がない）、7 つとも `NOT CAUGHT by the allocation check` になって数えられなかった。タイムアウトなどの別の理由の失敗を数えないことの確認である。
    -   終了コードは 1 だった。
-   **実行の記録:** 後の実行はコミット `d5e4b8f49815512ec30379a30392797248a716b6`（`origin/main` の `51f82a71` に本タスクの変更を加えたもの）で、前の実行はマージベース `51f82a7109d33691afb287bb03f4e68d2e628b38` で行った。どちらも `-timeout 90m`、`go1.27.1 darwin/arm64` である。どちらも壊さない木で 7 つのサブテストが通り（`unbroken: PASS`）、7 つの壊し方のすべてが確保量の確認のエラー（`parse allocated … want at most 8192 per byte`）で失敗し、`RESULT: PASS (unbroken passes; 7 of 7 caught)` で終わった。2 つの実行は並行して行った（時間は参考値）。
    -   前（4 MiB）の最初の実行は、実行の間に作業ツリーをほかのコミットに切り替えたため、壊し方のファイルが読めなくなり、4 つ目以降が `cannot apply` になった。`break_parse.sh` が壊し方を一時ディレクトリに写すよう直し、前と後の両方を取り直した。下の表は取り直した実行の結果である。

| 壊し方 | 前の時間 | 前の確保量 / 入力のバイト数（1 バイトあたり） | 後の時間 | 後の確保量 / 入力のバイト数（1 バイトあたり） |
|---|---:|---:|---:|---:|
| 01_shortest_form_a_lines | 41 秒 | 809,125,706,776 / 4,194,300（192,910） | 5 秒 | 52,345,690,720 / 1,048,575（49,920） |
| 02_one_long_form_a_line | 571 秒 | 17,626,323,563,240 / 4,194,304（4,202,443） | 52 秒 | 1,107,887,411,264 / 1,048,576（1,056,563） |
| 03_textless_form_b_timestamps | 81 秒 | 1,771,572,788,784 / 4,194,301（422,376） | 9 秒 | 113,040,151,136 / 1,048,576（107,803） |
| 04_trailing_zero_length_lines | 424 秒 | 8,813,234,618,976 / 4,194,304（2,101,238） | 35 秒 | 553,962,098,840 / 1,048,576（528,299） |
| 05_one_byte_prefix_lines | 349 秒 | 4,406,621,536,832 / 4,194,304（1,050,620） | 20 秒 | 276,982,221,816 / 1,048,576（264,150） |
| 06_form_a_headings | 12 秒 | 278,242,468,392 / 4,194,304（66,338） | 3 秒 | 17,982,370,360 / 1,048,576（17,149） |
| 07_form_b_headings | 189 秒 | 4,406,600,566,096 / 4,194,303（1,050,615） | 21 秒 | 276,976,896,488 / 1,048,575（264,146） |


### 7.2. 一時スクリプトの記録（`_checks/diff_scope.sh`）

-   **スクリプト:** `docs/tasks/0129_race_test_speedup/_checks/diff_scope.sh`。マージベースと後のコミットの差分を読み、次の判定を行う。AC-02・AC-04・AC-05・AC-06・AC-07・AC-08・AC-10 を確かめる。
-   **判定:**
    -   **`-race` なしの入力（AC-02）:** 後の `copied_format_test.go` で `copiedLargeInputSize` を `copiedLargeInputLimit` に置き換え、コメントの行を除くと、前の内容と同じになる。`copied_format_norace_test.go` は、`copiedLargeInputSize = copiedLargeInputLimit` を加えたことのほかは、コメントの行を除いて前と同じである（`copiedParseAllocPerByte = 128` と `copiedParseChecksDeadline = true` が変わらない）。`copiedParseDeadline` の定義が変わらない
    -   **並列化（AC-04）:** 5 つのテスト関数（`TestMakeDeadcodeScope`、`TestMakeDeadcodeVerdict`、`TestParseSubtitlesLimits`、`TestParseSubtitlesArrayElementLimit`、`TestAsArrayElementLimit`）の本体と、その中のすべての `t.Run` に渡す関数で、`t.Parallel()` を呼ぶ
    -   **`t.Parallel()` だけ（AC-05・AC-10）:** `cmd/yt2column/deadcode_test.go`、`internal/ytdlpsource/json3_test.go`、`internal/strictjson/strictjson_test.go` の差分は、`t.Parallel()` の行を加えることだけである
    -   **測るテストは直列（AC-06）:** 後のコミットの `TestParseCopiedLargeInputs` と `TestNeutralizedDisplayMarkdownLinearTime` が、`t.Parallel()` を呼ばない
    -   **変えないファイル（AC-07）:** `internal/publisher/display_render_test.go` が変わっていない
    -   **変えたファイルの範囲（AC-08）:** 変わったファイルのすべてが、`_test.go` のファイル、最初の行が `//go:build test` と一致するファイル（加えたか変えたファイルは後の内容で、消したファイルは前の内容で読む。読めないときは通ったとしない）、`docs/tasks/0129_race_test_speedup/` の下、`docs/dev/adr/0038-deadcode-and-race-free-timing.md` のどれかである
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる。壊し方は、コミットした内容の上で作業ツリーに入れ、確かめた後に戻す。
-   **予定の実行:**
    -   PR を作る前に、`/runplan` が差分の範囲の確認で 1 回実行する。ほかのスクリプトと記録を足し終えた後である。
    -   その後に比べるファイルかスクリプトが変わったとき（PR を開く前の上位モデルのレビューの直し、PR のレビューの直し、`origin/main` の取り込み）は、変えた人が実行し直す。
-   **テスト:** コミットしない。
-   **壊し方の記録:** スクリプトの内容は、コミット `bf95ee07112a1fcf9589e5d72c9a354eac41eeac` である。スクリプトは Git のオブジェクトを読むので、壊し方は一時的なコミットとして入れ（`--no-verify`）、実行した後に `git reset --hard` で元のコミットに戻した。
    -   `-race` なしの入力（`copied_format_test.go`）: `3 * time.Second` を `4 * time.Second` に変えた。`FAIL: AC-02 … differs from before beyond the constant's name`。
    -   `-race` なしの入力（`copied_format_norace_test.go`）: `copiedParseAllocPerByte` を 129 にした。`FAIL: AC-02 … differs from before: < copiedParseAllocPerByte = 128`。
    -   並列化: `TestAsArrayElementLimit` の 1 つのサブテストの `t.Parallel()` を消した。`FAIL: AC-04 TestAsArrayElementLimit: …`。
    -   `t.Parallel()` だけ: `TestParseSubtitlesArrayElementLimit` に `_ = 1` を加えた。`FAIL: AC-05/AC-10 internal/ytdlpsource/json3_test.go has a change other than added t.Parallel() lines`。
    -   測るテストは直列: `TestParseCopiedLargeInputs` に `t.Parallel()` を加えた。`FAIL: AC-06 TestParseCopiedLargeInputs calls t.Parallel()`。
    -   変えないファイル: `display_render_test.go` のパッケージの行に文字を加えた。`FAIL: AC-07 … changed or cannot be read`。
    -   変えたファイルの範囲: `Makefile` に 1 行を加えた。`FAIL: AC-08 a changed file is out of scope`。
    -   どの壊し方でも、最後は `RESULT: FAIL` になった。
-   **実行の記録:** （`/runplan` が書く。`diff` 検証のスクリプトなので、出力と 2 つのコミットは 7.0 節に置き、ここからはその場所を指す）

### 7.3. 一時スクリプトの記録（`_checks/measure.sh`）

-   **スクリプト:** `docs/tasks/0129_race_test_speedup/_checks/measure.sh`。前と後のコミットを交互にチェックアウトし、対象のテストの単独の実行と全体の実行の実時間を測り、AC-14 の判定を出す（設計書 3.6・6.2）。AC-13・AC-14 を確かめる。
-   **判定:**
    -   計測の前提（変更のない作業ツリー、`deadcode` が `PATH` にある）が満たされていること
    -   どの実行も通り、実時間を読めること
    -   AC-14 の判定: 比較に含める対象のテスト（線形の時間のテストと `TestParseSubtitlesArrayElementLimit` を除く 6 つ）のそれぞれで、後の中央値が前の最小値より小さいこと
    -   終わった後に、元のブランチに戻っていること
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる。壊し方は、コミットしたスクリプトを作業ツリーの外に写して入れ、対象のテストと回数を減らして実行する（ステップ 3-2）。
-   **予定の実行:**
    -   PR を作る前に、`/runplan` が時間の計測で、人にほかの重い処理がないことを確かめてから 1 回実行する。
    -   その後に対象のテストかスクリプトが変わったとき（PR を開く前の上位モデルのレビューの直しを含む）は、変えた人が、人への問いからやり直して実行し直す。
-   **テスト:** コミットしない。
-   **壊し方の記録:** スクリプトの内容は、コミット `1fd0b9ff3328d670c67d01ecf479c3595b88e297` である。スクリプトは、作業ツリーのチェックアウトと元のブランチへの戻しを伴うので、マージベース `51f82a7109d33691afb287bb03f4e68d2e628b38` と `1fd0b9ff…` を比べ、対象のテストと回数を減らして実行した（`MEASURE_TESTS`、`MEASURE_ROUNDS`、`MEASURE_FULL=0`）。壊し方のうち 2 つは、コミットしたスクリプトを作業ツリーの外に写して直した。
    -   前提（変更のない作業ツリー）: 作業ツリーに追跡されないファイルを置いて実行した。`RESULT: FAIL (the work tree is not clean)`、終了コード 2。
    -   前提（`deadcode`）: `PATH` から `deadcode` を除いて実行した。`RESULT: FAIL (deadcode is not on PATH)`、終了コード 2。
    -   どの実行も通り、実時間を読めること: 写したスクリプトで、対象のテストの名前を存在しないものに変えた。`RESULT: FAIL (a run failed)`、終了コード 1。そのとき元のブランチに戻っていた（`restored: on isseis/0129-race-test-speedup-04 …`）。
    -   AC-14 の判定: 前と後に同じコミットを指定した（`TestAsArrayElementLimit`、3 回）。前の最小値 1.89 秒、後の中央値 1.89 秒で、`FAIL (not shorter)`、`RESULT: FAIL (AC-14)`、終了コード 1。対照として、マージベースと後のコミットで `TestParseCopiedLargeInputs` を 3 回測ると、前の最小値 9.29 秒、後の中央値 3.52 秒で `RESULT: PASS`、終了コード 0 だった。
    -   元のブランチに戻っていること: 写したスクリプトの、戻すためのチェックアウトを `true` に置き換えて実行した。`RESTORE FAILED: …`、終了コード 3。分離した HEAD が残ったので、手で元のブランチに戻した。
-   **実行の記録:** （`/runplan` が書く。出力、2 つのコミット、人の答えは 7.0 節に置き、ここからはその場所を指す）

### 7.4. 大きな入力のテストの入力のバイト数（AC-01）

実行したコミットは `d5e4b8f49815512ec30379a30392797248a716b6` である。`go test -tags test -race -count=1 -v -run '^TestParseCopiedLargeInputs$' ./internal/transcript` のログの、7 つの入力のバイト数の行（`--- PASS: TestParseCopiedLargeInputs (2.06s)`）:

```text
1048575 bytes, 95325 segments, 745.466292ms, 1801496240 allocated (1718.04 per byte)
1048576 bytes, 1 segments, 8.569ms, 14392 allocated (0.01 per byte)
1048576 bytes, 1 segments, 786.232375ms, 1958348104 allocated (1867.63 per byte)
1048576 bytes, 1 segments, 26.402542ms, 16855632 allocated (16.07 per byte)
1048576 bytes, 1 segments, 71.44075ms, 8463416 allocated (8.07 per byte)
1048576 bytes, 32768 segments, 302.939583ms, 628406024 allocated (599.29 per byte)
1048575 bytes, 1 segments, 113.37175ms, 8427712 allocated (8.04 per byte)
```

7 つとも 4,194,304 バイトより小さい。順に、shortest form A lines、one long form A line、textless form B timestamps、trailing zero-length lines、one-byte prefix lines、form A headings between segments、one-byte form B headings である。`make test-parse-norace` は、`make test`（終了コード 0）の中で、同じコミットの作業ツリーで通った。

### 7.5. 手元のテストの実行（AC-09）

実行したコミットは `d5e4b8f49815512ec30379a30392797248a716b6`（作業ツリーは変更なし）である。

| 実行 | 終了コード | 実時間 |
|---|---:|---:|
| `go test -tags test -race -count=1 ./...`（1 回目） | 0 | 350 秒 |
| 同（2 回目） | 0 | 267 秒 |
| 同（3 回目） | 0 | 276 秒 |
| `make test` | 0 | — |
| `make lint` | 0（`0 issues.`） | — |

1 回目は、ほかの重い処理（`break_parse.sh` の実行）と並行した。

### 7.6. CI の `Test` ジョブの 3 回の実行（AC-09）

（PR を作った後に書く。`/runplan` の報告の後、人が同じセッションのエージェントに指示する。エージェントは人の承認を得て、`gh run view <run-id> --json jobs` で `Test` ジョブの ID を読み、PR の最初の CI の実行の `Test` ジョブを `gh run rerun <run-id> --job <job-id>` で 2 回再実行する。どちらのコマンドもネットワークを使うので、承認の対象に含める。3 回の実行の URL、結論、実行したコミットを書く。6 章のこの記録のチェックボックスにも印を付け、記録と合わせて 1 つのコミットにする。そのコミットを、`_lib/local-cli-review.md` でローカルの CLI レビューにかけてから push する。`/runpr` はこの記録を読まないので、レビュアーは、マージの質問に答える前に、この節が埋まっていることを確かめる。どれかが失敗したら、原因を記録して人に知らせる。その後に `/fixpr` か `/mergemain` が Go のコードを変えたときは、この記録は古くなるので、変えたコミットで取り直す）

### 7.7. カバレッジの比較（AC-11）

**`make coverage-compare BASE=51f82a7109d33691afb287bb03f4e68d2e628b38`**（コミット `d5e4b8f49815512ec30379a30392797248a716b6`、実行は 1 回、出力の全体）:

```text
target    make coverage-compare BASE="51f82a7109d33691afb287bb03f4e68d2e628b38"
base      51f82a7109d33691afb287bb03f4e68d2e628b38
head      d5e4b8f49815512ec30379a30392797248a716b6 tree bef6168ba07f7c23b51aa8f11496501432de827b
toolchain go1.27.1
tests     base 1831 head 1831
files     base 195 head 195
functions base 2348 head 2348
deleted   0
no compared function lost coverage
coverage-compare: PASS
```

**`-race` 付きの大きな入力のテストのプロファイル。** 前はマージベース `51f82a7109d33691afb287bb03f4e68d2e628b38` を `git archive` で展開した木で、後はコミット `d5e4b8f49815512ec30379a30392797248a716b6` の作業ツリーで、`go test -tags test -race -count=1 -coverpkg=./internal/transcript -coverprofile=<file> -run '^TestParseCopiedLargeInputs$' ./internal/transcript` を実行し、`go tool cover -func=<file>` の出力を比べた。2 つの出力は完全に同じだった（`diff` の出力は空）。後の出力（パッケージのパスの接頭辞は省いた）:

```text
internal/transcript/copied.go:38:		NewCopiedSource			0.0%
internal/transcript/copied.go:59:		Fetch				0.0%
internal/transcript/copied_format.go:119:	parseCopiedTranscript		69.6%
internal/transcript/copied_format.go:156:	copiedGlobalError		0.0%
internal/transcript/copied_format.go:162:	copiedLineError			0.0%
internal/transcript/copied_format.go:169:	copiedLineSeparator		50.0%
internal/transcript/copied_format.go:198:	splitCopiedLines		100.0%
internal/transcript/copied_format.go:211:	parseTimestampLine		100.0%
internal/transcript/copied_format.go:224:	parseTimestampPrefix		66.7%
internal/transcript/copied_format.go:244:	atoiMatch			100.0%
internal/transcript/copied_format.go:251:	timestampToMilliseconds		100.0%
internal/transcript/copied_format.go:260:	japaneseLabel			100.0%
internal/transcript/copied_format.go:270:	appendJapaneseUnit		66.7%
internal/transcript/copied_format.go:280:	englishLabel			0.0%
internal/transcript/copied_format.go:296:	appendEnglishUnit		0.0%
internal/transcript/copied_format.go:303:	formatEnglishUnit		0.0%
internal/transcript/copied_format.go:314:	matchFormALabel			42.9%
internal/transcript/copied_format.go:330:	checkCopiedLine			60.0%
internal/transcript/copied_format.go:340:	hasCopiedControlChar		75.0%
internal/transcript/copied_format.go:352:	copiedWhitespaceOnlyLine	100.0%
internal/transcript/copied_format.go:358:	copiedBlankLine			100.0%
internal/transcript/copied_format.go:364:	copiedIsOperationLabel		100.0%
internal/transcript/copied_format.go:373:	copiedTrailingStart		100.0%
internal/transcript/copied_format.go:389:	copiedTimestampLineAt		0.0%
internal/transcript/copied_format.go:407:	copiedPrefixEnd			66.7%
internal/transcript/copied_format.go:426:	hasCopiedDigitColonDigit	75.0%
internal/transcript/copied_format.go:435:	isASCIIDigit			100.0%
internal/transcript/copied_format.go:442:	copiedIsChapterHeading		85.7%
internal/transcript/copied_format.go:458:	checkCopiedTrailingLines	75.0%
internal/transcript/copied_format.go:476:	parseCopiedFormA		79.3%
internal/transcript/copied_format.go:529:	parseCopiedFormB		81.5%
internal/transcript/copied_format.go:570:	readableAsFormA			60.0%
internal/transcript/errors.go:29:		Error				0.0%
internal/transcript/errors.go:35:		Unwrap				0.0%
internal/transcript/test_helpers.go:15:	readTestdataFile		0.0%
internal/transcript/video_id.go:16:		ValidateVideoURL		0.0%
internal/transcript/video_id.go:64:		NormalizedVideoURL		0.0%
internal/transcript/video_id.go:73:		videoIDFromPath			0.0%
internal/transcript/video_id.go:82:		videoIDFromYouTube		0.0%
internal/transcript/video_id.go:99:		splitURLPath			0.0%
total:									(statements)			53.0%
```

前の出力も、`diff` が空なので同じ内容である（41 行）。

## 8. 成功基準 (Success Criteria)

-   **機能:** AC-01〜AC-14 の検証が、7 章のとおりに済んでいる。
-   **品質:** `make test`・`make lint` が通る。本番のコードのカバレッジが下がらない（AC-11）。手元と CI で 3 回ずつ通る（AC-09）。
-   **セキュリティ:** N/A（要件書 4.2）。
-   **文書:** ADR-0038 の食い違いの項が消えている。7 章の記録がある。

## 9. 次のステップ (Next Steps)

-   PR を作った後、7.6 節の CI の 3 回の実行を記録する。
-   PR のマージの後、issue #1166 を閉じる（`/runpr` が、タスクの最後の計画の PR の後に提案する）。

## 変更履歴

- 2026-11-26 承認（isseis、#1185）
