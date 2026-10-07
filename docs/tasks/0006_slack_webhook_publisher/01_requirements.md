# 要件定義書：Slack 互換の Webhook への投稿（SlackWebhookPublisher）

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-07 |
| Review date | - |
| Reviewer | - |
| Comments | 2026-10-07: 決定の変更（利用者の決定による）のため `draft` に戻した。実際に投稿する先を Mattermost とし、Mattermost の Slack 互換の Incoming Webhook を検証の対象とする。Slack は当面 best effort とし、将来の正式な対応に備えて、フラグ（`--slack`）・環境変数（`SLACK_WEBHOOK_URL`）・型の名前は Slack のままとする。これに伴い、Webhook URL の規則（`https` の URL）、ペイロード（`markdown` ブロックではなく `text`）、メンションの記法と抑止の手段（`silent` の指定と送信前の拒否の併用）を改めた。 |

## 1. 概要 (Overview)

**目的:** 本書は、生成した記事を Slack 互換の Incoming Webhook で投稿する `SlackWebhookPublisher` と、CLI で投稿先に Webhook を選べるようにする機能の要件を定義する。

**背景:**
yt2column は `URL → TranscriptSource → Transcript → ArticleWriter → Article → Publisher` というパイプラインで動作する（[project_overview.md](../../dev/project_overview.md)）。CLI の組み立て（#6、タスク `0005_cli_assembly`）までで、動画 URL から記事を生成し、`FilePublisher` でローカルのファイルに書き出せるようになった。`SLACK_WEBHOOK_URL` の読み込みと検証も #6 で実装済みだが、それを使う投稿先はまだない（0005 の要件定義書 2.3）。

本プロジェクトで記事を実際に投稿する先は Mattermost である。Mattermost の Incoming Webhook は、Slack の Incoming Webhook とおおむね互換に作られている（完全な互換ではない）。そこで、投稿の仕組みは「Slack 互換の Incoming Webhook」として作り、両者に共通の部分だけを使う。検証の対象は Mattermost だけとし、Slack へは当面 best effort で送る（2.2 の「対応の範囲」）。将来 Slack に正式に対応する可能性があるので、フラグ・環境変数・型の名前は Slack のままとする。

Mattermost は利用者が自分で運用するサーバであり、Webhook URL のホストは利用者ごとに異なる。#6 で実装した `SLACK_WEBHOOK_URL` の規則（`https://hooks.slack.com/` で始まる）は Mattermost の URL を受理しないので、本タスクで改める。

Mattermost の Incoming Webhook は、ペイロードの `text` を Markdown として表示する。`text` には 1 つの投稿の文字数の上限があり、記事がそれを超える場合は、全体の何番目かが分かる形で複数のメッセージに分割して投稿する。

Webhook URL は、それ自体が秘密情報である（[security.md](../../dev/security.md) §2）。`net/http` が返す `*url.Error` はリクエストの URL を含むため、そのまま返すと Webhook URL がエラーのメッセージに現れる。

記事の本文は LLM の出力であり、字幕へのプロンプトインジェクションによって内容を操作されうる（[security.md](../../dev/security.md) §6）。Mattermost でも Slack でも、`@channel` などの記法がチャンネル全体への通知を発生させる。

**本書の記述範囲:** 本書は、観測できる振る舞い（何を送り、何を成功とみなし、何をどのエラーで報告するか）を定める。その振る舞いを実現する手段（型や関数の名前、番兵エラーの名前、上限値、タイムアウトの値、拒否する入力の検出の手段、メッセージの正確な書式など）は設計（`02_architecture.md`）で決める。ただし、各拒否をどのエラーの分類で報告するかと、メンションの通知の抑止の手段（F-006）は、本書で定める。レビューで挙がった事項のうち設計で決めるものは、[design_handoff.md](design_handoff.md) に記録する。

本書で「Webhook」とだけ書く場合は、Slack 互換の Incoming Webhook（Mattermost と Slack の両方）を指す。

対応 issue: #7

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   生成した記事を、Mattermost の Incoming Webhook で Markdown として投稿できる。
-   1 つのメッセージに収まらない記事を、上限内の複数のメッセージに分割し、順に投稿できる。
-   分割投稿が途中で失敗した場合に、どこまで投稿したかが分かるエラーで報告する。
-   Webhook URL を、送信先への HTTP リクエストを除き、エラーやログを含むどの出力にも漏らさない。
-   LLM の出力に含まれる記法によって、メンションの通知を発生させない。
-   CLI から、投稿先としてファイルと Webhook のいずれかを選べる。

### 2.2. スコープ (In Scope)

-   `SlackWebhookPublisher`（`internal/publisher` 内）: `Publisher` interface の実装。
-   記事の分割と、分割したメッセージの順次投稿。
-   HTTP の送信（タイムアウト、レスポンスサイズの上限、リダイレクトの扱い）と、応答の検証。
-   失敗の報告（投稿済みの範囲、Webhook URL を含めないエラー）。
-   `SLACK_WEBHOOK_URL` の検証の規則を、Mattermost の URL も受理する規則に改めること（F-001）。
-   CLI のフラグによる投稿先の選択と、それに伴う実行の流れ・終了コード・標準エラー出力への表示の変更。
-   `httptest` を使うユニットテスト。
-   実際の Mattermost の Webhook を使う統合テスト（`make test` からは除外）と、実際の Mattermost での表示の手動確認。
-   README・[project_overview.md](../../dev/project_overview.md)・[package_reference.md](../../dev/developer_guide/package_reference.md)・[security.md](../../dev/security.md) の更新（F-010）。

**対応の範囲（Mattermost と Slack）:**

