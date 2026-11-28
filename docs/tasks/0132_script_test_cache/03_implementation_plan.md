# 実装計画書：スクリプト・Makefile を子プロセスに渡すテストのキャッシュ無効化と CI `doc-guard` の `-count=1` 付与

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-28 |
| Review date | 2026-11-28 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md` および `02_architecture.md` に基づき、リポジトリ内のシェルスクリプトや `Makefile` を子プロセスに渡す Go テストが Go プロセス自身でも対象ファイルを開くように修正し、CI の `doc-guard` ジョブに `-count=1` を追加してガードテストで固定する。

### 1.2. 実装方針

- `02_architecture.md` §3.2 の責務表と §8 の実装優先順位に従い、共通のテストヘルパーから順に修正する。
- Go のソースコード・コメント・識別子・文字列リテラルはすべて英語で書く（`CLAUDE.md`「Language」）。
- テストヘルパーは既存の `//go:build test` 構成を維持し、不要な新規ヘルパーファイルや `testutil/` は増やさない（`docs/dev/conventions/test_organization.md`）。
- `_test.go` 以外のテストヘルパー（`internal/maketestutil/make.go`・`cmd/yt2column/test_helpers_gitfixture.go`・`internal/ciskip/test_helpers.go`）の `os.ReadFile` で `gosec`（G304）が反応する場合は、該当行に最小限の `//nolint:gosec` とテスト専用パスである旨の英語コメントを付ける。

### 1.3. 既存コード調査結果

コミット `5bba819a` 時点で確認した既存コードの状況は以下のとおり。

- **`internal/maketestutil/make.go:106`（`RunTarget`）**: `make -s <target> GOTEST=<stub>` を `cmd.Dir = root` で実行するが、`filepath.Join(root, "Makefile")` を Go プロセスで開いていない。`internal/transcript/makefile_test.go`・`internal/llm/{deepseek,claude,openai}/makefile_test.go`・`cmd/yt2column/makefile_test.go` の 5 パッケージが利用している。`internal/maketestutil/make_test.go` が既存のユニットテストファイルとして存在する。
- **`internal/ciskip/test_helpers.go:145`（`(*testGitRepo).runners`）と `internal/ciskip/git_test.go:654`（`TestCheckExtensionChangesEarlyExitNoDeadlock`）**: どちらも `scriptOpts.Dir = filepath.Join("..", "..")` で `cmdrunner.NewOS(scriptOpts)` を生成し、`internal/ciskip/git.go:210` から `bash extension/scripts/has-extension-changes.sh` を呼ぶが、同スクリプトを Go プロセスで開いていない。
- **`internal/chief/markers_parity_test.go:64,107`（`TestMarkerReadingMatchesDecisionScript`）**: `decisionScript`（`../../.claude/commands/_lib/high-review-decision.sh`）を `filepath.Abs` で解決して `repo.Run(testBash, script, ...)` に渡すが、Go プロセスで開いていない。また同関数（44–49 行目）は他の並列テストからの `flock` 継承を避けるためトップレベルを直列実行している。
- **`cmd/yt2column/test_helpers_gitfixture.go:163`（`(*gitFixture).runScript`）**: `filepath.Abs(script)` を `bash.path` に渡すが、Go プロセスで開いていない。`review_records_test.go`・`high_review_decision_test.go`・`adr_candidates_docs_test.go` の 3 テストすべてがここを経由する。
- **`cmd/yt2column/makefile_test.go:215,415`（`makeDryRun`・`recipeStub.command`）**: `cmd.Dir = repositoryRoot` で `make` を実行するが、`Makefile` を Go プロセスで開いていない（`cmd/yt2column/deadcode_test.go:210,680` は既に `os.ReadFile` 済み）。
- **`.github/workflows/ci.yml:277-291`（`doc-guard`）・`CLAUDE.md:959`・`internal/ciskip/workflow_pin_test.go:67,575`**: `doc-guard` は `actions/setup-go@v6`（`cache: true`）を使用するが、`go test -tags test ...` に `-count=1` がない。`CLAUDE.md:959` も同じコマンドを引用しているため合わせて更新する。`workflow_pin_test.go` の `readWorkflow` は `wf.jobs["doc-guard"].steps` と各ステップの `runBody` を既に解析済みである。
- **外部前提の確認（Go のテストキャッシュ）**: `go help test`（`cmd/go/internal/test/test.go:135-138`）は、モジュール内のファイルを開いたテストはそのファイルが変わっていない実行にだけキャッシュが一致すると定めている。登録の確認は壊し確認（ステップ 1-6）で行い、`cmd/go` 専用の非公開フラグ `-test.testlogfile` には依存しない（`02_architecture.md` §1.2 の 2）。

