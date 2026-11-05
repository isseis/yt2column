# 実装計画書：CLI の Webhook への投稿（`--slack`）と `SlackWebhookPublisher.Publish` の削除

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-04 |
| Review date | 2026-11-04 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.1 節」などは設計書の節を指す。用語は要件書 6 章に従い、「サーバの投稿のジョブ」は設計書の冒頭の定義に従う。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-06 は、設計書 10 章がすべて記録している。

既存のコードについての記述は、コミット `791def11`（2026-11-04 の `main`）で確かめた。設計書が行番号を引いたコミット `df190ddc` から `791def11` までに、`cmd/yt2column` では `adr_test.go` が加わっただけで、`internal/config`・`internal/job`・`internal/publisher`・`internal/pipeline`・`Makefile`・`README.md`・`docs/dev/guides/`・`docs/dev/security.md`・`docs/dev/project_overview.md` は変わっていない（`git diff --stat df190ddc 791def11 -- <これらのパス>` の出力が空）。そのため、設計書がこれらについて引く `file:line` はそのまま使える。`CLAUDE.md` は ADR の節が加わって行がずれたので、本書は節の名前で指す。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

CLI の `--slack` と、CLI だけが使う Webhook の投稿の経路（出力先の選択、設定の読み出し、報告とヒント、`job.RemoteOutput`、`SlackMessageCount`、`make test-integration-cli-slack`）を削除し、その後で `SlackWebhookPublisher.Publish` を削除する。削除するもの、残すもの、型の形は設計書 2〜6 章で決まっている。本書は、それをどの順序で、どのコミットに分け、どのテストと確かめで検証するかを定める。

### 1.2. 実装の原則

-   PR は設計書 1.3 節・8 章のとおり 2 つにする。PR-1 はフェーズ 1（CLI の投稿の削除）、PR-2 はフェーズ 2（`Publish` の削除）である。
-   各 PR のコミットは設計書 7.4 節の順に分ける。テストの移し替えのコミット（製品のコードを変えない）を削除より先に置き、書き直すテストは新しい名前で加え、元のテストは、それが実行する関数を削除・書き換えるコミットで削除する（名前は 1.5 節）。
-   移し替えでも削除でも、使う所がなくなったテストの補助・定数・import は、そのコミットで削除する（`make lint` の `unused` はテストのファイルも調べる）。
-   削除の内容・残すもの・型の形は設計書の節を参照し、本書では繰り返さない。エラーの文言、テストの表の行、確かめのスクリプトの細部は、コードを書くときに決める。
-   加えたテストと確かめのスクリプトは、それが確かめる対象を壊して失敗することを実装のときに確かめ、コミットメッセージにそう書く（CLAUDE.md「Testing Strategy」）。否定の確かめ（触れない、問い合わせない）には、確かめる処理まで届いたことを示す肯定の確かめを組み合わせる。
-   削除したものがもうないことは `_checks/` のスクリプトで一度だけ確かめ、恒久のテストにしない（設計書 1.1 節）。今あるテストの行のうち、削除した文言が出ないことだけを確かめるもの（例: `TestRunExecutionPaths` の `notStderr` の `the webhook post timed out`）も、そのコミットで除く。
-   Go のコメント・識別子・文字列リテラルは英語で書く（CLAUDE.md「Go Idioms」）。`_checks/` のプログラムも同じである。
-   各ステップの終わりに `make fmt`・`make test`・`make lint` を通し、削除のステップでは `make deadcode` も通す。README を変えるステップでは `make ext-test` と `make readme-sync-check` を、`docs/dev/guides/development.md` を変えるステップでは `make ext-test` を通す（CLAUDE.md「Development Notes」）。

### 1.3. 計画の際に決めたこと（2026-11-04）

レビュアーに尋ねるべき未決の事項はなかった（設計書が計画に委ねた点は、どれも結果が大きく変わらない選択である）。本書の決定はどれも本タスクの中で閉じるので、ADR は作らない（CLAUDE.md「Design decisions (ADR)」の主要な決定に当たらない）。要件書が決めた CLI の投稿の削除は、既存の [ADR-0011](../../dev/adr/0011-webhook-publishing.md) の決定を変えるので、ステップ 2-5 で ADR-0011 を書き換える。CLI が投稿しないこと、`SlackWebhookPublisher` が `Publisher` を満たさなくなることは、要件書（#665）と設計書（#668）が決めたもので、本書は決めも変えもしない。設計書が計画に委ねた次の点は、理由とともにここに記録する。

-   **`--out` の検査の関数は `checkOut(fs *flag.FlagSet, opts *cliOptions) error` とする（設計書 3.1 節）:** `flagGiven(fs, "out")` で指定の有無を判断し、なければ `errOutRequired`、値が空なら `errOutEmpty` を返す。`errOutRequired` の文言は設計書 3.1 節の `"--out is required"` とする。理由: `chooseDestination` と同じ位置（手順 A2）に置け、`destination` を返す必要がなくなる。
-   **確かめのスクリプトは 2 つにする（設計書 7.3 節・7.4 節）:** `docs/tasks/0062_remove_cli_slack/_checks/residue/main.go`（以下 `_checks/residue`）と `_checks/coverage/main.go`（以下 `_checks/coverage`）である。どちらも 0059 の同じ名前のスクリプト（`docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go`・`_checks/coverage/main.go`）を写して直す。0059 の `residue` は、次のものをすでに持っている。git からだけ読む 2 つのコミットの引数、`go/ast` による宣言とリテラルの確かめ、テストでないファイルの判定（`testConstraint`）、`extension/` が変わらないことの確かめ、一時的な worktree の後始末である。`_checks/coverage` を `internal/coveragecompare` を使わずに写すのは、その道具の関数の対応づけが公開されておらず、道具の変更は本タスクの範囲の外だからである（要件書 2.3 節）。どちらも 1 行目を `//go:build test` にし、`go run -tags test` で動かす。スクリプトのステップ（1-1・2-1）を各 PR の最初に置くのは、確かめる側を、確かめられるコードより前にコミットして、コードの変更に合わせて確かめを緩められないようにするためである。確かめの実行は、設計書 8 章のとおり各 PR の最後（ステップ 1-5・2-6）に置く。`_checks/residue` は、名前を付けた確かめ（1.4 節の末尾の表）を 1 つずつ実行して結果を出し、すべて成り立てば `0`、成り立たないものがあれば `1`、実行できなければ `2` で終わる。引数は merge-base と head の 2 つのコミットで、それぞれの内容を `git` から読む（作業ツリーは読まない）。一時的な worktree と一時的なディレクトリは、終了コード `2` の場合を含め、どの終わり方でも消す。
-   **AC-01 の確かめは、head のコミットでビルドした CLI を子プロセスとして実行する（設計書 7.3 節）:** 実行ごとに新しい一時のディレクトリを作り、子の環境は親から何も引き継がず、次の変数だけで作る。
    -   `PATH=/usr/bin:/bin`（偽の `yt-dlp` が `mv` などを使うため）。
    -   `YT2COLUMN_LLM=deepseek/fake-model` と偽の `DEEPSEEK_API_KEY`。
    -   まだ存在しないパスを指す `YT2COLUMN_CACHE_DIR`。
    -   呼ばれたことを示す印のファイル（以下「印のファイル」）を作ってから `1` で終わる偽の `yt-dlp`（`transcripttestutil.NewTripwire` と同じ形の `#!/bin/sh` のスクリプト）を指す `YT2COLUMN_YTDLP_PATH`。
    -   `slackwebhook.ValidURL` を通る `https://127.0.0.1:<port>/hooks/<実行ごとの文字列>` の形の `SLACK_WEBHOOK_URL`。今の CLI はこの変数を毎回検証する（`internal/config/config.go:472-488`、`internal/slackwebhook/slackwebhook.go:20-26` は `https` だけを受理する）ので、`http` の URL では merge-base の CLI がすべての形で設定の誤りで止まり、確かめが目的と違う理由で失敗する。`<port>` は、受け付けた接続を数える TCP のリスナーである。
    -   閉じたループバックのアドレスを指す `HTTPS_PROXY`・`HTTP_PROXY`（`cmd/yt2column/main_test.go:27-38` と同じ）。

    偽の `yt-dlp` は字幕を出さずに失敗するので、merge-base の CLI でも LLM の API と Webhook に届かない。次の合格の条件は head の CLI について求めるもので、merge-base の CLI では成り立たないこと（受理された `--slack` の形が偽の `yt-dlp` を呼ぶこと）を、確かめが失敗しうる証拠として記録する（ステップ 1-1・1-5）。合格の条件は、終了コード `2`、印のファイルがない、キャッシュのディレクトリがまだ存在しない、`--out` に何もできていない、のすべてと、標準エラーの行が次のとおりであることである。URL より前の形では、標準ライブラリの未知のフラグの行（`flag provided but not defined: -slack`）と `usageHint` の行、URL より後の形では、引数の数の誤りの行（`expected exactly one video URL`）と `usageHint` の行である。リスナーが接続を受けていないことも確かめるが、偽の `yt-dlp` が公開より前に失敗するので、どちらのコミットでも失敗しえない予備の確かめであり、証拠としては記録しない。証拠は、`--slack` の各形で印のファイルがないことと、対照の実行（`--slack` を付けない `--out` だけの実行）で偽の `yt-dlp` が呼ばれて印のファイルができることである。
-   **AC-20 の語の一覧（設計書 7.3 節）:** 次の 3 つの規則で確かめる。どれも `docs/tasks/` の下の Go のファイルは探さない。
    1.  すべての Go のファイルのコメントに、`--slack`、`SlackWebhookPublisher.Publish`、`test-integration-cli-slack`、`CLI's publisher`、`CLI's webhook` がない。
    2.  `internal/publisher` の `publisher.go` と、名前が `slack` で始まるファイルと `test_helpers_slack.go` のコメントに、語としての `Publish`（前後が識別子の文字でないもの。`SlackPublishMaxDurationForTest` は当たらない）がない。
    3.  `cmd/yt2column` と `internal/job` の製品のファイルのコメントと文字列リテラルに、大文字と小文字を区別しない `webhook` がない。
    規則 3 は PR-1 で書き換えるコードだけに関わるので PR-1 の確かめ `cli-words` にし、規則 1・2 は PR-2 の確かめ `comments` にする（確かめの名前は 1.4 節の末尾の表）。`791def11` で、各規則に当たる箇所があることを grep で確かめた（例: 1 は `cmd/yt2column-server/run.go:61`、2 は `internal/publisher/slack_message.go:24`、3 は `cmd/yt2column/run.go:291`・`internal/job/job.go:54`）。
