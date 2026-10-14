# 実装計画書：コピーした字幕のパーサと TranscriptSource

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-14 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。番号だけの「§N」は、断りがない限り architecture の節を指し、本計画自身の節は「本計画 §N」と書く。既存のファイルの行番号は、断りがない限り HEAD `d8f163e`（ブランチ `claude/task-dphh91`）で確かめた。Go のソースとテストデータは、`4fe06e0`（`main`）から変わっていない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements の F-001〜F-004 を、architecture の設計どおりに実装する。`internal/transcript` に、コピーした字幕のパーサ（architecture §3.2・§3.3）と、`TranscriptSource` を実装する `CopiedSource`（§3.1）を加える。`testdata/` の実データ 3 ファイルで解析を確かめ、CLAUDE.md などの文書に例外とパッケージの説明を加える。

本計画のフェーズ 1〜5 は、architecture §8 の実装優先順位のフェーズ 1〜5 と同じ名前・同じ順序である。用語（コピーした字幕、形 A、形 B、時刻の行、本文の行、読み上げ用ラベル、パーサ、source）は architecture の冒頭の用語に従う。

### 1.2. 実装原則

- architecture §1.1 の設計原則 6 項目に従う。特に次の 4 点を守る。書式に合わない入力を補正せず拒否する（原則 1）。読み上げ用ラベルを時刻から導いて照合する（原則 2）。解析を `NewCopiedSource` で 1 回だけ行う（原則 3）。エラーに入力の文字列を含めない（原則 5）。
- 受理する形の唯一の定義は architecture §3.2、拒否の理由と検査の順序は §4.1・§4.3 である。本計画はそれらを書き写さず、節番号で参照する。
- Go の標準ライブラリだけを使う。新しい外部モジュールを追加しない（requirements §4.4）。
- Go のコメント・識別子・文字列リテラルは英語で書く。**例外:** 日本語 UI の読み上げ用ラベルの単位（`秒`・`分`・`時間`）は、パーサが照合する YouTube の書式そのものなので、`copied_format.go` に日本語の文字列リテラルとして置く。宣言は 1 か所にまとめ、その理由を英語の 1 行のコメントで記す（ステップ 1-3）。テストの入力に使う日本語の字幕の文字列も、入力データとしてだけ使う。
- `AC-NN`・`F-NNN` は Go のソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- 各テストが、その名目の理由で失敗しうることは、実装時に対象の分岐を壊して確かめ、コミットメッセージに記す（CLAUDE.md「Testing Strategy」）。本計画は、壊す箇所をタスクとして挙げるだけで、結果を先に主張しない。
- 各フェーズの終わりに、`make fmt` → `make test` → `make lint` を通す（`_context.md` の build checks）。

### 1.3. 既存コード調査結果

- **`internal/transcript/transcript.go`（変更しない）。** `Segment`（`:7-11`）、`Transcript`（`:13-21`）、`TranscriptSource`（`:23-28`。「空の `Transcript` を成功として返さない」は `:24-25`）。`CopiedSource` はこの interface を実装する。
- **`internal/transcript/errors.go`（フェーズ 1 で変える）。** 公開の番兵の `var` の並びは `:10-16`（`ErrInvalidVideoURL`・`ErrYtDlpExec`・`ErrParseSubtitles`・`ErrParseInfo`・`ErrNoSubtitles`）。`ErrCopiedTranscriptFormat` をこの並びに加える（architecture §3.1）。
- **`internal/transcript/video_id.go`（変更しない）。** `NormalizedVideoURL`（`:62-69`）を `CopiedInput.VideoID` の検査と正規化 URL の導出に使う。
- **`internal/transcript/ytdlp.go`（変更しない）。** `YtDlpSource.Fetch` は冒頭で `ctx.Err()` を確かめる（`:54-57`）。`CopiedSource.Fetch` も同じ形にする（architecture §3.1 の表）。
- **`internal/transcript/json3.go`（変更しない）。** 非公開の番兵で理由を分ける形（`:20-28`）をパーサでも使う。処理の共有はない（architecture §1.3）。
- **`internal/transcript/test_helpers.go`（フェーズ 3 で変える）。** `//go:build test` 付きで、`testdata/` のファイルのパスの定数を `:15-24` に並べ、`readTestdataFile`（`:231`）を持つ。実データ 3 ファイルの glob のパターンをこの並びに加え、ファイルの読み込みには `readTestdataFile` を使う。ほかのテストヘルパーは追加しない。
- **`internal/transcript` のテストファイル。** ユニットテストのファイル（`cache_test.go`・`json3_test.go` など）の 1 行目は `//go:build test` である（`integration_test.go:1` だけは `//go:build integration`）。新しいテストファイルは `//go:build test` にする。
- **`internal/writer/prompt.go`・`prompt_test.go`（`prompt_test.go` をフェーズ 3 で変える）。** `validateTranscript`（`prompt.go:50-82`）は、動画 ID、`VideoURL` の一致、`Segments` が空でないこと、各 `Text` が空や空白だけでないこと（`:61-65`）、メタデータと `Text` の UTF-8（`:71-80`）を確かめる。`prompt_test.go` は `//go:build test` の `package writer` で、`transcript` を import 済み（`:12`）。`TestWriteRejectsInvalidTranscript`（`:36`）は拒否の側を確かめており、`CopiedSource` の出力が受理されることを確かめるテストはない。`internal/transcript` の `test_helpers.go` の定数は非公開なので、`writer` のテストは glob のパターンを自分で持つ。
- **`internal/pipeline/pipeline_test.go`（変更しない）。** `TestCommonTypesFieldSets`（`:378`）は `Transcript`・`Segment` を変えないので通る。`TestFakesCarryBuildTag`（`:534`）は、`testutil/` と `test_helpers*.go` のファイルを増やさないので変わらない。`TestPackageReferenceListsPackages`（`:610`）は、パッケージを増やさないので行の追加を求めない。
- **`cmd/yt2column` の文書の guard（参照のみ）。** 部品 `readDoc`（`docs_test.go:304`）、`findTable`（`:368`）、`tableRow`（`:384`）、`sectionByHeading`（`:455`）、`extension_docs_test.go` の `sectionByHeadingPrefix`（`:347`）と、パスの定数 `projectOverviewPath`（`docs_test.go:22`）・`claudePath`（`extension_docs_test.go:22`）を再利用する。見出しの節を取り出す部品はあるが、CLAUDE.md の箇条書きの 1 項（`- **<名前>**:` で始まる項）を取り出す部品はない。これらの部品のテストには `TestSectionByHeading`（`docs_test.go:237`）・`TestSectionByHeadingPrefix`（`extension_docs_test.go:196`）がある。`cmd/yt2column` は CI の `doc-guard` ジョブの対象なので（CLAUDE.md「Development Notes」）、文書だけの変更でもここの guard が走る。
- **`testdata/`（変更しない）。** 実データ 3 ファイル、`testdata/README.md` の記載（`:41-70`）、`.gitattributes`（`testdata/*.transcript_copy.txt -text`）は、コミット `4fe06e0` で追加済みである。`.pre-commit-config.yaml:40-46` は `testdata/` を整形の対象から外している。テストの期待値と照らす独立の実測値は次のとおり（HEAD `d8f163e`）。

    | ファイル | バイト数（`wc -c`） | 行数（`awk 'END{print NR}'`） | 行全体が時刻の行の数（`grep -cE '^([1-9]?[0-9]:)?[0-5]?[0-9]:[0-5][0-9]$'`） | 本文のない時刻の行（`grep -n` で確認） |
    |---|---|---|---|---|
    | `IkngpxCMmRQ.transcript_copy.txt`（形 A） | 4786 | 40 | 0 | - |
    | `Yoboo9g7saY.transcript_copy.txt`（形 B） | 9015 | 370 | 185 | なし |
    | `mbyG85GZ0PI.transcript_copy.txt`（形 B） | 77824 | 1398 | 701 | 4（`51:07` が 889 行目、`52:06` が 906 行目、`52:13` が 907 行目、`1:18:57` が 1358 行目） |

    `mbyG85GZ0PI` の `1:00:07` は 1038 行目である。これらから、本文のあるセグメントの数は、`IkngpxCMmRQ` が 40、`Yoboo9g7saY` が 185、`mbyG85GZ0PI` が 697（701 − 4。行数 1398 = 701 + 697 と合う）と見込む。
