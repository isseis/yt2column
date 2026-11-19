# 実装計画書：chief は `main` をチェックアウトした worktree をタスクのない workspace と読まず、マージを確かめた PR からタスクを特定する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。

既存のファイルの内容と `file:line` の行番号は、コミット `b5475db4`（`origin/main`）で確かめた。設計書が確かめたコミット `910bb12b` の後に `internal/chief` を変えたのはタスク 0102 の PR #1012 だけで、変えたファイルは `herdr.go`・`sanitize.go`・`detail.go`・`test_helpers.go` とそのテストである。設計書が引く `state.go`・`snapshot.go`・`tick.go`・`autosend.go`・`records.go`・`request.go`・`resetoffer.go`・`create.go`・`position.go`・`removal.go`・`pending.go` は変わっていないので、設計書の行番号はそのまま使える。

用語は、要件書 3.1 と設計書 1 章の冒頭と同じ意味で使う（`main` をチェックアウトした worktree、chief の pane のセッション、chief がマージを確かめた PR、候補のタスク T、chief が確かめたほかのタスク、chief が最後に開いていると確かめた PR、履歴の記録、事象、`main` の行、特定の結果、項目 ID）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、chief が `main` をチェックアウトした worktree の行を、chief の pane のセッションが残した履歴の記録から特定したタスクで決め、特定できなければ要確認にし、人が選ぶ送信を人が選んだ項目で縛って、要件書の F-001〜F-004 を満たす。

### 1.2. 実装の原則

-   **履歴の記録を先に入れ、行を変える変更は 1 つの PR にまとめる。** PR-1 は履歴の記録の読み書きと特定の結果を入れるが、どこからも呼ばないので行も出力も変えない。`main` の行が提案を出し始める変更と、`chief send --item` の照合は、PR-2 に一緒に入れる（設計書 1.2 の最後の項、8 章）。
-   **`main` は HEAD の種類として宣言し、比べるのは 1 か所にする。** ブランチの名前と `main` をバイト列で比べるのは `readWorkspace` だけにし、以降は `headMain` と特定の結果の型で分ける（設計書 1.1）。
-   **自動送信は変えない。** 変えるのは、`gather` の PR の状態の読み取りがラウンドの中の表を使うことだけである（設計書 2.2）。
-   **外部のコマンドは差し込む。** テストは herdr・git・gh を本物で呼ばない。今のテストの道具（`internal/chief/test_helpers.go` の `recorder`、`internal/chief/world_test.go` のテストの world）を使う。
-   **各テストは壊して失敗を確かめる。** 確かめた結果をコミットのメッセージに書く（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-19）

1.  **PR を 2 つに分ける（人が選んだ）。** PR-1 は、履歴の記録の型・名前・読み取り・書き込み・書く条件・特定の結果（`trace.go`）、`isRecordFile` と `start` の片付け、テストのブランチ名 `main` の置き換え（設計書 8 章の 1）。PR-2 は、`headMain` から `chief send --item`・`Protocol`・`/chief` の定義・ガイドまで（8 章の 2〜4）。履歴の記録の読み取りは信頼境界の入口なので、PR-1 で単独にレビューできるようにする。PR-1 の関数はまだ製品のコードから呼ばれないが、`make deadcode` は `-test` でテストからの呼び出しも到達可能と数え（`Makefile:348-375`）、golangci-lint の `unused` も既定でテストを含むので、PR-1 だけでグリーンゲートを通る。テストのブランチ名の置き換えは振る舞いを変えないので、PR-2 の差分を小さくするために PR-1 に置く。1 つの PR にまとめる案は、PR-2 がさらに大きくなるので採らなかった。置き換えを PR-2 に置く案も、同じ理由で採らなかった。ADR: なし（PR の切り方はこのタスクの中で閉じるため）。
2.  **`mainTaskResult` は特定の関数だけが作る。** 行は `mainTaskResult.Kind` で分かれるので、`construction_test.go` の `guardedTypes`（`internal/chief/construction_test.go:19-28`）に `mainTaskResult` を足し、作ってよい関数を特定の関数（ステップ 1-4）だけにする。`chief targets` が「マージの記録なし」として決めるときも、事象のない入力で同じ関数を呼んで作る。型を葉のパッケージに移す案は、`workspaceFacts` と `classify` が同じパッケージにあり、移すと `paneinput.TaskID` 以外の型も動かすことになるので採らなかった。ADR: なし。
3.  **項目 ID の形は、`ItemID` の作り方（`hashName` の 32 桁の小文字の 16 進数、`internal/chief/records.go:385-389`、`internal/chief/pending.go:91-93`）と同じ形を `--item` の確かめに使う。** 今は `ItemID` を読む関数がないので、`pending.go` に形を確かめる関数を足す。`ItemID` は不透明な値で、`/chief` のセッションは出力の `id` をそのまま返すだけなので、形の確かめは打ち間違いを `ErrBadRequest` にするためのものである（CLAUDE.md の「Keep internals unobservable」）。ADR: なし。

