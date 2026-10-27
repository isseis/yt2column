# 実装計画書：LLM のプロバイダとモデルを 1 つの環境変数 `<プロバイダ>/<モデル ID>` で指定する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-27 |
| Review date | 2026-10-27 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的と原則

[要件定義書](01_requirements.md)（以下、要件書）の AC-01〜AC-30 を、[設計書](02_architecture.md) §3・§7・§8 に従って実装する。本書の用語（新しい変数、旧変数、組、段階、設定されている、プロバイダが決まる、プロバイダ固有の変数）は、設計書冒頭の定義に従う。

- 規則は `internal/config` の `loadLLM` の下に置き、本番のコードで変えるのは `internal/config/config.go` だけとする（設計書 §1.1・§3.7）。
- 旧変数の拒否、新しい変数の解析、`cmd/` のテストの環境、文書を同じ PR に入れ、旧変数で止まる規則と古い文書が並ぶ期間を作らない（設計書 §8）。
- テストは `LookupFunc` に渡す `map`、`yt-dlp` の仕掛け（tripwire）、`make` のスタブだけを使う。LLM の API・Webhook・`yt-dlp` は呼ばない（`CLAUDE.md`「Testing Strategy」）。
- Go のコメント・識別子・文字列は英語とし、受け入れ基準との対応は本書 §9 に記録する。ソースのコメントに `AC-NN` を書かない（`requirements_process.md` §4）。
- テストを加えたら、そのテストが対象とする仕組みを壊して失敗することを確認し、そのことをコミットメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。

### 1.2. 計画の際に決めたこと（2026-10-27）

- **実装を 2 PR に分ける（人間のレビュアーの選択）。** PR-1 は結合テストのモデルの変数の改名（設計書 §8 の 4）、PR-2 は `internal/config`・`cmd/` などのテスト・文書（設計書 §8 の 1〜3）とする。改名は振る舞いを変えないので、先に出して PR-2 のレビューの量を減らす。
- **設計書 §8 の 4 のうち、1 に依存する部分は PR-2 に入れる。** CLI の結合テストが使う、プロセスの環境変数を引き継がない環境（`newIntegrationRun`）は、本番の変数 `YT2COLUMN_MODEL` で DeepSeek のモデルを `run` に渡している（`cmd/yt2column/test_helpers_integration.go:164`）。この行を `YT2COLUMN_LLM=deepseek/<モデル>` に書き換えられるのは、`internal/config` が `YT2COLUMN_LLM` を読むようになってからである（設計書 §3.9 の 3 つ目）。そのため、この書き換えと、AC-29 の `static` の確認の範囲のうち `cmd/yt2column` の `*integration*` のファイル（設計書 §7.4）は、PR-2 の 3-1 で行う。PR-1 の確認の範囲は、`Makefile` と `internal/llm/deepseek/` の下とする。設計書 §8 の「4 は 1〜3 より前か、1〜3 と同じ PR でマージする」は、どちらの部分も満たす。PR の分け方は設計書が計画に委ねているので、設計書は変えない。
- **フェーズの順は、設計書 §8 の 4・1・2・3 の順にする。** PR-1 の対象を先頭のフェーズに置き、PR の作成ポイントがその後に来るようにするためである。対応は §3.1 に示す。設計書 §8 の内容と、1〜3 を同じ PR に入れる制約は変えない。
- **本物の DeepSeek API を呼ぶ結合テストは動かさない（人間のレビュアーの選択）。** 改名の後の名前の受け渡しは、`make` のスタブのテスト（`CheckChargedTarget`）と `TestSettingsFrom`（`internal/llm/deepseek/testutil/integration_settings_test.go`）が確かめる。CLI の結合テストの環境が両方の段階を DeepSeek に送ることは、通信なしの `TestIntegrationRunProviderSelection` が確かめる。課金を避けるためである（設計書 §7.4 が計画に委ねた点）。
- **AC-29 の目印のテスト（旧名の変数に目印の値を置き、それが使われないことを確かめるテスト。1-2）と `static` の確認は、`cmd/yt2column/makefile_test.go` に置く。** どちらも旧名 `YT2COLUMN_MODEL` を書く必要があるので、確認が走査する範囲の外に置く。`makefile_test.go` は、3 つのターゲットを動かす既存のテストを持ち、名前に `integration` を含まない。
- **PR-2 で削除するテストの扱い。** 削除するテストは、2-1 で除く関数（`modelWithoutProvider` など）と古い補助関数を参照するので、その関数と同じコミットで削除する。削除してよいことは、2-1 より前の基点の版で、対象のテストを含めた場合と `go test -skip` で外した場合のカバレッジを関数ごとに比べて確かめる。削除するテストが確かめていた規則（モデルだけの設定の拒否）は、要件書 F-003 の旧変数の拒否が置き換える。対応は 2-1 に記す。
- **`internal/config/test_helpers.go` に加える関数の名前を決める（設計書 §3.7 が計画に委ねた点）。** 名前の対応は §1.4 の表に置く。

### 1.3. 既存コード調査結果

調査の基点はコミット `c2ce520dd1ca340f387d106e64facb03aa4550a7`。この節の `file:line` はその版を指す。設計書が確かめたコミット `30dbae0` から基点までの間に、`docs/tasks/` の外のファイルの変更はない（`git diff --stat 30dbae0 c2ce520 -- ':!docs/tasks'` の出力が空であることを、本書の作成時に確かめた）。そのため、設計書の `file:line` も基点の版で有効である。

