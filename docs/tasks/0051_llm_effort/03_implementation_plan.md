# 実装計画書：`YT2COLUMN_LLM` に effort を含め、`YT2COLUMN_CLAUDE_EFFORT` を廃止する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-30 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.2」などは設計書の節を指す。用語は設計書の冒頭の定義（工程の選択、パラメータのパッケージ、設定されている、など）と同じ意味で使う。既存のコードについての記述は、コミット `0f1c59b3` で確かめた。`file:line` はこのコミットの行番号である。設計書が確かめたコミット `be657abc` から `0f1c59b3` までに、`internal/config/config.go`・`internal/llm/`・`cmd/yt2column/`・`cmd/yt2column-server/` の本体と、本書で変えるテストは変わっていない（`git diff --stat be657abc 0f1c59b3 -- internal cmd` で変わったのは `cmd/codexreview`・`cmd/fixprguard`・`internal/codexreview`・`internal/doctestutil`・`internal/config/envaccess_test.go` だけである）。そのため、設計書の行番号もそのまま使える。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の値の `+` の後ろを effort として解釈し、工程ごとにアダプタへ渡す。DeepSeek では `reasoning_effort` を送れるようにする。`YT2COLUMN_CLAUDE_EFFORT` は旧変数として拒否する。文書とそのガードテストを新しい規則に合わせる。設計は設計書 1〜6 章のとおりである。

### 1.2. 実装の原則

-   設計書のフェーズの順（設計書 8 章の手順 1〜4）で実装する。本書のフェーズ 1〜4 は、その手順 1〜4 にそのまま当たる。
-   型・関数・理由・検査の順序は設計書 3〜4 章で決まっている。本書は、それをどのファイルのどのテストで確かめるかと、作業の順を定める。設計の内容は繰り返さず、節を参照する。
-   既存のテストの表と補助関数を使い回す。旧変数の一覧を回すテスト（`RetiredLLMVarsForTest` と `retiredLLMVars` を回すもの）は、`YT2COLUMN_CLAUDE_EFFORT` を一覧に加えれば自動で対象になる。そのため、同じ確認を別のテストとして書き足さない（1.4 の表）。
-   テストを加えたら、その対象を壊して失敗することを確かめ、コミットメッセージにそう書く（`CLAUDE.md`「Testing Strategy」）。
-   Go のコメント・識別子・文字列リテラルは英語で書く。

### 1.3. 計画の際に決めたこと（2026-10-30）

-   **PR は 2 つに分ける（人間のレビュアーが決めた）:** PR-1 は設計書 8 章の手順 1〜2（パラメータの型と DeepSeek のアダプタ）である。PR-2 は、手順 3（実際の API での確認とその記録）と手順 4（設定の読み込み・`provider`・`cmd/` のテスト・文書）である。理由: 手順 1〜2 は、effort がゼロ値なら今と同じ要求を送るので、振る舞いを変えない。そのため、小さく独立してレビューできる。手順 3 で問題が見つかっても、PR-1 はそのまま使える。手順 4 はこれ以上分けられない。`ClaudeEffortEnvForTest` を消すと文書のガード（`cmd/yt2column/docs_test.go:470`・`:533`）がコンパイルできなくなり、設計書 8 章は文書を同じ PR に入れると決めているためである。
-   **実際の API での確認は、コミットしない一時的な Go テストで行う（人間のレビュアーが決めた。設計書 7.6 が実装計画に委ねた手段）:** `integration` タグの一時的なテストを `internal/llm/deepseek` に置く。設定は既存の `TestIntegrationGenerate`（`internal/llm/deepseek/integration_test.go:35-`）の `deepseektestutil.SettingsFrom` で読む。アダプタ経由で 4 つの値を送り、`max` は `testdata/` の実データの字幕から作った長いプロンプトでも送る。`none` の 1 回だけは、同じテストから生の HTTP 要求を送り、応答の `reasoning_content` を見る。今のアダプタは `reasoning_content` を捨てる（`internal/llm/deepseek/response.go:13-16`）ので、アダプタ経由では thinking が無効になったかが分からないためである。呼び出しは約 6 回である。理由: 手順 4 を書く前に確かめられ、問題があっても手順 4 をやり直さずに済む。

### 1.4. 既存コード調査結果

**設定の読み込み（`internal/config`）**

| 項目 | 今あるもの | 変えること |
|---|---|---|
| 工程の選択 | 非公開の `llmSelection`（`config.go:157-160`）。プロバイダとモデル ID だけを持つ | 公開の `LLMSelection` にし、effort を加える（設計書 3.1）。名前の変更は 1.5 の台帳に記す |
| 値の解析 | `parseLLM`（`config.go:509-530`）。最初の `/` で区切るだけ | 設計書 3.2 の手順 4〜7 を加える |
| Claude の effort | `claudeEffortEnv`（`:36`）、`loadClaudeEffort`（`:598-617`）、`reasonClaudeEffort`（`:84`）、`LLMConfig.claudeEffort`（`:170`）と `ClaudeEffort()`（`:215-220`） | すべて消す（設計書 3.1・3.4） |
| 平たいアクセサ | `Provider()`・`Model()`・`ReviewProvider()`・`ReviewModel()`（`:179-201`） | 消し、`Generation()`・`Review()` にする（設計書 1.2） |
| 旧変数 | `retiredLLMVars`（`:113-118`）、`retiredLLMErrors`（`:478-487`） | 一覧の末尾に `YT2COLUMN_CLAUDE_EFFORT` を加える。`retiredLLMErrors` は変えない |
| テスト用の公開 | `ClaudeEffortEnvForTest`（`test_helpers.go:68-69`）、`RetiredLLMVarsForTest`・`RetiredLLMVarErrorForTest`（`:77-98`） | `ClaudeEffortEnvForTest` を消す。ほかは変えない |

**旧変数の一覧を回す既存のテスト（`YT2COLUMN_CLAUDE_EFFORT` が自動で対象になるもの）**

