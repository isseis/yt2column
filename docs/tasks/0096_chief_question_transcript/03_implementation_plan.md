# 実装計画書：chief が Claude Code の質問を transcript から判定し、答えの中継の照合も transcript で行う

## Document Status

| Item | Value |
|---|---|
| Status | `withdrawn` |
| Created | 2026-11-17 |
| Review date | 2026-11-17 |
| Reviewer | isseis |
| Comments | 2026-11-17 取り下げ：Claude Code が答えを待っている質問の呼び出しを transcript に書かないことがあると分かったため（§7「PR-1 の実行の記録」） |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。

既存のファイルの内容、`file:line` の行番号、§1.4 のコマンドの結果は、`origin/main` のコミット `62d867cc` で確かめた。設計書が確かめたコミット `97ca1fc7` から、`internal/chief` で変わったのは、0095 の PR-1（#948）と 0092 の PR-1（#923）が変えたファイルだけである。`transcript.go`・`transcript_test.go`（新規）・`records.go`・`reset.go`・`samples_test.go`・`screen_transcript.go`・`screen_transcript_report.go`（新規）・`herdr.go`・`sanitize.go`・`detail.go`・`screen_report.go`・`screen_test.go` とそのテスト、`testdata/` がそれに当たる。設計書が引く `relay.go`・`state.go`・`snapshot.go`・`autosend.go`・`request.go`・`send.go`・`output.go`・`pending.go`・`screen.go`・`screen_claude.go` とテストの世界、`cmd/chief`・`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` は変わっていない（`git diff --stat 97ca1fc7 62d867cc -- <これらのパス>` の出力が空だった）。設計書の行番号は、`transcript.go` で 1 行ずれ（`resetTranscriptLimit` の `:30` は今の `sessionTranscriptLimit` の `:31`、`transcriptFailure` の `:75` は `:76`）、`records.go` の `SendRecord` は `:662` にある。CLAUDE.md の chief の例外は 489 行（「Declare, don't infer」）と 811 行（「Go Idioms」）にある。

0093 と 0095 の残りの PR も、`internal/chief` の同じファイルを変える（§5 のリスクの表）。そのため、各 PR の `/runplan` は、始める前に、その PR が変えるファイルについて §1.4 の調べ（行番号、「直す既存のテスト」、`--protocol` の行）を、そのときの `origin/main` でやり直す。行番号の違いは編集上の訂正として本計画を直す。振る舞いの違いが見つかれば、決定の変更として扱う。

用語は、要件書 3 章・6 章、設計書の冒頭（待ち状態・中継の前の画面の確かめ・質問による止め）、[0088 計画書](../0088_chief_of_staff/03_implementation_plan.md) の冒頭と同じ意味で使う。本計画は、次の用語を加える。

-   **切り替え:** 状況の表、人の判断を待つもの、答えの中継が、Claude Code の pane について、画面の代わりに待ち状態を使うようにすること。PR-4 で一度に行う（§1.3 の決定 1）。
-   **採取用の pane:** 人がサンプルの状態を作る Herdr の workspace `chief-poc` の Claude Code の pane（設計書 7.1）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、`internal/chief` に transcript の答えを待っている呼び出しの読み取りと待ち状態の判定を足し、Claude Code の pane の状況の表、送る前の確かめ、答えの中継の情報源を transcript に移す。あわせて `/chief` の定義、CLAUDE.md、chief 利用ガイドを直し、要件書の F-001〜F-006 を満たす。

### 1.2. 実装の原則

-   **部品を先に作り、止めを足してから切り替える。** サンプルと読み取り（PR-1）→ 配線しない判定と確かめの部品（PR-2）→ 質問による止めの配線（PR-3）→ 切り替えと `Protocol`（PR-4）→ 文書（PR-5）の順にする（§1.3 の決定 1）。PR-1 と PR-2 をマージして道具を入れ直しても、表・送信・中継は変わらない。PR-3 は送らない場合を足すだけで、出力は変えない。
-   **中身を調べるのは `screen` で始まるファイルだけにする。** 呼び出しの種類も読み取りが型の値として宣言し、ほかのファイルは型だけで分ける（設計書 1.1）。
-   **今ある部品を使う。** transcript の場所を決めて読む手順は、0095 の `readClaudeReport` と共有する（§1.3 の決定 7）。
-   **外部のコマンドと transcript はテストの中で用意する。** ユニットテストは herdr・git・gh を本物で呼ばない（0088 計画書の決定 19）。transcript は、一時的なディレクトリに置いたファイルで与える。本物の Herdr と Claude Code を使うのは、PR-1 のサンプルの採取と PR-4 の実地の確かめだけである。
-   **各テストは、対象の部分を壊して失敗することを確かめ、その結果を commit のメッセージに書く**（CLAUDE.md の「Testing Strategy」）。同じ出力を 2 つの層が作れるときは、対象の層だけが働く入力を使う（同じ節の「A layered path needs inputs only one layer can handle」）。

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **PR を 5 つに分ける（人が選んだ。最初は 4 つを選び、PR の区切りのレビューの後に、質問による止めの配線を独立させる 5 つに選び直した）。** 設計書 8 章は「PR の区切りと各段のステップは計画で決める」としている。本計画は、配線の順（送る前の確かめ → 表と中継 → 定義）と、文書の前に確かめることを保ち、部品を作る時期だけを前に寄せる。そのため設計書の決定は変えない。各 PR に置くものと理由は次のとおりである。
    -   **PR-1:** 8 章の 1 と、2 のうち transcript の読み取り。`Question` に足す欄と値（設計書 3.4.1）も、質問の呼び出しから `Question` を作るこの PR に入れる。
    -   **PR-2:** 2 の残りと、3・4 の部品のうち配線しないもの（中継の照合値、`claudeScreenAgrees`、`questionStop`）。PR-3・PR-4 の差分を、配線と既存のテストの直しに絞るためである。
    -   **PR-3:** 3 のうち質問による止めの配線（設計書 3.6）と、0092 の workspace の削除の確かめ（`agent_active`）の扱い（以下、0092 の削除の扱い）。止めは送らない場合を足すだけで、出力も `Protocol` も変えないので、切り替えと分けられる。表はまだ画面で決まるので、`questionStop` だけが止める入力（画面は入力欄で、transcript に答えを待っている呼び出しがある）でテストを書ける。ほかのタスクの文書を直しうる扱いも、大きな PR-4 から切り離せる。
    -   **PR-4:** 3・4 の残り（表・出力・中継の切り替え）、5 のうち `/chief` の定義、6（実地の確かめ。決定 3）。表と中継の切り替えは同じ PR で行う。分けると、2 つの PR の間は、表が示す照合値（transcript から作る）と中継が比べる照合値（画面から作る）が合わず、Claude Code の pane への中継が必ず `check_mismatch` で送られなくなるためである。出力を変えるので、`Protocol`、`/chief` の定義の `--protocol`、手引きのプロトコル番号も同時に変える（`cmd/chief/docs_test.go:205` の `TestDocumentedProtocolMatches`、`:405` の `TestGuideValuesMatchTool`）。
    -   **PR-5:** 5 の残りの文書（CLAUDE.md の例外の残り、手引きの文）。CLAUDE.md の「Declare, don't infer」と「Go Idioms」の chief の例外と `internal/chief/doc.go` には、transcript から呼び出しを読むことを、その読み取りを入れる PR-1 で書き足す（0092 計画書の決定 1 と同じ考え方）。画面からの判断を、送るのを止めるためだけに使うことは、PR-5 で書く。

    採らなかった案は次のとおりである。文書を切り替えの PR に含める案は、その PR がさらに大きくなるので採らない。表と中継を別の PR で切り替える案は、上の照合値のずれのために採らない。止めの配線を切り替えの PR に含める案は、PR が大きくなり、`questionStop` だけが止める入力を作りにくいので採らない。ADR: なし（PR の分け方はこのタスクの中で閉じるため）。
2.  **64 MiB の transcript の判定時間は、ベンチマークを残して測る（人が選んだ。設計書 1.2・7.2）。** PR-2 で、`go test -bench` のときだけ走るベンチマーク `BenchmarkReadClaudeWait` をコミットする。ID の重複しない、上限ちょうどの transcript を、計る範囲の外で一度だけ作る。行の形は PR-1 のサンプルに合わせる（長い応答の行と、呼び出しと答えの組の多い行の並び）。ベンチマークは `readClaudeWait` で読む。
    -   時間（1 回あたり）とメモリの割り当て、テストのバイナリのメモリの最大使用量を、§7 の「PR-2 の実行の記録」に書く。最大使用量は、transcript を作る分を含むことを添えて、参考として記録する。
    -   `make test` は時間を確かめない。`-race` の下や CI では時間がぶれ、常に走る時間の確かめは不安定になるためである。
    -   1 回の判定が 1 秒の予算（要件書 4.1）を超えたときは、そこで止まり、読み方を人と決め直す（決定の変更）。そのため、測定は `readClaudeWait` を作った直後に行い、PR-2 のほかの部品の前に置く（ステップ 2-3）。

    一時的に測って消す案は、予算を超えたかを後で測り直す手段が残らないので採らない。ADR: なし（測り方はこのタスクの中で閉じるため）。
