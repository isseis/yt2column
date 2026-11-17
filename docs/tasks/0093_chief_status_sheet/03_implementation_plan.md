# 実装計画書：chief の状況の表に、急ぎの度合いの印と、タスクの進行の位置とどちらの番かを示す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-17 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `61a16bb3` で確かめた。設計書が確かめたコミット `bade98e3` から、`internal/chief`・`cmd/chief`・`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` は変わっていない（`git diff --stat bade98e3 61a16bb3 -- <これらのパス>` の出力が空だった）。そのため、設計書が引くこれらの行番号は今も合う。

用語は、要件書 3 章・6 章と設計書の冒頭と同じ意味で使う（急ぎの度合い・番・進行の位置・実行中のコマンド・文書の段・文書の規則・行の仕上げ・進み具合・表示のための記録の読み取り など）。本計画は、次の用語を加える。

-   **0092 の確かめ:** 0092 の削除の提案が `origin/main` にあるかを確かめる手順（§1.3 の決定 6）。
-   **実地の確かめ:** 本物の Herdr とエージェントで行う手での確かめ（設計書 7.3）。結果を §7 の実行の記録に書く。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、道具 `chief` が行ごとに急ぎの度合い・番・進行の位置を決めて出力し、行を急ぎの度合いの順に並べ、`/chief` の定義と chief 利用ガイドがそれを示すようにして、要件書の F-001〜F-006 を満たす。

### 1.2. 実装の原則

-   **入出力を持たない規則から作り、出力の形を変える配線は最後の PR に置く。** 規則と型（PR-1）→ 実行中のコマンドの決定（PR-2）→ 配線・出力・`Protocol`・文書（PR-3）の順に PR を分ける（設計書 8 章、§1.3 の決定 3）。PR-1 と PR-2 は、道具の出力と表の並びを変えない。
-   **判定は 1 か所に置く。** 文書の段・急ぎの度合いの対応表・文書の規則・進み具合は、それぞれ 1 つの関数に置き、`position.go` にまとめる（設計書 1.1・2.2）。
-   **外部のコマンドと時刻は差し込む。** テストは herdr・git・gh を本物で呼ばず、読み直しの待ちは `Deps.Sleep` で置き換える（0088 計画書の決定 19）。セッションの記録は、テストの一時ディレクトリの `CLAUDE_CONFIG_DIR` の下に置く（`internal/chief/sim_test.go:51`・`:112` の `writeTranscript`）。
-   **各テストは壊して失敗を確かめる。** 確かめた結果を commit のメッセージに書く（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **`Protocol` は、実装するときの `origin/main` の値に 1 を足した値にする（2026-11-17、人が選んだ）。** 0092 の PR-7 も `Protocol` を 1 から 2 に上げる（0092 計画書の決定 3）。後から入る方が同じ番号のままだと、出力の形が違う 2 つの版を番号で見分けられない。0093 が先に入れば 2、0092 の PR-7 が先なら 3 になる。設計書 3.8 の「1 から 2 に上げる」は「1 つ上げる」と読み、設計書は直さない。`/chief` の定義と手引きの番号は、今のガードテスト（`TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool`）が定数と比べる。0092 にも同じ扱いを申し送る（0092 の [design_handoff.md](../0092_chief_task_workspace/design_handoff.md) の H-10）。ADR: なし（番号の扱いはこのタスクの中で閉じるため）。
2.  **書きかけの記録の読み直しは、最大 3 回、200 ms の間隔とする（2026-11-17、人が選んだ）。** 表示のための読み取りは、周ごとに `working` の Claude Code の pane ごとに行うので、待ちが長いと周が遅れる。pane ごとの待ちは最長 0.6 秒である。`/model` の後の読み直しの定数（`internal/chief/transcript.go:37-38`、10 回・500 ms）とは別の定数にする。ADR: なし（定数の値のため）。
3.  **PR を 3 つに分け、実地の確かめは PR-3 の中で行う（2026-11-17、人が選んだ）。** 設計書 8 章の 1 を PR-1、2 を PR-2、3〜5 を PR-3 にする。8 章の最後の段落のとおり、出力の形を変える `Row.Running` の型の変更、`Protocol`、`/chief` の定義は同じ PR（PR-3）に置く。実地の確かめは、0088・0092 と同じく、人が chief の worktree を PR-3 のブランチにしてそこから `make install-chief` を行い、確かめてからマージする（0092 計画書の決定 12）。ADR: なし（PR の切り方はこのタスクの中で閉じるため）。
4.  **文書の規則と進み具合の関数は、実行中のコマンドを `Row` とは別の引数で受け取る。** PR-1 と PR-2 では `Row.Running` を今の `*SendRecord` のまま残し、出力の形を変えないためである（決定 3）。PR-3 の行の仕上げが、`Row.Running` から実行中のコマンドを渡す。急ぎの度合いの対応表は `Row` だけを読む。ADR: なし（関数の形のため）。
5.  **実行中のセッションの記録のサンプルは、PR-1 の最初に、`/runplan` のセッションがこの計算機の既存の記録から読み取りだけで写して作る。** 設計書 7.1 は、計画の最初にサンプルを採るとしている。写す元は `~/.claude/projects/` の下の、コマンドが走っている途中の記録と、chief が答えを中継した後の記録である。採った人を特定できる値は、`internal/chief/testdata/README.md` の「置き換えたもの」の規則で置き換える。中継の後の記録が見つからないときは、人に頼み、採取用の workspace（0088 で使った `chief-poc`）で作ってもらう。サンプルが設計書の前提（すべての行が改行で終わる、中継の後の行が採取した種類の origin を持つ）と違うときは、そこで止まって人と決め直す（requirements_process.md の「Editing an approved document」）。ADR: なし。
6.  **0092 の確かめは、PR-1 と PR-3 の作業の最初に `/runplan` が行い、両方の PR のマージの前に人が行う。** 設計書 3.12 は、急ぎの度合いの対応表を実装する PR の前と、そのマージの直前に確かめるとしている。本計画では、対応表は PR-1 で作り、`Row.Idle` と並べ方への効き目は PR-3 の配線で生じるので、両方の PR で確かめる。確かめ方は、`git fetch origin main` の後に `git grep -n PendingRemoval origin/main -- internal/chief` を走らせ、終了コード 1（見つからない）なら 0092 の削除の提案はまだない（0092 設計書 3.6 の `PendingRemoval`）。`61a16bb3` では終了コード 1 だった。見つかったときは、その PR で次の 3 つを足し、AC-05 を確かめる（設計書 3.12）。(1) 削除の提案がある行を `Row.Idle` に当てない。(2) 急ぎの度合いの対応表の順 3 の前に「あとで」・人の行を足す。(3) 文書の規則の規則 2 を足す。マージの前の確かめは、`/runplan` が PR の説明に、人がマージの前に行う項目として書く。ADR: なし。
7.  **計画書が `approved` で、マージしていない最初の PR の番号を決められないときは、文書の規則の規則 10（「要確認」）とする。** PR マーカーの番号が数として読めないときや、PR の一覧が空のときに当たる。AC-07 の規則 7 は N を求めるので、N のないまま「PR-N」と示さないためである。ADR: なし（AC-07 の読み方の細部のため）。
8.  **送信の記録を読めないことは、`problems` に足さず、`running_unknown` の `unreadable` にだけ出す。** 今の `addRecords` は、「実行中」の行の `LastSend` が失敗すると、`problems` に「the records of … cannot be read」を足す（`internal/chief/snapshot.go:206-209`、`internal/chief/snapshot.go:135-137`）。設計書 3.5 の「失敗のとき」（申し送りの H-02）は、表示のための読み取りの失敗が `problems` を変えないとしている。送信の停止の記録を読めないことは、今までどおり `problems` に出す。ADR: なし（設計書の決定の範囲のため）。

