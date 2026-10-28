# 実装計画書：サーバとホストに、生成と投稿を分けた API と記事のオンメモリのキャッシュを加える

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-27 |
| Review date | 2026-10-27 |
| Reviewer | isseis |
| Comments | 2026-10-27 (editorial): `/runplan` の実装で、ステップ 2-3 の件数の上限の定数の名前を `defaultCapacity` から `DefaultCapacity` に改めた（設計書 §3.3.4 も同じ）。`internal/server` が別のパッケージから `articlecache.New` に渡すために公開が要るためで、値と決定は変えていない。 2026-10-28 (editorial): `/runplan` の PR-4 の実装で、ステップ 5-2 の説明に、`internal/articlecache` の 6 つの列挙のすべての値を返す関数を加えることを書き足した（設計書 §3.3.2 も同じ）。完了の時点の検査が投稿の状態とその理由のすべての値で最も長い応答を組み立て、ステップ 5-4 の `APICodesForTest` が列挙の最後の値の次までたどるために要るもので、決定は変えていない。 2026-10-28 (decision): `/runplan` の PR-4 の後、レビュアー（isseis）がセッションの中で、PR-4 のローカルのレビューで残した指摘のうち、ジョブのログが遷移の後に記録を読み直して古い値を書きうる件を、PR-5 のステップ 5-6 で直すと決め、作業の項目と PR-5 のレビュー観点を加えた（`Run.Fail`・`CompletePostable`・`CompleteUnpostable` の戻り値を変えるので、設計書 §3.3.2 はその実装のコミットで改める）。PR-5 の投稿のジョブも同じ形のログを書くので、同じ PR で揃える。 2026-10-28 (editorial): `/runplan` の PR-5 の実装で、ステップ 5-6 の遷移の時点の値のテストの壊し方を改めた。計画の壊し方（排他制御を解いた後に記録を読み直す）は、読み直すまでの間に別のゴルーチンが記録を変えたときにだけ結果が変わる。並行のテストを 200 回ずつ実行しても、読み直す側が排他制御を先に取り直し、一度も失敗しなかった。本番のコードにテストのための差し込み口を加えずに、決まった順序で割り込む手段はない。このため、返す値を誤って計算する 3 つの壊し方に替えた。決定（ログを戻り値で書くこと）は変えていない。 2026-10-28 (decision): `/runplan` の PR-6 の実装で、要件書の AC-40 の「サーバに接続せず」を、設計書 §3.10.1 の手順に合わせて「サーバにリクエストを送らずに」に改めることを、レビュアー（isseis）がセッションの中で決めた（要件書の `Comments`）。AC-40 の検証の `TestRelayNewFormRejections` は、接続が 1 回でリクエストが 0 件であることを確かめる。 2026-10-28 (editorial): 同じ実装のテストのレビューで、ステップ 6-3 の `TestStatusResponseFitsRelay` からフレームの上限の検査を除いた。`TestRelayResponseOverLimit` の上限ちょうどの行が同じ性質を確かめるためで、決定は変えていない。 2026-10-28 (editorial): PR-6 のレビューで、`TestRelayServerRejectsWithoutBody` を `TestRelayServerEarlyRejection` に改名した。サーバの早い拒否を中継し、本文を送らないことを確かめる形にしたためで、決定は変えていない。 |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、「設計書 §3.2」などは設計書の節を指す。既存のコードと文書についての記述は、コミット `c2ce520` で確かめた。`file:line` はこのコミットの行番号である。設計書が行番号を確かめたコミット `c3d20a7` と `c2ce520` の間では `docs/` の外のファイルが変わっていない（`git diff --stat c3d20a7 c2ce520 -- . ':!docs'` の出力が空）ので、設計書のコードの行番号はそのまま使える。用語は設計書の冒頭の「用語」に従う。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、次を行う。

-   #503 の後に行う旧 API の削除と簡素化の項目（設計書 §3.14）を issue に起票する（H-12）。
-   新しいパッケージ `internal/articlecache` に、記事のキー・記録の状態遷移・件数の上限と破棄・ハンドルを置く（F-001・F-004）。
-   `writer` に工程の進み具合の通知を、`publisher` に表示用の Markdown・ファイルの中身・組み立て済みのメッセージ・料金の書式・途中から再開できる送信を加える（F-002・F-003）。
-   サーバの `SLACK_WEBHOOK_URL` を任意にする（F-006）。
-   サーバに、生成の開始・状態の取得・投稿の操作の 3 つの操作を加える。旧 API（`/articles`）は今の振る舞いのまま残す（F-001〜F-005・F-007）。
-   ホストが、拡張の指定した操作を新しい API の送り先に対応づける（F-008）。
-   README・`docs/dev/security.md`・`docs/dev/project_overview.md`・`docs/dev/production_profile.md`・`docs/translation_glossary.md`・`CLAUDE.md` を更新する（F-009）。

拡張のコードとテストは変えない（要件書 2.3・4.4）。CLI の振る舞いも変えない。

### 1.2. 実装の方針

-   **設計は設計書に従い、本書は繰り返さない。** 型・関数の形、遷移、応答のコードの綴り、ログの属性は設計書が定める。本書のステップは、どの節に従って何を変えるかと、どう確かめるかを示す。
-   **フェーズは設計書 §8 の順序に従う。** フェーズ 1〜7 は設計書 §8 の 1〜7 に対応する。PR は 7 つに分ける（1.3、3.2）。
-   **各ステップの後でグリーンゲート（`make test && make lint`）が通る。** 型を変えるステップは、その型のすべての使用箇所（`cmd/` を含む）を同じステップで直す。Go のファイルを変えたら `make fmt` を先に走らせる。README を変えるステップでは `make ext-test` も走らせる（CLAUDE.md「Development Notes」）。
-   **テストが失敗しうることを、壊して確かめる。** 新しいテストのそれぞれについて、そのテストが確かめる経路を壊し、テストが失敗することを実装の際に確かめ、結果をコミットのメッセージに書く（CLAUDE.md「Every test must be able to fail for its stated reason」）。各ステップに挙げた壊し方は最低限のものである。
-   **既存のテストの入力と期待値は変えない。** 変えてよいのは、1.4 の「更新する既存のテスト」に挙げたものだけである。
-   **Go のソースは英語で書く。** コメント・識別子・文字列リテラルは英語である。テストの入力の ASCII でない文字は、`"あ"` のような Go のエスケープで書く（`internal/server/test_helpers.go:25-26` はテストのファイルを ASCII に保つ）。テストと要件の対応は本書 7 章に書き、ソースのコメントに AC の番号を書かない（`requirements_process.md` §4）。
-   **並行のテストは時間ではなくチャネルで同期する。** fake の止める仕組みは、解放のチャネルと `ctx.Done()` の両方を待つ。時間を使う確認は、今の `TestServeShutdownWaitsForJobs`（`server_test.go:123`）と同じ形に限る。

### 1.3. 計画の際に決めたこと（2026-10-27）

-   **PR は 7 つに分ける（レビュアーが選んだ）:** PR-1 はフェーズ 1・2（issue と土台）、PR-2 はフェーズ 3（`writer` と `publisher`）、PR-3 はフェーズ 4（設定と起動）、PR-4 はフェーズ 5 の前半（生成の開始・状態の取得・生成のジョブ・シャットダウンの取り消し）、PR-5 はフェーズ 5 の後半（投稿の操作と投稿のジョブ）、PR-6 はフェーズ 6（ホスト）、PR-7 はフェーズ 7（文書）とする。サーバの新しい API は最も大きく、リスクも最も高いので、2 つの PR に分けてレビューできるようにするためである。サーバを 1 つの PR にする案と、文書をホストの PR に含める案は採らなかった。
-   **文書は、ガードが確かめる値をコードと同じ PR に、説明の文を最後の PR に置く（レビュアーが選んだ）:** `cmd/yt2column-server/docs_test.go:81-107` の `rejection_reasons` は、README の拒否の理由の表が `RejectReasonsForTest()` と過不足なく一致することを確かめる。このため、新しい拒否の理由の行は、その理由を加えるステップで README に加える（PR-3・PR-4・PR-5）。状態・工程・理由を表すコード（設計書 §3.11）の表とそのガード（AC-29）も、そのコードを返す PR-4 で加える。README の説明の文、`security.md`・`project_overview.md`・`production_profile.md`・`translation_glossary.md` は PR-7 でまとめて更新する。PR-3 のマージから PR-7 のマージまでの間、README は `SLACK_WEBHOOK_URL` を必須と書いたままになる。利用者は 1 人で、Webhook を設定したまま使う限り振る舞いは変わらないので、この間を許す。設計書 §8 もこの置き方に合わせた。
-   **`package_reference.md` は、パッケージを変える PR で更新する:** 同書は「パッケージを加える・取り除く・責務を変えるコミットで更新する」と定める（`docs/dev/developer_guide/package_reference.md:5-6`）。この規則に従い、上の決定の例外として、各 PR がその PR で変えるパッケージの行を更新する。
-   **ホストの形の判定を、CLAUDE.md の例外として書く（レビュアーが選んだ）:** 設計書 §3.14 の 6 は、#503 の後にこの例外を `CLAUDE.md` から取り除くとしたが、加える作業がどこにもなかった。設計書の §2.2・§3.15・§3.17・§8 に `CLAUDE.md` を加え、決定の変更として設計書を `draft` に戻した（設計書の `Comments`）。本計画の PR で設計書とともに承認し直す。`CLAUDE.md` の例外は、判定を実装する PR-6（ステップ 6-2）で書く。規則の例外を、それに当たるコードより後に書くと、PR-6 が単独では `CLAUDE.md` に合わないためである（上の文書の置き方の決定の例外で、設計書 §8 も合わせた）。同じ PR で、設計書 §3.12 と §3.16 の H-02 の行に残っていた「SHA-256」を §3.2 の HMAC-SHA256 に合わせ、`SLACK_WEBHOOK_URL` を必須と書く `docs/dev/production_profile.md:42` を設計書の更新する文書に加えた（編集上の修正）。
-   **issue の起票は、`/runplan` がステップ 1-1 で行う:** 起票はネットワークの操作なので、`/runplan` が実行の前に承認を求める。本計画の作成（`/mkplan`）では起票しない。
-   **フェーズ 5 は、部品を先に作り、受け口を後に作る:** 生成のジョブと状態の応答の組み立て（ステップ 5-2）を、振り分けと受け口（ステップ 5-3）より先に作る。部品はハンドルを直接渡すテストで確かめられるので、各ステップの後でグリーンゲートが通る。
-   **投稿の操作のパスは PR-5 で振り分けに加える:** PR-4 の振り分けは `/articles`・`/jobs`・`/jobs/{key}` だけを受け付け、`/jobs/{key}/publish` は `not_found` にする。PR-4 だけがマージされた状態でも、受け付けないパスは今と同じ `not_found` で、記録は変わらない。
-   **旧 API の `too_many_requests` のログの `detail` は変えない:** 設計書 §3.12 の `read_slot`・`generation_slot`・`cache_full` は新しい API の受け口だけが使う。旧 API の拒否のログは今と同じ `too_many_requests`（`response.go:110`）のままにする（F-007）。
-   **`WebhookPublisher` は PR-3 で `internal/server/publish.go` に置く:** 設計書 §3.17 の置き場所のとおり、ステップ 4-2 で `publish.go` を作って interface だけを置き、投稿のジョブはステップ 5-6 で同じファイルに加える。
-   **サーバのテストの組み立ては、内部の組み立て関数で値を差し替える:** `New` は本番の値（件数の上限 100、`time.Now`、`crypto/rand` の秘密）で内部の組み立て関数を呼び、`internal/server/test_helpers.go` は同じ関数を小さい上限・差し替えた時計・固定の秘密で呼ぶ（設計書 §3.3.4「テストの上限」・§3.2「テスト」）。受け口の panic の後始末（設計書 §7.1）を確かめる差し込み口は、`Begin`・`BeginPublish` の後でジョブに渡す前の 2 か所だけに置き、本番では `nil` の内部のフィールドにする。本文の検査の中の panic は、今の `panickingReader`（`test_helpers.go:132-135`）で起こす。
-   **共有の fake は並行の呼び出しに耐える形にする:** 新しい API の生成のジョブは最大 2 つ、投稿のジョブは数を限らず並んで走る。`writertestutil.FakeArticleWriter` と `publishertestutil.FakePublisher` の記録の追記には排他制御がない（`internal/writer/testutil/mocks.go:31-33`、`internal/publisher/testutil/mocks.go:28-31`）。両方の記録を排他制御の下に置き、ジョブが走っている間に読むための排他制御付きの読み出しを加える。ジョブを待った後に `Calls` を直接読む今のテストは変えない。
-   **AC-41 は、ホストの単体テストと、サーバとホストの上限の関係を確かめるテストで確かめる:** `internal/nmhost` は `internal/server` を import できない（`.golangci.yml:113-119` の `nmhost-boundaries`）。サーバのテストは、受け付けた記事の状態の取得の応答が、どの投稿の状態でもサーバの上限以下であることを確かめる。ホストの単体テストは、ホストの上限ちょうどの応答を切り詰めずに中継することを確かめる。両方を import できる `cmd/yt2column-server` のテストが、サーバの上限がホストの上限以下であることを確かめる。
-   **「拡張のテストを変えずに通る」（AC-42・要件書 4.4）は差分の確認で確かめる:** 前後の関係を述べる性質なので、`docs/tasks/0044_article_job_api/_checks/extension_unchanged.sh` が、本タスクの各 PR のコミットのそれぞれで `extension/` が変わっていないことを確かめる。タスク 0043 などが同じ期間に `extension/` を変えるので、分岐元と先頭の 2 点の比較にはしない。PR-7 で 1 回だけ実行し、PR-1〜PR-6 の squash のコミットと PR-7 のブランチの各コミットを入力にして、出力とコミットの一覧を 7 章に記録する（`requirements_process.md` §4）。シェルと `git` は開発環境にあり、新しい依存を加えない。
-   **README の curl の例は、ガードと、外部のサービスを呼ばない実行で確かめる:** 生成の開始の実行は実際の LLM を、投稿の操作は Webhook を呼ぶので、承認なしには行わない（CLAUDE.md「Tool Execution Safety」）。新しい API の curl の例は、`TestServerDocsContract` が、例のパスがサーバのパスの定数と合うことを確かめる（設計書 §3.17 の「新しい API の curl の例のガード」）。生成と投稿の応答の形は `TestServerEndToEnd`（fake の LLM とループバックの Webhook）で確かめる。実際のサーバでは、LLM と Webhook を呼ばない例（不明なキーの状態の取得、`invalid_key`、Webhook のない開発用のサーバでの投稿の操作）だけを実行する。

