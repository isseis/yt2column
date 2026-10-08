# 実装計画書：Slack 互換の Webhook への投稿（SlackWebhookPublisher）

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-07 |
| Review date | 2026-10-07 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定義した Slack 互換の Incoming Webhook への投稿を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。具体的には、次のことを行う。

-   Webhook URL の規則を `internal/slackwebhook` に移し、`internal/config` と共有する。
-   `SlackWebhookPublisher`（準備と送信）を作る。
-   `internal/job` の投稿先を `job.Output` で表し、CLI に `--slack` を加える。
-   実際の Mattermost を使う統合テストと、それを実行する `make` のターゲットを加える。
-   文書を更新し、手動確認の結果を記録する。

本計画が使う記号は次のとおりである。AC-NN は要件書の受け入れ基準、F-NNN は要件書の機能要件、H-NN は [design_handoff.md](design_handoff.md) の項目を指す。V1〜V3（検査用の変換）と M1〜M4（拒否する記法の規則）は設計書 3.4、手順 A1〜C（CLI の `run` の手順）は設計書 3.10、E1〜E4（既存の方針や要件書の記述との違い）は設計書 3.14 で定義されている。

本計画は、設計書 §8 の実装優先順位 1〜8 を、そのままフェーズ 1〜8 とする。

### 1.2. 実装原則

-   設計書 §1.1 の設計原則に従う。特に、Webhook に固有のことを `internal/publisher` の Webhook 用のファイルに閉じ込めること、送る前に準備（記事の検査・分割・記法の検査）を終えること、Webhook URL をエラーにも構造体の表示にも入れないこと、投稿先を型で表すこと、URL の規則を 1 か所に置くことを守る。
-   本番コードで新設・変更するファイルは、設計書 §3.14 の表に挙げたものに限る。`test` のタグのファイルに加える、設計書にない補助（ステップ 4-2 の準備の結果を返す関数と、`Transport` を受け取る構築の補助）は本計画で定め、`package_reference.md` に記す（ステップ 4-4）。
-   ユニットテストのファイルの先頭には `//go:build test` を付ける。統合テストは `//go:build integration` とし、統合テストからも使う補助だけを `//go:build test || integration` とする（`docs/dev/developer_guide/test_organization.md` の例外）。
-   Go のコメント・識別子・文字列リテラルは英語で書く。`AC-NN`・`F-NNN`・`H-NN` は Go のソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
-   **テスト用の補助の lint。** `_test.go` でないテスト用のビルドのファイル（`test_helpers*.go`、`testutil/` の `.go`、`internal/loopbacktest/loopbacktest.go`）には、`.golangci.yml` の `_test.go` 向けの除外（`gosec`・`err113`・`errcheck`・`goconst`・`gocyclo`・`dupl`）が効かない。これらのファイルを作るか変えるステップ（2-1・2-3・4-2・5-3・6-6・7-1・7-2・7-6）では、テストが注入するエラーをパッケージの静的なエラーとして宣言し（`err113`）、後始末で無視する戻り値を `_ =` で受け（`errcheck`）、`gosec` に当たる行にだけ理由を付けた `//nolint:gosec // <理由>` を付ける（既存の `internal/llm/deepseek/testutil/make.go:92` と同じ形）。
-   テストの名前は計画上の名前である。実装で変える場合は、§5 を同じコミットで更新する。
-   各テストは、対象の処理を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（[CLAUDE.md](../../../CLAUDE.md)「Testing Strategy」）。壊す対象は各フェーズの壊し確認のステップに挙げる。
-   各フェーズの完了条件は、`make fmt` → `make test` → `make lint` が通ることである。`make lint` は `--build-tags test,integration` で解析し、続けて `go vet -tags integration ./...` を実行する（`Makefile` の `GOLINT` と `lint`）。このため、そのフェーズで加えたタグ付きのファイルは、実際に使うタグでコンパイルされる。
-   CI は、変更が `*.md` と `docs/` だけの PR ではテストを実行しない（`.github/workflows/ci.yml` の `check-changes`）。文書だけを変えるコミットでも、`make test` を手元で実行する（`cmd/yt2column/docs_test.go` と `internal/pipeline/pipeline_test.go` の文書のテストのため）。
-   実際の Mattermost の Webhook と実 DeepSeek API を使う作業（ステップ 7-13・8-6・8-7）は、利用者の承認を得てから行う（[CLAUDE.md](../../../CLAUDE.md)「Tool Execution Safety」）。

### 1.3. 既存コード調査結果

HEAD `44bc4df`（ブランチ `issei/0006-slack-webhook-publisher-02`）で確認した。設計書が行番号を記したコミット `1457754` から HEAD までに、`cmd/`・`internal/`・`Makefile`・`README.md`・`docs/dev/` の変更はない（`git diff --stat 1457754 HEAD -- cmd internal Makefile README.md docs/dev` の出力が空。2026-10-07）。したがって、設計書の `file:line` は HEAD でもそのまま使える。以下の `file:line` も HEAD `44bc4df` のものである。

**`internal/config`**

-   `validSlackWebhook` の参照は `config.go:223`（呼び出し）・`:273-278`（doc コメントと定義）だけである。`reasonSlackWebhook` は `config.go:53`（定義）・`:224`（使用）だけで、文言を比べるテストはない。`loadSlackWebhook` の doc コメント（`config.go:214-215`）は「a present value must be a hooks.slack.com URL」と書いており、規則の変更に合わせて直す（ステップ 1-3）。
-   `validSlackWebhook` を消すと、`config.go` の `unicode` の import を使う箇所がなくなる可能性がある。不要になった import は同じステップで消す（コンパイラが検出する）。
-   `hasHTTP2Debug` は `config.go:158`（呼び出し）・`:280-290`（定義）だけである。`TestLoadHTTP2Debug`（`config_test.go`）が `Load` を通して `http2debug=1`・`=2`・他の設定と並んだ形・未設定・`=0` の行を確かめている。`HTTP2DebugEnabledIn` は `hasHTTP2Debug` から判定の本体を取り出すだけなので、同じ行の表を `HTTP2DebugEnabledIn` に対して重ねて作らない（[CLAUDE.md](../../../CLAUDE.md)「Check for duplication before adding a table test for a helper」）。統合テストの判定のテスト（ステップ 7-3）が `GODEBUG` の行を持つ。
-   `SLACK_WEBHOOK_URL` の拒否の表は `config_test.go:172-178`（`TestLoadInvalid` の `slack webhook` の行）にある。`https://hooks.slack.com.example/services/x`（`:174`）・`https://HOOKS.SLACK.COM/services/x`・`https://hooks.slack.com/`（`:176`）は新しい規則で受理される。`config_test.go:211`・`:222`・`:246`・`:265` の値（`https://hooks.slack.com/services/…` と `http://…`）は、新しい規則でも受理・拒否が変わらない。
-   新しい規則（`net/url` が解析でき、スキームが `https`、ホストが空でない）の判定を、作業用の一時ディレクトリ（リポジトリの外）に置いた小さな Go のプログラムで確かめた（`go1.27.1`、2026-10-07）。AC-01 の 2 つの URL、`https://hooks.slack.com.example/services/x`・`https://HOOKS.SLACK.COM/services/x`・`https://hooks.slack.com/` は受理し、AC-03 の 4 つの値、E4 の `https://hooks.slack.com/services/%zz`・`https://hooks.slack.com/%`・末尾に DEL を付けた値、先頭に空白を付けた値は拒否した。`HTTPS://example.com/x` は `net/url` がスキームを小文字にするので受理した（要件書 F-001 の「スキームが `https`」に反しない）。
-   同じ確認で、`https://:443/x` は `url.URL.Host` が `":443"`、`Hostname()` が空になることを確かめた。「ホストが空でない」は `Hostname()` で判定する（ポートだけの値を受理すると、自分のマシンへ接続する。ステップ 1-1）。
-   従来の規則は空白文字を含む値を拒否していた（`config.go:277`）。新しい規則では、末尾に空白を付けた `https://hooks.slack.com/services/x ` やパスの途中の空白を含む値を `net/url` が解析でき、受理する（同じ確認で、`Host` が `hooks.slack.com` になることを確かめた）。要件書 F-001 の規則どおりの振る舞いだが、要件書 4.4 と設計書 3.14 の E4 には書かれていない。本計画は F-001 の規則に従い、空白を拒否する検査を加えない。利用者が、この振る舞いをリスクとして受け入れることを決めた（2026-10-07。§6.1）。
-   秘密の変数の名前を本番のコードに書けるのは `internal/config` だけである（`envaccess_test.go:57-68` の `secretEnvNames`・`allowedEnvRefs`）。`isTestCode`（`envaccess_test.go:279-299`）は、`_test.go` のファイルと、ビルド制約が `test` か `integration` のタグなしでは満たされないファイルを、テストのコードとして除外する。`YT2COLUMN_TEST_SLACK_WEBHOOK_URL` は `SLACK_WEBHOOK_URL` を部分文字列として含むが、置き場所（`internal/publisher/testutil/integration.go`、`//go:build test || integration`）はこの除外に当たる。

**`internal/llm/deepseek`（ループバックの判定の移動）**

-   `validateLoopbackEndpoint`・`errTestEndpointNoHost`・`errTestEndpointNotLoopback` は `test_helpers.go:42-73` にある。呼び出しは `test_helpers_endpoint.go:18` と、直接のテスト `TestNewTestClientRejectsNonLoopback`（`deepseek_test.go:166-187`）だけである（`rg -n "validateLoopbackEndpoint|errTestEndpointNoHost|errTestEndpointNotLoopback" --type go`、HEAD `44bc4df`）。
-   `TestNewForLoopbackTestRejectsNonLoopback`（`deepseek_test.go:215`）は `NewForLoopbackTest` を通して非ループバックの拒否を確かめるので、移動の後も残す。`fatalRecorder`（`deepseek_test.go:190-202`）は `deepseek` のテストのファイルの中の型であり、`internal/publisher` からは使えない。

**`internal/llm/deepseek/testutil`**

-   `RunMakeTarget(t, root, target, model)`（`make.go:81`）が記録する変数は、固定の `recordedEnv`（`make.go:45`。`DeepSeekOptInEnv`・`CLIOptInEnv`・`ModelEnv`）だけである。呼び出しは `make.go:170`（`CheckChargedTarget` の中）、`internal/llm/deepseek/makefile_test.go:31`、`cmd/yt2column/makefile_test.go:49` の 3 か所である。
-   `CheckChargedTarget`（`make.go:149-189`）は、出力に `calls the real DeepSeek API, which incurs charges` を含むこと、`go test` の引数、`-timeout` が `MinTimeout` を超えること、`c.OptInEnv` が `1` で記録されること、`ModelEnv` を確かめる。`c.OptInEnv` は `recordedEnv` にある変数でなければ記録されないので、Webhook の CLI のターゲットに使うには、`CheckChargedTarget` が `c.OptInEnv` を追加の記録の対象として `RunMakeTarget` に渡す必要がある（ステップ 7-1）。

**`internal/publisher`**

-   `renderArticle`（`file.go:178-185`）の doc コメントは「returns the file content」である。内容は変えず、コメントだけ直す（ステップ 3-3）。
-   `internal/publisher` のテストのファイルは `file_test.go`（`package publisher`）だけで、補助は `test_helpers.go`（`//go:build test`。`newFilePublisherWithSeams` など）にある。`internal/publisher/testutil` は `mocks.go`・`mocks_test.go`（`FakePublisher`）だけである。

**`internal/job`**

-   `OutPath` の参照は、`job.go` の 4 行（フィールドの定義、`precheckOutput(req.OutPath)`、`validateRequest` の `case` とそのエラーの文言）、`job_test.go` の 30 行（`rg -c OutPath internal/job/job_test.go`）、`internal/job/test_helpers.go` の `newOutput`（`:40-49`）、`cmd/yt2column/run.go:196` である（HEAD `44bc4df`。`cmd/yt2column/outpath_test.go` の 4 件は関数名 `TestOutPathInsideCacheDir…` で、`Request` とは関係しない）。
-   `job_test.go` の `Request` の組み立ては、複合リテラルの `OutPath: …, … Publisher: …` の形と、`req.OutPath, req.Publisher = …`（`job_test.go:216`・`:224`）の代入の形の 2 種類がある。`TestRunValidatesRequest`（`job_test.go:841`）は `empty out path`（`:856`）・`nil publisher`・`typed-nil publisher` の行を持つ。

**`cmd/yt2column`**

