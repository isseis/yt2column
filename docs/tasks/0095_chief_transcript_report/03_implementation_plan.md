# 実装計画書：chief が Claude Code の pane の最後の報告を、画面ではなく transcript から読む

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-17 |
| Review date | 2026-11-17 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `4b7e1831` で確かめた。設計書が確かめたコミットは `41683512` である。このコミットから `4b7e1831` までの間に、`internal/chief`・`cmd/chief`・`internal/strictjson`・`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` は変わっていない（`git diff --stat 41683512 4b7e1831 -- <これらのパス> CLAUDE.md` の出力は `CLAUDE.md` の `make wasm-check` の 1 行だけだった）。そのため、設計書が引く `internal/chief` の行番号は今も合う。

用語は、要件書 6 章、設計書の冒頭（応答・合成の応答・報告の読み取り部品・報告の入口・NEXT を名指す行・PR の詳細の控え・報告の控え）、要件書 1 章の記号 ①〜④ と同じ意味で使う。本計画は、次の用語を加える。

-   **実地の確かめ:** PR-3 のマージの後、PR-4 の `/runplan` の前に、人が `main` から入れ直した chief を本物の Herdr と Claude Code で動かして確かめること（§1.3 の決定 4）。
-   **消すコミット:** フェーズ 5 で、使われなくなる部品と、そのテストおよびテストの補助関数を消すコミット。コミットのメッセージに `make coverage-compare` の結果を書く（CLAUDE.md の「Testing Strategy」）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、Claude Code の pane の最後の報告と PR の記録を transcript から読む部品を足し、①〜④をその部品に替え、使われなくなる画面の読み取りを消し、CLAUDE.md と chief 利用ガイドを直して、要件書の F-001〜F-004 を満たす。

### 1.2. 実装の原則

-   **読む部品を先に作り、確かめの情報源を 1 か所ずつ替える。** サンプルと読む部品（PR-1）→ ①②（PR-2）→ ③④と報告の控え（PR-3）→ 消す部品と文書（PR-4）の順にする（設計書 8 章）。途中の PR をマージして道具を入れ直しても、替えていない確かめは今のまま画面を読む。
-   **確かめの組み立ては変えない。** ①〜④の条件の並び、送る入力、送り先の選び方は変えず、Claude Code の pane で画面を読んでいた所だけを報告の入口に替える（設計書 1.1）。
-   **OpenCode の pane のテストの行は変えない。** AC-13 は、今のテストの OpenCode の行が変更の前後で通ることで確かめる。
-   **ユニットテストは本物の Herdr・Claude Code を使わない。** transcript は今の fake（`newSendWorld` の `ClaudeConfigDir` と `writeTranscript`、`internal/chief/sim_test.go:47-56`・`107-127`）で置く。本物を使うのは、§1.3 の決定 1 のサンプルの採取と、決定 4 の実地の確かめだけである。
-   **テストの日本語の文字列は `screen` で始まる名前のファイルに置く。** 報告の読み取り部品のテストのうち、サンプルの日本語の文字列と比べる確かめは、`screen` で始まる名前のテストファイル（`screen_transcript_report_test.go`）だけに置く（CLAUDE.md の「Go Idioms」の chief の例外）。フェーズ 3・4 の `autosend_test.go`・`detail_test.go`・`reset_test.go` の報告は、英語で作った本文か、サンプルを `readSessionReport` で読んだ `Text` と比べて書き、日本語の文字列を持たない。NEXT の「なし」のように日本語の値が要る報告は、`screen` で始まる名前のテストファイルの補助関数で作る。transcript の行を組み立てる補助関数は、同じ種類の補助関数（`modeLine`・`commandLine`・`typedLine`）がある `internal/chief/sim_test.go` に足す。
-   **各テストは壊して失敗を確かめる。** 確かめた結果をコミットのメッセージに書く（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **Claude Code の今の `/runpr` の報告のサンプルは、本計画の PR の `/runpr` を、人が Claude Code の pane で走らせて採る（人が選んだ）。** 送り先の表は `/runpr` を OpenCode に送るので、手元の transcript に今の形の報告はない（設計書 1.2）。`/runplan` のエージェントは、ほかの pane に入力できない（`.claude/settings.json` が `herdr agent prompt` を拒む）。本計画の PR の `/runpr` はどのみち走るので、人がそれを Claude Code の pane で走らせれば、`/runpr` を余分に走らせずに今の形の報告が残る。PR-1 の `/runplan` は、その transcript を読むだけでサンプルを採れる。PR-1 の途中で人に頼む案は採らない。`/runpr` の実行が 1 回増え、PR-1 の自動送信を `hold` にすることになるためである。ADR: なし（サンプルを採る手順で、このタスクの中で閉じるため）。
2.  **1 つの transcript を 1 秒以内に読めることは、手元のいちばん大きい transcript で一度だけ測って記録し、恒久のテストにしない（人が選んだ）。** 実物の transcript はリポジトリに置けない（security.md §7）。25 MB の作った入力を `make test` で読むと、テストが遅くなり、`-race` の下で時間の比べが安定しない。測った値は、測ったコミットのメッセージと §7 の「PR-1 の実行の記録」に残す。ADR: なし（確かめの手段で、このタスクの中で閉じるため）。
3.  **PR を 4 つに分け、道具の `Protocol` は PR-3 で 1 回だけ上げる（人が選んだ）。** 設計書 8 章の 1・2 を PR-1、3 を PR-2、4 を PR-3、5・6 を PR-4 にする。出力に足す 2 つの項目（①の pane ごとの結果、PR の説明を示した理由。設計書 4.2）のうち、①の項目は PR-2 で出力に足し、`/chief` の定義には PR-3 で書く。PR-2 から PR-3 までの間、出力に定義が触れない項目が 1 つあるが、足しただけの項目なので、定義に従う chief のセッションは読み違えない。`Protocol` と定義の `--protocol` は、`TestDocumentedProtocolMatches`（`cmd/chief/docs_test.go:205`）が等しいことを求めるので、同じ PR-3 で変える。0092 と 0093 も `Protocol` を上げる計画なので、値は PR-3 を実装する時点の値に 1 を足したものとする。次の 2 案は採らない。1 つは、5 つに分けて出力の変更を独立させる案で、①の部品を 2 つの PR で続けて直すことになるためである。もう 1 つは、①〜④を 1 つの PR で替える案で、PR が大きくなりすぎるためである。ADR: なし（PR の分け方で、このタスクの中で閉じるため）。
4.  **実地の確かめは、PR-3 のマージの後、PR-4 の `/runplan` の前に、人が行う（人が選んだ）。** ③④が働くのは PR-3 の後なので、そこで確かめれば、見つかった問題を PR-4 で直せる。PR-4 の自動送信は `hold` にする。ADR: なし（確かめの時機で、このタスクの中で閉じるため）。

### 1.4. 既存コード調査結果