### 1.4. 既存コード調査結果

| 領域 | 今あるもの（`b5475db4`） | 変えること |
|---|---|---|
| HEAD の種類 | `headState` は `headUnread`・`headBranch`・`headDetached` の 3 つ（`internal/chief/state.go:220-233`）。`readWorkspace` がブランチの名前の空かどうかで決める（`internal/chief/snapshot.go:277-309`）。`headProblem(h headState)`（`state.go:291-302`）・`decideNext`（`:308-`）・`classify` の `Idle`（`:464`）・`taskDocumentOf`（`internal/chief/position.go:246-269`）・`addRemovals`（`internal/chief/removal.go:397`）・`Tick` の観測（`internal/chief/tick.go:52`）が HEAD の種類を読む | `headMain` を足す（設計書 3.2）。`headProblem` は `workspaceFacts` を受け取る。`addRemovals` と `Tick` の観測の比べ方は変えない（`headMain` は `headDetached` でも `headBranch` でもないので、今の比べ方のまま、削除の行にならず、観測も書かない）。PR-2 |
| `workspaceFacts` の作られ方 | 製品のコードで作るのは `readWorkspace`（`snapshot.go:277`）と自動送信の `decideNext`（`internal/chief/autosend.go:455`、`Head: headBranch`）だけ。テストの道具は `internal/chief/test_helpers.go:590` | `headMain` と特定の結果を入れるのは `readWorkspace` と `applyMainTask` だけにする。自動送信の `decideNext` は変えない（AC-10）。PR-2 |
| ラウンドの読み取り | `readRound`（`snapshot.go:90-149`）が読み取り・判定・記録の添付・削除の提案を 1 つで行う。呼ぶのは `TakeSnapshot`（`:79`）・`Tick`（`tick.go:35`）・`Targets`（`internal/chief/create.go:856`） | `readFacts` と `buildRows` に分ける（設計書 3.4）。PR-2 |
| 観測の記録と自動送信 | `Tick` が `row.PR` と `f.Head == headBranch` の行に観測を書く（`tick.go:49-60`）。`gather` が `PRStateOf` で PR の状態を読む（`autosend.go:359`）。HEAD は自分で読み直し、`main` を `headBranch` とする（`autosend.go:367-375`） | 観測は変えない。`gather` は、ラウンドの中の PR の状態の表に値があればそれを使い、なければ今のとおり読んで表に置く（設計書 3.3 の 1）。PR-2 |
| 記録のディレクトリ | worktree ごとのディレクトリ（`workspaceDirOf`、`internal/chief/records.go:553`、名前は 64 桁の 16 進数 `workspaceDirName`、`:595`）。読む側は `existingRecords`（`:621`）、排他は `WorkspaceRecords.Lock`（`:648`）。記録は `Store.read`（`:161-176`、`encoding/json` の `DisallowUnknownFields`）で読む。`isRecordFile`（`:1349-1353`）・`ClearRecords`（`:1362-1382`）・`removal.go` の `leftAfter`（`:900-` の中で `isRecordFile` を使う）。`start` はセッションの記録を `removeFinishedSessions`（`:497-518`）で消し、worktree ごとのディレクトリは `settleWorkspaces`（`:986-1032`）が排他を持って片付ける | 履歴の記録は `Store.read` を使わず、`internal/strictjson` で読む（設計書 3.1。`encoding/json` は項目の繰り返しと不正な UTF-8 を通すため）。`isRecordFile` に `trace-` で始まる名前を足す。`settleWorkspaces` の片付けで、ほかのセッションの事象と形に合わない `trace-` の名前を消す。PR-1 |
| ラウンドの ID | `BeginRound`（`records.go:1089`）が返す `Round.ID`（20 桁の時刻と 16 桁の 16 進数、`roundFileName`、`:1029-1034`） | 事象の名前と順にそのまま使う。PR-1 |
| タスクの読み取り | `taskOfBranch`・`Tasks.Read`（`internal/chief/task.go:795-822`）、`paneinput.ParseTaskID`（`internal/chief/paneinput/paneinput.go:37-47`） | `pr_open` のタスクの抽出と確かめ、特定したタスクの文書の読み取りにそのまま使う（設計書 3.1・3.2、H-05）。PR-1・PR-2 |
| 人が選ぶ送信 | `ParseSendRequest(workspace, command, issue, pr)`（`internal/chief/request.go:91-118`）、`Send(ctx, deps, req)`（`:194-254`）は `Owner` を受け取らず、`offered`（`:386-412`）が種類だけを比べる。`offeredMkreq`（`:416-425`）。`proposalID`（`internal/chief/pending.go:162-164`） | `SendRequest.item`、`ParseSendRequest` の `item`、`Send` の `Owner`・`applyMainTask`・項目 ID の照合（設計書 3.5）。PR-2 |
| リセットの送信 | `SendOfferedReset`（`internal/chief/resetoffer.go:115-`）が行を作り直す | `applyMainTask` を当てる（設計書 3.5）。PR-2 |
| `cmd/chief` | フラグ（`cmd/chief/main.go:59-67`）、`subcommandFlags`（`:71-86`）、使い方の文（`:88-`）、`parseRequests`（`:243-256`）、`dispatch` の `send`（`:270-272`） | `--item` を足し、`Send` に `owner` を渡す。PR-2 |
| `/chief` の定義 | 提案の送信 `chief send --workspace <workspace> --command <request> --protocol 4`（`.claude/commands/chief.md:418`）。ほかの `chief send` は `runpr`（`:393`）と `mkreq`（`:416`・`:463`）。すべての呼び出しが `--protocol 4` | `:418` に `--item <id>` を足す。すべての `--protocol` を `main` の値 + 1 にする。PR-2 |
| 文書のテスト | `TestDocumentedProtocolMatches`（`cmd/chief/docs_test.go:205`）、`TestDocumentedInvocationsParse`（`:268`、プレースホルダーの値は `placeholderValues`、`:217-225`）、`TestGuideValuesMatchTool`（`:502`） | `placeholderValues` に `<id>` を足す。ほかは番号が合えば今のまま通る。PR-2 |
| 利用ガイド | 番号の表（`docs/dev/guides/chief_guide.md:24-28`）、進行の位置（`:82`）、サブコマンドの表（`:126-139`） | AC-12 の説明、要確認から戻す方法、`--item`、番号（設計書 3.6）。PR-2 |
| ブランチ `main` を使うテスト | `internal/chief/snapshot_test.go` の `Branch: "main"`（`:71-73`・`:99`・`:154`・`:172`・`:465`・`:486`・`:490`・`:496`・`:707`・`:730`・`:880-881`）と `internal/chief/workspace_test.go:53` | `main` でも機能ブランチでもない名前に替える（設計書 3.7）。PR-1 |
| 変えない `"main"` | `autosend_test.go:61`・`:71`（自動送信の `headRef` と `branch`、AC-10 の理由の確かめ）、`task_test.go:282`・`:357`（ブランチの名前の読み取り）、`running_test.go:38`・`:499`（引数の文）、`create_test.go:1060`、`removal_test.go` の削除の確かめ（設計書 3.7） | 変えない |
| 型の作られ方の確かめ | `guardedTypes`（`internal/chief/construction_test.go:19-28`） | `mainTaskResult` を足す（§1.3 の 2）。PR-1 |

