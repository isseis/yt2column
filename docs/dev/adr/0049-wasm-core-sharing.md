# ADR-0049: 記事を作る処理は Go のコアを WebAssembly にして CLI と拡張が共有し、拡張との境界を JSON の文字列にする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

記事は拡張だけで作り、CLI は開発用の経路として残す。CLI と拡張は、記事を作る処理を共有する（[ADR-0048](0048-extension-only-generation.md)）。記事を作る処理（入力の検査、字幕の解析、3 つの工程、完了の時点の検査、メンションの無効化、保存するファイルの中身、料金の概算）は、今は Go で書かれている。CLI は今後も Go のまま使う。

同じ規則を Go と TypeScript の両方に書くと、片方だけを直す誤りが起きる（[0090 要件定義書](../../tasks/0090_extension_only_architecture/01_requirements.md) §4.5「保守性 (Maintainability)」）。

Go のパイプラインを WASM にしたものは、拡張のページから Claude API を呼んで記事を完成させた。WASM は 17,300,194 バイト（`gzip -9` で 4,138,823 バイト）で、ページの読み込みの開始から入口を呼べるようになるまで、Chrome で 62.4〜171.2 ms、Brave で 49.9〜206.5 ms だった。WASM を読み込むには、拡張のページの CSP に `'wasm-unsafe-eval'` が要った（[調査記録](../../research/wasm_pipeline_poc/README.md)「3 つの問いへの答え」「測定 2：読み込みと CSP、サイズと読み込みの時間」）。本番の構成での記事の生成は 95〜151 秒かかった（同「測定 6：本番の構成（Chrome、`83ef12bd`、背面のまま）」）。専用の Web Worker の中でも、WASM は 30.5〜68.3 ms で読み込め、Chrome と Brave で記事を完成させた（同「測定 7：一度も前面にならなかった背面のタブでの記事の生成（Worker）」「測定 8：Worker からの送信の守り（プローブ、ダミーのキー）」）。

## 決定

-   **記事を作る処理は、Go のコアを `GOOS=js GOARCH=wasm` でビルドした記事生成モジュール（`cmd/yt2column-wasm`）で、CLI と拡張が共有する。** 入力の検査、字幕の解析、3 つの工程、完了の時点の検査、メンションの無効化、保存するファイルの中身、料金の概算、API キーとワークスペース ID の検査は Go のコードで行い、拡張の TypeScript はこれらの規則を持たない。規則を Go の 1 か所に保つためである。共有する範囲は [0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md) の「共有する処理の範囲」に、拡張のジョブ 1 回分の処理を持つパッケージ（`internal/extjob`）は同じ設計書の「Go のパッケージの構成」にある。
-   **記事生成モジュールと拡張のページの間は、JSON の文字列だけで受け渡し、入力の JSON は厳格に検査する。** 記事生成モジュールが公開する関数は、設定の検査（`checkSettings`）、入力の検査（`prepare`）、記事の生成（`generate`）、ビルドの情報（`buildInfo`）である。JSON の解析と組み立て、ジョブ 1 回分の処理は `internal/extjob` に置き、`cmd/yt2column-wasm` は `syscall/js` での登録だけを持つ。知らないメンバと重複したメンバは拒否する（[ADR-0020](0020-strict-json.md)）。結果の JSON は、拡張の側でも受理する形で検査する。API キーは `checkSettings` と `generate` の引数にだけ入り、Go の側で受け取った直後に `secret.Secret` にする（[ADR-0031](0031-secret-closure.md)）。境界を 1 つの形に固定し、どちらの側でも形に合わない値を補正して使わないためである。解析を `internal/extjob` に置くのは、ネイティブのビルドでテストできるようにするためである。関数と JSON の形は 0090 設計書の「記事生成モジュールが JavaScript に公開する関数」に、検査の規則は同じ設計書の「信頼できない入力の境界（要件書 AC-18）」にある。
-   **`generate` は、`prepare` が Worker の Go のメモリに持った入力だけを生成する。** `generate` は入力を受け取らず、予約に使った `inputDigest` だけを受け取り、持っている入力と違えば `internal` の失敗にする。ジョブの記録が、予約した入力と別の入力の記事を持たないためである。規則は 0090 設計書の「記事生成モジュールが JavaScript に公開する関数」にある。
-   **記事生成モジュールは、Claude のクライアントを `claude.New` で直接作り、`internal/llm/provider` を使わない。** `provider` は DeepSeek と OpenAI のアダプタを import し、WASM でビルドできないためである。プロバイダを足すときは、そのアダプタを直接組み立てるか、`provider` を WASM でビルドできるようにする。規則は 0090 設計書の「Go のパッケージの構成」と「プロバイダを足すときに変える箇所（要件書 AC-04）」にある。
-   **記事生成モジュールは、プロセスの環境を読まず、LLM の設定を `config.LoadExtension` で作る。** その関数には、`ANTHROPIC_API_KEY` と `ANTHROPIC_WORKSPACE_ID` の 2 つの名前だけに値を返す、名前を引く関数を渡す。ほかの名前は未設定になるので、3 つの工程は本番の構成になる（[ADR-0004](0004-production-profile.md)）。値の検査は CLI と同じ関数で行う（[ADR-0003](0003-llm-config-contract.md)）。拡張を本番の構成に固定し、設定の規則を CLI と 1 つにするためである。規則は 0090 設計書の「設定の読み込み」にある。
-   **プロンプトのテンプレートは、埋め込みのものだけを使う。** 記事生成モジュールは上書きのファイルを読まない（[ADR-0034](0034-template-allowlist.md)）。規則は 0090 設計書の「共有する処理の範囲」にある。
-   **ビルド、成果物、CSP、`make wasm-check` は、それぞれの ADR が定める。** 拡張のビルドが Go を呼んで記事生成モジュールと `wasm_exec.js` を成果物に入れることと、成果物の検査の例外は [ADR-0026](0026-extension-platform.md) が、CSP の全体（WASM を読み込むための `'wasm-unsafe-eval'` を含む）は [ADR-0010](0010-extension-stores-nothing.md) が、`make wasm-check` は [ADR-0019](0019-pipeline-component-interfaces.md) が定める。
-   **WASM のサイズと読み込みの時間は受け入れる。** 読み込みは記事の生成 1 回に 1 度で、生成の所要時間に比べて小さいためである（背景）。規則は 0090 設計書の「設計の際に決めたこと（2026-11-12）」にある。

