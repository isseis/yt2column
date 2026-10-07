# 実装計画書：CLI の組み立て（設定の読み込み・FilePublisher・プロバイダの選択）

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-05 |
| Review date | 2026-10-05 |
| Reviewer | isseis |
| Comments | 編集上の修正（2026-10-06）: PR-9 作成ポイントのレビュー観点で、ステップ 7-1 の処理順（エスケープ → 伏せ字化）に合わせて「伏せ字化 → エスケープ」を「エスケープ → 伏せ字化」に直した。決定の変更はない。 編集上の修正（2026-10-06）: PR #96 のレビューで、最初のシグナルの直後に届いた 2 回目のシグナルが捨てられうることが分かり、ステップ 7-4 の手段の記述（購読を止める時点）を実装に合わせて直した。決定の変更はない（2 回目のシグナルで終了する、という設計書 3.8 の決定は同じ）。 実装に伴う追記（2026-10-06）: フェーズ 8 で、ステップ 8-2 に `TestValidateEnvNames`、8-4 に `integrationOptions` の変数と `TestIntegrationOptionsSkipMissingKey`、8-6・8-7 に `gateCLIIntegration` とその確かめ方を書き加え、8-8 の件数を 13 に改めた。いずれも既存の決定を具体化するもので、決定の変更はない。 実装に伴う修正（2026-10-07）: `/code-review` の指摘で、ステップ 8-2 の `make` の補助の引数（記録する変数の一覧とモデル名の変数を `make.go` の定義へ移す）と、共通の確認 `CheckChargedTarget` を書き改めた。決定の変更はない。 実装に伴う修正（2026-10-07）: PR-11 のレビューの指摘で、`security.md` §2 の統合テストの例外の更新をステップ 9-3（PR-12）から PR-11 へ前倒しした。ステップ 9-3 には `http2debug` の警告の記述が残る。 編集上の修正（2026-10-07）: 散文の意味は文字列一致では保証できないため、ステップ 9-4 から `TestREADMEDocumentsCLI` の案内の確認と `TestSecurityDocumentsCLIIntegration` を外し、機械的に確かめられる構造と契約値だけを固定するようにした（ガイドの「計画が再割り当てしたテスト」に当たる編集上の修正）。AC-43 の散文の部分（同時実行・SIGKILL・`--refresh` の案内、パーミッション `0o644` とハードリンク、統合テストの実行方法、`security.md` §2 の `http2debug` の警告と統合テストの例外）は、ステップ 9-7 の照合対象に挙げて突き合わせる。決定の変更はない。 |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定義した CLI の組み立てを、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。具体的には、既存のパッケージに必要な変更（設計書 3.10）を加えたうえで、`internal/config`・`internal/llm/provider`・`FilePublisher`・`internal/cachelock`・`internal/job`・`cmd/yt2column` を作り、統合テストと `make test-integration-cli` を加え、文書を更新する。

本計画は、設計書 §8 の実装優先順位 1〜9 を、そのままフェーズ 1〜9 とする。

### 1.2. 実装原則

-   設計書 §1.1 の設計原則に従う。特に、終了コードを手順で決めること、秘密情報を読むのは `internal/config` だけであること、上書きしないことを `link(2)` で保証すること、排他を `flock` で取り `yt-dlp` に引き継ぐこと、標準エラー出力に書く文字列を 1 つの関数で無害化することを守る。
-   本番コードで新設・変更するファイルは、設計書 §3.10 と §3.12 に挙げたものに限る。テスト用の補助として加えるファイルは §1.3 の「テスト用の補助」に挙げる。
-   ユニットテストのファイルは、対象と同じパッケージに置き、先頭に `//go:build test` を付ける（`internal/pipeline/pipeline_test.go:1` と同じ）。統合テストからも使う補助だけは `//go:build test || integration` とする（フェーズ 1 で `test_organization.md` に例外として足す）。
-   Go のコメント・識別子・文字列リテラルは英語で書く。`AC-NN`・`F-NNN`・`H-NN`・`I-NN` は Go のソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
-   **テスト用の補助の lint。** `test_helpers*.go` と `testutil/` の `.go` には `_test.go` の lint の除外が効かない（§1.3）。以下は、これらのファイルを作るか変えるすべてのステップに適用する。これらで `gosec` に当たる呼び出し（変数の実行ファイルの起動、変数のパスの読み書き、`0o600` より広いパーミッション）には、その行だけに `//nolint:gosec // <テストの一時ディレクトリの中であるなどの理由>` を付ける（既存の `internal/transcript/test_helpers.go:154` と同じ形）。テストが注入するエラーはパッケージの静的なエラーとして宣言し（`err113`）、後始末で無視する戻り値は `_ =` で受ける（`errcheck`）。対象のステップは 1-3・1-4・1-5・1-6・4-3・5-2・6-1・6-4・7-6・8-1・8-2 である。
-   テストの名前は計画上の名前である。実装で変える場合は、§5 を同じコミットで更新する。
-   各テストは、対象の分岐を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（[CLAUDE.md](../../../CLAUDE.md)「Testing Strategy」）。壊す対象を挙げるステップは、そのテストを導入する PR の中に置く（フェーズ 1 は PR-1 のステップ 1-3、PR-2 のステップ 1-5、PR-3 のステップ 1-11。フェーズ 7 は PR-9 のステップ 7-2、PR-10 のステップ 7-10。それ以外のフェーズは最後のステップ）。
-   各フェーズの完了条件は、`make fmt` → `make test` → `make lint` が通ることである。`make lint` は `--build-tags test,integration` と `go vet -tags integration ./...` でコンパイルするので、そのフェーズで加えたタグ付きのファイルは、最終的に使うタグでコンパイルされる。
-   CI は、変更が `*.md` と `docs/` だけの PR ではテストを実行しない（`.github/workflows/ci.yml` の `check-changes`）。文書だけを変えるコミットでも、`make test` を手元で実行する（ステップ 9-4 の文書のテストのため）。
-   実 `yt-dlp`・実 LLM API を使う確認（ステップ 5-4・8-11・9-5・9-6）は、利用者の承認を得てから行う（[CLAUDE.md](../../../CLAUDE.md)「Tool Execution Safety」）。

### 1.3. 既存コード調査結果

HEAD `386f6e2`（ブランチ `issei/0005-cli-assembly-02`）で確認した。設計書が行番号を記したコミット `274b18b` から HEAD までに、`cmd/`・`internal/`・`Makefile`・`.golangci.yml` の変更はない（`git diff --stat 274b18b HEAD -- cmd internal Makefile .golangci.yml` の出力が空。2026-10-05）。したがって設計書の `file:line` は HEAD でもそのまま使える。ベースラインの `go test -tags test ./...` は全パッケージが `ok` または `[no test files]` だった（2026-10-05、HEAD `386f6e2`。`cmd/yt2column`・`internal/publisher`・`prompts` は `[no test files]`）。

**`internal/transcript`**

-   `validateVideoURL` の参照は、`video_id.go:13`（doc コメント）・`:16`（定義）、`ytdlp.go:54`・`:82`、`video_id_test.go:33`・`:35`・`:83`・`:85`・`:89`（呼び出しと失敗メッセージ）である。`docs/tasks/` を除く文書には現れない（`rg -n "validateVideoURL" --glob '!docs/tasks/**' .`、HEAD `386f6e2`）。
-   `commandExecutor.Run` の呼び出しは、本番では `ytdlp.go:206` だけである。`osExecutor{}.Run` を直接呼ぶテストは `exec_test.go:69`・`:103`・`:130`・`:210`・`:286`・`:302` の 6 か所、`commandExecutor` の fake は `test_helpers.go:40-53` の `fakeCommandExecutor` だけである。引数を足すと、これらすべてに追従が要る（ステップ 1-4）。
-   `exec_test.go:260` の `TestCommandExecutorWaitDelay` の部分テスト「timeout while a descendant holds stderr」は、子孫（`sleep`）が標準エラー出力のパイプを保持したまま残ることを前提にしている。プロセスグループごと止める変更（ステップ 1-5）の後は、同じグループの子孫は取り消しで止まるので、この部分テストは `WaitDelay` を通らなくなる。`WaitDelay` が要るのは、子孫がグループの外へ出た場合である（設計書 3.4）。macOS には `setsid(1)` がなく、CI の `/bin/sh`（dash）は端末なしでは `set -m` でジョブを別のグループに移さないので、シェルだけでは子孫をグループの外へ出せない。テストのバイナリ自身を補助のプロセスとして起動する既存の仕組み（`exec_test.go:244` の `TestExecutorHelperProcess`）を使う（ステップ 1-5）。
-   `placeRealCache`（`test_helpers.go:169-176`）の呼び出しは、`cache_test.go` に 6 か所、`ytdlp_test.go` に 7 か所ある。これらは「スロット `a`、ポインタ `a`」の配置を前提にし、いずれも新しい `t.TempDir` から始まる。`writeSlot`（`cache.go:137-143`）は、有効なスロットがない場合に `a` を返す。`prepareWriteSlot`・`persistSlot`・`commitCache`（`cache.go:211`・`:238`・`:272`）はビルドタグのない本番のコードなので、`-tags integration` だけのビルドからも使える。
-   `PruneCache` の失敗は、既存のテストではスロットのディレクトリを `0o500` にして起こしている（`cache_test.go:177` の `TestPruneCachePartialFailure`。`requireNonRoot` と `chmodForTest` を使う）。
-   `requireNonRoot`・`chmodForTest` は `internal/transcript/test_helpers.go:307`・`:316` にあり、`requireNonRoot` は `internal/writer/test_helpers.go:122` にも同じものがある（タスク 0004 の計画で承認された重複）。

**`internal/llm/deepseek`**

-   `errPaddedModel` の参照は `errors.go:25`（定義）と `deepseek.go:61` だけで、テストからの参照はない。`TestNew`（`deepseek_test.go:79`）の「rejects invalid model names」は前後の空白のあるモデル名を拒否することを確かめるが、番兵は判別していない。
-   ループバックへ向けたクライアントを作る `newTestClient` と `validateLoopbackEndpoint` は `test_helpers.go:58-97`（`//go:build test`）にある。
-   統合テストの実行条件の判定は `integration_env_test.go:73-116` の `integrationSettingsFrom`（`//go:build test || integration`）で、呼び出しは `integration_test.go:31` と、`deepseek_test.go:743` の `TestIntegrationSettings`（`:784`）である。同じファイルの `integrationGenerateTimeout`・`integrationGenerateCalls`（`:30-43`）は `integration_test.go` と `makefile_test.go:115` が使う。`TestIntegrationTestBuildTag`（`deepseek_test.go:843`）が `integration_env_test.go` の 1 行目を `//go:build test || integration` に固定している。
-   `make` を偽のコマンドで実行する補助は `makefile_test.go:15-107`（`makeStubScript`・`makeChildEnvAllowlist`・`makeInvocation`・`runMakeTarget`）で、`TestMakeTestIntegrationDeepSeek`（`:109`）と `TestMakeOptInExportedToDeepSeekTargetOnly`（`:157`）が使う。
-   `firstLineIs` は `deepseek_test.go:826` と `internal/transcript/ytdlp_test.go:1676` に別々にある。本計画は `deepseek_test.go` のものだけを `deepseektestutil` へ移し（§1.4）、`internal/transcript` のものは変えない（`internal/transcript` のテストから `internal/llm/deepseek/testutil` を import するのは、パッケージの責務に合わないため）。
-   `TestMain`（`deepseek_test.go:37`）は、proxy の環境変数を到達できないリスナー（`test_helpers.go:426` の `startBlackholeProxy`。非公開）へ向けて、本番の送信先への要求がネットワークへ出ないようにしている。

**`internal/writer`・`internal/publisher`・`internal/pipeline`**

-   `checkDisplayString`（`output.go:115-123`）の呼び出しは `output.go:57`（タイトル）と `:108`（`checkModelString`）の 2 か所である。
-   `internal/publisher` は `Publisher` interface（`publisher.go`）だけで、テストファイルはない。fake は `internal/publisher/testutil/mocks.go` の `FakePublisher`。
-   `TestFakesCarryBuildTag`（`internal/pipeline/pipeline_test.go:515-534`）は `../*/testutil/*.go` を数えて 8 件に固定し、1 行目が `//go:build test` であることを確かめる。現在の 8 件は `internal/{llm,publisher,transcript,writer}/testutil/{mocks,mocks_test}.go` である。`test_helpers*.go` の 1 行目を確かめるテストはない。`pipeline_test.go` は、ほかにもリポジトリ全体の文書と構成のガード（`TestPromptsREADMEMatchesContract` など）を持つ。
-   fake（`FakeLLMClient`・`FakeArticleWriter`・`FakePublisher`・`FakeTranscriptSource`）は、設定した結果を返すだけで、呼び出しの途中で止まったり `context` を取り消したりできない。中断や準備完了の印が要るテストは、テストのファイルの中で fake を包む小さな型を作る（共有の fake は変えない）。

**環境変数を読む場所（AC-09、I-01）**

-   テスト以外の Go のコードで環境変数を読む関数を参照するのは、`internal/transcript/ytdlp.go:205`（`os.Environ`）だけである（`rg -n "Getenv|LookupEnv|Environ\(|ExpandEnv|syscall\.Getenv" -g '*.go' -g '!*_test.go' .` の結果は 2 件で、もう 1 件は `//go:build test` の `internal/transcript/test_helpers.go:293` である。HEAD `386f6e2`）。`cmd/mergepr`・`internal/mergepr` は該当しない。
-   秘密情報の変数名の文字列（`DEEPSEEK_API_KEY`・`SLACK_WEBHOOK_URL`）は、現在 `_test.go` と `//go:build test || integration` のファイルにだけ現れる（`rg -n "DEEPSEEK_API_KEY|SLACK_WEBHOOK_URL" -g '*.go' .`、HEAD `386f6e2`）。テスト用の `YT2COLUMN_TEST_DEEPSEEK_API_KEY` は `DEEPSEEK_API_KEY` を部分文字列として含むが、テストのコード（ステップ 8-1 の移動先も含む）にだけ現れる。
-   ビルドの制約は、現在 `test`・`integration`・`test || integration` の 3 種類だけである。フェーズ 5 の `internal/cachelock/cachelock.go` が、初めての `//go:build unix` の本番のファイルになる。

**標準ライブラリの前提**

-   `syscall.Flock`・`syscall.LOCK_EX`・`syscall.LOCK_NB`・`syscall.EWOULDBLOCK`・`syscall.SysProcAttr.Setpgid`・`syscall.Kill` は、`GOOS=darwin` と `GOOS=linux` でコンパイルでき、`GOOS=windows` では `syscall.Flock` が未定義になる（scratchpad の確認用ファイルを `go vet` で 3 つの GOOS についてコンパイルして確認。手元の Go は `go1.27.1`、2026-10-05）。`exec.Cmd.Cancel`（Go 1.20）・`exec.Cmd.ExtraFiles`・`exec.Cmd.WaitDelay`（Go 1.20）・`(*exec.Cmd).Environ`（Go 1.19）・`signal.Ignored`（Go 1.11）・`constraint.Parse`（Go 1.17）は `go doc` で存在を確認した。いずれも `go.mod` の `go 1.26.5` より前に加わったものである。
-   ステップ 1-5 で `internal/transcript` が `SysProcAttr.Setpgid` を使い、`internal/cachelock` が `//go:build unix` になるので、`cmd/yt2column` は Windows ではビルドできなくなる。要件書 4.4 は Windows を対象外とし、CI は `ubuntu-latest` だけである（`.github/workflows/ci.yml`）。

**lint の前提**

-   `.golangci.yml` は `gosec`・`err113`・`goconst`・`mnd`・`gocyclo`（20）などを有効にし、`_test.go` に限って `gosec`・`err113`・`goconst`・`gocyclo`・`errcheck`・`dupl` を除外する。`test_helpers*.go` と `testutil/` の `.go` は `_test.go` でないので除外が効かない。`mnd` は `0o600`・`0o700`・`0o644`・`0o755` を無視するが、`gosec` の G302・G306 は `0o600` より広いパーミッションを指摘する。`make lint` は `--build-tags test,integration` で解析し、続けて `go vet -tags integration ./...` を実行する（`Makefile`、`TestLintTagsIncludeIntegration` が固定）。

**テスト用の補助（設計書 §3.12 にないファイル）**

