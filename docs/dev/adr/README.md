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
