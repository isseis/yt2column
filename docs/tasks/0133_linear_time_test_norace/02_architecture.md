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

本タスクでは、`TestNeutralizedDisplayMarkdownLinearTime` が時間比を判定するかどうかを、ビルドタグ `race` と `!race` のテストのファイルに置く定数で切り替える（[01_requirements.md](01_requirements.md) F-001・F-002）。`-race` なしの実行は、既存の `make test-parse-norace` の対象を広げて行う（F-002）。そのレシピの引数は、`internal/transcript` と `internal/publisher` のガードのテストで固定する（AC-05）。

**設計原則:**

-   **前例の仕組みをそのまま使う。** 時間を確かめるかどうかをビルドタグのファイルの定数で決め、`test-parse-norace` で `-race` なしで走らせる仕組みは、`TestParseCopiedLargeInputs` が使っている（`internal/transcript/copied_format_norace_test.go`、`internal/transcript/copied_format_race_test.go`、[ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md)）。新しい仕組みは作らない（`CLAUDE.md`「YAGNI」）。
-   **判定の内容を変えない。** 時間比の閾値（8 倍）、入力の単位と大きさ（2000 単位と 8000 単位）、計測の回数（6 回のうち最初を除く 5 回の最良値）は変えない（要件書 2.3 節）。
-   **確認が黙って失われないようにする。** `-run` の対象と 2 つのパッケージ、並行に走らない条件は、ガードのテストで固定する（AC-05）。

**設計の際に決めたこと（2026-11-28）:**

-   `test-parse-norace` は、1 回の `go test` に `-p 1` を付け、`./internal/transcript` と `./internal/publisher` の 2 つのパッケージを渡す。`-run` は `^(TestParseCopiedLargeInputs|TestNeutralizedDisplayMarkdownLinearTime)$` とする（AC-03・AC-04）。
    -   理由: `go test` は既定で、パッケージのテストのバイナリを `GOMAXPROCS` の数まで並行に走らせる。`-p 1` は、同時に走らせるテストのバイナリを 1 つに限る（`go help build` の `-p n`）。呼び出しを 1 回に保つと、スタブの GOTEST が最後の呼び出しだけを残すことを前提にした既存のガード（`internal/transcript/makefile_test.go:86-95` の `TestMakeTestRunsParseNorace`）をそのまま使える。ADR-0038「結果」の「`-run` で絞った 1 回の `go test`」も保たれる。
    -   `-p 1` を外すと、2 つのパッケージが黙って並行に走るようになる。そのため、ガードで固定する（3.3 節）。
    -   採らなかった案: レシピで `go test` を 2 回、順に実行する案（スタブが最後の呼び出ししか残さないので、ガードとテストのヘルパー `internal/maketestutil` の作り直しが要る）。
    -   2026-11-28 に人が選んだ。
    -   ADR: [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) の「結果」の、時間の確認を行う `go test` の記述に加える。

本書の行番号は、コミット `5bba819a`（`origin/main`）で確かめた。

概念モデルとシステム構成の Mermaid の図は省く。変えるのはテストと開発用のターゲットだけで、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

### 3.1. 変更するファイル

| ファイル | 変更 | 責務 |
|---|---|---|
| `internal/publisher/display_render_test.go` | 変更 | 線形時間テストが、各入力で成功と出力の契約を確かめ、定数が真のときだけ時間比を判定する（3.2 節） |
| `internal/publisher/display_render_race_test.go` | 追加 | `//go:build test && race`。時間比を判定しないことを示す定数を置く |
| `internal/publisher/display_render_norace_test.go` | 追加 | `//go:build test && !race`。時間比を判定することを示す定数を置く |
| `internal/publisher/makefile_test.go` | 追加 | `test-parse-norace` のレシピが、線形時間テストと `./internal/publisher` を含むことを確かめるガード（3.3 節） |
| `internal/transcript/makefile_test.go` | 変更 | `TestMakeTestParseNorace` の期待する引数に、`-p 1`、`./internal/publisher`、2 つの名前の `-run` を加える（3.3 節） |
| `internal/maketestutil/make.go` | 変更 | `-run` の値を `^(A|B)$` の形として読み、名前の並びを返す小さな関数を足す（3.3 節） |
| `Makefile` | 変更 | `test-parse-norace` のレシピと、`test` と `test-parse-norace` のコメント（3.4 節） |
| `.github/workflows/ci.yml` | 変更 | `make test-parse-norace` の手順の名前だけを変える（3.4 節） |
| `CLAUDE.md` | 変更 | 「Test Commands」の `make test` と `make test-parse-norace` の項（3.4 節） |
| `docs/dev/adr/0038-deadcode-and-race-free-timing.md` | 変更 | 「結果」のガードの記述と「経緯」（1 章） |

製品のコード（`internal/publisher` のテスト以外のファイル）は変えない。

### 3.2. 線形時間テスト

-   2 つのビルドタグのファイルは、`internal/transcript` の `copiedParseChecksDeadline` と同じ形で、真偽の定数を 1 つ置く。どちらのファイルも、定数の意味と、`-race` 付きのビルドで判定しない理由をコメントに書く。
-   テストは、入力ごとのサブテスト（`open URLs`・`after a '['`）で、次の順に確かめる。
    1.  8000 単位の入力で `NeutralizedDisplayMarkdown` を 1 回呼び、エラーがないことと、`checkNeutralizedContract`（`internal/publisher/display_render_test.go:30-43`）の契約を確かめる（AC-02）。
    2.  定数が偽なら、ここで終える（AC-01）。
    3.  定数が真なら、いまと同じ計測と判定を行う（`display_render_test.go:243-263`）。計測の中の呼び出しのエラーの確認も残す（AC-03）。