-   `deps.newPublisher` の参照は、`run.go:42`（フィールド）・`:51`（`productionDeps`）・`:188`（呼び出し）と、`run_test.go:474`・`:547`・`:647`・`:881` の差し替えである。パッケージの関数 `newFilePublisher`（`run.go:58`）が既にあり、設計書 3.10 のフィールド名 `newFilePublisher` と同じ名前になる。Go ではフィールドと関数の名前は衝突しない（`productionDeps` で `newFilePublisher: newFilePublisher` と書ける）ので、関数の名前は変えない。
-   `testDeps`（`test_helpers.go:131-135`）と、`run_test.go` の `newRunEnv`（`run_test.go:95-129`）は、どちらも `productionDeps()` から始まる。`newRunEnv` の環境は `SLACK_WEBHOOK_URL` に `https://hooks.slack.com/…` の形の `testWebhook`（`run_test.go:37`）を持つので、`--slack` の行を足すと、そのままでは実際の `newSlackPublisher` が作られる。設計書 3.10 の「送らない側に倒す」は、この 2 か所の両方に適用する（設計書 3.14 に記録済み）。`TestMain`（`main_test.go:22-45`）は、プロキシの環境変数を閉じたループバックのアドレスへ向けているので、誤って作られた本番の `SlackWebhookPublisher` の `https` の送信もネットワークに出ない。これは代用品に加わる 2 つ目の層であり、代用品の代わりにはしない。
-   `run`（`run.go:129`）の循環的複雑度は 16 である（`gocyclo` で計測、2026-10-07）。`.golangci.yml` の `gocyclo` の上限は 20 なので、投稿先の分岐を `run` の中に足すと上限を超えうる。投稿先の決定（手順 A2）と、成功の要約（手順 C）は別の関数にする（ステップ 6-2・6-3）。
-   `reportRunError`（`run.go:232-254`）には、ステップ 6-4 で 4 つの案内が加わる。`run` と同じく、Webhook の案内は別の関数にまとめ、`gocyclo` の上限を超えないようにする。
-   `reportRunError` の `minutes`（`run.go:257-259`）は時間を分の整数で書く。`SlackPostTimeout`（30 秒）を渡すと `0-minute` になるので、Webhook のタイムアウトの案内には使わない（ステップ 6-4）。
-   `configuredSecrets`（`run.go:277-288`）は `SLACK_WEBHOOK_URL` の値全体だけを返す。
-   `executionPathRows`（`run_test.go:321`）と `TestRunExecutionPaths`（`run_test.go:736`）は、各経路の終了コードと 0005 の AC-44 の各項目（副作用、標準出力が空、秘密の値が出ないことなど）を行ごとに確かめる既存の仕組みである。成功の行の既定の検査は `--out` のファイルの存在（`run_test.go:800-804` の `default:`）なので、Webhook の行では検査を差し替える必要がある。`requireSafeOutput`（`run_test.go:174`）と `secretStrings`（`run_test.go:71`）は、秘密の値と末尾 8 文字が出力に現れないことを確かめる。ほかに、次の既存のテストと補助がある。Webhook の経路のテストは、これらに行を足すか、これらを使う。
    -   信頼できない文字列のエスケープ: `TestRunEscapesUntrustedText`（`:867`）
    -   `GODEBUG` の警告: `TestRunGODEBUGWarning`（`:924`）
    -   使い方: `TestRunHelp`（`:814`）
    -   構築の失敗で nil のインターフェースを返すこと: `TestNewFilePublisherNilOnFailure`（`:963`）
    -   `Done` を閉じずに `Err()` だけを変える `context`: `expiringContext`（`:197-220`）
-   子プロセスのモードは `runChildMode`（`test_helpers.go:140-156`）にあり、`stallingLLM` と `writeMarker`・`childReadyEnv` で準備完了を知らせる。`signal_test.go` の `requireInterruptedChildOutput`（`:87`）は `--out` のパスを引数に取る。
-   `cmd/yt2column/docs_test.go` の `configDocRows`（`:28-35`）が README と概要の設定の表の「未設定のとき」の欄を固定し、`TestREADMEDocumentsCLI` の `flags` の部分テストが `newFlagSet` のすべてのフラグを README のフラグの表に求める。`--slack` を足すと、README の表に `--slack` の行がなければ失敗する。`TestPlanRecordsManualRuns`（`:141`）は 0005 の計画書の手動確認の記録を確かめる仕組みであり、本計画の手動確認の記録にも同じ形を使う（ステップ 8-5）。
-   既存の統合テストの判定の補助 `gateCLIIntegration`・`lookupFrom`・`generateCounter` は `cmd/yt2column/test_helpers_integration.go`（`//go:build test || integration`）にある。`TestCLIIntegrationSettings`（`makefile_test.go:87`）は `gateRecorder`（`makefile_test.go:62-81`）で判定を確かめる。

**`internal/pipeline` のガードのテスト**

-   `TestFakesCarryBuildTag`（`pipeline_test.go:519`）は、`internal` の下のすべての `testutil/` の `.go` の数を 13 に固定している（`:541-542`）。現在の 13 件は `find internal -path '*/testutil/*.go'` の結果と一致した。ステップ 7-2・7-3 で `internal/publisher/testutil` に 2 件加わるので、15 にする（ステップ 7-10）。
-   `TestPackageReferenceListsPackages`（`pipeline_test.go:565`）は、`isTestOnlySource`（`:636-644`）が真のファイルを数えないので、`test` のタグのファイルだけを持つパッケージ（`internal/loopbacktest`）の行を `package_reference.md` に書くと「本番のコードがない」として失敗する。現在、`testutil/` 以外でテスト用のビルドのファイルだけを持つディレクトリはない（`cmd`・`internal` の各ディレクトリの `_test.go` 以外の `.go` の 1 行目を調べた。2026-10-07）。したがって、テスト用のビルドだけのファイルも数えるように改めても、増える行は `internal/loopbacktest` だけである（ステップ 2-4。設計書 3.14 に記録済み）。

**文書**

-   `--dry-run` は、要件書 2.3 で設けないことにした。`security.md:19` に「`--dry-run` の出力」の記述が残っている（`rg -n "dry-run" --glob '!docs/tasks/**' .` の結果はこの 1 件。`.claude/` を除く。HEAD `44bc4df`）。フェーズ 8 で直す（ステップ 8-3）。
-   README の「Webhook への投稿は未実装」の記述は `README.md:5-6` と `:24` の 2 か所にある。どちらもステップ 8-1 で直す。
-   `hooks.slack.com` は、`docs/tasks/` を除くと、`internal/config/config.go`（規則と doc コメント）と、`internal/config/config_test.go`・`cmd/yt2column/run_test.go` のテストの値にだけ現れる。テストの値は Slack の URL の形として残してよい（新しい規則でも受理・拒否は変わらない）。

**具体型の構築の経路**

-   `SlackWebhookPublisher` は非公開のフィールドだけを持ち、本番では `NewSlackWebhookPublisher` だけが作る（設計書 3.3）。パッケージの外から作れるのはゼロ値だけであり、ゼロ値の `Publish` は何も送らずに失敗する（ステップ 4-1・4-3）。テスト用の構築（`NewSlackWebhookPublisherForLoopbackTest` と、ステップ 4-2 の非公開の構築の補助）は `test` のタグのファイルにだけ置き、構築した後の値のフィールドは書き換えない（設計書 3.3 の「Its fields are set once by the constructor」）。`SlackWebhookPublisher` の具体型で分岐する処理はない。
-   `job.Output` は非公開のフィールドだけを持ち、`FileOutput`・`RemoteOutput` だけが作る。`job` の本番のコードでフィールドを書き換える処理は作らない。ゼロ値は `validateRequest` の `default` で拒否する（設計書 3.8）。

**architecture との整合**

-   設計書の決定を変える必要のある不整合は見つからなかった。
-   設計書 3.14 の表に、更新が必要な既存のテストとして、`internal/pipeline/pipeline_test.go` の 2 つのガードと、`run_test.go` の `newRunEnv` を、編集上の修正として書き加えた（コミット `49c0605`。決定の変更はない）。
-   設計書 3.14 の E4 が求める、要件書 4.4 の編集上の訂正を行った（コミット `49c0605`。決定の変更はない）。
-   設計書 3.12 は、統合テストで分割する記事について、1 つ目のメッセージを「分割の位置の表示を含めて 16,383 コードポイント」にするとしていた。3.5 の規則では、各断片は 16,383 − 9 = 16,374 コードポイント以内で、1 つ目の表示 `(1/N)\n\n` は 7 コードポイント（N が 10 なら 8）なので、1 つ目のメッセージは最大 16,381 コードポイント（N が 10 なら 16,382）にしかならない。設計書 3.12 と 7.4 の AC-30 の行は、上限ちょうど（16,383 コードポイント）のメッセージが受理されることを分割しない記事で確かめる形に、編集上の修正として直した（コミット `49c0605`。統合テストで確かめる事項は変わらない）。

### 1.4. 名前の変更・移動の一覧

本表が名前の変更と移動の唯一の一覧である。本計画の他の節は、本表に従う。

| 変更前 | 変更後 | ステップ |
|---|---|---|
| `config.validSlackWebhook`（非公開） | 削除。`slackwebhook.ValidURL` を呼ぶ | 1-3 |
| `config.hasHTTP2Debug` の判定の本体 | `config.HTTP2DebugEnabledIn`（`hasHTTP2Debug` は残し、これを呼ぶ） | 1-3 |
| `deepseek` の `validateLoopbackEndpoint`・`errTestEndpointNoHost`・`errTestEndpointNotLoopback`（`test_helpers.go`） | `loopbacktest.ValidateURL` と、`internal/loopbacktest` の非公開の静的エラー | 2-1・2-3 |
| `deepseek_test.go` の `TestNewTestClientRejectsNonLoopback` | `internal/loopbacktest/loopbacktest_test.go` の `TestValidateURL` | 2-2 |
| `job.Request` の `OutPath`・`Publisher` | `job.Request.Output`（`job.Output`、`job.FileOutput`・`job.RemoteOutput`） | 5-1 |
| `cmd/yt2column` の `deps.newPublisher` | `deps.newFilePublisher`（パッケージの関数 `newFilePublisher` は名前を変えない）と `deps.newSlackPublisher` | 6-1 |
| `pipeline_test.go` の `isTestOnlySource` | 削除（`TestPackageReferenceListsPackages` がテスト用のビルドだけのパッケージも数えるため、使う箇所がなくなる） | 2-4 |

### 1.5. design_handoff.md の項目への対応

`implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-07 は、すべて設計書 3.15 に対応が記録されている。本計画では、各項目を実装するステップを次に示す。

| ID | ステップ |
|---|---|
| H-01 | 3-2（上限と数え方）・4-1（成功の応答、間隔）・8-6（`silent` の版の記録） |
| H-02 | 3-2・3-4（V1〜V3・M1〜M4） |
| H-03 | 1-1・1-3・2-1・4-2 |
| H-04 | 5-1・6-3 |
| H-05 | 7-2・7-4・7-6・7-8 |
| H-06 | 4-1・4-3 |
| H-07 | 3-1・4-1 |

## 2. 実装ステップ (Implementation Steps)

各フェーズには、壊して失敗することの確認と、完了条件（`make fmt` → `make test` → `make lint`）を行うステップを置く。

### フェーズ 1: `internal/slackwebhook` と `internal/config`

**対象ファイル**
-   新設: `internal/slackwebhook/slackwebhook.go`・`slackwebhook_test.go`
-   変更: `internal/config/config.go`・`config_test.go`、`docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 1-1**: `internal/slackwebhook/slackwebhook.go` に `ValidURL` と `SensitiveParts` を作る（設計書 3.2）。標準ライブラリだけに依存する。ホストが空でないことは `url.URL.Hostname()` で判定する（§1.3）。
-   [x] **ステップ 1-2**: `slackwebhook_test.go` に次を作る。
    -   `TestValidURL`: 受理する値（AC-01 の 2 つの URL、0005 で拒否していた `https://hooks.slack.com.example/services/x`・`https://HOOKS.SLACK.COM/services/x`・`https://hooks.slack.com/`）と、拒否する値（AC-03 の 4 つの値、E4 の 3 つの値、`https://:443/x`）。
    -   `TestSensitiveParts`: 返す部分（URL、パス、query、userinfo のそれぞれの元の形と `net/url` が書き出す形、最後のパスの要素、末尾 8 文字の文字単位とバイト単位）を、ASCII 以外の文字を含む URL と userinfo・query を持つ URL で確かめる。8 バイトに満たない部分（`https://host/a` の `a`、パス `/`）を返さないこと。
-   [x] **ステップ 1-3**: `internal/config/config.go` を次のとおり変える（設計書 3.2・3.9）。
    -   `validSlackWebhook`（doc コメントを含む）を削除し、`loadSlackWebhook` は `slackwebhook.ValidURL` を呼ぶ。
    -   `reasonSlackWebhook` の値を `` `must start with "https://hooks.slack.com/" and contain no whitespace` `` から `"must be an https URL with a host"` に改める。
    -   `loadSlackWebhook` の doc コメントを `// loadSlackWebhook validates SLACK_WEBHOOK_URL. Unset is accepted; a present` / `// value must be a hooks.slack.com URL.` から `// loadSlackWebhook validates SLACK_WEBHOOK_URL. Unset is accepted; a present` / `// value must satisfy slackwebhook.ValidURL.` に改める。
    -   `RequireSlackWebhookURL` と `HTTP2DebugEnabledIn` を足し、`hasHTTP2Debug` は `HTTP2DebugEnabledIn` を呼ぶ。
