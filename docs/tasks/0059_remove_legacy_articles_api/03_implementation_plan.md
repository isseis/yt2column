# 実装計画書：旧 API（`/articles`）とホストの今の形の中継の削除

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-04 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.2.1 節」などは設計書の節を指す。用語は要件書 6 章に従う。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-10 は、設計書 10 章がすべて記録している。

既存のコードについての記述は、コミット `b8941e71`（2026-11-04 の `main`）で確かめた。設計書が行番号を引いたコミット `38aee56f` から `b8941e71` までに、`internal/server`・`internal/nmhost`・`internal/publisher`・`internal/failure`・`internal/pipeline`・`internal/serversock`・`cmd/yt2column-server`・`cmd/yt2column-nmhost`・`README.md`・`docs/dev/guides/development.md`・`docs/dev/security.md`・`docs/dev/project_overview.md`・`docs/dev/production_profile.md`・`docs/dev/conventions/package_reference.md` は変わっていない（`git diff --stat 38aee56f b8941e71 -- <これらのパス>` の出力が空）。`CLAUDE.md` は `:414` の後に 8 行が加わっただけで、設計書が引く `:279` と `:357-375` は変わっていない。そのため、設計書の `file:line` はそのまま使える。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

localhost サーバの旧 API（`POST /articles`）・旧 API のジョブ・失敗の通知と、Native Messaging のホストの今の形のメッセージの中継を削除し、新旧の両対応のために複雑になった生成の枠・停止の待ち方・その説明を、ジョブの API だけを前提にした形に戻す。削除するもの、残すもの、変える振る舞いは設計書 2〜6 章で決まっている。本書は、それをどの順序で、どのコミットに分け、どのテストと確かめで検証するかを定める。

### 1.2. 実装の原則

-   PR は設計書 8 章のとおり 2 つにする。PR-1 はホスト（フェーズ 1）、PR-2 はサーバ（フェーズ 2〜4）である。
-   各 PR で、テストの移し替えを、製品のコードを変えないコミットとして削除より先に置く（設計書 1.3 節、7.1 節「テストの削除の確かめ方」）。テストの移し替えのコミットと削除のコミットのそれぞれで、カバレッジを前後で比べる（ステップ 1-2・1-3・2-3・3-4）。
-   削除の内容・残すもの・型の形・ログの属性・案内の計算は、設計書の節を参照し、本書では繰り返さない。ログと案内の文言、テストの表の行、確かめのスクリプトの細部は、コードを書くときに決める。
-   加えたテストと確かめのスクリプトは、それが確かめる対象を壊して失敗することを実装のときに確かめ、コミットメッセージにそう書く（CLAUDE.md「Testing Strategy」）。
-   Go のコメント・識別子・文字列リテラルは英語で書く（CLAUDE.md「Go Idioms」）。
-   各フェーズの終わりに、`make fmt`・`make test`・`make lint`・`make deadcode` を通す。PR-2 は README と `docs/dev/guides/development.md` を変えるので、`make ext-test` も通す（CLAUDE.md「Development Notes」）。

### 1.3. 計画の際に決めたこと（2026-11-04）

レビュアーに尋ねるべき未決の事項はなかった。設計書が計画に委ねた次の点は、結果が大きく変わらない選択なので、理由とともにここに記録する。

-   **`job.go` は削除し、残る 4 つを移す（設計書 3.2.1 節）:** `maxInFlight`・`acquireSlot`・`releaseSlot` は `jobs_api.go` の `maxReading` の隣へ、`classifyWrite` はただ 1 つの呼び出し元がある `generation.go` へ移す。`job_test.go` も削除する（設計書 7.1 節の表で全テストが削除の対象）。理由: 2 種類の枠の定義が 1 つのファイルに並び、`classifyWrite` が使う側と同じファイルに置かれる。`job.go` という名前は旧 API のジョブを指しており、残すと名前と中身が合わなくなる。
-   **`SlackMaxPostDuration` は `internal/publisher/slack.go` に置く（設計書 3.3 節）:** 3 つの元の定数のうち 2 つ（`SlackPostTimeout`・`slackMessageInterval`、`slack.go:23`・`:28`）と、それを使う `PostMessages` が `slack.go` にある。`slackMaxMessages`（`slack_message.go:17`）は同じパッケージなので参照できる。
-   **AC-02・AC-06・AC-07・AC-13 の静的な確認は、`_checks/` の Go のプログラム 1 つにする（設計書 7.4 節）:** `docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go` を PR-1 で作り（ホストの確かめだけ）、PR-2 でサーバの確かめを加える。標準ライブラリの `go/parser`・`go/ast` でパッケージと識別子を読んで確かめ、文字列の検索をしない（名前の衝突を避けるため。例えば `cmd/yt2column/docs_test.go:597` のローカル変数 `stageOf`）。PR ごとに 1 度、PR の head で実行し、結果を 7.1 節に記録する。同じプログラムで、`extension/` が merge-base から変わっていないことも確かめる（AC-12 の `diff` の確かめ）。理由: 文書に書いただけのコマンドは実行されない。0051 の `_checks/no_effort_compat.go` と同じく、Go だけで動く。
-   **削除のコミットのカバレッジの比較は、関数の名前で対応を取る新しいスクリプトで行う:** `_checks/coverage/main.go` を PR-1 で作る。2 つの `go tool cover -func` の出力を、(パッケージ, 関数の名前) で対応づけて比べ、片方にしかない関数と、網羅率が変わった関数を出力する。理由: 削除のコミットでは、残る関数の行番号がずれ、`job.go` の関数はほかのファイルへ移る。行番号で関数を識別する 0052 のスクリプト（`docs/tasks/0052_doc_test_pruning/_checks/coverage/main.go`）では、対応が取れない。
-   **テストの移し替えのコミットの比較は、既存の道具を使う:** 製品のコードが変わらないので、その時点の `main` に `make coverage-compare`（タスク 0055 の PR-5）があればそれを使い、なければ 0052 のスクリプトを使う。どちらも行番号まで一致することを確かめる。今の `main`（`b8941e71`）の `Makefile` には `coverage-compare` の対象はまだない。
-   **PR-2 の中の順序は、テストの移し替え（フェーズ 2）、コードの削除（フェーズ 3）、文書（フェーズ 4）とする（設計書 8 章）:** 削除と同じコミットで直さないと `make test` が失敗する文書（`package_reference.md` の `internal/failure` の 2 行。`TestPackageReferenceListsPackages`、`internal/pipeline/pipeline_test.go:629`）と、curl の例のガードの `server.ArticlesPath` の行（`cmd/yt2column-server/docs_test.go:107`、コンパイルが通らなくなる）は、フェーズ 3 に置く。ほかの文書の書き換えはフェーズ 4 にまとめ、文書のレビューをコードの差分から分ける。`development.md` の `/articles` の curl の例は、ガードの行を除いた後は一致を求められないだけで失敗はしない（`pathForm` が `""` を返す。`docs_test.go:393-400`）ので、フェーズ 4 で除く。
-   **フェーズ 3 の削除は 2 つのコミットに分ける:** (a) サーバと `cmd/yt2column-server` の `/articles`・旧 API のジョブ・失敗の通知の配線の削除（`WebhookOptions.Notifier` を除くと `cmd/yt2column-server` のコンパイルが通らなくなるので、同じコミットにする）、(b) どこからも使われなくなった `internal/failure`・`SlackFailureNotifier` とその周りの削除。どちらのコミットもグリーンゲートを通す。理由: 差分の大きさを分け、(b) が純粋な削除であることを読みやすくする。
-   **統合テストに、今の形のメッセージが拒否される場合を加える:** 設計書 7.4 節は AC-10 を統合テストでも確かめるとした。設計書 7.2 節の表にその行はないので、`TestRelayEndToEnd` に、#503 より前の拡張が送る形のメッセージを本物のホストへ送り、`protocol_error` が返り、サーバがリクエストを受け取らないことを確かめるサブテストを加える（ステップ 1-3）。

