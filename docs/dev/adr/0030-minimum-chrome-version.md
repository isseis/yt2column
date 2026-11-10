# ADR-0030: 拡張の minimum_chrome_version を 110 にする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

拡張は Chrome と Brave で動く（[ADR-0026](0026-extension-platform.md)）。manifest の `minimum_chrome_version` は、拡張を読み込める Chrome の最も古い版を宣言する。拡張が頼る API や振る舞いのうち最も新しいものが、この版で使えなければならない。

タスク 0050 は、生成の開始を Service Worker（SW）から送るようにした。生成の開始のやり取りは最大 25 秒かかり、その間、ブラウザの API の呼び出しはない。Chrome 109 以前の SW は、イベントから 30 秒たつと止められうる。Chrome 110 以降は、拡張の API の呼び出しとイベントが、アイドルのタイマーを延ばす（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md)「設計の際に決めたこと（2026-10-31）」）。

## 決定

-   **manifest の `minimum_chrome_version` は `"110"` とする。** 生成の開始のやり取りの間に SW が止められないことは、Chrome 110 以降の SW のアイドルのタイマーの振る舞いに頼る。規則は [extension/static/manifest.json](../../../extension/static/manifest.json) の `minimum_chrome_version` と [0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md)「設計の際に決めたこと（2026-10-31）」にある。
-   **両方の README は、必要なブラウザとして Chrome 110 以降と、Chromium 110 以降の Brave を記す。** `README.md` と `README.en.md` は、同じ値を利用者向けに書く。[extension/test/manifest.test.ts](../../../extension/test/manifest.test.ts) は、manifest の値が `"110"` であることと、両方の README が書く版が manifest の値と等しいことを確かめる。利用者が読み込める版を知り、文書と manifest の値が食い違わないようにするためである。
-   **`:dir()`（Chrome 120 以降）を使うポップアップの末尾のプレビューの規則は、最小の版を上げずに使う。** Chrome 119 以前では、その規則だけが捨てられる。右から左の文字だけの末尾のプレビューで最後の文字が隠れることは、保証の外とする。Chrome 119 以前で捨てられるのはその規則だけで、保証の外とした範囲が少し広がるだけだからである（[0046 設計書](../../tasks/0046_popup_redesign/02_architecture.md)「設計の際に決めたこと（2026-10-28）」）。

## 採らなかった案

-   **`AbortSignal.timeout()` を使わなくなったときに、版を 103 より下げる。** 下げる要件がない（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1）。
-   **`:dir()` が使える Chrome 120 まで、最小の版を上げる。** Chrome 119 以前で捨てられるのはその規則だけで、保証の外とした範囲が少し広がるだけである（[0046 設計書](../../tasks/0046_popup_redesign/02_architecture.md)「設計の際に決めたこと（2026-10-28）」）。
-   **版を 103 のままにし、応答を待つ間、ポップアップとビューアの `runtime.connect` のポートを SW に開いて寿命を延ばす。** 0050 の設計書は、これを Chrome 110 以降の振る舞いが試作の確認で崩れた場合の代わりの方法としてだけ挙げた（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md) §3.16）。確認では崩れなかった（[0050 実装計画書](../../tasks/0050_viewer_tab/03_implementation_plan.md) §4.3.1）。版を上げずに最初からこの方法を採らなかった理由は記録されていない。

## 結果

-   Chrome 109 以前と、Chromium 109 以前の Brave には、拡張を読み込めない。
-   値を変えるときは、manifest、両方の README の記述、`manifest.test.ts` の値の確かめを同じ変更で直す。README の版が manifest と違うと、`manifest.test.ts` が失敗する。
-   SW の処理を足すときは、待ちに入る前の最後の API の呼び出しから最後の処理までを、30 秒に収める（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md) §3.4.3）。
-   新しい API や CSS の機能を使うときは、それが加わった版と最小の版を比べる。これまでのタスクは、版を上げるか（0020・0050）、古い版で欠ける範囲を保証の外とした（0046）。

## 経緯

- 2026-10-07 拡張の骨格を作るとき、使う API のうち最も新しい `chrome.storage.session` が加わった版に合わせて `"102"` とした（[0007 設計書](../../tasks/0007_browser_extension_skeleton/02_architecture.md) §3.1）
- 2026-10-16 拡張から送信するとき、送信の時間の上限に使う `AbortSignal.timeout()` が加わった版に合わせて `"103"` に上げた（[0020 設計書](../../tasks/0020_extension_send/02_architecture.md) §3.1）
- 2026-10-19 Native Messaging に移るとき、`AbortSignal.timeout()` を使わなくなったが、`"103"` のまま上げも下げもしなかった（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1）
- 2026-10-28 ポップアップの再設計で `:dir()`（Chrome 120 以降）を使うが、`"103"` を保ち、Chrome 119 以前の右から左の文字だけの末尾のプレビューを保証の外とした（[0046 設計書](../../tasks/0046_popup_redesign/02_architecture.md)「設計の際に決めたこと（2026-10-28）」）
- 2026-10-31 生成の開始を SW から送るようにするとき、Chrome 110 以降の SW のアイドルのタイマーの振る舞いに頼るため、`"110"` に上げ、README に書くことにした（[0050 設計書](../../tasks/0050_viewer_tab/02_architecture.md)「設計の際に決めたこと（2026-10-31）」）
