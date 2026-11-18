# 実装計画書：/runpr の NEXT 行の例を chief の読み取りで確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`internal/chief` に、`.claude/commands/runpr.md` の step 4 と step 5 の NEXT 行の例を chief の 2 つの読み取りに通すテストを足す（設計書 3 章）。`CLAUDE.md` の文書を読むガードの一覧にも 1 項目を足す。

### 1.2. 実装の原則

-   **製品コードと `runpr.md` は変えない**（AC-07）。
-   **例はテストに写さない**（要件書 4.5 節）。例は `runpr.md` から読み、読み取りは chief の関数に任せる。
-   **Go のコメント、識別子、文字列リテラルは英語で書く。** `runpr.md` の見出しは英語なので、日本語のリテラルは要らない。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   なし。人に聞くべき開いた判断はなかった。フェーズは 1 つ、PR は 1 本である（3.2 節）。変更の範囲（AC-07）は、記録した diff を読んで確かめる。`CLAUDE.md` の差分は 1 項目の追記で、スクリプトを書くより diff を読むほうが手間が少ないためである（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の「Proportion」）。

### 1.4. 既存コード調査結果

コミット `790652e3`（`origin/main`）で確かめた。

-   NEXT 行の文法は `internal/chief/screen_report.go:56-62`（`nextLine`・`nextCommand`・`nextNoneValue`）にある。画面からの読み取りは `readRunprNext`（`screen_report.go:72`）、transcript からの読み取りは `readReportNext`（`internal/chief/screen_transcript_report.go:303`）である。どちらも `RunprNext.Kind` を返す（`NextUnmatched`・`NextAbsent`・`NextNone`・`NextCommand`、`screen_report.go:14-26`）。
-   既存のテストは、NEXT の読み取りをテスト側の文字列で確かめている（`internal/chief/screen_transcript_report_test.go:384` の `readReportNext` の表など）。`runpr.md` から例を読むテストはない。`runpr.md` を読むテストは `cmd/yt2column/codex_review_docs_test.go`・`cmd/yt2column/fixpr_docs_test.go` にあるが、どちらも呼び出しの形とフィルタだけを読む。
-   `internal/chief` のテストがリポジトリの文書を読む例は、`internal/chief/task_test.go:25-32` の `repoContext`（`../../.claude/commands/_context.md` を `os.ReadFile` で読む）である。
-   見出しで節を取り出す関数は、`internal/doctestutil/markdown.go` の `SectionByHeading`（`:90`）と `SectionOutsideBlocks`（`:44`）にある（ビルドタグ `test`）。コードブロックの判別は `ScanDocLines`（`:130`）、ブロックの取り出しは `FencedBlocks`（`:165`）にある。
-   `runpr.md` の step 4 は `## 4. Confirm, record, and merge`（`runpr.md:777`）、step 5 は `## 5. Switch branch, then show NEXT`（`runpr.md:1338`）である。例は step 4 に 6 つ（`:1046` のコードブロックの 1 行と、`:1090-1098` のコードスパン 5 つ）、step 5 に 2 つ（`:1401`・`:1403`）ある。`:1399` の値のない `NEXT: ` は例でない（要件書 F-001）。
-   `CLAUDE.md` の文書を読むガードの一覧は「Development Notes」の `CLAUDE.md:753` にあり、`internal/chief` について「readings of the routing table and the PR markers」と書いている。`doc-guard` ジョブのコマンド（`CLAUDE.md:756`）は `./internal/chief` を含む。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: ガードテストと CLAUDE.md の 1 項目（設計書 3 章、AC-01〜AC-07）

#### ステップ 1-1: ガードテスト

**変えるファイル:** `internal/chief/runpr_next_doc_test.go`（新規。名前を `screen` で始めない、設計書 3.1 節）

