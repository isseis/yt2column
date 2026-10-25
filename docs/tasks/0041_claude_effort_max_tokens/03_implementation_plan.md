# 実装計画書：Claude の effort の既定を high にし、出力の上限 16,000 を緩和する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-25 |
| Review date | 2026-10-25 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「要件書」）の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下「設計書」）の節を指し、本計画自身の節は「本計画 §N」と書く。次の語は、要件書 6 章と設計書の冒頭で定めた意味で使う（effort、出力の上限、V、段階、本番の構成、打ち切り、既定の effort、既定の上限、最も遅い出力の速さ、上限のガイド）。既存のファイルの行番号は、断りがない限りコミット `68f4cb9`（`main`）で確かめた。設計書が行番号を確かめたコミット `d92286a` から `68f4cb9` までに、`internal/`・`cmd/`・`README.md`・`docs/dev/`・`CLAUDE.md` の変更はない（`git diff --stat d92286a 68f4cb9 -- internal cmd README.md docs/dev CLAUDE.md` の出力は空だった）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`YT2COLUMN_CLAUDE_EFFORT` が未設定のとき、effort を `high` にする（F-001）。Claude アダプタの既定の上限を 16,000 から V（40,000）に上げる（F-002）。両方の変更を文書に反映し、上限の根拠を測り直す手順を上限のガイドとして新設する（F-003）。

### 1.2. 実装の原則

-   **設計書の置き場所を守る。** 既定の effort は `internal/config` の `defaultClaudeEffort` だけが持ち（設計書 1.2・3.1）、既定の上限は `internal/llm/claude` の `defaultMaxOutputTokens` だけが持つ（設計書 3.2）。`internal/writer` と `internal/llm/deepseek` のコードは変えない（設計書 3.7）。
-   **数値を文書に写さない。** README と上限のガイドは V の値も出力の速さも書かない（設計書 1.1・1.2）。V の (a)・(b) の条件（(a) は観測した出力の最大の 2 倍以上であること、(b) は最も遅い出力の速さで V トークンを出力する時間が `LLMTimeout` の半分以内であること。もう 1 つの条件 (c) は、モデルが V を受け付けること。要件書 F-002・設計書 1.4）は、provider のテストの名前付きの値で固定する（設計書 7.1）。
-   **テストが働くことは、対象を壊して確かめる**（CLAUDE.md「Testing Strategy」）。各フェーズの最後のステップで、実装する人が対象を一時的に壊し、新しいテストが失敗することを見て、コミットのメッセージに書く。
-   **テストを外すときは、カバレッジを比べる**（CLAUDE.md「Deleting a test is a claim that must be checked」）。`TestLoadClaudeMissing` の `"effort unset"` の行を外す前後で、`go tool cover -func` の関数ごとの値を比べる（ステップ 1-2）。

### 1.3. 計画の際に決めたこと（2026-10-25）

人間のレビュアーが決めたもの:

-   **3 つのフェーズを 1 つの PR にする:** 設計書 8 章が実装計画に委ねた判断である。コードと文書を別の PR にすると、コードだけが `main` に入った間、README が `YT2COLUMN_CLAUDE_EFFORT` を必須と書いたままになる。また、タスク 0042（#490）の実装は本タスクの実装が `main` に入った後にマージする（0042 設計書 8 章の 6）ので、PR を分けると 0042 の待ちが延びる。コードの変更は数行で、差分の大部分はテストと上限のガイドなので、1 つの PR でもレビューできる大きさである。

計画で決めたもの（人間のレビュアーに諮らず、理由とともに記す）:

-   **既定の effort の config のテストは、新しいテスト関数 `TestLoadClaudeEffortDefault` にまとめる:** 設計書 7.1 の 3 つの環境（生成と検証・推敲の両方が `claude`、一方だけが `claude` の 2 つ）と `LoadServer` の経路は、どれも「未設定なら `EffortHigh`」という同じ性質を確かめる。既存の `TestLoadClaude` は設定した 5 つの値を確かめるテストなので、性質の違う行を混ぜない。
-   **provider の新しいテストは 2 つの関数に分ける:** 既定の effort の `TestNewClaudeDefaultEffort` と、V の (a)・(b) の `TestClaudeDefaultMaxTokensFitsBudget` である。前者はフェーズ 1、後者はフェーズ 2 の変更を確かめ、壊して確かめる対象も違う（ステップ 1-4・2-5）。
-   **AC-05 の追加の 2 つの組み合わせは、`TestLoadFor` の副テストにする:** 設計書 7.1 のとおりである。`Need` を変える既存の副テスト（`"generation only leaves the review settings unread"` など）と同じ形で書く。
-   **上限のガイドを指す Go のテストは足さない:** `internal/pricing/rates.go` の先頭のコメントが `pricing_update.md` を指すことは、`cmd/yt2column/docs_test.go:531-535` が確かめている。本タスクのガイドを指すのは、3 つのファイル（`config.go`・`request.go`・`provider.go`）のコメントである。パスを誤っても振る舞いは変わらず、PR のレビューで読めば分かる。設計書 3.7 のファイルの表にもこのテストはない。そのため、本計画でも足さない。
-   **テストの名前付きの値のコメントは、出典として設計書 1.4 を挙げてよい:** 設計書 1.1 は「コードのコメントは、タスクの文書ではなく上限のガイドだけを指す」とし、設計書 7.1 は (a)・(b) の名前付きの値の出典（設計書 1.4）をコメントに書くとしている。本計画は、1.1 の規則を本番のコードのコメント（設計書 3.1〜3.3 が直すもの）に当てる規則と読む。テストの値は測定の値そのものなので、その出典の記録を挙げる（ステップ 2-3）。
-   **設計書の 2 か所を編集の訂正として直す:** 計画の作成時に、設計書の 2 か所の誤りが分かった。どちらも決定は変えないので、設計書の `Status` は `approved` のまま、`Comments` に訂正を記録した（`requirements_process.md`「Editing an approved document」）。1 つ目は 3.5「How to measure」の例である。字数の調整が走らなかった実行のあとの検証・推敲は、`--after shorten` では行えない。字数の調整が走らなかった実行には、字数の調整の中間生成物がない（中間生成物は、字数の調整が走ったときだけ書かれる。`internal/writer/writer.go:366-380`）。そのため `--after shorten` の読み込みが失敗する（`internal/artifact/store.go:139-151`）。その場合は `--after generation --no-shorten` を使う（`resumeNeed`、`cmd/yt2column/run.go:481-493`）。2 つ目は 3.4・3.8 の README の段落の呼び名である。`README.md:116-121` は `--after` の段落ではなく、`--no-review` の段落である。

