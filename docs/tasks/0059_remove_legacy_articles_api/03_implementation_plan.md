# 実装計画書：旧 API（`/articles`）とホストの今の形の中継の削除

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-04 |
| Review date | 2026-11-04 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.2.1 節」などは設計書の節を指す。用語は要件書 6 章に従う。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-10 は、設計書 10 章がすべて記録している。

既存のコードについての記述は、コミット `b8941e71`（2026-11-04 の `main`）で確かめた。設計書が行番号を引いたコミット `38aee56f` から `b8941e71` までに、`internal/server`・`internal/nmhost`・`internal/publisher`・`internal/failure`・`internal/pipeline`・`internal/serversock`・`cmd/yt2column-server`・`cmd/yt2column-nmhost`・`README.md`・`docs/dev/guides/development.md`・`docs/dev/security.md`・`docs/dev/project_overview.md`・`docs/dev/production_profile.md`・`docs/dev/conventions/package_reference.md` は変わっていない（`git diff --stat 38aee56f b8941e71 -- <これらのパス>` の出力が空）。`CLAUDE.md` は `:414` の後に 8 行が加わっただけで、設計書が引く `:279` と `:357-375` は変わっていない。そのため、設計書の `file:line` はそのまま使える。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

localhost サーバの旧 API（`POST /articles`）・旧 API のジョブ・失敗の通知と、Native Messaging のホストの今の形のメッセージの中継を削除し、新旧の両対応のために複雑になった生成の枠・停止の待ち方・その説明を、ジョブの API だけを前提にした形に戻す。削除するもの、残すもの、変える振る舞いは設計書 2〜6 章で決まっている。本書は、それをどの順序で、どのコミットに分け、どのテストと確かめで検証するかを定める。

### 1.2. 実装の原則

-   PR は設計書 8 章のとおり 2 つにする。PR-1 はホスト（フェーズ 1）、PR-2 はサーバ（フェーズ 2〜4）である。
-   各 PR で、テストの移し替えを、製品のコードを変えないコミットとして削除より先に置く（設計書 1.3 節、7.1 節「テストの削除の確かめ方」）。テストの移し替えのコミットと削除のコミットのそれぞれで、カバレッジを前後で比べる（1.3 節「カバレッジの比べ方」）。
-   移し替えでも削除でも、使う所がなくなったテストの補助と型は、そのコミットで削除する（`make lint` の `unused` はテストのファイルも調べる）。
-   削除の内容・残すもの・型の形・ログの属性・案内の計算は、設計書の節を参照し、本書では繰り返さない。ログと案内の文言、テストの表の行、確かめのスクリプトの細部は、コードを書くときに決める。
-   加えたテストと確かめのスクリプトは、それが確かめる対象を壊して失敗することを実装のときに確かめ、コミットメッセージにそう書く（CLAUDE.md「Testing Strategy」）。否定の確かめ（何も送らない、何も書かない）には、確かめる処理まで届いたことを示す肯定の印（ログの `stage=`、`rule=`、`op=` など）を必ず組み合わせる。
-   Go のコメント・識別子・文字列リテラルは英語で書く（CLAUDE.md「Go Idioms」）。`_checks/` のプログラムも同じである。
-   各フェーズの終わりに、`make fmt`・`make test`・`make lint`・`make deadcode` を通す。PR-1 は拡張の表示のテスト（AC-12）のために、PR-2 は README と `docs/dev/guides/development.md` を変えるので、`make ext-test` も通す（CLAUDE.md「Development Notes」）。
-   文書のガードは、CI の `doc-guard` と同じ `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline` で実行する（以下「文書のガード」）。

### 1.3. 計画の際に決めたこと（2026-11-04）

レビュアーに尋ねるべき未決の事項はなかった。設計書が計画に委ねた次の点は、結果が大きく変わらない選択なので、理由とともにここに記録する。

-   **`job.go` は削除し、残るものを移す（設計書 3.2.1 節）:** 定数 `maxInFlight` と関数 `acquireSlot`・`releaseSlot` は `jobs_api.go` の `maxReading` の隣へ、`classifyWrite` は唯一の呼び出し元である `generation.go` へ移す。`job_test.go` も削除する（設計書 7.1 節の表で全テストが削除の対象）。理由: 2 種類の枠の定義が 1 つのファイルに並び、`classifyWrite` が使う側と同じファイルに置かれる。`job.go` という名前は旧 API のジョブを指しており、残すと名前と中身が合わなくなる。
-   **`SlackMaxPostDuration` は `internal/publisher/slack.go` に置き、同じ式のテストの補助と結びつける（設計書 3.3 節）:** `SlackMaxPostDuration` の元になる 3 つの定数のうち 2 つ（`SlackPostTimeout`・`slackMessageInterval`、`slack.go:23`・`:28`）と `PostMessages` が `slack.go` にある。同じ式は、テストの補助 `SlackPublishMaxDurationForTest(messages)`（`internal/publisher/test_helpers_slack.go:118`）が任意の通数について持っている。`cmd/yt2column/makefile_test.go:73` は最大の通数でこの補助を呼んでいるので、`SlackMaxPostDuration` を使う形に改める。補助は `internal/publisher/makefile_test.go:36` が任意の通数で使うので残し、`internal/publisher` のテストで、最大の通数での補助の値と `SlackMaxPostDuration` が一致することを確かめて、2 つの式が離れないようにする。
-   **停止の案内の分の丸めは、小さな関数にして境界を確かめる（設計書 3.3 節）:** 設計書 3.3 節は、切り捨てが 0 分や過小な値を出しうるので「最も近い分に丸め、1 分未満なら 1 分」を選んだ。今の値（309 秒）では切り捨てでも 5 分になり、`TestRunLogsListeningAndStop` だけでは選んだ規則を確かめられない。そこで、`cmd/yt2column-server` に時間から分を求める関数を置き、0・29 秒・30 秒・89 秒・90 秒・309 秒などの境界を表のテストで確かめる。
-   **AC-02・AC-06・AC-07・AC-08・AC-13 の静的な確認は、`_checks/` の Go のプログラム 1 つにする（設計書 7.4 節）:** `docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go`（以下 `_checks/residue`）を PR-1 で作り（ホストの確かめだけ）、PR-2 でサーバの確かめを加える。標準ライブラリの `go/parser`・`go/ast` でパッケージと識別子を読んで確かめ、文字列の検索をしない（名前の衝突を避けるため。例えば `cmd/yt2column/docs_test.go:597` のローカル変数 `stageOf`）。同じプログラムで、`extension/` が merge-base から変わっていないことも確かめる（AC-12 の `diff` の確かめ）。PR ごとに 1 度、PR の head で実行し、結果を 7.1 節に記録する。理由: 文書に書いただけのコマンドは実行されない。
-   **AC-08（Go のコメントとログの文字列）は、静的な確認と読む確認の両方にする:** AC-08 は文書ではなくコードについての AC なので、`manual` だけにしない。`_checks/residue` で、`internal/server`・`cmd/yt2column-server`・`internal/serversock`・`internal/publisher` のテストでないファイルのコメントと文字列リテラルに、`/articles` と失敗の通知を指す語が残っていないことを確かめる。語の一覧は、merge-base で確かめが成り立たず、PR の head で成り立つように実装のときに決める。文の意味は PR のレビューで読む。
-   **`_checks/` のプログラムの形:** 0052 の `_checks/coverage/main.go` と同じく、1 行目を `//go:build test` にし、`go run -tags test` で動かす（`internal/config/envaccess_test.go` の環境変数の読み取りの検査が、`docs/tasks/**/_checks` の Go のファイルも調べ、`test` の build 制約のないファイルを製品のコードとして扱うため）。どちらのプログラムも、確かめる 2 つのコミットを引数に含み、それぞれの内容を `git` から読む（作業ツリーは読まない）。一時的な worktree を使うときは、終了コード `2` の場合を含め、どの終わり方でも消す。
-   **カバレッジの比べ方:** どの比較も、`go test -tags test -count=1 -coverpkg=./... -coverprofile=<file> ./...` を両方のコミットで実行し（どちらもテストがすべて通ること）、`go tool cover -func` の出力を比べる。子プロセスで動くコード（`TestSignal*` の子のサーバ、`TestRelayEndToEnd` のホストのバイナリ）はカバレッジに現れないので、その経路は、移し替えたテストを壊して失敗することの確認だけが根拠になる。
    -   **テストの移し替えのコミット:** 製品のコードが変わらないので、0052 のスクリプト（`docs/tasks/0052_doc_test_pruning/_checks/coverage/main.go`、(ファイル, 行, 名前) で比べる）を使う。基準は、そのフェーズの後で残る関数のカバレッジが親コミットと一致することである。カバレッジが変わってよいのは、そのフェーズの後の削除で消える関数（ホストは 1.4 節の削除するもの、サーバは設計書 3.2.1 節の表と 3.5 節・4 章の削除するもの）だけで、変わった関数をすべてコミットメッセージに挙げる。
    -   **削除のコミット:** 残る関数の行番号がずれ、`job.go` の関数はほかのファイルへ移るので、関数を (パッケージ, レシーバ, 名前) で対応づける新しいスクリプト `docs/tasks/0059_remove_legacy_articles_api/_checks/coverage/main.go`（以下 `_checks/coverage`）を PR-1 で作る。レシーバは各コミットのソースを読んで求め、対応が 1 つに決まらなければ `2` で止まる。製品のコードの見分け方（`_test.go` と `test` で始まる build 制約のファイルを除く）は、各コミットのソースで判定する。基準は、削除前のコミットにだけある関数が、削除した関数だけであり、残る関数でカバレッジが下がったものがないことである。下がった関数があれば、`go tool cover -html` で前後を見比べ、削除の後に実行されなくなった文があればテストを加える。なければ（実行されていた文を削除しただけなら）その関数と理由をコミットメッセージに書く。
    -   0055 の道具（`internal/coveragecompare` は `main` にあり、`make coverage-compare` の入口はまだない）は使わない。0055 の要件は、削除以外の変更で対応が付かなければ止まることであり、残る関数を書き換える削除のコミットには使えない（設計書 1.3 節）。移し替えのコミットでは使えるが、入口がまだないので、0052 のスクリプトにそろえる。
