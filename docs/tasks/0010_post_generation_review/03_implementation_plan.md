# 実装計画書：生成後の検証・推敲と字数の調整

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-10 |
| Review date | 2026-10-10 |
| Reviewer | isseis |
| Comments | 承認の後、`/mkplan2` で PR の区切り（§2 の `PR-N 作成ポイント`・§3.2・§7、旧 §3.2 は §3.3 へ。§1.2・§2 の冒頭・§9 も合わせた）を埋め込み、フェーズ 5 の壊して確かめる対象をステップ 5-11 から 5-4・5-8 に分けた。編集上の変更で、何を作るかは変えていない。 |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md`（以下「requirements」）が定める 2 つの段階（字数の調整と検証・推敲）と、その設定・CLI・評価を、`02_architecture.md`（以下「architecture」）の設計どおりに実装する。段階は `internal/writer` の `ArticleWriter` の中に置き、`writer.Options` のゼロ値では段階を行わない（architecture §1.1）。検証・推敲の LLM は直した箇所の一覧（以下「一覧」）を返し、`ArticleWriter` はその応答を `internal/strictjson` で境界の検査をしてから文面に当てはめる。さらに、検証・推敲のプロバイダとモデルの設定、`--out` と Webhook の出力に加える段階のモデルの行、一覧を書き出すファイル（`--review-log`）、CLI のフラグを加え、最後に 0008 の 5 本の動画で評価する。

本計画のフェーズ 1〜6 は、architecture §8 の実装優先順位のフェーズ 1〜6 と同じ名前・同じ順序である。用語（段階、パイプラインの段、文面、文面の文字列、素材、本文、検査）は architecture の冒頭の「用語」に従う。

### 1.2. 実装原則

- architecture §1.1 の設計原則 8 項目に従う。特に、段階の有無は `Options` に明示し（原則 2）、検証・推敲の応答は補正せずに拒否し（原則 5）、段階の失敗では前の段階の文面を返さない（原則 8）。
- 新設・変更するファイルは architecture §3.12 の責務表を基本とする。表にないファイルの変更（guard テストの更新、古くなるコメントや文書の修正）は、各ステップに対象ファイルとして明記する。
- Go のコメント・識別子・文字列リテラルは英語で書く。`AC-NN`・`F-NNN`・`H-NN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- 振る舞いを変えない取り出し・一般化（ステップ 2-1・2-2・5-2）は、それぞれ独立したコミットにし、そのコミットで既存テストの期待値を変えない。差分に含まれるテストの変更が呼び出しの形の機械的な置き換えだけであることを、コミット前に差分で確かめ、コミットメッセージに書く。
- 各テストは、対象の分岐を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」）。各フェーズの最後のステップに、壊す対象を挙げる。フェーズ 5 は 3 つの PR に分かれるので、各 PR の最後のステップ（5-4・5-8・5-11）に、その PR の壊す対象を挙げる。
- 各フェーズの完了条件は、`make fmt` の後にグリーンゲート（`_context.md` の "Green gate"。`make test`・`make lint`）が通ることである。文書だけを変えるコミットでも `make ext-test` を通す（CLAUDE.md「Development Notes」）。

### 1.3. 既存コード調査結果

HEAD `4dd797d`（ブランチ `claude/mkplan-0010-xnazts`）で確認した。その後の `25d38bd` は architecture の Status だけを変えたコミットで、コードは同じである。architecture が行番号を確かめた `ce9ee27` はローカルにないが、architecture は「コードは `ef603fd` と同じ」と記録している。`git diff --stat ef603fd HEAD -- . ':!docs'`（2026-10-10）の差分は、`internal/llm/deepseek/{deepseek,errors,response}.go`、`internal/llm/llmhttp/` の新設、`internal/pipeline/pipeline_test.go` の 1 行（`testOnlyPackageDirs`）だけである。そのため、architecture が引く `internal/writer`・`internal/config`・`internal/llm/provider`・`internal/publisher`・`internal/job`・`cmd/yt2column`・`internal/strictjson` の行番号は HEAD でもそのまま有効である（以下の行番号も HEAD `4dd797d` で確かめた）。

ベースラインの `go test -tags test ./...`（2026-10-10、HEAD `4dd797d`、go1.26.5）は、失敗は 17 件で、すべて `requireNonRoot` 系の「unsupported test environment: permission-failure tests require a non-root user」であった。この作業環境は root で動いているためである。本タスクと関係のない環境の制約として扱い、グリーンゲートは root 以外の利用者で実行する（§6）。

- **`internal/writer`（フェーズ 2・3 で変える）。**
  - `writer.go:17-23` の `Article`、`:35-38` の `Options`、`:42-46` の `templateWriter`（`system`・`user` は `*template.Template`）、`:51-64` の `New`、`:66-97` の `Write`（doc コメントを含む）。
  - `output.go:44-86` の `checkResponse` は、`:45-76` が生成テキストの規則、`:77-84` がモデル名とモデルの版の規則である。`:133-141` の `newArticle`、`:101-111` の `checkModelString`、`:120-128` の `displayStringProblem`。
  - `template.go:81-91` の `loadTemplate`、`:134-156` の `parseTemplate`、`:317-324` の `checkField` は `reflect.TypeFor[templateData]()` に固定されている。`prompt.go:20-25` の `templateData`、`:100-114` の `expand`。
  - `parseTemplate` の呼び出しは、本番では `template.go:90` の 1 か所、テストでは `template_test.go:404`・`:426` の 2 か所である（`rg -n "parseTemplate\(" internal/writer`）。`expand` の呼び出しは `writer.go:80`・`:84` の 2 か所で、テストからの直接の呼び出しはない（`rg -n "expand\(" internal/writer`）。`syntaxChecker{tree: …}` を直接作るのは `template.go:151` と `template_test.go:381` である。
  - `template_test.go:245` の `TestTemplateSyntaxAllowlist` は、`test_helpers.go:100-106` の `overrideTargets`（`system`・`user` の 2 つ）を回して表を当てる。受理の表の行は `.Title`・`.ChannelName` などの生成のフィールドを参照するので、字数の調整のデータ型（`.Title` を持たない）にはそのまま当てられない（ステップ 3-9）。
  - 既存のテストのゼロ値の比較（`test_helpers.go:88` の `a != (Article{})`）は、architecture §3.1 のとおりポインタのフィールドなら変更せずに通る。
- **`internal/llm/testutil/mocks.go`。** `FakeLLMClient` は 1 つの `Result` を返し続ける。呼び出しの順に別の応答を返す fake はない（ステップ 3-2 で加える）。
- **`prompts`。** `prompts.go` は `system.tmpl`・`user.tmpl` を埋め込み、`System()`・`User()` で返す。`prompts/` にテストファイルはない。`prompts/README.md` の「参照できる値」の表は、`internal/pipeline/pipeline_test.go:691` の `TestPromptsREADMEMatchesContract` が、`readWriterTemplateContract`（`:800-843`）で `internal/writer` のソースから読んだ `templateData` のフィールドと比べている。`structFieldNames`（`:857-871`）は名前のあるフィールドだけを集めるので、埋め込みのフィールド（`reviewTemplateData` の `templateData`）は数えない。2 つの段階の表を README に加えるとき、この guard も同じコミットで広げる（ステップ 3-10）。この guard の更新は、architecture §3.12 の責務表に編集上の修正として加えた（同書の Comments）。
- **`internal/strictjson`。** `strictjson.go:83-95` の `Collect` は、指定しなかったキーとその重複を無視し、重複した指定のキーを `%q` で示す（`:90`）。`AsBool` に当たる関数はない。`Collect` の本番の呼び出し元は `internal/transcript/json3.go`・`info.go`、`internal/llm/deepseek/response.go`、`internal/publisher/slack.go:251` である（`rg -n "\.Collect\(" -g '*.go' internal`、2026-10-10）。したがって `internal/strictjson` を import するパッケージは、本タスクの後は `internal/transcript`・`internal/llm/deepseek`・`internal/publisher`・`internal/writer` の 4 つになる。`maxArrayElements` は `1 << 17`（`:23`）。
- **`internal/config`。** `config.go:155-170` の `Load`、`:173-187` の `loadProvider`（値の解釈も行う）、`:202-222` の `loadAPIKey`（生成のプロバイダだけを見る）。`apiKeyEnv` は `:30`、`reasonProvider` は `:52`。受理するプロバイダは `deepseek` だけで、0009 の `claude` はまだない（`rg -n -i claude internal/config/config.go` は一致なし）。
- **`internal/llm/provider`。** `newClient`（`provider.go:37`）の呼び出しは `provider.go:29` と `provider_test.go:132`・`:178` の 3 か所だけである（`rg -n "newClient\(" internal/llm/provider`）。
- **タスク 0009 との順序。** 0009 は PR-1（`internal/llm/llmhttp`、#152）だけがマージされ、設定とプロバイダの選択を変える PR-4 は未実装である。0009 の `03_implementation_plan.md`（`approved`）のステップ 4-2・4-5 は、architecture §1.3 の 3 つ目の例外（プロバイダ固有の変数を、生成と検証・推敲のどちらかが使うときに読む。`newClient` はモデルを引数で受け取る）と食い違う形を書いている。どちらを先に実装しても architecture §3.7 の形にそろえるため、ステップ 4-1・4-5 で、フェーズ 4 の着手時の 0009 の状態に応じて分岐する。
- **`internal/publisher`。** `file.go:180-186` の `renderArticle`。一時ファイルの書き出しと `link(2)` は `file.go:82-148`（`Publish`・`writeTemp`）で、テスト用の差し替え口 `wrapWriter`・`link` は `FilePublisher` のフィールド（`:50-51`）にあり、`test_helpers.go:16-34` の `newFilePublisherWithSeams` が書き換える。`slack_message.go:29` の UTF-8 の確認は `Title`・`Body`・`Model`・`ModelVersion` だけを見る。`testutil/integration.go:165-170` の `integrationHeader` は `renderArticle` の書式の写しで、段階のない記事にだけ使われる。
- **`internal/job`。** `job.go:83-86` の `Run` の doc コメントは「A nil error means the article was published」と定める。`:192-217` の `precheckFileOutput` は、規則の判定と `pipeline.StageError`（`StagePublish`）で包む処理を 1 つの関数で行っている。
- **`cmd/yt2column`。** `run.go:43-48` の `deps`、`:104-111` の `cliOptions`、`:116-127` の `newFlagSet`、`:169-256` の `run`（手順 A1〜A5・B・C）、`:263-287` の `chooseDestination`、`:336-355` の `reportSuccess`、`:377-400` の `reportRunError`、`:474-486` の `configuredSecrets`。
  - CLI のテストの `deps` は 3 通りに作られる。`run_test.go:139` の `newRunEnv` は `productionDeps()` を直接使い（大半のテスト）、`test_helpers.go` の `testDeps` も `productionDeps()` を基にする（`run_test.go:404` の `startHolder`、`:1414`、`signal_test.go` の `runChildMode` の子プロセス）。子プロセスの `childModeMain`（`signal_test.go:119`・`:207`）は本番の `main` をそのまま動かす。どのテストでも本番の `writer.New` が動く。既定で段階を行うと、`newReviewLLMClient` を差し替えない限り、本番の DeepSeek アダプタが構築される。`Write` まで進むテストでは、検証・推敲の `Generate` が閉じたプロキシ（`main_test.go` の `TestMain`）に向かって失敗する。ステップ 5-5 で、`newRunEnv` と `testDeps` の `newReviewLLMClient` を、何も作らずにエラーを返す関数にする。形は既存の `refusingSlackPublisher`（`test_helpers.go:187-192`）と同じにする。`childModeMain` には差し替え口がないので、引数で段階を無効にする。手順 A5 はパイプラインより前に実行される。そのため、段階を無効にする引数のないテストは、`Write` まで進まないものでも `exitUsage` になる。対象には、字幕の取得の失敗やキャッシュの排他の失敗（`run_test.go:419` の `executionPathRows` の `exitFailure` の行）と、`signal_test.go:213-217` の 2 回目の実行も含まれる。
  - CLI のテストの引数は次の箇所で組み立てる。`run_test.go` の `runEnv.cliArgs`（`:222-227`。既定で `--out <path> <URL>`）と、それを上書きする各テストの `e.args`。`run_test.go:404` の直書き。`signal_test.go:71-73` の `childSetup.args` と、`signal_test.go:180` の `"--slack", runVideoURL` の直書き。`integration_test.go`・`integration_slack_test.go:46`。`integration_*` は `//go:build integration` で、`test_helpers.go`（`//go:build test`）の値を使えないので、共有の値は `test_helpers_integration.go`（`//go:build test || integration`）に置く。`integration_test.go:122-128` の `generateCounter` は `newLLMClient` だけを包む。
  - `run_test.go:1063` の `TestRunHelp` は、使い方に載るべきフラグを手で並べた一覧で確かめている。
  - `docs_test.go:46` の `TestREADMEDocumentsCLI` は、`newFlagSet` のすべてのフラグが README のフラグの表にあること、`configDocRows`（`:33-40`）の変数が README の設定の表にあることを求める。`:118` の `TestProjectOverviewDocumentsConfig` は同じ行を `project_overview.md` に求める。フラグと変数を加えるコミットで、文書の行も加える必要がある。
  - `integration_test.go:152` は `Generate` の回数が 1 であることを確かめる。
