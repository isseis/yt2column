# 要件定義書：`make deadcode` の解析の範囲をモジュール全体にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`make deadcode` が到達できない関数だけを報告するようにし、報告があれば失敗させるための要件を定義する（[#361](https://github.com/isseis/yt2column/issues/361)）。

**背景:**

`Makefile` の `deadcode` ターゲットは `deadcode -test -tags test $(MAIN_PKG)` を実行する（`Makefile:244-245`）。`MAIN_PKG` は `./cmd/yt2column` である（`Makefile:14`）。そのため、解析の起点は CLI の `main` パッケージとそのテストだけになる。次のものは実際には使われているのに、到達できない関数として報告される。

-   `cmd/yt2column-server`・`cmd/yt2column-nmhost`・`cmd/fixprguard`・`cmd/mergepr` だけから呼ばれる本番のコード（`internal/config` の `ServerConfig.SocketPath`・`LoadHost`、`internal/nmhost` の `Install`、`internal/fixprguard` など）
-   `internal/transcript/copied*.go`（`CopiedSource` はサーバだけが使う）
-   ほかのパッケージのテストだけが使うテストヘルパー（`internal/*/test_helpers*.go`、`internal/cmdrunner/testutil/mocks.go`）

issue の作成時（`047bcde`）には 196 件だった報告は、本書の作成時（`f8ad397`）には 461 件ある。例は本書の作成時の報告から選んだ（issue が挙げた `LoadServer` は、今は報告されない）。同じ作業ツリーで `deadcode -test -tags test ./...` を実行すると 0 件になる。

このノイズのため、`make deadcode` は使われていない。`runplan.md` の step 6（`.claude/commands/runplan.md:117`）は「`make deadcode` を実行し、このフェーズ群で到達できなくなった関数を削除する」と定めている。しかし、数百件の報告から本当の候補を探すことになり、役に立たない。本当に到達できない関数が加わっても、ノイズに紛れて見逃す。また、`make deadcode` は報告があっても終了コード 0 で終わる。そのため、これまでのタスクは完了条件に使えないとして外してきた（`docs/tasks/0023_non_negative_int_unification/03_implementation_plan.md:45`、`docs/tasks/0033_multiple_trusted_reviewers/03_implementation_plan.md:116`、`docs/tasks/0034_fixprguard_ignored_files/03_implementation_plan.md:97`）。

