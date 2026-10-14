# 設計への申し送り：失敗を Slack Webhook へ投稿する仕組み

本書は、[01_requirements.md](01_requirements.md) の作成とレビューの過程で挙がった事項のうち、要件（観測できる振る舞い）ではなく、設計の段階で決める事項をまとめる。

-   `02_architecture.md` は、本書の各項目について、採用した手段または扱わない理由を記録する。本書の手段は候補であり、設計で別の手段を選んでもよい。ただし、対応する AC を満たすこと。
-   本書は承認対象の文書ではない。設計の作成時に参照し、項目を追加してよい。

## 項目一覧

| ID | 対象 | 概要 | 関連 AC |
|---|---|---|---|
| H-01 | F-003 | 送信の経路を `SlackWebhookPublisher` と共有する手段 | AC-07〜AC-13 |
| H-02 | F-002 | `Kind` ごとの固定の文の文言と、`RequestID`・動画 ID の文面への載せ方 | AC-02〜AC-06（AC-04a を含む） |
| H-03 | F-003・F-005 | `Notifier` の実装の置き場所と、`internal/server` との依存の向き | AC-12・AC-17 |
| H-04 | F-001 | `Notice`・`Notifier` のフィールドとメソッドの集合を固定する guard の置き方 | AC-01 |
| H-05 | F-003・F-004 | 送信の時間の上限の値と、`ctx` との関係（キャンセル・期限） | AC-10・AC-14 |
| H-06 | F-003 | テスト用の loopback の構築の手段と、上限を短くする方法 | AC-10・AC-13 |

## H-01: 送信の経路の共有

-   **事項:** `Notifier` の投稿は、`SlackWebhookPublisher` と同じ送信の性質（`POST`・`Content-Type: application/json`・`silent`・リダイレクトに従わない・応答の上限・`ok` の判定・タイムアウト・URL をエラーに含めない）を持つ（F-003）。この送信の処理を、記事の投稿と `Notifier` の間で共有する手段を決める。
-   **理由:** 同じ処理を 2 か所に書くと、一方だけが変わったときに食い違う（[CLAUDE.md](../../../CLAUDE.md)「DRY」）。`SlackWebhookPublisher.Publish` は `writer.Article` を受け取り、メンションの検査と分割を行うので、短い通知の投稿にはそのまま使えない。内部の送信の関数（`slack.go` の `postMessage` 相当）を、投稿する本文を受け取る形に切り出すことを候補とする。
-   **候補:** (a) `internal/publisher` の中の送信の関数を引数（本文、`silent` は常に真）で共有する。(b) `internal/slackwebhook` に送信の関数を移し、記事の投稿と `Notifier` の両方が使う。いずれの場合も、記事の投稿の振る舞い（既存のテスト）を変えない。現在の `postMessage` は送信の前に `ctx` を確かめない（確認は `Publish` のループにある）ので、共有する関数を `Notify` から呼ぶときは、キャンセル済みの `ctx` で送らないこと（AC-10）を `Notify` 側か共有する関数の側で保証する。
-   **番兵の対応:** AC-08・AC-09 は、応答の拒否を `ErrWebhookResponseRejected`（仮称）と判定できることを求める。共有する経路は既存の `ErrSlackHTTPStatus`（`200` 以外のステータス）・`ErrSlackInvalidResponse`（`200` で本文が `ok` 以外、上限を超える本文）を返すので、`Notifier` が `ErrWebhookResponseRejected` をどう定義するか（両者を包む・別名にする・`internal/failure` に置く）を設計で決める。
-   **関連:** F-003、AC-07〜AC-13

## H-02: `Kind` ごとの固定の文の文言と、文面への載せ方