- **`internal/pipeline/pipeline_test.go` の他の guard。** `TestCommonTypesFieldSets`（`:378`）は `writer.Article` のフィールドを 5 つの名前と型に固定している（`NumField` と各フィールドの型を比べる）。`Shorten`・`Review` を加えるステップ 3-4 の同じコミットで、この表に 2 行を加える。architecture §3.12 の責務表はこの更新を挙げていなかったので、編集上の修正として同表に加えた（同書の Comments）。`TestFakesCarryBuildTag`（`:531`）は `testutil` のファイルを 15 件に固定している。本計画は `testutil` に新しいファイルを加えない（`internal/llm/testutil/mocks.go`・`mocks_test.go` の変更だけ）ので、件数は変わらない。`TestWriterImports`（`:747`）は `internal/writer` が `net`・`net/*`・`internal/llm/*`（`internal/llm/testutil` をテスト専用のファイルから使う場合を除く）を import しないことを固定する。`internal/writer` → `internal/strictjson` の新しい import はこれに掛からない。新しいパッケージは作らないので、`TestPackageReferenceListsPackages`（`:607`）の行の追加は要らない。
- **文書の更新対象。** `README.md:26-50`（Usage のフラグの表）・`:57`（終了コード 1 の意味）・`:138-150`（Configuration。`:144` の `DEEPSEEK_API_KEY` の行を含む）、`docs/dev/project_overview.md:18-47`（パイプライン。`:31` の `Article` のフィールド）・`:76`（`internal/strictjson` の利用者）・`:91-104`（設定の表。`:97` の `DEEPSEEK_API_KEY` の行を含む）、`docs/dev/security.md` §4（`:52`）・§6（`:63`）、`docs/dev/developer_guide/package_reference.md:15`（`cmd/yt2column`）・`:18`（`internal/strictjson`）・`:21`（`internal/config`）・`:27`（`internal/llm/provider`）・`:28`（`internal/writer`）・`:29`（`prompts`）・`:30`（`internal/publisher`）・`:32`（`internal/job`）・`:36`（`internal/llm/testutil`）、`CLAUDE.md:94-95`（`internal/strictjson` の利用者）。`AGENTS.md` は `internal/strictjson` に触れない（`rg -n strictjson AGENTS.md` は一致なし）。英語版の文書（`*.en.md`）はない。
- **`docs/tasks/0008_column_prompt/check_article.py`。** `:26-28` の `HEADER_RE` は、モデルの 2 行の直後に空行を求めるので、段階のモデルの行がある `--out` のファイルに一致しない。字数の数え方は `:111`。
- **外部前提の確認。** `go.mod` は `go 1.26.5`、手元のツールチェーンも go1.26.5。`errors.AsType` と型引数を持つ構造体とメソッドが使える。`reflect.Type.FieldByName` は埋め込んだ構造体のフィールドを昇格して返し、埋め込みのフィールド自体（`templateData`）は `IsExported()` が偽になる（`reflect` の文書、architecture §3.4 の前提）。この 2 つの性質は、ステップ 3-9 のテスト（`.Title` の受理と `.templateData` の拒否）が実際に確かめる。外部モジュールは追加しない。
- **意図して変えないもの。** `internal/pipeline` と `internal/job` のパイプラインの段は 3 つのままである（architecture §1.1 の原則 1）。`writer.Article` の既存のフィールドの意味は変えない。
- **計画上の判断（architecture の決定は変えない）。**
  - 当てはめの単体テストと応答の検査の単体テストは、`internal/writer` の非公開の関数に対して書く（`Write` を通さずに規則の表を当てるため）。`Write` を通すテストは、各規則の代表の行と、段階の順序・失敗の伝播に絞る。同じ規則の行を両方の表に重ねない（CLAUDE.md「Check for duplication」）。
  - 既定の検証・推敲のテンプレートに理由の種類の値と JSON のキーがすべて書かれていることを、`RevisionReason` の対応表から読んで確かめる guard を置く（ステップ 3-9）。値の綴りを 1 か所に置く architecture §3.5.2 の方針を、テンプレートの文言にも及ぼすためである。
  - CLI のテストで段階を無効にする引数は、`cmd/yt2column/test_helpers_integration.go` の 1 つの値にまとめる（DRY）。`runEnv.cliArgs` と `childSetup.args` は、テストが段階を使うと宣言しない限り（`runEnv` に段階を使うことを示すフィールドを加える）、この値を引数の先頭に置く。こうすると約 40 か所の `e.args` の直書きを変えずに済み、`-h` や使い方の誤りの行にも害がない。直書きで `run` や子プロセスを起動する箇所（`run_test.go:404`、`signal_test.go:180`、`integration_*`）には、値を明示的に加える。
  - `writer` の `overrideTargets` は、`prompt_test.go:187`・`:229`・`:266` と `template_test.go:108`・`:157`・`:195`・`:358`・`:424` の 8 つのテストが回している。生成のフィールド（`.Title`）を使う固定の入力や、`Generate` の呼び出し 0 回の検査を持つので、6 つのテンプレートに広げるとこれらの意味が変わる。そのため `overrideTargets` は変えず、段階のテンプレートのための別の表を加える（ステップ 3-9）。
  - 一覧のファイルの書き出しの失敗を `internal/job` のテストで起こすには、`publisher.Publisher` の fake が `Publish` の中で一覧のファイルのパスにファイルを作る（事前確認の後、`link(2)` の前に既存のファイルができる）。乱数や時刻に頼らずに決まった順序で失敗させるためである。

### 1.4. 名前の変更・移動の一覧

本計画の他の箇所は、この表に従う。

| 変更前 | 変更後 | ステップ |
|---|---|---|
| `checkResponse` の前半（`output.go:45-76`） | `checkGeneratedText(text string) (title, body string, err error)`。`checkResponse` はこれと `checkModelString` を順に呼ぶ | 2-1 |
| `parseTemplate(src templateSource, text string) (*template.Template, error)` | `parseTemplate[T any](src templateSource, text string) (checkedTemplate[T], error)` | 2-2 |
| `loadTemplate(src templateSource, embedded string) (*template.Template, error)` | `loadTemplate[T any](src templateSource, embedded string) (checkedTemplate[T], error)` | 2-2 |
| `expand(tmpl *template.Template, data templateData) (string, error)` | `checkedTemplate[T]` のメソッド `expand(data T) (string, error)` | 2-2 |
| `templateWriter` の `system`・`user`（`*template.Template`） | `checkedTemplate[templateData]` | 2-2 |
| `syntaxChecker{tree}` と `checkField` の `reflect.TypeFor[templateData]()` | `syntaxChecker` が、参照できるフィールドを引くためのデータ型を持ち、`checkField` はその型から引く | 2-2 |
| `provider.newClient(provider config.Provider, apiKey secret.Secret, model string, build func(deepseek.Options) (llm.LLMClient, error))`（0009 の PR-4 がマージ済みなら、0009 が作る `newClient(provider config.Provider, cfg config.Config, build builders)`） | `provider.newClient(provider config.Provider, model string, cfg config.Config, build …)`（`build` の型は変更前のまま。architecture §3.7） | 4-4 |
| `FilePublisher` の `wrapWriter`・`link`・`writeTemp`（`file.go:50-51`・`:108-148`） | 非公開の型 `atomicFile` のフィールドとメソッド | 5-2 |
| `job.precheckFileOutput`（規則の判定と `StageError` で包む処理） | 規則の判定だけをする関数と、`--out` のための `StageError` で包む呼び出し、一覧のファイルのための固定の文言で包む呼び出し | 5-4 |

### 1.5. 申し送りの文書の項目への対応

- `implementation_handoff.md` はこのタスクのディレクトリにない。対応する `I-NN` の項目はない。
- `design_handoff.md` の H-01 は、architecture §3.11 に記録されている。本計画では、ステップ 1-1・2-5 で実装する。

## 2. 実装ステップ (Implementation Steps)

ステップは `X-Y` 形式で表す（X: フェーズ番号、Y: フェーズ内の連番）。テスト関数名と AC の対応は §5 にまとめ、各ステップでは対象の AC だけを示す。テスト関数名は計画上の名前であり、実装で変えた場合は §5 を同じコミットで直す。各フェーズの完了条件は、最後のステップで壊して失敗することを確かめ、`make fmt` の後にグリーンゲートが通ることである。各 PR の区切りは `PR-N 作成ポイント` に示し、各 PR はその最後のステップまでで単独でグリーンゲートを通す（§3.2）。

### フェーズ 1: `internal/strictjson`

**対象ファイル**
- 変更: `internal/strictjson/strictjson.go`・`strictjson_test.go`

**タスク**
- [x] **ステップ 1-1**: `strictjson.go` に architecture §3.5.3 の `Object.CollectOnly(keys ...string) (map[string]Value, error)` と `Value.AsBool() (bool, error)` を加える（H-01）。`CollectOnly` は、指定しなかったキー（その重複を含む）が 1 つでもあれば拒否し、指定したキーの重複も拒否する。指定したキーが揃うことは求めない。エラーには、未知のキー用と重複したキー用に 1 つずつ非公開の静的エラーを設け、破られた規則だけを示す。キーの文字列は含めない。`AsBool` は `true`・`false` 以外（`null`・文字列・数、ゼロ値の `Value`）を拒否する。`Collect` は変えない。
- [x] **ステップ 1-2**: `strictjson_test.go` に `TestObjectCollectOnly`（未知のキー、未知のキーの重複、指定したキーの重複、指定したキーの一部の欠落は受理、拒否のエラーにキーに埋め込んだ目印が現れない）と `TestValueAsBool`（`true`・`false` の受理、`null`・`"true"`・`1`・ゼロ値の `Value` の拒否）を加える。既存の `TestObjectCollect` は変えない。
- [x] **ステップ 1-3**: 壊して失敗することを確かめ、コミットメッセージに書く。対象: `CollectOnly` が未知のキーを無視する（`TestObjectCollectOnly` の未知のキーの行）、指定したキーの重複を受理する（同、重複の行）、エラーにキーを `%q` で入れる（同、目印の検査）、`AsBool` が `null` を `false` として受理する（`TestValueAsBool`）。`make fmt` → `make test` → `make lint` を通す。

### PR-1 作成ポイント: strict key collection in internal/strictjson

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `feat(0010): add CollectOnly and AsBool to internal/strictjson`

**レビュー観点**: `CollectOnly` が未知のキーとその重複・指定したキーの重複を拒否し、指定したキーの欠落は受理すること（H-01） / 拒否のエラーにキーの文字列を含めず、破られた規則だけを非公開の静的エラーで示すこと / `AsBool` がゼロ値の `Value` と `null` を拒否すること / 既存の `Collect` と呼び出し元 4 か所の振る舞いを変えていないこと

**実装モデル要件**: standard

**判定理由**: 2 つの関数の追加で、規則と拒否の形は architecture §3.5.3 で決まっており、パネルモードのトリガー・競合する実装案・高リスクの手順のどれにも該当しないため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `internal/writer` の部品

**対象ファイル**
- 変更: `internal/writer/output.go`・`template.go`・`prompt.go`・`writer.go`・`errors.go`・`template_test.go`
- 新設: `internal/writer/length.go`・`length_test.go`、`revision.go`・`revision_test.go`、`review.go`・`review_test.go`（`//go:build test` の `_test.go`）

**タスク**
- [ ] **ステップ 2-1**: §1.4 のとおり `checkGeneratedText` を取り出す（architecture §3.6）。規則と検査の順序は変えない。`checkResponse` の doc コメントを、変更前 `// checkResponse validates the generated text, Model, and ModelVersion in a` / `// fixed order and returns the title and the body part (everything after the` / `// first line's "\n", passed through normalizeBody before its checks). Nothing` / `// else is repaired: a value that breaks a rule is rejected. Errors wrap ErrMalformedOutput and name the broken rule only,` / `// never a value, because the response is untrusted.` → 変更後 `// checkResponse validates a generation or shorten response: the generated text` / `// with checkGeneratedText, then Model and ModelVersion. It returns the title and` / `// the body part. Errors wrap ErrMalformedOutput and name the broken rule only,` / `// never a value, because the response is untrusted.` に変える。`checkGeneratedText` の doc コメントは、旧 doc の前半（タイトルと本文の部分の定義、補正しないこと）を引き継いで書く。`output_test.go` を変えずに通し、ここまでを 1 つのリファクタリングのコミットにする。
- [ ] **ステップ 2-2**: §1.4 のとおりテンプレートの検査と展開をデータ型の型引数で一般化する（architecture §3.4）。`checkedTemplate[T]` は `parseTemplate[T]` だけが作り、`expand` は `T` だけを受け取る。`internal/writer` の中では `checkedTemplate[T]{}` の複合リテラルも書けるので、「`parseTemplate[T]` だけが作る」はパッケージの中の約束であり、コンパイラは確かめない。コンパイラが確かめるのは、別のデータ型での展開ができないことである。検査の規則（サイズ・UTF-8・空白・`define`/`block`・構文と関数の allowlist）は変えず、参照できるフィールドの集合だけを `T` から引く。`templateWriter` は `checkedTemplate[templateData]` を持つ。`template_test.go:381`・`:404`・`:426` は型引数を付けるか、フィールドの型を渡す形に機械的に改め、期待は変えない。コメントを次のとおり変える。
  - `checkField`（`template.go:315-316`）: 変更前 `// checkField accepts a reference to one exported field of templateData and` / `// nothing deeper (.Title.Foo is rejected).` → 変更後 `// checkField accepts a reference to one exported field of the template's data` / `// type, promoted fields of an embedded struct included, and nothing deeper` / `// (.Title.Foo is rejected).`
  - `expand`（`prompt.go:96-99`）: 変更前 `// expand executes tmpl with data into a buffer capped at maxPromptBytes. An` → 変更後 `// expand executes the template with data into a buffer capped at maxPromptBytes. An`（後続の 3 行は変えない）
  - `templateData`（`prompt.go:17-19`）: 変更前 `// templateData is the only value passed to a prompt template. Its fields are` / `// the four values a template may reference; the template checks derive the` / `// allowed field names from this type.` → 変更後 `// templateData is the only value passed to a generation template. Its fields` / `// are the four values such a template may reference; the template checks` / `// derive the allowed field names from this type.`

  既存テストを変えずに通し（呼び出しの形の置き換えだけ）、1 つのリファクタリングのコミットにする。`internal/pipeline` の `TestPromptsREADMEMatchesContract` もこの時点では変えずに通る（`templateData` は残る）。

### PR-2 作成ポイント: writer refactors for post-generation steps

