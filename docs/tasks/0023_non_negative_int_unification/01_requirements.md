# 要件定義書：字幕の `tStartMs` の読み取りを `strictjson` の負でない整数の読み取りに一本化する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-16 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、字幕（json3）の `events[].tStartMs` を読む処理を、`internal/transcript` 独自の実装（`decodeTimestamp`）から `internal/strictjson` の `Value.AsNonNegativeInt64` に移し、「負でない整数」の規則をコードベースで 1 つにするための要件を定義する。

**背景:**

現在、字幕の `tStartMs` は `internal/transcript/json3.go:148-158` の `decodeTimestamp` が読む。この関数は `strictjson.Value.AsInt64` で数を読んだ後に `startMs < 0` を拒否して、「負でない整数」を独自に実装している。タスク `0019_token_usage`（#247）で `internal/strictjson` に加わった `Value.AsNonNegativeInt64` は、同じ「負でない整数」を実装しているが、次の 2 点で `decodeTimestamp` と挙動が異なる。

| | `decodeTimestamp`（現状） | `AsNonNegativeInt64`（0019 で追加済み） |
|---|---|---|
| `-0` | `0` として受理する | 拒否する |
| エラーのメッセージ | 負の値は `tStartMs is negative`、範囲外と非整数（小数・指数）は `strconv.ParseInt` のメッセージ（入力の値を含む。例: `strconv.ParseInt: parsing "9223372036854775808": value out of range`、`strconv.ParseInt: parsing "1.5": invalid syntax`） | 壊れた規則だけを示し、値の文字列を含めない |

この 2 つの差は、次の問題になる。

- `-0` の受理は、負の符号を黙って捨てる正規化であり、[CLAUDE.md](../../../CLAUDE.md) の「Reject, don't normalize」に反する。
- 字幕のファイルは `yt-dlp` の出力であり信頼できない入力である。その値がエラーのメッセージに入る。標準エラーの行は `redact.Line` でエスケープされ、ファイルの大きさにも上限があるので実害はほぼないが、[security.md](../../dev/security.md) §6 の「拒否のエラーに値を含めない」方針と合わない。
- 同じ「負でない整数」の規則が 2 つあり、挙動が違う。

