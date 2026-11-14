# ADR-0020: 厳格な JSON の部品を internal/strictjson に集め、検証済みの値だけを扱う不透明な型にし、負でない整数の規則を 1 つにする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

LLM の API の応答、字幕の json3 と info.json、中間生成物、拡張のページから記事生成モジュールへ渡す JSON など、外部から受け取る JSON を読む箇所が複数ある。標準ライブラリの `encoding/json` は、不正な UTF-8、対になっていないサロゲートのエスケープ、後続のデータ、重複したメンバー、`null` を黙って補正することがある。補正された値をそのまま使うと、検証した値と実際に受理される値がずれる。

負でない整数の検査を各利用者が書くと、`-0` や小数の扱いが箇所ごとにずれる。

## 決定

-   **検証済みの JSON から値を取り出す部品は `internal/strictjson` にだけ置き、複数のパッケージで共有する。部品は、`encoding/json` が黙って補正する入力を拒否する。** 部品を複製するとサロゲートの走査などの細かいコードが 2 か所になり、`internal/transcript` に残すと LLM のアダプタ（構成要素）が字幕取得の構成要素に依存するためである。拒否する入力は [0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) の「3.5. `internal/strictjson`（厳格な JSON の部品）」と、`internal/strictjson` のパッケージと各メソッドの doc コメントにある。
-   **`Object` と `Value` は不透明な型にし、中身を持つ値は `ParseObject` からしか得られないようにする。** フィールドは非公開にする。ゼロ値の `Value` は中身を持たず、その読み取りはエラーを返す。ゼロ値の `Object` はメンバーを持たない。検証を経ていないバイト列を読み取りの関数に渡して U+FFFD への置き換えを受ける呼び出しを、規約でなく型で防ぎ、公開範囲を広げても検証済みの値だけを扱うという不変条件を保つためである。型の形は [0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) の「3.5. `internal/strictjson`（厳格な JSON の部品）」にある。
-   **読み取りの手段は、利用者の必要に合わせて `internal/strictjson` の 1 か所に持つ。** 検査を呼び出し元ごとに書くと書き忘れる余地が残り、部品の中で完結させればその余地がないためである（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md) の「3.5.3. 応答の形と受理の規則（H-01）」）。各手段の契約は `internal/strictjson` の各メソッドの doc コメントにある。`Object.Collect` の契約は [0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) の「3.5. `internal/strictjson`（厳格な JSON の部品）」にある。
-   **負でない整数の規則は `Value.AsNonNegativeInt64` の 1 か所にする。** 負でない整数の検査を各利用者が書くと、`-0` や小数の扱いが箇所ごとにずれるためである。受理する構文と、拒否のエラーに入力の値を含めないことは、`Value.AsNonNegativeInt64` の doc コメントにある。
-   **負でない整数が要る箇所では `Value.AsInt64` を使わない。** `AsInt64` は符号と `-0` を受理するためである。規則は [0023 設計書](../../tasks/0023_non_negative_int_unification/02_architecture.md) の「1. 設計の全体像 (Design Overview)」にある。
-   **この部品を使うパッケージの一覧は [package_reference.md](../conventions/package_reference.md) が持つ。** 同じ一覧をほかの文書にも書くと、利用者が増えるたびに何か所も直すことになるためである（[0068 要件定義書](../../tasks/0068_strictjson_package_reference/01_requirements.md) の「1. 概要 (Overview)」）。一覧は [package_reference.md](../conventions/package_reference.md) の `internal/strictjson` の行にある。

-   **拡張のページから記事生成モジュールへ渡す JSON も、この部品で読む。** 受け取るメンバの集合を閉じ、知らないメンバと重複したメンバを拒否する。入口の境界で、検証した値と受理する値をずらさないためである。規則は [0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md) の「信頼できない入力の境界（要件書 AC-18）」にある。

記事生成モジュールがこの部品で JSON を読むことは、まだコードに反映していない。今は localhost サーバが生成の開始の本文をこの部品で読む。この差分は [#773](https://github.com/isseis/yt2column/issues/773) の実装で解消する。

部品の置き場所と、LLM のアダプタとの関係は [ADR-0001](0001-llm-adapter-stdlib.md) が定める。

## 採らなかった案

-   **厳格な JSON の部品を `internal/llm/deepseek` に複製する。** サロゲートの走査などの細かいコードが 2 か所になる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **`internal/transcript` の非公開の関数を公開して LLM のアダプタ（構成要素）から使う。** LLM のアダプタ（構成要素）が字幕取得の構成要素に依存する（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **宣言していないキーの拒否を、メンバーの数と消費したキーの数の比較で行う。** `Object` のメンバーが非公開で数を取れず、数を公開すると呼び出し元ごとに比べる処理を書くことになり、比べ忘れる余地が残る（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md) §3.5.3）。

## 結果

-   検証を通った値だけが読み取りの関数に渡るので、`encoding/json` が黙って補正した値を記事や状態に使う経路がパッケージの外から作れない。
-   拒否のエラーは破られた規則を示す。`CollectOnly` と `AsNonNegativeInt64` は、入力の値やキーの文字列を含めない。`Collect` は、重複したキーを `%q` で示す。
-   負でない整数の規則が 1 か所になり、`-0`・小数・指数・範囲外の扱いが利用者ごとにずれない。
-   利用者を足すときは、`package_reference.md` の一覧を同じ変更で更新する。この ADR は部品の形を定め、使うパッケージの一覧は持たない。
-   `package_reference.md` の利用者の一覧とコードの利用者が食い違うと、`internal/pipeline` のテストが失敗し、`make test` と CI の `doc-guard` が通らない。

## 経緯

- 2026-10-02 DeepSeek アダプタを書くとき、字幕のパーサの中にあった厳格な JSON の部品を `internal/strictjson` に移し、検証済みの値だけを扱う不透明な型として公開した（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md)）
- 2026-10-10 検証・推敲の応答を読むために、宣言していないキーを拒否する `Object.CollectOnly` と、真偽値を読む `Value.AsBool` を加えた（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md)）
- 2026-10-15 トークン数を読むために `Value.IsNull` と `Value.AsNonNegativeInt64` を加え、負でない整数を `int64` で表すことにした（[0019 設計書](../../tasks/0019_token_usage/02_architecture.md)）
- 2026-10-16 字幕の `tStartMs` の読み取りを `Value.AsNonNegativeInt64` に一本化し、自前の負の値の検査を削除した（[0023 設計書](../../tasks/0023_non_negative_int_unification/02_architecture.md)）
- 2026-10-27 ホストがサーバへ本文をそのまま中継できるように `Value.Raw` を加え、`Object.Has` を振る舞いの選択に使える契約に改めた（[0044 設計書](../../tasks/0044_article_job_api/02_architecture.md)）
- 2026-11-09 `package_reference.md` の利用者の一覧をコードに合わせ、一覧とコードのずれを `make test` で見つけるテストを加えた（[0068 設計書](../../tasks/0068_strictjson_package_reference/02_architecture.md)）
- 2026-11-12 拡張だけの構成で、拡張のページから記事生成モジュールへ渡す JSON もこの部品で読むことにした（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)）
