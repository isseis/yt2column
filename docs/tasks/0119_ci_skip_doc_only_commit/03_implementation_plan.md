# 実装計画書：直前コミットで CI 通過済みかつドキュメントのみの変更時に重いジョブをスキップする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-21 |
| Review date | 2026-11-21 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番であり、すべて設計書の付録 A に扱いが記録されている。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `d45909cf` で確かめた。設計書が確かめたコミット `b004f8c0` から `d45909cf` までの差分（`git diff --stat b004f8c0 d45909cf`）は、本タスクの文書・ADR-0014・ADR-0054・`docs/translation_glossary.md` の 5 ファイルのみであり、設計書が引用する既存コードと設定の行番号は今も一致する。

用語は、要件書 §6 および設計書冒頭と同じ意味で使う（重いジョブ・常時実行ジョブ・コードファイル／非コードファイル・拡張対象ファイル／非拡張ファイル・`PRE_MERGE` コミット・PR head コミット・親コミット・通常分類・安全側フォールバック・食い違いの記述・`ciskip`・合成マージコミット・記録済みベース SHA・判定理由）。

---

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおり、Go 製の CI 専用ツール `cmd/ciskip` + `internal/ciskip` を新設し、`.github/workflows/ci.yml` の `check-changes` ジョブへ統合する。あわせて、常設ピン留めテスト・最終出力ステップ（`id: check`、設計書 §3.3.4）の実行テスト・リポジトリ規約の更新・ADR-0054／ADR-0014 の整合・GitHub Actions 上での実機確認記録を行い、要件書の F-001〜F-006（AC-01〜AC-20）を満たす。

### 1.2. 実装の原則

-   設計書 §8 の 3 フェーズの順に進める。フェーズ 1（`internal/ciskip` コアとテスト）、フェーズ 2（`cmd/ciskip`・`ci.yml` 統合・ピン留めテスト・規約更新）、フェーズ 3（ADR 整合と GitHub Actions 実機確認）の順とし、脅威の対象と切り戻し単位ごとに 5 本の PR へ分割する（§1.3 の決定 1）。
-   既存部品を再利用し、外部依存を追加しない。外部コマンド実行は `internal/cmdrunner`（`GitHardening`・`GitEnv`・`MaxOutput`・`ProcessGroup`）、シグナル処理は `internal/signalrun`、厳密 JSON 解析は `internal/strictjson`、拡張対象分類は既存の `extension/scripts/has-extension-changes.sh` をそのまま使う。
-   すべての異常系を安全側（通常分類）へ倒す。`Evaluator.Evaluate` は運用上の失敗をすべて非スキップの `Reason` へ写像し、`cmd/ciskip` の診断ログ・標準エラー・結果ファイルへ機密情報や非 200 応答の生本文を出さない（設計書 §4.1・§4.2）。
-   各 PR がスタブなしでグリーンゲートと `make deadcode` を通す。パッケージを追加する各 PR で、`make test` のガードが要求する `package_reference.md`・`CLAUDE.md`・`envaccess_test.go` の該当分を同じ PR に含める。
-   テストは壊して失敗することを確かめる。新設するテストと `_checks/` のスクリプトは、対象となる判定や条件を壊したときに失敗することを実装時に確認し、コミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-21）

本節は、設計書が計画に委ねた PR 分割（実機確認手順を含む）と AC-16 検証方式の 2 点について、利用者との確認で決めた内容を記録する。

1.  PR は 5 本に分ける（フェーズ 1 を PR-1・PR-2、フェーズ 2 を PR-3・PR-4、フェーズ 3 を PR-5 とし、`PR-5` のみ `自動送信: hold`）。
    -   **決定:**
        -   **PR-1（ステップ 1-1〜1-3）:** `internal/ciskip` のローカル検証単位（公開値型・入力検証・環境分離・`git.go`・ユニット／実 `git` 結合テスト）と `package_reference.md`（`Development tools` 定義拡張・`internal/ciskip` 行）・`CLAUDE.md`（「Declare, don't infer」例外）。
        -   **PR-2（ステップ 1-4〜1-5）:** `internal/ciskip` の GitHub API 検証と `Evaluator` 統合単位（`github.go`・`Evaluator`/`New`/`Evaluate`・セキュリティテスト）と `package_reference.md`（`internal/strictjson` 利用者一覧・`internal/ciskip` 行更新）。
        -   **PR-3（ステップ 2-1〜2-2）:** `cmd/ciskip`・`envaccess_test.go`・`package_reference.md`（`cmd/ciskip` 行）・`dev_tools.md`。本番の CI ワークフローには触れない。
        -   **PR-4（ステップ 2-3〜2-4）:** `.github/workflows/ci.yml`（`check-changes` ジョブのみ）・`internal/ciskip/workflow_pin_test.go`（静的ピン＋`id: check` 実行テスト）・`_checks/check-ac16-ci-diff.sh`・`03_implementation_plan.md`（§7 の AC-16 実行記録）。
        -   **PR-5（ステップ 3-1〜3-2、`自動送信: hold`）:** PR-4 の GitHub Actions 実行結果（コード変更を含む push の通常実行と、続く非コード・非拡張のみコミットでのスキップ）を `03_implementation_plan.md` に記録し（AC-20）、ADR-0054・ADR-0014 の食い違いの記述を除去する（AC-19）。
    -   **理由:** ローカル git／差分分類（PR-1）とリモート GitHub API／厳密 JSON（PR-2）の 2 つのセキュリティ関門を脅威の対象ごとに分離してレビュー規模を抑える。さらに、本番の CI ワークフローに影響しない `cmd/ciskip`＋規約（PR-3）と `.github/workflows/ci.yml` 統合（PR-4）を分けることで、CI ワークフローの変更だけを単独の squash コミットとして安全にレビュー・切り戻しでき、PR-3 のモデル要件も `sonnet` に下げられる。PR-4 は `.github/workflows/ci.yml` と `workflow_pin_test.go` を含むため AC-20 の実機確認対象になり、PR-4 マージ後に利用者が 2 ケースの実行ログ（`::notice` 読み戻し・先頭コミット種別・レビュー中の `main` 更新有無）を確認してから PR-5 を始めるため `PR-5` を `hold` とする。
    -   **人の答え:** 初回確認で 3 PR 構成（PR-3 は `hold`）を選んだ後、PR 境界レビューの Major 指摘（フェーズ 1 の脅威の対象ごとの分離とフェーズ 2 の CI ワークフロー変更の分離）を受けて 5 PR 構成（PR-1/2/4 は `opus`、PR-3/5 は `sonnet`、PR-5 のみ `hold`）を採用した。
    -   **ADR:** なし（本タスクの PR 分割と検証手順であり、タスクの中で閉じる）。
2.  AC-16（`.github/workflows/ci.yml` の差分範囲）の `diff` 検証は `_checks/` のシェルスクリプトで行う。
    -   **決定:** `docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh` を PR-4 で作成し、merge-base と比較対象コミットの `.github/workflows/ci.yml` について、(a) `  check-changes:` と `  test:` が各 1 回・この順に存在すること、(b) `  check-changes:` より前と `  test:` 以降末尾までがそれぞれ `cmp` でバイト一致すること、(c) 比較対象コミットの `  check-changes:` と `  test:` の間に別ジョブやトップレベルキー（`^ {0,2}[^ #]`）が挿入されていないことを検証する。
    -   **理由:** `check-changes` の変更は 45〜60 行程度のワークフロー YAML 差分になる見込みであり、`requirements_process.md` §4 が「差分の読み取り」の前提とする小規模な文書変更に当たらない。短いシェルスクリプトの方が変更規模と釣り合い、計画書が長大な YAML 差分で肥大化するのを防げる。
    -   **人の答え:** 案 A（`_checks/` のシェルスクリプト）。
    -   **ADR:** なし（本タスクの 1 つの AC の検証方式であり、タスクの中で閉じる）。

### 1.4. 既存コード調査結果

コミット `d45909cf` で既存のコード・テスト・設定・外部ツール前提を調査した結果をまとめる。変更不要な領域は省略する。

