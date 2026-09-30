# アーキテクチャ設計書：yt-dlp による字幕取得

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-09-30 |
| Review date | - |
| Reviewer | - |
| Comments | 2026-09-30: 要件 §5 の事前調査のうち、字幕なし動画での `yt-dlp` の終了コードと出力を §1.4 に追記した（終了コード 0・info.json のみ出力。既存の設計を変える結果ではない）。 |

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

1. **信頼できない入力は境界で検証し、補正しない。** URL・json3・info.json・外部コマンドの出力は信頼できない入力として扱い（[security.md](../../dev/security.md)）、要件定義書 3.2 が定める受理の形式に合致しない入力は、正規化・切り詰め・置換をせず、対応する番兵エラー（`errors.Is` で判別できる固定のエラー値）で拒否する。部分的な結果を返さない。
2. **外部コマンドは薄い差し替え可能な境界の背後に置く。** `YtDlpSource` は `yt-dlp` の起動を内部 interface（`commandExecutor`）越しに行い、ユニットテストは実 `yt-dlp` を呼ばずに引数・環境変数・終了状態を検証できる（AC-07・AC-27・AC-44）。
3. **キャッシュの変更は成功時だけ、ポインタの置き換え 1 回で行う。** `yt-dlp` には有効な世代へ直接書かせず、有効でない側のスロットへ出力させる。`Transcript` を組み立てられた場合に限り、有効なスロットを示すポインタを atomic に置き換える。ロールバックを持たず、中断で残ったエントリは dangling なエントリとして後で削除する（AC-34・AC-58・AC-59・AC-69・AC-70、design_handoff H-07）。
4. **キャッシュはファイルだけとする。** プロセス内のメモリに保持しない。これにより利用者がキャッシュファイルを削除すると、次の `Fetch` がキャッシュミスとして再取得する（AC-35）。
5. **「空はエラー」を守る。** セグメント 0 件の結果や info.json の欠落を成功として扱わない（AC-23・AC-60）。判断は §3.7 で固定した決定的な検証順序で行う。
6. **既存の interface とデータ型に従う。** 実装は `0001_pipeline_skeleton` で定義済みの `transcript.TranscriptSource`・`Transcript`・`Segment` を変更せずに、要件を満たす（AC-25）。パイプライン（`internal/pipeline`）は本タスクの変更を受けない。

### 1.2. 概念モデル

```mermaid
flowchart TB
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;

    URL[("入力 URL")]

    subgraph SRC["YtDlpSource（オーケストレーション）"]
        direction TB
        VID["URL 検証<br>（動画 ID の抽出・正規化）"]
        READ["キャッシュ読み込み<br>（ペアのヒット判定）"]
        PARSE["ペアの検証<br>（字幕 → info.json の順）"]
        ASM["Transcript の組み立て"]

        subgraph MISS["キャッシュミス / 強制再取得"]
            direction LR
            RUN["コマンド実行<br>（正規化 URL・allowlist 環境）"]
            YTDLP["yt-dlp<br>（外部コマンド）"]
            PAIR[("字幕 json3 と info.json<br>（ペア）")]
            RUN --> YTDLP
            YTDLP -->|"書き込み先スロットへ出力"| PAIR
        end
    end

    subgraph STORE["キャッシュディレクトリ（保存データ）"]
        direction LR
        WRITE["キャッシュ書き込み<br>（ポインタの切り替え）"]
        CF[("スロットとポインタ<br>（1 スロットが 1 世代）")]
        WRITE -->|"新しい世代"| CF
    end

    TC[("Transcript")]
    CALLER["呼び出し元<br>（パイプラインの次段階）"]

    URL --> VID
    VID -->|"動画 ID（キャッシュのキー）"| READ
    CF -->|"キャッシュしたペア"| READ
    READ -->|"ヒットしたペア"| PARSE
    READ -->|"ミス / 強制再取得"| RUN
    PAIR --> PARSE
    PARSE -->|"セグメントとメタ情報"| ASM
    ASM -->|"ミスの成功時だけコミット"| WRITE
    ASM --> TC
    TC -->|"戻り値"| CALLER

    class URL,PAIR,CF,TC data
    class VID,READ,PARSE,ASM,WRITE,RUN enhanced
    class YTDLP,CALLER process
```

**図1 概念モデル**。実線の矢印 A → B は「A の結果を B が入力として使う、または A の成功を受けて B を更新する」を表す。`YtDlpSource` は段階の入口と進行の制御を担うオーケストレータであり、URL 検証・キャッシュの読み込み・キャッシュの書き込み（ポインタの切り替え）・コマンド実行・ペアの検証・`Transcript` の組み立てを独立した責務として実装する（§3.3）。URL 検証で得た動画 ID はキャッシュ読み込みのキーに、正規化 URL はコマンド実行の引数に使う。**コマンド実行へ進むのは、キャッシュ読み込みがミスまたは強制再取得と判定した場合だけである**（図では `READ --> RUN` の 1 経路）。字幕 json3 と info.json は常にペアで処理し、キャッシュも 1 つのスロットの 2 ファイルを 1 世代として扱う。ヒットした場合はキャッシュしたペアを、ミスした場合は「キャッシュミス / 強制再取得」の処理（`yt-dlp` の起動と書き込み先スロットへの出力）で得たペアを、同じ「ペアの検証」に渡す。どちらの経路でも `Transcript` を組み立てて呼び出し元へ返す（`ASM --> TC --> CALLER`）。キャッシュを書き換えるのは、ミスの経路で `Transcript` を組み立てられた場合だけである（`ASM --> WRITE`）。dangling なエントリの削除（§3.5）は図を簡潔に保つため省略した。形式ごとの解析（json3 パーサ・info.json パーサ）は「ペアの検証」の内部にあり、詳細は §3.4 に示す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;

    D[("データ")]
    E["本タスクで追加・変更するコンポーネント"]
    P["既存または外部のコンポーネント"]

    class D data
    class E enhanced
    class P process
```

**凡例（図1 概念モデル）**。

以降の図では、色分け（`classDef`）を使う図にだけ凡例を付ける。シーケンス図（図3）・クラス図（図4）・状態図（図8）は色分けを使わないため凡例を省略し、矢印の意味をキャプションに記す。

### 1.3. 既存コードとの関係

本タスクは、既存パッケージ `internal/transcript` に実装を追加する。既存の `transcript.TranscriptSource` interface（`internal/transcript/transcript.go:26`）と `Transcript` 型（同 `:14`）・`Segment` 型（同 `:8`）は変更しない（執筆時点の HEAD `421251e` で確認）。`internal/pipeline` は `transcript.TranscriptSource` をフィールドに持つだけで（`internal/pipeline/pipeline.go:69`）、本タスクの実装の詳細に依存しないため、変更しない。

`internal/transcript` は現在 `transcript.go` とテスト用 fake（`testutil/mocks.go`・`testutil/mocks_test.go`、いずれも `//go:build test`）だけを持つ。本タスクで `internal/transcript` が `internal/secret` など他パッケージを import することはない（標準ライブラリのみ）。既存テストのうち、本設計が振る舞いを変えるものはない（fake のテストはそのまま通る）。

事前調査（§1.4）のための実 `yt-dlp` の実行は、リポジトリの外の一時ディレクトリで行った。得られた実出力は `testdata/` に保存した。配置は、[project_overview.md](../../dev/project_overview.md) の想定ディレクトリ構成（リポジトリ直下の `testdata/`）と、pre-commit の `check-added-large-files` が `^testdata/` を除外していることに合わせ、リポジトリ直下の `testdata/` とする（パッケージ内には置かない）。

### 1.4. 事前調査の結果（実データ）

要件定義書 §5 の指示により、`02_architecture.md` の作成時（承認前）に実 `yt-dlp` で出力を確認した。`yt-dlp` のバージョンは `2026.08.19` である。

**保存する実データの条件。** 字幕・タイトル・概要欄は著作物でありうる。通常の YouTube 標準ライセンスの動画の全文字起こしをリポジトリに保存しない。リポジトリに保存するのは、再配布が許諾された Creative Commons Attribution 3.0（CC BY 3.0。YouTube の動画ページの表示は "Creative Commons Attribution license (reuse allowed)" で、これは CC BY 3.0 を指す）の動画からの出力に限り、出典（動画 URL・チャンネル名・ライセンス）を [`testdata/README.md`](../../../testdata/README.md) に明記する。

**保存したフィクスチャ。** CC BY の動画 `2tcCWM-sRBw`（チャンネル「日本語で☆話しまSHOW」、日本語スピーチコンテスト）から、自動字幕の実出力を保存した。取得コマンドは `yt-dlp --ignore-config --no-plugin-dirs --skip-download --write-auto-subs --sub-langs ja --sub-format json3 --write-info-json -P <dir> -o "%(id)s" -- <URL>` である。

- `testdata/2tcCWM-sRBw.ja.json3`（338,040 バイト）: 自動字幕。トップレベルに `events` のほか `pens`・`wireMagic`・`wpWinPositions`・`wsWinStyles` があった。`events` は 704 件で、内訳は次の 3 種類だった。
  - 本文を持つ内容イベント 352 件。すべて `tStartMs`（整数）と `segs` を持ち、`segs` の要素は `utf8` のほか未知メンバーを持っていた。
  - `aAppend` を持ち、連結した本文が空白だけのイベント 351 件。
  - `segs` を持たないメタ情報イベント 1 件。
- `testdata/2tcCWM-sRBw.info.json`（1,457,534 バイト。取得者の IP アドレスを含む URL 文字列を置き換えた後の大きさ。`testdata/README.md`）: 63 個のメンバーを持ち、`id`・`title`・`channel`・`description` はいずれも JSON 文字列だった（`id` は動画 ID と一致）。`license` メンバーが CC BY を示す。
- **重複:** 内容イベントの本文にローリング表示に由来する重複は無かった。包含関係にある組は 3 件あったが、いずれも時間的に離れた繰り返しだった。

**手動字幕と自動字幕の優先。** 同じ CC BY 動画で `--write-subs --write-auto-subs --sub-langs ja` を実行すると、`ja.json3` には手動字幕が書かれた（手動が優先される）。ただし、この動画の手動トラックは空白だけのイベントが大半を占める位置調整用とみられるトラックで、本文は 2 行しか無く、実用的な文字起こしは自動字幕にあった。したがって本設計は、読み込むファイルを `<id>.ja.json3` の 1 つに固定し、本文を持つイベントが 0 件なら `ErrNoSubtitles` とする。手動トラックが実質空の場合に自動字幕へ切り替える仕組みは、今回のスコープでは設けない（既知の制限。§9・付録A）。

**レート制限。** 調査の後半で YouTube の字幕取得が HTTP 429（Too Many Requests）を返すようになった。保存したフィクスチャは 429 の前に取得済みである。

**出力ファイル名と `-P`。** 出力は `<動画 ID>.ja.json3` と `<動画 ID>.info.json` で、`-o` の `%(id)s` が動画 ID に展開された。あわせて `-P` に `%(title)s` と `%%` を含むディレクトリ名を指定して実行し、`yt-dlp` がその名前を文字どおりのディレクトリとして扱い（展開せず）、出力をその下にだけ作ることを確認した（AC-06、design_handoff H-02）。

**字幕なし動画での終了コードと出力。** 字幕の無い動画 `hlbh4P0Mz8M` に対し、§3.7 の本番の起動引数（`--write-subs --write-auto-subs --sub-langs ja --sub-format json3 --write-info-json -P <dir> -o "%(id)s"`）で `yt-dlp` を実行した（2026-09-30）。結果は次のとおりである。

- 終了コードは 0 だった。
- 出力は `<動画 ID>.info.json` だけで、字幕ファイルは作られなかった。info.json の `subtitles` に `ja` は無く（`live_chat` のみ）、`automatic_captions` は空だった。
- 字幕が無いことは標準出力の `[info] There are no subtitles for the requested languages` で通知され、エラーにはならなかった。標準エラー出力は、字幕と無関係な JavaScript ランタイム不在の警告だけだった。

したがって、字幕なし動画は `yt-dlp` の正常終了後の検証に達し、§6.2 の順序で字幕ファイルの不在として `ErrNoSubtitles` になる（`ErrYtDlpExec` ではない。AC-21・AC-61）。字幕なしの判定は終了コードや出力の文言に依存せず、字幕ファイルの有無だけで行う。この動画は観測時にライブ配信中（`live_status` が `is_live`）だった。字幕を要求した言語で得られない場合の通知は、配信形態によらず同じ `[info]` の経路とみられるが、通常の動画（VOD）では観測していない。この出力は CC BY ではないため、**保存する実データの条件**に従い `testdata/` には保存しない。字幕なしの経路のユニットテストは、fake executor が正常終了して info.json だけを書く状況で構成する（§7）。

