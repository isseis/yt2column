# 実装計画書：コピーした字幕に混じるパネルの行（前置き・チャプター見出し・末尾の操作ラベル）を読み飛ばす

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-18 |
| Review date | 2026-10-18 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定めた、コピーした字幕に混じるパネルの行（前置き・形 A と形 B のチャプターの見出し行・末尾の領域）の読み飛ばしを、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。ここで、**最初のセグメントの行**は入力の先頭から数えて時刻を読める最初の行、**前置き**はその行より前の行、**末尾の領域**は入力の末尾に続く空白文字だけの行と操作ラベル行の並びを指す（要件書 §6）。具体的には、次のことを行う。

-   `internal/transcript/copied_format.go` のパーサに、末尾の領域と前置きの決定、最初のセグメントの行による形の判定、元の入力の行番号の報告、設計書 3.3 の拒否の規則を加える（設計書 3.2.1・3.2.4・3.2.5・3.3）。
-   形 A の見出し行（設計書 3.2.2）と、形 B の時刻を期待する位置の見出し行（設計書 3.2.3）を読み飛ばす。
-   実データ `BkaUHsH_mkw` を `testdata/` に保存し、サンプルの集合を固定するテストと文書の guard を改める（設計書 3.5・7.2）。
-   4 MiB の入力のテストを加える（設計書 7.3）。
-   CLAUDE.md の「Declare, don't infer」の例外と `README.md` の案内を改める（設計書 1.3.1）。

本計画が使う記号は次のとおりである。AC-NN は要件書の受け入れ基準、F-NNN は要件書の機能要件を指す。「§N」は本計画の節、「設計書 N」は設計書の節を指す。ほかの文書の節は、文書名を前に付けて示す（例: 要件書 §1.1）。

本タスクには `design_handoff.md` も `implementation_handoff.md` もない（2026-10-18、コミット `5d9df51` の `docs/tasks/0024_copied_transcript_panel_lines/` で確認した）。

フェーズは設計書 8 章の実装優先順位 1〜6 にそのまま対応させる。

### 1.2. 実装原則

-   設計書 1.1 の 5 つの設計原則に従う。特に次の 3 点を守る。読み飛ばす行は明示的に定義した行だけにする。読み飛ばす範囲は、時刻の文法から決まる位置で定める。読み飛ばした行の内容を、`Segments`・エラーメッセージ・ログのどこにも出さない。
-   公開の型・関数・番兵を変えない（設計書 3.1・4.1）。変更は `internal/transcript` の非公開の関数と値に限る。
-   時刻の文法（`parseTimestampPrefix`・`parseTimestampLine`）、読み上げ用ラベルの照合、`checkCopiedLine`・`hasCopiedControlChar`・`readableAsFormA` を再実装せず、そのまま使う（設計書 1.3・3.4）。
-   Go のコメント・識別子・文字列リテラルは英語で書く。例外は、パネルの UI が出力する日本語のラベル（`チャプター`・`動画の時間と同期`）との比較に使う値である。これらは既存の `jpUnitSecond` などと同じく比較の対象なので日本語のまま置き、その理由をコメントに書く。テスト用に合成した入力に含める日本語の文字列（`タイムライン` など）も、パーサが受け取る入力の値なので日本語のままとする。
-   `AC-NN`・`F-NNN` はソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
-   各テストがその理由で失敗しうることは、対象の処理を壊して確かめ、コミットメッセージに書く（CLAUDE.md「Testing Strategy」）。本計画では、その確認を各フェーズのタスクとして置く。
-   読み飛ばしのテストは、読み飛ばす行を取り除いた入力（0015 の書式の行だけの入力）と `Segments` を比べる形にする（設計書 7.1）。比較の両辺は書き換えたパーサを通るので、取り除いた入力の `Segments` もリテラルの期待値（`[]Segment`）と比べる。

### 1.3. 既存コード調査結果

コミット `5d9df51`（作業ツリーがクリーンな `issei/transcript-format-04`）で確認した。設計書の行番号はコミット `cb4f268` のもので、`internal/transcript/`・`cmd/yt2column/copied_transcript_docs_test.go`・`internal/writer/prompt_test.go` は `cb4f268` から変わっていない（`git diff --stat cb4f268 HEAD -- internal/ cmd/` で確認）。`README.md` だけは行がずれている（下表）。

| 領域 | 既存のもの | 足りないもの・変えるもの |
|---|---|---|
| パーサの入口 | `parseCopiedTranscript`（`copied_format.go:86-102`）。形を `lines[0]` で判定する（`:98`） | 末尾の領域と前置きの決定を加え、最初のセグメントの行で形を判定する。関数のコメントの `decides which by the first line alone` を改める |
| 行の分割 | `splitCopiedLines`（`:148-157`）。最後の空の要素を 1 つ取り除く | 振る舞いは変えない（設計書 3.2.4）。コメントの `a second or later trailing separator does, and is caught by the empty-line check` を改める |
| 形 A | `parseCopiedFormA(lines []string)`（`:304-343`）。行番号は `index + 1` | 本体の範囲を受け取り、元の行番号を報告する。見出しの形の判定を `checkCopiedLine` の後に加える。コメントの `fixed by the first line` を改める |
| 形 B | `parseCopiedFormB(lines []string)`（`:351-386`）。時刻を期待する位置の非時刻行を `:370-372` で拒否する | 本体の範囲を受け取る。`:370-372` を設計書 3.3 の拒否の規則と見出しの読み飛ばしに変える。コメントの `so a chapter heading there is rejected` を改める |
| 行の検査 | `checkCopiedLine`（`:280-288`）・`hasCopiedControlChar`（`:290-297`） | 変えない。前置きと末尾の領域では `hasCopiedControlChar` だけを使う |
| 拒否の理由 | 非公開の番兵 11 個（`:16-28`）。`isCopiedReason`（`copied_format_test.go:657-676`）がこの集合を列挙する | 変えない。新しい理由は要らない（設計書 4.1） |
| 公開の番兵 | `TestExportedSentinelErrorsUnchanged`（`internal/transcript/json3_test.go:469-500`）が公開の番兵の集合を固定する | 変えない。AC-14 の静的な確認に使う |
| 拒否のテスト | `assertCopiedRejected`（`copied_format_test.go:463-476`）が `ErrCopiedTranscriptFormat`・理由・`segments != nil` を確かめる | 新しい拒否のテストもすべてこの関数を通す |
| 既存のテストの書き換え | 設計書 3.6 の表のケース | §2 の書き換えの台帳（§2.7）のとおり |
| 4 MiB の入力 | `TestParseCopiedLargeInputs`（`:496-535`）と 3 つの組み立て関数（`:537-551`）。`copiedParseAllocPerByte` は race で 8192、norace で 128（`copied_format_race_test.go:14`・`copied_format_norace_test.go:9`） | 4 つの入力と組み立て関数を加え、関数のコメントを改める |
| fuzz | `FuzzNewCopiedSource`（`:559-593`）。サンプルの数を `len(copiedSampleFiles)` で確かめる（`:564`） | 設計書 7.1 の性質 (a)・(b) と種の入力を加える。サンプルの数の確認は `copiedSampleFiles` に追随するので変えない |
| 実データ | `copiedSampleFiles`（`copied_test.go:344-348`。並びは `copiedFileNames` がソートした順）、`TestCopiedSourceRealSamples`（`:364-424`）、`checkCopiedSampleSegments`（`:428-459`） | `BkaUHsH_mkw.transcript_copy.txt` を先頭に加え（`B` は `I` より前）、期待値と見出しの前後の確認を加える |
| writer の連携 | `TestValidateTranscriptAcceptsCopiedSamples` の `const wantSamples = 3`（`internal/writer/prompt_test.go:342`） | `4` にする |
| 文書の guard | `TestCopiedTranscriptSurveyRecords`（`cmd/yt2column/copied_transcript_docs_test.go:25-106`）、「保存する」の確認は `:63-73`。`TestClaudeMDCopiedTranscriptException`（`:166-186`）。`markdownBullet`（`:319-341`）は空行で項目を終える | 前者は属性の表の一覧を読む。後者に確認の語を加える。CLAUDE.md の追記は空行を入れずに同じ項目の中に書く |
| サンプルの記録 | `testdata/README.md` の「コピーした字幕」の節。共通の項目（取得日 2026-10-14・取得方法・「見出しの行を含まない」）を 3 ファイルに当てている | 共通の項目を既存の 3 ファイルに限ると明記し、`BkaUHsH_mkw` の項目を別に書く |
| 利用者向けの案内 | `README.md:599-601`（`cb4f268` では `:513-515`）の `select only the caption lines — a trailing blank line is not part of the accepted format and the server rejects it.` | 改める（§2 フェーズ 6） |
| サーバのテスト | `TestRejectInvalidTranscript`（`internal/server/request_test.go:180`）の 3 つの入力、`TestServerEndToEnd`（`cmd/yt2column-server/server_test.go:47`）の `not a copied transcript`、`internal/server/handler_test.go:206` の `not a transcript`、`TestNewCopiedSource`（`copied_test.go:38`）の `not a transcript\n` | 変えない。どれも本設計でも拒否される（前置きだけの入力は `errCopiedNoText`、`0:24MARKER_BODY` は形 A の `errCopiedLabelMismatch`、`0:00\n` は `errCopiedNoText`）。`make test` で確かめる |