3.  **実地の確かめ（設計書 7.4）は、PR-4 のマージの前に行う（人が選んだ）。** 0092・0093 と同じく、人が、動いている `/chief` を止め、chief の worktree を PR-4 のブランチの先頭（detached）にする。そこで `make install-chief` で入れ直してから、chief の workspace で `/chief` を起動し直す（0092 計画書の決定 12）。マージの後、人が chief の worktree を `main` に戻し、`main` から入れ直して `/chief` を起動し直す。
    -   その間、レビュー前の PR-4 の版が、ほかのタスクの workspace への自動送信も行う。0092・0093 の実地の確かめと同じ扱いとして受け入れる。
    -   実地の確かめで中継するのは、採取用の workspace だけにする。送信の記録の呼び出しの ID の欄は、中継の記録にだけ書き、ほかの記録には書かない（ステップ 4-4）。今の版は知らない欄を持つ記録を読まない（`internal/chief/records.go:166` の `DisallowUnknownFields`）。そのため、PR-4 をマージせずに古い版を入れ直すときは、人が採取用の workspace の送信の記録を消してから起動し直す。
    -   確かめの後、レビューの直しで中継（`relay.go`）か画面の確かめ（`claudeScreenAgrees`）を変えたときは、マージの前に、ステップ 4-6 の中継の 2 つの項目を確かめ直す。ほかの直しでは確かめ直さない。

    マージの後に確かめる案は、問題が見つかったときに `main` に入った後で直すことになるので採らない。ADR: なし。
4.  **64 MiB の上限は、0095 の PR-1（#948）が改名した `sessionTranscriptLimit`（`internal/chief/transcript.go:31`）を使う（設計書 3.2.3 が名前を計画に委ねた）。** 本計画は名前を変えない。ADR: なし（定数の名前）。
5.  **PR-1 のサンプルが設計書の前提と違ったときは、そこで止まって人と決め直す。** 設計書が結果に応じた扱いをすでに決めているものは、その扱いを PR-2 か PR-4 のステップに入れる。設計書 3.5.3 の手順 3 の後に画面をもう一度読む確かめを足すこと、選択肢の名前と説明をつないで比べることがそれに当たる（設計書 3.5.3）。どの結果からどの扱いを選んだかは、§7 の「PR-1 の実行の記録」に書く。設計書が決めていないものは、要件書か設計書を `draft` に戻して決め直す（要件書 1 章の背景、設計書 1.2・3.5.3）。権限の確認の間に Herdr が `blocked` と報告しないこと、自由に書く答えに打った文字が画面で見分けられず、数字のキーが文字として入ることがそれに当たる。ADR: なし。
6.  **`Protocol` は、PR-4 を実装するときの `origin/main` の値に 1 を足した値にする（設計書 1.2）。** 0092・0093・0095 も `Protocol` を上げる。PR-4 を作る前と、マージの直前の 2 回、`origin/main` の `internal/chief/records.go` の `Protocol` を確かめる。マージの直前の確かめは、0093 計画書のステップ 3-11 と同じく、PR の説明に人が行う項目として書く（ステップ 4-5）。`main` はマージで取り込み、rebase しない（0099 の規則）。ADR: なし（0099 が ADR-0014 で定めた規則に従うだけのため）。
7.  **transcript の場所を決めて読む手順は、0095 の `readClaudeReport`（`internal/chief/transcript.go:142`）と共有する（要件書 4.5）。** `readClaudeReport` は、設計書 3.3 の `readClaudeWait` と同じ手順を持つ。場所を決め（`transcriptPath`・`transcriptFailure`）、`sessionTranscriptLimit` で読み、ファイルがないことを見分け、探したパスと transcript のディレクトリの有無を説明に添える。PR-2 で、この手順を 1 つの関数に取り出し、両方が使う。`readClaudeReport` の振る舞いは変えず、`TestReadClaudeReport`（`internal/chief/transcript_test.go:20`）は変えずに通す。設計書 4.2 の「ファイルがないのに `blocked`」の文が求めるパスは、この関数の説明から取る。
    -   自動送信の周では、1 つの Claude Code の pane の transcript を、0095 の報告の読み取りと待ち状態の判定で 2 回読みうる。要件書 4.1 の予算は 1 回の判定に対するものである。2 回分の時間は、§7 の「PR-2 の実行の記録」に見積もりとして書く。

    ADR: なし（関数の分け方）。
8.  **`ClaudeWait` に、transcript を読めないときの理由の欄 `Reason` を足す（設計書 3.1・3.3 の編集上の訂正）。** 設計書 3.5.2 は、transcript を読めない pane への中継で `transcriptFailure` の理由（`no_transcript` か `unsupported_path`）を示すと決めている。一方、設計書 3.3 の `ClaudeWait` の抜き書きには、それを運ぶ欄がなかった。本計画の PR で設計書の 2 か所に欄を足した。決定は変わらない。この訂正は、本計画の PR の commit のメッセージと PR の説明に、編集上の訂正として書く（requirements_process.md の「Editing an approved document」）。ADR: なし。
9.  **既存のテストの、Claude Code の画面の振る舞いを確かめていた行と subtest を消すことも、テストの削除として扱う（CLAUDE.md の「Testing Strategy」の「Deleting a test is a claim that must be checked」）。** PR-4 は多くのテストを、名前を保ったまま中身を直す（§1.4 の「直す既存のテスト」）。その中で、Claude Code の画面から作る照合値や画面の質問を確かめていた行・subtest・ループの回を、消すか別の入力に置き換える。それらの削除は、書き直しの前の単独のコミットにし、`make coverage-compare BASE=<親のコミット>` の結果を commit のメッセージと PR の説明に書く。ADR: なし。

### 1.4. 既存コード調査結果

`origin/main` のコミット `62d867cc` で確かめた。設計書が計画に委ねた点（定数の名前、時間の測り方）は §1.3 で決めた。`findRelayTarget` の戻り値の型は、実装で決める。複数の実装の案が残る箇所はない。

| 領域 | あるもの | 足りないもの・変えること |
|---|---|---|
| transcript の場所と読み込み | `transcriptPath`（`internal/chief/transcript.go:54`）、`transcriptFailure`（`:76`）、`readTranscript`（`:85`）、`openTranscript`（`:102`）、`errTranscript`（`:18`）、上限 `sessionTranscriptLimit`（`:31`）。上限を超えたときの理由は文字列（`:96`）。ファイルがないときは `fs.ErrNotExist` を `%w` で含む（`:107`。`internal/chief/send_test.go:825` が確かめる）。0095 の `readClaudeReport`（`:142`）とそのテスト（`internal/chief/transcript_test.go`、`//go:build test && unix`） | 共有の読み込みの関数（決定 7）、`errTranscriptLimit`（設計書 4.1）、`readClaudeWait`（3.3）。PR-2 |
| transcript の行の読み取り | `readModelSwitch`・`readLastHumanInput`（`internal/chief/screen_transcript.go`）、0095 の報告の読み取り（`internal/chief/screen_transcript_report.go`）。どれも本計画では変えない（要件書の「要件定義の際に決めたこと」、設計書 9 章） | 新しいファイル `screen_transcript_calls.go` の `readOpenCalls`（設計書 3.2）。PR-1 |
| 問いの型 | `QuestionKind`・`RelayBlock`（`internal/chief/screen.go:26-57`）、`Question`（`:71`）、`finishQuestion`（`:166`）、`mergeQuestionText`（`:113`）。出力の名前の `questionKindName`（`internal/chief/output.go:110`）は、知らない値を `default` で `unknown` にする | 3.4.1 の欄と値（PR-1）。`several_questions` の名前（PR-4） |
| 画面の読み取り | `parseClaudeScreen`（`internal/chief/screen_claude.go:69`）、自由に書く答えとチャットの名前（`:15-18`・`:29`）、画面のサンプルの期待 `claudeScreenWants`（`internal/chief/screen_test.go`）、サンプルの一覧 `screenRows`・`TestScreenSamplesCoverEveryRow`（`internal/chief/samples_test.go:100`・`:132`）、置き換えの確かめ `TestSamplesFollowReplacementRules`（`:72`。`testdata` の下のすべてのファイルを読む） | `claudeScreenAgrees`（3.5.3）。PR-2。今の読み取りは変えず、#908 に残す（設計書 1.2） |
| 照合値 | `checkValue`（`internal/chief/relay.go:35`）、`relayable`（`:61`）、`relayCheck(a Agent, q Question)`（`:74`） | 並べてハッシュにする部分を共有し、`transcriptCheckValue` を足す（PR-2）。`relayCheck` の引数を `QuestionItem` にする（3.5.1。PR-4） |
| 中継 | `paneQuestion`（`internal/chief/relay.go:125`）、`findRelayTarget` の戻り値 `(relayTarget, found, readFailed bool)`（`:149`）、`Relay`（`:177`）、`relay` の確かめ（`:210-247`） | Claude Code の pane の探し方と理由（3.5.2）、送る直前の 3 つの確かめ（3.5.3）。PR-4 |
| 送らない理由 | `NotSentReason` の並びは `NotSentConditions` で終わる（`internal/chief/send.go:85-148`） | `NotSentScreenMismatch`（`screen_mismatch`）。PR-4 |
| 記録 | `SendRecord`（`internal/chief/records.go:662`）、`Store.read` は知らない欄を拒む（`:166`）、`Protocol = 1`（`:28`） | 中継の記録にだけ書く呼び出しの ID の欄（PR-4）、`Protocol`（決定 6。PR-4） |
| 表 | `agentFacts`（`internal/chief/state.go:149`）、`QuestionItem`（`:103`）、`classify`（`:382`。画面を読めない pane と知らない形の画面を「要確認」にする：`:390-410`） | `agentFacts.Wait`（PR-2 で欄を足し、PR-3 で埋める）。`QuestionItem.CallID`・`Working` と、`classify` の Claude Code の pane の分け方（3.4.2。PR-4） |
| 状況の読み取り | `readWorkspace(ctx, herdr, git, tasks, w)`（`internal/chief/snapshot.go:230`）。呼び出し元は `snapshot.go:132`・`request.go:221`・`autosend.go:660` の 3 か所 | `Env` を引数に足し、Claude Code の pane ごとに `readClaudeWait` を呼ぶ。PR-3 |
| 送る前の確かめ | `offered`（`internal/chief/request.go:314`）、`quietness`（`internal/chief/autosend.go:376`。`blocked` と画面の質問の分かれは `:384`、入力欄でない画面の分かれは `:386`）、`checkPane`（`internal/chief/send.go:331-342`） | `questionStop`（PR-2）と、その呼び出し（PR-3）。`checkPane` は変えない |
| 出力と項目 | `QuestionOutput`（`internal/chief/output.go:59`）、`questionOutput`（`:147`）、`questionID(workspace, q Question)`（`internal/chief/pending.go:84`） | 3.4.3 の欄、`questionID` の引数を `QuestionItem` にする。PR-4 |
| テストの世界 | 状況の表の世界 `world`（`internal/chief/world_test.go:79`）は、エージェントにセッション ID を持たせ（`:159`）、`Env.Home` を一時的なディレクトリにする（`:127`）。transcript のファイルは置かない。送信の世界は transcript を書く `writeTranscript`（`internal/chief/sim_test.go:112`）を持つ。問いを出す `ask`（`:72`）と照合値を求める `checkOf`（`internal/chief/relay_test.go:17`）は画面だけを使う | 両方の世界に、Claude Code の pane の transcript に答えを待っている呼び出しを書く口（PR-3）と、transcript から照合値を求める口（PR-4） |
| 文書のガード | `TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestGuideValuesMatchTool`（`cmd/chief/docs_test.go:205`・`:257`・`:405`）。`--protocol 1` は `.claude/commands/chief.md` の 97・106・228・232・238・248・250・254・307 行、手引きのプロトコル番号は `docs/dev/guides/chief_guide.md:28` | テストは変えない。番号を新しい値にする。PR-4 |

