# 実装計画書：chief が Claude Code の質問を transcript から判定し、答えの中継の照合も transcript で行う

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-17 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `8252ed17` で確かめた。設計書が確かめたコミット `97ca1fc7` から、`internal/chief`・`cmd/chief`・`internal/strictjson`・`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` は変わっていない（`git diff --stat 97ca1fc7 8252ed17 -- <これらのパス>` の出力が空だった）。そのため、設計書が引くこれらの行番号は今も合う。変わったのは `CLAUDE.md` と `docs/dev/conventions/package_reference.md` だけで、設計書が引く CLAUDE.md の chief の例外の行は、`8252ed17` では 489 行（「Declare, don't infer」）と 811 行（「Go Idioms」）にある。

用語は、要件書 3 章・6 章、設計書の冒頭（待ち状態・中継の前の画面の確かめ・質問による止め）、[0088 計画書](../0088_chief_of_staff/03_implementation_plan.md) の冒頭と同じ意味で使う。本計画は、次の用語を加える。

-   **切り替え:** 状況の表、人の判断を待つもの、送る前の確かめ、答えの中継が、Claude Code の pane について、画面の代わりに待ち状態を使うようにすること。PR-3 で一度に行う（§1.3 の決定 1）。
-   **採取用の pane:** 人がサンプルの状態を作る Herdr の workspace `chief-poc` の Claude Code の pane（設計書 7.1）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、`internal/chief` に transcript の答えを待っている呼び出しの読み取りと待ち状態の判定を足し、Claude Code の pane の状況の表、送る前の確かめ、答えの中継の情報源を transcript に切り替える。あわせて `/chief` の定義、CLAUDE.md、chief 利用ガイドを直し、要件書の F-001〜F-006 を満たす。

### 1.2. 実装の原則

-   **部品を先に作り、切り替えは 1 つの PR で行う。** サンプルと読み取り（PR-1）→ 配線しない判定と確かめの部品（PR-2）→ 切り替えと `Protocol`（PR-3）→ 文書（PR-4）の順にする（§1.3 の決定 1）。PR-1 と PR-2 をマージして道具を入れ直しても、表・送信・中継は変わらない。新しい部品はテストからだけ呼ばれる。
-   **中身を調べるのは `screen` で始まるファイルだけにする。** 呼び出しの種類も読み取りが型の値として宣言し、ほかのファイルは型だけで分ける（設計書 1.1）。
-   **外部のコマンドと transcript はテストの中で用意する。** ユニットテストは herdr・git・gh を本物で呼ばない（0088 計画書の決定 19）。transcript は、一時的なディレクトリに置いたファイルで与える。本物の Herdr と Claude Code を使うのは、PR-1 のサンプルの採取と PR-3 の実地の確かめだけである。
-   **各テストは、確かめる部分を壊して失敗することを確かめ、その結果を commit のメッセージに書く**（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **PR を 4 つに分ける（人が選んだ）。** 設計書 8 章の 1 と、2 のうち transcript の読み取りを PR-1 に、2 の残りと、3・4 の部品のうち配線しないものを PR-2 に、3・4 の配線と 5 のうち `/chief` の定義を PR-3 に、5 の残りの文書を PR-4 に置く。8 章の 6（実地の確かめ）は PR-3 に置く（決定 3）。8 章から動かすものと、その理由は次のとおりである。
    -   `Question` に足す欄と値（設計書 3.4.1）は、質問の呼び出しから `Question` を作る PR-1 に入れる。
    -   中継の照合値の部品（設計書 3.5.1）・中継の前の画面の確かめ（3.5.3 の `claudeScreenAgrees`）・質問による止め（3.6 の `questionStop`）は、配線せずに PR-2 で作る。PR-3 の差分を、配線と既存のテストの直しに絞るためである。
    -   表と中継の切り替えは、同じ PR-3 で行う。分けると、間の期間、表が示す照合値（transcript から作る）と中継が比べる照合値（画面から作る）が合わず、Claude Code の pane への中継が必ず `check_mismatch` で送られなくなるためである。
    -   出力の形を変える PR では、`Protocol`、`/chief` の定義の `--protocol`、手引きのプロトコル番号を同時に変える必要がある（`cmd/chief/docs_test.go:205` の `TestDocumentedProtocolMatches`、`cmd/chief/docs_test.go:405` の `TestGuideValuesMatchTool`）。そのため、出力の変更と `/chief` の定義の直しも PR-3 に置く。手引きの文と CLAUDE.md の文の残りは PR-4 に置く。
    -   CLAUDE.md の「Declare, don't infer」と「Go Idioms」の chief の例外、および `internal/chief/doc.go` には、transcript から呼び出しを読むことを、その読み取りを入れる PR-1 で書き足す（0092 計画書の決定 1 と同じ考え方）。画面を送るのを止めるためだけに使うことは、PR-4 で書く。

    ほかの案は採らない。PR-4 を PR-3 に含める 3 つの案は、PR-3 がさらに大きくなるためである。表と中継を分ける 5 つの案は、上の照合値のずれのためである。ADR: なし（PR の分け方はこのタスクの中で閉じるため）。
