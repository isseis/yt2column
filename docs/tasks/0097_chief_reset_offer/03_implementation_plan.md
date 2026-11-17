# 実装計画書：chief が報告を読めずに見送ったリセットを、人の選択肢として示す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-17 |
| Review date | 2026-11-17 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

要件書の①②のリセットを、報告がないために見送ったときに見送りの記録を残し、round の `pending` に `reset` の項目として示す。人が選んだときは、`chief reset` で確かめ直して送る。合わせて、`/chief` の定義と chief 利用ガイドを直す。

### 1.2. 実装の原則

-   **今のリセットの判定を変えない**（AC-13）。①②の `Outcome`・`Reason`・打つ入力を変える差分を入れない。今のリセットのテストは、`Outcome`・`Reason` の期待値を変えずに通す。見送りの記録の有無を確かめる列を、今のテストの行に足すのはよい。
-   **送る道筋は今のものを使う。** 排他・送信の停止・送信の記録・送る前の pane の確かめは、`holdWorkspace` と `sendContext.run` を通す（設計書 1.1 節）。
-   **テストは、崩して落ちることを確かめてからコミットする**（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-17）

-   **実装の PR は 1 本とし、フェーズも 1 つにする。** 道具の出力に `reset` の項目が加わると、`/chief` の定義はそれを示さなければならない。また、`cmd/chief/docs_test.go` のガードは、サブコマンドを足すと定義と利用ガイドの更新を求める。分けると、間の期間に `/chief` が知らない種類の項目が出るうえ、ガードも通らないためである。ADR: なし（本タスクの中で閉じる）。
-   **Herdr での実地の確かめの手順は設けない。** 偽物の Herdr を使う今の単体テストで確かめ、実地では次に chief を使うときに観察する。実地の確かめには、長い報告で `no_report` を起こす人の作業が要る。また、送る入力は今のリセットと同じで、送る道筋も今のものを使うためである。ADR: なし（同上）。

### 1.4. 既存コード調査結果

コミット `97ca1fc7`（`origin/main`）で確かめた。

-   ①は `internal/chief/reset.go:94-143`（`resetAfterRunpr`）にある。`no_report` を返すのは、`reset.go:118-120`（PR の詳細が候補の pane から画面の報告を控えていない）、`reset.go:135-137`（画面に報告がない）、`reset.go:52-54`（`lastInputMadePR` が OpenCode の pane に `no_report` を返す）の 3 か所である。①を呼ぶのは `internal/chief/request.go:304-310`（`resetMaker`）で、`Send`（`request.go:194-254`）が `/runpr` を送れたときだけ呼ぶ。
-   ②は `internal/chief/autosend.go:728-777`（`resetRunpr`）にある。`no_report` を返すのは `autosend.go:769-771` の 1 か所で、報告が読めない、PR を名指さない、NEXT が違う、の 3 つの条件をまとめている。②を呼ぶのは `autoSender.send`（`autosend.go:633`）である。
-   PR の詳細の控えの出どころは、`ReportFromScreen` と `ReportFromDescription`（`internal/chief/detail.go:253-256`）である。
-   リセットの入力は `resetPrompt`（`reset.go:76-85`）が決める。送る前の pane の確かめは `checkPane`（`internal/chief/send.go:306-`）で、セッション、状態の変化の番号、入力を待っていること、質問でないこと、入力欄が空であることを確かめる。`sendContext.run`（`send.go:441-`）は、`checkPane` の後、最初の入力の前に `beforeBegin`（`send.go:448-452`）を呼ぶ。
-   transcript の人の最後の入力は `readLastHumanInput`（`internal/chief/screen_transcript.go:170`）で読む。
-   workspace ごとの記録は `Store.Records`・`existingRecords`（`internal/chief/records.go:605-627`）で開き、`Store.create`・`Store.read`（`records.go:144-174`）で書いて読む。消すのは `privatefile.Remove`（`records.go:509` で使用）である。`start` の片付け（`records.go:883-969`）は送信の記録と PR の詳細だけを消し、ほかの名前の記録には触れない。
-   `SentSince`（`records.go:843-850`）は送信の記録を入力の種類によらず数え、自動送信の条件（`autosend.go:672-679`）に使われる。
-   `pending` は `finishSnapshot`（`internal/chief/snapshot.go:148-161`）が `buildPending`（`internal/chief/pending.go:98-133`）で組み立て、`sortPending`（`pending.go:176-192`）が `kindRank`（`pending.go:158-164`）で並べる。`buildPending` は `Row.State` ごとに項目を作る。round が workspace の記録を読むのは `addRecords`（`snapshot.go:169-216`）である。
-   確かめの値は `relay.go` の `checkValue`（`internal/chief/relay.go:35-56`）が作り、形は `checkValueForm` で確かめる。
-   コマンドは `paneinput` の作る関数（`NewMkreq`・`NewMkarch`・`NewMkplan`・`NewRunplan`・`NewRunpr`、`internal/chief/paneinput/paneinput.go:297-331`）でだけ作れる。
-   サブコマンドの表と引数は `cmd/chief/main.go:38-73` にあり、`parseRequests`（`main.go:224-236`）が要求を確かめ、`dispatch`（`main.go:240-266`）が呼び分ける。
-   今のテストのうち、リセットを確かめるのは `internal/chief/reset_test.go` の `TestResetAfterRunpr`・`TestResetChecks`・`TestDetailSavedBeforeReset`・`TestResetFailureAfterClear`・`TestNoResetAfterAnUnrecordedRunpr` と、`internal/chief/autosend_test.go` の `TestResetAfterAutoSend`・`TestResetChecksAgain` である。`cmd/chief/main_test.go` の `TestRunUsageErrors`・`TestRunWorkspaceSubcommands`・`TestRunRefusals` は、サブコマンドごとの行を持つ。送り先の表を差し替えるテストの形は `internal/chief/send_test.go:720-725` にある。
-   文書のガードは `cmd/chief/docs_test.go` の `TestDocumentedInvocationsParse`（`chief.md` が各サブコマンドを呼び、その呼び出しが `parse` と `parseRequests` を通る）と `TestGuideValuesMatchTool`（利用ガイドのサブコマンドの表）である。`.claude/settings.json` は `Bash(chief *)` を許しているので、変えない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 見送りの記録、項目、`chief reset`、文書（設計書 3 章、AC-01〜AC-16）

