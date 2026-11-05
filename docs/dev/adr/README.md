# ADR の索引

この索引は、主要な設計の決定を記録した ADR（Architecture Decision Record）をすべて挙げ、今どの決定が効いているかを示す。各 ADR は、主題ごとに、今効いている決定、その理由、採らなかった案、タスクをまたぐ経緯を持つ。設計を始める前に、ここから関係する ADR を探して読む。

状態が `有効` の ADR が、今効いている決定である。`廃止` の ADR は、主題が終わった決定の記録として残す。ADR の書き方、番号の付け方、状態の意味、どの決定を主要とするかは、規約 [docs/dev/conventions/adr.md](../conventions/adr.md) にある。ADR を足すか状態を変える PR は、同じ PR でこの表を直す。

| 番号 | 題名 | 状態 |
|---|---|---|
| [ADR-0001](0001-llm-adapter-stdlib.md) | LLM のアダプタは SDK を使わず標準ライブラリで書き、共有の型と送信の経路をプロバイダの外に置く | `有効` |
| [ADR-0002](0002-post-generation-steps.md) | 生成の後に字数の調整と検証・推敲の 2 段階を置き、検証・推敲は直す箇所の一覧だけを返す | `有効` |
| [ADR-0003](0003-llm-config-contract.md) | LLM のプロバイダ・モデル・effort を 1 つの値 <プロバイダ>/<モデル ID>+<effort> で指定する | `有効` |
| [ADR-0004](0004-production-profile.md) | 本番の構成を claude-opus-5-5（effort high）の 3 段階とし、品質の目標と製品の制約を共通の文書に置く | `有効` |
| [ADR-0005](0005-server-architecture.md) | サーバを Native Messaging のホストと常駐サーバに分け、HTTP を Unix ドメインソケットの上で受ける | `有効` |
| [ADR-0006](0006-threat-model.md) | ブラウザ拡張と localhost サーバの脅威モデルは、同一ユーザーを信頼し、他のユーザーを排除する | `有効` |
| [ADR-0007](0007-manual-transcript-copy.md) | 字幕は利用者が文字起こしパネルから手動でコピーし、拡張はページを読むだけにする | `有効` |
| [ADR-0008](0008-launch-path.md) | 拡張の起動の経路をツールバーのポップアップだけにし、生成の開始は利用者のボタンの押下だけで行う | `有効` |
| [ADR-0009](0009-viewer-opening.md) | ビューアは生成の開始と同時に背面のタブで開き、ポップアップは閉じずに状態を表示する | `有効` |
| [ADR-0010](0010-extension-stores-nothing.md) | 拡張は何も保存せず、権限も加えず、状態の正しい値をサーバに置く | `有効` |
| [ADR-0011](0011-webhook-publishing.md) | 投稿先は Slack 互換の Incoming Webhook とし、投稿は利用者の操作でだけ行う | `有効` |
| [ADR-0012](0012-review-setup.md) | PR レビューは作成時の 1 回だけ Codex に頼み、その後の変更はローカルの CLI レビューで見る | `有効` |
