# 実装計画書：大きな入力の解析のテストが CI で時間の上限を超える原因を調べて改善する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-23 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「要件書」）の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下「設計書」）の節を指し、本計画自身の節は「本計画 §N」と書く。用語は設計書の冒頭と要件書 6 章の用語（大きな入力のテスト、時間の確認、時間以外の確認、`-race` なしの確認のターゲット、レシピのガード、計測のワークフロー、調査の記録など）を同じ意味で使う。既存のファイルの行番号は、断りがない限りコミット `d6ea458`（`main`）で確かめた。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`TestParseCopiedLargeInputs` が CI で時間の上限を超える原因を調べて記録し（F-001）、時間の確認を `-race` なしのビルドに移して CI で単独に実行する（F-002・F-003）。パーサの改善は、設計書 3.7 の条件が崩れたときだけ行う（F-004）。最後に、CI で 3 回続けて成功することで、安定を確かめる（F-005）。

### 1.2. 実装の原則

-   **計測を先に、変更を後に**（設計書 1.1）。フェーズ 1〜3 はコードを変えない。フェーズ 3 の判定で止まる条件（設計書 6 章の `STOP`）に当たったら、それより後に進まない。
-   **ネットワークに触れる操作は、その都度人間の承認を得る**（`CLAUDE.md`「Tool Execution Safety」）。本タスクでは、計測のワークフローの push・実行のし直し・ブランチの削除・ログの取得、`go.mod` の版のツールチェーンの取得（`GOTOOLCHAIN`）、PR での CI の再実行が当たる。
-   **計測のワークフローは `main` に入れない**（設計書 3.6.2）。計測用の一時的なブランチだけに置き、PR を作らない。
-   **テストが働くことは、対象を壊して確かめる**（`CLAUDE.md`「Testing Strategy」）。ステップ 5-6 では、実装する人が対象を一時的に壊し、テストが失敗することを見て、コミットのメッセージに書く。

### 1.3. 計画の際に決めたこと（2026-10-23）

人間のレビュアーが決めたもの:

-   **調査の記録は、コードより先に、文書だけの PR にする:** 設計書 8 章の末尾が実装計画に委ねた判断である。調査の結論、AC-05 の判定、規則で求めた上限の値を、コードを書く前の PR（PR-1。本計画 §3.2）でレビューする。設計書 6 章の止まる条件（`G1`〜`G3`）は人間の判断を要するので、そこで PR を区切る。
-   **パーサの改善の見込みを、プロファイルと同時に調べる（2026-10-23、PR のレビューでの指摘）:** ステップ 3-2 で、人間のレビュアーは改善するかを決める。そのとき、改善が技術的にできるか、どれだけ効くか、受理と拒否の規則を保てるかが分からないと、決められないためである。プロファイルの上位の箇所ごとに、改善の方法の案・見込める効果・AC-14 の危険を調査の記録に書く（ステップ 1-4）。設計書 3.7 の判断の条件は変えない。見込みは判断の材料であり、試作はしない。

計画で決めたもの（人間のレビュアーに諮らず、理由とともに記す）:

-   **`test-parse-norace` の `-timeout` は `5m` にする:** 設計書 3.2 が実装計画に委ねた値である。上限は 30 秒未満に限られる（設計書 3.5。30 秒以上なら止まる）ので、7 つの入力の上限の合計は 210 秒未満であり、5 分はそれより長い。`go test` の既定（10 分）より短い。上限の値に依らずに決まる値なので、上限を決める前に決められる。レシピのガードは、この値が 7 × `copiedParseDeadline` より長く 10 分より短いことを確かめる（ステップ 5-5）。
-   **レシピのガードは `internal/transcript/makefile_test.go` に置く:** 設計書 2.2 が実装計画に委ねたファイル名である。`make` のターゲットを確かめるテストを `makefile_test.go` に置く先例（`cmd/yt2column/makefile_test.go`・`internal/llm/claude/makefile_test.go`・`internal/llm/deepseek/makefile_test.go`・`internal/publisher/makefile_test.go`）に揃える。
-   **`make test` が `test-parse-norace` を実行することは、`RunTarget` で `make test` を実行して確かめる:** 設計書 3.3 が実装計画に委ねた確かめ方である。`make test` のレシピは、`-race` 付きの実行の後に `$(MAKE) test-parse-norace` を呼ぶ（設計書 3.2）。`RunTarget`（`internal/maketestutil/make.go:105`）は `make` のコマンドラインで `GOTEST` を記録用のスクリプトに置き換え、GNU make はコマンドラインの変数を下位の `make` に渡す。記録用のスクリプトは呼ばれるたびに記録を上書きする（`make.go:70` の `} > "$out"`）ので、記録に残るのは最後の `$(GOTEST)` の呼び出しの引数である。この引数が `test-parse-norace` の引数に等しいことを確かめる。下位の `make` への変数の受け渡しは、本計画の作成時に GNU Make 3.81（macOS）で確かめた。`$(GOTEST)` を呼んだ後に `$(MAKE) b` を呼ぶターゲット `a` と、`$(GOTEST)` を呼ぶターゲット `b` を持つ `Makefile` を、`env -i PATH=/usr/bin:/bin make -s a GOTEST=<記録用のスクリプト>` で実行すると、記録用のスクリプトに `a` と `b` の 2 回の呼び出しが届いた。`make -n test` の出力を読む案は、出力の形が `make` の版で変わるので採らない。この確かめ方は、`make test` の `-race` 付きの行が消えたこと、`$(MAKE)` の行の失敗が無視されることを検出しない。`-race` 付きの実行が残ることはステップ 5-9 で確かめ、残りは受け入れる（本計画 §6）。
-   **CI の手順は `ci.yml` の行を読んで確かめる:** YAML のライブラリは使えない（`CLAUDE.md`「Dependencies」）。`TestLintTagsIncludeIntegration`（`internal/transcript/ytdlp_test.go:1746`）が `ci.yml` を行で読む先例に揃える。
-   **パーサを改善する場合は、独立した PR にする（PR-2）:** 改善は振る舞いを変えないことを AC-14 で確かめる変更であり、時間の確認を移す変更（PR-3）とは確かめる観点が違う。改善しない場合は、PR-2 を作らない（本計画 §3.2）。
-   **AC-16 の 3 回の実行は、PR-3 のマージの前に、マージするヘッドで行い、URL を PR-4 で記録する:** 要件書 AC-16 が定める順序である。CI は `pull_request` でしか走らない（`.github/workflows/ci.yml:3-5`）ので、3 回の実行と AC-09 の確かめは、PR-3 を作った後でなければ行えない。`/runplan` は PR を作って止まり（`.claude/commands/runplan.md` step 5）、`/runpr` はマージの直前に記録のコミット（PRE_MERGE）を push してすぐマージする（`.claude/commands/runpr.md` step 4 の 3）。そのため、どちらのコマンドにも 3 回の成功を確かめる間がなく、確かめた後に `/runpr` がコミットを足せばヘッドが変わる。そこで、フェーズ 6 を `/runplan` と `/runpr` の間に手で行う。PRE_MERGE に当たる記録（PR-3 の「PR がマージされた」などのチェックボックス）を先に手でコミットしてから 3 回の成功を確かめ、その後の `/runpr` では PRE_MERGE が「すでに記録済み」で空になる（`runpr.md` の PRE_MERGE の定義「It leaves out what is already recorded」）ので、数えたヘッドがそのままマージされる。フェーズ 6 は PR-3 の作成からマージまでの間に行う。そのため、`/runplan` が PR-4 に進む時点では済んでおり、PR-4 で `/runplan` が行うのは記録だけである。フェーズ 6 のステップは、記録を書く PR-4 の対象に含める。
-   **記録の AC は `manual` で確かめる:** AC-01〜AC-06・AC-08・AC-13・AC-15・AC-16 は、計測・判断・CI の実行の結果を調査の記録または本計画に書くことを求める AC である。これらに `test` も `static` も付けない。記録があることを確かめる文書のテストは、タスク 0038 の要件書が削除すると決めた「タスクの記録の確認」（0038 の要件書 1 章「タスクの記録の確認は削除する」）に当たり、記録の中身の正しさも確かめないためである。先例はタスク 0037 の実装計画（1 章の決定と 5 章の例外の記述）である。mkplan の規則（AC ごとに `test` か `static` を 1 つ以上）の例外として、本計画 §5 に記す。