-   **改名の一覧。** 本計画は名前を変えない（決定 4）。
-   **変える関数の引数。** `readWorkspace` に `Env` を足す（呼び出し元の 3 か所。PR-3）。`relayCheck` と `questionID` の 2 つめの引数を `Question` から `QuestionItem` にする（PR-4）。どれも `internal/chief` の中だけで使われる（`62d867cc` で `rg -n 'readWorkspace\(|relayCheck\(|questionID\(' --glob '!docs/**' .` を走らせ、結果は `internal/chief` の `autosend.go`・`pending.go`・`pending_test.go`・`relay.go`・`request.go`・`snapshot.go`・`state.go` だけだった）。
-   **書き換える箇所の一覧（PR-4）。** `--protocol 1` を書き換える箇所は、上の表の「文書のガード」の行がすべてである（`62d867cc` で `rg -n -- '--protocol [0-9]' .claude/commands docs/dev/guides/chief_guide.md CLAUDE.md` を走らせ、`chief.md` の結果は表の 9 行だった。ほかは別の道具 `fixprguard` の `.claude/commands/fixpr.md`・`runpr.md` と `codexreview` の `.claude/commands/_lib/local-cli-review.md` の行で、変えない）。変え残しは `TestDocumentedProtocolMatches` が見つける。
-   **直す既存のテスト（PR-4）。** 設計書 3.9 の表のテストに加えて、次のテストの Claude Code の場合が変わる。テストの世界の既定のエージェントは Claude Code である（`internal/chief/world_test.go:155-157`）。PR-4 の `/runplan` は、`Kind` を書かない `fakeWorkspace`、`Status: "blocked"`、`questionPane("claude"`、`checkOf` の使い方を探し直し、この一覧に足りないものを足す。
    -   `TestTakeSnapshotStates` の `idle on a screen of no known shape`（`internal/chief/snapshot_test.go:71`）。今は「要確認」を期待するが、AC-26 で行は「要確認」にならない。同じテストの `blocked on an unknown shape`（`:76`）は、transcript のない `blocked` の「要確認」（AC-12）として通ることを確かめる。
    -   `TestTakeSnapshotUnreadable` の `screen fails`（`internal/chief/snapshot_test.go:144`）。画面を読めないことは `classify` から `questionStop` へ移る（設計書 3.6）。OpenCode の pane の場合は、`classify` の「要確認」として残す。
    -   `TestSendChecksBeforeFirstInput` の `a screen of no known shape`（`internal/chief/send_test.go:159`）。止める確かめが `offered` から `checkPane` に移る。
    -   `TestRelayRefusesOption` の Claude Code の行（`internal/chief/relay_test.go:371`）。transcript の問いの `n+1`・`n+2`・`n+3` の番号の行にする。
    -   `TestRelayUnreadablePane`（`internal/chief/relay_test.go:480`）。Claude Code の pane の画面を読めないことは `screen_mismatch` になる（設計書 3.5.3）。Herdr の `agent get` を読めないことは `unreadable` のまま（3.5.2）なので、その入力に替える。
    -   `TestRelayFailureAfterKeyStops`（`internal/chief/relay_test.go:529`）。照合値を transcript から作る。
    -   どのテストも、確かめていた守りは保ち、確かめる入力だけを替える。消す行と subtest は決定 9 のとおりに扱う。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: サンプル、transcript の読み取り（設計書 8 章の 1、2 のうち読み取り）