-   **PR-2 の中の順序は、テストの移し替え（フェーズ 2）、コードの削除（フェーズ 3）、文書（フェーズ 4）とする（設計書 8 章）:** 削除と同じコミットで直さないと `make test` が失敗する文書は、フェーズ 3 に置く。該当するのは、`package_reference.md` の `internal/failure` の 2 行（`TestPackageReferenceListsPackages`、`internal/pipeline/pipeline_test.go:629`）と、curl の例のガードの `server.ArticlesPath` の行（`cmd/yt2column-server/docs_test.go:107`。残すとコンパイルが通らない）である。ほかの文書の書き換えはフェーズ 4 にまとめ、文書のレビューをコードの差分から分ける。`development.md` の `/articles` の curl の例は、ガードの行を除いた後は一致を求められないだけで失敗はしない（`pathForm` が `""` を返す。`docs_test.go:393-400`）ので、フェーズ 4 で除く。
-   **フェーズ 3 の削除は 2 つのコミットに分ける:** (a) サーバと `cmd/yt2column-server` の `/articles`・旧 API のジョブ・失敗の通知の配線の削除（`WebhookOptions.Notifier` を除くと `cmd/yt2column-server` のコンパイルが通らなくなるので、同じコミットにする）、(b) どこからも使われなくなった `internal/failure`・`SlackFailureNotifier` とその周りの削除。どちらのコミットもグリーンゲートを通す。理由: 差分の大きさを分け、(b) が純粋な削除であることを読みやすくする。
-   **AC-18 の文書の値（同時に読む本文の最大 2 つ、約 5 分）にガードを加えない:** 同時に読む本文の数（`maxReading = 2`）は生成の枠の数（`maxInFlight = 2`）と同じ値なので、`security.md` §8.2 の今のガード（`TestServerDocsContract` の `security.md` のサブテスト、`docs_test.go:238-`）と同じ形の検査では 2 つを区別できない。「約 5 分」は計算した値を丸めて書く文で、値は AC-15 のテストがコードの側で確かめる。誤りの害は小さく、ガードの手入れの費用のほうが大きい（CLAUDE.md「Document tests」）。どちらも PR のレビューで読む。

### 1.4. 既存コード調査結果

**ホスト（`internal/nmhost`、`b8941e71` で確かめた）**

-   削除するもの（設計書 3.4.1 節）: `articlesURL`（`relay.go:54-55`）、型 `form` と `formLegacy`・`formNew`（`:427-434`）、`destination.form`（`:451`）、`route` の `!object.Has("v")` の分岐（`:468-470`）、`logAttrs` の `form` 属性（`:514`）、`request` の `switch d.form`（`:529-545`）。`route` は今、`CollectOnly` を版の検査より先に呼ぶ（`:473-479`）。
-   `strictjson.Object.Collect`（`internal/strictjson/strictjson.go:90`）と `Has`（`:126`）は今ある。`Has` はほかのパッケージが使うので残る（設計書 3.4.1 節）。
-   今の形のメッセージを入力に使うテストは、設計書 7.1 節のホストの表と 7.2 節の表のとおりで、名前はすべて今のコードにある。このほか、`TestRelayLengthChecks`（`relay_test.go:328`）が今の形の書き出し（`{"videoUrl":`）を入力の先頭に使う（本文は読まれない）。
-   残るテストのうち、`/articles` を含む文字列リテラルを持つものがある。`TestRelayNewFormRejections` の `"path":"/articles"` の行（`relay_test.go:940`）と `TestRelayKeyPassedUnchecked` のキー `"../../articles"`（`:992`）である。どちらも新しい形の検査の入力で、`/articles` の受け口とは関係がないので残す。そのため、AC-13 の文字列リテラルの確かめは、`internal/nmhost` のテストでないファイルだけを対象にする。
-   拡張が `protocol_error` を想定外の応答として表示することは、`extension/test/native.test.ts:82` のテストが確かめている（AC-12）。

**サーバ（`internal/server`）**

-   削除するもの・残すものは設計書 3.2.1 節の表のとおりである。`job.go` に残るのは定数 `maxInFlight`（`:22`）と関数 `acquireSlot`（`:30-37`）・`releaseSlot`（`:40-42`）・`classifyWrite`（`:147-`）で、どれも 1.3 節のとおり移す。
-   テストでないコードで `internal/pipeline` を import するのは、`cmd/yt2column/run.go:20`、`internal/job/job.go:19`、`internal/server/job.go:12` の 3 つである（`git grep -n 'internal/pipeline"' -- '*.go' ':!*_test.go'` の出力が 3 行）。`internal/job` は CLI の実行の部品で、`cmd/yt2column` が使う。テストでないコードで `.Publish(` を呼ぶのは `internal/pipeline/pipeline.go:116` だけである（`git grep -n '\.Publish(' -- '*.go' ':!*_test.go'` の出力が 1 行）。
-   設計書 7.1 節のサーバの表が「など」でまとめた補助と、表にないテストのうち、旧 API か失敗の通知を参照するもの（`b8941e71` で `ArticlesPath`・`notifier`・`Notifier`・`postArticle` などを含む関数を洗い出した）:
    -   `internal/server/server_test.go` の `newServer`（`:34`、引数に `failure.Notifier`）と `postOnUnix`（`:72`、`ArticlesPath` へ送る）。`postOnUnix` は、フェーズ 3 まで残す `TestServeShutdownWaitsForJobs` と `TestServeShutdownCancelsGeneration` も使う。
    -   `internal/server/jobs_api_test.go` の `splitWriter`（`:74`、`TestStartGenerationSlotsShared` だけが使う）。
    -   `internal/server/status_test.go` の `TestStatusHasNoSideEffects`（`:137`、`ts.notifier.Calls` を確かめる）。
    -   `cmd/yt2column-server/run_test.go` の `noticeSignal`（`:346-`、`TestOutputsOmitSecrets` だけが使う）、`TestStartsWithoutWebhook`（`:453-`、`d.newNotifier` を設定し、`mustPostArticle` で `/articles` へ送って `webhook_not_configured` を確かめる）、`TestBuildsSlackPublisher`（`:391-`、`d.newNotifier` が `*publisher.SlackFailureNotifier` を作ることを確かめる）、`TestWriterOptionsMatchCLIDefaults`（`:425-`、コメントだけが失敗の通知に触れる）。
    -   `cmd/yt2column-server/signal_test.go` の `TestSignalWaitsForJob` は、旧 API のジョブのログ `job succeeded`（`internal/server/job.go:236`）を確かめる（`signal_test.go:89`）。投稿のジョブのログは `post job succeeded`（`internal/server/publish.go:180`）である。
    -   `cmd/yt2column-server/run_test.go` の `TestStartDuringShutdownRefused`（`:283`）は子プロセスではなく、同じプロセスの中でサーバを動かす。
    -   `internal/server/handler_test.go` の `TestRejectHeaders` の `path_encoded` の行（`:30`）は `/%61rticles` を送る。
-   旧 API と失敗の通知に触れるコメントのうち、設計書 3.2.1 節が挙げていないもの: `publish.go:34`（`publishHandler`）、`request.go:35-37`（`readRequest`）、`jobs_api.go:84`、`handler.go:40`（`matchTarget`）。
-   `TestPublishFailureNotNotified`（`publish_test.go:249`）は、失敗の通知を送らないことのほかに、ただのエラーが投稿の失敗の理由 `internal` になることと、`post job failed` のログが `reason=` を持つことを確かめる。この 2 つを確かめるテストはほかにない。
-   `TestPublishWithoutWebhook` の `with_body` の行（`publish_test.go:345-357`）は `Connection: close` だけを確かめ、本文を読まないことを確かめない。本文を読まないことを確かめているのは、削除する `TestArticlesWithoutWebhook` である。
-   `TestOutputsOmitSecrets` が今待っている失敗の通知は、生成のジョブの失敗のログより後に送られる。ジョブの API の状態の取得で読める失敗は、そのログより前に記録される（`internal/server/generation.go:120-136`）。

**`internal/failure` を import するファイル:** `cmd/yt2column-server` の `run.go`・`run_test.go`・`test_helpers.go`、`internal/publisher` の `slack_failure.go`・`slack_failure_test.go`、`internal/server` の `handler.go`・`job.go`・`job_test.go`・`request_test.go`・`server.go`・`server_test.go`・`test_helpers.go`、`internal/failure/testutil/mocks.go`（`git grep -l 'yt2column/internal/failure'`）。