| 対象 | 現状と必要な変更 | 作業 |
|---|---|---|
| 設定の読み込み | `internal/config/config.go:405-434` の `loadLLM` が、`resolveProvider`（`:442`）・`loadModel`（`:645`）・`loadReview`（`:537`）・`modelWithoutProvider`（`:472`）・`loadProviderVars`（`:621`）を呼ぶ。設計書 §3.6 の表の仕組みを除き、§3.2〜§3.4 の `parseLLM`・段階の解決・旧変数の検査に置き換える。`loadProviderVars` は参照するフィールドだけを変える | 2-1 |
| 定数と型 | 旧変数の名前（`:32-35`）、`defaultProviderValue`（`:59`）、`reasonProvider`（`:77`）、`reasonModelWithoutProvider`（`:86-91`）、`LLMConfig` の 4 つのフィールド（`:128-132`）、アクセサのコメント（`:139-166`）、`Provider` の定数のコメント（`:20-23`）、受理する値のコメント（`:46`）が対象 | 2-1 |
| 呼び出し側 | `LoadServer`（`:282`）と `LoadFor`（`:370`）は `loadLLM` の呼び方を変えない。コメント（`:276-281`・`:362-369`）だけを書き換える。`internal/llm/provider` の `New`・`NewReview` はアクセサの値を `newClient` に渡すだけである（`internal/llm/provider/provider.go:33-42`） | 2-1 |
| `internal/config` のテスト | `config_test.go` の 19 のテスト関数、補助関数 `validEnv`・`validClaudeEnv`・`wantInvalidProvider`、変数 `wantModelOnly`（`:1421`）が、旧変数の定数か除く仕組みを使う。`validServerEnv`（`:970`）は `validEnv` から作る。`TestLoadHost`・`TestLoadHostHome`（`socketpath_test.go:154`・`:196`）は記録する `lookup`（`recordingLookup`。`:127`）で `HOME` だけを読むことを確かめている | 2-1、2-2、2-3 |
| テスト補助 | `internal/config/test_helpers.go:47-79` の 7 つの関数のうち、4 つが旧変数か除く仕組みを返す。まとまりのコメント（`:47-49`）と `DefaultClaudeModelForTest` のコメント（`:55-56`）も古い規則を述べる。呼び出し側は `cmd/yt2column/run_test.go:1997`・`:2019` と `cmd/yt2column/docs_test.go:260-262`・`:731`・`:780-782` だけ | 2-1、3-1 |
| CLI のテストの環境 | `newRunEnv`（`cmd/yt2column/run_test.go:119`）、`newChildSetup`（`signal_test.go:61-62`）、`newIntegrationRun`（`test_helpers_integration.go:163-164`）が旧変数で DeepSeek を選ぶ。旧変数を書き換える行は `run_test.go:521-537`・`:620`・`:1956`・`:1978-1979`・`:2007`・`:2274` | 3-1 |
| CLI の振る舞いのテスト | `TestResumeNeed`（`run_test.go:2601`）は 8 通りの組み合わせだけを確かめる。`buildWriter`（`cmd/yt2column/run.go:577-589`）がどの組み合わせでどのクライアントを作るかを、`resumeNeed`（`run.go:481-493`）と突き合わせるテストはない。`--slack` で `SLACK_WEBHOOK_URL` が未設定の行（`run_test.go:972-978`）は、設定の読み込みが成功したときに報告されることだけを確かめる。`--after` は `--artifacts` がないと使い方の誤りになる（`run.go:510-511`）が、設定の読み込み（`run.go:331`）は成果物の読み込みより前なので、成果物がなくても設定のエラーが報告される | 3-2 |
| 未知のプロバイダの拒否 | `TestNewUnknownProvider`（`internal/llm/provider/provider_test.go:229`）が、`newClient` が `ProviderUnset` を `errUnknownProvider` で拒否し、どのアダプタも作らないことを確かめている。再利用する | 3-2 |
| サーバとホスト | `newServerEnv`（`cmd/yt2column-server/test_helpers.go:126-127`）、設定のエラーの行（`run_test.go:66-71`）、`TestOutputsOmitSecrets`（`run_test.go:317-318`）、ホストの `fullEnv`（`cmd/yt2column-nmhost/main_test.go:123-133`）が旧変数を使う。ホストのテストの `invoke` は、`HOME` のほかの変数を読むと失敗する（`main_test.go:87-89`） | 3-3 |
| provider のテスト | `validEnv`・`claudeValidEnv`（`provider_test.go:89`・`:100`）、`TestNewClaudeDefaultEffort`（`:355`。旧変数は `:364-371`）、`TestNewPaddedModel`（`:543`）、`TestNewReviewUsesReviewModel`（`:559`。旧変数は `:562-592`）が旧変数を使う | 3-4 |
| 文書のガード | `TestLLMDefaultsDocsContract`（`cmd/yt2column/docs_test.go:258`）、`docsProfileEnv`（`:729`）、`productionProfileVars`（`:778`）、`checkProfileTable`（`:803`）、`TestProductionProfileDocsContract`（`:874`）が旧変数の名前で表と `export` の例を検査する。`checkExportBlock` は `export` のブロックの変数を読み取る。`TestClaudeDocsContract`（`:232`）は結合テストの変数の名前を README で検査する形の手本になる。DeepSeek の結合テストの変数には同じ形のガードがない | 1-2、3-1、4-2 |
| 結合テストの変数 | `Makefile:122-139`・`:166-181`・`:205-214` が `YT2COLUMN_MODEL` を使い、`deepseektestutil.ModelEnv`（`internal/llm/deepseek/testutil/integration.go:26`）がその名前を持つ。`CheckChargedTarget`（`internal/llm/deepseek/testutil/make.go:69-81`）は、3 つのターゲット（`internal/llm/deepseek/makefile_test.go:18`、`cmd/yt2column/makefile_test.go:42`・`:56`）で、未定義なら `deepseek-flash`、空はそのまま、値はそのまま渡ることを確かめている（`internal/maketestutil/make.go:223-268`）。名前は `ModelEnv` から取るので、定数を変えれば追従する。`maketestutil.RunTarget` は、許可した変数と `set` の変数だけを子の環境に渡す（`internal/maketestutil/make.go:40-43`・`:122-128`） | 1-1、1-2 |
| 変えないもの | `internal/maketestutil/make_test.go:12` は名前の形の検査の入力例として `YT2COLUMN_MODEL` を書くだけで、変数を読まない（設計書 §3.7）。`cmd/yt2column-server/docs_test.go` の文書のガードは、キャッシュと `yt-dlp` の変数の名前だけを検査する | - |

具象型による振る舞いの選択は、本タスクにない。`llmSelection` は `internal/config` の非公開の型であり、`parseLLM` と段階の解決だけが作る。ガードの対象にしない。

### 1.4. 名前の対応

本タスクで名前を変える、除く、加えるものを、この表にまとめる。本書のほかの節は、この表の名前を使う。

| 種類 | 旧 | 新 |
|---|---|---|
| 結合テストの変数（値） | `YT2COLUMN_MODEL` | `YT2COLUMN_TEST_DEEPSEEK_MODEL`（定数 `deepseektestutil.ModelEnv` の名前は変えない） |
| テスト補助（除く） | `DefaultProviderValueForTest`・`LLMProviderEnvForTest`・`ModelEnvForTest`・`ModelWithoutProviderErrorForTest` | - |
| テスト補助（加える） | - | `LLMEnvForTest`（`YT2COLUMN_LLM`）・`ReviewLLMEnvForTest`（`YT2COLUMN_REVIEW_LLM`）・`DefaultLLMValueForTest`（`defaultLLMValue`）・`RetiredLLMVarsForTest`（旧変数 4 つの名前、報告の順）・`RetiredLLMVarErrorForTest`（旧変数 1 つの拒否を設定のエラーとして出したときの文字列） |
| `internal/config` のテスト（名前を変える） | `TestLoadServerModelMissing` | `TestLoadServerLLMRejected` |
| `internal/config` のテスト（除く） | `TestLoadModelWithoutProvider`・`TestModelWithoutProvider` | - |
| `cmd/yt2column` のテスト（除く） | `TestRunModelWithoutProvider` | - |
| `internal/config` のテストの補助（除く） | 関数 `wantInvalidProvider`・変数 `wantModelOnly` | 理由を引数に取る拒否の補助（名前は実装で決める） |

設計書 §3.1 の非公開の名前（`llmEnv`・`reviewLLMEnv`・`llmSeparator`・`defaultLLMValue`・`reasonLLMForm`・`reasonLLMProvider`・`reasonRetiredLLM`・`reasonRetiredReviewLLM`・`retiredLLMVar`・`retiredLLMVars`・`llmSelection`・`parseLLM`）は、設計書のとおりとする。

### 1.5. 申し送りの扱い

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-07 は、設計書の付録 A がすべて記録している。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: 結合テストのモデルの変数の改名（設計書 §8 の 4。`newIntegrationRun` を除く）