### 1.4. 既存コード調査結果

| 領域 | すでにあるもの | 足りないもの・変えるもの |
|---|---|---|
| 大きな入力のテスト | `TestParseCopiedLargeInputs`（`internal/transcript/copied_format_test.go:1049-1092`）。時間の確認は `:1080-1082`、ログは確認の後の `:1088`。上限 `copiedParseDeadline` は `:1033`（15 秒）、その根拠のコメントは `:1024-1030`、関数の説明は `:1036-1048` | 時間の確認をビルドごとの定数で切り替える。ログを確認の前に移す。上限の値とコメントを改める（設計書 3.1・3.5） |
| ビルドごとの定数 | `copiedParseAllocPerByte`。`copied_format_race_test.go:16`（8192、`//go:build test && race`）、`copied_format_norace_test.go:9`（128、`//go:build test && !race`）。race のファイルのコメント（`:12-15`）は、`make test` が race のファイルだけをビルドし、norace の上限は lint が確かめると述べる | 両方のファイルに `copiedParseChecksDeadline` を加える（設計書 3.1）。`copiedParseAllocPerByte` の値は変えない。race のファイルの `:12-15` のコメントを、norace のファイルも `make test` と CI の `test-parse-norace` でビルドされる形に改める |
| 本タスクの外で norace のビルドを実行するもの | pre-commit のフック `go test -tags test ./...`（`.pre-commit-config.yaml:6`）と、`CLAUDE.md`「Test Commands」の `go test -tags test -v ./...`。どちらも全パッケージを並行に実行する | 変えない。本タスクの後は、ここでも時間の確認が行われる。ステップ 1-2 で、この条件の手元の値も計測する（本計画 §6） |
| `Makefile` | `test`（`Makefile:83-84`、`$(GOTEST) -tags test -race -v ./...`）、`test-ci`（`:87-89`、`-coverprofile` 付き、`-count=1` なし）、`.PHONY`（`:53`）。`GOTEST=$(GOCMD) test`（`:4`） | `test-parse-norace` を加え、`make test` のレシピから呼ぶ。`test-ci` に `-count=1` を付ける。`.PHONY` に加える（設計書 3.2・3.4） |
| CI | `Test` ジョブ（`.github/workflows/ci.yml:55-74`）。手順は「Run tests (race, coverage)」（`:70-71`、`make test-ci`）と「Build」（`:73-74`） | 2 つの手順の間に `make test-parse-norace` の手順を加える（設計書 3.4） |
| `make` のターゲットを確かめる補助 | `internal/maketestutil.RunTarget`（`internal/maketestutil/make.go:105`）。`//go:build test` のパッケージ。`MakeInvocation.Args` に最後の呼び出しの引数を返す。パッケージのコメント（`make.go:3-8`）は、対象を課金のある結合テストのターゲットとしている。`test-ci` は `$(GOCMD) tool cover` を直接呼ぶ（`Makefile:89`）ので、`RunTarget` では実行できない | 関数はそのまま使い、新しい補助は作らない。パッケージのコメントを、課金のないターゲットにも使うことがわかる形に改める（ステップ 5-5）。`test-ci` の `-count=1` はガードで確かめず、AC-16 の `(cached)` の確認で確かめる |
| `ci.yml` を読むテストの先例 | `TestLintTagsIncludeIntegration`（`internal/transcript/ytdlp_test.go:1746`）。リポジトリのルートを `filepath.Join("..", "..")` で求め、行で読む | 同じ方法でレシピのガードの CI の確認を書く |
| `make` の結合テストの先例 | `cmd/yt2column/makefile_test.go` は結合テストのターゲットと `make build` を確かめ（`:41`・`:55`・`:68`・`:331`）、`test`・`test-ci` は確かめない | 変えない |
| 拡張のテスト | `extension/test/repository.test.ts:230-260` が `node`・`npm` のない `PATH` で `make -n build test lint deadcode` を実行する。`make -n` は `$(MAKE)` の行を実行して下位の `make` にも `-n` を渡す（本計画の作成時に確かめた） | 変えない。`Makefile` を変えるので `make ext-test` で確かめる（設計書 3.2） |
| 拡張の CI ジョブの起動 | `extension/scripts/has-extension-changes.sh` は `Makefile` と `.github/workflows/*` の変更で拡張のジョブを起動する | 変えない。PR-3 では拡張のジョブも走る |
| `CLAUDE.md` | 「Test Commands」（`CLAUDE.md:69-72`）。`make test` は「Run all tests with race detection」 | `make test-parse-norace` の行を加え、`make test` の説明を改める（設計書 2.2） |
| 0015 の文書 | 設計書の `Comments`（`docs/tasks/0015_copied_transcript_source/02_architecture.md:11`）、実装計画の `Comments`（同 `03_implementation_plan.md:11`、今は `-`） | それぞれに `(decision)` の注記を加える（設計書 3.9） |
| パーサ | `internal/transcript/copied_format.go` | 設計書 3.7 の条件が崩れて改善すると決めた場合だけ変える（フェーズ 4） |

