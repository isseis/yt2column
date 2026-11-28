# 要件定義書：線形時間テストの時間比の判定を -race なしの単独の実行へ移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`internal/publisher` の線形時間テスト `TestNeutralizedDisplayMarkdownLinearTime` が、全体の実行（全パッケージのテストを一度に走らせる実行。以下同じ）で CPU の競合のために失敗する問題をなくす要件を定める。時間比の判定は、`-race` なしの単独の実行に移す。

**背景:**

-   `TestNeutralizedDisplayMarkdownLinearTime` は、入力を 4 倍にしたときの処理時間の比が 8 未満であることを確かめる。時間は 6 回計測し、最初の 1 回を除いた 5 回の最良値を使う。時間のほかに確かめるのは、`NeutralizedDisplayMarkdown` がエラーを返さないことだけである（`internal/publisher/display_render_test.go:227-266`）。
-   このテストは、全体の実行のたびに、ほかのパッケージのテストと並行に走る。
    -   `make test` と CI の `Test` ジョブでは、`-race` 付きの全体の実行（`make test-ci`）の中で走る（`Makefile:93-100`）。
    -   `-race` なしの全体の実行でも走る。`go test -tags test -v ./...`（`CLAUDE.md`「Test Commands」）と、`make coverage-compare` の Go の実行（`internal/coveragecompare/target_go.go:33-40`）である。後者は、テストが 1 つでも失敗すると止まる。
-   issue #1206 は、5bba819a で `go test -tags test -race -count=1 ./...` を実行したとき、`TestNeutralizedDisplayMarkdownLinearTime/open_URLs` が `ratio 11.3, want under 8` で失敗したと報告している。ほかのパッケージはすべて通った。
-   原因は、#1166 で並列化が進み、同時に走るパッケージとテストが増えたことによる CPU の競合である（issue #1206「原因」）。#1207 で直列のテストの並列化を進めると、競合はさらに強まる。時間比の判定の余裕は、線形の処理で期待される約 4 倍に対して 8 倍で、約 2 倍しかない。
-   前例として、コピーした字幕のパーサの時間の期限がある。
    -   期限は `-race` なしのビルドだけで確かめ、`-race` 付きのビルドでは時間以外を確かめ続ける（[ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md)）。
    -   期限を確かめる実行は `make test-parse-norace` で、`TestParseCopiedLargeInputs` だけを `-race` なしで走らせる（`Makefile:111-112`）。`make test` は `-race` 付きの実行の後にこのターゲットを呼び（`Makefile:93-95`）、CI の `Test` ジョブも別の手順として呼ぶ（`.github/workflows/ci.yml:168-169`）。
    -   ターゲットの引数、`make test` からの呼び出し、CI の手順は、`internal/transcript/makefile_test.go` のガードのテストが固定している。
-   時間比を判定するテストは、リポジトリにほかにない。
    -   Go のテストのほかの `time.Since` は、待ち時間の上限（タイムアウト）の確認である（例: `internal/llm/llmhttp/llmhttp_test.go:230`、`internal/cmdrunner/runner_test.go:85`）。
    -   拡張のテストの `parseArticleMarkdown performance` は、絶対値の上限（1000 ms）の確認で、`-race` とは関係がない（`extension/test/articleMarkdown.test.ts:822-840`）。

以上の行番号は、コミット `5bba819a`（`origin/main`）で確かめた。