| 領域 | 今あるもの（`d45909cf`） | 足りないもの・変えるもの |
|---|---|---|
| CI ワークフロー | `.github/workflows/ci.yml:7-8`（全体 `permissions: contents: read`）、`:12-73`（`check-changes`、`id: check` の 1 ステップで通常分類）、`:74-234`（他 5 ジョブ） | `check-changes` のみを変更する（PR-4）。ジョブ限定 `permissions`（`contents`・`actions`・`checks` 各 `read`）と `timeout-minutes: 10` を追加し、通常分類を `id: normal` へ移して後段に `id: check`（`cmd/ciskip` 呼び出し・最終出力・フォールバックログ）を置く（設計書 §3.3.4・§4.2） |
| 拡張 CI ピンテスト | `extension/test/repository.test.ts:35-55`（インデント基準のジョブ・ステップ切り出し）、`:143-251`（`steps.check.outputs.has-extension-changes`、`git diff -z`、`wasm-dep-dirs.sh`、`has-extension-changes.sh`、`else` 直後の `has_extension_changes=true` を検証） | 既存テストは変更せずに通す。`:232-235` の正規表現が `else` 直後に `has_extension_changes=true`（間に `echo` 1 行まで）を要求するため、`wasm_dep_dirs_ok=false` は `if` の前に初期化し、`then` 内で `true` にする。`id: check` 側の振る舞いとフォールバック伝播は `internal/ciskip/workflow_pin_test.go` の実行テストおよび静的ピンで検証する（PR-4） |
| 拡張変更分類スクリプト | `extension/scripts/wasm-dep-dirs.sh:1-39`（非空の `/` 終端行を出力）、`extension/scripts/has-extension-changes.sh:28-62`（依存ファイルが空でもエラーにせず `false` を返す） | スクリプト自体は変更しない。`internal/ciskip` 側で `depDirsFile` を有界読み取り（`os.Open`＋`io.LimitReader` で 64 KiB 上限）して非空・`/` 終端を独立検証したうえで、`has-extension-changes.sh` を `cmdrunner.InputRunner.RunInput`（`StderrIsFailure: true`）で再利用する |
| 外部コマンド実行・fake・一時 git 補助 | `internal/cmdrunner/runner.go:25-179`（`Runner`・`InputRunner`・`Options`・`ExitError`・`ErrOutputTooLarge`・`ErrUnexpectedStderr`・`NewOS`）、`internal/cmdrunner/git.go:19-45`（`GitHardening`・`GitEnv`）、`internal/cmdrunner/testutil/mocks.go:24-89`（`Fake`・`Step`）、`internal/mergepr/test_helpers_git.go:44-58,76`（`HOME`/`XDG_CONFIG_HOME` 分離とテストセットアップ用 `//nolint:gosec`） | `internal/ciskip` が `cmdrunner` を再利用する。`ProcessGroup: true` は標準入力がパイプバッファに収まる前提（`runner.go:170-173`）のため、`script` ランナーは `ProcessGroup: false` とする（設計書 §3.4 注 1）。一時 git リポジトリ結合テストでは既存の `cmdrunner.Options.Dir`（`runner.go:55`）を使い、`git` ランナーに `t.TempDir()`、`script` ランナーにリポジトリルートを指定する |
| 厳密 JSON・シグナル処理 | `internal/strictjson/strictjson.go:74,130,284,326`（`ParseObject`・`CollectExactCase`・`AsNonNegativeInt64`・`ParseArray`）、`internal/signalrun/signalrun.go:42`（`Run`） | `internal/ciskip`（PR-2）が `strictjson` を、`cmd/ciskip`（PR-3）が `signalrun.Run` を再利用する |
| `gh api -i` フレーミングの先行実装 | `internal/numreserve/tool.go:45-70,98-100`（`gh` 環境変数除外と `--hostname github.com -i`）、`internal/numreserve/reserve.go:181-242`（LF ステータス行＋CRLF ヘッダ分離） | 既存ツールは変更せず、`internal/ciskip/github.go` 内に閉じた実装を置く（設計書 §3.3.3） |
| 環境変数アクセスガード | `internal/config/envaccess_test.go:65-102`（`allowedEnvRefs` に各 `cmd/*/main.go` の `call:os.Environ: 1` を登録） | `"cmd/ciskip/main.go": {"call:os.Environ": 1}` を 1 行追加する（PR-3） |
| パッケージ一覧・`strictjson` 利用者ガード | `internal/pipeline/pipeline_test.go:630-669`（`TestPackageReferenceListsPackages`）、`internal/pipeline/strictjson_users_test.go:654-685`（`TestPackageReferenceStrictjsonUsers`）、`docs/dev/conventions/package_reference.md:52-58,109,112-129` | PR-1 で `Kinds of package` の `Development tools` 定義へ CI 専用ツールを追記し `internal/ciskip` 行を追加、PR-2 で `internal/strictjson` の利用者一覧へ `internal/ciskip` を追加して `internal/ciskip` 行を更新、PR-3 で `cmd/ciskip` 行を追加する |
| 開発ツール規約・`CLAUDE.md`・ADR | `docs/dev/conventions/dev_tools.md:10-24,140-142`、`CLAUDE.md:405-620`、`docs/dev/adr/0054-ci-skip-doc-only-commit.md:40,64`、`docs/dev/adr/0014-deterministic-checks-in-go.md:31`、`cmd/yt2column/adr_test.go:425`（`TestADRRecords`） | PR-1 で `CLAUDE.md`（「Declare, don't infer」例外）を更新し、PR-3 で `dev_tools.md`（CI 専用ツール区分と Rule 5 適用除外）を更新し、PR-5 で ADR-0054 と ADR-0014 の本タスクの食い違いの記述を除去する（ADR-0014:50 の `codexreview` の食い違いは別タスクなので触れない） |
| ビルドタグ・`golangci-lint` 除外 | `internal/pipeline/pipeline_test.go:554-602`（`TestFakesCarryBuildTag`：`testutil/` ファイル数は 24 固定、`test_helpers*.go` は先頭行 `//go:build test` 等を要求）、`Makefile:314`（`go vet -tags integration ./...` は `test` タグなしで実行）、`.golangci.yml:119-127`（`gosec` 等の除外は `_test.go` のみ） | `testutil/` は新設しない（`internal/cmdrunner/testutil` を再利用）。`internal/ciskip/test_helpers.go` および `internal/ciskip/*_test.go`・`cmd/ciskip/main_test.go` のすべてに先頭行 `//go:build test` を付ける。`test_helpers.go` で `exec.Command` 等の linter 対象構文を使う場合も `_test.go` 除外の対象外のため、最小範囲の `//nolint:gosec` とテスト専用根拠コメントを付ける |

**具象型の構築・変更経路の整理:**

-   `Reason`（`type Reason string`）: 公開の文字列型であり他パッケージから任意の値を作れるが、`Decision.Skip()` は `d.Reason == ReasonSkipped` のみで `true` を返す閉じた判定とする。ゼロ値（`ReasonUnspecified`）および未定義の値はすべて `Skip() == false`（安全側）になるため、リーフパッケージ化は不要であり、ゼロ値・未知値の安全側動作をユニットテストで確かめる。
-   `Decision`: 公開構造体だが、スキップ可否は `Skip()` が `Reason` のみから導出するため、`(skip=true, 非 ok 理由)` の不整合状態は表現できない（設計書 §3.2・§4.1）。
-   `Evaluator`: 全フィールド非公開とし、本番構築は `New(environ)`、テスト構築は非公開 `newWithRunners` のみとする（PR-2）。ゼロ値 `&Evaluator{}` が直接呼ばれてもパニックせず安全側（`Skip() == false`）を返すよう、`Evaluate` の冒頭でランナーの `nil` を検査する。

**外部ツール・環境前提の検証結果（`d45909cf`）:**

-   `go help run` および `go.mod:3`（`go 1.26.5`）・`.github/workflows/ci.yml:27-30`（`actions/setup-go@v6` + `go-version-file: go.mod`）・`Makefile:150`（`GOTOOLCHAIN=local`）より、セットアップ済み Go 上で `GOTOOLCHAIN=local go run ./cmd/ciskip` がツールチェーンの再取得なしに動く。
-   `extension/scripts/has-extension-changes.sh:1-62` は `bash` 組込みのみで構成され、ローカル（GNU bash 5.3.15）および `ubuntu-latest` の標準 `bash` で追加依存なしに動作する。
-   `gh api --hostname github.com -i` の出力形式（LF ステータス行＋CRLF ヘッダ）は既存の `internal/numreserve/reserve.go:199-242` およびタスク 0100 の実地記録（`docs/tasks/0100_number_reservation/03_implementation_plan.md:602-615`）で検証済みである。`GITHUB_TOKEN` + `checks: read` で `::notice` を Checks API から読み戻せる点は未実測であり、フェーズ 3（AC-20）で実機検証する（設計書 §5.3）。

### 1.5. 申し送りへの対応

本タスクに `implementation_handoff.md` は存在しない。`design_handoff.md` の H-01〜H-04 は、いずれも `02_architecture.md` の付録 A に設計上の扱いが記録済みである。

---

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: `internal/ciskip` コア実装とユニットテスト