**統合テストの既定の動画。** 手動トラックが位置調整用でない CC BY の動画として、`EQCUZyB4DqE`（太田市日本語スピーチコンテスト）を使う。`info.json` で CC BY と `ja` 自動字幕の存在を確認済みである（手動字幕は無い）。字幕の実取得は 429 のため本調査では完了しておらず、最初の手動実行（F-008・AC-33）で行う。

この観測に基づき、§3.7 で次の 7 項目を固定する。起動引数、字幕ファイル名と採用規則、重複の扱い、サイズ・件数の上限、環境変数の allowlist、タイムアウト後の猶予、統合テストの既定の動画。

---

## 2. システム構成 (System Structure)

### 2.1. パッケージ構成

```mermaid
graph TB
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;

    subgraph pkg_transcript["internal/transcript（既存。拡張する）"]
        TR["transcript.go<br>TranscriptSource / Transcript<br>（変更しない）"]
        TU["testutil/mocks.go<br>fake（既存・変更しない）"]
        YS["ytdlp.go<br>YtDlpSource / Options"]
        EX["exec.go<br>commandExecutor / cappedWriter"]
        VI["video_id.go<br>URL 検証"]
        J3["json3.go<br>json3 パーサ"]
        IF["info.go<br>info.json パーサ"]
        CA["cache.go<br>キャッシュ読み書き"]
        ER["errors.go<br>番兵エラー / ParseError"]
    end

    subgraph pkg_pipeline["internal/pipeline（既存。変更しない）"]
        PI["pipeline.go<br>Pipeline"]
    end

    YTDLP["yt-dlp<br>（外部コマンド）"]
    TD[("testdata/<br>実出力サンプル")]
    CACHE[("キャッシュディレクトリ")]

    PI --> TR
    YS --> EX
    YS --> VI
    YS --> J3
    YS --> IF
    YS --> CA
    YS --> ER
    EX --> YTDLP
    CA --> CACHE
    J3 -.->|"テストのみ"| TD
    IF -.->|"テストのみ"| TD

    class TR,TU,PI process
    class YS,EX,VI,J3,IF,CA,ER enhanced
    class YTDLP process
    class TD,CACHE data
```

**図2 パッケージ依存構造**。矢印 A → B は「A が B を利用（import または起動・読み書き）する」を表す。変更しない `transcript.go`（既存の interface とデータ型）と `pipeline.go` はオレンジ、本タスクで追加するファイルは緑、データは青である。`internal/transcript` 内の追加ファイルは同じパッケージの型を共有し、相互に import しない。`testdata/` への点線の矢印はテストだけが読むことを表し、本番コードは `testdata/` に依存しない。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;

    D[("データ")]
    E["追加・変更するファイル"]
    P["既存または外部のコンポーネント"]

    class D data
    class E enhanced
    class P process
```

**凡例（図2 パッケージ依存構造）**。

### 2.2. コンポーネント配置

- `internal/transcript` に `YtDlpSource` とその部品（URL 検証・パーサ・キャッシュ・コマンド実行）を置く。`internal/transcript` はこれまでどおりリーフであり、標準ライブラリ以外を import しない（`.golangci.yml` の `deps` ルール）。パッケージを分けない理由は、機能が単一の段階（字幕取得）に閉じており、分けると interface と実装の往復 import か、公開範囲の不必要な拡大を招くためである（YAGNI）。
- `cmd/yt2column/main.go` は変更しない（CLI 配線は #6）。`internal/pipeline` も変更しない。

### 2.3. データフロー（成功時・キャッシュミス）

```mermaid
sequenceDiagram
    participant C as 呼び出し元
    participant Y as YtDlpSource
    participant E as commandExecutor
    participant X as yt-dlp
    participant FS as キャッシュ領域

    C->>Y: Fetch(ctx, videoURL)
    Y->>Y: URL 検証 → 動画 ID・正規化 URL
    Y->>FS: キャッシュ確認（読み込み）
    Note over Y,FS: キャッシュミス（または強制再取得）
    Y->>FS: 書き込み先スロットを空で作成
    Y->>E: Run(ctx, name, args, env, stderr)
    E->>X: プロセス起動（シェルを経由しない）
    X-->>FS: 動画ID.ja.json3 / 動画ID.info.json を出力
    E-->>Y: 終了（成功）
    Y->>FS: 出力を検証して Transcript を組み立て
    Y->>FS: ポインタを置き換え（コミット）
    Y->>FS: dangling なエントリ（旧スロット）を削除
    Y-->>C: Transcript
```

**図3 データフロー（キャッシュミス時の成功経路）**。矢印 `->>` は呼び出し、`-->>` は戻り値または書き込みを表す。キャッシュヒット時は `E`/`X` の呼び出しを行わず、キャッシュの読み込みと検証だけで `Transcript` を返す（§6.1）。

---

## 3. コンポーネント設計 (Component Design)

### 3.1. 公開データ型とエラー

既存の型（`Segment`・`Transcript`・`TranscriptSource`、`internal/transcript/transcript.go:8-28`）は変更しない。追加する公開型は次のとおり。

```go
// internal/transcript

// Options configures a YtDlpSource. CacheDir and Timeout are required.
type Options struct {
    CacheDir     string
    YtDlpPath    string // empty means "yt-dlp" on PATH
    Timeout      time.Duration
    ForceRefresh bool
}

// YtDlpSource implements TranscriptSource by invoking yt-dlp.
type YtDlpSource struct { /* unexported fields */ }

// RemoveCache removes every cache entry of the video, valid or dangling.
// The caller must not run Fetch for the same video concurrently.
func (s *YtDlpSource) RemoveCache(ctx context.Context, videoURL string) error

// PruneCache removes dangling cache entries of every video in CacheDir.
// The caller must not run Fetch or another PruneCache on the same CacheDir concurrently.
func (s *YtDlpSource) PruneCache(ctx context.Context) error

// ParseError keeps the path of the file that failed to parse.
// Unwrap returns ErrParseSubtitles or ErrParseInfo.
type ParseError struct {
    Path string
    Err  error
}
func (e *ParseError) Error() string
func (e *ParseError) Unwrap() error
```

番兵エラーはパッケージレベルの値として定義する。番兵は判別のための値であり、`Fetch` は番兵そのものを返さず、常に文脈を付けて `errors.Is` で判別できる形でラップして返す（§4.1）。

```go
var (
    ErrInvalidVideoURL = errors.New("invalid video URL")
    ErrYtDlpExec       = errors.New("yt-dlp execution failed")
    ErrParseSubtitles  = errors.New("parse subtitles")
    ErrParseInfo       = errors.New("parse video info")
    ErrNoSubtitles     = errors.New("no subtitles")
)
```

### 3.2. interface

- `transcript.TranscriptSource`（既存）を実装する。`Fetch(ctx context.Context, videoURL string) (Transcript, error)`（`internal/transcript/transcript.go:27`）。
- 外部コマンドの実行は、パッケージ内の次の interface 越しに行う。テストはこれを差し替える。

```go
// internal/transcript