### 1.4. 既存コード調査結果

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-13 は、すべて設計書 §3.16 に記録されている。

**`internal/server`（フェーズ 4・5 で変える）**

| 対象 | 今あるもの | 変えること |
|---|---|---|
| 振り分け | `checkHeaders` が `r.RequestURI` を `ArticlesPath` と完全一致で比べる（`handler.go:119-127`） | 設計書 §3.1.1 の照合に置き換える。`TestRejectHeaders` の `path_encoded`（`/%61rticles`）・`path_other` は今の期待値のまま通る |
| 受け口 | `handler`（`handler.go:19-100`）。`recover` は枠を持っていれば失敗の通知を送る（`:46-48`） | 旧 API の受け口は残す。新しい API の受け口は別の関数と別の `recover` にする（設計書 §3.4.5） |
| 本文の読み込みと検査 | `readRequest`・`validateBody`（`request.go:34-110`）。`jobInput` は `source`・`videoURL`・`videoID`・`requestID` を持つ（`request.go:22-27`） | `validateBody` が `articlecache.KeyInput` を作って `jobInput` に入れる（設計書 §3.2「入力の作り方」） |
| 拒否の理由 | `reasonInternal`〜`reasonInvalidTranscript` と `reasonCount`（`response.go:17-31`）、`reasonValue`・`statusCode`・`notify`・`forceClose`（`:35-101`）、`reasonToKind`（`job.go:160-175`）、`rejection`（`response.go:122-128`） | 新しい 5 つの理由（設計書 §3.11.5）。理由ごとに `reasonValue`・`statusCode`・`notify`（通知しない）・`forceClose`・`reasonToKind`（`failure.KindUnknown`）を決める。`rejection` に本文を読み残したことを表す値を加える |
| 生成の枠と旧 API のジョブ | `maxInFlight = 2`（`job.go:19`）、`acquireSlot`（`:27-34`）、`startJob`・`runJob`（`:44-77`）、`classify`・`classifyWrite`（`:109-155`） | `classifyWrite` は `articlecache.FailureReason` を返し、`classify` が `failure.Kind` に対応づける（設計書 §3.11.3） |
| 組み立て | `Options`（`server.go:51-58`）、`validateOptions`（`:118-136`）、`errNilPublisher`・`errNilNotifier`（`:46-47`） | 設計書 §3.8 の `Options`・`WebhookOptions`。`errNilPublisher`・`errNilNotifier` は、そのフィールドがなくなるので、`errNilWebhookPublisher`・`errNilWebhookNotifier` に置き換える（1.5） |
| シャットダウン | `Serve`（`server.go:143-180`）は、受け付けをやめた後にジョブを取り消さずに待ち、`pending` に使用中の枠の数を出す（`:175`） | 設計書 §6.4 |
| 注釈 | パッケージの説明（`server.go:1-6`「runs the article pipeline」）、`slots` の説明（`:69-72`）、`Serve` の説明とログ（`:138-142`・`:175`）、`maxInFlight` の説明（`job.go:16-18`） | 新しい API の生成のジョブが枠を共有し、取り消されることに合わせて改める |
| テストの組み立て | `newTestServer`・`newCustomServer`（`test_helpers.go:51-84`）、`newServer`（`server_test.go:29`）、`New(Options{…})` の呼び出し 4 か所（`test_helpers.go:57`・`:74`、`server_test.go:31`、`job_test.go:297`） | 設計書 §3.13 の「`server.Options` の形」の行。`countingPublisher`（`test_helpers.go:86-104`）・`gateWriter`・`panickingWriter`（`:106-130`）にメソッドを加える |

**更新する既存のテスト（期待値を変えるものに印 \* を付ける）:**

-   `internal/config/config_test.go`: `TestLoadServerWebhookRequired`\*（`:1031-1048`。`unset` のサブテストだけをエラーにならない形に変え、`empty`・`invalid` は残す。名前は 1.5 のとおり変える）、`TestLoadServerValid`（`:991`、新しい戻り値）。
-   `cmd/yt2column-server/run_test.go` の `TestStartupFailures`: `model and webhook missing`\*（`:64-73`。Webhook の代わりにほかの必須の変数を欠かせ、設定の行が 2 行であることを保つ）、`writer not built`（`:127-133`、差し替えの関数の型）、`publisher not built`・`notifier not built`（`:135-150`、Webhook がある場合だけの組み立て）。`TestBuildsSlackPublisher`（`:350`）、`TestWriterOptionsMatchCLIDefaults`（`:384`、差し替えの関数の型）、`TestRunLogsListeningAndStop`（`:411`、`webhookConfigured=true` と WARN の行がないことを加える）。`cmd/yt2column-server/test_helpers.go` の `nopPublisher`（`:253-256`、`PostMessages` を加える）・`testDeps`・`loopbackDeps`（`:265-300`）。
-   `internal/server`: `TestRejectReasonSet`（`response_test.go:87`）・`TestReasonToKind`（`job_test.go:197`）・`TestRejectStatusCodes`（`handler_test.go:82`）に、理由を加えるステップごとに新しい理由の行を加える。`TestRejectStatusCodes` の `build` は `newTestServer` の値を受け取るので、Webhook のないサーバを要る行のために、表に組み立ての選択を加える。`request_test.go` の拒否の表（`TestRejectBodyTable`・`TestOptionalMembers`・`TestRejectVideoURLTable`・`TestRejectInvalidTranscript`・`TestBodyTooLarge` の表）は、AC-08 で生成の開始にも送るために、入力と期待値を変えずにパッケージの変数へ移し、同じ表で `/articles` と `/jobs` の両方を確かめる形にする。
-   `internal/nmhost/relay_test.go`: `TestRelayServerEarlyRejection`\*（`:309-324`）、`TestRelayHostErrors` の `body shorter than the declared length`\*（`:489`）。`TestRelayResponseOverLimit`（`:326`）は `maxResponseBytes` を名前で使うので期待値は変わらない。

**`internal/writer`（フェーズ 3）:** `ArticleWriter`（`writer.go:77-79`）と `templateWriter.Write`（`writer.go:325-407`）がある。工程の境目は、生成（`:352-362`）、短縮の条件 `w.shortenRuns() && bodyChars(body) > maxBodyChars`（`:367`）、検証・推敲（`:385-399`）、最後の長さの検査（`:401-405`）である。`writer.Stage`（`artifact.go:16-31`）を工程の値として使う。`writertestutil.FakeArticleWriter`（`internal/writer/testutil/mocks.go:16-34`）は `Write` だけを持ち、fake のテストは `internal/writer/testutil/mocks_test.go` にある。`ArticleWriter` の他の実装（CLI の `internal/job`、`internal/pipeline` の fake）は変えない。

**`internal/publisher`（フェーズ 3）:** `renderFileArticle`（`file_render.go:35-65`）、`articleCost`・`articleCostDecimals`（`format.go:11-20`）と、その 2 つの使用（`file_render.go:53-54`・`slack_render.go:103-104`）、`prepareSlackMessages`（`slack_message.go:26-58`）、`SlackMessageCount`（`:63-69`。CLI の `cmd/yt2column/run.go:854` と旧 API の `logJobSuccess`（`internal/server/job.go:203`）が使う）、`SlackMessagesForTest`（`test_helpers_slack.go:98`。`cmd/yt2column/output_layout_docs_test.go:112` が使う）、`Publish`（`slack.go:158-184`。最初に `p.client.configured()` を確かめる）、ループバックの Webhook へ送るテスト用の発行者 `NewSlackWebhookPublisherForLoopbackTest`（`test_helpers_slack.go:27`。`Interval` を 0 にできる）がある。`prepareSlackMessages` を直接呼ぶ今のテスト（`slack_message_test.go`）は変えない。`publishertestutil.FakePublisher`（`internal/publisher/testutil/mocks.go`）は `Publish` だけを持つ。

