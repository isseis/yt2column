# 実装計画書：localhost サーバ本体と拡張との HTTP API

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-14 |
| Review date | 2026-10-14 |
| Reviewer | isseis |
| Comments | 2026-10-14: mkplan2 で PR 境界（PR-1〜PR-7）を加え、§3.2 PR 構成と §7 チェックリストを PR 単位にした。あわせて、#222 で architecture §7.2 の改訂と再承認が済んだことに合わせ、再承認を「これから受ける」と書いていた記述を過去形に直した（editorial correction。計画の決定は変えていない）。 |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項目で、その対応は architecture §3.13 にある。既存のファイルの行番号と事実は、断りがない限り HEAD `e0f898e`（ブランチ `issei/localhost-server-01`）で確かめた。`implementation_handoff.md` は本タスクにはない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements が定める 10 の機能要件（F-001〜F-010）を、architecture の設計どおりに実装する。フェーズ 1 で契約と共有部品を凍結し（`internal/failure`・`internal/redact`・`internal/signalrun`・`internal/authtoken`・`internal/config` の分割）、フェーズ 2 で `internal/server`・`cmd/yt2column-server`・統合テスト・文書を作る。フェーズの名前と順序は architecture §8 の実装優先順位（フェーズ 1「契約の凍結」・フェーズ 2「サーバ本体」）に一致させる。

本計画では、#107（タスク 0015）が実装済みの `internal/transcript` の `CopiedSource`（architecture §3.7 の契約。`internal/transcript/copied.go:38`・`errors.go:18`）を使い、#108（タスク 0016）が実装する `failure.Notifier` を fake に置き換えてサーバをテストする。`internal/failure` の型は本タスクが新設する（architecture §3.9）。

### 1.2. 実装原則

