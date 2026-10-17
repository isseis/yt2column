# 実装計画書：コピーした字幕に混じるパネルの行（前置き・チャプター見出し・末尾の操作ラベル）を読み飛ばす

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-18 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。既存のファイルの行番号は、断りがない限りコミット `7d1644f` で確かめた。architecture が確かめた `b4466af` から `7d1644f` までの Go の差分は `cmd/yt2column/run.go` と `cmd/yt2column/cost_test.go`（0022）だけで、本計画が変えるファイルには差分がない（`git diff --stat b4466af HEAD -- '*.go'` で確認）。Go のソースのコメント・識別子・文字列リテラルは英語で書く（`_context.md`）。ただし §1.2 の例外のとおり、YouTube の UI の文字列そのものを照合する日本語のリテラルは日本語のまま置く。`AC-NN`・`F-NNN` は Go のソースに書かず本計画にだけ記録する（`requirements_process.md` §4）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements の機能要件（F-001〜F-007）を、architecture の設計どおりに実装する。`internal/transcript` のコピーした字幕のパーサ（`copied_format.go`）が、前置き（**最初のセグメントの行**（先頭から時刻を読める最初の行。§2 ステップ 1-2 で定義）より前の行。F-007）・形 A のチャプター見出し行（F-001）・末尾の領域（空白行と操作ラベル行。F-002・F-006）を読み飛ばし、それ以外のセグメントでない行は拒否し続けるように変える。あわせて、書式の判定を行の先頭ではなく最初のセグメントの行で行う。公開の型・関数・番兵の契約は変えない。実装は architecture §8 の実装優先順位どおり 5 つのフェーズに分ける。

### 1.2. 実装原則

- architecture §1.1 の設計原則（0015 §1.1 の 6 原則に加える 4 点）に従う。
- 変更するのは `internal/transcript/copied_format.go`・`internal/transcript/copied_format_test.go`・`CLAUDE.md`・`cmd/yt2column/copied_transcript_docs_test.go` だけである（architecture §3.4。`cmd/yt2column` の guard は §2.0 の editorial correction で加えた）。`internal/transcript/copied.go`・`errors.go`・`copied_test.go`・`internal/writer/prompt_test.go`・`internal/pipeline` の guard は変更しない。
- 読み飛ばす行の形（見出しの形と操作ラベルの一覧）と前置きの拒否の規則は、`internal/transcript` の 1 か所に置く（requirements §4.5）。
- 時刻の文法・読み上げ用ラベル・形 A と形 B の本文の規則・エラーの契約・サイドエフェクト契約は 0015 のままとする（architecture §3.2・§4.1・§4.2・§4.3）。
- **日本語の文字列リテラルの例外。** 日本語 UI の読み上げ用ラベルの単位（`秒`・`分`・`時間`。`copied_format.go:47-51`）と同じ理由で、見出しの語 `チャプター` と末尾の操作ラベル `動画の時間と同期` は、パーサが照合する YouTube の UI の文字列そのものなので、日本語の文字列リテラルとして `copied_format.go` に置く。宣言を 1 か所にまとめ、その理由を英語のコメントで記す（0015 の実装計画 §1.2 と同じ扱い）。テストの入力に使う日本語の字幕・見出し・ラベルの文字列も、入力データとしてだけ使う。
- 各 PR の完了時に、`_context.md` の "Build checks"（`make fmt` → `make test` → `make lint`）を通す。グリーンゲートは `make test && make lint`。`make test` は `-tags test -race` で走り（`Makefile:71-72`）、race なしの `test` ビルドは `make lint` の golangci-lint（`--build-tags test,integration`。`Makefile:10`・`:196-197`）が型検査する。`make lint` は続けて `go vet -tags integration ./...`（`Makefile:198`）も実行するが、これは `test` タグの付いたファイルをコンパイルしない。新しいテストのファイルには既存と同じく `//go:build test` を付ける（本計画は新しいテストファイルを作らない。§4.4）。
- 各テストは、対象の仕組みを壊して失敗することを実装時に確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Every test must be able to fail for its stated reason」）。本計画の「壊して確かめる」タスクは、この義務を具体化したものである。

### 1.3. 既存コード調査結果

**パーサ（`internal/transcript/copied_format.go`、フェーズ 1・2 で変える）。** `parseCopiedTranscript`（`:86-102`）は、UTF-8・空白・行の区切りの 3 つの検査の後、`splitCopiedLines`（`:148-157`）で行に分け、`parseTimestampLine(lines[0])`（`:98-101`）で形を決めて `parseCopiedFormB`・`parseCopiedFormA` に渡す。`parseTimestampLine`（`:161-167`）は行全体が時刻のときだけ真を返し、`parseTimestampPrefix`（`:174-190`）は行の先頭の時刻と読んだバイト数を返す。`checkCopiedLine`（`:280-288`）は全行に共通の空行・制御文字の検査をする。`parseCopiedFormA`（`:304-343`）は、時刻がなければ `errCopiedNotTimestamp`（`:315`）、時刻だけの行は `errCopiedMixedForms`（`:318`）、ラベル不一致は `errCopiedLabelMismatch`（`:322`）、本文が空白だけなら `errCopiedBlankText`（`:329`）で拒否する。`parseCopiedFormB`（`:351-386`）は、本文を待っていない位置の非時刻行を `errCopiedNotTimestamp`（`:371`）、形 A として読める行を `errCopiedMixedForms`（`:374`）、空白だけの本文を `errCopiedBlankText`（`:377`）で拒否する。`readableAsFormA`（`:391-398`）は形 B の混在検査に使う。拒否の理由の番兵は `:16-28`、日本語のラベル単位の定数は `:47-51`、時刻の文法の正規表現は `:56-59`、`matchFormALabel`（ラベルの言語の固定）は `:264-276` にある。**本タスクで変えるのは、`:86-102` の形の判定と処理の順序、`:148-157` の行の分割の扱い、全行の検査の前後の処理、および新設する見出し・操作ラベルの判定だけである。時刻の文法（`:174-197`）・ラベル導出（`:210-258`）・ラベル照合（`:264-276`）・`Segments` の組み立ては変えない。** `parseCopiedTranscript` の呼び出し元は `copied.go:43` だけである（`rg -n 'parseCopiedTranscript' internal cmd` で確認。ほかはテスト）。

**古くなった Go のコメント（フェーズ 1・2 で直す）。** `parseCopiedTranscript` の doc コメント（`:80-85`）は「先頭の行だけで形を決める」「どちらの形にも合わない入力をすべて拒否する」と述べており、本タスクの後は事実でなくなる。`splitCopiedLines` の doc コメント（`:145-147`）は「2 個目以降の末尾の区切りは空行を作り、空行の検査で捕まる」と述べており、末尾の領域の無視により事実でなくなる。`parseCopiedFormA` の doc コメント（`:299-303`）は見出し行の読み飛ばしを、`parseCopiedFormB` の doc コメント（`:345-350`）は末尾の領域の除去を述べていない。これらの doc コメントを本設計に合わせて直す。`checkCopiedLine`（`:278-279`）・`readableAsFormA`（`:388-390`）・`parseTimestampLine`・`parseTimestampPrefix` の doc コメントは変わらない。

