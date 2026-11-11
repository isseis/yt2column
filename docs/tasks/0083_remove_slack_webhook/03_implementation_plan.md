# 実装計画書：Slack の Webhook への投稿の機能を廃止し、メンションの無効化をクリップボードへのコピーに移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。用語（対象の `@`・無効化・URL・文字参照・URL のパスの `/@`・表示用の Markdown・コピーする文章・無効化した表示用の Markdown・無効化の部品・`@` の単位）は、要件書 §3.1 と設計書の冒頭と同じ意味で使う。本計画は、次の用語を加える。

-   **削除のコミット:** 関数・型・ファイルと、それを確かめるテストを消すだけのコミット。消すほかの変更を含めない。テストを消すので、`make coverage-compare BASE=<親のコミット>`（Go）と `make ext-coverage-compare BASE=<親のコミット>`（拡張）で確かめ、通った出力を最後の行まで commit のメッセージと PR の説明に書く（`CLAUDE.md`「Testing Strategy」、[ADR-0037](../../dev/adr/0037-test-deletion-coverage.md)）。coverage-compare は、残る関数の網羅率が下がると失敗し、削除でない変更を含むコミットでは止まる（設計書 8 章の 4）。
-   **書き換えのコミット:** 削除のコミットの前に、残る関数の呼び出し元を消すものから切り離すコミット。テストの関数や表の行を消さない。
-   **`_checks/` のスクリプト:** 本タスクのディレクトリの `_checks/` に置く、一回限りの確かめのスクリプト。PR のブランチで実行し、結果を §7 に記録する。`make test` と CI は実行しない（`docs/dev/conventions/requirements_process.md` §4）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

サーバ・ホスト・拡張・設定から Webhook への投稿の機能を消し、メンションの無効化を、ビューアの表示と「Markdown をコピーする」に移す（要件書 2.1 節）。何をどう作るかは設計書 3 章が定める。本計画は、作業の順序、PR の分け方、確かめ方を定める。

### 1.2. 実装の原則

-   **設計書 8 章の依存の順に進める。** 本文の UTF-8 の検査 → 規則の移動 → 無効化の部品と応答・拡張 → 投稿の機能の削除 → 文書 → 手動の確かめ。
-   **テストを消すコミットは、削除のコミットにする。** 残る関数の呼び出し元の書き換えは、その前の書き換えのコミットで行う。消すテストだけが網羅していた残る関数には、削除のコミットより前に、その関数を確かめるテストを加えるか移す（設計書 8 章の 4）。
-   **応答の形を変える側と受ける側は、同じ PR で変える**（H-05、設計書 8 章の 3）。サーバの状態の取得の応答と、拡張の `response.ts` の検査は、いつも同じ PR に入れる。
-   **文書を読むガードのテストは、文書と同じコミットで改める**（設計書 3.12、8 章の 5）。ガードのテストが読まない文書の記述は、PR-5 でまとめて改める。PR-3・PR-4 の間は、文書が、消えた機能を述べたまま残る箇所がある。
-   **Go と TypeScript のソースのコメント・識別子・文字列のリテラルは英語で書く**（`CLAUDE.md`「Go Idioms」）。拡張の利用者に見せる文は、英語と日本語のメッセージのカタログに置く。
-   **各フェーズの後に、`make fmt` → `make test` → `make lint` → `make deadcode` を通す。** 拡張を変えたフェーズは `make ext-check` も通す。

### 1.3. 計画の際に決めたこと（2026-11-11）

1.  **PR を 5 本に分ける。** PR-1 は本文の UTF-8 の検査と規則の移動（設計書 8 章の 1・2）、PR-2 は無効化の部品と応答・拡張の表示・コピー・見出し（8 章の 3）、PR-3 はジョブの API の投稿の削除（サーバの投稿の操作と投稿のジョブ、ホストの `publish`、記事のキャッシュの投稿の状態、応答の `webhookConfigured`・`publish`、ビューアの投稿の操作、それらに結びついた文書の表。8 章の 4 の前半）、PR-4 は内部に残る投稿のコードの削除（`internal/publisher` の Slack のコード、`internal/slackwebhook`、設定の `SLACK_WEBHOOK_URL`、`internal/redact`、`make test-integration-slack`、ガードのテスト。8 章の 4 の後半）、PR-5 は文書・エージェントの指示・ADR の注記と手動の確かめ（8 章の 5・6）である。理由: 投稿の削除は Go と拡張の約 30 のテストのファイルにわたり、1 本にするとレビューの量が大きすぎる。ジョブの API（応答・経路・ホスト・ビューア）と、それより内側のコードの境目で分けると、どちらの PR も単独でグリーンゲートを通る。
    -   採らなかった案: 投稿の削除を 1 本の PR にする案（4 本）。3 本にまとめる案。
    -   人の答え: 5 本の案を選んだ。
    -   ADR: なし（タスクの中で閉じる、PR の分け方である）。
2.  **Slack の投稿のゴールデンのファイル（`internal/publisher/testdata/golden/*.slack.txt`）と `TestRenderGoldenNeutralized` は、投稿のコードと一緒に消し、無効化した表示用の Markdown のゴールデンのファイルは作らない。** 理由: 無効化の規則は、設計書 7.1 の表のテストが U+200B を入れる位置で確かめる。`<case>.md` のゴールデンのファイルは残し、AC-16a の確かめに使う。設計書 3.13 の表は、これらのファイルとテストを挙げていなかった（§1.4）。
    -   採らなかった案: `<case>.display.txt` のゴールデンのファイルと、U+200B の位置の確かめを加える案。
    -   人の答え: 消す案を選んだ。
    -   ADR: なし。
3.  **AC-17 の手動の確かめは、最後の PR（PR-5）で行い、記録する。** 理由: AC-17 は「実装の後」と定める。PR-2 の後は必要な振る舞いがそろうが、確かめを 1 回にして記録の場所を 1 つにする。
    -   採らなかった案: PR-2 で行う案、PR-2 と PR-5 の両方で行う案。
    -   人の答え: 最後の PR の案を選んだ。
    -   ADR: なし。
4.  **規則の変更（設計書 3.3）は、PR-1 で移した `mentionAts` をその場で変え、投稿の経路には、直後に名前の文字が続く条件を保つ Slack 専用の包みを通す。包みは PR-4 で投稿のコードと一緒に消す。** 理由: PR-2 の時点では、投稿の無効化（`slack_render.go:116`〜`136`）、分割の切れ目の確かめ（`slack_message.go:182`）、拒否（`slack_message.go:253`）が `mentionAtsOf` を使っている。`mentionAts` から名前の文字の条件を除くだけだと、拒否が、無効化した後の `@`＋U+200B を対象の `@` とみなし、PR-3・PR-4 までの間、すべての投稿を拒否する。その場で変えると、対象の判定が 1 つのままで、移したテストを新しい規則に合わせて改める設計書 3.13 の扱いとも合う（行番号はコミット `e4cbff08` での値）。
    -   採らなかった案: 新しい規則を別の関数として加え、移した関数は投稿の振る舞いのまま PR-4 でテストと一緒に消す案。2 つの判定が 2 本の PR の間並ぶ。
    -   人の答え: その場で変えて包みを通す案を選んだ。
    -   ADR: なし（パッケージの中の関数の形である）。
5.  **`CLAUDE.md`「Declare, don't infer」の、無効化の部品の例外は、PR-5 ではなく、無効化の部品を加える PR-2 で加える。** 理由: 例外が覆うコードが入る PR に例外がないと、そのコードが規則に反するものとしてレビューされる。設計書 3.12 の表の書く内容は変えず、時期だけを早める。ADR: なし（`CLAUDE.md` の規則は ADR にしない）。
6.  **記録が持つ動画のタイトルは、`articlecache.Subject` に持たせる。** `Subject` は `Keyer.Subject` が `KeyInput` から作り（`internal/articlecache/key.go:53`〜`56`・`91`）、`KeyInput.Title` は鍵の計算に入っている（`key.go:82`）。そのため、同じ鍵の記録は同じタイトルを持ち、`Begin` が記録を作るときに `Subject` から写せばよい。ADR: なし（設計書 3.7 の決定の範囲の、置き場所の選択である）。