**対象ステップ**: 2-1 / 2-2

**推奨タイトル**: `refactor(0010): extract checkGeneratedText and type-parameterize writer templates`

**レビュー観点**: 2 つのコミットがそれぞれ独立したリファクタリングで、既存テストの期待値を変えず、テストの差分が呼び出しの形の機械的な置き換えだけであること / `checkField` が参照できるフィールドを型引数 `T` から引くようになっても、構文・関数の allowlist・サイズ・UTF-8 などの検査の規則が変わっていないこと / `checkedTemplate[T]` の `expand` が `T` 以外のデータ型を受け取れないこと（埋め込んだ構造体のフィールドの昇格の受理と `.templateData` の拒否は、データ型が揃う PR-4 の `TestTemplateDataFields` で確かめるので、本 PR では `reflect` の `FieldByName`・`IsExported` の使い方をコードで確かめる） / §1.4 の名前の変更と doc コメントの変更前後が計画どおりであること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-2 が、上書きテンプレートという信頼しない入力に対する検査（`checkField` のフィールドの allowlist）を型引数と `reflect` で作り直す孤立した高リスクの手順であり、振る舞いを保ったまま一般化する判断を要するため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
- [ ] **ステップ 2-3**: `length.go` に、本文の部分の字数を数える関数と、定数 `maxBodyChars = 4000`・`targetBodyChars = 3600` を置く（architecture §3.3）。`length_test.go` に `TestBodyChars` を置く。4,000 字と 4,001 字、改行を数えると 4,001 字以上になり、数えなければ 4,000 字になる入力（architecture §7.1）、Markdown の記号（`## `・`**`）と空白を数えること、サロゲートペアになる文字を 1 字と数えることを確かめる（AC-04 の数え方）。
- [ ] **ステップ 2-4**: `revision.go` に architecture §3.5.2 の `ReviewResult`・`Revision`・`RevisionAction`・`RevisionReason`・`WireValue`・`ReviewResult.Check` を、`errors.go` に `ErrInvalidReview` を置く。応答の値から `RevisionReason` への変換と `WireValue` は 1 つの対応表を使う。`Check` は、解析が受理した一覧なら必ず通る条件だけを確かめ、値をエラーに含めない。`revision_test.go` に `TestRevisionReasonWireValue`（5 つの値の往復、`ReasonUnknown` と範囲外の値が偽）と `TestReviewResultCheck`（列挙の外の `Action`・`Reason`、空の `Before`、`Action` と `After` の食い違いの両向き、空白だけの `Evidence`、各文字列の制御文字（`Evidence` の `\n` は受理）、`maxRevisions` を超える件数、モデル名の規則、エラーに値の目印が現れない）を置く。
- [ ] **ステップ 2-5**: `review.go` に、検証・推敲の応答の文字列から一覧を作る非公開の関数を置く（architecture §3.5.3、H-01）。手順は、`maxTextBytes` 以下であることの確認、`strictjson.ParseObject`、トップレベルの `CollectOnly("revisions")`、`AsArray`、`maxRevisions = 256` 以下であることの確認、各要素の `AsObject` と `CollectOnly`、architecture §3.5.3 の表の各規則の確認である。必須のキーと空でない文字列には既存の `strictjson.Required`・`RequiredString`・`OptionalString` を使う。どの拒否も `ErrMalformedOutput` を包み、規則と項目の番号だけを示す。部分的な一覧を返さない。`review_test.go` に `TestParseReviewResponse` を置き、architecture §3.5.3 の各規則について、その規則だけに反する応答の行を並べる（AC-12 の例、architecture §7.1 の追加の行、§7.3 の不正な UTF-8・対になっていないサロゲート・重複したキー）。境界は、項目 256 と 1 MiB ちょうどを受理し、257 と 1 MiB + 1 バイトを拒否する。拒否のエラーに応答の値の目印が現れないことも確かめる。
- [ ] **ステップ 2-6**: `review.go` に、一覧を文面の文字列に当てはめる非公開の関数を置く（architecture §3.5.4）。出現の数（重なり合う出現も数え、0 回・2 回以上を拒否）、範囲の重なり（接するのは受理）、元の文面の文字列に対する置き換え、`## ` で始まる行の列の比較の順に行う。拒否は `ErrMalformedOutput` を包み、項目の番号か、一致しなくなった最初の見出しの番号だけを示す。`review_test.go` に `TestApplyRevisions` を置き、次を確かめる（AC-10・AC-13・AC-14）。
  - AC-10 の `甲乙` の例と、項目の順序を入れ替えた結果が同じであること、削除の項目。
  - `ああ` と `あああ`: まずこの入力で `strings.Count` が 1 を返す（重ならない出現だけを数える方法では受理される）ことを確かめてから、関数が拒否することを確かめる（CLAUDE.md「A layered path」）。
  - 範囲が接する 2 項目の受理と、1 文字重なる 2 項目・同じ `before` の 2 項目の拒否。出現 0 回の拒否。
  - タイトルの行の中の項目の受理、見出しの中の項目・見出しの行を作る項目・コードフェンスの中の `## ` の行を変える項目の拒否。
- [ ] **ステップ 2-7**: 壊して失敗することを確かめ、コミットメッセージに書く。対象: 改行を数える（`TestBodyChars`）、重なり合う出現を `strings.Count` で数える（`TestApplyRevisions` の `あああ` の行）、置き換えを前の項目の結果の上で行う（`甲乙` の行）、接する範囲を重なりとする（接する行）、見出しの比較を外す（見出しの行）、項目の数の上限の確認を外す（`TestParseReviewResponse` の 257 の行）、`delete: false` を受理する（同）、`WireValue` の対応表に既定の値を足す（`TestRevisionReasonWireValue`）。`make fmt` → `make test` → `make lint` を通す。

### PR-3 作成ポイント: review response parsing and revision application

**対象ステップ**: 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0010): parse and apply review revisions in internal/writer`

**レビュー観点**: 検証・推敲の応答の解析が architecture §3.5.3 の各規則（1 MiB・256 項目の境界、未知と重複のキー、`delete` と `after` の組み合わせ、制御文字）を補正せずに拒否し、部分的な一覧を返さず、エラーに応答の値を含めないこと / 当てはめが重なり合う出現も数え（`ああ`/`あああ` の行が `strings.Count` との差を先に示していること）、範囲の重なりと見出しの列の変化を拒否し、元の文面の文字列に対して置き換えること / 本文の字数の数え方（改行・Markdown の記号・サロゲートペア）と 4,000／4,001 の境界 / `RevisionReason` の綴りが 1 つの対応表にだけあること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-5・2-6 が、LLM の応答という信頼しない入力の境界の検査と、重なりと見出しの判定を伴う当てはめを担う孤立した高リスクの手順であるため（セキュリティゲートや移行ではなく、パネルモードのトリガーには該当しない）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `internal/writer` の段階と `prompts`

**対象ファイル**
- 変更: `internal/writer/writer.go`・`errors.go`・`article.go`・`review.go`・`prompt.go`・`template.go`・`output.go`・`test_helpers.go`・`writer_test.go`・`template_test.go`・`article_test.go`
- 新設: `internal/writer/shorten.go`・`steps_test.go`（`//go:build test`）
- 変更: `internal/llm/testutil/mocks.go`・`mocks_test.go`
- 新設: `prompts/shorten_system.tmpl`・`shorten_user.tmpl`・`review_system.tmpl`・`review_user.tmpl`
- 変更: `prompts/prompts.go`・`prompts/README.md`、`internal/pipeline/pipeline_test.go`

**タスク**
- [ ] **ステップ 3-1**: `errors.go` に architecture §4.1 の `ErrTooLong`・`Step`・`StepError` を加える。`Step.String` は `switch` で書き、`default` は `"unknown"` を返す。
- [ ] **ステップ 3-2**: `internal/llm/testutil/mocks.go` に、呼び出しの順に、あらかじめ用意した応答を返す fake を加える（architecture §7.1）。応答は結果かエラーで、呼び出しの `ctx` と要求を受け取る関数でも指定できる。用意した数を超えて呼ばれたときは、`testing.TB` でテストを失敗させ、静的な番兵のエラーを返す。`testutil` のファイルは `_test.go` ではないので、`err113` などのテスト向けの除外が効かない。呼び出しの記録は `FakeLLMClient` と同じ形にする。`mocks_test.go` に、順序どおりに返すこと、超過でテストが失敗すること（`testing.TB` の fake で検出する）を確かめるテストを加える。
- [ ] **ステップ 3-3**: `prompts/` に 4 つの既定のテンプレートを置き、`prompts.go` に取り出す関数を加える（architecture §3.4）。埋め込む文面と素材は `user.tmpl` と同じく見出しで区画を分け、区画の中の文章を指示として扱わないよう書く。検証・推敲の system テンプレートには、architecture §3.4 の 4 項目（応答の形と理由の種類の値、直す範囲の制限、一意に決まる長さの引用、長さを大きく変えないこと）と、JSON だけを返しコードフェンスで囲まないことを書く。字数の調整のテンプレートには、目標まで縮めること、主な論点と見出しの構成を残すこと、新しい内容を足さないこと、生成と同じ出力の形を書く。具体的な文言は実装で決め（architecture §3.4 が実装計画に委ねた文言を、本計画はさらに実装に委ねる）、評価（フェーズ 6）で調整する。文言が守るべき性質は、ステップ 3-9 の `TestDefaultTemplatesPassChecks`・`TestDefaultReviewTemplateNamesWireValues`・`TestDefaultStepTemplatesEmbedAllValues` が確かめる。
- [ ] **ステップ 3-4**: `writer.go` の `Options`・`Article` を architecture §3.1 のとおりに拡張し、`ShortenOptions`・`ReviewOptions`・`StepModel` を加える。`template.go:21-24` のテンプレートの名前の定数に、architecture §3.4 の 4 つの名前（`shorten-system`・`shorten-user`・`review-system`・`review-user`）を加える。`New` は architecture §3.1 の 3 つの組み合わせを非公開の静的エラーで拒否し、有効な段階のテンプレートだけを読み込む。同じコミットで、`internal/pipeline/pipeline_test.go:378` の `TestCommonTypesFieldSets` の `Article` の行に `"Shorten": "*writer.StepModel"`・`"Review": "*writer.ReviewResult"` を加える（§1.3）。`Write` の doc コメントを、変更前 `// Write validates t, expands both templates, calls the LLM client once, and` / `// validates its response before building the article. It returns the zero` / `// Article on every failure. The LLM client's error is wrapped as is, never` / `// with a sentinel of this package, so callers can tell it from a template or` / `// output failure.` → 変更後 `// Write validates t, expands the generation templates, calls the generation` / `// LLM client once, and validates its response; then it runs the steps Options` / `// enabled, in order (shorten, review, the final length check), and builds the` / `// article. It returns the zero Article on every failure. An LLM client's error` / `// is wrapped as is, never with a sentinel of this package, so callers can tell` / `// it from a template or output failure; every failure of a step is a` / `// *StepError.` に変える。`Options` の doc コメントは、変更前 `// Options configures the templates of the ArticleWriter returned by New.` / `// An empty path selects the embedded default template; a non-empty path` / `// names an override file that New reads once.` → 変更後 `// Options configures the ArticleWriter returned by New. An empty template path` / `// selects the embedded default template; a non-empty path names an override` / `// file that New reads once. The zero value generates the article only:` / `// neither post-generation step runs.` に変える。`ShortenOptions`・`ReviewOptions`・`StepModel`・`Article` の doc コメントは architecture §3.1 のコードの文面を使う。`ShortenOptions` の「the rule of Options.SystemTemplatePath」は、上の `Options` の doc に書いた規則を指す。
- [ ] **ステップ 3-5**: `shorten.go` に字数の調整の段階を置く（architecture §3.3）。`shortenTemplateData` は `prompt.go` に置く。生成の `LLMClient` を 1 回呼び、`checkResponse` で検査し、縮めた後の字数が上限を超えれば `ErrTooLong`（上限と字数を含む）を返す。
- [ ] **ステップ 3-6**: `review.go` に検証・推敲の段階を置く（architecture §3.5.1）。`reviewTemplateData` は `prompt.go` に置き、`templateData` には生成で使った値をそのまま入れる。`ReviewOptions.Client` を 1 回呼び、ステップ 2-5 の解析、`checkModelString`、ステップ 2-6 の当てはめ、`checkGeneratedText` の順に行う。
- [ ] **ステップ 3-7**: `Write` を architecture §3.2 と図6 の順序にする。各段階の `Generate` の前に `ctx.Err()` を確かめ、段階の失敗はすべて `*StepError` で包む（生成の失敗は包まない）。最終の字数の確認は `Shorten.Enabled` のときだけ行い、その `ErrTooLong` は `StepReview` で包む（architecture §3.2・図6 の `E6`）。`newArticle` に `Shorten`・`Review` を渡す。`newArticle` の doc（`output.go:130-132`）を、変更前 `// newArticle builds the article from validated values. Body is the body part` / `// as checkResponse returned it, bodySeparator, then the source block, whose` / `// URL comes from the validated VideoID, never from the generated text.` → 変更後 `// newArticle builds the article from validated values. Body is the body part` / `// of the last checked text (from checkResponse or checkGeneratedText),` / `// bodySeparator, then the source block, whose URL comes from the validated` / `// VideoID, never from the generated text.` に変える。`expand` の展開のエラーの文字列（`prompt.go:104`）を、変更前 `"%w: %s prompt: %w of %d bytes; the template or the transcript values it embeds are too large"` → 変更後 `"%w: %s prompt: %w of %d bytes; the template or the values it embeds are too large"` に変える（字数の調整のプロンプトは字幕を埋め込まない）。
- [ ] **ステップ 3-8**: `article.go` の `CheckPublishable` を architecture §3.1 のとおりに広げる（段階のモデル名とモデルの版の `displayStringProblem`、`Review` の `ReviewResult.Check`）。doc の最後の文（`article.go:12`）を、変更前 `// ModelVersion may be empty; the other fields may not.` → 変更後 `// ModelVersion may be empty and Shorten and Review may be nil; the other fields` / `// may not be empty. A non-nil step result is checked like Model and` / `// ModelVersion, and Review also with ReviewResult.Check.` に変える。`article_test.go` に `TestArticleCheckPublishableSteps`（制御文字を含む `Shorten.Model`・`Review.Model.ModelVersion`、`Check` を通らない `Review` を拒否し、`ErrInvalidArticle` と `ErrInvalidReview` の両方に一致すること、段階のない記事の受理は既存のテストのまま）を加える。
- [ ] **ステップ 3-9**: テストを加える（AC-01〜AC-11・AC-13・AC-14・AC-23〜AC-26）。生成と字数の調整にはステップ 3-2 の fake を、検証・推敲には `FakeLLMClient` を使う。段階のテストの文面と応答の部品は `internal/writer/test_helpers.go` に置く。
  - `steps_test.go`: §5 の表の `steps_test.go` のテスト。各テストが確かめる条項は §5 の表に書く。`TestWriteStepContextDone` は、ステップ 3-2 の fake の関数の応答で `ctx` を取り消してから成功を返し、次の 3 行を確かめる。(1) 生成の中で取り消し、本文が上限を超える場合（字数の調整を呼ばない）。(2) 生成の中で取り消し、本文が上限以内の場合（検証・推敲を呼ばない）。(3) 字数の調整の中で取り消す場合（検証・推敲を呼ばない）。どの行も `errors.Is(err, context.Canceled)`、`*StepError`、ゼロ値の記事を確かめる。`TestWriteStepExpansionFailure` は、段階のテンプレートの展開の失敗（展開時のエラーと、検証・推敲のプロンプトが 1 MiB を超える場合）が `*StepError` と `ErrInvalidTemplate` に一致し、メッセージにテンプレートの名前があることを確かめる。
  - AC-06・AC-24 は、エラーのメッセージに上限・字数・段階の名前が含まれることを要件が求めるので、メッセージの部分文字列を確かめる。CLAUDE.md の「error の種類を文字列で判定しない」の例外であり、種類は `errors.Is`・`errors.AsType` で別に確かめる。テストのコメントにそのことを書く。
  - `review_test.go` の `甲乙`・`ああ` などの日本語の入力は、Go の文字列リテラルを英語にする規則に従い、`\u` のエスケープで書く。
  - `writer_test.go`: `TestNewStepOptions`（architecture §3.1 の 3 つの組み合わせの拒否（型付きの nil の `Client` を含む）と、有効な段階の上書きファイルの読み込み）。
  - `template_test.go`: `overrideTargets` は変えず（§1.3）、段階の 4 つのテンプレートの表（対象のデータ型が持つフィールドの名前を 1 つずつ持つ）を `test_helpers.go` に加える。`TestTemplateSyntaxAllowlist` と `TestInvalidTemplateErrorNamesSource` は、この表も回す。共通の表の行の中のフィールドの参照は、対象のデータ型が持つフィールドに置き換えて当てる（置き換えの方法は実装で決める）。拒否の行が「未知のフィールド」という別の理由で拒否され、意図した規則を確かめないままテストが成功することがないよう、置き換えた後の行のフィールドが対象のデータ型にあることを先に確かめる（CLAUDE.md「A layered path」）。`TestTemplateDataFields`（字数の調整の `.Transcript`・`.Title`、生成の `.ArticleBody`、検証・推敲の `.templateData` を構築時に拒否し、検証・推敲の `.Title`・`.ArticleBody` を受理する）、`TestDefaultTemplatesPassChecks` の 6 つへの拡張（4 つの新しいテンプレートは展開まで確かめる）、`TestDefaultReviewTemplateNamesWireValues`（既定の検証・推敲の system テンプレートが、`RevisionReason` の対応表の 5 つの値と、応答の 6 つのキー（`revisions`・`before`・`after`・`delete`・`reason`・`evidence`）をすべて含む。キーは解析と guard が共有するパッケージの定数にする）、`TestDefaultShortenTemplateHeadingInstruction`（既存の `TestDefaultSystemTemplateHeadingInstruction` と同じ形で、既定の字数の調整の system テンプレートが `# ` で始まる行で出力の形を示す）。
  - `writer_test.go`: 既存の `TestDefaultTemplatesEmbedAllValues`（`:127`）と同じ形の `TestDefaultStepTemplatesEmbedAllValues`（既定の字数の調整のプロンプトが文面の目印と上限・目標の値を埋め込み、既定の検証・推敲のプロンプトが文面の目印と 4 つの素材の目印を埋め込む）。
