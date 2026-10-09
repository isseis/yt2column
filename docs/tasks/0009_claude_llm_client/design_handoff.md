# 設計への申し送り：Claude アダプタ

## 本書の位置づけ

[01_requirements.md](01_requirements.md) の作成とレビューの過程で挙がった事項のうち、要件（観測できる振る舞い）ではなく、設計・実装の段階で決める、または注意すべき事項をまとめる。

-   `02_architecture.md` は、本書の各項目について、採用した手段または扱わない理由を記録する（要件書 §5）。本書の手段は候補であり、設計で別の手段を選んでもよい。ただし、対応する AC を満たすこと。
-   DeepSeek アダプタの申し送り（[0003 の design_handoff.md](../0003_deepseek_llm_client/design_handoff.md)）の H-01〜H-07（タイムアウトの報告、リダイレクト、標準ライブラリのエラーに含まれる値、応答本文のサイズの上限、`encoding/json` が黙って補正する入力、送信先の差し替え、`200` 以外の応答の本文）は、本タスクにもそのまま当てはまる。DeepSeek アダプタで採った手段を再利用できるかを最初に確認する（CLAUDE.md「DRY」）。
-   本書は承認対象の文書ではない。設計の作成時に参照し、項目を追加してよい。

## 項目一覧

| ID | 対象 | 概要 | 関連 AC |
|---|---|---|---|
| H-01 | 全体 | DeepSeek アダプタとの共通部分の切り出し | AC-17〜AC-22・AC-32〜AC-37 |
| H-02 | F-001・F-006 | effort の列挙型の置き場所 | AC-02・AC-06・AC-23・AC-25 |
| H-03 | F-002 | `max_tokens` の定数の決め方 | AC-05 |
| H-04 | F-004 | 非ストリーミングの長い生成とタイムアウト | AC-18 |
| H-05 | 3.2 | `content` の要素の、`type` による検証の分岐 | AC-33〜AC-36 |
| H-06 | F-002 | `anthropic-version` の固定値 | AC-03 |
| H-07 | F-007 | 統合テストの既定のモデルと effort | AC-29・AC-30 |
| H-08 | F-008 | ユニットテストの送信先の差し替え（`httptest`） | AC-31 |
| H-09 | 3.2 | 応答本文のサイズの上限値の決定 | AC-37 |

## H-01: DeepSeek アダプタとの共通部分の切り出し

-   HTTP クライアントの構成（リダイレクトに従わない、タイムアウトを `context.DeadlineExceeded` として報告する、応答本文の読み取りの上限、`*url.Error` からの秘密情報の除去）は、DeepSeek アダプタ（`internal/llm/deepseek`）に実装済みである。
-   応答の JSON の厳格な検証は `internal/strictjson` が提供する。Claude アダプタも同じパッケージを使う。
-   候補: 共通の HTTP 処理を `internal/llm` の下の共有パッケージへ切り出し、`ErrHTTPStatus`・`ErrInvalidResponse`・`ErrTransport` もそこへ移す。切り出す場合は、DeepSeek アダプタの既存のテストが変更なしに通ることを確かめる（AC-27）。切り出しをリファクタリングとして、Claude アダプタの追加とは別のコミットにする。
-   切り出しの範囲が小さく済まない場合は、Claude アダプタ側に同じ処理を持ち、共通化を別の issue にしてもよい。その場合は理由を記録する。

## H-02: effort の列挙型の置き場所

-   要件書 §4.5 は、effort の 5 つの値をアダプタと `internal/config` で別々に列挙しないことを求める。
-   `internal/config` は `internal/llm/claude` を import していない。`internal/llm/provider` が両者をつなぐ。
-   候補: (a) 列挙型を `internal/llm/claude` に置き、文字列からの変換関数を公開して `internal/config` から使う。(b) 列挙型を `internal/config` に置き、`provider` がアダプタの型へ変換する（変換の `switch` の `default` は失敗とする）。(c) 依存のない小さなパッケージに置く。import の向き（`internal/config` がアダプタパッケージに依存してよいか）を確認して選ぶ。
-   ゼロ値を「未指定」とし、構築がこれを拒否すること（AC-02）。

## H-03: `max_tokens` の定数の決め方

-   Messages API の `max_tokens` は、推論過程（thinking）のトークンを含む出力の上限である。effort を上げると推論過程が長くなり、同じ記事でも上限に達しやすくなる。
-   事前調査（要件書 §5）で、40 分前後の動画の字幕から記事を生成したときの出力トークン数を、`high` 以上の effort で測る。測定値に余裕を持たせた値を定数とし、根拠を `02_architecture.md` に記録する。
-   値が大きいほど非ストリーミングの応答に時間がかかりうる（H-04）。

## H-04: 非ストリーミングの長い生成とタイムアウト