設計書 §8「フェーズ 1」に対応する。ローカル検証・差分分類（PR-1）と GitHub API 検証・`Evaluator` 統合（PR-2）の 2 本の PR に分けて実装する。すべてのテストファイル（`*_test.go`）と `test_helpers.go` の先頭行には `//go:build test` を付与する。

**ステップ 1-1: 公開値型・上限・コンテキスト入力検証・環境分離を実装する**

-   対象ファイル：`internal/ciskip/ciskip.go`、`internal/ciskip/ciskip_test.go`（`//go:build test`）、`internal/ciskip/test_helpers.go`（`//go:build test`）
-   [x] 設計書 §3.2・§3.4 の公開値型（`Reason` 全定数・`Input`・`Decision`・`Decision.Skip`）、非公開の `evalLimits`、および設計書 §3.1・§3.3.1 のコンテキスト入力検証（リポジトリ許可リストと同一性、`BaseRef == "main"`、`HeadSHA` 40 桁小文字 16 進、`PRNumber`・`RunAttempt` の正の 10 進整数と `RunAttempt > 1` 拒否、`WasmDepDirsOK` 検証）を実装する（`Evaluator`・`New`・`Evaluate` はステップ 1-4 で追加する）。
-   [x] 設計書 §3.3.3・§3.4 の `git`・`script`・`gh` 各ランナーの `cmdrunner.Options` と環境変数サニタイズを純粋な関数として実装し、パッケージ doc コメントに外部出力・パス・API ヘッダの解析範囲と境界を明記する（設計書 §2.2）。
-   [x] `ciskip_test.go` でコンテキスト入力検証の正常／異常系（AC-09・AC-10・AC-12）、`Reason` ゼロ値・未知値の安全側動作、および各ランナーの環境分離と `Options` 上限を検証する。
-   完了の基準：`go test -tags test -race ./internal/ciskip` が通り、対応する検証を壊したときにテストが失敗することを確かめ、コミットメッセージに記録する。

**ステップ 1-2: `git` 検証と親コミット差分分類を実装する**

-   対象ファイル：`internal/ciskip/git.go`、`internal/ciskip/git_test.go`（`//go:build test`）、`internal/ciskip/test_helpers.go`（`//go:build test`）
-   [x] 設計書 §3.3.1・§3.3.2 の合成マージコミット検証（`HEAD^2 == HeadSHA` 照合と `HEAD^1` 取得）、`origin/main` 照合、単一親確認、`depDirsFile` の有界読み取り（`os.Open`＋`io.LimitReader` 64 KiB+1 バイト＋`defer Close`、必要な場合のみ該当行へ最小範囲の `//nolint:gosec` と根拠コメント）・非空／`/` 終端検証、`git -c core.quotePath=false diff -z --no-renames --name-only <parentSHA> <headSHA> --` 解析（0 バイト空コミット許容・末尾 NUL 欠落と空要素拒否）、非コード分類、および `has-extension-changes.sh` 呼び出しを実装する。
-   [x] `git_test.go` に、`cmdrunnertestutil.Fake` による合成マージコミット不整合・`HEAD^1 != origin/main`・親数（0 個／1 個／2 個以上）・`depDirsFile` 境界（64 KiB 受理・64 KiB+1 拒否・空行のみ）・空コミット時の `has-extension-changes.sh` 未呼出・分類器自身（`.github/workflows/ci.yml`・`cmd/ciskip/`・`internal/ciskip/`・`extension/scripts/*.sh`）を含むコード／拡張対象分類、および `git`・スクリプト異常系のユニットテストを書く。
-   [x] `git_test.go` に、`t.TempDir()` の実 `git` とリポジトリ直下の実 `extension/scripts/has-extension-changes.sh`（`cmdrunner.Options.Dir` でリポジトリルート指定）を使う結合テストを書く（対象は合成マージコミット、コードおよび拡張対象 `.md` のリネーム両側、ASCII エスケープ表記の非日本語 UTF-8 文字列を含む非 ASCII・タブ・改行パス、64 KiB を超える標準入力での早期終了時にデッドロックしないこと、設計書 §7.2）。補助関数は `test_helpers.go` に置き、`exec.Command` 等を使う場合はテストセットアップ専用である旨の根拠コメント付きで最小範囲の `//nolint:gosec` を付与し、`HOME`・`XDG_CONFIG_HOME` は `t.TempDir()` 配下へ分離する。
-   完了の基準：`go test -tags test -race ./internal/ciskip` が通り、対応する判定を壊したときに各テストが失敗することを確かめ、コミットメッセージに記録する。

**ステップ 1-3: PR-1 のパッケージ参照・`CLAUDE.md` 例外を更新する**

-   対象ファイル：`docs/dev/conventions/package_reference.md`、`CLAUDE.md`
-   [x] `package_reference.md` の「Kinds of package」の `Development tools` 定義へ CI 専用ツールを含める記述を加え、「Development Tools」表へ `internal/ciskip` の行を追加する（PR-1 時点では `internal/strictjson` をまだ import しないため、`internal/strictjson` 行の利用者一覧はステップ 1-5 で更新する。記載は `package_reference.md:14-22` の規約に従う）。
-   [x] `CLAUDE.md` の「Declare, don't infer」へ `internal/ciskip` の例外（対象範囲・理由・境界）を追加する（設計書 §2.2・§3.5）。
-   完了の基準：`make test`（`TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`TestFakesCarryBuildTag` を含む）・`make lint`（`go vet -tags integration ./...` および `-tags test,integration` の `golangci-lint` で `test_helpers.go` もコンパイル）・`make deadcode` がすべて通る。あわせて `CLAUDE.md` の例外記述と `internal/ciskip` のパッケージ doc／実装境界、`package_reference.md` の `internal/ciskip` 行と `ciskip.go` の公開型および記載規約（`package_reference.md:14-22`）の一致を実コードと突合して確認する。

### PR-1 作成ポイント: internal/ciskip local validation, git diff, and extension classification

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `feat(0119): CI スキップ判定のローカル検証と差分分類 internal/ciskip を追加する`

**レビュー観点**: 合成マージコミット HEAD^1/HEAD^2 検証・単一親確認・NUL 区切り差分解析がリネーム両側と特殊パスを見逃さないか / depDirsFile の 64 KiB 有界読み取りと非空検証および実 git・has-extension-changes.sh 結合テストが安全側に倒れているか / コンテキスト入力検証・環境変数分離・Reason ゼロ値の安全側動作に抜けがないか / package_reference.md と CLAUDE.md 例外が実装境界と一致しているか

**実装モデル要件**: opus

**判定理由**: ステップ 1-2 は CI スキップのローカル安全関門（合成マージコミット整合性・非コード／拡張差分分類・64 KiB 有界読み取り・実 git／bash 結合テスト）であり、複数レビュアーによる重点レビュー対象（セキュリティ関門および外部リソースを扱う処理）のトリガーに当たる。Sonnet で実装すると、HEAD^2 == HeadSHA 確認前にベース SHA を採用する、リネーム両側や非 ASCII／改行パスを取りこぼす、あるいは空の depDirsFile で has-extension-changes.sh が false を返す穴を見落としやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1081
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**ステップ 1-4: GitHub API 検証と `Evaluator` 統合・セキュリティテストを実装する**

-   対象ファイル：`internal/ciskip/github.go`、`internal/ciskip/ciskip.go`、`internal/ciskip/github_test.go`（`//go:build test`）、`internal/ciskip/ciskip_test.go`（`//go:build test`）
-   [x] 設計書 §3.3.3・§3.4 の `gh api --hostname github.com -i` 呼び出し、LF ステータス行＋CRLF ヘッダ分離、非ゼロ終了時の `HTTPStatus` 抽出と本文・stderr 破棄、RFC 8288 `Link` ヘッダ解析（大小文字不問・引用符なし `rel=next` 含む）と固定エンドポイントでのページネーション上限、および `strictjson`（`ParseObject`・`ParseArray`・`CollectExactCase`・`AsNonNegativeInt64`）による 3 エンドポイント（Runs・Jobs・Annotations）の厳密検証を実装する。
-   [x] `ciskip.go` へ設計書 §3.2・§3.4 の `Evaluator`・`New`・`Evaluator.Evaluate`・非公開 `newWithRunners` を追加し、`Evaluate` の全体タイムアウト（`RunTimeout: 45s`）・`nil` ランナーの安全側返却・`HEAD^2 == HeadSHA` 確認後のみ `NoticeBaseSHA = HEAD^1` を設定し以後の非スキップ判定でも保持する制御フロー（ローカル条件を先に検証し不通過時は `gh` を呼ばない）を配線する。
-   [x] `github_test.go` には、設計書 §7.1・§7.3 の API 検証とセキュリティテストの全ケース（正常通過・複数ページ取得・`doc-guard`／`secret-scan` 失敗時の通過・連続スキップ連鎖・`(run_started_at, created_at, id)` タイブレークと不正 RFC3339・`in_progress`／`queued`・`cancelled` 等・別ワークフロー／別 PR／`head_repository: null`／`ReasonAmbiguousPRLink`・4 ジョブ欠損／重複／`head_sha` 不一致／`in_progress` かつ `conclusion: success`・アノテーション 0 件／重複／不正 SHA／`main` 更新・厳密 JSON 違反・終了 0 かつ HTTP 非 200・非 `*cmdrunner.ExitError`・ページ上限・`ErrOutputTooLarge`・コマンドおよび `RunTimeout` タイムアウト・偽 `Link` URL 無視・非 200 本文／stderr の機密非露出）を書く。あわせて `ciskip_test.go` には、`NoticeBaseSHA` の設定／非設定不変条件（基本文脈失敗や `ReasonMergeCommitMismatch` では空、`ReasonCodeChanged`・`ReasonManualRerun`・`ReasonWasmDepDirsFailed` では設定）、コード／拡張変更時の `gh` 未呼出、空コミット時の先行検証と `has-extension-changes.sh` 未呼出、ゼロ値 `Evaluator` の安全側動作、および `New(environ)` 呼び出し（`make deadcode` 到達性）を書く。
-   完了の基準：`go test -tags test -race ./internal/ciskip` が通り、対応する判定を壊したときに各テストが失敗することを確かめ、コミットメッセージに記録する。