- architecture §1.1 の設計原則 8 項目に従う。特に、検査は安い順・信頼の低い順に並べ、副作用を最後に置くこと（原則 2）、拒否の理由と失敗の種類を型で表すこと（原則 4）、受け取った値を補正しないこと（原則 5）、秘密と字幕の本文を応答・ログ・失敗の通知に流さないこと（原則 6）、CLI と共有できる部品を `internal/` へ移すこと（原則 8）。
- Go のソースのコメント・識別子・文字列リテラルは英語で書く（`_context.md`）。例外は、日本語の文書の見出しや専門用語を照合する doc ガードの文字列リテラルである（`cmd/yt2column/extension_docs_test.go:89` と同じ）。`AC-NN`・`F-NNN`・`H-NN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- 各フェーズの完了時に `_context.md` の "Build checks"（`make fmt` → `make test` → `make lint`）を通す。グリーンゲートは `make test && make lint`。
- 振る舞いを変えない移動・分割（1-2・1-3）は、既存テストを内容を変えずに通すことで固定する。テストを移す場合は `go tool cover -func` で関数ごとのカバレッジが変わらないことを確かめる（CLAUDE.md「Deleting a test is a claim that must be checked」）。
- 新しいパッケージを加える各ステップでは、同じ変更で `package_reference.md` に行を加える（`internal/pipeline/pipeline_test.go:610` の `TestPackageReferenceListsPackages` が `make test` で全パッケージの行を要求する）。`testutil/` を加えるステップでは同じ変更で `TestFakesCarryBuildTag` の件数（`pipeline_test.go:558` の 17）を改める。

### 1.3. 既存コード調査結果

**再利用する既存の部品（変更しない）。**

| 部品 | 場所 | 使い方 |
|---|---|---|
| `transcript.ValidateVideoURL`・`NormalizedVideoURL` | `internal/transcript/video_id.go:16`・`:64` | `videoUrl` の検証と正規化 URL の導出（AC-09・AC-19） |
| `transcript.CopiedInput`・`NewCopiedSource`・`ErrCopiedTranscriptFormat` | `internal/transcript/copied.go:12`・`:38`、`errors.go:18` | 本文の検査で直接作り、字幕の書式の誤りを `400` に分ける（AC-11） |
| `pipeline.New`・`Run`・`StageError`・`Stage` | `internal/pipeline/pipeline.go:77`・`:88`・`:52`・`:19` | ジョブごとに組み、失敗を段階で分類する（§3.9） |
| `writer.New`・`ArticleWriter`・`Options` | `internal/writer/writer.go:101`・`:36`・`:44` | サーバの既定（字数の調整と検証・推敲を有効、組み込みテンプレート）で 1 つ構築して共有する |
| `publisher.NewSlackWebhookPublisher`・`SlackPostTimeout`・`SlackMessageCount` | `internal/publisher/slack.go:140`・`:23`、`slack_message.go:59` | 記事の投稿。1 つ構築してすべてのジョブで共有する（AC-20） |
| `provider.New`・`NewReview`・`LLMTimeout` | `internal/llm/provider/provider.go:30`・`:37`・`:20` | 1-3 で引数だけ `config.LLMConfig` に変える |
| `strictjson.ParseObject`・`Collect`・`RequiredString`・`Value.AsString` | `internal/strictjson/strictjson.go:67`・`:85`・`:140`・`:174` | 本文の検査（AC-05〜AC-07・AC-13）。`OptionalString`（`:158`）は空文字列を拒否するので使わない |
| `secret.Secret` | `internal/secret/secret.go:25` | API キー・Webhook URL・共有トークンを持つ |
| fake の `ArticleWriter`・`Publisher` | `internal/writer/testutil/mocks.go:16`、`internal/publisher/testutil/mocks.go:15` | サーバのユニットテスト |
| `internal/loopbacktest.ValidateURL` | `internal/loopbacktest/loopbacktest.go:23` | テスト用の loopback 構築 |
| `slackwebhook.ValidURL`・`SensitiveParts` | `internal/slackwebhook/slackwebhook.go:20`・`:34` | 1-2 で `internal/redact` が使う。`internal/config` も使う |
| CLI の `sanitize`・`escapeNonPrintable` | `cmd/yt2column/output.go:34`・`:139` | 1-2 で `internal/redact` へ移す |
| CLI の `configuredSecrets` | `cmd/yt2column/run.go:667` | 1-2 で `redact.SecretStrings` に一般化して移す |
| CLI の `runWithSignals`・`subscribedSignals`・`reraiseQueued` | `cmd/yt2column/main.go:50`・`:26`・`:78` | 1-2 で `internal/signalrun` へ移す |

**変更する既存のコード（architecture §3.14 の一覧を実装計画の単位に割り当てる）。**

- `internal/config/config.go`（`:103` の `Config`、`:204` の `Load`、`:405` の `loadSlackWebhook`、`:482` 以降の `VarError` ヘルパ）: 1-3 で `LLMConfig` を分離し、1-4 で `ServerConfig`・`LoadServer`・`loadLLM`・`loadSlackWebhook` の返り値変更を加える（§3.2.1）。
- `internal/config/cachedir.go`（`:14` の `defaultCacheDir`、`:41` の `absoluteEnv`）: 1-4 で OS ごとの分岐を一般化し、設定ディレクトリの導出と共有する（§3.2.3）。
- `internal/llm/provider/provider.go`（`:30`・`:37`・`:55`）: 1-3 で引数を `config.LLMConfig` にする。
- `internal/writer/errors.go`（`:47` の非公開 `errPromptTooLarge`）と `internal/writer/prompt.go`（`:101`・`:110`・`:122`・`:124` の参照）: 1-4 で公開の `ErrPromptTooLarge` に改める。包む関係（`ErrInvalidTemplate` も包む。`prompt.go:122-125`）は変えない。
- `internal/pipeline/pipeline_test.go`（`:558` の 17、`:610` の `TestPackageReferenceListsPackages`）: `testutil/` と新パッケージを加えるステップで更新する。
- `internal/config/envaccess_test.go`（`:65-68` の `allowedEnvRefs`）: 2-4 で `cmd/yt2column-server/main.go` の行を加える。
- `cmd/yt2column/test_helpers.go`（`:254` の子プロセスの入口）: 1-2 で `signalrun.Run` を呼ぶ形に改める。
- `cmd/yt2column/run.go`（`:44` の `deps`、`:357` の `buildWriter`、`:667` の `configuredSecrets`）: 1-3 で `deps` の型を、1-2 で `configuredSecrets` の呼び出しを改める（`:357` の `buildWriter` は `deps` の型に従う）。`errPromptTooLarge` の参照は `internal/writer` の外にないので、CLI の変更は要らない。

**新設するパッケージと `package_reference.md` の行（各ステップで同時に加える）。** `internal/failure`（1-1）・`internal/failure/testutil`（1-1）・`internal/redact`（1-2）・`internal/signalrun`（1-2）・`internal/authtoken`（1-5）・`internal/server`（2-1）・`cmd/yt2column-server`（2-4）。`internal/publisher` の行への `SlackFailureNotifier` の追記は #108（タスク 0016 のフェーズ 4）が行うので、本タスクでは加えない（architecture §3.12）。

**architecture §3.14 の記述のうち、HEAD `e0f898e` と食い違う点。** `TestProjectOverviewDocumentsConfig` はコミット `4443ddf` で削除済みで存在しない。architecture §3.14 をこの事実に合わせる editorial correction を、本計画と同じブランチで先に行った（#222 に含めた）。これを踏まえ、`project_overview.md` のサーバの内容は 2-5 の新しいガードで固定する。

**#108 との調整。** 1-1 で `internal/failure` の契約ガード（`internal/failure/failure_test.go`。契約は architecture §3.9、テストの内容はタスク 0016 の architecture §3.6）を置く。これにより、タスク 0016 の実装計画 §フェーズ 3 のステップ 3-1（「#109 が同等のガード を置いていない場合だけ加える」）は当てはまらなくなる（N/A）。`internal/publisher` の `depguard` 規則（同ステップ 3-2・3-3）は 0016 側に残る。

**スコープ外。** `internal/failure` の型の新設以外の #108 の作業、#107 のパーサ、拡張（#111）、起動の自動化、待ち行列、API のバージョニングは本タスクの対象外である（requirements §2.3）。

### 1.4. 対象外

requirements §2.3 のとおり。加えて、次のものは本計画で扱わない。`:8080` などの別ポートの受け渡し方法（#111 が決める）、共有トークンの自動ローテーション、記事の全文や進捗の返却。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 契約の凍結（#107・#108 の実装を待たない）

**対象ファイル:** 新設する `internal/failure`・`internal/redact`・`internal/signalrun`・`internal/authtoken` と、変更する `internal/config`・`internal/llm/provider`・`internal/writer`・`cmd/yt2column`。

- [x] **ステップ 1-1**: `internal/failure/failure.go` に `Kind`・`Notice`・`Notifier` を、architecture §3.9 のとおりに定義する。`Kind` の列挙の並びとゼロ値（`KindUnknown`）を §3.9 のまま固定する。`internal/failure/testutil/mocks.go`（`package failuretestutil`、`//go:build test`）に fake の `Notifier`（呼び出された `Notice` を記録する）を加える。`internal/failure/failure_test.go`（`//go:build test`）に契約ガード（`Notice` の exported フィールドが `RequestID`・`VideoID`・`Kind` の 3 つだけ、`Notifier` のメソッドが `Notify(ctx context.Context, Notice) error` の 1 つだけ、`Kind` のゼロ値が `KindUnknown`、`Kind` 型の定数の並びが §3.9 の列挙と一致する。内容は 0016 architecture §3.6）を置く。`package_reference.md` に `internal/failure`・`internal/failure/testutil` の行を加え、`TestFakesCarryBuildTag` の件数を 17 から 18 に増やす。

### PR-1 作成ポイント: failure contract package

**対象ステップ**: 1-1

**推奨タイトル**: `feat(0011): add the internal/failure contract`

**レビュー観点**: `Kind` の列挙の並びとゼロ値（`KindUnknown`）が architecture §3.9 のとおりであること / `Notice` の exported フィールドが `RequestID`・`VideoID`・`Kind` の 3 つだけで、`Notifier` のメソッドが `Notify(ctx context.Context, Notice) error` の 1 つであること / 契約ガードが architecture §3.9 と 0016 architecture §3.6 の内容を固定していること / `package_reference.md` の行と `TestFakesCarryBuildTag` の件数（17→18）を同じ変更で更新していること

**実装モデル要件**: standard

