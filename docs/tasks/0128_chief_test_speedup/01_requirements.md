# 要件定義書：internal/chief のテストを並列に実行して速くする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-26 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`internal/chief` パッケージのテストが確かめる範囲を保ったまま、その実行時間を短くする要件を定める。

**背景:**

-   `internal/chief` のテストは、どれも `t.Parallel()` を呼ばない（`internal/chief/*_test.go` を `grep` して 0 件）。そのため、テストとサブテストは 1 つずつ順に実行される。
-   issue #1169 は、`go test -tags test -race -count=1 ./...` の計測で、`internal/chief` の壁時計の時間が 210.4 秒（全パッケージの中で 3 位）だったと報告している。この計測は、ほかのパッケージのテストと同時に走らせたときのものである。
-   本書を書く際に、このパッケージだけを手元で計測した（2026-11-26、12 CPU の macOS、コミット `6c60f9f8`、`go test -tags test -race -count=1 -json ./internal/chief`）。
    -   壁時計の時間は 123 秒、CPU の使用率は 41% だった。`go test -json` がパッケージの終わりに報告する経過時間（`Elapsed`）は 118.3 秒だった。
    -   ファイルごとの時間の合計（トップレベルのテストの `Elapsed` の和）は、多い順に次のとおりである。`transcript_test.go` 22.3 秒、`markers_parity_test.go` 16.2 秒、`autosend_test.go` 13.0 秒、`removal_test.go` 7.2 秒、`reset_test.go` 6.4 秒、`snapshot_test.go` 6.0 秒。残りの約 50 秒は、ほかの 30 余りのファイルに散らばっている。
    -   `transcript_test.go` の時間は、ほぼすべてが `TestReadClaudeReport` の 1 つの親テストのものである（22.3 秒。サブテストの合計は約 0.5 秒）。
-   `markers_parity_test.go` の `TestMarkerReadingMatchesDecisionScript` は、23 のサブテストのそれぞれで、git のリポジトリを作り、2 回コミットし、`bash` を起動する（`internal/chief/markers_parity_test.go:91-105`）。リポジトリは `newTestRepo` が作る（`internal/chief/test_helpers.go:107-126`）。
-   Go のテストは、`t.Setenv` を呼んだテストとその親が `t.Parallel()` を呼ぶと panic する。`internal/chief` で `t.Setenv` を呼ぶのは次のテストである。
    -   `repo.RealCommands()` を使うテスト（`RealCommands` が `t.Setenv("PATH", …)` を呼ぶ。`internal/chief/test_helpers.go:134-137`）。
    -   `network_test.go` の 4 つのテスト（`internal/chief/network_test.go:20`・`:66`・`:141`・`:194`）。
    -   `TestGitClean`（`internal/chief/autosend_test.go:1078-1084`）。

以上の行番号は、コミット `6c60f9f8`（`origin/main`）で確かめた。

