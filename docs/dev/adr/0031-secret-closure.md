# ADR-0031: 秘密を secret.Secret 型のクロージャで持ち、送信先へのリクエストを除くどの出力にも流さない

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

LLM の API キーは、ログやエラーメッセージに漏れてはならない（[security.md](../security.md) §2）。秘密は設定の読み込みから LLM のアダプタまで渡るので、漏らさないことを各実装者の注意に頼らず、型で保証したい（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) §1）。

ただし、型にメソッドを実装するだけでは足りない。`fmt` と `log/slog` の TextHandler は、構造体の非公開フィールドに対してメソッドを呼ばず、リフレクションで中身を表示する。秘密を非公開フィールドに持つ構造体を出力すると、メソッドを経ずに値が現れる（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) §5）。

## 決定

-   **秘密は `secret.Secret` で持つ。** API キーは、設定を読むところで `secret.Secret` に包み、秘密を使う部品は `secret.Secret` のまま受け取って持つ。漏らさないことを各実装者の注意に頼らず、型で保証するためである（背景）。`internal/secret` の役割は [package_reference.md](../conventions/package_reference.md) の `internal/secret` の行に従う。
-   **API キーは、送信先へのリクエストを除き、エラー・ログ・標準出力を含むどの出力にも出さない。** 秘密がログやエラーメッセージに漏れてはならないためである（背景）。秘密の扱いは [security.md](../security.md) の §2 に従う。
-   **拡張では、API キーは決めた経路だけを通り、記事生成モジュールが受け取った直後に `secret.Secret` に包む。** API キーは、オプションページから `chrome.storage.local` へ、そこから記事の生成を実行するビューアのメインスレッドへ、`postMessage` で Worker へ、Worker の中の記事生成モジュールへと渡り、Anthropic の API のリクエストのヘッダに入る。設定の検査のときは、オプションページから Worker の検査の関数へも渡る。SW へのメッセージ、ジョブの記録、`BroadcastChannel`、記事生成モジュールの結果と進み具合、ログ、画面には入れない。SW は API キーの値を読まず、`chrome.storage.local.getKeys()` で項目の名前だけを得る（[ADR-0010](0010-extension-stores-nothing.md)）。TypeScript の側は型で守れないので経路を限ってテストで確かめ、Go の側に入った後は型で守るためである。規則は [0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md) の「API キーを出さない先（要件書 AC-06）」にある。

拡張が API キーを保存して記事生成モジュールに渡すことは、まだコードと security.md に反映していない。今の拡張は API キーを持たず、サーバが設定ファイルから読む。この差分は [#773](https://github.com/isseis/yt2column/issues/773) の実装で解消する。

-   **元の値は、型の唯一のフィールドであるクロージャに閉じ込める。** リフレクションが見られるのは関数のアドレスだけなので、`secret.Secret` を非公開フィールドに持つ構造体を出力しても、元の値は現れない。`fmt` と `log/slog` は非公開フィールドに対してメソッドを呼ばず、リフレクションで中身を表示するためである。規則は [0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) §5「制約条件 (Constraints)」と、[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §3.3「秘密情報型 `Secret`」にある。正本の文書はこの設計を述べないので、この ADR がコードの振る舞いとして述べる。
-   **`fmt`・`log/slog`・`encoding/json` が呼ぶメソッドは、元の値の代わりに固定の伏せ字を出す。** 型を経た出力に元の値が現れないようにするためである。規則は 0001 設計書 §3.3「秘密情報型 `Secret`」にある。
-   **元の値を取り出す経路は `Reveal()` だけにする。** 空の値からは作れず、ゼロ値の `Reveal()` はエラーを返す。`Reveal()` を呼ぶのは、送信先へのリクエストの組み立てなど、値そのものが要る箇所に限る。値そのものが要る箇所を限り、秘密が漏れる経路を狭めるためである。規則は 0001 要件定義書 F-005・§5「制約条件 (Constraints)」と、0001 設計書 §3.3「秘密情報型 `Secret`」にある。
-   **公開のメソッドの集合は、テストで固定する。** 秘密を取り出す経路が増えたことを、テストで検出するためである。規則は `internal/secret` と、[`internal/pipeline/pipeline_test.go`](../../../internal/pipeline/pipeline_test.go) の `TestSecretRevealExclusive` にある。

## 採らなかった案

-   **元の値を素の文字列のフィールドで持ち、`Format`・`String()` などのメソッドだけで隠す。** `fmt` と slog の TextHandler は、構造体の非公開フィールドではメソッドを呼ばずにリフレクションで表示するので、メソッドの実装だけでは隠せない（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) §5、[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §3.3）。
-   **値を出力しない等値の判定のメソッド（例：`Matches(plaintext string) bool`）を足す。** 等値の判定の要件がないので、YAGNI により足さない。設定の検証やテストで要るようになったときに足す（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §3.3・§9）。

## 結果

-   `secret.Secret` は関数のフィールドを持つので、比較できない。`==` はコンパイルエラーになり、`reflect.DeepEqual` は同じ値を持つ 2 つの `secret.Secret` を等しいとしない。`map` のキーにもできない（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §3.3）。
-   非公開フィールドを経た出力には、`[REDACTED]` ではなく、プロセスごとに変わる関数のアドレス（`0x...`）が出る。`secret.Secret` を非公開フィールドに持つ構造体に、フィールドの値をそのまま出す `String()` や `Format` を足してはならない。足すと、そのメソッドが元の値を出す経路になりうる（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §5.1）。
-   型が守るのは、`fmt`・`log/slog`・`encoding/json` の経路だけである。`Reveal()` で取り出した値は、呼び出し元が秘密として扱う。送信の失敗のエラーのように、型を経ない素の文字列に秘密が入る経路は型では防げないので、秘密を扱うかネットワークへ送る部品が、それぞれエラーに秘密を含めないことを確かめる（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §5.1・§5.2）。CLI は、これとは別に、出力する文字列の中の秘密の値を `internal/redact` で伏せ字にする。この伏せ字の仕組みは、この ADR の主題ではない。

## 経緯

- 2026-09-29 秘密を持つ型 `Secret` を `internal/secret` に置いた。元の値を `func() string` のクロージャで持ち、`fmt`・`log/slog`・`encoding/json` のどの経路でも元の値を出さず、元の値は `Reveal()` でだけ取り出せる。空の値とゼロ値は拒否する（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) F-005・§5）
- 2026-11-11 Webhook への投稿の機能の廃止に合わせて、秘密から Webhook URL を除いた（[0083 要件定義書](../../tasks/0083_remove_slack_webhook/01_requirements.md) F-001）
- 2026-11-12 拡張が API キーを保存して使う構成で、API キーが通る経路を決め、記事生成モジュールが受け取った直後に `secret.Secret` に包むことにした（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)）
