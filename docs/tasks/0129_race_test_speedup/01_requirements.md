# 要件定義書：-race 下の巨大入力テストの軽量化と独立したテストの並列化による Go テストの高速化

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Process | `full` |
| Created | 2026-11-26 |
| Review date | 2026-11-26 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`make test` の `-race` 付きの実行で時間のかかっている Go のテストを、本番の挙動とテストの検証の強さを落とさずに短くするための要件を定義する（[#1166](https://github.com/isseis/yt2column/issues/1166)）。

**背景:**

`make test` は、全パッケージを `-race` 付きで実行した後、`make test-parse-norace` を実行する（`Makefile:93-95`）。issue #1166 は、`go test -tags test -race -count=1 ./...` のファイルごとの実行時間を計測し、次のテストに時間がかかっていることを示した。

| ファイル | 主な対象テスト | 計測時間（`-race`） | `t.Parallel()` |
|---|---|---:|:---:|
| `internal/transcript/copied_format_test.go` | `TestParseCopiedLargeInputs` | 101.0 秒（ファイル計 104.1 秒） | なし |
| `internal/coveragecompare/run_test.go` | `TestRunAcceptsLargeOutput` | 91.1 秒（issue の表ではファイル計「Leaf 261.9s」） | 一部あり |
| `cmd/yt2column/deadcode_test.go` | `TestMakeDeadcodeScope`（42.0 秒）、`TestMakeDeadcodeVerdict`（23.8 秒） | 65.8 秒 | なし |
| `internal/ytdlpsource/json3_test.go` | `TestParseSubtitlesLimits`（12.7 秒）、`TestParseSubtitlesArrayElementLimit`（11.6 秒） | 28.3 秒 | なし |
| `internal/strictjson/strictjson_test.go` | `TestAsArrayElementLimit`（15.7 秒） | 20.8 秒 | なし |
| `internal/publisher/display_render_test.go` | `TestNeutralizedDisplayMarkdownLinearTime`（14.4 秒） | 14.4 秒 | なし |

今の振る舞いは次のとおりである。

-   **`TestParseCopiedLargeInputs`:**
    -   4 MiB（`copiedLargeInputLimit = 4 << 20`、`internal/transcript/copied_format_test.go:1043`）の上限まで作った 7 つの入力を、直列に解析する（`copied_format_test.go:1061-1106`）。
    -   `-race` 付きのビルドでは、時間を期限と比べず、解析の成功・セグメントの数・メモリの確保量の上限（1 バイトあたり 8,192 バイト）だけを確かめる（`copied_format_race_test.go`）。
    -   `-race` なしのビルドでは、期限 3 秒（`copied_format_test.go:1044`）と、確保量の上限（1 バイトあたり 128 バイト、`copied_format_norace_test.go`）も確かめる。この確認は `make test-parse-norace` が `-run` で絞って実行する（`Makefile:111-112`）。
    -   確保量を `runtime.MemStats.TotalAlloc` の差で測るので、並列に実行しない（`copied_format_test.go:1059-1060` のコメント）。
-   **`TestRunAcceptsLargeOutput`:** fake の go が、1 MiB + 1 KiB の行を 21 行（合わせて 20 MiB 超）出し、それを BASE と HEAD の 2 回、ツールの実行の経路で読ませる（`internal/coveragecompare/run_test.go:848-867`、`internal/coveragecompare/test_helpers.go:124-128`）。これは 0055 の AC-32（入力の大きさに上限を設けない。[0055 要件定義書](../0055_coverage_compare/01_requirements.md)）を実行の経路で確かめるテストである。20 MiB という大きさは要件ではなく、[0055 実装計画書](../0055_coverage_compare/03_implementation_plan.md)のフェーズ 4 で決めた値である。
-   **`TestMakeDeadcodeScope` と `TestMakeDeadcodeVerdict`:** サブテストを直列に実行する（`cmd/yt2column/deadcode_test.go:205-327`、`669-846`）。`TestMakeDeadcodeVerdict` の各ケースは自分の `t.TempDir()` を使い（`deadcode_test.go:225`）、`TestMakeDeadcodeScope` の `isolation` のサブテストは、親が作った 1 つのフィクスチャを共有する（`deadcode_test.go:719-720`）。`TestMakeDeadcodeScope` は `deadcode` コマンドが `PATH` にないとスキップする（`deadcode_test.go:670-673`）。
-   **境界のテスト:** どれも直列に実行する（3 つのファイルに `t.Parallel()` はない）。
    -   `TestParseSubtitlesLimits` は、大きさの上限 `maxSubtitlesBytes` とイベントの上限 `maxSubtitleEvents`（65,536 件）のそれぞれで、上限で受理し、上限を 1 つ超えると拒否することを確かめる（`internal/ytdlpsource/json3_test.go:244-274`）。
    -   `TestParseSubtitlesArrayElementLimit` は、`internal/strictjson` の配列の要素の上限（131,072 件）を 1 つ超える 131,073 件の `segs` を、上限の定数を参照せずリテラルの数値で作り、拒否されることだけを確かめる（`json3_test.go:276-286`）。
    -   `TestAsArrayElementLimit` は、本番の上限 `maxArrayElements` の 1 つ下、上限、1 つ上を確かめる（`internal/strictjson/strictjson_test.go:559`）。
-   **`TestNeutralizedDisplayMarkdownLinearTime`:** 入力を 4 倍にしたときの時間が 8 倍未満であることを確かめる（`internal/publisher/display_render_test.go:227-266`）。時間を測るテストである。

`-race` 付きのビルドで時間以外を確かめ、時間の期限を `-race` なしのビルドだけで確かめる方針は、[ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md)（make deadcode はモジュール全体とテストから届かない関数を報告して失敗し、パースの時間の期限は -race なしのビルドだけで確かめる）の決定である。

対応 issue: [#1166](https://github.com/isseis/yt2column/issues/1166)

**本書の記述範囲:** 本書は、どのテストを何のために変え、何を保ち、短くなったことをどう確かめるかを定める。`-race` 付きのビルドで使う入力の大きさとその置き場所、並列にするサブテストの選び方、大きな出力のテストの大きさと実行の回数、時間の測り方、カバレッジの比べ方は、設計（`02_architecture.md`）で決める。要件の検討で挙がった設計の事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #1166 で決めたこと:**

-   **対象:** 上の表の 6 つのファイルのテストを対象にする。`-race` 付きの実行で、巨大な入力の解析と、独立したサブテストの直列の実行が時間を使っていることが、計測で分かったためである。
-   **改善の方針:** `TestParseCopiedLargeInputs` の `-race` 付きのビルドの入力を小さくし、4 MiB の性能の確認を `make test-parse-norace` に一本化する。`TestRunAcceptsLargeOutput` の大きさを、検証に要る大きさへ見直す。独立したサブテストに `t.Parallel()` を加える。巨大な上限の境界のテストは、`t.Parallel()` の追加と、カバレッジと検証の強さを落とさない範囲での軽量化を検討する。
-   **完了の条件:** 本番の挙動と、テストの検証の強さ（カバレッジと仕様の保証）を落とさずに、対象のテストの `-race` 付きの実行時間が「大幅に短縮」されている。「大幅に」をどう測るかは、「要件定義の際に決めたこと（2026-11-26）」の「短くなったことの確かめ方」で決めた。`make test`（`test-parse-norace` を含む）と `make lint` が通る。

**要件定義の際に決めたこと（2026-11-26）:**

-   **進め方:** 通常の進め方（`full`）をとる。6 つのパッケージに及び、要件の段階で未決の点が複数あり、ADR-0038 の決定を更新するためである。新しい仕組みを作らない点だけは簡易な進め方に向くが、ほかの 3 点は通常の進め方に向く。人がこの推奨のとおりに選んだ。ADR: なし（進め方の選択はこのタスクの中で閉じる）。
-   **除外の判断:** 簡易な進め方の除外には当たらない。テストだけの変更で、`docs/dev/security.md` の規則と前提、開発ツールの信頼の境界（`fixprguard` と `mergepr`）、外部の依存のどれも変えないためである。ADR: なし。
-   **`-race` 付きのビルドの入力を小さくする:** `TestParseCopiedLargeInputs` は、`-race` 付きのビルドでは 4 MiB より小さい入力で、解析の成功・セグメントの数・確保量の上限を確かめる。4 MiB の入力での時間と確保量の確認は、`-race` なしのビルド（`make test-parse-norace`）だけに残す（F-001）。`-race` 付きのビルドの時間は本番の性能を表さず（ADR-0038）、4 MiB での確認は `-race` なしのビルドがより厳しい確保量の上限で続けるためである。ADR: [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) を更新する。`-race` 付きのビルドで何を確かめるかは、ADR-0038 の決定の 1 項目だからである。採らなかった案と理由は次のとおりで、ADR-0038 の採らなかった案にも記す。
    -   **`-race` 付きのビルドでも 4 MiB のままにする:** `-race` 付きの実行で最も時間を使うテストの 1 つのまま残る。
    -   **`-race` 付きのビルドからこのテストを外す（`//go:build !race`）:** ADR-0038 の「`-race` 付きのビルドでも、時間以外は確かめ続ける」に反し、`-race` 付きのビルドが解析の成功・セグメントの数・確保量を確かめなくなる。
    -   **4 MiB の入力を、`make test-parse-norace` だけが実行する別のテストに移す:** 7 つの入力を作る処理と解析を 2 つのテストで重複させる。ADR-0038 が「時間の確認を別のテスト関数に分ける」を採らなかったのと同じ理由である。
-   **本番のコードを変えない:** 境界のテストを軽くするために、本番のコードに、テストから上限を差し替えられる仕組みなどを作らない。境界のテストは本番の上限の値そのもので両側を確かめ続け、短くするのは並列化とテストの側の軽量化だけで行う（F-002、F-004）。issue が本番の挙動を変えないことを求めており、上限を差し替えられるようにすると、テストが本番の上限そのものを確かめなくなるためである。ADR: なし（このタスクの中で閉じる決定で、ほかのタスクや構成要素を縛らない）。
-   **測るテストを並列にしない:** 時間か確保量を測るテスト（`TestParseCopiedLargeInputs` と `TestNeutralizedDisplayMarkdownLinearTime`）は並列にしない。入力を小さくするのは、線形の処理と 2 乗の処理を今と同じく見分けられる範囲に限って認める（F-003）。並列にすると、測定がほかのテストの負荷を受け、結果が不安定になるためである。ADR: なし（このタスクの中で閉じる）。
-   **大きな出力のテストを小さくする:** `TestRunAcceptsLargeOutput` は、1 MiB を超える行を含む出力を実行の経路で判定まで読めることを確かめ続ける。合わせて 20 MiB を超える大きさと、BASE と HEAD の 2 回の実行は保たなくてよい（F-004）。20 MiB という値は 0055 の要件ではなく計画書の値で、0055 の AC-32（上限を設けない）は、どの有限の大きさのテストでも証明しきれないためである。1 MiB を超える行を残すのは、`bufio.Scanner` の既定の上限のような、1 行の小さな上限を確かめ続けるためである。合計の大きさは、このリポジトリの実際の出力の大きさ（約 6.8 MB、[0055 設計書](../0055_coverage_compare/02_architecture.md) §3.4）を超える 8 MiB より大きく保つ（AC-12）。実際の出力より小さな全体の上限が入ると、このリポジトリでの実行が止まるのに、テストが通ってしまうためである。8 MiB から 20 MiB の間の上限は見つからなくなるが、これは小さくすることの代償として受け入れる。ADR: なし（0055 の AC-32 は変えない）。
-   **短くなったことの確かめ方:** 同じ機械と同じコマンドで、変更の前と後に、対象のテストの時間と `-race` 付きの実行全体の実時間をそれぞれ複数回測って記録し、変更したテストのそれぞれが短くなっていれば、この条件を満たしたものとする。全体の実時間は記録するが、合否の判定には使わない。サブテストが 1 つだけで並列化だけをする `TestParseSubtitlesArrayElementLimit` も、記録するが判定には使わない（AC-14）。数値の目標は決めない（F-005）。機械によって時間が違い、issue の計測の環境と同じ数値を約束できないためである。全体の実時間を判定に使わないのは、設計の際の計測（[02_architecture.md](02_architecture.md) 2.1）で、全体の実時間がこのタスクの対象外の `internal/fixprguard` のテストで決まっていると分かったためである。対象のテストを短くした試作でも、全体の実時間は機械の負荷のばらつきの中で縮まなかった（2026-11-26 に設計の際に改めた。人がこの扱いを選んだ）。ADR: なし（このタスクの中で閉じる）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `make test` の `-race` 付きの実行で、上の表の対象のテストが使う時間を短くする。
-   本番のコードの挙動を変えない。
-   各テストが確かめている性質（境界の両側、上限がないこと、線形の時間と確保量、`make deadcode` の判定と範囲）を確かめ続ける。

### 2.2. スコープ (In Scope)

-   `internal/transcript` の `TestParseCopiedLargeInputs` の、`-race` 付きのビルドの入力の大きさ。
-   `internal/coveragecompare` の `TestRunAcceptsLargeOutput` の出力の大きさと実行の回数。
-   `cmd/yt2column` の `TestMakeDeadcodeScope` と `TestMakeDeadcodeVerdict` の、独立したサブテストの並列化。
-   `internal/ytdlpsource` の `TestParseSubtitlesLimits` と `TestParseSubtitlesArrayElementLimit`、`internal/strictjson` の `TestAsArrayElementLimit` の、並列化とテストの側の軽量化。
-   `internal/publisher` の `TestNeutralizedDisplayMarkdownLinearTime` の入力の縮小の検討（並列化はしない）。
-   変更の前と後の時間の計測と、その記録。
-   [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) の更新。

### 2.3. スコープ外 (Out of Scope)

-   **本番のコード（`_test.go` のファイルとテスト用のビルドタグのファイルを除くファイル）の変更。** 「要件定義の際に決めたこと（2026-11-26）」の「本番のコードを変えない」のとおりである。
-   **上の表にないテスト。** issue の計測が示した対象だけを扱う。ほかのテストは、別の issue で計測してから扱う。
-   **`Makefile` と CI（`.github/workflows/`）の、テストの実行の仕方の変更。** `-p`、`-parallel`、`GOMAXPROCS`、`-race` の有無、`make test-parse-norace` の呼び出しを変えない。issue が求めておらず、`make test-parse-norace` の呼び出しは ADR-0038 の決定であるためである。
-   **ブラウザ拡張のテスト。** issue の対象は Go のテストだけである。
-   **パーサやほかの本番の処理の高速化。** ADR-0038 の採らなかった案「パーサを改善する」のとおり、`-race` 付きのビルドの時間は本番の時間を表さない。
-   **時間の期限（`copiedParseDeadline`）と `-race` なしのビルドの確保量の上限の値の変更。**

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: `-race` 付きのビルドの `TestParseCopiedLargeInputs` の入力を小さくする

`-race` 付きのビルドでは、7 つの入力を 4 MiB より小さく作って解析する。`-race` なしのビルドは、今と同じ 4 MiB の上限まで作った入力で確かめる。

**Acceptance Criteria**:
- **AC-01**: `-race` 付きのビルドの `TestParseCopiedLargeInputs` は、7 つの入力（今の 7 つの経路のそれぞれ）を、どれも 4 MiB（4,194,304 バイト）より小さく作って解析し、解析の成功、セグメントの数、1 バイトあたりの確保量の上限を確かめる。テストのログに出る各入力のバイト数が 4,194,304 より小さい。小さくした大きさは、その大きさでの `-race` 付きの実行時間とともに `02_architecture.md` に記す。
- **AC-02**: `-race` なしのビルドの `TestParseCopiedLargeInputs`（`make test-parse-norace` が実行するもの）が解析する 7 つの入力は、変更の前と 1 バイトも違わない。期限（3 秒）と、1 バイトあたりの確保量の上限（128 バイト）の値も変わらない。
- **AC-03**: 7 つの入力のそれぞれについて、確保量が入力の長さの 2 乗に比例するように解析を壊す方法を、入力の大きさを決める前に決める（壊し方は `02_architecture.md` で入力ごとに決める）。壊した解析を使うと、変更の前の `-race` 付きのビルド（4 MiB の入力）でも、変更の後の `-race` 付きのビルド（小さくした入力）でも、テストが確保量の上限を超えて失敗する。これは、壊した解析で実際にテストを実行して確かめ、その結果を記録する。

#### F-002: 独立したサブテストを並列に実行する

書き換えられる共有の状態を持たないテストとサブテストを並列に実行する。

**Acceptance Criteria**:
- **AC-04**: `TestMakeDeadcodeScope`、`TestMakeDeadcodeVerdict`、`TestParseSubtitlesLimits`、`TestParseSubtitlesArrayElementLimit`、`TestAsArrayElementLimit` と、そのサブテストのうち、書き換えられる共有の状態を持たないものは、`t.Parallel()` で並列に実行される。書き換えられる共有の状態とは、あるテストが書き、ほかのテストが読むか書くファイルやディレクトリと、プロセス全体の環境変数や作業ディレクトリの変更（`t.Setenv`、`t.Chdir`、`os.Setenv` など）である。テストが始まる前に書かれ、読むだけの入力（リポジトリの `Makefile`、親のテストが作ったフィクスチャ）と、Go のビルドのキャッシュは、これに当たらない。ただし、並列にするとメモリの使用量、同時に走る子のプロセスの数、時間を測るテストの安定のいずれかに問題が出ることを測って確かめたテストは、直列に残してよい。直列に残すテストは、どちらの理由かを `02_architecture.md` に記す。
- **AC-05**: `TestMakeDeadcodeScope` と `TestMakeDeadcodeVerdict` の、変更の前からの差分は、`t.Parallel()` の呼び出しを加えることと、共有していた状態をサブテストごとに分ける移動（`02_architecture.md` が決める）に限られる。

#### F-003: 時間と確保量を測るテストを直列に保つ

時間か確保量を測るテストは並列にせず、結果を安定させる。

**Acceptance Criteria**:
- **AC-06**: `TestParseCopiedLargeInputs` と `TestNeutralizedDisplayMarkdownLinearTime` と、そのサブテストは、`t.Parallel()` を呼ばない。
- **AC-07**: `TestNeutralizedDisplayMarkdownLinearTime` の入力を小さくする場合、小さくした後の大きさで、今の実装ならば AC-09 の各実行でテストが通り、各 `@` か各 URL の始まりから行の先頭まで戻って探すように壊した実装ならばテストが失敗する。壊した実装でのテストの結果は記録する。小さくしない場合は、その理由を `02_architecture.md` に記す。

#### F-004: 本番の挙動と検証の強さを保つ

本番のコードを変えず、各テストが確かめている性質を確かめ続ける。

**Acceptance Criteria**:
- **AC-08**: このタスクの変更の差分は、`_test.go` のファイル、`//go:build test` のテスト用のファイル（`test_helpers.go` など）、`docs/tasks/0129_race_test_speedup/` の下のファイル、`docs/dev/adr/0038-deadcode-and-race-free-timing.md` に限られる。
- **AC-09**: 変更の後、計測に使う機械で、`go test -tags test -race -count=1 ./...` を 3 回続けて実行し、3 回とも通る（`-count=1` で、キャッシュから再利用した結果を数えない）。同じ機械で `make test`（`make test-parse-norace` を含む）と `make lint` も通る。CI の `Test` ジョブは、同じコミットで 3 回続けて通る。
- **AC-10**: `TestParseSubtitlesLimits`、`TestParseSubtitlesArrayElementLimit`、`TestAsArrayElementLimit` の、変更の前からの差分は、`t.Parallel()` の呼び出しを加えることと、入力の作り方の変更に限られる。入力の作り方を変えても、確かめる関数に渡すバイト列は変更の前と 1 バイトも違わない。
- **AC-11**: 本番のコードの関数ごとの文のカバレッジが、変更の前より下がらない。
- **AC-12**: `TestRunAcceptsLargeOutput` は、1 MiB（1,048,576 バイト）を超える行を 1 行以上含み、合わせて 8 MiB（8,388,608 バイト）を超える `-json` の出力を、ツールの実行の経路（子のプロセスとして起動した fake の go の出力をツールが読む経路）で判定まで読み、合格の判定になることを確かめる。8 MiB は、このリポジトリの実際の `-json` の出力の大きさ（約 6.8 MB、[0055 設計書](../0055_coverage_compare/02_architecture.md) §3.4）を超える値である。

#### F-005: 短くなったことを測って記録する

変更の前と後の時間を同じ条件で測り、実装計画書に記録する。

**Acceptance Criteria**:
- **AC-13**: 同じ機械と同じコマンドで、変更の前と後に、それぞれ 3 回以上、上の表の対象のテストのそれぞれと、`go test -tags test -race -count=1 ./...` の実時間を測り、測った機械、コマンド、各回の結果を実装計画書に記録する。対象のテストの時間は、そのテストだけを `-run` で絞った `go test -tags test -race -count=1 -run '^<テストの名前>$' <パッケージ>` の実時間とする（親のテストの `go test -json` の `Elapsed` は、並列にしたサブテストの時間を含まないためである）。`TestMakeDeadcodeScope` を計測するには、`deadcode` コマンドがある機械で計測する。`deadcode` コマンドがない機械では、AC-13 は満たせない。
- **AC-14**: AC-13 の記録で、このタスクが変えた対象のテストのそれぞれの時間について、変更の後の中央値が変更の前の最小値より小さい。全体の実時間は AC-13 のとおり記録するが、この比較には含めない。変えなかった対象のテスト（AC-07 で小さくしなかった場合の `TestNeutralizedDisplayMarkdownLinearTime` など）と、`TestParseSubtitlesArrayElementLimit` は、この比較から外す。`TestParseSubtitlesArrayElementLimit` はサブテストが 1 つだけで、`t.Parallel()` を加えても単独の実行では短くならないためである。その時間も AC-13 のとおり記録する。比較で短くならなかった対象があれば、その結果を記録し、設計に戻って扱いを決め直す。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   短くする対象は、`make test` の `-race` 付きの実行での、対象のテストの時間である。全体の実時間は記録するだけで、短くなることは求めない（AC-14）。数値の目標は決めない（「要件定義の際に決めたこと（2026-11-26）」の「短くなったことの確かめ方」）。
-   `make test-parse-norace` の時間は、このタスクでは短くしない。`-race` なしの 4 MiB の確認はそのまま残すからである（AC-02）。

### 4.2. セキュリティ (Security)

-   N/A。テストだけの変更で、`yt-dlp` の起動、API キー、キャッシュのディレクトリ、ネットワークへの送信、信頼できないテキストの扱いのどれも変えないためである（条件付きのガイド `docs/dev/security.md` の対象に当たらない）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   並列化によって、テストの結果が実行ごとに変わる（不安定になる）ことがあってはならない。時間と確保量を測るテストを並列にしないのは、このためである（AC-06）。変更の後に、キャッシュを使わない実行を手元と CI で 3 回続けて確かめる（AC-09）。

### 4.4. 互換性 (Compatibility)

-   計測に使う機械と CI（Linux）の両方で、テストが通り、`make lint` も通る（AC-09）。

### 4.5. 保守性 (Maintainability)

-   `-race` 付きのビルドと `-race` なしのビルドで違う値（入力の大きさを含む）は、ADR-0038 の決定のとおり、ビルドタグ `race` と `!race` のテストのファイルに置く。置き方の詳細は設計で決める（[design_handoff.md](design_handoff.md) H-01）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) の決定（時間の期限は `-race` なしのビルドだけで確かめ、`-race` 付きのビルドでも時間以外は確かめ続ける）に従う。このタスクはその決定を、`-race` 付きのビルドの入力を小さくする形に更新する。
-   テストを削除する場合（サブテストをまとめて減らす場合を含む）は、[CLAUDE.md](../../../CLAUDE.md) の「Testing Strategy」の「Deleting a test is a claim that must be checked」に従い、削除だけのコミットでカバレッジを比べる。
-   変えたテストは、[CLAUDE.md](../../../CLAUDE.md) の「Testing Strategy」の「Every test must be able to fail for its stated reason」に従い、確かめる対象を壊して失敗することを確かめる。
-   `internal/transcript` の、`Makefile` のレシピを守るテスト（`makefile_test.go`）が、`make test-parse-norace` の呼び出しを確かめている（ADR-0038 の「結果」）。このタスクはその呼び出しを変えない。

## 6. 用語集 (Glossary)

-   **`-race` 付きのビルド:** `go test -race` でデータ競合の検出器を入れたビルド。`make test` の最初の実行と CI の `Test` ジョブがこれを使う。
-   **`-race` なしのビルド:** データ競合の検出器を入れないビルド。本番の CLI と拡張の記事生成モジュールはこれでビルドされる。`make test-parse-norace` がこれで `TestParseCopiedLargeInputs` を実行する。
-   **確保量:** テストが `runtime.MemStats.TotalAlloc` の差で測る、解析が確保したメモリのバイト数。
-   **実時間:** 処理に実際にかかった時間（`docs/translation_glossary.md`）。
-   **境界のテスト:** 本番の上限の値そのものの大きさの入力で、上限で受理し、上限を 1 つ超えると拒否することを確かめるテスト。

## 変更履歴

- 2026-11-26 承認（isseis、#1170）
- 2026-11-26 draft への戻し：AC-14 で全体の実時間を記録だけにし、短くなったかの判定を対象のテストの単独の実行で行う；AC-14 の比較から TestParseSubtitlesArrayElementLimit を外す（#1179）
- 2026-11-26 承認（isseis、#1179）