### 1.4. 既存コード調査結果

| 領域 | すでにあるもの（`68f4cb9`） | 足りないもの・変えるもの |
|---|---|---|
| effort の読み込み | `loadClaudeEffort`（`internal/config/config.go:586-601`）。`!ok \|\| value == ""` で `missingVar` を返す（`:592-594`）。呼び出しは `loadProviderVars` の `claude` の判定の中だけ（`:532-540`） | 未設定（`!ok`）のときに `defaultClaudeEffort` を入れて `nil` を返す分岐を、空の値の拒否と分けて足す（設計書 3.1）。呼ばれる条件は変えない |
| effort の doc コメント | `LLMConfig.ClaudeEffort`（`config.go:158-159`）、`loadClaudeEffort`（`:586-589`）。`LoadFor` の doc コメント（`:339-345`）は、真の need について、選んだプロバイダの変数を「reads and requires」と書く | 3 つともステップ 1-1 で書き換える。`LoadFor` の doc コメントの語は、ワークスペース ID がすでに任意であり、effort も任意になるので「reads and validates」に直す |
| effort の読み込みの呼び出し元 | `loadLLM`（`config.go:369-380`）は、`need.Generation` が真のときだけ `loadProvider` を、`need.Review` が真のときだけ `loadReview` を呼ぶ。`Need` が偽の段階のプロバイダは `ProviderUnset` のままである | 変えない。そのため、`loadProviderVars` の `claude` の判定が `need` を見なくなっても、到達できる状態では結果が変わらない。ステップ 1-4 の壊す対象は、この判定以外の箇所から選ぶ |
| 既定の上限 | `defaultMaxOutputTokens = 16000`（`internal/llm/claude/request.go:23-26`）。`effectiveMaxTokens`（`:66-75`）が 0 をこの値に変える。打ち切りのメッセージは `requestSummary` 経由で同じ値を使う（`response.go:297-307`） | 値と doc コメントだけを変える（設計書 3.2）。リポジトリの Go のコードで `16000` という数値は `request.go:26` にだけある（`rg -n "16000\|16,000" --glob '!docs/tasks/**' .` を `68f4cb9` で実行すると、ほかに一致したのは `internal/fixprguard/changes.go:32` の `"160000"` だけだった） |
| タイムアウト | `LLMTimeout`（`internal/llm/provider/provider.go:16-20`） | doc コメントの最後の文だけを書き換える（設計書 3.3）。値は変えない |
| provider のテスト | `claudeValidEnv`（`provider_test.go:98-107`。effort `high` を含む）、`loadConfig`（`:109`）、`TestNewClaudeSendsConfiguredRequest`（`:257-308`）、`claudeRequestFor`（`:314-332`。生成のプロバイダとモデルだけでアダプタを作り、`MaxOutputTokens` 0 の `Generate` を 1 回送り、記録したリクエストを返す。タイムアウトが 15 分であることも確かめる） | `claudeRequestFor` を、検証・推敲のプロバイダとモデルでも作れるようにする（設計書 3.7）。新しい 2 つのテスト関数を足す。送ったリクエストの `max_tokens` は記録した本文から読む（`defaultMaxOutputTokens` は `claude` パッケージの非公開の定数で、provider のテストからは見えない） |
| config のテスト | `validClaudeEnv`（`config_test.go:43-52`）、`with`・`without`（`:55-68`）、`TestLoadClaude`（`:219-271`）、`TestLoadClaudeMissing`（`:273-298`。`"effort unset"` は `:281`）、`TestLoadClaudeInvalid`（`:300`）、`TestLoadIgnoresOtherProviderVars`（`:380-460`）、`assertVarError`（`:578-592`。最初の `*VarError` だけを見る）、`TestLoadErrorsOmitValues`（`:666`。effort の行は `:720`）、`validServerEnv`（`:859-863`）、`TestLoadFor`（`:1015`。`"no stage reads no LLM variable"` は `:1119-1133`） | `"effort unset"` の行を外し、`TestLoadClaudeEffortDefault` と、`TestLoadClaudeMissing` の AC-06 の副テスト、`TestLoadFor` の 2 つの副テストを足す（設計書 3.6・7.1） |
| effort を設定する既存のテスト | `YT2COLUMN_CLAUDE_EFFORT` を設定するテストは、`internal/config/config_test.go`、`internal/llm/provider/provider_test.go`、`cmd/yt2column/run_test.go:617`・`:1954`、`cmd/yt2column-server/run_test.go:320` にある（`rg -n "YT2COLUMN_CLAUDE_EFFORT\|claudeEffortEnv" --glob '!docs/tasks/**'` を `68f4cb9` で実行した）。未設定を `ErrMissing` と期待するのは `config_test.go:281` の行だけである。`cmd/yt2column/run_test.go:1949` の `TestRunClaudeConfigurationErrorsOneLineEach` は effort に不正な値 `High` を設定しており、既定の影響を受けない | 変えない |
| writer の 3 つの段階 | 生成 `writer.go:463`、字数の調整 `shorten.go:28`、検証・推敲 `review.go:256`。どれも `MaxOutputTokens: 0` を書いて `Generate` を呼ぶ。生成の 0 は `TestWriteCallsGenerateOnce`（`writer_test.go:138`。確認は `:153`）が確かめている。`TestWriteStepOrder`（`steps_test.go:43`）の `"over the limit"` の副テストは、生成と字数の調整の 2 回（`gen.Calls`）と検証・推敲の 1 回（`review.Calls`）の呼び出しを記録している | `"over the limit"` に、`gen.Calls[1]` と `review.Calls[0]` の `MaxOutputTokens` が 0 であることの確認を足す（設計書 7.1） |
| Claude アダプタのテスト | `TestGenerateMaxTokens`（`claude_test.go:342-372`）と `TestGenerateValidationOrder`（`response_test.go:160-181`）は定数 `defaultMaxOutputTokens` を参照する | 変えない（設計書 3.6） |
| DeepSeek のテスト | `TestGenerateSendsRequest`（`deepseek_test.go:216`。`MaxOutputTokens` 0 で `max_tokens` を送らないことの確認は `:282-284`）、`TestGenerateMaxTokens`（`:287`） | 変えない |
| タイムアウトのテスト | `claudeRequestFor`（`provider_test.go:318-320`）、`cmd/yt2column/run_test.go:633`（「15-minute」）、`cmd/yt2column-server/run_test.go:426-427`（「about 45 minutes」） | 変えない |
| README | 表の `YT2COLUMN_CLAUDE_EFFORT` の行（`README.md:724`）、空の値の文（`:735-736`）、`max` の注意（`:750-752`）、`--no-review` の段落（`:116-121`） | ステップ 3-1 で書き換える（設計書 3.4）。統合テストの節の `YT2COLUMN_TEST_CLAUDE_EFFORT`（`:974-975`）は別の変数なので変えない |
| ほかの文書 | `docs/dev/project_overview.md:115`、`docs/dev/developer_guide/package_reference.md:27`、`CLAUDE.md` の「Development Guides」（`CLAUDE.md:12`） | ステップ 3-2・3-3・3-5 で書き換える。`docs/dev/security.md` と `docs/translation_glossary.md` には effort も `max_tokens` も現れない（`68f4cb9` で `rg -n -i effort` を実行して確かめた）ので、変えない |
| 文書を読む既存のテスト | 設計書 3.8 の表のとおり。どれも本タスクが変える箇所の値を読まない | 変えない。README を変えるので `make ext-test` を実行する（CLAUDE.md「Development Notes」） |
| 上限のガイドの手順で使う CLI のフラグ | `--no-shorten`・`--no-review`（`cmd/yt2column/run.go:230-231`）、`--artifacts`・`--after`（`:237-238`）。`checkResumeFlags`（`:502-526`）は `--after` と `--no-review`・`--no-shorten` の組み合わせを拒否しない。`resumeNeed`（`:481-493`）は、`--after generation --no-shorten` を検証・推敲だけの実行（`Need{Review: true}`）にする。`--after shorten` は字数の調整の中間生成物を読み、なければ失敗する（`internal/artifact/store.go:139-151`） | ガイドの例は、本計画 §1.3 で訂正した設計書 3.5 の例に従う（ステップ 3-4） |

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-08 は、すべて設計書の付録 A に記録されている。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 既定の effort（F-001。AC-01〜AC-06）