| 領域 | 今あるもの | 本計画で行うこと |
|---|---|---|
| transcript の場所と読み込み | `transcriptPath`（`internal/chief/transcript.go:53-70`）、`readTranscript`（同 82-98）、`transcriptFailure`（同 75-81）、上限 `transcriptLimit`・`resetTranscriptLimit`（同 28-31） | そのまま使う。`resetTranscriptLimit` の名前を変える（§2 の改名の一覧）。報告の入口 `readClaudeReport` を同じファイルに足す |
| 人の入力の読み取り | `humanInputOf`（`internal/chief/screen_transcript.go:242-283`）、`otherOrigins`（同 153）、`readLastHumanInput`（同 170-189）。`readLastHumanInput` の呼び出し元は `internal/chief/reset.go:63` だけである | `humanInputOf` を `readSessionReport` から使う。`otherOrigins` に `auto-continuation` を足す。`readLastHumanInput` は PR-4 で消す |
| strict な JSON の読み取り | `strictjson.ParseObject`・`Object.Collect`・`RequiredString` など（`readModelSwitch` が使う形、`internal/chief/screen_transcript.go:46-90`） | 新しい読み取りもこれで行う。`strictjson` に足すものはない見込みである。足すときは `docs/dev/conventions/package_reference.md` の行も直す |
| NEXT と PR 番号 | `nextLine`（`internal/chief/screen_report.go:56`）、`readNextValue`（同 91-117）、`readRunprNext`（同 72-86）、`prReference`（同 305）、`RunprReport`（同 295-301） | `readReportNext` が `nextLine`・`readNextValue`・`prReference` を使い、`RunprReport` を返す。`readRunprNext` は OpenCode の画面のために残す |
| ①② | `reportsAgree`（`internal/chief/autosend.go:441-461`）、`detachedReport`（同 464-487）、`resetRunpr`（同 728-777）。`AutoSendResult`（同 238-246）、`AutoSendOutput`（`internal/chief/output.go:430-438`） | Claude Code の pane を報告の入口で読むように変える。①の pane ごとの結果を `AutoSendResult` と出力に足す |
| ③ | `readReport`（`internal/chief/detail.go:444-465`）、`keptReport`（同 419-439）、`readDetail`（同 305）、`ReportSource`（同 248-279）、`PRDetail` の `Report`・`ReportText`・`RecordErr`（同 334-344）、`DetailOutput`（`internal/chief/output.go:290-309`） | Claude Code の pane を報告の入口で読み、PR の記録で候補を決める。報告の控えを作る。PR の説明を示した理由を足す |
| ④ | `resetAfterRunpr`（`internal/chief/reset.go:94-143`）、`lastInputMadePR`（同 51-74）、`ensureDetail`（`internal/chief/request.go:371-381`） | 画面の確かめと、`lastInputMadePR` の中の transcript の読み込みを、報告の入口に替える。`ensureDetail` は `keptPaneReport` で控えを読む |
| 記録 | `detailPrefix`・`detailFileName`（`internal/chief/records.go:577`・`586`）、`detailMaxAge`（同 594）、`cleanup`（同 883-928）、`Protocol`（同 28） | 報告の控えの名前の形と `keptPaneReport` を足し、`cleanup` が報告の控えも消すようにする。`Protocol` を PR-3 で上げる |
| 示す本文の上限 | `CapScreen`（`internal/chief/sanitize.go:31-50`）、`CapDescription`（同 55-63）、`descriptionMaxBytes`（同 16）。`screenMaxLines`・`screenMaxBytes` は `screen_ansi.go:61`・`74` も使う | `CapDescription` を `CapHead` に改名する（PR-1）。`CapScreen` は PR-4 で消し、`screenMaxLines`・`screenMaxBytes` は残す |
| Claude Code の画面の報告の読み取り | `readCommandReport`・`claudeLastMessage`・`namesURL`・`claudeMessageMark`・`claudeDoneMark`・`claudeIndent`・`claudeRecapMark`（`internal/chief/screen_report.go:123-213`・`350-363`）。印の 4 つは `claudeLastMessage` のほかに使う所がない（4b7e1831 で `rg -n "claudeMessageMark\|claudeDoneMark\|claudeIndent\|claudeRecapMark" internal/chief` の結果が `screen_report.go` の中だけだった） | PR-4 で消す。`squeeze` と `joinTexts` は OpenCode の分と画面の読み取りが使うので残す |
| テストの fake | `newSendWorld`・`writeTranscript`・`transcript`（`internal/chief/sim_test.go:47-127`）、`detailWorld`、`readTestdata`、`readScreenSample` | そのまま使う。transcript の行を組み立てる補助関数は、同じ種類の補助関数がある `sim_test.go` に足す（§1.2）。`/runpr` を Claude Code に送る送り先の表は、`send_test.go:720-725` と同じく、`_context.md` を書き換えた写しを返して作る |
| サンプルとその検査 | `testdata/transcript/` の 3 つ、`TestSamplesFollowReplacementRules`（`internal/chief/samples_test.go:72-92`。`testdata` の下のすべてのファイルを置き換えの規則で調べる）、`TestScreenSamplesCoverEveryRow`（同 132-166）、`screenRows` の `command report` の行（同 117-119） | transcript のサンプルを足す。足したサンプルも `TestSamplesFollowReplacementRules` が調べる。`command report` の行は PR-4 で `input line` の行に移す |
| 文書のガード | `TestDocumentedProtocolMatches`（`cmd/chief/docs_test.go:205`）、`TestGuideValuesMatchTool`（同 405。手引きの `プロトコル番号` の値を `Protocol` と比べる）、`TestADRRecords`（`cmd/yt2column/adr_test.go:434`） | `Protocol` を上げる PR-3 で、定義と手引きの番号を直す。テストは変えない |
| パッケージの一覧 | `docs/dev/conventions/package_reference.md:135` の `internal/chief` の行が `CapScreen`・`CapDescription` を挙げる | PR-1 で `CapDescription` を `CapHead` に、PR-4 で `CapScreen` を消す |

`internal/chief` のパッケージの外で、本計画が改名するか消す名前を使う所はない（4b7e1831 で `rg -n "CapDescription|ReportFromScreen|readCommandReport|resetTranscriptLimit|CapScreen|readLastHumanInput|claudeLastMessage" --glob '!docs/tasks/**' .` を走らせ、`internal/chief` の外の結果は `docs/dev/conventions/package_reference.md:135` だけだった）。

設計書 3.1 の型 `SessionReport` は、作り方を 1 つに限る型ではない（振る舞いを選ぶのは `Kind` の値で、具体的な型による振り分けはない）。そのため、作り方の経路を数える確かめは要らない。

### 1.5. 実装の際に決めたこと（2026-11-17）

-   **ステップ 1-1 のサンプルは、runpr.md に NEXT の行を定めた PR の `/runpr` を、人が本タスクの worktree の Claude Code の pane で走らせて採る（人が選んだ）。** 本計画の PR（#935）の `/runpr` は Claude Code で走らず、本タスクの worktree の transcript にサンプルがなかった。代わりに、ほかの worktree で Claude Code で走った #925 の `/runpr` の報告を見たところ、`NEXT:` の行がなく、今の文法で読めなかった。そこで、報告に今の文法で読める NEXT の行を書くことを runpr.md で定めた（要件書の「実装の際に決めたこと（2026-11-17）」）。その PR の `/runpr` は runpr.md を変えたブランチで走るので、定めた形の報告が残る。§1.3 の決定 1 の、`/runpr` を余分に走らせずにサンプルを採るという考えは変わらず、使う PR だけが変わる。ADR: ADR-0046 を更新する（要件書の同じ項のため）。

## 2. 実装のステップ (Implementation Steps)

**改名の一覧。** 本計画が変える名前は次の表がすべてである。各ステップ、受け入れ基準の表、文書は、この表に従う。

| 旧 | 新 | PR |
|---|---|---|
| `resetTranscriptLimit` | `sessionTranscriptLimit` | PR-1 |
| `CapDescription` | `CapHead` | PR-1 |
| `descriptionMaxBytes` | `headMaxBytes` | PR-1 |
| `TestCapDescription` | `TestCapHead` | PR-1 |
| `ReportFromScreen` | `ReportFromPane`（記録と出力の値 `report` は変えない） | PR-3 |

### フェーズ 1: サンプルを採る（設計書 8 章の 1）

**変更するファイル:** `internal/chief/testdata/transcript/`（サンプルを足す）、`internal/chief/testdata/README.md`、本計画（§7 の「PR-1 の実行の記録」）