**`internal/config` と `cmd/yt2column-server`（フェーズ 4）:** `ServerConfig.WebhookURL() secret.Secret`（`config.go:259-262`）の呼び出しは `cmd/yt2column-server/run.go:159`・`:290`・`:294` と `config_test.go:991` だけである。`LoadServer`（`config.go:282-301`）は未設定を `missingVar` にする（`:290`）。`loadSlackWebhook`（`:747-764`）は空の値を `ErrMissing`、不正な値を `ErrInvalid` にし、本タスクでも変えない。`deps`（`run.go:49-55`）、`buildServer`（`run.go:277-309`）、`listening` のログ（`run.go:185-189`）、停止の案内（`run.go:231-236`）がある。進行中のタスク 0045（`docs/tasks/0045_llm_env_var/`、設計書が承認済み）は、同じ `config.go` と `cmd/yt2column-server/run.go` の LLM の設定を変える（5. リスク管理）。

**`internal/nmhost`（フェーズ 6）:** `forward`（`relay.go:215-255`）は、接続の後に標準入力を HTTP の本文として流し込み、`Expect: 100-continue` で送る。送り先は固定の `requestURL`（`:45`）、サーバの応答の上限は `maxResponseBytes = 64 << 10`（`:36`）、ログの段階は `length`・`connect`・`exchange`・`reply`（`:154-159`）である。設計書 §3.10.2「説明の更新」が挙げる注釈（`frame.go:1-4`・`:18-21`、`relay.go:23-27`・`:117-118`・`:176-183`・`:208-214`）と、`.golangci.yml:113-119` の `nmhost-boundaries` の説明を改める。

**`internal/strictjson`（フェーズ 2）:** `Object.Has`（`strictjson.go:123-132`）の契約は「診断のためだけに使う」である。`Value` は元のバイト列を `raw` に持つ（`:319-345` の `decodeObject` が `json.RawMessage` を写す）が、それを返すメソッドはない。

**文書とガード:** `TestServerDocsContract`（`cmd/yt2column-server/docs_test.go:30-150`）の `rejection_reasons` は、見出しのセルが `reason` に一致する最初の表を拒否の理由の表とみなし、`README.md` の節の検査は、見出しのセルが `Code` に一致する最初の表を終了コードの表とみなす（`findTable`、`docs_test.go:258-270`、大文字と小文字を区別しない完全一致）。PR-4 で加えるコードの表は、どの見出しのセルも `reason`・`Code` にしない。curl の例の検査は、`curl` と `server.ArticlesPath` を同じ行に持つことを確かめる（`docs_test.go:66-68`）。拡張の `extension/test/serverReasons.test.ts:13-42` は、README の `| reason | status code | extension shows |` の表の 3 列目が `fixed message` で始まる行だけを読む。`SLACK_WEBHOOK_URL` を必須と書く文書の記述は、`README.md:58`・`:86`・`:532`・`:810` と `docs/dev/production_profile.md:42` にある（`README.md:806` の設定の表の行は CLI の説明である）。

**外部の前提の確認（コミット `c2ce520`、`GOTOOLCHAIN=go1.26.5`（`go.mod` の版）と Go 1.27.1 の両方の `go run` で同じ結果）:** `http.NewRequest` に `"/jobs/" + url.PathEscape(key)` を与えて `httptest.Server` に送ると、サーバの `r.RequestURI` は、`abc/publish` → `/jobs/abc%2Fpublish`、`a?x=1` → `/jobs/a%3Fx=1`、`%2F` → `/jobs/%252F`、`\x01` → `/jobs/%01`、`あ` → `/jobs/%E3%81%82`、`..` → `/jobs/..`、空の文字列 → `/jobs/` になった。クライアントはエスケープした形を変えずに送り、パスを正規化しない（設計書 §3.10.3 の前提。ステップ 6-2 のテストで改めて確かめる）。`go doc crypto/rand.Read` は「It never returns an error, and always fills b entirely」と書く（設計書 §3.2 の前提）。

### 1.5. 名前の変更の表

本書のほかの記述は、この表に従う。

| 種類 | 今の名前 | 新しい名前 | ステップ |
|---|---|---|---|
| テスト | `internal/config/config_test.go::TestLoadServerWebhookRequired` | `TestLoadServerWebhookOptional` | 4-1 |
| エラーの値 | `internal/server` の `errNilPublisher`・`errNilNotifier` | `errNilWebhookPublisher`・`errNilWebhookNotifier` | 4-2 |
| メソッドの形 | `ServerConfig.WebhookURL() secret.Secret` | `ServerConfig.WebhookURL() (secret.Secret, bool)` | 4-1 |

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 旧 API の削除と簡素化の issue（設計書 §3.14、H-12）

#### ステップ 1-1: issue を起票する

-   **作業:**
    -   [x] 設計書 §3.14 の「削除するもの」8 項目と「簡素化するもの」6 項目を本文に書いた issue を、日本語で起票する。本文に、#503 の完了を待って着手することを書く。
    -   [x] 起票の前に、本文のファイルでメンションの検査（`.claude/commands/_lib/task-pr-handoff.md`「Mention check」）を行う。
    -   [x] issue を #501 の sub-issue にする（`task-pr-handoff.md` の「Choosing the issue to link」と同じ `sub_issues` の API）。
    -   [x] #503 を、起票した issue のブロッキング issue（blocked by）にする。GitHub の REST API `POST /repos/{owner}/{repo}/issues/{issue_number}/dependencies/blocked_by` で、パスに起票した issue の番号を、本文の `issue_id` に #503 の数値の `id`（番号ではない）を渡す（GitHub REST API の「Issue dependencies」の文書で確かめた）。
    -   [x] 起票した issue の番号を、本書 9 章に記録する。
-   **完了の条件:** issue が #501 の sub-issue として一覧に出て、#503 にブロックされていると表示される。

### フェーズ 2: 土台（`internal/strictjson`・`internal/articlecache`・`depguard`）

#### ステップ 2-1: `strictjson` の `Value.Raw` と `Object.Has` の契約

-   **ファイル:** `internal/strictjson/strictjson.go`・`strictjson_test.go`
-   **作業:**
    -   [x] `Value.Raw() []byte` を加える。元のバイト列の写しを返す（設計書 §3.10.3）。
    -   [x] `Object.Has` の注釈を、エスケープを解いた名前の完全一致を約束し、振る舞いの選択に使えるものに改める（設計書 §3.10.2）。
    -   [x] テスト: `Raw` が値の中の空白・エスケープ・メンバの並びを変えずに返すこと、返した写しを書き換えても `Value` が変わらないこと。`Has` が `"v"` と書いた名前を `v` として見つけること。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 2-2: 記事のキー

-   **ファイル:** `internal/articlecache/key.go`・`key_test.go`（新規）
-   **作業:**
    -   [x] 設計書 §3.2 の `KeyInput`・`Key`・`Keyer`・`NewKeyer`・`ParseKey`・`Key.String`・`ErrInvalidKey` を加える。符号化・HMAC・文字列の形は設計書 §3.2 のとおりである。
    -   [x] テスト（固定の秘密）: 5 つの値のどれか 1 つだけが違えば違うキーになること（AC-04）、2 つの値の境目だけを動かした入力が違うキーになること（AC-05）、同じ入力を違う秘密で計算すると違うキーになること（T-10）、`ParseKey` が `Key.String` の結果を受け付けて同じキーに戻すこと、`ParseKey` が 63 文字・65 文字・大文字・前後の空白・16 進でない文字・空の文字列を `ErrInvalidKey` で拒否し、エラーの文言が受け取った文字列を含まないこと（AC-44）。キーを固定の 16 進の値と比べるテストは書かない（設計書 §7.1）。
    -   [x] 壊し方: 長さの前置きを外すと AC-05 の行が、秘密を計算に使わないと T-10 の行が失敗することを確かめる。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 2-3: 記録・遷移・件数の上限・ハンドル

-   **ファイル:** `internal/articlecache/record.go`・`cache.go`・`cache_test.go`・`test_helpers.go`（新規。`test_helpers.go` は `//go:build test`）
-   **作業:**
    -   [x] 設計書 §3.3.2 の列挙・`Stages`・`Snapshot`・`Options`・`New`・`ErrInvalidOptions`・`Begin`・`BeginOutcome`・`Lookup`・`BeginPublish`・`PublishBegin`・`Eviction`・`Run`・`PublishRun` を加える。遷移は設計書 §3.3.1、時刻は §3.3.3、上限と破棄は §3.3.4、排他制御・記事の写し・世代・不変条件は §3.3.2 の箇条書きに従う。`DefaultCapacity`（別のパッケージのサーバが `New` に渡すので公開する）は 100 である。
    -   [x] `test_helpers.go` に、記録の全体を文字列にする関数を加える。サーバのテストが記録に目印がないことを確かめるために使う（設計書 §7.1 の AC-27）。記録の公開しないフィールドを読むので、`test_helpers.go` に置く。記録の構造体の全体から文字列を作り、後で加えたフィールドも含まれるようにする。
    -   [x] テスト（差し替えた時計と小さい上限）: 設計書 §7.1 の「`internal/articlecache`」の項目をすべて確かめる。とくに、`Begin` の 5 つの結果と `acquire` を呼ぶかどうか（AC-02・AC-06・AC-07・AC-26a）、`BeginExisting` が投稿の状態を変えないこと（AC-09c）、破棄の順序と対象（AC-25・AC-26）、`BeginPublish` の 5 つの結果（AC-20・AC-22・AC-23）、`Delivered` と `From` の関係（AC-21・AC-45a）、所要時間がやり直しの受け付けから数えられること（AC-14）、記録の記事の `Review.Revisions` が `nil` で、渡した記事が書き換えられないこと（AC-27）、新しいキーと `Failed` のキーのそれぞれで並行の `Begin` を `-race` で走らせ、`acquire` が 1 回だけ呼ばれ、`BeginStarted` または `BeginRestarted` が 1 つだけ返ること（AC-09b）、古い世代のハンドルと範囲外の入力が何もせず panic しないこと（T-09）。
    -   [x] 壊し方: 破棄の対象から `PublishPosting` を除く条件を外す、世代の比較を外す、`Begin` が `BeginExisting` でも `acquire` を呼ぶ、の 3 つでそれぞれのテストが失敗することを確かめる。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 2-4: `depguard` の規則と `package_reference.md`

-   **ファイル:** `.golangci.yml`・`docs/dev/developer_guide/package_reference.md`
-   **作業:**
    -   [x] `internal/articlecache` が `net/http`・`internal/llm`・`internal/publisher`・`internal/server` を import することを禁じる規則を加える（設計書 §2.1）。
    -   [x] 一時的に `_ "net/http"` を import して、`make lint` の出力が新しい規則の名前で失敗することを確かめ、元に戻す。結果をコミットのメッセージに書く。
    -   [x] `package_reference.md` に `internal/articlecache` の行を加え、`internal/strictjson` の行を改める（1.3）。
-   **完了の条件:** グリーンゲートが通る。

### PR-1 作成ポイント: legacy removal issue and article cache foundation

**対象ステップ**: 1-1 / 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `feat(0044): 旧 API の削除の issue を起票し、記事のキャッシュとキーの土台を加える`