-   **型による分岐:** 本設計は具体的な型で振る舞いを分けない。加える非公開の値（形 A の見出しの形・操作ラベルの一覧）は `copied_format.go` の中だけで使う。構築の経路の列挙は要らない。
-   **サンプルの元:** 2026-10-18 の取得物の候補が、リポジトリの外の `/private/var/folders/l2/sgkwbrrj3c12cvhfnfjbwt9r0000gn/T/opencode/BkaUHsH_mkw.panel.txt` にある（11775 バイト・307 行・末尾の改行なし・時刻だけの行 152・見出し 3 行が行 125・182・241、SHA-256 `aa417bda4f11c80c626a9a6649131fe01f8686823df9c9fc3ee76c639d13fa26`。本計画の作成時に `wc`・`grep`・`shasum` で確認）。OS の一時ディレクトリにあるので、消えることがある（§6.2）。この取得物を保存してよいかどうかと、取得の方法と条件は、取得者本人に確かめる（§2 フェーズ 4）。
-   **外部の前提の確認:** `unicode.IsSpace` は Unicode の White_Space の性質に従い、Latin-1 では `\t`・`\n`・`\v`・`\f`・`\r`・U+0020・U+0085・U+00A0 を真とする（`go doc unicode.IsSpace`、Go は `go.mod` の版）。要件 F-006 の「Unicode の White_Space」と一致する。

## 2. 実装ステップ (Implementation Steps)

各フェーズの完了の条件は、特に書かない限り `make fmt`・`make test`・`make lint` が通ることである。

### フェーズ 1: 末尾の領域・前置き・形の判定・行番号

設計書 3.2.1・3.2.4・3.2.5・3.3・3.4。フェーズ 1 は 2 つの PR に分ける。前半（ステップ 1-1〜1-10）で末尾の領域と元の行番号の報告を、後半（ステップ 1-11〜1-20）で前置きと最初のセグメントの行による形の判定を加える。

**変更するファイル:** `internal/transcript/copied_format.go`、`internal/transcript/copied_format_test.go`

-   [x] **ステップ 1-1**: 空白文字だけの行（1 文字以上、すべて `unicode.IsSpace`）の判定と、操作ラベルの一覧（`動画の時間と同期`・`Sync to video time`）を非公開の値として 1 か所に置く（設計書 3.2.4）。一覧のコメントに、比較の対象なので日本語のまま置く理由と出典を英語で書く。
-   [x] **ステップ 1-2**: 末尾の領域を決める処理を加える。直前の行が時刻だけの行である操作ラベル行で領域を終える（設計書 3.2.4）。
-   [x] **ステップ 1-3**: `parseCopiedFormA`・`parseCopiedFormB` が、行の並びと本体の範囲を受け取り、元の行番号を報告するようにする（設計書 3.4）。行を詰め直さない。
-   [x] **ステップ 1-4**: `parseCopiedTranscript` に、設計書 3.2.5 の手順 4（末尾の領域の決定）・手順 8（末尾の領域の制御文字の検査）・手順 9 を加える。手順 8 は本体の検査の後に置く。今は `parseCopiedFormA`・`parseCopiedFormB` の中にある、本文のあるセグメントがあるかの検査（`copied_format.go:339-341`・`:382-384`）を、手順 8 の後（手順 9）に移す。形の判定は、この時点では本体の最初の行で行う（ステップ 1-13 で最初のセグメントの行に変える）。末尾の領域が入力のすべての行を覆い本体が空のときは、手順 8 の前に `errCopiedNoText`（行番号なし）で拒否する（設計書 3.2.5 の、最初のセグメントの行がなければ手順 8 を行わない扱いと同じ）。
-   [x] **ステップ 1-5**: `splitCopiedLines` のコメントの `a second or later trailing separator does, and is caught by the empty-line check.` を「残る空の行は末尾の領域に入る」旨に変える。
-   [x] **ステップ 1-6**: §2.7 の台帳のうち、ステップ 1-6 の行を書き換える。
-   [x] **ステップ 1-7**: 新しいテストを加える。対象は AC-07・AC-09・AC-10・AC-21〜AC-23・AC-26・AC-33。拒否のテストは `assertCopiedRejected` を通す。AC-07 は日本語 UI と英語 UI（`Sync to video time`）の両方を含める。§2.7 で書き換えるケースと入力が同じになる AC（AC-22・AC-23 の空行だけの末尾）は、書き換えたケースを §5 の確認に対応させ、同じ入力のケースを新しく作らない。AC-23 の形 A の側は形 B の `three trailing separators` と入力が違うので別に加える（`TestParseCopiedTrailingRegion` の `form A two trailing empty lines` が 2 個以上の空行、`form A mixed empty lines and operation label` が空行と操作ラベルの混在）。本体が空の入力（操作ラベル行だけの `動画の時間と同期\n`。AC-11 の一部）の `errCopiedNoText` も加える。
-   [x] **ステップ 1-8**: `TestParseCopiedCheckOrder` に、手順 8 が手順 9 より先であるケース（`0:000 秒\n\t\n` が `errCopiedNoText` でなく行 2 の `errCopiedControlChar`）と、本体が空なら手順 8 を行わないケース（`動画の時間と同期\n\t\n` が `errCopiedControlChar` でなく `errCopiedNoText`）を加える。
-   [x] **ステップ 1-9**: `FuzzNewCopiedSource` に設計書 7.1 の性質 (a)（末尾の空白行と操作ラベル行）を加え、関数のコメントの性質の数（`checks four properties`）を改める。種の入力に、末尾の領域を含む入力を加える。足す空白文字だけの行は、`unicode.IsControl` でも真になる文字（`\t`・`\v`・`\f`・U+0085）を使わない。足す行の区切りは入力の区切りに合わせ、区切りのない 1 行の入力には LF を使う。
-   [x] **ステップ 1-10**: 次の壊し方をそれぞれ試し、名前を挙げたテストが失敗することを確かめて、結果をコミットメッセージに書く。
    -   (i) 末尾の領域の「直前が時刻だけの行」の条件を外す → AC-33 のテスト。
    -   (ii) 手順 8 を外す → AC-26 のテスト。
    -   (iii) 手順 8 と手順 9 の順を入れ替える → `TestParseCopiedCheckOrder` の `0:000 秒\n\t\n` のケース。
    -   (iv) 拒否の経路で `nil` でない `[]Segment` を返す → `assertCopiedRejected` を通るテスト。
    -   (v) 末尾の領域の処理を外す → 種の入力での性質 (a)。
    -   (vi) 本体が空のときの `errCopiedNoText` を外し、手順 8 へ進める → `TestParseCopiedCheckOrder` の `動画の時間と同期\n\t\n` のケース。

