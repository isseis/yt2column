# アーキテクチャ設計書：スクリプト・Makefile を子プロセスに渡すテストのキャッシュ無効化と CI `doc-guard` の `-count=1` 付与

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

- **既存の前例への統一:** `cmd/yt2column/deadcode_test.go:210,680`（コミット `5bba819a`）と同じく、子プロセス（`bash`・`make`）に渡すリポジトリ内のファイルを Go のテストプロセス自身でも `os.ReadFile` で開くことで、Go のテストキャッシュ（`GOCACHE`）のキーに確実に含める。
- **共通ヘルパーへの集約:** 個別のテスト関数ではなく、子プロセスを起動する共通のテストヘルパー内で対象ファイルを読む。これにより、同じヘルパーを使う既存テストと今後追加されるテストの両方で書き忘れを防ぐ。
- **CI での予防的なキャッシュ無効化:** テストプロセス側でのファイル読み込みに加え、`.github/workflows/ci.yml` の `doc-guard` ジョブに `-count=1` を付与し、`actions/setup-go` のキャッシュ（将来 `go.sum` 等が加わった際に復元されうる `GOCACHE`）の再利用を防ぐ。`actions/checkout` が実行のたびにファイルの更新時刻を新しくするため、復元されたキャッシュは現時点の Go の挙動ではヒットしない。`-count=1` はこの挙動に頼らないための予防であり、CI で現に起きている穴の修正ではない。さらに既存のワークフロー検証テスト（`internal/ciskip/workflow_pin_test.go`）で `-count=1` の存在を固定する。
- **YAGNI・最小変更:** 新しいパッケージや外部依存を追加せず、既存の 4 パッケージのテストヘルパー／テストと `.github/workflows/ci.yml` のみで完結させる。

### 1.2. 設計の際に決めたこと（2026-11-28）

1. `internal/ciskip/workflow_pin_test.go` に `doc-guard` の `-count=1` を固定するガードテストを追加する。
   - 理由：`internal/ciskip/workflow_pin_test.go:67,575`（コミット `5bba819a`）は既に `.github/workflows/ci.yml` を解析して `doc-guard` ジョブと `steps[].runBody` を読み取っており、既存パーサを再利用して将来の `-count=1` 脱落を自動検出できるため。
   - ADR：なし（既存のワークフロー検証テストに 1 項目を追加するのみで、他タスクやコンポーネント境界を縛る主要決定ではない）。
2. キャッシュキー登録の検証は実装時の壊し確認（§7.2）で行い、登録を確かめる恒久的な自動テストは置かない。各 `os.ReadFile` には、`cmd/yt2column/deadcode_test.go:207-211` と同じく、テストキャッシュのキーに入れるために読む旨の英語コメントを付ける。
   - 理由：`go help test` は「モジュール内のファイルを開いたテストは、そのファイルが変わっていない実行にだけ一致する」と定めており（`cmd/go/internal/test/test.go:135-138`）、`os.ReadFile` による登録は文書化された挙動である。一方、登録を自動で確かめる手段は `cmd/go` 専用の非公開フラグ `-test.testlogfile`（`testing.go` のフラグ説明は「for use only by cmd/go」）しかなく、ログ形式も内部仕様のため、恒久テストにすると Go の更新で壊れうる。読み込みが将来消えても害は単独変更時のキャッシュ済み PASS に限られ、前例の `deadcode_test.go` も自動テストを持たないため、自動テストは維持費に見合わない（`CLAUDE.md`「Testing Strategy」）。
   - 採らなかった案：(a) `-test.testlogfile` で自テストバイナリを再実行する自動テスト（非公開機能に依存するため）、(b) 読み込み関数を差し替え可能にして呼び出しを記録するテスト（差し替え口が 4 パッケージに増え、確かめられるのも呼び出しまでのため）、(c) 読んだ内容を一時ファイルに書き出して実行する構造（スクリプトには使えるが、`$(MAKE)` で自身を再帰呼び出しする `Makefile` には使えないため）。
   - 人間の判断：2026-11-28、PR #1211 のレビューで (a) に代えて本案を選んだ。
   - ADR：なし（個別タスク内のテスト設計であり、主要決定ではない）。