### 1.4. 既存コード調査結果

**ホスト（`internal/nmhost`、`b8941e71` で確かめた）**

-   削除するもの（設計書 3.4.1 節）: `articlesURL`（`relay.go:54-55`）、型 `form` と `formLegacy`・`formNew`（`:427-434`）、`destination.form`（`:451`）、`route` の `!object.Has("v")` の分岐（`:468-470`）、`logAttrs` の `form` 属性（`:514`）、`request` の `switch d.form`（`:529-545`）。`route` は今、`CollectOnly` を版の検査より先に呼ぶ（`:473-479`）。
-   `strictjson.Object.Collect`（`internal/strictjson/strictjson.go:90`）と `Has`（`:126`）は今ある。`Has` はほかのパッケージが使うので残る（設計書 3.4.1 節）。
-   今の形のメッセージを入力に使うテストは、設計書 7.1 節のホストの表と 7.2 節の表のとおりで、名前はすべて今のコードにある。

**サーバ（`internal/server`）**

-   削除するもの・残すものは設計書 3.2.1 節の表のとおりである。`job.go` に残るのは `maxInFlight`（`:22`）・`acquireSlot`（`:30-37`）・`releaseSlot`（`:40-42`）・`classifyWrite`（`:147-`）の 4 つで、どれも 1.3 節のとおり移す。
-   `internal/server` の中で `internal/pipeline` を import するのは `job.go:12` だけである（`git grep -n 'internal/pipeline"' -- internal/server cmd/yt2column-server` の出力が `job.go:12` の 1 行）。テストでないコードで `.Publish(` を呼ぶのは `internal/pipeline/pipeline.go:116` だけである（`git grep -n '\.Publish(' -- '*.go' ':!*_test.go'` の出力が 1 行）。
-   設計書 7.1 節のサーバの表が「など」でまとめた補助と、表にないテストのうち、旧 API か失敗の通知を参照するもの（`b8941e71` で `ArticlesPath`・`notifier`・`Notifier`・`postArticle` などを含む関数を数えた）:
    -   `internal/server/server_test.go` の `newServer`（`:34`、引数に `failure.Notifier`）と `postOnUnix`（`:75`、`ArticlesPath` へ送る）。
    -   `internal/server/status_test.go` の `TestStatusUnknown`（`:136`、コメントだけが notifier に触れる）。
    -   `cmd/yt2column-server/run_test.go` の `noticeSignal`（`:346-`、`TestOutputsOmitSecrets` が使う）と `TestWriterOptionsMatchCLIDefaults`（`:425-`、`d.newNotifier` を設定する）。
-   `internal/failure` を import するのは、`cmd/yt2column-server` の `run.go`・`run_test.go`・`test_helpers.go`、`internal/publisher` の `slack_failure.go`・`slack_failure_test.go`、`internal/server` の `handler.go`・`job.go`・`job_test.go`・`request_test.go`・`server.go`・`server_test.go`・`test_helpers.go`、`internal/failure/testutil/mocks.go` である（`git grep -l 'yt2column/internal/failure'`）。

**`internal/publisher`・`internal/pipeline`**

-   `SlackPostTimeout`（`slack.go:23`）、`slackMessageInterval`（`slack.go:28`）、`slackMaxMessages`（`slack_message.go:17`）がある。
-   `TestFakesCarryBuildTag`（`internal/pipeline/pipeline_test.go:577-578`）は testutil のファイルの数を `25` で固定している。`internal/failure/testutil/mocks.go` を削除するコミットで 1 つ減らす。その時点で 0055 などがファイルを加えて数が変わっていれば、その時点の値から 1 つ減らす。
-   `TestPackageReferenceListsPackages`（`pipeline_test.go:629`）は、`package_reference.md` の行とパッケージの集合が一致することを確かめる。`internal/failure` と `internal/failure/testutil` の行は、パッケージを削除するコミットで除く。

**`cmd/yt2column-server`**

-   削除・変更するものは設計書 3.5 節のとおりである。停止の案内（`run.go:238-243`）と起動時の警告（`run.go:195`）の今の文言は、それぞれ `stopNoticeLogger` と `logger.Warn` の行にある。
-   `docs_test.go` の curl の例のガードは `server.ArticlesPath` の行（`:107`）を持ち、`pathForm`（`:393-`）が `server.ArticlesPath` を参照する。`TestServerDocsContract` の `security.md` のサブテスト（`:109-`）は、§8.2 に生成の枠の数（`server.MaxInFlightForTest`）と本文の上限が書かれていることを確かめる（AC-21）。
-   子プロセスのテストのモード `childModeBlock`（`test_helpers.go:72-75`）は、LLM の呼び出しで止まる `blockingLLM`（`:519`）を使う。

**文書を読むガード**

-   `cmd/yt2column-server/docs_test.go::TestServerDocsContract` は、README の「localhost サーバ」節、`development.md` の HTTP API の節（curl の例と拒否の理由の表）、`project_overview.md`、`security.md` §8.2 を読む。
-   拡張の `serverReasons.test.ts`・`metadataDocs.test.ts` は `development.md` の拒否の理由の表と README の拡張の節を読む。PR-2 で `make ext-test` を通す。`extension/node_modules` はこの作業ツリーにあり、`node --version`（`v24.21.0`）は `extension/.node-version` と一致する。

