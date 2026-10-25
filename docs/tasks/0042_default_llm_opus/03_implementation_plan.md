# 実装計画書：既定の LLM を Claude Opus 5.5 にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-25 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的と原則

[要件定義書](01_requirements.md) の AC-01〜AC-29 を、[設計書](02_architecture.md) §3・§7・§8 に従って実装する。以下の「段階」「出どころ」「モデルだけの設定の拒否」は、設計書冒頭の定義に従う。段階は、本書では設定を解決する生成と検証・推敲の単位を指す。

- 既定と拒否の判定は `internal/config` に置く。モデル名の中身でプロバイダを推測しない。
- 既定の変更、モデルだけの設定の拒否、利用者への案内を同じ PR に入れる。途中のフェーズの状態を `main` に入れない。
- テストでは fake の環境、既存の tripwire、ループバックの受信先を使う。実 API・実 Webhook を呼ぶ統合テストは別途人間の承認を得て実行する。
- Go のコメント・識別子・文字列は英語とし、受け入れ基準との対応は本書に記録する。

### 1.2. 計画の際に決めたこと（2026-10-25）

- **実装を 2 PR に分ける（人間のレビュアーの選択）。** PR-1 はフェーズ 1 のテスト環境の修正、PR-2 はフェーズ 2〜4 とする。振る舞いを変えない準備を先にレビューし、送り先の変更と拒否・案内は一緒にレビューするためである。
- **既定のモデルの定数は設計どおり `internal/config` に置く。** タスク 0041 の承認済み設計書 §1.2 も、effort の既定を同じパッケージに置くと決めている。0042 の設計書 §1.2 が留保した配置の調整は不要である。
- **AC-24 に通信なしの検証を加える。** 共通の `newIntegrationRun` が組み立てる環境を設定ローダーに渡し、生成と検証・推敲が DeepSeek を選ぶことをテストする。実 API の実行を承認する前にも、この設定の検証を実行できるようにする。

### 1.3. 既存コード調査結果

調査の基点はコミット `68f4cb99e4201c960230ec92a9d6aa7a3d8c3744`。この節の `file:line` はその版を指す。新設するテスト名には §4 で「新設」と付ける。実装を始めるときに、先行タスクの変更を反映して再確認する。

| 対象 | 現状と必要な変更 | 作業 |
|---|---|---|
| プロバイダの解決 | `internal/config/config.go:382-450` の `loadProvider`・`loadProviderOptional`・`parseProvider` に未設定時の処理がある。`loadLLM`（`:373`）と `loadReview`（`:482`）の呼び出しを共通の解決にまとめる | 2-1、3-1 |
| モデルの解決と受け継ぎ | 同 `:417-429` の `loadModelOptional`、`:461-514` の `loadReview`、`:545-553` の `loadModel` が設定する。`loadReview` の受け継ぎは `:477`・`:485`・`:508`。値と出どころを一緒に扱い、`LLMConfig` のフィールドは増やさない | 2-1、3-1 |
| 呼び出し側 | `LoadServer`（同 `:260`）と `LoadFor`（`:347`）が `loadLLM` を呼ぶ。`internal/llm/provider/provider.go:29-39` の `New`・`NewReview` はアクセサの値を渡す。設定の失敗時にゼロ値を返す既存の境界を保つ（`config.go:273-275`・`:355-357`） | 2-1、3-1、3-2 |
| 古い既定を使うテスト | `internal/config/config_test.go:108` の `TestLoadDefaults`、`:150` の欠落の行、`:1108`・`:1164` の `TestLoadFor` の部分テストが対象。残す検査と削る行を §2 に分ける | 2-2 |
| DeepSeek を前提にした環境 | `cmd/yt2column/run_test.go:119` の `newRunEnv`、`signal_test.go:46` の `newChildSetup`、`integration_test.go:112` の `newIntegrationRun`、`cmd/yt2column-server/test_helpers.go:121` の `newServerEnv` が対象。`integration_slack_test.go:40` は共通の `newIntegrationRun` を使う | 1-1、1-2 |
| 使い回す検査 | `internal/config/config_test.go:380` の `TestLoadIgnoresOtherProviderVars`、`:666` の `TestLoadErrorsOmitValues`、`:1015` の `TestLoadFor` を拡張・再利用する。`:578` の `assertVarError` は最初の `*VarError` を調べるため、複数の拒否の件数には全要素の検査が必要 | 2-2、3-1 |
| API への受け渡し | `internal/llm/provider/provider_test.go:257` の `TestNewClaudeSendsConfiguredRequest` と `:312` の `claudeRequestFor` が生成のモデル・effort のリクエストを検査する。`:442` の `TestNewReviewUsesReviewModel` に Claude のモデルの受け渡しを加える。字数の調整が生成のクライアントを使う検査は `internal/writer/steps_test.go:43` の `TestWriteStepOrder` を再利用する | 3-2 |
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

