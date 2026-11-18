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

`internal/chief` に、`.claude/commands/runpr.md` の step 4 と step 5 の NEXT 行の例を chief の 2 つの読み取りに通すテストを足す（設計書 3 章）。そのために、例を取り出す既存の関数を `internal/doctestutil` に移す。`CLAUDE.md` の文書を読むガードの一覧にも 1 項目を足す。

### 1.2. 実装の原則

-   **製品コードと `runpr.md` は変えない**（AC-07）。
-   **例はテストに写さない**（要件書 4.5 節）。例は `runpr.md` から読み、拾う形と読み取りは chief の正規表現と関数に任せる。
-   **関数は振る舞いを変えずに移す。** `cmd/yt2column` の既存のテストは、呼び出しを書き換えた後も変更なしで通る。
-   **Go のコメント、識別子、文字列リテラルは英語で書く。** `runpr.md` の見出しは英語なので、日本語のリテラルは要らない。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   なし。人に聞くべき未決の判断はなかった。フェーズは 1 つ、PR は 1 本である（3.2 節）。変更の範囲（AC-07）は、記録した diff を読んで確かめる。読む diff は `CLAUDE.md` の 1 項目の追記とファイルの一覧だけであり、スクリプトを書くより diff を読むほうが手間が少ないためである（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の「Proportion」）。

### 1.4. 既存コード調査結果

コミット `790652e3`（`origin/main`）で確かめた。

-   NEXT 行の文法は `internal/chief/screen_report.go:56-62`（`nextLine`・`nextCommand`・`nextNoneValue`）にある。NEXT を名指す行と見なす形（`nextNaming`）は `internal/chief/screen_transcript_report.go:293` にある。読み取りは 2 つあり、画面からの読み取りは `readRunprNext`（`screen_report.go:72`、`RunprNext` を返す）、transcript からの読み取りは `readReportNext`（`screen_transcript_report.go:303`、`RunprReport` を返し、NEXT は `.Next`）である。結果の種類は `NextUnmatched`・`NextAbsent`・`NextNone`・`NextCommand` である（`screen_report.go:14-26`）。
-   既存のテストは、NEXT の読み取りをテスト側の文字列で確かめている（`internal/chief/screen_transcript_report_test.go:384` の `readReportNext` の表など）。`runpr.md` を読むテスト（要件書 1 章の背景に挙げた 5 つ）のどれも、NEXT 行の形を見ない。
-   文書の読み込みは `internal/doctestutil/read.go:13` の `ReadDoc`、見出しでの節の取り出しは `internal/doctestutil/markdown.go:44` の `SectionOutsideBlocks` にある（ビルドタグ `test`）。
-   例を取り出す関数は `cmd/yt2column/docs_test.go:442-460` の `codeTexts` で、コードブロックの行と、ブロックの外のコードスパンの中身を返す。`codeSpanRE`（`docs_test.go:440`）を使う。呼び出しは `cmd/yt2column/adr_candidates_docs_test.go:473`・`:489`、`fable_state_docs_test.go:51`、`review_records_test.go:1236`・`:1256`・`:1331` にある。`codeSpanRE` は、`docs_test.go:142`・`:380`・`:464`、`devbuild_docs_test.go:95`、`docpaths_test.go:44` も使う。
-   `cmd/fixprguard/docs_test.go:460` と `cmd/chief/docs_test.go:30` にも似た正規表現の写しがあるが、本タスクでは触らない（要件書 AC-07）。
-   `runpr.md` の step 4 は `## 4. Confirm, record, and merge`（`runpr.md:777`）、step 5 は `## 5. Switch branch, then show NEXT`（`runpr.md:1338`）である。2 つの節の中に、見出しのほかに `#` で始まる行はない。例は設計書 3.2 節のとおり、step 4 に 6 つ、step 5 に 2 つある。
-   `CLAUDE.md` の文書を読むガードの一覧は「Development Notes」の `CLAUDE.md:753` にあり、`internal/chief` について「readings of the routing table and the PR markers」と書いている。`doc-guard` ジョブのコマンド（`.github/workflows/ci.yml`）は `./internal/chief` を含む。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 関数の移動、ガードテスト、CLAUDE.md の 1 項目（設計書 3 章、AC-01〜AC-07）

#### ステップ 1-1: 例を取り出す関数を移す

**変えるファイル:** `internal/doctestutil/markdown.go` とそのテスト、`cmd/yt2column/docs_test.go` と `codeTexts` を呼ぶテスト

- [ ] `codeTexts` を、振る舞いを変えずに `internal/doctestutil` の公開の関数として移す（設計書 3.1 節）。`internal/doctestutil` のテストに、コードブロックの行、ブロックの外のコードスパン、行をまたぐコードスパンを確かめる行を足す
- [ ] `codeSpanRE` も `internal/doctestutil` に移して公開する（設計書 3.1 節）
- [ ] `cmd/yt2column` の `codeTexts` と `codeSpanRE` の定義を消し、呼び出し（1.4 節の `codeTexts` の 6 か所と `codeSpanRE` の 5 か所）を移した先に替える

**完了の条件:** `go test -tags test ./internal/doctestutil ./cmd/yt2column` が通る。

#### ステップ 1-2: ガードテスト

**変えるファイル:** `internal/chief/runpr_next_doc_test.go`（新規。名前を `screen` で始めない、設計書 3.1 節）