| テスト | 確かめていること | 本書の扱い |
|---|---|---|
| `internal/config/config_test.go::TestRetiredLLMVars`（`:1722-1726`） | `retiredLLMVars` が、テストに書き写した一覧 `retiredLLMVarsWant`（`:1712-1717`）と等しいこと | `retiredLLMVarsWant` に 1 行を加える（加えないと失敗する） |
| `internal/config/config_test.go::TestLoadRejectsRetiredLLMVars`（`:1733-1775`） | 旧変数が、値ありでも空でも、`LoadFor` の 4 つの `Need` と `LoadServer` を止めること | 変えない。`retiredLLMVarsWant` を回すので自動で対象になる（AC-21・AC-22・AC-24） |
| `internal/config/socketpath_test.go::TestLoadHost`（`:166-`） | 旧変数をすべて設定しても `LoadHost` が `HOME` だけを読むこと（`withRetiredLLMVars`。`:150-158`） | 変えない（AC-26） |
| `cmd/yt2column/run_test.go::TestRunRejectsRetiredLLMVars`（`:2682-2776`） | `--no-review`・`--after review` などの各行の実行が終了コード `2` で止まり、旧変数の行を出し、LLM のクライアントを作らず、yt-dlp も起動しないこと。`-h` と使い方の誤りは設定より先に報告すること | 変えない。基本の環境は DeepSeek なので、両方の工程が DeepSeek の実行でも止まることも含めて確かめられる（AC-21・AC-23） |
| `cmd/yt2column-nmhost/main_test.go` の `hostEnvs`（`:139-`）を使う `TestRelayUsesDerivedSocket`・`TestInstallAndUninstall` | 旧変数を設定してもホストが止まらないこと | 変えない（AC-26） |
| `cmd/yt2column/makefile_test.go::TestMakeTargetsDoNotReadRetiredModelEnv`（`:115-`）・`TestRetiredModelEnvIsGone`（`:184-`） | DeepSeek の make の対象が旧変数からモデルを取らないこと。Makefile・`internal/llm/deepseek` の Go コード・`cmd/yt2column` の結合テストのファイルが旧変数の名前を含まないこと | 変えない。`Makefile` の `YT2COLUMN_TEST_CLAUDE_EFFORT` は、名前の境界で区切る照合（`retiredLLMEnvRE`。`:152-158`）に当たらない。`cmd/yt2column/integration_env_test.go:76` の文字列は、このテストのために消す（ステップ 4-4） |
| `cmd/yt2column-server/run_test.go::TestStartupFailures`（`:83-86` の行） | 一覧の先頭の旧変数で、サーバが待ち受ける前に止まること | 一覧の末尾（`YT2COLUMN_CLAUDE_EFFORT`）の行を加える（AC-22） |

**`YT2COLUMN_CLAUDE_EFFORT` と消すアクセサを使うテスト（コンパイルが止まるので、消せばコンパイラがすべて示す）**

-   `internal/config/config_test.go`: 共通の環境 `validClaudeEnv`（`:43-`）と `defaultsBaseEnv`、および設計書 3.6 の表が挙げる 14 のテスト。`awk` で `claudeEffortEnv`・`ClaudeEffort()` を含む関数を数えると、設計書の一覧と一致した。
-   `internal/llm/provider/provider_test.go`: `claudeValidEnv`（`:99-106`）、`TestNewClaudeSendsConfiguredRequest`・`TestNewClaudeDefaultEffort`・`TestNewDeepSeekIgnoresClaudeVars`・`TestNewReviewUsesReviewModel`・`claudeRequestFor`、および `newClient` を直接呼ぶ `TestNewDeepSeekSendsConfiguredRequest`（`:187`）・`TestNewUnknownProvider`（`:241`）・`:505`。工程を選ぶ補助 `stageSelector`・`generationStage`・`reviewStage`（`:310-320`）は、`newGeneration`・`newReview` を使う形（設計書 3.5）にすると要らなくなる。
-   `cmd/yt2column/run_test.go`: `executionPathRows` の `:635`、`TestRunClaudeConfigurationErrorsOneLineEach`（`:1980-1997`。2 つ目の拒否を `YT2COLUMN_CLAUDE_EFFORT="High"` で作っている）、`TestRunDefaultLLMConfig`（`:2002-2025`）。
-   `cmd/yt2column-server/run_test.go::TestOutputsOmitSecrets`（`:333`）、`cmd/yt2column-server/run.go:188-189`。
-   `cmd/yt2column/docs_test.go`: `checkExportBlockMatchesDefaults`（`:427-460`）、`productionProfileVars`（`:470-475`）、`checkProfileTable`（`:485-560`）、`TestMigrationDocsContract`（`:609-667`）。
-   `cmd/yt2column/integration_env_test.go:59-68`・`:76`。

**文書のガード（`cmd/yt2column/docs_test.go`）で、設計書 7.4 が挙げていないもの**

-   `exportLineRE`（`:361`）は、設定のブロックの値に英数字と `.`・`_`・`-`・`/` だけを許す。このままでは `export YT2COLUMN_LLM=claude/claude-opus-5-5+high` の行に一致せず、本番の構成と移行の節のガードが失敗する。値の文字に `+` を加え、直前のコメント（`:358-360`）も直す（ステップ 4-5）。設計の変更ではなく、設計書 7.4 のガードを動かすための実装の細部である。
-   `TestLLMDefaultsDocsContract`（`:272-`）は、README の表を列名 `Variable` で、`project_overview.md` の表を列名 `変数` で、最初に見つかった表として探す（`doctestutil.FindTable`）。新しい effort の表（設計書 3.7）にこの列名を使うと、別の表を拾う。effort の表の列名は、これらと重ならないものにする。

**DeepSeek のアダプタ（`internal/llm/deepseek`）**

| 項目 | 今あるもの | 変えること |
|---|---|---|
| 構築 | `Options`（`deepseek.go:32-36`）は `APIKey`・`Model`・`Timeout` だけ。`New`（`:51-75`）が検査する | `Effort` を加え、範囲外の値を拒否する（設計書 3.5） |
| 要求本文 | `chatRequest`（`request.go:20-28`）、`newChatRequestBody(model, req)`（`:40-58`） | `reasoning_effort` を加える。effort を引数で受け取る |
| HTTP の状態のエラー | `statusError(status, maxTokens)`（`deepseek.go:131-137`）。モデルと `max_tokens` を示す | `reasoning_effort` も示す |
| 応答の解釈 | `parseResponse(data)`（`response.go:55`）。テストは `Generate` 経由で呼ぶ（`response_test.go` に直接の呼び出しはない） | 送った要求の要約を受け取り、打ち切り・想定外の終了理由・空の応答のエラーに `reasoning_effort` を加える。Claude の `requestSummary`（`internal/llm/claude/response.go:69-80`）と同じ形 |
| テスト補助 | `NewForLoopbackTest`（`test_helpers_endpoint.go:17-34`）は `New` と同じ検査をする。`test_helpers.go:56` などで `Options` を組み立てる | 変えない。`Effort` を省いた `Options` はゼロ値（指定なし）になり、今と同じ要求を送る |