**`internal/publisher`・`internal/pipeline`**

-   `SlackPostTimeout`（`slack.go:23`）、`slackMessageInterval`（`slack.go:28`）、`slackMaxMessages`（`slack_message.go:17`）がある。
-   `TestFakesCarryBuildTag`（`internal/pipeline/pipeline_test.go:577-578`）は testutil のファイルの数を `25` で固定している。`internal/failure/testutil/mocks.go` を削除するコミットで 1 つ減らす。その時点でほかのタスクがファイルを加えて数が変わっていれば、その時点の値から 1 つ減らす。
-   `TestPackageReferenceListsPackages`（`pipeline_test.go:629`）は、`package_reference.md` の行とパッケージの集合が一致することを確かめる。`internal/failure` と `internal/failure/testutil` の行は、パッケージを削除するコミットで除く。

**`cmd/yt2column-server`**

-   削除・変更するものは設計書 3.5 節のとおりである。停止の案内は `stopNoticeLogger`（`run.go:238-243`）、起動時の警告は `run.go:195` にある。
-   `docs_test.go` の curl の例のガードは `server.ArticlesPath` の行（`:107`）を持ち、`pathForm`（`:393-`）が `server.ArticlesPath` を参照する。`TestServerDocsContract` の `security.md` のサブテスト（`:238-`）は、§8.2 に生成の枠の数（`server.MaxInFlightForTest`）と本文の上限が書かれていることを確かめる（AC-21）。
-   子プロセスのテストのモード `childModeBlock`（`test_helpers.go:72-75`）は、LLM の呼び出しで止まる `blockingLLM`（`:513`）を使う。子のサーバの環境（`newServerEnv`、`test_helpers.go:116-124`）は、今も `SLACK_WEBHOOK_URL` を設定している。

**文書を読むガード**

-   `cmd/yt2column-server/docs_test.go::TestServerDocsContract` は、README の「localhost サーバ」節、`development.md` の HTTP API の節（curl の例と拒否の理由の表）、`project_overview.md`、`security.md` §8.2 を読む。
-   拡張の `serverReasons.test.ts`・`metadataDocs.test.ts` は `development.md` の拒否の理由の表と README の拡張の節を読む。`extension/node_modules` はこの作業ツリーにあり、`node --version`（`v24.21.0`）は `extension/.node-version` と一致する。

**確かめの道具**

-   `docs/tasks/0052_doc_test_pruning/_checks/coverage/main.go` は、2 つの `go tool cover -func` の出力を (ファイル, 行, 名前) で比べる。製品のコードを変えないコミットにそのまま使える。
-   `go tool cover -func` はメソッドの名前をレシーバなしで出す。`internal/server` の製品のコードには `notify` が 2 つある（`job.go:87` の関数と `response.go:97` のメソッド）。

**型による保証の選び方:** 本タスクは具体的な型で振る舞いを分けない。`destination` の `form` を除き、`request` は `Operation` の値だけで分ける（設計書 3.4.1 節）。ゼロ値の `Operation` は `request` の `default` が拒否し、`TestRequestRefusesUnroutable` で確かめる（ステップ 1-3）。

### 1.5. 名前の変更の一覧

本書のほかの箇所の名前は、この表に従う。

| 種類 | 今の名前 | 新しい名前 | ステップ |
|---|---|---|---|
| テスト | `internal/server/publish_test.go::TestPublishPostsLikeArticles` | `TestPublishPostsLikeCLI` | 2-1 |
| テスト | `internal/server/jobs_api_test.go::TestStartGenerationSlotsShared` | `TestStartGenerationSlotsFull` | 2-1 |
| テスト | `internal/server/publish_test.go::TestPublishFailureNotNotified` | `TestPublishFailureReason` | 3-2 |
| サブテスト | `cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd/invalid_transcript_notifies_failure` | `invalid_transcript_posts_nothing` | 1-2 |
| ログの属性 | シャットダウンの待ち始めの行の `pending` | `canceledGenerations`（設計書 3.2.2 節） | 3-2 |
| ファイル | `internal/server/job.go`・`job_test.go` | 削除（中身の移し先は 1.3 節） | 3-2 |

### 1.6. 実装の際に決めたこと（2026-11-04）

-   **削除と移行が済んだことの確認は一度だけ行い、永続のテストにしない（設計書 1.4 節）:** 永続のテストには、新しく設計しても書く、最終的な仕様の規則と値だけを置く。削除したもの・移行前の形がもうないことの確認は、`_checks/residue` と PR の時点の確認で一度だけ行う。PR-1 のレビューの中で、レビュアーと決めた。理由: 削除したものが戻る経路はコードを書き戻すことだけであり、その不在を永続のテストで固定すると、退役した仕様をテストが持ち続ける。この決定で、次のように変えた。
    -   **PR-1:** 1.3 節にあった「統合テストに、今の形のメッセージが拒否される場合を加える」をやめた。ステップ 1-3 で加えた `TestRelayEndToEnd/message_without_version_is_rejected` とその補助 `runHostLog`、ログに `form=` が出ないことの確かめ（`requireNoFormAttr`）、#503 より前の拡張のメッセージを入力にした行を削除した。`TestRelayRejectsOtherForms` に残る規則の行（`v` がない、厳格な JSON オブジェクトでない 5 種類）は `TestRelayNewFormRejections` の表に移し、ログに値を書かないことの行は `TestRelayNewFormLogOmitsMessage` に移した。
    -   **PR-2:** `/articles` 専用の拒否のテスト、警告・停止の案内・シャットダウンのログが `/articles` と失敗の通知に触れないことのテストは書かない。`/articles` の不在と、テストでないコードの文字列リテラルの語は `_checks/residue`（ステップ 3-4）で確かめる。未知のパスが本文を読まず枠を取らないことは、最終的な仕様として `/articles` ではない未知のパスで確かめる（ステップ 3-2）。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: ホストの今の形の中継の削除（設計書 8 章の 1）

#### ステップ 1-1: 確かめのスクリプト

**変更するファイル:** `docs/tasks/0059_remove_legacy_articles_api/_checks/coverage/main.go`（新規）、`docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go`（新規）

- [x] `_checks/coverage` を、1.3 節「カバレッジの比べ方」の削除のコミットの比べ方のとおりに作る。引数は 2 つのコミットと、それぞれの `go tool cover -func` の出力である。
- [x] `_checks/residue` を作る。引数は merge-base と head の 2 つのコミットである。PR-1 の確かめは次の 2 つで、確かめごとに結果を出し、すべて成り立てば `0`、成り立たないものがあれば `1`、実行できなければ `2` で終わる。
    -   AC-13: `internal/nmhost` に `articlesURL`・型 `form`・`formLegacy`・`formNew`・`destination` のフィールド `form` の宣言がなく、テストでないファイルに `/articles` を含む文字列リテラルがない（1.4 節）。
    -   AC-12: `extension/` が merge-base と head の間で変わっていない。
- [x] 2 つのスクリプトの冒頭のコメントに、確かめる内容、実行の仕方、終了コードを英語で書く。`make test` と CI はこれらを実行しない。
- [x] `_checks/coverage` に、手で作った入力を与えて確かめる。片方にだけある関数、カバレッジの変わった関数、ほかのファイルへ移った関数（変わらないと扱う）、同じパッケージで同じ名前の 2 つのメソッド（レシーバで区別する）、対応が 1 つに決まらない場合（`2` で止まる）である。
- [x] スクリプトだけをコミットする（製品のコードもテストも変えない）。

**完了の条件:** `go vet -tags test` が 2 つのスクリプトを通し、`_checks/residue` に merge-base を head として渡すと、AC-13 の確かめが成り立たないと出力して `1` で終わる。

#### ステップ 1-2: ホストのテストの移し替え（製品のコードを変えない）

**変更するファイル:** `internal/nmhost/relay_test.go`、`cmd/yt2column-nmhost/main_test.go`、`cmd/yt2column-server/relay_e2e_test.go`

- [x] `TestRelayForwardsOneRequest` を新しい形のメッセージで作り直す（設計書 7.1 節）。`TestRelayNewFormOperations` の `start` の行（`relay_test.go:839-842`）と重なる確かめは除き、重ならない性質（正しい UTF-8 でないサーバの応答をそのまま中継することなど）に絞る。
- [x] `TestRelayServerEarlyRejection`・`TestRelayHostErrors`・`TestRelayOutputs`・`TestRelayNoReplyFrame`・`TestRelayMessageReadFailures`・`TestRelayLengthChecks` の入力を、新しい形のメッセージに移す（設計書 7.1 節）。移した各行は、届くはずの段階のログの `stage=` を確かめる（`protocol_error` を期待する行は、形の検査の `stage=form` でないことも確かめる）。サーバと交換する行は、サーバがリクエストを 1 つ受け取ったことも確かめる。
- [x] 一時的に `route` がすべてのメッセージを拒否するようにして、移したテストがすべて失敗することを確かめ、コミットメッセージに書く（変更は残さない）。
- [x] `TestRelayUsesDerivedSocket` の `{}` のメッセージを、新しい形のメッセージに移す。
- [x] `TestRelayEndToEnd` の `accepted_job_continues_after_host_exits`・`stopped_server_is_unavailable`・`body_over_server_limit_is_rejected_by_server` を新しい形に移す（設計書 7.2 節）。
- [x] `TestRelayEndToEnd/invalid_transcript_notifies_failure` を `invalid_transcript_posts_nothing` に置き換える。新しい形の生成の開始で字幕が正しくない場合に、ホストの応答が `400` の `invalid_transcript` であること、サーバを止めた後（停止はジョブと失敗の通知を待つ）に Webhook が何も受け取っていないことを確かめる（設計書 7.2 節、AC-02）。生成の開始の拒否が失敗の通知を送るようにする一時的な変更で、このサブテストが失敗することを確かめる。
- [x] `TestRelayLegacyBytesUnchanged` など、今の形だけを確かめるテストは、このステップでは残す。
- [x] 親コミットとこのコミットで、1.3 節「カバレッジの比べ方」の移し替えのコミットの比べ方で比べ、結果をコミットメッセージに書く。