**既存のテスト（`internal/transcript/copied_format_test.go`、フェーズ 3 で書き換える）。** architecture §3.5 のとおり、次が本設計で結果を変える（行番号はコミット `7d1644f`）。

| ケース（`copied_format_test.go`） | 今の期待 | 本設計での期待 |
|---|---|---|
| `TestParseCopiedRejects` の `two trailing separators`（`:245`） | `errCopiedEmptyLine` 行 3 | 受理（末尾の領域） |
| `TestParseCopiedRejects` の `three trailing separators`（`:246`） | `errCopiedEmptyLine` 行 3 | 受理（末尾の領域） |
| `TestParseCopiedLineBreaks` の `trailingRejects` の同名 2 ケース（`:343-344`） | `errCopiedEmptyLine` 行 3 | 受理（末尾の領域） |
| `TestParseCopiedRejects` の `empty line at the start`（`:244`。入力 `\n0:00\nx\n`） | `errCopiedEmptyLine` 行 1 | 受理（先頭の空行は前置き） |
| `TestParseCopiedRejects` の `leading display items`（`:249`。入力 `タイムライン\n`） | `errCopiedNotTimestamp` 行 1 | `errCopiedNoText`（前置きだけでセグメントの行がない） |
| `TestParseCopiedCheckOrder` の `line kind before blank text`（`:394-398`。入力 `0:00\nx\n \n`） | `errCopiedNotTimestamp` | 受理（末尾の空白行を無視） |
| `TestParseCopiedRejects` の `form B whitespace-only text`（`:264`。入力 `0:00\n \n`） | `errCopiedBlankText` 行 2 | `errCopiedNoText`（末尾の U+0020 だけの行を無視し、本文のあるセグメントが残らない） |

`TestParseCopiedCheckOrder` の `line kind before blank text`（`:394-398`）は本設計では受理に変わるため、順序の確認を、空白文字だけの行がセグメントの間にある入力（`0:00\nx\n \n0:05\ny\n`。`errCopiedNotTimestamp` 行 3）に移し、元の入力は受理のケースとして残す（この書き換えはステップ 3-2 のタスク）。`TestParseCopiedErrorOmitsInput` の `not a timestamp omits the line`（`:425-427`。入力 `MARKER_LINE\n`）は前置きだけの入力になり `errCopiedNoText` に変わるので、行を最初のセグメントの行より後に置く（`0:00\nx\nMARKER_LINE\n`。`errCopiedNotTimestamp` 行 3。architecture §3.5 の 2 つ目の表）。

時刻の文法の拒否のケース（`leading BOM`・`minute leading zero`・`minute over 59`・`single-digit second`・`zero hour in h:mm:ss`・`three-digit hour`・`m:ss then colon and three digits`・`m:ss then colon and one-digit second`。`:251-258`）は、いずれも先頭の行が数字・`:`・数字の連続を含むため、§3.2.1 の前置きの拒否の規則（`errCopiedNotTimestamp`）により、理由と行番号を変えずに通る。`chapter heading after text`（`:250`。入力 `0:00\nx\n存在感のある声\n`）は形 B の入力で見出しが本文の行の直後にあり、時刻の行でも見出しの形の行でもないため、引き続き `errCopiedNotTimestamp` 行 3 で拒否する。`empty line in the middle`（`:243`）・`bad timestamp at a form B position`（`:259`）・`blank newline`（`:239`）・`TestParseCopiedCheckOrder` の他の行（`:363-392`）・`TestParseCopiedAccepts` のすべての場合（`:56-224`）は、入力が前置き・末尾の領域を含まないため、結果を変えない。

**検証の資産（フェーズ 3 で使う）。** `assertCopiedRejected`（`:463-476`）は、`errors.Is(err, ErrCopiedTranscriptFormat)` と理由の番兵を確かめ、部分的な結果（`nil` でない `[]Segment`）を返さないことを確かめる。`TestExportedSentinelErrorsUnchanged`（`internal/transcript/json3_test.go:472-499`）は `internal/transcript` の公開の番兵の集合を固定しており、`ErrCopiedTranscriptFormat` を包む新しい公開の番兵を足せば失敗する（AC-14 の「新しい番兵を足さない」を実行される guard で確かめる。新しい guard は要らない）。`FuzzNewCopiedSource`（`copied_format_test.go:559-593`）と `assertCopiedFuzzAcceptance`（`:608-635`）は、受理した結果の不変条件（本文が空白だけでない・UTF-8・`StartMs` の非減少）と LF/CRLF の一致を確かめる。本設計はこれらの不変条件を変えない（前置き・末尾の領域の行はセグメントにならず、形 A の空白だけの本文と形 B の空白だけの本文は従来どおり拒否する設計である）ので、fuzz の種と不変条件は変えない。

**実データの guard（フェーズ 3・対象外の確認）。** `copiedSampleFiles`（`copied_test.go:344-348`）と `TestCopiedSourceRealSamples`（`:364-424`）は、`testdata/` のコピーした字幕のファイルをちょうど 3 つに固定する。`TestCopiedTranscriptSurveyRecords`（`cmd/yt2column/copied_transcript_docs_test.go:25-106`）は、`testdata/README.md` が保存した各サンプルの出典・ライセンス・取得方法を記録していることを確かめる。本タスクは `testdata/` を変えないので、両方とも変更なく通る（AC-16・AC-17 の実行される確認）。

**性能テストの資産（フェーズ 4 で使う）。** `TestParseCopiedLargeInputs`（`copied_format_test.go:496-535`）は 4 MiB の合成入力 3 つ（`:537-551` の組み立て関数）を解析し、`copiedParseDeadline`（`:487`）以内に完走し、`copiedParseAllocPerByte` バイト/入力バイト以下を割り当てることを確かめる。`copiedParseAllocPerByte` は `-race` 付きのビルドで `copied_format_race_test.go:14`（8192）、race なしで `copied_format_norace_test.go:9`（128）である。`TestParseCopiedLargeInputs` は `t.Parallel` を呼ばず、`runtime.MemStats` の差で割り当てを測る（`:493-495`）。本設計は前置き・末尾の領域・見出し行の判定を加えるが、いずれも行の長さに比例するので、この上限の範囲に収まる。

**guard のテスト（フェーズ 5 で変える）。** `TestClaudeMDCopiedTranscriptException`（`cmd/yt2column/copied_transcript_docs_test.go:166-186`）は、`claudeBullet`（`:315-317`）で `CLAUDE.md` の「Declare, don't infer」の項（`:150-168`）を取り出し、`copied-transcript parser`・`internal/transcript`・`rejected` などの語を含むことを確かめる。本タスクは例外の境界の文を加えるので、この guard に、前置きの範囲を位置で決めることと、最初のセグメントの行より後に読み飛ばすのは明示的に定義した行だけであることの確認を加える（AC-27）。既存の要求語は残す。

**パッケージ境界と具体型。** 本タスクは新しいパッケージ・公開の名前を作らない。`internal/pipeline` の `TestCommonTypesFieldSets`（`pipeline_test.go:432`）が固定する `transcript.Segment`・`Transcript` のフィールド集合を変えない。具体型で振る舞いを分ける設計（mkplan ステップ 5 の最終項目）は加えない。読み飛ばす行の形と操作ラベルの一覧は非公開の定数と関数で、`internal/transcript` の外からは作れない。

