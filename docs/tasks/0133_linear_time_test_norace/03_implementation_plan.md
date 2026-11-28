# 実装計画書：線形時間テストの時間比の判定を -race なしの単独の実行へ移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`TestNeutralizedDisplayMarkdownLinearTime` の時間比の判定を `-race` 付きのビルドから外し、`make test-parse-norace` の `-race` なしの単独の実行で行う（F-001・F-002）。`-race` 付きのビルドでは、成功と出力の契約を確かめる。レシピはガードのテストで固定する（AC-05）。

### 1.2. 実装の原則

-   時間比の閾値、入力、計測の回数は変えない（設計書 1 章）。
-   製品のコードは変えない。壊し方の確認で一時的に変えた製品のコードは、確認の後に元に戻し、コミットしない。
-   Go のコメント、識別子、文字列リテラルは英語で書く。

### 1.3. 計画の際に決めたこと（2026-11-28）

-   `-run` に入る 2 つのテストの名前は、それぞれのパッケージのガードが関数値から取って確かめる。`internal/transcript` のガードは引数の全体を固定し、`internal/publisher` に新しいガードを置いて、自分のテストの名前と `./internal/publisher` を確かめる（設計書 3.3 節）。
    -   理由: publisher のテスト関数は transcript のパッケージから参照できない。transcript のガードに publisher のテストの名前を文字列で書くと、publisher のテストの名前を変えたとき、`-run` が何にも合わなくなっても通り、確認が黙って失われる。
    -   採らなかった案: transcript のガードだけにし、publisher のテストの名前を文字列で書く案。
    -   2026-11-28 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる、テストの作り方である。
-   フェーズは 1 つ、実装の PR は 1 本とする。
    -   理由: 変更はテスト、ターゲット、ガード、記述の小さな組で、ガードはレシピと同じコミットで変えないと `make test` が通らない。
    -   ADR: なし。本タスクの中で閉じる。
-   本計画書のステップは、書き方 A（`- [ ] **ステップ N-M**`）で書く。chief が確実に読める書き方だからである（タスク 0113 の要件書 1 章）。
    -   ADR: なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `5bba819a`（`origin/main`）で確かめた。

-   `TestNeutralizedDisplayMarkdownLinearTime` は `internal/publisher/display_render_test.go:233-265` にある。ファイルのビルドタグは `test` である（`:1`）。出力の契約は、同じファイルの `checkNeutralizedContract`（`:30-43`）が確かめる。入力は `articleWith`（`:20-28`）で作る。
-   `internal/publisher` に、`race` と `!race` のビルドタグのテストのファイルはない（`ls internal/publisher`）。
-   前例の定数は `copiedParseChecksDeadline` で、`internal/transcript/copied_format_norace_test.go` と `internal/transcript/copied_format_race_test.go` にある。
-   `test-parse-norace` のレシピは `Makefile:111-112`、`make test` からの呼び出しは `Makefile:93-95`、CI の手順は `.github/workflows/ci.yml:168-169` にある。
-   `test-parse-norace` の名前が出てくるのは、`Makefile`、`CLAUDE.md`、`.github/workflows/ci.yml`、`internal/transcript/makefile_test.go`、`internal/transcript/copied_format_test.go`、`internal/transcript/copied_format_race_test.go`、`internal/maketestutil/make.go` のパッケージのコメント、ADR-0038、それに過去のタスクの文書である（`grep -rln test-parse-norace`）。過去のタスクの文書は変えない。
-   ガードは `internal/transcript/makefile_test.go` の `TestMakeTestParseNorace`（`:37-84`）、`TestMakeTestRunsParseNorace`（`:86-95`）、`TestCIRunsParseNorace`（`:97-176`）である。`TestCIRunsParseNorace` は手順を `run` の値で探し、名前は読まない（`:166`）。
-   `maketestutil.RunTarget`（`internal/maketestutil/make.go:106`）は、GOTEST をスタブに替えてターゲットを走らせ、最後の呼び出しの引数を返す。
-   `go help build` は、`-p n` を「同時に走らせられるビルドのコマンドやテストのバイナリの数」と述べている。
-   新しいテストのヘルパーのファイルは要らない。`internal/maketestutil` に関数を 1 つ足すだけである（`docs/dev/conventions/test_organization.md`）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 線形時間テスト、レシピ、ガード、記述（設計書 3 章、AC-01〜AC-07）

