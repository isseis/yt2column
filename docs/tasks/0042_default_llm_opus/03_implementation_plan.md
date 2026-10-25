# 実装計画書：既定の LLM を Claude Opus 5.5 にする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-25 |
| Review date | 2026-10-25 |
| Reviewer | isseis |
| Comments | 2026-10-25、PR-2 の実装で編集上の訂正を 1 つ行った。3-2 の「`TestNewReviewUsesReviewModel` に Claude の検証・推敲のモデルの検査を加える」は、本書の作成の後に 0041 の実装（#498）が加えた `TestNewClaudeDefaultEffort` の `both stages claude review` の行が同じことを確かめている（生成と異なる検証・推敲のモデルを、`NewReview` と同じ段階の選び方で Claude の構築に渡し、リクエストのモデル名を確かめる）。重複を避けて部分テストを加えず、§1.3・3-2・§8 の AC-08・AC-10・AC-22 の行の参照をこのテストに改めた。設計書 3.5・7.1 も同じく改めた。何を作るかは変わらない。 |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的と原則

[要件定義書](01_requirements.md) の AC-01〜AC-29 を、[設計書](02_architecture.md) §3・§7・§8 に従って実装する。以下の「段階」「出どころ」「モデルだけの設定の拒否」は、設計書冒頭の定義に従う。段階は、本書では設定を解決する生成と検証・推敲の単位を指す。

- 既定と拒否の判定は `internal/config` に置く。モデル名の中身でプロバイダを推測しない。
- 既定の変更、モデルだけの設定の拒否、利用者への案内を同じ PR に入れる。途中のフェーズの状態を `main` に入れない。
- テストでは偽の環境、起動を検出する既存の偽コマンド（tripwire）、ループバックの受信先を使う。実 API・実 Webhook を呼ぶ統合テストは別途人間の承認を得て実行する。
- Go のコメント・識別子・文字列は英語とし、受け入れ基準との対応は本書に記録する。

### 1.2. 計画の際に決めたこと（2026-10-25）

- **実装を 2 PR に分ける（人間のレビュアーの選択）。** PR-1 はフェーズ 1 のテスト環境の修正、PR-2 はフェーズ 2〜4 とする。振る舞いを変えない準備を先にレビューし、送り先の変更と拒否・案内は一緒にレビューするためである。
- **既定のモデルの定数は設計どおり `internal/config` に置く。** タスク 0041 の承認済み設計書 §1.2 も、effort の既定を同じパッケージに置くと決めている。0042 の設計書 §1.2 が留保した配置の調整は不要である。
- **AC-24 を通信なしのテストで CI でも確かめる（人間のレビュアーの選択）。** 共通の `newIntegrationRun` を `integration` だけのファイルから `test || integration` のファイルへ移し、`test` タグのテストがそれを呼ぶ。組み立てた環境を設定ローダーに渡し、生成と検証・推敲が DeepSeek を選ぶことを確かめる。`integration` だけのファイルに置くと `make test` でも CI でも走らず、マージ後にプロバイダの明示が外れても検出できないからである（設計書 §3.5・§7.4）。
- **#171 が要るのは PR-2 だけとする（人間のレビュアーの選択）。** PR-1 は振る舞いを変えず、#171 が変える `internal/config`・README・`project_overview.md` に触れないので、#171 を待たずにマージする。PR-2 のうち #171 が要るのは、AC-22 のテスト、4-1 の文書、既定を変えた後の利用者の振る舞いである（要件書 §5、設計書 §8）。

### 1.3. 既存コード調査結果

調査の基点はコミット `68f4cb99e4201c960230ec92a9d6aa7a3d8c3744`。この節の `file:line` はその版を指す。新設するテスト名には §4 で「新設」と付ける。実装を始めるときに、先行タスクの変更を反映して再確認する。