-   **網羅率の比べ方（設計書 7.4 節、AC-14）:**
    -   **テストの移し替えのコミット（ステップ 1-2・2-2）と、関数をまるごと削除するコミット（ステップ 2-3）:** 親のコミットを `BASE` にした `make coverage-compare` を実行し、`coverage-compare: PASS` で終わる出力を最後の行まで、そのコミットのメッセージと PR の説明に書く（CLAUDE.md「Deleting a test is a claim that must be checked」）。
    -   **残る関数を書き換える削除のコミット（ステップ 1-3）:** `_checks/coverage` で比べる。0059 のスクリプト（`docs/tasks/0059_remove_legacy_articles_api/_checks/coverage/main.go`）を写し、比べる値を、2 つの coverprofile から求めた関数ごとの「実行されなかった文の数」に替える。関数は (パッケージ, レシーバ, 名前) で対応づけ、各コミットのソースを `git show` で読んで、coverprofile のブロックをそれを含む関数に割り当てる。`-coverpkg=./...` の coverprofile には同じブロックがテストのバイナリごとに重なって出る。例えば `791def11` で `go test -tags test -count=1 -coverpkg=./internal/job,./internal/pipeline -coverprofile=… ./internal/job ./internal/pipeline` を実行すると、226 行のうち 113 のブロックが 2 度出た（go1.27.1）。そのため、ブロックは位置（ファイル、始まりの行と列、終わりの行と列）でまとめて文の数を 1 度だけ数え、重なって出たどれかの行の回数が 0 でなければ実行されたと数える（`internal/coveragecompare/profile.go` の `profilePosition` と同じまとめ方）。スクリプトは、両方にある関数のうち、(a) 実行されなかった文の数が増えたもの（不合格）と、(b) 実行された文の数が減ったもの（実行されていた文を削除しただけか、確かめが失われたかを人が判断する）を挙げ、削除した関数と加えた関数も挙げる。(b) を挙げるのは、同じ関数の中で、実行されなかった文の削除と、実行されていた文が実行されなくなったことが打ち消し合うと (a) には現れないからであり、AC-14 の「網羅率が下がった残る関数」を漏らさないためである。(b) の各関数には、加えたテストか、削除した文だけが減ったことの理由を、コミットメッセージに書く。手で作った入力でのスクリプトの確かめ（ステップ 1-1）は、そのコミットのメッセージに書く。どちらのコミットも、クリーンな作業ツリーから `go test -tags test -count=1 -coverpkg=./... -coverprofile=<file> ./...` で取り、テストがすべて通ること。
    -   比べた結果は、コミットメッセージと PR の説明のほか、本書 7.1 節にも書き写す（squash でマージされると、ブランチのコミットメッセージは `main` から辿れないため）。
-   **設定の確かめ直しの名前と置き場所（設計書 7.1 節）:** `Load` で確かめていた `SLACK_WEBHOOK_URL` の受理・拒否・伏せ字は、`LoadServer` では既存のテストが確かめる。URL の形の受理と拒否は `internal/slackwebhook` の `TestValidURL`、`LoadServer` が規則を当てはめて値を保つことは `TestLoadServerValid`、拒否が値を持たないことは `TestLoadServerWebhookOptional/invalid` である。ステップ 1-2 で加えた `TestLoadServerWebhookAccepted`・`TestLoadServerWebhookRejected` は、これらと重なるので、ステップ 1-6 で削除した（1.6 節）。`TestConfigOutputRedactsSecrets` の `deepseek` の行が持つ Webhook URL は、削除の後の `Config` が持たないので、`ServerConfig` の出力に Webhook URL が出ないことを確かめる `TestServerConfigOutputRedactsWebhook` を加えて確かめ直す。値が `ServerConfig` に届いたことを先に確かめ（`TestConfigOutputRedactsSecrets` の `present` と同じ）、同じ種類の出力（`fmt` の各形、`slog` のテキストと JSON、`json.Marshal`）で確かめる。理由: 同じ性質を、値を持ち続けるサーバの設定で確かめ続ける。
-   **サーバの起動の拒否の行は加えない（AC-08）:** `cmd/yt2column-server/run_test.go::TestStartupFailures` は、`LoadServer` がどの値を拒否するかは `internal/config` のテストの関心事であり、どの拒否も同じ経路を通ると述べている（`:72-75`）。不正な `SLACK_WEBHOOK_URL` の拒否は `TestLoadServerWebhookOptional/invalid` と `internal/slackwebhook` の `TestValidURL` が確かめる。
-   **`TestREADMEDocumentsFlags` に逆向きの確かめを加える（AC-17）:** 今のテスト（`cmd/yt2column/docs_test.go:115`）は、`newFlagSet` の各フラグが `cli.md` の表にあることだけを確かめる。AC-17 は表と定義が「一致」することを求めるので、表の各フラグが `newFlagSet` で定義されていることも確かめる（`-h`・`--help` の扱いは実装のときに決める）。これは削除の確かめではなく、最終的な仕様の値の規則である。`cli.md` から `--slack` の行を除くのと同じコミット（ステップ 1-4）で加える。
-   **`make test-integration-slack` の実行（AC-16）:** 本物の Webhook に投稿するので、ステップ 2-6 で利用者の承認を得てから実行する（CLAUDE.md「Tool Execution Safety」）。承認が得られなければ、実行しなかったことを 7.1 節と PR の説明に書く。

### 1.4. 既存コード調査結果

**CLI（`cmd/yt2column`、`791def11` で確かめた）**

-   削除するもの・変えるものは設計書 2.2 節の表と 3.1 節のとおりで、名前はすべて `run.go` にある（`newSlackPublisher` `:151-160`、`destination` `:162-170`、拒否のセンチネル `:172-179`、`chooseDestination` `:409-438`、`warnHTTP2Debug` `:630-641`、`buildOutput` `:643-662`、`reportSuccess` `:840-871`、`messageCount` `:925-932`、`reportRunError` `:940-977`、`reportReviewLogFailure` `:979-990`、`reportWebhookHints` `:1005-1037`、`rejectedPostStatus` `:1039-1049`、`seconds` `:1056-1059`）。`errOutEmpty`（`:176`）は残る。
-   import のうち、`net/http` は `rejectedPostStatus` だけが、`strconv` は `messageCount` だけが、`internal/secret` は `newSlackPublisher` と `buildOutput` の型だけが使う（設計書 2.1 節）。`pipeline.StagePublish` を使うのは `reportWebhookHints` だけだが、`pipeline` の import は `reportRunError` が使い続ける。
-   コメントのうち削除の後に古くなるもの: `run` の説明（`:288-292`、「the webhook is touched」）、手順 A2 のコメント（`:306-307`）、手順 A5 のコメント（`:367-369`）、`reportRunError` の説明（`:940-944`、「webhook post timeout」）。`warnHTTP2Debug`（`:630-632`）・`reportSuccess`（`:840-846`）・`reportReviewLogFailure`（`:979-981`）の説明も、Webhook の場合を述べる。どれも、その関数を書き換えるステップ 1-3 で直す。
-   テストの補助のうち削除するもの（設計書 2.2 節）: `test_helpers.go` の `childModeSlackStall`（`:49-52`）、`errSlackPublisherNotSubstituted`・`errLoopbackWebhookURL`（`:86-90`）、`loopbackPostTimeout`（`:97-99`）、`stallingPublisher`（`:160-178`）、`refusingSlackPublisher`（`:193-198`）、`loopbackSlackPublisher`（`:208-226`）、`runChildMode` の `childModeSlackStall` の分岐。`testDeps`（`:180-191`）は `newSlackPublisher` の差し替えだけを除く。`test_helpers_integration.go` の `cliSlackDeepSeekOptions`・`gateCLISlackIntegration`（`:62-97`）を削除すると、`publishertestutil` の import が使われなくなる。`net/url`・`secret` の import も `test_helpers.go` で使われなくなる。
-   `run_test.go` の補助のうち削除するもの: `webhookServer`（`:160-192`）、`useWebhook`（`:194-204`）、`failMessage`（`:207-217`）、`slackUsageRow`（`:420-433`）、`runEnv.webhook`（`:104`）、`newRunEnv` の `newSlackPublisher` の差し替え（`:156`）、`rejectedMention`（`:1498-1509`）、`threeMessageArticle`（`:229`、Webhook の行だけが使う）、`statusFailure`（`TestRunSlackFailureHints` の中、`:1526`）、`TestRunEscapesUntrustedText` の `withheld` のフィールドとその分岐（`webhook failure response body` の行だけが使う）。`TestRunExecutionPaths` の `e.webhook` の確かめ（`:1217-1219`・`:1238-1241`）も除く。`newRunEnv` の `SLACK_WEBHOOK_URL`（`:144`）と `secretStrings`（`:86-90`）は残す（設計書 1.3 節）。`signal_test.go` の `newChildSetup` の `SLACK_WEBHOOK_URL`（`:63`）も残す。
-   `TestRunExecutionPaths` の表（`executionPathRows`）で Webhook に触れる行: `neither --out nor --slack`（`:511`）、`SLACK_WEBHOOK_URL invalid`（`:523-525`）、`--slack` で始まる行と `slackUsageRow` の行（`:907-1060` の範囲）。`--out with an empty path`（`:967`）は `slackUsageRow` で書かれているが `--slack` を使わないので、`usageRow` に移して残す（AC-04）。`notStderr` の `the webhook post timed out`（`:650`・`:905`）は除く（1.2 節）。
-   設計書 7.1 節が「Webhook の場合の行と設定を除く」とした `TestRunPromptOverrides`（`:1310`）・`TestRunRejectsInvalidEffortPart`（`:2046`）・`TestRunDefaultWriterOptions`（`:1761`）は、`791def11` で Webhook・`--slack`・`newSlackPublisher` に触れていない。変える所はない（設計書の表の範囲を狭めるだけで、決定は変わらない）。
-   `TestRunReviewLogFailure`（`:1848-1891`）は `--slack` のサブテストだけを持つ。`--out` の同じ経路は `executionPathRows` の `review-log write failure after the publish` の行（`:1104-1128`）が同じ確かめ（記事を書いた行、ログの失敗の行、`do not run again`、`the run failed:` と残したファイルの行がないこと）をするので、`TestRunReviewLogFailure` はまるごと削除する。`TestRunReviewLog`（`:1786`）は `--out` のサブテストを残す。
-   `TestRunHelp`（`:1260`）は、使い方に `--slack` と `Mattermost` があること、`--slack` の説明が `cli.md` を指すことを確かめる（`:1270-1300`）。`--slack` の説明の確かめは、使い方の冒頭が `cli.md` を指す確かめ（`:1292-1294`）だけを残す。
-   `TestRunGODEBUGWarning`（`:1432`）は、`--slack` の 3 行と `webhookWarning` を持つ。
-   費用とトークンの行のテスト: `cost_test.go` の `TestRunCostLinesFailure/publish failure after every stage`（`:577-587`）、`TestRunCostLinesReviewLogFailure`（`:616-649`）、`token_test.go` の `TestRunTokenLinesAfterAllCalls/publish failure`（`:284-300` 付近）、`TestRunTokenLinesReviewLogFailure`（`:484`）が Webhook を使う。`--out` で同じ経路を作る形は、`run_test.go` の `executionPathRows` の `cache removal failure warns` の行（`publisherFunc` で `newFilePublisher` を包む、`:724-739`）と `review-log write failure after the publish` の行（`:1104-1128`）にある。`cost_test.go:443` のコメントは「(--out and Slack)」と書く（ステップ 1-3 で直す）。
-   `TestREADMEDocumentsFlags`（`docs_test.go:115`）は片方向の確かめである（1.3 節）。`TestSlackDocsContract`（`:136-165`）と `TestDeepSeekDocsContract` の `cli_slack` の行（`:214-218`）が `CLISlack*` と `DevCLIWebhookIntegrationHeading` を参照する。文書の値を求めるだけなので、文書を先に変えなくても、値を除いた後のガードは通る。
-   `makefile_test.go` で削除するもの: `TestMakeTestIntegrationCLISlack`（`:72`）、`TestCLISlackIntegrationSettings`（`:380`）、`TestCLISlackIntegrationTestBuildTag`（`:472`）。行を除くもの: `TestMakeOptInsAreTargetSpecific`（`:91`・`:102`・`:106`）、`TestMakeTargetsDoNotReadRetiredModelEnv`（`:124`）。
-   `integration_env_test.go:29` のコメント（「nothing is posted to a webhook」）は、CLI が投稿しないことを述べるだけなので変えない。

