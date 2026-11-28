# 実装計画書：スクリプト・Makefile を子プロセスに渡すテストのキャッシュ無効化と CI `doc-guard` の `-count=1` 付与

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md` および `02_architecture.md` に基づき、リポジトリ内のシェルスクリプトや `Makefile` を子プロセスに渡す Go テストが Go プロセス自身でも対象ファイルを開くように修正し、CI の `doc-guard` ジョブに `-count=1` を追加してガードテストで固定する。

### 1.2. 実装方針

- `02_architecture.md` §3.2 の責務表と §8 の実装優先順位に従い、共通のテストヘルパーから順に修正する。
- Go のソースコード・コメント・識別子・文字列リテラルはすべて英語で書く（`CLAUDE.md`「Language」）。
- テストヘルパーは既存の `//go:build test` 構成を維持し、不要な新規ヘルパーファイルや `testutil/` は増やさない（`docs/dev/conventions/test_organization.md`）。
- 非 `_test.go` のテストヘルパー（`internal/maketestutil/make.go`・`cmd/yt2column/test_helpers_gitfixture.go`・`internal/ciskip/test_helpers.go`）の `os.ReadFile` で `gosec`（G304）が反応する場合は、該当行に最小限の `//nolint:gosec` とテスト専用パスである旨の英語コメントを付ける。

### 1.3. 既存コード調査結果

コミット `5bba819a` 時点で確認した既存コードの状況は以下のとおり。

- **`internal/maketestutil/make.go:106`（`RunTarget`）**: `make -s <target> GOTEST=<stub>` を `cmd.Dir = root` で実行するが、`filepath.Join(root, "Makefile")` を Go プロセスで開いていない。`internal/transcript/makefile_test.go`・`internal/llm/{deepseek,claude,openai}/makefile_test.go`・`cmd/yt2column/makefile_test.go` の 5 パッケージが利用している。`internal/maketestutil/make_test.go` が既存のユニットテストファイルとして存在する。
- **`internal/ciskip/test_helpers.go:145`（`(*testGitRepo).runners`）と `internal/ciskip/git_test.go:654`（`TestCheckExtensionChangesEarlyExitNoDeadlock`）**: どちらも `scriptOpts.Dir = filepath.Join("..", "..")` で `cmdrunner.NewOS(scriptOpts)` を生成し、`internal/ciskip/git.go:210` から `bash extension/scripts/has-extension-changes.sh` を呼ぶが、同スクリプトを Go プロセスで開いていない。
- **`internal/chief/markers_parity_test.go:64,107`（`TestMarkerReadingMatchesDecisionScript`）**: `decisionScript`（`../../.claude/commands/_lib/high-review-decision.sh`）を `filepath.Abs` で解決して `repo.Run(testBash, script, ...)` に渡すが、Go プロセスで開いていない。また同関数（44–49 行目）は他並列テストの `flock` 継承を避けるためトップレベルを直列実行している。
- **`cmd/yt2column/test_helpers_gitfixture.go:163`（`(*gitFixture).runScript`）**: `filepath.Abs(script)` を `bash.path` に渡すが、Go プロセスで開いていない。`review_records_test.go`・`high_review_decision_test.go`・`adr_candidates_docs_test.go` の 3 テストすべてがここを経由する。
- **`cmd/yt2column/makefile_test.go:215,415`（`makeDryRun`・`recipeStub.command`）**: `cmd.Dir = repositoryRoot` で `make` を実行するが、`Makefile` を Go プロセスで開いていない（`cmd/yt2column/deadcode_test.go:210,680` は既に `os.ReadFile` 済み）。
- **`.github/workflows/ci.yml:277-291`（`doc-guard`）と `internal/ciskip/workflow_pin_test.go:67,575`**: `doc-guard` は `actions/setup-go@v6`（`cache: true`）を使用するが、`go test -tags test ...` に `-count=1` がない。`workflow_pin_test.go` の `readWorkflow` は `wf.jobs["doc-guard"].steps` と各ステップの `runBody` を既に解析済みである。
- **外部前提の検証（`-test.testlogfile`）**: コミット `5bba819a` で `go test -tags test -c` の出力バイナリに `-test.testlogfile` を渡して実行し、Go プロセスが `os.ReadFile` したファイルが `open <path>` 行としてログに出力されることを実機確認済み。

### 1.4. 計画の際に決めたこと（2026-11-28）