| 対象 | 現状と必要な変更 | 作業 |
|---|---|---|
| プロバイダの解決 | `internal/config/config.go:382-450` の `loadProvider`・`loadProviderOptional`・`parseProvider` に未設定時の処理がある。`loadLLM`（`:373`）と `loadReview`（`:482`）の呼び出しを共通の解決にまとめる | 2-1、3-1 |
| モデルの解決と受け継ぎ | 同 `:417-429` の `loadModelOptional`、`:461-514` の `loadReview`、`:545-553` の `loadModel` が設定する。`loadReview` の受け継ぎは `:477`・`:485`・`:508`。値と出どころを一緒に扱い、`LLMConfig` のフィールドは増やさない | 2-1、3-1 |
| 呼び出し側 | `LoadServer`（同 `:260`）と `LoadFor`（`:347`）が `loadLLM` を呼ぶ。`internal/llm/provider/provider.go:29-39` の `New`・`NewReview` はアクセサの値を渡す。設定の失敗時にゼロ値を返す既存の境界を保つ（`config.go:273-275`・`:355-357`） | 2-1、3-1、3-2 |
| 古い既定を使うテスト | `internal/config/config_test.go:108` の `TestLoadDefaults`、`:150` の欠落の行、`:1109`・`:1166` の `TestLoadFor` の部分テストが対象。残す検査と削る行を §2 に分ける | 2-2 |
| DeepSeek を前提にした環境 | `cmd/yt2column/run_test.go:119` の `newRunEnv`、`signal_test.go:46` の `newChildSetup`、`integration_test.go:112` の `newIntegrationRun`、`cmd/yt2column-server/test_helpers.go:121` の `newServerEnv` が対象。`integration_slack_test.go:40` は共通の `newIntegrationRun` を使う | 1-1、1-2 |
| 使い回す検査 | `internal/config/config_test.go:380` の `TestLoadIgnoresOtherProviderVars`、`:666` の `TestLoadErrorsOmitValues`、`:1015` の `TestLoadFor` を拡張・再利用する。`:578` の `assertVarError` は最初の `*VarError` を調べるため、複数の拒否の件数には全要素の検査が必要 | 2-2、3-1 |
| API への受け渡し | `internal/llm/provider/provider_test.go:257` の `TestNewClaudeSendsConfiguredRequest` と `:314` の `claudeRequestFor` が生成のモデル・effort のリクエストを検査する。Claude の検証・推敲のモデルの受け渡しは、0041 が加えた `TestNewClaudeDefaultEffort` が確かめている。字数の調整が生成のクライアントを使う検査は `internal/writer/steps_test.go:43` の `TestWriteStepOrder` を再利用する | 3-2 |
| 文書の値 | `cmd/yt2column/docs_test.go:184-280` に `readDoc`・`findTable`・`tableRow` などがある。`internal/config/test_helpers.go` の既存の `test` タグ付きファイルに既定を返すアクセサを加え、値の比較に使う | 4-1 |
| 文書の説明 | `README.md:718-748`、`:626` 付近のサーバの案内、`docs/dev/project_overview.md:12`・`:67`・`:109-126`、`docs/dev/developer_guide/package_reference.md:27` が変更対象。履歴である過去のタスク文書・評価記録は書き換えない | 4-1 |

変更箇所の検索は `loadProvider`・`loadProviderOptional`・`parseProvider`・`loadModel`・`loadModelOptional`・`loadReview` の識別子、モデルとプロバイダの環境変数名、既定を述べる日本語・英語の表現で行った。同名の `internal/artifact` の `loadReview` は対象外である。

`LLMConfig` への値の書き込みは上表の設定ローダー内を調べた（`config.go:390`・`:402`・`:409`・`:425`・`:474-508`・`:550`）。本タスクは具象型を識別して処理を免除する設計を加えない。新しい出どころの型は設計書 §3.1 の非公開の enum とし、モデルだけの設定の判定をその値で行う。

統合テストのモデルの既定は `Makefile:128`、Claude 統合テストのモデルは `Makefile:150` で別に与える。これらのテスト用の既定は変更しない。`cmd/yt2column/makefile_test.go` の環境は Make の検査用なので変更しない（設計書 §3.5）。

### 1.4. 名称の対応と申し送り

| 現在の名称 | 計画する名称・扱い |
|---|---|
| `loadProvider`・`loadProviderOptional` | 共通の `resolveProvider` にまとめ、両方の旧関数を除く |
| `parseProvider(name, value, present)` | `parseProvider(name, value)`。存在する値の検査に限定する |
| `loadModel`・`loadModelOptional`・`loadReview` | 名前は維持し、設計書 §3.1 の出どころの受け渡しを加える |

設計書 §3.5 の名称対応もこの表と一致している。`implementation_handoff.md` は存在しない。`design_handoff.md` の H-01〜H-06 は設計書付録 A に対応があり、本計画では H-01 を 1-1・1-2・2-2、H-02 を 2-1・4-1、H-03 を 2-1・3-1、H-04 を 3-1、H-05 を 3-2・§3.1、H-06 を 4-1 で扱う。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: テストの環境で DeepSeek を明示する

#### ステップ 1-1: 共通のテスト環境を修正する

**変更ファイル:** `cmd/yt2column/run_test.go`・`signal_test.go`・`integration_test.go`、`cmd/yt2column-server/test_helpers.go`。

- [x] §1.3 の 4 つの環境に `YT2COLUMN_LLM_PROVIDER=deepseek` を加える。Slack の統合テストにも同じ環境が渡ることを確認する。
- [x] 設計書 §7.2 に従い、既存の 2 か所の既定を一時的に Claude に変えてテストを実行し、失敗する部分テストが設計書 §1.4 の 4 件に含まれるか確認する。漏れは同じフェーズで直し、一時的な変更は戻す。結果をコミットメッセージに記録する。この切り替えはプロバイダの既定だけを模すので、モデルの既定やモデルだけの設定の拒否で初めて壊れるテストは見つからない。`assertVarError` は最初の `*VarError` だけを調べるため、`review only requires a model`・`review only rejects an empty inherited model` は切り替えだけでは失敗しないことがある。残りは 3-1 の確認で洗い出す。