- **文書（フェーズ 5 で変える）。** CLAUDE.md の Architecture Overview の `TranscriptSource` の項は `:92-93`、「Delegate the fragile part」は `:128-129`、「Declare, don't infer」は `:132-138`。`docs/dev/project_overview.md` の字幕取得の方針は `:11`、パイプラインの `TranscriptSource` の実装は `:26`、想定ディレクトリ構成の `internal/transcript/` の注記は `:76`（architecture §3.4 の責務表にある）。`docs/dev/developer_guide/package_reference.md` の `internal/transcript` の行は `:22`（`internal/transcript/testutil` の行 `:38` は、`testutil/` を変えないので変えない）。`docs/translation_glossary.md` の用語の表に、architecture §3.4 の 6 語はまだない。
- **lint。** `make lint` は `golangci-lint run --build-tags test,integration`（`Makefile:10`）で、`//go:build test` のテストも対象になる。テストからの使用も使用として数えるので、フェーズ 1 のパーサがフェーズ 2 まで本番のコードから呼ばれなくても `unused` には当たらない。`.golangci.yml` は `mnd`（`:38`）、`gocyclo`（`:16`。`min-complexity: 20` は `:43`）、`err113`（`:34`）を有効にしており、本番のコードの `60`・`1000` などの数値、1 つの関数に詰めたパーサ、関数の中の `errors.New` が指摘されうる。数値は名前つきの定数に、パーサは形 A・形 B・入力全体の検査などの小さな関数に分け、エラーはすべてパッケージの番兵にする。
- **`make deadcode`。** `deadcode -test -tags test $(MAIN_PKG)`（`Makefile:214`）の起点は `cmd/yt2column` とそのテストだけである。#109（サーバのタスク。本計画 §9 を参照）がまだないので、`NewCopiedSource`・`Fetch`・パーサは到達不能として報告される。これらは #109 が使う、意図した公開の型と関数であり、`make deadcode` の報告を理由に削除しない（`/runplan` の手順で到達不能な関数の削除が求められても、本タスクの部品は除く）。
- **外部前提の確認。** fuzz のテストには Go 標準の `testing.F` を使う。`go test` は、fuzz のターゲットの種（`f.Add` と `testdata/fuzz/<Target>/`）を通常のテストとして実行する。fuzz で失敗が見つかると、その入力は `testdata/fuzz/<Target>/` に書き出され、以後は種になる（`testing` パッケージの文書の Fuzzing の節。`go help testflag` の `-fuzz` の説明が参照する）。`-fuzz` は 1 つのパッケージと 1 つの fuzz のターゲットにだけ使える（`go help testflag` の `-fuzz` の説明）。`internal/transcript/testdata/fuzz/` は `.gitignore` の対象ではない（`git check-ignore` で確認）。メモリの計測は `runtime.ReadMemStats` の `TotalAlloc`（割り当ての累計で、GC で減らない。`runtime` パッケージの文書）を使う。`make test` は `-race` 付き（`Makefile` の `test:`）なので、時間の上限は `-race` 付きの実行で満たす必要がある。
- **意図して変えないもの。** `Transcript`・`Segment`・`TranscriptSource`、`ValidateVideoURL`・`NormalizedVideoURL`、`YtDlpSource`、json3・info.json のパーサ、`internal/writer` の本番のコード、`internal/pipeline`、`testdata/` の実データと `testdata/README.md`、`.gitattributes`、拡張（`extension/`）。

### 1.4. 名前の変更の一覧

本タスクはシンボル・ファイル・テストの名前を変更しない。新しい公開の名前（`CopiedInput`・`CopiedSource`・`NewCopiedSource`・`ErrCopiedTranscriptFormat`）と非公開の番兵（`errCopied` で始まる名前）は、リポジトリの Go のソースにまだない（`rg -n "Copied" -g '*.go' -g '!extension/**'` の出力は該当なし。HEAD `d8f163e` で実行）。

### 1.5. 申し送りへの対応

このタスクのディレクトリに `design_handoff.md` と `implementation_handoff.md` はない。architecture §9 が #109・#108・#111 に求める事項は、本タスクの範囲外なので、本計画では扱わない。それらは本計画 §9「次のステップ」に記す。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 番兵、時刻の文法、読み上げ用ラベル、パーサ（F-002）

**対象ファイル:** `internal/transcript/errors.go`（変更）、`internal/transcript/copied_format.go`（新規）、`internal/transcript/copied_format_test.go`（新規）

