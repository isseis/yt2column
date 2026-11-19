# 実装計画書：chief が見出しと太字の段落で書いた計画書のステップを読む

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-19 |
| Review date | 2026-11-19 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

chief の計画書の読み取り（`internal/chief/task.go` の `ParsePlan`）が、ステップを書き方 A・書き方 B・書き方 C（計画書のステップの 3 つの書き方。要件書 1 章の背景と 3 章の冒頭で定める）から読むようにする。読めないときと 0 件のときは、要確認の理由を分けて示す（設計書 3 章）。`CLAUDE.md` の例外の文言を足す。

### 1.2. 実装の原則

-   **書き方 A の読み方と、PR の進み具合と完了の判定は変えない**（AC-11・AC-13）。
-   **テストの計画書は `internal/chief/testdata/plans/` のサンプルに置き、Go のリテラルに持たない**（要件書 4.5 節）。
-   **Go のコメント、識別子、文字列リテラルは英語で書く。** 日本語のリテラルは、計画書のテキストと一致させる `ステップ` などに限る（AC-15）。
-   **protocol 番号は変えない**（AC-14）。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   **フェーズは 1 つ、PR は 1 本とする。** 人に聞くべき未決の判断はなかった。読み取りと理由の文は、互いのテストなしには確かめられず、分けても小さな PR が 2 つになるだけである。
    -   ADR: なし。本タスクの中で閉じる。
-   **本計画書のステップは、書き方 A（`- [ ] **ステップ N-M**`）で書く。** 本タスクの実装がマージされるまで、インストール済みの chief は書き方 A しか読まない。書き方 B で書くと、3 文書の PR のマージの後、chief が本タスク自身の `/runplan` を決められない。
    -   ADR: なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `b93d1111`（`origin/main`）で確かめた。

-   ステップの読み取りは `ParsePlan`（`task.go:532-602`）の中にある。`stepBox`（`:522`）に合う行を `plan.steps` に入れ、重複は `ParsePlan` の誤りにする（`:550-555`）。
-   `StepsLeft`（`:682-691`）の呼び出し側は `nextFromPlan`（`state.go:357`）だけである。`StepsDone`（`:707-734`）の呼び出し側は、引き継ぎの判定（`snapshot.go:331`）だけである。
-   PR マーカーの対象ステップの読み取りの誤りは、`PlanPR.stepsErr`（`:497-499`）に持たせ、`StepsDone` だけが返す。この持ち方を、ステップの読み取りの誤りにも使う（設計書 1 章）。
-   フェンスの外の行は `fencedLines`（`:193`）が、見出しは `anyHeading`（`:277`）が判定する。計画書の 2 章（見出しが `## 2.` で始まる章。要件書 3 章の冒頭）の判定と、書き方 B の見出しの判定に使う。
-   `decideNext` の理由は、`classify`（`state.go:455-461`）で `Row.Unreadable` に入り、自動送信の判定（`autosend.go:456`）と、リセットの判定（`reset.go:31`）でも使われる。
-   テストのサンプルは `planSample`（`test_helpers.go:538`）で `testdata/plans/` から読む。`ParsePlan` の表のテストは `task_test.go:105` の `TestParsePlanProgress`、状態の表のテストは `snapshot_test.go:85-100` 付近、引き継ぎのテストは `snapshot_test.go:299` の `TestTakeoverProposal` にある。
-   書き方 A のステップ ID の重複を確かめるテストはない（設計書 3.1 節）。
-   作ったサンプルの出どころは、`testdata/README.md` の「作ったサンプルの計画書」の節（`:157-164`）に書いてある。
-   `CLAUDE.md` の `task.go` の例外は `CLAUDE.md:919-933` にある。
-   `main` の計画書のステップの書き方の件数は、要件書 1 章の背景にある。
-   本計画を書く際のレビューで、`b93d1111` の 105 件の計画書に、提案の規則と同じ判定を Python で書いて当てた（製品コードではない。書き方 A に似た行の規則は、`grep` で別に確かめた）。PR が残る計画書（0102・0106・0110・0111）に、読めなくなるものはなく、0110・0111 は残りのステップありと読めた。読めなくなるのは、終わったタスクの計画書（0022・0028・0051・0057・0069・0070・0071・0074・0075・0083）だけで、AC-11 によって完了と読む。ステップ 1-3 は、実装の時点の `origin/main` で、実際のコードを使って確かめ直す。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: ステップの読み取りと理由の文（設計書 3 章、AC-01〜AC-16）

