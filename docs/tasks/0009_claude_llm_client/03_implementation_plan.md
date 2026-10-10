# 実装計画書：Claude アダプタ（LLMClient 実装）

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-10 |
| Review date | 2026-10-10 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md`（以下「requirements」）で定義した Claude アダプタと、それを CLI から選ぶための設定を、`02_architecture.md`（以下「architecture」）の設計どおりに実装する。具体的には、DeepSeek アダプタの HTTP 通信の手順を新設の `internal/llm/llmhttp` へ移し、effort とワークスペース ID の型を `internal/llm/claudeparam` に置き、`internal/llm/claude` に `llm.LLMClient` の実装を新設する。続けて、`internal/config`・`internal/llm/provider`・`cmd/yt2column` で `claude` プロバイダを選べるようにし、実 API を使う統合テストを `make test-integration-claude` に分離する。

本計画は architecture §8 の実装優先順位をそのままフェーズ 1〜6 とする。事前調査（architecture §1.4）は実施済みで、フィクスチャ 2 件と `testdata/README.md` の出典はコミット `9e7d0db` で追加済みである（§1.3）。

### 1.2. 実装原則

- architecture §1.1 の設計原則 7 項目に従う。特に、応答本文は補正せずに拒否し（原則 3）、`internal/llm/claude` の非テストのソースで `Reveal()` を呼ぶのは `request.go` の 2 か所だけとし（原則 6。統合テストが `Model` に API キーが含まれないことを確かめるための呼び出しは対象外）、モデル名から挙動を推測しない（原則 5）。
- 新設・変更するファイルは architecture §3.8 の一覧を基本とする。一覧にないファイルの変更（guard の更新、古くなるコメントの修正、テストの追加）は、各ステップに対象ファイルとして明記する。
- `test_helpers*.go` を使う `_test.go` の先頭には `//go:build test` を付ける（`internal/llm/deepseek/deepseek_test.go:1` と同じ）。
- Go のコメント・識別子・文字列リテラルは英語で書く。`AC-NN`・`F-NNN`・`H-NN`・`I-NN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- 上限値・猶予・送信先・`anthropic-version`・既定の `max_tokens` などは名前付き定数にする（`mnd`・`goconst`）。
- 各フェーズの完了条件は `make fmt` → `make test` → `make lint` が通ること。各テストは、対象の分岐を実際に壊して失敗することを確認し、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」）。

### 1.3. 既存コード調査結果

HEAD `7864eec`（ブランチ `issei/llm-claude-02`）で確認した。architecture が既存コードを確認した `c45b750` から HEAD までの差分は、`docs/` 以外では `testdata/README.md`・`testdata/claude_messages_end_turn.json`・`testdata/claude_messages_max_tokens.json` の追加だけである（`git diff --stat c45b750 HEAD -- . ':!docs'`、2026-10-10）。そのため、architecture が引く行番号は HEAD でもそのまま有効である。ベースラインの `go test -tags test ./...`（2026-10-10、HEAD `7864eec`、go1.27.1）は、`internal/transcript` の `TestCommandExecutorDrainsStderr` が並列実行の負荷で 1 回失敗した以外は、すべて `ok` だった。同テストを単独で再実行すると `ok` になった。本タスクと関係のない既存の不安定さとして扱う。

- **`internal/llm/deepseek` の送信の手順（フェーズ 1 で移す）。** `deepseek.go:74-81`（リダイレクトに従わない `http.Client`）、`:104-143`（`generate`。`ctx` の確認、`context.WithTimeoutCause`、`200` 以外の本文を読まない、本文の読み取り）、`:145-174`（`classifyFailure`・`contextFailure`）、`response.go:40-51`（`readResponseBody`）、`errors.go:32`（`errAdapterTimeout`）。これらの識別子の参照は、すべて `deepseek.go`・`response.go`・`errors.go` の中にあり、テストと `test_helpers*.go` は参照しない（`rg -n "readResponseBody|classifyFailure|contextFailure|errAdapterTimeout"`、2026-10-10、HEAD `7864eec`）。`statusError`（`deepseek.go:179-185`）はモデル名と `max_tokens` を加えるため、DeepSeek アダプタに残し、`llmhttp.Errors.Status` に渡す関数から呼ぶ。
- **DeepSeek のテストが参照する非公開の名前。** architecture §3.4 の表のとおりである。HEAD で確認した参照箇所は、`client.httpClient.Transport` が `deepseek_test.go:686`、`maxResponseBytes` が `response_test.go:366-386`、`errorPrefix` が `test_helpers.go:182-186` である。
- **DeepSeek のタイムアウトのテストはメッセージの部分文字列を検査する。** `deepseek_test.go:493`（アダプタのタイムアウトの値 `100ms` を含む）、`:514`（呼び出し元の期限が先のとき、アダプタのタイムアウトの値 `2s` を含まない）、`:517`（`caller` を含む）。経過時間を加えるとき（ステップ 1-8）、その表記が `2s` を部分文字列として含んで `:514` の検査を誤って失敗させることがないように、表記を選ぶ必要がある（§6）。
- **テストサーバーの部品。** `internal/llm/deepseek/test_helpers.go:220-427`（記録するサーバー、固定の応答のサーバー、待ち続けるサーバー、最初のリクエストを知らせるサーバー、本文の途中で閉じるリスナー、接続を受けてすぐ閉じるプロキシ）。architecture §7.1 のとおり、これらを `internal/llm/llmhttp/llmhttptest` に作り直し、DeepSeek の `test_helpers.go` は変えない。
- **パッケージに関する guard（`internal/pipeline/pipeline_test.go`）。**
  - `TestFakesCarryBuildTag`（`:530`）は、`testutil` ディレクトリの `.go` ファイル（`_test.go` を含む）を数え、15 件に固定している（`:554`）。本計画は `internal/llm/deepseek/testutil/make_test.go` を残して中身を入れ替え（ステップ 5-2）、`internal/llm/claude/testutil/integration.go`・`integration_settings_test.go` を加える（ステップ 5-3）。最終的な件数は 17 である。
  - `TestWriterImports`（`:746`）は、`internal/writer` が `net`・`net/*`・`internal/llm/*` を import しないことを、import の解析で固定している。ステップ 4-4 の設定の import の guard はこの形を手本にする。
  - `testutil` 以外の test 専用パッケージは `testOnlyPackageDirs`（`:521-523`、現在は `internal/loopbacktest` だけ）に挙げる必要がある。新設の `internal/llm/llmhttp/llmhttptest`（ステップ 1-2）と `internal/maketestutil`（ステップ 5-1）を加える。
  - `TestPackageReferenceListsPackages`（`:606`）は、非テストのソースを持つパッケージと `package_reference.md` の行の一致を双方向に求める。そのため、パッケージを新設するステップで同じコミットに行を加える（architecture §8 の末尾の段落）。
- **`internal/config/config.go`。** `loadAPIKey`（`:199-222`）は、プロバイダによらず空の `DEEPSEEK_API_KEY` を拒否する。`reasonProvider` は `:52`、http2debug のコメントは `:40-41`、`apiKeyEnv` は `:30` にある。`apiKeyEnv` の参照は `config.go` と `config_test.go`（`:32`・`:71-72`・`:97`・`:134-135`・`:151`・`:289`・`:301`・`:359`）だけである（`rg -n apiKeyEnv`、2026-10-10、HEAD `7864eec`）。`config_test.go:173` は `claude` を不正なプロバイダとして挙げている。
- **`internal/config/envaccess_test.go`。** 秘密情報の変数名は `secretEnvNames`（`:59`）で、文字列リテラルの部分一致で検査する（`:200-201`）。テストのコード（`_test.go`、または `test`・`integration` タグがないとビルドされないファイル）は検査の対象外である（`:276-299` の `isTestCode`）。そのため、`internal/llm/claude/testutil/integration.go`（`test || integration`）の `YT2COLUMN_TEST_ANTHROPIC_API_KEY` は、部分文字列 `ANTHROPIC_API_KEY` を含むが検査に掛からない。
- **`internal/llm/provider`。** `newClient` の呼び出しは `provider.go:29` と `provider_test.go:132`・`:178` の 3 か所だけである（`rg -n "newClient\("`）。`provider_test.go` には `TestMain` がなく、`TestNewUsesDeepSeekAdapter`（`:153`）は本番の `deepseek.New` で構築するが `Generate` は呼ばない。
- **`cmd/yt2column`。** `configuredSecrets`（`run.go:474-486`）は DeepSeek の API キーと Webhook URL だけを返す。対応するテストは `run_test.go:1398` の `TestConfiguredSecretsIncludesWebhookParts` である。`warnHTTP2Debug`（`run.go:302-310`）の文言はキーの種類を問わない。`docs_test.go:33-40` の `configDocRows` は README と project_overview の設定の表を検査するため、表の更新と同じコミットで行を加える（ステップ 6-2）。
- **Make の部品。** `internal/llm/deepseek/testutil/make.go` の公開の名前（`RunMakeTarget`・`MakeInvocation`・`ChargedTarget`・`CheckChargedTarget`・`FirstLineIs`・`ErrFirstLineMismatch`）は、`cmd/yt2column/makefile_test.go`・`internal/publisher/makefile_test.go`・`internal/llm/deepseek/makefile_test.go`・`internal/llm/deepseek/deepseek_test.go` が使う（`rg -n "FirstLineIs|RunMakeTarget|CheckChargedTarget"`）。公開の形を変えずにラッパーとして残す。`make_test.go` は非公開の `validateEnvNames`・`recordedNames` だけをテストしている。
- **Makefile。** `test-integration-deepseek`（`Makefile:92-110`）が手本である。`.PHONY`（`:49`）と `lint` の前のコメント（`:163-167`）は統合テストのターゲットを列挙している。`make lint` は `go vet -tags integration ./...` も実行するため、`integration` タグだけのビルドの型の誤りも検出する（`Makefile:168-170`）。
- **フィクスチャ。** `testdata/claude_messages_end_turn.json`（1459 バイト）と `testdata/claude_messages_max_tokens.json`（596 バイト）、および `testdata/README.md` の出典の節は、コミット `9e7d0db` で追加済みである。architecture §8 の 6 の「testdata の README」は完了しており、本計画は内容の確認だけを行う（ステップ 6-7）。
- **ドキュメントの更新対象。** `README.md:142`・`:144`（設定の表）、`:160`・`:228-275`（統合テスト）。`docs/dev/project_overview.md:43`・`:79-82`（パッケージ構成）・`:95-97`（設定の表）・`:104`（空の値の扱い）。`docs/dev/security.md:18-41`（§2）・§4（`:52`）。`CLAUDE.md:95-99`（Architecture Overview）・`:210`（秘密情報の例）。`docs/dev/developer_guide/package_reference.md:18`・`:24-25`・`:32`・`:35`。英語版の文書（`*.en.md`）は存在しない。
- **外部前提の確認。**
  - `go.mod` は `go 1.26.5`、手元のツールチェーンは go1.27.1。`errors.AsType`（Go 1.26）と `context.WithTimeoutCause`（Go 1.21）が使える。外部モジュールは追加しない。
  - GNU Make 3.81（手元の `make --version`）。`?=`、ターゲット固有の `export`、コマンドラインの変数の優先の意味は、`0003_deepseek_llm_client` の実装計画 §1.3 で確認済みで、`test-integration-deepseek` が使っている意味と同じである。本計画の Make ターゲットもこれと同じ形にする。
  - プロキシの環境変数は `http.ProxyFromEnvironment` がプロセス内で 1 回だけ読む。`internal/llm/deepseek/deepseek_test.go:36` の `TestMain` はこれに依拠しており、本計画の `TestMain`（ステップ 3-6・4-6）も同じ形にする。
- **古くなるコメントと文書（lint では検出されない）。** `internal/config/config.go:37`（`providerDeepSeek is the only accepted value`）、`:199-201`（`loadAPIKey` の doc）、`internal/llm/provider/provider.go:32-36`（`newClient` の doc。`production passes deepseek.New`）、`internal/llm/deepseek/errors.go:23-25`（`errAdapterTimeout` に触れる）、`internal/llm/deepseek/deepseek.go:25-26`（`maxResponseBytes` の doc。読み取りは `llmhttp` が行うようになる）、`CLAUDE.md:94-95` と `package_reference.md:18`（`internal/strictjson` の利用者に Claude アダプタが加わる）、`AGENTS.md:10`（秘密情報の例。`CLAUDE.md:210` と同じ内容）。それぞれ、該当するステップで直す。
- **意図して変えないもの。** `internal/publisher/slack.go:111-113` もリダイレクトに従わない `http.Client` を持つが、`llmhttp` は LLM の呼び出しのための部品であり、移さない。
- **計画上の判断（architecture の決定は変えない）。**
  - `claudeparam.Effort` に、5 つの値のいずれかであるかを返すメソッド（`Valid`）を加える。architecture §3.2 の公開 API の概略にはないが、`claude.New` が「5 つの値のどれでもない effort」を拒否するときに、値の一覧を `claudeparam` の外に持たないためである（requirements §4.5）。`ParseEffort(e.String())` の往復でも判定できるが、文字列を経由した判定は意図が読み取りにくいため採らない。
  - requirements AC-31 の設定の部分は、`internal/config` と `internal/llm/claudeparam` が、`net`・`net/*` と、`internal/llm/claudeparam` 以外の `internal/llm/*` を直接 import しないことを固定する guard で確かめる（ステップ 4-4）。推移的な依存は検査しない（`internal/config` は `internal/slackwebhook` を通じて既に `net/url` に依存している）。アダプタのパッケージへの直接の import を禁じることで、`net/http` を持つパッケージへの依存を防ぐ。
  - `TestMain` のプロキシと Claude の統合テストの判定のテストは、DeepSeek の `TestMain`・`TestIntegrationOptionsSkipMissingKey`（`deepseek_test.go:777`）と同じ形にする。`Post` の契約（タイムアウト・キャンセル・通信の失敗・上限・`200` 以外）は `llmhttp` と各アダプタの両方で検証する。requirements §1 が、共通の規則をアダプタごとに独立して検証できるように AC として定めているためである。
  - `cmd/yt2column/docs_test.go` に、本計画の手動実行の記録（§5.1）と、統合テストの変数名・Make ターゲットの記述を固定する guard を加える（ステップ 6-2）。`0005`・`0006` の `TestPlanRecordsManualRuns`・`TestSlackDocsContract`（`docs_test.go:146`・`:174`）と同じ形である。
  - architecture §4.2 は、CLI が `400` のときにワークスペース ID の環境変数を案内してもよいとする。本計画では加えない（YAGNI）。
  - `deepseektestutil` の `make_test.go` のテストは、移した非公開の関数のテストであるため `internal/maketestutil` に移す。ファイルは残し、ラッパーが記録する変数を固定するテストに入れ替える（ステップ 5-2）。
  - architecture §8 は、実装計画の作成時に編集上の修正をした（同書の Comments）。テストサーバーの部品をフェーズ 1 に、パッケージの行の追加を各フェーズに置いた。決定は変えていない。

### 1.4. 名前の変更・移動の一覧

本計画の他の箇所は、この表に従う。

| 変更前 | 変更後 | ステップ |
|---|---|---|
| `internal/llm/deepseek` の `readResponseBody`・`classifyFailure`・`contextFailure`・`errAdapterTimeout` | `internal/llm/llmhttp` の `Post` の内部（非公開） | 1-4 |
| `internal/llm/deepseek` の `maxResponseBytes`（`8 << 20`） | `llmhttp.MaxResponseBytes` を指す定数として残す | 1-4 |
| `deepseek.go:74-81` の `http.Client` の構成 | `llmhttp.NewClient()` | 1-4 |
| `internal/config` の `apiKeyEnv` | `deepSeekAPIKeyEnv` | 4-1 |
| `internal/config` の `loadAPIKey` | `loadDeepSeekAPIKey` | 4-1 |
| `provider.newClient(provider config.Provider, apiKey secret.Secret, model string, build func(deepseek.Options) (llm.LLMClient, error))` | `provider.newClient(provider config.Provider, cfg config.Config, build builders)`（architecture §3.3） | 4-5 |
| `deepseektestutil` の `childEnvAllowlist`・`envNamePattern`・`validateEnvNames`・`makeStubScript`・`errEnvName`・記録の読み取り（`MakeInvocation` の組み立て）・`FirstLineIs`・`ErrFirstLineMismatch`、`CheckChargedTarget` の引数・`-timeout`・オプトインの検査 | `internal/maketestutil` の部品。記録する変数と子プロセスの環境に渡す変数、表示の文言、既定値は呼び出し側が指定する。`deepseektestutil` の公開の名前（`RunMakeTarget`・`MakeInvocation`・`ChargedTarget`・`CheckChargedTarget`・`FirstLineIs`・`ErrFirstLineMismatch`）は、型の別名またはラッパーとして残す | 5-1・5-2 |
| `deepseektestutil` の `recordedEnv`・`recordedNames` | `recordedEnv`（DeepSeek・CLI のオプトインとモデル名）は `deepseektestutil` に残し、重複を除いて連結する処理は `maketestutil` に移す | 5-1・5-2 |
| `internal/llm/deepseek/testutil/make_test.go` の `TestValidateEnvNames`・`TestRecordedNames` | `internal/maketestutil/make_test.go`（汎用の形）。`deepseektestutil/make_test.go` には、ラッパーが記録する変数を固定するテストを置く | 5-1・5-2 |

### 1.5. implementation_handoff.md の項目への対応

| ID | 対応 |
|---|---|
| I-01 | テストは `errors.AsType[*HTTPStatusError](err)` でステータスコードを取り出し、応答のステータスコードと比べる（ステップ 3-6 の `TestGenerateHTTPStatus`）。 |
| I-02 | 接続できない送信先は、接続を受けてすぐ閉じるリスナー（DeepSeek の AC-19 のテストと同じ手段）を `llmhttptest` に作って使う。本文の途中で閉じる送信先は、`Content-Length` に本文より大きい値を示し、有効な応答の JSON の一部だけを書いて接続を閉じるリスナーを `llmhttptest` に作って使う（ステップ 1-2）。 |
| I-03 | `//go:build integration` で分離し、`make test-integration-claude` は `go test -tags integration -count=1 -timeout 15m -v ./internal/llm/claude` で実行する（ステップ 5-6）。API キーの変数の未設定・空は `t.Skip`、モデル名・effort の欠落・不正とワークスペース ID の空・不正は `t.Fatal` にする（ステップ 5-3・5-5）。`-timeout` の 15 分は architecture §7.2 のとおり、`Generate` 2 回分のタイムアウト（5 分 × 2）より長い。 |
| I-04 | `testdata/claude_messages_end_turn.json` と `claude_messages_max_tokens.json` をそのまま入力にし、さらにトップレベルと `content` の要素にテスト用の未知のメンバーを加えたものも入力にする（ステップ 3-7 の `TestGenerateResponseFixtures`・`TestGenerateUnconsumedMembers`）。 |

`design_handoff.md` の H-01〜H-11 は、すべて architecture §3.9 に記録されている。

## 2. 実装ステップ (Implementation Steps)

ステップは `X-Y` 形式で表す（X: フェーズ番号、Y: フェーズ内の連番）。テスト関数名と AC の対応は §5 にまとめ、各ステップでは対象の AC だけを示す。各フェーズの完了条件は、最後のステップ（壊して失敗することの確認と `make fmt` → `make test` → `make lint`）が通ることである。新設のパッケージの `package_reference.md` の行と、`internal/pipeline/pipeline_test.go` の guard の更新は、そのパッケージを作るステップと同じコミットに入れる（§1.3）。

### フェーズ 1: HTTP 通信の手順の切り出し（`internal/llm/llmhttp`）

**対象ファイル**
- 新設: `internal/llm/llmhttp/llmhttp.go`・`llmhttp_test.go`（`//go:build test`）
- 新設: `internal/llm/llmhttp/llmhttptest/llmhttptest.go`（`//go:build test`、パッケージ `llmhttptest`）
- 変更: `internal/llm/deepseek/deepseek.go`・`response.go`・`errors.go`、`internal/pipeline/pipeline_test.go`、`docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] **ステップ 1-1**: `llmhttp.go` に architecture §3.4 の `MaxResponseBytes`・`NewClient`・`Errors`・`Call`・`Post` を実装する。手順は §1.3 に挙げた DeepSeek アダプタの処理を、振る舞いを変えずに移す。エラーメッセージの文言も変えない。これに加えて、architecture §3.4 の「リダイレクトの防止を呼び出し側に頼らない」と「不完全な `Call` では送らない」の 2 つを実装する。import は標準ライブラリだけとし、`internal/secret` を import しない。
- [ ] **ステップ 1-2**: `llmhttptest.go` に、`llmhttp` と `claude` のテストが使うテストサーバーの部品を作る（I-02）。`llmhttptest` は `llmhttp` を import しない（`llmhttp_test.go` が同じパッケージのテストとして `llmhttptest` を使うため）。部品は次のとおり。
  - `*testing.T` を受け取る部品: リクエストを記録するサーバー（件数と、メソッド・URL・ヘッダー・本文）、固定のステータスと本文を返すサーバー、ヘッダーを返さずに待つサーバー、ヘッダーと本文の一部を返して待つサーバー、`200` 以外のステータスとヘッダーを返して本文を送らずに待つサーバー、最初のリクエストを知らせて待つサーバー、本文の途中で閉じるリスナー、接続を受けてすぐ閉じるリスナー。作った時点で `t.Cleanup` に後始末を登録し、待つ部品はハンドラを解放してからサーバーを閉じる。
  - `TestMain` から使う部品: 接続を受けてすぐ閉じるリスナーを、`*testing.T` を取らずに作る関数（エラーを返す）と、閉じるメソッド・受け付けた件数を返すメソッド。DeepSeek の `startBlackholeProxy`・`stop`（`internal/llm/deepseek/test_helpers.go:395-427`）と同じ形で、`TestMain` は `m.Run()` の後に閉じる。
  - `internal/pipeline/pipeline_test.go` の `testOnlyPackageDirs` に `internal/llm/llmhttp/llmhttptest` を加え、`package_reference.md` に `internal/llm/llmhttp/llmhttptest` の行を加える。
- [ ] **ステップ 1-3**: `llmhttp_test.go` に architecture §7.1 の `llmhttp` のテストを作る。
  - `Post` の分類: `200` 以外、構築時のタイムアウト、呼び出し元の期限、実行中のキャンセル、送信前に終わっている `ctx`、接続の失敗、本文の途中の切断、上限ちょうどと上限 + 1 バイト。分類は、`Errors` に渡したテスト用の番兵と `context` のエラーに対して `errors.Is` で判定し、該当するものが 1 つだけであることを確かめる。
  - `200` 以外: 本文に埋め込んだ目印がエラーに現れないこと、`Errors.Status` が受け取ったステータスコード。本文を送らずに待つサーバーに対して、`Post` が構築時のタイムアウトより十分前に戻ることで、本文を読まないことを確かめる。
  - 送らないことの検査: 送信前に終わっている `ctx` と不完全な `Call` は、記録するサーバーの件数だけでは判定できない（`http.Transport` は終わった `ctx` では接続しない）。`NewRequest` の呼び出し回数と、`Client` に差し込んだ記録する `RoundTripper` の呼び出し回数が 0 であることで確かめる。
  - 不完全な `Call`: `Timeout` 0 以下、`NewRequest` が `nil`、`Errors` の各フィールドが `nil`、`NewRequest` が別の `ctx` でリクエストを作った場合のそれぞれで、送らないことと、返るエラーが不完全な `Call` を表すエラーであり `context.DeadlineExceeded` ではないことを確かめる。
  - リダイレクト: リダイレクトに従う設定の `Client` と `nil` の `Client` のどちらでも、`307` の `Location` 先へ送らないこと。
  - `NewClient` の `Transport` が `nil` であること。
- [ ] **ステップ 1-4**: DeepSeek アダプタの送信を `llmhttp.Post` の呼び出しに置き換える。`client.httpClient` は `llmhttp.NewClient()` の値にし、`maxResponseBytes` は `llmhttp.MaxResponseBytes` を指す定数として残す。§1.4 の表の 4 つの非公開の要素を削除する。`Errors.Status` には、既存の `statusError` を呼ぶ関数を渡す。architecture §3.4 の表の非公開の名前（`client`・`client.httpClient`・`client.endpoint`・`errorPrefix`・`maxResponseBytes`・`keyModel`・`keyChoices`・`maxReasonBytes`）を残す。古くなるコメントを次のとおり直す。
  - `deepseek.go:25-26`: 変更前 `// maxResponseBytes caps the response body. The adapter reads one byte` / `// past the limit to detect an oversized body without buffering it all.` → 変更後 `// maxResponseBytes is the response body cap that llmhttp.Post enforces;` / `// the tests refer to it by this name.`
  - `errors.go:23-25`: 変更前 `// Static errors for constructing a client and for failure paths that have no` / `// public sentinel. errAdapterTimeout is the cause of the deadline the adapter` / `// adds to the caller's context.` → 変更後 `// Static errors for constructing a client and for failure paths that have no` / `// public sentinel.`
- [ ] **ステップ 1-5**: DeepSeek アダプタの既存のテストを変更せずに `make test` が通ることを確認する。このコミットの差分に `internal/llm/deepseek/*_test.go`・`test_helpers*.go` が含まれないことを、コミット前に差分の一覧で確認する（H-01）。
- [ ] **ステップ 1-6**: `package_reference.md` に `internal/llm/llmhttp` の行を加え、`internal/llm/deepseek` の行に、HTTP の送信を `llmhttp` で行うことを加える。
- [ ] **ステップ 1-7**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: `Post` が `Client` の写しにリダイレクトを追わない設定を加えない（`TestPostNoRedirect` の、リダイレクトに従う `Client` のケース）、`Call` の検査をそれぞれ外す（`TestPostIncompleteCall` の該当行。返るエラーの判定と `RoundTripper` の呼び出し回数）、上限 + 1 バイトの判定を外す（`TestPostSizeLimit`）、送信前の `ctx` の確認を外す（`TestPostCanceled` の送信前に終わっている `ctx` のケース。`NewRequest` の呼び出し回数）、`200` 以外でも本文を読んでから `Errors.Status` を呼ぶ（`TestPostNon200` の本文を送らないサーバーのケース）、分類で呼び出しの `ctx` を先に確かめない（`TestPostTimeout` の本文の途中で待つサーバーのケース）。`make fmt` → `make test` → `make lint` を通し、ここまでを 1 つのリファクタリングのコミットにする（H-01）。
- [ ] **ステップ 1-8**: 別のコミットで、`Post` の構築時のタイムアウトと `Errors.Transport` のエラーのメッセージに、送信を始めてからの経過時間を加える（architecture §3.4「経過時間の記録」）。呼び出し元の期限とキャンセルのメッセージには加えない（architecture §3.4 の対象外）。経過時間は整数のミリ秒で表し、`Duration.String()` の秒の表記（例 `1.2s`）を使わない。表記が `2s` を含むと、DeepSeek の `deepseek_test.go:514` の検査（メッセージがアダプタのタイムアウトの値 `2s` を含まない）が失敗するためである（§6）。経過時間がメッセージに含まれることを検査するテストを `llmhttp_test.go` に加え、経過時間の追加を外すと失敗することを確認する。DeepSeek のテストを変更せずに `make test` が通ることを確かめ、`make fmt` → `make test` → `make lint` を通す。

### フェーズ 2: effort とワークスペース ID の型（`internal/llm/claudeparam`）

**対象ファイル**
- 新設: `internal/llm/claudeparam/claudeparam.go`・`claudeparam_test.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] **ステップ 2-1**: `claudeparam.go` に architecture §3.2 の `Effort`・定数・`ParseEffort`・`String`・`WorkspaceID`・`ParseWorkspaceID`・`Value` と、§1.3 の `Valid` を実装する。`ParseEffort` と `String` は `switch` で書き、`default` を、`ParseEffort` では拒否、`String` では固定の文字列にする。5 つの値の一覧はこのファイルの外に持たない。import は標準ライブラリだけとする。
- [ ] **ステップ 2-2**: `claudeparam_test.go` に `TestParseEffort`（5 つの値の受理と `String` との往復、requirements AC-25 の例を含む拒否、空文字列の拒否）、`TestEffortStringAndValid`（`EffortUnset` と範囲外の値の `String` は `ParseEffort` が受理しない値であり、`Valid` が偽であること）、`TestParseWorkspaceID`（境界の `0x21`・`0x7E` の受理、requirements AC-39・AC-42 の例と空文字列・`0x7F` の拒否）、`TestWorkspaceIDZeroValue`（ゼロ値の `Value` が「指定なし」を返すこと）、`TestWorkspaceIDHasNoExportedFields`（`reflect` で `WorkspaceID` が公開のフィールドを持たないこと。AC-39 は、不正な値を `ParseWorkspaceID` 以外では作れないことに依拠するため）を作る。
- [ ] **ステップ 2-3**: `package_reference.md` に `internal/llm/claudeparam` の行を加える。
- [ ] **ステップ 2-4**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: `ParseEffort` が大文字を受理する（`TestParseEffort`）、`Valid` が `EffortUnset` に真を返す（`TestEffortStringAndValid`）、`ParseWorkspaceID` が空白を受理する・空文字列を受理する（`TestParseWorkspaceID`）、`WorkspaceID` のフィールドを公開にする（`TestWorkspaceIDHasNoExportedFields`）。`make fmt` → `make test` → `make lint` を通す。

### フェーズ 3: Claude アダプタ（`internal/llm/claude`）

**対象ファイル**
- 新設: `internal/llm/claude/claude.go`・`request.go`・`response.go`・`errors.go`
- 新設: `internal/llm/claude/test_helpers.go`・`test_helpers_endpoint.go`（`//go:build test`）、`claude_test.go`・`response_test.go`（`//go:build test`）
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] **ステップ 3-1**: `errors.go` に architecture §4.1 の番兵・`HTTPStatusError`（`Error` は §4.2 の表の案内文をステータスコードから決める）と、構築時の非公開の静的エラーを作る。
- [ ] **ステップ 3-2**: `request.go` に、API キーの形の検査（`Reveal()` の 1 か所目）、リクエスト本文の非公開の構造体と組み立て（architecture §3.7）、ヘッダーの設定（`Reveal()` の 2 か所目。`anthropic-workspace-id` は `WorkspaceID.Value()` が指定ありを返すときだけ）を作る。送信先、`anthropic-version`、`MaxOutputTokens` が 0 のときの `max_tokens` は、architecture §3.6 の値を名前付きの非公開の定数にする。リクエストの作成には、DeepSeek の `request.go:82` と同じく、その 1 行に限った `//nolint:gosec` と理由のコメントを付ける（gosec が指摘した場合）。
- [ ] **ステップ 3-3**: `claude.go` に `Options`・`New`・非公開の `client`・`Generate` を作る。`New` は architecture §3.1 の検査を行い、effort は `Valid` で検査する。`client` は `httpClient`（`llmhttp.NewClient()` の値）と `endpoint` を持つ。`Generate` は `req.Validate()` → 本文の組み立て → `llmhttp.Post` → 応答の検証の順に呼び、すべてのエラーの先頭に `claude: ` を 1 回だけ付ける。`Errors.Status` に渡す関数は、architecture §4.2 のとおり、`HTTPStatusError` を `%w` で包んだエラーを返す。そのメッセージには、モデル名・送った `max_tokens`・effort・ワークスペース ID を指定したかどうかを加える。`HTTPStatusError` 自体は `StatusCode` だけを持つ。この形は DeepSeek の `statusError`（`deepseek.go:179-185`）と同じである。
- [ ] **ステップ 3-4**: `response.go` に、architecture §3.5 の応答本文の検証と `GenerateResponse` の組み立てを作る。検証の順序は requirements F-003 のとおりとし、`ModelVersion` は空文字列にする。`llm.ErrTruncated`・`llm.ErrUnexpectedFinishReason`・`ErrInvalidResponse` のメッセージは architecture §4.2 に従う（`stop_reason` は 64 バイトで切って引用し、`text` ブロックのないまま `max_tokens` で打ち切られた場合は effort を下げる案内を加える）。
- [ ] **ステップ 3-5**: `test_helpers_endpoint.go` に、DeepSeek の `NewForLoopbackTest`（`internal/llm/deepseek/test_helpers_endpoint.go`）と同じ形の、ループバックの送信先だけを受け付けるテスト用の構築を作る（H-08）。`test_helpers.go` に、テスト用の構築の呼び出し、フィクスチャのパス、共有のアサーション（返るエラーに一致する番兵が 1 つだけ、`claude: ` が 1 回だけ、`Error()`・`%v`・`%+v`・`%#v` に API キーが現れない、拒否時の `GenerateResponse` がゼロ値）を置く。テストサーバーは `llmhttptest` の部品を使う。フィクスチャの読み込みには、DeepSeek の `test_helpers.go:79` と同じ 1 行に限った `//nolint:gosec` を付ける（`test_helpers*.go` は `_test.go` ではないため、テスト向けの lint の除外が効かない）。
- [ ] **ステップ 3-6**: `claude_test.go` に、DeepSeek の `deepseek_test.go:36` と同じ `TestMain`（プロキシの環境変数を `llmhttptest` のすぐ閉じるリスナーに向け、`NO_PROXY` を消す）と、§5 の `claude_test.go` のテストを作る。対象は構築（AC-01・AC-02）、送信するリクエスト（AC-03〜AC-07・AC-40・AC-44）、不正なリクエスト（AC-08）、リダイレクト（AC-09）、ステータス（AC-11・AC-16 の後半）、タイムアウト・キャンセル・通信の失敗（AC-18〜AC-20）、値の出力（AC-22）、番兵の区別（AC-17）、`Reveal()` の呼び出し箇所、本番の送信先に届かないこと（AC-31）、ステータスごとの案内文である。`Reveal()` の呼び出し箇所の guard は、DeepSeek の `TestRevealOnlyInRequestFile`（`deepseek_test.go:634`）と同じ形にする。送信するリクエストのテストでは、`messages` が要素 1 つで、その `role` が `user`、`content` が（ブロックの配列でなく）JSON 文字列であり、それ以外のメンバーを持たないことも確かめる（AC-04）。ステータスのテストでは、`200` 以外の応答の本文に目印を埋め込み、`Generate` のエラーに現れないことを確かめる（AC-11）。
- [ ] **ステップ 3-7**: `response_test.go` に、§5 の `response_test.go` のテストを作る（AC-10・AC-12〜AC-16・AC-32〜AC-38）。入力はフィクスチャを基に 1 か所だけを書き換えたものとし、上限のテストは `llmhttp.MaxResponseBytes` を参照する（H-09）。拒否のケースはすべて共有のアサーションを使う（AC-21）。
- [ ] **ステップ 3-8**: `package_reference.md` に `internal/llm/claude` の行を加える。
- [ ] **ステップ 3-9**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: `New` の effort の検査を外す（`TestNew`）、`anthropic-workspace-id` を常に送る・送らない（`TestGenerateWorkspaceHeader`）、本文に `thinking` のフィールドを加える（`TestGenerateSendsRequest` のトップレベルのメンバーの検査）、`content` の要素の `switch` の `default` を受理にする（`TestGenerateContentShape`）、2 つ目の `text` ブロックを受理する（同）、`stop_reason` の判定を生成テキストの判定の後にする（`TestGenerateValidationOrder`）、`thinking` ブロックの `thinking` を `Text` に連結する（`TestGenerateThinkingBlocks`）、`Reveal()` を `claude.go` から呼ぶ（`TestRevealOnlyInRequestFile`）。`make fmt` → `make test` → `make lint` を通す。

### フェーズ 4: 設定・プロバイダの選択・CLI

**対象ファイル**
- 変更: `internal/config/config.go`・`config_test.go`・`envaccess_test.go`
- 変更: `internal/llm/provider/provider.go`・`provider_test.go`
- 変更: `cmd/yt2column/run.go`・`run_test.go`
- 変更: `internal/pipeline/pipeline_test.go`・`docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] **ステップ 4-1**: 最初のコミットで、振る舞いを変えない改名だけを行う。§1.4 の表のとおり `apiKeyEnv`・`loadAPIKey` を改名し、`config_test.go` の参照を合わせる。このコミットの `config_test.go` の差分が識別子の置き換えだけで、期待値・アサーションを変えないことを、コミット前に差分で確認してコミットメッセージに書く（AC-27）。
- [ ] **ステップ 4-2**: `config.go` を architecture §3.2 のとおりに変える。`ProviderClaude`、3 つの変数名の定数、`AnthropicAPIKey`・`ClaudeEffort`・`AnthropicWorkspaceID` を加える。プロバイダ固有の変数の読み込みは `switch cfg.provider` で分け、`default` は何も読まない。effort とワークスペース ID の変換は `claudeparam` の関数で行う。新しい拒否の理由は定数とし、値を含めない。`ANTHROPIC_API_KEY` の変数名の定数には、`config.go:30` と同じ 1 行に限った `//nolint:gosec` を付ける。既存の文字列とコメントは次のとおり変える。
  - `reasonProvider`（`:52`）: 変更前 `` `must be exactly "deepseek"` `` → 変更後 `` `must be exactly "deepseek" or "claude"` ``
  - `:37`: 変更前 `// providerDeepSeek is the only accepted value of providerEnv.` → 変更後 `// The accepted values of providerEnv.`（`providerDeepSeek` と、新設の `claude` の値の定数を 1 つの const ブロックにまとめる）
  - `:40-41`: 変更前 `// The GODEBUG values that turn on the Go HTTP/2 transport's verbose log, which` / `// writes the Authorization header to standard error.` → 変更後 `// The GODEBUG values that turn on the Go HTTP/2 transport's verbose log, which` / `// writes every request header, the API key headers included, to standard error.`
  - `loadDeepSeekAPIKey` の doc（旧 `:199-201`）: 変更前 `// loadAPIKey validates DEEPSEEK_API_KEY, which is required when the provider is` / `// deepseek. An empty value is rejected however the provider is set; a non-empty` / `// value is kept only for deepseek.` → 変更後 `// loadDeepSeekAPIKey validates DEEPSEEK_API_KEY, which is required when the` / `// provider is deepseek. It is called only for that provider, so the variable is` / `// never read, and never rejected, for another provider.`
- [ ] **ステップ 4-3**: `config_test.go` を更新する。`:173` の不正なプロバイダの例から `claude` を除いて `Claude`・`anthropic` を加え、§5 の `config_test.go` のテスト（AC-23〜AC-26・AC-41・AC-42・AC-45）を加える。
  - `TestLoadIgnoresOtherProviderVars` のプロバイダが不正な行は、Claude の 3 つの変数に空・不正な値を入れる（正しい値ではエラーが増えないため、読んだかどうかを区別できない）。
  - 拒否した値が現れないことの検査（AC-25・AC-42）は、`TestLoadErrorsOmitValues`（`:276`）に、プロバイダが `claude` の環境を基にした行として加える（既存の行の基の環境はプロバイダが `deepseek` で、Claude の変数を読まない）。
  - Anthropic の API キーが `Config` の出力に現れないことは、プロバイダが `claude` の環境で読み込み、まず `AnthropicAPIKey()` がゼロ値でないことを確かめてから検査する（`TestConfigOutputRedactsSecrets`（`:353`）に Claude の環境の行を加える）。
- [ ] **ステップ 4-4**: `envaccess_test.go` の `secretEnvNames` に `ANTHROPIC_API_KEY` を加える（architecture §5.2）。`internal/pipeline/pipeline_test.go` に、`TestWriterImports`（`:746`）と同じ形で `TestConfigImports` を加え、`internal/config` と `internal/llm/claudeparam` の非テストの Go ファイルが、`net`・`net/*` と、`internal/llm/claudeparam` 以外の `internal/llm/*` を直接 import しないことを固定する（AC-31 の設定の部分。§1.3）。
- [ ] **ステップ 4-5**: `provider.go` を architecture §3.3 のとおりに変える（`builders`、`newClient(provider, cfg, build)`、`ProviderClaude` の分岐）。コメントを次のとおり変える。
  - `LLMTimeout` の doc: 変更前 `// LLMTimeout bounds one LLM call. DeepSeek holds a request for up to ten` / `// minutes before inference starts; the remaining five minutes cover the` / `// generation itself.` → 変更後 `// LLMTimeout bounds one LLM call, for every provider. DeepSeek holds a request` / `// for up to ten minutes before inference starts; the remaining five minutes` / `// cover the generation itself. A non-streaming Claude call within the Claude` / `// adapter's default output limit was measured to finish well inside it.`
  - `newClient` の doc（`:32-36`）: 変更前 `// newClient builds the adapter for provider. It takes the provider directly so` / `// a package test can exercise the unknown-provider branch, which a Config from` / `// Load cannot produce. build is the adapter constructor: production passes` / `// deepseek.New, a test passes the loopback constructor or a spy. An unknown` / `// provider returns errUnknownProvider without calling build.` → 変更後 `// newClient builds the adapter for provider. It takes the provider directly so` / `// a package test can exercise the unknown-provider branch, which a Config from` / `// Load cannot produce. build holds the adapter constructors: production passes` / `// deepseek.New and claude.New, a test passes loopback constructors or spies. An` / `// unknown provider returns errUnknownProvider without calling any of them.`
- [ ] **ステップ 4-6**: `provider_test.go` を更新する。`:132`・`:178` は新しい `newClient` の形への呼び出しの置き換えだけとし、`TestNewUnknownProvider` はどちらの構築関数も呼ばれないことを確かめる形にする。`TestMain`（ステップ 1-2 の `TestMain` から使う部品で、ステップ 3-6 と同じ設定）を加え、§5 の `provider_test.go` のテスト（AC-23・AC-26・AC-41・AC-31）を加える。Claude の分岐のテストは `claude.NewForLoopbackTest` を構築関数に渡し、送られたリクエストを記録して確かめる。
- [ ] **ステップ 4-7**: `run.go` の `configuredSecrets` に `cfg.AnthropicAPIKey()` を加える（architecture §5.2）。`run_test.go` に、プロバイダが `claude` の環境から読み込んだ `configuredSecrets` で、Anthropic の API キーとその末尾 8 文字を含む行が伏せ字化されることを確かめるテスト（`TestConfiguredSecretsIncludesAnthropicAPIKey`）を加える。
- [ ] **ステップ 4-8**: `package_reference.md` の `internal/config`・`internal/llm/provider` の行を、`claude` プロバイダと `builders` に合わせて更新する。
- [ ] **ステップ 4-9**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: `loadDeepSeekAPIKey` をプロバイダによらず読む形に戻す（`TestLoadIgnoresOtherProviderVars`・`TestLoadClaudeIgnoresDeepSeekKey`）、Claude の変数をプロバイダが不正なときにも読む（`TestLoadIgnoresOtherProviderVars` のプロバイダが不正な行）、ワークスペース ID の空を受理する（`TestLoadClaudeInvalid`）、`provider` の Claude の分岐でワークスペース ID を渡さない（`TestNewClaudeSendsConfiguredRequest`）、`configuredSecrets` から Anthropic のキーを外す（`TestConfiguredSecretsIncludesAnthropicAPIKey`）、`secretEnvNames` の `ANTHROPIC_API_KEY` を残したまま `cmd/yt2column` の本番のファイルに文字列 `"ANTHROPIC_API_KEY"` を書く（`TestEnvAccessConfined`）、`internal/config` に `internal/llm/claude` の import を加える（`TestConfigImports`）。`make fmt` → `make test` → `make lint` を通す。

### フェーズ 5: Make の部品の移動・統合テスト・Make ターゲット

**対象ファイル**
- 新設: `internal/maketestutil/make.go`・`make_test.go`（`//go:build test`）
- 変更: `internal/llm/deepseek/testutil/make.go`・`make_test.go`・`integration.go`
- 新設: `internal/llm/claude/testutil/integration.go`（`//go:build test || integration`、パッケージ `claudetestutil`）・`integration_settings_test.go`（`//go:build test`）
- 新設: `internal/llm/claude/integration_env_test.go`（`//go:build test || integration`）・`integration_test.go`（`//go:build integration`）・`makefile_test.go`（`//go:build test`）
- 変更: `internal/llm/claude/claude_test.go`、`Makefile`、`cmd/yt2column/makefile_test.go`、`internal/pipeline/pipeline_test.go`、`docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] **ステップ 5-1**: `internal/maketestutil/make.go` に、§1.4 の表の部品を移す。子プロセスの環境に渡す変数、記録する変数、表示の文言、モデル名などの変数と既定値は、呼び出し側が指定する。`CheckChargedTarget` の引数・`-timeout`・オプトインの検査も、DeepSeek と Claude の両方が使える形でここに置く。`make_test.go` に `TestValidateEnvNames`・`TestRecordedNames` を汎用の形で移す。`FirstLineIs` のエラーの経路は `deepseek_test.go:801-814` が既に検査しているため、`maketestutil` には同じテストを加えない。同じコミットで、`internal/pipeline/pipeline_test.go` の `testOnlyPackageDirs` に `internal/maketestutil` を加え、`package_reference.md` に行を加える。
- [ ] **ステップ 5-2**: 同じコミットで、`deepseektestutil` の `make.go` を、公開の形（§1.4）を変えずに `maketestutil` を呼ぶだけのラッパーにする。`deepseektestutil/make_test.go` の中身を、ラッパーが記録する変数（`DeepSeekOptInEnv`・`CLIOptInEnv`・`ModelEnv` の後に呼び出し側の変数が重複なく続くこと）を固定するテストに入れ替える。`TestMakeOptInsAreTargetSpecific` が他のターゲットのオプトインを検出できるのは、この記録に依存するためである。移す前に `deepseektestutil` の `go tool cover -func` を取り、移した後に `maketestutil` と `deepseektestutil` の同じ出力を取る。移した関数とラッパーの関数の網羅率が下がっていないことを確認し、コミットメッセージに書く（CLAUDE.md「Deleting a test」）。`cmd/yt2column`・`internal/publisher`・`internal/llm/deepseek` のテストを変更せずに通ることを確かめる。`package_reference.md` の `internal/llm/deepseek/testutil` の行を、`RunMakeTarget`・`FirstLineIs` が `maketestutil` のラッパーであることに合わせて更新する。
- [ ] **ステップ 5-3**: `claudetestutil` の `integration.go` に、統合テストの環境変数の名前と、実行するかどうかを決める純粋な関数を作る。関数は `getenv func(string) string` を受け取り、architecture §7.2 の判定の順序（オプトイン → API キー → モデル名・effort → ワークスペース ID → `http2debug`）で、スキップ・失敗・実行と理由、および実行時の API キー・モデル名・effort・ワークスペース ID を返す。effort とワークスペース ID は `claudeparam`、`http2debug` は `config.HTTP2DebugEnabledIn` で判定する。理由の文字列は変数名を含み、API キーを含まない。API キーの変数名の定数には、`internal/llm/deepseek/testutil/integration.go:25` と同じ 1 行に限った `//nolint:gosec` を付ける。`integration_settings_test.go` に `TestIntegrationSettings` を作り、判定の各分岐（オプトインの欠如と `1` 以外の値、API キーの未設定・空でのスキップ、`ANTHROPIC_API_KEY` だけではスキップ、モデル名・effort の未設定・空・不正な effort での失敗、ワークスペース ID の空・不正での失敗と未設定での指定なし、`ANTHROPIC_WORKSPACE_ID` だけでは指定なし、`http2debug=1`・`=2` での失敗、すべてそろったときの値）を検証する。同じコミットで `TestFakesCarryBuildTag` の件数を 17 にし、`package_reference.md` に `internal/llm/claude/testutil` の行を加える。
- [ ] **ステップ 5-4**: `integration_env_test.go` に、統合テストの判定の設定と、architecture §3.6 の値（1 回の `Generate` のタイムアウト、正常な生成と打ち切りの `MaxOutputTokens`）と `Generate` の回数（2）を定数として置く。テスト関数は置かない。`claude_test.go` に、DeepSeek の `TestIntegrationOptionsSkipMissingKey`（`deepseek_test.go:777`）と同じ形の `TestIntegrationOptionsSkipMissingKey` を加え、この設定でオプトインがあり API キーがないときにスキップになることを確かめる。
- [ ] **ステップ 5-5**: `integration_test.go` に、正常な生成と打ち切りの 2 つのサブテストを持つ `TestIntegrationGenerate` を作る（AC-30）。判定はステップ 5-3 の関数で行い、ワークスペース ID を指定したときは構築に渡す（AC-43）。プロンプトは短い固定の英文とする。`Model` に API キーが含まれないことを確かめてから、`Model` をエスケープしてログに出す（architecture §7.2）。打ち切りのサブテストがエラーなしで終わった場合は、前提が崩れたことを示すメッセージで失敗させる。
- [ ] **ステップ 5-6**: `Makefile` に `test-integration-claude` を加える。`test-integration-deepseek`（`Makefile:92-110`）と同じ形で、architecture §7.2 の既定のモデル・effort を `?=` で定義し、このターゲットにだけ、この 2 つとオプトイン `YT2COLUMN_CLAUDE_INTEGRATION := 1` をエクスポートする。実 API を使い料金が発生することと、モデル名・effort を表示する。レシピは `-tags integration -count=1 -timeout $(CLAUDE_INTEGRATION_TIMEOUT) -v ./internal/llm/claude` で `$(GOTEST)` を呼び、`CLAUDE_INTEGRATION_TIMEOUT ?= 15m` とする（I-03）。`.PHONY` と `lint` の前のコメントの一覧に加える。
- [ ] **ステップ 5-7**: `internal/llm/claude/makefile_test.go` に `TestMakeTestIntegrationClaude` を作る。ステップ 5-1 の検査でターゲットを実行し、表示、`go test` の引数（`-tags integration`・`-count=1`・`-timeout`・`-v`、パッケージのパスが最後の単独の引数であること）、`-timeout` がステップ 5-4 の回数 × タイムアウトより長いこと、オプトイン、モデル名・effort の変数が未定義なら既定値・空ならそのまま・値があればその値になることを検証する（AC-29）。
- [ ] **ステップ 5-8**: `claude_test.go` に `TestIntegrationTestBuildTag` を加え、`integration_test.go` の先頭行が `//go:build integration`、`integration_env_test.go` の先頭行が `//go:build test || integration` であることを `maketestutil.FirstLineIs` で固定する（AC-28）。
- [ ] **ステップ 5-9**: `cmd/yt2column/makefile_test.go` の `TestMakeOptInsAreTargetSpecific`（`:61-90`）に、`test-integration-claude` のターゲットと `YT2COLUMN_CLAUDE_INTEGRATION` を加える。`YT2COLUMN_CLAUDE_INTEGRATION` は記録する変数にも加える（architecture §3.8）。
- [ ] **ステップ 5-10**: `internal/llm/deepseek/testutil/integration.go` の http2debug のコメント（`:33-37`）を次のとおり変える。
  - 変更前: `// http2VerboseSettings are the GODEBUG settings that make the HTTP/2 transport` / `// log every request header, Authorization included, to standard error. The`
  - 変更後: `// http2VerboseSettings are the GODEBUG settings that make the HTTP/2 transport` / `// log every request header, the API key header included, to standard error. The`
  - 続く 3 行（`// transport reads GODEBUG once ...` 以降）は変えない。
- [ ] **ステップ 5-11**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: 判定で API キーの未設定を失敗にする（`TestIntegrationSettings` のスキップの行と `TestIntegrationOptionsSkipMissingKey`）、ワークスペース ID の空を指定なしにする（`TestIntegrationSettings`）、`http2debug` の判定を外す（同）、Makefile の `-count=1` を外す・オプトインのエクスポートを外す（`TestMakeTestIntegrationClaude`）、オプトインを全体にエクスポートする（`TestMakeOptInsAreTargetSpecific`）、`deepseektestutil` のラッパーが `DeepSeekOptInEnv` を記録しない（`deepseektestutil/make_test.go` のテスト）、`integration_test.go` の先頭行を変える（`TestIntegrationTestBuildTag`）。`make fmt` → `make test` → `make lint` を通す（`make lint` は `go vet -tags integration ./...` で統合テストもビルドする）。
- [ ] **ステップ 5-12**: 料金の発生しない確認として、`YT2COLUMN_TEST_ANTHROPIC_API_KEY` を設定せずに `make test-integration-claude` を実行し、スキップの行が変数名を示すことを §5.1 に記録する。続けて、人間の承認を得てから、`YT2COLUMN_TEST_ANTHROPIC_API_KEY` を設定して実行し、結果を §5.1 に記録する（AC-29・AC-30・AC-43。実 API を呼び料金が発生する）。

### フェーズ 6: ドキュメント

**対象ファイル**
- 変更: `README.md`・`docs/dev/project_overview.md`・`docs/dev/security.md`・`CLAUDE.md`・`AGENTS.md`・`docs/dev/developer_guide/package_reference.md`・`cmd/yt2column/docs_test.go`

**タスク**
- [ ] **ステップ 6-1**: `README.md` を更新する。設定の表（`YT2COLUMN_LLM_PROVIDER` に `claude`、`ANTHROPIC_API_KEY`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID` の行、プロバイダ固有の変数は選んだプロバイダのときだけ検査すること）、effort `max` の注意（architecture §3.6）、`make test-integration-claude` の説明（テスト用の変数、既定のモデル・effort、料金）を書く。
- [ ] **ステップ 6-2**: `docs_test.go` の `configDocRows` に、`ANTHROPIC_API_KEY`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID` の行を加える。期待する「未設定のとき」の文言は、requirements F-006 の規則を、既存の行（`docs_test.go:34-39`）と同じ書き方で表したものにする（コードから取らない。`docs_test.go:24-26` の方針）。project_overview の欄の期待値は、既存の行と同じく日本語の文字列リテラルになる。これは日本語の表の欄に一致させる guard のリテラルであるため、既存の行の前例に従う。README と project_overview の表の更新と同じコミットにする。同じファイルに、`TestSlackDocsContract`（`:174`）と同じ形で、README と `security.md` が `YT2COLUMN_TEST_ANTHROPIC_API_KEY`・`YT2COLUMN_TEST_ANTHROPIC_WORKSPACE_ID`・`YT2COLUMN_CLAUDE_INTEGRATION`・`test-integration-claude` を記載していることを固定する `TestClaudeDocsContract` と、`TestPlanRecordsManualRuns`（`:146`）と同じ形で、本計画のステップ 5-12 にチェックが入っていれば §5.1 に実行日・HEAD・モデル名・`--- PASS` の記録があることを固定する `TestClaudePlanRecordsManualRun` を加える。
- [ ] **ステップ 6-3**: `project_overview.md` を更新する。`:43` と `:79-82` のパッケージ構成（`internal/llm/claude`・`claudeparam`・`llmhttp` を実装済みにする）、`:95-97` の設定の表、`:104` の後に「選択したプロバイダに関係しない変数は読まず、検査しない」原則（requirements §5.1）を書く。
- [ ] **ステップ 6-4**: `security.md` を更新する。§2 の統合テストの例外の表に Claude の統合テストの行を加え、テスト用のキーは利用上限を設けたワークスペースのものにすることを書く。http2debug の説明（`:40`）に `x-api-key` を加える。§4 に、Anthropic の API へ送るデータの扱い（API の入出力をモデルの学習に使うかどうか、保存期間）を書く。§4 の記述は Anthropic の公開文書を確かめて書き、その URL と確認した日付を本文に記す（ネットワークを使うため、確認の前に人間の承認を得る）。
- [ ] **ステップ 6-5**: `CLAUDE.md` の Architecture Overview に `internal/llm/llmhttp`・`internal/llm/claude` を加え、`internal/strictjson` の説明（`:94-95`）の利用者に `internal/llm/claude` を加え、Development Notes の秘密情報の例（`:210`）に `ANTHROPIC_API_KEY` を加える。`AGENTS.md:10` の秘密情報の例にも `ANTHROPIC_API_KEY` を加える。
- [ ] **ステップ 6-6**: `package_reference.md` の `internal/strictjson` の行（`:18`）の利用者に `internal/llm/claude` を加え、各行が実装と一致することを確認する（行の追加はフェーズ 1〜5 で済んでいる）。
- [ ] **ステップ 6-7**: `testdata/README.md` の Claude の節（コミット `9e7d0db`）が、フィクスチャの内容と architecture §1.4 の記録に一致することを確認する。
- [ ] **ステップ 6-8**: フェーズ 6 で書いた各記述を根拠と照合し、照合した根拠（コードの場所、`make -n test-integration-claude` の出力、§5.1 の記録、Anthropic の文書の URL）をコミットメッセージに書く。
- [ ] **ステップ 6-9**: `make fmt` → `make test` → `make lint` を通す。CLAUDE.md を変えるため、`make ext-test` も通す（CLAUDE.md「Development Notes」）。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 | 完了条件 |
|---|---|---|---|
| M1 | フェーズ 1 | `llmhttp`・`llmhttptest`・`llmhttp_test.go`、DeepSeek アダプタの書き換え（リファクタリングのコミット）、経過時間の追加（独立したコミット） | `make test`・`make lint` が通り、DeepSeek のテストが無変更で通る |
| M2 | フェーズ 2 | `claudeparam` とそのテスト | 同上 |
| M3 | フェーズ 3 | `internal/llm/claude` とユニットテスト | 同上 |
| M4 | フェーズ 4 | `config`・`provider`・`cmd/yt2column` の変更とテスト | 同上 |
| M5 | フェーズ 5 | `maketestutil`、`claudetestutil`、統合テスト、Make ターゲット、§5.1 の記録 | 同上・`make test-integration-claude` が通る |
| M6 | フェーズ 6 | README・project_overview・security・CLAUDE.md の更新、`docs_test.go` の行 | 同上・`make ext-test` が通り、根拠との照合を済ませた |

### 3.2. 実装順序の根拠

architecture §8 の順序に従う。`llmhttp` は Claude アダプタが使うため最初に置き、DeepSeek アダプタの無変更のテストで振る舞いの保存を確かめられる独立したリファクタリングとする（H-01）。`claudeparam` はアダプタと `internal/config` の両方が使うため、アダプタより前に置く。アダプタが揃ってから `config`・`provider`・CLI で選べるようにし、その後で統合テストと Make ターゲット、最後に実装の確定した内容で文書を更新する。PR の区切りは、本計画の承認後に `/mkplan2` で決める。

## 4. テスト戦略 (Test Strategy)

architecture §7 のテスト戦略に従う。テスト関数名と AC の対応は §5 に示す。

### 4.1. ユニットテスト

- `internal/llm/claude` と `internal/llm/provider` のテストは、`TestMain` でプロキシの環境変数を接続をすぐ閉じるローカルのリスナーに向け、送信先は `llmhttptest` のループバックのサーバーにする（AC-31）。テストの API キーとワークスペース ID は、実在しない固定の文字列とする。
- 応答の検証のテストは、フィクスチャを基に 1 か所だけを書き換えた入力を使う（I-04）。
- タイムアウト・キャンセルのテストは時間の厳密な比較をせず、「構築時のタイムアウト + 猶予 2 秒」以内に戻ることで判定する（architecture §3.6）。
- **網羅率の目標:** `internal/llm/llmhttp`・`internal/llm/claudeparam`・`internal/llm/claude` の本番コードの、到達できるすべての文を、それぞれのパッケージのテストが実行すること（文の網羅率）。`go test -tags test -coverprofile` の出力で網羅されていない文を確かめ、到達できない文が残る場合は、その文と理由をコミットメッセージに書く。
- **後方互換性:** DeepSeek アダプタのテストはフェーズ 1 で変更しない（ステップ 1-5）。`internal/config` の既存テストは、ステップ 4-1 の改名だけのコミットで期待値を変えずに通し、その後の変更は `:173` の不正な値の入れ替えと、既存のテストへの Claude の行の追加とする。`provider_test.go` の既存テストは、`newClient` の呼び出しの形だけを変える（AC-27）。`cmd/yt2column`・`internal/publisher` の Make のテストは、`deepseektestutil` の公開の形を変えないため変更しない（ステップ 5-2）。
- **重複の理由:** `Post` の契約（タイムアウト・キャンセル・通信の失敗・上限・`200` 以外）は、`llmhttp` のテストと各アダプタのテストの両方で検証する。requirements §1 が、共通の規則をアダプタごとに AC として定め、独立して検証できるようにしているためである（§1.3）。

### 4.2. 統合テスト

architecture §7.2 と requirements F-007 に従う。正常な生成と打ち切りを 1 つのテスト関数のサブテストとして順に実行する（`Generate` 2 回）。環境変数の判定はステップ 5-3 の純粋な関数で行い、`make test` で単体テストする。実 API での実行は人間の承認を得てから行い、結果を §5.1 に記録する。

### 4.3. セキュリティテスト

architecture §7.3 に従う。計画固有の事項は次のとおり。

- API キーの非漏洩（AC-21）は、失敗のケースを持つすべてのテストで共有のアサーションを使って確かめる（ステップ 3-5）。
- `Reveal()` の呼び出し箇所を `TestRevealOnlyInRequestFile` で固定する（ステップ 3-6）。
- リダイレクトに従わないことは、`llmhttp` のテスト（`Post` 自身の保証）と `claude` のテスト（`Generate` が `307` を返すこと）の両方で確かめる（AC-09）。
- 秘密情報の変数名の検査に `ANTHROPIC_API_KEY` を加え（ステップ 4-4）、CLI の出力の伏せ字化に Anthropic の API キーを加える（ステップ 4-7）。

### 4.4. テストヘルパー

- `internal/llm/llmhttp/llmhttptest`: `llmhttp`・`claude`・`provider` のテストが共有するテストサーバーの部品。公開の API だけで作れるが、`testutil/` ではなく architecture §7.1 の決めた場所に置く。`//go:build test` のみとし、`testOnlyPackageDirs` に加える（ステップ 1-2）。`*testing.T` を取る部品と、`TestMain` から使う部品の 2 つの形を持つ。
- `internal/llm/claude/test_helpers.go`・`test_helpers_endpoint.go`（`test_organization.md` 分類 B）: 非公開の `client` を使うため。
- `internal/maketestutil`: Make の部品。`deepseektestutil` と `claude` の `makefile_test.go` が使う。`//go:build test` のみとし、`testOnlyPackageDirs` に加える（ステップ 5-1）。`deepseektestutil/make_test.go` は残し、ラッパーが記録する変数を固定するテストを置く（ステップ 5-2）。
- `internal/llm/claude/testutil`（`claudetestutil`、分類 A）: 統合テストとユニットテストの両方が使う判定の関数のため、`//go:build test || integration` とする（`test_organization.md` の例外）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は実行可能なテスト、`static` は guard テスト・`make` ターゲットを指す。`manual` は補助的な確認であり、`test` または `static` を置き換えない。表の「共有アサーション」は、ステップ 3-5 の共有のアサーションを、そのテストの各失敗ケースで使うことを指す。パスは `internal/llm/claude/` を `claude/`、`internal/llm/llmhttp/` を `llmhttp/` と略す。

| AC | 内容 | 種別 | 検証の実行場所 |
|---|---|---|---|
| AC-01 | 有効な構築（5 つの effort、ワークスペース ID の有無） | test | `claude/claude_test.go::TestNew` |
| AC-02 | 不正な API キー・モデル名・effort・タイムアウトの拒否 | test | `claude/claude_test.go::TestNew`（ゼロ値の `Secret`、空・前後に空白・不正な UTF-8 のモデル名、`EffortUnset` と範囲外の `Effort`、0 と負のタイムアウト。エラーと `nil`）・`internal/llm/claudeparam/claudeparam_test.go::TestParseEffort` |
| AC-03 | `POST` 1 回、3 つのヘッダー、`anthropic-beta`・`Authorization` なし、本文と URL に API キーなし | test | `claude/claude_test.go::TestGenerateSendsRequest` |
| AC-04 | `model`・`system`・`messages` とプロンプトの同一性 | test | `claude/claude_test.go::TestGenerateSendsRequest`（前後の空白・改行を含むプロンプトを、デコードした文字列で比較。`messages` は要素 1 つ、`role` が `user`、`content` が JSON 文字列） |
| AC-05 | `max_tokens` が正の値ならその値、0 なら定数 | test | `claude/claude_test.go::TestGenerateMaxTokens` |
| AC-06 | `output_config.effort` が 5 つの値のそれぞれ | test | `claude/claude_test.go::TestGenerateEffort` |
| AC-07 | トップレベルと `output_config` のメンバーの集合 | test | `claude/claude_test.go::TestGenerateSendsRequest`（メンバーの集合が一致すること） |
| AC-08 | 不正なリクエストは `ErrInvalidRequest` で送信 0 回 | test | `claude/claude_test.go::TestGenerateInvalidRequest`（記録するサーバーへのリクエスト 0 件。既に終わった `ctx` と同時の場合も `ErrInvalidRequest`） |
| AC-09 | リダイレクトに従わない | test | `llmhttp/llmhttp_test.go::TestPostNoRedirect`（リダイレクトに従う `Client`・`nil` の `Client`）・`claude/claude_test.go::TestGenerateNoRedirect`（`307` と `Location`、リダイレクト先へのリクエスト 0 件、`StatusCode` 307） |
| AC-10 | 成功時の `Text`・`Model`・`ModelVersion` | test | `claude/response_test.go::TestGenerateResponseFixtures`（実応答の `end_turn`、`text` の前後に空白と改行を足した応答で完全一致）・`TestGenerateModelFromResponse` |
| AC-11 | `200` 以外は `ErrHTTPStatus`、ステータスコード、本文の目印が現れない | test | `claude/claude_test.go::TestGenerateHTTPStatus`（`400`・`401`・`403`・`404`・`413`・`429`・`500`・`529`・`307`。`errors.AsType[*HTTPStatusError]`。本文の目印が現れない）・`llmhttp/llmhttp_test.go::TestPostNon200`（本文の目印、本文を送らないサーバーに対して本文を読まずに戻る） |
| AC-12 | `max_tokens` は `ErrTruncated`、ゼロ値 | test | `claude/response_test.go::TestGenerateStopReason`（空でない `text` の `max_tokens`）・`TestGenerateResponseFixtures`（実応答の `max_tokens`） |
| AC-13 | `end_turn`・`max_tokens` 以外は `ErrUnexpectedFinishReason` | test | `claude/response_test.go::TestGenerateStopReason`（`refusal`・`stop_sequence`・`tool_use`・`pause_turn`・`model_context_window_exceeded`・未知の値。`ErrTruncated` でないこと） |
| AC-14 | `text` ブロックなし・空・空白だけは `ErrEmptyResponse` | test | `claude/response_test.go::TestGenerateEmptyText` |
| AC-15 | `thinking`・`redacted_thinking` を `Text` に含めない、順序によらない | test | `claude/response_test.go::TestGenerateThinkingBlocks` |
| AC-16 | 検証の順序 | test | `claude/response_test.go::TestGenerateValidationOrder`（`max_tokens` かつ `text` ブロックなしは `ErrTruncated`）・`claude/claude_test.go::TestGenerateHTTPStatus`（`200` 以外かつ不正な JSON は `ErrHTTPStatus` で `ErrInvalidResponse` でない） |
| AC-17 | 番兵の相互の区別 | test | `claude/claude_test.go::TestGenerateSentinelsDistinct`・共有アサーション（`claude_test.go`・`response_test.go` の全失敗ケース） |
| AC-18 | タイムアウトは猶予 2 秒以内に `context.DeadlineExceeded` | test | `claude/claude_test.go::TestGenerateTimeout`（ヘッダーを返さないサーバー、本文の途中で待つサーバー）・`llmhttp/llmhttp_test.go::TestPostTimeout` |
| AC-19 | キャンセルは `context.Canceled`、キャンセル済みなら送信 0 回 | test | `claude/claude_test.go::TestGenerateCanceled`・`llmhttp/llmhttp_test.go::TestPostCanceled`（キャンセル済みでは `NewRequest` と `RoundTripper` の呼び出しが 0 回） |
| AC-20 | 通信の失敗は `ErrTransport`、途中の切断も `ErrTransport` | test | `claude/claude_test.go::TestGenerateTransportFailure`（接続をすぐ閉じるリスナー、本文の途中で閉じるリスナー。`context` のエラーにも `ErrInvalidResponse` にも該当せず、ゼロ値）・`llmhttp/llmhttp_test.go::TestPostTransportFailure` |
| AC-21 | エラーの各書式に API キーが現れない | test | 共有アサーション（`TestNew`・`TestGenerateInvalidRequest`・`TestGenerateNoRedirect`・`TestGenerateHTTPStatus`・`TestGenerateStopReason`・`TestGenerateEmptyText`・`TestGenerateTimeout`・`TestGenerateCanceled`・`TestGenerateTransportFailure` と、`response_test.go` の 3.2 の各拒否のテスト）・`claude/claude_test.go::TestRevealOnlyInRequestFile` |
| AC-22 | アダプタの値の出力に API キーが現れない | test | `claude/claude_test.go::TestClientOutputDoesNotLeakAPIKey`（`fmt` の `%v`・`%+v`・`%#v`、`slog` の TextHandler・JSONHandler、`encoding/json`） |
| AC-23 | `claude` の設定から 5 つの effort で構築し、送る値が環境変数の値 | test | `internal/config/config_test.go::TestLoadClaude`（5 つの effort、各アクセサ）・`internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`（5 つの effort で `model`・`output_config.effort`・`x-api-key`）・`TestNewUsesClaudeAdapter` |
| AC-24 | API キー・effort の未設定・空は `ErrMissing`、変数名を取り出せる | test | `internal/config/config_test.go::TestLoadClaudeMissing`（`DEEPSEEK_API_KEY` だけがある場合を含む。`errors.AsType[*VarError]` で変数名） |
| AC-25 | 不正な effort・未知のプロバイダは `ErrInvalid`、値が現れない | test | `internal/config/config_test.go::TestLoadClaudeInvalid`（effort の例）・`TestLoadInvalid`（`Claude`・`anthropic`）・`TestLoadErrorsOmitValues`（プロバイダが `claude` の環境の行で目印） |
| AC-26 | `deepseek` のとき Claude の変数を検査しない、不正なプロバイダのエラーが増えない | test | `internal/config/config_test.go::TestLoadIgnoresOtherProviderVars`（3 つの変数の未設定・空・不正・正しい値の各組み合わせで、DeepSeek の設定がすべて未設定のときと同じ。プロバイダが不正なときは、Claude の変数が空・不正でも `YT2COLUMN_LLM_PROVIDER` のエラーだけ）・`internal/llm/provider/provider_test.go::TestNewDeepSeekIgnoresClaudeVars`（構築関数が受け取る `deepseek.Options` が同じ） |
| AC-27 | 既存の DeepSeek の設定の振る舞いが変わらない | test | `internal/config/config_test.go` の既存テスト（ステップ 4-1 の改名だけのコミットで期待値を変えずに通る。以後の変更は `:173` の入れ替えと行の追加）・`internal/llm/provider/provider_test.go::TestNewDeepSeekSendsConfiguredRequest`（`newClient` の呼び出しの形だけを変える）・`make test` |
| AC-28 | 統合テストは `make test`・`make test-ci` に含まれない | static | `claude/claude_test.go::TestIntegrationTestBuildTag`・`make test`・`make test-ci`（`-tags test` だけでビルドする） |
| AC-29 | `make test-integration-claude` のフラグ・表示・スキップ | test / manual | `claude/makefile_test.go::TestMakeTestIntegrationClaude`・`internal/llm/claude/testutil/integration_settings_test.go::TestIntegrationSettings`（API キーの未設定・空でスキップし理由に変数名、`ANTHROPIC_API_KEY` だけではスキップ）・`claude/claude_test.go::TestIntegrationOptionsSkipMissingKey`・`cmd/yt2column/makefile_test.go::TestMakeOptInsAreTargetSpecific`・`internal/llm/deepseek/testutil/make_test.go`（ラッパーの記録する変数）・§5.1 の記録（API キーなしのスキップの行を含む） |
| AC-30 | 実 API での正常な生成と打ち切り | test / manual | `claude/integration_test.go::TestIntegrationGenerate`（`make test-integration-claude` で実行）・§5.1 の記録（`cmd/yt2column/docs_test.go::TestClaudePlanRecordsManualRun`） |
| AC-31 | ユニットテストは外部ホストを呼ばない | test / static | `claude/claude_test.go::TestUnitTestsCannotReachProductionEndpoint`（`TestMain` のプロキシと `Transport` が `nil`）・`TestNewForLoopbackTestRejectsNonLoopback`・`internal/llm/provider/provider_test.go::TestUnitTestsCannotReachProductionEndpoint`・`llmhttp/llmhttp_test.go::TestNewClient`・`internal/pipeline/pipeline_test.go::TestConfigImports`（`internal/config`・`claudeparam` の直接の import） |
| AC-32 | 不正な JSON・空・オブジェクト以外・後続データ | test | `claude/response_test.go::TestGenerateInvalidBody` |
| AC-33 | 消費するメンバーの欠落・`null`・種類の不一致、空の `model` | test | `claude/response_test.go::TestGenerateConsumedMembers`（`ErrEmptyResponse` でないことも確かめる） |
| AC-34 | `content` の要素の種類、未知の `type`、2 つ以上の `text` | test | `claude/response_test.go::TestGenerateContentShape` |
| AC-35 | 消費するメンバーの重複 | test | `claude/response_test.go::TestGenerateDuplicateMembers` |
| AC-36 | 不正な UTF-8・対になっていないサロゲート（消費するメンバーと消費しないメンバー） | test | `claude/response_test.go::TestGenerateEncoding` |
| AC-37 | 上限ちょうどは受理、超えたら `ErrInvalidResponse` | test | `claude/response_test.go::TestGenerateSizeLimit`（`llmhttp.MaxResponseBytes` を参照）・`llmhttp/llmhttp_test.go::TestPostSizeLimit` |
| AC-38 | 消費しないメンバーを無視して受理、実応答の形も受理 | test | `claude/response_test.go::TestGenerateUnconsumedMembers`（`id`・`usage`・`stop_details`・`signature`・未知のメンバー）・`TestGenerateResponseFixtures`（フィクスチャそのまま） |
| AC-39 | 空・不正なワークスペース ID の拒否 | test | `internal/llm/claudeparam/claudeparam_test.go::TestParseWorkspaceID`・`TestWorkspaceIDHasNoExportedFields`（architecture §7.4 のとおり、空・不正な値は `WorkspaceID` の値として作れない。別パッケージから非公開のフィールドに書けないことで構築を `ParseWorkspaceID` に限る） |
| AC-40 | `anthropic-workspace-id` の有無と値 | test | `claude/claude_test.go::TestGenerateWorkspaceHeader` |
| AC-41 | `ANTHROPIC_WORKSPACE_ID` の設定・未設定とヘッダー | test | `internal/config/config_test.go::TestLoadClaude`（未設定で成功しゼロ値）・`internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`（設定ありで値と同一、未設定でヘッダーなし） |
| AC-42 | ワークスペース ID の空は `ErrMissing`、不正は `ErrInvalid`、値が現れない | test | `internal/config/config_test.go::TestLoadClaudeInvalid`・`TestLoadClaudeMissing`（`errors.AsType[*VarError]` で変数名）・`TestLoadErrorsOmitValues`（プロバイダが `claude` の環境の行） |
| AC-43 | 統合テストのワークスペース ID | test / manual | `internal/llm/claude/testutil/integration_settings_test.go::TestIntegrationSettings`（設定ありで値、未設定で指定なし、`ANTHROPIC_WORKSPACE_ID` だけでは指定なし、空・不正で失敗）・§5.1 の記録 |
| AC-44 | ワークスペース ID は本文と URL に現れない | test | `claude/claude_test.go::TestGenerateSendsRequest`（ワークスペース ID を指定した構築で、本文と URL に現れない） |
| AC-45 | `claude` のとき `DEEPSEEK_API_KEY` の値で結果が変わらない | test | `internal/config/config_test.go::TestLoadClaudeIgnoresDeepSeekKey`（未設定・空・空でない値で、成功の場合も、エラーの場合の変数の集合も同じ） |

### 5.1. 手動実行の記録 (AC-29・AC-30・AC-43)

ステップ 5-12 で記入する。記録する項目は、API キーなしでの実行のスキップの行と、API キーありでの実行日、HEAD、モデル名・effort、ワークスペース ID の指定の有無（値は書かない）、`TestIntegrationGenerate` の 2 つのサブテストの `--- PASS` と所要時間である。チェックが入った後の記録の有無は `cmd/yt2column/docs_test.go::TestClaudePlanRecordsManualRun` が固定する。

（未実施）

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| `llmhttp` への切り出しで DeepSeek のエラーのメッセージや分類が変わる | DeepSeek の既存テストが失敗する | ステップ 1-4 で文言を変えずに移し、DeepSeek のテストを変更せずに通す（ステップ 1-5）。経過時間の追加は別のコミットにする（ステップ 1-8）。 |
| 経過時間の表記が `2s` を含む（例えば `1.2s` と表記された）ため、`deepseek_test.go:514` の「アダプタのタイムアウトの値 `2s` を含まない」検査が失敗する | 負荷の高い CI で DeepSeek のテストが断続的に失敗する | ステップ 1-8 で、`2s` を含まない経過時間の表記（整数のミリ秒）を選ぶ。 |
| `test_helpers.go`・`llmhttptest`・`maketestutil` は `_test.go` ではないため、`.golangci.yml` のテスト向けの除外が効かない | `make lint` が通らない | 未チェックのエラーを残さない、静的エラーを使う、固定の文字列を定数にする。`gosec` の指摘は、指摘された 1 行に限った `//nolint:gosec` と理由のコメントで抑える。ファイル・パッケージ単位の抑制はしない。 |
| タイムアウトのテストが `-race` 付きの CI で不安定になる | CI が断続的に失敗する | 猶予 2 秒で判定し、厳密な時間比較をしない。待つハンドラは `t.Cleanup` で解放する。 |
| `TestMain` のプロキシの設定が、`-tags test,integration` で統合テストを実行したときにも効く | その実行では統合テストが本番の送信先へ届かず失敗する | 統合テストは `make test-integration-claude`（`-tags integration` だけ）で実行する。失敗は外部へ送らない側に倒れる。 |
| 統合テストの打ち切りの確認が、モデルの入れ替えで `end_turn` になる | アダプタの不具合でない失敗 | 長い出力を求めるプロンプトと、前提が崩れたことを示す失敗メッセージ（ステップ 5-5）。 |
| 非ストリーミングの長い生成が、利用者のネットワークで無通信のまま切断される | 料金が発生し、記事は投稿されない | architecture §3.6 のとおり受け入れる。メッセージの経過時間で原因を見分ける（ステップ 1-8）。 |
| `llmhttptest`・Claude アダプタ・`maketestutil` が DeepSeek の部品と似た形になり、`dupl` が指摘する | `make lint` が通らない | 共通にできる部分は `llmhttp`・`maketestutil` に置く（ステップ 1-1・5-1）。残る重複は、番兵と応答の形がアダプタごとに異なるためである（architecture §1.1 の原則 2）。指摘が出た場合は、共通化できるかを先に検討し、できない場合だけ範囲を限った抑制と理由のコメントを付ける。 |
| フェーズ 6 の Anthropic の公開文書の確認、またはステップ 5-12 の実 API での実行の承認を待つ | フェーズ 5・6 が止まる | 両方とも人間の承認が要る作業であり、フェーズ 1〜4 の実装中に承認を依頼しておく。実 API での実行の前に、料金の発生しないスキップの確認を済ませる（ステップ 5-12）。 |
| `make test-integration-claude` の実行に料金が発生する | 想定外の費用 | 実行は人間の承認を得てから行う（ステップ 5-12）。既定のモデル・effort は料金を抑える組み合わせにする。 |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（ステップ 1-1〜1-8。リファクタリングと経過時間の追加が別のコミット）
- [ ] フェーズ 2 完了（ステップ 2-1〜2-4）
- [ ] フェーズ 3 完了（ステップ 3-1〜3-9）
- [ ] フェーズ 4 完了（ステップ 4-1〜4-9。改名だけのコミットが独立している）
- [ ] フェーズ 5 完了（ステップ 5-1〜5-12）
- [ ] フェーズ 6 完了（ステップ 6-1〜6-9）
- [ ] 各フェーズで `make fmt` → `make test` → `make lint` が通る
- [ ] `make test-integration-claude` が通り、§5.1 に実行の記録が残っている
- [ ] §5 のすべての AC の検証が通る
- [ ] `implementation_handoff.md` の I-01〜I-04 が §1.5 のとおり反映されている

## 8. 成功基準 (Success Criteria)

- **機能:** AC-01〜AC-45 のすべてが §5 の検証で確認されている。
- **品質:** `make test`・`make lint` が通る。§4.1 の網羅率の目標を満たす。各テストは対象を壊して失敗することを確認済みで、そのことがコミットメッセージに記録されている。
- **セキュリティ:** API キーの非漏洩（AC-03・AC-21・AC-22）、`Reveal()` の呼び出し箇所の固定、リダイレクトに従わないこと（AC-09）、応答本文の上限（AC-37）、`200` 以外の応答本文を読まないこと（AC-11）、ユニットテストが外部ホストへ届かないこと（AC-31）、CLI の出力の伏せ字化と秘密情報の変数名の検査に Anthropic の API キーが含まれることを確認済みである。
- **互換性:** DeepSeek アダプタの既存テストがフェーズ 1 で無変更のまま通る。依存モジュールを追加していない（`.golangci.yml` の depguard を変更していない）。
- **ドキュメント:** `README.md`・`project_overview.md`・`security.md`・`package_reference.md`・`CLAUDE.md` が実装と一致し、記述の根拠を照合済みである。

## 9. 次のステップ (Next Steps)

- 本計画のレビューと承認（`approved`）。
- 承認後、`/mkplan2 0009` で PR の区切りを本計画に埋め込み、`/runplan 0009` で実装する。
- 実装の完了後、architecture §9 の申し送り（DeepSeek アダプタのテストの `llmhttptest` への移行、effort の記事のメタ情報への記録、`request-id` の扱い）を、必要に応じて別の issue にする。