- [ ] **ステップ 1-1**: `errors.go` の公開の番兵の並びに `ErrCopiedTranscriptFormat` を加える（文面と doc comment は architecture §3.1）。
- [ ] **ステップ 1-2**: `copied_format.go` に、拒否の理由の非公開の番兵（architecture §4.1 の 11 個）と、エラーを組み立てる関数を置く。エラーは `ErrCopiedTranscriptFormat` と理由の番兵の両方を `errors.Is` で判定でき、文面は architecture §4.2 の形（固定の文と行番号だけ）にする。
- [ ] **ステップ 1-3**: 時刻の文法（architecture §3.2.2）と、時刻から日本語 UI・英語 UI の読み上げ用ラベルを導く関数（§3.2.3）を `copied_format.go` に書く。日本語の単位の文字列リテラルは本計画 §1.2 の例外として 1 か所で宣言する。
- [ ] **ステップ 1-4**: パーサを `copied_format.go` に書く。入力全体の検査（§3.2.1）、形の判定（§3.2.5）、形 A の行（§3.2.4。ラベルの言語は最初の行で固定）、形 B の行の分類（§3.2.4・§6.2）、時刻の順序と本文のあるセグメントの有無（§3.2.6）、`Segments` の組み立て（§3.2.7）を、§4.3 の順で検査する。拒否したときは部分的な結果を返さない。行番号を求めるために入力の先頭から数え直さない（§3.3）。
- [ ] **ステップ 1-5**: `copied_format_test.go` に、ラベルの導出の表のテスト `TestCopiedLabels` を書く（`0:00`、単位の省略、英語の単数・複数、時を含む形）。
- [ ] **ステップ 1-6**: `copied_format_test.go` に、受理のテスト `TestParseCopiedAccepts` を書く。architecture §7.1 の「英語 UI」「時刻とラベルの境界」「本文のないセグメント」「観測していないが受理する形」の各項と、requirements AC-04a・AC-04b の例を含める。時刻の文法の受理の境界（`0:00`・`59:59`・`1:00:00`・`99:59:59`）と、形 B の本文の行の位置にある `05:00`・`100:00:00`・`1:30 から始めます` の受理（§3.2.4・§4.1 の表の後の注記）を含める。
- [ ] **ステップ 1-7**: `copied_format_test.go` に、拒否のテスト `TestParseCopiedRejects` を書く。architecture §4.1 の表の各行について、その理由だけに当たる入力で、`ErrCopiedTranscriptFormat` と理由の番兵の両方を `errors.Is` で確かめ、報告する行番号を確かめる（§7.1「拒否」の注意: ほかの規則に当たらない入力を作る）。時刻の位置での文法の誤りは、§3.2.2 の例（`05:00`・`60:00`・`0:5`・`0:01:00`・`100:00:00`）と §3.2.4 の `m:ss` の直後が `:` の例（`0:12:34`・`1:23:4`）をすべて含める。`errCopiedBlank` は、ASCII の空白だけの入力に加え、U+3000・NBSP だけの入力も含める。§7.1 が挙げる追加の入力（先頭の表示項目だけ、チャプターの見出し、先頭の BOM、形の混在の両方の向き、別の言語のラベル）も含める。
- [ ] **ステップ 1-8**: `copied_format_test.go` に、行の区切りのテスト `TestParseCopiedLineBreaks` を書く（requirements AC-07a）。同じ内容の LF と CRLF の入力、末尾の区切り 0 個と 1 個の入力が、それぞれ同じ `Segments` になること。LF と CRLF の混在、単独の CR、行末の CR、末尾の区切り 2 個・3 個を拒否し、報告する行番号が architecture §3.2.1・§4.2 のとおりであること。
- [ ] **ステップ 1-9**: `copied_format_test.go` に、検査の順序のテスト `TestParseCopiedCheckOrder` を書く。2 つの規則に同時に当たる入力で、architecture §4.3 の順で先に来る理由が返ることを確かめる（入力全体の検査どうし、入力全体の検査と行ごとの検査、1 行の中の検査のそれぞれから 1 組以上）。
- [ ] **ステップ 1-10**: `copied_format_test.go` に、エラーの文面のテスト `TestParseCopiedErrorOmitsInput` を書く。拒否する入力に目印の文字列（本文・時刻・ラベル）を入れ、`Error()` に現れないことを確かめる（AC-06。メタデータの目印はステップ 2-5）。
- [ ] **ステップ 1-11**: ステップ 1-5〜1-10 の各テストが、その理由で失敗しうることを確かめる。少なくとも、(a) ラベルの照合を外す（時刻の後の任意の数字をラベルとして読み飛ばす）、(b) ラベルの言語の固定を外す、(c) 形 B の本文の行の混在の検査を外す、(d) 時刻の順序の検査を外す、(e) 末尾の区切りを 2 個まで受理する、(f) 検査の順序を 1 か所入れ替える、(g) エラーの文面に行の中身を加える、(h) 時刻の分の上限を 60 にする、(i) 空白の判定を ASCII の空白だけにする、(j) `h:mm:ss` の時をミリ秒に換算する係数を誤らせる、の各変更で、対応するテストが失敗することを確かめ、コミットメッセージに記す。

**完了条件:** パーサが architecture §3.2 の書式を受理し、§4.1 の各理由で拒否する。`make fmt`・`make test`・`make lint` が通る。

### フェーズ 2: `CopiedInput`・`CopiedSource`・`NewCopiedSource`・`Fetch`（F-003）

**対象ファイル:** `internal/transcript/copied.go`（新規）、`internal/transcript/copied_test.go`（新規）