**完了の条件:** `make test` と `make lint` が通り、カバレッジの比較が 1.3 節の基準を満たす。

#### ステップ 1-3: 形の判定の削除

**変更するファイル:** `internal/nmhost/relay.go`、`internal/nmhost/frame.go`、`internal/nmhost/relay_test.go`、`cmd/yt2column-server/relay_e2e_test.go`、`.golangci.yml`

- [x] `route` を設計書 3.4.1 節のとおりに書き換える。`ParseObject` に通らなければ `ruleObject`、版の検査を `Collect("v")` でメンバの検査より先に行う。`rule` に `ruleObject` を加え、`ruleVersion` のコメントを設計書 3.4.1 節のコードのとおりに直す。
- [x] 1.4 節の削除するもの（`articlesURL`、型 `form`、`formLegacy`・`formNew`、`destination.form`、`request` の `form` による分岐、`logAttrs` の `form` 属性）を削除する。`request` は `op` だけで分け、`default` は `errUnroutable` を返す。
- [x] 設計書 3.4.3 節が挙げる説明（`forward`・`Relay`・`route`・`bodyRequest`・`stageForm`・`destination` のコメント、`frame.go:3-6` のパッケージの説明、`.golangci.yml` の `nmhost-boundaries` の説明）から、今の形との対比を除く。
- [x] `TestRelayLegacyBytesUnchanged` を削除する。同じ入力が送られずに `protocol_error` になることは、受理の規則として `TestRelayNewFormRejections` の行で確かめる。各行で、ログに `stage=form` と期待する `rule=` があること（肯定の印）と、サーバへ接続できる状態でもサーバがリクエストを受け取らないことを確かめる（AC-10・AC-11）。ログにメッセージの値がないことは、`TestRelayNewFormLogOmitsMessage` に `rule=version` と `rule=object` の行を加えて確かめる（AC-14）。行には次を含める（1.6 節）。
    -   `rule=version`: `v` だけを欠き、`op` と `key` はそろっているメッセージ（AC-10 の「`v` 以外のメンバがそろっていても」）、`v` が最上位にないメッセージ。
    -   `rule=object`: AC-11 の 5 種類（正しくない UTF-8、JSON として正しくない、最上位がオブジェクトでない、後続のデータ、対になっていないサロゲートのエスケープ）。
- [x] `TestRelayNewFormOperations`・`TestRelayNewFormLogOmitsMessage`・`TestRelayNewFormRejections` から `form=` の確かめを除く（`form=` が出ないことは永続のテストにしない。1.6 節）。`form=new` を肯定の印にしていた確かめ（`TestRelayNewFormLogOmitsMessage`、`relay_test.go:1073`）は、`op=` などの別の肯定の印に替える。`TestRelayNewFormRejections` に、版の検査がメンバの検査より先にあることを確かめる 2 行を加える（設計書 7.1 節）。既存の行の期待する値は変えない。
- [x] `TestRequestRefusesUnroutable` を、`op` だけを持つ送り先（ゼロ値、範囲の外の値）で `request` が何も送らずに拒否することを確かめる形に作り直す（設計書 7.1 節）。
- [-] `TestRelayEndToEnd` に、#503 より前の拡張が送る形のメッセージが拒否されることのサブテストを加える。加えた後に、1.6 節の決定で削除した（移行の確認は永続のテストにしない）。
- [x] 加えたテストの行が、それぞれの規則の検査を外すと失敗することを確かめ、コミットメッセージに書く。
- [x] ステップ 1-2 のコミットとこのコミットで、1.3 節「カバレッジの比べ方」の削除のコミットの比べ方で比べ、結果をこのコミットのメッセージに書く。コミットの後で比べるので、push の前に `git commit --amend` でメッセージだけを直し、内容が変わらないことを `git diff HEAD@{1} HEAD` で確かめる（ステップ 1-2・2-3・3-3 の比較も同じ）。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。

#### ステップ 1-4: ホストの文書

**変更するファイル:** `CLAUDE.md`、`docs/dev/security.md`、`docs/dev/conventions/package_reference.md`

- [x] `CLAUDE.md` の「Declare, don't infer」から、ホストが今の形と新しい形をメッセージの中身で選ぶ例外の段落（`:357-375`）を取り除く（AC-19 の前半）。
- [x] `docs/dev/security.md` §8 のホストの記述（`:143-144`）を、新しい形だけを受理し、ほかはすべて送らずに `protocol_error` にすることに直す（AC-18）。
- [x] `package_reference.md` の `internal/nmhost` の行（`:50`）から、今の形の中継と属性 `form` の記述を除き、規則 `object` を加える。
- [x] 文書のガードと `make ext-test` を通す。

**完了の条件:** 文書のガード、`make test`、`make ext-test` が通る。

#### ステップ 1-5: ホストの静的な確認の実行と記録

**変更するファイル:** 本書 7.1 節

- [x] PR-1 のすべての変更をコミットした後、PR のブランチで `_checks/residue` を merge-base（`git merge-base origin/main HEAD`）と `HEAD` で実行し、出力、終了コード、2 つのコミット ID を 7.1 節に書く。merge-base を head として渡すと AC-13 の確かめが成り立たないことも書く。
- [x] AC-12 の確かめは merge-base でも成り立つので、`extension/` の 1 行を変えた一時的なコミット（ブランチに載せない）を head として渡し、その確かめが成り立たないと出ることを確かめ、7.1 節に書く。
- [x] ステップ 1-2・1-3 のカバレッジの比較の結果を 7.1 節に書き写す（squash でマージされるとブランチのコミットメッセージは `main` から辿れないため）。

**完了の条件:** `_checks/residue` が PR の head で `0` を返し、その記録が 7.1 節にある。

### PR-1 作成ポイント: host legacy-form relay removal

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `refactor(0059): ホストの今の形のメッセージの中継を削除する`

**レビュー観点**: `v` を持たないメッセージと厳格な JSON オブジェクトでないメッセージが送られずに `protocol_error` になり、ログに固定の値だけが出ること / 版の検査をメンバの検査より先にしたことによる規則の値の変化が設計書 3.4.1 節のとおりであること / 移し替えたテストが対象とする段階まで届いていること（`stage=` の確かめと、`route` を壊した確認） / CLAUDE.md の例外の段落の削除

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-3 がホストの入力の境界（信頼できないブラウザからのメッセージについて、ホストが受理する形）を変える、セキュリティの境界の変更である（mkplan のパネルモードの条件に当たる）

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/654
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: サーバのテストの移し替え（設計書 8 章の 2 のうち、製品のコードを変えない部分）

このフェーズは製品のコードを変えない。移し替えたテストは、今の製品のコードで通る。旧 API と失敗の通知だけを確かめるテストと、その補助（`postOnUnix`、`serveAccepted`、`postArticle` など、まだ使う所があるもの）は、このフェーズでは残す（設計書 7.1 節「テストの削除の確かめ方」の 1）。

#### ステップ 2-1: `internal/server` のテスト

**変更するファイル:** `internal/server/handler_test.go`、`request_test.go`、`response_test.go`、`server_test.go`、`jobs_api_test.go`、`publish_test.go`、`test_helpers.go`

- [x] `TestRejectHeaders`・`TestAcceptsWithoutBrowserHeaders`・`TestRejectStatusCodes` を、ジョブの API のリクエストで組み立て直す。`TestRejectStatusCodes` の `webhook_not_configured` の行は投稿の操作のリクエストで組み立て（設計書 3.2.3 節）、`TestRejectHeaders` の `path_encoded` の行は、ジョブの API のパスを符号化したもの（`/%6Aobs` など）に替える。補助の `validHTTPRequest` も直す。既存の行の期待する理由と状態コードは変えない。
- [x] `request_test.go` の各テストに、生成の開始で同じ検査を確かめる版を加える。`TestStartRejectsLikeArticles` は、検査の表のテストに生成の開始の行として統合する（設計書 7.1 節）。`TestAcceptValidRequest` と `TestUnknownMemberIgnored` には加えない（生成の開始では `TestStartAccepted` と `TestStartKeyEquivalence` が同じことを確かめている）。`/articles` で確かめる版（失敗の通知の種類の確かめを含む）は、ステップ 3-2 で削除する。
- [x] `TestResponseOmitsInputs` と補助の `rejectRequest` を、ジョブの API のリクエストに移す。
- [x] `TestExpectContinueOversizedBodyNotSent`・`TestUnparsableRequestBypassesHandler`・`TestServeOnUnixListener`・`TestServeListenerFailureStops` を、ジョブの API で確かめ直す。ソケットへ送る補助は、ジョブの API へ送るものを別に加える（`postOnUnix` は残すテストが使うので、ステップ 3-2 で削除する）。
- [x] `TestNewCopiesWebhookOptions` を、投稿の操作で、渡した後に書き換えた `WebhookOptions` が使われないことを確かめる形に作り直す。
- [x] `TestPublishWithoutWebhook` の `with_body` の行で、本文を読んだバイト数が 0 であることも確かめる（削除する `TestArticlesWithoutWebhook` が確かめていた性質。1.4 節）。
- [x] `TestStartGenerationSlotsShared` を `TestStartGenerationSlotsFull` に改め、2 つの生成のジョブで生成の枠を埋める形にする（1.5 節）。使う所がなくなる `splitWriter` は削除する。生成のジョブだけが枠を取ることは、`publish_test.go` の `TestPostJobTakesNoGenerationSlot`（投稿のジョブが送信を待つ間、生成の枠が空いていて、枠の数だけ生成を始められる）で確かめる。
- [x] `TestPublishPostsLikeArticles` を `TestPublishPostsLikeCLI` に改め、同じ記事を `SlackWebhookPublisher.Publish` で同じループバックの Webhook へ送ったメッセージと、投稿の操作が送るメッセージが一致することを確かめる（設計書 7.1 節）。