`d6ea458` で `rg -n 'copiedParseDeadline|within the deadline' internal/` を実行すると、`copied_format_test.go` の `:1033`・`:1046`・`:1080`・`:1081` の 4 行だけが一致した。これに、一致しない上限の根拠のコメント（`:1024-1030`）を加えたものが、時間の確認を参照する箇所である。0015・0024 の文書の `TestParseCopiedLargeInputs` への言及は、0015 の `Comments` の注記で扱い、本文は書き換えない（設計書 3.9）。

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-08 は、すべて設計書の付録 A に記録されている。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 手元の調査（AC-01・AC-03・AC-13 の材料）

設計書 8 章の優先順位 1。コードを変えない。値はフェーズ 3 で調査の記録 `04_investigation.md` に書く。

**変更するファイル:** `docs/tasks/0039_large_input_test_timeout/04_investigation.md`（新規）

-   [ ] **ステップ 1-1**: 調査の記録 `04_investigation.md` を作り、計測の環境（OS、CPU とそのコア数、Go の版）と、計測したコミット（本タスクの変更の前の `main`）の節を置く。`Document Status` は持たない（設計書 3.8）。
-   [ ] **ステップ 1-2**: 設計書 3.6.1 の 4 つの条件 (a)〜(d) で、7 つの入力の時間とメモリの確保量を、各条件 3 回ずつ計測する。Go の版は `GOTOOLCHAIN` で `go.mod` の版（`go.mod:3`）に揃える（ツールチェーンの取得はネットワークに触れるので、先に承認を得る）。(b) は `go clean -testcache` の後の `make test` で、`internal/transcript` の行が `(cached)` でないことを確かめる。あわせて、AC-01 の外の条件 (e) として、pre-commit のフックと同じ `-race` なしの全パッケージの並行の実行（`-count=1 -v` を付ける）の値も計測する（本計画 §6 のリスクの判断に使う）。入力ごと・条件ごとの最小・中央・最大と、各条件で実行したコマンドの行をそのまま調査の記録に書く。
-   [ ] **ステップ 1-3**: 設計書 3.6.3 の手順で、`shortest_form_A_lines` と `textless_form_B_timestamps` の CPU とメモリの確保のプロファイルを、`-race` 付きと `-race` なしのそれぞれで取る（`-memprofilerate=1`、Go の版はステップ 1-2 と同じ）。設計書 3.6.3 の 3 つの記録の項目（確保量の上位の関数とその割合の対比、H-04 の仮説の成否と根拠になる標準ライブラリの箇所、`-race` 付きの確保量の妥当性の判断と理由）を調査の記録に書く。プロファイルのファイルはリポジトリに置かない。
-   [ ] **ステップ 1-4**: ステップ 1-3 のプロファイルで、確保量または CPU の時間の上位に現れたパーサの箇所ごとに、改善の見込みを調査の記録に書く。書くのは、改善の方法の案、見込める効果（`-race` 付きと `-race` なしのそれぞれ）、受理と拒否の規則を変えないこと（AC-14）を保つうえでの危険とその確かめ方である。コードは書かず、試作もしない。ステップ 3-2 で人間のレビュアーが改善するかを決めるときの材料にするためである。

**完了の条件:** 調査の記録に、AC-01 の 4 つの条件の値と環境・コミット、AC-03 の 3 つの項目、ステップ 1-4 の改善の見込みがある。

### フェーズ 2: CI での計測（AC-02・AC-06）

設計書 8 章の優先順位 2。設計書 3.6.2 の手順で、計測のワークフローを一時的なブランチで実行する。

**変更するファイル:** 計測のワークフロー（一時的なブランチの `.github/workflows/` の 1 ファイル。`main` と本タスクの PR のブランチには入れない）、`04_investigation.md`

-   [ ] **ステップ 2-1**: `origin/main` から計測用の一時的なブランチを作り、設計書 3.6.2 のとおりの計測のワークフローの 1 ファイルだけをコミットする。設計書に加えて、次を満たす。
    -   `actions/checkout`・`actions/setup-go` の版と `go-version-file: go.mod` を `ci.yml:61-68` に揃える。
    -   `ci-equivalent` の 2 つめの実行（ステップ 5-3 の `test-parse-norace` と同じ引数の `go test`）の手順に `if: ${{ !cancelled() }}` を付け、1 つめの実行が失敗しても実行する。
-   [ ] **ステップ 2-2**: push の前に、ワークフローの内容（`contents: read` だけの権限、ブランチの名前に限った `push` の起動、秘密を参照しないこと、`fail-fast: false`）と、使う認証情報が `workflow` の権限を持つことを人間に示して承認を得る。一時的なブランチを push し、1 回めの実行を起こす。少なくとも 3 時間あけて、承認を得て 2 回めを起こす（実行のし直しか push のし直し）。各条件の標本数は 10 になる。
-   [ ] **ステップ 2-3**: 承認を得て両方の実行のログを取り、テストの `t.Logf` の行から入力ごと・条件ごとの時間を集める。集計の補助のスクリプトはリポジトリに置かない。調査の記録に、次を書く。
    -   入力ごと・条件ごとの最小・中央・最大。`ci-equivalent` の 2 つめの実行の値は AC-06 の値として、ほかと分けて書く。
    -   各実行の URL。実行のし直しは同じ実行の ID を使うので、試行ごとの URL（`.../runs/<id>/attempts/<n>`）と試行の番号を書く。
    -   各ジョブの runner の情報（設計書 3.6.2 の 4 項目）。
    -   計測したコミットと、計測のワークフローのファイルの全文。ブランチはステップ 2-4 で削除し、実行のログは保存の期限を過ぎると消えるので、PR-1 のレビューで計測の条件を確かめられるのはこの全文だけである。
-   [ ] **ステップ 2-4**: 承認を得て、一時的なブランチを `origin` とローカルから削除する。

**完了の条件:** 調査の記録に、AC-02 の (a)〜(d) と `race-cover-package` と AC-06 の値が、実行の URL と runner の情報とともにある。一時的なブランチが `origin` にもローカルにもない。

### フェーズ 3: 結論と判断（AC-04・AC-05・AC-08・AC-13）

設計書 8 章の優先順位 3。止まる条件に当たったら、ここで止まる。

**変更するファイル:** `04_investigation.md`