- [ ] **ステップ 2-1**: `copied.go` に `CopiedInput`・`CopiedSource`・`NewCopiedSource`・`(*CopiedSource).Fetch` を、architecture §3.1 の宣言と表の性質のとおりに書く。`NewCopiedSource` は `VideoID` を先に検査し、次に `Raw` をフェーズ 1 のパーサで解析する。`CopiedSource` のフィールドは `transcript Transcript` だけにする。`Fetch` は §3.1 の表の (1)〜(3) の順に確かめる。(2) のゼロ値のエラーは非公開の番兵として宣言する（`err113` に適合させ、テストで `errors.Is` により判定できるようにするため）。エラーの文面は §4.2。
- [ ] **ステップ 2-2**: `copied_test.go` に、構築のテスト `TestNewCopiedSource` を書く。書式に合う `Raw` で成功すること、書式に合わない `Raw` で `ErrCopiedTranscriptFormat` を包むエラーと nil の source を返すこと、`NormalizedVideoURL` が拒否する `VideoID`（空、10 文字、`/` を含むもの）で `ErrInvalidVideoURL` を包み `ErrCopiedTranscriptFormat` を包まないエラーを返すこと、両方が誤っている入力で `ErrInvalidVideoURL` の側になること（§3.1 の表の検査の順序）。`*CopiedSource` が `TranscriptSource` を満たすことを、コンパイル時の代入でテストに置く。
- [ ] **ステップ 2-3**: `copied_test.go` に、返す `Transcript` のテスト `TestCopiedSourceTranscript` を書く。`VideoID` が入力の値、`VideoURL` が `NormalizedVideoURL(VideoID)` の値であること。`Title`・`ChannelName`・`Description` が入力の値のままであること（空の値、前後に空白のある値、無効な UTF-8 を含む値を含める）。`Title` などに別の動画の URL を入れても `VideoURL` が導出した値であること。`Segments` が、同じ `Raw` をフェーズ 1 のパーサに直接渡した結果と一致すること。
- [ ] **ステップ 2-4**: `copied_test.go` に、`Fetch` のテスト `TestCopiedSourceFetch` を書く。一致する URL でだけ返すこと。空文字列、別の動画の正規化 URL、`youtu.be` の形、`http` の形、余分なクエリ付きの形を `ErrInvalidVideoURL` を包むエラーで拒否し、エラーに `videoURL` の値を含めないこと。取り消された `ctx` で `context.Canceled` を返すこと（URL が一致しない場合も `ctx` の誤りが先に返ること）。ゼロ値の `CopiedSource` の `Fetch(ctx, "")` が、ステップ 2-1 のゼロ値の番兵を包み、`ErrInvalidVideoURL` を包まないエラーを返すこと（§7.1「ゼロ値」）。
- [ ] **ステップ 2-5**: `copied_test.go` に、解析をやり直さないことのテスト `TestCopiedSourceHoldsOnlyTranscript`（`reflect` で `CopiedSource` のフィールドが `Transcript` 型の 1 つだけであること、2 回の `Fetch` が同じ値を返すこと）と、エラーの文面のテスト `TestNewCopiedSourceErrorOmitsInput`（拒否のエラーの文面に、入力のメタデータと `VideoID` の目印が現れないこと。AC-06）を書く。
- [ ] **ステップ 2-6**: ステップ 2-2〜2-5 の各テストが、その理由で失敗しうることを確かめる。少なくとも、(a) 構築の検査の順序を入れ替える、(b) `VideoURL` を `Title` から取る、(c) `Fetch` の URL の照合を外す、(d) `Fetch` の `Segments` が空のときの検査を外す、(e) `Fetch` の `ctx.Err()` の確認を外す、または URL の照合の後に移す、(f) `CopiedSource` に `Raw` のフィールドを足す、(g) メタデータを `TrimSpace` する、(h) 動画 ID の誤りのエラーに `VideoID` を含める、の各変更で、対応するテストが失敗することを確かめ、コミットメッセージに記す。
- [ ] **ステップ 2-7**: `make deadcode` が `NewCopiedSource` などを報告しても削除しないことを、コミットメッセージに記す（本計画 §1.3「`make deadcode`」）。

**完了条件:** `CopiedSource` が `transcript.TranscriptSource` を満たし、architecture §3.1 の性質を満たす。`make fmt`・`make test`・`make lint` が通る。

### フェーズ 3: 実データのテスト、`writer` との整合のテスト、調査記録の guard（F-001・F-002・F-003）

**対象ファイル:** `internal/transcript/test_helpers.go`（変更）、`internal/transcript/copied_test.go`（変更）、`internal/writer/prompt_test.go`（変更）、`cmd/yt2column/copied_transcript_docs_test.go`（新規）

`testdata/` の 3 ファイル、`testdata/README.md`、`.gitattributes` はコミット `4fe06e0` で追加済みなので、変更しない（本計画 §1.3）。

- [ ] **ステップ 3-1**: `test_helpers.go` のパスの定数の並びに、`testdata/*.transcript_copy.txt` の glob のパターンを加える。
- [ ] **ステップ 3-2**: `copied_test.go` に、実データのテスト `TestCopiedSourceRealSamples` を書く（architecture §7.1「実データ」）。glob で見つけたファイルの集合が、テストが期待値を持つ 3 つの動画 ID の集合とちょうど一致することを確かめる（期待値のないファイルが加わったら失敗する）。各ファイルについて、`readTestdataFile` で読み、次を確かめる。期待値は、本計画 §1.3 の実測値と、ファイルを目で読んで書いた定数から決める。パーサの出力からは作らない。
    - バイト数と、`NewCopiedSource` での解析の成功。
    - セグメント数（本計画 §1.3 の見込みの 40・185・697）。
    - 先頭と末尾のセグメントの `StartMs`・`Text`、§7.1 が挙げる特徴的な箇所（`IkngpxCMmRQ` の `1:00`、`mbyG85GZ0PI` の `1:00:07` と、本文のない 4 つの時刻が `Segments` に入らないこと）。
    - すべてのセグメントについての性質: `StartMs` が減らないこと。形 B では各 `Text` が入力中のいずれかの行と一致すること、形 A では各 `Text` が入力中のいずれかの行の末尾の部分であること（行の分割を、目で読んだ定数とは別の方法で確かめる）。