設計書 8 章の優先順位 1。

**変更するファイル:** `internal/config/config.go`、`internal/config/config_test.go`、`internal/llm/provider/provider_test.go`

-   [x] **ステップ 1-1**: `internal/config/config.go` を次のように改める（設計書 3.1）。
    -   変更の前に、まだ何も変更していない状態（本計画の承認の後の `main`）で `go test -tags test -coverprofile=<スクラッチの場所>/before.out ./internal/config` と `go tool cover -func` を実行し、関数ごとの値を控える（ステップ 1-2 の比較に使う）。手元の変更を退ける必要があるときも、`git stash` は使わない（stash はほかの worktree と共有される）。
    -   設計書 3.1 のコードのとおりに、`defaultClaudeEffort` を doc コメントとともに足す。
    -   `loadClaudeEffort` で、未設定（`!ok`）のときは `cfg.claudeEffort` に `defaultClaudeEffort` を入れて `nil` を返す。空の値は今までどおり `missingVar(claudeEffortEnv)` を返す。
    -   `LLMConfig.ClaudeEffort` の doc コメントを、`// ClaudeEffort returns the effort. It is not claudeparam.EffortUnset when` `// the generation or the review provider is ProviderClaude.` から、`// ClaudeEffort returns the effort: the value of YT2COLUMN_CLAUDE_EFFORT, or` `// defaultClaudeEffort when it is unset. It is not claudeparam.EffortUnset when` `// the generation or the review provider is ProviderClaude.` に変える。
    -   `loadClaudeEffort` の doc コメントを、`// loadClaudeEffort validates YT2COLUMN_CLAUDE_EFFORT, which is required when` `// the generation or the review provider is claude. It is called only when one` `// of them is. The accepted values live in claudeparam and are not repeated` `// here.` から、`// loadClaudeEffort validates YT2COLUMN_CLAUDE_EFFORT when the generation or` `// the review provider is claude. It is called only when one of them is. Unset` `// selects defaultClaudeEffort; an empty value is rejected, not treated as` `// unset. The accepted values live in claudeparam and are not repeated here.` に変える。
    -   `LoadFor` の doc コメントの `// zero Config on any error. A need that is true reads and requires its stage's` `// provider and model variables and the selected providers' own variables; a` を、`// zero Config on any error. A need that is true reads and requires its stage's` `// provider and model variables and reads and validates the selected providers'` `// own variables; a` に変える。残りの行は変えない。
    -   （行の折り返しは `make fmt` の結果に従ってよい。文面は変えない。）
