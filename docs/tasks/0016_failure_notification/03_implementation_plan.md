# 実装計画書：失敗を Slack Webhook へ投稿する仕組み

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-14 |
| Review date | 2026-10-14 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、H-NN は [design_handoff.md](design_handoff.md) の項目、I-NN は [implementation_handoff.md](implementation_handoff.md) の項目、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。「#109 §N」は [0011_localhost_server/02_architecture.md](../0011_localhost_server/02_architecture.md) の節を指す。既存のファイルの行番号は、断りがない限り HEAD `bf3bd58`（ブランチ `isesi/failure-post-on-slack-03`）で確かめた。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements が定める 5 つの機能要件（F-001〜F-005）を、architecture の設計どおりに実装する。`internal/publisher` の中で、記事の投稿と通知の実装が共有する送信の経路（非公開の型 `webhookClient`）を切り出し（architecture §3.1）、`failure.Notice` を固定の文面で Webhook へ 1 回だけ投稿する `SlackFailureNotifier` を作る（architecture §3.2〜§3.5）。あわせて、`internal/failure` の型の集合を固定する guard と、`internal/publisher` の import を制限する `depguard` の規則を加える（architecture §3.6・§3.7）。最後に、#109 の issue に本タスクの契約（実装名・文面・リクエスト ID の形）を記録する（architecture §8 の手順 4）。

本計画のフェーズ 1〜3 は、architecture §8 の実装優先順位の手順 1〜3 と同じ名前・同じ順序である。用語（失敗の通知、通知の実装、送信の経路、文面、種類の文、目印）は architecture §1 の用語と requirements §6 に従う。

### 1.2. 実装原則

- architecture §1.1 の設計原則 5 項目に従う。特に重要なのは、送信の経路を 1 つにすること（原則 1）、値の形を通知の実装自身が検査すること（原則 2）、種類の文を網羅的な `switch` で決めること（原則 3）、載せてはならないものを型で載せられなくすること（原則 4）、通知の実装が記録しないこと（原則 5）である。
- 新しいパッケージは作らない。通知の実装は `internal/publisher` に置く（architecture §2.1）。
- `cmd/yt2column` と、requirements §2.3 が #109 に委ねた文書（`CLAUDE.md`・`project_overview.md`・`security.md`・`package_reference.md`・`README.md`）は変更しない（AC-18）。
- Go のソースのコメント・識別子・文字列リテラルは英語で書く（`_context.md`）。`AC-NN`・`F-NNN`・`H-NN`・`I-NN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- 各フェーズの完了時に、`_context.md` の "Build checks"（`make fmt` → `make test` → `make lint`）を通す。グリーンゲートは `make test && make lint`。

### 1.3. 既存コード調査結果

**`internal/publisher/slack.go`（フェーズ 1 で変える）。** 記事の投稿は 1 つの型に集まっている。`SlackWebhookPublisher` は `webhookURL secret.Secret`・`timeout`・`interval time.Duration`・`httpClient *http.Client` の 4 フィールドを持つ（`:78-83`）。`NewSlackWebhookPublisher`（`:91-100`）が URL を検査し、`newSlackWebhookPublisher`（`:106-118`）が `http.Client` を組む（`CheckRedirect` が `http.ErrUseLastResponse` を返す）。`Publish`（`:125-153`）は URL を取り出し（`:126-129`）、記事を準備し、`SensitiveParts` を導出し（`:134`）、`postMessage`（`:158-194`）を呼ぶ。`postMessage` は `context.WithTimeoutCause`（`:159`）で上限をかけ、`webhookPayload{Text, Silent:true}` を JSON にし（`:162`）、`Content-Type: application/json` を付け（`:205`）、`io.LimitReader` で 8,193 バイトまで読み（`:211-213`）、`200` かつ本文がちょうど `ok` のときだけ成功とする（`:177-185`）。失敗は `postFailure`（`:220-228`）が番兵 `ErrSlackTransport`・時間切れ・中断に写す。番兵は `errZeroWebhookURL`・`errInvalidWebhookURL`・`errSlackBuildRequest`・`errSlackPostTimeout`（`:43-48`）と、`ErrSlackHTTPStatus`・`ErrSlackInvalidResponse`・`ErrSlackTransport`・`ErrSlackUnsplittable`・`ErrSlackMention`（`slack_errors.go:10-16`）。`SlackPostTimeout = 30s`（`:22-23`）。`postMessage`・`postFailure` への参照は `slack.go` の中だけである（HEAD `bf3bd58` で `rg` により確認）。

**`internal/publisher/test_helpers_slack.go`（フェーズ 1・2 で変える）。** `//go:build test`。`NewSlackWebhookPublisherForLoopbackTest`（`:27-30`）と、transport の差し込み口を持つ非公開の `newSlackWebhookPublisherForLoopbackTest`（`:35-51`）がある。後者は `loopbacktest.ValidateURL`・`Timeout` の正値・空でない URL を `t.Fatal` で検査し（メッセージは `slack test publisher: ...` で URL を含めない）、`newSlackWebhookPublisher` を呼ぶ。

**`internal/publisher/slack_errors.go`（フェーズ 2 で変える）。** 冒頭のコメントは "Sentinels for SlackWebhookPublisher"（`:8`）。番兵 `ErrSlackHTTPStatus`・`ErrSlackInvalidResponse`・`ErrSlackTransport`・`ErrSlackUnsplittable`・`ErrSlackMention`（`:10-16`）、`*SlackHTTPStatusError`（`:21-41`。`Unwrap` は `ErrSlackHTTPStatus`）、`*SlackPostError`（`:43-52`・`:54-65`。`Attempted` が「HTTP クライアントを呼んだか」を表す）。

**`internal/publisher/publisher.go`（フェーズ 2 で変える）。** パッケージコメントは「defines the publishing stage」（`:1-2`）。`Publisher` interface（`:14-16`）だけを持つ。

**`internal/publisher/slack_message.go`（変更しない）。** `rejectMention`（`:142-149`）はメンションの記法（`rejectMention` が規則 M1〜M4 と呼ぶ形。`@` に続く名前、`<!`・`<@`、文字参照、`<a|b>` の形）を検出し、`ErrSlackMention` を包むエラーを返す。architecture §1.3 のとおり、通知の経路では実行時に呼ばず、テストで全 `Kind` の文面に当てて nil であることを確かめる（AC-06）。

**`internal/transcript/video_id.go`（変更しない）。** `NormalizedVideoURL(videoID)`（`:64-69`）は `^[A-Za-z0-9_-]{11}$` に一致するときだけ `https://www.youtube.com/watch?v=<id>` を返す。`VideoID` の形の検査と正規化 URL の導出にこれを使う。

**`internal/slackwebhook/slackwebhook.go`（変更しない）。** `ValidURL`（`:20-26`）は `https` かつホストが非空のときだけ真。`SensitiveParts`（`:34-67`）は URL・パス・クエリ・userinfo・末尾 8 文字などの部分を返す。既存の送信の経路と同じ使い方をする。