### 1.4. 既存コード調査結果

コミット `e4cbff08`（`origin/main`）で確かめた。以下の `file:line` はこのコミットの行番号である。設計書は `ee6d14df` の行番号を使う。その後のコミットが変えた製品のコードは `cmd/yt2column-server/run.go:191`（`SecretStrings` の引数に OpenAI のキーを加えた 1 行）と CLI だけで、設計書が引く行番号は今も合う（`git diff --stat ee6d14df e4cbff08 -- internal cmd extension Makefile`）。

-   **設計書 3.13 の表にない、変える・消すもの。** `git grep -n -i -E 'webhook|slack'` で洗い出した。各ステップで扱う。
    -   `internal/publisher/golden_test.go`（`TestRenderGolden` の `.slack.txt` の比較、`TestRenderGoldenNeutralized`）と `testdata/golden/*.slack.txt` の 10 ファイル（§1.3 の決定 2）。
    -   `internal/publisher/makefile_test.go`（`TestMakeTestIntegrationSlack`・`TestSlackIntegrationTestBuildTag`）。
    -   `internal/publisher/testutil/integration.go` の全体。設計書 3.4 は「ほかの統合テストが使う部分（`IntegrationOptions` など）は残す」とするが、このファイルの名前を `internal/publisher` の外で使うのは、どれも Slack の統合テストとそのガードだけである（`cmd/yt2column/docs_test.go:179`〜`181`、`cmd/yt2column/makefile_test.go:71`・`82`・`86`、`internal/publisher/makefile_test.go`）。そのため、残す部分はなく、ファイルの全体と `integration_settings_test.go` を消す。`testutil/mocks.go` の `FakePublisher` は `Publish` を残し、`PostMessages`・`PostErr`・`PostCalls`・`FakePostCall`・`RecordedPostCalls` を消す（`internal/pipeline/pipeline_test.go` が `Publish` の側を使う）。
    -   `internal/redact/redact.go:90`（`SecretStrings(webhookURL secret.Secret, others ...secret.Secret)`）は `internal/slackwebhook` を import し、Webhook の URL に `SensitiveParts` を加える。`internal/slackwebhook` を消すので、引数を `...secret.Secret` だけにする書き換えが要る。呼び出し元は `cmd/yt2column-server/run.go:191` と `cmd/yt2column/run.go:302` で、CLI は 1 つ目の引数に DeepSeek のキーを渡している。CLI では、キーの値と末尾の 8 文字は `secretReplacements`（`redact.go` の同じ節）が伏せるので、`SensitiveParts` がなくなっても伏せる文字列は減らない。これを、既存の `internal/redact` と `cmd/yt2column` のテストが通ることで確かめる。`TestSecretStringsWebhookParts`（`internal/redact/redact_test.go:202`）は消す。
    -   `cmd/yt2column/docs_test.go:171`（`TestSlackDocsContract`）、`:557`（`config.SlackWebhookEnvForTest()` を使う設定）、`cmd/yt2column/makefile_test.go:71`・`82`・`86`（Slack の make のターゲットと opt-in の変数）、`cmd/yt2column/output_layout_docs_test.go:110`〜`119`（README の Slack の投稿の例と `SlackMessagesForTest` の比較、`doctestutil.ReadmeSlackPostHeading`）。
    -   `internal/config/test_helpers.go:25`（`SlackWebhookEnvForTest`）、`internal/config/envaccess_test.go:59`（`secretEnvNames` の `SLACK_WEBHOOK_URL`。同じファイルの `:553`・`:557` が、ガードの自己確かめの例にこの名前を使う）。
    -   `cmd/yt2column-server/test_helpers.go:44`〜`48`・`143`（テストの設定ファイルに `SLACK_WEBHOOK_URL` を書く）。AC-05 の拒否を入れると、このままではテストのサーバが起動しない。拒否と同じコミットで消す。
    -   `cmd/yt2column-nmhost/main_test.go:126`（`fullEnv` が「サーバの変数」として `SLACK_WEBHOOK_URL` を持つ）。
    -   `internal/doctestutil/guides.go:52`〜`53`（`development.md` の投稿の状態と理由の表の見出し）、`internal/doctestutil/readme.go:81`（`ReadmeSlackPostHeading`）、`internal/doctestutil/markdown_test.go:134`（見出しの例の 1 行。Slack の README の見出しを使うが、`Locator` は使わない）。
    -   拡張: `src/core/sendMessages.ts:136`（`publishResultMessage`）、`src/poller.ts:7`・`46`（コメント）、`src/send.ts:3`（コメント）、`static/theme.css:93`（コメント）、`test/startCoordinator.test.ts:363`、`test/helpers/developmentDoc.ts:25`〜`26`、`test/secretsAbsent.test.ts:10`〜`13`（コメント）。
    -   `.golangci.yml:88`〜`134` の depguard の規則のうち、`server-no-pipeline` の理由（「投稿の操作の post job だけが Webhook に送る」）と、`publisher-no-server`・`articlecache-boundaries` のコメントと `desc` が投稿を述べる。規則（禁じる import）は変えず、理由の文を改める。
    -   Go のコメントだけで Webhook を例に挙げるもの: `internal/secret/secret.go:2`・`22`、`internal/transcript/exec.go:43`、`internal/codexreview/run.go:165`、`internal/articlecache/key.go:4`、`internal/publisher/publisher.go:3`〜`5`、`internal/publisher/format.go:23`、`cmd/yt2column-server/main.go:4`、`cmd/yt2column/integration_env_test.go:29`、`cmd/yt2column/run_test.go:1184`、`internal/server/test_helpers.go:372`〜`379`。
-   **変えないもの。** `internal/transcript/exec_test.go:173` は、子のプロセスに渡さない秘密らしい変数の例として `SLACK_WEBHOOK_URL` を使う。名前は任意の例で、廃止の後も確かめの意味は変わらないので残す。`internal/coveragecompare/testdata_test.go:22` と `testdata/` は、ファイルのパスを fixture の文字列として持つだけなので残す。`extension/test/secretsAbsent.test.ts:20` の禁じる語の一覧の `SLACK_WEBHOOK_URL` は、拡張が秘密を持たないことのガードとして残す（コメントだけを改める）。
-   **パッケージの一覧のガード。** `internal/pipeline/pipeline_test.go:625`（`TestPackageReferenceListsPackages`）は、`docs/dev/conventions/package_reference.md` が製品のコードのあるパッケージをちょうど並べることを確かめる。`internal/slackwebhook` を消すコミットで、その行（`package_reference.md:31`）も消す。
-   **`development.md` を読む拡張のテスト。** `extension/test/serverReasons.test.ts` は拒否の理由の表を、`jobCodes.test.ts` は状態の取得の表（`test/helpers/developmentDoc.ts` の見出し）を読む。`cmd/yt2column-server/docs_test.go:108`・`463`〜`467` は、API の表の `server.PublishSuffix` の行を読む。どれも PR-3 で、コードと同じコミットで改める。
-   **README を読むガード。** `extension/test/metadataDocs.test.ts:58` はビューアの手順の `**${catalog.viewer.publish}**` を、`cmd/yt2column/docs_test.go:171` と `output_layout_docs_test.go` は投稿の上限と例を読む。カタログの項目やコードを消すコミットで、ガードの該当の確かめを消す。README の本文は PR-5 で改める。
-   **対象の `@` の判定の使われ方（§1.3 の決定 4）。** `mentionAtsOf` は `slack_render.go:136`（`insertAfterAts`）、`slack_message.go:182`（`cutStartsMention`）、`:253`（`findMention`）が呼ぶ。`replaceLookAlike` は `:352`（`inspectMentionText`）と `:414` が呼ぶ。`mentionAts` 自体は、すでに入力の長さに比例する 1 回のたどりである（`slack_message.go:394`〜`406`）。
-   **`writer.ProseRanges` の補助の関数。** `codeSpans`・`codeSpansExempt`・`proseLines` は `checkBodyMarkdown`（`internal/writer/markdown.go:108`〜`117`）も使うので、`ProseRanges`（`:49`）と `ByteRange`（`:32`）を消しても残る。
-   **完了の時点の検査の今の振る舞い。** `completeGeneration`（`internal/server/generation.go:71`〜`110`）は、`CheckPublishable`、応答の大きさ、`PrepareSlackMessages` の順に検査し、拒否された記事を `CompleteUnpostable` で `done` にする。`prepareSlackMessages` の本文の UTF-8 の検査（`slack_message.go:30`〜`35`）は、PR-1 の後は `CheckPublishable` が先に拒否するので通らなくなる。設計書 3.5 のとおり、PR-4 で Slack のコードと一緒に消す。
-   **テストの名前の確かめ。** §2 と §7 が引く既存のテストは、`grep -n '^func Test'` でファイルにあることを確かめた。新しく作るテストの名前は §2 の各ステップが決める。
-   型の構成の経路の検査（`mkplan.md` の該当の項目）は当たらない。設計は具体的な型でふるまいを分けない。