**完了条件:** 一時的な変更を戻した状態で `make test`・`make lint` が通り、本番の既定はまだ変わっていない。

#### ステップ 1-2: 統合テストの設定を通信なしで確認する

**変更ファイル:** `cmd/yt2column/integration_test.go`・`test_helpers_integration.go`、`cmd/yt2column/integration_env_test.go`（新設）。

- [x] `newIntegrationRun` と、それだけが使う `integrationRun`・`videoCacheEntries`・`integrationVideoID`・`integrationTestdata` を、`integration_test.go` から `test || integration` の `test_helpers_integration.go` へ移す。統合テストだけが使う残り（`checkRun`・`integrationVideoURL` など）は移さない。移したものを両方のビルドが使うことを `make lint`（`test,integration` タグ）と CI の `go vet -tags integration ./...` で確かめる。
- [x] `integration_env_test.go`（`//go:build test`）に `TestIntegrationRunProviderSelection` を新設する。偽のテストキーとモデルで `newIntegrationRun` を呼び、共通の環境に生成のプロバイダが明示されていることを確認してから、変数を読んだ記録を残す `LookupFunc` 越しに設定ローダーに渡す。生成と検証・推敲の両段階が DeepSeek と指定モデルを選び、Anthropic の変数を読まないことを検査する。LLM のクライアントは作らず、Webhook の投稿もしない。このテストは `make test`・CI の `test` ジョブ・`doc-guard` ジョブで走る。
- [x] CLI と Slack の統合テストが同じ `newIntegrationRun` の環境を使い、関数の外で LLM の変数を組み立て直していないことをコードを読んで確認する。
- [x] `newIntegrationRun` の明示したプロバイダを外して、このテストの環境の検査が失敗することを確認する。一時的な変更を戻し、結果を記録する。

**完了条件:** `make test` で `TestIntegrationRunProviderSelection` が走って通り、`make lint` と `go vet -tags integration ./...` が通る。実 API を呼ぶ 2 つの統合テストは実行していない。

### PR-1 作成ポイント: explicit DeepSeek test environments

**対象ステップ**: 1-1 / 1-2

**推奨タイトル**: `test(0042): テスト環境で DeepSeek を明示する`

**レビュー観点**: 共通のテスト環境の修正漏れ / 統合テストの送り先 / 本番の既定が変わらないこと

**実装モデル要件**: standard

**判定理由**: 1-1・1-2 は既存の環境の明示と通信なしの設定の検査であり、上位モデルを必要とする条件に該当しない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/499
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 既定のプロバイダとモデル

#### ステップ 2-1: 設定の解決を共通化し、既定を加える

**変更ファイル:** `internal/config/config.go`。

- [x] 設計書 §3.1〜§3.3 に従い、`defaultProviderValue`・`defaultClaudeModel` と `defaultModel` を加える。既定のモデルを選ぶ `switch` の `default` は既定なしとする。
- [x] §1.4 の表に従いプロバイダの解決をまとめ、全呼び出し側を更新する。未設定と空の区別を維持する。
- [x] 生成の段階、検証・推敲だけを実行する場合の受け継ぎ、検証・推敲のプロバイダだけを指定する場合の各経路で、同じモデルの既定を使う。空・不正なプロバイダや空のモデルに既定を当てはめない。
- [x] 出どころを値と一緒に返すための型と引数を設計書 §3.1 に合わせて準備する。コメントも新しい既定と受け継ぎに合わせる（§4.3）。

**完了条件:** 2-2 の既定と既存の拒否のテストが通る。モデルだけの設定の拒否を含む行は 3-1 で検査する。このフェーズだけでは PR を開かず、3-1 の拒否を続けて加える。

#### ステップ 2-2: 既定と拒否の既存テストを更新する

**変更ファイル:** `internal/config/config_test.go`。

- [x] 設計書 §3.2・§3.3 のうち、既定と既存の拒否を扱う行を、設計書 §7.1 の実行の種類で検査する `TestLoadGenerationDefaults`・`TestLoadReviewDefaults` を新設する。既定以外の生成モデルと、検証・推敲の既定のモデルも区別する。モデルだけの設定の拒否を含む行は 3-1 の `TestLoadModelWithoutProvider` で検査する。既存の検査と重複する行は同じテストにまとめる。
- [x] `TestLoadDefaults` に DeepSeek の明示を加え、Slack・`yt-dlp`・キャッシュディレクトリの既存の検査を残す。明示した値をそのまま確かめるだけになる `Provider()` の検査は削る（`CLAUDE.md`「Do not assert ... a struct field holds what was just assigned to it」）。
- [x] `TestLoadMissing` の `provider unset and API key unset` の行だけを削る。DeepSeek のキーがない場合の残る行を確認し、削除の直前・直後の関数ごとのカバレッジを比較して結果を記録する。
- [x] `TestLoadFor` の `review only requires a model` に DeepSeek の明示を加える。
- [x] `TestLoadFor` の `review only rejects an empty inherited model` に DeepSeek の明示を加える。
- [x] `TestLoadFor` の `no stage reads no LLM variable` を拡張し、全 LLM 変数の参照がなく、モデル・プロバイダのアクセサに既定が入らないことを検査する。

