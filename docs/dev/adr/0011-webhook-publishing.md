# ADR-0011: 投稿先は Slack 互換の Incoming Webhook とし、投稿は利用者の操作でだけ行う

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `廃止` |
| 廃止の日付 | 2026-11-11 |
| 廃止の理由 | Webhook への投稿の機能を廃止し、利用者がクリップボードへコピーした記事を手で貼り付ける方法で置き換えることにしたため。投稿のコードとそのテストは、0083 の実装の PR で削除するまで残る（[0083 要件定義書](../../tasks/0083_remove_slack_webhook/01_requirements.md)） |
| 代わりの ADR | [ADR-0043](0043-copy-mention-neutralization.md) |

## 背景

生成した記事を、利用者が使う Mattermost や Slack のチャンネルへ投稿する。Mattermost の Incoming Webhook は Slack の Incoming Webhook とおおむね互換に作られているが、完全な互換ではない。Webhook URL はそれ自体が秘密情報であり、記事の本文は LLM の出力で、字幕へのプロンプトインジェクションによって内容を操作されうる。Mattermost でも Slack でも、`@channel` などの記法がチャンネル全体への通知を発生させる。

## 決定

-   **投稿先は Slack 互換の Incoming Webhook とする。** 検証の対象は Mattermost とし、Slack へは当面 best effort で送る。Mattermost の Incoming Webhook は Slack とおおむね互換で、記事を利用者のチャンネルへ届けられるためである。規則は [0006 要件定義書](../../tasks/0006_slack_webhook_publisher/01_requirements.md) §1「概要 (Overview)」と [project_overview.md](../project_overview.md) にある。
-   **投稿先のチャンネル・表示名・アイコンは Webhook の設定に従い、ペイロードで上書きしない。** 投稿先を記事ごとに変えず、Webhook の設定を 1 つの正本にするためである。規則は 0006 要件定義書 §2.3「スコープ外 (Out of Scope)」にある。
-   **ペイロードは `text` と `silent: true` だけを送る。** `text` は Markdown として表示される。通知を起こさないよう、`silent` を常に付ける。規則は [security.md](../security.md) §6「信頼できないテキストの取り扱い」と [0028 要件定義書](../../tasks/0028_output_layout/01_requirements.md) F-002「Slack の投稿の構成」にある。
-   **1 つのメッセージは 16,383 コードポイント以内とし、記事は最大 10 通に分割する。** 分割するときは、全体の何番目かを示す位置の印（`(k/N)`）を各メッセージの先頭に付ける。上限内に分割できない記事は、1 通も送らずに拒否する。Incoming Webhook の `text` の上限に収めるためである。規則は 0028 要件定義書 F-002「Slack の投稿の構成」・AC-12・AC-13 にある。
-   **成功は、HTTP の状態コードが 200 で、応答の本文が `ok` のときとする。** 1 通ごとに 30 秒で打ち切り、応答の本文は上限までしか読まない。リダイレクトには従わない。リダイレクト先が Webhook URL を別のホストへ送るのを防ぐためである。自動で再送しないことは [ADR-0028](0028-no-automatic-resend.md) が定める。規則は security.md §3「ネットワーク通信」にある。
-   **メンションの通知を起こさない。** 対象の `@` は、見出し行・脚注・本文のコードの外で、`@` の直後に U+200B を入れて無効化する。無効化しない並びを含む記事は、1 通も送らずに拒否する。語の途中の `@` はメンションにならないので、拒否も変更もしない。拒否の報告には、規則の名前と位置だけを示し、記事の文面を含めない。チャンネル全体への通知を起こさないためである。対象の `@` と無効化しない並びの定義は [security.md](../security.md) §6 と、[0028 要件定義書](../../tasks/0028_output_layout/01_requirements.md) AC-14・AC-14b・AC-14c にある。
-   **Webhook への送信の失敗は、URL を含まないエラーに直す。** `*url.Error` をそのまま返すと URL が現れるためである。Webhook URL の持ち方と、値を出力に流さないことは [ADR-0031](0031-secret-closure.md) が定める。規則は security.md §2「秘密情報（API キー・Webhook URL）」にある。
-   **投稿は利用者の操作でだけ行う。** Webhook へ投稿するのはサーバだけであり、ビューアからの投稿の操作（`POST /jobs/{key}/publish`）だけが投稿を始める。サーバの投稿のジョブは、記事を検査・分割して送る。CLI は投稿せず、記事を `--out` のファイルに書くだけである。生成の完了時に自動で投稿する予約は設けない。理由は記録されていない。投稿の操作を含むジョブの API は [ADR-0027](0027-article-job-api.md) が定め、規則は [0044 要件定義書](../../tasks/0044_article_job_api/01_requirements.md) §1「概要 (Overview)」、[docs/dev/guides/cli.md](../guides/cli.md) と security.md §8.2「localhost サーバ」にある。