**設定（`internal/config`）**

-   削除するもの: `Config.slackWebhookURL`（`config.go:291`）、`SlackWebhookURL`（`:302-308`）、`RequireSlackWebhookURL`（`:310-318`）、`LoadFor` の `loadSlackWebhook` の呼び出しと代入（`:475`・`:485`）。`slackEnv`（`:38`）、`loadSlackWebhook`（`:717-735`）、`LoadServer`（`:381`）は残る。
-   `config_test.go` で `Load` を通して Webhook を確かめるもの: `TestLoadValid`（`:88-98`）、`TestLoadDefaults`（`:125-127`）、`TestLoadEmpty` の `slackEnv`（`:166`）、`TestLoadInvalid` の `slack webhook` の行（`:188-200`）、`TestLoadSlackWebhookAccepted`（`:590-619`）、`TestRequireSlackWebhookURL`（`:621-662`）、`TestLoadErrorsOmitValues` の `slack webhook` の 5 行（`:726-732`）と `base` の `slackEnv`（`:686`）、`TestConfigOutputRedactsSecrets` の Webhook（`:851`・`:855`・`:872`）。`LoadServer` には `TestLoadServerValid`（`:961`）と `TestLoadServerWebhookOptional`（`:1015`、未設定・空・不正な値 1 つ）がある。
-   `envaccess_test.go` は変えない。`SLACK_WEBHOOK_URL` は秘密の変数の一覧に残り（サーバが読む）、CLI の投稿だけのための許可の項目はない（設計書 2.2 節の表は挙げているが、変える所はない）。このテストは `docs/tasks/**/_checks` の Go のファイルも調べ、`test` の build 制約のないファイルを製品のコードとして扱う（0059 の 1.3 節と同じ。`_checks/` の 1 行目を `//go:build test` にする理由）。

**ジョブ（`internal/job`）**

-   削除するもの: `outputKind` とその定数（`job.go:28-36`）、`Output.kind`（`:42`）、`RemoteOutput`（`:54-57`）、`validateRequest`（`:175-202`）・`precheckOutput`（`:204-222`）の種類の分岐。`job_test.go` の `TestRunRemoteOutput`（`:208`）と、拒否の表の `remote output` の 2 行（`:928-932`）を削除する。ゼロ値・空のパス・`nil` と型付きの `nil` の `Publisher` の行（`:916-927`）は残る。
-   `internal/job/test_helpers.go` は `Output` の種類にも Webhook にも触れていない（設計書 2.2 節の表は挙げているが、変える所はない）。
-   型による保証の選び方: 本タスクは具体的な型で振る舞いを分けない。`Output` は非公開のフィールドとコンストラクタ `FileOutput` を残し、構築の経路は `FileOutput` とゼロ値の 2 つだけになる（設計書 3.3 節）。ゼロ値は `validateRequest` がパスの空で拒否し、`job_test.go` の `zero-value output` の行で確かめる。

**投稿（`internal/publisher`）**

-   `Publish`（`slack.go:166-179`）と `var _ Publisher = (*SlackWebhookPublisher)(nil)`（`:147`）を削除する。`webhookClient.configured` は `PostMessages`（`:191-`）が使い続ける。
-   `SlackMessageCount`（`slack_message.go:83-92`）をテストでないコードで呼ぶのは `cmd/yt2column/run.go:854` だけである。テストでは `slack_message_test.go::TestSlackMessageCount`（`:801`）、`slack_test.go:589`、`slack_integration_test.go:50-55`、`cmd/yt2column/run_test.go:1504`（`rejectedMention`、ステップ 1-3 で削除）が呼ぶ（`git grep -n SlackMessageCount -- '*.go'` の出力、`791def11`）。`slack_test.go:570` のコメントも `SlackMessageCount` に触れる。`PrepareSlackMessages` は `(SlackMessages, error)` を返すので、以下で「`PrepareSlackMessages` の返す値の `Len()`」と書くときは、`PrepareSlackMessages` が誤りなしで返した `SlackMessages` の `Len()` を指す。
-   `SlackMessageCount` の 3 つの確かめと重なる既存のテスト: 通数は `TestPostMessagesResume`（`slack_test.go:1287` で `m.Len()` を確かめる）、メンションの拒否は `slack_message_test.go` の `TestSlackMentionRejected` と `TestSlackPrepareRejectsInvalidArticle`。`TestPrepareSlackMessages` で始まるテストはない（設計書 7.1 節の「今の `TestPrepareSlackMessages*`」はこれらを指す）。
-   `slack_test.go` で `Publish` を呼ぶテストは 15 ある（`awk` で関数ごとに `.Publish(` を数えた、`791def11`）: `TestSlackPublishPayload`（`:467`）、`TestSlackPublishCostLine`（`:525`）、`TestSlackPublishSplit`（`:571`）、`TestSlackPublishWaitsBetweenMessages`（`:640`）、`TestSlackPublishPrepareSendsNothing`（`:667`）、`TestSlackPublishResponses`（`:705`）、`TestSlackPublishRedirect`（`:860`）、`TestSlackPublishTimeout`（`:886`）、`TestSlackPublishBodyFailures`（`:907`）、`TestSlackPublishCanceled`（`:951`）、`TestSlackPublishPartialFailure`（`:1054`）、`TestSlackPublishErrorsOmitWebhookURL`（`:1081`）、`TestSlackPublishTransportErrorWithheld`（`:1148`）、`TestSlackPublishZeroValue`（`:1167`）、`TestSlackPublishErrorClasses`（`:1180`）。補助 `postFrom`（`:1269`）と `TestPostMessages*`（`:1277`〜）がある。
-   `Publish` に触れるコメント（AC-20）: `publisher.go:1-3`、`slack.go:140-141`・`:228`、`slack_message.go:24`・`:43`・`:60`・`:73`・`:83-84`・`:94`、`slack_message_test.go:632`、`test_helpers_slack.go:73-76`、`testutil/integration.go:27-28`（`CLISlackOptInEnv` とともに削除）。
-   `testutil/integration.go` の `CLISlackOptInEnv`（`:27-29`）・`CLISlackIntegrationOptions`（`:54-57`）は削除し、「2 つの Webhook の統合テスト」と書く説明（`:45-47`）は直す。`CLISlackOptInEnv`・`CLISlackIntegrationOptions` を使うのは、`cmd/yt2column` の `docs_test.go`・`makefile_test.go`・`test_helpers_integration.go` と、`testutil/integration_settings_test.go:53` の `other_opt_in_only` の行である。

**サーバ（`internal/server`・`cmd/yt2column-server`）**

-   `internal/server/publish_test.go::TestPublishPostsLikeCLI`（`:69-94`）は、比べる相手を `hook.publisherFor(t).Publish(...)` で作る。補助 `publisherFor`（`test_helpers.go:463`）は `publish_test.go:44` も使うので残る。
-   CLI の `--slack` に触れるコメント: `cmd/yt2column-server/run.go:60-62`（`newPublisher`）、`run_test.go:373-375`（`TestBuildsSlackPublisher`）。

**`Makefile`**

-   削除するもの: `.PHONY` の `test-integration-cli-slack`（`:57`）、注釈と `CLI_SLACK_INTEGRATION_TIMEOUT`（`:297-303`）、ターゲット（`:305-312`）、`lint` の注釈の該当箇所（`:314-319`）。`test-integration` はこのターゲットに依存しない（`:214-216`）。
-   `test-integration-slack` の注釈（`:281-287`）と `SLACK_INTEGRATION_TIMEOUT` は変えない（設計書 3.5 節）。

**確かめの道具**

-   `make coverage-compare`（`Makefile:128-`）は `main` にある。0059 の `_checks/coverage/main.go` は `go tool cover -func` の出力を (パッケージ, レシーバ, 名前) で比べる。

**`_checks/residue` の確かめの名前**

| 名前 | 確かめる内容 | AC | 加えるステップ |
|---|---|---|---|
| `slack-flag` | 1.3 節のとおり、`--slack` の各形（`--slack`・`-slack` と、それぞれに `=true`・`=false` を付けた形、どれも `--out` あり・なし、URL の後の `--slack`）で CLI を実行する。対照の実行も含む | AC-01 | 1-1 |
| `cli-code` | `cmd/yt2column` の製品のファイルに、`publisher.NewSlackWebhookPublisher`・`publisher.SlackWebhookPublisher`・`SlackPostError`・`SlackHTTPStatusError`・`ErrSlackMention`・`SlackPostTimeout` の参照がない。`cliOptions` に `slack` のフィールドがなく、`errSlackFalse`・`errOutAndSlack`・`destinationWebhook`・`newSlackPublisher` の宣言がなく、`"slack"` を名前とするフラグの定義がない。`internal/job` に `RemoteOutput`・`outputKind` の宣言がない | AC-03・AC-05・AC-12 | 1-1 |
| `slack-literal` | `docs/tasks/` の外の Go のファイルに、`--slack`・`-slack` で始まる文字列リテラル、語として `--slack` を含む文字列リテラル（`writeUsage` の説明の文など、`run.go:258`）、`YT2COLUMN_CLI_SLACK_INTEGRATION` の文字列リテラルがなく、`Makefile` に `--slack` がない | AC-03・AC-11・AC-12 | 1-1 |
| `cli-words` | 1.3 節「AC-20 の語の一覧」の規則 3 | AC-05・AC-20 | 1-1 |
| `config` | `internal/config` の `Config` に `SlackWebhookURL`・`RequireSlackWebhookURL` のメソッドがなく、`ServerConfig` に `WebhookURL` がある | AC-10 | 1-1 |
| `make-target` | `Makefile` に `test-integration-cli-slack` と `CLI_SLACK_INTEGRATION_TIMEOUT` がなく、`cmd/yt2column/integration_slack_test.go` がない | AC-11 | 1-1 |
| `ignores-webhook` | `_checks/residue/testdata/` の `TestRunIgnoresSlackWebhookURL` を `cmd/yt2column` に、`TestLoadIgnoresSlackWebhookURL` を `internal/config` に、head の一時的な worktree で写して実行し、どちらも通る（1.6 節）。前者は、実行が `lookup` に `SLACK_WEBHOOK_URL` を問い合わせず（`DEEPSEEK_API_KEY` は問い合わせる）、空・`https` でない値でも `--out` の実行が未設定の場合と同じ終了コード、`--out` のファイル、標準エラーの行になること。後者は、`LoadServer` が拒否するその 2 つの値を `Load` が拒否しないこと | AC-06・AC-07 | 1-6 |
| `unchanged` | `extension/` の下のファイルと、`internal/server/` の製品の Go のファイル（名前が `_test.go` で終わらず、最初の `//go:build` の行が `test` の制約で始まらないもの。0059 のスクリプトの `testConstraint` と同じ判定）が、merge-base と head の間で変わっていない | AC-22 | 1-1 |
| `publish-method` | `internal/publisher` の `SlackWebhookPublisher` に `Publish` のメソッドがなく、`FilePublisher` にある | AC-09 | 2-1 |
| `comments` | 1.3 節「AC-20 の語の一覧」の規則 1・2 | AC-20 | 2-1 |

`slack-flag` の確かめで CLI をビルドするコミットは head である。

### 1.5. 名前の変更の一覧

本書のほかの箇所の名前は、この表に従う。「加える」は新しい名前のテストを移し替えのコミットで加え、元のテストを削除のコミットで削除することを表す（1.2 節）。