-   `internal/transcript/testutil/helpers.go`（`//go:build test || integration`、ステップ 6-1）: 偽の `yt-dlp` のスクリプトを作る公開関数と、ディレクトリの内容の一覧を取る公開関数。`internal/job` と `cmd/yt2column` のユニットテスト、`cmd/yt2column` の統合テストが共有する。`test_organization.md` の分類 A（公開の API だけを使い、複数のパッケージから使う）に当たる。
-   `cmd/yt2column/test_helpers_integration.go`（`//go:build test || integration`、ステップ 7-6）: 統合テストとユニットテストが共有する補助（環境の対応表から `config.LookupFunc` を作る関数、CLI の統合テストの `IntegrationOptions`、`Generate` の回数を数える型）。
-   `internal/publisher/test_helpers.go`・`internal/cachelock/test_helpers.go`・`internal/job/test_helpers.go`・`cmd/yt2column/test_helpers.go`（いずれも `//go:build test`）: パッケージの中の補助。権限の失敗を確かめるテストのための `requireNonRoot` と `chmodForTest`（パーミッションを後始末で戻す）も、ここに置く。共有するには、特定のパッケージに属さないテスト専用のパッケージを新設する必要がある。しかし `test_organization.md` は `testutil/` を対象のパッケージの下に置くと定めるだけで、この種の汎用の補助の置き場所を定めていない。そのため、タスク 0004 と同じく各パッケージに複製する（`requireNonRoot` は計 6 か所、`chmodForTest` は計 5 か所になる）。**この判断は本計画のレビューで承認を受ける。**

**具体型の構築の経路**

-   `config.Config` は非公開のフィールドだけを持ち、ゼロ値のほかは `Load` だけが作る（設計書 3.1）。`internal/config` の本番コードで `Config{...}` の複合リテラルを書くのは `Load` の中だけにする。`provider` の AC-11 は、プロバイダを直接受け取る `provider` の非公開の関数で確かめる（設計書 3.2）ので、`Config` を `Load` 以外で作る必要はない。
-   `publisher.FilePublisher` は非公開のフィールドを持ち、本番では `NewFilePublisher` だけが作る。I-03 の継ぎ目（書き込み先と `link` の差し替え）は非公開のフィールドとし、テストはパッケージの中の補助で差し替える。`FilePublisher` の具体型で分岐する処理はないので、リーフパッケージへの移動はしない。

**architecture との整合**

-   調査の範囲で、設計書の決定を変える必要のある不整合は見つかっていない。
-   設計書 §8 に、次の 2 点を編集上の修正として書き加えた（同書の Comments に記録。決定の変更はない）: `test_organization.md` の例外と `TestFakesCarryBuildTag` の改修を §8 の 1（本計画のフェーズ 1）で行うこと。例外を初めて使うのがフェーズ 1 の `test_helpers_cache_seed.go` だからである。`package_reference.md` の各パッケージの行を、同書の規則（パッケージを追加・変更するコミットで更新する）に従って各フェーズで更新すること。

### 1.4. 名前の変更・移動の一覧

本表が名前の変更の唯一の一覧である。本計画の他の節は、本表に従う。

| 変更前 | 変更後 | ステップ |
|---|---|---|
| `internal/transcript` の非公開の `validateVideoURL` | `transcript.ValidateVideoURL`（内容は変えない） | 1-1 |
| `internal/llm/deepseek` の非公開の `errPaddedModel` | `deepseek.ErrPaddedModel` | 1-2 |
| `TestFakesCarryBuildTag` の対象 `../*/testutil/*.go` | `internal` の下のすべての `testutil/` の `.go`。あわせて `cmd/`・`internal/` の `test_helpers*.go` の 1 行目も確かめる | 1-8 |
| `integration_env_test.go` の `integrationSettingsFrom`・`integrationSettings`・`integrationAction`（`integrationSkip`・`integrationFail`・`integrationRun`） | `deepseektestutil.SettingsFrom`・`IntegrationSettings`・`IntegrationAction`（`ActionSkip`・`ActionFail`・`ActionRun`）。`MissingKeyAction`・`IntegrationOptions` を新設 | 8-1 |
| `integration_env_test.go` の `integrationOptInEnv`・`integrationAPIKeyEnv`・`integrationModelEnv`・`godebugEnv`・`integrationOptInValue`・`http2VerboseSettings` | `deepseektestutil` の `DeepSeekOptInEnv`・`APIKeyEnv`・`ModelEnv`・`GODEBUGEnv`・`OptInValue`（`http2VerboseSettings` は非公開のまま移す）。CLI 用に `CLIOptInEnv` を新設 | 8-1 |
| `makefile_test.go` の `makeStubScript`・`makeChildEnvAllowlist`・`makeInvocation`・`runMakeTarget` | `deepseektestutil` の `RunMakeTarget`・`MakeInvocation`（スクリプトと allowlist は非公開のまま移す） | 8-2 |
| `deepseek_test.go` の `firstLineIs` | `deepseektestutil.FirstLineIs` | 8-2 |
| `deepseek_test.go` の `TestIntegrationSettings` | `internal/llm/deepseek/testutil/integration_settings_test.go` の `TestSettingsFrom` | 8-3 |

`integration_env_test.go` は削除せず、`integrationGenerateTimeout`・`integrationGenerateCalls` を残す（`TestIntegrationTestBuildTag` が 1 行目を固定しているので、ビルドタグも変えない）。

### 1.5. implementation_handoff.md の項目への対応

| ID | 対応 |
|---|---|
| I-01 | 走査は、設計書 3.1 の定義に従い、プロセスの環境変数を読む標準ライブラリの関数への参照を、許可した場所以外ではすべて失敗にする。対象を固定の一覧に限定せず、呼び出し元に値を返すものに限らず、内部で環境変数を読むもの（`os.UserCacheDir` のように呼び出し元に値を返さず内部で読むものを含む）も対象にする（限定すると、一覧にない内部で読む関数が検査をすり抜ける）。検査は lint の規則ではなく、`internal/config/envaccess_test.go` のテストとする（`depguard`・`forbidigo` は import や識別子の単位の規則で、許可する場所を「ファイルと参照の種類と回数の組」で表せないため）。対象を外すと失敗することは、走査の関数を一時ディレクトリの小さなソースに当てる自己テストで確かめ、内部で環境変数を読む関数（間接に読むもの）を少なくとも 1 つ含める（ステップ 2-5）。 |
| I-02 | `cmd/yt2column` のテストのバイナリ自身を、環境変数で CLI のモードに切り替えて子プロセスとして起動する。子プロセスは `main` と同じシグナルの購読の関数を、fake の `LLMClient` を返す `deps` で実行する。偽の `yt-dlp` と fake の `LLMClient` は、準備ができた時点で印のファイルを原子的に（一時ファイルへ書いてから改名して）作り、テストはその印を上限付きで待ってからシグナルを送る。終了後、記録した PID のプロセスがないことを上限付きで確かめる（ステップ 7-8）。シグナルの購読を外すと失敗することを、実装を壊して確かめる（ステップ 7-10）。 |
| I-03 | `FilePublisher` の非公開のフィールドに、一時ファイルへの書き込み先を包む関数と、`link` を行う関数を置く。本番の値は `NewFilePublisher` が設定し、テストはパッケージの中の補助で「一定のバイト数を書いた後にエラーを返す書き込み先」と「指定の errno を返す `link`」に差し替える（ステップ 4-1・4-2）。後始末（一時ファイルの削除）を外すと失敗することを確かめる（ステップ 4-5）。 |
| I-04 | 統合テストは `run` を 1 回だけ呼び、部分テストやリトライの中で呼ばない。`productionDeps()` の `newLLMClient` が返す実物のクライアントを、`Generate` の呼び出しを数えるだけの薄い型で包み、`run` の後に回数が 1 であることを確かめる。組み立ては `productionDeps()` のままで、包むのは LLM のクライアントの外側だけである（ステップ 8-7）。`Generate` の内側（アダプタや Transport）のリトライはこの方法では数えられないが、LLM の呼び出しのリトライはスコープ外（#48）であり、現在のアダプタにはない。 |

design_handoff.md の H-01〜H-03 は、すべて設計書 §3.13 に対応が記録されている。

## 2. 実装ステップ (Implementation Steps)

ステップは `X-Y` 形式で表す（X: フェーズ番号、Y: フェーズ内の連番）。テストと AC の対応は §5 にまとめる。

### フェーズ 1: 既存パッケージの変更

**対象ファイル**
-   変更: `internal/transcript/video_id.go`・`video_id_test.go`・`ytdlp.go`・`ytdlp_test.go`・`exec.go`・`exec_test.go`・`test_helpers.go`
-   新設: `internal/transcript/test_helpers_cache_seed.go`（`//go:build test || integration`）
-   変更: `internal/llm/deepseek/errors.go`・`deepseek.go`・`deepseek_test.go`・`test_helpers.go`
-   新設: `internal/llm/deepseek/test_helpers_endpoint.go`（`//go:build test`）
-   変更: `internal/writer/output.go`・`output_test.go`。新設: `internal/writer/article.go`・`article_test.go`
-   変更: `internal/pipeline/pipeline_test.go`・`docs/dev/developer_guide/test_organization.md`・`docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 1-1**: `validateVideoURL` を `ValidateVideoURL` に改名する（§1.4）。§1.3 に挙げた参照をすべて追従させ、doc コメントの先頭の名前と、`video_id_test.go` の失敗メッセージの関数名も改める。検証の内容は変えない。
-   [x] **ステップ 1-2**: `errPaddedModel` を `ErrPaddedModel` として公開し（§1.4）、公開の番兵の宣言に移して doc コメントを書く（構築のエラーで公開する番兵はこれだけである。設計書 3.10 の例外 E2）。`TestNew` の「rejects invalid model names」で、前後に空白のあるモデル名では `errors.Is(err, ErrPaddedModel)` が真、それ以外の不正なモデル名では偽であることを確かめる。
-   [x] **ステップ 1-3**: `test_helpers_endpoint.go` に `NewForLoopbackTest`（設計書 3.10）を作る。送信先は既存の `validateLoopbackEndpoint` で検査し、ループバック以外なら `t.Fatal` する。`newTestClient` は、同じ構築の手順を重複させずにこの関数を使う形に改める。`TestNewForLoopbackTest` を足す: ループバックの送信先へ要求が届くこと（既存の `newRecordingServer`）と、ループバック以外の送信先では `Fatal` を記録して戻らず、クライアントを返さないこと（`Fatal` 系を記録する `testing.TB` の包みで確かめる）。PR-1 のテスト（`ValidateVideoURL` の改名の参照、`TestNew` の `ErrPaddedModel`、`TestNewForLoopbackTest`）を 1 つずつ壊して失敗することを確かめ、コミットメッセージに記録し、`make fmt` → `make test` → `make lint` を通す。

### PR-1 作成ポイント: existing internal symbol exposure

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `feat(0005): publish the video URL validator, padded-model sentinel, and loopback constructor`

**レビュー観点**: `ValidateVideoURL` の改名で検証の内容と番兵が変わらず、§1.3 の参照がすべて追従していること（ステップ 1-1） / `ErrPaddedModel` の公開で前後の空白の拒否だけがこの番兵になり、ほかの不正なモデル名は包まないこと（ステップ 1-2） / `NewForLoopbackTest` がループバック以外を `Fatal` で拒否し、`newTestClient` と同じ構築の手順を重複させないこと（ステップ 1-3） / `//go:build test` の非 `_test.go` の補助がそのタグでコンパイルされ、`gosec` の抑止が対象の行だけであること（ステップ 1-3）

**実装モデル要件**: standard

**判定理由**: 既存の識別子の改名・公開とテスト用コンストラクタの追加に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・2 つ以上の Conditional check のいずれにも該当しないため（該当する Conditional check はビルドタグ下の非 `_test.go` のソースの 1 つだけである）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

-   [x] **ステップ 1-4**: `Options` に `InheritedFiles` を足し、`commandExecutor.Run` の引数に加えて `osExecutor` が `exec.Cmd.ExtraFiles` に渡す（設計書 3.10）。§1.3 の 6 か所のテストの呼び出しと `fakeCommandExecutor`（渡されたファイルを記録する）を追従させる。テストを 2 つ足す: `TestFetchPassesInheritedFiles`（`Fetch` が `Options` のファイルをそのまま実行に渡す。fake で確かめる）、`TestCommandExecutorInheritedFiles`（子プロセスが記述子 3 で渡したファイルを読める。スクリプトで確かめる）。
-   [x] **ステップ 1-5**: `osExecutor.Run` で、子プロセスを新しいプロセスグループで起動し、`context` の終了時にグループ全体へ SIGKILL を送る（`exec.Cmd.SysProcAttr` と `exec.Cmd.Cancel`。設計書 3.10）。
    -   `TestCommandExecutorKillsProcessGroup` を足す: 子が孫を起動して両方の PID を記録し、取り消しの後、両方のプロセスがなくなることを上限付きで確かめる。孫は、テストが後始末で作る解放の印のファイルが現れるか、数分の上限に達したら自分で終了するものにする（失敗時やタイムアウトでも残り続けない）。
    -   `TestCommandExecutorWaitDelay` の「timeout while a descendant holds stderr」は前提が変わる（§1.3）。`osExecutor` が起動する子をテストのバイナリ自身の補助のモード（既存の `TestExecutorHelperProcess` と同じ仕組み）にし、その子が子孫を `SysProcAttr.Setpgid` で起動して、子孫がグループの外で標準エラー出力のパイプを保持する形にする（シェルのスクリプトではプロセスグループを変えられないため）。この子孫も、解放の印か数分の上限で自分で終了するものにする。`WaitDelay` の上限の確認は変えない。
    -   PR-2 のテスト（`TestFetchPassesInheritedFiles`・`TestCommandExecutorInheritedFiles`・`TestCommandExecutorKillsProcessGroup`・`TestCommandExecutorWaitDelay` の改めた部分テスト）を 1 つずつ壊して失敗することを確かめ、コミットメッセージに記録し、`make fmt` → `make test` → `make lint` を通す。

### PR-2 作成ポイント: yt-dlp child-process management

**対象ステップ**: 1-4 / 1-5

**推奨タイトル**: `feat(0005): pass inherited files to yt-dlp and terminate its process group`

**レビュー観点**: `InheritedFiles` が `Fetch` から `osExecutor` までそのまま渡り、記述子 3 で子がファイルを読めること（ステップ 1-4） / 子を新しいプロセスグループで起動し、`context` の終了でグループ全体を SIGKILL して、子孫が標準エラー出力のパイプを保持しても `WaitDelay` で戻ること（ステップ 1-5） / 止まらないテストの子孫が解放の印か上限で自分から終了し、終了済みの PID へシグナルを送らないこと（ステップ 1-5） / 高リスクな子プロセスの制御をこの PR に隔離してレビューすること

**実装モデル要件**: frontier-recommended

**判定理由**: プロセスグループの停止（ステップ 1-5）は復旧・シグナル制御に類する独立した高リスクなステップであり、この PR に隔離してレビューするため。加えてビルドタグ下の非 `_test.go` のソースという Conditional check に該当するため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

