# 実装計画書：字幕の `tStartMs` の読み取りを `strictjson` の負でない整数の読み取りに一本化する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-16 |
| Review date | 2026-10-17 |
| Reviewer | isseis |
| Comments | 実装中のチェックボックス更新（判断の変更なし）。レビューを受け、受理の表への統合とガードテスト（AC-11 はパッケージ全体、AC-12 は 0 境界の比較のみ）の記述を実装に合わせた（作るものは変えない）。 |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番を指し、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。用語は requirements §6 に従う。既存のコードとテストの行番号は、断りがない限り HEAD `581b39c` で確かめた。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements の 4 つの機能要件（F-001〜F-004）を architecture の設計どおりに実装する。字幕（json3）の消費するイベント（`segs` を持ち、`utf8` を連結した本文が空白だけではないイベント。requirements §6）の `tStartMs` を、`internal/transcript` の自前の `decodeTimestamp`（`AsInt64` の後に負の値を拒否する）ではなく `internal/strictjson.Value.AsNonNegativeInt64` で読み（F-001）、「負でない整数」の規則をコードベースで 1 つにする。拒否のエラーの契約（`ErrParseSubtitles` を包む `*ParseError`・`ErrNoSubtitles` にマッチしない・部分結果を返さない・値の文字列を含めない）を要件どおりにする（F-002）。規則が 1 か所であること（F-003）と、`-0` 以外の受理・拒否が変わらないこと（F-004）をテストで固定する。

architecture §8 の実装優先順位はフェーズ 1 だけである。本計画もフェーズ 1 のみとし、1 つの PR で実装する。

### 1.2. 実装原則

