# ADR-0001: LLM のアダプタは SDK を使わず標準ライブラリで書き、共有の型と送信の経路をプロバイダの外に置く

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

記事の生成は、プロバイダごとの薄いアダプタ（`LLMClient` の実装）を通して LLM を呼ぶ。DeepSeek と Claude の 2 つのプロバイダがあり、将来 Gemini を加える前提である（[project_overview.md](../project_overview.md)「決定済みの方針」）。

アダプタを足すたびに、次のものが同じ振る舞いを求められる。

-   HTTP の送信の手順: リダイレクトに従わない、1 本の期限で送信から本文の読み取りまでをカバーする、応答本文を上限までしか読まない、通信の失敗を期限切れ・キャンセル・通信の失敗に分ける。
-   プロバイダに依存しない型と番兵: `GenerateRequest`・`GenerateResponse`・`LLMClient`、および不正なリクエスト・打ち切り・想定外の終了理由・空の応答。
-   応答の厳格な検証: 標準ライブラリのデコーダが黙って補正する入力を拒否する部品。
-   プロバイダ固有のパラメータの型: Claude の effort・ワークスペース ID、DeepSeek の reasoning effort。

これらを各アダプタに複製すると、秘密情報の扱いに関わるコードと検証のコードがプロバイダの数だけ増える。また、設定の読み込み（`internal/config`）はアダプタのパッケージを import できないので、設定とアダプタの両方が使う型は、どちらのパッケージにも置けない。

## 決定

-   **SDK を使わず、標準ライブラリで API を呼ぶ。** Claude は Anthropic Messages API を、DeepSeek は OpenAI 互換の Chat Completions API を、`net/http` と `encoding/json` で直接呼ぶ。現在のモジュールは、標準ライブラリの外のモジュールを 1 つも持たない。
-   **プロバイダに依存しない型と番兵は `internal/llm` に置く。** `LLMClient` と `GenerateRequest`・`GenerateResponse`、`ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse`、リクエストの検証はここにある。
-   **HTTP の送信の経路は `internal/llm/llmhttp` に置き、両方のアダプタが使う。** 期限の適用、リダイレクトの禁止、応答本文の上限、失敗の分類をここで行う。`llmhttp` はプロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告する。HTTP の状態・応答本文の形・通信の失敗の番兵は、プロバイダの名前を持つので各アダプタに残す。
-   **プロバイダ固有のパラメータの型は、アダプタの外の小さなパッケージに置く。** Claude は `internal/llm/claudeparam`、DeepSeek は `internal/llm/deepseekparam` である。どちらも標準ライブラリだけに依存し、設定の読み込みとアダプタの両方が使う。
-   **応答の厳格な検証は `internal/strictjson` に置き、複数のパッケージで共有する。** 字幕のパーサ、両方のアダプタ、`ArticleWriter`、`internal/publisher`、`internal/artifact`、`internal/fixprguard`、`internal/server`、`internal/nmhost`、`internal/codexreview`、`internal/coveragecompare` が使う。

この決定の仕様の正本は [project_overview.md](../project_overview.md) の「決定済みの方針」と「パイプライン」、および [package_reference.md](../conventions/package_reference.md) のパッケージの表である。

## 採らなかった案

-   **プロバイダの公式 SDK を使う。** SDK の JSON のデコードは寛容で、応答を補正せずに拒否する検証と両立しにくい。外部モジュールを増やさない方針にも合わない（[0009 要件定義書](../../tasks/0009_claude_llm_client/01_requirements.md) §2.3）。将来 Gemini を加えるときも、SDK が HTTP 通信を持つため `llmhttp` を使わない見込みである（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §9）。
-   **HTTP の送信の手順を各アダプタに持つ。** 秘密情報の扱いに関わるコードを 2 か所に持つことになる（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §1.1）。
-   **厳格な JSON の部品を `internal/llm/deepseek` に複製する。** サロゲートの走査などの細かいコードが 2 か所になる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **`internal/transcript` の非公開の関数を公開して LLM の段階から使う。** LLM の段階が字幕取得の段階に依存する（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **番兵をすべて各アダプタに置く。** `ArticleWriter` が打ち切りを判別するために、プロバイダのパッケージを import することになる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **番兵をすべて `internal/llm` に置く。** SDK を使うプロバイダが、HTTP ステータスや応答本文の形に固有の番兵を `internal/llm` に合わせる必要が生じる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **`claudeparam` を `llmparam` に改名し、2 つのプロバイダの型をそこにまとめる。** 改名の差分が大きく、プロバイダの詳細をそのプロバイダのパッケージに置く境界もぼやける（[0051 設計書](../../tasks/0051_llm_effort/02_architecture.md) §1.2）。
-   **アダプタの具体型を公開する。** パッケージの外でゼロ値を作れ、構築時の検証を通らない値で `Generate` が呼ばれるのを検出する分岐とテストが要る（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **`http.Client` の差し替え口を公開する。** 本番のコードから、API キーを任意のホストへ送れる構成を作れる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **`http.Client.Timeout` を期限と併用する。** どちらの期限が先に発火したかで、返るエラーの型が変わる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。

## 結果

-   2 つのアダプタは 1 つの送信の経路を共有する。送信の振る舞い（リダイレクト、期限、本文の上限、失敗の分類）を変えると、両方のアダプタに同時に効く。
-   モジュールは標準ライブラリの外のモジュールを持たない。プロバイダの API の形と、その API が返す値を検証するコードは、各アダプタが持つ。
-   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`internal/llm` の型と `llmhttp` は変えない。
-   プロバイダ固有の失敗（HTTP ステータス・応答本文の形・通信の失敗）は、プロバイダごとの番兵で報告される。呼び出し側は、プロバイダに依存しない 4 つの番兵だけで、打ち切り・想定外の終了理由・空の応答・不正なリクエストを判別できる。

## 経緯

- 2026-10-02 DeepSeek アダプタを標準ライブラリで書き、プロバイダに依存しない型と番兵を `internal/llm` に置いた（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md)）
- 2026-10-10 Claude アダプタを加えるとき、HTTP の送信の手順を `internal/llm/llmhttp` に切り出し、effort とワークスペース ID の型を `internal/llm/claudeparam` に置いた（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md)）
- 2026-10-30 DeepSeek でも effort を工程ごとに指定できるようにしたとき、その型を `internal/llm/deepseekparam` に置いた（[0051 設計書](../../tasks/0051_llm_effort/02_architecture.md)）