- [ ] 文書全体のテキストを受け取り、step 4 と step 5 の節ごとの失敗を返す関数を書く（設計書 7 章）。節は `SectionOutsideBlocks` で取り出し、見つからなければ失敗とする（AC-03）
- [ ] 節から、ステップ 1-1 の関数でコードスパンを取り出し、コードスパンの中の改行と、それに続く空白を 1 つの空白にまとめ、`nextNaming` に合うもの（コロンの後に何もないものを除く）を例とする。例が 1 つもない節は失敗とする（設計書 3.2 節、AC-03）
- [ ] 例のそれぞれを `readRunprNext` と `readReportNext` に渡し、`NextNone` か `NextCommand` でなければ、節と例と経路を示す失敗とする（設計書 3.3 節、AC-01・AC-02・AC-04）
- [ ] テストは `ReadDoc` で `runpr.md` を相対パス（設計書 3.2 節）から読み、上の関数が失敗を返さないことを確かめる
- [ ] テストが理由どおりに失敗できることを確かめる。`runpr.md` の中身を変えたコピーを関数に渡し、次のそれぞれで失敗することを見る。確かめたことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）
    - step 4 か step 5 の 1 つの例を、読めない形に変える（タスク番号を 3 桁に、知らないコマンドに、`NEXT: なし（…）` の全角の括弧を半角に、コロンの後の空白を消す）
    - 1 つの節から例をすべて消す
    - 1 つの節の見出しを変える

**完了の条件:** `go test -tags test ./internal/chief` が通り、上の壊し方でテストが失敗したことをコミットメッセージが述べる。

#### ステップ 1-3: CLAUDE.md の一覧

**変えるファイル:** `CLAUDE.md`

- [ ] 「Development Notes」の `internal/chief`'s readings of the routing table and the PR markers（`CLAUDE.md:753`）に、`runpr.md` の NEXT 行の例を足す（AC-06）

**完了の条件:** 一覧が `runpr.md` の NEXT 行の例を挙げる。

#### ステップ 1-4: 仕上げ

- [ ] `make fmt`・`make test`・`make lint`・`make deadcode` を通す（AC-05）
- [ ] 7 章の AC-07 の diff の記録を書く

**完了の条件:** グリーンゲートが通り、diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 移した関数、ガードテスト、`CLAUDE.md` の 1 項目、diff の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-4 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（テストの関数の移動と、既存の読み取りの関数を呼ぶテスト 1 つの追加で、方式の選択も危険度の高いステップもない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   新しいテスト（ステップ 1-2）が AC-01〜AC-04 を確かめる。テストが理由どおりに失敗できることは、ステップ 1-2 の最後の項目で、`runpr.md` の中身を変えたコピーを渡して確かめる。
-   移した関数は、`internal/doctestutil` の新しいテストと、`cmd/yt2column` の既存のテストで確かめる（ステップ 1-1）。
-   AC-05 は、テストを `internal/chief` に置き、ビルドタグ `test` で走らせることで満たす。`make test` と `doc-guard` ジョブがこのパッケージを含む。
-   既存の NEXT の読み取りのテスト（`screen_transcript_report_test.go` など）は変えない。文法そのものの確認は既存のテストが受け持ち、新しいテストは文書の例と文法の照合だけを受け持つ。
-   AC-06 は PR のレビューで読む。AC-07 は記録した diff を読む。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 例の取り出しが緩く、例でないテキスト（値のない `NEXT: ` など）を拾って誤って失敗する | 要件書 F-001 の定義（値のないスパンを除く）で取り出す。2026-11-19 の `runpr.md` でテストが通ることを確かめる |
| 例の取り出しが狭すぎ、形の違う例を拾わず、テストが通ってしまう | 例を `nextNaming` で拾い、節ごとに 1 つ以上を求める（AC-03）。ステップ 1-2 の壊し方で確かめる |
| 関数を移すときに振る舞いが変わり、`cmd/yt2column` のガードが緩む | 振る舞いを変えずに移し、`cmd/yt2column` の既存のテストが変更なしで通ることを確かめる（ステップ 1-1） |
| 後で `runpr.md` の見出しが変わり、テストが失敗する | 失敗は意図どおりである。見出しの変更と同じ PR でテストを直す（設計書 9 章） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-4）
- [ ] PR を作成した
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | `internal/chief/runpr_next_doc_test.go` の step 5 の例の確認（`go test -tags test ./internal/chief`） |
| AC-02 | `test` | 同じテストの step 4 の例の確認 |
| AC-03 | `test` | 同じテストの、節が見つからないときと例がないときの失敗。ステップ 1-2 の壊し方で確かめる |
| AC-04 | `test` | 同じテストの、例を読めないときの失敗とメッセージ。ステップ 1-2 の壊し方で確かめる |
| AC-05 | `static` | `make test` と CI の `doc-guard` ジョブ（`./internal/chief` を含む） |
| AC-06 | `manual` | PR のレビューで、`CLAUDE.md` の一覧が `runpr.md` の NEXT 行の例を挙げることを読んで確かめる |
| AC-07 | `diff` | 下の diff の記録にある、変わったファイルの一覧と `CLAUDE.md` の差分を、PR のレビューで読む。`cmd/yt2column` と `internal/doctestutil` の差分が関数の移動に限られることは、PR のレビューでコードを読んで確かめる |

Diff record (AC-07):

- Compared paths: `CLAUDE.md`

## 8. 成功の基準 (Success Criteria)

-   7 章のすべての AC の検証が通り、グリーンゲートが通る。

## 9. 次のステップ (Next Steps)

-   3 つの文書が承認されたら、`/runplan 0107` でフェーズ 1 を実装し、PR を開く。
