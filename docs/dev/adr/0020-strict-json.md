# ADR-0020: 厳格な JSON の部品を internal/strictjson に集め、検証済みの値だけを扱う不透明な型にし、負でない整数の規則を 1 つにする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

LLM の API の応答、字幕の json3 と info.json、中間生成物、サーバとホストのメッセージなど、外部から受け取る JSON を読む箇所が複数ある。標準ライブラリの `encoding/json` は、不正な UTF-8、対になっていないサロゲートのエスケープ、後続のデータ、重複したメンバー、`null` を黙って補正することがある。補正された値をそのまま使うと、検証した値と実際に受理される値がずれる。

最初はこの部品が字幕のパーサの中にだけあった。二つ目の利用者として LLM のアダプタが加わり、部品を `internal/transcript` に残すと LLM のアダプタ（構成要素）が字幕取得の構成要素に依存し、複製するとサロゲートの走査などの細かいコードが 2 か所になる。検証済みの値だけを扱えることを型で保証すれば、公開範囲を広げても不変条件を保てる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §3.5）。

利用者が増えるにつれて、宣言したキー以外を拒否する読み取り、真偽値、`null` の判定、元のバイト列、負でない整数の読み取りも必要になった。負でない整数の検査を各利用者が書くと、`-0` や小数の扱いが箇所ごとにずれる。

## 決定

-   **検証済みの JSON から値を取り出す部品は `internal/strictjson` にだけ置き、複数のパッケージで共有する。** `ParseObject` は、不正な UTF-8、対になっていないサロゲートのエスケープ、後続のデータ、オブジェクトでない入力を拒否する。
-   **`Object` と `Value` は不透明な型にし、`ParseObject` から得た値だけを扱えるようにする。** フィールドは非公開で、`AsString`・`AsObject`・`AsArray` などは、検証を通っていないバイト列を受け取れない。ゼロ値の `Value` は中身を持たず、`AsString` などの読み取りはエラーを返す。ゼロ値の `Object` はメンバーを持たない。
-   **読み取りの手段を、利用者の必要に合わせて 1 か所に持つ。** `Object.Collect` は消費しないキーを無視し、`Object.CollectOnly` は宣言していないキーと重複したキーを拒否する。`Object.Has` はエスケープを解いた名前の完全一致で有無を返し、振る舞いの選択に使える。`Value.AsBool` は `true` と `false` だけを受理する。`Value.IsNull` は `null` とそれ以外を区別する。`Value.Raw` は値の元のバイト列を返す。
-   **負でない整数の規則は `Value.AsNonNegativeInt64` の 1 か所にする。** 符号・小数部・指数部のない 10 進の整数で `int64` の範囲にあり、負でないものだけを受理する。`-0` も拒否し、拒否のエラーには入力の値を含めない。字幕の `tStartMs`、トークン数、中間生成物の数値、ホストのメッセージの版がこれを使う。`AsInt64` は符号と `-0` を受理するので、負でない整数が要る箇所では使わない。
-   **この部品を使うパッケージの一覧は [package_reference.md](../conventions/package_reference.md) が持つ。** 現在のコードでは、一覧に挙げられたパッケージに加え、`internal/server`・`internal/nmhost`・`internal/codexreview`・`internal/coveragecompare` もこの部品を使う。一覧にこの 4 つがないことは、記録した issue [#713](https://github.com/isseis/yt2column/issues/713) で扱う。
-   **字幕の json3 と info.json の読み取りは、この部品を使う。** 部品を移すことで `internal/transcript` の返す番兵とその条件は変えない（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §3.5）。

この決定の仕様の正本は [package_reference.md](../conventions/package_reference.md) の `internal/strictjson` の行である。部品の置き場所と、LLM のアダプタとの関係は [ADR-0001](0001-llm-adapter-stdlib.md) が定める。

## 採らなかった案

-   **厳格な JSON の部品を `internal/llm/deepseek` に複製する。** サロゲートの走査などの細かいコードが 2 か所になる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **`internal/transcript` の非公開の関数を公開して LLM のアダプタ（構成要素）から使う。** LLM のアダプタ（構成要素）が字幕取得の構成要素に依存する（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **宣言していないキーの拒否を、メンバーの数と消費したキーの数の比較で行う。** `Object` のメンバーが非公開で数を取れず、数を公開すると呼び出し元ごとに比べる処理を書くことになり、比べ忘れる余地が残る（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md) §3.5.3）。

## 結果

-   検証を通った値だけが読み取りの関数に渡るので、`encoding/json` が黙って補正した値を記事や状態に使う経路がパッケージの外から作れない。
-   拒否のエラーは破られた規則を示す。`CollectOnly` と `AsNonNegativeInt64` は、入力の値やキーの文字列を含めない。`Collect` は、重複したキーを `%q` で示す。
-   負でない整数の規則が 1 か所になり、`-0`・小数・指数・範囲外の扱いが利用者ごとにずれない。
-   利用者を足すときは、`package_reference.md` の一覧を同じ変更で更新する。この ADR は部品の形を定め、使うパッケージの一覧は持たない。

## 経緯

- 2026-10-02 DeepSeek アダプタを書くとき、字幕のパーサの中にあった厳格な JSON の部品を `internal/strictjson` に移し、検証済みの値だけを扱う不透明な型として公開した（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md)）
- 2026-10-10 検証・推敲の応答を読むために、宣言していないキーを拒否する `Object.CollectOnly` と、真偽値を読む `Value.AsBool` を加えた（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md)）
- 2026-10-15 トークン数を読むために `Value.IsNull` と `Value.AsNonNegativeInt64` を加え、負でない整数を `int64` で表すことにした（[0019 設計書](../../tasks/0019_token_usage/02_architecture.md)）
- 2026-10-16 字幕の `tStartMs` の読み取りを `Value.AsNonNegativeInt64` に一本化し、自前の負の値の検査を削除した（[0023 設計書](../../tasks/0023_non_negative_int_unification/02_architecture.md)）
- 2026-10-27 ホストがサーバへ本文をそのまま中継できるように `Value.Raw` を加え、`Object.Has` を振る舞いの選択に使える契約に改めた（[0044 設計書](../../tasks/0044_article_job_api/02_architecture.md)）