**パラメータのパッケージと import の制約**

-   `claudeparam`（`internal/llm/claudeparam/claudeparam.go`）の値は、`effortLow` などの非公開の定数（`:26-34`）に 1 回だけ書かれている。`EffortValues` はこの定数から作る。
-   `TestConfigImports`（`internal/pipeline/pipeline_test.go:879-922`）は、`internal/config` と `claudeparam` のファイルだけを調べ、`claudeparam` 以外の `internal/llm/` の import を禁じる。`deepseekparam` を加え、許す（ステップ 1-3）。
-   `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:672-733`）は、本体のコードを持つパッケージに `package_reference.md` の行がないと失敗する。`deepseekparam` の行はフェーズ 1 で加える。
-   `.golangci.yml` の `depguard` は、モジュールの中の import を制限しない（`:53-59`）。
-   `make deadcode` は、テストも到達の起点に含める（`CLAUDE.md`「Code Quality」）。PR-1 の時点では `deepseekparam.ParseEffort` と `claudeparam.EffortValues` を本番のコードから呼ばないが、テストから呼ぶので報告されない。PR-1 の完了の条件で `make deadcode` を走らせて確かめる。

**construction の経路（mkplan 手順 5 の「具体的な型の構築と変更の経路」）**

`LLMSelection` は、フィールドが非公開の型を `internal/config` に置く（設計書 3.1）。同じパッケージの中でこれを作るのは `parseLLM` と、継承の写し（`loadLLM`。`config.go:465-469`）だけである。`provider` は `LLMSelection` を受け取るだけで、作らない（ゼロ値の `LLMSelection{}` はテストが未知のプロバイダの枝を通すために使う）。型で分岐する設計ではないので、葉のパッケージへ移すことは要らない。本書は型の置き場所を変えない。

**`implementation_handoff.md`**

このタスクにはない。`design_handoff.md` の H-01〜H-09 は、すべて設計書の付録 A に記録されている。

### 1.5. 名前の変更と削除の台帳

本書で名前を変える、または消すものは次のとおりである。ほかの節は、この表に従う。

| 種類 | 変更前 | 変更後 |
|---|---|---|
| 型 | `config.llmSelection` | `config.LLMSelection` |
| アクセサ | `LLMConfig.Provider()` | `LLMConfig.Generation().Provider()` |
| アクセサ | `LLMConfig.Model()` | `LLMConfig.Generation().Model()` |
| アクセサ | `LLMConfig.ReviewProvider()` | `LLMConfig.Review().Provider()` |
| アクセサ | `LLMConfig.ReviewModel()` | `LLMConfig.Review().Model()` |
| アクセサ | `LLMConfig.ClaudeEffort()` | `LLMConfig.Generation().ClaudeEffort()` または `LLMConfig.Review().ClaudeEffort()`（工程による） |
| 定数 | `claudeEffortEnv` | 消す（旧変数の一覧に名前を直接書く） |
| 関数 | `loadClaudeEffort` | 消す |
| 理由 | `reasonClaudeEffort` | 消す |
| テスト補助 | `config.ClaudeEffortEnvForTest` | 消す（旧変数の名前は `RetiredLLMVarsForTest` から得る） |
| テスト補助 | `provider_test.go` の `stageSelector`・`generationStage`・`reviewStage` | 消す（`newGeneration`・`newReview` を呼ぶ） |
| 関数（新規） | - | `provider.newGeneration`・`provider.newReview`（設計書 3.5） |

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: パラメータの型（設計書 8 章の手順 1）

#### ステップ 1-1: `internal/llm/deepseekparam` を加える

**変更するファイル:** `internal/llm/deepseekparam/deepseekparam.go`（新規）、`internal/llm/deepseekparam/deepseekparam_test.go`（新規）

-   [ ] 設計書 3.1 の `Effort`・定数・`ParseEffort`・`String`・`Valid`・`EffortValues` を書く。値の文字列は非公開の定数に 1 回だけ書き、`ParseEffort`・`String`・`EffortValues` がそれを使う（`claudeparam` と同じ形）。
-   [ ] `ParseEffort` のエラーは、`claudeparam` と同じく非公開の番兵で返す。
-   [ ] テストを書く（設計書 7.2）。一覧の各値の往復（`ParseEffort` → `String`）、`Valid` が真のすべての列挙値の文字列が一覧にあること（両方向）、`EffortUnspecified` の `Valid` が偽で `String` が `ParseEffort` の拒否する値であること、大文字・前後の空白・空・`medium`・`xhigh`・`ultra`・`minimal` の拒否。

**完了の条件:** `go test -tags test ./internal/llm/deepseekparam` が通る。

#### ステップ 1-2: `claudeparam.EffortValues` を加える

**変更するファイル:** `internal/llm/claudeparam/claudeparam.go`、`internal/llm/claudeparam/claudeparam_test.go`

-   [ ] `EffortValues` を加える（設計書 3.1）。`effortLow` などの既存の定数から作る。
-   [ ] ステップ 1-1 と同じ両方向の一致のテストを加える。既存の `TestParseEffort`・`TestEffortStringAndValid` の行と重なる確認は加えない。

**完了の条件:** `go test -tags test ./internal/llm/claudeparam` が通る。

#### ステップ 1-3: import のガードとパッケージの一覧を直す