#### ステップ 1-1: 見送りの記録

**変えるファイル:** `internal/chief/skiprecord.go`（新規）、`internal/chief/skiprecord_test.go`（新規）、`internal/chief/records.go`

- [x] 設計書 3.1 節の記録の型（JSON の形と、確かめを通った値の型）、作る・一覧する・使い済みにする操作を足す
- [x] 作る・読む・同じ名前の 2 回目（すでに記録したものとして扱う）のテストと、設計書 3.1 節の読めない記録の各場合（重なった欄を含む）のテストを書く
- [x] 設計書 3.1 節のとおり、`HeldWorkspace.cleanup`（`internal/chief/records.go`）が 14 日を過ぎた見送りの記録を消すようにし、過ぎた記録（読めない記録を含む）が消え、過ぎていない記録が残るテストを書く

**完了の条件:** 記録のテストが通る。

#### ステップ 1-2: ①②の見送りの種類と記録の書き込み

**変えるファイル:** `internal/chief/reset.go`、`internal/chief/request.go`、`internal/chief/autosend.go`、`internal/chief/screen_report.go`、`internal/chief/reset_test.go`、`internal/chief/autosend_test.go`

- [x] 設計書 3.2 節のとおり、`resetAfterRunpr` と `resetRunpr` が見送りの種類を返すようにする。`reset.go:118-120` と `autosend.go:769-771` は、設計書 3.2 節の表の行ごとに条件を分ける
- [x] 設計書 3.2 節のとおり、`openCodeLastMessage`（`internal/chief/screen_report.go`）がメッセージの始まりが画面にあるかも返すようにし、②の `Detail` を条件ごとに分ける。始まりが画面の外に出た OpenCode の報告（`testdata/screens/opencode-runpr-report.txt` の先頭を切ったもの）が `skipReportAbsent` になり、全体が見える別の PR の報告と、NEXT が違う報告が `skipNotOffered` になるテストを書く。`readRunprReport` のほかの呼び出し元の今のテストが変わらずに通ることを確かめる
- [x] `skipReportAbsent` のときにそのほかの条件を評価し、`resetMaker` と `autoSender.send` が記録を書いて `Detail` に添えるようにする
- [x] 表の各行について、見送りの種類と記録の有無を確かめるテストを書く。①の OpenCode の行は、送り先の表で `runpr` を Claude Code に差し替えて（`send_test.go:720-725` の形）テストする
- [x] `skipReportAbsent` の行で、そのほかの条件を 1 つずつ崩したとき、その読み取り（Herdr・transcript）を失敗させたとき、記録の書き込みを失敗させたとき（`Store.beforeCreate`）のテストを書く。どれも `Outcome`・`Reason` が `not_sent`・`no_report` のままで、打った入力が変わらないことを確かめる
- [x] `no_report` 以外の理由で見送る今のテストの行に、記録が書かれないことの確かめを足す
- [x] 今のリセットのテストが、`Outcome`・`Reason` の期待値を変えずに通ることを確かめる