テストの道具は、新しい cross-package の道具を足さない。履歴の記録を書いたテストの状態のディレクトリを作る道具が要れば、`internal/chief/test_helpers.go`（`//go:build test`）に足す。

## 2. 実装の手順 (Implementation Steps)

### フェーズ 1: 履歴の記録と片付け（設計書 8 章の 1）

**変えるファイル:** `internal/chief/trace.go`（新規）・`internal/chief/trace_test.go`（新規）・`internal/chief/records.go`・`internal/chief/records_test.go`・`internal/chief/construction_test.go`・`internal/chief/snapshot_test.go`・`internal/chief/workspace_test.go`

-   [ ] **ステップ 1-1**: `snapshot_test.go` と `workspace_test.go` の `Branch: "main"`（§1.4 の表の箇所）を、`main` でも機能ブランチの名前の規則にも合わない名前に替える。テストの期待は変えない。フェーズ 2 で `main` が `/mkreq` を示さなくなるための準備で、置き換えの後もそのまま通ることを確かめる。§1.4 の「変えない `"main"`」は替えない。
-   [ ] **ステップ 1-2**: `trace.go` に、設計書 3.1 の `traceKind`・`traceEvent`、事象の名前（`trace-<セッションのハッシュ>-<ラウンドの ID>-<事象のハッシュ>.json`）の作り方と形、JSON の形を置く。
-   [ ] **ステップ 1-3**: `trace.go` に、今のセッションの履歴の記録を、状態のディレクトリのすべての worktree ごとのディレクトリから読む関数を置く。設計書 3.1 の「読み取り」と「名前とディレクトリ」のすべての求めを満たす。返すのは、worktree ごとの事象の列と、読めない記録の一覧（パスと誤り）である。拒んだ記録は `errRecord` で包む。
-   [ ] **ステップ 1-4**: `trace.go` に、事象の列を設計書 3.2 の順（ラウンドの ID の順。同じラウンドでは `pr_merged`・`pr_closed` を先に）で当てはめた状態と、そこから特定の結果 `mainTaskResult` を決める特定の関数を置く。設計書 3.2 の表の 6 つの順をそのとおりに当てる。`Detail` は設計書 4.2 の理由の `<...>` に入れる値を持つ。`guardedTypes` に `mainTaskResult` を足す（§1.3 の 2）。
-   [ ] **ステップ 1-5**: `trace.go` に、当てはめた状態とそのラウンドの読み取りから、設計書 3.1 の「書く条件」の表のとおりに書く事象を決める関数を置く（純粋な関数で、ファイルを読み書きしない）。`records.go` に、`HeldWorkspace` の排他の下で事象を 1 件書くメソッドを置く（作るだけで上書きしない）。ラウンドへの配線はフェーズ 3 で行う。
-   [ ] **ステップ 1-6**: `records.go` の `isRecordFile` に `trace-` で始まる名前をすべて足す（設計書 3.1 の「名前とディレクトリ」）。`settleWorkspaces` の片付けで、呼び出したセッションでない事象と、`trace-` で始まり形に合わない名前を消す。今のセッションの事象は消さない。
-   [ ] **ステップ 1-7**: テストを書く（§4.1 の PR-1 の行）。