**確かめの道具**

-   `docs/tasks/0052_doc_test_pruning/_checks/coverage/main.go` は、2 つの `go tool cover -func` の出力を (ファイル, 行, 名前) で比べる。製品のコードを変えないコミットにそのまま使える。
-   `go run ./docs/tasks/<task>/_checks/<dir>/` の形で `_` で始まるディレクトリのプログラムを動かせることは、0051 の `_checks/no_effort_compat.go` の実行の記録（0051 の計画書 7 章の AC-29 の行）で確かめられている。

**型による保証の選び方:** 本タスクは具体的な型で振る舞いを分けない。`destination` の `form` を除き、`request` は `Operation` の値だけで分ける（設計書 3.4.1 節）。ゼロ値の `Operation` は `request` の `default` が拒否し、`TestRequestRefusesUnroutable` で確かめる（ステップ 1-3）。

### 1.5. 名前の変更の一覧

本書のほかの箇所の名前は、この表に従う。

| 種類 | 今の名前 | 新しい名前 | ステップ |
|---|---|---|---|
| テスト | `internal/server/publish_test.go::TestPublishPostsLikeArticles` | `TestPublishPostsLikeCLI` | 2-1 |
| テスト | `internal/server/jobs_api_test.go::TestStartGenerationSlotsShared` | `TestStartGenerationSlotsFull` | 2-1 |
| テスト | `internal/server/publish_test.go::TestPublishFailureNotNotified` | `TestPublishFailureNotRetried`（確かめが残らなければ削除） | 3-2 |
| サブテスト | `cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd/invalid_transcript_notifies_failure` | `invalid_transcript_posts_nothing` | 1-2 |
| ログの属性 | シャットダウンの待ち始めの行の `pending` | `canceledGenerations`（設計書 3.2.2 節） | 3-2 |
| ファイル | `internal/server/job.go`・`job_test.go` | 削除（中身の移し先は 1.3 節） | 3-2 |

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: ホストの今の形の中継の削除（設計書 8 章の 1）

#### ステップ 1-1: 確かめのスクリプト

**変更するファイル:** `docs/tasks/0059_remove_legacy_articles_api/_checks/coverage/main.go`（新規）、`docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go`（新規）

- [ ] `_checks/coverage/main.go` を作る。2 つの `go tool cover -func` の出力を (パッケージ, 関数の名前) で対応づけ、前だけにある関数、後だけにある関数、網羅率が変わった関数を出力する。1 つのパッケージに同じ名前の関数が複数あるとき（レシーバの違うメソッド）の扱いは、対応を誤らない形を実装のときに決める。製品のコードの見分け方（`_test.go` と `test` で始まる build 制約のファイルを除く）は 0052 のスクリプトに倣う。
- [ ] `_checks/residue/main.go` を作る。リポジトリの Go のファイル（テストを含む、build 制約によらない）を `go/parser` で読み、`internal/nmhost` に `articlesURL`・型 `form`・`formLegacy`・`formNew`・`destination` のフィールド `form` がなく、`/articles` を含む文字列リテラルがないことを確かめる（AC-13）。あわせて、`extension/` が merge-base と head の間で変わっていないことを `git diff` で確かめる（AC-12）。確かめごとに結果を出し、すべて成り立てば `0`、成り立たないものがあれば `1`、実行できなければ `2` で終わる。
- [ ] 2 つのスクリプトの冒頭のコメントに、確かめる内容、実行の仕方、終了コードを英語で書く。`make test` と CI はこれらを実行しない。
- [ ] `_checks/coverage` に、片方にだけある関数と網羅率の変わった関数を含む 2 つの出力を与え、それぞれを報告することを確かめる。
- [ ] スクリプトだけをコミットする（製品のコードもテストも変えない）。

**完了の条件:** `go vet` が 2 つのスクリプトを通し、`_checks/residue` を merge-base で実行すると、ホストの確かめが成り立たないと出力して `1` で終わる。

#### ステップ 1-2: ホストのテストの移し替え（製品のコードを変えない）

**変更するファイル:** `internal/nmhost/relay_test.go`、`cmd/yt2column-nmhost/main_test.go`、`cmd/yt2column-server/relay_e2e_test.go`

- [ ] `TestRelayForwardsOneRequest` を、生成の開始の `input` のバイト列がそのまま送られることを確かめる形に作り直す（設計書 7.1 節）。
- [ ] `TestRelayServerEarlyRejection`・`TestRelayHostErrors`・`TestRelayOutputs`・`TestRelayNoReplyFrame`・`TestRelayMessageReadFailures` の入力を、新しい形のメッセージに移す（設計書 7.1 節）。交換の段階を確かめるテストは、サーバがリクエストを 1 つ受け取ったことも確かめる（設計書 7.1 節の前置き）。
- [ ] `TestRelayUsesDerivedSocket` の `{}` のメッセージを、新しい形のメッセージに移す。
- [ ] `TestRelayEndToEnd` の `accepted_job_continues_after_host_exits`・`stopped_server_is_unavailable`・`body_over_server_limit_is_rejected_by_server` を新しい形に移す（設計書 7.2 節）。
- [ ] `TestRelayEndToEnd/invalid_transcript_notifies_failure` を `invalid_transcript_posts_nothing` に置き換える。新しい形の生成の開始で字幕が正しくない場合に、ホストの応答が `400` の `invalid_transcript` で、Webhook が何も受け取らないことを確かめる（設計書 7.2 節、AC-02）。
- [ ] `TestRelayLegacyBytesUnchanged` など、今の形だけを確かめるテストは、このステップでは残す。
- [ ] このコミットの親とこのコミットで `go tool cover -func` の出力を取り、1.3 節の道具（`make coverage-compare` か 0052 のスクリプト）で比べ、関数ごとの結果が変わらないことを確かめる。結果をコミットメッセージに書く。

**完了の条件:** `make test` と `make lint` が通り、製品のコードの関数ごとの網羅率が親コミットと一致する。

#### ステップ 1-3: 形の判定の削除

**変更するファイル:** `internal/nmhost/relay.go`、`internal/nmhost/frame.go`、`internal/nmhost/relay_test.go`、`cmd/yt2column-server/relay_e2e_test.go`、`.golangci.yml`