- architecture §1 の設計原則に従う。新しい型・インターフェース・エラー型・依存を増やさず、規則を `AsNonNegativeInt64` の 1 か所に置く。
- 変更するファイルは `internal/transcript/json3.go`（本番）と `internal/transcript/json3_test.go`（テスト）だけである（architecture §3）。新しいファイル・`testutil/` ヘルパ・`test_helpers*.go` は作らない。
- Go のコメント・識別子・文字列リテラルは英語で書く。`AC-NN`・`F-NNN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- 変更した読み取りの分岐を対象とするテストは、その分岐を本タスクの前の `decodeTimestamp` に戻して落ちることを確かめ、そのことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。手順は §4.2 に従う。
- 各フェーズの完了条件は、グリーンゲート（`_context.md` の "Green gate"、`make test && make lint`）と `make fmt` が通ることである。

### 1.3. 既存コード調査結果

- **`decodeEvent`（`internal/transcript/json3.go:88-117`）。** 消費するイベントの `tStartMs` を `members.Collect("tStartMs")`（`:104`）で取り出し、`decodeTimestamp`（`:112`）で読む。本計画は `:112` を `start.AsNonNegativeInt64()` に置き換える。欠落の `errMissingTimestamp`（`:110`）と重複の `Collect` の規則は変えない。
- **`decodeTimestamp`（`json3.go:148-158`）と `errNegativeTimestamp`（`json3.go:26`）は、本計画で削除する。** リポジトリ全体を `decodeTimestamp`・`errNegativeTimestamp` で検索した結果（HEAD `581b39c`）、参照は `json3.go` の宣言と `:112`・`:155` だけである。テストで `errors.Is(err, errNegativeTimestamp)` や `"tStartMs is negative"` を確かめているものは無い（`internal/transcript` の全 `_test.go` を検索）。したがって、削除後に残る参照は無く、仮に残ればコンパイルエラーになる。
- **`strictjson.Value.AsNonNegativeInt64`（`internal/strictjson/strictjson.go:249`）は変更しない。** `-1`・`-0`・小数（`1.0`・`1.5`）・指数（`1e3`・`1E3`）・`int64` を超える数・文字列・真偽値・`null`・配列・オブジェクトを拒否し、エラーに値を含めないことは、既存の `TestAsNonNegativeInt64`（`internal/strictjson/strictjson_test.go:600`）が既に確かめている。本計画はこれを再利用する（AC-13）。
- **`strictjson.Value.AsInt64`（`strictjson.go:225`）は削除しない。** 中間生成物の形式の版（`internal/artifact/format.go:228`）が使い続ける（architecture §3）。
- **再利用するテストとヘルパ（`internal/transcript/json3_test.go`）。**
  - `TestParseSubtitlesTimestamps`（`:110`）と `assertSubtitlesRejected`（`:288`。`ErrParseSubtitles`・`ErrNoSubtitles` にマッチしないこと・`*ParseError.Path`・`nil` の `[]Segment` を確かめる）。拒否のケースはこの表に足す。
  - `TestParseSubtitlesDuplicateMembers`（`:159`）の `duplicate tStartMs`（`:162`）は、`tStartMs` の重複の拒否を既に確かめている（AC-06）。
  - `TestParseSubtitlesRealData`（`:15`）と `subtitlesOracle`（`:311`）は、実際のサンプルの結果を固定する（AC-14）。
  - `TestParseSubtitlesTimestamps` の `zero is valid`（`:125`）と `discarded event ignores tStartMs`（`:136`）は、受理と捨てるイベントの基準になる。
- **`internal/transcript/errors.go:10-19` の exported の番兵**は `ErrInvalidVideoURL`・`ErrYtDlpExec`・`ErrParseSubtitles`・`ErrParseInfo`・`ErrNoSubtitles`・`ErrCopiedTranscriptFormat` の 6 つである。`internal/transcript` の exported の番兵を固定する既存のガードテストは無い（`internal/pipeline/pipeline_test.go` は型のフィールド集合・インターフェースの doc コメント・`testutil` の build tag を固定するが、本パッケージの番兵は対象外）。
- **`internal/pipeline` のガードテストは更新しない。** `TestCommonTypesFieldSets`（`pipeline_test.go:432`）は exported の型のフィールド集合、`TestInterfaceDocComments`（`:511`）は 4 つのインターフェースの doc コメント、`TestFakesCarryBuildTag`（`:593`）は `testutil/`・`test_helpers*.go` の第一行を固定する。本計画はこれらを変更せず、`Segment` のフィールド集合もインターフェースの doc コメントも `testutil/` のファイルも変えないので、いずれも通る。
- **文書は更新しない。** `docs/dev/developer_guide/package_reference.md` の `internal/transcript` の行は `tStartMs` の読み取りの関数名を記述せず、`internal/strictjson` の行は既に `AsNonNegativeInt64` を記す。`cmd/yt2column/docs_test.go:749` のパッケージ表に `AsNonNegativeInt64` は記録済みで、本計画は追加を要しない。
- **`make deadcode`（`Makefile:216-217`）は、本計画のゲートに使わない。** HEAD `581b39c` で実行すると exit 0 のまま `internal/writer/test_helpers.go` などのテスト専用のヘルパを「unreachable func」として列挙し、パス／失敗の判定に使えない。`decodeTimestamp` の削除で参照を残さないことは、`make test` のコンパイル（削除した識別子への参照はコンパイルエラーになる）と、ステップ 1-4 の `TestTimestampReadingIsUnified` で確かめる。
- **外部前提。** 標準ライブラリだけを使い、外部モジュール・外部コマンド・ネットワーク・CI のスキーマ・コンテナを新たに使わない（requirements §5・`CLAUDE.md`「Dependencies」）。検証は Go のテストだけであり、確かめる外部前提は無い。

### 1.4. 申し送りへの対応

`design_handoff.md` と `implementation_handoff.md` は無い。対応すべき `H-NN`・`I-NN` の項目は無い。

## 2. 実装ステップ (Implementation Steps)

各ステップは、対象ファイルと、そのステップだけで判定できる成功基準を述べる（`requirements_process.md` §3）。フェーズ末尾のグリーンゲートをそのまま残す。

### フェーズ 1: 読み取りの置き換えと削除

**対象ファイル:** `internal/transcript/json3.go`（変更）、`internal/transcript/json3_test.go`（変更）

- [x] **ステップ 1-1**: `json3.go` の `decodeEvent`（`:112`）の `decodeTimestamp(start)` を `start.AsNonNegativeInt64()` に置き換える。`decodeTimestamp`（`:148-158`）と `errNegativeTimestamp`（`:26`）を削除する。`errMissingTimestamp` と `Collect` の規則は変えない。（対象: `internal/transcript/json3.go`。成功基準: `json3.go` に `AsNonNegativeInt64` があり、`decodeTimestamp`・`errNegativeTimestamp`・`AsInt64` が無い。`make build` が通る）
- [x] **ステップ 1-2**: `TestParseSubtitlesTimestamps`（`json3_test.go:110`）の `cases` の表に、`-0`（AC-03）と、`1.0`・`1e3`・`true`・`[]`・`{}`（AC-05 の小数・指数・真偽値・配列・オブジェクトのうち既存の行が無いもの）を加える。受理は `zero is valid` の 1 ケースを `0`・正の整数・`int64` の最大値（`9223372036854775807`）を受理して `Segment.StartMs` を確かめる 1 つの表にまとめる（AC-01。`0` のケースは既存の `zero is valid` の内容を引き継ぐ）。`discarded event ignores tStartMs`（`:136`）のサブテストに、捨てる 2 つの形（`segs` がない、`utf8` を連結した本文が空白だけ）のそれぞれについて、`-0` の `tStartMs` を持つイベントが検査されないケースを加える（AC-07）。既存の拒否の行は変えない。（対象: `internal/transcript/json3_test.go`。成功基準: 追加した拒否の行が `assertSubtitlesRejected` を通り、受理の表が通る）
- [x] **ステップ 1-3**: `json3_test.go` に `TestParseSubtitlesTimestampErrorOmitsValue` を加える。`-1`・`-0`・`9223372036854775808`・`1.5`・`1e3` の `tStartMs` を `parseSubtitles` に渡し、まず各呼び出しが `nil` でないエラーを返すことを確かめたうえで（`-0` は本タスクの前は受理されるので、この確認が要る）、そのエラーの `Error()` が値の文字列を含まないことを確かめる（AC-09）。`assertSubtitlesRejected`（`ErrParseSubtitles`・`*ParseError.Path`・部分結果を返さないことを確かめ、エラーメッセージは見ない）とは別に、`parseSubtitles` の戻り値のエラー文字列を直接調べる。（対象: `internal/transcript/json3_test.go`。成功基準: 5 つの値のそれぞれで、`err` が `nil` でなく、エラー文字列に値の文字列が含まれない）
- [x] **ステップ 1-4**: `json3_test.go` に、AC-11・AC-12 のガードテストを加える（architecture §7）。`TestTimestampReadingIsUnified` は `json3.go` を解析し、`tStartMs` の読み取りが `AsNonNegativeInt64` であり、`AsInt64`・`decodeTimestamp`・`errNegativeTimestamp` が残っておらず、`AsNonNegativeInt64` が返した値を保持する識別子に対する自前の負でない整数の判定（`< 0`・`<= -1`・`>= 0`・`> -1` など、0 を境界として負を拒否するか非負を受理する比較。`<= -1` など表記が異なる同値の形を含む）が残っていないことを確かめる（AC-12）。文字列の照合ではなく、Go のソースを解析して比較の式を調べる方法で確認し、特定の表記に依存しない。`v > 0` のような正の値だけを受理する別の規則や、読み取り値と無関係な比較（`len(x) > 0`）は対象外とする。`TestExportedSentinelErrorsUnchanged` はパッケージの非テストの Go ファイルを読み、`Err` で始まる exported の番兵の宣言の集合が §1.3 の 6 つと一致することを確かめる（AC-11）。どちらもソースを読むガードであり、対象のファイルのパスはテストの作業ディレクトリ（パッケージディレクトリ）からの相対で渡す。（対象: `internal/transcript/json3_test.go`。成功基準: 2 つのガードテストが通り、番兵を 1 つ足す・読み取りを `decodeTimestamp` に戻す（`AsInt64` と負の値判定が現れる）・自前の負でない整数の判定を別の表記で戻す、のいずれかで落ちる）
- [x] **ステップ 1-5**: フェーズ 1 の完了条件として、`make fmt` → `make test` → `make lint` を通す。§4.2 の壊し確認を行い、結果をコミットメッセージに記録する。既存の拒否のケース（`TestParseSubtitlesTimestamps` の既存の行・`TestParseSubtitlesDuplicateMembers`）と、`internal/strictjson` の `TestAsNonNegativeInt64` が引き続き通ることを確かめる（AC-13・AC-15）。

**完了条件:** 消費するイベントの `tStartMs` が `AsNonNegativeInt64` で読まれ、`-0` が拒否され、`-0` 以外の受理・拒否が変わらず、拒否のエラーが値の文字列を含まない。規則が 1 か所であることと exported の番兵の集合が変わらないことがガードテストで固定されている。

### PR-1 作成ポイント: unify the tStartMs non-negative integer rule

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `refactor(0023): unify the tStartMs non-negative integer rule`

**レビュー観点**: `decodeEvent` が `AsNonNegativeInt64` で読み、`decodeTimestamp`・`errNegativeTimestamp` を削除していること / 変更が `-0` の拒否だけであり、`0`・正の整数・`int64` の最大値の受理と、ほかの拒否の形が変わっていないこと / 拒否のエラーが `ErrParseSubtitles` を包む `*ParseError` で、`ErrNoSubtitles` にマッチせず、値の文字列を含まないこと / ガードテストがソースの形を固定し、AC-11・AC-12 を実行される検査で確かめていること / `AsInt64` を削除していないこと

**実装モデル要件**: standard

**判定理由**: 変更が 1 ファイルの読み取りの置き換えと 2 つの非公開シンボルの削除に閉じ、設計判断・panel-mode トリガー・競合する実装案・複数の Conditional check・分離された高リスク／複雑なステップのいずれにも該当しないため。security.md §6 の条件付きガイドには該当する（architecture §5）が、内容は信頼できない値をエラーメッセージから除く変更であり、新しいデータの流れや外部の面を持たない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | フェーズ | 成果物と完了の合図 |
|---|---|---|
| M1 | フェーズ 1 | `tStartMs` が `AsNonNegativeInt64` で読まれ、`-0` が拒否され、エラーが値を含まず、規則の一本化と番兵の集合がガードテストで固定される。グリーンゲート（`make test && make lint`）が通る |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | `internal/transcript/json3.go` の読み取りの置き換えと `decodeTimestamp`・`errNegativeTimestamp` の削除、`json3_test.go` の拒否・受理・エラー・ガードのテスト | standard |

### 3.3. 実装順序の根拠

architecture §8 の実装優先順位はフェーズ 1 だけであり、PR も 1 つである。ステップ 1-1（本番の変更）を先に行い、1-2〜1-4（テスト）をその変更に対して書き、1-5 でゲートを通す。並べ替えの余地は無い。

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

ユニットテストは `yt-dlp` も API も Webhook も呼ばない（`CLAUDE.md`「Testing Strategy」）。確かめる対象は次のとおり。

- 受理の形（AC-01）: `TestParseSubtitlesTimestamps` に加える受理のサブテスト（`0`・正の整数・`int64` の最大値）。
- 拒否の形（AC-02〜AC-06）: 同テストの `cases` に加える `-0`・`1.0`・`1e3`・`true`・`[]`・`{}` の行と、既存の `negative`・`fractional`・`string`・`overflow`・`null tStartMs`・`missing tStartMs` の行、既存の `TestParseSubtitlesDuplicateMembers` の `duplicate tStartMs`。
- 捨てるイベント（AC-07）: `discarded event ignores tStartMs` に加える `-0` のケース（`segs` がない形と、本文が空白だけの形のそれぞれ）。
- エラーの契約（AC-08・AC-10）: `assertSubtitlesRejected` が、追加した拒否の各行で `ErrParseSubtitles`・`*ParseError.Path`・`nil` の `[]Segment`・`ErrNoSubtitles` にマッチしないことを確かめる。
- エラーに値を含まないこと（AC-09）: `TestParseSubtitlesTimestampErrorOmitsValue`。
- 規則の一本化と番兵の集合（AC-11・AC-12）: `TestTimestampReadingIsUnified`・`TestExportedSentinelErrorsUnchanged`。
- 既存の振る舞いの維持（AC-14・AC-15）: `TestParseSubtitlesRealData`（実際のサンプルと `subtitlesOracle` の一致）と、`json3_test.go` の既存の各行・サブテストがそのまま通ること。`-0` の拒否だけが変わり、実際のサンプルに `-0` は無い。
- `AsNonNegativeInt64` の挙動の維持（AC-13）: `internal/strictjson/strictjson_test.go::TestAsNonNegativeInt64` がそのまま通ること。

統合テストは該当なしである。このタスクは外部状態（コンテナ・DB・ネットワーク・キャッシュ）を触らず、確かめるのはパッケージ内の純粋な処理だけである。後方互換については、`-0` 以外の受理・拒否が変わらないことを既存のテスト（AC-02・AC-04〜AC-06・AC-14・AC-15）で確かめる。

### 4.2. 壊し確認

`CLAUDE.md`「Testing Strategy」に従い、実装時に次の壊し確認を行い、その結果をコミットメッセージに記録する。手順は、対象の分岐を本タスクの前の形に戻す → 名指ししたテストだけを実行する → 落ちることを確かめる → 戻して `make test` が通ることを確かめる、の順とする。

- `decodeEvent` の読み取りを `decodeTimestamp`（`AsInt64` の後に負の値を拒否する）に戻すと、`TestParseSubtitlesTimestamps` の `-0` の行（`-0` を `0` として受理する）と `TestTimestampReadingIsUnified`（`AsInt64`・`decodeTimestamp`・負かどうかを調べる式が現れる）が落ちること。
- `decodeTimestamp` に戻すと、`TestParseSubtitlesTimestampErrorOmitsValue` の範囲外（`9223372036854775808`）と非整数（`1.5`・`1e3`）のケースが落ちること（`strconv` のエラーが値を含むため）、および `-0` のケースが落ちること（本タスクの前は受理されて `err` が `nil` になるため）。
- 自前の負の値判定を `<= -1` など別の表記で読み取りの後に戻すと、`TestTimestampReadingIsUnified` が落ちること（表記に依存せず検出する）。
- `errors.go` に exported の番兵を 1 つ足すと、`TestExportedSentinelErrorsUnchanged` が落ちること。

変更しないコード（`AsNonNegativeInt64`・`AsInt64`）を対象とする `internal/strictjson` の既存のテストには、壊し確認を課さない。

### 4.3. テストヘルパー

新しいテストは既存の `internal/transcript/json3_test.go` に加える。`testutil/` のヘルパも `test_helpers*.go` も追加しない（`test_organization.md` の Classification A・B のどちらも使わない）。したがって `TestFakesCarryBuildTag`（`internal/pipeline/pipeline_test.go:593`）の固定するファイル数は変わらない。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 検証の区分 | 検証 |
|---|---|---|
| AC-01 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps`（受理のサブテスト: `0`・正の整数・`int64` の最大値）。`make test` で実行する |
| AC-02 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps`（既存の `negative` の行）。`make test` で実行する |
| AC-03 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps`（追加する `-0` の行）。`make test` で実行する |
| AC-04 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps`（既存の `overflow` の行）。`make test` で実行する |
| AC-05 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps`（追加する `1.0`・`1e3`・`true`・`[]`・`{}` の行と、既存の `fractional`・`string`・`null tStartMs` の行）。`make test` で実行する |
| AC-06 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps`（既存の `missing tStartMs` の行）と `internal/transcript/json3_test.go::TestParseSubtitlesDuplicateMembers`（`duplicate tStartMs`）。`make test` で実行する |
| AC-07 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestamps`（`discarded event ignores tStartMs` のサブテストに加える `-0` のケース。`segs` がない形と本文が空白だけの形のそれぞれ）と `internal/transcript/json3_test.go::TestParseSubtitlesEmptySegs`（既存の `no segs member`・`whitespace text`）。`make test` で実行する |
| AC-08 | test | `assertSubtitlesRejected`（`internal/transcript/json3_test.go:288`）が、AC-02〜AC-06 の各行で `ErrParseSubtitles`・`ErrNoSubtitles` にマッチしないこと・`*ParseError.Path`・部分結果を返さないことを確かめる。`make test` で実行する |
| AC-09 | test | `internal/transcript/json3_test.go::TestParseSubtitlesTimestampErrorOmitsValue`（`-1`・`-0`・`9223372036854775808`・`1.5`・`1e3` のエラー文字列が値を含まないこと）。`make test` で実行する |
| AC-10 | test | `assertSubtitlesRejected` が、AC-02〜AC-06 の各行で `parseSubtitles` の `[]Segment` が `nil` であることを確かめる。`make test` で実行する |
| AC-11 | static | `internal/transcript/json3_test.go::TestExportedSentinelErrorsUnchanged`（パッケージの非テストの Go ファイルの exported の番兵の集合を固定するガードテスト）。`make test` で実行する |
| AC-12 | static | `internal/transcript/json3_test.go::TestTimestampReadingIsUnified`（`json3.go` を解析し、読み取りが `AsNonNegativeInt64` で、`AsInt64`・`decodeTimestamp`・`errNegativeTimestamp` と、`AsNonNegativeInt64` が返した値を保持する識別子に対する自前の負でない整数の判定が無いことを、表記に依存せず固定するガードテスト）。`make test` で実行する |
| AC-13 | test | `internal/strictjson/strictjson_test.go::TestAsNonNegativeInt64`（`AsNonNegativeInt64` の受理・拒否と、エラーが値を含まないこと）。`make test` で実行する |
| AC-14 | test | `internal/transcript/json3_test.go::TestParseSubtitlesRealData`（`testdata/` の json3 と `subtitlesOracle` の一致）。`make test` で実行する |
| AC-15 | test | `internal/transcript/json3_test.go` の既存の拒否・重複・上限・UTF-8 の各テスト（`TestParseSubtitlesRejectsSimple`・`TestParseSubtitlesElementKinds`・`TestParseSubtitlesTimestamps` の既存の行・`TestParseSubtitlesDuplicateMembers`・`TestParseSubtitlesLimits`・`TestParseSubtitlesArrayElementLimit`・`TestParseSubtitlesUTF8`）。`make test` で実行する |
| AC-16 | test | ステップ 1-2・1-3 のテスト（`-0` の拒否、エラーの値の非包含）と、§4.2 の壊し確認（読み取りを `decodeTimestamp` に戻すと `TestParseSubtitlesTimestamps` の `-0` の行と `TestParseSubtitlesTimestampErrorOmitsValue` が落ちること）。`make test` で実行する |