-   [ ] **ステップ 3-1**: 設計書 3.6.4 の表の 6 つの比を、入力ごとの中央の値で求め、ばらつきを最小と最大で示して、AC-04 の結論を調査の記録に書く。
-   [ ] **ステップ 3-2**: 設計書 3.7 の 2 つの条件（条件 1: `-race` に固有の増加と確かめたか、条件 2: AC-02 (d) の全計測の最大が 1 秒未満か）を判定し、AC-13 の判断と理由を調査の記録に書く。条件 2 が崩れたら、F-002 に進まずに、ステップ 1-4 の改善の見込みを添えて、人間のレビュアーに要件の変更として判断を求める（AC-05 (a)）。条件 1 が崩れたら、ステップ 1-4 の改善の見込みを示して、パーサを改善するかを人間のレビュアーに尋ねる。改善しないと決めたら止まる（AC-05 (b)）。改善すると決めたら、ステップ 3-3 を `[-]`（「改善するので、上限はステップ 4-4 で決める」）にする。改善しない場合は、フェーズ 4 の各ステップと PR-2 作成ポイントのチェックボックスを `[-]`（「ステップ 3-2 で改善しないと判断した」）にする。どちらの印も、PR-1 のブランチで、PR-1 のグリーンゲートの前にコミットする。
-   [ ] **ステップ 3-3**: （パーサを改善しない場合）設計書 3.5 の規則で上限を求める。時間の最も長い入力の AC-06 の最大 ÷ 最小が 3 を超えるか、求めた上限が 30 秒以上なら、人間のレビュアーに判断を求めて止まる。AC-08 の 6 つの値（AC-06 の最大と最小、上限、上限 ÷ AC-06 の最大、上限と 1 秒との比、上限と 30 秒との比）を調査の記録に書く。上限の節は、ステップ 5-2 のテストのコメントが参照するので、見出しを付けておく。ステップ 1-2 の条件 (e) の最大と上限との比も書き、(e) の最大が上限以上なら、人間のレビュアーに判断を求めて止まる（本計画 §6）。

**完了の条件:** 調査の記録に、AC-04 の結論、AC-13 の判断と理由、（改善しない場合）AC-08 の値がある。止まる条件に当たっていない。

### PR-1 作成ポイント: investigation record

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3

**推奨タイトル**: `docs(0039): 大きな入力の解析の時間の調査の記録を加える`

**レビュー観点**: 計測の条件が設計書 3.6.1・3.6.2 の表と一致し、各値に環境・コミット・実行の URL が付いていること / AC-04 の比が記録の値から正しく求められていること / 設計書 3.7 の 2 つの条件の判定と、3.5 の規則による上限の計算が記録の値と合っていること

**実装モデル要件**: frontier-required

**判定理由**: フェーズ 2 は一時的な CI のワークフローを書いて外部の runner で実行する（mkplan のパネルの契機の CI・外部の資源）うえ、フェーズ 3 は止まるかどうかを決める判断を含む。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: パーサの改善（条件付き。AC-14・AC-15）

設計書 8 章の優先順位 4。ステップ 3-2 で人間のレビュアーが改善すると決めた場合だけ行う。改善しない場合の印は、ステップ 3-2 が PR-1 でつける。

**変更するファイル:** `internal/transcript/copied_format.go`、`04_investigation.md`

-   [ ] **ステップ 4-1**: プロファイル（ステップ 1-3）が示した箇所を改善する。受理の形・拒否の番兵・大きさの上限は変えない（AC-14、設計書 5 章）。改善は、ほかの変更と別のコミットにし、コミットのメッセージに改善が負う正しさの義務を書く（`CLAUDE.md`「Performance」）。
-   [ ] **ステップ 4-2**: 0015・0024 の受け入れ基準を確かめる既存のテスト（fuzz のテストの種を含む）を、変更しないまま `make test` で通す。改善のコミットがテストのファイルを変えていないことを `git diff --stat` で確かめ、コミットのメッセージに書く（AC-14）。
-   [ ] **ステップ 4-3**: 改善の前と後で、ステップ 1-2 と同じ条件の時間とメモリの確保量を計測し、調査の記録に書く（AC-15）。
-   [ ] **ステップ 4-4**: 改善の後のコードで、ステップ 2-1〜2-4 の手順で AC-06（`ci-equivalent`）を計測し直す。計測用の一時的なブランチは、`origin/main` ではなく PR-2 のブランチの改善のコミットから作り、計測のワークフローの 1 ファイルだけを加える。計測したコミットを調査の記録に書く。ステップ 3-3 の規則で上限を求め直し、AC-08 の値を調査の記録に書く（設計書 6 章「改善の前の AC-06 の値は、改善の後の解析の時間を表さない」）。

**完了の条件:** `make test && make lint` が通る。調査の記録に、AC-15 の前後の値と、改善の後の AC-06・AC-08 の値がある。

### PR-2 作成ポイント: parser improvement (conditional)

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4

**推奨タイトル**: `perf(0039): コピーした字幕のパーサのメモリの確保を減らす`

**レビュー観点**: 改善のコミットが振る舞いの変更と分かれ、テストのファイルを変えていないこと / 改善の前後の値と、改善の後の上限が記録されていること / 受理の形・拒否の番兵・大きさの上限が変わっていないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 4-1 はプロファイルの結果に応じて改善の方法を新たに決める判断を含み（前例のない設計の判断）、ステップ 4-4 は CI の計測をやり直す。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 時間の確認の移動と CI の手順の追加（AC-07・AC-09〜AC-12）

設計書 8 章の優先順位 5。設計書 3.1〜3.4・3.9 の変更を 1 つの PR にまとめる。

**変更するファイル:** `internal/transcript/copied_format_test.go`、`internal/transcript/copied_format_race_test.go`、`internal/transcript/copied_format_norace_test.go`、`internal/transcript/makefile_test.go`（新規）、`Makefile`、`.github/workflows/ci.yml`、`CLAUDE.md`、`docs/tasks/0015_copied_transcript_source/02_architecture.md`、`docs/tasks/0015_copied_transcript_source/03_implementation_plan.md`

-   [ ] **ステップ 5-1**: `copied_format_race_test.go` に `copiedParseChecksDeadline = false`、`copied_format_norace_test.go` に `copiedParseChecksDeadline = true` を、`copiedParseAllocPerByte` と同じ `const` の組に加える（設計書 3.1）。それぞれのファイルのコメントに、時間の確認をそのビルドで行うか行わないかと、その理由（本番は `-race` なしでビルドされる）を英語で加える。race のファイルの `:12-15` の段落（norace の上限は lint だけが確かめるという記述）を、norace のファイルも `make test` と CI の `test-parse-norace` でビルドされ実行されることに合わせて改める。
-   [ ] **ステップ 5-2**: `TestParseCopiedLargeInputs`（`copied_format_test.go`）を次のように改める（設計書 2.3・3.1・3.5）。
    -   `t.Logf` を、解析の直後、どの確認よりも前に移す。
    -   時間の確認を、`copiedParseChecksDeadline` が真のときだけ行う。失敗の形（`parsed %d bytes in %s, want under %s`）は変えない。
    -   `copiedParseDeadline` を、ステップ 3-3（改善した場合はステップ 4-4）で求めた値にする。
    -   上限の根拠のコメント（`:1024-1030`）を、時間は `-race` なしのビルドで確かめること、上限の決め方と調査の記録の節への参照、上限が検出するもの（設計書 3.5「この上限が検出するもの」）に書き換える。関数の説明（`:1045-1048`）の「each must parse within the deadline」を、時間の確認は `-race` なしのビルドだけで行うことがわかる文に改める。コメントは英語で書く。