この決定は、まだコードと仕様の文書（[package_reference.md](../conventions/package_reference.md)・[development.md](../guides/development.md)・[CLAUDE.md](../../../CLAUDE.md)・[security.md](../security.md)）に反映していない。記事生成モジュール、`internal/extjob`、拡張のビルドの変更はまだない。この差分は [#773](https://github.com/isseis/yt2column/issues/773) の実装で解消する。

## 採らなかった案

-   **記事を作る処理を TypeScript に一本化する。** 字幕の解析、メンションの無効化、3 つの工程、料金の概算を TypeScript で書き直すことになり、CLI が Go の同じ規則を持ち続けるので、規則が 2 つになる（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）。
-   **`make wasm-check` を外し、拡張のビルドだけで WASM のビルドを確かめる。** 記事生成モジュールをビルドできなくなる変更が、拡張のビルドを走らせるまで分からない（0090 設計書「ビルドと成果物」）。
-   **記事生成モジュールで `internal/llm/provider` を使う。** `provider` が import する DeepSeek と OpenAI のアダプタは WASM でビルドできない（0090 設計書「Go のパッケージの構成」）。
-   **`generate` が入力を受け取り、`prepare` と同じ検査をもう一度行う。** 検査は入力が正しいことを確かめるが、予約した入力と同じことは確かめない（0090 設計書「記事生成モジュールが JavaScript に公開する関数」）。

## 結果

-   拡張の成果物は、約 17 MB の WASM を含む。
-   記事を描画するビューアにも `'wasm-unsafe-eval'` を含む CSP が当たるので、記事からスクリプトを動かさない守り（[ADR-0017](0017-viewer-markdown-rendering.md)）は今までどおり要る。
-   記事生成モジュールが依存するパッケージに、`js` のビルドで使えない OS に固有の API を足す変更は、`make wasm-check` で止まる（ADR-0019）。
-   `js` のビルドだけの振る舞い（リダイレクトに従わない指定、ブラウザから呼ぶためのヘッダ）は、`js` のビルドのファイルに置く。ネイティブのビルドのテストでは確かめられないので、ビルドと手動の確認で確かめる。
-   `syscall/js` に触れる登録の部分は、Go の手順が Node.js を呼ばないので、Go のテストでは実行しない。JSON の解析と組み立ては `internal/extjob` に置き、ネイティブのビルドでテストする。

## 経緯

- 2026-11-12 記事を作る処理を Go のコアの WASM で CLI と拡張が共有し、拡張との境界を JSON の文字列にして処理を `internal/extjob` に置き、Claude のクライアントを直接作ることにした（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）