| 種類 | 今の名前 | 新しい名前 | 加える | 元を削除 |
|---|---|---|---|---|
| サブテスト | `cmd/yt2column/cost_test.go::TestRunCostLinesFailure/publish failure after every stage` | `TestRunCostLinesFailure/file publish failure after every stage` | 1-2 | 1-3 |
| テスト | `cmd/yt2column/cost_test.go::TestRunCostLinesReviewLogFailure` | `TestRunCostLinesFileReviewLogFailure` | 1-2 | 1-3 |
| サブテスト | `cmd/yt2column/token_test.go::TestRunTokenLinesAfterAllCalls/publish failure` | `TestRunTokenLinesAfterAllCalls/file publish failure` | 1-2 | 1-3 |
| テスト | `cmd/yt2column/token_test.go::TestRunTokenLinesReviewLogFailure` | `TestRunTokenLinesFileReviewLogFailure` | 1-2 | 1-3 |
| テスト | `internal/config/config_test.go::TestLoadSlackWebhookAccepted` | `TestLoadServerWebhookAccepted` | 1-2 | 1-3 |
| 行 | `config_test.go::TestLoadInvalid` の `slack webhook`、`TestLoadErrorsOmitValues` の `slack webhook` の 5 行 | `TestLoadServerWebhookRejected` | 1-2 | 1-3 |
| 確かめ | `config_test.go::TestConfigOutputRedactsSecrets` の Webhook URL | `TestServerConfigOutputRedactsWebhook` | 1-2 | 1-3 |
| テスト | `internal/publisher/slack_message_test.go::TestSlackMessageCount` | `TestPrepareSlackMessagesLen` | 1-2 | 1-3 |
| テスト | `internal/publisher/slack_test.go::TestSlackPublishPayload`・`CostLine`・`Split`・`WaitsBetweenMessages`・`Responses`・`Redirect`・`Timeout`・`BodyFailures`・`Canceled`・`ErrorsOmitWebhookURL`・`TransportErrorWithheld`・`ErrorClasses`（12） | それぞれ `TestSlackPost` に続けて同じ語（例: `TestSlackPostPayload`） | 2-2 | 2-3 |
| テスト | `slack_test.go::TestSlackPublishPrepareSendsNothing` | `TestPrepareSlackMessagesRejectionNotPostError` | 2-2 | 2-3 |
| テスト | `internal/server/publish_test.go::TestPublishPostsLikeCLI` | `TestPublishPostsPreparedMessages` | 2-2 | 2-3 |

`TestSlackPublishPartialFailure`・`TestSlackPublishZeroValue` は置き換えずに削除する（設計書 7.2 節）。

上の表の `TestLoadServerWebhookAccepted`・`TestLoadServerWebhookRejected`・`TestPrepareSlackMessagesLen` は、ステップ 1-2 で加えた後、ステップ 1-6 で削除した（1.6 節）。

### 1.6. 実装の際に決めたこと（2026-11-05）

PR-1（#682）を作った後、レビューの担当者がテストを見直し、次のことを決めた。理由はどれも、残り続けるテストに、重なる確かめと削除の確かめを持ち込まないことである。削除の確かめは `_checks/` で一度だけ行う（1.2 節、CLAUDE.md「Testing Strategy」）。

-   **CLI が `SLACK_WEBHOOK_URL` を読まないことは、`_checks/residue` で一度だけ確かめる（AC-06・AC-07）。** `TestRunIgnoresSlackWebhookURL`・`TestLoadIgnoresSlackWebhookURL` をパッケージから削除し、`_checks/residue/testdata/` に移す。`_checks/residue` の `ignores-webhook` が、head の一時的な worktree の `cmd/yt2column`・`internal/config` に写して実行する。同じ理由で、`newRunEnv` とシグナルのテストの子の環境は `SLACK_WEBHOOK_URL` を設定せず、`secretStrings` は Webhook URL を持たない。これは設計書 1.3 節の「CLI のテストは `SLACK_WEBHOOK_URL` を設定したまま走らせる」を改めるもので、設計書 1.4 節に記録した。
-   **`TestLoadServerWebhookAccepted`・`TestLoadServerWebhookRejected` は削除する。** 値は `TestValidURL` の行の写しで、`LoadServer` での確かめは `TestLoadServerValid`・`TestLoadServerWebhookOptional` と重なる（1.3 節）。`TestServerConfigOutputRedactsWebhook` は、`ServerConfig` の出力を確かめるほかのテストがないので残す。
-   **`TestPrepareSlackMessagesLen` は削除する。** 分かれる記事の通数は `TestPostMessagesResume` が確かめ、残る 1 通の記事と拒否の確かめは価値が小さい。
-   **`splitJoined` と `TestSplitJoined` は削除する。** `TestSplitJoined` は、どの実行も通らない、結合されていない誤りの分岐を確かめるためだけのテストだった。`configurationError` は、`err` だけの一覧から始めて、結合された誤りならその要素に置き換える形にし、分岐を別に持たない。結合されていない誤りを 1 行で書く振る舞いは変わらない。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: CLI の投稿の削除（設計書 8 章の 1）

#### ステップ 1-1: 確かめのスクリプト

**変更するファイル:** `docs/tasks/0062_remove_cli_slack/_checks/residue/main.go`（新規）、`docs/tasks/0062_remove_cli_slack/_checks/coverage/main.go`（新規）

- [x] `_checks/coverage` を、1.3 節「網羅率の比べ方」のとおりに作る。引数は 2 つのコミットと、それぞれの coverprofile である。
- [x] `_checks/coverage` に手で作った入力を与えて確かめる。片方にだけある関数、実行されなかった文の数が増えた関数と減った関数、ほかのファイルへ移った関数、同じパッケージで同じ名前の 2 つのメソッド（レシーバで区別する）、同じブロックが重なって出る記録（どれかが 0 でなければ実行された）、対応が 1 つに決まらない場合（`2` で止まる）である。
- [x] `_checks/residue` を、1.3 節と 1.4 節の末尾の表のとおりに作り、ステップ 1-1 の行の確かめを持たせる。識別子は `go/parser`・`go/ast` でパッケージと宣言を読んで確かめ、文字列の検索で代えない（設計書 7.3 節）。
- [x] 2 つのスクリプトの冒頭のコメントに、確かめる内容、実行の仕方、終了コードを英語で書く。`make test` と CI はこれらを実行しない。
- [x] スクリプトだけをコミットする（製品のコードもテストも変えない）。

**完了の条件:** `go vet -tags test` が 2 つのスクリプトを通る。`_checks/residue` に `791def11` を merge-base と head の両方として渡すと、`unchanged` だけが成り立ち、ほかの確かめが成り立たないと出力して `1` で終わる。`slack-flag` は、`--out` を付けない `--slack`・`--slack=true`・`-slack`・`-slack=true` の形では、印のファイルができたことを挙げる。`--slack=false`・`-slack=false` の形と、`--out` を同時に指定したすべての形では、CLI が標準ライブラリの未知のフラグの行を書かなかったことを挙げる（今の CLI は専用の拒否で止まる）。対照の実行では、印のファイルができる。

#### ステップ 1-2: テストの移し替え（製品のコードを変えない）

**変更するファイル:** `cmd/yt2column/cost_test.go`、`cmd/yt2column/token_test.go`、`cmd/yt2column/run_test.go`（補助を共有するとき）、`internal/config/config_test.go`、`internal/publisher/slack_message_test.go`

- [x] 1.5 節の費用とトークンの 4 つを加える。どのテストも `--out` で実行し、`d.newFilePublisher` を差し替える。差し替えた関数は、すべての段階が終わった後の公開で失敗する `Publisher` か、`Publish` が成功した後にログの書き込みを失敗させる `Publisher` を返す（設計書 7.1 節、1.4 節の `executionPathRows` の 2 行と同じ形）。各テストが、失敗が公開の段階で起きたこと（`publish` の段階の行、または検証・推敲のログの失敗の行）を確かめる。包む `Publisher` は 4 つのテストと `executionPathRows` の 2 行で同じ形なので、`run_test.go` に補助を 1 つ置いて共有してよい。
- [x] `TestLoadServerWebhookAccepted`・`TestLoadServerWebhookRejected`・`TestServerConfigOutputRedactsWebhook` を加える（1.3 節）。受理する値は `TestLoadSlackWebhookAccepted` の 5 つ、拒否する値は `TestLoadInvalid` の `slack webhook` の行の値、値を出さないことは `TestLoadErrorsOmitValues` の 5 行の値と確かめる部分をそのまま移す。
- [x] `TestPrepareSlackMessagesLen` を加える。`TestSlackMessageCount` が確かめる 3 つ（1 通の記事、分かれる記事、メンションの拒否）のうち、1.4 節の重なる既存のテストが確かめていないものだけを、`PrepareSlackMessages` の返す値の `Len()` と返るエラーで確かめる。重なるものは、どのテストが確かめるかをコミットメッセージに書く。
- [x] 加えたテストが、それぞれ確かめる対象を壊すと失敗することを確かめ、コミットメッセージに書く。
- [x] 親コミットを `BASE` に `make coverage-compare` を実行し、`coverage-compare: PASS` の出力を最後の行までコミットメッセージに書く（1.3 節）。

**完了の条件:** `make test` と `make lint` が通り、`make coverage-compare` が `PASS` で終わる。

#### ステップ 1-3: CLI・設定・ジョブの Webhook の経路の削除

**変更するファイル:** `cmd/yt2column/` の `run.go`・`test_helpers.go`・`test_helpers_integration.go`・`integration_slack_test.go`（削除）・`run_test.go`・`signal_test.go`・`cost_test.go`・`token_test.go`・`makefile_test.go`・`docs_test.go`、`internal/config/config.go`・`config_test.go`、`internal/job/job.go`・`job_test.go`、`internal/publisher/slack_message.go`・`slack_message_test.go`・`slack_test.go`・`slack_integration_test.go`・`testutil/integration.go`・`testutil/integration_settings_test.go`、`internal/doctestutil/guides.go`、`Makefile`