**完了の条件:** `make test` と `make lint` が通る。行と出力は変わらない（ステップ 1-1 のテストが期待を変えずに通る）。

### PR-1 作成ポイント: history records and their cleanup

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `feat(0105): chief の worktree ごとの履歴の記録と特定の結果を足す`

**レビュー観点**: 履歴の記録の読み取りが要件書 4.2 の形をすべて求め、直さずに拒むこと / 特定の結果が設計書 3.2 の順と同じラウンドの事象の順に従うこと / 片付けが今のセッションの事象を消さず、ほかのセッションの事象と形に合わない名前を消すこと / 行と出力が変わらないこと

**実装モデル要件**: opus

**判定理由**: ステップ 1-3 は、同じ利用者のほかのプロセスが書ける記録を読む信頼境界の確かめ（mkplan.md の panel-mode の条件のうち security-gate）で、ステップ 1-4 は同じラウンドの事象の順で特定する側に倒れうる判定である。Sonnet は、`Store.read` の `encoding/json` を使って項目の繰り返しや不正な UTF-8 を通したり、事象をファイルの名前の順で当てはめたりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `main` の行の判定と、`snapshot`・`targets`・`reset` の読み取り（設計書 8 章の 2）

**変えるファイル:** `internal/chief/state.go`・`internal/chief/position.go`・`internal/chief/snapshot.go`・`internal/chief/trace.go`・`internal/chief/create.go`・`internal/chief/resetoffer.go`・`internal/chief/tick.go`（`readRound` の呼び出しを置き換えるだけ）と、そのテスト

-   [ ] **ステップ 2-1**: `state.go` に `headMain` を足し、`workspaceFacts` に特定の結果を足す。`readWorkspace` は、ブランチの名前がバイト列でちょうど `main` のとき `headMain` とする（ほかの読み取りは今のまま）。比べる値は定数 1 つにする。
-   [ ] **ステップ 2-2**: `trace.go` に `applyMainTask` を置く（設計書 3.2 の宣言と「行への当てはめ」）。T と特定したときは `Tasks.Read(worktree, Branch)` で文書を読み、読めなければ `DocsUnreadable` にする。読めないときは理由を `Unreadable` に入れる。
-   [ ] **ステップ 2-3**: `state.go` の `headProblem` を `workspaceFacts` を受け取る形にし、`headMain` の理由を設計書 4.2 の文言で返す。`decideNext` は、特定しなかった `headMain` に `/mkreq` の規則を当てない。`needsTakeoverCheck` は `headMain` で偽を返す。`classify` の `Idle` を設計書 3.2 の「印」のとおりにする。
-   [ ] **ステップ 2-4**: `position.go` の `taskDocumentOf` で、特定しなかった `headMain` の行を要確認にする（`PositionDocNoTask` にしない）。T と特定した行と、`/mkreq` が走っている行は今の規則のままにする。
-   [ ] **ステップ 2-5**: `snapshot.go` の `readRound` を `readFacts` と `buildRows` に分ける（設計書 3.4）。`buildRows` は、渡されたセッションの履歴の記録を読み（ステップ 1-3）、`main` の行ごとに特定の結果を決めて `applyMainTask` を当ててから、今の `classifyWorkspace`・`addRecords`・`addResets`・`addRemovals` を行う。`TakeSnapshot` は `owner` のセッションを渡す。`Targets` は履歴を読まず、事象のない入力で特定の関数を呼んだ結果を使う。`Tick` はこのステップでは `readFacts` の後にすぐ `buildRows` を呼び、今の振る舞いを保つ（書く順はフェーズ 3）。
-   [ ] **ステップ 2-6**: `resetoffer.go` の `SendOfferedReset` で、行を作り直すときに `owner` のセッションの履歴を読み、`applyMainTask` を当てる（設計書 3.5 の最後）。
-   [ ] **ステップ 2-7**: テストを書く（§4.1 の PR-2 のうちフェーズ 2 の行）。

**完了の条件:** `make test` と `make lint` が通る。フェーズ 2 だけでは PR を作らない（`main` の行が提案を出すので、フェーズ 4 の照合と同じ PR に入れる）。

