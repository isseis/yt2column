# 実装計画書：chief が issue からタスク用の worktree と workspace を作り、タスクが終わったら確かめて消す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-16 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `30bb673c` で確かめた。設計書が確かめたコミット `bc41cf17` から、`internal/chief`・`cmd/chief`・`internal/strictjson`・`internal/cmdrunner`・`internal/privatefile`・`internal/cachelock`・`.claude/settings.json`・`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` は変わっていない（`git diff --stat bc41cf17 30bb673c -- <これらのパス>` の出力が空だった）。そのため、設計書が引くこれらの行番号は今も合う。

用語は、要件書 3 章・6 章、設計書の冒頭、[0088 計画書](../0088_chief_of_staff/03_implementation_plan.md) の冒頭と同じ意味で使う（作った workspace・作る場所・作ったことの記録・パスの workspace・削除の候補・削除の確かめ・呼び出しの記録係・実地の確かめ など）。本計画は、次の用語を加える。

-   **作成の排他:** `create` と `tick` の自動送信が取り合う、状態のディレクトリの下の排他（設計書 3.1、§1.3 の決定 5）。
-   **確かめ用の workspace:** PR-1 の実地の確かめで、人が作って確かめ、確かめた後に人が片付ける使い捨ての worktree と workspace（§1.3 の決定 2）。タスクの作業には使わない。
-   **書き込みの許可:** 呼び出しの記録係で、テストが書き込みの操作ごとに置く応答（§1.3 の決定 7）。置かれていない書き込みは、テストを失敗にする。
-   **書く部品の型:** `TaskSpace`・`WorkspaceMaker`・`WorkspaceCloser`・`RemovalCandidate` の 4 つ。作り方を 1 つに限る型である（設計書 3.2・3.6、§1.3 の決定 11）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、道具 `chief` にサブコマンド `targets`・`create`・`remove` と削除の候補の行と項目を足し、`/chief` の定義、`.claude/settings.json`、CLAUDE.md、chief 利用ガイドを直して、要件書の F-001〜F-007 を満たす。

### 1.2. 実装の原則

-   **読む部品から順に作り、`cmd/chief` の配線は最後の PR に置く。** 実地の確かめ → 読み取りの部品 → 記録と送信の入口 → 作成 → 削除の候補 → 消す実行 → 配線と文書の順に PR を分ける（設計書 8 章、§1.3 の決定 1・3）。途中の PR をマージして道具を入れ直しても、表と送信は変わらない（§1.3 の決定 4）。
-   **Herdr と git へ書くのは、書く部品の型だけにする。** workspace を作るのは `WorkspaceMaker`、閉じるのは `WorkspaceCloser` だけで、どちらも検証済みの値からしか作れない（設計書 3.6、§1.3 の決定 11）。
-   **外部のコマンドは呼び出しの記録係で置き換える。** ユニットテストは herdr・git・gh を本物で呼ばない（0088 計画書の決定 19）。書き込みの呼び出しは、テストが操作ごとに許したものだけが通る（§1.3 の決定 7）。git の振る舞いに頼る読み取りは、一時的なリポジトリの本物の git でも確かめる。本物の Herdr を使うのは、PR-1 と PR-7 の実地の確かめだけである。
-   **時刻・待ち・時間の上限は差し込む。** テストは実時間で待たない。
-   **各テストは壊して失敗を確かめる。** 確かめた結果を commit のメッセージに書く（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-16）

1.  **PR を 7 つに分ける（2026-11-16・17、人が選んだ）。** 設計書 8 章の 3（作成）を、記録と送信の入口（PR-3）と、作る部品と `create`・`targets`（PR-4）に分ける。8 章の 4（削除）を、削除の候補と確かめ（PR-5。読むだけ）と、消す実行（PR-6）に分ける。0088 の送信の経路（`holdWorkspace`）を変える PR と、Herdr へ書く新しい部品の PR を分け、消す操作だけの PR を置いて、それぞれのレビューの焦点を保つためである。8 章から動かすものは次のとおりである。
    -   Herdr の新しい出力の読み取り（8 章の 2）を PR-1 に入れる。採ったサンプルを確かめるテストが要るためである（0088 の PR-1 と同じ）。gh と git の出力のサンプルは PR-1 で採り、それを読む部品は PR-2 で作る。
    -   CLAUDE.md の「Go Idioms」の task reader の例外を広げること（8 章の 5）を、その literal を入れる PR-2 に入れる（0088 計画書の決定 17 と同じ）。
    -   `cmd/chief` にサブコマンドを足すこと（8 章の 3・4）を、PR-7 に入れる（決定 3）。
    -   作成から削除までを続けて走らせる結合テスト（設計書 7.2）を、作成と削除の両方がそろう PR-6 に入れる。

    ADR: なし（PR の分け方はこのタスクの中で閉じるため）。
2.  **PR-1 の実地の確かめでは、拒否される herdr と git の書き込みの呼び出しを人が打つ（2026-11-16、人が選んだ）。** `/runplan` のセッションが、確かめる項目ごとに打つコマンドと保存先を示す。人は、確かめ用の workspace の外のシェル（chief の workspace のシェルの pane など）で打ち、出力をリポジトリの外のファイルに保存する。セッションはそのファイルを読み、採取した人を特定できる情報を置き換えて `internal/chief/testdata/` に置く。理由: `.claude/settings.json` は Claude Code のセッションに `herdr agent start`・`herdr workspace close` を拒んでおり（`.claude/settings.json:11,22`）、PR-7 で `herdr workspace create`・`git worktree add` なども拒む。拒否を一時的に緩める案は、戻し忘れると守りが外れるので採らない。読み取り（`herdr pane list`・`agent get`・`agent read` など）は、セッションが打ってよい。ADR: なし。
3.  **`targets`・`create`・`remove` を `cmd/chief` に足すことと、`Protocol` を `2` に上げることと、`/chief` の定義・手引きを直すことを、同じ PR（PR-7）で行う。** `cmd/chief/docs_test.go:257` の `TestDocumentedInvocationsParse` は、`/chief` の定義が `subcommandFlags`（`cmd/chief/main.go:64`）のすべてのサブコマンドを呼ぶことを求める。`cmd/chief/docs_test.go:405` の `TestGuideValuesMatchTool` は、手引きのサブコマンドの表とプロトコル番号が道具と同じことを求める。`cmd/chief/docs_test.go:205` の `TestDocumentedProtocolMatches` は、定義の `--protocol` が `chief.Protocol` と同じことを求める。そのため、サブコマンドを足す PR と、定義と手引きを直す PR を分けると、間の PR がグリーンゲートを通らない。`internal/chief` の部品は、それより前の PR に置く。設計書 8 章の最後の段落（`Protocol` と `--protocol` を同じ PR で変える）を満たす。ADR: なし。
4.  **途中の PR（PR-1〜PR-6）をマージして道具を入れ直しても、表と送信は変えない。** 削除の候補は `created.json` のある workspace にしか現れず（設計書 3.4.1）、`created.json` を書く `create` は PR-7 まで呼ばれない。PR-5 で足す行の例外も、`created.json` のない workspace では 0088 の規則のままである。例外は 1 つある。PR-4 から、`tick` は自動送信の前に作成の排他を取り、取れない周（排他のディレクトリを作れないときを含む）は自動送信をせず、そのことを出力の問題に示す（設計書 3.1）。`create` が呼ばれない間は、排他がほかに持たれていて取れない周は起こらない。ADR: なし。
5.  **作成の排他は、状態のディレクトリの下のディレクトリ `create` に置く。** 状態のディレクトリの直下の `internal/cachelock` の排他は、ロックの引き継ぎがすでに使っている（`internal/chief/records.go:302`）。`cachelock` はディレクトリごとに決まった名前のファイルを排他にする（`internal/cachelock/cachelock.go:21,77`）ので、作成の排他には別のディレクトリが要る。この名前 `create` は、workspace ごとの記録のディレクトリの名前（64 桁の 16 進。`internal/chief/records.go:589`）と重ならず、`start` の片付け（`internal/chief/records.go:940`）はこのディレクトリに触れない。ディレクトリは `privatefile.MkdirBelow` で作る（0088 計画書の決定 13 と同じ）。手引きの 7 章の記録の表に足す（ステップ 7-4）。ADR: なし。
6.  **時間と長さの値を次のとおりにする（設計書 3.1・3.3.1・3.3.4・3.4.3 が計画に委ねた値）。**
    -   `create` の全体の上限は 7 分、`remove` の全体の上限は 3 分とする。`/chief` の定義は、2 つの呼び出しに Bash の上限 600000 ミリ秒（10 分）を明示して渡す。作成は 2 つのエージェントの起動（各 60 秒まで）と `/mkreq` の送信（0088 の `/clear`・`/model` の確かめを含む）を順に行うので、余裕を持たせた。
    -   `herdr agent start` の `--timeout` は 60 秒（設計書 3.3.4）とし、引数にはミリ秒で `60000` を渡す（Herdr 0.9.1 の `herdr agent` の使い方の `[--timeout MS]`）。その呼び出しの時間の上限は 75 秒とする。Herdr が `--timeout` で返す前に道具が殺さないためである。
    -   題名は、無害化した後の文字（rune）で 200 文字までとする。GitHub の issue の題名の上限は 256 文字だが、表示の 1 行に収まりやすい長さにした。
    -   閉じた workspace の pane が消えるのを、500 ミリ秒ごとに 20 回（10 秒）まで確かめる。

    PR-1 で観測した時間に合わなければ、PR-1 でこの決定の値を直す（定数は、それを使う PR-4・PR-6 で作る）。ADR: なし（定数の値）。