**判定理由**: 契約は architecture §3.9 と 0016 architecture §3.6 に確定しており、前例のない設計判断・競合する実装案・分離された高リスク／複雑なステップのいずれにも該当しない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [x] **ステップ 1-2**: `internal/redact`（`Line`・`Writer`・`SecretStrings`）と `internal/signalrun`（`Run`）を新設し（architecture §3.11）、CLI を切り替える。`cmd/yt2column/output.go` を削除する。`cmd/yt2column/run.go` の `configuredSecrets` の呼び出しを `redact.SecretStrings` に、`sanitize` の呼び出しを `redact.Line` に置き換える。`cmd/yt2column/main.go` と `cmd/yt2column/test_helpers.go:254` は `signalrun.Run` を使う。`TestSanitize*`・`TestSubscribedSignals` を `internal/redact`・`internal/signalrun` のテストとして移す。`TestConfiguredSecrets*`（`cmd/yt2column/run_test.go:1867`・`:1904`）は `config.Load` と `cmd/yt2column` の `newRunEnv` で組んだ `config.Config` を使うので、そのままは移せない。`internal/redact` には `secret.Secret` から組む `SecretStrings` のテストを新しく書く。`cmd/yt2column` では設定から `redact.SecretStrings` に渡す秘密の組（Webhook URL の各部分と `ANTHROPIC_API_KEY`・`DEEPSEEK_API_KEY`）を `run` の実行経路で確かめる。`configuredSecrets` の呼び出しをテスト側で再現するだけでは `run` の対応付けの退行を検出できないため、`TestConfiguredSecrets*` は残さずに削除し、既存の「write failure whose message holds the API key」の行が DeepSeek 鍵を、新設する Claude 設定で `ANTHROPIC_API_KEY` を載せた失敗の行が Anthropic 鍵を、`--slack failure whose message holds the webhook path` の行が Webhook URL の一部分を、それぞれ `[REDACTED]` として確かめる。子プロセスで `subscribedSignals` の無視の分岐を確かめる `TestIgnoredSignalNotSubscribed` などのシグナルのテストは `cmd/yt2column` に残る。`package_reference.md` に 2 つの行を加える。`internal/redact` は `internal/config` を import しない（秘密は `secret.Secret` の列で受け取る）。

### PR-2 作成ポイント: move the shared CLI parts

**対象ステップ**: 1-2

**推奨タイトル**: `refactor(0011): move redaction and signal handling into internal`

**レビュー観点**: 振る舞いを変えない移動であること（既存テストを内容を変えずに通す）/ `internal/redact` が `internal/config` を import しないこと / `TestSanitize*`・`TestSubscribedSignals` の移設と `TestConfiguredSecrets*` の実行経路の行への置き換えがステップ 1-2 の記述どおりであること / `package_reference.md` の 2 つの行を同じ変更で加えていること

**実装モデル要件**: standard

**判定理由**: architecture §3.11 に確定した振る舞い不変の移動で、前例のない設計判断・競合する実装案・分離された高リスク／複雑なステップのいずれにも該当しない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [x] **ステップ 1-3**: `internal/config` に `LLMConfig` を分離する（architecture §3.2.1）。`Config` は LLM の設定を非公開のフィールドで持ち、`LLM()` で返す。`provider.New`・`NewReview`・`newClient` の引数を `config.LLMConfig` に変える。`cmd/yt2column` の `deps.newLLMClient`・`newReviewLLMClient` の型と `cfg` のアクセサの呼び出し（`cfg.Provider()` → `cfg.LLM().Provider()`）を書き換える。`config_test.go`・`provider_test.go`・`run_test.go`・`test_helpers.go`・`integration_test.go`（`:127`・`:134` の `func(cfg config.Config)`）のアクセサと fake の型を改める。`make lint` は golangci-lint を `--build-tags test,integration` で走らせ、`go vet -tags integration ./...` も走らせる（`Makefile:10`・`:195`）ので、`integration` タグのビルドも通ることを確かめる。振る舞いは変えない。
- [x] **ステップ 1-4**: `ServerConfig`・`LoadServer`・`YT2COLUMN_SERVER_PORT`（既定 `8765`、`1024`〜`65535`）・設定ディレクトリとトークンのパスの導出を加える（architecture §3.2.1〜§3.2.3）。`cachedir.go` の OS ごとの分岐を、ディレクトリ名と XDG の変数名を引数に取る関数に一般化し、`defaultCacheDir` と共有する。`loadLLM` を切り出し、`loadSlackWebhook` の返り値を「値・設定されていたか・拒否」に改める。`internal/config` は `net` を import しない（`pipeline_test.go:817` の `TestConfigImports`。ポートの検査は `strconv` と文字の比較で行う）。`internal/writer` の `errPromptTooLarge` を公開の `ErrPromptTooLarge` に改め、`errors.go`・`prompt.go` と `prompt_test.go` の参照を改める。ポートの受理・拒否の表（§3.2.2）と設定ディレクトリの導出の表（§3.2.3。`goos`・`HOME`・`XDG_CONFIG_HOME` と失敗した変数の名前）をテストで固定する。

### PR-3 作成ポイント: split the LLM config and add the server config

**対象ステップ**: 1-3 / 1-4

**推奨タイトル**: `feat(0011): split config.LLMConfig and add LoadServer`

**レビュー観点**: `Config.LLM()` の分離で「非ゼロの `Config` は `Load` からしか来ない」保証が崩れていないこと / `provider.New`・`NewReview` の引数の変更が全呼び出し元とテスト（`integration_test.go` を含む）に及んでいること / `LoadServer` が `YT2COLUMN_CACHE_DIR`・`YT2COLUMN_YTDLP_PATH` を読まないこと（AC-29・AC-30）/ `internal/config` が `net` を import していないこと（`TestConfigImports`）

**実装モデル要件**: standard

**判定理由**: 分離と新設の方法は architecture §3.2 に確定しており、前例のない設計判断・競合する実装案・分離された高リスク／複雑なステップのいずれにも該当しない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [x] **ステップ 1-5**: `internal/authtoken`（`Token`・`LoadOrCreate`・`Matches`・`IsZero`・`Value`・`FileError`・`ErrInvalidTokenFile`）を architecture §3.3・§4.1 のとおりに実装する。一時ファイルと `link(2)` で作り、読み込みで通常ファイル・権限・所有者・64 バイト・`^[0-9a-f]{64}$` を検査し、末尾の改行も拒否する。`crypto/subtle.ConstantTimeCompare` で比較する。`package_reference.md` に行を加える。

### PR-4 作成ポイント: shared token file

**対象ステップ**: 1-5

**推奨タイトル**: `feat(0011): add internal/authtoken`

**レビュー観点**: 一時ファイルと `link(2)` で作る手順と、読み込みの検査（通常ファイル・権限 `0077` のビット・所有者・64 バイト・`^[0-9a-f]{64}$`・末尾の改行の拒否）が architecture §3.3 のとおりであること / 比較が定数時間で、ゼロ値と空の値が一致しないこと / 不正なファイルを修復も再生成もしないこと（AC-03）/ `FileError` のメッセージにファイルの内容が現れないこと