-   [x] **ステップ 1-2**: `internal/config/config_test.go` を次のように改める（設計書 3.6・7.1）。
    -   `TestLoadClaudeMissing` から `"effort unset"` の行を外す。`"effort empty"` の行は残す。
    -   AC-06 の確認を、`TestLoadClaudeMissing` の表の行ではなく、同じテストの別の副テストとして足す（表の行は `assertVarError` だけを呼ぶので、追加の確認を `"effort empty"` などの行に加えない）。環境は `validClaudeEnv()` から `ANTHROPIC_API_KEY` と `YT2COLUMN_CLAUDE_EFFORT` の両方を外したもので、`assertVarError` で `ANTHROPIC_API_KEY` の `ErrMissing` を確かめる。加えて、結合したエラーを `Unwrap() []error` でたどり、どの `*VarError` の変数名も `YT2COLUMN_CLAUDE_EFFORT` でないことを確かめる（たどり方の先例は `internal/publisher/slack_test.go:224`）。
    -   `TestLoadClaudeEffortDefault` を足す。`YT2COLUMN_CLAUDE_EFFORT` を設定しない次の環境で、`Load` が成功し、`ClaudeEffort()` が `claudeparam.EffortHigh` であることを確かめる（AC-01・AC-02）。
        -   (1) 生成と検証・推敲の両方が `claude`（`validClaudeEnv()` から effort を外したもの）。
        -   (2) 生成が `deepseek`、`YT2COLUMN_REVIEW_PROVIDER=claude`（`validEnv()` に `YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`・`ANTHROPIC_API_KEY` を加える）。
        -   (3) 生成が `claude`、`YT2COLUMN_REVIEW_PROVIDER=deepseek`（(1) の環境に `YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`・`DEEPSEEK_API_KEY` を加える）。
        -   `LoadServer`: 同じことを `LoadServer` でも確かめる。環境は、(1) から `YT2COLUMN_CACHE_DIR` を外し、`HOME` と `SLACK_WEBHOOK_URL` を加えたものとする（`validServerEnv` は DeepSeek の環境から作るので、そのままは使えない）。
    -   `TestLoadFor` に 2 つの副テストを足す（AC-05）。1 つ目は、生成が `deepseek`・検証・推敲が `claude` で `Need{Generation: true}` の場合、2 つ目は、生成が `claude`・検証・推敲が `deepseek` で `Need{Review: true}` の場合である。どちらも、`YT2COLUMN_CLAUDE_EFFORT` が未設定・空・不正（`High`）のどれでも `LoadFor` が成功し、`ClaudeEffort()` が `claudeparam.EffortUnset` であることを確かめる。
    -   変更の後に、ステップ 1-1 と同じコマンドでカバレッジを取る。`loadClaudeEffort` は中身が変わるので、カバレッジが 100% であることを確かめる。ほかの関数は、値が変わっていないことを確かめる。比べた結果は、コミットのメッセージに書く。
-   [x] **ステップ 1-3**: `internal/llm/provider/provider_test.go` を次のように改める（設計書 7.1）。
    -   `claudeRequestFor` を、生成のプロバイダとモデル（`cfg.LLM().Provider()`・`Model()`）でも、検証・推敲のプロバイダとモデル（`ReviewProvider()`・`ReviewModel()`）でもアダプタを作れるようにする。`claudeRequestFor` は `newClient` を直接呼ぶので、`New`・`NewReview` がどちらを選ぶかは確かめない。それは既存の `TestNewReviewUsesReviewModel`（`provider_test.go:442`）が確かめている。呼び出し元の `TestNewClaudeSendsConfiguredRequest` の期待する値は変えない。
    -   `TestNewClaudeDefaultEffort` を足す。ステップ 1-2 の `TestLoadClaudeEffortDefault` と同じ 3 つの環境（`YT2COLUMN_CLAUDE_EFFORT` を設定しない）で `config.Load` した設定から、`claude` の段階のアダプタを `claudeRequestFor` で作り、送ったリクエストの `output_config.effort` が `high` であることを確かめる。(1) では生成と検証・推敲の両方のアダプタを確かめる（AC-01）。(2) では検証・推敲のアダプタ、(3) では生成のアダプタを確かめる（AC-02）。
-   [x] **ステップ 1-4**: 次のそれぞれの変更を一時的に加え、名指ししたテストを実行して、そのテストが述べた理由で失敗することを確かめる。確かめたら変更を元に戻し、結果をコミットのメッセージに書く。
    -   `loadClaudeEffort` の未設定の分岐を、変更前の `missingVar(claudeEffortEnv)` を返す形に戻す。`TestLoadClaudeEffortDefault`・`TestNewClaudeDefaultEffort`・`TestLoadClaudeMissing` の AC-06 の副テストが失敗することを確かめる。
    -   `defaultClaudeEffort` を `claudeparam.EffortMedium` にする。`TestLoadClaudeEffortDefault` と `TestNewClaudeDefaultEffort` が失敗することを確かめる。
    -   既定を `loadClaudeEffort` の外に移し、`loadLLM` の最後で、`cfg.claudeEffort` が `claudeparam.EffortUnset` なら need によらず `defaultClaudeEffort` を入れるようにする。`TestLoadFor` の足した 2 つの副テストが失敗することを確かめる（既定が、実行する段階に `claude` のない実行に及ぶ変更を、この 2 つの副テストが捉えるかを確かめるためである）。