**`internal/publisher/slack_test.go`（フェーズ 2 で補助関数だけ変える）。** `slack_test.go` は `//go:build test`。再利用する補助関数は次のとおり。`slackWebhookMarker`（`:27`）・`slackWebhookPath`（`:30`）・`fatalRecorder`（`:36-49`）・`newSlackServer`（`:101-123`、リクエストを記録する `httptest.Server`）・`newSlackOKServer`（`:126-131`）・`slackPayloadText`（`:157-168`）・`requireSlackPayloadShape`（`:183-206`、`POST`・`application/json`・`text` と `silent: true` の 2 メンバだけであることを確かめる）・`requireErrorOmitsEndpoint`（`:210-223`、`errors.Unwrap` でたどれるエラーが endpoint や `SensitiveParts` を含まないことを確かめる）。`TestNewSlackWebhookPublisherForLoopbackTestRejects`（`:414-438`）は失敗メッセージが空でないこと・目印を含まないこと・nil を返すことを確かめており、メッセージの文言が変わっても内容を変えずに通る。`TestSlackPublishPayload`（`:442`）・`TestSlackPublishResponses`・`TestSlackPublishRedirect`・`TestSlackPublishTimeout`・`TestSlackPublishBodyFailures`・`TestSlackPublishCanceled`・`TestSlackPublishErrorsOmitWebhookURL`・`TestSlackPublishZeroValue`（`:1093`）・`TestSlackPublishErrorClasses`（`:1106`）ほかが、フェーズ 1 の後に内容を変えずに通ることで、記事の投稿の振る舞いを変えていないことを確かめる。`SlackWebhookPublisher` のフィールド（`.httpClient`・`.timeout`）や `postMessage` を直接参照するテストはない（HEAD `bf3bd58` で `rg` により確認）。

**`internal/pipeline/pipeline_test.go`（変更しない。件数の確認だけ）。** `TestCommonTypesFieldSets`（`:378`）は `transcript`・`llm`・`writer` の型の exported フィールドを固定する（`failure` の型は対象外。本タスクは対象の型を変えないので通る）。`TestFakesCarryBuildTag`（`:534`）は `testutil/` のファイル数を数える（`:558` の 17）。本タスクは `testutil/` にファイルを加えない。`TestPackageReferenceListsPackages`（`:610`）は新しいパッケージに行を要求する。本タスクは新しいパッケージを作らない。