7.  **呼び出しの記録係（`internal/chief/test_helpers.go:207` の `readOnlyCommands`、`internal/chief/test_helpers.go:217` の `recorder`）を次のとおりに広げる。**
    -   読み取りの一覧に、新しい読み取りの呼び出し（`git grep`、`git worktree list`、`git merge-base --is-ancestor`、`herdr pane list`、`herdr pane process-info`）を足す（PR-2）。
    -   `gh api graphql` は、前方一致の読み取りの一覧に入れない。`gh api graphql` は変更（mutation）も送れ、`-X` で方法も変えられるためである。記録係は、道具の 1 つの決まった形（問い合わせの定数と `-F owner`・`-F repo`・`-F n` の欄だけ）の呼び出しだけを読み取りとして通す。`issue.go` のテストで、問い合わせの定数が `query` の操作で `mutation` を含まないことを確かめる（PR-2）。
    -   書き込みの呼び出し（`herdr workspace create`・`tab create`・`tab rename`・`agent start`・`workspace close`、`git worktree add`・`worktree remove`）は、操作ごとの書き込みの許可でだけ通す（PR-3）。`workspace close` と `worktree remove` の許可は、期待する workspace の ID と worktree のパスを持ち、ほかの引数では失敗にする。作成のテストは閉じる・消す操作の許可を置かず、削除のテストは作る・起動する操作の許可を置かない。
    -   この確かめが及ぶのは `internal/chief` のテストである。`cmd/chief/main_test.go` は自分の fake（`cmd/chief/main_test.go:34-114`）を使うので、PR-7 の `targets` のテストで、その fake が書き込みの呼び出しを受けないことを確かめる。

    ADR: なし。
8.  **PR-1 の実地の確かめが設計書の前提と違ったときは、そこで止まって人と決め直す。** 設計書 1.2 の (i)〜(vi) と Herdr の出力の形の確かめの結果が、設計書の想定と違ったときは、`/runplan` が人に示し、決定の変更として扱う（requirements_process.md の「Editing an approved document」）。設計書を直す必要があるときは、設計書を先に直す。例: (vi) の `ask` の規則が auto mode で確認を出さない。(iv) の `workspace close` が `workspace_group_close_required` を返す。(v) で、起動したばかりの Claude Code が、最初の入力の前には transcript のファイルを持たない（0088 の送信は最初の確かめで transcript を読む。`internal/chief/send.go:423`）。または、その画面が、入力を受け付ける状態と読めない。ADR: なし。
9.  **`package_reference.md` の行は、その名前を exported にする PR で直す。** `internal/strictjson` の行は PR-2 で、`internal/chief` の行は PR-2〜PR-6 で、`cmd/chief` の行は PR-7 で直す。`TestPackageReferenceStrictjsonUsers`（`internal/pipeline/strictjson_users_test.go:657`）が確かめるのは使う側のパッケージの一覧だけで、`internal/chief` はすでに挙がっている。ADR: なし。
10. **PR-7 の削除の実地の確かめでは、人が作ったことの記録を手で置いた workspace を使う（2026-11-17、人が選んだ）。** PR-7 で `create` が作る workspace は、新しい未完了のタスクのものなので、削除の候補にならない（AC-23）。完了したタスクの閉じた issue では、`create` が `issue_closed` で拒む（AC-05）。そこで、人が、完了したタスク（0088 の対応 issue #838 など）について、`issue-<N>` の worktree を `origin/main` で detached に作り、それを開いた workspace でエージェントを起動し、`created.json` を手で置く。chief がそれを提案し、確かめ、消すことを確かめる。これは、偽った記録で `issue-<N>` の形の worktree を名指せること（設計書 5.2 の残る危険）を、確かめを通して実際に消すことでもある。ほかの案（chief が作った最初の本物のタスクが完了するまで確かめを延ばす）は、PR-7 のマージまでに削除の実地の確かめがないので採らない。ADR: なし。
11. **書く部品の型は、ゼロ値では何もせず、コンストラクタのほかで作られていないことをガードのテストで確かめる（2026-11-17、人が選んだ）。** 4 つの型は `internal/chief` の `Commands`・`Worktree`・Herdr の読み取りを使うので、`internal/chief` に置く。同じパッケージの中の composite literal で作れる経路が残る。そこで次の 2 つを行う。
    -   どの型も、ゼロ値のメソッドは外部のコマンドを走らせずにエラーを返す（`PaneWriter` と同じ。`internal/chief/herdr.go:444-452`、`internal/chief/herdr_test.go:202`）。
    -   ガードのテストは、`internal/chief` のテストでないソースを Go の構文木で読み、4 つの型の composite literal（ゼロ値の `T{}` を含む）と欄への代入が、設計書 3.6 と §2 のステップが名指すコンストラクタの中にだけあることを確かめる。型を足す PR（PR-2・PR-4・PR-5・PR-6）で、ガードの対象に足す。

    サブパッケージへ移す案は、狭い runner の interface を足す構成の変更が大きいので採らない。ADR: なし。
12. **PR-7 の実地の確かめは、chief の worktree を PR-7 のブランチにして行う。** 人が、動いている `/chief` を止め、chief の worktree を PR-7 のブランチの先頭（detached）にし、そのブランチで `make install-chief` を行ってから、chief の workspace で `/chief` を起動し直す。新しい定義と道具はプロトコル 2 で合い、ほかのタスクの workspace の待ち受けと自動送信も続く。マージの後、人が chief の worktree を `main` に戻し、`main` から入れ直して `/chief` を起動し直す（0088 の PR-7 と同じ順。0088 計画書の §7 の「PR-7 の実行の記録」）。ADR: なし。

### 1.4. 既存コード調査結果

コミット `30bb673c` で確かめた。複数の実装の案が残る箇所はない。