- [x] **ステップ 1-1**: ステップの読み取り
    - 変えるファイル: `internal/chief/task.go`・`internal/chief/task_test.go`・`internal/chief/testdata/plans/`（新しいサンプル）・`internal/chief/testdata/README.md`。`StepsLeft` の形を変えるので、唯一の呼び出し側（`state.go:357`）も、コンパイルが通るようにこのステップで合わせる。理由の文の分け方はステップ 1-2 で行う
    - 設計書 3.2 節のとおり、`ParsePlan` が 2 章を判定し、書き方 B・書き方 C のステップと、その範囲のステップのチェックボックスを読むようにする。ステップの行に似た行、別の形のチェックボックス、チェックボックスのないステップ、ステップ ID の重複を、`Plan` のステップの読み取りの誤りとする（AC-01〜AC-04・AC-07・AC-09・AC-10）
    - 設計書 3.3 節のとおり、`StepsLeft` を `(bool, error)` にし、0 件では `errNoSteps` を返す。`StepsDone` は、ステップの読み取りの誤りを最初に返す。ステップの問題で `ParsePlan` が誤りを返さないようにする（AC-11）
    - サンプルを足す。少なくとも、7 章の各行が名指すサンプルを足す。内訳は次のとおりである
        - 書き方 B（残りあり、すべて済み、小見出しの下にだけ `[ ]` があるもの）と書き方 C
        - 計画書の 2 章の外の記録の行
        - 読めない 4 つの条件。別の形のチェックボックスは `[ ]` と `[x]` の 2 つを足す。重複は、2 章の外の書き方 A の重複を含める
        - 0 件
        - すべてマージ済みで読めないもの（書き方 A の重複を含む）
    - `b93d1111` の 0110・0111・0102・0098・0069 の計画書の写しも足す（設計書 7 章）。写しと作ったサンプルの出どころを `testdata/README.md` に書く
    - `task_test.go` に、サンプルごとの `StepsLeft`・`StepsDone`・`Done`・`NextPR` の表のテストを足す。今の書き方 A のサンプルの結果が変わらないことも同じ表で確かめる（AC-13）。誤りは `errors.AsType[*stepError]` で `Kind` と `Detail` を比べ、`errNoSteps` は `errors.Is` で見分ける（設計書 3.2 節）
    - テストが想定した理由で失敗できることを確かめる。読み取りの部分を 1 つずつ外して失敗することを見て、コミットメッセージに書く（設計書 7 章）。外す部分は、書き方 B、書き方 C、2 章の判定、似た行の判定、別の形のチェックボックスの判定、チェックボックスのないステップの判定、重複の判定、`errNoSteps` の分岐である
    - 完了の条件: `go test -tags test ./internal/chief` が通る。上の壊し方でテストが失敗したことを、コミットメッセージが述べる
- [x] **ステップ 1-2**: 判定の理由
    - 変えるファイル: `internal/chief/state.go`・`internal/chief/snapshot_test.go`（引き継ぎのテストが別のファイルなら、そのファイルも）
    - 設計書 3.4 節のとおり、`nextFromPlan` が `StepsLeft` の 3 つの結果で理由を選ぶようにする（AC-03・AC-06・AC-07・AC-12）
    - 状態の表のテストに、次の行を足す（AC-01・AC-02・AC-06・AC-07・AC-11・AC-12）。0110 と 0111 の写しで `/runplan` を提案すること。書き方 C で `/runplan` を提案すること。読めない計画書と 0 件の計画書で要確認になり、理由が互いに違い、`no step left` の理由とも違うこと。すべてマージ済みで読めない計画書が完了になること
    - 引き継ぎのテストに、次のことを足す（AC-05・AC-08）。ブランチの計画書が書き方 B のときと書き方 C のときに、対象ステップが済んでいれば提案すること。対象の外のステップが残っていても提案すること。読めないときに提案せず、要確認の理由を示すこと
    - テストが想定した理由で失敗できることを、`nextFromPlan` の分岐を 1 つずつ外して確かめ、コミットメッセージに書く
    - 完了の条件: `go test -tags test ./internal/chief` が通る
