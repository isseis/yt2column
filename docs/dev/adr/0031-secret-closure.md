# ADR-0031: 秘密を secret.Secret 型のクロージャで持ち、送信先へのリクエストを除くどの出力にも流さない

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

LLM の API キーと Slack の Webhook URL は、ログやエラーメッセージに漏れてはならない。Webhook URL は、URL そのものが秘密である（[security.md](../security.md) §2）。この保証を実装者の注意に頼らず、型で持たせたい。秘密を受け取る LLM のアダプタと Webhook への送信より先に、秘密を持つ型を用意する必要があった（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) §1）。

型にメソッドを実装するだけでは足りない。`fmt` と `log/slog` の TextHandler は、構造体の非公開フィールドに対して `Format`・`String()`・`GoString()` を呼ばず、リフレクションで中身を表示する。秘密を非公開フィールドに持つ構造体を出力すると、メソッドを経ずに値が現れる（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) §5）。

## 決定

-   **秘密は `secret.Secret` で持つ。** 設定を読む `internal/config` は、API キーと Webhook URL を `secret.New` で `secret.Secret` に包む。LLM のアダプタと Webhook への送信は、秘密を `secret.Secret` のまま受け取って持つ。`internal/secret` の役割は [package_reference.md](../conventions/package_reference.md) の `internal/secret` の行に従う。
-   **API キーと Webhook URL は、送信先へのリクエストを除き、エラー・ログ・標準出力を含むどの出力にも出さない。** 値が出てよいのは、LLM API へのリクエストのヘッダーと、Webhook への HTTP リクエストの URL だけである。Webhook URL をログとエラーメッセージに含めないことは [security.md](../security.md) の §2「秘密情報（API キー・Webhook URL）」に、サーバが API キーと Webhook URL を出力に流さないことは同じ文書の §8.2「localhost サーバ」に従う。Webhook への送信の失敗（`*url.Error`）を URL を含まないエラーに直すことは、Webhook への送信の実装として [ADR-0011](0011-webhook-publishing.md) が定める。
-   **元の値は `func() string` のクロージャで持つ。** 型の唯一のフィールドは、値を閉じ込めたクロージャである。リフレクションが見られるのは関数のアドレスだけなので、`secret.Secret` を非公開フィールドに持つ構造体を `fmt` や `log/slog` で出力しても、元の値は現れない。`fmt` のどの書式指定子でも、`String()`・`GoString()`・`LogValue()`・`MarshalJSON()` でも、元の値の代わりに固定の文字列 `[REDACTED]` を出す（`%q` だけは引用符で囲む）。
-   **元の値を取り出す経路は `Reveal()` だけにする。** 空の値からは作れず、`New("")` はエラーを返す。ゼロ値は値がないことを表し、`Reveal()` は空文字列を返さずにエラーを返す。公開のメソッドの集合は、テストで固定する。テスト以外のコードで `Reveal()` を呼ぶのは、送信先へのリクエストを組み立てるとき、値があるかと値の形を確かめるとき、出力の伏せ字にする文字列の一覧を作るとき（`internal/redact`）だけである。

秘密を出力に流さないことの仕様の正本は、[security.md](../security.md) の §2「秘密情報（API キー・Webhook URL）」と §8.2「localhost サーバ」、[package_reference.md](../conventions/package_reference.md) の `internal/secret` の行である。クロージャで持つ設計を述べる文書はほかにないので、この ADR がコードの振る舞いとして述べる。

## 採らなかった案

-   **元の値を素の文字列のフィールドで持ち、`Format`・`String()` などのメソッドだけで隠す。** `fmt` と slog の TextHandler は、構造体の非公開フィールドではメソッドを呼ばずにリフレクションで表示するので、メソッドの実装だけでは隠せない（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) §5、[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §3.3）。
-   **値を出力しない等値の判定のメソッド（例：`Matches(plaintext string) bool`）を足す。** 等値の判定の要件がないので、YAGNI により足さない。設定の検証やテストで要るようになったときに足す（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §3.3・§9）。

## 結果

-   `secret.Secret` は関数のフィールドを持つので、比較できない。`==` はコンパイルエラーになり、`reflect.DeepEqual` は同じ値を持つ 2 つの `secret.Secret` を等しいとしない。`map` のキーにもできない（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §3.3）。
-   非公開フィールドを経た出力には、`[REDACTED]` ではなく、プロセスごとに変わる関数のアドレス（`0x...`）が出る。`secret.Secret` を非公開フィールドに持つ構造体に、フィールドの値をそのまま出す `String()` や `Format` を足してはならない。足すと、そのメソッドが元の値を出す経路になりうる（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §5.1）。
-   型が守るのは、`fmt`・`log/slog`・`encoding/json` の経路だけである。`Reveal()` で取り出した値は、呼び出し元が秘密として扱う。送信の失敗のエラーのように、型を経ない素の文字列に秘密が入る経路は型では防げないので、秘密を扱うかネットワークへ送る部品が、それぞれエラーに秘密を含めないことを確かめる（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §5.1・§5.2）。

## 経緯

- 2026-09-29 秘密を持つ型 `Secret` を `internal/secret` に置いた。元の値を `func() string` のクロージャで持ち、`fmt`・`log/slog`・`encoding/json` のどの経路でも `[REDACTED]` を出し、元の値は `Reveal()` でだけ取り出せる。空の値とゼロ値は拒否する（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) F-005・§5）