**完了条件:** 既定を当てはめる場合・当てはめない場合と、実行しない段階の扱いが §4 のテストで確認できる。

### フェーズ 3: モデルだけの設定の拒否

#### ステップ 3-1: 出どころを受け継いで一度だけ拒否する

**変更ファイル:** `internal/config/config.go`・`config_test.go`。

- [x] 設計書 §3.4・§4.1 に従い `reasonModelWithoutProvider` と `modelWithoutProvider` を加え、必要な段階の解決後に一度だけ判定する。拒否後もプロバイダ固有の変数を検査し、値を含まない固定の理由で報告する。理由の文言は設計書 §1.2・§4.2 を使う。
- [x] `TestLoadModelWithoutProvider`・`TestModelWithoutProvider` を新設する。複数の拒否は全要素を調べ、名前・番兵・理由・件数を検査する。空の変数の拒否と区別し、出どころが「なし」の場合も拒否する。
- [x] `TestLoadDefaultProviderVariables` を新設し、参照した変数を記録して、既定の Claude の変数だけを検査することを確認する。`TestLoadErrorsOmitValues` にモデルだけの設定と目印を加える。既定の Claude の実行で DeepSeek の変数を読まないことは `TestLoadDefaultProviderVariables` で検査する。既存の `TestLoadIgnoresOtherProviderVars` は、DeepSeek を明示した実行で Anthropic の変数を読まないこと（AC-23）の検査として再利用する。
- [x] `TestLoadServerLLMDefaults` を新設し、サーバの既定と更新前の DeepSeek の設定の拒否を検査する。サーバ固有の HOME・Webhook の必須条件を満たす環境で実行する。
- [x] 既定のモデルの枝、モデルだけの設定の判定、受け継ぐ出どころ、拒否を 1 件にまとめる処理をそれぞれ壊して対応するテストの失敗を確認する。受け継ぎの変更では、モデルの出どころが「変数」である経路を検査する。結果をコミットメッセージに記録する。
- [x] 2-1・3-1 の実装を入れた状態で、`internal/config/config_test.go` と `cmd/` のテストを基点の版に一時的に戻して `make test` を実行し、失敗する部分テストが設計書 §1.4 の 4 件に限られることを確認する。1-1 の切り替えでは見つからない、モデルの既定と拒否に依存するテストをここで洗い出す。一時的な変更は戻し、結果をコミットメッセージに記録する。

**完了条件:** 2-2 のテストと合わせて設計書 §3.2・§3.3 の全行を確認できる。AC-16〜AC-21 の拒否の種類・件数・合わせて報告する拒否と、値を含まないことが確認できる。

#### ステップ 3-2: 入口とプロバイダの受け渡し、先行タスクとの組み合わせを確認する

**変更ファイル:** `cmd/yt2column/run_test.go`、`internal/config/config_test.go`、`internal/llm/provider/provider_test.go`。

- [x] `TestRunDefaultLLMConfig` を新設する。既存の `newRunEnv` の LLM の変数を入れ替え、Anthropic の偽のキーと明示した effort で、既定のプロバイダ・モデルがクライアント構築に届くことを確認する。
- [x] `TestRunModelWithoutProvider` を新設する。終了コード 2、変数を挙げる標準エラー、クライアントを作らないこと、既存の tripwire が起動されないことを検査する。
- [x] 生成と異なるモデルを使う Claude の検証・推敲のモデルが構築に届くことは、既存の `TestNewClaudeDefaultEffort`（0041 が追加）の `both stages claude review` の行で確かめる。重複するので部分テストは加えない。生成のリクエストと字数の調整の共有は既存のテストを使う（§4.1）。
- [x] `TestLoadProductionDefaults` を新設する。LLM の変数のうち Anthropic の偽のキーだけで、生成と検証・推敲のモデル・プロバイダと effort `high` を確認する。このテストだけは effort を明示しない。
- [x] AC-22 のテストを #171 の effort の既定がない状態でも実行し、`YT2COLUMN_CLAUDE_EFFORT` の未設定を理由に失敗するか確認して、本書とコミットメッセージに記録する。#171 のマージ前に作業していればその状態で実行する。マージ後なら、#171 が `loadClaudeEffort` に加えた既定の枝を一時的に外し（未設定を `ErrMissing` に戻す）、実行してから戻す。ほかの失敗で隠さないよう、0042 の既定はそのままにする。確認できた範囲だけを CI によるマージ順序の検査として扱う。

**完了条件:** #171 の実装を含む状態で全テストが通り、§3.1 のマージ順序を確認できる。#171 が未マージなら、このフェーズを最終確認済みにはしない。2-1〜3-1 と、3-2 のうち AC-22 以外の作業は、#171 のマージ前に進めてよい。

### フェーズ 4: 文書

#### ステップ 4-1: 既定と移行の案内を更新して値を検査する

