# 実装計画書：chief が報告を読めずに見送ったリセットを、人の選択肢として示す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-17 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

①②のリセットを `no_report` で見送ったときに見送りの記録を残し、round の `pending` に `reset` の項目として示す。人が選んだときは、`chief reset` で確かめ直して送る。合わせて、`/chief` の定義と chief 利用ガイドを直す。

### 1.2. 実装の原則

-   **今のリセットの結果を変えない**（AC-13）。①②の `Outcome`・`Reason` を変える差分を入れない。今のリセットのテストは、期待値を変えずに通す。
-   **送る道筋は今のものを使う。** 排他・送信の停止・送信の記録・送る前の pane の確かめは、`holdWorkspace` と `sendContext.run` を通す（設計書 1.1 節）。
-   **テストは、崩して落ちることを確かめてからコミットする**（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-17）

-   **実装の PR は 1 本とし、フェーズも 1 つにする。** 道具の出力に `reset` の項目が加わると、`/chief` の定義はそれを示さなければならない。また、`cmd/chief/docs_test.go` のガードは、サブコマンドを足すと定義と利用ガイドの更新を求める。分けると、間の期間に `/chief` が知らない種類の項目が出るうえ、ガードも通らないためである。ADR: なし（本タスクの中で閉じる）。
-   **Herdr での実地の確かめの手順は設けない。** 偽物の Herdr を使う今の単体テストで確かめ、実地では次に chief を使うときに観察する。実地の確かめには、長い報告で `no_report` を起こす人の作業が要る。一方で、送る入力は今のリセットと同じで、送る道筋も今のものを使うためである。ADR: なし（同上）。

### 1.4. 既存コード調査結果

コミット `9e3b265ff459adbb58c2cf0517e74449d7e0f7c3` で確かめた。

-   ①は `internal/chief/reset.go:94-143`（`resetAfterRunpr`）にある。`no_report` を返すのは、`reset.go:118-120`（PR の詳細の控えがない）、`reset.go:135-137`（画面に報告がない）、`reset.go:52-54`（`lastInputMadePR` が OpenCode の pane に返す）の 3 か所である。①を呼ぶのは `internal/chief/request.go:304-310`（`resetMaker`）で、`Send`（`request.go:194-254`）が `/runpr` を送れたときだけ呼ぶ。
-   ②は `internal/chief/autosend.go:728-777`（`resetRunpr`）にある。`no_report` を返すのは `autosend.go:769-771` の 1 か所である。
-   リセットの入力は `resetPrompt`（`reset.go:76-85`）が決める。送る前の pane の確かめは `checkPane`（`internal/chief/send.go:306-`）で、セッション・状態の番号・入力を待つこと・質問でないこと・入力欄が空であることを確かめる。
-   workspace ごとの記録は `Store.Records`・`existingRecords`（`internal/chief/records.go:605-627`）で開き、`Store.create`・`Store.read`（`records.go:144-174`）で書いて読む。消すのは `privatefile.Remove`（`records.go:509` で使用）である。
-   `pending` は `finishSnapshot`（`internal/chief/snapshot.go:148-161`）が `buildPending` で組み立て、`sortPending`（`internal/chief/pending.go:176-192`）が `kindRank` で並べる。round が workspace の記録を読むのは `addRecords`（`snapshot.go:169-216`）である。
-   確かめの値は `relay` の `checkValue`（`internal/chief/relay.go:18-56`）にある。形の検査は `checkValueForm` で行う。
-   サブコマンドの表と引数は `cmd/chief/main.go:38-73` にあり、`dispatch`（`main.go:240-266`）が呼び分ける。
-   今のテストのうち、リセットを確かめるのは `internal/chief/reset_test.go` の `TestResetAfterRunpr`・`TestResetChecks`・`TestDetailSavedBeforeReset`・`TestResetFailureAfterClear`・`TestNoResetAfterAnUnrecordedRunpr` と、`internal/chief/autosend_test.go` の `TestResetAfterAutoSend`・`TestResetChecksAgain` である。この変更で、これらの期待値は変えない。
-   文書のガードは `cmd/chief/docs_test.go` の `TestDocumentedInvocationsParse`（`chief.md` が各サブコマンドを呼ぶ）と `TestGuideValuesMatchTool`（利用ガイドのサブコマンドの表）である。`.claude/settings.json` は `Bash(chief *)` を許しているので、変えない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 見送りの記録、項目、`chief reset`、文書（設計書 3 章、AC-01〜AC-16）