**実装モデル要件**: frontier-recommended

**判定理由**: 唯一の認証要素である共有トークンのファイル作成と検査（シンボリックリンク・権限・所有者）と定数時間比較を扱う、分離されたセキュリティ上重要なステップである。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**完了条件:** `make fmt`・`make test`・`make lint` が通る。既存テスト（1-2・1-3 で触れないもの）が内容を変えずに通る。

### フェーズ 2: サーバ本体（#107 の実装と、#108 の実装を前提とする）

**対象ファイル:** 新設する `internal/server`・`cmd/yt2column-server` と、変更する `Makefile`・文書。ユニットテストは fake の `failure.Notifier`（1-1）を使うので #108 の実物を待たない。実物の通知が要るのは 2-4 の統合テストだけである。

**統合テストの方式の変更（#222 で再承認済み）。** architecture §7.2 は、統合テストで「`integration` のビルドタグで実際のバイナリを起動し、LLM と Webhook を loopback の fake のサーバに向ける」としていたが、実装できなかった。LLM のエンドポイントは設定で差し替えられず（`internal/llm/deepseek/deepseek.go` と `internal/llm/claude` に固定されており、`config.LoadServer` にエンドポイントの変数はない）、adapter と記事の投稿の loopback の構築関数（`deepseek.NewForLoopbackTest`・`claude.NewForLoopbackTest`・`publisher.NewSlackWebhookPublisherForLoopbackTest`）と fake の `llm.LLMClient`（`internal/llm/testutil`）はすべて `//go:build test` で、`integration` タグだけのビルドからは見えないためである。LLM の API も Webhook も実物を呼ばないテストなので、architecture §7.2 を、`//go:build test` のテストとして `make test` に含め、`run` の `deps` の構築関数を差し替える方式に改め（`cmd/yt2column/signal_test.go` と同じタグ）、`draft` に戻し、#222 で再承認を受けた。あわせて §3.12・§3.14 から `test-integration-server` のターゲットとその検査を除いた。したがって 2-4 はこの点ではブロックされない。フェーズ 1 と 2-1〜2-3 はこの修正に依存しない。

- [ ] **ステップ 2-1**: `internal/server/server.go` に `Options`・`New`・`Server`・`Listen`・`Serve` と `http.Server` の設定（architecture §3.5.1）を、`handler.go` に検査の順序（§3.5.2 の手順 1〜7）とジョブの起動を、`response.go` に `rejectReason`・拒否の詳細・状態コード・追加のヘッダ・応答の本文・リクエスト ID（§3.4.2〜§3.4.3・§3.10）を実装する。`New` は nil の依存部品・ゼロの `Token`・ゼロの `Port` を拒否し、ログと `ErrorLog` を `redact.Writer` の上に作る。`Serve` は取り消し・いずれかの `net.Listener` の失敗で終了の手順（§3.8.3）に入る。`package_reference.md` に行を加える。
- [ ] **ステップ 2-2**: `internal/server/request.go` に、枠を取った後の本文の読み込みと検査（`Content-Length` の事前検査、`http.MaxBytesReader`、`strictjson`（§3.6 の (a)〜(d)）、`ValidateVideoURL`（(e)）、`NewCopiedSource`（§3.5.2 の手順 8〜12））を実装する。検査の順序により、ヘッダの検査に失敗したリクエストと枠を取れなかったリクエストの本文は 1 バイトも読まない（AC-12）。
- [ ] **ステップ 2-3**: `internal/server/job.go` に、枠（`maxInFlight = 2`）・ジョブの goroutine・パニックの回収・失敗の分類（§3.9 の表を網羅的な `switch` で実装し、`rejectReason` から `Kind` への対応も 1 つの `switch` にする）・失敗の通知（枠を持ったまま、`Notify` に 60 秒の期限を付ける）を実装する。枠の持ち主を明示的に移し、解放はちょうど 1 つの持ち主が行う（§3.8.4）。

### PR-5 作成ポイント: the server core

**対象ステップ**: 2-1 / 2-2 / 2-3

**推奨タイトル**: `feat(0011): add internal/server`

**レビュー観点**: 検査の順序（architecture §3.5.2 の手順 1〜7）で、ヘッダの検査に失敗したリクエストと枠を取れないリクエストが本文を 1 バイトも読まないこと（AC-12）/ 枠の持ち主を明示的に移し、解放がちょうど 1 つであること、起動するすべての goroutine がパニックを回収すること（§3.8.4）/ 失敗の分類が §3.9 の表の網羅的な `switch` で、`rejectReason` から `Kind` への対応も 1 つの `switch` であること / 応答・ログ・通知に秘密と字幕の本文が現れないこと（`redact.Writer` を必ず通す）

**実装モデル要件**: frontier-recommended

**判定理由**: 枠（同時実行数）・goroutine のパニック回収・失敗の分類と通知という分離された高リスク／複雑なステップ（並行処理と回復）を含む。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [ ] **ステップ 2-4**: `cmd/yt2column-server/main.go`（`os.LookupEnv`・標準出力・標準エラー出力を `run` に渡し、`signalrun.Run` で包む）と `cmd/yt2column-server/run.go`（起動の手順と終了コード、部品の構築、ログの組み立て。CLI の `deps` と同じく、LLM・`Publisher`・`Notifier` の構築関数をテストで差し替えられる形にする）を実装する。`Makefile` の `build` を 2 つの実行ファイルにする（統合テストは `make test` に含まれるので、専用のターゲットは加えない）。`internal/config/envaccess_test.go` の `allowedEnvRefs` に `cmd/yt2column-server/main.go` を加える。`cmd/yt2column-server/server_test.go`（`//go:build test`）に §4.2 の統合テストを書く。サーバの `writer.Options` が CLI の既定の解析結果と一致することを `cmd/yt2column-server` のユニットテストで固定する（§3.1）。

### PR-6 作成ポイント: the server entry point and integration test

**対象ステップ**: 2-4

**推奨タイトル**: `feat(0011): add the yt2column-server entry point`

**レビュー観点**: 起動の手順と終了コード（2・3・4・5）が architecture §3.1 のとおりであること / `server.Options.Secrets` に API キー・Webhook URL・共有トークンが入り、標準エラー出力に秘密が現れないこと / 統合テストが `deps` を差し替え、`SLACK_WEBHOOK_URL` に `https` の値を渡していること（`http://` の `httptest` の URL を渡していない）/ 子プロセスの起動時に `t.Cleanup` に kill と reap を登録していること。起動の配線と統合テストは #108 の実物（`NewSlackFailureNotifierForLoopbackTest`）に依存するので #108 のマージ後に行う