// commandExecutor runs an external command. Tests replace this.
type commandExecutor interface {
    Run(ctx context.Context, name string, args, env []string, stderr io.Writer) error
}
```

`Run` は生の実行結果を返す（`*exec.ExitError`・`*exec.Error`・`exec.ErrWaitDelay` など）。タイムアウト・キャンセルで子プロセスを終了させた場合も、`exec.Cmd.Wait` は context のエラーではなく `*exec.ExitError`（`signal: killed`）を返す。したがって `Fetch` は `Run` の戻り値の型でタイムアウト・キャンセルを判別せず、`Run` が失敗したら先に `yt-dlp` の実行に使った（タイムアウトを設定した）`ctx` の `ctx.Err()` を確認し、非 nil ならそれ（`context.DeadlineExceeded` または `context.Canceled`）を返す。`ctx.Err()` が nil の場合だけ、起動・待機の失敗および非ゼロ終了を、`ErrYtDlpExec` をラップしたエラー（§4.1）に変換する。テスト用の fake はこの契約に従い、タイムアウト・キャンセルを模擬するときは実装と同じく context のエラーではない失敗（非ゼロ終了相当）を返す。

標準エラー出力の上限（4 KiB）とドレイン（上限を超えた分も読み続けて捨てること）は `cappedWriter` が担い、`Fetch` がこれを生成して `Run` に渡す。実装（`exec.go`）は受け取った `io.Writer` を子プロセスの標準エラー出力に接続するだけにする。この分担により、上限の切り詰めは fake を使う `Fetch` レベルのテストで検証でき、子プロセスのドレイン（読み取りを止めないこと）は実プロセスを使う `exec_test.go` で検証できる（§7.1）。

`YtDlpSource` のフィールドは非公開とし、構築は `NewYtDlpSource(opts Options) (*YtDlpSource, error)` だけを入口にする。構築時に `CacheDir` が空、または `Timeout` が 0 以下ならエラーを返す（AC-28、要件 §5 の「キャッシュディレクトリの指定がない場合の扱いは設計で決める」に対する決定）。実行パスが空の場合は `"yt-dlp"` を使い、実行時に見つからなければ `ErrYtDlpExec` になる。

### 3.3. `YtDlpSource` と内部の責務

`YtDlpSource` は段階の入口（`TranscriptSource` の実装）と進行の制御だけを担う。処理の実体は、次の独立した責務に分ける（図1）。いずれも同一パッケージ内の関数または小さな型で実装し、外部から差し替える必要があるのは外部コマンドの実行（`commandExecutor`、§3.2）だけである。

- **URL 検証**（`video_id.go`）: 入力 URL から動画 ID と正規化 URL を返す純粋な処理（§3.6）。
- **キャッシュの読み込み**と**キャッシュの書き込み**（`cache.go`）: 読み込みはポインタによるヒット判定とペアの読み出し、書き込みは書き込み先スロットの準備と成功時のポインタの置き換え、あわせて dangling なエントリの判定と削除（`Fetch` 時・キャッシュの削除・掃除で共通。§3.5）。
- **コマンド実行**（`exec.go`）: `yt-dlp` の起動、allowlist 環境、標準エラー出力の上限付きドレイン（§5.2）。
- **ペアの検証**（`json3.go`・`info.go`）: 字幕ファイルと info.json をペアとして、字幕 → info.json の決定的な順序で検証し、セグメントとメタ情報を返す純粋な処理（§3.4）。形式ごとの解析は json3 パーサ（`json3.go`）と info.json パーサ（`info.go`）が担い、ペアの順序と番兵の優先は `Fetch` が制御する（§6.2）。
- **`Transcript` の組み立て**: 動画 ID・正規化 URL・メタ情報・セグメントをまとめる。

`Fetch` の処理順は §6.1 に示す。`Fetch` は `ctx` のキャンセルに従い、開始前に `ctx` が終了済みなら `yt-dlp` を起動せず `ctx.Err()` を返す（キャンセル済みなら `context.Canceled`（AC-26）、期限切れなら `context.DeadlineExceeded`）。

### 3.4. ペアの検証とパーサ

字幕 json3 と info.json は常にペアで処理する。キャッシュの完全性も 2 ファイルの存在で判定し（AC-29）、検証は字幕 → info.json の決定的な順序で行う（§6.2）。形式ごとの解析は json3 パーサと info.json パーサに分けるが、呼び出し側から見た責務は「ペアの検証」の 1 つであり、順序と番兵の優先は `Fetch` が制御する。

json3 と info.json のパーサは、外部コマンドの起動から分離した純粋な処理とし、`testdata/` の実出力でテストできる形にする（AC-13・AC-27、要件 §4.5）。両者は次の共通手段で「標準ライブラリが黙って補正する入力」を拒否する（design_handoff H-05・H-12）。

- デコードの前に生のバイト列を検証し、不正な UTF-8、および対になっていない UTF-16 サロゲートのエスケープを拒否する。
- トップレベルがちょうど 1 つの JSON 値であり、その後に空白以外のデータが残らないことを検証する。
- `null`、配列要素の `null`、型の不一致（例: `utf8` が `null`）、`int64` の範囲を超える数値を、値ごとに区別して拒否する。欠落（ゼロ値）と `0`・空文字列を区別できる中間表現を使って検証する。
- 消費するメンバーが同じオブジェクトの中で重複する入力（例: `{"id":"wrong","id":"<動画 ID>"}`）を拒否する。標準ライブラリのデコーダは後の値を黙って採るため、これも黙って補正される入力にあたる。対象は、json3 のトップレベル（`events`）・`events` の各要素（`tStartMs`・`segs`）・`segs` の各要素（`utf8`）と、info.json のトップレベル（`id`・`title`・`channel`・`description`）である。重複する値が同じか異なるかを問わず拒否し、番兵は形式ごとに `ErrParseSubtitles`・`ErrParseInfo` とする。検出の手段は [implementation_handoff.md](implementation_handoff.md) I-03 に記録する。
- 上記は「消費するフィールド」に限定する。消費しない未知のメンバーは、重複していても無視して受理する（AC-64・AC-67。消費するフィールドは F-003 と F-004 が定める）。`aAppend` などの未知メンバーの値によって挙動を変えない（要件 3.2 の「消費するフィールドだけを厳密に検証する」に従う）。
- 本文を持つイベントとは、`segs` を持ち、その `utf8` を連結した文字列に空白以外の文字を含むイベントをいう。`segs` を持たないイベントと、連結した本文が空白文字だけのイベント（行区切り）は、本文を持たないものとしてセグメントに含めない。この判定は `aAppend` の有無に依存しない。
- ファイルサイズと件数の上限（§3.7）を読み込みの時点で判定する（design_handoff H-09）。

失敗は、対象ファイルのパスを保持する `*ParseError`（`Err` は `ErrParseSubtitles` または `ErrParseInfo`）として返す。キャッシュ読み込み中の失敗はこの型でパスを保持し、利用者が破損ファイルを特定できる（AC-36）。

### 3.5. キャッシュ

キャッシュは動画 ID ごとに、2 つの**スロット**（世代を入れるディレクトリ）と 1 つの**ポインタ**（どちらのスロットが有効かを示すファイル）で構成する。1 つのスロットには 1 回の `yt-dlp` の出力（字幕ファイルと info.json。§3.7）だけが入り、有効な世代はポインタが指すスロットである。キャッシュの置き換え（コミット）はポインタの atomic な置き換え 1 回だけで行う。そのため、既存の世代を退避する手順も、失敗時に元へ戻すロールバックも持たない（付録A）。

**エントリの固定名。** 本実装が作成・変更・削除するのは、検証済みの動画 ID から組み立てた次の固定名のエントリだけである（AC-20・AC-59）。

| 種別 | 名前 | 期待する種別 |
|---|---|---|
| スロット | `<id>.a`・`<id>.b`（中身は `yt-dlp` が出力する `<id>.ja.json3`・`<id>.info.json`。§3.7） | ディレクトリ（`0o700`） |
| ポインタ | `<id>.current`（内容は `a` または `b` の 1 バイトだけ） | 通常ファイル（`0o600`） |
| ポインタの一時ファイル | `<id>.current.tmp` | 通常ファイル（`0o600`） |
| スロット内の出力 | 各スロットの中の `<id>.ja.json3`・`<id>.info.json`（§3.7） | 通常ファイル（永続化の後は `0o600`） |

接頭辞や任意のサフィックスでエントリを識別しない。したがって、同じ動画 ID で始まる名前であっても、上表にないエントリ（利用者が置いた `<id>.notes`、`<id>.ja-orig.json3` など）には触れない。上表の名前でも、期待する種別と異なるエントリ（シンボリックリンクを含む）には触れない（辿らない）。ただし、スロットを削除するときは、中のエントリを種別によらず、辿らずにスロットごと削除する。

スロット内の出力にこの規則を適用した結果は次のとおりである。字幕ファイルまたは info.json が通常ファイルでない場合（`yt-dlp` が出力したものか、既存のキャッシュに置かれたものかを問わない）、それを辿らず、読まず、パーミッションを変更せず、同期もしない。存在の判定では存在するものとして数え、§6.2 の「読み取り不能」として扱う（字幕ファイルなら `ErrParseSubtitles`、info.json なら `ErrParseInfo`。いずれもパスを保持する。AC-63）。キャッシュ読み込みと実行後のどちらでも同じである。

`Fetch` はキャッシュディレクトリを列挙しない。列挙するのは掃除だけである（後述）。**既知の制限:** macOS の既定（APFS）など大文字と小文字を区別しないファイルシステムでは、大文字と小文字だけが異なる 2 つの動画 ID（例: `aaaaaaaaaaA` と `aaaaaaaaaaa`）の固定名が同じエントリに対応する。この場合、一方のキャッシュを読むと info.json の `id` の照合（AC-47）で `ErrParseInfo` になり、強制再取得は他方の動画のキャッシュを置き換え、掃除は他方の動画のスロットを dangling と誤認しうる。動画 ID の空間（64^11）に対して実際に衝突する確率は無視できるため、本スコープでは対処しない。

**ヒット判定と読み込み。** ポインタを読み、内容が `a` または `b` ちょうどであれば、そのスロットを有効なスロットとする。ポインタの読み取りは有界とし、1 バイトを超える内容を全体として読み込まずに「内容がそれ以外」と判定する（[implementation_handoff.md](implementation_handoff.md) I-04）。ポインタが存在しない場合、内容がそれ以外の場合、およびポインタが通常ファイルでない場合は、有効なキャッシュは無い（キャッシュミス）。以降の表では、通常ファイルでないポインタを「内容がそれ以外」として扱う（ただし種別が異なるので削除はしない）。有効なスロットに字幕ファイルと info.json の両方が存在すればキャッシュヒットとし、どちらかが欠けていればキャッシュミスとする（AC-29）。ヒット時の読み込みは、通常ファイルでない出力を固定名の規則（上記）どおりに扱う。読み込むのは有効なスロットの 2 ファイルだけであり、1 回の出力だけが入ったスロットから読むため、新旧の世代が混在した `Transcript` を返すことはない（AC-69）。

**置き換え（コミット）の手順。** キャッシュミスまたは強制再取得のときに行う。キャッシュヒットではキャッシュを書き換えない。

1. **書き込み先スロットの準備。** 有効なスロットでない側（有効なスロットが無ければ `a`）を書き込み先とする。同じ名前のエントリの種別を辿らずに（lstat で）確認し、ディレクトリが残っていれば削除し、空のディレクトリとして `0o700` で作り直す。これにより前回の残存ファイルを今回の出力と誤認しない（AC-30）。ディレクトリ以外の種別（シンボリックリンクを含む）のエントリが残っていれば、固定名の規則どおりそれに触れず、ファイルシステムのエラーを返す（§4.3）。
2. **出力と検証。** `yt-dlp` に書き込み先スロットへ `-P` で直接出力させる（§3.7 の起動引数、design_handoff H-07）。スロットがそのままステージング領域を兼ねるため、出力ファイルを移動しない。出力を §6.2 の順序で検証し、`Transcript` を組み立てる。通常ファイルでない出力は、固定名の規則（上記）どおりこの検証で拒否され、手順 3 に進まない。
3. **永続化。** 手順 2 の検証を通った（したがって通常ファイルである）書き込み先スロットの 2 ファイルを `0o600` にし（AC-19、design_handoff H-08）、ファイルとスロットの内容をディスクへ同期する（fsync）。
4. **コミット。** 書き込み先スロットの名前をポインタの一時ファイルに書いて同期し、`<id>.current` へリネームする。一時ファイルは、残っている `<id>.current.tmp` が通常ファイルなら削除したうえで、排他作成（`O_CREATE|O_EXCL`）で作る。既存のエントリを開いて書き込まないため、シンボリックリンクを辿らない。`<id>.current.tmp` または `<id>.current` に通常ファイル以外の種別のエントリがあれば、それに触れずにファイルシステムのエラーを返す（リネームでシンボリックリンクのポインタを置き換えない）。これが唯一のコミット点である。リネームは同じディレクトリ内の atomic な置き換えであり、ポインタは旧スロットか新スロットのどちらかを指す。リネームの後、キャッシュディレクトリを同期する（ベストエフォート）。

手順 1〜4 のいずれかが失敗した場合、またはプロセスが終了した場合、ポインタは旧スロットを指したまま（または存在しないまま）であり、既存のキャッシュは変更されていない（AC-58）。元へ戻す操作は無く、書き込み先スロットは dangling なエントリとして後述の削除の対象になる。手順 4 の後、旧スロットは dangling なエントリになる。有効な世代は常に今回の出力だけで構成されるため、「前回の出力にだけ存在した当該動画のキャッシュファイル」は有効なキャッシュに残らない（AC-34）。

**dangling なエントリ。** 当該動画の固定名のエントリ（上表）のうち、次のものを dangling なエントリとする（要件 F-005）。判定はポインタの状態だけで決まり、`Fetch` 時の削除・キャッシュの削除・掃除は同じ判定を使う。

| ポインタの状態 | dangling なエントリ |
|---|---|
| 内容が `a` または `b` | 指していない側のスロット、ポインタの一時ファイル |
| 存在しない | 両方のスロット、ポインタの一時ファイル |
| 内容がそれ以外 | ポインタ、両方のスロット、ポインタの一時ファイル |

ポインタが指すスロットは、中身が欠けていても dangling とはしない。この状態はキャッシュミスとして扱い、次のコミットは他方のスロットへ書くため、欠けたスロットは手順 4 の後に dangling になる。中断で残りうる状態とその後の扱いは §6.3 に列挙する。

**`Fetch` 時の削除。** `Fetch` は、動画 ID を得た後であれば、成功・失敗・キャッシュヒットを問わず、終了時に当該動画の dangling なエントリを削除する（AC-69）。判定には終了時点のポインタの状態を使う。キャッシュディレクトリは列挙しない。削除はベストエフォートとし、失敗しても `Fetch` の戻り値（成功・失敗とそのエラー）を変えない。残ったエントリは、同じ動画の次の `Fetch` または掃除で削除される。

**キャッシュの削除（`RemoveCache`）。** 動画 1 本のキャッシュを削除する公開メソッドを設ける（§3.1）。#6 が投稿（Publisher）の成功後に既定で呼ぶ（要件 2.3）。

1. `ctx` が終了済みなら何もせずに `ctx.Err()` を返す。動画 URL を URL 検証（§3.6）にかけ、不正なら `ErrInvalidVideoURL` をラップして返す（何も削除しない。AC-74）。
2. **ポインタを最初に削除する。** ポインタが通常ファイルとして存在すれば削除する。これがキャッシュの無効化の点であり、削除の直後からその動画はキャッシュミスになる。ポインタの削除に失敗した場合は、他のエントリに触れずにエラーを返す（有効なキャッシュはそのまま残る。AC-75）。ポインタが存在しない、または通常ファイルでない場合はこの手順を飛ばす。
3. 手順 2 の後のポインタの状態で dangling なエントリを判定し（ポインタを削除した場合は「存在しない」の行。両方のスロットとポインタの一時ファイル）、期待する種別のものだけを削除する。失敗しても残りの削除を続け、失敗したエントリのパスをすべて含むエラーを `errors.Join` でまとめて返す（AC-75）。
4. どのエントリも存在しなければ、何も変更せずに成功する（AC-74）。

ポインタを先に削除するため、途中で失敗・中断しても、残る状態は「削除前の有効なキャッシュ」（手順 2 の前）か「ポインタの無いキャッシュミス」（手順 2 の後）のどちらかであり、残ったスロットは dangling なエントリとして次の `Fetch` または掃除で削除される（§6.3 の S6）。スロットを先に消すと、ポインタが中身の欠けたスロットを指す状態が生じるが、これもキャッシュミスであり混在は生じない。それでもポインタを先にするのは、無効化の点を 1 回の削除に固定し、途中の状態を §6.3 の表の既存の行に収めるためである。

**掃除（`PruneCache`）。** 二度と `Fetch` されない動画の dangling なエントリのために、キャッシュディレクトリ全体を対象とする掃除を `YtDlpSource` の公開メソッドとして設ける（§3.1）。#6 は CLI の各実行で呼ぶ（要件 2.3）。キャッシュの削除が既定で行われるため、残る dangling なエントリは中断の分だけであり、各実行での列挙のコストは小さい。

1. キャッシュディレクトリを列挙する（シンボリックリンクを辿らない）。キャッシュディレクトリが存在しなければ何もせずに成功する。
2. 各エントリ名を、`<動画 ID>.<サフィックス>`（サフィックスは `a`・`b`・`current`・`current.tmp` のいずれか）の形に完全に一致するかで判定する。動画 ID は `.` を含まないため、最初の `.` で一意に分割できる。動画 ID の部分は URL 検証と同じ動画 ID の検証（§3.6）を通ったものだけを採る。一致しないエントリは候補にしない（AC-20・AC-71）。
3. 候補を動画 ID ごとにまとめ、上表の判定で dangling なエントリを決める。削除するパスは、列挙した名前ではなく、検証済みの動画 ID から固定名として組み立て直す。
4. 各エントリの種別を、辿らずに（lstat で）確認し、期待する種別と一致するものだけを削除する（AC-71）。
5. 削除に失敗したエントリがあっても残りの削除を続け、失敗したエントリのパスをすべて含むエラーを `errors.Join` でまとめて返す（AC-72）。`ctx` が終了した場合は、次のエントリへ進む前に打ち切り、`ctx.Err()` をそれまでの失敗とあわせて返す。

**単一ライターの前提。** 同じキャッシュディレクトリ・同じ動画に対する `Fetch` とキャッシュの削除の同時実行はサポートしない。掃除の実行中は、同じキャッシュディレクトリに対する `Fetch`・キャッシュの削除・別の掃除も実行しない。いずれも呼び出し側が直列化する（CLI は 1 回の実行で 1 本の動画を処理する）。掃除と `Fetch` が並行すると、実行中の `Fetch` の書き込み先スロットを掃除が dangling として削除しうるためである。CLI は各実行で掃除を呼ぶため、同じキャッシュディレクトリに対して CLI を同時に実行しない前提（またはその排他）は #6 が扱う（要件 2.3）。

- キャッシュの読み書きに使うファイル名は、検証済みの動画 ID から組み立てる。ディレクトリ内の他のファイル名は信用しない。掃除での列挙も、上記の命名規則と動画 ID の検証を通った名前だけを扱う（AC-20）。
- キャッシュディレクトリは `0o700` で作成する（AC-19、design_handoff H-08）。既存のキャッシュディレクトリ・ファイルのパーミッションは検査・変更しない（スコープ外、design_handoff H-11）。
- 他の動画のファイルや無関係なファイルには触れない（上記の固定名の規則。AC-59）。
- キャッシュが一部だけ存在する場合はキャッシュミスとして扱い、再取得する（AC-29）。
- **キャッシュの無効化。** プログラムからは `RemoveCache` で削除する。利用者が手で無効化する場合は `<id>.current` を削除する。有効なスロットやその中のファイルを削除しても、キャッシュミスになる（AC-35）。残ったエントリは次の `Fetch` または掃除で dangling として削除される。
- **キャッシュ読み込み中の削除への追随。** キャッシュの完全性の確認とファイルの読み込みは別のシステムコールである。確認と読み込みの間に利用者がファイルを削除した場合（AC-35 の削除による無効化を含む）、読み込みが「存在しない」で失敗したらキャッシュミスとして扱い、再取得する。存在するが読み取れない場合だけを `ErrParseSubtitles` / `ErrParseInfo`（パスを保持）とする（AC-63）。

### 3.6. URL 検証

`video_id.go` が入力 URL を検証し、動画 ID と正規化 URL を返す（F-001）。スキーム・ホスト・パス形式・動画 ID の文字種・動画 ID の後の余分なパス要素を検証し、合致しなければ `ErrInvalidVideoURL` を返す。キャッシュのパスには動画 ID だけを使い、URL 文字列は使わない（AC-04）。正規化 URL は `Transcript.VideoURL` にも使う（§3.7、design_handoff H-15）。

### 3.7. 事前調査で固定した具体値

§1.4 の観測に基づき、次の値を固定する。いずれも、`02_architecture.md` の承認によって確定する決定事項である。

**起動引数。** F-002 が設計に委ねた `yt-dlp` の引数配列を、次の 1 つに固定する。実行ファイル（`Options.YtDlpPath`、空なら `"yt-dlp"`）の後に、この順で各要素を 1 個の引数として渡し、これ以外の引数は渡さない（AC-06）。

| # | 引数 | 目的 |
|---|---|---|
| 1 | `--ignore-config` | 利用者・システムの設定ファイルを読まない（F-002） |
| 2 | `--no-plugin-dirs` | 既定のプラグインディレクトリを読まない（F-002） |
| 3 | `--skip-download` | 動画本体をダウンロードしない |
| 4 | `--write-subs` | 手動字幕を出力する |
| 5 | `--write-auto-subs` | 自動字幕を出力する |
| 6 | `--sub-langs`、`ja` | 字幕の言語を `ja` に限る |
| 7 | `--sub-format`、`json3` | 字幕の形式を json3 にする |
| 8 | `--write-info-json` | info.json を出力する |
| 9 | `-P`、`<書き込み先スロットのパス>` | 出力先ディレクトリ（§3.5。テンプレートとして展開されない。§1.4） |
| 10 | `-o`、`%(id)s` | 出力テンプレート（固定の文字列） |
| 11 | `--`、`<正規化 URL>` | オプション解釈を止め、正規化 URL を 1 個の引数として渡す（§3.6） |

§1.4 の実測（`--write-subs --write-auto-subs --sub-langs ja` で手動字幕が `ja.json3` に書かれること、`-P` と `-o "%(id)s"` で `<動画 ID>.ja.json3`・`<動画 ID>.info.json` が出力されること）は、この配列の構成要素で確認した。§1.4 のフィクスチャ取得コマンドは自動字幕だけを取得するための調査用であり、本番の引数配列ではない。

**字幕ファイル名と採用規則。** スロット `<cacheDir>/<動画 ID>.<a|b>` の中で、字幕ファイルは `<動画 ID>.ja.json3`、info.json は `<動画 ID>.info.json` とする。この 2 つがスロット内で読むファイルのすべてである（§3.5 の固定名の規則）。`Fetch` が読む字幕ファイルは `<動画 ID>.ja.json3` ただ 1 つで、他の字幕ファイル（例: `<動画 ID>.ja-orig.json3`）は読まない。`<動画 ID>.ja-orig.json3` は起動引数の `--sub-langs ja` では出力されず、キャッシュで読む名前でもない（スロットの中にあっても読まない。キャッシュディレクトリ直下にあれば無関係なファイルとして扱い、作成も削除もしない）。起動引数の `--sub-langs ja` が `ja` の字幕を `ja.json3` に書くため、手動字幕と自動字幕の両方が利用できる場合も、読むファイルは `ja.json3` の 1 つに定まる。§1.4 のとおり、両方がある場合に `ja.json3` に書かれるのは手動字幕だった。ただしその動画の手動トラックは空白だけのイベントが大半で、本文が 2 行しかなかった。手動トラックが実質空の場合に自動字幕へ切り替える仕組みは設けず、既知の制限として記録する（§1.4・§9）。AC-31 は、有効なスロットに両方のファイルがあるキャッシュでは `ja.json3` から採用することを検証する。

**重複の扱い。** §1.4 のとおり、実データの json3 にはローリング表示に由来する重複イベントが無く、包含と時刻の重なりによる素朴な除去は実際の本文を含む行を削除した。要件 F-003 はこれを受けて除去を行わないことにした（仮に重複が含まれても、後段の LLM による記事化で吸収されると見込む）。規則は次のとおりとする。

1. 本文を持たないイベント（`segs` を持たない、または連結した本文が空白文字だけ）は、行区切り・メタ情報としてセグメントに含めない。この判定は `aAppend` の有無に依存しない（§3.4）。
2. ローリング重複の自動除去は行わない。時間的に離れた同一本文は意図的な繰り返しとして本文に残す（AC-46）。実データに含まれる断片と完全な行の組（§1.4）もそのまま残す。
3. 本文を持つ各イベントはちょうど 1 つのセグメントになる。行区切りのイベントの本文を前後のセグメントに連結したり、1 つのイベントから複数のセグメントを作ったりしない（AC-32）。
4. 将来の出力でローリング重複が観測され、成果物に影響する場合は、実データに基づく除去規則を要件とあわせて追加する（§9）。

**サイズ・件数の上限。** 字幕ファイル 8 MiB（8,388,608 バイト）・イベント数 65,536 件、info.json 8 MiB。保存したフィクスチャは字幕 338 KB・704 件、info.json 1.46 MB で、別の約 56 分の動画の観測（字幕 0.9 MB・1,984 件、info.json 1.1 MB）を含めても十分な余裕がある。上限ちょうどの入力は受理し、超える入力は対応する番兵で拒否する（AC-48・AC-49・AC-50、design_handoff H-09）。

**環境変数の allowlist。** 子プロセスに渡す環境変数の集合は次で固定する（AC-66、design_handoff H-01）。

- `PATH`・`HOME`・`TMPDIR`
- `XDG_CONFIG_HOME`・`XDG_CACHE_HOME`
- `HTTP_PROXY`・`HTTPS_PROXY`・`NO_PROXY`・`ALL_PROXY` と、その小文字形（`http_proxy`・`https_proxy`・`no_proxy`・`all_proxy`）
- `LANG`・`LC_ALL`・`LC_CTYPE`
- `SSL_CERT_FILE`・`SSL_CERT_DIR`

秘密情報の変数名（`*_API_KEY`・Webhook URL など）は決して含めない。親の環境で設定されている変数だけを渡し、1 つも設定されていなくても空の（非 nil の）環境を渡す。proxy 変数は認証情報を含みうるため、その値は標準エラー出力をエラーに含める前に伏字にする（§4.2）。

**タイムアウト後の猶予。** タイムアウト・キャンセル時に `exec.CommandContext` が終了させるのは直接の子プロセス（`yt-dlp`）だけであるため、子孫プロセスが標準エラー出力を開いたままでも `Fetch` が有界の時間で戻るように、猶予（`exec.Cmd.WaitDelay` 相当）を 5 秒で固定する（AC-08、design_handoff H-03）。

**統合テストの既定の動画。** `make test-integration` の既定値は、URL `https://www.youtube.com/watch?v=EQCUZyB4DqE`、動画 ID `EQCUZyB4DqE` とする（§1.4。CC BY で、手動トラックが位置調整用でない自動字幕のみの動画）。`YT2COLUMN_TEST_VIDEO_URL` と `YT2COLUMN_TEST_VIDEO_ID` で差し替えられる（F-008）。