**完了の条件:** 新しいテストと今のリセットのテストが通る。

#### ステップ 1-3: 項目の組み立てと出力

**変えるファイル:** `internal/chief/state.go`、`internal/chief/snapshot.go`、`internal/chief/pending.go`、`internal/chief/output.go`、`internal/chief/relay.go`、`internal/chief/send.go`、そのテスト

- [x] 確かめの値を作る部分を `relay.go` から共有の関数に切り出し、`CheckValue` と `NotSentCheckMismatch` の説明をリセットの項目にも広げる
- [x] 設計書 3.3 節のとおり、記録から項目を作る関数（条件の確かめと確かめの値）を足し、`addRecords` で `Row.Resets` に入れる。round は記録を消さない
- [x] `buildPending` が、「要確認」の行を除き、`Row.State` によらず `Row.Resets` を `PendingReset` の項目にするようにする。`kindRank` が `PendingReset` に `PendingProposal` と同じ順位を返すようにする。出力に `reset` の欄を足す
- [x] 設計書 3.3 節の条件を 1 つずつ崩すと項目が出ないこと、round が記録を消さないこと、読めない記録が `problems` に出て消されないことのテストを書く
- [x] 条件を満たさない間は項目が出ないこと、「実行中」の行でも項目が出ること、同じ見送りが 1 つの項目になること、並び順と番号、`snapshot` と `tick` の両方に出ることのテストを書く
- [x] 見送りの後に `Store.Start` で別のセッションが chief を起動し直しても、項目が出て、番号と通知が付くことのテストを書く

**完了の条件:** `snapshot` と `tick` の結果に、条件どおりに項目が出る。

#### ステップ 1-4: `chief reset`

**変えるファイル:** `internal/chief/resetoffer.go`（新規）、`internal/chief/resetoffer_test.go`（新規）、`internal/chief/send.go`、`cmd/chief/main.go`、`cmd/chief/main_test.go`

- [x] 設計書 3.4 節の `ParseOfferedResetRequest` と `SendOfferedReset` を足す。項目はステップ 1-3 の関数で作り、transcript の確かめは `readLastHumanInput` を使う（OpenCode の pane では `lastInputMadePR` を呼ばない）
- [x] `cmd/chief/main.go` に `reset` の定数、引数（必須の `--protocol`・`--workspace`・`--check`）、`parseRequests` の場合分け（`ParseOfferedResetRequest` を呼ぶ）、`dispatch` の場合分け、使い方の文言、パッケージの説明の更新を足す
- [x] round が示した確かめの値で送れること（往復）と、pane・セッション・状態の変化の番号のそれぞれが変わると `check_mismatch` になることのテストを書く
- [x] 送り先の表を読めないとき（`unreadable`）、経路が引けないか合わないとき（`check_mismatch`）、transcript の確かめ（①は記録のコマンドの名前と引数、②は名前だけ）が通らないとき、`checkPane` の各理由、使い済みにできないときに何も打たないことのテストを書く。①では、PR がマージされて `prMaker` が別のコマンドを返す状態でも、記録のコマンドで確かめて送れることと、`/mkreq` の記録では引数のある `/mkreq` の入力で送れることを確かめる
- [x] 送れたときに送信の記録があり、記録が消え、次の round に項目が出ないこと、`checkPane` で止まったときは項目が残ること、打った後の失敗で送信が止まることのテストを書く
- [x] `beforeBegin` で chief の持ち主の確かめ直しが通らないときに記録が残って何も打たないこと、使い済みにした後に送信の記録を書けないとき（`Store.beforeCreate`）に `not_sent` で項目が戻らないこと、マージを見た後に人がリセットを選ぶと次の自動送信が提案に変わることのテストを書く
- [x] 設計書 3.1 節のとおり、`sendContext.run` が pane へ最初の入力を打てたときにその pane の見送りの記録を消すようにし、そのテストを書く
- [x] `snapshot`・`tick` が項目のリセットを送らないことのテストを書く
- [x] `TestRunUsageErrors` に `reset` の行（`--check` がない、形に合わない、`--workspace` が相対パス）、`TestRunWorkspaceSubcommands` に workspace がない行、`TestRunRefusals` に `start` の前とロックを持たないときの行を足す

