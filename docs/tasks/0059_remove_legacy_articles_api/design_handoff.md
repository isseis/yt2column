# 設計への申し送り（0059 旧 API の削除）

要件の作成中に挙がった、設計で決める事項。`02_architecture.md` は、各項目の扱い（採った方法、または当てはまらない理由）を記録する。

-   **H-01 シャットダウンのログの数え方（AC-13）:** 今は `pending` に生成の枠の使用数（`len(s.slots)`）を、`postJobs` に投稿のジョブの数を書く（`internal/server/server.go:262-273`）。旧 API が消えた後、生成の枠を持つのは取り消した生成のジョブだけになる。`pending` をそのまま残すか、名前と意味を改めるか、ログの文言（「failure notices」）をどう直すかを決める。
-   **H-02 ホストのログの属性（AC-10・AC-11）:** ホストのログは属性 `form`（`legacy`・`new`）と段階 `form` を持つ（`internal/nmhost/relay.go:425-433`、`0044` 設計書 §3.10.5）。形が 1 つになった後、属性 `form` を残すか、`v` がない場合を新しい規則の名前（例えば既存の `version` か `members`）でログに書くかを決める。
-   **H-03 `webhook_not_configured` の `Connection: close`（issue の簡素化 2）:** 今は `forceClose` が `webhook_not_configured` を含む（`internal/server/response.go:106-118`）。`/articles` は本文を読まずに拒否するので接続を閉じる必要があったが、投稿の操作は本文を持たない。投稿の操作の拒否に `Connection: close` を付け続けるかを決める。拒否の理由と状態コードは変えない（AC-03）。
-   **H-04 ホストの説明の更新（issue の簡素化 5）:** `forward` の契約、`Relay` の契約、パッケージの説明、`maxFrameBytes` などの注釈、`.golangci.yml` の `nmhost-boundaries` の説明から、今の形の流し込みとの対比を除く（`0044` 設計書 §3.10.2「説明の更新」）。全体を読んでから送る今の順序（`0044` 設計書 §3.10.1）は、新しい形の検査に必要なので変えない。
-   **H-05 旧 API に依存するテストの扱い:** `/articles` や旧 API のジョブを使うテストがある（`cmd/yt2column-server/relay_e2e_test.go`、`cmd/yt2column-server/run_test.go`、`cmd/yt2column-server/signal_test.go` の `TestSignalWaitsForJob`、`internal/server` の `TestServeShutdownWaitsForJobs` など、`internal/nmhost/relay_test.go` の今の形の中継のテスト）。旧 API だけを確かめるテストは削除し、停止の待ち方やソケットの中継のように残る振る舞いを旧 API で確かめていたテストは、ジョブの API（投稿のジョブなど）で確かめ直す。削除の前後で `go tool cover -func` を比べる（CLAUDE.md「Testing Strategy」）。
-   **H-06 `jobInput` と `slots` の整理（issue の簡素化 1・4）:** `jobInput` の `source`・`requestID` のうち旧 API だけが使うもの、`slots` の説明と、生成の枠を持つ主体（リクエスト・ジョブ・失敗の通知）の区別を除く。`maxInFlight` の値（`2`）は変えない（AC-03）。
-   **H-07 停止の案内の目安の計算（AC-12）:** 「約 5 分」を、投稿のジョブの最長の時間（Slack への最大の通数・1 通の時間の上限・送信の間隔）から計算するか、固定の文言にするかを決める。今の「約 45 分」は `llmCallsPerJob` と `provider.LLMTimeout` から計算している（`cmd/yt2column-server/run.go:237-243`）。
-   **H-08 開発用のサーバが動いているかの確認の例（AC-17）:** `docs/dev/guides/development.md:37` は `GET /articles`（`method_not_allowed` が返る）でサーバが応えることを確かめている。ジョブの API のどのリクエスト（例えば知らないキーの状態の取得）に置き換えるかを決める。
-   **H-09 PR の分け方:** サーバ、ホスト、文書の変更をどの PR に分けるかを決める。README の変更は `/mktrans` で `README.en.md` と同じコミットに入れる。CLAUDE.md の例外の記述は、ホストの今の形の削除と同じ PR で取り除く。