2.  **64 MiB の transcript の判定時間は、ベンチマークを残して測る（人が選んだ。設計書 1.2・7.2）。** PR-2 で `go test -bench` のときだけ走るベンチマーク `BenchmarkReadClaudeWait` をコミットする。ID の重複しない 64 MiB の transcript を一時的なディレクトリに作り、`readClaudeWait` で読む。時間とメモリの最大使用量を測って、§7 の「PR-2 の実行の記録」に書く。`make test` は時間を確かめない。`-race` の下や CI では時間がぶれ、常に走る時間の確かめは不安定になるためである。測り直しで 1 秒の予算（要件書 4.1）を超えたときは、そこで止まり、読み方を人と決め直す（決定の変更）。一時的に測って消す案は、予算を超えたかを後で測り直す手段が残らないので採らない。ADR: なし（測り方はこのタスクの中で閉じるため）。
3.  **実地の確かめ（設計書 7.4）は、PR-3 のマージの前に行う（人が選んだ）。** 0092・0093 と同じく、人が、動いている `/chief` を止め、chief の worktree を PR-3 のブランチの先頭（detached）にし、`make install-chief` で入れ直してから、chief の workspace で `/chief` を起動し直す（0092 計画書の決定 12）。マージの後、人が chief の worktree を `main` に戻し、`main` から入れ直して `/chief` を起動し直す。マージの後に確かめる案は、問題が見つかったときに `main` に入った後で直すことになるので採らない。ADR: なし。
4.  **64 MiB の上限の定数の名前は `sessionTranscriptLimit` にする（設計書 3.2.3 が計画に委ねた）。** 0095 の計画が、今の `resetTranscriptLimit`（`internal/chief/transcript.go:30`）を同じ名前に改名する（0095 計画書の改名の一覧）。PR-2 を作るときに `origin/main` を確かめ、0095 がすでに改名していればその名前を使い、していなければ本計画が同じ名前に改名する。どちらの順に入っても、同じ名前に合わさる。ADR: なし（定数の名前）。
5.  **PR-1 のサンプルが設計書の前提と違ったときは、そこで止まって人と決め直す。** 設計書が結果に応じた扱いをすでに決めているもの（3.5.3 の、手順 3 の後に画面をもう一度読む確かめを足すこと、選択肢の名前と説明をつないで比べること）は、その扱いを PR-2 か PR-3 のステップに入れ、§7 の「PR-1 の実行の記録」に、どの結果からどの扱いを選んだかを書く。設計書が決めていないもの（権限の確認の間に Herdr が `blocked` と報告しない、自由に書く答えに打った文字が画面で見分けられず数字のキーが文字として入る）は、要件書か設計書を `draft` に戻して決め直す（要件書 1 章の背景、設計書 1.2・3.5.3）。ADR: なし。
6.  **`Protocol` は、PR-3 を実装するときの `origin/main` の値に 1 を足した値にする（設計書 1.2）。** 0092・0093・0095 も `Protocol` を上げる。PR-3 を作る前と、マージの直前の 2 回、`origin/main` の `internal/chief/records.go` の `Protocol` を確かめる。マージの直前の確かめは、0093 計画書のステップ 3-11 と同じく、PR の説明に人が行う項目として書く。`main` はマージで取り込み、rebase しない（0099 の規則）。ADR: なし（0099 が ADR-0014 で定めた規則に従うだけのため）。

### 1.4. 既存コード調査結果

コミット `8252ed17` で確かめた。設計書が計画に委ねた点（定数の名前、`findRelayTarget` の戻り値の型、時間の測り方）のうち、型の形は実装で決め、ほかは §1.3 で決めた。複数の実装の案が残る箇所はない。

| 領域 | あるもの | 足りないもの・変えること |
|---|---|---|
| transcript の場所と読み込み | `transcriptPath`（`internal/chief/transcript.go:53`）、`transcriptFailure`（`:75`）、`readTranscript`（`:84`）、`openTranscript`（`:101`）、`errTranscript`（`:17`）。上限は `transcriptLimit`（1 MiB）と `resetTranscriptLimit`（64 MiB）（`:28-31`）。上限を超えたときの理由は文字列（`:95`）。ファイルがないときは `fs.ErrNotExist` を `%w` で含む（`:106`、`internal/chief/send_test.go:825` が確かめる） | `errTranscriptLimit`（設計書 4.1）、`readClaudeWait`（3.3）。定数の改名（決定 4）。PR-2 |
| transcript の行の読み取り | `readModelSwitch`・`readLastHumanInput`（`internal/chief/screen_transcript.go:46`・`:170`）。どちらも 1 行でも読めなければ全体を読めないとし、本計画でも変えない（要件書の「要件定義の際に決めたこと」） | 新しいファイル `screen_transcript_calls.go` の `readOpenCalls`（設計書 3.2）。PR-1 |
| 問いの型 | `QuestionKind`・`RelayBlock`（`internal/chief/screen.go:26-57`）、`Question`（`:71`）、`finishQuestion`（`:166`）、`mergeQuestionText`（`:113`） | 3.4.1 の欄と値。PR-1 |
| 画面の読み取り | `parseClaudeScreen`（`internal/chief/screen_claude.go:69`）、自由に書く答えとチャットの名前（`:15-18`・`:29`）、画面のサンプルの期待 `claudeScreenWants`（`internal/chief/screen_test.go`）、サンプルの一覧 `screenRows`・`TestScreenSamplesCoverEveryRow`（`internal/chief/samples_test.go:99`・`:132`）、置き換えの確かめ `TestSamplesFollowReplacementRules`（`:72`。`testdata` の下のすべてのファイルを読む） | `claudeScreenAgrees`（3.5.3）。PR-2。今の読み取りは変えず、#908 に残す（設計書 1.2） |
| 照合値 | `checkValue`（`internal/chief/relay.go:35`）、`relayable`（`:61`）、`relayCheck(a Agent, q Question)`（`:74`） | 並べてハッシュにする部分を共有し、`transcriptCheckValue` を足す（PR-2）。`relayCheck` の引数を `QuestionItem` にする（3.5.1。PR-3） |
| 中継 | `paneQuestion`（`internal/chief/relay.go:125`）、`findRelayTarget` の戻り値 `(relayTarget, found, readFailed bool)`（`:149`）、`Relay`（`:177`）、`relay` の確かめ（`:210-247`） | Claude Code の pane の探し方と理由（3.5.2）、送る直前の 3 つの確かめ（3.5.3）。戻り値の型は実装で決める。PR-3 |
| 送らない理由 | `NotSentReason` の並びは `NotSentConditions` で終わる（`internal/chief/send.go:85-148`） | `NotSentScreenMismatch`（`screen_mismatch`）。PR-3 |
| 送信の記録 | `SendRecord`（`internal/chief/records.go:658-667`）、`Protocol = 1`（`:28`） | 呼び出しの ID の欄（PR-3）、`Protocol`（決定 6。PR-3） |
| 表 | `agentFacts`（`internal/chief/state.go:149`）、`QuestionItem`（`:103`）、`classify`（`:382`。画面を読めない pane と知らない形の画面を「要確認」にする：`:390-410`） | `agentFacts.Wait`・`QuestionItem.CallID`・`Working`（PR-2 で欄を足す）。`classify` の Claude Code の pane の分け方（3.4.2。PR-3） |
| 状況の読み取り | `readWorkspace(ctx, herdr, git, tasks, w)`（`internal/chief/snapshot.go:230`）。呼び出し元は `snapshot.go:132`・`request.go:221`・`autosend.go:660` の 3 か所 | `Env` を引数に足し、Claude Code の pane ごとに `readClaudeWait` を呼ぶ。PR-3 |
| 送る前の確かめ | `offered`（`internal/chief/request.go:312`）、`quietness`（`internal/chief/autosend.go:376`）、`checkPane`（`internal/chief/send.go:331-342`） | `questionStop`（PR-2）と、その呼び出し（PR-3）。`checkPane` は変えない |
| 出力と項目 | `QuestionOutput`（`internal/chief/output.go:59`）、`questionKindName`（`:110`）、`questionOutput`（`:147`）、`questionID(workspace, q Question)`（`internal/chief/pending.go:84`） | 3.4.3 の欄と `several_questions`、`questionID` の引数を `QuestionItem` にする。PR-3 |
| テストの世界 | 状況の表の世界 `world`（`internal/chief/world_test.go:79`）は、エージェントにセッション ID を持たせ（`:159`）、`Env.Home` を一時的なディレクトリにする（`:126`）。transcript のファイルは置かない。送信の世界は transcript を書く `writeTranscript`（`internal/chief/sim_test.go:113`）を持つ | 状況の表の世界に、Claude Code の pane の transcript を置く口。0095 が `sim_test.go` に transcript の行の補助を足していれば使う。PR-3 |
| 文書のガード | `TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestGuideValuesMatchTool`（`cmd/chief/docs_test.go:205`・`:257`・`:405`）。`--protocol 1` は `.claude/commands/chief.md` の 97・106・228・232・238・248・250・254・307 行、手引きのプロトコル番号は `docs/dev/guides/chief_guide.md:28` | テストは変えない。番号を新しい値にする。PR-3 |