**PR-1 の成功の条件:** §2.7 の台帳のステップ 1-6 の行以外の既存のケースが変更なしで通る（AC-18）。

### PR-1 作成ポイント: trailing region and original line numbers

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10

**推奨タイトル**: `feat(0024): skip the trailing region of a copied transcript`

**レビュー観点**: 末尾の領域が空白文字だけの行と一覧の操作ラベル行だけからなり、直前が時刻だけの行である操作ラベル行を含めず形 B の本文として残すこと / 末尾の領域の制御文字の検査が本体の検査の後・本文の有無の検査の前に起こること（設計書 3.2.5 の手順 8・9） / 形ごとの解析が範囲を受け取っても、拒否の行番号が元の入力の行番号のままであり、§2.7 のステップ 1-6 の行以外の既存のケースの理由と行番号が変わらないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-3・1-4 は、形ごとの解析が受け取る範囲と行番号、および入口の検査の順序（本文の有無の検査の位置）を同時に組み替える複雑な手順で、既存の拒否の理由と行番号を保つ必要があるため。

-   [x] グリーンゲート（`make test && make lint`。`_context.md` の "Green gate"）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

-   [x] **ステップ 1-11**: 数字・`:`・数字の連続を含むかを返す非公開の述語を 1 つ加える（設計書 3.3）。前置きとステップ 3-1 の形 B の見出しの位置で共有する。
-   [x] **ステップ 1-12**: 前置きを決める処理を、設計書 3.2.1 の (1)〜(4) の順で加える。最初のセグメントの行が見つからないときは `errCopiedNoText` で拒否する。
-   [x] **ステップ 1-13**: `parseCopiedTranscript` に設計書 3.2.5 の手順 5（前置き）と手順 6（形の判定）を加える。前置きの走査は末尾の領域の直前で止める。形は最初のセグメントの行で判定する。
-   [x] **ステップ 1-14**: 関数のコメントを改める。`parseCopiedTranscript` の `decides which by the first line alone` を「最初のセグメントの行で判定し、前置きと末尾の領域を読み飛ばす」旨に、`parseCopiedFormA` の `The label's language is fixed by the first line.` を「本体の最初の行で固定する」旨に変える。
-   [x] **ステップ 1-15**: §2.7 の台帳のうち、ステップ 1-15 の行を書き換える。
-   [x] **ステップ 1-16**: 新しいテストを加える。対象は AC-11・AC-24・AC-28〜AC-32 と、前置きのある入力での本体の拒否の行番号（AC-15）。拒否のテストは `assertCopiedRejected` を通す。AC-31 の `タイムライン\n` は、§2.7 で書き換える `leading display items` を §5 の確認に対応させ、同じ入力のケースを新しく作らない。
-   [x] **ステップ 1-17**: `TestParseCopiedCheckOrder` に、前置きの走査が末尾の領域に入らないケース（`文字起こし\n\t\n` が `errCopiedControlChar` でなく `errCopiedNoText`）を加える。
-   [x] **ステップ 1-18**: `TestParseCopiedErrorOmitsInput` に、前置きの拒否（数字・`:`・数字の連続の行、制御文字の行）で目印の文字列がメッセージに入らないケースを加える（AC-15）。
-   [x] **ステップ 1-19**: `FuzzNewCopiedSource` に設計書 7.1 の性質 (b)（前置き）を加え、関数のコメントの性質の数を改める。種の入力に、前置きを含む入力を加える。足す行の区切りは入力の区切りに合わせ、区切りのない 1 行の入力には LF を使う。
-   [x] **ステップ 1-20**: 次の壊し方をそれぞれ試し、名前を挙げたテストが失敗することを確かめて、結果をコミットメッセージに書く。
    -   (i) 前置きの処理の (2) と (3) の順を入れ替える → AC-28 のテスト。
    -   (ii) 数字・`:`・数字の述語が常に偽を返す → AC-30 と `leading BOM` などの時刻の文法の拒否のケース。
    -   (iii) 前置きの走査を末尾の領域まで進める → `TestParseCopiedCheckOrder` の `文字起こし\n\t\n` のケース。
    -   (iv) 前置きの処理を外す → 種の入力での性質 (b)。

**PR-2 の成功の条件:** §2.7 の台帳のステップ 1-6・1-15 の行以外の既存のケースが変更なしで通る（AC-18）。

### PR-2 作成ポイント: prefix and form detection by the first segment line

**対象ステップ**: 1-11 / 1-12 / 1-13 / 1-14 / 1-15 / 1-16 / 1-17 / 1-18 / 1-19 / 1-20

**推奨タイトル**: `feat(0024): skip the prefix before the first segment line`

**レビュー観点**: 前置きの各行が設計書 3.2.1 の (1)〜(4) の順で扱われ、数字・`:`・数字の連続を含む行と制御文字の行を読み飛ばさずに拒否すること / 時刻の文法の拒否のケース（`leading BOM` など）が理由と行番号を変えずに通ること / 前置きの走査が末尾の領域の直前で止まり、形の判定が最初のセグメントの行で行われること / 前置きのある入力の拒否の行番号が元の入力の行番号であること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-12・1-13 は、内容にかかわらず行を読み飛ばす前置きの範囲を時刻の文法から決める手順で、(2) と (3) の順や述語を誤るとセグメントを黙って失うため（設計書 5.4）。

-   [x] グリーンゲート（`make test && make lint`。`_context.md` の "Green gate"）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 形 A のチャプターの見出し行

設計書 3.2.2。

**変更するファイル:** `internal/transcript/copied_format.go`、`internal/transcript/copied_format_test.go`

-   [x] **ステップ 2-1**: 形 A の見出しの形（`チャプター`・`Chapter` + U+0020 + `[1-9][0-9]*` + `:` + U+0020 + タイトル）を非公開の値として 1 か所に置く。新しい表記は、見出しの形を 1 つ加えるだけで対応できるようにする（要件書 §4.5）。タイトルの最後の文字の判定は `unicode.IsSpace` で行う（`regexp` の `\S` を使わない。設計書 3.2.2）。
-   [x] **ステップ 2-2**: `parseCopiedFormA` の本体の行の検査で、`checkCopiedLine` の後、時刻の接頭辞の読み取りの前に、見出しの形に一致する行を読み飛ばす。見出し行は言語の固定と時刻の順序に影響させない。
-   [x] **ステップ 2-3**: 新しいテストを加える。対象は AC-01〜AC-06（AC-05 は見出し行だけの入力の `errCopiedNoText`）と、形 A の側の AC-12・AC-13、および前置き・見出し行・末尾の領域を含む形 A のパネル全体の選択（AC-19 の自動の側）。AC-04 の拒否のケースには、U+3000 で終わるタイトル（`\S` では一致してしまう行）を含める。
-   [x] **ステップ 2-4**: `TestParseCopiedErrorOmitsInput` に、見出しの形に一致しない形 A の行（目印を含むもの）の拒否のケースを加える（AC-15）。
-   [x] **ステップ 2-5**: 次の壊し方をそれぞれ試し、名前を挙げたテストが失敗することを確かめて、結果をコミットメッセージに書く。見出しの判定を外す → AC-01〜AC-03・AC-06 のテストと、形 A のパネル全体の選択のケース。最後の文字の判定を `\S` 相当（ASCII の空白だけを空白と見なす判定）に変える → U+3000 で終わるタイトルのケース。番号の `[1-9]` を `[0-9]` にする → `チャプター 0`・`チャプター 01` のケース。

