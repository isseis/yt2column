# 実装計画書：OpenAI API のアダプタを追加する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-10 |
| Review date | 2026-11-10 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装概要 (Implementation Overview)

### 1.1 目的と実装方針

[要件定義書](01_requirements.md)の AC-01〜25 を、[設計書](02_architecture.md)に従って実装する。OpenAI の選択、Responses API のアダプタ、料金、再開、CLI・サーバの利用案内をそろえる。本番の既定は変更しない。

設計書 §8 の4フェーズを維持し、フェーズ2の前提となる使用量・中間生成物の変更を独立した PR にする。新しい構築口・テスト名は以下で「新規」と記す。既存の契約・判定規則は設計書を参照し、本書では実装箇所と実行する検証を定める。製品の実装は本書の承認後に開始する。

### 1.2 計画の際に決めたこと（2026-11-10）

- **実装は5本の PR に分ける。** レビュアーが推奨案を選択した。設定、料金条件と版3/4の互換性、アダプタ、配線と料金、統合テストと利用案内の順にする。再開の互換性をアダプタの解析処理から分離してレビューするためである。実装の契約は承認済み設計のままなので ADR の追加・更新は不要。
- API、Standard、30分のキャッシュ、既定上限6,080、料金条件、版4と版3の読解は設計書 §1.2・§1.4・§3.4 で確定済み。ADR-0001・0003・0018・0021・0022・0023・0041 を適用し、再決定しない。`implementation_handoff.md` は存在しない。設計への申し送り H-01〜06 は設計書 §3.3・§3.4・§1.4・§3.1・§7.2 に記録済み。

### 1.3 既存コード調査結果

調査の基準は `e5412181f2f751e872d8080b0685079ada7f33f1`。下表の行番号はこのコミットを指す。

| 対象 | 既存の部品と変更点 |
|---|---|
| 設定 | `internal/config/config.go:29` の provider 列挙、同ファイルの `parseLLM`・`loadProviderVars` を拡張する。`serverfile.go` の許可リスト・雛形、`envaccess_test.go:59` の秘密名も合わせる。既存の provider・effort・再開のテスト表を拡張し、同じ表を別に作らない |
| 通信と JSON | `internal/llm/llmhttp/llmhttp.go:70` の `Post`、`internal/strictjson/strictjson.go:77` の `ParseObject` と `Collect` を再利用する。OpenAI の要求・消費メンバーの検査・終了判定は新設する。DeepSeek の応答処理は共通化しない |
| 使用量 | `internal/llm/usage.go:35` の `Usage` と `KnownUsage`・`UnknownUsage` に料金条件を加える。既存の全構築箇所は旧2プロバイダ、writer、artifact、fake、テストにある。既存構築口の条件をゼロ値として保ち、OpenAI と版4の読解だけで新構築口を使う |
| 中間生成物 | `internal/artifact/artifact.go:19` と `format.go:236` は版3のみを前提にし、`format.go:488` の `recordID` も現在版を使う。版別の厳格な検証と識別子計算を分ける。`Parse`・`Marshal` の利用者である `Store.Load`・`Store.Save` と writer の再開を、既存のテストに加えて混在連鎖で検証する |
| 料金 | `internal/pricing/pricing.go:99` の `Estimate` と `:171` の `chooseRates`、`rates.go` を拡張する。新しい不明理由の利用者は CLI の `costReasonWord` と `internal/pricing/testutil/helpers.go`。トークン数の表示と料金の既知・不明を分離する |
| 配線と秘密 | `internal/llm/provider/provider.go:87` の `newClient` と `builders`・`productionBuilders` を拡張する。`cmd/yt2column/run.go:301` と `cmd/yt2column-server/run.go:191` の `redact.SecretStrings` の両方に新しいキーを加える |
| テスト用の部品 | `internal/llm/llmhttp/llmhttptest` と `internal/llm/testutil` の fake を再利用する。アダプタの非公開構築口には新しい `test_helpers_endpoint.go` が必要。統合テストの純粋な設定判定は既存 Claude の `testutil/integration.go` の方式を参考に OpenAI 用に置く |
| ガードと利用案内 | `internal/pipeline/pipeline_test.go` の `TestConfigImports`・`TestFakesCarryBuildTag`、`cmd/yt2column/docs_test.go` の effort・既定・使用量の値のガードを同じ変更で更新する。設計書 §3.5 の全利用案内を対象にし、旧タスクの履歴は現行仕様に書き換えない |