| 送信先 | 対応 | 意味 |
|---|---|---|
| Mattermost の Incoming Webhook | 検証の対象 | 本書の要件は Mattermost で成り立つ。統合テストと手動確認（F-008・F-009）は Mattermost で行う |
| Slack の Incoming Webhook | best effort | Mattermost と同じペイロードを送る。Slack での表示（`text` は Slack では標準の Markdown ではなく mrkdwn として表示される）、文字数の上限、`silent` による通知の抑止は検証せず、保証しない。F-006 の拒否は Slack の記法も対象とする |

### 2.3. スコープ外 (Out of Scope)

-   Slack の正式な対応（Slack での表示と上限の検証、Block Kit の `markdown` ブロックの使用、Slack での通知の抑止の保証）。将来のタスクで扱う可能性がある。
-   投稿の自動リトライ（HTTP 429 の `Retry-After` に従う待機を含む）。Incoming Webhook への投稿は冪等ではなく、応答を受け取れなかったメッセージを再送すると、同じメッセージが重複して投稿されうるためである。
-   分割投稿の途中で失敗した場合の、続きからの再開。CLI を再実行すると、記事を生成し直して最初のメッセージから投稿する（F-007）。
-   投稿に失敗した記事の保存。記事を残したい場合は、`--out` でファイルに書き出す。
-   ファイルと Webhook の両方への同時の投稿。投稿先は 1 回の実行で 1 つとする（F-007）。
-   `--dry-run`。投稿せずに記事を確認する用途は、`--out` によるファイルへの書き出しで満たせる。0005 の要件定義書 2.3 で本タスクに委ねられた判断であり、設けないことにする。
-   Webhook のペイロードによる投稿先のチャンネル・表示名・アイコンの上書き（`channel`・`username`・`icon_url` など）。Webhook に設定されたチャンネルと表示名で投稿する。
-   Mattermost・Slack の REST API（トークンを使う API）、スレッドへの投稿、投稿したメッセージの編集と削除。
-   分割の境界をまたぐ Markdown の構造（コードブロック、リスト、表など）の表示の連続性。分割の位置は改行を優先して選ぶ（F-003）が、構造の内側で分割した場合の表示は保証しない。
-   記事中のリンクのプレビューの制御。
-   投稿の失敗を Webhook に通知すること（ブラウザ拡張と localhost サーバ、#43）。
-   LLM API の使用量と費用を投稿に追記すること（#99）。
-   Slack 互換の Incoming Webhook 以外の投稿先（Discord など）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: Webhook URL の設定と SlackWebhookPublisher の構築

**設定（`internal/config`）:**

-   `SLACK_WEBHOOK_URL` は、引き続き未設定なら受理する（値なし）。設定されている場合は、次の規則に合う値だけを受理する。合わない値と空の値は、0005 の F-001・F-002 と同じく、変数の名前を含み値を含まないエラーで拒否する。
    -   URL として解析できる（Go の `net/url` が解析でき、制御文字を含まない）。
    -   スキームが `https` である。
    -   ホストが空でない。
-   値は `secret.Secret` 型で保持する（0005 の F-001 と同じ）。
-   この規則は、0005 の要件定義書の `SLACK_WEBHOOK_URL` の規則（`https://hooks.slack.com/` で始まり、その後に 1 文字以上続き、空白文字を含まない）を置き換える。ホストを `hooks.slack.com` に限らないので、0005 の AC-05 で拒否していた値のうち `https://hooks.slack.com.example/services/x`・`https://HOOKS.SLACK.COM/services/x`・`https://hooks.slack.com/` は受理されるようになる。

**構築（`internal/publisher`）:**

`SlackWebhookPublisher` は、`Publisher` interface を実装する。

-   Webhook URL は `secret.Secret` 型として構築時に受け取る。環境変数は読まない。
-   値のない `secret.Secret`（ゼロ値）は、構築をエラーにする。
-   本番で使う構築の手段は、`internal/config` が `SLACK_WEBHOOK_URL` に課すのと同じ規則に合わない URL を拒否する。規則は `internal/config` と共有し、複製しない（共有の手段は [design_handoff.md](design_handoff.md) H-03）。現状は `internal/config` で検証済みの値しか渡らないが、他の呼び出し元が `http://` などの URL を渡して Webhook URL を平文で送ってしまうことを防ぐためである。
-   構築の拒否のエラーは、渡された URL の値もその一部も含まない。
-   テストでは、ループバックアドレスの送信先（`httptest` のサーバ）へ `http` で送る `SlackWebhookPublisher` を構築できる。この手段はテスト用のビルドでだけ使える（既存の `deepseek.NewForLoopbackTest` と同じ考え方）。
-   構築は、ネットワークにアクセスしない。

**Acceptance Criteria**:
- **AC-01**: `https://mattermost.example.com/hooks/xxxxxxxxxxxxxxxxxxxxxxxxxx` と `https://hooks.slack.com/services/T000/B000/XXXX` の形の Webhook URL で構築すると、いずれもエラーにならない。テスト用の送信先へ送るように構築した `SlackWebhookPublisher` は、構築しただけでは送信先にリクエストを送らない。
- **AC-02**: ゼロ値の `secret.Secret` で構築すると、エラーになる。
- **AC-03**: 次の値は、`SLACK_WEBHOOK_URL` としても、構築の引数としても拒否される。`SLACK_WEBHOOK_URL` の拒否は「値が不正」の番兵エラーになり、エラーのメッセージは変数の名前を含む。いずれのエラーのメッセージも、渡した値もその末尾 8 文字も含まない。
    -   `http://mattermost.example.com/hooks/x`（スキームが `https` でない）
    -   `mattermost.example.com/hooks/x`（スキームがない）
    -   `https:///hooks/x`（ホストが空）
    -   `https://mattermost.example.com/hooks/x` の末尾に改行（U+000A）を付けた値（制御文字）
- **AC-03a**: `SLACK_WEBHOOK_URL` に Mattermost の形の URL（AC-01）を設定して読み込むと、エラーにならず、`secret.Secret` として保持され、`Reveal` で元の値が得られる。