**レビュー観点**: 記録の状態遷移と破棄の対象（処理中・投稿中を捨てない） / 排他制御の中で止まらないこと、範囲外の入力で panic しないこと、ハンドルの世代 / キーの符号化の曖昧さのなさと秘密の扱い / issue の本文が設計書 §3.14 の全項目を含むこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-3 が、並行の呼び出しと世代を持つ状態機械を単独で実装する、リスクの高いステップである。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/527
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `writer` と `publisher`

#### ステップ 3-1: 工程の進み具合（`writer`）

-   **ファイル:** `internal/writer/writer.go`・`progress_test.go`（新規）、`internal/writer/testutil/mocks.go`・`mocks_test.go`
-   **作業:**
    -   [x] 設計書 §3.5 の `ProgressObserver`・`ProgressWriter`・`NewProgressWriter` を加える。`templateWriter` が `WriteWithProgress` を実装し、`Write` は何もしない通知先でそれを呼ぶ。通知の時点は設計書 §3.5「通知の約束」に従う。
    -   [x] `FakeArticleWriter` に `WriteWithProgress` を加える。指定した工程の通知を順に送り、工程ごとに止められる仕組みを持たせる。止める仕組みは解放のチャネルと `ctx.Done()` の両方を待ち、取り消されたら `ctx` のエラーを返す。記録を排他制御の下に置き、排他制御付きの読み出しを加える（1.3）。これらを `mocks_test.go` で確かめる。
    -   [x] テスト（fake の `LLMClient`）: 通知の順序（AC-10）、下書きの本文が 4,000 字ちょうどで `StageSkipped`、4,001 字で短縮の `StageStarted`・`StageFinished`（AC-11）、各工程の失敗の後にその工程の `StageFinished` が来ないこと、最後の長さの検査が失敗したときに検証・推敲の `StageFinished` が来ないこと、`Write` と `WriteWithProgress` が同じ記事を返すこと。
    -   [x] 壊し方: 検証・推敲の `StageFinished` を最後の長さの検査の前に移すと、該当の行が失敗することを確かめる。
-   **完了の条件:** グリーンゲートが通り、`writer` の今のテストが変更なしで通る。

#### ステップ 3-2: 表示用の Markdown・ファイルの中身・料金の書式

-   **ファイル:** `internal/publisher/format.go`・`display_render.go`（新規）・`file_render.go`・`file.go`・`slack_render.go`、テストは `display_render_test.go`（新規）・`file_render_test.go`
-   **作業:**
    -   [x] `FormatArticleCost` を加え、ファイルの整形と Slack の脚注をそれを呼ぶ形にする（設計書 §3.6.1）。
    -   [x] `renderDisplayMarkdown` と `DisplayMarkdown` を加え、`renderFileArticle` のフロントマターの後の部分をそれで書く形にする（設計書 §3.6.2）。
    -   [x] `FileContent` を加え、`FilePublisher.Publish` が書く中身をそれで作る（設計書 §3.6.1）。
    -   [x] テスト: `golden_test.go` の入力のそれぞれで、`FileContent` の結果が `FilePublisher` の書いたファイルとバイト単位で同じであること（AC-12）。`DisplayMarkdown` の並び（チャンネル名がある場合とない場合）と、`CheckPublishable` に通らない記事での拒否。`TestRenderGolden`・`TestRenderGoldenNeutralized` はゴールデンファイルを変えずに通る。
-   **完了の条件:** グリーンゲートが通り、`internal/publisher/testdata/golden/` に差分がない。

#### ステップ 3-3: 組み立て済みのメッセージと途中からの送信

-   **ファイル:** `internal/publisher/slack_message.go`・`slack.go`・`test_helpers_slack.go`・`slack_test.go`、`internal/publisher/testutil/mocks.go`・`mocks_test.go`、`docs/dev/developer_guide/package_reference.md`
-   **作業:**
    -   [x] `SlackMessages`・`PrepareSlackMessages` を加え、`SlackMessageCount` と `SlackMessagesForTest` をそれを呼ぶ形にする（設計書 §3.6.1）。
    -   [x] `PostMessages` を加え、`Publish` を `PrepareSlackMessages` と `PostMessages(ctx, m, 0, 何もしない関数)` の形にする（設計書 §3.7.1）。`PostMessages` も `Publish` と同じく、最初に Webhook の URL を持つかを確かめる。
    -   [x] `FakePublisher` に `PostMessages` を加え、記録を排他制御の下に置き、排他制御付きの読み出しを加える（1.3）。`mocks_test.go` で確かめる。
    -   [x] テスト（`NewSlackWebhookPublisherForLoopbackTest` を `Interval` 0 で使う）: N 通に分かれる記事で、k = 1 の再開（`from` が 0）と 1 < k の再開（`from` = k − 1）のそれぞれで k 通目から送り、`delivered` が届いた位置ごとに呼ばれること（AC-21）、n 通目の失敗で `*SlackPostError` の `Posted` が最初から数えた数になること、範囲外の `from` を送らずに拒否すること、`from` が全体の数なら何も送らずに `nil` を返すこと。`Publish` の今のテスト（`slack_test.go`）は変更なしで通る。
    -   [x] 壊し方: `PostMessages` が `from` を無視して最初から送ると、再開の行が失敗することを確かめる。
    -   [x] `package_reference.md` の `internal/writer`・`internal/publisher` の行を改める（1.3）。
-   **完了の条件:** グリーンゲートが通る。

### PR-2 作成ポイント: writer progress and publisher rendering and resumable send

**対象ステップ**: 3-1 / 3-2 / 3-3

**推奨タイトル**: `feat(0044): writer に工程の進み具合の通知を、publisher に表示用の整形と途中からの送信を加える`

**レビュー観点**: `Write` と CLI の振る舞いが変わらないこと（今のテストとゴールデンファイル） / 検証・推敲の `StageFinished` を最後の長さの検査の後に呼ぶこと / `PostMessages` の再開の位置と `Posted` の数え方 / 共有の fake の排他制御と、取り消しで止まる仕組み

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-3 の `PostMessages` が、投稿の再開（AC-21・AC-45a）が依存する回復の手順の土台で、誤ると重複や欠落の投稿になる。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/530
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 設定と起動

#### ステップ 4-1: `SLACK_WEBHOOK_URL` を任意にする（`config`）

-   **ファイル:** `internal/config/config.go`・`config_test.go`、`cmd/yt2column-server/run.go`
-   **作業:**
    -   [x] `LoadServer` が未設定をエラーにしない形にし、`ServerConfig.WebhookURL` を `(secret.Secret, bool)` を返す形にする（設計書 §3.8、1.5）。
    -   [x] `cmd/yt2column-server/run.go` の 3 つの呼び出し（`:159`・`:290`・`:294`）を新しい形に合わせる。このステップでは、未設定のときも今と同じく発行者と通知を組み立てようとして失敗する（組み立ての分岐はステップ 4-2 で加える）。
    -   [x] 注釈を改める: `config.go:249` の「always set」、`:259`、`:277` の「(required)」、`:745-746` の「LoadServer, where the Webhook is required」。
    -   [x] テスト: `TestLoadServerWebhookRequired` を `TestLoadServerWebhookOptional` に改め（1.5）、`unset` のサブテストは、エラーがなく `WebhookURL` が未設定を返すことを確かめる（AC-32）。`empty`・`invalid` のサブテストは期待値を変えない（AC-33）。`TestLoadServerValid` は新しい戻り値で設定済みを確かめる。`TestStartupFailures` の `model and webhook missing` を 1.4 のとおりに改める。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 4-2: `server.Options` と旧 API の `webhook_not_configured`

-   **ファイル:** `internal/server/server.go`・`handler.go`・`response.go`・`job.go`・`publish.go`（新規）・`test_helpers.go`、`internal/server` のテスト、`cmd/yt2column-server/run.go`・`run_test.go`・`test_helpers.go`、`README.md`
-   **作業:**
    -   [x] `Options`・`WebhookOptions` を設計書 §3.8 の形にし、`Options.Writer` を `writer.ProgressWriter` にする。`WebhookPublisher`（設計書 §3.7.1）を `publish.go` に置く（1.3）。`validateOptions` は設計書 §3.8 のとおり検査する（1.5）。
    -   [x] 拒否の理由 `webhook_not_configured`（503、`forceClose`、通知しない）を加え、`Webhook` が `nil` のとき、旧 API がパスとメソッドの検査の後、枠を取る前に、本文を読まずにこの理由で拒否する（設計書 §3.9）。
    -   [x] `cmd/yt2column-server` の `deps.newWriter`・`deps.newPublisher` の型と本番の値（`writer.NewProgressWriter`）を変え、`buildServer` が `cfg.WebhookURL()` が設定済みを返したときだけ発行者と通知を作って `WebhookOptions` を渡す形にする（設計書 §3.8）。
    -   [x] 注釈を改める: `server.go:51` の「Every field is required」。
    -   [x] テストの組み立てを、設計書 §3.13 の「`server.Options` の形」と「`cmd/yt2column-server` の `deps` の型」の行と、1.4 の「更新する既存のテスト」のとおりに改める。
    -   [x] テスト: Webhook のないサーバで `/articles` が `webhook_not_configured` と `Connection: close` を返し、本文を読まず、枠を取らず、書き手と通知を呼ばないこと（AC-34 の `/articles` の部分）。`New` が `WebhookOptions` のどちらかのフィールドが `nil`（型付きの `nil` を含む）のときに拒否すること。`TestRejectReasonSet`・`TestReasonToKind`・`TestRejectStatusCodes` に新しい理由の行を加える。
    -   [x] README の拒否の理由の表に `webhook_not_configured` の行を加える。3 列目は `unexpected reply` とする（設計書 §3.15）。
-   **完了の条件:** グリーンゲートと `make ext-test` が通り、旧 API の今のテスト（7 章の AC-36 の行）が期待値を変えずに通る。

#### ステップ 4-3: 起動のログと、Webhook なしの起動のテスト

-   **ファイル:** `cmd/yt2column-server/run.go`・`run_test.go`、`docs/dev/developer_guide/package_reference.md`
-   **作業:**
    -   [x] `listening` の行に `webhookConfigured` を加え、未設定のときに WARN の行を 1 行書く（設計書 §3.8「起動のログ」）。
    -   [x] テスト: Webhook のない環境でサーバが起動し、`webhookConfigured=false` と WARN の行を書き、発行者と通知の組み立て関数を呼ばないことを確かめる `TestStartsWithoutWebhook` を加える（AC-32）。`TestRunLogsListeningAndStop` に、設定されているときの `webhookConfigured=true` と WARN の行がないことを加える。
    -   [x] `package_reference.md` の `cmd/yt2column-server`・`internal/server`・`internal/config` の行を改める（1.3）。
-   **完了の条件:** グリーンゲートが通る。

### PR-3 作成ポイント: optional webhook and server options

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `feat(0044): サーバの SLACK_WEBHOOK_URL を任意にし、未設定なら /articles を拒否する`

**レビュー観点**: 空の値と不正な値が今と同じく起動を止めること / Webhook のないサーバで `/articles` が本文を読まず、ジョブも通知も始めないこと / 旧 API のテストの期待値が変わっていないこと

**実装モデル要件**: standard

