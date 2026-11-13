# ADR-0030: 拡張の minimum_chrome_version は決める日の 1 年前の安定版とし、今は 142 にする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

拡張は Chrome と Brave で動く（[ADR-0026](0026-extension-platform.md)）。manifest の `minimum_chrome_version` は、拡張を読み込める Chrome の最も古い版を宣言する。拡張が頼る API や振る舞いのうち最も新しいものが、この版で使えなければならない。

Service Worker（SW）は、生成の開始を受けてから、ビューアが入力を受け取って予約を求めるまで、最大 60 秒待つ（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「SW の調停」）。Chrome 109 以前の SW は、イベントから 30 秒たつと止められうる。Chrome 110 以降は、拡張の API の呼び出しとイベントが、アイドルのタイマーを延ばす（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md)「設計の際に決めたこと（2026-10-31）」）。

拡張だけの構成では、完成した記事を `chrome.storage.session` に持ち（[ADR-0027](0027-article-job-api.md)）、API キーの有無を `chrome.storage.local.getKeys()` で値を読まずに確かめる（[ADR-0010](0010-extension-stores-nothing.md)）。`chrome.storage.session` の容量は Chrome 112 から約 10 MB で、`getKeys()` は Chrome 130 から使える。`setAccessLevel` と CSP の `'wasm-unsafe-eval'` は Chrome 102 から使える（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「構成を縛る外部の条件（要件書 AC-20）」）。

Chrome は、セキュリティの修正を最新の安定版にだけ入れ、延長の安定版も 1 つ前の版を約 8 週支えるだけである。1 年以上前の版のブラウザは、拡張と関係なく、修正されていない脆弱性を抱えている。`minimum_chrome_version` は更新を強いるものではなく、それより古い版への導入と更新を止めるだけである。

## 決定

-   **最小の版は、決める日の 1 年前の時点の安定版とする。使う機能がそれより新しい版を要するときだけ、その版に上げる。** 古い版の利用者を守っても、その利用者はブラウザ自体の脆弱性にさらされたままで、守れるものがないためである。決まった規則にすれば、機能を足すたびに版を調べずに済む。版の値と日付は、[Chromium Dash](https://chromiumdash.appspot.com/schedule) で確かめる。規則は [0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md) の「ID と対象のブラウザ」と「設計の際に決めたこと（2026-11-12）」にある。
-   **manifest の `minimum_chrome_version` は `"142"` とする。** 2026-11-13 に決めた。その 1 年前の 2025-11-13 の時点の安定版が Chrome 142（2025-10-28）で、使う機能のうち最も新しい `getKeys()`（130）より新しい。生成の開始から予約までの待ちの間は、タブの作成と `chrome.storage.session` の読み書き、ビューアからのリクエストが SW の止まるまでの時間を延ばす。それでも SW が止まったときは、ポップアップは結果不明として示す。規則は [extension/static/manifest.json](../../../extension/static/manifest.json) の `minimum_chrome_version` と、[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md) の「ID と対象のブラウザ」にある。
-   **両方の README は、必要なブラウザとして Chrome 142 以降と、Chromium 142 以降の Brave を記す。** `README.md` と `README.en.md` は、同じ値を利用者向けに書く。[extension/test/manifest.test.ts](../../../extension/test/manifest.test.ts) は、manifest の値と、両方の README が書く版が manifest の値と等しいことを確かめる。利用者が読み込める版を知り、文書と manifest の値が食い違わないようにするためである。
-   **ポップアップの末尾のプレビューの規則が使う `:dir()`（Chrome 120 以降）は、最小の版で必ず使える。** 最小の版を 142 にしたので、Chrome 119 以前で規則が捨てられる場合はなくなった（[0046 設計書](../../tasks/0046_popup_redesign/02_architecture.md)「設計の際に決めたこと（2026-10-28）」）。

最小の版を `"142"` にすることは、まだ manifest と両方の README に反映していない。今の値は `"110"` である。この差分は [#773](https://github.com/isseis/yt2column/issues/773) の実装で解消する。

## 採らなかった案

-   **`AbortSignal.timeout()` を使わなくなったときに、版を 103 より下げる。** 下げる要件がない（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1）。
-   **版を 103 のままにし、応答を待つ間、ポップアップとビューアの `runtime.connect` のポートを SW に開いて寿命を延ばす。** 0050 の設計書は、これを Chrome 110 以降の振る舞いが試作の確認で崩れた場合の代わりの方法としてだけ挙げた（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md) §3.16）。確認では崩れなかった（[0050 実装計画書](../../tasks/0050_viewer_tab/03_implementation_plan.md) §4.3.1）。版を上げずに最初からこの方法を採らなかった理由は記録されていない。
-   **`"110"` のままにする。** Chrome 111 以前では `chrome.storage.session` に記事を 1 MB しか持てず、上限を版ごとに変えることになる（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）。
-   **使う機能が要る版を最小の版にする（今なら `getKeys()` の 130）。** 機能を足すたびに要る版を調べることになるが、古い版の利用者はブラウザ自体の脆弱性にさらされていて、導入できる範囲を広げても守れるものがない（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）。
-   **`chrome.storage.session` の容量に合わせて `"112"` にする。** API キーの有無を確かめるのに `getKeys()` を使えず、値を読まずに確かめるための秘密でない印を別に保存することになる（同上）。

## 結果

-   Chrome 141 以前と、Chromium 141 以前の Brave には、拡張を導入できず、既存の利用者にも更新が届かない。
-   値を変えるときは、manifest、両方の README の記述、`manifest.test.ts` の値の確かめを同じ変更で直す。README の版が manifest と違うと、`manifest.test.ts` が失敗する。
-   SW の処理を足すときは、待ちに入る前の最後の API の呼び出しから最後の処理までを、30 秒に収める（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md) §3.4.3）。
-   新しい API や CSS の機能を使うときは、それが加わった版と最小の版を比べる。新しい版を要するときは、その版に上げる。最小の版を見直すときは、見直す日の 1 年前の安定版を求め直す。

## 経緯

- 2026-10-07 拡張の骨格を作るとき、使う API のうち最も新しい `chrome.storage.session` が加わった版に合わせて `"102"` とした（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.1）
- 2026-10-16 拡張から送信するとき、送信の時間の上限に使う `AbortSignal.timeout()` が加わった版に合わせて `"103"` に上げた（[0020 設計書](../../tasks/0020_extension_send/02_architecture.md) §3.1）
- 2026-10-19 Native Messaging に移るとき、`AbortSignal.timeout()` を使わなくなったが、`"103"` のまま上げも下げもしなかった（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1）
- 2026-10-28 ポップアップの再設計で `:dir()`（Chrome 120 以降）を使うが、`"103"` を保ち、Chrome 119 以前の右から左の文字だけの末尾のプレビューを保証の外とした（[0046 設計書](../../tasks/0046_popup_redesign/02_architecture.md)「設計の際に決めたこと（2026-10-28）」）
- 2026-10-31 生成の開始を SW から送るようにするとき、Chrome 110 以降の SW のアイドルのタイマーの振る舞いに頼るため、`"110"` に上げ、README に書くことにした（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md)「設計の際に決めたこと（2026-10-31）」）
- 2026-11-13 最小の版を、決める日の 1 年前の時点の安定版とする規則を定め、`"142"` に上げることにした。古い版を保証の外とした `:dir()` の扱いと、Chrome 120 まで上げる案を採らなかったことは、この版で意味を失った（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)「設計の際に決めたこと（2026-11-12）」）
