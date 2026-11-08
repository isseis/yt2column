# ADR-0026: ブラウザ拡張は TypeScript の Manifest V3 の拡張とし、バンドラを使わず、manifest の key で ID を固定し、Chrome と Brave だけを対象にする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

`yt-dlp` による字幕の取得は、YouTube から HTTP 429 を繰り返し受けることがある。そこで、利用者が文字起こしパネルで字幕を選び、ブラウザ拡張がそれをサーバへ送る形にした（[0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §1）。リポジトリは Go のプロジェクトで、拡張の開発環境は何もなかった。拡張を書く言語、ビルドの道具、対象のブラウザを新たに決める必要があった。

拡張はストアに公開せず、利用者がビルドしたものを「パッケージ化されていない拡張機能を読み込む」で読み込む（[0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §1）。パッケージ化されていない拡張の ID は、既定では読み込んだディレクトリのパスから決まり、環境ごとに異なる（同 §6）。一方、拡張の送り先は、送り手の拡張を ID で指定する。今の送り先は Native Messaging のホストで、ホストの manifest の `allowed_origins` が ID を指定する。そのため、ID はどの環境でも同じでなければならない。

拡張の実行時に読み込むコードは、本リポジトリで書いたものだけとする。npm の依存パッケージは開発用（ビルド・型検査・lint・テスト）に限る。これは、Go のモジュールについての CLAUDE.md「Dependencies」の考え方を拡張にも当てはめたものである（[0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) F-001）。

## 決定

-   **ブラウザ拡張は TypeScript で書き、Manifest V3 の拡張とする。** ソース・設定・テストは `extension/` に置く。型検査は `strict` を含む厳格な設定で行い、型検査のエラーがあればビルドは失敗する。`src/core/` は、`chrome` の型も DOM の型も含まない設定で別に型検査する。バックグラウンドの処理は、モジュールとして読む Service Worker で行う。Go の手順は `extension/` を対象にせず、Node.js を要しない。
-   **バンドラを使わず、`tsc` だけでビルドする。** ビルドは、`tsc` が `src/` の各 `.ts` を 1 対 1 で `dist/` の `.js` に変換し、`static/` のファイルを複製する。その後、`dist/` がこの 2 つの和にちょうど一致し、モジュールの指定がすべて `dist/` の中を指す相対パスで、動的な `import()` がないことを検査する。npm の依存パッケージはすべて開発用で、`dist/` に入るのは本リポジトリで書いたコードだけである。
-   **拡張の ID を manifest の `key` で固定する。** `key` には RSA 2048 ビットの公開鍵（DER の SubjectPublicKeyInfo を base64 にしたもの）を置く。ID はこの公開鍵から決まり、読み込むマシンやディレクトリによらず `clfmbbcdpnjcefbdihdoahomaifbabkk` になる。鍵の組を作った直後に秘密鍵を削除し、リポジトリに含めない。`key` は公開情報なので、ID は秘密ではない。
-   **対象のブラウザは Chrome と Brave だけにする。** Native Messaging のホストの manifest を登録・削除するコマンドが受け付けるブラウザは、`chrome` と `brave` だけである。登録先は、それぞれのブラウザの決まった置き場所だけで、ほかのブラウザ向けに置き場所を指定する手段は持たない。

拡張が要求する Chrome の最小の版は [ADR-0030](0030-minimum-chrome-version.md) が、拡張の表示の言語は [ADR-0029](0029-extension-ui-languages.md) が定める。拡張の権限と、何も保存しないことは [ADR-0010](0010-extension-stores-nothing.md) が、起動の経路は [ADR-0008](0008-launch-path.md) が定める。ホストを起動できる拡張を、固定した ID の拡張だけにすること（`allowed_origins`）は、ホストの中継として [ADR-0005](0005-server-architecture.md) が、脅威モデルとして [ADR-0006](0006-threat-model.md) が定める。

この決定の仕様の正本は、[CLAUDE.md](../../../CLAUDE.md) の「Extension Commands」と「Dependencies」、[security.md](../security.md) の §8.1「ブラウザ拡張（#110）」、対象のブラウザについては [internal/nmhost/manifest.go](../../../internal/nmhost/manifest.go) である。

## 採らなかった案

-   **バンドラ（esbuild・Vite など）を使う。** 実行時に読み込む依存パッケージがないので、まとめる対象がなく、`tsc` の 1 対 1 の出力で足りる（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.8）。また、esbuild などはインストール時にバイナリを取得する。依存パッケージのインストール時のスクリプトを実行しない方針に合わないことも、使わない理由の 1 つである（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §5.2）。
-   **ストアに公開する、または `.crx` にパッケージ化する。** 理由は記録されていない（[0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §1 は「ストアには公開しない」と決め、§2.3 はストアへの公開・`.crx` へのパッケージ化・自動更新をスコープ外とするだけである）。
-   **Chrome と Brave 以外のブラウザ（Firefox・Safari・Edge など）も対象にする。** [0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §2.3 は、Edge は Chromium 系なので動作する可能性があるが確認しない、と述べる。ほかの理由は記録されていない。Firefox は Manifest V3 の Service Worker の扱いが異なるので、対応するにはエントリポイントと manifest を分ける必要がある（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §9）。
-   **ホストの manifest の置き場所を任意に指定する手段（`--manifest-dir`）を設け、Chrome と Brave 以外の Chromium 系のブラウザ（Beta・Canary・Chromium・Edge など）でもホストを使えるようにする。** 手動で確認していないブラウザ向けの手段を、確かめないまま提供しないためである（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.6）。

## 結果

-   拡張の開発には Node.js と npm が要る。版は `extension/.node-version` と `package.json` の `packageManager` で固定し、`ext-` のターゲットは版が合わなければ動かない。Go の `make test`・`make lint`・`make build` は Node.js を要しない。
-   成果物に依存パッケージのコードが入る経路は、`dist/` の余分なファイル、シンボリックリンク、相対パスでないモジュールの指定だけで、どれもビルドの検査で失敗する（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.9）。実行時に npm のパッケージを使うには、この決定を変える必要がある。
-   Service Worker を含むすべてのモジュールは、相対パスの静的な `import` だけで読む。拡張の Service Worker は動的な `import()` を受け付けないので、ソースでも使わない（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.9）。
-   秘密鍵を削除したので、同じ ID の `.crx` は後から作れない。`.crx` が要るときは鍵を作り直し、ID も変わる（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.1）。ID を変えると、ホストの `allowed_origins` と、文書に書いた ID も変えることになる。
-   `key` は公開しているので、誰でも同じ `key` で同じ ID の拡張を読み込める。ID は送り元の認証にならない（[ADR-0006](0006-threat-model.md)）。
-   ID は `key` だけで決まるので、拡張を一度削除して読み込み直しても ID は変わらず、ホストを登録し直す必要はない（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1）。
-   macOS では、Brave は Chrome の置き場所からホストの manifest を読むので、Chrome と Brave は 1 つの manifest のファイルを共有する。どちらのブラウザを指定して登録を削除しても、両方のブラウザからホストが外れる（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.6）。
-   Chrome と Brave 以外の Chromium 系のブラウザで拡張が動く可能性はあるが、ホストを登録する手段がなく、手動の確認もしない。

## 経緯

- 2026-10-07 ブラウザ拡張を TypeScript で書く Manifest V3 の拡張とし、対象のブラウザを Chrome と Brave にした。ストアには公開せず、パッケージ化されていない拡張として読み込み、拡張の ID を固定することにした（[0007 要件定義書](../../tasks/0007_browser_extension_skeleton/01_requirements.md) §1「概要」・F-003）
- 2026-10-07 拡張の ID を manifest の `key` に置いた公開鍵で固定し、秘密鍵を削除した。バンドラを使わず、`tsc` が `src/` の各ファイルを 1 対 1 で変換したものと `static/` のファイルだけを成果物にし、ビルドの最後に検査することにした（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §1.1・§3.1・§3.8・§3.9）
- 2026-10-19 Native Messaging のホストの manifest を登録するコマンドが受け付けるブラウザを Chrome と Brave に限り、ほかのブラウザ向けに置き場所を指定する手段を設けなかった（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.6）
- 2026-11-01 Service Worker とビューアを加えるときも、バンドラを加えず、`tsc` が出力する ES のモジュールを Service Worker が読む形にした（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md) §3.15）
