# 実装計画書：Slack の Webhook への投稿の機能を廃止し、メンションの無効化をクリップボードへのコピーに移す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-11 |
| Review date | 2026-11-11 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。用語（対象の `@`・無効化・URL・文字参照・URL のパスの `/@`・表示用の Markdown・コピーする文章・無効化した表示用の Markdown・無効化の部品・`@` の単位）は、要件書 §3.1 と設計書の冒頭と同じ意味で使う。本計画は、次の用語を加える。

-   **コミットの種類:** テストの削除の確かめ方（設計書 7.4）を決めるため、コミットを次の 3 つに分ける。どのコミットも、その時点で `make test` と（拡張を変えたときは）`make ext-test` が通る。
    -   **テストのコミット:** 製品のコードを変えずにテストを移す・加える・書き換える・消すコミットと、製品の関数を丸ごと消し、それだけを確かめるテストを消すだけのコミット。テストを消したときは、`make coverage-compare BASE=<親のコミット>`（Go）と `make ext-coverage-compare BASE=<親のコミット>`（拡張）で確かめる。
    -   **書き換えを伴う削除のコミット:** 残る関数を書き換えながら、テストを消すコミット。`_checks/coverage` で確かめる（設計書 7.4、§1.3 の決定 7）。
    -   **書き換えのコミット:** 製品のコードを変え、テストは加える・書き換える・ファイルの間で移すだけで消さないコミット。比較は要らない。書き換えたテストの行は、新しい期待で確かめる。
-   **比較の記録:** テストを消したコミットの比較の出力を、最後の行まで commit のメッセージと PR の説明に書く（`CLAUDE.md`「Testing Strategy」）。
-   **`_checks/` のスクリプト:** 本タスクのディレクトリの `_checks/` に置く、一回限りの確かめのスクリプト。PR のブランチで実行し、結果を §7 か commit のメッセージに記録する。`make test` と CI は実行しない（`docs/dev/conventions/requirements_process.md` §4）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

サーバ・ホスト・拡張・設定から Webhook への投稿の機能を消し、メンションの無効化を、ビューアの表示と「Markdown をコピーする」に移す（要件書 2.1 節）。何をどう作るかは設計書 3 章が定める。本計画は、作業の順序、PR の分け方、確かめ方を定める。

### 1.2. 実装の原則

-   **設計書 8 章の依存の順に進める。** 本文の UTF-8 の検査 → 規則の移動 → 無効化の部品と応答・拡張 → 投稿の機能の削除 → 文書 → 手動の確かめ。
-   **コミットの種類を守る。** テストを消すコミットは、テストのコミットか、書き換えを伴う削除のコミットにする。テストを消さずに済む書き換えは、その前の書き換えのコミットで行う（設計書 8 章の 4）。
-   **応答の形を変える側と受ける側は、同じ PR で変える**（H-05、設計書 8 章の 3）。
-   **互いに縛り合うコードと文書は、同じコミットで変える。** 文書を読むガードのテスト（`CLAUDE.md`「Development Notes」と拡張のテスト）が読む表や値は、それを定めるコードと同じコミットで改める（設計書 3.12、8 章の 5）。ガードが読まない文書の記述は、PR-5 でまとめて改める。PR-3・PR-4 の後、PR-5 のマージまでは、文書が、消えた機能を述べたまま残る箇所がある。
-   **加えた・書き換えたテストは、確かめる対象を壊して失敗することを確かめ、commit のメッセージに書く**（`CLAUDE.md`「Testing Strategy」）。経路を取り違えやすいものは、各ステップで壊す対象を挙げる。
-   **Go と TypeScript のソースのコメント・識別子・文字列のリテラルは英語で書く**（`CLAUDE.md`「Go Idioms」）。`_checks/` のスクリプトも同じである。利用者に見せる文は、英語と日本語のメッセージのカタログに置く。
-   **各フェーズの後に、`make fmt` → `make test` → `make lint` → `make deadcode` を通す。** 拡張を変えたフェーズは `make ext-check` も通す。`README.md` を変えたコミットは `/mktrans README.md` で `README.en.md` を同期し、同じコミットに入れる（`CLAUDE.md`「Translation Guidelines」）。

### 1.3. 計画の際に決めたこと（2026-11-11）

1.  **PR を 5 本に分ける。** PR-1 は本文の UTF-8 の検査と規則の移動（設計書 8 章の 1・2）、PR-2 は無効化の部品と応答・拡張の表示・コピー・見出し（8 章の 3）、PR-3 はジョブの API の投稿の削除（サーバの投稿の操作と投稿のジョブ、ホストの `publish`、記事のキャッシュの投稿の状態、応答の `webhookConfigured`・`publish`、ビューアの投稿の操作、それらと縛り合う文書の表。8 章の 4 の前半）、PR-4 は内部に残る投稿のコードの削除（`internal/publisher` の Slack のコード、`internal/slackwebhook`、設定の `SLACK_WEBHOOK_URL`、`internal/redact`、`make test-integration-slack`、ガードのテスト。8 章の 4 の後半）、PR-5 は文書・エージェントの指示・ADR の注記と手動の確かめ（8 章の 5・6）である。理由: 投稿の削除は Go と拡張の約 30 のテストのファイルにわたり、1 本にするとレビューの量が大きすぎる。ジョブの API（応答・経路・ホスト・ビューア）と、それより内側のコードの境目で分けると、どちらの PR も単独でグリーンゲートを通る。
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
4.  **規則の変更（設計書 3.3）は、PR-1 で移した `mentionAts` をその場で変え、無効化の部品は生の `@` の判定にそれを使う。投稿の経路には、直後に名前の文字が続く条件を保つ Slack 専用の包みを通す。包みは PR-4 で投稿のコードと一緒に消す。** 理由: PR-2 の時点では、投稿の無効化（`slack_render.go:116`〜`136`）、分割の切れ目の確かめ（`slack_message.go:182`）、拒否（`slack_message.go:253`）が `mentionAtsOf` を使っている。`mentionAts` から名前の文字の条件を除くだけだと、拒否が、無効化した後の `@`＋U+200B を対象の `@` とみなし、無効化した `@` を含む記事の投稿を拒否する。その場で変えると、対象の判定が 1 つのままになる。移したテストを新しい規則に合わせて改めるという設計書 3.13 の扱いとも合う（行番号はコミット `e4cbff08` での値）。
    -   採らなかった案: 新しい規則を別の関数として加え、移した関数は投稿の振る舞いのまま PR-4 でテストと一緒に消す案。2 つの判定が 2 本の PR の間並ぶ。
    -   人の答え: その場で変えて包みを通す案を選んだ。
    -   ADR: なし（パッケージの中の関数の形である）。
5.  **`CLAUDE.md`「Declare, don't infer」の、無効化の部品の例外は、PR-5 ではなく、無効化の部品を加える PR-2 で加える。** 理由: 例外の対象になるコードが入る PR に例外がないと、そのコードが規則に反するものとしてレビューされる。設計書 3.12 の表の書く内容は変えず、時期だけを早める。ADR: なし（`CLAUDE.md` の規則は ADR にしない）。
6.  **記録が持つ動画のタイトルは、`articlecache.Subject` に持たせる。** `Subject` は `Keyer.Subject` が `KeyInput` から作り（`internal/articlecache/key.go:53`〜`56`・`91`）、`KeyInput.Title` は鍵の計算に入っている（`key.go:82`）。そのため、同じ鍵の記録は同じタイトルを持ち、`Begin` が記録を作るときに `Subject` から写せばよい。ADR: なし（設計書 3.7 の決定の範囲の、置き場所の選択である）。
7.  **テストの削除の確かめ方は、本タスクに限り `CLAUDE.md`「Testing Strategy」の手順の例外とし、0059 と 0062 の前例に従う。** 書き換えを伴う削除のコミットは、`_checks/coverage` で、関数を名前で対応づけて前後の網羅を比べる。理由: 投稿の分岐は、残る関数（`matchTarget`・`renderStatus`・`maxStatusSize`、ホストの `parseOperation`、`cmd/yt2column-server` の `run`、`config.LoadServer`、拡張の `jobView.ts`・`renderViewer.ts` など）の中にある。分岐を残したままテストだけを消すと網羅率が下がり、テストは消す識別子を参照するので分岐を先に消すこともできない。`make coverage-compare` と `make ext-coverage-compare` は、残る関数の中に削除でない差分の行があると止まる（`internal/coveragecompare/classify.go:37`〜`47`・`106`〜`131`、[ADR-0037](../../dev/adr/0037-test-deletion-coverage.md)「結果」）。承認済みの要件書 5 章と設計書 8 章の 4 は coverage-compare で確かめるとしていたので、本計画の PR で要件書の 1 章（実装計画の際に改めたこと）・2.2 節・5 章と、設計書の 1.2・3.13・7.4・8 章を改め、どちらも `draft` に戻した。要件書と設計書は本計画と一緒に承認を受ける。
    -   採らなかった案: どのコミットも道具が分類できる形にする段階のコミットを加える案。コミットが大きく増え、経路の選び方や応答の構造体のように、それでも分類できない箇所が残りうる。
    -   人の答え: 0059・0062 の前例の例外にする案を選んだ。
    -   ADR: [ADR-0037](../../dev/adr/0037-test-deletion-coverage.md) の「結果」の前例のタスクの並びに 0083 を加えた（規則は変えない）。
8.  **PR-1 で移すテストは `TestMentionAts` だけにする。** 設計書 3.13 の表の 2 行目は、`slack_render_test.go`・`slack_message_test.go` のうち対象の `@` の判定と U+200B を入れる処理のテストを移すとする。`TestNeutralizeMentions`・`TestNeutralizeProse`・`TestNeutralizeProseGivenRanges` は、Slack に固有の `neutralizeMentions`・`neutralizeProse`（`ProseRanges` の範囲を使う）を確かめるので、投稿の振る舞いが残る PR-4 までそのままにし、関数と一緒に消す。直前が `/` の `@` とコードの中の `@` の新しい扱いは、ステップ 2-2 の無効化の部品のテストが確かめる。ADR: なし。

### 1.4. 既存コード調査結果

コミット `e4cbff08`（`origin/main`）で確かめた。以下の `file:line` はこのコミットの行番号である。設計書は `ee6d14df` の行番号を使う。その後のコミットのうち、本タスクが変える箇所に関わるのは、`cmd/yt2column-server/run.go:191`（`SecretStrings` の引数に OpenAI のキーを加えた）と `Makefile`（設計書 3.11 の `Makefile:285`〜`300` の Slack のターゲットは、今は `:307`〜`317` にある）である。

