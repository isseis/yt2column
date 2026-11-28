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

`TestNeutralizedDisplayMarkdownLinearTime` の時間比の判定を全体の実行から外し、`make test-parse-norace` の `-race` なしの単独の実行だけで行う（F-001・F-002）。判定しないときは、成功と出力の契約を確かめる。レシピと環境変数はガードのテストで固定する（AC-06）。

### 1.2. 実装の原則

-   時間比の閾値、入力、計測の回数は変えない（設計書 1 章）。
-   製品のコードは変えない。壊し方の確認で一時的に変えた製品のコードやレシピは、確認の後に元に戻し、コミットしない。
-   壊し方の確認の `go test` には `-count=1` を付ける。ガードはレシピを `make` の子プロセスから読むので、`Makefile` だけを変えてもテストのキャッシュが無効にならないためである。
-   Go のコメント、識別子、文字列リテラルは英語で書く。

### 1.3. 計画の際に決めたこと（2026-11-28）

-   `-run` に入る 2 つのテストの名前は、それぞれのパッケージのガードが関数値から取って確かめる。`internal/transcript` のガードは引数の全体と環境変数を固定し、`internal/publisher` に新しいガードを置いて、自分のテストの名前を確かめる（設計書 3.3 節）。
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

-   `TestNeutralizedDisplayMarkdownLinearTime` は `internal/publisher/display_render_test.go:227-266` にある。ファイルのビルドタグは `test` である（`:1`）。出力の契約は、同じファイルの `checkNeutralizedContract`（`:30-43`）が確かめる。入力は `articleWith`（`:20-28`）で作る。
-   `ls internal/publisher` の出力は `display_render.go display_render_test.go file.go file_render.go file_render_test.go file_test.go format.go golden_test.go mention.go mention_test.go publisher.go reviewlog.go reviewlog_test.go test_helpers.go testdata testutil` で、`race` と `!race` のビルドタグのテストのファイルはない。
-   前例の定数は `copiedParseChecksDeadline` で、`internal/transcript/copied_format_norace_test.go` と `internal/transcript/copied_format_race_test.go` にある。
-   `test-parse-norace` のレシピは `Makefile:111-112`、`make test` からの呼び出しは `Makefile:93-95`、CI の手順は `.github/workflows/ci.yml:168-169` にある。
-   `grep -rln test-parse-norace` が過去のタスクの文書のほかに挙げたのは、`Makefile`、`CLAUDE.md`、`.github/workflows/ci.yml`、`internal/transcript/makefile_test.go`、`internal/transcript/copied_format_test.go`、`internal/transcript/copied_format_race_test.go`、`internal/maketestutil/make.go`、`docs/dev/adr/0038-deadcode-and-race-free-timing.md` である。過去のタスクの文書は変えない。
-   ガードは `internal/transcript/makefile_test.go` の `TestMakeTestParseNorace`（`:36-79`）、`TestMakeTestRunsParseNorace`（`:81-91`）、`TestCIRunsParseNorace`（`:99-184`）である。`TestCIRunsParseNorace` は手順を `run` の値で探し、名前は読まない（`:172`）。ファイルの冒頭のコメント（`:18-21`）と `TestMakeTestParseNorace` の説明（`:36-41`）は、`TestParseCopiedLargeInputs` だけを走らせることを前提に書かれている。
-   `largeInputTestName`（`internal/transcript/makefile_test.go:29-34`）は、`TestParseCopiedLargeInputs` を関数値として参照する。テスト関数の宣言の名前だけを変えると、コンパイルが通らない。
-   `maketestutil.RunTarget`（`internal/maketestutil/make.go:106`）は、GOTEST をスタブに替えてターゲットを走らせ、最後の呼び出しの引数と、指定した名前の環境変数の値（`MakeInvocation.Env`）を返す。`internal/maketestutil/make_test.go` がある。
-   テストの環境変数の名前の例は `YT2COLUMN_TEST_VIDEO_URL`・`YT2COLUMN_TEST_VIDEO_ID` である（`Makefile:209`）。`YT2COLUMN_TEST_TIMING` はまだ使われていない（`grep -rn YT2COLUMN_TEST_TIMING` で 0 件）。
-   `go help build` は、`-p n` を「the number of programs, such as build commands or test binaries, that can be run in parallel」と述べている。
-   `make coverage-compare` の Go の実行は、`-race` なしで `go test -tags test -count=1 -json -coverpkg=./... -coverprofile=... ./...` を走らせる（`internal/coveragecompare/target_go.go:33-40`）。
-   新しいテストのヘルパーのファイルは要らない。`internal/maketestutil/make.go` に定数と関数を足すだけである（`docs/dev/conventions/test_organization.md`）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 線形時間テスト、レシピ、ガード、記述（設計書 3 章、AC-01〜AC-08）