対応 issue: [#1169](https://github.com/isseis/yt2column/issues/1169)

**本書の記述範囲:** 本書は、実行時間の目標、テストが確かめる範囲を保つ条件、安定して通る条件を定める。どのテストを並列にするか、変更前と変更後をどう比べるかは、`02_architecture.md` で定める。手順と検証の記録は `03_implementation_plan.md` で定める。

**issue #1169 で決めたこと:**

-   **対象は `internal/chief` パッケージのテストである。** ほかのパッケージは対象にしない。
-   **テストが確かめる範囲を保つ。** 速くするために、テストを消したり、確かめる内容を減らしたりしない。
-   **`make test` と `make lint` が通る。**

**要件定義の際に決めたこと（2026-11-26）:**

-   **進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。**
    -   理由: 変えるのは 1 つのパッケージのテストのコードだけで、製品の振る舞いは変えない。既存の ADR の決定も変えない。新しい仕組みも作らない。
    -   除外の判断: `docs/dev/security.md` の規則と前提、開発の道具の信頼境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）、外部依存のどれも変えない。`fixprguard` と `mergepr` には触れず、chief の振る舞いも変えない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   2026-11-26 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる、進め方の選択である。
-   **実行時間の目標は、変更前の 50% 以下とする（F-001）。** 同じ機械で、同じコマンド（`go test -tags test -race -count=1 -json ./internal/chief`）を変更前と変更後に続けて実行し、パッケージの経過時間（`Elapsed`）を比べる。
    -   理由: 時間の値そのものは機械と負荷で変わる。同じ機械で続けて測った比なら、機械に依存せずに判定できる。上の計測では、パッケージ全体を並列にすれば、最も長い 1 つのテスト（約 22 秒）が時間の下限になる。並列化だけで届く見込みがある。
    -   採らなかった案: 変更前の 30% 以下とする案（フィクスチャの高速化も要る見込みが高い）。数値の基準を置かず、計測を記録するだけにする案（完了の判定が人に任される）。
    -   2026-11-26 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる。
-   **テストが確かめる範囲を保つことは、実行されて PASS したテストとサブテストの名前の集合が、変更前と変更後で同じであることで確かめる（F-002）。**
    -   理由: 並列化は、テストを消さず、名前も変えない。名前の集合が同じなら、テストが減っていないことを機械的に確かめられる。
    -   採らなかった案: 加えて `make coverage-compare` で関数ごとのカバレッジを比べる案（テストを消さない変更には、名前の集合で足り、比較に数分かかる）。
    -   2026-11-26 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる。
-   **並列化で不安定にならないことは、`go test -tags test -race -count=3 ./internal/chief` が通ることで確かめる（F-003）。**
    -   理由: 共有の状態によるデータ競合と、実行の重なり方で結果が変わる失敗を、`-race` と複数回の実行で見つけるためである。
    -   採らなかった案: 加えて `-shuffle=on` で実行の順序を変える案。`make test` が通るだけにする案。
    -   2026-11-26 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる。
-   **変える範囲は、`internal/chief` のテストのファイルとテストのヘルパーに限る（F-004）。** 製品のコード（`internal/chief` の `_test.go` でもテストのヘルパーでもないファイル）は変えない。
    -   理由: 製品の振る舞いを変えずに速くするためである。テストのために製品のコードを変えると、それ自体が確かめる対象になる。
    -   ADR: なし。本タスクの中で閉じる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `internal/chief` のテストの実行時間を、変更前の半分以下にする。
-   テストが確かめる範囲を変えない。
-   並列に実行しても、テストが安定して通るようにする。

### 2.2. スコープ (In Scope)

-   `internal/chief` のテスト（`_test.go`）と、テストのヘルパー（`//go:build test` の `test_helpers.go`）の変更。
-   変更前と変更後の比較に使う一時スクリプト（本タスクの `_checks/`）。

### 2.3. スコープ外 (Out of Scope)

-   **ほかのパッケージのテスト。** issue の対象ではない。
-   **`make test` 全体の時間の目標。** `make test` はすべてのパッケージを同時に走らせるので、その時間は、このパッケージの時間だけでは決まらない。issue の完了の条件は、このパッケージの時間である。
-   **製品のコードの変更。** F-004 のとおりである。
-   **新しいテストの追加と、既存のテストの確かめる内容の変更。** issue は求めていない。
-   **`t.Parallel()` を呼ばないテストを見つけるリンタ（`paralleltest`・`tparallel` など）の導入。** issue は求めていない。リポジトリ全体の規則になり、このタスクの範囲を超える。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 実行時間を変更前の半分以下にする

**Acceptance Criteria**:

-   **AC-01**: 同じ機械で、`go test -tags test -race -count=1 -json ./internal/chief` を、マージベースの木（変更前）と PR の木（変更後）で続けて実行する。変更後のパッケージの経過時間（パッケージの `pass` の行の `Elapsed`）が、変更前の経過時間の 50% 以下である。
-   **AC-02**: AC-01 の計測の結果（変更前と変更後の経過時間、その比、計測したコミット）が、実装の計画書の検証の記録と、PR の説明に記録されている。

#### F-002: テストが確かめる範囲を保つ

**Acceptance Criteria**:

-   **AC-03**: AC-01 の 2 回の実行で、`pass` になったテストとサブテストの名前の集合が同じである。どちらの実行にも、`fail` か `skip` になったテストとサブテストはない。

#### F-003: 並列に実行しても安定して通る

**Acceptance Criteria**:

-   **AC-04**: PR の木で、`go test -tags test -race -count=3 ./internal/chief` が終了コード 0 で終わる。

#### F-004: 製品のコードを変えない

**Acceptance Criteria**:

-   **AC-05**: マージベースと PR の木の差分の、変わったファイルの一覧が、`internal/chief` の `_test.go` のファイル、`internal/chief/test_helpers.go`、本タスクのディレクトリ（`docs/tasks/0128_chief_test_speedup/`）の下のファイルに限られる。

#### F-005: 既存の確認が通る

**Acceptance Criteria**:

-   **AC-06**: PR の木で、`make test` と `make lint` が通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   F-001 のとおりである。

### 4.2. セキュリティ (Security)

-   N/A。`docs/dev/security.md` の条件（外部コマンド `yt-dlp` の起動、API キー、キャッシュのディレクトリ、ネットワークへの送信、信頼しないテキストの埋め込み）のどれにも当たらない。変えるのはテストの実行の仕方だけである。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   F-003 のとおりである。

### 4.4. 互換性 (Compatibility)

-   N/A。製品のコードと、テストを実行するコマンド（`make test` などの Makefile のターゲット）を変えない。

### 4.5. 保守性 (Maintainability)

-   テストが本来の理由で失敗できることを変えない。並列化はテストの実行の仕方を変えるだけで、テストの判定を変えない（F-002）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md`「Testing Strategy」と `docs/dev/conventions/test_organization.md` に従う。
-   一時スクリプトは、`docs/dev/conventions/requirements_process.md`「Temporary scripts (`_checks/`)」に従う。

## 6. 用語集 (Glossary)

-   **変更前・変更後:** 変更前はマージベース（`git merge-base origin/main HEAD`）の木、変更後は PR の木である。
-   **パッケージの経過時間:** `go test -json` がパッケージの終わりに出す `pass` の行（`Test` を持たない行）の `Elapsed` の値（秒）である。テストのバイナリの実行の時間で、ビルドの時間は含まない。
-   用語は `docs/translation_glossary.md` と統一する。