| 領域 | あるもの | 足りないもの・変えること |
|---|---|---|
| 道具の入口 | `cmd/chief/main.go:39-73` のサブコマンドと引数の表、`requireOwner`（`cmd/chief/main.go:270`）、`--timeout` の引数（`cmd/chief/main.go:60`） | `targets`・`create`・`remove` と `--issue` の文字列のままの受け取り（設計書 3.1）。PR-7 |
| プロトコル | `const Protocol = 1`（`internal/chief/records.go:28`）。`/chief` の定義の `--protocol 1` は `.claude/commands/chief.md` の 97・106・228・232・238・248・250・254・307 行、手引きは `docs/dev/guides/chief_guide.md:28` | すべて `2` にする。PR-7 |
| 外部のコマンド | `Commands.herdr` は `herdrTimeout`（10 秒。`internal/chief/runner.go:38`）で固定（`internal/chief/runner.go:71-75`）。`gitLimit` は上限を取る（`internal/chief/runner.go:86`）。`cmdrunner.ExitError` は終了コードと標準出力を持つ（`internal/cmdrunner/runner.go:67-75`） | 上限を取る herdr の呼び出し（`agent start`）。PR-4。gh の終了コード 1 の標準出力（`NOT_FOUND`）は `ExitError.Stdout` から読める（PR-2） |
| Herdr の読み取り | `parseAgentList`・`parseAgentGet`・`parseWorkspaceList`（`internal/chief/herdr.go:221,241,342`）。`herdrResult`（`internal/chief/herdr.go:176`）が `strictjson` で読む。`PaneWriter` は `agent prompt`・`send-keys` だけを書き、コメントは「Herdr へ書く唯一の部品」と言う（`internal/chief/herdr.go:388-452`） | `workspace create`・`tab create`・`tab rename`・`agent start`・`pane list`・`pane process-info`・`workspace close` の応答とエラーの応答の読み取り（PR-1）。`PaneWriter` のコメントを「pane へ入力を送る唯一の部品」に直す（PR-4） |
| 画面のサンプル | 画面のサンプルごとの期待の読み取り `claudeScreenWants`（`internal/chief/screen_test.go:83`）、サンプルの一覧の確かめ `screenRows`・`TestScreenSamplesCoverEveryRow`（`internal/chief/samples_test.go:99,132`）、置き換えの規則の確かめ `TestSamplesFollowReplacementRules` | 起動したばかりの Claude Code の画面のサンプルと、その期待の読み取り（PR-1） |
| 厳密な JSON | `Object.Collect` は名前の完全一致で重なりを拒む（`internal/strictjson/strictjson.go:91-103`）。大文字と小文字だけ違う名前は拒まない | それを拒む新しいメソッド。今の `Collect`・`CollectOnly` は変えない。PR-2 |
| 作る場所 | `Worktree` は `CanonicalWorktree` だけが作る（`internal/chief/records.go:519-548`）。送れない文字の規則 `transcriptDirChars`（`internal/chief/transcript.go:45`）。`Env.Home`（`internal/chief/records.go:68`） | `TaskSpace` と、作る前の正規のパスの作り方（設計書 3.2）。PR-2 |
| issue | `paneinput.ParseIssueNumber`（`internal/chief/paneinput/paneinput.go:66`）と `NewMkreq`（`internal/chief/paneinput/paneinput.go:297`）。無害化 `Sanitize`（`internal/chief/sanitize.go:75`）。外部のコマンドの出力の上限は 4 MiB（`internal/chief/runner.go:48`） | `gh api graphql` の問い合わせと応答の読み取り、応答の 64 KiB の上限、題名の上限。PR-2 |
| git | `GitReader` の `Fetch`・`Branch`・`Head`・`Clean`・`RemoteMain`（`internal/chief/workspace.go:86-280`）。どれも読み取りである。`Clean` を本物の git で確かめるテストはない（`internal/chief/workspace_test.go`） | `worktree list --porcelain`・`merge-base --is-ancestor` と `Clean` の本物の git のテスト（PR-2）。`worktree add --detach` は作成の中で `TaskSpace` を取って（PR-4）、`worktree remove` は `WorkspaceCloser` のメソッドで（PR-6）呼ぶ |
| タスクの文書 | `Tasks.Read` はブランチ名からディレクトリを探す（`internal/chief/task.go:786-855`）。`Plan.Done`（`internal/chief/task.go:693`） | `対応 issue:` の行の読み方と、ディレクトリの名前から読む口。`Tasks.Read` と同じ読み方を使う。PR-2 |
| 記録 | 記録は同じ名前を置き換えない（`internal/chief/records.go:144-157`、`privatefile.Create`）。消すのは `privatefile.Remove`（`internal/privatefile/privatefile.go:485`。`internal/chief/records.go:890` が使う）。記録の名前は `internal/chief/records.go:572-578`、排他は `WorkspaceRecords.Lock`（`internal/chief/records.go:642`）。記録のディレクトリを作らずに読む `existingRecords`（`internal/chief/records.go:615`） | `created.json` の作る・読む・消す、ディレクトリの記録を消す操作（PR-3）、作成の排他（決定 5。PR-4） |
| 送信 | `Send` は `offered` で行の状態を確かめる（`internal/chief/request.go:224-227`）。`holdWorkspace` は自分で排他を取る（`internal/chief/request.go:263-298`）。`sendCommand`（`internal/chief/send.go:399`）は最初の確かめで transcript を読む（`internal/chief/send.go:423`） | すでに持っている排他を受け取る入口 `sendAfterCreate`。`holdWorkspace` の排他を取った後の部分を共有する。PR-3 |
| 周 | `readRound`（`internal/chief/snapshot.go:90-146`）は `gh pr list` も走らせる（`internal/chief/network_test.go:20-55` が確かめる）。`Tick`（`internal/chief/tick.go:27-77`）。`FindTaskWorkspaces` はエージェントの作業ディレクトリから見つける（`internal/chief/workspace.go:183-230`） | 削除の候補の読み取りと行の例外（PR-5）。`tick` の作成の排他（PR-4） |
| 状態と並べ方 | `classify`（`internal/chief/state.go:382`）、`headProblem`（`internal/chief/state.go:256-263`）、`Row`（`internal/chief/state.go:113`）、`kindOrder`（`internal/chief/pending.go:167`）、出力（`internal/chief/output.go`） | 行の例外、`Row.Task` のコメント、`PendingRemoval`、`RemovalProposal` とその出力。PR-5 |
| テストの補助 | 呼び出しの記録係（`internal/chief/test_helpers.go:178-382`）は、読み取りの一覧にない呼び出しを、書き込みの応答 `onPrompt`・`onKeys`（`internal/chief/test_helpers.go:231-235`）のほかは失敗にする（`internal/chief/test_helpers.go:354`）。応答のない呼び出しも失敗にする（`internal/chief/test_helpers.go:368`）。`newTestRepo`（`internal/chief/test_helpers.go:106`）、Herdr と git を模した世界（`internal/chief/world_test.go:79`、`internal/chief/sim_test.go:31`）、呼び出しの場所と種類の確かめ（`internal/chief/network_test.go`） | 決定 7 の拡張。作成と削除を模す世界の拡張（PR-6）。`targets`・`create`・`remove` の呼び出しの場所の確かめ（PR-4・PR-6） |
| 文書のガード | `deniedHerdr` と `TestSettingsDeniesPaneInput`・`TestSettingsAllowsTool`（`cmd/chief/docs_test.go:361-385`）、`settings` の構造体は `Allow`・`Deny` だけを持つ（`cmd/chief/docs_test.go:301-306`）、`TestDocumentedInvocationsParse` はすべての呼び出しが確認なしに許されると言う（`cmd/chief/docs_test.go:250-256`）、`placeholderValues` は `<issue>` を持つ（`cmd/chief/docs_test.go:216-224`） | 拒否に足す規則、`ask` の規則とそのテスト、`TestDocumentedInvocationsParse` の契約の直し。PR-7 |
| 変えないテスト | `TestTakeSnapshotStates`（`internal/chief/snapshot_test.go:51`）、`TestRowOrderRules`（`internal/chief/snapshot_test.go:467`）、`TestDetachedRowIdleOnlyWhenEverythingWasRead`（`internal/chief/snapshot_test.go:508`）、`TestAutoSendDetached`（`internal/chief/autosend_test.go:343`）、`TestMkreqTarget`（`internal/chief/send_test.go:538`）、`TestNextCommandRules`（`internal/chief/snapshot_test.go:200`）、`TestTasksRead`（`internal/chief/task_test.go:292`）、`internal/chief/network_test.go` のテスト | 作ったことの記録のない workspace について変わらずに通る（設計書 3.4.1・3.9） |

-   **書き換える箇所の一覧（PR-7）。** `--protocol 1` を `2` にする箇所は、上の表の「プロトコル」の行がすべてである（`30bb673c` で `rg -n -- '--protocol 1|プロトコル番号' .claude/commands docs/dev/guides/chief_guide.md CLAUDE.md` を走らせた。出力は、表に挙げた 10 行と、別の道具 `codexreview` の `.claude/commands/_lib/local-cli-review.md:131` だけだった。後者は変えない）。`TestDocumentedProtocolMatches` が、変え残しを見つける。
-   **拒否に足す呼び出しを使う今のコマンド。** `1153172b` で `rg -n 'herdr (workspace create|tab create|pane split|pane move|worktree create|worktree open)|git worktree (add|remove|move)' .claude/commands CLAUDE.md docs/dev/guides` を走らせ、出力は空だった。拒否を足しても、今のコマンドの定義と手引きの手順は妨げられない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 実地の確かめ、サンプル、Herdr の新しい出力の読み取り（設計書 8 章の 1、2 のうち Herdr の出力）

**変更するファイル:** `internal/chief/testdata/herdr/`・`testdata/gh/`・`testdata/git/`（新規）・`testdata/screens/`（サンプルを足す）、`internal/chief/testdata/README.md`、`internal/chief/herdr.go`・`herdr_test.go`・`samples_test.go`・`screen_test.go`、本計画

-   [ ] **ステップ 1-1**: 確かめの準備をする（決定 2）。
    -   Herdr・Claude Code・OpenCode・gh の版を記録する。
    -   確かめ用の worktree のパス（`$HOME/.herdr/worktrees/yt2column/` の下の、`issue-` で始まらない名前）を決め、人に、確かめる項目ごとに打つコマンドと出力の保存先を示す。
-   [ ] **ステップ 1-2**: 設計書 1.2 の (i)〜(vi) と、`herdr worktree` を使わないときに Herdr が git で作った worktree をどう扱うかを、確かめ用の workspace で確かめる（設計書 7.3）。
    -   (v) は、起動したばかりの Claude Code の pane で、`herdr agent get` の状態と Herdr のセッション ID、`transcriptPath`（`internal/chief/transcript.go:53`）が求める場所に transcript のファイルがあるか、画面がステップ 1-5 のテストで入力を受け付ける状態と読めるかで確かめる。
    -   (vi) は、chief の worktree の `.claude/settings.local.json` に `ask` の `Bash(chief remove *)` を一時的に置き、chief の pane の許可の設定（auto mode を含む）のまま、新しい Claude Code のセッションで `chief remove` を呼んで、確認が出るかを見る。今の道具は `remove` を知らないので、呼んでも使い方の誤りで何もしない。確かめた後、その設定を消す。
    -   確かめの後、確かめ用の workspace と worktree は人が片付ける。
-   [ ] **ステップ 1-3**: 出力のサンプルを採る。
    -   Herdr: `workspace create`・`tab create`・`tab rename`・`agent start`（成功、時間切れ、エラーの応答）・`agent get`（起動した直後）・`pane list`・`pane process-info`（シェルだけのとき、前面で別のプロセスが動いているとき）・`workspace close` の出力と終了コード、起動したばかりの Claude Code の画面（フォルダの信頼を問う画面が出れば、それも）。
    -   git: `git worktree list --porcelain`（`locked` のある worktree と、ディレクトリのない登録を含む）。
    -   gh: 閉じた issue の `issueOrPullRequest` の応答を、人の承認を得て採る。設計書 1.2 で採った 3 つの応答も、同じ問い合わせで採り直して置く。