**完了の条件:** `make test` と `make lint` が通る。ステップ 1-2 のカバレッジの比較と、ステップ 1-4 の確かめが、コミットのメッセージにある。ステップ 1-1 だけでは `TestLoadClaudeMissing` の `"effort unset"` の行が失敗するので、ステップ 1-1〜1-4 は 1 つのコミットにする（フェーズ 2・3 も、それぞれ 1 つのコミットにする）。

### フェーズ 2: 既定の上限（F-002。AC-07〜AC-12）

設計書 8 章の優先順位 2。フェーズ 1 とは互いに依存しない。

**変更するファイル:** `internal/llm/claude/request.go`、`internal/llm/provider/provider.go`、`internal/llm/provider/provider_test.go`、`internal/writer/steps_test.go`

-   [x] **ステップ 2-1**: `internal/llm/claude/request.go` の `defaultMaxOutputTokens` を `16000` から `40000` に変える（設計書 1.2・3.2）。doc コメントを、`// defaultMaxOutputTokens is sent as max_tokens when the request leaves the` `// output limit to the provider (MaxOutputTokens 0). The Messages API` `// requires max_tokens, so a value must always be sent.` から、次の文面に変える。`// defaultMaxOutputTokens is sent as max_tokens when the request leaves the` `// output limit to the provider (MaxOutputTokens 0). The Messages API` `// requires max_tokens, so a value must always be sent. The value is at least` `// twice the largest observed output, and producing it at the slowest observed` `// output rate takes at most half of provider.LLMTimeout; a test in` `// internal/llm/provider pins both conditions.` `// docs/dev/developer_guide/claude_output_limit.md describes how to re-measure` `// them.` コメントはタスクの文書を指さない（設計書 1.1）。
-   [x] **ステップ 2-2**: `internal/llm/provider/provider.go` の `LLMTimeout` の doc コメントを、`// LLMTimeout bounds one LLM call, for every provider. DeepSeek holds a request` `// for up to ten minutes before inference starts; the remaining five minutes` `// cover the generation itself. A non-streaming Claude call within the Claude` `// adapter's default output limit was measured to finish well inside it.` から、次の文面に変える（設計書 3.3）。`// LLMTimeout bounds one LLM call, for every provider. DeepSeek holds a request` `// for up to ten minutes before inference starts; the remaining five minutes` `// cover the generation itself. The Claude adapter's default output limit is` `// sized so that a non-streaming call producing that many tokens at the slowest` `// observed output rate finishes within half of this timeout; a test in this` `// package pins that relation, and` `// docs/dev/developer_guide/claude_output_limit.md describes how to re-measure` `// it.` 値（15 分）は変えない。
-   [x] **ステップ 2-3**: `internal/llm/provider/provider_test.go` に `TestClaudeDefaultMaxTokensFitsBudget` を足す（設計書 7.1）。
    -   `claudeValidEnv()` の設定から `claudeRequestFor` で生成のアダプタを作り（`MaxOutputTokens` 0 の `Generate` を送る）、記録した本文の `max_tokens` を読む。
    -   観測した出力の最大（15,884 トークン）と、最も遅い出力の速さ（14.374 秒で 1,364 トークン）を、テストの名前付きの値にする。値の出典（0041 の設計書 1.4、0018 評価記録 §4.7.2・§4.7.4）をコメントに英語で書く。
    -   (a) 送った `max_tokens` が、観測した出力の最大の 2 倍以上であることを確かめる。(b) 送った `max_tokens` を最も遅い出力の速さで出力する時間が、`LLMTimeout` の半分以内であることを確かめる。(b) は、トークンあたりの時間を求める割り算をせず、`time.Duration` の掛け算で比べる（`max_tokens × 14.374 秒 ≤ 1,364 × (LLMTimeout ÷ 2)`）。両辺は約 6.1 × 10^14 ナノ秒で、int64 の範囲に収まる。
-   [x] **ステップ 2-4**: `internal/writer/steps_test.go` の `TestWriteStepOrder` の `"over the limit"` に、字数の調整の呼び出し（`gen.Calls[1]`）と検証・推敲の呼び出し（`review.Calls[0]`）の `Request.MaxOutputTokens` が 0 であることの確認を足す（AC-08 の writer の層。設計書 7.1）。
-   [x] **ステップ 2-5**: 次のそれぞれの変更を一時的に加え、名指ししたテストを実行して、そのテストが述べた理由で失敗することを確かめる。確かめたら変更を元に戻し、結果をコミットのメッセージに書く（設計書 7.1 の最後の項目）。
    -   `defaultMaxOutputTokens` を `31767` にする。`TestClaudeDefaultMaxTokensFitsBudget` が (a) の確認で失敗することを確かめる。
    -   `defaultMaxOutputTokens` を `42703` にする。`TestClaudeDefaultMaxTokensFitsBudget` が (b) の確認で失敗することを確かめる。
    -   `writer.go:463`・`shorten.go:28`・`review.go:256` の `MaxOutputTokens: 0` を、1 か所ずつ順に `1` にする。生成の箇所では `TestWriteCallsGenerateOnce` が、字数の調整と検証・推敲の箇所では `TestWriteStepOrder` が失敗することを確かめる。

**完了の条件:** `make test` と `make lint` が通る。ステップ 2-5 の確かめが、コミットのメッセージにある。

### フェーズ 3: 文書（F-003。AC-14〜AC-19）

設計書 8 章の優先順位 3。フェーズ 1・2 の振る舞いを書くので最後にする。どの文書にも V の数値と出力の速さを書かない（設計書 1.2）。