対応 issue: [#1206](https://github.com/isseis/yt2column/issues/1206)

関連 issue: [#1166](https://github.com/isseis/yt2column/issues/1166)、[#1207](https://github.com/isseis/yt2column/issues/1207)（本 issue を前提とする）、[#1209](https://github.com/isseis/yt2column/issues/1209)（本 issue を前提とする）

**本書の記述範囲:** 本書は、線形時間テストが時間比を判定する条件と、判定しないときに確かめること、`-race` なしの単独の実行の条件、その実行が黙って失われないための条件を定める。判定の切り替え方、ターゲットのレシピ、ガードのテストの作りは `02_architecture.md` で定める。手順と検証の記録は `03_implementation_plan.md` で定める。

**issue #1206 で決めたこと:**

-   前例の `TestParseCopiedLargeInputs` に倣い、時間比の判定を `-race` なしで単独に走るターゲットへ移す。`-race` 付きの実行では、結果と構造の確認だけを残す。
    -   理由: `-race` 付きの全体の実行では、ほかのパッケージとの CPU の競合で計測がぶれる。
-   `test-parse-norace` の `-run` の対象と、CI の同じ手順を合わせて更新する。
-   時間比を判定するほかのテストがあれば、同じ扱いにするか検討する。
-   完了の条件は、`-race` の全体の実行で時間比の判定が行われないこと、`-race` なしの単独のターゲットで判定が行われ、判定を壊すと失敗することを確かめること、`make test`・`make lint`・CI の該当するジョブが通ることである。

**要件定義の際に決めたこと（2026-11-28）:**

-   進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。
    -   理由: 変えるのは 1 つのテスト、`Makefile` の 1 つのターゲット、CI の 1 つの手順、そのガードのテストだけで、製品の振る舞いは変えない。前例のある仕組み（ビルドタグのファイルの定数と `test-parse-norace`）を広げ、テストの環境変数を 1 つ足すだけである。
    -   除外の判断: `docs/dev/security.md` の規則と前提、開発の道具の信頼境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）、外部依存のどれも変えない。CI の変更は既存の手順の名前だけで、アクションも追加・更新しない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   2026-11-28 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる、進め方の選択である。
-   時間比の判定は、`-race` なしのビルドで、テストの環境変数が判定を求めるときだけ行う。その環境変数は `make test-parse-norace` だけが設定する（F-002）。
    -   背景: ビルドタグだけで切り替えると、`-race` なしの全体の実行（1 章「背景」）では、ほかのパッケージと並行に走る中で時間比を判定し続ける。時間比の余裕は約 2 倍しかなく、#1207 の並列化で競合が強まると、そこでも失敗しうる。`make coverage-compare` は、その失敗で止まる。
    -   理由: 判定を、ほかのテストと並行に走らない実行に限るためである。前例の `TestParseCopiedLargeInputs` はビルドタグだけで切り替えるので、この点で前例と違う形になる。
    -   環境変数が判定を求める値でも空でもないとき（たとえば判定を止めるつもりで `0` を設定したとき）、テストはどのビルドでも失敗する（AC-04）。値を読み替えず拒むためで（`CLAUDE.md`「Reject, don't normalize」）、課金のある結合テストの指定（`docs/dev/security.md` 2 章）が `1` 以外を指定なしとして扱うのとは違う。この扱いは人に尋ねていない。PR のレビューで確かめてもらう。
    -   採らなかった案: ビルドタグだけで切り替え、`-race` なしの全体の実行では失敗しうることを既知の制約として記録する案。
    -   2026-11-28 に人が選んだ（3 文書のレビューで見つかった問題について、改めて尋ねた）。
    -   ADR: [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) を更新する（下の ADR の項）。
-   時間比を判定しないとき、線形時間テストは、各入力で `NeutralizedDisplayMarkdown` がエラーを返さないことと出力の契約を確かめる（F-001）。この 2 つを合わせて「成功と出力の契約」と呼ぶ。出力の契約とは、次の 2 つである。出力から挿入した U+200B を除くと表示用の Markdown に戻ること。`Neutralized` が、U+200B を挿入したかどうかを正しく表していること（`internal/publisher/display_render_test.go:30-43` の `checkNeutralizedContract`）。時間比を判定するときも、同じ確認を行う。
    -   理由: issue の「結果と構造の確認だけを残す」に当たる。いまのテストは成功しか確かめていないが、同じファイルにある契約の確認を長い入力にも当てれば、`-race` 付きのビルドでも意味のある確認が残る。
    -   採らなかった案: 成功だけを確かめる案（いまの時間以外の確認をそのまま残すだけになる）。
    -   2026-11-28 に人が選んだ。
    -   ADR: [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) を更新する（下の ADR の項）。
-   `-race` なしで時間を確かめるターゲットの名前は、`test-parse-norace` のままとする。`-run` の対象に線形時間テストを加える（F-002）。`CLAUDE.md` の「Test Commands」の説明は、2 つのテストを走らせることに合わせて書き直す（F-003）。
    -   理由: issue の方針どおりで、変える箇所が少ない。名前に `parse` が残るが、ターゲットの役目は説明に書く。
    -   採らなかった案: `test-timing-norace` のような汎用の名前に変える案（`CLAUDE.md`・ADR-0038・ガードのテスト・CI の手順の名前もまとめて変えることになる）。
    -   2026-11-28 に人が選んだ。
    -   ADR: [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) の「採らなかった案」に記録する。
-   時間比を判定するほかのテストはないので、扱いを変えるのは `TestNeutralizedDisplayMarkdownLinearTime` だけとする（1 章「背景」）。
    -   理由: issue が求めた検討の結果である。タイムアウトの確認は時間比の判定ではなく、拡張の上限の確認は `-race` の実行に含まれない。
    -   ADR: なし。
-   パーサの時間の期限（`TestParseCopiedLargeInputs`）は、いまのままビルドタグだけで切り替える。環境変数による判定の指定は加えない。
    -   理由: issue の範囲は線形時間テストである。パーサの期限は、時間比ではなく絶対値の上限で、全体の実行の競合で失敗した報告はない。この判断は人に尋ねていない。PR のレビューで確かめてもらう。
    -   ADR: なし。ADR-0038 の決定を変えない。
-   [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) を更新する。時間の判定を `-race` なしのビルドだけで行う決定と、`-race` 付きのビルドでも時間以外を確かめ続ける決定を、線形時間テストにも広げる。線形時間テストでは、判定をテストの環境変数の指定に限る。
    -   理由: 時間を確かめるテストを全体の実行から外し、`-race` なしの単独の実行に置く方針は、#1207 と #1209 の並列化を含め、後のタスクを縛る。既存の ADR の決定の範囲を広げる変更なので、`CLAUDE.md`「Design decisions (ADR)」の基準で major に当たる。
    -   2026-11-28 に人が選んだ。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   全体の実行（`-race` 付きと `-race` なし）で、線形時間テストが CPU の競合のために失敗しないようにする。
-   線形時間の性質（入力の長さに比例する時間で処理すること）を、`-race` なしの単独の実行で確かめ続ける。

### 2.2. スコープ (In Scope)

-   `TestNeutralizedDisplayMarkdownLinearTime` が時間比を判定する条件と、判定しないときに確かめること。
-   判定を求めるテストの環境変数。
-   `make test-parse-norace` の対象とその説明。CI の `Test` ジョブでこのターゲットを呼ぶ手順の名前。
-   そのガードのテスト。
-   ADR-0038 と `CLAUDE.md` の「Test Commands」の記述。

### 2.3. スコープ外 (Out of Scope)

-   `NeutralizedDisplayMarkdown` の製品のコード。
-   時間比の判定の閾値（8 倍）、入力の大きさ（2000 単位と 8000 単位）、計測の回数。この変更は判定を走らせる条件だけを変える。
-   `TestParseCopiedLargeInputs` の確認の内容と、その期限を判定する条件（1 章「要件定義の際に決めたこと」）。
-   タイムアウトを確かめるテストと、拡張のテストの時間の上限（1 章「背景」。時間比の判定ではない）。
-   直列のテストの並列化（#1207）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: -race 付きのビルドでの線形時間テスト

`-race` 付きのビルドでは、線形時間テストは時間比を判定せず、成功と出力の契約だけを確かめる。

**Acceptance Criteria**:

-   **AC-01**: `-race` 付きのビルドで `TestNeutralizedDisplayMarkdownLinearTime` を実行すると、テストは時間比を判定しない。判定を求める環境変数（AC-04）を設定しても判定しない。`NeutralizedDisplayMarkdown` の時間が入力の長さの 2 乗に比例するように製品のコードを壊しても、出力が変わらなければ、このビルドではテストが通る。
-   **AC-02**: `-race` 付きのビルドで、テストは 2 つの入力（`open URLs` と `after a '['`）のそれぞれで、成功と出力の契約（1 章「要件定義の際に決めたこと」）を確かめる。契約を破るように製品のコードを壊すと、このビルドでテストが失敗する。

#### F-002: -race なしの単独の実行での時間比の判定

`make test-parse-norace` は、`-race` なしのビルドで、判定を求める環境変数を設定して線形時間テストを走らせ、時間比を判定する。

**Acceptance Criteria**:

-   **AC-03**: `make test-parse-norace` は、`-race` なしのビルドで `TestParseCopiedLargeInputs` と `TestNeutralizedDisplayMarkdownLinearTime` を実行する。線形時間テストは、2 つの入力のそれぞれで時間比が 8 未満であることを判定し、AC-02 と同じ成功と出力の契約も確かめる。時間が入力の長さの 2 乗に比例するように製品のコードを壊すと、このターゲットが失敗する。
-   **AC-04**: 線形時間テストが時間比を判定するのは、`-race` なしのビルドで、判定を求める環境変数が、判定を求める値に設定されているときだけである。
    -   環境変数がないか空のとき、テストは時間比を判定せず、成功と出力の契約だけを確かめる。そのため、`go test -tags test ./...` と `make coverage-compare` の実行では判定しない。AC-01 と同じ壊し方で、`-race` なしのビルドで環境変数なしに実行したテストが通る。
    -   環境変数がそれ以外の値のとき、テストは失敗し、受け付ける値と読んだ値を示す。値を判定を求める値に読み替えたり、判定しないものとして扱ったりしない（`CLAUDE.md`「Reject, don't normalize」）。
    -   環境変数の名前と判定を求める値は、`02_architecture.md` で定める。
-   **AC-05**: `make test-parse-norace` の中で、時間を確かめるテストは、ほかのテストと並行に走らない。2 つのパッケージのテストが同時に走ることもない。このターゲットの `go test` は、`-race` もカバレッジも付けず、キャッシュした結果を使わず（`-count=1`）、`-timeout` を、パーサの 7 つの入力の期限の合計より長く、`go test` の既定の 10 分より短くする（`internal/transcript/makefile_test.go:36-79`）。
-   **AC-06**: 次のどの変更をしても、`make test` で走るガードのテストが失敗する。
    -   `test-parse-norace` のレシピから、線形時間テストを `-run` の対象から外すか、`-run` に 3 つ目の名前を加える。
    -   `./internal/publisher` を、レシピが `go test` に渡すパッケージから外す。
    -   AC-05 の並行に走らない条件を外す。
    -   判定を求める環境変数の設定をレシピから外すか、値を変える。
    -   `TestNeutralizedDisplayMarkdownLinearTime` か `TestParseCopiedLargeInputs` の名前を変え（参照もすべて変える）、レシピをそのままにする。

    また、`make test` はその最後にこのターゲットを呼び、CI の `Test` ジョブは `make test-ci` の後の別の手順でこのターゲットを呼ぶ。手順には、失敗を無視させる `if` や `continue-on-error` を付けない。これらを変えると、ガードのテストが失敗する。この条件も、いまのガードが確かめている条件と同じである（`internal/transcript/makefile_test.go:81-184`）。

#### F-003: 記述の更新

**Acceptance Criteria**:

-   **AC-07**: 次の記述が、変更後の振る舞いに合う。
    -   `CLAUDE.md`「Test Commands」の `make test` と `make test-parse-norace` の項は、このターゲットが 2 つのテストを走らせ、パーサの期限と線形時間の時間比を `-race` なしのビルドで判定することを述べる。
    -   `CLAUDE.md`「Test Commands」の、`-race` なしの `go test` のコマンドもパーサの期限を確かめるという記述は、線形時間の時間比はそこでは判定しないことと合う。
    -   `Makefile` の `test` と `test-parse-norace` のコメントも、このターゲットについて同じことを述べる。
    -   CI の手順の名前は、パーサの期限だけを確かめると読めない名前にする。
    -   ADR-0038 は、1 章の ADR の項のとおりに更新されている。

#### F-004: 全体の確認

**Acceptance Criteria**:

-   **AC-08**: `make test`・`make lint`・CI の `Test` ジョブと `doc-guard` ジョブが通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   `make test-parse-norace` の実行時間の増分は、publisher のテストのビルドと起動、線形時間テストの実行に収める。3 文書のレビューの際の計測（コミット `5bba819a`、12 CPU）では、線形時間テストは約 0.12 秒だった。ビルドのキャッシュがあるとき、2 つのパッケージを `-p 1` で順に走らせた実行の全体は約 1.2〜2.3 秒で、いまのレシピは約 0.75 秒だった。キャッシュが空のときは約 8.1 秒だった。`-timeout` は、いまの範囲の中に保つ（AC-05）。

### 4.2. セキュリティ (Security)

-   N/A。テストと開発用のターゲットだけを変え、信頼できない入力を扱う処理、秘密、ネットワークに触れない。テストの環境変数は、判定するかどうかを選ぶだけで、秘密を持たない。`docs/dev/security.md` の条件付きガイドの起動条件にも当たらない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   全体の実行（`-race` 付きと `-race` なし）の結果が、同時に走るパッケージの数や CPU の負荷によって、線形時間テストの時間比で左右されない（F-001、AC-04）。
-   時間比の判定は、ほかのテストと CPU を取り合わない実行で行う（AC-05）。ただし、手元で `make test` をほかの重い処理と同時に実行したときは、その負荷の影響を防げない（ADR-0038「結果」と同じ）。

### 4.4. 互換性 (Compatibility)

-   N/A。開発の手順だけの変更で、製品とその利用者に影響しない。

### 4.5. 保守性 (Maintainability)

-   時間を確かめるテストを `-race` なしの単独の実行に置く方針を ADR-0038 に記録し、後のタスクがその方針に従えるようにする（AC-07）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR-0038 の決定（時間の判定は `-race` なしのビルドだけで行い、`-race` 付きのビルドでも時間以外を確かめ続ける）に従う。
-   `CLAUDE.md`「Testing Strategy」に従い、時間比の判定と成功と出力の契約の確認が失敗できることを、製品のコードを壊して確かめる（AC-01〜AC-04）。

## 6. 用語集 (Glossary)

-   **時間比:** 線形時間テストが比べる、8000 単位の入力の処理時間と 2000 単位の入力の処理時間の比。
-   **時間比の判定:** 時間比が 8 未満であることを確かめること。
-   **線形時間テスト:** `TestNeutralizedDisplayMarkdownLinearTime`。
-   **成功と出力の契約:** 1 章「要件定義の際に決めたこと」で定めた、`NeutralizedDisplayMarkdown` がエラーを返さないことと、その出力が満たすべき性質。
-   **`-race` 付きのビルド／`-race` なしのビルド:** データ競合の検出器を有効にしたテストのビルドと、有効にしないビルド。ADR-0038 の用語に合わせる。