-   **改名の一覧。** 本計画が変える名前は次の表がすべてである。各ステップ、受け入れ基準の表、文書は、この表に従う。

    | 今の名前 | 新しい名前 | PR |
    |---|---|---|
    | `resetTranscriptLimit` | `sessionTranscriptLimit`（0095 が先に改名していれば、本計画は改名しない。決定 4） | PR-2 |

-   **変える関数の引数。** `readWorkspace` に `Env` を足す（呼び出し元の 3 か所）。`relayCheck` と `questionID` の 2 つめの引数を `Question` から `QuestionItem` にする。どれも `internal/chief` の中だけで使われる（`8252ed17` で `rg -n 'readWorkspace\(|relayCheck\(|questionID\(' --glob '!docs/**' .` を走らせ、結果は `internal/chief` の中だけだった）。
-   **書き換える箇所の一覧（PR-3）。** `--protocol 1` を書き換える箇所は、上の表の「文書のガード」の行がすべてである（`8252ed17` で `rg -n -- '--protocol [0-9]' .claude/commands docs/dev/guides/chief_guide.md CLAUDE.md` を走らせ、`chief.md` の結果は表の 9 行だった。ほかは別の道具 `fixprguard` の `.claude/commands/fixpr.md`・`runpr.md` と `codexreview` の `.claude/commands/_lib/local-cli-review.md` の行で、変えない）。PR-3 を作る時点の行番号は探し直す。変え残しは `TestDocumentedProtocolMatches` が見つける。
-   **既存のテストの扱い。** 設計書 3.9 の表のテストは、名前を保ったまま中身を直す。Claude Code の場合を確かめていたテスト関数を消すか、別の名前の関数に置き換えるときは、その削除を単独のコミットにし、`make coverage-compare BASE=<親のコミット>` の結果を commit のメッセージと PR の説明に書く（CLAUDE.md の「Testing Strategy」）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: サンプル、transcript の読み取り（設計書 8 章の 1、2 のうち読み取り）