**判定理由**: 組み立ての型の変更と 1 つの拒否の追加で、競合する案もリスクの高いステップもなく、条件付きの検査の該当もない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/536
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: サーバの新しい API

#### ステップ 5-1: 組み立てと、キーの入力

-   **ファイル:** `internal/server/server.go`・`request.go`・`job.go`・`test_helpers.go`、テストは `request_test.go`・`job_test.go`
-   **作業:**
    -   [x] `Server` に記事のキャッシュ、`Keyer`、読み込みの枠（`maxReading = 2`）、`genCtx` を持たせ、秘密を `New` で作る（設計書 §3.2・§3.4.2・§3.4.4）。テストの値を渡す内部の組み立て関数を設ける（1.3）。
    -   [x] `validateBody` が `KeyInput` を作って `jobInput` に入れる（設計書 §3.2）。
    -   [x] `classifyWrite` が `articlecache.FailureReason` を返し、`classify` がそれを `failure.Kind` に対応づける形にする（設計書 §3.11.3）。`TestFailureClassification` は期待値を変えずに通る。
    -   [x] 1.4 の「注釈」の行の注釈を改める。
    -   [x] テスト: `validateBody` が、省略した任意のメンバと空文字列のメンバで同じ `KeyInput` を作り、`videoUrl` に正規化した URL を入れること。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 5-2: 生成のジョブ・完了の時点の検査・状態の応答の組み立て

-   **ファイル:** `internal/server/generation.go`・`status.go`（新規）、テストは `generation_test.go`（新規）、`internal/server/test_helpers.go`
-   **作業:**
    -   [x] 設計書 §3.4.4 の生成のジョブ（取り消しの扱いと panic の回復を含む）と、§3.6.3 の完了の時点の検査を加える。
    -   [x] 状態の取得の応答を記録の写しから組み立てる関数を加える（設計書 §3.1.3・§3.11）。応答のコード（状態や理由を表す値）は網羅的な `switch` で選ぶ。完了の時点の検査の応答の大きさは、この関数で最も長い場合の応答を組み立てて確かめる（`maxStatusResponseBytes` = 256 KiB）。最も長い場合は、工程の状態、投稿の状態とその理由のすべての値でたどるので、`internal/articlecache` の 6 つの列挙に、最後の値の次を表す公開しない値と、すべての値を返す関数（本番のコードが使う 4 つは `StageStates`・`PublishStates`・`UnpostableReasons`・`PublishFailureReasons`、ほかの 2 つは `-tags test` の `JobStatesForTest`・`FailureReasonsForTest`、ステップ 5-4 も使う）を加える。上限を `-tags test` の定数で公開する（ステップ 6-3 が使う）。
    -   [x] テスト（`Begin` で得た `Run` を生成のジョブに直接渡す）: 7 章の AC-10・AC-16・AC-17a・AC-23（完了の時点の判定。メンションを含む記事と分割できない本文の記事の両方）・AC-27・AC-30・AC-45 の行のテスト。AC-30 は、fake の書き手が返す番兵のエラー、公開できない記事、応答に収まらない記事を使い、8 つの失敗の理由がそれぞれ別の値で返ることを確かめる。AC-27 は、コピーした字幕と、fake の書き手が返す訂正の一覧の `Evidence` に同じ目印を入れ、完了の後の記録の全体（ステップ 2-3 の関数）に目印がないことを確かめる。目印を文言に含むエラーで失敗した後の記録も確かめる。
    -   [x] テスト（AC-41 の前提）: 応答の上限ちょうどの記事が受け付けられ、1 バイト超える記事が `article_too_large` になること。上限ちょうどの記事の応答が、投稿の状態を `posting`・`failed`（最も長い理由）・`posted` にしても上限以下であること。
    -   [x] 壊し方: 完了の時点の検査の 1 と 3 の順序を入れ替える、`Review.Revisions` を捨てずに記録に渡す、最も長い場合の組み立てから 1 つのメンバを外す、の 3 つでそれぞれのテストが失敗することを確かめる。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 5-3: 振り分けと、生成の開始・状態の取得の受け口

-   **ファイル:** `internal/server/handler.go`・`jobs_api.go`（新規）・`response.go`・`test_helpers.go`、テストは `jobs_api_test.go`・`status_test.go`（新規）・`request_test.go`・`response_test.go`・`job_test.go`・`handler_test.go`、`README.md`
-   **作業:**
    -   [x] 新しい API のパスの定数を `internal/server` に加え、振り分けを設計書 §3.1.1 の照合に置き換える。この PR では `/jobs/{key}/publish` を `not_found` にする（1.3）。本文がないことの検査、`invalid_key`（400）、`rejection` の本文を読み残したことを表す値、`too_many_requests` の新しい `detail` の値を加える（設計書 §3.1.1・§3.11.5・§3.12、1.3）。
    -   [x] 生成の開始の受け口（設計書 §3.4.1 の順序と表、§3.4.5 の持ち主と `recover`、§3.12 のログ）と、状態の取得の受け口を加える。新しい API の受け口から通知を呼ばない。
    -   [x] README の拒否の理由の表に `invalid_key` の行を加える（3 列目は `unexpected reply`）。`TestRejectReasonSet`・`TestReasonToKind`・`TestRejectStatusCodes` に新しい理由の行を加える。
    -   [x] テスト（パスと境界）: 設計書 §7.3 のサーバ単体のパスの照合（`/jobs/<64 文字>/publish/x`・`?` を含むターゲット・大文字や `%` を含むキーの部分・空のキーの部分）、63 文字・65 文字・`%20` で前後を埋めたキーの拒否（AC-44、T-02）、メソッドの拒否と `Allow`、`Content-Length` が 0 でない状態の取得と `Transfer-Encoding` を持つ状態の取得の `invalid_body` と `Connection: close`、どの拒否でも記録が変わらないこと（AC-44）。`TestRejectHeaders` は期待値を変えずに通る。
    -   [x] テスト（生成の開始）: 7 章の AC-01・AC-02・AC-03・AC-06・AC-07・AC-08・AC-09・AC-09a・AC-09b・AC-25・AC-26a・AC-37 の行のテスト。AC-03 は、メンバの順序・空白・エスケープ・宣言していないメンバ・`youtu.be` と `watch?v=` の URL・省略と空文字列の組を送り、同じキーが返ることを確かめる。AC-07 は、生成の枠を旧 API のジョブ（`gateWriter` で止める）と新しい API の生成のジョブ（進み具合の止める仕組みで止める）で埋め、新しいキーが拒否されて状態の取得が `unknown` を返し、処理中のキーと完了したキーがそれぞれキーを返すことを確かめる。AC-08 は、1.4 のとおりパッケージの変数に移した拒否の表で `/articles` と `/jobs` の両方を確かめ、拒否の後に記録がないことを確かめる。AC-09a は、読み込みに入ったことを知らせて止まる本文で読み込みの枠を埋め、3 つ目の要求の本文が読まれない（`recordingReader` の読んだ数が 0）ことを確かめる。AC-09b は、書き手を止めたまま同じ要求をすべて返させ、書き手の呼び出しが 1 回であることを、新しいキーと失敗したキーのそれぞれで確かめる。
    -   [x] テスト（状態の取得）: 7 章の AC-12・AC-13・AC-14・AC-15・AC-17・AC-28・AC-31・AC-34（生成の開始と状態の取得）・AC-35 の行のテスト。AC-28 は、1 つ目のサーバで記事を `done` にしてから、新しいサーバの値に同じキーを問い合わせる。AC-31 は、fake のエラーの文言・秘密の値・コピーした字幕・`videoUrl`・メタデータに目印を入れ、生成の開始と状態の取得のすべての応答（拒否を含む）に、完了した記事の 2 つの文字列以外には目印が現れないことを確かめる。
    -   [x] テスト（後始末、T-09）: `Begin` の後でジョブに渡す前の差し込み口の panic で、生成の枠と読み込みの枠が空になり、記録が `失敗` になり、`internal_error` が返り、通知が送られないこと。本文の検査の中の `panickingReader` の panic（`Begin` の前で、記録はまだない）で、読み込みの枠が空になり、`internal_error` が返り、記録が作られず、通知が送られないこと。生成のジョブが panic した後の要求にサーバが応答することを確かめる（AC-45）。
    -   [x] 壊し方: 新しい API に別の生成の枠を持たせると AC-07 の行が、キーの部分をデコードしてから照合すると T-02 の行が失敗することを確かめる。
-   **完了の条件:** グリーンゲートと `make ext-test` が通る。

#### ステップ 5-4: コードの表とそのガード

-   **ファイル:** `internal/server/test_helpers.go`、`cmd/yt2column-server/docs_test.go`、`README.md`
-   **作業:**
    -   [x] `APICodesForTest` を加える。6 つの集合のそれぞれについて、列挙のすべての値を（列挙の最後の値の次を表す値まで）たどり、状態の取得の応答と同じ符号化の関数でコードにする（`RejectReasonsForTest` と同じ形、`test_helpers.go:216-222`）。
    -   [x] README の「Localhost server」節にコードの 6 つの表を加え、`TestServerDocsContract` にサブテスト `api_codes` を加える（設計書 §3.15「コードの表のガード」、AC-29）。サブテストは「Localhost server」節の中だけを探し、6 つの見出しの表をそれぞれちょうど 1 つ見つける。集合の中のコードが重複しないことも確かめる（`switch` が名指ししない値は `default` のコードになり、ほかの値のコードと重なるため）。表の見出しのセルは `reason`・`Code` にしない（1.4）。
    -   [x] 壊し方: README の表から 1 つの値を消す、列挙に値を加えて README に行を加えない、の 2 つで `api_codes` が失敗することを確かめる。
-   **完了の条件:** グリーンゲートと `make ext-test` が通る。

#### ステップ 5-5: シャットダウンでの生成の取り消しと、端から端までのテスト

-   **ファイル:** `internal/server/server.go`・`server_test.go`、`cmd/yt2column-server/run.go`・`run_test.go`・`server_test.go`、`docs/dev/developer_guide/package_reference.md`
-   **作業:**
    -   [x] `Serve` が、まず `genCtx` を取り消し、取り消したジョブのログを書く形にする（設計書 §6.4・§3.12）。停止の案内に、新しい API の生成が取り消されることを加え、その文を確かめる行を `run_test.go` の停止の案内のテストに加える。
    -   [x] テスト（AC-46 の生成と旧 API の部分）: 生成のジョブの書き手が取り消しを受け取って終わり、記録が失敗になり、取り消しの行だけを書き、生成の失敗の行を書かないこと。同じときに旧 API のジョブの書き手の `ctx` は取り消されず、ジョブが完了してから `Serve` が戻ること。`TestServeShutdownWaitsForJobs` は期待値を変えずに通る。
    -   [x] 壊し方: `genCtx` を取り消さないと、このテストが失敗することを確かめる。
    -   [x] `TestServerEndToEnd` に、fake の LLM で生成の開始 → 状態の取得が `done` を返すまでの流れを加える（設計書 §7.2）。
    -   [x] `package_reference.md` の `internal/server` の行を改める（1.3）。
-   **完了の条件:** グリーンゲートが通る。

### PR-4 作成ポイント: generation start, status, and generation job

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5

**推奨タイトル**: `feat(0044): サーバに生成の開始と状態の取得の API を加える`