3. `internal/ciskip` の実スクリプト実行テストで使うランナー生成を `internal/ciskip/test_helpers.go` の共通ヘルパーに集約する。
   - 理由：`internal/ciskip/test_helpers.go:145`（`(*testGitRepo).runners`）と `internal/ciskip/git_test.go:654`（`TestCheckExtensionChangesEarlyExitNoDeadlock`）の 2 箇所がそれぞれ `cmdrunner.NewOS(scriptOpts)` を組み立てており、共通ヘルパー（`newRealScriptRunner`）へ切り出すことで `hasExtensionChangesScript` の読み込みを 1 箇所に集約できるため。
   - ADR：なし（パッケージ内部のテストヘルパー整理であり、主要決定ではない）。

### 1.3. 概念モデル

Go の `go test` キャッシュはテストプロセス自身の `os.Open` 呼び出し（`testlog` の `open` 記録）を追跡し、子プロセスが開いたファイルは追跡しない。本設計では、子プロセスを起動するテストヘルパーの入口で対象ファイル（シェルスクリプトまたは `Makefile`）を Go プロセス自身が開くことで、対象ファイルの状態（サイズ・更新時刻等の属性）を Go テストキャッシュの検証対象に組み込む。CI の `doc-guard` ジョブではさらに `-count=1` を指定してキャッシュ参照自体を無効化する。

（簡易な進め方のため Mermaid 図は省略する。）

## 2. システム構成 (System Structure)

本タスクは本番バイナリ（CLI・記事生成モジュール・ブラウザ拡張）のコードを変更せず、テストコード（`//go:build test` のテストヘルパーおよび `*_test.go`）、CI ワークフロー定義（`.github/workflows/ci.yml`）、およびその引用箇所（`CLAUDE.md`）のみを対象とする。

対象とするパッケージと配置は次の 4 パッケージおよび CI ワークフロー・開発者向け規約である。

- `cmd/yt2column`（スクリプト実行ヘルパー・`Makefile` 実行ヘルパー）
- `internal/chief`（`high-review-decision.sh` との整合性テスト）
- `internal/ciskip`（`has-extension-changes.sh` の実スクリプトランナー生成ヘルパー・`doc-guard` ワークフローガードテスト）
- `internal/maketestutil`（`Makefile` ターゲット実行ヘルパー）
- `.github/workflows/ci.yml`・`CLAUDE.md`（`doc-guard` ジョブの `go test` コマンドとその引用）

（簡易な進め方のため Mermaid 図は省略する。）

## 3. コンポーネント設計 (Component Design)

### 3.1. データ構造・インターフェース拡張

N/A（既存の公開インターフェース・型定義・関数シグネチャの変更はない。`internal/ciskip/test_helpers.go` に追加する `newRealScriptRunner` はパッケージ非公開のテストヘルパーである）。

### 3.2. コンポーネント責務表

