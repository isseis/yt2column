# 設計への申し送り：Claude の effort の既定と出力の上限

## 本書の位置づけ

[01_requirements.md](01_requirements.md) の作成とレビューの過程で挙がった事項のうち、要件（観測できる振る舞い）ではなく、設計・実装の段階で決める、または注意すべき事項をまとめる。

-   `02_architecture.md` は、本書の各項目について、採用した手段または扱わない理由を記録する。本書の手段は候補であり、設計で別の手段を選んでもよい。ただし、対応する AC を満たすこと。
-   本書は承認対象の文書ではない。設計の作成時に参照し、項目を追加してよい。

## 項目一覧

| ID | 対象 | 概要 | 関連 AC |
|---|---|---|---|
| H-01 | F-002 | 既存の測定からの V の決め方 | AC-07・AC-13 |
| H-02 | F-002 | モデルが受け付ける出力の上限 | AC-13・AC-15 |
| H-03 | F-002・4.1 | 非ストリーミングの目安と長い無通信 | AC-12・AC-13 |
| H-04 | 4.2 | 応答の本文の読み取りの上限 | AC-07 |
| H-05 | 4.3 | 打ち切られた実行の推定コスト | AC-13 |
| H-06 | F-001 | effort の既定値の置き場所 | AC-01〜AC-06 |
| H-07 | F-001・F-002 | 既存のテストとコメントの更新 | AC-01・AC-07・AC-12 |
| H-08 | F-003 | 開発者ガイドの構成 | AC-17・AC-18 |

## H-01: 既存の測定からの V の決め方

-   使う測定: 0009 `02_architecture.md` §1.4（Opus 5.5・Sonnet 5.5 の effort ごとの出力と所要時間。テンプレートは当時の仮の文面）、0008 評価記録 §5.7（Sonnet 5.5・high・v5 の打ち切り 2 件）、0018 評価記録 §4.7.2（B3。本番の構成の 10 件）・§4.7.4（B4。v7 の 10 件と打ち切り 1 件）。
-   0018 の CLI の所要時間は、生成の呼び出しだけでなく、プロンプトの組み立てや入力の処理も含む。出力の速さを所要時間から計算すると、実際の出力の速さより遅めに見積もることになる。F-002 (b) の見積もりでは、遅めの側に倒れるので許容できる。
-   0018 では 10 件を並列に起動している（0018 評価記録 §3.3）。並列による速さの低下が測定に含まれうることを記録する。
-   (a) の余裕は、観測した最大値（15,884）に対する倍率か、上限に届いた打ち切りを含めた分布からの距離として記録する。0008 §5.7 の Sonnet 5.5 の打ち切り（本文を書く前に 16,000 に達した）は、Sonnet 5.5・high・v5 では 16,000 を超える推論がありうることを示しており、Sonnet を使う構成の参考になる。
-   テンプレート v5 での 40 分前後の動画の出力は測っていない（要件定義書「要件定義の際に決めたこと」）。この欠けを、余裕の大きさの判断でどう扱ったかを記録する。

## H-02: モデルが受け付ける出力の上限

-   公開文書（Anthropic のモデルの一覧）で、claude-opus-5-5・claude-sonnet-5-5 の出力の上限を確かめ、出典と確認日を記録する。0009 §1.4 では、どちらも `max_tokens` 64,000 を受け付けた。
-   claude-haiku-5-5 など、要件の対象外のモデルの出力の上限も確かめる。V がそのモデルの上限を超える場合、リクエストは `400` で拒否されうる（0009 §1.4 の表の #5 と同じ `invalid_request_error`）。その場合は、README で案内する（例: そのモデルでは使えない、または統合テストのように正の `MaxOutputTokens` を渡す経路だけで使う）。統合テストは正の `MaxOutputTokens` を渡すので、影響を受けない（0009 `02_architecture.md` §3.6）。
-   README の `max` の注意（AC-15）の文を、V と 0009 §1.4 の観測に合わせて直す。

## H-03: 非ストリーミングの目安と長い無通信

-   0009 `02_architecture.md` §3.6 は、「Anthropic は非ストリーミングの `max_tokens` の目安を約 16,000 としている」ことを根拠の 1 つにした。この目安の現在の内容（公開文書、または Anthropic の公式 SDK の振る舞い）を確かめ、出典を記録する。
-   本アダプタは公式 SDK を使わず、標準ライブラリで HTTP を送る（0009 要件定義書 §2.3）。SDK が長い非ストリーミングのリクエストを拒否する仕組みがあっても、本アダプタには当てはまらない。API そのものが大きな `max_tokens` の非ストリーミングのリクエストを拒否しないかを確かめる（0009 §1.4 では 64,000 を受け付けた）。
-   無通信が長いと、利用者のネットワーク（NAT・プロキシ）で接続が切れうる。Go の HTTP/2 の既定の設定は PING を送らないため、切れた場合は 15 分のタイムアウトまで待ってから失敗する（0009 `02_architecture.md` §3.6）。このリスクの扱い（受け入れるか、HTTP/2 の PING の設定などで検出を早めるか）を記録する。新たな仕組みを足す場合は、CLAUDE.md「Performance」の「Do not close a review finding by adding a mechanism until the finding's premise is verified」に従う。
-   `provider.LLMTimeout` の doc コメント（`internal/llm/provider/provider.go:16-19`）は、「Claude アダプタの既定の出力の上限の非ストリーミングの呼び出しは、十分に短い時間で終わると測定した」と書いている。V に合わせて、根拠の記述を直す。