### 1.4. 既存コード調査結果

| 領域 | 今あるもの | 変えること |
|---|---|---|
| 次のコマンドの規則 | `decideNext`（`internal/chief/state.go:273-309`）の `switch` が、文書の状態から次のコマンドを決める。`nextFromPlan`（`:314-342`）が計画書から決める | 文書の判定を文書の段の関数に取り出し、`decideNext` はその段を読む。振る舞いは変えず、`TestNextCommandRules`（`internal/chief/snapshot_test.go:200`）を変えずに通す。PR-1 |
| `classify` | `internal/chief/state.go:382-445`。`Idle` は `:434`（detached で fetch が成功）と `:442`（次のコマンドが `/mkreq`）で付く。文書を読めないことは `Unreadable` の文字列「task documents on origin/main」だけで表す（`internal/chief/snapshot.go:259-265`） | `readWorkspace` が文書を読めないことを真偽値でも残し、`classify` が `positionFacts` を `Row` に残す（設計書 3.1・3.3）。PR-1 |
| 計画書の PR | `Plan.PRs` は `Target`・`Created`・`Merged` を持つ（`internal/chief/task.go:483-504`）。`Plan.Done`（`:693-703`）は、すべての PR が作られてマージされたことを見る。単一の PR の計画書の `Target` は `single`（`:351`） | 文書の規則が、`Merged` でない最初の PR の番号を読む。PR-1 |
| 計画書のサンプル | `internal/chief/testdata/plans/` に 26 個。最初の PR の前（`marker-real-forms.md`）、途中の PR（`marker-opus.md` など）、すべてマージ（`marker-all-merged.md`）、単一の PR（`single-recorded.md`）がある。PR を作ってマージする前のサンプルはない（`61a16bb3` で、各ファイルの `- [x] PR を作成した` と `- [x] PR がマージされた` の数がどれも等しかった） | PR を作ってマージする前のサンプルを足す。PR-1 |
| マージの確認の並べ方 | `mergeOnlyRank`（`internal/chief/snapshot.go:308-313`）が `Row.MergeQuestion` と `Questions` を読む。`Row.MergeQuestion` は `classify` が付け（`internal/chief/state.go:395-397`）、出力の `merge_question`（`internal/chief/output.go:35`・`:193`）と `snapshot_test.go:60`・`:65` が読む | 「すべての質問がマージの確認」を 1 つの関数に取り出し、`mergeOnlyRank` と進み具合が使う（PR-1）。`Row.MergeQuestion` と `merge_question` を外す（PR-3） |
| 送信の記録 | `SendRecord`（`internal/chief/records.go:658-667`）は送ったコマンドを文字列 `Command` だけで持つ。開始の記録を書くのは `sendContext.run`（`internal/chief/send.go:461`）と中継（`internal/chief/relay.go:250`）の 2 か所。終わりの記録（`sendEnd`、`:670-675`）の結果を読む関数はない。`LastSend`（`:824-839`）は中継を除いた最後の開始の記録を返す。記録は知らない欄を持つものを読まない（`:158-172`） | `Kind`・`Arg` の欄を足し、`sendCommand`（`internal/chief/send.go:399`）の経路だけが書く。送信の終わりの結果を読む関数を足す。PR-2 |
| コマンドの種類 | `paneinput.CommandKind`（`internal/chief/paneinput/paneinput.go:92-107`）と `Command`（`:288-294`）は、種類と引数を外へ出さない。引数の型は `ParseTaskID`（`:37`）、`ParseIssueNumber`（`:66`）、`fixprguard.ParsePRNumber`（`internal/fixprguard/types.go:134`） | 種類の名前の表と、`Command` の種類と検証した引数の文字列を読む関数を足す。PR-2 |
| セッションの記録の読み取り | `transcriptPath`（`internal/chief/transcript.go:53-69`）、`readTranscript`（`:84-96`、上限 `resetTranscriptLimit` 64 MiB は `:30`）、`readLastHumanInput`（`internal/chief/screen_transcript.go:170-189`）。改行で終わらない最後の行は JSON として読めず、`HumanInputUnreadable` になる。リセットの確かめ（`internal/chief/reset.go:51-73`）がこれらを使う。サンプルは `claude-human-input.jsonl`・`claude-queued-input.jsonl`・`claude-clear-model.jsonl` の 3 つ | 改行で終わるまで読み直す読み取りと、人の最後の入力を `RunningCommand` で返す入口を足す。`readLastHumanInput` とリセットの確かめは変えない。PR-2 |
| 「実行中」の行 | `addRecords`（`internal/chief/snapshot.go:169-216`）が、`working` の pane への `LastSend` を `Row.Running`（`*SendRecord`、`internal/chief/state.go:131-133`）に入れる。テストは `TestRunningRowShowsTheLastSend`（`snapshot_test.go:537`）、`relay_test.go:471-473`、`autosend_test.go:227` | 実行中のコマンドの決定に置き換え、`Row.Running` を `*RunningShown` にする（設計書 3.1・3.5）。3 つのテストを直す（設計書 3.11）。PR-3 |
| 自動送信の周 | `applyAutoSends`（`internal/chief/tick.go:79-104`）が、送った行を「実行中」にし、`:95` で `SendRecord` を組み立てる。送信の停止は読み直さない | 提案のコマンドから実行中のコマンドを作り、入力を打った workspace の送信の停止を読み直す（設計書 1.2・3.5）。PR-3 |
| 並べ方と行の仕上げ | `finishSnapshot`（`internal/chief/snapshot.go:148-160`）が人の判断を待つものを並べ、`sortRows`（`:339-354`）が `idleRank` → 状態の順 → `mergeOnlyRank` → 時刻 → パスで並べる | 人の判断を待つものを並べる前に行の仕上げを呼び、`sortRows` の 2 番目に急ぎの度合いを足す（設計書 3.6・3.7）。PR-3 |
| 出力 | `RowOutput`（`internal/chief/output.go:27-41`）、`RunningOutput`（`:236-242`）、`StopOutput`（`:245-249`）、`Snapshot.Output`（`:174-232`）。範囲の外の値は `unknownName`（`internal/chief/records.go:688`）。`Protocol` は `internal/chief/records.go:28` | 設計書 3.8 のとおりにする。`Protocol` を決定 1 の値にする。PR-3 |
| `/chief` の定義と手引き | `--protocol 1` は `.claude/commands/chief.md` の 97・106・228・232・238・248・250・254・307 行と、`docs/dev/guides/chief_guide.md:28`（0092 計画書 §1.4 と同じ箇所。`61a16bb3` で `rg -n -- '--protocol 1' .claude/commands docs/dev/guides/chief_guide.md` と `grep -n プロトコル docs/dev/guides/chief_guide.md` で確かめた）。表の示し方は `.claude/commands/chief.md:124-145` | 設計書 3.9・3.10 のとおりにする。PR-3 |
| 文書のガードテスト | `cmd/chief/docs_test.go` の `TestDocumentedProtocolMatches`（`:205`）と `TestGuideValuesMatchTool`（`:405`）が番号を定数と比べる | 値の対応表のテストを足す（設計書 7.2）。番号のテストは変えない。PR-3 |
| パッケージの一覧 | `docs/dev/conventions/package_reference.md:135-136` が `internal/chief` と `paneinput` の主な exported の名前を挙げる | 新しく exported にした名前を足す。PR-3 |