### PR-3 作成ポイント: form A chapter headings

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0024): skip form A chapter heading lines`

**レビュー観点**: 見出しの形が行全体の一致で、番号が `[1-9][0-9]*`、タイトルの最後の文字の判定が `\S` でなく `unicode.IsSpace` であること / 見出しの判定が `checkCopiedLine` の後・時刻の接頭辞の読み取りの前にあり、言語の固定と時刻の順序に影響しないこと / 見出しの形に一致しない行（AC-04 の各行）と本体の未知の行を、形 A では引き続き拒否すること

**実装モデル要件**: standard

**判定理由**: 設計書 3.2.2 が見出しの形と判定の位置を確定しており、状態の変更も外部の資源もないため、どの契機にも当たらない。

-   [x] グリーンゲート（`make test && make lint`。`_context.md` の "Green gate"）がパスしていることを確認した
-   [ ] PR を作成した
-   [ ] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 形 B のチャプターの見出し行

設計書 3.2.3。ステップ 1-11 の数字・`:`・数字の述語を使う。

**変更するファイル:** `internal/transcript/copied_format.go`、`internal/transcript/copied_format_test.go`

-   [ ] **ステップ 3-1**: `parseCopiedFormB` の時刻を期待する位置（`copied_format.go:370-372` の `!awaitingText` の分岐）を、設計書 3.2.3 の手順 3・4 に変える。空白文字だけの行と、数字・`:`・数字の連続を含む行は `errCopiedNotTimestamp` で拒否する。それ以外の行は見出しとして読み飛ばし、`awaitingText` は偽のままにする。
-   [ ] **ステップ 3-2**: `parseCopiedFormB` のコメントの `The line after a text line must be a timestamp, so a chapter heading there is rejected.` を、「時刻を期待する位置の非時刻行は、拒否の規則に当たらなければ見出しとして読み飛ばす」旨に改める。
-   [ ] **ステップ 3-3**: §2.7 の台帳のうち、ステップ 3-3 の行を書き換える。`chapter heading omits the line` は、入力を、設計書 3.3 の規則で拒否される形 B の見出しの位置の行（設計書 3.6）に差し替え、拒否のメッセージに目印が入らないことを確かめる（AC-15）。
-   [ ] **ステップ 3-4**: 新しいテストを加える。対象は AC-08（前置き・形 B の見出し・末尾を持つパネル全体の選択）・AC-35〜AC-38 と、設計書 5.4 の前置きの終わりの拒否の経路（`errCopiedTimestampOrder`）。AC-35 には、形 A では拒否される行（`タイムライン`・`チャプター: タイトル`・`動画の時間と同期について話します`）を形 B の時刻を期待する位置に置くケースを含める（AC-04・AC-09・AC-12・AC-13 の形 B の側）。AC-36 は設計書 3.3 の表の形 B の列の 4 行それぞれを含める。
-   [ ] **ステップ 3-5**: 次の壊し方をそれぞれ試し、名前を挙げたテストが失敗することを確かめて、結果をコミットメッセージに書く。見出しの読み飛ばしを外す → AC-35・AC-08 のテスト。手順 3 を外す → AC-36 の空白文字だけの行と数字・`:`・数字の行、`bad timestamp at a form B position`・`form A line where form B wants a timestamp`。見出しを読み飛ばした後に `awaitingText` を真にする → 連続する 2 行の見出しのケース。

### PR-4 作成ポイント: form B chapter headings by position

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0024): skip form B chapter headings at timestamp-expected positions`

**レビュー観点**: 時刻を期待する位置の非時刻行のうち、空白文字だけの行と数字・`:`・数字の連続を含む行を `errCopiedNotTimestamp` で拒否し、それ以外だけを読み飛ばすこと / 見出しを読み飛ばした後も `awaitingText` が偽のままで、連続する見出しと本文を期待する位置の扱い（AC-37）が設計書 3.2.3 のとおりであること / `chapter heading omits the line` の書き換えで、受理の側と拒否のメッセージの側の両方を確かめていること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-1 は、形 B の位置の状態（`awaitingText`）の遷移に、0015 §3.2.9 の方針を反転する読み飛ばしを加える状態機械の変更で、誤るとセグメントを黙って失うため。

-   [ ] グリーンゲート（`make test && make lint`。`_context.md` の "Green gate"）がパスしていることを確認した
-   [ ] PR を作成した
-   [ ] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 実データ `BkaUHsH_mkw`

設計書 3.5・7.2。

**変更するファイル:** `testdata/BkaUHsH_mkw.transcript_copy.txt`（新規）、`testdata/README.md`、`internal/transcript/copied_test.go`、`internal/writer/prompt_test.go`、`cmd/yt2column/copied_transcript_docs_test.go`

-   [ ] **ステップ 4-1**: 取得者に、2026-10-18 の取得物（§1.3 の候補）と、取得の方法（パネルの選択を手でコピーしたものか、プログラムで抽出したものか）と条件（ブラウザとその版、ログインの有無、選択の範囲）を確かめる。手でのコピー（`window.getSelection().toString()` と同じ文字列になる方法）でなければ、取り直す。確認が取れるまでファイルを保存しない。
-   [ ] **ステップ 4-2**: 確かめたファイルを加工せずに `testdata/BkaUHsH_mkw.transcript_copy.txt` に複写する。複写の後に SHA-256 とバイト数が元のファイルと一致することを確かめる。取得者を特定できる情報を含まないことを確かめ、PR の説明に確かめた内容を書いて、レビューで人間がもう一度確かめられるようにする（`docs/dev/security.md` §7）。
-   [ ] **ステップ 4-3**: `testdata/README.md` の「コピーした字幕」の節を改める。冒頭の `3 ファイルとも` を 4 ファイルに合わせ、共通の項目（取得日 2026-10-14・取得方法・見出しを含まないこと）は既存の 3 ファイルのものと明記する。`BkaUHsH_mkw` について、AC-17 が挙げる項目（動画タイトル・URL・チャンネル・ライセンスとその URI・取得日 2026-10-18・取得方法・加工内容なし・見出し 3 行を含むこと・末尾の改行がないこと）を書く。値は動画のページと取得者の申告から書き写す。
-   [ ] **ステップ 4-4**: `copiedSampleFiles` に `BkaUHsH_mkw.transcript_copy.txt` を加え、`TestCopiedSourceRealSamples` に期待値を加える。期待値はファイルを読んで決め、パーサの出力から取らない（`copiedSample` のコメントの方針）。
-   [ ] **ステップ 4-5**: `checkCopiedSampleSegments` に `BkaUHsH_mkw` の確認を加える。ファイルから見出しの 3 行を取り除いた入力を解析した `Segments` と等しいこと（152 要素）、見出しの直前・直後のセグメントの `StartMs` と `Text` が対応するセグメントと等しいこと（AC-34）。取り除いた入力が見出しの行を含まないことを先に確かめる。
-   [ ] **ステップ 4-6**: `TestValidateTranscriptAcceptsCopiedSamples` の `const wantSamples = 3` を `const wantSamples = 4` にする。
-   [ ] **ステップ 4-7**: `TestCopiedTranscriptSurveyRecords` の「保存する」の確認（`:63-73`）を、0015 `02_architecture.md` §1.4.1 の表と、要件書 §1.1（見出し `### 1.1. 本タスクで保存するサンプルの記録`）の表の一覧から読む形に変える（設計書 3.5）。要件書のパスの定数を `copiedTranscriptArchitecturePath` の隣に加える。0015 の表に対する他の検査は変えない。関数のコメントを、一覧の表から読む旨に改める。
-   [ ] **ステップ 4-8**: 次の壊し方をそれぞれ試し、名前を挙げたテストが失敗することを確かめて、結果をコミットメッセージに書く。試した変更は元に戻す。要件書 §1.1 の表の `BkaUHsH_mkw` の行の `testdata/` 列を「保存しない」に変える、一覧から要件書の表を外す → `TestCopiedTranscriptSurveyRecords`。見出しの読み飛ばし（ステップ 3-1）を外す → `TestCopiedSourceRealSamples/BkaUHsH_mkw`。