**`.golangci.yml`（フェーズ 3 で変える）。** `depguard` の規則は、`deps`（`list-mode: strict`・`files: $all`・`allow: $gostd` と自モジュール）と、`gemini-sdk-confinement`・`claude-sdk-confinement`・`legacy`（いずれも `list-mode: lax` と `deny`）である。`internal/publisher` を対象にした規則はない。`depguard` の `files` は [gobwas/glob](https://github.com/gobwas/glob) で照合し、`!` を先頭に付けた glob は「一致しない」を表し、`!$test` はテストファイル以外を表す（depguard の README で確認した）。`make lint` は `--build-tags test,integration` で走る（`Makefile:9-10`）。

**`cmd/yt2column`（変更しない）。** `cmd/yt2column/docs_test.go` の `TestREADMEDocumentsFlags`（`:37-52`）が `newFlagSet` のすべてのフラグを README の表に求める。そのため、フラグを足すとこのテストが落ちる。本タスクは `cmd/yt2column` を変更しない（AC-18）。

**まだ実装されていない部品。** `internal/failure`（`Kind`・`Notice`・`Notifier`）は、#109 の手順 1 の PR が新設する。HEAD `bf3bd58` の時点で `internal/failure` のディレクトリはない。本計画のフェーズ 2・3 は、その PR がマージされた後に着手する（§2.0）。

### 1.4. 名前の変更の一覧

本タスクが移す・付ける名前である。以降の本文・AC の表・申し送りの対応は、この表の名前を使う。

| 旧 | 新 | 備考 |
|---|---|---|
| `SlackWebhookPublisher.postMessage` | `webhookClient.post` | 受け手を移し、URL の取り出しと `SensitiveParts` の導出を中に入れる。振る舞いは変えない（architecture §3.1） |
| `SlackWebhookPublisher.postFailure` | `webhookClient.postFailure` | 受け手だけを移す（architecture §3.1） |
| `SlackWebhookPublisher` のフィールド `webhookURL`・`timeout`・`httpClient` | `webhookClient` のフィールド `webhookURL`・`timeout`・`httpClient` | `SlackWebhookPublisher` は `interval` と `client webhookClient` を持つ |
| `newSlackWebhookPublisher`（`slack.go:106-118`） | `newSlackWebhookPublisher(interval time.Duration, client webhookClient)`（名前は同じ。引数を `interval` と `webhookClient` に改める） | `NewSlackWebhookPublisher` とテスト用の構築関数の両方が使う。本番の呼び出しだけになって引数が常に同じ値（`SlackPostTimeout`・`slackMessageInterval`・`nil`）を受け取る形（`unparam` の対象）にしない |
| （新しい切り出し） | `webhookClient`・`newWebhookClient`・`configured`・`checkWebhookURL`・`newLoopbackWebhookClient` | architecture §3.1・§3.2 |
| 要件の仮称 `ErrWebhookResponseRejected` | `ErrSlackNoticeRejected` | 実装の名前。`internal/publisher/slack_errors.go` に置く（architecture §3.8 の H-01）。要件本文の仮称は変更しない |
| （新設） | `ErrSlackInvalidNotice`・`errZeroNotifier`・`SlackFailureNotifier`・`NewSlackFailureNotifier`・`NewSlackFailureNotifierForLoopbackTest`・`newSlackFailureNotifierForLoopbackTest` | architecture §3.2・§4.1 |

`errZeroWebhookURL`（`slack.go:44`。メッセージ "webhook: the publisher has no webhook URL"）は名前もメッセージも変えない。通知の実装はこれとは別に `errZeroNotifier` を持つ（architecture §4.1）。

### 1.5. 申し送りへの対応

`design_handoff.md` の H-01〜H-06 は architecture §3.8 が対応を記録しており、本計画は architecture のとおりに実装する。`implementation_handoff.md` の I-01 は、本計画のフェーズ 2 が扱う。

| 項目 | 本計画での扱い |
|---|---|
| H-01 送信の経路の共有 | フェーズ 1 で `webhookClient` を切り出して共有する。`ErrSlackNoticeRejected` をフェーズ 2 で加える（§1.4） |
| H-02 種類の文と文面 | フェーズ 2 のステップ 2-2・2-3 |
| H-03 置き場所と依存の向き | フェーズ 2 のステップ 2-1（`internal/publisher` に置く）とフェーズ 3 のステップ 3-2（`depguard`） |
| H-04 guard | フェーズ 3 のステップ 3-1 |
| H-05 上限と `ctx` | フェーズ 2 のステップ 2-4（`SlackPostTimeout` を使う） |
| H-06 テスト用の構築 | フェーズ 1 のステップ 1-5・フェーズ 2 のステップ 2-6 |
| I-01 `Notify` の処理手順 | フェーズ 2 のステップ 2-4。検査の順序・エラーの包み方・`ctx` の確認の置き場所を architecture §3.4 の契約の下で決める。テスト用の構築関数の拒否（AC-12）の検査方法もここで決める |

### 1.6. 対象外（requirements §2.3）

`internal/failure` の型の新設（#109 の手順 1 の PR）、サーバ本体・HTTP API・認証・枠（同時実行数の制限）・失敗の種類の判定（#109）、CLI からの投稿（本タスクは経路を再利用可能にするだけ）、文書の更新（#109 の F-010）、記事の投稿の振る舞いの変更、投稿の自動リトライ・集約・抑制は、本タスクの対象外である。

## 2. 実装ステップ (Implementation Steps)

### 2.0. 前提

- **フェーズ 1 は、`internal/failure` の有無によらず着手できる**（architecture §3.1・§8。`internal/failure` に依存しない）。`internal/failure` がまだ無い状態でフェーズ 1 を先に進めてよい。
- **フェーズ 2・3 は、#109 の手順 1 の PR（`internal/failure` の新設）がマージされていることを前提とする。** マージされるまでフェーズ 2・3 に着手しない。ステップ 2-1 の冒頭で、`internal/failure` に `Kind`・`Notice`・`Notifier` があり、`Notice` の exported フィールドが `RequestID`・`VideoID`・`Kind` の 3 つで、`Notifier` のメソッドが `Notify(ctx context.Context, n Notice) error` の 1 つであることを確かめる（AC-01）。この前提が満たされない場合は、#109 に問い合わせてから進める。
- フェーズ 3 のステップ 3-1 は、#109 の手順 1 の PR が同等の guard を既に置いている場合は、それを用いて重複するテストを加えない（architecture §3.6）。置いていない場合だけ加える。

### フェーズ 1: 送信の経路の切り出し（振る舞いを変えないリファクタリング）

**対象ファイル:** `internal/publisher/slack.go`（変更）、`internal/publisher/test_helpers_slack.go`（変更）

- [ ] **ステップ 1-1**: `slack.go` に非公開の型 `webhookClient` と `newWebhookClient(webhookURL secret.Secret, timeout time.Duration, transport http.RoundTripper) webhookClient` を加える。`newSlackWebhookPublisher` の `http.Client` の構築（`CheckRedirect` が `http.ErrUseLastResponse` を返す。`:106-118`）を `newWebhookClient` に移す。
- [ ] **ステップ 1-2**: `webhookClient` に、`configured() error`（URL が無ければ `errZeroWebhookURL`）と `post(ctx context.Context, message string) error` を加える。`post` は現在の `postMessage`（`:158-194`）の受け手を移したもので、先頭で自分自身の `webhookURL` の取り出しと `slackwebhook.SensitiveParts` の導出を行い、`p.webhookURL`・`p.timeout`・`p.httpClient` を使っていた箇所を自分のフィールドに置き換える。`context.WithTimeoutCause`・`webhookPayload`・`Content-Type`・8,193 バイトの上限・`ok` の判定・番兵・`postFailure`（移した受け手のまま `webhookClient.postFailure` にする）は変えない。`post` 自身は `ctx.Err()` を確かめない（`*SlackPostError.Attempted` の意味を変えないため。architecture §3.1）。`post` は URL のない `webhookClient` では送らずに `errZeroWebhookURL` を返す。
- [ ] **ステップ 1-3**: 非公開の `checkWebhookURL(webhookURL secret.Secret) error` を加え、`NewSlackWebhookPublisher`（`:91-100`）の検査（ゼロ値 → `errZeroWebhookURL`、`slackwebhook.ValidURL` の拒否 → `errInvalidWebhookURL`）をそのまま切り出す。`NewSlackWebhookPublisher` は `checkWebhookURL` を通した後、`newSlackWebhookPublisher(slackMessageInterval, newWebhookClient(webhookURL, SlackPostTimeout, nil))` を呼ぶ。
- [ ] **ステップ 1-4**: `SlackWebhookPublisher` を `interval time.Duration` と `client webhookClient` の 2 フィールドに改め、`newSlackWebhookPublisher` は `newSlackWebhookPublisher(interval time.Duration, client webhookClient) *SlackWebhookPublisher` に改める（`&SlackWebhookPublisher{interval: interval, client: client}` を返す。`CheckRedirect` を含む `http.Client` は `newWebhookClient` が持つ）。本番とテスト用の構築関数の両方がこれを呼ぶ。`Publish` は先頭（`prepareSlackMessages` の前）で `p.client.configured()` を呼び、`SensitiveParts` の導出（`:134`）を消し、送信を `p.client.post(ctx, message)` に置き換える。`waitContext`・`*SlackPostError` の `Total`/`Posted`/`Attempted` の値・`p.interval` を使う待機（`:140`）は変えない。
- [ ] **ステップ 1-5**: `test_helpers_slack.go` に非公開の `newLoopbackWebhookClient(t testing.TB, timeout time.Duration, endpoint string, transport http.RoundTripper) (webhookClient, bool)` を加え、`newSlackWebhookPublisherForLoopbackTest`（`:35-51`）の検査（`loopbacktest.ValidateURL`・`Timeout` の正値・`secret.New`）と `webhookClient` の構築を移す。検査に失敗したら `t.Fatal` を呼んで `false` を返す（`TestNewSlackWebhookPublisherForLoopbackTestRejects` が確かめる「nil を返す」振る舞いを保つ）。`newSlackWebhookPublisherForLoopbackTest` は `newLoopbackWebhookClient` を呼び、`false` なら nil を、そうでなければ `newSlackWebhookPublisher(opts.Interval, client)` を返す。**`SlackTestOptions.Interval` を `interval` に渡すこと**（`slack_test.go:931` ほかの「メッセージの間の待機中の中断」を確かめる既存テストは `Interval` を引き継ぐことを前提にする）。失敗のメッセージの共通の語は `slack test client: ...` に改める（URL は含めない）。
- [ ] **ステップ 1-6**: `internal/publisher` の既存テストを 1 行も変えずに通す（`make test`）。`SlackWebhookPublisher` のフィールドを直接参照するテストがないことを前提とするが、万一 `make test` が落ちた場合は、テストの内容を変えずにテストの組み立てだけを直し、その旨をコミットメッセージに記録する。
- [ ] **ステップ 1-7**: 残存参照とコメントを整える。`rg -n "postMessage" internal/publisher` の結果が doc コメントの「Moved from」の言及を除いて 0 件であること、`rg -n "postFailure" internal/publisher` の結果が `webhookClient.postFailure` の定義と `post` の中の呼び出しだけであること（`Publish` の送信が `p.client.post` に置き換わっていること）を確かめる。`postMessage` の doc コメント・`postFailure` の doc コメント・`SlackWebhookPublisher` の doc コメントを、移動後の事実に合わせる。

**完了条件:** `make fmt`・`make test`・`make lint` が通る。`internal/publisher` の既存テストが内容を変えずに通る（記事の投稿の振る舞いが変わっていないことの確認）。

### PR-1 作成ポイント: shared webhook send path extraction

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `refactor(0016): extract the shared webhook send path`

**レビュー観点**: `webhookClient` に移す送信・URL の取り出し・`SensitiveParts` の導出・`CheckRedirect`・8,193 バイトの上限・`ok` の判定が architecture §3.1 のとおりで、記事の投稿の振る舞いを変えていないこと / `post` が `ctx.Err()` を確かめず、`*SlackPostError` の `Total`・`Posted`・`Attempted` の意味を変えていないこと / `internal/publisher` の既存テストを 1 行も変えずに通していること / テスト用の構築関数の検査を `newLoopbackWebhookClient` に切り出しても、失敗メッセージが URL を含まず nil を返す振る舞いを保っていること

**実装モデル要件**: standard

**判定理由**: 振る舞いを変えない切り出しであり、移す処理は architecture §3.1 に確定し、既存テストを無変更で通すことで固定される。前例のない設計判断・panel-mode トリガー・競合する実装案・分離された高リスク／複雑なステップのいずれにも該当しない。該当しうる Conditional check はビルドタグ下の非 `_test.go` のソース（`test_helpers_slack.go`）の 1 つだけで、2 つには達しない（`make test` が `-tags test` で読むため完了条件で満たされる）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 通知の実装

**対象ファイル:** `internal/publisher/slack_failure.go`（新規）、`internal/publisher/slack_errors.go`（変更）、`internal/publisher/test_helpers_slack.go`（変更）、`internal/publisher/slack_test.go`（補助関数の変更）、`internal/publisher/slack_failure_test.go`（新規）、`internal/publisher/publisher.go`（変更）

**前提:** ステップ 2-1 の先頭で §2.0 の前提（`internal/failure` の型）を確かめる。

- [ ] **ステップ 2-1**: `slack_failure.go` に `SlackFailureNotifier`（フィールドは `client webhookClient` だけ）と `NewSlackFailureNotifier(webhookURL secret.Secret) (*SlackFailureNotifier, error)` を加える。構築は `checkWebhookURL` を通し、`newWebhookClient(webhookURL, SlackPostTimeout, nil)` を持つ値を返す。ネットワークにも環境変数にも触れない（AC-12）。`var _ failure.Notifier = (*SlackFailureNotifier)(nil)` を置く。
- [ ] **ステップ 2-2**: `Kind` から種類の文への対応を、非公開の関数の網羅的な `switch` で書く。文は architecture §3.3 の表のとおり。`default` は `KindUnknown` と同じ文を返す（AC-02・AC-04）。文は英語の固定の一文とする。
- [ ] **ステップ 2-3**: `Notice` から文面を組み立てる処理を書く。`RequestID` が `^[0-9a-f]{16}$` に一致しないとき、`VideoID` が空でなく `transcript.NormalizedVideoURL(VideoID)` が `ok=false` を返すときは、何も送らずに `ErrSlackInvalidNotice` を包むエラーを返す（値は書かない）。`VideoID` が空のときは 3 行目を書かず、空でないときは `NormalizedVideoURL` の返した URL を `<` と `>` で囲んで載せる（AC-03）。時刻・リクエストの値・エラーメッセージ・パス・秘密情報は載せない（architecture §3.3）。
- [ ] **ステップ 2-4**: `Notify(ctx context.Context, notice failure.Notice) error` を書く（I-01）。architecture §3.4 の契約（無効な通知を送らない、送信は多くとも 1 回、結果を番兵に写す、届いたかどうかをエラーの文から読み取れる、状態を持たず記録しない）を満たす処理手順を決める。少なくとも次を含める。受け手が nil・ゼロ値のときは `errZeroNotifier` を返して送らない。`Notice` の形が合わないときは、`ctx` の状態によらず `ErrSlackInvalidNotice` を返して送らない（architecture §3.4。形の検査を `ctx` の確認より先に置く）。`ctx` が既に取り消されている・期限を過ぎているときは `context.Canceled`・`context.DeadlineExceeded` を返して送らない。送信は `client.post` を 1 回だけ呼ぶ。`ErrSlackHTTPStatus`・`ErrSlackInvalidResponse` と判定できるエラーは `ErrSlackNoticeRejected` と元のエラーの両方で判定できる形（複数の `%w`）に包む。送信の途中の時間切れ・中断・通信の失敗は、architecture §4.2 の「届いたかどうか」を示す接頭辞（送っていない場合は `the failure notice was not sent`、届いたかもしれない場合は `the failure notice may have been posted`）に従う。エラーにも出力にも Webhook URL を含めない。`Notify` は `*SlackPostError` を返さない。
- [ ] **ステップ 2-5**: `slack_errors.go` に `ErrSlackInvalidNotice`・`ErrSlackNoticeRejected` を加える（architecture §4.1 の文言）。冒頭のコメント "Sentinels for SlackWebhookPublisher"（`:8`）を、送信の経路と通知の実装の番兵であることが分かる形に改める。`errZeroNotifier` は `slack_failure.go` に非公開で置く（メッセージ "webhook: the notifier has no webhook URL"）。
- [ ] **ステップ 2-6**: `test_helpers_slack.go` に、非公開の `newSlackFailureNotifierForLoopbackTest(t testing.TB, timeout time.Duration, endpoint string, transport http.RoundTripper) *SlackFailureNotifier` と、公開の `NewSlackFailureNotifierForLoopbackTest(t testing.TB, timeout time.Duration, endpoint string) *SlackFailureNotifier` を加える。どちらも `newLoopbackWebhookClient`（ステップ 1-5）を使う。公開する方は transport を取らない（`NewSlackWebhookPublisherForLoopbackTest` と同じ形）。`slackwebhook.ValidURL` は当てないので `http` の loopback を使える。`timeout` を取り `SlackTestOptions` を取らないのは、通知に関係するのが時間の上限だけのためである（architecture §3.2）。
- [ ] **ステップ 2-7**: `slack_test.go` の `requireErrorOmitsEndpoint`（`:210-223`）を、`errors.Unwrap` の枝だけでなく `interface{ Unwrap() []error }` の枝もたどるように改める。記事の投稿のエラーは `Unwrap() []error` を持たないので、既存の呼び出し元の結果は変わらない。他の既存テストの内容は変えない。
- [ ] **ステップ 2-8**: `//go:build test` の `slack_failure_test.go` に architecture §7.1・§7.3 のテストを書く。`httptest` の既存補助（`newSlackServer`・`newSlackOKServer`・`slackWebhookPath`・`slackPayloadText`・`requireSlackPayloadShape`・`requireErrorOmitsEndpoint`）と `newSlackFailureNotifierForLoopbackTest` を使う。対象は architecture §7.1 の表の各行（文面の表・値の形の検査・ゼロ値・送信の形・応答の拒否・リダイレクト・時間の上限と `ctx`・再試行しない・構築・テスト用の構築の拒否・失敗の後の再利用・並行の呼び出し）と、§7.3 のセキュリティテスト（AC-05・AC-06・AC-13）。目印は既存の `slackWebhookMarker`・`slackWebhookPath` を使う。AC-10 の時間の上限は短い値（たとえば 100 ミリ秒）で構築して確かめ、呼び出し元の `ctx` には長い期限（たとえば 1 時間）を付ける。
- [ ] **ステップ 2-9**: `publisher.go` のパッケージコメント（`:1-2`）に、`internal/publisher` が失敗の通知の実装も置くことを加える。

**完了条件:** `make fmt`・`make test`・`make lint` が通る。`slack_failure_test.go` の各テストが architecture §7.1・§7.3 を満たす。

### PR-2 作成ポイント: Slack failure notifier implementation

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9

**推奨タイトル**: `feat(0016): add the Slack failure notifier`

**レビュー観点**: `Notify` の検査の順序（受け手のゼロ値 → 値の形 → `ctx`）と、形の誤りを `ctx` の状態によらず `ErrSlackInvalidNotice` とする契約（architecture §3.4）を満たしていること / 文面が種類の文（固定の定数）と検証済みの `RequestID`・`VideoID` だけで組み立てられ、Webhook URL・リクエストの値・秘密情報・時刻を含まないこと（AC-05・AC-06） / 応答の拒否を `ErrSlackNoticeRejected` と元のエラーの両方で判定できる形（複数の `%w`）に包み、`Unwrap() error` と `Unwrap() []error` の両方をたどる拡張した `requireErrorOmitsEndpoint` で URL が現れないことを確かめていること / 送信が多くとも 1 回で、再試行もリダイレクトの追跡もなく、同じ値からの並行の `Notify` が安全であること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-1〜2-4 の通知の実装は Webhook URL（唯一の秘密）を扱うセキュリティの中核で、誤ると URL がエラーや文面に漏れる。ステップ 2-4 の `Notify` は、検査の順序・エラーの包み方・`ctx` の確認の置き場所を自ら決め、複数の `%w` の包みと並行の呼び出しへの安全を扱う、独立した高リスクで込み入ったステップである。これをこの PR に隔離する（`frontier-recommended` の「分離された高リスク／複雑なステップ」に該当）。あわせて、ステップ 2-6 はビルドタグ下の非 `_test.go` のソース（`test_helpers_slack.go`）の追加という Conditional check にも該当する。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: guard と静的な検査

**対象ファイル:** `internal/failure/failure_test.go`（新規。ただし §2.0 の前提を参照）、`.golangci.yml`（変更）

- [ ] **ステップ 3-1**: `internal/failure` に guard のテストを置く（#109 が同等の guard を既に置いていない場合）。`//go:build test` の `failure_test.go` に、architecture §3.6 の 4 つのテストを書く。`TestNoticeFieldSet`（`reflect.TypeFor[Notice]()` の exported フィールドが `RequestID string`・`VideoID string`・`Kind`（`reflect` が返す型名は `failure.Kind`）の 3 つだけ）、`TestNotifierMethodSet`（`reflect.TypeFor[Notifier]()` のメソッドが `Notify` の 1 つで、型が `func(context.Context, Notice) error`）、`TestKindZeroValue`（`Kind` のゼロ値が `KindUnknown`）、`TestKindConstants`（パッケージの `_test.go` 以外のすべての Go のファイルを `go/parser` で読み、`Kind` 型の定数の名前を宣言の順に集めて requirements F-001 の列挙と一致させる。1 つも見つからなければ失敗させる）。失敗したときのメッセージは英語で書き、architecture §3.6 の表の第 3 列が伝える内容（その guard が失敗したときに、読む人に何を直せばよいかを伝える内容）を含める（表の第 3 列は日本語の散文なので、そのまま写さず英語にする）。
- [ ] **ステップ 3-2**: `.golangci.yml` の `depguard` に 2 つの規則を加える（architecture §3.7）。`publisher-no-server`（`files: ["**/internal/publisher/**"]`、`list-mode: lax`、`deny: github.com/isseis/yt2column/internal/server`。AC-17）。`publisher-no-log`（`files: ["**/internal/publisher/**", "!$test"]`、`list-mode: lax`、`deny: log` と `log/slog`。AC-15）。既存の `deps` と `*-confinement` の規則は変えない。
- [ ] **ステップ 3-3**: `depguard` の規則が実際に効くことを確かめる。`internal/publisher` の非テストのファイルに一時的に `log`（または `log/slog`）を import して `make lint` が `publisher-no-log` で落ちることを確かめる（これは `files` の glob `**/internal/publisher/**` が絶対パスに一致することの確認でもある。落ちなければ glob を直す）。次に、`publisher-no-server` は `internal/server` がまだ無いので、`deny` の対象を一時的に既存の `github.com/isseis/yt2column/internal/failure` に変え、`publisher-no-server` が同じ glob と `list-mode` の下で落ちることを確かめる（PR-3 はフェーズ 2 の後に行うので、`slack_failure.go` が既に `internal/failure` を import しており、一時的な import は要らない）。確認の後、`deny` の対象と（加えた場合は）一時的な import を戻し、`make lint` が通ることを確かめる。（`internal/server` を実際に import する違反の確認は、#109 が `internal/server` を新設した後でないとできない。これは `internal/publisher` が `internal/server` を import しないことを保つ予防的な規則である。）あわせて、`depguard` では禁じられない AC-15 の標準出力・標準エラー出力の部分を確かめる。`internal/publisher` の非テストの Go ファイル（送信の経路を含む）をレビューし、`fmt.Print*` や `os.Stdout`・`os.Stderr` への書き込みがなく、`SlackFailureNotifier` がロガーも `io.Writer` も持たないこと（architecture §3.2・§3.7）を確認して、その結果を記録する。

**完了条件:** `make fmt`・`make test`・`make lint` が通る。`make lint` が 2 つの新しい規則を読み込む。ステップ 3-3 の確認を済ませる。あわせて、AC-15 の標準出力・標準エラー出力の部分を、`internal/publisher` の非テストの Go ファイルのレビューで確かめ、その結果を記録する（architecture §3.7）。

### フェーズ 4: #109 への連絡

**対象ファイル:** なし（#109 の issue への記録）

- [ ] **ステップ 4-1**: #109 の issue に、architecture §8 の手順 4 の (a)〜(f) を記録する。(a) 通知の実装の名前（`publisher.NewSlackFailureNotifier`）。(b) architecture §3.3 の文面（#109 §4.2 の例を改めるため。文面と種類の文は architecture §3.3 からそのまま写し、写した内容が §3.3 と一致することを確かめる）。(c) `package_reference.md` の `internal/publisher` の行に通知の実装を加えること（#109 §3.12）。(d) リクエスト ID の形 `^[0-9a-f]{16}$` を本実装（architecture §3.4 の設計）が検査していることと、#109 が生成したリクエスト ID を実物の `SlackFailureNotifier` に通すテストを置くこと、そのテストが使う公開のテスト用の構築関数 `NewSlackFailureNotifierForLoopbackTest` を本タスクが用意すること。(e) 通知が `silent` のため利用者に気づかれにくいこと（architecture §5.4）。(f) #109 の手動の確認で通知の表示（`<URL>` のリンクと改行）を Mattermost で確かめること。

**完了条件:** #109 の issue に (a)〜(f) の記録がある（issue の URL を控える）。(b) の文面と種類の文が architecture §3.3 と一致する（記録を §3.3 と照合する）。

### PR-3 作成ポイント: failure contract guards, depguard rules, and #109 handoff

**対象ステップ**: 3-1 / 3-2 / 3-3 / 4-1

**推奨タイトル**: `chore(0016): guard the failure contract and record it for #109`

**レビュー観点**: `internal/failure` の guard が architecture §3.6 の 4 つのテスト（`Notice` のフィールド集合・`Notifier` のメソッド集合・`Kind` のゼロ値・`Kind` 定数の並び）を実装し、#109 の手順 1 の PR が同等の guard を置いている場合は重複させないこと / `depguard` の 2 つの規則（`publisher-no-server`・`publisher-no-log`）が architecture §3.7 のとおりで、既存の `deps`・`*-confinement` の規則を変えていないこと / ステップ 3-3 で規則が実際に効くことを（`deny` の対象と非テストの import を一時的に変えて）確かめ、確認後に戻して `make lint` が通ること / ステップ 4-1 の記録が architecture §8 の手順 4 の (a)〜(f) を含み、(b) の文面と種類の文が §3.3 と一致すること

**実装モデル要件**: standard

**判定理由**: guard は `internal/pipeline` の既存の guard と同じ考え方で architecture §3.6 に確定しており、`depguard` の規則も architecture §3.7 に確定している。#109 への記録（ステップ 4-1）も architecture §8 の手順 4 の列挙に従う。前例のない設計判断・panel-mode トリガー・競合する実装案・分離された高リスク／複雑なステップのいずれにも該当せず、Conditional check にも該当しない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 | 対応するフェーズ |
|---|---|---|
| M1: 送信の経路が 1 つになる | `webhookClient` と、それを共有する `SlackWebhookPublisher`（マージ済み。記事の投稿の振る舞いは不変） | フェーズ 1 |
| M2: 通知の実装が動く | `SlackFailureNotifier`・番兵・テスト用の構築関数・テスト（マージ済み） | フェーズ 2 |
| M3: 契約が静的に固定される | `internal/failure` の guard と `.golangci.yml` の `depguard` の規則（マージ済み） | フェーズ 3 |
| M4: #109 が本タスクの契約を知る | #109 の issue の記録 | フェーズ 4 |

- フェーズ 1 はフェーズ 2・3 の前提である（architecture §8）。
- フェーズ 1 は `internal/failure` に依存しないので、#109 の手順 1 の PR のマージを待たずに着手できる。
- フェーズ 2 とフェーズ 3 は互いに独立である（architecture §8）。ただし PR の順序は §3.2 のとおり PR-2 → PR-3 とし、フェーズ 3 のステップ 3-2・3-3 は、フェーズ 2 の後に走らせて `depguard` の効果を実物のコードで確かめる。
- フェーズ 4 はフェーズ 2 の後（実装名と文面が確定した後）に行う。

### 3.2 PR 構成

PR はフェーズを単位とし、コードの差分を生まないステップ 4-1 は PR-3 に含めて独立した PR にしない（PR は 3 つである）。各 PR は単一の関心事（送信の経路の切り出し / 通知の実装 / 契約の guard・`depguard`・#109 への記録）を持ち、単独でグリーンゲートを通せる単位とする。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | `internal/publisher/slack.go` に `webhookClient`・`newWebhookClient`・`configured`・`post`・`checkWebhookURL` を切り出し、`SlackWebhookPublisher` を `interval` と `client` に切り替える。`test_helpers_slack.go` に `newLoopbackWebhookClient` を切り出す。既存テストは無変更で通す | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 | `internal/publisher/slack_failure.go`（新規）に `SlackFailureNotifier`・`NewSlackFailureNotifier`・`Notify` と種類の文の `switch` を加え、`slack_errors.go` に番兵を、`test_helpers_slack.go` にテスト用の構築関数を、`slack_failure_test.go`（新規）に §7.1・§7.3 のテストを加える。`requireErrorOmitsEndpoint` を `Unwrap() []error` 対応にする | frontier-recommended |
| PR-3 | 3-1 / 3-2 / 3-3 / 4-1 | `internal/failure/failure_test.go`（新規）に guard を、`.golangci.yml` の `depguard` に 2 つの規則を加え、#109 の issue に本タスクの契約を記録する | standard |

PR-1 は `internal/` の送信の経路を切り出し、PR-2 はそれを使う通知の実装を加えるので、PR-1 → PR-2 の順とする。PR-2 は PR-1 の `webhookClient` に依存する。PR-3 の guard と `depguard` は PR-1・PR-2 のどちらにも依存しないが、architecture §8 の順序（guard と静的な検査を通知の実装の後）に従い PR-3 に置く。ステップ 4-1（#109 への記録）は実装名と文面が確定した後に行う必要があり、コードの差分を生まないので、独立した PR にせず、契約を締める PR-3 に含める。`internal/` の変更（PR-1・PR-2）は、それを使う層より先に着地する。

PR-2 は、実装（ステップ 2-1〜2-4）とその契約を確かめるテスト（ステップ 2-8）を分けない。`SlackFailureNotifier` と `Notify` の振る舞いを観測できるのは `Notify` を通す経路だけで、テストが契約そのものであるため、実装とテストを同じ PR に置く（interface・実装・テストの密結合した単位を分けない）。応答の拒否・リダイレクト・時間の上限・再試行・セキュリティのテストを別の PR に切り出すこともできるが、それらはステップ 2-6 の共通のテスト用の構築関数とステップ 2-7 の `requireErrorOmitsEndpoint` の拡張を共有するので、切り出してもレビューの単位は小さくならない。したがって PR-2 の大きさは意図的である。

PR-3 のグリーンゲートは、コードの変更（ステップ 3-1〜3-3 の guard と `depguard`）を対象とする。ステップ 4-1 は #109 の issue への記録でリポジトリの差分を生まないため、PR-3 の非コードのチェック項目として扱う。

### 3.3 ブランチとコミットの構成

- フェーズ 1 は 1 つのコミットにする（architecture §8 の手順 1「1 つのコミットにする」）。振る舞いを変えないリファクタリングとしてレビューできるようにする。
- フェーズ 2・3 は、`internal/failure` がマージされた後に、フェーズ 2 とフェーズ 3 をそれぞれ別のコミットにする。
- PR の境界と実装モデル要件は §3.2 と各 `### PR-N 作成ポイント` のとおりである。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

実際の Webhook を呼ばない。`net/http/httptest` の既存補助（`newSlackServer`・`newSlackOKServer`）と、`newSlackFailureNotifierForLoopbackTest`（transport の差し込み口つき）を使う。対象と主なテストは architecture §7.1 の表のとおりで、本計画のフェーズ 2（ステップ 2-8）が実装する。AC との対応は §5 の表に記す。

### 4.2. 統合テスト

本タスクでは、実際の Webhook へ投稿する統合テストを加えない（architecture §7.2）。通知は記事の投稿と同じ送信の経路で、同じ形のペイロード（`text` と `silent`）を送る。文面の Markdown は改行と `<URL>` の自動リンクだけである。この経路・ペイロードと、この 2 つの Markdown の要素が Mattermost で受理されることは、0006 の統合テスト（`make test-integration-slack`）が確かめている。通知の表示（`<URL>` のリンクと改行）は #109 の手動の確認に含める（フェーズ 4 の (f)）。新しい外部の機能を使わない。

### 4.3. セキュリティテスト

architecture §7.3 の 3 つの検査を、フェーズ 2 のステップ 2-8 の `slack_failure_test.go` に書く。

- **AC-05**: 目印を含む Webhook URL（`slackWebhookPath`）で構築し、すべての `Kind` について `Notify` する。受け取ったすべての `text` に、URL・パス・末尾 8 文字・`slackwebhook.SensitiveParts` の各要素が現れないことを確かめ、文面の全体を表で照合する。
- **AC-06**: 受け取ったすべての `text` に `rejectMention`（`slack_message.go:142-149`）を当てて nil であること、`@`・`&`・`|`・`!` を含まないことを確かめる。
- **AC-13**: 同じ目印の URL で、AC-08 の各応答・3xx・時間切れ・接続の拒否を起こす。返されたエラーから `Unwrap() error` と `Unwrap() []error` の両方をたどって得られるすべてのエラーの `Error()` に `SensitiveParts` の各要素が現れないことを、拡張した `requireErrorOmitsEndpoint`（ステップ 2-7）で確かめる。`SlackFailureNotifier` の値を `%v`・`%+v`・`%#v`・`%s` で書いても目印が現れないことも確かめる（`TestSlackPublishErrorsOmitWebhookURL` の `struct formatting` の部分テストと同じ形）。

### 4.4. テストヘルパー

新しい `testutil/` ファイルは追加しない。通知の実装のテスト用の構築関数は、既存の `test_helpers_slack.go`（`//go:build test`）に加える（`test_organization.md` の分類 B）。`internal/failure/failure_test.go` は同パッケージの `_test.go` で、`//go:build test` を付ける。`TestFakesCarryBuildTag` が数える `testutil/` のファイル数（`pipeline_test.go:558` の 17）は変わらない。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

各 AC の検証を実行する。`test` は、実行され、誤った振る舞いで落ちるテストを指す。`static` は、実行される guard・`make` ターゲットを指す。`manual` は、PR での観察を指す。ファイルのパスは `path::TestName` 形式で記す。

| AC | 検証の区分 | 検証 |
|---|---|---|
| AC-01 | test | `internal/failure/failure_test.go::TestNoticeFieldSet`・`::TestNotifierMethodSet`・`::TestKindZeroValue`（ステップ 3-1。`#109` の手順 1 の PR が同等の guard を置いていればそれを使う） |
| AC-01a | test | `internal/failure/failure_test.go::TestKindConstants`（ステップ 3-1） |
| AC-02 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyText`（12 の `Kind` と列挙にない値の文面を表で照合する。ステップ 2-2・2-3・2-8） |
| AC-03 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyText`（`VideoID` が空・非空・`_` 終わりの各場合） |
| AC-04 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyText`・`::TestSlackFailureNotifyZeroValue`（`KindUnknown`・列挙にない値でパニックしない） |
| AC-04a | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyInvalidNotice`（`RequestID` が空・15 文字・17 文字・大文字・`<!channel>` を含む、`VideoID` が 10 文字・12 文字・`/` を含む。取り消し済みの `ctx` でも `ErrSlackInvalidNotice` と判定できる） |
| AC-05 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyOmitsWebhookURL`（§4.3 の AC-05 の検査） |
| AC-06 | test / static | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyOmitsMention`（全 `Kind` の文面に `rejectMention` を当て、メンションの記法の材料になる `@`・`&`・`!`・半角の縦棒を含まないことを確かめる）。静的な確認は、文面が architecture §3.3 の種類の文（固定の定数）と、検査を通った `[0-9a-f]{16}` の `RequestID` と `[A-Za-z0-9_-]{11}` から導いた URL だけで組み立てられ、他に文字を足す経路がないことである（ステップ 2-2・2-3） |
| AC-07 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyPayload`（`requireSlackPayloadShape` を使う） |
| AC-08 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyRejected`（`200` で `""`・`OK`・`ok\n`・`{"ok":true}`、`400`・`403`・`500`、8,193 バイトの本文。`ErrSlackNoticeRejected` と判定できる。`ErrSlackTransport` の場合は判定できない） |
| AC-09 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyRedirect`（`302` と別の送信先の `Location`。リダイレクト先のリクエストの数が 0） |
| AC-10 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyTimeout`（応答を返さない送信先、上限 100 ミリ秒、`ctx` の期限 1 時間。`context.DeadlineExceeded` と判定できるエラーが有限の時間で返る。取り消し済みの `ctx` では送信先のリクエストの数が 0） |
| AC-11 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyNoRetry`（AC-08・AC-09・AC-10 の各場合で送信先のリクエストの数がちょうど 1、取り消し済みの `ctx` では 0） |
| AC-12 | test | `internal/publisher/slack_failure_test.go::TestNewSlackFailureNotifier`（ゼロ値の `secret.Secret`・`http://`・ホストのない URL・制御文字を含む URL でエラー。メッセージに URL もその一部も現れない。`https` の正しい URL で構築しても TLS のテスト用の送信先のリクエストの数が 0）と `::TestNewSlackFailureNotifierForLoopbackTestRejects`（loopback でない URL・非正の `timeout`・空の URL を拒否し nil を返す） |
| AC-13 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyOmitsWebhookURL`（§4.3 の AC-13 の検査。`requireErrorOmitsEndpoint` の `Unwrap() []error` 対応を含む） |
| AC-14 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyRejected`（非 nil のエラーを返し、パニックしない） |
| AC-15 | static / manual | `make lint` の `depguard` 規則 `publisher-no-log`（ステップ 3-2。`internal/publisher` の非テストのファイルが `log`・`log/slog` を import しない）。標準出力・標準エラー出力へ書かないことは `depguard` では禁じられない（`fmt`・`os` は他の目的でも使う）ので、`internal/publisher` の非テストの Go ファイルのレビューで確かめる（ステップ 3-3。architecture §3.7）。`SlackFailureNotifier` がロガーも `io.Writer` も受け取らないことは型の定義（ステップ 2-1）で確認する |
| AC-16 | test | `internal/publisher/slack_failure_test.go::TestSlackFailureNotifyReuse`（`500` の後に `ok` を返す送信先で、2 回目の `Notify` が成功しリクエストの数が 2） |
| AC-17 | static | `make lint` の `depguard` 規則 `publisher-no-server`（ステップ 3-2。`internal/publisher` が `internal/server` を import しない） |
| AC-18 | static / manual | `make test` が `cmd/yt2column` の既存テスト（`cmd/yt2column/docs_test.go::TestREADMEDocumentsFlags` が `newFlagSet` の全フラグを README の表に求める）を無変更で通すこと。`cmd/yt2column` の差分がないことは PR で確認する（manual。フェーズ 1〜3 で `cmd/yt2column` を変更しない） |

- フェーズ 2・3 でも、`make test` と `make lint` を完了条件とする（§1.2）。
- 並行の呼び出しのテスト（`slack_failure_test.go` の 1 つの値からの複数 goroutine の `Notify`）は特定の AC に対応しないが、architecture §3.4 の「状態を持たず、記録しない」を確かめるのでフェーズ 2 のステップ 2-8 に含める。
- AC-15・AC-17 は文書の存在ではなくコードの構造（`internal/publisher` の import と、通知の実装がロガー・`io.Writer` を持たないこと）の不変条件である。実行される検証は `make lint` の `depguard` の規則と、`SlackFailureNotifier` の型の定義（ステップ 2-1）であり、architecture §3.7 の 2 つの規則がこれに当たる。AC-15 の標準出力・標準エラー出力の部分（`fmt.Print*` と `os.Stdout`・`os.Stderr` への書き込みがないこと）は `depguard` では禁じられないので、`internal/publisher` の非テストの Go ファイルのレビューで確かめる（ステップ 3-3。architecture §3.7）。
- AC-18 は「`cmd/yt2column` を変更しない」という差分の性質である。実行される検証は `make test` が `cmd/yt2column` の既存テスト（`TestREADMEDocumentsFlags` を含む）を無変更で通すことで、`cmd/yt2column` の差分がないことは PR で確認する（manual）。`cmd/yt2column` に失敗の投稿の経路を足さないことを import の検査で固定することは architecture §3.7 が定めておらず、`cmd/yt2column` を変更しないという要件とも矛盾するので、本タスクでは行わない（規則を足す場合は architecture の改訂が要る）。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| `internal/failure` がまだ無く、フェーズ 2・3 に着手できない | フェーズ 2・3 がブロックされる | フェーズ 1（`internal/failure` に依存しない）を先に進める（§2.0）。フェーズ 2・3 は #109 の手順 1 の PR のマージ後に着手し、ステップ 2-1 の冒頭で型の存在を確かめる |
| フェーズ 1 のリファクタリングが記事の投稿の振る舞いを変える | 記事の投稿が壊れる（本タスクの対象外の振る舞い） | `internal/publisher` の既存テストを 1 行も変えずに通すことを完了条件にする（ステップ 1-6）。`Publish` の `*SlackPostError` の `Total`/`Posted`/`Attempted` の値を変えない |
| `post` が `ctx.Err()` を確かめるようになり、`*SlackPostError.Attempted` の意味が変わる | 記事の投稿のエラーの `Attempted` が、HTTP クライアントを呼ばなかった場合にも `true` になりうる | architecture §3.1 のとおり、`post` 自身は `ctx` を確かめない。取り消し済みの `ctx` を送らないことは `Notify`（ステップ 2-4）の責務にする |
| `depguard` の `files` の glob が絶対パスに一致せず、規則が効かない | AC-15・AC-17 が満たされず、レビューでは気づけない | ステップ 3-3 で、`log` の一時的な import により `publisher-no-log` が落ちること（glob の確認）と、`publisher-no-server` の `deny` の対象を一時的に既存のパッケージに変えて落ちることを確かめる。落ちなければ `**/internal/publisher/**` の glob を直す |
| `internal/server` がまだ無く、`publisher-no-server` の違反を実物で確認できない | 規則が実際に効くことを `internal/server` で確かめられない | ステップ 3-3 で `deny` の対象を既存のパッケージに変えて規則の形（glob・`list-mode`・`deny`）を確かめる。`internal/server` を新設する #109 の後に、実物での確認が可能になる（規則は予防的な宣言である。architecture §3.7） |
| `ErrSlackNoticeRejected` を複数の `%w` で包むことで `Unwrap() []error` ができ、AC-13 の「`errors.Unwrap` でたどれるすべてのエラー」の検査が届かない | AC-13 が一部のエラーを見逃す | ステップ 2-7 で `requireErrorOmitsEndpoint` を `Unwrap() []error` にも対応させる（architecture §4.1・§7.3） |
| `Kind` の定数の並びを固定する guard が、#109 のファイルの分け方に依存する | guard が誤って落ちる、または何も確かめずに通る | ステップ 3-1 で `TestKindConstants` をパッケージのすべての非テストファイルを読む形にし、1 つも見つからなければ失敗させる（architecture §3.6）。#109 が同等の guard を既に置いていればそれを使う |
| `internal/failure` の guard が #109 の手順 1 の PR と重複する | 同じことを 2 か所で確かめる | ステップ 3-1 の先頭で #109 の PR を確認し、同等の guard があれば加えない（architecture §3.6） |
| `depguard` の `publisher-no-log` が `_test.go` にも当たる | テストが `log` を使えなくなる、または規則の意図と違う | `files` に `!$test` を付ける（ステップ 3-2。depguard の README で確認した `!` の否定） |
| 新しい `Kind`（#109 が後で足す）で文面の対応が漏れる | 通知が「unclassified reason」で投稿される（通知自体は失われない） | 文面の `switch` の `default` を `KindUnknown` と同じ文にする（AC-04）。`TestKindConstants` が並びの変化を検出する（architecture §9） |

### 6.2. セキュリティリスク

- 通知の文面・エラー・ログのいずれにも Webhook URL とその一部を含めない。AC-05・AC-13 のテスト（§4.3）と、既存の `slackwebhook.SensitiveParts` の対策（`slack.go:233-291` を変更しない）で確かめる。
- 通知の実装は環境変数を読まず、`internal/server` を import せず、`log`・`log/slog` を import しない（AC-15・AC-17）。
- リダイレクトに従わない（AC-09）。応答の本文の読み込みに上限を設ける（AC-08）。

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）。フェーズ 1 の完了条件を満たす
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9）。フェーズ 2 の完了条件を満たす
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 4-1）。フェーズ 3 の完了条件を満たし、#109 の issue に記録した
- [ ] §5 のすべての AC の検証の区分（`test`・`static`）が実行され、通る
- [ ] `make fmt`・`make test`・`make lint` がすべてのフェーズの完了時に通る

## 8. 成功基準 (Success Criteria)

- **機能:** `SlackFailureNotifier.Notify` が architecture §3.4 の契約を満たし、AC-02〜AC-04a・AC-07〜AC-11・AC-14・AC-16 を満たす。記事の投稿の振る舞いは変わらない（既存テストが無変更で通る）。
- **品質:** `make test`・`make lint` が通り、`internal/failure` の guard（AC-01・AC-01a）と `depguard`（AC-15・AC-17）が通る。`internal/publisher` の既存テストへの影響がない。
- **セキュリティ:** 文面・エラー・ログのいずれにも Webhook URL・リクエストの値・API キー・共有トークンが現れない（AC-05・AC-13）。通知は `silent` で投稿し、文面にメンションの記法を含まない（AC-06）。
- **文書:** 本タスクは文書を更新しない（requirements §2.3）。更新は #109 の F-010 が行う。

## 9. 次のステップ (Next Steps)

- 本書の PR の境界と実装モデル要件（§3.2 と各 `### PR-N 作成ポイント`）に従い、`/runplan 0016` で PR-1 から順に実装する。PR-1 は `internal/failure` のマージを待たずに着手できる。PR-2・PR-3 は #109 の手順 1 の PR のマージ後に行う。
- #109 の手順 1 の PR が同等の `internal/failure` の guard を置いた場合は、ステップ 3-1 を飛ばし、重複するテストを加えない（§2.0）。
