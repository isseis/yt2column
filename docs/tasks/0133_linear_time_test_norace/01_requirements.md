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

**目的:** 本書は、`internal/publisher` の線形時間テスト `TestNeutralizedDisplayMarkdownLinearTime` が、`-race` 付きの全体の実行で CPU の競合のために失敗する問題をなくす要件を定める。時間比の判定は、`-race` なしの単独の実行に移す。

**背景:**

-   `TestNeutralizedDisplayMarkdownLinearTime` は、入力を 4 倍にしたときの処理時間の比が 8 未満であることを、6 回の計測の最良値で確かめる。時間のほかに確かめるのは、`NeutralizedDisplayMarkdown` がエラーを返さないことだけである（`internal/publisher/display_render_test.go:233-265`）。
-   このテストは、`make test` と CI の `Test` ジョブの `-race` 付きの全体の実行（`make test-ci`）の中でだけ走る（`Makefile:93-100`）。issue #1206 は、5bba819a で `go test -tags test -race -count=1 ./...` を実行したとき、`TestNeutralizedDisplayMarkdownLinearTime/open_URLs` が `ratio 11.3, want under 8` で失敗したと報告している。ほかのパッケージはすべて通った。
-   原因は、#1166 で並列化が進み、同時に走るパッケージとテストが増えたことによる CPU の競合である（issue #1206「原因」）。#1207 で直列のテストの並列化を進めると、競合はさらに強まる。
-   前例として、コピーした字幕のパーサの時間の期限がある。期限は `-race` なしのビルドだけで確かめ、`-race` 付きのビルドでは時間以外を確かめ続ける（[ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md)）。その実行は `make test-parse-norace` で、`TestParseCopiedLargeInputs` だけを `-race` なしで走らせる（`Makefile:111-112`）。`make test` はその `-race` 付きの実行の後にこのターゲットを呼ぶ（`Makefile:93-95`）。CI の `Test` ジョブも別の手順として呼ぶ（`.github/workflows/ci.yml:168-169`）。ターゲットの引数、`make test` からの呼び出し、CI の手順は、`internal/transcript/makefile_test.go` のガードのテストが固定している。
-   時間比を判定するテストは、リポジトリにほかにない。
    -   Go のテストのほかの `time.Since` は、待ち時間の上限（タイムアウト）の確認である（例: `internal/llm/llmhttp/llmhttp_test.go:230`、`internal/cmdrunner/runner_test.go:85`）。
    -   拡張のテストの `parseArticleMarkdown performance` は、絶対値の上限（1000 ms）の確認で、`-race` とは関係がない（`extension/test/articleMarkdown.test.ts:822-840`）。

以上の行番号は、コミット `5bba819a`（`origin/main`）で確かめた。

