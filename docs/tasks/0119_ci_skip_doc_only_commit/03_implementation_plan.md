# 実装計画書：直前コミットで CI 通過済みかつドキュメントのみの変更時に重いジョブをスキップする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-21 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番であり、すべて設計書の付録 A に扱いが記録されている。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `d45909cf` で確かめた。設計書が確かめたコミット `b004f8c0` から `d45909cf` までの差分（`git diff --stat b004f8c0 d45909cf`）は、本タスクの文書・ADR-0014・ADR-0054・`docs/translation_glossary.md` の 5 ファイルのみであり、設計書が引用する既存コードと設定の行番号は今も一致する。

用語は、要件書 §6 および設計書冒頭と同じ意味で使う（重いジョブ・常時実行ジョブ・コードファイル／非コードファイル・拡張対象ファイル／非拡張ファイル・`PRE_MERGE` コミット・PR head コミット・親コミット・通常分類・安全側フォールバック・食い違いの記述・`ciskip`・合成マージコミット・記録済みベース SHA・判定理由）。

---

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおり、Go 製の CI 専用ツール `cmd/ciskip` + `internal/ciskip` を新設し、`.github/workflows/ci.yml` の `check-changes` ジョブへ統合する。あわせて、常設ピン留めテスト・リポジトリ規約の更新・ADR-0054／ADR-0014 の整合・GitHub Actions 上での実機確認記録を行い、要件書の F-001〜F-006（AC-01〜AC-20）を満たす。

### 1.2. 実装の原則

-   **設計書 §8 の 3 フェーズの順に進める。** フェーズ 1（`internal/ciskip` コアとテスト）、フェーズ 2（`cmd/ciskip`・`ci.yml` 統合・ピン留めテスト・規約更新）、フェーズ 3（ADR 整合と GitHub Actions 実機確認）の順とし、各フェーズを 1 本の PR に対応するまとまりとする（§1.3 の決定 1）。
-   **既存部品を再利用し、外部依存を追加しない。** 外部コマンド実行は `internal/cmdrunner`（`GitHardening`・`GitEnv`・`MaxOutput`・`ProcessGroup`）、シグナル処理は `internal/signalrun`、厳密 JSON 解析は `internal/strictjson`、拡張対象分類は既存の `extension/scripts/has-extension-changes.sh` をそのまま使う。
-   **すべての異常系を安全側（通常分類）へ倒す。** `Evaluator.Evaluate` は運用上の失敗をすべて非スキップの `Reason` へ写像し、`cmd/ciskip` の診断ログ・標準エラー・結果ファイルへ機密情報や非 200 応答の生本文を出さない（設計書 §4.1・§4.2）。
-   **既存テストとガードを弱めずにグリーンゲートを通す。** `internal/ciskip` を追加するフェーズ 1 の時点で、`make test` のガードが要求する `package_reference.md` の更新と `CLAUDE.md` の例外記述を合わせて入れる。
-   **テストは壊して失敗することを確かめる。** 新設するテストと `_checks/` のスクリプトは、対象となる判定や条件を壊したときに失敗することを実装時に確認し、コミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-21）

本節は、設計書が計画に委ねた PR 分割・実機確認手順・AC-16 検証方式の 2 点について、利用者との確認で決めた内容を記録する。

1.  **PR は 3 本に分ける（PR-1: フェーズ 1、PR-2: フェーズ 2、PR-3: フェーズ 3、`PR-3` は `自動送信: hold`）。**
    -   **決定:** PR-1 に `internal/ciskip` コア実装・テストおよびグリーンゲートと規約整合に必要な `package_reference.md`（`internal/ciskip` 行・`internal/strictjson` 利用者一覧）と `CLAUDE.md`（「Declare, don't infer」例外）を含める。PR-2 に `cmd/ciskip`・`.github/workflows/ci.yml`・`workflow_pin_test.go`・`envaccess_test.go`・`package_reference.md`（`cmd/ciskip` 行・`Development tools` 定義拡張）・`dev_tools.md`・AC-16 検証スクリプトを含める。PR-3 で PR-2 の GitHub Actions 実行結果（コード変更コミットでの通常実行と、`/runpr` の `PRE_MERGE` コミットでのスキップ）を `03_implementation_plan.md` に記録し（AC-20）、ADR-0054・ADR-0014 の食い違い記述を除去する（AC-19）。
    -   **理由:** PR-1 で判定ロジックとセキュリティ境界を `ci.yml` から分離してレビューしつつ単体でグリーンゲートと `make deadcode` を通せる。さらに PR-2 のコード変更コミットとマージ直前の `PRE_MERGE` コミットで AC-20 の 2 ケースを実運用そのままに検証でき、PR-3 は最終 PR の作成前チェック時点で全 AC が完了した状態にできる。PR-2 マージ後に利用者が PR-2 の実行結果（`::notice` 読み戻しやレビュー中の `main` 更新有無）を確かめてから PR-3 を始めるため、PR-3 の `自動送信` は `hold` とする。
    -   **人の答え:** 案 A（3 PR 構成、PR-3 は `hold`）。
    -   **ADR:** なし（本タスクの PR 分割と検証手順であり、タスクの中で閉じる）。
2.  **AC-16（`.github/workflows/ci.yml` の差分範囲）の `diff` 検証は `_checks/` のシェルスクリプトで行う。**
    -   **決定:** `docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh` を PR-2 で作成し、merge-base と比較対象コミットの `.github/workflows/ci.yml` から `  check-changes:` より前の部分と次のジョブ（`  test:`）以降末尾までをそれぞれ抽出し、`cmp` でバイト一致を検証する。
    -   **理由:** `check-changes` の変更は 45〜60 行程度のワークフロー YAML 差分になる見込みであり、`requirements_process.md` §4 が「差分の読み取り」の前提とする小規模な文書変更に当たらない。約 20 行のシェルスクリプトの方が変更規模と釣り合い、計画書が長大な YAML 差分で肥大化するのを防げる。
    -   **人の答え:** 案 A（`_checks/` のシェルスクリプト）。
    -   **ADR:** なし（本タスクの 1 つの AC の検証方式であり、タスクの中で閉じる）。

