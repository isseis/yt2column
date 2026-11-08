# ADR-0019: パイプラインの構成要素を interface にし、構成要素ごとに型を持つパッケージを分け、番兵はプロバイダに依存するかで置き場所を分ける

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

記事を作る処理は、字幕の取得・記事の作成・公開というステージを順に呼ぶ。ステージの実装は、CLI と localhost サーバ、テストの fake で差し替える。ステージの間で受け渡すデータ型と、失敗を呼び出し側が判別するための番兵も、ステージごとに決まる。

実装を差し替えられるようにするには、呼び出す側が実装の型ではなく契約だけに依存する必要がある。契約を各ステージのパッケージに置けば、実装を足してもパイプラインは変わらない。逆に、プロバイダに依存する型や番兵を共通の場所へ集めると、プロバイダを足すたびに共通の型を変えることになる。

LLM の失敗は、どのプロバイダでも起こるもの（不正なリクエスト・打ち切り・想定外の終了理由・空の応答）と、プロバイダの API に固有のもの（HTTP の状態・応答本文の形・通信の失敗）に分かれる。前者を共通の型に、後者を各アダプタに置くと、呼び出し側は共通の番兵だけで打ち切りなどを判別できる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §1.1）。Claude のアダプタも同じ方針を保った（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §1.1）。

## 決定

-   **構成要素は、それぞれのパッケージが interface と入出力のデータ型を持つ。** `TranscriptSource` は `internal/transcript`、`ArticleWriter` は `internal/writer`、`Publisher` は `internal/publisher`、`LLMClient` は `internal/llm` が持つ。`internal/pipeline` は interface だけに依存し、実装の詳細を知らない（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §1.1・§3.2）。
-   **ステージの間で受け渡すデータ型が契約である。** ステージは値を受け渡し、空の結果を正常として返さないことを各 interface の英語のドキュメントコメントに書く。すべてのメソッドは第 1 引数に `context.Context` を受け取り、キャンセルとタイムアウトに従う。
-   **プロバイダに依存しない番兵とリクエストの検証は `internal/llm` に置く。** `ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse` と `GenerateRequest.Validate` がここにある。
-   **プロバイダに固有の番兵は各アダプタに置く。** HTTP の状態・応答本文の形・通信の失敗は、プロバイダの名前を持つので `internal/llm/deepseek` と `internal/llm/claude` がそれぞれ持つ。

LLM のアダプタの構成と送信の経路、プロバイダ固有のパラメータの型は [ADR-0001](0001-llm-adapter-stdlib.md) が、サーバの構成は [ADR-0005](0005-server-architecture.md) が定める。

この決定の仕様の正本は [CLAUDE.md](../../../CLAUDE.md) の「Key Design Patterns」の「Separation of Concerns」と「Interface-based Design」、[project_overview.md](../project_overview.md) の「パイプライン」、[package_reference.md](../conventions/package_reference.md) のパッケージの表である。

## 採らなかった案

-   **番兵をすべて各アダプタに置く。** `ArticleWriter` が打ち切りを判別するために、プロバイダのパッケージを import することになる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **番兵をすべて `internal/llm` に置く。** SDK を使うプロバイダが、HTTP ステータスや応答本文の形に固有の番兵を `internal/llm` に合わせる必要が生じる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。

## 結果

-   パイプラインはステージの interface だけに依存するので、実装を fake に差し替えてテストでき、ステージを足してもパイプラインの呼び出し以外は変わらない。
-   呼び出し側は、プロバイダに依存しない 4 つの番兵だけで、不正なリクエスト・打ち切り・想定外の終了理由・空の応答を判別できる。プロバイダ固有の失敗は、そのアダプタを選んだ呼び出し側だけが扱う。
-   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`llmhttp` は変えない。プロバイダに依存しない型は変えない。
-   LLM のアダプタの構成と送信の経路は [ADR-0001](0001-llm-adapter-stdlib.md) に残る。この ADR は、構成要素と番兵の置き場所だけを定める。

## 経緯

- 2026-09-29 パイプラインのステージを interface にし、ステージごとのパッケージが型を持つ形にした。空の結果を正常としない契約を interface のドキュメントコメントに書いた（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md)）
- 2026-10-02 DeepSeek アダプタを書くとき、プロバイダに依存しない番兵を `internal/llm` に、HTTP と応答本文に依存する番兵をアダプタに置いた（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md)）
- 2026-10-10 Claude アダプタを足すとき、番兵をアダプタごとに置く方針を保ち、HTTP の送信の経路を `internal/llm/llmhttp` に切り出した（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md)）