-   **事項:** `Kind` ごとの固定の文（例: `failure: the body was too large`）の文言と、`RequestID`・動画の正規化 URL を文面のどこにどう載せるか（例: `... (request <ID>, <URL>)`）を決める。
-   **理由:** `Notifier` の実装に求める性質は #109 §3.9 が定めるが、実際の文言は設計で固定する。AC-02 は「`Kind` ごとに区別できる固定の文」を求める。文言は、利用者が Slack の通知から失敗の種類を読み取れるものであればよい。
-   **候補:** 固定の文は `Kind` を識別できる英語の一文とし、`RequestID` と URL を続ける。`KindUnknown` と列挙にない値には「不明」を表す文を使う。文面は、メンションの記法を生まない（`@` を使わない）ものとする（AC-06）。
-   **関連:** F-002、AC-02〜AC-06

## H-03: `Notifier` の実装の置き場所と依存の向き

-   **事項:** `Notifier` の実装をどのパッケージに置くかを決める。#109 §2.1・§3.9 は、`internal/server` が `internal/failure` の interface だけに依存し、#108 の実装が `internal/publisher` に置かれても循環しないとしている。CLI からも使える形にする（F-005・AC-17）必要がある。
-   **候補:** `internal/publisher`（`SlackWebhookPublisher` と同じパッケージ。送信の経路を共有しやすい）。この場合、`internal/publisher` が `internal/failure` を import し、`internal/server` は両方を import する。`internal/publisher` が `internal/server` を import しないことを確かめる。`internal/server` は記事の投稿のために `internal/publisher` を import する（#109 §2.1 の図 2）ので、この配置では「`internal/server` が実装を import しない」ことを import の検査では確かめられない。`internal/server` が実装の型を参照しないことは #109 の側で保証する。
-   **関連:** F-003・F-005、AC-12・AC-17

## H-04: フィールドとメソッドの集合を固定する guard

-   **事項:** AC-01 は、`Notice` の exported フィールドが `RequestID`・`VideoID`・`Kind` の 3 つだけで、`Notifier` のメソッドが `Notify` 1 つだけであることを求める。これを固定する手段（`internal/pipeline` の `TestCommonTypesFieldSets` と同じ種類の guard をどこに置くか）を決める。
-   **理由:** `Notice` に欄を足して秘密情報や字幕の本文を載せる変更を、テストで気づけるようにするため（「Enforce invariants with the type」の補助）。
-   **候補:** `internal/failure` に guard のテストを置く（`internal/pipeline` の guard は `internal/pipeline` の型だけを対象にしている）。`internal/pipeline` に `internal/failure` の型を加える場合は、guard の対象と根拠を示す。
-   **関連:** F-001・F-002、AC-01

## H-05: 送信の時間の上限と `ctx` の関係

-   **事項:** 送信の時間の上限の値（30 秒以下。既存の `SlackPostTimeout` と同じか、より短くするか）と、呼び出し元の `ctx` との関係を決める。F-003 は「自分の上限で打ち切る（`ctx` の期限に頼らない）」ことを求める。#109 のサーバは `Notify` に 60 秒の期限を付ける（#109 §3.9）ので、`Notifier` の上限はそれより短くする。
-   **候補:** 既存の `publisher.SlackPostTimeout`（30 秒）と同じ値を使い、`context.WithTimeoutCause` で `Notifier` 自身の上限を与える（`http.Client.Timeout` は使わない）。`ctx` がキャンセルされたら直ちに返す。
-   **関連:** F-003・F-004、AC-10・AC-14

## H-06: テスト用の loopback の構築と、上限を短くする方法

-   **事項:** F-003 は、ループバックの送信先へ `http` で送る構築をテスト用のビルドでだけ使えるようにすることを求める。テストで送信の時間の上限を短くする方法（AC-10）も決める。
-   **候補:** 既存の `newSlackWebhookPublisher`（非公開。テストから呼べる）と `deepseek.NewForLoopbackTest` の方式を踏襲する。上限は、非公開の構築関数の引数でテストから短くする。
-   **関連:** F-003、AC-10・AC-13