### 3.8. コンポーネント責務表

| ファイル | 責務 | 状態 |
|---|---|---|
| `internal/transcript/ytdlp.go` | `Options`・`YtDlpSource`・`NewYtDlpSource`・`Fetch`・`RemoveCache`・`PruneCache` の全体制御とキャッシュ判定。ペアの検証の順序制御 | 新設 |
| `internal/transcript/video_id.go` | URL 検証と動画 ID の抽出、正規化 URL の組み立て（F-001） | 新設 |
| `internal/transcript/exec.go` | `commandExecutor` interface、`os/exec` を使う実装、4 KiB 上限付きでドレインする `cappedWriter`、allowlist 環境の組み立て（F-002） | 新設 |
| `internal/transcript/json3.go` | json3 の解析（ペアの検証の一部。F-003） | 新設 |
| `internal/transcript/info.go` | info.json の解析（ペアの検証の一部。F-004） | 新設 |
| `internal/transcript/cache.go` | 固定名の組み立て、ポインタとスロットの読み込み、書き込み先スロットの準備、ポインタの置き換え、ポインタを先にするキャッシュの削除、dangling なエントリの判定と削除、掃除での列挙と名前の判定（F-005） | 新設 |
| `internal/transcript/errors.go` | 番兵エラーと `ParseError`（F-006・AC-24・AC-36） | 新設 |
| `internal/transcript/video_id_test.go` | AC-01〜AC-05・AC-03 のテスト | 新設 |
| `internal/transcript/json3_test.go` | AC-10〜AC-13・AC-32・AC-45・AC-46・AC-48・AC-49・AC-52・AC-54・AC-55・AC-57・AC-63・AC-67 のテスト | 新設 |
| `internal/transcript/info_test.go` | AC-14〜AC-16・AC-47・AC-50・AC-53・AC-64・AC-65 のテスト | 新設 |
| `internal/transcript/ytdlp_test.go` | AC-06〜AC-09・AC-17〜AC-31・AC-34〜AC-36・AC-44・AC-51・AC-56・AC-58〜AC-63・AC-66・AC-68・AC-69 のテスト（fake の `commandExecutor` と一時キャッシュ） | 新設 |
| `internal/transcript/exec_test.go` | `commandExecutor` の実装と `cappedWriter` のテスト。子プロセスを起動するヘルパーを使い、ドレインと `WaitDelay` の有界性を検証する（AC-08・AC-51・AC-56） | 新設 |
| `internal/transcript/cache_test.go` | ポインタの置き換え・dangling の判定と削除・掃除・パーミッションのテスト（AC-19・AC-20・AC-29・AC-30・AC-34・AC-58・AC-59・AC-70〜AC-75） | 新設 |
| `internal/transcript/integration_test.go` | F-008 の統合テスト（`//go:build integration`。AC-37〜AC-43） | 新設 |
| `testdata/2tcCWM-sRBw.ja.json3` | 実 `yt-dlp` 出力（CC BY 動画の自動字幕。§1.4） | 新設 |
| `testdata/2tcCWM-sRBw.info.json` | 実 `yt-dlp` 出力（CC BY 動画の info.json。§1.4） | 新設 |
| `testdata/README.md` | フィクスチャの出典（動画 URL・チャンネル名・ライセンス）と取得方法 | 新設 |
| `Makefile` | `test-integration` ターゲットの追加、lint のビルドタグを `test,integration` に変更（AC-38・AC-43） | 変更 |
| `.pre-commit-config.yaml` | golangci-lint フックのビルドタグを `test,integration` に変更（AC-43）。`trailing-whitespace`・`end-of-file-fixer` と `check-added-large-files` の対象から `^testdata/` を除外し、`testdata/` のバイト列を書き換えないようにする | 変更 |
| `.github/workflows/ci.yml` | lint ジョブのビルドタグを `test,integration` に変更（AC-43） | 変更 |
| `docs/dev/developer_guide/package_reference.md` | `internal/transcript` の責務の更新（実装追加） | 変更 |
| `docs/dev/developer_guide/requirements_process.md` | 境界チェックの「黙って無視する入力」の記述を、消費するフィールドに限定し、拡張可能な未消費メンバーを除外するよう整合（design_handoff H-13） | 変更 |