-   [x] **ステップ 1-1**: runpr.md に NEXT の行を定めた PR の `/runpr` を走らせた Claude Code のセッションの transcript から、今の形の `/runpr` の報告を含むサンプルを採る（§1.3 の決定 1、§1.5 の決定）。その transcript は、本タスクの worktree の transcript のディレクトリ（`<設定のディレクトリ>/projects/<worktree のパスの / と . を - にした名前>/`）から探す。探すのは、人が入力したコマンドの行が `/runpr <その PR の番号>` であるセッションである。chief が後で同じ pane に `/clear` を送るので、pane の今のセッションとは限らない。見つからないとき、または 2 つ以上見つかるときは、採らずに止まり、サンプルの出どころを人に尋ねる。採った報告の NEXT の行が、今の文法（`nextLine`・`readNextValue`）で読めるかを確かめ、結果を §7 の「PR-1 の実行の記録」に書く。読めなければ、それ以上進まずに止まり、要件に戻って文法を決め直すことを人に尋ねる（設計書 1.2・8 章の 1）。
-   [x] **ステップ 1-2**: 設計書 7.1 の 1 つ目の項のサンプルを、手元の transcript から読み取りだけで採る。PR を作ったコマンドの実行の終わり（最後の報告と PR の記録があるもの）、合成の応答、recap、compact の要約、task-notification に応えた応答、`auto-continuation` の行を含むものである。1 つのファイルが複数を兼ねてよい。
-   [x] **ステップ 1-3**: 各サンプルを、テストが読む行に絞る。今の transcript のサンプルと同じく、ツールの結果の行（`gh pr view` の JSON や git の出力を含む）など、報告の読み取り部品が読まない行は削り、残す行も、受け付ける形の表（設計書 3.1）の項目と、拡張を許す部分を確かめるのに要る項目だけにしてよい（`cwd`・`gitBranch` など、採った人を特定しうる項目を残さない）。そのうえで、採った人を特定できる値を `testdata/README.md` の「置き換えたもの」の表のとおりに置き換える。PR の記録の `prUrl`・`prRepository` と報告の本文の GitHub のログイン名も `owner` に置き換える。
-   [x] **ステップ 1-4**: 置き換えの後に、`TestSamplesFollowReplacementRules` が通ることと、足したサンプルに GitHub のログイン名と利用者の氏名が残っていないことを、手元で一度だけ探して確かめる。後者は、探す値をリポジトリに書かず、探したことと結果（0 件）だけを §7 の「PR-1 の実行の記録」に書く。`TestSamplesFollowReplacementRules` はログイン名と氏名を見つけないためである（`internal/chief/samples_test.go:22-31` の規則は、ホームのパス・UUID・noreply のアドレス・`/home/` のパスだけを見る）。
-   [x] **ステップ 1-5**: `testdata/README.md` に、足したサンプルの採った日と Claude Code の版、採り方、削った行と項目、置き換えた値（PR の記録の URL を含む）を書く。「Claude Code の `/runpr` の報告は、`/runpr` を OpenCode で走らせるので要らない」の文（同 78 行）を、今のサンプルに合わせて書き直す。

**完了の条件:** サンプルが置かれ、`TestSamplesFollowReplacementRules` が通り、ステップ 1-1 と 1-4 の結果が記録されている。

### フェーズ 2: 報告の読み取り部品と入口（設計書 8 章の 2）

**変更するファイル:** `internal/chief/screen_transcript_report.go`（新規）・`screen_transcript_report_test.go`（新規）、`internal/chief/screen_transcript.go`、`internal/chief/transcript.go`・`transcript_test.go`（新規）、`internal/chief/records.go`・`records_test.go`、`internal/chief/sanitize.go`・`sanitize_test.go`、`internal/chief/sim_test.go`（transcript の行の補助関数）、`internal/chief/screen_test.go`（ステップ 2-1 の行）、`internal/chief/detail.go`・`detail_test.go`（改名の分）、`internal/chief/reset.go`（改名の分）、`docs/dev/conventions/package_reference.md`、本計画

-   [x] **ステップ 2-1**: `otherOrigins` に `auto-continuation` を足す（設計書 1.2）。この変更は、PR-1 のうちに製品の振る舞いを変える。④の `readLastHumanInput`（`internal/chief/reset.go:63`）が、`auto-continuation` の行のあるセッションを読めないものとしなくなる。`TestReadLastHumanInput` に `auto-continuation` の行を足して、PR-4 でその各行を消すまで確かめる。
-   [x] **ステップ 2-2**: `screen_transcript_report.go` に `SessionReportKind`・`SessionReport`・`Names`・`readSessionReport` を足す（設計書 3.1。受け付ける形の表と、最後の報告の決め方の 1〜5）。テスト `TestReadSessionReport` は、フェーズ 1 のサンプルと、それを書き換えた入力で、次を確かめる。
    -   PR を作ったコマンドの最後の報告が全文で読め、PR の記録が残ること（AC-01・AC-05）。
    -   AC-02 の各場合（報告の後の人の入力と新しい応答の始まり、終わっていない最後の応答、本文のない応答、終わった応答がないこと、`/clear` で替わった新しいセッション）。報告の後の人の入力には、キューに積まれた人の入力（`queued_command` の `attachment` の行）を含める。逆に、報告の後に peer や task-notification の入力の行しかなければ、報告は残る。
    -   1 つの `message.id` が複数の行に分かれ、どれか 1 行の `stop_reason` が `end_turn` でないときは報告がないこと（設計書 3.1 の規則 3）。`stop_reason` が JSON の `null` のときも報告がないこと。
    -   本文が、複数の行の `text` の部分を書かれた順に空行を挟んでつなぎ、前後の空白を除いたものであり、`thinking`・`tool_use`・知らない種類の部分を含まないこと（同 規則 5）。
    -   サブエージェントの行（AC-03）と、合成の応答・recap・compact の要約（AC-04。後に合成の応答しかない報告は残り、終わっていない応答の後に合成の応答があれば報告はない）。
    -   task-notification に応えた応答も報告になること（設計書 1.2）。
    -   `LastHuman` が、今の `TestReadLastHumanInput`（`internal/chief/screen_test.go:809`）の各行と同じ値になること。`auto-continuation` の行が人の入力にならず、読めないものにもならないこと。
-   [x] **ステップ 2-3**: テスト `TestReadSessionReportRejects` で、次が `SessionReportUnreadable` になり、`Kind` のほかに何も入らない（`Text` が空で、`Names` が偽になる）ことを確かめる（設計書 7.1 の「形の拒み方」）。
    -   要件書 4.2 の「拒むもの」: 重なった項目、型の違い（文字列でも `null` でもない `stop_reason`、文字列でない `text` の部分の `text` を含む）、正でない PR 番号、不正な UTF-8、対にならないサロゲートのエスケープ。
    -   使う項目のない応答の行と PR の記録の行、空の `type`、空の `prUrl`。
    -   書きかけの最後の行。

    同じテストで、拡張を許す部分の結果も確かめる。知らない種類の行と知らない項目は結果を変えない。知らない種類の本文の部分は本文に含めない。知らない `stop_reason` は、読めないものにならず、終わっていない応答として報告がないものになる。
-   [x] **ステップ 2-4**: テスト `TestSessionReportNames` で、`Names` が番号と URL の両方が等しいときだけ真になることを確かめる（設計書 1.2。番号だけ、URL だけが等しい行と、`#42` と `#421` の違い）。
-   [x] **ステップ 2-5**: `readReportNext` を同じファイルに足す（設計書 3.2）。テスト `TestReadReportNext` で、今の文法の行、強調・コード・引用・番号のリストで名指す行（値を読めない NEXT）、文の中で触れるだけの行と `Next:` の行（NEXT がない）、食い違う 2 行（一致しない）、PR 番号の読み取り（折り返しをつなぎ直さないこと）を確かめる（AC-07・AC-08）。ステップ 1-1 の `/runpr` の報告のサンプルの NEXT が読めることも確かめる。NEXT を名指す行のコロンの全角 `：` は、正規表現の中で `\x{FF1A}` のエスケープとして書き、Go のソースに全角の文字を置かない。値の読み取りは今の `readNextValue`・`nextNoneValue` を使い、日本語の文字列を足さない。
-   [x] **ステップ 2-6**: `resetTranscriptLimit` を `sessionTranscriptLimit` に改名し、`ClaudeReport` と `readClaudeReport` を `transcript.go` に足す（設計書 3.3）。テスト `TestReadClaudeReport` で、次を確かめる（AC-02・AC-12）。
    -   設計書 3.3 の表の各行の種類と理由。
    -   セッションのファイルがないときと、transcript のディレクトリごとないときで、`Detail` が違うこと。
    -   上限をちょうど 1 バイト超える transcript が読めないものになること（設計書 7.4）。
    -   FIFO とディレクトリが、報告なしではなく読めないものになること。

    上限を超える transcript は、報告のサンプルの後に空行を足して上限をちょうど 1 バイト超えるファイルとして作り、上限がなければ報告ありと読まれることを先に確かめる（形の読み取りが拒まない入力にして、上限だけが拒むことを確かめるため。上限の確かめは形の読み取りの前に失敗するので、読む時間はかからない。§1.3 の決定 2 が退けた 25 MB の読み取りとは違う）。FIFO は `syscall.Mkfifo` で作るので、`transcript_test.go` は `//go:build test && unix` とする。FIFO が開くときに待たないことは、今の `TestTranscriptFIFORefused`（`internal/chief/send_test.go:833`）が確かめているので、ここでは種類だけを確かめる。