-   [x] **ステップ 1-6**: `test_helpers_cache_seed.go` に `SeedCacheForTest`（設計書 3.10）を作る。`prepareWriteSlot`・`persistSlot`・`commitCache` で、空のキャッシュに対してスロット `a`・ポインタ `a` の配置を作る。内容は検証しない（AC-48 のテストが途中で切れた字幕を置くため）。`placeRealCache` の中身をこの関数の呼び出しに置き換え、§1.3 の 13 か所の呼び出しを変えずに通す。
-   [x] **ステップ 1-7**: `checkDisplayString` を、番兵を包まずに理由だけを返す内部の関数に分け、既存の 2 か所の呼び出しは `ErrMalformedOutput` で包む。`article.go` に `ErrInvalidArticle` と `Article.CheckPublishable`（設計書 3.5 の表）を作る。`article_test.go` に `TestArticleCheckPublishable` を作る: 表の各フィールドと各条件の拒否、受理する境界（`ModelVersion` が空、`Body` の `\n`・`\t`）、エラーが `ErrInvalidArticle` を包み `ErrMalformedOutput` を包まないこと、エラーの文字列に値（目印の文字列）が現れないこと。`output_test.go` の既存のテストは変更せずに通し、`ErrMalformedOutput` の拒否が `ErrInvalidArticle` を包まないことを 1 件加える。
-   [x] **ステップ 1-8**: `test_organization.md` に、統合テストからも使う補助（`testutil/` のファイルと `test_helpers_*.go`）は `//go:build test || integration` とする例外を足す。同じコミットで `TestFakesCarryBuildTag` を改める（§1.4、設計書 3.11）: `internal` の下のすべての `testutil/` の `.go` を数え（件数はこの時点では 8 のまま）、それらと `cmd/`・`internal/` の `test_helpers*.go` の 1 行目が `//go:build test` か `//go:build test || integration` のどちらかであることを確かめる。
-   [x] **ステップ 1-9**: `package_reference.md` を更新する。`internal/transcript` の行: `ValidateVideoURL`・子プロセスへ引き継ぐファイル・プロセスグループの停止・`SeedCacheForTest`（`test` または `integration` のビルドだけ）。`internal/llm/deepseek` の行: `ErrPaddedModel`、`test` のビルドでは他のパッケージも `NewForLoopbackTest` でループバックの送信先に向けられること（現在の「only the test helper replaces」の記述を改める）。`internal/writer` の行: `CheckPublishable`。
-   [x] **ステップ 1-10**: [cache_consistency.md](../../dev/cache_consistency.md) §7 のチェックリストを、本フェーズの `ytdlp.go`・`exec.go` の変更（中断された `yt-dlp` の止め方を含む）に当てはめて確かめ、結果をコミットメッセージに記録する。
-   [x] **ステップ 1-11**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `CheckPublishable` の各条件を 1 つずつ外す、`TestFakesCarryBuildTag` の走査を 1 段に戻す・`test_helpers*.go` を対象から外す（誤ったタグの入れ子の `testutil/` のファイルと `test_helpers_x.go` を一時的に置いて、それぞれ失敗すること）。
-   [x] **ステップ 1-12**: 次の名前が `docs/tasks/` 以外に残っていないことを検索で確かめる: `validateVideoURL`・`errPaddedModel`（コメントと文書を含む）。`make fmt` → `make test` → `make lint` を通す。

### PR-3 作成ポイント: shared cache seeding, article validation, and build-tag guard

**対象ステップ**: 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12

**推奨タイトル**: `feat(0005): add the shared cache seeder, article validation, and build-tag guard`

**レビュー観点**: `SeedCacheForTest` が空のキャッシュにスロット `a`・ポインタ `a` を置き、`placeRealCache` の 13 か所の呼び出しを変えずに通すこと（ステップ 1-6） / `Article.CheckPublishable` が設計書 3.5 の表のとおりに必須の値と制御文字を拒否し、エラーが値を含まず `ErrMalformedOutput` を包まないこと（ステップ 1-7） / `TestFakesCarryBuildTag` が `internal` の下のすべての `testutil/` と `cmd/`・`internal/` の `test_helpers*.go` を数え、`test || integration` の 1 行目を許すこと（ステップ 1-8） / `package_reference.md` が PR-1〜PR-3 の公開 API と一致すること（ステップ 1-9） / フェーズ 1 の締め（`cache_consistency.md` の確認・`CheckPublishable` とガードの壊し確認・改名した名前の検索）がステップ 1-10〜1-12 で行われ、`validateVideoURL`・`errPaddedModel` が `docs/tasks/` 以外に残らないこと

**実装モデル要件**: frontier-recommended

**判定理由**: `gosec` の抑止（ステップ 1-6 の非 `_test.go` の補助）とビルドタグ下の非 `_test.go` のソース（同）の 2 つの Conditional check に該当し、加えて制御文字を拒否する `CheckPublishable` というセキュリティの検査を含むため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `internal/config`

**対象ファイル**
-   新設: `internal/config/config.go`・`cachedir.go`・`config_test.go`・`cachedir_test.go`・`envaccess_test.go`
-   変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 2-1**: `config.go` に `Provider`・`Config` とその公開メソッド・`LookupFunc`・`Load`・`ErrMissing`・`ErrInvalid`・`VarError` を作る（設計書 3.1）。検証の規則は要件書 F-001 の表と箇条書きのとおりとし、値を補正しない。`VarError.Reason` は固定の文字列の定数から選び、値から組み立てない。拒否した変数ごとの `*VarError` を `errors.Join` で返す。
-   [x] **ステップ 2-2**: `cachedir.go` に、OS の名前と `LookupFunc` を引数で受け取る非公開の関数で、キャッシュディレクトリの既定値の規則（設計書 3.1 の表と箇条書き）を作る。`Load` は `runtime.GOOS` を渡す。既定値を決められない場合は、`YT2COLUMN_CACHE_DIR` の `ErrMissing` として、`HOME` または `XDG_CACHE_HOME` から決められなかったことを固定の文言で報告する。
-   [x] **ステップ 2-3**: `Load` に `GODEBUG` の HTTP/2 の記録の判定を加え、`HTTP2DebugEnabled` で返す（設計書 3.1）。拒否はしない。
-   [x] **ステップ 2-4**: `config_test.go`・`cachedir_test.go` に次を作る。入力は要件書の AC の例をそのまま使う。
    -   `TestLoadValid`（AC-01）、`TestLoadDefaults`（AC-02）、`TestLoadMissing`（AC-03）、`TestLoadEmpty`（AC-04。表の 6 つの変数のそれぞれ）、`TestLoadInvalid`（AC-05）。
    -   `TestLoadErrorsOmitValues`（AC-06）: 各変数に互いに異なる目印を含む値を設定し、どの拒否のエラーの `Error()` にもどの目印も現れないこと。
    -   `TestLoadReportsAllInvalid`（AC-07）: `ErrMissing` と `ErrInvalid` の両方が `errors.Is` で真になり、拒否した変数の名前がすべて現れること。
    -   `TestConfigOutputRedactsSecrets`（AC-08）: `%v`・`%+v`・`%#v`・`log/slog`（テキストと JSON）・`encoding/json` に、秘密情報の値が現れないこと。JSON の行は、フィールドが非公開であることによって成り立つ構造上の確認である。
    -   `TestLoadHTTP2Debug`（AC-51 の設定の部分）: `http2debug=1`・`http2debug=2`・他の設定と並んだ `http2debug=1` で真、未設定・`http2debug=0` で偽であること。
    -   `TestDefaultCacheDir`: `darwin`・`linux` のそれぞれで、`HOME`・`XDG_CACHE_HOME` の有無・空・相対パスの組み合わせ、およびそれ以外の OS で既定値がないこと。
-   [x] **ステップ 2-5**: `envaccess_test.go` に、環境変数を読む場所の検査（設計書 3.1、I-01）を作る。
    -   **走査の対象:** リポジトリのテスト以外の `.go` ファイル。次のファイルをテストのコードとして除く: `_test.go` と、`//go:build` の制約を `go/build/constraint` で評価して次の両方を満たすファイルである。(1) `test`・`integration` がともに偽のとき、制約に現れるそれ以外のタグをどう真偽に割り当てても、制約が偽になる。(2) `test` または `integration` が真のとき、制約が真になる割り当てがある。どちらにも当たらないファイル（`//go:build unix`・`//go:build linux`・`//go:build windows` など）と、`//go:build` の行がなくファイル名の接尾辞（`_linux.go` など）だけで制約されるファイルは、本番のコードとして走査する。
    -   **失敗にする参照:** プロセスの環境変数を読む標準ライブラリの関数への参照（呼び出しに限らず、関数の値としての参照を含む）を、許可した場所以外では失敗にする。対象を固定の一覧に限定せず、内部で環境変数を読む関数（`os.UserCacheDir` のように呼び出し元に値を返さず内部で読むものを含む）も含める。`os`・`syscall` のドットインポートも失敗にする。`import` の別名を解決して判定し、型を解決できない参照は拒否の側に倒す。
    -   **許可する場所:** ファイルと参照の種類と回数の組で許可する。`internal/config` のすべてのファイルの参照、`cmd/yt2column/main.go` の `os.LookupEnv` の関数の値としての参照 1 か所（設計書 3.1）、`internal/transcript/ytdlp.go` の `os.Environ` 1 か所。
    -   **秘密情報の変数名:** 文字列リテラルが `DEEPSEEK_API_KEY` または `SLACK_WEBHOOK_URL` を含めば、`internal/config` 以外では失敗にする。
    -   **空振りの防止:** `TestEnvAccessConfined` は、走査したファイルの数が 0 でないことと、許可した参照を実際に観測したこと（`ytdlp.go` の `os.Environ`。フェーズ 7 以降は `main.go` の `os.LookupEnv` も）を確かめる。
    -   **自己テスト `TestEnvAccessScannerDetects`:** 走査の関数を、一時ディレクトリに置いた小さなソース（許可の判定のため、リポジトリと同じ相対パスに置く）に当てる。直接読む関数と、内部で環境変数を読む関数（間接に読むもの。少なくとも 1 つ）、別名の import、関数の値としての参照、ドットインポート、秘密情報の変数名の文字列、`//go:build unix`・`//go:build linux`・`//go:build windows` の本番のファイルの参照を、1 つずつ検出すること。許可した場所と同じファイルの別の関数（`ytdlp.go` の `os.Getenv`、`main.go` の `os.Getenv` と `os.LookupEnv` の呼び出し）、同じパッケージの別のファイル（`cmd/yt2column/run.go` の `os.LookupEnv`）を検出すること。`_test.go` と `//go:build test` のファイルの同じ参照、許可した場所の参照を検出しないこと。
-   [x] **ステップ 2-6**: `package_reference.md` に `internal/config` の行を加える。
-   [x] **ステップ 2-7**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: 空の値を未設定と同じに扱う、`YT2COLUMN_LLM_PROVIDER` の比較を大文字と小文字を区別しないものにする、`SLACK_WEBHOOK_URL` の空白の検査を外す、相対パスの検査を外す、最初の誤りで止める、`Reason` に値を含める、API キーを `secret.Secret` でなく `string` のフィールドで持つ（`TestConfigOutputRedactsSecrets` の `%+v`・`%#v`）、`XDG_CACHE_HOME` の空を値ありとして扱う、`GODEBUG` の判定を外す、走査の対象の関数を 1 つずつ外す、テストのコードの判定を (1) だけにする（`//go:build windows` の自己テスト）、許可を `cmd/yt2column` のパッケージ全体に広げる（`run.go` の自己テスト）。`make fmt` → `make test` → `make lint` を通す。

### PR-4 作成ポイント: internal/config

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0005): add internal/config for environment loading and cache directory defaults`

**レビュー観点**: F-001 の検証の規則が表と箇条書きのとおりで、値を補正せず、拒否を `errors.Join` でまとめ、エラーに値を含めないこと（ステップ 2-1〜2-4） / `GODEBUG` の `http2debug` を拒否せず `HTTP2DebugEnabled` で返し、キャッシュディレクトリの既定値を OS と `LookupFunc` から決めること（ステップ 2-2・2-3） / `envaccess_test.go` の走査がテストのコードを正しく除外し、対象の関数（内部で読むものを含む）を検出し、許可した参照を観測して空振りしないこと（ステップ 2-5） / `package_reference.md` の行が公開 API と一致すること（ステップ 2-6）

**実装モデル要件**: standard

**判定理由**: 環境変数の読み込み・検証と、環境変数を読む場所の静的な走査のテストに限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・2 つ以上の Conditional check のいずれにも該当しないため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `internal/llm/provider`

**対象ファイル**
-   新設: `internal/llm/provider/provider.go`・`provider_test.go`
-   変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 3-1**: `provider.go` に `LLMTimeout`・`New`・非公開の `newClient`（プロバイダを直接受け取り、アダプタの構築関数を引数で受け取る）・`errUnknownProvider` を作る（設計書 3.2）。構築のエラーは `%w` で包む。
-   [x] **ステップ 3-2**: `provider_test.go` に次を作る。
    -   `TestNewDeepSeekSendsConfiguredRequest`（AC-10）: `config.Load` で読み込んだ設定と、`deepseek.NewForLoopbackTest` を呼ぶ構築関数で `newClient` を呼ぶ。`httptest` のサーバが受け取った要求のモデル名と `Authorization` ヘッダーが設定の値であること、構築関数が受け取った `Timeout` が `LLMTimeout` であることを確かめる。応答の本文は既存の `testdata/deepseek_chat_completion_stop.json` を使う。
    -   `TestNewUsesDeepSeekAdapter`: 有効な設定で `New` を呼ぶと、エラーがなく、返ったクライアントの動的な型が `internal/llm/deepseek` のものであること（`reflect` の `PkgPath`）。`New` が本番の `deepseek.New` を `newClient` に渡していることを確かめる。要求は送らない。
    -   `TestNewUnknownProvider`（AC-11）: ゼロ値と範囲外の値で、エラーが `errUnknownProvider` を包み、`LLMClient` が nil で、構築関数が呼ばれないこと。
    -   `TestNewPaddedModel`（AC-12）: 前後に空白のある `YT2COLUMN_MODEL` を `config.Load` で読み込み、`New` のエラーが `deepseek.ErrPaddedModel` を包み、`LLMClient` が nil であること。
-   [x] **ステップ 3-3**: `package_reference.md` に `internal/llm/provider` の行を加える。
-   [x] **ステップ 3-4**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `default` で DeepSeek を構築する、`%w` を `%v` にする、モデル名か API キーを別の値にして渡す、`New` が別の構築関数を渡す（`TestNewUsesDeepSeekAdapter`）。`make fmt` → `make test` → `make lint` を通す。

### PR-5 作成ポイント: internal/llm/provider

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4

**推奨タイトル**: `feat(0005): add internal/llm/provider for provider selection`

**レビュー観点**: `New` が `default` で本番の `deepseek.New` を渡し、構築のエラーを `%w` で包み、未知のプロバイダで構築関数を呼ばずに nil を返すこと（ステップ 3-1・3-2） / `TestNewDeepSeekSendsConfiguredRequest` が設定のモデル名・API キー・タイムアウトをループバックの送信先で確かめ、本番の送信先へ要求を送らないこと（ステップ 3-2） / 前後に空白のあるモデル名で `deepseek.ErrPaddedModel` を包み、`LLMClient` が nil であること（ステップ 3-2） / 構築関数の継ぎ目が非公開で本番の `New` だけが本番の値を使うこと（ステップ 3-1・3-2）

**実装モデル要件**: standard

**判定理由**: 設定に応じた構築関数の選択に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・2 つ以上の Conditional check のいずれにも該当しないため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: `FilePublisher`

**対象ファイル**
-   新設: `internal/publisher/file.go`・`file_test.go`・`test_helpers.go`（`//go:build test`）
-   変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 4-1**: `file.go` に `FilePublisher`・`NewFilePublisher`・`Publish`・`ErrOutputExists`・`ErrNoHardLink`・`KeptFileError` を作る（設計書 3.5 の検査・書式・作成の手段 1〜6）。I-03 の継ぎ目（§1.5）を非公開のフィールドに置き、本番の値は `NewFilePublisher` が設定する。`link` の失敗の分類は `errors.Is` で errno を判定する。一時ファイルを `0o644` にする呼び出しには、その行だけに `//nolint:gosec // the article is a non-secret document built from a public video` の形で理由を付ける（設計書 3.5 の 3）。
-   [x] **ステップ 4-2**: `file_test.go` に次を作る。いずれも、出力先のパスと、出力先のディレクトリに残った一時ファイルの有無を確かめる。
    -   `TestFilePublisherWritesArticle`（AC-13）: 内容が設計書 3.5 の書式に一致し、`Body` で終わること。`ModelVersion` が空なら `(none)` を書くこと。パーミッションが `0o644` であること。成功の後に一時ファイルが残らないこと。
    -   `TestFilePublisherExistingPath`（AC-14）: 通常のファイル・ディレクトリ・存在するファイルへのリンク・存在しない先へのリンクの 4 種類で、`ErrOutputExists` を包んだ `*KeptFileError` になり、既存の内容とリンク先が変わらず、存在しない先が作られないこと。`TempPath` のファイルが完成した記事を持つこと。
    -   `TestFilePublisherDirectoryFailure`（AC-15）: ディレクトリがない場合と、書き込めない場合（`requireNonRoot`・`chmodForTest`）。
    -   `TestFilePublisherRejectsArticle`（AC-16）: 要件書 AC-16 のすべての例と、設計書 3.5 で広げた条件（`Title`・`Model` の制御文字、`ModelVersion` の制御文字）で、`ErrInvalidArticle` を包むこと。
    -   `TestFilePublisherCanceled`（AC-17）、`TestNewFilePublisherEmptyPath`（AC-18）。
    -   `TestFilePublisherWriteFailure`（AC-46）: 継ぎ目で一定のバイト数の後に書き込みを失敗させる場合と、書き込みの途中で `ctx` を取り消す場合。一時ファイルが削除されること。
    -   `TestFilePublisherLinkFailure`: 継ぎ目の `link` が `EPERM`・`ENOTSUP`・`EOPNOTSUPP`・`EXDEV`・`EMLINK` を返すと `ErrNoHardLink` を、`EACCES` を返すとどちらの番兵も包まない `*KeptFileError` になり、一時ファイルが残ること。