**変更するファイル:** `internal/chief/testdata/transcript/`・`testdata/screens/`（サンプルを足す）、`internal/chief/testdata/README.md`、`internal/chief/screen_transcript_calls.go`・`screen_transcript_calls_test.go`（新規）、`internal/chief/screen.go`、`internal/chief/screen_test.go`・`samples_test.go`、`internal/chief/doc.go`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`、本計画（§7 の記録）

-   [ ] **ステップ 1-1**: 採取の準備をする。Claude Code と Herdr の版を記録する。人に、採取用の pane で作る状態の順と、各状態でセッションが読むもの（`herdr agent get`・`herdr agent read`、transcript のファイルの写し）を示す。セッションは読み取りだけを行う（`internal/chief/testdata/README.md` の「採り方」）。
-   [ ] **ステップ 1-2**: 設計書 7.1 の 6 つの項目を採る。権限の確認の間の `herdr agent get` の状態を最初に確かめ、`blocked` でなければ、決定 5 のとおり止まる。親の transcript にない問いの部品（設計書 3.5.3）、打った文字の画面、`/compact`・`--resume`・`/rewind` の後の transcript は、採れたものと採れなかったものを分けて記録する。
-   [ ] **ステップ 1-3**: 採ったものから、採取した人を特定できる情報（利用者の名前、ホームのパス、セッション ID、問いと選択肢と道具の入力の中身のうち作業の中身）を、`testdata/README.md` の「置き換えたもの」のとおりに置き換える（security.md §7）。transcript の行は、要件書 4.2 の「使う部分」と行の種類を表すメンバーに絞る（今の `claude-human-input.jsonl` と同じ）。ただし、`api-request-blob` の行は、同じ呼び出しの ID を持つことが確かめの対象なので残す（H-06）。版と採った日と絞り方を README に書く。機械で確かめられる新しい規則は `samples_test.go` の置き換えの規則に足す。
-   [ ] **ステップ 1-4**: 足した画面のサンプルを `claudeScreenWants` と `screenRows` に足し、今の `parseClaudeScreen` の読み取りの期待を書く。中継の前の画面の確かめのためのサンプルは、`screenRows` に行を足して入れ、その行を README の表にも足す。
-   [ ] **ステップ 1-5**: `screen.go` の `QuestionKind`・`RelayBlock` に `QuestionSeveral`・`RelayBlockSeveral`・`RelayBlockTooMany` を、`Question` に `Header`・`Parts`・`Tool`・`Input`・`InputCut` を足し、`Question` のコメントを直す（設計書 3.4.1）。
-   [ ] **ステップ 1-6**: `screen_transcript_calls.go` に `CallKind`・`OpenCall`・`readOpenCalls`・`errTranscriptForm` と、質問の呼び出しから `Question` を作る部分を書く（設計書 3.2.1・3.2.2）。テストは次を確かめる。
    -   ステップ 1-2 の各サンプルの、答えを待っている呼び出しの一覧と種類。答えのある呼び出し（`is_error` の答えを含む）、`api-request-blob` の行の同じ ID、対応する呼び出しのない答え。
    -   要件書 4.2 の拒む入力の行を 1 つずつ。誤りが `errors.Is(err, errTranscript)` と `errors.Is(err, errTranscriptForm)` を満たし、行の番号を含み、行の中身を含まないこと。
    -   改行で終わっていない最後の行を読まないこと、空白だけの行。
    -   設計書 3.2.2 の表の各行（選択肢が 1・7・8・9・10 の境界を含む）、`n+1`・`n+2` の番号の選択肢、`Parts` の各問いの番号、マージの確認の見分け（`RelayAllowed` の問いだけ）。
-   [ ] **ステップ 1-7**: CLAUDE.md の「Declare, don't infer」の chief の例外の範囲に、Claude Code の transcript の答えを待っている呼び出しを読み、呼び出しの種類、問いの形・文・選択肢、マージの確認を読むことを足す。「Go Idioms」の chief の例外に、transcript の問いの文と同じでなければならない文字列を足す（設計書 3.8、決定 1）。`internal/chief/doc.go` の同じ説明も直す。`package_reference.md` の `internal/chief` の行の、画面の読み取りが返す型に `OpenCall` を足す。
-   [ ] **ステップ 1-8**: §7 の「PR-1 の実行の記録」に、版、ステップ 1-2 の各項目の結果、決定 5 で選んだ扱いを書く。

**完了の条件:** ステップ 1-6 のテストが通り、§7 に PR-1 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: samples and the transcript reader of open calls

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8

**推奨タイトル**: `feat(0096): 答えを待っている間のサンプルを採り、transcript の答えを待っている呼び出しの読み取りを作る`

**レビュー観点**: 権限の確認の間の Herdr の状態など、サンプルの結果と決定 5 の扱い / サンプルに採取した人を特定できる情報が残っていないか / 要件書 4.2 の拒む入力をすべて拒み、広げてよい部分のほかを受け付けていないか / CLAUDE.md の例外の範囲の書き足し

**実装モデル要件**: opus

**判定理由**: ステップ 1-2 は、確かめていない外部の振る舞い（権限の確認の間の Herdr の状態、答えを待っている間の transcript、打った文字の画面）を本物の Claude Code で確かめる Phase 0 型の確かめである。Sonnet は、想定と違う結果を確かめたものとして記録し、要件書を戻すべき食い違いを後の PR へ持ち越しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 判定と確かめの部品（設計書 8 章の 2 の残り、3・4 のうち配線しない部品）

**変更するファイル:** `internal/chief/transcript.go`、`internal/chief/claudewait.go`・`claudewait_test.go`（新規）、`internal/chief/reset.go`（改名の分）、`internal/chief/screen_claude.go`・`screen_test.go`、`internal/chief/relay.go`・`relay_test.go`、`internal/chief/state.go`、`docs/dev/conventions/package_reference.md`、本計画（§7 の記録）

-   [ ] **ステップ 2-1**: `origin/main` で 0095 の改名を確かめ、決定 4 のとおり上限の定数の名前を決める。`readTranscript` が上限を超えたときに `errTranscriptLimit` を `errTranscript` に含めて返すようにする（設計書 3.2.3・4.1）。リセットの振る舞いは変えず、今の `reset_test.go` のテストは変えずに通す。
-   [ ] **ステップ 2-2**: `claudewait.go` に `ClaudeWaitKind`・`ClaudeWait`・`judgeClaudeWait`・`permissionQuestion`・`permissionInputLimit` を書き、`transcript.go` に `readClaudeWait` を書く（設計書 3.3）。`Why` は設計書 4.2 の文の形にする。テストは次を確かめる。
    -   `judgeClaudeWait` を、設計書 3.3 の表の各行について、その行より前の行に当たらない入力で確かめる。状態のゼロ値と、表にない状態の値も含める。
    -   `readClaudeWait` で、場所が決まらない（セッションがない、作業ディレクトリの文字、設定のディレクトリ）、ファイルがない、上限を超える、通常のファイルでない、形の合わない行のそれぞれ。上限は `errors.Is(err, errTranscriptLimit)` で見分ける。
    -   `Why` が pane の ID を含み、transcript の問いの文・選択肢・道具の入力を含まないこと。ひとりでは消えない「要確認」で `/clear` を添えること。
    -   `permissionQuestion` の切り方を、2,000 文字と 2,001 文字（複数バイトの文字を含む）の境界で確かめる。
-   [ ] **ステップ 2-3**: `screen_claude.go` に `claudeScreenAgrees` を書く（設計書 3.5.3）。§7 の「PR-1 の実行の記録」で選んだ扱い（名前と説明をつないで比べるか）を入れる。テストは、PR-1 の実物の画面のサンプルだけで、設計書 7.2 の「中継の前の画面の確かめ」の各場合と、`Type something.` を名前に持つ選択肢の画面が合わないこと（設計書 7.5）を確かめる。`why` が画面の中身を含まないことも確かめる。
-   [ ] **ステップ 2-4**: `relay.go` で、`checkValue` の、部分を長さ付きで並べてハッシュにする部分を共有の関数にし、`transcriptCheckValue` と `transcriptCheckVersion` を足す（設計書 3.5.1）。今の `TestCheckValueChanges` は変えずに通す。テストは、設計書 3.5.1 の部分のそれぞれを変えると値が変わること、選択の位置で値が変わらないこと、同じ中身の画面の照合値と等しくならないことを確かめる。
-   [ ] **ステップ 2-5**: `state.go` の `agentFacts` に `Wait`、`QuestionItem` に `CallID`・`Working` を足し、`claudewait.go` に `questionStop` を書く（設計書 3.6）。テストは、AC-24 の (a)〜(d)、`blocked` でない pane の画面の質問、読めない画面で止めること、知らない形の画面と OpenCode の pane の `Wait` のゼロ値で止めないことを確かめる。
-   [ ] **ステップ 2-6**: `claudewait_test.go` に `BenchmarkReadClaudeWait` を書き（決定 2）、1 回走らせて、時間とメモリの最大使用量（`/usr/bin/time -l` の最大常駐量）を §7 の「PR-2 の実行の記録」に、走らせたコマンド・機械・コミットとともに書く。1 秒を超えれば、決定 2 のとおり止まる。
-   [ ] **ステップ 2-7**: `package_reference.md` の `internal/chief` の行の、画面の読み取りが返す型に `ClaudeWait` を足す。

**完了の条件:** 各ステップのテストが通り、§7 に PR-2 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。新しい部品はテストからだけ呼ばれ、表と送信の振る舞いは変わらない。

### PR-2 作成ポイント: wait judgement, screen agreement, check value, and question stop

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0096): 待ち状態の判定、中継の前の画面の確かめ、transcript の照合値、質問による止めの部品を作る`