**ステップ 1-5: `package_reference.md` の `strictjson` 利用者一覧と `internal/ciskip` 行を更新する**

-   対象ファイル：`docs/dev/conventions/package_reference.md`
-   [x] `package_reference.md` の `internal/strictjson` 行の `Shared by ...` 一覧へ `internal/ciskip` を追加し、`internal/ciskip` 行へ `Evaluator`・`New`・`Evaluate` を追記する（設計書 §2.2・§3.5、`package_reference.md:14-22` の記載規約に従う）。
-   完了の基準：`make test`（`TestPackageReferenceStrictjsonUsers` を含む）・`make lint`・`make deadcode` がすべて通り、`package_reference.md` の `internal/ciskip` 行が `ciskip.go` の公開 API と一致することを突合して確認する。

### PR-2 作成ポイント: internal/ciskip GitHub API verification and Evaluator orchestration

**対象ステップ**: 1-4 / 1-5

**推奨タイトル**: `feat(0119): internal/ciskip に GitHub API 検証と Evaluator 統合を追加する`

**レビュー観点**: GitHub API 3 エンドポイントの strictjson 解析・消費メンバ重複キー拒否・ページ上限・曖昧 PR 紐付け拒否（ReasonAmbiguousPRLink）が漏れなく実装されているか / 親 run 全件完了確認・4 ジョブの status==completed と欠損重複拒否・ci-base-sha アノテーション照合が設計どおりか / 非 200 応答本文・stderr・偽 Link URL に対する機密非露出と安全側フォールバックがテストで確かめられているか / NoticeBaseSHA の設定不変条件と package_reference.md の strictjson 利用者一覧が一致しているか

**実装モデル要件**: opus

**判定理由**: ステップ 1-4 は GitHub API 信頼境界（LF／CRLF フレーミング・RFC 8288 Link 解析・3 エンドポイントの厳密 JSON・曖昧 PR 紐付けと未完了ジョブの拒否・機密非露出）を扱うセキュリティ関門であり、複数レビュアーによる重点レビュー対象（セキュリティ関門および外部リソースを扱う処理）のトリガーに当たる。Sonnet で実装すると、曖昧な pull_requests 配列や status=in_progress かつ conclusion=success のジョブを黙って通過させる、あるいは非 200 応答の生本文や stderr を出力へ漏らしやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1086
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `cmd/ciskip`・`.github/workflows/ci.yml` 統合・ピン留めテスト・規約更新

設計書 §8「フェーズ 2」に対応する。本番の CI ワークフローへ影響しない CLI と規約更新（PR-3）と、`.github/workflows/ci.yml` 統合・ピン留めテスト・AC-16 検証スクリプト（PR-4）の 2 本の PR に分けて実装する。

**ステップ 2-1: `cmd/ciskip` CLI とユニットテストを実装する**

-   対象ファイル：`cmd/ciskip/main.go`、`cmd/ciskip/main_test.go`（`//go:build test`）、`internal/config/envaccess_test.go`
-   [x] 設計書 §3.1・§4.2 のとおり、`signalrun.Run`・フラグ解析（余分な位置引数 `fs.NArg() > 0` も終了コード `2` で拒否）・`os.Environ()` 1 回読み取り・`ciskip.New` 呼び出し・`NoticeBaseSHA` の `::notice title=ci-base-sha::<sha>` 出力・1 行診断ログ出力・`--result-file` 書き込み（`gosec` 対象になる場合のみ該当行へ最小範囲の `//nolint:gosec` と根拠コメント）・終了コード制御（`0` / `2`）を実装する。テスト差し替え用に非公開 `evaluator` インターフェースを置く。
-   [x] `internal/config/envaccess_test.go` の `allowedEnvRefs` に `"cmd/ciskip/main.go": {"call:os.Environ": 1}` を追加する（設計書 §2.2）。
-   [x] `cmd/ciskip/main_test.go` に、フラグ構文エラー・余分な位置引数・`--result-file` 欠落／書き込み失敗（終了 `2`・未検証引数／フラグ値非出力）、空コンテキスト値（終了 `0`）、`skip=true\n`／`skip=false\n` の結果ファイル書き込み、`::notice` の出力有無、および 1 行診断ログ形式（`run=`・`status=`・機密非露出）を検証するテストを書く（AC-02・AC-12・AC-13）。
-   完了の基準：`go test -tags test -race ./cmd/ciskip ./internal/config` が通り、対応する振る舞いを壊したときにテストが失敗することを確かめ、コミットメッセージに記録する。

**ステップ 2-2: `cmd/ciskip` のパッケージ参照と開発ツール規約を更新する**

-   対象ファイル：`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/dev_tools.md`
-   [x] `package_reference.md` の「Development Tools」表へ `cmd/ciskip` の行を追加する（設計書 §2.2・§3.5、`package_reference.md:14-22` の記載規約に従う）。
-   [x] `dev_tools.md` の「Kinds of tools」と「Rule 5」（`Per-run tools` 付近）へ CI 専用ツール（`ciskip`）の区分と Rule 5・プロトコル番号の適用除外を追記する（設計書 §2.2・§3.5）。
-   完了の基準：`make test`・`make lint`・`make deadcode` がすべて通る。あわせて `dev_tools.md` の追記内容と `ADR-0014:30`・設計書 §1.2／§2.2、`package_reference.md` の `cmd/ciskip` 行と `cmd/ciskip/main.go` および記載規約（`package_reference.md:14-22`）の一致を突合して確認する。

### PR-3 作成ポイント: cmd/ciskip CLI and developer-tool conventions

**対象ステップ**: 2-1 / 2-2

**推奨タイトル**: `feat(0119): CI スキップ判定 CLI cmd/ciskip と開発ツール規約を追加する`

**レビュー観点**: cmd/ciskip の終了コード（0 / 2）・余分な位置引数拒否・固定 stderr（未検証引数と機密の非出力）・signalrun.Run が設計どおりか / 結果ファイル（skip=true\n / skip=false\n）・::notice 出力・1 行診断ログ（run= / status=）がテストで確かめられているか / internal/config/envaccess_test.go の os.Environ 許可が 1 か所のみに限定されているか / package_reference.md と dev_tools.md の CI 専用ツール区分・Rule 5 除外が ADR-0014 および実装と一致しているか

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（判定ロジックを実装済みの internal/ciskip を呼び出すだけの薄い CLI ラッパーと規約文書の更新のみで、本番の CI ワークフローの変更や未確定の実装方針を含まない）。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1089
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**ステップ 2-3: `.github/workflows/ci.yml` の `check-changes` ジョブを更新する**

-   対象ファイル：`.github/workflows/ci.yml`
-   [x] 設計書 §3.3.4・§4.2 のとおり、`check-changes` ジョブ内のみを更新する（ワークフロー全体の `permissions` および他の 5 ジョブには一切触れない、AC-15・AC-16）。
-   [x] `check-changes` にジョブ限定 `permissions`（`contents: read`・`actions: read`・`checks: read`）と `timeout-minutes: 10` を追加し、`id: normal`（既存の通常分類・`wasm_dep_dirs_ok` 出力、`GH_TOKEN` なし）と `id: check`（`env:` 経由の入力渡し、`GOTOOLCHAIN: local`、`mktemp`・`go run ./cmd/ciskip`・`cmp -s` による最終出力・安全側フォールバックおよび終了コード番号付きフォールバックログ、AC-12・AC-13・AC-14・AC-17）を構成する。
-   完了の基準：`make ext-test` で既存の `extension/test/repository.test.ts`（`:143-251`）が無変更のまま通る。