-   [x] **ステップ 4-3**: `test_helpers.go` に、継ぎ目を差し替えて `FilePublisher` を作る補助と、`requireNonRoot`・`chmodForTest`（§1.3）を置く。
-   [x] **ステップ 4-4**: `package_reference.md` の `internal/publisher` の行に `FilePublisher` を加える。
-   [x] **ステップ 4-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `link` の代わりに `O_CREATE` だけで出力先を直接作る（`TestFilePublisherExistingPath` の存在しない先へのリンク）、書き込みの失敗時に一時ファイルを削除しない（`TestFilePublisherWriteFailure`）、errno の分類から 1 つずつ外す、`CheckPublishable` を呼ばない、`ctx` の確認を外す、パーミッションを変えない。`make fmt` → `make test` → `make lint` を通す。

### PR-6 作成ポイント: FilePublisher

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0005): add FilePublisher with hard-link no-overwrite publishing`

**レビュー観点**: 上書きを `link(2)` で拒否し、errno の分類で `ErrOutputExists`・`ErrNoHardLink`・どちらも包まない `*KeptFileError` を分け、既存の内容とリンク先を変えないこと（ステップ 4-1・4-2） / 記事の書式とパーミッション `0o644` が設計書 3.5 のとおりで、`gosec` の抑止が対象の行だけであること（ステップ 4-1・4-2） / 記事の拒否・`ctx` の確認・失敗時の一時ファイルの削除が行われること（ステップ 4-2） / 継ぎ目が非公開のフィールドでテストから差し替えられること（ステップ 4-1・4-3）

**実装モデル要件**: frontier-recommended

**判定理由**: `gosec` の抑止（ステップ 4-1 の `0o644` の一時ファイル）とビルドタグ下の非 `_test.go` のソース（ステップ 4-3）の 2 つの Conditional check に該当し、加えて上書きを防ぐセキュリティの中核を含むため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: `internal/cachelock`

**対象ファイル**
-   新設: `internal/cachelock/cachelock.go`（`//go:build unix`）・`cachelock_test.go`・`test_helpers.go`（`//go:build test`）
-   変更: `docs/dev/developer_guide/package_reference.md`、本計画（ステップ 5-4 の記録）

**タスク**
-   [x] **ステップ 5-1**: `cachelock.go` に `Lock`・`Acquire`・`File`・`Close`・`ErrLocked`・`LockedError` を作る（設計書 3.7）。排他のファイルは、シンボリックリンクをたどらず、名前付きパイプで待たずに読み取り専用で開き、開いた記述子で通常のファイルであることを確かめる。`gosec` が指摘する変数のパスでの `OpenFile` と、記述子の整数への変換には、その行だけに理由付きの `//nolint:gosec` を付ける。
-   [x] **ステップ 5-2**: `cachelock_test.go` と `test_helpers.go`（`requireNonRoot`・`chmodForTest`）に次を作る。取得を待ちうる呼び出しは、別の goroutine で上限付きで待ち、上限を超えたら失敗として報告する（壊した実装が待ち続けても、`go test` のタイムアウトまで止まらない）。
    -   `TestAcquireCreatesDirectory`: ないディレクトリを `0o700` で作り、既存のディレクトリのパーミッションを変えず、排他のファイルが `0o600` であること。
    -   `TestAcquireLocked`（AC-34）: 同じディレクトリへの 2 つ目の `Acquire` が待たずに `ErrLocked` を包むこと、エラーが `*LockedError` として排他のファイルのパスを `Path` に保持すること（`errors.AsType` で取り出して確かめる。メッセージの文字列一致はしない）。`Close` の後は取得できること。
    -   `TestAcquireOtherDirectory`（AC-35）、`TestAcquireAfterHolderGone`（AC-36。排他のファイルを残したまま、保持者が閉じた後に取得できること）。
    -   `TestAcquireOtherFailures`（AC-47）: 親が通常のファイル、書き込めないディレクトリ（`requireNonRoot`・`chmodForTest`）で、エラーが `ErrLocked` を包まないこと。
    -   `TestAcquireRejectsNonRegularLockFile`: 排他のファイルの名前に、シンボリックリンク・ディレクトリ・名前付きパイプがある場合に拒否すること。名前付きパイプの後始末では書き込み側を開き、壊した実装で止まった呼び出しを解放する。
    -   `TestLockFileSurvivesPrune`（AC-37）: 排他のファイルを置いたディレクトリで `transcript.YtDlpSource.PruneCache` を呼び、エラーがなく、ファイルが残ること。
    -   `TestLockInheritedByChild`（H-02 の仕組み）: `File()` を `ExtraFiles` で渡した子プロセス（`exec.Cmd` で起動する）を起動し、`Close` の後も 2 つ目の `Acquire` が `ErrLocked` になること、子を `Process.Kill` で止めて `Wait` した後に取得できること。起動した時点で、止めて `Wait` する処理を `t.Cleanup` に登録する。
-   [x] **ステップ 5-3**: `package_reference.md` に `internal/cachelock` の行を加える。
-   [x] **ステップ 5-4**（手動、利用者の承認が必要）: 実際の `yt-dlp` が起動する子プロセスが記述子 3 を引き継ぐかを確かめる（設計書 §8 の 5、3.7）。排他のファイルを記述子 3 で開いた状態で実 `yt-dlp` に字幕を取得させ、実行中に `lsof <排他のファイル>` で保持しているプロセスを記録する。結果（`yt-dlp` の版、保持していたプロセス）を本ステップの下に追記する。
    -   実施: 2026-10-06。`yt-dlp` の版: 2026.08.19。検証用の一時プログラムが `cachelock.Acquire` の `Lock.File()` を `transcript.Options.InheritedFiles` に渡し、`https://www.youtube.com/watch?v=2tcCWM-sRBw` の字幕を取得させた。
    -   結果: 実行中に `lsof <排他のファイル>` で、検証用プログラム（`3r`）と `yt-dlp`（COMMAND は `Python`）が排他のファイルを記述子 3 で開いていることを観測した。字幕のみの取得では `yt-dlp` の子プロセス（`deno`・`ffmpeg` など）は現れず、記述子 3 を保持したのは `yt-dlp` 本体だけであった。親の終了後の `lsof` は空で、排他が解放されることを確認した。別の動画（`EQCUZyB4DqE`）でも同じく `Python` が記述子 3 を保持した（この回は HTTP 429 で取得に失敗したが、記述子の引き継ぎは同じ）。
-   [x] **ステップ 5-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `LOCK_NB` を外す（`TestAcquireLocked` が上限で失敗すること）、`flock` を `fcntl` のロックに替える（`TestAcquireLocked`・`TestLockInheritedByChild`）、`EWOULDBLOCK` 以外も `ErrLocked` で包む（`TestLockError`）、通常のファイルの確認を外す、`O_NONBLOCK` を外す、ディレクトリのパーミッションを変える。`make fmt` → `make test` → `make lint` を通す。

### PR-7 作成ポイント: internal/cachelock

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5

**推奨タイトル**: `feat(0005): add internal/cachelock for flock-based exclusion`

**レビュー観点**: `flock` の `LOCK_EX`・`LOCK_NB` で待たずに `ErrLocked` を返し、シンボリックリンク・名前付きパイプ・通常でないファイルを開く前に拒否すること（ステップ 5-1・5-2） / 排他のファイルが `0o600`、作るディレクトリが `0o700` で、既存のディレクトリのパーミッションを変えないこと（ステップ 5-1・5-2） / 記述子 3 の引き継ぎと、`PruneCache` に対して排他のファイルが残ること、保持者の終了後に再取得できることを確かめること（ステップ 5-2） / 実際の `yt-dlp` が記述子 3 を引き継ぐことの手動確認を記録すること（ステップ 5-4）

**実装モデル要件**: frontier-recommended

**判定理由**: `flock` による同時実行の排他（ステップ 5-1・5-2）は独立した高リスクな並行性の制御であり、この PR に隔離してレビューするため。加えて `gosec` の抑止とビルドタグ下の非 `_test.go` のソースという Conditional check に該当するため。ステップ 5-4 の実 `yt-dlp` の確認は設計を決める探索ではなく、設計書 3.7 が安全側に倒れることの確認であるので、`frontier-required` の探索には当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: `internal/job`

**対象ファイル**
-   新設: `internal/transcript/testutil/helpers.go`（`//go:build test || integration`）
-   新設: `internal/job/job.go`・`job_test.go`・`test_helpers.go`（`//go:build test`）
-   変更: `internal/pipeline/pipeline_test.go`（`TestFakesCarryBuildTag` の件数）・`docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 6-1**: `internal/transcript/testutil/helpers.go` に、偽の `yt-dlp` のスクリプトを一時ディレクトリに作る公開関数と、ディレクトリの内容の一覧を取る公開関数を置く。
    -   スクリプトの種類: トリップワイヤ（起動されたら印のファイルを作って失敗する）、途中で止まるもの（自身と子の PID と、記述子 3 が開いているかを記録し、準備完了の印を作って止まる）、指定の内容を標準エラー出力に書いて失敗するもの、`-P` のディレクトリに指定の字幕と info.json を書き出して成功するもの。
    -   印と PID のファイルは、一時ファイルへ書いてから改名して作る。
    -   止まるものは無期限には待たない: 解放の印のファイルが現れるか、数分の上限に達したら、子とともに自分で終了する。作った時点で、解放の印を作る処理を `t.Cleanup` に登録する（PID を指定してシグナルを送る後始末はしない。終了済みの PID が再利用されて無関係のプロセスを止めるのを避けるため）。
    -   `TestFakesCarryBuildTag` の件数を 9 にする。
-   [x] **ステップ 6-2**: `job.go` に `Request`・`Result`・`YtDlpTimeout`・`Run` を作る（設計書 3.4 の手順 B0〜C1、3.6 の事前確認の表）。事前確認で出力先に何かがある場合は `publisher.ErrOutputExists` を包み、親ディレクトリの確認の失敗はそれを包まない。どちらも段階を投稿とする `pipeline.StageError` で返す。
-   [x] **ステップ 6-3**: `job_test.go` に次を作る。`Writer` は `FakeArticleWriter`、`Publisher` は `FilePublisher` を基本とし、キャッシュは `transcript.SeedCacheForTest` で置く。同じテストで `Run` を 2 回以上呼ぶ場合は §4.1 の規則に従う。偽の `yt-dlp` のスクリプトを起動するテストは `t.Parallel` にしない（Linux で書いた直後の実行ファイルの起動が `ETXTBSY` で失敗しうるため）。
    -   `TestRunRemovesCache`（AC-24。別の動画の有効なキャッシュが残ること）、`TestRunKeepCache`（AC-25）、`TestRunFailureKeepsCache`（AC-26。記事の生成の失敗と投稿の失敗）、`TestRunPrunesDangling`（AC-27。後続が失敗する場合を含む）、`TestRunPruneFailureWarns`（AC-28、`requireNonRoot`・`chmodForTest`）、`TestRunRemoveCacheFailureWarns`（AC-29。投稿の成功の後に `ctx` を取り消して `RemoveCache` を失敗させ、`Run` がエラーを返さず `Warnings` を持ち、`--out` が残ること）、`TestRunRefresh`（AC-30。キャッシュと異なる字幕を書き出す偽の `yt-dlp` と、`FakeArticleWriter` が受け取った `Transcript`）。
    -   `TestRunCanceled`（AC-31）: 字幕の取得（途中で止まる `yt-dlp` の準備完了の印を待ってから）・記事の生成・投稿のそれぞれの途中で `ctx` を取り消す。生成と投稿の途中の取り消しは、テストのファイルの中で fake と `FilePublisher` を包む型で起こす。キャッシュが残り、`--out` が作られないこと。
    -   `TestRunInvalidCache`（AC-48）: 途中で切れた字幕と途中で切れた info.json のそれぞれで、`transcript.ErrParseSubtitles`・`ErrParseInfo` を包むエラーになり、続けて `Refresh` と字幕を書き出す偽の `yt-dlp` で成功すること。
    -   `TestRunOutputExists`（AC-50）: 4 種類の既存のパスのそれぞれで、`publisher.ErrOutputExists` を包む投稿の段階の `pipeline.StageError` になり、既存の内容とリンク先が変わらず、存在しない先が作られず、トリップワイヤが起動されず、`Writer` が呼ばれず、キャッシュディレクトリ（ない場合は作られないこと、ある場合は内容の一覧が変わらないこと）が変わらないこと。
    -   `TestRunOutputParentInvalid`（AC-52）: 親がない場合、親が通常のファイルの場合、親の途中の要素が通常ファイルの場合（`ENOTDIR`）、親を探索できない場合（`EACCES`）で、`ErrOutputExists` を包まない投稿の段階の `pipeline.StageError` になり、トリップワイヤ・`Writer`・キャッシュディレクトリについて AC-50 と同じことを確かめる。
    -   `TestRunLocked`（AC-34）: 1 つ目の `Run` が途中で止まる `yt-dlp` で保持している間（準備完了の印を待つ）に、トリップワイヤを指定した 2 つ目の `Run` が待たずに `cachelock.ErrLocked` を包むエラーで戻り、キャッシュディレクトリの内容（2 つ目の開始前の一覧との比較）を変えず、`Writer` を呼ばず、`--out` を作らないこと。1 つ目の `Run` の goroutine は、後始末で取り消して終了を待つ。
    -   `TestRunOtherCacheDir`（AC-35）、`TestRunLockFailure`（AC-47。書き込めない場合は `requireNonRoot`・`chmodForTest`）。
    -   `TestRunPassesLockToYtDlp`: 途中で止まる `yt-dlp` が、記述子 3 が開いていたことを記録すること。
    -   `TestRunValidatesRequest`: B0 の各条件（空の文字列、nil、typed nil）で、キャッシュディレクトリを作らずにエラーになること。
    -   `TestRunReleasesLock`: 成功と失敗のそれぞれの後に、`cachelock.Acquire` が成功すること。
-   [x] **ステップ 6-4**: `test_helpers.go` に `requireNonRoot`・`chmodForTest`（§1.3）と、`Request.OutPath` と `FilePublisher` を同じパスから作る補助を置く。
-   [x] **ステップ 6-5**: `package_reference.md` に `internal/job` の行を加える。`internal/transcript/testutil` の行に偽の `yt-dlp` とディレクトリの一覧の補助を加え、「`-tags test` だけでビルドされる」という記述を、`helpers.go` は `integration` のビルドにも含まれる旨に改める。
-   [x] **ステップ 6-6**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: C1 を `KeepCache` によらず行う・行わない、失敗時にもキャッシュを削除する、B4 の失敗をエラーとして返す、C1 の失敗をエラーとして返す、事前確認を外す（`TestRunOutputExists` のトリップワイヤとキャッシュディレクトリ）、親ディレクトリの確認を外す、排他の取得を外す（`TestRunLocked`）、排他を閉じない（`TestRunReleasesLock`）、`InheritedFiles` を渡さない（`TestRunPassesLockToYtDlp`）、`ForceRefresh` を渡さない（`TestRunRefresh`）。`make fmt` → `make test` → `make lint` を通す。

### PR-8 作成ポイント: internal/job

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6

**推奨タイトル**: `feat(0005): add internal/job to orchestrate one run`

**レビュー観点**: 手順 B0〜C1 の順（事前確認・排他の取得・掃除・パイプライン・キャッシュの削除）と、各失敗の扱いが設計書 3.4 のとおりであること（ステップ 6-2・6-3） / 事前確認で `ErrOutputExists` を包み、親ディレクトリの確認の失敗は包まず、どちらも投稿の段階の `pipeline.StageError` で返すこと（ステップ 6-2・6-3） / 掃除と削除の失敗が警告にとどまり、`--out` とキャッシュの状態が期待どおりであること（ステップ 6-3） / 偽の `yt-dlp` を起動するテストが `t.Parallel` を使わず、`t.Cleanup` を取得時に登録し、複数回の `Run` を §4.1 の規則で扱うこと（ステップ 6-1・6-3）

**実装モデル要件**: frontier-recommended

**判定理由**: 事前確認・排他・掃除・パイプライン・削除の順を守る実行の進め方（ステップ 6-2・6-3）は状態機械に類する独立した最も込み入ったステップであり、この PR に隔離してレビューするため。加えてビルドタグ下の非 `_test.go` のソース、`gosec` の抑止、取得時の `t.Cleanup`、`run`・`Run` の複数回起動という複数の Conditional check に該当するため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: `cmd/yt2column`

**対象ファイル**
-   変更: `cmd/yt2column/main.go`。新設: `run.go`・`outpath.go`・`output.go`
-   新設: `cmd/yt2column/main_test.go`・`run_test.go`・`outpath_test.go`・`output_test.go`・`signal_test.go`・`test_helpers.go`（いずれも `//go:build test`）・`test_helpers_integration.go`（`//go:build test || integration`）
-   変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 7-1**: `output.go` に、エスケープ → 伏せ字化の順に処理する関数を作る（設計書 3.8「標準エラー出力の無害化」）。`output_test.go` の `TestSanitize` で、値と末尾 8 文字（文字単位とバイト単位）の伏せ字化（8 文字以下の値を含む）、制御文字・`U+202E`・不正な UTF-8・バックスラッシュのエスケープ、制御文字を含む秘密情報の値も伏せ字になること、エスケープが合成した値と印に含まれる値を伏せ字にすることを確かめる。
-   [x] **ステップ 7-2**: `outpath.go` に、`--out` がキャッシュディレクトリの中かの判定を作る（設計書 3.6 の性質の箇条書き）。`outpath_test.go` の `TestOutPathInsideCacheDir` で、要件書 AC-20 の `--out` の例すべて（カレントディレクトリをキャッシュディレクトリにする例は `t.Chdir`）、途中のシンボリックリンクの後の `..`、存在しない部分の綴りの大小だけが違うパス、キャッシュディレクトリがない場合、キャッシュディレクトリへのシンボリックリンク、`Lstat` が権限で失敗する場合（`requireNonRoot`・`chmodForTest`）を「中」と、外を指すパスを「外」と判定することを確かめる。設計書 3.6 の「安全側に倒すことによる誤判定」の例も「中」と判定されることを確かめる。PR-9 のテスト（`TestSanitize`・`TestOutPathInsideCacheDir`）を 1 つずつ壊して失敗することを確かめ、コミットメッセージに記録し、`make fmt` → `make test` → `make lint` を通す。