**外部の前提の確認。** 本タスクは外部コマンド・ネットワーク・秘密情報を新たに扱わない。照合する文字列の出所は次のとおりである。日本語 UI の末尾の操作ラベル `動画の時間と同期` は、コミット済みの `testdata/2tcCWM-sRBw.transcript_panel.html:3206`・`:3211` に実在することを確認した。日本語 UI の見出しの語 `チャプター` は、requirements §1 が記録した観測（2026-10-17、`LR6mhKRKX9Q` の `チャプター 2: 通常スピード`。requirements `:22`）による。英語 UI の末尾のラベル `Sync to video time` は、requirements §F-002 が記録した観測（2026-10-17、英語 UI のパネル）による。英語 UI の見出し `Chapter` は**観測していない**。requirements §F-001 は、日本語 UI の見出しと同じ規則として対称性によりこれを受理し、理由のある未観測の形と位置づけている。したがって、英語 UI の見出しを実物で確認することは、§2 の手動の確認（M-1）の対象にしない。

**申し送り。** `design_handoff.md`・`implementation_handoff.md` は本タスクのディレクトリに存在しない（`ls` で確認）。対応する `H-NN`・`I-NN` はない。

### 1.4. 対象外（requirements §2.3）

形 B の 2 つ目以降のチャプターの見出し、前置きの行と末尾の領域の行の数の上限、パーサが受理するセグメントの書式の変更、拡張（`extension/`）の変更、サーバ（`internal/server`）の拒否の理由と応答の形式の変更、`transcript.Transcript`・`Segment`・`TranscriptSource` の型の変更は、本タスクの対象外である。

### 1.5. 横断検索チェックリスト

本タスクは既存の名前を変更・削除しない。新しい非公開の関数・定数は `internal/transcript` に閉じる。`make lint`・`make test` で検出できないのは、古い振る舞いを述べる Go のコメントの残存だけである（§1.3 の「古くなった Go のコメント」）。これはフェーズ 1・2 のタスクに含め、実装時に `rg -n 'first line|trailing separator|reject' internal/transcript/copied_format.go` で残存を確かめる。AC との重複を避けるため、AC の検証表には同じ確認を置かない。文書と用語集については、新しい用語（前置き・見出しの形・末尾の領域）を `translation_glossary.md` に加えるという architecture の要求はなく、これを強制する guard もないので、加えない。

## 2. 実装ステップ (Implementation Steps)

### 2.0. 前提（解消済み）

計画の作成時に、architecture にある「変更するファイル」の一覧（§2.1・§3.4）が `cmd/yt2column/copied_transcript_docs_test.go` を欠いているのを見つけ、editorial correction として加えた（status は `approved` のまま。`Comments` に記録）。AC-27 の静的な確認は、既存の guard `TestClaudeMDCopiedTranscriptException` の拡張として実行する。設計の決定は変えていない。

### フェーズ 1: 前置き・末尾の領域の除去と形の判定

**対象ファイル:** `internal/transcript/copied_format.go`、`internal/transcript/copied_format_test.go`

- [ ] **ステップ 1-1**: architecture §3.2.3・§3.2.4 の手順 4 のとおりに、**末尾の領域**を入力の最後から前に向かって決めて除く関数を `copied_format.go` に加える。末尾の領域は、0 文字の行・`unicode.IsSpace` が真の文字だけからなる行・**本文の位置にない**操作ラベルの行が続く限りである。本文の位置（直前の行が時刻だけの行である位置）の判定は、行の局所的な性質だけで行う。制御文字を含む行は末尾の領域に含めず、そこで末尾の領域を終える。この関数は拒否をせず、除く範囲を返す。操作ラベルの一覧（`動画の時間と同期`・`Sync to video time`）と全体一致の判定を、§1.2 の日本語リテラルの例外に従って 1 か所に置き、理由を英語のコメントで記す。新しく観測されたラベルを 1 行加えるだけで対応できる形にする（requirements §4.5）。
- [ ] **ステップ 1-2**: architecture §3.2.1・§3.2.4 の手順 5 のとおりに、**前置き**を残った行の先頭から決めて除く処理を加える。前置きの各行には、制御文字の検査（`errCopiedControlChar`）と数字・`:`・数字の連続の検査（`errCopiedNotTimestamp`）をこの順に適用する。先頭の行から時刻が読める最初の行（`parseTimestampPrefix` が成功する最初の行）で前置きは終わる。前置きだけの入力（最初のセグメントの行がない）は `errCopiedNoText` で拒否する。
- [ ] **ステップ 1-3**: architecture §3.2.4 の手順 6 のとおりに、**形の判定**を `lines[0]` ではなく最初のセグメントの行で行う（時刻だけの行なら形 B、そうでなければ形 A）。形ごとの検査の対象は、最初のセグメントの行から末尾の領域の直前までの範囲とする。`parseCopiedFormA`・`parseCopiedFormB` は `lineNumber := index + 1` で行番号を作る（`:309`・`:357`）ので、範囲の先頭の行番号を基準として渡す（開始行番号を引数で渡すか、除く前の `lines` と開始位置を渡すか、いずれかとする）。これにより、除く前の入力の行番号を保つ（`errCopiedEmptyLine` などの拒否の行番号を変えない。AC-15・AC-18）。
- [ ] **ステップ 1-4**: ステップ 1-1〜1-3 で事実でなくなる Go のコメント（§1.3 の「古くなった Go のコメント」）を直す。`parseCopiedTranscript`（`:80-85`）・`splitCopiedLines`（`:145-147`）・`parseCopiedFormA`（`:299-303`）・`parseCopiedFormB`（`:345-350`）の doc コメントを、前置き・末尾の領域の除去と、受理・拒否の境界の記述に合わせる。
- [ ] **ステップ 1-5**: フェーズ 1 のテストを `copied_format_test.go` に書く。返る `Segments` を、前置き・末尾の領域を取り除いた入力の `Segments` と比べる（requirements の AC の文言のとおり。要素の数・`StartMs`・`Text`・順序）。
  - `TestParseCopiedPrefix`: 観測した前置き（`動画の関連情報`・空行 2 つ・`タイムライン`・空行・`文字起こし`・`文字音声変換を検索` 2 行・空行）の後に形 A のセグメントが続く入力（AC-28）、前置きの後に形 B のセグメントが続く入力（AC-29）、前置きに日本語の行があり最初のセグメントの行が英語 UI のラベル（`0:000 secondsHello`）の入力（AC-32）。前置きのどの行の文字列も返る `Segment.Text` に含まれないこと（AC-28）。先頭の空行が前置きとして読み飛ばされること（AC-24 の前半）。
  - `TestParseCopiedTrailingArea`: §1 で観測した末尾（最後のセグメントの行・空行・`動画の時間と同期`。最後の行の区切りなし。`…\n17:0017 分<本文>\n\n動画の時間と同期`）を持つ形 A の入力（AC-07）、英語 UI のラベル `Sync to video time` を末尾に持つ入力（AC-07）、形 A の末尾の空行（`17:3017 分 30 秒寝ます\n\n`。AC-21）、末尾の U+0020 だけの行・U+3000 だけの行（AC-22）、末尾に 2 個以上の空行・空行と `動画の時間と同期` が混じる末尾（AC-23）、形 B の末尾の空行（`0:00\nx\n\n` と `0:00\nx\n0:05\n \n`。後者は `0:05` が本文のないセグメントになる。AC-22）。返る `Segments` を末尾の領域を取り除いた入力と比べ、`動画の時間と同期` の文字列が返るどの `Segment.Text` にも含まれないことを確かめる（AC-07）。
  - `TestParseCopiedRejects` に次のケースを加える（既存のケースと重複させない）。前置きの数字・`:`・数字の連続を含む入力行をその行番号の `errCopiedNotTimestamp` で拒否する（AC-30）。このうち `05:00`・`0:5`・`﻿0:00` は、既存の `minute leading zero`（`:252`）・`single-digit second`（`:254`）・`leading BOM`（`:251`）が同じ理由と行番号で通っており、本設計では前置きの検査の経路を通るので、新しいケースは加えない。新たに ` 0:00`（先頭に空白）の入力のケースを加える。前置きの制御文字を含む行（`タイ\tムライン`）をその行番号の `errCopiedControlChar` で拒否するケースを加える（AC-30）。最初のセグメントの行がない入力（`タイムライン\n`・`文字起こし\n\n動画の関連情報`）を `errCopiedNoText` で拒否する（AC-31。`leading display items`（`:249`）のケースを `errCopiedNoText` に書き換え、`文字起こし\n\n動画の関連情報` のケースを加える）。最初のセグメントの行と末尾の領域の間にある空行（`0:000 秒はい\n\n0:1010 秒です\n`）を `errCopiedEmptyLine` で拒否する（AC-24 の後半。既存の `empty line in the middle`（`:243`）が同じ）。末尾の行にタブを含む入力（`0:000 秒はい\n\t\n`）を `errCopiedControlChar` で拒否する（AC-26）。入力のすべてが空行（`\n\n`）を `errCopiedBlank` で拒否する（AC-25。`TestParseCopiedCheckOrder` の `blank before empty line`（`:370-374`）が同じ）。
  - `TestParseCopiedRejects` に、末尾の操作ラベルの拒否と、末尾の領域の外の操作ラベルの拒否を加える。操作ラベルと同じ語を含むが一致しない末尾の行（`動画の時間と同期について話します`・`動画の時間と同期 `（末尾に空白）・`Sync to video`・`sync to video time`）を拒否するケース（AC-09）。セグメントの間にある `動画の時間と同期` の行（`0:000 秒はい\n動画の時間と同期\n0:1010 秒です\n`）を `errCopiedNotTimestamp` で拒否するケース（AC-10）。操作ラベル行だけで本文のあるセグメントがない入力（`文字起こし\n動画の時間と同期\n`）を `errCopiedNoText` で拒否するケース（AC-11）。