-   [x] **ステップ 1-4**: `internal/config/config_test.go` を更新する。
    -   `TestLoadInvalid` の `slack webhook` の行（`config_test.go:172-178`）から、新しい規則で受理される 3 つの値を除き、AC-03 の 4 つの値、E4 の 3 つの値、`https://:443/x` を足す。除いた 3 つの値と AC-01 の Mattermost の URL は、受理を確かめるテスト `TestLoadSlackWebhookAccepted`（AC-03a。`Reveal` で元の値が得られること）に置く。
    -   AC-03 の 4 つの値の拒否のエラーの文言に、値もその末尾 8 文字も現れないことを確かめる。`TestLoadInvalid` には値の検査がないので、`TestLoadErrorsOmitValues`（`config_test.go` の目印の値を使うテスト）に、AC-03 の 4 つの形で目印を含む `SLACK_WEBHOOK_URL` の行を足す。
    -   `TestRequireSlackWebhookURL`: 未設定なら `SLACK_WEBHOOK_URL` を名前に持ち `ErrMissing` を包む `*VarError`、設定されていれば同じ値の `secret.Secret` を返すこと。
-   [x] **ステップ 1-5**: `package_reference.md` に `internal/slackwebhook` の行を加え、`internal/config` の行の Webhook URL の規則の記述と公開の関数（`RequireSlackWebhookURL`・`HTTP2DebugEnabledIn`）を改める。
-   [x] **ステップ 1-6**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `ValidURL` のスキームの検査を外す、ホストの検査を外す、ホストの検査を `Hostname()` から `Host` に替える（`https://:443/x` の行）、`url.Parse` の代わりに接頭辞の比較にする（AC-03 の改行の行）、`SensitiveParts` から `net/url` が書き出す形を外す、8 バイトの下限を外す、`RequireSlackWebhookURL` が未設定で nil のエラーを返す。`make fmt` → `make test` → `make lint` を通す。

### PR-1 作成ポイント: shared webhook URL rule and config accessors

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0006): share the webhook URL rule and add the Slack config accessors`

**レビュー観点**: `slackwebhook.ValidURL` が要件書 F-001 の規則（`net/url` で解析でき、スキームが `https`、`Hostname()` が空でない）だけを受理し、E4 で拒否に変わる値も含めて受理と拒否を正しく分けること（ステップ 1-1・1-3） / `SensitiveParts` が URL・パス・query・userinfo（元の形と `net/url` の書き出しの両方）・最後のパスの要素・末尾 8 文字を返し、8 バイト未満の部分を返さないこと（ステップ 1-1） / `RequireSlackWebhookURL` が変数名を持つ `*VarError` を返し、`HTTP2DebugEnabledIn` が `hasHTTP2Debug` の判定の本体になっていること（ステップ 1-3） / `config` の受理と拒否の表と、値も末尾 8 文字も含まないことの検査が新しい規則に一致すること（ステップ 1-4）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-1 の `SensitiveParts` は、URL を元の形と `net/url` の書き出しの両方で照合し、最後のパスの要素と末尾 8 文字を文字単位とバイト単位で返し、8 バイト未満を除くという込み入った境界の判定を持ち、誤ると Webhook URL の一部が伏せ字から漏れるセキュリティの中核である。`ValidURL` の `https` とホストの検査とあわせ、独立した高リスクなステップとしてこの PR に隔離する（Conditional check には該当しない）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `internal/loopbacktest` への移動

**対象ファイル**
-   新設: `internal/loopbacktest/loopbacktest.go`・`loopbacktest_test.go`（いずれも `//go:build test`）
-   変更: `internal/llm/deepseek/test_helpers.go`・`test_helpers_endpoint.go`・`deepseek_test.go`、`internal/pipeline/pipeline_test.go`、`docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 2-1**: `internal/loopbacktest/loopbacktest.go` に `ValidateURL` を作る（設計書 3.3）。本体と 2 つの静的エラーは、`deepseek` の `validateLoopbackEndpoint`・`errTestEndpointNoHost`・`errTestEndpointNotLoopback`（`test_helpers.go:42-73`）を、振る舞いを変えずに移したものとする。
-   [x] **ステップ 2-2**: `loopbacktest_test.go` に `TestValidateURL` を作る。受理の値は、移す前の `TestNewTestClientRejectsNonLoopback`（`deepseek_test.go:166-187`）の一覧をそのまま使う。拒否の値は同じ一覧にホストが空の値 `https:///chat` を足し、各行が期待する静的エラー（`errTestEndpointNoHost` か `errTestEndpointNotLoopback`）を `errors.Is` で確かめる（移す前のテストは `err != nil` だけを見ており、ホストが空の値も無かったので、ホストの検査の行を外しても入れ替えても失敗しなかった）。移した後、`deepseek_test.go` の `TestNewTestClientRejectsNonLoopback` を削除する。
-   [x] **ステップ 2-3**: `internal/llm/deepseek` のテスト用の補助を変える。
    -   [x] `test_helpers.go` の `validateLoopbackEndpoint` を削除する。
    -   [x] `test_helpers.go` の `errTestEndpointNoHost` を削除する。
    -   [x] `test_helpers.go` の `errTestEndpointNotLoopback` を削除する（`var` のブロックが空になれば、ブロックと「Static errors of the test helpers themselves.」のコメントも消す）。
    -   [x] `test_helpers_endpoint.go:18` の呼び出しを `loopbacktest.ValidateURL(endpoint)` に替える。使われなくなった import を消す。
-   [x] **ステップ 2-4**: `internal/pipeline/pipeline_test.go` の `TestPackageReferenceListsPackages` を、`_test.go` 以外の `.go` を持つすべてのパッケージ（テスト用のビルドのファイルだけを持つものを含む）に行を求めるように改め、doc コメントも同じ内容に直す。
    -   使われなくなる `isTestOnlySource` を削除する。
    -   `TestFakesCarryBuildTag` の対象に、`internal/loopbacktest` の `_test.go` 以外の `.go` を加え、1 行目が `//go:build test` であることを確かめる（`isTestOnlySource` を消すと、`loopbacktest.go` のビルドタグを確かめるものがなくなるため）。doc コメントも直す。
    -   `package_reference.md` に `internal/loopbacktest` の行（テスト用のビルドだけで使える、ループバックの URL の判定。`deepseek` のテスト用の構築が使う。`publisher` の分はフェーズ 4 で足す）を加える。
-   [x] **ステップ 2-5**: テストの移動と削除の確認。移動の前後で `go test -tags test -coverprofile` と `go tool cover -func` を `internal/llm/deepseek` と `internal/loopbacktest` に対して実行し、移した関数の既存の行の網羅率が移動の前後で同じであること（`https:///chat` を足した分だけ `ValidateURL` は上がる）、`deepseek` の他の関数の網羅率が変わらないことを確かめ、コミットメッセージに書く（[CLAUDE.md](../../../CLAUDE.md)「Deleting a test is a claim that must be checked」）。
-   [x] **ステップ 2-6**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `ValidateURL` がループバックでない IP を受理する（`TestValidateURL` と `TestNewForLoopbackTestRejectsNonLoopback`）、ホストの検査を外す、`TestPackageReferenceListsPackages` の確認を、`package_reference.md` から `internal/loopbacktest` の行を消して失敗させる、`loopbacktest.go` の 1 行目を消す（`TestFakesCarryBuildTag`）。`make fmt` → `make test` → `make lint` を通す。

### PR-2 作成ポイント: loopback URL check extraction

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `refactor(0006): extract the loopback URL check into internal/loopbacktest`

**レビュー観点**: `ValidateURL` が `deepseek` の `validateLoopbackEndpoint` とその静的エラーを振る舞いを変えずに移したもので、ループバック以外を拒否すること（ステップ 2-1〜2-3） / `deepseek` の既存のテスト（`TestNewForLoopbackTestRejectsNonLoopback`）を変えずに通し、移した関数の既存の行の網羅率が移動の前後で同じであること（ステップ 2-2・2-5） / `TestPackageReferenceListsPackages` と `TestFakesCarryBuildTag` がテスト用のビルドだけのパッケージも数え、`internal/loopbacktest` の行と 1 行目を固定すること（ステップ 2-4） / 移動で `deepseek` の本番の振る舞いが変わらないこと（ステップ 2-3・2-5）

**実装モデル要件**: standard

**判定理由**: 既存のテスト用補助の移動と `pipeline_test.go` のガードの追従に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・2 つ以上の Conditional check のいずれにも該当しないため（該当する Conditional check はビルドタグ下の非 `_test.go` のソースの 1 つだけである）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `SlackWebhookPublisher` の準備の段階とエラー型

**対象ファイル**
-   新設: `internal/publisher/slack_errors.go`・`slack_message.go`・`slack_errors_test.go`・`slack_message_test.go`
-   変更: `internal/publisher/file.go`（コメントだけ）

**タスク**
-   [x] **ステップ 3-1**: `slack_errors.go` に、番兵（`ErrSlackHTTPStatus`・`ErrSlackInvalidResponse`・`ErrSlackTransport`・`ErrSlackUnsplittable`・`ErrSlackMention`）、`SlackHTTPStatusError`、`SlackPostError` と、それぞれの `Error()`・`Unwrap()` を作る（設計書 4.1・4.2）。`SlackPostError.Error()` の 2 つの形と、識別子がない場合の固定の文言は、設計書 4.2 に従う。
-   [x] **ステップ 3-2**: `slack_message.go` に、準備の段階を作る（設計書 3.4・3.5）。
    -   上限値（1 つのメッセージの 16,383 コードポイント、分割の数の上限 10）をパッケージの非公開の定数にする。
    -   非公開の準備の関数: `CheckPublishable` → 4 つのフィールドの UTF-8 の確認（`writer.ErrInvalidArticle` を包む）→ `renderArticle` → 分割 → 投稿する文字列の全体への M1〜M4 の判定、の順に行い、送るメッセージのテキストの列を返す。
    -   V1〜V3 の変換は V2 → V3 → V1 の順に行い、判定のためだけに使う。拒否のエラーは、当たった規則と、投稿する文字列での行と列の番号だけを含める。
    -   `SlackMessageCount` は準備の関数を呼び、メッセージの数を返す。
-   [x] **ステップ 3-3**: `file.go` の `renderArticle` の doc コメントを `// renderArticle returns the file content; it ends with the article body.` から `// renderArticle returns the text FilePublisher and SlackWebhookPublisher both output for a; it ends with the article body.` に改める。関数の内容は変えない。
-   [x] **ステップ 3-4**: `slack_message_test.go` に次を作る（`package publisher`）。準備の関数を直接呼ぶ。
    -   `TestSlackSplit`: ちょうど 16,383 コードポイントで 1 つ、16,384 で 2 つ（日本語・U+10000 以上の文字を含む組み合わせ）。空白文字だけの断片を避ける改行の選択、改行のない行のコードポイントの境界での分割、各メッセージが 16,383 コードポイント以内で正しい UTF-8 であること、分割の位置の表示を除いて連結すると投稿する文字列と一致すること、分割しない場合に表示がないこと。
    -   `TestSlackSplitRejects`: 分割の数が 10 を超える記事と、上限を超える空白文字の連なりを含む記事が `ErrSlackUnsplittable` を包むこと。
    -   `TestSlackMentionRejected`: 次の記事のエラーが `ErrSlackMention` を包み、`writer.ErrInvalidArticle` を包まないこと。
        -   AC-21 の 11 種類の記法のそれぞれを `Title` と `Body` に置いた記事。
        -   設計書 7.1 の「記法」の項に挙げた形: コードスパンの中、`Model` の中、Mattermost の語の分け方で通知になる形、V1〜V3 と M3・M4 の各例、V2 → V3 → V1 の順でなければ見逃す並び、`<a|b|c>`。
        -   コードポイントの境界での分割で、`@` が断片の先頭に来る記事。
    -   `TestSlackMentionAccepted`: 拒否しない並び（自動リンク、`writer` が付ける出典のリンクの形、`~town-square`）を含む記事が拒否されないこと。
    -   `TestSlackMentionMessagesFollowPosted`: 記法に近い並び（`\!channel>`・`here`・`#64;here`・`lt;x`・`x|y>`・U+200B の後の `!channel>`）が断片の先頭に来る記事を分割し、境界がその位置にあることを確かめたうえで、全体の判定を通った記事のどのメッセージにも M1〜M4 が見つからないこと（設計書 3.4 の、各メッセージを判定しない前提）。
    -   `TestSlackPrepareRejectsInvalidArticle`: AC-05 の例（`Title` が空、`Body` が空白文字だけ、`Body` が ESC を含む、`SourceURL` が空）と、`Title`・`Body`・`Model`・`ModelVersion` のそれぞれに不正な UTF-8 を含む記事が `writer.ErrInvalidArticle` を包むこと。不正な UTF-8 の行では、まず `CheckPublishable` がその記事を受理することを確かめ、UTF-8 の確認だけが拒否していることを示す（[CLAUDE.md](../../../CLAUDE.md)「A layered path needs inputs only one layer can handle」）。
    -   日本語などの ASCII 以外の文字を Go のテストのソースに書くときは、`\u`・`\U` のエスケープで書く（既存の `cmd/yt2column/output_test.go`・`internal/strictjson/strictjson_test.go` と同じ）。フェーズ 4・7 のテストも同じ。
    -   `TestSlackPrepareErrorsOmitContent`: 準備の段階の各拒否のエラーの文言に、記事に置いた目印の文字列が現れないこと。
    -   `TestSlackMessageCount`: 準備の関数が返すメッセージの数と一致し、準備の拒否と同じエラーを返すこと。