**完了の条件:** `chief reset` のテストと `cmd/chief` のテストが通る。

#### ステップ 1-5: `/chief` の定義と利用ガイド

**変えるファイル:** `.claude/commands/chief.md`、`docs/dev/guides/chief_guide.md`

- [x] 設計書 3.5 節のとおり、`chief.md` に項目の示し方（要件書の AC-03 の添え書き）、選んだときの `chief reset` の呼び出し、結果の示し方、`check_mismatch` の説明を足す
- [x] 利用ガイドのサブコマンドの表、状態のディレクトリの表（`skip-*.json` と、読めない記録を手で消してよいこと）、既知の制限「長い報告は画面から読めない」を直し、道具を入れ直した後に `/chief` を起動し直すことも書く
- [x] `go test -tags test ./cmd/chief` を通す

**完了の条件:** 文書のガードが通る。

#### ステップ 1-6: 仕上げ

- [x] `make fmt`・`make test`・`make lint`・`make deadcode` を通す

**完了の条件:** 4 つのコマンドがすべて通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果 |
|---|---|
| M1 | ステップ 1-1〜1-3: 見送りが項目として示される |
| M2 | ステップ 1-4〜1-6: 人が選んで送れ、文書が道具と合う |

### 3.2 PR 構成

フェーズ 1 のすべて（ステップ 1-1〜1-6）を 1 本の PR にする。

**実装モデル要件**: opus