- [ ] **ステップ 3-3**: `prompt_test.go` に、`TestValidateTranscriptAcceptsCopiedSamples` を書く。`../../testdata/*.transcript_copy.txt` の各ファイルについて、ファイル名の先頭の動画 ID を `VideoID` にして `transcript.NewCopiedSource` と `Fetch` で `Transcript` を得て、`validateTranscript` を通ることを確かめる（architecture §7.1「`writer` との整合」）。ファイルが 3 つ見つからない場合は失敗させる。`writer` の検査の条件を `internal/transcript` のテストに写さない。
- [ ] **ステップ 3-4**: `cmd/yt2column/copied_transcript_docs_test.go`（`//go:build test`、`package main`）に、調査記録の guard `TestCopiedTranscriptSurveyRecords` を書く。本計画 §1.3 の文書の guard の部品を再利用する。次を確かめる。
    - `testdata/*.transcript_copy.txt` の各ファイルについて、`testdata/README.md` のコピーした字幕の節が、そのファイル名、動画の URL、CC BY のライセンスを記載していること。同じ節が、取得日・取得方法・加工内容・取得者を特定できる情報を含まないことの確認の各項を持つこと（AC-01）。
    - architecture §1.4.1 の 1 つ目の表が、requirements AC-01b の列（動画 ID・字幕の言語・自動/手動・長さ・ライセンス・ブラウザ・表示言語）を持ち、各実データの動画 ID の行の `testdata/` の欄が保存を示すこと。日本語・英語と自動・手動の 4 つの組み合わせの行がそろうこと（AC-01b）。
    - architecture §1.4.1 の 2 つ目の表が、requirements AC-01b の観測した書式の列を持つこと（AC-01b）。
    - architecture §1.4.1 に `h:mm:ss` の観測（`2:10:26`・`1:00:07`）が記録されていること（AC-01a）。
    - architecture §1.4.2 が、パネル全体を選択できたこと、調べた最長の長さ、選択できない場合の手段として CLI（`yt-dlp` の経路）を記載していること（AC-03）。
- [ ] **ステップ 3-5**: ステップ 3-2〜3-4 の各テストが、その理由で失敗しうることを確かめる。少なくとも、(a) 期待値の 1 つ（末尾のセグメントの `StartMs`、セグメント数）を変える、(b) `testdata/` に期待値のない `*.transcript_copy.txt` を一時的に置く、(c) 本文のないセグメントを `Segments` に含めない処理を一時的に外す（このとき `TestValidateTranscriptAcceptsCopiedSamples` は `-run` で単独に実行する。`mbyG85GZ0PI` の 4 つの時刻が空の `Text` になり、`validateTranscript` が拒否する）、(d) `testdata/README.md` から 1 ファイルの URL と取得日を順に消す、(e) architecture §1.4.1 の表から列や行を 1 つ消す、§1.4.2 から CLI の記載を消す、の各変更で、対応するテストが失敗することを確かめ、コミットメッセージに記す。変更はすべて元に戻す。
- [ ] **ステップ 3-6**: `testdata/README.md` の記載が security.md §7（取得者を特定できる情報を含まない）を満たすことを、実データ 3 ファイルを目で見て確かめ、本計画 §5.2 に記録する。

**完了条件:** 実データ 3 ファイルが解析され、`writer` の検査を通る。調査記録の guard が通る。`make fmt`・`make test`・`make lint` と、`go test -tags test ./cmd/yt2column ./internal/pipeline`（CI の `doc-guard` ジョブと同じ）が通る。

### フェーズ 4: 4 MiB の入力、時間とメモリ、fuzz のテスト（F-002）

**対象ファイル:** `internal/transcript/copied_format_test.go`（変更）

fuzz のターゲットは `NewCopiedSource`（フェーズ 2）を呼ぶので、このフェーズはフェーズ 2 の後に行う。

- [ ] **ステップ 4-1**: テスト `TestParseCopiedLargeInputs` に、architecture §7.3 の合成の入力 (a)〜(c) を作る関数を書く。各入力は、リクエストの本文の上限（4 MiB、`4 << 20` バイト）以下で最大の大きさにする。このテストで、解析が成功することを確かめる（AC-08）。
- [ ] **ステップ 4-2**: 同じテストで、(a)〜(c) のそれぞれの解析が 5 秒未満で終わることを確かめる（architecture §7.3「時間」）。`make test` は `-race` 付きなので、その実行で満たすこと。
- [ ] **ステップ 4-3**: 同じテストで、(a)〜(c) のそれぞれについて、解析の前後の `runtime.MemStats.TotalAlloc` の差が、入力の長さの定数倍以下であることを確かめる（architecture §7.3「メモリ」）。テストは `t.Parallel` を呼ばない。定数は (a)〜(c) の実測の最大から余裕を持たせて決め、実測値と定数を本計画 §5.1 に記録する。
- [ ] **ステップ 4-4**: `FuzzNewCopiedSource` を書く。`VideoID` は固定の正しい値を使う。種は `testdata/` の 3 ファイルと、フェーズ 1 のテストの受理・拒否の入力の一部とする。architecture §7.3 の (1)〜(4) の性質を確かめる。`make test` では種だけが実行される。
- [ ] **ステップ 4-5**: `go test -tags test -run '^$' -fuzz '^FuzzNewCopiedSource$' -fuzztime 60s ./internal/transcript` を手元で 1 回実行し、結果（実行回数、見つかった失敗の有無）を本計画 §5.1 に記録する。失敗が見つかった場合は、パーサを直し、その入力を表のテストの行に加え、`internal/transcript/testdata/fuzz/` に書かれたファイルを削除する（コミットしない）。
- [ ] **ステップ 4-6**: ステップ 4-1〜4-4 の各テストが、その理由で失敗しうることを確かめる。少なくとも、(a) 行ごとに入力の先頭から行番号を数え直す実装に一時的に変えると、時間の検査が失敗すること、(b) 行ごとに入力の残り全体を写す（割り当てが入力の長さの 2 乗に比例する）実装に一時的に変えると、メモリの検査が失敗すること、(c) fuzz の性質 (2) を破る値（空白だけの `Text`）をパーサが返すように一時的に変えると、種の実行が失敗すること、を確かめ、コミットメッセージに記す。

**完了条件:** 4 MiB の 3 つの入力が時間とメモリの上限の中で解析される。fuzz の種が通り、60 秒の fuzz で失敗がない。`make fmt`・`make test`・`make lint` が通る。

### フェーズ 5: 文書（F-004）

**対象ファイル:** `CLAUDE.md`（変更）、`docs/dev/project_overview.md`（変更）、`docs/dev/developer_guide/package_reference.md`（変更）、`docs/translation_glossary.md`（変更）、`cmd/yt2column/copied_transcript_docs_test.go`（変更）

