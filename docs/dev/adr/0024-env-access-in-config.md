# ADR-0024: プロセスの環境変数を設定として読むのは internal/config と決めた入口だけに限る

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

API キーは環境変数から読む。読む場所がコードベースの各所に散ると、秘密情報を読む経路を数えられず、設定の検証を通らない値が使われる。開発者向けの道具は、子プロセスに環境を渡すために環境全体を読む必要がある。どの場所が環境を読んでよいかを決め、テストで固定する必要がある（[0005 要件定義書](../../tasks/0005_cli_assembly/01_requirements.md) F-002）。

## 決定

-   **プロセスの環境変数を設定として読むのは `internal/config` だけにする。** 設定を使うコードは、環境そのものではなく、`internal/config` が返した値か、名前を引く関数を受け取る。秘密情報の変数名は、テスト以外の Go のコードでは `internal/config` にだけ現れる。秘密情報を読む場所を限るためである（[0005 設計書](../../tasks/0005_cli_assembly/02_architecture.md) §3.1 の「環境変数を読む場所の検査（F-002・AC-09）」）。規則は [0005 要件定義書](../../tasks/0005_cli_assembly/01_requirements.md) の F-002 にある。秘密情報の変数名の一覧は `internal/config/envaccess_test.go` にある。
-   **`internal/config` の外で環境を参照してよいのは、決めた入口だけである。** 入口は 3 種類で、プロセスの環境を名前を引く関数として設定の読み込みに渡す、CLI・サーバ・Native Messaging のホスト・開発用のサーバの `main` パッケージ、`yt-dlp` に引き継ぐ変数を allowlist で選ぶ処理（[security.md](../security.md) §1）、子プロセスに渡す環境を読む開発者向けの道具である。各 `main` はプロセスの環境を `internal/config` に渡すためだけに参照し、`yt-dlp` に引き継ぐ処理は秘密情報の変数を引き継がない（0005 要件定義書 F-002）。開発者向けの道具は、子プロセスに環境を渡すために環境全体を読む必要がある。許す場所とその参照の数の一覧は `internal/config/envaccess_test.go` にある。
-   **標準ライブラリが自分の動作のために読む変数は、この規則の対象にしない。** `exec.LookPath` の `PATH`、`os.TempDir` の `TMPDIR`、`os.Getwd` の `PWD` などは、呼び出し元に値を返さず、秘密情報の変数を読まない。秘密情報を読む場所を限るという目的は、この区別で満たせるためである（0005 設計書 §3.1 の「環境変数を読む場所の検査（F-002・AC-09）」）。
-   **規則は `internal/config/envaccess_test.go` が、テスト以外の Go のコードを走査して確かめる。** 対象の関数と許す場所を一覧で持ち、一覧にない参照と、`internal/config` 以外の秘密情報の変数名を失敗にする。新しい入口を足すときは、この一覧に行を足す。理由は記録されていない。

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
- 2026-11-11 fake の LLM を使う開発用のサーバを足すとき、その `main.go` を入口の 1 つ目の種類に加えた。開発用のサーバが読むのは `HOME` だけである（[0086 設計書](../../tasks/0086_dev_fake_llm_server/02_architecture.md)）