-   **設計書 3.13 の表にない、変える・消すもの。** `git grep -n -i -E 'webhook|slack|publish'` を `cmd/`・`internal/`・`extension/`・`Makefile`・`.golangci.yml`・`.github/` にかけて洗い出した。各ステップで扱う。
    -   `internal/publisher/golden_test.go`（`TestRenderGolden` の `.slack.txt` の比較、`TestRenderGoldenNeutralized`）と `testdata/golden/*.slack.txt` の 10 ファイル（§1.3 の決定 2）。
    -   `internal/publisher/makefile_test.go`（`TestMakeTestIntegrationSlack`・`TestSlackIntegrationTestBuildTag`）。
    -   `internal/publisher/testutil/integration.go` の全体。設計書 3.4 は「ほかの統合テストが使う部分（`IntegrationOptions` など）は残す」とするが、このファイルの名前を `internal/publisher` の外で使うのは、どれも Slack の統合テストとそのガードだけである（`cmd/yt2column/docs_test.go:179`〜`181`、`cmd/yt2column/makefile_test.go:71`・`82`・`86`、`internal/publisher/makefile_test.go`）。そのため、残す部分はなく、ファイルの全体と `integration_settings_test.go` を消す。`testutil/mocks.go` の `FakePublisher` は `Publish` を残し、`PostMessages`・`PostErr`・`PostCalls`・`FakePostCall`・`RecordedPostCalls` を消す（`internal/pipeline/pipeline_test.go` が `Publish` の側を使う）。
    -   `internal/redact/redact.go:90`（`SecretStrings(webhookURL secret.Secret, others ...secret.Secret)`）は `internal/slackwebhook` を import し、1 つ目の引数に `SensitiveParts` を加える。`internal/slackwebhook` を消すので、引数を `...secret.Secret` だけにする（§1.5）。呼び出し元は `cmd/yt2column-server/run.go:191` と `cmd/yt2column/run.go:302` で、CLI は 1 つ目の引数に DeepSeek のキーを渡している。書き換えの後は、CLI でもそのキーに `SensitiveParts` の形が加わらなくなる。`TestSecretStringsWebhookParts`（`internal/redact/redact_test.go:202`）は消す。
    -   `internal/server`: `test_helpers.go:372`〜`474` の `loopbackWebhookTimeout`・`threeMessageArticle`（`PrepareSlackMessages` を呼ぶ）・`loopbackWebhook`、`server.go:48` の `errNilWebhookPublisher`、`:114`〜`116` の `beforePublishJob`、`test_helpers.go:285`〜`334` の `RejectReasonsForTest` の投稿の状態と理由の表、`generation_test.go:266`（`TestStatusResponseWithinLimit`）の投稿の状態の繰り返し（`:289`〜`301`）、`handler_test.go:96`・`139`。
    -   `cmd/yt2column-server` のテストの仕組みが、投稿のジョブに依っている。`signal_test.go:35`〜`125` の `TestSignalWaitsForJob`・`TestSecondSignalExitsAtOnce` は `blockingChild`（投稿を止めておく偽の Webhook の書き手、`test_helpers.go:533`〜`580` の `childModeBlock`・`blockingPublisher`）で投稿のジョブを止め、`post job succeeded` を確かめる。`server_test.go:20` の `TestServerEndToEnd` は、2 回目の起動が同じソケットで動くことを投稿で確かめる（`:52`〜`61`）。`run_test.go` の `TestStartDuringShutdownRefused`（`:547`〜`578`、`gatedPublisher`）、`TestWriterOptionsMatchCLIDefaults`（`:794`、書き手を作った直後に投稿の書き手の失敗で止める）、`TestOpenAIConfiguration`（`:709`〜`733`）、`TestOutputsOmitSecrets`（`:619`）、`TestStartupFailures` の `publisher not built` の行（`:248`）も投稿に依る。
    -   `cmd/yt2column-server/docs_test.go`: `TestServerDocsContract` の拒否の理由の確かめは、`development.md` の表、各 README の表（`checkReadmeRejectionTable`、`:490`〜`528`）、`server.RejectReasonsForTest()` を、行の数まで比べる。`webhook_not_configured`・`article_not_ready`・`article_not_found`・`article_not_postable` は `README.md:393`〜`397`・`README.en.md:580`〜`584`・`development.md:106`〜`110` にある。同じテストが、`RejectReasonsForTest` の `publish state`・`publish reason` の表と `development.md` の投稿の状態と理由の表を比べる。API の表の確かめは `server.PublishSuffix` を読む（`:108`・`463`〜`467`）。
    -   `cmd/yt2column/docs_test.go:171`（`TestSlackDocsContract`）、`:557`（`config.SlackWebhookEnvForTest()` を使う設定）、`cmd/yt2column/makefile_test.go:71`・`82`・`86`（Slack の make のターゲットと opt-in の変数）、`cmd/yt2column/output_layout_docs_test.go:110`〜`119`（README の Slack の投稿の例と `SlackMessagesForTest` の比較、`doctestutil.ReadmeSlackPostHeading`）。
    -   `internal/config`: `test_helpers.go:25`（`SlackWebhookEnvForTest`）。`serverfile_test.go:68`〜`72`・`77`〜`78`（`TestParseServerFileAccepts` が、`SLACK_WEBHOOK_URL` を受け付けるキーとして使う行。値に `#` を含む行の解析の確かめを含む）、`:128`（`TestParseServerFileRejects` の前置きの行。各行の期待する行の番号は、この行があることを前提にしている）。`config_test.go:39`・`335`・`1532`（環境の fixture の `slackEnv`）、`:930`〜`939`（`TestLoadServerValid` の Webhook の URL の確かめ）、`:2147`〜`2168`（`serverEnvWithFile`）、`:2245`〜`2258`（`TestLoadServerValueRejections` の 1 行）、`:45`・`2243`（コメント）。`envaccess_test.go:59`（`secretEnvNames` の `SLACK_WEBHOOK_URL`。`:553`・`557`・`598` が、ガードの自己確かめの例にこの名前を使う）。
    -   `cmd/yt2column-server/test_helpers.go:44`〜`48`・`143`（テストの設定ファイルに `SLACK_WEBHOOK_URL` を書く）。AC-05 の拒否を入れると、このままではテストのサーバが起動しない。拒否と同じコミットで消す。
    -   `cmd/yt2column-nmhost/main_test.go:126`（`fullEnv` が「サーバの変数」として `SLACK_WEBHOOK_URL` を持つ）。`internal/codexreview/run_test.go:123`（`t.Setenv("SLACK_WEBHOOK_URL", …)`）。
    -   `internal/doctestutil/guides.go:52`〜`53`（`development.md` の投稿の状態と理由の表の見出し）、`internal/doctestutil/readme.go:81`（`ReadmeSlackPostHeading`）。
    -   拡張: `src/core/sendMessages.ts:136`（`publishResultMessage`）、`src/poller.ts:7`・`46`（コメント）、`src/send.ts:3`（コメント）、`static/theme.css:93`（コメント）、`test/startCoordinator.test.ts:363`、`test/helpers/developmentDoc.ts:25`〜`26`（`jobCodes.test.ts` が `development.md` の投稿の状態と理由の表を読む見出し）、`test/helpers/dom.ts`、`test/secretsAbsent.test.ts:10`〜`13`（コメント）。
    -   `Makefile:322`（lint のコメントが `make test-integration-slack` を挙げる）。
    -   `.golangci.yml:88`〜`134` の depguard の規則のうち、`server-no-pipeline` の理由（「投稿の操作の post job だけが Webhook に送る」）と、`publisher-no-server`・`articlecache-boundaries` のコメントと `desc` が投稿を述べる。規則（禁じる import）は変えず、理由の文を改める。
    -   Go のコメントだけで Webhook を例に挙げるもの: `internal/secret/secret.go:2`・`22`、`internal/transcript/exec.go:43`、`internal/codexreview/run.go:165`、`internal/articlecache/key.go:4`、`internal/publisher/publisher.go:3`〜`5`、`internal/publisher/format.go:23`、`cmd/yt2column-server/main.go:4`、`cmd/yt2column/integration_env_test.go:29`、`cmd/yt2column/run_test.go:1184`。
-   **変えないもの。** `internal/transcript/exec_test.go:173` は、子のプロセスに渡さない秘密らしい変数の例として `SLACK_WEBHOOK_URL` を使う。名前は任意の例で、廃止の後も確かめの意味は変わらないので残す。`internal/coveragecompare/testdata_test.go:22` と `testdata/` は、ファイルのパスを fixture の文字列として持つだけなので残す。`extension/test/secretsAbsent.test.ts:20` の禁じる語の一覧の `SLACK_WEBHOOK_URL` は、拡張が秘密を持たないことのガードとして残す（コメントだけを改める）。
-   **パッケージの一覧のガード。** `internal/pipeline/pipeline_test.go:629`（`TestPackageReferenceListsPackages`）は、`docs/dev/conventions/package_reference.md` が製品のコードのあるパッケージをちょうど並べることを確かめる。`internal/slackwebhook` を消すコミットで、その行（`package_reference.md:31`）も消す。
-   **`development.md` と README を読むガードの縛り合い。** 投稿の状態と理由の表は、Go の `TestServerDocsContract`（`RejectReasonsForTest`）と拡張の `jobCodes.test.ts` の両方が `development.md` と比べる。拒否の理由の表は、Go の `TestServerDocsContract` が `development.md` と 2 つの README に対して、拡張の `serverReasons.test.ts`（`:88`〜`107`）が `development.md` に対して、それぞれ比べる。そのため、投稿の状態・理由・拒否の理由を消すことは、Go・拡張・`development.md`・2 つの README を 1 つのコミットで変えないと、そのコミットのテストが通らない（ステップ 3-5）。
-   **README を読むほかのガード。** `extension/test/metadataDocs.test.ts:58` はビューアの手順の `**${catalog.viewer.publish}**` を、`cmd/yt2column/docs_test.go:171` と `output_layout_docs_test.go` は投稿の上限と例を読む。カタログの項目やコードを消すコミットで、ガードの該当の確かめを消す。README のそのほかの本文は PR-5 で改める。
-   **対象の `@` の判定の使われ方（§1.3 の決定 4）。** `mentionAtsOf` は `slack_render.go:136`（`insertAfterAts`）、`slack_message.go:182`（`cutStartsMention`）、`:253`（`findMention`）が呼ぶ。`replaceLookAlike` は `:352`（`inspectMentionText`）と `:414` が呼ぶ。`mentionAts` は入力の長さに比例する 1 回のたどりで、ループの終わりは `i+1 < len(text)` である（`slack_message.go:394`〜`406`）。
-   **`writer.ProseRanges` の補助の関数。** `codeSpans`・`codeSpansExempt`・`proseLines` は `checkBodyMarkdown`（`internal/writer/markdown.go:108`〜`117`）も使うので、`ProseRanges`（`:49`）と `ByteRange`（`:32`）を消しても残る。
-   **完了の時点の検査の今の振る舞い。** `completeGeneration`（`internal/server/generation.go:71`〜`110`）は、`CheckPublishable`、応答の大きさ（`maxStatusSize(article)`、`status.go:172`）、`PrepareSlackMessages` の順に検査し、拒否された記事を `CompleteUnpostable` で `done` にする。`maxStatusSize` は記事だけを受け取り、記録が持つ動画のタイトルを見ない。`prepareSlackMessages` の本文の UTF-8 の検査（`slack_message.go:30`〜`35`）は、PR-1 の後は `CheckPublishable` が先に拒否するので通らなくなる。設計書 3.5 のとおり、PR-4 で Slack のコードと一緒に消す。
-   **時間の上限のテスト。** `make test` は `-race` で走る。リポジトリの時間の上限のテスト（`internal/transcript/copied_format_norace_test.go`・`_race_test.go`、`make test-parse-norace`）は、`-race` の構成では時間を比べない。
-   **テストの名前の確かめ。** §2 と §7 が引く既存のテストは、`grep -n '^func Test'` でファイルにあることを確かめた。新しく作るテストの名前は §2 の各ステップが決める。
-   型の構成の経路の検査（`mkplan.md` の該当の項目）は当たらない。設計は具体的な型でふるまいを分けない。