-   [x] **ステップ 3-5**: `slack_errors_test.go` に次を作る。
    -   `TestSlackPostErrorMessage`: `Attempted` の真偽ごとの文言が、N と k、k+1 番目を示し、偽のときだけ「送っていない」と言うこと。`errors.AsType[*SlackPostError]` で `Total`・`Posted` を取り出せること。
    -   `TestSlackHTTPStatusErrorMessage`: ステータスコード、`Reason`・`RequestID` があるときはそれら、ないときは固定の文言を含むこと。`errors.Is(err, ErrSlackHTTPStatus)` が真であること。
-   [x] **ステップ 3-6**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: 上限の比較を `<` と `<=` で入れ替える、コードポイントでなくバイトで数える、改行の候補から「前側の断片が空白文字だけにならない」条件を外す、分割の数の上限の検査を外す、V1〜V3 の順を V1 → V2 → V3 にする、V1〜V3 のそれぞれを外す、M1〜M4 のそれぞれを外す、分割の位置の表示に `<`・`@`・`&` のいずれかを加える、UTF-8 の確認を外す、`SlackPostError` の文言の `Attempted` の分岐を入れ替える。`make fmt` → `make test` → `make lint` を通す。

### PR-3 作成ポイント: publisher preparation (splitting and mention rejection) and error types

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6

**推奨タイトル**: `feat(0006): add the SlackWebhookPublisher preparation and error types`

**レビュー観点**: V1〜V3 を V2 → V3 → V1 の順に当てた検査用の文字列で M1・M2 を、投稿する文字列で M3・M4 を判定し、投稿する文字列の全体で拒否すること。各メッセージを判定しない前提（各メッセージで見つかる並びは全体でも見つかる）がテストで固定されていること（ステップ 3-2・3-4） / 分割がコードポイントで数え、16,383 以内で改行を優先し、空白文字だけの断片を生じさせず、分割の数の上限を守ること（ステップ 3-2・3-4） / 拒否と分割のエラーが `ErrSlackMention`・`ErrSlackUnsplittable`・`writer.ErrInvalidArticle` を正しく分け、記事の内容を含めないこと（ステップ 3-1・3-2） / 不正な UTF-8 の拒否が `writer` ではなく `SlackWebhookPublisher` 側にあり、`FilePublisher` の振る舞いを変えないこと（ステップ 3-2）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-2 の V1〜V3・M1〜M4 によるメンションの拒否と分割は、誤ると通知の抑止が破れるセキュリティの中核を含む独立した高リスクかつ込み入ったステップであり、この PR に隔離するため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: `SlackWebhookPublisher` の送信とテスト用の構築

**対象ファイル**
-   新設: `internal/publisher/slack.go`・`test_helpers_slack.go`（`//go:build test`）・`slack_test.go`
-   変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 4-1**: `slack.go` に、`SlackPostTimeout`、`SlackWebhookPublisher`、`NewSlackWebhookPublisher`、`Publish` を作る（設計書 3.3・3.6・3.7・4.2）。
    -   構築の拒否、ゼロ値の構造体の `Publish`、URL を取り出せない場合、`http.NewRequestWithContext` の失敗は、値を含まない非公開の固定のエラーにする。
    -   `http.Client` の `CheckRedirect` は `http.ErrUseLastResponse` を返す。タイムアウトは `context.WithTimeoutCause` で与え、`http.Client.Timeout` は使わない。
    -   本文は 8,193 バイトまで読み、`200` は 8,192 バイト以内でちょうど `ok` のときだけ成功とする。`200` 以外の本文からの識別子の取り出しは `strictjson.ParseObject` を使い、設計書 4.2 の形と `slackwebhook.SensitiveParts` による除外に従う。
    -   通信のエラーは `*url.Error` の内側のエラーの文言だけを `%w` で包まずに使い、`SensitiveParts` のいずれかを含めば固定の文言（設計書 4.2）に替える。タイムアウト・キャンセルは、その `context` の `Err()` を `%w` で包む。
    -   2 つ目以降のメッセージの前に 1 秒待ち、各メッセージの直前に `ctx` を確かめ、最後のメッセージの成功の後は `ctx` を確かめない。準備の後の失敗はすべて `*SlackPostError` で包む。
-   [x] **ステップ 4-2**: `test_helpers_slack.go` に `SlackTestOptions` と `NewSlackWebhookPublisherForLoopbackTest` を作る（設計書 3.3）。送信先は `loopbacktest.ValidateURL` で確かめ、ループバックでない送信先と正でない `Timeout` は `t` を失敗させる。`ValidateURL` のエラーは送信先の URL を含む（`deepseek` から振る舞いを変えずに移すため）。送信先は Webhook URL のパス（ステップ 6-6）を含みうるので、失敗のメッセージにはエラーの文言を含めず、固定の文言にする。同じファイルに、次の 2 つを置く。
    -   `http.RoundTripper` を引数に取り、その `Transport` を持つ値を構築する非公開の補助（ステップ 4-3 の通信のエラーの文言の差し替えのテストが使う）。送信先は同じくループバックに限る。構築した後の値は書き換えない。
    -   準備の結果のメッセージのテキストの列を返す、`test` のタグだけの公開の関数（ステップ 7-5 の記事の長さを、外部のテストのパッケージから確かめるため）。
-   [x] **ステップ 4-3**: `slack_test.go` に次を作る（`package publisher`、`httptest` のサーバ。サーバは受け取ったリクエストを記録し、テストごとに応答を決める）。
    -   **サーバの後始末。** サーバ（リダイレクト先のサーバを含む）を作った時点で `t.Cleanup(server.Close)` を登録する。応答しないハンドラは `r.Context().Done()` か、`server.Close` より後に登録した `t.Cleanup` で閉じるチャネルを待って戻る（`httptest.Server.Close` はハンドラが戻るまで待つので、戻らないハンドラは後始末を止める）。
    -   **キャンセルの順序。** キャンセルの時点は、サーバが応答を返したことや待機に入ったことを印（チャネル）で知らせ、テストがそれを待ってからキャンセルする形で決め、固定の `sleep` を使わない。応答の読み取りと競合させずに「送信の直前の確認」でだけ中断させる行は、`cmd/yt2column/run_test.go` の `expiringContext` と同じく、`Done` を閉じずに `Err()` だけを変える `context` を使う。待機中のキャンセルの行は、`SlackTestOptions.Interval` を長くする。
    -   `TestNewSlackWebhookPublisher`（AC-01）: AC-01 の 2 つの URL で構築できること。テスト用の構築で作っただけでは、サーバがリクエストを受け取らないこと。
    -   `TestNewSlackWebhookPublisherRejects`（AC-02・AC-03）: ゼロ値の `secret.Secret` と AC-03 の 4 つの値の拒否。エラーの文言に、値もその末尾 8 文字も現れないこと。
    -   `TestNewSlackWebhookPublisherForLoopbackTestRejects`: ループバックでない送信先と、正でない `Timeout` で `t` が失敗し、記録した失敗のメッセージに送信先のパスの目印が現れないこと（`Fatalf` を記録する `testing.TB` をこのテストのファイルに作る）。
    -   `TestSlackPublishPayload`（AC-04・AC-06・AC-07・AC-21a）: メソッド、`Content-Type`、JSON のメンバーが `text` と `silent` だけであること（`blocks`・`attachments` がない）、すべてのペイロードで `silent` が `true` であること、AC-06 の文字を含む記事と `ModelVersion` が空の記事で `text` が投稿する文字列と一致すること、1 つのメッセージの場合に分割の位置の表示がないこと。
    -   `TestSlackPublishSplit`（AC-08・AC-09・AC-10）: 上限の 2 倍を超える複数行の記事で 3 つ以上のリクエストが順に届くこと、改行のない長い行の記事、ちょうど上限と上限 + 1 の記事。各リクエストの `text` が 16,383 コードポイント以内で正しい UTF-8 であり、分割の位置の表示を含み、表示を除いて連結すると投稿する文字列と一致すること。受け取ったリクエストの数が `SlackMessageCount` と一致すること。
    -   `TestSlackPublishPrepareSendsNothing`（AC-05・AC-11・AC-21）: 準備の段階の各拒否で、サーバがリクエストを受け取らず、エラーが `*SlackPostError` を包まないこと。
    -   `TestSlackPublishResponses`（AC-12・AC-13・AC-14・AC-20）: 2 つに分割される記事の 1 つ目に 400・403・404・410・429・500 を返すと `ErrSlackHTTPStatus` になり、文言がステータスコードを含み、2 つ目が送られないこと。`200` で本文が空・`OK`・`ok\n`・`{"ok":true}`・8,192 バイトの `ok` 以外・8,193 バイトのとき `ErrSlackInvalidResponse` になること。`200` 以外の本文の識別子の表（`AppError` の JSON の `id`・`request_id`、識別子の形でない `id`、平文の識別子、識別子の形でない本文、Webhook URL の一部と一致する `id`・`request_id`）。AC-20 の行は、ステータス 400 と `web.incoming_webhook.parse.app_error` の `id` で、文言がその両方を含むこと。
    -   `TestSlackPublishRedirect`（AC-15）: 301・302・307・308 で、`Location` に指定した別のサーバがリクエストを受け取らず、`ErrSlackHTTPStatus` になること。
    -   `TestSlackPublishTimeout`（AC-16）: 応答しないサーバと短い `Timeout` で、有限の時間で `context.DeadlineExceeded` を包む `*SlackPostError` が返ること。
    -   `TestSlackPublishCanceled`（AC-17）: キャンセル済みの `ctx` でリクエストが届かず `context.Canceled` を包み `Posted` が 0 であること。3 つに分割される記事の 1 つ目の成功の後にキャンセルすると、2 つ目以降が届かず、`Total` = 3・`Posted` = 1 であること。メッセージの間の待機中のキャンセルで `Attempted` が偽であること。最後のメッセージの成功の後のキャンセルでは `nil` が返ること。
    -   `TestSlackPublishPartialFailure`（AC-18）: 3 つに分割される記事の 2 つ目に 500 を返すと、`Total` = 3・`Posted` = 1・`Attempted` が真で、3 つ目が届かず、文言が 3 つのうち 1 つと 2 つ目の失敗を示すこと。
    -   `TestSlackPublishErrorsOmitWebhookURL`（AC-19）: パスに目印を含む URL で、AC-19 の 6 つの失敗（閉じたサーバ、タイムアウト、キャンセル、`200` 以外、上限を超える本文、3xx）を起こし、返したエラーから `errors.Unwrap`（単一と複数の両方）でたどれるすべてのエラーの `Error()` に、URL、パス、末尾 8 文字が現れないこと。応答の本文に URL のパスを返すサーバの行、構造体の `%+v`・`%#v` の行を含める。
    -   `TestSlackPublishTransportErrorWithheld`: ステップ 4-2 の補助で、URL のパスを文言に含むエラーを返す `Transport` を持つ値を構築すると、エラーの文言が固定の文言になり、パスが現れないこと。
    -   `TestSlackPublishZeroValue`: ゼロ値の構造体の `Publish` がリクエストを送らずに失敗すること。
    -   `TestSlackPublishErrorClasses`（AC-35）: HTTP ステータスの失敗・不正な応答・通信の失敗・分割の拒否・記法の拒否・`writer.ErrInvalidArticle` のそれぞれが、6 つの分類のうち自分の分類だけに `errors.Is`／`errors.AsType` で当たること。
-   [x] **ステップ 4-4**: `package_reference.md` の `internal/publisher` の行に `SlackWebhookPublisher`・`SlackMessageCount`・テスト用の構築と、ステップ 4-2 の準備の結果を返す関数（`test` のタグだけ）を加える。
-   [x] **ステップ 4-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `CheckRedirect` を外す、本文の読み取りを上限ちょうどにする（8,193 でなく 8,192）、`ok` の比較を前後の空白を除いたものにする、`*url.Error` をそのまま包む、通信のエラーの文言の差し替えを外す、識別子の `SensitiveParts` による除外を外す、`http.Client.Timeout` に替える、テスト用の構築の失敗のメッセージに `ValidateURL` のエラーの文言を含める、送信の前の `ctx` の確認を外す、最後のメッセージの後に `ctx` を確かめる、`silent` を外す、`Posted` を数え違える（失敗したメッセージを含める）、通信の失敗の原因を `%w` で包む（`TestSlackPublishErrorClasses`）。`make fmt` → `make test` → `make lint` を通す。

### PR-4 作成ポイント: publisher send path and loopback test construction

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0006): add the SlackWebhookPublisher send path`

**レビュー観点**: `Publish` が準備を終えた後にだけ送り、リダイレクトに従わず、応答の検証とタイムアウト・キャンセルの扱いが設計書 3.6 のとおりであること（ステップ 4-1・4-3） / 準備の後の失敗がすべて `*SlackPostError` で包まれ、`Attempted` と `Posted` が正しく数えられ、各失敗が AC-35 の 1 つの分類だけに当たること（ステップ 4-1・4-3） / エラーと `errors.Unwrap` でたどれるすべてのエラーの文言に、Webhook URL・そのパス・末尾 8 文字が現れず、構造体の `%+v`・`%#v` にも現れないこと（ステップ 4-1・4-3） / `test_helpers_slack.go` がビルドタグ下の非 `_test.go` のソースとして `gosec`・`errcheck` の対象になり、テスト用の構築がループバックに限られること（ステップ 4-2）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-1 の送信と応答の検証は、Webhook URL を出力に漏らさないこと・リダイレクトに従わないことを含むセキュリティの中核であり、ステップ 4-2 はビルドタグ下の非 `_test.go` のソースという Conditional check にも該当するため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: `internal/job` の `Output`