**レビュー観点**: デコードする前のターゲットの照合とキーの形の検査（パスの注入、T-02） / 受け口とジョブの panic の後始末で枠と記録が残らないこと（T-09） / 生成の枠の合算と読み込みの枠、`too_many_requests` の 3 つの場合 / シャットダウンで新しい API の生成だけを取り消し、旧 API のジョブを待つこと / 応答とログに字幕・秘密・文言が出ないこと、コードの表の見出しが既存のガードの表と衝突しないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 5-3 のパスの照合が信頼できない入力の境界（セキュリティの関門）で、ステップ 5-2・5-3 が並行のジョブと後始末を扱う。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/538
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 5-6: 投稿の操作と投稿のジョブ

-   **ファイル:** `internal/server/handler.go`・`jobs_api.go`・`publish.go`・`response.go`・`server.go`・`generation.go`・`test_helpers.go`、`internal/articlecache/record.go`・`cache_test.go`、テストは `publish_test.go`（新規）・`generation_test.go`・`response_test.go`・`job_test.go`・`handler_test.go`・`jobs_api_test.go`、`cmd/yt2column-server/server_test.go`、`README.md`、`docs/dev/developer_guide/package_reference.md`
-   **作業:**
    -   [x] 投稿の操作のパスの定数を加え、振り分けに `/jobs/{key}/publish` を加える。検査の順序と結果ごとの応答は設計書 §3.7.2 に従う。拒否の理由 `article_not_ready`・`article_not_found`・`article_not_postable` を加え、README の拒否の理由の表に 3 行を加え（3 列目は `unexpected reply`）、`TestRejectReasonSet`・`TestReasonToKind`・`TestRejectStatusCodes` に行を加える。
    -   [x] 投稿のジョブを加える（設計書 §3.7.2「投稿のジョブ」「panic」「通知しない」）。投稿のジョブは `s.jobs` に数え、シャットダウンで取り消さずに待つ。シャットダウンのログに投稿のジョブの数を別の属性で出す（設計書 §6.4）。
    -   [x] テスト: 7 章の AC-09c・AC-18・AC-19・AC-20・AC-21・AC-22・AC-23（投稿の操作の拒否）・AC-24・AC-31（投稿の操作）・AC-34（投稿の操作）・AC-44（投稿の操作）・AC-45a・AC-46（投稿のジョブの部分）の行のテスト。AC-18 は、同じ記事を旧 API のジョブが投稿したときに Webhook が受け取るメッセージと比べる（ループバックの Webhook、`Interval` 0）。AC-19 は、応答を止めておける fake の Webhook で、`202` が Webhook の応答より先に返ることを確かめる。AC-21 は、n 通目で失敗させる Webhook で k = 1 と 1 < k を確かめる。AC-44 は、ステップ 5-3 のキーと本文とクエリの拒否を投稿の操作のパスにも当て、Webhook のないサーバでは `webhook_not_configured` がキーの検査より先に返ることも確かめる。AC-45a は、`PostMessages` が k − 1 通目まで `delivered` を呼んでから panic する fake（1 < k）で、やり直しが `from` = k − 1 で呼ばれることと、最後の `delivered` の後の panic のやり直しが何も送らずに `posted` になることを確かめる。
    -   [x] テスト（T-09）: `BeginPublish` の後でジョブに渡す前の差し込み口の panic で、記録が `投稿の失敗` になり、`internal_error` が返り、通知が送られないこと。
    -   [x] 壊し方: やり直しを `From` ではなく 0 から送ると AC-21 の行が、`BeginPublish` が `PublishPosting` の記録にも新しいハンドルを返すと AC-20 の行が失敗することを確かめる。
    -   [x] `TestServerEndToEnd` に、ループバックの Webhook で投稿の操作 → 状態の取得が `posted` を返すまでの流れを加える。
    -   [x] ハンドルが遷移の時点の記録の値を返す形にし、ジョブのログをその値で書く（PR-4 のレビューで残した指摘、2026-10-28 にレビュアーが決めた）。今の生成のジョブは、`Run.Fail`・`CompletePostable`・`CompleteUnpostable` の後に `Cache.Lookup` で記録を読み直してログの `step`・`duration` を書くので、その間に同じキーのやり直しや破棄があると、古い値（`step=""`、`duration=0s`）を書きうる。`Run.Fail` は失敗した工程を、`CompletePostable`・`CompleteUnpostable` は所要時間を、排他制御の中で取って返す（ハンドルが無効なら、何もしなかったことを返す）。投稿のジョブの `PublishRun.Delivered`・`Finish`・`Fail` の後のログも、記録を読み直さずに書く。設計書 §3.3.2 の型の節を同じコミットで改める。テスト: `articlecache` で、返す値が遷移の時点の値であり、遷移の後のやり直しや破棄で変わらないこと、無効なハンドルが何もしなかったことを返すこと。壊し方: `Fail` が工程に `StageFailed` を付ける前の値を返す、`CompletePostable` が完了の時刻を記録する前の値から所要時間を計算する、`PublishRun` の `Finish`・`Fail` が記録の値ではなくハンドルの `From` を返す、の 3 つで、このテストが失敗することを確かめる（排他制御を解いた後に記録を読み直す誤りは、間に別のゴルーチンが記録を変えたときにだけ値が変わり、テストから決まった順序で割り込む手段がないので、壊し方にしない）。
    -   [x] `package_reference.md` の `internal/server` の行を改める（1.3）。
-   **完了の条件:** グリーンゲートと `make ext-test` が通る。

### PR-5 作成ポイント: publish operation and post job

**対象ステップ**: 5-6

**推奨タイトル**: `feat(0044): サーバに投稿の操作の API を加える`

**レビュー観点**: 投稿の冪等性と同時の投稿の操作 / 失敗と panic の後に届かなかった最初のメッセージから再開すること / 投稿の操作のパスの照合と検査の順序（Webhook・本文・キー） / 投稿の操作が Webhook を待たずに応答すること、シャットダウンで投稿のジョブを待つこと / ハンドルが遷移の時点の値を返し、ジョブのログが記録を読み直さないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 5-6 が、並行の操作の冪等性と送信の再開を扱う、リスクの高いステップである。パスの照合の変更は、PR-4 でレビューした照合に 1 つの形を加えるだけである。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/542
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: ホスト

#### ステップ 6-1: メッセージの全体の読み込み

-   **ファイル:** `internal/nmhost/relay.go`・`relay_test.go`
-   **作業:**
    -   [x] 接続の後、送る前にメッセージの全体を、中継の時間の上限の中で読む形にする（設計書 §3.10.1 の 3）。読み込みの失敗はログの段階 `read` にする。
    -   [x] 今の形のメッセージは、読んだバイト列を変えずに `POST /articles` の本文として、今と同じヘッダで送る（設計書 §3.10.1「今の形の送り方」）。
    -   [x] テスト: 1.4 の `TestRelayServerEarlyRejection`・`TestRelayHostErrors` の該当の場合を、設計書 §3.10.1「更新する既存のテスト」のとおりに改める。標準入力が止まると `server_timeout` を返し、リクエストを送らないこと。`TestRelayForwardsOneRequest` は期待値を変えずに通る。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 6-2: 形の判定と新しい形

-   **ファイル:** `internal/nmhost/relay.go`・`relay_test.go`、`CLAUDE.md`
-   **作業:**
    -   [x] 設計書 §3.10.2 の判定、§3.10.3 の新しい形の検査・`Operation`・送り先・本文を加える。キーは `url.PathEscape` でエスケープし、送り先の形は `Operation` の網羅的な `switch` で選ぶ。ログの属性 `form`・`op`・`rule` と段階 `form` を加える（設計書 §3.10.5）。
    -   [x] テスト: 7 章の AC-38・AC-39・AC-40・AC-40a・AC-41a の行のテスト。AC-40 は、拒否のどの場合でもテストのサーバにリクエストが届かないことを確かめる。AC-39 は、`v` らしい文字列を含む不正な UTF-8 と後続のデータを持つメッセージを含める。
    -   [x] テスト（T-02）: キーが `abc/publish`・`?x=1` を含む・`..`・`%2F`・制御文字・ASCII でない文字のとき、テストのサーバが受け取る `RequestURI` が `/jobs/` とエスケープした 1 つのパスの部分だけからなること（1.4 の確認と同じ形）。
    -   [x] `CLAUDE.md` の「Declare, don't infer」に、この判定の例外を、今の例外（コピーした字幕の解析）と同じく方針・範囲・理由・境界の形で加える（設計書 §3.15、1.3）。
    -   [x] 壊し方: キーをエスケープせずに埋め込むと T-02 の行が、`v` を持つメッセージを今の形に戻すと AC-40 の行が失敗することを確かめる。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 6-3: 応答の上限・注釈・端から端までのテスト

-   **ファイル:** `internal/nmhost/relay.go`・`frame.go`・`relay_test.go`・`test_helpers.go`、`.golangci.yml`、`cmd/yt2column-server/relay_e2e_test.go`、`docs/dev/developer_guide/package_reference.md`
-   **作業:**
    -   [x] `maxResponseBytes` を 512 KiB にする（設計書 §3.10.4）。`TestRelayResponseOverLimit` が新しい上限ちょうどの応答を切り詰めずに中継することを確かめる（AC-41）。
    -   [x] 設計書 §3.10.2「説明の更新」の注釈と `.golangci.yml` の `nmhost-boundaries` の説明を改める。
    -   [x] ホストの応答の上限を `-tags test` の定数で公開し、`cmd/yt2column-server` に、サーバの状態の取得の応答の上限（ステップ 5-2）がホストの上限以下であることを確かめる `TestStatusResponseFitsRelay` を加える（1.3、AC-41）。ホストの上限ちょうどの応答の base64 と固定のメンバがフレームの上限に収まることは、`TestRelayResponseOverLimit` の上限ちょうどの行が確かめる（フレームの上限を超える応答は `encodeReply` が `protocol_error` に替える）。サーバの上限をホストの上限より大きくすると失敗することを確かめる。
    -   [x] `TestRelayEndToEnd` に、新しい形の 3 つの操作と、T-02 の注入のキー（`abc/publish` など）を実際のホストとサーバで通し、注入のキーに `invalid_key` が返る場合を加える（設計書 §7.2・§7.3）。
    -   [x] `package_reference.md` の `internal/nmhost` の行を改める（1.3）。
-   **完了の条件:** グリーンゲートが通る。

### PR-6 作成ポイント: host operation routing

**対象ステップ**: 6-1 / 6-2 / 6-3

**推奨タイトル**: `feat(0044): ホストが操作の値を新しい API の送り先に対応づける`

**レビュー観点**: 今の形のバイト列と送り先が変わらないこと（今の拡張との互換） / キーのエスケープと、送り先を `Operation` だけで選ぶこと（T-02） / `v` を持つメッセージを今の形に戻さないこと（T-03）、読み込みの時間の上限 / `CLAUDE.md` の例外の範囲と境界が実装と合うこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 6-2 が、CLAUDE.md の例外となる形の判定とパスの注入を防ぐ関門（セキュリティの関門）を実装する。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/545
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: 文書

#### ステップ 7-1: README