-   [ ] **ステップ 5-3**: `Makefile` を次のように改める（設計書 3.2・3.4）。
    -   ターゲット `test-parse-norace` を加える。レシピは `$(GOTEST)` の 1 回の実行で、引数は `-tags test -count=1 -v -timeout 5m -run '^TestParseCopiedLargeInputs$$' ./internal/transcript`（`-race` も `-coverprofile` も付けない）。ターゲットの前に、何を確かめるかと、`make test` がレシピから呼ぶ理由（前提条件にすると `make -j` で並行に走る）を英語のコメントで書く。
    -   `test` のレシピの、今の `-race` 付きの行の後に、`ext-check`（`Makefile:302-307`）と同じ形の `@$(MAKE) --no-print-directory test-parse-norace` を加える。前提条件にはしない。
    -   `test-ci` の `go test` に `-count=1` を加える。
    -   `.PHONY`（`:53`）に `test-parse-norace` を加える。
-   [ ] **ステップ 5-4**: `.github/workflows/ci.yml` の `Test` ジョブの「Run tests (race, coverage)」と「Build」の間に、`make test-parse-norace` を実行する手順を加える。手順の名前は、`-race` なしの時間の確認であることがわかるものにする（設計書 4 章の最後から 2 行めの表の行）。`if:`・`continue-on-error:` は付けない（設計書 3.4）。
-   [ ] **ステップ 5-5**: レシピのガードを `internal/transcript/makefile_test.go`（`//go:build test`）に書く（設計書 3.3）。確かめることは次の 3 つで、テスト関数は 3 つに分ける。あわせて、`internal/maketestutil` のパッケージのコメント（`make.go:3-8`）を、課金のないターゲットにも使うことがわかる形に改める。
    -   `test-parse-norace` のレシピ: `RunTarget` で実行し、記録した引数の並び全体が、`-tags test -count=1 -v -timeout <値> -run <値> ./internal/transcript` に等しいこと（`internal/publisher/makefile_test.go` と同じく、値の位置を置き換えてから並び全体を比べる。`-timeout` がなければ失敗する）。`-run` の値が `^` + テスト関数の名前 + `$` に等しいこと（名前は `TestParseCopiedLargeInputs` の関数の値から求め、文字列で書かない）。`-timeout` の値が 7 × `copiedParseDeadline` より長く、10 分より短いこと。`copiedParseDeadline` が 0015 の予算の 30 秒より短いこと（AC-07）。
    -   `make test`: `RunTarget` で実行し、記録した最後の呼び出しの引数が、`test-parse-norace` を実行したときの引数に等しいこと（本計画 §1.3）。
    -   CI: `ci.yml` を読み、次を確かめる。`test` ジョブ（空白 2 つで字下げした `test:` の行から、次の同じ深さのキーの直前まで）の中に、`run:` の値がちょうど `make test-parse-norace` である手順があり、その手順が、`run:` の値が `make test-ci` である手順より後にあること。その手順は、`run:` の前後のどこにも、手順の深さの `if:`・`continue-on-error:` を持たない。`test` ジョブはジョブの深さの `continue-on-error:` を持たない（ジョブの深さの `if:`（`ci.yml:59`）は今のまま持つ）。行の読み方は `TestLintTagsIncludeIntegration` に揃える。
-   [ ] **ステップ 5-6**: 次のそれぞれで、対象を一時的に壊してテストが失敗することを確かめ、元に戻し、確かめたことをコミットのメッセージに書く（設計書 7.1）。
    -   `copiedParseDeadline` を解析の時間より小さくすると、`make test-parse-norace` が時間の確認で失敗し、`-race` 付きの `go test -tags test -race -count=1 -run '^TestParseCopiedLargeInputs$' ./internal/transcript` は失敗しない（AC-07・AC-10）。
    -   `copied_format_norace_test.go` の `copiedParseAllocPerByte` を実測より小さくすると、`make test-parse-norace` がメモリの確保量の確認で失敗する（AC-12）。
    -   `copied_format_race_test.go` の `copiedParseAllocPerByte` を実測より小さくすると、`-race` 付きの上の `go test` がメモリの確保量の確認で失敗する（AC-11。ステップ 5-2 の書き換えの後も race のビルドで時間以外の確認が働くこと）。
    -   レシピから `-count=1` を外す、レシピに `-race` または `-cover` を加える、`-run` の名前を変える、`-timeout` を外すか 10 分以上にする、`copiedParseDeadline` を 30 秒にする、`make test` から `test-parse-norace` の呼び出しを消す、`ci.yml` から手順を消す、手順を `lint` ジョブに移す、手順の `run:` の前に `if:` を加える、手順に `continue-on-error: true` を付ける、`test` ジョブに `continue-on-error: true` を付ける、のそれぞれで、ステップ 5-5 の対応するテストが失敗することを確かめる。
-   [ ] **ステップ 5-7**: `CLAUDE.md` の「Test Commands」に `make test-parse-norace` の行（`TestParseCopiedLargeInputs` を `-race` なし・単独で実行し、時間の上限を確かめる）を加え、`make test` の行を、`-race` 付きの全テストの後に `test-parse-norace` も実行することがわかる説明に改める（設計書 2.2）。
-   [ ] **ステップ 5-8**: 0015 の設計書と実装計画の `Comments` に、`2026-MM-DD (decision):` で始まる注記を加える（設計書 3.9）。注記は、本タスクで時間の確認を `-race` なしのビルドに移したこと、この決定は人間のレビュアー（isseis）が本タスクの要件書の承認（2026-10-23）で承認したこと、本タスクの要件書・設計書への参照、`Status` を `approved` のまま変えないことを含める。日付は実装の日にする。設計書の `Comments` は今の記述の後に続け、実装計画の `Comments` は `-` を置き換える。0015 の本文と 0024 の文書は変えない。
-   [ ] **ステップ 5-9**: `make fmt`・`make test`・`make lint`・`make ext-test` を通す。`make lint` が `copiedParseChecksDeadline` の常に真の条件を指摘しないことを確かめる（設計書 3.1）。指摘した場合は、その行だけに `//nolint:<linter>` と理由のコメントを付け、そのことをコミットのメッセージに書く。`make test` の出力で、`-race` 付きの実行の後に `-race` なしの `TestParseCopiedLargeInputs` が実行され、7 つの入力のログが出ることを確かめる。