- [ ] **ステップ 5-1**: `translation_glossary.md` の用語の表に、「コピーした字幕」「形 A」「形 B」「時刻の行」「本文の行」「読み上げ用ラベル」の対訳を加える。ステップ 5-2〜5-6 の英語の記述は、この対訳を使う。
- [ ] **ステップ 5-2**: CLAUDE.md の「Declare, don't infer」の項（`:132-138`）に、architecture §3.6 の範囲・理由・境界を英語で加える（AC-14）。
- [ ] **ステップ 5-3**: CLAUDE.md の「Delegate the fragile part」の項（`:128-129`）に、architecture §1.3.1 の例外 2 の範囲（パネルの出力の書式への依存はパーサに限ること、`yt-dlp` の経路を代わりの手段として残すこと）を英語で加える。
- [ ] **ステップ 5-4**: CLAUDE.md の Architecture Overview の `TranscriptSource` の項（`:92-93`）に、`CopiedSource`（コピーした字幕を解析して `Transcript` にする実装）を加える。
- [ ] **ステップ 5-5**: `project_overview.md` の字幕取得の方針（`:11`）に例外 2 を、パイプラインの `TranscriptSource` の実装（`:26`）と想定ディレクトリ構成の注記（`:76`）に `CopiedSource` を、日本語で加える。
- [ ] **ステップ 5-6**: `package_reference.md` の `internal/transcript` の行（`:22`）に、`CopiedSource`・`NewCopiedSource`・`ErrCopiedTranscriptFormat` を英語で加える。
- [ ] **ステップ 5-7**: `copied_transcript_docs_test.go` に、CLAUDE.md の箇条書きの 1 項を、項の先頭の太字の名前で取り出す部品を書き、その部品の表のテストを置く（既存の `TestSectionByHeading` と同じ形）。
- [ ] **ステップ 5-8**: `copied_transcript_docs_test.go` に、guard `TestClaudeMDCopiedTranscriptException` を書く。ステップ 5-7 の部品で取り出した「Declare, don't infer」の項が、例外の範囲（コピーした字幕の書式の解析）・理由（呼び出し側が型で表せない外部の値）・境界（`internal/transcript` のパーサだけ、書式に合わない入力は拒否）を述べていることを確かめる（AC-14）。
- [ ] **ステップ 5-9**: 同じファイルに、guard `TestCopiedTranscriptDocsUpdated` を書く。次を確かめる。CLAUDE.md の「Delegate the fragile part」の項が、パーサに限る範囲と `yt-dlp` の経路を代わりの手段とすることを述べること。CLAUDE.md の Architecture Overview の `TranscriptSource` の項と、`project_overview.md`（`projectOverviewPath`）が `CopiedSource` を挙げること。`package_reference.md` の `internal/transcript` の行が `CopiedSource`・`NewCopiedSource`・`ErrCopiedTranscriptFormat` を挙げること。`translation_glossary.md` の用語の表に 6 語の行があること。
- [ ] **ステップ 5-10**: ステップ 5-7〜5-9 の部品と guard が、その理由で失敗しうることを確かめる。少なくとも、(a) CLAUDE.md の例外の文から範囲・理由・境界のどれか 1 つを消す、(b) 同じ語を項の外に移す、(c) `package_reference.md` から `NewCopiedSource` を消す、(d) 用語集から 1 語の行を消す、の各変更で失敗することを確かめ、コミットメッセージに記す。
- [ ] **ステップ 5-11**: ステップ 5-1〜5-6 の文面が architecture §1.3.1・§3.4・§3.6 の内容と一致することを、節ごとに読み合わせて確かめる（guard は語の有無だけを確かめるので、文意はこの読み合わせで確かめる）。

**完了条件:** 文書が architecture §3.4・§3.6 の内容を満たす。`make fmt`・`make test`・`make lint` と `go test -tags test ./cmd/yt2column ./internal/pipeline` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

| マイルストーン | 成果物 | 対応するフェーズ |
|---|---|---|
| M1: パーサが書式の定義どおりに受理・拒否する | `copied_format.go`・`errors.go` の番兵とそのテスト | フェーズ 1 |
| M2: #109 が使う公開の型と関数がそろう | `copied.go` とそのテスト | フェーズ 2 |
| M3: 実データで解析と `writer` との整合が確かめられ、調査記録が guard される | 実データのテスト、`writer` のテスト、調査記録の guard | フェーズ 3 |
| M4: 大きな入力と想定外の入力への耐性が確かめられる | 4 MiB のテスト、fuzz のテスト、本計画 §5.1 の記録 | フェーズ 4 |
| M5: 文書が新しい実装と例外を反映する | CLAUDE.md・`project_overview.md`・`package_reference.md`・`translation_glossary.md` と文書の guard | フェーズ 5 |

フェーズは 1 → 2 → 3 → 4 → 5 の順に行う。フェーズ 4 は architecture §8 ではフェーズ 1 だけに依存するが、fuzz のターゲットがフェーズ 2 の `NewCopiedSource` を呼ぶので、フェーズ 2 の後に行う。#109 はフェーズ 2 の公開の型と関数に依存する（architecture §8）。PR の境界は `/mkplan2` で決める。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

architecture §7.1 のとおり、すべて文字列の入力だけで行い、`yt-dlp`・ネットワーク・LLM に触れない。パーサのテストは `copied_format_test.go`、source のテストは `copied_test.go` に置く。

- 拒否のテストは、architecture §4.1 の表の各行を 1 つ以上の入力で確かめ、その入力がほかの規則に当たらないように作る（CLAUDE.md「A layered path needs inputs only one layer can handle」）。
- 時刻の文法は、パーサを通した受理・拒否のテストで確かめ、§3.2.2・§3.2.4 の境界の例をすべて含める（ステップ 1-6・1-7）。時刻の文法だけの表のテストは置かない（同じ行が重複するため。CLAUDE.md「Check for duplication before adding a table test for a helper」）。ラベルの導出だけは、時刻とラベルの対応を一覧できるように単独の表のテストを置く（ステップ 1-5）。
- エラーの判定は `errors.Is` だけで行い、文面の一致では判定しない。文面については、入力の目印が現れないことと行番号だけを確かめる。

### 4.2. 実データのテストと `writer` との整合

フェーズ 3 で、`testdata/` の実データ 3 ファイルを解析する。期待値は、目で読んだ定数と、本計画 §1.3 の独立の実測値から決める。パーサの出力からは作らない。先頭・末尾・特徴的な箇所の値に加え、すべてのセグメントについて、各 `Text` が入力中のいずれかの行と対応することを確かめる（ステップ 3-2）。`writer` の `validateTranscript` を通ることは `internal/writer` 側で確かめ、`writer` の条件を `internal/transcript` に写さない（architecture §7.1）。

### 4.3. 性能・セキュリティのテスト

フェーズ 4 で、architecture §7.3 の 4 MiB の入力、時間、メモリ、fuzz のテストを置く。fuzz は `make test` では種だけを実行し、長時間の実行は手元で 1 回行って本計画 §5.1 に記録する。