- [ ] **ステップ 1-1**: `internal/maketestutil` の定数と関数
    - 変えるファイル: `internal/maketestutil/make.go`・`internal/maketestutil/make_test.go`・`docs/dev/conventions/package_reference.md`
    - 設計書 3.2 節・3.3 節のとおり、環境変数の名前と値の定数、環境変数の値を判定する純粋な関数、`-run` の値を名前の並びとして読む関数、それぞれのセンチネルエラー、その表のテストを足す
    - `docs/dev/conventions/package_reference.md` の `internal/maketestutil` の行を、足した名前に合わせて同じコミットで直す
    - 壊し方の確認（どれも確認の後に元に戻す）: `-run` の関数が `)$` のない値を受け付けるように変える。環境変数の関数が `0` を判定を求めない値として受け付けるように変える。どちらでも `go test -tags test -count=1 ./internal/maketestutil` が失敗することを確かめる。結果をコミットメッセージに書く
    - 完了の条件: `go test -tags test -count=1 ./internal/maketestutil` が通る
- [ ] **ステップ 1-2**: 線形時間テストの切り替え
    - 変えるファイル: `internal/publisher/display_render_test.go`。追加するファイル: `internal/publisher/display_render_race_test.go`・`internal/publisher/display_render_norace_test.go`
    - 設計書 3.2 節のとおりに書く。環境変数の名前と値は、ステップ 1-1 の定数を参照する
    - 2 乗の壊し方: `neutralizeAts`（`internal/publisher/mention.go:78`）の、各 `@` の単位を判定する箇所（`ats[nextAt] == i` の分岐）に、次の処理を足す。
        1. それより前の本文全体で `@` を数える: `strings.Count(text[:offsets[i]], "@")`（`i` はルーンの位置なので、バイトの位置の `offsets[i]` で切る）
        2. その結果をパッケージの変数に足す

      出力は変わらず、時間は入力の長さの 2 乗に比例する。確認の後に元に戻す
    - 壊し方の確認（どれも確認の後に元に戻す。各実行の時間もコミットメッセージに書く）:
        - 2 乗の壊し方で、`YT2COLUMN_TEST_TIMING=1 go test -tags test -race -count=1 -timeout 30m -run '^TestNeutralizedDisplayMarkdownLinearTime$' ./internal/publisher` が通ること（AC-01）
        - 2 乗の壊し方で、`go test -tags test -count=1 -run '^TestNeutralizedDisplayMarkdownLinearTime$' ./internal/publisher`（環境変数なし）が通ること（AC-04）
        - 2 乗の壊し方で、`YT2COLUMN_TEST_TIMING=1 go test -tags test -count=1 -run '^TestNeutralizedDisplayMarkdownLinearTime$' ./internal/publisher` が時間比で失敗すること（AC-03 の判定そのもの）
        - 壊し方なしで、`YT2COLUMN_TEST_TIMING=2 go test -tags test -count=1 -run '^TestNeutralizedDisplayMarkdownLinearTime$' ./internal/publisher` が値の確認で失敗すること（AC-04）
        - `NeutralizedDisplayMarkdown`（`internal/publisher/display_render.go:28`）が、U+200B を挿入しても `Neutralized` を偽にして返すように壊し、`go test -tags test -race -count=1 -run '^TestNeutralizedDisplayMarkdownLinearTime$' ./internal/publisher` が契約の確認で失敗すること（AC-02）
    - 壊し方とその結果をコミットメッセージに書く
    - 完了の条件: 両方のビルドで、環境変数なしと `1` のどちらでもテストが通り、コミットメッセージに壊し方の記録がある
- [ ] **ステップ 1-3**: レシピとガード
    - 変えるファイル: `Makefile`（レシピ）・`internal/transcript/makefile_test.go`。追加するファイル: `internal/publisher/makefile_test.go`（ガードの名前は `TestMakeTestParseNoraceRunsLinearTime`）
    - 設計書 1 章と 3.3 節・3.4 節のとおりに書く。古くなるコメント（`internal/transcript/makefile_test.go:18-21`・`:36-41`、`internal/transcript/copied_format_test.go:1029-1032`）も直す
    - レシピとガードは 1 つのコミットで変える
    - `make test-parse-norace` を実行し、2 つのテストが順に走って通ることを確かめる（AC-03・AC-05）
    - 2 乗の壊し方（ステップ 1-2）で `make test-parse-norace` が時間比で失敗することを確かめ、元に戻す（AC-03）
    - ガードの壊し方の確認（どれも確認の後に元に戻す）。次のものを 1 つずつ変えて、`go test -tags test -count=1 -run 'TestMakeTest' ./internal/transcript ./internal/publisher` でガードが失敗すること、コンパイルの失敗ではなくガードの確認で失敗することを確かめる（AC-06）:
        - レシピの `-run` から線形時間テストの名前を外す
        - レシピの `-run` に `|TestX` を加える
        - レシピから `./internal/publisher` を外す
        - レシピから `-p 1` を外す
        - レシピから `YT2COLUMN_TEST_TIMING=1` を外す、または値を `2` に変える
        - `TestNeutralizedDisplayMarkdownLinearTime` の名前を、宣言と参照（新しいガードの関数値）のすべてで変え、レシピはそのままにする
        - `TestParseCopiedLargeInputs` の名前を、宣言と参照（`largeInputTestName` の関数値）のすべてで変え、レシピはそのままにする
    - 壊し方とその結果をコミットメッセージに書く
    - 完了の条件: ガードが通り、コミットメッセージに壊し方の記録がある