**完了の条件:** `go test -tags test ./internal/server/` が通る。

#### ステップ 2-2: `cmd/yt2column-server` のテスト

**変更するファイル:** `cmd/yt2column-server/signal_test.go`、`run_test.go`、`server_test.go`、`test_helpers.go`

- [x] 子プロセスのモード `childModeBlock` を、印のファイルができるまで `PostMessages` が返らない投稿の相手を使う形にする。待ちには、今の `blockingLLM` と同じく上限の時間を付ける。`blockingChild`・`TestSignalWaitsForJob`・`TestSecondSignalExitsAtOnce` は、生成を終えさせ、投稿の操作を送り、送信が始まったのを待ってからシグナルを送る（設計書 7.1 節）。`TestSignalWaitsForJob` が確かめるログは、`job succeeded` から `post job succeeded` に替える。
- [x] `TestStartDuringShutdownRefused`（同じプロセスの中のテスト）は、チャネルで止める偽の投稿の相手で、投稿のジョブが終わるまで次のサーバが起動を断ることを確かめる形にする。
- [x] `TestServerEndToEnd`・`TestOutputsOmitSecrets` を、ジョブの API（生成の開始・状態の取得・投稿の操作）で確かめ直す。`TestOutputsOmitSecrets` は、状態の取得で失敗を読んだ時点ではなく、失敗のログの行が書かれたのを待ってから止める（1.4 節）。使う所がなくなる `noticeSignal` は削除する。
- [x] `TestStartsWithoutWebhook` の拒否の確かめを、投稿の操作が `webhook_not_configured` を返すことに替える（設計書 7.1 節）。失敗の通知の送り手を作らないことの確かめ（`d.newNotifier`）は、ステップ 3-2 まで残す。
- [x] 使う所がなくなった `postArticle`・`mustPostArticle` を削除する。`blockingLLM` も使う所がなくなれば削除する。`requestBody` は `relay_e2e_test.go` が使うので残す。
- [x] 移し替えた停止のテストが、投稿の相手が返る前にはサーバが終わらないことを確かめていることを、投稿の相手の待ちを外すと失敗することで確かめ、コミットメッセージに書く。`go test -count=20 -run 'TestSignal|TestStartDuringShutdownRefused|TestOutputsOmitSecrets'` を手元で実行し、不安定さが出ないことを確かめる。

**完了の条件:** `go test -tags test ./cmd/yt2column-server/` が通る。

#### ステップ 2-3: カバレッジの比較とコミット

**変更するファイル:** なし（コミットメッセージだけ）

- [x] ステップ 2-1・2-2 の変更を 1 つのコミットにする（製品のコードを含まないことを `git diff --stat` で確かめる）。使う所がなくなった補助と型が残っていないことを `make lint` で確かめる。
- [x] 親コミットとこのコミットで、1.3 節「カバレッジの比べ方」の移し替えのコミットの比べ方で比べ、結果をコミットメッセージに書く。

**完了の条件:** `make test`・`make lint` が通り、カバレッジの比較が 1.3 節の基準を満たす。

### フェーズ 3: サーバの旧 API と失敗の通知の削除（設計書 8 章の 2 のうち、コード）

#### ステップ 3-1: `SlackMaxPostDuration`

**変更するファイル:** `internal/publisher/slack.go`、`internal/publisher` のテスト、`cmd/yt2column/makefile_test.go`

- [x] `SlackMaxPostDuration` を、`slackMaxMessages`・`SlackPostTimeout`・`slackMessageInterval` から定義して加える（設計書 3.3 節、1.3 節）。
- [x] `cmd/yt2column/makefile_test.go:73` の `SlackPublishMaxDurationForTest(SlackMaxMessagesForTest)` を `SlackMaxPostDuration` に替え、`internal/publisher` のテストで、最大の通数での `SlackPublishMaxDurationForTest` と `SlackMaxPostDuration` が一致することを確かめる（1.3 節）。

**完了の条件:** `make test` と `make lint` が通る。

#### ステップ 3-2: `/articles`・旧 API のジョブ・失敗の通知の配線の削除（1.3 節のコミット (a)）

**変更するファイル:** `internal/server/` の `handler.go`・`job.go`（削除）・`jobs_api.go`・`generation.go`・`request.go`・`response.go`・`server.go`・`publish.go` とそのテスト・`test_helpers.go`、`internal/serversock/serversock.go`、`cmd/yt2column-server/` の `run.go` とそのテスト・`test_helpers.go`・`docs_test.go`

- [x] `handler.go` から `ArticlesPath`・`routeArticles`・`matchTarget` の `/articles` の分岐・`articlesHandler` を削除する（設計書 3.2.1 節）。
- [x] `maxInFlight`・`acquireSlot`・`releaseSlot` を `jobs_api.go` へ、`classifyWrite` を `generation.go` へ移し、`job.go` を削除する（1.3 節）。
- [x] `request.go`・`response.go`・`server.go`・`publish.go` から、設計書 3.2.1 節の表の残りの行（`rejectReason.notify`、`rejection.videoID`、`detailTooManyRequests`、`jobInput.requestID` と `requestID` の引数、`WebhookOptions.Notifier`・`Server.notifier`・`errNilWebhookNotifier` とその検査、`WebhookPublisher` の `publisher.Publisher`）を削除する。
- [x] シャットダウンのログを設計書 3.2.2 節のとおりにする（`genCancel` の前の生成の枠の使用数を `canceledGenerations` に書き、`postJobs` は残し、待ち始めと待ち終えた行から失敗の通知を除く）。
- [x] 設計書 3.2.1 節・3.2.2 節・3.2.3 節が挙げるコメント、1.4 節のコメント（`publish.go:34`、`request.go:35-37`、`jobs_api.go:84`、`handler.go:40`）、`internal/serversock/serversock.go:273-274` のコメントから、旧 API と失敗の通知を除く（AC-08）。
- [x] `cmd/yt2column-server/run.go` を設計書 3.5 節のとおりにする（`deps.newNotifier`・`newNotifier`・`llmCallsPerJob` の削除、`server.WebhookOptions{Publisher: pub}`、起動時の警告、コメント）。停止の案内は、`SlackMaxPostDuration` を 1.3 節の分を求める関数に通して書く。
- [x] 分を求める関数の境界の値を、表のテストで確かめる（1.3 節）。
- [x] `internal/server/job_test.go` を削除する（11 のテストはすべて削除の対象。設計書 7.1 節）。
- [x] `handler_test.go` の `TestArticlesWithoutWebhook` を削除する。
- [x] `jobs_api_test.go` の `TestArticlesIgnoresCache` を削除する。
- [x] `jobs_api_test.go` の `TestNewAPINeverNotifies` を削除する。
- [x] `server_test.go` の `TestServeShutdownWaitsForJobs` を削除する。
- [x] `server_test.go` の `TestBodyReadFailureWritesNoResponse` を削除する。
- [x] `request_test.go` の `/articles` で確かめる版（ステップ 2-1）を削除する。残る拒否の表（形の誤り、文字列でない任意のメンバ、動画の URL、字幕）は、同じ準備と確かめで入力だけが違うので、`TestStartRejectsBody` の 1 つの表にまとめる。`TestResponseOmitsInputs` は、移した後は `status_test.go` の `TestResponsesOmitInputs` と同じ入力を確かめるので削除する（`rejectRequest` は `TestRejectStatusCodes` が使うので残す）。
- [x] 使う所がなくなる補助 `postOnUnix`・`serveAccepted`・`panickingNotifier`・`nopNotifier`・`countingPublisher`・`nopPublisher.Publish` と、`newCustomServer`・`gateWriter`・`ctxRecordingWriter` を削除する。
- [x] 未知のパスの拒否のテスト（`jobs_api_test.go` の `TestJobsTargetRejections` の行か、並べたサブテスト。`TestUnknownPathTouchesNothing` として並べた）に、今のテストにない確かめだけを加える。`/articles` ではない未知のパスを使い、生成の枠と読み込みの枠をあらかじめすべて埋め、本文に読んだバイト数を数えるリーダーを渡したうえで、各メソッドで送り、`not_found` になること、本文を 1 バイトも読まないこと、Writer と投稿の相手が呼ばれないことを確かめる（AC-01）。`/articles` 専用の行は加えない（1.6 節。`/articles` の不在はステップ 3-4 の `_checks/residue` で確かめる）。
- [x] `TestServeShutdownCancelsGeneration` を、停止を始めた時点で実行中だった生成の数が `canceledGenerations=1` になることを確かめる形にする（行が失敗の通知に触れないことは、ステップ 3-4 の `_checks/residue` で確かめる。1.6 節）。`TestServeShutdownCancelsLateStart` で、停止の後に受け付けた生成が数に入らない（`canceledGenerations=0`）ことを確かめる（設計書 3.2.2 節、AC-16）。
- [x] `TestPublishFailureNotNotified` を `TestPublishFailureReason` に改め、失敗の通知の確かめだけを除く（失敗の理由 `internal` とログの `reason=` の確かめは残す。1.4 節）。
- [x] 失敗の通知の送り手を参照するテストと補助の値を直す: `TestStartHandlerPanicCleanup`・`TestPublishHandlerPanicCleanup`・`TestStatusHasNoSideEffects`・`TestNewWebhookOptions`・`newServer`・`newCustomServer`・`newTestServer`・`newJobsTestServer`。
- [x] `cmd/yt2column-server` のテストを直す: `TestStartupFailures` の失敗の通知の送り手の行を削除する。`TestBuildsSlackPublisher` から失敗の通知の送り手の確かめを除く。`TestStartsWithoutWebhook` から `d.newNotifier` を除く（AC-04）。`TestRunLogsListeningAndStop` を、停止の案内が「about 5 minutes」を含むことを確かめる形にする（AC-15、計算の式を使わずに文字列で確かめる）。警告と停止の案内が `/articles` と失敗の通知に触れないことは、ステップ 3-4 の `_checks/residue` で確かめる（1.6 節）。`TestWriterOptionsMatchCLIDefaults`・`testDeps`・`loopbackDeps` から失敗の通知を除く。
- [x] `cmd/yt2column-server/docs_test.go` の curl の例のガードから `server.ArticlesPath` の行を除き、`pathForm` から `server.ArticlesPath` を除く（設計書 3.6 節、AC-20）。
- [x] 加えたテストの行と書き換えた確かめが、それぞれ対象を壊して失敗することを確かめ、コミットメッセージに書く。