### 1.5. 名前の変更の一覧

本計画の名前の変更は、この表だけが定める。ほかの節は、この表に従う。

| 種類 | 前の名前 | 新しい名前 | ステップ |
|---|---|---|---|
| 関数 | `publisher.DisplayMarkdown` | `publisher.NeutralizedDisplayMarkdown`（戻り値は `NeutralizedDisplay`） | 2-2 |
| テスト | `TestDisplayMarkdown`（`internal/publisher/display_render_test.go`） | `TestNeutralizedDisplayMarkdown` | 2-2 |
| テスト | `TestDisplayMarkdownRejectsUnpublishable` | `TestNeutralizedDisplayMarkdownRejectsUnpublishable` | 2-2 |
| メソッド | `articlecache.Run.CompletePostable`・`CompleteUnpostable` | `articlecache.Run.Complete` | 3-1 |
| テスト | `TestGenerationUnpostable`（`internal/server/generation_test.go`） | `TestGenerationMentionsDone` | 3-1 |
| 関数の引数 | `redact.SecretStrings(webhookURL secret.Secret, others ...secret.Secret)` | `redact.SecretStrings(secrets ...secret.Secret)` | 4-4 |

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 本文の UTF-8 の検査と規則の移動（設計書 8 章の 1・2）

**変更するファイル:** `internal/writer/article.go`・`article_test.go`、`internal/server/generation_test.go`、`internal/publisher/file_test.go`、`internal/publisher/mention.go`（新規）・`mention_test.go`（新規）、`internal/publisher/slack_message.go`・`slack_render.go`・`slack_message_test.go`

-   [ ] **ステップ 1-1**: `writer.Article.CheckPublishable` に本文の UTF-8 の検査を加える（設計書 3.5）。違反は `ErrInvalidArticle` を包み、理由は本文の値を含まない固定の文字列にする。制御文字の検査より前に置く。
    -   `internal/writer/article_test.go` に `TestArticleCheckPublishableBodyUTF8` を加える。不正な UTF-8 の本文を `ErrInvalidArticle` で拒否し、エラーの文字列が本文のバイトを含まないことを確かめる。不正なバイトだけを含み制御文字を含まない本文を入力にする。
    -   `internal/server/generation_test.go::TestGenerationUnpublishable` に、不正な UTF-8 の本文の場合を加え、`failed` の `article_invalid` で記事を持たないことを確かめる。
    -   `internal/publisher/file_test.go::TestFilePublisherRejectsArticle` に、不正な UTF-8 の本文の場合を加え、ファイルを書かずに `ErrInvalidArticle` で失敗することを確かめる。
    -   既存の Slack の検査のテスト（`TestSlackPrepareRejectsInvalidArticle` など）が本文の UTF-8 の場合に別のエラーを期待していれば、期待を `ErrInvalidArticle` を包むエラーに改める。
    -   加えた検査を外すと上の 3 つのテストが失敗することを確かめ、commit のメッセージに書く。
-   [ ] **ステップ 1-2**: 対象の `@` の判定と U+200B を入れる処理を、振る舞いを変えずに `internal/publisher/mention.go` へ移す（設計書 3.2「ファイルの分け方」、H-04）。
    -   移すもの: `zeroWidthSpace`、`replaceLookAlike`、`mentionAts`、`mentionAtsOf`、`isMentionNameChar`、`insertAfterAts`、`followsSlash`。Slack に固有の `neutralizeMentions`・`neutralizeProse`・`inRanges` は `slack_render.go` に残す。
    -   移すテスト: `TestMentionAts`（`slack_message_test.go:872`）を `mention_test.go` へ移す。ほかのテストは動かさない。
    -   移すだけのコミットにする。テストの関数の数と名前が移す前と同じであることを commit のメッセージに書く。
-   [ ] **ステップ 1-3**: AC-16a の差分の記録を書く（§7 の「差分の記録」の PR-1 の欄）。

**完了の条件:** 上の 3 つのテストが加わり通る。`mention.go` が移した名前を持ち、`slack_message.go`・`slack_render.go` に同じ名前の定義がない。`make test`・`make lint`・`make deadcode` が通る。

### フェーズ 2: 無効化の部品と、応答・拡張の変更（設計書 8 章の 3）

**変更するファイル:** `internal/publisher/mention.go`・`mention_test.go`・`display_render.go`・`display_render_test.go`・`slack_message.go`・`slack_render.go`、`internal/articlecache/key.go`・`record.go`・`cache.go`・`cache_test.go`、`internal/server/status.go`・`status_test.go`、`cmd/yt2column-server/relay_e2e_test.go`、`extension/src/core/response.ts`・`jobView.ts`・`i18n/catalog.ts`・`en.ts`・`ja.ts`、`extension/src/ui/renderViewer.ts`、`extension/src/viewerApp.ts`、`extension/scripts/popup-preview.ts`、拡張のテストと `test/helpers/job.ts`、`CLAUDE.md`

-   [ ] **ステップ 2-1**: 対象の `@` の規則を変える（設計書 3.3 の「`@` の単位と対象の判定」のうち生の `@` の部分、§1.3 の決定 4）。
    -   `mentionAts` から、直後に名前の文字が続く条件を除く。移した `TestMentionAts` の行のうち、直後が空白・記号・U+200B・末尾の行を新しい期待に改める。
    -   投稿の経路（`insertAfterAts` を通る `neutralizeMentions`・`neutralizeProse`、`cutStartsMention`、`findMention`）には、名前の文字の条件を保つ Slack 専用の包みを通し、投稿の振る舞いを変えない。既存の Slack のテスト（`slack_render_test.go`・`slack_message_test.go`・`golden_test.go`）を変えずに通すことで確かめる。
    -   ステップ 1-2 のコミットとも、ステップ 2-2 のコミットとも分けたコミットにする（設計書 3.2）。
-   [ ] **ステップ 2-2**: 無効化の部品を作る（設計書 3.2、3.3、6.1）。
    -   `mention.go` に、`@` の単位（生の `@` と、`@` を表しうる文字参照）を探し、対象の条件と URL のパスの `/@` の 5 つの条件を、前から 1 回たどって判定し、U+200B を入れる処理を加える。入力の長さに比例する時間で終える（設計書 1.2）。
    -   `NeutralizedDisplay`・`NeutralizedDisplayMarkdown`・`NeutralizeHeading` を設計書 3.2 の形で `display_render.go` に置き、`DisplayMarkdown` を置き換える（§1.5）。呼び出し元の `internal/server/status.go:135` は、ステップ 2-4 で変える。
    -   `display_render_test.go` に、設計書 7.1 の「無効化の部品」「層を分ける入力」「時間」を確かめるテストを置く。
        -   `TestNeutralizedDisplayMarkdown`: 見出しの行・チャンネル名の行・本文（コードスパンとフェンス付きコードブロックの中を含む）の表のテスト。要件書 §3.1 の例と、設計書 6.3 の U1〜U6・R1・R2・S1〜S3 の形を行に入れ、期待値を U+200B を入れる位置で書く。すべての行で、入れた U+200B を除くと `renderDisplayMarkdown` の結果と一致することと、`Neutralized` の値（表示用の Markdown がもともと U+200B を含み無効化しない行で偽を含む）を確かめる。
        -   `TestNeutralizedDisplayMarkdownURLPathConditions`: 条件 1〜5 のそれぞれについて、ほかの 4 つを満たしその 1 つだけを満たさない入力が無効化されることと、条件 2 の `<` の分かれ道の当たる入力と当たらない入力を確かめる。
        -   `TestNeutralizedDisplayMarkdownKeptAts`: 語の途中の `@` だけが当たる入力と、URL のパスの `/@` だけが当たる入力を分け、それぞれ変えないことを確かめる。各入力について、先にもう一方の規則が当たらないことを確かめる。
        -   `TestNeutralizedDisplayMarkdownLinearTime`: `/@`・`[`・`@`・`://` を多く含み、応答の大きさの上限に近い長さの入力で、決まった時間の中で終わることを確かめる。
        -   `TestNeutralizedDisplayMarkdownRejectsUnpublishable`（§1.5）: `CheckPublishable` のエラーを返す。
        -   `TestNeutralizeHeading`: URL のパスの `/@` を含むすべての対象の `@` を無効化することを確かめる。
    -   URL のパスの `/@` の判定を外す（いつも無効化する）と `TestNeutralizedDisplayMarkdownKeptAts` が、条件のどれか 1 つの判定を外すと `TestNeutralizedDisplayMarkdownURLPathConditions` の該当の行が失敗することを確かめ、commit のメッセージに書く。
