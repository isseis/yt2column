# 要件定義書：失敗を Slack Webhook へ投稿する仕組み

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-14 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、認証済みのリクエストの処理が失敗したときに、その失敗を Slack 互換の Incoming Webhook へ投稿する仕組み（`failure.Notifier` の実装）の要件を定義する。

**背景:**
`yt2column` の `Publisher` は `Article` を投稿するだけで、失敗を報告する経路を持たない。ブラウザ拡張から字幕を受け取り、記事を生成して Slack へ投稿する localhost サーバ（[#104](https://github.com/isseis/yt2column/issues/104)）では、`202 Accepted` を返した後の非同期処理で失敗が起きる。その失敗は応答として利用者に返せないので、Slack へ投稿して知らせる必要がある。

#104 は、コピーした字幕のパーサと `TranscriptSource`（[#107](https://github.com/isseis/yt2column/issues/107)）、本タスク（失敗を Slack へ投稿する仕組み、[#108](https://github.com/isseis/yt2column/issues/108)）、サーバ本体と API（[#109](https://github.com/isseis/yt2column/issues/109)）に分かれている。`0011_localhost_server/02_architecture.md` §3.9 が、サーバが使う失敗の通知の契約（`internal/failure` の `Kind`・`Notice`・`Notifier`）と、`Notifier` の実装に求める性質を定めている。`internal/failure` の型は #109 の最初の PR（同書 §8 の手順 1）が新設し、本タスクはその契約に従って `Notifier` を実装する。本タスクは #109 の前提であり、#109 のサーバ（F-007）が本タスクの実装を使う。

対応 issue: #108（親: #104、#43）。後続: #109。

要件レビューの過程で挙がった、設計で決める事項は [design_handoff.md](design_handoff.md) に申し送る。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `internal/failure` の契約（#109 §3.9）に従い、`Notifier` の実装（Slack への投稿）を作る（F-001・F-003）。
-   失敗の種類（`Kind`）ごとの固定の文と、`RequestID`・動画 ID だけで投稿の文面を組み立てる（F-002）。
-   投稿に秘密情報（API キー・Webhook URL・共有トークン）と受け取った字幕の本文を含めないことを、型（`Notice` がそれらの欄を持たないこと）とテストで保証する（F-002）。
-   投稿自体が失敗したときに、エラーを返し、再試行せず、パニックせず、`Notifier` 自身は出力を書かない（呼び出し元が記録する）ことを定める（F-004）。
-   サーバ（#109）から使う。CLI からも再利用できる形にする（F-005）。

### 2.2. スコープ (In Scope)

-   `failure.Notifier` の実装。既存の送信の経路を共有する（配置と共有の手段は設計で決める。[design_handoff.md](design_handoff.md) H-01・H-03）。
-   `Kind` ごとの固定の文と、`Notice` の `RequestID`・`VideoID`（正規化 URL）だけの文面の組み立て（F-002）。
-   Webhook への送信（`POST`・`Content-Type: application/json`・`silent`、リダイレクトに従わない、応答の上限、成功の判定、送信の時間の上限、再試行しない）（F-003）。
-   構築（Webhook URL の検証、ゼロ値の拒否、テスト用の loopback の構築）（F-003）。
-   ユニットテスト（`net/http/httptest`・`internal/loopbacktest`）。秘密情報も字幕の本文も投稿に現れないこと、投稿自体が失敗したときの振る舞いのテスト（F-002・F-004）。

### 2.3. スコープ外 (Out of Scope)

-   `internal/failure` の型（`Kind`・`Notice`・`Notifier`）の新設。#109 の手順 1 の PR が行う（#109 §8）。本タスクは契約に従い、型のフィールドとメソッドを変更しない。
-   サーバ本体、HTTP API、認証、枠（同時実行数の制限）、失敗の種類の判定（#109）。本タスクはサーバから `Notice` を受け取って投稿するだけである。
-   サーバ側のログと、`Notify` のエラーの記録（#109 §3.10）。
-   CLI の失敗の Slack への投稿（F-005）。再利用できる形にするが、本タスクでは CLI は投稿しない。
-   文書（CLAUDE.md・`project_overview.md`・`security.md`・`package_reference.md`・README）の更新。#109 の F-010 が扱う（#109 §3.12）。
-   記事の投稿（`SlackWebhookPublisher`）の振る舞いの変更。本タスクは送信の経路を共有するが、記事の投稿の結果を変えない。
-   投稿の自動リトライ、通知の集約・抑制（レート制限、重複排除、通知の頻度の制限）。
-   どの `Kind` を通知するかの判断（#109 が呼び出しの場所で決める。§3.9）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: `internal/failure` の契約（#109 §3.9）

-   本タスクは、#109 の設計 §3.9 が定める `internal/failure` の型に従う。型は #109 の手順 1 の PR で新設される（#109 §8）。本タスクは型を変更しない。
-   `Kind` は、失敗の種類を表す列挙型で、ゼロ値は `KindUnknown` とする。値は、#109 §3.9 のとおり `KindUnknown`・`KindBodyTooLarge`・`KindInvalidBody`・`KindInvalidVideoURL`・`KindInvalidTranscript`・`KindTranscriptTooLong`・`KindLLMTimeout`・`KindLLMEmpty`・`KindLLMTruncated`・`KindLLM`・`KindPublish`・`KindInternal` とする。
-   `Notice` は、`RequestID string`・`VideoID string`・`Kind Kind` の 3 つの exported フィールドだけを持つ。`Notice` はリクエストの値（字幕・タイトル・チャンネル名・概要欄）やエラーメッセージを載せる欄を持たない（#109 §3.9「`Notice` は失敗の投稿に載せてよいものすべて。すべての欄はサーバが選ぶか生成し、リクエストから写したものはない」）。
-   `Notifier` は、`Notify(ctx context.Context, n Notice) error` の 1 つのメソッドを持つ interface とする。
-   本タスクは `Notifier` を実装する（F-003）。実装は `internal/server` を import しない（F-005）。

**Acceptance Criteria**:
- **AC-01**: `internal/failure` に `Kind`・`Notice`・`Notifier` がある。`Notice` の exported フィールドは `RequestID`・`VideoID`・`Kind` の 3 つだけで、それ以外の exported フィールドを持たない。`Notifier` のメソッドは `Notify(context.Context, Notice) error` の 1 つだけである。`Kind` のゼロ値は `KindUnknown` である（静的な確認）。本タスクの変更でこれらが増減しない。
- **AC-01a**: `Kind` の値は、`KindUnknown` と、`KindBodyTooLarge`・`KindInvalidBody`・`KindInvalidVideoURL`・`KindInvalidTranscript`・`KindTranscriptTooLong`・`KindLLMTimeout`・`KindLLMEmpty`・`KindLLMTruncated`・`KindLLM`・`KindPublish`・`KindInternal` である。順序と名前は #109 §3.9 のとおり（静的な確認）。

#### F-002: 投稿する内容

-   投稿するメッセージの文面は、`Kind` ごとの固定の文と、`Notice.RequestID`、`Notice.VideoID` から導いた正規化 URL だけで組み立てる。
-   `VideoID` が空のときは、動画の URL を文面に載せない。`VideoID` が空でないときは、`transcript.NormalizedVideoURL` と一致する正規化 URL を載せる（#109 §3.9）。
-   `KindUnknown` と、列挙にない `Kind` の値にも、固定の文を使う。ゼロ値でパニックしない。
-   文面に、リクエストから来た値（コピーした字幕 `transcript`・`title`・`channelName`・`description`）、エラーメッセージの文字列、スタックトレース、ローカルのパス、API キー、Webhook URL、共有トークンを含めない。`Notice` にそれらの欄がないので、載せようがない（型による保証）。
-   時刻は文面に載せない。Slack は投稿の時刻を示し、処理の時刻は #109 のサーバがログに記録する。
-   文面は、メンションの記法（`@channel`・`@all`・`@here`・`@everyone`・`<@U…>`・`<!subteam^…>` など）を含まない。

**Acceptance Criteria**:
- **AC-02**: 16 進 16 文字の `RequestID` と検証済みの `VideoID` を持つ `Notice` を、テスト用の送信先へ `Notify` すると、受け取る `text` は、その `Kind` に対応する固定の文と、`RequestID` と、動画の正規化 URL を含む。すべての `Kind`（`KindUnknown` を含む列挙の各値）について、同じ `Kind` では常に同じ文面になり、異なる `Kind` の文面は互いに区別できる（表で確かめる）。
- **AC-03**: `Notice` の `VideoID` が空のとき、`text` は動画の URL を含まない。`VideoID` が空でないとき、`text` は `transcript.NormalizedVideoURL` と一致する URL を含む。
- **AC-04**: `KindUnknown`（ゼロ値）と、列挙にない `Kind` の値で `Notify` すると、パニックせず、固定の文で投稿する。
- **AC-05**: 目印の値を入れた `RequestID` と、目印の値を入れたリクエストの値（`transcript`・`title`・`channelName`・`description`）・API キー・Webhook URL・共有トークン・エラーメッセージ・ローカルのパスを用いて `Notify` すると、テスト用の送信先が受け取るすべての `text` に、それらの目印が現れない。載りうるのは `RequestID`・`VideoID`（正規化 URL）・`Kind` の文面だけである。
- **AC-06**: 任意の有効な `Notice` で投稿される `text` は、メンションの記法を含まない。固定の文と、`[0-9a-f]{16}` の `RequestID` と、`[A-Za-z0-9_-]{11}` から導いた URL だけで組み立てるためである（静的な確認と、テスト用の送信先が受け取る `text` の検査）。

#### F-003: Webhook への投稿

-   `Notifier` は Webhook URL を `secret.Secret` として構築時に受け取り、環境変数は読まない。値のないゼロ値は構築をエラーにする。`slackwebhook.ValidURL` に合わない URL も構築をエラーにする（`SlackWebhookPublisher` の `NewSlackWebhookPublisher` と同じ規則）。
-   投稿先は記事と同じ Webhook とし、`SlackWebhookPublisher` と同じ送信の性質で送る（#109 §3.9）。
    -   メッセージごとに `POST` し、本文は `Content-Type: application/json` の JSON とする。
    -   ペイロードは、`text` と `silent`（真）を持つ JSON とする。`silent` は Mattermost の通知を抑止する。記事の投稿と同じく、`blocks`・`attachments` などの他の項目は使わない。
    -   リダイレクトに従わない（`http.Client.CheckRedirect` が `http.ErrUseLastResponse` を返す）。3xx の応答は投稿の失敗とし、`Location` の送信先にリクエストを送らない。
    -   投稿の成功は、ステータス `200` で本文がちょうど `ok` のときだけとする。応答の本文は 8,193 バイトまで読み、それを超える本文はステータスが `200` でも失敗とする。
    -   送信の時間の上限を自分で持ち、上限を過ぎたらエラーを返す。上限は 30 秒以下とする。呼び出し元の `ctx` の期限に頼らない（`ctx` がまだ生きていても、自分の上限で打ち切る）。`ctx` がキャンセルされているときはリクエストを送らない。
    -   自動でリトライしない。1 回の `Notify` は、多くとも 1 つのメッセージを送る。
-   構築はネットワークに触れない。テスト用の loopback の送信先へ `http` で送る構築の手段を、テスト用のビルドでだけ使えるようにする（既存の `newSlackWebhookPublisher`・`deepseek.NewForLoopbackTest` と同じ考え方）。
-   送信の経路は `SlackWebhookPublisher` と共有し、複製しない（共有の手段は設計で決める。[design_handoff.md](design_handoff.md) H-01）。
-   返すエラーは、Webhook URL とその一部を含まない（`errors.Unwrap` でたどれるエラーを含む。記事の投稿と同じ保証）。

**Acceptance Criteria**:
- **AC-07**: テスト用の送信先へ `Notify` すると、受け取るリクエストは `POST` で、`Content-Type` が `application/json`、ペイロードは `text` と `silent`（JSON の `true`）を持つ。
- **AC-08**: テスト用の送信先が、`200` で本文が `ok` 以外（空・`OK`・`ok\n`・`{"ok":true}`）、`200` 以外のステータス（`400`・`403`・`500` など）、読み取る上限を超える本文、のいずれかを返すと、`Notify` はエラーを返す。
- **AC-09**: テスト用の送信先が、別のテスト用の送信先を `Location` に指定した 3xx を返すと、`Notify` はエラーを返し、リダイレクト先はリクエストを 1 つも受け取らない。
- **AC-10**: テスト用の送信先が応答を返さないとき、`Notify` は有限の時間（設計で定める 30 秒以下の上限。テストでは短い上限で構築する）でエラーを返す。呼び出し元の `ctx` に長い期限があるときも、自分の上限で打ち切る。キャンセル済みの `ctx` で `Notify` すると、送信先はリクエストを受け取らない。
- **AC-11**: AC-08・AC-09・AC-10 のそれぞれの場合で、テスト用の送信先が受け取るリクエストは 1 つだけである（自動でリトライしない）。
- **AC-12**: ゼロ値の `secret.Secret`、または `slackwebhook.ValidURL` が拒否する URL で構築すると、エラーになる。いずれのエラーのメッセージも、URL の値もその一部も含まない。構築は送信先にリクエストを送らない。
- **AC-13**: 目印の値を入れた Webhook URL（パスに特徴的な文字列を含む）で構築した `Notifier` で AC-08〜AC-10 の各失敗を起こすと、返されたエラーと `errors.Unwrap` でたどれるすべてのエラーの `Error()` の値に、Webhook URL・そのパス・その末尾 8 文字のいずれも現れない。

#### F-004: 投稿自体が失敗したときの扱い

-   `Notify` は、投稿の失敗をエラーとして返す。パニックしない。
-   `Notify` は自動でリトライしない（F-003）。投稿の失敗は呼び出し元が扱う。#109 のサーバは、返されたエラーをログに記録して動作を続ける（#109 の AC-26）。
-   `Notify` 自身は、標準出力・標準エラー出力・ログのいずれにも書かない。失敗は戻り値だけで返す。これにより、投稿の失敗の経路から秘密情報や字幕の本文が出力に流れない。失敗の記録は呼び出し元が行う。
-   エラーは、Webhook URL とその一部を含まない（F-003・AC-13）。

**Acceptance Criteria**:
- **AC-14**: テスト用の送信先が失敗（AC-08 のいずれか）を返すとき、`Notify` は非 nil のエラーを返し、パニックしない。
- **AC-15**: `Notifier` の実装は、ログのための `io.Writer`・`*slog.Logger`・ファイルを受け取らず、投稿の失敗を書き出さない（静的な確認）。失敗の記録は呼び出し元が行う。
- **AC-16**: 投稿の失敗のあと、同じ `Notifier` で改めて `Notify` すると、送信先へ新しいリクエストが届く（失敗が `Notifier` の状態を壊さない）。

#### F-005: CLI からの利用

-   目的は「サーバだけでなく CLI からも使える経路にする」ことである。本タスクは、`Notifier` の実装を `internal/server` に依存させず、CLI からも呼べる形にする。
-   一方、**本タスクでは CLI は失敗を Slack へ投稿しない。** `cmd/yt2column` の振る舞い（標準エラー出力への表示と終了コード）を変えない。理由は次のとおり。
    -   #104 の完了条件と #109 の設計は、失敗の投稿をサーバに限っている（#109 の F-007）。
    -   CLI では、ターミナル（標準エラー出力）へのメッセージと終了コードで利用者に十分に伝わる。
    -   サーバはリクエストごとに `RequestID` を生成するが、CLI には対応する値がない。CLI から投稿すると、`Notice` の契約（すべての欄をサーバが選ぶ・生成する。#109 §3.9）を変える必要がある。
    -   YAGNI（[CLAUDE.md](../../../CLAUDE.md)）。CLI から投稿する必要が生じたときに、`RequestID` を空にする等の契約の変更を #109 と合わせて決める。
-   したがって、`Notifier` の実装は `internal/server` を import しない（#109 §2.1 の循環を避け、CLI からも使える形にする）。

**Acceptance Criteria**:
- **AC-17**: `Notifier` の実装を含むパッケージは `internal/server` を import しない（静的な確認。`go list` の依存の検査）。`internal/server` は `internal/failure` の interface だけに依存する（#109 §2.1）。
- **AC-18**: `cmd/yt2column` に失敗の投稿の呼び出しを加えない。新しいフラグ・環境変数を加えない（静的な確認）。既存の CLI の振る舞いは変わらない。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

本タスクが扱うのは、`Notice`（呼び出し元が生成する値）と、Webhook の応答（信頼できない入力）である。受け取った字幕の本文（`transcript`）などは本タスクの境界に存在しない。#109 のサーバはそれらを `Notice` に載せない（#109 §3.9）。

| 境界 | 受理する形 | 上限 | 拒否時の扱い | 主な AC |
|---|---|---|---|---|
| Webhook の応答 | ステータス `200` で、本文がちょうど `ok`。3xx には従わない | 読み取る本文は 8,193 バイト（1 バイト多く読んで超過を検出する） | 投稿の失敗（エラーを返す。再試行しない） | AC-08・AC-09 |
| `Notice.RequestID` | 呼び出し元（サーバ）が生成する 16 進 16 文字。`Notifier` は中身を検証しない | - | なし（検証は呼び出し元の責務） | AC-02・AC-05 |
| `Notice.VideoID` | 呼び出し元が検証した 11 文字の `[A-Za-z0-9_-]`（空を許す） | - | なし（検証は呼び出し元の責務） | AC-03・AC-05 |
| `Notice.Kind` | 列挙のいずれかの値 | - | 列挙にない値は `KindUnknown` と同じ固定の文 | AC-04 |

-   標準ライブラリが黙って修復・破棄・無視すれば受理できてしまう入力は、受理せずに失敗とする。応答の本文は、失敗の理由として識別子だけを読む場合でも自由文を表示しない（`SlackWebhookPublisher` と同じ。security.md §3）。
-   `Notice` の値は呼び出し元が生成する。`Notifier` は値の中身によって振る舞いを選ばない。`Kind` は列挙と `switch` で文面に写し、`default` は固定の文（`KindUnknown` と同じ）に倒す（[CLAUDE.md](../../../CLAUDE.md)「Declare, don't infer」）。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   投稿の処理は、LLM の呼び出しに比べて無視できる時間で終わればよい。性能のための仕組みは設けない（[CLAUDE.md](../../../CLAUDE.md)「Performance」）。
-   1 回の `Notify` は、送信の時間の上限（30 秒以下）のうちに必ず返る。

### 4.2. セキュリティ (Security)

-   投稿の文面は、固定の文と `RequestID`・動画 ID だけで組み立てる。API キー・Webhook URL・共有トークン・受け取った字幕の本文を、投稿・エラー・ログのいずれにも含めない（[security.md](../../dev/security.md) §2・§3・§8）。
-   Webhook URL は `secret.Secret` で保持し、`https` の URL だけを受理する。エラーの `Error()` と `errors.Unwrap` でたどれるすべてのエラーに、Webhook URL もその一部も含めない（security.md §2）。
-   リダイレクトに従わない（Webhook URL が別のホストへ漏れることを防ぐ。security.md §3）。
-   応答の本文の読み込みに上限を設ける（security.md §3）。
-   通知は `silent` で投稿し、通知を発生させない（#109 §3.9。security.md §6）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   投稿の失敗で `Notifier` は停止しない。`Notify` はエラーを返し、次の呼び出しを処理できる。
-   自動でリトライしない。1 回の `Notify` は多くとも 1 つのメッセージを送る。
-   `Notify` はパニックしない。
-   送信の時間の上限により、応答しない送信先に対しても `Notify` は有限の時間で返る。

### 4.4. 互換性 (Compatibility)

-   Go の標準ライブラリだけで実装する。外部ライブラリを追加しない。
-   macOS と Linux で同じ振る舞いをする。
-   検証の対象は Mattermost の Incoming Webhook とし、Slack へは best effort で送る（`SlackWebhookPublisher` と同じ。0006 §2.2）。`silent` は Slack では効かない。

### 4.5. 保守性 (Maintainability)

-   送信の経路は `SlackWebhookPublisher` と共有し、複製しない（[CLAUDE.md](../../../CLAUDE.md)「DRY」）。
-   `Kind` から文面への対応は、網羅的な `switch`（`default` は固定の文）で行い、エラーメッセージの文字列から推測しない（「Declare, don't infer」）。
-   新しい `Kind` を加えたときに、文面の対応が漏れないようにする（`switch` と、すべての `Kind` を試すテスト）。
-   `Notice`・`Notifier` のフィールドとメソッドの集合（AC-01）を、`internal/pipeline` の guard と同じ考え方で固定する（`Notice` に欄を足して秘密情報や字幕を載せる変更を、テストで気づけるようにする。固定の手段は設計で決める）。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」と [CLAUDE.md](../../../CLAUDE.md) に従う。外部ライブラリを追加しない。
-   依存: `SlackWebhookPublisher` と `internal/slackwebhook`（[#7](https://github.com/isseis/yt2column/issues/7)）、`internal/failure` の型（#109 §8 の手順 1 の PR）。本タスクは、#107・#109 の本体を待たずに、#109 の手順 1 の PR の後で実装できる（#109 §8。本タスクの実装は #109 の手順 2 に並行する）。
-   本タスクは、`0011_localhost_server/02_architecture.md` §3.9 の契約（`Kind`・`Notice`・`Notifier` の性質と、`Notifier` の実装に求める性質）に従う。性質を変える場合は #109 に知らせ、#109 の設計書の修正と再承認を要する。
-   `transcript.Transcript`・`transcript.Segment`・`writer.Article` の exported フィールドを変更しない（`internal/pipeline` の `TestCommonTypesFieldSets` が固定している）。
-   記事の投稿（`SlackWebhookPublisher.Publish`）の振る舞いを変えない。送信の経路を共有するための内部の変更は、`internal/publisher` の既存のテストが通る範囲で行う。
-   ユニットテストは、実際の Webhook も LLM の API も呼ばない。`net/http/httptest` と `internal/loopbacktest` を使う（[CLAUDE.md](../../../CLAUDE.md)「Testing Strategy」）。本番の Webhook URL も API キーも使わない（security.md §2）。

## 6. 用語集 (Glossary)

-   **失敗の通知 / `Notifier`:** 認証済みのリクエストの処理の失敗を Slack へ投稿する仕組み。本タスクが実装するのは `Notifier` の実装（Slack への投稿）である。
-   **`Notice`:** 失敗の通知に載せてよいものすべて。`RequestID`・`VideoID`・`Kind` だけを持つ（#109 §3.9）。
-   **`Kind`:** 失敗の種類を表す列挙型。ゼロ値は `KindUnknown`。
-   **`RequestID`:** サーバがリクエストごとに生成する、推測できない短い識別子（#109 §3.4.2）。応答・ログ・失敗の通知に同じ値を載せ、三者を結び付ける。秘密ではない。
-   **`VideoID`:** 検証済みの 11 文字の動画 ID。失敗の通知では、そこから導いた正規化 URL を載せる。
-   **Slack 互換の Incoming Webhook:** 記事の投稿先と同じ（Mattermost を検証済み、Slack は best effort。0006 §2.2）。
-   **`silent`:** Mattermost の Incoming Webhook のペイロードの項目。真にすると、投稿はチャンネルに表示されるが通知を発生させない。Slack にはない。
-   用語は [translation_glossary.md](../../translation_glossary.md) と統一する。