**対象ファイル**
-   変更: `internal/job/job.go`・`job_test.go`・`test_helpers.go`、`cmd/yt2column/run.go`（`job.Request` の組み立てだけ）、`docs/dev/developer_guide/package_reference.md`

**タスク**
-   [x] **ステップ 5-1**: `job.go` に `Output`・`FileOutput`・`RemoteOutput` を作り、`Request` の `OutPath`・`Publisher` を `Output` に置き換える（設計書 3.8）。`validateRequest` と事前確認を `Output` の種類の `switch` にし、`default`（ゼロ値）は `errInvalidRequest` で拒否する。`Run` と `Request` の doc コメントの `OutPath` への言及を、`Output` に合わせて直す。
-   [x] **ステップ 5-2**: `cmd/yt2column/run.go:192-201` の `job.Request` の組み立てを `Output: job.FileOutput(opts.out, pub)` にする（`--slack` はフェーズ 6）。
-   [x] **ステップ 5-3**: `job_test.go` を更新する。
    -   複合リテラルの `OutPath: <x>, … Publisher: <p>` の形のすべての箇所を `Output: FileOutput(<x>, <p>)` に、`req.OutPath, req.Publisher = <x>, <p>` の形（`job_test.go:216`・`:224`）を `req.Output = FileOutput(<x>, <p>)` に書き換える。`internal/job/test_helpers.go` の `newOutput` は、呼び出し側が書きやすい形に変えてよい（設計書 3.14）。書き換えの後、`rg -n "OutPath" internal/job cmd/yt2column/run.go` が 0 件であることを確かめる。
    -   `TestRunValidatesRequest` の `empty out path` の行を `empty file path` とし `FileOutput("", pub)` に、`nil publisher`・`typed-nil publisher` の行を `FileOutput` の `Publisher` が nil の行にする。ゼロ値の `Output`、`RemoteOutput(nil)`、型付きの nil を渡した `RemoteOutput` の行を足す。
    -   `TestRunRemoteOutput`: `RemoteOutput` で、出力のパスなしに `Run` が成功し、`Publisher` が 1 回呼ばれ、キャッシュの削除までの手順が `FileOutput` と同じく行われること。
-   [x] **ステップ 5-4**: `package_reference.md` の `internal/job` の行の `--out` の事前確認の記述を、`Output`（ファイルの場合だけ事前確認する）に改める。
-   [x] **ステップ 5-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `validateRequest` の `default` を受理にする（ゼロ値の行）、リモートでも出力のパスを必須にする（`TestRunRemoteOutput`）、ファイルの事前確認を外す（既存の事前確認のテスト）。`make fmt` → `make test` → `make lint` を通す。

### PR-5 作成ポイント: job.Output typing

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5

**推奨タイトル**: `feat(0006): type the job output as file or remote`

**レビュー観点**: `Output` のゼロ値・空のパスの `FileOutput`・`RemoteOutput(nil)`・型付き nil を `validateRequest` が拒否し、事前確認がファイルの場合だけ行われること（ステップ 5-1・5-3） / `OutPath`・`Publisher` の参照がすべて `Output` に置き換わり、`rg -n "OutPath" internal/job cmd/yt2column/run.go` が 0 件であること（ステップ 5-1〜5-3） / `RemoteOutput` で出力のパスなしに `Run` が成功し、キャッシュの削除までの手順が `FileOutput` と同じであること（ステップ 5-3） / `cmd/yt2column/run.go` の `job.Request` の組み立てだけを変え、フェーズ 6 の `--slack` を先取りしないこと（ステップ 5-2）

**実装モデル要件**: standard

**判定理由**: 投稿先の型付けと `switch` による検証に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・2 つ以上の Conditional check のいずれにも該当しないため（ビルドタグ下の非 `_test.go` のソースは、ステップ 5-3 で変えうる既存の `internal/job/test_helpers.go` だけで、新設はない）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: `cmd/yt2column`

**対象ファイル**
-   変更: `cmd/yt2column/run.go`・`run_test.go`・`test_helpers.go`・`signal_test.go`、`README.md`（フラグの表とその直前の文だけ）、`docs/dev/developer_guide/package_reference.md`

**タスク**
-   [ ] **ステップ 6-1**: `deps` の `newPublisher` を `newFilePublisher` と `newSlackPublisher` に分け、`productionDeps` の `newSlackPublisher` に、`publisher.NewSlackWebhookPublisher` を包み構築の失敗で nil のインターフェースを返す関数を設定する（設計書 3.10）。`run_test.go` の `e.d.newPublisher` のすべての差し替え（`run_test.go:474`・`:547`・`:647`・`:881`）を `e.d.newFilePublisher` に改める。
-   [ ] **ステップ 6-2**: フラグと使い方を変える（設計書 3.10）。
    -   `cliOptions` に `slack bool` を足し、`newFlagSet` に `--slack` を加える。説明の文は設計書 3.10 のとおりとする。
    -   `--out` の説明を `` "write the article to this `path` (required); it must not exist, its directory must exist, and it must be outside the cache directory" `` から `` "write the article to this `path`; it must not exist, its directory must exist, and it must be outside the cache directory" `` に改める。
    -   `writeUsage` の `"Generates a column article from a YouTube video's subtitles and writes it to the --out file.\n"` を、`--out` のファイルに書くか `--slack` で Webhook に投稿するかのどちらか一方を指定することを示す文に改める（文言は実装で決める）。
    -   `README.md` のフラグの表に `--slack` の行を加える。`TestREADMEDocumentsCLI` の `flags` が `newFlagSet` のすべてのフラグを README の表に求めるので、この行はフラグを加えるこのフェーズで必要である。同じ変更で、`--out` が必須でなくなったことに合わせ、フラグの表の `--out` の行の「Required.」と、表の直前の文（`README.md:32`）の「`--out` is required」を、`--out` と `--slack` のどちらか一方を指定する記述に改める（PR-6 の時点で README がフラグの説明と矛盾しないようにするため。説明の文の推敲と他の文書はフェーズ 8）。
-   [ ] **ステップ 6-3**: 手順 A2〜C を設計書 3.10 の「投稿先の決定」と「手順の変更」の表のとおりに変える。
    -   投稿先の決定は、`flag.FlagSet.Visit` でフラグが現れたかを調べる別の関数にし、設計書 3.10 の表の拒否の順と文言に従う。`run.go:150-152` の `opts.out == ""` の判定を置き換える。
    -   A3 の `RequireSlackWebhookURL`、A3 の Webhook URL の警告（文言は設計書 3.10）、A4 をファイルの場合だけ行うこと、A5 の `newSlackPublisher`、B の `job.RemoteOutput`、C の要約（`SlackMessageCount` がエラーなら `unknown`）を加える。要約は別の関数にする（`run` の循環的複雑度を 20 以下に保つため。§1.3）。
-   [ ] **ステップ 6-4**: `reportRunError` に、設計書 3.10 の「失敗の表示に足すもの」の 4 つの案内を加える。Webhook のタイムアウトの案内は `minutes` を使わず、`SlackPostTimeout` を秒で書く（§1.3）。
-   [ ] **ステップ 6-5**: `configuredSecrets` が、`SLACK_WEBHOOK_URL` が設定されていれば `slackwebhook.SensitiveParts` の各文字列も返すようにする（設計書 3.10）。
-   [ ] **ステップ 6-6**: テストの組み立てを変える（設計書 3.10）。
    -   `test_helpers.go` に、何も送らずに固有の静的エラーを返す `newSlackPublisher` の代用品を作り、`testDeps` と `newRunEnv`（`run_test.go:95-129`）の両方の既定にする。代用品が使われたことは、そのエラーの文言が標準エラー出力の `build the publisher` の行に現れることで見分けられるようにする。
    -   `test_helpers.go` に、ループバックの `httptest` のサーバへ送る `newSlackPublisher` を返す補助を作る（`NewSlackWebhookPublisherForLoopbackTest` を使う）。送信先は、`server.URL` に、`run` が受け取る `SLACK_WEBHOOK_URL` のパス（`testWebhook` の目印を含むパス）を付けたものにする。こうすると、投稿の失敗が送信先の URL の一部を出力した場合に、CLI の伏せ字化（`configuredSecrets`）と `secretStrings` の検査の対象になる。プロセス内の Webhook の経路のテストはこれに差し替える。
    -   子プロセスのモードに、すぐに記事を返す偽の LLM と、待機の状態になったことを `childReadyEnv` の印で知らせ、`ctx` が終わると `&publisher.SlackPostError{Total: 1, Posted: 0, Attempted: true, Err: ctx.Err()}` を返す偽の `Publisher` を使うモードを足す。
-   [ ] **ステップ 6-7**: `run_test.go` を更新し、テストを足す。
    -   `no --out` の行（`run_test.go:357`）の期待する文言を `--out is required` から `one of --out or --slack is required` に改める。
    -   **`executionPathRows` への行の追加（AC-23・AC-24・AC-25・AC-26）。** 次の `--slack` の経路を行として足し、`TestRunExecutionPaths` の 0005 の AC-44 の各項目の検査を適用する。成功の行の既定の検査（`--out` のファイルの存在）は、Webhook の行ではループバックのサーバが受け取ったメッセージの検査に替える（行の `check` か、投稿先で分ける形。実装で決める）。
        -   成功（終了コード `0`、標準出力が空、標準エラー出力に投稿したこととメッセージの数、キャッシュの削除）と、`--keep-cache` での成功（キャッシュが残る）。
        -   設計書 3.10 の表の拒否の 4 つの行と、拒否の順の例（`--out "" --slack`、`--out x --slack=false`）、`--slack` で `SLACK_WEBHOOK_URL` が未設定の行（標準エラー出力に名前が現れる）。いずれも終了コード `2` で、排他のファイル・キャッシュ・トリップワイヤ・LLM の呼び出し・ループバックのサーバへのリクエストがないこと。
        -   最初のメッセージの失敗（500。終了コード `1`、キャッシュが残る、残るメッセージの案内がない）。
        -   分割投稿の途中の失敗（3 つに分割される記事の 2 つ目に失敗を返す。終了コード `1`、投稿の段階の失敗・3 つのうち 1 つ・再実行の案内、キャッシュが残る）。
        -   準備の段階の拒否（記法を含む記事。終了コード `1`、記法の拒否の案内、リクエストがない、キャッシュが残る）。
    -   `TestRunSlackFailureHints`（AC-25）: 偽の `Publisher` が返すエラーごとに、`Attempted` の真偽ごとの文言、400・403・404・501 の案内と request ID の有無、500 で案内が出ないこと、タイムアウトの案内。
    -   `TestRunSlackSummaryUnknownCount`: 成功する `FakePublisher` と記法を含む記事で、要約の数が `unknown` で終了コード `0` であること。
    -   `TestRunSlackSignalAfterPosting`: 投稿が成功した後に `ctx` を取り消す偽の `Publisher` で、終了コード `0` であること（設計書 3.10 の実行経路の一覧の最後の行）。
    -   `TestRunGODEBUGWarning`（`run_test.go:924`、AC-28）に行を足す。`GODEBUG` が `http2debug=1`・`http2debug=2` のとき、`--slack` では Webhook URL の警告が LLM の呼び出しより前に現れ、値も末尾 8 文字も含まないこと。`--out` と、`GODEBUG` がない `--slack` では、この警告が出ないこと。
    -   `TestRunEscapesUntrustedText`（`run_test.go:867`、AC-26）に `--slack` の行を足す。要約の `Model`・`ModelVersion` のエスケープと、失敗の応答の本文にエスケープシーケンスと偽の行を始める改行を含むサーバの行である。応答の本文は識別子の形の値しかエラーに入らない（設計書 4.2）ので、後者では、標準エラー出力に生の ESC・偽の行・本文の文字列が現れないことを確かめる。
    -   `TestConfiguredSecretsIncludesWebhookParts`（AC-26）: パスだけの断片を含む行が伏せられ、8 バイトに満たない部分が伏せる対象に入らないこと。
    -   `TestRunHelp`（`run_test.go:814`、AC-27）で、`-h`・`--help` の出力が `--slack` と `Mattermost` を含むことを確かめる。
    -   `TestTestDepsSlackPublisherDoesNotSend`: `testDeps` と `newRunEnv` の既定の `newSlackPublisher` で `--slack` を実行すると、終了コード `2` で、標準エラー出力の `build the publisher` の行に代用品のエラーの文言が現れること。
    -   `TestNewSlackPublisherNilOnFailure`: `productionDeps` の `newSlackPublisher` が、ゼロ値の `secret.Secret` で nil のインターフェース（型付きの nil でない）とエラーを返すこと（`TestNewFilePublisherNilOnFailure` と同じ形）。
