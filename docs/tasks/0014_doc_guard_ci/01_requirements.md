# 要件定義書：文書だけの変更でも CI で文書の同期 guard を走らせる

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-14 |
| Review date | 2026-10-14 |
| Reviewer | isseis |
| Comments | 2026-10-14 決定変更: 対象パッケージを `./cmd/yt2column` から `./cmd/yt2column ./internal/pipeline` に拡大（`internal/pipeline/pipeline_test.go` が `package_reference.md`・`prompts/README.md` を読む文書 guard を持つため）。AC-02 と制約を更新し、再承認済み。 |

## 1. 概要 (Overview)

**目的:** 本書は、文書（`README.md`・`CLAUDE.md`・`docs/`）と `.claude/`・`LICENSE` だけを変える PR でも、文書の**テキスト同期**の guard を CI で実行するための要件を定義する。

**背景:**

- タスク `0012`（issue [#180](https://github.com/isseis/yt2column/issues/180)）で、文書のテキスト同期の guard を拡張の node のテスト（`extension/test/docs.test.ts`）から Go（`cmd/yt2column/extension_docs_test.go`・`docs_test.go`）へ移し、`make test`（`go test -tags test ./...`）で実行するようにした。移した検査は node を必要としない。
- ただし `ci.yml` の `check-changes`（`.github/workflows/ci.yml:36`）は `*.md`・`docs/`・`.claude/`・`LICENSE` だけの変更をコードの変更とみなさない。そのため文書だけの PR では Go のテストのジョブ（`test`）も拡張のジョブ（`extension`）も走らず、移した guard は CI で実行されない。移設の前後でこの状況は変わらない。
- `0012` はこの点をスコープ外とし（`docs/tasks/0012_doc_sync_guards/01_requirements.md` §2.3・制約、`design_handoff.md` H-03）、ローカルの `make test`（`CLAUDE.md:209`）に委ねた。残るリスクは `0012` の `02_architecture.md` §7.4 に記してある。
- 残るリスクは、手元でテストを実行しない経路（GitHub のウェブ画面での編集、bot）で文書の guard が壊れると、次にコードの変更を含む PR で `make test` が走ったときに初めて検出されることである。原因の PR とは別の PR で検出されるため、原因の特定が遅れる。
- 文書を読む Go の guard は `cmd/yt2column` だけではない。`internal/pipeline/pipeline_test.go`（`//go:build test`）も、`:646` で `docs/dev/conventions/package_reference.md` を、`:696` で `prompts/README.md` を読んで同期を検査する。どちらも `check-changes` では非コード扱い（`*.md`・`docs/`）なので、文書だけの PR では `test` ジョブが走らず、同じ穴がある。

対応 issue: [#181](https://github.com/isseis/yt2column/issues/181)（label: `devenv`）

要件レビュー（と起草）の過程で挙がった、設計で決める事項は [design_handoff.md](design_handoff.md) に申し送る。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   すべての PR で必ず走る小さな CI ジョブ（`doc-guard`）を `ci.yml` に追加し、文書の同期 guard を実行する（F-001）。形は既存の `secret-scan` ジョブ（`.github/workflows/ci.yml:169-184`）に倣い、`setup-go` のあとに `go test -tags test ./cmd/yt2column ./internal/pipeline` を実行する。
-   文書だけの変更を CI が守ることを `CLAUDE.md` の規則に反映する（F-002）。
-   `0012` の `02_architecture.md` §7.4 の「残るリスク」の記述を、本タスクで CI が守るようになったことに合わせて更新する（F-003）。
-   文書だけの PR でジョブが走ること、guard を壊すと赤になることを、捨てコミットで確認し記録する（F-004）。

### 2.2. スコープ (In Scope)

-   `ci.yml` への `doc-guard` ジョブの追加（F-001）。対象は文書を読む guard を持つ `./cmd/yt2column`・`./internal/pipeline`。
-   `CLAUDE.md` の「Development Notes」の該当規則の更新（F-002）。
-   `0012` の `02_architecture.md` §7.4 の記述の更新（F-003）。
-   捨てコミットによる GitHub Actions での検証と、その結果の記録（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   `check-changes` の分類（`.github/workflows/ci.yml:24-53` の grep と `extension/scripts/has-extension-changes.sh`）の変更。`has-code-changes`・`has-extension-changes` の意味は変えない。既存の `extension/test/ciChanges.test.ts` などに影響しない。
-   `test`・`lint`・`extension` ジョブのトリガ・内容の変更。
-   `.gitignore` の ignore 規則の検査（`extension/test/repository.test.ts` の `git check-ignore`）の Go への移設。`0012` の H-07 で node 側に残すと決めており、本タスクの対象外とする。
-   文書の guard が読む文書の内容・guard 本体（`cmd/yt2column/extension_docs_test.go`・`docs_test.go`・`internal/pipeline/pipeline_test.go`）の変更。本タスクは guard を実行する経路を足すだけで、検査の内容は変えない。
-   新しい外部モジュール・新しい Makefile ターゲットの追加。
-   branch protection の必須チェックへの `doc-guard` の登録（リポジトリ設定の人手の作業。制約を参照）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 常時実行の `doc-guard` ジョブ

`ci.yml` に、PR ごとに必ず走る `doc-guard` ジョブを追加し、文書の同期 guard を実行する。形は `secret-scan` ジョブに倣う（`needs` を持たず、`if` で `check-changes` の出力を見ない）。

-   ステップは checkout → setup-go（`go-version-file: go.mod`、`cache: true`）→ `go test -tags test ./cmd/yt2column ./internal/pipeline` とする。
-   `-run` でテスト名を絞らない。対象のパッケージ全体を対象にする。絞ると、新しく足した guard が正規表現から漏れても気づけない。
-   対象は、文書を読む guard を持つパッケージ（`cmd/yt2column`・`internal/pipeline`）である。文書を読まないパッケージは対象に含めない。
-   `doc-guard` は `check-changes` に依存しない（`needs: check-changes` を付けない）。`check-changes` の分類が `has-code-changes=false` でも走る。

**Acceptance Criteria**:
- **AC-01**: `ci.yml` に `doc-guard` という名前のジョブがある。このジョブは `needs` を持たず、ジョブに `if:` を付けない（`check-changes` の出力に依存しない）。したがって文書だけの PR でも走る。
- **AC-02**: `doc-guard` ジョブは、checkout → `actions/setup-go`（`go-version-file: go.mod`）→ `go test -tags test ./cmd/yt2column ./internal/pipeline` の順でステップを持つ。`go test` のコマンドに `-run` がなく、テスト名を絞らない。
- **AC-03**: `check-changes` ジョブの分類ロジック（`ci.yml` の `Classify changed files` ステップの grep、`extension/scripts/has-extension-changes.sh`）を変更しない。`has-code-changes`・`has-extension-changes` の値の意味が変わらない。

#### F-002: `CLAUDE.md` の規則の更新

`CLAUDE.md` の「Development Notes」の、文書だけの変更に関する規則（`CLAUDE.md:211-215`）を、文書だけの変更でも CI が guard を走らせる旨に更新する。

-   現行の趣旨（文書だけの変更は `make ext-test` を要求しない。移した guard は `make test` に含まれ node を必要としない）は維持する。
-   CI が守るようになったことを明記し、ローカルの `make test` を前提にした書き方は残す（編集の後は `make test` を実行する。`CLAUDE.md:209`）。

**Acceptance Criteria**:
- **AC-04**: `CLAUDE.md` の「Development Notes」の該当規則が、文書（`README.md`・`CLAUDE.md`・`docs/`）だけの変更でも CI の `doc-guard` ジョブが文書の guard を実行することと、文書だけの変更で `make ext-test` を要求しないことを述べている（静的確認）。

#### F-003: `0012` の `02_architecture.md` §7.4 の記述の更新

`0012` が残るリスクとして記した §7.4 の「残るリスク（明示）」を、本タスク（#181）で文書だけの変更を CI が守るようになったことに合わせて更新する。

-   更新は `0012` の承認済み文書に対する事後の変更である。決定（何を作るか）を変えるか、事実だけが古くなったかの区分は `requirements_process.md` §0 の「Editing an approved document」に従い、必要なら `Comments` に記す。
-   残った限界（`doc-guard` は文書を読む guard を持つと分かっているパッケージ（`./cmd/yt2column`・`./internal/pipeline`）だけを対象にすること、後から別パッケージに足した文書 guard は、コマンドを更新するまで守らないこと）は、消さずに記す。

**Acceptance Criteria**:
- **AC-05**: `docs/tasks/0012_doc_sync_guards/02_architecture.md` §7.4 の記述が、文書だけの変更を `doc-guard` ジョブ（#181）が CI で守るようになったことを反映している。記述が実装と一致し、古い「CI はこの変更を守らない」という記述が現在も有効なものとして残っていない（静的確認）。

#### F-004: 捨てコミットによる検証と記録

GitHub Actions は手元で動かせないので、捨てコミットを使って次を確認し、結果（workflow の実行 URL など）を実装計画の手動確認の節に記録する。

-   文書だけを変える PR で `doc-guard` ジョブが走ること。
-   文書の guard が読むテキストを壊す変更を含む PR で `doc-guard` ジョブが赤になること。

**Acceptance Criteria**:
- **AC-06**: 文書だけを変える捨てコミットの PR で `doc-guard` ジョブが走り、成功することを GitHub Actions で確認し、その PR または workflow の実行の参照を `03_implementation_plan.md` の手動確認の節に記録している（静的確認。記録の形式は `0012` の手動確認の節に倣う）。
- **AC-07**: 文書の guard が読むテキストを壊す変更（例: `CLAUDE.md` の `ext-` ターゲットの一覧から 1 つ削る）を含む捨てコミットの PR で `doc-guard` ジョブが赤になることを確認し、その参照を `03_implementation_plan.md` の手動確認の節に記録している（静的確認）。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

本タスクが変えるのは CI のジョブ定義（`.github/workflows/ci.yml`）と文書だけである。`doc-guard` ジョブは、リポジトリにコミットされたファイルを `go test` で読むだけで、ネットワーク・外部コマンド・利用者が実行時に与える入力の新しい境界を持たない。したがって、本タスクに信頼できない入力の境界はない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   `doc-guard` ジョブは、`setup-go` を含めて 1 分前後で終わる見込みとする。手元の実測では `go test -tags test ./cmd/yt2column ./internal/pipeline` は合計約 14 秒（`cmd/yt2column` 約 13 秒、`internal/pipeline` 約 0.5 秒）。`internal/pipeline` を足しても実行時間はほぼ変わらない。
-   コードの変更を含む PR では `test` ジョブと重複するが、許容範囲とする。`check-changes` の分類を変えてまで重複を避けない。

### 4.2. セキュリティ (Security)

-   `doc-guard` ジョブは秘密情報（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`SLACK_WEBHOOK_URL`）を扱わない。workflow レベルの `permissions: contents: read` を維持し、ジョブに追加の権限を与えない。
-   ジョブの実行に secrets を渡さない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   `doc-guard` ジョブは、文書の guard が落ちたときに赤になる（無言で通さない）。`go test` の非ゼロ終了をそのままジョブの失敗とする。
-   `doc-guard` ジョブの追加が、既存のジョブ（`test`・`lint`・`extension`・`secret-scan`）の成否・実行条件を変えない。

### 4.4. 互換性 (Compatibility)

-   `doc-guard` ジョブは既存と同じ `ubuntu-latest` と `go-version-file: go.mod` を使う。対象パッケージの文書 guard は macOS と Linux のどちらでも `make test` で動く（`0012` §7.3）。
-   `-tags test` で実行する。対象パッケージの文書 guard は `//go:build test` であり、`integration` タグは不要である。

### 4.5. 保守性 (Maintainability)

-   文書の guard を実行する経路を 1 か所（`doc-guard` ジョブ）に集約し、`check-changes` の分類とは独立に保つ。
-   実行するコマンドは `go test -tags test ./cmd/yt2column ./internal/pipeline` のように、対象パッケージを明示する。`make test`（`./...`）を `doc-guard` で呼ぶと、文書だけの PR でも全パッケージを race 付きで走らせることになり、性能の要件に反する。
-   対象パッケージ（`cmd/yt2column`・`internal/pipeline`）を変える変更（guard を別パッケージへ移す等）は、同じ変更で `doc-guard` ジョブのコマンドを更新する。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」に従う。外部ライブラリは追加しない（`CLAUDE.md`「Dependencies」）。
-   `check-changes` の分類（`ci.yml:36` の grep、`has-extension-changes.sh`）は変更しない。`extension/test/ciChanges.test.ts` などの既存テストを変更しない。
-   `doc-guard` ジョブの実行対象は、文書を読む guard を持つ `./cmd/yt2column`・`./internal/pipeline` とする（`0012` の guard と、`internal/pipeline/pipeline_test.go` の `package_reference.md`・`prompts/README.md` の guard）。`-run` でテスト名を絞らない。絞ると、新しく足した guard が正規表現から漏れても気づけない。
-   `0012` はマージ済みである。`0012` の承認済み文書を更新する場合は、`requirements_process.md` §0 の「Editing an approved document」に従い、決定を変える編集は `draft` に戻して再承認を求め、事実だけが古くなった編集は状態を保って `Comments` にその旨を記す（区分は実装時に判断し、曖昧なら再承認を求める）。
-   GitHub Actions は手元で動かせないため、`doc-guard` ジョブの挙動の検証は捨てコミットによる PR で行う。捨てコミットは `main` にマージしない。
-   branch protection で必須チェックを設定している場合は、`doc-guard` を登録する人手の作業が要る。本タスクの実装対象（リポジトリ内のファイル）には含めない。完了の前提として `03_implementation_plan.md` の Next Steps に記す。
-   実装は `requirements_process.md` の承認の流れ（`01_requirements.md` → `02_architecture.md` → `03_implementation_plan.md` の順に承認）に従う。

## 6. 用語集 (Glossary)

-   **文書の同期 guard:** 文書・`Makefile`・manifest の間で一致すべきテキスト（拡張 ID、`ext-` ターゲットの一覧、permission の列挙など）を検査する Go のテスト（`cmd/yt2column/extension_docs_test.go`・`docs_test.go`。タスク `0012`）。
-   **`doc-guard` ジョブ:** 本タスクで `ci.yml` に追加する、PR ごとに必ず走るジョブ。`go test -tags test ./cmd/yt2column ./internal/pipeline` を実行する。
-   **常時実行ジョブ:** `check-changes` の分類に依存せず、すべての PR で走るジョブ（既存の `secret-scan`、本タスクの `doc-guard`）。
-   **docs-only の変更（文書だけの変更）:** `README.md`・`CLAUDE.md`・`docs/`・`.claude/`・`LICENSE` だけを変える変更。`ci.yml` の `check-changes` はこれらをコードの変更とみなさない。
-   **`check-changes`:** `ci.yml` の最初のジョブ。変更ファイルを `has-code-changes`・`has-extension-changes` に分類し、後続ジョブの実行条件を決める。
-   **壊し確認:** テストが主張する対象を無効にして、そのテストが落ちることを確かめる手順（`CLAUDE.md` の testing strategy。`0012` と同じ手順）。
-   用語は [translation_glossary.md](../../translation_glossary.md) と統一する。