**完了の条件:** `make test && make lint` と `make ext-test` が通る。ステップ 5-6 の確かめがコミットのメッセージにある。

### PR-3 作成ポイント: race-free deadline check in CI

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8 / 5-9

**推奨タイトル**: `test(0039): 解析の時間の確認を -race なしのビルドに移して CI で単独に実行する`

**レビュー観点**: 時間の確認が `-race` なしのビルドだけで行われ、時間以外の確認は両方のビルドで行われること / `test-parse-norace` のレシピ・`make test` の呼び出し・CI の手順を、レシピのガードが確かめていること / 上限の値とコメントが調査の記録と一致すること / マージの前に、フェーズ 6 を `/runpr` より先に行うこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 5-3〜5-5 が CI のワークフローと `Makefile` のレシピを変え、それを確かめるガードを新たに書く（mkplan の step 8 のパネルの契機の CI の面）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: PR-3 のマージの前の CI での確認（AC-09・AC-16）

設計書 8 章の優先順位 6 の前半。`/runplan` が PR-3 を作って止まった後、`/runpr` で PR-3 をマージする前に、人間の承認のもとで手で行う（本計画 §1.3）。CI は `pull_request` でしか走らないので、PR-3 を作る前には行えない。結果は PR-3 のコメントに控え、本計画への記録はフェーズ 7 で行う。フェーズ 6 のチェックボックスは、PR-3 のマージの後も `[ ]` のまま `main` に残る。`/runplan` が PR-4 に進んだときは、フェーズ 6 を実行し直さず、ステップ 7-1 で PR-3 のコメントの記録に従って `[x]` にする。PR のコメントの投稿は、`.claude/commands/_lib/task-pr-handoff.md` の言及の確認（Mention check）を通してから行う。

**変更するファイル:** PR-3 のブランチの、ステップ 6-2 の一時的なコミットとその取り消しのコミット、ステップ 6-3 の記録のコミット（本計画）

-   [ ] **ステップ 6-1**: Codex の PR のレビューを待ち、未解決のスレッドを `/fixpr` で解決する。CI が通ることを確かめる。ここまでの変更が済むまで、後のステップに進まない（後でヘッドが変わると、ステップ 6-4 を数え直すことになる）。
-   [ ] **ステップ 6-2**: `copiedParseDeadline` を解析の時間より小さくしたコミットを、承認を得て PR-3 のブランチに push し、`Test` ジョブが `make test-parse-norace` の手順で失敗し、PR の CI が失敗することを確かめる。そのコミットを取り消すコミットを push する（AC-09、設計書 7.1）。失敗した実行の URL を控える。
-   [ ] **ステップ 6-3**: `/runpr` がマージの直前にコミットする記録（`runpr.md` の PRE_MERGE の「Checkpoint」）と同じ内容を、手でコミットして push する。PR-3 作成ポイントの「PR がマージされた」と「次のブランチへ切り替えた」、本計画 §7 の「PR-3 マージ済み」の 3 つを `[x]` にし、ほかは変えない。`git diff --name-only HEAD~1` で、変わったのが本計画だけであることを確かめる。PR-3 がマージされずに終わった場合は、このコミットを取り消す。
-   [ ] **ステップ 6-4**: ステップ 6-3 の後のヘッドで、`Test` ジョブを、最初の実行と、承認を得た再実行 2 回の計 3 回が、続けて成功することを確かめる（設計書 6 章「AC-16 の確かめ方」）。各実行で、`make test-ci` と `make test-parse-norace` の出力に `internal/transcript` の `(cached)` がないことを確かめる。大きな入力のテストが失敗したら、数え直さずに止まり、その実行を控えて人間のレビュアーの判断を求める。テストに関わらない失敗（runner の割り当て、ネットワーク）だけは、その分類を控えて数え直す。ヘッドが変わったら新しいヘッドで数え直す。3 回の試行ごとの URL（`.../runs/<id>/attempts/<n>`）と試行の番号を控える。
-   [ ] **ステップ 6-5**: `/runpr` で PR-3 をマージする。PRE_MERGE はステップ 6-3 で記録済みなので空になり、ステップ 6-4 で数えたヘッドがそのままマージされる。`/runpr` がヘッドを変えた場合（新しいスレッドへの対応など）は、マージせずにステップ 6-4 から数え直す。

**完了の条件:** ステップ 6-2 の失敗の実行の URL と、ステップ 6-4 の 3 回の成功の試行の URL が PR-3 のコメントにあり、PR-3 がステップ 6-4 のヘッドのままマージされた。

### フェーズ 7: AC-16 の記録

設計書 8 章の優先順位 6 の後半。PR-3 のマージの後に、`docs/` だけを変える PR で記録する（要件書 AC-16）。

**変更するファイル:** `docs/tasks/0039_large_input_test_timeout/03_implementation_plan.md`

-   [ ] **ステップ 7-1**: 本計画 §8 の「AC-16 の記録」の表に、ステップ 6-4 の 3 回の試行の URL と試行の番号、ヘッドのコミット、`(cached)` がなかったことを書く。ステップ 6-2 の失敗の実行の URL も、AC-09 の確かめとして同じ節に書く。数え直しがあった場合は、その実行と分類も書く。フェーズ 6 の各ステップのチェックボックスを、PR-3 のコメントに控えた結果に従って `[x]` にする（PR-3 のブランチで印をつけるとヘッドが変わるので、ここでつける）。

**完了の条件:** 本計画 §8 に、AC-16 の 3 回の URL と AC-09 の確かめの URL がある。

### PR-4 作成ポイント: stability record

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 7-1

**推奨タイトル**: `docs(0039): CI での安定の確認の記録を加える`

**レビュー観点**: 3 回の実行が同じヘッドで続けて成功し、どれも `(cached)` を含まないこと / 記録した URL のヘッドが PR-3 のマージの直前のヘッドであること

**実装モデル要件**: standard

**判定理由**: フェーズ 6 の CI の操作は、PR-3 のマージの前に、`/runplan` とは別に人間の承認のもとで行う。`/runplan` が PR-4 で行うのはステップ 7-1 の記録だけで、どの契機にも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 | 対応するフェーズ |
|---|---|---|
| M1: 原因の特定 | 調査の記録（AC-01〜AC-04・AC-06・AC-08・AC-13）。AC-05 の判定を通った | 1〜3（PR-1） |
| M2: パーサの改善（条件付き） | 改善のコミット、AC-15 の記録、改善の後の上限 | 4（PR-2） |
| M3: 時間の確認の移動 | 時間の確認が `-race` なしのビルドで CI に実行される | 5（PR-3） |
| M4: 安定の確認 | AC-09・AC-16 の CI の実行の記録 | 6・7（PR-4） |