-   [ ] **ステップ 1-4**: 採ったものから、採取した人を特定できる情報（利用者の名前、ホームのパス、ホスト名、セッション ID、issue の題名の中身）を置き換える（security.md §7）。置き換えの規則、版、採った日を `testdata/README.md` に書く。機械で確かめられる新しい規則は `samples_test.go` の置き換えの規則の表に足し、確かめられないもの（題名の中身など）は README にレビューで確かめると書く。
-   [ ] **ステップ 1-5**: `herdr.go` に、ステップ 1-3 の Herdr の出力とエラーの応答（標準エラーの `{"error":{"code":…,"message":…}}`）を型に読む部品を足す（設計書 3.6）。`tab create` の応答のタブと pane が、指定した workspace のものであることも読む。知らない値・欠けた項目・重なったメンバーは読めないものとする。テストは、サンプルごとの読み取りと、形の合わない出力を拒むことを確かめる（`TestHerdrRejectsUnknownShapes` と同じ形）。起動したばかりの Claude Code の画面のサンプルを `claudeScreenWants` と `screenRows` に足し、期待の読み取り（入力を受け付ける状態。信頼を問う画面は質問）を確かめる。
-   [ ] **ステップ 1-6**: §7 の「PR-1 の実行の記録」に、版、(i)〜(vi) の結果、出力の形、エージェントの起動にかかった時間、`workspace close` の後に pane が消えるまでの時間を書く。設計書と違う事実が出たら、決定 8 のとおりに止まる。決定 6 の値が観測と合わなければ、ここで直す。

**完了の条件:** ステップ 1-3 の Herdr の出力と画面のサンプルを読むテストが通る（gh と git のサンプルは PR-2 で読む）。§7 に PR-1 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: field checks, samples, and new herdr output readers

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0092): 実地の確かめとサンプルの採取を行い、Herdr の新しい出力の読み取りを作る`

**レビュー観点**: 設計書 1.2 の (i)〜(vi) の結果と、設計書との食い違いの扱い / サンプルに採取した人を特定できる情報が残っていないか / 形の合わない Herdr の出力とエラーの応答を読めないものにしているか / 起動したばかりの Claude Code の画面の読み取り

**実装モデル要件**: opus

**判定理由**: ステップ 1-2・1-3 が、確かめていない外部の振る舞い（エージェントの起動、`workspace close`、`ask` の規則）を本物の Herdr で確かめる実現性の確かめ（Phase 0 型）である。Sonnet で進めると、想定と違う結果を確かめたものとして記録し、設計の誤りを後の PR へ持ち越しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 読み取りの部品（設計書 8 章の 2）

**変更するファイル:** `internal/strictjson/strictjson.go`・`strictjson_test.go`、`internal/chief/taskspace.go`・`issue.go`・`construction_test.go`（新規）とそのテスト、`internal/chief/records.go`（`Worktree` のコメント）、`internal/chief/workspace.go`・`task.go` とそのテスト、`internal/chief/test_helpers.go`、`internal/chief/testdata/requirements/`（新規、要件定義書のサンプル）、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 2-1**: 呼び出しの記録係を、決定 7 の読み取りの分（読み取りの一覧と、`gh api graphql` の決まった形）だけ広げる。
-   [ ] **ステップ 2-2**: `strictjson` に、読むメンバーと大文字と小文字だけが違う名前のメンバーを拒む読み方を、新しいメソッドとして足す（設計書 3.3.1）。今の `Collect`・`CollectOnly` の振る舞いとテストは変えない。テストは、拒む名前、重なり、読まないメンバーの読み飛ばしを確かめる。`package_reference.md` の `internal/strictjson` の行に名前を足す（決定 9）。
-   [ ] **ステップ 2-3**: `taskspace.go` に `TaskSpace`・`NewTaskSpace`・`TaskSpace.Worktree` を書き、`records.go` の `Worktree` の型のコメントに 2 つめの作り方を書く（設計書 3.2）。テストは、番号からパスと名前が決まること、`$HOME` が相対パスのとき、送れない文字を含むとき（リンクを解決した後に含むときを含む）、途中にディレクトリでないものがあるとき、ゼロ値が何も返さないことを確かめる。一時的なディレクトリとシンボリックリンクで確かめる。`construction_test.go` に決定 11 のガードを作り、`TaskSpace` を対象にする。
-   [ ] **ステップ 2-4**: `taskspace.go` に、`herdr pane list` からパスの workspace を求める読み方を書く（設計書 3.3.2）。テストは、パスそのもの・その下・その外（名前の前方だけ一致するパスを含む）の pane、chief の workspace、シェルだけの pane を確かめる。
-   [ ] **ステップ 2-5**: `issue.go` に、`gh api graphql` の `issueOrPullRequest` の問い合わせと応答の読み取りを書く（設計書 3.3.1）。問い合わせの変数は `-F` で渡す（`{owner}`・`{repo}` を gh が埋め、`$n` を数として渡すため。`gh api --help`）。応答は 64 KiB を上限とする。結果は、ゼロ値を読めないものとする列挙の型で返す。題名は無害化して上限で切り、切ったことを返す（決定 6）。テストは、PR-1 のサンプルで 4 つの形を見分けること、設計書 7.1 の拒む形を 1 つずつ拒むこと、64 KiB の境界、題名の上限の境界を確かめる。題名の境界のテストは、日本語でない複数バイトの文字を使う。`--jq` を使わないこと、chief の worktree で `GH_REPO`・`GH_HOST` なしに走ること、問い合わせが `mutation` を含まないことを確かめる（決定 7）。
-   [ ] **ステップ 2-6**: `workspace.go` の `GitReader` に、`worktree list --porcelain` の読み取り（パス、`locked`、ディレクトリのない登録）と `merge-base --is-ancestor`（0・1・それ以外を分ける）を足す。`GitReader` は読み取りだけを持ち、`worktree add`・`worktree remove` は持たない（生のパスで worktree を作るか消す口を作らないため。書き込みは、ステップ 4-2 の `TaskSpace` を取る作成と、ステップ 6-1 の `WorkspaceCloser` が持つ）。`newTestRepo` の一時的なリポジトリの本物の git で、次を確かめる。
    -   PR-1 のサンプルと本物の git の出力で、porcelain を読める。
    -   `merge-base --is-ancestor` の、祖先・祖先でない・ないコミットの 3 つ。
    -   今の `Clean` が、ステージした変更と入れ子のディレクトリの追跡していないファイルを数え、git が無視するファイルを数えない。
-   [ ] **ステップ 2-7**: `task.go` に、`origin/main` の要件定義書の `対応 issue:` の行を 1 回の `git grep` で読み、issue 番号からタスクのディレクトリを探す読み方と、ディレクトリの名前から文書を読む口を書く（設計書 3.4.4）。`git grep` が終了コード 1（合う行がない）を返したときは、行がないものとして読む。`Tasks.Read` とディレクトリの名前から読む口は、同じ文書の読み方を使う。`TestTasksRead` は変えずに通す。テストは次を確かめる。
    -   要件定義書のサンプル（`testdata/requirements/`）で、`（` の中のリンク、リンクでない番号、形の合わないリンク、2 つの行、コードのフェンスの中の行、2 つのタスクが同じ issue を挙げる場合。
    -   `refs/remotes/origin/main` に、glob の深さの要件定義書と、それより深い場所のファイルを置いた一時的なリポジトリで、本物の `git grep` が前者だけを読むこと。
-   [ ] **ステップ 2-8**: CLAUDE.md の「Go Idioms」の task reader の例外の範囲に、要件定義書の `対応 issue: ` の行の頭と、それを区切る全角の `（` を足し、テストが読むサンプルのディレクトリに `testdata/requirements/` を足す（設計書 3.4.4、決定 1）。`package_reference.md` の `internal/chief` の行に、このフェーズで exported にした名前を足す（決定 9）。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。

### PR-2 作成ポイント: readers for the space, the issue, git, and the task of an issue

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8

**推奨タイトル**: `feat(0092): 作る場所、issue、git、issue からタスクを探す読み取りの部品を作る`

**レビュー観点**: issue の応答の拒む形が設計書 3.3.1 の表のとおりか（広く受け付けていないか） / `対応 issue` の数え方が、読めない書き方を、提案が減る向きに倒しているか / `strictjson` の新しい読み方が今の `Collect`・`CollectOnly` を使うほかのパッケージを変えないか / `gh api graphql` を決まった形のほかで読み取りとして通さないか / CLAUDE.md の例外の範囲

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 2-5・2-7 は、信頼できない入力（issue の応答と要件定義書の行）を、拒む形と安全側に倒す規則の多い読み方で読む、独立した危険の高いステップである。Sonnet は、拒むべき形を黙って受け付けるか、読めない書き方を推し量って補いやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 作ったことの記録と、作った後の送信の入口（設計書 8 章の 3 のうち記録と送信）

**変更するファイル:** `internal/chief/records.go`・`records_test.go`、`internal/chief/request.go`・`send.go`・`send_test.go`、`internal/chief/test_helpers.go`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 3-1**: 呼び出しの記録係に、決定 7 の書き込みの許可を足す。`onPrompt` のコメント（「the only write the tool makes」。`internal/chief/test_helpers.go:228`）を直す。記録係そのもののテストを書き、許可のない書き込みがテストを失敗にすること、`workspace close` と `worktree remove` の許可が期待と違う ID とパスで失敗にすること、`gh api graphql` の決まった形でない呼び出しを読み取りとして通さないことを確かめる（使う部品は PR-4・PR-6 で作るので、この PR の `make deadcode` もこのテストで通す）。
-   [ ] **ステップ 3-2**: `records.go` に、作ったことの記録（`created.json`）を作る・読む・消す操作と、workspace ごとの記録のディレクトリから排他のファイルのほかの記録を消す操作を足す（設計書 3.5）。読むときは、形・権限・記録のパスとディレクトリの名前・記録のパスと記録の issue 番号の作る場所の一致を確かめ、合わなければ読めないものとする（AC-24）。テストは、それぞれの合わない記録と、排他を持たずには消せないことを確かめる。
-   [ ] **ステップ 3-3**: `request.go`・`send.go` に、すでに持っている排他を受け取って `/mkreq` を送る入口 `sendAfterCreate` を書く（設計書 3.3.4）。`holdWorkspace` の、排他を取った後の部分（終わりのない送信の停止への変換、送信の停止の確かめ、エージェントの読み直し）を、`holdWorkspace` と共有する。`offered` の確かめだけを省き、送り先と入力の確かめは `sendCommand` のままにする。今の `send_test.go`・`autosend_test.go`・`relay_test.go` のテストは変えずに通す。テストは次を確かめる。
    -   `/clear`・`/model opus`・`/mkreq <N>` をこの順に送る。
    -   送信の停止があれば送らない。
    -   最初の入力の後に確かめが外れれば、送信の停止を作る（AC-18）。
    -   PR-1 の起動したばかりの Claude Code の画面とエージェントの状態で、送る前の確かめを通る。
-   [ ] **ステップ 3-4**: `package_reference.md` の `internal/chief` の行に、このフェーズで exported にした名前を足す。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。

### PR-3 作成ポイント: created records and the send after creation

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4

**推奨タイトル**: `feat(0092): 作ったことの記録と、作った workspace へ /mkreq を送る入口を作る`

**レビュー観点**: `holdWorkspace` の共有で、0088 の `Send`・中継・自動送信の振る舞いが変わらないか / `sendAfterCreate` が `offered` のほかの確かめを省いていないか / 読めない作ったことの記録を読めないものとしているか / 書き込みの許可が操作ごとに分かれているか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 3-3 は、0088 の送信と自動送信が今使う経路（`holdWorkspace`）を分けて共有する、独立した危険の高いステップである。Sonnet は、共有した部分で終わりのない送信の扱いや停止の確かめの順を変え、0088 の送信の守りを崩しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 作る部品、`create`・`targets`、作成の排他（設計書 8 章の 3 のうち作成）

**変更するファイル:** `internal/chief/create.go`・`create_test.go`（新規）、`internal/chief/herdr.go`（`PaneWriter` のコメント）、`internal/chief/runner.go`・`runner_test.go`、`internal/chief/records.go`、`internal/chief/output.go`、`internal/chief/tick.go`・`autosend_test.go`、`internal/chief/network_test.go`、`internal/chief/construction_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 4-1**: `runner.go` に、時間の上限を取る herdr の呼び出しを足す（決定 6）。`create.go` に `WorkspaceMaker` を書く（設計書 3.3.3・3.6）。
    -   `TaskSpace` からだけ作り、`workspace create` の応答の ID だけを、後の `tab rename`・`tab create`・`pane process-info`・`agent start` に使う。`tab create` の応答のタブと pane が自分の workspace のものでなければ、その段の失敗とする。
    -   `agent start` には、名前・種類・pane・`--timeout` のほかの引数を付けない（AC-10）。
    -   ゼロ値は何も走らせない（決定 11）。`construction_test.go` のガードの対象に足す。`PaneWriter` のコメントを「pane へ入力を送る唯一の部品」に直す（設計書 2.2）。
    -   テストは、herdr のすべての書き込みが自分の作った ID だけを持ち、`--session` などの引数を持たないこと（AC-09・AC-13）、`agent start` の時間の上限が 75 秒であること（記録係が記録する上限で確かめる）を確かめる。