- [ ] **ステップ 3-10**: `prompts/README.md` に、2 つの段階のテンプレートと、それぞれで参照できる値の表を加える（architecture §3.4。`.Title` と `.ArticleTitle` を並べて書く）。同じコミットで、`internal/pipeline/pipeline_test.go` の `readWriterTemplateContract` と `TestPromptsREADMEMatchesContract` を、3 つのデータ型のそれぞれのフィールド（埋め込んだ構造体のフィールドを含む）と README の 3 つの表を比べる形に広げる。README の表の行を 1 つ消すと guard が失敗することを確かめる。
- [ ] **ステップ 3-11**: 壊して失敗することを確かめ、コミットメッセージに書く。対象: 本文が上限以内でも字数の調整を呼ぶ（`TestWriteStepOrder` の AC-01 の行）、検証・推敲に字数の調整の前の文面を渡す（同、AC-02 の行）、字数の調整のテンプレートのデータに `Transcript` を加えて既定のテンプレートで参照する（`TestShortenPrompt`）、検証・推敲の段階のエラーを `StepError` で包まない（`TestWriteStepFailures`）、段階の前の `ctx` の確認を外す（`TestWriteStepContextDone`）、最終の字数の確認を外す（`TestWriteTooLong` の AC-07 の行）、最終の字数の確認を字数の調整が無効でも行う（同、無効の行）、失敗時に前の段階の文面の記事を返す（`TestWriteStepFailures` のゼロ値の検査）、`New` が `Review.Enabled` が偽で `Client` があるのを受理する（`TestNewStepOptions`）、`checkField` を `templateData` に戻す（`TestTemplateDataFields`）、既定の検証・推敲のテンプレートから理由の値を 1 つ消す（`TestDefaultReviewTemplateNamesWireValues`）、`review_system.tmpl` に `{{printf}}` を書く（`TestDefaultTemplatesPassChecks`）、字数の調整の後の `ErrTooLong` を `StepReview` で包む（`TestWriteTooLong` の `Step` の検査）、段階のテンプレートの展開の失敗を `StepError` で包まない（`TestWriteStepExpansionFailure`）、段階に生成のモデル名を記録する（`TestWriteStepOrder` のモデルの検査）。`make fmt` → `make test` → `make lint` を通す。

### PR-4 作成ポイント: shorten and review steps in ArticleWriter

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11

**推奨タイトル**: `feat(0010): add the shorten and review steps to ArticleWriter`

**レビュー観点**: `Write` の順序（生成・字数の調整・検証・推敲・最終の字数の確認）と、各段階の前の `ctx.Err()`、段階の失敗の `*StepError` での包み方（最終の `ErrTooLong` は `StepReview`）、失敗時にゼロ値の記事を返すことが architecture §3.2・図6 と一致すること / `Options` のゼロ値で段階を行わず、`overrideTargets` と既存のテストを変えずに通ること（AC-27） / 4 つの既定のテンプレートが素材の区画の中の文章を指示として扱わないよう書かれ、検証・推敲のテンプレートが理由の値と応答のキーを `TestDefaultReviewTemplateNamesWireValues` で固定されていること / `TestCommonTypesFieldSets` と `TestPromptsREADMEMatchesContract` の guard の拡張が、変更と同じコミットにあること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-7 が、段階の順序・取り消し・失敗の伝播を持つ状態遷移を `Write` に組み込む孤立した複雑な手順であるため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 設定と構築

**対象ファイル**
- 変更: `internal/config/config.go`・`config_test.go`
- 変更: `internal/llm/provider/provider.go`・`provider_test.go`
- 条件付きで変更: `docs/tasks/0009_claude_llm_client/03_implementation_plan.md`（ステップ 4-5）

**タスク**
- [ ] **ステップ 4-1**（このフェーズの前提）: 0009 の PR-4（`internal/config` の `ProviderClaude`、`loadAPIKey` → `loadDeepSeekAPIKey` の改名、`newClient(provider, cfg, build)`）がマージ済みかを、`main` の `internal/config/config.go`・`internal/llm/provider/provider.go` で確かめ、結果（マージ済みか、確かめたコミット）を本ステップの下に記録する。以降のステップは、記録した状態の「変更前」を使う。§1.4 の `newClient` の行と、ステップ 4-2・4-4 の「変更前」の文面は、未マージの場合（`25d38bd`。コードは §1.3 の `4dd797d` と同じ）のものである。マージ済みの場合は、0009 の計画のステップ 4-1・4-2・4-5 が書いた形が「変更前」になる。
- [ ] **ステップ 4-2**: `config.go` を architecture §3.7 のとおりに変える。`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL` の定数、`Config` の 2 つのフィールド、`ReviewProvider`・`ReviewModel` を加える。文字列から `Provider` への変換は `loadProvider` から 1 つの関数に取り出し、生成と検証・推敲の読み込みの両方で使う。読み込みの順序は、生成のプロバイダとモデル、検証・推敲のプロバイダとモデル、プロバイダ固有の変数の順とする。DeepSeek の API キーは、生成と検証・推敲のどちらかが DeepSeek を使うときに必須とする（0009 がマージ済みなら、Claude の変数も同じ条件で読む）。コメントを次のとおり変える。
  - DeepSeek の API キーの読み込みの doc: 未マージなら、変更前 `// loadAPIKey validates DEEPSEEK_API_KEY, which is required when the provider is` / `// deepseek. An empty value is rejected however the provider is set; a non-empty` / `// value is kept only for deepseek.` → 変更後 `// loadAPIKey validates DEEPSEEK_API_KEY, which is required when the generation` / `// or the review provider is deepseek. An empty value is rejected however the` / `// providers are set; a non-empty value is kept only when one of them is` / `// deepseek.`。マージ済みなら、0009 のステップ 4-2 の変更後の文面 → 変更後 `// loadDeepSeekAPIKey validates DEEPSEEK_API_KEY, which is required when the` / `// generation or the review provider is deepseek. It is called only when one of` / `// them is, so the variable is never read, and never rejected, otherwise.`
  - `DeepSeekAPIKey`（`:111-112`）: 変更前 `// DeepSeekAPIKey returns the API key. It is non-zero when the provider is` / `// ProviderDeepSeek.` → 変更後 `// DeepSeekAPIKey returns the API key. It is non-zero when the generation or the` / `// review provider is ProviderDeepSeek.`
  - `Provider`（`:101`）: 変更前 `// Provider returns the LLM provider.` → 変更後 `// Provider returns the LLM provider of the generation.`。`Model`（`:106`）: 変更前 `// Model returns the model name. It is non-empty.` → 変更後 `// Model returns the model name of the generation. It is non-empty.`
  - `loadProvider`（`:172`）: 変更前 `// loadProvider validates YT2COLUMN_LLM_PROVIDER. Unset defaults to deepseek.` → 変更後 `// loadProvider validates YT2COLUMN_LLM_PROVIDER with parseProvider. Unset` / `// defaults to deepseek.`（取り出す関数の名前を `parseProvider` とした場合。名前を変えたら同じ名前にする）
- [ ] **ステップ 4-3**: `config_test.go` に `TestLoadReview` を加え、architecture §3.7 の表の 7 行を確かめる（AC-15・AC-16 の前半・AC-17・AC-18。AC-16 の前半は、`YT2COLUMN_REVIEW_PROVIDER=deepseek` と `YT2COLUMN_REVIEW_MODEL` を設定し、検証・推敲のモデルが `YT2COLUMN_REVIEW_MODEL`、生成のモデルが `YT2COLUMN_MODEL` のままであること）。拒否は `errors.Is` と `errors.AsType[*VarError]` の変数名で判定する。`TestLoadErrorsOmitValues`（`:276`）に、不正な `YT2COLUMN_REVIEW_PROVIDER` の値の目印がエラーに現れない行を加える。既存の行の期待は変えない。
- [ ] **ステップ 4-4**: `provider.go` に `NewReview` を加え、§1.4 のとおり `newClient` の引数を変える（architecture §3.7）。doc コメントを次のとおりにする。
  - `New`: 変更前 `// New builds the LLMClient for cfg.Provider(). It never reads environment` / `// variables; every value comes from the already-validated Config.` → 変更後 `// New builds the generation LLMClient from cfg.Provider() and cfg.Model(). It` / `// never reads environment variables; every value comes from the` / `// already-validated Config.`
  - `NewReview`（新設）: `// NewReview builds the review LLMClient from cfg.ReviewProvider() and` / `// cfg.ReviewModel(). It never reads environment variables; every value comes` / `// from the already-validated Config.`
  - `newClient`（未マージの場合の変更前）: `// newClient builds the adapter for provider. It takes the provider directly so` / `// a package test can exercise the unknown-provider branch, which a Config from` / `// Load cannot produce. build is the adapter constructor: production passes` / `// deepseek.New, a test passes the loopback constructor or a spy. An unknown` / `// provider returns errUnknownProvider without calling build.` → 変更後 `// newClient builds the adapter for provider and model, so New and NewReview` / `// share it; the other adapter values come from cfg. It takes the provider` / `// directly so a package test can exercise the unknown-provider branch, which a` / `// Config from Load cannot produce. build is the adapter constructor: production` / `// passes deepseek.New, a test passes the loopback constructor or a spy. An` / `// unknown provider returns errUnknownProvider without calling build.`（マージ済みなら、0009 のステップ 4-5 の変更後の文面の 1 文目を同じ趣旨に変える）

  `provider_test.go:132`・`:178` は新しい形への呼び出しの置き換えだけとする。`TestNewReviewUsesReviewModel` を加える。既存の `TestNewPaddedModel`（`:193`）と同じ形で、`YT2COLUMN_REVIEW_MODEL` だけを前に空白のある値にした設定で、`NewReview` が `deepseek.ErrPaddedModel` で失敗し、`New` が成功することを確かめる。あわせて、検証・推敲の 2 つの変数が未設定で `YT2COLUMN_MODEL` が前に空白のある値なら、`NewReview` も同じエラーで失敗することを確かめる（`NewReview` 自身を通して、検証・推敲のモデルの選び方を確かめるため）。