**変更するファイル:** `README.md`、`docs/dev/project_overview.md`、`docs/dev/developer_guide/package_reference.md`、`docs/dev/developer_guide/claude_output_limit.md`（新規）、`CLAUDE.md`

-   [x] **ステップ 3-1**: `README.md` を、設計書 3.4 の表の README の 4 行のとおりに改める。
    -   「Configuration」の表の `YT2COLUMN_CLAUDE_EFFORT` の行の「When unset」を、``Required when the generation or review provider is `claude` (an error)`` から `` `high` when the generation or review provider is `claude` `` に変える（AC-14）。
    -   `max` の注意（`:750-752`）を残し、既定の出力の上限でも `max` は推論だけで上限を使い切って記事の本文を返さないと見込まれることを、数値を挙げずに書く（AC-15）。
    -   その後に、既定の `high` は claude-opus-5-5 に合わせて選んだ値であること、ほかのモデル（例: claude-sonnet-5-5）では `high` でも出力の上限に達して打ち切られうること、その場合は `YT2COLUMN_CLAUDE_EFFORT` で effort を下げられることを書く段落を足す（AC-19）。
    -   `--no-review` の段落（`:116-121`）の `that provider's variables (its API key, and for` `` `claude` its effort) are still required`` を、API キーだけが必須である書き方に直す（AC-14）。設定した effort は今までどおり確かめられる（空の値や不正な値は拒否される）ことが読み取れる書き方にする。これは、`--no-review` でも `--after` のない実行は検証・推敲の設定を読む（`resumeNeed`、`cmd/yt2column/run.go:482-484`）ためである。
-   [x] **ステップ 3-2**: `docs/dev/project_overview.md` の「設定（環境変数）」の表の `YT2COLUMN_CLAUDE_EFFORT` の行（`:115`）の「未設定のとき」を、``生成または検証・推敲のプロバイダが `claude` ならエラー`` から ``生成または検証・推敲のプロバイダが `claude` なら `high` `` に変える（AC-16）。
-   [x] **ステップ 3-3**: `docs/dev/developer_guide/package_reference.md` の `internal/config` の行（`:27`）の ``the Claude effort via `claudeparam.ParseEffort` `` に、未設定なら `high`（`defaultClaudeEffort`）であることを加える（設計書 3.4）。
-   [x] **ステップ 3-4**: 上限のガイド `docs/dev/developer_guide/claude_output_limit.md` を英語で新設する（AC-17。設計書 3.5）。
    -   節は設計書 3.5 の表の 7 つ（When to re-measure、Cost and approval、How to measure、What to collect、How to judge、What to update、Current record）とし、各節に表の「内容」を書く。`pricing_update.md` の書き方（手順の番号付きの列挙、参照の書き方）に揃える。
    -   「What to update」には、ステップ 2-3 の名前付きの値（観測した出力の最大と最も遅い出力の速さ）とそのテストのファイルを挙げる。「Current record」は、今の根拠の記録として本タスクの設計書 1.4 を指し、値は写さない。
    -   「How to measure」のコマンドの例は、訂正した設計書 3.5 のとおりにする。生成は `--no-review --no-shorten`、字数の調整は `--after generation --no-review`、検証・推敲は、字数の調整が走った実行のあとなら `--after shorten`、走らなかった実行のあとなら `--after generation --no-shorten` である（本計画 §1.3）。どれも `--artifacts` を付ける。`--after generation --no-shorten` は字数の調整を飛ばし、最後の字数の確認も行わない（`internal/writer/writer.go:401`）ので、字数の調整が走らなかった実行のあとに限って使うことをガイドに書く。
    -   例のフラグと組み合わせを、コードで確かめる。フラグは `go run ./cmd/yt2column --help`（実 API は呼ばない）、組み合わせの受理は `checkResumeFlags`（`cmd/yt2column/run.go:502-526`）、実行する段階は `resumeNeed`（`:481-493`）と `internal/writer/writer.go:366-380`、中間生成物の読み込みは `internal/artifact/store.go:139-151` で確かめる。確かめたことをコミットのメッセージに書く。打ち切りのエラーの文（`generation truncated by the output token limit`）は、`internal/llm/llm.go:16` の `ErrTruncated` の文と照らす。
-   [x] **ステップ 3-5**: `CLAUDE.md` の Quick Links の「Development Guides」に、上限のガイドへのリンクと 1 行の説明を足す（AC-18）。位置は `pricing_update.md` の行の後とする。
-   [x] **ステップ 3-6**: `make fmt`・`make test`・`make lint`・`make ext-test` を通す（README を変えるので `make ext-test` も実行する。CLAUDE.md「Development Notes」）。本計画 §5 の横断の確認の項目を確かめる。

**完了の条件:** `make test && make lint` と `make ext-test` が通る。本計画 §5 の横断の確認の項目を確かめた。

### PR-1 作成ポイント: claude effort default and output limit

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6

**推奨タイトル**: `feat(0041): Claude の effort の既定を high にし、出力の上限を引き上げる`

**レビュー観点**: 既定が未設定のときだけ適用され、空の値の扱いと、claude の段階がない実行の扱いが変わらないこと（`TestLoadFor` の足した副テスト）/ (a)・(b) のテストの名前付きの値が設計書 1.4 と一致し、writer の 3 か所の `MaxOutputTokens` 0 の確認とともに、壊して確かめた記録がコミットのメッセージにあること / README・`project_overview.md`・上限のガイドに V の数値がなく、effort を必須とする記述が残っていないこと / 上限のガイドの手順とフラグが今の CLI と一致すること

**実装モデル要件**: standard

