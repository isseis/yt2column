# ADR-0005: サーバを Native Messaging のホストと常駐サーバに分け、HTTP を Unix ドメインソケットの上で受ける

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `廃止` |
| 廃止の日付 | 2026-11-12 |
| 廃止の理由 | 記事の生成を拡張だけで行うことにし、localhost サーバと Native Messaging のホストを廃止することにしたため。サーバとホストのコードは、[#773](https://github.com/isseis/yt2column/issues/773) の実装で撤去するまで残る（[0090 要件定義書](../../tasks/0090_extension_only_architecture/01_requirements.md)） |
| 代わりの ADR | [ADR-0047](0047-extension-only-generation.md) |

## 背景

記事を作る処理は、開発用の CLI とブラウザ拡張の両方から使う。拡張は、利用者が YouTube の文字起こしパネルからコピーした字幕と動画ページの情報を、利用者のマシンで動くサーバへ送る。記事の生成は、利用者のマシンで、ブラウザを閉じた後も続けられる必要がある。ブラウザは、拡張が送るたびにプロセスを起動する Native Messaging の仕組みを持つが、そのプロセスは長く動かず、シェルの環境変数も引き継がない。

サーバが loopback の TCP ポートで待ち受けると、ウェブページからサーバへ届く経路が残り、DNS rebinding や preflight の不要な `POST` への対策を別に要する（[0011 設計書](../../tasks/0011_localhost_server/02_architecture.md)、[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md)）。

## 決定

-   **サーバは `cmd/yt2column-server` の別の実行ファイルとし、利用者が手動で起動する。ログイン時の自動起動はしない。** 理由は記録されていない。規則は [0011 要件定義書](../../tasks/0011_localhost_server/01_requirements.md) F-001 にある。
-   **記事を作る処理は、CLI と同じパッケージを共有し、サーバ固有のコードは HTTP の入口・検査・応答・ジョブに限る。** CLI と同じコアロジックを共有するためである。規則は 0011 要件定義書 §2.1「目的 (Goals)」に、構成要素の分け方は [ADR-0019](0019-pipeline-component-interfaces.md) と、[package_reference.md](../conventions/package_reference.md) のパッケージの表にある。
-   **サーバは `yt-dlp` を呼ばない。** 字幕は拡張から受け取り、`internal/transcript` の `CopiedSource` で解析する。`yt-dlp` を使うのは CLI の経路だけである。サーバは拡張から字幕を受け取る経路だけで記事を作るので、`yt-dlp` を呼ぶ必要がない。字幕の入力の決定は [ADR-0007](0007-manual-transcript-copy.md) が定める。規則は 0011 要件定義書 §2.3「スコープ外 (Out of Scope)」にある。
-   **待ち受けは Unix ドメインソケット（UDS）だけにし、TCP のポートを開かない。** ソケットのパスは `HOME` だけから導出する。TCP を待ち受けるとウェブページからサーバへ届く経路が残り、DNS rebinding と preflight の対策を別に要するためである。規則は [security.md](../security.md) の §8「ブラウザ拡張と localhost サーバ」と §8.2「localhost サーバ」にあり、ソケットの置き場所と権限、起動のロック、stale の扱いもそこに従う。
-   **UDS の上でも HTTP/1.1 を使い、新しいフレーム形式を作らない。** リクエストの検査はパスとメソッドだけで、`Host`・`Origin`・`Content-Type`・`Authorization` は検査しない。`http.Server` の上限と時間の設定、既存のテスト、`curl` による確認を再利用でき、ソケットに接続できる相手はこれらのヘッダを偽れるので、検査しても排除できる相手がいないためである。規則は [0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.4「UDS 上のプロトコル」と security.md §8.2 にある。
-   **生成の開始は `202` で受け付け、ジョブをサーバのメモリで動かす。** 1 回の中継が 20 秒で打ち切られるので、長い処理を待たずに受け付ける。同時に走る生成の数、本文の大きさ、読み取り・書き込み・待機の時間の上限は security.md §8「ブラウザ拡張と localhost サーバ」と §8.2 に従う。ジョブの API と記事のキー・キャッシュは [ADR-0027](0027-article-job-api.md) が定める。
-   **拡張は Native Messaging のホストだけに送り、ホストが UDS へ中継する。** ブラウザはページから UDS へ接続できないので、拡張の送信はホストを通す。規則は security.md §8 と §8.2、[ADR-0026](0026-extension-platform.md) にある。
-   **送り先は固定のホスト名 `io.github.isseis.yt2column` とし、ホストを起動できる拡張は manifest の `allowed_origins` に載った固定の拡張だけとする。** 拡張以外からホストを起動させないためである。拡張の ID を manifest の `key` で固定することは ADR-0026 が定め、規則は security.md §8.1「ブラウザ拡張」と §8.2 にある。
-   **ホストは 1 つのメッセージにつき 1 回起動し、1 つのリクエストを送って 1 つの応答を返し、終了する。** ブラウザが送信のたびにプロセスを起動する仕組みに合わせるためである。規則は security.md §8.2 と、0026 設計書 §3.5.1「起動の形」にある。
-   **ホストはサーバを起動せず、ジョブはサーバに残る。ブラウザを閉じてもジョブは続く。** 記事の生成をブラウザの寿命から切り離すためである。規則は security.md §8 と、[project_overview.md](../project_overview.md) の「決定済みの方針」にある。

Slack への投稿の処理は、まだコードに残っている。この差分は [#770](https://github.com/isseis/yt2column/issues/770) の実装で解消する。

## 採らなかった案

-   **UDS の上に 4 バイトの長さと JSON の独自のフレームを作る。** HTTP/1.1 を再利用すれば要件を満たせ、`http.Server` の上限と時間の設定、既存のテスト、`curl` による確認を捨てる理由がない。独自のフレームにすると、長さの上限・時間の上限・応答の形・テストの道具を新しく書くことになる（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) 付録 A「決定の記録」）。
-   **`Host`・`Origin` の検査を残し、ホストに `Origin` を付けさせる。** UDS ではソケットに接続できる相手なら誰でもこれらのヘッダを偽れるので、検査しても排除できる相手がいない（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md) §3.4）。
-   **Native Messaging のホストに記事の生成と Slack への投稿まで持たせる（フル移行）。** ジョブの寿命（ブラウザを閉じたとき、拡張を再読み込みしたとき）の問題を避けられず、秘密情報と設定の読み方、ログの出力先の問題も、ブラウザが起動するプロセス側に移る（[0026 要件定義書](../../tasks/0026_native_messaging_relay/01_requirements.md) §1）。
-   **移行しない（TCP のサーバのままにする）。** 理由は記録されていない（0026 要件定義書 §1 は 3 つの案を挙げ、中間案を採った理由だけを書いている）。
-   **TCP と共有トークンのフォールバックを残し、新旧の経路を併存させる。** 理由は記録されていない（0026 要件定義書 §2.3 は「移行は一括とし、旧経路は残さない」とだけ述べる）。

## 結果

-   サーバは TCP のポートを開かないので、ウェブページからサーバへ届く経路がない。他のユーザーはソケットの権限で排除される。同じユーザーのプロセスは接続できる（[ADR-0006](0006-threat-model.md)）。
-   ブラウザは送信のたびにホストを起動するので、1 回の送信でホストのプロセスが 1 つ起動し、応答の後に終了する。ホストはサーバを起動しないので、サーバが動いていなければ送信は失敗する。
-   ジョブはサーバのメモリにあり、サーバを再起動すると失われる。ブラウザを閉じてもジョブは続く。
-   待ち受けを変えても、記事を作る処理は CLI と共有したままである。ホストは記事の JSON を解釈せず、決まった形のメッセージを決まったパスへ中継する。

## 経緯

- 2026-10-13 localhost サーバを別の実行ファイルとし、loopback の TCP だけで待ち受け、`Host`・`Origin`・`Content-Type`・共有トークンで入口を限定した。生成は非同期のジョブとし、同時に走る生成を 2 つに限った（[0011 設計書](../../tasks/0011_localhost_server/02_architecture.md)）
- 2026-10-15 拡張が、loopback の TCP の API へ共有トークンを付けて HTTP を送るクライアントを加えた。サーバの構成は変えていない（[0020 要件定義書](../../tasks/0020_extension_send/01_requirements.md)）
- 2026-10-19 待ち受けを TCP から UDS に変え、ブラウザが起動する Native Messaging のホストが中継する形にした。共有トークン・ポートの設定・ヘッダの検査を削除し、HTTP/1.1 を UDS の上で再利用した（[0026 設計書](../../tasks/0026_native_messaging_relay/02_architecture.md)）
- 2026-11-11 Webhook への投稿の機能の廃止に合わせて、サーバが CLI と共有する処理から Slack への投稿を除いた（[0083 要件定義書](../../tasks/0083_remove_slack_webhook/01_requirements.md) F-001）
- 2026-11-12 記事の生成を拡張だけで行うことにし、localhost サーバと Native Messaging のホストを廃止することにした。この ADR を廃止した（[0090 要件定義書](../../tasks/0090_extension_only_architecture/01_requirements.md)「issue #769・#772 で決めたこと」）