**完了の条件:** `make test` と `make lint` が通る。

#### ステップ 3-3: 使われなくなったものの削除と `depguard`（1.3 節のコミット (b)）

**変更するファイル:** `internal/failure/`（削除）、`internal/publisher/slack_failure.go`・`slack_failure_test.go`（削除）、`slack_errors.go`、`slack.go`、`publisher.go`、`test_helpers_slack.go`、`internal/pipeline/pipeline_test.go`、`docs/dev/conventions/package_reference.md`、`.golangci.yml`

- [x] `internal/failure` と `internal/failure/testutil` を削除する。
- [x] `internal/publisher/slack_failure.go` と `slack_failure_test.go` を削除する。
- [x] `slack_errors.go` から `ErrSlackInvalidNotice`・`ErrSlackNoticeRejected` を削除する。
- [x] `test_helpers_slack.go` から `NewSlackFailureNotifierForLoopbackTest`・`newSlackFailureNotifierForLoopbackTest` を削除する。
- [x] `slack.go:85` の `webhookClient` の説明と `publisher.go:3` のパッケージの説明から、失敗の通知を除く。
- [x] `TestFakesCarryBuildTag` の testutil のファイルの数を 1 つ減らす（1.4 節）。
- [x] `package_reference.md` の `internal/failure` と `internal/failure/testutil` の行を削除する（`TestPackageReferenceListsPackages`）。
- [x] `.golangci.yml` の `depguard` に、`internal/server` が `internal/pipeline` を import しないことの規則を加える（設計書 7.4 節）。一時的に `internal/server` に `internal/pipeline` の import を加えて `make lint` が失敗することを確かめ、コミットメッセージに書く。
- [x] ステップ 2-3 のコミットとこのコミットで、1.3 節「カバレッジの比べ方」の削除のコミットの比べ方で比べ（`job.go` から移した `acquireSlot`・`releaseSlot`・`classifyWrite` は名前で対応が取れること。定数 `maxInFlight` は `go tool cover -func` に現れない）、結果をこのコミットのメッセージに書く。比べるのはコミットを作った後なので、`git commit --amend` でメッセージだけを直し、内容が変わらないことを `git diff HEAD@{1} HEAD` で確かめる。push の前、ステップ 3-4 のコミットより前に行う。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。

#### ステップ 3-4: サーバの静的な確認の追加

**変更するファイル:** `docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go`

- [x] `_checks/residue` に、サーバの確かめを加える。
    -   AC-06: AC-06 が挙げる識別子（`cmd/yt2column-server` の `newNotifier`・`deps` のフィールド `newNotifier`・`llmCallsPerJob` を含む）が、それを定義していたパッケージ（`internal/server`・`internal/publisher`・`cmd/yt2column-server`）に宣言されておらず、ほかのパッケージから修飾子つきで参照されていない。どの Go のファイルも `internal/failure` と `internal/failure/testutil` を import しない。`internal/server` に `classifyWrite` が宣言されている。
    -   AC-07: `internal/server` の `WebhookPublisher` のメソッドが `PostMessages` だけで、埋め込みを持たない。`internal/server` と `cmd/yt2column-server` は、テストでないコードで `internal/pipeline` を直接にも間接にも import しない（各コミットのソースで `go list -deps` を実行して読む）。テストでないコードで `.Publish` のセレクタ式の呼び出しは `internal/pipeline` にだけある（メソッド値は、型の情報なしでは `articlecache` と状態の応答が持つフィールド `Publish` の読み出しと見分けられないので確かめない）。
    -   AC-02: `internal/server` と `cmd/yt2column-server` のテストでないコードで、`.PostMessages` のセレクタ式は `internal/server` の `runPublish` の中にだけある。
    -   AC-08（AC-01・AC-04・AC-15・AC-16 の移行の確認を兼ねる。1.6 節）: 1.3 節のとおり、`internal/server`・`cmd/yt2column-server`・`internal/serversock`・`internal/publisher` のテストでないファイルのコメントと文字列リテラルに、`/articles` と失敗の通知を指す語がない。`matchTarget` の `/articles` の分岐、起動時の警告、停止の案内、シャットダウンのログの文字列を含む。
- [x] 各確かめについて、merge-base を head として渡したときの結果を確かめる。merge-base で成り立つ確かめ（`classifyWrite` の宣言、`.Publish` のセレクタ式、`.PostMessages` のセレクタ式）は、それぞれを成り立たなくする一時的なコミット（ブランチに載せない）を head として渡して、成り立たないと出ることを確かめる。

**完了の条件:** `go vet -tags test` がスクリプトを通し、`_checks/residue` が `HEAD` で `0` を返す（記録はステップ 4-3）。

### フェーズ 4: 文書（設計書 8 章の 2 のうち、文書）

#### ステップ 4-1: 開発者の文書と CLAUDE.md

**変更するファイル:** `docs/dev/guides/development.md`、`docs/dev/security.md`、`docs/dev/project_overview.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/production_profile.md`、`docs/translation_glossary.md`、`CLAUDE.md`

- [ ] `development.md`: `/articles` の curl の例（`:77-`）を除き、開発用のサーバの確認の例（`:37`）を `GET /jobs`（`method_not_allowed`）に置き換える（設計書 3.6 節、AC-20）。`:46`（受け付けたリクエストが生成して投稿し、失敗の通知を投稿するという記述）と、HTTP API の節の `/articles` と失敗の通知の記述（`:107`・`:111`・`:114-115`・`:119`・`:153`・`:159`・`:168`）を、ジョブの API だけを前提にした記述に直す。生成の枠はジョブの API の生成のジョブだけが数え、同時に読む本文は最大 2 つ、停止のときに待つのは投稿のジョブで最長およそ 5 分と書く（AC-18）。
- [ ] `security.md` §8 のサーバの記述（`:136`・`:137`・`:140`・`:155-156`）から `/articles`・失敗の通知・二重の生成と投稿のリスクを除き、「新しい API」を「ジョブの API」と呼ぶ（AC-18、要件書 4.2）。
- [ ] `project_overview.md` の `:10`・`:109`（`internal/failure` の行）・`:147` と、`production_profile.md:40` から `/articles` と失敗の通知を除く。
- [ ] `package_reference.md` の `cmd/yt2column-server`・`internal/server`・`internal/publisher`・`internal/publisher/testutil` の行を、削除の後の形に直す（設計書 2.2 節）。
- [ ] `translation_glossary.md` の「生成の枠」「枠」「リクエスト ID」の行（`:45`・`:65`・`:96`）から旧 API と失敗の通知を除き、「新しい API」を「ジョブの API」にする。
- [ ] `CLAUDE.md` の Architecture Overview から `internal/failure` の記述（`:279-280`）を取り除く（AC-19 の後半）。
- [ ] 文書のガードと `make ext-test` を通す。

**完了の条件:** 文書のガードと `make ext-test` が通る。

#### ステップ 4-2: README

**変更するファイル:** `README.md`、`README.en.md`