- [ ] **ステップ 1-6**: 壊して確かめる。次の操作で、それぞれ対応するテストが失敗することを確かめる。末尾の領域の判定から本文の位置を除く条件を外す（本文が操作ラベルと一致する形 B のセグメントが読み飛ばされ、AC-33 のテストが失敗する。AC-33 はフェーズ 3 で書くので、そのとき確かめる）。末尾の領域の判定で制御文字の除外を外す（`TestParseCopiedRejects` の末尾のタブのケース）。前置きの数字・`:`・数字の検査を外す（AC-30 のケース）。前置きの制御文字の検査を外す（AC-30 の制御文字のケース）。形の判定を `lines[0]` に戻す（`TestParseCopiedPrefix` の形 B のケース）。

**完了条件:** フェーズ 1〜3 は 1 つの PR（PR-1）として実装するので、個別にはグリーンゲートを通さない。フェーズ 1 のテスト（ステップ 1-5）と、フェーズ 1 で結果が変わる既存のテスト（続くフェーズ 3 で書き換える §1.3 の表の行）が、PR-1 の完了時に `make test`・`make lint` を通す。

### フェーズ 2: 形 A のチャプター見出し行の読み飛ばし

**対象ファイル:** `internal/transcript/copied_format.go`、`internal/transcript/copied_format_test.go`

- [ ] **ステップ 2-1**: architecture §3.2.2 の見出しの形を判定する非公開の関数を `copied_format.go` に置く。1 つの行の全体が `チャプター` + U+0020（1 つ）+ 先頭に `0` を付けない正の整数 + `:`（ASCII）+ U+0020（1 つ）+ タイトル（1 文字以上、最後の文字は空白文字でない）に一致するか、または `Chapter` を使う同じ形に一致するかを返す。数字は ASCII の `0`〜`9` だけを使う。行の先頭・末尾の空白文字を許さない。`チャプター`・`Chapter` の語は §1.2 の例外に従って日本語・英語のリテラルで置き、理由を英語のコメントで記す。見出しの形を 1 か所に置く（requirements §4.5）。
- [ ] **ステップ 2-2**: `parseCopiedFormA` の行ごとの検査で、architecture §3.2.4 の手順 7 のとおりに、**空行の検査と制御文字の検査の後、形ごとの規則の前**に見出し行の読み飛ばしの判定を置く。一致する行はセグメントを作らずに次へ進む。最初のセグメントの行より前の見出し行は前置きとして読み飛ばす（フェーズ 1）。形 B（`parseCopiedFormB`）では見出し行を読み飛ばさない（本文の位置の見出しはその時刻の本文になる。§3.2.2）。見出しの語の言語は読み上げ用ラベルの言語の固定に影響しない。
- [ ] **ステップ 2-3**: `TestParseCopiedChapterHeadings` を書く。日本語 UI の見出し行がセグメントの間にある入力（AC-01。返る `Segments` を見出し行を取り除いた入力の `Segments` と比べる）、タイトルの文字列が返るどの `Segment.Text` にも含まれず、要素の数が時刻で始まり本文を持つ行の数に等しいこと（AC-02）、最後のセグメントの行の後の見出し行（AC-03）、見出しの語の言語と読み上げ用ラベルの言語が異なる入力（英語 UI のラベルのセグメントと日本語 UI の見出し行。AC-03）、英語 UI の見出し行（`Chapter 1: Getting started`。F-001 の英語 UI の形。AC-03 の言語の組み合わせの確認と合わせる）、タイトルに `:`・空白・`チャプター` を含む見出し行（`チャプター 5: 手順 2: 次のチャプターの準備`・`チャプター 12: 朝 と 夜`。AC-06）。`TestParseCopiedRejects` には次の 2 つのケースを加える。1 つ目は、見出しの形に一致しない行（`チャプター: タイトル`・`チャプター 0: タイトル`・`チャプター 01: タイトル`・`チャプター2: タイトル`・`チャプター 2 タイトル`・`チャプター 2: `・英語 UI の `chapter 2: title`）を最初のセグメントの行より後に置き、`errCopiedNotTimestamp` で拒否するケース（AC-04）。2 つ目は、見出し行だけからなり本文のあるセグメントがない入力（`チャプター 1: はじめに\n`）を `errCopiedNoText` で拒否するケース（AC-05）。
- [ ] **ステップ 2-4**: 壊して確かめる。`parseCopiedFormA` の見出し行の読み飛ばしの判定を外すと、AC-01・AC-02・AC-03・AC-06 のテストが `errCopiedNotTimestamp`（または `errCopiedLabelMismatch`）で失敗することを確かめる。見出しの形の正の整数の先頭の `0` の禁止を外すと AC-04 のテストが失敗することを確かめる。