### フェーズ 3: `tick` の書く順と、PR の状態の読み取りの共有（設計書 8 章の 3）

**変えるファイル:** `internal/chief/tick.go`・`internal/chief/autosend.go`・`internal/chief/snapshot.go`（`roundView` に表を足す）と、そのテスト

-   [ ] **ステップ 3-1**: `roundView` に、そのラウンドで読んだ PR の状態の表（PR の番号ごと）を足す。`gather` は、表に値があればそれを使い、なければ今のとおり `PRStateOf` で読んで表に置く。自動送信の条件と結果は変えない。
-   [ ] **ステップ 3-2**: `Tick` を設計書 2.3 の順にする。`readFacts` → `BeginRound` → 観測の記録（今のまま。`view.rows` ではなく読み取りの `PR` を使う）→ 履歴の PR の状態の読み取り（設計書 3.3 の 1）→ worktree ごとの事象の書き込み（3.3 の 2）→ `buildRows` → 作成の排他と自動送信 → `applyAutoSends`。`BeginRound` に失敗したラウンドは、事象を書かずに `buildRows` で行を決める。
-   [ ] **ステップ 3-3**: 事象の書き込みは、書く事象があるときだけ、その worktree の排他を取り、排他の下でブランチと履歴の記録を読み直してから書く（設計書 3.3 の 2）。排他を取れない、ブランチが変わった、`pr_open` が消えていた、PR の状態を読めない、書けない、のどれでも書かず、設計書 4.2 の文言でラウンドの問題に示す。
-   [ ] **ステップ 3-4**: テストを書く（§4.1 の PR-2 のうちフェーズ 3 の行）。

**完了の条件:** `make test` と `make lint` が通る。今の `autosend_test.go` のテストが、期待を変えずに通る。

### フェーズ 4: `chief send --item`、`cmd/chief`、`Protocol`、`/chief` の定義、利用ガイド（設計書 8 章の 4）

**変えるファイル:** `internal/chief/pending.go`・`internal/chief/request.go`・`internal/chief/records.go`（`Protocol`）・`cmd/chief/main.go`・`cmd/chief/docs_test.go`・`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` と、`ParseSendRequest`・`Send` を呼ぶテスト（`send_test.go`・`create_send_test.go`・`records_test.go`・`reset_test.go`・`resetoffer_test.go`）

-   [ ] **ステップ 4-1**: `pending.go` に、文字列が項目 ID の形（§1.3 の 3）かを確かめて `ItemID` を返す関数を置く。
-   [ ] **ステップ 4-2**: `request.go` の `SendRequest` に `item` を足し、`ParseSendRequest` に `item` の引数を足す。`--item` は `mkarch`・`mkplan`・`runplan`・`runplan-takeover` で必須、`mkreq`・`runpr` では受け付けず、形に合わなければ `ErrBadRequest` とする（設計書 3.5）。
-   [ ] **ステップ 4-3**: `Send` に `owner` を足す。排他を持って行を作り直すときに、`owner` のセッションの履歴を読み、`applyMainTask` を当てる。作り直した行の提案（次のコマンドか引き継ぎ）の項目 ID を `proposalID` で作り、`--item` と違えば、何も送らずに `NotSentNotOffered` で拒む。理由は設計書 4.2 の文言にする。送る前の確かめ（`sendCommand`）は変えない。
-   [ ] **ステップ 4-4**: `cmd/chief/main.go` に `--item` を足し（`send` の任意のフラグ、使い方の文）、`parseRequests` から渡し、`dispatch` の `send` に `owner` を渡す。
-   [ ] **ステップ 4-5**: `Protocol`（`internal/chief/records.go:29`）を、実装するときの `origin/main` の値 + 1 にする（CLAUDE.md の「Protocol numbers」。タスク 0102 の PR-3 も上げるので、実装するときに読み直す）。`/chief` の定義のすべての `--protocol` とガイドの番号の表を同じ値にする。
-   [ ] **ステップ 4-6**: `/chief` の定義の提案の送信（`.claude/commands/chief.md:418`）を、設計書 3.6 の形にし、`<id>` が人の選んだ項目の `id` であることを書く。`mkreq` と `runpr` の呼び出しは変えない。`cmd/chief/docs_test.go` の `placeholderValues` に `<id>` の値を足す。
-   [ ] **ステップ 4-7**: `docs/dev/guides/chief_guide.md` に、要件書 AC-12 の 4 点と「-」が `main` の worktree に使われないこと、設計書 3.6 の要確認から戻す方法、セッションが続く場合、`chief send` の `--item` を書く。進行の位置の説明（`:82`）と、サブコマンドの表（`:126-139`）の `send` の行を直す。
-   [ ] **ステップ 4-8**: テストを書く（§4.1 の PR-2 のうちフェーズ 4 の行と、結合テスト）。