- [ ] §1.3 の 4 つの環境に `YT2COLUMN_LLM_PROVIDER=deepseek` を加える。Slack の統合テストにも同じ環境が渡ることを確認する。
- [ ] 設計書 §7.2 に従い、既存の 2 か所の既定を一時的に Claude に変えてテストを実行し、失敗する部分テストが設計書 §1.4 の 4 件に限られるか確認する。漏れは同じフェーズで直し、一時的な変更は戻す。結果をコミットメッセージに記録する。

**完了条件:** 一時的な変更を戻した状態で `make test`・`make lint` が通り、本番の既定はまだ変わっていない。

#### ステップ 1-2: 統合テストの設定を通信なしで確認する

**変更ファイル:** `cmd/yt2column/integration_test.go`。

- [ ] `TestIntegrationRunProviderSelection` を新設する。偽のテストキーとモデルで `newIntegrationRun` を呼び、共通の環境に生成のプロバイダが明示されていることを確認してから設定ローダーに渡す。生成・検証・推敲の両方が DeepSeek と指定モデルを選び、Anthropic の変数を読まないことを検査する。LLM の生成と Webhook の投稿は呼ばない。
- [ ] `test,integration` タグでこのテストだけを選んで実行する。CLI と Slack の統合テストが同じ環境を使うこともコードを読んで確認する。
- [ ] `newIntegrationRun` の明示したプロバイダを外して、このテストの環境の検査が失敗することを確認する。一時的な変更を戻し、結果を記録する。

**完了条件:** 統合テストのタグでコンパイルと選択実行が通る。実 API を呼ぶ 2 つの統合テストは実行していない。

### フェーズ 2: 既定のプロバイダとモデル

#### ステップ 2-1: 設定の解決を共通化し、既定を加える

**変更ファイル:** `internal/config/config.go`。

- [ ] 設計書 §3.1〜§3.3 に従い、`defaultProviderValue`・`defaultClaudeModel` と `defaultModel` を加える。既定のモデルを選ぶ `switch` の `default` は既定なしとする。
- [ ] §1.4 の表に従いプロバイダの解決をまとめ、全呼び出し側を更新する。未設定と空の区別を維持する。
- [ ] 生成・検証・推敲だけの受け継ぎ・検証用プロバイダだけの指定の各経路で、同じモデルの既定を使う。空・不正なプロバイダや空のモデルに既定を当てはめない。
- [ ] 出どころを値と一緒に返すための型と引数を設計書 §3.1 に合わせて準備する。コメントも新しい既定と受け継ぎに合わせる（§4.3）。

**完了条件:** 2-2 の既定のテストが通る。このフェーズだけでは PR を開かず、3-1 の拒否を続けて加える。

#### ステップ 2-2: 既定と拒否の既存テストを更新する

**変更ファイル:** `internal/config/config_test.go`。

- [ ] 設計書 §3.2・§3.3 の各行を、§7.1 の実行の種類で検査する `TestLoadGenerationDefaults`・`TestLoadReviewDefaults` を新設する。既定以外の生成モデルと、検証・推敲の既定のモデルも区別する。既存の検査と重複する行は同じテストにまとめる。
- [ ] `TestLoadDefaults` に DeepSeek の明示を加え、Slack・`yt-dlp`・キャッシュディレクトリの既存の検査を残す。
- [ ] `TestLoadMissing` の `provider unset and API key unset` の行だけを削る。DeepSeek のキーがない場合の残る行を確認し、削除の直前・直後の関数ごとのカバレッジを比較して結果を記録する。
- [ ] `TestLoadFor` の `review only requires a model` に DeepSeek の明示を加える。
- [ ] `TestLoadFor` の `review only rejects an empty inherited model` に DeepSeek の明示を加える。
- [ ] `TestLoadFor` の `no stage reads no LLM variable` を拡張し、全 LLM 変数の参照がなく、モデル・プロバイダのアクセサに既定が入らないことを検査する。

