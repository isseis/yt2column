# ADR-0019: パイプラインの構成要素を interface にし、構成要素ごとに型を持つパッケージを分け、番兵はプロバイダに依存するかで置き場所を分ける

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

記事を作る処理は、字幕の取得・記事の作成・公開というステージを順に呼ぶ。ステージの実装は、CLI と localhost サーバ、テストの fake で差し替える。ステージの間で受け渡すデータ型と、失敗を呼び出し側が判別するための番兵も、ステージごとに決まる。

実装を差し替えられるようにするには、呼び出す側が実装の型ではなく契約だけに依存する必要がある。契約を各ステージのパッケージに置けば、実装を足してもパイプラインは変わらない。逆に、プロバイダに依存する型や番兵を共通の場所へ集めると、プロバイダを足すたびに共通の型を変えることになる。

字幕のステージには、利用者がコピーした字幕を解析する実装と、外部のコマンド `yt-dlp` を起動して字幕を取得する実装がある。前者は拡張から使う主経路で、後者は CLI だけが使う。外部のプロセスを起動する実装と同じパッケージにあると、型と interface を使うだけの側も、プロセスの起動に要るプラットフォームの機能に依存する。

LLM の失敗は、どのプロバイダでも起こるもの（不正なリクエスト・打ち切り・想定外の終了理由・空の応答）と、プロバイダの API に固有のもの（HTTP の状態・応答本文の形・通信の失敗）に分かれる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §1.1）。

## 決定