| ファイルパス | 区分 | 責務と変更内容 | 対応 AC |
|---|---|---|---|
| `cmd/yt2column/test_helpers_gitfixture.go` | 変更 | `(*gitFixture).runScript`（163 行目）で `filepath.Abs(script)` の直後に対象スクリプトを `os.ReadFile` で読み、読み込み失敗時は `f.t.Fatalf` で即座に失敗させる。これにより `review_records_test.go`・`high_review_decision_test.go`・`adr_candidates_docs_test.go` の 3 テストすべてを対象にできる | AC-01 |
| `cmd/yt2column/makefile_test.go` | 変更 | `makeDryRun`（215 行目）および `recipeStub.command`（415 行目）で `filepath.Join(repositoryRoot, "Makefile")` を `os.ReadFile` で読み、失敗時は `t.Fatalf` で失敗させる | AC-04 |
| `internal/chief/markers_parity_test.go` | 変更 | `TestMarkerReadingMatchesDecisionScript`（64 行目付近）で `decisionScript` の絶対パスを `os.ReadFile` で読み、失敗時は `t.Fatalf` で失敗させる | AC-02 |
| `internal/ciskip/test_helpers.go` | 変更 | `newRealScriptRunner(t *testing.T) cmdrunner.InputRunner` を追加し、`scriptOpts.Dir` をリポジトリルートに設定すると同時に `hasExtensionChangesScript` を `os.ReadFile` で読む。`(*testGitRepo).runners`（145 行目）からこれを呼ぶ | AC-03 |
| `internal/ciskip/git_test.go` | 変更 | `TestCheckExtensionChangesEarlyExitNoDeadlock`（654 行目）の実スクリプトランナー生成を `newRealScriptRunner(t)` に置き換える | AC-03 |
| `.github/workflows/ci.yml` | 変更 | `doc-guard` ジョブ（291 行目）の `go test` コマンドに `-count=1` を追加する | AC-05 |
| `CLAUDE.md` | 変更 | 959 行目の `doc-guard` 実行コマンドの引用に `-count=1` を反映し、`.github/workflows/ci.yml` と一致させる | AC-05 |
| `internal/ciskip/workflow_pin_test.go` | 変更 | `doc-guard` ジョブの検証に、実行ステップの `go test` コマンドに `-count=1` が含まれることの確認を追加する | AC-06 |
| `internal/maketestutil/make.go` | 変更 | `RunTarget`（106 行目）で `filepath.Join(root, "Makefile")` を `os.ReadFile` で読み、失敗時は `t.Fatalf` で失敗させる。これにより `internal/transcript`・`internal/llm/{deepseek,claude,openai}`・`cmd/yt2column` の各 `makefile_test.go` をまとめて対象にできる | AC-04 |

既存テストへの影響：既存テストが前提とする振る舞いは変わらず、既存のテストを修正・削除する必要はない（`internal/ciskip/git_test.go:655-657` のみ共通ヘルパー呼び出しへ置き換える）。

## 4. エラーハンドリング設計 (Error Handling Design)

- テストヘルパーまたはテスト関数が対象ファイル（スクリプト・`Makefile`）の `os.ReadFile` に失敗した場合、テストを黙って続行したりスキップしたりせず、ファイルパスとエラー内容を添えて `t.Fatalf` で即座に失敗させる。
- 新しいエラー型の定義は不要（N/A）。

## 5. セキュリティ考慮事項 (Security Considerations)

N/A（`01_requirements.md` §4.2 のとおり、`docs/dev/security.md` の適用条件に該当しない。テストコード内の `os.ReadFile` で `gosec` G304 が反応する箇所には、既存の `internal/maketestutil/make.go:274` や `cmd/yt2column/makefile_test.go:473` と同様に、テスト専用の対象ファイルであることを示す最小限の `//nolint:gosec` と理由コメントを付ける）。

## 6. 処理フロー詳細 (Processing Flow Details)

### 6.1. テスト実行時のファイル登録フロー

1. 各テストまたは共通ヘルパーが子プロセス（`bash`・`make`）を起動する前に、渡すリポジトリ内ファイルのパスを解決する。
2. 解決したパスに対して Go のテストプロセス自身で `os.ReadFile` を実行する（失敗時は `t.Fatalf`）。この呼び出しにより Go ランタイムが `testlog.Open` へパスを記録し、`go test` がファイルのサイズと更新時刻をテストキャッシュキーに含める。
3. 従来どおり子プロセスを起動してテストを実行する。

## 7. テスト戦略 (Test Strategy)

### 7.1. ユニットテスト