各マイルストーンは前のマイルストーンを前提とする。M1 の判定で止まる条件に当たったら、M2 以降に進まない。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3 | 調査の記録 `04_investigation.md`（手元の計測・プロファイル・CI での計測・結論・判断・上限） | frontier-required |
| PR-2 | 4-1 / 4-2 / 4-3 / 4-4 | （条件付き）パーサの改善と、その前後の記録・上限の決め直し。改善しない場合は作らない | frontier-required |
| PR-3 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8 / 5-9 | 時間の確認の移動、`test-parse-norace`、CI の手順、レシピのガード、`CLAUDE.md`、0015 の注記 | frontier-required |
| PR-4 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 7-1 | AC-09・AC-16 の CI の実行の記録（フェーズ 6 は PR-3 のマージの前に手で行う） | standard |

PR-1・PR-4 は文書だけの PR なので、CI の `Test` ジョブは走らず、`doc-guard` ジョブだけが走る。PR-3 は `Makefile` と `ci.yml` を変えるので、`Test` ジョブと拡張のジョブが走る。

## 4. テスト戦略 (Test Strategy)

### 4.1 単体テスト

-   `TestParseCopiedLargeInputs` は、`make test`（`-race` 付き、続けて `-race` なし）、CI の `make test-ci`（`-race` 付き）と `make test-parse-norace`（`-race` なし）で実行される（設計書 7.1）。テストの入力と期待するセグメント数は変えない。
-   レシピのガード（ステップ 5-5）は、`make test`・`make test-ci` の `-race` 付きの実行と `make lint` の型の検査で動く。`RunTarget` は `GOTEST` を記録用のスクリプトに置き換えるので、ガードは `go test` を入れ子に実行しない。
-   壊して確かめること（ステップ 5-6）で、各確認が述べた理由で失敗することを確かめる。

### 4.2 統合テスト

本タスクの統合テストはない（設計書 7.2）。CI での安定はフェーズ 6 で確かめる。

### 4.3 後方互換のテスト

-   `make test` と `make test-ci` は、これまでどおり `-race` 付きで全パッケージのテストを実行する（要件書 4.4）。ステップ 5-9 で確かめる。
-   `make -n build test lint deadcode` が `node`・`npm` を呼ばないこと（`extension/test/repository.test.ts:230`）は、`make ext-test` で確かめる。
-   パーサを改善した場合、既存の 0015・0024 のテストを変更なしで通す（ステップ 4-2）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | manual | `04_investigation.md` の手元の計測の節（PR-1 のレビュー） | 1-1・1-2 |
| AC-02 | manual | `04_investigation.md` の CI での計測の節。実行の URL と runner の情報付き（PR-1 のレビュー） | 2-1〜2-4 |
| AC-03 | manual | `04_investigation.md` のプロファイルの節（PR-1 のレビュー） | 1-3 |
| AC-04 | manual | `04_investigation.md` の結論の節（PR-1 のレビュー） | 3-1 |
| AC-05 | manual | ステップ 3-2・3-3 の判定。止まる条件に当たったら、人間のレビュアーの判断なしに後のフェーズに進まない | 3-2・3-3 |
| AC-06 | manual | `04_investigation.md` の `ci-equivalent` の 2 つめの実行の値。実行の URL と runner の情報付き（PR-1、または改善した場合は PR-2 のレビュー） | 2-3・4-4 |
| AC-07 | test | `internal/transcript/copied_format_test.go::TestParseCopiedLargeInputs`（`make test-parse-norace`。norace のビルド）。上限を下げると失敗することをステップ 5-6 で確かめる | 3-3・5-1・5-2・5-6 |
| AC-07 | static | `internal/transcript/makefile_test.go` の `test-parse-norace` のレシピのテスト（`-race` なし・`-run` の値・`copiedParseDeadline` が 30 秒より短いこと） | 5-5 |
| AC-08 | manual | `04_investigation.md` の上限の節。`copied_format_test.go` のコメントがその節を参照する（PR-1・PR-3 のレビュー） | 3-3・4-4・5-2 |
| AC-09 | static | `internal/transcript/makefile_test.go` の `make test` のテストと CI のテスト（`Test` ジョブが `if:`・`continue-on-error:` なしで `make test-parse-norace` を実行する） | 5-4・5-5 |
| AC-09 | manual | ステップ 6-2 の、上限を下げたコミットで PR の CI が失敗した実行（本計画 §8 に URL を記録） | 6-2・7-1 |
| AC-10 | test | `internal/transcript/copied_format_test.go::TestParseCopiedLargeInputs`（`make test`・`make test-ci`。race のビルド）。上限を下げても `-race` 付きの実行が失敗しないことをステップ 5-6 で確かめる | 5-1・5-2・5-6 |
| AC-11 | test | `internal/transcript/copied_format_test.go::TestParseCopiedLargeInputs`（`make test`・`make test-ci`。race のビルドで成功・セグメント数・`copiedParseAllocPerByte` の 8192 を確かめる）。race の上限を下げると失敗することをステップ 5-6 で確かめる | 5-2・5-6・5-9 |
| AC-12 | test | `internal/transcript/copied_format_test.go::TestParseCopiedLargeInputs`（`make test-parse-norace`。norace のビルドで同じ 3 つを、上限 128 で確かめる）。上限を下げると失敗することをステップ 5-6 で確かめる | 5-2・5-6 |
| AC-12 | static | `internal/transcript/makefile_test.go` の 3 つのテスト（CI で実行されること） | 5-5 |
| AC-13 | manual | `04_investigation.md` の判断の節と、その材料の改善の見込みの節（PR-1 のレビュー） | 1-4・3-2 |
| AC-14 | test | 改善した場合だけ。`make test` の既存の 0015・0024 のテスト（fuzz のテストの種を含む）を変更なしで通す。改善のコミットがテストのファイルを変えていないことを PR-2 のレビューで確かめる | 4-2 |
| AC-15 | manual | 改善した場合だけ。`04_investigation.md` の改善の前後の節（PR-2 のレビュー） | 4-3 |
| AC-16 | manual | 本計画 §8 の「AC-16 の記録」の 3 回の実行の URL と `(cached)` の確認（PR-4 のレビュー）。キャッシュの再利用を防ぐ `-count=1` のうち、`test-parse-norace` の分は `makefile_test.go` が確かめ、`test-ci` の分は各実行の `(cached)` の確認で確かめる | 6-3・6-4・6-5・7-1 |