#### F-002: 投稿する内容

-   投稿の前に、記事が出力してよいものであることを `Article.CheckPublishable` で確認する。拒否された記事は、1 つのメッセージも投稿せずにエラーにする。
-   投稿する内容は、次をこの順に含む。正確な書式は `02_architecture.md` で決める。
    -   `Article.Title`（見出しとして）
    -   生成に使ったモデル（`Article.Model`）とモデルの版の識別子（`Article.ModelVersion`。空の場合は空であることを示す）。`FilePublisher` と同じく、どのモデルで生成したかを追えるようにするためである。
    -   `Article.Body`（加工しない）。`ArticleWriter` は元動画へのリンク（出典）を `Article.Body` の末尾に置く（#5）ので、出典は最後のメッセージの末尾に来る。
-   上記をこの順に並べた文字列を、本書では**投稿する文字列**と呼ぶ。F-003 の分割と、AC-04・AC-06・AC-08〜AC-10 の比較は、投稿する文字列に対して行う。
-   記事は、ペイロードの `text` に Markdown として入れる。Mattermost と Slack に共通の項目だけを使い、Block Kit の `blocks`、`attachments`、Mattermost に固有の `props` による表示は使わない。
-   投稿する文字列は、JSON の文字列の値として送る。記事が `"`・`\`・`{` などを含んでも、送るペイロードの構造は変わらず、送信先が受け取る `text` は投稿する文字列（分割した場合はその一部と分割の位置の表示）と一致する。

**Acceptance Criteria**:
- **AC-04**: 有効な `Article` を投稿すると、テスト用の送信先は、`Content-Type` が `application/json` の `POST` で、`text` を含む JSON のペイロードを受け取る。ペイロードは `blocks` と `attachments` を含まない。受け取ったすべてのメッセージの `text` を順に連結し、分割の位置の表示（F-003）を取り除くと、投稿する文字列と一致する。投稿する文字列は `Title`・`Model`・`ModelVersion`・`Body` をこの順に含み、`Body` で終わる。`ModelVersion` が空の `Article` も投稿できる。
- **AC-05**: `Article.CheckPublishable` が拒否する `Article`（`Title` が空、`Body` が空白文字だけ、`Body` が ESC を含む、`SourceURL` が空など）を投稿すると、`writer.ErrInvalidArticle` に対して `errors.Is` が真になるエラーになり、テスト用の送信先はリクエストを 1 つも受け取らない。
- **AC-06**: `Body` に `"`・`\`・`}`・`</script>`・U+2028 を含む `Article` を投稿すると、テスト用の送信先が受け取るペイロードは JSON として正しく、`text` から分割の位置の表示を取り除いたものは投稿する文字列と一致する。

#### F-003: 分割

-   Mattermost の 1 つの投稿の `text` には、文字数の上限がある。上限の値と文字数の数え方の単位は、`02_architecture.md` で Mattermost の公式文書（必要ならサーバのソース）を確認して決め、出典を記す（[design_handoff.md](design_handoff.md) H-01）。文字数の数え方は、Mattermost が数える値以上になるもの（安全側）にする。上限を超える `text` を Mattermost が自動で分割する場合も、本ツールは自分で分割し、Mattermost の自動の分割に頼らない（分割の位置の表示を付けるため）。Slack の上限は検証しない（2.2 の「対応の範囲」）。
-   記事が 1 つのメッセージに収まる場合は、1 つのメッセージで投稿する。このメッセージには、分割の位置の表示を付けない。
-   収まらない場合は、複数のメッセージに分割し、記事の順に投稿する。
    -   各メッセージには、全体の何番目かを示す表示（例: `(2/3)`。書式は設計で決める）を付ける。分割投稿が途中で止まった場合に、読み手が続きのメッセージが届いていないことに気づけるようにするためである。各メッセージの文字数は、この表示を含めて上限以内とする。
    -   分割の位置は、上限に収まる範囲にある改行の位置とする。上限に収まる範囲に改行がない場合（1 行が上限を超える場合）に限り、Unicode のコードポイントの境界で分割する。UTF-8 のバイト列の途中では分割しない。
    -   分割の位置の表示を取り除いて各メッセージを順に連結すると、投稿する文字列と一致する。分割によって文字を失ったり加えたりしない。
    -   空のメッセージや空白文字だけのメッセージは送らない（[CLAUDE.md](../../../CLAUDE.md)「Empty is an error」）。上の規則で分割すると空白文字だけのメッセージが生じる記事（上限を超える長さの空白文字の連なりを含む記事など）は、1 つのメッセージも投稿せずにエラーにする。
-   分割の数には上限を設ける（値は設計で決める）。分割の数が上限を超える記事は、1 つのメッセージも投稿せずにエラーにする。LLM の異常な出力によって大量のメッセージを投稿することを防ぐためである。
-   分割の数と各メッセージの内容は、最初のメッセージを送る前に確定させる。

**Acceptance Criteria**:
- **AC-07**: 上限以内の長さの記事を投稿すると、テスト用の送信先が受け取るリクエストは 1 つで、その `text` に分割の位置の表示はない。
- **AC-08**: 複数の行からなり、上限の 2 倍を超える長さの記事を投稿すると、テスト用の送信先は 3 つ以上のリクエストを記事の順に受け取る。各メッセージの文字数は、設計で定めた数え方で上限以内である。各メッセージは改行の位置で分割され、全体の何番目かを示す表示を含む。表示を取り除いて順に連結すると、投稿する文字列と一致する。空のメッセージや空白文字だけのメッセージはない。
- **AC-09**: 改行を含まずに上限を超える 1 行（日本語の文字と、U+10000 以上の文字（絵文字など）を含む）を投稿すると、コードポイントの境界で分割される。各メッセージの `text` は正しい UTF-8 であり、文字数は設計で定めた数え方で上限以内である。表示を取り除いて順に連結すると、投稿する文字列と一致する。
- **AC-10**: 投稿する文字列の長さが、設計で定めた数え方でちょうど上限の記事は 1 つのメッセージで投稿され、上限を 1 単位超える記事は 2 つのメッセージで投稿される。
- **AC-11**: 分割の数が上限を超える長さの記事と、上限を超える長さの空白文字の連なりを含む記事を投稿すると、いずれも送信前の分割の拒否（3.2）に対して `errors.Is` が真になるエラーになり、テスト用の送信先はリクエストを 1 つも受け取らない。

#### F-004: 送信と応答の検証

-   各メッセージは、Webhook URL への `POST`（本文は JSON）で送る。
-   次のメッセージは、前のメッセージの投稿の成功を確認してから送る。
-   投稿の成功は、Mattermost の公式文書が成功として定める応答（HTTP ステータス 200 で、本文が `ok`。設計で公式文書を確認する。[design_handoff.md](design_handoff.md) H-01）とする。Slack の成功の応答も同じ形である。それ以外の応答（200 以外のステータス、200 で本文が `ok` 以外）は、投稿の失敗とする。
-   HTTP の送信にはタイムアウトを設定する（値は設計で決める）。応答しない送信先に対しても、`Publish` は有限の時間で返る。
-   応答の本文は、上限（値は設計で決める）までしか読まない。上限を超える本文は、ステータスが 200 でも投稿の失敗とする。
-   リダイレクトには従わない。3xx の応答は投稿の失敗とし、`Location` の送信先にはリクエストを送らない。Go の HTTP クライアントはリダイレクト先へのリクエストに元の URL を `Referer` ヘッダーとして付けることがあり、Webhook URL がリダイレクト先に漏れるためである。
-   投稿を自動でリトライしない（2.3）。HTTP 429 も投稿の失敗とする。
-   `ctx` がキャンセルされた場合は、送信中のメッセージを中断し、残りのメッセージを送らない。`ctx` が既にキャンセルされている場合は、リクエストを送らない。
-   複数のメッセージを続けて送る場合のレート制限への配慮（メッセージの間に待つかどうか）は、設計で決める（[design_handoff.md](design_handoff.md) H-01）。

**Acceptance Criteria**:
- **AC-12**: 2 つに分割される記事を投稿し、テスト用の送信先が 1 つ目のメッセージにステータス 400・403・404・410・429・500 のいずれかを返すと、投稿は HTTP ステータスの失敗（3.2）に対して `errors.Is` が真になるエラーになり、エラーのメッセージはステータスコードを含む。2 つ目のメッセージは送られない。
- **AC-13**: テスト用の送信先が、ステータス 200 で本文が `ok` 以外（空、`OK`、`ok\n`、`{"ok":true}`）の応答を返すと、投稿は不正な応答（3.2）に対して `errors.Is` が真になるエラーになる。
- **AC-14**: テスト用の送信先が、ステータス 200 で上限を超える長さの本文を返すと、投稿は AC-13 と同じく不正な応答に対して `errors.Is` が真になるエラーになる。
- **AC-15**: テスト用の送信先が、別のテスト用のサーバを `Location` に指定した 3xx（301・302・307・308）を返すと、投稿は HTTP ステータスの失敗（3.2）に対して `errors.Is` が真になるエラーになり、別のサーバはリクエストを 1 つも受け取らない。
- **AC-16**: テスト用の送信先が応答を返さない場合、投稿は設計で定めたタイムアウトの後、有限の時間でエラーを返し、エラーは `context.DeadlineExceeded` に対して `errors.Is` が真になる（テストでは短いタイムアウトで構築する）。
- **AC-17**: キャンセル済みの `ctx` で投稿すると、テスト用の送信先はリクエストを受け取らず、エラーは `context.Canceled` に対して `errors.Is` が真になる。3 つに分割される記事の 1 つ目の投稿の成功後に `ctx` をキャンセルすると、2 つ目以降は送られず、エラーは `context.Canceled` に対して `errors.Is` が真になり、エラーから N = 3、k = 1 を取り出せる（F-005）。

#### F-005: 失敗の報告

-   投稿の失敗のエラーから、分割の数（N）と、投稿の成功を確認したメッセージの数（k）を、`errors.AsType` で取り出せる。分割しない場合も、N を 1 として同じ形で報告する。最初のメッセージを送る前の拒否（記事の拒否（F-002）、分割の数の上限の超過と空白文字だけのメッセージ（F-003）、F-006 の拒否）は、投稿を始めていないので、この形では報告しない。
-   エラーのメッセージは、N のうち k 個のメッセージを投稿したこと、k+1 番目のメッセージの投稿に失敗したこと、それより後のメッセージは送っていないことを示す。k+1 番目のメッセージは、失敗の理由によっては投稿された可能性がある（応答を受け取る前のタイムアウトなど）。そのため、エラーのメッセージは k+1 番目が投稿されていないとは断定しない。
-   失敗の原因は、3.2 で定めるエラーの分類として、`errors.Is` または `errors.AsType` で判別できる。`context.Canceled` と `context.DeadlineExceeded` は、元のエラーとして判別できる。
-   HTTP ステータスによる失敗のエラーは、ステータスコードと、応答の本文から読み取ったエラーの理由（Mattermost のエラーの応答の識別子など。何をどう読み取るかは設計で決める。[design_handoff.md](design_handoff.md) H-07）を含む。応答の本文は信頼できない値であり、CLI は既存の仕組みで制御文字をエスケープして表示する（0005 の F-008）。
-   エラーのメッセージは、Webhook URL もその一部（パスやその末尾）も含まない。`*url.Error` をそのまま返さない。この保証は、返すエラーの `Error()` の値だけでなく、`errors.Unwrap` でたどれるすべてのエラーの `Error()` の値に適用する。呼び出し元が `errors.AsType` で取り出した内側のエラーを出力しても、Webhook URL が現れないようにするためである。

**Acceptance Criteria**:
- **AC-18**: 3 つに分割される記事を投稿し、テスト用の送信先が 2 つ目のメッセージに 500 を返すと、エラーから N = 3、k = 1 を取り出せる。テスト用の送信先は 3 つ目のメッセージを受け取らない。エラーのメッセージは、3 つのうち 1 つを投稿したこと、2 つ目で失敗したことを示す。
- **AC-19**: Webhook URL のパスに特徴的な文字列を含めて構築した `SlackWebhookPublisher` で、次の各失敗を起こす。いずれの場合も、返されたエラーと、`errors.Unwrap` でたどれるすべてのエラーの `Error()` の値に、Webhook URL、そのパス、その末尾 8 文字のいずれも現れない。
    -   接続できない送信先（閉じたテスト用のサーバ）
    -   タイムアウト（AC-16 の条件）
    -   キャンセル（AC-17 の条件）
    -   200 以外のステータス（AC-12 の条件）
    -   上限を超える応答の本文（AC-14 の条件）
    -   3xx の応答（AC-15 の条件）
- **AC-20**: AC-12 の条件でテスト用の送信先が、ステータス 400 と、Mattermost のエラーの応答の形の JSON（`id` に `web.incoming_webhook.parse.app_error` を持つ）を返すと、エラーのメッセージはステータスコードと `web.incoming_webhook.parse.app_error` を含む。

#### F-006: メンションの通知の抑止

-   記事の内容（LLM の出力）にかかわらず、`SlackWebhookPublisher` の投稿によって、メンションの通知（`@channel`・`@all`・`@here`・`@everyone`、ユーザーへのメンション、ユーザーグループへのメンション）を発生させない。記事は字幕へのプロンプトインジェクションによって操作されうる（[security.md](../../dev/security.md) §6）。抑止しなければ、外部の第三者がチャンネルの全員への通知を起こせてしまうためである。
-   抑止の手段は、次の 2 つを併用する（利用者の決定）。一方が働かない場合（送信先が `silent` に対応しない場合や、検出の漏れ）にも、もう一方で通知を防ぐためである。
    -   **通知しない投稿として送る。** すべてのメッセージのペイロードに `silent`（真）を指定する。Mattermost の公式文書によれば、`silent` の投稿はチャンネルに表示されるが、デスクトップ・プッシュ・メールの通知を発生させない。メンションでない通常の投稿の通知も発生しない。Slack には `silent` に当たる項目がなく、Slack では効かない（2.2 の「対応の範囲」）。
    -   **通知の記法を含む記事を拒否する。** 記事が、Mattermost または Slack がメンションとして解釈する記法を含む場合は、最初のメッセージを送る前に拒否し、1 つのメッセージも投稿しない。対象は、Mattermost の `@channel`・`@all`・`@here`・`@ユーザー名`・`@グループ名`、Mattermost が Slack 互換として解釈する `<!channel>`・`<!here>`・`<!all>`、Slack の `<!everyone>`・`<@U…>`・`<!subteam^…>`・`@everyone` を含む。拒否する記法の正確な範囲と検出の手段（エスケープや見た目の似た文字などの変形を含めるか）は設計で決める（[design_handoff.md](design_handoff.md) H-02）。
-   拒否のエラーは、記事の内容を含まない。

**Acceptance Criteria**:
- **AC-21**: `Body` または `Title` に `@channel`・`@all`・`@here`・`@everyone`・`@someone`・`<!channel>`・`<!here>`・`<!all>`・`<!everyone>`・`<@U12345678>`・`<!subteam^S12345678>` のいずれかを含む `Article` を投稿すると、F-006 の拒否（3.2）に対して `errors.Is` が真になるエラーになり、テスト用の送信先はリクエストを 1 つも受け取らない。
- **AC-21a**: 有効な `Article` を投稿すると、テスト用の送信先が受け取るすべてのペイロードで、`silent` が JSON の `true` である。
- **AC-22**: 実装計画書に、次の手動確認を完了条件として含め、結果を記録する（F-009）。
    -   本番で使う Mattermost のサーバで、`silent` を指定した投稿が通知を発生させないこと。確認には、`@channel` と、確認する人のユーザー名へのメンションを含むメッセージを、Webhook に直接送る（本ツールは拒否するため）。
    -   確認に使ったサーバの版。

#### F-007: CLI での投稿先の選択

CLI の呼び出し形式は `yt2column [フラグ] <動画 URL>` のままとし、投稿先を選ぶフラグを加える。

| フラグ | 意味 |
|---|---|
| `--out <パス>` | 記事をファイルに書き出す（0005 の F-004〜F-006。振る舞いは変えない） |
| `--slack` | 記事を `SLACK_WEBHOOK_URL` の Slack 互換の Incoming Webhook（Mattermost など）に投稿する |

-   `--out` と `--slack` のうち、ちょうど 1 つを指定する。どちらもない場合と、両方がある場合は、使い方の誤りとして終了コード `2` で拒否する。
-   `--slack` を指定し、`SLACK_WEBHOOK_URL` が未設定の場合は、設定の誤りとして終了コード `2` で拒否する。標準エラー出力には変数の名前が現れ、値は現れない。`--out` を指定した場合は、従来どおり `SLACK_WEBHOOK_URL` を必須としない（設定されていれば検証する）。
-   終了コード `2` になるこれらの誤りは、0005 の F-005 と同じく、排他の取得、キャッシュの掃除、`yt-dlp` の起動、LLM API の呼び出しのいずれよりも前に検出する。
-   `--out` に固有の確認（キャッシュディレクトリの中を指すパスの拒否、事前確認。0005 の F-005・F-006）は、`--out` を指定した場合だけ行う。
-   実行の流れ（排他、掃除、パイプライン、キャッシュの削除）は、投稿先によらず 0005 の F-006 と同じとする。投稿に成功し、`--keep-cache` を指定していない場合は、その動画のキャッシュを削除する。投稿に失敗した場合（分割投稿の途中での失敗を含む）は、キャッシュを削除せず、終了コード `1` で終了する。
-   投稿の途中で SIGINT または SIGTERM を受けた場合は、0005 の F-006 と同じく `ctx` をキャンセルし、終了コード `1` で終了する。すべてのメッセージの投稿が完了した後に受けた場合は、終了コード `0` とする。
-   分割投稿の途中で失敗した場合、標準エラー出力に、N のうち k 個のメッセージを投稿したこと（F-005）と、再実行すると記事を生成し直して最初のメッセージから投稿し直すことを書く。
-   投稿に成功した場合、標準エラー出力に、Webhook に投稿したこと、メッセージの数、`Article.Model`、`Article.ModelVersion` を書く（エスケープは 0005 の F-008 と同じ）。Webhook URL は書かない。
-   `GODEBUG` が `http2debug=1` または `http2debug=2` を含む場合、Go の HTTP/2 の通信の記録はリクエストのパスを標準エラー出力に書くので、Webhook URL のパスも書かれうる。`--slack` を指定した場合、CLI は既存の API キーの警告（0005 の F-008）に加えて、Webhook URL が書かれうることを警告する。警告は、Webhook URL の値もその一部も含まない。
-   0005 の F-005 の「実行経路の一覧」に、次の経路を加える。

| 経路 | 終了コード |
|---|---|
| `--slack` での成功 | `0` |
| `--out` と `--slack` のどちらもない、または両方がある | `2` |
| `--slack` で `SLACK_WEBHOOK_URL` が未設定 | `2` |
| `--slack` での投稿の失敗（最初のメッセージでの失敗） | `1` |
| `--slack` での分割投稿の途中での失敗 | `1` |
| `--slack` での投稿中の SIGINT・SIGTERM | `1` |

**Acceptance Criteria**:
- **AC-23**: 有効な環境と `--slack <有効な動画 URL>` で実行し、各段階が成功すると、終了コードは `0` で、テスト用の送信先が記事のメッセージを受け取る。標準出力は空で、標準エラー出力には Webhook に投稿したこととメッセージの数が現れる。字幕のキャッシュは、`--keep-cache` を付けない場合は削除され、付けた場合は残る。
- **AC-24**: 次の呼び出しは、終了コード `2` になり、標準エラー出力に誤りの内容を書く。いずれの場合も、排他のためのファイルを作らず、キャッシュディレクトリを変更せず、`yt-dlp` を起動せず、LLM API を呼ばず、Webhook に送らない。
    -   `--out` と `--slack` のどちらもない
    -   `--out` と `--slack` の両方がある
    -   `--slack` を指定し、`SLACK_WEBHOOK_URL` が未設定（標準エラー出力に `SLACK_WEBHOOK_URL` の名前が現れる）
- **AC-25**: `--slack` で実行し、テスト用の送信先が 3 つに分割される記事の 2 つ目に失敗を返すと、終了コード `1` で終了する。標準エラー出力は、失敗した段階が投稿であること、3 つのうち 1 つを投稿したこと、再実行すると最初から投稿し直すことを示す。その動画のキャッシュは残る。
- **AC-26**: 本節の表で加えたすべての経路について、0005 の AC-44 の (a)〜(f) が成り立つ（(f) の `--out` のパスの条件は `--out` を指定した経路にだけ適用する）。(d) の信頼できない文字列には、投稿の失敗の経路で送信先が返す応答の本文（テスト用の送信先が、エスケープシーケンスと、偽の行を始める改行を含む本文を返す）を加える。加えて、`GODEBUG` が `http2debug` を含まない環境で `SLACK_WEBHOOK_URL` に特徴的な値を設定して実行すると、Webhook URL の値もその末尾 8 文字も、標準出力・標準エラー出力のいずれにも現れない。
- **AC-27**: `-h`・`--help` の使い方は `--slack` を含み、Mattermost に投稿できることを示す。
- **AC-28**: `--slack` を指定し、`GODEBUG` に `http2debug=1` または `http2debug=2` を設定して実行すると、LLM API の呼び出しより前に、Webhook URL が標準エラー出力に書かれうることを示す警告が現れる。警告は Webhook URL の値もその末尾 8 文字も含まない。`--out` を指定した場合は、この Webhook URL の警告を出さない。

#### F-008: 実際の Webhook を使う統合テスト

実際の Mattermost の Incoming Webhook に投稿する統合テストを設ける。0005 で本タスクに委ねられた、testdata の字幕から投稿までの通しの統合テストもここで扱う（0005 の要件定義書 2.3）。Slack の Webhook を使う統合テストは設けない（2.2 の「対応の範囲」）。

-   次の 2 つのテストを設ける。
    -   `SlackWebhookPublisher` の統合テスト: 固定の `Article`（LLM を呼ばない）を、テスト用の Webhook に投稿する。記事は、2 つ以上のメッセージに分割される長さとする。
    -   CLI の統合テスト: 0005 の F-009 と同じく testdata の字幕から、実際の DeepSeek API で記事を生成し、`--slack` でテスト用の Webhook に投稿する。CLI と同じ組み立てで実行する。
-   いずれのテストも `//go:build integration` のビルドタグを持ち、`make test` と CI では実行しない。専用の make のターゲットだけがオプトインの変数を値 `1` でエクスポートし、値がちょうど `1` のときだけ実行する。それ以外（未設定、空、`0`、`true` など）の場合はスキップする（[security.md](../../dev/security.md) §2 の既存の取り決め）。変数とターゲットの名前、既存のターゲットとの関係は設計で決める（[design_handoff.md](design_handoff.md) H-05）。
-   Webhook URL は、テスト専用の環境変数 `YT2COLUMN_TEST_SLACK_WEBHOOK_URL` からだけ読む。本番の `SLACK_WEBHOOK_URL` は、設定されていても読まない。テスト用の Webhook には、本番とは別の、Mattermost のテスト用のチャンネルに投稿するものを用意する。オプトインの変数の値が `1` で、テスト用の Webhook URL がない場合は、スキップせずに失敗する。
-   CLI の統合テストは、テスト用の Webhook URL を、CLI の組み立てに与える環境の中で `SLACK_WEBHOOK_URL` という名前で渡す。プロセスの環境変数は変更しない。DeepSeek の API キーの扱いは 0005 の F-009 と同じとする。
-   テストの出力（ログ・失敗メッセージ・スキップの理由）に、Webhook URL も API キーも、それらの一部も書かない。`GODEBUG` が `http2debug=1` または `http2debug=2` を含む場合、テストは Webhook に送る前に失敗する。

**Acceptance Criteria**:
- **AC-29**: 各統合テストは、オプトインの変数が未設定または値が `1` 以外の環境ではスキップし、Webhook に送らない。値が `1` で `YT2COLUMN_TEST_SLACK_WEBHOOK_URL` がない環境では、Webhook に送らずに失敗する（本番の `SLACK_WEBHOOK_URL` だけが設定されている環境でも、それを使わない）。値が `1` で `GODEBUG` が `http2debug=1` または `http2debug=2` を含む環境では、Webhook に送らずに失敗する。
- **AC-30**: `SlackWebhookPublisher` の統合テストを make のターゲットで実行すると、エラーにならずに成功する。Mattermost のテスト用のチャンネルには、2 つ以上のメッセージが、全体の何番目かを示す表示とともに順に投稿される。
- **AC-31**: CLI の統合テストを make のターゲットで実行すると、CLI の組み立ての終了コードが `0` で、`yt-dlp` の代わりに指定したトリップワイヤ（§6）は起動されず、`2tcCWM-sRBw` のキャッシュのエントリは残っていない。テスト用の Webhook URL もテスト用の API キーも、それらの末尾 8 文字も、標準出力・標準エラー出力とテストの出力のいずれにも現れない。
- **AC-32**: 統合テストは `make lint`（`go vet -tags integration`）でコンパイルされ、`make test` では実行されない。

#### F-009: 実際の Mattermost での表示の手動確認

-   実装計画書に、次の手動確認を完了条件として含める。確認には、Mattermost のテスト用のチャンネルの Webhook を使う。使った記事（動画 URL または固定の記事）と結果を実装計画書に記録する。
    -   1 つのメッセージに収まる記事と、分割される記事を投稿し、見出し・段落・リスト・リンク・日本語が Markdown として表示されること、分割した場合は全体の何番目かを示す表示とともに記事の順に並ぶことを確かめる。
    -   AC-22 の通知の確認。
-   Slack での表示の確認は行わない（2.2 の「対応の範囲」）。

**Acceptance Criteria**:
- **AC-33**: 実装計画書に、上記の手動確認が完了条件として含まれ、結果が記録されている。

#### F-010: 文書

-   README に、次を記す。
    -   冒頭の説明（Slack への投稿は未実装という記述）の更新
    -   `--slack` の使い方。`--out` と `--slack` のどちらか一方が必須であること
    -   `SLACK_WEBHOOK_URL` の用途と形式（Mattermost の Webhook URL を設定できること）
    -   対応の範囲（Mattermost は検証済み、Slack は best effort）
    -   長い記事は分割して投稿されること
    -   投稿は通知なしで行われること（Mattermost の場合）。メンションの記法を含む記事は投稿されないこと
    -   分割投稿の途中で失敗した場合に、再実行すると最初から投稿し直されること（重複が起こりうること）
    -   統合テストの実行方法
-   [project_overview.md](../../dev/project_overview.md) の、投稿先を Slack とする記述（概要、`Publisher` の初期実装、「前提・制約」の `markdown` ブロックと上限値の記述、「設定（環境変数）」の表の `SLACK_WEBHOOK_URL`）を、Slack 互換の Incoming Webhook（検証の対象は Mattermost）と、設計で確認した上限値と出典に改める。`SLACK_WEBHOOK_URL` には、`--slack` を指定した場合は必須であることと、Mattermost の URL を受理することを加える。
-   [package_reference.md](../../dev/developer_guide/package_reference.md) の `internal/publisher` の責務に `SlackWebhookPublisher` を加え、`internal/config` の Webhook URL の規則の記述を改める。
-   [security.md](../../dev/security.md) に、次を加える。
    -   §2: テスト用の Webhook URL（`YT2COLUMN_TEST_SLACK_WEBHOOK_URL`）を使う統合テストの例外と、統合テストの表への追加。`--slack` で `GODEBUG` の `http2debug` を設定した場合に Webhook URL が書かれうること。
    -   §3: Webhook への送信でリダイレクトに従わないこと、自動でリトライしないこと。
    -   §6: 記事によるメンションの通知を抑止すること（F-006）。

**Acceptance Criteria**:
- **AC-34**: 上記の各文書に、上記の内容が記載されている（静的な確認）。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

送信先の応答と、LLM の出力である記事から作る投稿する文字列は、信頼できない入力である。各境界で受理する形と、拒否したときのエラーの分類を次に定める。分類の名前は仮称であり、番兵エラーの名前と置き場所、上限値、拒否する入力の検出の手段は設計で決める（[design_handoff.md](design_handoff.md) H-07）。

| 境界 | 受理する形 | 上限 | 拒否時の分類 | 主な AC |
|---|---|---|---|---|
| 応答のステータス | `200`。3xx のリダイレクトには従わない（F-004） | - | HTTP ステータスの失敗 | AC-12・AC-15 |
| 応答の本文（ステータス `200`） | ちょうど `ok` | 読む本文の長さ（値は設計で決める） | 不正な応答 | AC-13・AC-14 |
| 投稿する文字列の分割（F-003） | 分割の数が上限以内で、空白文字だけのメッセージを生じない | 分割の数（値は設計で決める） | 送信前の分割の拒否 | AC-11 |
| 投稿する文字列のメンションの記法（F-006） | 設計で定める記法を含まない | - | メンションの記法の拒否 | AC-21 |

-   **HTTP ステータスの失敗**（仮称 `ErrHTTPStatus`）: `200` 以外のステータスの応答（3xx・429 を含む）。ステータスコードと、応答の本文から読み取ったエラーの理由を含む（F-005）。
-   **不正な応答**（仮称 `ErrInvalidResponse`）: ステータスが `200` で、本文が `ok` 以外の応答と、本文が上限を超える応答。どちらも送信先が成功を示さなかった応答であり、呼び出し元が扱いを変える理由がないので、同じ分類とする。
-   **通信の失敗**（仮称 `ErrTransport`）: 応答を受け取れなかった失敗（接続できない場合など）。元の原因の内容を含むが、Webhook URL を含まない（F-005）。タイムアウトとキャンセルは、`context.DeadlineExceeded`・`context.Canceled` で判別する（AC-16・AC-17）。
-   **送信前の分割の拒否**（仮称 `ErrUnsplittable`）: 分割の数の上限の超過と、空白文字だけのメッセージを生じる記事（F-003）。1 つのメッセージも送らない。
-   **メンションの記法の拒否**（仮称 `ErrMention`）: F-006 の拒否。1 つのメッセージも送らない。上記のいずれとも、`writer.ErrInvalidArticle`（F-002）とも別の分類とする。

**Acceptance Criteria**:
- **AC-35**: HTTP ステータスの失敗・不正な応答・通信の失敗・送信前の分割の拒否・メンションの記法の拒否・`writer.ErrInvalidArticle` は、`errors.Is` または `errors.AsType` で相互に区別できる。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   投稿の処理（分割、送信）は、LLM API の呼び出しに比べて無視できる時間で終わればよい。性能のための仕組みは設けない（[CLAUDE.md](../../../CLAUDE.md)「Performance」）。

### 4.2. セキュリティ (Security)
-   Webhook URL は `secret.Secret` で受け取り、送信先への HTTP リクエスト以外のどの出力にも書かない（F-005・F-007）。
-   Webhook URL は `https` の URL だけを受理し、リダイレクトに従わない（F-001・F-004）。
-   応答の本文の読み込みに上限を設ける（F-004。[security.md](../../dev/security.md) §3）。
-   LLM の出力によってメンションの通知を発生させない（F-006）。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   HTTP の送信にタイムアウトを設定する（F-004）。
-   投稿を自動でリトライしない。部分的な失敗は、投稿済みの範囲とともに報告する（F-005）。

### 4.4. 互換性 (Compatibility)
-   macOS と Linux で動作すること（0005 と同じ）。
-   `SLACK_WEBHOOK_URL` の規則を広げること（F-001）で、これまで拒否していた値の一部を受理するようになる。拒否していた値を受理するだけなので、これまで動いていた設定が動かなくなることはない。

### 4.5. 保守性 (Maintainability)
-   投稿先を追加するときに、`ArticleWriter`・パイプラインを変更せずに済むこと。
-   投稿先の選択は、文字列の内容（`--out` が空かどうかなど）から推し量らず、型で表す（[CLAUDE.md](../../../CLAUDE.md)「Declare, don't infer」。[design_handoff.md](design_handoff.md) H-04）。
-   Slack に正式に対応するときに、フラグと環境変数の名前を変えずに済むこと。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」に従う。ただし、投稿先を Slack とし `markdown` ブロックを使うとする記述は、本タスクで改める（F-010）。
-   標準ライブラリだけで実装する（`net/http`・`encoding/json`）。Mattermost や Slack のクライアントのライブラリなどの外部のモジュールを追加しない。
-   `FilePublisher` と、`--out` を指定した場合の CLI の振る舞いは変更しない。ただし、`SLACK_WEBHOOK_URL` の規則の変更（F-001）は、`--out` を指定した場合にも及ぶ。`internal/job` と `cmd/yt2column` の変更が必要な場合は、`02_architecture.md` で理由とともに示す。
-   ユニットテストは実際の Webhook を呼ばない。`net/http/httptest` を使う。

## 6. 用語集 (Glossary)

-   **Incoming Webhook**: Mattermost や Slack が発行する、特定のチャンネルにメッセージを投稿するための URL（Mattermost は `https://<サーバ>/hooks/<キー>`）。URL を知っていれば誰でも投稿できるため、URL そのものが秘密情報である。
-   **Slack 互換の Incoming Webhook**: Slack の Incoming Webhook と同じ形のペイロード（`text` など）を受け付ける Incoming Webhook。Mattermost の Incoming Webhook はこれに当たる（完全な互換ではない）。
-   **`silent`**: Mattermost の Incoming Webhook のペイロードの項目。真にすると、投稿はチャンネルに表示されるが、デスクトップ・プッシュ・メールの通知を発生させない。Slack にはない。
-   **分割の位置の表示**: 分割投稿の各メッセージに付ける、全体の何番目かを示す表示（例: `(2/3)`）。
-   **トリップワイヤ**: テストで `yt-dlp` の代わりに指定する実行ファイル。起動されたことを記録する（0005 の用語集）。
-   用語は [translation_glossary.md](../../translation_glossary.md) と統一する。