**判定理由**: どのステップも設計書が手段を決めており、`既存コード調査結果` に競合する手段はない。CI のワークフローは変えず（走るのは既存のジョブ）、外部サービスもデータの移行もなく、mkplan の条件付きの確認の項目にも当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 | 対応するフェーズ |
|---|---|---|
| M1: 既定の effort | 未設定なら `high`。config と provider のテスト | 1 |
| M2: 既定の上限 | V = 40,000、`LLMTimeout` の doc コメント、(a)・(b) のテスト、writer のテスト | 2 |
| M3: 文書 | README・`project_overview.md`・`package_reference.md` の更新、上限のガイド、`CLAUDE.md` のリンク | 3 |

フェーズ 1 と 2 は互いに依存しない。フェーズ 3 は両方の振る舞いを書くので最後にする（設計書 8 章）。3 つとも PR-1 に入る（本計画 §1.3）。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 | 既定の effort、既定の上限、そのテスト、文書、上限のガイド | standard |

PR-1 はコードを変えるので、CI の `Test` ジョブが走る。README を変えるので、拡張のジョブ（README の表を読む node のテスト）も走る。

## 4. テスト戦略 (Test Strategy)

### 4.1 単体テスト

-   **新しいテスト:** `TestLoadClaudeEffortDefault`・`TestLoadClaudeMissing` の AC-06 の副テスト・`TestLoadFor` の 2 つの副テスト（`internal/config`）、`TestNewClaudeDefaultEffort`・`TestClaudeDefaultMaxTokensFitsBudget`（`internal/llm/provider`）、`TestWriteStepOrder` の確認の追加（`internal/writer`）。
-   **外すテスト:** `TestLoadClaudeMissing` の `"effort unset"` の行だけである。カバレッジの比較はステップ 1-2 で行う。
-   **変えない既存のテスト:** AC-03・AC-04・AC-09〜AC-12 は、設計書 7.1 のとおり既存のテストが確かめる。定数を参照する `TestGenerateMaxTokens` と `TestGenerateValidationOrder` は、値を V に変えるとそのまま V を確かめる。
-   **2 つの層（AC-08）:** writer の層（偽の `LLMClient` が受け取る `MaxOutputTokens` が 0）と、アダプタの層（0 を受け取ると既定の上限を送る `TestGenerateMaxTokens`）を分けて確かめる（設計書 7.1）。偽の `LLMClient` は 0 を変換しないので writer の層だけを、`TestGenerateMaxTokens` は writer を通らないのでアダプタの層だけを確かめる。
-   **壊して確かめること:** ステップ 1-4・2-5。

### 4.2 統合テスト

本タスクで実 API の呼び出しは行わない。実 API を呼ぶ統合テスト（`internal/llm/claude/integration_test.go`）は正の `MaxOutputTokens` を渡すので、本タスクの影響を受けない（設計書 7.2）。

### 4.3 後方互換のテスト

