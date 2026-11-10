# ADR-0024: プロセスの環境変数を設定として読むのは internal/config と決めた入口だけに限る

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

API キーと Webhook URL は環境変数から読む。読む場所がコードベースの各所に散ると、秘密情報を読む経路を数えられず、設定の検証を通らない値が使われる。開発者向けの道具は、子プロセスに環境を渡すために環境全体を読む必要がある。どの場所が環境を読んでよいかを決め、テストで固定する必要がある（[0005 要件定義書](../../tasks/0005_cli_assembly/01_requirements.md) F-002）。

## 決定

-   **プロセスの環境変数を設定として読むのは `internal/config` だけにする。** 設定を使うコードは、環境そのものではなく、`internal/config` が返した値か、名前を引く関数を受け取る。秘密情報の変数名（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`SLACK_WEBHOOK_URL`）は、テスト以外の Go のコードでは `internal/config` にだけ現れる。
-   **`internal/config` の外で環境を参照してよいのは、次の場所だけである。**
    -   `cmd/yt2column/main.go`・`cmd/yt2column-server/main.go`・`cmd/yt2column-nmhost/main.go` は、`os.LookupEnv` を 1 回だけ参照し、その関数を設定の読み込みに渡す。
    -   `internal/transcript/ytdlp.go` は、`os.Environ` を 1 回だけ参照し、`yt-dlp` に引き継ぐ変数を allowlist で選ぶ（[security.md](../security.md) §1）。
    -   `cmd/fixprguard/main.go` は、git に渡す環境を読み、`GH_REPO` と `GH_HOST` は拒否するためだけに参照する。`cmd/codexreview/main.go`・`cmd/coveragecompare/main.go` は、子プロセスに渡す環境を読む。
-   **標準ライブラリが自分の動作のために読む変数は、この規則の対象にしない。** `exec.LookPath` の `PATH`、`os.TempDir` の `TMPDIR`、`os.Getwd` の `PWD` などは、呼び出し元に値を返さず、秘密情報の変数を読まない。
-   **規則は `internal/config/envaccess_test.go` が、テスト以外の Go のコードを走査して確かめる。** 対象の関数と許す場所を一覧で持ち、一覧にない参照と、`internal/config` 以外の秘密情報の変数名を失敗にする。新しい入口を足すときは、この一覧に行を足す。

LLM の設定の値の文法と、値の解析を `internal/config` が行うことは [ADR-0003](0003-llm-config-contract.md) が定める。

## 採らなかった案

-   **ソケットのパスなどに `TMPDIR` を使う。** `HOME` だけからパスを導出する方針を崩し、標準ライブラリでは `confstr` を呼べないので採らない（[0026 design_handoff.md](../../tasks/0026_native_messaging_relay/design_handoff.md) H-10）。
-   **標準ライブラリが内部で読む `PATH`・`TMPDIR`・`PWD` も、検査の対象にする。** これらは呼び出し元に値を返さず、秘密情報の変数を読まない。対象にすると、無関係なコードを許可の一覧に足すことになる（[0005 設計書](../../tasks/0005_cli_assembly/02_architecture.md) §3.1）。

## 結果

-   秘密情報を読む場所が 1 か所になり、設定の検証を通らない値が使われる経路がない。
-   新しい入口が環境を読むときは、`internal/config/envaccess_test.go` の一覧を同じ変更で更新する。テストは、一覧にない参照を失敗にする。
-   開発者向けの道具は、子プロセスに渡す環境を読める。秘密情報の変数名を持たないことは、同じ検査が確かめる。
-   [ADR-0018](0018-server-config-file.md) は、サーバが設定ファイルだけから設定を読み、環境からは `HOME` と `GODEBUG` だけを読むと決めている。サーバのコードが環境から読むのはその 2 つだけである。

## 経緯

- 2026-10-05 CLI を組み立てるとき、環境変数の読み込みを `internal/config` に集め、環境を読む場所の検査を加えた（[0005 設計書](../../tasks/0005_cli_assembly/02_architecture.md)）
- 2026-10-10 Claude のアダプタを足すとき、秘密情報の変数名の一覧に `ANTHROPIC_API_KEY` を加えた（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md)）
- 2026-10-13 localhost サーバを足すとき、サーバの `main.go` が `os.LookupEnv` を 1 回だけ参照する場所として、検査の一覧に加えた（[0011 設計書](../../tasks/0011_localhost_server/02_architecture.md)）
- 2026-10-19 Native Messaging のホストを足すとき、ホストの `main.go` を検査の一覧に加えた。ホストが読むのは `HOME` だけである（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md)）
- 2026-11-02 カバレッジの比較の道具を足すとき、その `main.go` が子プロセスに渡す環境を読むことを、検査の一覧に加えた（[0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md)）
- 2026-11-10 ADR-0018 の決定がタスク 0066 で実装されたので、「結果」の項目を今の状態に合わせた（[0078 要件定義書](../../tasks/0078_adr0024_server_env_result/01_requirements.md)）