### PR-9 作成ポイント: CLI pre-side-effect validation (stderr sanitization and --out path check)

**対象ステップ**: 7-1 / 7-2

**推奨タイトル**: `feat(0005): sanitize stderr and validate the --out path for the CLI`

**レビュー観点**: エスケープ → 伏せ字化の順で、値と末尾 8 文字・制御文字・`U+202E`・不正な UTF-8・バックスラッシュを無害化し、制御文字を含む秘密情報の値も伏せ字にすること（ステップ 7-1） / `--out` の判定がキャッシュディレクトリの中を安全側に倒して「中」とし、外を指すパスを「外」とし、途中のシンボリックリンクの後の `..` や綴りの大小の違いを誤って「外」としないこと（ステップ 7-2） / `TestOutPathInsideCacheDir` が AC-20 の例すべてと権限の失敗・誤判定の例を網羅すること（ステップ 7-2） / 無害化と判定が `run`・`main` から独立し、この PR だけでグリーンゲートを通すこと（ステップ 7-1・7-2）

**実装モデル要件**: frontier-recommended

**判定理由**: 秘密情報と制御文字の無害化（ステップ 7-1）はセキュリティの中核であり、`run`・`main`（ステップ 7-3 以降）から独立してこの PR に隔離してレビューするため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

-   [x] **ステップ 7-3**: `run.go` に `deps`・`productionDeps`・`run` を作る（設計書 3.8 の手順 A1〜C、実行経路の一覧、メッセージの形、3.3 のタイムアウトの文言）。フラグの定義は、`run` と文書のテスト（ステップ 9-4）が同じものを使えるよう、`flag.FlagSet` を作る非公開の関数にまとめる。`productionDeps` の `newPublisher` は、構築の失敗時に typed nil ではなく nil の interface を返す。
-   [x] **ステップ 7-4**: `main.go` を次の 3 つに分ける。
    -   起動時の `signal.Ignored` の結果から購読するシグナルを選ぶ非公開の関数（SIGTERM と、それぞれ無視されていなければ SIGINT・SIGHUP。設計書 3.8 の SIGHUP・SIGINT の項）。
    -   その結果でシグナルを購読し、最初のシグナルで購読を止めてから `context` を取り消し、その間に届いた 2 回目のシグナルを送り直し、`run` を呼んで終了コードを返す関数（設計書 3.8 の `main` の項）。シグナルが来ないまま `run` が戻った場合も購読を止める。引数は、コマンドラインの引数・`config.LookupFunc`・標準出力・標準エラー出力・`deps` とする。
    -   `os.LookupEnv` と `productionDeps()` を渡して上の関数を呼び、その戻り値で `os.Exit` する `main`。環境変数の参照は、この `os.LookupEnv` の 1 か所だけにする。
-   [x] **ステップ 7-5**: `main_test.go` に `TestMain` を作る。
    -   proxy の環境変数を、開いてすぐ閉じたループバックのリスナーのアドレスに向け、`http.ProxyFromEnvironment` が本番の送信先についてそのアドレスを返すことを確かめてから `m.Run` を呼ぶ（設計書 7.1）。
    -   I-02 の子プロセスのモードは、`m.Run` より前に判定する。子プロセスは、テストで確かめるシグナルの購読の経路を、fake の `LLMClient` を返す `deps` で実行し、標準出力にはそれ以外を書かずに `os.Exit` する。fake の動作（`ctx` の終了まで止まる、`ctx` を無視して止まり続ける）と準備完了の印は、テストの補助（ステップ 7-6）が担う。加えて、本番の `main` をそのまま呼ぶモードを設ける。LLM を呼ぶ前に止まる `yt-dlp` の実行中のテスト（`TestSignalDuringYtDlp`・`TestSIGKILLWhileYtDlpRuns`）はこのモードで実行する（本番の `deepseek.New` は構築するが、要求は送らない）。成功する fake は、子プロセスのテストで使う経路がないので設けない。
    -   **検証事項（AC-44・AC-45）:** 本番の `main` が、テストしたシグナルの購読の関数を介して `run` を呼ぶことを確かめる。`main` から購読を外すと対応するテストが失敗すること。`TestSignalDuringYtDlp` が上の `main` のモードで子プロセスを起動するので、`main` の購読を外すと同テストが失敗する（ステップ 7-10 で確かめた）。
-   [x] **ステップ 7-6**: 補助を置く。
    -   `test_helpers_integration.go`（`test || integration`）: 環境の対応表から `config.LookupFunc` を作る関数、CLI の統合テストの `deepseektestutil.IntegrationOptions`（`CLIOptInEnv`・`test-integration-cli`・`MissingKeyFail`）を 1 か所で定義した変数、`Generate` の回数を数える型（包む対象のクライアントが nil なら nil の interface を返す）。このファイルはステップ 7-6 で、`IntegrationOptions` の変数を除いて作り、変数はステップ 8-6 で足す（`deepseektestutil` がフェーズ 8 でできるため）。
    -   `test_helpers.go`（`test`）: fake を返す `deps` を作る補助、子プロセスを起動する補助、`requireNonRoot`・`chmodForTest`。子プロセスの環境は、`PATH`・`HOME`・`TMPDIR` と `TestMain` が設定した proxy の変数だけを親から引き継ぐ allowlist（既存の `makeChildEnvAllowlist` と同じ考え方）で作り、`YT2COLUMN_*`・秘密情報・モードの変数はテストが明示して足す。
-   [x] **ステップ 7-7**: `run_test.go` に次を作る。同じテストで `run` を 2 回以上呼ぶ場合は §4.1 の規則に従う。
    -   `TestRunExecutionPaths`（AC-44・AC-38・AC-20・AC-23・AC-19・AC-28・AC-29・AC-31・AC-34・AC-47・AC-48・AC-50・AC-52 の CLI の部分）: 実行経路の一覧のうち、シグナルを除くすべての経路を 1 つの表で実行する。行ごとに、終了コード、標準出力の期待（`-h`・`--help` の行は使い方が書かれること、それ以外の行は空であること（AC-44 (b)））、標準エラー出力に含むべき文字列（段階の名前、`yt2column -h` の案内、`--refresh` の案内、排他のファイルのパスと `yt-dlp` が残っている可能性、一時ファイルのパス、タイムアウトの種類と分）、AC-44 (e)・(f) の副作用（排他のファイル・キャッシュディレクトリの内容の一覧・トリップワイヤ・fake の `LLMClient` の呼び出しの回数・`--out`・その動画のキャッシュ）を表の項目として持つ。
        -   AC-20 の入力はすべて「引数の誤り」「環境変数の誤り」「`LLMClient` の構築の失敗」「テンプレートの構築の失敗」の行にする。環境変数の誤りの行は、標準エラー出力に変数の名前が現れ、どの変数の値に置いた目印も現れないことを確かめる。`YT2COLUMN_MODEL` の前後の空白の行だけは `productionDeps()` の `newLLMClient` を使う。
        -   すべての行で `DEEPSEEK_API_KEY` と `SLACK_WEBHOOK_URL` に特徴的な値を設定し、標準出力・標準エラー出力・`--out` のファイルに、値も末尾 8 文字も現れないことを確かめる（AC-38）。記事の生成の失敗の行には、fake の `LLMClient` のエラーのメッセージに API キーを含むものを入れる。
        -   中断の行（AC-31 の CLI の部分）: 記事の生成と投稿のそれぞれの途中で、テストが `run` に渡した `ctx` を取り消し、終了コード `1` になること。
        -   警告の行（AC-28・AC-29）: 掃除の失敗（`requireNonRoot`・`chmodForTest`）と、投稿の後の `ctx` の取り消しによる削除の失敗で、終了コード `0`、警告が標準エラー出力にあり、`--out` が残ること。
        -   同時実行の行（AC-34）: 別の goroutine で `run` を実行し、途中で止まる `yt-dlp` が排他を保持している間（準備完了の印を待つ）に、同じキャッシュディレクトリでトリップワイヤを指定した `run` を呼ぶ。この `run` が終了コード `1` で終わり、排他のファイルのパスを含むメッセージを書くこと。最初の `run` は後始末で取り消して終了を待つ。
        -   一時ファイルの行: fake の `LLMClient` が `Generate` の中で `--out` を作り、`link` が `EEXIST` で失敗する。タイムアウトの行: 途中で止まる `yt-dlp` の実行中に、短い期限の `ctx` で期限を切らす。
    -   `TestRunHelp`（AC-21・AC-44 のヘルプの経路）: `TestRunExecutionPaths` のヘルプの行より詳しく、使い方の内容を確かめる。要件書 F-005 の表の 6 つのフラグが使い方に現れること、環境変数が空でも同じであること、使い方が標準出力にだけ書かれること。
    -   `TestRunPromptOverrides`（AC-22）。
    -   `TestRunEscapesUntrustedText`（AC-39）: 要件書 AC-39 の 3 つの経路で、`\x1b[2J` と偽の行を始める改行がそのままの形で現れないこと。成功の経路は fake の `Publisher` を使う（`FilePublisher` は制御文字を含む `Model` を拒否するため）。
    -   `TestRunGODEBUGWarning`（AC-51）: 警告の有無、警告が LLM の呼び出しより前に書かれること（fake の `LLMClient` が呼ばれた時点の標準エラー出力の内容で確かめる）、警告が API キーの値も末尾 8 文字も含まないこと、終了コードが設定しない場合と同じであること。
-   [x] **ステップ 7-8**: `signal_test.go` に、子プロセスを使うテストを作る（I-02）。シグナルは、準備完了の印を上限付きで待ってから送る。起動した子プロセスは、起動した時点で止めて `Wait` する処理を `t.Cleanup` に登録する。偽の `yt-dlp` は、ステップ 6-1 の解放の印で後始末する。PID のプロセスがないことの確認は、`kill(pid, 0)` が `ESRCH` を返すまで上限付きで待つ（親が終了した孤児は init が回収するまで少し残るため）。
    -   `TestSignalDuringYtDlp`（AC-45・AC-44 の SIGINT・SIGTERM の経路）: 途中で止まる `yt-dlp` の実行中に SIGINT・SIGTERM を送り、終了コード `1`、AC-44 (b)〜(d)・(f)、記録したすべての PID のプロセスがないことを確かめる。
    -   `TestSignalDuringGenerate`（AC-44 の SIGINT・SIGTERM の経路）: キャッシュを置いて fake の `LLMClient` で止めた状態でシグナルを送り、終了コード `1`、その動画のキャッシュが残り、`--out` が作られないことを確かめる。
    -   `TestSIGKILLWhileYtDlpRuns`（AC-49）: CLI の子プロセスを SIGKILL で止めた後も、偽の `yt-dlp` は動き続けている。この間に同じキャッシュディレクトリで `run` を実行し、終了コード `1` で終わること、トリップワイヤを起動せず、fake の `LLMClient` を呼ばず、`--out` を作らず、キャッシュディレクトリの内容を変えないこと、`yt-dlp` が残っている可能性と対処の方法を書くことを確かめる。続けて偽の `yt-dlp` を解放の印で終了させ（`yt-dlp` の子を含めて記述子 3 が閉じる）、`cachelock.Acquire` が成功するまで上限付きで待ってから、成功する実行を確かめる。
    -   `TestSIGKILLWithoutYtDlp`（AC-36）: キャッシュがあり、fake の `LLMClient` で止まっている子プロセスを SIGKILL で止めた後、同じキャッシュディレクトリでの実行が成功することを確かめる。
    -   `TestSubscribedSignals`（同じプロセスの中のテスト）: 購読するシグナルを選ぶ関数が、SIGINT・SIGHUP のそれぞれについて、無視されていなければ含み、無視されていれば含まないこと、SIGTERM を常に含むこと。この関数は無視されているかの判定を引数で受け取り（本番は `signal.Ignored`）、テストは両方の結果を与える（テストのプロセスのシグナルの扱いを変えないため）。`TestSIGHUP`（子プロセス）: 無視されていない状態で起動した子に SIGHUP を送ると終了コード `1` になること。テストのプロセス自身が SIGHUP を無視した状態で起動された場合（`nohup` の下など）は子も無視を引き継ぐので、その旨を示してスキップする。
    -   `TestIgnoredSignalNotSubscribed`（子プロセス）: SIGINT・SIGHUP のそれぞれを無視させた状態で（`/bin/sh` の `trap '' <名前>` の後に `exec` して）起動した子に、そのシグナルを送っても、一定の時間のうちに子が終了しないこと。誤って購読した場合だけがこの時間のうちに子を終了させるので、遅い環境でも誤って失敗しない。
    -   `TestSecondSignal`: SIGTERM で確かめる（テストのプロセスが SIGINT を無視した状態で起動されても実行できるように）。`ctx` を無視する fake で止め、1 回目のシグナルの後に「`ctx` が終わった」印を待ち、2 回目のシグナルを、子がシグナルで終了するまで上限付きで繰り返し送る。終了状態がシグナルによる終了であることを確かめる。
    -   子プロセスの標準エラー出力は、網羅率の計測（`make test-ci`）で警告が加わりうるので、空であることは確かめず、含む・含まない文字列で確かめる。