**変更するファイル:** `internal/chief/testdata/transcript/`・`testdata/screens/`（サンプルを足す）、`internal/chief/testdata/README.md`、`internal/chief/screen_transcript_calls.go`・`screen_transcript_calls_test.go`（新規）、`internal/chief/screen.go`、`internal/chief/screen_test.go`・`samples_test.go`、`internal/chief/doc.go`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`、本計画（§7 の記録）

-   [ ] **ステップ 1-1**: 採取の準備をする。Claude Code と Herdr の版を記録する。人に、採取用の pane で作る状態の順と、それぞれの状態を作るために打つ合成の依頼文を示す。依頼文は、問い・選択肢・道具の入力に作業の中身が入らないものにする（権限の確認なら、害のない `echo` など）。そうすれば、問いや選択肢の文を後から置き換えずに済み、transcript と画面の組を、採ったままのバイト列で保てる。セッションは読み取りだけを行う（`herdr agent get`・`herdr agent read`、transcript のファイルの写し。`internal/chief/testdata/README.md` の「採り方」）。
-   [ ] **ステップ 1-2**: 設計書 7.1 の 6 つの項目と、選択肢の名前が `Type something.` の 1 つを選ぶ問いの transcript と画面の組（設計書 7.5）を採る。権限の確認の間の `herdr agent get` の状態を最初に確かめ、`blocked` でなければ、決定 5 のとおり止まる。親の transcript にない問いの部品（設計書 3.5.3）、打った文字の画面、`/compact`・`--resume`・`/rewind` の後の transcript は、採れたものと採れなかったものを分けて記録する。
-   [ ] **ステップ 1-3**: 採ったものを、`testdata/README.md` の「置き換えたもの」と、0095 が transcript について決めた削り方（同じ README の 0095 の節）に従って直す（security.md §7）。
    -   transcript の行は、要件書 4.2 の「使う部分」と行の種類を表すメンバーに絞る。
    -   `api-request-blob` の行は、`type` と、同じ呼び出しの ID を持つメンバーだけに絞る（H-06）。
    -   呼び出しの ID（`toolu_` で始まる値）は、ファイルごとに通し番号に置き換え、同じ値は同じ値にする。この規則を README に書き、機械で確かめられる形なら `samples_test.go` の置き換えの規則に足す。
    -   版と採った日と絞り方を README に書く。
-   [ ] **ステップ 1-4**: 足した画面のサンプルを `claudeScreenWants` と `screenRows` に足し、今の `parseClaudeScreen` の読み取りの期待を書く。中継の前の画面の確かめのためのサンプルは、`screenRows` に行を足して入れ、その行を README の表にも足す。
-   [ ] **ステップ 1-5**: `screen.go` の `QuestionKind`・`RelayBlock` に `QuestionSeveral`・`RelayBlockSeveral`・`RelayBlockTooMany` を、`Question` に `Header`・`Parts`・`Tool`・`Input`・`InputCut` を足し、`Question` のコメントを直す（設計書 3.4.1）。
-   [ ] **ステップ 1-6**: `screen_transcript_calls.go` に `CallKind`・`OpenCall`・`readOpenCalls`・`errTranscriptForm` と、質問の呼び出しから `Question` を作る部分を書く（設計書 3.2.1・3.2.2）。合わなかった行の番号は、`errors.AsType` で取り出せる型の誤りに持たせる。テストは `screen_transcript_calls_test.go` に書き、次を確かめる。
    -   ステップ 1-2 の各サンプルの、答えを待っている呼び出しの一覧と種類。答えのある呼び出し（`is_error` の答えを含む）、`api-request-blob` の行の同じ ID、対応する呼び出しのない答え（`TestReadOpenCallsSamples`）。
    -   要件書 4.2 の拒む入力の行を 1 つずつ。誤りが `errTranscript` と `errTranscriptForm` を `errors.Is` で満たし、行の番号を型で持ち、誤りの文に行の中身を含まないこと（`TestReadOpenCallsRejects`）。
    -   改行で終わっていない最後の行を読まないこと、空白だけの行（`TestReadOpenCallsPartialLastLine`）。
    -   設計書 3.2.2 の表の各行（選択肢が 1・7・8・9・10 の境界を含む）、`n+1`・`n+2` の番号の選択肢、`Parts` の各問いの番号、マージの確認の見分け（`RelayAllowed` の問いだけ）（`TestQuestionFromCall`）。マージの確認の問いの transcript は、サンプルのファイルに置く。
-   [ ] **ステップ 1-7**: CLAUDE.md の「Declare, don't infer」の chief の例外の範囲に、Claude Code の transcript の答えを待っている呼び出しを読むことを足す。読むのは、呼び出しの種類、問いの形・文・選択肢、マージの確認である。「Go Idioms」の chief の例外に、transcript の問いの文と同じでなければならない文字列を足す（設計書 3.8、決定 1）。`internal/chief/doc.go` の同じ説明も直す。`package_reference.md` の `internal/chief` の行の、画面の読み取りが返す型に `OpenCall` を足す。
-   [ ] **ステップ 1-8**: §7 の「PR-1 の実行の記録」に、版、ステップ 1-2 の各項目の結果、決定 5 で選んだ扱いを書く。

**完了の条件:** ステップ 1-6 のテストが通り、§7 に PR-1 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: samples and the transcript reader of open calls

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8

**推奨タイトル**: `feat(0096): 答えを待っている間のサンプルを採り、transcript の答えを待っている呼び出しの読み取りを作る`

**レビュー観点**: 権限の確認の間の Herdr の状態など、サンプルの結果と決定 5 の扱い / サンプルに採取した人を特定できる情報や作業の中身が残っていないか（`api-request-blob` の行の削り方を含む） / 要件書 4.2 の拒む入力をすべて拒み、広げてよい部分のほかを受け付けていないか / CLAUDE.md の例外の範囲の書き足し

**実装モデル要件**: opus

**判定理由**: ステップ 1-2 は、確かめていない外部の振る舞い（権限の確認の間の Herdr の状態、答えを待っている間の transcript、打った文字の画面）を本物の Claude Code で確かめる Phase 0 型の確かめである。Sonnet は、想定と違う結果を確認済みの事実として記録し、要件書を戻すべき食い違いを後の PR へ持ち越しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 判定と確かめの部品（設計書 8 章の 2 の残り、3・4 のうち配線しない部品）

**変更するファイル:** `internal/chief/transcript.go`・`transcript_test.go`、`internal/chief/claudewait.go`・`claudewait_test.go`（新規）、`internal/chief/screen_claude.go`・`screen_test.go`、`internal/chief/relay.go`・`relay_test.go`、`internal/chief/state.go`、`docs/dev/conventions/package_reference.md`、本計画（§7 の記録）

-   [ ] **ステップ 2-1**: `transcript.go` で、`readClaudeReport` の場所を決めて読む手順を共有の関数に取り出す（決定 7）。`readTranscript` が上限を超えたときに `errTranscriptLimit` を `errTranscript` に含めて返すようにする（設計書 3.2.3・4.1）。`TestReadClaudeReport` とリセットのテストは変えずに通す。`transcript_test.go` に、上限を超えたときの誤りを `errors.Is(err, errTranscriptLimit)` で見分けるテストを足す。
-   [ ] **ステップ 2-2**: `claudewait.go` に `ClaudeWaitKind`・`ClaudeWait`（`Reason` を含む。決定 8）・`judgeClaudeWait`・`permissionQuestion`・`permissionInputLimit` を書き、`transcript.go` に `readClaudeWait` を書く（設計書 3.3）。`Why` は設計書 4.2 の文の形にする。テストは次を確かめる。
    -   `claudewait_test.go` の `TestJudgeClaudeWait` で、設計書 3.3 の表の各行を、その行より前の行に当たらない入力で確かめる。状態のゼロ値と、表にない状態の値も含める。
    -   `transcript_test.go` の `TestReadClaudeWait` で、`readClaudeWait` の各場合を確かめる。場所が決まらない 3 つ（セッションがない、作業ディレクトリの文字、設定のディレクトリ）は、`Kind` と `Reason` で確かめる。ファイルがない場合は `working`・`idle` で「なし」、`blocked` で「要確認」になる。そのほかに、通常のファイルでない場合と、形の合わない行の場合を確かめる。上限については、先頭に答えを待っている質問の呼び出しがあり、`blocked` のエージェントで上限を 1 バイト超える transcript が「要確認」になることを確かめる（一部から判定しない。AC-22）。上限ちょうどの transcript は読めることも確かめる。大きなファイルは `Truncate` などで作る。
    -   `Why` が pane の ID を含み、transcript の問いの文・選択肢・道具の入力を含まないこと。ひとりでは消えない「要確認」で `/clear` を添えること。
    -   `claudewait_test.go` の `TestPermissionQuestion` で、`permissionQuestion` の切り方を、2,000 文字と 2,001 文字の境界で確かめる。複数バイトの文字には、日本語でない文字か `\u` のエスケープを使う（CLAUDE.md の「Go Idioms」）。
-   [ ] **ステップ 2-3**: `claudewait_test.go` に `BenchmarkReadClaudeWait` を書く（決定 2）。テストのバイナリを `go test -c -tags test` で作り、`/usr/bin/time -l` の下でそのバイナリのベンチマークだけを 1 回走らせる。時間、`-test.benchmem` の割り当て、最大使用量を、走らせたコマンド・機械・コミット・transcript の組み立てとともに §7 の「PR-2 の実行の記録」に書く。決定 7 の 2 回分の見積もりも書く。1 回の判定が 1 秒を超えれば、決定 2 のとおり止まり、後のステップに進まない。
-   [ ] **ステップ 2-4**: `screen_claude.go` に `claudeScreenAgrees` を書く（設計書 3.5.3）。§7 の「PR-1 の実行の記録」で選んだ扱い（名前と説明をつないで比べるか）を入れる。テスト `TestClaudeScreenAgrees`（`screen_test.go`）は、PR-1 の実物の画面と transcript の組だけで確かめる。対象は、設計書 7.2 の「中継の前の画面の確かめ」の各場合と、`Type something.` を名前に持つ選択肢の画面が合わないこと（設計書 7.5）である。`why` が画面の中身を含まないことも確かめる。
-   [ ] **ステップ 2-5**: `relay.go` で、`checkValue` の、部分を長さ付きで並べてハッシュにする部分を共有の関数にし、`transcriptCheckValue` と `transcriptCheckVersion` を足す（設計書 3.5.1）。今の `TestCheckValueChanges` は変えずに通す。テスト `TestTranscriptCheckValue`（`relay_test.go`）は、設計書 3.5.1 の部分のそれぞれを変えると値が変わること、選択の位置で値が変わらないこと、同じ中身の画面の照合値と等しくならないことを確かめる。
-   [ ] **ステップ 2-6**: `state.go` の `agentFacts` に `Wait` を足し、`claudewait.go` に `questionStop` を書く（設計書 3.6）。テスト `TestQuestionStop`（`claudewait_test.go`）は、AC-24 の (a)〜(d)、`blocked` でない pane の画面の質問、読めない画面で止めることを確かめる。知らない形の画面と、OpenCode の pane の `Wait` のゼロ値で止めないことも確かめる。
-   [ ] **ステップ 2-7**: `package_reference.md` の `internal/chief` の行の、表の分け方の名前の並び（`TakeSnapshot`・`Snapshot` など）に `ClaudeWait` を足す。画面の読み取りが返す型の並びには入れない（設計書 1.1 の 3 つの層）。

**完了の条件:** 各ステップのテストが通り、§7 に PR-2 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。新しい部品はテストからだけ呼ばれ、表と送信の振る舞いは変わらない。

### PR-2 作成ポイント: wait judgement, screen agreement, check value, and question stop

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0096): 待ち状態の判定、中継の前の画面の確かめ、transcript の照合値、質問による止めの部品を作る`

**レビュー観点**: 待ち状態の表が設計書 3.3 のとおりで、表にない状態を「要確認」に倒しているか / `claudeScreenAgrees` が実物のサンプルで、合わない画面をすべて合わないとしているか / 照合値が設計書 3.5.1 の部分をすべて含むか / 共有した読み込みで 0095 の報告の読み取りが変わらないか / `Why` と `why` が transcript と画面の中身を含まないか / ベンチマークの結果が 1 秒の予算に収まるか

**実装モデル要件**: opus