`internal/transcript/transcript.go` と `internal/pipeline` は変更しない。既存テストの更新は不要である。

### 3.9. design_handoff の各項目への対応

| ID | 本設計の対応 |
|---|---|
| H-01 | 環境変数は allowlist の変数だけを集めた非 nil のスライスとして `commandExecutor` に渡す（§3.7・§5.2）。 |
| H-02 | 起動引数の配列を §3.7 で 1 つに固定し、出力先ディレクトリ（書き込み先スロット）は `-P`、出力テンプレートは `-o "%(id)s"` で渡す（§3.7・§5.2）。 |
| H-03 | `WaitDelay` 相当の猶予を 5 秒で固定する（§3.7）。 |
| H-04 | 標準エラー出力は 4 KiB を上限に保持しつつ、読み取りを止めずドレインする `cappedWriter` で捕捉する（§3.2・§5.2）。 |
| H-05 | 生バイト列の検証、後続データの検出、`null` と型の不一致の区別、数値の範囲検証を §3.4 の手段で行う。 |
| H-06 | 重複の扱い（§3.7）はローリング重複の除去を行わず、`tStartMs` 以外の数値フィールド（`dDurationMs` など）を読まないため、追加の数値検証は行わない（該当なし）。除去規則を将来追加する場合は、読むフィールドの検証とオーバーフロー対策をあわせて定める。 |
| H-07 | 候補の「ステージング領域」と「リネームによる置き換え」を、有効でない側のスロットへの出力（ステージングを兼ねる）とポインタのリネームによる置き換えとして採る。固定名だけに触れる規則、中断で残る dangling なエントリの削除、他動画のファイルに触れないことを §3.5 で定める。候補の `os.MkdirTemp` による一意名は採らず、固定名のスロットを使う（掃除で dangling を判定できるようにするため。付録A）。コミット時の失敗は、ポインタを置き換える前であれば既存のキャッシュを変更しない（§3.5・§6.3）。 |
| H-08 | スロットは `0o700` で作り、コミット前にスロット内のファイルを `0o600` にする（§3.5）。 |
| H-09 | 上限値を §3.7 で固定する。 |
| H-10 | テストの組み立て方は §7.1・§7.2 の方針に従う（fake の executor、fake で起こす失敗と直接作るキャッシュの状態、実データ）。 |
| H-11 | 既存キャッシュのパーミッションは検査・変更しない（スコープ外、§3.5・§5.2）。 |
| H-12 | 境界の網羅チェックリストを §3.4・§3.7 の設計に反映した。 |
| H-13 | 消費するフィールドだけを厳密に検証し、未消費メンバーは無視する（§3.4）。あわせて `requirements_process.md` の文言を整合させる（§3.8）。 |
| H-14 | キャッシュ読み込み時も、字幕ファイルを info.json より先に検証する決定的な順序を適用する（§6.2）。 |
| H-15 | `Transcript.VideoURL` には常に正規化 URL を入れる（§3.6）。 |
| H-16 | 破棄するイベント（マーカー、`segs` なし）は `tStartMs` の検証対象にしない。検証対象は本文を持つイベントだけであり、順序は §6.1 で固定する。 |

---

## 4. エラーハンドリング設計 (Error Handling Design)

### 4.1. エラー型

```mermaid
classDiagram
    class ParseError {
        <<struct>>
        +Path string
        +Err error
        +Error() string
        +Unwrap() error
    }
    class TranscriptSource {
        <<interface>>
        +Fetch(ctx context.Context, videoURL string) (Transcript, error)
    }
    class YtDlpSource {
        <<struct>>
        -options Options
        -exec commandExecutor
        +Fetch(ctx context.Context, videoURL string) (Transcript, error)
        +RemoveCache(ctx context.Context, videoURL string) error
        +PruneCache(ctx context.Context) error
    }
    TranscriptSource <|.. YtDlpSource : implements
    YtDlpSource --> ParseError : returns
```

**図4 主要な型と interface**。`<|..` は「左の型が右の interface を実装する」関係、`-->` は「左が右の型を返す」関係を表す。`ParseError` は `Unwrap()` で対応する番兵（`ErrParseSubtitles` または `ErrParseInfo`）を返す。他の番兵（`ErrInvalidVideoURL`・`ErrYtDlpExec`・`ErrNoSubtitles`）は `ParseError` を介さないが、番兵そのものは返さない。

**番兵のラップ。** 本パッケージの番兵は、いずれも文脈を付けてラップしたエラーとして返し、`errors.Is` による判別を保つ（要件 3.2・3.2.1）。番兵ごとに付ける文脈は次のとおりである（メッセージの方針は §4.2）。

| 番兵 | ラップの形 | 付ける文脈 | 主な AC |
|---|---|---|---|
| `ErrInvalidVideoURL` | 文脈付きのラップ | 拒否の理由（スキーム・ホスト・パス形式・動画 ID の文字種など、どの検証に失敗したか） | AC-03・AC-24 |
| `ErrYtDlpExec` | 文脈付きのラップ | 解決した実行ファイルのパス、終了状態（非ゼロ終了のコード、または起動・待機の失敗の種別）、捕捉した標準エラー出力（先頭 4 KiB まで、§4.2 の伏字化の後。ある場合） | AC-09・AC-51・AC-62 |
| `ErrNoSubtitles` | 文脈付きのラップ | 対象の動画 ID と、字幕ファイルの不在かセグメント 0 件かの区別 | AC-21・AC-22・AC-23・AC-61 |
| `ErrParseSubtitles`・`ErrParseInfo` | `*ParseError`（§3.1） | 対象ファイルのパス（`Path`） | AC-36・AC-63 |

図6・図7 の終端に書いた番兵名は、返るエラーが `errors.Is` で判別できる番兵を表し、番兵そのものが返ることを意味しない。

タイムアウトとキャンセルは本パッケージが定義する番兵ではなく、`context.DeadlineExceeded`・`context.Canceled` として返す（AC-08・AC-24・AC-26）。

### 4.2. エラーメッセージ設計パターン

- エラーには対象（動画 ID またはパス）を含め、字幕がないことはメッセージから分かるようにする（AC-22）。番兵ごとに付ける文脈は §4.1 の「番兵のラップ」で定める。
- パースエラーのメッセージには対象ファイルのパスを含め、利用者が破損したファイルを特定できるようにする（AC-36）。`yt-dlp` の出力のパースに失敗した場合、パスは書き込み先スロットを指す。このスロットは `Fetch` の終了時に dangling なエントリとして削除されるため、メッセージは「そのファイルがまだ存在する」ことを前提にしない（利用者が再現するには同じ動画を強制再取得する）。
- `yt-dlp` の実行失敗（`ErrYtDlpExec`）のメッセージには、終了状態（非ゼロ終了のコード、または起動・待機の失敗の種別）と、解決した実行ファイルのパス（`Options.YtDlpPath`、空なら `"yt-dlp"`）を含める。誤った・古い `yt-dlp` を特定できるようにするためである。
- `yt-dlp` の標準エラー出力をメッセージに含める場合、メッセージには保持した先頭 4 KiB だけを含める（AC-51）。
- **標準エラー出力の伏字化。** allowlist の proxy 変数（§3.7）は `https://user:password@proxy` のように認証情報を含むことがあり、`yt-dlp` はその値を標準エラー出力に書きうる。4 KiB の上限は量を制限するだけで、開示は防がない。そこで、保持した標準エラー出力を `ErrYtDlpExec` に含める前に、子プロセスへ実際に渡した proxy 変数の空でない値と、URL の userinfo（`://` と `@` の間）を伏字に置き換える。上限の境界で切れた値の断片も残さない。proxy 以外の allowlist の変数（`PATH`・`HOME`・locale など）は秘密情報ではないため置き換えない。
- **秘密情報がエラーに現れない範囲。** 本パッケージは秘密情報を読まず、子プロセスへ渡す環境は allowlist で組み立て、親の値をエラーへ付けない（AC-44・AC-66 が検証するのはこの機構である）。したがって allowlist 外の秘密情報（API キー・Webhook URL など）は子プロセスにも、そこからエラーにも届かない。allowlist 内で認証情報を運びうる proxy 変数の値は、上記の伏字化によってエラーから除く。`yt-dlp` が環境変数以外の出所（キャッシュディレクトリ外の自身のファイルなど）から得た値を標準エラー出力に書く場合は、この保証の対象外とする。

### 4.3. 失敗経路と番兵の対応

要件 3.2.1 の表をそのまま実装する。設計上の補足は次のとおり。

- **検証の実行フェーズ。** 番兵を返すのは、キャッシュが揃っている場合の読み込みと、`yt-dlp` の正常終了後の検証である。キャッシュが一部だけ存在する場合はどの失敗経路にも該当せず、キャッシュミスとして `yt-dlp` を起動する（AC-29）。
- **字幕優先の順序。** 実行後はもとより、キャッシュ読み込み時も、字幕ファイルを info.json より先に各段階で検証する。これにより、両方に失敗がある場合に返る番兵が実装の順序に依存しない（AC-68、design_handoff H-14）。
- **info.json 不在の扱い。** 「字幕ファイルは存在するが info.json が存在しない」は、キャッシュが揃っている場合には起こらない（揃っていない場合はキャッシュミス）。`yt-dlp` の正常終了後は `ErrParseInfo` とする（AC-60）。
- **破棄イベントの扱い。** 本文を持たないイベントは `tStartMs` の検証前に破棄する。同じ入力に対して返る番兵が実装の順序に依存しない（design_handoff H-16）。
- **キャッシュのファイルシステム操作の失敗。** 書き込み先スロットの準備、パーミッションの変更、同期、ポインタの一時ファイルの書き込みとリネームの失敗は要件 3.2.1 の表に無い。これらには番兵を定義せず、`*fs.PathError` などをラップしたエラーとして返す。いずれもポインタの置き換え（コミット点）以前の失敗であり、ポインタは旧スロットを指したままなので、既存のキャッシュは変更されていない（§3.5）。元へ戻す操作は無い。**dangling なエントリの削除だけの失敗は `Fetch` の戻り値を変えない**（成功は成功のまま、失敗はその失敗のまま）。コミット後のキャッシュディレクトリの同期の失敗も同様である。呼び出し元はこれらをファイルシステムのエラーとして扱い、番兵では判別しない。
- **キャッシュの削除の失敗。** `RemoveCache` は URL 検証の失敗だけを `ErrInvalidVideoURL` で返し、ファイルシステムの失敗には番兵を定義しない。ポインタの削除の失敗はそのエラーを、その後の削除の失敗は `errors.Join` でまとめたエラーを返す（AC-75）。
- **掃除の失敗。** `PruneCache` は番兵を定義せず、削除に失敗したエントリごとの `*fs.PathError` などを `errors.Join` でまとめて返す（AC-72）。キャッシュディレクトリの列挙自体の失敗は、そのエラーをラップして返す。
- **キャッシュ読み込み中のファイル消失。** 完全性の確認後にファイルが削除された場合は「存在しない」としてキャッシュミスに倒し、再取得する（§3.5）。「存在するが読み取れない」とは区別する。

### 4.4. サイドエフェクト契約

本タスクで外部への影響を変えるオプションは `Options.ForceRefresh` だけである。

| オプション | ネットワーク・外部コマンド | キャッシュの読み書き | 書き込み先スロット |
|---|---|---|---|
| `ForceRefresh = false`（既定） | キャッシュミス時だけ実行する | 読み込みは常に。ポインタの置き換えは実行成功時だけ | 実行時だけ作成する。成功時は有効なスロットになり、失敗時は終了時に削除する |
| `ForceRefresh = true` | キャッシュが揃っていても実行する | 実行成功時だけポインタを置き換える。失敗時は変更しない | 同上 |

いずれの場合も、`Fetch` がエラーを返したときはキャッシュの内容を変更せず、終了時に当該動画の dangling なエントリを削除する（AC-58・AC-59・AC-69）。`RemoveCache` と `PruneCache` はネットワークも外部コマンドも使わない。`RemoveCache` は指定した動画のエントリだけを削除し（AC-73）、`PruneCache` は dangling なエントリだけを削除する（有効なキャッシュは変更しない。AC-70）。

---

## 5. セキュリティ考慮事項 (Security Considerations)

本機能は、`.claude/commands/_context.md` の条件付きガイドのトリガー（外部コマンドの起動、キャッシュの読み書き、ネットワーク、信頼できないテキスト）のすべてに該当する。そのため [security.md](../../dev/security.md) §1・§3・§5 を参照する。