**成功の条件:** `make test` が通る。`TestFakesCarryBuildTag` などのパイプラインの guard が変更なしで通る（AC-18）。

### PR-5 作成ポイント: BkaUHsH_mkw real sample

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8

**推奨タイトル**: `test(0024): add the BkaUHsH_mkw copied-transcript sample`

**レビュー観点**: サンプルが手でのコピーで取得され、加工されずに保存されていること（SHA-256 とバイト数の一致） / `testdata/README.md` の出典・ライセンス・取得日・取得方法・見出し 3 行・末尾の改行の記録が、既存の 3 ファイルの共通の項目と混ざらないこと / `TestCopiedSourceRealSamples` の期待値がパーサの出力でなくファイルから決められ、見出しを取り除いた入力との比較が AC-34 を確かめること / `TestCopiedTranscriptSurveyRecords` が 0015 の表を書き換えずに、要件書 §1.1 の表から「保存する」を読むこと

**実装モデル要件**: standard

**判定理由**: サンプルの保存と既存のテスト・guard の期待値の更新で、どの契機にも当たらない。ステップ 4-1・4-2 の確認（取得の方法・ライセンス・取得者を特定できる情報を含まないこと）は、`docs/dev/security.md` §7 の既存の手順をそのまま当てるもので、新しい安全の関門や移行の手順ではない。取得の方法は取得者が申告し、PR のレビューで人間がもう一度確かめる。

-   [ ] グリーンゲート（`make test && make lint`。`_context.md` の "Green gate"）がパスしていることを確認した
-   [ ] PR を作成した
-   [ ] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 4 MiB の入力

設計書 7.3。フェーズ 2・3 の後に行う。

**変更するファイル:** `internal/transcript/copied_format_test.go`

-   [ ] **ステップ 5-1**: `TestParseCopiedLargeInputs` に設計書 7.3 の 4 つの入力（末尾の 0 文字の行、1 バイトの前置きの行、形 A の見出し行とセグメントの交互、形 B の本文の後の 1 バイトの見出し行）と、それぞれの組み立て関数を加える。各入力は 4 MiB 以下で、期待するセグメントの数を持つ。
-   [ ] **ステップ 5-2**: 関数のコメントの `parses three synthetic 4 MiB inputs: the shortest form A line repeated (the most segments), one form A line nearly 4 MiB long, and many textless form B timestamps followed by one text.` を、7 つの入力とそれぞれが走査する経路を述べる形に改める。
-   [ ] **ステップ 5-3**: `make test`（race あり、`copiedParseAllocPerByte` は 8192）と、race なしの `go test -tags test -run TestParseCopiedLargeInputs ./internal/transcript`（同 128）の両方で、既存の 3 つと新しい 4 つを合わせた 7 つの入力が期限とメモリの上限に収まることを確かめ、両方の `t.Logf` の値をコミットメッセージに書く。

### PR-6 作成ポイント: 4 MiB input tests

**対象ステップ**: 5-1 / 5-2 / 5-3

**推奨タイトル**: `test(0024): cover the new skip paths with 4 MiB inputs`

**レビュー観点**: 4 つの入力がそれぞれ設計書 7.3 の走査の経路（末尾の領域・前置き・形 A の見出し・形 B の見出し）を通り、4 MiB 以下であること / race あり・なしの両方で期限と `copiedParseAllocPerByte` に収まり、その値がコミットメッセージにあること

**実装モデル要件**: standard

**判定理由**: 既存の `TestParseCopiedLargeInputs` に入力を足すだけのテストの変更で、どの契機にも当たらない。

-   [ ] グリーンゲート（`make test && make lint`。`_context.md` の "Green gate"）がパスしていることを確認した
-   [ ] PR を作成した
-   [ ] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: CLAUDE.md・README・手動の確認

設計書 1.3.1（例外 5 と CLAUDE.md への記載）・7.5。フェーズ 2・3 の後に行う。フェーズ 6 は 2 つの PR に分ける。ステップ 6-1〜6-5（文書）と、利用者の承認と実際のサービスを要するステップ 6-6（手動の確認）である。

**変更するファイル:** `CLAUDE.md`、`cmd/yt2column/copied_transcript_docs_test.go`、`README.md`、本計画（§5.1）

-   [ ] **ステップ 6-1**: CLAUDE.md の「Declare, don't infer」の Exception の文に、設計書 1.3.1 の 4 種類の読み飛ばしと、例外の境界との関係の 3 点を英語で加える。空行を入れずに同じ項目の続きとして書く（`markdownBullet` は空行で項目を終える）。既存の Boundary の文（`an input that matches neither form is rejected, not accepted by widening the interpretation`）は変えない。
-   [ ] **ステップ 6-2**: `TestClaudeMDCopiedTranscriptException` の確認の語に、設計書 1.3.1 の `first segment line`・`timestamp-expected position`・`chapter heading`・`trailing`・`explicitly defined` と、前置きと形 A の見出しをそれぞれ指す語（`prefix`・`Chapter N: `）を加える。後の 2 語は設計書 1.3.1 の語の一覧にないが、4 種類の読み飛ばしのそれぞれを 1 語以上で確かめるために加える。関数のコメントの `states the copied-transcript exception's scope, reason, and boundary` を、読み飛ばしの範囲も確かめる旨に改める。
-   [ ] **ステップ 6-3**: `README.md:599-601` の文 `The selection is sent exactly as copied from the transcript panel; the extension does not trim it, so select only the caption lines — a trailing blank line is not part of the accepted format and the server rejects it.` を、パネル全体を選んでよいこと、読み飛ばす行（前置き・チャプターの見出し・末尾の空白行と `Sync to video time`／`動画の時間と同期`）、それでも拒否される行（形 A の本体の未知の行、本体の空行、時刻に似た文字列を含む前置きと形 B の見出し）を述べる英語の文に改める。内容が設計書 3.2・3.3 と合うことを、PR の説明に対応する節を挙げて示す。
-   [ ] **ステップ 6-4**: `make ext-test` を実行する（README の拒否の理由の表を `serverReasons.test.ts` が読むため）。
-   [ ] **ステップ 6-5**: CLAUDE.md から加えた語を 1 つずつ消し、そのたびに `TestClaudeMDCopiedTranscriptException` が失敗することを確かめて、結果をコミットメッセージに書く。

### PR-7 作成ポイント: documentation of the skipped panel lines

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5

**推奨タイトル**: `docs(0024): document the skipped panel lines`

**レビュー観点**: CLAUDE.md の追記が「Declare, don't infer」の項目の中に空行なしで入り、既存の Boundary の文を変えないこと / `TestClaudeMDCopiedTranscriptException` の語が 4 種類の読み飛ばしと境界の 3 点をそれぞれ確かめること / README の案内が設計書 3.2・3.3 の受理と拒否の範囲と一致すること

**実装モデル要件**: standard

**判定理由**: 文書と guard の語の追加だけで、設計上の判断も状態の変更も外部の資源もないため、どの契機にも当たらない。

-   [ ] グリーンゲート（`make test && make lint`、README を変えるので `make ext-test` も。`_context.md` の "Green gate"）がパスしていることを確認した
-   [ ] PR を作成した
-   [ ] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

-   [ ] **ステップ 6-6**: AC-19 の手動の確認を行う。実際のサーバと拡張を使い、LLM の API と Webhook を呼ぶので、実行の前に利用者の承認を得る（CLAUDE.md「Tool Execution Safety」）。結果を §5.1 に記録する。PR-5（`BkaUHsH_mkw` のサンプル）と PR-7 のマージ後に行う。