### 1.4. 既存コード調査結果

コミット `d45909cf` で既存のコード・テスト・設定・外部ツール前提を調査した結果をまとめる。変更不要な領域は省略する。

| 領域 | 今あるもの（`d45909cf`） | 足りないもの・変えるもの |
|---|---|---|
| CI ワークフロー | `.github/workflows/ci.yml:7-8`（全体 `permissions: contents: read`）、`:12-73`（`check-changes`、`id: check` の 1 ステップで通常分類）、`:74-234`（他 5 ジョブ） | `check-changes` のみを変更する。ジョブ限定 `permissions`（`contents`・`actions`・`checks` 各 `read`）と `timeout-minutes: 10` を追加し、通常分類を `id: normal` へ移して後段に `id: check`（`cmd/ciskip` 呼び出し・最終出力）を置く（設計書 §3.3.4） |
| 拡張 CI ピンテスト | `extension/test/repository.test.ts:35-55`（インデント基準のジョブ・ステップ切り出し）、`:143-251`（`steps.check.outputs.has-extension-changes`、`git diff -z`、`wasm-dep-dirs.sh`、`has-extension-changes.sh`、`else` 直後の `has_extension_changes=true` を検証） | 既存テストは変更せずに通す。`:232-235` の正規表現が `else` 直後に `has_extension_changes=true`（間に `echo` 1 行まで）を要求するため、`wasm_dep_dirs_ok=false` は `if` の前に初期化し、`then` 内で `true` にする。`id: check` 側のフォールバック伝播は `internal/ciskip/workflow_pin_test.go` でピン留めする |
| 拡張変更分類スクリプト | `extension/scripts/wasm-dep-dirs.sh:1-39`（非空の `/` 終端行を出力）、`extension/scripts/has-extension-changes.sh:28-62`（依存ファイルが空でもエラーにせず `false` を返す） | スクリプト自体は変更しない。`internal/ciskip` 側で `depDirsFile` の非空・`/` 終端を独立検証したうえで、`has-extension-changes.sh` を `cmdrunner.InputRunner.RunInput`（`StderrIsFailure: true`）で再利用する |
| 外部コマンド実行・fake | `internal/cmdrunner/runner.go:25-60,96-179`（`Runner`・`InputRunner`・`Options`・`ExitError`・`ErrOutputTooLarge`・`ErrUnexpectedStderr`）、`internal/cmdrunner/git.go:19-45`（`GitHardening`・`GitEnv`）、`internal/cmdrunner/testutil/mocks.go:24-89`（`Fake`・`Step`） | `internal/ciskip` がそのまま再利用する。`ProcessGroup: true` は標準入力がパイプバッファに収まる前提（`runner.go:170-173`）のため、`script` ランナーは `ProcessGroup: false` とする（設計書 §3.4 注 1）。一時 git リポジトリ結合テストでは `git` を `t.TempDir()` で走らせるため、`script` ランナーの作業ディレクトリまたは非公開パスでリポジトリ直下の `extension/scripts/has-extension-changes.sh` を参照できるようにする |
| 厳密 JSON・シグナル処理 | `internal/strictjson/strictjson.go:74,130,284,326`（`ParseObject`・`ParseArray`・`CollectExactCase`・`AsNonNegativeInt64`）、`internal/signalrun/signalrun.go:42`（`Run`） | `internal/ciskip` が `strictjson` を、`cmd/ciskip` が `signalrun.Run` を再利用する |
| `gh api -i` フレーミングの先行実装 | `internal/numreserve/tool.go:45-70,98-100`（`gh` 環境変数除外と `--hostname github.com -i`）、`internal/numreserve/reserve.go:181-242`（LF ステータス行＋CRLF ヘッダ分離） | 既存ツールは変更せず、`internal/ciskip/github.go` 内に閉じた実装を置く（設計書 §3.3.3） |
| 環境変数アクセスガード | `internal/config/envaccess_test.go:65-102`（`allowedEnvRefs` に各 `cmd/*/main.go` の `call:os.Environ: 1` を登録） | `"cmd/ciskip/main.go": {"call:os.Environ": 1}` を 1 行追加する（フェーズ 2） |
| パッケージ一覧・`strictjson` 利用者ガード | `internal/pipeline/pipeline_test.go:630-669`（`TestPackageReferenceListsPackages`）、`internal/pipeline/strictjson_users_test.go:654-685`（`TestPackageReferenceStrictjsonUsers`）、`docs/dev/conventions/package_reference.md:52-58,109,112-129` | フェーズ 1 で `internal/ciskip` 行と `internal/strictjson` の利用者一覧を追加し、フェーズ 2 で `Development tools` 定義への CI 専用ツール追記と `cmd/ciskip` 行を追加する |
| 開発ツール規約・`CLAUDE.md`・ADR | `docs/dev/conventions/dev_tools.md:10-24,140-142`、`CLAUDE.md:405-620`、`docs/dev/adr/0054-ci-skip-doc-only-commit.md:40`、`docs/dev/adr/0014-deterministic-checks-in-go.md:31`、`cmd/yt2column/adr_test.go:425`（`TestADRRecords`） | フェーズ 1 で `CLAUDE.md`（「Declare, don't infer」例外）を更新し、フェーズ 2 で `dev_tools.md`（CI 専用ツール区分と Rule 5 適用除外）を更新し、フェーズ 3 で ADR-0054 と ADR-0014 の本タスクの食い違い記述を除去する（ADR-0014:50 の `codexreview` 食い違いは別タスクなので触れない） |
| テスト補助のビルドタグガード | `internal/pipeline/pipeline_test.go:554-602`（`TestFakesCarryBuildTag`：`testutil/` ファイル数は 24 固定、`test_helpers*.go` は先頭行 `//go:build test` または `//go:build test || integration` を要求） | `testutil/` は新設しない（`internal/cmdrunner/testutil` を再利用）。`internal/ciskip` のテスト間で共有する補助は `internal/ciskip/test_helpers.go`（先頭行 `//go:build test`）に置く |