**完了条件:** 既定を当てはめる場合・当てはめない場合と、実行しない段階の扱いが §4 のテストで確認できる。

### フェーズ 3: モデルだけの設定の拒否

#### ステップ 3-1: 出どころを受け継いで一度だけ拒否する

**変更ファイル:** `internal/config/config.go`・`config_test.go`。

- [ ] 設計書 §3.4・§4.1 に従い `reasonModelWithoutProvider` と `modelWithoutProvider` を加え、必要な段階の解決後に一度だけ判定する。拒否後もプロバイダ固有の変数を検査し、値を含まない固定の理由で報告する。理由の文言は設計書 §1.2・§4.2 を使う。
- [ ] `TestLoadModelWithoutProvider`・`TestModelWithoutProvider` を新設する。複数の拒否は全要素を調べ、名前・番兵・理由・件数を検査する。空の変数の拒否と区別し、出どころが「なし」の場合も拒否する。
- [ ] `TestLoadDefaultProviderVariables` を新設し、参照した変数を記録して、既定の Claude の変数だけを検査することを確認する。`TestLoadErrorsOmitValues` にモデルだけの設定と目印を加える。DeepSeek の非参照は既存の `TestLoadIgnoresOtherProviderVars` を再利用する。
- [ ] `TestLoadServerLLMDefaults` を新設し、サーバの既定と更新前の DeepSeek の設定の拒否を検査する。サーバ固有の HOME・Webhook の必須条件を満たす環境で実行する。
- [ ] 既定のモデルの枝、モデルだけの設定の判定、受け継ぐ出どころ、拒否を 1 件にまとめる処理をそれぞれ壊して対応するテストの失敗を確認する。受け継ぎの変更では、モデルの出どころが「変数」である経路を検査する。結果をコミットメッセージに記録する。

**完了条件:** AC-16〜AC-21 の拒否の種類・件数・合わせて報告する拒否と、値を含まないことが確認できる。

#### ステップ 3-2: 入口とプロバイダの受け渡し、先行タスクとの組み合わせを確認する

**変更ファイル:** `cmd/yt2column/run_test.go`、`internal/config/config_test.go`、`internal/llm/provider/provider_test.go`。

- [ ] `TestRunDefaultLLMConfig` を新設する。既存の `newRunEnv` の LLM の変数を入れ替え、Anthropic の偽のキーと明示した effort で、既定のプロバイダ・モデルがクライアント構築に届くことを確認する。
- [ ] `TestRunModelWithoutProvider` を新設する。終了コード 2、変数を挙げる標準エラー、クライアントを作らないこと、既存の tripwire が起動されないことを検査する。
- [ ] `TestNewReviewUsesReviewModel` に、生成と異なるモデルを使う Claude の検証・推敲のモデルが構築に届く検査を加える。生成のリクエストと字数の調整の共有は既存のテストを使う（§4.1）。
- [ ] `TestLoadProductionDefaults` を新設する。LLM の変数のうち Anthropic の偽のキーだけで、生成と検証・推敲のモデル・プロバイダと effort `high` を確認する。このテストだけは effort を明示しない。
- [ ] AC-22 のテストを #171 の変更がない状態でも実行し、effort の未設定を理由に失敗するか確認して、本書とコミットメッセージに記録する。ほかの失敗で隠さないよう設定の既定は 0042 のままにする。確認できた範囲だけを CI によるマージ順序の検査として扱う。

**完了条件:** #171 の実装を含む状態で全テストが通り、§3.1 のマージ順序を確認できる。#171 が未マージなら、このフェーズを最終確認済みにはしない。

### フェーズ 4: 文書

#### ステップ 4-1: 既定と移行の案内を更新して値を検査する

**変更ファイル:** `README.md`、`docs/dev/project_overview.md`、`docs/dev/developer_guide/package_reference.md`、`cmd/yt2column/docs_test.go`、`internal/config/test_helpers.go`。