- [x] `cmd/yt2column/run.go` を設計書 3.1 節のとおりにする。`checkOut` と `errOutRequired` を加え（1.3 節）、1.4 節の削除するもの、使われなくなる import を削除し、`checkReviewLog`・`checkCacheDirs` から `destination` を除いて `--out` と常に比べる。`writeUsage` の説明を、記事を `--out` のファイルに書くこと、`--out` が必須であることに改める（AC-03）。1.4 節の古くなるコメントを直す。
- [x] `internal/config/config.go` を設計書 3.2 節のとおりにする（`loadSlackWebhook` の説明の「Load and LoadServer」を `LoadServer` に直す）。
- [x] `internal/job/job.go` を設計書 3.3 節のとおりにする。`Output` の説明を設計書 3.3 節のコードのとおりにする。
- [x] `internal/publisher/slack_message.go` の `SlackMessageCount` を削除する。`slack_test.go:589` と `slack_integration_test.go:50-55` の呼び出しを、`PrepareSlackMessages` の返す値の `Len()` に直し、`slack_test.go:570` のコメントも直す。
- [x] `CLISlackOptInEnv`・`CLISlackIntegrationOptions`（`internal/publisher/testutil/integration.go`）、`DevCLIWebhookIntegrationHeading`（`internal/doctestutil/guides.go:29-31`）、`cmd/yt2column/integration_slack_test.go` を削除し、`integration.go:45-47` の説明を 1 つの統合テストのものに直す。`integration_settings_test.go` の `other_opt_in_only` の行を、テストのためだけの別の名前で確かめる形にする（設計書 7.1 節）。
- [x] `Makefile` から 1.4 節の削除するものを削除する。
- [x] テストを削除する（設計書 7.1 節）。`run_test.go` の `TestRunSlackFailureHints`・`TestRunSlackSignalAfterPosting`・`TestRunSlackSummaryUnknownCount`・`TestNewSlackPublisherNilOnFailure`・`TestTestDepsSlackPublisherDoesNotSend`。
- [x] `run_test.go` の `TestRunReviewLogFailure` を削除する（1.4 節）。
- [x] `signal_test.go` の `TestSignalDuringSlackPost` を削除する。
- [x] `makefile_test.go` の `TestMakeTestIntegrationCLISlack`・`TestCLISlackIntegrationSettings`・`TestCLISlackIntegrationTestBuildTag` を削除する。
- [x] `internal/job/job_test.go` の `TestRunRemoteOutput` を削除する。
- [x] `internal/config/config_test.go` の `TestRequireSlackWebhookURL` と `TestLoadSlackWebhookAccepted` を削除する。
- [x] `internal/publisher/slack_message_test.go` の `TestSlackMessageCount` を削除する。
- [x] `cost_test.go`・`token_test.go` の 1.5 節の元のテストとサブテストを削除する。
- [x] 1.4 節の補助と表の行を、次のとおり削除・移動・修正する。
    -   `executionPathRows` の Webhook の行と、`notStderr` の古い文言を削除する。`--out with an empty path` の行は `usageRow` に移す。
    -   `review-log write failure after the publish` の行の `wantStderr` にログのパスを加える（削除する `TestRunReviewLogFailure` がパスを含めて確かめていたため）。
    -   `TestRunEscapesUntrustedText` の `--slack` の 2 行と `withheld`、`TestRunReviewLog` の `--slack` のサブテスト、`TestRunGODEBUGWarning` の `--slack` の行を削除する。`cost_test.go:443` のコメントを直す。
    -   `job_test.go` の `remote output` の 2 行、`config_test.go` の 1.4 節の `Load` の Webhook の確かめ、`makefile_test.go`・`docs_test.go` の `CLISlack*` と `cli_slack` の行を削除する。
- [x] 新しい振る舞いのテストを加える（設計書 7.4 節の規則により、このコミットに入れる）。
    -   `run_test.go::TestCheckOut`: 指定がなければ `errOutRequired`（`errOutEmpty` ではない）、値が空なら `errOutEmpty`（`errOutRequired` ではない）、空でない値なら `nil` を返すこと（AC-02）。`executionPathRows` に `no --out` の行（終了コード `2`、`usageHint`、`untouched`）を加える。
    -   `run_test.go::TestRunIgnoresSlackWebhookURL`: 問い合わせた名前を記録する `lookup` で、`SLACK_WEBHOOK_URL` が問い合わせられないこと（肯定の確かめとして、ほかの変数が問い合わせられたこと）を確かめる（AC-06）。`SLACK_WEBHOOK_URL` が空・`https` でない値のとき、`--out` の実行が未設定の場合と同じ終了コード、`--out` のファイル、標準エラーの行になることを確かめる（AC-07）。比べる 2 つの実行は、同じパスの配置と固定の時計（`useFixedClock`）で走らせ、実行ごとに違う値が比べる行に入らないようにする。（ステップ 1-6 で `_checks/residue/testdata/` に移した。1.6 節）
    -   `config_test.go::TestLoadIgnoresSlackWebhookURL`: `Load` が空・不正な `SLACK_WEBHOOK_URL` を拒否しないこと。（ステップ 1-6 で `_checks/residue/testdata/` に移した。1.6 節）
    -   `TestRunGODEBUGWarning`: `http2debug` が有効なとき、警告が API キーの 1 行だけであること（AC-05）。
    -   `TestRunHelp`: `--slack` と `Mattermost` を求める確かめと `--slack` の説明の確かめを除き、`--out` の説明か冒頭が `--out` を必須と示すことを確かめる（AC-03）。
- [x] AC-04 の行（1.4 節、7 章の AC-04 の行）は、期待する値（`wantCode`・`wantStderr`・`notStderr`）を、上で挙げた変更のほかは変えない。
- [x] 加えたテストと書き換えた確かめが、それぞれ対象を壊すと失敗することを確かめ、コミットメッセージに書く。
- [x] ステップ 1-2 のコミットとこのコミットで `_checks/coverage` を実行し、削除した関数、加えた関数、1.3 節の (a)・(b) に当たる関数とその扱い（加えたテスト、または理由）をこのコミットのメッセージに書く（AC-14）。子プロセスで動くコード（`signal_test.go` の子の CLI）は coverprofile に現れないので、`TestSignalDuringSlackPost` の削除はこの比較に現れないことも書く。比べるのはコミットを作った後なので、`git commit --amend` でメッセージだけを直し、内容が変わらないことを `git diff HEAD@{1} HEAD` で確かめる（push の前、次のコミットより前に行う）。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。

#### ステップ 1-4: PR-1 の文書

**変更するファイル:** `docs/dev/guides/cli.md`、`README.md`、`README.en.md`、`docs/translation_glossary.md`、`docs/dev/guides/development.md`、`docs/dev/security.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`CLAUDE.md`、`cmd/yt2column/docs_test.go`

- [x] `cli.md` を AC-17 のとおりに直す（冒頭、「必要なもの」、「使い方」とフラグの表、「Webhook への投稿」の節の削除、「出力の書式」、「推定の料金」、終了コード `0`、「設定」）。
- [x] `TestREADMEDocumentsFlags` に逆向きの確かめを加える（1.3 節）。`cli.md` に `--slack` の行を戻すと失敗することを確かめ、コミットメッセージに書く。
- [x] `README.md` を AC-18 のとおりに直し（`:10`、`:191-192`、「設定」の表とその後の説明 `:398-404`）、`/mktrans README.md` で `README.en.md` に写す。`README.md:192` と `README.en.md:273` は `cli.md` の「Webhook への投稿」の見出しへリンクし、`TestReadmeLinks`（`cmd/yt2column/docs_test.go:650`）がリンク先の見出しがあることを確かめる。2 つの README からこのリンクを除き（AC-18 の書き直しの一部）、`cli.md` の節の削除と同じコミットに入れる。`translation_glossary.md:167` を残る見出しに合わせる。
- [x] `development.md` の `make test-integration-cli-slack` の記述（`:17`、「CLI と Webhook の統合テスト」の節 `:273-276`）を除く。
- [x] `security.md` §2 の表の CLI の行（`:40`）とテストの数、`GODEBUG` の項（`:42-43`）を直す。
- [x] `package_reference.md` の次の行を直す（設計書 3.6 節）。`cmd/yt2column`・`internal/job` の行、`internal/config` の行の `RequireSlackWebhookURL` と「the Slack Webhook URL」の記述、`internal/publisher` の行の `SlackMessageCount` の記述（`:48`）、`internal/slackwebhook` の行の「including `cmd/yt2column`'s」（`:31`。ステップ 1-3 で `integration_slack_test.go` を削除した後は事実と合わなくなる）。
- [x] `project_overview.md` の冒頭（`:3`）と `SLACK_WEBHOOK_URL` の行（`:140`）、`production_profile.md:40` を直す（設計書 3.6 節）。
- [x] `CLAUDE.md` の「Tool Execution Safety」の CLI を本物のサービスに対して実行する項を直す。本物の Webhook への投稿（サーバの投稿のジョブと `make test-integration-slack`）に承認が要ることは保つ（設計書 3.6 節）。
- [x] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview/... ./internal/pipeline`）、`make readme-sync-check`、`make ext-test`、`go test -tags test ./cmd/yt2column-nmhost/`（`TestReadmeHostExitCodes`）を通す。

**完了の条件:** 上のテストと `make test` が通る。

#### ステップ 1-5: PR-1 の確かめの実行と記録

**変更するファイル:** 本書 7.1 節

- [x] PR-1 のすべての変更をコミットした後、`_checks/residue` を merge-base（`git merge-base origin/main HEAD`）と `HEAD` で実行し、出力、終了コード、2 つのコミット ID を 7.1 節に書く。merge-base を head として渡したとき、`unchanged` を除く確かめが成り立たないことも書く（`slack-flag` は、どの形で印のファイルができたかを書く）。
- [x] `unchanged` は merge-base でも成り立つので、`extension/` の 1 行と `internal/server/` の製品のファイルの 1 行をそれぞれ変えた一時的なコミット（ブランチに載せない）を head として渡し、成り立たないと出ることを確かめ、7.1 節に書く。
- [x] ステップ 1-2・1-3 の網羅率の比較の結果を 7.1 節に書き写す。
- [x] `make test`・`make lint`・`make deadcode` を通す。

**完了の条件:** `_checks/residue` が PR の head で `0` を返し、その記録が 7.1 節にある。

#### ステップ 1-6: レビューでの決定によるテストの整理（1.6 節）

**変更するファイル:** `docs/tasks/0062_remove_cli_slack/_checks/residue/main.go`・`testdata/`（新規）、`cmd/yt2column/run.go`・`run_test.go`・`signal_test.go`、`internal/config/config_test.go`、`internal/publisher/slack_message_test.go`、本書、設計書

- [x] `_checks/residue` に `ignores-webhook` を加え、`TestRunIgnoresSlackWebhookURL`・`TestLoadIgnoresSlackWebhookURL` を `testdata/` に写す。merge-base で、2 つのテストが述べた理由で失敗することを確かめる。
- [x] `configurationError` を、`splitJoined` を呼ばない形にする（製品のコードだけのコミット）。
- [x] `TestLoadServerWebhookAccepted`・`TestLoadServerWebhookRejected`・`TestLoadIgnoresSlackWebhookURL`・`TestPrepareSlackMessagesLen`・`TestRunIgnoresSlackWebhookURL` と、`newRunEnv`・シグナルのテストの子の環境の `SLACK_WEBHOOK_URL`、`secretStrings` の Webhook URL を削除する（テストだけのコミット）。親のコミットを `BASE` に `make coverage-compare` を実行し、`PASS` の出力をコミットメッセージに書く。
- [x] `splitJoined` と `TestSplitJoined` を削除する。親のコミットを `BASE` に `make coverage-compare` を実行し、`PASS` の出力をコミットメッセージに書く。
- [x] `_checks/residue` を merge-base と head で実行し、7.1 節に書く。2 つの網羅率の比較の結果も 7.1 節に書き写す。
- [x] `make test`・`make lint`・`make deadcode` を通す。

**完了の条件:** `_checks/residue` が PR の head で `0` を返し、2 つの `make coverage-compare` が `PASS` で終わり、その記録が 7.1 節にある。

### PR-1 作成ポイント: CLI webhook post removal

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `refactor(0062): CLI の Webhook への投稿（--slack）を削除する`

**レビュー観点**: CLI が `SLACK_WEBHOOK_URL` を問い合わせず、不正な値でも `--out` の実行が変わらないこと（AC-06・AC-07）と、伏せ字の対象から Webhook URL が外れても出力に書く経路がないこと / `--out` の必須化と、削除で振る舞いが変わらない拒否と報告の行（AC-02・AC-04） / テストの移し替えと削除のコミットの網羅率の比較（`make coverage-compare` と `_checks/coverage`）の記録 / `_checks/residue` の `slack-flag` の確かめが、merge-base で失敗し head で通る記録

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-3 が CLI の秘密情報の扱い（伏せ字の対象と環境変数の読み出し）を変え、多数のテストの削除と書き換えを伴う。mkplan のパネルモードの条件（セキュリティの境界と多数のテストの更新）に当たる

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/682
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `SlackWebhookPublisher.Publish` の削除（設計書 8 章の 2）