全件調査では `KnownUsage`・`UnknownUsage`・`Usage{`、`ReasonUsageUnknown`、`artifactVersion`、`version 3`・`版3`・`版 3`、`ANTHROPIC_API_KEY`、`SecretStrings` をコードと利用案内で照合した。変更箇所を次の表に集約する。

| パターンと所在 | 実装時の処置 |
|---|---|
| `Usage{` とフィールド代入：`internal/llm/usage.go`、同パッケージのテスト | 既存のゼロ値・比較可能性を維持する。具体型での特別扱いや構築口だけに依存した検査は導入しない |
| `KnownUsage`・`UnknownUsage`：旧アダプタ・writer・artifact・fake・テスト | 旧呼び出しの意味を保ち、usage の不整合理由を追加する。保存・読解は版別の条件を反映する |
| `artifactVersion`・版3の検証とコメント：`artifact.go`・`format.go`・`format_test.go`、利用案内の版の説明 | 書く版と読める版を分け、版3の検証を残す。版3のみとする現行のコメント・診断・ガイドを新しい契約に合わせる |
| 料金の不明理由：`pricing.go`、CLI `run.go`・`cost_test.go`、pricing の `testutil` | 列挙、表示、テストを同じ PR で追加する |
| キー一覧・provider の案内：config、両入口、Makefile、設計書 §3.5 の利用案内 | 秘密の入力元・伏せ字・統合テスト用のキーを一貫させる。yt-dlp の子プロセスの環境許可リストは広げない |

改名・削除するシンボルとテストはない。既存テストは期待値と対象ケースを拡張する。具体型に依存した例外やガードは新設しない。

## 2. 実装手順 (Implementation Steps)

### フェーズ1：設定・effort・キーの経路

#### ステップ 1-1：OpenAI の設定を読み取る

**変更ファイル:** `internal/llm/openaiparam/openaiparam.go`・`openaiparam_test.go`（新規）、`internal/config/config.go`・`config_test.go`・`test_helpers.go`。

- [x] 設計書 §3.1 の effort 型・一覧・解析と `ProviderOpenAI`、工程の選択、キーの取得口を加える。
- [x] `test_helpers.go` の `EffortValuesByProviderForTest` と OpenAI の既定 effort の取得口を更新し、既存の設定テストを拡張して、7 effort、既定・継承、未知モデルと `/`、不正な選択、両方向の混在を検証する。
- [x] キーの欠け・空、未選択のキーの無視、再開で必要な工程だけを読むことを検証する。旧2プロバイダと本番の既定を維持する。

#### ステップ 1-2：サーバの入力境界とガードをそろえる

**変更ファイル:** `internal/config/serverfile.go`・`serverfile_test.go`・`envaccess_test.go`、`internal/pipeline/pipeline_test.go`、`cmd/yt2column/docs_test.go`・`cmd/yt2column-server/docs_test.go`、`README.md`・`README.en.md`、`docs/dev/project_overview.md`・`docs/dev/security.md`・`docs/dev/conventions/package_reference.md`。

- [x] 許可する7キーとコメントだけの雛形を追加し、既存6キーのファイルも読み込む。環境による補完を設けない。
- [x] `TestParseServerFileAccepts`・`TestParseServerFileRejects`・`TestServerFileTemplate` と設定のテストを拡張し、書式・サイズ・所有者・権限・未知キーの拒否を保つ。
- [x] `TestEnvAccessConfined` の秘密名と `TestConfigImports` の許可パッケージを更新する。新しい環境読み込みを入口と config 以外に広げない。

- [x] `TestEffortValuesDocsContract` と `TestServerDocsContract` が参照する provider・effort・7キーの値を両 README・project_overview・security に反映する。`openaiparam` をパッケージ参照表に加え、`TestPackageReferenceListsPackages` を実行する。README は同じコミットで `/mktrans README.md` により同期する。案内では配線が後続の PR であることを区別する。