### PR-8 作成ポイント: manual verification record

**対象ステップ**: 6-6

**推奨タイトル**: `docs(0024): record the manual verification of copied panel selections`

**レビュー観点**: §5.1 の 3 つの入力（報告された動画・パネル全体の選択・`BkaUHsH_mkw`）の結果と日付が記録されていること / 送った入力の全体を `testdata/` にも本計画にも保存していないこと（AC-16）

**実装モデル要件**: standard

**判定理由**: 本計画の §5.1 に結果を書くだけの文書の変更である。手動の確認は実際の LLM の API と Webhook を 1 回ずつ呼ぶが、統合テストや CI の仕組みを作るものではないので、外部の資源の契機（多くの統合テスト・CI・外部の資源）に当たらない。実行の前に利用者の承認を得る。

-   [ ] グリーンゲート（`make test && make lint`。`_context.md` の "Green gate"）がパスしていることを確認した
-   [ ] PR を作成した
-   [ ] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### 2.7. 既存のテストの書き換えの台帳

設計書 3.6 の表を、書き換えるステップとの対応を付けて再掲する。AC-18 の「書き換えたケースは 03 に記録する」はこの表で満たす。ケースの名前を変える場合は、この表の「新しい名前」の列を同じコミットで埋める。

| ステップ | テスト | ケース | 変更 | 新しい名前 |
|---|---|---|---|---|
| 1-15 | `TestParseCopiedRejects` | `empty line at the start` | 受理のケースへ移す | `TestParseCopiedAccepts` の `empty line at the start` |
| 1-6 | `TestParseCopiedRejects` | `two trailing separators` | 受理のケースへ移す | `TestParseCopiedAccepts` の `form B two trailing separators` |
| 1-6 | `TestParseCopiedRejects` | `three trailing separators` | 受理のケースへ移す | `TestParseCopiedAccepts` の `form B three trailing separators` |
| 1-15 | `TestParseCopiedRejects` | `leading display items` | 理由を `errCopiedNoText`（行番号なし）にする | - |
| 1-6 | `TestParseCopiedRejects` | `form B whitespace-only text` | 理由を `errCopiedNoText` にし、`errCopiedBlankText` の確認を、本体の中に空白文字だけの本文がある入力のケースとして別に加える | -（別に加えるケースは `form B whitespace-only text in the body`） |
| 1-6 | `TestParseCopiedLineBreaks` | `trailingRejects` の 2 ケース | 受理の確認に変える。入力が `two/three trailing separators` と同じなので、`TestParseCopiedAccepts` の表にだけ残す | `TestParseCopiedAccepts` の `form B two trailing separators`・`form B three trailing separators` |
| 1-6 | `TestParseCopiedCheckOrder` | `line kind before blank text` | 今の入力 `0:00\nx\n \n` は受理のケースとして `TestParseCopiedAccepts` へ移し、このケースの入力を、空白文字だけの行が本体の中にあるもの（設計書 3.6）に変える | 受理へ移す入力は `TestParseCopiedAccepts` の `form B trailing whitespace-only line` |
| 1-15 | `TestParseCopiedErrorOmitsInput` | `not a timestamp omits the line` | 目印の行を形 A の本体に置く（設計書 3.6） | - |
| 3-3 | `TestParseCopiedRejects` | `chapter heading after text` | 受理のケースへ移す | - |
| 3-3 | `TestParseCopiedErrorOmitsInput` | `chapter heading omits the line` | 今の入力は受理されるので、`TestParseCopiedErrorOmitsInput`（受理されると `Fatalf` する）には置けない。この入力は `TestParseCopiedFormBChapterHeadings` に移し、`Segments` に目印が入らないことを確かめる。元のケースは名前を残し、設計書 3.3 の規則で拒否される行の入力（設計書 3.6）に変えて拒否のメッセージを確かめる | `TestParseCopiedFormBChapterHeadings` の `chapter heading is not in the segments`（受理の側） |

`TestParseCopiedRejects` から受理へ移す 4 つのケースと `trailingRejects` の 2 ケースは、受理の確認に `TestParseCopiedAccepts` の表か、読み飛ばす行を取り除いた入力との比較を使う。ステップ 1-6 の `two trailing separators`・`three trailing separators` と `trailingRejects` の 2 ケースは入力が同じなので、`TestParseCopiedAccepts` の表と `TestParseCopiedLineBreaks` のどちらか一方にだけ残し、重複させない。重複を除いた後、`go tool cover -func` の結果が関数ごとに変わらないことを確かめ、コミットメッセージに書く（CLAUDE.md「Deleting a test is a claim that must be checked」）。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | フェーズ | 成果物 | 依存 |
|---|---|---|---|
| M1 | 1 | 前置き・末尾の領域・形の判定・行番号とそのテスト | なし |
| M2 | 2 | 形 A の見出し行の読み飛ばしとそのテスト | M1 |
| M3 | 3 | 形 B の見出し行の読み飛ばしとそのテスト | M1 |
| M4 | 4 | `BkaUHsH_mkw` のサンプル・記録・guard | M3 |
| M5 | 5 | 4 MiB の入力のテスト | M2・M3 |
| M6 | 6 | CLAUDE.md・README・手動の確認の記録 | M2・M3（§5.1 の `BkaUHsH_mkw` の行だけは M4） |

フェーズ 2 と 3 は互いに依存しない（設計書 8 章）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 | 末尾の領域の読み飛ばし、形ごとの解析への範囲の受け渡しと元の行番号の報告、本文の有無の検査の移動、fuzz の性質 (a) | frontier-recommended |
| PR-2 | 1-11 / 1-12 / 1-13 / 1-14 / 1-15 / 1-16 / 1-17 / 1-18 / 1-19 / 1-20 | 前置きの読み飛ばしと拒否の規則、最初のセグメントの行による形の判定、fuzz の性質 (b) | frontier-recommended |
| PR-3 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | 形 A のチャプターの見出し行の読み飛ばしとそのテスト | standard |
| PR-4 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | 形 B の時刻を期待する位置の見出し行の読み飛ばし、§2.7 のステップ 3-3 の書き換え | frontier-recommended |
| PR-5 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 | `BkaUHsH_mkw` のサンプル・`testdata/README.md`・サンプルの数を固定するテストと文書の guard | standard |
| PR-6 | 5-1 / 5-2 / 5-3 | 4 MiB の入力のテスト | standard |
| PR-7 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 | CLAUDE.md の例外・その guard・README の案内 | standard |
| PR-8 | 6-6 | AC-19 の手動の確認の記録 | standard |

各 PR は前の PR がマージされた `main` から作り、それだけでグリーンゲートを通す。依存は次のとおりである。