**ステップ 2-4: 常設ピン留めテスト・`id: check` 実行テスト・AC-16 検証スクリプトを追加する**

-   対象ファイル：`internal/ciskip/workflow_pin_test.go`（`//go:build test`）、`docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh`、`docs/tasks/0119_ci_skip_doc_only_commit/03_implementation_plan.md`
-   [x] `workflow_pin_test.go` に、設計書 §7.1 に基づく次の 5 項目の常設ピン留めテストを書く。(1) `ci.yml` から抽出した非コード除外正規表現を `regexp.Compile` し、代表パス群で `internal/ciskip` の非コード分類関数と一致することを検証する。(2) `ci.yml` 全ジョブから `check-changes` と `needs: check-changes` を持つジョブの `name:` 集合を抽出し、`internal/ciskip` の必須ジョブ定数集合と要素・基数が一致することを検証する。(3) `doc-guard`・`secret-scan` に `needs:`/`if:` がないこと、および `test`・`lint`・`extension` の `if:` 条件と出力名を検証する（AC-14・AC-17）。(4) トップレベル `permissions` が `contents: read` のみ、`check-changes` が `{contents, actions, checks}: read` のみ、他ジョブに `permissions:` がなく、非コメント行に `: write`／`write-all`／`read-all` が不在であることを検証する（AC-15）。(5) `check-changes` の `run:` 本文に `${{` が直接埋め込まれていないこと、`timeout-minutes`・`GOTOOLCHAIN`・`steps.normal.outputs.*` の伝播、および `id: check` が渡す全フラグ名と `cmd/ciskip` の実フラグ集合の一致を検証する。
-   [x] `workflow_pin_test.go` に、`ci.yml` から `id: check` の `run:` 本文を抽出して `bash -eo pipefail` で実際に動かして確かめる実行テスト（`t.TempDir()` 上のフェイク `go`／`mktemp` とテスト用の `RUNNER_TEMP`・`GITHUB_OUTPUT` を注入）を書く。このテストで、両 `false` 時の `go` 未呼出、空文字／不正値の `true` 正規化、`skip=true\n` 時の `false`/`false` 出力と `stdout`（`::notice`・ログ）通過、`skip=false\n` 時の終了コード 0・通常分類出力維持、および不正内容／`go` 非ゼロ終了（1・2）／`mktemp` 失敗時の終了コード 0・通常分類出力維持・終了コード番号付きフォールバックログを検証する（AC-12・AC-13・AC-14・AC-17）。
-   [x] `docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh` を作成し、merge-base と比較対象コミットの `.github/workflows/ci.yml` について §1.3 決定 2 の 3 条件（アンカーの一意性と順序、`check-changes` 前後のバイト一致、中間への別ジョブ／トップレベルキー非挿入）を検証する。各チェックが少なくとも 1 つの意図的な破損で失敗することを確かめ、PR ブランチ上で実行した結果と両コミット ID を §7「AC-16 検証スクリプト実行記録」へ記録する。
-   完了の基準：`make test`・`make lint`・`make deadcode`・`make ext-test`・`_checks/check-ac16-ci-diff.sh` がすべて通り、`dev_tools.md` の記述が `.github/workflows/ci.yml` の `go run ./cmd/ciskip` 呼び出しと一致することを突合して確認する。

### PR-4 作成ポイント: CI workflow integration, pin/execution tests, and AC-16 diff script

**対象ステップ**: 2-3 / 2-4

**推奨タイトル**: `ci(0119): ci.yml の check-changes スキップ統合とピン留め・実行テストを追加する`

**レビュー観点**: ci.yml の変更が check-changes ジョブ限定・最小読み取り権限のみで、set -eo pipefail 下でも異常終了せず通常分類へフォールバックするか / workflow_pin_test.go の id: check 実行テストと重いジョブ集合・正規表現等価性・権限・フラグ一致ピンが網羅されているか / extension/test/repository.test.ts が無変更で通り、AC-16 検証スクリプトの実行記録が §7 に残っているか

**実装モデル要件**: opus

**判定理由**: ステップ 2-3・2-4 は .github/workflows/ci.yml の権限付与とスキップ分岐（複数レビュアーによる重点レビュー対象である CI ワークフロー変更およびセキュリティ関門のトリガー）および set -eo pipefail 下の実行テスト・常設ピン留めテストを扱う。Sonnet で実装すると、set -eo pipefail 下で mktemp や go run の失敗時に check-changes ジョブ自体を落とす、あるいは extension/test/repository.test.ts の else 直後ピンや最小権限の制約を壊しやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1091
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: ADR の整合と GitHub Actions 実機確認

設計書 §8「フェーズ 3」に対応する。PR-4 の GitHub Actions 実行結果を実機で確認・記録し、ADR-0054・ADR-0014 から本タスクの未実装を示す食い違いの記述を除去する。

**ステップ 3-1: PR-4 の GitHub Actions 実機確認結果を検証・記録する**

-   対象ファイル：`docs/tasks/0119_ci_skip_doc_only_commit/03_implementation_plan.md`
-   [ ] PR-4 マージ後（`自動送信: hold` により利用者が確認した後に開始）、PR-4 の GitHub Actions 実行履歴から AC-20 の 2 ケースを確認する：
    1.  コード変更を含む push（PR-4 上でコード変更コミットを先頭とする `/fixpr` 等の push、または PR-4 のコード変更に対する全 6 ジョブ通常実行の run）で、`check-changes` が `::notice title=ci-base-sha::<sha>` を出力し、`test`・`lint`・`extension`・`doc-guard`・`secret-scan` が通常実行されて成功したこと（なお `/runplan` は PR 作成前後に空の記録コミットや PR 作成直後の `AFTER_CREATE` コミットを積むため、対象 run の確認時に先頭コミット種別も併せて確認する）。
    2.  直後の非コード・非拡張のみのコミット（PR-4 の `/runpr` による `PRE_MERGE` コミット等）で、`check-changes` が親 run の `ci-base-sha` アノテーションを読み戻して `has-code-changes=false`・`has-extension-changes=false` を出力し、`test`・`lint`・`extension` が `skipped`、`doc-guard`・`secret-scan` が実行・成功したこと。
-   [ ] 確認日・PR 番号・2 ケースそれぞれのワークフロー実行 URL・全 6 ジョブの結果（およびケース 1 で先頭コミットがコード変更だったか記録コミットだったかの事実）を、本計画書 §7「GitHub Actions 実機確認の記録（AC-20）」へ記録する（AC-20）。
-   [ ] 万一ケース 1 が未確認の場合（PR-4 にコード変更先頭コミットの push が発生せず利用者が直接確認を求める場合を含む）、ケース 2 が `main` 更新などによりスキップされなかった場合、または `::notice` 読み戻しに設計想定と異なる挙動が見つかった場合は、ADR の食い違いの記述除去へ進まずに停止し、追加の確認手段または対処方針を利用者と決定する（設計書 §5.3）。
-   完了の基準：§7「GitHub Actions 実機確認の記録（AC-20）」に 2 ケースの実行 URL と全 6 ジョブ結果が記録されている。

**ステップ 3-2: ADR-0054 と ADR-0014 の食い違いの記述を除去する**

-   対象ファイル：`docs/dev/adr/0054-ci-skip-doc-only-commit.md`、`docs/dev/adr/0014-deterministic-checks-in-go.md`
-   [ ] `docs/dev/adr/0054-ci-skip-doc-only-commit.md` の `## 決定` から未実装を示す「食い違い」の項目（`:40`）を除去し、本文が実装済みの振る舞いと一致していることを確かめる（AC-19）。また `## 結果` の未実測表記（`:64`）をステップ 3-1 の実機確認結果に合わせて更新する。
-   [ ] `docs/dev/adr/0014-deterministic-checks-in-go.md` の `## 決定` から、本タスクの CI 専用ツール（`ciskip`）に関する「食い違い」の項目（`:31`）のみを除去する（`:50` の `codexreview` に関する別タスクの食い違いは触れない）。
-   完了の基準：`make test`（`TestADRRecords` を含む）・`make lint`・`make deadcode` がすべて通る。

### PR-5 作成ポイント: GitHub Actions verification record and ADR alignment

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `docs(0119): GitHub Actions 実機確認結果を記録し ADR-0054・ADR-0014 の食い違いの記述を除く`

