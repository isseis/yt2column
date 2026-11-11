# ADR-0010: 拡張は何も保存せず、権限も加えず、状態の正しい値をサーバに置く

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

ブラウザ拡張は、字幕と動画メタデータをサーバへ送り、記事をビューアに表示する。拡張が設定や記事の状態を保存すると、保存した値の更新と、保存の権限の管理が要る。Service Worker はブラウザによっていつでも停止されるので、そのメモリだけを状態の正しい置き場所にはできない。

## 決定

-   **拡張は `chrome.storage` を使わない。** 設定を保存せず、オプションページも持たない。送信ごとの値は、ビューアの URL と各ページのメモリにだけ置き、Service Worker のメモリにある値はビューアに届けるまでの短い間だけのものとする。保存した値の更新と、保存の権限の管理を要さないためである。規則は [security.md](../security.md) §8.1「ブラウザ拡張」にある。
-   **要求する権限は `activeTab`・`scripting`・`nativeMessaging` の 3 つだけにする。** `host_permissions`・`storage`・`options_ui`・`tabs`・`downloads`・`clipboardWrite` を加えず、ページの読み込み時に自動で実行されるコード（`content_scripts`）も宣言しない（起動の経路は [ADR-0008](0008-launch-path.md)）。閲覧内容全般に及ぶ権限を持たないためである。規則は [security.md](../security.md) §8.1 と [extension/static/manifest.json](../../../extension/static/manifest.json) にある。
-   **権限を広げずに、ビューアを開き、保存し、コピーする。** ビューアのタブは `chrome.tabs` の権限の要らない操作で開き、`chrome.windows.update` で前面に出す。記事の保存は拡張のページからの Blob のダウンロードで、コピーは利用者の操作の中のクリップボードへの書き込みで行う。権限を広げずに要件を満たすためである。規則は [security.md](../security.md) §8.1 にある。
-   **状態の正しい値はサーバに置く。** 生成と工程の状態はサーバが持ち、Service Worker は生成の開始を 1 回送るだけで、ポーリングはしない。ポップアップとビューアが記事のキーで状態の取得を送る。Service Worker はいつでも停止されるので、そのメモリだけを状態の正しい置き場所にしないためである。規則は [security.md](../security.md) §8.1・§8.2「localhost サーバ」と [project_overview.md](../project_overview.md) にある。
-   **サーバの記事の状態の持ち方は [ADR-0027](0027-article-job-api.md) が、生成の開始を送り直さないことは [ADR-0028](0028-no-automatic-resend.md) が定める。**
-   **古い版が保存した値は、読めないまま残ることを受け入れる。** 0020 の版が `chrome.storage.local` に保存した共有トークンとポートは、拡張を更新しても残りうる。拡張は `storage` の権限を持たないので、これを読むことも削除することもできない。サーバはトークンを読まないので、残った値が通用する相手はない。規則は [0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1「manifest」と [security.md](../security.md) §8.1 にある。
-   **ビューアの URL に持つ値は、ブラウザの履歴に残ることを受け入れる。** URL に持つ値と、コピーした字幕と概要欄を入れない規則は [security.md](../security.md) §8.1 に従う。値を URL に置けば、再読み込み・閉じたタブの復元・履歴からの表示の後も同じ記事を表示できるためである。規則は [security.md](../security.md) §8.1 と [0058 要件定義書](../../tasks/0058_same_video_article/01_requirements.md) F-004 にある。

投稿の状態は、まだコードに残っている。この差分は [#770](https://github.com/isseis/yt2column/issues/770) の実装で解消する。

## 採らなかった案

-   **古い版の値を削除するために `storage` の権限を残す。** 0027 で Service Worker をなくしたので `chrome.runtime.onInstalled` を受け取れず、削除の処理を置く場所がない。ポップアップを開くたびに削除する形にすると、1 回の削除のために `storage` の権限を持ち続けることになる（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1）。
-   **古い版が保存した値を、更新時に削除する。** 残しても害がない。サーバはトークンを読まないので、残った値が通用する相手はない。値はプロファイルのディレクトリにあり、読めるのは同一ユーザーのプロセスだけである（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1）。
-   **`downloads`・`clipboardWrite`・`tabs`・`storage`・host permission を加えて、保存・コピー・ビューアを実装する。** 権限の範囲を今より広げないためである（[0050 要件定義書](../../tasks/0050_viewer_tab/01_requirements.md) §1）。
-   **最近の記事の一覧を設け、記事や入力を拡張に保存する。** 「設定を保存しない」という今の性質を保ち、権限を加えずに済ませるためである（[0050 要件定義書](../../tasks/0050_viewer_tab/01_requirements.md) §1）。

## 結果

-   拡張のプロファイルに残るのは、ブラウザの履歴に残るビューアの URL と、0020 の版が残した読めない値だけである。拡張は設定を持たないので、設定の移行も要らない。
-   状態の表示はサーバへの状態の取得で行うので、Service Worker が停止しても、ビューアとポップアップは最新の状態を表示できる。
-   拡張の権限は、利用者が起動したタブの読み取りと、固定のホストへの送信に限られる。閲覧内容全般に及ぶ権限を持たない。

## 経緯

- 2026-10-15 共有トークンとサーバのポートを `chrome.storage.local` に保存し、`host_permissions` とオプションページを加えた。トークンがブラウザの外へ出るので、`chrome.storage.sync` には保存しない。送信の履歴と結果の自動再送は保存しない（[0020 要件定義書](../../tasks/0020_extension_send/01_requirements.md) §1）
- 2026-10-19 共有トークン・ポート・`storage` の権限・`host_permissions`・オプションページを削除し、`nativeMessaging` に移した。拡張は `chrome.storage` を参照せず、ジョブはサーバ側で続く。旧版の保存値は削除せず、読めないまま残ることを残余のリスクとして受け入れた（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.7.1）
- 2026-10-29 拡張は何も保存せず、状態の正しい値をサーバに置くことにした。送信ごとの値はビューアの URL とページのメモリだけに置き、権限は `activeTab`・`scripting`・`nativeMessaging` のまま加えない（[0050 要件定義書](../../tasks/0050_viewer_tab/01_requirements.md) §1）
- 2026-11-04 同じ動画の記事を示す機能でも、拡張に記事の履歴を保存しない（`chrome.storage` を使わない）方針を保った（[0058 要件定義書](../../tasks/0058_same_video_article/01_requirements.md)「issue #593 で決めたこと」）
- 2026-11-04 既存の記事を表示していることを、ビューアの URL の印 `e=1` で持つことにした（[0058 設計書](../../tasks/0058_same_video_article/02_architecture.md)「設計の際に決めたこと（2026-11-04）」）
- 2026-11-11 Webhook への投稿の機能の廃止に合わせて、サーバが持つ状態から投稿の状態を除いた（[0083 要件定義書](../../tasks/0083_remove_slack_webhook/01_requirements.md) F-001）