### 1.5. 名前の変更の一覧

本計画の名前の変更は、この表だけが定める。ほかの節は、この表に従う。

| 種類 | 前の名前 | 新しい名前 | ステップ |
|---|---|---|---|
| 関数 | `publisher.DisplayMarkdown` | `publisher.NeutralizedDisplayMarkdown`（戻り値は `NeutralizedDisplay`） | 2-2 |
| テスト | `TestDisplayMarkdown`（`internal/publisher/display_render_test.go`） | `TestNeutralizedDisplayMarkdown` | 2-2 |
| テスト | `TestDisplayMarkdownRejectsUnpublishable` | `TestNeutralizedDisplayMarkdownRejectsUnpublishable` | 2-2 |
| メソッド | `articlecache.Run.CompletePostable`・`CompleteUnpostable` | `articlecache.Run.Complete` | 3-6 |
| テスト | `TestGenerationUnpostable`（`internal/server/generation_test.go`） | `TestGenerationMentionsDone` | 3-3 |
| テスト | `TestEvictionKeepsRunningAndPosting`（`internal/articlecache/cache_test.go`） | `TestEvictionKeepsRunning` | 3-6 |
| 関数の引数 | `redact.SecretStrings(webhookURL secret.Secret, others ...secret.Secret)` | `redact.SecretStrings(secrets ...secret.Secret)` | 4-5 |

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 本文の UTF-8 の検査と規則の移動（設計書 8 章の 1・2）

**変更するファイル:** `internal/writer/article.go`・`article_test.go`、`internal/server/generation_test.go`、`internal/publisher/file_test.go`・`slack_message.go`・`slack_message_test.go`・`slack_render.go`、`internal/publisher/mention.go`（新規）・`mention_test.go`（新規）、`docs/tasks/0083_remove_slack_webhook/_checks/coverage/main.go`（新規）、`_checks/golden_md_unchanged.sh`（新規）

-   [x] **ステップ 1-1**: `writer.Article.CheckPublishable` に本文の UTF-8 の検査を加える（設計書 3.5）。違反は `ErrInvalidArticle` を包み、理由は本文の値を含まない固定の文字列にする。制御文字の検査より前に置く。
    -   `internal/writer/article_test.go` に `TestArticleCheckPublishableBodyUTF8` を加える。不正なバイトだけを含み制御文字を含まない本文を `ErrInvalidArticle` で拒否し、エラーの文字列が本文のバイトを含まないことを確かめる。`TestArticleCheckPublishable` にも不正な UTF-8 の本文の行を加える。
    -   `internal/server/generation_test.go::TestGenerationUnpublishable` に、目印の文字列と不正なバイトを含む本文の場合を加え、`failed` の `article_invalid` で記事を持たないことと、失敗のログの行（`generation job failed`）が目印を含まないことを確かめる。
    -   `internal/publisher/file_test.go::TestFilePublisherRejectsArticle` に、不正な UTF-8 の本文の場合を加え、ファイルを書かずに `ErrInvalidArticle` で失敗することを確かめる。
    -   `internal/publisher/slack_message_test.go::TestSlackPrepareRejectsInvalidArticle` の `body invalid utf-8` の行（`:742`）は、`CheckPublishable` が先に拒否するようになるので、前提の `checkPublishableOK` を偽に改める。その行が確かめていた Slack の側の検査の分岐は、ステップ 4-2 で消す（`_checks/coverage` が、その文が実行されなくなったことを報告する）。
    -   加えた検査を外すと `TestArticleCheckPublishableBodyUTF8`・`TestArticleCheckPublishable`・`TestFilePublisherRejectsArticle`・`TestGenerationUnpublishable` が失敗することを確かめる。
-   [x] **ステップ 1-2**: 対象の `@` の判定と U+200B を入れる処理を、振る舞いを変えずに `internal/publisher/mention.go` へ移す（設計書 3.2「ファイルの分け方」、H-04、§1.3 の決定 8）。
    -   移すもの: `zeroWidthSpace`、`replaceLookAlike`、`mentionAts`、`mentionAtsOf`、`isMentionNameChar`、`insertAfterAts`、`followsSlash`。Slack に固有の `neutralizeMentions`・`neutralizeProse`・`inRanges` は `slack_render.go` に残す。
    -   移すテスト: `TestMentionAts`（`slack_message_test.go:872`）を `mention_test.go` へ移す。
    -   移すだけの書き換えのコミットにする（関数がファイルの間を動くので coverage-compare は使えず、テストも消さないので比較は要らない）。テストの関数の数と名前が移す前と同じであることを commit のメッセージに書く。
-   [x] **ステップ 1-3**: `_checks/coverage/main.go` と `_checks/golden_md_unchanged.sh` を書く（§1.3 の決定 7、設計書 7.4）。`_checks/coverage` はステップ 3-1 で作るものだが、実装の後の網羅の比較でステップ 1-1 の変更を確かめるのに要るので、この PR で先に作る（フェーズ 3 のステップ 3-1 は、Go の側をこの PR のものとし、拡張の側を加える）。
    -   `_checks/golden_md_unchanged.sh` で AC-16a を確かめる。2 つのコミットを引数で受け取り、`internal/publisher/testdata/golden/` の `.md` のファイルの集まりと中身が両者で同じであることを確かめる。シェルで書く。各チェックが少なくとも 1 つの壊し方で失敗することを確かめ、commit のメッセージに書く。merge-base と PR の最後のコミットで実行し、出力・終了コード・2 つのコミットの ID を §7 の「実行の記録」の PR-1 の欄に書く。
    -   `_checks/coverage/main.go` は、0059・0062 の `_checks/coverage` と同じく、関数を (パッケージ, レシーバ, 名前) で対応づけて 2 つのコミットの網羅を比べる。そのうち 0062 の道具は、関数に属さない塊（トップレベルの `var` の本体など）で `0 function declarations hold the block` を出して止まる。このリポジトリの `internal/privatefile` はその形（`gofmt` が `var hook = func(...)` の本体を宣言の行に置く）を含むので、その塊を宣言の名前で対応づける形にする（壊し方で確かめる）。このフェーズの merge-base と最後のコミットで実行し、出力と終了コードを §7 の「実行の記録」の PR-1 の欄に書く。

**完了の条件:** 上の 3 つのテストが加わり通る。`mention.go` が移した名前を持ち、`slack_message.go`・`slack_render.go` に同じ名前の定義がない。§7 に AC-16a の PR-1 の記録と、`_checks/coverage` の PR-1 の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: body UTF-8 check and mention rule move

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `feat(0083): 本文の UTF-8 の検査を完了の時点に加え、メンションの判定を Webhook に依らないファイルへ移す`

**レビュー観点**: 本文の UTF-8 の違反がサーバと CLI の両方で拒否され値を出さないか / 移すコミットが振る舞いを変えていないか / ゴールデンの .md が変わっていないか

**実装モデル要件**: standard

**判定理由**: どのステップも、既存の検査への 1 つの検査の追加と振る舞いを変えない移動で、mkplan.md のパネルの契機にも Conditional checks にも当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/842
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 無効化の部品と、応答・拡張の変更（設計書 8 章の 3）

**変更するファイル:** `internal/publisher/mention.go`・`mention_test.go`・`display_render.go`・`display_render_test.go`・`slack_message.go`・`slack_render.go`、`internal/articlecache/key.go`・`record.go`・`cache.go`・`cache_test.go`、`internal/server/status.go`・`generation.go`・`status_test.go`・`generation_test.go`、`cmd/yt2column-server/relay_e2e_test.go`、`extension/src/core/response.ts`・`jobView.ts`・`i18n/catalog.ts`・`en.ts`・`ja.ts`、`extension/src/ui/renderViewer.ts`、`extension/src/viewerApp.ts`、`extension/scripts/popup-preview.ts`、拡張のテストと `test/helpers/job.ts`、`CLAUDE.md`

-   [x] **ステップ 2-1**: 対象の `@` の規則を変える（設計書 3.3 の「`@` の単位と対象の判定」のうち生の `@` の部分、§1.3 の決定 4）。
    -   `mentionAts` から、直後に名前の文字が続く条件を除き、末尾の `@` も対象にする。移した `TestMentionAts` の行のうち、直後が空白・記号・U+200B・末尾の行を新しい期待に改める。
    -   投稿の経路（`insertAfterAts` を通る `neutralizeMentions`・`neutralizeProse`、`cutStartsMention`、`findMention`）には、名前の文字の条件を保つ Slack 専用の包みを通し、投稿の振る舞いを変えない。既存の Slack のテスト（`slack_render_test.go`・`slack_message_test.go`・`golden_test.go`）を変えずに通す。包みを外すと失敗する既存のテストの名前を commit のメッセージに書く。
    -   ステップ 1-2 のコミットとも、ステップ 2-2 のコミットとも分けたコミットにする（設計書 3.2）。