**判定理由**: ステップ 2-4・2-6 は、人が選んでいない入力をキーで送らないための守り（中継の前の画面の確かめと質問による止め）を作るもので、mkplan.md の panel-mode の条件の security-gate に当たる。Sonnet は、合わない画面や表にない状態を合うもの・止めないものとして通し、送る側に倒しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 質問による止めの配線と 0092 の削除の扱い（設計書 8 章の 3 のうち送る前の確かめ）

**変更するファイル:** `internal/chief/snapshot.go`・`request.go`・`autosend.go` とそのテスト（`send_test.go`・`autosend_test.go`）、`internal/chief/world_test.go`・`sim_test.go`、0092 の削除の扱い（ステップ 3-4）で直すファイル、本計画（§7 の記録）

-   [ ] **ステップ 3-1**: 状況の表の世界と送信の世界に、Claude Code の pane の transcript に答えを待っている呼び出しを書く口を足す（§1.4 の「テストの世界」）。transcript の行は PR-1 のサンプルの形で作る。0095 の transcript の行の補助があれば使う。
-   [ ] **ステップ 3-2**: `readWorkspace` に `Env` を足し（呼び出し元の 3 か所）、Claude Code の pane ごとに `readClaudeWait` を呼んで `agentFacts.Wait` に入れる（設計書 3.4.2）。`classify` はまだ変えない。今の状況の表のテストは変えずに通す。
-   [ ] **ステップ 3-3**: `Send` が行を決めた後に `questionStop` を確かめ、`offered` が送らないときに「要確認」の理由を添えるようにする。`quietness` が最初に `questionStop` を呼ぶようにする（設計書 3.6）。テストは、画面が入力欄の形で、transcript だけが止める理由を持つ入力で、`questionStop` だけが止めることを確かめる。この PR では `classify` が画面で行を決めるので、`offered` はこれらの入力を通す。
    -   コマンドの送信（`TestSendStopsOnQuestion`）: 送り先と別の Claude Code の pane が、`idle` で画面が入力欄の形のまま、AC-24 の (a)・(c)・(d) に当たる入力（transcript に答えを待っている質問、ほかの「要確認」、transcript を読めない）。行が次のコマンドを提案していて `offered` が通ることを先に確かめ、示す理由も確かめる。
    -   自動送信（`TestAutoSendStopsOnQuestion`）: `idle`・`done` で画面が入力欄の形の pane で、AC-24 の (a)・(c)・(d)。
    -   AC-24 の (b)（`blocked`）と、画面が質問を示すとき・読めないとき（AC-25）は、今の `classify`・`quietness` でも止まるので、同じテストに、振る舞いを保つ確かめとして置く。
    -   `offered` に添える「要確認」の理由を確かめる。
    -   送り先の pane そのものの transcript を読めないとき（場所が決まらないときを含む）、`Send` は今と同じく待ち状態の `Reason`（`no_transcript` か `unsupported_path`）と `Why` で送らない。そのほかの止めは、行が提案していないときと同じ `not_offered` の理由で送らない。今の `TestActionResults` の `not sent`（`internal/chief/send_test.go:695`。セッションのない送り先で `no_transcript` を期待する）は、変えずに通す。
    -   `sim_test.go` が書く今の transcript の行（`mode` の行、モデルの切り替え、コマンド、報告など）が、答えを待っている呼び出しのない transcript として読めることを確かめる。読めなければ、今の送信と自動送信のテストが止めに当たる。
    -   `Send` と `quietness` の `questionStop` の呼び出しをそれぞれ外したとき、どのテストが落ちるかを commit のメッセージに書く。
-   [ ] **ステップ 3-4**: 0092 の workspace の削除のコードがあるかどうかに応じた扱いを、設計書 3.6 の手順で決める。PR を作る前とマージの直前の 2 回、`origin/main` を確かめる。
    -   削除のコードがあれば、`agent_active` の確かめに `questionStop` を足し、AC-24 の (a)〜(d) で消さないことのテストを足す。
    -   なければ、0092 の設計書の `agent_active` の行と、それを実装する計画書のステップに、`questionStop` を呼ぶことと、AC-24 の (a)〜(d) で消さないことを確かめるテストの名前を書き足す。直すのは、そのステップと AC の表の行だけにする。これは 0092 の 2 つの文書の決定の変更なので、どちらも `draft` に戻し（requirements_process.md の「Editing an approved document」）、この PR の `/runpr` のマージの問いで承認を受ける。その間も、0092 の `/runplan` は `origin/main` の承認済みの文書を読むので止まらない。
    -   マージの直前の確かめは、PR の説明に、人が行う項目として書く。その間に削除のコードが入っていれば、「削除のコードがある」場合の扱いに直してからマージする。この直しはコードとテストを PR の作成の後に足すので、PR レビューをもう一度頼む変更として扱う（`.claude/commands/_lib/pr-review-escalation.md`）。
    -   どちらにしたかと、確かめた `origin/main` の 2 つのコミットを、§7 の「PR-3 の実行の記録」に書く。

**完了の条件:** 各ステップのテストが通り、§7 に PR-3 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。出力と `Protocol` は変わらない。

### PR-3 作成ポイント: wire the question stop into sends and automatic sends

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4

**推奨タイトル**: `feat(0096): コマンドの送信と自動送信に transcript による質問の止めを足す`

**レビュー観点**: `questionStop` が `Send` では行を決めた後、`quietness` では最初に呼ばれるか / `questionStop` だけが止める入力でテストし、呼び出しを外すとどのテストが落ちるかを確かめたか / 今の送信と自動送信のテストが変わらずに通るか / 0092 の削除の扱いと、マージの直前の確かめの項目

**実装モデル要件**: opus

**判定理由**: ステップ 3-3 は、送信と自動送信の送る前の確かめという安全の境界を直すもので、mkplan.md の panel-mode の条件の security-gate に当たる。Sonnet は、`questionStop` をほかの確かめが先に止める位置に置くか、ほかの層でも止まる入力だけでテストし、配線が外れても気づかない形にしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 表・出力・中継の切り替え、`Protocol`、`/chief` の定義、実地の確かめ（設計書 8 章の 3・4・6、5 のうち定義）

**変更するファイル:** `internal/chief/state.go`・`output.go`・`pending.go`・`relay.go`・`send.go`・`records.go` とそのテスト（`snapshot_test.go`・`pending_test.go`・`relay_test.go`・`send_test.go`・`autosend_test.go`・`wait_test.go`・`records_test.go`）、`internal/chief/sim_test.go`（照合値を求める口）、`.claude/commands/chief.md`、`docs/dev/guides/chief_guide.md`（プロトコル番号の行だけ）、本計画（§7 の記録）

-   [ ] **ステップ 4-1**: `QuestionItem` に `CallID`・`Working` を足し、`classify` を設計書 3.4.2 の表のとおりにし、`relayCheck` の引数を `QuestionItem` にする（設計書 3.5.1）。設計書 3.9 の表と §1.4 の「直す既存のテスト」の状況の表のテストを、決定 9 のとおり直す。
    -   新しいテスト `TestClaudeWaitRows`（`snapshot_test.go`）で、AC-01〜AC-13・AC-19〜AC-23・AC-26 の各状態の行と項目を確かめる。
    -   画面が質問の形で transcript に呼び出しがない入力と、画面が入力欄の形で transcript に質問の呼び出しがある入力を置き、行が transcript だけで決まることを確かめる。画面が知らない形でも、Claude Code の pane の行が「要確認」にならないことも確かめる。
-   [ ] **ステップ 4-2**: 出力を設計書 3.4.3 のとおりにし（`QuestionPartOutput`、足す欄、`several_questions`）、`questionID` を Claude Code の項目について呼び出しの ID から作る。`TestPendingQuestionIdentity` を直す。新しいテスト `TestClaudeQuestionOutput`（`snapshot_test.go`）で、状況の表の行と人の判断を待つものの一覧の出力を確かめる。種類ごとの使う欄と空の欄、`working`、制御文字の置き換え、空の配列が `[]` であることを確かめる。
-   [ ] **ステップ 4-3**: 切り替えで `classify` が Claude Code の pane の画面を見なくなるので、ステップ 3-3 で振る舞いを保つ確かめとして置いた AC-25 の入力を直す。この入力は、送り先と別の Claude Code の pane が `idle` か `done` で待ち状態が「なし」であり、その画面が質問を示すとき、または読めないときである。コマンドの送信では、行が次のコマンドを提案していて `offered` が通ることを先に確かめ、`questionStop` だけが止めることを確かめる形にする。自動送信では、今の `quietness` の分かれ（`internal/chief/autosend.go:384-386`）でも止まるので、振る舞いを保つ確かめとする。`TestSendChecksBeforeFirstInput` を §1.4 のとおり直す。
-   [ ] **ステップ 4-4**: 中継を設計書 3.5.2・3.5.3 のとおりにする。`NotSentScreenMismatch` と、`SendRecord` の呼び出しの ID の欄を足す。この欄は空のときに書かず（`omitempty`）、中継の記録にだけ書く（決定 3）。§7 の「PR-1 の実行の記録」で、親の transcript にない問いの部品が重なりうると分かっていれば、設計書 3.5.3 の手順 3 の後に画面をもう一度読む確かめを足す（決定 5）。送信の世界の照合値を求める口を、Claude Code の pane では transcript から求めるようにする。設計書 3.9 の表と §1.4 の「直す既存のテスト」の中継のテストの Claude Code の場合を直し、OpenCode の場合は変えずに通す。新しいテスト（`relay_test.go`）は次を確かめる。
    -   transcript の問いの選択肢の番号のキーだけを送ること（`TestRelayClaudeSendsChosenOption`）。
    -   送る直前の 3 つの確かめのそれぞれについて、その情報源だけが変わる入力で送らないこと（`TestRelayClaudeRechecksBeforeTheKey`）。transcript だけに答えが書かれる場合、Herdr の状態だけが変わる場合、画面だけが違う場合（自由に書く答えに移る、画面を読めないを含む）である。ほかの 2 つの確かめがその入力で通ることを先に確かめる。
    -   画面が合わないとき、送らず、その pane の行が「要確認」にならないこと（`TestRelayClaudeScreenMismatch`）。
    -   設計書 3.5.2 の理由の順を、それぞれの理由に当たる pane が 1 つずつある入力で確かめること（`TestRelayClaudeReasons`）。
    -   中継の記録に呼び出しの ID があり、コマンドの送信の記録にその欄がないこと、この欄のない今の記録を読めること（`records_test.go`）。