-   既存の LLM のタイムアウト（`provider.LLMTimeout`、15 分）は、DeepSeek がリクエストを最大 10 分保留することを根拠にしている。Claude には同じ根拠がない。
-   Anthropic の SDK は、長い生成が見込まれる非ストリーミングのリクエストにストリーミングを勧めている。生成中に接続が長く無通信になると、経路上で切断されうる。
-   事前調査で、`xhigh`・`max` の effort と H-03 の定数で生成したときの所要時間を測り、タイムアウト内に収まるか、無通信による切断が起きないかを確かめる。問題があれば、要件書 §2.3 のとおりストリーミングを別の issue にする。
-   タイムアウトの値を DeepSeek と共有するか分けるかを決め、根拠を記録する（要件書 F-006）。

## H-05: `content` の要素の、`type` による検証の分岐

-   `content` の要素は、`type` の値によって消費するメンバーが変わる（`text` なら `text` を消費し、`thinking`・`redacted_thinking` なら `type` 以外を消費しない）。
-   検証は 2 段階になる。まず `type` を厳格に読み、受理する 3 つの値以外を拒否する。次に `type` が `text` の要素だけ、`text` を厳格に読む。`type` の判定は `switch` で行い、`default` は拒否とする（CLAUDE.md「Declare, don't infer」）。
-   `thinking` ブロックの `thinking` は消費しないが、本文全体の UTF-8・サロゲートの検査は及ぶ（AC-36）。`internal/strictjson` の検査が本文全体に適用されることを確かめる。

## H-06: `anthropic-version` の固定値

-   Messages API は `anthropic-version` ヘッダーを必須とする。現行の値は `2023-06-01` である。設計時に Anthropic の公開文書で確認し、定数として固定する。
-   値を利用者の設定から変えられるようにしない。

## H-07: 統合テストの既定のモデルと effort

-   `make test-integration-claude` の既定値は、料金を抑える組み合わせにする。候補は、現行の Haiku（`claude-haiku-5-5`）と `low` である。
-   打ち切りの検出（AC-30）は、小さな `MaxOutputTokens` を指定する。thinking が有効なモデルでは、推論過程の途中で打ち切られ、`text` ブロックがない応答になりうる。この場合も `stop_reason` が `max_tokens` なら `llm.ErrTruncated` になることを、検証の順序（要件書 F-003・AC-16）で保証している。
-   Messages API が受け付ける `max_tokens` の最小値を確認し、それ以上の値を使う。

## H-08: ユニットテストの送信先の差し替え

-   アダプタのユニットテストは、`net/http/httptest` のサーバーを送信先にして、F-001〜F-005 と 3.2 の各 AC を検証する（AC-31 のテストの振る舞いを満たす手段）。テストは Claude の API もネットワーク上の外部ホストも呼ばない。
-   送信先をテストから差し替える手段（テスト用の構築、`http.Client` または `http.RoundTripper` の差し替えなど）と、本番の送信先（`https://api.anthropic.com/v1/messages`）を利用者の設定から変えられないようにする方法は、DeepSeek アダプタの申し送り（[0003 の design_handoff.md](../0003_deepseek_llm_client/design_handoff.md) H-06）で採った手段を再利用できるかを最初に確認して決める（要件書 F-001）。
-   設定（`internal/config`）のユニットテストも Claude の API と外部ホストを呼ばない（AC-31）。

## H-09: 応答本文のサイズの上限値の決定

-   要件書 3.2 は、応答本文のサイズに上限を設け、上限を超える応答を拒否し、ちょうど上限の応答を受理すると定める。上限の具体値は要件書に置かず、設計で固定する。
-   レビューで、この上限値を要件書（`01_requirements.md`）に数値として記載し、AC-37 にも同じ値を書くよう求める指摘があった。上限値は設計の定数であり、要件の水準では「有限の上限を超える応答は拒否される」という観測できる境界だけを定めればよいため、要件書には数値を置かず、本文の表現も `02_architecture.md` の名指しから設計の水準に改めた。
-   候補: 上限値は、想定する生成の長さ（40 分前後の動画のコラム記事）と、応答全体（`thinking` ブロックを含む）の大きさから決める。DeepSeek アダプタの [design_handoff.md](../0003_deepseek_llm_client/design_handoff.md) H-04（応答本文のサイズの上限と超過の検出）で採った手段（上限 + 1 バイトまで読み、超えて読めたら拒否する）と値を再利用できるかを最初に確認する。
-   AC-37 のテストは、設計で固定した上限の定数を参照し、上限ちょうどは受理・上限 + 1 バイトは `ErrInvalidResponse` となることを検証する。上限値を変えたときはテストの期待値も追随させる。
-   関連: AC-37。`02_architecture.md` の上限値の記録、0003 の design_handoff.md H-04。