具体の型で振る舞いを分ける設計は、`RunningCommand` だけである（設計書 3.1）。種類と引数の欄は unexported で、構築関数だけが作る。`internal/chief` の中では複合リテラルで作れるが、0 の値（種類 0）は 5 つのコマンドのどれでもなく、示さない（ステップ 1-4 のテスト）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: サンプル、型、入出力を持たない規則（設計書 8 章の 1）

**変更するファイル:** `internal/chief/testdata/transcript/`・`testdata/plans/`・`testdata/README.md`、`internal/chief/position.go`・`position_test.go`（新規）、`internal/chief/running.go`（新規、型と構築関数だけ）、`internal/chief/state.go`、`internal/chief/snapshot.go`、`internal/chief/screen_test.go`、`internal/chief/snapshot_test.go`

-   [ ] **ステップ 1-1**: 実行中のセッションの記録のサンプルを採る（§1.3 の決定 5、設計書 7.1）。コマンドが走っている途中の記録と、chief が答えを中継した後の記録を `testdata/transcript/` に置き、採った日・版・置き換えを `testdata/README.md` に足す。`screen_test.go::TestReadLastHumanInput` に、各サンプルの期待する結果の行を足す。すべての行が改行で終わることも確かめる。
-   [ ] **ステップ 1-2**: 0092 の確かめを行い（§1.3 の決定 6）、結果を §7 の「PR-1 の実行の記録」に書く。見つかったときは、決定 6 の 3 つを、このフェーズのステップ 1-5・1-6 に含める。
-   [ ] **ステップ 1-3**: `position.go` に、設計書 3.1 の `Urgency`・`Turn`・`PositionDocument`・`Progress`・`Position` と、文書の段を返す関数を書き、`decideNext` が段を読むようにする（設計書 3.3）。`TestNextCommandRules` を変えずに通す。
-   [ ] **ステップ 1-4**: `running.go` に、設計書 3.1 の `RunningCommand` と、種類と引数から作る構築関数を書く。引数は設計書 3.5 の「引数」の表の型で検証し、検証できない引数と空の引数は引数なしにする。テストは、5 つの種類と、検証できない引数（制御文字、数でない文字列、2 つ目の引数）、0 の値と範囲の外の種類を確かめる。
-   [ ] **ステップ 1-5**: `positionFacts` を `Row` に足し、`classify` が残すようにする（設計書 3.1・3.3）。文書を読めないことは、`readWorkspace` が `workspaceFacts` に真偽値で残す。`Row.StopUnreadable` も足す（値を入れるのは PR-3）。テストは、`classify` が fetch の失敗、文書を読めないこと、HEAD の 3 つの状態、命名に合うかどうか、段、計画の完了、PR の番号を残すことを確かめる。
-   [ ] **ステップ 1-6**: `position.go` に、急ぎの度合いの対応表（設計書 3.2）、文書の規則（設計書 3.3、§1.3 の決定 4・7）、進み具合（設計書 3.4）の関数を書く。「すべての質問がマージの確認」を 1 つの関数に取り出し、`mergeOnlyRank` もそれを使う。
    -   対応表のテストは、6 つの状態 × 送信の停止（なし・あり・読めない）× `Idle` の有無のすべてを確かめ、どれも順 1〜6 のどれかに当たることを確かめる（AC-01〜AC-04・AC-06）。
    -   文書の規則のテストは、規則 1・3〜10 のそれぞれだけに当たる事実を作る。命名に合わないブランチの「次を提案」（`/mkreq`）と「レビュー待ち」の行、detached の行（`/mkreq` の実行中とそれ以外）、決定 7 の場合、AC-08 の 2 つの場合を含める。規則 7 は `testdata/plans/` のサンプルを `ParsePlan` で読んで確かめ、PR を作ってマージする前のサンプルを足す（AC-07・AC-08）。
    -   進み具合のテストは、AC-09 の表の各行と、質問のない「質問で停止」の行を確かめる。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。`snapshot` と `tick` の出力と行の並びは変わらない（`snapshot_test.go` と `autosend_test.go` の今のテストが変えずに通る）。