#### ステップ 1-1: 変数の名前を変える

**変えるファイル**: `Makefile`、`internal/llm/deepseek/testutil/integration.go`

- [x] `Makefile` の `YT2COLUMN_MODEL` を、すべて `YT2COLUMN_TEST_DEEPSEEK_MODEL` に変える（変数の定義、3 つのターゲットの `export`、`printf` の `$$YT2COLUMN_MODEL`、コメント）。`?=` と `export ... :=` の形は変えない（設計書 §3.9）。
- [x] `deepseektestutil.ModelEnv` の値を `"YT2COLUMN_TEST_DEEPSEEK_MODEL"` に変える。
- [x] `ModelEnv` を使う既存のテスト（`TestRecordedEnv`・`TestModelEnv`・`TestSettingsFrom`・`TestCLIIntegrationSettings`・`TestCLISlackIntegrationSettings`・`internal/publisher/makefile_test.go`、3 つの `CheckChargedTarget`）が、変更なしで通ることを確かめる。

**完了の条件**: `go test -tags test ./internal/llm/deepseek/... ./internal/publisher` と `go test -tags test ./cmd/yt2column -run 'Make|IntegrationSettings'` が通る。

#### ステップ 1-2: 旧名を読まないことと文書を確かめる

**変えるファイル**: `cmd/yt2column/makefile_test.go`、`cmd/yt2column/docs_test.go`、`README.md`

- [x] **目印のテスト（AC-29）:** `cmd/yt2column/makefile_test.go` に、3 つのターゲットのそれぞれについて、`maketestutil.RunTarget` の `set` に `YT2COLUMN_MODEL=<目印>` だけを置き（`ModelEnv` は未定義）、記録する変数に `ModelEnv` を含めて `make` のスタブを動かすテストを加える。記録された `ModelEnv` が `deepseek-flash` であることを確かめる。壊し方は、`Makefile` の既定値を `$(or $(YT2COLUMN_MODEL),deepseek-flash)` にすることとする。このとき既存の `CheckChargedTarget` は通り（子の環境に `YT2COLUMN_MODEL` がない）、目印のテストだけが失敗することを確かめ、両方の結果をコミットメッセージに書く。
- [x] **`static` のガード（AC-29）:** 同じファイルに、`Makefile` と `internal/llm/deepseek/` の下の Go のファイルに、`YT2COLUMN_MODEL` という語（後ろに識別子の文字が続かないもの）が現れないことを確かめるガードテストを加える。走査の範囲ごとに、走査したファイルが 1 つ以上あることも確かめる。`cmd/yt2column` の `*integration*` のファイルは、3-1 で範囲に加える（§1.2）。壊し方は、`Makefile` のコメントに旧名を書き戻すこととする。
- [x] **文書のテスト（AC-30）:** `cmd/yt2column/docs_test.go` に、README が `deepseektestutil` の結合テストの変数（`ModelEnv`・`APIKeyEnv`・`DeepSeekOptInEnv`・`CLIOptInEnv`）の名前を含むことを、`TestClaudeDocsContract` と同じ形で確かめるテストを加える（設計書 §7.3）。壊し方は、README の名前を旧名に戻すこととする。
- [x] README の結合テストの説明（`README.md:1094`・`:1128`・`:1155` 付近）の `YT2COLUMN_MODEL` を `YT2COLUMN_TEST_DEEPSEEK_MODEL` に書き換える（AC-30）。Makefile のコメントは 1-1 で書き換えている。
- [x] README を変えたので `make ext-test` を動かす（`CLAUDE.md`「Development Notes」）。

**完了の条件**: グリーンゲート（`make test && make lint`）と `make ext-test` が通る。

### PR-1 作成ポイント: integration test model variable rename

**対象ステップ**: 1-1 / 1-2

**推奨タイトル**: `test(0045): DeepSeek の結合テストのモデルの変数を YT2COLUMN_TEST_DEEPSEEK_MODEL に改名する`

**レビュー観点**: Makefile の 3 つのターゲットで、未定義なら deepseek-flash、空はそのまま渡す振る舞いが保たれているか / 目印のテストが、旧名から値を取る Makefile を見分けられるか（コミットメッセージの壊し方の記録） / 旧名を走査するガードの範囲が Makefile と internal/llm/deepseek の下に限られ、CLI の結合テストの範囲は PR-2 で加えることになっているか / README の結合テストの説明が新しい名前で読めるか

**実装モデル要件**: standard