-   [ ] **ステップ 4-2**: `create.go` に `Create` を書く（設計書 3.3.2・3.3.3・3.3.4・4.1）。
    -   worktree は、`TaskSpace` を取る unexported の関数で `git worktree add --detach` を呼んで作り、生のパスを取らない。`newTestRepo` の一時的なリポジトリの本物の git で、ブランチを作らないことを確かめる。
    -   確かめの順、作る段、`agent start` のエラーの応答の扱い（`agent get` でエージェントの有無を確かめる）、全体の時間の上限（決定 6）、結果の型（`CreateOutcome` など、設計書 3.6）と、その出力（`output.go`）を書く。
    -   テストは設計書 7.1 の「作成」の項目と 7.4 を受け持つ。各段を 1 つずつ失敗させ、後の段の呼び出しと `/mkreq` がなく、作ったものを示すことを確かめる。結果の出力も確かめる。
    -   `network_test.go` に、`create` が gh を chief の worktree の `api graphql` の決まった形でだけ、git を chief の worktree と作った worktree でだけ走らせることを確かめるテストを足す。
-   [ ] **ステップ 4-3**: `create.go` に `Targets` を書く（設計書 3.3.5）。`readRound` で読み、行が `/mkreq` を提案している workspace を返す。番号の形が違えば `issue_form` を返す。テストは、候補があるとき・ないとき、形の違う番号、記録を作らないこと、書き込みをしないこと、`gh api graphql`（F-002 の issue の読み取り）を走らせないこと（AC-04）、結果の出力を確かめる。
-   [ ] **ステップ 4-4**: 作成の排他を足す（決定 5、設計書 3.1）。`Create` は作り始める前に作成の排他を取る。`tick` は自動送信の前に作成の排他を取り、自動送信を終えるまで持つ。取れない周は自動送信をせず、出力の問題に示す。今の `autosend_test.go` のテストは変えずに通す。テストは次を確かめる。
    -   作成の排他が持たれている周で、自動送信をしない。
    -   `tick` が持つ間に、`Create` が `busy` で拒む。
    -   `tick` を終えた後に、`Create` が排他を取れる。
-   [ ] **ステップ 4-5**: `package_reference.md` の `internal/chief` の行に、このフェーズで exported にした名前を足す。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。

### PR-4 作成ポイント: workspace maker, create, targets, and the creation exclusion

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0092): issue からタスクの worktree と workspace を作り、/mkreq を送る create と targets を作る`

**レビュー観点**: `WorkspaceMaker` が自分の作った ID のほかで Herdr に書かないか / 途中で止まったときに作ったものをすべて示し、`/mkreq` を送らないか / 作成の排他と `tick` の自動送信の順 / ゼロ値とガードのテスト

**実装モデル要件**: opus

**判定理由**: ステップ 4-1・4-2 は、Herdr への書き込みを型で限る安全の境界（設計書 5.2）を作るステップである。Sonnet は、呼び出し側から ID を受け取る口を作るか、途中の失敗の後に後の段を続け、作ってはならないものを作りやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 削除の候補の行と項目、削除の確かめ（設計書 8 章の 4 のうち読み取り）

**変更するファイル:** `internal/chief/removal.go`・`removal_test.go`（新規）、`internal/chief/state.go`・`snapshot.go`・`pending.go`・`output.go` とそのテスト、`internal/chief/autosend_test.go`、`internal/chief/construction_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 5-1**: `removal.go` に、削除の候補の読み取りを書く（設計書 3.4.1）。
    -   タスクの workspace ごとに、作ったことの記録（記録のディレクトリを作らずに読む）、パスの workspace がちょうど 1 つあり、それがこの workspace であること、記録の issue 番号のタスクが `origin/main` にちょうど 1 つあり `Plan.Done` であることを確かめ、`RemovalCandidate` を作る。
    -   読める作ったことの記録のない workspace については、外部のコマンドを 1 つも走らせない。作ったことの記録が 1 つもない周では、`git grep` も `herdr pane list` も走らせない。そのため、§1.4 の「変えないテスト」は、記録係に新しい応答を足さずに通る。
    -   `RemovalCandidate` のゼロ値は何も通さず、`construction_test.go` のガードの対象に足す（決定 11）。
    -   テストは、AC-23 の条件を 1 つずつ外した入力で候補を作らないこと、AC-24 の各場合、偽った記録（ほかの worktree のパス、ほかの issue の作る場所）で候補を作らないことを確かめる。
-   [ ] **ステップ 5-2**: `removal.go` に、削除の確かめ（設計書 3.4.2 の表のすべての行）を、確かめの結果の一覧を返す関数として書く。読めないものは通らないものとし、その名前を添える。テストは、各確かめを 1 つずつ外したときと読めないときに、その確かめだけが通らないことを確かめる。
-   [ ] **ステップ 5-3**: 行の例外と項目を足す（設計書 3.4.1・3.6）。
    -   AC-20 の条件をすべて満たす行を「タスク完了」・記録の issue のタスク・「削除を提案」にし、`Row` に削除の候補の行の印を足し、`Row.Task` のコメントを直す（設計書 2.2）。
    -   `PendingRemoval` を `kindOrder` の最後に足し、`RemovalProposal` と項目の識別（workspace のパスと issue 番号から作る）、出力（`output.go`）を足す。
    -   テストは、AC-20 の条件だけを 1 つずつ外したときに、行は 0088 の規則のままだが項目は作られること、並べ方、出力を確かめる。§1.4 の「変えないテスト」は変えずに通す。