- [ ] 設計書 §3.6 に挙げた表・説明・サーバのトラブル対処と、DeepSeek を使い続ける案内を更新する。`project_overview.md` の方針の 2 か所も改める。#171 の effort の記述を維持し、#488 が先に更新していれば、その版の説明に合わせる。
- [ ] `test_helpers.go` に `DefaultProviderValueForTest`・`DefaultClaudeModelForTest` を追加する。先頭の `//go:build test` を維持する。
- [ ] `TestLLMDefaultsDocsContract` を新設する。既存の Markdown の表を読む関数を再利用し、設計書 §7.3 に従い、未設定時の列の最初のコードスパンをコードの定数と比較する。文章の意味はテストにしない。
- [ ] 新しい表の説明と本文を要件書 AC-25〜AC-29・実装した設定の解決と照合する。`package_reference.md` も実装を根拠に読む。モデルだけの設定を消すと送り先が変わることと、DeepSeek の明示を案内することを確認する。
- [ ] `make test`・`make lint`・`make ext-test` を実行する。文書の値の変更も壊して対応するテストの失敗を確かめ、記録する。

**完了条件:** 文書の値はコードと一致し、移行の案内を PR レビューで読んで確認できる。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序と前提

1-1 → 1-2 → 2-1 → 2-2 → 3-1 → 3-2 → 4-1 の順で進める。フェーズの名前と順序は設計書 §8 に従う。PR-1 で準備をマージし、PR-2 では設定の変更と拒否、文書をそろえて完了させる。

本タスクの実装のマージは #171（タスク 0041）の実装のマージ後とする。PR-1 にもこの条件を適用する。各実装 PR の作成前とマージ前に、0041 の実装が `main` にあることを確認し、PR に根拠を記録する。0041 の設計の承認だけでは条件を満たさない。未マージなら待つ。

#171 の変更を取り込んでから AC-22 を最終確認する。`config.go`・README・`project_overview.md` の衝突は #171 の effort の設計を維持して解く。#488（タスク 0040）の実装は本タスクの後にマージする。順序が逆になった場合の文書の調整は 4-1 で行う。

## 4. テスト戦略 (Test Strategy)

### 4.1. 検証の分担

§8 の新設テストはすべて実行する。`internal/config/config_test.go` に `TestLoadGenerationDefaults`・`TestLoadReviewDefaults`・`TestLoadModelWithoutProvider`・`TestModelWithoutProvider`・`TestLoadDefaultProviderVariables`・`TestLoadServerLLMDefaults`・`TestLoadProductionDefaults`、`cmd/yt2column/run_test.go` に `TestRunDefaultLLMConfig`・`TestRunModelWithoutProvider`、`docs_test.go` に `TestLLMDefaultsDocsContract`、`integration_test.go` に `TestIntegrationRunProviderSelection` を新設する。

既存の `envLookup`・`validEnv`・`validClaudeEnv`・`with`・`without`、CLI の fake と tripwire、プロバイダの `builders`・`claudeRequestFor`、文書の表を読む関数を使う。複数の拒否を調べる共通関数が必要なら、`internal/config/test_helpers.go` に置き、`test` タグを維持する。新しい `testutil/` は作らない。

生成のモデルと effort がリクエストに入ることは `internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`、字数の調整が生成のクライアントを共有することは `internal/writer/steps_test.go::TestWriteStepOrder` で確認する。これらを複製せず、設定の結果と既存の受け渡しを合わせて検証する。

### 4.2. 実行範囲と外部の前提

各 PR は `make test` と `make lint` を通す。文書を変える PR-2 は `make ext-test` も通す。`TestIntegrationRunProviderSelection` は通常の `make test` に含まれないので、PR-1 と PR-2 で `test,integration` タグを指定してこのテストだけを実行する。実 API を呼ぶテストを選ばない。

ビルドタグとテスト選択はローカルの `go help testflag`、`Makefile:85-99`・`:128-181` と実際のビルドで確認した。2026-10-25、基点のコミットで `go test -tags test,integration -run '^$' ./cmd/yt2column` を実行し、`ok github.com/isseis/yt2column/cmd/yt2column [no tests to run]`、終了コード 0 を得た。`TestMain` のループバック待受けが必要なため、待受けを許可した環境で実行した。

実 API を呼ぶ `make test-integration-cli`・`make test-integration-cli-slack` は承認後の補助確認とする。承認がなければ未実行と報告する。テスト専用のキー・Webhook と既存のオプトインを使い、本番の値は使わない。シェルから統合テストへ渡すモデル名は既定に影響されうるため、DeepSeek のテストモデルであることを事前に確認する（設計書 §7.4）。新しい外部サービス・CI の仕組みは加えない。

### 4.3. コメントと文書の整合