**完了条件:** フェーズ 2 のテストが通り、フェーズ 1 で結果が変わる既存のテストの書き換え（フェーズ 3）と合わせて PR-1 のグリーンゲートを通す。

### フェーズ 3: 既存のテストの書き換えと拒否・エラー契約のテスト

**対象ファイル:** `internal/transcript/copied_format_test.go`

- [ ] **ステップ 3-1**: §1.3 の表の既存のケースを本設計の期待に書き換える。`TestParseCopiedRejects` の `two trailing separators`（`:245`）・`three trailing separators`（`:246`）を受理のテーブルへ移し、`empty line at the start`（`:244`）を受理へ、`leading display items`（`:249`）を `errCopiedNoText` へ、`form B whitespace-only text`（`:264`）を `errCopiedNoText` へ書き換える。`TestParseCopiedLineBreaks` の `trailingRejects`（`:339-353`）の 2 ケースを受理のテストへ移す。受理のケースは、`TestParseCopiedAccepts` の表に加えるか、フェーズ 1・2 の新しいテストに含める。
- [ ] **ステップ 3-2**: 確かめている経路を本設計で通らなくなる次の 2 つのケースを、同じ経路を通る入力に書き換える（architecture §3.5 の 2 つ目の表）。
  - `TestParseCopiedCheckOrder` の `line kind before blank text`（`:394-398`）: 入力 `0:00\nx\n \n` を受理のテストとして残したうえで、順序の確認を `0:00\nx\n \n0:05\ny\n`（`errCopiedNotTimestamp` 行 3）に移す。§3.2.3 の「末尾に限る」が残す順序である。
  - `TestParseCopiedErrorOmitsInput` の `not a timestamp omits the line`（`:425-427`）: 入力を `0:00\nx\nMARKER_LINE\n`（`errCopiedNotTimestamp` 行 3）に変え、メッセージが行の文字列を含まないことを確かめ続ける。
- [ ] **ステップ 3-3**: AC-12〜AC-15 のテストを加える。
  - `TestParseCopiedRejects` に、最初のセグメントの行より後にある、時刻で始まらず見出しの形にも末尾の領域にも当たらない行（`タイムライン`・`存在感のある声`・`What is symmetry in physics?`・`こんにちは`）と、未知の新しいラベル（`チャプターの一覧`・`Download transcript`）を `ErrCopiedTranscriptFormat` を包むエラーで拒否するケースを加える（AC-12・AC-13）。
  - `assertCopiedRejected`（`:463-476`）が、フェーズ 1〜3 で加えるすべての拒否のケースについて、`errors.Is(err, ErrCopiedTranscriptFormat)` と部分的な結果を返さないことを確かめる（AC-14）。
  - `TestParseCopiedErrorOmitsInput`（`:413-458`）の各マーカーで、拒否のエラーの文字列が入力の字幕の文字列（本文・見出しのタイトル・操作ラベル・前置きの行・時刻）を含まないことを確かめる（AC-15）。`TestNewCopiedSourceErrorOmitsInput`（`copied_test.go:277-317`）はそのまま通る。
- [ ] **ステップ 3-4**: architecture §7.1 が求める末尾の領域のテストとして新設する `TestParseCopiedTrailingArea` に次の 2 つを加える。1 つ目は、形 B で最後のセグメントの本文の全体が操作ラベルと完全に一致する入力（`0:00\n前の字幕\n0:05\n動画の時間と同期`。末尾に行の区切りがあってもよい）が受理され、`0:05` の `Text` が `動画の時間と同期` のままであること。2 つ目は、本文を `本文` にし末尾に空行と操作ラベル行を足した入力（`0:00\n前の字幕\n0:05\n本文\n\n動画の時間と同期`）では末尾の操作ラベル行だけが読み飛ばされること（AC-33）。あわせて、形 B の前置きと末尾の両方を持つパネル全体の選択（2 つ目以降のチャプターの見出しを含まないもの）が受理されること（AC-08）を加える。
- [ ] **ステップ 3-5**: 壊して確かめる。末尾の領域の判定から本文の位置を除く条件を外すと AC-33 のテストが失敗することを確かめる（本文が操作ラベルと一致する形 B のセグメントが読み飛ばされ、セグメントが減る）。`assertCopiedRejected` の部分的な結果の検査を外すと、拒否のテストが失敗することを確かめる（拒否の経路が部分的な結果を返すように壊した場合）。`TestExportedSentinelErrorsUnchanged`（`json3_test.go:472`）が、`ErrCopiedTranscriptFormat` を包む新しい公開の番兵を足すと失敗することを確かめる。

**完了条件:** PR-1 のグリーンゲート（`make test && make lint`）が通る。§1.3 の表の範囲の外の既存のテストを変えていない。

### フェーズ 4: 4 MiB の入力と性能のテスト

**対象ファイル:** `internal/transcript/copied_format_test.go`

- [ ] **ステップ 4-1**: architecture §7.3・AC-20 のとおりに、`TestParseCopiedLargeInputs` の 4 MiB の入力を、前置き・形 A のチャプター見出し行・末尾の領域（空行と操作ラベル行）を含むようにする（§1.3 の組み立て関数 `:537-551` に前置きの行と末尾の行を加え、少なくとも 1 つの入力にはチャプター見出し行を加える）。`wantSegments` を合わせて更新する。入力は `copiedLargeInputLimit`（4 MiB）以下を保つ。`-race` 付き（`copiedParseAllocPerByte = 8192`。`copied_format_race_test.go:14`）と race なし（`128`。`copied_format_norace_test.go:9`）の両方の上限を満たすこと。
- [ ] **ステップ 4-2**: 解析の時間と割り当てを測り、その値をコミットメッセージに記録する（0015 の実装計画 §5.1 と同じ方法。`make test` は `-race` 付きで走るので、その実行で満たすこと。race なしの測定も併せて記録する）。上限を変える前に、解析が入力の長さに比例しているか（architecture §3.3）を確かめる。

**完了条件:** 4 MiB の入力が `copiedParseDeadline`（15 秒）以内に完走し、入力バイト当たりの割り当てが両方の上限内である。`make test`・`make lint` が通る。

### フェーズ 5: 文書（CLAUDE.md の例外の境界）

**対象ファイル:** `CLAUDE.md`、`cmd/yt2column/copied_transcript_docs_test.go`