-   [x] **ステップ 7-9**: `package_reference.md` に `cmd/yt2column` の行を加える。`TestEnvAccessConfined`（ステップ 2-5）に、`cmd/yt2column/main.go` の `os.LookupEnv` の参照を観測したことの確認を加える（ファイルの有無で確認を切り替えない）。
-   [x] **ステップ 7-10**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: シグナルの購読を外す・SIGTERM だけ外す（`TestSignalDuringYtDlp`）、プロセスグループの停止を外す（同、孫の PID）、`InheritedFiles` を渡さない（`TestSIGKILLWhileYtDlpRuns`）、A5 を `job.Run` の後に移す（`TestRunExecutionPaths` のトリップワイヤ）、`GODEBUG` の警告を外す・LLM の呼び出しの後に移す（`TestRunGODEBUGWarning`）、使い方を標準エラー出力に書く（`TestRunHelp`）、SIGHUP・SIGINT を `signal.Ignored` によらず購読する（`TestSubscribedSignals`・`TestIgnoredSignalNotSubscribed`）、2 回目のシグナルで購読を止めない（`TestSecondSignal`）。`make fmt` → `make test` → `make lint` を通す。

### PR-10 作成ポイント: cmd/yt2column CLI wiring and run

**対象ステップ**: 7-3 / 7-4 / 7-5 / 7-6 / 7-7 / 7-8 / 7-9 / 7-10

**推奨タイトル**: `feat(0005): assemble the cmd/yt2column CLI`

**レビュー観点**: 実行経路の一覧の終了コードと、段階の名前・`yt2column -h` と `--refresh` の案内・一時ファイルのパス・タイムアウトの文言が設計書 3.8 のとおりであること（ステップ 7-3・7-7） / `main` がテストしたシグナルの購読の関数を介して `run` を呼び、子プロセスの停止と SIGKILL 後の残存が AC-45・AC-49 のとおりであること（ステップ 7-4・7-5・7-8） / 子プロセスの環境が allowlist で作られ、偽の `yt-dlp` と PID の後始末が取得時に登録されること（ステップ 7-6・7-8） / 別プロセスのシグナルのテストが準備完了の印を待ち、上限付きで PID の消滅を確かめること（ステップ 7-8）

**実装モデル要件**: frontier-required

**判定理由**: 本番の CLI を別プロセスで起動し実シグナルを送る重い統合テストの面（ステップ 7-5・7-8）というパネルモードのトリガーに該当するため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 8: 統合テストの実行条件の移動と統合テスト

**対象ファイル**
-   新設: `internal/llm/deepseek/testutil/integration.go`（`//go:build test || integration`）・`make.go`（`//go:build test`）・`integration_settings_test.go`（`//go:build test`）
-   変更: `internal/llm/deepseek/integration_env_test.go`・`integration_test.go`・`deepseek_test.go`・`makefile_test.go`
-   新設: `cmd/yt2column/integration_test.go`（`//go:build integration`）・`cmd/yt2column/makefile_test.go`（`//go:build test`）。変更: `cmd/yt2column/test_helpers_integration.go`
-   変更: `Makefile`・`internal/pipeline/pipeline_test.go`（件数）・`docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 8-1**: `integration.go` に、設計書 3.11 の型と `SettingsFrom`、§1.4 の公開の定数を作る（`package deepseektestutil`）。判定の順序と内容は既存の `integrationSettingsFrom` と同じとし、キーがない場合だけ `MissingKey` に従う。スキップと失敗の理由は `IntegrationOptions` のオプトインの変数名と make のターゲットを示す。
-   [x] **ステップ 8-2**: `make.go` に、`makefile_test.go` の `make` の実行の補助と `deepseek_test.go` の `firstLineIs` を移す（§1.4）。移した補助（`RunMakeTarget`）は、リポジトリの根のパスとターゲットを引数で受け取り、記録する環境変数（2 つのオプトインとモデル名）は `make.go` の 1 か所で定める（設計書 3.11）。2 つのターゲットに共通する確認は `CheckChargedTarget` にまとめ、`TestMakeTestIntegrationDeepSeek` と `TestMakeTestIntegrationCLI` が使う。`make.go` を使うのは `test` のタグのテストだけなので、ビルドタグは `test` とする。`FirstLineIs` の不一致のエラーは静的なエラーを `%w` で包む。偽の `GOTEST` のスクリプトは記録する変数の名前を埋め込むので、名前がシェルの変数名の形でなければ拒否し、その判定を `make_test.go` の `TestValidateEnvNames` で確かめる。
-   [x] **ステップ 8-3**: `TestIntegrationSettings` のケースを、すべての確認（`DEEPSEEK_API_KEY` を読まないこと、理由に API キーが現れないこと、実行しない場合にモデル名とキーを持たないこと）とともに `integration_settings_test.go` の `TestSettingsFrom` に移し、`MissingKeySkip` で同じ結果になることを確かめる。`MissingKeyFail` でキーがない場合に失敗になること、`IntegrationOptions` のゼロ値の `MissingKey` が失敗であること、理由にオプトインの変数名と make のターゲットが現れること、理由に API キーの末尾 8 文字も現れないことを加える。`deepseek_test.go` から `TestIntegrationSettings` を削除する。
-   [x] **ステップ 8-4**: `internal/llm/deepseek` を追従させる。`integration_env_test.go` は判定の部分を削除してタイムアウトの定数だけを残す。`integration_test.go` は `SettingsFrom` を `MissingKeySkip` で呼ぶ。`makefile_test.go` は移した補助を使い、`TestMakeOptInExportedToDeepSeekTargetOnly` は `make test-integration` が 2 つのオプトインのどちらもエクスポートしないことを確かめる。`TestIntegrationTestBuildTag` は `FirstLineIs` を使う。DeepSeek の統合テストが渡す `IntegrationOptions` は `integration_env_test.go` に変数として置き（`makefile_test.go` と `integration_test.go` が共有する）、`TestIntegrationOptionsSkipMissingKey` で、この変数がキーのない場合にスキップすること（移動の前と同じ振る舞い）を固定する。
-   [x] **ステップ 8-5**: `Makefile` に `test-integration-cli` を足す（設計書 3.11）。`YT2COLUMN_CLI_INTEGRATION=1` と、未定義のときだけ `deepseek-flash` にする `YT2COLUMN_MODEL` をこのターゲットのレシピにだけエクスポートし、`-timeout` は 20 分、実行前に料金が発生することを表示する。`.PHONY` と、`go vet -tags integration` の行の上のコメントにターゲットを加える。
-   [x] **ステップ 8-6**: `cmd/yt2column/test_helpers_integration.go` に、ステップ 7-6 の CLI の統合テストの `IntegrationOptions` の変数と、その変数で `SettingsFrom` を呼び、実行の場合だけ本体の関数を呼び、それ以外は理由で `Skip`・`Fatal` する関数 `gateCLIIntegration` を足す（統合テストはこの関数を通して判定に従う）。`cmd/yt2column/makefile_test.go` に次を作る。
    -   `TestMakeTestIntegrationCLI`: 引数、`-timeout` が `provider.LLMTimeout` より長いこと、オプトインの値、モデル名の 3 つの場合（既存の `TestMakeTestIntegrationDeepSeek` と同じ）。
    -   `TestMakeOptInsAreTargetSpecific`: `test-integration-cli` が DeepSeek のオプトインを、`test-integration-deepseek` が CLI のオプトインをエクスポートしないこと。
    -   `TestCLIIntegrationSettings`（AC-40）: CLI の統合テストの `IntegrationOptions` の変数についての AC-40 の環境ごとの判定（スキップ・失敗）と、理由に API キーもその末尾 8 文字も現れないことを確かめる。入力の詳細は実装で決める。
    -   **検証事項（AC-40）:** CLI の統合テストが `SettingsFrom` の結果に従って動くこと。スキップと失敗のときは `run` も LLM のクライアントも呼ばないこと。`SettingsFrom` のユニットテストだけではこの経路を満たさない。`TestCLIIntegrationSettings` は、`Skip`・`Fatal` を記録して終了しない `testing.TB` の包みで `gateCLIIntegration` を呼び、スキップと失敗の環境で本体（`run` と LLM のクライアントを呼ぶ部分）が呼ばれないことを確かめる。`TestIntegrationCLI` が `gateCLIIntegration` を通ることは `integration` のタグのビルドにだけあるので自動では確かめず、オプトインなし・キーなしで実行してスキップ・失敗することを手元で確かめる（コミットメッセージに記録）。
    -   `TestCLIIntegrationTestBuildTag`（AC-42）: `integration_test.go` の 1 行目が `//go:build integration` であること。
-   [x] **ステップ 8-7**: `cmd/yt2column/integration_test.go` に `TestIntegrationCLI` を作る（設計書 3.11 のテストの組み立て）。`gateCLIIntegration(t, os.Getenv, <本体>)` を通して `SettingsFrom(os.Getenv, <ステップ 8-6 の変数>)` を呼ぶ。I-04（§1.5）のとおり LLM のクライアントを包み、`run` を 1 回だけ呼ぶ。AC-41 の各項目を確かめる。API キーとその末尾 8 文字が標準出力・標準エラー出力・`--out` のファイルに現れないことを確かめるまでは、それらをテストの出力に書かない。失敗のメッセージは場所の名前だけを示し、内容を含めない。
-   [x] **ステップ 8-8**: `TestFakesCarryBuildTag` の件数を、`internal/llm/deepseek/testutil/` の 4 件（`integration.go`・`make.go`・`integration_settings_test.go`・`make_test.go`）を加えた 13 にする。`package_reference.md` に `internal/llm/deepseek/testutil` の行を加える。
-   [x] **ステップ 8-9**: テストの削除の確認（[CLAUDE.md](../../../CLAUDE.md)「Deleting a test」）。移動の前後で `go test -tags test -coverprofile` を `./internal/llm/deepseek/...` に対して取り、`go tool cover -func` の結果を関数ごとに比べる。`integrationSettingsFrom` は `_test.go` にあったので移動の前の結果に現れない。`internal/llm/deepseek` の本番の関数の行が変わらないこと、`SettingsFrom` の網羅率（到達しない `secret.New` の失敗の分岐を除いてすべての文）、ステップ 8-3 で移した確認がすべて残っていることを、コミットメッセージに記録する。
-   [x] **ステップ 8-10**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `MissingKey` を無視して常にスキップする（`TestSettingsFrom`）、CLI の `IntegrationOptions` の変数を `MissingKeySkip` か DeepSeek のオプトインにする（`TestCLIIntegrationSettings`）、オプトインをグローバルにエクスポートする（`TestMakeOptInsAreTargetSpecific`・`TestMakeOptInExportedToDeepSeekTargetOnly`）、`-timeout` を 15 分以下にする（`TestMakeTestIntegrationCLI`）、統合テストのビルドタグを外す（`TestCLIIntegrationTestBuildTag`）。次の名前が `docs/tasks/` 以外に残っていないことを検索で確かめる: `integrationSettingsFrom`・`integrationSkip`・`integrationFail`・`integrationRun`・`runMakeTarget`・`makeChildEnvAllowlist`。`make fmt` → `make test` → `make lint` を通す。
-   [x] **ステップ 8-11**（利用者の承認が必要）: `make test-integration-cli` を実行し、成功することと、出力に API キーとその末尾 8 文字が現れないことを確かめる。結果（日付、モデル、成否）を本ステップの下に追記する。
    -   実行の記録（2026-10-07、モデル `deepseek-flash`）: 1 回目（2026-10-06）は DeepSeek のアカウントのクレジットが尽きていたため、`run` が終了コード `1` で失敗した（テストの出力に API キーとその末尾 8 文字は現れなかった）。クレジットを補充した後の 2 回目は成功した（`TestIntegrationCLI` PASS、16.88 秒）。両方の出力に API キーとその末尾 8 文字が現れないことを、出力の全文を検索して確かめた。

### PR-11 作成ポイント: integration test move and CLI integration test

**対象ステップ**: 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8 / 8-9 / 8-10 / 8-11

**推奨タイトル**: `feat(0005): move the DeepSeek integration settings and add the CLI integration test`

**レビュー観点**: `SettingsFrom` が既存の判定と同じ順序と内容で、キーがない場合だけ `MissingKey` に従い、理由に API キーの値も末尾 8 文字も含めないこと（ステップ 8-1・8-3） / 移動の前後で `go tool cover -func` の本番の関数の結果が変わらず、移した確認がすべて残っていること（ステップ 8-9） / `make test-integration-cli` が CLI のオプトインとモデルだけをエクスポートし、`-timeout` を 20 分にして料金の発生を表示すること（ステップ 8-5） / 統合テストのビルドタグと `TestFakesCarryBuildTag` の件数が正しいこと（ステップ 8-7・8-8）

**実装モデル要件**: frontier-required

**判定理由**: 実 DeepSeek API を呼び料金が発生する外部リソースの面と、`make test-integration-cli` による CI の面（ステップ 8-5・8-7・8-11）というパネルモードのトリガーに該当し、加えて環境変数によるスキップの判定（ステップ 8-1）という Conditional check に該当するため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 9: 文書と手動確認

**対象ファイル**
-   変更: `README.md`・`docs/dev/project_overview.md`・`docs/dev/security.md`・`docs/dev/developer_guide/package_reference.md`
-   新設: `cmd/yt2column/docs_test.go`（`//go:build test`）。変更: `internal/pipeline/pipeline_test.go`
-   変更: 本計画（ステップ 9-5・9-6 の記録）

**タスク**
-   [x] **ステップ 9-1**: `README.md` に、要件書 F-010 の README の項目すべてと、記事のファイルのパーミッション `0o644`、出力先のディレクトリにハードリンクを作れる必要があること（設計書 3.5・3.12）を書く。終了コードは見出し付きの表にする。`Configuration` の表に未設定のときの値の列を加える。`Development` の一覧と統合テストの節に `make test-integration-cli` を加える。
-   [x] **ステップ 9-2**: `project_overview.md` の「設定（環境変数）」の表に、F-001 の未設定のときの値を反映する。
-   [x] **ステップ 9-3**: `security.md` §2 に、CLI は `GODEBUG` の `http2debug` を拒否せず警告にとどめることを書く。実在の API キーを使う統合テストの例外に `cmd/yt2column/integration_test.go` と `make test-integration-cli`・`YT2COLUMN_CLI_INTEGRATION` を加えることは、課金の経路を加える PR-11 で行った（PR-11 のレビューで、§2 が課金の経路を 1 つとする記述のままでは事実と食い違うと指摘されたため）。
-   [x] **ステップ 9-4**: 文書の記載を確かめるテストを作る（AC-43・AC-32・AC-33 の `static`）。期待する値は要件書（F-001 の表の未設定のときの値など）から取り、テストの対象のコードから導かない。
    -   `cmd/yt2column/docs_test.go::TestREADMEDocumentsCLI`: README の機械的に確かめられる部分。対象は、呼び出し形式（`yt2column [flags] <動画 URL>` の形）、`run` の `flag.FlagSet` のすべてのフラグ、終了コードの表の `0`・`1`・`2` の行、`Configuration` の表の要件書 F-001 の 6 つの変数の行と、各行の未設定のときの値が F-001 の表の値と一致することである。振る舞いを述べる散文（同時実行・SIGKILL・`--refresh` の案内、パーミッション `0o644`、ハードリンク、統合テストの実行方法）は、文字列一致では意味を保証できないので固定しない（ステップ 9-7 で実装と突き合わせる）。
    -   `cmd/yt2column/docs_test.go::TestProjectOverviewDocumentsConfig`: `project_overview.md` の「設定（環境変数）」の表に 6 つの変数の行があり、各行の未設定のときの値が F-001 の表の値と一致すること。
    -   `security.md` §2 の記載（`http2debug` の警告、統合テストの例外）は、文字列一致では意味を保証できないのでテストで固定せず、ステップ 9-7 で実装と突き合わせる。
    -   `cmd/yt2column/docs_test.go::TestPlanRecordsManualRuns`（AC-32・AC-33）: 本計画にステップ 9-5・9-6 が完了条件として存在し、チェック済み（`[x]`）のステップには、使用した動画 URL と結果の記録があること。実際の実行が行われたことそのものは確かめられないので、ステップ 9-5・9-6 の `manual` で補う。
    -   `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`: `cmd/`・`internal/` の下の、本番のコード（`_test.go` でなく、テスト用のタグだけでビルドされるのでもない `.go`。`//go:build unix` の `internal/cachelock` を含む）を持つディレクトリ、`testutil/` のディレクトリ、`prompts` が、`package_reference.md` の表の行になっていること（リポジトリ全体のガードなので、既存のガードと同じファイルに置く）。