- [ ] `README.md` の「localhost サーバ」節の `/articles` と失敗の通知の記述（`:187`・`:225`・`:249`）を直す。停止の説明は、待つのは投稿のジョブで最長およそ 5 分とする（AC-18）。版がそろっていない場合の説明（`:562-564`）を、要件書 4.4 の表の組み合わせと設計書 3.6 節のとおりに直す。
- [ ] `/mktrans README.md` で `README.en.md` に写し、2 つを同じコミットに入れる（CLAUDE.md「Translation Guidelines」）。
- [ ] `make readme-sync-check`、文書のガード、`make ext-test`、`go test -tags test ./cmd/yt2column-nmhost/`（`TestReadmeHostExitCodes`）を通す。

**完了の条件:** 上のテストがすべて通る。

#### ステップ 4-3: 静的な確認の実行と記録

**変更するファイル:** 本書 7.1 節

- [ ] PR-2 のすべての変更をコミットした後、PR のブランチで `_checks/residue` を merge-base と `HEAD` で実行し、出力、終了コード、2 つのコミット ID、merge-base と一時的なコミットでの結果（ステップ 3-4）を 7.1 節に書く。
- [ ] ステップ 2-3・3-3 のカバレッジの比較の結果を 7.1 節に書き写す。
- [ ] `make test`・`make lint`・`make deadcode`・`make ext-test` を通す。

**完了の条件:** `_checks/residue` が PR の head で `0` を返し、その記録が 7.1 節にある。

### PR-2 作成ポイント: server legacy API and failure notice removal

**対象ステップ**: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3

**推奨タイトル**: `refactor(0059): サーバの旧 API と失敗の通知を削除する`

**レビュー観点**: 移し替えた停止のテストが投稿のジョブを待つことを確かめていること（子プロセスとシグナル、不安定さ） / シャットダウンのログの `canceledGenerations` と停止の案内の「約 5 分」（分を求める関数の境界）が設計書 3.2.2 節・3.3 節のとおりであること / テストの移し替えと削除の前後のカバレッジの比較の記録 / 文書がジョブの API だけを前提に書き直され、README と `README.en.md` がそろっていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2 が子プロセスとシグナルを使う停止のテストを作り直し、ステップ 3-2 が停止の待ち方と並行して走るジョブの数え方を変える。多数のテストの更新を伴う削除で、パネルモードの条件に当たる。PR の数は設計書で 2 つと決まっているので、ステップ 3-2 は PR の中で独立させず、フェーズとコミットで分ける

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: ホスト | 確かめのスクリプト、テストの移し替え、形の判定の削除、ホストの文書、静的な確認の記録 | PR-1 がマージされた |
| M2: サーバのテストの移し替え | ジョブの API で確かめ直したテスト | フェーズ 2 のコミット（PR-2 の中） |
| M3: サーバの削除 | `/articles`・旧 API のジョブ・失敗の通知・`internal/failure` の削除、`SlackMaxPostDuration`、`depguard` の規則、サーバの静的な確認 | フェーズ 3 のコミット（PR-2 の中） |
| M4: 文書 | 開発者の文書、CLAUDE.md、README と `README.en.md`、静的な確認の記録 | PR-2 がマージされた |

M1 は M2〜M4 に依存しない。設計書 8 章のとおり、ホストを先にすると、PR-2 の時点で `/articles` に届くのは `curl` だけになる。M2 は M3 の前に置く（1.2 節）。M3 は M2 で移したテストの上で削除する。M4 のうち、削除と同じコミットで直す文書は M3 に入る（1.3 節）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | 確かめのスクリプト、ホストのテストの移し替え、形の判定の削除、CLAUDE.md の例外の段落・`security.md` §8 と `package_reference.md` のホストの記述 | frontier-required |
| PR-2 | 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 | サーバのテストの移し替え、`/articles`・旧 API のジョブ・失敗の通知・`internal/failure` の削除、`SlackMaxPostDuration`、`depguard`、残りの文書と README | frontier-required |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどのテストで確かめるかは本書 7 章に示す。