- [ ] **ステップ 4-5**: ステップ 4-1 の結果に応じて、次のどちらかを行う。結果を本ステップの下に記録する。
  - **マージ済みの場合:** ステップ 4-2・4-4 で 0009 の形を architecture §3.7 にそろえたことを確かめ、`config_test.go` に `TestLoadReviewProviderRequiresItsKey`（検証・推敲のプロバイダが生成と異なり、そのプロバイダの API キーの変数が未設定・空なら `ErrMissing`。AC-16 の後半）を加える。
  - **未マージの場合:** 0009 の `03_implementation_plan.md` を architecture §3.7 の形に合わせる。対象は、ステップ 4-1（改名の対象の doc）、4-2（読み込みの条件と doc の文面）、4-3（AC-16 の後半の `TestLoadReviewProviderRequiresItsKey` を加える）、4-5・4-6（`newClient` がモデルを引数で受け取る形）、4-8（`package_reference.md` の行。本タスクのステップ 5-9 と同じ行）、4-9（「選んだプロバイダのときだけ読む」の壊し方）、§1.3 の `config.go` の行番号と古くなるコメントの一覧である。何を作るかが変わる編集なので、0009 の計画の Status を `draft` に戻し、レビューアに再承認を依頼する（`requirements_process.md`「Editing an approved document」）。§5 の AC-16 の行に、引き継いだ先を書く。
- [ ] **ステップ 4-6**: 壊して失敗することを確かめ、コミットメッセージに書く。対象: `YT2COLUMN_REVIEW_PROVIDER` を設定したとき `YT2COLUMN_REVIEW_MODEL` を省略できる（`TestLoadReview` の AC-17 の行）、検証・推敲のモデルの既定を `YT2COLUMN_REVIEW_MODEL` だけの場合も `YT2COLUMN_MODEL` にする（AC-18 の行）、拒否の理由に値を入れる（`TestLoadErrorsOmitValues` の追加の行）、`NewReview` が `cfg.Model()` を渡す（`TestNewReviewUsesReviewModel`）。`make fmt` → `make test` → `make lint` を通す。

### PR-5 作成ポイント: review provider and model configuration

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6

**推奨タイトル**: `feat(0010): configure the review provider and model`

**レビュー観点**: ステップ 4-1 で確かめた 0009 の PR-4 の状態と、ステップ 4-5 で選んだ分岐が本ステップの下に記録され、未マージなら 0009 の計画が `draft` に戻されていること / 検証・推敲の変数の 7 つの組み合わせ（architecture §3.7）と、API キーをどちらかの段が使うときだけ必須にする条件 / `newClient` がモデルを引数で受け取り、`New` と `NewReview` がそれぞれ生成と検証・推敲のモデルを渡すこと / 拒否のエラーに変数の値が現れないこと / ステップ 4-5 で 0009 の計画を `draft` に戻した場合、本 PR は 0009 の再承認を待たずにマージし、再承認は 0009 の計画の Document Status で追うこと

**実装モデル要件**: frontier-recommended

**判定理由**: §1.3「タスク 0009 との順序」のとおり、ステップ 4-1・4-5 が 0009 の PR-4 の状態に応じて 2 つの実装の形（0009 の形にそろえるか、0009 の計画を改めるか）から選ぶ手順であり、実装の形が着手時まで決まらないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 出力と CLI

**対象ファイル**
- 変更: `internal/publisher/file.go`・`slack_message.go`・`test_helpers.go`・`file_test.go`
- 新設: `internal/publisher/reviewlog.go`・`reviewlog_test.go`（`//go:build test`）
- 変更: `internal/job/job.go`・`job_test.go`・`test_helpers.go`
- 変更: `cmd/yt2column/run.go`・`run_test.go`・`signal_test.go`・`test_helpers.go`・`test_helpers_integration.go`・`docs_test.go`・`integration_test.go`・`integration_slack_test.go`、`internal/publisher/slack_message_test.go`
- 変更: `README.md`・`docs/dev/project_overview.md`・`docs/dev/security.md`・`docs/dev/developer_guide/package_reference.md`・`CLAUDE.md`・`docs/tasks/0008_column_prompt/check_article.py`

**タスク**
- [ ] **ステップ 5-1**: `renderArticle` に段階のモデルの行を加え、`(none)` への置き換えを 1 つの関数にする（architecture §3.8）。`slack_message.go:29` の UTF-8 の確認の対象に、段階のモデル名とモデルの版を加える。`file_test.go` に `TestRenderArticleSteps`（段階の行は字数の調整・検証・推敲の順で、呼ばなかった段階の行はなく、段階のモデルの版が空なら `(none)`）を加える。段階のない記事の書式は、既存の `TestFilePublisherWritesArticle`（`file_test.go:64`）・`TestFilePublisherEmptyModelVersion`（`:89`）が文字列で確かめているので、同じ行を加えない。`slack_message_test.go:296` の `TestSlackPrepareRejectsInvalidArticle` に、段階のモデル名とモデルの版が不正な UTF-8 の行を、既存の `checkPublishableOK` の形（まず `CheckPublishable` だけでは拒否されないことを確かめる）で加える。`testutil/integration.go:165-170` の写しは変えない。
- [ ] **ステップ 5-2**: §1.4 のとおり `atomicFile` を取り出す。`FilePublisher` の振る舞い（`KeptFileError` を含む）と一時ファイルの接頭辞 `.yt2column-` は変えない。`test_helpers.go` の `newFilePublisherWithSeams` は差し替え口の移動に合わせて改め、`file_test.go` は変えずに通す。`atomicFile` に移る `tempPrefix`・`articleFileMode`・`//nolint:gosec` の理由（`file.go:27-28`・`:34-37`・`:122`）と `file.go:49` の `// Seams for tests; NewFilePublisher sets the production values.` は、このコミットでは記事だけを書くので文言を変えない。次の 3 点は、ステップ 5-3 で一覧のファイルにも `atomicFile` を使うコミットで変える。`atomicFile` を作る側が値（接頭辞）を渡す形にする。`articleFileMode` の doc を、変更前 `// articleFileMode is the permission of the published file. An article is a` / `// non-secret document built from a public video, and the process umask is not` / `// applied because the standard library cannot read it without changing it.` → 変更後 `// articleFileMode is the permission of the published file and the review log.` / `// Both are non-secret documents built from a public video, and the process` / `// umask is not applied because the standard library cannot read it without` / `// changing it.` に変える。`//nolint:gosec` の理由を `// the article and the review log are non-secret documents built from a public video` に変える。ステップ 5-2 のコミットは、取り出しだけのリファクタリングのコミットにする。
- [ ] **ステップ 5-3**: `reviewlog.go` に architecture §3.8 の `ReviewLogWriter`・`NewReviewLogWriter`・`Path`・`Write` と、一覧のファイルの書式を置く。一時ファイルの接頭辞は `.yt2column-reviewlog-` とし、`link(2)` の失敗でも一時ファイルを消す。既存の `errEmptyPath`・`classifyLinkError`（`ErrOutputExists` の付与）・`syncDir`、ステップ 5-1 の `(none)` の関数を使う。`reviewlog_test.go` に `TestReviewLogWriterFormat`（空の一覧、削除の項目、改行を含む根拠の各行の `> `、モデルの版が空の `(none)`。ファイル全体を文字列のリテラルと比べる）、`TestReviewLogWriterNoOverwrite`（`ErrOutputExists`）、`TestReviewLogWriterLinkFailure`（一時ファイルが残らず、`KeptFileError` でない）、`TestReviewLogWriterRejects`（`Review` が nil、`Check` を通らない一覧）、`TestNewReviewLogWriterEmptyPath` を置く（AC-20）。
- [ ] **ステップ 5-4**: `internal/job` に architecture §3.9 の `Request.ReviewLog`・`ReviewLogError`・事前確認・書き出しを加える。§1.4 のとおり `precheckFileOutput` の規則の判定を分ける。doc コメントを次のとおり変える。
  - パッケージ（`job.go:1-3`）: 変更前 `// Package job runs one end-to-end job: the file-output pre-checks, the cache` / `// directory lock, a prune, the pipeline, and the cache cleanup. It holds no` / `// terminal or flag handling, so a caller other than the CLI can use it.` → 変更後 `// Package job runs one end-to-end job: the file-output pre-checks, the cache` / `// directory lock, a prune, the pipeline, the cache cleanup, and the review` / `// log. It holds no terminal or flag handling, so a caller other than the CLI` / `// can use it.`
  - `Run`（`job.go:83-86`）: 変更前 `// Run executes one run: it validates the request, pre-checks a file output's` / `// path, takes the cache directory lock, prunes dangling entries, runs the` / `// pipeline, and removes the video's cache unless KeepCache is set. A nil error` / `// means the article was published.` → 変更後 `// Run executes one run: it validates the request, pre-checks a file output's` / `// path and the review log path, takes the cache directory lock, prunes` / `// dangling entries, runs the pipeline, removes the video's cache unless` / `// KeepCache is set, and writes the review log when ReviewLog is set. A nil` / `// error means the article was published and the review log, when requested,` / `// was written; a *ReviewLogError means the article was published and only` / `// writing the review log failed.`

  `job_test.go` に `TestRunReviewLogWritten`、`TestRunReviewLogPrecheck`（既存のパスと、親がディレクトリでないパスで、パイプラインを動かさず、エラーが `pipeline.StageError` でない）、`TestRunReviewLogNotWrittenOnFailure`（記事の生成の失敗と、`Review` を持つ記事の投稿の失敗の 2 行。後者は、投稿の前に書き出す誤りを区別するため）、`TestRunReviewLogWriteFailure`（§1.3 の fake で起こし、`*ReviewLogError`・`Result.Article`・キャッシュが削除されたこと・`Result.Warnings` を確かめる）を加える（AC-20）。

  このステップの最後に、ステップ 5-1〜5-4 のテストについて、壊して失敗することを確かめ、コミットメッセージに書く（PR-6 の完了条件）。対象: 呼ばなかった段階の行を書く（`TestRenderArticleSteps`）、段階のモデル名の UTF-8 の確認を外す（`TestSlackPrepareRejectsInvalidArticle` の追加の行）、`ReviewLogWriter` が `link(2)` の失敗で一時ファイルを残す（`TestReviewLogWriterLinkFailure`）、一覧のファイルを投稿の前に書き出す（`TestRunReviewLogNotWrittenOnFailure` の投稿の失敗の行）、`job` が書き出しをキャッシュの削除の前にし、書き出しの失敗で削除せずに戻る（`TestRunReviewLogWriteFailure` のキャッシュの検査）。`make fmt` → `make test` → `make lint` を通す。

### PR-6 作成ポイント: model lines and review log output in publisher and job

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4

**推奨タイトル**: `feat(0010): render step model lines and write the review log`

**レビュー観点**: ステップ 5-2 の `atomicFile` の取り出しが独立したリファクタリングのコミットで、`file_test.go` を変えずに通り、`KeptFileError` と接頭辞 `.yt2column-` を保つこと / `ReviewLogWriter` が上書きせず（`ErrOutputExists`）、`link(2)` の失敗で一時ファイルを残さないこと / `job.Run` が一覧のファイルを投稿の後・キャッシュの削除の後に書き出し、その失敗を `*ReviewLogError` で返し、事前確認の失敗を `pipeline.StageError` で包まないこと / 段階のない記事の `--out` の書式が変わらないこと（AC-19）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 5-4 が、投稿・キャッシュの削除・一覧のファイルの書き出しの順序と、記事を出した後の部分的な失敗（`ReviewLogError`）の扱いを決める孤立した失敗処理の手順であり、ステップ 5-3 も `link(2)` の失敗時の後始末を伴うため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
- [ ] **ステップ 5-5**: `cmd/yt2column/run.go` に architecture §3.9 のフラグ、手順 A2・A4 の検査、手順 A5 の構築（`deps` の `newReviewLLMClient`・`newReviewLogWriter`）を加える。同じコミットで次を行う。
  - README のフラグの表に 7 つのフラグの行を加える（`TestREADMEDocumentsCLI` が求める）。
  - `newRunEnv` と `testDeps` の `newReviewLLMClient` を、`refusingSlackPublisher` と同じ形の、何も作らずにエラーを返す関数にする。段階を無効にする引数（`--no-shorten`・`--no-review`）を `test_helpers_integration.go` に 1 つの値として置く。
  - 手順 A5 まで進む既存のすべてのテストで段階を無効にする（AC-27。§1.3）。`runEnv.cliArgs` と `childSetup.args` は、テストが段階を使うと宣言しない限りその値を先頭に置く。直書きの `run_test.go:404`・`signal_test.go:180`・`integration_test.go`・`integration_slack_test.go:46` には値を加える。`Write` まで進まないテスト（`executionPathRows` の `exitFailure` の行など）も対象である。テストの期待は変えない。
  - `integration_test.go` の `generateCounter` が `newReviewLLMClient` で作った `LLMClient` も数えるようにし、`Generate` が 1 回であることの確認（`:152`）が段階を行わなかったことを示すようにする。
  - `TestTestDepsSlackPublisherDoesNotSend`（`run_test.go:1410`）と同じ形で、`newRunEnv` と `testDeps` の既定の `newReviewLLMClient` が何も作らないことを確かめるテスト（`TestTestDepsReviewClientBuildsNothing`）を加える。