- [x] **ステップ 1-3**: `main` の計画書の一括の確認
    - 実装の際に一度だけ、`main`（実装の時点の `origin/main`）のすべての `docs/tasks/*/03_implementation_plan.md` を `ParsePlan` に通す。ステップの読み取りの誤りが出る計画書と、その誤りを一覧にする（設計書 7 章）。通すプログラムは使い捨てとし、コミットしない
    - 結果と、通したコミットを 7 章の「`main` の計画書の一括の確認の記録」に書く。PR が残っている計画書（作業中のタスク）に誤りが出たら、作業を止めて人に知らせる
    - 完了の条件: 記録が 7 章にある
- [x] **ステップ 1-4**: `CLAUDE.md` の例外
    - 変えるファイル: `CLAUDE.md`
    - 「Go Idioms」の `internal/chief/task.go` の例外（`CLAUDE.md:919-933`）が挙げるリテラルの種類に、ステップの太字の行の頭を足し、リテラルを読む目的に、計画書のステップとその進み具合を足す。例の一覧に `ステップ` を足すかどうかは、文の長さを見て決める（AC-15）
    - 完了の条件: 例外が、ステップの太字の行の頭と、計画書のステップとその進み具合を挙げる
- [x] **ステップ 1-5**: 仕上げ
    - `make fmt`・`make test`・`make lint`・`make deadcode` を通す
    - 7 章の AC-14・AC-16 の diff の記録を書く
    - 完了の条件: グリーンゲートが通り、diff の記録がある

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | ステップの読み取り、理由の文、テストとサンプル、`main` の計画書の一括の確認の記録、`CLAUDE.md` の文言の追記、diff の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-5 を含む。

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-1 が、危険度の高い単独のステップに当たる。2 章の範囲、ステップの範囲、似た行と別の形のチェックボックスの拒否を組み合わせる読み取りである。Sonnet が範囲の終わりや拒否の条件を取り違えると、残りのステップを見落とし、自動送信の判定を誤らせうる

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   読み取りの表のテスト（ステップ 1-1）が、AC-01〜AC-04・AC-07・AC-09〜AC-11・AC-13 を `ParsePlan` の結果で確かめる。
-   状態の表と引き継ぎのテスト（ステップ 1-2）が、AC-01・AC-02・AC-05〜AC-08・AC-11・AC-12 を chief の行の状態と理由で確かめる。
-   テストが想定した理由で失敗できることは、ステップ 1-1・1-2 の「テストが想定した理由で失敗できることを確かめる」の項目で、読み取りと分岐を 1 つずつ外して確かめる。
-   既存の計画書のテスト（`markers_parity_test.go` の `opus-review-decision.sh` との照合など）は変えない。PR の読み取りは変えないので、そのまま通る。
-   `main` のすべての計画書は、ステップ 1-3 で一度だけ通し、記録する。恒久のテストにはしない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 2 章の中の、ステップでない行が、似た行として拒まれ、作業中のタスクの計画書が要確認になる | ステップ 1-3 で `main` のすべての計画書を通し、PR が残る計画書に誤りが出たら作業を止めて人に知らせる |
| 範囲の終わりを取り違え、ステップでないチェックボックス（PR の進み具合、6 章のチェックリスト）をステップに数える | 範囲を見出しで終える（設計書 3.2 節）。0110・0111・0102 の実物の写しで確かめる |
| ステップの問題が完了の判定まで止め、終わったタスクの workspace が要確認のまま残る | ステップの問題を `ParsePlan` の誤りにしない（設計書 1 章）。AC-11 のテストで確かめる |
| 実装と並行して、0102・0105 など、chief を変えるほかのタスクの PR が `task.go` や `state.go` を変える | 衝突は、`origin/main` を取り込んで解く（`CLAUDE.md`「Protocol numbers」の規則は、番号を変えない本タスクには当たらない） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | `internal/chief/task_test.go` の読み取りの表（0110・0111 の写し）と、`snapshot_test.go` の状態の表の `/runplan` の提案 |
| AC-02 | `test` | 同じ 2 つのテストの、書き方 C のサンプル |
| AC-03 | `test` | 読み取りの表の、書き方 B・書き方 C のステップの済みと残り（書き方 B の小見出しの下のチェックボックスを含む）。状態の表の `no step left` の理由（書き方 B ですべて済んだサンプル） |
| AC-04 | `test` | 読み取りの表の、2 章の外の記録の行のサンプルと、0102・0098 の写し |
| AC-05 | `test` | 引き継ぎのテストの、書き方 B と書き方 C のブランチの計画書での提案（対象の外のステップが残るものを含む） |
| AC-06 | `test` | 状態の表の、読めない計画書（0069 の写しの PR の進み具合のチェックボックスを `[ ]` に戻したサンプルを含む）で要確認になり、`/runplan` を提案しないこと |
| AC-07 | `test` | 読み取りの表の、4 つの条件ごとの誤り（`errors.AsType[*stepError]` の `Kind` と `Detail`）。状態の表の理由が、理由の定数と `Detail` から組み立てた文と完全に一致すること（設計書 3.4 節） |
| AC-08 | `test` | 引き継ぎのテストの、読めないブランチの計画書で提案しないこと |
| AC-09 | `test` | 読み取りの表の、範囲の中の別の形のチェックボックスが `[ ]` のサンプルと `[x]` のサンプル |
| AC-10 | `test` | 読み取りの表の、2 章の外の似た行を持つサンプルと、2 章の外の書き方 A の重複を持つサンプル |
| AC-11 | `test` | 読み取りの表と状態の表の、すべてマージ済みで読めない計画書（書き方 A の重複を含む）が完了になること |
| AC-12 | `test` | 読み取りの表の `errNoSteps`（`errors.Is`）。状態の表の、0 件の理由が 0 件の理由の定数と完全に一致すること。0 件で似た行も持つ計画書が AC-07 の理由になること |
| AC-13 | `test` | 読み取りの表の、今の書き方 A のサンプルの結果。既存の状態の表と引き継ぎのテストが変更なしで通ること |
| AC-14 | `diff` | 下の diff の記録で、`internal/chief/records.go` に差分がなく、製品コードの差分が `task.go` と `state.go` に限られ、`state.go` の差分が `nextFromPlan` と理由の定数に限られることを読む（設計書 7 章） |
| AC-15 | `manual` | PR のレビューで、`CLAUDE.md` の例外が、ステップの太字の行の頭と、計画書のステップとその進み具合を挙げることを読む。差分は下の diff の記録にある |
| AC-16 | `diff` | 下の diff の記録の、変わったファイルの一覧を PR のレビューで読む |