-   [x] **ステップ 2-7**: `records.go` に、報告の控えの名前の形と `keptPaneReport` を足し、`cleanup` が報告の控えも `detailMaxAge` で消すようにする（設計書 3.6）。テスト `TestKeptPaneReport` で、報告の控えが PR の詳細の控えより先に読まれること、どちらもないときに `ok` が偽になること、名前の PR 番号・head と合わない報告の控えと `Source` が `report` でない報告の控えが記録の誤りになることを確かめる。`TestStartCleansSendsAndDetails`（`internal/chief/records_test.go:636`）に、報告の控えが消える行と残る行を足す。
-   [x] **ステップ 2-8**: 改名の一覧の `CapDescription`・`descriptionMaxBytes`・`TestCapDescription` を改名し、関数のコメントと、読む量の上限のコメント（`internal/chief/sanitize.go:10-12`）を設計書 3.8 のとおりに直す。値と切り方は変えない。`package_reference.md` の `internal/chief` の行の `CapDescription` を `CapHead` に直し、画面の読み取りの型の一覧に `SessionReport` を足す。
-   [x] **ステップ 2-9**: 手元でいちばん大きい transcript を `readSessionReport` で読む時間を、一時的なテストで、`-race` を付けずに測る（設計書 1.2 の 98 ms と同じ条件にするため）。測るのは `readSessionReport` を足したコミットの上とし、結果（ファイルの大きさ、時間、測ったコミット）を §7 の「PR-1 の実行の記録」と、その記録を書くコミットのメッセージに書く。一時的なテストはコミットしない（§1.3 の決定 2）。1 秒を超えたら、止まって人に尋ねる。

**完了の条件:** グリーンゲート（`_context.md` の "Green gate"）が通る。製品のコードで振る舞いが変わるのはステップ 2-1 だけで、新しい部品はテストだけから呼ばれる（`make deadcode` はテストも根に数える）。

### PR-1 作成ポイント: transcript samples, session report reader, and report entry

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9

**推奨タイトル**: `feat(0095): transcript のサンプルを採り、最後の報告と PR の記録を読む部品と報告の入口を足す`

**レビュー観点**: Claude Code の `/runpr` の報告の NEXT が今の文法で読めたかと、その記録 / 受け付ける形の表のとおりに、項目のない行と型の違う行を拒み、知らない値と種類を読み飛ばしているか / サンプルに採った人を特定できる値（PR の記録の URL を含む）が残っていないか / 報告の入口の種類と理由が設計書 3.3 の表と合うか

**実装モデル要件**: opus

**判定理由**: ステップ 1-1 は、今の `/runpr` の報告が文法で読めるかを確かめて要件に戻るかを決める実現性の確かめで、ステップ 2-2・2-3 は信頼できない入力の受け付ける形を決める信頼の境界の部品である。Sonnet で進めると、読めない報告を読めたと記録するか、項目のない行を読み飛ばして古い応答を報告にしやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/948
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: ①② を transcript に替える（設計書 8 章の 3）

**変更するファイル:** `internal/chief/autosend.go`・`autosend_test.go`（出力の項目の確かめを含む）、`internal/chief/output.go`、`internal/chief/sim_test.go`

-   [ ] **ステップ 3-1**: `reportsAgree` を、Claude Code の pane では報告の入口と `readReportNext` で、そのほかの pane では今のとおり画面で読むように変える（設計書 3.4）。読めないときは、送らない理由に pane と入口の `Detail` を含める。理由が作業ディレクトリの文字であるときは、`transcriptFailure` が示す、人が取れる対処も含める（AC-14）。
-   [ ] **ステップ 3-2**: `detachedReport` を、`/runpr` の送り先が Claude Code のときは報告の入口で読むように変える（設計書 3.5）。読めないときも報告がないときも、送らない理由に入口の `Detail` を含める。
-   [ ] **ステップ 3-3**: `resetRunpr` を、`/runpr` の送り先が Claude Code のときは報告の入口で読むように変える。読めないときの理由の値は入口の `Reason` にし、報告がないか PR とコマンドを名指さないときは今の `no_report` にする（設計書 3.5）。
-   [ ] **ステップ 3-4**: ①を通って送ったときも、Claude Code の pane ごとの照合の結果（照合した、報告がない（理由とパス）、NEXT を名指す行がない）を `AutoSendResult` に残し、`AutoSendOutput` に項目を足す（設計書 4.2）。残すのは、排他の中で読み直したとき（`recheck`）の結果とする。送るかどうかを決めたのはその読み直しだからである。項目の名前は実装で決める。`/chief` の定義には PR-3 で書く（§1.3 の決定 3）。
-   [ ] **ステップ 3-5**: 自動送信のテスト（`internal/chief/autosend_test.go`）を直す。今のテストの報告は、どれも OpenCode の pane（`w1:p2`）の画面にある `/runpr` の報告である（`mergeWorld.merge`、`internal/chief/autosend_test.go:164-178`）。Claude Code の pane（`w1:p1`）は `claude-idle.ansi` を示すだけで、送り先の表は `/runpr` を OpenCode に送る。そのため、今の行は OpenCode の行として変えずに通し（AC-13）、Claude Code の行はすべて新しく足す。
    -   **①の行。** 送り先の表は変えず、`w1:p1` の transcript に報告を置く。設計書 4.1 の表の①の列の各行（読めない、作業ディレクトリの文字、ファイルがない、報告がない、NEXT を名指す行がない、NEXT が読めない・違う・ない・食い違う）を確かめる。作業ディレクトリの文字の行には、`_` を含む worktree のパスが要る。`newWorld` はこれを避けているので（`internal/chief/world_test.go:99-102`）、パスを変える手段を fake に足す。
    -   **②の行。** `/runpr` を Claude Code に送る送り先の表（`_context.md` を書き換えた写し、`internal/chief/send_test.go:720-725` と同じ手順）と、`/runpr` の報告と PR の記録を Claude Code の pane の transcript に書く `merge` の別形を、fake に足す。そのうえで、`detachedReport`（`TestAutoSendDetached`）と `resetRunpr`（`TestResetAfterAutoSend`・`TestResetChecksAgain`）の、設計書 4.1 の表の②の列の各行を確かめる。自動送信の後のリセットの行は、次のコマンドの送り先を `/runpr` の pane と別の pane にして、`same_pane` で止まらないようにする。
    -   **AC-06 の行。** 画面にだけ報告がある Claude Code の pane は報告がなく、transcript にだけ報告がある pane は報告がある。①②のそれぞれで確かめる。
    -   **書きかけの行。** transcript の最後の行が書きかけのとき、①が送らず、次の周に持ち越す（設計書 7.1）。
    -   **AC-11 の行。** 報告が送るものを決めないことを、報告と文書を分けて確かめる。NEXT の行が一致し、その後に別の文字が続く報告のとき、打った入力が文書の次のコマンドとちょうど同じであること。NEXT を名指す行のない報告のとき、文書の次のコマンドが送られること。報告の NEXT が違うとき送らないこと（設計書 7.4）。
    -   **出力の行。** ステップ 3-4 の項目が、照合した pane、報告がない pane、NEXT を名指す行がない pane で、それぞれの結果になることを、`TickOutcome` の `Output` で確かめる。

**完了の条件:** グリーンゲートが通る。①②は、Claude Code の pane で画面を読まない。