**判定理由**: 名前の変更と、既存の形に倣うテストの追加だけであり、どの条件にも当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した（[#528](https://github.com/isseis/yt2column/pull/528)）
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `internal/config` の変更とそのテスト（設計書 §8 の 1）

#### ステップ 2-1: 値の解析、段階の解決、旧変数の検査を実装し、不要になった仕組みとテストを除く

**変えるファイル**: `internal/config/config.go`、`internal/config/test_helpers.go`、`internal/config/config_test.go`

- [ ] 書き換えを始める前に、基点の版で、削除するテストを含めた場合と `-skip` で外した場合のカバレッジを関数ごとに比べる。外すのは、`internal/config` では `-skip '^(TestLoadModelWithoutProvider|TestModelWithoutProvider)$'`、`cmd/yt2column` では `-skip '^TestRunModelWithoutProvider$'` である。値が下がる関数が、2-1 で除く関数（設計書 §3.6 の表）だけであることを確かめ、結果をコミットメッセージに書く（`CLAUDE.md`「Deleting a test is a claim that must be checked」）。
- [ ] 設計書 §3.1 の定数と型を加え、`LLMConfig` の 4 つのフィールドを `generation`・`review` の 2 つの `llmSelection` に置き換える。アクセサの名前と型は保ち、コメントを §3.1 の契約の表に合わせる。
- [ ] `parseLLM` を設計書 §3.2 の表と契約のとおりに実装する（プロバイダが決まれば、エラーを返すときも組に入れる。`switch` の `default` は `reasonLLMProvider` の拒否）。
- [ ] 生成の組を決める関数（`YT2COLUMN_LLM` を読み、未設定なら `defaultLLMValue` を `parseLLM` に通す）を加え、`loadLLM` を設計書 §3.3 の表と §6.1 の流れのとおりに書き直す。生成の組を `cfg.generation` に入れるのは `Need.Generation` が真のときだけとする。
- [ ] `loadLLM` の最初で、`retiredLLMVars` の順に旧変数が設定されているかを `lookup` で確かめ、設定されていれば `invalidVar(name, reason)` をエラーの列にそれぞれ別の要素として加える（設計書 §3.4）。値は使わない。
- [ ] `loadProviderVars` が参照するフィールドを `cfg.generation.provider`・`cfg.review.provider` に変える（設計書 §3.5）。
- [ ] 設計書 §3.6 の表の仕組み（`settingOrigin`・`stageOrigins`・`modelWithoutProvider`・`reasonModelWithoutProvider`・`resolveProvider`・`parseProvider`・`reasonProvider`・`defaultModel`・`loadModel`・`loadModelOptional`・`loadReview`・`defaultProviderValue`・`providerEnv`・`modelEnv`・`reviewProviderEnv`・`reviewModelEnv`）を除く。
- [ ] `TestLoadModelWithoutProvider` を除く。確かめていた規則（モデルの変数だけを設定した実行を止めること）は、`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_MODEL` を設定した旧変数のテスト（2-3）が引き継ぐ。
- [ ] `TestModelWithoutProvider` を除く。確かめていた仕組み（`modelWithoutProvider`）はこのステップで除いている。
- [ ] コメントを書き換える: `Provider` の定数（`:20-23`）、受理する値の定数（`:46`）、既定値の定数のまとまり（`:52-63`）、`LoadServer`（`:276-281`。旧変数の検査は例外であること）、`LoadFor`（`:362-369`。同じ例外と、検証・推敲だけの実行で `YT2COLUMN_LLM` を読む条件）、`loadLLM`、`loadProviderVars`（`:612-620`）。
- [ ] `test_helpers.go` から §1.4 の除く 4 つの関数を除き、加える 5 つの関数を加える。`RetiredLLMVarErrorForTest` は旧変数でない名前に対して panic する（既存の `SocketPathTailForTest` と同じ扱い）。まとまりのコメント（`:47-49`）と `DefaultClaudeModelForTest` のコメント（`:55-56`）を新しい規則に合わせる。

**完了の条件**: `go build -tags test ./internal/config` が通る（`test_helpers.go` を同じタグで型検査する）。`internal/config` のテストは 2-2 の書き直しまで除いた定数を参照し、`cmd/` のテストは 3-1 まで古い補助関数を参照するので、この時点ではどちらもビルドできない。

#### ステップ 2-2: 既存のテストを新しい変数で書き直す

**変えるファイル**: `internal/config/config_test.go`

- [ ] `validEnv` を `YT2COLUMN_LLM=deepseek/deepseek-chat`、`validClaudeEnv` を `YT2COLUMN_LLM=claude/claude-opus-5-5` に書き換え、旧変数の定数を使うテスト（§1.3 の表の 19 の関数のうち、2-1 で削除した 2 つを除く 17）を新しい変数で書き直す。確かめる内容は保ち、古い規則だけに意味があった行（モデルの変数の未設定・空、`deepseek` でモデルが必須であること、検証・推敲のプロバイダだけを設定するとそのプロバイダの既定のモデルになること）は、新しい規則の行に置き換える。
- [ ] `TestLoadGenerationDefaults`・`TestLoadReviewDefaults` を、設計書 §3.3 の表の各行（AC-01・AC-07・AC-10・AC-11）を確かめる形に書き直す。
- [ ] `TestLoadFor` の部分テスト「no stage reads no LLM variable」を設計書 §1.4 のとおりに書き換える。不正な値は新しい変数で与え、読まないことを確かめる一覧は新しい変数 2 つとプロバイダ固有の変数にし、旧変数 4 つは読むことを確かめる。
- [ ] `TestLoadServerModelMissing` を `TestLoadServerLLMRejected` に改め、`YT2COLUMN_LLM` の空の値と `deepseek/` で `LoadServer` が `YT2COLUMN_LLM` を名指しして失敗することを確かめる。
- [ ] `TestLoadServerLLMDefaults`・`TestLoadProductionDefaults`・`TestLoadDefaultProviderVariables` を、旧変数を含まない環境で保つ（AC-21）。`TestLoadServerLLMDefaults` の「古い既定に頼った deepseek の設定」の行は、旧変数を設定した行として 2-3 の旧変数のテストに移す。
- [ ] `wantInvalidProvider`・`wantModelOnly` を除き、理由を引数に取る拒否の補助に置き換える。

**完了の条件**: `go test -tags test ./internal/config` が通る。

#### ステップ 2-3: 解析・段階・旧変数・プロバイダ固有の変数のテストを加える

**変えるファイル**: `internal/config/config_test.go`、`internal/config/socketpath_test.go`

- [ ] **値の解析:** 設計書 §3.2 の表の各行の例を、`YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` の両方で確かめるテストを加える。受理する値ではアクセサの組（`claude/vendor/model-x` のモデルは `vendor/model-x`）、拒否する値では `Name`・番兵・理由（AC-02〜AC-05・AC-08・AC-09）。`parseLLM` が、`claude/` ではエラーとともにプロバイダを返し、プロバイダの部分が不正な値では `ProviderUnset` を返すことも確かめる（§3.2 の契約）。
- [ ] **前後の空白:** `claude/claude-opus-5-5 `・`claude/ claude-opus-5-5`・UTF-8 として不正なモデル ID を、設定の読み込みが受理し、モデルがそのまま入ることを確かめる（AC-06 の前半）。
- [ ] **段階とアクセサ:** 記録する `lookup` を使い、`Review` だけが真で `YT2COLUMN_REVIEW_LLM` が設定されていれば `YT2COLUMN_LLM` を読まないこと（`YT2COLUMN_LLM` が不正な値でも成功すること）、未設定なら読むことを確かめる（AC-10）。`Need{Review: true}` で `Provider()`・`Model()` がゼロ値、`Need{Generation: true}` で検証・推敲のアクセサがゼロ値であることを確かめる（設計書 §3.1 の表）。
- [ ] **旧変数の表:** 旧変数 4 つの名前と、それぞれの理由の定数（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL` は `reasonRetiredLLM`、`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL` は `reasonRetiredReviewLLM`）を、テストの中に書いた表として持つ。この表を `retiredLLMVars` と比べ、旧変数のテストのケースもこの表から作る。`retiredLLMVars` から 1 つ消えても、テストが一緒に縮まないようにするためである（AC-14）。
- [ ] **旧変数:** 4 つの旧変数のそれぞれを、値ありと空の値で、`Need` の 4 通りと `LoadServer` に渡し、旧変数を名指しする `ErrInvalid` と、代わりの変数に応じた理由が返ることを確かめる（AC-12〜AC-14）。旧変数と `YT2COLUMN_LLM` の両方を設定しても失敗すること（AC-16。設計書 §6.2 の 8 行目）。
- [ ] **まとめて報告すること:** 1 回の読み込みで、2 つ以上の旧変数、不正な `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM`、プロバイダ固有の変数の拒否、共通の変数（`YT2COLUMN_CACHE_DIR` など）の拒否が、別々の要素としてすべて返ることを確かめる（AC-15・AC-20）。旧変数だけで DeepSeek を指定した設定で、`ANTHROPIC_API_KEY` の未設定も報告されること（設計書 §6.2 の 7 行目）。
- [ ] **値を含まないこと:** `TestLoadErrorsOmitValues` に、新しい変数 2 つと旧変数 4 つの値に目印を入れる行を加える（AC-05・AC-14）。
- [ ] **プロバイダ固有の変数:** 設計書 §6.2 の 4〜6 行目と、`YT2COLUMN_REVIEW_LLM` からプロバイダが決まらない場合を、どのプロバイダ固有の変数を読み、どれを拒否するかで確かめる（AC-19）。`TestLoadIgnoresOtherProviderVars` は 2-2 の書き直しで保つ（AC-18）。
- [ ] **ホスト:** `TestLoadHost`・`TestLoadHostHome` の環境に旧変数 4 つを加え、成功して `HOME` だけを読むことを確かめる（AC-17。設計書 §7.1）。
- [ ] 加えたテストのそれぞれについて、そのテストが対象とする仕組みを壊すと失敗することを確認し、コミットメッセージに書く（たとえば、旧変数の検査を `need` の分岐の中に移す、`retiredLLMVars` から 1 つ消す、`parseLLM` で前後の空白を取り除く、`claude/` で `ProviderUnset` を返す、継承するときに `YT2COLUMN_LLM` を読まない）。

**完了の条件**: `go test -tags test ./internal/config` が通る。

### フェーズ 3: `cmd/` と `internal/llm/provider` のテスト（設計書 §8 の 2。PR-1 から残った §8 の 4 の部分を含む）

#### ステップ 3-1: CLI のテストの環境と補助関数の参照を書き換える

**変えるファイル**: `cmd/yt2column/run_test.go`、`signal_test.go`、`test_helpers_integration.go`、`integration_env_test.go`、`docs_test.go`、`makefile_test.go`

- [ ] `newRunEnv`・`newChildSetup`・`newIntegrationRun` の環境を、`YT2COLUMN_LLM=deepseek/<モデル>` の 1 行にする。コメント（`test_helpers_integration.go:127`）も書き換える。
- [ ] `run_test.go` で旧変数を書き換える行（§1.3 の表）を新しい変数に書き換える。設定のエラーの行は、`YT2COLUMN_LLM` の空の値、不正なプロバイダ（値に目印を含める）、両方の組み合わせにし、空白のある行は `YT2COLUMN_LLM="deepseek/ fake-model "` にする。
- [ ] `TestRunDefaultLLMConfig` は、`YT2COLUMN_LLM` を除いて既定値を確かめる形に変える。確かめ方（`DefaultClaudeModelForTest` と比べる）は保つ。
- [ ] `TestRunResumeConfigScoping` は、`YT2COLUMN_LLM` と `DEEPSEEK_API_KEY` を除いて `--after review` が成功することを確かめる形に変える（設計書 §1.4）。
- [ ] `TestRunModelWithoutProvider` を除く（カバレッジの比較は 2-1 で済んでいる）。確かめていた規則は、3-2 の CLI の旧変数のテストが引き継ぐ。
- [ ] `TestIntegrationRunProviderSelection` は、環境の `YT2COLUMN_LLM` が `deepseek/` で始まり、残りが渡したモデルであることを確かめる形に変える。壊し方は、`newIntegrationRun` の環境から `YT2COLUMN_LLM` を外すこととする。
- [ ] `docs_test.go` の除いた補助関数の呼び出しを、新しい補助関数に置き換える。`TestLLMDefaultsDocsContract` の `YT2COLUMN_LLM` の行は `LLMEnvForTest`・`DefaultLLMValueForTest` から作り、`productionProfileVars` は `LLMEnvForTest`・`ClaudeEffortEnvForTest` の 2 つにする。文書がまだ古いので、これらのガードは 4-1 まで失敗する。
- [ ] 1-2 の `static` のガードの範囲に、`cmd/yt2column` の名前に `integration` を含む Go のファイルを加える（§1.2）。範囲ごとに 1 つ以上のファイルがあることの確認も、加えた範囲に適用する。あわせて、ガードの `retiredModelEnv` の値を `config.RetiredLLMVarsForTest`（2-1）から取る。旧変数の名前をテストに書き写さないためである。

**完了の条件**: `go vet -tags test ./cmd/yt2column` と `go vet -tags integration ./cmd/yt2column` が通る（`test_helpers_integration.go` は `test || integration` で作る）。`go test -tags test ./cmd/yt2column -skip 'DocsContract'` が通る。

#### ステップ 3-2: CLI の旧変数・空白・クライアントの構築・エラーの順序のテストを加える

**変えるファイル**: `cmd/yt2column/run_test.go`

- [ ] **旧変数（AC-12）:** 要件書 §3.1 の表の各行の実行（`--after` なし、`--no-review`、`--after generation`、`--after shorten`、`--after review`）に旧変数を 1 つ加えたとき、次の 3 点を確かめるテストを加える。終了コード `2` で止まること。`yt2column: configuration: ` に `RetiredLLMVarErrorForTest` の結果をつないだ行を出すこと。tripwire が動かず、LLM のクライアントも作らないこと。`--after` の行には `--artifacts` を付ける（成果物は要らない。§1.3）。旧変数 4 つのそれぞれを少なくとも 1 回ずつ使う。`-h` は旧変数があっても終了コード `0` で使い方を出すこと、引数の誤りは使い方の誤りとして報告され、旧変数の行が出ないことも確かめる。
- [ ] **エラーの順序（AC-20）:** 旧変数を設定し、`--slack` で `SLACK_WEBHOOK_URL` を未設定にした実行が、旧変数の行を出し、`configuration: SLACK_WEBHOOK_URL` の行を出さないことを確かめる行を加える。旧変数を設定し、`YT2COLUMN_LLM` のモデル ID の前後に空白を入れ、本物の `newLLMClient` を使う実行が、`build the LLM client` を出さないことを確かめる行も加える。
- [ ] **前後の空白（AC-06）:** `--after` なしで `--no-review` を付けた実行（設定の読み込みは `YT2COLUMN_REVIEW_LLM` も読む。要件書 AC-11）で、`YT2COLUMN_REVIEW_LLM` のモデル ID の前後に空白があっても成功することを確かめるテストを加える。検証・推敲のクライアントは、本物の `newReviewLLMClient` を使うか、作った回数が 0 であることを確かめる。
- [ ] **クライアントの構築（設計書 §1.2・§7.2）:** `TestResumeNeed` を、`--after` の 4 通り（なし・`generation`・`shorten`・`review`）と `--no-shorten`・`--no-review` のすべての組み合わせに広げる（引数の検査が拒否する組み合わせは除く）。同じ組み合わせで、作ったクライアントの数を数える偽の依存を渡して `buildWriter` を呼び、生成のクライアントを作るのは `resumeNeed` の `Generation` が真のときだけ、検証・推敲のクライアントを作るのは `Review` が真のときだけであることを確かめるテストを加える。壊し方は、`buildWriter` が `--after shorten` でも生成のクライアントを作るようにすることとする。`Provider()` が `ProviderUnset` のときに `provider.New` が何も作らないことは、`New` が `newClient` に渡すだけなので（§1.3）、既存の `TestNewUnknownProvider` が確かめる。
- [ ] 加えたテストのそれぞれについて、そのテストが対象とする仕組みを壊すと失敗することを確認し、コミットメッセージに書く（たとえば、`loadLLM` の旧変数の検査を `need` の分岐の中に移すと `--after review` の行が失敗する）。

**完了の条件**: `go test -tags test ./cmd/yt2column -skip 'DocsContract'` が通る。

#### ステップ 3-3: サーバとホストのテストを書き換え、旧変数のテストを加える

**変えるファイル**: `cmd/yt2column-server/test_helpers.go`、`cmd/yt2column-server/run_test.go`、`cmd/yt2column-nmhost/main_test.go`

- [ ] `newServerEnv` の環境を `YT2COLUMN_LLM=deepseek/<モデル>` にする。設定のエラーの行（`run_test.go:66-71`）は `YT2COLUMN_LLM` を空の値にし、`hint` を `configuration: YT2COLUMN_LLM` にする。`TestOutputsOmitSecrets` は `YT2COLUMN_REVIEW_LLM=claude/fake-review-model` にする。
- [ ] 同じ設定のエラーの表に、旧変数を 1 つ加えた行を加え、ソケットを作る前に終了コード `2` で止まり、旧変数を名指しする行を出すことを確かめる（AC-13）。壊し方は、`loadLLM` の旧変数の検査を外すこととする。
- [ ] ホストの `fullEnv` を新しい変数に書き換え、「サーバの設定を誤って読んでも成功する環境」という目的を保つ。
- [ ] `fullEnv` に旧変数 4 つを加えた環境で、中継・`install`・`uninstall` が `fullEnv` のときと同じ結果になることを確かめるテストを加える（AC-17。設計書 §7.2）。`invoke` の読んだ変数の検査（`main_test.go:87-89`）がこのテストにも当たる。

**完了の条件**: `go test -tags test ./cmd/yt2column-server ./cmd/yt2column-nmhost` が通る。

#### ステップ 3-4: provider のテストを書き換える

**変えるファイル**: `internal/llm/provider/provider_test.go`

- [ ] `validEnv`・`claudeValidEnv` を `YT2COLUMN_LLM` の 1 行にし、`TestNewClaudeDefaultEffort` の環境を `YT2COLUMN_REVIEW_LLM` で組み立て直す。
- [ ] `TestNewPaddedModel` と `TestNewReviewUsesReviewModel` の部分テストを、`YT2COLUMN_LLM=deepseek/ deepseek-flash` や `YT2COLUMN_REVIEW_LLM=deepseek/ <モデル>` のような値に変え、アダプタの構築が失敗して送信がないことを保つ（AC-06 の後半）。

**完了の条件**: `go test -tags test ./internal/llm/provider` が通る。

### フェーズ 4: 文書とガードテスト（設計書 §8 の 3）

#### ステップ 4-1: 文書を書き換える

**変えるファイル**: `README.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/dev/security.md`、`docs/dev/developer_guide/package_reference.md`

- [ ] 設計書 §3.8 の表の各行（README の Configuration・Recommended configuration・Using DeepSeek（移行の節）・トラブルシューティング・段階の説明・再開の説明、`project_overview.md` の 3 か所、`production_profile.md`、`security.md` §3・§4、`package_reference.md`）のとおりに書き換える。結合テストの説明は 1-2 で書き換えている。
- [ ] README の移行の節の、書き換えた後の設定の例は、4-2 のガードが読み取れる形（`export` の行だけの `sh` のコードブロック）で書く。書き換える前の例は、`sh` 以外の情報文字列のコードブロックか表で書く（`checkExportBlock` は節の中の `sh` のブロックを 1 つだけ受け付ける。`docs_test.go:685`）。
- [ ] README の見出しを変えた場合は、その節へのリンク（`README.md:718-723` など）も直す。
- [ ] 書き換えた説明が、2-1 の実装（`parseLLM` の表、段階の解決、旧変数の理由の文言）と一致することを、実装と突き合わせて確かめる。トラブルシューティングが設定のエラーの行を引用するなら、`RetiredLLMVarErrorForTest` の結果と一致させる。

**完了の条件**: 4-2 のガードテストが通る。

#### ステップ 4-2: 文書のガードテストを仕上げる

**変えるファイル**: `cmd/yt2column/docs_test.go`

- [ ] `TestLLMDefaultsDocsContract` を、README と `project_overview.md` の表に `YT2COLUMN_LLM` の行がちょうど 1 つあり、「未設定のとき」の列の最初のコードスパンが `DefaultLLMValueForTest` であること、`YT2COLUMN_REVIEW_LLM` の行がちょうど 1 つあることを検査する形に仕上げる（AC-22・AC-27。設計書 §7.3）。旧変数の 3 行の検査は除く。
- [ ] `TestProductionProfileDocsContract` のコメントの「three variables」を、3-1 で 2 つにした `productionProfileVars` に合わせる。
- [ ] `checkProfileTable` を、プロバイダの列とモデルの列の最初のコードスパンを `/` でつなぎ、「生成」「字数の調整」の行は `YT2COLUMN_LLM`、「検証・推敲」の行は `YT2COLUMN_REVIEW_LLM` に入れて `LoadServer` で読み込む形に変える（設計書 §1.2・§7.3）。
- [ ] README の移行の節の、書き換えた後の設定の例を `checkExportBlock` と同じ方法で読み取り、`LoadServer` で読み込んで、要件書 AC-24 が述べる組（`YT2COLUMN_LLM=deepseek/deepseek-flash` なら生成と検証・推敲が `deepseek` の `deepseek-flash`）になることを確かめるテストを加える（AC-24）。
- [ ] 変えた、または加えたガードのそれぞれについて、文書の値を壊すと失敗することを確かめ、コミットメッセージに書く（たとえば、README の `YT2COLUMN_LLM` の既定値を `claude/claude-sonnet-5-5` にする、`production_profile.md` の段階の表の検証・推敲のモデルを変える、`export` の例に旧変数を足す、移行の例のプロバイダを `Deepseek` にする）。
- [ ] README を変えたので `make ext-test` を動かす。

**完了の条件**: グリーンゲート（`make test && make lint`）と `make ext-test` が通る。§5 の横断検索を行い、結果を記録する。

### PR-2 作成ポイント: one LLM variable per stage

**対象ステップ**: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2

**推奨タイトル**: `feat(0045): LLM のプロバイダとモデルを YT2COLUMN_LLM・YT2COLUMN_REVIEW_LLM の 1 つの値で指定する`

**レビュー観点**: 旧変数の検査が Need によらず行われ、ホストでは行われないか / parseLLM と段階の解決が設計書 §3.2・§3.3 の表のとおりで、空の値や誤りを既定のプロバイダで補っていないか / 除いたテストの規則が旧変数のテストに引き継がれ、カバレッジの比較が記録されているか / クライアントの構築が、フラグのすべての組み合わせで resumeNeed に従うか（3-2） / 文書のガードが表と移行の例から値を作り直し、LoadServer で読み込んでいるか（4-2） / README の移行の節とトラブルシューティングが、旧変数で止まった利用者を正しい直し方へ導くか

**実装モデル要件**: frontier-required

**判定理由**: 2-1 は、旧変数を拒否する新しい規則を入れると同時に既存の拒否（モデルだけの設定）を外す移行であり、送り先の選択というセキュリティの境界を変える（`mkplan.md` step 8 の panel mode の条件に当たる）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序と前提

| 本書のフェーズ | 設計書 §8 の項目 | PR |
|---|---|---|
| フェーズ 1 | 4. 結合テストのモデルの変数の改名（`newIntegrationRun` の環境と、AC-29 の確認の範囲のうち `cmd/yt2column` の `*integration*` のファイルを除く） | PR-1 |
| フェーズ 2 | 1. `internal/config` の変更とそのテスト | PR-2 |
| フェーズ 3 | 2. `cmd/` と `internal/llm/provider` のテスト。4 の残り（3-1） | PR-2 |
| フェーズ 4 | 3. 文書とガードテスト | PR-2 |

- PR-1 を先にマージする。PR-2 のブランチは PR-1 のマージの後に `main` から作る。改名を先に入れるので、開発者の環境に残った本番用の `YT2COLUMN_MODEL` が結合テストのモデルに使われる期間は生じない（設計書 §8）。
- PR-2 の途中では、次のテストが通らない。
    - ビルドできない: 2-1 から 2-2 までの `internal/config`、2-1 から 3-1 までの `cmd/yt2column`。
    - 旧変数の環境のために失敗する: 2-1 から 3-3 までの `cmd/yt2column-server`、2-1 から 3-4 までの `internal/llm/provider`。
    - 文書が古いために失敗する: 3-1 から 4-1 までの文書のガード。

  レビューしやすいように、ステップごとに 1 つのコミットにする。各ステップの完了の条件は、その時点でビルドできるものだけを挙げている。グリーンゲートは PR-2 の作成ポイントで確かめる。
- PR-2 の説明には、移行の手順（README の移行の節）と、移行の後に動いているサーバを起動し直すことを書く（設計書 §8）。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 | 結合テストのモデルの変数を `YT2COLUMN_TEST_DEEPSEEK_MODEL` に改名し、旧名を読まないことのテストとガード、README の結合テストの説明 | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 | `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の解析と旧変数の拒否、全テストの環境の書き換えと新しいテスト、文書とガード | frontier-required |

### 3.3. マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（PR-1 のマージ） | 結合テストの変数の改名。本番の設定の変数は変わらない |
| M2（PR-2 のマージ） | 新しい変数による指定、旧変数の拒否、文書の更新。本タスクの完了 |

## 4. テスト戦略 (Test Strategy)

### 4.1. 検証の分担

| 層 | 確かめること | ステップ |
|---|---|---|
| `internal/config` のユニットテスト | 値の解析、段階の解決と継承、アクセサの契約、旧変数の表と拒否（`Need` の 4 通りと `LoadServer`）、まとめて報告すること、プロバイダ固有の変数、値を含まないこと、ホストの読み込み | 2-2、2-3 |
| `cmd/yt2column` の `run` のテスト | 旧変数で止まり、`yt-dlp` もクライアントも動かないこと、`-h` と使い方の誤りの順序、設定の後に報告するエラーが出ないこと、`--no-review` の空白、クライアントの構築が `Need` に従うこと | 3-1、3-2 |
| `cmd/yt2column-server`・`cmd/yt2column-nmhost` | サーバが待ち受ける前に止まること、ホストが旧変数で止まらないこと | 3-3 |
| `internal/llm/provider` | 前後に空白があるモデル ID でアダプタの構築が失敗し、送信がないこと | 3-4 |
| 文書のガード | 既定値の行、`export` の例、段階の表、移行の例を、コードの値と `LoadServer` で突き合わせる。結合テストの変数の名前が README にあること | 1-2、4-2 |
| `make` のスタブ | 3 つのターゲットのモデルの変数の受け渡し、旧名を読まないこと | 1-1、1-2 |

本物の LLM の API・Webhook・`yt-dlp` を呼ぶテストは動かさない（§1.2）。後方互換は求めない（要件書 §4.4）。旧変数で止まることは、互換が失われることを確かめるテストとして 2-3・3-2・3-3 に含める。

### 4.2. 実行範囲と外部の前提

- テストは `-tags test` で作る。`cmd/yt2column/test_helpers_integration.go` は `test || integration` で作るので、3-1 の完了の条件で `-tags integration` でも型を確かめる。
- `make` のスタブのテストは、既存の `maketestutil.RunTarget` を使う。新しい外部の道具は入れない。`go test -skip` は Go 1.20 から使える。
- README を変える PR（PR-1・PR-2）は `make ext-test` を動かす（拡張の node のテストが README の表を読む。`CLAUDE.md`「Development Notes」）。

## 5. 横断検索 (Cross-Search Checklist)

`make lint` と `make test` が見つけられないものだけを挙げる。PR-2 の 4-2 の後に行い、結果を PR-2 の説明に書く。

- [ ] 旧変数の名前（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`）が `docs/tasks/` の外で現れる箇所を検索する。残ってよいのは、`retiredLLMVars` の表、旧変数を設定するテスト（`internal/config`・`cmd/yt2column`・`cmd/yt2column-server`・`cmd/yt2column-nmhost`）、README の移行の節とトラブルシューティング、`docs/dev/project_overview.md` の拒否の説明、`internal/maketestutil/make_test.go:12`、1-2 のガードテストと目印のテストだけである。Go のコメントに残っていないことも、この検索で確かめる。
- [ ] 除いた仕組みの名前（`settingOrigin`・`modelWithoutProvider`・`resolveProvider`・`parseProvider`・`defaultModel`・`loadModelOptional`・`loadReview`）が、`internal/config` のコメントと `docs/dev/developer_guide/package_reference.md` に残っていないことを確かめる。`internal/artifact/store.go` の `loadReview` は別の関数なので対象外である。
- [ ] 文書の用語と文章の整合（旧変数を現行の設定として説明しないこと）は、PR のレビューで読んで確かめる（§9 の `manual`）。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 旧変数を設定した開発者の環境で、PR-2 のマージ後に CLI とサーバが止まる | 起動できない | 意図した振る舞いである（F-003）。PR-2 の説明に移行の手順とサーバの起動し直しを書く（§3.1） |
| PR-2 が大きく、レビューで見落とす | 規則の誤り | 設計書 §8 が 1〜3 を分けられない理由を述べている。改名を PR-1 に分けた。レビュー観点を PR-2 の作成ポイントに挙げた |
| 既存のテストの書き直しで、確かめていた内容が落ちる | 回帰を見逃す | テストを除く前に基点の版でカバレッジを比べる（2-1）。除くテストの規則を引き継ぐテストを示す（2-1・3-1）。書き直すテストは確かめる内容を保つ（2-2） |
| 生成の組を `Need.Generation` が偽のときに空にするため、将来 `resumeNeed` と `buildWriter` の規則がずれるおそれがある | 生成のクライアントの構築が失敗する | 3-2 で全組み合わせのテストを加える。ずれても `provider.New` が `ProviderUnset` を拒否し、何も送らない（設計書 §1.2） |
| タスク 0044（設計書は承認済み）も `LoadServer`（`SLACK_WEBHOOK_URL` を任意にする）、`cmd/yt2column-server/run_test.go` の設定のエラーの行、`project_overview.md` を変える | 後からマージする側が衝突する | 後からマージする側が `main` を取り込み、`LoadServer` のコメントとその行（`configLines` を含む）を、両方の変更に合わせて作り直す |
| 本物の API で結合テストを動かさない | 改名後の実行環境の誤りを見逃す | 名前の受け渡しはスタブのテスト、設定の選択は通信なしのテストが確かめる（§1.2） |

## 7. 実装チェックリスト (Implementation Checklist)

フェーズごと:

- [x] フェーズ 1 完了（1-1 / 1-2）
- [ ] フェーズ 2 完了（2-1 / 2-2 / 2-3）
- [ ] フェーズ 3 完了（3-1 / 3-2 / 3-3 / 3-4）
- [ ] フェーズ 4 完了（4-1 / 4-2）、§5 の横断検索の結果を記録した

PR ごと:

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2）

## 8. 成功基準 (Success Criteria)

- 機能: 要件書の AC-01〜AC-30 のすべてが、§9 の検証で確かめられている。
- 品質: グリーンゲートと `make ext-test` が通る。除いたテストについて、関数ごとのカバレッジの比較が記録されている。
- セキュリティ: 空の値、構文の誤り、旧変数が残っていることのどれでも、LLM の API に何も送らずに止まることがテストで確かめられている（要件書 §4.2）。
- 文書: 設計書 §3.8 の文書がすべて更新され、§5 の横断検索の結果が記録されている。

## 9. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストの名前のうち、既存のものは名前で示す。新しく加えるテストは「（新設）」と付け、名前は実装で決めて、この表に書き戻す。

| AC | 種別 | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test | `internal/config/config_test.go::TestLoadGenerationDefaults`、`cmd/yt2column/run_test.go::TestRunDefaultLLMConfig` | 2-1 |
| AC-02 | test | `internal/config/config_test.go` の値の解析のテスト（新設、2-3） | 2-1 |
| AC-03 | test | 同上（`claude/vendor/model-x`） | 2-1 |
| AC-04 | test | 同上（空の値）、`internal/config/config_test.go::TestLoadServerLLMRejected`、`cmd/yt2column/run_test.go` の設定のエラーの行（3-1） | 2-1 |
| AC-05 | test | 同上（構文の誤り）、`internal/config/config_test.go::TestLoadErrorsOmitValues`、`cmd/yt2column/run_test.go` の設定のエラーの行（3-1） | 2-1 |
| AC-06 | test | `internal/config/config_test.go` の前後の空白のテスト（新設、2-3）、`internal/llm/provider/provider_test.go::TestNewPaddedModel`・`TestNewReviewUsesReviewModel`、`cmd/yt2column/run_test.go` の `--no-review` の空白のテスト（新設、3-2）と空白の行（3-1） | 2-1 |
| AC-07 | test | `internal/config/config_test.go::TestLoadReviewDefaults` | 2-1 |
| AC-08 | test | `internal/config/config_test.go` の値の解析のテスト（新設、2-3）、`internal/llm/provider/provider_test.go::TestNewReviewUsesReviewModel` | 2-1 |
| AC-09 | test | `internal/config/config_test.go` の値の解析のテスト（新設、2-3。`YT2COLUMN_REVIEW_LLM` の行） | 2-1 |
| AC-10 | test | `internal/config/config_test.go::TestLoadReviewDefaults`、段階とアクセサのテスト（新設、2-3。記録する `lookup`） | 2-1 |
| AC-11 | test | `internal/config/config_test.go::TestLoadFor`（「no stage reads no LLM variable」）、`cmd/yt2column/run_test.go::TestResumeNeed`（`--after` なしで `--no-review` でも検証・推敲の設定を検査すること）・`TestRunResumeConfigScoping` | 2-1 |
| AC-12 | test | `internal/config/config_test.go` の旧変数のテスト（新設、2-3）、`cmd/yt2column/run_test.go` の旧変数のテスト（新設、3-2） | 2-1 |
| AC-13 | test | `internal/config/config_test.go` の旧変数のテスト（新設、2-3。`LoadServer`）、`cmd/yt2column-server/run_test.go` の設定のエラーの表の旧変数の行（新設、3-3） | 2-1 |
| AC-14 | test | `internal/config/config_test.go` の旧変数の表と旧変数のテスト（新設、2-3）、`TestLoadErrorsOmitValues` | 2-1 |
| AC-15 | test | `internal/config/config_test.go` のまとめて報告することのテスト（新設、2-3） | 2-1 |
| AC-16 | test | `internal/config/config_test.go` の旧変数のテスト（新設、2-3。旧変数と `YT2COLUMN_LLM` の両方） | 2-1 |
| AC-17 | test | `internal/config/socketpath_test.go::TestLoadHost`・`TestLoadHostHome`、`cmd/yt2column-nmhost/main_test.go` の旧変数のテスト（新設、3-3） | 2-1（旧変数の検査は `loadLLM` だけにあり、`LoadHost`・`LoadHostHome` はそれを呼ばない） |
| AC-18 | test | `internal/config/config_test.go::TestLoadIgnoresOtherProviderVars` | 2-1 |
| AC-19 | test | `internal/config/config_test.go` のプロバイダ固有の変数のテスト（新設、2-3） | 2-1 |
| AC-20 | test | `internal/config/config_test.go` のまとめて報告することのテスト（新設、2-3）、`cmd/yt2column/run_test.go` のエラーの順序の行（新設、3-2） | 2-1 |
| AC-21 | test | `internal/config/config_test.go::TestLoadProductionDefaults`・`TestLoadServerLLMDefaults` | 2-1 |
| AC-22 | test、manual | test: `cmd/yt2column/docs_test.go::TestLLMDefaultsDocsContract`・`TestProductionProfileDocsContract`。manual: 旧変数を現行の設定として載せないこと、例の変数の数を述べる文（「次の 3 つ」「These three values」など）が例に合うことを PR-2 のレビューで読む | 4-1、4-2 |
| AC-23 | manual | 値の構文、既定値と継承、空の値と旧変数の拒否、再開の記述の例外、「前提・制約」の例外の範囲を、PR-2 のレビューで読む | 4-1 |
| AC-24 | test、manual | test: `cmd/yt2column/docs_test.go` の移行の例のテスト（新設、4-2）。manual: 移行の説明の文章を PR-2 のレビューで読む | 4-1、4-2 |
| AC-25 | manual | README のトラブルシューティングを PR-2 のレビューで読む | 4-1 |
| AC-26 | manual | 5 つの文書が旧変数を現行の設定として説明しないことを PR-2 のレビューで読む（§5 の横断検索の結果を手がかりにする） | 4-1 |
| AC-27 | test | `cmd/yt2column/docs_test.go::TestLLMDefaultsDocsContract`・`TestProductionProfileDocsContract`（`checkProfileTable` を含む） | 3-1、4-2 |
| AC-28 | test | `internal/llm/deepseek/makefile_test.go`・`cmd/yt2column/makefile_test.go::TestMakeTestIntegrationCLI`・`TestMakeTestIntegrationCLISlack`（`CheckChargedTarget` の `undefined_uses_default`・`empty_is_kept`・`value_is_kept`）、`internal/llm/deepseek/testutil/integration_settings_test.go::TestSettingsFrom`（空の値でテストが失敗すること） | 1-1 |
| AC-29 | test、static | test: `cmd/yt2column/makefile_test.go::TestMakeTargetsDoNotReadRetiredModelEnv`（1-2）。static: `cmd/yt2column/makefile_test.go::TestRetiredModelEnvIsGone`（1-2。3-1 で範囲を広げる） | 1-1、3-1 |
| AC-30 | test、manual | test: `cmd/yt2column/docs_test.go::TestDeepSeekDocsContract`（1-2）。manual: README と Makefile の結合テストの説明を PR-1 のレビューで読む | 1-1、1-2 |

## 10. 次のステップ (Next Steps)

- 本書の承認の後、`/runplan 0045` で PR-1 を実装する。
- PR-2 のマージの後、`/runpr` の案内に従い、タスクの issue #512 を閉じる。
- 実装の途中で、新設したテストの名前を §9 の表に書き戻す。