-   PR-2 は PR-1 を要する（前置きの走査は末尾の領域の直前で止まる）。PR-3・PR-4 は PR-2 を要し、互いには依存しないので、順を入れ替えてもよい。
-   PR-5 は PR-4（形 B の見出しの読み飛ばし）を要する。PR-6・PR-7 は PR-3・PR-4 を要し、PR-5 には依存しない。サンプルの確認（ステップ 4-1）が遅れるときは、PR-6・PR-7 を PR-5 より先に作ってよい。PR-8 は PR-5 と PR-7 の後に行う。
-   フェーズ 1 を PR-1 と PR-2 に分けるのは、末尾の領域と前置きがそれぞれ独立に振る舞いを変え、台帳の書き換えも分けられるためである。各 PR の中で、非公開の述語・処理（ステップ 1-1・1-2、1-11・1-12）は呼び出し側と同じ PR に置く。呼び出し側のない非公開の関数は `make lint` の `unused` で失敗する（`.golangci.yml` で有効）。
-   PR-1 から PR-7 のマージまで、`main` の `README.md:599-601` の案内（末尾の空行を拒否する）と CLAUDE.md の例外の記載は、パーサの振る舞いより古いままになる。利用者に配布していない開発中の状態なので、この差を受け入れ、PR-7 でまとめて直す。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   設計書 7.1 の方針に従う。AC-34 を除き、入力は `_test.go` の中の合成の文字列とし、見出しのタイトルと本文は架空の文字列にする。報告された入力（`dsuNgos_kxA`）と要件書 §1 で観測した入力の全体は保存しない（AC-16）。
-   読み飛ばしのテストは、読み飛ばす行を取り除いた入力の `Segments` と比べる。比較の相手となる、行を取り除いた入力について、先頭の行が時刻の行であること、0 文字の行と空白文字だけの行を含まないこと、取り除いた行の文字列を含まないことをテストの中で確かめる。さらに、その入力の `Segments` をリテラルの期待値とも比べる（CLAUDE.md「A layered path needs inputs only one layer can handle」）。
-   拒否のテストは `assertCopiedRejected` を通し、理由・`ErrCopiedTranscriptFormat`・部分的な結果を返さないことを確かめる。行番号のある拒否は、前置きのある入力でも行番号が元の入力のものであることを確かめる。
-   網羅の目標: `go test -tags test -coverprofile` と `go tool cover -func` で、`copied_format.go` の新しい関数と変更した関数がすべて実行されていることを確かめる。設計書 3.3 の拒否の規則と、末尾の領域・前置きの決定の分岐は、それぞれ 1 つ以上のテストで通す。
-   新しいケースは、既存の表（`TestParseCopiedAccepts`・`TestParseCopiedRejects`・`TestParseCopiedErrorOmitsInput`）に足せるものは足し、読み飛ばす行を取り除いた入力との比較が要るものは新しいテスト関数にする。テスト関数の名前は §5 の表に記録し、実装で変えたら同じコミットで §5 を更新する。

### 4.2. 実データのテスト

`TestCopiedSourceRealSamples` に `BkaUHsH_mkw` を加える（§2 フェーズ 4）。`TestValidateTranscriptAcceptsCopiedSamples` と `FuzzNewCopiedSource` の種は、`testdata/` のサンプルを読むので、ファイルを加えるだけで新しいサンプルを含む。

### 4.3. 統合テスト

本タスク単体の統合テストはない。サーバの経路は既存の `TestRejectInvalidTranscript`・`TestServerEndToEnd` が確かめ、その入力は本設計でも拒否される（§1.3）。実際のサーバと拡張を通す確認は AC-19 の手動の確認で行う。

### 4.4. 後方互換性

-   0015 のパーサが受理する入力は、本設計でも同じ `Segments` になる（設計書 7.1 の後方互換の性質）。既存の受理のケースと実データの 3 件が変更なしで通ることと、`FuzzNewCopiedSource` の性質 (a)・(b) で確かめる。
-   拒否の振る舞いが変わるのは §2.7 の台帳のケースだけである。それ以外の既存の拒否のケースが理由と行番号を変えずに通ることを、`make test` で確かめる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別: `test` は失敗しうる実行の確認、`static` は guard テスト・`make` ターゲット、`manual` は人の確認。テストの場所はすべて `internal/transcript/copied_format_test.go` を既定とし、別のファイルのものはパスを書く。新しいテスト関数の名前は計画上の名前で、§4.1 のとおり実装に合わせて更新する。「フェーズ」の列が 1 の行は、ステップ 1-7・1-8 の対象（AC-07・AC-09・AC-10・AC-21〜AC-23・AC-26・AC-33、および AC-11 の本体が空の入力）が PR-1、ステップ 1-16・1-17 の対象（AC-11 の残り・AC-24・AC-28〜AC-32、前置きのある入力の AC-15）が PR-2 である。

| AC | 種別 | 確認 | フェーズ |
|---|---|---|---|
| AC-01 | test | `TestParseCopiedFormAChapterHeadings`（見出し行を取り除いた入力との比較） | 2 |
| AC-02 | test | `TestParseCopiedFormAChapterHeadings`（タイトルが `Text` に入らないこと、要素の数） | 2 |
| AC-03 | test | `TestParseCopiedFormAChapterHeadings`（最後のセグメントの後の見出し、見出しとラベルの言語の違い） | 2 |
| AC-04 | test | `TestParseCopiedRejects`（一致しない見出しの各行、U+3000 で終わるタイトル） | 2 |
| AC-05 | test | `TestParseCopiedRejects`（見出し行だけの入力の `errCopiedNoText`） | 2 |
| AC-06 | test | `TestParseCopiedFormAChapterHeadings`（タイトルに `:`・空白・`チャプター`） | 2 |
| AC-07 | test | `TestParseCopiedTrailingRegion`（日本語 UI と英語 UI の操作ラベル） | 1 |
| AC-08 | test | `TestParseCopiedFormBChapterHeadings`（形 B の末尾、パネル全体の選択） | 3 |
| AC-09 | test | `TestParseCopiedRejects`（形 A の一致しない末尾の行）、`TestParseCopiedFormBChapterHeadings`（形 B の側） | 1・3 |
| AC-10 | test | `TestParseCopiedRejects`（本体の操作ラベル行） | 1 |
| AC-11 | test | `TestParseCopiedRejects`（操作ラベル行だけの入力の `errCopiedNoText`） | 1 |
| AC-12 | test | `TestParseCopiedRejects`（形 A の本体の `タイムライン`・`こんにちは`）、`TestParseCopiedFormBChapterHeadings`（形 B の側） | 2・3 |
| AC-13 | test | `TestParseCopiedRejects`（形 A の本体の未知のラベル）、`TestParseCopiedFormBChapterHeadings`（形 B の側） | 2・3 |
| AC-14 | test | 新しい拒否のテストすべてが通る `assertCopiedRejected`（`ErrCopiedTranscriptFormat` と `segments != nil` の確認） | 1〜3 |
| AC-14 | static | `internal/transcript/json3_test.go::TestExportedSentinelErrorsUnchanged`（公開の番兵の集合） | 1 |
| AC-15 | test | `TestParseCopiedErrorOmitsInput`（前置き・形 A の見出し・形 B の見出しの位置・本体の拒否の目印）、`TestParseCopiedRejects` の `body error line counts the prefix`（前置きのある入力の行番号） | 1〜3 |
| AC-16 | test | `internal/transcript/copied_test.go::TestCopiedSourceRealSamples`（`testdata/` のサンプルの集合を `copiedSampleFiles` に固定し、`dsuNgos_kxA` を含まない） | 4 |
| AC-17 | test | `cmd/yt2column/copied_transcript_docs_test.go::TestCopiedTranscriptSurveyRecords`（README のファイル名・URL・ライセンス・項目、要件書 §1.1 の「保存する」）、`internal/transcript/copied_test.go::TestCopiedSourceRealSamples`（バイト数） | 4 |
| AC-17 | manual | PR のレビューで、`testdata/README.md` の値を動画のページと取得者の申告に照らす | 4 |
| AC-18 | test | `make test`（§2.7 の台帳以外の既存のケース、`internal/writer/prompt_test.go::TestValidateTranscriptAcceptsCopiedSamples`、パイプラインの guard） | 1〜6 |
| AC-19 | test | `TestParseCopiedFormBChapterHeadings`（パネル全体の選択）、`TestParseCopiedFormAChapterHeadings`（前置き・見出し・末尾を含む形 A のパネル全体の選択）、`internal/transcript/copied_test.go::TestCopiedSourceRealSamples/BkaUHsH_mkw` | 2〜4 |
| AC-19 | manual | 実際のサーバと拡張で 3 つの入力を送り `202` を確かめる（§5.1） | 6 |
| AC-20 | test | `TestParseCopiedLargeInputs`（設計書 7.3 の 4 つの入力を含む 7 つ） | 5 |
| AC-21 | test | `TestParseCopiedTrailingRegion`（形 A の末尾の空行） | 1 |
| AC-22 | test | `TestParseCopiedTrailingRegion`（U+0020・U+3000 だけの行）、§2.7 で書き換える `form B whitespace-only text`（形 B の末尾の空白文字だけの行）、§2.7 で受理へ移す `two trailing separators`（形 B の末尾の空行） | 1 |
| AC-23 | test | §2.7 で受理へ移す `three trailing separators`（2 個以上の空行）、`TestParseCopiedTrailingRegion`（空行と操作ラベルの混在） | 1 |
| AC-24 | test | `TestParseCopiedRejects`（本体の空行）、`TestParseCopiedPrefix`（先頭の空行を受理） | 1 |
| AC-25 | test | `TestParseCopiedRejects` の `blank newline` と `TestParseCopiedCheckOrder` の `blank before empty line`（既存。`\n`・`\n\n` の `errCopiedBlank`） | 1 |
| AC-26 | test | `TestParseCopiedRejects`（末尾のタブの行の `errCopiedControlChar`）、`TestParseCopiedCheckOrder`（手順 8 と手順 9 の順序） | 1 |
| AC-27 | test | `cmd/yt2column/copied_transcript_docs_test.go::TestClaudeMDCopiedTranscriptException` | 6 |
| AC-28 | test | `TestParseCopiedPrefix`（観測した前置きと形 A） | 1 |
| AC-29 | test | `TestParseCopiedPrefix`（前置きと形 B） | 1 |
| AC-30 | test | `TestParseCopiedRejects`（前置きの数字・`:`・数字の行、制御文字の行） | 1 |
| AC-31 | test | `TestParseCopiedRejects` の書き換えた `leading display items`（`タイムライン\n`）と、`文字起こし\n\n動画の関連情報` のケース。`TestParseCopiedCheckOrder`（前置きの走査が末尾の領域に入らない順序） | 1 |
| AC-32 | test | `TestParseCopiedPrefix`（日本語の前置きと英語 UI のラベル） | 1 |
| AC-33 | test | `TestParseCopiedTrailingRegion`（形 B の本文が操作ラベルと一致する入力） | 1 |
| AC-34 | test | `internal/transcript/copied_test.go::TestCopiedSourceRealSamples/BkaUHsH_mkw` | 4 |
| AC-35 | test | `TestParseCopiedFormBChapterHeadings` | 3 |
| AC-36 | test | `TestParseCopiedRejects`（形 B の時刻を期待する位置の 4 種類の行） | 3 |
| AC-37 | test | `TestParseCopiedFormBChapterHeadings`（本文を期待する位置の見出し） | 3 |
| AC-38 | test | `TestParseCopiedFormBChapterHeadings`（前置きの素のタイトル） | 3 |