#### ステップ 2-1: 確かめのスクリプトへの追加

**変更するファイル:** `docs/tasks/0062_remove_cli_slack/_checks/residue/main.go`

- [x] `_checks/residue` に、1.4 節の末尾の表のステップ 2-1 の行（`publish-method`・`comments`）を加える。
- [x] スクリプトだけをコミットする。

**完了の条件:** `go vet -tags test` がスクリプトを通り、PR-2 の merge-base を merge-base と head の両方として渡すと、`publish-method` と `comments` が成り立たないと出て `1` で終わる。

#### ステップ 2-2: テストの移し替え（製品のコードを変えない）

**変更するファイル:** `internal/publisher/slack_test.go`、`internal/server/publish_test.go`

- [x] 1.5 節の `TestSlackPost*` の 12 のテストを加える（設計書 7.2 節）。各テストは元のテストの publisher の作り方、オプション（`Timeout`・`Interval`）、context、transport、送り先をそのまま使い、`p.Publish(ctx, a)` を、`PrepareSlackMessages(a)` の誤りを確かめた後に `p.PostMessages(ctx, m, 0, …)` で送る形に替える。既存の補助 `postFrom`（`slack_test.go:1269`）は、元のテストが既定のオプションと `context.Background()` で送る所にだけ使う。新しい補助は加えない。`TestSlackPublishErrorClasses` の準備の誤りの分類（分けられない、メンション、正しくない記事）は `PrepareSlackMessages` の誤りで確かめる。元のテストの確かめ（送る形、費用の行、分割、`silent`、順序と間隔、応答、リダイレクト、時間切れ、本文の失敗、取り消し、URL を出さないこと、通信の誤りを伏せること、エラーの分類）は変えずに移す。
- [x] `TestPrepareSlackMessagesRejectionNotPostError` を加える。`TestSlackPublishPrepareSendsNothing` の 7 つの行（`slack_test.go:673-685`）をすべて持ち、各行で期待するエラーであることと、`*SlackPostError` でないことを確かめる（設計書 7.2 節）。
- [x] `TestPostMessagesRejectsWithoutSending` の `zero value publisher` の行（`slack_test.go:1446-1452`）に、エラーが `*SlackPostError` でないことの確かめを加える（削除する `TestSlackPublishZeroValue` が確かめていたため）。
- [x] `TestPublishPostsPreparedMessages` を加える。比べる相手を `publisher.SlackMessagesForTest(article)` にし、準備した 3 通のメッセージがそのまま送られて成功すること、成功のログの行の `posted=3`・`total=3` を確かめる（設計書 7.2 節）。
- [x] 加えたテストが、それぞれ確かめる対象を壊すと失敗することを確かめ、コミットメッセージに書く。
- [x] 親コミットを `BASE` に `make coverage-compare` を実行し、`PASS` の出力をコミットメッセージに書く。

**完了の条件:** `make test` と `make lint` が通り、`make coverage-compare` が `PASS` で終わる。

#### ステップ 2-3: `Publish` の削除

**変更するファイル:** `internal/publisher/slack.go`、`internal/publisher/slack_test.go`、`internal/publisher/slack_integration_test.go`、`internal/server/publish_test.go`

- [x] `slack.go` から `Publish` と `var _ Publisher = (*SlackWebhookPublisher)(nil)` を削除する。残る関数の中は書き換えず、コメントはステップ 2-4 で直す（設計書 7.4 節の 2）。
- [x] 1.5 節の `slack_test.go` の元の 13 のテストを削除する。
- [x] `TestSlackPublishPartialFailure` を削除する（設計書 7.2 節の理由）。
- [x] `TestSlackPublishZeroValue` を削除する（設計書 7.2 節の理由）。
- [x] `internal/server/publish_test.go` の `TestPublishPostsLikeCLI` を削除する。
- [x] `slack_integration_test.go::TestIntegrationSlackWebhookPublisher` を設計書 3.5 節のとおりに書き直す（すべての記事の通数を送る前に確かめ、検査に通った `SlackMessages` の値そのものを `PostMessages(ctx, m, 0, …)` で送り、`delivered` が `m.Len()` 回呼ばれたことを確かめる）。`make lint` の `go vet -tags integration` で型の誤りがないことを確かめる。
- [x] 親コミットを `BASE` に `make coverage-compare` を実行し、`PASS` の出力（削除した関数の一覧を含む）をコミットメッセージに書く。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通り、`make coverage-compare` が `PASS` で終わる。

#### ステップ 2-4: コメントの書き直し

**変更するファイル:** `internal/publisher/publisher.go`・`slack.go`・`slack_message.go`・`slack_message_test.go`・`test_helpers_slack.go`、`cmd/yt2column-server/run.go`・`run_test.go`、`internal/slackwebhook/slackwebhook.go`

- [x] 設計書 3.4 節が挙げる説明（パッケージの説明、型の説明、`webhookClient.post` の説明、`slack_message.go` の「`Publish` が送る」）と、1.4 節の `slack_message_test.go:632`、`test_helpers_slack.go:73-76` の `SlackMessagesForTest` の説明（設計書 3.5 節の「production publisher」を含む）を、`PrepareSlackMessages` と `PostMessages` の説明に直す。
- [x] `cmd/yt2column-server/run.go:60-62` と `run_test.go:373-375` から CLI の `--slack` を除く。
- [x] `internal/slackwebhook/slackwebhook.go` のパッケージの説明の共有先から `cmd/yt2column` を除き、今の import 元（`internal/publisher/testutil`・`internal/redact`）に合わせる。
- [x] `_checks/residue` の `comments` が HEAD で成り立つことを確かめる。

**完了の条件:** `make test` と `make lint` が通る。

#### ステップ 2-5: PR-2 の文書

**変更するファイル:** `CLAUDE.md`、`docs/dev/project_overview.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/guides/development.md`、`docs/dev/security.md`、`docs/dev/adr/0011-webhook-publishing.md`

- [x] `CLAUDE.md` の「Architecture Overview」の `Publisher` の項を、`SlackWebhookPublisher` が `Publisher` を満たさず、サーバの投稿のジョブが `PrepareSlackMessages` と `PostMessages` で送ることに合わせる（AC-19）。
- [x] `project_overview.md` の `Publisher` の初期実装の項（`:75-76`）、`package_reference.md` の `internal/publisher` の行、`make test-integration-slack` の説明（`development.md`、`security.md` §2 の表の `:39`）を、設計書 3.6 節の PR-2 のとおりに直す。
- [x] ADR-0011 の「決定」「結果」から CLI の `--slack` を除き、0062 で採らなかった案（CLI の投稿を残す、CLI が `SLACK_WEBHOOK_URL` を拒否する）と経緯を加える。
- [x] 文書のガードと `make ext-test` を通す（`development.md` を変えるため）。

**完了の条件:** 文書のガード、`make ext-test`、`make test` が通る。

#### ステップ 2-6: PR-2 の確かめの実行と記録

**変更するファイル:** 本書 7.1 節

- [x] 利用者の承認を得て `make test-integration-slack` を実行し、結果（2 つの記事の通数、成功したこと）を 7.1 節に書く（1.3 節、AC-16）。承認が得られなければ、実行しなかったことを書く。
- [x] PR-2 のすべての変更をコミットした後、`_checks/residue` を PR-2 の merge-base と `HEAD` で実行し、出力、終了コード、2 つのコミット ID を 7.1 節に書く。
- [x] PR-2 の merge-base と PR-1 の merge-base（7.1 節の PR-1 の記録にあるもの）をそれぞれ head として渡し、どちらでも `comments` と `publish-method` が成り立たないことを確かめ、7.1 節に書く（設計書 7.3 節）。
- [x] ステップ 2-2・2-3 の網羅率の比較の結果を 7.1 節に書き写す。
- [x] `make test`・`make lint`・`make deadcode` を通す。

**完了の条件:** `_checks/residue` が PR の head で `0` を返し、その記録が 7.1 節にある。

### PR-2 作成ポイント: SlackWebhookPublisher.Publish removal

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `refactor(0062): SlackWebhookPublisher.Publish を削除する`

**レビュー観点**: `Publish` を通して確かめていた振る舞いが、`PostMessages` で送る形に書き直したテストにすべて移っていること（1.5 節の対応と `make coverage-compare` の記録） / 統合テストが送る前にすべての記事の通数を確かめ、`PostMessages` で送ること（AC-16） / `_checks/residue` の `comments` の語の一覧が、PR-1 の前と PR-2 の前のどちらでも失敗し、head で通ること / CLAUDE.md の `Publisher` の項

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-2 が 14 のテスト（`slack_test.go` の 13 と `internal/server` の 1）を、元のオプション・context・transport を保ったまま移し替え、網羅率を下げずに保つ必要がある（確かめの取りこぼしが起きやすい、複雑なステップ）。本物の Webhook の統合テストの書き直しは 1 ファイルにとどまり、パネルモードの条件のうち、外部の資源を重く使う面には当たらない

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/694
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: CLI の投稿の削除 | 確かめのスクリプト、テストの移し替え、CLI・設定・ジョブ・`SlackMessageCount`・`make test-integration-cli-slack` の削除、PR-1 の文書、確かめの記録 | PR-1 がマージされた |
| M2: `Publish` の削除 | 確かめの追加、`slack_test.go` の移し替え、`Publish` の削除、統合テストの書き直し、コメント、PR-2 の文書、確かめの記録 | PR-2 がマージされた |

M2 は M1 の後に置く（設計書 8 章）。PR-1 の後は `Publish` を呼ぶのがテストだけになり、PR-2 の製品のコードの変更は `internal/publisher` の中で閉じる。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | 確かめのスクリプト、テストの移し替え、`--slack`・CLI の `SLACK_WEBHOOK_URL` の読み出し・`job.RemoteOutput`・`SlackMessageCount`・`CLISlack*`・`make test-integration-cli-slack` の削除、`--out` の必須化、PR-1 の文書と README | frontier-required |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | 確かめの追加、`Publish` のテストの移し替え、`Publish` の削除、統合テストの書き直し、コメント、PR-2 の文書 | frontier-recommended |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどのテストで確かめるかは本書 7 章に示す。