### 1.4. 計画の際に決めたこと（2026-11-28）

1. 全ステップを 1 フェーズ・1 PR にまとめる。
   - 理由：4 パッケージのテストヘルパー修正と `.github/workflows/ci.yml` の 1 行修正は規模が小さく、「スクリプト・Makefile 単独変更時のテストキャッシュ漏れ防止」という単一の関心事で完結しているため。
   - ADR：なし（PR 分割はタスク固有の実装計画上の判断であり、主要決定ではない）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: テストプロセスでのファイル読み込み追加と CI `doc-guard` の `-count=1` 固定

#### ステップ 1-1: `internal/maketestutil` の `Makefile` 読み込み

- **対象ファイル**: `internal/maketestutil/make.go`
- **作業内容**:
  - [x] `internal/maketestutil/make.go` の `RunTarget` で、`make` を起動する前に `filepath.Join(root, "Makefile")` を `os.ReadFile` で読み込み、失敗時は `t.Fatalf` で失敗させる（`gosec`（G304）が反応する場合は、テスト専用パスである旨の英語コメント付きで該当行のみに `//nolint:gosec` を付与する）。
  - [x] 読み込み行に、テストキャッシュのキーに `Makefile` を入れるために読む旨の英語コメントを付ける（`cmd/yt2column/deadcode_test.go:207-209` と同じ形）。
- **完了条件**: `go test -tags test -count=1 ./internal/maketestutil` が成功する。

#### ステップ 1-2: `internal/ciskip` の `has-extension-changes.sh` 読み込み集約

- **対象ファイル**: `internal/ciskip/test_helpers.go`, `internal/ciskip/git_test.go`
- **作業内容**:
  - [x] `internal/ciskip/test_helpers.go` にパッケージ非公開ヘルパー `newRealScriptRunner(t *testing.T) cmdrunner.InputRunner` を追加し、リポジトリルートを基準に `hasExtensionChangesScript` を `os.ReadFile` で読んで失敗時は `t.Fatalf` としたうえで `cmdrunner.NewOS(scriptOpts)` を返す（必要に応じて該当行のみ `//nolint:gosec` を付与する）。
  - [x] `(*testGitRepo).runners` と `internal/ciskip/git_test.go` の `TestCheckExtensionChangesEarlyExitNoDeadlock` が `newRealScriptRunner(t)` を使うように置き換える。
  - [x] 読み込み行に、テストキャッシュのキーにスクリプトを入れるために読む旨の英語コメントを付ける。
- **完了条件**: `go test -tags test -count=1 ./internal/ciskip` が成功する。

#### ステップ 1-3: `internal/chief/markers_parity_test.go` のスクリプト読み込み

- **対象ファイル**: `internal/chief/markers_parity_test.go`
- **作業内容**:
  - [x] `TestMarkerReadingMatchesDecisionScript` で `filepath.Abs(decisionScript)` の直後に `os.ReadFile(script)` を実行し、失敗時は `t.Fatalf` で失敗させる。
  - [x] 読み込み行に、テストキャッシュのキーにスクリプトを入れるために読む旨の英語コメントを付ける。
- **完了条件**: `go test -tags test -count=1 ./internal/chief` が成功する。

#### ステップ 1-4: `cmd/yt2column` のスクリプト・`Makefile` ヘルパー修正

- **対象ファイル**: `cmd/yt2column/test_helpers_gitfixture.go`, `cmd/yt2column/makefile_test.go`
- **作業内容**:
  - [x] `cmd/yt2column/test_helpers_gitfixture.go` の `(*gitFixture).runScript` で、`filepath.Abs(script)` の直後に `os.ReadFile(abs)` を実行し、失敗時は `f.t.Fatalf` で失敗させる（必要に応じて該当行のみ `//nolint:gosec` を付与する）。
  - [x] `cmd/yt2column/makefile_test.go` の `makeDryRun` と `recipeStub.command` で、`filepath.Join(repositoryRoot, "Makefile")` を `os.ReadFile` で読み込み、失敗時は `t.Fatalf` で失敗させる。
  - [x] 3 箇所の読み込み行に、テストキャッシュのキーに対象ファイルを入れるために読む旨の英語コメントを付ける。
- **完了条件**: `go test -tags test -count=1 ./cmd/yt2column` が成功する。

#### ステップ 1-5: CI `doc-guard` への `-count=1` 追加、`CLAUDE.md` 更新、およびワークフローガードテスト追加