-   **ファイル:** `README.md`、`cmd/yt2column-server/docs_test.go`
-   **作業:**
    -   [x] 「Localhost server」節に、設計書 §3.15 の「README の「Localhost server」節」の項目と、新しい API の 3 つの操作の curl の例を加える。「Stopping the server」に、新しい API の処理中の生成が取り消されることを加える。
    -   [x] `SLACK_WEBHOOK_URL` を必須と書く `README.md:58`・`:86`・`:532`・`:810`（1.4）を、サーバでは任意で投稿の操作と `/articles` に要ることに改め、未設定の場合の振る舞いを書く（設計書 §3.15「README の設定の表と本文」）。
    -   [x] `TestServerDocsContract` の `README.md` のサブテストに、新しい API の curl の例が、サーバのパスの定数（ステップ 5-3・5-6）から作った生成の開始・状態の取得・投稿の操作のパスを持つことを確かめる検査を加える（1.3、今の `ArticlesPath` の検査と同じ形）。
    -   [x] 開発用のサーバ（README「Running a separate server for development」）を `SLACK_WEBHOOK_URL` なしで起動し、LLM と Webhook を呼ばない例（不明なキーの状態の取得、`invalid_key`、投稿の操作の `webhook_not_configured`）を実行して、記載の応答と合うことを確かめる。結果をコミットのメッセージに書く。生成の開始と投稿の例は実行しない（1.3）。
-   **完了の条件:** グリーンゲートと `make ext-test` が通る。

#### ステップ 7-2: そのほかの文書

-   **ファイル:** `docs/dev/security.md`・`docs/dev/project_overview.md`・`docs/dev/production_profile.md`・`docs/translation_glossary.md`
-   **作業:**
    -   [x] 設計書 §3.15 の `security.md` §8・`project_overview.md`・`production_profile.md`・`translation_glossary.md` の項目のとおりに更新する（`CLAUDE.md` はステップ 6-2 で更新した）。
-   **完了の条件:** グリーンゲートが通る。

#### ステップ 7-3: 拡張が変わっていないことの差分の確認

-   **ファイル:** `docs/tasks/0044_article_job_api/_checks/extension_unchanged.sh`（新規）
-   **作業:**
    -   [x] 引数のコミットのそれぞれについて、その親との間で `extension/` に差分がないことを確かめ、差分があれば終了コード 1 でそのコミットとファイルを出すスクリプトを書く。
    -   [x] PR-1〜PR-6 の squash のコミットと、PR-7 のブランチの `origin/main` から先のコミットを入力にして実行し、出力とコミットの一覧を 7 章の AC-42 の行に記録する（1.3）。
    -   [x] 壊し方: `extension/` を変えた一時的なコミットを入力に加えると終了コード 1 になることを確かめ、そのコミットを捨てる。
-   **完了の条件:** スクリプトが終了コード 0 で終わる。

### PR-7 作成ポイント: documentation

**対象ステップ**: 7-1 / 7-2 / 7-3

**推奨タイトル**: `docs(0044): 新しい API と記事のキャッシュを README・security.md などに書く`

**レビュー観点**: 文書の記述が F-001〜F-008 の振る舞いと合っていること（AC-43） / `security.md` §8 の残るリスクの書き直しとホストの上限の記述 / curl の例のガードと、外部のサービスを呼ばない実行の結果

**実装モデル要件**: standard

**判定理由**: 文書とガードの追加と、LLM と Webhook を呼ばない手動の確認だけで、競合する案もリスクの高いステップもなく、条件付きの検査の該当もない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

| マイルストーン | 成果物 | 依存 |
|---|---|---|
| M1（PR-1） | 旧 API の削除の issue、`strictjson` の追加、`internal/articlecache` | なし |
| M2（PR-2） | `ProgressWriter`、表示用の Markdown・ファイルの中身・料金の書式、`PostMessages` | なし（M1 と独立） |
| M3（PR-3） | Webhook の任意化、`server.Options` の新しい形 | M2（`ProgressWriter`・`PostMessages`） |
| M4（PR-4） | 生成の開始・状態の取得・生成のジョブ・シャットダウンの取り消し | M1・M2・M3 |
| M5（PR-5） | 投稿の操作と投稿のジョブ | M4 |
| M6（PR-6） | ホストの形の判定と新しい形 | M1（`Value.Raw`）・M5（送り先のパス） |
| M7（PR-7） | 文書、拡張が変わっていないことの確認 | M6 |

PR-1 と PR-2 は互いに依存しないが、PR は番号の順にマージする。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 2-2 / 2-3 / 2-4 | issue の起票、`strictjson`、`internal/articlecache`、`depguard` | frontier-recommended |
| PR-2 | 3-1 / 3-2 / 3-3 | `writer` の進み具合、`publisher` の整形と途中からの送信、共有の fake | frontier-recommended |
| PR-3 | 4-1 / 4-2 / 4-3 | Webhook の任意化、`server.Options`、`webhook_not_configured` | standard |
| PR-4 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 | 組み立て、生成のジョブ、振り分け、生成の開始、状態の取得、コードの表、シャットダウンの取り消し | frontier-required |
| PR-5 | 5-6 | 投稿の操作と投稿のジョブ | frontier-recommended |
| PR-6 | 6-1 / 6-2 / 6-3 | ホストの全体の読み込み、形の判定、新しい形、応答の上限 | frontier-required |
| PR-7 | 7-1 / 7-2 / 7-3 | README・`security.md`・`project_overview.md`・`production_profile.md`・用語集、curl の例のガード、差分の確認 | standard |

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 §7.1 のパッケージごとの項目を、各ステップのテストの作業で確かめる。AC との対応は 7 章にある。すべてのユニットテストは LLM API・Webhook・実際のブラウザを呼ばない。fake の `LLMClient`・`FakeArticleWriter`・ループバックの Webhook（`httptest`）を使う（要件書 5.、H-13）。並行の振る舞いを確かめるテスト（`articlecache` の並行の `Begin`、サーバの同時の生成の開始と投稿の操作）は、`make test` の `-race` の実行で確かめる。

### 4.2. 統合テスト

`cmd/yt2column-server` の `TestServerEndToEnd`（ステップ 5-5・5-6）と `TestRelayEndToEnd`（ステップ 6-3）に、新しい API を実際のソケットで通す場合を加える（設計書 §7.2）。実際の LLM と Webhook を呼ぶ統合テストは加えない。

### 4.3. 後方互換のテスト

旧 API と今の形のメッセージについては、今のテストを入力と期待値を変えずに通すことで確かめる（1.2）。変えてよいのは 1.4 の「更新する既存のテスト」だけである。CLI の振る舞いは、`writer`・`publisher` の今のテストとゴールデンファイルで確かめる（ステップ 3-1〜3-3）。

### 4.4. 差分の確認

ステップ 7-3 の `_checks/extension_unchanged.sh` だけである（AC-42）。`make test` と CI は実行しない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| キャッシュの排他制御の中で止まる、または panic で排他制御が解かれない | `acquire` は待たない関数だけを渡し、すべてのメソッドで排他制御を `defer` で解く（設計書 §3.3.2）。範囲外の入力のテスト（ステップ 2-3）と `-race` の実行 |
| fake の書き手やループバックの Webhook を使う並行のテストが不安定になる | チャネルで同期し、止める仕組みは `ctx.Done()` も待つ（1.2）。ジョブが走っている間は排他制御付きの読み出しで記録を読む |
| タスク 0045 と `internal/config/config.go`・`cmd/yt2column-server/run.go` で競合する | 先にマージされた側に合わせて、後の PR のブランチで `origin/main` を取り込む。PR-3 の作業の前に 0045 の進み具合を確かめる |
| PR-3 から PR-7 までの間、README が `SLACK_WEBHOOK_URL` を必須と書いたままになる | 1.3 の決定として受け入れる。拒否の理由の表の行は各 PR で加える |
| ホストの読み込みの変更で、今の拡張から見た振る舞いが変わる | 今の形のテスト（`TestRelayForwardsOneRequest` など）を期待値を変えずに通し、変える既存のテストは設計書 §3.10.1 の 2 つに限る |
| ゴールデンファイルが変わる | ステップ 3-2 の完了の条件で差分がないことを確かめる。変わった場合は整形の共有の誤りとして直し、ゴールデンファイルを再生成しない |
| PR-4 が大きく、レビューに時間がかかる | 1.3 の決定で投稿の操作を PR-5 に分けた。PR-4 のステップごとにコミットを分ける |
| 同じ期間にほかのタスクが `extension/` を変える | 差分の確認は本タスクのコミットだけを入力にする（1.3） |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 2-2 / 2-3 / 2-4）
- [x] PR-2 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3）
- [x] PR-3 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）
- [x] PR-4 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5）
- [x] PR-5 マージ済み（対象ステップ: 5-6）
- [x] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3）
- [ ] PR-7 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3）

**横断の確認（`make lint` と `make test` が見つけないもの）:**

- [x] 1.5 の表の今の名前（`TestLoadServerWebhookRequired`・`errNilPublisher`・`errNilNotifier`）が、コードと文書に残っていない（PR-3）。
- [x] Webhook を必須とする注釈（`always set`・`required`）が `internal/config` と `cmd/yt2column-server` に残っていない（PR-3）。
- [x] 1.4 の「注釈」の行の `internal/server` の注釈が、新しい API の生成のジョブと取り消しに合っている（PR-4）。
- [x] 設計書 §3.10.2「説明の更新」の注釈に、メッセージを解析しない・標準入力の残りを読まない・64 KiB という記述が残っていない（PR-6）。
- [x] `package_reference.md` の各行が、その PR で変えたパッケージの責務と合っている（各 PR）。

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（実行するテスト）、`static`（ガードのテスト・`make` の対象）、`diff`（`_checks/` のスクリプト）、`manual`（PR のレビューで読む）である。新しいテストの名前は予定の名前で、実装のときに変えたら本表を同じコミットで直す。