**レビュー観点**: 待ち状態の表が設計書 3.3 のとおりで、表にない状態を「要確認」に倒しているか / `claudeScreenAgrees` が実物のサンプルで、合わない画面をすべて合わないとしているか / 照合値が設計書 3.5.1 の部分をすべて含むか / `Why` と `why` が transcript と画面の中身を含まないか / ベンチマークの結果が 1 秒の予算に収まるか

**実装モデル要件**: opus

**判定理由**: ステップ 2-3・2-5 は、人が選んでいない入力をキーで送らないための守り（中継の前の画面の確かめと質問による止め）を作る、安全の境界のステップである。Sonnet は、合わない画面や表にない状態を合うもの・止めないものとして通し、送る側に倒しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 切り替え、出力、`Protocol`、`/chief` の定義、実地の確かめ（設計書 8 章の 3・4・6、5 のうち定義）

**変更するファイル:** `internal/chief/snapshot.go`・`state.go`・`output.go`・`pending.go`・`request.go`・`autosend.go`・`relay.go`・`send.go`・`records.go` とそのテスト（`snapshot_test.go`・`pending_test.go`・`relay_test.go`・`send_test.go`・`autosend_test.go`・`wait_test.go`）、`internal/chief/world_test.go`・`sim_test.go`、`.claude/commands/chief.md`、`docs/dev/guides/chief_guide.md`（プロトコル番号の行だけ）、0092 の削除の扱い（ステップ 3-6）で直すファイル、本計画（§7 の記録）

-   [ ] **ステップ 3-1**: 状況の表のテストの世界に、Claude Code の pane の transcript を置く口を足す（設計書 3.9）。transcript の行は PR-1 のサンプルか、その形に合わせた行で作る。0095 の transcript の行の補助が `main` にあれば使う。
-   [ ] **ステップ 3-2**: `readWorkspace` に `Env` を足し、Claude Code の pane ごとに `readClaudeWait` を呼ぶ。`classify` を設計書 3.4.2 の表のとおりにし、`relayCheck` の引数を `QuestionItem` にする（設計書 3.5.1）。設計書 3.9 の表の状況の表のテストを、transcript で同じ状態を作る形に直す。新しいテストで、AC-01〜AC-13・AC-19〜AC-23・AC-26 の各状態の行と項目を確かめる。画面が知らない形でも、Claude Code の pane の行が「要確認」にならないことを含める。
-   [ ] **ステップ 3-3**: 出力を設計書 3.4.3 のとおりにし（`QuestionPartOutput`、足す欄、`several_questions`）、`questionID` を Claude Code の項目について呼び出しの ID から作る。`TestPendingQuestionIdentity` を直す。テストで、種類ごとの使う欄と空の欄、制御文字の置き換え、空の配列が `[]` であることを確かめる。
-   [ ] **ステップ 3-4**: `Send` が行を決めた後に `questionStop` を確かめ、`offered` が送らないときに「要確認」の理由を添えるようにする。`quietness` が最初に `questionStop` を呼ぶようにする（設計書 3.6）。`TestSendChecksBeforeFirstInput` を直す。テストで、コマンドの送信と自動送信が AC-24 の (a)〜(d) のそれぞれで送らないこと、送り先でない Claude Code の pane の画面が質問を示すときにコマンドの送信が送らないこと（AC-25）を確かめる。
-   [ ] **ステップ 3-5**: 中継を設計書 3.5.2・3.5.3 のとおりにする。`NotSentScreenMismatch` と、`SendRecord` の呼び出しの ID の欄を足す。§7 の「PR-1 の実行の記録」で、親の transcript にない問いの部品が重なりうると分かっていれば、手順 3 の後に画面をもう一度読む確かめを足す（決定 5）。設計書 3.9 の表の中継のテストの Claude Code の場合を直し、OpenCode の場合は変えずに通す。新しいテストで、設計書 7.2 の「中継」の各場合と、送信の記録の呼び出しの ID を確かめる。
-   [ ] **ステップ 3-6**: 0092 の削除のコードとの扱いを、設計書 3.6 の手順で決める。PR を作る前に `origin/main` を確かめ、削除のコードがあれば `agent_active` の確かめに `questionStop` を足してテストを足す。なければ 0092 の設計書と計画書に書き足す（requirements_process.md の「Editing an approved document」の決定の変更として扱う）。どちらにしたかと、確かめた `origin/main` のコミットを §7 の「PR-3 の実行の記録」に書く。
-   [ ] **ステップ 3-7**: `Protocol` を決定 6 の値にする。`.claude/commands/chief.md` を設計書 3.4.3 のとおりに直し、すべての `--protocol` を同じ値にする。手引きのプロトコル番号の行を同じ値にする。PR の説明に、マージの直前に人が `origin/main` の `Protocol` を確かめる項目を書く（決定 6）。
-   [ ] **ステップ 3-8**: 実地の確かめをする（設計書 7.4、決定 3）。人が chief の worktree をこのブランチにして入れ直し、`/chief` で、採取用の pane の Claude Code の 1 つを選ぶ問いを示して中継できること、複数の問い・権限の確認・サブエージェントの待ちが中継されずに示されることを確かめる。結果を §7 の「PR-3 の実行の記録」に書く。