### 4.4. 文書の guard

`cmd/yt2column/copied_transcript_docs_test.go` が、`testdata/README.md`・architecture §1.4・CLAUDE.md・`project_overview.md`・`package_reference.md`・`translation_glossary.md` の記載を確かめる（フェーズ 3・5）。guard は AC が求める項目と語の有無を確かめ、文意は読み合わせで確かめる（ステップ 5-11）。`cmd/yt2column` は CI の `doc-guard` ジョブの対象なので、文書だけの変更でも走る。

### 4.5. 既存テストの回帰とテストヘルパー

`Transcript`・`Segment`・`TranscriptSource` を変えないので、既存のテストは変更しない。新しいテストヘルパーのファイル（`testutil/`・`test_helpers_*.go`）は追加しない。`internal/transcript/test_helpers.go` に glob のパターンの定数を 1 つ加えるだけである（ステップ 3-1）。CLAUDE.md の箇条書きの項を取り出す部品は、それを使う guard と同じ `cmd/yt2column/copied_transcript_docs_test.go` に置く（ステップ 5-7）。統合テストはない（architecture §7.2）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

区分は `test`（実行して誤った振る舞いで失敗する）、`static`（guard テストなど、実行される成果物）、`manual`（手作業の確認）である。テスト名は計画の時点の名前である。実装で名前を変えたら、この表を同じコミットで直す。

| AC | 区分 | 検証 |
|---|---|---|
| AC-01 | static / test / manual | `cmd/yt2column/copied_transcript_docs_test.go::TestCopiedTranscriptSurveyRecords`（`testdata/README.md` の出典・ライセンス・取得日・取得方法・加工内容。ステップ 3-4）。`internal/transcript/copied_test.go::TestCopiedSourceRealSamples`（glob の集合が期待値の 3 つの動画 ID と一致し、形 A・形 B の両方を含むこと。ステップ 3-2）。security.md §7 の確認（manual。ステップ 3-6、本計画 §5.2） |
| AC-01a | test / static | `internal/transcript/copied_test.go::TestCopiedSourceRealSamples`（`mbyG85GZ0PI` の `1:00:07` の `StartMs`。ステップ 3-2）。`cmd/yt2column/copied_transcript_docs_test.go::TestCopiedTranscriptSurveyRecords`（architecture §1.4.1 の `2:10:26`・`1:00:07`。ステップ 3-4） |
| AC-01b | static | `cmd/yt2column/copied_transcript_docs_test.go::TestCopiedTranscriptSurveyRecords`（architecture §1.4.1 の 2 つの表の列と、言語・種類の 4 つの組み合わせの行。ステップ 3-4） |
| AC-02 | test | `internal/transcript/copied_test.go::TestCopiedSourceRealSamples`（`testdata/` の各ファイルが書式に一致して解析されること。ステップ 3-2）。`internal/transcript/copied_format_test.go::TestParseCopiedRejects`（形 A・形 B のどちらにも合わない入力の拒否。ステップ 1-7）。利用者への案内は #108・#109 が実装し、そこで確かめる（architecture §4.4・§7.4） |
| AC-03 | static | `cmd/yt2column/copied_transcript_docs_test.go::TestCopiedTranscriptSurveyRecords`（architecture §1.4.2 の、全体の選択の可否・調べた最長の長さ・CLI の手段。ステップ 3-4） |
| AC-04 | test | `internal/transcript/copied_test.go::TestCopiedSourceRealSamples`（ステップ 3-2） |
| AC-04a | test | `internal/transcript/copied_format_test.go::TestParseCopiedAccepts`（`0:00\n0:05\nこんにちは\n`、形 A の本文のない行、時刻の行が 3 行続く形 B。ステップ 1-6）。`internal/transcript/copied_format_test.go::TestParseCopiedRejects`（`0:00\n0:05\n` の `errCopiedNoText`。ステップ 1-7） |
| AC-04b | test | `internal/transcript/copied_format_test.go::TestCopiedLabels`（ステップ 1-5）。`internal/transcript/copied_format_test.go::TestParseCopiedAccepts`（requirements AC-04b の受理の例。ステップ 1-6）。`internal/transcript/copied_format_test.go::TestParseCopiedRejects`（カンマのないラベル、`0:24abc…`。ステップ 1-7） |
| AC-05 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（architecture §4.1 の各理由。ステップ 1-7）。`internal/transcript/copied_format_test.go::TestParseCopiedCheckOrder`（ステップ 1-9）。`internal/transcript/copied_format_test.go::FuzzNewCopiedSource`（拒否は `ErrCopiedTranscriptFormat` を包む。ステップ 4-4） |
| AC-06 | test | `internal/transcript/copied_format_test.go::TestParseCopiedErrorOmitsInput`（ステップ 1-10）。`internal/transcript/copied_test.go::TestNewCopiedSourceErrorOmitsInput`（メタデータと `VideoID`。ステップ 2-5）。`internal/transcript/copied_test.go::TestCopiedSourceFetch`（`videoURL` の値。ステップ 2-4）。`internal/transcript/copied_format_test.go::FuzzNewCopiedSource`（文面が理由の文と行番号だけ。ステップ 4-4） |
| AC-07 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（`errCopiedInvalidUTF8`、ASCII の空白・U+3000・NBSP だけの入力の `errCopiedBlank`。ステップ 1-7） |
| AC-07a | test | `internal/transcript/copied_format_test.go::TestParseCopiedLineBreaks`（ステップ 1-8）。`internal/transcript/copied_format_test.go::FuzzNewCopiedSource`（性質 (3)。ステップ 4-4） |
| AC-08 | test | `internal/transcript/copied_format_test.go::TestParseCopiedLargeInputs`（ステップ 4-1〜4-3） |
| AC-09 | test | `internal/transcript/copied_test.go::TestNewCopiedSource`（ステップ 2-2）。`internal/transcript/copied_test.go::TestCopiedSourceHoldsOnlyTranscript`（ステップ 2-5） |
| AC-10 | test | `internal/transcript/copied_test.go::TestCopiedSourceFetch`（ステップ 2-4） |
| AC-11 | test | `internal/transcript/copied_test.go::TestCopiedSourceTranscript`（ステップ 2-3） |
| AC-12 | test | `internal/transcript/copied_test.go::TestCopiedSourceTranscript`（ステップ 2-3） |
| AC-13 | test | `internal/transcript/copied_test.go::TestCopiedSourceTranscript`（パーサの直接の結果との一致。ステップ 2-3）。`internal/writer/prompt_test.go::TestValidateTranscriptAcceptsCopiedSamples`（ステップ 3-3） |
| AC-14 | static | `cmd/yt2column/copied_transcript_docs_test.go::TestClaudeMDCopiedTranscriptException`（ステップ 5-8） |