- [ ] **ステップ 1-4**: CI の手順の名前と記述
    - 変えるファイル: `.github/workflows/ci.yml`・`CLAUDE.md`・`Makefile` のコメント
    - 設計書 3.4 節のとおりに書く。ADR-0038 は 3 文書の PR で更新済みである。決定の食い違いの項（#1206 の実装で解消する）を消す。`git grep -nE 'issues/1206([^0-9]|$)' origin/main -- docs/dev/adr/` でほかに食い違いの項がないかも確かめる（`docs/dev/conventions/adr.md`「Discrepancy statements」）。実装が設計とずれたときはそれも直す
    - `make fmt`・`make test`・`make lint` を通す（AC-08）
    - 完了の条件: `make test`・`make lint` が通る

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `internal/maketestutil` の定数と関数、切り替えた線形時間テスト、`test-parse-norace` のレシピとガード、CI の手順の名前、`CLAUDE.md` の記述 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-4 を含む。

**実装モデル要件**: low

**判定理由**: どの判定条件にも当たらない。前例のある仕組みを広げる変更で、競合する実装の案も条件付きの確認の重なりもない

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   `internal/maketestutil` の新しい関数は、表のテストで受け付ける形と拒む形を確かめる（ステップ 1-1）。
-   AC-01〜AC-04 は、ステップ 1-2 とステップ 1-3 の壊し方の確認で確かめる。
-   AC-05・AC-06 は、ガードのテストとステップ 1-3 の壊し方の確認で確かめる。
-   AC-07 は、PR のレビューで記述を読む。
-   AC-08 は、`make test`・`make lint` と CI で確かめる。
-   テストは消さない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| `-p 1` が外れ、2 つのパッケージが並行に走って時間の判定が負荷を受ける | `internal/transcript` のガードが引数の全体を固定する（設計書 3.3 節） |
| どちらかのテストの名前を変え、`-run` が何にも合わなくなっても `go test` が 0 で終わる | それぞれのパッケージのガードが、関数値から取った名前で確かめる（1.3 節） |
| 2 乗の壊し方の `-race` 付きの実行が遅く、確認が終わらない | `-timeout 30m` を付け、各実行の時間を記録する。終わらなければ、壊し方を軽くして人に知らせる |
| `-race` なしのビルドの定数を偽に変えたり、線形時間テストのサブテストに `t.Parallel()` を足したりして、時間比の判定が黙って弱くなる | ガードでは防がない。定数がその値であることを確かめるテストは、`CLAUDE.md`「Testing Strategy」の定数のテストの禁止に当たる。前例の `copiedParseChecksDeadline` と同じく受け入れ、PR のレビューで読む |
| 手元で `make test` をほかの重い処理と同時に実行し、時間比の判定が負荷を受ける | 防がない。ADR-0038「結果」と同じく受け入れる（要件書 4.3 節） |
| 利用者のシェルに `YT2COLUMN_TEST_TIMING=1` が残り、全体の実行で時間比を判定する | 防がない。値はこのレシピだけが設定するもので、手で設定したときだけ起きる |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-4）
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（最終の PR のため）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | ステップ 1-2 の壊し方の確認（2 乗の時間で、環境変数を `1` にした `-race` 付きのビルドが通る）。記録はコミットメッセージ |
| AC-02 | `test` | `TestNeutralizedDisplayMarkdownLinearTime` の成功と出力の契約の確認と、ステップ 1-2 の壊し方の確認 |
| AC-03 | `test` | `make test-parse-norace` と、ステップ 1-3 の壊し方の確認（2 乗の時間でターゲットが失敗する） |
| AC-04 | `test` | `internal/maketestutil` の環境変数の関数の表のテストと、ステップ 1-2 の壊し方の確認（環境変数なしでは 2 乗の時間でも通り、値が `2` では失敗する） |
| AC-05 | `test` | `TestMakeTestParseNorace` |
| AC-06 | `test` | `TestMakeTestParseNorace`・`TestMakeTestParseNoraceRunsLinearTime`・`TestMakeTestRunsParseNorace`・`TestCIRunsParseNorace` と、ステップ 1-3 の壊し方の確認 |
| AC-07 | `manual` | PR のレビューで、`CLAUDE.md`・`Makefile` のコメント・CI の手順の名前・ADR-0038 を読む |
| AC-08 | `test` | `make test`・`make lint`・CI |