- [ ] `route` を設計書 3.4.1 節のとおりに書き換える。`ParseObject` に通らなければ `ruleObject`、版の検査を `Collect("v")` でメンバの検査より先に行う。`rule` に `ruleObject` を加え、`ruleVersion` の注釈を設計書 3.4.1 節のコードのとおりに直す。
- [ ] 1.4 節の削除するもの（`articlesURL`、型 `form`、`formLegacy`・`formNew`、`destination.form`、`request` の `form` による分岐、`logAttrs` の `form` 属性）を削除する。`request` は `op` だけで分け、`default` は `errUnroutable` を返す。
- [ ] 設計書 3.4.3 節が挙げる説明（`forward`・`Relay`・`route`・`bodyRequest`・`stageForm`・`destination` の注釈、`frame.go:3-6` のパッケージの説明、`.golangci.yml` の `nmhost-boundaries` の説明）から、今の形との対比を除く。
- [ ] `TestRelayLegacyBytesUnchanged` を削除し、同じ入力が送られずに `protocol_error` になり、ログに `stage=form` と `rule=version`・`rule=object` を書き、メッセージのメンバ名と値を書かないことを確かめるテストに置き換える。行には、#503 より前の拡張が送る形のメッセージ（`rule=version`）と、AC-11 の 5 種類（正しくない UTF-8、JSON として正しくない、最上位がオブジェクトでない、後続のデータ、対になっていないサロゲートのエスケープ）を含める（AC-10・AC-11・AC-14）。どの行でも、サーバへの接続はできるが、サーバがリクエストを受け取らないことを確かめる。
- [ ] `TestRelayNewFormOperations`・`TestRelayNewFormLogOmitsMessage`・`TestRelayKeyPassedUnchecked`・`TestRelayNewFormRejections` から `form=` の確かめを除き、ログに `form=` が出ないことを確かめる。`TestRelayNewFormRejections` に、版の検査がメンバの検査より先にあることを確かめる 2 行を加える（設計書 7.1 節）。
- [ ] `TestRequestRefusesUnroutable` を、`op` だけを持つ送り先（ゼロ値、範囲の外の値）で `request` が何も送らずに拒否することを確かめる形に作り直す（設計書 7.1 節）。
- [ ] `TestRelayEndToEnd` に、#503 より前の拡張が送る形のメッセージを送り、`protocol_error` が返り、サーバがリクエストを受け取らないことを確かめるサブテストを加える（1.3 節、AC-10）。
- [ ] 加えたテストの行が、それぞれの規則の検査を外すと失敗することを確かめ、コミットメッセージに書く。
- [ ] ステップ 1-2 のコミットとこのコミットで `go tool cover -func` の出力を取り、`_checks/coverage` で比べる。前だけにある関数が削除したものだけであること、残る関数で網羅率が下がったものがあれば、その関数から削除した文で説明できることを確かめる。説明できなければテストを加える。結果をコミットメッセージに書く。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。

#### ステップ 1-4: ホストの文書

**変更するファイル:** `CLAUDE.md`、`docs/dev/security.md`、`docs/dev/conventions/package_reference.md`