### PR-2 作成ポイント: next check and runpr pane reset read the transcript

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0095): 自動送信の前の NEXT の照合と /runpr の pane のリセットで、Claude Code の pane の報告を transcript から読む`

**レビュー観点**: transcript を読めないときに送らず空にせず、報告がないときと NEXT を名指す行がないときだけ①を通すか / OpenCode の pane の振る舞いとテストの行が変わっていないか / 送らない理由と pane ごとの結果に、入口の `Detail` と人ができることが含まれるか

**実装モデル要件**: opus

**判定理由**: 自動送信を止める条件の情報源を替え、読めない transcript では止まる方向に、最後の報告だけを照合する方向とファイルのない pane では通る方向に、同時に変える（`mkplan.md` の step 8 のパネル方式の契機のうち、止める方向と通す方向を同時に変える変更）。Sonnet で進めると、読めないと報告がないの扱いを取り違えて、送ってはいけないときに送りやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: ③④ を transcript に替え、報告の控えを足す（設計書 8 章の 4）

**変更するファイル:** `internal/chief/detail.go`・`detail_test.go`、`internal/chief/request.go`、`internal/chief/reset.go`・`reset_test.go`、`internal/chief/network_test.go`、`internal/chief/sim_test.go`、`internal/chief/autosend.go`（`reportSource` の移動）、`internal/chief/output.go`、`internal/chief/records.go`（`Protocol`）、`.claude/commands/chief.md`、`docs/dev/guides/chief_guide.md`（プロトコル番号の行だけ）

-   [ ] **ステップ 4-1**: `readReport` を、Claude Code の pane で報告の入口を呼び、最後の報告があり `Names` がその PR（`OpenPR` の番号と URL）について真の pane を候補にするように変える。候補がちょうど 1 つならその報告を `CapHead` で切って示し、そうでなければ PR の説明を示す。控えに残す `StateChangeSeq` は、transcript を読む前に読んだ `ws.Agents` の値とする（設計書 3.6）。`readReport`・`keptReport`・`CollectDetail` のコメントのうち、画面を読むことと、控えを 1 回だけ作ることを述べる文を、報告の控えを含む今の振る舞いに直す。`reportSource` は、使う所が `autosend.go` の OpenCode の分だけになるので、`autosend.go` に移す。
-   [ ] **ステップ 4-2**: 改名の一覧の `ReportFromScreen` を `ReportFromPane` に改名する。記録と出力の値 `report` は変えない（要件書 4.4）。
-   [ ] **ステップ 4-3**: PR の説明を示したときの理由（Claude Code の pane がない、各 pane が読めない・報告がない・PR の記録が名指さない、候補が 2 つ以上）を `PRDetail` と `DetailOutput` に足す（設計書 3.6・4.2）。項目の名前は実装で決める。
-   [ ] **ステップ 4-4**: `keptReport` を `keptPaneReport` で控えを読むように変え、出どころが pane の報告でない控えしかないときは pane を読み直して、報告がちょうど 1 つ読めれば報告の控えを作る（設計書 3.6）。`ensureDetail` を `keptPaneReport` で控えを読むように変える。`ensureDetail` は、控えが読めないとき（HEAD を読めない、記録を読めない、手元の HEAD が PR の head と違うなど）に `CollectDetail` を呼ぶので、`CollectDetail` と `keptReport` に、報告の控えを作ってよいかを呼ぶ側が型で渡す引数を足す。ゼロ値は「作らない」とし、`chief detail` だけが「作る」を渡す（設計書 1.2・3.6）。
-   [ ] **ステップ 4-5**: `resetAfterRunpr` の画面の確かめ（`internal/chief/reset.go:131-137`）と、`lastInputMadePR` の中の transcript の読み込み（同 55-62）を、どちらも設計書 3.7 の 1〜4 の確かめに替える。`lastInputMadePR` は人の最後の入力を引数で受け取る。控えの出どころの比べ（同 118）は `ReportFromPane` にする。`resetAfterRunpr` の契約のコメント（画面がまだ報告を示すこと）を、transcript の確かめに直す。
-   [ ] **ステップ 4-6**: `Protocol` を、実装の時点の値に 1 を足した値にする（§1.3 の決定 3）。`.claude/commands/chief.md` のすべての `--protocol` の値と、`docs/dev/guides/chief_guide.md` の `プロトコル番号` の行を合わせる。`chief.md` の、人に示す項目の説明に、①の pane ごとの結果（PR-2 で足した項目）と、PR の説明を示した理由を足す（設計書 4.2）。
-   [ ] **ステップ 4-7**: `TestDetailReportSource`・`TestDetailAfterReset`（`internal/chief/detail_test.go`）を、transcript の報告と PR の記録で与えるように直し、次の行を足す。各行で、示した出どころと、PR の説明を示した理由を確かめる。
    -   PR の記録が名指す pane が 1 つ・2 つ・ないとき。PR の記録が別の PR を名指すとき。
    -   PR の記録と本文を分けた行（AC-05）。本文が PR の URL を含み、PR の記録がないか別の PR を名指す pane は候補にならない。本文が URL を含まず、PR の記録がその PR を名指す pane は候補になる。
    -   読めない pane だけのときと、読めない pane と候補になる pane が 1 つずつあるとき（AC-14 の③。後者は報告を示す）。transcript のファイルがない pane。
    -   画面にだけ報告があり transcript にない pane（AC-06 の③）。
    -   Claude Code の pane がない workspace（`detailWorld` は Claude Code の pane を必ず持つので、OpenCode の pane だけの workspace を作る）。
    -   64 KiB を超える報告。示す本文が `headMaxBytes` 以下で、切ったことが示され、出どころが `report` であること（要件書 4.2 の示す量の上限）。
    -   `herdr agent get` が workspace の一覧と違う `StateChangeSeq` を返すとき、控えの `Seq` が一覧の値であり、続く `/runpr` の後のリセットが `pane_changed` で止まること（設計書 3.6 の pane の状態の番号を読む順）。
-   [ ] **ステップ 4-8**: 報告の控えのテスト `TestDetailReportRecord` を `detail_test.go` に足す。PR の説明から PR の詳細の控えを作った後に報告が書かれたとき、`chief detail` が報告の控えを 1 回だけ作り、④がそれを使うこと、2 回目の `chief detail` が控えを上書きしないこと、読み直しても報告がちょうど 1 つ見つからないときは今の控えを使い報告の控えを作らないことを確かめる（設計書 7.1）。`ensureDetail` が報告の控えを作らないことは、手元の HEAD が PR の head と違い、PR の head に PR の説明から作った控えがあるとき（`CollectDetail` を呼ぶ経路）に、`report-*.json` ができないことで確かめる。
-   [ ] **ステップ 4-9**: `TestResetAfterRunpr`・`TestResetChecks`・`TestDetailSavedBeforeReset`・`TestResetFailureAfterClear`・`TestNoResetAfterAnUnrecordedRunpr`（`internal/chief/reset_test.go`）を、transcript の報告と PR の記録で与えるように直す。「text typed after the command」「text typed while the command worked」の行（`internal/chief/reset_test.go:70-71`）の理由を `no_report` に直し、入力の後に終わった応答を置いて `other_input` を確かめる行を足す（設計書 3.11）。「the report left the screen」の行（同 101-104）は、画面を `claude-idle.ansi` に替えても transcript に報告が残るので、今の期待（`no_report`）では通らなくなる。この行は消さず、下の AC-06 の行（画面にだけ報告があるときは空にせず、transcript にだけ報告があるときは空にする）の組に書き換える。`TestResetChecks` の「unsent text」の行が使う `withInput(reportScreen(...))` は、`claude-idle.ansi` に入力を残した画面に替える（`reportScreen` は PR-4 で消すため）。次の行も足す。
    -   transcript のファイルがないとき `no_report` になる（設計書 3.7）。
    -   読めないとき `no_transcript`、作業ディレクトリの文字では `unsupported_path` になる（AC-14 の④）。
    -   PR の記録と本文を分けた行（AC-05・AC-10）。本文が PR の URL を含み PR の記録が名指さないときは空にせず、本文が URL を含まず PR の記録が名指すときは空にする。
    -   画面にだけ報告があるときは空にせず、transcript にだけ報告があるときは空にする（AC-06 の④）。
    -   送り先の表が選ばない pane の PR の記録がその PR を名指していても、リセットの候補は送り先の表の pane のままである（AC-11）。
    -   pane の状態を読んでから transcript を読むまでの間に状態が変わったとき、空にしない（設計書 7.1）。
    -   このタスクの前に画面から作った形の控えでも、同じ条件で空にする（要件書 4.4）。控えは新しいコードで作らず、このタスクの前の形の JSON（`Source` が `report`）を記録のディレクトリに直に書いて与える。
-   [ ] **ステップ 4-10**: `TestDetailAndSendRunOnlyReadsInTheRightPlaces`（`internal/chief/network_test.go:64`）で、画面の報告の代わりに transcript を与えるように直す。`Detail` の呼び出しが `herdr agent read` を 1 回も走らせないことと、リセットが `w1:p1` の画面を読むのが入力の直前の確かめ（`checkPane`、`internal/chief/send.go:320`）の 1 回だけであることを確かめる。
-   [ ] **ステップ 4-11**: `chief.md` に書いた 2 つの項目の名前が、`output.go` の `json` のタグと同じであることを、PR のレビューで読んで確かめる（§7）。

**完了の条件:** グリーンゲートが通る（`TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool` を含む）。③④は、Claude Code の pane で画面を読まない。

### PR-3 作成ポイント: PR detail and maker pane reset read the transcript, with the report record

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9 / 4-10 / 4-11

**推奨タイトル**: `feat(0095): PR の詳細と PR を作った pane のリセットで transcript の報告と PR の記録を読み、報告の控えを足す`

**レビュー観点**: PR の記録と最後の報告で候補を決め、読めない pane を報告なしとするか / 報告の控えを `chief detail` だけが作り、どちらの控えも上書きしないか / `resetAfterRunpr` の確かめの並びが変わらず、transcript を 1 回だけ読むか / `Protocol` と定義・手引きの番号と、人に示す 2 つの項目

**実装モデル要件**: opus

**判定理由**: 人が使っている pane を空にするかを決める確かめの情報源を替え、遅れて書かれた報告のための新しい記録と、その作る時機と読む順を足す（`mkplan.md` の step 8 のパネル方式の契機のうち、信頼の境界の変更）。Sonnet で進めると、`ensureDetail` で報告の控えを作るか、pane の状態の番号を transcript の後に読み直して、人の pane を空にしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 実地の確かめ、使われなくなる部品とテストを消す（設計書 8 章の 5、7.3）

**前提:** PR-3 のマージの後、人が `make install-chief` で `main` の chief を入れ直し、`/chief` のセッションを起動し直してから（PR-3 が `Protocol` を変えるので、古い定義のセッションは `protocol_mismatch` で拒まれる）、ステップ 5-1 の実地の確かめを終え、PR-4 の `/runplan` を始める（§1.3 の決定 4）。確かめには、Claude Code のコマンドが作り、まだ開いている PR が要る。そのときほかのタスクにそうした PR があればそれを使い、なければ、人がそうした PR ができるまで待つか、確かめの範囲を人が決める。

**変更するファイル:** `internal/chief/screen_report.go`、`internal/chief/screen_transcript.go`、`internal/chief/sanitize.go`・`sanitize_test.go`、`internal/chief/screen_test.go`、`internal/chief/samples_test.go`、`internal/chief/testdata/README.md`、`docs/dev/conventions/package_reference.md`、本計画

-   [ ] **ステップ 5-1**: 人が行った実地の確かめの結果を、§7 の「PR-4 の実行の記録」に書く。確かめるのは、画面より長い報告（`/mkarch`・`/mkplan`・`/runplan` など）が PR の詳細に示されること（AC-01・AC-09）と、`/runpr` の後に PR を作った pane が空にされること（AC-10）である。どちらかが期待と違えば、それ以上進まずに止まり、人に尋ねる。
-   [ ] **ステップ 5-2**: 消すコミットを 1 つ作る。このコミットには、次の削除と、ビルドとテストを通すのに要る編集だけを入れる。消す製品のコードとそのテストを同じコミットにするのは、`make coverage-compare` が、消した関数を下がった関数でなく消した関数として挙げるためである。消さずに済む編集（サンプルの一覧の行の移動など）は、ステップ 5-4 で別のコミットにする。
    -   [ ] `readCommandReport`
    -   [ ] `claudeLastMessage`
    -   [ ] `namesURL`
    -   [ ] `claudeMessageMark`・`claudeDoneMark`・`claudeIndent`・`claudeRecapMark`
    -   [ ] `readRunprReport` の Claude Code の分（OpenCode だけを読む部品にし、コメントを直す）
    -   [ ] `CapScreen`
    -   [ ] `readLastHumanInput`（各行はステップ 2-2 で `TestReadSessionReport` に移してある）
    -   [ ] `TestCapScreen`（`internal/chief/sanitize_test.go:35`）
    -   [ ] `TestReadCommandReport`（`internal/chief/screen_test.go:657`）
    -   [ ] `TestReadCommandReportUnderRecap`（同 696）
    -   [ ] `TestReadCommandReportUnderTitledBox`（同 725）
    -   [ ] `TestReadLastHumanInput`（同 809）
    -   [ ] `TestReadRunprReport` の、Claude Code の画面から報告を読む行（同 882-885）。913-915 行の、読まないことを確かめる行は残す（Claude Code の分を消した後も、Claude Code の pane と知らないエージェントは読まないので通る）
    -   [ ] テストの補助関数 `reportScreen`・`reportMention`・`isSampleReport`（同 785-805）。呼ぶ所が残らないことを確かめる（`TestResetChecks` の呼び出しは、ステップ 4-9 で替えてある）
-   [ ] **ステップ 5-3**: 消すコミットの親を BASE として `make coverage-compare` を走らせる（10 分以上の時間の上限か、バックグラウンドで）。最後の行が `coverage-compare: PASS` であることを確かめ、最後の行までの出力を、木の ID を変えずに消すコミットのメッセージに書き、PR の説明にも書く（CLAUDE.md の「Testing Strategy」）。`PASS` でなければ止まり、人に尋ねる。
-   [ ] **ステップ 5-4**: `samples_test.go` の `command report` の行の 2 つのサンプルを `input line` の行に移し、`command report` の行を消す（設計書 3.9）。`RunprReport.PRs` のコメントを、画面と transcript の両方に合うように直す（設計書 2.2）。`package_reference.md` の `internal/chief` の行から `CapScreen` を消す。`testdata/README.md` の `claude-command-report.ansi`・`claude-command-report-recap.ansi` の行に、今は入力欄の判定のテストが使うことを書く。
-   [ ] **ステップ 5-5**: ステップ 5-2 の一覧のうち、名前ごと消したもので、製品のコード・テスト・コメント・`docs/dev/conventions/package_reference.md` を探し、残っていないことを確かめる。一部だけを消した `readRunprReport`・`TestReadRunprReport` は、Claude Code の分の読み取りと行が残っておらず、OpenCode の分が残っていることを読んで確かめる。走らせたコマンドと出力を §7 の「PR-4 の実行の記録」に書く（設計書 7.3。`make deadcode` はテストが呼ぶ部品を報告しないため）。

**完了の条件:** グリーンゲートと `make deadcode` が通る。消すコミットに `coverage-compare: PASS` の出力がある。

### フェーズ 6: 文書を更新する（設計書 8 章の 6）

**変更するファイル:** `CLAUDE.md`、`docs/dev/guides/chief_guide.md`、`docs/dev/adr/0046-chief-of-staff.md`（食い違いがあるときだけ）、本計画（§7 の差分の記録）

-   [ ] **ステップ 6-1**: CLAUDE.md の「Declare, don't infer」の chief の画面の読み取りの例外の段落の、範囲と境界の文を、設計書 3.10 の 1 つ目の項のとおりに書き換える（AC-17）。ほかの文は変えない。
-   [ ] **ステップ 6-2**: chief 利用ガイドを、設計書 3.10 の 2 つ目の項のとおりに直す（AC-15）。「1. 前提条件」のエージェントの行に、transcript の形を確かめた Claude Code の版（フェーズ 1 で採ったサンプルの版）を書く。「8. 既知の制限」の「長い報告は画面から読めない」を置き換え、設計書 3.10 の 3 つの制限を書き、「worktree のパスの文字」に workspace の自動送信も止まることを足す。「7. 状態のディレクトリ」の workspace ごとの記録と片付けに、報告の控え（`report-*.json`）を足す。「8. 既知の制限」の「transcript の場所の規則が変わったとき」に、置き場所が変わると①が報告なしとして通り、③④は安全側に倒れることを足す（要件書「要件定義の際に決めたこと」の 4 つ目の項）。「5.1 保証すること」に、transcript の値や種類の名前だけが変わったときと、ファイルがないときは①が通ることを、保証の例外として書く（設計書 1.2）。
-   [ ] **ステップ 6-3**: ADR-0046 を読み、実装した振る舞いと食い違う文がないかを確かめる。ADR-0046 は、AC-16 の分を要件定義の PR（#912）と設計の PR（#920）で更新済みである。食い違いがあれば、ADR の規約（`docs/dev/conventions/adr.md`）に従って直し、その理由を PR の説明に書く。
-   [ ] **ステップ 6-4**: AC-17 の差分の記録を、§7 の「Diff record (AC-17)」に書く（`requirements_process.md` の「`diff` verification」の決まったコマンド）。

**完了の条件:** グリーンゲートが通る（`TestADRRecords`・`TestGuideValuesMatchTool` を含む）。§7 の差分の記録が書かれている。

### PR-4 作成ポイント: remove the Claude Code screen report reader and update the documents

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 6-1 / 6-2 / 6-3 / 6-4

**推奨タイトル**: `refactor(0095): Claude Code の画面から報告を読む部品を消し、CLAUDE.md の例外と chief 利用ガイドを直す`

**レビュー観点**: 実地の確かめの結果 / 消すコミットの `coverage-compare` の出力と、消し漏れの探索の結果 / CLAUDE.md の例外の範囲と境界の文が、設計書 3.10 のとおりで、差分がその段落に限られるか / 利用ガイドの既知の制限が、読めないときの振る舞いと残る危険を正しく述べるか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 5-2 は、ほかと切り離した高いリスクのステップ（部品とテストを 1 つのコミットで漏れなく消し、`coverage-compare` で確かめる）である。ステップ 6-1 は送る条件を変えない文書の書き換えなので、パネル方式の契機（信頼の境界の変更）には当たらない。Sonnet で進めると、補助関数やコメントの消し漏れを残すか、例外の範囲を広げる言い回しにしやすい。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 確かめ |
|---|---|---|
| M1（PR-1） | サンプル、`readSessionReport`・`readReportNext`・`readClaudeReport`・`keptPaneReport`・`CapHead` | `/runpr` の報告の NEXT が読め、1 つの transcript を 1 秒以内に読める |
| M2（PR-2） | ①② が transcript を読む | 自動送信のテストが通る |
| M3（PR-3） | ③④ が transcript を読み、報告の控えがある。`Protocol` を上げる | PR の詳細とリセットのテストが通る。PR-3 のマージの後に実地の確かめ |
| M4（PR-4） | 画面の報告の読み取りを消し、文書を直す | `coverage-compare: PASS`、差分の記録 |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 | サンプル、報告の読み取り部品、報告の入口、報告の控えの読み方、`CapHead`、読む時間の記録 | opus | auto |
| PR-2 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | ①② の情報源を transcript に替え、①の pane ごとの結果を出力に足す | opus | auto |
| PR-3 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9 / 4-10 / 4-11 | ③④ の情報源を transcript に替え、報告の控えを作り、`Protocol` と `/chief` の定義を直す | opus | auto |
| PR-4 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 6-1 / 6-2 / 6-3 / 6-4 | 実地の確かめの記録、消すコミット、CLAUDE.md、chief 利用ガイド | sonnet+opus-review | hold |

PR の分け方の理由は §1.3 の決定 3 にある。各 PR は単独でグリーンゲートを通る。PR-1 の部品はテストだけから呼ばれ、PR-2・PR-3 がそれを使う。PR-4 の `hold` は、PR-3 のマージの後の実地の確かめ（§1.3 の決定 4）のためである。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト。** 報告の読み取り部品は、フェーズ 1 の実物のサンプルと、それを書き換えた入力で確かめる（設計書 7.1）。書き換えた入力のテストは、書き換えが入力を変えたこと（書き換えの前後が違うこと）も確かめる。①〜④は、今の fake で transcript のファイルと Herdr の画面を置き、Claude Code の pane と OpenCode の pane の組み合わせで、設計書 4.1 の表の各行を確かめる。
-   **2 つの情報源を分ける行。** AC-06 は、画面と transcript のどちらか一方にだけ報告を置き、①〜④のそれぞれで transcript の側だけが効くことを確かめる。AC-05 は、③④で本文の URL と PR の記録のどちらか一方だけがその PR を名指す行を置き、PR の記録の側だけが効くことを確かめる（CLAUDE.md の「A layered path needs inputs only one layer can handle」）。
-   **後方互換。** このタスクの前に画面から作った形の PR の詳細の控え（`Source` が `report`）が、③④で今までどおり読まれ使われることを、ステップ 4-9 で確かめる（要件書 4.4）。OpenCode の pane の結果が変わらないことは、今のテストの OpenCode の行を変えずに通すことで確かめる（AC-13）。
-   **結合テストと手での確かめ。** 実の Herdr と Claude Code での確かめは、ステップ 5-1 の実地の確かめだけである。ユニットテストは本物の Herdr・Claude Code・GitHub を呼ばない。
-   **時間。** 読む時間は、ステップ 2-9 で一度だけ測る（§1.3 の決定 2）。
-   **消すテスト。** ステップ 5-3 の `make coverage-compare` で確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 今の Claude Code の `/runpr` の報告の NEXT が、今の文法で読めない | ステップ 1-1 で止まり、要件に戻る（設計書 1.2）。PR-1 の前の製品のコードは①②で画面を読んだままなので、止まっている間の振る舞いは変わらない |
| 本計画の PR の `/runpr` が Claude Code で走らず、サンプルがない | ステップ 1-1 で止まり、サンプルの出どころ（ほかの PR の `/runpr` を Claude Code で走らせるなど）を人に選んでもらう |
| 報告の読み取り部品が遅く、25 MB の transcript で 1 秒を超える | ステップ 2-9 で止まり、人に尋ねる。末尾から読む仕組みは足さない（設計書 1.2） |
| 0092・0093 も `Protocol` を上げ、同時に進むと値が衝突する | ステップ 4-6 で、実装の時点の値に 1 を足す。後からマージする PR は、`main` を取り込んだときに値を合わせ、`TestDocumentedProtocolMatches` が食い違いを拾う |
| 0092 が workspace を消すときに記録を消す処理を入れる | 0092 が先に入れば、PR-3 でその処理が報告の控えも消すように直す。後に入れば、0092 の実装が報告の控えを含める（設計書 3.6） |
| Claude Code の版が上がり、transcript の値や種類の名前が変わる | 報告がないものとして①が通りうる。残る危険として受け入れ、①の pane ごとの結果（ステップ 3-4）で見分けられるようにし、利用ガイドに書く（ステップ 6-2、設計書 1.2） |
| 実地の確かめで期待と違う振る舞いが見つかる | ステップ 5-1 で止まり、人に尋ねる。PR-4 の中で直すか、決定の変更として扱う |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9）
- [ ] PR-2 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
- [ ] PR-3 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9 / 4-10 / 4-11）
- [ ] PR-4 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 6-1 / 6-2 / 6-3 / 6-4）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストのパスは `internal/chief/` からの相対である（`cmd/` で始まるものを除く）。

| AC | 確かめ | 種類 | ステップ |
|---|---|---|---|
| AC-01 | `screen_transcript_report_test.go::TestReadSessionReport`（長い報告の全文）、`transcript_test.go::TestReadClaudeReport`、`detail_test.go::TestDetailReportSource`。実地の確かめ（§7 の「PR-4 の実行の記録」） | test、manual | 2-2 / 2-6 / 4-7 / 5-1 |
| AC-02 | `screen_transcript_report_test.go::TestReadSessionReport`（各場合）、`transcript_test.go::TestReadClaudeReport`（ファイルがない、ディレクトリがない）、`reset_test.go::TestResetAfterRunpr`（ファイルがないとき `no_report`） | test | 2-2 / 2-6 / 4-9 |
| AC-03 | `screen_transcript_report_test.go::TestReadSessionReport`（サブエージェントの行） | test | 2-2 |
| AC-04 | `screen_transcript_report_test.go::TestReadSessionReport`（合成の応答・recap・compact の要約） | test | 2-2 |
| AC-05 | `screen_transcript_report_test.go::TestSessionReportNames`、`detail_test.go::TestDetailReportSource`・`reset_test.go::TestResetAfterRunpr`（PR の記録と本文を分けた行） | test | 2-4 / 4-7 / 4-9 |
| AC-06 | `autosend_test.go::TestAutoSendReportMismatch`・`TestAutoSendDetached`・`TestResetAfterAutoSend`（①②）、`detail_test.go::TestDetailReportSource`（③）、`reset_test.go::TestResetAfterRunpr`（④）の、画面だけに報告がある行と transcript だけに報告がある行 | test | 3-5 / 4-7 / 4-9 |
| AC-07 | `screen_transcript_report_test.go::TestReadReportNext`、`autosend_test.go::TestAutoSendReportMismatch`（Claude Code の pane の行） | test | 2-5 / 3-5 |
| AC-08 | `screen_transcript_report_test.go::TestReadReportNext`（PR 番号）、`autosend_test.go::TestAutoSendDetached`・`TestResetAfterAutoSend`・`TestResetChecksAgain`（`/runpr` を Claude Code に送る送り先の表の行） | test | 2-5 / 3-5 |
| AC-09 | `detail_test.go::TestDetailReportSource`・`TestDetailReportRecord`・`TestDetailAfterReset`。実地の確かめ | test、manual | 4-7 / 4-8 / 5-1 |
| AC-10 | `reset_test.go::TestResetAfterRunpr`・`TestResetChecks`・`TestDetailSavedBeforeReset`、`detail_test.go::TestDetailReportRecord`（報告の控えを④が使う）。実地の確かめ | test、manual | 4-8 / 4-9 / 5-1 |
| AC-11 | `autosend_test.go::TestAutoSendReportMismatch`（AC-11 の行: NEXT の後に別の文字が続く報告と、NEXT のない報告で、打った入力が文書の次のコマンドであること）、`reset_test.go::TestResetAfterRunpr`（リセットの候補は送り先の表で決まり、PR の記録の有無で変わらない行） | test | 3-5 / 4-9 |
| AC-12 | `transcript_test.go::TestReadClaudeReport`（設計書 3.3 の表の読めない各行と、上限を 1 バイト超える入力）、`screen_transcript_report_test.go::TestReadSessionReportRejects` | test | 2-3 / 2-6 |
| AC-13 | `autosend_test.go` の今の行（報告はすべて OpenCode の pane にある。`w1:p1` の transcript に報告がないまま、変えずに通す）、`screen_test.go::TestReadRunprReport` の OpenCode の行 | test | 3-5 / 5-2 |
| AC-14 | `autosend_test.go::TestAutoSendReportMismatch`・`TestAutoSendDetached`・`TestResetAfterAutoSend`（読めない、作業ディレクトリの文字）、`detail_test.go::TestDetailReportSource`（読めない pane）、`reset_test.go::TestResetAfterRunpr`（`no_transcript`・`unsupported_path`） | test | 3-5 / 4-7 / 4-9 |
| AC-15 | `docs/dev/guides/chief_guide.md` の 1 章・7 章・8 章を PR のレビューで読む。プロトコル番号の値は `cmd/chief/docs_test.go::TestGuideValuesMatchTool` | manual、test | 6-2 / 4-6 |
| AC-16 | `docs/dev/adr/0046-chief-of-staff.md` を PR のレビューで読む（#912・#920 で更新済み。ステップ 6-3 で実装との食い違いを確かめる）。ADR の形は `cmd/yt2column/adr_test.go::TestADRRecords` | manual、test | 6-3 |
| AC-17 | 差分の記録を読む（下の「Diff record (AC-17)」。比べるパスは `CLAUDE.md`）。範囲と境界の文の中身は PR のレビューで読む | diff、manual | 6-1 / 6-4 |

本計画のほかの確かめ（受け入れ基準に対応しないもの）は次のとおりである。

-   `Protocol` と定義の `--protocol` が等しいこと: `cmd/chief/docs_test.go::TestDocumentedProtocolMatches`（ステップ 4-6）。
-   サンプルに採った人を特定できる値がないこと: ホームのパス・UUID・noreply のアドレスは `samples_test.go::TestSamplesFollowReplacementRules`。GitHub のログイン名と氏名はこのテストが見つけないので、ステップ 1-3 で読まない行と項目を削り、ステップ 1-4 で一度だけ探した結果を「PR-1 の実行の記録」に残す（manual）。
-   `/chief` の定義に書いた出力の項目の名前が `output.go` の `json` のタグと同じであること: PR-3 のレビューで読む（manual、ステップ 4-11）。
-   消したテストが確かめていた振る舞いが残ること: ステップ 5-3 の `make coverage-compare` の出力（消すコミットのメッセージと PR の説明）。

Diff record (AC-17):

- Compared paths: `CLAUDE.md`

### PR-1 の実行の記録

-   **ステップ 1-1（2026-11-17）。** 本タスクの worktree の transcript のディレクトリで、人が入力したコマンドの行が `/runpr 940` であるセッションは 1 つだけだった（Claude Code 2.1.325）。その最後の報告の NEXT の行は `NEXT: なし（タスク文書の承認がない PR のため）` で、今の文法で読めた（`nextLine` に合い、`readNextValue` が「なし」と読む）。この報告を `testdata/transcript/claude-runpr-report.jsonl` に採った。
-   **ステップ 1-4（2026-11-17）。** 足した 6 つのサンプルで、利用者の GitHub のログイン名、氏名、名前の小文字の綴りを、大文字と小文字を区別せずに探した。見つかったのは 0 件だった。一時ディレクトリのパス（`/tmp/`・`/var/folders`・`/private/`）も探し、`/runpr` の報告の `/tmp/runpr.` の後の乱数の部分を `XXXXXX` に置き換えた後は、その置き換えた形のほかに見つからなかった。`TestSamplesFollowReplacementRules` も通った。
-   **ステップ 2-9（2026-11-17）。** 手元でいちばん大きい transcript（25,155,306 バイト）を、`readSessionReport` を足したコミット `8f5b6dadbb7f` の上で、一時的なテストから `-race` を付けずに 3 回読んだ。かかった時間は 113 ms・136 ms・110 ms で、1 秒以内だった（結果は報告あり）。一時的なテストはコミットしていない。

### PR-4 の実行の記録

（ステップ 5-1 の実地の確かめの結果と、ステップ 5-5 の消し漏れの探索のコマンドと出力を、`/runplan` が書く。）

## 8. 成功の基準 (Success Criteria)

-   **機能:** AC-01〜AC-17 のすべてが §7 の確かめで満たされる。
-   **品質:** グリーンゲートと `make deadcode` が通る。消すコミットの `make coverage-compare` が `PASS` である。各テストは、確かめる部品を壊して失敗することを確かめてからコミットされている。
-   **セキュリティ:** 受け付ける形に合わない transcript は読めないものとして扱われ、報告の本文と PR の記録は、送るコマンドとタスクと送り先を決めない（AC-11・AC-12、要件書 4.2）。サンプルに採った人を特定できる値がない。
-   **文書:** CLAUDE.md の例外、chief 利用ガイド、ADR-0046 が、実装した振る舞いと合う。

## 9. 次のステップ (Next Steps)

-   PR-4 のマージの後、issue #905 と #892 を閉じる（要件書の「要件定義の際に決めたこと」）。
-   Claude Code の版が上がったときは、transcript のサンプルを採り直す（chief 利用ガイドの「1. 前提条件」）。
-   0096（Claude Code の質問の判定）の設計で、本タスクの報告の読み取り部品を使うかを決める（設計書 9 章）。

## 変更履歴

- 2026-11-17 承認（isseis、#935）
- 2026-11-17 セッション中の再承認：ステップ 1-1 のサンプルを、runpr.md に NEXT の行を定めた PR の `/runpr` から採ることにした（isseis、#940）