`main` の計画書の一括の確認の記録（ステップ 1-3）:

-   2026-11-19、コミット `a68f2532`（`origin/main`）の 107 件の `docs/tasks/*/03_implementation_plan.md` を、実装したコードの `ParsePlan` に通した（使い捨てのテストで、コミットしていない）。`ParsePlan` 自体の誤りは 0014 の 1 件で、PR の進み具合のチェックボックスの誤りである（本タスクの前から同じ。ステップの読み取りの変更とは関係がない）。残る 106 件のうち、ステップの読み取りの誤りが出たのは 10 件で、どれも終わったタスクの計画書であり、すべての PR がマージ済みなので AC-11 により完了と読む。誤りの内訳（Kind は `stepNearMiss`）は、0022（`- [x] **ステップ M-1**:`）、0028（`#### ステップ 4-10a:`）、0051（`**ステップ 4-1・4-2 の完了の条件:**`）、0057（`#### ステップ 1-1・1-2:`）、0069・0070・0071・0074・0075・0083（`- [x] **ステップ N-M（…）**:`）である。承認済みで作っていない PR が残る計画書（0102・0105・0106・0110、と本タスク自身の 0113）に、誤りは出なかった（0111 は、この時点で PR がマージ済みである）。

Diff record (AC-14・AC-15・AC-16):