**変更ファイル:** `README.md`、`docs/dev/project_overview.md`、`docs/dev/developer_guide/package_reference.md`、`cmd/yt2column/docs_test.go`、`internal/config/test_helpers.go`。

- [x] 設計書 §3.6 に挙げた表・説明・サーバのトラブル対処と、DeepSeek を使い続ける案内を更新する。`project_overview.md` の方針の 2 か所も改める。#171 の effort の記述を維持し、#488 が先に更新していれば、その版の説明に合わせる。
- [x] `test_helpers.go` に `DefaultProviderValueForTest`・`DefaultClaudeModelForTest` を追加する。先頭の `//go:build test` を維持する。
- [x] `TestLLMDefaultsDocsContract` を新設する。既存の Markdown の表を読む関数を再利用し、設計書 §7.3 に従い、未設定時の列の最初のコードスパンをコードの定数と比較する。文章の意味はテストにしない。
- [x] 新しい表の説明と本文を要件書 AC-25〜AC-29・実装した設定の解決と照合する。`package_reference.md` も実装を根拠に読む。モデルだけの設定を消すと送り先が変わることと、DeepSeek の明示を案内することを確認する。
- [x] `make test`・`make lint`・`make ext-test` を実行する。文書の値の変更も壊して対応するテストの失敗を確かめ、記録する。

**完了条件:** 文書の値はコードと一致し、移行の案内を PR レビューで読んで確認できる。

### PR-2 作成ポイント: Claude defaults and migration guard

**対象ステップ**: 2-1 / 2-2 / 3-1 / 3-2 / 4-1

**推奨タイトル**: `feat(0042): 既定の LLM を Claude Opus 5.5 にして旧設定を安全に拒否する`

**レビュー観点**: 既定と出どころの受け継ぎ / 通信前の拒否と重複の防止 / #171 との組み合わせ / 文書の値と移行の案内

**実装モデル要件**: frontier-required

**判定理由**: 2-1・3-1 は送り先の既定を変えると同時に旧設定を拒否する移行であり、mkplan のセキュリティの判定・移行の条件に該当する。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序と前提

1-1 → 1-2 → 2-1 → 2-2 → 3-1 → 3-2 → 4-1 の順で進める。フェーズの名前と順序は設計書 §8 に従う。PR-1 で準備をマージし、PR-2 では設定の変更と拒否、文書をそろえて完了させる。

#171（タスク 0041）の実装との順序は、PR ごとに次のとおりとする（要件書 §5、設計書 §8）。

- **PR-1:** #171 に依存しない。振る舞いを変えず、#171 が変える `internal/config`・README・`project_overview.md` に触れないので、#171 のマージを待たずに作成・マージする。
- **PR-2:** 作業は #171 と並行して進めてよい。PR の作成とマージは、0041 の実装（effort の既定 `high` と `max_tokens` の上限の緩和）が `main` に入った後とする。作成の前とマージの前に 0041 の実装が `main` にあることを確認し、PR に根拠を記録する。0041 の設計の承認だけでは条件を満たさない。未マージなら待つ。作成を待つのは、AC-22 のテストが #171 なしでは CI で失敗し、4-1 の文書が #171 の書き換えた effort の行を前提にするからである。

#171 の変更を `main` から取り込んでから AC-22 を最終確認する。`config.go`・README・`project_overview.md` の衝突は #171 の effort の設計を維持して解く。#488（タスク 0040）の実装は本タスクの後にマージする。順序が逆になった場合の文書の調整は 4-1 で行う。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 | DeepSeek を明示するテスト環境と、通信なしの統合テスト設定の検査 | standard |
| PR-2 | 2-1 / 2-2 / 3-1 / 3-2 / 4-1 | Claude の既定、モデルだけの設定の拒否、文書と値の検査 | frontier-required |

PR-1 は本番の設定の解決を変えずに確認できる。PR-2 は設定の移行を一体でレビューする。設計書 §8 の制約に従い、フェーズ 2〜4 と AC-22 の検査を分割しない。ステップの並べ替えと番号の変更はない。

## 4. テスト戦略 (Test Strategy)

### 4.1. 検証の分担

§8 の新設テストはすべて実行する。`internal/config/config_test.go` に `TestLoadGenerationDefaults`・`TestLoadReviewDefaults`・`TestLoadModelWithoutProvider`・`TestModelWithoutProvider`・`TestLoadDefaultProviderVariables`・`TestLoadServerLLMDefaults`・`TestLoadProductionDefaults`、`cmd/yt2column/run_test.go` に `TestRunDefaultLLMConfig`・`TestRunModelWithoutProvider`、`docs_test.go` に `TestLLMDefaultsDocsContract`、`integration_env_test.go`（`test` タグ）に `TestIntegrationRunProviderSelection` を新設する。

既存の `envLookup`・`validEnv`・`validClaudeEnv`・`with`・`without`、CLI の fake と tripwire、プロバイダの `builders`・`claudeRequestFor`、文書の表を読む関数を使う。複数の拒否を調べる共通関数が必要なら、`internal/config/test_helpers.go` に置き、`test` タグを維持する。新しい `testutil/` は作らない。