-   [ ] **ステップ 5-4**: 削除の候補のある周で、`tick` の自動送信が `removal` の項目を送信の対象にしないことを確かめるテストを足す（AC-26）。テストの周は、自動送信の条件を満たすほかの workspace を持ち、その pane への `agent prompt` だけを許す。`workspace close` と `worktree remove` の許可は置かず、周の後に `created.json` とほかの記録がディスクに残ることを確かめる。`snapshot` と `targets` についても、記録が残ることを確かめる。
-   [ ] **ステップ 5-5**: `package_reference.md` の `internal/chief` の行に、このフェーズで exported にした名前を足す。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。

### PR-5 作成ポイント: removal candidates, rows, items, and checks

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5

**推奨タイトル**: `feat(0092): 完了したタスクの作った workspace を削除の候補として示し、削除の確かめを作る`

**レビュー観点**: 偽った作ったことの記録が、`issue-<N>` の形のほかの worktree を名指せないか / 行の例外と新しい外部のコマンドが、作ったことの記録のない workspace に及ばないか / 削除の確かめが読めないものを通らないものとしているか / 自動送信のある周で削除の項目に触れないか

**実装モデル要件**: opus

**判定理由**: ステップ 5-1・5-2 は、削除の可否を決める確かめと、偽った記録の拒否（設計書 7.4）を持つ安全の境界である。Sonnet は、読めない状態を通るものとして扱うか、記録のパスと issue の作る場所の一致を確かめ漏らしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: 消す実行（設計書 8 章の 4 のうち削除）