**具象型の構築・変更経路の整理:**

-   `Reason`（`type Reason string`）: 公開の文字列型であり他パッケージから任意の値を作れるが、`Decision.Skip()` は `d.Reason == ReasonSkipped` のみで `true` を返す閉じた判定とする。ゼロ値（`ReasonUnspecified`）および未定義の値はすべて `Skip() == false`（安全側）になるため、リーフパッケージ化は不要であり、ゼロ値・未知値の安全側動作をユニットテストで確かめる。
-   `Decision`: 公開構造体だが、スキップ可否は `Skip()` が `Reason` のみから導出するため、`(skip=true, 非 ok 理由)` の不整合状態は表現できない（設計書 §3.2・§4.1）。
-   `Evaluator`: 全フィールド非公開とし、本番構築は `New(environ)`、テスト構築は非公開 `newWithRunners` のみとする。ゼロ値 `&Evaluator{}` が直接呼ばれてもパニックせず安全側（`Skip() == false`）を返すよう、`Evaluate` の冒頭でランナーの `nil` を検査する。

**外部ツール・環境前提の検証結果（`d45909cf`）:**

-   `go help run` および `go.mod:3`（`go 1.26.5`）・`.github/workflows/ci.yml:27-30`（`actions/setup-go@v6` + `go-version-file: go.mod`）・`Makefile:150`（`GOTOOLCHAIN=local`）より、セットアップ済み Go 上で `GOTOOLCHAIN=local go run ./cmd/ciskip` がツールチェーンの再取得なしに動く。
-   `extension/scripts/has-extension-changes.sh:1-62` は `bash` 組込みのみで構成され、ローカル（GNU bash 5.3.15）および `ubuntu-latest` の標準 `bash` で追加依存なしに動作する。
-   `gh api --hostname github.com -i` の出力形式（LF ステータス行＋CRLF ヘッダ）は既存の `internal/numreserve/reserve.go:199-242` およびタスク 0100 の実地記録（`docs/tasks/0100_number_reservation/03_implementation_plan.md:602-615`）で検証済みである。`GITHUB_TOKEN` + `checks: read` で `::notice` を Checks API から読み戻せる点は未実測であり、フェーズ 3（AC-20）で実機検証する（設計書 §5.3）。

### 1.5. 申し送りへの対応

本タスクに `implementation_handoff.md` は存在しない。`design_handoff.md` の H-01〜H-04 は、いずれも `02_architecture.md` の付録 A に設計上の扱いが記録済みである。

---

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: `internal/ciskip` コア実装とユニットテスト

設計書 §8「フェーズ 1」に対応する。`internal/ciskip` の公開 API・ローカル検証・GitHub API 検証・全ユニット／結合テストと、フェーズ 1 のグリーンゲート成立および規約整合に必要な文書更新を行う。

**ステップ 1-1: 公開型・上限・コンテキスト入力検証・環境分離を実装する**

-   対象ファイル：`internal/ciskip/ciskip.go`、`internal/ciskip/ciskip_test.go`、`internal/ciskip/test_helpers.go`（`//go:build test`）
-   [ ] 設計書 §3.2・§3.4 の公開型（`Reason` 全定数・`Input`・`Decision`・`Decision.Skip`・`Evaluator`・`New`・`Evaluator.Evaluate`）と非公開の `evalLimits`・`newWithRunners`、および `Evaluate` の全体タイムアウト（`RunTimeout: 45s`）と `nil` ランナーの安全側返却を実装する。
-   [ ] 設計書 §3.1・§3.3.1 のコンテキスト入力検証（リポジトリ許可リストと同一性、`BaseRef == "main"`、`HeadSHA` 40 桁小文字 16 進、`PRNumber`・`RunAttempt` の正の 10 進整数と `RunAttempt > 1` 拒否、`WasmDepDirsOK` 検証）を実装する。
-   [ ] 設計書 §3.3.3・§3.4 の `git`・`script`・`gh` 各ランナーの `cmdrunner.Options` と環境変数サニタイズを純粋な関数として実装する。
-   [ ] パッケージ doc コメントに外部出力・パス・API ヘッダを解析する範囲と境界を明記する（設計書 §2.2）。
-   [ ] `ciskip_test.go` に、コンテキスト入力検証の全正常／異常系（AC-09・AC-10・AC-12）、`Reason` ゼロ値・未知値とゼロ値 `Evaluator` の安全側動作、および各ランナーの環境分離・`Options` 上限を検証するテストを書く。
-   完了の基準：`go test -tags test -race ./internal/ciskip` が通る。テストが対応する検証を壊したときに失敗することを確かめ、コミットメッセージに記録する。

**ステップ 1-2: `git` 検証と親コミット差分分類を実装する**

