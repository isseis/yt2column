# 実装計画書：chief の状況の表に、急ぎの度合いの印と、タスクの進行の位置とどちらの番かを示す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-17 |
| Review date | 2026-11-17 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `61a16bb3` で確かめた。設計書が確かめたコミット `bade98e3` から、`internal/chief`・`cmd/chief`・`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` は変わっていない（`git diff --stat bade98e3 61a16bb3 -- <これらのパス>` の出力が空だった）。そのため、設計書が引くこれらの行番号は今も合う。

用語は、要件書 3 章・6 章と設計書の冒頭と同じ意味で使う（急ぎの度合い・番・進行の位置・実行中のコマンド・文書の段・文書の規則・行の仕上げ・進み具合・表示のための記録の読み取り など）。本計画は、次の用語を加える。

-   **0092 の確かめ:** 0092 の削除の提案が `origin/main` にあるかを確かめる手順（§1.3 の決定 6）。
-   **実地の確かめ:** 本物の Herdr とエージェントを使い、人の手で行う確かめ（設計書 7.3）。結果を §7 の実行の記録に書く。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、道具 `chief` が行ごとに急ぎの度合い・番・進行の位置を決めて出力し、行を急ぎの度合いの順に並べ、`/chief` の定義と chief 利用ガイドがそれを示すようにして、要件書の F-001〜F-006 を満たす。

### 1.2. 実装の原則