**変更するファイル:** `internal/chief/removal.go`・`removal_test.go`、`internal/chief/output.go`、`internal/chief/network_test.go`、`internal/chief/world_test.go`・`sim_test.go`、`internal/chief/construction_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 6-1**: `removal.go` に `WorkspaceCloser` を書く（設計書 3.6）。削除の確かめをすべて通った `RemovalCandidate` からだけ作り、`herdr workspace close <ID>` を `--group` なしで呼ぶ。`git worktree remove`（`--force` なし）も `WorkspaceCloser` のメソッドだけが、候補の worktree のパスで呼ぶ。`newTestRepo` の一時的なリポジトリの本物の git で、変更のある worktree と追跡していないファイルのある worktree を git が拒むこと、detached の worktree でブランチを作ってから detached に戻した worktree を消してもそのブランチが残ることを確かめる。ゼロ値は何も走らせず、`construction_test.go` のガードの対象に足す（決定 11）。テストは、確かめが 1 つでも通らない候補から作れないこと、ゼロ値が herdr を呼ばないこと、呼び出しの引数を確かめる。
-   [ ] **ステップ 6-2**: `removal.go` に `Remove` を書く（設計書 3.4.3・4.1）。
    -   `--workspace` を `locate`（`internal/chief/request.go:134`）で今のタスクの workspace と比べる。排他、fetch、確かめ、閉じる直前の読み直し、閉じる、pane が消えるのを待つ（決定 6）、HEAD の確かめ直し、`git worktree remove`、記録を消す（作ったことの記録を最後に）の順に行う。
    -   全体の時間の上限と、結果の型（`RemoveOutcome` など）とその出力を書く。
    -   テストは設計書 7.1 の「削除」の項目を受け持つ。`network_test.go` に、`remove` が git を chief の worktree と消す worktree でだけ走らせることを確かめるテストを足す。結果の出力も確かめる。そのほか、次を確かめる。
        -   各段を失敗させ、残ったものを示し、消し直さない。
        -   確かめの後に pane の集まりかエージェントの状態が変われば閉じない（`changed`）。確かめの後に HEAD が動けば消さない（`recheck_head`）。
        -   `--force` と `--group` を渡さない。
        -   一致しない `--workspace` のパスで git を走らせない。
-   [ ] **ステップ 6-3**: 結合テストを書く（設計書 7.2）。Herdr と git を模した世界を広げ、次を続けて走らせる。
    -   `Create` から `/mkreq` の送信まで。
    -   `/runpr` の後の detached の worktree と完了したタスクの文書で、削除の候補になるまで。
    -   `Remove` で消して表から消え、次の周で二度と提案されないまで（AC-33）。
-   [ ] **ステップ 6-4**: `package_reference.md` の `internal/chief` の行に、このフェーズで exported にした名前を足す。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。

### PR-6 作成ポイント: removal of a finished workspace and worktree

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4

**推奨タイトル**: `feat(0092): 確かめを通った作った workspace を閉じて worktree を消す部品を作る`

**レビュー観点**: 閉じる直前の読み直しと、消す直前の HEAD の確かめ直し / `--force`・`--group` を渡さないこと / 途中で止まったときに残ったものをすべて示し、消し直さないか / 結合テストで、消した workspace が次の周で提案されないか

**実装モデル要件**: opus

**判定理由**: ステップ 6-1・6-2 は、作業を失わせうる操作（workspace を閉じ、worktree を消す）の安全の境界で、ステップ 6-3 は Herdr と git を模した世界を広げる重い結合テストである。Sonnet は、確かめ直しを省くか、失敗の後に消し直す経路を作りやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: 配線、`/chief` の定義、設定、文書、最後の実地の確かめ（設計書 8 章の 3〜5）

**変更するファイル:** `cmd/chief/main.go`・`main_test.go`・`docs_test.go`、`internal/chief/records.go`（`Protocol`）、`.claude/commands/chief.md`、`.claude/settings.json`、`CLAUDE.md`、`docs/dev/guides/chief_guide.md`、`docs/dev/conventions/package_reference.md`、本計画

-   [ ] **ステップ 7-1**: `cmd/chief/main.go` に `targets --issue <N>`・`create --issue <N>`・`remove --workspace <W>` を足し、パッケージのコメントと使い方の文を直す（設計書 3.1）。`targets` と `create` は `--issue` を文字列のまま道具へ渡す。`Protocol` を `2` にする（決定 3）。テストは次を確かめる。
    -   引数の表、3 つのサブコマンドがロックの持ち主を確かめること、出力と終了コード。
    -   形の違う `--issue` が、終了コード 2 ではなく、終了コード 0 と `issue_form` になること（AC-06）。
    -   `targets` で、`cmd/chief` の fake が書き込みの呼び出しを受けないこと（決定 7）。
-   [ ] **ステップ 7-2**: `.claude/commands/chief.md` を直す（設計書 3.7）。新しいタスクを始める入口と流れ、作成の結果、削除の提案と人の確認の問い、削除の結果を書き、`create` と `remove` に Bash の上限を渡す（決定 6）。すべての `--protocol` を `2` にする（§1.4）。
-   [ ] **ステップ 7-3**: `.claude/settings.json` の `deny` と `ask` を設計書 3.8 のとおりに直し、`cmd/chief/docs_test.go` を直す（設計書 3.9）。
    -   `deniedHerdr` に herdr の規則を足す。git の規則を確かめるテストを足し、`allow` に `Bash(git worktree` で始まる規則がないことも確かめる。
    -   `settings` の構造体に `Ask` を足し、`ask` が `Bash(chief remove *)` を持つことを確かめるテストを足す。
    -   `TestDocumentedInvocationsParse` の契約（すべての呼び出しが確認なしに許される）を、`remove` は `ask` の確認を出すと直す。定義の `remove` の呼び出しが `chief remove` で始まることを確かめる。
-   [ ] **ステップ 7-4**: CLAUDE.md の「Tool Execution Safety」（issue の読み取り）と「Claude Commands」の `/chief` の項、chief 利用ガイドの 1〜8 章（設計書 3.8。7 章の記録の表には作成の排他のディレクトリ `create` も足す。決定 5）、`package_reference.md` の `cmd/chief` の行を直す。
-   [ ] **ステップ 7-5**: 最後の実地の確かめをする（設計書 7.3、決定 10・12）。人が、決定 12 のとおり chief の worktree をこのブランチにして入れ直し、`/chief` で次を行う。結果を §7 の「PR-7 の実行の記録」に書く。マージの後に `main` へ戻すことも、決定 12 のとおりである。
    -   新しいタスクを始めること（新しい workspace と既存の workspace）。新しく作る issue は人が選ぶ。
    -   作成の拒否（存在しない番号、閉じた issue、PR、すでにあるパス）。
    -   削除の提案、削除、削除の拒否。決定 10 の workspace を使う。拒否は、その workspace に追跡していないファイルを置くなどして起こす。
    -   `once` の問いで、示せる数を超える項目があるときに、「新しいタスクを始める」を名前で選べること。
    -   作成で作った workspace は、確かめの後に人が端末で片付けるか、そのタスクに使う。

**完了の条件:** 各ステップのテストが通り、§7 に PR-7 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-7 作成ポイント: subcommands, protocol 2, chief command, settings, and documents

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4 / 7-5

**推奨タイトル**: `feat(0092): chief に targets・create・remove を足し、/chief の定義、設定、手引きを直す`

**レビュー観点**: `.claude/settings.json` の拒否と `ask` の規則 / `/chief` の定義が、人の確認の後にだけ `remove` を呼び、issue の題名と Herdr のエラーの文をデータとして扱うか / 手引きの保証すること・しないことと既知の制限 / 実地の確かめの結果

**実装モデル要件**: opus

**判定理由**: ステップ 7-3 は、リポジトリのすべてのセッションに効く守り（拒否と `ask`）を変える安全の境界の変更で、ステップ 7-5 は本物の Herdr でのタスクの作成と削除を伴う。Sonnet は、規則の前方一致の形を誤るか、実地の確かめで通らなかった項目を通ったものとして記録しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（PR-1） | 実地の確かめの記録、Herdr・gh・git のサンプル、Herdr の新しい出力の読み取り | 設計書 1.2 の前提が確かめられる |
| M2（PR-2） | 作る場所、issue、git、issue からタスクを探す読み取り | 作成と削除の材料を読める（まだ使われない） |
| M3（PR-3） | 作ったことの記録、`sendAfterCreate` | 作った workspace へ送る入口がある（まだ呼ばれない） |
| M4（PR-4） | `WorkspaceMaker`、`Create`・`Targets`、作成の排他 | 道具の中で作成ができる（まだ呼ばれない） |
| M5（PR-5） | 削除の候補の行と項目、削除の確かめ | `created.json` のある workspace が提案される（まだ現れない。決定 4） |
| M6（PR-6） | `WorkspaceCloser`、`Remove`、結合テスト | 道具の中で削除ができる（まだ呼ばれない） |
| M7（PR-7） | `targets`・`create`・`remove`、`Protocol` 2、`/chief` の定義、設定、手引き | chief で作成と削除ができる |

順序は設計書 8 章のとおりで、8 章から動かしたものは §1.3 の決定 1 にすべて挙げた。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | 実地の確かめ、サンプル、Herdr の新しい出力の読み取り | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 | 記録係の読み取り、`strictjson` の読み方、作る場所、パスの workspace、issue、git、`対応 issue`、CLAUDE.md の例外 | sonnet+opus-review | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 | 記録係の書き込みの許可、作ったことの記録、`holdWorkspace` の共有、`sendAfterCreate` | sonnet+opus-review | auto |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | `WorkspaceMaker`、`Create`・`Targets`、作成の排他 | opus | auto |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 | 削除の候補、削除の確かめ、行の例外、`removal` の項目 | opus | auto |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 | `WorkspaceCloser`、`Remove`、結合テスト | opus | auto |
| PR-7 | 7-1 / 7-2 / 7-3 / 7-4 / 7-5 | サブコマンド、`Protocol` 2、`/chief` の定義、設定、CLAUDE.md、手引き、実地の確かめ | opus | auto |

どの PR も、前の PR のマージと `/runplan` の間に、`/runplan` の外の作業を置かない。PR-1 と PR-7 の実地の確かめは、`/runplan` のセッションの中で人と行う。そのため、自動送信はすべて `auto` とする。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 判定と副作用は `internal/chief` と `cmd/chief` のユニットテストで確かめる。外部のコマンドは呼び出しの記録係で、Herdr・gh・git の出力は `testdata/` のサンプルで、要件定義書と計画書は `testdata/requirements/`・`testdata/plans/` のサンプルで、時刻・待ち・時間の上限は差し込んだ fake で置き換える。設計書 7.1 の各項目は、§2 の各ステップのテストが受け持つ。
-   **書き込みの確かめ:** 呼び出しの記録係は、テストが操作ごとに置いた書き込みの許可だけを通し、ほかの書き込みでテストを失敗にする（決定 7）。`gh api graphql` は、決まった形のほかを通さない。そのため、`internal/chief` のテストでは、`snapshot`・`tick`・`targets` と、確かめを通らない作成と削除が書き込みをしないことを確かめられる。状態のディレクトリの記録を消さないことは、テストがディスクの記録を読んで確かめる（ステップ 5-4）。
-   **本物の git:** `worktree add --detach`・`worktree remove`・porcelain・`merge-base --is-ancestor`・`status --porcelain`・`git grep` の振る舞いは、`newTestRepo` の一時的なリポジトリでも確かめる（ステップ 2-6・2-7・4-2・6-1）。利用者の git の設定と署名には触れない（0088 計画書の決定 19）。
-   **書く部品の型:** ゼロ値が何も走らせないことと、構文木のガード（決定 11）で確かめる。
-   **並行の処理:** 作成の排他と workspace ごとの排他は、テストが排他を持ったまま相手を呼ぶ形で、決まった順に起こす（ステップ 4-4）。ゴルーチンの競争の結果には頼らない。
-   **境界の値:** 題名の上限、issue の応答の 64 KiB、`agent start` の時間の上限、全体の時間の上限、pane が消えるのを待つ回数は、境界の前後をテストする。
-   **テストの補助:** `internal/chief` の中だけで使う補助は `internal/chief/test_helpers.go`（必要なら `test_helpers_<category>.go`）に `//go:build test` で置く（test_organization.md の Classification B）。ほかのパッケージから使う補助は作らない。テストの日本語の文字列は、CLAUDE.md の「Go Idioms」の例外の範囲（ステップ 2-8）に限り、要件定義書の行はサンプルのファイルに置く。
-   **結合テスト:** 作成から削除までを、Herdr と git を模した世界で続けて走らせる（ステップ 6-3）。
-   **文書のガード:** `cmd/chief/docs_test.go` が、`--protocol` の番号、呼び出しの形、手引きのサブコマンドの表とプロトコル番号、settings.json の拒否と `ask` を確かめる（ステップ 7-3）。CI の `doc-guard` はすでに `./cmd/chief` と `./internal/chief` を走らせる。「Tool Execution Safety」の承認と「Go Idioms」の例外の文、`/chief` の定義の問いの流れは、PR のレビューで読む（設計書 7.2）。
-   **後方互換:** 作ったことの記録のない workspace の行、送信、自動送信が 0088 のままであることを、§1.4 の「変えないテスト」が変わらずに通ることで確かめる。
-   **実地の確かめ:** PR-1 で設計書 1.2 の前提を、PR-7 で chief 全体を、本物の Herdr で確かめる（設計書 7.3）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 実地の確かめで、設計書 1.2 の前提が違う（信頼の画面が出る、`workspace close` がグループを求める、`ask` が確認を出さない、起動したばかりの Claude Code に transcript がない） | 決定 8 のとおり止まり、人と決め直す。設計書は、どの結果でも安全側に倒れる（設計書 3.3.4・3.4.3） |
| 採ったサンプルに、採取した人を特定できる情報が残る | ステップ 1-4 の置き換えの規則と、サンプルの確かめのテスト。PR-1 のレビュー観点に入れる |
| 途中の PR のマージで道具を入れ直したとき、chief の振る舞いが変わる | 決定 4。作成は PR-7 まで呼ばれず、削除の候補は現れない。作成の排他が取れない周だけ、自動送信をせずに示す |
| 0088 の送信の経路の共有で、送信と自動送信の守りが崩れる | PR-3 を独立させ、今の送信・中継・自動送信のテストを変えずに通す（ステップ 3-3） |
| エージェントの起動が遅く、作成の全体の上限を超える | PR-1 で起動の時間を観測し、決定 6 の値を直す。上限を超えても、作ったものを示して止まる（AC-12） |
| `Protocol` を上げる PR の後、`/chief` の定義と道具の番号が合わない | PR-7 で両方を変える（決定 3）。実地の確かめの間は、chief の worktree をそのブランチにする（決定 12） |
| 削除が作業を失わせる | 削除の確かめ、閉じる直前の読み直し、`--force` なしの `git worktree remove`、人の確認と `ask`（設計書 3.4、5.2）。PR-5 と PR-6 を独立させてレビューする（決定 1） |
| 実地の確かめで作った workspace と worktree が残る | PR-1 は確かめの後に人が片付ける（ステップ 1-2）。PR-7 の作成で作ったものは人が片付けるかタスクに使い、決定 10 の workspace は chief で消す |
| 日程: PR が 7 つになり、chief を使うほかのタスクの PR と並ぶ | PR-1〜PR-6 は道具の振る舞いを変えないので、ほかのタスクの待ち受けを止めずにマージできる（決定 4）。待ち受けが止まるのは、PR-7 の実地の確かめで chief を起動し直す間だけである（決定 12） |
| 日程: PR-1 か PR-7 の実地の確かめで、人の手が空かない | 実地の確かめは人がいるときにだけ進める。`/runplan` は人を待つだけで、ほかのタスクを妨げない |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5）
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5）
- [ ] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4）
- [ ] PR-7 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4 / 7-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

表のパスの `ic/` は `internal/chief/`、`cc/` は `cmd/chief/` を表す。テストの名前は計画の時点の名前で、実装で名前を変えたときは、この表を同じ PR で直す（編集上の訂正）。`manual` は、`/chief` の定義の文を PR のレビューで読むことと、PR-7 の実地の確かめを指す。`/chief` の定義の文だけで実現する部分（問いの順と文、一覧の中の位置）は、0088 計画書の決定 14 と同じく、文の語を探すテストを置かず、`manual` で確かめる。AC-01 はその部分だけからなるので `manual` だけで確かめる。これは、`test`・`static`・`diff` のどれかを求める requirements_process.md §4 の規則の例外で、0088 計画書の決定 14 と同じ理由（AC が `/chief` の定義の文と、セッションがそれに従うことだけで実現する）による。ほかの AC は、道具の側の `test` か、設定の `static` を持つ。

| AC | 確かめ | 種類 | ステップ | PR |
|---|---|---|---|---|
| AC-01 | 定義の入口の位置と `once` の扱い、実地の確かめ（`once` の問いで名前で選ぶ） | manual | 7-2・7-5 | PR-7 |
| AC-02 | `ic/create_test.go::TestTargets`（番号の形、候補、候補のないとき）、定義の問いの順 | test、manual | 4-3・7-2 | PR-4、PR-7 |
| AC-03 | `ic/send_test.go::TestMkreqTarget`、`ic/snapshot_test.go::TestNextCommandRules`（どちらも変えずに通る） | test | 3-3・5-3 | PR-3、PR-5 |
| AC-04 | `ic/create_test.go::TestTargets`（`gh api graphql` を走らせない）、`ic/send_test.go::TestMkreqTarget` | test | 4-3 | PR-4 |
| AC-05 | `ic/issue_test.go::TestReadIssueSamples`、`ic/issue_test.go::TestReadIssueRejects`、`ic/create_test.go::TestCreateRefusesIssue`（何も作らない） | test | 2-5・4-2 | PR-2、PR-4 |
| AC-06 | `ic/create_test.go::TestCreateIssueForm`、`ic/create_test.go::TestTargets`（`issue_form`）、`cc/main_test.go::TestIssueFormIsNoUsageError` | test | 4-2・4-3・7-1 | PR-4、PR-7 |
| AC-07 | `ic/issue_test.go::TestIssueTitleSanitizedAndCapped`、`ic/create_test.go::TestCreateTitleSteersNothing` | test | 2-5・4-2 | PR-2、PR-4 |
| AC-08 | `ic/create_test.go::TestWorktreeAddDetached`（本物の git でブランチを作らない）、`ic/create_test.go::TestCreateStages`（fetch の後のコミットで作る） | test | 4-2 | PR-4 |
| AC-09 | `ic/create_test.go::TestCreateStages`（workspace とタブ 3 つ、`--session` なし）、`ic/create_test.go::TestCreateChecksPanes`、実地の確かめ | test、manual | 4-2・7-5 | PR-4、PR-7 |
| AC-10 | `ic/create_test.go::TestWorkspaceMakerStartsAgentsWithoutArguments`、`ic/send_test.go::TestSendAfterCreateInputs` | test | 3-3・4-1 | PR-3、PR-4 |
| AC-11 | `ic/create_test.go::TestCreateRefusesExisting`（ファイル・ディレクトリ・リンク、登録だけの worktree、パスの workspace）、`ic/taskspace_test.go::TestTaskSpaceRefusesUnsupportedPath`、`ic/create_test.go::TestCreateFetchFailure` | test | 2-3・4-2 | PR-2、PR-4 |
| AC-12 | `ic/create_test.go::TestCreateStopsAtEachStage`、`ic/create_test.go::TestCreateDeadline` | test | 4-2 | PR-4 |
| AC-13 | `ic/create_test.go::TestWorkspaceMakerUsesOnlyItsIDs`、`ic/construction_test.go::TestWriterTypesOnlyFromConstructors`、`ic/autosend_test.go::TestTickSkipsAutoSendWhileCreating`、`ic/create_test.go::TestCreateBusyDuringTick`、定義が `create` を前景で呼ぶこと | test、manual | 4-1・4-4・7-2 | PR-4、PR-7 |
| AC-14 | `ic/records_test.go::TestCreatedRecord`、`ic/create_test.go::TestCreateReplacesOldRecords`、`ic/create_test.go::TestCreateStopsAtEachStage`（記録が残る） | test | 3-2・4-2 | PR-3、PR-4 |
| AC-15 | `ic/send_test.go::TestSendAfterCreateInputs`、`ic/create_test.go::TestCreateSendsMkreq` | test | 3-3・4-2 | PR-3、PR-4 |
| AC-16 | `ic/screen_test.go` の起動したばかりの画面のサンプルの読み取り（信頼を問う画面が採れれば、それを質問と読むこと）、`ic/send_test.go::TestSendAfterCreateInputs`（起動したばかりの状態で送る前の確かめを通る）、`ic/create_test.go::TestCreateAgentNotReady`（エラーの応答でエージェントがいるとき、`blocked` のとき）、実地の確かめ | test、manual | 1-5・3-3・4-2・7-5 | PR-1、PR-3、PR-4、PR-7 |
| AC-17 | `ic/create_test.go::TestCreateAgentNotReady`（送らなかった理由）、`ic/snapshot_test.go::TestCreatedDetachedRowOffersNoMkreq`、定義の示し方 | test、manual | 4-2・5-3・7-2 | PR-4、PR-5、PR-7 |
| AC-18 | `ic/send_test.go::TestSendAfterCreateStopsAfterInput` | test | 3-3 | PR-3 |
| AC-19 | `ic/snapshot_test.go::TestCreatedWorkspaceRows`（ブランチを作った後はタスクの行）、§1.4 の「変えないテスト」 | test | 5-3 | PR-5 |
| AC-20 | `ic/removal_test.go::TestRemovalRow` | test | 5-3 | PR-5 |
| AC-21 | `ic/removal_test.go::TestRemovalRowFallsBack`、§1.4 の「変えないテスト」 | test | 5-3 | PR-5 |
| AC-22 | `ic/pending_test.go::TestSortPending`（`removal` を足す）、定義が「新しいタスクを始める」を最後に置くこと | test、manual | 5-3・7-2 | PR-5、PR-7 |
| AC-23 | `ic/removal_test.go::TestRemovalCandidateConditions`、`ic/task_test.go::TestTasksOfIssue`、`ic/task_test.go::TestRequirementLinesFromGitGrep`（本物の git） | test | 2-7・5-1 | PR-2、PR-5 |
| AC-24 | `ic/removal_test.go::TestRemovalCandidateRefusals`、`ic/records_test.go::TestCreatedRecordUnreadable` | test | 3-2・5-1 | PR-3、PR-5 |
| AC-25 | `ic/removal_test.go::TestRemovalProposal`（項目と出力） | test | 5-3 | PR-5 |
| AC-26 | `ic/autosend_test.go::TestAutoSendLeavesRemovalItems`（自動送信のある周で、閉じる・消す呼び出しがなく、記録が残る） | test | 5-4 | PR-5 |
| AC-27 | `cc/docs_test.go::TestSettingsAsksBeforeRemove`、`ic/removal_test.go::TestRemovalProposal`（問いに示す名前・パス・pane の数）、定義の確認の問い、実地の確かめ | static、test、manual | 5-3・7-2・7-3・7-5 | PR-5、PR-7 |
| AC-28 | `ic/removal_test.go::TestRemovalChecks`、`ic/workspace_test.go::TestCleanRealGit`、`ic/workspace_test.go::TestIsAncestorRealGit`、`ic/removal_test.go::TestRemoveRefusesOnEachCheck`（何も閉じず消さない）、`ic/removal_test.go::TestRemoveRefusesWhenChanged` | test | 2-6・5-2・6-2 | PR-2、PR-5、PR-6 |
| AC-29 | `ic/removal_test.go::TestRemoveFetchFailure` | test | 6-2 | PR-6 |
| AC-30 | `ic/removal_test.go::TestRemoveClosesThenRemoves`、`ic/sim_test.go::TestCreateToRemove`、実地の確かめ | test、manual | 6-2・6-3・7-5 | PR-6、PR-7 |
| AC-31 | `ic/removal_test.go::TestRemoveLeavesTheRest`（`--force` なし、その workspace の記録だけを消す）、`ic/removal_test.go::TestWorktreeRemoveKeepsBranch`（本物の git） | test | 6-1・6-2 | PR-6 |
| AC-32 | `ic/removal_test.go::TestRemoveStopsAtEachStage`、`ic/removal_test.go::TestRemoveStopsWhenHeadMoved`、`ic/removal_test.go::TestRemoveDeadline` | test | 6-2 | PR-6 |
| AC-33 | `ic/sim_test.go::TestCreateToRemove`（消した後の周で行も項目もない） | test | 6-3 | PR-6 |

設計書 7.4 のセキュリティのテストは、AC-07（題名）、AC-11・`ic/taskspace_test.go::TestTaskSpaceIgnoresOtherInputs`（パスと名前）、AC-13（書く部品の型）、AC-24（偽った記録）、AC-26（書き込みをしない）の行が受け持つ。

### PR-1 の実行の記録

（PR-1 の `/runplan` が書く。）

### PR-7 の実行の記録

（PR-7 の `/runplan` が書く。）

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 の表のすべての行の確かめが通り、PR-7 の実地の確かめで設計書 7.3 の最後のフェーズの項目をそれぞれ 1 回以上確かめた。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。各テストは壊して失敗を確かめ、commit のメッセージに書いた。
-   **セキュリティ:** 設計書 7.4 の項目と、書き込みの呼び出しがテストの許したものだけであること（決定 7）と、書く部品の型の作り方（決定 11）をテストで確かめ、settings.json の拒否と `ask` をガードで確かめた。
-   **文書:** `/chief` の定義、手引き、CLAUDE.md、`package_reference.md` が実装と一致する。

## 9. 次のステップ (Next Steps)

-   PR-7 のマージの後、人が決定 12 のとおり chief の worktree を `main` に戻し、`main` から `make install-chief` で入れ直して `/chief` を起動し直す。
-   設計書 9 章の将来の拡張（エージェントのいない workspace の削除、リポジトリの名前、作成の途中で止まった workspace の片付け）は、別のタスクで扱う。