-   対象ファイル：`internal/ciskip/git.go`、`internal/ciskip/git_test.go`
-   [ ] 設計書 §3.3.1 の合成マージコミット検証（`git rev-list --parents -n 1 HEAD` で 2 親かつ `HEAD^2 == HeadSHA` を確認して初めて `NoticeBaseSHA = HEAD^1` を設定）、`origin/main` との一致確認、および PR head コミットの単一親確認を実装する。
-   [ ] 設計書 §3.3.2 の `depDirsFile` の独立検証（64 KiB 上限・非空・全非空行が `/` 終端）、`git -c core.quotePath=false diff -z --no-renames --name-only <parentSHA> <headSHA> --` の解析（0 バイト空コミット許容・末尾 NUL 検証）、非コード分類、および既存 `extension/scripts/has-extension-changes.sh` の呼び出しを実装する。
-   [ ] `git_test.go` に、`cmdrunnertestutil.Fake` による合成マージコミット・親数（0 個／1 個／2 個以上）・空コミット・非コード／コード／拡張対象ファイル・`depDirsFile` 不正・スクリプト異常系のユニットテストを書く（コード変更時は `gh` を 1 回も呼ばないことも `Fake` で検証する）。
-   [ ] `git_test.go` に、`t.TempDir()` の実 git リポジトリと実 `extension/scripts/has-extension-changes.sh` を使う結合テスト（合成マージコミット、リネーム両側、非 ASCII・タブ・改行パス、64 KiB 超標準入力＋早期終了でデッドロックしないこと、設計書 §7.2）を書く。
-   完了の基準：`go test -tags test -race ./internal/ciskip` が通り、対応する判定を壊したときに各テストが失敗することを確かめ、コミットメッセージに記録する。

**ステップ 1-3: GitHub API 検証とセキュリティテストを実装する**

-   対象ファイル：`internal/ciskip/github.go`、`internal/ciskip/github_test.go`
-   [ ] 設計書 §3.3.3・§3.4 の `gh api --hostname github.com -i` 呼び出し、LF ステータス行＋CRLF ヘッダ分離、非ゼロ終了時の `HTTPStatus` 抽出と本文・stderr 破棄、RFC 8288 `Link` ヘッダ解析と固定エンドポイントでのページネーション上限、`strictjson`（`ParseObject`・`ParseArray`・`CollectExactCase`・`AsNonNegativeInt64`）による 3 エンドポイント（Runs・Jobs・Annotations）の厳密検証を実装する。
-   [ ] `github_test.go` に、設計書 §7.1・§7.3 の全ケース（正常通過、`doc-guard`／`secret-scan` 失敗時の通過、連続スキップ連鎖、`in_progress`／`queued`、`cancelled` 等、別ワークフロー／別 PR／`head_repository: null`／`ReasonAmbiguousPRLink`、必須 4 ジョブの欠損・重複・`in_progress` かつ `conclusion: success`、アノテーション 0 件・重複・不正 SHA・`main` 更新、厳密 JSON 違反、ページ上限・`ErrOutputTooLarge`・タイムアウト、偽 `Link` URL 無視、非 200 本文／stderr の機密非露出）を書く。
-   完了の基準：`go test -tags test -race ./internal/ciskip` が通り、対応する判定を壊したときに各テストが失敗することを確かめ、コミットメッセージに記録する。

**ステップ 1-4: フェーズ 1 のパッケージ参照・`CLAUDE.md` 例外を更新する**