- [ ] `runpr.md` を読み、step 4 と step 5 の節を見出しで取り出す。`internal/doctestutil` の関数を使う。節が見つからなければ失敗する（設計書 3.2 節、AC-03）
- [ ] 節から、要件書 F-001 の定義どおりに NEXT 行の例を取り出す。例が 1 つもない節では失敗する（AC-03）
- [ ] 例のそれぞれを `readRunprNext` と `readReportNext` に渡し、`NextNone` か `NextCommand` でなければ、節と例と経路を示して失敗する（設計書 3.3 節、AC-01・AC-02・AC-04）
- [ ] 例を確かめる部分は、節のテキストを受け取る関数にし、文書を読む部分と分ける（設計書 7 章）
- [ ] テストが理由どおりに失敗できることを確かめる。節のテキストを変えて関数に渡し、次の 3 つでそれぞれ失敗することを見る: 例を 1 つ読めない形に変える（例: `NEXT: ` を `NEXT → ` に、またはタスク番号を 3 桁に）、節から例をすべて消す、見出しを変える。確かめたことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）

**完了の条件:** `go test -tags test ./internal/chief` が通り、上の 3 つの壊し方でテストが失敗したことをコミットメッセージが述べる。

#### ステップ 1-2: CLAUDE.md の一覧

**変えるファイル:** `CLAUDE.md`

- [ ] 「Development Notes」の `internal/chief`'s readings of the routing table and the PR markers（`CLAUDE.md:753`）に、`runpr.md` の NEXT 行の例を読むことを足す（AC-06）

**完了の条件:** 一覧が `runpr.md` の NEXT 行の例を挙げる。

#### ステップ 1-3: 仕上げ

- [ ] `make fmt`・`make test`・`make lint` を通す（AC-05）
- [ ] 7 章の AC-07 の diff の記録を書く

**完了の条件:** グリーンゲートが通り、diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | ガードテスト、`CLAUDE.md` の 1 項目、diff の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-3 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（既存の読み取りの関数を呼ぶテスト 1 つの追加で、方式の選択も危険の高いステップもない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   新しいテスト（ステップ 1-1）が AC-01〜AC-04 を確かめる。テストが理由どおりに失敗できることは、ステップ 1-1 の最後の項目で、節のテキストを変えて確かめる。
-   AC-05 は、テストを `internal/chief` に置き、ビルドタグ `test` で走らせることで満たす。`make test` と `doc-guard` ジョブがこのパッケージを含む。
-   既存の NEXT の読み取りのテスト（`screen_transcript_report_test.go` など）は変えない。文法そのものの確かめは既存のテストが受け持ち、新しいテストは文書の例と文法の突き合わせだけを受け持つ。
-   AC-06 は PR のレビューで読む。AC-07 は記録した diff を読む。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 例の取り出しが緩く、例でないテキスト（`NEXT: ` だけのコードスパンなど）を拾って誤って失敗する | 要件書 F-001 の定義（`NEXT: ` に続けて空白でない文字）で取り出す。2026-11-19 の `runpr.md` でテストが通ることを確かめる |
| 例の取り出しが狭すぎ、例を拾わずに通る | 節ごとに 1 つ以上を求める（AC-03）。ステップ 1-1 の壊し方で確かめる |
| 後で `runpr.md` の見出しが変わり、テストが失敗する | 失敗は意図どおりである。見出しの変更と同じ PR でテストを直す（設計書 9 章） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-3）
- [ ] PR を作成した
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | `internal/chief/runpr_next_doc_test.go` の step 5 の例の確かめ（`go test -tags test ./internal/chief`） |
| AC-02 | `test` | 同じテストの step 4 の例の確かめ |
| AC-03 | `test` | 同じテストの、節が見つからないときと例がないときの失敗。ステップ 1-1 の壊し方で確かめる |
| AC-04 | `test` | 同じテストの、例を読めないときの失敗とメッセージ。ステップ 1-1 の壊し方で確かめる |
| AC-05 | `static` | `make test` と CI の `doc-guard` ジョブ（`./internal/chief` を含む） |
| AC-06 | `manual` | PR のレビューで、`CLAUDE.md` の一覧が `runpr.md` の NEXT 行の例を挙げることを読んで確かめる |
| AC-07 | `diff` | 下の記録のファイルの一覧と `CLAUDE.md` の差分を、PR のレビューで読む |

Diff record (AC-07):

- Compared paths: `CLAUDE.md`
