# 設計書：線形時間テストの時間比の判定を -race なしの単独の実行へ移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 設計の全体像

本タスクでは、`TestNeutralizedDisplayMarkdownLinearTime` が時間比を判定するかどうかを、2 つの条件で決める（[01_requirements.md](01_requirements.md)、以下、要件書。F-001・F-002）。1 つ目はビルドタグ `race` と `!race` のテストのファイルに置く定数、2 つ目はテストの環境変数である。`-race` なしの単独の実行は、既存の `make test-parse-norace` の対象を広げて行い、そのレシピだけが環境変数を設定する。レシピの引数と環境変数は、`internal/transcript` と `internal/publisher` のガードのテストで固定する（AC-06）。

**設計原則:**

-   **前例の仕組みを使う。** 時間を確かめるかどうかをビルドタグのファイルの定数で決め、`test-parse-norace` で `-race` なしで走らせる仕組みは、`TestParseCopiedLargeInputs` が使っている（`internal/transcript/copied_format_norace_test.go`、`internal/transcript/copied_format_race_test.go`、[ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md)）。足すのは、判定を求める環境変数 1 つだけである（要件書 1 章「要件定義の際に決めたこと」）。
-   **判定の内容を変えない。** 時間比の閾値（8 倍）、入力の単位と大きさ（2000 単位と 8000 単位）、計測の回数（6 回のうち最初を除く 5 回の最良値）は変えない（要件書 2.3 節）。
-   **確認が黙って失われないようにする。** `-run` の対象、2 つのパッケージ、並行に走らない条件、環境変数は、ガードのテストで固定する（AC-06）。

**設計の際に決めたこと（2026-11-28）:**

-   `test-parse-norace` は、1 回の `go test` に `-p 1` を付け、`./internal/transcript` と `./internal/publisher` の 2 つのパッケージを渡す。`-run` は `^(TestParseCopiedLargeInputs|TestNeutralizedDisplayMarkdownLinearTime)$` とする（AC-03・AC-05）。
    -   理由: `go test` は既定で、パッケージのテストのバイナリを `GOMAXPROCS` の数まで並行に走らせる。`-p 1` は、同時に走らせるテストのバイナリを 1 つに限る（`go help build` の `-p n`）。3 文書のレビューの際に、このレシピで 2 つのパッケージが順に走り、それぞれで 1 つのテストだけが走ることを確かめた。
    -   既存のガード（`internal/transcript/makefile_test.go:81-91` の `TestMakeTestRunsParseNorace`）は、`go test` の代わりに呼ばれるスタブ（GOTEST）が最後の呼び出しだけを残すことを前提にしている。呼び出しを 1 回に保てば、このガードをそのまま使える。ADR-0038「結果」の「`-run` で絞った 1 回の `go test`」も保たれる。
    -   `-p 1` を外すと、2 つのパッケージが黙って並行に走るようになる。そのため、ガードで固定する（3.3 節）。
    -   採らなかった案: レシピで `go test` を 2 回、順に実行する案（スタブが最後の呼び出ししか残さないので、ガードとテストのヘルパー `internal/maketestutil` の作り直しが要る）。
    -   2026-11-28 に人が選んだ。
    -   ADR: [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) の「結果」の、時間の判定を行う `go test` の記述に加える。
-   判定を求める環境変数は `YT2COLUMN_TEST_TIMING` とし、判定を求める値は `1` とする。レシピは、`go test` の呼び出しの前にこの変数を置いて設定する（要件書 AC-04）。
    -   理由: 名前の形は、テストの環境変数の既存の名前（`YT2COLUMN_TEST_VIDEO_URL` など。`Makefile:209`）に倣う。値を `1` の 1 つに限ると、ほかの値を拒め、読み替えも起きない。名前と値は 1 か所に置き、テストとガードはそれを参照する（3.3 節）。
    -   本書を書く際に決めた。人には尋ねていない。PR のレビューで確かめてもらう。
    -   ADR: なし。名前と値は本タスクの中で閉じる。判定を環境変数の指定に限ることは、要件の決定として ADR-0038 に記録する。

本書の行番号は、コミット `5bba819a`（`origin/main`）で確かめた。

概念モデルとシステム構成の Mermaid の図は省く。変えるのはテストと開発用のターゲットだけで、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

### 3.1. 変更するファイル