-   テストのコメントの「no absolute limit is set, because make test runs with the race detector」は、時間比の判定が `-race` なしのビルドだけで行われることに合わせて書き直す。絶対値の上限を置かない理由（機械によって時間が違う）は残す。

### 3.3. ガードのテスト

-   **`internal/maketestutil` の関数:** `-run` の値が `^(` で始まり `)$` で終わり、その間が `|` で区切られた空でない名前の並びのときに、その名前の並びを返す。ほかの形はエラーとする。`regexp` の構文として解釈はせず、文字列の形だけを見る（`CLAUDE.md`「Reject, don't normalize」）。
-   **`internal/transcript` の `TestMakeTestParseNorace`:** 期待する引数を `-tags test -count=1 -v -p 1 -timeout <timeout> -run <run> ./internal/transcript ./internal/publisher` とし、引数の全体を順に比べる（AC-04・AC-05）。`<run>` は上の関数で読み、名前の並びがちょうど 2 つで、その 1 つが `largeInputTestName()`（関数値から取る名前）であることを確かめる。`-timeout` の範囲の確認は変えない。
-   **`internal/publisher` の新しいガード:** `maketestutil.RunTarget` で `test-parse-norace` を走らせ、その引数に `./internal/publisher` があることと、`-run` の名前の並びに、`TestNeutralizedDisplayMarkdownLinearTime` の関数値から取った名前があることを確かめる（AC-05）。関数値から名前を取る形は、`internal/transcript/makefile_test.go:31-35` の `largeInputTestName` に倣う。
-   2 つのテストの名前は、それぞれのパッケージのガードが関数値から取る。どちらの名前を変えても、レシピが古い名前のままならどちらかのガードが失敗する（AC-05）。
-   `TestMakeTestRunsParseNorace` と `TestCIRunsParseNorace`（`internal/transcript/makefile_test.go:86-176`）は変えない。CI の手順は `run` の値で探すので、手順の名前を変えても読み方は変わらない。

### 3.4. ターゲット、CI、記述

-   `Makefile` の `test-parse-norace` のレシピは、1 章の決定のとおりにする。`-timeout 5m` は変えない。線形時間テストが加わっても、2 つのパッケージで数十秒に収まり、ガードの範囲（7 つの入力の期限の合計より大きく、10 分より小さい）の中にある。
-   `Makefile` のコメントは、このターゲットが 2 つのテストを `-race` なしで、`-p 1` で 1 つずつ走らせること、パーサの期限と線形時間の時間比をここだけで確かめることを述べる。
-   CI の `Test` ジョブの手順の名前は、`Check parse deadline (no race, alone)` から `Check timing (no race, alone)` に変える。`run` の値と位置は変えない。
-   `CLAUDE.md` の「Test Commands」の `make test` と `make test-parse-norace` の項を、要件書 AC-06 のとおりに書き直す。
-   ADR-0038 の「結果」の、時間の確認は `-run` で絞った 1 回の `go test` だけで行われ、`internal/transcript` のガードが確かめるという項目に、`-p 1` と `internal/publisher` のガードを加える。

## 4. エラーハンドリング設計

N/A。製品のコードのエラーの扱いは変えない。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の条件のどれにも当たらない（[01_requirements.md](01_requirements.md) 4.2 節）。

## 6. 処理フローの詳細

N/A。製品の処理の流れは変わらない。`make test` は、いまと同じく `-race` 付きの全体の実行の後に `test-parse-norace` を呼び、その中で 2 つのパッケージのテストが 1 つずつ順に走る。

## 7. テスト戦略

-   **AC-01:** `NeutralizedDisplayMarkdown` の時間が入力の長さの 2 乗に比例するように製品のコードを一時的に壊し、出力は変えないまま、`-race` 付きのビルドで線形時間テストが通ることを確かめる。
-   **AC-02:** 出力の契約を破るように製品のコードを一時的に壊し（例: U+200B を挿入しても `Neutralized` を偽にする）、`-race` 付きのビルドで線形時間テストが失敗することを確かめる。
-   **AC-03:** AC-01 と同じ壊し方で、`make test-parse-norace` が失敗することを確かめる。
-   **AC-04・AC-05:** 3.3 節のガードのテストで確かめる。レシピの各条件を 1 つずつ外し、テストの名前を変え、ガードが失敗することを確かめる。
-   **AC-06:** PR のレビューで記述を読む（`manual`）。
-   **AC-07:** `make test`・`make lint` と CI で確かめる。
-   壊し方と失敗の確認は、`CLAUDE.md`「Testing Strategy」のとおり、コミットメッセージに書く。

## 8. 実装の優先順位

1.  線形時間テストとビルドタグのファイルを変え、AC-01〜AC-03 の壊し方を確かめる。
2.  `Makefile` のレシピとガードのテストを変え、AC-05 の壊し方を確かめる。
3.  CI の手順の名前、`CLAUDE.md`、ADR-0038 の記述を変える。

## 9. 将来の拡張性

-   ほかに時間を確かめるテストを加えるときも、同じくビルドタグのファイルの定数で切り替え、`test-parse-norace` の `-run` に加えられる。そのとき、`internal/transcript` のガードが確かめる名前の数と、加えたテストのパッケージのガードも直す。