-   [ ] **ステップ 2-3**: 記事のキャッシュの記録に、生成の開始が送った動画のタイトルを持たせ、`Snapshot` で返す（設計書 3.7、§1.3 の決定 6）。`internal/articlecache/cache_test.go` に `TestSnapshotVideoTitle` を加え、タイトルがあるとき・ないとき（空文字列）に `Snapshot` が返す値を確かめる。
-   [ ] **ステップ 2-4**: 状態の取得の応答を改める（設計書 3.1、3.6 の `statusArticle`）。
    -   `done` の `article` を、`markdown`（無効化した表示用の Markdown）・`mentionsNeutralized`・`videoTitle`・`channelName`（`NeutralizeHeading` の値）・`file`・`cost`・`durationMs` にする。`webhookConfigured` と `publish` は PR-3 まで残す。
    -   `maxStatusSize`（`status.go:172`）が、加えたメンバと無効化で増えたバイトを含めて測ることを確かめる。
    -   `internal/server/status_test.go` に `TestStatusArticleNeutralized` を加える。`article` のメンバの集まりがちょうど設計書 3.1 の形であること、`markdown` が `NeutralizedDisplayMarkdown` の値で `mentionsNeutralized` がその `Neutralized` と同じこと、`videoTitle`・`channelName` が無効化した値で、ないときは空文字列であること、`file` が `FileContent` の値と同じことを確かめる。いまの投稿が拒否する並び（コードの中の対象の `@`、`AC/@x`、`<!channel>`、文字参照、`<a|b>`）を含む記事も `markdown` を返すことを確かめる。既存の `TestStatusDoneArticle` は新しいメンバに合わせる。
    -   `cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd` と `TestStatusResponseFitsRelay` を新しい応答の形に合わせる。
-   [ ] **ステップ 2-5**: 拡張の応答の検査・見出し・コピー・通知を改める（設計書 3.10 のうち投稿の部分を除くもの）。
    -   `response.ts`: `done` の記事の決まったメンバに `mentionsNeutralized`・`videoTitle`・`channelName` を加える。`Article` の型を設計書 3.10 の形にする。
    -   `renderViewer.ts` の `renderViewerHeader`: 完了した記事では応答の `videoTitle`・`channelName` を表示し、`videoTitle` が空ならビューアの URL のタイトルのまま、`channelName` が空ならチャンネル名の行を隠す。完了する前はビューアの URL の値のまま。
    -   `viewerApp.ts` の `copyArticle`: `article.markdown` をそのまま書き、`ActionNotice` の `copied` に `mentionsNeutralized` を持たせ、真なら `viewer.copiedNeutralized` を示す。
    -   カタログ: `viewer.copiedNeutralized` を英語（文面は設計書 1.2）と日本語（要件書 AC-18 の文）に加える。
    -   `scripts/popup-preview.ts` と `test/helpers/job.ts` の `done` の記事に、加えたメンバを持たせる。
    -   テスト（`response.test.ts`・`renderViewer.test.ts`・`viewerApp.test.ts`）: 加えたメンバを求め、`mentionsNeutralized` を欠く応答を想定外として拒否し記事を表示もコピーもしないこと、見出しの表示、コピーの通知の 3 つの場合（無効化した・しなかった・失敗した）、`copy` イベントを送っても通知の領域が変わらないことを確かめる。`popupHtml.test.ts` の通知の領域（`role="status"`）の確かめは、そのまま通ることを確かめる。
-   [ ] **ステップ 2-6**: `CLAUDE.md`「Declare, don't infer」に、無効化の部品の例外を、設計書 3.12 の表の `CLAUDE.md` の行のとおりの範囲・理由・境界で加える（§1.3 の決定 5）。
-   [ ] **ステップ 2-7**: AC-16a の差分の記録を書く（§7 の「差分の記録」の PR-2 の欄）。

**完了の条件:** 上のテストが加わり通る。`make test`・`make lint`・`make deadcode`・`make ext-check` が通る。この PR の後も、ビューアの投稿の操作は動く（投稿の削除は PR-3）。

### フェーズ 3: ジョブの API の投稿を消す（設計書 8 章の 4 の前半）

**変更するファイル:** `internal/server/`（`generation.go`・`status.go`・`handler.go`・`response.go`・`jobs_api.go`・`server.go`・`publish.go`・`test_helpers.go` とテスト）、`internal/articlecache/`（`record.go`・`cache.go`・`cache_test.go`）、`internal/nmhost/relay.go`・`relay_test.go`、`cmd/yt2column-server/run.go` とテスト・`test_helpers.go`、`cmd/yt2column-nmhost/main_test.go`、`docs/dev/guides/development.md`（拡張とガードが読む表）、`internal/doctestutil/guides.go`、拡張（`src/`・`static/`・`scripts/`・`test/`）

-   [ ] **ステップ 3-1（書き換えのコミット）**: 残る関数を、消すものから切り離す。
    -   `completeGeneration` が `PrepareSlackMessages` を呼ばず、記事をいつも `done` にする（設計書 3.6 の「完了の時点の検査」）。成功のログの行と `logEviction` の行から `publish` の属性を除く。
    -   `CompletePostable`・`CompleteUnpostable` を `Complete` にまとめ、`evictable` を、`done` の記録をいつも落とせる形にする（設計書 3.7、§1.5）。
    -   `stopNoticeLogger`（`cmd/yt2column-server/run.go:268`〜`276`）から投稿の待ちの文を除き、二度目の合図ですぐ終わる案内を残す。
    -   `TestGenerationUnpostable` を `TestGenerationMentionsDone`（§1.5）に書き換え、メンションを含む本文といまの分割の上限を超える本文の記事が `done` になり、成功のログの行の属性の集まりがちょうど `key`・`videoId`・`model`・`modelVersion`・`duration` であることを確かめる。`internal/articlecache/cache_test.go` に `TestEvictsDoneRecord` を加え、`done` の記録が上限で落ちることを確かめる。`cmd/yt2column-server/run_test.go::TestRunLogsListeningAndStop` の停止の案内の確かめを新しい文に合わせる。