-   `YT2COLUMN_CLAUDE_EFFORT` を設定した利用者の振る舞い（AC-03）は、既存の `TestLoadClaude`・`TestNewClaudeSendsConfiguredRequest` が確かめる。
-   DeepSeek だけを使う実行（AC-05・AC-11）は、既存の `TestLoadIgnoresOtherProviderVars` と DeepSeek のテストが確かめる。
-   正の `MaxOutputTokens`（AC-09）は、既存の `TestGenerateMaxTokens` の `"positive sends its value"` が確かめる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | test | `internal/config/config_test.go::TestLoadClaudeEffortDefault`（環境 (1) と `LoadServer`）、`internal/llm/provider/provider_test.go::TestNewClaudeDefaultEffort`（環境 (1) の生成と検証・推敲のアダプタ） | 1-1・1-2・1-3・1-4 |
| AC-02 | test | `internal/config/config_test.go::TestLoadClaudeEffortDefault`（環境 (2)・(3)）、`internal/llm/provider/provider_test.go::TestNewClaudeDefaultEffort`（環境 (2)・(3)） | 1-1・1-2・1-3・1-4 |
| AC-03 | test | `internal/config/config_test.go::TestLoadClaude`、`internal/llm/provider/provider_test.go::TestNewClaudeSendsConfiguredRequest`（既存。5 つの値） | — |
| AC-04 | test | `internal/config/config_test.go::TestLoadClaudeMissing`（`"effort empty"`）、`::TestLoadClaudeInvalid`、`::TestLoadErrorsOmitValues`（`"claude effort invalid"`）（既存） | 1-2 |
| AC-05 | test | `internal/config/config_test.go::TestLoadIgnoresOtherProviderVars`、`::TestLoadFor`（既存の `"no stage reads no LLM variable"` と、足す 2 つの副テスト） | 1-2・1-4 |
| AC-06 | test | `internal/config/config_test.go::TestLoadClaudeMissing`（足す副テスト） | 1-2 |
| AC-07 | test | `internal/llm/provider/provider_test.go::TestClaudeDefaultMaxTokensFitsBudget`、`internal/llm/claude/claude_test.go::TestGenerateMaxTokens`（既存の `"zero sends the default"`） | 2-1・2-3・2-5 |
| AC-08 | test | `internal/writer/steps_test.go::TestWriteStepOrder`（字数の調整と検証・推敲）、`internal/writer/writer_test.go::TestWriteCallsGenerateOnce`（生成。既存）、`internal/llm/claude/claude_test.go::TestGenerateMaxTokens`（アダプタの層。既存） | 2-4・2-5 |
| AC-09 | test | `internal/llm/claude/claude_test.go::TestGenerateMaxTokens`（既存の `"positive sends its value"`） | — |
| AC-10 | test | `internal/llm/claude/response_test.go::TestGenerateValidationOrder`（既存） | 2-1 |
| AC-11 | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateSendsRequest`、`::TestGenerateMaxTokens`（既存） | — |
| AC-12 | test | `internal/llm/provider/provider_test.go::claudeRequestFor` の 15 分の確認（`TestNewClaudeSendsConfiguredRequest` などが通る）、`cmd/yt2column/run_test.go` の `"LLM timeout"` の行（`:633`）、`cmd/yt2column-server/run_test.go` の停止の通知の確認（`:426-427`）（既存） | 2-2 |
| AC-13 | manual | 設計書 1.4 が V と (a)・(b)・(c) の根拠を記録している。設計書の PR（#494）のレビューで読み、承認済み | — |
| AC-14 | manual | PR-1 のレビューで、README の表の行と `--no-review` の段落を読み、README と `project_overview.md` に effort を必須とする記述が残っていないことを確かめる | 3-1・3-2 |
| AC-15 | manual | PR-1 のレビューで、README の `max` の注意を読む | 3-1 |
| AC-16 | manual | PR-1 のレビューで、`project_overview.md` の表の行を読む | 3-2 |
| AC-17 | manual | PR-1 のレビューで、上限のガイドの 4 つの内容（契機、集める値、判定の方法、更新するもの）を読む。コマンドの例のフラグは、ステップ 3-4 で今の CLI と照らした記録をコミットのメッセージで確かめる | 3-4 |
| AC-18 | manual | PR-1 のレビューで、`CLAUDE.md` の「Development Guides」のリンクと説明を読む | 3-5 |
| AC-19 | manual | PR-1 のレビューで、README の既定の effort とモデルの段落を読む | 3-1 |

AC-13〜AC-19 は文書の文面を定める AC なので、`manual` の読みで確かめる（設計書 7.4、CLAUDE.md「Document tests check values, structure, and executable examples, never prose」）。文書に数値を写さないので、値の一致を確かめる文書のテストも要らない（設計書 1.2）。

**横断の確認（`make test`・`make lint` が検出しないもの。ステップ 3-6 で、PR の差分を読んで確かめる。文書の語の有無を `rg` で確かめる手段にはしない）:**

-   [x] `README.md` と `docs/dev/project_overview.md` に、`YT2COLUMN_CLAUDE_EFFORT` を必須とする文やエラーとする文が残っていない（ステップ 3-1・3-2 の書き換えの漏れ。PR-1 のレビューでも読む）。
-   [x] `README.md`・`docs/dev/project_overview.md`・上限のガイドに、V の値（40,000 またはその表記の揺れ）と出力の速さの数値が書かれていない（設計書 1.2）。
-   [x] Go のコメントが、上限のガイドを指し、`docs/tasks/` を指さない（`request.go`・`provider.go`・`config.go` の変えたコメント。テストの名前付きの値の出典のコメントは、設計書 1.4 と評価記録を出典として挙げてよい）。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| タスク 0042 の実装が、同じ `config.go`・README・`project_overview.md` の箇所を変える | 取り込みの衝突 | 0042 は本タスクの後にマージし、衝突は本タスクの形を正として解く（0042 設計書 8 章の 6）。本タスクでは対策を要しない |
| `TestLoadClaudeMissing` の行を外したことで、`loadClaudeEffort` の分岐のカバレッジが落ちる | 未設定の扱いの退行を検出できない | ステップ 1-2 で、変更の前後の `go tool cover -func` を比べる |
| (b) の境界の判定を誤る（割り算の丸めなど） | ステップ 2-5 で `42703` に変えても (b) の確認が失敗せず、境界を確かめられない | ステップ 2-3 で `time.Duration` の掛け算で比べる。この比べ方では、`42702` は 613,798,548 ≤ 613,800,000 で (b) を満たし、`42703` は 613,812,922 で (b) を満たさない（単位はミリ秒×トークン。計画のレビューで計算した）。ステップ 2-5 では、`42703` で (b) の確認が実際に失敗することを確かめる |
| タスク 0042 が本タスクの PR-1 のマージを待つ（0042 設計書 8 章の 6） | PR-1 のレビューや CI が長引くと、0042 の実装も遅れる | PR を 1 つにして、待つのを 1 回のマージにした（本計画 §1.3）。PR-1 が長引く場合は、0042 の実装を先に進め、マージだけを PR-1 の後にする |
| 上限のガイドの手順が、今の CLI のフラグの組み合わせ（とくに `--after shorten`）と食い違う | 測り直しの手順が実行できない | ステップ 3-4 で `--help` と README の再開の段落で確かめる。実 API での実行はしない（要件書 2.3） |
| 長い非ストリーミングの呼び出しが、無通信で途中で切れる | 料金がかかり、記事が出ない | 設計書 1.2・3.6・4.3 のとおり受け入れる。本計画では仕組みを足さない |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6）

## 8. 成功の基準 (Success Criteria)

-   **機能の完全性:** `YT2COLUMN_CLAUDE_EFFORT` が未設定なら effort が `high` になり、Claude アダプタの既定の上限が 40,000 になる。本計画 §5 のすべての AC の検証が済んでいる。
-   **品質:** `make test && make lint` と `make ext-test` が通る。ステップ 1-2 のカバレッジの比較と、ステップ 1-4・2-5 の壊して確かめた記録が、コミットのメッセージにある。
-   **セキュリティ:** 送る相手とデータの種類は変わらない（設計書 5 章）。effort の拒否のエラーが値を含まないことは、既存の `TestLoadErrorsOmitValues` が確かめる。
-   **文書:** README・`project_overview.md`・`package_reference.md`・上限のガイド・`CLAUDE.md` が、本計画 §5 の AC-14〜AC-19 を満たし、V の数値を含まない。

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/runplan 0041` でフェーズ 1 から始める。
-   PR-1 のマージの後、issue #171 を閉じる（`Refs #171` では自動で閉じない）。その後、タスク 0042 の実装を進められる（0042 設計書 8 章の 6）。
