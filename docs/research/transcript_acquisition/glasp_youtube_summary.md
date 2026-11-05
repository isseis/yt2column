# YouTube Summary with ChatGPT & Claude（Glasp）

## 調査の対象と時点

| 項目 | 値 |
|---|---|
| 名前 | YouTube Summary with ChatGPT & Claude（Chrome Web Store での名前は「YouTube Summary & ChatGPT by Glasp」） |
| 提供者 | Glasp |
| 拡張 ID | `cdjifpfganmhoojfclednjdnnpooaojb` |
| 版 | 2.4.1 |
| 調査日 | 2026-11-05 |
| 入手の方法 | Chrome Web Store の配布元から CRX（Chrome 拡張のパッケージ）をダウンロードして展開し、JavaScript を読んだ。コードは実行していない |
| 利用者数 | 確かめていない |

配布されているコードは、Vite とその Chrome 拡張用のプラグイン CRXJS でバンドルし、ミニファイしたものである。関数の中の名前は短い名前に置き換わっているが、文字列リテラルとプロパティ名は残っており、難読化まではされていない。以下に挙げる関数名は、この短い名前である。

この拡張の初期の版のソースは、GitHub の [kazuki-sf/YouTube_Summary_with_ChatGPT](https://github.com/kazuki-sf/YouTube_Summary_with_ChatGPT) で公開されている（最後のコミットは 2023-01-11）。その版は、動画ページの HTML から字幕トラックの一覧を切り出し、`baseUrl` を PO トークン（YouTube のボット対策の仕組みが出力する値）なしで取得するだけだった（`src/contentscript/transcript.js`）。現在の版は、この取得処理を大きく拡張している。

## 構成と権限

-   `permissions` は `storage` だけで、`host_permissions` はない。
-   コンテンツスクリプト（`youtube-helper`）を、`https://www.youtube.com/*` に常に注入する。YouTube への `fetch` は、このスクリプトから同一オリジンで行う。
-   取得の処理の本体は `assets/summary-panel-event-*.js` にあり、`Qb` → `Y_` → `YF` の順に呼ばれる。

## 取得の流れ

### 動画ページの HTML の取り直し（`Qb`・`fF`）

`fetch("https://www.youtube.com/watch?v=<動画ID>")` で、ブラウザが読み込み済みのページとは別に、動画ページの HTML を取得し直す。HTML に埋め込まれた `ytInitialPlayerResponse` を、正規表現で位置を探し、波かっこの対応を数えて切り出し、JSON として読む（`m_`・`b_`）。そこから、字幕トラックの一覧 `captions.playerCaptionsTracklistRenderer.captionTracks` と、文字起こしパネル用の `getTranscriptEndpoint.params` を取り出す。

### 経路を順に試す（`YF`）

次の経路を順に試し、文字起こしが取れた時点で終える。

| 順 | 経路（`source` の値） | 方法 |
|---|---|---|
| 1 | `get_transcript` の API（`page`） | `POST https://www.youtube.com/youtubei/v1/get_transcript?prettyPrint=false` に、`{context:{client:{clientName:"WEB", clientVersion:"2.<直近 30 日のうちランダムな日付>.00.00"}}, params}` を送る（`WF`・`AF`）。応答の `transcriptSegmentListRenderer.initialSegments[].transcriptSegmentRenderer` から、`startMs`・`endMs`・`snippet` を読む。文字起こしパネルが表示するのと同じデータである |
| 2 | timedtext（字幕を配信する `/api/timedtext`）、PO トークン付き（`page`） | `captionTracks[].baseUrl` に `&pot=<トークン>&c=WEB` を付けて取得し、XML を解析する（`G0` の第 3 引数が `true`） |
| 3 | timedtext、PO トークンなし（`page`） | `baseUrl` をそのまま取得する（`G0` の第 3 引数が `false`） |
| 4 | 提供者の共有キャッシュ（`shared-cache`） | バックグラウンドを経由して、動画 ID と字幕トラックの識別子 `vssId` で提供者のサーバ（Firestore）に問い合わせる（`KF` が送るメッセージ `get_yt_scripts`）。他の利用者が取得して保存した文字起こしを受け取る |
| 5 | 文字起こしパネルの DOM（`panel`） | 「文字起こしを表示」ボタンを `click()` してパネルを開き、要素から時刻と本文を読む（`VF`）。古い DOM 構造の `ytd-transcript-segment-renderer` と、新しい DOM 構造の `transcript-segment-view-model` の両方に対応する |

言語を指定しない場合は、経路 2・3 を、一覧の先頭のトラックに対しても試す（`listed-first`）。経路 5 は、呼び出し側がフォールバックの利用を許可したときだけ試す。

## PO トークンの扱い

PO トークンは、プレーヤー自身のリクエストから拾う（`bF`）。

1.  `M_` が `performance.getEntriesByType("resource")` で、ページがすでに行ったリクエストの URL を調べる。そのうち `/api/timedtext?` を含み、クエリの `v` が対象の動画 ID と一致するものから、`pot` の値を取り出す。
2.  見つからなければ、`CF` がプレーヤーの字幕ボタン（`button.ytp-subtitles-button`）を 2 回 `click()` する。1 回目で字幕を表示させてプレーヤーに timedtext を取得させ、2 回目で表示を元に戻す。その後、最大 2 秒間、50 ms ごとに `M_` を繰り返す。
3.  得たトークンは、動画ごとにキャッシュする。

## 動画との結びつき

-   動画 ID を指定してページの HTML を取り直すので、YouTube の中で別の動画へ移った後でも、前の動画の字幕トラックを使わない。
-   PO トークンは、timedtext のリクエストの `v` が動画 ID と一致するものからだけ拾う。
-   パネルの DOM を読む経路では、読む前と読んだ後に、ページの URL の動画 ID が対象と一致することを確かめる。

## ページの操作と外部への送信

-   **ページの操作：** PO トークンを得るために字幕ボタンを押し、パネルの DOM を読むために「文字起こしを表示」ボタンを押す。
-   **外部への送信：** 取得した文字起こしを、提供者のサーバに保存する（バックグラウンドのメッセージ `save_yt_transcript_doc`）。保存した文字起こしは、経路 4 で他の利用者に返される。

## 本プロジェクトから見た評価

-   **動画との結びつき：** パネルの DOM を読む案を取り下げた理由は、パネルの中身と動画を結びつける信号がないことだった（[ADR-0007](../../dev/adr/0007-manual-transcript-copy.md)）。このツールは、動画 ID によるページの取り直しと URL の確かめ直しで、この問題に対処している。
-   **ページの操作：** 字幕ボタンや「文字起こしを表示」ボタンを押すので、ページを操作しないという ADR-0007 の決定と相容れない。
-   **利用規約との関係：** このツールは、クライアントのバージョンをなりすまして Innertube（YouTube の内部 API）を呼び、プレーヤーが使った PO トークンを拾って付け直す。[YouTube 利用規約](https://www.youtube.com/static?template=terms)の「許可と制限事項」に抵触しうる。
-   **壊れやすさ：** Innertube の内部 API、HTML からの文字列の切り出し、CSS セレクタに依存する。5 つの経路を重ねて持つことで、いずれかが壊れても動き続けるようにしている。このように自前で追従し続ける方式は、YouTube の非公式な仕様への追従を、活発に保守されているツールに任せるという本プロジェクトの方針とは逆である。
-   **取り入れうる点（取得経路）：** 経路 1 は、主経路で受け取っている文字列と同じ中身を、時刻をミリ秒の整数で持つ構造化された形で返す。そのため、記事の品質は変わらないと見込まれる（未検証）。
-   **取り入れうる点（PO トークン）：** 利用者が自分で字幕を表示しておけば、ボタンを押さずに `M_` の方法だけで PO トークンを拾える可能性がある（未検証）。

## 出典

-   配布されている CRX（版 2.4.1）の `manifest.json` と `assets/summary-panel-event-*.js`
    -   `Qb`・`fF`・`m_`・`b_`：動画ページの HTML の取り直しと解析
    -   `Y_`・`YF`：経路の順序
    -   `WF`・`AF`：`get_transcript` の API
    -   `G0`：timedtext
    -   `KF`：共有キャッシュ
    -   `VF`：パネルの DOM
    -   `bF`・`M_`・`CF`：PO トークン
-   配布されている CRX の `assets/index.ts-*.js`：バックグラウンドのメッセージ `get_yt_scripts`・`save_yt_transcript_doc` の処理
-   [kazuki-sf/YouTube_Summary_with_ChatGPT](https://github.com/kazuki-sf/YouTube_Summary_with_ChatGPT) の `src/contentscript/transcript.js`：初期の版