**完了条件:** 対象テスト、README 同期、`make ext-test` とグリーンゲートが通る。この時点では provider の通信経路を追加しない。

### PR-1 作成ポイント: configuration and effort

**対象ステップ**: 1-1 / 1-2

**推奨タイトル**: `feat(0077): OpenAIの設定とeffortの型を追加する`

**レビュー観点**: 工程の継承と再開 / 7キーの入力境界 / ガードと日英同期

**実装モデル要件**: standard

**判定理由**: 確定済みの設定拡張であり、上位モデルを要する条件に該当しない。

- [x] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/792
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ2：OpenAI の要求・応答・使用量・エラー

#### ステップ 2-1：料金の適用条件を共有する

**変更ファイル:** `internal/llm/usage.go`・`usage_test.go`。

- [x] 設計書 §3.4.1 の `PricingContext` と構築・取得口、および内訳の不整合を示す固定の不明理由を追加する。
- [x] 新規 `TestKnownUsageWithPricingContext` で列挙外の拒否、既存構築口の条件、比較可能性、既知の数と条件の分離を検証する。未知の使用量の条件は `unspecified` とする。

#### ステップ 2-2：版4を書き、版3の再開を保つ

**変更ファイル:** `internal/artifact/artifact.go`・`format.go`・`format_test.go`・`store_test.go`・`chain_test.go`、`cmd/yt2column/docs_test.go`、版の説明を持つ `docs/dev/guides/cli.md`・`docs/dev/project_overview.md`。

- [x] 設計書 §3.4.2 に従い版4の必須条件と識別子を保存し、版3は元の形式・識別子で検証する。版1・2・未知の版の拒否を保つ。
- [x] 既存の round-trip・不正 usage・識別子のテストを拡張する。新規 `TestPricingContextFormat` で条件の欠け・重複・型違い・未知値、未知 usage との不整合、条件だけの改変を検証する。
- [x] 新規 `TestVersion3Compatibility` で変更前の版3のバイト列と識別子を使い、元の時刻・使用量・識別子を返すことを検証する。版3に版4のキーを加えた入力と版を偽った入力も拒否する。
- [x] 新規 `TestMixedVersionResume` を `chain_test.go` に加え、版3から版4への連鎖と再開を検証する。再利用したファイルのバイト列が変わらず、新しい工程が元の識別子を参照することを確認する。
- [x] 版についての現行コメント・診断・利用案内を更新し、既存の文書ガードを同じ PR で合わせる。自然言語は手動で読み、版の値は実装と照合する。

**完了条件:** PR-2 は PR-3 の前提作業として完了させる。既存2プロバイダの round-trip と料金の回帰、混在連鎖、グリーンゲートが通る。

### PR-2 作成ポイント: pricing context and artifact compatibility

**対象ステップ**: 2-1 / 2-2

**推奨タイトル**: `feat(0077): 料金条件を保存し版3と版4の再開を支える`

**レビュー観点**: 旧版の識別子 / 版3/4の混在連鎖と再利用ファイルの保持 / 条件の厳格な検証

**実装モデル要件**: frontier-required

**判定理由**: 2-2 が保存形式の移行と旧版の受理範囲の拡張を含み、移行時のレビュー条件に該当する。

- [x] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/797
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 2-3：OpenAI アダプタとループバックの検証を追加する

**追加ファイル:** `internal/llm/openai/` の `openai.go`・`request.go`・`response.go`・`errors.go`・`openai_test.go`・`response_test.go`・`test_helpers.go`・`test_helpers_endpoint.go`。
**変更ファイル:** `docs/dev/conventions/package_reference.md`。