## 採らなかった案

-   **Slack に正式に対応し、Block Kit へ移行する。** Slack は折りたたみに対応しない。`context` ブロックは使えるが、`section` ブロックの上限（1 つ 3,000 文字）に合わせて分割処理を作り直す必要があり、今の分割は `text` の上限 16,383 文字を前提としている（[0006 要件定義書](../../tasks/0006_slack_webhook_publisher/01_requirements.md) §2.3、[0028 要件定義書](../../tasks/0028_output_layout/01_requirements.md) §1）。
-   **ペイロードで投稿先のチャンネル・表示名・アイコンを上書きする。** Webhook に設定されたチャンネルと表示名で投稿する（[0006 要件定義書](../../tasks/0006_slack_webhook_publisher/01_requirements.md) §2.3）。
-   **CLI の 1 回の実行で、ファイルへの保存と Webhook への投稿の両方を行う。** CLI が投稿していた間の案である（0062 で CLI の投稿を削除した）。出力先は 1 回の実行で 1 つとする（[0006 要件定義書](../../tasks/0006_slack_webhook_publisher/01_requirements.md) §2.3）。両方を同時に行わない理由は記録されていない。
-   **CLI に `--dry-run` を設ける。** CLI が投稿していた間の案である（0062 で CLI の投稿を削除した）。投稿せずに記事を確認する用途は、`--out` によるファイルへの書き出しで満たせる（[0006 要件定義書](../../tasks/0006_slack_webhook_publisher/01_requirements.md) §2.3）。
-   **CLI の Webhook への投稿（`--slack`）を残す。** CLI からの投稿は拡張のビューアからの投稿と役割が重なり、開発で記事を確かめるには `--out` のファイルで足りる（[0062 要件定義書](../../tasks/0062_remove_cli_slack/01_requirements.md) §1）。
-   **CLI は、`SLACK_WEBHOOK_URL` が設定されていたら拒否する。** サーバのために設定した環境のままでは CLI を実行できなくなる（[0062 要件定義書](../../tasks/0062_remove_cli_slack/01_requirements.md) §1）。
-   **生成の完了時に自動で投稿する。** 理由は記録されていない（[0044 要件定義書](../../tasks/0044_article_job_api/01_requirements.md) §1 は「完了したら自動で投稿する予約は設けない」とだけ述べる）。
-   **ジョブの API の失敗を Webhook へ通知する。** 生成と投稿の失敗は状態の取得で返す。ビューアが失敗を表示できるので、Webhook への通知は重複になる（[0044 要件定義書](../../tasks/0044_article_job_api/01_requirements.md) §1）。
-   **分割したメッセージをスレッドの返信としてつなぐ。** Incoming Webhook は投稿したメッセージの識別子を返さない（成功の応答は本文 `ok` だけ）ので、Incoming Webhook ではつなげない（[0044 設計書](../../tasks/0044_article_job_api/02_architecture.md) §1.3）。Slack や Mattermost の API で投稿すれば、投稿したメッセージの識別子を受け取ってつなげるが、今は実装を簡単にするために Incoming Webhook を使う。API による投稿は将来の選択肢になりうるが、Slack と Mattermost とで実装が大きく異なる。