- [ ] **ステップ 5-6**: 手順 C の要約の行と、`reportRunError` の案内を architecture §3.9・§4.2 のとおりに加える。`*job.ReviewLogError` のとき、`run` は要約を先に書き、`reportRunError` は `ReviewLogError` を `StageError` より先に判定する。段階は `errors.AsType[*writer.StepError]` で取り出す。`reportRunError` の doc（`run.go:373-376`）を、変更前 `// reportRunError reports a job.Run failure, naming the pipeline stage when` / `// there is one, and adds the hint that matches the failure. A deadline is` → 変更後 `// reportRunError reports a job.Run failure, naming the pipeline stage when` / `// there is one (a review log failure after the publish is reported first, on` / `// its own), and adds the hint that matches the failure. A deadline is`に変える（後続の 2 行は変えない）。手順 A5 のコメント（`run.go:217-218`）も、変更前 `// A5: build the stages. writer.New only reads the template files, and` / `// neither publisher constructor touches its destination.` → 変更後 `// A5: build the stages. writer.New only reads the template files, and` / `// neither the publisher constructors nor the review log writer touches its` / `// destination.` に変える。
- [ ] **ステップ 5-7**: `run_test.go` に、architecture §7.2 の CLI の組み立ての各項目を確かめるテストを加える（§5 の表の `run_test.go` の行。AC-19〜AC-23・AC-27）。あわせて次のことも確かめる。
  - 既定で 2 つの段階を行い、検証・推敲に検証・推敲の `LLMClient` を渡すこと（`TestRunDefaultSteps`。生成と検証・推敲を別の fake にし、本文が上限を超えるとき生成の fake が 2 回、検証・推敲の fake が 1 回、`newReviewLLMClient` が 1 回呼ばれる）。
  - 使い方の誤り（空の `--review-log`、`filepath.Clean` で `--out` と同じ `--review-log`、無効にした段階のテンプレートの上書き。既存の `usageRow` の形）。
  - キャッシュディレクトリの中の `--review-log`（既存の `insideCacheRow` の形）。
  - `ErrTooLong` のときに `--no-shorten` の案内が出ること（`TestRunTooLongHint`）。
  - 検証・推敲の段階の `ErrMalformedOutput` のときだけ `--no-review` の案内が出ること（`TestRunReviewMalformedHint`。生成と字数の調整の `ErrMalformedOutput` では出ない）。

  セキュリティ（architecture §7.3）として、検証・推敲の fake が DeepSeek の API キーを含むエラーを返したとき、標準エラー出力にキーとその末尾 8 文字が現れないことを、既存の `requireSafeOutput`・`secretStrings` で確かめる（`TestRunReviewDoesNotLeakAPIKey`）。新しい実行の経路（一覧のファイルの書き出しの失敗、`--review-log` と `--no-review` の同時指定、段階の失敗とその案内）は、`run_test.go:419` の `executionPathRows` にも行を加え、既存の経路ごとの保証（秘密情報が出ないこと、エスケープ、使い方の誤りでの副作用のなさ）を当てる。
- [ ] **ステップ 5-8**: `TestRunHelp`（`run_test.go:1063`）の一覧に 7 つのフラグを加える。ステップ 5-5 のコミットで `make lint` の `go vet -tags integration` が通り、`integration` タグのビルドで段階を無効にする値が使えることを確かめる。

  このステップの最後に、ステップ 5-5〜5-8 のテストについて、壊して失敗することを確かめ、コミットメッセージに書く（PR-7 の完了条件）。対象: CLI が検証・推敲に生成の `LLMClient` を渡す（`TestRunDefaultSteps`）、`--review-log` のキャッシュディレクトリの検査を外す（`insideCacheRow` の追加の行）、`--review-log` と `--no-review` の検査を外す（`TestRunReviewLogWithNoReview`）、`ReviewLogError` を判定せず「the run failed:」の分岐に落とす（`TestRunReviewLogFailure` の、その文言が出ないことの検査）、`--no-review` の案内を `StepShorten` でも出す（`TestRunReviewMalformedHint`）。`make fmt` → `make test` → `make lint` を通す。ステップ 5-5 で README を変えるので、`make ext-test` も通す。

### PR-7 作成ポイント: CLI flags and default-on steps

**対象ステップ**: 5-5 / 5-6 / 5-7 / 5-8

**推奨タイトル**: `feat(0010): enable the shorten and review steps in the CLI by default`

**レビュー観点**: 既定で段階を有効にする変更と同じコミットで、手順 A5 まで進む既存のすべての CLI のテスト（直書きの `run_test.go:404`・`signal_test.go:180`・`integration_*` を含む）が段階を無効にする値を使い、期待を変えていないこと（AC-27） / `newRunEnv`・`testDeps` の `newReviewLLMClient` が何も作らず、`generateCounter` が検証・推敲の `LLMClient` も数えること / 手順 A2・A4 の使い方の誤り（空の `--review-log`、`--out` と同じパス、無効にした段階のテンプレートの上書き、`--review-log` と `--no-review`）が LLM を呼ばずに `exitUsage` になること / `ReviewLogError` の報告（要約が先、`reportRunError` が `StageError` より先に判定）と、`executionPathRows` の新しい経路で秘密情報が出ないこと / 0009 の PR-4（CLI の配線）がマージ済みなら、その後に加わった CLI のテストの起動箇所も段階を無効にする値を使い、`TestRunReviewDoesNotLeakAPIKey` の対象の秘密情報に 0009 が加えたキーも入っていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 5-5 が CLI の既定の振る舞いを変える（2 つの段階を既定で有効にする）のと同時に、既存の CLI のテストの多数（約 40 か所の引数と子プロセスの起動）を段階を無効にする形へ移す移行の手順であり、`mkplan.md` ステップ 8 のパネルモードのトリガー（多数のテストの更新を伴う移行）に該当するため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
- [ ] **ステップ 5-9**: architecture §3.10 の文書を更新する（§1.3 の「文書の更新対象」のすべての箇所）。
  - `README.md`: 2 つの段階の説明、設定の表の 2 行と `DEEPSEEK_API_KEY` の行（検証・推敲のプロバイダが `deepseek` の場合も必須）、終了コード 1 の意味（記事の書き出し・投稿の後に一覧のファイルの書き出しだけが失敗した場合）、更新時の注意 5 項目（architecture §3.10）、`--no-review` でも検証・推敲の設定を検査すること。
  - `project_overview.md`: パイプラインの 2 つの段階と `:31` の `Article` のフィールド、設定の表の 2 行と `:97` の `DEEPSEEK_API_KEY` の行、`:76` の `internal/strictjson` の利用者（§1.3 の 4 つ）。
  - `security.md` §4・§6。`CLAUDE.md:94-95`（`internal/strictjson` の利用者を §1.3 の 4 つにする）。
  - `package_reference.md` の `:15`・`:18`・`:21`・`:27`・`:28`・`:29`・`:30`・`:32`・`:36` の行。0009 の計画のステップ 4-8 も同じ行を変えるので、後に実装する側が先の変更を残して書き足す。

  設定の表の行を変えるコミットで、`docs_test.go` の `configDocRows` に 2 つの変数を加え、`DEEPSEEK_API_KEY` の行の文言を表の新しい文言に合わせる（当てる文言は requirements F-004 の表現から実装で決める）。各記述を実装と照らし、照らした箇所をコミットメッセージに書く。`make ext-test` を通す。
- [ ] **ステップ 5-10**: `check_article.py` の `HEADER_RE` を、生成の 2 行の後に段階のモデルの行（`- Shorten model:`・`- Shorten model version:`・`- Review model:`・`- Review model version:`）を任意で受け付ける形にする。0008 の記事と評価のキャッシュはリポジトリにない（`docs/tasks/0008_column_prompt/v5/` はテンプレートだけで、キャッシュは 0008 §2.4 のとおりリポジトリの外）。そのため、一時ディレクトリに、短い合成した記事（段階の行がないものと、段階の行を加えたもの）と、スクリプトが読む最小のキャッシュ（`<video ID>.current` に `a` か `b` を書き、そのスロットのディレクトリ `<video ID>.a` などに、数の `duration` を持つ `<video ID>.info.json` を置く。`check_article.py:36-42`・`:133`）を作る。段階の行がある記事とない記事の両方でスクリプトを実行し、同じ字数と判定が出ることを確かめる。実行したコマンドと出力をコミットメッセージに書く。
- [ ] **ステップ 5-11**: ステップ 5-9 のテストについて、壊して失敗することを確かめ、コミットメッセージに書く（ステップ 5-1〜5-8 の壊す対象は、ステップ 5-4・5-8 で確かめた）。対象: `configDocRows` の行に対応する README の行を消す（`TestREADMEDocumentsCLI`）。`make fmt` → `make test` → `make lint` → `make ext-test` を通す。

### PR-8 作成ポイント: documentation and the evaluation script

**対象ステップ**: 5-9 / 5-10 / 5-11

**推奨タイトル**: `feat(0010): document post-generation steps and update check_article.py`

**レビュー観点**: 0009 の PR-7（文書）がマージ済みなら、README・`project_overview.md`・`security.md`・`CLAUDE.md`・`configDocRows` の重なる行（`DEEPSEEK_API_KEY` の行など）で、先の変更を残して書き足していること / `README.md`・`project_overview.md`・`security.md`・`package_reference.md`・`CLAUDE.md` の各記述が実装と照らされ、照らした箇所がコミットメッセージにあること（§1.3「文書の更新対象」のすべての行） / `DEEPSEEK_API_KEY` の行と `configDocRows` が、検証・推敲のプロバイダが `deepseek` の場合も必須であることを表すこと / `check_article.py` の `HEADER_RE` が段階の行のある記事とない記事で同じ字数と判定を出すことを、合成した記事と最小のキャッシュで実行して示していること / 0009 の計画のステップ 4-8 と同じ `package_reference.md` の行を、先の変更を残して書き足していること

**実装モデル要件**: standard

**判定理由**: 文書と評価用スクリプトの正規表現の更新だけで、パネルモードのトリガー・競合する実装案・高リスクの手順のどれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: 評価

**対象ファイル**
- 新設: `docs/tasks/0010_post_generation_review/02_evaluation.md`
- 変更: `cmd/yt2column/docs_test.go`

**タスク**
- [ ] **ステップ 6-1**: `02_evaluation.md` に評価の手順を書く（requirements F-008、architecture §8 のフェーズ 6）。0008 の [02_evaluation.md](../0008_column_prompt/02_evaluation.md) の §2（手順）・§3.2（動画）・§4（判定者と校正）を参照し、同じ部分は写さない。本書に書くのは次のことである。v5 のテンプレートの指定、段階なし（`--no-shorten --no-review`）と段階ありの記事の生成、`--review-log` の指定、記録の表（AC-28〜AC-30 の項目と、architecture §5.4 の失敗の種類）、料金が 2 倍になる時間帯を避けること（requirements §5）。
- [ ] **ステップ 6-2**: `cmd/yt2column/docs_test.go` に `TestReviewEvaluationRecords` を加える。ステップ 6-3 にチェックが入った後は、`02_evaluation.md` の記録が、0008 の §3.2 の 5 本の動画のそれぞれについて、v5 のテンプレートを使ったこと、F-006 の観点ごとの判定、一覧の各項目の判定と段階が入れた新しい根拠のずれの有無、検証・推敲のプロバイダとモデル、呼び出しの回数、所要時間を持つことを求める。動画の ID は 0008 の文書から読み、テストに写さない。ステップの区画と節は、既存の `stepBlock`・`docSection`（`docs_test.go:365`・`:342`）で取り出す。比べる文言は実装で決める。ステップ 6-3 にチェックが入る前は何も求めないことと、記録を 1 行消すと失敗することを確かめる。

### PR-9 作成ポイント: evaluation procedure and record guard

**対象ステップ**: 6-1 / 6-2

**推奨タイトル**: `feat(0010): add the post-generation review evaluation procedure and its record guard`

**レビュー観点**: `02_evaluation.md` が 0008 の §2・§3.2・§4 を参照し、同じ部分を写していないこと / 記録の表が AC-28〜AC-30 の項目と architecture §5.4 の失敗の種類を持つこと / `TestReviewEvaluationRecords` が動画の ID を 0008 の文書から読み、ステップ 6-3 にチェックが入る前は何も求めず、入った後に記録の 1 行の欠落で失敗すること

**実装モデル要件**: standard

**判定理由**: 手順の文書と、既存の `stepBlock`・`docSection` を使う guard テストの追加だけで、パネルモードのトリガー・競合する実装案・高リスクの手順のどれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
- [ ] **ステップ 6-3**: 検証・推敲のモデルを生成と同じ deepseek-flash にして、5 本の動画で評価し、`02_evaluation.md` に記録する（AC-28〜AC-30）。実 API の実行は、実行のたびに人の承認を得る（requirements §5、CLAUDE.md「Tool Execution Safety」）。
- [ ] **ステップ 6-4**: 0009 の Claude アダプタが使えるようになっていれば、検証・推敲を Claude にして同じ評価をし、比べる。使えなければ、そのことと理由を `02_evaluation.md` に書く。
- [ ] **ステップ 6-5**: 評価の結論（2 つの段階を含めた構成で 0008 の合格条件を満たせる見込みか、0008 の評価を再開するか）と、失敗の種類ごとの件数を `02_evaluation.md` に書く。テンプレートの文言や `targetBodyChars` を評価の結果で変える場合は、別のコミットにし、変えた後の評価も記録する。誤認識の写しが段階の後も残る場合は、別の issue にすることを書く（requirements §2.3）。

### PR-10 作成ポイント: evaluation records with real APIs

**対象ステップ**: 6-3 / 6-4 / 6-5

**推奨タイトル**: `feat(0010): record the post-generation review evaluation on five videos`