- Compared paths: `CLAUDE.md`, `internal/chief/records.go`, `internal/chief/state.go`
- Merge-base: `a68f2532fecd2c0f9d44e0c6b1da190a23f2c800`
- Compared commit: `8fe37726fefe4def140216326b2757df512849d0`
- 読み方（実装の際の注記）: `records.go` に差分はない。`state.go` の差分は、`errors` の import、理由の定数、理由を組み立てる関数 `stepsUnreadableReason`、`nextFromPlan` に限られる（AC-14 の行は「`nextFromPlan` と理由の定数」と書く。実装は、設計書 3.4 節の「条件ごとに 1 つずつの文」を `Detail` から組み立てる処理を、`nextFromPlan` の外の関数に出した。型・欄・引数は足していないので、AC-14 の実質は満たすと読む。この読みは実装者のもので、レビューで確かめてほしい）。`CLAUDE.md` の差分は `task.go` の例外の 1 項目に限られる（AC-15）。製品コードの差分は `task.go` と `state.go` だけである（下のファイルの一覧）。

Path diff:

````text
diff --git a/CLAUDE.md b/CLAUDE.md
index 8873fc5526ed1eba74c923eb22fc1a4e077aed46..9f5be33a22f9dfe3ec1b45d2eb0c1b7f510067e5 100644
--- a/CLAUDE.md
+++ b/CLAUDE.md
@@ -948,13 +948,15 @@ Only the conventions a linter does not check are worth stating:
   as for a guard test.
 - The same exception covers the chief tool's task reader,
   `internal/chief/task.go`: a literal that has to equal a heading's text or a
-  fixed part of it, a field label, a value line's prefix, or a checkbox's text
-  in an implementation plan (such as `作成ポイント`, `**実装モデル要件**: `,
-  `**自動送信**: `, `PR 構成`, and `PR を作成した`), which it reads to find
-  a plan's PRs, their values, and their progress; and the start of the line
-  of a requirements document that names its issue (`対応 issue: `) with the
-  full-width parenthesis (`（`) that ends the part of the line it reads,
-  which it reads to find the task that answers an issue. Comments,
+  fixed part of it, a field label, a value line's prefix, the start of a
+  step's bold line, or a checkbox's text in an implementation plan (such as
+  `作成ポイント`, `**実装モデル要件**: `, `**自動送信**: `, `PR 構成`,
+  `PR を作成した`, and `ステップ`), which it reads to find a plan's PRs,
+  their values and progress, and its steps and their progress; and the
+  start of the line of a requirements document that names its issue
+  (`対応 issue: `) with the full-width parenthesis (`（`) that ends the part
+  of the line it reads, which it reads to find the task that answers an
+  issue. Comments,
   identifiers, and the text the tool prints, other than the status table's
   fixed words (the status table exception below), stay English, and the
   plans and
diff --git a/internal/chief/state.go b/internal/chief/state.go
index e413a05f962d2bf3b3d57a5cbc12233658c32783..0b69bef572ff14c20a3f21e360fc9024f43727af 100644
--- a/internal/chief/state.go
+++ b/internal/chief/state.go
@@ -1,6 +1,7 @@
 package chief

 import (
+	"errors"
 	"fmt"
 	"slices"

@@ -341,6 +342,50 @@ func decideNext(f workspaceFacts) (next *Next, done bool, why string) {
 	}
 }

