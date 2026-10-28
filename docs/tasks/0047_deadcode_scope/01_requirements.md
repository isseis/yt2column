# 要件定義書：`make deadcode` の解析の範囲をモジュール全体にする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-28 |
| Review date | 2026-10-28 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`make deadcode` が本当に到達できない関数だけを報告し、報告があれば失敗するようにするための要件を定義する（[#361](https://github.com/isseis/yt2column/issues/361)）。

**背景:**

`Makefile` の `deadcode` ターゲットは `deadcode -test -tags test $(MAIN_PKG)` を実行する（`Makefile:244-245`）。`MAIN_PKG` は `./cmd/yt2column` である（`Makefile:14`）。そのため、解析の起点は CLI の `main` パッケージとそのテストだけになる。次の関数は実際には使われているが、起点が CLI に限られるため、到達できない関数として報告される。

-   本番のコードのうち、CLI からは届かず、ほかの `main` パッケージ（`cmd/yt2column-server`・`cmd/yt2column-nmhost`・`cmd/fixprguard`・`cmd/mergepr`）かそのパッケージ自身のテストから届く関数（`internal/config` の `ServerConfig.SocketPath`・`LoadHost`、`internal/nmhost` の `Install`、`internal/transcript` の `NewCopiedSource`、`internal/fixprguard` の関数など）
-   ほかのパッケージのテストだけが使うテストヘルパー（`internal/*/test_helpers*.go`、`internal/cmdrunner/testutil/mocks.go` など）

報告は、issue の作成時（`047bcde`）には 196 件だったが、本書の作成時（`f8ad397`）には 461 件になっている。上の例は、本書の作成時の報告から選んだ。同じ作業ツリーで `deadcode -test -tags test ./...` を実行すると、報告は 0 件になる。

このノイズのため、`make deadcode` は役に立っていない。`runplan.md` の step 6（`.claude/commands/runplan.md:117`）は「`make deadcode` を実行し、このフェーズ群で到達できなくなった関数を削除する」と定める。しかし実際には、数百件の報告から本当の候補を探すことになる。本当に到達できない関数が加わっても、ノイズに紛れて見逃す。また、`make deadcode` は報告があっても終了コード 0 で終わる。そのため、これまでのタスクは、`make deadcode` を完了条件に使えないものとして、完了条件から外してきた（`docs/tasks/0023_non_negative_int_unification/03_implementation_plan.md:45`、`docs/tasks/0033_multiple_trusted_reviewers/03_implementation_plan.md:116`、`docs/tasks/0034_fixprguard_ignored_files/03_implementation_plan.md:97`）。

対応 issue: [#361](https://github.com/isseis/yt2column/issues/361)

**本書の記述範囲:** 本書は、`make deadcode` の観測できる振る舞いを定める。何を報告し、何を報告せず、どの場合に失敗し、何を表示するかである。レシピの書き方（報告の有無の判定方法、出力の扱い）と、受け入れ基準の確かめ方は、設計（`02_architecture.md`）で決める。要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #361 で決めたこと:**

-   **解析の起点をモジュール全体にする:** `deadcode` ターゲットは `./...` を解析する。どの `main` パッケージから届く関数も、どのパッケージのテストから届く関数も、到達できると数えるためである。
-   **0 件を期待値にする:** 修正後の `main` で報告が 0 件になることを確かめる。また、本当に到達できない関数を加えたときに、それが報告されることを確かめる。

**要件定義の際に決めたこと（2026-10-28）:**

-   **テストを起点に含める（`-test` を残す）:** テストだけが呼ぶ本番のコードは報告しない。`-test` を外すと、また多くのノイズが出るからである。本書の作成時（`f8ad397`）の測定は次のとおりである。
    -   今のレシピから `-test` だけを外した `deadcode -tags test ./...` は、422 件を報告した。ビルドタグ `test` の付いたテストヘルパーが、到達できないと数えられるためである。
    -   タグも外した `deadcode ./...` は、44 件を報告した。その大半は、後の PR が呼ぶ予定の意図した足場（6 章で定義。`internal/articlecache`）と、テストが使う公開関数（`internal/fixprguard` の `StopKinds` など）である。
-   **ビルドタグ `integration` も有効にする:** 解析には、ビルドタグ `test` と `integration` の両方を使う。`make lint` と同じ組み合わせである（`Makefile:10` の `--build-tags test,integration`）。こうすると、`//go:build integration` のファイル（`internal/llm/claude/testutil/integration.go` など）の中の到達できない関数も報告の対象になり、統合テストから届く関数は到達できると数えられる。本書の作成時、`deadcode -test -tags test,integration ./...` の報告は 0 件だった。
-   **報告が 1 件でもあれば失敗させる:** `make deadcode` は、報告が 1 件以上あれば 0 以外の終了コードで終わる。0 件を期待値にする以上、報告の有無を終了コードで示せば、完了条件に使えるからである。`-test` を残すので、テストから届く足場は到達できると数えられ、失敗の原因にならない。テストから届かない足場は失敗の原因になる。そのような足場は、テストを加えて届くようにするか、使う PR まで加えない。
-   **`runplan.md` の step 6 を改める:** 上の決定に合わせて、step 6 に次のことを書く。意図した足場を残すならテストから届くようにすること、`make deadcode` が通ること。今の step 6 は「意図した足場を残す」としか書いていない。そのため、テストから届かない足場を残して `make deadcode` が失敗したときに、どう扱えばよいかが読み取れないからである。
-   **`deadcode` がないときの表示を gofumpt に揃える:** `PATH` に `deadcode` がないとき、`deadcode` が見つからないこととインストール方法を表示して失敗する。`gofumpt` がないときの表示（`Makefile:20-26` の `check_gofumpt`）と同じ形にする。
-   **`CLAUDE.md` の説明を改める:** `CLAUDE.md` の「Code Quality」の `make deadcode` の説明は「Report unreachable functions」だけである。解析の範囲と終了コードが変わるので、それを述べるように改める。
-   **CI への組み込みはスコープ外にする:** CI で実行するには、CI の環境に `deadcode` を入れる手順と、その版の固定が要る。issue はそれを求めていないので、本タスクでは手元のターゲットだけを直す。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `make deadcode` が、モジュールのどの `main` パッケージからも、どのテストからも届かない関数だけを報告するようにする。
-   報告の有無を `make deadcode` の終了コードで示し、完了条件に使えるようにする。

### 2.2. スコープ (In Scope)

-   `Makefile` の `deadcode` ターゲットの解析の範囲、終了コード、表示
-   `CLAUDE.md` の「Code Quality」にある `make deadcode` の説明
-   `.claude/commands/runplan.md` の step 6

### 2.3. スコープ外 (Out of Scope)

-   **CI での `make deadcode` の実行:** 1 章「要件定義の際に決めたこと」のとおりである。
-   **`deadcode` の版の固定:** 今の `make deadcode` も、`PATH` にある `deadcode` を使う（`Makefile:245`）。版の固定が必要になるのは CI に組み込むときなので、CI への組み込みと一緒に扱う。
-   **テストだけが呼ぶ本番のコードの検出:** 1 章「要件定義の際に決めたこと」のとおり、`-test` を残すので検出しない。
-   **今ある到達できない関数の削除:** 本書の作成時、`deadcode -test -tags test,integration ./...` の報告は 0 件であり、削除する対象はない。
-   **`runplan.md` の step 6 以外のコマンド定義:** `.claude/commands/_context.md:168`（「Dead-code check」）と `.claude/commands/weakreview.md:8` は `make deadcode` の名前を挙げるだけで、その振る舞いを述べていない。そのため、本タスクの後も記述は正しい。
-   **これまでのタスクの文書の記述:** これまでのタスクの文書には、今の解析の範囲を前提にした記述がある（1 章「背景」に挙げたもの、`docs/tasks/0024_copied_transcript_panel_lines/03_implementation_plan.md:492` など）。これらは当時の記録なので改めない。
-   **ブラウザ拡張（`extension/`）のコード:** `deadcode` は Go だけを解析する。`go.mod` の `ignore ./extension`（`go.mod:5`）のとおり、`extension/` は Go のパッケージの一覧に含まれない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 解析の範囲

`make deadcode` は、ビルドタグ `test` と `integration` を有効にしたときのモジュールのパッケージ（`go list -tags test,integration ./...` が列挙するもの）をすべて解析する。いずれかの `main` パッケージから届く関数と、いずれかのパッケージのテストから届く関数は、到達できると数える。

`go list` が列挙しないディレクトリ（名前が `_` か `.` で始まるもの、`testdata`）は解析の起点にならない。そこにあるプログラム（例: `docs/tasks/0018_article_quality/_material_check/main.go`）からだけ届く関数は、到達できない関数として報告される。解析は、実行した環境の OS とアーキテクチャ向けのビルドだけを対象にし、ビルドタグ `race` は有効にしない。

**Acceptance Criteria**:
- **AC-01**: 本タスクの実装を含む作業ツリーで `make deadcode` を実行すると、到達できない関数の報告は 0 件である。
- **AC-02**: `internal/` の下のパッケージに関数を加え、`cmd/yt2column` 以外の `main` パッケージの本番のコードからだけ呼ぶ。この関数にはテストから届かず、今のレシピ（`deadcode -test -tags test ./cmd/yt2column`）では報告される。この状態で、`make deadcode` はこの関数を報告しない。
- **AC-03**: ビルドタグ `test` の付いたテストヘルパーのうち、ほかのパッケージのテストだけが使う関数（例: `internal/cmdrunner/testutil/mocks.go` の `New`）を、到達できない関数として報告しない。
- **AC-04**: 本番のコードのうち、どの `main` パッケージからも届かず、テストからだけ呼ばれる関数（例: `internal/config` の `Load`）を、到達できない関数として報告しない。
- **AC-05**: `internal/` の下のパッケージに、どこからも参照されない関数を加える。関数はパッケージ水準の関数（メソッドでない）で、生成されたファイルでないファイルに置く。この状態で、`make deadcode` はその関数を報告する。報告には、その関数のファイルと行と名前が含まれる。
- **AC-06**: `cmd/yt2column` 以外の `main` パッケージ（例: `cmd/yt2column-server`）に、AC-05 と同じ条件の関数を加えると、`make deadcode` はその関数を報告する。報告の中身は AC-05 と同じである。
- **AC-07**: ビルドタグ `integration` の付いたファイルに AC-05 と同じ条件の関数を加えると、`make deadcode` はその関数を報告する。
- **AC-08**: 統合テスト（ビルドタグ `integration` の付いたテスト）からだけ呼ばれる関数を、到達できない関数として報告しない。
- **AC-09**: モジュールに新しい `main` パッケージを加え、そこからだけ `internal/` の関数を呼ぶ。この状態で、`deadcode` ターゲットを変えずに、`make deadcode` はその関数を報告しない。

#### F-002: 終了コードと表示

`make deadcode` は、報告の有無と解析の成否を終了コードで示す。終了コード 0 で終わるのは、`deadcode` が終了コード 0 で終わり、標準出力にも標準エラーにも何も出力しなかった場合だけである。それ以外の場合、つまり `deadcode` の出力が報告の形かどうかに関係なく何かを出力した場合は、0 以外の終了コードで終わる。`deadcode` の出力の形が変わっても、黙って報告 0 件と判定しないためである。

**Acceptance Criteria**:
- **AC-10**: 解析が成功し、`deadcode` が標準出力にも標準エラーにも何も出力しなかったとき（報告が 0 件のとき）、`make deadcode` は終了コード 0 で終わる。
- **AC-11**: 到達できない関数の報告が 1 件以上あるとき、`make deadcode` は 0 以外の終了コードで終わる。報告はすべて表示される。
- **AC-12**: 解析そのものが失敗したとき（例: パッケージのコンパイルエラー）、`make deadcode` は 0 以外の終了コードで終わり、`deadcode` が出したエラーのメッセージが表示される。
- **AC-13**: `PATH` に `deadcode` がないとき、`make deadcode` は 0 以外の終了コードで終わり、`deadcode` が見つからないことと、インストール方法（`go install` のコマンド）を表示する。

#### F-003: 開発者向けの説明

**Acceptance Criteria**:
- **AC-14**: `CLAUDE.md` の「Code Quality」にある `make deadcode` の説明が、次の 2 点を述べている。解析の範囲（モジュール全体と、そのテスト）と、報告が 1 件以上あれば失敗すること。
- **AC-15**: `.claude/commands/runplan.md` の step 6 が、次の 2 点を述べている。意図した足場を残すならテストから届くようにすること、`make deadcode` が通ること。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   要件はない。本書の作成時、`deadcode -test -tags test ./...` は手元の macOS で約 1 秒で終わった。開発者が手で実行するターゲットであり、この時間は問題にならない。

### 4.2. セキュリティ (Security)

-   N/A。本タスクは開発用のターゲットだけを変える。次のいずれも行わないので、`docs/dev/security.md` を適用する条件に当たらない。外部コマンド `yt-dlp` を呼ばない。API キーと Webhook URL を扱わない。キャッシュのディレクトリを読み書きしない。ネットワークにデータを送らない。信頼できないテキストをプロンプトや記事に埋め込まない。
-   レシピは外部コマンド `deadcode` の出力を読んで終了コードを決める。この出力の扱いは F-002 のとおりで、何かが出力されれば失敗とする。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   F-002 のとおり、解析の失敗や見慣れない出力を、報告 0 件と取り違えない。

### 4.4. 互換性 (Compatibility)

-   macOS と Linux で動作する。手元の macOS の GNU Make 3.81 でも、Linux の GNU Make 4.x でも同じ振る舞いになる。
-   `ext-` で始まるものを除く `Makefile` のターゲットは、Node.js を必要としない。`extension/test/repository.test.ts:230-258` は、`node` と `npm` のない `PATH` で `make -n build test lint deadcode` を実行し、次の 3 点を確かめている。終了コードが 0 であること、標準エラーが空であること、出力に `node`・`npm`・`npx` が現れないこと。このテストは CI の `ubuntu-latest` でも実行される（`.github/workflows/ci.yml`）。本タスクの後も、この条件を満たす。

### 4.5. 保守性 (Maintainability)

-   新しい `main` パッケージやパッケージを加えても、`deadcode` ターゲットを変えずに解析の範囲に入る（AC-09）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   新しい外部のツールやモジュールを加えない。`deadcode` は今と同じく、`PATH` にあるものを使う。

## 6. 用語集 (Glossary)

-   **到達できない関数（unreachable func）:** `deadcode` が、解析の起点（`main` パッケージと、`-test` のときはテスト）から呼び出しの連鎖でたどり着けないと判定した関数。本書では、起点から呼び出しの連鎖でたどり着けることを「届く」とも書く。`deadcode` は Rapid Type Analysis で判定するので、インターフェースに変換される型のメソッドは、呼ぶ箇所がなくても到達できると判定されることがある。また、既定では生成されたファイルの関数を報告しない。AC-05〜AC-07 が対象をパッケージ水準の関数に限り、生成されたファイルを除くのは、このためである。
-   **報告:** `deadcode` が到達できない関数ごとに出す行。`golang.org/x/tools` v0.47.0 では `<file>:<line>:<col>: unreachable func: <name>` の形である。
-   **意図した足場:** 後のフェーズや PR が呼ぶ予定で、今はまだ本番のコードから呼ばれない関数（`runplan.md` の step 6 の「intentional scaffolding」）。
-   **完了条件:** 実装計画で、フェーズや PR を終えるために通らなければならない手順（`make test` など）。