**実装モデル要件**: frontier-required

**判定理由**: 子プロセスでシグナルと再起動を確かめる統合テストと、`Makefile`・起動の配線を扱う heavy な integration-test surface に該当する（mkplan.md の panel-mode trigger）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [ ] **ステップ 2-5**: 文書を F-010・architecture §3.12 のとおりに更新する。`CLAUDE.md`（2 つの入口、Architecture Overview に新しいパッケージ）、`docs/dev/project_overview.md`（実行形式、入力元、想定ディレクトリ構成、`YT2COLUMN_SERVER_PORT` と読まない変数）、`docs/dev/security.md` §8（サーバの節）、`README.md`（起動と確認の手順、`curl` の例、拒否の理由の一覧）、`docs/translation_glossary.md`。`cmd/yt2column-server/docs_test.go`（`//go:build test`）に、サーバの文書の内容を固定するガードを加える。ガードは、実装と食い違いうる値（既定のポート `8765`、拒否の理由と状態コードの対応、`config` の変数の名前、`rejectReason` の列挙）を `internal/config`・`internal/server` の定数や表から導く（`rejectReason` は `internal/server` の非公開なので、`internal/server` に `-tags test` の公開のテスト用アクセサを用意してガードから読む。`publisher.SlackMaxMessageRunesForTest` と同じ形）。`README` の `curl` の例が要求する要素（パス・送るヘッダ・標準入力から渡す `Authorization`）を確かめる（`cmd/yt2column/docs_test.go:62` の `TestSlackDocsContract` が値を実装の定数から導く形に倣う）。文書を読む補助（`readDoc`・`findTable`・`sectionByHeading` など。`cmd/yt2column/docs_test.go:304-478`）は `cmd/yt2column` の非公開なので、共有の `testutil` に切り出すか、複製する理由を記録する。ガードの日本語の見出しの文字列リテラルは §1.2 の英語の規則の例外とする（`cmd/yt2column/extension_docs_test.go:89` と同じ）。既存の doc ガード（`cmd/yt2column/docs_test.go` の `TestREADMEDocumentsFlags`・`TestSlackDocsContract`・`TestClaudeDocsContract`、`cmd/yt2column/extension_docs_test.go`）が引き続き通ることを確かめる。あわせて、`doc-guard` の CI ジョブ（`.github/workflows/ci.yml:184`。文書だけの PR でも走る）が `go test -tags test ./cmd/yt2column ./internal/pipeline` だけを走らせるので、`./cmd/yt2column-server` を加え、文書だけの PR でもサーバのガードが走るようにする。

### PR-7 作成ポイント: the docs

**対象ステップ**: 2-5

**推奨タイトル**: `docs(0011): document the localhost server`

**レビュー観点**: F-010 と architecture §3.12 の各文書（`CLAUDE.md`・`project_overview.md`・`security.md` §8・`README.md`・`translation_glossary.md`）が更新されていること / 文書のガードが実装の定数・表から値を導き、`project_overview.md` の変数・README の `curl` の例・security.md §8 の内容を確かめていること / `doc-guard` の CI ジョブに `./cmd/yt2column-server` を加え、文書だけの PR でもガードが走ること / 既存の doc ガード（`TestREADMEDocumentsFlags`・`TestSlackDocsContract`・`TestClaudeDocsContract`・`TestExtensionDocumentation`）が引き続き通ること

**実装モデル要件**: standard

**判定理由**: 文書の更新と、`doc-guard` の CI ジョブへの 1 パッケージの追加が主で、前例のない設計判断・競合する実装案・分離された高リスク／複雑なステップのいずれにも該当しない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**完了条件:** `make fmt`・`make test`・`make lint` が通る。§4.2 の統合テスト（LLM の fake と loopback の Webhook）は `make test` に含まれる。AC-01 の外部インターフェースの確認、実際のソケットで上限を超える本文と誤ったトークンを送ったときの結果の記録、通知の表示（`video:` の行の `<URL>` のリンクと改行）の Mattermost での確認を、PR の手動の確認として記録する。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 対応するフェーズ |
|---|---|---|
| M1: 契約が凍結される | `internal/failure`（型・fake・ガード）と共有部品（`internal/redact`・`internal/signalrun`） | フェーズ 1（1-1・1-2） |
| M2: 設定とトークンが揃う | `LLMConfig`・`LoadServer`・設定ディレクトリ・`ErrPromptTooLarge`・`internal/authtoken` | フェーズ 1（1-3〜1-5） |
| M3: サーバが動く | `internal/server`（受け口・本文の検査・ジョブと枠と通知と終了） | フェーズ 2（2-1〜2-3） |
| M4: サーバが組み上がる | `cmd/yt2column-server`・`Makefile`・統合テスト | フェーズ 2（2-4） |
| M5: 文書が揃う | CLAUDE.md・project_overview・security §8・README・glossary とそのガード | フェーズ 2（2-5） |

- フェーズ 1 は #107・#108 に依存しない。1-1 の契約は #107・#108 の issue に記録済み（architecture §1.3）。
- フェーズ 2 の 2-1〜2-3 のユニットテストは fake の `Notifier` と #107 の実物で足りる。#108 の実物が要るのは 2-4 だけである（architecture §8）。

### 3.2. PR 構成

PR は、レビューの単位（1 つの関心事）と、単独でグリーンゲートを通せる単位で分ける。`internal/` を変えるステップは、それを使う `cmd/` より先に着地する。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 | `internal/failure` の型・fake・契約ガード | standard |
| PR-2 | 1-2 | `internal/redact`・`internal/signalrun` の新設と CLI の切り替え（振る舞い不変） | standard |
| PR-3 | 1-3 / 1-4 | `LLMConfig` の分離、`LoadServer`・ポート・設定ディレクトリ、`ErrPromptTooLarge` | standard |
| PR-4 | 1-5 | `internal/authtoken` | frontier-recommended |
| PR-5 | 2-1 / 2-2 / 2-3 | `internal/server`（受け口・本文の検査・ジョブと枠と通知と終了） | frontier-recommended |
| PR-6 | 2-4 | `cmd/yt2column-server`・`Makefile`・統合テスト | frontier-required |
| PR-7 | 2-5 | 文書（F-010）とサーバの文書のガード、`doc-guard` の CI | standard |