**完了の条件:** `make test` と `make lint` が通る。`chief` を PR のブランチで入れ直してから `/runpr` を走らせる（ガイドの 2 章の手順）。

### PR-2 作成ポイント: main worktree rows, history writes, item-bound send, and protocol

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8

**推奨タイトル**: `feat(0105): main をチェックアウトした worktree のタスクを履歴から特定し、人が選んだ項目で送信を縛る`

**レビュー観点**: 特定しなかった `main` の行が `/mkreq` を示さず、送れもしないこと / `tick` が排他の下でブランチと履歴を読み直してから事象を書き、PR の状態を 1 ラウンドに 1 回しか読まないこと / `chief send` が作り直した行の項目 ID と `--item` を照合し、自動送信の提案の別のタスクへ送らないこと / 機能ブランチ・detached HEAD・ほかのブランチの行と自動送信が変わらないこと

**実装モデル要件**: opus

**判定理由**: ステップ 4-5・4-6 は `Protocol` を上げて道具の口と `/chief` の定義を同時に変える移行で、ステップ 3-3 は worktree の排他の下で読み直してから書く並行の処理である（mkplan.md の panel-mode の条件のうち migration）。Sonnet は、排他の外で読んだブランチのまま事象を書いたり、`Send` の照合を種類だけに残したり、テストの期待を新しい振る舞いに合わせて書き換えて AC-11 の確かめを落としたりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 | 確かめ方 |
|---|---|---|
| M1（PR-1） | `trace.go` の型・読み取り・書く条件・特定の結果、書き込みのメソッド、片付け | §4.1 の PR-1 の行のテスト。今のテストが期待を変えずに通る |
| M2（PR-2） | `main` の行の判定、`tick` の書き込み、`chief send --item`、`Protocol`、`/chief` の定義、ガイド | §4.1 の PR-2 の行のテストと結合テスト。§7 の表 |

フェーズ 2〜4 は、設計書 8 章の順に、それぞれ別のコミットにする。レビューで、行の判定、書く順、送信の照合と文書を分けて読めるようにするためである。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | 履歴の記録の読み書き・書く条件・特定の結果、片付け、テストのブランチ名の置き換え | opus | auto |
| PR-2 | 2-1 〜 2-7 / 3-1 〜 3-4 / 4-1 〜 4-8 | `main` の行の判定、`tick` の書き込み、`chief send --item`、`Protocol`、`/chief` の定義、ガイド | opus | auto |

## 4. テスト戦略 (Test Strategy)

### 4.1 単体テスト