-   [ ] **ステップ 3-2（削除のコミット、Go）**: 投稿の操作・投稿のジョブ・投稿の状態を消す（設計書 3.6・3.7・3.8）。
    -   `internal/server`: `publish.go`・`publish_test.go`、`routePublish`・`PublishSuffix`、`webhook_not_configured` と `article_not_ready`・`article_not_found`・`article_not_postable`、`statusResponse.WebhookConfigured`・`statusJob.Publish` と `statusPublish`・`publishStatus`・`publishStateCode`・`unpostableCode`・`publishFailureCode`、`Options.Webhook`・`WebhookOptions`・`WebhookPublisher`・`webhookConfigured()`・`postJobs`。
    -   消すテスト（それぞれ 1 つずつ確かめて消す）:
        -   [ ] `internal/server/status_test.go::TestStatusWebhookConfigured`
        -   [ ] `internal/server/server_test.go::TestNewWebhookOptions`
        -   [ ] `internal/server/server_test.go::TestNewCopiesWebhookOptions`
        -   [ ] `internal/articlecache/cache_test.go::TestEvictionKeepsRunningAndPosting`
        -   [ ] `internal/articlecache/cache_test.go::TestStalePublishRunAfterEviction`
        -   [ ] `internal/articlecache/cache_test.go::TestCompleteUnpostable`
        -   [ ] `internal/articlecache/cache_test.go::TestBeginPublishOutcomes`
        -   [ ] `internal/articlecache/cache_test.go::TestPublishResumeFrom`
        -   [ ] `internal/articlecache/cache_test.go::TestBeginPublishConcurrentStartsOnce`
        -   [ ] `cmd/yt2column-server/run_test.go::TestBuildsSlackPublisher`
        -   [ ] `cmd/yt2column-server/run_test.go::TestStartsWithoutWebhook`
        -   [ ] `cmd/yt2column-server/run_test.go::TestApproxMinutes`（`approxMinutes` と一緒に）
    -   ほかのテストの中の、投稿の状態・`webhookConfigured`・投稿の拒否の理由・投稿のジョブを確かめる行を消す（`git grep -n -i -E 'publish|webhook' -- internal/server internal/articlecache internal/nmhost cmd/yt2column-server cmd/yt2column-nmhost` の当たりのうち、テストと `test_helpers.go`）。
    -   `internal/articlecache`: `PublishState`・`UnpostableReason`・`PublishFailureReason` とその列挙、`Snapshot` の投稿の欄、記録の投稿の欄と `publishGen`・`delivered`、`BeginPublish`・`PublishRun`、`Eviction.Publish`。
    -   `internal/nmhost`: `OperationPublish`・`opPublish`・`publishSuffix` とその経路。
    -   `cmd/yt2column-server`: `newPublisher` の配線、`listening` の行の `webhookConfigured`、Webhook がないことの警告の行、`approxMinutes`。`cfg.WebhookURL()` を `redact.SecretStrings` に渡す行は PR-4 まで残す。
    -   消すことで網羅率が下がる残る関数があれば、このコミットの前に、その関数を確かめるテストを加えるか移す。
-   [ ] **ステップ 3-3（テストを加えるコミット、Go）**: 消した後の振る舞いを確かめるテストを加える。
    -   `internal/server/jobs_api_test.go::TestJobsTargetRejections` に、`POST /jobs/{key}/publish` が `not_found` で本文を読まず、キャッシュを変えないことを確かめる行を加える。
    -   `internal/nmhost/relay_test.go::TestRelayNewFormRejections` に、`op` が `publish` のメッセージがサーバへ何も送らずに `protocol_error` になる行を加える。`cmd/yt2column-nmhost/main_test.go` の `publish` を受け付ける確かめを、同じ拒否の確かめに改める。
    -   `internal/server/status_test.go::TestStatusArticleNeutralized` に、応答の最上位と `job` のメンバの集まりがちょうど設計書 3.1 の形（`webhookConfigured`・`publish` がない）であることを加える。`cmd/yt2column-server/run_test.go::TestRunLogsListeningAndStop` に、`listening` の行の属性の集まりと、警告の行がないことを加える。
    -   `cmd/yt2column-server/docs_test.go` の API の表の確かめから `PublishSuffix` の行を除き、`docs/dev/guides/development.md` の API の表の投稿の行を同じコミットで消す。
-   [ ] **ステップ 3-4（拡張の書き換え・削除）**: 設計書 3.10 の投稿の部分。
    -   書き換えのコミット: `poll.ts` で `done` の記事のときはいつもポーリングを止め、`JobDisplay` から `publish`・`webhookConfigured` を除き、`response.ts` が `done` の `job` の `publish` と `ok` の応答の `webhookConfigured` を受け付けない形にする。`poll.test.ts`・`poller.test.ts`・`response.test.ts`・`jobView.test.ts`・`launch.test.ts`・`test/helpers/job.ts` の応答の例と期待を新しい形に合わせる。
    -   削除のコミット: ビューアの投稿のボタンと状態の行（`static/viewer.html`・`viewer.css`）、`publishArticle` と投稿中の状態、投稿の状態の告知、`PublishOpNotice` の `none` 以外、`classifyPublish`・`PublishOutcome`・投稿の拒否の理由、`hostMessage.ts` の `publish` と `publishMessage`、`sendMessages.ts` の `publishResultMessage`、カタログの投稿のメッセージ（設計書 3.10 の「メッセージのカタログ」の一覧。`repeatedPostStep` は、ほかの使い道が残らなければ消す）、`scripts/popup-preview.ts` の投稿の状態の行ごとのページ、それらを確かめるテスト（`renderViewer.test.ts` の `publish rows`・`publish state announcements` の describe、`viewerApp.test.ts`・`entryPoints.test.ts`・`popupHtml.test.ts`・`hostMessage.test.ts`・`sendMessages.test.ts`・`startCoordinator.test.ts`・`popupPreview.test.ts`・`catalog.test.ts`・`metadataDocs.test.ts` の投稿の部分）。`make ext-coverage-compare` で確かめる。
    -   `serverReasons.test.ts`・`jobCodes.test.ts` が読む `development.md` の表（投稿の状態と理由の表、投稿の拒否の理由の行）と `test/helpers/developmentDoc.ts`・`internal/doctestutil/guides.go` の見出しを、拡張のコードと同じコミットで改める。
    -   `src/poller.ts`・`src/send.ts`・`static/theme.css`・`test/secretsAbsent.test.ts` のコメントの投稿への言及を改める。
    -   `popupHtml.test.ts` のビューアの要素の並びの確かめから投稿のボタンと行を除き、残る要素の並びが見た目の順であることを確かめる（要件書 4.6 節のフォーカスの順）。
-   [ ] **ステップ 3-5**: `.golangci.yml` の `server-no-pipeline`・`articlecache-boundaries` の理由の文から投稿の操作の post job と Webhook を除く（規則は変えない）。

**完了の条件:** 上のテストが加わり通る。削除のコミットごとに coverage-compare が PASS し、その出力が commit のメッセージにある。`make test`・`make lint`・`make deadcode`・`make ext-check` が通る。

### フェーズ 4: 内部に残る投稿のコードを消す（設計書 8 章の 4 の後半）

**変更するファイル:** `internal/publisher/`（Slack のコード、`mention.go`、`golden_test.go`、`testdata/golden/*.slack.txt`、`makefile_test.go`、`test_helpers_slack.go`、`testutil/`）、`internal/writer/markdown.go`・`markdown_test.go`、`internal/slackwebhook/`、`internal/config/`、`internal/redact/`、`cmd/yt2column-server/run.go`・`test_helpers.go`、`cmd/yt2column/` のテスト、`cmd/yt2column-nmhost/main_test.go`、`internal/doctestutil/readme.go`、`Makefile`、`docs/dev/conventions/package_reference.md`（ガードが読む行）、`docs/tasks/0083_remove_slack_webhook/_checks/slack_integration_removed.sh`（新規）

-   [ ] **ステップ 4-1（削除のコミット）**: `internal/publisher` の投稿のコードを消す（設計書 3.4）。
    -   消すファイル: `slack.go`、`slack_errors.go`、`slack_integration_test.go`、`slack_test.go`、`slack_errors_test.go`、`slack_articles_test.go`、`test_helpers_slack.go`、`makefile_test.go`、`testutil/integration.go`、`testutil/integration_settings_test.go`、`testdata/golden/*.slack.txt`（10 ファイル）。
    -   消す部分: `slack_message.go`・`slack_render.go` の残り（ステップ 2-1 の Slack 専用の包み、`renderSlackPost`・`slackFootnote`・`stageModels`、`neutralizeMentions`・`neutralizeProse`・`inRanges`）とそのテストのファイル（`slack_message_test.go`・`slack_render_test.go`）、`insertAfterAts`・`followsSlash`・`isMentionNameChar`（無効化の部品が使わなければ）、`golden_test.go` の `.slack.txt` の比較と `TestRenderGoldenNeutralized`、`testutil/mocks.go`・`mocks_test.go` の `PostMessages` の偽物（§1.4）。
    -   このコミットの前に、`replaceLookAlike` の `<`・`!` の分岐を確かめるテストを `mention_test.go` に加え（このテストは、消すテストだけが網羅していた分岐を残すためのものである）、`make coverage-compare` が PASS することを確かめる。
    -   `cmd/yt2column/docs_test.go::TestSlackDocsContract`、`cmd/yt2column/makefile_test.go` の Slack の行、`cmd/yt2column/output_layout_docs_test.go` の Slack の投稿の例の確かめと `doctestutil.ReadmeSlackPostHeading` を、同じ削除のコミットで消す。