| AC | 種別 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | test | `internal/server/jobs_api_test.go::TestStartAccepted` | 5-3 |
| AC-02 | test | `internal/server/jobs_api_test.go::TestStartSameInputGeneratesOnce`、`internal/articlecache/cache_test.go::TestBeginOutcomes` | 5-3・2-3 |
| AC-03 | test | `internal/server/jobs_api_test.go::TestStartKeyEquivalence` | 5-3 |
| AC-04 | test | `internal/articlecache/key_test.go::TestKeyDistinguishesEachValue` | 2-2 |
| AC-05 | test | `internal/articlecache/key_test.go::TestKeyValueBoundary` | 2-2 |
| AC-06 | test | `internal/server/jobs_api_test.go::TestStartRetriesFailed`（枠に空きがある場合とない場合）、`internal/articlecache/cache_test.go::TestBeginOutcomes` | 5-3・2-3 |
| AC-07 | test | `internal/server/jobs_api_test.go::TestStartGenerationSlotsShared` | 5-3 |
| AC-08 | test | `internal/server/request_test.go::TestStartRejectsLikeArticles`（パッケージの変数に移した `TestRejectBodyTable` などの拒否の表を `/articles` と `/jobs` の両方に当てる、1.4） | 5-3 |
| AC-09 | test | `internal/server/jobs_api_test.go::TestNewAPINeverNotifies` | 5-3 |
| AC-09a | test | `internal/server/jobs_api_test.go::TestStartReadSlotsFull` | 5-3 |
| AC-09b | test | `internal/server/jobs_api_test.go::TestStartConcurrentSameKey`（新しいキーと失敗したキー）、`internal/articlecache/cache_test.go::TestBeginConcurrentAcquiresOnce` | 5-3・2-3 |
| AC-09c | test | `internal/server/publish_test.go::TestStartKeepsPosted`、`internal/articlecache/cache_test.go::TestBeginOutcomes` | 5-6・2-3 |
| AC-10 | test | `internal/writer/progress_test.go::TestWriteWithProgressOrder`、`internal/server/generation_test.go::TestGenerationStages` | 3-1・5-2 |
| AC-11 | test | `internal/writer/progress_test.go::TestWriteWithProgressShortenBoundary` | 3-1 |
| AC-12 | test | `internal/publisher/file_render_test.go::TestFileContentMatchesFilePublisher`、`internal/server/status_test.go::TestStatusDoneArticle` | 3-2・5-3 |
| AC-13 | test | `internal/server/status_test.go::TestStatusDoneArticle`、`::TestStatusCostUnknown` | 5-3 |
| AC-14 | test | `internal/server/status_test.go::TestStatusDuration`、`internal/articlecache/cache_test.go::TestDurationFromAcceptance` | 5-3・2-3 |
| AC-15 | test | `internal/server/status_test.go::TestStatusUnknown` | 5-3 |
| AC-16 | test | `internal/server/generation_test.go::TestGenerationFailedStep` | 5-2 |
| AC-17 | test | `internal/server/status_test.go::TestStatusHasNoSideEffects` | 5-3 |
| AC-17a | test | `internal/server/generation_test.go::TestGenerationUnpublishable` | 5-2 |
| AC-18 | test | `internal/server/publish_test.go::TestPublishPostsLikeArticles` | 5-6 |
| AC-19 | test | `internal/server/publish_test.go::TestPublishRespondsBeforeWebhook` | 5-6 |
| AC-20 | test | `internal/server/publish_test.go::TestPublishIdempotent`、`internal/articlecache/cache_test.go::TestBeginPublishOutcomes`・`::TestBeginPublishConcurrentStartsOnce` | 5-6・2-3 |
| AC-21 | test | `internal/server/publish_test.go::TestPublishResumesAfterFailure`、`internal/publisher/slack_test.go::TestPostMessagesResume` | 5-6・3-3 |
| AC-22 | test | `internal/server/publish_test.go::TestPublishRejections` | 5-6 |
| AC-23 | test | `internal/server/generation_test.go::TestGenerationUnpostable`、`internal/server/publish_test.go::TestPublishRejections` | 5-2・5-6 |
| AC-24 | test | `internal/server/publish_test.go::TestPublishFailureNotNotified` | 5-6 |
| AC-25 | test | `internal/articlecache/cache_test.go::TestEvictionOrder`、`internal/server/jobs_api_test.go::TestStartEvictsOldest` | 2-3・5-3 |
| AC-26 | test | `internal/articlecache/cache_test.go::TestEvictionKeepsRunningAndPosting` | 2-3 |
| AC-26a | test | `internal/articlecache/cache_test.go::TestBeginFullChangesNothing`、`internal/server/jobs_api_test.go::TestStartCacheFull` | 2-3・5-3 |
| AC-27 | test | `internal/articlecache/cache_test.go::TestCompleteDropsRevisions`、`internal/server/generation_test.go::TestGenerationKeepsNoTranscript` | 2-3・5-2 |
| AC-28 | test | `internal/server/status_test.go::TestStatusUnknownAfterRestart` | 5-3 |
| AC-29 | static | `cmd/yt2column-server/docs_test.go::TestServerDocsContract/api_codes` | 5-4 |
| AC-30 | test | `internal/server/generation_test.go::TestGenerationFailureReasons` | 5-2 |
| AC-31 | test | `internal/server/status_test.go::TestResponsesOmitInputs`（生成の開始・状態の取得）、`internal/server/publish_test.go::TestPublishResponsesOmitInputs`（投稿の操作） | 5-3・5-6 |
| AC-32 | test | `internal/config/config_test.go::TestLoadServerWebhookOptional`、`cmd/yt2column-server/run_test.go::TestStartsWithoutWebhook` | 4-1・4-3 |
| AC-33 | test | `internal/config/config_test.go::TestLoadServerWebhookOptional`（`empty`・`invalid`） | 4-1 |
| AC-34 | test | `internal/server/handler_test.go::TestArticlesWithoutWebhook`、`internal/server/status_test.go::TestStatusWebhookConfigured`、`internal/server/publish_test.go::TestPublishWithoutWebhook` | 4-2・5-3・5-6 |
| AC-35 | test | `internal/server/status_test.go::TestStatusWebhookConfigured` | 5-3 |
| AC-36 | test | `internal/server` と `cmd/yt2column-server` の今のテストのうち、1.4 の「更新する既存のテスト」を除くすべてが、入力と期待値を変えずに通る。旧 API の検査・拒否・同時実行の上限・非同期の生成と投稿・失敗の通知を確かめる主なテストは、`request_test.go` の `TestAcceptValidRequest`・`TestUnknownMemberIgnored`・`TestHeaderRejectionReadsNoBody`・`TestChannelNameAgreesWithWriter`、`job_test.go` の `TestAcceptStartsJob`・`TestJobTranscriptValues`・`TestSlotLimit`・`TestSlotReleasedOnce`・`TestFailureClassification`・`TestNotifyNoticeFields`・`TestNotifyErrorContinues`・`TestPanicRecycledAsInternal`・`TestNotifyPanicLogged`、`handler_test.go` の `TestRejectHeaders`・`TestAcceptsWithoutBrowserHeaders`、`response_test.go` の `TestResponseOmitsInputs`・`TestRejectReasonZeroValueIs500`、`server_test.go` の `TestServeShutdownWaitsForJobs`・`TestBodyReadFailureWritesNoResponse`・`TestExpectContinueOversizedBodyNotSent`・`TestUnparsableRequestBypassesHandler`、`cmd/yt2column-server/signal_test.go::TestSignalWaitsForJob` である。AC-08 の表のテストは入力と期待値を変えずに `/articles` にも当てる | 4-2・5-1〜5-6 |
| AC-37 | test | `internal/server/jobs_api_test.go::TestArticlesIgnoresCache` | 5-3 |
| AC-38 | test | `internal/nmhost/relay_test.go::TestRelayNewFormOperations` | 6-2 |
| AC-39 | test | `internal/nmhost/relay_test.go::TestRelayLegacyBytesUnchanged` | 6-2 |
| AC-40 | test | `internal/nmhost/relay_test.go::TestRelayNewFormRejections` | 6-2 |
| AC-40a | test | `internal/nmhost/relay_test.go::TestRelayKeyPassedUnchecked` | 6-2 |
| AC-41 | test | `internal/server/generation_test.go::TestStatusResponseWithinLimit`、`internal/nmhost/relay_test.go::TestRelayResponseOverLimit`、`cmd/yt2column-server/relay_e2e_test.go::TestStatusResponseFitsRelay` | 5-2・6-3 |
| AC-41a | test | `internal/nmhost/relay_test.go::TestRelayNewFormTimeout` | 6-2 |
| AC-42 | static・diff | `cmd/yt2column-server/docs_test.go::TestServerDocsContract/rejection_reasons`、`make ext-test`（`extension/test/serverReasons.test.ts`）、`docs/tasks/0044_article_job_api/_checks/extension_unchanged.sh`（2026-10-28 に PR-7 のブランチで、レビューと CodeRabbit の修正の後に実行した。入力は PR-1〜PR-6 の 6 つの squash のコミット `29843fef`・`7161b660`・`9cfde0e0`・`f9890c10`・`6c11b46e`・`31cc50a2` と、PR-5 の後にマージされた本タスクの補足の PR #543 のコミット `de651e46`、および PR-7 のブランチのコミット `d45475ca`・`ad96abda`・`60d02201`・`06beefcc`・`709ad375`・`46884979`・`4d561992`・`d017f1a0` の 15 個である。出力は各行が `<コミット>: extension/ unchanged` の 15 行で、終了コードは 0 である。分岐元（`git merge-base origin/main HEAD`）は `31cc50a2`、実行した時点の HEAD は `d017f1a0` である。この記録を書くコミットは本計画書だけを変えるので、入力に含めていない） | 4-2・5-3・5-6・7-3 |
| AC-43 | manual | PR-7 のレビューで、README・`security.md`・`project_overview.md`・`production_profile.md`・`translation_glossary.md` を F-001〜F-008 と照らして読む。PR-6 のレビューで `CLAUDE.md` の例外を、各 PR のレビューで `package_reference.md` の行を読む | 6-2・7-1・7-2 |
| AC-44 | test | `internal/articlecache/key_test.go::TestParseKey`、`internal/server/jobs_api_test.go::TestJobsTargetRejections`（状態の取得）、`internal/server/publish_test.go::TestPublishTargetRejections`（投稿の操作）。本文を使う新しい操作はない（本文を持つのは生成の開始だけで、その検査は AC-08 の表で確かめる）ので、要件書 AC-44 の本文を使う操作の部分は当てはまらない | 2-2・5-3・5-6 |
| AC-45 | test | `internal/server/generation_test.go::TestGenerationPanic`（記録が内部の失敗になること）、`internal/server/jobs_api_test.go::TestGenerationPanicServerResponds`（その後の要求に応答すること） | 5-2・5-3 |
| AC-45a | test | `internal/server/publish_test.go::TestPublishPanicResumes`、`internal/articlecache/cache_test.go::TestPublishResumeFrom` | 5-6・2-3 |
| AC-46 | test | `internal/server/server_test.go::TestServeShutdownCancelsGeneration`、`internal/server/publish_test.go::TestServeShutdownWaitsForPublish` | 5-5・5-6 |

## 8. 成功基準 (Success Criteria)

-   **機能:** 7 章のすべての行の検証が通り、AC-43 のレビューが済んでいる。
-   **品質:** 各ステップでグリーンゲートが通る。README を変えるステップで `make ext-test` が通る。新しいテストのそれぞれについて、壊して失敗することを確かめ、コミットのメッセージに書いている。
-   **セキュリティ:** 設計書 §7.3 の項目（パスの注入、形の取り違え、資源の上限、漏洩、後始末、キーの予測）が、7 章の AC-44・AC-40・AC-09a・AC-07・AC-26a・AC-27・AC-31 の行、ステップ 5-3・6-2・6-3 の T-02 のテスト、ステップ 5-3・5-6 の T-09 のテスト、ステップ 2-2 の T-10 のテストで確かめられている。
-   **文書:** README・`security.md`・`project_overview.md`・`production_profile.md`・用語集・`package_reference.md`・`CLAUDE.md` が更新され、本タスクのコミットが `extension/` を変えていない（ステップ 7-3）。

## 9. 次のステップ

-   **起票した issue:** [#526](https://github.com/isseis/yt2column/issues/526)（#501 の sub-issue、#503 にブロックされている。2026-10-27 に起票）
-   `/runplan` は、本書と設計書の両方が `approved` になってから始める（設計書は本書の PR で承認し直す、1.3）。
-   PR-7 のマージの後、#502 を閉じる（`/runpr` が最後の計画の PR の後に提案する）。
-   #503 で拡張が新しい API に移った後、ステップ 1-1 の issue の作業を行う。
