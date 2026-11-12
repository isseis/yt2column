# ADR-0049: 記事を作る処理は Go のコアを WebAssembly にして CLI と拡張が共有し、拡張のビルドが WASM と wasm_exec.js を成果物に入れる

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

記事は拡張だけで作り、CLI は開発用の経路として残す。CLI と拡張は、記事を作る処理を共有する（[ADR-0048](0048-extension-only-generation.md)）。記事を作る処理（入力の検査、字幕の解析、3 つの工程、完了の時点の検査、メンションの無効化、保存するファイルの中身、料金の概算）は、今は Go で書かれている。CLI は今後も Go のまま使う。

同じ規則を Go と TypeScript の両方に書くと、片方だけを直す誤りが起きる（[0090 要件定義書](../../tasks/0090_extension_only_architecture/01_requirements.md) §4.5「保守性 (Maintainability)」）。

Go のパイプラインを WASM にしたものは、拡張のページから Claude API を呼んで記事を完成させた。WASM は 17,300,194 バイト（`gzip -9` で 4,138,823 バイト）で、ページの読み込みの開始から入口を呼べるようになるまで、Chrome で 62.4〜171.2 ms、Brave で 49.9〜206.5 ms だった。WASM を読み込むには、拡張のページの CSP に `'wasm-unsafe-eval'` が要った（[調査記録](../../research/wasm_pipeline_poc/README.md)「3 つの問いへの答え」「測定 2：読み込みと CSP、サイズと読み込みの時間」）。本番の構成での記事の生成は 95〜151 秒かかった（同「測定 6：本番の構成（Chrome、`83ef12bd`、背面のまま）」）。

## 決定

-   **記事を作る処理は、Go のコアを `GOOS=js GOARCH=wasm` でビルドした WASM の入口（`cmd/yt2column-wasm`）で、CLI と拡張が共有する。** 入力の検査、字幕の解析、3 つの工程、完了の時点の検査、メンションの無効化、保存するファイルの中身、料金の概算、API キーとワークスペース ID の検査は Go のコードで行い、拡張の TypeScript はこれらの規則を持たない。規則を Go の 1 か所に保つためである。共有する範囲は [0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md) の「共有する処理の範囲」に、拡張のジョブ 1 回分の処理を持つパッケージ（`internal/extjob`）は同じ設計書の「Go のパッケージの構成」にある。
-   **WASM の入口と拡張のページの間は、JSON の文字列だけで受け渡し、入力の JSON は厳格に検査する。** 入口が登録する関数は、設定の検査（`checkSettings`）、入力の検査（`prepare`）、記事の生成（`generate`）の 3 つである。知らないメンバと重複したメンバは拒否する（[ADR-0020](0020-strict-json.md)）。結果の JSON は、拡張の側でも受理する形で検査する。API キーは `generate` の引数にだけ入り、Go の側で受け取った直後に `secret.Secret` にする（[ADR-0031](0031-secret-closure.md)）。境界を 1 つの形に固定し、どちらの側でも形に合わない値を補正して使わないためである。関数と JSON の形は 0090 設計書の「入口が JavaScript に登録する関数」に、検査の規則は同じ設計書の「信頼できない入力の境界（要件書 AC-18）」にある。
-   **WASM の入口は、プロセスの環境を読まず、LLM の設定を `internal/config` の拡張の入口用の関数で作る。** その関数には、`ANTHROPIC_API_KEY` と `ANTHROPIC_WORKSPACE_ID` の 2 つの名前だけに値を返す、名前を引く関数を渡す。ほかの名前は未設定になるので、3 つの工程は本番の構成になる（[ADR-0004](0004-production-profile.md)）。値の検査は CLI と同じ関数で行う（[ADR-0003](0003-llm-config-contract.md)）。拡張を本番の構成に固定し、設定の規則を CLI と 1 つにするためである。規則は 0090 設計書の「設定の読み込み」にある。
-   **プロンプトのテンプレートは、埋め込みのものだけを使う。** WASM の入口は上書きのファイルを読まない（[ADR-0034](0034-template-allowlist.md)）。規則は 0090 設計書の「共有する処理の範囲」にある。
-   **`make ext-build` が Go を呼んで WASM の入口をビルドし、Go のツールチェーンの `wasm_exec.js` とともに `extension/dist/` に置く。** Go の手順（`make test`・`make lint` など）は、今までどおり拡張のディレクトリを含まず、Node.js を呼ばない（[ADR-0026](0026-extension-platform.md)）。拡張の成果物に、ビルドした時点の Go のコードを入れるためである。規則は 0090 設計書の「ビルドと成果物」にある。
-   **成果物の検査は、`wasm_exec.js` と WASM の入口を、本リポジトリで書いたコードだけにする規則の例外として宣言する。** `wasm_exec.js` は、`go.mod` が定める Go の版のツールチェーンのものと中身が一致することを検査する。WASM は本リポジトリの Go のコードからビルドする。Go の WASM を読み込むのに `wasm_exec.js` が要り、ほかのコードが成果物に入る経路を今と同じく閉じるためである。規則は 0090 設計書の「ビルドと成果物」と「方針の例外」にある。
-   **拡張のページの CSP に `'wasm-unsafe-eval'` を加える。** WASM を読み込むのに要るためである。許すのは WASM のコンパイルだけで、JavaScript の `eval` とインラインのスクリプトは今と同じく許さない。CSP の全体は [ADR-0010](0010-extension-stores-nothing.md) が、値は 0090 設計書の「CSP」にある。
-   **`make wasm-check` は、WASM の入口とその依存を `GOOS=js GOARCH=wasm` でビルドできることを、恒久の検査として確かめる。** WASM の入口がビルドできなくなる変更を、拡張のビルドを待たずに Go の CI で止めるためである。規則は [ADR-0019](0019-pipeline-component-interfaces.md) と 0090 設計書の「ビルドと成果物」にある。
-   **WASM のサイズと読み込みの時間は受け入れる。** 読み込みは記事の生成 1 回に 1 度で、生成の所要時間に比べて小さいためである（背景）。規則は 0090 設計書の「設計の際に決めたこと（2026-11-12）」にある。