**判定理由**: ステップ 1-2・1-4 は chief が pane に入力を送る条件（開発用の道具の信頼の境界）を変えるセキュリティの関門で、Sonnet では今のリセットの理由を変えてしまうことや、確かめ直しの抜けが起きやすい

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   単体テストは `internal/chief` の今の偽物（Herdr・コマンドの実行・状態のディレクトリ）で書き、Herdr・エージェント・GitHub には触れない（設計書 7 章）。
-   今のリセットのテストが `Outcome`・`Reason` の期待値を変えずに通ることと、ステップ 1-2 の失敗の経路のテストで、AC-13（今の判定を変えない）が保たれることを確かめる。
-   文書は `cmd/chief/docs_test.go` のガードで値と形を確かめ、文言は PR のレビューで読む。利用ガイドの状態のディレクトリの表の記録の名前は、ほかの記録の名前（`send-*.json`・`stop.json`）と同じく、ガードを足さずに PR のレビューで読む。誤りの害が小さく、ガードの手入れの手間が見合わないためである（CLAUDE.md の「Testing Strategy」）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| ①②の条件の評価を足すときに、今の理由が変わる | 今のリセットのテストの期待値を変えない。ステップ 1-2 で、読み取りの失敗と書き込みの失敗でも `Outcome`・`Reason` が変わらないことを確かめる |
| round とバックグラウンドの `chief send` が同時に動く | round は記録を消さない（設計書 1.2 節）。消すのは workspace の排他の下だけ（`chief reset`、コマンドかリセットを打てた後、`start` の片付け）である |
| タスク 0095 が同じ `reset.go` を変える | どちらかが先にマージされたら、後の PR が `origin/main` を取り込み、設計書 3.2 節の規則を保ったまま箇所を付け替える |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 の PR マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | ステップ 1-2・1-3 のテスト（`internal/chief/reset_test.go`・`internal/chief/autosend_test.go`・`internal/chief/resetoffer_test.go`）: 見送りの後の `snapshot` と `tick` に項目が 1 つ出る |
| AC-02 | `test` | ステップ 1-2 のテスト: 設計書 3.2 節の表の各行の見送りの種類、そのほかの条件を 1 つずつ崩すと記録が書かれないこと、`no_report` 以外の理由の今の行で記録が書かれないこと |
| AC-03 | `test` | ステップ 1-3 のテスト: 項目の出力が pane・エージェント・入力・場面・コマンド・`check` を持つ |
| AC-03 | `manual` | PR のレビューで、`chief.md` の項目の示し方と添え書きを読む |
| AC-04 | `test` | ステップ 1-3 のテスト: `reset` の項目が提案と同じ順位で並び、番号が付く |
| AC-05 | `test` | ステップ 1-1・1-3・1-4 のテスト: 条件を 1 つずつ崩すと項目が出ない。起動し直しても項目が出る。その pane へ入力を打てた後と、14 日を過ぎた後の `start` の後は、記録が消えて項目が出ない |
| AC-06 | `test` | ステップ 1-1・1-3 のテスト: 同じ見送りの 2 回目で記録が増えず、項目が 1 つ |
| AC-07 | `test` | ステップ 1-2 のテスト: `Detail` に、項目にしたか、外れた条件か、書けなかったことが添えられる。②の `no_report` の `Detail` が条件ごとに違う |
| AC-08 | `test` | ステップ 1-4 のテスト: 各確かめが成り立つときだけ 1 回送る。確かめの値の往復。①は記録のコマンドの名前と引数、②は名前を比べる |
| AC-08 | `test` | `cmd/chief/main_test.go::TestRunRefusals`: `start` の前とロックを持たないときに拒む |
| AC-09 | `test` | ステップ 1-4 のテスト: 確かめの値の不一致と各理由で何も打たず、`not_sent` と理由を返す |
| AC-09 | `test` | `cmd/chief/main_test.go::TestRunUsageErrors`: 形に合わない `--check` が使い方の誤りになる |
| AC-10 | `test` | ステップ 1-4 のテスト: 結果の形、送信の記録、打った後の失敗で送信が止まる |
| AC-11 | `test` | ステップ 1-4 のテスト: 送れた後に記録が消え、次の round に項目が出ない。`checkPane` と持ち主の確かめ直しで止まったときは項目が残る。使い済みにできないときは何も打たない。使い済みにした後に送信の記録を書けないときは項目が戻らない |
| AC-12 | `test` | ステップ 1-4 のテスト: `snapshot`・`tick` がリセットを送らない |
| AC-13 | `test` | `internal/chief/reset_test.go`・`internal/chief/autosend_test.go` の今のリセットのテストが `Outcome`・`Reason` の期待値を変えずに通る。ステップ 1-2 のテストが、読み取りの失敗と書き込みの失敗でも `Outcome`・`Reason` が変わらないことを確かめる。ステップ 1-4 のテストが、マージを見た後に選んだリセットで次の自動送信が提案に変わることを確かめる |
| AC-14 | `static` | `cmd/yt2column/adr_test.go::TestADRRecords`: ADR-0046 の形 |
| AC-14 | `manual` | PR のレビュー（本タスクの文書の PR）で、ADR-0046 の決定・採らなかった案・結果・経緯を読む |
| AC-15 | `static` | `cmd/chief/docs_test.go::TestDocumentedInvocationsParse`: `chief.md` が `chief reset` を呼び、その呼び出しが `parse` と `parseRequests` の検査を通る |
| AC-15 | `manual` | PR のレビューで、`chief.md` の選んだときの動きと理由の説明を読む |
| AC-16 | `static` | `cmd/chief/docs_test.go::TestGuideValuesMatchTool`: 利用ガイドのサブコマンドの表が `reset` を含む道具のサブコマンドと一致する |
| AC-16 | `manual` | PR のレビューで、利用ガイドの状態のディレクトリの表と既知の制限を読む |

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-16 の検証が済む。
-   ステップ 1-2・1-4 のテストで、AC-02 と AC-08 の信頼の境界の確かめが成り立つ。
-   `make test`・`make lint`・`make deadcode` が通る。

## 9. 次のステップ (Next Steps)

-   PR のマージの後、`make install-chief` で chief を入れ直し、`/chief` を起動し直す。次に chief を使うとき、長い報告の後の `/runpr` で `reset` の項目が出るかを観察する。
-   タスクの最後の PR のマージの後、`/runpr` が issue #919 を閉じる。

## 変更履歴

- 2026-11-17 承認（isseis、#922）