- **対象ファイル**: `.github/workflows/ci.yml`, `CLAUDE.md`, `internal/ciskip/workflow_pin_test.go`
- **作業内容**:
  - [x] `.github/workflows/ci.yml` の `doc-guard` ジョブ（`Run documentation sync guards` ステップ）の `go test` コマンドに `-count=1` を追加する。
  - [x] `CLAUDE.md:959` の `doc-guard` 実行コマンドの引用を `go test -tags test -count=1 ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief` に更新する。
  - [x] `internal/ciskip/workflow_pin_test.go` に `TestWorkflowDocGuardDisablesCache` を追加し、`doc-guard` ジョブに `go test` を実行するステップが 1 件以上存在し、行内コメント（`#` 以降）を除いたすべての `go test` コマンドに引数トークンとして `-count=1` が含まれることを確かめる。
- **完了条件**: `go test -tags test -count=1 ./internal/ciskip` が成功する。

#### ステップ 1-6: 壊し確認と全体検証

- **対象ファイル**: なし（検証ステップ）
- **作業内容**:
  - [x] 修正前コミット（または `os.ReadFile` を一時的に外した状態）で、issue #1205 の手順どおりスクリプトを壊したときに `(cached)` で PASS することを確かめる。
  - [x] 修正後の状態で、`review-records.sh`・`high-review-decision.sh`・`adr-correction-candidates.sh`・`has-extension-changes.sh`・`Makefile` のそれぞれについて単独変更（壊し）を行い、対応するテストがキャッシュを使わず再実行されて失敗することを確かめ、変更を元に戻す。
  - [x] 6 箇所（`RunTarget`・`newRealScriptRunner`・`TestMarkerReadingMatchesDecisionScript`・`runScript`・`makeDryRun`・`recipeStub.command`）のそれぞれについて、その箇所の `os.ReadFile` だけを一時的に外した状態で対象ファイルを変えると `(cached)` の PASS に戻り、戻すと再実行されることを確かめ、元に戻す（`02_architecture.md` §7.2 の手順 3）。実行は `-run` でその箇所を通るテストだけに絞る。`cmd/yt2column` では、`Makefile` を読む既存の `deadcode_test.go:210,680` ともう一方のヘルパーが同じ実行に入らないようにする（`makeDryRun` は `TestMakeWasmCheck`、`recipeStub.command` は `TestCoverageCompareRecipeForwardsSignal` など）。
  - [x] `.github/workflows/ci.yml` の `doc-guard` から `-count=1` を一時的に外したときに `TestWorkflowDocGuardDisablesCache` が失敗することを確かめ、元に戻す。
  - [x] 壊し確認の結果をコミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。
  - [x] `make fmt`・`make test`・`make lint` を実行し、すべて成功することを確かめる。
- **完了条件**: すべての壊し確認が期待どおり失敗し、元に戻した状態で `make test && make lint` が成功する。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

- **M1（フェーズ 1 完了）**: 4 パッケージのキャッシュキー登録修正、`doc-guard` の `-count=1` 付与、壊し確認、および `make test && make lint` の通過。

### 3.2. PR 構成

本計画は単一フェーズ（ステップ 1-1〜1-6）であり、PR 境界マーカーは設けず 1 本の実装 PR で完結させる。

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 1-1・1-2・1-4 が Conditional check（`_test.go` 以外のヘルパーへの限定 `//nolint:gosec` 付与）に該当し、ステップ 1-6 の壊し確認は、箇所ごとに読み込みを外して `-run` を正しく絞らないと別の読み込み（`deadcode_test.go` や同じ `Makefile` を読む別ヘルパー）に覆われて確認が成り立たないため。上位モデル未満で実装すると、`//nolint:gosec` の範囲漏れや、成り立たない壊し確認を成功と記録するおそれがある。

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