-   [x] **ステップ 9-5**（AC-32、手動、利用者の承認が必要）: 日本語字幕のある実際の動画 1 本で CLI を実行し、記事が `--out` のファイルに書き出されることを確かめる。使用した動画 URL と結果（終了コード、出力のファイルの見出し、`Model`）を本ステップの下に記録する。
    -   実施: 2026-10-07。動画 URL: `https://www.youtube.com/watch?v=2tcCWM-sRBw`。結果: 終了コード `0`。`--out` のファイル（パーミッション `0o644`）の見出しは「ことばを越えて届くもの——ネパール人留学生たちの日本語スピーチコンテスト」、`Model` は `deepseek-flash`、`ModelVersion` は `aeb56401ca74e127821c4f9126dcb669`。標準出力・標準エラー出力・記事のいずれにも API キーの値と末尾 8 文字は現れなかった。
-   [x] **ステップ 9-6**（AC-33、手動、利用者の承認が必要）: ステップ 9-5 と同じ動画に `--keep-cache` を付けて 1 回実行してキャッシュを残した後、`--refresh` を付けて実行し、`yt-dlp` が再実行されたことを、キャッシュのポインタが指すスロットの切り替わり（`a` ↔ `b`）で確かめる。各実行の `--out` には、まだ存在しない別のパスを指定する。使用した動画 URL と結果を本ステップの下に記録する。
    -   実施: 2026-10-07。動画 URL: `https://www.youtube.com/watch?v=2tcCWM-sRBw`。1 回目（`--keep-cache`、`--out` は未存在パス）の終了コードは `0` で、ポインタ（`2tcCWM-sRBw.current`）は `a`。2 回目（`--refresh --keep-cache`、別の未存在パス）の終了コードも `0` で、ポインタは `b` に切り替わり、`yt-dlp` が再実行されたことを確認した。結果の記事の見出し・`Model` は 1 回目と同じ。
-   [x] **ステップ 9-7**: 文書の内容を実装と突き合わせて読む。突き合わせる先: フラグの定義（`run.go`）、未設定のときの値（`internal/config` の `cachedir.go` と `Load`）、終了コード（設計書 3.8 の実行経路の一覧）、同時実行・SIGKILL・`--refresh` の説明（`internal/cachelock`・`internal/job` の振る舞いとステップ 7-8 のテスト）、README のパーミッション `0o644` とハードリンク要件（`internal/publisher/file.go`）、README の統合テストの実行方法（`Makefile` の `make test-integration-cli`）、`security.md` §2 の `http2debug` の警告（`cmd/yt2column/run.go` の警告）と統合テストの例外（`cmd/yt2column/integration_test.go`・`make test-integration-cli`・`YT2COLUMN_CLI_INTEGRATION`）、`Makefile` のターゲット。自動のテストで固定している項目（README のフラグ・終了コード・未設定のときの値、`package_reference.md` の行、チェック済みのステップ 9-5 の記録）は壊して失敗することを確かめ（README からフラグを 1 つ消す、終了コードの表の行を消す、README と `project_overview.md` の未設定のときの値を 1 つ書き換える、`package_reference.md` から行を 1 つ消す、チェック済みにしたステップ 9-5 の記録を消す）、散文の項目は突き合わせて読むことで確かめる。結果をコミットメッセージに記録する。`make fmt` → `make test` → `make lint` を通す。

### PR-12 作成ポイント: documentation and manual verification

**対象ステップ**: 9-1 / 9-2 / 9-3 / 9-4 / 9-5 / 9-6 / 9-7

**推奨タイトル**: `docs(0005): document the CLI and verify it against the implementation`

**レビュー観点**: README・`project_overview.md`・`security.md` が F-010 の項目と F-001 の未設定のときの値を反映し、文書のテストが要件書の値を期待値にして実装から導かないこと（ステップ 9-1〜9-4） / `package_reference.md` が `cmd/`・`internal/` の本番のコードを持つすべてのパッケージと `testutil/`・`prompts` の行を持ち、`TestPackageReferenceListsPackages` がそれを固定すること（ステップ 9-4） / 手動の実行（ステップ 9-5・9-6）の URL と結果、文書の突き合わせ（ステップ 9-7）の記録が本計画にあること（AC-32・AC-33）

**実装モデル要件**: standard

**判定理由**: 文書の更新・文書のテスト・手動確認に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・2 つ以上の Conditional check のいずれにも該当しないため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | フェーズ | 成果物 | 完了の判定 |
|---|---|---|---|
| M1: 既存パッケージの準備 | 1 | `ValidateVideoURL`・`ErrPaddedModel`・`NewForLoopbackTest`・`InheritedFiles`・プロセスグループの停止・`SeedCacheForTest`・`CheckPublishable`・ビルドタグの規則の例外とガード | 既存のテストが §1.4 の追従と §1.3 の部分テストのほかは変更なしで通り、新しいテストが通る |
| M2: 設定とプロバイダ | 2・3 | `internal/config`・`internal/llm/provider` | AC-01〜AC-12 と AC-51 の設定の部分のテストが通る |
| M3: 投稿と排他 | 4・5 | `FilePublisher`・`internal/cachelock`、ステップ 5-4 の記録 | AC-13〜AC-18・AC-46 と、AC-34〜AC-37・AC-47 のユニットテストが通る |
| M4: 実行の進め方と CLI | 6・7 | `internal/job`・`cmd/yt2column` | AC-19〜AC-31・AC-34〜AC-36・AC-38・AC-39・AC-44・AC-45・AC-47〜AC-52 のテストが通る |
| M5: 統合テスト | 8 | `deepseektestutil`・`make test-integration-cli`・統合テスト | AC-40〜AC-42 のテストが通り、ステップ 8-11 が成功する |
| M6: 文書と手動確認 | 9 | 文書、ステップ 9-5・9-6 の記録 | AC-43 のテストが通り、AC-32・AC-33 の記録がある |

### 3.2. PR 構成

PR はフェーズを基本の単位とし、高リスクなステップを含むフェーズ 1 とフェーズ 7 だけを分ける。フェーズ 1 は、子プロセスの管理（PR-2）とその他の共有部品（PR-1・PR-3）に分ける。フェーズ 7 は、副作用の前の検証（標準エラー出力の無害化と `--out` の判定、PR-9）と、CLI の配線と `run`（PR-10）に分ける。各 PR は主たる関心事（既存の識別子の公開 / 子プロセスの管理 / 共有の補助と記事の検査 / 設定 / プロバイダの選択 / 投稿 / 排他 / 実行の進め方 / CLI の前処理 / CLI の配線 / 統合テスト / 文書）を持ち、単独でグリーンゲートを通せる単位とする。

**PR の区切りの不変条件。** `/runplan` は §2 を文書の順に走査し、`PR-N 作成ポイント` に達したときにだけ PR を作る。そのため §2 は次を満たす。(1) 文書の順で、すべてのステップは、自分の PR の 1 つ前の作成ポイント（先頭の PR では文書の先頭）と自分の PR の作成ポイントの間にあり、他の PR のステップがその間に入らない。(2) すべての PR が作成ポイントを持ち、§3.2 の表・§7 のチェックリスト・§9 の実行順に現れる。本計画はステップを並べ替えていないため、ステップの番号の順と文書の順は一致する。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | `transcript.ValidateVideoURL`・`deepseek.ErrPaddedModel`・`NewForLoopbackTest` の公開 | standard |
| PR-2 | 1-4 / 1-5 | `InheritedFiles` の引き継ぎとプロセスグループの停止 | frontier-recommended |
| PR-3 | 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 | `SeedCacheForTest`、`Article.CheckPublishable`、ビルドタグの規則の例外と `TestFakesCarryBuildTag`、`package_reference.md`、フェーズ 1 の壊し確認と検索 | frontier-recommended |
| PR-4 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | `internal/config`（環境変数の読み込み・検証、キャッシュディレクトリの既定値、環境変数を読む場所の検査） | standard |
| PR-5 | 3-1 / 3-2 / 3-3 / 3-4 | `internal/llm/provider`（プロバイダの選択） | standard |
| PR-6 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | `internal/publisher` の `FilePublisher` | frontier-recommended |
| PR-7 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 | `internal/cachelock` と `yt-dlp` への引き継ぎの手動確認 | frontier-recommended |
| PR-8 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 | `internal/job` と偽の `yt-dlp` の共有の補助 | frontier-recommended |
| PR-9 | 7-1 / 7-2 | `cmd/yt2column` の `output.go`・`outpath.go`（標準エラー出力の無害化と `--out` の判定） | frontier-recommended |
| PR-10 | 7-3 / 7-4 / 7-5 / 7-6 / 7-7 / 7-8 / 7-9 / 7-10 | `cmd/yt2column` の `run.go`・`main.go` の配線、テストの補助、同一プロセスと別プロセスのテスト | frontier-required |
| PR-11 | 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8 / 8-9 / 8-10 / 8-11 | `internal/llm/deepseek/testutil` への移動、`cmd/yt2column` の統合テスト、`make test-integration-cli` | frontier-required |
| PR-12 | 9-1 / 9-2 / 9-3 / 9-4 / 9-5 / 9-6 / 9-7 | 文書、文書のテスト、手動確認 | standard |

フェーズ 1 を 3 つの PR に分けるのは、子プロセスの管理（`InheritedFiles` とプロセスグループの停止、ステップ 1-4・1-5）を高リスクな変更として隔離するためである。PR-1 は先に公開する識別子（`ValidateVideoURL`・`ErrPaddedModel`・`NewForLoopbackTest`）を提供し、PR-3 は残りの共有部品（`SeedCacheForTest`・`CheckPublishable`）とビルドタグの規則の例外・ガード、`package_reference.md` の更新、フェーズ 1 の壊し確認と検索を担う。`SeedCacheForTest` が `//go:build test || integration` を初めて使うので、規則の例外とガードの改修（ステップ 1-8）は同じ PR-3 に置く。

フェーズ 7 を 2 つの PR に分けるのは、秘密情報と制御文字の無害化（`output.go`、ステップ 7-1）というセキュリティの中核を、`run`・`main` の配線から隔離してレビューするためである。`output.go` は `run.go`（ステップ 7-3）が使うので先に置く必要があり、`outpath.go`（ステップ 7-2）も副作用の前に `--out` を検証する同じ関心事なので PR-9 に置く。`main` のシグナルの購読（ステップ 7-4）は、別プロセスのテスト（ステップ 7-8）と同じテストの補助（ステップ 7-5・7-6）を共有するので、実装とテストを分けないために同じ PR-10 に置く。そのため PR-10 では、CLI の配線（ステップ 7-3〜7-5）、テストの補助（ステップ 7-6）、同一プロセスのテスト（ステップ 7-7）、別プロセスのシグナルのテスト（ステップ 7-8）の 4 つのチェックポイントでレビューする。

`internal/` の変更は `cmd/` に先行する。PR-1〜PR-8 が本番の `internal/` のパッケージ（既存パッケージの変更と、`internal/config`・`internal/llm/provider`・`internal/publisher`・`internal/cachelock`・`internal/job`）を完成させ、PR-9・PR-10 がそれらを使う `cmd/yt2column` を組み立てる。依存の向きは次のとおり。PR-5 は PR-1 の `ErrPaddedModel`・`NewForLoopbackTest` を使う。PR-6 は PR-3 の `Article.CheckPublishable` を使う。PR-8 は PR-2 の `InheritedFiles`・プロセスグループの停止、PR-6 の `FilePublisher`、PR-7 の `cachelock` を使う。PR-10 は PR-9 の無害化と `--out` の判定を使う。PR-11 は PR-3 の `SeedCacheForTest` と PR-10 の CLI を使う。`package_reference.md` はパッケージを追加・変更するコミットで更新する規則に従い、各パッケージの PR（ステップ 1-9・2-6・3-3・4-4・5-3・6-5・7-9・8-8）で更新する。

**利用者の承認が要るステップ。** ステップ 5-4・8-11・9-5・9-6 は、実 `yt-dlp`・実 LLM API・手動の実行を伴うので、それぞれの PR（PR-7・PR-11・PR-12）の中で利用者の承認を得てから行う。

### 3.3. 実装順序の根拠

フェーズの順序は設計書 §8 のとおりである。後のフェーズが前のフェーズの成果物を使う関係は次のとおり: フェーズ 3 はフェーズ 1 の `ErrPaddedModel`・`NewForLoopbackTest` とフェーズ 2 を、フェーズ 6 はフェーズ 1・4・5 を、フェーズ 7 はフェーズ 2〜6 を、フェーズ 8 はフェーズ 1 の `SeedCacheForTest` とフェーズ 6・7 を使う。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   設計書 §7.1 の分担に従う。`internal/job` と `cmd/yt2column` のテストは、実際の `YtDlpSource` に偽の `yt-dlp` のスクリプト（ステップ 6-1）を与え、一時ディレクトリをキャッシュディレクトリにする。実 `yt-dlp` と実 LLM API は使わない。
-   `cmd/yt2column` のテストで本番の `deepseek.New` を使うのは、`YT2COLUMN_MODEL` の前後の空白の行だけである（構築で失敗し、要求を送らない）。`TestMain` が proxy を到達できないアドレスに向ける。
-   **同じテストの中で `run` や `job.Run` を 2 回以上呼ぶ場合**（AC-25・AC-26・AC-34・AC-36・AC-48・AC-49 のテストと、`TestRunLocked`・`TestRunReleasesLock`・`TestRunExecutionPaths` の同時実行の行）: 各回に、まだ存在しない別の `--out` のパスを使い、`job.Run` では同じパスから新しい `FilePublisher` を作る（ステップ 6-4 の補助）。回ごとに変えるのは、各テストが名前を挙げた設定（`YT2COLUMN_YTDLP_PATH` のトリップワイヤや字幕を書き出す偽の `yt-dlp`、`--refresh`）だけとし、それ以外の環境と引数は同じにする。2 回目以降も手順 A1〜A5 と B1 の検証を通るためである。
-   権限の失敗を起こすテストは `requireNonRoot` で root での実行を失敗にし、`chmodForTest` で後始末にパーミッションを戻す（既存の方針）。

### 4.2. 別プロセスで CLI を起動するテスト

-   I-02 の方法（§1.5、ステップ 7-8）。`make test` で実行する。

### 4.3. 統合テスト

-   `cmd/yt2column/integration_test.go`（ステップ 8-7）。`make test-integration-cli` だけが実行する。実行条件の判定は、`SettingsFrom` のユニットテスト（ステップ 8-3）と、CLI の統合テストが渡す設定のユニットテスト（ステップ 8-6 の `TestCLIIntegrationSettings`）で確かめる。統合テストが判定の結果に従って動くこと（スキップ・失敗では `run` も LLM のクライアントも呼ばないこと）は、ステップ 8-6 の検証事項で確かめる。

### 4.4. 後方互換性

-   既存の段階の振る舞いは変えない（要件書 §5）。フェーズ 1 の変更の後も、既存のテストを §1.4 の追従と §1.3 の `TestCommandExecutorWaitDelay` の部分テストのほかは変更せずに通す。
-   既存の DeepSeek の統合テストは `MissingKeySkip` で振る舞いを変えない（ステップ 8-3 の `MissingKeySkip` のケースが、移動の前と同じ結果を確かめる）。

### 4.5. 網羅率

-   新しいパッケージの本番の関数は、エラーの経路を含めてテストで実行する。網羅率の数値の目標は設けず、ステップ 8-9 のテストの削除の確認だけで数値を比べる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別: `test`（実行して振る舞いを確かめる）、`static`（ガードのテストなど、文書やソースの記載を機械的に確かめる）、`manual`（人が実行して確かめる）。「実装」はその AC を満たすコードを作るステップ、「テスト」はテストを作るステップである。