-   [ ] **ステップ 4-2（書き換えのコミット）**: `replaceLookAlike` を `@` の見た目の似た文字だけを扱う形に減らし、ステップ 4-1 で加えたテストを合わせる（設計書 3.2）。
-   [ ] **ステップ 4-3（削除のコミット）**: `writer.ProseRanges`・`writer.ByteRange` と `internal/writer/markdown_test.go::TestProseRanges` を消す（設計書 3.4）。
-   [ ] **ステップ 4-4（書き換えのコミット）**: `redact.SecretStrings` を `SecretStrings(secrets ...secret.Secret)` にする（§1.5）。`cmd/yt2column-server/run.go` は API キーだけを渡し、HTTP/2 のデバッグの警告（`run.go:194`）から Webhook の URL を除く。`cmd/yt2column/run.go:302` の呼び出しを合わせる。`internal/redact/redact_test.go::TestSecretStrings` を新しい引数に合わせる。
-   [ ] **ステップ 4-5（削除のコミット）**: Webhook の URL の設定とパッケージを消す（設計書 3.9）。
    -   `internal/redact/redact_test.go::TestSecretStringsWebhookParts`、`internal/slackwebhook/`（`slackwebhook.go`・`slackwebhook_test.go`）、`package_reference.md` の `internal/slackwebhook` の行（`TestPackageReferenceListsPackages` のため同じコミット）。
    -   `internal/config`: `ServerConfig.webhookURL`・`WebhookURL()`・`loadSlackWebhook`・`reasonSlackWebhook` と、`config_test.go::TestLoadServerWebhookOptional`・`TestServerConfigOutputRedactsWebhook`。`cmd/yt2column-server/run.go` の `cfg.WebhookURL()` の行。
-   [ ] **ステップ 4-6**: `SLACK_WEBHOOK_URL` の行を知らないキーとして拒否する（設計書 3.9、AC-05・AC-06）。
    -   `serverFileKeys` から `slackEnv` を除き、雛形から 3 行を消し、`slackEnv`・`SlackWebhookEnvForTest` を消す。同じコミットで `cmd/yt2column-server/test_helpers.go:44`〜`48`・`143` と `cmd/yt2column/docs_test.go:557` の設定から `SLACK_WEBHOOK_URL` を除く。
    -   `internal/config/serverfile_test.go::TestParseServerFileRejects` に、`SLACK_WEBHOOK_URL` の行（値があるとき・空のとき）が `ErrConfigFileFormat` と行の番号で拒否され、エラーの文字列が値を含まない行を加える。`TestParseServerFileAccepts` に、`SLACK_WEBHOOK_URL` を含む `#` の行を受理する行を加える。`TestServerFileTemplate` に、雛形が `SLACK_WEBHOOK_URL` を含まないことを加える。
    -   `cmd/yt2column-server/run_test.go` に `TestRefusesSlackWebhookLine` を加え、設定ファイルに `SLACK_WEBHOOK_URL` の行があるとサーバが起動せず（終了コード `2`）、標準エラーが値を含まないことを確かめる。
    -   `serverFileKeys` に `slackEnv` を戻すと `TestParseServerFileRejects` の加えた行と `TestRefusesSlackWebhookLine` が失敗することを確かめ、commit のメッセージに書く。
-   [ ] **ステップ 4-7**: `Makefile` から `test-integration-slack`・`SLACK_INTEGRATION_TIMEOUT` と `.PHONY` の項目を消す（設計書 3.11）。
-   [ ] **ステップ 4-8**: 残りの参照を改める。
    -   `internal/config/envaccess_test.go:59` の `secretEnvNames` から `SLACK_WEBHOOK_URL` を除き、`:553`・`:557` の自己確かめの例を、一覧に残る名前に改める。`cmd/yt2column-nmhost/main_test.go:126` の `fullEnv` から `SLACK_WEBHOOK_URL` を除く。
    -   §1.4 の「Go のコメントだけで Webhook を例に挙げるもの」の各行の文から、Webhook と投稿を除く。
    -   `.golangci.yml` の `publisher-no-server` のコメントと `articlecache-boundaries` の `desc` から Webhook を除く。
    -   `git grep -n -i -E 'webhook|slack' -- '*.go' '*.ts' Makefile .golangci.yml` の残りが、§1.4 の「変えないもの」と、`SLACK_WEBHOOK_URL` の拒否のテストだけであることを確かめ、その一覧を commit のメッセージに書く。
-   [ ] **ステップ 4-9**: `_checks/slack_integration_removed.sh` を書いて実行する（AC-09）。`Makefile` に `test-integration-slack` のターゲットがないことと、`cmd/`・`internal/`・`Makefile`・`.github/` に `YT2COLUMN_TEST_SLACK_WEBHOOK_URL` と `YT2COLUMN_SLACK_INTEGRATION` がないことを確かめる。シェルで書く。各チェックが少なくとも 1 つの壊し方で失敗することを確かめ、commit のメッセージに書く。出力・終了コード・比べたコミットの ID を §7 の「PR-4 の実行の記録」に書く。
-   [ ] **ステップ 4-10**: AC-16a の差分の記録を書く（§7 の「差分の記録」の PR-4 の欄）。

**確かめ直し。** ステップ 4-9 の比べたコミットより後に、`Makefile` か `cmd/`・`internal/`・`.github/` を変える commit が入ったら（`/fixpr` の修正を含む）、ステップ 4-9 をやり直して §7 の記録を改める。

**完了の条件:** 削除のコミットごとに coverage-compare が PASS し、その出力が commit のメッセージにある。`_checks/slack_integration_removed.sh` の記録が §7 にある。`make test`・`make lint`・`make deadcode` が通る。

### フェーズ 5: 文書・エージェントの指示と手動の確かめ（設計書 8 章の 5・6）

**変更するファイル:** 設計書 3.12 の表の文書、`docs/dev/conventions/test_organization.md`、`docs/dev/guides/cli.md`・`claude_output_limit.md`・`pricing_update.md`・`docs/dev/conventions/mermaid_reference.md`（投稿を今の機能として述べる箇所があれば）、ADR、本計画