### 5.1. 脅威モデル：外部コマンドと信頼できない出力

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    CALLER[("呼び出し元<br>URL")]
    SRC["YtDlpSource"]
    SECRETS[("親プロセスの<br>秘密情報")]
    YTDLP["yt-dlp"]
    OUT[("json3 / info.json<br>（信頼できない）")]
    FS[("キャッシュ")]

    CALLER --> SRC
    SECRETS -->|"allowlist 以外は渡さない"| SRC
    SRC --> YTDLP
    YTDLP --> OUT
    OUT --> SRC
    SRC --> FS

    class CALLER,SECRETS,OUT,FS data
    class SRC enhanced
    class YTDLP problem
```

**図5 脅威モデル**。実線の矢印 A → B は「A が B に影響を及ぼし、またはデータを渡す」ことを表す。`SECRETS` → `SRC` の矢印には「allowlist 以外は渡さない」という制約を付す。`YTDLP` と `OUT` は信頼しない。`YtDlpSource` は `yt-dlp` に引数を配列で渡してシェルを経由せず、環境は allowlist に限定し、出力は境界で検証してからキャッシュ・`Transcript` に取り込む。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    D[("データ")]
    E["防御するコンポーネント"]
    X["信頼しない外部要素"]

    class D data
    class E enhanced
    class X problem
```

**凡例（図5 脅威モデル）**。

### 5.2. 外部コマンド（`yt-dlp`）の起動

- 引数を配列で渡し、シェルを経由しない（AC-07）。URL は正規化 URL を 1 個の引数として渡し、直前の `--` でオプション解釈を止める（AC-06）。
- 子プロセスへ渡す環境は §3.7 の allowlist に限定する。allowlist の変数が 1 つも設定されていなくても、空の非 nil の環境を渡して親の環境を引き継がせない（AC-44・AC-66、design_handoff H-01）。
- `--ignore-config` と `--no-plugin-dirs` を常に渡し（§3.7 の起動引数）、利用者・システムの設定ファイルとプラグインを読み込ませない（AC-06）。設定ファイルからの `--exec` などのオプション注入や出力先の上書きを防ぐ。
- 引数配列は §3.7 の「起動引数」で固定した 1 つだけを使う。出力先ディレクトリ（書き込み先スロット）は `-P`、出力テンプレートは `-o "%(id)s"` で渡す。キャッシュディレクトリのパスを `-o` に含めないため、パス中の `%` が書式として解釈されない（AC-06、design_handoff H-02）。
- タイムアウトを設定し、タイムアウト・キャンセル後は 5 秒の猶予で有界に戻る（AC-08、§3.7）。
- 標準エラー出力は読み取りの時点で 4 KiB に制限しつつドレインし、子プロセスの出力量によらず親のメモリを消費しない。子プロセスが大量の標準エラー出力を出しても、その終了を妨げない（AC-51・AC-56、design_handoff H-04）。
- **書き込み先スロットでも読む名前は固定する。** `yt-dlp` には `-o "%(id)s"` で出力させるが、`YtDlpSource` が読むのは検証済みの動画 ID から組み立てた `<id>.ja.json3` と `<id>.info.json` だけである。`yt-dlp` が別の名前で出力したファイルは読まない。したがって、出力名の展開が検証済みの ID と食い違っても、キャッシュに取り込まれることはない。
- **前提:** キャッシュディレクトリは利用者が用意したユーザー専用のディレクトリである（design_handoff H-11）。

**残余リスク（記録）。** タイムアウト・キャンセルで終了させるのは直接の子プロセスだけである。`yt-dlp` が起動した子孫プロセスは残りうる。`Fetch` は 5 秒の猶予で有界に戻るが（§3.7）、子孫プロセスが CPU やネットワークを使い続ける可能性は残る。プロセスグループの操作は行わない。この残余リスクを記録し、対処は問題になった時点で行う。

### 5.3. キャッシュ

- ディレクトリは `0o700`、ファイルは `0o600` で作成する（AC-19）。
- ファイル名は検証済みの動画 ID から組み立て、URL 文字列やディレクトリ内の任意のファイル名を信用しない（AC-04・AC-20）。
- 掃除でディレクトリを列挙する場合も、命名規則に完全に一致し動画 ID の検証を通った名前だけを候補とし、削除するパスは検証済みの動画 ID から組み立て直す。シンボリックリンクを辿らず、期待する種別と異なるエントリには触れない（AC-71、[security.md](../../dev/security.md) §5）。
- `yt-dlp` の出力は書き込み先スロットに閉じ込め、コミットしなかったスロットは dangling なエントリとして削除する（AC-59・AC-69）。

### 5.4. 信頼できない出力の取り扱い

- 字幕・タイトル・概要欄は信頼できない入力として扱い、本タスクではキャッシュへの保存と `Transcript` としての受け渡しだけを行う。他のコマンドやコードとして実行しない。
- サイズ・件数の上限（§3.7）により、極端に大きな入力でもメモリを消費し尽くさない（AC-48〜AC-50）。
- 秘密情報がエラーに現れない範囲と、標準エラー出力の伏字化は §4.2 で定める。

### 5.5. 対象クライアント環境の検証

本タスクは外部サービスの新機能（Slack Block Kit など）を利用しない。対象クライアント環境（`_context.md` Domain-specific）に依存する新しい API 機能はない（該当なし）。

---

## 6. 処理フロー詳細 (Processing Flow Details)

### 6.1. `Fetch` の全体フロー

```mermaid
flowchart TD
    Start(["Fetch(ctx, videoURL)"]) --> Cancel{"ctx は既に<br>終了済みか"}
    Cancel -->|"はい"| Canceled(["ctx.Err()<br>（context.Canceled /<br>context.DeadlineExceeded）"])
    Cancel -->|"いいえ"| Parse["URL を検証し<br>動画 ID・正規化 URL を得る"]
    Parse -->|"失敗"| Invalid(["ErrInvalidVideoURL"])
    Parse -->|"成功"| Hit{"ポインタが指すスロットに<br>2 ファイルが揃っていて<br>強制再取得でないか"}
    Hit -->|"はい"| Load["キャッシュを読み込み<br>字幕 → info.json の順に検証"]
    Load -->|"失敗"| Fail1(["番兵（パスを保持）"])
    Load -->|"読み込み中に消失"| Stage
    Load -->|"成功"| Done(["Transcript"])
    Hit -->|"いいえ"| Stage["書き込み先スロットを<br>空で作成"]
    Stage --> Exec["yt-dlp を起動<br>（timeout・WaitDelay・allowlist 環境）"]
    Exec -->|"タイムアウト / キャンセル"| Fail2(["context.DeadlineExceeded /<br>context.Canceled"])
    Exec -->|"起動・待機失敗 / 非ゼロ終了"| Fail3(["ErrYtDlpExec"])
    Exec -->|"成功"| Verify["字幕 → info.json の順に検証"]
    Verify -->|"字幕なし / 0 件"| Fail4(["ErrNoSubtitles"])
    Verify -->|"パース失敗"| Fail5(["ErrParseSubtitles /<br>ErrParseInfo（パスを保持）"])
    Verify -->|"成功"| Commit["0o600 化・同期の後<br>ポインタを置き換え（コミット）"]
    Commit -->|"失敗"| Fail6(["ファイルシステムのエラー<br>（ポインタは旧のまま。<br>既存のキャッシュは不変）"])
    Commit --> Done
    Load -.->|"終了時"| Cleanup["当該動画の<br>dangling なエントリを削除"]
    Stage -.-> Cleanup
    Exec -.-> Cleanup
    Verify -.-> Cleanup
    Commit -.-> Cleanup
```

**図6 `Fetch` の全体フロー**。矢印 A → B は「A の次に B を行う」を表し、`-->|"条件"|` は条件分岐を表す。点線の矢印は、動画 ID を得た後であれば、キャッシュヒットを含め成功・失敗を問わず `Fetch` の終了時に当該動画の dangling なエントリを削除することを表す（ベストエフォート。§3.5）。キャッシュの置き換えは、検証に成功して `Transcript` を組み立てられた場合にだけ、ポインタのリネーム 1 回で行う。その前に失敗した場合はファイルシステムのエラーを返し、ポインタは旧スロットを指したままなので既存のキャッシュは変わらない（§3.5・§4.3）。終端の番兵名の意味は §4.1 の「番兵のラップ」のとおりである。

### 6.2. 検証順序（キャッシュ読み込みと実行後で共通）

```mermaid
flowchart TD
    A["字幕ファイルの存在"] -->|"なし（実行後のみ）"| N(["ErrNoSubtitles"])
    A -->|"あり"| B["字幕ファイルの読み取り"]
    B -->|"不能"| P1(["ErrParseSubtitles<br>（パスを保持）"])
    B -->|"可能"| C["json3 の検証とセグメント化"]
    C -->|"不正"| P2(["ErrParseSubtitles"])
    C -->|"0 件"| N
    C -->|"1 件以上"| D["info.json の存在・読み取り"]
    D -->|"なし（実行後のみ）・不能"| P3(["ErrParseInfo"])
    D -->|"あり"| E["info.json の検証"]
    E -->|"不正"| P4(["ErrParseInfo"])
    E -->|"正常"| OK(["Transcript を組み立て"])
```

**図7 検証順序**。矢印 A → B は「A の検証を通過したら B の検証へ進む」を表す。字幕ファイルを info.json より先に検証するため、両方に失敗がある場合の番兵は字幕側になる（AC-61・AC-68）。キャッシュが揃っている場合、2 ファイルは存在するが、読み取りは失敗しうる（AC-63）。通常ファイルでないエントリ（シンボリックリンクを含む）は存在するが読み取り不能として扱う（§3.5 の固定名の規則）。ファイルが存在しない場合はキャッシュミスとして扱われ（§3.5）、番兵を返すのは実行後だけである。

### 6.3. キャッシュ更新の状態と中断時の残留

```mermaid
stateDiagram-v2
    [*] --> 旧世代が有効
    旧世代が有効 --> 書き込み中: 書き込み先スロットを空で作成し yt-dlp が出力
    書き込み中 --> 永続化済み: 検証・Transcript の組み立て・0o600 化・同期に成功
    書き込み中 --> 旧世代が有効: 失敗・中断（書き込み先スロットは dangling）
    永続化済み --> 旧世代が有効: ポインタの置き換え前の失敗・中断（書き込み先スロットは dangling）
    永続化済み --> 新世代が有効: ポインタをリネームで置き換え（コミット点）
    新世代が有効 --> [*]: 成功を返す（旧スロットは dangling）
    旧世代が有効 --> [*]: 失敗を返す（既存のキャッシュは不変）
```

**図8 キャッシュ更新の状態**。遷移 A --> B は「A から B への状態変化」を表す。「旧世代が有効」と「新世代が有効」は、ポインタがそれぞれ旧スロット・新スロットを指す状態である（初回の取得では「旧世代が有効」はポインタが存在しない状態を表す）。状態を変えるのはポインタのリネームだけであり、元へ戻す遷移は無い。いずれの終端でも、`Fetch` は終了時に当該動画の dangling なエントリを削除する（§3.5）。

**中断時の残留（AC-69）。** プロセスが途中で終了すると `Fetch` 時の削除が走らず、次の状態が残りうる。旧世代がスロット `a` にある場合を示す（`b` の場合は `a` と `b` を入れ替える）。いずれも、次の `Fetch`（ヒット・ミス・強制再取得・失敗のいずれでも）または掃除で dangling なエントリが削除され、強制再取得なしの `Fetch` はポインタが指す一方の世代だけから `Transcript` を返す。

| 状態 | 中断した時点 | 残る状態 | 次の `Fetch` の読み込み | dangling なエントリ |
|---|---|---|---|---|
| S1 | 書き込み先スロットの準備中・`yt-dlp` の実行中・検証中 | ポインタ `a`、スロット `b`（空または途中） | 旧世代（ヒット） | `b` |
| S2 | 永続化の後、ポインタの一時ファイルの書き込み中またはリネームの前 | ポインタ `a`、スロット `b`（完全）、`<id>.current.tmp` | 旧世代（ヒット） | `b`、`<id>.current.tmp` |
| S3 | ポインタのリネームの後、旧スロットの削除の前または途中 | ポインタ `b`、スロット `a`（完全または途中）、スロット `b`（完全） | 新世代（ヒット） | `a` |
| S4 | 初回の取得で、ポインタのリネームの前 | ポインタなし、スロット `a`（任意）、`<id>.current.tmp`（ある場合） | なし（ミス） | `a`、`<id>.current.tmp` |
| S5 | （中断以外）ポインタの内容が `a`・`b` 以外になった場合 | 不正なポインタ、スロット（任意） | なし（ミス） | ポインタ、両方のスロット、`<id>.current.tmp` |
| S6 | キャッシュの削除で、ポインタを削除した後 | ポインタなし、スロット（任意）、`<id>.current.tmp`（ある場合） | なし（ミス） | 両方のスロット、`<id>.current.tmp` |

S1〜S4・S6 のテストは、各状態をキャッシュディレクトリに直接作ってから `Fetch` と掃除を実行して行う（§7.1）。キャッシュの削除がポインタの削除の前に中断した場合は、削除前の有効なキャッシュがそのまま残る（表に追加の行は要らない）。