1. **全ステップを 1 フェーズ・1 PR にまとめる。**
   - 理由：4 パッケージのテストヘルパー修正と `.github/workflows/ci.yml` の 1 行修正は規模が小さく、「スクリプト・Makefile 単独変更時のテストキャッシュ漏れ防止」という単一の関心事で完結しているため。
   - ADR：なし（PR 分割はタスク固有の実装計画上の判断であり、主要決定ではない）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: テストプロセスでのファイル読み込み追加と CI `doc-guard` の `-count=1` 固定

#### ステップ 1-1: `internal/maketestutil` の `Makefile` 読み込みと検証テスト追加

- **対象ファイル**: `internal/maketestutil/make.go`, `internal/maketestutil/make_test.go`
- **作業内容**:
  - [ ] `internal/maketestutil/make.go` の `RunTarget` で、`make` を起動する前に `filepath.Join(root, "Makefile")` を `os.ReadFile` で読み込み、失敗時は `t.Fatalf` で失敗させる（`gosec` が必要な場合はテスト専用パスである旨の英語コメント付き `//nolint:gosec` を該当行のみに付与する）。
  - [ ] `internal/maketestutil/make_test.go` に `TestRunTargetOpensMakefile` を追加し、自テストバイナリ（`os.Args[0]`）を `-test.testlogfile` 付きで再実行して `RunTarget` が対象ディレクトリの `Makefile` を `open` 記録することを確かめる。
- **完了条件**: `go test -tags test -count=1 ./internal/maketestutil` が成功する。

#### ステップ 1-2: `internal/ciskip` の `has-extension-changes.sh` 読み込み集約と検証テスト追加

- **対象ファイル**: `internal/ciskip/test_helpers.go`, `internal/ciskip/git_test.go`
- **作業内容**:
  - [ ] `internal/ciskip/test_helpers.go` にパッケージ非公開ヘルパー `newRealScriptRunner(t *testing.T) cmdrunner.InputRunner` を追加し、リポジトリルートを基準に `hasExtensionChangesScript` を `os.ReadFile` で読んで失敗時は `t.Fatalf` としたうえで `cmdrunner.NewOS(scriptOpts)` を返す（必要に応じて該当行のみ `//nolint:gosec` を付与する）。
  - [ ] `(*testGitRepo).runners` と `internal/ciskip/git_test.go` の `TestCheckExtensionChangesEarlyExitNoDeadlock` が `newRealScriptRunner(t)` を使うように置き換える。
  - [ ] `internal/ciskip/git_test.go` に `TestRealScriptRunnerOpensHasExtensionChangesScript` を追加し、`-test.testlogfile` で `extension/scripts/has-extension-changes.sh` が `open` 記録されることを確かめる。
- **完了条件**: `go test -tags test -count=1 ./internal/ciskip` が成功する。

#### ステップ 1-3: `internal/chief/markers_parity_test.go` のスクリプト読み込みと検証テスト追加

- **対象ファイル**: `internal/chief/markers_parity_test.go`
- **作業内容**:
  - [ ] `TestMarkerReadingMatchesDecisionScript` で `filepath.Abs(decisionScript)` の直後に `os.ReadFile(script)` を実行し、失敗時は `t.Fatalf` で失敗させる。
  - [ ] `internal/chief/markers_parity_test.go` に直列トップレベルテスト `TestMarkerReadingOpensDecisionScript` を追加し、`-test.testlogfile` で `.claude/commands/_lib/high-review-decision.sh` が `open` 記録されることを確かめる（`flock` 継承を避けるため `t.Parallel()` は付けない）。
- **完了条件**: `go test -tags test -count=1 ./internal/chief` が成功する。

#### ステップ 1-4: `cmd/yt2column` のスクリプト・`Makefile` ヘルパー修正と検証テスト追加

- **対象ファイル**: `cmd/yt2column/test_helpers_gitfixture.go`, `cmd/yt2column/makefile_test.go`, `cmd/yt2column/test_cache_inputs_test.go`
- **作業内容**:
  - [ ] `cmd/yt2column/test_helpers_gitfixture.go` の `(*gitFixture).runScript` で、`filepath.Abs(script)` の直後に `os.ReadFile(abs)` を実行し、失敗時は `f.t.Fatalf` で失敗させる（必要に応じて該当行のみ `//nolint:gosec` を付与する）。
  - [ ] `cmd/yt2column/makefile_test.go` の `makeDryRun` と `recipeStub.command` で、`filepath.Join(repositoryRoot, "Makefile")` を `os.ReadFile` で読み込み、失敗時は `t.Fatalf` で失敗させる。
  - [ ] `cmd/yt2column/test_cache_inputs_test.go` を作成し、`TestRunScriptOpensScript`（`runScript` が対象スクリプトを `open` 記録すること）および `TestMakeHelpersOpenMakefile`（`makeDryRun` と `recipeStub.command` が `Makefile` を `open` 記録すること）を `-test.testlogfile` で検証する。