-   [ ] **ステップ 6-8**: `signal_test.go` に `TestSignalDuringSlackPost`（AC-26）を足す。ステップ 6-6 の子プロセスのモードで `--slack` で実行し、待機の印を待って SIGINT と SIGTERM のそれぞれを送ると、終了コード `1` で、0005 の AC-44 の各項目が成り立つこと。(f) のうち `--out` のパスの条件は適用しないが、動画のキャッシュが削除されないことは確かめる。`requireInterruptedChildOutput` が `--out` のパスを前提にする部分は、`--slack` の経路では外せる形に変える。
-   [ ] **ステップ 6-9**: `package_reference.md` の `cmd/yt2column` の行に、`--out` と `--slack` のどちらか一方を選ぶこと、`SLACK_WEBHOOK_URL` の部分も伏せることを加える。
-   [ ] **ステップ 6-10**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: 投稿先の決定を値で判定する（`--slack=false` の行）、拒否の順を入れ替える、`RequireSlackWebhookURL` の呼び出しを外す、Webhook の警告を LLM の呼び出しの後に移す、`--out` でも Webhook の警告を出す、`configuredSecrets` から `SensitiveParts` を外し、偽の `Publisher` のエラーの文言に送信先の URL のパスを含める（`executionPathRows` の失敗の行と `requireSafeOutput`）、`Posted` が 0 でも残るメッセージの案内を出す、`testDeps` と `newRunEnv` のそれぞれの既定を `productionDeps` の値に戻す（`TestTestDepsSlackPublisherDoesNotSend`）、`newSlackPublisher` の包みが型付きの nil を返す、子プロセスで `ctx` の終了を待たない、取り消しでキャッシュを削除する（`TestSignalDuringSlackPost`）。`make fmt` → `make test` → `make lint` を通す。

### PR-6 作成ポイント: cmd/yt2column --slack wiring

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10

**推奨タイトル**: `feat(0006): add the --slack output path to the CLI`

**レビュー観点**: 投稿先の決定が `flag.FlagSet.Visit` による「フラグが現れたか」で行われ、設計書 3.10 の表の拒否の順と文言のとおりで、README のフラグの表に `--slack` の行が加わって `TestREADMEDocumentsCLI` が通り、`--out` を必須とする記述が残らないこと（ステップ 6-2・6-3） / `SLACK_WEBHOOK_URL` の未設定・`http2debug` の警告・`configuredSecrets` の `SensitiveParts` の追加が、副作用より前の正しい位置にあること（ステップ 6-3〜6-5） / `testDeps` と `newRunEnv` の既定の `newSlackPublisher` が送らずに失敗し、ループバックの送信先を使う差し替えと子プロセスのモードが正しく動くこと（ステップ 6-6・6-7） / 投稿中の SIGINT・SIGTERM が別プロセスで終了コード `1` と AC-44 の各項目を満たし、投稿の成功後に受けたシグナルが終了コード `0` になること（ステップ 6-8）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 6-6・6-8 は本番の CLI を別プロセスで起動して実シグナルを送り、OS のシグナルのタイミングと子プロセスの回収を扱う重いテストの面を持ち、`mkplan.md` のパネルモードのトリガー（重い統合テストの面）に該当するため（0005 の PR-10 と同じ判断）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: 統合テストと `make` のターゲット

**対象ファイル**
-   新設: `internal/publisher/testutil/integration.go`（`//go:build test || integration`）・`integration_settings_test.go`（`//go:build test`）、`internal/publisher/slack_integration_test.go`（`//go:build integration`、`package publisher_test`）・`slack_articles_test.go`・`makefile_test.go`（いずれも `//go:build test`、`package publisher_test`）、`cmd/yt2column/integration_slack_test.go`（`//go:build integration`）
-   変更: `internal/llm/deepseek/testutil/make.go`・`make_test.go`、`cmd/yt2column/integration_test.go`・`test_helpers_integration.go`・`makefile_test.go`、`internal/pipeline/pipeline_test.go`、`Makefile`、`docs/dev/developer_guide/package_reference.md`

**タスク**
-   [ ] **ステップ 7-1**: `RunMakeTarget` に、記録する変数の名前を呼び出し元が追加する可変長の引数を足す（設計書 3.12）。追加の名前も `validateEnvNames` で確かめ、`recordedEnv` と重なる名前は 1 回だけ記録する。`CheckChargedTarget` は `c.OptInEnv` を追加の名前として渡す（§1.3）。既存の 3 つの呼び出しは引数を変えない。`make_test.go` に、追加の名前が記録されることと、シェルの変数名でない追加の名前が拒否されることの行を足す。
-   [ ] **ステップ 7-2**: `internal/publisher/testutil/integration.go` に、設計書 3.12 の定数・型・`SettingsFrom` を作る。判定は設計書 3.12 の表の順とし、理由の文言は URL を含まず、スキップの理由はオプトインの変数と `make` のターゲットを示す。同じファイルに次も置く。
    -   2 つの統合テストの `IntegrationOptions` の変数（`SlackIntegrationOptions`・`CLISlackIntegrationOptions`）。統合テストと `make` のターゲットのテストの両方がこれを使い、オプトインの変数とターゲットの名前を 1 か所で決める。
    -   ステップ 7-4 の 2 つの固定の記事を組み立てる関数。分割しない記事は、投稿する文字列がちょうど 16,383 コードポイントになるように組み立てる。どちらの記事も、日本語と U+10000 以上の文字を含み、M1〜M4 に当たらず、`Title` に実行ごとの固定長の目印（実行の日時など）を含めて、テスト用のチャンネルで自分の投稿を見分けられるようにする。
-   [ ] **ステップ 7-3**: `internal/publisher/testutil/integration_settings_test.go` に `TestSlackSettingsFrom`（AC-29）を作る。オプトインが未設定・空・`0`・`true`・` 1` でスキップ、`1` で `YT2COLUMN_TEST_SLACK_WEBHOOK_URL` がない（本番の `SLACK_WEBHOOK_URL` だけがある場合を含む）と失敗、`ValidURL` に合わないと失敗、`GODEBUG` が `http2debug=1`・`http2debug=2` を含むと失敗、それ以外で実行し `WebhookURL` が値を返すこと。実行でない場合は `WebhookURL` がゼロ値であること。どの理由にも URL とその末尾 8 文字が現れないこと。
-   [ ] **ステップ 7-4**: `internal/publisher/slack_integration_test.go` に `TestIntegrationSlackWebhookPublisher`（AC-30）を作る（設計書 3.12）。`publishertestutil.SettingsFrom` と `SlackIntegrationOptions` で判定し、`NewSlackWebhookPublisher` で構築した値に、ステップ 7-2 の 2 つの記事を順に投稿してエラーがないことを確かめる。投稿の前に、`SlackMessageCount` が分割しない記事で 1、分割する記事で 2 以上であることを確かめる。テストの出力に URL を書かない。
-   [ ] **ステップ 7-5**: `internal/publisher/slack_articles_test.go` と `makefile_test.go` に次を作る。
    -   `TestIntegrationArticles`（`slack_articles_test.go`）: ステップ 4-2 の準備の結果を返す関数で、ステップ 7-2 の分割しない記事がちょうど 16,383 コードポイントの 1 つのメッセージになり、分割する記事が 2 つ以上のメッセージになり、どちらも準備で拒否されないこと（統合テストを実行しなくても `make test` で記事の形を確かめる）。
    -   `TestMakeTestIntegrationSlack`（AC-32、`makefile_test.go`）: `RunMakeTarget` に、モデル名の引数として nil を、Webhook の 2 つのオプトインを追加の名前として渡し、`make test-integration-slack` が `go test -tags integration -count=1 -timeout <値> -v ./internal/publisher` を実行し、`SlackIntegrationOptions.OptInEnv` を `1` でエクスポートし、`CLISlackOptInEnv` をエクスポートしないこと。`YT2COLUMN_MODEL`（`deepseektestutil.ModelEnv`）をエクスポートしないこと。出力が実際の Webhook に投稿することを示し、DeepSeek の課金の注意を含まないこと（`CheckChargedTarget` は使わない）。`-timeout` が、2 つの記事のメッセージを `SlackPostTimeout` と間隔で送る最大の時間を超えること。ターゲットの名前は `SlackIntegrationOptions.MakeTarget` から取る。
    -   `TestSlackIntegrationTestBuildTag`（AC-32、`makefile_test.go`）: `slack_integration_test.go` の 1 行目が `//go:build integration` であること（`deepseektestutil.FirstLineIs`）。
-   [ ] **ステップ 7-6**: `cmd/yt2column/test_helpers_integration.go` に、CLI の Webhook の統合テストの判定の補助（`deepseektestutil.SettingsFrom` を `OptInEnv: publishertestutil.CLISlackOptInEnv`・欠けたキーは失敗で呼び、`publishertestutil.SettingsFrom` も `CLISlackIntegrationOptions` で実行と判定した場合だけ本体を呼ぶ）を、`gateCLIIntegration` と同じ形で作る（設計書 3.12）。
-   [ ] **ステップ 7-7**: CLI の Webhook の統合テストを作る（AC-31）。
    -   `cmd/yt2column/integration_test.go` の `runIntegrationCLI`（`:45-129`）から、キャッシュの用意・トリップワイヤ・`productionDeps()` と `generateCounter` の包み・秘密の値の検査を、両方の統合テストが呼ぶ補助として取り出す。`TestIntegrationCLI` の振る舞いは変えない。
    -   `cmd/yt2column/integration_slack_test.go` に `TestIntegrationCLISlack` を作る。取り出した補助で組み立て、`lookupFrom` の環境に `SLACK_WEBHOOK_URL` としてテスト用の Webhook URL を入れ、`--slack` で `run` を 1 回呼ぶ。何かを出力する前に、標準出力・標準エラー出力に、テスト用の Webhook URL・テスト用の API キー・それぞれの末尾 8 文字（文字単位とバイト単位）が現れないことを確かめる。終了コード `0`、トリップワイヤが起動されないこと、`Generate` が 1 回であること、キャッシュのエントリが残らないことを確かめる。
-   [ ] **ステップ 7-8**: `cmd/yt2column/makefile_test.go` を更新する。
    -   `TestMakeTestIntegrationCLISlack`（AC-32）: `CheckChargedTarget` で `CLISlackIntegrationOptions.MakeTarget`（`OptInEnv` は `CLISlackIntegrationOptions.OptInEnv`、`Package` は `./cmd/yt2column`、`MinTimeout` は `provider.LLMTimeout` に Webhook の投稿の最大の時間を足したもの）を確かめる。
    -   `TestMakeOptInsAreTargetSpecific`: 対象のターゲットを `test-integration`・`test-integration-deepseek`・`test-integration-cli`・`test-integration-slack`・`test-integration-cli-slack` の 5 つにし、Webhook の 2 つのオプトインを追加の名前として渡し、各ターゲットが自分以外のオプトインをエクスポートしないことを確かめる。
    -   `TestCLISlackIntegrationSettings`（AC-29）: `TestCLIIntegrationSettings` と同じく `gateRecorder` で、どちらかの判定がスキップか失敗なら本体を呼ばないこと、両方が実行なら呼ぶこと、理由に URL と API キーが現れないこと。
    -   `TestCLISlackIntegrationTestBuildTag`（AC-32）: `integration_slack_test.go` の 1 行目が `//go:build integration` であること。
-   [ ] **ステップ 7-9**: `Makefile` に `test-integration-slack` と `test-integration-cli-slack`、それぞれのタイムアウトの変数を加え、`.PHONY` と `lint` のコメント（`go vet -tags integration` で確かめるターゲットの一覧）を更新する。各ターゲットは自分のオプトインだけを `1` でエクスポートする。`test-integration-cli-slack` は `YT2COLUMN_MODEL` もエクスポートし、`CheckChargedTarget` が求める課金の注意を出力する。`test-integration-slack` は、実際の Webhook に投稿することを出力する。
-   [ ] **ステップ 7-10**: `internal/pipeline/pipeline_test.go` の `TestFakesCarryBuildTag` の `testutil/` のファイルの数を 13 から 15 に改める（ステップ 7-2・7-3 の 2 件）。
-   [ ] **ステップ 7-11**: `package_reference.md` の `internal/publisher/testutil` の行に `SettingsFrom`・統合テストの `IntegrationOptions`・固定の記事と `integration.go` のビルドタグを、`internal/llm/deepseek/testutil` の行に `RunMakeTarget` の追加の名前を加える。
-   [ ] **ステップ 7-12**: 壊して失敗することを確かめ、コミットメッセージに記録する。対象: `SettingsFrom` の判定の 1〜4 のそれぞれを外す、オプトインの比較を空でないかどうかにする、本番の `SLACK_WEBHOOK_URL` を読む、`Makefile` の各ターゲットで他方のオプトインもエクスポートする、`test-integration-slack` の `-tags integration` を外す、統合テストのファイルの 1 行目を `//go:build test` にする、CLI の判定の補助で一方の判定だけを見る、`SlackIntegrationOptions` のオプトインを `CLISlackOptInEnv` にする（`TestMakeTestIntegrationSlack`）、分割しない記事を 1 コードポイント短くする（`TestIntegrationArticles`）、`RunMakeTarget` で追加の名前の検査を外す。`make fmt` → `make test` → `make lint` を通す。
-   [ ] **ステップ 7-13**: 利用者の承認を得て、`make test-integration-slack` と `make test-integration-cli-slack` を Mattermost のテスト用のチャンネルの Webhook で実行し（AC-30・AC-31）、次を本ステップの下に記録する。実行日、HEAD のコミット、サーバの版、各ターゲットの結果（`PASS`／`FAIL` と終了コード）、テスト用のチャンネルに投稿されたメッセージの数と分割の位置の表示（記事の `Title` の目印で見分ける）。

