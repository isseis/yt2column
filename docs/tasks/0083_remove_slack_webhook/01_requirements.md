# 要件定義書：Slack の Webhook への投稿の機能を廃止し、メンションの無効化をクリップボードへのコピーに移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、Slack 互換の Incoming Webhook への投稿の機能（以下、投稿の機能）を廃止し、記事によるメンションへの対策を、ビューアの Markdown のコピーに移すための要件を定義する。

**背景:**

拡張のビューアには、記事を Webhook へ投稿する操作と、記事を Markdown 形式でクリップボードへコピーする操作がある。投稿の代わりに、利用者が Slack や Mattermost のチャンネルを開いて、コピーした記事を貼り付ければよい。親 issue [#769](https://github.com/isseis/yt2column/issues/769) は、拡張だけの構成へ移る方針を決めた。そこへ移る前に、秘密（Webhook の URL）と、任意の送り先への `host_permissions` を要する機能をなくしておく。

ただし、いまのコピーは、サーバが返す表示用の Markdown（`publisher.DisplayMarkdown`、`internal/publisher/display_render.go:15`）をそのまま書く（`extension/src/viewerApp.ts:548`）。メンションの無効化は、投稿の経路だけにある（`docs/dev/security.md` §6「信頼できないテキストの取り扱い」の 3 つの層、`internal/publisher/slack_render.go:116`・`internal/publisher/slack_message.go:220`）。投稿の機能だけを消すと、メンションへの対策がなくなる。YouTube のハンドルは `@` で始まるので、概要欄やチャンネル名から `@名前` が記事に入りうる。

対応 issue: [#770](https://github.com/isseis/yt2column/issues/770)（親: [#769](https://github.com/isseis/yt2column/issues/769)、関連: [#771](https://github.com/isseis/yt2column/issues/771)・[#772](https://github.com/isseis/yt2column/issues/772)・[#773](https://github.com/isseis/yt2column/issues/773)）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。廃止する機能、廃止の後にサーバ・ホスト・拡張が返すもの、コピーした文章のどの `@` をどう変えるか、利用者に何を示すかである。無効化をどの部品で行うか（サーバか拡張か）、数をどう拡張に渡すか、規則のコードをどこに置くか、テストの手段は、設計（`02_architecture.md`）で決める。設計へ申し送る事項は [design_handoff.md](design_handoff.md) にある。

**issue #770 で決めたこと:**

-   **廃止するもの:** サーバの投稿の操作とその API、`SlackWebhookPublisher`・`PrepareSlackMessages`・`PostMessages`、`internal/slackwebhook`、`SLACK_WEBHOOK_URL` の設定、`make test-integration-slack`、拡張のビューアの投稿の操作。メンションへの対策のうち、Webhook に固有の層（`silent` の付与、メッセージの分割）も廃止する。利用者が手で貼り付ける方法で置き換えられるためである。
-   **コピーへ移す規則:** コピーでは、対象の `@`（`docs/dev/security.md` §6 の定義）を、`@` の直後に U+200B を入れていつも無効化する。対象は見出し行と、本文のコードスパンとフェンス付きコードブロックの外である。語の途中の `@` と、直前が `/` の `@` は変えない。コードの範囲を判定できない本文では、すべての対象の `@` を無効化する。
-   **変えないもの:** 表示（ビューア）と `--out` のファイルの内容は変えない。
-   **実機での確かめ:** 貼り付けた `@channel`・`@here`・`@名前` がメンションになるか、U+200B が送信の後も残って無効化が効くか、`<!channel>`・`<@…>`・文字参照・`<a|b>` の形が貼り付けで特別な意味を持つかを、実機で確かめる。その結果で、いまの拒否の規則（§6 の層 (3)）をコピーに残すかを決める。
-   **文書:** ADR-0011 を廃止し、ADR-0028 の投稿に関する決定と、ADR-0027 の投稿の状態を改める。`security.md` の該当の項を、コピーの経路の規則として書き直す。`production_profile.md`・`project_overview.md`・README（と `README.en.md`）・`development.md`・`CLAUDE.md` を改める。
-   **テストの削除:** テストの削除を含むので、coverage-compare の手続き（`CLAUDE.md`「Testing Strategy」）に従う。

**要件定義の際に決めたこと（2026-11-11）:**

-   **進め方は full とする。** 簡易プロセスの除外に当たる変更と判断した。`docs/dev/security.md` の規則（§2 の秘密、§3 の Webhook への通信、§6 のメンションの扱い、§8 のサーバの投稿）を書き換えるためである（`requirements_process.md`「Simple Process」の除外の 1 つ目）。人もこの判断のとおり full を選んだ。ADR の判断: ADR を作らない（`Process` 行と本節に記録する）。
-   **実機での確かめを、要件を書く前に行った。** 2026-11-11 に、利用者が Mattermost のチャンネルに次の並びを貼り付けて送信し、表示を確かめた。Mattermost の版は記録していない。Slack では確かめていない（いまの方針で Slack は best effort であり、確かめの対象ではない。`.claude/commands/_context.md`「Target client environments」）。

    | 貼り付けた並び | 結果 |
    |---|---|
    | `@channel`・`@here`・実在の利用者の `@名前` | メンションになった |
    | 上の 3 つの `@` の直後に U+200B を入れたもの | メンションにならず、ただの文字列になった |
    | `＠channel`（全角）、`＠` の直後に U+200B を入れたもの | ただの文字列になった |
    | コードスパンの中の `` `@channel` ``、フェンス付きコードブロックの中の `@channel` | メンションにならなかった |
    | `https://example.com/@channel` | 全体で 1 つのリンクになり、メンションにならなかった |
    | `<!channel>`・`<@名前>`・`&#64;channel`・`&lt;!channel&gt;` | ただの文字列になった |
    | `<https://example.com\|click>` | `about:blank#blocked` へのリンクになり、メンションにならなかった |

    ADR の判断: [ADR-0043](../../dev/adr/0043-copy-mention-neutralization.md)（クリップボードへのコピーで記事のメンションを無効化し、コピーは拒否しない）を作り、確かめた結果を背景に置く。
-   **拒否の層 (3) をコピーに持ち込まない。** コピーは、記事の内容によって拒否しない。いまの投稿が拒否する並び（コードの中の対象の `@`、直前が `/` の対象の `@`、`<!`・`<@`、文字参照、`<a|b>` の形）は、コピーした文章にそのまま残す（F-002）。上の確かめで、どの並びも貼り付けた後にメンションにならなかったためである。拒否すると、利用者はコピーできず、保存したファイルなどから手でコピーするしかなくなる。コードの範囲を判定できない本文の扱いは issue のとおりで、拒否せずにすべての対象の `@` を無効化する。ADR の判断: ADR-0043 に含める。
-   **見出しの下のチャンネル名の行も、無効化の対象にする。** コピーする文章（表示用の Markdown）では、チャンネル名はタイトルの見出しの下の独立した行にある（`internal/publisher/display_render.go:31`）。いまの投稿では、チャンネル名は見出し行の中にあり、無効化の対象である（`internal/publisher/slack_render.go:68`）。チャンネル名は外部由来の値で、YouTube のハンドルのように `@` を含みうるので、行が分かれても対象から外さない。ADR の判断: ADR-0043 に含める。
-   **無効化した数を、コピーの結果の表示で知らせる。** 1 個以上の `@` を無効化したときだけ、コピーの結果の通知にその数を示す（F-003）。0 個のときは、いまの通知のままにする。貼り付けた文章が見た目と違う文字を含むことを、利用者が知れるようにするためである。ADR の判断: ADR-0043 に含める。
-   **サーバの設定ファイルに残った `SLACK_WEBHOOK_URL` の行は、知らないキーとして拒否する。** サーバは起動しない（F-001）。廃止した `YT2COLUMN_MODEL` と同じ扱いである（`internal/config/serverfile.go:71`、`internal/config/serverfile_test.go:140`）。黙って無視すると、設定の誤りを隠し、`CLAUDE.md`「Reject, don't normalize」に反するためである。利用者には README で行の削除を案内する。ADR の判断: [ADR-0018](../../dev/adr/0018-server-config-file.md) を改める。
-   **投稿の廃止に合わせて ADR を改める。** [ADR-0011](../../dev/adr/0011-webhook-publishing.md) を廃止し、代わりの ADR を ADR-0043 とする。[ADR-0027](../../dev/adr/0027-article-job-api.md) は投稿の操作と投稿の状態を、[ADR-0028](../../dev/adr/0028-no-automatic-resend.md) は投稿の冪等性と Webhook への送り直しの決定を除く。Webhook や投稿に触れる [ADR-0005](../../dev/adr/0005-server-architecture.md)・[ADR-0006](../../dev/adr/0006-threat-model.md)・[ADR-0008](../../dev/adr/0008-launch-path.md)・[ADR-0009](../../dev/adr/0009-viewer-opening.md)・[ADR-0010](../../dev/adr/0010-extension-stores-nothing.md)・[ADR-0022](../../dev/adr/0022-cost-estimate.md)・[ADR-0031](../../dev/adr/0031-secret-closure.md)・[ADR-0034](../../dev/adr/0034-template-allowlist.md) も、決定から Webhook と投稿を除く。既存の ADR の決定を変えるので、どれも主要な決定である（`CLAUDE.md`「Design decisions (ADR)」）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   Webhook の URL という秘密と、それを使う送信の経路をなくす。
-   コピーした記事を Slack や Mattermost に貼り付けても、記事の中の `@channel`・`@here`・`@名前` などがメンションにならないようにする。
-   コピーは、記事の内容によって失敗させない。
-   記事の表示・保存・`--out` のファイルは、いまのまま保つ。

### 2.2. スコープ (In Scope)

-   投稿の機能の廃止（F-001）。サーバの投稿の操作・投稿のジョブ・投稿の状態・Webhook の設定と送信、ホストの `publish` の操作、拡張のビューアの投稿の操作と表示、`make test-integration-slack` とそのテスト専用の変数。
-   ビューアのコピーでのメンションの無効化（F-002）。
-   コピーの結果の通知に、無効化した数を示すこと（F-003）。
-   文書の改訂（F-004）。`docs/dev/security.md`・`docs/dev/production_profile.md`・`docs/dev/project_overview.md`・`docs/dev/guides/development.md`・`README.md`・`README.en.md`・`CLAUDE.md`・`docs/translation_glossary.md` と、ほかに投稿の機能を説明する文書（`docs/dev/conventions/package_reference.md` など）。
-   投稿の機能だけが使うテストの削除と、その coverage-compare による確かめ。

### 2.3. スコープ外 (Out of Scope)

-   **`.md` で保存するファイルの無効化。** 保存するファイルは `--out` のファイルと同じ中身で（`extension/src/viewerApp.ts:535`）、issue が変えないと決めたものである。保存したファイルの中身を貼り付けると、メンションになりうる。これは受け入れる。
-   **Slack での確かめ。** Slack は best effort で、確かめの対象ではない（§1「要件定義の際に決めたこと」）。
-   **localhost サーバと Native Messaging のホストの撤去、WASM への移行。** [#771](https://github.com/isseis/yt2column/issues/771)・[#772](https://github.com/isseis/yt2column/issues/772)・[#773](https://github.com/isseis/yt2column/issues/773) で扱う。
-   **ほかの投稿先や、新しい送信の経路。**
-   **前の版の拡張・ホスト・サーバとの互換。** 互換の仕組みは作らない（[ADR-0027](../../dev/adr/0027-article-job-api.md)）。前の版の拡張が投稿の操作を送れば、F-001 の拒否になる。
-   **CLI の変更。** CLI はすでに投稿せず、`SLACK_WEBHOOK_URL` を読まない（[0062 要件定義書](../0062_remove_cli_slack/01_requirements.md)）。CLI の振る舞いは変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 用語

-   **対象の `@`:** `docs/dev/security.md` §6 の定義による。`@` の直後に名前の文字（文字・数字・`_`・`.`・`-`）が続き、かつ `@` が先頭にあるか、`@` の直前が文字でも数字でもない `@` である。`＠` などの見た目の似た文字を含む。いまの判定は `internal/publisher/slack_message.go:394` にある。語の途中の `@`（`Tech@Work`、`user@example.com`）は対象の `@` ではない。
-   **無効化:** 対象の `@` の直後に U+200B（ゼロ幅スペース）を 1 つ入れること。`@` そのもの（見た目の似た文字を含む）は変えない。
-   **コードの範囲:** 本文のコードスパンとフェンス付きコードブロック。いまの判定は `writer.ProseRanges`（`internal/writer/markdown.go:49`）で、判定できない本文（対にならないバッククォートなど）では範囲がないことを返す。
-   **表示用の Markdown:** 状態の取得の応答の `markdown`。タイトルの見出しの行、チャンネル名があればその行、本文の順に並ぶ（`internal/publisher/display_render.go:28`）。
-   **コピーする文章:** ビューアの「Markdown をコピーする」がクリップボードに書く文字列。

### 3.2. 機能一覧

#### F-001: 投稿の機能の廃止

サーバ・ホスト・拡張から、Webhook への投稿の機能をなくす。記事の生成・状態の取得・表示・保存・コピーは残る。

**Acceptance Criteria**:

-   **AC-01**: ビューアは、投稿のボタン（いまの「Slack に投稿する」）、投稿の状態とその理由の表示、Webhook が設定されていないことの表示を持たない。記事の表示・「.md で保存する」・「Markdown をコピーする」は、いまの条件で使える。
-   **AC-02**: サーバは、`POST /jobs/{key}/publish` のリクエストを、ほかの受け付けないリクエストのターゲットと同じく `not_found` で拒否する。本文を読まず、記事のキャッシュを変えず、ネットワークへ何も送らない（いまのターゲットの拒否の規則は `docs/dev/security.md` §8.2「ヘッダの検査はパスとメソッドだけ」）。
-   **AC-03**: 状態の取得の応答の差分は、最上位の `webhookConfigured` のメンバと、`job` の `publish` のメンバ（いまは `done` の応答だけが持つ）をなくすことに限る（いまの応答の形は `internal/server/status.go:24`）。
-   **AC-04**: ホストは、`op` が `publish` のメッセージを、ほかの受け付けない `op` と同じく、サーバへ何も送らずに `protocol_error` で返す（いまの規則は [ADR-0027](../../dev/adr/0027-article-job-api.md)）。
-   **AC-05**: サーバの設定ファイルに `SLACK_WEBHOOK_URL` をキーとする行があれば、値が空でも、サーバは起動しない。拒否は知らないキーの拒否（`ErrConfigFileFormat`、その行の番号と固定の理由）で、エラーとログは行の値を含まない。`#` で始まるコメントの行は、`SLACK_WEBHOOK_URL` を含んでいても受理する。
-   **AC-06**: `yt2column-server config init` が作る雛形は、`SLACK_WEBHOOK_URL` を含む行を持たない。
-   **AC-07**: サーバは、送る前の検査（いまの `PrepareSlackMessages`）を記事の完了の判定に使わない。完了の時点の検査（`CheckPublishable` など、いまの `article_invalid`・`article_too_large`）を通る記事は、`done` になり、表示・保存・コピーできる。メンションを含む記事も含む。
-   **AC-08**: サーバは、終了の合図を受けたとき、投稿のジョブの終わりを待たない（いまは最長でおよそ 5 分待つ。`README.md:314`）。生成の取り消しなど、ほかの終了の振る舞いは変えない。起動のログの `listening` の行の差分は `webhookConfigured` をなくすことに限り、Webhook が設定されていないことの警告の行（いまは `cmd/yt2column-server/run.go:223`）は出さない。
-   **AC-09**: `make test-integration-slack` のターゲットは存在しない。`YT2COLUMN_TEST_SLACK_WEBHOOK_URL` と `YT2COLUMN_SLACK_INTEGRATION` を読むコードとターゲットもない。

#### F-002: コピーでのメンションの無効化

ビューアの「Markdown をコピーする」は、表示用の Markdown の対象の `@` を無効化した文章を書く。記事の内容によってコピーを拒否しない。

**Acceptance Criteria**:

-   **AC-10**: コピーする文章と表示用の Markdown との差分は、U+200B を入れることに限る。U+200B は、AC-11 と AC-13 が定める `@` の直後にだけ入り、1 つの `@` に 1 つだけ入る。
-   **AC-11**: タイトルの見出しの行と、チャンネル名の行では、すべての対象の `@` を無効化する。本文では、コードの範囲の外の対象の `@` を無効化する。ただし、どの行でも、直前が `/` の対象の `@` は無効化しない（URL を壊さないため）。
-   **AC-12**: 本文のコードの範囲の中の `@`、語の途中の `@`、直前が `/` の `@` は変えない。
-   **AC-13**: コードの範囲を判定できない本文では、本文のすべての対象の `@`（直前が `/` のものを除く）を無効化する。
-   **AC-14**: いまの投稿が拒否する並び（コードの中の対象の `@`、直前が `/` の対象の `@`、`<!`・`<@`、文字参照、`<a|b>` の形）を含む記事も、コピーできる。それらの並びは、AC-10〜AC-13 のとおりに書かれ、そのほかは変わらない。
-   **AC-15**: 対象の `@` を無効化しない記事（対象の `@` がないか、すべてがコードの中か直前が `/`）では、コピーする文章は表示用の Markdown とバイト単位で同じである。
-   **AC-16**: ビューアに表示する記事、「.md で保存する」が保存するファイル、CLI の `--out` のファイルは、この task の前と比べて変わらない。それぞれの差分は空である。
-   **AC-17**: 実装の後、ビューアから、タイトル・チャンネル名・本文のコードの外に `@channel`・`@here`・実在の利用者の `@名前` を含む記事をコピーし、Mattermost のチャンネルに貼り付けて送信すると、どれもメンションにならない。確かめた日と Mattermost の版を記録する（手動の確かめ）。

#### F-003: コピーの結果の表示

コピーの結果の通知で、無効化した `@` の数を知らせる。

**Acceptance Criteria**:

-   **AC-18**: コピーに成功し、無効化した `@` の数 N が 1 以上のとき、通知は「Markdown をコピーしました。メンションにならないよう、N 個の @ の後ろに見えない文字を入れました。」（英語の表示では同じ内容の英文）を示す。N は、コピーする文章に入れた U+200B の数である。
-   **AC-19**: コピーに成功し、N が 0 のとき、通知はいまの「Markdown をコピーしました。」のままである。コピーに失敗したときは、いまの「Markdown をコピーできませんでした。」を示し、数を示さない。
-   **AC-20**: AC-18 の通知は、いまのコピーの通知と同じく、フォーカスを動かさずに支援技術へ伝わる（WCAG 2.2 の 4.1.3。いまの通知の領域は `extension/static/viewer.html:29` の `role="status"`）。通知は英語と日本語のメッセージのカタログの両方にある。

#### F-004: 文書の改訂

**Acceptance Criteria**:

-   **AC-21**: `docs/dev/security.md` は、投稿の機能の規則（§2 の Webhook の URL、§3 の Webhook への送信、§6 のメンションの 3 つの層、§8 の投稿の操作と投稿のジョブ）を持たず、§6 にコピーの経路の無効化の規則（F-002 の内容）を持つ。手動で読んで確かめる。
-   **AC-22**: §2.2 に挙げたほかの文書は、投稿の機能がない今の振る舞いを説明し、コピーの無効化に触れる必要のあるところ（README のビューアの説明）では F-002 と F-003 の振る舞いを説明する。`README.en.md` は `README.md` と同期している（`TestReadmeSyncMarker`）。文書の値や構造を確かめる既存のテスト（`CLAUDE.md`「Development Notes」の text-sync のガード）は、改めた文書と合う。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。コピーの無効化は記事 1 本の文字列の変換で、記事の生成（LLM の呼び出し）に比べて無視できる。

### 4.2. セキュリティ (Security)

`docs/dev/security.md` を適用する（`.claude/commands/_context.md`「Conditional-guide trigger」の、Webhook の URL を扱うことと、信頼できないテキストを出力に入れることに当たる）。

-   **信頼できない入力:** コピーの入力は表示用の Markdown で、LLM の出力と、外部由来のタイトル・チャンネル名を含む。字幕と動画メタデータへのプロンプトインジェクションで、記事に `@channel` などを入れられることを前提にする。
    -   受け付ける形: 状態の取得の `done` の応答の `markdown`。応答の検査は、いまの拡張の規則のまま変えない。
    -   拒否: コピーは記事の内容によって拒否しない（§1「要件定義の際に決めたこと」）。記事の値の検査は、いまの完了の時点の検査（`CheckPublishable` など。`article_invalid`・`article_too_large`）が行い、これは変えない。
    -   標準ライブラリが黙って直す入力: 本文が正しい UTF-8 でない場合の扱いは、設計で確かめる（[design_handoff.md](design_handoff.md) H-03）。無効化が、記事の文字を U+FFFD などに黙って置き換えてはならない。
-   **秘密:** Webhook の URL は秘密ではなくなる（存在しなくなる）。API キーの扱いは変えない。秘密を出力に流さない規則（[ADR-0031](../../dev/adr/0031-secret-closure.md)）は、残る秘密に引き続き適用する。
-   **ネットワーク:** サーバが外へ送るのは LLM の API だけになる。
-   **残るリスク:** (1) Slack では確かめていない。(2) 保存したファイルの中身を貼り付けた場合は無効化されない（§2.3）。(3) `<https://…|表示>` の形は、Mattermost でリンクになる（行き先は `about:blank#blocked` で、メンションにはならない）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   サーバの設定ファイルに `SLACK_WEBHOOK_URL` の行が残っていると、サーバは起動しない（AC-05）。利用者は、README の案内に従ってその行を消す。

### 4.4. 互換性 (Compatibility)

-   前の版との互換は求めない（§2.3）。拡張・ホスト・サーバは同じ版で使う。

### 4.5. 保守性 (Maintainability)

-   無効化の規則（対象の `@` の定義、コードの範囲）は 1 か所に置き、`docs/dev/security.md` §6 を正本とする。issue は、規則が Go の `internal/publisher` にあり、拡張だけの構成への移行の後は WASM の中でそのまま使えることを挙げる。規則の置き場所は設計で決める（[design_handoff.md](design_handoff.md) H-01）。

### 4.6. アクセシビリティ (Accessibility)

ビューアの UI を変えるので、`CLAUDE.md`「Accessibility」の各点を確かめる。

-   **状態のメッセージ:** AC-20。
-   **言語:** 通知は表示の言語のカタログから出す（AC-20）。
-   **キーボード・見えるフォーカス・名前と役割と状態・対象の大きさ:** 投稿のボタンを除くだけで、操作を加えない。残る操作の振る舞いは変えない。投稿のボタンを除いた後のフォーカスの順は、見た目の順に従う（2.4.3）。
-   **コントラスト・色だけに頼らない・動き:** N/A。新しい色・図形・動きを加えない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   テストの削除は、削除だけのコミットにして coverage-compare で確かめる（`CLAUDE.md`「Testing Strategy」、[ADR-0037](../../dev/adr/0037-test-deletion-coverage.md)）。
-   本番の Webhook にも、テスト用の Webhook にも送らない。この task の確かめで Webhook を使う手順はない。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。投稿の機能に固有の用語（「投稿の状態」など）は、F-004 で用語集から除くか、意味を改める。
-   **メンションの無効化:** §3.1 の「無効化」。
-   **コピーする文章:** §3.1 を参照。