-   [ ] **ステップ 4-5**: `Protocol` を決定 6 の値にする。`.claude/commands/chief.md` を設計書 3.4.3 のとおりに直し、すべての `--protocol` を同じ値にする。手引きのプロトコル番号の行を同じ値にする。PR の説明に、マージの直前に人が `origin/main` の `Protocol` を確かめる項目を書く（決定 6）。
-   [ ] **ステップ 4-6**: 実地の確かめをする（設計書 7.4、決定 3）。人が chief の worktree をこのブランチにして入れ直し、採取用の workspace で `/chief` を使って、次のそれぞれの期待どおりになることを確かめる。結果を §7 の「PR-4 の実行の記録」に書く。
    -   1 つを選ぶ問いを示し、中継すると、キーが押されて問いから離れる（`sent`）。
    -   中継の前に、人が pane で選択を自由に書く答えに移すと、中継は `screen_mismatch` で送らず、行は「要確認」にならない。
    -   複数の問い・権限の確認・サブエージェントの待ちは、中継の選択肢なしで示される。

**完了の条件:** 各ステップのテストが通り、§7 に PR-4 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る（`TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool` を含む）。

### PR-4 作成ポイント: switch Claude Code questions and relays to the transcript

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6

**推奨タイトル**: `feat(0096): Claude Code の質問の判定と中継の照合を transcript に切り替え、/chief の定義と Protocol を直す`

**レビュー観点**: `classify` が Claude Code の pane の画面を見ず、待ち状態だけで分けるか / 中継の送る直前の 3 つの確かめの順と、どれか 1 つだけが合わないときに何も送らないか / 既存のテストの直しで守りを保ち、消した行と subtest のカバレッジを比べたか / 呼び出しの ID の欄が中継の記録にだけ書かれるか / `Protocol` と `/chief` の定義と手引きの番号、実地の確かめの結果

**実装モデル要件**: opus

**判定理由**: ステップ 4-1・4-2・4-4 は、0088 の表・出力・中継の振る舞いを変え、多くの既存のテストを直す移行で、`Protocol` を上げる（mkplan.md の panel-mode の条件の migration）。Sonnet は、テストの期待を新しい振る舞いに合わせるだけで直し、OpenCode の pane や送る前の確かめのうち変えない振る舞いを壊しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 文書（設計書 8 章の 5 の残り）

**変更するファイル:** `CLAUDE.md`、`docs/dev/guides/chief_guide.md`、`docs/dev/conventions/package_reference.md`（変え残しがあるときだけ）

-   [ ] **ステップ 5-1**: CLAUDE.md の「Declare, don't infer」の chief の例外を、設計書 3.8 の振る舞いに合わせる（AC-27）。Claude Code の pane の質問を transcript から読むこと、画面からの判断は送るのを止めるためにだけ使うこと、中継の前に画面の 1 つを選ぶ問いが transcript の問いと同じかを確かめることである。
-   [ ] **ステップ 5-2**: chief 利用ガイドを設計書 3.8 のとおりに直す（AC-28）。対象は、1 章の表、5.1 の中継の説明、8 章の 4 つの項目（中継しない質問、transcript の場所の規則、ひとりでは消えない「要確認」、`screen_mismatch` が続くとき）である。§7 の「PR-1 の実行の記録」の `/compact`・`--resume`・`/rewind` の結果のうち、「要確認」がひとりでは消えなくなる操作も 8 章に書く。
-   [ ] **ステップ 5-3**: `package_reference.md` の `internal/chief` の行が、PR-1・PR-2 で足した型（画面の読み取りの型の並びの `OpenCall`、表の分け方の並びの `ClaudeWait`）を挙げていることを確かめ、足りなければ直す。

**完了の条件:** `make test`・`make lint` が通る。

### PR-5 作成ポイント: CLAUDE.md exception and chief guide

**対象ステップ**: 5-1 / 5-2 / 5-3

**推奨タイトル**: `docs(0096): CLAUDE.md の chief の例外と chief 利用ガイドを transcript による質問の判定に合わせる`

**レビュー観点**: CLAUDE.md の例外の範囲と境界が、PR-4 の振る舞い（画面は送るのを止めるためだけ）と合うか / 利用ガイドの中継の照合と中継しない質問の説明が F-001〜F-003 と合うか / ひとりでは消えない「要確認」と `screen_mismatch` が続くときに人ができること

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
| M2（PR-2） | 共有の読み込み、`readClaudeWait`・`judgeClaudeWait`、ベンチマークと測定の記録、`claudeScreenAgrees`、`transcriptCheckValue`、`questionStop` | 部品がそろう（まだ配線されない） |
| M3（PR-3） | `readWorkspace` の待ち状態、`Send` と `quietness` の質問による止め、0092 の削除の扱い | transcript で質問を待っている pane のある workspace へ、送信と自動送信をしない。表と中継はまだ画面で決まる |
| M4（PR-4） | 切り替え、出力、`Protocol`、`/chief` の定義、実地の確かめ | chief が Claude Code の質問を transcript で示し、中継する |
| M5（PR-5） | CLAUDE.md の例外、chief 利用ガイド | 文書が振る舞いと合う |

順序は設計書 8 章のとおりで、8 章から動かしたものは §1.3 の決定 1 にすべて挙げた。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 | サンプルの採取、`readOpenCalls`、`Question` の欄と値、CLAUDE.md の例外の範囲 | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | 共有の読み込み、待ち状態の判定、ベンチマーク、中継の前の画面の確かめ、transcript の照合値、質問による止め | opus | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 | テストの世界の transcript、`readWorkspace` の待ち状態、送信と自動送信の質問による止め、0092 の削除の扱い | opus | auto |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 | 表・出力・中継の切り替え、`Protocol`、`/chief` の定義、実地の確かめ | opus | auto |
| PR-5 | 5-1 / 5-2 / 5-3 | CLAUDE.md の例外、chief 利用ガイド | sonnet | auto |