-   [ ] **ステップ 5-1**: `docs/dev/security.md` を改める（設計書 3.12 の表の行、AC-21）。
-   [ ] **ステップ 5-2**: `README.md` を改め、`/mktrans README.md` で `README.en.md` に写す（設計書 3.12 の表の行、AC-22・AC-23）。ガードのテスト（`TestReadmeSyncMarker`、`metadataDocs.test.ts`、`manifest.test.ts`、`TestReadmeHostExitCodes` ほか）が通ることを、`make test` と `make ext-test` で確かめる。
-   [ ] **ステップ 5-3**: `docs/dev/guides/development.md` の残り（ジョブの API の 2 つの操作、状態の取得の応答の形）を改める。
-   [ ] **ステップ 5-4**: `CLAUDE.md`（「Architecture Overview」「Tool Execution Safety」「Commands」「Development Notes」の `16,383` の記述と `make test-integration-slack`）、`AGENTS.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/test_organization.md`（「Golden Files」の節の `.slack.txt` と `TestRenderGoldenNeutralized`）、`docs/translation_glossary.md`（要件書 6 章）を改める。
-   [ ] **ステップ 5-5**: エージェントの指示を改める。`.claude/commands/_context.md` の「Conditional-guide trigger」と「Target client environments」（AC-24 の文）、`git grep -n -i -E 'webhook|slack' -- .claude .github` が返す指示（`runplan.md`・`mkarch.md`・`mkplan.md`・`_lib/opus-review.md`・`.github/workflows/deepseek-review.yml`）。`mkplan.md`・`runplan.md` の「Domain examples」の例のうち、投稿の機能を前提にするものも改める。
-   [ ] **ステップ 5-6**: ADR を改める（設計書 3.12 の表の ADR の行）。ADR-0005・0006・0008・0010・0018・0022・0027・0028・0031・0034・0043 の #770 へリンクした食い違いの注記を消し、ADR-0009 の背景の投稿の記述を直す。どちらも編集上の訂正として、commit のメッセージに訂正の行を書く（`docs/dev/conventions/adr.md`）。`git grep -n -i -E 'webhook|slack|投稿' -- docs/dev/adr` が返すほかの ADR（ADR-0003・0024・0037 など）は、記述が経緯としての言及か、今の機能の説明かを読んで、今の機能の説明なら直す。`cmd/yt2column/adr_test.go::TestADRRecords` が通ることを確かめる。
-   [ ] **ステップ 5-7**: `git grep -n -i -E 'webhook|slack|投稿' -- docs ':!docs/tasks' ':!docs/research' ':!docs/dev/adr'` の残りを読み、投稿の機能を今の機能として述べる箇所が残っていないことを確かめる。
-   [ ] **ステップ 5-8**: AC-17 の手動の確かめを行い、§7 の「PR-5 の実行の記録」に日付・Mattermost のサーバとデスクトップアプリの版・ブラウザ（Chrome と Brave）を書く（設計書 7.3）。確かめに使う記事の内容は要件書 AC-17 のとおりで、設計書 6.3 の U1〜U6・R1・R2・S1〜S4 の形も同じ手順で貼り付ける。拡張とサーバは PR-5 のブランチから作ったものを使う。通知されたものがあれば、何も直さずに止まり、人に報告する。

**完了の条件:** 設計書 3.12 の表の各行の文書が改まり、ADR の注記がない。§7 に AC-17 の記録がある。`make test`・`make lint`・`make ext-test` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（フェーズ 1） | 本文の UTF-8 の検査、`mention.go` | 本文が正しい UTF-8 でない記事は `article_invalid` になり、`--out` は書かない |
| M2（フェーズ 2） | 無効化の部品、応答の新しいメンバ、ビューアの表示・見出し・コピー・通知 | ビューアの表示とコピーが無効化した文字列になる。投稿の操作はまだ動く |
| M3（フェーズ 3） | ジョブの API とビューアから投稿がなくなる | 投稿の操作・投稿の状態・Webhook の設定の表示がなくなり、記事はいつも `done` になる |
| M4（フェーズ 4） | 内部の投稿のコード・`SLACK_WEBHOOK_URL`・`make test-integration-slack` がなくなる | 設定ファイルの `SLACK_WEBHOOK_URL` の行でサーバが起動しなくなる |
| M5（フェーズ 5） | 文書・指示・ADR、手動の確かめの記録 | 文書が今の機能と一致する |

順序は設計書 8 章のとおりである。PR-3 の後、PR-4 のマージまでは、設定ファイルの `SLACK_WEBHOOK_URL` を読むが使わない状態になる。PR-3・PR-4 の後、PR-5 のマージまでは、README などが投稿の機能を述べたまま残る。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 設計書 7.1 のとおり。無効化の部品は表のテストで、要件書 §3.1 の例と設計書 6.3 の各形を、U+200B を入れる位置で確かめる（ステップ 2-2）。層を分ける入力は、片方の規則だけが当たる形にし、もう一方が当たらないことを先に確かめる。サーバ・キャッシュ・ホスト・設定・拡張は、各ステップのテストで確かめる。
-   **結合テスト:** `cmd/yt2column-server/relay_e2e_test.go` で、ホストからサーバまでの状態の取得が新しい形の応答を返すことを確かめる（ステップ 2-4、3-3）。Webhook の統合テストは消える。
-   **テストの削除:** 削除のコミットごとに coverage-compare を走らせ、出力を commit のメッセージと PR の説明に書く（§1.2）。1 回の実行は Go で数分かかるので、10 分以上の時間の制限で走らせる。
-   **ゴールデンのファイル:** `<case>.md` は変えず、AC-16a の確かめに使う。`.slack.txt` は消す（§1.3 の決定 2）。
-   **一回限りの確かめ:** `_checks/slack_integration_removed.sh`（AC-09、PR-4）と、AC-16a の差分の記録（PR-1・PR-2・PR-4）。
-   **手動の確かめ:** AC-17（PR-5）、AC-16 の見た目、AC-21〜AC-24 の文書の文。
-   **後方互換:** 求めない（要件書 4.4 節）。前の版のサーバの応答を新しい拡張が想定外として拒否することは、ステップ 2-5 のテストで確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 無効化の規則の誤りで、通知される `@` が残る | 設計書 6.3 の各形を表のテストに入れる。迷う形は無効化する側に倒す（設計書 1.1）。AC-17 で Mattermost に貼り付けて確かめる |
| 規則の変更（ステップ 2-1）で、PR-2 から PR-4 までの投稿が壊れる | Slack 専用の包みを通し、既存の Slack のテストを変えずに通す（§1.3 の決定 4） |
| 無効化が遅く、状態の取得のたびに応答が遅れる | 前から 1 回たどる形にし、`TestNeutralizedDisplayMarkdownLinearTime` で確かめる |
| 削除のコミットに書き換えが混じり、coverage-compare が止まる | 書き換えのコミットを先に分ける（§1.2）。止まったら、コミットを分け直す |
| 消すテストだけが網羅していた残る関数の網羅率が下がる | 削除のコミットの前に、テストを加えるか移す（ステップ 3-2、4-1） |
| PR-3 と PR-4 の間、`SLACK_WEBHOOK_URL` を読むが使わない | 利用者は 1 人で、互換を求めない。PR-4 で拒否に変わる |
| PR-3・PR-4 の後、PR-5 のマージまで文書が古い | PR-5 を続けて行う。ガードが読む箇所はコードと同じコミットで改める |
| 文書を読むガードのテストが、コードの変更と同時に壊れる | §1.4 のガードの一覧を、各ステップで改める |
| AC-17 で通知される形が見つかる | 何も直さずに止まり、人に報告する（ステップ 5-8）。規則の変更は決定の変更として扱う |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1（ステップ 1-1〜1-3）
-   [ ] フェーズ 2（ステップ 2-1〜2-7）
-   [ ] フェーズ 3（ステップ 3-1〜3-5）
-   [ ] フェーズ 4（ステップ 4-1〜4-10）
-   [ ] フェーズ 5（ステップ 5-1〜5-8）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の意味は次のとおりである。