-   [x] **ステップ 2-2**: 無効化の部品を作る（設計書 3.2、3.3、6.1）。
    -   `mention.go` に、`@` の単位を前から 1 回たどって探し、対象かどうかと URL のパスの `/@` の 5 つの条件を判定して、U+200B を入れる処理を加える。生の `@` の対象の判定には、ステップ 2-1 で変えた `mentionAtsOf` を使う。`@` を表しうる文字参照の判定と、U+200B を入れる処理（文字参照では `;` の直後に入れる）は新しく書き、`insertAfterAts`・`followsSlash` は使わない。入力の長さに比例する時間で終える（設計書 1.2）。
    -   `NeutralizedDisplay`・`NeutralizedDisplayMarkdown`・`NeutralizeHeading` を設計書 3.2 の形で `display_render.go` に置く（§1.5）。`DisplayMarkdown` を消すのは、唯一の呼び出し元（`internal/server/status.go:135`）を変えるステップ 2-4 と同じコミットにする。
    -   `display_render_test.go` に、設計書 7.1 の「無効化の部品」「層を分ける入力」「時間」を確かめるテストを置く。
        -   `TestNeutralizedDisplayMarkdown`: 見出しの行・チャンネル名の行・本文（コードスパンとフェンス付きコードブロックの中を含む）の表のテスト。要件書 §3.1 の例、設計書 6.3 の U1〜U6・R1・R2・S1〜S3 の形、いまの投稿が拒否する並び（`<!channel>`・`<@名前>`・`<a|b>`・`AC/@x`・コードの中の `@`・文字参照。AC-14）を行に入れ、期待値を U+200B を入れる位置で書く。すべての行で、入れた U+200B を除くと `renderDisplayMarkdown` の結果と一致することを確かめる。あわせて `Neutralized` の値も確かめる（表示用の Markdown がもともと U+200B を含み無効化しない行では、偽になることを含む）。
        -   `TestNeutralizedDisplayMarkdownURLPathConditions`: 条件 1〜5 のそれぞれについて、ほかの 4 つを満たしその 1 つだけを満たさない入力が無効化されることと、条件 2 の `<` の分かれ道の当たる入力と当たらない入力を確かめる。
        -   `TestNeutralizedDisplayMarkdownKeptAts`: 語の途中の `@` だけが当たる入力と、URL のパスの `/@` だけが当たる入力を分け、それぞれ変えないことを確かめる。各入力について、先にもう一方の規則が当たらないことを確かめる。
        -   `TestNeutralizedDisplayMarkdownLinearTime`: `/@`・`[`・`@`・`://` を多く含む同じ形の入力で、長さ n と 4n の処理の時間を数回ずつ測り、それぞれ 5 回の最小値の比が 8 倍未満であることを確かめる（線形なら約 4 倍、2 乗なら約 16 倍）。壁時計の絶対の上限は置かない（`make test` は `-race` で走るため。§1.4）。
        -   `TestNeutralizedDisplayMarkdownRejectsUnpublishable`（§1.5）: `CheckPublishable` のエラーを返す。
        -   `TestNeutralizeHeading`: URL のパスの `/@` を含むすべての対象の `@` を無効化することを確かめる。
    -   書き換えのコミットとして入れる。`TestDisplayMarkdown`・`TestDisplayMarkdownRejectsUnpublishable` を新しい名前（§1.5）と新しい期待に書き換えるのは、`DisplayMarkdown` を消すステップ 2-4 のコミットで行い、行は消さない。
    -   壊して確かめる対象: URL のパスの `/@` の判定（いつも無効化する形にすると `TestNeutralizedDisplayMarkdownKeptAts` の URL の行が失敗する）、語の途中の判定（同じテストの語の途中の行）、条件 1〜5 のそれぞれ（`TestNeutralizedDisplayMarkdownURLPathConditions` の該当の行）、文字参照の判定（`TestNeutralizedDisplayMarkdown` の文字参照の行）、URL の判定のたびに前から探し直す形（2 乗の時間になり `TestNeutralizedDisplayMarkdownLinearTime` が失敗する）。
-   [x] **ステップ 2-3**: 記事のキャッシュの記録に、生成の開始が送った動画のタイトルを持たせ、`Snapshot` で返す（設計書 3.7、§1.3 の決定 6）。`internal/articlecache/cache_test.go` に `TestSnapshotVideoTitle` を加え、タイトルがあるとき・ないとき（空文字列）に `Snapshot` が返す値を確かめる。
-   [x] **ステップ 2-4**: 状態の取得の応答を改める（設計書 3.1、3.6 の `statusArticle`）。
    -   `done` の `article` を、`markdown`（無効化した表示用の Markdown）・`mentionsNeutralized`・`videoTitle`・`channelName`（`NeutralizeHeading` の値）・`file`・`cost`・`durationMs` にする。`webhookConfigured` と `publish` は PR-3 まで残す。
    -   `maxStatusSize` が、加えたメンバと無効化で増えたバイトを含めて測るようにする。動画のタイトルは記録にあり記事にないので（§1.4）、完了の時点の検査（`completeGeneration`）が、生成の開始の値の動画のタイトルを `maxStatusSize` に渡す形にする。
    -   `internal/server/generation_test.go::TestStatusResponseWithinLimit` に、`@` の多い本文と長い動画のタイトルで、上限ちょうどの記事が `done` になり 1 バイト超える記事が `article_too_large` になる場合を加える。無効化する前の文字列で測る形に壊すと失敗することを確かめる。
    -   `internal/server/status_test.go` に `TestStatusArticleNeutralized` を加える。`article` のメンバの集まりがちょうど設計書 3.1 の形であること、`markdown` が `NeutralizedDisplayMarkdown` の値で `mentionsNeutralized` がその `Neutralized` と同じこと、`videoTitle`・`channelName` が無効化した値で、ないときは空文字列であること、`file` が `FileContent` の値と同じことを確かめる。いまの投稿が拒否する並びを含む記事も `markdown` を返すことを確かめる（AC-14）。既存の `TestStatusDoneArticle` は新しいメンバに合わせる（ステップ 2-4・2-5 は書き換えのコミットで、テストを消さない）。
    -   `cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd` と `TestStatusResponseFitsRelay` を新しい応答の形に合わせる。
-   [x] **ステップ 2-5**: 拡張の応答の検査・見出し・コピー・通知を改める（設計書 3.10 のうち投稿の部分を除くもの）。
    -   `response.ts`: `done` の記事の決まったメンバに `mentionsNeutralized`・`videoTitle`・`channelName` を加える。`Article` の型を設計書 3.10 の形にする。
    -   `renderViewer.ts` の `renderViewerHeader`: 完了した記事では応答の `videoTitle`・`channelName` を表示し、`videoTitle` が空ならビューアの URL のタイトルのまま、`channelName` が空ならチャンネル名の行を隠す。完了する前はビューアの URL の値のまま。
    -   `viewerApp.ts` の `copyArticle`: `article.markdown` をそのまま書き、`ActionNotice` の `copied` に `mentionsNeutralized` を持たせ、真なら `viewer.copiedNeutralized` を示す。
    -   カタログ: `viewer.copiedNeutralized` を英語（文面は設計書 1.2）と日本語（要件書 AC-18 の文）に加える。
    -   `scripts/popup-preview.ts` と `test/helpers/job.ts` の `done` の記事に、加えたメンバを持たせる。
    -   テスト（`response.test.ts`・`renderViewer.test.ts`・`viewerApp.test.ts`）: 加えたメンバを求め、`mentionsNeutralized` を欠く応答を想定外として拒否し記事を表示もコピーもしないこと、見出しの表示、コピーの通知の 3 つの場合（無効化した・しなかった・失敗した）、`copy` イベントを送っても通知の領域が変わらないことを確かめる。`popupHtml.test.ts` の通知の領域（`role="status"`）の確かめは、そのまま通ることを確かめる。
-   [x] **ステップ 2-6**: `CLAUDE.md`「Declare, don't infer」に、無効化の部品の例外を、設計書 3.12 の表の `CLAUDE.md` の行のとおりの範囲・理由・境界で加える（§1.3 の決定 5）。
-   [x] **ステップ 2-7**: `_checks/golden_md_unchanged.sh` を PR-2 の merge-base と最後のコミットで実行し、§7 の「実行の記録」の PR-2 の欄に書く（AC-16a）。

**完了の条件:** 上のテストが加わり通る。§7 に AC-16a の PR-2 の記録がある。`make test`・`make lint`・`make deadcode`・`make ext-check` が通る。この PR の後も、ビューアの投稿の操作は動く（投稿の削除は PR-3）。