PR-1 と PR-2 は独立である。PR-3 は 1-3 → 1-4 の順に、PR-4 は PR-3 の後に置く。PR-5 は PR-1（`failure` の型）・PR-2（`redact`）・PR-3（`writer.ErrPromptTooLarge`）・PR-4（`authtoken`）に依存し、PR-6 は PR-5・PR-3（`config.LoadServer`）と #108 の実物に依存する。PR-7 は PR-6 に依存する。

### 3.3. 各ステップの依存

- 1-2 は `internal/secret`・`internal/slackwebhook` だけに依存し、1-1・1-3 と独立である。1-3 → 1-4 → 1-5 は一本の連鎖である（1-4 は 1-3 の `LLMConfig` に、1-5 は 1-4 の設定ディレクトリ導出に依存する）。1-1 はどのステップにも依存しない。
- 2-1 → 2-2 → 2-3 の順に積む（2-3 は 2-1 の `Options.Notifier` と 2-2 の検査結果を使う）。2-4 は 2-3（と #108 の実物）に、2-5 は 2-4 に依存する。
- 1-1 は 2-1 の前提である（`Options.Notifier` の型）。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

LLM の API も Webhook も呼ばない。`net/http/httptest` と、既存の fake（`internal/writer/testutil`・`internal/publisher/testutil`）と 1-1 の fake `failure.Notifier` を使う。`internal/server` のテストはハンドラを `httptest.NewRecorder` で直接呼ぶか、`127.0.0.1:0` の `net.Listener` を `Serve` に渡して loopback の接続を通して呼ぶ。対象と主なテストは architecture §7.1 の表のとおりである。`internal/failure` のガードは 1-1、`internal/config`・`internal/authtoken` はフェーズ 1、`internal/server`・`cmd/yt2column-server` はフェーズ 2 が実装する。

### 4.2. 統合テスト

`cmd/yt2column-server` の統合テスト（`cmd/yt2column-server/server_test.go`、`//go:build test`。`make test` に含まれる）を 2-4 で書く。方式は改めた architecture §7.2 に従い、`run` の `deps` の LLM・`Publisher`・`Notifier` の構築関数を差し替える。`HOME` を一時ディレクトリに向け、空きポートを探して `YT2COLUMN_SERVER_PORT` に渡す。`SLACK_WEBHOOK_URL` には `slackwebhook.ValidURL` を通る `https` の値（`cmd/yt2column/run_test.go:136` の `testWebhook` と同じ）を渡す。`config.LoadServer` は `https` 以外を拒否する（`config.go:405`・`slackwebhook.go:20`）ので、`http://` の `httptest` の URL を `SLACK_WEBHOOK_URL` に渡してはならない。必要な環境の一式（モデル・API キー・`SLACK_WEBHOOK_URL`・`HOME`・ポート）を 1 つ作ってすべての起動で使い、各起動が `config.LoadServer` を通ることを確かめる。

- **同じプロセスのテスト。** テストが `run` を goroutine で呼び、`ctx` の取り消しで終え、`run` が返るのを待つ。`deps` の LLM を fake の `llm.LLMClient`（`internal/llm/testutil`）に、`Publisher` を `publisher.NewSlackWebhookPublisherForLoopbackTest` で `httptest` の Webhook へ送る実物に、`Notifier` を `publisher.NewSlackFailureNotifierForLoopbackTest`（タスク 0016 が用意するテスト用の構築関数）で同じく loopback の Webhook へ送る実物に差し替える（`cmd/yt2column/test_helpers.go:212` の `loopbackSlackPublisher` と同じ形。どちらも `testing.TB` を取るので、テストの中で構築する）。`run` は実物の `server.Listen` で待ち受けるので、リクエストは実際のソケットを通る。トークンの生成から再起動後の認証（AC-02。前の `run` が返ってポートが解放された後に、同じ `HOME` で `run` をもう一度呼ぶ）、受理から投稿、実際のソケットで上限を超える本文と誤ったトークンを送ったときの結果（`401` か接続の切断か。記録する。architecture §7.2）を確かめる。サーバが生成したリクエスト ID を実物の通知の実装へ loopback の Webhook で通すテストを置き、応答の `requestId` と同じ値が Webhook の受け取った `text` に現れること、および `ErrSlackInvalidNotice` を返さないことを確かめる（architecture §7.2。このテストは #108 の実物に依存するので、#108 がマージされた後に書く）。
- **子プロセスのテスト。** シグナル 1 回と 2 回の終了は、`cmd/yt2column/signal_test.go` と同じ方式で確かめる。テスト用バイナリを子プロセスで起動し（`cmd/yt2column/test_helpers.go:230` の `runChildMode`・`:268` の `startCLIChild` と同じ形。`os.Args[0]` を子モードで再実行し、子は差し替えた `deps` で `signalrun.Run` 越しに `run` を呼ぶ）、子の `deps` にはネットワークに触れない fake（ブロックする LLM と、Webhook へ送らない `Publisher`・`Notifier`。CLI の子モードの `stallingLLM`・`stallingPublisher` と同じ考え方）を使う。子は `TestMain` から `m.Run` の前に分岐するので `testing.TB` を持たず、`testing.TB` を取る loopback の構築関数を呼べないためである。子プロセスの起動は、起動した時点で `t.Cleanup` に kill と reap を登録し（`cmd/yt2column/test_helpers.go:267` の方針と同じ）、終了コードと出力を確かめる。

実際の LLM と Webhook は呼ばない。AC-01 の外部インターフェースの確認と、通知の表示の確認は手動とする。

### 4.3. セキュリティテスト

architecture §7.3 を 2-1〜2-3 のテストに書く。(1) AC-12（本文を読まない）: 読まれたバイト数を数える `io.Reader` を本文に差し込み、ヘッダの検査の各手順で失敗させて読まれたバイト数が 0 であること、上限超えの本文が `413` になり読まれたバイト数が上限 + 1 を超えないことを確かめる。上限超えの本文と誤ったトークンの組み合わせが `413` ではなく `401` になり、読まれたバイト数が 0 であることも確かめる（AC-12）。(2) 秘密の漏洩: API キー・Webhook URL・共有トークンに目印を入れ、すべての応答・ログの出力（`redact.Writer` を通した後）・fake の `Notifier` に渡された `Notice` に目印と字幕の目印が現れないことを確かめる。(3) 各検査のテストは、その検査を外した実装で失敗することを確かめ、コミットメッセージに書く（CLAUDE.md「Every test must be able to fail for its stated reason」）。

### 4.4. テストヘルパー