- [ ] **ステップ 5-1**: `CLAUDE.md` の「Declare, don't infer」の項（`:150-168`）の例外の境界の文に、英語で次を加える（AC-27）。例外が及ぶのは `internal/transcript` のコピーした字幕のパーサと `CopiedSource` の `Raw` の解析であること（既存）。読み飛ばす行は、前置き（**最初のセグメントの行より前**の行。範囲は行の内容ではなく行の先頭から時刻を読める最初の行の**位置**で決める）、形 A のチャプターの見出し行（**明示的に定義した見出しの形**に一致する行だけ）、入力の末尾の領域（空白行と**明示的に定義した操作ラベル**の行）だけであること。最初のセグメントの行より後では、明示的に定義したこれらの行だけを読み飛ばし、それ以外のセグメントでない行は解釈を広げて受理せずに拒否すること。
- [ ] **ステップ 5-2**: `TestClaudeMDCopiedTranscriptException`（`cmd/yt2column/copied_transcript_docs_test.go:166-186`）を拡張し、「Declare, don't infer」の項が次の 2 点を述べていることを確かめる。前置きの範囲を位置で決めること。最初のセグメントの行より後に読み飛ばすのは明示的に定義した行だけであること（AC-27）。既存の要求語（`copied-transcript parser`・`form A`・`form B`・`screen-reader label`・`Segments`・`external value`・`cannot express in a type`・`internal/transcript`・`rejected`）は残す。確認する語はステップ 5-1 の文面を書くときに決め、`claudeBullet`（`:315-317`）で項を取り出す。
- [ ] **ステップ 5-3**: 壊して確かめる。`CLAUDE.md` の例外の文から位置の規則の記述を消すと `TestClaudeMDCopiedTranscriptException` が失敗することを確かめる。`make test`（`doc-guard` の CI ジョブが走らせる `go test -tags test ./cmd/yt2column`）が通ることを確かめる。

**完了条件:** グリーンゲートが通る。`TestClaudeMDCopiedTranscriptException` が通る。

### 手動の確認（人の承認を得て行う）

実際のサーバ（`cmd/yt2column-server`）と拡張（`extension/`）を通して送る確認であり、実行には人の承認が要る（CLAUDE.md「Tool Execution Safety」、requirements §5）。PR-1 の変更を実際に動かす確認なので、PR-1 のマージの前に行い、結果を本節の表に記録して PR-1 に含める（AC-19。0011 の手動確認と同じ方法）。

- [ ] **ステップ M-1**: サーバを起動し、拡張を読み込んで、報告された動画（`https://www.youtube.com/watch?v=dsuNgos_kxA`）の文字起こしをチャプターの見出し行を含めて送り、`202` で受理されることを確かめる。あわせて、§1 で観測した前置きから末尾の `動画の時間と同期` までを含むパネル全体の選択を送り、`202` で受理されることを確かめる。

| 確認 | 日時 | 結果 |
|---|---|---|
| M-1（AC-19） | - | - |

### PR-1 作成ポイント: panel lines in the copied transcript parser

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 4-1 / 4-2 / M-1

**推奨タイトル**: `feat(0024): skip the panel lines in the copied transcript parser`

**レビュー観点**: 前置きの範囲が行の内容ではなく最初のセグメントの行の位置で決まること / 形の判定が最初のセグメントの行で行われること / 形 A の見出しの形と末尾の操作ラベルの一覧が §1.2 の例外に従って 1 か所に置かれ、理由の英語コメントがあること / 末尾の領域の判定が制御文字の行と本文の位置の操作ラベル行で止まること / 拒否の理由・行番号・エラーの契約が 0015 のままであること / §1.3 の表の範囲の外の既存のテストを変えていないこと / 4 MiB の入力の測定値がコミットメッセージに記録されていること / M-1 の日時と結果が §2 の表に記録されていること

**実装モデル要件**: frontier-recommended

**判定理由**: パーサの処理の順序と受理・拒否の境界を、既存のテストを書き換えながら組み替える分離された込み入ったステップ（フェーズ 1〜3）を含む。移行・外部リソース・CI の面は持たないので `frontier-required` ではない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### PR-2 作成ポイント: CLAUDE.md exception boundary

**対象ステップ**: 5-1 / 5-2 / 5-3

**推奨タイトル**: `docs(0024): record the panel-line exception in the copied transcript parser`

**レビュー観点**: `CLAUDE.md` の「Declare, don't infer」の例外の境界が、前置きの範囲を位置で決めることと、最初のセグメントの行より後に読み飛ばすのは明示的に定義した行だけであることを述べていること / `TestClaudeMDCopiedTranscriptException` が既存の要求語を保ったまま新しい境界の記述を確かめること / 例外の境界が PR-1 の実装と一致すること

**実装モデル要件**: standard

**判定理由**: 文書の 1 項の追記と、既存の guard の確認の追加である。前例のない設計判断・移行・外部リソースのいずれにも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | このフェーズで作るもの | フェーズ |
|---|---|---|
| M1: 前置きと末尾の領域を読み飛ばせる | `copied_format.go` の末尾の領域・前置きの除去、形の判定の位置、そのテスト | フェーズ 1 |
| M2: 形 A の見出し行を読み飛ばせる | `copied_format.go` の見出しの形の判定と読み飛ばし、そのテスト | フェーズ 2 |
| M3: 既存のテストが本設計に合う | `copied_format_test.go` の §3.5 の書き換え、AC-12〜AC-15 のテスト | フェーズ 3 |
| M4: 4 MiB の入力で性能を確かめる | `TestParseCopiedLargeInputs` の拡張と測定値の記録 | フェーズ 4 |
| M5: 文書が揃う | `CLAUDE.md` の例外の境界、`TestClaudeMDCopiedTranscriptException` | フェーズ 5 |

- フェーズの名前と順序は architecture §8 の実装優先順位 1〜5 と同じである。
- フェーズ 2 は、フェーズ 1 が除いた後の残りの行（前置き・末尾の領域を除いた範囲）を対象とする。フェーズ 3 はフェーズ 1・2 の結果に依存する。フェーズ 4 はフェーズ 1・2 に依存し、フェーズ 5 はフェーズ 2 に依存する（architecture §8）。

### 3.2 PR 構成

PR は 2 つとする。PR-1 はフェーズ 1〜4（パーサの変更とすべてのテスト）、PR-2 はフェーズ 5（`CLAUDE.md` と guard）である。

フェーズ 1〜3 を 1 つの PR にするのは、フェーズ 1 の時点で §1.3 の表の既存のテストが未更新のまま失敗するためである（末尾の区切り・先頭の空行・前置きだけの入力・末尾の空白行の扱いが変わる）。グリーンゲートは PR の完了時に通せばよいので、フェーズ 1〜3 を別々の PR にすると中間の PR が赤くなる。フェーズ 4（4 MiB の性能テスト）はフェーズ 1〜3 の結果を使う同じテストファイルの変更なので、PR-1 に含める。フェーズ 5 は文書と `cmd/yt2column` の guard だけで、パーサの変更に依存する（例外の境界が新しい振る舞いを述べる）ので PR-2 とする。

PR は PR-1 → PR-2 の順とし、PR-2 は PR-1 に依存する。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1〜1-6 / 2-1〜2-4 / 3-1〜3-5 / 4-1〜4-2 / M-1 | `copied_format.go` の前置き・末尾の領域の除去、形の判定の位置、見出し行の読み飛ばし、`copied_format_test.go` の新しいテストと既存のテストの書き換え、4 MiB の性能テスト | frontier-recommended |
| PR-2 | 5-1〜5-3 | `CLAUDE.md` の「Declare, don't infer」の例外の境界、`TestClaudeMDCopiedTranscriptException` の拡張 | standard |

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

ユニットテストは実際の `yt-dlp`・LLM の API・Webhook・ブラウザを呼ばない（CLAUDE.md「Testing Strategy」）。テストの入力は `copied_format_test.go` の中の合成の文字列で作り、見出しのタイトルとセグメントの本文は架空の文字列とする（AC-16）。報告された入力（動画 `dsuNgos_kxA`）と観測した入力の全体は `testdata/` に保存しない（AC-16・AC-17）。対象は architecture §7.1 のとおりで、AC との対応は §5 に記す。目標は、§5 の各 AC の行のテストが通ることと、`make test` が通ることである。