#### ステップ 1-1: 見送りの記録

**変えるファイル:** `internal/chief/records.go`（または新しいファイル）、そのテスト

- [ ] 設計書 3.1 節の見送りの記録の型と、作る・一覧する・消す操作を足す。`Agent`・`Scene` は列挙型にし、決まった値でない記録は読めないものとする
- [ ] 作る・読む・読めない記録（知らない欄、続きのデータ、決まった値でない `Agent`・`Scene`、pane ID の形でない `Pane`）・同じ名前の 2 回目のテストを書く

**完了の条件:** 記録のテストが通る。

#### ステップ 1-2: ①②で記録を書く

**変えるファイル:** `internal/chief/reset.go`、`internal/chief/autosend.go`、`internal/chief/reset_test.go`、`internal/chief/autosend_test.go`

- [ ] 設計書 3.2 節の表の 4 か所で、`no_report` を返す前に残りの条件を評価し、満たせば記録を書く。結果の `Outcome`・`Reason` は変えない。書けなかったときは `Detail` に添える
- [ ] 表の各行について、記録が書かれるテストと、残りの条件を 1 つずつ崩して記録が書かれないテストを書く（OpenCode の pane が PR を作った①を含む）
- [ ] 今のリセットのテストが、期待値を変えずに通ることを確かめる

**完了の条件:** 新しいテストと今のリセットのテストが通る。

#### ステップ 1-3: 項目の組み立てと出力

**変えるファイル:** `internal/chief/snapshot.go`、`internal/chief/pending.go`、`internal/chief/output.go`、そのテスト

- [ ] 設計書 3.3 節のとおり、round で記録を読み、条件を満たすものを `PendingReset` の項目にし、満たさないものを消す。読めない記録は `problems` に示す
- [ ] `kindRank` で `PendingProposal` と同じ順位にし、出力に `reset` の欄を足す
- [ ] 項目の出る条件と出ない条件、記録が消えて戻らないこと、読めない記録、同じ見送りが 1 つの項目になること、並び順のテストを書く

**完了の条件:** `snapshot` と `tick` の結果に、条件どおりに項目が出る。

#### ステップ 1-4: `chief reset`

**変えるファイル:** `internal/chief/` の新しいファイル（例: `resetoffer.go`）とそのテスト、`cmd/chief/main.go`、`cmd/chief/main_test.go`

- [ ] 設計書 3.4 節の `ParseResetRequest` と `Reset` を足す
- [ ] `cmd/chief/main.go` に `reset` のサブコマンド（必須の引数 `--protocol`・`--workspace`・`--check`）を足す
- [ ] 確かめの値の不一致、transcript の確かめ（①は名前と引数、②は名前だけ）、`checkPane` の各理由、送れたとき（送信の記録、記録が消えること）、打った後の失敗で送信が止まることのテストを書く
- [ ] `snapshot`・`tick` が項目のリセットを送らないことのテストを書く

**完了の条件:** `chief reset` のテストと `cmd/chief` のテストが通る。

#### ステップ 1-5: `/chief` の定義と利用ガイド

**変えるファイル:** `.claude/commands/chief.md`、`docs/dev/guides/chief_guide.md`

- [ ] 設計書 3.5 節のとおり、`chief.md` に項目の示し方、選んだときの `chief reset` の呼び出し、結果の示し方、`check_mismatch` の説明を足す
- [ ] 利用ガイドのサブコマンドの表、状態のディレクトリの表、既知の制限「長い報告は画面から読めない」を直す
- [ ] `go test -tags test ./cmd/chief` を通す

**完了の条件:** 文書のガードが通る。

#### ステップ 1-6: 仕上げ