- [ ] `CLAUDE.md` の「Declare, don't infer」から、ホストが今の形と新しい形をメッセージの中身で選ぶ例外の段落（`:357-375`）を取り除く（AC-19 の前半）。
- [ ] `docs/dev/security.md` §8 のホストの記述（`:143-144`）を、新しい形だけを受理し、ほかはすべて送らずに `protocol_error` にすることに直す（AC-18）。
- [ ] `package_reference.md` の `internal/nmhost` の行（`:50`）から、今の形の中継と属性 `form` の記述を除き、規則 `object` を加える。
- [ ] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview/... ./internal/pipeline`）を通す。

**完了の条件:** 文書のガードと `make test` が通る。

#### ステップ 1-5: ホストの静的な確認の実行と記録

**変更するファイル:** 本書 7.1 節

- [ ] PR-1 のすべての変更をコミットした後、PR のブランチで `_checks/residue` を merge-base（`git merge-base origin/main HEAD`）と `HEAD` で実行し、出力、終了コード、2 つのコミット ID を 7.1 節に書く。merge-base で `1` になる（ホストの確かめが成り立たない）ことも書く。
- [ ] `extension/` が変わっていないことの確かめは merge-base では成り立つので、`extension/` の 1 行を変えた一時的なコミット（ブランチに載せない）を head に渡して、その確かめが成り立たないと出ることを確かめ、7.1 節に書く。
- [ ] ステップ 1-2・1-3 のカバレッジの比較の結果を 7.1 節に書き写す（squash でマージされるとブランチのコミットメッセージは `main` から辿れないため）。

**完了の条件:** `_checks/residue` が PR の head で `0` を返し、その記録が 7.1 節にある。

### PR-1 作成ポイント: host legacy-form relay removal

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `refactor(0059): ホストの今の形のメッセージの中継を削除する`

**レビュー観点**: `v` を持たないメッセージと厳格な JSON オブジェクトでないメッセージが送られずに `protocol_error` になり、ログに固定の値だけが出ること / 版の検査をメンバの検査より先にしたことによる規則の値の変化が設計書 3.4.1 節のとおりであること / 移し替えたテストが交換の段階まで届いていること / CLAUDE.md の例外の段落の削除

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-3 がホストの入力の境界（信頼しないブラウザのメッセージの受理する形）を狭める、独立した危険度の高い変更である

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: サーバのテストの移し替え（設計書 8 章の 2 のうち、製品のコードを変えない部分）

このフェーズは製品のコードを変えない。移し替えたテストは、今の製品のコードで通る。旧 API と失敗の通知だけを確かめるテストは、このフェーズでは残す（設計書 7.1 節「テストの削除の確かめ方」の 1）。

#### ステップ 2-1: `internal/server` のテスト

**変更するファイル:** `internal/server/handler_test.go`、`request_test.go`、`response_test.go`、`server_test.go`、`jobs_api_test.go`、`publish_test.go`、`test_helpers.go`

- [ ] `TestRejectHeaders`・`TestAcceptsWithoutBrowserHeaders`・`TestRejectStatusCodes` を、ジョブの API のリクエストで組み立て直す。`TestRejectStatusCodes` の `webhook_not_configured` の行は、投稿の操作のリクエストで組み立てる（設計書 3.2.3 節）。補助の `validHTTPRequest` も直す。
- [ ] `request_test.go` の全体を、生成の開始で確かめ直す。`TestStartRejectsLikeArticles` は、比べる相手がなくなるので、検査の表のテストに統合する（設計書 7.1 節）。
- [ ] `TestResponseOmitsInputs` と補助の `rejectRequest` を、ジョブの API のリクエストに移す。
- [ ] `TestExpectContinueOversizedBodyNotSent`・`TestUnparsableRequestBypassesHandler`・`TestServeOnUnixListener`・`TestServeListenerFailureStops` と補助の `postOnUnix` を、ジョブの API で確かめ直す。
- [ ] `TestStartGenerationSlotsShared` を `TestStartGenerationSlotsFull` に改め、2 つの生成のジョブで生成の枠を埋める形にする（1.5 節）。
- [ ] `TestPublishPostsLikeArticles` を `TestPublishPostsLikeCLI` に改め、同じ記事を `SlackWebhookPublisher.Publish` で同じループバックの Webhook へ送ったメッセージと、投稿の操作が送るメッセージが一致することを確かめる（設計書 7.1 節）。

**完了の条件:** `go test -tags test ./internal/server/` が通る。

#### ステップ 2-2: `cmd/yt2column-server` のテスト

**変更するファイル:** `cmd/yt2column-server/signal_test.go`、`run_test.go`、`server_test.go`、`test_helpers.go`

- [ ] 子プロセスのモード `childModeBlock` を、`SLACK_WEBHOOK_URL` を設定し、印のファイルができるまで `PostMessages` が返らない投稿の相手を使う形にする。`blockingChild`・`TestSignalWaitsForJob`・`TestSecondSignalExitsAtOnce`・`TestStartDuringShutdownRefused` は、生成を終えさせ、投稿の操作を送り、送信が始まったのを待ってからシグナルを送る（設計書 7.1 節）。
- [ ] `TestServerEndToEnd`・`TestBuildsSlackPublisher`・`TestOutputsOmitSecrets` を、ジョブの API（生成の開始・状態の取得・投稿の操作）で確かめ直す。`TestOutputsOmitSecrets` が `noticeSignal` で待っていた失敗は、ジョブの API の失敗（状態の取得で読める失敗の理由）で待つ。
- [ ] 補助の `postArticle`・`mustPostArticle` を、使う所がなくなれば削除する。`blockingLLM` は使う所がなくなるので削除する。
- [ ] 移し替えた停止のテストが、投稿の相手が返る前にはサーバが終わらないことを確かめていることを、投稿の相手の待ちを外すと失敗することで確かめ、コミットメッセージに書く。

**完了の条件:** `go test -tags test ./cmd/yt2column-server/` が通る。

#### ステップ 2-3: カバレッジの比較とコミット

**変更するファイル:** なし（コミットメッセージだけ）

- [ ] ステップ 2-1・2-2 の変更を 1 つのコミットにする（製品のコードを含まないことを `git diff --stat` で確かめる）。
- [ ] 親とこのコミットで `go tool cover -func` の出力を取り、1.3 節の道具で比べ、関数ごとの結果が変わらないことを確かめる。結果をコミットメッセージに書く。

**完了の条件:** `make test`・`make lint` が通り、製品のコードの関数ごとの網羅率が親コミットと一致する。

### フェーズ 3: サーバの旧 API と失敗の通知の削除（設計書 8 章の 2 のうち、コード）

#### ステップ 3-1: `SlackMaxPostDuration`

**変更するファイル:** `internal/publisher/slack.go`

- [ ] `SlackMaxPostDuration` を、`slackMaxMessages`・`SlackPostTimeout`・`slackMessageInterval` から定義して加える（設計書 3.3 節、1.3 節）。定数だけを確かめるテストは加えない（設計書 7.1 節）。

**完了の条件:** `make test` と `make lint` が通る。

#### ステップ 3-2: `/articles`・旧 API のジョブ・失敗の通知の配線の削除（1.3 節のコミット (a)）

**変更するファイル:** `internal/server/` の `handler.go`・`job.go`（削除）・`jobs_api.go`・`generation.go`・`request.go`・`response.go`・`server.go`・`publish.go` とそのテスト・`test_helpers.go`、`internal/serversock/serversock.go`、`cmd/yt2column-server/` の `run.go` とそのテスト・`test_helpers.go`・`docs_test.go`

- [ ] `handler.go` から `ArticlesPath`・`routeArticles`・`matchTarget` の `/articles` の分岐・`articlesHandler` を削除する（設計書 3.2.1 節）。
- [ ] `maxInFlight`・`acquireSlot`・`releaseSlot` を `jobs_api.go` へ、`classifyWrite` を `generation.go` へ移し、`job.go` を削除する（1.3 節）。
- [ ] `request.go`・`response.go`・`server.go`・`publish.go` から、設計書 3.2.1 節の表の残りの行（`rejectReason.notify`、`rejection.videoID`、`detailTooManyRequests`、`jobInput.requestID` と `requestID` の引数、`WebhookOptions.Notifier`・`Server.notifier`・`errNilWebhookNotifier` とその検査、`WebhookPublisher` の `publisher.Publisher`）を削除する。
- [ ] シャットダウンのログを設計書 3.2.2 節のとおりにする（`genCancel` の前の生成の枠の使用数を `canceledGenerations` に書き、`postJobs` は残し、待ち始めと待ち終えた行から失敗の通知を除く）。
- [ ] 設計書 3.2.1 節・3.2.2 節・3.2.3 節が挙げる注釈（パッケージの説明、`Serve`・`Shutdown` の待ち方・`slots`・`jobs`・`jobCtx`・`Server.publisher`・`webhookConfigured`・`handler`・`maxInFlight`・`maxReading`・`forceClose`・`startHandler`・`WebhookPublisher` の説明）と `internal/serversock/serversock.go:273-274` の注釈から、旧 API と失敗の通知を除く（AC-08）。
- [ ] `cmd/yt2column-server/run.go` を設計書 3.5 節のとおりにする（`deps.newNotifier`・`newNotifier`・`llmCallsPerJob` の削除、`server.WebhookOptions{Publisher: pub}`、起動時の警告、`SlackMaxPostDuration` から分を計算する停止の案内、注釈）。
- [ ] `internal/server/job_test.go` を削除する（11 のテストはすべて削除の対象。設計書 7.1 節）。
- [ ] `handler_test.go` の `TestArticlesWithoutWebhook` を削除する。
- [ ] `jobs_api_test.go` の `TestArticlesIgnoresCache` を削除する。
- [ ] `jobs_api_test.go` の `TestNewAPINeverNotifies` を削除する。
- [ ] `server_test.go` の `TestServeShutdownWaitsForJobs` を削除する。
- [ ] `server_test.go` の `TestBodyReadFailureWritesNoResponse` を削除する。
- [ ] `TestJobsTargetRejections` に、`/articles` を各メソッドで送り、`not_found`・本文を読まないこと・枠を取らないこと・Writer を呼ばないこと・Webhook へ送らないことを確かめる行を加える（AC-01）。
- [ ] `TestServeShutdownCancelsGeneration` を、停止を始めた時点で実行中だった生成の数が `canceledGenerations=1` になることを確かめる形にする（設計書 7.1 節、AC-16）。
- [ ] 失敗の通知の送り手を参照するテストと補助の値を直す: `TestStartHandlerPanicCleanup`・`TestPublishHandlerPanicCleanup`・`TestStatusHasNoSideEffects`・`TestStatusUnknown`（コメント）・`TestNewWebhookOptions`・`TestNewCopiesWebhookOptions`・`newServer`・`newCustomServer`・`newTestServer`・`newJobsTestServer`・`countingPublisher.Publish`・`panickingNotifier`・`serveAccepted`。`TestPublishFailureNotNotified` は 1.5 節のとおり改めるか削除する。
- [ ] `cmd/yt2column-server` のテストの値を直す: `TestStartupFailures` の失敗の通知の送り手の行を削除し、`TestStartsWithoutWebhook` に警告が `/articles` に触れないことの確かめを加え（AC-04）、`TestRunLogsListeningAndStop` を「about 5 minutes」と `/articles`・失敗の通知に触れないことの確かめにし（AC-15、計算の式を使わずに文字列で確かめる）、`TestWriterOptionsMatchCLIDefaults`・`testDeps`・`loopbackDeps`・`nopNotifier`・`noticeSignal`・`nopPublisher.Publish` から失敗の通知と `Publish` を除く。
- [ ] `cmd/yt2column-server/docs_test.go` の curl の例のガードから `server.ArticlesPath` の行を除き、`pathForm` から `server.ArticlesPath` を除く（設計書 3.6 節、AC-20）。
- [ ] 加えたテストの行と書き換えた確かめが、それぞれ対象を壊して失敗することを確かめ、コミットメッセージに書く。

**完了の条件:** `make test` と `make lint` が通る。

#### ステップ 3-3: 使われなくなったものの削除と `depguard`（1.3 節のコミット (b)）

**変更するファイル:** `internal/failure/`（削除）、`internal/publisher/slack_failure.go`・`slack_failure_test.go`（削除）、`slack_errors.go`、`slack.go`、`publisher.go`、`test_helpers_slack.go`、`internal/pipeline/pipeline_test.go`、`docs/dev/conventions/package_reference.md`、`.golangci.yml`

- [ ] `internal/failure` と `internal/failure/testutil` を削除する。
- [ ] `internal/publisher/slack_failure.go` と `slack_failure_test.go` を削除する。
- [ ] `slack_errors.go` から `ErrSlackInvalidNotice`・`ErrSlackNoticeRejected` を削除する。
- [ ] `test_helpers_slack.go` から `NewSlackFailureNotifierForLoopbackTest`・`newSlackFailureNotifierForLoopbackTest` を削除する。
- [ ] `slack.go:85` の `webhookClient` の説明と `publisher.go:3` のパッケージの説明から、失敗の通知を除く。
- [ ] `TestFakesCarryBuildTag` の testutil のファイルの数を 1 つ減らす（1.4 節）。
- [ ] `package_reference.md` の `internal/failure` と `internal/failure/testutil` の行を削除する（`TestPackageReferenceListsPackages`）。
- [ ] `.golangci.yml` の `depguard` に、`internal/server` が `internal/pipeline` を import しないことの規則を加える（設計書 7.4 節）。一時的に `internal/server` に `internal/pipeline` の import を加えて `make lint` が失敗することを確かめ、コミットメッセージに書く。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。

#### ステップ 3-4: 削除のカバレッジの比較と、静的な確認の追加

**変更するファイル:** `docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go`

- [ ] ステップ 2-3 のコミットとステップ 3-3 のコミットで `go tool cover -func` の出力を取り、`_checks/coverage` で比べる。ステップ 1-3 と同じ基準で確かめる。`job.go` から移した 4 つの関数は、名前で対応が取れて網羅率が変わらないことを確かめる。結果をステップ 3-3 のコミットメッセージに書く（`git commit --amend` でメッセージだけを直し、内容が変わらないことを `git diff HEAD@{1} HEAD` で確かめる。push の前に行う）。
- [ ] `_checks/residue` に、サーバの確かめを加える。
    -   AC-06: AC-06 が挙げる識別子が、それを定義していたパッケージ（`internal/server`・`internal/publisher`・`cmd/yt2column-server`）に宣言されておらず、ほかのパッケージから修飾子つきで参照されていない。どの Go のファイルも `internal/failure` と `internal/failure/testutil` を import しない。`internal/server` が `internal/pipeline` を import しない。`classifyWrite` は `internal/server` に宣言されている。
    -   AC-07: `internal/server` の `WebhookPublisher` のメソッドが `PostMessages` だけで、埋め込みを持たない。テストでないコードで `internal/pipeline` を import するのは `cmd/yt2column` だけで、`.Publish(` を呼ぶのは `internal/pipeline` だけである。
    -   AC-02: `internal/server` で `PostMessages` を呼ぶのは `runPublish` だけである。
- [ ] 加えた確かめが merge-base で成り立たないと出ることを確かめる。merge-base で成り立つ確かめ（`classifyWrite` の宣言、`PostMessages` の呼び出し元）は、成り立たなくした一時的なコミット（ブランチに載せない）を head に渡して確かめる。

**完了の条件:** `_checks/residue` が `HEAD` で `0` を返す（記録はステップ 4-3）。

### フェーズ 4: 文書（設計書 8 章の 2 のうち、文書）

#### ステップ 4-1: 開発者の文書と CLAUDE.md

**変更するファイル:** `docs/dev/guides/development.md`、`docs/dev/security.md`、`docs/dev/project_overview.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/production_profile.md`、`docs/translation_glossary.md`、`CLAUDE.md`

- [ ] `development.md`: `/articles` の curl の例（`:77-`）を除き、開発用のサーバの確認の例（`:37`）を `GET /jobs`（`method_not_allowed`）に置き換える（設計書 3.6 節、AC-20）。HTTP API の節の `/articles` と失敗の通知の記述（`:107`・`:111`・`:114-115`・`:119`・`:153`・`:159`・`:168` など）を、ジョブの API だけを前提にした記述に直す。生成の枠はジョブの API の生成のジョブだけが数え、同時に読む本文は最大 2 つ、停止のときに待つのは投稿のジョブで最長およそ 5 分と書く（AC-18）。
- [ ] `security.md` §8 のサーバの記述（`:136`・`:140`・`:155-156` など）から `/articles`・失敗の通知・二重の生成と投稿のリスクを除き、「新しい API」を「ジョブの API」と呼ぶ（AC-18、要件書 4.2）。
- [ ] `project_overview.md` の `:10`・`:109`（`internal/failure` の行）・`:147` と、`production_profile.md:40` から `/articles` と失敗の通知を除く。
- [ ] `package_reference.md` の `cmd/yt2column-server`・`internal/server`・`internal/publisher`・`internal/publisher/testutil` の行を、削除の後の形に直す（設計書 2.2 節）。
- [ ] `translation_glossary.md` の「生成の枠」「枠」「リクエスト ID」の行（`:45`・`:65`・`:96`）から旧 API と失敗の通知を除き、「新しい API」を「ジョブの API」にする。
- [ ] `CLAUDE.md` の Architecture Overview から `internal/failure` の記述（`:279-280`）を取り除く（AC-19 の後半）。
- [ ] 文書のガード（ステップ 1-4 と同じコマンド）と `make ext-test` を通す。

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
- [ ] ステップ 2-3・3-4 のカバレッジの比較の結果を 7.1 節に書き写す。
- [ ] `make test`・`make lint`・`make deadcode`・`make ext-test` を通す。

**完了の条件:** `_checks/residue` が PR の head で `0` を返し、その記録が 7.1 節にある。

### PR-2 作成ポイント: server legacy API and failure notice removal

**対象ステップ**: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3

**推奨タイトル**: `refactor(0059): サーバの旧 API と失敗の通知を削除する`

**レビュー観点**: 移し替えた停止のテストが投稿のジョブを待つことを確かめていること（子プロセスとシグナル） / シャットダウンのログの `canceledGenerations` と停止の案内の「約 5 分」が設計書 3.2.2 節・3.3 節のとおりであること / テストの削除の前後のカバレッジの比較の記録 / 文書がジョブの API だけを前提に書き直され、README と `README.en.md` がそろっていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2 が子プロセスとシグナルを使う停止のテストを作り直し、ステップ 3-2 が停止の待ち方と並行して走るジョブの数え方を変える。多数のテストの更新を伴う削除で、パネルモードの条件に当たる

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: ホスト | 形の判定の削除、テストの移し替え、ホストの文書、確かめのスクリプトとその記録 | PR-1 がマージされた |
| M2: サーバのテストの移し替え | ジョブの API で確かめ直したテスト | フェーズ 2 のコミット（PR-2 の中） |
| M3: サーバの削除 | `/articles`・旧 API のジョブ・失敗の通知・`internal/failure` の削除、`SlackMaxPostDuration`、`depguard` の規則 | フェーズ 3 のコミット（PR-2 の中） |
| M4: 文書 | 開発者の文書、CLAUDE.md、README と `README.en.md`、静的な確認の記録 | PR-2 がマージされた |

M1 は M2〜M4 に依らない。設計書 8 章のとおり、ホストを先にすると、PR-2 の時点で `/articles` に届くのは `curl` だけになる。M2 は M3 の前に置く（1.2 節）。M3 は M2 で移したテストの上で削除する。M4 のうち、削除と同じコミットで直す文書は M3 に入る（1.3 節）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | 確かめのスクリプト、ホストのテストの移し替え、形の判定の削除、CLAUDE.md の例外の段落・`security.md` §8 と `package_reference.md` のホストの記述 | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 | サーバのテストの移し替え、`/articles`・旧 API のジョブ・失敗の通知・`internal/failure` の削除、`SlackMaxPostDuration`、`depguard`、残りの文書と README | frontier-required |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどのテストで確かめるかは本書 7 章に示す。

-   **単体テスト:** 残る振る舞いを旧 API や今の形のメッセージで確かめていたテストは、ジョブの API や新しい形のメッセージで確かめ直す（ステップ 1-2・2-1・2-2）。旧 API と今の形だけを確かめるテストは削除する（ステップ 1-3・3-2）。確かめ直すテストは、述べた段階まで届くことも確かめる（設計書 7.1 節の前置き）。
-   **統合テスト:** `TestRelayEndToEnd` を新しい形に移し、字幕が正しくない場合に Webhook が何も受け取らないこと（AC-02）と、今の形のメッセージが拒否されること（AC-10）のサブテストを持たせる（ステップ 1-2・1-3）。
-   **後方互換:** ジョブの API の受理・拒否・応答（AC-03）、新しい形の中継（AC-09）、CLI（AC-05）は、既存のテストがそのまま通ることで確かめる。版がそろっていない組み合わせ（要件書 4.4）は、本タスクのホストが今の形を拒否すること（ステップ 1-3）とサーバが `/articles` を `not_found` にすること（ステップ 3-2）で確かめる。
-   **セキュリティ:** ホストの入力の境界は、ステップ 1-3 の拒否のテストで、標準ライブラリが黙って直す入力が送られずに拒否されることを確かめる。Webhook へ送る経路が投稿のジョブだけであることは、`_checks/residue`（AC-02・AC-07）と `depguard` の規則で確かめる（設計書 7.3 節）。
-   **テストの削除の確かめ:** テストの移し替えのコミットと削除のコミットのそれぞれで、`go tool cover -func` の関数ごとの結果を比べる（1.3 節、ステップ 1-2・1-3・2-3・3-4）。
-   **壊して確かめる:** 加えたテストの行、作り直した停止のテスト、`depguard` の規則、`_checks/` のスクリプトは、その対象を壊して失敗することを実装のときに確かめ、コミットメッセージに書く。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 移し替えたテストが、述べた段階に届かないまま通る（今の形のメッセージが形の検査で止まる） | ホストの交換の段階や停止の待ち方の確かめが失われる | 交換の段階を確かめるテストは、サーバがリクエストを受け取ったことも確かめる（ステップ 1-2）。停止のテストは、投稿の相手の待ちを外すと失敗することを確かめる（ステップ 2-2） |
| テストの削除で、残る関数の確かめが失われる | 削除と関係のない関数の網羅率が下がる | 移し替えと削除のコミットを分け、それぞれでカバレッジを比べる（1.3 節、ステップ 1-2・1-3・2-3・3-4） |
| 子プロセスの停止のテストが不安定になる（送信の始まりを待つ前にシグナルを送る） | CI が時々失敗する | 印のファイルで送信が始まったのを待ってからシグナルを送る（ステップ 2-2）。`-count` を増やして手元で繰り返し実行する |
| PR-2 が大きく、レビューの見落としが起きる | 削除の漏れや文書の誤り | フェーズを移し替え・削除・文書に分け、削除を 2 つのコミットに分ける（1.3 節）。削除の漏れは `_checks/residue` と `make deadcode` で確かめる |
| `TestFakesCarryBuildTag` の数が、ほかのタスクのマージで変わる | ステップ 3-3 の値が古くなり、`make test` が失敗する | その時点の値から 1 つ減らす（1.4 節） |
| 0055 の PR-6 が先にマージされ、CLAUDE.md の削除の確認の規則が変わる | 1.3 節の確かめ方が規則に合わなくなる | その規則に従う（設計書 7.1 節）。変わった点を 7.1 節に書く |
| 予定の遅れ | N/A | 削除と書き換えが中心で、新しい仕組みは確かめのスクリプト 2 つだけである |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（実行するテストや `make` の対象）、`static`（コミットしたスクリプトや `make lint` による確かめ）、`diff`（`_checks/` のスクリプトで merge-base と PR の head を比べる）、`manual`（PR のレビューで読む）である。`_checks/residue` は `docs/tasks/0059_remove_legacy_articles_api/_checks/residue/main.go` を指す。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | test | `internal/server/jobs_api_test.go::TestJobsTargetRejections`（`/articles` の行） | 3-2 |
| AC-02 | test・static | `cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd/invalid_transcript_posts_nothing`。static: `_checks/residue`（`PostMessages` の呼び出し元が `runPublish` だけ、`WebhookPublisher` が `PostMessages` だけ）、`.golangci.yml` の `depguard` の規則（`make lint`） | 1-2・3-3・3-4・4-3 |
| AC-03 | test | 既存のジョブの API のテストが通ること: `internal/server/jobs_api_test.go`（`TestStartAccepted`・`TestStartReadSlotsFull`・`TestStartGenerationSlotsFull`・`TestJobsTargetRejections` など）、`publish_test.go`・`status_test.go`、`handler_test.go::TestRejectStatusCodes`、`cmd/yt2column-server/docs_test.go::TestServerDocsContract`（拒否の理由と状態コード） | 2-1・3-2 |
| AC-04 | test | `cmd/yt2column-server/run_test.go::TestStartsWithoutWebhook`（警告が `/articles` に触れない）・`TestStartupFailures`、`internal/server/publish_test.go::TestPublishWithoutWebhook`、`internal/server/server_test.go::TestNewWebhookOptions` | 3-2 |
| AC-05 | test | `make test` の `cmd/yt2column` と `internal/pipeline` のテスト（`cmd/yt2column/run_test.go::TestRunSlackFailureHints` など、変えない） | 3-2 |
| AC-06 | static | `_checks/residue`（AC-06 の識別子・パッケージの不在、`classifyWrite` の存在）。`make deadcode` | 3-3・3-4・4-3 |
| AC-07 | static | `_checks/residue`（`WebhookPublisher` のメソッド、`internal/pipeline` の importer、`.Publish(` の呼び出し元） | 3-4・4-3 |
| AC-08 | manual | PR-2 のレビューで、`internal/server` のパッケージの説明・`Serve` の説明・起動時の警告・シャットダウンのログと設計書 3.2.1 節が挙げる注釈を読む | 3-2 |
| AC-09 | test | `internal/nmhost/relay_test.go::TestRelayNewFormOperations`・`TestRelayKeyPassedUnchecked`・`TestRelayNewFormRejections`、`cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd/new_form_operations` | 1-3 |
| AC-10 | test | ステップ 1-3 で `TestRelayLegacyBytesUnchanged` を置き換えるテスト（`internal/nmhost/relay_test.go`、`rule=version` の行）、`TestRelayNewFormRejections`（版の検査の順序の 2 行）、`TestRelayEndToEnd` に加えるサブテスト | 1-3 |
| AC-11 | test | ステップ 1-3 で置き換えるテスト（`rule=object` の 5 種類の行） | 1-3 |
| AC-12 | diff・manual | diff: `_checks/residue`（`extension/` が merge-base から変わっていない）。manual: PR-1 のレビューで、拡張が `protocol_error` を想定外の応答として表示すること（`extension/src/core/native.ts:73-74`）を読む | 1-5・4-3 |
| AC-13 | static・test | `_checks/residue`（`internal/nmhost` の今の形の値と `/articles` の不在）、`internal/nmhost/relay_test.go::TestRequestRefusesUnroutable` | 1-3・1-5 |
| AC-14 | test | ステップ 1-3 で置き換えるテスト（ログに固定の値だけを書く）、`internal/nmhost/relay_test.go::TestRelayNewFormLogOmitsMessage` | 1-3 |
| AC-15 | test | `cmd/yt2column-server/run_test.go::TestRunLogsListeningAndStop` | 3-2 |
| AC-16 | test | `internal/server/server_test.go::TestServeShutdownCancelsGeneration` | 3-2 |
| AC-17 | test | `cmd/yt2column-server/signal_test.go::TestSignalWaitsForJob`・`TestSecondSignalExitsAtOnce`、`run_test.go::TestStartDuringShutdownRefused`、`internal/server/publish_test.go::TestServeShutdownWaitsForPublish`・`server_test.go::TestServeShutdownCancelsGeneration` | 2-2・3-2 |
| AC-18 | manual・test | manual: PR-1・PR-2 のレビューで AC-18 が挙げる文書を読む。test: `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（値の部分）、`make readme-sync-check`（README の 2 つの版の同期） | 1-4・4-1・4-2 |
| AC-19 | manual | PR-1・PR-2 のレビューで CLAUDE.md の 2 か所を読む | 1-4・4-1 |
| AC-20 | test・manual | test: `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`development.md` のサブテスト、3 つの操作の curl の例）。manual: サーバの確認の例が `GET /jobs` を使うこと（設計書 7.4 節） | 3-2・4-1 |
| AC-21 | test | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`rejection_reasons` と `security.md` のサブテスト）、`extension/test/serverReasons.test.ts`（`make ext-test`） | 4-1・4-2 |

要件書 4.5 の「`make deadcode` は何も報告しない」は、各フェーズの完了の条件で確かめる。

### 7.1. 実行の記録

（ステップ 1-5 と 4-3 で記入する。）

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 7 章のすべての行の確かめが済み、7.1 節に `_checks/residue` の実行の記録がある。
-   **品質:** グリーンゲート（`make test && make lint`）、`make deadcode`、`make ext-test` が通る。テストの移し替えと削除の各コミットで、残る関数の網羅率の比較の結果が 7.1 節にある。
-   **セキュリティ:** ホストが新しい形でないメッセージをすべて送らずに拒否し（AC-10・AC-11）、サーバが Webhook へ送るのが投稿のジョブだけである（AC-02）。
-   **文書:** AC-18 が挙げる文書と CLAUDE.md が、削除の後の振る舞いだけを説明している。タスク文書（`docs/tasks/`）の過去の記述は変えない（要件書 2.3）。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`/runpr` が issue #526 を閉じる案内をする（本タスクの最後の PR のため）。
-   ジョブの API の失敗を Webhook へ通知する必要が生じたときは、別のタスクで、ジョブの API の失敗の理由から設計する（設計書 9 章）。