どの PR も、前の PR のマージから次の `/runplan` までの間に、`/runplan` の外の作業を挟まない。PR-1 のサンプルの採取と PR-4 の実地の確かめは、`/runplan` のセッションの中で人と行う。PR-4 のマージの後に chief の worktree を `main` に戻す作業（決定 3）は、PR-5 の `/runplan` の前提ではない。そのため、自動送信はすべて `auto` とする。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 読み取り・判定・画面の確かめ・照合値・質問による止めは、PR-1・PR-2 で、ほかの部品に依らないユニットテストで確かめる。transcript は PR-1 のサンプルと、そこから 1 か所を変えた行で与え、一時的なディレクトリに置く。画面は実物のサンプルだけを使う（設計書 7.2）。
-   **層ごとの入力:** `judgeClaudeWait` の表の行は、その行より前の行に当たらない入力で確かめる。質問による止めは、`questionStop` のほかの確かめが止めない入力で確かめる。PR-3 では画面が入力欄の形で transcript だけが理由を持つ入力（ステップ 3-3）、PR-4 では待ち状態が「なし」で画面だけが理由を持つ入力（ステップ 4-3）を使う。PR-4 の表のテストは、画面と transcript の片方だけが質問を示す入力で、行が transcript で決まることを確かめる。中継の送る直前の確かめは、1 つの情報源だけが変わる入力で確かめる（ステップ 4-4）。
-   **結合の確かめ:** 状況の表・送信・自動送信・中継は、Herdr と git を模したテストの世界に transcript を置いて確かめる（設計書 3.9、ステップ 3-1）。
-   **誤り:** 誤りは `errors.Is` と `errors.AsType` で見分け、誤りの文を比べない（`errTranscript`・`errTranscriptForm`・`errTranscriptLimit`・`fs.ErrNotExist`、行の番号を持つ型）。`readClaudeWait` の段では、`Kind` と `Reason` で見分ける。
-   **境界の値:** 選択肢の数（1・7・8・9・10）、権限の確認の入力の 2,000 文字、読む量の 64 MiB を、境界の前後で確かめる。
-   **セキュリティ:** 設計書 7.5 の項目を、ステップ 1-6・2-2・2-4・4-2 のテストで確かめる（§7 の表の後の注）。
-   **性能:** `BenchmarkReadClaudeWait` を PR-2 で 1 回走らせて記録する（決定 2）。`make test` は時間を確かめない。
-   **既存のテストの直し:** 名前を保ったまま入力を替え、確かめていた守りを保つ。消す行・subtest・テスト関数は単独のコミットにし、カバレッジを比べる（決定 9）。
-   **テストの補助:** `internal/chief` の中だけで使う補助は、今の `test_helpers.go` か、テストの世界のファイル（`world_test.go`・`sim_test.go`）に置く（test_organization.md の Classification B）。ほかのパッケージから使う補助は作らない。テストの日本語の文字列は、CLAUDE.md の「Go Idioms」の例外の範囲（`screen` で始まるファイルのテスト）に限る。まとまった画面と transcript は、サンプルのファイルに置く。FIFO を使うテストは、今の `transcript_test.go` と同じく `unix` の build tag の下に置く。
-   **文書のガード:** `cmd/chief/docs_test.go` の今のテストが、`--protocol` の番号、呼び出しの形、手引きのプロトコル番号を確かめる。テストは変えない。CLAUDE.md・利用ガイド・`/chief` の定義の文は、PR のレビューで読む（設計書 7.3）。
-   **後方互換:** OpenCode の pane の表・送信・中継は、今のテストが変わらずに通ることで確かめる。0088 の `/chief` の定義との組み合わせは、`Protocol` で拒む（決定 6）。新しい版は、呼び出しの ID の欄のない今の送信の記録を読める（ステップ 4-4）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 権限の確認の間に Herdr が `blocked` と報告しない、など要件書の前提が成り立たない | PR-1 の最初（ステップ 1-2）で確かめ、決定 5 のとおり止まって要件書を `draft` に戻す |
| 自由に書く答えに打った文字が画面で見分けられず、数字のキーが文字として入る | PR-1 で確かめ、見分けられなければ決定 5 のとおり止まる（設計書 3.5.3） |
| 親の transcript にない問いの部品が重なり、中継のキーが別の部品に入る | PR-1 で確かめ、重なりうるなら PR-4 で設計書 3.5.3 の手順 3 の後に画面を読み直す（ステップ 4-4） |
| サンプルに採取した人を特定できる情報や作業の中身が残る | 合成の依頼文で採り（ステップ 1-1）、`api-request-blob` の行を削る（ステップ 1-3）。`TestSamplesFollowReplacementRules` と PR-1 のレビュー観点 |
| 64 MiB の transcript の判定が 1 秒を超える | PR-2 のステップ 2-3 で、ほかの部品の前に測り、超えれば決定 2 のとおり止まる |
| 切り替えの既存のテストの直しで、変えない振る舞いを壊すか、守りの確かめを失う | OpenCode の場合と送る前の確かめ（`checkPane`）のテストは変えずに通す。消す行と subtest のカバレッジを比べる（決定 9）。止めの配線は PR-3 で先に入れ、PR-4 の差分から外す |
| 実地の確かめの後に PR-4 をマージせず古い版に戻すと、新しい欄を持つ送信の記録を古い版が読めない | 呼び出しの ID の欄は中継の記録にだけ書き、中継は採取用の workspace だけで行う。戻すときはその workspace の送信の記録を消す（決定 3） |
| 0093 と 0095 の残りの PR が同じファイルを変える。0093 は `state.go`・`snapshot.go`・`output.go`・`records.go`・`transcript.go`・`relay_test.go`・`autosend_test.go`・`chief.md`・手引き、0095 は `autosend.go`・`request.go`・`records.go`・`output.go`・`sim_test.go`・`screen_test.go`・`samples_test.go`・`testdata/README.md` | マージの順は前提にしない。各 PR の `/runplan` は、始める前に §1.4 の調べをそのときの `origin/main` でやり直す（冒頭）。`main` はマージで取り込み、取り込んだ後に `make test` を通す。`Protocol` は決定 6 のとおり 2 回確かめる。上限の定数と読み込みは 0095 のものを使う（決定 4・7） |
| 0092 の削除の確かめが `questionStop` を知らずに入る | ステップ 3-4 で PR を作る前とマージの直前に `main` を確かめ、コードか 0092 の文書のどちらかに足す（設計書 3.6） |
| 日程: PR-1 のサンプルの採取と PR-4 の実地の確かめで、人の手が空かない | 人がいるときにだけ進める。`/runplan` は人を待つだけで、ほかのタスクを妨げない。PR-4 の実地の確かめの間だけ、ほかのタスクの待ち受けは PR-4 の版で動く（決定 3） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6）
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

表のパスの `ic/` は `internal/chief/` を表す。テストの名前とファイルは計画の時点のもので、各ステップはこの表の名前とファイルでテストを書く。実装で名前を変えたときは、この表を同じ PR で直す（編集上の訂正）。`manual` は、文書と `/chief` の定義の文を PR のレビューで読むことと、PR-4 の実地の確かめを指す。

| AC | 確かめ | 種類 | ステップ | PR |
|---|---|---|---|---|
| AC-01 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（番号、`n+1`・`n+2`）、`ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows`、`ic/snapshot_test.go::TestClaudeQuestionOutput`（表と一覧の出力、見出し・説明・制御文字）、定義の示し方 | test、manual | 1-6・2-2・4-1・4-2・4-5 | PR-1、PR-2、PR-4 |
| AC-02 | `ic/screen_transcript_calls_test.go::TestReadOpenCallsSamples`（`is_error` の答え）、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 1-6・4-1 | PR-1、PR-4 |
| AC-03 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（`QuestionSeveral`）、`ic/snapshot_test.go::TestClaudeQuestionOutput`（`parts`、中継しない）、定義の示し方 | test、manual | 1-6・4-2・4-5 | PR-1、PR-4 |
| AC-04 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（`QuestionMulti`）、`ic/snapshot_test.go::TestClaudeQuestionOutput` | test | 1-6・4-2 | PR-1、PR-4 |
| AC-05 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（選択肢 1・7・8・9・10）、`ic/snapshot_test.go::TestClaudeWaitRows`（中継しない問いに照合値がない） | test | 1-6・4-1 | PR-1、PR-4 |
| AC-06 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows`、`ic/snapshot_test.go::TestClaudeQuestionOutput`（`working`）、`ic/relay_test.go::TestRelayClaudeReasons`（`not_accepting`） | test | 2-2・4-1・4-2・4-4 | PR-2、PR-4 |
| AC-07 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・4-1 | PR-2、PR-4 |
| AC-08 | `ic/screen_transcript_calls_test.go::TestQuestionFromCall`（マージの確認）、`ic/snapshot_test.go::TestClaudeWaitRows`（`MergeQuestion`） | test | 1-6・4-1 | PR-1、PR-4 |
| AC-09 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・4-1 | PR-2、PR-4 |
| AC-10 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/claudewait_test.go::TestPermissionQuestion`（2,000 文字の境界）、`ic/snapshot_test.go::TestClaudeQuestionOutput`（`tool`・`input`・`input_cut`、制御文字）、定義の示し方 | test、manual | 2-2・4-2・4-5 | PR-2、PR-4 |
| AC-11 | `ic/screen_transcript_calls_test.go::TestReadOpenCallsSamples`（`CallSubagent`）、`ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 1-6・2-2・4-1 | PR-1、PR-2、PR-4 |
| AC-12 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows`、`ic/snapshot_test.go::TestTakeSnapshotStates`（`blocked on an unknown shape`） | test | 2-2・4-1 | PR-2、PR-4 |
| AC-13 | `ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・4-1 | PR-2、PR-4 |
| AC-14 | `ic/relay_test.go::TestRelayClaudeSendsChosenOption`、`ic/relay_test.go::TestRelayClaudeRechecksBeforeTheKey`（transcript だけ、Herdr だけが変わる場合） | test | 4-4 | PR-4 |
| AC-15 | `ic/screen_test.go::TestClaudeScreenAgrees`、`ic/relay_test.go::TestRelayClaudeRechecksBeforeTheKey`（画面だけが違う、画面を読めない）、`ic/relay_test.go::TestRelayClaudeScreenMismatch`（送らず、行は「要確認」にならない）、実地の確かめ（`screen_mismatch`） | test、manual | 2-4・4-4・4-6 | PR-2、PR-4 |
| AC-16 | `ic/relay_test.go::TestRelayClaudeSendsChosenOption`（番号のキーだけ）、`ic/relay_test.go::TestRelayRefusesOption`（Claude Code の行を transcript の番号で直す） | test | 4-4 | PR-4 |
| AC-17 | `ic/relay_test.go::TestTranscriptCheckValue`、`ic/relay_test.go::TestRelayClaudeReasons`（`check_mismatch`） | test | 2-5・4-4 | PR-2、PR-4 |
| AC-18 | `ic/relay_test.go::TestRelayFailureAfterKeyStops`（照合値を transcript から作り、送り直さないことと止めることを同じく確かめる）と、OpenCode の場合の中継の今のテスト | test | 4-4 | PR-4 |
| AC-19 | `ic/transcript_test.go::TestReadClaudeWait`（場所が決まらない 3 つ、`Reason`）、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・4-1 | PR-2、PR-4 |
| AC-20 | `ic/transcript_test.go::TestReadClaudeWait`（ファイルがない：`working`・`idle` で「なし」、`blocked` で「要確認」）、`ic/claudewait_test.go::TestJudgeClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-2・4-1 | PR-2、PR-4 |
| AC-21 | `ic/screen_transcript_calls_test.go::TestReadOpenCallsRejects`、`ic/screen_transcript_calls_test.go::TestReadOpenCallsPartialLastLine`、`ic/transcript_test.go::TestReadClaudeWait`、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 1-6・2-2・4-1 | PR-1、PR-2、PR-4 |
| AC-22 | `ic/transcript_test.go` の上限の誤りのテスト（`errTranscriptLimit`）、`ic/transcript_test.go::TestReadClaudeWait`（先頭に質問のある上限を超える transcript が「要確認」、上限ちょうどは読める）、`ic/snapshot_test.go::TestClaudeWaitRows` | test | 2-1・2-2・4-1 | PR-2、PR-4 |
| AC-23 | `ic/transcript_test.go::TestReadClaudeWait`（開けない、通常のファイルでない）、`ic/snapshot_test.go::TestClaudeWaitRows`、`ic/relay_test.go::TestRelayClaudeReasons`（`no_transcript`・`unsupported_path`） | test | 2-2・4-1・4-4 | PR-2、PR-4 |
| AC-24 | `ic/claudewait_test.go::TestQuestionStop`、`ic/send_test.go::TestSendStopsOnQuestion`、`ic/autosend_test.go::TestAutoSendStopsOnQuestion`。workspace の削除は、ステップ 3-4 で削除のコードがあれば足すテスト。なければ 0092 の計画書に書き足したテスト（§8 の注） | test、manual | 2-6・3-3・3-4 | PR-2、PR-3 |
| AC-25 | `ic/claudewait_test.go::TestQuestionStop`（画面の質問、読めない画面）、`ic/send_test.go::TestSendStopsOnQuestion`（送り先でない pane の画面の質問・読めない画面）、`ic/autosend_test.go::TestAutoSendStopsOnQuestion`（同じ 2 つが止まり続ける）、`ic/send_test.go::TestSendChecksBeforeFirstInput`（入力欄の形の確かめ） | test | 2-6・3-3・4-3 | PR-2、PR-3、PR-4 |
| AC-26 | `ic/snapshot_test.go::TestClaudeWaitRows`（知らない形の画面）、`ic/snapshot_test.go::TestTakeSnapshotStates`（`idle on a screen of no known shape` を直す）、`ic/claudewait_test.go::TestQuestionStop`（知らない形で止めない） | test | 2-6・4-1 | PR-2、PR-4 |
| AC-27 | CLAUDE.md の「Declare, don't infer」と「Go Idioms」の chief の例外の文（自然言語の文だけを定める） | manual | 1-7・5-1 | PR-1、PR-5 |
| AC-28 | chief 利用ガイドの 1 章の表・中継の照合・中継しない質問の文（自然言語の文だけを定める） | manual | 5-2 | PR-5 |