- **キャッシュキー登録**: 恒久的な自動テストは置かず、ステップ 1-6 の壊し確認で各箇所の読み込みが再実行の原因であることを確かめ、結果をコミットメッセージに記録する（`02_architecture.md` §1.2 の 2）。
- **ワークフローガードテスト**: `internal/ciskip/workflow_pin_test.go` で `doc-guard` の `go test` コマンドに `-count=1` が含まれることを確かめる。
- **壊し確認**: ステップ 1-6 で、各対象ファイルの単独変更によるキャッシュ無効化と、`doc-guard` の `-count=1` 削除によるガード失敗を実機で確認する。
- **回帰確認**: `make test`（`-race` 実行＋`test-parse-norace`）および `make lint`（`wasm-check`・`golangci-lint --build-tags test,integration`・`go vet -tags integration ./...`）を実行する。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| `_test.go` 以外のテストヘルパーの `os.ReadFile` が `gosec`（G304）で失敗する | `make lint` が通らない | 該当行のみに最小限の `//nolint:gosec` とテスト専用ファイルである旨の英語コメントを付け、`make lint` で確認する |
| 将来の変更で `os.ReadFile` が消えても自動テストが検出しない | 対象ファイルの単独変更時にキャッシュ済みの PASS が再び返る（CI の `doc-guard` は `-count=1` で影響を受けない） | 各読み込み行に理由の英語コメントを付け、`cmd/yt2column/deadcode_test.go:207-211` と同じ前例に揃えて削除されにくくする |
| 壊し確認で同じファイルを読む別の箇所が同じ実行に入る | 読み込みを外しても再実行され、確認が成り立たないまま成功と記録される | ステップ 1-6 のとおり `-run` で箇所ごとに実行を絞り、読み込みを外した状態で `(cached)` に戻ることを先に確かめる |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] ステップ 1-1: `internal/maketestutil` の `Makefile` 読み込み完了
- [x] ステップ 1-2: `internal/ciskip` の `newRealScriptRunner` 集約完了
- [x] ステップ 1-3: `internal/chief/markers_parity_test.go` のスクリプト読み込み完了
- [x] ステップ 1-4: `cmd/yt2column` の `runScript`・`makeDryRun`・`recipeStub.command` 修正完了
- [x] ステップ 1-5: `.github/workflows/ci.yml` の `doc-guard` `-count=1` 追加、`CLAUDE.md` 更新、および `TestWorkflowDocGuardDisablesCache` 追加完了
- [x] ステップ 1-6: 壊し確認および `make fmt`・`make test`・`make lint` 通過
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1213
- [x] PR がマージされた

## 7. 完了基準 (Success Criteria)

- **機能**: `01_requirements.md` の AC-01〜AC-07 をすべて満たす。
- **品質**: `make test` と `make lint` がエラーなく通過し、すべての壊し確認がコミットメッセージに記録されている。
- **セキュリティ**: N/A（`01_requirements.md` §4.2 および `02_architecture.md` §5 のとおりセキュリティ要件の対象外）。
- **文書**: 本タスクの 3 文書に issue #1205 の「要確認」各項目の該当有無と対処が記録されている。

## 8. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種別 | 検証箇所 | 対応ステップ | 確認内容 |
|---|---|---|---|---|
| AC-01 | `test` | ステップ 1-6 の壊し確認（`go test` の実行結果をコミットメッセージに記録） | 1-4, 1-6 | `runScript` の読み込みを外すと `(cached)` に戻り、戻すと各スクリプトの単独変更でキャッシュ済みの PASS にならず失敗すること |
| AC-02 | `test` | ステップ 1-6 の壊し確認（同上） | 1-3, 1-6 | `TestMarkerReadingMatchesDecisionScript` の読み込みを外すと `(cached)` に戻り、戻すと `high-review-decision.sh` の単独変更でキャッシュ済みの PASS にならず失敗すること |
| AC-03 | `test` | ステップ 1-6 の壊し確認（同上） | 1-2, 1-6 | `newRealScriptRunner` の読み込みを外すと `(cached)` に戻り、戻すと `has-extension-changes.sh` の単独変更でキャッシュ済みの PASS にならず失敗すること |
| AC-04 | `test` | ステップ 1-6 の壊し確認（同上） | 1-1, 1-4, 1-6 | `RunTarget`・`makeDryRun`・`recipeStub.command` のそれぞれについて、読み込みを外すと `(cached)` に戻り、戻すと `Makefile` の単独変更で再実行されること |
| AC-05 | `test` | `internal/ciskip/workflow_pin_test.go::TestWorkflowDocGuardDisablesCache` | 1-5 | `.github/workflows/ci.yml` の `doc-guard` の `go test` コマンドに `-count=1` が含まれること |
| AC-06 | `test` | `internal/ciskip/workflow_pin_test.go::TestWorkflowDocGuardDisablesCache`、およびステップ 1-6 の壊し確認 | 1-5, 1-6 | `doc-guard` から `-count=1` を外したときにガードテストが失敗すること |
| AC-07 | `test` | `make test` および `make lint` | 1-6 | 全テストと静的解析が成功すること |

## 9. 次のステップ (Next Steps)

1. 本 3 文書 PR のレビューと `/runpr` での承認・マージを受ける。
2. マージ後、`/runplan 0132` でフェーズ 1 を実装し、実装 PR を開く。

## 変更履歴

- 2026-11-28 承認（isseis、#1211）