### 6.4. 掃除の流れ

`PruneCache` は §3.5 の「掃除」の手順 1〜5 に従う。列挙した名前は、命名規則と動画 ID の検証を通った場合にだけ動画 ID ごとの候補になり、dangling の判定は `Fetch` 時と同じ表（§3.5）を使う。削除するパスは検証済みの動画 ID から組み立て直し、種別を辿らずに確認してから削除する。個々の削除の失敗では打ち切らない。

---

## 7. テスト戦略 (Test Strategy)

### 7.1. ユニットテスト

実 `yt-dlp` を呼び出さず、ネットワークにも接続しない（AC-27）。`commandExecutor` をテスト用の fake に差し替え、`testdata/` の実出力と `t.TempDir` のキャッシュを使う。例外として、実装（`exec.go`）のテストは、標準エラー出力を大量に書く、標準エラー出力を開いたまま残る子孫プロセスを起動する、といった挙動を再現するために、`t.TempDir` に置いた小さなヘルパー実行ファイル（シェルスクリプト、またはテストバイナリの再実行）を起動する。これは実 `yt-dlp` でもネットワークでもない。

- **URL 検証:** AC-01〜AC-05 をテーブルテストで検証する。受理する形式と、拒否すべき入力（スキーム・ホスト・動画 ID・余分なパス要素・空白）を含める。
- **パーサ:** `testdata/2tcCWM-sRBw.ja.json3` と `testdata/2tcCWM-sRBw.info.json` を入力にする。正常系（AC-10・AC-11・AC-13・AC-14・AC-32・AC-46・AC-67）と、拒否系（AC-12・AC-15・AC-16・AC-45・AC-47〜AC-50・AC-52〜AC-55・AC-57・AC-63〜AC-65）を検証する。拒否系の入力は、実データの一部を壊したもの（`null` への置換、型の変更、上限超過など）と、実データと同じ構造を持つ最小入力を組み合わせる。UTF-8・サロゲートのサンプルはバイト列で用意する。§3.4 の各対象レベル（json3 のトップレベル・`events` の要素・`segs` の要素、info.json のトップレベル）で消費するメンバーを重複させた入力が、値が同じ場合も異なる場合も対応する番兵で拒否され、消費しないメンバーの重複は受理されることを確認する（AC-12・AC-15・AC-64・AC-67）。
- **`Fetch` の制御:** fake の executor が記録した引数・環境・標準エラー出力と、キャッシュの読み書きを検証する（AC-06〜AC-09・AC-17〜AC-31・AC-34〜AC-36・AC-44・AC-51・AC-56・AC-58〜AC-63・AC-66・AC-68）。
  - 引数の検証では、記録した引数配列が §3.7 で固定した起動引数と要素ごとに一致すること（`-P` に書き込み先スロット、`-o` に固定のテンプレート、`--` の直後に正規化 URL）を確認する。キャッシュディレクトリ名に `%(title)s` と `%%` を含めて起動し、`-o` に現れないことを確認する（AC-06）。
  - 環境の検証では、親の環境に `YT2COLUMN_TEST_UNLISTED_MARKER`・`DEEPSEEK_API_KEY`・allowlist の各変数を設定し、渡された環境を確認する。allowlist を 1 つも設定しない場合に空の非 nil であることも確認する（AC-44・AC-66）。
  - 標準エラー出力の検証では、4 KiB を大きく超える出力を fake から書き、エラーに含まれるのが先頭 4 KiB だけであること、子が自分で終了して `ErrYtDlpExec` になることを確認する（AC-51・AC-56）。
  - 伏字化の検証では、認証情報を含む proxy 変数を親の環境に設定し、fake がその値と userinfo を含む URL を標準エラー出力に書いたとき、エラーにその値も認証情報も現れないことを確認する。値が 4 KiB の境界で切れる場合も含める（§4.2）。
  - 失敗時のキャッシュ保護は、成功後に fake の executor で各失敗（非ゼロ終了・出力なし・パースできない出力など）を起こし、有効なスロットのファイルとポインタのバイト列が変わらないことと、dangling なエントリが残らないことを確認する（AC-58・AC-59）。コミット段階の個々の操作（`0o600` 化・同期・ポインタの一時ファイルの書き込み・リネーム）には失敗を注入しない。差し替えられる境界は `commandExecutor` だけであり（§3.3）、これらの操作を移植性のある方法で個別に失敗させる手段が無いためである。その途中の失敗が残しうるディスク上の状態は §6.3 の各状態に限られるため、各状態を直接作って `Fetch` の振る舞いを確認する（次項、AC-69）。キャッシュディレクトリに当該動画 ID で始まる無関係なファイル（例: `<id>.notes`）を置き、成功・失敗のいずれの後も変更されずに残ることを確認する（AC-59）。
  - 中断時の残留は、§6.3 の表の各状態をキャッシュディレクトリに直接作り、キャッシュヒット・キャッシュミス・強制再取得の成功・失敗の各 `Fetch` の後に、当該動画の dangling なエントリが残らないことと、返る `Transcript` がポインタの指す一方の世代だけから組み立てられていることを確認する。新旧のスロットには本文とメタ情報が異なる内容を置き、混在を検出できるようにする（AC-69）。
  - キャッシュ読み込み中にファイルを削除し、キャッシュミスとして再取得することを確認する（§3.5）。
  - キャッシュのファイルシステム操作の失敗は、移植性のある方法で再現できるものだけを起こす。固定名に期待と異なる種別のエントリ（スロット名のファイル、`<id>.current.tmp` のディレクトリ、`<id>.current` のシンボリックリンクなど）を置き、番兵ではなくファイルシステムのエラーが返り、既存キャッシュとそのエントリが変わらないことを確認する（§3.5・§4.3）。dangling なエントリの削除だけの失敗は `Fetch` の結果を変えないことも確認する。
  - スロット内の出力が通常ファイルでない場合（固定名の規則、§3.5）は、fake の executor が字幕ファイルまたは info.json としてシンボリックリンクを出力する場合と、有効なスロットにシンボリックリンクを置いたキャッシュの場合の両方で、字幕側は `ErrParseSubtitles`、info.json 側は `ErrParseInfo`（パスを保持）が返り、リンク先のファイルの内容とパーミッションが変わらず、既存のキャッシュが変わらないことを確認する（AC-58・AC-63）。
- **キャッシュの削除:** 成功後の削除で当該動画のエントリがすべて消え、続く `Fetch` がキャッシュミスになり、別の動画のキャッシュと無関係なファイルが変わらないことを確認する（AC-73）。キャッシュの無い動画と不正な URL では何も変わらないことを確認する（AC-74）。ポインタの削除の失敗とスロットの削除の失敗を、パーミッションで再現できる環境（root 以外での実行）でそれぞれ起こし、エラーが返ることと、続く `Fetch` が削除前のキャッシュから返るか（ポインタの削除の失敗）キャッシュミスになるか（スロットの削除の失敗）のどちらかで、その後に dangling なエントリが残らないことを確認する（AC-75）。
- **掃除:** 複数の動画について §6.3 の各状態と有効なキャッシュ、無関係なファイル（`<id>.notes`、命名規則に似ているが一致しない名前、動画 ID として不正な部分を持つ名前）、期待と異なる種別のエントリ（固定名のシンボリックリンク、ファイルであるべき名前のディレクトリ）を置いて `PruneCache` を実行し、dangling なエントリだけが消え、それ以外が存在して内容が変わらず、続く `Fetch` が有効なキャッシュから `yt-dlp` を起動せずに返すことを確認する（AC-70・AC-71）。シンボリックリンクの先にあるファイルが残ることも確認する。削除できない dangling なエントリを含めて実行し、他の dangling なエントリが削除され、返るエラーに失敗したパスがすべて含まれることを確認する（AC-72）。
  - パーミッションは、作成後に `os.Stat` で確認する（AC-19）。
- **実プロセスを使う境界のテスト:** `exec_test.go` で、標準エラー出力を 4 KiB を大きく超えて書くヘルパーと、標準エラー出力を開いたまま残る子孫を起動するヘルパーを使い、ドレイン（読み取りを止めないこと）と `WaitDelay` による有界な待機を検証する（AC-08・AC-51・AC-56）。`cappedWriter` 自体も直接検証する。
- 各テストは、対象の仕組みを実際に壊して失敗することを確認してからコミットする（[CLAUDE.md](../../../CLAUDE.md) Testing Strategy）。

### 7.2. 統合テスト

実 `yt-dlp` とネットワークを使う（F-008）。`//go:build integration` で既定のテストから分離し、`make test-integration` で手動実行する（AC-37・AC-38）。`YT2COLUMN_TEST_VIDEO_URL` と `YT2COLUMN_TEST_VIDEO_ID` を使う。これらの `make` における既定値は §3.7 のとおり固定した。キャッシュは `t.TempDir` を使う（AC-42 を含む）。確認内容は、実際の動画からの `Transcript` 取得（AC-39）、キャッシュの再利用（AC-40）、強制再取得（AC-41）である。`yt-dlp` が見つからない場合やネットワークの失敗はスキップせず失敗として報告する。統合テストファイルは `make lint`・pre-commit・CI の lint の解析対象に含める（AC-43）。

### 7.3. セキュリティテスト

- 環境 allowlist により秘密情報が子プロセスへ渡らないこと（AC-44・AC-66）を §7.1 の環境検証で確認する。
- キャッシュのパスに URL 文字列が現れないこと（AC-04）を、`v` 以外のクエリパラメータに `../` を含む URL で確認する。
- サイズ・件数の上限により、極端に大きな入力が拒否されること（AC-48〜AC-50）を確認する。
- 標準エラー出力の保持量が上限を超えないこと（AC-51）と、proxy 変数の認証情報がエラーに現れないこと（§4.2）を確認する。

### 7.4. 受け入れ基準と設計要素の対応