## H-04: 応答の本文の読み取りの上限

-   応答の本文は `llmhttp.MaxResponseBytes`（8 MiB。`internal/llm/llmhttp/llmhttp.go:19`）まで読む。0009 §1.4 では、64,000 トークンの打ち切りの応答の本文が約 245 KB だった（大部分は `thinking` ブロックの `signature`）。
-   V で返りうる応答の本文の大きさが、この上限に十分に収まることを、0009 §1.4 の観測から見積もって記録する。

## H-05: 打ち切られた実行の推定コスト

-   上限を上げると、打ち切られた 1 回の実行の料金が上がりうる。V トークンの出力で打ち切られた生成の 1 回の推定コストを、`internal/pricing` の単価の表（claude-opus-5-5）から計算して記録する。比較のため、16,000 で打ち切られた実行の推定コスト（0018 評価記録 §4.7.4 の 0.415428 USD）も挙げる。
-   effort `max` は、0009 §1.4 で 64,000 トークンをすべて推論に使った。`max` を選んだ利用者が V まで推論に使って打ち切られた場合の料金も、同じ計算で示せる。README の `max` の注意（AC-15）に料金を書くかは設計で決める。

## H-06: effort の既定値の置き場所

-   既定値 `high` を持つ場所の候補: `internal/config` の `loadClaudeEffort`（未設定のときに `claudeparam.EffortHigh` を入れる）、`internal/llm/claudeparam` の名前付きの値（例: 既定の effort を表す定数）。effort の値の一覧は `claudeparam` だけにある（`internal/llm/claudeparam` の doc コメント）。既定値の置き場所も、一覧と同じ場所にするかを決める。
-   `claudeparam.EffortUnset` はゼロ値で、アダプタの構築が拒否する（0009 の設計）。既定を入れた後も、`EffortUnset` がアダプタに届かないことを保つ（CLAUDE.md「Declare, don't infer」の、ゼロ値を最も仮定の少ない解釈とし、`default` で安全側に失敗する方針）。
-   アダプタ（`internal/llm/claude`）には既定値を持たせない。環境変数の解釈と既定値は `internal/config` だけが行う（0009 要件定義書 F-006）。
-   `LLMConfig.ClaudeEffort` の doc コメント（`internal/config/config.go:158-162`）と、`loadClaudeEffort` の doc コメント（`internal/config/config.go:586-589`）を、既定の動作に合わせて直す。
-   `--after` の再開では、実行する段階の設定だけを読む（`internal/config/config.go` の `Need`）。既定は、`loadClaudeEffort` が呼ばれる条件の中だけで当てる（AC-05）。

## H-07: 既存のテストとコメントの更新

-   0009 の AC-24 のテスト（プロバイダが `claude` で `YT2COLUMN_CLAUDE_EFFORT` が未設定なら `config.ErrMissing`）は、AC-01・AC-02 と矛盾する。未設定の場合のテストを、`high` になることを確かめるテストに置き換える。空の値の場合のテストは残す（AC-04）。置き換えの前後で、`go tool cover -func` の関数ごとの値を比べる（CLAUDE.md「Deleting a test is a claim that must be checked」）。
-   AC-08 は、3 つの段階のアダプタがどれも V を送ることを求める。`internal/writer` の 3 か所が `MaxOutputTokens: 0` を渡していることと、アダプタが 0 を V に変えることの両方を、テストでどう確かめるかを決める（CLAUDE.md「A layered path needs inputs only one layer can handle」）。
-   V を定数の値と比べるだけのテストは、定数がその値であることしか確かめない（CLAUDE.md「Do not assert that a constant equals its own literal」）。AC-07 は、送るリクエスト本文の `max_tokens` を見るテストで確かめる。
-   AC-12 のタイムアウトとサーバの停止の通知は、既存のテスト（`cmd/yt2column`・`cmd/yt2column-server`）がすでに確かめているかを確認し、確かめていればそのままにする。

## H-08: 開発者ガイドの構成

-   ファイル名の候補: `docs/dev/developer_guide/claude_output_limit.md`。`pricing_update.md` と同じく、`docs/dev/developer_guide/*.md` は英語で書く（CLAUDE.md「Documents」）。
-   測り直しには実 API の呼び出しが要り、料金がかかる。ガイドには、人の承認を得てから実行すること（CLAUDE.md「Tool Execution Safety」）と、評価の手順（0008 評価記録 §2.3、0018 評価記録 §3）への参照を書く。
-   集める値の出どころ: README「Token usage」の `tokens:` の行の `output=`、`--artifacts` の中間生成物に記録したトークン数、CLI の打ち切りのエラー（`generation truncated by the output token limit`）。
-   ガイドの値（V、出力の速さ）を文書に写すと、コードと食い違いうる。ガイドは V の値を写さず、V を定義する場所と、その根拠の記録（`02_architecture.md`）を参照する形にするかを決める。値を写す場合は、値の一致を確かめるテストが要る（CLAUDE.md「Document tests check values, structure, and executable examples, never prose」）。