`static` の 2 件（AC-11・AC-12）は、いずれも `make test` が実行するガードテストである。`manual` だけの AC は無い。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| 読み取りを置き換えたつもりで古い `decodeTimestamp` の経路が残る | `-0` が受理され、規則が一本化されない | ステップ 1-1 で `decodeTimestamp`・`errNegativeTimestamp` を削除し、`-0` の拒否の行と `TestTimestampReadingIsUnified` で固定する。§4.2 の壊し確認で落ちることを確かめる |
| ガードテストが特定の表記に依存し、同値の別表記の退行を見逃す | 自前の負の値判定が別の表記で戻ってもガードが通る | `TestTimestampReadingIsUnified` は `json3.go` を解析し、識別子（`AsInt64`・`decodeTimestamp`・`errNegativeTimestamp`）と、`tStartMs` から読んだ値に対する負の値判定（`< 0`・`<= -1` などの同値の形を含む）を、表記に依存せずに調べる。§4.2 の壊し確認で、別表記の再導入でも落ちることを確かめる |
| 実際のサンプル（`testdata/`）に `-0` が含まれ、受理のテストが変わる | AC-14 が落ちる | 実際のサンプルに `-0` は無い（`testdata/2tcCWM-sRBw.ja.json3` の `tStartMs` は `0` または正の整数＝負でない整数）。ステップ 1-5 の `TestParseSubtitlesRealData` で確かめる |
| エラーに値を含まないテストが、`ParseError.Path` の文字列を誤って値として扱う | テストが偽陽性で落ちる | `TestParseSubtitlesTimestampErrorOmitsValue` は値の文字列（`-1` など）だけを照合し、`Path` に渡す名前（`inline.json3`）は値を含まない |
| テストが想定した理由で落ちないままコミットされる | 検証が空振りする | §4.2 の壊し確認を全項目行い、結果をコミットメッセージに記録する |