対応 issue: [#361](https://github.com/isseis/yt2column/issues/361)

**本書の記述範囲:** 本書は、`make deadcode` の観測できる振る舞いを定める。つまり、何を報告し、何を報告せず、どの場合に失敗するかである。レシピの書き方（報告の有無の判定方法、出力の扱い）と、受け入れ基準の確かめ方は、設計（`02_architecture.md`）で決める。要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #361 で決めたこと:**

-   **解析の起点をモジュール全体にする:** `deadcode` ターゲットは `./...` を解析する。どの `main` パッケージから届く関数も、どのパッケージのテストから届く関数も、到達できるものとして数えるためである。
-   **0 件を期待値にする:** 修正後の `main` で報告が 0 件になることを確かめる。また、本当に到達できない関数を加えたときに報告されることを確かめる。

**要件定義の際に決めたこと（2026-10-28）:**

-   **テストを起点に含める（`-test` を残す）:** テストだけが呼ぶ本番のコードは報告しない。本書の作成時に `-test` を外して `deadcode ./...` を実行すると、44 件が報告された。その大半は、後の PR が呼ぶ予定の意図した足場（`internal/articlecache`）と、テストが使う公開関数（`internal/fixprguard` の `StopKinds` など）である。これを報告すると、またノイズになる。
-   **報告が 1 件でもあれば失敗させる:** `make deadcode` は、報告が 1 件以上あれば 0 以外の終了コードで終わる。0 件を期待値にするなら、報告の有無を終了コードで示せば完了条件に使えるからである。`-test` を残すので、テストのある足場は到達できるものとして数えられ、失敗の原因にならない。
-   **CI への組み込みはスコープ外にする:** CI で実行するには、CI の環境に `deadcode` を入れる手順と、その版の固定が要る。issue はそれを求めていないので、本タスクでは手元のターゲットだけを直す。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `make deadcode` が、モジュールのどの `main` パッケージからも、どのテストからも届かない関数だけを報告するようにする。
-   報告の有無を `make deadcode` の終了コードで示し、完了条件に使えるようにする。

### 2.2. スコープ (In Scope)

-   `Makefile` の `deadcode` ターゲットの解析の範囲と終了コード
-   `CLAUDE.md` の「Code Quality」にある `make deadcode` の説明

### 2.3. スコープ外 (Out of Scope)

-   **CI での `make deadcode` の実行:** 1 章「要件定義の際に決めたこと」のとおりである。
-   **`deadcode` の導入手順と版の固定:** 今の `make deadcode` も、`PATH` にある `deadcode` を使う（`Makefile:245`）。版の固定が必要になるのは CI に組み込むときなので、CI への組み込みと一緒に扱う。
-   **テストだけが呼ぶ本番のコードの検出:** 1 章「要件定義の際に決めたこと」のとおり、`-test` を残すので検出しない。
-   **今ある到達できない関数の削除:** 本書の作成時、`deadcode -test -tags test ./...` の報告は 0 件である。削除する対象はない。
-   **`runplan.md` などのコマンド定義の変更:** `runplan.md` の step 6（`.claude/commands/runplan.md:117`）の手順は、本タスクの後もそのまま使える。`make deadcode` を完了条件に加えるかどうかは、計画ごとに決めればよい。
-   **これまでのタスクの文書の記述:** これまでのタスクの文書には、今の解析の範囲を前提にした記述がある（1 章「背景」に挙げたもの、`docs/tasks/0024_copied_transcript_panel_lines/03_implementation_plan.md:492` など）。これらは当時の記録なので改めない。
-   **ブラウザ拡張（`extension/`）のコード:** `deadcode` は Go だけを解析する。`go.mod` の `ignore ./extension`（`go.mod:5`）のとおり、`extension/` は Go のパッケージの一覧に含まれない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 解析の範囲

`make deadcode` は、モジュールのパッケージ（`go list ./...` が列挙するもの）をすべて解析する。いずれかの `main` パッケージから届く関数と、いずれかのパッケージのテストから届く関数は、到達できるものとして数える。テストのコンパイルに使うビルドタグ `test` は、今と同じく有効にする（`Makefile:245`）。

**Acceptance Criteria**:
- **AC-01**: 本タスクの実装を含む作業ツリーで、`make deadcode` を実行すると、到達できない関数の報告が 0 件である。
- **AC-02**: 本番のコードのうち、`cmd/yt2column` 以外の `main` パッケージ（例: `cmd/yt2column-nmhost`）からだけ届く関数（例: `internal/nmhost` の `Install`）を、到達できない関数として報告しない。
- **AC-03**: ビルドタグ `test` の付いたテストヘルパーのうち、ほかのパッケージのテストだけが使う関数（例: `internal/cmdrunner/testutil/mocks.go` の `New`）を、到達できない関数として報告しない。
- **AC-04**: 本番のコードのうち、どの `main` パッケージからも届かず、テストからだけ呼ばれる関数（例: `internal/config` の `Load`）を、到達できない関数として報告しない。
- **AC-05**: `internal/` の下のパッケージに、どこからも呼ばれない関数を加えると、`make deadcode` はその関数を報告する。報告には、その関数のファイルと行と名前が含まれる。
- **AC-06**: `cmd/yt2column` 以外の `main` パッケージ（例: `cmd/yt2column-server`）に、どこからも呼ばれない関数を加えると、`make deadcode` はその関数を報告する。報告の中身は AC-05 と同じである。

#### F-002: 終了コード

`make deadcode` は、報告の有無と解析の成否を終了コードで示す。

**Acceptance Criteria**:
- **AC-07**: 到達できない関数の報告が 0 件で、解析が成功したとき、`make deadcode` は終了コード 0 で終わる。
- **AC-08**: 到達できない関数の報告が 1 件以上あるとき、`make deadcode` は 0 以外の終了コードで終わる。このとき、報告はすべて端末に表示される。
- **AC-09**: 解析そのものが失敗したとき（例: パッケージのコンパイルエラー、`PATH` に `deadcode` がない）、`make deadcode` は 0 以外の終了コードで終わり、`deadcode` が出したエラーのメッセージが端末に表示される。報告が 0 件であるかのように終了コード 0 で終わることはない。

#### F-003: 開発者向けの説明

**Acceptance Criteria**:
- **AC-10**: `CLAUDE.md` の「Code Quality」にある `make deadcode` の説明が、解析の範囲（モジュール全体と、そのテスト）と、報告が 1 件以上あれば失敗することを述べている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   要件はない。本書の作成時、`deadcode -test -tags test ./...` は手元の macOS で約 1 秒で終わった。開発者が手で実行するターゲットであり、この時間は問題にならない。

### 4.2. セキュリティ (Security)

-   N/A。本タスクは開発用のターゲットだけを変える。外部コマンド `yt-dlp` を呼ばず、API キーと Webhook URL を扱わず、キャッシュのディレクトリを読み書きせず、ネットワークにデータを送らず、信頼できないテキストをプロンプトや記事に埋め込まない（`docs/dev/security.md` を適用する条件に当たらない）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   F-002 のとおり、解析の失敗を報告 0 件と取り違えない。

### 4.4. 互換性 (Compatibility)

-   macOS と Linux で動作する。手元の GNU Make 3.81 でも、CI の GNU Make 4.x でも同じ振る舞いになる。
-   Go のターゲットは Node.js を必要としない。`extension/test/repository.test.ts:230-258` は、`node` と `npm` のない `PATH` で `make -n build test lint deadcode` を実行し、終了コード 0、標準エラーが空、出力に `node`・`npm`・`npx` が現れないことを確かめている。本タスクの後も、この条件を満たす。

### 4.5. 保守性 (Maintainability)

-   新しい `main` パッケージやパッケージを加えても、`deadcode` ターゲットを変えずに解析の範囲に入る。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   新しい外部のツールやモジュールを加えない。`deadcode` は今と同じく `PATH` にあるものを使う。

## 6. 用語集 (Glossary)

-   **到達できない関数（unreachable func）:** 解析の起点（`main` パッケージと、`-test` のときはテスト）から、呼び出しの連鎖でたどり着けない関数。`deadcode` の報告の 1 行が 1 つの関数に当たる。
-   **報告:** `deadcode` が到達できない関数ごとに出す行（`<file>:<line>:<col>: unreachable func: <name>`）。
-   **意図した足場:** 後のフェーズや PR が呼ぶ予定で、今はまだ本番のコードから呼ばれない関数（`runplan.md` の step 6 の「intentional scaffolding」）。