| AC | 検証 | 種別 | 実装 | テスト |
|---|---|---|---|---|
| AC-01 | `internal/config/config_test.go::TestLoadValid` | test | 2-1 | 2-4 |
| AC-02 | `internal/config/config_test.go::TestLoadDefaults`、`internal/config/cachedir_test.go::TestDefaultCacheDir` | test | 2-1・2-2 | 2-4 |
| AC-03 | `internal/config/config_test.go::TestLoadMissing` | test | 2-1 | 2-4 |
| AC-04 | `internal/config/config_test.go::TestLoadEmpty` | test | 2-1 | 2-4 |
| AC-05 | `internal/config/config_test.go::TestLoadInvalid` | test | 2-1 | 2-4 |
| AC-06 | `internal/config/config_test.go::TestLoadErrorsOmitValues` | test | 2-1 | 2-4 |
| AC-07 | `internal/config/config_test.go::TestLoadReportsAllInvalid` | test | 2-1 | 2-4 |
| AC-08 | `internal/config/config_test.go::TestConfigOutputRedactsSecrets` | test | 2-1 | 2-4 |
| AC-09 | `internal/config/envaccess_test.go::TestEnvAccessConfined`・`::TestEnvAccessScannerDetects` | static | 2-5・7-4 | 2-5・7-9 |
| AC-10 | `internal/llm/provider/provider_test.go::TestNewDeepSeekSendsConfiguredRequest`・`::TestNewUsesDeepSeekAdapter` | test | 1-3・3-1 | 3-2 |
| AC-11 | `internal/llm/provider/provider_test.go::TestNewUnknownProvider` | test | 3-1 | 3-2 |
| AC-12 | `internal/llm/provider/provider_test.go::TestNewPaddedModel`、`internal/llm/deepseek/deepseek_test.go::TestNew` | test | 1-2・3-1 | 1-2・3-2 |
| AC-13 | `internal/publisher/file_test.go::TestFilePublisherWritesArticle` | test | 4-1 | 4-2 |
| AC-14 | `internal/publisher/file_test.go::TestFilePublisherExistingPath` | test | 4-1 | 4-2 |
| AC-15 | `internal/publisher/file_test.go::TestFilePublisherDirectoryFailure` | test | 4-1 | 4-2 |
| AC-16 | `internal/publisher/file_test.go::TestFilePublisherRejectsArticle`、`internal/writer/article_test.go::TestArticleCheckPublishable` | test | 1-7・4-1 | 1-7・4-2 |
| AC-17 | `internal/publisher/file_test.go::TestFilePublisherCanceled` | test | 4-1 | 4-2 |
| AC-18 | `internal/publisher/file_test.go::TestNewFilePublisherEmptyPath` | test | 4-1 | 4-2 |
| AC-19 | `cmd/yt2column/run_test.go::TestRunExecutionPaths`（成功の行） | test | 6-2・7-3 | 7-7 |
| AC-20 | `cmd/yt2column/run_test.go::TestRunExecutionPaths`（終了コード `2` の行）、`cmd/yt2column/outpath_test.go::TestOutPathInsideCacheDir` | test | 1-1・2-1・3-1・7-2・7-3 | 7-2・7-7 |
| AC-21 | `cmd/yt2column/run_test.go::TestRunHelp` | test | 7-3 | 7-7 |
| AC-22 | `cmd/yt2column/run_test.go::TestRunPromptOverrides` | test | 7-3 | 7-7 |
| AC-23 | `cmd/yt2column/run_test.go::TestRunExecutionPaths`（3 つの段階の失敗の行） | test | 7-1・7-3 | 7-7 |
| AC-24 | `internal/job/job_test.go::TestRunRemovesCache` | test | 6-2 | 6-3 |
| AC-25 | `internal/job/job_test.go::TestRunKeepCache` | test | 6-2 | 6-3 |
| AC-26 | `internal/job/job_test.go::TestRunFailureKeepsCache` | test | 6-2 | 6-3 |
| AC-27 | `internal/job/job_test.go::TestRunPrunesDangling` | test | 6-2 | 6-3 |
| AC-28 | `internal/job/job_test.go::TestRunPruneFailureWarns`、`cmd/yt2column/run_test.go::TestRunExecutionPaths`（警告の行） | test | 6-2・7-3 | 6-3・7-7 |
| AC-29 | `internal/job/job_test.go::TestRunRemoveCacheFailureWarns`、`cmd/yt2column/run_test.go::TestRunExecutionPaths`（警告の行） | test | 6-2・7-3 | 6-3・7-7 |
| AC-30 | `internal/job/job_test.go::TestRunRefresh` | test | 6-2 | 6-3 |
| AC-31 | `internal/job/job_test.go::TestRunCanceled`、`cmd/yt2column/run_test.go::TestRunExecutionPaths`（中断の行） | test | 4-1・6-2・7-3 | 6-3・7-7 |
| AC-32 | `cmd/yt2column/docs_test.go::TestPlanRecordsManualRuns`、ステップ 9-5 の記録 | static・manual | 7-3 | 9-4・9-5 |
| AC-33 | `cmd/yt2column/docs_test.go::TestPlanRecordsManualRuns`、ステップ 9-6 の記録 | static・manual | 7-3 | 9-4・9-6 |
| AC-34 | `internal/cachelock/cachelock_test.go::TestAcquireLocked`、`internal/job/job_test.go::TestRunLocked`、`cmd/yt2column/run_test.go::TestRunExecutionPaths`（同時実行の行） | test | 5-1・6-2・7-3 | 5-2・6-3・7-7 |
| AC-35 | `internal/cachelock/cachelock_test.go::TestAcquireOtherDirectory`、`internal/job/job_test.go::TestRunOtherCacheDir` | test | 5-1・6-2 | 5-2・6-3 |
| AC-36 | `internal/cachelock/cachelock_test.go::TestAcquireAfterHolderGone`、`cmd/yt2column/signal_test.go::TestSIGKILLWithoutYtDlp` | test | 5-1・6-2 | 5-2・7-8 |
| AC-37 | `internal/cachelock/cachelock_test.go::TestLockFileSurvivesPrune` | test | 5-1 | 5-2 |
| AC-38 | `cmd/yt2column/run_test.go::TestRunExecutionPaths`、`cmd/yt2column/signal_test.go::TestSignalDuringYtDlp`、`cmd/yt2column/output_test.go::TestSanitize` | test | 7-1・7-3・7-4 | 7-1・7-7・7-8 |
| AC-39 | `cmd/yt2column/run_test.go::TestRunEscapesUntrustedText`、`cmd/yt2column/output_test.go::TestSanitize` | test | 7-1・7-3 | 7-1・7-7 |
| AC-40 | `cmd/yt2column/makefile_test.go::TestCLIIntegrationSettings`、`internal/llm/deepseek/testutil/integration_settings_test.go::TestSettingsFrom`、統合テストが判定に従うこと（ステップ 8-6 の検証事項） | test | 8-1・8-6・8-7 | 8-3・8-6 |
| AC-41 | `cmd/yt2column/integration_test.go::TestIntegrationCLI`（`make test-integration-cli`）、ステップ 8-11 の記録 | test・manual | 1-6・8-5・8-7 | 8-7・8-11 |
| AC-42 | `cmd/yt2column/makefile_test.go::TestCLIIntegrationTestBuildTag`、`internal/transcript/ytdlp_test.go::TestLintTagsIncludeIntegration`（`go vet -tags integration` の行）、`make lint` | static | 8-5・8-7 | 8-6 |
| AC-43 | `cmd/yt2column/docs_test.go::TestREADMEDocumentsCLI`・`::TestProjectOverviewDocumentsConfig`、`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`、ステップ 9-7 の突き合わせ（散文の確認を含む） | static・manual | 1-9・2-6・3-3・4-4・5-3・6-5・7-9・8-8・9-1〜9-3 | 9-4・9-7 |
| AC-44 | `cmd/yt2column/run_test.go::TestRunExecutionPaths`・`::TestRunHelp`、`cmd/yt2column/signal_test.go::TestSignalDuringYtDlp`・`::TestSignalDuringGenerate` | test | 6-2・7-1・7-3・7-4 | 7-7・7-8 |
| AC-45 | `cmd/yt2column/signal_test.go::TestSignalDuringYtDlp`、`internal/transcript/exec_test.go::TestCommandExecutorKillsProcessGroup`、`main` の購読の経路（ステップ 7-5 の検証事項） | test | 1-5・6-2・7-4 | 1-5・7-8 |
| AC-46 | `internal/publisher/file_test.go::TestFilePublisherWriteFailure` | test | 4-1 | 4-2 |
| AC-47 | `internal/cachelock/cachelock_test.go::TestAcquireOtherFailures`、`internal/job/job_test.go::TestRunLockFailure`、`cmd/yt2column/run_test.go::TestRunExecutionPaths`（排他の取得の失敗の行） | test | 5-1・6-2・7-3 | 5-2・6-3・7-7 |
| AC-48 | `internal/job/job_test.go::TestRunInvalidCache`、`cmd/yt2column/run_test.go::TestRunExecutionPaths`（キャッシュの内容が不正な行） | test | 6-2・7-3 | 6-3・7-7 |
| AC-49 | `cmd/yt2column/signal_test.go::TestSIGKILLWhileYtDlpRuns`、`internal/cachelock/cachelock_test.go::TestLockInheritedByChild` | test | 1-4・5-1・6-2・7-3 | 5-2・7-8 |
| AC-50 | `internal/job/job_test.go::TestRunOutputExists`、`cmd/yt2column/run_test.go::TestRunExecutionPaths`（`--out` が既存の行） | test | 6-2・7-3 | 6-3・7-7 |
| AC-51 | `cmd/yt2column/run_test.go::TestRunGODEBUGWarning`、`internal/config/config_test.go::TestLoadHTTP2Debug` | test | 2-3・7-3 | 2-4・7-7 |
| AC-52 | `internal/job/job_test.go::TestRunOutputParentInvalid`、`cmd/yt2column/run_test.go::TestRunExecutionPaths`（親ディレクトリの行） | test | 6-2・7-3 | 6-3・7-7 |

-   **AC-32・AC-33 の `static` と `manual` の分担:** 両 AC は、実際の動画に対する手動の実行を本計画の完了条件に含め、その結果を本計画に記録することを求める。完了条件のステップがあることと、チェック済みのステップに動画 URL と結果の記録があることは、`TestPlanRecordsManualRuns` が `static` で確かめる。実際の実行そのものは、実 `yt-dlp` と実 LLM API を使い利用者の承認が要る（§1.2）ので、自動の検証には置き換えられず、ステップ 9-5・9-6 の `manual` で行う。
-   **AC-41 の「テストの出力」:** テストが標準出力・標準エラー出力・`--out` のファイルを確かめる部分は `test` である。テスト自身のログに API キーが出ないことは、ステップ 8-7 の書き方（確かめるまで書かない）と、ステップ 8-11 で実際の出力を確かめる `manual` で補う。
-   **AC-43 の `static` と `manual` の分担:** 文書のテストは、機械的に確かめられる構造と契約値（README の呼び出し形式・フラグ・終了コード・設定の既定値、`project_overview.md` の設定表、`package_reference.md` のパッケージ一覧）を確かめる。散文の意味（同時実行・SIGKILL・`--refresh` の案内、パーミッション `0o644` とハードリンク、統合テストの実行方法、`security.md` §2 の `http2debug` の警告と統合テストの例外）は文字列一致では保証できないので文書のテストでは固定せず、ステップ 9-7 で実装と突き合わせて確かめる。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| シグナルと子プロセスのテストが CI で不安定になる | `make test` の失敗が断続的に起きる | シグナルは準備完了の印を待ってから送る。待ちはすべて上限付きの条件の待ちにし、固定の待ち時間を使わない。印のファイルは原子的に作る（ステップ 6-1・7-5・7-8） |
| テストの中断（`go test` のタイムアウトや SIGKILL）で偽の `yt-dlp` が残る | 開発機と CI の環境を汚す | 止まる偽の `yt-dlp` は解放の印か数分の上限で自分で終了する（ステップ 1-5・6-1） |
| 後始末で終了済みの PID にシグナルを送り、再利用された無関係のプロセスを止める | 利用者のプロセスの停止 | PID を指定してシグナルを送る後始末はしない。テストが `exec.Cmd` で起動した子は `Process.Kill` と `Wait` で止める（ステップ 5-2・6-1・7-8） |
| 開発者の環境変数（`GODEBUG`・`YT2COLUMN_*`・実在の API キー）が子プロセスに漏れる | 期待しない警告、実在のキャッシュの変更、目印の検査の空振り | 子プロセスの環境を allowlist で作る（ステップ 7-6） |
| `-race` で子プロセスの起動が遅くなる | 上限の時間内に準備が終わらない | 上限は秒の単位で十分に長くし、上限に達したら原因（どの印を待っていたか）を示して失敗する |
| 書いた直後のスクリプトの実行が Linux で `ETXTBSY` になる | 断続的な失敗 | 偽の `yt-dlp` を起動するテストは `t.Parallel` にしない（ステップ 6-3） |
| 実 `yt-dlp` の子プロセスが記述子 3 を引き継ぐ | 排他が長く保持される | 安全側に倒れる（設計書 3.7）。ステップ 5-4 で確かめて記録する |
| 権限の失敗を起こすテストが root で実行される | テストの前提が崩れる | `requireNonRoot` で失敗にする |
| 大文字と小文字を区別しないファイルシステム（macOS）と区別するもの（CI の Linux）で `--out` の判定の結果が変わる | 一方の環境でだけ失敗する | 綴りの大小の違いのテストは、存在しない部分で作る（大文字と小文字を区別せずに比べる規則なので、両方で同じ結果になる）。存在する部分の綴りの違いは、`os.SameFile` の結果が環境で変わるので、期待値をその結果から決めない |
| `make` を偽のコマンドで実行するテストが外側の `make` の変数を引き継ぐ | 誤った引数の記録 | 移した補助は、子の環境を allowlist で組み立てる既存の方法を保つ（ステップ 8-2） |

### 6.2. スケジュールのリスク

| リスク | 対策 |
|---|---|
| フェーズ 7 の子プロセスのテストに時間がかかる | フェーズ 7 を、同じプロセスの中のテスト（ステップ 7-1〜7-7）と子プロセスのテスト（ステップ 7-8）の 2 つのコミットに分けてよい |
| ステップ 5-4・8-11・9-5・9-6 が利用者の承認待ちになる | 承認の依頼を早めに出し、待つ間は現在の PR の中で、実ネットワークを使わないステップを進める。現在の PR をマージし次のブランチを選ぶ前に、次のフェーズのコードを始めない（§3.2 の不変条件） |

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
-   [ ] PR-2 マージ済み（対象ステップ: 1-4 / 1-5）
-   [ ] PR-3 マージ済み（対象ステップ: 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12）
-   [ ] PR-4 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
-   [ ] PR-5 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4）
-   [ ] PR-6 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5）
-   [ ] PR-7 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5）
-   [ ] PR-8 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6）
-   [ ] PR-9 マージ済み（対象ステップ: 7-1 / 7-2）
-   [ ] PR-10 マージ済み（対象ステップ: 7-3 / 7-4 / 7-5 / 7-6 / 7-7 / 7-8 / 7-9 / 7-10）
-   [ ] PR-11 マージ済み（対象ステップ: 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8 / 8-9 / 8-10 / 8-11）
-   [ ] PR-12 マージ済み（対象ステップ: 9-1 / 9-2 / 9-3 / 9-4 / 9-5 / 9-6 / 9-7）
-   [ ] 各 PR で `make fmt` → `make test` → `make lint` が通る
-   [ ] §5 のすべての AC の検証が通り、`manual` の記録がそろっている
-   [ ] §1.4 の変更前の名前が、`docs/tasks/` 以外に残っていない（ステップ 1-12・8-10）

## 8. 成功基準 (Success Criteria)

-   **機能:** §5 のすべての AC の検証が通る。`make test-integration-cli` が成功する（ステップ 8-11）。
-   **品質:** `make test` と `make lint` が通る。各テストについて、対象を壊すと失敗することをコミットメッセージに記録している。
-   **セキュリティ:** AC-09・AC-38・AC-39・AC-14・AC-50・AC-20・AC-40・AC-41 の検証が通る（設計書 7.4）。`gosec` の抑止は理由付きで対象の行だけに付けている。
-   **文書:** AC-43 の検証が通り、ステップ 9-7 で文書を実装と突き合わせている。

## 9. 次のステップ (Next Steps)

-   本計画の Status は現在 `draft` である。レビュー担当者が計画を確認して Status を `approved` に設定する。PR の区切りは §2 の `PR-N 作成ポイント` と §3.2 に埋め込み済みである。
-   Status が `approved` になった後にだけ、`/runplan 0005` で PR の順（PR-1 → PR-2 → … → PR-12）に実装する（各 PR は独立してグリーンゲートを通す。§3.2 の不変条件）。
-   実装の完了後、Slack の投稿先（#7）の要件の作成に進む。設計書 §9 の `Article.CheckPublishable` と `config.Config.SlackWebhookURL` を使う。
