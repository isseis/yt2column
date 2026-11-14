# ADR-0026: ブラウザ拡張は TypeScript の Manifest V3 の拡張とし、バンドラを使わず、manifest の key で ID を固定し、Chrome と Brave だけを対象にする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

ブラウザ拡張は、利用者が YouTube の文字起こしパネルで選んだ字幕から、記事を作る（[ADR-0048](0048-extension-only-generation.md)）。

開発中は、拡張をストアに公開せず、利用者がビルドしたものを「パッケージ化されていない拡張機能を読み込む」で読み込む（[0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §1）。拡張は Chrome Web Store で公開できる形にする（ADR-0048）。パッケージ化されていない拡張の ID は、既定では読み込んだディレクトリのパスから決まり、環境ごとに異なる（同 §6）。拡張が `chrome.storage.local` に保存する値（API キーとワークスペース ID）は、拡張の ID ごとに分かれるので、ID が変わると保存した値は引き継がれない（[ADR-0010](0010-extension-stores-nothing.md)）。

拡張の実行時に読み込むコードは、本リポジトリで書いたものだけとする。npm の依存パッケージは開発用（ビルド・型検査・lint・テスト）に限る。これは、Go のモジュールについての CLAUDE.md「Dependencies」の考え方を拡張にも当てはめたものである（[0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) F-001）。記事を作る処理は Go のコアを WebAssembly にしたもので共有し、その読み込みには Go のツールチェーンの `wasm_exec.js` が要る（[ADR-0049](0049-wasm-core-sharing.md)）。

## 決定

-   **ブラウザ拡張は TypeScript で書き、Manifest V3 の拡張とする。** ソース・設定・テストは `extension/` に置く。バックグラウンドの処理は、モジュールとして読む Service Worker で行う。理由は記録されていない。規則は [0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §1.1「設計原則」・§3.8「開発環境」と [CLAUDE.md](../../../CLAUDE.md) の「Extension Commands」にある。
-   **型検査は `strict` を含む厳格な設定で行い、`src/core/` は `chrome` の型も DOM の型も含まない設定で別に型検査する。** ブラウザに依存しない部分を分けるためである。規則は [0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §1.1・§3.8 にある。
-   **Go の手順は拡張のディレクトリを含まず、Node.js を呼ばない。拡張のビルド（`make ext-build`）は Go を呼んで記事生成モジュールをビルドする。** Go の手順の結果を変えないためである。拡張のビルドが Go を呼ぶのは、成果物に記事生成モジュールを入れるためである（ADR-0049）。規則は [0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §1.1・§3.10「既存のファイルの変更と、Go の手順への影響」と、[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md) の「ビルドと成果物」にある。
-   **バンドラを使わず、`tsc` だけで TypeScript をビルドする。** ビルドは `tsc` の出力、`static/` の写し、記事生成モジュール、Go のツールチェーンの `wasm_exec.js` だけを成果物とし、ほかのものが成果物に入らないことを検査する。`wasm_exec.js` は本リポジトリで書いたコードでないので、成果物の検査の例外として宣言し、`go.mod` が定める Go の版のものと中身が一致することを検査する。実行時に読み込む npm のパッケージがなく、まとめる対象がないためである。`wasm_exec.js` を例外にするのは、Go の WASM を読み込むのに要るためである。規則は [0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.8・§3.9「成果物の検査」と、0090 設計書の「ビルドと成果物」「方針の例外」にある。
-   **拡張の ID を manifest の `key` で固定し、Web Store の項目を作るときに Web Store の公開鍵に置き換える。** 今の `key` は RSA 2048 ビットの公開鍵で、ID は `clfmbbcdpnjcefbdihdoahomaifbabkk` になる。ID を固定するのは、開発用に読み込んだ拡張の ID を、読み込んだディレクトリやワークツリーによらず同じにし、保存した API キーを引き継ぐためである。Web Store の公開鍵に置き換えるのは、開発用の拡張と Web Store から導入した拡張の ID を同じにするためである。置き換えは、Web Store の申請を扱う issue の作業にする。鍵の組を作った直後に秘密鍵を削除し、リポジトリに含めない。`key` は公開情報なので、ID は秘密ではない。規則は [0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.1「manifest」と、0090 設計書の「ID と対象のブラウザ」「構成を縛る外部の条件（要件書 AC-20）」にある。
-   **成果物の WASM は、公開用の記事生成モジュール（`cmd/yt2column-wasm`）からビルドしたものに限る。開発用のビルド（`make ext-build-dev`）は、manifest に `key` を持たず、名前に開発用の印を付けて、別の場所（`extension/dist-dev/`）に置く。** 成果物の検査は、`go version -m` が示す main のパッケージのパスで、WASM の出どころを確かめる。開発用のビルドの ID を公開用と別にし、開発用のビルドで保存したダミーの値が本物の API キーを上書きしないためであり、fake の LLM を含む WASM が公開用の成果物に入らないためである（[ADR-0045](0045-dev-fake-llm.md)）。規則は 0090 設計書の「料金をかけずに拡張を動かす方法」にある。
-   **対象のブラウザは Chrome と Brave だけにする。** 手動で確認していないブラウザを、確かめないまま対象にしないためである。Brave は、Chrome Web Store の Manifest V3 の拡張が Chrome と同じに動くとしている。規則は [0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §1・§2.3 と、0090 設計書の「ID と対象のブラウザ」にある。

拡張が要求する Chrome の最小の版は [ADR-0030](0030-minimum-chrome-version.md) が、拡張の表示の言語は [ADR-0029](0029-extension-ui-languages.md) が定める。拡張の権限と、拡張が保存するものは [ADR-0010](0010-extension-stores-nothing.md) が、起動の経路は [ADR-0008](0008-launch-path.md) が、脅威モデルは [ADR-0006](0006-threat-model.md) が定める。

この決定のうち、拡張のビルドが Go を呼んで記事生成モジュールと `wasm_exec.js` を成果物に入れること、成果物の検査の例外、開発用のビルドは、まだコードと [CLAUDE.md](../../../CLAUDE.md) に反映していない。今の拡張のビルドは Go を呼ばない。この差分は [#773](https://github.com/isseis/yt2column/issues/773) の実装で解消する。

## 採らなかった案

-   **バンドラ（esbuild・Vite など）を使う。** 実行時に読み込む依存パッケージがないので、まとめる対象がなく、`tsc` の 1 対 1 の出力で足りる（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.8）。また、esbuild などはインストール時にバイナリを取得する。依存パッケージのインストール時のスクリプトを実行しない方針に合わないことも、使わない理由の 1 つである（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §5.2）。
-   **Chrome と Brave 以外のブラウザ（Firefox・Safari・Edge など）も対象にする。** [0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §2.3 は、Edge は Chromium 系なので動作する可能性があるが確認しない、と述べる。ほかの理由は記録されていない。Firefox は Manifest V3 の Service Worker の扱いが異なるので、対応するにはエントリポイントと manifest を分ける必要がある（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §9）。
-   **ホストの manifest の置き場所を任意に指定する手段（`--manifest-dir`）を設け、Chrome と Brave 以外の Chromium 系のブラウザ（Beta・Canary・Chromium・Edge など）でもホストを使えるようにする。** 手動で確認していないブラウザ向けの手段を、確かめないまま提供しないためである（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.6）。
-   **manifest の `key` を除く。** 読み込んだディレクトリごとに ID が変わり、ワークツリーを替えるたびに API キーを入れ直すことになる（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）。

## 結果

-   拡張の開発には Node.js と npm が要る。版は `extension/.node-version` と `package.json` の `packageManager` で固定し、`ext-` のターゲットは版が合わなければ動かない。Go の `make test`・`make lint`・`make build` は Node.js を要しない。拡張のビルドは Go も要する。
-   成果物に依存パッケージのコードが入る経路は、`dist/` の余分なファイル、シンボリックリンク、相対パスでないモジュールの指定だけで、どれもビルドの検査で失敗する（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.9）。実行時に npm のパッケージを使うには、この決定を変える必要がある。本リポジトリで書いていないコードで成果物に入るのは、`wasm_exec.js` だけである。
-   Service Worker を含むすべてのモジュールは、相対パスの静的な `import` だけで読む。拡張の Service Worker は動的な `import()` を受け付けないので、ソースでも使わない（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.9）。
-   秘密鍵を削除したので、同じ ID の `.crx` は後から作れない。`.crx` が要るときは鍵を作り直し、ID も変わる（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.1）。ID を変えると、`chrome.storage.local` に保存した値は引き継がれず、文書に書いた ID も変えることになる。
-   `key` は公開しているので、誰でも同じ `key` で同じ ID の拡張を読み込める。ID は送り元の認証にならない（[ADR-0006](0006-threat-model.md)）。
-   ID は `key` だけで決まるので、拡張を一度削除して読み込み直しても ID は変わらない。ただし、削除すると `chrome.storage.local` に保存した値は消える。
-   Web Store が、上げたパッケージの `key` をどう扱うかは、公式の文書に書かれていない。Web Store の申請の issue で確かめる（0090 設計書「構成を縛る外部の条件（要件書 AC-20）」）。
-   Chrome と Brave 以外の Chromium 系のブラウザで拡張が動く可能性はあるが、手動の確認はしない。

## 経緯

- 2026-10-07 ブラウザ拡張を TypeScript で書く Manifest V3 の拡張とし、対象のブラウザを Chrome と Brave にした。ストアには公開せず、パッケージ化されていない拡張として読み込み、拡張の ID を固定することにした（[0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §1「概要」・F-003）
- 2026-10-07 拡張の ID を manifest の `key` に置いた公開鍵で固定し、秘密鍵を削除した。バンドラを使わず、`tsc` が `src/` の各ファイルを 1 対 1 で変換したものと `static/` のファイルだけを成果物にし、ビルドの最後に検査することにした（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §1.1・§3.1・§3.8・§3.9）
- 2026-10-19 Native Messaging のホストの manifest を登録するコマンドが受け付けるブラウザを Chrome と Brave に限り、ほかのブラウザ向けに置き場所を指定する手段を設けなかった（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.6）
- 2026-11-01 Service Worker とビューアを加えるときも、バンドラを加えず、`tsc` が出力する ES のモジュールを Service Worker が読む形にした（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md) §3.15）
- 2026-11-12 拡張のビルドが Go を呼んで記事生成モジュールと `wasm_exec.js` を成果物に入れ、`wasm_exec.js` を成果物の検査の例外として宣言することにした。`key` を残す理由を、保存した値の持ち主の ID を固定することに改め、Web Store の項目を作るときに Web Store の公開鍵に置き換えることにした。対象のブラウザを Chrome と Brave に限る理由から、ホストの manifest の登録を除いた（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）
