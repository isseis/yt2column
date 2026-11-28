# 要件定義書：スクリプト・Makefile を子プロセスに渡すテストのキャッシュ無効化と CI `doc-guard` の `-count=1` 付与

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

**目的:** このドキュメントは、開発用シェルスクリプトや `Makefile` だけを編集したときに `make test` や CI の `doc-guard` ジョブ（後述の §6 で定義）がキャッシュ済みの PASS を返してしまう問題を解消するための要件を定義する。

- 対応 issue: [#1205](https://github.com/isseis/yt2column/issues/1205)
- 関連 issue: [#1181](https://github.com/isseis/yt2column/issues/1181)（開発ツール系テストの分離。分離後に Go のテストキャッシュを信頼して再実行を省くには、本タスクの修正が前提になる）

**背景:**

Go のテストキャッシュ（`go test`）は、テストプロセス自身が `os.Open` 等で開いたファイルと環境変数だけをキャッシュキーに含める。`Makefile:93-95`（コミット `5bba819a`）の `test` ターゲットは `-count=1` を付けないため、テストプロセス自身が開いていないリポジトリ内ファイルを子プロセス（`bash`・`make`）に渡すテストは、そのファイルだけを編集してもキャッシュ済みの PASS を返す。

たとえば `cmd/yt2column/test_helpers_gitfixture.go:163`（`(*gitFixture).runScript`）はスクリプトのパスを `bash` に渡すだけで Go プロセス側では開いていないため、`.claude/commands/_lib/high-review-decision.sh` の末尾に `exit 3` を足しても `go test -tags test -run '^TestHighReviewDecision$' ./cmd/yt2column` が `(cached)` で PASS する。同じ問題は `cmd/yt2column/deadcode_test.go:210,680` が `Makefile` について既に `os.ReadFile` で回避している。

また CI（`.github/workflows/ci.yml:277-291`、コミット `5bba819a`）では、`.claude/` 配下だけの変更は `check-changes`（`.github/workflows/ci.yml:49` および `internal/ciskip/git.go:199`）でコード以外の変更と判定されるため、`-count=1` 付きの `test` ジョブは走らず `doc-guard` ジョブだけが走る。`doc-guard` は `actions/setup-go@v6`（`cache: true`）で `GOCACHE` を復元する一方、実行コマンドに `-count=1` が付いていないため、キャッシュ復元により同じ穴が CI にも現れうる。

**本書で決めること／`02_architecture.md` に委ねること:**

- 本書では、対象とするテスト系統と CI ジョブの範囲、観測可能な振る舞い（受け入れ基準）、および issue #1205 の「要確認」項目の調査結果を定める。
- 共通ヘルパーの具体的な配置、キャッシュキー登録を確かめる自動テストの構造、および `doc-guard` ガードテストの組み込み方は `02_architecture.md` で定める。

### issue #1205 で決めたこと

- `make test` に `-count=1` を付けてキャッシュ自体を無効化するのではなく、テストプロセス自身が対象ファイルを読むことでキャッシュキーに含める（`cmd/yt2column/deadcode_test.go` の前例に揃え、#1181 の分離後にキャッシュを信頼できるようにするため）。
- テストごとの書き忘れを防ぐため、共通のヘルパー（`runScript` など）で読む形にする。
- CI の `doc-guard` について、`actions/setup-go` のキャッシュが復元されうるかを確かめ、復元されうるなら `-count=1` を付ける。

### 要件定義の際に決めたこと（2026-11-28）

1. 進め方（プロセス）を簡易な進め方（`simple`）にする。
   - 理由：影響範囲がテストヘルパー／テスト（4 パッケージ）と `.github/workflows/ci.yml` の `doc-guard` 1 行に収まり、既存の前例（`deadcode_test.go` の `os.ReadFile`）の横展開で新しい仕組みを作らず、既存 ADR の決定も変更しないため。
   - 除外条件の判定：該当しない（`docs/dev/security.md` の規則・前提および ADR-0006 を変更せず、開発ツールの信頼境界である ADR-0015 を変更せず、外部依存や CI アクションの追加・変更も行わないため）。除外判定の上書きなし。
   - ADR：なし（進め方の選択はタスク固有の記録であり、`01_requirements.md` の `Process` 行に記録する）。
2. キャッシュキー修正の対象を、シェルスクリプト実行系 3 系統（`cmd/yt2column`・`internal/chief`・`internal/ciskip`）と `Makefile` 実行系（`internal/maketestutil`・`cmd/yt2column/makefile_test.go`）のすべてとする。
   - 理由：issue #1205 の「子プロセスにリポジトリ内のファイルを渡すほかのテスト」を漏れなく閉じ、`deadcode_test.go` 以外の `Makefile` 実行テストでも単独変更時のキャッシュ漏れをなくすため。
   - ADR：なし（個別タスク内のテストキャッシュ修正であり、他タスクやコンポーネント境界を縛る主要決定ではない）。
3. CI の `.github/workflows/ci.yml` の `doc-guard` ジョブの `go test` コマンドに `-count=1` を追加し、その存在を自動テスト（ガードテスト）で固定する。
   - 理由：`doc-guard` は `actions/setup-go@v6`（`cache: true`）で `GOCACHE`（テストキャッシュを含む）を復元するため、`Makefile:97-100` の `test-ci` と同様にキャッシュ済みの PASS の再利用を防ぐ必要がある。あわせて、将来の `ci.yml` 編集で `-count=1` が誤って落ちるのを防ぐため、`make test` に含まれる自動テストで固定する。
   - ADR：なし（`ADR-0016` は `doc-guard` を全 PR で実行する方針と対象パッケージを定めており、`-count=1` の追加とガードテストによる固定はその決定を変更しない）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

- 開発用シェルスクリプトまたは `Makefile` だけを編集したあとの `make test`（および `-run` による単体実行）で、該当テストがキャッシュ済みの PASS を返さずに再実行される状態にする。
- CI の `doc-guard` ジョブが、復元されたテストキャッシュを再利用して古い PASS を返さない状態にする。
- issue #1205 の「要確認」各項目とリポジトリ全体の他テストについて、該当有無と対処を明示的に記録する。

### 2.2. スコープ (In Scope)

コミット `5bba819a` 時点のリポジトリ全体（`*_test.go` および `*test_helpers*.go` の `exec.Command` / `cmdrunner.NewOS`）を調査した結果を以下に示す。該当する 4 系統と CI の `doc-guard` を本タスクの対象とする。

| 調査項目（issue #1205 の区分） | 箇所（`file:line`、`5bba819a`） | 対象ファイル | 該当有無 | 対処 |
|---|---|---|---|---|
| 確認済み：`review_records_test.go` / `high_review_decision_test.go` | `cmd/yt2column/test_helpers_gitfixture.go:163`（`(*gitFixture).runScript`） | `.claude/commands/_lib/review-records.sh`・`high-review-decision.sh` | 該当 | `runScript` 内で対象スクリプトを Go プロセスで読む |
| 要確認：`adr_candidates_docs_test.go` | `cmd/yt2column/adr_candidates_docs_test.go:117` → `runScript` | `.claude/commands/_lib/adr-correction-candidates.sh` | 該当 | 上記 `runScript` の修正で一括解消 |
| 要確認：`internal/chief/markers_parity_test.go` | `internal/chief/markers_parity_test.go:64,107` | `.claude/commands/_lib/high-review-decision.sh` | 該当 | `TestMarkerReadingMatchesDecisionScript` で対象スクリプトを Go プロセスで読む |
| 要確認：他テスト（`internal/ciskip`） | `internal/ciskip/test_helpers.go:145`（`runners`）・`internal/ciskip/git_test.go:654` → `internal/ciskip/git.go:210` | `extension/scripts/has-extension-changes.sh` | 該当 | `internal/ciskip` の実スクリプトランナー生成ヘルパーで対象スクリプトを Go プロセスで読む |
| 要確認：他テスト（`Makefile` 実行系） | `internal/maketestutil/make.go:106`（`RunTarget`）・`cmd/yt2column/makefile_test.go:215,415`（`makeDryRun`・`recipeStub.command`） | `Makefile` | 該当 | 各ヘルパーで対象ディレクトリの `Makefile` を Go プロセスで読む |
| 要確認：他テスト（既に対処済み・非該当） | `cmd/yt2column/deadcode_test.go:210,680`（`Makefile` 読み込み済み）、`cmd/yt2column/fixpr_docs_test.go:240`・`cmd/fixprguard/docs_test.go:781`・`internal/ciskip/workflow_pin_test.go:709,909`（対象文書を Go プロセスで読み込み済み）、`os.Args[0]` 再実行系・`t.TempDir()` 内一時ファイル実行系 | — | 非該当 | 変更不要 |
| 要確認：CI `doc-guard` | `.github/workflows/ci.yml:284-291` | `doc-guard` の `go test` | 該当（`actions/setup-go@v6` の `cache: true` で `GOCACHE` が復元されうる） | `go test` に `-count=1` を追加し、ガードテストで固定する |

### 2.3. スコープ外 (Out of Scope)

- `Makefile` の `test` ターゲットへの `-count=1` 追加（ローカルのテストキャッシュ活用を維持するため、issue #1205 で除外済み）。
- 開発ツール系テストのパッケージ分離や CI ジョブ分割（[#1181](https://github.com/isseis/yt2column/issues/1181) の範囲）。
- ブラウザ拡張（`extension/`）の Node.js テスト（Go のテストキャッシュ機構の対象外）。
- 外部コマンドバイナリ自体（`bash`・`make`・`git`・`deadcode` 等）のバージョン更新によるキャッシュ無効化（`cmd/yt2column/deadcode_test.go:669-671` と同じく `-count=1` の手動実行で扱う）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: シェルスクリプトを実行するテストのキャッシュキー登録

リポジトリ内のシェルスクリプトを子プロセス（`bash`）に渡して実行するすべての Go テストにおいて、テストプロセス自身もそのスクリプトファイルを開いて読む。

**Acceptance Criteria**:
- **AC-01**: `cmd/yt2column` のスクリプト実行テスト（`review_records_test.go`・`high_review_decision_test.go`・`adr_candidates_docs_test.go`）は、子プロセスに渡す対象シェルスクリプトを Go のテストプロセス自身でも開く。対象スクリプトだけを変更したあとの `make test`（および `-run` 指定の実行）ではキャッシュ済みの PASS を返さず再実行され、スクリプトを壊した変更では失敗する。
- **AC-02**: `internal/chief/markers_parity_test.go` は、子プロセスに渡す `.claude/commands/_lib/high-review-decision.sh` を Go のテストプロセス自身でも開く。同スクリプトだけを変更したあとの `make test`（および `-run` 指定の実行）ではキャッシュ済みの PASS を返さず再実行され、スクリプトを壊した変更では失敗する。
- **AC-03**: `internal/ciskip` の実スクリプト実行テスト（`git_test.go`）は、子プロセスに渡す `extension/scripts/has-extension-changes.sh` を Go のテストプロセス自身でも開く。同スクリプトだけを変更したあとの `make test`（および `-run` 指定の実行）ではキャッシュ済みの PASS を返さず再実行され、スクリプトを壊した変更では失敗する。

#### F-002: `Makefile` を実行するテストのキャッシュキー登録

リポジトリの `Makefile` を子プロセス（`make`）に渡して実行する Go テストにおいて、テストプロセス自身もその `Makefile` を開いて読む。

**Acceptance Criteria**:
- **AC-04**: `internal/maketestutil` の `RunTarget`（`internal/transcript`・`internal/llm/{deepseek,claude,openai}`・`cmd/yt2column` の各 `makefile_test.go` が利用）および `cmd/yt2column/makefile_test.go` の `make` 実行ヘルパー（`makeDryRun`・`recipeStub.command`）は、対象の `Makefile` を Go のテストプロセス自身でも開く。`Makefile` だけを変更したあとの `make test` および `-run` 指定の単体実行ではキャッシュ済みの PASS を返さず再実行される。

#### F-003: CI `doc-guard` ジョブでのキャッシュ再利用防止

`.github/workflows/ci.yml` の `doc-guard` ジョブが `actions/setup-go` の復元キャッシュからテスト結果を再利用しないようにし、その設定が誤って外されないことをガードテストで固定する。

**Acceptance Criteria**:
- **AC-05**: `.github/workflows/ci.yml` の `doc-guard` ジョブが実行する `go test` コマンドに `-count=1` が含まれている。
- **AC-06**: `make test` に含まれる自動テストが `.github/workflows/ci.yml` の `doc-guard` ジョブの `go test` コマンドに `-count=1` が含まれることを確かめ、`-count=1` を外したときに失敗する。

#### F-004: 既存検証の維持

本タスクの変更後もプロジェクトの既存テスト・静的解析がすべて通る。

**Acceptance Criteria**:
- **AC-07**: `make test` および `make lint` がすべて成功する。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

- テストプロセスで読み込む対象ファイル（シェルスクリプトおよび `Makefile`）はいずれも数十 KiB 以下であり、テスト実行時間の増加は無視できる範囲であること。

### 4.2. セキュリティ (Security)

- N/A（本変更は Go のテストコードと CI ワークフローのフラグ追加のみであり、`docs/dev/security.md` の適用条件 — `yt-dlp` 実行・API キー・キャッシュディレクトリ・ネットワーク通信・信頼できないテキストのプロンプト／記事への埋め込み — のいずれにも該当しない）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

- 対象のスクリプトや `Makefile` が存在しないか読み込めない場合、テストは黙ってスキップせず即座に失敗すること。

### 4.4. 互換性 (Compatibility)

- macOS（開発者の手元環境）および Linux（CI の `ubuntu-latest`）の両方で動作すること。

### 4.5. 保守性 (Maintainability)

- 対象ファイルの読み込みは共通のテストヘルパーに集約し、個別のテストケースごとに読み込みを書き忘れる構成にしないこと。

## 5. 制約条件 (Constraints)

- `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」および `CLAUDE.md` の規約（Go のソースコード・コメント・識別子・文字列リテラルは英語、テストヘルパーは `//go:build test` 等）に従う。
- `Makefile` の `test` ターゲットには `-count=1` を追加しない。
- 新しい外部依存を追加しない。

## 6. 用語集 (Glossary)

- 用語は `docs/translation_glossary.md` に統一する。
- **テストキャッシュ（Go test cache）**: `go test` が `GOCACHE` に保存するパッケージ単位のテスト結果キャッシュ。テストプロセス自身が開いたファイルと環境変数をキャッシュキーに含める。
- **`doc-guard`**: `.github/workflows/ci.yml` の常時実行ジョブ（表示名 `Documentation guard`）。変更の種類にかかわらずすべての PR で文書・スクリプト関連の Go テストを実行する。
- **壊し確認**: 確かめの対象を意図的に壊して、テストやガードが失敗することを確かめる手順（`CLAUDE.md`「Testing Strategy」）。