### PR-7 作成ポイント: webhook integration tests and make targets

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6 / 7-7 / 7-8 / 7-9 / 7-10 / 7-11 / 7-12 / 7-13

**推奨タイトル**: `feat(0006): add the webhook integration tests and make targets`

**レビュー観点**: `publishertestutil.SettingsFrom` が設計書 3.12 の表の順でスキップ・失敗・実行を決め、理由に URL とその末尾 8 文字を含めず、本番の `SLACK_WEBHOOK_URL` を読まないこと（ステップ 7-2・7-3） / 統合テストが `//go:build integration` を持ち、`make lint` の `go vet -tags integration` でコンパイルされ、`make test` では実行されないこと（ステップ 7-4・7-7・7-9・7-10） / 固定の記事が分割しない記事でちょうど 16,383 コードポイントになり、実行ごとの目印で自分の投稿を見分けられること（ステップ 7-2・7-5） / `RunMakeTarget` の追加の名前がオプトインを記録し、各ターゲットが自分以外のオプトインをエクスポートしないこと（ステップ 7-1・7-5・7-8・7-9）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 7-4・7-13 は実 Mattermost の Webhook に投稿する外部リソースの面、ステップ 7-9 は `make` のターゲットによる CI の面という `mkplan.md` のパネルモードのトリガーに該当し、加えてステップ 7-2・7-3 の環境変数によるスキップの判定と、実行ごとの目印による再実行の隔離という複数の Conditional check に該当するため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 8: 文書と手動確認

**対象ファイル**
-   変更: `README.md`、`docs/dev/project_overview.md`、`docs/dev/security.md`、`docs/dev/developer_guide/package_reference.md`（最終確認）、`cmd/yt2column/docs_test.go`

**タスク**
-   [ ] **ステップ 8-1**: `README.md` を、設計書 3.13 の表と要件書 F-010 の各項目のとおりに更新する。フラグの表の `--slack` の行（ステップ 6-2 で加えたもの）の説明を確かめ、設定の表の `SLACK_WEBHOOK_URL` の行を改め、`make test-integration-slack`・`make test-integration-cli-slack` の実行方法を加える。冒頭（`README.md:5-6`）と前提（`:24`）の未実装の記述も直す。
-   [ ] **ステップ 8-2**: `docs/dev/project_overview.md` を、設計書 3.13 の表のとおりに更新する（概要、`Publisher` の初期実装、「前提・制約」の上限値と出典、想定ディレクトリ構成のコメント、「設定（環境変数）」の表）。
-   [ ] **ステップ 8-3**: `docs/dev/security.md` を、設計書 3.13 の表のとおりに更新する（§2・§3・§6）。§2 の統合テストの表に Webhook の 2 つのテストを加える。§2 の `--dry-run` の記述（§1.3）は、`security.md:19` の文「Webhook URL は URL 自体が秘密情報である。ログ・エラーメッセージ・`--dry-run` の出力に含めない。」を「Webhook URL は URL 自体が秘密情報である。ログ・エラーメッセージに含めない。」に改める（同じ行の続きの文は変えない）。
-   [ ] **ステップ 8-4**: `package_reference.md` の各行が、フェーズ 1〜7 で変えた公開の API と一致することを確かめる（`TestPackageReferenceListsPackages` は行の有無だけを確かめる）。
-   [ ] **ステップ 8-5**: `cmd/yt2column/docs_test.go` を更新する（AC-34・AC-22・AC-33）。
    -   `configDocRows` の `SLACK_WEBHOOK_URL` の行（`docs_test.go:32`）の README と概要の「未設定のとき」の期待する文言を、ステップ 8-1・8-2 の表の新しい記述に合わせる。期待する文言は要件書 F-007 の「`--slack` では必須」から決める。
    -   `TestSlackDocsContract`: 機械的に確かめられる契約値だけを固定する。README と `security.md` が、`publishertestutil` の 2 つのオプトインの変数と `WebhookURLEnv` の名前、2 つの `make` のターゲットの名前を含むこと。README と概要が上限値 16,383 を含むこと。散文の意味は固定しない（0005 の計画書のステップ 9-4 の判断と同じ）。
    -   `TestSlackPlanRecordsManualChecks`: 本計画のステップ 7-13・8-6・8-7 が存在し、チェックされている場合は、その下に結果の記録（サーバの版、`結果`）があること。既存の `stepBlock` を使い、計画書のパスは新しい定数にする。
    -   日本語の文書の文字列と照合するリテラル（`結果` など）は、既存の `docs_test.go`（`値なし`・`結果`）と同じく日本語で書く。照合する文書の内容そのものであり、それ以外のコメントと識別子は英語で書く。
-   [ ] **ステップ 8-6**: 手動確認（AC-22。利用者の承認を得て、本番で使う Mattermost のサーバで行う）。`silent: true` を付けた 2 つのメッセージ（`@channel` を含むもの、確認する人のユーザー名へのメンションを含むもの）を Webhook に直接送り、それぞれについて、通知（デスクトップ・プッシュ・メール）が発生しないこと、確認する人のチャンネルの未読数とメンション数が増えないこと、「New Messages」の表示が付かないことを確かめる。本ステップの下に、確認日、サーバの版、各項目の結果を記録する。
-   [ ] **ステップ 8-7**: 手動確認（AC-33・F-009。利用者の承認を得て、Mattermost のテスト用のチャンネルの Webhook で行う）。1 つのメッセージに収まる記事と、分割される記事を `--slack` で投稿し、見出し・段落・リスト・リンク・日本語が Markdown として表示されること、分割した場合は分割の位置の表示とともに記事の順に並び、サーバによる追加の分割が起きていないことを確かめる。本ステップの下に、確認日、使った記事（動画 URL または固定の記事）、サーバの版、結果を記録する。
-   [ ] **ステップ 8-8**: 文書の内容の照合。要件書 F-010 の各項目と設計書 3.13 の表の各項目を、ステップ 8-1〜8-3 で書いた本文と 1 つずつ突き合わせ、記述の根拠（README の上限値と拒否される記事の種類は `internal/publisher` の定数と M1〜M4、統合テストの実行方法は `Makefile` のターゲット）を確かめる。照合した項目の一覧をコミットメッセージに書く。`make fmt` → `make test` → `make lint` を通す。`TestSlackDocsContract`・`configDocRows` は、README の該当の記述を一時的に消して失敗することを確かめる。

### PR-8 作成ポイント: documentation and manual verification

**対象ステップ**: 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8

**推奨タイトル**: `docs(0006): document webhook publishing and verify it manually`

**レビュー観点**: README・`project_overview.md`・`security.md` が要件書 F-010 の各項目と設計書 3.13 の表のとおりで、`--slack` の使い方・上限値・拒否される記事・統合テストの実行方法を含むこと（ステップ 8-1〜8-3） / `docs_test.go` の `TestSlackDocsContract` が機械的に確かめられる契約値だけを固定し、散文の意味を固定しないこと（ステップ 8-5） / `TestSlackPlanRecordsManualChecks` がステップ 7-13・8-6・8-7 の存在と、チェック済みの場合の結果の記録を確かめること（ステップ 8-5） / 手動確認（ステップ 8-6・8-7）の確認日・サーバの版・結果が本計画に記録されていること（ステップ 8-6・8-7）

**実装モデル要件**: standard

**判定理由**: 文書の更新・文書のテスト・手動確認に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・2 つ以上の Conditional check のいずれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | フェーズ | 成果物 | 完了の判定 |
|---|---|---|---|
| M1: 規則の共有 | 1・2 | `internal/slackwebhook`、`internal/config` の新しい規則、`internal/loopbacktest` | `make test`・`make lint` が通り、AC-03・AC-03a のテストが通る |
| M2: Publisher | 3・4 | `SlackWebhookPublisher`（準備と送信）、テスト用の構築 | AC-01〜AC-21a・AC-35 のテストが通る |
| M3: CLI | 5・6 | `job.Output`、`--slack` | AC-23〜AC-28 のテストが通る |
| M4: 統合テスト | 7 | 判定、固定の記事、統合テスト、`make` のターゲット | AC-29・AC-32 のテストと `TestIntegrationArticles` が通り、ステップ 7-13 に AC-30・AC-31 の結果が記録されている |
| M5: 文書と手動確認 | 8 | 文書、手動確認の記録 | AC-34 のテストが通り、ステップ 8-6・8-7 に結果が記録されている |

### 3.2. PR 構成

PR はフェーズと 1 対 1 に対応させる。各 PR は主たる関心事（規則の共有 / ループバックの判定の移動 / Publisher の準備 / Publisher の送信 / `job.Output` / CLI の `--slack` / 実際の Webhook を使う統合テストの面とその `make` のターゲット / 文書）を持ち、単独でグリーンゲートを通せる単位とする。フェーズ 7 は、`RunMakeTarget` の追加の名前、`publishertestutil` のオプトインの名前、統合テスト、`make` のターゲットのテストが同じ定数とターゲット名を共有するため、1 つの PR とする。`make` のターゲットとそのテストを別の PR に分けると、テストが相手の PR で足すターゲットを参照して片方のグリーンゲートが通らなくなる。ステップは並べ替えていないので、ステップ番号の順と文書の順は一致し、各 `### PR-N 作成ポイント` は直前のフェーズの最後のステップの後にある。`internal/` の変更（PR-1〜PR-5）は、それを使う `cmd/`（PR-6）に先行する。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | `internal/slackwebhook`（`ValidURL`・`SensitiveParts`）と、`internal/config` の規則の共有・`RequireSlackWebhookURL`・`HTTP2DebugEnabledIn` | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | ループバックの判定を `internal/loopbacktest` へ移し、`pipeline_test.go` のガードを追従させる | standard |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 | `SlackWebhookPublisher` の準備（分割・V1〜V3・M1〜M4・UTF-8 の確認・`SlackMessageCount`）とエラー型 | frontier-recommended |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | `SlackWebhookPublisher` の送信（`Publish`・応答の検証・リダイレクト・タイムアウト）とテスト用の構築 | frontier-recommended |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 | `internal/job` の `Output`（`FileOutput`・`RemoteOutput`）と `cmd/yt2column/run.go` の組み立て | standard |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10 | `cmd/yt2column` の `--slack`（投稿先の決定・手順の分岐・表示・伏せ字化）とテストの組み立て | frontier-required |
| PR-7 | 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6 / 7-7 / 7-8 / 7-9 / 7-10 / 7-11 / 7-12 / 7-13 | 統合テストの判定・固定の記事・2 つの統合テストと `make` のターゲット | frontier-required |
| PR-8 | 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8 | 文書（README・`project_overview.md`・`security.md`・`package_reference.md`）と手動確認の記録 | standard |

### 3.3. 実装順序の根拠

設計書 §8 の依存のとおりである。フェーズ 2 と 5 は他のフェーズに依存しないが、§8 の順に行う。フェーズ 5 は `cmd/yt2column/run.go` の `job.Request` の組み立てを同じステップで直す（ステップ 5-2）。そうしないと、フェーズ 5 の完了条件の `make test` がコンパイルで失敗する。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   設計書 7.1 のとおりである。Webhook への送信は、すべて `httptest` のループバックのサーバへ、`NewSlackWebhookPublisherForLoopbackTest` で構築した値から行う。実際の Webhook は呼ばない。
-   準備の段階（分割と記法の検査）は、非公開の準備の関数を直接呼ぶテスト（フェーズ 3）と、送信を通したテスト（フェーズ 4）の両方で確かめる。前者は境界の行を網羅し、後者は AC が求める「送信先が受け取るもの」を確かめる。後者で前者の行の表を繰り返さない。
-   CLI のテストは、既存の `executionPathRows` の副作用の検査、`requireSafeOutput`、子プロセスのモードを使う。

### 4.2. 統合テスト

設計書 3.12・7.2 のとおりである。2 つの統合テストは `//go:build integration` を持ち、`make test` と CI では実行されない。`make lint` の `go vet -tags integration ./...` がコンパイルする。実行は、利用者の承認を得たうえでステップ 7-13 で行う。

### 4.3. 後方互換性

-   `--out` の経路は、`--out ""` の誤りの文言（設計書 3.14 の E3）と、`--out` を指定しない場合の文言（`run_test.go:357`）を除き、変わらない。既存の `--out` のテスト（`run_test.go`・`signal_test.go`・`integration_test.go`）の期待する値を変えずに通すことで確かめる。テストのコードで変えるのは、`e.d.newPublisher` の名前、`no --out` の文言、`newRunEnv` の `newSlackPublisher` の既定（ステップ 6-6）、`requireInterruptedChildOutput` の `--out` の前提（ステップ 6-8）、`runIntegrationCLI` からの補助の取り出し（ステップ 7-7）、Webhook の行の追加だけである。
-   `SLACK_WEBHOOK_URL` の規則の変更（E4）は、`config_test.go` の受理と拒否の表で確かめる。
-   `internal/llm/deepseek` の振る舞いは変えない。`NewForLoopbackTest` の既存のテストを変えずに通す。