-   対象ファイル：`docs/dev/conventions/package_reference.md`、`CLAUDE.md`
-   [ ] `package_reference.md` の「Development Tools」表へ `internal/ciskip` の行を追加し、`internal/strictjson` 行の `Shared by ...` 一覧へ `internal/ciskip` を追加する（設計書 §2.2・§3.5）。
-   [ ] `CLAUDE.md` の「Declare, don't infer」へ `internal/ciskip` の例外（対象範囲・理由・境界）を追加する（設計書 §2.2・§3.5）。
-   完了の基準：`make test`（`TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`TestFakesCarryBuildTag` を含む）・`make lint`（`-tags test,integration` で `test_helpers.go` もコンパイル）・`make deadcode` がすべて通る。

### フェーズ 2: `cmd/ciskip`・`.github/workflows/ci.yml` 統合・ピン留めテスト・規約更新

設計書 §8「フェーズ 2」に対応する。CLI エントリポイント、ワークフロー統合、常設ピン留めテスト、AC-16 検証スクリプト、および残りの規約・ガード更新を行う。

**ステップ 2-1: `cmd/ciskip` CLI とユニットテストを実装する**

-   対象ファイル：`cmd/ciskip/main.go`、`cmd/ciskip/main_test.go`、`internal/config/envaccess_test.go`
-   [ ] 設計書 §3.1・§4.2 のとおり、`signalrun.Run`・フラグ解析・`os.Environ()` 1 回読み取り・`ciskip.New` 呼び出し・`NoticeBaseSHA` の `::notice title=ci-base-sha::<sha>` 出力・1 行診断ログ出力・`--result-file` 書き込み・終了コード制御（`0` / `2`）を実装する。テスト差し替え用に非公開 `evaluator` インターフェースを置く。
-   [ ] `internal/config/envaccess_test.go` の `allowedEnvRefs` に `"cmd/ciskip/main.go": {"call:os.Environ": 1}` を追加する（設計書 §2.2）。
-   [ ] `cmd/ciskip/main_test.go` に、フラグ構文エラー・`--result-file` 欠落／書き込み失敗（終了 `2`・未検証フラグ値非出力）、空コンテキスト値（終了 `0`）、`skip=true\n`／`skip=false\n` の結果ファイル書き込み、`::notice` の出力有無、および 1 行診断ログ形式（`run=`・`status=`・機密非露出）を検証するテストを書く（AC-02・AC-12・AC-13）。
-   完了の基準：`go test -tags test -race ./cmd/ciskip ./internal/config` が通り、対応する振る舞いを壊したときにテストが失敗することを確かめ、コミットメッセージに記録する。

**ステップ 2-2: `.github/workflows/ci.yml` の `check-changes` ジョブを更新する**

-   対象ファイル：`.github/workflows/ci.yml`
-   [ ] 設計書 §3.3.4 のとおり、`check-changes` ジョブ内のみを更新する（ワークフロー全体の `permissions` および他の 5 ジョブには一切触れない、AC-15・AC-16）。
-   [ ] `check-changes` にジョブ限定 `permissions`（`contents: read`・`actions: read`・`checks: read`）と `timeout-minutes: 10` を追加し、`id: normal`（既存の通常分類と `wasm_dep_dirs_ok` 出力、`GH_TOKEN` なし）と `id: check`（`env:` 経由の入力渡し、`GOTOOLCHAIN: local`、`mktemp`・`go run ./cmd/ciskip`・`cmp -s` による最終出力と安全側フォールバック）を構成する。
-   完了の基準：`make ext-test` で既存の `extension/test/repository.test.ts`（`:143-251`）が無変更のまま通る。

**ステップ 2-3: 常設ピン留めテストと AC-16 検証スクリプトを追加する**

-   対象ファイル：`internal/ciskip/workflow_pin_test.go`、`docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh`
-   [ ] `workflow_pin_test.go` に、設計書 §7.1 の常設ピン留めテスト（`ci.yml` の非コード除外正規表現と `internal/ciskip` の一致、4 ジョブ表示名の一致、ワークフロー全体およびジョブ別 `permissions` と `: write` 不在（AC-15）、`check-changes` の `run:` 本文に `${{` が直接埋め込まれていないこと、`steps.normal.outputs.*` のフォールバック伝播、`timeout-minutes`・`GOTOOLCHAIN` 設定、出力名と `skip=true` 時の両出力 `false` 設定）を書く。
-   [ ] `docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh` を作成し、merge-base と比較対象コミットの `.github/workflows/ci.yml` について `  check-changes:` より前と `  test:` 以降末尾までがそれぞれバイト一致することを `cmp` で検証する（§1.3 の決定 2）。各チェックが少なくとも 1 つの意図的な破損で失敗することを確かめ、PR ブランチ上で実行した結果と両コミット ID を §7 の AC-16 行へ記録する。
-   完了の基準：`go test -tags test -race ./internal/ciskip` と `_checks/check-ac16-ci-diff.sh` が通る。

**ステップ 2-4: 残りのリポジトリ規約文書を更新する**

-   対象ファイル：`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/dev_tools.md`
-   [ ] `package_reference.md` の「Kinds of package」の `Development tools` 定義へ CI 専用ツールを含める記述を加え、「Development Tools」表へ `cmd/ciskip` の行を追加する（設計書 §2.2・§3.5）。
-   [ ] `dev_tools.md` の「Kinds of tools」と「Rule 5」（`Per-run tools` 付近）へ CI 専用ツール（`ciskip`）の区分と Rule 5・プロトコル番号の適用除外を追記する（設計書 §2.2・§3.5）。
-   完了の基準：`make test`・`make lint`・`make deadcode`・`make ext-test` がすべて通る。

### フェーズ 3: ADR の整合と GitHub Actions 実機確認

設計書 §8「フェーズ 3」に対応する。PR-2 の GitHub Actions 実行結果を実機で確認・記録し、ADR-0054・ADR-0014 から本タスクの未実装を示す食い違い記述を除去する。

**ステップ 3-1: PR-2 の GitHub Actions 実機確認結果を検証・記録する**

-   対象ファイル：`docs/tasks/0119_ci_skip_doc_only_commit/03_implementation_plan.md`
-   [ ] PR-2 マージ後（`自動送信: hold` により利用者が確認した後に開始）、PR-2 の GitHub Actions 実行履歴から AC-20 の 2 ケースを確認する：
    1.  コード変更コミットで `check-changes` が `::notice title=ci-base-sha::<sha>` を出力し、`test`・`lint`・`extension`・`doc-guard`・`secret-scan` が通常実行されて成功したこと。
    2.  直後の非コード・非拡張のみのコミット（PR-2 の `/runpr` による `PRE_MERGE` コミット）で、`check-changes` が親 run の `ci-base-sha` アノテーションを読み戻して `has-code-changes=false`・`has-extension-changes=false` を出力し、`test`・`lint`・`extension` が `skipped`、`doc-guard`・`secret-scan` が実行・成功したこと。
-   [ ] 確認日・PR 番号・2 ケースそれぞれのワークフロー実行 URL・全 6 ジョブの結果を、本計画書 §7「GitHub Actions 実機確認の記録（AC-20）」へ記録する（AC-20）。
-   [ ] 万一 PR-2 で `main` 更新などによりケース 2 がスキップされなかった場合、または `::notice` 読み戻しに設計想定と異なる挙動が見つかった場合は、ADR の食い違い除去へ進まずに停止し、原因と対処方針を利用者と決定する（設計書 §5.3）。
-   完了の基準：§7「GitHub Actions 実機確認の記録（AC-20）」に 2 ケースの実行 URL と全 6 ジョブ結果が記録されている。

**ステップ 3-2: ADR-0054 と ADR-0014 の食い違い記述を除去する**

-   対象ファイル：`docs/dev/adr/0054-ci-skip-doc-only-commit.md`、`docs/dev/adr/0014-deterministic-checks-in-go.md`
-   [ ] `docs/dev/adr/0054-ci-skip-doc-only-commit.md` の `## 決定` から未実装を示す「食い違い」の項目（`:40`）を除去し、本文が実装済みの振る舞いと一致していることを確かめる（AC-19）。また `## 結果` の未実測表記（`:64`）をステップ 3-1 の実機確認結果に合わせて更新する。
-   [ ] `docs/dev/adr/0014-deterministic-checks-in-go.md` の `## 決定` から、本タスクの CI 専用ツール（`ciskip`）に関する「食い違い」の項目（`:31`）のみを除去する（`:50` の `codexreview` に関する別タスクの食い違いは触れない）。
-   完了の基準：`make test`（`TestADRRecords` を含む）・`make lint`・`make deadcode` がすべて通る。

---

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

フェーズ 1〜3 を依存順に完了する。

| マイルストーン | 成果物 | 対応するフェーズ |
|---|---|---|
| M1：`internal/ciskip` コアとテスト | `internal/ciskip` 一式（公開型・git／差分分類・GitHub API 厳密検証・ユニット／結合テスト）、`package_reference.md`（`internal/ciskip` 行・`strictjson` 利用者）、`CLAUDE.md` 例外 | フェーズ 1 |
| M2：CLI・CI 統合・ピン留め・規約 | `cmd/ciskip`、`envaccess_test.go`、`.github/workflows/ci.yml`（`check-changes` のみ）、`workflow_pin_test.go`、`_checks/check-ac16-ci-diff.sh`、`package_reference.md`（`cmd/ciskip`）・`dev_tools.md` | フェーズ 2 |
| M3：実機確認記録と ADR 整合 | §7 の GitHub Actions 実機確認記録（AC-20）、ADR-0054・ADR-0014 の食い違い記述除去（AC-19） | フェーズ 3 |

---

## 4. テスト戦略 (Test Strategy)

設計書 §7 のテスト戦略に基づき、ユニットテスト・一時 git リポジトリ結合テスト・常設ピン留めテスト・`diff` スクリプト・GitHub Actions 実機確認の 5 層で検証する。

-   **ユニットテスト（`internal/ciskip`・`cmd/ciskip`）:** `internal/ciskip` は `newWithRunners` へ `internal/cmdrunner/testutil.Fake` を注入し、ネットワークや実リポジトリに依存せず決定論的に全条件・全境界・全異常系を検証する（設計書 §7.1）。`cmd/ciskip` は非公開 `evaluator` インターフェースを差し替えてフラグ・出力・終了コードを検証する（設計書 §7.2）。
-   **一時 git リポジトリ結合テスト（`internal/ciskip/git_test.go`）:** `t.TempDir()` 上の実 git リポジトリとリポジトリ内の実 `extension/scripts/has-extension-changes.sh` を使い、合成マージコミット・リネーム両側・非 ASCII／タブ／改行パス・64 KiB 超標準入力での早期終了非デッドロックを検証する（設計書 §7.2）。外部の長期状態には触れない。
-   **セキュリティテスト（`internal/ciskip/github_test.go`・`cmd/ciskip/main_test.go`）:** 非 200 応答本文・`ExitError.Stderr`・未検証フラグ値の機密非露出、偽 `Link` URL の無視、`ErrOutputTooLarge` とタイムアウトの安全側フォールバックを検証する（設計書 §7.3）。
-   **常設ピン留めテストと既存ガード:** `internal/ciskip/workflow_pin_test.go` で `.github/workflows/ci.yml` の非コード正規表現・ジョブ表示名・権限・インジェクション防止・フォールバック伝播をピン留めする。既存テスト `extension/test/repository.test.ts`（`make ext-test`）・`internal/config/envaccess_test.go`・`internal/pipeline/pipeline_test.go`・`internal/pipeline/strictjson_users_test.go`・`cmd/yt2column/adr_test.go` をすべて無変更（`envaccess_test.go` の許可 1 行追加を除く）で通す。
-   **`diff` 検証（AC-16）:** `_checks/check-ac16-ci-diff.sh` で `.github/workflows/ci.yml` の `check-changes` 外が無変更であることを機械検証し、実行結果と両コミット ID を §7 へ記録する。
-   **実機確認（AC-20）:** PR-2 の GitHub Actions 実行（コード変更コミットと `/runpr` の `PRE_MERGE` コミット）で通常実行・`::notice` 読み戻し・重い 3 ジョブのスキップ・常時実行 2 ジョブの継続成功を確認し、§7 へ記録する。
-   **テスト補助ファイルの配置:** `internal/cmdrunner/testutil` を再利用し、新規の `testutil/` は作らない。`internal/ciskip` 内で複数テストファイルが共有する補助は `internal/ciskip/test_helpers.go`（先頭行 `//go:build test`）へ置く（`docs/dev/conventions/test_organization.md` Classification B）。
-   **壊して失敗を確かめる義務:** 新設するすべてのユニットテスト・結合テスト・ピン留めテスト・`_checks/` スクリプトについて、対応する判定や条件を壊したときに失敗することを確認し、コミットメッセージに記録する。

---

## 5. リスク管理 (Risk Management)

技術的リスクと日程・運用上のリスクを以下に整理する。

| # | リスク | 対策 |
|---|---|---|
| 1 | `GITHUB_TOKEN` + `checks: read` で `::notice title=ci-base-sha::<sha>` を Checks API から読み戻せない（未実測、設計書 §5.3） | 読み戻せない場合は `ReasonParentBaseMissing` で安全側（通常分類）へ倒れるため誤スキップは起きない。PR-2 の実機確認で検証し、問題があれば PR-3 で ADR 食い違いを除去する前に停止して方式を見直す（ステップ 3-1） |
| 2 | PR-2 のレビュー中に別 PR が `main` へマージされ、PR-2 の `PRE_MERGE` コミットが `ReasonMainAdvanced` で通常実行になる | PR-3 を `自動送信: hold` とし、利用者が PR-2 の実行ログを確認する。`main_advanced` になった場合は、スキップが効く状態での追加確認手順を PR-3 開始前に利用者と確認する |
| 3 | `ci.yml` の `id: normal` 書き換えで `extension/test/repository.test.ts:232-235` の正規表現ピンが壊れる | `wasm_dep_dirs_ok=false` を `if` の前に初期化し、`else` 直後の構造を変えない（§1.4）。ステップ 2-2 の完了基準で `make ext-test` を実行して確認する |
| 4 | `os.ReadFile(depDirsFile)` や `os.WriteFile(resultFile, ...)` が `golangci-lint` の `gosec`（G304）に抵触する | 該当行に最小範囲の `//nolint:gosec` と根拠コメントを付け、ファイル・パッケージ全体への抑制は行わない。`make lint` で確認する |
| 5 | `make deadcode` でフェーズ 1 単体時に `New` や公開メソッドが未到達になる | フェーズ 1 のテスト（`ciskip_test.go`）から `New`・`Evaluate`・`Skip` を必ず呼び、`make deadcode` をフェーズ 1 の完了基準に含める |
| 6 | 将来 `.github/workflows/ci.yml` に新しい重いジョブを追加した際に `internal/ciskip` の必須ジョブ一覧と乖離する | `workflow_pin_test.go` で `ci.yml` のジョブ表示名・`needs: check-changes` の重いジョブと `internal/ciskip` の定数一覧を常設ピン留めする |

**日程上のリスク:** フェーズ 3 は PR-2 マージ後の GitHub Actions 実機結果を利用者が確認する待ち時間を含む。`PR-3` を `自動送信: hold` として確認完了後に着手する。

---

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（ステップ 1-1 / 1-2 / 1-3 / 1-4）
- [ ] フェーズ 2 完了（ステップ 2-1 / 2-2 / 2-3 / 2-4）
- [ ] フェーズ 3 完了（ステップ 3-1 / 3-2）

---

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

すべての受け入れ基準（AC-01〜AC-20）と検証手段・実装ステップの対応を示す。テスト関数名は実装時に確定し、変更があれば `/runplan` が本表を更新する。

| AC | 種類 | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test | `internal/ciskip/git_test.go::TestEvaluateParentCount`（親コミット 2 つ以上および 0 個で `ReasonNotSingleParent`・`Skip() == false`、一時 git リポジトリ結合テスト含む） | ステップ 1-2 |
| AC-02 | test | `internal/ciskip/ciskip_test.go::TestEvaluateSkipEligible`（単一親かつ非コード・非拡張ファイルのみ、および 0 バイト空コミットで `ReasonSkipped`・`Skip() == true`）、`cmd/ciskip/main_test.go::TestRunSkipAndFallback`（結果ファイルへ `skip=true\n` を書き終了コード 0） | ステップ 1-1・1-2・2-1 |
| AC-03 | test | `internal/ciskip/ciskip_test.go::TestEvaluateChangedFilesReject`・`internal/ciskip/git_test.go::TestEvaluateWithRealGitAndScript`（コードファイルで `ReasonCodeChanged` かつ `gh` 未呼出、`extension/`・`README.md`・`README.en.md`・`docs/dev/guides/development.md`・WASM 依存配下で `ReasonExtensionChanged`） | ステップ 1-2 |
| AC-04 | test | `internal/ciskip/git_test.go::TestEvaluateWithRealGitAndScript`（実 `git` と実 `has-extension-changes.sh` でコード／拡張対象から文書へのリネーム両側、非 ASCII・タブ・改行パスを検出し `Skip() == false`） | ステップ 1-2 |
| AC-05 | test | `internal/ciskip/github_test.go::TestVerifyParentCIValid`（親 run が `completed` かつ結論 `success`／`failure`、`Check changed files` が `success`、重い 3 ジョブが `success`／`skipped`、`doc-guard`・`secret-scan` 成否不問で `ReasonSkipped`） | ステップ 1-3 |
| AC-06 | test | `internal/ciskip/github_test.go::TestVerifyParentCIInvalidStates`（親 run 未完了、結論 `cancelled`／`timed_out`／`startup_failure` 等、4 ジョブの欠損・重複・未完了（`in_progress` かつ `conclusion: success` 含む）・不適合結論・`check-changes` 失敗で各非スキップ `Reason`） | ステップ 1-3 |
| AC-07 | test | `internal/ciskip/github_test.go::TestVerifyParentCIRunSelection`（親 run 不在、別ワークフロー・別イベント・別 PR・別リポジトリ・`head_repository: null`、同一リポジトリで `pull_requests` 空／複数による `ReasonAmbiguousPRLink`、古い run のみ成功で最新 run が失敗・キャンセル・未完了） | ステップ 1-3 |
| AC-08 | test | `internal/ciskip/github_test.go::TestVerifyParentCISkipChain`（親 run が `check-changes=success` かつ重い 3 ジョブすべて `skipped` の連鎖時に `ReasonSkipped` かつ `NoticeBaseSHA` を設定） | ステップ 1-3 |
| AC-09 | test | `internal/ciskip/ciskip_test.go::TestEvaluateContextValidation`（フォーク PR・空 `headRepo`・リポジトリ形式違反・非 `main` ベース・欠損で `ReasonForkOrRepoMismatch` または `ReasonNonMainBase`、`Skip() == false`） | ステップ 1-1 |
| AC-10 | test | `internal/ciskip/ciskip_test.go::TestEvaluateContextValidation`（`run_attempt >= 2` で `ReasonManualRerun`、`0`・負数・非整数・空白付きで `ReasonInvalidInput`、`Skip() == false`） | ステップ 1-1 |
| AC-11 | test | `internal/ciskip/ciskip_test.go::TestEvaluateBaseConsistency`・`internal/ciskip/github_test.go::TestVerifyParentBaseAnnotation`（`HEAD^1 != origin/main` で `ReasonMainMovedAtCheckout`、アノテーション 0 件／重複／不正 SHA／`mergeBaseSHA` 不一致で各非スキップ `Reason`） | ステップ 1-2・1-3 |
| AC-12 | test | `internal/ciskip/ciskip_test.go::TestEvaluateFailSafe`・`internal/ciskip/github_test.go::TestVerifyParentCIAPIFailures`・`cmd/ciskip/main_test.go::TestRunSkipAndFallback`（API 障害・非 200・タイムアウト・`ErrOutputTooLarge`・ページ上限・厳密 JSON 違反・`git` 失敗・`has-extension-changes.sh` 失敗・`wasm-dep-dirs-ok=false`・`depDirsFile` 空／不正で `Skip() == false`・終了コード 0・機密非露出） | ステップ 1-1・1-2・1-3・2-1 |
| AC-12 | static | `internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckChangesPin`（`id: check` のフォールバック伝播と異常終了捕捉）、`extension/test/repository.test.ts`（`make ext-test`：`wasm-dep-dirs.sh` 失敗時の `has_extension_changes=true` 維持） | ステップ 2-2・2-3 |
| AC-13 | test | `cmd/ciskip/main_test.go::TestRunDiagnosticLog`（スキップ時・通常分類フォールバック時の 1 行診断ログ出力、`Reason`・`run=`・`status=` 形式、生本文・stderr・トークン・未検証フラグ値の非出力） | ステップ 2-1 |
| AC-14 | static | `internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckChangesPin`（`skip=true\n` 一致時に両出力を `false` にし、`test`・`lint`・`extension`・`doc-guard`・`secret-scan` の条件を維持） | ステップ 2-2・2-3 |
| AC-14 | manual | 下の「GitHub Actions 実機確認の記録（AC-20）」（実機で重い 3 ジョブが `skipped`、常時実行 2 ジョブが実行・成功） | ステップ 3-1 |
| AC-15 | static | `internal/ciskip/workflow_pin_test.go::TestCIWorkflowPermissions`（ワークフロー全体は `contents: read` のまま、追加権限は `check-changes` 限定の `: read` のみで `: write` 不在） | ステップ 2-2・2-3 |
| AC-16 | diff | `docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh`（PR-2 のマージベースと比較対象コミットで `.github/workflows/ci.yml` の `check-changes` 外のバイト一致を検証。実行結果と両コミット ID は実装時にここへ記録する） | ステップ 2-2・2-3 |
| AC-17 | static | `internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckChangesPin`・`extension/test/repository.test.ts`（`make ext-test`：出力名 `has-code-changes`・`has-extension-changes`・非コード除外正規表現・通常分類とフォールバック維持） | ステップ 2-2・2-3 |
| AC-17 | test | `internal/ciskip/git_test.go::TestIsNonCodePath`（非コード分類規則が既存の `ci.yml` 規則と一致） | ステップ 1-2 |
| AC-18 | static | `cmd/yt2column/adr_test.go::TestADRRecords`（既存テスト。ADR-0054 の存在・状態 `有効`・`docs/dev/adr/README.md` の番号順索引行を検証） | 既存（`#1075`・`#1076` で作成済み） |
| AC-19 | manual | PR-3 のレビューで、`docs/dev/adr/0054-ci-skip-doc-only-commit.md`（および `0014-deterministic-checks-in-go.md` の本タスク分）の `## 決定` が実装済みの振る舞いと一致し、未実装の食い違い記述が残っていないことを読む | ステップ 3-2 |
| AC-20 | manual | 下の「GitHub Actions 実機確認の記録（AC-20）」へ確認日・PR 番号・2 ケースの実行 URL・全 6 ジョブ結果を記録し、PR-3 のレビューで確認する（スキップ判定ロジック自体は AC-02・AC-05・AC-14 の `test`／`static` で検証済み） | ステップ 3-1 |

### AC-16 検証スクリプト実行記録

PR-2 の実装時に `_checks/check-ac16-ci-diff.sh` を実行し、merge-base コミット ID・比較対象コミット ID・実行出力をここへ記録する。

### GitHub Actions 実機確認の記録（AC-20）

フェーズ 3（ステップ 3-1）で、PR-2 の実行結果から以下を記録する。

-   **確認日:** （未記録）
-   **対象 PR:** （未記録）
-   **ケース 1（コード変更コミットの通常実行）:**
    -   実行 URL: （未記録）
    -   `check-changes`（`ci-base-sha` notice 出力含む） / `test` / `lint` / `extension` / `doc-guard` / `secret-scan`: （未記録）
-   **ケース 2（直後の非コード・非拡張のみコミットのスキップ）:**
    -   実行 URL: （未記録）
    -   `check-changes` / `test` / `lint` / `extension` / `doc-guard` / `secret-scan`: （未記録）

---

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 の AC-01〜AC-20 の検証がすべて完了し、フェーズ 1〜3 のすべての実装タスクが完了している。
-   **品質:** 各 PR で `make test`・`make lint`・`make deadcode`（および `.github/workflows/ci.yml` を変更する PR では `make ext-test`）がすべて通る。新設したテストと `_checks/` スクリプトは、対象条件を壊したときに失敗することを確かめてコミットメッセージに記録している。
-   **セキュリティ:** 最小読み取り権限（AC-15）、フォーク PR 排除（AC-09）、環境変数分離と `run:` への `${{` 非直埋め込み、厳密 JSON・出力上限・タイムアウト、および機密・生応答本文の非出力がテストで確かめられている。
-   **文書:** `package_reference.md`・`dev_tools.md`・`CLAUDE.md`・ADR-0054・ADR-0014 が実装済みの振る舞いと一致し、本タスクの未実装を示す食い違い記述が残っていない。

---

## 9. 次のステップ (Next Steps)

-   全 PR のマージ後、`/runpr` が issue #1073 を閉じる。
-   将来 `.github/workflows/ci.yml` に `check-changes` の出力で制御される新しい重いジョブを追加するときは、同じ PR で `internal/ciskip` の必須ジョブ一覧と `workflow_pin_test.go` を更新する（[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)「結果」）。