-   **入出力を持たない規則から作り、出力の形を変える配線は最後の PR に置く。** 規則と型（PR-1）→ 実行中のコマンドの決定（PR-2）→ 配線・出力・`Protocol`・文書（PR-3）の順に PR を分ける（設計書 8 章、§1.3 の決定 3）。PR-1 と PR-2 は、道具の出力と表の並びを変えない。
-   **判定は 1 か所に置く。** 文書の段・急ぎの度合いの対応表・文書の規則・進み具合は、それぞれ 1 つの関数に置き、`position.go` にまとめる（設計書 1.1・2.2）。
-   **外部のコマンドと時刻は差し込む。** テストは herdr・git・gh を本物で呼ばず、読み直しの待ちは `Deps.Sleep` で置き換える（0088 計画書の決定 19）。セッションの記録は、テストの一時ディレクトリの `CLAUDE_CONFIG_DIR` の下に置く（`internal/chief/sim_test.go:51`・`:112` の `writeTranscript`）。
-   **各テストは壊して失敗を確かめる。** 確かめた結果をコミットのメッセージに書く（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **`Protocol` は、実装するときの `origin/main` の値に 1 を足した値にする（2026-11-17、人が選んだ）。** 0092 の PR-7 も `Protocol` を 1 から 2 に上げる（0092 計画書の決定 3）。後から入る方が同じ番号のままだと、出力の形が違う 2 つの版を番号で見分けられない。0093 が先に入れば 2、0092 の PR-7 が先なら 3 になる。設計書 3.8 の「1 から 2 に上げる」は「1 つ上げる」と読み、設計書は直さない。`/chief` の定義と手引きの番号は、今のガードテスト（`TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool`）が定数と比べる。0092 の PR-7 と並行して進めているときに備え、PR-3 のマージの直前に `origin/main` の値を確かめ直し、同じ値なら 1 つ上げる（ステップ 3-11）。確かめ直すのは、2 つの PR が同じ値へ書き換えても、git は衝突なくまとめ、ガードテストも通ってしまうためである。0092 にも同じ扱いを申し送る（0092 の [design_handoff.md](../0092_chief_task_workspace/design_handoff.md) の H-10。0092 の計画書への取り込みは、人が 0092 のタスクに渡す。決定 9）。番号を 2 に固定し、0092 が先に入っていたら止めて決め直す案は採らない。並行するタスクごとに止まって人と決め直すことになり、止める手順がなければ衝突に気づかないためである。この決め方は、3 つの道具に共通する規則としてタスク 0099（[#933](https://github.com/isseis/yt2column/issues/933)、[PR #937](https://github.com/isseis/yt2column/pull/937)）が定め、規則の正本を CLAUDE.md に、決定を ADR-0014 に置く。本計画はその規則に先に従うだけで、規則を別に定めない。0099 の確かめがマージの経路に入った後は、PR-3 もその確かめを受ける。ADR: なし（0099 が ADR-0014 を更新する。同じ主題の決定を 2 つの ADR に分けないため）。
2.  **書きかけの記録の読み直しは、最大 3 回、200 ms の間隔とする（2026-11-17、人が選んだ）。** 最初の読み取りの後、改行で終わらなければ 200 ms 待って読み直すことを最大 3 回行う（読み取りは最大 4 回、待ちは最大 3 回）。待ちが中断されて失敗したときも、その pane は決まらないものとし、周は失敗させない。表示のための読み取りは、周ごとに `working` の Claude Code の pane ごとに行うので、待ちが長いと周が遅れる。pane ごとの待ちは最長 0.6 秒である。`/model` の後の読み直しの定数（`internal/chief/transcript.go:37-38`、10 回・500 ms）とは別の定数にする。ADR: なし（定数の値のため）。
3.  **PR を 3 つに分け、実地の確かめは PR-3 の中で行う（2026-11-17、人が選んだ）。** 設計書 8 章の 1 を PR-1、2 を PR-2、3〜5 を PR-3 にする。8 章の最後の段落のとおり、出力の形を変える `Row.Running` の型の変更、`Protocol`、`/chief` の定義は同じ PR（PR-3）に置く。実地の確かめは、0088・0092 と同じく、人が chief の worktree を PR-3 のブランチの先頭（detached）にしてそこから `make install-chief` を行い、確かめてからマージする（0092 計画書の決定 12）。ADR: なし（PR の切り方はこのタスクの中で閉じるため）。
4.  **文書の規則と進み具合の関数は、実行中のコマンドを `Row` とは別の引数で受け取る。** PR-1 と PR-2 では `Row.Running` を今の `*SendRecord` のまま残し、出力の形を変えないためである（決定 3）。PR-3 の行の仕上げが、`Row.Running` から実行中のコマンドを渡す。急ぎの度合いの対応表は `Row` だけを読む。ADR: なし（関数の形のため）。
5.  **実行中のセッションの記録のサンプルは、PR-1 の最初に、`/runplan` のセッションがこの計算機の既存の記録から読み取りだけで写して作る。** 設計書 7.1 は、計画の最初にサンプルを採るとしている。写す元は `~/.claude/projects/` の下の、コマンドが走っている途中の記録と、chief が答えを中継した後の記録である。採取した人を特定できる値は、`internal/chief/testdata/README.md` の「置き換えたもの」の規則で置き換える。中継の後の記録が見つからないときは、人に頼み、採取用の workspace（0088 で使った `chief-poc`）で作ってもらう。設計書の前提（すべての行が改行で終わる、どの行も採取した種類の origin を持つ、`readLastHumanInput` が読める）は、置き換えて削る前の元の記録のまま、リポジトリの外で確かめる。削った後のサンプルでは、削る作業で前提が成り立ってしまいうるためである。確かめの出力（行の数、見つかった origin の種類、読み取りの結果）を §7 の「PR-1 の実行の記録」に書く。前提と違うときは、そこで止まって人と決め直す（requirements_process.md の「Editing an approved document」）。ADR: なし。
6.  **0092 の確かめは、PR-1 と PR-3 の作業の最初に `/runplan` が行い、両方の PR のマージの前に人が行う。** 設計書 3.12 は、急ぎの度合いの対応表を実装する PR の前と、そのマージの直前に確かめるとしている。本計画では、対応表は PR-1 で作り、`Row.Idle` と並べ方への効き目は PR-3 の配線で生じるので、両方の PR で確かめる。確かめ方は、`git fetch origin main` の後に `git grep -n PendingRemoval origin/main -- 'internal/chief/*.go'` を走らせ、Go のファイルだけを探す（PR-3 で、`internal/chief/testdata/` の記録のサンプルがこのコマンドの文を引いて当たったため。§7 の「PR-3 の実行の記録」）。終了コード 1（見つからない）なら 0092 の削除の提案はまだない（0092 設計書 3.6 の `PendingRemoval`）。終了コードが 0 でも 1 でもないときは、ないとはみなさず、止まって人に示す。`61a16bb3` では終了コード 1 だった。見つかったときは、その PR で次の 3 つを足し、AC-05 を確かめる（設計書 3.12）。(1) 削除の提案がある行を `Row.Idle` に当てない。(2) 急ぎの度合いの対応表の順 3 の前に「あとで」・人の行を足す。(3) 文書の規則の規則 2 を足す。マージの前の確かめは、`/runplan` が PR-1・PR-3 の説明に、人がマージの前に行う項目として書く（ステップ 1-7・3-11）。PR-1 で見つかったときは (1) が行の並びを変えるので、§1.2 の「PR-1 と PR-2 は、道具の出力と表の並びを変えない」は、その場合に限り PR-1 に当てはまらない。ADR: なし。
7.  **計画書が `approved` で、マージしていない最初の PR の番号を決められないときは、文書の規則の規則 10（「要確認」）とする。** `ParsePlan` の出力でこれに当たるのは、すべての PR が `Merged` なのに、`Created` でない PR があるため `Plan.Done`（`internal/chief/task.go:693-703`）が偽になる計画書である。AC-07 の規則 7 は N を求めるので、N のないまま「PR-N」と示さず、要確認の側に倒す。ADR: なし（AC-07 の読み方の細部のため）。
8.  **送信の記録を読めないことは、`problems` に足さず、`running_unknown` の `unreadable` にだけ出す。** 今の `addRecords` は、「実行中」の行の `LastSend` が失敗すると、`problems` に「the records of … cannot be read」を足す（`internal/chief/snapshot.go:206-209`、`internal/chief/snapshot.go:135-137`）。設計書 3.5 の「失敗のとき」（申し送りの H-02）は、表示のための読み取りの失敗が `problems` を変えないとしている。送信の停止の記録を読めないことは、今までどおり `problems` に出す。ADR: なし（設計書の決定の範囲のため）。
9.  **0092 への申し送り（H-09・H-10）は、人が 0092 のタスクに直接渡す（2026-11-17、人が選んだ）。** 0092 の 3 文書はすでに `approved` で、H-09・H-10 を取り込んでいない。`/runplan 0092` は計画書を読み、`design_handoff.md` は読まない。0093 の計画書は 0092 の計画書を直さず、0093 の側の手順（決定 1・6）だけを持つ。0093 が先に入り、0092 が H-09 を取り込まないまま削除の提案を実装すると、AC-05 は確かめられない。そのときは §7 の AC-05 の行のとおり、人に示す。ADR: なし（タスクの間の受け渡しのため）。
10. **セッションの記録の origin の `kind` に `auto-continuation` を足し、人でない kind として受け付ける（2026-11-17、人が選んだ）。** ステップ 1-1 で、決定 5 の前提「どの行も採取した種類の origin を持つ」を元の記録で確かめたところ、この計算機の 612 ファイルのうち 12 ファイルに `kind` が `auto-continuation` の行があった（使用量の上限が解けた後の続行の入力）。ほかの前提（すべての行が改行で終わる 612 ファイル中 612、すべての行が JSON として読める）は成り立った。決定 5 のとおり止まって人と決め直し、設計書 1.2.1 のとおり、`screen_transcript.go` の人でない kind の一覧（`otherOrigins`）に足し、PR-1 のステップ 1-1 に含める。ADR: なし（設計書 1.2.1 と同じ）。
11. **人が引数なしで打ったコマンドは、実行中のコマンドとして示されない既知の制限とする（2026-11-17、人が選んだ）。** PR のレビューで、人が引数なしで打ったコマンド（`/runpr` など）の記録は `<command-args>` の要素を持たず、今の `readLastHumanInput` が「そのほかの人の入力」と読むことが分かった（5 つのコマンドの人の入力 599 件のうち 53 件）。受け付ける形は広げず、設計書 3.5 の「残る制限」に足し、PR-3 の手引き（ステップ 3-9）に書く。ADR: なし（表示だけの制限のため）。

### 1.3.1. 実装の際に決めたこと（2026-11-17）

-   **ステップ 3-12 の最初の実地の確かめを受けて、状況の表の形と実行中のコマンドの示し方を変えるステップ 3-13 を PR-3 に足し、その後に実地の確かめをやり直す。** 8 つの列の表が pane の幅に収まらず縦に並び、また後から自由な文で指示を足したセッションで、何を実行しているかが示されなかったためである。決定と理由は要件書の「実装の際に決めたこと（2026-11-17）」にある。`Protocol` は、この PR で上げた値（決定 1）のままとし、この変更のためにもう 1 つは上げない（設計書 1.2.1）。人が選んだ（2026-11-17）。ADR: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する（要件書の同じ項）。

### 1.4. 既存コード調査結果

| 領域 | 今あるもの | 変えること |
|---|---|---|
| 次のコマンドの規則 | `decideNext`（`internal/chief/state.go:273-309`）の `switch` が、文書の状態から次のコマンドを決める。`nextFromPlan`（`:314-342`）が計画書から決める | 文書の判定を文書の段の関数に取り出し、`decideNext` はその段を読む。振る舞いは変えず、`TestNextCommandRules`（`internal/chief/snapshot_test.go:200`）を変えずに通す。PR-1 |
| `classify` | `internal/chief/state.go:382-445`。`Idle` は `:434`（detached で fetch が成功）と `:442`（次のコマンドが `/mkreq`）で付く。文書を読めないことは `Unreadable` の文字列「task documents on origin/main」だけで表す（`internal/chief/snapshot.go:259-265`） | `readWorkspace` が文書を読めないことを真偽値でも残し、`classify` が `positionFacts` を `Row` に残す（設計書 3.1・3.3）。PR-1 |
| 計画書の PR | `Plan.PRs` は `Target`・`Created`・`Merged` を持つ（`internal/chief/task.go:483-504`）。`Plan.Done`（`:693-703`）は、すべての PR が作られてマージされたことを見る。単一の PR の計画書の `Target` は `single`（`:351`） | 文書の規則が、`Merged` でない最初の PR の番号を読む。PR-1 |
| 計画書のサンプル | `internal/chief/testdata/plans/` に 26 個。最初の PR の前（`marker-real-forms.md`）、途中の PR（`marker-opus.md` など）、すべてマージ（`marker-all-merged.md`）、単一の PR（`single-recorded.md`）がある。PR を作成したがマージしていない状態のサンプルはない（`61a16bb3` で、各ファイルの `- [x] PR を作成した` と `- [x] PR がマージされた` の数がどれも等しかった） | PR を作成したがマージしていない状態のサンプルを足す。PR-1 |
| マージの確認の並べ方 | `mergeOnlyRank`（`internal/chief/snapshot.go:308-313`）が `Row.MergeQuestion` と `Questions` を読む。`Row.MergeQuestion` は `classify` が付け（`internal/chief/state.go:395-397`）、出力の `merge_question`（`internal/chief/output.go:35`・`:193`）と `snapshot_test.go:60`・`:65` が読む | 「すべての質問がマージの確認」を 1 つの関数に取り出し、`mergeOnlyRank` と進み具合が使う（PR-1）。`Row.MergeQuestion` と `merge_question` を外す（PR-3） |
| 送信の記録 | `SendRecord`（`internal/chief/records.go:658-667`）は送ったコマンドを文字列 `Command` だけで持つ。開始の記録を書くのは `sendContext.run`（`internal/chief/send.go:461`）と中継（`internal/chief/relay.go:250`）の 2 か所。終わりの記録（`sendEnd`、`:670-675`）の結果を読む関数はない。`LastSend`（`:824-839`）は中継を除いた最後の開始の記録を返す。知らない欄を持つ記録は読まない（`:158-172`） | `Kind`・`Arg` の欄を足し、`sendCommand`（`internal/chief/send.go:399`）の経路だけが書く。送信の終わりの結果を読む関数を足す。PR-2 |
| コマンドの種類 | `paneinput.CommandKind`（`internal/chief/paneinput/paneinput.go:92-107`）と `Command`（`:288-294`）は、種類と引数を外へ出さない。引数の型は `ParseTaskID`（`:37`）、`ParseIssueNumber`（`:66`）、`fixprguard.ParsePRNumber`（`internal/fixprguard/types.go:134`） | 種類の名前の表と、`Command` の種類と検証した引数の文字列を読む関数を足す。PR-2 |
| セッションの記録の読み取り | `transcriptPath`（`internal/chief/transcript.go:53-69`）、`readTranscript`（`:84-96`、上限 `resetTranscriptLimit` 64 MiB は `:30`）、`readLastHumanInput`（`internal/chief/screen_transcript.go:170-189`）。改行で終わらない最後の行は JSON として読めず、`HumanInputUnreadable` になる。リセットの確かめ（`internal/chief/reset.go:51-73`）がこれらを使う。サンプルは `claude-human-input.jsonl`・`claude-queued-input.jsonl`・`claude-clear-model.jsonl` の 3 つ | 改行で終わるまで読み直す読み取りと、人の最後の入力を `RunningCommand` で返す入口を足す。`readLastHumanInput` とリセットの確かめは変えない。PR-2 |
| 「実行中」の行 | `addRecords`（`internal/chief/snapshot.go:169-216`）が、`working` の pane への `LastSend` を `Row.Running`（`*SendRecord`、`internal/chief/state.go:131-133`）に入れる。テストは `TestRunningRowShowsTheLastSend`（`snapshot_test.go:537`）、`relay_test.go:471-473`、`autosend_test.go:227` | 実行中のコマンドの決定に置き換え、`Row.Running` を `*RunningShown` にする（設計書 3.1・3.5）。3 つのテストを直す（設計書 3.11）。PR-3 |
| 自動送信の周 | `applyAutoSends`（`internal/chief/tick.go:79-104`）が、送った行を「実行中」にし、`:95` で `SendRecord` を組み立てる。送信の停止は読み直さない | 提案のコマンドから実行中のコマンドを作り、入力を打った workspace の送信の停止を読み直す（設計書 1.2・3.5）。PR-3 |
| 並べ方と行の仕上げ | `finishSnapshot`（`internal/chief/snapshot.go:148-160`）が人の判断を待つものを並べ、`sortRows`（`:339-354`）が `idleRank` → 状態の順 → `mergeOnlyRank` → 時刻 → パスで並べる | 人の判断を待つものを並べる前に行の仕上げを呼び、`sortRows` の 2 番目に急ぎの度合いを足す（設計書 3.6・3.7）。PR-3 |
| 出力 | `RowOutput`（`internal/chief/output.go:27-41`）、`RunningOutput`（`:236-242`）、`StopOutput`（`:245-249`）、`Snapshot.Output`（`:174-232`）。範囲の外の値は `unknownName`（`internal/chief/records.go:688`）。`Protocol` は `internal/chief/records.go:28` | 設計書 3.8 のとおりにする。`Protocol` を決定 1 の値にする。PR-3 |
| `/chief` の定義と手引き | `--protocol 1` は `.claude/commands/chief.md` の 97・106・228・232・238・248・250・254・307 行と、`docs/dev/guides/chief_guide.md:28`（0092 計画書 §1.4 と同じ箇所。`61a16bb3` で `rg -n -- '--protocol 1' .claude/commands docs/dev/guides/chief_guide.md` と `grep -n プロトコル docs/dev/guides/chief_guide.md` で確かめた）。表の示し方は `.claude/commands/chief.md:124-145` | 設計書 3.9・3.10 のとおりにする。PR-3 |
| 文書のガードテスト | `cmd/chief/docs_test.go` の `TestDocumentedProtocolMatches`（`:205`）と `TestGuideValuesMatchTool`（`:405`）が番号を定数と比べる | 値の対応表のテストを足す（設計書 7.2）。番号のテストは変えない。PR-3 |
| パッケージの一覧 | `docs/dev/conventions/package_reference.md:135-136` が `internal/chief` と `paneinput` の主な exported の名前を挙げる | 新しく exported にした名前を足す。PR-3 |

作り方を構築関数に限る必要がある具体の型は、`RunningCommand` だけである（設計書 3.1）。種類と引数の欄は unexported で、構築関数だけが作る。`internal/chief` の中では複合リテラルで作れる。そこで、出力の文字列を作るメソッドが、種類と引数を構築関数と同じ規則で検証し直し、検証できない引数は落とし、範囲の外の種類と 0 の値は示さない。本計画は、型をほかのパッケージへ移さず、この検証し直しで守る（ステップ 1-4 のテスト）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: サンプル、型、入出力を持たない規則（設計書 8 章の 1）

**変更するファイル:** `internal/chief/testdata/transcript/`・`testdata/plans/`・`testdata/README.md`、`internal/chief/position.go`・`position_test.go`（新規）、`internal/chief/running.go`・`running_test.go`（新規。型と構築関数だけ）、`internal/chief/state.go`、`internal/chief/snapshot.go`、`internal/chief/screen_test.go`、`internal/chief/snapshot_test.go`、本計画（§7 の実行の記録）

-   [x] **ステップ 1-1**: 実行中のセッションの記録のサンプルを採る（§1.3 の決定 5、設計書 7.1）。前提は元の記録で確かめ、結果を §7 に書く。`auto-continuation` の行（§1.3 の決定 10）を含むサンプルも採り、`otherOrigins` に `auto-continuation` を足す。コマンドが走っている途中の記録と、chief が答えを中継した後の記録を `testdata/transcript/` に置き、採った日・版・置き換えを `testdata/README.md` に足す。`screen_test.go` に、各サンプルの期待する結果を確かめるテスト（`TestReadLastHumanInputOfRunningSessions`）を足す。
-   [x] **ステップ 1-2**: 0092 の確かめを行い（§1.3 の決定 6）、結果を §7 の「PR-1 の実行の記録」に書く。見つかったときは、決定 6 の 3 つを、このフェーズのステップ 1-5・1-6 に含める。
-   [x] **ステップ 1-3**: `position.go` に、設計書 3.1 の `Urgency`・`Turn`・`PositionDocument`・`Progress`・`Position` と、文書の段を返す関数を書き、`decideNext` が段を読むようにする（設計書 3.3）。`TestNextCommandRules` は、「要確認」の行の理由を段ごとに確かめる形に強め、取り出しの前に強めた版が通ることを確かめてから取り出す。
-   [x] **ステップ 1-4**: `running.go` に、設計書 3.1 の `RunningCommand` と、種類と引数から作る構築関数を書く。引数は設計書 3.5 の「引数」の表の型で検証する。構築関数は、検証できない引数を落としたことを呼び出し側へ返す（AC-12 のまとめ方が、落とした引数を見分けるため）。出力の文字列を作るメソッドは、§1.4 の最後の段落のとおり検証し直す。テストは、5 つの種類、空の引数、検証できない引数（制御文字、数でない文字列、2 つ目の引数）、0 の値、範囲の外の種類、検証していない引数を複合リテラルで入れた値を確かめる。
-   [x] **ステップ 1-5**: `positionFacts` を `Row` に足し、`classify` がどの経路で返す場合にも残すようにする（設計書 3.1・3.3）。文書を読めないことは、`readWorkspace` が `workspaceFacts` に真偽値で残す。`Row.StopUnreadable` も足す（値を入れるのは PR-3）。テストは、6 つの状態のそれぞれについて、`classify` が fetch の失敗、文書を読めないこと、HEAD の 3 つの状態、命名に合うかどうか、段、計画の完了、PR の番号を残すことを確かめる。
-   [x] **ステップ 1-6**: `position.go` に、急ぎの度合いの対応表（設計書 3.2）、文書の規則（設計書 3.3、§1.3 の決定 4・7）、進み具合（設計書 3.4）の関数を書く。「すべての質問がマージの確認」を 1 つの関数に取り出し、`mergeOnlyRank` もそれを使う。
    -   対応表のテストは、6 つの状態 × 送信の停止（なし・あり・読めない）× `Idle` の有無のすべてと、範囲の外の状態を確かめる。「要確認」の行は `Unreadable` の中身を変えても結果が変わらないことを確かめる（AC-01〜AC-04・AC-06）。
    -   文書の規則のテストは、規則 1・3〜10 のそれぞれだけに当たる事実を作る。命名に合わないブランチの「次を提案」（`/mkreq`）と「レビュー待ち」の行、detached の行（`/mkreq` の実行中とそれ以外）、決定 7 の場合、AC-08 の 2 つの場合を含める。規則 7 は `testdata/plans/` のサンプルを `ParsePlan` で読んで確かめる。PR を作成したがマージしていない状態のサンプルは今なく、`origin/main` では短い間しか現れないので、`marker-opus.md` から PR-2 の「PR を作成した」だけを済ませた形で作り、作ったサンプルであることを `testdata/README.md` に書く（AC-07・AC-08）。
    -   進み具合のテストは、AC-09 の表の各行と、質問のない「質問で停止」の行を確かめる。
-   [x] **ステップ 1-7**: PR の説明に、人がマージの前に行う項目として 0092 の確かめを書く（§1.3 の決定 6）。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。`snapshot` と `tick` の出力と行の並びは変わらない（`snapshot_test.go` と `autosend_test.go` の今のテストが変えずに通る。決定 6 の場合を除く）。

### PR-1 作成ポイント: samples, position types, and pure position rules

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `feat(0093): 実行中の記録のサンプルを採り、急ぎの度合い・文書・進み具合の規則を作る`

**レビュー観点**: 文書の段を取り出した後も次のコマンドと理由が変わらないか / 文書の規則の順（とくに detached と命名に合わないブランチの行）が設計書 3.3 のとおりか / 規則が `Unreadable` の文字列を読まず、宣言した値だけで決めているか / サンプルの前提を元の記録で確かめ、採った人を特定できる値が残っていないか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-3・1-6 は、次のコマンドの規則を変えずに文書の段を取り出し、AC-07 の 10 の規則を順に当てる規則の表で、mkplan2.md の「独立した複雑なステップ（状態機械）」に当たる。Sonnet は、detached の行を「タスクなし」にまとめてしまったり、規則 1 を `Unreadable` の文字列から判定してしまったりしやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/944
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 実行中のコマンドの決定（設計書 8 章の 2）

**変更するファイル:** `internal/chief/paneinput/paneinput.go`・`paneinput_test.go`、`internal/chief/records.go`・`records_test.go`、`internal/chief/send.go`・`send_test.go`、`internal/chief/transcript.go`、`internal/chief/screen_transcript.go`・`screen_test.go`、`internal/chief/running.go`・`running_test.go`、`internal/chief/reset_test.go`、`internal/chief/world_test.go`、`internal/chief/testdata/transcript/`

-   [x] **ステップ 2-1**: `paneinput` に、名前から `CommandKind` を読む表（種類から名前を返す `CommandKind.Name` は、ステップ 1-4 で足した）と、`Command` の種類と検証した引数の文字列を読む関数を足す（設計書 3.1）。テストは、名前と種類の往復、表にない名前、0 の値の `Command` を確かめる。
-   [x] **ステップ 2-2**: `SendRecord` に `Kind`・`Arg` を足し、`sendCommand` の経路だけが書く。引き継ぎの `/runplan` も `runplan` と書く。送信の終わりの記録の結果を読む関数を足す（設計書 3.1・3.5）。テストは、コマンドの送信と引き継ぎの送信（`TestSendTakeover` を広げる）が欄を書くこと、リセットと中継が書かないこと、欄のない記録が読めること、終わりの記録がない送信と結果ごとの読み取りを確かめる。
-   [x] **ステップ 2-3**: `transcript.go` に、改行で終わるまで読み直す読み取り（§1.3 の決定 2）と、上限を超えたことを表す番兵のエラーを足す（`errors.Is` で確かめるため。上限のテストは小さな上限を渡す）。`world_test.go` の偽の `Deps` に、待たない `Sleep` を足す。
    -   テストは、`Deps.Sleep` の差し込みで、3 回目の読み直しで改行で終わる場合と、終わらずに決まらない場合、待ちの間隔が 200 ms であること、待ちが失敗した場合を確かめる。
    -   リセットの確かめについては、最後の行が完全な JSON で改行だけがない記録で、今までどおり読めること（読み直さず、`Sleep` を呼ばないこと）を `reset_test.go` で確かめる。新しい読み取りは、同じ記録で決まらない（`partial_line`）ことを確かめる。
-   [x] **ステップ 2-4**: `screen_transcript.go` に、改行で終わる記録から人の最後の入力を `RunningCommand` で返す入口を書く（設計書 3.5）。`readLastHumanInput` は変えない。テストは、`testdata/transcript/` のすべてのサンプル、5 つの名前とほかの名前、自由な文、人の入力がない記録、読めない記録、検証できない引数を確かめる。
-   [x] **ステップ 2-5**: `running.go` に、pane ごとの決め方（AC-11）、まとめ方（AC-12）、モデル（AC-14）、決まらない理由（`RunningUnknown`）を書く（設計書 3.5・6.1）。行にはまだつながない。テストが確かめるのは、設計書 7.1 の「実行中のコマンド」の OpenCode の 6 つの記録、名前の表にない `kind` を持つ記録、Claude Code のサンプル、2 つの pane が同じコマンド・違うコマンド・片方が決まらない場合、検証できない引数を落とした pane、モデルを示す 3 つの条件、決まらない理由のすべてである。
-   [x] **ステップ 2-6**: PR の説明に次の 2 点を書く。この PR から送信の記録に `kind`・`arg` が入ること。この PR より前の道具に戻すとその記録を読めないので、戻すときは送信の記録を消すこと。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。`snapshot` と `tick` の出力は変わらない。

### PR-2 作成ポイント: running command from transcripts and send records

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `feat(0093): セッションの記録と送信の記録から実行中のコマンドを決める部品を作る`

**レビュー観点**: セッションの記録の受け付ける形を広げていないか（書きかけの行を飛ばさない、記録の文字列をそのまま出さない） / 送信の記録から種類を宣言した欄だけで読み、コマンドの文字列を読み直していないか / 読み直しがリセットの確かめに及んでいないか / まとめ方とモデルの条件

**実装モデル要件**: opus

**判定理由**: ステップ 2-3・2-4 は、Claude Code が書く外部の値（セッションの記録）を表示の値に変える信頼の境界で、ステップ 2-2 は状態のディレクトリの記録の形を変える（mkplan.md の panel-mode の条件のうち security-gate と migration）。Sonnet は、書きかけの最後の行を飛ばして 1 つ前のコマンドを示してしまったり、記録の引数を検証せずに出してしまったりしやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/955
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 配線、出力、`Protocol`、`/chief` の定義と手引き、実地の確かめ（設計書 8 章の 3〜5）

**変更するファイル:** `internal/chief/state.go`・`snapshot.go`・`tick.go`・`output.go`・`records.go`・`position.go`・`running.go` とそのテスト、`internal/chief/output_test.go`（新規）、`internal/chief/relay_test.go`・`autosend_test.go`・`reset_test.go`、`.claude/commands/chief.md`、`docs/dev/guides/chief_guide.md`、`cmd/chief/docs_test.go`、`docs/dev/conventions/package_reference.md`、`docs/tasks/0093_chief_status_sheet/_checks/`（新規）、本計画（§7 の実行の記録）

-   [x] **ステップ 3-1**: 0092 の確かめを行い（§1.3 の決定 6）、結果を §7 の「PR-3 の実行の記録」に書く。見つかったときは、決定 6 の 3 つをこのフェーズで足し、AC-05 を確かめる。
-   [x] **ステップ 3-2**: `Row` を設計書 3.1 のとおりにする（`Running` を `*RunningShown` に、`RunningUnknown`・`Urgency`・`Turn`・`Position` を足し、`MergeQuestion` を外す）。`addRecords` は `Deps` を受け取り、送信の停止を読めないことを `StopUnreadable` に残す。「実行中」の行の処理は実行中のコマンドの決定に置き換え、送信の記録がない workspace でも走らせる（Claude Code の pane は、送信の記録がなくても決まるため）。設計書 3.11 の 3 つのテストと `snapshot_test.go:60`・`:65` を直す。
    -   テストは、送信の記録を読めないときに `problems` に足されず `running_unknown` が `unreadable` になることと、送信の停止の記録を読めないときは今までどおり `problems` に出て `StopUnreadable` が真になることの両方を確かめる（§1.3 の決定 8、H-02）。後者は `TestIdleClearedByWhatTheHumanMustSee` の「読めない停止」の場合を広げる。
-   [x] **ステップ 3-3**: `applyAutoSends` を設計書 3.5 の「自動送信の周」のとおりにし、入力を打った workspace の送信の停止を読み直す（設計書 1.2）。テストは次の 3 つを確かめる（AC-03・AC-10）。`TestAutoSendAfterMerge` では、送った行に実行中のコマンドとモデルが示されること。`TestAutoSendFailureAfterInput` では、入力を打った後に送信が失敗した行が、その周から「🔴 すぐ」になること。`TestResetChecksAgain` では、コマンドの後のリセットが失敗した行が、その周から「🔴 すぐ」になること。
-   [x] **ステップ 3-4**: `position.go` に行の仕上げを書き、`finishSnapshot` が人の判断を待つものを並べる前に呼ぶようにする。`sortRows` の 2 番目に急ぎの度合いを足す（設計書 3.6・3.7）。
    -   テストは、送信が止まった「実行中」の行、停止を読めない行、送信が止まった「レビュー待ち」の行が「すぐ」の位置へ移ること、`tick` で送った行の進み具合が「作成中」になることを確かめる。
    -   送信が止まった「次を提案」の行と、止まっていない「レビュー待ち」の行を並べ（`TestPendingOrderIgnoresUrgency`）、行は入れ替わるが人の判断を待つものはレビュー → 提案の順のままであることを確かめる（AC-17）。`TestRowOrderRules`・`TestTakeSnapshotOrder` は変えずに通す。
-   [x] **ステップ 3-5**: 出力を設計書 3.8 のとおりにし、`Protocol` を §1.3 の決定 1 の値にする。値の名前の集合を返す関数を公開する（設計書 7.2）。集合は `Output` が使う名前の関数から、各型の値を 0 から順に `unknown` になるまでたどって作り、`unknown` は含めない。
    -   テストは、`snapshot` と `tick` の JSON が行ごとに `urgency`・`turn`・`position`・`idle`・`running_unknown` を持ち `merge_question` を持たないこと、範囲の外の値が `unknown` になること、質問ごとの `merge` が残ること、偽った送信の記録の `Arg` とセッションの記録の検証できない引数が `running.command` に入らないことを確かめる（AC-13・AC-18・AC-20）。
    -   値の名前の集合の各名前が出力の名前の関数から返り、各型の範囲のすぐ外の値が `unknown` になることを確かめる。
-   [x] **ステップ 3-6**: `.claude/commands/chief.md` を設計書 3.9 のとおりに直す。値から日本語の表示への対応は、ステップ 3-7 のテストが読める 1 つの Markdown の表に置く。
-   [x] **ステップ 3-7**: `cmd/chief/docs_test.go` に、`/chief` の定義の値の対応表が、ステップ 3-5 の集合（`state`・`urgency`・`turn`・`position.document`・`position.progress`）をちょうど持つことを確かめるテストを足す（設計書 7.2）。
-   [x] **ステップ 3-8**: AC-19 の `diff` の確かめのスクリプトを `docs/tasks/0093_chief_status_sheet/_checks/` に shell で書く。merge-base とこの PR のコミットで `.claude/commands/chief.md` の「Under the table」の項目を取り出して比べ、「質問で停止」の行ごとの質問の要点と pane の項目がなくなり、ほかの項目が同じ順で残ることを確かめる。スクリプトの各検査は、少なくとも 1 つの壊し方で失敗することを確かめる。PR のブランチで走らせ、出力と 2 つのコミットの ID を §7 の「PR-3 の実行の記録」に書く（AC-19 の行はそこを指す）。その後に `.claude/commands/chief.md` を変えるコミットを足したときは、走らせ直して書き直す。
-   [x] **ステップ 3-9**: `docs/dev/guides/chief_guide.md` を設計書 3.10 のとおりに直す（既知の制限は、決定 11 の引数なしのコマンドを含む 3 つ）。古い版に戻す手順は、道具と `/chief` の定義を同じコミットから入れることと、送信の記録の新しい欄を古い道具が読めないので記録を消すことを書く。
-   [x] **ステップ 3-10**: `package_reference.md` の `internal/chief` と `internal/chief/paneinput` の行に、このタスクで exported にした名前を足す。
-   [x] **ステップ 3-11**: PR の説明に、人がマージの直前に行う 2 つの項目を書く。0092 の確かめ（§1.3 の決定 6）と、`origin/main` の `Protocol` がこの PR の値と同じなら 1 つ上げること（§1.3 の決定 1）である。`main` はマージで取り込み、rebase しない（0099 の規則。rebase すると、同じ値への書き換えが履歴から消えるため）。上げるときは、人が `/runpr` かこの PR のセッションに、`Protocol` と `/chief` の定義と手引きの番号を直すコミットを頼む。今の `Protocol` のガードテストが、番号の変え残しを見つける。
-   [x] **ステップ 3-12**: 実地の確かめを、人と行う（§1.3 の決定 3）。動いている `/chief` を人が止め、chief の worktree をこの PR のブランチの先頭（detached）にして `make install-chief` を行い、`/chief` と `/chief once` を起動し直す。設計書 7.3 の 5 つの項目を見る。`working` の Claude Code の pane が数個あり、記録が数十 MB ある状態で、1 周の時間を測る。結果と確かめたコミットを §7 の「PR-3 の実行の記録」に書く。その後に `internal/chief` か `.claude/commands/chief.md` を変えるコミットを足したときは、確かめ直す。マージの後、人が chief の worktree を `main` に戻し、`main` から入れ直す。
-   [x] **ステップ 3-13**: ステップ 3-12 の最初の実地の確かめを受けた決定の変更（要件書・設計書の「実装の際に決めたこと（2026-11-17）」）を実装する。`readRunningCommand` は人が最後に打ったタスクのコマンドと、その後の人の入力の有無を返し、`RunningShown.Later` と出力の `running.later_input` に通す。後の入力があればモデルを示さない。`OutputValueNames` を表に示す `urgency`・`position.document`・`position.progress`・`running_unknown` にする。`/chief` の定義の表を 6 つの列（急・名前・タスク・文書・状態・内容）にし、印を「🔴 今」「🟡 後」「🟢 待」に、`running_unknown` を内容の列に示す。手引きの 3.1 と 8 章、ADR-0046、CLAUDE.md の chief の画面の読み取りの例外の範囲の文を合わせる。その後、ステップ 3-12 の実地の確かめをやり直す。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。AC-19 のスクリプトの結果と実地の確かめの結果が §7 にある。

### PR-3 作成ポイント: wiring, output, protocol, chief command, and guide

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11 / 3-12 / 3-13

**推奨タイトル**: `feat(0093): chief の状況の表に急ぎの度合い・番・進行の位置を示し、急ぎの度合いの順に並べる`

**レビュー観点**: 配線（ステップ 3-2〜3-5）を文書と分けて読み、行の仕上げが `applyAutoSends` の後の 1 か所だけにあるか / 表示のための読み取りの失敗が、行の状態・送る判断を変えず、`problems` を決定 8 のとおりにしているか / 出力の形と `Protocol` と `/chief` の定義が同じ PR で合い、マージの直前の `Protocol` の確かめが PR の説明にあるか / `/chief` の定義がセッションに印や並びを決めさせていないか

**実装モデル要件**: opus

**判定理由**: ステップ 3-2〜3-5 は、0088 の決まり（AC-02・AC-03・AC-05）を変える多くのテストの直しと、`Protocol` を上げる移行を同時に持つ（mkplan.md の panel-mode の条件のうち、既存の振る舞いを変えて多くのテストを直す migration）。Sonnet は、テストの期待を新しい振る舞いに合わせるだけで直し、0088 の振る舞いのうち変えないもの（`Idle`、人の判断を待つものの順）を壊しやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
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
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | サンプル、型、文書の段、急ぎの度合いの対応表、文書の規則、進み具合 | sonnet+opus-review | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | コマンドの種類の名前、送信の記録の欄、読み直し、記録の入口、実行中のコマンドの決定 | opus | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11 / 3-12 / 3-13 | 配線、出力、`Protocol`、`/chief` の定義、手引き、文書のガード、AC-19 のスクリプト、実地の確かめ | opus | auto |

どの PR も、前の PR のマージと次の `/runplan` の起動の間に、`/runplan` の外の作業を置かない。PR-1 のサンプルの採取と PR-3 の実地の確かめは、`/runplan` のセッションの中で行う。そのため、自動送信はすべて `auto` とする。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 規則（文書の段、急ぎの度合いの対応表、文書の規則、進み具合）は、事実を直接作る表のテストで確かめる。対応表は、入力の組み合わせのすべてを渡す（設計書 7.1）。
-   **サンプル:** セッションの記録は `testdata/transcript/` の実物から採ったサンプルで、計画書は `testdata/plans/` のサンプルで確かめる。新しいサンプルは `testdata/README.md` の規則で置き換える。サンプルの前提は、削る前の元の記録で確かめる（§1.3 の決定 5）。
-   **結合:** `TakeSnapshot` と `Tick` を、今の偽の Herdr・git・状態のディレクトリ（`internal/chief/world_test.go`・`sim_test.go`）で走らせる。新しい結合のテストの仕組みは作らない（設計書 7.2）。
-   **時刻と待ち:** 読み直しの待ちは `Deps.Sleep` で差し込み、実時間で待たない。`world_test.go` の偽の `Deps` にも待たない `Sleep` を置く（ステップ 2-3）。
-   **境界の値:** 読み直しの回数の前後（3 回目の読み直しで終わる・終わらない）と、記録の上限を超える記録（小さな上限を渡し、番兵のエラーで確かめる）を確かめる。
-   **層を分ける入力:** リセットの確かめと表示のための読み取りの違いは、最後の行が完全な JSON で改行だけがない記録で確かめる（ステップ 2-3）。人の判断を待つものの順が急ぎの度合いに引きずられないことは、行の順と人の判断を待つものの順が食い違う入力で確かめる（ステップ 3-4）。
-   **セキュリティ:** 検証できない引数と偽った記録の値が出力に入らないこと（ステップ 1-4・2-4・3-5、設計書 7.3）。送信の部品の結果が表示の値で変わらないことは、今の送信のテストが変えずに通ることで確かめる。
-   **後方互換:** 0088 の振る舞いのうち変えないもの（次のコマンド、`Idle`、人の判断を待つものの順、リセットの確かめ）は、今のテスト（`TestNextCommandRules`・`TestDetachedRowIdleOnlyWhenEverythingWasRead`・`TestIdleClearedByWhatTheHumanMustSee`・`TestRowOrderRules`・`TestResetChecks`）が変えずに通ることで確かめる。版の互換は持たない（利用者は 1 人。設計書 1.2）。
-   **テストの補助:** `internal/chief` の中だけで使う補助は、今の `internal/chief/test_helpers.go` と `world_test.go`・`sim_test.go` に足す（test_organization.md の Classification B）。ほかのパッケージから使う補助は作らない。
-   **文書のガード:** `cmd/chief/docs_test.go` が、`--protocol` の番号（今のテスト）と、値の対応表（ステップ 3-7）を確かめる。CI の `doc-guard` はすでに `./cmd/chief` と `./internal/chief` を走らせる。`/chief` の定義と手引きの文は、PR のレビューで読む。
-   **`diff` の確かめ:** AC-19 は、`_checks/` の shell のスクリプトで 1 回だけ確かめる（ステップ 3-8）。`make test` と CI は走らせない。
-   **実地の確かめ:** PR-3 で、本物の Herdr で表と周の時間を確かめる（設計書 7.3）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 採ったサンプルが設計書の前提と違う（改行で終わらない行がある、採取していない種類の origin を持つ行がある） | §1.3 の決定 5 のとおり元の記録で確かめ、違えば止まって人と決め直す。PR-1 の最初に採るので、後の PR の作業は無駄にならない |
| 0092 と並行して進み、どちらかの PR が相手の変更を知らずにマージされる | 決定 6 の 0092 の確かめを PR-1・PR-3 の最初とマージの前に行う。`Protocol` は決定 1 のとおり、PR-3 のマージの直前に確かめ直す。0092 の計画書への取り込みは、人が 0092 のタスクに渡す（決定 9） |
| 0094〜0097 のタスク（セッションの記録から質問と報告を読む）が `screen_transcript.go` と `transcript.go` を変え、衝突する | 2026-11-17 の時点では、どれも要件定義か設計の段にある。PR-2 の前に `origin/main` を取り込み、衝突は読み取りの受け付ける形を広げずに解く |
| PR-3 が大きく、レビューの焦点がぼやける | 規則と実行中のコマンドの決定を PR-1・PR-2 に分け、PR-3 には配線と文書だけを残す（決定 3）。レビュー観点で配線を文書と分けて読む |
| 読み直しで周が遅れる | 決定 2 で pane ごとの待ちを最長 0.6 秒にする。PR-3 の実地の確かめで 1 周の時間を測る |
| 絵文字の幅で表の列がずれる | 印を最初の列に置き（設計書 1.2）、PR-3 の実地の確かめで見る。ずれが読みにくければ、PR-3 の中で人と決め直す |
| `make install-chief` の後に、古い定義の `/chief` が `protocol_mismatch` で止まる | PR-3 の実地の確かめで chief の worktree をそのブランチにし、マージの後に `main` から入れ直す（決定 3）。手引きに入れ替えの手順を書く（ステップ 3-9） |
| PR-2 の後に古い道具へ戻すと、送信の記録を読めない | PR-2 の説明（ステップ 2-6）と手引き（ステップ 3-9）に、記録を消す手順を書く |
| 日程: PR-3 の実地の確かめで、人の手が空かない | 実地の確かめは人がいるときにだけ進める。`/runplan` は人を待つだけで、ほかのタスクを妨げない |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
- [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11 / 3-12 / 3-13）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

表のパスの `ic/` は `internal/chief/`、`cc/` は `cmd/chief/` を表す。テストの名前は計画の時点の名前で、実装で名前を変えたときは、この表を同じ PR で直す（編集上の訂正）。`manual` は、`/chief` の定義の文を PR のレビューで読むことと、PR-3 の実地の確かめを指す。AC-22 は、`/chief` の定義の文とセッションがそれに従うことだけで実現するので、`manual` だけで確かめる（文書の自然言語の内容だけを定める AC。requirements_process.md §4）。

| AC | 確かめ | 種類 | ステップ | PR |
|---|---|---|---|---|
| AC-01 | `ic/position_test.go::TestUrgencyTable`（3 つの値のどれか 1 つ）、`ic/output_test.go::TestOutputValueNames`（名前の集合が出力の名前と一致）、`cc/docs_test.go::TestDisplayValuesMatchTool`（`urgency` の値の集合）、定義の印の表示、実地の確かめ | test、static、manual | 1-6・3-5・3-7・3-12・3-13 | PR-1、PR-3 |
| AC-02 | `ic/position_test.go::TestUrgencyTable`（すべての組み合わせがどれかの順に当たる） | test | 1-6 | PR-1 |
| AC-03 | `ic/position_test.go::TestUrgencyTable`（停止あり・読めない）、`ic/autosend_test.go::TestAutoSendFailureAfterInput`・`ic/autosend_test.go::TestResetChecksAgain`（その周から「すぐ」） | test | 1-6・3-3 | PR-1、PR-3 |
| AC-04 | `ic/position_test.go::TestUrgencyTable`（「要確認」は `Idle` のほか「あとで」、`Unreadable` の中身によらない） | test | 1-6 | PR-1 |
| AC-05 | 0092 が先に入ったとき: `ic/position_test.go::TestUrgencyTable` と `ic/snapshot_test.go::TestRowOrderRules` に削除の提案の行を足す。0092 が後のとき: 0092 が確かめる（決定 9）。どちらだったかを §7 の実行の記録に書く。PR-3 のマージの時点でどちらも確かめていなければ、人に示す | test | 1-2・3-1 | PR-1、PR-3 |
| AC-06 | `ic/position_test.go::TestUrgencyTable`（番）、定義が `turn` を示さないこと | test、manual | 1-6・3-13 | PR-1、PR-3 |
| AC-07 | `ic/position_test.go::TestDocumentRule`、`ic/position_test.go::TestClassifyKeepsPositionFacts`、`ic/snapshot_test.go::TestNextCommandRules`（理由まで確かめる形に強めて通る） | test | 1-3・1-5・1-6 | PR-1 |
| AC-08 | `ic/position_test.go::TestDocumentRule`（状態が「要確認」で文書が読める、文書が「要確認」で状態が変わらない） | test | 1-6 | PR-1 |
| AC-09 | `ic/position_test.go::TestProgress` | test | 1-6 | PR-1 |
| AC-10 | `ic/autosend_test.go::TestAutoSendAfterMerge`（送ったコマンドとモデル、進み具合が「作成中」） | test | 3-3・3-4 | PR-3 |
| AC-11 | `ic/running_test.go::TestRunningCommandPerPane`、`ic/screen_test.go::TestReadRunningCommand`、`ic/screen_test.go::TestReadLastHumanInputOfRunningSessions`（新しいサンプル）、`ic/snapshot_test.go::TestRunningRowShowsTheLastSend`（記録のない workspace の Claude Code の pane）、`ic/running_test.go::TestRunningLater`（後の入力） | test | 1-1・2-4・2-5・3-2・3-13 | PR-1、PR-2、PR-3 |
| AC-12 | `ic/running_test.go::TestRunningCommandPanes` | test | 2-5 | PR-2 |
| AC-13 | `ic/running_test.go::TestNewRunningCommand`、`ic/screen_test.go::TestReadRunningCommand`（検証できない引数）、`ic/snapshot_test.go::TestRunningOutputHasNoRawText` | test | 1-4・2-4・3-5 | PR-1、PR-2、PR-3 |
| AC-14 | `ic/running_test.go::TestRunningModel`、`ic/snapshot_test.go::TestRunningRowShowsTheLastSend`（書き直したもの）、`ic/output_test.go::TestRunningOutputLaterInput`、`cc/docs_test.go::TestDisplayValuesMatchTool`（`running_unknown` の値の集合）、定義の「内容」の列 | test、static、manual | 2-5・3-2・3-6・3-13 | PR-2、PR-3 |
| AC-15 | `ic/snapshot_test.go::TestStoppedRowsOrderFirst`、`ic/snapshot_test.go::TestRowOrderRules` | test | 3-4 | PR-3 |
| AC-16 | `ic/snapshot_test.go::TestStoppedRowsOrderFirst`（「すぐ」の中の状態の順）、`ic/snapshot_test.go::TestTakeSnapshotOrder`（変えずに通る） | test | 3-4 | PR-3 |
| AC-17 | `ic/snapshot_test.go::TestPendingOrderIgnoresUrgency`（行は入れ替わり、人の判断を待つものの順は変わらない）、`ic/snapshot_test.go::TestRowOrderRules`・`TestTakeSnapshotOrder`（変えずに通る） | test | 3-4 | PR-3 |
| AC-18 | `ic/snapshot_test.go::TestSnapshotRowOutput`（質問ごとの `merge` が残り、`merge_question` がない）、定義の「内容」の列、実地の確かめ | test、manual | 3-5・3-6・3-12 | PR-3 |
| AC-19 | `docs/tasks/0093_chief_status_sheet/_checks/` の「Under the table」の比較のスクリプト（merge-base と PR のコミットの `.claude/commands/chief.md`）。結果は「PR-3 の実行の記録」。設計書の付録 B の「PR のレビュー」より強い確かめにした | diff | 3-8 | PR-3 |
| AC-20 | `ic/snapshot_test.go::TestSnapshotRowOutput`、`ic/autosend_test.go::TestTickOutputRows`（`tick` の出力） | test | 3-5 | PR-3 |
| AC-21 | `cc/docs_test.go::TestDisplayValuesMatchTool`、`ic/output_test.go::TestOutputValueNames`、定義の 6 つの列と「道具の順と値のまま示す」の文、実地の確かめ | static、test、manual | 3-5・3-6・3-7・3-12・3-13 | PR-3 |
| AC-22 | 定義の「2. One round」が引数のない場合と `once` で共通であること、実地の確かめ（待ち受けの表。`once` で表が見えることは #968 に回した） | manual | 3-6・3-12 | PR-3 |

### PR-1 の実行の記録

**0092 の確かめ（ステップ 1-2、2026-11-17）。** `git fetch origin main` の後に `git grep -n PendingRemoval origin/main -- internal/chief` を走らせた。`origin/main` は `12fdc257` で、終了コードは 1（見つからない）だった。0092 の削除の提案はまだ `main` にない。決定 6 の 3 つは足さず、AC-05 は 0092 が後に入るときの扱い（決定 9）のままとする。マージの前の確かめは、PR の説明に人の項目として書く。

**サンプルの前提の確かめ（ステップ 1-1、2026-11-17、元の記録のまま、リポジトリの外で行った）。** 対象は `~/.claude/projects/*/*.jsonl` の 612 ファイル（Claude Code 2.1.287〜2.1.326。`auto-continuation` の行を持つ 12 ファイルは 2.1.306〜2.1.326）である。

| 前提 | 結果 |
|---|---|
| すべての行が改行で終わる | 612 ファイルのすべてが改行で終わり、どの行も JSON として読めた（読めない行は 0） |
| どの行も採取した種類の origin を持つ | 最上位か `attachment` の `origin` の `kind` は、`task-notification` 3726 行、`human` 2608 行、`peer` 2182 行、`auto-continuation` 12 行だった。`auto-continuation` は採取した種類にないので、前提は成り立たなかった。12 行は 12 ファイルにあり、`/runplan`・`/mkplan`・`/mkarch`・`/runpr` のセッションを含む。人が決め直した（§1.3 の決定 10、2026-11-17） |
| `readLastHumanInput` が読める | 決定 10 の前: 読めない記録は 12 ファイルで、すべて `auto-continuation` の行を持つファイルだった。読めたのは、人の入力なし 37、コマンド 228、そのほか 335。決定 10 の後（`otherOrigins` に足した後）: 読めない記録は 0、人の入力なし 37、コマンド 232、そのほか 343 |

サンプルは次の 2 つである。コマンドが走っている途中の記録（`claude-running-command.jsonl`、`/runplan 0060`、`auto-continuation` の行を含む）と、`AskUserQuestion` の問いに答えた後の記録（`claude-running-after-answer.jsonl`、`/runplan 0093`）である。chief が答えを中継した後の記録は、中継も人が答えた場合も、transcript の上では同じ形（`origin` を持たない `tool_result` の行）なので、後者を中継の後の記録のサンプルとして使う。同じ形であることは、実地の確かめで chief が中継した記録（`chief-poc` の 2026-11-15 のセッション、Claude Code 2.1.325）の中継の直後の行が `origin` を持たない `tool_result` であることで、あとから確かめられた（PR のレビューでの Fable の確かめ）。採取用の workspace での作り直しは要らなかった。どちらも、読み取りの結果は最後のコマンドである（`TestReadLastHumanInputOfRunningSessions`）。

### PR-3 の実行の記録

**0092 の確かめ（ステップ 3-1、2026-11-17）。** `git fetch origin main` の後に `git grep -n PendingRemoval origin/main -- internal/chief` を走らせた。`origin/main` は `d23784ec` で、終了コードは 0 だった。ただし当たったのは `internal/chief/testdata/transcript/claude-synthetic.jsonl` の 1 行だけである。これはタスク 0095 が足したセッションの記録のサンプルで、PR-1 の報告の文がこの確かめのコマンドを引いているために当たった。Go のコードに絞った `git grep -n PendingRemoval origin/main -- 'internal/chief/*.go'` の終了コードは 1（見つからない）だった（この pathspec の `*` は `/` にも当たり、`internal/chief/paneinput/` の Go のファイルも対象に含む）。0092 の削除の提案はまだ `main` にないと判断し、決定 6 の 3 つは足さず、AC-05 は 0092 が後に入るときの扱い（決定 9）のままとする。マージの前の確かめは、PR の説明に人の項目として、Go のコードに絞ったコマンドで書く。

**AC-19 の確かめ（ステップ 3-8・3-13、2026-11-17）。** ステップ 3-13 で、行に収まらないもの（引き継ぎと送信の停止）の行ごとの一覧を表の下の最初に足したので、AC-19 とスクリプトをそれに合わせた。`origin/main` を 2 回目のマージで取り込んだ後に（2026-11-18）、`_checks/ac19_under_the_table.sh` を、merge-base `552d7cd89972087c7de0054171e064d494f8ba77` と、`.claude/commands/chief.md` を最後に変えたコミット（そのマージ）`cfe4da01b2a468e350dd005fdc5ad8628f3d5c52` で走らせた。終了コードは 0 で、出力は次のとおりである。

```text
base 552d7cd89972087c7de0054171e064d494f8ba77 items:
for each row in 質問で停止, each question's gist and its pane;
each question of this session not answered yet (for example, whether to send again after the human switched an OpenCode pane's model), one per line; keep this list yourself across rounds;
each of `problems`.

head cfe4da01b2a468e350dd005fdc5ad8628f3d5c52 items:
for each row, by its 名前, what does not fit on its line: a `takeover` that is set; and, when `stopped` is set, that sends to the workspace are stopped, with its `reason` and `at`, or, when its `unreadable` is true, that sends are stopped and the stop record could not be read;
each question of this session not answered yet (for example, whether to send again after the human switched an OpenCode pane's model), one per line; keep this list yourself across rounds;
each of `problems`.

PASS: only the per-row question list was removed and the per-row list of what does not fit was added first; the other 2 items stay in order
```

スクリプトの各検査は、次の壊し方で失敗することを確かめた（壊した版は一時的なコミットオブジェクトで作り、ブランチには置いていない）。base と head に同じコミットを渡す（質問の一覧が残り、足した一覧が base にもある）、足した一覧のない版（`ef80839d`）を head に渡す、足した一覧を最後に動かす、`problems` の項目を消す。

**実地の確かめ（ステップ 3-12、2026-11-17）。** 1 回目（`d12b817f`）では、8 つの列の表が pane の幅に収まらず、Claude Code が行ごとに項目を縦に並べて示した。また、「実行中」の行の多くで、何を実行しているかが示されなかった（OpenCode の pane で人が打ったコマンド、`/runplan` の後に自由な文の指示を足した Claude Code のセッション）。これを受けて、ステップ 3-13 の決定の変更を行った。2 回目（`53f15510`）では、`/chief` の表は 6 つの列の表として描かれた。次のことを確かめた。
-   急ぎの列の絵文字と 1 文字で、列はずれない。長い値は、セルの中で折り返される。
-   各列の値は、そのときの状況と合っていた。
-   「質問で停止」の行の内容の列に、質問の要点と pane が出た。
-   後から自由な文の指示を足したセッションに `/runplan 0097（後に指示あり）` が、OpenCode の pane に「人が打ったコマンド（OpenCode）」が出た。
-   送信が止まった行は、確かめの間に現れなかった（その位置は `TestStoppedRowsOrderFirst` で確かめている）。
-   1 周の時間: 8 つの workspace（`working` の Claude Code の pane を含む）で、`chief snapshot --protocol 2` は 8 行を返して 9.0 秒（CPU は利用者 1.1 秒、システム 1.4 秒）だった。CPU の時間は 2.5 秒で、残りは `git fetch`・`gh` の呼び出しの待ちである。
-   `/chief once` では、セッションが表を書かずに問い（`AskUserQuestion`）を開き、表が見えなかった。0088 から続く `once` の作りによるもので、今回の列の変更とは関わらないため、人が選んで別の課題（[#968](https://github.com/isseis/yt2column/issues/968)）にした（2026-11-17）。

2 回目の後に、表示の言葉の修正（`385887cd`）と、`origin/main` のマージ（`f4d36f20`、`Protocol` を 3 にした）が `internal/chief` と `chief.md` を変えたので、確かめ直した。3 回目（`83264471`）では、値と言葉は正しく出たが、長い PR の題名を内容の列に入れた行のために、表がまた縦に並んだ。人と試作を見比べて、PR の題名と質問の要点を 20 文字ほどで切ることにした（2026-11-17）。その変更（`b11be4ac`）の後に、4 回目（`67989081`）を行った（2026-11-18）。次のことを確かめた。
-   表は 6 つの列の表として描かれ、縦に並ばなかった。
-   1 周の時間: 6 つの workspace で、`chief snapshot --protocol 3` は 6 行を返して 6.5 秒（CPU は利用者 0.75 秒、システム 0.70 秒）だった。残りは `git fetch`・`gh` の呼び出しの待ちである。読んだ Claude Code の記録のうち大きいものは 5.1 MB・3.8 MB・3.7 MB で、計画の想定の数十 MB には届かなかった。CPU の時間が 1 周の 4 分の 1 ほどなので、記録が 10 倍になっても、読み取りで周が大きく遅れることはないと見ている（記録の上限を超えた pane は `over_cap` として読まない）。

4 回目の後に、`origin/main` の 2 回目のマージ（`cfe4da01`、0097・0092 の PR-7・0099 を取り込んだ）が `internal/chief` と `chief.md` を変えたので、5 回目（`5ea1c71a`、道具を入れ直した）を行った（2026-11-18）。表は 6 つの列の表として描かれ、値はそのときの状況と合っていた。1 周の時間は、6 つの workspace で、`chief snapshot --protocol 3` が 6 行を返して 6.1 秒（CPU は利用者 0.73 秒、システム 0.63 秒）だった。

## 8. 成功の基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-22 の確かめがすべて通る（AC-05 は §7 のとおり）。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。各テストは、確かめる部品を壊して失敗することを確かめてからコミットする。
-   **セキュリティ:** セッションの記録と送信の記録の文字列が、検証を通らずに出力に入らない（設計書 5.2）。送る入力の範囲と送る判断は変わらない。
-   **文書:** `/chief` の定義、chief 利用ガイド、`package_reference.md` が新しい出力と表に合う。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、人が chief の worktree を `main` に戻し、`main` から `make install-chief` を行って `/chief` を起動し直す（§1.3 の決定 3）。
-   0092 が後に入るときは、0092 が H-09 と H-10 を扱う。人が 0092 のタスクに申し送りを渡す（§1.3 の決定 9）。

## 変更履歴

- 2026-11-17 承認（isseis、#926）
- 2026-11-17 セッション中の再承認：セッションの記録の origin の `kind` に `auto-continuation` を人でない kind として足す（isseis、#944）
- 2026-11-17 セッション中の再承認：人が引数なしで打ったコマンドを、実行中のコマンドとして示されない既知の制限とする（isseis、#944）
- 2026-11-17 セッション中の再承認：実地の確かめを受けて、状況の表を 6 つの列にし、印を「🔴 今」「🟡 後」「🟢 待」にし、実行中のコマンドに「（後に指示あり）」と決まらない理由を示すステップ 3-13 を足した（isseis、#未記入）