### 4.4. 網羅率

新設の `internal/slackwebhook`・`internal/loopbacktest`、`internal/publisher` の Webhook 用のファイルの関数は、到達できない防御の分岐（`http.NewRequestWithContext` の失敗など）を除き、ユニットテストで実行する。フェーズ 4 の完了時に `go tool cover -func` で確かめ、実行されない分岐をコミットメッセージに挙げる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

ラベル: `test` は失敗しうる実行のテスト、`static` は文書・ビルドタグ・`make` のターゲットを確かめるテストや `make` のターゲット、`manual` は手動の確認である。

| AC | ラベル | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test | `internal/publisher/slack_test.go::TestNewSlackWebhookPublisher` | ステップ 4-1・4-2 |
| AC-02 | test | `internal/publisher/slack_test.go::TestNewSlackWebhookPublisherRejects` | ステップ 4-1 |
| AC-03 | test | `internal/slackwebhook/slackwebhook_test.go::TestValidURL`、`internal/config/config_test.go::TestLoadInvalid`・`TestLoadErrorsOmitValues`、`internal/publisher/slack_test.go::TestNewSlackWebhookPublisherRejects` | ステップ 1-1・1-3・4-1 |
| AC-03a | test | `internal/config/config_test.go::TestLoadSlackWebhookAccepted` | ステップ 1-3 |
| AC-04 | test | `internal/publisher/slack_test.go::TestSlackPublishPayload` | ステップ 3-2・4-1 |
| AC-05 | test | `internal/publisher/slack_message_test.go::TestSlackPrepareRejectsInvalidArticle`、`internal/publisher/slack_test.go::TestSlackPublishPrepareSendsNothing` | ステップ 3-2・4-1 |
| AC-06 | test | `internal/publisher/slack_test.go::TestSlackPublishPayload` | ステップ 3-2・4-1 |
| AC-07 | test | `internal/publisher/slack_test.go::TestSlackPublishPayload`、`internal/publisher/slack_message_test.go::TestSlackSplit` | ステップ 3-2・4-1 |
| AC-08 | test | `internal/publisher/slack_test.go::TestSlackPublishSplit`、`internal/publisher/slack_message_test.go::TestSlackSplit` | ステップ 3-2・4-1 |
| AC-09 | test | `internal/publisher/slack_test.go::TestSlackPublishSplit`、`internal/publisher/slack_message_test.go::TestSlackSplit` | ステップ 3-2・4-1 |
| AC-10 | test | `internal/publisher/slack_test.go::TestSlackPublishSplit`、`internal/publisher/slack_message_test.go::TestSlackSplit` | ステップ 3-2・4-1 |
| AC-11 | test | `internal/publisher/slack_message_test.go::TestSlackSplitRejects`、`internal/publisher/slack_test.go::TestSlackPublishPrepareSendsNothing` | ステップ 3-2・4-1 |
| AC-12 | test | `internal/publisher/slack_test.go::TestSlackPublishResponses` | ステップ 4-1 |
| AC-13 | test | `internal/publisher/slack_test.go::TestSlackPublishResponses` | ステップ 4-1 |
| AC-14 | test | `internal/publisher/slack_test.go::TestSlackPublishResponses` | ステップ 4-1 |
| AC-15 | test | `internal/publisher/slack_test.go::TestSlackPublishRedirect` | ステップ 4-1 |
| AC-16 | test | `internal/publisher/slack_test.go::TestSlackPublishTimeout` | ステップ 4-1・4-2 |
| AC-17 | test | `internal/publisher/slack_test.go::TestSlackPublishCanceled` | ステップ 4-1 |
| AC-18 | test | `internal/publisher/slack_test.go::TestSlackPublishPartialFailure`、`internal/publisher/slack_errors_test.go::TestSlackPostErrorMessage` | ステップ 3-1・4-1 |
| AC-19 | test | `internal/publisher/slack_test.go::TestSlackPublishErrorsOmitWebhookURL`、`internal/publisher/slack_test.go::TestSlackPublishTransportErrorWithheld`、`internal/slackwebhook/slackwebhook_test.go::TestSensitiveParts` | ステップ 1-1・4-1・4-2 |
| AC-20 | test | `internal/publisher/slack_test.go::TestSlackPublishResponses`、`internal/publisher/slack_errors_test.go::TestSlackHTTPStatusErrorMessage` | ステップ 3-1・4-1 |
| AC-21 | test | `internal/publisher/slack_message_test.go::TestSlackMentionRejected`、`internal/publisher/slack_test.go::TestSlackPublishPrepareSendsNothing` | ステップ 3-2・4-1 |
| AC-21a | test | `internal/publisher/slack_test.go::TestSlackPublishPayload` | ステップ 4-1 |
| AC-22 | static・manual | `cmd/yt2column/docs_test.go::TestSlackPlanRecordsManualChecks`（ステップ 8-6 の記録の有無）、ステップ 8-6 の手動確認 | ステップ 8-5・8-6 |
| AC-23 | test | `cmd/yt2column/run_test.go::TestRunExecutionPaths`（`--slack` の成功と `--keep-cache` の行） | ステップ 5-1・6-1・6-3・6-6 |
| AC-24 | test | `cmd/yt2column/run_test.go::TestRunExecutionPaths`（投稿先の拒否と `SLACK_WEBHOOK_URL` の未設定の行）、`internal/config/config_test.go::TestRequireSlackWebhookURL` | ステップ 1-3・6-3 |
| AC-25 | test | `cmd/yt2column/run_test.go::TestRunExecutionPaths`（分割投稿の途中の失敗の行）、`cmd/yt2column/run_test.go::TestRunSlackFailureHints` | ステップ 3-1・6-4 |
| AC-26 | test | `cmd/yt2column/run_test.go::TestRunExecutionPaths`（`--slack` のすべての行）・`TestRunEscapesUntrustedText`・`TestConfiguredSecretsIncludesWebhookParts`、`cmd/yt2column/signal_test.go::TestSignalDuringSlackPost` | ステップ 6-5・6-6・6-8 |
| AC-27 | test | `cmd/yt2column/run_test.go::TestRunHelp` | ステップ 6-2 |
| AC-28 | test | `cmd/yt2column/run_test.go::TestRunGODEBUGWarning` | ステップ 6-3 |
| AC-29 | test | `internal/publisher/testutil/integration_settings_test.go::TestSlackSettingsFrom`、`cmd/yt2column/makefile_test.go::TestCLISlackIntegrationSettings` | ステップ 7-2・7-3・7-6・7-8 |
| AC-30 | test・static・manual | `internal/publisher/slack_integration_test.go::TestIntegrationSlackWebhookPublisher`（`make test-integration-slack` で実行）、`internal/publisher/slack_articles_test.go::TestIntegrationArticles`、`internal/publisher/makefile_test.go::TestMakeTestIntegrationSlack`、`cmd/yt2column/docs_test.go::TestSlackPlanRecordsManualChecks`（ステップ 7-13 の記録の有無） | ステップ 7-2・7-4・7-5・7-9・7-13 |
| AC-31 | test・static・manual | `cmd/yt2column/integration_slack_test.go::TestIntegrationCLISlack`（`make test-integration-cli-slack` で実行）、`cmd/yt2column/makefile_test.go::TestMakeTestIntegrationCLISlack`、`cmd/yt2column/docs_test.go::TestSlackPlanRecordsManualChecks`（ステップ 7-13 の記録の有無） | ステップ 7-7・7-8・7-9・7-13 |
| AC-32 | static | `internal/publisher/makefile_test.go::TestMakeTestIntegrationSlack`・`TestSlackIntegrationTestBuildTag`、`cmd/yt2column/makefile_test.go::TestMakeTestIntegrationCLISlack`・`TestCLISlackIntegrationTestBuildTag`・`TestMakeOptInsAreTargetSpecific`、`make lint`（`go vet -tags integration ./...`） | ステップ 7-5・7-8・7-9 |
| AC-33 | static・manual | `cmd/yt2column/docs_test.go::TestSlackPlanRecordsManualChecks`（ステップ 8-7 の記録の有無）、ステップ 8-7 の手動確認 | ステップ 8-5・8-7 |
| AC-34 | static・manual | `cmd/yt2column/docs_test.go::TestREADMEDocumentsCLI`（`--slack` の行）・`TestProjectOverviewDocumentsConfig`（`configDocRows`）・`TestSlackDocsContract`、`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`、ステップ 8-8 の照合 | ステップ 1-5・2-4・4-4・5-4・6-2・6-9・7-11・8-1〜8-5・8-8 |
| AC-35 | test | `internal/publisher/slack_test.go::TestSlackPublishErrorClasses` | ステップ 3-1・4-1 |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| 記法の拒否（M2・M3・M4）が実際の記事を多く拒否する | `--slack` で記事を投稿できず、LLM の費用が無駄になる | 設計書 5.2 のとおり受け入れる。ステップ 7-13・8-7 の実行で拒否が起きたかを記録し、多ければ設計書 9 章の緩和の検討の材料にする |
| CLI の統合テスト（ステップ 7-13）で、LLM が生成した記事がメールアドレスや `@` で始まる語を含み、記法の拒否で終了コード `1` になる | 統合テストが記事の内容によって失敗する | 失敗した場合は、記法の拒否の案内が表示されたことを記録し、再実行する。テストの判定は変えない（拒否は仕様どおりの振る舞いであり、統合テストはそれを含めて CLI の組み立てを確かめる） |
| 本番の Mattermost のサーバが `silent` に対応しない版である | 通知の抑止が拒否の層だけになる | ステップ 8-6 で版と効果を記録し、結果を利用者に報告する（設計書 5.2） |
| テスト用のサーバの `MaxPostSize()` が 16,383 より小さい | サーバが追加の分割をし、分割の位置の表示と食い違う | ステップ 7-13・8-7 で投稿の数を記録して確かめる |
| `run`・`reportRunError` の循環的複雑度が `gocyclo` の上限を超える | `make lint` が失敗する | ステップ 6-3・6-4 で投稿先の決定、要約、Webhook の案内を関数に分ける |
| 末尾の空白などを含む `SLACK_WEBHOOK_URL` を、新しい規則が受理する（§1.3） | 誤って貼り付けた値が、LLM の呼び出しの後の投稿で初めて失敗する | 利用者がリスクとして受け入れた（2026-10-07）。要件書 F-001 の規則のとおり、空白を拒否する検査は加えない |
| タイミングに依存するテスト（タイムアウト、待機中のキャンセル、子プロセスのシグナル）が遅い CI で不安定になる | テストが時々失敗する | 既存のテストと同じく、短いタイムアウトと準備完了の印を使い、固定の `sleep` で順序を作らない。メッセージの間隔は、待機中のキャンセルの行を除き `SlackTestOptions.Interval` で 0 にする。キャンセルの時点は印で決める（ステップ 4-3） |

### 6.2. スケジュールのリスク

| リスク | 対策 |
|---|---|
| 手動確認（ステップ 8-6・8-7）と統合テストの実行（ステップ 7-13）は、利用者の承認と Mattermost の準備（テスト用のチャンネルの Webhook、確認する人のアカウント）を待つ | フェーズ 7・8 のコードと文書の作業を先に終え、承認を待つ間に他のステップを止めない |
| フェーズ 4 のテストの量が多い | フェーズ 3（準備）とフェーズ 4（送信）を分け、それぞれで完了条件を満たす |

## 7. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
-   [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）
-   [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5）
-   [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5）
-   [ ] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10）
-   [ ] PR-7 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6 / 7-7 / 7-8 / 7-9 / 7-10 / 7-11 / 7-12 / 7-13）
-   [ ] PR-8 マージ済み（対象ステップ: 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8）
-   [ ] §5 のすべての AC の検証が通り、手動確認の結果が記録されている

## 8. 成功基準 (Success Criteria)

-   **機能:** §5 の `test`・`static` の検証がすべて通る。AC-22・AC-30・AC-31・AC-33 の結果が本計画に記録されている。
-   **品質:** `make test` と `make lint` が通る。各テストが壊して失敗することを確かめ、コミットメッセージに記録している。§4.4 の網羅率を確かめている。
-   **セキュリティ:** AC-15・AC-19・AC-21・AC-21a・AC-26・AC-28・AC-29・AC-31 のテストが通り、ステップ 8-6 で `silent` の効果とサーバの版を記録している。
-   **文書:** ステップ 8-8 の照合を終え、`package_reference.md` の各行が公開の API と一致している。

## 9. 次のステップ (Next Steps)

-   PR の境界は §2・§3.2・§7 に埋め込み済みである。`/runplan 0006` で PR-1 から実装する。
-   実装の完了の後、ステップ 8-6 の結果（`silent` に対応する版か）と、ステップ 7-13・8-7 で記法の拒否が起きたかを利用者に報告し、設計書 9 章の「メンションの拒否の緩和」を検討するかを決める。