| ファイル | 変更 | 責務 |
|---|---|---|
| `internal/publisher/display_render_test.go` | 変更 | 線形時間テストが、各入力で成功と出力の契約を確かめ、3.2 節の条件を満たすときだけ時間比を判定する |
| `internal/publisher/display_render_race_test.go` | 追加 | `//go:build test && race`。時間比を判定しないことを示す定数を置く |
| `internal/publisher/display_render_norace_test.go` | 追加 | `//go:build test && !race`。時間比を判定できることを示す定数を置く |
| `internal/publisher/makefile_test.go` | 追加 | `test-parse-norace` の `-run` が線形時間テストの名前を含むことを確かめるガード（3.3 節） |
| `internal/transcript/makefile_test.go` | 変更 | `TestMakeTestParseNorace` の期待する引数と環境変数を変え、古くなるコメントを直す（3.3 節） |
| `internal/transcript/copied_format_test.go` | 変更 | `make test-parse-norace` について述べるコメントだけを直す（3.3 節） |
| `internal/maketestutil/make.go` | 変更 | 判定を求める環境変数の名前と値の定数と、`-run` の値を名前の並びとして読む関数を足す（3.3 節） |
| `internal/maketestutil/make_test.go` | 変更 | 足した関数の受け付ける形と拒む形のテスト（3.3 節） |
| `Makefile` | 変更 | `test-parse-norace` のレシピと、`test` と `test-parse-norace` のコメント（3.4 節） |
| `.github/workflows/ci.yml` | 変更 | `make test-parse-norace` の手順の名前だけを変える（3.4 節） |
| `CLAUDE.md` | 変更 | 「Test Commands」の `make test`・`make test-parse-norace`・`-race` なしの `go test` の項（3.4 節） |
| `docs/dev/adr/0038-deadcode-and-race-free-timing.md` | 変更 | 3 文書の PR で、題名・背景・決定・採らなかった案・結果・経緯を更新済み。実装が設計とずれたときだけ直す |
| `docs/dev/adr/README.md` | 変更 | 3 文書の PR で、ADR-0038 の行の題名を更新済み |

製品のコード（`internal/publisher` のテスト以外のファイル）は変えない。

### 3.2. 線形時間テスト

-   2 つのビルドタグのファイルは、`internal/transcript` の `copiedParseChecksDeadline` と同じ形で、真偽の定数を 1 つ置く。どちらのファイルも、定数の意味と、`-race` 付きのビルドで判定しない理由をコメントに書く。
-   テストは、最初に環境変数 `YT2COLUMN_TEST_TIMING` を読む。
    -   ないか空なら、判定を求めない。
    -   `1` なら、判定を求める。
    -   それ以外の値なら、受け付ける値（`1`）と読んだ値を示して `t.Fatalf` で失敗する（AC-04）。
-   時間比を判定するのは、`-race` なしのビルドの定数が真で、判定を求めるときだけである。`-race` 付きのビルドでは、環境変数が `1` でも判定しない（AC-01）。
-   入力ごとのサブテスト（`open URLs`・`after a '['`）は、次の順に確かめる。
    1.  8000 単位の入力で `NeutralizedDisplayMarkdown` を 1 回呼び、エラーがないことと、`checkNeutralizedContract`（`internal/publisher/display_render_test.go:30-43`）の契約を確かめる（AC-02）。
    2.  時間比を判定しないなら、ここで終える（AC-01・AC-04）。
    3.  判定するなら、いまと同じ計測と判定を行う（`display_render_test.go:243-263`）。計測の中の呼び出しのエラーの確認も残す（AC-03）。
-   テストのコメントの「no absolute limit is set, because make test runs with the race detector」は、時間比の判定が `-race` なしの単独の実行だけで行われることに合わせて書き直す。絶対値の上限を置かない理由（機械によって時間が違う）は残す。

### 3.3. ガードのテスト

-   **`internal/maketestutil` に足すもの:**
    -   判定を求める環境変数の名前（`YT2COLUMN_TEST_TIMING`）と値（`1`）の定数。線形時間テストとガードは、この定数を参照する。
    -   `-run` の値を読む関数。値が `^(` で始まり `)$` で終わり、その間が `|` で区切られた空でない名前の並びのときに、その名前の並びを返す。ほかの形は、センチネルエラーを包んで返す。`regexp` の構文として解釈はせず、文字列の形だけを見る（`CLAUDE.md`「Reject, don't normalize」）。
    -   その関数のテストを `internal/maketestutil/make_test.go` に足す。受け付ける例を 1 つと、拒む代表の例（`^(` がない、`)$` がない、空の名前がある、`^A$` の形）を表にし、拒む例は `errors.Is` でセンチネルエラーを確かめる。
-   **`internal/transcript` の `TestMakeTestParseNorace`:**
    -   `maketestutil.RunTarget` に環境変数の名前を記録させ、その値が `1` であることを確かめる（AC-06）。
    -   期待する引数を `-tags test -count=1 -v -p 1 -timeout <timeout> -run <run> ./internal/transcript ./internal/publisher` とし、引数の全体を順に比べる（AC-05・AC-06）。
    -   `<run>` は上の関数で読み、名前の並びがちょうど 2 つで、その 1 つが `largeInputTestName()`（関数値から取る名前。`internal/transcript/makefile_test.go:29-34`）であることを確かめる。
    -   `-timeout` の範囲の確認は変えない。