- **完了条件**: `go test -tags test -count=1 ./cmd/yt2column` が成功する。

#### ステップ 1-5: CI `doc-guard` への `-count=1` 追加とワークフローガードテスト追加

- **対象ファイル**: `.github/workflows/ci.yml`, `internal/ciskip/workflow_pin_test.go`
- **作業内容**:
  - [ ] `.github/workflows/ci.yml` の `doc-guard` ジョブ（`Run documentation sync guards` ステップ）の `go test` コマンドに `-count=1` を追加する。
  - [ ] `internal/ciskip/workflow_pin_test.go` に `doc-guard` ジョブの `go test` 実行ステップに `-count=1` が含まれることを検証するサブテスト（またはテスト関数 `TestWorkflowDocGuardDisablesCache`）を追加する。
- **完了条件**: `go test -tags test -count=1 ./internal/ciskip` が成功する。

#### ステップ 1-6: 壊し確認と全体検証

- **対象ファイル**: なし（検証ステップ）
- **作業内容**:
  - [ ] 修正前コミット（または `os.ReadFile` を一時的に外した状態）で、issue #1205 の手順どおりスクリプトを壊したときに `(cached)` で PASS することを確かめる。
  - [ ] 修正後の状態で、`review-records.sh`・`high-review-decision.sh`・`adr-correction-candidates.sh`・`has-extension-changes.sh`・`Makefile` のそれぞれについて単独変更（壊し）を行い、対応するテストがキャッシュを使わず再実行されて失敗することを確かめ、変更を元に戻す。
  - [ ] `.github/workflows/ci.yml` の `doc-guard` から `-count=1` を一時的に外したときに `workflow_pin_test.go` のガードテストが失敗することを確かめ、元に戻す。
  - [ ] 壊し確認の結果をコミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。
  - [ ] `make fmt`・`make test`・`make lint` を実行し、すべて成功することを確かめる。
- **完了条件**: すべての壊し確認が期待どおり失敗し、元に戻した状態で `make test && make lint` が成功する。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

- **M1（フェーズ 1 完了）**: 4 パッケージのキャッシュキー登録修正、`doc-guard` の `-count=1` 付与、自動検証テスト、壊し確認、および `make test && make lint` の通過。

### 3.2. PR 構成

本計画は単一フェーズ（ステップ 1-1〜1-6）であり、PR 境界マーカーは設けず 1 本の実装 PR で完結させる。

**実装モデル要件**: low

**判定理由**: high および standard+high-review のいずれの判定基準にも該当しないため。

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