-   `test`: `make test`・`make ext-test` が走らせるテスト。
-   `static`: 型の検査（`make ext-typecheck`）か、`_checks/` のスクリプト。スクリプトは PR のブランチで実行し、出力・終了コード・比べたコミットの ID を下の実行の記録に書く。`make test` と CI は実行しない。
-   `diff`: `docs/dev/conventions/requirements_process.md` §4 のとおり。
-   `manual`: PR のレビューで読むか、手で操作して確かめる。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `extension/test/popupHtml.test.ts`（ビューアの要素の並び）、`extension/test/viewerApp.test.ts`（保存とコピーが動く）、`extension/test/metadataDocs.test.ts`、ビューアを開いて見る | test、manual | ステップ 3-4 | PR-3 |
| AC-02 | `internal/server/jobs_api_test.go::TestJobsTargetRejections`、`::TestUnknownPathTouchesNothing` | test | ステップ 3-2・3-3 | PR-3 |
| AC-03 | `internal/server/status_test.go::TestStatusArticleNeutralized`、`cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd`、`extension/test/response.test.ts` | test | ステップ 2-4・2-5・3-3・3-4 | PR-2、PR-3 |
| AC-04 | `internal/nmhost/relay_test.go::TestRelayNewFormRejections`、`cmd/yt2column-nmhost/main_test.go` | test | ステップ 3-2・3-3 | PR-3 |
| AC-05 | `internal/config/serverfile_test.go::TestParseServerFileRejects`、`::TestParseServerFileAccepts`、`cmd/yt2column-server/run_test.go::TestRefusesSlackWebhookLine` | test | ステップ 4-6 | PR-4 |
| AC-06 | `internal/config/serverfile_test.go::TestServerFileTemplate` | test | ステップ 4-6 | PR-4 |
| AC-07 | `internal/server/generation_test.go::TestGenerationMentionsDone` | test | ステップ 3-1 | PR-3 |
| AC-08 | `cmd/yt2column-server/run_test.go::TestRunLogsListeningAndStop`、`cmd/yt2column-server/signal_test.go::TestSignalWaitsForJob`・`::TestSecondSignalExitsAtOnce` | test | ステップ 3-1・3-2・3-3 | PR-3 |
| AC-09 | `_checks/slack_integration_removed.sh`（下の「PR-4 の実行の記録」） | static | ステップ 4-7・4-9 | PR-4 |
| AC-09a | `internal/writer/article_test.go::TestArticleCheckPublishableBodyUTF8`、`internal/server/generation_test.go::TestGenerationUnpublishable`、`internal/publisher/file_test.go::TestFilePublisherRejectsArticle` | test | ステップ 1-1 | PR-1 |
| AC-10 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`（契約） | test | ステップ 2-2 | PR-2 |
| AC-11 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`、`::TestNeutralizedDisplayMarkdownURLPathConditions` | test | ステップ 2-2 | PR-2 |
| AC-12 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdownKeptAts` | test | ステップ 2-2 | PR-2 |
| AC-13 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`、`internal/publisher/mention_test.go::TestMentionAts` | test | ステップ 2-1・2-2 | PR-2 |
| AC-14 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`、`internal/server/status_test.go::TestStatusArticleNeutralized`、`internal/server/generation_test.go::TestGenerationMentionsDone` | test | ステップ 2-2・2-4・3-1 | PR-2、PR-3 |
| AC-15 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`（無効化しない行） | test | ステップ 2-2 | PR-2 |
| AC-16 | `extension/test/renderViewer.test.ts`（`article.markdown` をそのまま描画する）、表示の見た目を読む | test、manual | ステップ 2-5 | PR-2 |
| AC-16b | `internal/publisher/display_render_test.go::TestNeutralizeHeading`、`internal/server/status_test.go::TestStatusArticleNeutralized`、`internal/articlecache/cache_test.go::TestSnapshotVideoTitle`、`extension/test/renderViewer.test.ts`（`renderViewerHeader`）、手動の確かめ（ステップ 5-8） | test、manual | ステップ 2-2〜2-5 | PR-2、PR-5 |
| AC-16a | `internal/publisher/golden_test.go::TestRenderGolden`（`<case>.md`）、`internal/publisher/file_render_test.go::TestFileContentMatchesFilePublisher`、`internal/server/status_test.go::TestStatusArticleNeutralized`（`file`）、記録した差分の読み取り（下の「差分の記録」） | test、diff | ステップ 1-3・2-7・4-10 | PR-1、PR-2、PR-4 |
| AC-17 | 手動の確かめ（ステップ 5-8、下の「PR-5 の実行の記録」） | manual | ステップ 5-8 | PR-5 |
| AC-18 | `extension/test/viewerApp.test.ts`（無効化したときの通知）、`extension/test/catalog.test.ts`、`make ext-typecheck`（両方のカタログが `viewer.copiedNeutralized` を持つ） | test、static | ステップ 2-5 | PR-2 |
| AC-19 | `extension/test/viewerApp.test.ts`（無効化しなかったとき・失敗したときの通知） | test | ステップ 2-5 | PR-2 |
| AC-19a | `extension/test/viewerApp.test.ts`（`copy` イベントで通知の領域が変わらない） | test | ステップ 2-5 | PR-2 |
| AC-20 | `extension/test/popupHtml.test.ts`（通知の領域が `role="status"`）、`extension/test/viewerApp.test.ts`（フォーカスが動かない）、`make ext-typecheck` | test、static | ステップ 2-5 | PR-2 |
| AC-21 | `docs/dev/security.md` を読む | manual | ステップ 5-1 | PR-5 |
| AC-22 | `TestReadmeSyncMarker`、`make test`・`make ext-test` の文書のガード、README と §2.2 の文書を読む | test、manual | ステップ 3-3・3-4・4-1・4-5・5-2〜5-7 | PR-3、PR-4、PR-5 |
| AC-23 | README の設定ファイルの説明を読む | manual | ステップ 5-2 | PR-5 |
| AC-24 | `.claude/commands/_context.md` の「Target client environments」を読む | manual | ステップ 5-5 | PR-5 |

### 差分の記録

AC-16a は、`<case>.md` のゴールデンのファイルの差分が空であることを、記録した差分の読み取りで確かめる。比べるパスは、`internal/publisher/testdata/golden/` の 10 の `.md` のファイルである。差分は空になるはずなので、読む量は小さい。ゴールデンのファイルを作る `renderFileArticle` に近いコードを変える PR-1・PR-2・PR-4 で記録する。

PR-1:

Diff record (AC-16a):

- Compared paths: `internal/publisher/testdata/golden/channel_heading.md` `internal/publisher/testdata/golden/channel_list.md` `internal/publisher/testdata/golden/channel_trailing_hash.md` `internal/publisher/testdata/golden/cost_unknown.md` `internal/publisher/testdata/golden/mentions.md` `internal/publisher/testdata/golden/mid_word_at.md` `internal/publisher/testdata/golden/mixed_models.md` `internal/publisher/testdata/golden/same_model.md` `internal/publisher/testdata/golden/shorten_skipped.md` `internal/publisher/testdata/golden/without_channel.md`

PR-2:

Diff record (AC-16a):

- Compared paths: `internal/publisher/testdata/golden/channel_heading.md` `internal/publisher/testdata/golden/channel_list.md` `internal/publisher/testdata/golden/channel_trailing_hash.md` `internal/publisher/testdata/golden/cost_unknown.md` `internal/publisher/testdata/golden/mentions.md` `internal/publisher/testdata/golden/mid_word_at.md` `internal/publisher/testdata/golden/mixed_models.md` `internal/publisher/testdata/golden/same_model.md` `internal/publisher/testdata/golden/shorten_skipped.md` `internal/publisher/testdata/golden/without_channel.md`

PR-4:

Diff record (AC-16a):

- Compared paths: `internal/publisher/testdata/golden/channel_heading.md` `internal/publisher/testdata/golden/channel_list.md` `internal/publisher/testdata/golden/channel_trailing_hash.md` `internal/publisher/testdata/golden/cost_unknown.md` `internal/publisher/testdata/golden/mentions.md` `internal/publisher/testdata/golden/mid_word_at.md` `internal/publisher/testdata/golden/mixed_models.md` `internal/publisher/testdata/golden/same_model.md` `internal/publisher/testdata/golden/shorten_skipped.md` `internal/publisher/testdata/golden/without_channel.md`

### PR-4 の実行の記録

（ステップ 4-9 で書く。）

### PR-5 の実行の記録

（ステップ 5-8 で書く。）

## 8. 成功の基準 (Success Criteria)

-   要件書の AC-01〜AC-24 が、§7 の確かめで満たされている。
-   各 PR で `make test`・`make lint`・`make deadcode` が通り、拡張を変えた PR で `make ext-check` が通る。
-   テストを消したコミットごとに、coverage-compare の PASS の出力が commit のメッセージと PR の説明にある。
-   §7 に、AC-09 の実行の記録、AC-16a の 3 つの差分の記録、AC-17 の手動の確かめの記録がある。
-   セキュリティ: Webhook の URL が設定・型・伏せ字の対象・ログのどこにも残らない。サーバが外へ送るのは LLM の API だけになる（設計書 5 章）。
-   文書: 設計書 3.12 の表の文書と ADR が、今の機能と一致する。

## 9. 次のステップ (Next Steps)

-   拡張だけの構成への移行（[#771](https://github.com/isseis/yt2column/issues/771)・[#772](https://github.com/isseis/yt2column/issues/772)・[#773](https://github.com/isseis/yt2column/issues/773)）で、無効化の部品を WASM の中で使う（設計書 9 章）。
-   確かめの対象の Mattermost のサーバの版を上げるときは、設計書 6.3 の比較をやり直す。