**変更するファイル:** `internal/pipeline/pipeline_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] `TestConfigImports` の調べるファイルに `../llm/deepseekparam/*.go` を加え、`deepseekparam` の import を許す。テストのコメントと失敗の文を合わせる（設計書 3.6）。
-   [ ] `package_reference.md` に `internal/llm/deepseekparam` の行を加え、`internal/llm/claudeparam` の行に `EffortValues` を加える。`internal/config`・`internal/llm/provider`・`internal/llm/deepseek` の行はフェーズ 2 とフェーズ 4 で直す。
-   [ ] `deepseekparam` のファイルに `net/http` の import を一時的に書き加え、`TestConfigImports` が失敗することを確かめる（コミットしない）。

**完了の条件:** `go test -tags test ./internal/pipeline` が通る。

### フェーズ 2: DeepSeek のアダプタ（設計書 8 章の手順 2）

#### ステップ 2-1: `Options.Effort` と構築の検査

**変更するファイル:** `internal/llm/deepseek/deepseek.go`、`internal/llm/deepseek/errors.go`、`internal/llm/deepseek/deepseek_test.go`

-   [ ] `Options` に `Effort deepseekparam.Effort` を加え、`client` に保持する（設計書 3.5）。`Options` のコメントを設計書 3.5 の形に直す。
-   [ ] `New` は、`Effort` がゼロ値でも `Valid()` が真でもない値を、新しい非公開の番兵で拒否する。番兵は `errors.go` の既存の番兵に並べる。
-   [ ] `TestNew` の表に、4 つの値とゼロ値の受理、範囲外の値（`deepseekparam.Effort(99)` など）の拒否を加える。`TestGenerateSentinelsDistinct`（`deepseek_test.go:701-`）が番兵の一覧を持っていれば、新しい番兵を加える。

#### ステップ 2-2: 要求本文の `reasoning_effort`

**変更するファイル:** `internal/llm/deepseek/request.go`、`internal/llm/deepseek/deepseek.go`、`internal/llm/deepseek/deepseek_test.go`

-   [ ] `chatRequest` に `reasoning_effort` のフィールドを加える。effort が `Valid()` なら `String()` の値を入れ、ゼロ値なら本文に現れない。どちらでもない値なら、本文を作らずにエラーを返す（設計書 3.5）。`thinking` は加えない。
-   [ ] `chatRequest` のコメント（`request.go:20-22`）を、`reasoning_effort` だけを例外として持つ形に直す。
-   [ ] `TestGenerateSendsRequest`（`deepseek_test.go:216-`）に、effort を指定しないクライアントの本文に `reasoning_effort` がないことを加える。`thinking` がないことの確認（`:279-280`）は残す。
-   [ ] 4 つの値のそれぞれで、`httptest` のサーバが受けた本文の `reasoning_effort` がその値であり、`thinking` がないことを確かめるテストを加える（AC-12）。
-   [ ] 範囲外の値で要求本文の組み立てが失敗し、何も送らないことを確かめる。`New` はこの値を通さないので、同じパッケージのテストから要求本文を組み立てる関数を直接呼ぶ。

#### ステップ 2-3: エラーに送った `reasoning_effort` を示す

**変更するファイル:** `internal/llm/deepseek/deepseek.go`、`internal/llm/deepseek/response.go`、`internal/llm/deepseek/deepseek_test.go`、`internal/llm/deepseek/response_test.go`

-   [ ] `statusError` に、送った `reasoning_effort`（送らなかったときはそのこと）を加える（設計書 1.2・3.5）。今のモデルと `max_tokens` の表示は変えない。
-   [ ] `parseResponse` が送った要求の要約を受け取り、打ち切り・想定外の終了理由・空の応答のエラーに `reasoning_effort` を加える。要約の型は Claude の `requestSummary` と同じ形にする。応答の解釈の規則は変えない（設計書 3.5）。
-   [ ] `TestGenerateHTTPStatus`（`deepseek_test.go:424-`）と、`response_test.go` の打ち切り・終了理由・空の応答のテスト（`TestGenerateFinishReason`・`TestGenerateEmptyContent`）に、effort を指定したときと指定しないときのエラーの表示の確認を加える。

#### ステップ 2-4: 文書の行を直す

**変更するファイル:** `docs/dev/conventions/package_reference.md`

-   [ ] `internal/llm/deepseek` の行に、`Options.Effort`、`reasoning_effort` を指定があるときだけ送ること、エラーに送った値を示すことを加える（設計書 3.7）。

**フェーズ 2 の完了の条件:** `make fmt`、`make test`、`make lint`、`make deadcode` が通る。effort を指定しないクライアントの要求本文が今と同じであることを `TestGenerateSendsRequest` が確かめている。

### PR-1 作成ポイント: param types and DeepSeek reasoning_effort

**対象ステップ**: 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `feat(0051): DeepSeek のアダプタが reasoning_effort を送れるようにし、effort の値の一覧を置く`

**レビュー観点**: effort を指定しないクライアントの要求本文が今と同じであること / 範囲外の effort を構築と要求本文の組み立ての両方で拒否すること / エラーの表示が秘密情報を含まないこと / `TestConfigImports` の対象と許可の追加

**実装モデル要件**: standard

**判定理由**: 振る舞いを変えない追加であり、既存コード調査結果に競合する案はなく、Conditional checks にも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 実際の API での確認（設計書 8 章の手順 3、設計書 7.6）

#### ステップ 3-1: DeepSeek の `reasoning_effort` を実際の API で確かめる

**変更するファイル:** なし（一時的なテストはコミットしない）。結果は本書のこのステップに記録する。

-   [ ] 実際の API を呼ぶ前に、呼び出しの回数と内容を示して人間の承認を得る（`CLAUDE.md`「Tool Execution Safety」）。
-   [ ] 1.3 の手段で、設計書 7.6 の 3 点を確かめる。モデルは `deepseek-flash` とする。
    -   4 つの値（`none`・`low`・`high`・`max`）のそれぞれで、API が `200` を返し、アダプタが応答を受理すること。
    -   `none` の生の応答に `reasoning_content` がないか空であること。
    -   `max` を `testdata/` の実データの字幕から作ったプロンプトで送ったとき、`provider.LLMTimeout` の中で終わり、`max_tokens` を送らない要求で打ち切りにならないこと。
-   [ ] 一時的なテストのファイルを消し、作業ツリーに残っていないことを `git status` で確かめる。
-   [ ] 結果（日付、各値の状態コードと受理の可否、`none` の `reasoning_content` の有無、`max` の所要時間と `finish_reason`）を、このステップの下に記録する。API キーとそこから作った値は記録しない。
-   [ ] 受け付けられない値や、`max` での時間切れ・打ち切りがあれば、フェーズ 4 に進まず、要件の変更として人間に諮る（設計書 7.6）。

**完了の条件:** 3 点がすべて満たされたことが記録されている。または、人間が判断した結果が記録されている。

### フェーズ 4: 設定の読み込み・`provider`・文書（設計書 8 章の手順 4）

このフェーズの変更は 1 つの PR に入れる（1.3）。コンパイルが通る単位で、ステップごとにコミットしてよい。

#### ステップ 4-1: 設定の読み込みを変える

**変更するファイル:** `internal/config/config.go`、`internal/config/test_helpers.go`

-   [ ] 1.5 の台帳のとおり、`llmSelection` を `LLMSelection` にし、`claudeEffort`・`deepSeekEffort` のフィールドとアクセサを加える。`LLMConfig` に `Generation()`・`Review()` を加え、台帳のアクセサとフィールド `claudeEffort` を消す（設計書 3.1）。
-   [ ] `parseLLM` に設計書 3.2 の手順 4〜7 を加える。effort の部分の検査はプロバイダの `switch` で選び、`default` は拒否にする。変換の関数のエラーは捨てる。
-   [ ] 理由を設計書 4.1 の表のとおりに加え、`reasonLLMForm` を直し、`reasonClaudeEffort` を消す。一覧を含む 2 つの理由は、パラメータのパッケージの `EffortValues` から組み立てるパッケージの変数にする。`reasonRetiredClaudeEffort` の既定値と既定の effort は `defaultLLMValue` と `defaultClaudeEffort.String()` から作る。
-   [ ] `retiredLLMVars` の末尾に `{"YT2COLUMN_CLAUDE_EFFORT", reasonRetiredClaudeEffort}` を加える。`claudeEffortEnv`・`loadClaudeEffort` を消し、`loadProviderVars` から呼び出しを消す（設計書 3.4）。
-   [ ] `ClaudeEffortEnvForTest` を消す（`test_helpers.go:68-69`）。
-   [ ] 設計書 3.6 の表が挙げるコメントを直す: 理由の定数の前置き（`:77-79`）、`retiredLLMVar`（`:103-108`）、`llmSelection` のコメント（`:151-156`）、`parseLLM`（`:500-508`）、`defaultClaudeEffort`（`:592-595`）、`loadLLM`（`:441-442` の「generation accessors」）、`LoadFor`・`LoadServer` の旧変数の例外の説明（`:319-322`・`:409-412`）。

#### ステップ 4-2: 設定の読み込みのテスト

**変更するファイル:** `internal/config/config_test.go`

-   [ ] 共通の環境 `validClaudeEnv`・`defaultsBaseEnv` から `claudeEffortEnv` を消し、設計書 3.6 の表が挙げる 14 のテストを追従させる。`YT2COLUMN_CLAUDE_EFFORT` で effort を与えていた行は、LLM の変数の `+<effort>` に置き換える。DeepSeek だけの実行で `YT2COLUMN_CLAUDE_EFFORT` の不正な値を無視することを確かめていた行は消す（設計書 1.4 (2)）。`TestLoadFor` の部分テスト「no stage reads no LLM variable」の扱いは設計書 1.4 (2) のとおり。
-   [ ] 台帳のアクセサの呼び出しを、`Generation()`・`Review()` の値のアクセサに置き換える。`TestLoadParsesLLMValue` の工程の選び方（`:1630` の `reviewLLMEnv` の行）も同じく置き換える。
-   [ ] `retiredLLMVarsWant`（`:1712-1717`）に `YT2COLUMN_CLAUDE_EFFORT` の行を加える。
-   [ ] `TestLoadParsesLLMValue` の表に、AC-01〜AC-07・AC-09 の値の行を加え、工程の選択の `ClaudeEffort`・`DeepSeekEffort` も比べる（設計書 7.1）。表は両方の LLM の変数について回るので、AC-10 もこの表で確かめる。拒否の行は、AC-05 と AC-07 の値をすべて含め、`Reason` と番兵を確かめる。`+` を含まない既存の行は変えない（AC-29）。
-   [ ] effort の理由の一覧を、固定の前置きの後ろから取り出して区切り文字で分け、`claudeparam.EffortValues()`・`deepseekparam.EffortValues()` の並びと比べるテストを加える（設計書 7.1、AC-41）。部分文字列で探さない。
-   [ ] 理由の文が設計書 4.1 の表の内容を含むことを確かめる（AC-06・AC-24）。`reasonRetiredClaudeEffort` は、2 つの LLM の変数の名前、`defaultLLMValue`、`defaultClaudeEffort.String()` を含むことを、コードの値から作った文字列で確かめる。
-   [ ] 継承のテストを加える（AC-15・AC-16・AC-19・AC-20）。`LoadFor` の `Need` の組み合わせごとに、検証・推敲の工程の選択の effort を確かめる。AC-19 は、`YT2COLUMN_LLM` の拒否が 1 件だけであることを確かめる。
-   [ ] `TestLoadProviderVarsFollowSelection`（`:1777-`）に、effort の部分が不正な値と `+` が 2 つ以上の値の行を加え、そのプロバイダの変数を読むことを確かめる（AC-28）。
-   [ ] `TestLoadReportsAllInvalid`（`:807-`）に、`YT2COLUMN_CLAUDE_EFFORT` とほかの拒否を合わせて報告する行を加える（AC-25）。
-   [ ] `TestLoadErrorsOmitValues`（`:717-`）に、effort の部分とモデル ID の部分に目印を埋め込んだ値を、2 つの LLM の変数と 2 つのプロバイダについて加える（設計書 7.5）。

**ステップ 4-1・4-2 の完了の条件:** `go test -tags test ./internal/config` が通る。

#### ステップ 4-3: `provider` を変える

**変更するファイル:** `internal/llm/provider/provider.go`、`internal/llm/provider/provider_test.go`

-   [ ] `newGeneration`・`newReview` を加え、`newClient` が工程の選択を受け取る形にする。`New`・`NewReview` は本番の構築関数を `builders` に入れて、それぞれを呼ぶだけにする（設計書 3.5）。Claude の枝は `sel.ClaudeEffort()` を、DeepSeek の枝は `sel.DeepSeekEffort()` を渡す。
-   [ ] `New`・`NewReview`・`newClient` のコメント（`provider.go:31`・`:38-39`・`:52-58`）を直す。
-   [ ] `claudeValidEnv` から `YT2COLUMN_CLAUDE_EFFORT` を消す。`stageSelector`・`generationStage`・`reviewStage` を消し、`claudeRequestFor` は `newGeneration` か `newReview` を受け取る形にする（1.5 の台帳）。
-   [ ] 設計書 3.6 の表が挙げるテストを追従させる。`TestNewClaudeSendsConfiguredRequest` の 5 つの effort は、`YT2COLUMN_LLM` の `+<effort>` で与える（AC-11）。`TestNewClaudeDefaultEffort` は、effort を省いた値で `high` を送ることを確かめる（AC-11・AC-18）。
-   [ ] `TestNewUnknownProvider` は、ゼロ値の `LLMSelection{}` を渡す形にし、`Provider(99)` の行を消す（設計書 3.5）。消した後に `go tool cover -func` の結果が関数ごとに変わらないことを確かめ、コミットメッセージに書く。
-   [ ] 設定の文字列から要求本文までを通すテストを加える（設計書 7.2 の `provider` の項の組み合わせをすべて含める）。設定の文字列を読み込み、ループバックへ送る構築関数を `newGeneration`・`newReview` に渡し、サーバが受けた本文の effort を確かめる。DeepSeek の本文は `reasoning_effort` の有無と値を、Claude の本文は `output_config.effort` を見る。
-   [ ] `TestNewPaddedModel`（`:539-`）に `claude/claude-opus-5-5 +high` の行を加える（AC-09）。

**完了の条件:** `go test -tags test ./internal/llm/provider` が通る。

#### ステップ 4-4: `cmd/` の本体とテスト

**変更するファイル:** `cmd/yt2column-server/run.go`、`cmd/yt2column-server/run_test.go`、`cmd/yt2column/run_test.go`、`cmd/yt2column/integration_env_test.go`

-   [ ] `cmd/yt2column-server/run.go:188-189` を台帳のとおりに置き換える。ログのキーと値は変えない。
-   [ ] `cmd/yt2column-server/run_test.go::TestOutputsOmitSecrets`（`:333`）の `YT2COLUMN_CLAUDE_EFFORT` を、`YT2COLUMN_REVIEW_LLM` の `+low` に置き換える。
-   [ ] `cmd/yt2column-server/run_test.go::TestStartupFailures` に、`config.RetiredLLMVarsForTest()` の末尾の変数を設定した行を加える。待ち受ける前に、終了コード `2` で止まり、その変数の行を出すことを確かめる（AC-22）。
-   [ ] `cmd/yt2column/run_test.go` の `executionPathRows`（`:635`）と `TestRunDefaultLLMConfig`（`:2002-2025`）から `YT2COLUMN_CLAUDE_EFFORT` を消し、`:2019-2022` を台帳のとおりに置き換える。
-   [ ] `TestRunClaudeConfigurationErrorsOneLineEach`（`:1980-1997`）の 2 つ目の拒否を、`YT2COLUMN_LLM` の不正な effort の部分（`claude/fake-model+High`）で作る。
-   [ ] effort の部分が不正な値（`claude/claude-opus-5-5+ultra`）で、CLI が終了コード `2` で止まり、LLM のクライアントを作らないことを確かめるテストを加える（AC-08）。
-   [ ] `cmd/yt2column/integration_env_test.go:76` の一覧から `YT2COLUMN_CLAUDE_EFFORT` を外し、`:59-68` を台帳のとおりに置き換える。このファイルに旧変数の名前の文字列を書かない（1.4、`TestRetiredModelEnvIsGone`）。

**完了の条件:** `go test -tags test ./cmd/...` と `go test -run '^$' -tags integration ./cmd/yt2column` が通る（`integration_env_test.go` は `integration` タグでもコンパイルされるため）。

#### ステップ 4-5: 文書のガード

**変更するファイル:** `cmd/yt2column/docs_test.go`

-   [ ] `exportLineRE`（`:361`）の値の文字に `+` を加え、コメント（`:358-360`）を直す（1.4）。
-   [ ] 本番の構成のガードを、設計書 7.4 の 1 つ目の項のとおりに直す。`productionProfileVars` は `YT2COLUMN_LLM` だけを返す。`checkExportBlockMatchesDefaults` と `checkProfileTable` は、工程の選択をプロバイダ・モデル・`ClaudeEffort`・`DeepSeekEffort` で比べる。表のセルは `<プロバイダ>/<モデル>+<effort>` につないで読み込む。モデルの値を失敗の文に出さない今の形は保つ。
-   [ ] 移行の節のガードを、設計書 7.4 の 2 つ目の項のとおりに直す。変更前の `text` ブロックと続く `sh` ブロックを組にし、組の種類を旧変数で決める。
-   [ ] effort の値の一覧のガードを加える（設計書 7.4 の 3 つ目の項、AC-40）。表の列名は 1.4 のとおり、`TestLLMDefaultsDocsContract` が探す列名と重ならないものにする。
-   [ ] 各ガードについて、文書の値を一時的に壊して失敗することを確かめる（例: 設定のブロックの effort を `low` に、effort の表から 1 つの値を消す、移行の例の変更後の effort を変える）。

#### ステップ 4-6: 文書

**変更するファイル:** `README.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/dev/guides/cli.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/security.md`

-   [ ] 設計書 3.7 の表のとおりに、各文書を直す。`docs/dev/guides/claude_output_limit.md` は変えない（設計書 1.2）。
-   [ ] `README.md` の変更は、その節に関わる AC（AC-30〜AC-36・AC-39・AC-40）を満たすように書く。`README.md` を変えるので、`make ext-test` を走らせる（`CLAUDE.md`「Development Notes」）。
-   [ ] `README.en.md` がこの時点であれば、`/mktrans README.md` で反映する（2026-10-30 の時点ではない）。
-   [ ] `docs/dev/project_overview.md` の DeepSeek の thinking の記述（`:88`）を、AC-37 を満たすように直す。
-   [ ] `package_reference.md` の `internal/config`・`internal/llm/provider` の行を直す（AC-38）。

**フェーズ 4 の完了の条件:** `make fmt`、`make test`、`make lint`、`make deadcode`、`make ext-test` が通る。7 章の検証表の `manual` の行を、PR の説明に確認の項目として載せる。

### PR-2 作成ポイント: effort in LLM variables and retirement of YT2COLUMN_CLAUDE_EFFORT

**対象ステップ**: 3-1 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6

**推奨タイトル**: `feat(0051): YT2COLUMN_LLM の値で effort を工程ごとに指定し、YT2COLUMN_CLAUDE_EFFORT を廃止する`

**レビュー観点**: `parseLLM` の検査の順序と、拒否してもプロバイダを返す契約 / 生成と検証・推敲の effort の取り違えを、設定の文字列から要求本文までのテストが捉えること / 旧変数の追加で止まる実行と、止まらないホスト / 文書の値とガードテストの対応、およびステップ 3-1 の記録

**実装モデル要件**: frontier-required

**判定理由**: ステップ 4-1 は、旧変数を拒否して今の設定の実行を止める一方で新しい値を受理する移行（受理の拡大と縮小の同時の変更）であり、ステップ 3-1 は実際の外部 API を使う

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: パラメータの型とアダプタ | `deepseekparam`、`claudeparam.EffortValues`、DeepSeek のアダプタの `reasoning_effort` | PR-1 がマージされた |
| M2: 実際の API での確認 | ステップ 3-1 の記録 | 3 点が満たされたことが本書に記録された |
| M3: 設定と文書 | `LLMSelection`、`parseLLM` の手順 4〜7、旧変数、`provider`、文書とガード | PR-2 がマージされた |

M2 は M3 の作業より前に終える。M2 で問題があれば、M3 に進む前に人間に諮る（設計書 7.6）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 2-3 / 2-4 | パラメータの型、DeepSeek のアダプタの `reasoning_effort` とエラーの表示、import のガード。振る舞いは変えない | standard |
| PR-2 | 3-1 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 | 実際の API での確認の記録、設定の読み込み、`provider`、`cmd/` のテスト、文書とガード | frontier-required |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。本書は、どのテストで確かめるかを 7 章の表に示す。

-   **ユニットテスト:** LLM の API を呼ばない。DeepSeek と Claude の要求本文は `httptest` のループバックのサーバで受ける。設定は `LookupFunc` で与え、プロセスの環境を変えない。
-   **層をまたぐテスト:** 工程の取り違えは、設定の文字列から要求本文までを通すテスト（ステップ 4-3）で捉える。`newClient` を直接呼ぶテストでは捉えられない（設計書 7.2）。
-   **後方互換:** `+` を含まない値の既存のテストの行は変えずに通す（AC-29）。effort を指定しない DeepSeek の要求本文が今と同じであることを、`TestGenerateSendsRequest` が確かめる（AC-13）。
-   **結合テスト:** 恒久の結合テストに effort を指定する手段は加えない（要件書 2.3）。実際の API での確認はステップ 3-1 だけで行う。
-   **テストを消すとき:** `TestNewUnknownProvider` の `Provider(99)` の行を消した後、`go tool cover -func` の結果が関数ごとに変わらないことを確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| DeepSeek の API が、ある `reasoning_effort` の値を受け付けない | その値の工程が HTTP の状態のエラーで失敗する | ステップ 3-1 で、PR-2 の作業の前に確かめる。問題があれば人間に諮る |
| `max` で時間切れか打ち切りになる | 記事が出ない | ステップ 3-1 で長いプロンプトを使って確かめる |
| DeepSeek の effort が配線の途中で落ちても、構築が失敗しない（ゼロ値が正しい値のため。設計書 5.2） | 利用者の指定と異なる thinking で動く | ステップ 4-3 の、設定の文字列から要求本文までを通すテスト |
| フェーズ 4 の変更が大きく、レビューの見落としが起きる | 文書とコードの食い違い | 文書の値はガードテスト（ステップ 4-5）で確かめ、文の内容は 7 章の `manual` の行を PR の説明に載せて読む |
| 文書のガードが新しい値の形を受け付けない（`exportLineRE`） | `make test` が通らない | ステップ 4-5 の最初に直す |
| 予定の遅れ（ステップ 3-1 の承認待ち） | PR-2 の着手が遅れる | PR-1 のレビューの間に承認を求める |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 2-1 / 2-2 / 2-3 / 2-4）
- [ ] PR-2 マージ済み（対象ステップ: 3-1 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6）

**横断の確認（`make lint` と `make test` では検出できないもの）:**

- [ ] Go のコメントに、消した名前（1.5 の台帳）と `YT2COLUMN_CLAUDE_EFFORT` が、現行の設定や今の API として残っていない。2026-10-30 の時点の所在は、`internal/config/config.go:151`・`:189-198`・`:215-216`・`:592`・`:598-600`、`internal/config/test_helpers.go:68`、`internal/config/config_test.go:357`・`:1573`、`internal/llm/provider/provider.go:31`・`:38-39`、`internal/llm/provider/provider_test.go:349-351`、`cmd/yt2column/docs_test.go:472` である（PR のレビューで読む）。
- [ ] `docs/translation_glossary.md` に、本書で増えた用語（effort の部分など）で表に載せるべきものがないか確かめる（PR のレビューで読む）。

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 確かめ方 | 実装 |
|---|---|---|---|
| AC-01 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`（effort の行）、`internal/llm/provider/provider_test.go` の設定から要求本文までのテスト（ステップ 4-3） | ステップ 4-1・4-3 |
| AC-02 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`、`internal/llm/deepseekparam/deepseekparam_test.go`、`internal/llm/claudeparam/claudeparam_test.go` | ステップ 1-1・1-2・4-1 |
| AC-03 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`、`internal/llm/provider/provider_test.go::TestNewClaudeDefaultEffort` | ステップ 4-1・4-3 |
| AC-04 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`（`claude/vendor/model-x+medium`） | ステップ 4-1 |
| AC-05 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`（拒否の行）、`TestLoadErrorsOmitValues` | ステップ 4-1 |
| AC-06 | test | `internal/config/config_test.go` の理由の文のテスト（ステップ 4-2）と `TestLoadParsesLLMValue` の `Reason` | ステップ 4-1 |
| AC-07 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`（`claude/+`・`claude/+ultra`・`deepseek/deepseek-flash+a+b`） | ステップ 4-1 |
| AC-08 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`、`cmd/yt2column/run_test.go` の不正な effort の部分のテスト（ステップ 4-4） | ステップ 4-1 |
| AC-09 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`、`internal/llm/provider/provider_test.go::TestNewPaddedModel` | ステップ 4-1・4-3 |
| AC-10 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`（両方の LLM の変数を回す） | ステップ 4-1 |
| AC-11 | test | `internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`・`TestNewClaudeDefaultEffort` | ステップ 4-3 |
| AC-12 | test | `internal/llm/deepseek/deepseek_test.go` の `reasoning_effort` のテスト（ステップ 2-2）、`internal/llm/provider/provider_test.go` の設定から要求本文までのテスト | ステップ 2-2・4-3 |
| AC-13 | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateSendsRequest`、`internal/llm/provider/provider_test.go` の設定から要求本文までのテスト | ステップ 2-2・4-3 |
| AC-14 | test | `internal/llm/provider/provider_test.go` の設定から要求本文までのテスト（工程ごとに effort が異なる組） | ステップ 4-3 |
| AC-15 | test | `internal/config/config_test.go` の継承のテスト（ステップ 4-2）、`internal/llm/provider/provider_test.go` の受け継ぐ組 | ステップ 4-1・4-3 |
| AC-16 | test | `internal/config/config_test.go` の継承のテスト | ステップ 4-1 |
| AC-17 | test | `internal/llm/provider/provider_test.go` の設定から要求本文までのテスト（生成のクライアントの effort） | ステップ 4-3 |
| AC-18 | test | `internal/config/config_test.go::TestLoadProductionDefaults`、`internal/llm/provider/provider_test.go::TestNewClaudeDefaultEffort` | ステップ 4-2・4-3 |
| AC-19 | test | `internal/config/config_test.go` の継承のテスト（拒否が 1 件） | ステップ 4-1 |
| AC-20 | test | `internal/config/config_test.go` の継承のテスト（`Need` が検証・推敲だけ） | ステップ 4-1 |
| AC-21 | test | `cmd/yt2column/run_test.go::TestRunRejectsRetiredLLMVars`、`internal/config/config_test.go::TestLoadRejectsRetiredLLMVars`・`TestRetiredLLMVars` | ステップ 4-1・4-2 |
| AC-22 | test | `cmd/yt2column-server/run_test.go::TestStartupFailures`、`internal/config/config_test.go::TestLoadRejectsRetiredLLMVars`（`LoadServer`） | ステップ 4-1・4-4 |
| AC-23 | test | `cmd/yt2column/run_test.go::TestRunRejectsRetiredLLMVars`（クライアントを作らない） | ステップ 4-1 |
| AC-24 | test | `internal/config/config_test.go` の理由の文のテスト、`TestLoadRejectsRetiredLLMVars`、`TestLoadErrorsOmitValues` | ステップ 4-1 |
| AC-25 | test | `internal/config/config_test.go::TestLoadReportsAllInvalid` | ステップ 4-2 |
| AC-26 | test | `cmd/yt2column-nmhost/main_test.go::TestRelayUsesDerivedSocket`・`TestInstallAndUninstall`、`internal/config/socketpath_test.go::TestLoadHost` | ステップ 4-1（テストは変えない） |
| AC-27 | test | `internal/config/config_test.go::TestLoadIgnoresOtherProviderVars`・`TestLoadProviderVarsFollowSelection` | ステップ 4-1・4-2 |
| AC-28 | test | `internal/config/config_test.go::TestLoadProviderVarsFollowSelection`（effort の行） | ステップ 4-1 |
| AC-29 | test | `internal/config/config_test.go::TestLoadParsesLLMValue`（`+` を含まない既存の行）・`TestParseLLMProvider` | ステップ 4-1 |
| AC-30 | manual | PR のレビューで、README の設定の節の表と `project_overview.md` の表を読む | ステップ 4-6 |
| AC-31 | manual | PR のレビューで、effort の部分の規則の説明を読む | ステップ 4-6 |
| AC-32 | manual | PR のレビューで、継承の文とプロバイダの変数の検査の文を読む | ステップ 4-6 |
| AC-33 | manual | PR のレビューで、旧変数の列挙と例外の文、`--no-review` の段落、`max` と既定の effort の注記を読む | ステップ 4-6 |
| AC-34 | test・manual | `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`（設定のブロックが `YT2COLUMN_LLM` の 1 行で、既定の工程の選択と一致する）。変数の数と effort の記述の文は PR のレビューで読む | ステップ 4-5・4-6 |
| AC-35 | test・manual | `cmd/yt2column/docs_test.go::TestMigrationDocsContract`（`YT2COLUMN_CLAUDE_EFFORT` の組の値）。`YT2COLUMN_REVIEW_LLM` についての文は PR のレビューで読む | ステップ 4-5・4-6 |
| AC-36 | manual | PR のレビューで、README のトラブルシューティングの項目を読む | ステップ 4-6 |
| AC-37 | manual | PR のレビューで、`project_overview.md:88` の記述を読む | ステップ 4-6 |
| AC-38 | manual | PR のレビューで、`package_reference.md` の行と `claude_output_limit.md:10` を読む。`TestPackageReferenceListsPackages` が `deepseekparam` の行の存在を確かめる | ステップ 1-3・2-4・4-6 |
| AC-39 | manual | PR のレビューで、6 つの文書が `YT2COLUMN_CLAUDE_EFFORT` を拒否と移行の説明でだけ扱うことを読む | ステップ 4-6 |
| AC-40 | test | `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`・`TestLLMDefaultsDocsContract`・effort の値の一覧のガード（ステップ 4-5） | ステップ 4-5・4-6 |
| AC-41 | test | `internal/config/config_test.go` の理由の一覧のテスト（ステップ 4-2）、`internal/llm/deepseekparam/deepseekparam_test.go`・`internal/llm/claudeparam/claudeparam_test.go` の一覧のテスト | ステップ 1-1・1-2・4-1 |

新しく加えるテストの関数名は実装で決める。実装した後、この表の「ステップ 4-2」などの記述を `path::TestName` の形に書き換える。

## 8. 成功基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-41 が、7 章の確かめ方で満たされている。
-   **品質:** `make test`・`make lint`・`make deadcode`・`make ext-test` が通る。加えたテストは、対象を壊して失敗することを確かめ、コミットメッセージに記録している。
-   **セキュリティ:** 拒否のメッセージが設定した値を含まないこと（`TestLoadErrorsOmitValues`）、DeepSeek の要求に列挙値だけが入ること（ステップ 2-2 のテスト）、拒否した実行が LLM に何も送らないこと（`TestRunRejectsRetiredLLMVars` とステップ 4-4 のテスト）を確かめている。
-   **文書:** 7 章の `manual` の行を、PR-2 のレビューで読んでいる。ステップ 3-1 の結果が本書に記録されている。

## 9. 次のステップ (Next Steps)

-   PR-1 を `/runplan 0051` で実装する。
-   PR-1 のマージの後、ステップ 3-1 の実際の API での確認の承認を求め、PR-2 を `/runplan 0051` で実装する。
-   PR-2 のマージの後、issue #566 を閉じる（`/runpr` が提案する）。