**レビュー観点**: §7 に PR-4 の 2 ケース（コード変更を含む push の通常実行と直後の文書のみコミットのスキップ）の確認日・PR 番号・実行 URL・全 6 ジョブ結果が正確に記録されているか / ADR-0054 の食い違いの記述除去と ## 結果の未実測表記（:64）の更新が実機結果と一致し、ADR-0014 から本タスク分のみが除かれて別タスクの食い違いが残っているか / TestADRRecords を含むグリーンゲートが通るか

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（PR-4 マージ後に利用者が実機結果を確認したうえで記録を書き、ADR-0054・ADR-0014 の食い違いの記述を除去する文書更新のみの PR である）。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

フェーズ 1〜3 を依存順に完了する。

| マイルストーン | 成果物 | 対応する PR |
|---|---|---|
| M1：`internal/ciskip` ローカル検証・差分分類 | `internal/ciskip` ローカル側（公開値型・入力検証・環境分離・`git.go`・ユニット／実 `git` 結合テスト）、`package_reference.md`（`Development tools` 定義拡張・`internal/ciskip` 行）、`CLAUDE.md` 例外 | PR-1 |
| M2：`internal/ciskip` GitHub API・`Evaluator` 統合 | `internal/ciskip/github.go`・`Evaluator`/`New`/`Evaluate`・セキュリティテスト、`package_reference.md`（`strictjson` 利用者・`internal/ciskip` 行更新） | PR-2 |
| M3：`cmd/ciskip` CLI と開発ツール規約 | `cmd/ciskip`、`envaccess_test.go`、`package_reference.md`（`cmd/ciskip`）・`dev_tools.md` | PR-3 |
| M4：CI ワークフロー統合とピン留め・実行テスト | `.github/workflows/ci.yml`（`check-changes` のみ）、`workflow_pin_test.go`（静的ピン＋`id: check` 実行テスト）、`_checks/check-ac16-ci-diff.sh`、§7 の AC-16 実行記録 | PR-4 |
| M5：実機確認記録と ADR 整合 | §7 の GitHub Actions 実機確認記録（AC-20）、ADR-0054・ADR-0014 の食い違いの記述除去（AC-19） | PR-5 |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | `internal/ciskip` ローカル検証・差分分類とユニット／結合テスト、`package_reference.md`（`internal/ciskip`）・`CLAUDE.md` 例外 | opus | auto |
| PR-2 | 1-4 / 1-5 | `internal/ciskip` GitHub API 検証・`Evaluator` 統合・セキュリティテスト、`package_reference.md`（`strictjson` 利用者一覧） | opus | auto |
| PR-3 | 2-1 / 2-2 | `cmd/ciskip`、`envaccess_test.go`、`package_reference.md`（`cmd/ciskip`）・`dev_tools.md` | sonnet | auto |
| PR-4 | 2-3 / 2-4 | `.github/workflows/ci.yml` 統合、`workflow_pin_test.go`（静的ピン＋`id: check` 実行テスト）、`_checks/check-ac16-ci-diff.sh`、§7 の AC-16 実行記録 | opus | auto |
| PR-5 | 3-1 / 3-2 | PR-4 の GitHub Actions 実機確認結果の記録（AC-20）、ADR-0054・ADR-0014 の食い違いの記述除去（AC-19） | sonnet | hold |

PR 分割の理由は §1.3 の決定 1 にある。PR-4 のマージ後、利用者が PR-4 の GitHub Actions 実行ログで AC-20 の 2 ケースと `::notice` 読み戻しを確認してから PR-5 の `/runplan` を開始するため、PR-5 の `自動送信` を `hold` とする。

---

## 4. テスト戦略 (Test Strategy)

設計書 §7 のテスト戦略に基づき、ユニットテスト・一時 git リポジトリ結合テスト・`ci.yml` 実行テストおよび常設ピン留めテスト・`diff` スクリプト・GitHub Actions 実機確認の 5 層で検証する。

-   **ユニットテスト（`internal/ciskip`・`cmd/ciskip`）:** `internal/ciskip` は非公開のヘルパー／`newWithRunners` へ `internal/cmdrunner/testutil.Fake` を注入し、ネットワークや実リポジトリに依存せず決定論的に全条件・全境界・全異常系を検証する（設計書 §7.1）。パス分類の全境界は `TestIsNonCodePath` と `workflow_pin_test.go` の正規表現等価性テストに集約し、`TestEvaluateChangedFilesReject` は判定全体の制御フロー（コード変更時の早期終了と `script`/`gh` 未呼出、拡張変更時の `gh` 未呼出）に絞ってテスト表の重複を避ける。`cmd/ciskip` は非公開 `evaluator` インターフェースを差し替えてフラグ・位置引数拒否・出力・終了コードを検証する（設計書 §7.2）。
-   **一時 git リポジトリ結合テスト（`internal/ciskip/git_test.go`）:** `t.TempDir()` 上の実 `git`（`HOME`・`XDG_CONFIG_HOME` を `t.TempDir()` 配下へ分離し `~/.gitconfig` 干渉を防ぐ、`internal/mergepr/test_helpers_git.go:44-58` と同方式）とリポジトリ直下の実 `extension/scripts/has-extension-changes.sh`（`cmdrunner.Options.Dir` でリポジトリルート指定）を使い、合成マージコミット・コードおよび拡張対象 `.md` のリネーム両側・非 ASCII／タブ／改行パス・64 KiB を超える標準入力を渡した状態で早期終了してもデッドロックしないことを検証する（設計書 §7.2）。非 ASCII パスのテスト入力には日本語リテラルを避け ASCII エスケープ表記の非日本語 UTF-8 文字列を使い、`CLAUDE.md`「Go Idioms」を守る。外部の長期状態には触れない。
-   **セキュリティテスト（`internal/ciskip/github_test.go`・`cmd/ciskip/main_test.go`）:** 非 200 応答本文・`ExitError.Stderr`・未検証引数／フラグ値の機密非露出、偽 `Link` URL の無視、`ErrOutputTooLarge` とタイムアウトの安全側フォールバックを検証する（設計書 §7.3）。
-   **`ci.yml` 実行テスト・常設ピン留めテストと既存ガード:** `internal/ciskip/workflow_pin_test.go` で、`id: check` ステップの `run:` 本文を `bash -eo pipefail` で実際に動かして確かめる実行テストと、`.github/workflows/ci.yml` の非コード正規表現等価性・重いジョブ集合不変性・常時実行ジョブ条件・権限・インジェクション防止・フラグ集合一致の静的ピンを行う。既存テスト `extension/test/repository.test.ts`（`make ext-test`）・`internal/config/envaccess_test.go`・`internal/pipeline/pipeline_test.go`・`internal/pipeline/strictjson_users_test.go`・`cmd/yt2column/adr_test.go` をすべて無変更（`envaccess_test.go` の許可 1 行追加を除く）で通す。
-   **`diff` 検証（AC-16）:** `_checks/check-ac16-ci-diff.sh` で `.github/workflows/ci.yml` の `check-changes` 外が無変更かつ中間への別ジョブ／トップレベルキー非挿入であることを機械検証し、実行結果と両コミット ID を §7「AC-16 検証スクリプト実行記録」へ記録する。
-   **実機確認（AC-20）:** PR-4 の GitHub Actions 実行で通常実行・`::notice` 読み戻し・重い 3 ジョブのスキップ・常時実行 2 ジョブの継続成功を確認し、§7 へ記録する。
-   **テストファイルと補助ファイルのビルドタグ・配置:** `internal/cmdrunner/testutil` を再利用し、新規の `testutil/` は作らない。`internal/ciskip` 内で複数テストファイルが共有する補助は `internal/ciskip/test_helpers.go`（`docs/dev/conventions/test_organization.md` Classification B）へ置き、`test_helpers.go`・`internal/ciskip/*_test.go`・`cmd/ciskip/main_test.go` のすべてに先頭行 `//go:build test` を付与して `go vet -tags integration ./...` との整合を保つ。
-   **壊して失敗を確かめる義務:** 新設するすべてのユニットテスト・結合テスト・`ci.yml` 実行／ピン留めテスト・`_checks/` スクリプトについて、対応する判定や条件を壊したときに失敗することを確認し、コミットメッセージに記録する。

---

## 5. リスク管理 (Risk Management)

技術的リスクと日程・運用上のリスクを以下に整理する。