**レビュー観点**: 5 本の動画のそれぞれに、v5 のテンプレート・F-006 の観点ごとの判定・一覧の各項目の判定と新しい根拠のずれの有無・プロバイダとモデル・呼び出しの回数・所要時間が記録され、`TestReviewEvaluationRecords` が通ること（本 PR が `docs/` だけを変える場合（ステップ 6-5 でテンプレートや `targetBodyChars` を変えない場合）は、CI が Go のテストを省く。ステップ 6-3 にチェックを入れた後に手元で `make test` を実行し、その結果を PR の説明に書く） / 実 API の実行ごとに人の承認を得て、料金が 2 倍になる時間帯を避けたこと / テンプレートの文言や `targetBodyChars` を変えた場合は別のコミットで、変えた後の評価も記録されていること / 結論と失敗の種類ごとの件数、0008 の評価を再開するかの判断が根拠とともに書かれていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 6-3・6-4 が実 API（DeepSeek と、使えれば Claude）という外部リソースに触れる評価の実行であり、ステップ 6-5 がその結果からテンプレートの文言と `targetBodyChars` を調整する前例のない判断を含むため（`mkplan.md` ステップ 8 のパネルモードのトリガーの、外部リソースの面に該当）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 | 完了条件 |
|---|---|---|---|
| M1 | フェーズ 1 | `CollectOnly`・`AsBool` とテスト | `make test`・`make lint` が通る |
| M2 | フェーズ 2 | `checkGeneratedText` と型引数のテンプレート（リファクタリングのコミット）、本文の字数、一覧の型、応答の検査、当てはめとテスト | 同上・リファクタリングのコミットで既存テストの期待が変わらない |
| M3 | フェーズ 3 | 2 つの段階、`Write` の順序、4 つの既定のテンプレート、`prompts/README.md` と guard | 同上 |
| M4 | フェーズ 4 | 検証・推敲の設定、`provider.NewReview`、0009 との整合（ステップ 4-1・4-5） | 同上・ステップ 4-1・4-5 の結果が記録されている |
| M5 | フェーズ 5 | モデルの行、`ReviewLogWriter`、`job`、CLI、文書、`check_article.py` | 同上・`make ext-test` が通る |
| M6 | フェーズ 6 | `02_evaluation.md` の手順と記録、guard | 同上・評価の記録と結論がある |

### 3.2. PR 構成

PR は、フェーズをおおむね関心事ごとに分け、それぞれが単独でグリーンゲートを通せる単位とする。フェーズ 2 は、既存の振る舞いを変えないリファクタリング（PR-2）と、検証・推敲の応答の検査と当てはめ（PR-3）に分ける。前者は既存テストを変えないことだけを、後者は信頼しない入力の境界を、それぞれ集中してレビューできる。フェーズ 5 は、`internal/publisher`・`internal/job` の出力（PR-6）、CLI の既定の変更と既存のテストの移行（PR-7）、文書と評価用スクリプト（PR-8）に分ける。PR-7 の既定の変更は多数のテストに触れるため、出力の層の変更と混ぜずに隔離する。フェーズ 5 の壊して確かめる手順は、各 PR の最後のステップ（5-4・5-8・5-11）に分けて置き、ステップの番号は変えない。フェーズ 6 は、実 API を使わない手順と guard（PR-9）と、実 API の評価の記録（PR-10）に分け、承認を待つ評価の実行が手順のマージを止めないようにする。PR-4 と PR-7 は大きいが分けない。PR-4 の段階のテンプレートの検査（ステップ 3-9）は、ステップ 3-5・3-6 が置くデータ型を使う。PR-7 の各ステップは、段階の既定の有効化と `--review-log` の両方を扱う。どちらも、分けるにはステップの内容を組み替える必要がある。代わりに、判定理由で高リスクの手順（ステップ 3-7・5-5）を名指し、レビュー観点でその内容を挙げる。

`internal/` の変更が `cmd/` に先行する順序は、`internal/writer`（PR-2〜PR-4）・`internal/config`・`internal/llm/provider`（PR-5）・`internal/publisher`・`internal/job`（PR-6）が、それらを使う `cmd/yt2column` の変更（PR-7）に先行することで満たす。PR-4 までは `Options` のゼロ値で段階を行わず、PR-6 の追加は CLI から使われない。PR-5 の後は、`config.Load` が検証・推敲の 2 つの変数を検査する（AC-17）が、2 つの変数が未設定なら CLI の振る舞いは変わらない。README のフラグの表は PR-7 で更新する（`TestREADMEDocumentsCLI` が求める）。それ以外の文書の更新（設定の表、段階の説明、`DEEPSEEK_API_KEY` の行など）は PR-8 にまとめるので、PR-5〜PR-7 のマージから PR-8 のマージまでは、それらが新しい変数と既定の段階を説明しない。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | `strictjson` の `CollectOnly`・`AsBool` とテスト | standard |
| PR-2 | 2-1 / 2-2 | `checkGeneratedText` の取り出しと、型引数によるテンプレートの検査と展開の一般化（2 つのリファクタリングのコミット） | frontier-recommended |
| PR-3 | 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | 本文の字数、一覧の型と `Check`、検証・推敲の応答の解析、一覧の当てはめとテスト | frontier-recommended |
| PR-4 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11 | `StepError`、順序どおりに応答を返す fake、4 つの既定のテンプレート、`Options`・`Article` の拡張、2 つの段階と `Write` の順序、`CheckPublishable`、`prompts/README.md` と guard | frontier-recommended |
| PR-5 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 | 検証・推敲のプロバイダとモデルの設定、`provider.NewReview`、0009 との整合 | frontier-recommended |
| PR-6 | 5-1 / 5-2 / 5-3 / 5-4 | `--out`・Webhook の段階のモデルの行、`atomicFile` の取り出し、`ReviewLogWriter`、`job` の一覧のファイルの事前確認と書き出し | frontier-recommended |
| PR-7 | 5-5 / 5-6 / 5-7 / 5-8 | CLI のフラグと検査、段階の既定の有効化、既存の CLI のテストの移行、要約と案内 | frontier-required |
| PR-8 | 5-9 / 5-10 / 5-11 | README・`project_overview.md`・`security.md`・`package_reference.md`・`CLAUDE.md` の更新、`check_article.py` | standard |
| PR-9 | 6-1 / 6-2 | `02_evaluation.md` の評価の手順と `TestReviewEvaluationRecords` | standard |
| PR-10 | 6-3 / 6-4 / 6-5 | 5 本の動画の実 API での評価の記録と結論 | frontier-required |

### 3.3. 実装順序の根拠

architecture §8 の順序に従う。`strictjson` は他に依存せず、応答の検査（フェーズ 2）が使うので最初に置く。フェーズ 2 は、段階が使う部品を、既存の `Write` の振る舞いを変えずに先に揃える（取り出しと一般化は独立したリファクタリングのコミット）。フェーズ 3 で段階を `Write` につなぐが、`Options` のゼロ値では段階を行わないので、CLI はまだ段階を使わず、既存の CLI のテストは変わらない。フェーズ 4 で検証・推敲の `LLMClient` を構築できるようにし、フェーズ 5 で CLI から段階を既定で有効にする。既存の CLI のテストに段階を無効にする引数を加えるのは、既定を変えるステップ 5-5 と同じコミットである。評価は、実装が揃ってから行う。

## 4. テスト戦略 (Test Strategy)

architecture §7 のテスト戦略に従う。テスト関数名と AC の対応は §5 に示す。

### 4.1. ユニットテスト

- 段階のテストは、生成と字数の調整にステップ 3-2 の fake を、検証・推敲に `FakeLLMClient` を使い、LLM の API もネットワークも呼ばない（AC-26）。`internal/writer` は `net` を import しないことを、既存の `TestWriterImports` が固定している。
- 応答の検査と当てはめは、規則ごとの表を非公開の関数に当てる（ステップ 2-5・2-6）。`Write` を通すテストは代表の行に絞り、規則の表を重ねない（§1.3）。
- 字数の境界は、本文の字数の関数（ステップ 2-3）と `Write`（ステップ 3-9 の `TestWriteBodyLength`）の両方で確かめる。前者は数え方、後者はタイトルの行と出典のブロックを数えないことと、字数の調整を呼ぶかどうかの境界を確かめる（architecture §7.1）。
- **網羅率の目標:** `internal/writer` の新設のファイル（`length.go`・`revision.go`・`review.go`・`shorten.go`）と `internal/strictjson` の追加の関数の、到達できるすべての文を、それぞれのパッケージのテストが実行すること。`go test -tags test -coverprofile` で網羅されていない文を確かめ、到達できない文が残る場合は、その文と理由をコミットメッセージに書く。
- **後方互換性（AC-27）:** `internal/writer`・`internal/job` の既存のテストは `Options{}`・`ReviewLog` が nil のまま変えない。ステップ 2-1・2-2・5-2 のリファクタリングのコミットでは、テストの変更は呼び出しの形の置き換えだけである。CLI の既存のテストは、ステップ 5-5 で段階を無効にする引数を加えるだけで、期待は変えない。

### 4.2. 統合テスト

- CLI の組み立ては、fake の `LLMClient` を使う `run_test.go` のテストで確かめる（ステップ 5-7）。
- 実 API の統合テスト（`TestIntegrationCLI`、`integration_slack_test.go`）は段階を無効にして実行し、`Generate` が 1 回であることを保つ（ステップ 5-5）。2 つの段階を実 API で通す確認は、評価（フェーズ 6）の実行で行う（architecture §7.2）。

### 4.3. セキュリティテスト

architecture §7.3 に従う。計画固有の事項は次のとおり。

- §5.1 の各脅威に対応する入力（巨大な応答、項目の数の超過、重複したキー、不正な UTF-8、対になっていないサロゲート、制御文字、あいまいな「直す前の記述」）は、`TestParseReviewResponse` と `TestApplyRevisions` の行に含める。
- 応答の拒否のエラーに応答の値が現れないことを、目印で確かめる（`TestParseReviewResponse`・`TestApplyRevisions`・`TestReviewResultCheck`）。
- 検証・推敲の失敗のエラーに API キーが含まれても、標準エラー出力に出ないことを、生成と検証・推敲が同じ DeepSeek のキーを使う構成で確かめる（`TestRunReviewDoesNotLeakAPIKey`）。一覧のファイルは記事の値と応答の値だけから作り、設定の値を受け取らないので（architecture §5.1）、ファイルにキーが出ないことを確かめるテストは置かない（失敗しうる形を作れないため）。新しい実行の経路（一覧のファイルの書き出しの失敗、`--review-log` と `--no-review` の同時指定、段階の失敗とその案内）は、`run_test.go:419` の `executionPathRows` にも行を加え、既存の経路ごとの保証（秘密情報が出ないこと、エスケープ、使い方の誤りでの副作用のなさ）を当てる。

### 4.4. テストヘルパー

- `internal/llm/testutil/mocks.go`（`test_organization.md` 分類 A）: 呼び出しの順に応答を返す fake。`internal/writer` のテストと `cmd/yt2column` のテストの両方が使えるよう、公開の API だけで作る。新しいファイルは作らない。
- `internal/writer/test_helpers.go`（分類 B）: 段階のテストの文面と応答の部品（上限ちょうどの本文、一覧の JSON の組み立てなど）。非公開の定数（`maxBodyChars` など）を使うため。
- `internal/publisher/test_helpers.go`（分類 B）: `atomicFile` の差し替え口を使う `ReviewLogWriter` のテスト用の構築。
- `cmd/yt2column/test_helpers.go`（分類 B）: `testDeps` の `newReviewLLMClient`。
- `cmd/yt2column/test_helpers_integration.go`（分類 B、`//go:build test || integration`）: 段階を無効にする引数の値（`integration` タグのテストも使うため）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は実行可能なテスト、`static` は guard テスト・`make` ターゲットを指す。`manual` は補助的な確認であり、`test` または `static` を置き換えない。パスは `internal/writer/` を `writer/`、`cmd/yt2column/` を `cmd/` と略す。