- [ ] `make fmt`・`make test`・`make lint`・`make deadcode` を通す

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
-   今のリセットのテストを変えずに通すことで、AC-13 の今の結果を保つことを確かめる。
-   文書は `cmd/chief/docs_test.go` のガードで値と形を確かめ、文言は PR のレビューで読む。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| ①②の条件の評価を足すときに、今の理由が変わる | 今のリセットのテストの期待値を変えない。3.2 節の各箇所で `Outcome`・`Reason` を確かめるテストを足す |
| 合わない記録を消す処理が、round の読み取りの失敗で項目を消す | Herdr の一覧にない workspace や、記録のディレクトリを読めない workspace では消さない（設計書 3.3 節）。そのテストを書く |
| タスク 0095 が同じ `reset.go` を変える | どちらかが先にマージされたら、後の PR が `origin/main` を取り込み、3.2 節の表の箇所を付け替える |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 の PR マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | ステップ 1-2・1-3 のテスト（`internal/chief/reset_test.go`・`internal/chief/autosend_test.go`・項目の組み立てのテスト）: 見送りの後の round に項目が 1 つ出る。OpenCode の pane が PR を作った①を含む |
| AC-02 | `test` | ステップ 1-2 のテスト: 残りの条件を 1 つずつ崩すと記録が書かれない。`no_report` 以外の理由では書かれない |
| AC-03 | `test` | ステップ 1-3 のテスト: 項目の出力が pane・エージェント・入力・コマンド・`check` を持つ |
| AC-03 | `manual` | PR のレビューで、`chief.md` の項目の示し方を読む |
| AC-04 | `test` | ステップ 1-3 のテスト: `reset` の項目が提案と同じ順位で並び、番号が付く |
| AC-05 | `test` | ステップ 1-3 のテスト: 条件を 1 つずつ崩すと項目が出ず、記録が消え、条件が戻っても項目が戻らない |
| AC-06 | `test` | ステップ 1-1・1-3 のテスト: 同じ見送りの 2 回目で記録が増えず、項目が 1 つ |
| AC-07 | `test` | ステップ 1-1・1-3 のテスト: 読めない記録が項目にならず、`problems` に出る |
| AC-08 | `test` | ステップ 1-4 のテスト: 各確かめが成り立つときだけ 1 回送る。①は名前と引数、②は名前を比べる |
| AC-09 | `test` | ステップ 1-4 のテスト: 確かめの値の不一致と各理由で何も打たず、`not_sent` と理由を返す |
| AC-10 | `test` | ステップ 1-4 のテスト: 結果の形、送信の記録、打った後の失敗で送信が止まる |
| AC-11 | `test` | ステップ 1-4 のテスト: 送れた後の round に項目が出ない |
| AC-12 | `test` | ステップ 1-4 のテスト: `snapshot`・`tick` がリセットを送らない |
| AC-13 | `test` | `internal/chief/reset_test.go`・`internal/chief/autosend_test.go` の今のリセットのテストが期待値を変えずに通る。ステップ 1-2 のテストが、各箇所の `Outcome`・`Reason` を確かめる |
| AC-14 | `manual` | PR のレビュー（本タスクの文書の PR）で、ADR-0046 の決定・採らなかった案・結果・経緯を読む |
| AC-15 | `static` | `cmd/chief/docs_test.go::TestDocumentedInvocationsParse`: `chief.md` が `chief reset` を呼び、その呼び出しが道具の引数の検査を通る |
| AC-15 | `manual` | PR のレビューで、`chief.md` の選んだときの動きと理由の説明を読む |
| AC-16 | `static` | `cmd/chief/docs_test.go::TestGuideValuesMatchTool`: 利用ガイドのサブコマンドの表が `reset` を含む道具のサブコマンドと一致する |
| AC-16 | `manual` | PR のレビューで、利用ガイドの状態のディレクトリの表と既知の制限を読む |

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-16 の検証が済む。
-   `make test`・`make lint`・`make deadcode` が通る。

## 9. 次のステップ (Next Steps)

-   PR のマージの後、`make install-chief` で chief を入れ直す。次に chief を使うとき、長い報告の後の `/runpr` で `reset` の項目が出るかを観察する。
-   タスクの最後の PR のマージの後、`/runpr` が issue #919 を閉じる。