- **`doc-guard` ワークフローガードテスト:** `internal/ciskip/workflow_pin_test.go` で `.github/workflows/ci.yml` の `doc-guard` ジョブの `go test` ステップを検索し、行内コメント（`#` 以降）を除いたコマンド文字列をトークン単位（`strings.Fields`）で解析して、すべての `go test` 実行コマンドに引数として `-count=1` が含まれること、および `go test` ステップが 1 件以上存在すること（空振りによる偽陽性防止）を確かめる。

### 7.2. 統合テスト・壊し確認

- 実装フェーズで、issue #1205 の再現手順に沿って以下を実機確認し、コミットメッセージに記録する。
  1. 修正前の状態で、スクリプト（例：`.claude/commands/_lib/high-review-decision.sh`）を壊してもキャッシュ済みの PASS が返ることを確かめる。
  2. 修正後の状態で、各対象ファイル（`review-records.sh`・`high-review-decision.sh`・`adr-correction-candidates.sh`・`has-extension-changes.sh`・`Makefile`）だけを壊す変更をしたあとの `go test`／`make test` がキャッシュを使わずに再実行されて失敗することを確かめる（壊しは確認後に戻す）。
  3. 6 箇所（`RunTarget`・`newRealScriptRunner`・`TestMarkerReadingMatchesDecisionScript`・`runScript`・`makeDryRun`・`recipeStub.command`）について、その箇所の `os.ReadFile` だけを一時的に外した状態で対象ファイルを変えると `(cached)` の PASS に戻り、戻すと再実行されることを確かめる。これにより、各箇所の読み込みが再実行の原因であることを示す。実行は、その箇所を通るテストだけに `-run` で絞る。同じファイルを読む別の箇所が同じ実行に入ると、読み込みを外しても再実行されてしまうためである。`cmd/yt2column` では、既存の `deadcode_test.go:210,680` も `Makefile` を読む。`makeDryRun` は `TestMakeWasmCheck`、`recipeStub.command` は `TestCoverageCompareRecipeForwardsSignal` のように、そのヘルパーだけを使うテストに絞る。
  4. `doc-guard` の `-count=1` を外したときに `workflow_pin_test.go` のガードが失敗することを確かめる。
- 最後に `make test` と `make lint` の全通過を確かめる。

### 7.3. セキュリティテスト

N/A（§5 のとおりセキュリティ要件の対象外）。

## 8. 実装の優先順位 (Implementation Priorities)

変更規模が小さく全体が 1 つの関心事で完結するため、単一のフェーズ（フェーズ 1）で次の順に実装する。

1. `internal/maketestutil/make.go`（複数パッケージが依存する共通ヘルパーを先に修正）
2. `internal/ciskip/test_helpers.go`・`internal/ciskip/git_test.go`（`has-extension-changes.sh` の読み込み集約）
3. `internal/chief/markers_parity_test.go`（`high-review-decision.sh` の読み込み）
4. `cmd/yt2column/test_helpers_gitfixture.go`・`cmd/yt2column/makefile_test.go`（スクリプト・`Makefile` ヘルパーの修正）
5. `.github/workflows/ci.yml`・`CLAUDE.md`・`internal/ciskip/workflow_pin_test.go`（`doc-guard` への `-count=1` 追加、引用更新、およびガードテスト）
6. 壊し確認と全体検証（`make test && make lint`）の実行

## 9. 将来の拡張性 (Future Extensibility)

- [#1181](https://github.com/isseis/yt2column/issues/1181) で開発ツール系テストを `cmd/yt2column` から別パッケージへ分離する際も、`runScript` を移動先の共通ヘルパーとして引き継ぐだけでキャッシュキー登録の性質が保たれる。
- 今後新たにリポジトリ内のスクリプトや設定ファイルを子プロセスへ渡すテストを追加する場合は、既存の共通ヘルパー（`runScript`・`maketestutil.RunTarget`・`newRealScriptRunner`）を経由するか、同様に Go プロセスで `os.ReadFile` してから子プロセスを起動する。