- [x] 設計書 §3.2 の非公開クライアントと `Options`・`New` を追加し、共有の要求検証・HTTP 送信・strictjson を使う。
- [x] 新規 `TestNew`・`TestGenerateRequest` で構築と要求の拒否時の送信0回、固定先、認証、役割、effort、正の上限と既定上限、保存・方式・TTL の設定を記録した要求で検証する。
- [x] 設計書 §3.3 と要件 F-003 の全消費境界を検査する。新規 `TestResponseShape` で各階層の欠け・重複・型、エスケープされた同名キー、大文字違い、未知種類、本文の不正と途中の不正要素を検証する。拡張可能なメンバーの扱いも確認する。
- [x] 新規 `TestResponseFinish` で最終回答の順序、phase、区切りなしの連結、モデルと版、終了の優先順位、拒否・空・打ち切りを検証する。3種の失敗のモデル・使用量と、それ以外のゼロ値を区別する。
- [x] 新規 `TestResponseUsage` で全消費境界の整数・内訳・桁あふれ、未知 usage の理由、推論の二重加算を検証する。成功と3種の失敗について、不正な使用量だけで本文の結果を変えないことを確認する。
- [x] 新規 `TestResponsePricingContext` で `service_tier` の欠け・重複・型違い・未知値と確認済み条件を検証する。有効なトークン数と固定の条件だけを返す。
- [x] 新規 `TestGenerateTransport` でリダイレクト拒否、HTTP 状態と固定案内、送信回数、送信中・本文読取中の期限・キャンセル・切断、本文上限を検証する。未知値や拒否文に偽の秘密・制御文字を含め、結果と診断に出さない。
- [x] 非公開構築口を使うテスト補助には `test` タグを付け、ループバックだけを許可する。`llmhttptest` の既存ケースを再利用し、共有部品自体の同じテスト表は作り直さない。

- [x] `openai` をパッケージ参照表に加え、`TestPackageReferenceListsPackages` を実行する。

**完了条件:** 新しいアダプタのユニットテスト、旧アダプタの回帰、グリーンゲートが通る。実 API は呼ばない。

### PR-3 作成ポイント: Responses API adapter

**対象ステップ**: 2-3

**推奨タイトル**: `feat(0077): OpenAIのResponses APIアダプタを追加する`

**レビュー観点**: 消費境界と終了の優先順位 / 使用量と料金条件 / 通信と秘密

**実装モデル要件**: frontier-recommended

**判定理由**: 2-3 が応答の状態判定を含む複雑な処理であり、リスクを独立してレビューする。

- [x] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/806
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ3：両工程の配線・伏せ字・料金

#### ステップ 3-1：両入口へ接続し、秘密を伏せる

**変更ファイル:** `internal/llm/provider/provider.go`・`provider_test.go`、`cmd/yt2column/run.go`・`run_test.go`、`cmd/yt2column-server/run.go`・`run_test.go`。

- [ ] `builders`・`productionBuilders`・`newClient` に OpenAI を追加し、生成・調整と検証・推敲の選択を正しく渡す。既存の provider の spy とループバックの検証を拡張する。
- [ ] 両入口の `redact.SecretStrings` に OpenAI のキーを加え、偽のキーと末尾断片をエラー・ログ・メタデータ・一覧へ出さないことを既存の秘密検証と新規 `TestOpenAIOutputsOmitSecrets` で確認する。
- [ ] 両入口に新規 `TestOpenAIConfiguration` を追加する。設定からクライアント・工程の結果までを偽の依存で通し、欠け・空のキーの終了コード2と送信0回、サーバの環境無視、再開時の設定範囲、混在を確認する。
- [ ] `provider_test.go` に新規 `TestOpenAIDefaultMaxTokensFitsBudget` を加え、実際の要求の既定上限と設計書 §1.4 の観測最大・最低速度・モデルの最大出力・`LLMTimeout` の関係を検証する。正の上限はそのまま送り、3工程で既定を共有することも確認する。

#### ステップ 3-2：料金と工程の記録を確認する

**変更ファイル:** `internal/pricing/pricing.go`・`rates.go`・`rates_test.go`・`pricing_test.go`、`internal/pricing/testutil/helpers.go`・`helpers_test.go`、`cmd/yt2column/run.go`・`cost_test.go`。