新しい `testutil/` は `internal/failure/testutil` の 1 つだけである（1-1）。既存の fake（`ArticleWriter`・`Publisher`）はそのまま使う。`internal/server` のテスト内の補助は `internal/server/test_helpers.go`（`//go:build test`）に置く。`cmd/yt2column-server` のテスト内の補助は、統合テストも `//go:build test` なので、すべて `cmd/yt2column-server/test_helpers.go`（`//go:build test`）に置く。`integration` タグのビルドから見える補助（`test_helpers_integration.go`）は作らず、既存の部品（`internal/publisher/test_helpers_slack.go`・`internal/llm/testutil`・#108 の `NewSlackFailureNotifierForLoopbackTest`）のビルドタグも広げない。子プロセスを起動する行には `//nolint:gosec` を付ける（`cmd/yt2column/test_helpers.go:270` と同じ）。`TestFakesCarryBuildTag` が数える `testutil/` のファイル数は 17 から 18 になる（1-1）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

各 AC の検証を実行する。`test` は実行され誤った振る舞いで落ちるテスト、`static` はガードテストまたは `make` ターゲット、`manual` は PR での観察を指す。ファイルのパスは `path::TestName` 形式で記す。

| AC | 区分 | 検証 |
|---|---|---|
| AC-01 | test / manual | `internal/server/server_test.go::TestListenBindsLoopbackOnly`（`Listen` が返す `net.Listener` のアドレスが `127.0.0.1:<ポート>` と `[::1]:<ポート>` だけである）。外部インターフェースのアドレスからの接続の拒否は、フェーズ 2 の手動の確認として記録する |
| AC-02 | test / static | `internal/authtoken/authtoken_test.go::TestLoadOrCreateCreates`・`::TestLoadOrCreateReadsAgain`（生成の権限 `0600`・64 文字・`created`・再読み込み）、`cmd/yt2column-server/run_test.go::TestTokenShownOnlyWhenCreated`（生成時だけ標準出力に 1 行）、`cmd/yt2column-server/run_test.go::TestOutputsOmitToken`。統合テスト `cmd/yt2column-server/server_test.go::TestServerEndToEnd`（生成から再起動後の認証） |
| AC-03 | test | `cmd/yt2column-server/run_test.go::TestStartupFailures`（設定の欠落・不正なトークンのファイルで待ち受けを開始せず終了コード 2）、`internal/authtoken/authtoken_test.go::TestLoadOrCreateRejectsInvalidFile`（各理由と、ファイルを変更しないこと。所有者の不一致は所有者を変えられる環境でだけ）、`::TestFileErrorMessageOmitsContent`（`FileError` のメッセージがファイルの内容を含まないことを単独で確かめる） |
| AC-04 | test | `internal/server/request_test.go::TestAcceptValidRequest`（`202`、`status`=`accepted`） |
| AC-05 | test | `internal/server/request_test.go::TestRejectBodyTable`（`videoUrl`・`transcript` の欠落・非文字列・空・空白のみ） |
| AC-06 | test | `internal/server/request_test.go::TestOptionalMembers`（欠落・空を受理し `Transcript` が空文字列、非文字列・`null` を拒否） |
| AC-07 | test | `internal/server/request_test.go::TestUnknownMemberIgnored` |
| AC-08 | test | `internal/server/handler_test.go::TestRejectHeaders`（`Content-Type` の不一致、`POST` 以外） |
| AC-09 | test | `internal/server/request_test.go::TestRejectVideoURLTable`（`transcript.ValidateVideoURL` が拒否する形） |
| AC-10 | test | `internal/server/request_test.go::TestRejectBodyTable`（空・空白のみの `transcript`） |
| AC-11 | test | `internal/server/request_test.go::TestRejectInvalidTranscript`（#107 の実物の `NewCopiedSource` が返す `ErrCopiedTranscriptFormat`） |
| AC-12 | test | `internal/server/request_test.go::TestHeaderRejectionReadsNoBody`（読み込みを数える `io.Reader` で 0 バイト。上限超えの本文と誤ったトークンの組み合わせも `401` で 0 バイト）、`::TestBodyTooLarge`（`Content-Length` あり・なしの `413`、読まれたバイト数が上限 + 1 以下）。統合テスト `::TestServerEndToEnd` で実際のソケットの結果を記録する |
| AC-13 | test | `internal/server/request_test.go::TestRejectBodyTable`（無効な UTF-8・対になっていないサロゲート。宣言していないメンバの中も含む） |
| AC-14 | test | `internal/server/handler_test.go::TestRejectHeaders`（`Host` の大文字・ポートの省略・非 loopback） |
| AC-15 | test | `internal/server/handler_test.go::TestRejectHeaders`（`Origin` の欠落・不一致・複数） |
| AC-16 | test | `internal/server/handler_test.go::TestRejectHeaders`（トークンの欠落・不一致・`Bearer ` でない・長さ違い、複数） |
| AC-17 | test | `internal/server/handler_test.go::TestRejectHeaders`（fake の `ArticleWriter`・`Publisher`・`Notifier` が呼ばれない）、`internal/server/request_test.go::TestHeaderRejectionReadsNoBody`・`::TestBodyTooLarge`（fake の `Notifier` が呼ばれない） |
| AC-18 | test | `internal/server/job_test.go::TestAcceptStartsJob`（`202`、fake の `Publisher` が呼ばれる） |
| AC-19 | test | `internal/server/job_test.go::TestJobTranscriptValues`（fake の `ArticleWriter` が受け取った `Transcript` の `VideoID`・`VideoURL` が導出値と一致し、`videoUrl` の文字列と異なる） |
| AC-20 | test | `cmd/yt2column-server/run_test.go::TestBuildsSlackPublisher`（構築される `Publisher` が `*publisher.SlackWebhookPublisher` である）、統合テスト `::TestServerEndToEnd` |
| AC-21 | test | `internal/server/response_test.go::TestResponseOmitsInputs`（`transcript`・`title`・`channelName`・`description`・API キー・Webhook URL の目印が応答の本文に現れない） |
| AC-22 | test | `internal/server/job_test.go::TestSlotLimit`（fake の `ArticleWriter` をブロックさせ、次のリクエストが `429` になり、ブロックを解くと処理中のジョブが完了する。fake の `Notifier` は呼ばれない）、`::TestSlotReleasedOnce`（本文の検査の失敗・本文の読み込みの失敗・ハンドラのパニックの後に、枠がちょうど 1 つ解放される。architecture §7.1） |
| AC-23 | test | `internal/server/job_test.go::TestFailureClassification`（fake の `ArticleWriter`・`Publisher` に各番兵を返させ、fake の `Notifier` が §3.9 の表のとおりの `Kind` で呼ばれる。`rejectReason` のすべての値についての対応も含む）。統合テスト `::TestServerEndToEnd`（実物の通知を `publisher.NewSlackFailureNotifierForLoopbackTest` で組み、loopback の Webhook へ通す） |
| AC-24 | test | `internal/server/job_test.go::TestFailureClassification`・`::TestNotifyNoticeFields`（fake の `Notifier` が受け取った `Notice` が `RequestID`・`VideoID`・`Kind` だけで組み立てられ、字幕・メタデータ・秘密を含まない） |
| AC-25 | test | `internal/server/handler_test.go::TestRejectHeaders`（fake の `Notifier` が呼ばれない） |
| AC-26 | test | `internal/server/job_test.go::TestNotifyErrorContinues`（fake の `Notifier` がエラーを返しても、時間切れになっても、サーバが次のリクエストを処理でき、その失敗が `LogOutput` のログに記録される） |
| AC-27 | test | `internal/server/handler_test.go::TestRejectStatusCodes`（§3.4.3 の状態コード・`reason`・追加のヘッダ）、`internal/server/response_test.go::TestResponseOmitsInputs`、`::TestRejectReasonZeroValueIs500` |
| AC-28 | test | `internal/server/job_test.go::TestPanicRecycledAsInternal`（ハンドラのパニックが `500` と `KindInternal` の通知になり、応答に秘密を含まない。ジョブと通知の goroutine のパニックは、`202` を返した後なので応答を変えず、回収されて `KindInternal` の通知とログになる。architecture §3.8.4 の 1・§7.1） |
| AC-29 | test | `internal/config/config_test.go::TestLoadServerModelMissing`（`errors.Is(err, config.ErrMissing)`）、`cmd/yt2column-server/run_test.go::TestStartupFailures` |
| AC-30 | test | `internal/config/config_test.go::TestLoadServerWebhookRequired`（未設定・不正。エラーに値が現れない）、`cmd/yt2column-server/run_test.go::TestStartupFailures` |
| AC-31 | static | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（CLAUDE.md・project_overview.md・security.md・README.md・translation_glossary.md の必須の内容。実装と食い違いうる値は実装の定数・表から導く）。文書だけの PR でも走るよう、`doc-guard` の CI ジョブに `./cmd/yt2column-server` を加える。既存の `cmd/yt2column/docs_test.go::TestREADMEDocumentsFlags`・`::TestSlackDocsContract`・`::TestClaudeDocsContract`、`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages` |