| # | リスク | 対策 |
|---|---|---|
| 1 | `GITHUB_TOKEN` + `checks: read` で `::notice title=ci-base-sha::<sha>` を Checks API から読み戻せない（未実測、設計書 §5.3） | 読み戻せない場合は `ReasonParentBaseMissing` で安全側（通常分類）へ倒れるため誤スキップは起きない。PR-4 の実機確認で検証し、問題があれば PR-5 で ADR の食い違いの記述を除去する前に停止して方式を見直す（ステップ 3-1） |
| 2 | PR-4 のレビュー中に別 PR が `main` へマージされて `PRE_MERGE` が `ReasonMainAdvanced` になる、または `/runplan` の記録コミットにより PR-4 でコード変更先頭コミットの push が走らない | PR-5 を `自動送信: hold` とし、利用者が PR-4 の実行ログを確認する。2 ケースのどちらかが未確認なら、ADR 更新へ進まずに追加の確認手順を利用者と決める（ステップ 3-1） |
| 3 | `ci.yml` の `id: normal` 書き換えで `extension/test/repository.test.ts:232-235` の正規表現ピンが壊れる | `wasm_dep_dirs_ok=false` を `if` の前に初期化し、`else` 直後の構造を変えない（§1.4）。ステップ 2-3 の完了基準で `make ext-test` を実行して確認する |
| 4 | `os.Open(depDirsFile)`（64 KiB+1 バイト上限の `io.LimitReader` 付き有界読み取り）・`os.WriteFile(resultFile, ...)`、および `test_helpers.go`（`_test.go` 除外対象外）の `exec.Command` 等が `golangci-lint` の `gosec` に抵触する | 該当行のみに最小範囲の `//nolint:gosec` と用途を限定した根拠コメント（`test_helpers.go` ではテストセットアップ専用と明記）を付け、ファイル・パッケージ全体への抑制は行わない。`make lint` で確認する |
| 5 | `make deadcode` で PR-1・PR-2 単体時に未到達関数が報告される | `make deadcode`（`Makefile:370`）は `deadcode -test -tags test,integration ./...` でテストからの呼び出しも到達可能と判定する。PR-1（ステップ 1-1・1-2）では `Evaluator` を含めずローカル関数群をテストからすべて呼び、PR-2（ステップ 1-4）では `ciskip_test.go` から `New`・`Evaluate`・`Skip` を呼ぶ。両 PR で `make deadcode` を完了基準に含める |
| 6 | 将来 `.github/workflows/ci.yml` に新しい重いジョブを追加した際に `internal/ciskip` の必須ジョブ一覧と乖離する | `workflow_pin_test.go` で `ci.yml` の `check-changes` と `needs: check-changes` を持つジョブ集合（要素・基数）と `internal/ciskip` の必須ジョブ定数集合の一致を常設ピン留めする（ステップ 2-4） |

**日程上のリスク:** フェーズ 3（PR-5）は PR-4 マージ後の GitHub Actions 実機結果を利用者が確認する待ち時間を含む。`PR-5` を `自動送信: hold` として確認完了後に着手する。

---

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
- [x] PR-2 マージ済み（対象ステップ: 1-4 / 1-5）
- [x] PR-3 マージ済み（対象ステップ: 2-1 / 2-2）
- [x] PR-4 マージ済み（対象ステップ: 2-3 / 2-4）
- [ ] PR-5 マージ済み（対象ステップ: 3-1 / 3-2）

---

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

すべての受け入れ基準（AC-01〜AC-20）と検証手段・実装ステップの対応を示す。テスト関数名は実装時に確定し、変更があれば `/runplan` が本表を更新する。