- [ ] 設計書 §3.4 の2モデルの単価・条件・しきい値と新しい不明理由を追加する。CLI と統合テスト用の表示を同時に追加する。
- [ ] 新規 `TestEstimateOpenAI` で独立した手計算、入力合計272,000と272,001、キャッシュだけでしきい値を越える場合、未登録の応答モデル、条件の不一致を検証する。既存2プロバイダの単価・計算を維持する。
- [ ] `TestRunCostLinesAllStages`・`TestRunCostLinesUnknown`・`TestRunCostLinesFailure`・`TestRunCostLinesResume` を拡張し、数が既知でも料金が不明のケースを含める。新規 `TestRunOpenAIUsageRecords` で工程・記事・中間生成物への4区分とモデル、再開時の記録の保持を確認する。
- [ ] 失敗時に記事を保存・表示・投稿しないことと、料金が不明でも生成・保存・公開が継続することを CLI とサーバの偽の依存で検証する。

**完了条件:** 3プロバイダを両方向に混在させた配線、秘密、不明の料金、再開の検証とグリーンゲートが通る。

### PR-4 作成ポイント: provider wiring and cost estimates

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `feat(0077): OpenAIを両工程に接続し料金を見積もる`

**レビュー観点**: 両方向の混在 / 両入口の伏せ字 / 条件付きの料金と再開

**実装モデル要件**: standard

**判定理由**: 設計済みの配線と料金表の拡張であり、上位モデルを要する条件に該当しない。

- [ ] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ4：統合テスト・実応答・利用案内

#### ステップ 4-1：実 API のテストを明示的な実行に限る

**追加ファイル:** `internal/llm/openai/integration_env_test.go`・`integration_test.go`・`makefile_test.go`、`testutil/integration.go`・`integration_settings_test.go`。
**変更ファイル:** `Makefile`、`internal/pipeline/pipeline_test.go`。

- [ ] 設計書 §7.2 の純粋な設定判定と薄い実行ラッパーを追加する。偽 lookup で未指定・不正・正常伝播を検証し、本番のキーへアクセスしないこと、オプトインと HTTP/2 デバッグの拒否を新規 `TestIntegrationSettings` で確認する。
- [ ] 課金の注意・呼び出し数・既定モデルと effort・全体の期限を Make ターゲットに定義し、新規 `TestMakeIntegrationOpenAI` で実 API を呼ばず検証する。他のオプトインを有効にしない。
- [ ] 新規 `TestIntegrationResponses` で独自の短い英語プロンプトによる成功と打ち切りを確認する。前提が成立しない場合は報告し、再送しない。ログの秘密の伏せ字と表示のエスケープを行い、本文を出力しない。
- [ ] `openaitestutil` の共有ファイルは `test || integration` タグ、非公開の構築口だけを使う補助は `test` タグにする。導入時に実際のタグでコンパイルを確認し、`TestFakesCarryBuildTag` の固定ファイル数とパッケージ参照表の `openai/testutil` 行を同じ PR で更新する。

#### ステップ 4-2：実応答を取得し、利用案内を同期する

**追加ファイル:** `testdata/openai_responses_completed.json`・`openai_responses_incomplete.json`、`docs/dev/guides/openai_output_limit.md`。
**変更ファイル:** `testdata/README.md`、`README.md`・`README.en.md`・`CLAUDE.md`、`docs/dev/project_overview.md`・`security.md`・`conventions/package_reference.md`・`guides/development.md`・`guides/cli.md`・`guides/pricing_update.md`、`cmd/yt2column/docs_test.go`。

- [ ] 取得に必要な実行条件・回数・テスト専用キー・加工手順を準備してから、別途その実行の許可を得る。設計時の測定許可は流用しない。
- [ ] 許可された範囲で独自の短いプロンプトの成功・打ち切りの応答を取得する。API キーと取得者を識別する情報を除き、構造を保つ。取得日・条件・出典・加工箇所・再取得手順を記録する。新規 `TestCapturedResponses` で製品の解析経路から期待する結果を検証する。
- [ ] 設定・課金・effort・未対応時の失敗・保持の説明・版3/4の互換性・パッケージ参照を実装に合わせる。上限のガイドは設計書 §1.4 の測定結果と再測定条件を参照し、新しい計測を実行しない。
- [ ] API の値・単価・保持の記述は設計書が挙げる公式出典を再確認して確認日を記す。コマンド例は安全なローカル実行または Make の検査で値と経路を確かめ、有料の例は許可なく実行しない。各文書に実装の参照先を示す。
- [ ] 既存 `TestEffortValuesDocsContract` 等を拡張し、新規 `TestOpenAIDocsContract` で設定名・effort・統合テストの値を実装と照合する。文の意味は手動で読み、単語の有無を固定しない。
- [ ] 日本語 README の変更と同じコミットで `/mktrans README.md` により英語版を同期する。日英差分と用語集をレビューする。