| PR | テスト | 確かめること |
|---|---|---|
| PR-1 | `internal/chief/trace_test.go::TestReadTraceRejects` | 設計書 7.3 の記録をそれぞれ `errors.Is(err, errRecord)` で拒み、読めない記録の一覧に入れる。上限ちょうどの記録は読める |
| PR-1 | `internal/chief/trace_test.go::TestReadTraceSessionOnly` | 今のセッションの事象だけを読み、ほかのセッションの事象を読まない。一覧を読めないディレクトリは読めない記録になる |
| PR-1 | `internal/chief/trace_test.go::TestIdentifyMainTask` | 設計書 3.2 の 6 つの順の結果と、設計書 7.1 の「特定の結果」の場合（AC-06 の 2 つの例、AC-07 の 3 つの例、ブランチの行き来、同じラウンドのマージと `branch_task`、タスクの空の PR、開き直された PR）。事象のない入力はマージの記録なし |
| PR-1 | `internal/chief/trace_test.go::TestTraceWrites` | 設計書 3.1 の書く条件の表の各行で書くことと、変化がないラウンド、`OPEN`、状態を読めなかった PR では書かないこと |
| PR-1 | `internal/chief/trace_test.go::TestWriteTraceUnderTheExclusion` | 排他の下で書き、同じ名前の記録を上書きしない |
| PR-1 | `internal/chief/records_test.go::TestClearRecordsKeepsOnlyTheExclusion`（今のテストに履歴の記録を足す） | `ClearRecords` が履歴の記録を消す |
| PR-1 | `internal/chief/records_test.go::TestStartRemovesOtherSessionTraces` | `start` が、ほかのセッションの事象と形に合わない `trace-` の名前を消し、今のセッションの事象を残す |
| PR-1 | `internal/chief/construction_test.go`（今のガード） | `mainTaskResult` を特定の関数のほかで作らない |
| PR-2 | `internal/chief/maintask_test.go::TestMainRowFollowsTaskRules` | T と特定した `main` の行が、設計書が approved で計画書がないとき `/mkplan T`・計画書・作成待ち（AC-01）、設計書が `draft` のとき要確認と今の理由 `the architecture document is not approved`（`internal/chief/state.go:328`）（AC-02）、計画書のすべての PR がマージ済みのときタスク完了で、chief が作った workspace の削除の提案が detached の確かめで止まる（AC-03）。質問で止まったエージェントがあれば質問の行、引き継ぎを出さない、`Idle` が付かない |
| PR-2 | `internal/chief/maintask_test.go::TestMainRowNotIdentified` | マージの記録なし・開いている PR あり・ほかのタスクあり・読めない、の各行の状態・文書（要確認）・理由・次のコマンドなし・`Idle`（設計書 3.2 の「印」。fetch に失敗した行には付かない）（AC-06・AC-07・AC-09） |
| PR-2 | `internal/chief/maintask_test.go::TestOtherHeadsIgnoreHistory` | 履歴の記録があっても、機能ブランチ・detached HEAD・ほかのブランチの行が、記録のないときと同じ行になる（AC-11） |
| PR-2 | `internal/chief/maintask_test.go::TestTickWritesHistory` | `tick` が書く順のとおりに書き、マージを確かめたラウンドの行に T を示す。排他を取れないとき、排他の下でブランチが変わっていたとき、`pr_open` が消えていたとき、状態を読めなかったときは書かず、ラウンドの問題に示す。同じ PR の状態を 1 ラウンドに 1 回しか読まない |
| PR-2 | `internal/chief/maintask_test.go::TestMainRowNoAutoSend` | 特定した `main` の worktree の PR の自動送信の結果が、今と同じく機能ブランチでないことを理由に送らない（AC-10） |
| PR-2 | `internal/chief/send_test.go::TestSendArgumentValidation`（今のテストに足す） | `--item` が要るコマンドで欠けたとき、要らないコマンドに付いたとき、形に合わないときの `ErrBadRequest` |
| PR-2 | `internal/chief/send_test.go::TestSendChecksTheChosenItem` | 作り直した行の提案が `--item` の項目でないとき（行のタスクが変わった、セッションが変わった、ほかのタスクの事象が増えた、記録を読めなくなった、自動送信の提案が行と別のタスクを示す）に `not_offered` で何も送らない（AC-05）。項目が同じなら、特定した `main` の行へ `/mkplan T` を送り、送る前の確かめと送信の記録を経る（AC-04） |
| PR-2 | `internal/chief/send_test.go::TestSendMkreqToMainRefused` | 特定したかによらず、`main` の行への `mkreq` を `not_offered` で拒む（AC-08） |
| PR-2 | `internal/chief/resetoffer_test.go::TestSendOfferedResetOnIdentifiedMain` | 特定した `main` の行のリセットの項目を送れる |
| PR-2 | `internal/chief/create_test.go::TestTargets`（今のテストに足す） | `targets` が `main` の worktree を挙げない |
| PR-2 | `internal/chief/position_test.go`（今のテストに足す） | `taskDocumentOf` が特定しなかった `headMain` を要確認にし、`/mkreq` が走っている行を要件定義書にする |

ステップ 1-1 で名前を替えたテストは、PR-2 の後も期待を変えずに通ることで、AC-11 の 3 つ目（ほかのブランチは `/mkreq`）を確かめる。

### 4.2 結合テスト

`internal/chief/maintask_test.go::TestMainAfterMergeFlow`：テストの world（`internal/chief/world_test.go`）で、設計書 6.2 の流れを 2 ラウンドの `Tick` で走らせる。マージを確かめたラウンドの行に T と `/mkplan T` が示され、続く `Send`（`--item` はその行の項目 ID）で送られ、別のセッションの `TakeSnapshot` では要確認になることを確かめる（設計書 7.2）。

### 4.3 後方互換

`snapshot`・`tick` の出力の項目は変えない（要件書 4.4）。今の `snapshot_test.go`・`autosend_test.go`・`network_test.go` のテストは、ステップ 1-1 の名前の置き換えと、`ParseSendRequest`・`Send` の引数の追加のほかは変えずに通す。古い道具と新しい定義は `Protocol` で断る（設計書 3.6）。

### 4.4 文書