### PR-2 作成ポイント: mention neutralization in status, viewer, and copy

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0083): ビューアの表示とコピーでメンションを無効化し、無効化したことをコピーの通知で知らせる`

**レビュー観点**: URL のパスの /@ の 5 つの条件と文字参照の判定が設計書 3.3 と 6.3 のとおりか / 層を分ける入力と壊して確かめる対象 / 応答の大きさの上限と、mentionsNeutralized を欠く応答を拡張が拒否するか / 投稿の経路が Slack 専用の包みで保たれ、CLAUDE.md の例外の範囲が無効化の部品に限られているか

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2 は、貼り付けた記事の通知を防ぐセキュリティの規則を新しく実装し（mkplan.md のパネルの契機のセキュリティの関門）、2-4・2-5 が応答の形を変える側と受ける側を同時に変える。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/850
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: ジョブの API の投稿を消す（設計書 8 章の 4 の前半）

**変更するファイル:** `internal/server/`（`generation.go`・`status.go`・`handler.go`・`response.go`・`jobs_api.go`・`server.go`・`publish.go`・`test_helpers.go` とテスト）、`internal/articlecache/`（`record.go`・`cache.go`・`cache_test.go`）、`internal/nmhost/relay.go`・`relay_test.go`、`cmd/yt2column-server/`（`run.go`・`test_helpers.go`・テスト）、`cmd/yt2column-nmhost/main_test.go`、`docs/dev/guides/development.md`・`README.md`・`README.en.md`（ガードが読む表）、`internal/doctestutil/guides.go`、拡張（`src/`・`static/`・`scripts/`・`test/`）、`.golangci.yml`、`docs/tasks/0083_remove_slack_webhook/_checks/coverage/main.go`（新規）

-   [x] **ステップ 3-1**: `_checks/coverage/main.go` を作る（§1.3 の決定 7、設計書 7.4）。
    -   Go で書く。関数を (パッケージ, レシーバ, 名前) で対応づけるには、各コミットのソースを `go/parser` で読む必要があり、シェルでは足りないためである。拡張は、lcov の報告の関数を (ファイル, 名前) で対応づけ、名前のない関数は対応づけず、その数が前後で変わったファイルを挙げる。拡張の側は、同じパッケージの `_checks/coverage/ext.go` に置き、1 つ目の引数 `ext` で選ぶ。
    -   入力は 2 つのコミットと、それぞれで測った Go の網羅の profile と拡張の lcov の報告である。親のコミットにだけある関数を「消した関数」として挙げ、残る関数について、Go は網羅率が下がらないこと、拡張は親で実行された関数の実行回数が 0 にならないことを確かめる。終了コードは、合格が `0`、不合格が `1`、比べられない（対応が 1 つに決まらない、測れていない、テストが失敗した）が `2` である。
    -   各チェックが少なくとも 1 つの壊し方で失敗することを確かめ、commit のメッセージに書く。テストのコミットとして入れる。
-   [x] **ステップ 3-2（テストのコミット）**: 投稿のジョブに依る `cmd/yt2column-server` のテストの仕組みを、生成のジョブに依る形に書き換える（§1.4）。投稿のテストは消さない。
    -   `blockingChild` と子のプロセスの止め方を、生成を止めておく偽の書き手で止める形にし、`TestSignalWaitsForJob` を「最初の合図で、止めておいた生成を取り消して待ってから終わる」、`TestSecondSignalExitsAtOnce` を「生成を止めたまま、二度目の合図ですぐ終わる」確かめに書き換える。
    -   `TestServerEndToEnd` の 2 回目の起動の確かめを、生成の開始と状態の取得にする。`TestStartDuringShutdownRefused` の停止を待たせる仕組みを、生成で待たせる形にする。`TestWriterOptionsMatchCLIDefaults` の止め方と、`TestOpenAIConfiguration` の投稿の部分を、投稿に依らない形にする。
    -   書き換えた各テストは、確かめる振る舞い（取り消し・待ち・すぐの終了・同じソケット）を壊すと失敗することを確かめる。
    -   `make coverage-compare` で確かめる。投稿の関数の網羅率が下がると報告されたら、その書き換えをステップ 3-5 のコミットに移し、移したテストを 3-5 の commit のメッセージに書く。
-   [x] **ステップ 3-3（書き換えのコミット）**: テストを消さずに済む書き換えを先に行う（設計書 8 章の 4）。
    -   `completeGeneration` は `PrepareSlackMessages` を呼ばず、記事をいつも `CompletePostable` で `done` にし、成功のログの行から `publish` の属性を除く。`TestGenerationUnpostable` を `TestGenerationMentionsDone`（§1.5）に書き換え、メンションを含む本文といまの分割の上限を超える本文の記事が `done` になり、成功のログの行の属性の集まりがちょうど `key`・`videoId`・`model`・`modelVersion`・`duration` であることを確かめる。
    -   `stopNoticeLogger`（`cmd/yt2column-server/run.go:268`〜`276`）から投稿の待ちの文を除き、二度目の合図ですぐ終わる案内を残す。`TestRunLogsListeningAndStop`・`TestOutputsOmitSecrets` の停止の案内の確かめを新しい文に合わせる。
    -   `completeGeneration` が `CompleteUnpostable` を呼ぶ形に戻すと `TestGenerationMentionsDone` が失敗することを確かめる。
-   [x] **ステップ 3-4（テストのコミット）**: 使われなくなった `approxMinutes` と `cmd/yt2column-server/run_test.go::TestApproxMinutes` を消す。関数を丸ごと消すだけなので、`make coverage-compare` で確かめる。
-   [x] **ステップ 3-5（書き換えを伴う削除のコミット）**: ジョブの API の投稿を、サーバ・ホスト・`cmd/yt2column-server`・拡張・文書の表で 1 つのコミットとして消す（設計書 3.6・3.8、3.10 の投稿の部分）。投稿の状態と理由・拒否の理由の表が、Go・拡張・`development.md`・README の間で縛り合うので、これらを 1 つにする（§1.4）。
    -   `internal/server`: `publish.go`・`publish_test.go`、`routePublish`・`PublishSuffix`、`webhook_not_configured` と `article_not_ready`・`article_not_found`・`article_not_postable`、`statusResponse.WebhookConfigured`・`statusJob.Publish` と `statusPublish`・`publishStatus`・`publishStateCode`・`unpostableCode`・`publishFailureCode`、`Options.Webhook`・`WebhookOptions`・`WebhookPublisher`・`webhookConfigured()`・`errNilWebhookPublisher`・`beforePublishJob`・`postJobs`、`test_helpers.go` の `loopbackWebhook` まわりと `RejectReasonsForTest` の投稿の表。`logEviction` の行から `publish` の属性を除く。`maxStatusSize`・`renderStatus`・`matchTarget`・`handler`・`New`・`Serve` から投稿の分岐を除く。サーバは、記事のキャッシュの投稿の API（`BeginPublish`・`Snapshot` の投稿の欄・`Eviction.Publish`）を使わなくなる。
    -   `internal/nmhost`: `OperationPublish`・`opPublish`・`publishSuffix` とその経路。
    -   `cmd/yt2column-server`: `newPublisher` の配線と `deps.newPublisher`、`listening` の行の `webhookConfigured`、Webhook がないことの警告の行。`cfg.WebhookURL()` の行は `webhookURL, _ :=` にして PR-4 まで残す。テストの補助（`test_helpers.go` の `nopPublisher`・`loopbackDeps`・`requireTestWebhook`・`errUnexpectedWebhook`・`webhookServer`・`waitPublished`・`generateAndPublish`・`blockingPublisher`）と、それを使うテスト（`relay_e2e_test.go` の `accepted_job_continues_after_host_exits`・`invalid_transcript_posts_nothing`・`new_form_operations` の投稿の部分、`TestServerEndToEnd` の前半の投稿の部分、`run_test.go:709`〜`733`・`:830` の投稿の部分）を消すか書き換える。
    -   拡張: `poll.ts` は `done` の記事でいつもポーリングを止める。`JobDisplay` から `publish`・`webhookConfigured` を除き、`response.ts` が `done` の `job` の `publish` と `ok` の応答の `webhookConfigured` を受け付けない形にする。ビューアの投稿のボタンと状態の行（`static/viewer.html`・`viewer.css`）、`publishArticle` と投稿中の状態、投稿の状態の告知、`PublishOpNotice` の `none` 以外、`classifyPublish`・`PublishOutcome`・投稿の拒否の理由、`hostMessage.ts` の `publish` と `publishMessage`、`sendMessages.ts` の `publishResultMessage`、カタログの投稿のメッセージ（設計書 3.10 の一覧。`repeatedPostStep` は、残る使い道の `viewer.lost` の手順が「もう一度投稿すると重複する」という、投稿がなくなると成り立たない文なので、その手順と一緒に消す）、`scripts/popup-preview.ts` の投稿の状態の行ごとのページ。
    -   文書の表: `development.md` の API の表の投稿の行、投稿の状態と理由の表、拒否の理由の 4 行、2 つの README の拒否の理由の表の 4 行（`README.md` を変え `/mktrans` で `README.en.md` を同期する）。`internal/doctestutil/guides.go:52`〜`53` と `extension/test/helpers/developmentDoc.ts:25`〜`26` の見出し、`cmd/yt2column-server/docs_test.go` の `PublishSuffix` の確かめ。
    -   消すテスト（それぞれ 1 つずつ確かめて消す）:
        -   [x] `internal/server/status_test.go::TestStatusWebhookConfigured`
        -   [x] `internal/server/server_test.go::TestNewWebhookOptions`（Webhook の行を消し、書き手とログの出力がないときの行は `TestNewOptions` として残す）
        -   [x] `internal/server/server_test.go::TestNewCopiesWebhookOptions`
        -   [x] `cmd/yt2column-server/run_test.go::TestBuildsSlackPublisher`
        -   [x] `cmd/yt2column-server/run_test.go::TestStartsWithoutWebhook`
        -   [x] `cmd/yt2column-server/run_test.go::TestStartupFailures` の `publisher not built` の行
    -   書き換えるテスト: `TestStatusResponseWithinLimit` の投稿の状態の繰り返しを除く。拡張の `poll.test.ts`・`poller.test.ts`・`response.test.ts`・`jobView.test.ts`・`launch.test.ts`・`jobCodes.test.ts`・`serverReasons.test.ts`・`test/helpers/job.ts`・`test/helpers/dom.ts` の応答の例と期待を新しい形に合わせ、投稿中にポーリングを続ける場合と、投稿の状態・理由・拒否の理由の行を除く。
    -   ほかのテストの中の、投稿の状態・`webhookConfigured`・投稿の拒否の理由・投稿のジョブを確かめる部分を消す（`git grep -n -i -E 'publish|webhook' -- internal/server internal/nmhost cmd/yt2column-server cmd/yt2column-nmhost extension/src extension/test extension/static extension/scripts` の当たり。拡張では `renderViewer.test.ts` の `publish rows`・`publish state announcements` の describe、`viewerApp.test.ts`・`entryPoints.test.ts`・`popupHtml.test.ts`・`hostMessage.test.ts`・`sendMessages.test.ts`・`startCoordinator.test.ts`・`popupPreview.test.ts`・`catalog.test.ts`・`metadataDocs.test.ts` の投稿の部分）。
    -   `_checks/coverage` で確かめ、出力を commit のメッセージと PR の説明に書く。残る関数で網羅率が下がったものは、テストを加える。加えない場合は、実行されていた文を消しただけであることを理由として書く。
-   [x] **ステップ 3-6（書き換えを伴う削除のコミット）**: 記事のキャッシュの投稿の状態を消す（設計書 3.7）。
    -   `PublishState`・`UnpostableReason`・`PublishFailureReason` とその列挙、`Snapshot` の投稿の欄、記録の投稿の欄と `publishGen`・`delivered`、`BeginPublish`・`PublishRun`、`Eviction.Publish`。`CompletePostable`・`CompleteUnpostable` を `Complete` にまとめ（§1.5。`completeGeneration` の呼び出しも合わせる）、`evictable` を `done` の記録をいつも落とせる形にする。
    -   消すテスト（それぞれ 1 つずつ確かめて消す）:
        -   [x] `internal/articlecache/cache_test.go::TestStalePublishRunAfterEviction`
        -   [x] `internal/articlecache/cache_test.go::TestCompleteUnpostable`
        -   [x] `internal/articlecache/cache_test.go::TestBeginPublishOutcomes`
        -   [x] `internal/articlecache/cache_test.go::TestPublishResumeFrom`
        -   [x] `internal/articlecache/cache_test.go::TestBeginPublishConcurrentStartsOnce`
    -   書き換えるテスト: `TestEvictionKeepsRunningAndPosting` は、投稿中の行を除き、実行中を残し失敗と `done` を落とす行を残して `TestEvictionKeepsRunning` にする（§1.5）。`TestEvictionOrder`・`TestRunEndsAfterFirstOutcome` と `internal/server/handler_test.go:139` の `CompletePostable`・`CompleteUnpostable`・`Eviction.Publish` の使い方を合わせる。
    -   `_checks/coverage` で確かめる。
-   [x] **ステップ 3-7（テストのコミット）**: 消した後の振る舞いを確かめるテストを加える。
    -   `internal/server/jobs_api_test.go::TestUnknownPathTouchesNothing` に `POST /jobs/{key}/publish` を加え、`not_found` で本文を読まず、枠・キャッシュ・書き手を変えないことを確かめる。`TestJobsTargetRejections` にも同じターゲットの行を加える。
    -   `internal/nmhost/relay_test.go::TestRelayNewFormRejections` に、`op` が `publish` のメッセージがサーバへ何も送らずに `protocol_error` になる行を加える。`cmd/yt2column-nmhost/main_test.go` には `publish` を受け付ける確かめがないので、変えない。
    -   `internal/server/status_test.go::TestStatusArticleNeutralized` に、`done` の応答の最上位と `job` のメンバの集まりがちょうど設計書 3.1 の形（`webhookConfigured`・`publish` がない）であることを加え、`running` の応答の最上位のメンバの集まりを確かめるテスト `TestStatusTopLevelMembers` を加える。`cmd/yt2column-server/run_test.go::TestRunLogsListeningAndStop` に、`listening` の行の属性の集まりと、警告の行がないことを加える。
    -   壊して確かめる対象: 経路の選び方に `publish` の経路を戻すと `TestUnknownPathTouchesNothing` が、ホストに `publish` の操作を戻すと `TestRelayNewFormRejections` の加えた行が、応答に `webhookConfigured` を戻すと `TestStatusTopLevelMembers` が失敗することを確かめる。
    -   `extension/test/popupHtml.test.ts` のビューアの要素の並びの確かめが、投稿のボタンと行を除いた見た目の順（要件書 4.6 節のフォーカスの順）を確かめていることを見る。
-   [x] **ステップ 3-8**: 投稿に触れるコメントを改める。`src/poller.ts`・`src/send.ts`・`static/theme.css`・`test/secretsAbsent.test.ts` のコメント、`.golangci.yml` の `server-no-pipeline`・`articlecache-boundaries` の理由の文（規則は変えない）。

**完了の条件:** 上のテストが加わり通る。ステップ 3-2・3-4 の比較と、ステップ 3-5・3-6 の `_checks/coverage` の出力が commit のメッセージと PR の説明にある。`make test`・`make lint`・`make deadcode`・`make ext-check` が通る。

### PR-3 作成ポイント: job API publish removal

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8

**推奨タイトル**: `feat(0083): ジョブの API とビューアから Webhook への投稿の操作を消す`

**レビュー観点**: ステップ 3-5 の 1 つのコミットで Go・拡張・development.md・README の表がそろっているか / _checks/coverage の比べ方と出力 / 合図と停止のテストが生成のジョブで確かめているか / 消した後のターゲットと op の拒否

**実装モデル要件**: frontier-required

**判定理由**: ステップ 3-5 が Go・拡張・文書を 1 つのコミットで同時に消し書き換える移行で、多くのテストの更新を伴う（mkplan.md のパネルの契機の移行）。3-2 は並行の振る舞い（合図・取り消し・待ち）のテストを書き換え、3-6 は記事のキャッシュの状態の遷移と追い出しを変える。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/853
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 内部に残る投稿のコードを消す（設計書 8 章の 4 の後半）

**変更するファイル:** `internal/publisher/`（Slack のコード、`mention.go`・`mention_test.go`、`golden_test.go`、`testdata/golden/*.slack.txt`、`makefile_test.go`、`test_helpers_slack.go`、`testutil/`）、`internal/writer/markdown.go`・`markdown_test.go`、`internal/slackwebhook/`、`internal/config/`、`internal/redact/`、`cmd/yt2column-server/run.go`・`test_helpers.go`・`run_test.go`、`cmd/yt2column/` のテスト・`run.go`、`cmd/yt2column-nmhost/main_test.go`、`internal/codexreview/run_test.go`、`internal/doctestutil/readme.go`、`Makefile`、`docs/dev/conventions/package_reference.md`（ガードが読む行）、`docs/tasks/0083_remove_slack_webhook/_checks/slack_removed.sh`（新規）

-   [x] **ステップ 4-1（テストのコミット）**: `replaceLookAlike` の `<`・`!` の分岐を確かめるテストを `mention_test.go` に加える。ステップ 4-2 で消すテストだけが網羅していた分岐を、4-2 の比較の前に網羅しておくためである。
-   [ ] **ステップ 4-2（書き換えを伴う削除のコミット）**: `internal/publisher` の投稿のコードを消す（設計書 3.4）。
    -   消すファイル: `slack.go`、`slack_errors.go`、`slack_integration_test.go`、`slack_test.go`、`slack_errors_test.go`、`slack_articles_test.go`、`slack_message_test.go`、`slack_render_test.go`、`test_helpers_slack.go`、`makefile_test.go`、`testutil/integration.go`、`testutil/integration_settings_test.go`、`testdata/golden/*.slack.txt`（10 ファイル）。
    -   消す部分: `slack_message.go`・`slack_render.go` の残り（ステップ 2-1 の Slack 専用の包み、`renderSlackPost`・`slackFootnote`・`stageModels`、`neutralizeMentions`・`neutralizeProse`・`inRanges`）、`mention.go` の `insertAfterAts`・`followsSlash`・`isMentionNameChar`、`golden_test.go` の `.slack.txt` の比較と `TestRenderGoldenNeutralized`、`testutil/mocks.go`・`mocks_test.go` の `PostMessages` の偽物（§1.4）。
    -   同じコミットで、`Makefile` から `test-integration-slack`・`SLACK_INTEGRATION_TIMEOUT` と `.PHONY` の項目を消し、`:322` の lint のコメントを改める（設計書 3.11。統合テストと、それを走らせるターゲットを一緒に消す）。
    -   同じコミットで、`cmd/yt2column/docs_test.go::TestSlackDocsContract`、`cmd/yt2column/makefile_test.go` の Slack の行、`cmd/yt2column/output_layout_docs_test.go` の Slack の投稿の例の確かめと `doctestutil.ReadmeSlackPostHeading` を消す。
    -   `_checks/coverage` で確かめる。
-   [ ] **ステップ 4-3（書き換えを伴う削除のコミット）**: `replaceLookAlike` を `@` の見た目の似た文字だけを扱う形に減らし、ステップ 4-1 のテストの `<`・`!` の行を消す（設計書 3.2）。`_checks/coverage` で確かめる。消した文は、消した分岐そのものである。
-   [ ] **ステップ 4-4（テストのコミット）**: `writer.ProseRanges`・`writer.ByteRange` と `internal/writer/markdown_test.go::TestProseRanges` を消す（設計書 3.4）。関数を丸ごと消すだけなので、`make coverage-compare` で確かめる。
-   [ ] **ステップ 4-5（書き換えを伴う削除のコミット）**: `redact.SecretStrings` を `SecretStrings(secrets ...secret.Secret)` にし（§1.5）、`internal/redact/redact_test.go::TestSecretStringsWebhookParts` を消す。`cmd/yt2column-server/run.go` は API キーだけを渡し、`cfg.WebhookURL()` の行を消し、HTTP/2 のデバッグの警告（`run.go:194`）から Webhook の URL を除く。`cmd/yt2column/run.go:302` の呼び出しを合わせる。`TestSecretStrings` を新しい引数に合わせる。CLI は 1 つ目の引数に DeepSeek のキーを渡していたので（§1.4）、書き換えの前後で、CLI が伏せる文字列の集まりを、英字・数字・`-`・`_` からなるキーで比べ、同じであることを commit のメッセージに書く。違えば、要件書 2.3 節（CLI の振る舞いを変えない）に関わるので、止まって人に尋ねる。`_checks/coverage` で確かめる。
-   [ ] **ステップ 4-6（書き換えを伴う削除のコミット）**: Webhook の URL の設定とパッケージを消し、`SLACK_WEBHOOK_URL` の行を知らないキーとして拒否する（設計書 3.9、AC-05・AC-06）。
    -   消すもの: `internal/slackwebhook/`（`slackwebhook.go`・`slackwebhook_test.go`）と `package_reference.md` の行（`TestPackageReferenceListsPackages` のため）、`ServerConfig.webhookURL`・`WebhookURL()`・`loadSlackWebhook`・`reasonSlackWebhook`・`slackEnv`・`SlackWebhookEnvForTest`、`serverFileKeys` の `slackEnv`、雛形の 3 行、`config_test.go::TestLoadServerWebhookOptional`・`TestServerConfigOutputRedactsWebhook`、`TestLoadServerValueRejections` の Webhook の行。
    -   書き換えるテストと fixture（消さずに、残るキーに置き換える）: `serverfile_test.go` の `TestParseServerFileAccepts` の `SLACK_WEBHOOK_URL` の行（値に `#` を含む行の解析の確かめは残る）と、`TestParseServerFileRejects` の前置きの行。`config_test.go` の環境の fixture（`:39`・`335`・`1532`）、`TestLoadServerValid` の Webhook の確かめ、`serverEnvWithFile`。`cmd/yt2column-server/test_helpers.go:44`〜`48`・`143` と `cmd/yt2column/docs_test.go:557` の設定。`git grep -n -i -E 'slack|webhook' -- internal/config cmd/yt2column-server cmd/yt2column` の残りを、この一覧と照らす。
    -   `_checks/coverage` で確かめる。
-   [ ] **ステップ 4-7（テストのコミット）**: 拒否の確かめを加える。
    -   `internal/config/serverfile_test.go::TestParseServerFileRejects` に、`SLACK_WEBHOOK_URL` の行（値があるとき・空のとき）が `ErrConfigFileFormat` と行の番号で拒否され、エラーの文字列が値を含まない行を加える。`TestParseServerFileAccepts` に、`SLACK_WEBHOOK_URL` を含む `#` の行を受理する行を加える。
    -   `TestServerFileTemplate` に、雛形の `# KEY=` の行がどれも `serverFileKeys` のキーを名指すことを加える（キーと行が 1 対 1 で対応する）。
    -   `cmd/yt2column-server/run_test.go::TestStartupFailures` に、設定ファイルに `SLACK_WEBHOOK_URL` の行があるとサーバが起動せず（終了コード `2`）、標準エラーが値を含まない行を加える。
    -   `serverFileKeys` に `slackEnv` を戻すと、加えた拒否の行と `TestStartupFailures` の加えた行が失敗することを確かめる。
-   [ ] **ステップ 4-8**: 残りの参照を改める。
    -   `internal/config/envaccess_test.go:59` の `secretEnvNames` から `SLACK_WEBHOOK_URL` を除き、`:553`・`557`・`598` の自己確かめの例を、一覧に残る名前に改める。`cmd/yt2column-nmhost/main_test.go:126` の `fullEnv` と `internal/codexreview/run_test.go:123` から `SLACK_WEBHOOK_URL` を除く。
    -   §1.4 の「Go のコメントだけで Webhook を例に挙げるもの」と `config_test.go:45`・`2243` のコメントから、Webhook と投稿を除く。
    -   `.golangci.yml` の `publisher-no-server` のコメントと `articlecache-boundaries` の `desc` から Webhook を除く。
    -   `git grep -n -i -E 'webhook|slack' -- '*.go' '*.ts' Makefile .golangci.yml` の残りが、§1.4 の「変えないもの」と、ステップ 4-7 の拒否の確かめだけであることを確かめ、その一覧を commit のメッセージに書く。
-   [ ] **ステップ 4-9**: `_checks/slack_removed.sh` を書いて実行する（AC-06・AC-09）。`Makefile` に `test-integration-slack` のターゲットがないこと、`cmd/`・`internal/`・`Makefile`・`.github/` に `YT2COLUMN_TEST_SLACK_WEBHOOK_URL` と `YT2COLUMN_SLACK_INTEGRATION` がないこと、`internal/config/serverfile.go` の雛形が `SLACK_WEBHOOK_URL` を含まないことを確かめる。シェルで書く。各チェックが少なくとも 1 つの壊し方で失敗することを確かめ、commit のメッセージに書く。出力・終了コード・比べたコミットの ID を §7 の「実行の記録」の PR-4 の欄に書く。
-   [ ] **ステップ 4-10**: `_checks/golden_md_unchanged.sh` を PR-4 の merge-base と最後のコミットで実行し、§7 の「実行の記録」の PR-4 の欄に書く（AC-16a）。

**確かめ直し。** ステップ 4-9・4-10 の比べたコミットより後に、`Makefile`・`cmd/`・`internal/`・`.github/` を変える commit が入ったら（`/fixpr` の修正を含む）、そのステップをやり直して §7 の記録を改める。

**完了の条件:** テストを消したコミットごとの比較の出力が commit のメッセージにある。§7 に AC-06・AC-09・AC-16a の PR-4 の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-4 作成ポイント: internal publish code and Webhook setting removal

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9 / 4-10

**推奨タイトル**: `feat(0083): 内部に残る Slack の投稿のコードを消し、設定ファイルの SLACK_WEBHOOK_URL を拒否する`

**レビュー観点**: 書き換えを伴う削除のコミットごとの _checks/coverage の出力 / SLACK_WEBHOOK_URL の行の拒否が値を出さないか / SecretStrings の書き換えで伏せる文字列が減っていないか / 残すキーに置き換えた設定のテストが確かめを失っていないか

**実装モデル要件**: frontier-required

**判定理由**: ステップ 4-5（秘密の伏せ字の対象の書き換え）と 4-6・4-7（秘密を持つ設定の行の起動の拒否）が、同じ PR で振る舞いを狭める側と強める側に同時に変えるセキュリティの関門の変更である（mkplan.md のパネルの契機）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 文書・エージェントの指示と手動の確かめ（設計書 8 章の 5・6）

**変更するファイル:** 設計書 3.12 の表の文書、`docs/dev/conventions/test_organization.md`、`docs/dev/guides/cli.md`・`claude_output_limit.md`・`pricing_update.md`・`docs/dev/conventions/mermaid_reference.md`（投稿を今の機能として述べる箇所があれば）、ADR、本計画

-   [ ] **ステップ 5-1**: `docs/dev/security.md` を改める（設計書 3.12 の表の行、AC-21）。
-   [ ] **ステップ 5-2**: `README.md` を改め、`/mktrans README.md` で `README.en.md` に写す（設計書 3.12 の表の行、AC-22・AC-23）。ガードのテスト（`TestReadmeSyncMarker`、`metadataDocs.test.ts`、`manifest.test.ts`、`TestReadmeHostExitCodes` ほか）が通ることを、`make test` と `make ext-test` で確かめる。
-   [ ] **ステップ 5-3**: `docs/dev/guides/development.md` の残り（ジョブの API の 2 つの操作、状態の取得の応答の形）を改める。
-   [ ] **ステップ 5-4**: `CLAUDE.md`（「Architecture Overview」「Tool Execution Safety」「Commands」「Development Notes」の `16,383` の記述と `make test-integration-slack`）、`AGENTS.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/test_organization.md`（「Golden Files」の節の `.slack.txt` と `TestRenderGoldenNeutralized`）、`docs/translation_glossary.md`（要件書 6 章）を改める。
-   [ ] **ステップ 5-5**: エージェントの指示を改める。`.claude/commands/_context.md` の「Conditional-guide trigger」と「Target client environments」（AC-24 の文）、`git grep -n -i -E 'webhook|slack' -- .claude .github` が返す指示（`runplan.md`・`mkarch.md`・`mkplan.md`・`_lib/opus-review.md`・`.github/workflows/deepseek-review.yml`）。`mkplan.md`・`runplan.md` の「Domain examples」の例のうち、投稿の機能を前提にするものも改める。
-   [ ] **ステップ 5-6**: ADR を改める（設計書 3.12 の表の ADR の行）。ADR-0005・0006・0008・0010・0018・0022・0027・0028・0031・0034・0043 の #770 へリンクした食い違いの注記を消し、ADR-0009 の背景の投稿の記述を直す。どちらも編集上の訂正として、commit のメッセージに訂正の行を書く（`docs/dev/conventions/adr.md`）。`git grep -n -i -E 'webhook|slack|投稿' -- docs/dev/adr` が返すほかの ADR（ADR-0003・0024・0037 など）は、記述が経緯としての言及か、今の機能の説明かを読んで、今の機能の説明なら直す。`cmd/yt2column/adr_test.go::TestADRRecords` が通ることを確かめる。
-   [ ] **ステップ 5-7**: `git grep -n -i -E 'webhook|slack|投稿' -- docs ':!docs/tasks' ':!docs/research' ':!docs/dev/adr'` の残りを読み、投稿の機能を今の機能として述べる箇所が残っていないことを確かめる。
-   [ ] **ステップ 5-8**: AC-17 の手動の確かめを行い、§7 の「実行の記録」の PR-5 の欄に日付・Mattermost のサーバとデスクトップアプリの版・ブラウザ（Chrome と Brave）を書く（設計書 7.3）。確かめに使う記事の内容は要件書 AC-17 のとおりで、設計書 6.3 の U1〜U6・R1・R2・S1〜S4 の形も同じ手順で貼り付ける。拡張とサーバは PR-5 のブランチから作ったものを使う。通知されたものがあれば、何も直さずに止まり、人に報告する。

**完了の条件:** 設計書 3.12 の表の各行の文書が改まり、ADR の注記がない。§7 に AC-17 の記録がある。`make test`・`make lint`・`make ext-test` が通る。

### PR-5 作成ポイント: documents, instructions, ADR notes, and manual check

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8

**推奨タイトル**: `docs(0083): 投稿の廃止に合わせて文書とエージェントの指示を改め、Mattermost での手動の確かめを記録する`

**レビュー観点**: security.md §6 の無効化の規則と残るリスク / README の設定ファイルの案内と英語版の同期 / ADR の注記の削除が編集上の訂正の範囲か / AC-17 の記録

**実装モデル要件**: standard

**判定理由**: 文書の改訂と手動の確かめで、mkplan.md のパネルの契機にも Conditional checks にも当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

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

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | 本文の UTF-8 の検査、対象の `@` の判定の移動 | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | 無効化の部品、応答の新しいメンバ、ビューアの見出し・コピー・通知、`CLAUDE.md` の例外 | frontier-required |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 | `_checks/coverage`、合図と停止のテストの書き換え、ジョブの API・ホスト・ビューア・キャッシュの投稿の削除 | frontier-required |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9 / 4-10 | `internal/publisher` の Slack のコード、`ProseRanges`、`internal/slackwebhook`、`internal/redact`、`SLACK_WEBHOOK_URL` の拒否、`make test-integration-slack` の削除 | frontier-required |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8 | 文書・エージェントの指示・ADR の注記、AC-17 の手動の確かめ | standard |

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 設計書 7.1 のとおり。無効化の部品は表のテストで、要件書 §3.1 の例と設計書 6.3 の各形を、U+200B を入れる位置で確かめる（ステップ 2-2）。層を分ける入力は、片方の規則だけが当たる形にし、もう一方が当たらないことを先に確かめる。時間は、長さの違う入力の時間の比で確かめ、`-race` で揺れる絶対の上限を置かない。サーバ・キャッシュ・ホスト・設定・拡張は、各ステップのテストで確かめる。
-   **結合テスト:** `cmd/yt2column-server/relay_e2e_test.go` で、ホストからサーバまでの状態の取得が新しい形の応答を返すことを確かめる（ステップ 2-4、3-5）。`cmd/yt2column-server` の合図と停止のテストは、生成のジョブで止める形に書き換える（ステップ 3-2）。Webhook の統合テストは消える。
-   **テストの削除:** コミットの種類ごとに、`make coverage-compare`・`make ext-coverage-compare` か `_checks/coverage` で確かめ、出力を commit のメッセージと PR の説明に書く（§1.3 の決定 7）。1 回の実行は Go で数分かかるので、10 分以上の時間の制限で走らせる。子のプロセスで動くコード（合図のテストの子のサーバ、中継のテストのホスト）は網羅の測定に現れないので、その経路は、書き換えたテストを壊して失敗することの確認が根拠になる。
-   **ゴールデンのファイル:** `<case>.md` は変えず、AC-16a の確かめに使う。`.slack.txt` は消す（§1.3 の決定 2）。
-   **一回限りの確かめ:** `_checks/golden_md_unchanged.sh`（AC-16a、PR-1・PR-2・PR-4）、`_checks/slack_removed.sh`（AC-06・AC-09、PR-4）、`_checks/coverage`（PR-3・PR-4）。
-   **手動の確かめ:** AC-17（PR-5）、AC-16 の見た目、AC-18 の文面、AC-21〜AC-24 の文書の文。
-   **後方互換:** 求めない（要件書 4.4 節）。前の版のサーバの応答を新しい拡張が想定外として拒否することは、ステップ 2-5 のテストで確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 無効化の規則の誤りで、通知される `@` が残る | 設計書 6.3 の各形を表のテストに入れる。迷う形は無効化する側に倒す（設計書 1.1）。AC-17 で Mattermost に貼り付けて確かめる |
| 規則の変更（ステップ 2-1）で、PR-2 から PR-4 までの投稿が壊れる | Slack 専用の包みを通し、既存の Slack のテストを変えずに通す（§1.3 の決定 4） |
| 無効化が遅く、状態の取得のたびに応答が遅れる | 前から 1 回たどる形にし、`TestNeutralizedDisplayMarkdownLinearTime` で確かめる |
| 無効化で増えたバイトや動画のタイトルで、応答が上限を超える | `maxStatusSize` が実際の応答の形で測り、`TestStatusResponseWithinLimit` で上限の前後を確かめる（ステップ 2-4） |
| 書き換えを伴う削除のコミットで、テストの削除による網羅の低下を見落とす | `_checks/coverage` で関数を名前で対応づけて比べ、下がった関数には理由を書く（§1.3 の決定 7） |
| ステップ 3-5 のコミットが大きい | 縛り合うのは投稿の状態・理由・拒否の理由の表だけなので（§1.4）、テストを消さない書き換え（3-3）、`approxMinutes`（3-4）、記事のキャッシュ（3-6）は別のコミットに分けた。PR の説明で、消したものと書き換えたものを分けて挙げる |
| 合図と停止のテストが、投稿のジョブを消すと意味を失う | ステップ 3-2 で、投稿を消す前に生成のジョブで止める形に書き換える |
| PR-3 と PR-4 の間、`SLACK_WEBHOOK_URL` を読むが使わない | 利用者は 1 人で、互換を求めない。PR-4 で拒否に変わる |
| PR-3・PR-4 の後、PR-5 のマージまで文書が古い | PR-5 を続けて行う。ガードが読む箇所はコードと同じコミットで改める |
| AC-17 で通知される形が見つかる | 何も直さずに止まり、人に報告する（ステップ 5-8）。規則の変更は決定の変更として扱う |

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
-   [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
-   [x] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9 / 4-10）
-   [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の意味は次のとおりである。

-   `test`: `make test`・`make ext-test` が走らせるテスト。
-   `static`: 型の検査（`make ext-typecheck`）か、`_checks/` のスクリプト。スクリプトは PR のブランチで実行し、出力・終了コード・比べたコミットの ID を下の「実行の記録」に書く。`make test` と CI は実行しない。
-   `diff`: `docs/dev/conventions/requirements_process.md` §4 の `diff` の確かめの、スクリプトの形。
-   `manual`: PR のレビューで読むか、手で操作して確かめる。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `extension/test/popupHtml.test.ts`（ビューアの要素の並び）、`extension/test/viewerApp.test.ts`（保存とコピーが動く）、`extension/test/metadataDocs.test.ts`、ビューアを開いて見る | test、manual | ステップ 3-5・3-7 | PR-3 |
| AC-02 | `internal/server/jobs_api_test.go::TestUnknownPathTouchesNothing`、`::TestJobsTargetRejections` | test | ステップ 3-5・3-7 | PR-3 |
| AC-03 | `internal/server/status_test.go::TestStatusArticleNeutralized`、`::TestStatusTopLevelMembers`、`cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd`、`extension/test/response.test.ts` | test | ステップ 2-4・2-5・3-5・3-7 | PR-2、PR-3 |
| AC-04 | `internal/nmhost/relay_test.go::TestRelayNewFormRejections` | test | ステップ 3-5・3-7 | PR-3 |
| AC-05 | `internal/config/serverfile_test.go::TestParseServerFileRejects`、`::TestParseServerFileAccepts`、`cmd/yt2column-server/run_test.go::TestStartupFailures` | test | ステップ 4-6・4-7 | PR-4 |
| AC-06 | `internal/config/serverfile_test.go::TestServerFileTemplate`（キーと行の 1 対 1）、`_checks/slack_removed.sh` | test、static | ステップ 4-6・4-7・4-9 | PR-4 |
| AC-07 | `internal/server/generation_test.go::TestGenerationMentionsDone` | test | ステップ 3-3 | PR-3 |
| AC-08 | `cmd/yt2column-server/run_test.go::TestRunLogsListeningAndStop`、`cmd/yt2column-server/signal_test.go::TestSignalWaitsForJob`・`::TestSecondSignalExitsAtOnce`（生成のジョブで止める形）、`internal/server/server_test.go::TestServeShutdownCancelsGeneration` | test | ステップ 3-2・3-3・3-5・3-7 | PR-3 |
| AC-09 | `_checks/slack_removed.sh` | static | ステップ 4-2・4-9 | PR-4 |
| AC-09a | `internal/writer/article_test.go::TestArticleCheckPublishableBodyUTF8`、`internal/server/generation_test.go::TestGenerationUnpublishable`（ログの行を含む）、`internal/publisher/file_test.go::TestFilePublisherRejectsArticle` | test | ステップ 1-1 | PR-1 |
| AC-10 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`（契約） | test | ステップ 2-2 | PR-2 |
| AC-11 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`、`::TestNeutralizedDisplayMarkdownURLPathConditions` | test | ステップ 2-2 | PR-2 |
| AC-12 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdownKeptAts` | test | ステップ 2-2 | PR-2 |
| AC-13 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`、`internal/publisher/mention_test.go::TestMentionAts` | test | ステップ 2-1・2-2 | PR-2 |
| AC-14 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`（投稿が拒否する並びの行）、`internal/server/status_test.go::TestStatusArticleNeutralized`、`internal/server/generation_test.go::TestGenerationMentionsDone` | test | ステップ 2-2・2-4・3-3 | PR-2、PR-3 |
| AC-15 | `internal/publisher/display_render_test.go::TestNeutralizedDisplayMarkdown`（無効化しない行） | test | ステップ 2-2 | PR-2 |
| AC-16 | `extension/test/renderViewer.test.ts`（`article.markdown` をそのまま描画する）、表示の見た目を読む | test、manual | ステップ 2-5 | PR-2 |
| AC-16b | `internal/publisher/display_render_test.go::TestNeutralizeHeading`、`internal/server/status_test.go::TestStatusArticleNeutralized`、`internal/articlecache/cache_test.go::TestSnapshotVideoTitle`、`extension/test/renderViewer.test.ts`（`renderViewerHeader`）、手動の確かめ（ステップ 5-8） | test、manual | ステップ 2-2〜2-5 | PR-2、PR-5 |
| AC-16a | `internal/publisher/golden_test.go::TestRenderGolden`（`<case>.md`）、`internal/publisher/file_render_test.go::TestFileContentMatchesFilePublisher`、`internal/server/status_test.go::TestStatusArticleNeutralized`（`file`）、`_checks/golden_md_unchanged.sh` | test、diff | ステップ 1-3・2-7・4-10 | PR-1、PR-2、PR-4 |
| AC-17 | 手動の確かめ（ステップ 5-8） | manual | ステップ 5-8 | PR-5 |
| AC-18 | `extension/test/viewerApp.test.ts`（無効化したときの通知）、`extension/test/catalog.test.ts`、`make ext-typecheck`（両方のカタログが `viewer.copiedNeutralized` を持つ）、日本語の文面をカタログで読む | test、static、manual | ステップ 2-5 | PR-2 |
| AC-19 | `extension/test/viewerApp.test.ts`（無効化しなかったとき・失敗したときの通知） | test | ステップ 2-5 | PR-2 |
| AC-19a | `extension/test/viewerApp.test.ts`（`copy` イベントで通知の領域が変わらない） | test | ステップ 2-5 | PR-2 |
| AC-20 | `extension/test/popupHtml.test.ts`（通知の領域が `role="status"`）、`extension/test/viewerApp.test.ts`（フォーカスが動かない）、`make ext-typecheck` | test、static | ステップ 2-5 | PR-2 |
| AC-21 | `docs/dev/security.md` を読む | manual | ステップ 5-1 | PR-5 |
| AC-22 | `TestReadmeSyncMarker`、`make test`・`make ext-test` の文書のガード、README と §2.2 の文書を読む | test、manual | ステップ 3-5・4-2・4-6・5-2〜5-7 | PR-3、PR-4、PR-5 |
| AC-23 | README の設定ファイルの説明を読む | manual | ステップ 5-2 | PR-5 |
| AC-24 | `.claude/commands/_context.md` の「Target client environments」を読む | manual | ステップ 5-5 | PR-5 |

### 実行の記録

各 PR の `_checks/` のスクリプトの出力・終了コード・比べた 2 つのコミットの ID を、ここに書く。記録した比べたコミットより後に、そのスクリプトの対象を変える commit が入ったら（`/fixpr` の修正を含む）、スクリプトを走らせ直して記録を改める。

PR-1:

**`_checks/golden_md_unchanged.sh`（AC-16a）。** 比べたコミットは、merge-base `a56f085eb59fc2d65e0fd393773067013be3727c` と、マージの前の確かめ直しで読んだ PR の head `84a96e0e` である。出力と終了コード:

```text
before: a56f085eb59fc2d65e0fd393773067013be3727c
after:  84a96e0e
files:  internal/publisher/testdata/golden/*.md
OK   the same 10 file(s) with the same content at both commits
OK   all 10 file(s) are byte for byte the same
exit=0
```

壊し方（それぞれ赤くなり、戻した）: ファイルの中身を変えると `NG   <path> changed at HEAD` と `NG   the .md files changed` で終了コード `1`。ファイルを足すと `NG   the files differ:` と前後の一覧で `1`。ファイルを消すと `NG   <path> is missing at HEAD` で `1`。空白を含む名前のファイルを足すと `1`、そのファイルを両側で変えずに置くと `0`、変えると `1`（名前を NUL とタブで切る）。知らないコミットを渡すと `<ID> is not a commit` で `2`。

**`_checks/coverage`（ステップ 1-1 の変更の確認）。** 2 つの側の profile を `go test -tags test -count=1 -coverpkg=./... -coverprofile=<profile> ./...` で測った（どちらも終了コード `0`、失敗なし）。比べたコミットは `a56f085eb59fc2d65e0fd393773067013be3727c` と `58e19c01` である。出力と終了コード:

```text
before  a56f085eb59fc2d65e0fd393773067013be3727c (1369 product functions)
after   58e19c01 (1369 product functions)
only before (removed): 0
only after (added): 0
more unexecuted (fails): 0
fewer executed (give each a reason): 1
     internal/publisher prepareSlackMessages: executed 12 -> 10 (0 of 12 unexecuted -> 0 of 10 unexecuted)
fewer unexecuted: 0
OK   no function found on both sides has more unexecuted statements
exit=0
```

`prepareSlackMessages` は、消した本文の検査の分だけ実行された文が減った。ステップ 1-1 の変更だけを比べた途中の実行（`a56f085e..d5de06b0`）では、`prepareSlackMessages: 0 of 12 unexecuted -> 1 of 12 unexecuted` で不合格（`NG   1 function(s) found on both sides have more unexecuted statements`、終了コード `1`）になり、Slack の本文の検査の文が実行されなくなったことを報告した。その検査を消した後の上の実行で、この報告が消えた。

この道具は、`go run` ではなく `go build -tags test` で作った実行ファイルで走らせる（`go run` は 1 と 2 をどちらも `1` にする）。壊し方（それぞれ赤くなり、戻した）: 関数に属さない塊（文の数が `0` の位置）を混ぜると `0 declarations hold the block` で終了コード `2`。`var` の本体の塊を両側で実行させたまま比べると、前後で対応づけられ、不合格にならない。実行されていた文を後ろの側で実行されなくすると `0 of 2 unexecuted -> 1 of 2 unexecuted` で終了コード `1`。0062 の道具を同じ 2 つの側に使うと、`internal/privatefile/privatefile.go:174.44` で `0 function declarations hold the block, want 1` を出して止まる（終了コード `2`）。

PR-2:

**`_checks/golden_md_unchanged.sh`（AC-16a）。** 比べたコミットは、merge-base `141ee751b0fa053c9f287408153e97f1d355c26a` と、PR の作成ポイントで確かめ直した head `7b670722` である。出力と終了コード:

```text
before: 141ee751b0fa053c9f287408153e97f1d355c26a
after:  7b670722
files:  internal/publisher/testdata/golden/*.md
OK   the same 10 file(s) with the same content at both commits
OK   all 10 file(s) are byte for byte the same
exit=0
```

PR-4:

（ステップ 4-9・4-10 で書く。）

PR-5:

（ステップ 5-8 で書く。）

## 8. 成功の基準 (Success Criteria)

-   要件書の AC-01〜AC-24 が、§7 の確かめで満たされている。
-   各 PR で `make test`・`make lint`・`make deadcode` が通り、拡張を変えた PR で `make ext-check` が通る。
-   テストを消したコミットごとに、比較の出力が commit のメッセージと PR の説明にある。
-   §7 に、AC-06・AC-09・AC-16a のスクリプトの実行の記録と、AC-17 の手動の確かめの記録がある。
-   セキュリティ: Webhook の URL が設定・型・伏せ字の対象・ログのどこにも残らない。サーバが外へ送るのは LLM の API だけになる（設計書 5 章）。
-   文書: 設計書 3.12 の表の文書と ADR が、今の機能と一致する。

## 9. 次のステップ (Next Steps)

-   拡張だけの構成への移行（[#771](https://github.com/isseis/yt2column/issues/771)・[#772](https://github.com/isseis/yt2column/issues/772)・[#773](https://github.com/isseis/yt2column/issues/773)）で、無効化の部品を WASM の中で使う（設計書 9 章）。
-   確かめの対象の Mattermost のサーバの版を上げるときは、設計書 6.3 の比較をやり直す。

## 変更履歴

- 2026-11-11 承認（isseis、状態表から写した）