-   **`internal/transcript` の `TestMakeTestRunsParseNorace`:** 引数に加えて、記録した環境変数も `test-parse-norace` のものと比べる。
-   **`internal/publisher` の新しいガード:** `maketestutil.RunTarget` で `test-parse-norace` を走らせ、`-run` の名前の並びに、`TestNeutralizedDisplayMarkdownLinearTime` の関数値から取った名前があることを確かめる（AC-06）。関数値から名前を取る形は、`largeInputTestName` に倣う。引数の全体と環境変数は `internal/transcript` のガードが固定するので、ここでは確かめない。
-   2 つのテストの名前は、それぞれのパッケージのガードが関数値から取る。名前を変えたときにガードが失敗することは、計画書の壊し方の確認で確かめる。
-   `TestCIRunsParseNorace`（`internal/transcript/makefile_test.go:99-184`）は変えない。CI の手順は `run` の値で探すので、手順の名前を変えても読み方は変わらない。
-   次のコメントは、2 つのテストと 2 つのパッケージ、`-p 1`、環境変数に合わせて書き直す。
    -   `internal/transcript/makefile_test.go:18-21`: 時間の判定が `TestParseCopiedLargeInputs` だけを走らせる 1 回の `go test` で行われるというコメント。
    -   `internal/transcript/makefile_test.go:36-41`: `TestMakeTestParseNorace` の説明のコメント。
    -   `internal/transcript/copied_format_test.go:1029-1032`: `make test-parse-norace` がパーサの期限の確認だけを走らせるというコメント。

### 3.4. ターゲット、CI、記述

-   `Makefile` の `test-parse-norace` のレシピは、1 章の決定のとおりにする。`-timeout 5m` は変えない。要件書 4.1 節の計測では、2 つのパッケージの実行の全体が約 2.3 秒で、ガードの範囲（7 つの入力の期限の合計より大きく、10 分より小さい）の中にある。
-   `Makefile` のコメントは、このターゲットが 2 つのテストを `-race` なしで、`-p 1` で 1 つずつ走らせること、環境変数で線形時間の時間比の判定を求めること、パーサの期限と線形時間の時間比をここで判定することを述べる。
-   CI の `Test` ジョブの手順の名前は、`Check parse deadline (no race, alone)` から `Check timing (no race, alone)` に変える。`run` の値と位置は変えない。
-   `CLAUDE.md` の「Test Commands」の項を、要件書 AC-07 のとおりに書き直す。

## 4. エラーハンドリング設計

-   線形時間テストは、環境変数の値が空でも `1` でもないとき、受け付ける値と読んだ値を示して失敗する（3.2 節）。
-   `internal/maketestutil` の `-run` を読む関数は、受け付けない形をセンチネルエラーで返し、ガードはそのエラーで失敗する（3.3 節）。
-   製品のコードのエラーの扱いは変えない。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の条件のどれにも当たらない（[01_requirements.md](01_requirements.md) 4.2 節）。

## 6. 処理フローの詳細

N/A。製品の処理の流れは変わらない。`make test` は、いまと同じく `-race` 付きの全体の実行の後に `test-parse-norace` を呼び、その中で 2 つのパッケージのテストが 1 つずつ順に走る。

## 7. テスト戦略

-   **AC-01:** `NeutralizedDisplayMarkdown` の時間が入力の長さの 2 乗に比例するように製品のコードを一時的に壊し、出力は変えないまま、`-race` 付きのビルドで、環境変数を `1` にしても線形時間テストが通ることを確かめる。
-   **AC-02:** 出力の契約を破るように製品のコードを一時的に壊し、`-race` 付きのビルドで線形時間テストが失敗することを確かめる。
-   **AC-03:** レシピを変えた後に、AC-01 と同じ壊し方で `make test-parse-norace` が失敗することを確かめる。
-   **AC-04:** AC-01 と同じ壊し方で、`-race` なしのビルドで環境変数なしに実行したテストが通ること、環境変数を `2` にした実行が値の確認で失敗することを確かめる。
-   **AC-05・AC-06:** 3.3 節のガードのテストで確かめる。レシピの各条件を 1 つずつ外すか変え、テストの名前を変え、ガードが失敗することを確かめる。
-   **AC-07:** PR のレビューで記述を読む（`manual`）。
-   **AC-08:** `make test`・`make lint` と CI で確かめる。
-   壊し方と失敗の確認は、`CLAUDE.md`「Testing Strategy」のとおり、コミットメッセージに書く。

## 8. 実装の優先順位

1.  `internal/maketestutil` の定数と関数、そのテストを足す。
2.  線形時間テストとビルドタグのファイルを変え、AC-01・AC-02・AC-04 の壊し方を確かめる。
3.  `Makefile` のレシピとガードのテストを変え、AC-03・AC-06 の壊し方を確かめる。
4.  CI の手順の名前、`CLAUDE.md`、`Makefile` のコメントを変える。

## 9. 将来の拡張性

-   ほかに時間を確かめるテストを加えるときは、同じくビルドタグのファイルの定数と `YT2COLUMN_TEST_TIMING` で判定を切り替え、`test-parse-norace` の `-run` にも加える。そのとき、`internal/transcript` のガードが確かめる名前の数と、加えたテストのパッケージのガードも直す。