| AC | 種類 | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test | `internal/ciskip/git_test.go::TestVerifyParentCount`（親コミット 2 つ以上および 0 個で `ReasonNotSingleParent`、一時 git リポジトリ結合テスト含む） | ステップ 1-2 |
| AC-02 | test | `internal/ciskip/git_test.go::TestClassifyParentDiff`・`::TestClassifyDiffWithRealGitAndScript`・`internal/ciskip/ciskip_test.go::TestDecisionSkip`（PR-1 の単一親・非コード／非拡張のみおよび 0 バイト空コミットの判定と `Skip()`）、`internal/ciskip/ciskip_test.go::TestEvaluateSkipEligible`（単一親かつ非コード・非拡張ファイルのみ、および 0 バイト空コミットの先行検証通過で `ReasonSkipped`・`Skip() == true`）、`cmd/ciskip/main_test.go::TestRunWritesDecision`・`::TestRunMapsFlagsToInput`（各フラグから `ciskip.Input` への対応と、skip の結果ファイルへ `skip=true\n` を書き終了コード 0）、`internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckStepExecution`（`id: check` が `skip=true\n` を受けて両出力を `false` に書く） | ステップ 1-1・1-2・1-4・2-1・2-3・2-4 |
| AC-03 | test | `internal/ciskip/git_test.go::TestIsNonCodePath`・`::TestClassifyParentDiff`・`::TestClassifyDiffWithRealGitAndScript`・`internal/ciskip/ciskip_test.go::TestEvaluateChangedFilesReject`（分類器自身 `.github/workflows/ci.yml`・`cmd/ciskip/`・`internal/ciskip/`・`extension/scripts/*.sh` を含むコードファイルで `ReasonCodeChanged` かつ `gh` 未呼出、`extension/`・`README.md`・`README.en.md`・`docs/dev/guides/development.md`・WASM 依存配下で `ReasonExtensionChanged`） | ステップ 1-2・1-4 |
| AC-04 | test | `internal/ciskip/git_test.go::TestClassifyDiffWithRealGitAndScript`（実 `git` と実 `has-extension-changes.sh` でコードおよび拡張対象 `.md` から文書へのリネーム両側、非 ASCII・タブ・改行パスを検出し `ReasonCodeChanged` または `ReasonExtensionChanged`） | ステップ 1-2 |
| AC-05 | test | `internal/ciskip/github_test.go::TestVerifyParentCIValid`（親 run が `completed` かつ結論 `success`／`failure`、複数ページ正常取得含む、`Check changed files` が `success`、重い 3 ジョブが `success`／`skipped`、`doc-guard`・`secret-scan` 成否不問で `ReasonSkipped`） | ステップ 1-4 |
| AC-06 | test | `internal/ciskip/github_test.go::TestVerifyParentCIInvalidStates`（親 run 未完了、結論 `cancelled`／`timed_out`／`startup_failure` 等、4 ジョブの欠損・重複・`head_sha` 不一致・未完了（`in_progress` かつ `conclusion: success` 含む）・不適合結論・`check-changes` 失敗・不正 RFC3339 で各非スキップ `Reason`） | ステップ 1-4 |
| AC-07 | test | `internal/ciskip/github_test.go::TestVerifyParentCIRunSelection`（親 run 不在、別ワークフロー・別イベント・別 PR・別リポジトリ・`head_repository: null`、同一リポジトリで `pull_requests` 空／複数による `ReasonAmbiguousPRLink`、`(run_started_at, created_at, id)` タイブレーク、古い run のみ成功で最新 run が失敗・キャンセル・未完了） | ステップ 1-4 |
| AC-08 | test | `internal/ciskip/github_test.go::TestVerifyParentCISkipChain`（親 run が `check-changes=success` かつ重い 3 ジョブすべて `skipped` の連鎖時に `ReasonSkipped` かつ `NoticeBaseSHA` を設定） | ステップ 1-4 |
| AC-09 | test | `internal/ciskip/ciskip_test.go::TestValidateBaseContext`・`::TestEvaluateContextValidation`（フォーク PR・空 `headRepo`・リポジトリ形式違反・非 `main` ベース・欠損で `ReasonForkOrRepoMismatch` または `ReasonNonMainBase`・`Skip() == false`・`NoticeBaseSHA == ""`） | ステップ 1-1・1-4 |
| AC-10 | test | `internal/ciskip/ciskip_test.go::TestValidateRemainingContext`・`::TestEvaluateContextValidation`（`run_attempt >= 2` で `ReasonManualRerun`、`0`・負数・非整数・空白付きで `ReasonInvalidInput`、`Skip() == false`） | ステップ 1-1・1-4 |
| AC-11 | test | `internal/ciskip/git_test.go::TestResolveSyntheticMergeBase`・`::TestCheckOriginMain`・`internal/ciskip/ciskip_test.go::TestEvaluateBaseConsistency`・`internal/ciskip/github_test.go::TestVerifyParentBaseAnnotation`（`NoticeBaseSHA` 設定／非設定不変条件、`HEAD^1 != origin/main` で `ReasonMainMovedAtCheckout`、アノテーション 0 件／重複／不正 SHA／`mergeBaseSHA` 不一致で各非スキップ `Reason`） | ステップ 1-2・1-4 |
| AC-12 | test | `internal/ciskip/ciskip_test.go::TestValidateBaseContext`・`::TestValidateRemainingContext`・`::TestRunnerOptions`・`internal/ciskip/git_test.go::TestValidateDepDirsFile`・`::TestClassifyParentDiff`・`::TestCheckExtensionChangesEarlyExitNoDeadlock`・`internal/ciskip/ciskip_test.go::TestEvaluateFailSafe`・`internal/ciskip/github_test.go::TestVerifyParentCIAPIFailures`・`cmd/ciskip/main_test.go::TestRunWritesDecision`・`::TestRunEvaluatesEmptyContext`・`::TestRunMapsFlagsToInput`（評価完了は終了コード 0 とフラグ対応）・`::TestRunRejectsCommandLine`・`::TestRunResultFileWriteFailure`（コマンド行不正・結果ファイル書き込み失敗は終了コード 2・固定 stderr）・`::TestRunDoesNotEchoFlagValues`（フラグ値・機密の非出力）（API 障害・非 200・非 `*cmdrunner.ExitError`・コマンドおよび `RunTimeout` タイムアウト・`ErrOutputTooLarge`・ページ上限・厳密 JSON 違反・`git` 失敗・`has-extension-changes.sh` 失敗・`wasm-dep-dirs-ok=false`・`depDirsFile` 空／64 KiB+1／不正で `Skip() == false`・終了コード 0・機密非露出）、`internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckStepExecution`（`set -eo pipefail` 下で `mktemp` 失敗・`go` 非ゼロ終了・不正結果ファイル時に終了 0 で通常分類へフォールバック） | ステップ 1-1・1-2・1-4・2-1・2-3・2-4 |
| AC-12 | static | `internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckChangesPin`（`steps.normal.outputs.*` 伝播とフラグ集合一致）、`extension/test/repository.test.ts`（`make ext-test`：`wasm-dep-dirs.sh` 失敗時の `has_extension_changes=true` 維持） | ステップ 2-3・2-4 |
| AC-13 | test | `cmd/ciskip/main_test.go::TestRunWritesDecision`・`::TestShortSHA`（スキップ時・通常分類フォールバック時の 1 行診断ログ、`Reason`・短縮 SHA・`run=`・`status=` 形式）、`::TestRunDoesNotEchoFlagValues`・`::TestRunRejectsCommandLine`・`::TestRunResultFileWriteFailure`（フラグ値・未検証引数・結果ファイルパスの非出力と固定 stderr）、`internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckStepExecution`（`id: check` の `stdout` 通過と終了コード番号付きフォールバックログ） | ステップ 2-1・2-3・2-4 |
| AC-14 | test | `internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckStepExecution`（`skip=true\n` 一致時に `$GITHUB_OUTPUT` へ `has-code-changes=false`・`has-extension-changes=false` を出力し `stdout` の `::notice` を通過させる） | ステップ 2-3・2-4 |
| AC-14 | static | `internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckChangesPin`（`test`・`lint`・`extension` の `if:` 条件と必須ジョブ集合の一致、および `doc-guard`・`secret-scan` に `needs:`・`if:` がないこと） | ステップ 2-3・2-4 |
| AC-14 | manual | 下の「GitHub Actions 実機確認の記録（AC-20）」（実機で重い 3 ジョブが `skipped`、常時実行 2 ジョブが実行・成功） | ステップ 3-1 |
| AC-15 | static | `internal/ciskip/workflow_pin_test.go::TestCIWorkflowPermissions`（トップレベルが `contents: read` のみ、`check-changes` が `{contents, actions, checks}: read` のみ、他ジョブに `permissions:` なし、非コメント行に `: write`・`write-all`・`read-all` 不在） | ステップ 2-3・2-4 |
| AC-16 | diff | `docs/tasks/0119_ci_skip_doc_only_commit/_checks/check-ac16-ci-diff.sh`（PR-4 の merge-base と比較対象コミットで `.github/workflows/ci.yml` の `check-changes` 外のバイト一致と中間への別ジョブ／トップレベルキー非挿入を検証。実行結果と両コミット ID は下の「AC-16 検証スクリプト実行記録」へ記録する） | ステップ 2-3・2-4 |
| AC-17 | test | `internal/ciskip/git_test.go::TestIsNonCodePath`（非コード分類規則の境界テスト）、`internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckChangesPin`（`ci.yml` から抽出した正規表現を `regexp.Compile` して `isNonCodePath` と等価検証）・`::TestCIWorkflowCheckStepExecution`（非スキップ時・両 `false` 時に通常分類出力を維持） | ステップ 1-2・2-3・2-4 |
| AC-17 | static | `extension/test/repository.test.ts`（`make ext-test`：出力名 `has-extension-changes`・`git diff -z`・`wasm-dep-dirs.sh` 失敗時フォールバック維持）、`internal/ciskip/workflow_pin_test.go::TestCIWorkflowCheckChangesPin`（出力名 `has-code-changes`・`has-extension-changes` 維持） | ステップ 2-3・2-4 |
| AC-18 | static | `cmd/yt2column/adr_test.go::TestADRRecords`（既存テスト。ADR-0054 の存在・状態 `有効`・`docs/dev/adr/README.md` の番号順索引行を検証） | 既存（`#1075`・`#1076` で作成済み） |
| AC-19 | manual | PR-5 のレビューで、`docs/dev/adr/0054-ci-skip-doc-only-commit.md`（および `0014-deterministic-checks-in-go.md` の本タスク分）の `## 決定` が実装済みの振る舞いと一致し、未実装を示す食い違いの記述が残っていないことを目視で確認する | ステップ 3-2 |
| AC-20 | manual | 下の「GitHub Actions 実機確認の記録（AC-20）」へ確認日・PR 番号・2 ケースの実行 URL・全 6 ジョブ結果を記録し、PR-5 のレビューで確認する（スキップ判定および `id: check` の動作自体は AC-02・AC-05・AC-14 の `test`／`static` で検証する） | ステップ 3-1 |

### AC-16 検証スクリプト実行記録

-   **Merge-base:** `812f94a0dc3eef41bd69d8a4e023f994d68a0ee9`
-   **比較対象コミット:** `e7bd67bea926930982ae59b852b9fe9ac27bb987`
-   **実行出力:**

```text
PASS: .github/workflows/ci.yml diff is confined to check-changes
  base:     812f94a0dc3eef41bd69d8a4e023f994d68a0ee9 (check-changes:12, test:74)
  compared: e7bd67bea926930982ae59b852b9fe9ac27bb987 (check-changes:12, test:150)
```

### GitHub Actions 実機確認の記録（AC-20）

フェーズ 3（ステップ 3-1）で、PR-4 の実行結果から以下を記録する。

-   **確認日:** （未記録）
-   **対象 PR:** （未記録）
-   **ケース 1（コード変更を含む push での通常実行）:**
    -   実行 URL: （未記録）
    -   先頭コミットの種別（コード変更／記録コミット）と `check-changes`（`ci-base-sha` notice 出力含む） / `test` / `lint` / `extension` / `doc-guard` / `secret-scan`: （未記録）
-   **ケース 2（直後の非コード・非拡張のみコミットのスキップ）:**
    -   実行 URL: （未記録）
    -   `check-changes` / `test` / `lint` / `extension` / `doc-guard` / `secret-scan`: （未記録）

---

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 の AC-01〜AC-20 の検証がすべて完了し、§6 の 5 本の PR がすべてマージされている。
-   **品質:** 各 PR で `make test`・`make lint`・`make deadcode`（および `.github/workflows/ci.yml` を変更する PR-4 では `make ext-test`）がすべて通る。新設したテストと `_checks/` スクリプトは、対象条件を壊したときに失敗することを確かめてコミットメッセージに記録している。
-   **セキュリティ:** 最小読み取り権限（AC-15）、フォーク PR 排除（AC-09）、環境変数分離と `run:` への `${{` 非直埋め込み、厳密 JSON・出力上限・タイムアウト、および機密・生応答本文の非出力がテストで確かめられている。
-   **文書:** `package_reference.md`・`dev_tools.md`・`CLAUDE.md`・ADR-0054・ADR-0014 が実装済みの振る舞いと一致し、本タスクの未実装を示す食い違いの記述が残っていない。

---

## 9. 次のステップ (Next Steps)

-   全 PR のマージ後、`/runpr` が issue #1073 を閉じる。
-   将来 `.github/workflows/ci.yml` に `check-changes` の出力で制御される新しい重いジョブを追加するときは、同じ PR で `internal/ciskip` の必須ジョブ一覧と `workflow_pin_test.go` を更新する（[ADR-0054](../../dev/adr/0054-ci-skip-doc-only-commit.md)「結果」）。

## 変更履歴

- 2026-11-21 承認（isseis、#1078）