`--protocol` の値は `TestDocumentedProtocolMatches` と `TestGuideValuesMatchTool` が、`--item` の呼び出しの形は `TestDocumentedInvocationsParse` が確かめる。ガイドの説明の文と ADR-0046 が設計書と合っていることは、PR のレビューで読んで確かめる（CLAUDE.md の「Document tests check values, structure, and executable examples, never prose」）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 並行するタスク 0102 の PR-3 が同じ `Protocol` を上げる | ステップ 4-5 で実装するときの `origin/main` の値 + 1 にする。`mergepr` のマージの確かめが、`origin/main` の値 + 1 でなければ止める。止まったときは `mergepr_guide.md` の 7 章のとおり直す |
| 0102 の PR-3 が `/chief` の定義と `snapshot.go` の `readRound` を同じ時期に変える | 先にマージされた側を、後の側が `origin/main` をマージして取り込み、`readRound` の分割と表の配線を合わせる。`TestDocumentedInvocationsParse` が定義の呼び出しの形を確かめる |
| 履歴の記録を書けず、古いタスクを示す | 設計書 5 章の受け入れた残る危険。ラウンドの問題に示し、送るには人が項目を選ぶ |
| 読めない記録で、すべての `main` の行が要確認になる | 安全な側の失敗。ガイドに、新しいセッションにして戻す方法を書く（ステップ 4-7） |
| PR-2 が大きい | フェーズごとにコミットを分ける（§3.1）。PR-1 で記録の読み取りを先にレビューする |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 〜 2-7 / 3-1 〜 3-4 / 4-1 〜 4-8）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 確かめ | 実装 |
|---|---|---|---|
| AC-01 | test | `internal/chief/maintask_test.go::TestMainRowFollowsTaskRules`、`internal/chief/maintask_test.go::TestMainAfterMergeFlow`、`internal/chief/trace_test.go::TestIdentifyMainTask` | ステップ 1-4・2-1〜2-5・3-2 |
| AC-02 | test | `internal/chief/maintask_test.go::TestMainRowFollowsTaskRules` | ステップ 2-2・2-3 |
| AC-03 | test | `internal/chief/maintask_test.go::TestMainRowFollowsTaskRules` | ステップ 2-2・2-3 |
| AC-04 | test | `internal/chief/send_test.go::TestSendChecksTheChosenItem`、`internal/chief/maintask_test.go::TestMainAfterMergeFlow` | ステップ 4-3 |
| AC-05 | test | `internal/chief/send_test.go::TestSendChecksTheChosenItem`、`internal/chief/send_test.go::TestSendArgumentValidation` | ステップ 4-1〜4-4 |
| AC-06 | test | `internal/chief/trace_test.go::TestIdentifyMainTask`、`internal/chief/maintask_test.go::TestMainRowNotIdentified` | ステップ 1-4・2-3 |
| AC-07 | test | `internal/chief/trace_test.go::TestIdentifyMainTask`、`internal/chief/maintask_test.go::TestMainRowNotIdentified`、`internal/chief/maintask_test.go::TestMainAfterMergeFlow`（別のセッション）、`internal/chief/position_test.go` | ステップ 1-4・2-3・2-4・2-5 |
| AC-08 | test | `internal/chief/send_test.go::TestSendMkreqToMainRefused`、`internal/chief/create_test.go::TestTargets` | ステップ 2-3・2-5 |
| AC-09 | test | `internal/chief/trace_test.go::TestReadTraceRejects`、`internal/chief/trace_test.go::TestReadTraceSessionOnly`、`internal/chief/maintask_test.go::TestMainRowNotIdentified` | ステップ 1-3・2-2 |
| AC-10 | test | `internal/chief/maintask_test.go::TestMainRowNoAutoSend`、今の `internal/chief/autosend_test.go::TestAutoSendConditions` | ステップ 3-1・3-2 |
| AC-11 | test | `internal/chief/maintask_test.go::TestOtherHeadsIgnoreHistory`、ステップ 1-1 で名前を替えた `internal/chief/snapshot_test.go::TestTakeSnapshotStates` ほか | ステップ 1-1・2-1・2-3 |
| AC-12 | manual | PR-2 のレビューで、`docs/dev/guides/chief_guide.md` の説明を読んで確かめる。`--protocol` の値は `cmd/chief/docs_test.go::TestGuideValuesMatchTool` | ステップ 4-5・4-7 |
| AC-13 | manual | ADR-0046 は設計書の PR（#1015）で更新済み。PR-2 のレビューで、実装が ADR-0046 の「ブランチ `main` をチェックアウトした worktree のタスク」と合っていることを読んで確かめる | - |

要件書 4.1（新しい種類の呼び出しを足さない、同じ PR を 1 ラウンドに 2 回読まない）は `internal/chief/maintask_test.go::TestTickWritesHistory` が、4.2 の形の求めは `internal/chief/trace_test.go::TestReadTraceRejects` が確かめる。`chief send` の呼び出しの形とプロトコル番号は `cmd/chief/docs_test.go::TestDocumentedInvocationsParse`・`::TestDocumentedProtocolMatches`・`::TestGuideValuesMatchTool` が確かめる。

## 8. 成功基準 (Success Criteria)

-   **機能:** AC-01〜AC-13 が §7 の確かめで通る。
-   **品質:** `make test` と `make lint`（`make deadcode` を含む CI）が通る。各テストを、確かめる部分を壊して失敗させたことをコミットのメッセージに書く。
-   **セキュリティ:** 履歴の記録の読み取りが要件書 4.2 の形をすべて求める（`TestReadTraceRejects`）。送りうる入力の範囲は変わらない（ADR-0046）。
-   **文書:** `/chief` の定義とガイドが新しい `Protocol` と `--item` を書き、ガイドが AC-12 の説明を持つ。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`make install-chief` で道具を入れ直し、`/chief` を起動し直す（ガイドの 2 章）。
-   本計画が approved になった後、`/runplan 0105` で PR-1 から実装する。