**完了の条件:** 各ステップのテストが通り、§7 に PR-3 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る（`TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool` を含む）。

### PR-3 作成ポイント: switch Claude Code questions, relays, and send checks to the transcript

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8

**推奨タイトル**: `feat(0096): Claude Code の質問の判定と中継の照合を transcript に切り替え、/chief の定義と Protocol を直す`

**レビュー観点**: `classify` が Claude Code の pane の画面を見ず、待ち状態だけで分けるか / 中継の送る直前の 3 つの確かめの順と、どれかが合わないときに何も送らないか / 送信と自動送信が `questionStop` で止まり、`checkPane` の確かめが変わらないか / 既存のテストの直しが期待を新しい振る舞いに合わせるだけになっていないか / `Protocol` と `/chief` の定義と手引きの番号、0092 の削除の扱い、実地の確かめの結果

**実装モデル要件**: opus

**判定理由**: ステップ 3-2〜3-5 は、0088 の表・送信・中継の振る舞いを変え、多くの既存のテストを直す移行で、`Protocol` を上げる（mkplan.md の panel-mode の条件の migration）。Sonnet は、テストの期待を新しい振る舞いに合わせるだけで直し、OpenCode の pane や送る前の確かめのうち変えない振る舞いを壊しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 文書（設計書 8 章の 5 の残り）

**変更するファイル:** `CLAUDE.md`、`docs/dev/guides/chief_guide.md`、`docs/dev/conventions/package_reference.md`（変え残しがあるときだけ）

-   [ ] **ステップ 4-1**: CLAUDE.md の「Declare, don't infer」の chief の例外を、Claude Code の pane の質問を transcript から読み、画面からの判断は送るのを止めるためにだけ使い、中継の前に画面の 1 つを選ぶ問いが transcript の問いと同じかを確かめる、という振る舞いに合わせる（設計書 3.8、AC-27）。
-   [ ] **ステップ 4-2**: chief 利用ガイドの 1 章の表、5.1 の中継の説明、8 章の 4 つの項目（中継しない質問、transcript の場所の規則、ひとりでは消えない「要確認」、`screen_mismatch` が続くとき）を設計書 3.8 のとおりに直す（AC-28）。§7 の「PR-1 の実行の記録」の `/compact`・`--resume`・`/rewind` の結果のうち、「要確認」がひとりでは消えなくなる操作も 8 章に書く。
-   [ ] **ステップ 4-3**: `package_reference.md` の `internal/chief` の行が、PR-1〜PR-3 で足した型（`OpenCall`・`ClaudeWait`）を挙げていることを確かめ、足りなければ直す。

**完了の条件:** `make test`・`make lint` が通る。

### PR-4 作成ポイント: CLAUDE.md exception and chief guide

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `docs(0096): CLAUDE.md の chief の例外と chief 利用ガイドを transcript による質問の判定に合わせる`

**レビュー観点**: CLAUDE.md の例外の範囲と境界が、PR-3 の振る舞い（画面は送るのを止めるためだけ）と合うか / 利用ガイドの中継の照合と中継しない質問の説明が F-001〜F-003 と合うか / ひとりでは消えない「要確認」と `screen_mismatch` が続くときに人ができること

**実装モデル要件**: sonnet

**判定理由**: 文書の直しだけで、mkplan2.md の Model requirement のどの条件にも当たらない。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（PR-1） | サンプル、PR-1 の実行の記録、`readOpenCalls`、`Question` の欄と値、CLAUDE.md の例外の範囲の書き足し | 要件書と設計書の前提が確かめられる。読み取りはテストからだけ呼ばれる |
| M2（PR-2） | `readClaudeWait`・`judgeClaudeWait`、`claudeScreenAgrees`、`transcriptCheckValue`、`questionStop`、ベンチマークと測定の記録 | 部品がそろう（まだ配線されない） |
| M3（PR-3） | 切り替え、出力、`Protocol`、`/chief` の定義、0092 の削除の扱い、実地の確かめ | chief が Claude Code の質問を transcript で示し、中継する |
| M4（PR-4） | CLAUDE.md の例外、chief 利用ガイド | 文書が振る舞いと合う |

順序は設計書 8 章のとおりで、8 章から動かしたものは §1.3 の決定 1 にすべて挙げた。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 | サンプルの採取、`readOpenCalls`、`Question` の欄と値、CLAUDE.md の例外の範囲 | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | 待ち状態の判定、中継の前の画面の確かめ、transcript の照合値、質問による止め、ベンチマーク | opus | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 | 表・出力・送る前の確かめ・中継の切り替え、`Protocol`、`/chief` の定義、0092 の削除の扱い、実地の確かめ | opus | auto |
| PR-4 | 4-1 / 4-2 / 4-3 | CLAUDE.md の例外、chief 利用ガイド | sonnet | auto |

