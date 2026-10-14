# 実装への申し送り：失敗を Slack Webhook へ投稿する仕組み

本書は、[02_architecture.md](02_architecture.md) の作成とレビューの過程で挙がった事項のうち、設計（構成・責務・インターフェース・データフロー）ではなく、実装の段階で決める事項をまとめる。

-   [03_implementation_plan.md](03_implementation_plan.md) は、本書の各項目について、採用した手段または扱わない理由を記録する。本書の手段は候補であり、実装で別の手段を選んでもよい。ただし、対応する AC を満たすこと。
-   本書は承認対象の文書ではない。実装計画の作成時に参照し、項目を追加してよい。

## 項目一覧

| ID | 対象 | 概要 | 関連 AC |
|---|---|---|---|
| I-01 | F-003・F-004 | `Notify` の契約を満たすための具体的な処理手順（検査・送信の順序、エラーの判定と包み方、`ctx` の確認の置き場所）と、テスト用の構築関数の拒否の検査方法 | AC-04a・AC-08・AC-09・AC-10・AC-11・AC-12・AC-14 |

## I-01: `Notify` の処理手順

-   **事項:** `02_architecture.md` §3.4 は、`Notify` の外部から観測できる契約（無効な通知を送らない、送信は多くとも 1 回、結果を文書化した番兵に写す、届いたかどうかを示す、状態を持たず記録しない）を定める。この契約を満たすための具体的な処理手順（どの順でゼロ値の確認・値の形の検査・`ctx` の確認・送信を行うか、`errors.Is` でどの番兵に分岐するか、元のエラーを `%w` でどう残すか）は、実装の段階で決める。あわせて、無効な `ctx` を送らないための確認を `Notify` に置くか共有の送信の経路（`webhookClient.post`）に置くかも、実装の段階で決める。ただし §3.1 の契約により、共有の `post` は `*SlackPostError.Attempted`（`slack_errors.go:43-52`）の意味を変えない。テスト用の構築関数 `NewSlackFailureNotifierForLoopbackTest` の拒否（loopback でない URL・非正の `timeout`・空の URL。AC-12）をどう検査するか、たとえば `fatalRecorder` を使う `TestNewSlackWebhookPublisherForLoopbackTestRejects` と同じ形の記録用の補助を使うかも、実装の段階で決める。
-   **理由:** レビューで、設計文書は「Focus on high-level design」「Avoid implementation details」に従い、処理の順序やエラーの判定・包み方の手段まで指定しないことが求められた。これらは `02_architecture.md` のレベルを下回るため、実装計画で扱う。
-   **候補:** `03_implementation_plan.md` の該当タスクに、`02_architecture.md` §3.4 の契約を満たす実装手順として書く。少なくとも、(1) 受け手がゼロ値のとき・`Notice` の形が合わないとき・`ctx` が既に無効なときは送信の前に弾くこと、(2) 送信は `webhookClient.post` を 1 回だけ呼ぶこと、(3) `ErrSlackHTTPStatus`・`ErrSlackInvalidResponse` と判定できるエラーを `ErrSlackNoticeRejected` と元のエラーの両方で判定できる形に包むこと、(4) 送信の途中の時間切れ・中断・通信の失敗を「届いたかもしれない」エラーとして包むこと、(5) `ctx` の確認を `Notify` と共有の送信の経路のどちらに置くかを、§3.1 の契約（`*SlackPostError.Attempted` の意味を変えない）の下で決めること、(6) テスト用の構築関数の拒否（AC-12）の検査の方法を決めること、を含める。各 AC の証跡（テスト）への対応づけもここで行う。
-   **関連:** F-003・F-004、AC-04a・AC-08・AC-09・AC-10・AC-11・AC-12・AC-14