AC-25 は、入力全体の検査（`errCopiedBlank`）が先に起こるので、本タスクの変更の後も既存のケースのまま確かめられる。PR-1・PR-2 の後に既存のケースが通ることを確かめる。

### 5.1. 手動の確認の記録（AC-19）

ステップ 6-6（PR-8）で記録する。

| 入力 | 経路 | 結果 | 日付 |
|---|---|---|---|
| 報告された動画 `dsuNgos_kxA`（形 A・見出し 24 行） | 拡張 → サーバ | - | - |
| 要件書 §1 の前置きから末尾の `動画の時間と同期` までのパネル全体の選択 | 拡張 → サーバ | - | - |
| `BkaUHsH_mkw`（形 B・見出し 3 行） | 拡張 → サーバ | - | - |

英語 UI の形 A の見出し（`Chapter`）を実物で確かめられた場合は、この表に行を加える（要件書 §7）。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 対策 |
|---|---|
| 前置きと末尾の領域を除いた後の行番号が、元の入力の行番号からずれる | 行を詰め直さず範囲で表す（設計書 3.4）。前置きのある入力の拒否で行番号を確かめる（AC-15） |
| 時刻の文法の拒否のケースが、前置きとして黙って読み飛ばされる | 数字・`:`・数字の述語の確認（ステップ 1-20 の (ii)）。既存のケースを変えずに通す（AC-18） |
| 形 B の最後の本文が操作ラベルと一致するとき、本文を失う | 末尾の領域の「直前が時刻だけの行」の条件と AC-33 のテスト、ステップ 1-10 の (i) |
| 4 MiB の前置きの入力で、行の並びのメモリが race の上限を超える | フェーズ 5 で `copiedParseAllocPerByte` の範囲に収まることを確かめる。収まらなければ、上限を変えずに組み立て方を設計書 7.3 の範囲で見直し、それでも収まらなければ設計書の見直しを依頼する |
| 設計書 5.4 の残余のリスク（形 B の時刻の行の欠落、正しい入力の拒否） | 本タスクでは受け入れる。運用での観測は設計書 9 章の将来の拡張に残す |

### 6.2. スケジュールのリスク

| リスク | 対策 |
|---|---|
| `BkaUHsH_mkw` の取得物と取得の条件の確認が遅れる | フェーズ 4 のうちサンプルの取得と確認は他のフェーズと独立しているので、先に依頼しておける。フェーズ 5 と、フェーズ 6 のうち手動の確認以外のタスクは、フェーズ 4 に依存しない。PR-6・PR-7 は PR-5 より先に作ってよい（§3.2）。PR-8 は PR-5 の後に行う |
| §1.3 の候補のファイルが、OS の一時ディレクトリの掃除で消える | 取り直し、§1.3 の測定（バイト数・行数・見出しの位置）をやり直してから期待値を決める |
| AC-19 の手動の確認に必要な環境（API キー・Webhook）が用意できない | フェーズ 6 のその他のタスクを先に終え、§5.1 は確認ができた時点で記録する |

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10）
-   [ ] PR-2 マージ済み（対象ステップ: 1-11 / 1-12 / 1-13 / 1-14 / 1-15 / 1-16 / 1-17 / 1-18 / 1-19 / 1-20）
-   [ ] PR-3 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）
-   [ ] PR-4 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
-   [ ] PR-5 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8）
-   [ ] PR-6 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3）
-   [ ] PR-7 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5）
-   [ ] PR-8 マージ済み（対象ステップ: 6-6）
-   [ ] §2.7 の台帳の全行を書き換えた
-   [ ] §5 の表のテスト関数の名前が実装と一致する
-   [ ] `make test`・`make lint`・`make deadcode`・`make ext-test` が通る

## 8. 成功基準 (Success Criteria)

-   **機能:** §5 の表のすべての `test`・`static` の確認が通り、§5.1 の 3 つの入力が `202` で受理される。
-   **品質:** `make test`・`make lint` が通り、`make deadcode` が新しい到達できない関数を報告しない。§4.1 の網羅の目標を満たす。各フェーズの「失敗しうることを確かめる」タスクの結果がコミットメッセージにある。
-   **セキュリティ:** 読み飛ばした行と拒否した行の文字列がエラーメッセージに入らない（AC-15）。部分的な結果を返さない（AC-14）。`testdata/` に加えるのは CC BY の `BkaUHsH_mkw` だけで、報告された入力を保存しない（AC-16・AC-17）。
-   **文書:** `testdata/README.md`・CLAUDE.md・`README.md` が本設計の受理の範囲と合い、文書の guard が通る。

## 9. 次のステップ (Next Steps)

-   実装の後、設計書 5.4 の観測の対処案（拒否の理由と読み飛ばした行の数のログ）を、必要なら別のタスクとして起こす。
-   本タスクの用語（最初のセグメントの行・前置き・時刻を期待する位置・末尾の領域）を `docs/translation_glossary.md` に加えるかを決める。設計書 3.5 の変更の対象にないので、本計画のタスクにはしない。
-   英語 UI の形 A の見出しと、末尾の領域を一覧の外に広げるか（要件書 §7）は、実データが得られた時点で見直す。