### 4.2. 統合テスト・手動の確認

本タスク単体の統合テストはない（architecture §7.2）。サーバからの呼び出しは #109 の統合テストで確かめる。実際のサーバと拡張を通した受理は、人の承認を得て M-1 で行い、結果を §2 の表に記録する（AC-19）。§5 の AC-19 の行はこの確認を指す。

### 4.3. セキュリティテスト

architecture §5.2 の脅威の対策を、次のテストに含める。読み飛ばしで本来のセグメントを黙って失わないこと（AC-30・AC-33。ステップ 1-5・3-4）、読み飛ばした行の内容が出力に漏れないこと（AC-02・AC-07・AC-28。ステップ 1-5・2-3）、拒否のエラーの文字列が入力の字幕の文字列を含まないこと（AC-15。ステップ 3-3）、書式に合わない入力を補正して受理しないこと（AC-04・AC-09・AC-12。ステップ 1-5・2-3・3-3）、病的な入力で解析が時間やメモリを使い尽くさないこと（AC-20。ステップ 4-1・4-2）。

### 4.4. テストヘルパー

新しいテストヘルパーのファイルは作らない。フェーズ 1〜4 のテストはすべて `internal/transcript` の既存の `copied_format_test.go`（`//go:build test`）に置き、同じパッケージの非公開の関数（`parseCopiedTranscript`）と既存の補助（`assertCopiedRejected`）を使う。`test_organization.md` の分類 A（`testutil/`）・分類 B（`test_helpers.go`）のどちらも要らない。`copiedParseAllocPerByte` は既存の `copied_format_race_test.go`・`copied_format_norace_test.go` の定数をそのまま使う。`cmd/yt2column` の guard は既存の `copied_transcript_docs_test.go` の補助（`claudeBullet`・`readDoc`）をそのまま使う。

### 4.5. 既存のテストとの互換

requirements AC-18 が認める変更は、architecture §3.5・§3.2.3 の一覧と、本計画のステップ 3-1・3-2・4-1 の更新だけである。`copied_format_test.go` の §1.3 の表のケースと、確かめている経路を通らなくなる 2 つのケースだけを書き換える。`copied_test.go`・`internal/writer/prompt_test.go`・`internal/pipeline` の guard・`TestExportedSentinelErrorsUnchanged`（`json3_test.go`）は内容を変えずに `make test` で通す。大きな入力のテスト（`TestParseCopiedLargeInputs`）はステップ 4-1 で入力を拡張し、`wantSegments` を更新する。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

「検証の種類」の `test` は、実行され、誤った振る舞いで失敗するテストを指す。`static` は、実行される guard・`make` ターゲットを指す。`manual` は、人が行う確認を指す。テストのパスは `path::TestName` の形で記す。表の新しいテストは §2 の各ステップで加える。

| AC | 検証の種類 | 検証 |
|---|---|---|
| AC-01 | test | `internal/transcript/copied_format_test.go::TestParseCopiedChapterHeadings` |
| AC-02 | test | `internal/transcript/copied_format_test.go::TestParseCopiedChapterHeadings` |
| AC-03 | test | `internal/transcript/copied_format_test.go::TestParseCopiedChapterHeadings` |
| AC-04 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（見出しの形に一致しない行） |
| AC-05 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（見出し行だけの入力の `errCopiedNoText`） |
| AC-06 | test | `internal/transcript/copied_format_test.go::TestParseCopiedChapterHeadings` |
| AC-07 | test | `internal/transcript/copied_format_test.go::TestParseCopiedTrailingArea` |
| AC-08 | test | `internal/transcript/copied_format_test.go::TestParseCopiedTrailingArea` |
| AC-09 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（ラベルに一致しない末尾の行） |
| AC-10 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（セグメントの間の操作ラベル行） |
| AC-11 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（操作ラベル行だけの入力の `errCopiedNoText`） |
| AC-12 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects` |
| AC-13 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects` |
| AC-14 | test / static | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（`assertCopiedRejected` が `ErrCopiedTranscriptFormat` と部分的な結果を返さないことを確かめる）、`internal/transcript/json3_test.go::TestExportedSentinelErrorsUnchanged`（公開の番兵の集合を固定する。static） |
| AC-15 | test | `internal/transcript/copied_format_test.go::TestParseCopiedErrorOmitsInput`、`internal/transcript/copied_test.go::TestNewCopiedSourceErrorOmitsInput` |
| AC-16 | test / static | `make test`（AC-01〜AC-15・AC-21〜AC-26・AC-28〜AC-33 の新しいテスト。`internal/transcript/copied_format_test.go::TestParseCopiedPrefix`・`::TestParseCopiedChapterHeadings`・`::TestParseCopiedTrailingArea`・`::TestParseCopiedRejects`・`::TestParseCopiedErrorOmitsInput`。test）、`internal/transcript/copied_test.go::TestCopiedSourceRealSamples`（`testdata/` のサンプルを 3 つに固定する）・`cmd/yt2column/copied_transcript_docs_test.go::TestCopiedTranscriptSurveyRecords`（保存した各サンプルの記録を確かめる。static） |
| AC-17 | static | `internal/transcript/copied_test.go::TestCopiedSourceRealSamples`・`cmd/yt2column/copied_transcript_docs_test.go::TestCopiedTranscriptSurveyRecords`（本タスクはサンプルを加えない）。得られなかったことの記録は architecture §7.1 にある |
| AC-18 | test | `make test`（`internal/transcript/copied_format_test.go` の `TestParseCopiedAccepts`・`::TestParseCopiedRejects`・`::TestParseCopiedLineBreaks`・`::TestParseCopiedCheckOrder`・`::TestParseCopiedErrorOmitsInput`・`::TestParseCopiedLargeInputs`、`internal/transcript/copied_test.go::TestCopiedSourceRealSamples`、`internal/writer/prompt_test.go::TestValidateTranscriptAcceptsCopiedSamples`、`internal/pipeline/pipeline_test.go` の guard を、変えたケース以外は変えずに通す） |
| AC-19 | manual / test | §2 の手動の確認 M-1（日時と結果を表に記録する。manual）、`make test`（M-1 と同じ振る舞いを確かめる `internal/transcript/copied_format_test.go::TestParseCopiedPrefix`・`::TestParseCopiedChapterHeadings`・`::TestParseCopiedTrailingArea` が通る。test） |
| AC-20 | test | `internal/transcript/copied_format_test.go::TestParseCopiedLargeInputs` |
| AC-21 | test | `internal/transcript/copied_format_test.go::TestParseCopiedTrailingArea` |
| AC-22 | test | `internal/transcript/copied_format_test.go::TestParseCopiedTrailingArea` |
| AC-23 | test | `internal/transcript/copied_format_test.go::TestParseCopiedTrailingArea` |
| AC-24 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（セグメントの間の空行の `errCopiedEmptyLine`）、`::TestParseCopiedPrefix`（先頭の空行の受理） |
| AC-25 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（`errCopiedBlank`）、`::TestParseCopiedCheckOrder`（`blank before empty line` が `\n\n` を `errCopiedBlank` で拒否する） |
| AC-26 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（末尾のタブの行の `errCopiedControlChar`） |
| AC-27 | static | `cmd/yt2column/copied_transcript_docs_test.go::TestClaudeMDCopiedTranscriptException`（`CLAUDE.md` の「Declare, don't infer」の項が新しい境界を述べていることを確かめる） |
| AC-28 | test | `internal/transcript/copied_format_test.go::TestParseCopiedPrefix` |
| AC-29 | test | `internal/transcript/copied_format_test.go::TestParseCopiedPrefix` |
| AC-30 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（前置きの数字・`:`・数字の連続と制御文字） |
| AC-31 | test | `internal/transcript/copied_format_test.go::TestParseCopiedRejects`（最初のセグメントの行がない入力の `errCopiedNoText`） |
| AC-32 | test | `internal/transcript/copied_format_test.go::TestParseCopiedPrefix` |
| AC-33 | test | `internal/transcript/copied_format_test.go::TestParseCopiedTrailingArea` |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| 前置きを取り除いたことで行が詰まり、拒否のエラーの行番号が入力とずれる | 利用者に誤った行を案内する | 検査する範囲だけを変え、行番号は除く前の入力の行番号を保つ（ステップ 1-3）。時刻の文法の拒否の既存のテストが理由と行番号を変えずに通ることを確かめる（ステップ 3-1） |
| 末尾の領域の判定が制御文字の行を取り込み、タブの行を黙って読み飛ばす | 制御文字の拒否が効かない | 制御文字を含む行で末尾の領域を終える（ステップ 1-1）。末尾のタブの行を `errCopiedControlChar` で拒否するテストを加える（AC-26。ステップ 1-5） |
| 形 B で本文の全体が操作ラベルと一致するセグメントを読み飛ばす | 本来のセグメントを黙って失う | 本文の位置（直前の行が時刻だけの行）の操作ラベル行を末尾の領域に含めない（ステップ 1-1）。AC-33 のテストを加え、除外を外すと失敗することを確かめる（ステップ 3-4・3-5） |
| 見出しの形を位置で推測し、未知の行を読み飛ばしてしまう | 利用者がパネル以外の文字列を送っても気づかない | 見出しの形は行全体の明示的な形に一致する行だけとし、それ以外は拒否する（ステップ 2-1・2-2。F-003） |
| 前置きの数字・`:`・数字の検査が、見出しのタイトルに `10:00` のような文字列を含む正しいパネルを拒否する | 正しい入力が `invalid_transcript` で拒否される | architecture §5.4 の残余のリスクとして受け入れる。頻度が高ければ前置きの拒否の規則を見直す（実装では変えない） |
| `make test` が `-race` 付きで、4 MiB の解析が 15 秒を超える | `make test` が不安定になる | ステップ 4-2 で時間と割り当てを測りコミットメッセージに記録する。超える場合は、解析が入力の長さに比例しているか（architecture §3.3）を確かめてから上限を見直す |
| 新しいテストの入力で、入力バイト当たりの割り当てが race なしの上限 `copiedParseAllocPerByte = 128`（`copied_format_norace_test.go:9`）を超える | `make test`（`-race`）は 8192 の上限を通るが、race なしの実行は 128 で失敗する。`make lint` の golangci-lint は型検査のみでテストを実行しない | ステップ 4-2 で race なしの実行も行い、上限内であることを記録する |