-   要件書 4.1（性能）は、`ic/claudewait_test.go::BenchmarkReadClaudeWait` と §7 の「PR-2 の実行の記録」で確かめる（ステップ 2-3）。
-   要件書 4.2・設計書 7.5 のセキュリティの項目は、次の行が受け持つ。
    -   制御文字の置き換え: AC-01・AC-10 の行。
    -   2,000 文字を超える入力を切ること: AC-10 の行。
    -   `Type something.` を名前に持つ選択肢の画面が合わないこと: AC-15 の行（ステップ 1-2 で採る組）。
    -   行の形の違いで transcript 全体を読めなくすること: AC-21 の行。
    -   `Why` と `why` に transcript の中身が出ないこと: ステップ 2-2・2-4 のテスト（`TestReadClaudeWait`・`TestClaudeScreenAgrees`）。
-   `Protocol` と `/chief` の定義・手引きの番号がそろうことは、今の `cmd/chief/docs_test.go::TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool` が確かめる（ステップ 4-5）。

### PR-1 の実行の記録

2026-11-17 に、ステップ 1-1・1-2 の採取の途中で、決定 5 のとおり止まった。要件書の中心の前提（答えを待っている間、その呼び出しが transcript に書かれていること）が成り立たなかったためである。人と相談して、タスクを取り下げた。採った生のデータは、採取した人を特定できる値を含むので、リポジトリには置いていない。

-   **版と採り方。** Claude Code 2.1.326、Herdr 0.9.1。人が採取用の workspace `chief-poc` の Claude Code の pane で、合成の依頼文を送って状態を作った。`/runplan` のセッションは、`herdr agent get`・`herdr agent read`（`--source visible --lines 200`）と transcript のファイルの写しだけで採った。
-   **止めた理由: 答えを待っている質問の呼び出しが transcript に書かれない。** 1 つを選ぶ問い・複数を選ぶ問い・問いが 2 つの呼び出しなど 7 回、`AskUserQuestion` の問いを画面に出した。そのうち 5 回は、問いが画面に出て Herdr が `blocked` と報告している間、transcript にその呼び出しの行がなかった。そのうち 1 回は、3 秒おきに 3 分読んでも、行の数が変わらなかった。行は、人が答えた時点で、同じターンのほかの行（思考、`Agent` の呼び出しとその答え）とまとめて書かれた。行の `timestamp` は、問いが出た時刻だった。残りの 2 回は、問いが出ている間に書かれていた。書かれるかどうかを、chief が見分ける手段はない。このため、transcript の答えを待っている呼び出しから質問を示すこと（F-001）も、中継の直前の照合（F-003）も、送る前の止め（F-005 の (a)）も成り立たない。
-   **成り立った前提。** 権限の確認の間、Herdr は `blocked` と報告し、transcript にはその `Bash` の呼び出しが答えのないまま書かれていた。
-   **そのほかに分かったこと。** 後のタスクで使えるよう記録する。
    -   1 つを選ぶ問いで、選択を自由に書く答え（`Type something.`）かチャット（`Chat about this`）に動かすと、問いが出たまま、Herdr の状態は `idle` になった。選択を番号の選択肢に戻すと `blocked` に戻り、`state_change_seq` が変わった。
    -   自由に書く答えに打った文字は、その行の名前として描かれた（`4. abc`）。選択を 1 に戻しても残った。
    -   問いの文が折り返すときは、各行の頭に `│` が付いた。選択肢の名前の折り返しの続きの行は、説明と同じ字下げで描かれた。
    -   人が Esc で閉じた問いは、`is_error` が `true` の `tool_result` になった。
    -   前面での実行を頼んでも、`Agent` の呼び出しには、すぐ `async_launched` の答えが付いた。サブエージェントの行は、親のセッションの下の `subagents/agent-*.jsonl` に書かれた。サブエージェントの権限の確認が出ている間、Herdr は `blocked` と報告したが、親の transcript に答えを待っている呼び出しはなかった。
    -   裏のサブエージェントの権限の確認は、親の問いが出ている間は画面に出ず、問いを閉じた直後に出た。そのとき `state_change_seq` は変わらなかった。
    -   起動した直後の、まだ何も入力していないセッションでは、Herdr は `idle` と報告し、transcript のファイルはなかった。
    -   transcript に、`permission-mode` という種類の行があった。
-   **採らなかったもの。** `/compact`・`--resume`・`/rewind` の後の transcript は、止めたので採っていない。

### PR-2 の実行の記録

（PR-2 の `/runplan` が書く。）

### PR-3 の実行の記録

（PR-3 の `/runplan` が書く。）

### PR-4 の実行の記録

（PR-4 の `/runplan` が書く。）

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 の表のすべての行の確かめが通り、PR-4 の実地の確かめでステップ 4-6 の項目を確かめた。AC-24 のうち workspace の削除の部分は、ステップ 3-4 で 0092 の文書に書き足した場合、0092 の計画書のテストが受け持つものとして、PR-3 の実行の記録に書く。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。各テストは壊して失敗を確かめ、commit のメッセージに書いた。消した行・subtest・テスト関数は、カバレッジの比べの結果を書いた（決定 9）。
-   **性能:** 64 MiB の transcript の判定が 1 秒以内であることを、PR-2 のベンチマークで記録した。
-   **セキュリティ:** 設計書 7.5 の項目をテストで確かめた。送る入力の範囲（AC-16）と送る直前の確かめ（AC-14・AC-15）は、テストと実地の確かめで確かめた。
-   **文書:** `/chief` の定義、CLAUDE.md、chief 利用ガイド、`package_reference.md` が実装と一致する。

## 9. 次のステップ (Next Steps)

-   PR-4 のマージの後、人が決定 3 のとおり chief の worktree を `main` に戻し、`main` から `make install-chief` で入れ直して `/chief` を起動し直す。
-   使わなくなった Claude Code の画面の読み取りの整理は、#908 で扱う（設計書 1.2・9 章）。
-   選択肢が 8 以上の問いの中継と、transcript の行の読み方を 0095 とそろえることは、設計書 9 章のとおり別のタスクで考える。

## 変更履歴

- 2026-11-17 承認（isseis、#954）
- 2026-11-17 withdrawn：Claude Code が答えを待っている質問の呼び出しを transcript に書かないことがあると分かったため（§7「PR-1 の実行の記録」）（#未記入）