- [ ] **ステップ 1-1**: 線形時間テストの切り替え
    - 変えるファイル: `internal/publisher/display_render_test.go`。追加するファイル: `internal/publisher/display_render_race_test.go`・`internal/publisher/display_render_norace_test.go`
    - 設計書 3.2 節のとおりに書く
    - 壊し方の確認（どれも確認の後に元に戻す）:
        - `neutralizeAts`（`internal/publisher/mention.go:78`）に、各 `@` から本文の先頭まで戻って調べる処理を足し、出力を変えずに時間を入力の長さの 2 乗に比例させる。`go test -tags test -race -run '^TestNeutralizedDisplayMarkdownLinearTime$' ./internal/publisher` が通ること（AC-01）と、`go test -tags test -count=1 -run '^TestNeutralizedDisplayMarkdownLinearTime$' ./internal/publisher` が時間比で失敗すること（AC-03）を確かめる
        - U+200B を挿入しても `Neutralized` を偽にするように壊し、`-race` 付きのビルドで線形時間テストが契約の確認で失敗することを確かめる（AC-02）
    - 壊し方とその結果をコミットメッセージに書く
    - 完了の条件: 両方のビルドでテストが通り、コミットメッセージに壊し方の記録がある
- [ ] **ステップ 1-2**: レシピとガード
    - 変えるファイル: `Makefile`・`internal/transcript/makefile_test.go`・`internal/maketestutil/make.go`。追加するファイル: `internal/publisher/makefile_test.go`
    - 設計書 1 章と 3.3 節・3.4 節のとおりに書く
    - レシピとガードは 1 つのコミットで変える
    - 壊し方の確認（どれも確認の後に元に戻す）。レシピから次のものを 1 つずつ外すか変えて、`go test -tags test ./internal/transcript ./internal/publisher -run 'MakeTest'` でガードが失敗することを確かめる（AC-05）:
        - `-run` の線形時間テストの名前
        - `./internal/publisher`
        - `-p 1`
        - `TestNeutralizedDisplayMarkdownLinearTime` の名前（テスト関数の名前だけを変える）
        - `TestParseCopiedLargeInputs` の名前（テスト関数の名前だけを変える）
    - 壊し方とその結果をコミットメッセージに書く
    - `make test-parse-norace` を実行し、2 つのテストが走って通ることを確かめる（AC-03・AC-04）
    - 完了の条件: ガードが通り、コミットメッセージに壊し方の記録がある
- [ ] **ステップ 1-3**: CI の手順の名前と記述
    - 変えるファイル: `.github/workflows/ci.yml`・`CLAUDE.md`・`Makefile` のコメント
    - 設計書 3.4 節のとおりに書く。ADR-0038 は 3 文書の PR で更新済みである。実装が設計とずれたときだけ直す
    - `make fmt`・`make test`・`make lint` を通す（AC-07）
    - 完了の条件: `make test`・`make lint` が通る

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 切り替えた線形時間テスト、`test-parse-norace` のレシピとガード、CI の手順の名前、`CLAUDE.md` の記述 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-3 を含む。

**実装モデル要件**: low

**判定理由**: どの判定条件にも当たらない。前例のある仕組みをそのまま広げる変更で、競合する実装の案も条件付きの確認の重なりもない

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   AC-01〜AC-03 は、ステップ 1-1 の壊し方の確認で確かめる。
-   AC-04・AC-05 は、ガードのテストとステップ 1-2 の壊し方の確認で確かめる。
-   AC-06 は、PR のレビューで記述を読む。
-   AC-07 は、`make test`・`make lint` と CI で確かめる。
-   テストは消さない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| `-p 1` が外れ、2 つのパッケージが並行に走って時間の確認が負荷を受ける | `internal/transcript` のガードが引数の全体を固定する（設計書 3.3 節） |
| どちらかのテストの名前を変え、`-run` が何にも合わなくなっても `go test` が 0 で終わる | それぞれのパッケージのガードが、関数値から取った名前で確かめる（1.3 節） |
| 手元で `make test` をほかの重い処理と同時に実行し、`-race` なしの時間比の判定が負荷を受ける | 防がない。ADR-0038「結果」と同じく受け入れる（要件書 4.3 節） |
| `-race` 付きのビルドの線形時間テストで、8000 単位の入力の確認が遅くなる | 呼び出しは 1 回だけで、計測の繰り返しより短い。`go test` のパッケージのタイムアウトの中に収まる |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-3）
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（最終の PR のため）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | ステップ 1-1 の壊し方の確認（2 乗の時間で `-race` 付きのビルドが通る）。記録はコミットメッセージ |
| AC-02 | `test` | `TestNeutralizedDisplayMarkdownLinearTime` の `-race` 付きのビルドの契約の確認と、ステップ 1-1 の壊し方の確認 |
| AC-03 | `test` | `make test-parse-norace` と、ステップ 1-1 の壊し方の確認（2 乗の時間で `-race` なしのビルドが失敗する） |
| AC-04 | `test` | `TestMakeTestParseNorace` |
| AC-05 | `test` | `TestMakeTestParseNorace`・`internal/publisher` の新しいガード・`TestMakeTestRunsParseNorace`・`TestCIRunsParseNorace` と、ステップ 1-2 の壊し方の確認 |
| AC-06 | `manual` | PR のレビューで、`CLAUDE.md`・`Makefile` のコメント・CI の手順の名前・ADR-0038 を読む |
| AC-07 | `test` | `make test`・`make lint`・CI |
