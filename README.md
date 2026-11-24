[English](README.en.md)

開発者向けの文書: [記事の生成の詳細](docs/dev/guides/generation.md)・[CLI の使い方](docs/dev/guides/cli.md)・[開発の手順](docs/dev/guides/development.md)

# yt2column

yt2column は、YouTube の動画を、LLM で雑誌のコラム風の記事にする。主経路はブラウザ拡張だけで完結する。利用者が
YouTube の文字起こしパネルから選択した字幕を拡張が集め、拡張に入れた記事生成モジュール（Go のコードを WebAssembly にビルドしたもの）が、ビューアのタブの中で Anthropic の API を呼んで記事を生成する。API キーは拡張のオプションページで設定し、そのブラウザの中にだけ保存する。記事は拡張のビューアのタブに表示され、`.md` ファイルへの保存と Markdown のクリップボードへのコピーは、利用者がビューアで選ぶ。

- 拡張は、ブラウザのほかに何も導入しない。記事の生成のためのサーバもホストも要らない。

> **状態:** 開発の初期段階。まだリリースしていない。

## クイックスタート

入手から最初の記事の保存までの手順は、[INSTALL.md](INSTALL.md) にある。INSTALL.md は、今は日本語だけで書いている。

以前の版（localhost サーバと Native Messaging のホストを使う版）を使っていたときは、拡張を更新する前に[以前の版からの移行](#以前の版からの移行)を行う。

## 必要なもの

**主経路（ブラウザ拡張）に必要なもの:**

- Chrome 142 以降、または Chromium 142 以降の Brave
- Anthropic の API キー（[INSTALL.md](INSTALL.md#使う前に必要なもの)を参照）

## プライバシー

API キーとワークスペース ID は、このブラウザのプロファイルの `chrome.storage.local` にだけ保存する。ブラウザを再起動しても残り、ほかの端末へは同期しない。拡張が API キーを送る先は `https://api.anthropic.com` だけで、ポップアップや Service Worker は API キーの値を読まない。ただし、同じ OS のユーザーとして動くプロセスは、プロファイルのファイルを読める。

拡張は、記事の生成ごとの記録（状態、工程、完成した記事）をブラウザのメモリに置き、ディスクには書かない。ビューアの URL は、集めた URL とページのタイトルを持ち、ブラウザの履歴に残る。URL の値を細工したビューアは、どんな見出しやタイトルでも表示しうるので、見出しは記事の内容を保証しない。

## 以前の版からの移行

以前の版は、localhost サーバ（`yt2column-server`）と Native Messaging のホスト（`yt2column-nmhost`）で記事を生成していた。今の拡張は、どちらも使わない。拡張を更新する前に、次の手順で移る。

1. 動いている `yt2column-server` を止める。
2. 以前の版のホストのバイナリで、使っていた各ブラウザからホストの manifest を消す（下のコマンド）。バイナリがもうないなどで実行できないときは、下の manifest のファイルを手で消す。
3. 拡張をビルドし直して再読み込みし（[拡張のビルドと読み込み](docs/dev/guides/development.md#拡張のビルドと読み込み)を参照）、オプションページで API キーを保存する（[API キーとワークスペース ID](INSTALL.md#api-キーとワークスペース-id)を参照）。
4. 拡張だけで記事を 1 本作れたら、サーバの設定ファイル `~/.config/yt2column/config.env` を消す。このファイルは API キーの平文の写しを持つ。消すまでは、以前の版のサーバとホストに戻せる。同じディレクトリに `.config.env.tmp-` で始まる名前のファイルがあれば、それも消す。`config init` が途中で止められたときに残る一時ファイルで、設定ファイルと同じ中身を指すことがある。

```sh
./build/yt2column-nmhost uninstall chrome
./build/yt2column-nmhost uninstall brave
```

手で消すときの manifest のファイル:

- macOS の Chrome と Brave: `$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts/io.github.isseis.yt2column.json`
- ほかの Unix の Chrome: `$HOME/.config/google-chrome/NativeMessagingHosts/io.github.isseis.yt2column.json`
- ほかの Unix の Brave: `$HOME/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts/io.github.isseis.yt2column.json`

以前の版がブラウザに保存した共有トークンとポート（`sharedToken`・`serverPort`）が残っていても、今の拡張はそれを読まず、消しもしない。消したいときは、`chrome://extensions/`（Brave: `brave://extensions/`）で拡張を一度削除してから、`extension/dist` をもう一度読み込む。拡張を削除すると、保存した API キーも消えるので、もう一度保存する。