### 5.1. 計測と fuzz の記録

ステップ 4-3・4-5 の結果を記録する。欄は実装時に埋める。

| 項目 | 値 | 条件（コミット・実行環境） |
|---|---|---|
| (a)・(b)・(c) の解析の `TotalAlloc` ÷ 入力の長さ（実測） | - | - |
| テストの上限の定数 | - | - |
| (a)・(b)・(c) の解析時間（`make test`、`-race` 付き） | - | - |
| 60 秒の fuzz の実行回数と結果 | - | - |

### 5.2. 手動確認の記録

| 確認 | 日付 | 結果 |
|---|---|---|
| 実データ 3 ファイルが取得者を特定できる情報を含まないこと（security.md §7。ステップ 3-6） | - | - |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| 形 A の行で、本文の先頭の数字や `秒`・`seconds` をラベルの一部と取り違える | 本文の分割が黙ってずれる | ラベルを時刻から導いて照合する（architecture §1.1 原則 2）。ステップ 1-6 で本文が数字で始まる行を確かめ、ステップ 1-11 (a) でテストが照合の欠落を検出することを確かめる |
| 拒否のテストの入力が、意図と別の規則で拒否される | テストが目的の規則を確かめない | ステップ 1-7 で理由の番兵まで `errors.Is` で確かめる。ステップ 1-11 で規則を外したときに失敗することを確かめる |
| `-race` 付きの実行で、4 MiB の解析が 5 秒を超える | `make test` が不安定になる | ステップ 4-2 で `make test` の実行時間を測り本計画 §5.1 に記録する。超える場合は、上限を変える前に、解析が入力の長さに比例しているか（architecture §3.3）を確かめる |
| メモリの上限の定数が厳しすぎて、Go の版や GC の挙動で揺れる | テストが不安定になる | `TotalAlloc`（累計で GC に左右されない）を使い、定数は実測に余裕を持たせる（ステップ 4-3）。2 乗に比例する割り当てで失敗することはステップ 4-6 (b) で確かめる |
| fuzz の長時間の実行で失敗が見つかる | 実装の誤りがある | ステップ 4-5 でパーサを直し、その入力を表のテストに加え、自動生成のファイルを削除する |
| 実データの期待値をパーサの出力から写してしまう | テストがパーサの誤りを固定する | ステップ 3-2 で、期待値は本計画 §1.3 の独立の実測値と目で読んだ値から書く |
| `mnd`・`gocyclo`・`err113` が本番のコードを指摘する | `make lint` が通らない | 数値は名前つきの定数に、パーサは小さな関数に分け、エラーはすべてパッケージの番兵にする（本計画 §1.3「lint」） |
| `make deadcode` が `NewCopiedSource` などを到達不能として報告し、削除されてしまう | #109 が使う公開の型と関数が消える | 本計画 §1.3「`make deadcode`」とステップ 2-7 のとおり、削除しない |
| CLAUDE.md の例外の文が広すぎて、ほかの箇所で文字列の内容から振る舞いを選ぶことの根拠に使われる | 「Declare, don't infer」が骨抜きになる | ステップ 5-2 で、境界（`internal/transcript` のパーサだけ）を明記する。ステップ 5-8 の guard が境界の記載を確かめる |
| 調査記録の guard が、approved の architecture の表の書式の細部に依存して壊れやすい | architecture の記述の訂正のたびに guard が失敗する | guard は AC が求める列・行・語の存在だけを確かめ、セルの文面の細部を確かめない |

### 6.2. スケジュールのリスク

| リスク | 影響 | 対策 |
|---|---|---|
| fuzz や時間・メモリの計測で、パーサの作り直しが要る誤りが見つかる | フェーズ 4 が延びる | 計算量の性質（architecture §3.3）はフェーズ 1 のパーサの実装の時点で守る。フェーズ 4 で見つかった誤りは、フェーズ 1 のテストに行を加えてから直す |
| #109 がフェーズ 2 の公開の型と関数を待つ | #109 の着手が遅れる | フェーズ 2 までを先に PR にできるよう、`/mkplan2` で PR の境界をフェーズ 2 の後に置くことを検討する |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（ステップ 1-1〜1-11）
- [ ] フェーズ 2 完了（ステップ 2-1〜2-7）
- [ ] フェーズ 3 完了（ステップ 3-1〜3-6）と本計画 §5.2 の記録
- [ ] フェーズ 4 完了（ステップ 4-1〜4-6）と本計画 §5.1 の記録
- [ ] フェーズ 5 完了（ステップ 5-1〜5-11）
- [ ] 本計画 §5 のすべての AC の検証が通る
- [ ] `make test`・`make lint`・`go test -tags test ./cmd/yt2column ./internal/pipeline` が通る

## 8. 成功基準 (Success Criteria)

- **機能:** パーサが architecture §3.2 の形 A・形 B を受理し、§4.1 の各理由で拒否する。`CopiedSource` が §3.1 の性質を満たす。実データ 3 ファイルが解析され、`writer` の検査を通る（AC-01〜AC-13）。
- **品質:** 各テストが、その理由で失敗しうることを壊して確かめ、コミットメッセージに記録している。`make test`・`make lint` が通る。
- **セキュリティ:** 拒否のエラーに入力の文字列を含めない（AC-06）。部分的な結果を返さない。4 MiB の入力で時間とメモリが入力の長さに比例する（AC-08）。fuzz でパニックしない。外部モジュールを追加しない。実データが security.md §7 を満たす（本計画 §5.2）。
- **文書:** CLAUDE.md の 2 つの例外、`project_overview.md`、`package_reference.md`、`translation_glossary.md` が architecture §3.4・§3.6 を満たす（AC-14）。

## 9. 次のステップ (Next Steps)

- 本計画のレビューと承認の後、`/mkplan2 0015` で PR の境界を埋め込み、`/runplan 0015` で実装する。
- #109（サーバ）は、フェーズ 2 の公開の型と関数を使う。architecture §9 の #109・#108・#111 への要求（拒否のログ、受理したときのログ、案内の文面、拡張が見出しを除かないこと）は、それぞれのタスクで扱う。