| AC | 内容 | 種別 | 検証の実行場所 |
|---|---|---|---|
| AC-01 | 上限以内なら生成 1 回・検証・推敲 1 回、字数の調整なし、タイトルと本文は生成のまま | test | `writer/steps_test.go::TestWriteStepOrder`（2 つの fake の呼び出しの回数と順序、空の一覧でタイトルと本文の部分が生成と同じ、字数の調整を有効にしていても `Shorten` が nil、`Review.Model` が検証・推敲の fake のモデル名） |
| AC-02 | 上限超過なら生成・字数の調整・検証・推敲の順に 1 回ずつ、検証・推敲に渡るのは縮めた文面 | test | `writer/steps_test.go::TestWriteStepOrder`（呼び出しの順序、検証・推敲のプロンプトに縮めた文面の目印があり、生成の文面の目印がない、`Shorten.Model` が字数の調整の応答のモデル名） |
| AC-03 | 字数の調整の出力・当てはめた文面が生成テキストの検査で拒否されると `ErrMalformedOutput`、ゼロ値 | test | `writer/steps_test.go::TestWriteStepMalformedOutput`（字数の調整の出力の 1 行目が `# ` でない・本文が空、タイトルの文字を消して空のタイトルにする一覧）・`writer/output_test.go` の既存のテスト（ステップ 2-1 の後も `checkResponse` を通して規則を確かめる） |
| AC-04 | 本文の字数の数え方と 4,000／4,001 の境界 | test | `writer/length_test.go::TestBodyChars`・`writer/steps_test.go::TestWriteBodyLength`（タイトルの行と出典のブロックを数えず、4,000 字で字数の調整を呼ばず 4,001 字で呼ぶ） |
| AC-05 | 字数の調整のプロンプトに文面と上限があり、字幕がない | test | `writer/steps_test.go::TestShortenPrompt`・`writer/template_test.go::TestTemplateDataFields`（`.Transcript` の拒否） |
| AC-06 | 縮めた後も超過なら `ErrTooLong`、検証・推敲を呼ばない、上限と字数をエラーに含む | test | `writer/steps_test.go::TestWriteTooLong`（字数の調整の後の行。`errors.Is(err, ErrTooLong)`、`StepError` の `Step` が `StepShorten`、メッセージが上限と数えた字数を数として含む、検証・推敲の fake の呼び出しが 0 回、ゼロ値の記事） |
| AC-07 | 検証・推敲の後に超過なら `ErrTooLong` | test | `writer/steps_test.go::TestWriteTooLong`（検証・推敲で長くなる行。`ErrTooLong`、`Step` が `StepReview`、ゼロ値の記事。字数の調整を無効にすると長い本文でも `ErrTooLong` にならない行） |
| AC-08 | 字数の調整は生成と同じ `LLMClient` | test | `writer/steps_test.go::TestWriteStepOrder`（生成の fake に 2 回、検証・推敲の fake に 1 回） |
| AC-09 | 検証・推敲のプロンプトに文面と 4 つの素材、字幕は生成と同じ文字列 | test | `writer/steps_test.go::TestReviewPrompt`（生成の user と検証・推敲の user を、字幕の前後に固有の区切りを置く上書きテンプレートにし、2 つのプロンプトの区切りの間の文字列が一致する。文面の目印と、動画タイトル・チャンネル名・概要欄の目印がある） |
| AC-10 | 一覧の当てはめと一覧の取り出し | test | `writer/review_test.go::TestApplyRevisions`（`甲乙` の例、順序の入れ替え）・`writer/steps_test.go::TestWriteReviewApplies`（`Article.Review.Revisions` の 4 つの要素） |
| AC-11 | 空の一覧なら文面は変わらず、一覧は空 | test | `writer/steps_test.go::TestWriteReviewApplies`（空の一覧の行。`Review` は nil でない） |
| AC-12 | 受理の規則に反する応答は `ErrMalformedOutput` | test | `writer/review_test.go::TestParseReviewResponse`・`writer/steps_test.go::TestWriteReviewRejects`（代表の行で `StepReview` とゼロ値） |
| AC-13 | 出現 0 回・2 回以上・重なりの拒否 | test | `writer/review_test.go::TestApplyRevisions`・`writer/steps_test.go::TestWriteReviewRejects`（代表の行） |
| AC-14 | 改行・見出しの中・見出しを作る項目の拒否、タイトルの行の中の項目の受理 | test | `writer/review_test.go::TestParseReviewResponse`（改行の行）・`TestApplyRevisions`（見出しの行、タイトルの行）・`writer/steps_test.go::TestWriteReviewApplies`（タイトルの置き換え） |
| AC-15 | 2 つの変数が未設定なら生成と同じプロバイダとモデル | test | `internal/config/config_test.go::TestLoadReview`・`internal/llm/provider/provider_test.go::TestNewReviewUsesReviewModel` |
| AC-16 | 2 つの変数を設定したらそのプロバイダとモデル。異なるプロバイダの API キーが必須 | test | 前半: `internal/config/config_test.go::TestLoadReview`・`internal/llm/provider/provider_test.go::TestNewReviewUsesReviewModel`。後半: `internal/config/config_test.go::TestLoadReviewProviderRequiresItsKey`（ステップ 4-5。0009 の PR-4 の前なら 0009 の計画に引き継ぎ、引き継いだ先をここに書く） |
| AC-17 | プロバイダだけ設定は `ErrMissing`、不正な値は `ErrInvalid` で値が現れない | test | `internal/config/config_test.go::TestLoadReview`・`TestLoadErrorsOmitValues`（追加の行） |
| AC-18 | モデルだけ設定なら生成のプロバイダとそのモデル | test | `internal/config/config_test.go::TestLoadReview`・`internal/llm/provider/provider_test.go::TestNewReviewUsesReviewModel` |
| AC-19 | `--no-review`・`--no-shorten`・両方の指定 | test | `cmd/run_test.go::TestRunStepFlags`（`--no-review` で検証・推敲の fake が呼ばれない。`--no-shorten` は検証・推敲を有効にしたまま上限を超える本文で、字数の調整を呼ばず成功する。両方の指定で `--out` のファイルと標準エラー出力が文字列のリテラルで書いた従来の書式と一致）・`internal/publisher/file_test.go` の既存の `TestFilePublisherWritesArticle`・`TestFilePublisherEmptyModelVersion`（段階のない記事の書式） |
| AC-20 | `--review-log` の書き出し、失敗時の扱い | test | `internal/publisher/reviewlog_test.go` の 5 つのテスト・`internal/job/job_test.go::TestRunReviewLogWritten`・`TestRunReviewLogNotWrittenOnFailure`・`TestRunReviewLogWriteFailure`・`cmd/run_test.go::TestRunReviewLog`・`TestRunReviewLogFailure`（`--out` と `--slack` のそれぞれで、メッセージが記事を書き出した（投稿した）ことを示し、`article was kept` と `the run failed:` を含まず、要約が先、終了コード 1） |
| AC-21 | `--review-log` と `--no-review` の同時指定は LLM を呼ばずに使い方の誤り | test | `cmd/run_test.go::TestRunReviewLogWithNoReview`（`exitUsage`、生成の fake の呼び出し 0 回） |
| AC-22 | `--out` のモデルの行に呼んだ段階の行だけ | test | `internal/publisher/file_test.go::TestRenderArticleSteps`・`cmd/run_test.go::TestRunStepModelLines`（字数の調整を有効にして呼ばなかった場合に字数の調整の行がない行を含む）・`writer/steps_test.go::TestWriteStepOrder`（`Shorten` が nil） |
| AC-23 | 上書きテンプレートの検査と既定のテンプレートの検査 | test | `writer/template_test.go::TestTemplateSyntaxAllowlist`（生成の 2 つと段階の 4 つのテンプレート）・`TestDefaultTemplatesPassChecks`・`TestInvalidTemplateErrorNamesSource`・`cmd/run_test.go::TestRunInvalidStepTemplate`（4 つの上書きのフラグのそれぞれで、LLM を呼ばずに `exitUsage`） |
| AC-24 | 段階の LLM の失敗で元の番兵を判定でき、段階がメッセージで分かる | test | `writer/steps_test.go::TestWriteStepFailures`（2 つの段階 × `llm.ErrTruncated`・`llm.ErrEmptyResponse`・`context.DeadlineExceeded`・テスト用の番兵。`errors.AsType[*StepError]` の `Step`、メッセージの段階の名前、ゼロ値） |
| AC-25 | 段階の前のキャンセルでその段階の LLM を呼ばない | test | `writer/steps_test.go::TestWriteStepContextDone`（2 つの段階の前の取り消しの 3 行。その段階の fake が呼ばれず、`context.Canceled`） |
| AC-26 | ユニットテストは LLM の API もネットワークも呼ばない | test / static | フェーズ 3 のすべての `writer/` のテスト（fake の `LLMClient`）・`internal/pipeline/pipeline_test.go::TestWriterImports`・`cmd/run_test.go::TestTestDepsReviewClientBuildsNothing`（CLI のテストの既定の `newReviewLLMClient` が何も作らない） |
| AC-27 | 段階を無効にした既存のテストがそのまま通る | test | `writer/` と `internal/job/` の既存のテスト（変更なし。`overrideTargets` も変えない）・`cmd/` の既存のテスト（ステップ 5-5 で段階を無効にする引数を加えるだけ）・`make test` |
| AC-28 | 5 本の動画の観点ごとの判定の記録 | static / manual | `cmd/docs_test.go::TestReviewEvaluationRecords`・ステップ 6-3 の評価 |
| AC-29 | 一覧の各項目の判定と新しい根拠のずれの有無の記録 | static / manual | `cmd/docs_test.go::TestReviewEvaluationRecords`・ステップ 6-3 の評価 |
| AC-30 | プロバイダとモデル、呼び出しの回数、所要時間の記録 | static / manual | `cmd/docs_test.go::TestReviewEvaluationRecords`・ステップ 6-3 の評価 |

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 作業環境が root で動き、`requireNonRoot` を持つ 17 件のテストが失敗する（§1.3） | `make test` が環境の理由で失敗し、本タスクの失敗と区別しにくい | グリーンゲートは root 以外の利用者で実行する。root でしか実行できない場合は、失敗が `requireNonRoot` の 17 件だけであることを確かめ、そのことを PR に書く。 |
| 0009 の PR-4 と本タスクのフェーズ 4 の順序が決まっていない | 後に実装した側が architecture §3.7 と食い違う形を作る | ステップ 4-1・4-5 で、着手時の状態に応じて自分の側を合わせるか、0009 の計画を改めて再承認を求める。 |
| CLI の既定で段階を行うため、段階を無効にしないテストが本番のアダプタを構築し、閉じたプロキシへの `Generate` で分かりにくく失敗する（`exitFailure` を期待するテストが別の理由で通ることもある） | AC-27 の確認が意味を失う | `newRunEnv` と `testDeps` の `newReviewLLMClient` を、何も作らずにエラーを返す関数にする。`runEnv.cliArgs`・`childSetup.args` が既定で段階を無効にし、直書きの起動箇所には値を加える（ステップ 5-5）。段階を無効にし忘れたテストは手順 A5 の `exitUsage` で失敗し、理由がメッセージに出る。`childModeMain` は引数だけで無効にする。 |
| `overrideTargets` を回す 8 つの既存のテストと `TestTemplateSyntaxAllowlist` の表が、生成のフィールドと生成の呼び出しの回数を前提にしている | 6 つのテンプレートに広げると既存のテストの意味が変わり、AC-27 に反する | `overrideTargets` は変えず、段階のテンプレートの表を別に加え、行の中のフィールドの参照を対象のデータ型のフィールドに置き換えて当てる（ステップ 3-9）。 |
| `TestPromptsREADMEMatchesContract` が埋め込みのフィールドを数えない | README の検証・推敲の表の `.Title` などが guard で比べられない | guard を 3 つのデータ型と埋め込みのフィールドに広げ、README の更新と同じコミットに入れる（ステップ 3-10）。 |
| 検証・推敲の応答がコードフェンスや説明文を伴う、または「直す前の記述」が 2 回以上現れる | 記事全体が失敗する（architecture §5.4） | 既定のテンプレートで JSON だけを返し、一意に決まる長さを引用するよう指示する。評価で失敗の種類ごとに記録し、多いものはテンプレートの文言で対処する（ステップ 6-5）。リトライや部分的な受理はしない。 |
| 縮めた本文が検証・推敲の後に上限を超える（AC-07） | 記事が `ErrTooLong` で失敗する | `targetBodyChars` で余裕を残す（architecture §3.3）。評価で頻度を記録し、必要なら値を見直す。 |
| `make ext-test` の Node.js の版が手元と合わない | 文書のコミットで extension の guard テストを走らせられない | 固定された版の Node.js を用意して実行する。用意できない場合は、変更が extension の guard テストの読む内容（拡張の ID、`ext-` のターゲット、ignore の規則）に触れないことを確かめ、実行しなかったことを PR に書く。 |
| 評価の実 API の実行の承認を待つ、または料金の高い時間帯に当たる | フェーズ 6 が止まる | フェーズ 1〜5 の実装中に承認を依頼しておく。実行は料金が 2 倍になる時間帯を避ける（requirements §5）。 |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2。2-1・2-2 がそれぞれ独立したリファクタリングのコミット）
- [ ] PR-3 マージ済み（対象ステップ: 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
- [ ] PR-4 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11）
- [ ] PR-5 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6。ステップ 4-1・4-5 の結果を記録済み）
- [ ] PR-6 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4。5-2 が独立したリファクタリングのコミット）
- [ ] PR-7 マージ済み（対象ステップ: 5-5 / 5-6 / 5-7 / 5-8）
- [ ] PR-8 マージ済み（対象ステップ: 5-9 / 5-10 / 5-11）
- [ ] PR-9 マージ済み（対象ステップ: 6-1 / 6-2）
- [ ] PR-10 マージ済み（対象ステップ: 6-3 / 6-4 / 6-5）
- [ ] 各 PR で `make fmt` → `make test` → `make lint` が通る。文書を変えたコミットで `make ext-test` が通る
- [ ] §5 のすべての AC の検証が通る
- [ ] AC-16 の後半の検証（`TestLoadReviewProviderRequiresItsKey`）が、本タスクか 0009 のどちらかで実装されている（ステップ 4-5 の記録で追う）

## 8. 成功基準 (Success Criteria)

- **機能:** AC-01〜AC-30 のすべてが §5 の検証で確認されている。AC-16 の後半は、ステップ 4-5 の結果に従って本タスクか 0009 で確認されている。
- **品質:** `make test`・`make lint` が通る。§4.1 の網羅率の目標を満たす。各テストは対象を壊して失敗することを確認済みで、そのことがコミットメッセージに記録されている。リファクタリングのコミット（ステップ 2-1・2-2・5-2）で既存テストの期待が変わっていない。
- **セキュリティ:** 検証・推敲の応答の境界（大きさ・項目の数・未知と重複のキー・UTF-8・サロゲート・制御文字）、エラーに応答の値を含めないこと、検証・推敲の失敗で標準エラー出力に API キーが出ないことを確認済みである。
- **互換性:** `--no-shorten --no-review` の CLI の出力が本タスクの前と同じである（AC-19）。依存モジュールを追加していない（`.golangci.yml` の depguard を変更していない）。
- **ドキュメント:** `README.md`・`project_overview.md`・`security.md`・`package_reference.md`・`CLAUDE.md`・`prompts/README.md` が実装と一致し、記述の根拠を照合済みである。
- **評価:** `02_evaluation.md` に 5 本の動画の評価の記録と結論がある。

## 9. 次のステップ (Next Steps)

- 本計画は `approved`。PR の区切りは §2 の `PR-N 作成ポイント` と §3.2 に埋め込み済み。`/runplan 0010` で PR-1 から実装する。
- 評価の結論に従い、0008 の評価を再開するか、誤認識の写しが残る場合は生成の前の字幕の整形を別の issue にする（requirements §2.3、architecture §9）。
- JSON の形に従わない応答が多い場合は、architecture §9 のプロバイダの JSON モードを検討する。