どの PR も、前の PR のマージと `/runplan` の間に、`/runplan` の外の作業を置かない。PR-1 のサンプルの採取と PR-3 の実地の確かめは、`/runplan` のセッションの中で人と行う。PR-3 のマージの後に chief の worktree を `main` に戻す作業（決定 3）は、PR-4 の `/runplan` の前提ではない。そのため、自動送信はすべて `auto` とする。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 読み取り・判定・画面の確かめ・照合値・質問による止めは、PR-1・PR-2 で、ほかの部品に依らないユニットテストで確かめる。transcript は PR-1 のサンプルと、そこから 1 か所を変えた行で与え、一時的なディレクトリに置く。画面は実物のサンプルだけを使う（設計書 7.2）。
-   **層ごとの入力:** `judgeClaudeWait` の表の行は、その行より前の行に当たらない入力で確かめる（CLAUDE.md の「A layered path needs inputs only one layer can handle」）。PR-3 の表のテストでは、画面が質問の形で transcript に呼び出しがない入力と、その逆の入力を置き、どちらの情報源で決まったかを分けて確かめる。
-   **結合の確かめ:** 状況の表・送信・自動送信・中継は、PR-3 で、Herdr と git を模したテストの世界に transcript を置いて確かめる（設計書 3.9）。
-   **誤り:** 誤りは `errors.Is` で見分ける（`errTranscript`・`errTranscriptForm`・`errTranscriptLimit`・`fs.ErrNotExist`）。
-   **境界の値:** 選択肢の数（1・7・8・9・10）、権限の確認の入力の 2,000 文字、読む量の 64 MiB を、境界の前後で確かめる。
-   **セキュリティ:** 設計書 7.5 の項目を、ステップ 1-6・2-2・2-3・3-3 のテストで確かめる（§7 の表の後の注）。
-   **性能:** `BenchmarkReadClaudeWait` を PR-2 で 1 回走らせて記録する（決定 2）。`make test` は時間を確かめない。
-   **テストの補助:** `internal/chief` の中だけで使う補助は、今の `test_helpers.go` か、テストの世界のファイル（`world_test.go`・`sim_test.go`）に置く（test_organization.md の Classification B）。ほかのパッケージから使う補助は作らない。テストの日本語の文字列は、CLAUDE.md の「Go Idioms」の例外の範囲（`screen` で始まるファイルのテスト）に限り、まとまった画面と transcript はサンプルのファイルに置く。
-   **文書のガード:** `cmd/chief/docs_test.go` の今のテストが、`--protocol` の番号、呼び出しの形、手引きのプロトコル番号を確かめる。テストは変えない。CLAUDE.md・利用ガイド・`/chief` の定義の文は、PR のレビューで読む（設計書 7.3）。
-   **後方互換:** OpenCode の pane の表・送信・中継は、今のテストが変わらずに通ることで確かめる。0088 の `/chief` の定義との組み合わせは、`Protocol` で拒む（決定 6）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 権限の確認の間に Herdr が `blocked` と報告しない、など要件書の前提が成り立たない | PR-1 の最初（ステップ 1-2）で確かめ、決定 5 のとおり止まって要件書を `draft` に戻す |
| 自由に書く答えに打った文字が画面で見分けられず、数字のキーが文字として入る | PR-1 で確かめ、見分けられなければ決定 5 のとおり止まる（設計書 3.5.3） |
| 親の transcript にない問いの部品が重なり、中継のキーが別の部品に入る | PR-1 で確かめ、重なりうるなら PR-3 で手順 3 の後に画面を読み直す（ステップ 3-5） |
| サンプルに採取した人を特定できる情報や作業の中身が残る | ステップ 1-3 の置き換えと `TestSamplesFollowReplacementRules`、PR-1 のレビュー観点 |
| 64 MiB の transcript の判定が 1 秒を超える | PR-2 のベンチマークで測り、超えれば決定 2 のとおり止まる |
| 切り替えの既存のテストの直しで、変えない振る舞いを壊す | OpenCode の場合と送る前の確かめ（`checkPane`）のテストは変えずに通す。テスト関数を消すときはカバレッジを比べる（§1.4） |
| 0092・0093・0095 と `Protocol`・`transcript.go`・テストの世界がぶつかる | `Protocol` は決定 6 のとおり 2 回確かめる。上限の定数は決定 4 のとおり同じ名前に合わせる。0095 の補助が `main` にあれば使う（ステップ 3-1） |
| 0092 の削除の確かめが `questionStop` を知らずに入る | ステップ 3-6 で `main` を確かめ、コードか 0092 の文書のどちらかに足す（設計書 3.6） |
| 日程: PR-1 のサンプルの採取と PR-3 の実地の確かめで、人の手が空かない | 人がいるときにだけ進める。`/runplan` は人を待つだけで、ほかのタスクを妨げない。PR-3 の実地の確かめの間だけ、ほかのタスクの待ち受けが止まる（決定 3） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

表のパスの `ic/` は `internal/chief/` を表す。テストの名前は計画の時点の名前である。実装で名前を変えたときは、この表を同じ PR で直す（編集上の訂正）。`manual` は、文書と `/chief` の定義の文を PR のレビューで読むことと、PR-3 の実地の確かめを指す。

