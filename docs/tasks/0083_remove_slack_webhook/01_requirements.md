# 要件定義書：Slack の Webhook への投稿の機能を廃止し、メンションの無効化をクリップボードへのコピーに移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | 決定の変更で draft に戻した：URL のパスの `/@` の定義を、Mattermost が自動リンクにする形だけに狭めた |

## 1. 概要 (Overview)

**目的:** 本書は、Slack 互換の Incoming Webhook への投稿の機能（以下、投稿の機能）を廃止し、記事によるメンションの無効化を、ビューアの表示と Markdown のコピーに移すための要件を定義する。

**背景:**

拡張のビューアには、記事を Webhook へ投稿する操作、`.md` で保存する操作、Markdown 形式でクリップボードへコピーする操作がある（`extension/src/viewerApp.ts:557`〜`559`）。投稿の代わりに、利用者が Slack や Mattermost のチャンネルを開いて、コピーした記事を貼り付ければよい。親 issue [#769](https://github.com/isseis/yt2column/issues/769) は、拡張だけの構成へ移る方針を決めた。そこへ移る前に、秘密（Webhook の URL）と、任意の送り先への `host_permissions` を要する機能をなくしておく。

ただし、いまのコピーは、サーバが返す表示用の Markdown（`publisher.DisplayMarkdown`、`internal/publisher/display_render.go:15`）をそのまま書く（`extension/src/viewerApp.ts:548`）。メンションの無効化は、投稿の経路だけにある（`docs/dev/security.md` §6「信頼できないテキストの取り扱い」の 3 つの層、`internal/publisher/slack_render.go:116`・`internal/publisher/slack_message.go:220`）。投稿の機能だけを消すと、メンションの無効化がなくなる。YouTube のハンドルは `@` で始まるので、概要欄やチャンネル名から `@名前` が記事に入りうる。

対応 issue: [#770](https://github.com/isseis/yt2column/issues/770)（親: [#769](https://github.com/isseis/yt2column/issues/769)、関連: [#771](https://github.com/isseis/yt2column/issues/771)・[#772](https://github.com/isseis/yt2column/issues/772)・[#773](https://github.com/isseis/yt2column/issues/773)・[#720](https://github.com/isseis/yt2column/issues/720)）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。廃止する機能、廃止の後にサーバ・ホスト・拡張が返すもの、コピーした文章のどの `@` をどう変えるか、利用者に何を示すかである。無効化をどの部品で行うか（サーバか拡張か）、無効化したかどうかをどう拡張に渡すか、規則のコードをどこに置くか、URL とコードの範囲の判定の細部、テストの手段は、設計（`02_architecture.md`）で決める。設計へ申し送る事項は [design_handoff.md](design_handoff.md) にある。

**issue #770 で決めたこと:**

-   **廃止するもの:** サーバの投稿の操作とその API、`SlackWebhookPublisher`・`PrepareSlackMessages`・`PostMessages`、`internal/slackwebhook`、`SLACK_WEBHOOK_URL` の設定、`make test-integration-slack`、拡張のビューアの投稿の操作。メンションへの対策のうち、Webhook に固有の層（`silent` の付与、メッセージの分割）も廃止する。利用者が手で貼り付ける方法で置き換えられるためである。
-   **コピーへ移す規則:** コピーでは、対象の `@`（`docs/dev/security.md` §6 の定義）を、`@` の直後に U+200B を入れていつも無効化する。対象は見出し行と、本文のコードスパンとフェンス付きコードブロックの外である。語の途中の `@` は変えない。コードの範囲を判定できない本文では、すべての対象の `@` を無効化する。issue は直前が `/` の `@` も変えないとしたが、実機での確かめの結果で、URL のパスの `/@` だけを変えないことに改めた。コードの範囲を判定できない本文でも、URL のパスの `/@` は変えない（下の「要件定義の際に決めたこと」）。URL のパスの `/@` の定義は、設計の際に Mattermost のソースを読んで狭めた（下の「設計の際に改めたこと」）。
-   **変えないもの:** 表示（ビューア）と `--out` のファイルの内容は変えない。issue に理由は記録されていない。表示については下の「要件定義の際に決めたこと」で改め、`--out` を変えない理由も同じ節に記録した。
-   **実機での確かめ:** 貼り付けた `@channel`・`@here`・`@名前` がメンションになるか、U+200B が送信の後も残って無効化が効くか、`<!channel>`・`<@…>`・文字参照・`<a|b>` の形が貼り付けで特別な意味を持つかを、実機で確かめる。その結果で、いまの拒否の規則（§6 の層 (3)。issue は §3 と書くが、規則は §6 にある）をコピーに残すかどうかを決める。
-   **文書:** ADR-0011 を廃止し、ADR-0028 の投稿に関する決定と、ADR-0027 の投稿の状態を改める。`security.md` の該当の項を、コピーの経路の規則として書き直す。`production_profile.md`・`project_overview.md`・README（と `README.en.md`）・`development.md`・`CLAUDE.md` を改める。
-   **テストの削除:** テストの削除を含むので、coverage-compare の手続き（`CLAUDE.md`「Testing Strategy」）に従う。

**要件定義の際に決めたこと（2026-11-11）:**

-   **進め方は full とする。** 簡易プロセスの除外に当たる変更と判断した。`docs/dev/security.md` の規則（§2 の秘密、§3 の Webhook への通信、§6 のメンションの扱い、§8 のサーバの投稿）を書き換えるためである（`requirements_process.md`「Simple Process」の除外の 1 つ目）。レビュー担当者もこの判断のとおり full を選んだ。ADR の判断: ADR を作らない（`Process` 行と本節に記録する）。
-   **実機での確かめを、要件を書く前に行った。** 2026-11-11 に、利用者が Mattermost のテスト用のチャンネルに次の並びを貼り付けて送信した。確かめた版は、Mattermost サーバ 11.11.3、デスクトップアプリ 6.3.2 である。Slack では確かめていない（いまの方針で Slack は best effort であり、確かめの対象ではない。`.claude/commands/_context.md`「Target client environments」）。確かめは 2 回に分けた。1 回目は、送った人の画面で、メンションとして強調されるか（「メンションになった」）、ただの文字列か、リンクかを見た。2 回目は、同じチャンネルの 2 つ目のアカウントで、通知・メンションの数のバッジ・「最近のメンション」への掲載のどれかが起きるか（「通知された」）を見た。

    | 貼り付けた並び | 結果 |
    |---|---|
    | `@channel`・`@here`・実在の利用者の `@名前` | メンションになった（1 回目） |
    | 上の 3 つの `@` の直後に U+200B を入れたもの | メンションにならず、ただの文字列になった（1 回目） |
    | `＠channel`（全角）、`＠` の直後に U+200B を入れたもの | ただの文字列になった（1 回目） |
    | コードスパンの中の `` `@channel` ``、フェンス付きコードブロック（` ``` `）の中の `@channel` | メンションにならなかった（1 回目） |
    | `https://example.com/@channel` | 全体で 1 つのリンクになり、メンションにならなかった（1 回目） |
    | `<!channel>`・`<@名前>`・`&#64;channel`・`&lt;!channel&gt;` | ただの文字列になった（1 回目） |
    | `<https://example.com\|click>` | `about:blank#blocked` へのリンクになり、メンションにならなかった（1 回目） |
    | `https://example.com/@名前`・`https://example.com/@here` | 通知されなかった（2 回目） |
    | `AC/@名前`（URL でない、直前が `/` の `@`） | **通知された**（2 回目） |
    | コードスパンの中の `` `@名前` ``、`~~~` のフェンスの中の `@名前` | 通知されなかった（2 回目） |

    ADR の判断: [ADR-0043](../../dev/adr/0043-copy-mention-neutralization.md)（ビューアの表示とクリップボードへのコピーで記事のメンションを無効化し、コピーは拒否しない）を作り、確かめた結果を背景に置く。
-   **拒否の層 (3) をコピーに持ち込まない。** コピーは、記事の内容によって拒否しない。いまの投稿が拒否する並びのうち、コードの中の対象の `@`、URL のパスの `/@`、`<!`、文字参照、`<a|b>` の形は、コピーした文章にそのまま残す。`<@` の形も拒否しないが、その `@` は対象の `@` なので、ほかの対象の `@` と同じく無効化する（F-002）。上の確かめで、どれも貼り付けた後にメンションにも通知にもならなかったためである。拒否すると、利用者はコピーできず、保存したファイルなどから手でコピーするしかなくなる。コードの範囲を判定できない本文の扱いは issue のとおりで、拒否せずにすべての対象の `@` を無効化する。ADR の判断: ADR-0043 に含める。
-   **直前が `/` の `@` は、URL のパスの `/@` のときだけ変えない。** ほかの直前が `/` の対象の `@` は無効化する（F-002）。URL のパスの `/@` は、§3.1 の定義による（ホストの後ろの `/` の直後の `@`。例: `https://www.youtube.com/@ハンドル`）。上の確かめで、`AC/@名前` は通知され、URL のパスの `/@`（`https://example.com/@名前`）は通知されなかったためである。issue の「直前が `/` の `@` は変えない」を、この結果で改めた。URL のパスの `/@` を無効化すると、`https://www.youtube.com/@ハンドル` のような URL が壊れる。URL の中でも、確かめていない位置の `@`（ホストのない `https://@channel`、クエリの `?@here` など）は無効化する。`https://` を付けるだけで無効化を逃れられないよう、プロンプトインジェクションに備えるためである。この規則はタイトルの行・チャンネル名の行・本文で同じとする。URL のパスの `/@` の定義は、設計の際に、Mattermost が自動リンクにする形だけに狭めた（下の「設計の際に改めたこと」）。ADR の判断: ADR-0043 に含める。
-   **コードの範囲の中の `@` は変えず、判定の食い違いは残るリスクとする。** こちらがコードと判定する範囲（`writer.ProseRanges`）を Mattermost がコードと扱わない書き方では、`@` がメンションのまま残る（§4.2）。食い違いの洗い出しは設計で行う（H-06）。コードの中も無効化する案は採らない。コードの中に見えない文字が入り、貼り付けたコードを写すと、その文字が混じるためである。ADR の判断: ADR-0043 に含める。
-   **見出しの下のチャンネル名の行も、無効化の対象にする。** コピーする文章（表示用の Markdown）では、チャンネル名はタイトルの見出しの下の独立した行にある（`internal/publisher/display_render.go:31`）。いまの投稿では、チャンネル名は見出し行の中にあり、無効化の対象である（`internal/publisher/slack_render.go:68`〜`70`）。チャンネル名は外部由来の値で、YouTube のハンドルのように `@` を含みうるので、行が分かれても対象から外さない。ADR の判断: ADR-0043 に含める。
-   **ビューアの表示も、コピーと同じ規則で無効化する。** 表示する記事の文字には、コピーする文章と同じ位置に U+200B が入る（F-002）。利用者は、コピーボタンを使わずに、表示した記事を選択してコピーすることがある。ブラウザは描画した文字をコピーするので、表示が `@channel` のままなら、貼り付けた先でメンションになる。U+200B は目に見えないので、見た目は変わらない。issue の「表示は変えない」を、この理由で改めた。代わりに、ページ内検索で `@名前` を探しても、無効化した箇所は見つからない。ADR の判断: ADR-0043 に含める。
-   **`--out` のファイルと「.md で保存する」のファイルは無効化しない。** `--out` は開発用の CLI の出力で、エンドユーザー向けの機能ではなく、なるべく加工しないデータが要るためである。保存するファイルは `--out` のファイルと同じ中身である（§2.3）。ADR の判断: ADR-0043 に含める。
-   **無効化したことを、「Markdown をコピーする」ボタンの結果の通知で知らせる。数は示さない。** 1 個以上の `@` を無効化したときだけ、ボタンを押した後の通知でそのことを示す（F-003）。表示した記事を利用者が選択してブラウザでコピーしたときは、通知を出さない。その操作にはビューアのボタンが関わらず、通知を出す場所が操作と結びつかないためである（§4.2 の残るリスク 6）。選択によるコピーをブラウザの `copy` イベントで捉えて通知する案は採らない。利用者がボタンを操作していない場所に通知が出て分かりにくく、貼り付けた結果は表示の無効化ですでに安全だからである。無効化しなかったときは、いまの通知のままにする。貼り付けた文章が見た目と違う文字を含むことを、利用者が知れるようにするためである。数は示さない。利用者は数を見ても次の行動が変わらず、有無で目的を果たせるためである。ADR の判断: ADR-0043 に含める。
-   **本文が正しい UTF-8 でない記事は、完了の時点の検査で `article_invalid` として失敗させる。** いまは、送る前の検査だけが本文の UTF-8 を検査し、そのような記事を `done` のまま投稿できない記事にしている（`internal/publisher/slack_message.go:30`〜`35`、`internal/server/generation.go:89`〜`98`）。送る前の検査を廃止するので、検査を完了の時点に移す（F-001）。CLI の `--out` も、同じ検査で記事を書かずに失敗する。黙って U+FFFD に置き換えると、`CLAUDE.md`「Reject, don't normalize」に反するためである。ADR の判断: ADR を作らない。既存の完了の時点の検査に 1 つの値の検査を加えるだけで、後のタスクを縛る新しい決定ではないためである。
-   **サーバの設定ファイルに残った `SLACK_WEBHOOK_URL` の行は、知らないキーとして拒否する。** サーバは起動しない（F-001）。廃止した `YT2COLUMN_MODEL` と同じ扱いである（`internal/config/serverfile.go:71`、`internal/config/serverfile_test.go:140`）。黙って無視すると、設定の誤りを隠し、`CLAUDE.md`「Reject, don't normalize」に反するためである。利用者には README で行の削除を案内する。ADR の判断: [ADR-0018](../../dev/adr/0018-server-config-file.md) を改める。
-   **確かめの対象の環境を、Mattermost への貼り付けに改める。** `.claude/commands/_context.md` の「Target client environments」は、いま Incoming Webhook と `silent` を前提にしている。これを「Mattermost（ビューアでコピーした Markdown の貼り付け）。Slack は best effort で、確かめの対象ではない」に改める（F-004）。ADR の判断: ADR を作らない。確かめの対象が Mattermost で Slack が best effort であることは変わらず、経路の名前を改めるだけだからである。
-   **投稿の廃止に合わせて ADR を改める。** [ADR-0011](../../dev/adr/0011-webhook-publishing.md) を廃止し、代わりの ADR を ADR-0043 とする。[ADR-0027](../../dev/adr/0027-article-job-api.md) は投稿の操作と投稿の状態を、[ADR-0028](../../dev/adr/0028-no-automatic-resend.md) は投稿の冪等性と Webhook への送り直しの決定を除く。Webhook や投稿に触れる [ADR-0005](../../dev/adr/0005-server-architecture.md)・[ADR-0006](../../dev/adr/0006-threat-model.md)・[ADR-0008](../../dev/adr/0008-launch-path.md)・[ADR-0010](../../dev/adr/0010-extension-stores-nothing.md)・[ADR-0022](../../dev/adr/0022-cost-estimate.md)・[ADR-0031](../../dev/adr/0031-secret-closure.md)・[ADR-0034](../../dev/adr/0034-template-allowlist.md) も、決定から Webhook と投稿を除く。既存の ADR の決定を変えるので、どれも主要な決定である（`CLAUDE.md`「Design decisions (ADR)」）。実装までの間、各 ADR は #770 へリンクした食い違いの注記を持ち、#770 の本文はそれらの ADR を名指しする。[ADR-0009](../../dev/adr/0009-viewer-opening.md) の背景は投稿に触れるが、決定は変わらないので、実装の PR で編集上の訂正として直す。

**設計の際に改めたこと（2026-11-11）:**

-   **URL のパスの `/@` を、Mattermost が自動リンクにする形だけに狭めた。** 設計（[02_architecture.md](02_architecture.md)）の際に、Mattermost の `master`（コミット `d504067`）のソースを読んだ。Mattermost は、段落のテキストのノードだけからメンションを探し、自動リンクの中は探さない。一方、前の定義が URL のパスの `/@` とした形の中に、Mattermost が自動リンクにしない形があった。スキームの直前に ASCII の英字・数字がある形、ホストに `_` を含むか先頭が記号の形、`/@` より前に `<`・`>` がある形、同じ段落の前のほうに `[` がある形、URL がコードスパンの中から始まる形である。これらの `@` は、前の定義では変えないが、貼り付けると通知になる。そこで §3.1 の定義を、Mattermost の自動リンクの条件より狭い条件に改め、当たらない `@` を無効化する。狭めたので、Mattermost が自動リンクにする URL のうち、ポート付きのホスト（`https://example.com:8080/@名前`）などは、U+200B が入って壊れる。通知を防ぐことを、URL を保つことより優先する。読解の内容は 02_architecture.md にある。ADR の判断: [ADR-0043](../../dev/adr/0043-copy-mention-neutralization.md) を改める。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   Webhook の URL という秘密と、それを使う送信の経路をなくす。
-   コピーした記事を Mattermost に貼り付けても、記事の中の `@channel`・`@here`・`@名前` などがメンションにならないようにする。Slack は best effort とする。
-   コピーは、記事の内容によって失敗させない。
-   記事の見た目と、保存するファイル・`--out` のファイルは、いまのまま保つ。

### 2.2. スコープ (In Scope)

-   投稿の機能の廃止（F-001）。サーバの投稿の操作・投稿のジョブ・投稿の状態・Webhook の設定と送信、ホストの `publish` の操作、拡張のビューアの投稿の操作と表示、`make test-integration-slack` とそのテスト専用の変数。
-   本文の UTF-8 の検査を、完了の時点の検査へ移すこと（F-001）。
-   ビューアの表示とコピーでのメンションの無効化（F-002）。
-   「Markdown をコピーする」ボタンの結果の通知で、無効化したことを示すこと（F-003）。
-   文書の改訂（F-004）。`docs/dev/security.md`・`docs/dev/production_profile.md`・`docs/dev/project_overview.md`・`docs/dev/guides/development.md`・`docs/dev/conventions/package_reference.md`・`README.md`・`README.en.md`・`CLAUDE.md`・`AGENTS.md`・`docs/translation_glossary.md`、エージェントの指示（`.claude/commands/_context.md` の「Conditional-guide trigger」「Target client environments」、`.claude/commands/runplan.md` など投稿の機能に触れるもの）、CI のレビューの指示（`.github/workflows/deepseek-review.yml`）と、ほかに投稿の機能を今の機能として説明する文書。
-   投稿の機能だけが使うテストの削除と、その coverage-compare による確かめ。
-   issue [#720](https://github.com/isseis/yt2column/issues/720)（security.md §3 と投稿のやり直しの食い違い）の解消。投稿の機能をなくすと、食い違う規則とコードがともになくなる。

### 2.3. スコープ外 (Out of Scope)

-   **`.md` で保存するファイルの無効化。** 保存するファイルは `--out` のファイルと同じ中身で（`internal/server/status.go:51`〜`52`、`internal/publisher/display_render.go:24`〜`25`）、`--out` と同じく加工しない（§1「要件定義の際に決めたこと」）。保存したファイルの中身を貼り付けると、メンションになりうる。これは受け入れる。
-   **Slack での確かめ。** Slack は best effort で、確かめの対象ではない（§1「要件定義の際に決めたこと」）。
-   **localhost サーバと Native Messaging のホストの撤去、WASM への移行。** [#771](https://github.com/isseis/yt2column/issues/771)・[#772](https://github.com/isseis/yt2column/issues/772)・[#773](https://github.com/isseis/yt2column/issues/773) で扱う。
-   **ほかの投稿先や、新しい送信の経路。**
-   **前の版の拡張・ホスト・サーバとの互換。** 互換の仕組みは作らない（[ADR-0027](../../dev/adr/0027-article-job-api.md)）。前の版の拡張が投稿の操作を送れば、F-001 の拒否になる。
-   **CLI の変更。** CLI はすでに投稿せず、`SLACK_WEBHOOK_URL` を読まない（[0062 要件定義書](../0062_remove_cli_slack/01_requirements.md)）。CLI の振る舞いは、本文の UTF-8 の検査（F-001）のほかは変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

本節で使う語は次のとおりとする。

-   **対象の `@`:** `docs/dev/security.md` §6 の定義による。直後に名前の文字（文字・数字・`_`・`.`・`-`）が続く `@` のうち、先頭にあるもの、または直前が文字でも数字でもないものである。`＠` などの見た目の似た文字を含む。いまの判定は `internal/publisher/slack_message.go:372`〜`420`（`replaceLookAlike`・`mentionAts`・`mentionAtsOf`）にある。語の途中の `@`（`Tech@Work`、`user@example.com`）は対象の `@` ではない。
-   **無効化:** 対象の `@` の直後に U+200B（ゼロ幅スペース）を 1 つ入れること。`@` そのもの（見た目の似た文字を含む）は変えない。
-   **URL:** `http://` か `https://` で始まり、空白か改行の直前までの並び。
-   **URL のパスの `/@`:** 対象の `@` のうち、次のすべてを満たすもの。Mattermost が自動リンクにして、メンションとして扱わない形である（§1「設計の際に改めたこと」）。
    1.  `@` の直前が `/` である。
    2.  `@` より前の同じ行に、URL の始まり（小文字の `http://` か `https://`）があり、その直前は行頭か、ASCII の英字・数字でない文字である。
    3.  ホスト（`://` の直後の並び）は、文字・数字・`.`・`-` だけの 1 文字以上で、最初の文字は文字か数字であり、直後に `/` が続く。
    4.  URL の始まりから `@` までに、空白・`<`・`>`・`?`・`#`・`` ` `` がない。
    5.  URL の始まりより前に、同じ段落（空行で区切られた行の並び）の中に `[` がない。

    例えば `https://www.youtube.com/@handle` の `@` は当たる。`https://@channel`・`https://example.com/?@here`・`https://example.com)@here`・`https://example.com?/@here`・`xhttps://example.com/@here`・`https://a_b.example.com/@here`・`https://example.com:8080/@here`・`https://example.com<b/@here`・`[https://example.com/@here]`・`` `https://example.com`/@here `` の `@` は当たらない。
-   **コードの範囲:** 本文のコードスパンとフェンス付きコードブロック。いまの判定は `writer.ProseRanges`（`internal/writer/markdown.go:49`）で、判定できない本文（対にならないバッククォートなど）では範囲がないことを返す。
-   **表示用の Markdown:** いまの状態の取得の応答の `job.article.markdown`（`publisher.DisplayMarkdown` の出力）。タイトルの見出しの行、チャンネル名があればその行、本文の順に並ぶ（`internal/publisher/display_render.go:28`）。本書では、無効化する前の文字列を指す。
-   **コピーする文章:** ビューアの「Markdown をコピーする」がクリップボードに書く文字列。

#### F-001: 投稿の機能の廃止

サーバ・ホスト・拡張から、Webhook への投稿の機能をなくす。記事の生成・状態の取得・表示・保存・コピーは残る。

**Acceptance Criteria**:

-   **AC-01**: ビューアは、投稿のボタン（いまの「Slack に投稿する」）、投稿の状態とその理由の表示、Webhook が設定されていないことの表示を持たない。記事の表示・「.md で保存する」・「Markdown をコピーする」は、いまの条件で使える。
-   **AC-02**: サーバは、`POST /jobs/{key}/publish` のリクエストを、ほかの受け付けないリクエストのターゲットと同じく `not_found` で拒否する。本文を読まず、記事のキャッシュを変えず、ネットワークへ何も送らない（いまのターゲットの拒否の規則は `docs/dev/security.md` §8.2「ヘッダの検査はパスとメソッドだけ」）。
-   **AC-03**: 状態の取得の応答の差分は、最上位の `webhookConfigured` のメンバと、`job` の `publish` のメンバ（いまは `done` の応答だけが持つ）をなくすこと、F-002・F-003 のために設計が加えるメンバ、F-002 の無効化による `job.article.markdown` の値の変化に限る（いまの応答の形は `internal/server/status.go:26`）。
-   **AC-04**: ホストは、`op` が `publish` のメッセージを、ほかの受け付けない `op` と同じく、サーバへ何も送らずに `protocol_error` で返す（いまの規則は [ADR-0027](../../dev/adr/0027-article-job-api.md)）。
-   **AC-05**: サーバの設定ファイルに `SLACK_WEBHOOK_URL` をキーとする行があれば、値が空でも、サーバは起動しない。拒否は知らないキーの拒否（`ErrConfigFileFormat`、その行の番号と固定の理由）で、エラーとログは行の値を含まない。`#` で始まるコメントの行は、いまと同じく、`SLACK_WEBHOOK_URL` を含んでいても受理する（いまの規則は `internal/config/serverfile.go:59`）。
-   **AC-06**: `yt2column-server config init` が作る雛形は、`SLACK_WEBHOOK_URL` を含む行を持たない。
-   **AC-07**: 生成のジョブの成功のログの行（いまは `generation job succeeded`、`internal/server/generation.go:103`）の差分は、`publish` の属性をなくすことに限る。完了の時点の検査を通る記事は、メンションを含む記事や、いまの分割の上限を超える記事も、`done` になる。
-   **AC-08**: サーバは、終了の合図を受けたとき、投稿のジョブの終わりを待たない（いまは最長でおよそ 5 分待つ。`README.md:314`）。生成の取り消しなど、ほかの終了の振る舞いは変えない。起動のログの `listening` の行の差分は `webhookConfigured` をなくすことに限り、Webhook が設定されていないことの警告の行（いまは `cmd/yt2column-server/run.go:226`）は出さない。
-   **AC-09**: `make test-integration-slack` のターゲットは存在しない。`YT2COLUMN_TEST_SLACK_WEBHOOK_URL` と `YT2COLUMN_SLACK_INTEGRATION` を読むコードとターゲットもない。
-   **AC-09a**: 本文が正しい UTF-8 でない記事は、サーバでは完了の時点の検査で `failed` の `article_invalid` になり、CLI では `--out` のファイルを書かずに失敗する。どちらも本文の値をエラーとログに含めない。本文の文字を U+FFFD などに置き換えて受理しない。

#### F-002: 表示とコピーでのメンションの無効化

ビューアの「Markdown をコピーする」は、表示用の Markdown の対象の `@` を無効化した文章を書く。ビューアが表示する記事も、同じ位置で無効化する。記事の内容によってコピーを拒否しない。

**Acceptance Criteria**:

-   **AC-10**: コピーする文章と表示用の Markdown との差分は、U+200B を入れることに限る。U+200B は、AC-11 と AC-13 が定める `@` の直後にだけ入り、1 つの `@` に 1 つだけ入る。
-   **AC-11**: タイトルの見出しの行と、チャンネル名の行では、URL のパスの `/@` を除くすべての対象の `@` を無効化する。本文では、コードの範囲の外の対象の `@` を、URL のパスの `/@` を除いて無効化する。直前が `/` の対象の `@` も、URL のパスの `/@` でなければ無効化する（例: `AC/@here`、`https://@channel`）。URL の中でも、URL のパスの `/@` でない対象の `@` は無効化する（例: `https://example.com/?@here`）。
-   **AC-12**: どの行でも、語の途中の `@` と、URL のパスの `/@`（例: `https://www.youtube.com/@handle`）は変えない。本文のコードの範囲の中の `@` は変えない。
-   **AC-13**: コードの範囲を判定できない本文では、本文の対象の `@` を、URL のパスの `/@` を除いてすべて無効化する。
-   **AC-14**: いまの投稿が拒否する並び（コードの中の対象の `@`、直前が `/` の対象の `@`、`<!`・`<@`、文字参照、`<a|b>` の形）を含む記事も、コピーできる。それらの並びは、AC-10〜AC-13 のとおりに書かれ、そのほかは変わらない。
-   **AC-15**: 無効化する `@` がない記事（対象の `@` がないか、すべてがコードの範囲の中にあるか URL のパスの `/@` である）では、コピーする文章は表示用の Markdown とバイト単位で同じである。
-   **AC-16**: ビューアが表示する記事の文字は、コピーする文章を描画したものである。表示した記事の全体を選択してブラウザでコピーすると、AC-11 と AC-13 が定める `@` の直後に U+200B が入っている。表示の見た目は、この task の前と変わらない（手動で読んで確かめる）。
-   **AC-16a**: 「.md で保存する」が保存するファイルと、CLI の `--out` のファイルは、本文が正しい UTF-8 の記事について、この task の前と比べて変わらない。それぞれの差分は空である。
-   **AC-17**: 実装の後、次の手動の確かめを行い、日付と Mattermost の版を記録する。ビューアから、タイトル・チャンネル名・本文のコードの外に `@channel`・`@here`・実在の利用者の `@名前`・`AC/@名前` を含み、本文に URL のパスの `/@名前` と、`https://@channel`・`https://example.com/?@here` を含む記事をコピーする。それを Mattermost のテスト用のチャンネルに 1 つ目のアカウントで貼り付けて送信する。同じチャンネルの 2 つ目のアカウントでは、通知・メンションの数のバッジ・「最近のメンション」への掲載のどれも起きない。送るときに `@channel` の確かめのダイアログも出ない。ビューアで表示した記事の全体を選択してコピーし、同じように貼り付けて送信した場合も、同じ結果になる。

#### F-003: 「Markdown をコピーする」ボタンの結果の通知

「Markdown をコピーする」ボタンを押した後の通知で、`@` を無効化したことを知らせる。通知を出すのはこのボタンの操作だけで、表示した記事を選択してブラウザでコピーしたときは出さない。

**Acceptance Criteria**:

-   **AC-18**: 「Markdown をコピーする」ボタンを押してコピーに成功し、コピーする文章に U+200B を 1 つ以上入れたとき、通知は日本語の表示では「Markdown をコピーしました。メンションにならないよう、@ の後ろに見えない文字を入れました。」を示す。英語の表示では、コピーしたことと、`@` の後ろに見えない文字を入れたことを英文で示す（文面は設計で決める）。どちらの表示も、入れた数を示さない。
-   **AC-19**: 「Markdown をコピーする」ボタンを押してコピーに成功し、U+200B を 1 つも入れなかったとき、通知はいまの「Markdown をコピーしました。」のままである。ボタンを押してコピーに失敗したときは、いまの「Markdown をコピーできませんでした。」を示し、無効化したことを示さない。
-   **AC-19a**: 表示した記事を利用者が選択してブラウザでコピーしたとき、ビューアは通知を出さず、通知の領域の表示を変えない。
-   **AC-20**: AC-18 の通知は、いまのコピーの通知と同じく、フォーカスを動かさずに支援技術へ伝わる（WCAG 2.2 の 4.1.3。いまの通知の領域は `extension/static/viewer.html:29` の `role="status"`）。通知は英語と日本語のメッセージのカタログの両方にある。

#### F-004: 文書の改訂

**Acceptance Criteria**:

-   **AC-21**: `docs/dev/security.md` は、投稿の機能の規則（§2 の Webhook の URL、§3 の Webhook への送信、§6 のメンションの 3 つの層、§8 の投稿の操作と投稿のジョブ）を持たず、§6 にコピーの経路の無効化の規則（F-002 の内容）と、§4.2 の残るリスクを持つ。手動で読んで確かめる。
-   **AC-22**: §2.2 に挙げたほかの文書とエージェントの指示は、投稿の機能を今の機能として説明しない。README（と `README.en.md`）のビューアの説明は、表示とコピーで `@` を無効化することと、無効化したことの通知（F-002・F-003）を説明する。手動で読んで確かめる。`README.en.md` は `README.md` と同期している（`TestReadmeSyncMarker`）。文書の値や構造を確かめる既存のテスト（`CLAUDE.md`「Development Notes」の text-sync のガード）は、改めた文書と合う。
-   **AC-23**: README の設定ファイルの説明は、`SLACK_WEBHOOK_URL` の行が残っているとサーバが起動しないことと、その行を消す手順を示す。手動で読んで確かめる。
-   **AC-24**: `.claude/commands/_context.md` の「Target client environments」は、Mattermost へのコピーした Markdown の貼り付けを確かめの対象とし、Slack を best effort とする。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。コピーの無効化は記事 1 本の文字列の変換で、記事の生成（LLM の呼び出し）に比べて無視できる。

### 4.2. セキュリティ (Security)

`docs/dev/security.md` を適用する（`.claude/commands/_context.md`「Conditional-guide trigger」の、Webhook の URL を扱うことと、信頼できないテキストを出力に入れることに当たる）。

-   **信頼できない入力:** 表示とコピーの入力は表示用の Markdown で、LLM の出力と、外部由来のタイトル・チャンネル名を含む。字幕と動画メタデータへのプロンプトインジェクションで、記事に `@channel` などを入れられることを前提にする。
    -   受け付ける形: 状態の取得の `done` の応答。拡張は、いまと同じく、応答のメンバの集まりが、定められた応答の形とどの階層でもちょうど一致するときだけ受理し、拡張できるメンバを宣言しない（`extension/src/core/response.ts:1`〜`7`）。応答の形は、`publish`・`webhookConfigured` を除き、F-002・F-003 のために設計が加えるメンバを含めた形に改める（AC-03）。コピーが使うのは、表示用の Markdown（`job.article.markdown`）と、設計が加えるメンバだけである。
    -   拒否: コピーは記事の内容によって拒否しない（§1「要件定義の際に決めたこと」）。記事の値の検査は、完了の時点の検査（いまの `article_invalid`・`article_too_large`）が行う。本文が正しい UTF-8 でない記事は、そこで `article_invalid` として拒否する（AC-09a）。
    -   黙って直さない: 無効化は U+200B を入れるだけで、記事のほかの文字を変えない（AC-10）。
-   **秘密:** Webhook の URL は存在しなくなる。API キーの扱いは変えない。秘密を出力に流さない規則（[ADR-0031](../../dev/adr/0031-secret-closure.md)）は、残る秘密に引き続き適用する。
-   **ネットワーク:** サーバが外へ送るのは LLM の API だけになる。
-   **残るリスク:**
    1.  Slack では確かめていない。
    2.  保存したファイルの中身を貼り付けた場合は無効化されない（§2.3）。
    3.  `<https://…|表示>` の形は、Mattermost でリンクになる（行き先は `about:blank#blocked` で、メンションにはならない）。
    4.  こちらがコードの範囲と判定するのに Mattermost がコードと扱わない書き方では、その中の `@` がメンションになる（H-06）。
    5.  こちらが URL のパスの `/@` と判定するのに Mattermost が自動リンクにしない並びでは、その `@` がメンションになる。定義（§3.1）は、2026-11-11 に読んだ Mattermost の `master` の自動リンクの条件より狭くしたが、読解していない版と形では保証しない（H-06）。
    6.  表示した記事を選択してコピーした場合は、無効化されるが、無効化したことの通知は出ない（F-003 の通知は「Markdown をコピーする」ボタンの操作だけ。AC-19a）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   サーバの設定ファイルに `SLACK_WEBHOOK_URL` の行が残っていると、サーバは起動しない（AC-05）。利用者は、README の案内（AC-23）に従ってその行を消す。

### 4.4. 互換性 (Compatibility)

-   前の版との互換は求めない（§2.3）。拡張・ホスト・サーバは同じ版で使う。

### 4.5. 保守性 (Maintainability)

-   無効化の規則（対象の `@` の定義、URL とコードの範囲）は 1 か所に置き、`docs/dev/security.md` §6 を正本とする。issue は、規則が Go の `internal/publisher` にあり、拡張だけの構成への移行の後は WASM の中でそのまま使えることを挙げる。規則の置き場所は設計で決める（[design_handoff.md](design_handoff.md) H-01）。

### 4.6. アクセシビリティ (Accessibility)

ビューアの UI を変えるので、`CLAUDE.md`「Accessibility」の各点を確かめる。

-   **状態のメッセージ:** AC-20。
-   **言語:** 通知は表示の言語のカタログから出す（AC-20）。
-   **キーボード・見えるフォーカス・名前と役割と状態・対象の大きさ:** 投稿のボタンを除くだけで、操作を加えない。残る操作の振る舞いは変えない。投稿のボタンを除いた後のフォーカスの順は、見た目の順に従う（2.4.3）。
-   **表示の文字:** 表示する記事に入る U+200B は幅のない文字で、見た目を変えない（AC-16）。
-   **コントラスト・色だけに頼らない・動き:** N/A。新しい色・図形・動きを加えない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   テストの削除は、削除だけのコミットにして coverage-compare で確かめる（`CLAUDE.md`「Testing Strategy」、[ADR-0037](../../dev/adr/0037-test-deletion-coverage.md)）。
-   本番の Webhook にも、テスト用の Webhook にも送らない。この task の確かめで Webhook を使う手順はない。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。F-004 で、用語集を次のように改める。
    -   除く: 「投稿の状態」など、投稿の機能に固有の用語。
    -   意味を改める: 「主経路」「ビューア」（投稿を除き、コピーを加える）、「結果不明」（投稿の操作を除く）、「完了の時点の検査」（送る前の検査を除き、本文の UTF-8 の検査を加える）。
    -   加える: 「メンションの無効化」（§3.1 の「無効化」）、「対象の `@`」、「コピーする文章」。

## 変更履歴

- 2026-11-11 承認（isseis、#813）
- 2026-11-11 draft への戻し：URL のパスの `/@` の定義を、Mattermost が自動リンクにする形だけに狭めた（#未記入）