| AC | 設計要素 | テストの対象 |
|---|---|---|
| AC-01〜AC-05 | `video_id.go` の URL 検証 | URL 検証のテーブルテスト |
| AC-06 | §3.7 で固定した起動引数（§5.2） | fake executor が記録した引数（`%` 入りのディレクトリ名を含む） |
| AC-07 | `commandExecutor` interface（§3.2） | fake executor への差し替えと引数・環境の観測 |
| AC-08 | タイムアウト・キャンセル・5 秒の猶予（§3.7） | fake によるタイムアウトと、ヘルパー実行ファイルの子孫プロセスが標準エラー出力を開いたまま残るケース（`exec_test.go`） |
| AC-09 | 非ゼロ終了・実行ファイル不在の番兵と文脈付きのラップ（§4.1） | fake が非ゼロ・起動不能を返すケース |
| AC-10・AC-11 | json3 のセグメント化（§3.4） | 実データと最小入力のパース |
| AC-12 | 受理しない形の拒否（§3.4） | `null`・非配列・後続データの入力 |
| AC-13 | `testdata/` の実出力（§1.4） | 実データのパース |
| AC-14〜AC-16 | info.json のメタ情報と必須項目（§3.4） | 実データと欠落・空の入力 |
| AC-17・AC-18 | キャッシュヒット・ミスの判定（§3.5・§6.1） | fake executor の呼び出し回数とキャッシュの有無 |
| AC-19 | ディレクトリ `0o700`・ファイル `0o600`（§3.5） | `os.Stat` による確認 |
| AC-20 | 動画 ID からのファイル名組み立てと、掃除での名前の判定（§3.5） | キャッシュのパスと読み書き、掃除の候補 |
| AC-21〜AC-23 | 字幕なしの検出と動画 ID を付けたラップ（§4.1・§4.3・§6.2） | 字幕ファイルなし・0 件のケース |
| AC-24 | 番兵の判別（§4.1） | `errors.Is` による各番兵の判別 |
| AC-25 | `TranscriptSource` の実装（§3.2） | コンパイル時検証と `Fetch` の戻り値 |
| AC-26 | 開始前キャンセル・実行中キャンセル（§3.3・§6.1） | キャンセル済みの `ctx` と fake の実行中キャンセル |
| AC-27 | 外部を呼ばないテスト（§7.1） | fake executor と `testdata/` |
| AC-28 | タイムアウト 0 以下の拒否（§3.2） | 構築のエラー |
| AC-29 | 一部だけのキャッシュ（§3.5・§4.3） | 片方だけを置いたキャッシュ |
| AC-30 | 残存ファイルの誤認防止（§3.5） | 字幕を出力しない成功ケース |
| AC-31 | 採用する字幕ファイル（§3.7） | `ja.json3` と `ja-orig.json3` を置いたキャッシュ |
| AC-32・AC-46 | 重複の扱い（§3.7） | 実データで本文を持つイベントとセグメントが 1 対 1 に対応し、行区切りのイベントがセグメントを作らないこと、および断片と完全な行の組・時間的に離れた繰り返しが除去されずに残ること |
| AC-33 | 手動実行の完了条件（§7.2） | 03_implementation_plan.md に記録する（実装計画側） |
| AC-34 | ポインタの置き換え（§3.5・§6.3） | 強制再取得の成功と、旧スロットが残らないこと |
| AC-35 | 削除後の再取得（§3.5 のキャッシュの無効化） | ポインタまたは有効なスロットのファイルを削除した後の `Fetch` |
| AC-36 | `ParseError` のパスと番兵（§3.4・§4.1） | `errors.AsType` / `errors.Is` |
| AC-37〜AC-43 | 統合テスト（§7.2） | `make test-integration`・lint の解析対象 |
| AC-44・AC-66 | 環境 allowlist（§3.7・§5.2） | fake executor が観測した環境 |
| AC-45・AC-54 | `tStartMs` の必須・範囲（§3.4） | 欠落・負数・非整数・範囲外の入力 |
| AC-47 | info.json の `id` 一致（§3.4） | 不一致の入力 |
| AC-48・AC-49 | 字幕の上限（§3.7） | 上限ちょうど・超過の入力 |
| AC-50 | info.json の上限（§3.7） | 上限ちょうど・超過の入力 |
| AC-51・AC-56 | 標準エラー出力の捕捉（§3.7・§5.2） | fake executor が書く大量の出力 |
| AC-52・AC-53 | UTF-8・サロゲートの拒否（§3.4） | 不正バイト列のサンプル |
| AC-55・AC-57・AC-67 | 配列要素・入れ子・未知メンバー（§3.4） | `null` 要素・非オブジェクト要素・未知メンバーを含む入力 |
| AC-58・AC-59 | 失敗時のキャッシュ保護、固定名だけに触れる規則、dangling なエントリの削除（§3.5・§6.3） | fake による各失敗、固定名に置いた種別の異なるエントリ、§6.3 の各状態の直接作成とファイルの比較 |
| AC-60・AC-61 | info.json 不在と字幕なしの優先（§4.3・§6.2） | 出力なし・info.json なしのケース |
| AC-62 | 起動・待機失敗の番兵（§4.1） | fake が権限拒否などを返すケース |
| AC-63 | 読み取り不能な字幕ファイル（§3.4・§4.3） | ディレクトリをパスに置くケース |
| AC-64・AC-65 | 未消費メンバーと消費フィールドの型（§3.4） | 実データと型を変えた入力 |
| AC-68 | 検証順序（§6.2） | 複合失敗のケース |
| AC-69 | 中断時の残留と `Fetch` 時の削除（§3.5・§6.3） | §6.3 の各状態を置いた後の各 `Fetch` |
| AC-70・AC-71 | 掃除（§3.5・§6.4） | dangling・有効なキャッシュ・無関係なエントリ・種別の異なるエントリを置いた掃除 |
| AC-72 | 掃除の失敗の集約（§3.5・§4.3） | 削除できないエントリを含む掃除 |
| AC-73・AC-74 | キャッシュの削除（§3.5） | 成功後・キャッシュなし・不正な URL での削除 |
| AC-75 | ポインタを先にする削除の順序（§3.5・§6.3） | パーミッションで起こすポインタ・スロットの削除の失敗 |

テストの実装上の詳細（テスト関数名、ファイル内の位置）は `03_implementation_plan.md` で定める。

---

## 8. 実装優先順位 (Implementation Priorities)

1. **フェーズ 1: 純粋な処理** — `errors.go`・`video_id.go`・`json3.go`・`info.go` と、`testdata/` を使うテスト。外部コマンドに依存しない。
2. **フェーズ 2: 外部コマンドの境界** — `exec.go`（`commandExecutor`・`cappedWriter`・allowlist 環境）と fake を使うテスト。
3. **フェーズ 3: キャッシュと `Fetch`** — `cache.go`・`ytdlp.go`（`RemoveCache`・`PruneCache` を含む）と、fake executor を使う `Fetch`・キャッシュの削除・掃除のテスト。
4. **フェーズ 4: 統合テストと lint 経路** — `integration_test.go`、`Makefile`・pre-commit・CI の lint ビルドタグ。
5. **フェーズ 5: ドキュメント** — `package_reference.md`・`requirements_process.md` の更新。

各フェーズで `make fmt && make test && make lint` を通す。各パッケージを追加・変更するコミットで `package_reference.md` を更新する。

---

## 9. 将来の拡張性 (Future Extensibility)

- **他の字幕言語・翻訳**: `-o`・ファイル名の言語部分を設定にすれば、同じ構造で別言語へ広げられる。#6 の設定追加で対応する。
- **手動・自動字幕の選択**: 現設計は読み込みファイルを `<id>.ja.json3` の 1 つに固定する。手動トラックが実質空の場合に `ja-orig` の自動字幕へ切り替える拡張を、必要になった時点で追加する（§1.4・付録A）。
- **ローリング重複への対応**: §3.7 のとおり、実データでは重複を観測しておらず、仮に含まれても後段の LLM による記事化で吸収されると見込む。将来の出力形式で観測され、成果物に影響する場合は、`dDurationMs` など規則が読むフィールドの検証（範囲・オーバーフロー対策を含む）とあわせて除去規則を追加する。
- **長い動画（#13）**: チャンク分割はスコープ外。上限制御により、想定外に大きい入力は安全に拒否できる。
- **音声文字起こし（#12）**: 字幕がない動画へのフォールバックはスコープ外。`ErrNoSubtitles` を判別できるため、別段階として追加できる。
- **YAGNI**: リトライ・並列取得・メモリキャッシュは行わない。必要になった時点で追加する。

---

## 付録A: 決定履歴 (Decision History)

- **リポジトリに保存する実データを CC BY の動画に限定した。** 通常の YouTube 標準ライセンスの動画の全文字起こしは著作物であり、公開リポジトリへの保存は適切でない。フィクスチャは CC BY の動画からの出力に限り、出典とライセンスを `testdata/README.md` に明記する（§1.4）。
- **フィクスチャは実出力のまま保存する（サイズの判断）。** ただし info.json のうち取得者の IP アドレスを含む URL 文字列だけは置き換える（`testdata/README.md`）。生は 1.80 MB（json3 338 KB・info.json 1.46 MB）だが、git は blob を圧縮するため clone で増えるパックは約 68 KB（`gzip -9` 実測: json3 31.8 KB・info.json 35.8 KB）であり、GitHub の上限（1 ファイル 100 MB、リポジトリ推奨 1 GB）に対しても十分小さい。info.json の大半は未使用メンバーだが、上記の置き換えを除いて実出力のまま保持してパーサテストの忠実性を優先する。将来、更新で履歴が膨らむ場合は gzip 保存や未使用メンバーの除去を検討する（§1.4）。
- **読み込む字幕ファイルを `<id>.ja.json3` に固定した。** §1.4 の調査で、`--sub-langs ja` が `ja.json3` だけを出力し、手動字幕と自動字幕の両方がある場合は手動字幕が `ja.json3` に書かれることを確認した。両方を含むキャッシュでは `ja.json3` を採用し、`ja-orig.json3` などは読まない（AC-31）。
- **手動トラックが実質空の場合を既知の制限とした。** 調査した CC BY 動画の手動トラックは空白だけのイベントが大半で、本文が 2 行しかなかった。本スコープでは自動字幕へのフォールバックを設けない。将来、`ja-orig` を追加で取得して本文を持つ方へ切り替える拡張を検討する（§1.4・§9）。
- **ローリング重複の除去を行わない（要件 F-003・AC-32 もあわせて改訂）。** 実データに重複が無く、文言の包含と時刻の重なりによる素朴な除去規則を適用すると実際の本文を含む行まで削除された。仮に重複が含まれても、後段の LLM による記事化で吸収され、最終的な成果物には影響しないと見込む。当初の設計は承認済みの AC-32（重複の除去）を「パーサが重複を作らないこと」と読み替えていたが、要件の意味を設計書の側で変えることになるため、要件のほうを改めた。将来の出力形式で重複が観測された場合、読むフィールドの検証とオーバーフロー対策をあわせて規則を追加する。
- **上限値を 8 MiB / 65,536 件 / 8 MiB とした。** 保存したフィクスチャ（338 KB・704 件・1.46 MB）と、別の約 56 分の動画の観測（0.9 MB・1,984 件・1.1 MB）に対する余裕から決めた。
- **キャッシュディレクトリを必須とした。** 未指定を許す既定ディレクトリは、利用者の環境に依存するため設けない。`#6` が設定から渡す。
- **キャッシュの置き換えをポインタのリネーム 1 回で行い、ロールバックを持たない。** キャッシュを 2 つの固定名のスロットとポインタで構成し、有効でない側のスロットへ出力して検証・同期した後、ポインタを一時ファイルからのリネームで置き換える（§3.5）。コミット点がリネーム 1 回なので、その前の失敗・中断では既存のキャッシュが変わらず、元へ戻す操作が要らない。中断で残ったエントリは dangling なエントリとして、`Fetch` 時と掃除で削除する。データを先に書き、1 つのファイルの atomic な置き換えを確定点とし、中断後は再実行で収束させる方式は、tlsrpt-digest の `.eml` と index の保存に倣った。
  - **退避とロールバックによる方式を採らなかった。** 当初は、2 つの正規ファイルを退避名へリネームしてから新しいファイルを設置し、失敗時はロールバックする方式だった。2 つのファイルを 1 回のリネームで置き換えられないため、新旧の混在を防ぐ順序の不変条件と、ロールバック自体の失敗の扱いが必要になり、手順とテストが複雑になった。
  - **2 つのファイルを 1 つにまとめる方式を採らなかった。** コミットは 1 回のリネームで済むが、要件が字幕ファイルと info.json の 2 ファイルを前提としており（AC-29 の「一部だけ存在」など）、要件の変更が必要になる。
  - **スロットを一意名ではなく固定名にした。** 一意名（`os.MkdirTemp`）のスロットでは、ポインタの内容が任意の名前になり、ディレクトリ内の文字列を信用しない規則（AC-20）に反する。固定名の 2 スロットなら、ポインタの内容は `a`・`b` の 2 値だけを受理すればよく、掃除も命名規則で dangling を判定できる。同じ動画の同時実行を許さない前提（単一ライター）の下では、固定名で衝突しない。
- **キャッシュは投稿の成功後に既定で削除する（呼び出しは #6）。** 処理を終えた動画の文字起こしをローカルに残し続けないためである。キャッシュの目的は、同じ動画の処理のやり直し（記事生成・投稿の失敗後の再実行、プロンプトの調整）で再取得しないことに絞り、プロンプトの調整などで残したい場合は #6 のオプションで残す。`TranscriptSource` からは投稿の成否が見えないため、削除は `YtDlpSource` の具体型のメソッド `RemoveCache` として提供し、`TranscriptSource` interface とパイプラインは変更しない（§1.1 の原則 6）。削除はポインタを先に消し、無効化の点を 1 回の削除に固定する（§3.5）。
- **dangling なエントリを `Fetch` 時と明示的な掃除の 2 つで削除する。** `Fetch` 時の削除は当該動画の固定名だけを見て、ディレクトリを列挙しない。二度と `Fetch` されない動画のために、列挙を伴う掃除を `PruneCache` として分け、#6 が CLI の各実行で呼ぶ。キャッシュの削除が既定で行われるため、残る dangling なエントリは中断の分だけで少なく、各実行での列挙のコストは小さい。掃除と `Fetch` の排他は、単一ライターの前提をキャッシュディレクトリ全体に広げて呼び出し側に任せ、ロックファイルや経過時間のしきい値は設けない（YAGNI）。
- **キャッシュのエントリを固定名で識別する。** 本実装が触れるのは、スロット 2 つ・ポインタ・ポインタの一時ファイルの固定名だけである（置き換え前の方式では、正規ファイル 2 つ・退避ファイル 2 つ・ステージング領域 1 つだった）。当初は `<動画 ID>.` 接頭辞で当該動画のエントリを識別して残存エントリを削除する案だったが、利用者が置いた `<動画 ID>.notes` などの無関係なファイルまで削除し AC-59 に反するため、採らなかった。スロットには 1 回の出力だけが入り、旧スロットは丸ごと削除されるため、前回だけ存在したファイルを探して削除する処理も不要である（AC-34、§3.5）。
- **`yt-dlp` の起動引数を 1 つの配列に固定した。** F-002 が設計に委ねたオプション列を §3.7 に固定し、§5.2 とテスト（AC-06）はこれを参照する。
- **`-P` の値はテンプレートとして展開されないことを実測した。** `%(title)s` と `%%` を含むディレクトリ名で `yt-dlp` を実行し、文字どおりのディレクトリへ出力されることを確認した（§1.4）。
- **`requirements_process.md` の文言を整合させる。** 「標準ライブラリが黙って無視する入力も拒否する」というチェックは、要件が拡張可能と宣言する未消費メンバーの受理と矛盾するため、消費するフィールドに限定する（design_handoff H-13）。