### 6.2. スケジュールのリスク

- M-1 は人の承認と実際のサーバ・拡張を要する。PR-1 は M-1 を待たずに実装・レビューできるが、マージの前に M-1 を行い、結果を §2 の表に記録する。PR-2（文書）の作業は M-1 を待たずに進められる。

### 6.3. セキュリティリスク

- 読み飛ばした前置き・見出しのタイトル・操作ラベルの内容は、`Segments`・プロンプト・記事・中間生成物・ログのいずれにも含めない（ステップ 1-1・1-2・2-2）。テストで、返る `Segment.Text` にこれらの文字列が含まれないことを確かめる（AC-02・AC-07・AC-28。ステップ 1-5・2-3）。
- 拒否のエラーの文字列には、入力の字幕の文字列を含めない（ステップ 3-3。AC-15）。
- 報告された入力と観測した入力の全体は `testdata/` に保存しない（AC-16・AC-17。`TestCopiedSourceRealSamples` がサンプルの集合を固定する）。

## 7. 実装チェックリスト (Implementation Checklist)

- [x] architecture にある「変更するファイル」の一覧を editorial correction として直した（§2.0）
- [ ] PR-1 マージ済み（対象ステップ: 1-1〜1-6 / 2-1〜2-4 / 3-1〜3-5 / 4-1〜4-2 / M-1）
- [ ] PR-2 マージ済み（対象ステップ: 5-1〜5-3）
- [ ] §5 のすべての AC の `test`・`static` の検証が実行され、通る
- [ ] M-1 の日時と結果を §2 の表に記録した
- [ ] `make fmt`・`make test`・`make lint` が PR-1・PR-2 の完了時に通る

## 8. 成功基準 (Success Criteria)

- **機能:** パネル全体の選択（前置き・形 A のチャプター見出し行・末尾のボタンを含む）が `invalid_transcript` で拒否されずに受理される（AC-01・AC-07・AC-08・AC-28・AC-29）。末尾に空行が入った入力が受理される（AC-21〜AC-23）。形 A の見出し行と末尾の領域の行が `Segments` に含まれない（AC-02・AC-07）。前置き・見出し行・末尾の領域を含まない入力の解析結果が変わらない（AC-18）。
- **品質:** `make test`・`make lint` が通る。AC-18 が認めるケースの外の既存のテストを変えていない。各テストが、対象の仕組みを壊して失敗することを確かめ、コミットメッセージに書いた。
- **セキュリティ:** 読み飛ばした行の内容が、`Segments`・プロンプト・記事・中間生成物・ログのいずれにも含まれず、拒否のエラーの文字列にも含まれない（requirements §4.2）。
- **保守性:** 見出しの形と操作ラベルの一覧が `internal/transcript` の 1 か所にあり、新しい操作ラベルを 1 行加えるだけで対応できる（requirements §4.5）。
- **性能:** 4 MiB の入力の解析が 15 秒以内に完走し、割り当てが入力の長さに比例する（AC-20）。
- **手動の確認:** 実際のサーバと拡張を通した送信が `202` で受理される（AC-19）。
- **文書:** `CLAUDE.md` の「Declare, don't infer」の例外の境界が前置き・形 A の見出し行・末尾の領域の読み飛ばしを述べ、`TestClaudeMDCopiedTranscriptException` がそれを確かめる（AC-27）。

## 9. 次のステップ (Next Steps)

- この計画をレビューし、`approved` にしたうえで、`/runplan 0024` で PR-1 から実装する。
- 実装中に architecture の記述が実際のコードと合わない箇所が見つかった場合は、本計画を独自に変えず、`requirements_process.md` の「Editing an approved document」に従って architecture を直す。
- 本タスクの後に、形 B の 2 つ目以降のチャプターの見出しと、末尾の領域を一覧の外に広げるかを、将来のタスクで扱う（requirements §7・architecture §9）。