+// The reasons nextFromPlan gives for a plan PR left to create with no
+// /runplan to propose. They differ from one another so that a plan whose
+// steps were not read is not taken for one whose steps are all done.
+const (
+	reasonNoStepLeft = "the plan has a PR left to create but no step left to do"
+	reasonNoSteps    = "the plan has a PR left to create but no step could be read from it"
+	// reasonStepsUnreadable is the start of the reasons for a plan whose
+	// steps cannot be read; stepsUnreadableReason adds the condition's text
+	// and the step ID or line.
+	reasonStepsUnreadable = "the steps of the plan cannot be read"
+)
+
+// Condition texts of reasonStepsUnreadable, one for each stepErrorKind.
+const (
+	reasonStepNoCheckbox    = "a step has no checkbox"
+	reasonStepNearMiss      = "a line looks like a step but is in no accepted form"
+	reasonStepOtherCheckbox = "a checkbox in a step is in no accepted form"
+	reasonStepDuplicate     = "a step ID appears twice"
+)
+
+// stepsUnreadableReason builds the reason for a plan whose steps cannot be
+// read. An error that is not a *stepError, or has no known kind, still gives
+// the plain reason.
+func stepsUnreadableReason(err error) string {
+	se, ok := errors.AsType[*stepError](err)
+	if !ok {
+		return reasonStepsUnreadable
+	}
+	var cond string
+	switch se.Kind {
+	case stepNoCheckbox:
+		cond = reasonStepNoCheckbox
+	case stepNearMiss:
+		cond = reasonStepNearMiss
+	case stepOtherCheckbox:
+		cond = reasonStepOtherCheckbox
+	case stepDuplicate:
+		cond = reasonStepDuplicate
+	default:
+		return reasonStepsUnreadable
+	}
+	return fmt.Sprintf("%s: %s: %q", reasonStepsUnreadable, cond, se.Detail)
+}
+
 // nextFromPlan decides the next command from an approved plan: the task is
 // done when every PR is merged, else the next PR to create picks the route of
 // its /runplan by its model requirement.
@@ -354,8 +399,14 @@ func nextFromPlan(f workspaceFacts, plan *Plan) (next *Next, done bool, why stri
 	}
 	// /runpr offers no /runplan for a plan with no step left to do, so a
 	// plan PR left to create without one is not guessed to be a /runplan.
-	if !plan.StepsLeft() {
-		return nil, false, "the plan has a PR left to create but no step left to do"
+	left, err := plan.StepsLeft()
+	switch {
+	case errors.Is(err, errNoSteps):
+		return nil, false, reasonNoSteps
+	case err != nil:
+		return nil, false, stepsUnreadableReason(err)
+	case !left:
+		return nil, false, reasonNoStepLeft
 	}
 	key := routeKeyOf(pr.Requirement.Value)
 	if key == paneinput.RouteUnknown {
````

File list:

````text
CLAUDE.md
docs/tasks/0113_plan_step_reading/03_implementation_plan.md
internal/chief/snapshot_test.go
internal/chief/state.go
internal/chief/task.go
internal/chief/task_test.go
internal/chief/testdata/README.md
internal/chief/testdata/plans/real-0069-pr-open.md
internal/chief/testdata/plans/real-0069.md
internal/chief/testdata/plans/real-0098.md
internal/chief/testdata/plans/real-0102.md
internal/chief/testdata/plans/real-0110.md
internal/chief/testdata/plans/real-0111.md
internal/chief/testdata/plans/steps-b-all-done.md
internal/chief/testdata/plans/steps-b-markers.md
internal/chief/testdata/plans/steps-b-open.md
internal/chief/testdata/plans/steps-b-subheading.md
internal/chief/testdata/plans/steps-c-all-done.md
internal/chief/testdata/plans/steps-c-markers.md
internal/chief/testdata/plans/steps-c-open.md
internal/chief/testdata/plans/steps-duplicate.md
internal/chief/testdata/plans/steps-grammar.txt
internal/chief/testdata/plans/steps-merged-duplicate.md
internal/chief/testdata/plans/steps-merged-unreadable.md
internal/chief/testdata/plans/steps-near-miss-bold.md
internal/chief/testdata/plans/steps-near-miss-checkbox.md
internal/chief/testdata/plans/steps-near-miss-heading.md
internal/chief/testdata/plans/steps-no-checkbox.md
internal/chief/testdata/plans/steps-none-near-miss.md
internal/chief/testdata/plans/steps-none.md
internal/chief/testdata/plans/steps-other-checkbox-done.md
internal/chief/testdata/plans/steps-other-checkbox-open.md
internal/chief/testdata/plans/steps-outside-chapter2.md
internal/chief/testdata/plans/steps-outside-duplicate.md
````

## 8. 成功の基準 (Success Criteria)

-   7 章のすべての AC の検証が通り、グリーンゲートが通る。

## 9. 次のステップ (Next Steps)

-   3 つの文書が承認されたら、`/runplan 0113` でフェーズ 1 を実装し、PR を開く。
-   実装の PR のマージの後、`make install-chief` で chief を入れ直す（要件書 4.4 節）。

## 変更履歴

- 2026-11-19 承認（isseis、#1028）