この決定は、まだコードと仕様の文書（[package_reference.md](../conventions/package_reference.md)・[development.md](../guides/development.md)・[CLAUDE.md](../../../CLAUDE.md)・[security.md](../security.md)）に反映していない。WASM の入口、`internal/extjob`、拡張のビルドの変更はまだない。この差分は [#773](https://github.com/isseis/yt2column/issues/773) の実装で解消する。

## 採らなかった案

-   **記事を作る処理を TypeScript に一本化する。** 字幕の解析、メンションの無効化、3 つの工程、料金の概算を TypeScript で書き直すことになり、CLI が Go の同じ規則を持ち続けるので、規則が 2 つになる（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）。
-   **`make wasm-check` を外し、拡張のビルドだけで WASM のビルドを確かめる。** WASM の入口をビルドできなくなる変更が、拡張のビルドを走らせるまで分からない（0090 設計書「ビルドと成果物」）。

## 結果

-   拡張のビルドに Go が要る。CI の拡張の手順は Go を用意する。
-   拡張の成果物は、約 17 MB の WASM を含む。
-   拡張のページの CSP は、MV3 の既定より `'wasm-unsafe-eval'` の分だけ広い。記事を描画するビューアにもこの CSP が当たるので、記事からスクリプトを動かさない守り（[ADR-0017](0017-viewer-markdown-rendering.md)）は今までどおり要る。
-   WASM の入口が依存するパッケージに、`js` のビルドで使えない OS に固有の API を足す変更は、`make wasm-check` で止まる。そうした処理は、`js` 以外のビルドのファイルに分けるか、別のパッケージに置く。
-   `js` のビルドだけの振る舞い（リダイレクトに従わない指定、ブラウザから呼ぶためのヘッダ）は、`js` のビルドのファイルに置く。ネイティブのビルドのテストでは確かめられないので、ビルドと手動の確認で確かめる。
-   `syscall/js` に触れる登録の部分は、Go の手順が Node.js を呼ばないので、Go のテストでは実行しない。JSON の解析と組み立ては `js` に依存しない関数に置き、ネイティブのビルドでテストする。

## 経緯

- 2026-11-12 記事を作る処理を Go のコアの WASM で CLI と拡張が共有し、拡張のビルドが WASM と `wasm_exec.js` を成果物に入れ、CSP に `'wasm-unsafe-eval'` を加え、`make wasm-check` を恒久の検査にすることにした（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）