設定ローダーを変えるときは、旧関数のコメントとともに、`parseProvider` の未設定、`loadModel` の既定なし、`loadModelOptional` のモデルを空にする説明、`loadReview` のモデル必須・DeepSeek への既定・不完全な設定という説明を更新する。`LLMConfig.Model`・`ReviewModel` のコメントも設計書 §3.2・§3.3 に合わせる。正確な英語の表現は実装時に決める。

文書・用語の残りは §8 の文書の確認に含め、独立した文言検索のチェックリストは作らない。旧関数の参照は全呼び出し側の更新とコンパイルで確認し、同名の別パッケージの関数は変えない。

## 5. リスク管理 (Risk Management)

| リスク | 対策・遅れた場合の扱い |
|---|---|
| 既定の送り先だけが変わり、旧設定のデータを送る | フェーズ 2〜4 を同じ PR に入れ、3-1・3-2 の拒否と通信前の停止を確認する。`docs/dev/security.md` §2〜§4・§8.2 に照らしてレビューする |
| 出どころを失う、拒否を重複して加える | 受け継ぎの実行の種類を検査し、拒否の名前・理由だけでなく件数も確認する。出どころが「なし」の場合も単体で検査する |
| API キーや設定値がエラーに混ざる | 固定の理由と既存の秘密情報の型を維持し、目印による検査と他プロバイダの非参照の検査を実行する |
| #171 の実装がまだない | マージを待つ。先行実装との統合確認の時間をフェーズ 3 に確保し、AC-22 の結果を記録する。effort の既定を本タスクで先に実装しない |
| 古い既定に依存するテストが残る | 1-1 の一時的な既定の切り替えで洗い出す。型検査には `integration` タグのファイルも含める |
| 復旧時に実装と文書が食い違う | PR-2 を戻す必要があれば、既定・拒否・文書・対応テストを含む squash コミット全体を戻し、グリーンゲートを通す。既定の定数だけを戻さない |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（1-1 / 1-2）
- [ ] フェーズ 2 完了（2-1 / 2-2）
- [ ] フェーズ 3 完了（3-1 / 3-2）
- [ ] フェーズ 4 完了（4-1）
- [ ] §8 の検証を実行し、結果と壊して確かめた内容を記録した
- [ ] #171 の実装とのマージ順序を確認した

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
| AC-08 | 2-1 / 2-2 / 3-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults`、`internal/llm/provider/provider_test.go::TestNewReviewUsesReviewModel`、`internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest` |
| AC-09 | 2-1 / 2-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults` |
| AC-10 | 2-1 / 2-2 / 3-2 | test | `internal/config/config_test.go::TestLoadReviewDefaults`、`internal/llm/provider/provider_test.go::TestNewReviewUsesReviewModel` |
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
| AC-22 | 3-2 | test | `internal/config/config_test.go::TestLoadProductionDefaults`、`internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`、`internal/llm/provider/provider_test.go::TestNewReviewUsesReviewModel`、`internal/writer/steps_test.go::TestWriteStepOrder` |
| AC-23 | 2-1 / 3-1 | test | `internal/config/config_test.go::TestLoadIgnoresOtherProviderVars`、`internal/config/config_test.go::TestLoadReview`、`internal/writer/steps_test.go::TestWriteStepOrder` |
| AC-24 | 1-1 / 1-2 | test | `cmd/yt2column/integration_test.go::TestIntegrationRunProviderSelection`（通信なしで共通の環境を検査） |
| AC-25 | 4-1 | test、manual | `cmd/yt2column/docs_test.go::TestLLMDefaultsDocsContract`。分岐の説明は設定の実装・要件書と照合して PR レビューで読む |
| AC-26 | 4-1 | manual | README・`project_overview.md` の表の説明と本文を、AC-10・AC-11・F-004 と照合して読む。`package_reference.md` の同じ規則も読む |
| AC-27 | 4-1 | manual | README・`project_overview.md` の DeepSeek を使い続ける案内を要件書と照合して読む |
| AC-28 | 4-1 | manual | `project_overview.md` の LLM の方針を AC-28 と照合して読む |
| AC-29 | 4-1 | manual | `project_overview.md` のモデル名の方針を AC-29 と照合して読む |

## 9. 次のステップ (Next Steps)

本計画の PR をレビューし、`/runpr` で承認・マージする。その後、#171 の実装のマージを確認し、承認済みの本計画に対して `/runplan 0042` を実行する。各 PR の検証結果と進捗は実装時に本書へ記録する。
