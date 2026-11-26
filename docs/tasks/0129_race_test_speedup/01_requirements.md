# 要件定義書：-race 下の巨大入力テストの軽量化と独立したテストの並列化による Go テストの高速化

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-26 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`make test` の `-race` 付きの実行で時間を多く使っている Go のテストを、本番の挙動とテストの検証の強さを落とさずに短くするための要件を定義する（[#1166](https://github.com/isseis/yt2column/issues/1166)）。

**背景:**

`make test` は、全パッケージを `-race` 付きで実行した後、`make test-parse-norace` を実行する（`Makefile:93-95`）。issue #1166 は、`go test -tags test -race -count=1 ./...` のファイルごとの実行時間を計測し、次のテストが時間を多く使っていることを示した。

| ファイル | 主な対象テスト | 計測時間（`-race`） |
|---|---|---:|
| `internal/transcript/copied_format_test.go` | `TestParseCopiedLargeInputs` | 101.0 秒（ファイル計 104.1 秒） |
| `internal/coveragecompare/run_test.go` | `TestRunAcceptsLargeOutput` | 91.1 秒 |
| `cmd/yt2column/deadcode_test.go` | `TestMakeDeadcodeScope`（42.0 秒）、`TestMakeDeadcodeVerdict`（23.8 秒） | 65.8 秒 |
| `internal/ytdlpsource/json3_test.go` | `TestParseSubtitlesLimits`（12.7 秒）、`TestParseSubtitlesArrayElementLimit`（11.6 秒） | 28.3 秒 |
| `internal/strictjson/strictjson_test.go` | `TestAsArrayElementLimit`（15.7 秒） | 20.8 秒 |
| `internal/publisher/display_render_test.go` | `TestNeutralizedDisplayMarkdownLinearTime`（14.4 秒） | 14.4 秒 |

今の振る舞いは次のとおりである。

-   **`TestParseCopiedLargeInputs`:** 4 MiB（`copiedLargeInputLimit = 4 << 20`、`internal/transcript/copied_format_test.go:1043`）の上限まで作った 7 つの入力を、直列に解析する（`copied_format_test.go:1061-1106`）。`-race` 付きのビルドでは時間を期限と比べず、解析の成功・セグメントの数・メモリの確保量の上限（1 バイトあたり 8,192 バイト）だけを確かめる（`copied_format_race_test.go`）。`-race` なしのビルドは、期限 3 秒（`copied_format_test.go:1044`）と、確保量の上限（1 バイトあたり 128 バイト、`copied_format_norace_test.go`）を確かめる。`-race` なしの確認は `make test-parse-norace` が `-run` で絞って実行する（`Makefile:111-112`）。このテストは確保量を `runtime.MemStats.TotalAlloc` の差で測るので、並列に実行しない（`copied_format_test.go:1059-1060` のコメント）。
-   **`TestRunAcceptsLargeOutput`:** 偽の go が、1 MiB + 1 KiB の行を 21 行（合わせて 20 MiB 超）出し、それを BASE と HEAD の 2 回、道具の実行の経路で読ませる（`internal/coveragecompare/run_test.go:848-867`、`internal/coveragecompare/test_helpers.go:124-128`）。これは 0055 の AC-32（入力の大きさに上限を設けない。[0055 要件定義書](../0055_coverage_compare/01_requirements.md)）を実行の経路で確かめるテストである。20 MiB という大きさは要件ではなく、[0055 実装計画書](../0055_coverage_compare/03_implementation_plan.md)のフェーズ 4 で決めた値である。
-   **`TestMakeDeadcodeScope` と `TestMakeDeadcodeVerdict`:** 各サブテストが自分の `t.TempDir()` を使うが、直列に実行する（`cmd/yt2column/deadcode_test.go:205-224`、`669-831`）。`TestMakeDeadcodeScope` は `deadcode` コマンドが `PATH` にないとスキップする（`deadcode_test.go:670-673`）。
-   **境界のテスト:** `TestParseSubtitlesLimits` と `TestParseSubtitlesArrayElementLimit` は、本番の上限（大きさの上限、65,536 件のイベント、131,072 件の配列の要素）そのものの大きさの入力で、上限の両側を確かめる（`internal/ytdlpsource/json3_test.go:244-285`）。`TestAsArrayElementLimit` も、本番の上限 `maxArrayElements` の両側を確かめる（`internal/strictjson/strictjson_test.go:559`）。どれも直列に実行する。
-   **`TestNeutralizedDisplayMarkdownLinearTime`:** 入力を 4 倍にしたときの時間が 8 倍未満であることを確かめる（`internal/publisher/display_render_test.go:227-266`）。時間を測るテストである。

`-race` 付きのビルドで時間以外を確かめ、時間の期限を `-race` なしのビルドだけで確かめる方針は、[ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md)（make deadcode はモジュール全体とテストから届かない関数を報告して失敗し、パースの時間の期限は -race なしのビルドだけで確かめる）の決定である。

対応 issue: [#1166](https://github.com/isseis/yt2column/issues/1166)

**本書の記述範囲:** 本書は、どのテストを何のために変え、何を保ち、短くなったことをどう確かめるかを定める。`-race` 付きのビルドで使う入力の大きさとその置き場所、並列にするサブテストの選び方、大きな出力のテストの大きさと実行の回数、時間の測り方、カバレッジの比べ方は、設計（`02_architecture.md`）で決める。要件の検討で挙がった設計の事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #1166 で決めたこと:**

-   **対象:** 上の表の 6 つのファイルのテストを対象にする。`-race` 付きの実行で、巨大な入力の解析と、独立したサブテストの直列の実行が時間を使っていることが、計測で分かったためである。
-   **改善の方針:** `TestParseCopiedLargeInputs` の `-race` 付きのビルドの入力を小さくし、4 MiB の性能の確認を `make test-parse-norace` に一本化する。`TestRunAcceptsLargeOutput` の大きさを、検証に要る大きさへ見直す。独立したサブテストに `t.Parallel()` を加える。巨大な上限の境界のテストは、`t.Parallel()` の追加と、カバレッジと検証の強さを落とさない範囲での軽量化を検討する。
-   **完了の条件:** 本番の挙動と、テストの検証の強さ（カバレッジと仕様の保証）を落とさずに、対象のテストの `-race` 付きの実行時間を短くする。`make test`（`test-parse-norace` を含む）と `make lint` が通る。

**要件定義の際に決めたこと（2026-11-26）:**

-   **進め方:** 通常の進め方（`full`）をとる。6 つのパッケージに及び、要件の段階で未決の点が複数あり、ADR-0038 の決定を更新するためである。新しい仕組みを作らない点だけは簡易な進め方に向くが、ほかの 3 点は通常の進め方に向く。人がこの推奨のとおりに選んだ。ADR: なし（進め方の選択はこのタスクの中で閉じる）。
-   **除外の判断:** 簡易な進め方の除外には当たらない。テストだけの変更で、`docs/dev/security.md` の規則と前提、開発ツールの信頼の境界（`fixprguard` と `mergepr`）、外部の依存のどれも変えないためである。ADR: なし。
-   **`-race` 付きのビルドの入力を小さくする:** `TestParseCopiedLargeInputs` は、`-race` 付きのビルドでは 4 MiB より小さい入力で、解析の成功・セグメントの数・確保量の上限を確かめる。4 MiB の入力での時間と確保量の確認は、`-race` なしのビルド（`make test-parse-norace`）だけに残す（F-001）。`-race` 付きのビルドの時間は本番の性能を表さず（ADR-0038）、4 MiB での確認は `-race` なしのビルドがより厳しい確保量の上限で続けるためである。ADR: [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) を更新する。`-race` 付きのビルドで何を確かめるかは、ADR-0038 の決定の 1 項目だからである。
-   **本番のコードを変えない:** 境界のテストを軽くするために、本番のコードに、テストから上限を差し替える口などを作らない。境界のテストは本番の上限の値そのもので両側を確かめ続け、短くするのは並列化とテストの側の軽量化だけで行う（F-002、F-005）。issue が本番の挙動を変えないことを求めており、上限を差し替えられるようにすると、テストが本番の上限そのものを確かめなくなるためである。ADR: なし（このタスクの中で閉じる決定で、ほかのタスクや構成要素を縛らない）。
-   **測るテストを並列にしない:** 時間か確保量を測るテスト（`TestParseCopiedLargeInputs` と `TestNeutralizedDisplayMarkdownLinearTime`）は並列にしない。入力を小さくするのは、線形の処理と 2 乗の処理を今と同じく見分けられる範囲に限って認める（F-003）。並列にすると、測定がほかのテストの負荷を受け、結果が不安定になるためである。ADR: なし（このタスクの中で閉じる）。
-   **大きな出力のテストを小さくする:** `TestRunAcceptsLargeOutput` は、1 MiB を超える行を含む出力を実行の経路で判定まで読めることを確かめ続ける。合わせて 20 MiB を超える大きさと、BASE と HEAD の 2 回の実行は保たなくてよい（F-004）。20 MiB という値は 0055 の要件ではなく計画書の値で、0055 の AC-32（上限を設けない）は、どの有限の大きさのテストでも証明しきれないためである。1 MiB を超える行は、`bufio.Scanner` の既定の上限などの小さな上限がないことを示す。ADR: なし（0055 の AC-32 は変えない）。
-   **短くなったことの確かめ方:** 同じ機械と同じコマンドで、変更の前と後に、対象のテストの時間と `-race` 付きの実行全体の実時間を測って記録し、対象のテストのそれぞれと全体が短くなっていれば満たすとする。数値の目標は決めない（F-006）。機械によって時間が違い、issue の計測の環境と同じ数値を約束できないためである。ADR: なし（このタスクの中で閉じる）。

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
-   `internal/publisher` の `TestNeutralizedDisplayMarkdownLinearTime` の入力の縮小（並列化はしない）。
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
- **AC-01**: `-race` 付きのビルドの `TestParseCopiedLargeInputs` は、7 つの入力（今の 7 つの経路のそれぞれ）を、どれも 4 MiB（4,194,304 バイト）より小さく作って解析し、解析の成功、セグメントの数、1 バイトあたりの確保量の上限を確かめる。テストのログに出る各入力のバイト数が 4,194,304 より小さい。
- **AC-02**: `-race` なしのビルドの `TestParseCopiedLargeInputs`（`make test-parse-norace` が実行するもの）は、7 つの入力を、それぞれ 4 MiB の上限を超えない範囲で今と同じ作り方で作って解析し、解析の成功、セグメントの数、3 秒の期限、1 バイトあたり 128 バイトの確保量の上限を確かめる。
- **AC-03**: `-race` 付きのビルドで使う大きさの入力でも、1 バイトあたりの確保量の上限は、確保量が入力の長さの 2 乗に比例する解析なら上限を超えて失敗する大きさである。7 つの入力のどれについても成り立つ。

#### F-002: 独立したサブテストを並列に実行する

互いに状態を共有しないサブテストを並列に実行する。

**Acceptance Criteria**:
- **AC-04**: `TestMakeDeadcodeScope` と `TestMakeDeadcodeVerdict` のサブテストのうち、ほかのサブテストと状態（ファイル、プロセスの環境変数、作業ディレクトリ）を共有しないものは、`t.Parallel()` で並列に実行される。共有するために並列にしないサブテストがあれば、その理由を `02_architecture.md` に記す。
- **AC-05**: `TestParseSubtitlesLimits`、`TestParseSubtitlesArrayElementLimit`、`TestAsArrayElementLimit` と、そのサブテストのうち、状態を共有しないものは、`t.Parallel()` で並列に実行される。
- **AC-06**: `TestMakeDeadcodeScope` と `TestMakeDeadcodeVerdict` は、変更の後も同じケースのそれぞれについて、今と同じ項目（`make` の終了コード、標準出力、標準エラーの行、スタブの引数と実行の有無、一時ファイルの後始末、報告される関数）を確かめる。

#### F-003: 時間と確保量を測るテストを直列に保つ

**Acceptance Criteria**:
- **AC-07**: `TestParseCopiedLargeInputs` と `TestNeutralizedDisplayMarkdownLinearTime` と、そのサブテストは、`t.Parallel()` を呼ばない。
- **AC-08**: `TestNeutralizedDisplayMarkdownLinearTime` の入力を小さくする場合、小さくした後も、線形の処理では入力を 4 倍にしたときの時間の比が 8 倍未満になり、2 乗の処理（各 `@` や各 URL の始まりから行の先頭まで戻って探す処理）では 8 倍以上になる。

#### F-004: 大きな出力のテストを小さくする

**Acceptance Criteria**:
- **AC-09**: `TestRunAcceptsLargeOutput` は、1 MiB（1,048,576 バイト）を超える行を 1 行以上含む `-json` の出力を、道具の実行の経路（`runTool`）で判定まで読み、合格の判定になることを確かめる。

#### F-005: 本番の挙動と検証の強さを保つ

**Acceptance Criteria**:
- **AC-10**: このタスクの変更の差分は、`_test.go` のファイル、`//go:build test` のテスト用のファイル（`test_helpers.go` など）、`docs/tasks/0129_race_test_speedup/` の下のファイル、`docs/dev/adr/0038-deadcode-and-race-free-timing.md` に限られる。
- **AC-11**: `TestParseSubtitlesLimits`、`TestParseSubtitlesArrayElementLimit`、`TestAsArrayElementLimit` は、変更の後も、本番の上限の値（`maxSubtitlesBytes`、`maxSubtitleEvents`、`maxArrayElements`）そのものを使い、上限で受理し上限を 1 つ超えると拒否することを、今と同じ組み合わせについて確かめる。
- **AC-12**: 本番のコードの関数ごとの文のカバレッジが、変更の前より下がらない。
- **AC-13**: 変更の後、`make test`（`make test-parse-norace` を含む）と `make lint` が通る。

#### F-006: 短くなったことを測って記録する

**Acceptance Criteria**:
- **AC-14**: 同じ機械と同じコマンドで、変更の前と後に、上の表の対象のテストのそれぞれの時間と、`go test -tags test -race -count=1 ./...` の実時間を測り、測った機械、コマンド、結果を実装計画書に記録する。`TestMakeDeadcodeScope` を測るために、`deadcode` コマンドがある機械で測る。
- **AC-15**: AC-14 の記録で、対象のテストのそれぞれの時間（並列にしたテストは、それを含むパッケージの時間）と、全体の実時間が、変更の前より短い。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   短くする対象は `make test` の `-race` 付きの実行の時間である。数値の目標は決めない（「要件定義の際に決めたこと（2026-11-26）」の「短くなったことの確かめ方」）。
-   `make test-parse-norace` の時間は、このタスクでは短くしない。`-race` なしの 4 MiB の確認はそのまま残すからである（AC-02）。

### 4.2. セキュリティ (Security)

-   N/A。テストだけの変更で、`yt-dlp` の起動、API キー、キャッシュのディレクトリ、ネットワークへの送信、信頼できないテキストの扱いのどれも変えないためである（条件付きのガイド `docs/dev/security.md` の対象に当たらない）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   並列化によって、テストの結果が実行ごとに変わる（不安定になる）ことがあってはならない。時間と確保量を測るテストを並列にしないのは、このためである（AC-07）。

### 4.4. 互換性 (Compatibility)

-   手元（macOS）と CI（Linux）の両方で、`make test` と `make lint` が通る（AC-13）。

### 4.5. 保守性 (Maintainability)

-   `-race` 付きのビルドと `-race` なしのビルドで違う値は、今と同じくビルドタグ `race` と `!race` のテストのファイルに置くかどうかを、設計で決める（[design_handoff.md](design_handoff.md) H-01）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) の決定（時間の期限は `-race` なしのビルドだけで確かめ、`-race` 付きのビルドでも時間以外は確かめ続ける）に従う。このタスクはその決定を、`-race` 付きのビルドの入力を小さくする形に更新する。
-   テストを削除する場合（サブテストをまとめて減らす場合を含む）は、[CLAUDE.md](../../../CLAUDE.md) の「Testing Strategy」の「Deleting a test is a claim that must be checked」に従い、削除だけのコミットでカバレッジを比べる。
-   変えたテストは、[CLAUDE.md](../../../CLAUDE.md) の「Testing Strategy」の「Every test must be able to fail for its stated reason」に従い、確かめる対象を壊して失敗することを確かめる。
-   `make test` は `internal/transcript` のレシピのガードのテストで、`make test-parse-norace` の呼び出しを確かめている（ADR-0038 の「結果」）。このタスクはその呼び出しを変えない。

## 6. 用語集 (Glossary)

-   **`-race` 付きのビルド:** `go test -race` でデータ競合の検出器を入れたビルド。`make test` の最初の実行と CI の `Test` ジョブがこれを使う。
-   **`-race` なしのビルド:** データ競合の検出器を入れないビルド。本番の CLI と拡張の記事生成モジュールはこれでビルドされる。`make test-parse-norace` がこれで `TestParseCopiedLargeInputs` を実行する。
-   **確保量:** テストが `runtime.MemStats.TotalAlloc` の差で測る、解析が確保したメモリのバイト数。
-   **実時間:** 処理に実際にかかった時間（`docs/translation_glossary.md`）。
-   **境界のテスト:** 本番の上限の値そのものの大きさの入力で、上限で受理し、上限を 1 つ超えると拒否することを確かめるテスト。