-   **単体テスト:** 削除するテスト、確かめ直すテスト、値を直すテストの 3 つに分ける（設計書 7 章の前置き）。確かめ直すテストは新しい名前で先に加え（ステップ 1-2・2-2）、元のテストは関数とともに削除する（ステップ 1-3・2-3）。新しい振る舞い（`--out` の必須、`SLACK_WEBHOOK_URL` を読まないこと）のテストは、製品のコードを変えるステップ 1-3 で加える。
-   **統合テスト:** `make test-integration-slack` の統合テストを `PrepareSlackMessages` → `PostMessages` の形にし（ステップ 2-3）、利用者の承認の後に実行する（ステップ 2-6）。同意の変数と URL の扱いは `TestSlackSettingsFrom` が確かめ、変えない。
-   **後方互換:** `--out` の実行の振る舞い（AC-04）とサーバの `SLACK_WEBHOOK_URL` の扱い（AC-08）は、既存のテストが通ることで確かめる。移し替えで書き換えるテストでも、既存の行の期待する値は変えない。
-   **セキュリティ:** CLI が Webhook URL を読まないこと（AC-06・AC-07）は、`_checks/residue` の `ignores-webhook` で PR-1 の head で一度だけ確かめる（1.6 節、設計書 1.4 節）。
-   **削除の確かめ:** 削除したものがもうないことは `_checks/residue` で一度だけ確かめる（ステップ 1-5・2-6）。
-   **テストの削除の確かめ:** 1.3 節「網羅率の比べ方」のとおりに比べる（ステップ 1-2・1-3・2-2・2-3）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 移し替えた費用とトークンのテストが、公開の段階に届かないまま通る（`--out` の事前の確かめで止まる） | すべての段階の後の失敗の経路の確かめが失われる | 失敗が公開の段階で起きたことを各テストが確かめる（ステップ 1-2）。差し替えた `Publisher` を壊して失敗することを確かめる |
| ステップ 1-3 が大きく、網羅率の下がった関数の理由が追えない | 削除と関係のない確かめが失われる | 移し替えをステップ 1-2 に分け、ステップ 1-3 は `_checks/coverage` で関数ごとに実行されなかった文の数を比べる（1.3 節） |
| `slack-flag` の確かめが merge-base の CLI で本物の LLM や Webhook に触れる | 課金や投稿が起きる | 偽の `yt-dlp` が字幕を出さずに失敗し、`SLACK_WEBHOOK_URL` はループバックを、プロキシは閉じたアドレスを指す（1.3 節） |
| `make coverage-compare` がステップ 2-3 で止まる（残る関数の範囲に差分がある） | 確かめの記録ができない | ステップ 2-3 では残る関数の中を書き換えず、コメントはステップ 2-4 に分ける（設計書 7.4 節）。止まったときは `_checks/coverage` で比べ、そのことを記録する |
| 本物の Webhook の統合テストを実行できない | AC-16 の本物の投稿が確かめられない | 同意と URL の扱いは単体テストで確かめ、実行しなかったことを記録する（ステップ 2-6） |
| 予定の遅れ | 小さい | 削除と書き換えが中心で、新しい仕組みは確かめのスクリプト 2 つだけである |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（実行するテスト、`make` の対象、実行して確かめるスクリプト）、`static`（コミットしたスクリプトや `make` の対象による静的な確かめ）、`diff`（`_checks/` のスクリプトで merge-base と PR の head を比べる）、`manual`（PR のレビューで読む、承認の後に実行する）である。`_checks/residue` の確かめの名前は 1.4 節の末尾の表による。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | test | `_checks/residue` の `slack-flag`（`--slack` の各形が終了コード `2` で副作用なく拒否され、対照の実行が `yt-dlp` に届く）。`cmd/yt2column/run_test.go::TestRunExecutionPaths/an_undefined_flag`（未知のフラグの拒否、恒久） | 1-1・1-5 |
| AC-02 | test | `cmd/yt2column/run_test.go::TestCheckOut`、`TestRunExecutionPaths/no_--out` | 1-3 |
| AC-03 | test・static | test: `cmd/yt2column/run_test.go::TestRunHelp`。static: `_checks/residue` の `cli-code`（`"slack"` のフラグの定義がない）と `slack-literal`（使い方の文に `--slack` がない） | 1-3・1-5 |
| AC-04 | test | `cmd/yt2column/run_test.go::TestRunExecutionPaths`（`success`、`--out with an empty path`、`inside the cache directory` の行、`--review-log` と `--out` が同じファイルの行）、`TestRunExecutionPaths/review-log_write_failure_after_the_publish`、`TestRunReviewLog/--out`、`cost_test.go::TestRunCostLinesFileReviewLogFailure`、`token_test.go::TestRunTokenLinesFileReviewLogFailure` | 1-2・1-3 |
| AC-05 | test・static | test: `cmd/yt2column/run_test.go::TestRunGODEBUGWarning`。static: `_checks/residue` の `cli-code`・`cli-words` | 1-3・1-5 |
| AC-06 | test | `_checks/residue` の `ignores-webhook`（`TestRunIgnoresSlackWebhookURL`。1.6 節） | 1-6 |
| AC-07 | test | `_checks/residue` の `ignores-webhook`（`TestRunIgnoresSlackWebhookURL`・`TestLoadIgnoresSlackWebhookURL`。1.6 節） | 1-6 |
| AC-08 | test | `internal/config/config_test.go::TestLoadServerValid`・`TestLoadServerWebhookOptional`、`internal/slackwebhook/slackwebhook_test.go::TestValidURL`、`cmd/yt2column-server/run_test.go::TestStartsWithoutWebhook`、`internal/server/publish_test.go::TestPublishWithoutWebhook` | 1-2 |
| AC-09 | static | `_checks/residue` の `publish-method`。`internal/publisher/file.go:45` の `var _ Publisher = (*FilePublisher)(nil)`（`make test` のビルド） | 2-1・2-6 |
| AC-10 | static | `_checks/residue` の `config` | 1-1・1-5 |
| AC-11 | static | `_checks/residue` の `make-target`・`slack-literal` | 1-1・1-5 |
| AC-12 | static | `_checks/residue` の `cli-code`・`slack-literal` | 1-1・1-5 |
| AC-13 | static | `make deadcode`（ステップ 1-3・1-5・2-3・2-6 の完了の条件） | 1-3・2-3 |
| AC-14 | test | ステップ 1-2・1-6・2-2・2-3 の `make coverage-compare`、ステップ 1-3 の `_checks/coverage`。結果は各コミットのメッセージ、PR の説明、7.1 節にある | 1-2・1-3・1-6・2-2・2-3 |
| AC-15 | test | `internal/publisher/slack_test.go::TestSlackPost*`（1.5 節の 12）・`TestPrepareSlackMessagesRejectionNotPostError`・`TestPostMessages*`、`internal/server/publish_test.go::TestPublishPostsPreparedMessages`、ステップ 2-2・2-3 の `make coverage-compare` | 2-2・2-3 |
| AC-16 | test・manual | test: `internal/publisher/testutil/integration_settings_test.go::TestSlackSettingsFrom`、`internal/publisher/makefile_test.go::TestMakeTestIntegrationSlack`、`make lint`（`go vet -tags integration` が統合テストをビルドする）。manual: 承認の後の `make test-integration-slack` の実行（ステップ 2-6） | 2-3・2-6 |
| AC-17 | test・manual | test: `cmd/yt2column/docs_test.go::TestREADMEDocumentsFlags`（表と定義の一致）。manual: PR-1 のレビューで `cli.md` を読む | 1-4 |
| AC-18 | test・manual | test: `make readme-sync-check`。manual: PR-1 のレビューで README と `translation_glossary.md` を読む | 1-4 |
| AC-19 | test・manual | test: `cmd/yt2column/docs_test.go::TestSlackDocsContract`（`development.md`・`security.md` の値）。manual: PR-1・PR-2 のレビューで AC-19 が挙げる文書を読む | 1-4・2-5 |
| AC-20 | static・manual | static: `_checks/residue` の `cli-words`（PR-1）と `comments`（PR-2）（1.3 節の語の一覧）。manual: PR-1・PR-2 のレビューでコメントの文の意味を読む | 1-3・2-4・2-6 |
| AC-21 | test | `cmd/yt2column/docs_test.go::TestSlackDocsContract`・`TestDeepSeekDocsContract`（`make test`） | 1-3 |
| AC-22 | diff | `_checks/residue` の `unchanged`（PR-1・PR-2 のそれぞれの merge-base と head） | 1-5・2-6 |

### 7.1. 実行の記録

#### PR-1

**`_checks/residue`（ステップ 1-5）:** merge-base `43e51be6b1713163487cdf09c390ccb38505e08f`、head `2de9bb0265e4b5ed60cdb0cc4f44218954db9bc5`（ステップ 1-4 のコミット。この記録のコミットはこの節だけを変える）。

```text
go run -tags test ./docs/tasks/0062_remove_cli_slack/_checks/residue/ 43e51be6 2de9bb02
```

```text
OK   slack-flag
OK   cli-code
OK   slack-literal
OK   cli-words
OK   config
OK   make-target
OK   unchanged
exit 0
```

-   merge-base を head として渡すと（`43e51be6` を 2 つの引数に渡す）、`unchanged` だけが成り立ち、ほかの 6 つの確かめは成り立たず、終了コード `1` で終わる（slack-flag 24、cli-code 14、slack-literal 55、cli-words 18、config 2、make-target 9 の指摘）。`slack-flag` では、`--out` を付けない `--slack`・`--slack=true`・`-slack`・`-slack=true` の 4 つの形で偽の `yt-dlp` の印のファイルができた（受理された形が `yt-dlp` に届いた）。`--slack=false`・`-slack=false` の形と、`--out` を同時に指定したすべての形の 12 の形は、標準ライブラリの未知のフラグの行を書かなかった（今の CLI は専用の拒否で止まる）。URL の後の 2 つの形は、どちらのコミットでも引数の数の誤りで止まる。対照の実行（`--out` だけ）は、どちらのコミットでも印のファイルを作った。
-   `unchanged` が失敗しうること: ブランチに載せない一時的なコミットを head として渡した。`extension/src/background.ts` に 1 行を加えたコミット `f21c685f` では `NG unchanged: extension/src/background.ts changed`、`internal/server/publish.go` に 1 行を加えたコミット `7b73ba9c` では `NG unchanged: internal/server/publish.go (a product file) changed` と出て、どちらも終了コード `1` で終わった。

**網羅率の比較（ステップ 1-2、`make coverage-compare`）:** ステップ 1-2 のコミット `6b0fca75`（tree `711594f9`）。出力の `head` はメッセージを直す前のコミット ID で、tree ID は同じである。

```text
target    make coverage-compare BASE="HEAD~1"
base      a97a28e33bb7ffaeecc922b05a5eaf975ff6054c
head      ecffd0da7ec2eedeeab89327c535c4a9fd01cef4 tree 711594f9fc661549c21796c94dc7b345a883f29a
toolchain go1.27.1
tests     base 1266 head 1272
files     base 136 head 136
functions base 1288 head 1288
deleted   0
no compared function lost coverage
coverage-compare: PASS
```

**網羅率の比較（ステップ 1-3、`_checks/coverage`）:** before はステップ 1-2 のコミット `6b0fca75`、after はステップ 1-3 のコミット `1f9b9055`（tree `744792a5`）。どちらも、そのコミットのクリーンな worktree で `go test -tags test -count=1 -coverpkg=./... -coverprofile=<file> ./...` を実行し、すべてのテストが通った。