-   **構成要素は、それぞれのパッケージが interface と入出力のデータ型を持つ。** `TranscriptSource` は `internal/transcript`、`ArticleWriter` は `internal/writer`、`Publisher` は `internal/publisher`、`LLMClient` は `internal/llm` が持つ（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §1.1・§3.2）。後続のタスクが、定めた型と interface だけに依存し、fake を相手に単体で開発とテストを進められるようにするためである（[0001 要件定義書](../../tasks/0001_pipeline_skeleton/01_requirements.md) §1）。規則は [CLAUDE.md](../../../CLAUDE.md) の「Key Design Patterns」の「Separation of Concerns」と、[package_reference.md](../conventions/package_reference.md) のパッケージの表にある。
-   **`TranscriptSource` の実装のうち、`yt-dlp` を起動するもの（`YtDlpSource`）は `internal/ytdlpsource` に置く。** `internal/transcript` には、データ型・interface・コピーした字幕の実装（`CopiedSource`）・動画 URL の検証・字幕のステージの番兵を残す。型と interface だけを使う側（記事の作成や、拡張のページで動かす WASM の版）が、プロセスの起動・ファイルのキャッシュに依存しないようにするためである（[0084 設計書](../../tasks/0084_wasm_pipeline_poc/02_architecture.md)）。`yt-dlp` の経路の失敗の番兵（実行の失敗・字幕と info.json の解析の失敗・字幕がないこと）も `internal/transcript` に残す。呼び出し側は、今と同じく `internal/transcript` の番兵で判別する。規則は [package_reference.md](../conventions/package_reference.md) のパッケージの表の `internal/transcript` と `internal/ytdlpsource` の行に置く。
-   **[#772](https://github.com/isseis/yt2column/issues/772) が拡張だけの構成を決めるまで、型と interface だけを使う側のパッケージ（`internal/transcript`・`internal/writer`・`internal/llm/claude`・`internal/pricing`）を `GOOS=js GOARCH=wasm` でビルドできることを、`make` の目標と CI の検査で守る。** 検査がないと、後の変更がこれらを再び WASM でビルドできなくしても #772・#773 まで気づかず、PoC で測った値が `main` に当てはまらなくなるためである（[0084 要件定義書](../../tasks/0084_wasm_pipeline_poc/01_requirements.md)「要件定義の際に決めたこと」）。Go の WASM にすると決めるものではなく、#772 が TS に一本化すると決めたら外し、Go の WASM にすると決めたら残す。検査の目標は `make wasm-check`（[CLAUDE.md](../../../CLAUDE.md) の「Code Quality」）である。
-   **`internal/pipeline` は interface だけに依存し、実装の詳細を知らない。** 各構成要素の実装を差し替えられるようにするためである。規則は [CLAUDE.md](../../../CLAUDE.md) の「Key Design Patterns」の「Interface-based Design」と、[project_overview.md](../project_overview.md) の「パイプライン」にある。
-   **ステージの間で受け渡すデータ型が契約である。ステージは値を受け渡し、空の結果を正常として返さないことを各 interface の英語のドキュメントコメントに書く。** 値で返すので nil で正否を判定できず、空の場合は必ずエラーを返す契約にするためである（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §3.2）。契約の文面は、各構成要素のパッケージの interface のドキュメントコメントにある。
-   **すべてのメソッドは第 1 引数に `context.Context` を受け取り、キャンセルとタイムアウトに従う。** 理由は記録されていない。規則は [0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) の「3.2. 段階の interface」にある。
-   **どのプロバイダでも起こる失敗の番兵と、リクエストの検証は `internal/llm` に置く。** 不正なリクエスト・打ち切り・想定外の終了理由・空の応答の番兵と、`GenerateRequest.Validate` である。呼び出し側が、プロバイダのパッケージを import せずに、共通の番兵だけで打ち切りなどを判別できるようにするためである（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §3.2）。番兵の一覧は [package_reference.md](../conventions/package_reference.md) のパッケージの表の `internal/llm` の行にある。
-   **プロバイダの API に固有の失敗の番兵は、各アダプタのパッケージに置く。** HTTP の状態・応答本文の形・通信の失敗の番兵は、`internal/llm/deepseek` と `internal/llm/claude` がそれぞれ持つ。これらは HTTP と JSON の形に依存し、SDK を使うプロバイダでは SDK のエラーから変換する別の形になるためである（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §3.2）。規則は [package_reference.md](../conventions/package_reference.md) のパッケージの表の、各アダプタの行にある。

LLM のアダプタの構成と送信の経路、プロバイダ固有のパラメータの型は [ADR-0001](0001-llm-adapter-stdlib.md) が、サーバの構成は [ADR-0005](0005-server-architecture.md) が定める。

## 採らなかった案

-   **番兵をすべて各アダプタに置く。** `ArticleWriter` が打ち切りを判別するために、プロバイダのパッケージを import することになる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
-   **番兵をすべて `internal/llm` に置く。** SDK を使うプロバイダが、HTTP ステータスや応答本文の形に固有の番兵を `internal/llm` に合わせる必要が生じる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。

-   **`yt-dlp` の経路のファイルにビルドタグ（`//go:build !js`）を付け、`internal/transcript` に残す。** 差分は最も小さいが、同じパッケージの API がプラットフォームによって変わり、型だけを使う側がプロセスの起動に依存しないことをパッケージの単位で示せない（[0084 設計書](../../tasks/0084_wasm_pipeline_poc/02_architecture.md)）。
-   **`yt-dlp` の実装を `internal/transcript` の子パッケージに置く。** `internal/llm/<provider>` と同じ形になるが、パッケージの階層が深くなり、テストが `testdata/` を指す相対パスの書き換えが要る（[0084 設計書](../../tasks/0084_wasm_pipeline_poc/02_architecture.md)）。

## 結果

-   パイプラインはステージの interface だけに依存するので、実装を fake に差し替えてテストでき、ステージを足してもパイプラインの呼び出し以外は変わらない。
-   呼び出し側は、プロバイダに依存しない 4 つの番兵だけで、不正なリクエスト・打ち切り・想定外の終了理由・空の応答を判別できる。プロバイダ固有の失敗は、そのアダプタを選んだ呼び出し側だけが扱う。
-   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`llmhttp` は変えない。プロバイダに依存しない型は変えない。
-   `internal/transcript` は、外部のプロセスとファイルのキャッシュに依存しない。CLI の字幕の取得の処理（コマンドの実行・キャッシュ・json3 と info.json の解析）を変えるときは、`internal/ytdlpsource` を変える。偽の `yt-dlp` のスクリプトは、`internal/transcript/testutil` に残る。上の 4 つのパッケージに、`js` のビルドで使えない OS に固有の API（`syscall` のフラグやプロセスの起動など）を足す変更は、#772 が決めるまで CI で止まる。そうした処理は、`js` 以外のビルドのファイルに分けるか、`internal/ytdlpsource` のような別のパッケージに置く。
-   LLM のアダプタの構成と送信の経路は [ADR-0001](0001-llm-adapter-stdlib.md) に残る。この ADR は、ステージの構成要素と番兵の置き場所だけを定める。

## 経緯

- 2026-09-29 パイプラインのステージを interface にし、ステージごとのパッケージが型を持つ形にした。空の結果を正常としない契約を interface のドキュメントコメントに書いた（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md)）
- 2026-10-02 DeepSeek アダプタを書くとき、プロバイダに依存しない番兵を `internal/llm` に、HTTP と応答本文に依存する番兵をアダプタに置いた（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md)）
- 2026-10-10 Claude アダプタを足すとき、番兵をアダプタごとに置く方針を保ち、HTTP の送信の経路を `internal/llm/llmhttp` に切り出した（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md)）
- 2026-11-11 `yt-dlp` を起動する `TranscriptSource` の実装を `internal/transcript` から `internal/ytdlpsource` に移すことにした（[0084 設計書](../../tasks/0084_wasm_pipeline_poc/02_architecture.md)）
- 2026-11-11 `internal/transcript`・`internal/writer`・`internal/llm/claude`・`internal/pricing` を WASM でビルドできることを、#772 が決めるまで `make` の目標と CI の検査で守ることにした（[0084 要件定義書](../../tasks/0084_wasm_pipeline_poc/01_requirements.md)「要件定義の際に決めたこと」）