生成のモデルと effort がリクエストに入ることは `internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`、字数の調整が生成のクライアントを共有することは `internal/writer/steps_test.go::TestWriteStepOrder` で確認する。これらを複製せず、設定の結果と既存の受け渡しを合わせて検証する。

### 4.2. 実行範囲と外部の前提

各 PR は `make test` と `make lint` を通す。文書を変える PR-2 は `make ext-test` も通す。`TestIntegrationRunProviderSelection` は `test` タグのファイルに置くので `make test` と CI で走り、別に実行する手順は要らない。

ビルドタグは `Makefile:85-99`・`:128-181` と `.github/workflows/ci.yml`（`make test-ci`、`go vet -tags integration ./...`、`doc-guard` の `go test -tags test ./cmd/yt2column`）で確認した。移す `newIntegrationRun` の依存（`transcript.SeedCacheForTest`・`transcripttestutil.NewTripwire`・`deepseektestutil.IntegrationSettings`・`lookupFrom`・`generateCounter`）は、どれも `test || integration` のファイルにある。

実 API を呼ぶ `make test-integration-cli`・`make test-integration-cli-slack` は承認後の補助確認とする。承認がなければ未実行と報告する。テスト専用のキー・Webhook と既存のオプトインを使い、本番の値は使わない。シェルから統合テストへ渡すモデル名は既定に影響されうるため、DeepSeek のテストモデルであることを事前に確認する（設計書 §7.4）。新しい外部サービス・CI の仕組みは加えない。

### 4.3. コメントと文書の整合

設定ローダーを変えるときは、旧関数のコメントとともに、`parseProvider` の未設定、`loadModel` の既定なし、`loadModelOptional` のモデルを空にする説明、`loadReview` のモデル必須・DeepSeek への既定・不完全な設定という説明を更新する。`loadProviderVars` の削除する関数への参照と、`LoadFor` のモデル変数を必須とする説明も改める。`LLMConfig.Model`・`ReviewModel` のコメントも設計書 §3.2・§3.3 に合わせる。正確な英語の表現は実装時に決める。

文書・用語の残りは §8 の文書の確認に含め、独立した文言検索のチェックリストは作らない。旧関数の参照は全呼び出し側の更新とコンパイルで確認し、同名の別パッケージの関数は変えない。

## 5. リスク管理 (Risk Management)

| リスク | 対策・遅れた場合の扱い |
|---|---|
| 既定の送り先だけが変わり、旧設定のデータを送る | フェーズ 2〜4 を同じ PR に入れ、3-1・3-2 の拒否と通信前の停止を確認する。`docs/dev/security.md` §2〜§4・§8.2 に照らしてレビューする |
| 出どころを失う、拒否を重複して加える | 受け継ぎの実行の種類を検査し、拒否の名前・理由だけでなく件数も確認する。出どころが「なし」の場合も単体で検査する |
| API キーや設定値がエラーに混ざる | 固定の理由と既存の秘密情報の型を維持し、目印による検査と他プロバイダの非参照の検査を実行する |
| #171 の実装がまだない | PR-1 は待たずに進める。PR-2 は 2-1〜3-1 を進め、作成とマージを待つ。先行実装との統合確認の時間をフェーズ 3 に確保し、AC-22 の結果を記録する。effort の既定を本タスクで先に実装しない |
| 古い既定に依存するテストが残る | 1-1 の一時的な既定の切り替えで洗い出す。型検査には `integration` タグのファイルも含める |
| 復旧時に実装と文書が食い違う | PR-2 を戻す必要があれば、既定・拒否・文書・対応テストを含む squash コミット全体を戻し、グリーンゲートを通す。既定の定数だけを戻さない |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 3-1 / 3-2 / 4-1）
- [x] §8 の検証を実行し、結果と壊して確かめた内容を記録した
- [x] #171 の実装とのマージ順序を確認した

## 7. 成功基準 (Success Criteria)

AC-01〜AC-29 が §8 の対応で確認され、必要なテスト・lint・文書の値の検査が通っていること。モデルだけの旧設定は通信前に拒否され、明示した DeepSeek の設定は従来どおり動くこと。文書の意味は PR レビューで確認され、実 API の未実行を成功したものとして扱っていないこと。

## 8. 受け入れ基準の検証 (Acceptance Criteria Verification)

以下のパスはリポジトリのルートからの相対パス。テスト名が同じ行で複数ある場合は、すべて実行する。新設・拡張の内容は §2・§4 に記す。`manual` は PR レビューで読む確認である。