## 結果

-   投稿先は Webhook の設定で決まる。記事ごとにチャンネルや表示名を変えることはできない。
-   記事が長いときは、位置の印を付けた複数のメッセージになる。途中で失敗した場合は、どこまで投稿したかが分かるエラーで報告する。
-   投稿は利用者の操作でだけ始まるので、記事の生成（LLM の呼び出し）には料金が生じるが、投稿の操作がなければ投稿は起きない。サーバが Webhook へ送るのは、投稿の操作が始めた記事だけであり、失敗は状態の取得で返す。
-   Webhook が未設定でもサーバは起動する。投稿の操作は、Webhook が未設定であることを理由に拒否される。CLI は Webhook URL（`SLACK_WEBHOOK_URL`）を読まない。
-   無効化しない並びを含む記事は、1 通も送らずに拒否される。拒否は記事だけで決まるので、同じ記事を送り直しても結果は変わらない。

## 経緯

- 2026-10-07 Slack 互換の Incoming Webhook を投稿先とし、`text` と `silent` だけを送る `SlackWebhookPublisher` と CLI の `--slack` を加えた。検証の対象は Mattermost、Slack は best effort とし、メンションの抑止、16,383 コードポイントでの分割、失敗のエラーに Webhook URL を含めないことを定めた（[0006 要件定義書](../../tasks/0006_slack_webhook_publisher/01_requirements.md)）
- 2026-10-14 サーバの旧 `/articles` の失敗を、記事と同じ Webhook へ `silent` の 1 通で通知する経路を加えた。送信の経路を記事の投稿と共有し、CLI は通知しない（[0016 要件定義書](../../tasks/0016_failure_notification/01_requirements.md)）
- 2026-10-19 Slack の投稿を、見出し行（タイトルとチャンネル名）・本文・末尾の脚注（モデルと料金）の順に改め、Block Kit へは移行せず plain text のままとした。語の途中の `@` を受理する規則に変え、対象の `@` を U+200B で無効化する（[0028 要件定義書](../../tasks/0028_output_layout/01_requirements.md)）
- 2026-10-27 サーバが生成の完了時に続けて投稿していた形を改め、生成・状態の取得・投稿を別の操作に分け、Slack への投稿をビューアからの利用者の操作で行うようにした。生成の完了時には自動で投稿しない。サーバの投稿のジョブは `PostMessages` で送り、CLI の `--slack` は `Publish` を残した。投稿の操作を冪等にし、新しい API の投稿の失敗を Webhook へ通知しない（[0044 要件定義書](../../tasks/0044_article_job_api/01_requirements.md)）
- 2026-11-04 旧 API（`/articles`）を削除するのに合わせて、旧 API の失敗の通知（`internal/failure`・`SlackFailureNotifier`）を削除した。サーバが Webhook へ送るのは、利用者の投稿の操作が始めた記事だけになった（[0059 要件定義書](../../tasks/0059_remove_legacy_articles_api/01_requirements.md)「issue #526 で決めたこと」）
- 2026-11-05 CLI の Webhook への投稿（`--slack`）と `SlackWebhookPublisher.Publish` を削除し、Webhook へ投稿するのをサーバの投稿のジョブだけにした。CLI は `SLACK_WEBHOOK_URL` を読まず、`SlackWebhookPublisher` は `Publisher` を満たさない（[0062 要件定義書](../../tasks/0062_remove_cli_slack/01_requirements.md)）
- 2026-11-11 Webhook への投稿の機能を廃止することにした。メンションの無効化はビューアのコピーに移し、コピーは拒否しない（[0083 要件定義書](../../tasks/0083_remove_slack_webhook/01_requirements.md)「要件定義の際に決めたこと（2026-11-11）」）