手動の確認（PR に記録する）: AC-01 の外部インターフェースの拒否、通知の表示（`video:` の行の `<URL>` のリンクと改行）の Mattermost での確認、4.2 の統合テストの結果。

## 6. リスクと対策 (Risk Management)

| リスク | 対策 |
|---|---|
| 1-2・1-3 の移動・分割で CLI の振る舞いが変わる | 既存テストを内容を変えずに通し、移したテストは `go tool cover -func` でカバレッジの変化を確かめる（§1.2） |
| 新しいパッケージの追加で `TestPackageReferenceListsPackages`・`TestFakesCarryBuildTag` が落ちる | パッケージを加える同じステップで `package_reference.md` の行と件数を更新する（§1.3） |
| `internal/config` が `net` を import して `TestConfigImports` が落ちる | ポートの検査は `strconv` と文字の比較で行う（1-4） |
| 実装の手順と architecture が食い違う（検査の順序、上限、状態コード） | architecture を唯一の設計の出典とし、変える場合は architecture を先に直す（architecture §3.5.2・§3.4.3・§3.5.3） |
| #108 の実物が無いために 2-4 の統合テストが書けない | 2-1〜2-3 のユニットテストは fake の `Notifier` で進める。2-4 の統合テストは #108 のマージ後に行う（§3.1） |
| 文書の更新で既存の doc ガードが落ちる | 2-5 で `TestREADMEDocumentsFlags`・`TestSlackDocsContract`・`TestClaudeDocsContract`・`TestExtensionDocumentation` を確認する |
| architecture §7.2 の旧方式の統合テスト（実際のバイナリ、`integration` タグだけ）が実装できない（LLM のエンドポイントを差し替えられない） | architecture §7.2 を `//go:build test` と `deps` の差し替えの方式に改めて `draft` に戻し、#222 で再承認を受けた（フェーズ 2 の阻害要因は解消済み。フェーズ 1・2-1〜2-3 はこの修正に依存しない） |
| 子プロセスや `net.Listener` のリークで再実行が `EADDRINUSE` を起こしたり不安定になったりする | 起動・取得の時点で `t.Cleanup` に kill と reap・close を登録し、再起動の前にポートの解放を待つ（§4.2） |

## 7. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1）
- [x] PR-2 マージ済み（対象ステップ: 1-2）
- [x] PR-3 マージ済み（対象ステップ: 1-3 / 1-4）
- [x] PR-4 マージ済み（対象ステップ: 1-5）
- [ ] PR-5 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3）
- [ ] PR-6 マージ済み（対象ステップ: 2-4）
- [ ] PR-7 マージ済み（対象ステップ: 2-5）
- [ ] 各 PR でグリーンゲート（`make test && make lint`）が通った
- [ ] AC-01 の外部インターフェースの確認と、通知の表示の Mattermost での確認を PR に記録した

## 8. 完了条件 (Success Criteria)

- requirements の F-001〜F-010 を、architecture の設計どおりに実装している。
- §5 の AC の検証がすべて実行され、実行されないものは手動の確認として PR に記録されている。
- グリーンゲート（`make test && make lint`）が通る。#108 のマージ後に、§4.2 の統合テストを含めて `make test` が通る。
- 秘密（API キー・Webhook URL・共有トークン）と字幕の本文が、応答・ログ・失敗の通知に現れない。
- `package_reference.md`・文書のガードが、新しいパッケージと文書の内容を固定している。

## 9. 次のステップ (Next Steps)

- 本計画の承認後、PR-1 から順に実装する。各 PR の後にグリーンゲートを通す。
- 1-1（PR-1）の後、`internal/failure` の契約をタスク 0016 のフェーズ 2（`SlackFailureNotifier`）が使える状態にする。
- 2-4 の統合テスト（実物の通知）は #108（タスク 0016）のマージ後に行う。
- 2-5 の後、#111（拡張の送信）が本 API に従える状態にする。