### PR-1 作成ポイント: samples, position types, and pure position rules

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0093): 実行中の記録のサンプルを採り、急ぎの度合い・文書・進み具合の規則を作る`

**レビュー観点**: 文書の段を取り出した後も次のコマンドが変わらないか / 文書の規則の順（とくに detached と命名に合わないブランチの行）が設計書 3.3 のとおりか / 規則が `Unreadable` の文字列を読まず、宣言した値だけで決めているか / サンプルに採った人を特定できる値が残っていないか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-3・1-6 は、次のコマンドの規則を変えずに文書の段を取り出し、AC-07 の 10 の規則を順に当てる規則の表である。Sonnet は、detached の行を「タスクなし」にまとめるか、規則 1 を `Unreadable` の文字列から判定しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 実行中のコマンドの決定（設計書 8 章の 2）

**変更するファイル:** `internal/chief/paneinput/paneinput.go`・`paneinput_test.go`、`internal/chief/records.go`・`records_test.go`、`internal/chief/send.go`・`send_test.go`、`internal/chief/transcript.go`、`internal/chief/screen_transcript.go`・`screen_test.go`、`internal/chief/running.go`・`running_test.go`、`internal/chief/reset_test.go`、`internal/chief/testdata/transcript/`

-   [ ] **ステップ 2-1**: `paneinput` に、`CommandKind` の名前の表（`mkreq`・`mkarch`・`mkplan`・`runplan`・`runpr`）と、`Command` の種類と検証した引数の文字列を読む関数を足す（設計書 3.1）。テストは、名前と種類の往復、表にない名前、0 の値の `Command` を確かめる。
-   [ ] **ステップ 2-2**: `SendRecord` に `Kind`・`Arg` を足し、`sendCommand` の経路だけが書く。引き継ぎの `/runplan` も `runplan` と書く。送信の終わりの記録の結果を読む関数を足す（設計書 3.1・3.5）。テストは、コマンドの送信が欄を書くこと、リセットと中継が書かないこと、欄のない記録が読めること、終わりの記録がない送信と結果ごとの読み取りを確かめる。
-   [ ] **ステップ 2-3**: `transcript.go` に、改行で終わるまで最大 3 回、200 ms の間隔で読み直す読み取りを足す（§1.3 の決定 2、設計書 1.2）。採った記録の最後の行の途中で切ったサンプルを足す。テストは、`Deps.Sleep` の差し込みで、読み直しの間に改行で終わる場合と、終わらずに「決まらない」になる場合を確かめる。改行で終わらない記録を、リセットの確かめが今までどおり読めないものとすることを `reset_test.go` で確かめる。
-   [ ] **ステップ 2-4**: `screen_transcript.go` に、改行で終わる記録から人の最後の入力を `RunningCommand` で返す入口を書く（設計書 3.5）。`readLastHumanInput` は変えない。テストは、`testdata/transcript/` のすべてのサンプル、5 つの名前とほかの名前、自由な文、人の入力がない記録、読めない記録、検証できない引数を確かめる。
-   [ ] **ステップ 2-5**: `running.go` に、pane ごとの決め方（AC-11）、まとめ方（AC-12）、モデル（AC-14）、決まらない理由（`RunningUnknown`）を書く（設計書 3.5・6.1）。行にはまだつながない。テストは、設計書 7.1 の「実行中のコマンド」の OpenCode の 6 つの記録、Claude Code のサンプル、2 つの pane の 3 つの場合、検証できない引数を落とした pane、モデルを示す 3 つの条件、決まらない理由のすべてを確かめる。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。`snapshot` と `tick` の出力は変わらない。

### PR-2 作成ポイント: running command from transcripts and send records

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0093): セッションの記録と送信の記録から実行中のコマンドを決める部品を作る`

