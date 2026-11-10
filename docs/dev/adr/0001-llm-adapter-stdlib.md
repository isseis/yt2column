# ADR-0001: LLM のアダプタは SDK を使わず標準ライブラリで書き、共有の型と送信の経路をプロバイダの外に置く

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

記事の生成は、プロバイダごとの薄いアダプタ（`LLMClient` の実装）を通して LLM を呼ぶ。DeepSeek と Claude の 2 つのプロバイダがあり、将来 Gemini を加える前提である（[project_overview.md](../project_overview.md)「決定済みの方針」「パイプライン」）。

アダプタを足すたびに、HTTP の送信の手順、応答の厳格な検証、プロバイダ固有のパラメータの型が、どのアダプタでも同じ振る舞いを求められる。

これらを各アダプタに複製すると、秘密情報の扱いに関わるコードと検証のコードがプロバイダの数だけ増える。また、設定の読み込み（`internal/config`）はアダプタのパッケージを import できないので、設定とアダプタの両方が使う型は、どちらのパッケージにも置けない。

## 決定

-   **SDK を使わず、標準ライブラリで API を呼ぶ。** Claude は Anthropic Messages API を、DeepSeek は OpenAI 互換の Chat Completions API を、OpenAI は Responses API を、`net/http` と `encoding/json` で直接呼ぶ。OpenAI の追加は [0077 要件定義書](../../tasks/0077_openai_llm_client/01_requirements.md) が定め、API 固有の方針は [ADR-0041](0041-openai-api.md) に従う。提供元ごとの詳細を閉じ込め、OpenAI の要求・応答の処理は DeepSeek と共通化しない。SDK の JSON のデコードは寛容で、応答を補正せずに拒否する検証と両立しにくく、外部モジュールを増やさない方針にも合うためである（[0009 要件定義書](../../tasks/0009_claude_llm_client/01_requirements.md) §2.3）。規則は [project_overview.md](../project_overview.md) の「決定済みの方針」にある。
-   **HTTP の送信の経路は `internal/llm/llmhttp` の 1 か所に置き、Claude・DeepSeek・OpenAI のアダプタが使う。** 秘密情報の扱いに関わるコードを 2 か所に持たないためである（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §1.1）。`llmhttp` が受け持つ送信の手順は、[package_reference.md](../conventions/package_reference.md) のパッケージの表の `internal/llm/llmhttp` の行にある。
-   **`llmhttp` はプロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告する。** 各アダプタが自分の番兵を保つためである（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §3.4）。規則は [package_reference.md](../conventions/package_reference.md) のパッケージの表の `internal/llm/llmhttp` の行にある。
-   **プロバイダ固有のパラメータの型は、アダプタの外の小さなパッケージに置く。** Claude は `internal/llm/claudeparam`、DeepSeek は `internal/llm/deepseekparam`、OpenAI は `internal/llm/openaiparam` である。どれも標準ライブラリだけに依存し、設定の読み込みとアダプタの両方が使う。`internal/config` はアダプタのパッケージを import できないので、両方が参照する型はアダプタの外に置く必要があるためである（[0051 設計書](../../tasks/0051_llm_effort/02_architecture.md) §1.2）。規則は [package_reference.md](../conventions/package_reference.md) のパッケージの表のそれぞれのパッケージの行にある。
-   **アダプタは、応答の厳格な検証に、[ADR-0020](0020-strict-json.md) が定める共有の部品を使う。** 部品を複製すると、サロゲートの走査などの細かいコードが 2 か所になるためである（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。部品の規則は [ADR-0020](0020-strict-json.md) が定める。

プロバイダに依存しない型と番兵、その置き場所は [ADR-0019](0019-pipeline-component-interfaces.md) が定める。

OpenAI のアダプタ本体はまだコードに反映していない。この差分は [#568](https://github.com/isseis/yt2column/issues/568) の実装で解消する。

## 採らなかった案

-   **プロバイダの公式 SDK を使う。** SDK の JSON のデコードは寛容で、応答を補正せずに拒否する検証と両立しにくい。外部モジュールを増やさない方針にも合わない（[0009 要件定義書](../../tasks/0009_claude_llm_client/01_requirements.md) §2.3）。将来 Gemini を加えるときも、SDK が HTTP 通信を持つため `llmhttp` を使わない見込みである（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §9）。
-   **HTTP の送信の手順を各アダプタに持つ。** 秘密情報の扱いに関わるコードを 2 か所に持つことになる（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §1.1）。
-   **`claudeparam` を `llmparam` に改名し、2 つのプロバイダの型をそこにまとめる。** 改名の差分が大きく、プロバイダの詳細をそのプロバイダのパッケージに置く境界もぼやける（[0051 設計書](../../tasks/0051_llm_effort/02_architecture.md) §1.2）。
-   **アダプタの具体型を公開する。** パッケージの外でゼロ値を作れ、構築時の検証を通らない値で `Generate` が呼ばれるのを検出する分岐とテストが要る（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **`http.Client` の差し替え口を公開する。** 本番のコードから、API キーを任意のホストへ送れる構成を作れる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **`http.Client.Timeout` を期限と併用する。** どちらの期限が先に発火したかで、返るエラーの型が変わる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。

## 結果

-   これらのアダプタは 1 つの送信の経路を共有する。送信の振る舞い（リダイレクト、期限、本文の上限、失敗の分類）を変えると、各アダプタに同時に効く。
-   モジュールは標準ライブラリの外のモジュールを持たない。プロバイダの API の形と、その API が返す値を検証するコードは、各アダプタが持つ。
-   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`llmhttp` は変えない。プロバイダに依存しない型は [ADR-0019](0019-pipeline-component-interfaces.md) が定める。

## 経緯

- 2026-10-02 DeepSeek アダプタを標準ライブラリで書き、プロバイダに依存しない型と番兵を `internal/llm` に置いた（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md)）
- 2026-10-10 Claude アダプタを加えるとき、HTTP の送信の手順を `internal/llm/llmhttp` に切り出し、effort とワークスペース ID の型を `internal/llm/claudeparam` に置いた（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md)）
- 2026-10-30 DeepSeek でも effort を工程ごとに指定できるようにしたとき、その型を `internal/llm/deepseekparam` に置いた（[0051 設計書](../../tasks/0051_llm_effort/02_architecture.md)）
- 2026-11-10 OpenAI の Responses API 用のアダプタを標準ライブラリと共有の送信経路で追加し、DeepSeek の要求・応答の処理とは共通化しないことにした（[0077 要件定義書](../../tasks/0077_openai_llm_client/01_requirements.md)）