### 6.2. スケジュールのリスク

本タスクは 1 フェーズ・1 ファイル（本番）に閉じ、外部依存が無いため、スケジュール上のリスクは低い。

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）。フェーズ 1 のステップが完了し、M1 の完了条件を満たす
- [x] §4.2 の壊し確認をすべて行い、結果をコミットメッセージに記録した
- [x] §5 のすべての AC の検証が通り、§5 の表の `test`・`static` の検証が `make test` で実行される
- [x] グリーンゲート（`make test && make lint`）が通る

## 8. 成功基準 (Success Criteria)

- **機能:** 消費するイベントの `tStartMs` が `AsNonNegativeInt64` で読まれ（AC-12）、`0`・正の整数・`int64` の最大値が受理され（AC-01）、`-0`・`-1`・小数・指数・`int64` を超える数・文字列・`null`・真偽値・配列・オブジェクトが拒否される（AC-02〜AC-06）。拒否は `ErrParseSubtitles` を包む `*ParseError` で、`ErrNoSubtitles` にマッチせず、部分結果を返さず（AC-08・AC-10）、値の文字列を含まない（AC-09）。`AsNonNegativeInt64` と既存のテストの振る舞いは変わらない（AC-13〜AC-15）。
- **品質:** `make test`・`make lint` が通る。追加した各テストが §4.2 の壊し確認で落ちることが確認され、結果がコミットメッセージに記録されている。
- **セキュリティ:** 新しい外部モジュール・外部コマンド・ネットワーク送信・キャッシュの読み書きを加えない。拒否のエラーに信頼できない入力（`tStartMs` の値）を含めない（architecture §5）。`tStartMs` の値によって実行する処理を選ばない。
- **文書:** 変更するソースのコメント・識別子・文字列リテラルは英語である。`AC-NN`・`F-NNN` を Go ソースに書かない。`package_reference.md` とほかの文書を編集しない（§1.3、architecture §2）。

## 9. 次のステップ (Next Steps)

- 本書の PR の境界（§3.2）に従い、`/runplan 0023` で PR-1 を実装する。
- 本タスクの完了後、`internal/transcript` の `tStartMs` の読み取りと、LLM の応答の `usage`、中間生成物のトークン数の「負でない整数」が、いずれも `AsNonNegativeInt64` の 1 つの規則になる（architecture §9）。