**レビュー観点**: セッションの記録の受け付ける形を広げていないか（書きかけの行を飛ばさない、記録の文字列をそのまま出さない） / 送信の記録から種類を宣言した欄だけで読み、コマンドの文字列を読み直していないか / 読み直しがリセットの確かめに及んでいないか / まとめ方とモデルの条件

**実装モデル要件**: opus

**判定理由**: ステップ 2-3・2-4 は、Claude Code が書く外部の値（セッションの記録）を表示の値に変える信頼の境界である（要件書 4.2、設計書 5.2）。Sonnet は、書きかけの最後の行を飛ばして 1 つ前のコマンドを示すか、記録の引数を検証せずに出しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 配線、出力、`Protocol`、`/chief` の定義と手引き、実地の確かめ（設計書 8 章の 3〜5）

**変更するファイル:** `internal/chief/state.go`・`snapshot.go`・`tick.go`・`output.go`・`records.go`・`position.go` とそのテスト、`internal/chief/relay_test.go`・`autosend_test.go`、`.claude/commands/chief.md`、`docs/dev/guides/chief_guide.md`、`cmd/chief/docs_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 3-1**: 0092 の確かめを行い（§1.3 の決定 6）、結果を §7 の「PR-3 の実行の記録」に書く。見つかったときは、決定 6 の 3 つをこのフェーズで足し、AC-05 を確かめる。
-   [ ] **ステップ 3-2**: `Row` を設計書 3.1 のとおりにする（`Running` を `*RunningShown` に、`RunningUnknown`・`Urgency`・`Turn`・`Position` を足し、`MergeQuestion` を外す）。`addRecords` が送信の停止を読めないことを `StopUnreadable` に残し、「実行中」の行の処理を実行中のコマンドの決定に置き換える（設計書 3.5、§1.3 の決定 8）。設計書 3.11 の 3 つのテストと `snapshot_test.go:60`・`:65` を直す。記録を読めないときに、行の状態・`Unreadable`・`problems` が変わらないことを確かめる（H-02）。
-   [ ] **ステップ 3-3**: `applyAutoSends` を設計書 3.5 の「自動送信の周」のとおりにし、入力を打った workspace の送信の停止を読み直す（設計書 1.2）。テストは、送った行の実行中のコマンドとモデル、打った後に失敗した行とコマンドの後のリセットが失敗した行がその周から「🔴 すぐ」になることを確かめる（AC-03・AC-10）。
-   [ ] **ステップ 3-4**: `finishSnapshot` が、人の判断を待つものを並べる前に行の仕上げを呼ぶようにし、`sortRows` の 2 番目に急ぎの度合いを足す（設計書 3.6・3.7）。テストは、送信が止まった「実行中」の行、停止を読めない行、送信が止まった「レビュー待ち」の行が「すぐ」の位置へ移ることと、`tick` で送った行の進み具合が「作成中」になることを確かめる。`TestRowOrderRules`・`TestTakeSnapshotOrder` は変えずに通す（AC-15〜AC-17）。
-   [ ] **ステップ 3-5**: 出力を設計書 3.8 のとおりにし、`Protocol` を §1.3 の決定 1 の値にする。値の名前の集合を返す関数を公開する（設計書 7.2）。テストは、`snapshot` と `tick` の JSON が行ごとに `urgency`・`turn`・`position`・`idle`・`running_unknown` を持ち `merge_question` を持たないこと、範囲の外の値が `unknown` になること、質問ごとの `merge` が残ること、偽った送信の記録の `Arg` とセッションの記録の検証できない引数が `running.command` に入らないことを確かめる（AC-13・AC-18・AC-20）。
-   [ ] **ステップ 3-6**: `.claude/commands/chief.md` を設計書 3.9 のとおりに直す。値から日本語の表示への対応は、ステップ 3-7 のテストが読める 1 つの Markdown の表に置く。
-   [ ] **ステップ 3-7**: `cmd/chief/docs_test.go` に、`/chief` の定義の値の対応表が、道具の `state`・`urgency`・`turn`・`position.document`・`position.progress` の値の集合をちょうど持つことを確かめるテストを足す（設計書 7.2）。
-   [ ] **ステップ 3-8**: `docs/dev/guides/chief_guide.md` を設計書 3.10 のとおりに直す。古い版に戻す手順は、送信の記録の新しい欄を古い道具が読めないことを書く。
-   [ ] **ステップ 3-9**: `package_reference.md` の `internal/chief` と `internal/chief/paneinput` の行に、このタスクで exported にした名前を足す。
-   [ ] **ステップ 3-10**: 実地の確かめを、人と行う（§1.3 の決定 3）。人が動いている `/chief` を止め、chief の worktree をこの PR のブランチの先頭にして `make install-chief` を行い、`/chief` と `/chief once` を起動し直す。設計書 7.3 の 5 つの項目を見る。`working` の Claude Code の pane が数個あり、記録が数十 MB ある状態で、1 周の時間を測る。結果を §7 の「PR-3 の実行の記録」に書く。マージの後、人が chief の worktree を `main` に戻し、`main` から入れ直す。
-   [ ] **ステップ 3-11**: PR の説明に、人がマージの前に行う項目として 0092 の確かめを書く（§1.3 の決定 6）。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。実地の確かめの結果が §7 にある。

### PR-3 作成ポイント: wiring, output, protocol, chief command, and guide

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11

**推奨タイトル**: `feat(0093): chief の状況の表に急ぎの度合い・番・進行の位置を示し、急ぎの度合いの順に並べる`

**レビュー観点**: 行の仕上げが `applyAutoSends` の後の 1 か所だけにあるか / 表示のための読み取りの失敗が、行の状態・`problems`・送る判断を変えていないか / 出力の形と `Protocol` と `/chief` の定義が同じ PR で合っているか / `/chief` の定義がセッションに印や並びを決めさせていないか

**実装モデル要件**: opus

**判定理由**: ステップ 3-2〜3-5 は、0088 の決まり（AC-02・AC-03・AC-05）を変える多くのテストの直しと、`Protocol` を上げる移行を同時に持つ（mkplan.md の panel-mode の条件）。Sonnet は、テストの期待を新しい振る舞いに合わせるだけで直し、0088 の振る舞いのうち変えないもの（`Idle`、人の判断を待つものの順）を壊しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（PR-1） | 実行中の記録のサンプル、型、文書の段、急ぎの度合いの対応表、文書の規則、進み具合 | 規則がテストで確かめられる（まだ行に使われない） |
| M2（PR-2） | 送信の記録の `kind`・`arg`、読み直し、実行中のコマンドの決定 | 実行中のコマンドを決められる（まだ行に使われない） |
| M3（PR-3） | 配線、出力、`Protocol`、`/chief` の定義、手引き、実地の確かめ | 表に印・番・進行の位置が出て、急ぎの度合いの順に並ぶ |

順序は設計書 8 章のとおりで、8 章から動かしたものはない。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | サンプル、型、文書の段、急ぎの度合いの対応表、文書の規則、進み具合 | sonnet+opus-review | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | コマンドの種類の名前、送信の記録の欄、読み直し、記録の入口、実行中のコマンドの決定 | opus | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11 | 配線、出力、`Protocol`、`/chief` の定義、手引き、文書のガード、実地の確かめ | opus | auto |

どの PR も、前の PR のマージと `/runplan` の間に、`/runplan` の外の作業を置かない。PR-1 のサンプルの採取と PR-3 の実地の確かめは、`/runplan` のセッションの中で行う。そのため、自動送信はすべて `auto` とする。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 規則（文書の段、急ぎの度合いの対応表、文書の規則、進み具合）は、事実を直接作る表のテストで確かめる。対応表は、入力の組み合わせのすべてを渡す（設計書 7.1）。
-   **サンプル:** セッションの記録は `testdata/transcript/` の実物から採ったサンプルで、計画書は `testdata/plans/` のサンプルで確かめる。新しいサンプルは `testdata/README.md` の規則で置き換える。
-   **結合:** `TakeSnapshot` と `Tick` を、今の偽の Herdr・git・状態のディレクトリ（`internal/chief/world_test.go`・`sim_test.go`）で走らせる。新しい結合のテストの仕組みは作らない（設計書 7.2）。
-   **時刻と待ち:** 読み直しの待ちは `Deps.Sleep` で差し込み、実時間で待たない。
-   **境界の値:** 読み直しの回数の前後（3 回目で終わる・終わらない）、記録の上限（64 MiB）を超える記録を確かめる。
-   **セキュリティ:** 検証できない引数と偽った記録の値が出力に入らないこと（ステップ 1-4・2-4・3-5、設計書 7.3）。送信の部品の結果が表示の値で変わらないことは、今の送信のテストが変えずに通ることで確かめる。
-   **後方互換:** 0088 の振る舞いのうち変えないもの（次のコマンド、`Idle`、人の判断を待つものの順、リセットの確かめ）は、今のテスト（`TestNextCommandRules`・`TestDetachedRowIdleOnlyWhenEverythingWasRead`・`TestIdleClearedByWhatTheHumanMustSee`・`TestRowOrderRules`・`TestResetChecks`）が変えずに通ることで確かめる。版の互換は持たない（利用者は 1 人。設計書 1.2）。
-   **テストの補助:** `internal/chief` の中だけで使う補助は、今の `internal/chief/test_helpers.go` と `world_test.go`・`sim_test.go` に足す（test_organization.md の Classification B）。ほかのパッケージから使う補助は作らない。
-   **文書のガード:** `cmd/chief/docs_test.go` が、`--protocol` の番号（今のテスト）と、値の対応表（ステップ 3-7）を確かめる。CI の `doc-guard` はすでに `./cmd/chief` と `./internal/chief` を走らせる。`/chief` の定義と手引きの文は、PR のレビューで読む。
-   **実地の確かめ:** PR-3 で、本物の Herdr で表と周の時間を確かめる（設計書 7.3）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 採ったサンプルが設計書の前提と違う（改行で終わらない行がある、中継の後の行が採取していない種類の origin を持つ） | §1.3 の決定 5 のとおり止まり、人と決め直す。PR-1 の最初に採るので、後の PR の作業は無駄にならない |
| 0092 と並行して進み、どちらかの PR が相手の変更を知らずにマージされる | 決定 6 の 0092 の確かめを PR-1・PR-3 の最初とマージの前に行う。`Protocol` は決定 1 で衝突を避ける（0092 の H-10） |
| 0094〜0097 のタスク（セッションの記録から質問と報告を読む）が `screen_transcript.go` と `transcript.go` を変え、衝突する | 2026-11-17 の時点では、どれも要件定義か設計の段にある。PR-2 の前に `origin/main` を取り込み、衝突は読み取りの受け付ける形を広げずに解く |
| PR-3 が大きく、レビューの焦点がぼやける | 規則と実行中のコマンドの決定を PR-1・PR-2 に分け、PR-3 には配線と文書だけを残す（決定 3） |
| 読み直しで周が遅れる | 決定 2 で pane ごとの待ちを最長 0.6 秒にする。PR-3 の実地の確かめで 1 周の時間を測る |
| 絵文字の幅で表の列がずれる | 印を最初の列に置き（設計書 1.2）、PR-3 の実地の確かめで見る。ずれが読みにくければ、PR-3 の中で人と決め直す |
| `make install-chief` の後に、古い定義の `/chief` が `protocol_mismatch` で止まる | PR-3 の実地の確かめで chief の worktree をそのブランチにし、マージの後に `main` から入れ直す（決定 3）。手引きに入れ替えの手順を書く（ステップ 3-8） |
| 日程: PR-3 の実地の確かめで、人の手が空かない | 実地の確かめは人がいるときにだけ進める。`/runplan` は人を待つだけで、ほかのタスクを妨げない |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

表のパスの `ic/` は `internal/chief/`、`cc/` は `cmd/chief/` を表す。テストの名前は計画の時点の名前で、実装で名前を変えたときは、この表を同じ PR で直す（編集上の訂正）。`manual` は、`/chief` の定義の文を PR のレビューで読むことと、PR-3 の実地の確かめを指す。AC-19 と AC-22 は、`/chief` の定義の文とセッションがそれに従うことだけで実現するので、`manual` だけで確かめる（文書の自然言語の内容だけを定める AC。requirements_process.md §4）。

| AC | 確かめ | 種類 | ステップ | PR |
|---|---|---|---|---|
| AC-01 | `ic/position_test.go::TestUrgencyTable`（3 つの値のどれか 1 つ）、`cc/docs_test.go::TestDisplayValuesMatchTool`（`urgency` の値の集合）、定義の印の表示、実地の確かめ | test、static、manual | 1-6・3-7・3-10 | PR-1、PR-3 |
| AC-02 | `ic/position_test.go::TestUrgencyTable`（すべての組み合わせがどれかの順に当たる） | test | 1-6 | PR-1 |
| AC-03 | `ic/position_test.go::TestUrgencyTable`（停止あり・読めない）、`ic/autosend_test.go::TestAutoSendRowAfterFailure`（その周から「すぐ」） | test | 1-6・3-3 | PR-1、PR-3 |
| AC-04 | `ic/position_test.go::TestUrgencyTable`（「要確認」は `Idle` のほか「あとで」） | test | 1-6 | PR-1 |
| AC-05 | 0092 が先に入ったとき: `ic/position_test.go::TestUrgencyTable` と `ic/snapshot_test.go::TestRowOrderRules` に削除の提案の行を足す。0092 が後のとき: 0092 が確かめる（0092 の H-09）。どちらかを §7 の実行の記録に書く | test | 1-2・3-1 | PR-1、PR-3 |
| AC-06 | `ic/position_test.go::TestUrgencyTable`（番） | test | 1-6 | PR-1 |
| AC-07 | `ic/position_test.go::TestDocumentRule`、`ic/position_test.go::TestClassifyKeepsPositionFacts`、`ic/snapshot_test.go::TestNextCommandRules`（変えずに通る） | test | 1-3・1-5・1-6 | PR-1 |
| AC-08 | `ic/position_test.go::TestDocumentRule`（状態が「要確認」で文書が読める、文書が「要確認」で状態が変わらない） | test | 1-6 | PR-1 |
| AC-09 | `ic/position_test.go::TestProgress` | test | 1-6 | PR-1 |
| AC-10 | `ic/autosend_test.go::TestAutoSendAfterMerge`（送ったコマンドとモデル、進み具合が「作成中」） | test | 3-3・3-4 | PR-3 |
| AC-11 | `ic/running_test.go::TestRunningCommandPerPane`、`ic/screen_test.go::TestReadRunningCommand`、`ic/screen_test.go::TestReadLastHumanInput`（新しいサンプル） | test | 1-1・2-4・2-5 | PR-1、PR-2 |
| AC-12 | `ic/running_test.go::TestRunningCommandPanes` | test | 2-5 | PR-2 |
| AC-13 | `ic/running_test.go::TestNewRunningCommand`、`ic/screen_test.go::TestReadRunningCommand`（検証できない引数）、`ic/snapshot_test.go::TestRunningOutputHasNoRawText` | test | 1-4・2-4・3-5 | PR-1、PR-2、PR-3 |
| AC-14 | `ic/running_test.go::TestRunningModel`、`ic/snapshot_test.go::TestRunningRowShowsTheLastSend`（書き直したもの）、定義の「次」の列 | test、manual | 2-5・3-2・3-6 | PR-2、PR-3 |
| AC-15 | `ic/snapshot_test.go::TestStoppedRowsOrderFirst`、`ic/snapshot_test.go::TestRowOrderRules` | test | 3-4 | PR-3 |
| AC-16 | `ic/snapshot_test.go::TestStoppedRowsOrderFirst`（「すぐ」の中の状態の順）、`ic/snapshot_test.go::TestTakeSnapshotOrder`（変えずに通る） | test | 3-4 | PR-3 |
| AC-17 | `ic/snapshot_test.go::TestRowOrderRules`・`ic/snapshot_test.go::TestTakeSnapshotOrder`（人の判断を待つものの順。変えずに通る） | test | 3-4 | PR-3 |
| AC-18 | `ic/snapshot_test.go::TestSnapshotRowOutput`（質問ごとの `merge` が残り、`merge_question` がない）、定義の「次」の列、実地の確かめ | test、manual | 3-5・3-6・3-10 | PR-3 |
| AC-19 | 定義の表の下の一覧 | manual | 3-6 | PR-3 |
| AC-20 | `ic/snapshot_test.go::TestSnapshotRowOutput`、`ic/autosend_test.go::TestTickOutputRows`（`tick` の出力） | test | 3-5 | PR-3 |
| AC-21 | `cc/docs_test.go::TestDisplayValuesMatchTool`、定義の「道具の順と値のまま示す」の文 | static、manual | 3-6・3-7 | PR-3 |
| AC-22 | 定義の「2. One round」が引数のない場合と `once` で共通であること、実地の確かめ | manual | 3-6・3-10 | PR-3 |

### PR-1 の実行の記録

（`/runplan` が、0092 の確かめの結果とサンプルの確かめの結果を書く）

### PR-3 の実行の記録

（`/runplan` が、0092 の確かめの結果、実地の確かめの結果、1 周の時間を書く）

## 8. 成功の基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-22 の確かめがすべて通る（AC-05 は §7 のとおり）。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。各テストは、確かめる部品を壊して失敗することを確かめてからコミットする。
-   **セキュリティ:** セッションの記録と送信の記録の文字列が、検証を通らずに出力に入らない（設計書 5.2）。送る入力の範囲と送る判断は変わらない。
-   **文書:** `/chief` の定義、chief 利用ガイド、`package_reference.md` が新しい出力と表に合う。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、人が chief の worktree を `main` に戻し、`main` から `make install-chief` を行って `/chief` を起動し直す（§1.3 の決定 3）。
-   0092 が後に入るときは、0092 が H-09 と H-10 を扱う。