**完了条件:** 実応答の検証、文書の値のガード、手動の読解、README 同期、グリーンゲートと必要な拡張テストが通る。取得が未許可・未完了なら AC-23 を完了扱いにしない。

### PR-5 作成ポイント: integration fixtures and documentation

**対象ステップ**: 4-1 / 4-2

**推奨タイトル**: `test(0077): OpenAIの実応答と統合テストと利用案内を追加する`

**レビュー観点**: 有料実行のオプトイン / 実応答の加工 / 値のガードと日英同期

**実装モデル要件**: frontier-recommended

**判定理由**: 4-1 が環境による実行判定とビルドタグ別の補助を導入し、複数の条件付きチェックに該当する。

- [ ] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1 順序

設定 → 使用量と保存・再開 → アダプタ → 配線と料金 → 統合テストと利用案内の順に進める。各 PR で依存する型と利用者・ガードをそろえ、後続 PR のスタブを置かずビルド可能にする。PR-1〜3 の段階では OpenAI の選択が読み取れてもアプリケーションの通信には接続しない。機能が利用可能になるのは PR-4、タスク全体の完了は PR-5 とする。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 | OpenAIの設定とeffortの型を追加する | standard |
| PR-2 | 2-1 / 2-2 | 料金条件を保存し版3と版4の再開を支える | frontier-required |
| PR-3 | 2-3 | OpenAIのResponses APIアダプタを追加する | frontier-recommended |
| PR-4 | 3-1 / 3-2 | OpenAIを両工程に接続し料金を見積もる | standard |
| PR-5 | 4-1 / 4-2 | OpenAIの実応答と統合テストと利用案内を追加する | frontier-recommended |

## 4. テスト戦略 (Test Strategy)

ユニットテストは偽のキー、既存の fake、ループバック HTTP、`t.TempDir` を使う。ネットワークの資源は取得時に cleanup を登録する。実サービスの応答ID・日時を固定比較せず、消費する構造と分類を検証する。永続的な外部の資源を作らず、統合テストは保存応答の一覧や順序に依存しない。

`*_test.go` 以外の新しい補助ファイルは、導入する PR で、実際に使うビルドタグを指定してコンパイルを確認する。実 API の統合テストはオプトインを設定せずにタグ付きビルドを検証し、課金される実行は別の許可の範囲で行う。Make ターゲットの引数は既存の統合テストターゲットと Go のヘルプに照合する。実装では統合テスト用のタグだけでも補助がコンパイルされることを確認する。

変更した各テストでは実装中に対応する分岐・デフォルト・協力オブジェクトを一時的に壊して実際に失敗することを確認し、コミットに記録する。特に料金条件の改変はその条件を読み出す経路、版3の互換性は旧版の識別子の検証経路、終了の優先順位は複数条件を含む応答の分類を対象にする。

各 PR の完了時に `make test`・`make lint` を実行する。Go を編集した場合は先に `make fmt` を実行する。README または development ガイドを変更する PR-1・PR-5 では `make ext-test` も実行する。使われない構築口が残っていないことを `make deadcode` で確かめる。既存のテストは削除しない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 版3の再開が壊れる | PR-2 を独立させ、旧版のバイト列・識別子と混在連鎖を検証する。ロールバック時は版4の成果物を旧版で読めないことを案内し、自動変換しない |
| 部分応答や誤った料金の公開 | 形と終了の判定、usage と料金条件を分けて確認する。HTTP と未知の自由文を診断へ出さない |
| 秘密の漏えい | 両入口の伏せ字、固定送信先、専用キーの読み込み、統合テストのデバッグ拒否を確認する。yt-dlp の環境許可リストを維持する |
| 実 API の許可・利用権限待ち | 最終 PR の取得だけを未完了として残す。既存の測定許可を使い回さず、取得前に条件を提示する |
| 上限の測定結果・外部仕様の変動 | 設計書 §1.4 の適用限界をガイドに残す。再測定で条件が両立しなければ数値を変える前に相談する |
| 一部だけ戻してガードと実装が不一致になる | 対応する型・利用者・ガード・文書の変更を同じ単位で戻し、グリーンゲートを再実行する。旧成果物の書き換えを回避策にしない |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（1-1 / 1-2）
- [x] PR-2 マージ済み（2-1 / 2-2）
- [x] PR-3 マージ済み（2-3）
- [ ] PR-4 マージ済み（3-1 / 3-2）
- [ ] PR-5 マージ済み（4-1 / 4-2）
- [ ] 下表の全 AC の検証結果を記録した