```text
before  6b0fca75 before.prof (1288 product functions)
after   1f9b9055 after.prof (1278 product functions)
only before (removed): 11
     cmd/yt2column buildOutput (2 of 10 unexecuted)
     cmd/yt2column chooseDestination (0 of 12 unexecuted)
     cmd/yt2column messageCount (0 of 3 unexecuted)
     cmd/yt2column newSlackPublisher (1 of 4 unexecuted)
     cmd/yt2column rejectedPostStatus (0 of 3 unexecuted)
     cmd/yt2column reportWebhookHints (0 of 14 unexecuted)
     cmd/yt2column seconds (0 of 1 unexecuted)
     internal/config Config.RequireSlackWebhookURL (0 of 3 unexecuted)
     internal/config Config.SlackWebhookURL (0 of 3 unexecuted)
     internal/job RemoteOutput (0 of 1 unexecuted)
     internal/publisher SlackMessageCount (0 of 4 unexecuted)
only after (added): 1
     cmd/yt2column checkOut (0 of 4 unexecuted)
more unexecuted (fails): 1
     internal/cmdrunner signalGroup: 0 of 3 unexecuted -> 1 of 3 unexecuted
fewer executed (give each a reason): 10
     cmd/yt2column newFlagSet: executed 19 -> 18 (0 of 19 unexecuted -> 0 of 18 unexecuted)
     cmd/yt2column reportReviewLogFailure: executed 5 -> 2 (0 of 5 unexecuted -> 0 of 2 unexecuted)
     cmd/yt2column reportRunError: executed 19 -> 18 (0 of 19 unexecuted -> 0 of 18 unexecuted)
     cmd/yt2column reportSuccess: executed 13 -> 7 (1 of 14 unexecuted -> 0 of 7 unexecuted)
     cmd/yt2column run: executed 61 -> 57 (0 of 61 unexecuted -> 0 of 57 unexecuted)
     cmd/yt2column warnHTTP2Debug: executed 5 -> 3 (0 of 5 unexecuted -> 0 of 3 unexecuted)
     internal/cmdrunner signalGroup: executed 3 -> 2 (0 of 3 unexecuted -> 1 of 3 unexecuted)
     internal/config LoadFor: executed 10 -> 8 (0 of 10 unexecuted -> 0 of 8 unexecuted)
     internal/job precheckOutput: executed 5 -> 3 (1 of 6 unexecuted -> 0 of 3 unexecuted)
     internal/job validateRequest: executed 12 -> 8 (0 of 12 unexecuted -> 0 of 8 unexecuted)
fewer unexecuted: 3
     cmd/yt2column reportSuccess: 1 of 14 unexecuted -> 0 of 7 unexecuted
     internal/job precheckOutput: 1 of 6 unexecuted -> 0 of 3 unexecuted
     internal/slackwebhook lastPathSegment: 1 of 6 unexecuted -> 0 of 6 unexecuted
NG   1 function(s) found on both sides have more unexecuted statements
```

-   削除した 11 の関数は、ステップ 1-3 が削除するものである。加えた `checkOut` は `TestCheckOut` と `no --out` の行が実行する。
-   この比較は (a) の 1 つの関数のために終了コード `1` で終わった。その関数の扱いは次のとおりである。
-   (a) `internal/cmdrunner` の `signalGroup`: ステップ 1-3 は `internal/cmdrunner` を変えていない。増えた 1 文は、プロセスグループへの `kill` が `ESRCH` 以外で失敗したとき（ゾンビだけが残る間）の `process.Signal` で、子の終わる時刻の巡り合わせで通るかが決まる。同じ tree を 3 回測ると、この分岐を通った回数は 0・0・1 だった。削除したテストはこの分岐に届かない。
-   (b) 実行された文が減った 10 の関数は、どれも削除した文の分だけ減り、after で実行されなかった文は 0 である（`signalGroup` を除く）: `newFlagSet`（`--slack` の定義）、`reportReviewLogFailure`（Webhook の文言の分岐）、`reportRunError`（`reportWebhookHints` の呼び出し）、`reportSuccess`（Webhook と既定の要約の分岐）、`run`（Webhook URL と `RequireSlackWebhookURL` の文）、`warnHTTP2Debug`（Webhook の警告）、`LoadFor`（`loadSlackWebhook` の呼び出しと代入）、`precheckOutput`・`validateRequest`（出力の種類の分岐）。
-   最初の比較では、`run` の「build the publisher」の失敗の分岐と `splitJoined` の結合されていない誤りの分岐が (a) に挙がった（削除した `--slack` のテストだけが実行していた）。`executionPathRows` の `the --out publisher cannot be built` の行と `TestSplitJoined` をステップ 1-3 のコミットに加え、上の比較で解消した。
-   `signal_test.go` の子の CLI のコードは coverprofile に現れないので、`TestSignalDuringSlackPost` の削除はこの比較に現れない。

**ステップ 1-6 の記録（1.6 節）**

`_checks/residue`: merge-base `43e51be6b1713163487cdf09c390ccb38505e08f`、head `1e4eb2970e49499c1e69c82ca422ddeaaaa41a59`（ステップ 1-6 のコードの最後のコミット。この記録のコミットは本書と設計書だけを変える）。

```text
go run -tags test ./docs/tasks/0062_remove_cli_slack/_checks/residue/ 43e51be6 1e4eb297
```

```text
OK   slack-flag
OK   ignores-webhook
OK   cli-code
OK   slack-literal
OK   cli-words
OK   config
OK   make-target
OK   unchanged
exit 0
```

-   merge-base を head として渡すと、`ignores-webhook` は成り立たない。`TestRunIgnoresSlackWebhookURL` は、`lookup` が `SLACK_WEBHOOK_URL` を問い合わせたことと、空・`https` でない値の実行が設定エラーで止まり `--out` のファイルがないことで失敗し、`TestLoadIgnoresSlackWebhookURL` は、`Load` が 2 つの値を拒否したことで失敗した。ステップ 1-6 の前の head では、写したテストがパッケージの同じ名前のテストと重なるので、この確かめは使えない（テストを削除するコミットの後で実行する）。

テストだけを削除するコミット `097bb00c`（tree `388d8cb1`）の `make coverage-compare`（出力の `head` はメッセージを直す前のコミット ID）:

```text
target    make coverage-compare BASE="d99edbf1"
base      d99edbf15e1be04e28332552905c9ef28b024990
head      42a19a38d54e3463677741ab1d1187a7578e8feb tree 388d8cb17c73c044113a84b9e4f0c3734b98b203
toolchain go1.27.1
tests     base 1260 head 1255
files     base 136 head 136
functions base 1278 head 1278
deleted   0
no compared function lost coverage
coverage-compare: PASS
```

`splitJoined` と `TestSplitJoined` を削除するコミット `1e4eb297`（tree `61d45190`）の `make coverage-compare`:

```text
target    make coverage-compare BASE="097bb00c"
base      097bb00c2e383764608396c374af7a74fd599354
head      4047f474a43d2b2cfaecbec0fbfa62c2589dab2b tree 61d451905ded428e3ec29ee97cfb7393624a69be
toolchain go1.27.1
tests     base 1255 head 1254
files     base 136 head 136
functions base 1278 head 1277
deleted   1
deleted   "cmd/yt2column/run.go":908 "splitJoined"
no compared function lost coverage
coverage-compare: PASS
```

-   どちらの比較も、`PASS` の前の実行が、このタスクと関係のない `internal/transcript` の `TestCommandExecutorDrainsStderr` の失敗（負荷の下で 2 秒の期限を超える。issue #684）で `STOP test-failed` になった。テストのコードを変えずに実行し直して `PASS` を得た。

#### PR-2

**`make test-integration-slack`（ステップ 2-6、AC-16）:** 利用者の承認を得て、2026-11-05 にコミット `0bb1e58b`（ステップ 2-5 のコミット。統合テストはステップ 2-3 から変わっていない）で実行した。`TestIntegrationSlackWebhookPublisher` が通った（single は 1 通、split は 3 通。どちらも `PostMessages` が誤りなしで返り、`delivered` の回数が通数と一致した）。

**`_checks/residue`（ステップ 2-6）:** merge-base `363b69280a7ade3d92fa85f2f4e56ab50279b0a8`、head `e65e67612a6d1993567b70fa8ea8ca0e1d8a0af7`（PR-2 のコードの最後のコミット。この記録のコミットは本書だけを変える）。

```text
go run -tags test ./docs/tasks/0062_remove_cli_slack/_checks/residue/ 363b6928 e65e6761
```

```text
OK   slack-flag
OK   ignores-webhook
OK   cli-code
OK   slack-literal
OK   cli-words
OK   config
OK   make-target
OK   publish-method
OK   comments
OK   unchanged
exit 0
```

-   PR-2 の merge-base `363b6928` を head として渡すと、`publish-method`（`slack.go:171` の `SlackWebhookPublisher.Publish`）と `comments`（16 の指摘）が成り立たず、ほかの確かめは成り立って、終了コード `1` で終わる。PR-1 の merge-base `43e51be6` を head として渡しても、`publish-method`（1）と `comments`（37）が成り立たない。
-   `publish-method` が `FilePublisher` の側と埋め込みのフィールドでも失敗しうること: ブランチに載せない一時的なコミットを head として渡した。`FilePublisher.Publish` の名前を変えたコミットでは「`FilePublisher.Publish` がない」、`SlackWebhookPublisher` に `sync.Mutex` を埋め込んだコミットでは埋め込みのフィールドの指摘が出た。

**網羅率の比較（ステップ 2-2、`make coverage-compare`）:** ステップ 2-2 のコミット `6cfd488d`（tree `d996c9e2`）。出力の `head` はメッセージを直す前のコミット ID で、tree ID は同じである。

```text
target    make coverage-compare BASE="7beecad2~1"
base      6f104eeb523636f0616146bf112eaa536d5d56f9
head      7beecad24495935edb01f4bf888069707e8e26a3 tree d996c9e29430a19f25b1b2398bbfa6ddcd879b3b
toolchain go1.27.1
tests     base 1256 head 1270
files     base 136 head 136
functions base 1277 head 1277
deleted   0
no compared function lost coverage
coverage-compare: PASS
```

**網羅率の比較（ステップ 2-3、`make coverage-compare`）:** `Publish` を削除するコミット `90234cca`（tree `4ec140df`）。

```text
target    make coverage-compare BASE="e3d64e16~1"
base      6cfd488db409338e0bb14c5958c7d9e6c67dfab4
head      e3d64e1674d89cec30111dbbda5a02b0bcdea362 tree 4ec140dfdbbd46d2e298ea9308de279086ebe7f8
toolchain go1.27.1
tests     base 1270 head 1254
files     base 136 head 136
functions base 1277 head 1276
deleted   1
deleted   "internal/publisher/slack.go":171 "(*SlackWebhookPublisher).Publish"
no compared function lost coverage
coverage-compare: PASS
```

-   ステップ 2-2 の比較は、最初の実行が `internal/transcript` の `TestCommandExecutorDrainsStderr` の失敗（issue #684）で `STOP test-failed`、次の実行が `internal/cmdrunner` の `signalGroup` の 3/3 → 2/3（PR-1 の記録の、子の終わる時刻で通るかが決まる分岐。PR-2 は変えていない）で `FAIL` になり、変えずに実行し直して `PASS` を得た。ステップ 2-3 の比較も、最初の実行が同じ `signalGroup` で `FAIL` になり、実行し直して `PASS` を得た。

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 7 章のすべての行の確かめが済み、7.1 節に `_checks/residue` の実行の記録がある。
-   **品質:** グリーンゲート（`make test && make lint`）と `make deadcode` が通る。テストの移し替えと削除の各コミットで、網羅率の比較の結果が 7.1 節にある。
-   **セキュリティ:** CLI が `SLACK_WEBHOOK_URL` を読まず（AC-06）、製品のコードで Webhook へ送るのがサーバの投稿のジョブだけである（AC-05・AC-09）。
-   **文書:** AC-17〜AC-19 が挙げる文書と CLAUDE.md が、削除の後の振る舞いだけを説明している。過去のタスクの文書は変えない（要件書 2.3 節）。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`/runpr` が issue #642 を閉じる案内をする（本タスクの最後の PR のため）。
-   CLI から投稿を確かめる必要が再び生じたときは、サーバのジョブの API を使う（設計書 9 章）。

## 変更履歴

- 2026-11-04 承認（isseis、#671）
- 2026-11-05 セッション中の再承認：CLI が `SLACK_WEBHOOK_URL` を読まないことの確かめを `_checks/residue` に移し、重なるテストと網羅率のためだけのテストを削除する（isseis、#682）