0019 の設計書（`docs/tasks/0019_token_usage/02_architecture.md` §3.3）は、この 2 つの差を既知の差として記録し、字幕の読み取りの移行を [#259](https://github.com/isseis/yt2column/issues/259) で扱うとしている。本書はその移行の要件を定める。

対応 issue: #259。前提: #247（0019。`internal/strictjson` の `Value.AsNonNegativeInt64` と `Value.IsNull` の追加。`AsNonNegativeInt64` は実装済み）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

- 字幕の `tStartMs` の読み取りを `AsNonNegativeInt64` に移し、「負でない整数」の規則をコードベースで 1 つにする（F-001・F-003）。
- `-0` を拒否し、負の符号を黙って捨てる正規化をなくす（F-001）。
- 拒否のエラーに、信頼できない入力である値の文字列を含めない（F-002）。
- `tStartMs` の受理・拒否と、拒否のエラーの契約（`ErrParseSubtitles`）のうち、本タスクが変えるのは `-0` だけであり、ほかの振る舞いを維持する（F-004）。

### 2.2. スコープ (In Scope)

- 字幕（json3）の消費するイベントの `tStartMs` の読み取り（F-001）。
- その拒否のエラーの契約（`ErrParseSubtitles`・`ErrNoSubtitles`・`*ParseError`、値の非包含。F-002）。
- 上記を確かめるテスト（F-004）。
- 関連する文書（`docs/dev/developer_guide/package_reference.md` の `internal/transcript`・`internal/strictjson` の説明など）の更新の要否は `02_architecture.md` で決める。

### 2.3. スコープ外 (Out of Scope)

- `internal/strictjson.Value.AsNonNegativeInt64` の挙動の変更（0019 で確定済み。本書はその挙動を前提にする）。
- `internal/strictjson.Value.AsInt64` の変更。中間生成物の形式の版（`internal/artifact/format.go:228`）が引き続き使うので残す。
- 中間生成物の形式の版や `usage` の読み取り（0017・0019 の範囲。本書は変えない）。
- 字幕の `tStartMs` 以外の要素（`segs`・`utf8`・`events`・info.json）の受理・拒否の規則。0002 のままとする。
- `Segment.StartMs` を読む側（`internal/writer` など）の変更。`Segment` の形と `StartMs` の型（`int64`）は変えない。
- 字幕の `tStartMs` の、負でない整数以外の意味（動画の時刻としての妥当性、単調性など）の検証。0002 の範囲とする。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: `tStartMs` の読み取りの一本化

字幕（json3）のイベントのうち、`segs` を持ち、`utf8` を連結した本文が空白だけではないもの（以下「消費するイベント」）について、`tStartMs` を `internal/strictjson` の `Value.AsNonNegativeInt64` で読む。`internal/transcript` に「負でない整数」を実装する独自の検査（`AsInt64` の後の `startMs < 0` の拒否）を残さない。

`AsNonNegativeInt64` が受理する形は、符号・小数部・指数部のない 10 進の整数で、`int64` の範囲にあり、負でないものだけである。したがって、`-1` と `-0`、小数（`1.0`・`1.5`）、指数（`1e3`）、`int64` を超える整数（`9223372036854775808`）、文字列、`null`、真偽値、配列、オブジェクトは、どれも拒否する。`0` と正の整数（`int64` の最大値まで）は受理する。

**決定（`-0` の扱い）:** `-0` は拒否する。従来の `decodeTimestamp` は `AsInt64` が `-0` を `0` として読むため、これを受理していた。`-0` は数としては `0` だが、負の符号を持つ表記であり、受理すると符号を黙って捨てる正規化になる（[CLAUDE.md](../../../CLAUDE.md)「Reject, don't normalize」）。`yt-dlp` の出力や本プロジェクトの `Marshal` は `-0` を書かないので、受理する理由もない。この `-0` が、本タスクが変える唯一の受理・拒否の振る舞いである。

**決定（捨てるイベントの `tStartMs`）:** 消費するイベントの `tStartMs` だけを読む。`segs` を持たない、または本文が空白だけのイベントは、`tStartMs` の値にかかわらず検査しない（0002 の「捨てるイベントの `tStartMs` は検査しない」を維持する）。

**Acceptance Criteria**:
- **AC-01**: 消費するイベントの `tStartMs` が `0`・正の整数・`int64` の最大値（`9223372036854775807`）のとき、`parseSubtitles` は成功し、返る `Segment.StartMs` がその値になる。
- **AC-02**: 消費するイベントの `tStartMs` が `-1` のとき、`parseSubtitles` は拒否する。
- **AC-03**: 消費するイベントの `tStartMs` が `-0` のとき、`parseSubtitles` は拒否する。本タスクの前は `0` として受理していた。
- **AC-04**: 消費するイベントの `tStartMs` が `int64` を超える整数（`9223372036854775808`）のとき、`parseSubtitles` は拒否する。
- **AC-05**: 消費するイベントの `tStartMs` が小数（`1.0`・`1.5`）・指数（`1e3`）・文字列（`"0"`）・`null`・真偽値（`true`）・配列（`[]`）・オブジェクト（`{}`）のとき、`parseSubtitles` は拒否する（本タスクの前と同じ）。
- **AC-06**: 消費するイベントに `tStartMs` がないときは、`tStartMs` の不在として拒否する。`tStartMs` が重複しているときは、消費するメンバの重複の規則で拒否する（どちらも本タスクの前と同じ）。
- **AC-07**: 捨てるイベント（`segs` がない、または本文が空白だけ）の `tStartMs` は、`null` や `-0` を含むどの値でも、検査されない（受理される）。

`-0` を含む上記の拒否の入力は、文書全体の検査（§3.2。0002 と `internal/strictjson` のまま）を通った文書に対する `tStartMs` の値である。

#### F-002: 拒否のエラーの契約

`tStartMs` の拒否のエラーの契約を、本タスクの前と同じにする。すなわち、拒否は `ErrParseSubtitles` を包む `*ParseError`（`Path` に呼び出し元が渡したパスを持つ）として返り、`ErrNoSubtitles` にはマッチしない。部分的な結果（`nil` でない `[]Segment`）を返さない。拒否のエラーメッセージは、拒否した値の文字列（`-1`・`-0`・`9223372036854775808` など）を含めない（[security.md](../../dev/security.md) §6「拒否のエラーに値を含めない」）。

**決定（`-1` と `-0` を区別する番兵）:** `-1` と `-0` を区別する、パッケージ外から観測できる新しい番兵エラーは加えない。`tStartMs` の拒否は、`AsNonNegativeInt64` が返す規則ごとのエラー（非公開。負の符号・非整数・範囲外・非数）をそのまま `ErrParseSubtitles` として包む。既存の非公開の番兵 `errNegativeTimestamp`（`internal/transcript/json3.go:26`）は本タスクで削除する。これを観測する呼び出し元もテストもない（`internal/transcript` の外からは見えず、`errors.Is` で確かめているテストは存在しない）。残して `errors.Is(err, errNegativeTimestamp)` で `-1` を確かめても、`-0` も同じ規則（負の符号）で拒否されるため、番兵の意味が曖昧になる。`AsNonNegativeInt64` の拒否が規則ごとの非公開のエラーである以上、`internal/transcript` が独自の番兵でそれらを包み直す理由はない。

**Acceptance Criteria**:
- **AC-08**: AC-02〜AC-06 のすべての拒否について、返るエラーは `errors.Is(err, ErrParseSubtitles)` を満たし、`errors.Is(err, ErrNoSubtitles)` を満たさない。`errors.AsType[*ParseError]` で取り出した `ParseError.Path` は、`parseSubtitles` に渡したパスと等しい。
- **AC-09**: AC-02〜AC-06 のすべての拒否について、返るエラーの文字列は、拒否した値の文字列を含まない。負の値（`-1`・`-0`）、範囲外の値（`9223372036854775808`）、非整数の値（`1.5`・`1e3`）のそれぞれで確かめる。
- **AC-10**: 部分的な結果を返さない。AC-02〜AC-06 のすべての拒否で、`parseSubtitles` が返す `[]Segment` は `nil` である。
- **AC-11**: 拒否のエラーの契約は `ErrParseSubtitles` であり、`-1` と `-0` を区別する新しい、`internal/transcript` の外から観測できる番兵エラーを追加しない（静的な確認。`internal/transcript` の exported の番兵の集合が変わらないこと）。

#### F-003: 規則が 1 つであること

「負でない整数」の規則がコードベースで 1 つ（`internal/strictjson.Value.AsNonNegativeInt64`）であることを確かめる。`internal/transcript` の本番のソースに、`tStartMs` の負の値を拒否する独自の検査（`startMs < 0` に相当する分岐）と、そのための独自の番兵エラーを残さない。

**Acceptance Criteria**:
- **AC-12**: `internal/transcript` の本番のソース（`json3.go`）の `tStartMs` の読み取りが `Value.AsNonNegativeInt64` であり、`AsInt64` の後の `startMs < 0` の拒否を含まない（静的な確認）。`internal/transcript` の本番のソースに、`tStartMs` の値の符号を自前で検査する分岐がない。
- **AC-13**: `internal/strictjson.Value.AsNonNegativeInt64` の挙動を変えない（0019 の `TestAsNonNegativeInt64` が本タスクの後もそのまま通る）。

#### F-004: 既存の振る舞いの維持とテスト

**Acceptance Criteria**:
- **AC-14**: 実際の字幕のサンプル（`testdata/` の json3）の解析結果（`Segment` の列）は、本タスクの前と同じである。`tStartMs` が負でない整数である実際のサンプルでは、挙動は変わらない。
- **AC-15**: 字幕の既存のテスト（`internal/transcript/json3_test.go` の受理・拒否・重複・上限・UTF-8 の各テスト）は、本タスクの後もそのまま通る（既存のテストに、消費するイベントの `tStartMs` が `-0` のものはない）。
- **AC-16**: `-0` の拒否（AC-03）と、拒否のエラーメッセージが負の値・範囲外の値・非整数の値の文字列を含まないこと（AC-09）を確かめるテストを加える。`-0` の拒否と、範囲外・非整数の値を含まないことのテストは、対象のコードを本タスクの前の `decodeTimestamp` に戻すと失敗することを確かめる（[CLAUDE.md](../../../CLAUDE.md)「Every test must be able to fail for its stated reason」）。`-1` の値を含まないことのテストは、本タスクの前のコードでも通る（従来のメッセージ `tStartMs is negative` は値を含まない）ので、退行を防ぐ検査として置き、本タスクの前に戻して失敗することは求めない。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

字幕（json3）のファイルは `yt-dlp` の出力であり、信頼できない入力として扱う。本タスクが変えるのは、そのうち消費するイベントの `tStartMs` 1 つの読み取りだけである。文書全体の検査（生バイトの UTF-8、1 つの最上位の値と末尾に続くデータの拒否、宣言していないメンバの扱い）と、`events`・`segs`・`utf8` の境界は、0002 と `internal/strictjson` のままとする。

| 境界 | 消費する要素 | 受理する形 | 上限 | 拒否時の扱い | 主な AC |
|---|---|---|---|---|---|
| 字幕 json3 の消費するイベントの `tStartMs` | 消費するイベントの `tStartMs` だけ | 符号・小数部・指数部のない 10 進の整数で、`int64` の範囲にあり、負でないこと（`AsNonNegativeInt64` の形）。1 回だけ現れる | 1 値（ファイル全体の上限は `maxSubtitlesBytes`、イベント数の上限は `maxSubtitleEvents` のまま） | `ErrParseSubtitles`（`*ParseError`）。部分的な結果を返さない。`ErrNoSubtitles` にマッチしない。値の文字列を含めない | AC-01〜AC-11 |

-   標準ライブラリが黙って修復・正規化すれば受理できてしまう入力（`-0` の符号の除去がその例）は、受理する形を 1 つに定めたうえで、それ以外を受理しない。本タスクは、残っていた `-0` の正規化を除く。
-   `tStartMs` の値によって実行する処理を選ばない。値は `Segment.StartMs` として保持するだけで、解析の振る舞いを変えない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   LLM の呼び出しと `yt-dlp` の呼び出しの回数を変えない。読み取りのコストは、本タスクの前後で無視できる（[CLAUDE.md](../../../CLAUDE.md)「Performance」）。

### 4.2. セキュリティ (Security)

-   拒否のエラーメッセージに、信頼できない入力である `tStartMs` の値の文字列を含めない（AC-09。[security.md](../../dev/security.md) §6）。本タスクは、負の値については従来から値を含んでいなかったが、範囲外の値と非整数の値（小数・指数。どちらも `strconv.ParseInt` のメッセージ）については値を含んでいた。この点を改善する。
-   標準エラーの行が `redact.Line` を通ることは、0017・0019 のままとする（本タスクで変えない）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   受理・拒否の変更は `-0` だけであり、正しい json3（`yt-dlp` の出力、`testdata/` の実データ）の解析を失敗させない（AC-14）。

### 4.4. 互換性 (Compatibility)

-   `Segment` の形と `StartMs` の型（`int64`）、`parseSubtitles` のシグネチャ、`ErrParseSubtitles`・`ErrNoSubtitles`・`*ParseError` の契約を変えない。
-   `internal/transcript` の `decodeTimestamp`（非公開）と `errNegativeTimestamp`（非公開）を削除する。`internal/transcript` の外から観測できる型・関数・番兵の集合を変えない。
-   `internal/pipeline` の guard（`TestCommonTypesFieldSets`）が固定する型のフィールド集合を変えない。
-   `internal/strictjson.Value.AsInt64` を削除しない（`internal/artifact/format.go` の中間生成物の形式の版が使う）。

### 4.5. 保守性 (Maintainability)

-   「負でない整数」の規則を 1 か所（`AsNonNegativeInt64`）に置き、字幕・LLM の応答の `usage`・中間生成物のトークン数が同じ規則を共有する（[CLAUDE.md](../../../CLAUDE.md)「DRY」）。
-   字幕の `tStartMs` の読み取りに、値ごとの分岐や正規化を残さない。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」と [CLAUDE.md](../../../CLAUDE.md) に従う。外部ライブラリを追加しない。
-   タスク `0019_token_usage` で実装された `internal/strictjson.Value.AsNonNegativeInt64` を前提とする。`AsNonNegativeInt64` が存在しない場合は実装を始めない（0019 の完了後であること）。
-   `internal/strictjson` の `AsNonNegativeInt64` と `IsNull` の挙動、`AsInt64` の挙動を変えない。
-   タスク `0002_ytdlp_transcript_source` の json3 パーサの契約（`events`・`segs`・`utf8`・`tStartMs` の受理・拒否、`ErrParseSubtitles`・`ErrNoSubtitles`・`*ParseError`）のうち、本タスクが変えるのは `-0` の受理だけであり、それ以外を変えない。
-   タスク `0017_intermediate_artifacts`・`0019_token_usage` の承認済みの文書が定める中間生成物の読み取りを変えない。
-   ユニットテストは、実際の `yt-dlp` も LLM の API も Webhook も呼ばない（[CLAUDE.md](../../../CLAUDE.md)「Testing Strategy」）。

## 6. 用語集 (Glossary)

-   **`tStartMs`:** json3 のイベント（`events[]`）が持つ、字幕の表示開始時刻（ミリ秒）。`Segment.StartMs` として保持する（0002 の用語と同じ）。
-   **消費するイベント:** `segs` を持ち、`utf8` を連結した本文が空白だけではないイベント。`tStartMs` を読む対象である。それ以外は捨てるイベントで、`tStartMs` を検査しない。
-   **`AsNonNegativeInt64`:** `internal/strictjson.Value` のメソッド。負でない整数で `int64` の範囲にある数を受理し、それ以外を、規則ごとの非公開の静的エラーで拒否する。エラーは入力の値を含まない（0019 の用語と同じ）。
-   **`AsInt64`:** `internal/strictjson.Value` のメソッド。整数を受理するが、負の値も受理し、範囲外・非整数のエラーは `strconv` のメッセージ（値を含む）を返す（0019 の用語と同じ）。
-   **番兵エラー:** `errors.Is` で種類を判定できる、パッケージが公開する静的なエラー（`ErrParseSubtitles`・`ErrNoSubtitles`）。
-   **`ErrParseSubtitles`・`ErrNoSubtitles`・`*ParseError`:** `internal/transcript` の字幕の解析のエラーの契約（0002 の用語と同じ）。
-   **字幕・json3:** 0002 の用語と同じ。
-   用語は [translation_glossary.md](../../translation_glossary.md) と統一する。