-   **単体テスト:** 残る振る舞いを旧 API や今の形のメッセージで確かめていたテストは、ジョブの API や新しい形のメッセージで確かめ直す（ステップ 1-2・2-1・2-2）。旧 API と今の形だけを確かめるテストは削除する（ステップ 1-3・3-2）。確かめ直すテストは、対象とする段階まで届くことも、ログの `stage=` などの肯定の印で確かめる（設計書 7.1 節の前置き、1.2 節）。
-   **統合テスト:** `TestRelayEndToEnd` を新しい形に移し、字幕が正しくない場合に Webhook が何も受け取らないこと（AC-02）のサブテストを持たせる（ステップ 1-2）。今の形のメッセージが拒否されることは、単体テストの規則の行で確かめる（1.6 節）。
-   **削除と移行の確認:** 削除したもの・移行前の形がもうないことは、永続のテストにせず、`_checks/residue` と PR の時点の確認で一度だけ確かめる（1.6 節）。
-   **後方互換:** ジョブの API の受理・拒否・応答（AC-03）、新しい形の中継（AC-09）、CLI（AC-05）は、既存のテストが通ることで確かめる。移し替えで書き換えるテストでも、既存の行の期待する値は変えない。版がそろっていない組み合わせ（要件書 4.4）は、本タスクのホストが今の形を拒否すること（ステップ 1-3）とサーバが `/articles` を `not_found` にすること（ステップ 3-2）で確かめる。
-   **セキュリティ:** ホストの入力の境界は、ステップ 1-3 の拒否のテストで、標準ライブラリが黙って直す入力が送られずに拒否されることを確かめる。Webhook へ送る経路が投稿のジョブだけであることは、`_checks/residue`（AC-02・AC-07）と `depguard` の規則で確かめる（設計書 7.3 節）。
-   **テストの削除の確かめ:** テストの移し替えのコミットと削除のコミットのそれぞれで、1.3 節「カバレッジの比べ方」のとおりに比べる（ステップ 1-2・1-3・2-3・3-3）。
-   **壊して確かめる:** 加えたテストの行、作り直した停止のテスト、移したホストのテスト、`depguard` の規則、`_checks/` のスクリプトは、その対象を壊して失敗することを実装のときに確かめ、コミットメッセージに書く。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 移し替えたテストが、対象とする段階に届かないまま通る（新しい形のメッセージの誤りで形の検査で止まる） | ホストの交換の段階や停止の待ち方の確かめが失われる | 移した行はログの `stage=` を確かめ、`route` を壊して全部が失敗することを確かめる（ステップ 1-2）。停止のテストは、投稿の相手の待ちを外すと失敗することを確かめる（ステップ 2-2） |
| テストの削除で、残る関数の確かめが失われる | 削除と関係のない関数のカバレッジが下がる | 移し替えと削除のコミットを分け、それぞれでカバレッジを比べる（1.3 節）。カバレッジに現れない確かめ（ログの値、失敗の理由）は 1.4 節で洗い出し、残すテストに持たせる（ステップ 2-1・3-2） |
| 子プロセスの停止のテストや、ログを待つテストが不安定になる | CI が時々失敗する | 印のファイルで送信が始まったのを待ってからシグナルを送り、ログの行を待ってから止める（ステップ 2-2）。`-count=20` で手元で繰り返し実行する |
| PR-2 が大きく、レビューの見落としが起きる | 削除の漏れや文書の誤り | フェーズを移し替え・削除・文書に分け、削除を 2 つのコミットに分ける（1.3 節）。削除の漏れは `_checks/residue` と `make deadcode` で確かめる |
| `TestFakesCarryBuildTag` の数が、ほかのタスクのマージで変わる | ステップ 3-3 の値が古くなり、`make test` が失敗する | その時点の値から 1 つ減らす（1.4 節） |
| 0055 の PR-6 が先にマージされ、CLAUDE.md の削除の確認の規則が変わる | 1.3 節の確かめ方が規則に合わなくなる | その規則に従う（設計書 7.1 節）。変わった点を 7.1 節に書く |
| 予定の遅れ | N/A | 削除と書き換えが中心で、新しい仕組みは確かめのスクリプト 2 つだけである |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（実行するテストや `make` の対象）、`static`（コミットしたスクリプトや `make lint` による確かめ）、`diff`（`_checks/` のスクリプトで merge-base と PR の head を比べる）、`manual`（PR のレビューで読む）である。`_checks/residue` は `docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go` を指す。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | test・static | test: `internal/server/jobs_api_test.go` の未知のパスの拒否のテスト（`TestJobsTargetRejections` の行か、並べたサブテスト。本文を読まない、枠を取らない）。static: `_checks/residue`（`ArticlesPath` の不在、`/articles` を含む文字列リテラルの不在） | 3-2・3-4・4-3 |
| AC-02 | test・static | `cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd/invalid_transcript_posts_nothing`。static: `_checks/residue`（`.PostMessages` のセレクタ式が `runPublish` だけ、`WebhookPublisher` が `PostMessages` だけ）、`.golangci.yml` の `depguard` の規則（`make lint`） | 1-2・3-3・3-4・4-3 |
| AC-03 | test | 既存のジョブの API のテストが通ること: `internal/server/jobs_api_test.go`（`TestStartAccepted`・`TestStartReadSlotsFull`・`TestStartGenerationSlotsFull`・`TestJobsTargetRejections` など）、`publish_test.go`・`status_test.go`、`handler_test.go::TestRejectStatusCodes`、`cmd/yt2column-server/docs_test.go::TestServerDocsContract`（拒否の理由と状態コード） | 2-1・3-2 |
| AC-04 | test・static | test: `cmd/yt2column-server/run_test.go::TestStartsWithoutWebhook`・`TestStartupFailures`、`internal/server/publish_test.go::TestPublishWithoutWebhook`（本文を読まない）、`internal/server/server_test.go::TestNewWebhookOptions`。static: `_checks/residue`（警告の文字列が `/articles` に触れない） | 2-1・2-2・3-2・3-4 |
| AC-05 | test | `make test` の `cmd/yt2column` と `internal/pipeline` のテスト（`cmd/yt2column/run_test.go::TestRunSlackFailureHints` など、変えない） | 3-2 |
| AC-06 | static | `_checks/residue`（AC-06 の識別子・パッケージの不在、`classifyWrite` の存在）。`make deadcode` | 3-3・3-4・4-3 |
| AC-07 | static | `_checks/residue`（`WebhookPublisher` のメソッド、`internal/server`・`cmd/yt2column-server` の依存に `internal/pipeline` がないこと、`.Publish` のセレクタ式） | 3-4・4-3 |
| AC-08 | static・manual | static: `_checks/residue`（コメントと文字列リテラルの語。警告・停止の案内・シャットダウンのログを含む）。manual: PR-2 のレビューで、コメントの文の意味を読む | 3-2・3-4・4-3 |
| AC-09 | test | `internal/nmhost/relay_test.go::TestRelayNewFormOperations`・`TestRelayKeyPassedUnchecked`・`TestRelayNewFormRejections`・`TestRelayLengthChecks`・`TestRelayResponseOverLimit`・`TestRelayNewFormTimeout`、`cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd/new_form_operations` | 1-2・1-3 |
| AC-10 | test | `internal/nmhost/relay_test.go::TestRelayNewFormRejections`（`rule=version` の行と、版の検査の順序の 2 行） | 1-3 |
| AC-11 | test | `internal/nmhost/relay_test.go::TestRelayNewFormRejections`（`rule=object` の 5 種類の行） | 1-3 |
| AC-12 | test・diff | test: `extension/test/native.test.ts`（`protocol_error` を想定外の応答として表示する、`make ext-test`）。diff: `_checks/residue`（`extension/` が merge-base から変わっていない） | 1-4・1-5・4-3 |
| AC-13 | static・test | `_checks/residue`（`internal/nmhost` の今の形の値の宣言と、テストでないファイルの `/articles` の文字列の不在）、`internal/nmhost/relay_test.go::TestRequestRefusesUnroutable` | 1-3・1-5 |
| AC-14 | test | `internal/nmhost/relay_test.go::TestRelayNewFormLogOmitsMessage`（`rule=version`・`rule=object` の行を含む） | 1-3 |
| AC-15 | test・static | test: `cmd/yt2column-server/run_test.go::TestRunLogsListeningAndStop`（「about 5 minutes」）、ステップ 3-2 で加える分を求める関数の表のテスト。static: `_checks/residue`（停止の案内の文字列が `/articles` と失敗の通知に触れない） | 3-2・3-4 |
| AC-16 | test・static | test: `internal/server/server_test.go::TestServeShutdownCancelsGeneration`・`TestServeShutdownCancelsLateStart`（`canceledGenerations`）。static: `_checks/residue`（シャットダウンのログの文字列が `/articles` と失敗の通知に触れない） | 3-2・3-4 |
| AC-17 | test | `cmd/yt2column-server/signal_test.go::TestSignalWaitsForJob`・`TestSecondSignalExitsAtOnce`、`run_test.go::TestStartDuringShutdownRefused`、`internal/server/publish_test.go::TestServeShutdownWaitsForPublish`・`server_test.go::TestServeShutdownCancelsGeneration` | 2-2・3-2 |
| AC-18 | manual・test | manual: PR-1・PR-2 のレビューで AC-18 が挙げる文書を読む（同時に読む本文の数と約 5 分を含む。ガードを加えない理由は 1.3 節）。test: `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（値の部分）、`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（`package_reference.md` の行の集合）、`make readme-sync-check`（README の 2 つの版の同期） | 1-4・3-3・4-1・4-2 |
| AC-19 | manual | PR-1・PR-2 のレビューで CLAUDE.md の 2 か所を読む | 1-4・4-1 |
| AC-20 | test・manual | test: `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`development.md` のサブテスト、3 つの操作の curl の例）。manual: `/articles` の curl の例がないこと（ガードの行を除いた後は `pathForm` が `""` を返し、ガードでは見分けられない）と、サーバの確認の例が `GET /jobs` を使うこと（設計書 7.4 節） | 3-2・4-1 |
| AC-21 | test | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`rejection_reasons` と `security.md` のサブテスト）、`extension/test/serverReasons.test.ts`（`make ext-test`） | 4-1・4-2 |

要件書 4.5 の「`make deadcode` は何も報告しない」は、各フェーズの完了の条件で確かめる。

### 7.1. 実行の記録

（ステップ 1-5 と 4-3 で記入する。）

#### PR-1（ステップ 1-5、2026-11-04）

**`_checks/residue`:** merge-base `302a4b28f93a625a14a59e20f04eeceebebefd83`、head `d45efb5b530e277631aa7f93fdb1140bf09f4caa`（PR-1 の実装、レビューの修正、1.6 節の決定によるテストの整理を含む最後のコミット。この後のコミットは本書だけを変え、確かめの対象のファイルを変えない）。

```text
$ go run -tags test ./docs/tasks/0059_remove_legacy_articles_api/_checks/residue/ 302a4b28f93a625a14a59e20f04eeceebebefd83 d45efb5b530e277631aa7f93fdb1140bf09f4caa
OK   host
OK   extension
（終了コード 0）
```

-   merge-base を head として渡すと、AC-13 の確かめ（`host`）が成り立たず、終了コード `1` になる。`internal/nmhost/relay.go` の `articlesURL`・`form`・`formLegacy`・`formNew`・`destination.form` の宣言と、`"/articles"` を含む文字列リテラルの 6 件を挙げる。`extension` は `OK` である。
-   `extension/src/core/native.ts` に 1 行を加えた一時的なコミット `b602f059c988bb9060a9fc9dc01ef48a4a75891f`（ブランチに載せない）を head として渡すと、AC-12 の確かめ（`extension`）が `NG`（`extension/src/core/native.ts changed`）になり、終了コード `1` になる。`host` は `OK` である。

**カバレッジの比較:** どれも `go test -tags test -count=1 -coverpkg=./... -coverprofile=<file> ./...` を両方のコミットで実行し（どちらもテストがすべて通った）、`go tool cover -func` の出力を比べた。

| ステップ | 比べたコミット | スクリプト | 結果 |
|---|---|---|---|
| 1-2（テストの移し替え） | `28e460b0` → `4860cdb3` | 0052 の `_checks/coverage` | `OK`: 1292 の製品の関数すべてでカバレッジが同じ |
| 1-3（形の判定の削除） | `4860cdb3` → `e4d258d6` | 本タスクの `_checks/coverage` | 前だけ・後だけの関数なし、下がった関数なし。上がった関数が 1 つ（`internal/cmdrunner` の `signalGroup` 66.7% → 100.0%）。本タスクが変えないパッケージの、時間に左右される分岐である |

-   レビューの修正で `TestRelayForwardsOneRequest` を削除したコミット `db29be10` では、`internal/nmhost` の `go tool cover -func`（`-coverpkg=./internal/nmhost`）が削除の前後で関数ごとに同じであることを確かめた。
-   1.6 節の決定でテストを削除・統合したコミットでは、`internal/nmhost`（`-coverpkg=./internal/nmhost`）と `cmd/yt2column-server`（`-coverpkg=./...`）の `go tool cover -func` が、前後で関数ごとに同じであることを確かめた。
-   カバレッジを測る実行では、`internal/transcript` の `TestCommandExecutorDrainsStderr` が負荷のために 1 回ずつ失敗し、再実行で通った。本タスクと関係のない、時間に左右されるテストである。

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 7 章のすべての行の確かめが済み、7.1 節に `_checks/residue` の実行の記録がある。
-   **品質:** グリーンゲート（`make test && make lint`）、`make deadcode`、`make ext-test` が通る。テストの移し替えと削除の各コミットで、カバレッジの比較の結果が 7.1 節にある。
-   **セキュリティ:** ホストが新しい形でないメッセージをすべて送らずに拒否し（AC-10・AC-11）、サーバが Webhook へ送るのが投稿のジョブだけである（AC-02）。
-   **文書:** AC-18 が挙げる文書と CLAUDE.md が、削除の後の振る舞いだけを説明している。タスク文書（`docs/tasks/`）の過去の記述は変えない（要件書 2.3）。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`/runpr` が issue #526 を閉じる案内をする（本タスクの最後の PR のため）。
-   ジョブの API の失敗を Webhook へ通知する必要が生じたときは、別のタスクで、ジョブの API の失敗の理由から設計する（設計書 9 章）。

## 変更履歴

- 2026-11-04 承認（isseis、#651）
- 2026-11-04 セッション中の再承認：削除と移行が済んだことの確認を一度だけの確認にし、永続のテストには最終的な仕様の規則と値だけを置く（isseis、#654）