**mkplan の規則の例外:** AC-01〜AC-06・AC-08・AC-13・AC-15・AC-16 は `manual` だけで確かめる（AC-16 は一部を `static` が支える）。本計画 §1.3「記録の AC は `manual` で確かめる」のとおり、これらは計測・判断・CI の実行の結果を記録することを求める AC であり、記録があることを確かめる文書のテストはタスク 0038 が削除すると決めた種類に当たるためである。AC-05 は作業の進め方を定める AC で、確かめる対象のコードがない。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| CI の runner の計測で、`-race` なしの時間が 1 秒以上になる、または揺れが 3 倍を超える | F-002 に進めない | ステップ 3-2・3-3 で止まり、人間のレビュアーに判断を求める（AC-05、設計書 3.5） |
| `.github/workflows/` のファイルの push に `workflow` の権限が要る | ステップ 2-2 で push が拒否される | ステップ 2-2 の前に、使う認証情報が `workflow` の権限を持つかを人間に確かめる（設計書 3.6.2） |
| 計測のワークフローが、ほかのブランチへの push で起動する | 余計な CI の実行 | `push` の起動をそのブランチの名前に限る（ステップ 2-1）。`ci.yml` は `pull_request` だけで起動するので、一時的なブランチの push で起動しない（`ci.yml:3-5`） |
| 手元の `make test` を重い処理と同時に実行すると、`-race` なしの時間の確認が上限を超える | 手元の偽の失敗 | 上限は CI の最大の 10 倍で、手元の `-race` なしの最大は 0.116 秒（要件書 1 章）なので、まれと見込む。超えたら負荷のない状態で `make test-parse-norace` を実行し直す（設計書 3.2） |
| pre-commit のフック（`go test -tags test ./...`。`.pre-commit-config.yaml:6`）は、`-race` なしで全パッケージを並行に実行する。本タスクの後は、ここでも時間の確認が行われ、負荷で上限を超えると `git commit` が止まる | 手元の偽の失敗 | ステップ 1-2 の条件 (e) で、この条件の手元の値を計測し、上限との比を調査の記録に書く。条件 (e) の最大が上限以上なら、ステップ 3-3 で人間のレビュアーに判断を求める |
| レシピのガードの `make test` のテストは最後の `$(GOTEST)` の呼び出ししか見ない。`test-ci` は `RunTarget` で実行できない | `make test` の `-race` 付きの行が消えること、`$(MAKE)` の行の失敗が無視されること、`test-ci` から `-count=1` が消えることを、ガードが検出しない | 受け入れる。`-race` 付きの行はステップ 5-9 で、`test-ci` の `-count=1` はステップ 6-4 の `(cached)` の確認で確かめる。後の変更は PR のレビューで見る |
| AC-16 の 3 回の実行の途中で、大きな入力のテストが失敗する | 安定を確かめられない | 数え直さずに止まり、人間のレビュアーの判断を求める（ステップ 6-4、設計書 6 章） |
| `RunTarget` が `make test` の記録を上書きする動作に頼るガードが、`make` の版で変わる | ガードの偽の成功や失敗 | 動作は GNU make のコマンドラインの変数の受け渡しと、記録用のスクリプトの上書きによる。ステップ 5-6 の `make test` の呼び出しを消す確かめで、ガードが失敗することを確かめる |
| PR-2 のレビューで `copied_format.go` が変わり、ステップ 4-4 で計測したコードと食い違う | 上限が改善の後のコードを表さない | `copied_format.go` が変わったら、ステップ 4-4 をやり直す |
| 計測の 2 回の実行の間を 3 時間あけること、ネットワークに触れる操作ごとの承認、CI の再実行の待ちで、フェーズ 2 とフェーズ 6 が 1 回のセッションで終わらない | 日程の遅れ | フェーズ 2 は 2 回めの実行を起こすところでセッションを分けてよい（ブランチと 1 回めの URL を調査の記録に控えてから止まる）。フェーズ 6 は PR-3 のコメントに進み具合を控え、ヘッドを変えない限り、途中から再開できる |
| ステップ 6-2 の上限を下げたコミットを取り消し忘れる | 失敗する PR のマージ | ステップ 6-4 は 3 回の成功を求めるので、取り消していなければ先に進めない |
| ステップ 6-3 で「PR がマージされた」を先に記録した後、PR-3 がマージされない | 計画のチェックボックスが事実と食い違う | ステップ 6-3 のとおり、記録のコミットを取り消す |
| `/runpr` が PR-3 のヘッドを変える（新しいスレッドへの対応など） | 数えたヘッドと、マージするヘッドが食い違う | ステップ 6-5 のとおり、マージせずにステップ 6-4 から数え直す |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3）
- [ ] PR-2 マージ済み、または改善しないと判断して作らなかった（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4）
- [ ] PR-3 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8 / 5-9）
- [ ] PR-4 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 7-1）
- [ ] 計測のワークフローの一時的なブランチが `origin` にもローカルにもない（ステップ 2-4）

## 8. 成功の基準 (Success Criteria)

-   **機能の完全性:** 時間の確認が `-race` なしのビルドだけで行われ、CI の `Test` ジョブで単独に実行される。時間以外の確認は両方のビルドで行われる。本計画 §5 のすべての AC の検証が済んでいる。
-   **品質:** `make test && make lint` と `make ext-test` が通る。ステップ 5-6 の壊して確かめることがコミットのメッセージにある。
-   **安定:** AC-16 の 3 回の実行が続けて成功し、下の記録がある。
-   **セキュリティ:** 計測のワークフローが `contents: read` だけの権限を持ち、一時的なブランチの名前に限った `push` で起動し、秘密を参照しない（ステップ 2-2 で人間が確かめ、ステップ 2-3 で全文を調査の記録に残す）。`ci.yml` の権限は変えない（設計書 5 章）。
-   **文書:** 調査の記録が AC-01〜AC-04・AC-06・AC-08・AC-13（改善した場合は AC-15）を含む。0015 の設計書と実装計画の `Comments` に `(decision)` の注記がある。`CLAUDE.md` の「Test Commands」が `make test-parse-norace` を含む。

### AC-16 の記録

（ステップ 7-1 で書く。）

| 実行 | 試行の URL | 試行の番号 | ヘッドのコミット | `(cached)` がない |
|---|---|---|---|---|
| 1 | - | - | - | - |
| 2 | - | - | - | - |
| 3 | - | - | - | - |

AC-09 の確かめ（ステップ 6-2）の実行: -

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/runplan 0039` でフェーズ 1 から始める。
-   PR-4 のマージの後、issue #463 を閉じる（`Closes #N` では自動で閉じない）。
-   ほかの時間の上限を持つテスト（`internal/transcript/exec_test.go` など）が CI で失敗するようになったら、別の issue で扱う（要件書 2.3、設計書 9 章）。