| AC | 確かめ | 種類 | ステップ | PR |
|---|---|---|---|---|
| AC-01 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（番号、`n+1`・`n+2`）、`ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows`、`ic/snapshot_test.go::TestClaudeQuestionOutput`（見出し・説明・制御文字）、定義の示し方 | test、manual | 1-6・2-2・3-2・3-3・3-7 | PR-1〜PR-3 |
| AC-02 | `ic/screen_transcript_calls_test.go::TestReadOpenCallsSamples`（`is_error` の答え）、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 1-6・3-2 | PR-1、PR-3 |
| AC-03 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（`QuestionSeveral`）、`ic/snapshot_test.go::TestClaudeQuestionOutput`（`parts`、中継しない）、定義の示し方 | test、manual | 1-6・3-3・3-7 | PR-1、PR-3 |
| AC-04 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（`QuestionMulti`）、`ic/snapshot_test.go::TestClaudeQuestionOutput` | test | 1-6・3-3 | PR-1、PR-3 |
| AC-05 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（選択肢 1・7・8・9・10）、`ic/snapshot_test.go::TestClaudeWaitRows`（中継しない問いに照合値がない） | test | 1-6・3-2 | PR-1、PR-3 |
| AC-06 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows`、`ic/relay_test.go::TestRelayClaudeReasons`（`not_accepting`） | test | 2-2・3-2・3-5 | PR-2、PR-3 |
| AC-07 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・3-2 | PR-2、PR-3 |
| AC-08 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（マージの確認）、`ic/snapshot_test.go::TestClaudeWaitRows`（`MergeQuestion`） | test | 1-6・3-2 | PR-1、PR-3 |
| AC-09 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・3-2 | PR-2、PR-3 |
| AC-10 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/claudewait_test.go::TestPermissionQuestion`（2,000 文字の境界）、`ic/snapshot_test.go::TestClaudeQuestionOutput`（`tool`・`input`・`input_cut`、制御文字）、定義の示し方 | test、manual | 2-2・3-3・3-7 | PR-2、PR-3 |
| AC-11 | `ic/screen_transcript_calls_test.go::TestReadOpenCallsSamples`（`CallSubagent`）、`ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 1-6・2-2・3-2 | PR-1〜PR-3 |
| AC-12 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows`、`ic/snapshot_test.go::TestTakeSnapshotStates`（`blocked on an unknown shape`） | test | 2-2・3-2 | PR-2、PR-3 |
| AC-13 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・3-2 | PR-2、PR-3 |
| AC-14 | `ic/relay_test.go::TestRelayClaudeSendsChosenOption`、`ic/relay_test.go::TestRelayClaudeRechecksBeforeTheKey`（画面を読んだ後に答えられ同じ文で問い直される、transcript に答えが書かれる、Herdr の状態が変わる） | test | 3-5 | PR-3 |
| AC-15 | `ic/screen_test.go::TestClaudeScreenAgrees`、`ic/relay_test.go::TestRelayClaudeScreenMismatch`（送らず、行は「要確認」にならない）、実地の確かめ | test、manual | 2-3・3-5・3-8 | PR-2、PR-3 |
| AC-16 | `ic/relay_test.go::TestRelayClaudeSendsChosenOption`（番号のキーだけ）、`ic/relay_test.go::TestRelayRefusesOption`（変えずに通る） | test | 3-5 | PR-3 |
| AC-17 | `ic/relay_test.go::TestTranscriptCheckValue`、`ic/relay_test.go::TestRelayClaudeReasons`（`check_mismatch`） | test | 2-4・3-5 | PR-2、PR-3 |
| AC-18 | `ic/relay_test.go::TestRelayFailureAfterKeyStops`・`ic/relay_test.go::TestRelayUnreadablePane` と、中継の排他・停止・記録の今のテスト（Claude Code の場合を transcript で作り直しても、確かめる守りは変えない） | test | 3-5 | PR-3 |
| AC-19 | `ic/claudewait_test.go::TestReadClaudeWait`（場所が決まらない 3 つ）、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・3-2 | PR-2、PR-3 |
| AC-20 | `ic/claudewait_test.go::TestReadClaudeWait`（ファイルがない）、`ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・3-2 | PR-2、PR-3 |
| AC-21 | `ic/screen_transcript_calls_test.go::TestReadOpenCallsRejects`、`ic/screen_transcript_calls_test.go::TestReadOpenCallsPartialLastLine`、`ic/claudewait_test.go::TestReadClaudeWait` | test | 1-6・2-2 | PR-1、PR-2 |
| AC-22 | `ic/claudewait_test.go::TestReadClaudeWait`（`errTranscriptLimit`） | test | 2-1・2-2 | PR-2 |
| AC-23 | `ic/claudewait_test.go::TestReadClaudeWait`（開けない、通常のファイルでない）、`ic/snapshot_test.go::TestClaudeWaitRows`、`ic/relay_test.go::TestRelayClaudeReasons`（`no_transcript`・`unsupported_path`） | test | 2-2・3-2・3-5 | PR-2、PR-3 |
| AC-24 | `ic/claudewait_test.go::TestQuestionStop`、`ic/send_test.go::TestSendStopsOnQuestion`、`ic/autosend_test.go::TestAutoSendStopsOnQuestion`。workspace の削除は、ステップ 3-6 で削除のコードがあれば足すテスト、なければ 0092 の文書の書き足し | test、manual | 2-5・3-4・3-6 | PR-2、PR-3 |
| AC-25 | `ic/claudewait_test.go::TestQuestionStop`（画面の質問、読めない画面）、`ic/send_test.go::TestSendStopsOnQuestion`（送り先でない pane の画面の質問）、`ic/send_test.go::TestSendChecksBeforeFirstInput`（入力欄の形の確かめ） | test | 2-5・3-4 | PR-2、PR-3 |
| AC-26 | `ic/snapshot_test.go::TestClaudeWaitRows`（知らない形の画面）、`ic/claudewait_test.go::TestQuestionStop`（知らない形で止めない） | test | 2-5・3-2 | PR-2、PR-3 |
| AC-27 | CLAUDE.md の「Declare, don't infer」と「Go Idioms」の chief の例外の文（自然言語の文だけを定める） | manual | 1-7・4-1 | PR-1、PR-4 |
| AC-28 | chief 利用ガイドの 1 章の表・中継の照合・中継しない質問の文（自然言語の文だけを定める） | manual | 4-2 | PR-4 |

-   要件書 4.1（性能）は、`ic/claudewait_test.go::BenchmarkReadClaudeWait` と §7 の「PR-2 の実行の記録」で確かめる（ステップ 2-6）。
-   要件書 4.2・設計書 7.5 のセキュリティの項目は、次の行が受け持つ。制御文字の置き換えは AC-01・AC-10 の行。2,000 文字を超える入力を切ることは AC-10 の行。`Type something.` を名前に持つ選択肢の画面が合わないことは AC-15 の行。行の形の違いで transcript 全体を読めなくすることは AC-21 の行。`Why` と `why` に transcript の中身が出ないことは、ステップ 2-2・2-3 のテスト（`TestReadClaudeWait`・`TestClaudeScreenAgrees`）。
-   `Protocol` と `/chief` の定義・手引きの番号がそろうことは、今の `cc/docs_test.go::TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool`（`cc/` は `cmd/chief/`）が確かめる（ステップ 3-7）。

### PR-1 の実行の記録

（PR-1 の `/runplan` が書く。）

### PR-2 の実行の記録

（PR-2 の `/runplan` が書く。）

### PR-3 の実行の記録

（PR-3 の `/runplan` が書く。）

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 の表のすべての行の確かめが通り、PR-3 の実地の確かめで設計書 7.4 の項目を 1 回以上確かめた。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。各テストは壊して失敗を確かめ、commit のメッセージに書いた。テスト関数を消したときは、カバレッジの比べの結果を書いた。
-   **性能:** 64 MiB の transcript の判定が 1 秒以内であることを、PR-2 のベンチマークで記録した。
-   **セキュリティ:** 設計書 7.5 の項目をテストで確かめ、送る入力の範囲（AC-16）と送る直前の確かめ（AC-14・AC-15）をテストと実地の確かめで確かめた。
-   **文書:** `/chief` の定義、CLAUDE.md、chief 利用ガイド、`package_reference.md` が実装と一致する。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、人が決定 3 のとおり chief の worktree を `main` に戻し、`main` から `make install-chief` で入れ直して `/chief` を起動し直す。
-   使わなくなった Claude Code の画面の読み取りの整理は、#908 で扱う（設計書 1.2・9 章）。
-   選択肢が 8 以上の問いの中継と、transcript の行の読み方を 0095 とそろえることは、設計書 9 章のとおり別のタスクで考える。