対応 issue: [#1206](https://github.com/isseis/yt2column/issues/1206)

関連 issue: [#1166](https://github.com/isseis/yt2column/issues/1166)、[#1207](https://github.com/isseis/yt2column/issues/1207)（本 issue を前提とする）、[#1209](https://github.com/isseis/yt2column/issues/1209)（本 issue を前提とする）

**本書の記述範囲:** 本書は、`-race` 付きの実行と `-race` なしの実行のそれぞれで線形時間テストが確かめること、`-race` なしの単独の実行の条件、その実行が黙って失われないための条件を定める。ビルドタグによる切り替え方、ターゲットのレシピ、ガードのテストの作りは `02_architecture.md` で定める。手順と検証の記録は `03_implementation_plan.md` で定める。

**issue #1206 で決めたこと:**

-   前例の `TestParseCopiedLargeInputs` に倣い、時間比の判定を `-race` なしで単独に走るターゲットへ移す。`-race` 付きの実行では、結果と構造の確認だけを残す。
    -   理由: `-race` 付きの全体の実行では、ほかのパッケージとの CPU の競合で計測がぶれる。
-   `test-parse-norace` の `-run` の対象と、CI の同じ手順を合わせて更新する。
-   時間比を判定するほかのテストがあれば、同じ扱いにするか検討する。
-   完了の条件は、`-race` の全体の実行で時間比の判定が行われないこと、`-race` なしの単独のターゲットで判定が行われ、判定を壊すと失敗することを確かめること、`make test`・`make lint`・CI の該当するジョブが通ることである。

**要件定義の際に決めたこと（2026-11-28）:**

-   進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。
    -   理由: 変えるのは 1 つのテスト、`Makefile` の 1 つのターゲット、CI の 1 つの手順、そのガードのテストだけで、製品の振る舞いは変えない。前例のある仕組み（ビルドタグのファイルの定数と `test-parse-norace`）を広げるだけで、新しい仕組みは作らない。
    -   除外の判断: `docs/dev/security.md` の規則と前提、開発の道具の信頼境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）、外部依存のどれも変えない。CI の変更は既存の手順の名前だけで、アクションも追加・更新しない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   2026-11-28 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる、進め方の選択である。
-   `-race` 付きのビルドでは、線形時間テストは、各入力で `NeutralizedDisplayMarkdown` がエラーを返さないことと、出力の契約を確かめる（F-001）。出力の契約とは、出力から挿入した U+200B を除くと表示用の Markdown に一致し、`Neutralized` が U+200B を挿入したかどうかに一致することである（`internal/publisher/display_render_test.go:30-43` の `checkNeutralizedContract`）。時間は測らない。
    -   理由: issue の「結果と構造の確認だけを残す」に当たる。いまのテストは成功しか確かめていないが、同じファイルにある契約の確認を長い入力にも当てれば、`-race` 付きのビルドでも意味のある確認が残る。
    -   採らなかった案: 成功だけを確かめる案（いまの時間以外の確認をそのまま残すだけになる）。
    -   2026-11-28 に人が選んだ。
    -   ADR: [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) を更新する（下の ADR の項）。
-   `-race` なしで時間を確かめるターゲットの名前は、`test-parse-norace` のままとする。`-run` の対象に線形時間テストを加える（F-002）。`CLAUDE.md` の「Test Commands」の説明は、2 つのテストを走らせることに合わせて書き直す（F-003）。
    -   理由: issue の方針どおりで、変える箇所が少ない。名前に `parse` が残るが、ターゲットの役目は説明に書く。
    -   採らなかった案: `test-timing-norace` のような汎用の名前に変える案（`CLAUDE.md`・ADR-0038・ガードのテスト・CI の手順の名前もまとめて変えることになる）。
    -   2026-11-28 に人が選んだ。
    -   ADR: なし。名前を変えないので、既存の決定は変わらない。
-   時間比を判定するほかのテストはないので、扱いを変えるのは `TestNeutralizedDisplayMarkdownLinearTime` だけとする（1 章「背景」）。
    -   理由: issue が求めた検討の結果である。タイムアウトの確認は時間比の判定ではなく、拡張の上限の確認は `-race` の実行に含まれない。
    -   ADR: なし。
-   [ADR-0038](../../dev/adr/0038-deadcode-and-race-free-timing.md) を更新する。「パーサの時間の期限は `-race` なしのビルドだけで確かめる」と「`-race` 付きのビルドでも、時間以外は確かめ続ける」を、線形時間テストの時間比にも広げる。
    -   理由: 時間を確かめるテストを `-race` 付きの全体の実行から外し、`-race` なしの単独の実行に置く方針は、#1207 と #1209 の並列化を含め、後のタスクを縛る。既存の ADR の決定の範囲を広げる変更なので、`CLAUDE.md`「Design decisions (ADR)」の基準で major に当たる。
    -   2026-11-28 に人が選んだ。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `-race` 付きの全体の実行で、線形時間テストが CPU の競合のために失敗しないようにする。
-   線形時間の性質（入力の長さに比例する時間で処理すること）を、`-race` なしの単独の実行で確かめ続ける。

### 2.2. スコープ (In Scope)

-   `TestNeutralizedDisplayMarkdownLinearTime` の、`-race` 付きのビルドと `-race` なしのビルドでの確認の内容。
-   `make test-parse-norace` の対象とその説明。CI の `Test` ジョブでこのターゲットを呼ぶ手順の名前。
-   そのガードのテスト。
-   ADR-0038 と `CLAUDE.md` の「Test Commands」の記述。

### 2.3. スコープ外 (Out of Scope)

-   `NeutralizedDisplayMarkdown` の製品のコード。
-   時間比の判定の閾値（8 倍）、入力の大きさ（2000 単位と 8000 単位）、計測の回数。この変更は判定を走らせる場所だけを変える。
-   `TestParseCopiedLargeInputs` の確認の内容。
-   タイムアウトを確かめるテストと、拡張のテストの時間の上限（1 章「背景」。時間比の判定ではない）。
-   直列のテストの並列化（#1207）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: -race 付きのビルドでの線形時間テスト

`-race` 付きのビルドでは、線形時間テストは時間比を判定せず、結果と出力の契約だけを確かめる。

**Acceptance Criteria**:

-   **AC-01**: `-race` 付きのビルドで `TestNeutralizedDisplayMarkdownLinearTime` を実行すると、テストは時間比を判定しない。`NeutralizedDisplayMarkdown` の時間が入力の長さの 2 乗に比例するように製品のコードを壊しても、出力が変わらなければ、このビルドではテストが通る。
-   **AC-02**: `-race` 付きのビルドで、テストは 2 つの入力（`open URLs` と `after a '['`）のそれぞれで、`NeutralizedDisplayMarkdown` がエラーを返さないことと出力の契約（1 章「要件定義の際に決めたこと」）を確かめる。契約を破るように製品のコードを壊すと、このビルドでテストが失敗する。

#### F-002: -race なしの単独の実行での時間比の判定

`make test-parse-norace` は、`-race` なしのビルドで、線形時間テストの時間比を判定する。

**Acceptance Criteria**:

-   **AC-03**: `make test-parse-norace` は、`-race` なしのビルドで `TestParseCopiedLargeInputs` と `TestNeutralizedDisplayMarkdownLinearTime` を実行する。線形時間テストは、2 つの入力のそれぞれで時間比が 8 未満であることを判定し、AC-02 と同じ結果と契約の確認も行う。時間が入力の長さの 2 乗に比例するように製品のコードを壊すと、このターゲットが失敗する。
-   **AC-04**: `make test-parse-norace` の中で、時間を確かめるテストは、ほかのテストと並行に走らない。2 つのパッケージのテストが同時に走ることもない。そのほかの実行の条件（`-race` もカバレッジも付けないこと、キャッシュした結果を使わないこと、`-timeout` の範囲）は、いまの `test-parse-norace` のとおりとする（`internal/transcript/makefile_test.go:37-84`）。
-   **AC-05**: 次のどの変更をしても、`make test` で走るガードのテストが失敗する。
    -   `test-parse-norace` のレシピから、線形時間テストを `-run` の対象から外す。
    -   `./internal/publisher` をパッケージから外す。
    -   AC-04 の並行に走らない条件を外す。
    -   `TestNeutralizedDisplayMarkdownLinearTime` か `TestParseCopiedLargeInputs` の名前を変え、レシピをそのままにする。

    `make test` の最後にこのターゲットを呼ぶことと、CI の `Test` ジョブが `make test-ci` の後の別の手順でこのターゲットを呼ぶことは、いまのガードのとおり固定されたままとする（`internal/transcript/makefile_test.go:86-176`）。

#### F-003: 記述の更新

**Acceptance Criteria**:

-   **AC-06**: 次の記述が、変更後の振る舞いに合う。
    -   `CLAUDE.md`「Test Commands」の `make test` と `make test-parse-norace` の項は、このターゲットが 2 つのテストを走らせ、パーサの期限と線形時間の時間比を `-race` なしのビルドだけで確かめることを述べる。
    -   `Makefile` の `test` と `test-parse-norace` のコメントも、同じことを述べる。
    -   CI の手順の名前は、パーサの期限だけを確かめると読めない名前にする。
    -   ADR-0038 は、1 章の ADR の項のとおりに更新されている。

#### F-004: 全体の確認

**Acceptance Criteria**:

-   **AC-07**: `make test`・`make lint`・CI の `Test` ジョブと `doc-guard` ジョブが通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   `make test-parse-norace` の実行時間の増分は、線形時間テストの実行時間（`-race` なしで数秒以内）に収める。`-timeout` は、いまの範囲の中に保つ（AC-04）。

### 4.2. セキュリティ (Security)

-   N/A。テストと開発用のターゲットだけを変え、信頼できない入力を扱う処理、秘密、ネットワークに触れない。`docs/dev/security.md` の条件付きガイドの起動条件にも当たらない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   `-race` 付きの全体の実行の結果が、同時に走るパッケージの数や CPU の負荷に左右されない（F-001）。
-   時間比の判定は、ほかのテストと CPU を取り合わない実行で行う（AC-04）。ただし、手元で `make test` をほかの重い処理と同時に実行したときの負荷は防がない（ADR-0038「結果」と同じ）。

### 4.4. 互換性 (Compatibility)

-   N/A。開発の手順だけの変更で、製品とその利用者に影響しない。

### 4.5. 保守性 (Maintainability)

-   時間を確かめるテストを `-race` なしの単独の実行に置く方針を ADR-0038 に記録し、後のタスクがその方針に従えるようにする（AC-06）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR-0038 の決定（時間の期限は `-race` なしのビルドだけで確かめ、`-race` 付きのビルドでも時間以外を確かめ続ける）に従う。
-   `CLAUDE.md`「Testing Strategy」に従い、時間比の判定と結果の確認が失敗できることを、製品のコードを壊して確かめる（AC-01〜AC-03）。

## 6. 用語集 (Glossary)

-   **時間比:** 線形時間テストが比べる、8000 単位の入力の処理時間と 2000 単位の入力の処理時間の比。
-   **線形時間テスト:** `TestNeutralizedDisplayMarkdownLinearTime`。
-   **出力の契約:** 1 章「要件定義の際に決めたこと」で定めた、`NeutralizedDisplayMarkdown` の出力が満たすべき性質。
-   **`-race` 付きのビルド／`-race` なしのビルド:** データ競合の検出器を有効にしたテストのビルドと、しないビルド。ADR-0038 の用語に合わせる。