| AC | 実装ステップ | 種別 | 実行する検証 |
|---|---|---|---|
| AC-01 | 2-1 / 2-2 / 3-1 / 3-2 | test | `internal/config/config_test.go::TestLoadGenerationDefaults`、`internal/config/config_test.go::TestLoadDefaultProviderVariables`、`cmd/yt2column/run_test.go::TestRunDefaultLLMConfig` |
| AC-02 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadGenerationDefaults` |
| AC-03 | 2-1 / 2-2 / 3-2 | test | `internal/config/config_test.go::TestLoadGenerationDefaults`、`internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`、`internal/writer/steps_test.go::TestWriteStepOrder` |
| AC-04 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadGenerationDefaults` |
| AC-05 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadGenerationDefaults` |
| AC-06 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadGenerationDefaults` |
| AC-07 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults` |
| AC-08 | 2-1 / 2-2 / 3-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults`、`internal/llm/provider/provider_test.go::TestNewClaudeDefaultEffort`、`internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest` |
| AC-09 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults` |
| AC-10 | 2-1 / 2-2 / 3-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults`、`internal/llm/provider/provider_test.go::TestNewClaudeDefaultEffort` |
| AC-11 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults` |
| AC-12 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults` |
| AC-13 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults` |
| AC-14 | 2-1 / 2-2 / 3-1 | test | `internal/config/config_test.go::TestLoadReviewDefaults`、`internal/config/config_test.go::TestLoadModelWithoutProvider` |
| AC-15 | 2-2 | test | `internal/config/config_test.go::TestLoadFor`（LLM の変数を読まない実行の検査を拡張） |
| AC-16 | 3-1 / 3-2 | test | `internal/config/config_test.go::TestLoadModelWithoutProvider`、`cmd/yt2column/run_test.go::TestRunModelWithoutProvider` |
| AC-17 | 3-1 | test | `internal/config/config_test.go::TestLoadModelWithoutProvider` |
| AC-18 | 3-1 | test | `internal/config/config_test.go::TestLoadModelWithoutProvider` |
| AC-19 | 3-1 | test | `internal/config/config_test.go::TestLoadModelWithoutProvider`、`internal/config/config_test.go::TestModelWithoutProvider`、`internal/config/config_test.go::TestLoadErrorsOmitValues` |
| AC-20 | 3-1 | test | `internal/config/config_test.go::TestLoadModelWithoutProvider` |
| AC-21 | 3-1 | test | `internal/config/config_test.go::TestLoadDefaultProviderVariables`、`internal/config/config_test.go::TestLoadErrorsOmitValues` |
| AC-22 | 3-2 | test | `internal/config/config_test.go::TestLoadProductionDefaults`、`internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`、`internal/llm/provider/provider_test.go::TestNewClaudeDefaultEffort`、`internal/writer/steps_test.go::TestWriteStepOrder` |
| AC-23 | 2-1 / 3-1 | test | `internal/config/config_test.go::TestLoadIgnoresOtherProviderVars`、`internal/config/config_test.go::TestLoadReview`、`internal/writer/steps_test.go::TestWriteStepOrder` |
| AC-24 | 1-1 / 1-2 | test | `cmd/yt2column/integration_env_test.go::TestIntegrationRunProviderSelection`（通信なしで共通の環境を検査。`make test`・CI で実行） |
| AC-25 | 4-1 | test、manual | `cmd/yt2column/docs_test.go::TestLLMDefaultsDocsContract`。分岐の説明は設定の実装・要件書と照合して PR レビューで読む |
| AC-26 | 4-1 | manual | README・`project_overview.md` の表の説明と本文を、AC-10・AC-11・F-004 と照合して読む。`package_reference.md` の同じ規則も読む |
| AC-27 | 4-1 | manual | README・`project_overview.md` の DeepSeek を使い続ける案内を要件書と照合して読む |
| AC-28 | 4-1 | manual | `project_overview.md` の LLM の方針を AC-28 と照合して読む |
| AC-29 | 4-1 | manual | `project_overview.md` のモデル名の方針を AC-29 と照合して読む |

## 9. 次のステップ (Next Steps)

本計画の PR をレビューし、`/runpr` で承認・マージする。その後、承認済みの本計画に対して `/runplan 0042` を実行し、PR-1 を作る。PR-1 は #171 を待たない。PR-2 の作成の前に、#171 の実装のマージを確認する。各 PR の検証結果と進捗は実装時に本書へ記録する。

### 9.1. 実装の記録

**PR-1（2026-10-25）:**

- 1-1: `parseProvider` と `loadProviderOptional` の既定を一時的に `claude` に変えた `make test` では、`TestLoadDefaults` と `TestLoadMissing/provider_unset_and_API_key_unset` が失敗した（`TestLoadMissing` の部分テストは 1 件）。どちらも設計書 §1.4 の 4 件に含まれる。`TestLoadFor` の `review only requires a model`・`review only rejects an empty inherited model` は、`assertVarError` が最初の `*VarError` だけを調べるため失敗しなかった。設計書 §7.2 のとおり、この 2 件は 3-1 の確認で洗い出す。一時的な変更は戻し、`make test`・`make lint` が通ることを確かめた。
- 1-2: `newIntegrationRun` から `YT2COLUMN_LLM_PROVIDER=deepseek` を外すと、`TestIntegrationRunProviderSelection` が「共通の環境にプロバイダの明示がない」の理由で失敗した。戻した状態で `make test`・`make lint`・`go vet -tags integration ./...` が通る。CLI と Slack の統合テストは同じ `newIntegrationRun` の環境を使い、関数の外で LLM の変数を組み立て直していないことを読んで確認した（Slack のテストが足すのは `SLACK_WEBHOOK_URL` だけである）。実 API を呼ぶ 2 つの統合テストは実行していない。

**PR-2（2026-10-25）:**

- フェーズ 2・3 を 1 つのグループとして実装した。フェーズ 2 だけでは出どころの型が未使用になって `make lint` が通らず、既定だけが入って拒否のない状態にもなるためである。
- 3-1 の「基点の版」は PR-2 の作業の開始時点（コミット `05c1c92`。PR-1 と #171 を含む `main`）と解した。調査の基点 `68f4cb9` の `cmd/` のテストは PR-1 の DeepSeek の明示を含まず、既定の変更と無関係に失敗するからである。2-1・3-1 の実装を入れ、テストをこの版のままにした `make test` では、失敗は `TestLoadDefaults`、`TestLoadMissing/provider_unset_and_API_key_unset`、`TestLoadFor/review_only_requires_a_model` だけで、設計書 §1.4 の 4 件に含まれる。`TestLoadFor/review_only_rejects_an_empty_inherited_model` は、`assertVarError` が最初の `*VarError`（`YT2COLUMN_MODEL`）だけを調べるため失敗しなかったが、DeepSeek の明示を加えて意図を保った。`cmd/` のテストに失敗はなかった。
- 削った `TestLoadMissing` の行について、新しいテストを入れた状態で、削る前（行を戻した状態）と後の `go tool cover -func` を比べ、関数ごとの値が同じであることを確かめた。
- 壊して確かめたこと: 既定のモデルの `ProviderClaude` の枝を外すと `TestLoadGenerationDefaults`・`TestLoadReviewDefaults`・`TestLoadDefaultProviderVariables` が失敗する。モデルだけの設定の判定を外すと `TestLoadModelWithoutProvider`・`TestModelWithoutProvider`・`TestLoadServerLLMDefaults`・`TestLoadDefaultProviderVariables`・`TestRunModelWithoutProvider` が失敗する。判定を段階ごとに行うと `both models` などの件数の検査が失敗する。条件を「出どころが既定」に書き換えると `TestModelWithoutProvider` の出どころ「なし」の行が失敗する。生成を実行する実行で受け継ぐプロバイダの出どころを「変数」にすると、`TestLoadModelWithoutProvider` の `review model`・`review model empty` が失敗する。検証・推敲だけを実行する実行で、受け継ぐプロバイダまたはモデルの出どころを写さないと、AC-17・AC-18 の行が失敗する。拒否の理由に値を混ぜると `TestLoadErrorsOmitValues/model_without_provider` が失敗する。既定のプロバイダを `deepseek` に戻すと `TestRunDefaultLLMConfig` が失敗する。
- AC-22: #171 が `loadClaudeEffort` に加えた既定の枝を一時的に外し（未設定を `ErrMissing` に戻し）、0042 の既定はそのままにして実行すると、`TestLoadProductionDefaults` は `YT2COLUMN_CLAUDE_EFFORT: is unset or empty` で失敗した（`internal/config` のテストで、ほかに失敗したのは #171 自身のテスト `TestLoadClaudeMissing`・`TestLoadClaudeEffortDefault` だけ）。したがって、このテストは #171 の effort の既定がない状態のマージを CI で止める。戻した状態で通る。
- 4-1 の `DefaultProviderValueForTest`・`DefaultClaudeModelForTest` は、`TestRunDefaultLLMConfig` が既定のモデルを読むため、このグループで先に `internal/config/test_helpers.go` に加えた。
- 4-1: README「Configuration」の表と本文、新しい「Using DeepSeek」の節、「Localhost server」の「Troubleshooting」の項目、`project_overview.md` の方針の 2 か所と「設定（環境変数）」の表と本文、`package_reference.md` の `internal/config` の行を改めた。#488 の文書はまだ `main` にないので、今の文書に合わせた。本文は `config.go` の解決（`resolveProvider`・`loadModel`・`loadModelOptional`・`loadReview`・`modelWithoutProvider`）と、サーバが設定の拒否で終了コード 2 を返すこと（`cmd/yt2column-server/run.go` の `exitUsage`）を根拠に書いた。`TestLLMDefaultsDocsContract` は、README の `YT2COLUMN_LLM_PROVIDER` の未設定の列を `deepseek` に戻すと失敗し、`defaultClaudeModel` の値を変えると両文書の 2 行で失敗する。最初のコードスパンの前に文を足しただけの変更では失敗しない（文の意味は PR レビューで読む）。`make test`・`make lint`・`make ext-test` が通る。
- PR-2 の作成の前に、0041 の実装（#498、コミット `7abc0e1`）が `origin/main` にあることを確かめた。マージの前にも `/runpr` で確かめる。