- **ユニットテスト（`-test.testlogfile`）**: 各パッケージのテストファイルで `os.Args[0]` を `-test.testlogfile` 付きで実行し、各ヘルパーが対象スクリプトまたは `Makefile` を Go の `testlog` に `open` 記録することを確かめる。再帰実行を防ぐため、子プロセス専用環境変数または明確に分離したヘルパーサブテストを使う。
- **ワークフローガードテスト**: `internal/ciskip/workflow_pin_test.go` で `doc-guard` の `go test` コマンドに `-count=1` が含まれることを確かめる。
- **壊し確認**: ステップ 1-6 で、各対象ファイルの単独変更によるキャッシュ無効化と、`doc-guard` の `-count=1` 削除によるガード失敗を実機で確認する。
- **回帰確認**: `make test`（`-race` 実行＋`test-parse-norace`）および `make lint`（`wasm-check`・`golangci-lint --build-tags test,integration`・`go vet -tags integration ./...`）を実行する。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 非 `_test.go` のテストヘルパーの `os.ReadFile` が `gosec`（G304）で失敗する | `make lint` が通らない | 該当行のみに最小限の `//nolint:gosec` とテスト専用ファイルである旨の英語コメントを付け、`make lint` で確認する |
| `internal/chief` で並列テストから子プロセスを起動すると他テストの `flock` を継承する | `cachelock` を使う他テストが間欠失敗する | `TestMarkerReadingOpensDecisionScript` に `t.Parallel()` を付けず直列実行する（`markers_parity_test.go:44-49` と同じ方針） |
| `-test.testlogfile` テストが子プロセスで再帰起動する | テストが無限再帰またはタイムアウトする | 子プロセス起動時に専用環境変数または別名のヘルパーサブテストで分岐し、親のみが子を起動する構造にする |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] ステップ 1-1: `internal/maketestutil` の `Makefile` 読み込みと `TestRunTargetOpensMakefile` 完了
- [ ] ステップ 1-2: `internal/ciskip` の `newRealScriptRunner` 集約と `TestRealScriptRunnerOpensHasExtensionChangesScript` 完了
- [ ] ステップ 1-3: `internal/chief/markers_parity_test.go` のスクリプト読み込みと `TestMarkerReadingOpensDecisionScript` 完了
- [ ] ステップ 1-4: `cmd/yt2column` の `runScript`・`makeDryRun`・`recipeStub.command` 修正と `test_cache_inputs_test.go` 完了
- [ ] ステップ 1-5: `.github/workflows/ci.yml` の `doc-guard` `-count=1` 追加と `workflow_pin_test.go` ガード追加完了
- [ ] ステップ 1-6: 壊し確認および `make fmt`・`make test`・`make lint` 通過

## 7. 完了基準 (Success Criteria)

- **機能**: `01_requirements.md` の AC-01〜AC-07 をすべて満たす。
- **品質**: `make test` と `make lint` がエラーなく通過し、すべての壊し確認がコミットメッセージに記録されている。
- **セキュリティ**: N/A（`01_requirements.md` §4.2 および `02_architecture.md` §5 のとおりセキュリティ要件の対象外）。
- **文書**: 本タスクの 3 文書に issue #1205 の「要確認」各項目の該当有無と対処が記録されている。

## 8. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種別 | 検証箇所 | 対応ステップ | 確認内容 |
|---|---|---|---|---|
| AC-01 | `test` | `cmd/yt2column/test_cache_inputs_test.go::TestRunScriptOpensScript`、およびステップ 1-6 の壊し確認 | 1-4, 1-6 | `runScript` が対象スクリプトを `open` 記録すること、および各スクリプト単独変更でキャッシュ済み PASS にならず失敗すること |
| AC-02 | `test` | `internal/chief/markers_parity_test.go::TestMarkerReadingOpensDecisionScript`、およびステップ 1-6 の壊し確認 | 1-3, 1-6 | `TestMarkerReadingMatchesDecisionScript` が `high-review-decision.sh` を `open` 記録すること、および単独変更でキャッシュ済み PASS にならず失敗すること |
| AC-03 | `test` | `internal/ciskip/git_test.go::TestRealScriptRunnerOpensHasExtensionChangesScript`、およびステップ 1-6 の壊し確認 | 1-2, 1-6 | `newRealScriptRunner` が `has-extension-changes.sh` を `open` 記録すること、および単独変更でキャッシュ済み PASS にならず失敗すること |
| AC-04 | `test` | `internal/maketestutil/make_test.go::TestRunTargetOpensMakefile`、`cmd/yt2column/test_cache_inputs_test.go::TestMakeHelpersOpenMakefile`、およびステップ 1-6 の壊し確認 | 1-1, 1-4, 1-6 | `RunTarget`・`makeDryRun`・`recipeStub.command` が `Makefile` を `open` 記録すること、および `Makefile` 単独変更で再実行されること |
| AC-05 | `test` | `internal/ciskip/workflow_pin_test.go::TestWorkflowDocGuardDisablesCache` | 1-5 | `.github/workflows/ci.yml` の `doc-guard` の `go test` コマンドに `-count=1` が含まれること |
| AC-06 | `test` | `internal/ciskip/workflow_pin_test.go::TestWorkflowDocGuardDisablesCache`、およびステップ 1-6 の壊し確認 | 1-5, 1-6 | `doc-guard` から `-count=1` を外したときにガードテストが失敗すること |
| AC-07 | `test` | `make test` および `make lint` | 1-6 | 全テストと静的解析が成功すること |

## 9. 次のステップ (Next Steps)

1. 本 3 文書 PR のレビューと `/runpr` での承認・マージを受ける。
2. マージ後、`/runplan 0132` でフェーズ 1 を実装し、実装 PR を開く。