## 7. 受け入れ条件の検証 (Acceptance Criteria Verification)

以下の「新規」は実装で追加するテストであり、この計画 PR で実行済みという意味ではない。型は `test` が振る舞いの検証、`static` がガード、`manual` が PR での読解を表す。各テストは該当ステップの PR と全体のテストで実行する。

| AC | ステップ | 型と実行する検証 |
|---|---|---|
| AC-01 | 1-1 / 2-3 / 3-1 | test: `internal/config/config_test.go::TestParseLLMProvider`、`internal/llm/openai/openai_test.go::TestGenerateRequest`（新規） |
| AC-02 | 1-1 / 2-3 | test: `internal/config/config_test.go::TestLoadParsesLLMValue`、`internal/llm/openai/openai_test.go::TestGenerateRequest`（新規） |
| AC-03 | 1-1 / 3-1 | test: `internal/llm/provider/provider_test.go::TestNewSendsStageEffort` を3プロバイダに拡張、`cmd/yt2column/run_test.go::TestOpenAIConfiguration`・`cmd/yt2column-server/run_test.go::TestOpenAIConfiguration`（新規） |
| AC-04 | 1-1 / 3-1 | test: `internal/config/config_test.go::TestLoadOpenAI`（新規、キーの欠け・空）、`cmd/yt2column/run_test.go::TestOpenAIConfiguration`・`cmd/yt2column-server/run_test.go::TestOpenAIConfiguration`（新規、キーの欠け・空の終了コードと送信回数） |
| AC-05 | 1-1 / 1-2 / 3-1 | test: `internal/config/config_test.go::TestLoadProviderVarsFollowSelection`、`TestLoadServerReadsOnlyHomeAndGodebug`、`cmd/yt2column/run_test.go::TestOpenAIConfiguration`・`cmd/yt2column-server/run_test.go::TestOpenAIConfiguration`（新規） |
| AC-06 | 1-2 | test: `internal/config/serverfile_test.go::TestParseServerFileAccepts`、`TestParseServerFileRejects`、`TestServerFileTemplate` |
| AC-07 | 2-3 | test: `internal/llm/openai/openai_test.go::TestGenerateRequest`（新規） |
| AC-08 | 2-3 | test: `internal/llm/openai/openai_test.go::TestNew`、`TestGenerateRequest`（新規、拒否と送信0回） |
| AC-09 | 2-3 / 3-1 | test: `internal/llm/openai/openai_test.go::TestGenerateRequest`、`internal/llm/provider/provider_test.go::TestOpenAIDefaultMaxTokensFitsBudget`（新規） |
| AC-10 | 3-1 / 4-2 | test: `internal/llm/provider/provider_test.go::TestOpenAIDefaultMaxTokensFitsBudget`（新規）。manual: 設計書 §1.4 の既存測定記録と上限ガイドの照合 |
| AC-11 | 2-3 | test: `internal/llm/openai/response_test.go::TestResponseShape`、`internal/llm/openai/openai_test.go::TestGenerateTransport`（新規） |
| AC-12 | 2-3 | test: `internal/llm/openai/response_test.go::TestResponseFinish`（新規） |
| AC-13 | 2-3 | test: `internal/llm/openai/response_test.go::TestResponseFinish`（新規、文面なしの打ち切りと固定の案内） |
| AC-14 | 2-3 | test: `internal/llm/openai/response_test.go::TestResponseFinish`（新規、複合異常と優先順位） |
| AC-15 | 2-3 / 3-2 | test: `internal/llm/openai/response_test.go::TestResponseFinish`、`TestResponseShape`、`cmd/yt2column/run_test.go::TestOpenAIConfiguration`・`cmd/yt2column-server/run_test.go::TestOpenAIConfiguration`（新規、失敗した記事を出さない） |
| AC-16 | 2-3 | test: `internal/llm/openai/response_test.go::TestResponseUsage`（新規） |
| AC-17 | 2-1 / 2-3 | test: `internal/llm/usage_test.go::TestUnknownReason`・`TestKnownUsageWithPricingContext`、`internal/llm/openai/response_test.go::TestResponseUsage`（新規、成功と拒否時の不明の使用量） |
| AC-18 | 2-2 / 3-2 | test: `internal/artifact/chain_test.go::TestMixedVersionResume`、`cmd/yt2column/run_test.go::TestRunOpenAIUsageRecords`（新規）、`cmd/yt2column/cost_test.go::TestRunCostLinesResume` |
| AC-19 | 2-3 / 3-2 | test: `internal/pricing/pricing_test.go::TestEstimateOpenAI`、`internal/llm/openai/response_test.go::TestResponsePricingContext`（新規）、`cmd/yt2column/cost_test.go::TestRunCostLinesUnknown`。static: `internal/pricing/rates_test.go::TestRateTableSourceComments`。manual: 公式単価との照合 |
| AC-20 | 2-3 / 3-1 | test: `internal/llm/openai/openai_test.go::TestGenerateRequest`、`TestGenerateTransport`、`internal/llm/provider/provider_test.go::TestOpenAIDefaultMaxTokensFitsBudget`（新規） |
| AC-21 | 2-3 | test: `internal/llm/openai/openai_test.go::TestGenerateTransport`（新規） |
| AC-22 | 1-2 / 2-3 / 3-1 / 4-1 / 4-2 | test: `cmd/yt2column/run_test.go::TestOpenAIOutputsOmitSecrets`・`cmd/yt2column-server/run_test.go::TestOpenAIOutputsOmitSecrets`、`internal/llm/openai/testutil/integration_settings_test.go::TestIntegrationSettings`（新規）。static: `internal/config/envaccess_test.go::TestEnvAccessConfined`。manual: 保存サンプルの加工確認 |
| AC-23 | 4-1 / 4-2 | test: `internal/llm/openai/response_test.go::TestCapturedResponses`、`internal/llm/openai/integration_test.go::TestIntegrationResponses`（新規、後者は別途許可）。manual: 出典・取得条件・加工・再取得の記録 |
| AC-24 | 1-1 / 4-2 | test: `internal/config/config_test.go::TestLoadProductionDefaults`、`make ext-test`。static: `cmd/yt2column/docs_test.go::TestEffortValuesDocsContract`、`TestOpenAIDocsContract`（新規）、`make readme-sync-check`。manual: 日英 README と project_overview・CLAUDE・cli・development・pricing_update・package_reference・上限ガイドを実装と照合 |
| AC-25 | 4-2 | manual: `docs/dev/security.md` と README の送信データ・保存範囲・監視ログ・キャッシュの説明を公式出典と照合。自然言語の内容だけを定める AC として手動で検証 |

## 8. 完了条件 (Success Criteria)

- [ ] AC-01〜25 の検証が完了し、失敗した応答を記事として出力しない。
- [ ] 既存2プロバイダの設定・料金・再開と、本番の既定の回帰が通る。
- [ ] 設計書 §5 と security ガイドの秘密・通信・信頼できない入力の境界を確認した。
- [ ] 実応答の加工記録、利用案内、日英同期を含めてグリーンゲートが通る。

## 9. 次のステップ (Next Steps)

本書の PR をレビューし、`/runpr` で承認・マージ後、`/runplan 0077` で PR-1 から順に進める。実装完了後、モデルの組み合わせの品質評価と本番構成の判断を #569 に引き継ぐ。

## 変更履歴

- 2026-11-10 承認（isseis、#784）
