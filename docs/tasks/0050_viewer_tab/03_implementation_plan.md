# 実装計画書：拡張を新しい API に移し、ビューアのタブで記事を表示・投稿・保存する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-31 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、「設計書 3.2」などは設計書の節を指す。既存のコードと文書についての記述は、コミット `bf314092` で確かめた。`file:line` はこのコミットの行番号を指す。設計書は行番号をコミット `577e7b51` で確かめている。`git diff --stat 577e7b51 bf314092 -- extension cmd/yt2column/extension_docs_test.go internal/nmhost internal/server docs/dev/guides/development.md` の出力は空なので、設計書の拡張・Go のガード・ホスト・サーバ・`development.md` の行番号は、そのまま使える（`README.md` と `docs/dev/security.md` は、タスク `0051` の変更で行がずれている）。拡張のファイルのパスは、設計書と同じく `extension/` を省いて書く（例: `src/launch.ts`）。テストの場所は `path::describe の名前` で示す。まだないテストの describe の名前は、実装のときに決める仮の名前である。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、次を行う。

-   ホストとの 1 回のやり取り、厳格な JSON の解析、3 つの操作の応答の検査を作る（設計書 3.1〜3.3）。
-   ビューアの URL・拡張の中のメッセージ・ポーリング・投稿のボタン・ファイル名の純粋な判断と、新しい文言を作る（設計書 3.4〜3.10・3.13）。
-   Service Worker（以下 SW）が生成の開始を送り、同じキーのビューアの再利用を調整する（設計書 3.4・6.1・6.2）。
-   ビューアのページ（表示・ポーリング・投稿・保存・コピー）と、ポップアップの新しい状態を作る（設計書 3.6〜3.12・3.14・6.3〜6.6）。
-   ポップアップの送信を新しい経路に切り替え、`/articles` のためのコードを取り除き、文書を改める（設計書 3.17・8 章の 7）。

サーバとホストの Go のコードは変えない（要件書 2.3）。Go の変更は、ガードのテスト `cmd/yt2column/extension_docs_test.go` に確かめを 1 つ加えることだけである（設計書 3.3）。

### 1.2. 実装の方針

-   **設計は設計書に従い、本書は繰り返さない。** 型と関数の形は設計書 3.1〜3.12、文言の節は 3.13、アクセシビリティの規則は 3.14、ログの値は 4.4、テストの構成は 7 章が定める。本書のステップは、どの節に従って何を作り、どう確かめるかだけを示す。
-   **古い経路は、ポップアップを切り替えるまで残す（1.3）。** フェーズ 2〜5 は、新しい部品を `/articles` の経路と並べて加える。どの PR をマージした後も、main の拡張は今の経路で動く。古い部品は、ポップアップを切り替えるフェーズ 6 と同じ PR のフェーズ 7 でまとめて取り除く。
-   **テストが失敗しうることを、壊して確かめる。** 新しいテストと、確認の内容を変えたテストのそれぞれについて、確かめる経路を壊し、そのテストが失敗することを実装のときに確かめ、結果をコミットのメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。各ステップの「完了の条件」に、壊す対象を挙げる。
-   **テストを消すときは、守っていた性質の行き先を確かめる。** 拡張には `go tool cover` がないので、タスク `0046` と同じく `node --test --experimental-test-coverage --test-reporter=lcov` の出力を、消す前と後で比べる。消した関数を除く `src/` の各関数の実行回数（lcov の `FN`・`FNDA`）が 0 にならないことを確かめ、比べたコマンドと結果をコミットのメッセージに書く（`CLAUDE.md`「Deleting a test is a claim that must be checked」の拡張での読み替え）。
-   **各ステップの後で `make ext-check` が通る。** 原則としてステップごとにコミットし、どのコミットでも拡張の型の検査・lint・書式・テスト・ビルドが通る状態を保つ。PR の前には、グリーンゲート（`make test && make lint`）も通す。
-   **拡張のソースは英語で書く。** コメント・識別子・文字列リテラルは英語である。例外は `CLAUDE.md`「Go Idioms」の 3 つ（`ja.ts` の文言、日本語の文言の性質を確かめるテストの語の表、日本語の文書に一致させるガードのテストのリテラル）だけである。`test/jobCodes.test.ts` の表の見出しと列の名前は、3 つ目の例外に当たる（設計書 7.1）。
-   **ソースの値をログに出さない。** 新しいログは、設計書 4.4 のログの関数を通した値だけを書く。
-   **フェーズは設計書 8 章の順序に従う。** フェーズ 1〜7 は、設計書 8 章の 1〜7 に当たる。

### 1.3. 計画の際に決めたこと（2026-10-31）

-   **新しい部品は古い経路と並べて加え、古い部品はポップアップの切り替えと同じ PR で取り除く（レビュアーが選んだ）:** 今のポップアップは、`sendArticle`（`src/send.ts:28-55`）→ `classifyNativeReply`（`src/core/native.ts:114-171`）→ `classifyResponse`（`src/core/response.ts:158-223`）の経路で `/articles` を使う。設計書 8 章の 2 は「`response.ts` を書き換える」と書くが、その場で書き換えると、ポップアップを切り替えるまでの間、main の拡張が動かなくなる。そこで、今の `/articles` の検査を一時的なファイル `src/core/articlesResponse.ts` に移し、`response.ts` を新しい 3 つの操作の検査として書く（1.4 の台帳）。文言の新しい節も、古い節と別の名前で加える。古いファイルと節は、フェーズ 7 で取り除く。最終の形は設計書のとおりで、途中の形だけが設計書の記述と違う。設計書 8 章の 7 が `/articles` の検査の削除を最後の段階に置いているので、設計の決定は変えない。
-   **フェーズ 1 の試作と道具は、リポジトリに入れない（レビュアーが選んだ）:** 設計書 3.16 の確認に使う試作の拡張、応答を約 19 秒遅らせるテスト用のサーバ、25 秒応答しないテスト用のホストは、作業ツリーの外（スクラッチ）で作る。本書には、手順の要点、ブラウザの版、確かめた結果だけを記録する（4.3）。捨てるコードをレビューする手間に見合わないためである。代わりに、再現できるよう、手順は 4.3 に、確かめた結果ごとに書く。
-   **ポップアップの切り替え（フェーズ 6）と、古い部品の削除・文書（フェーズ 7）は、同じ PR にする（レビュアーが選んだ）:** 切り替えると、README の「結果は Slack に届く」などの案内が画面と食い違う。同じ PR にすれば、main の文書と挙動が食い違う期間がない。その PR は大きくなるので、レビューの観点を PR の作成ポイントに分けて書く。
-   **秘密情報の語のガードを狭める（レビュアーが選び、設計書 3.17 に加えた）:** `test/secretsAbsent.test.ts:10-18` は `webhook` と `SLACK_WEBHOOK_URL` の語を禁じ、本設計の応答のメンバ（`webhookConfigured`）・理由（`webhook_not_configured`、投稿の理由 `webhook`）・文言（`SLACK_WEBHOOK_URL` を設定する手順）と両立しない。設計書 3.17「秘密情報の語のガード」のとおりに狭める（ステップ 2-3）。設計書の `Comments` に、この決定をセッションの中で承認したことを記録した。
-   **設計書 3.17 の「送る場所」の確かめと SW のリスナーの登録の確かめは、ガードのテストにする:** 設計書 3.17 は、送ってよいきっかけの静的な確かめを「実装計画に記録する」と書き、7.1 はリスナーの登録を構文木で確かめると書く。本書は、どちらも一度だけ実行して記録するのではなく、`test/entryPoints.test.ts`（新規）として、TypeScript の構文木を読むテストにする（ステップ 4-5・7-2）。後の変更で崩れたときに気付けるようにするためである。
-   **ログの値を作る関数は `src/core/response.ts` に置く:** 設計書 4.4 は `startLogView`・`statusLogView`・`publishLogView` の置き場所を決めていない。これらは結果の型（`…OutcomeLog`）だけを読むので、その型と同じ `response.ts` に置く。

### 1.4. 既存コード調査結果

`bf314092` で確かめた。設計書 1.4・3.17・3.18 が挙げる部品は、すべて設計書の場所にある。以下は、設計書に書かれていないことと、計画の上で注意が要ることである。

**名前の変更の台帳:** 並べて加える間（1.3）だけ、今の `/articles` の部品を次の名前に移す。本書のほかの場所と、コード・テストの参照は、この表に従う。フェーズ 7 で、右の列の名前はすべてなくなる。

| 今の名前（`bf314092`） | 並べて加える間の名前 | フェーズ 7 の後 |
|---|---|---|
| `src/core/response.ts` の `/articles` の部分（`ServerRejectReason`・`serverRejectStatus`・`isServerRejectReason`・`maxResponseBytes`・`ReceivedResponse`・`UnexpectedCheck`・`SendOutcome`・`SendResult`・`toSendOutcome`・`ResponseResult`・`classifyResponse`） | `src/core/articlesResponse.ts` の `ArticlesRejectReason`・`articlesRejectStatus`・`isArticlesRejectReason`・`maxArticlesResponseBytes`・`ArticlesReceivedResponse`・`ArticlesUnexpectedCheck`・`SendOutcome`・`SendResult`・`toSendOutcome`・`ArticlesResponseResult`・`classifyArticlesResponse` | なし（ファイルごと削除） |
| `NotSentCause`（`src/core/response.ts:68`） | `src/core/exchange.ts` の `NotSentCause`（同じ値。設計書 3.1） | 同じ |
| `RequestId`・`parseRequestId`・`hasExactKeys`（`src/core/response.ts:42-53`・`:131-141`） | 新しい `src/core/response.ts` に残す | 同じ |
| `classifyNativeReply`（`src/core/native.ts:114`） | `classifyHostReply`（`HostOutcome` を返す。設計書 3.1）と、それを `SendResult` に変える `classifyNativeReply`（古い経路のための包み） | `classifyHostReply` だけ |
| `test/response.test.ts` の `classifyResponse`・`toSendOutcome`・`serverRejectStatus` の describe | `test/articlesResponse.test.ts` | なし（ファイルごと削除） |

**古い部品と、それを使う場所（フェーズ 7 でなくす）:**

| 部品 | 定義 | 使う場所 |
|---|---|---|
| `sendArticle` | `src/send.ts:28-55` | `src/launch.ts:19`・`:227`、`test/send.test.ts:38` の describe |
| `renderSendOutcome` | `src/ui/renderSend.ts:26-34` | `src/launch.ts:24`・`:230`、`test/renderSend.test.ts`（全体） |
| `serverRejectMessage`・`sendOutcomeMessage` | `src/core/sendMessages.ts:18-23`・`:57-80` | `src/ui/renderSend.ts:7`、`test/sendMessages.test.ts:60`・`:84` の describe |
| `logSend` | `src/launch.ts:133-150` | `src/launch.ts:228`、`test/launch.test.ts:877` の describe（`send log`） |
| 文言の節 `serverReject`・`notSent`・`accepted`・`noResponse`・`hostFailed`・`unexpected`・`sending`・`primaryButton` | `src/core/i18n/catalog.ts:27-39`・`:61`、`en.ts:40-122`・`:148`、`ja.ts` の対応する行 | `src/core/sendMessages.ts`、`test/catalog.test.ts`、`test/sendMessages.test.ts`、`test/metadataDocs.test.ts:51`・`:66`、`test/typecheck.test.ts:160`（`primaryButton` を欠かす例） |
| `/articles` の Slack の案内のコメント | `src/core/i18n/catalog.ts:29-32`、`src/core/sendMessages.ts:25` | - |

`primaryButton` は、主ボタンの 3 つの名前（設計書 3.13 のポップアップの節）に替わる。`test/typecheck.test.ts:160` の「1 つの項目を欠いた `Catalog` は型の検査を通らない」例は、欠かす項目を新しい節の項目に替えて残す。

**既存のガードとの衝突:** `test/secretsAbsent.test.ts:10-18` は 1.3 のとおり（ステップ 2-3 で狭める）。`test/sendMessages.test.ts:47-50`・`:176-211` は、結果不明の文言が Slack を確かめるよう求めることを確かめる。これは設計書 3.17 が書き換えると決めたテストで、フェーズ 7 で新しい文言の確かめに置き換える。lint の `menuPathNames`（`eslint.config.js:98`）は `windows` を禁じ、設計書 3.17 のとおり `chromeDeps.ts` でだけ許す（ステップ 4-4）。`networkNames`（`eslint.config.js:58`）は、新しいコードが使わない API なので変えない。

**外部の前提の確かめ:**

-   **`ES2023.Intl`:** `node_modules/typescript/lib/lib.es2023.intl.d.ts:57` の `NumberFormat.format` は `number | bigint | StringNumericLiteral` を受け取る（TypeScript 6.0.3、`extension/node_modules`）。設計書 3.13 の型の検査の前提は成り立つ。
-   **`chrome.tabs.getCurrent`:** `node_modules/@types/chrome/index.d.ts:12002` に `getCurrent(): Promise<Tab | undefined>` がある。ブラウザでの振る舞いはフェーズ 1 で確かめる（設計書 3.16）。
-   **`src/core/` の型:** `tsconfig.core.json` は `lib` が `ES2022` だけで、`TextDecoder`（`fatal`・`ignoreBOM`）・`TextEncoder`・`URL`・`URLSearchParams.getAll` を `types/core-encoding.d.ts`・`types/core-url.d.ts` で宣言する。`strictJson.ts` の UTF-8 の検査と `watchVideoId` は、この宣言の中で書ける。`encodeURIComponent`・`decodeURIComponent` は ES の標準の関数なので、`viewerUrl.ts` も `src/core/` に置ける。`BroadcastChannel` と `crypto.getRandomValues` は `src/core/` の外（`src/viewerQuery.ts`・`src/browser/chromeDeps.ts`）で使う。
-   **Node.js:** `extension/.node-version` は `24.21.0` で、`node --version` も `v24.21.0` である。`Blob.prototype.arrayBuffer` と `BroadcastChannel` は Node.js 24 の大域にあるが、テストは fake を使い、本物の `BroadcastChannel` は使わない（設計書 7.1）。
-   **Go の厳格な JSON のテスト:** 設計書 7.1 が「`internal/strictjson` のテストが拒否する入力の代表」と呼ぶものは、`internal/strictjson/strictjson_test.go:37-50` の `TestParseObject` の拒否の表（`invalid utf8`・`unpaired high`・`unpaired low`・`trailing object` など）である。拡張のテストは、その入力を同じバイト列で持つ。
-   **ホストの応答の上限:** `internal/nmhost/test_helpers.go:80` の `MaxResponseBytesForTest` は `maxResponseBytes`（`internal/nmhost/relay.go:43`、`512 << 10`）である。Go のガードは、テスト用の補助を加えずにこれを読める。

**型の作り手を限ること:** `ArticleKey`・`UsdAmount` は、`eslint.config.js:149` の `brandNames` に加え、`creatorConfig`（`:197-208`）でそれぞれ `src/core/articleKey.ts` と `src/core/response.ts` にだけ作ることを許す（設計書 3.1）。`PreparedRequest` は、今の `prepareSend` に加えて `prepareStartInput` が作る。どちらも `src/core/prepare.ts` にあり、今の `creatorConfig("src/core/prepare.ts", ["PreparedRequest"])`（`eslint.config.js:262`）のままで足りる。ブランドを持たない判別共用体（`JobStatus`・`ViewerState` など）は、作る経路が検査の関数だけであることを型では守らない。表示の側はそれを受け取るだけなので、ガードは加えない。

**テストの補助:** 拡張のテストの補助は `test/helpers/` に置く（今の `dom.ts`・`native.ts`・`logger.ts` など）。`docs/dev/conventions/test_organization.md` は Go の規則（`testutil/`・`test_helpers.go`・`//go:build test`）で、拡張には当たらない。本タスクは、`test/helpers/chrome.ts`（新規。`runtime`・`tabs`・`windows`・`BroadcastChannel`・クリップボード・ダウンロードの fake）を加え、`test/helpers/native.ts`（新しい形のメッセージの fake）と `test/helpers/dom.ts`（ビューアの骨格）を広げる（設計書 7.1）。Go のガードの変更は、既存の `nmhost.MaxResponseBytesForTest` を使うので、Go のテストの補助は加えない。

**実装の方法が 2 つ以上ある箇所:** ない。設計書が型・関数・メッセージ・CSS の分け方を決めている。並べて加える方法、試作の置き場所、ガードのテストにする確かめ、ログの関数の置き場所は 1.3 で決めた。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 確認の試作

#### ステップ 1-1: 設計書 3.16 の振る舞いを試作で確かめる

**変更するファイル:** `docs/tasks/0050_viewer_tab/03_implementation_plan.md`（4.3 の記録）

-   [ ] 作業ツリーの外に試作の拡張を作る。manifest に `background`（`type: "module"`）と `viewer.html` を持ち、権限は今の 3 つのままにする。設計書 3.16 の表の各行の API を呼ぶ最小の処理だけを持つ。
-   [ ] 応答を約 19 秒遅らせるテスト用のサーバと、25 秒応答しないテスト用のホストを、作業ツリーの外に作る。テスト用のホストを登録したら、確かめの後に `yt2column-nmhost install <browser>` で本物のホストの登録に戻す。
-   [ ] 利用者が、Chrome（110 以降の今の版）と Brave で、設計書 3.16 の表の 10 行を表の「確かめ方」のとおりに確かめる。結果を 4.3 の表に記録する（ブラウザと版、確かめた日、結果）。
-   [ ] 崩れた行があれば、設計書 3.16 の「崩れた場合の方法」に切り替えることを、レビュアーとその場で決め、設計書を改める（`requirements_process.md`「Editing an approved document」）。「なし」の行（`tabs` の API、`<a download>`）が崩れたら、フェーズ 2 に進まず、レビュアーと扱いを決める。

**完了の条件:** 4.3 の表のすべての行に結果がある。崩れた行の扱いが、設計書と本書に記録されている。

### フェーズ 2: やり取りと検査

#### ステップ 2-1: やり取りの型と、ホストへのメッセージ

**変更するファイル:** `src/core/exchange.ts`（新規）、`src/core/articleKey.ts`（新規）、`src/core/hostMessage.ts`（新規）、`src/core/prepare.ts`、`test/articleKey.test.ts`（新規）、`test/hostMessage.test.ts`（新規）、`test/prepare.test.ts`

-   [ ] `exchange.ts` に、設計書 3.1 の `NotSentCause`・`ExchangeFailure`・`ExchangeFailureLog`・`HostOutcome`・`maxResponseBytes`（`524_288`）と、3.3 の新しい `UnexpectedCheck` を置く。どのモジュールも import しない。
-   [ ] `articleKey.ts` に `ArticleKey`・`maxKeyLength`・`parseArticleKey` を置く（設計書 3.1）。
-   [ ] `prepare.ts` に `StartInput` を加え、`ArticleBody.message` の型を `StartInput` にする。`StartInput` の UTF-8 の大きさを測る関数を 1 つ置き、`buildArticleBody` と新しい `prepareStartInput` の両方がそれを使う（設計書 3.1）。
-   [ ] `hostMessage.ts` に `HostMessage` と `startMessage`・`statusMessage`・`publishMessage` を置く（設計書 3.1）。
-   [ ] テスト: `parseArticleKey` の空・上限の境目・上限を超える値・文字列でない値、`prepareStartInput` が `prepareSend` と同じ大きさの判定をすること、3 つのメッセージの形。

**完了の条件:** `make ext-check` が通る。`prepareStartInput` の大きさの判定を `>=` に変えるように壊し、4 MiB ちょうどの入力のテストが失敗することを確かめる。

#### ステップ 2-2: 厳格な JSON の解析

**変更するファイル:** `src/core/strictJson.ts`（新規）、`test/strictJson.test.ts`（新規）

-   [ ] 設計書 3.2 の `JsonValue`・`StrictJsonError`・`parseStrictJson` と、表の 6 つの規則を作る。`StrictJsonError` の種類が `exchange.ts` の `UnexpectedCheck` に含まれることを、型の検査で確かめる形にする（設計書 3.3）。
-   [ ] テスト: 表の規則ごとに、拒否する入力と受理する入力を置く。`internal/strictjson/strictjson_test.go:37-50` の拒否の表の入力を、同じバイト列で拒否することを確かめる（1.4）。深さの上限の境目（16 段と 17 段）も置く。

**完了の条件:** `make ext-check` が通る。重複したメンバの検査を外すように壊し、重複のテストが失敗することを確かめる。

#### ステップ 2-3: サーバの応答の検査と、秘密情報の語のガード

**変更するファイル:** `src/core/response.ts`、`src/core/articlesResponse.ts`（新規、一時的）、`src/core/native.ts`、`src/core/sendMessages.ts`、`src/core/i18n/catalog.ts`、`src/ui/renderSend.ts`、`src/launch.ts`、`eslint.config.js`、`test/response.test.ts`、`test/articlesResponse.test.ts`（新規、一時的）、`test/serverReasons.test.ts`、`test/secretsAbsent.test.ts`、`test/lintRules.test.ts`

-   [ ] 1.4 の台帳のとおりに、今の `/articles` の検査を `articlesResponse.ts` に移し、古い部品の import を移す。古い経路の振る舞いは変えない（1,024 バイトの上限も `articlesResponse.ts` に残す）。
-   [ ] `response.ts` に、設計書 3.3 の型、受理する形の表、拒否の理由の 3 つの `Record`、`classifyStart`・`classifyStatus`・`classifyPublish`、`toStartOutcome`・`toStatusOutcome`・`toPublishOutcome` を作る。集合はどれも `Record<…, true>` の定数とともに宣言する（設計書 3.3）。
-   [ ] 同じ `response.ts` に、設計書 4.4 の `startLogView`・`statusLogView`・`publishLogView` を置く（1.3）。
-   [ ] `eslint.config.js` の `brandNames` に `UsdAmount` を加え、`response.ts` の `creatorConfig` が作ってよい型に加える。
-   [ ] `test/secretsAbsent.test.ts` を、設計書 3.17「秘密情報の語のガード」のとおりに狭める。狭めた語が、`webhook url` などの設定の名前と、文言の 2 つのファイルの外の `SLACK_WEBHOOK_URL` を拒否し続けることを確かめるテストを加える。
-   [ ] `test/serverReasons.test.ts` の突き合わせの相手を `articlesRejectStatus` にする。文書の表の「拡張の表示」の列は、ポップアップを切り替えるまで今のまま正しいので、フェーズ 7 まで変えない。
-   [ ] テスト（`test/response.test.ts` を新しい検査のテストに書き換える）: AC-41 の各項目に 1 つ以上の入力、AC-42 の 256 KiB いっぱいの完了した記事（Go の `json.Marshal` と同じエスケープを含む）、古いホストの応答（`/articles` の `invalid_body`）が生成の開始の `invalid_body` の拒否になること（設計書 7.1）、3 つの操作の拒否の理由の集合と状態コードの組、ログの関数が記事・キーを返す値に含めないこと（設計書 7.3。目印の文字列で確かめる）。

**完了の条件:** `make ext-check` が通る。メンバの集合の検査に任意のメンバを許すように壊し、AC-41 の「宣言していないメンバ」のテストが失敗することを確かめる。`statusLogView` が記事を返す値に入れるように壊し、ログのテストが失敗することを確かめる。

#### ステップ 2-4: ホストの応答の外側と、1 回のやり取り

**変更するファイル:** `src/core/native.ts`、`src/send.ts`、`src/browser/chromeDeps.ts`、`src/core/sendMessages.ts`、`test/native.test.ts`、`test/send.test.ts`、`test/helpers/native.ts`、`test/sendMessages.test.ts`

-   [ ] `native.ts` に `classifyHostReply` を作り、本文のバイト列と状態コードを返す。拡張自身の時間切れを応答なしにする（設計書 3.1）。古い `classifyNativeReply` は、`classifyHostReply` の結果を `classifyArticlesResponse` に渡す包みにする（1.4 の台帳）。
-   [ ] `send.ts` に `exchange`（`HostMessage` を受け取る。設計書 3.1）を作る。`sendTimeoutMs` の宣言の形は変えない。`sendArticle` は、ポップアップを切り替えるまで残し、`exchange` と同じ 1 回の送信の処理を共有する。`SendDeps.sendNative` の引数の型は、並べて加える間は `HostMessage | StartInput`（古い経路の本文は `StartInput` である）にし、フェーズ 7 で `HostMessage` だけにする。
-   [ ] `sendMessages.ts:36` の `extension-timeout` の行を、設計書 3.1 のとおり改める。
-   [ ] `test/helpers/native.ts` の `recordingNative`・`deferredNative` を、新しい形のメッセージを記録できる形に広げる。
-   [ ] テスト: `classifyHostReply` の検査の順と内容が今の `classifyNativeReply` と同じであること（`test/native.test.ts:37` の describe を移す）、時間切れが応答なしになること、`exchange` が 1 回だけ送り送り直さないこと、25 秒の後に届いた応答を無視すること。

**完了の条件:** `make ext-check` が通る。`exchange` が時間切れの後にもう一度送るように壊し、送り直さないことのテストが失敗することを確かめる。

#### ステップ 2-5: 応答の上限の Go のガード

**変更するファイル:** `cmd/yt2column/extension_docs_test.go`

-   [ ] `TestNativeHostContract` に、`extension/src/core/exchange.ts` の `export const maxResponseBytes = 524_288;` を `sendTimeoutMs`（`:120`）と同じ固定の形で読み、`nmhost.MaxResponseBytesForTest` と等しいことを確かめる subtest を加える（設計書 3.3）。読み取りのパスの定数は、今の `extensionSendPath`（`:29`）と同じ形で加える。

**完了の条件:** `make fmt`・`make test`・`make lint` が通る。`exchange.ts` の値を 1 だけ変えて、この subtest が失敗することを確かめる。

### フェーズ 3: 純粋な判断と文言

#### ステップ 3-1: ビューアの URL

**変更するファイル:** `src/core/viewerUrl.ts`（新規）、`test/viewerUrl.test.ts`（新規）

-   [ ] 設計書 3.5 の `ViewerSeed`・`ViewerParams`・`ViewerUrlError`・`viewerQuery`・`parseViewerUrl` を作る。表の 6 つの名前・上限・検査、作り方と読み方の規則、`+` の扱いに従う。
-   [ ] テスト: 作った URL が読み方の規則を必ず通る往復の性質、AC-43 の拒否の各項目に 1 つ以上の URL、各上限の境目、`ViewerSeed` から作った URL が字幕と概要欄の目印の文字列を含まないこと（AC-09。設計書 7.3）。

**完了の条件:** `make ext-check` が通る。重複した名前を後勝ちで受理するように壊し、AC-43 の重複のテストが失敗することを確かめる。

#### ステップ 3-2: 拡張の中のメッセージ

**変更するファイル:** `src/core/extensionMessages.ts`（新規）、`test/extensionMessages.test.ts`（新規）

-   [ ] 設計書 3.4 のメッセージの型、5 つの `parse…` と `chooseViewer` を作る。`parseStartRequest` は設計書 3.4.1 の項目 3 の 4 の検査（メンバの集合、`prepareStartInput` による 4 MiB、ビューアの URL の規則、`sentAt` の上限、対になっていないサロゲート、`source` の整数）を行い、例外が起きても拒否にする。
-   [ ] テスト: 各メッセージの受理と拒否（メンバの過不足、型の違い、上限）、`StartRequest` の 4 MiB の境目、対になっていないサロゲート、未来の `sentAt`、`chooseViewer` の順（送信の時刻、同じならタブの ID）。

**完了の条件:** `make ext-check` が通る。`parseStartRequest` が大きさの検査を飛ばすように壊し、4 MiB を超える求めのテストが失敗することを確かめる。

#### ステップ 3-3: 表示の状態・ポーリングの規則・投稿のボタン

**変更するファイル:** `src/core/jobView.ts`（新規）、`src/core/poll.ts`（新規）、`test/jobView.test.ts`（新規）、`test/poll.test.ts`（新規）

-   [ ] `jobView.ts` に、設計書 3.6 の `ViewerState`・`StartFailure`・`JobDisplay`・`RetryingStatus`・`StoppingStatus`・`PollNotice` と、3.9 の `PublishRow`・`PublishControl`・`publishControl`・`PublishOpNotice` を作る。
-   [ ] `poll.ts` に、設計書 3.7 の定数と `PollStep`・`afterStatus` を作る。3.6 の表の行のとおりに表示と続けるか止めるかを決め、`unknown` で直前の `done` の記事を `kept` に残す。
-   [ ] テスト: `afterStatus` の表の各行（AC-11・AC-13〜AC-16・AC-18）、`publishControl` の 7 つの行と、2 つの行が同時に当てはまる組み合わせ（AC-28）、`publishing` の間と `lost` で押せないこと（AC-32）。

**完了の条件:** `make ext-check` が通る。`publishControl` の行の順を入れ替えるように壊し、重なりのテストが失敗することを確かめる。`afterStatus` が `posting` で止めるように壊し、AC-18 の行が失敗することを確かめる。

#### ステップ 3-4: ファイル名と動画 ID

**変更するファイル:** `src/core/fileName.ts`（新規）、`src/core/acceptedUrl.ts`、`test/fileName.test.ts`（新規）、`test/acceptedUrl.test.ts`

-   [ ] `acceptedUrl.ts` に `watchVideoId` を加える（設計書 3.10）。`isAcceptedWatchUrl` と同じ解析を使う。
-   [ ] `fileName.ts` に `articleFileName` を作り、設計書 3.10 の表の規則を当てる。
-   [ ] テスト: 表の置き換える文字の各群、先頭の `.`、UTF-8 で 200 バイトの境目とコードポイントの境目での切り詰め、`t` がない・空白だけのときの動画 ID、どの場合もパスの区切りと制御文字を含まず `.` で始まらず `.md` で終わること（AC-34）。

**完了の条件:** `make ext-check` が通る。切り詰めをバイトの途中で行うように壊し、境目のテストが失敗することを確かめる。

#### ステップ 3-5: 文言の新しい節と、コードの表のガード

**変更するファイル:** `src/core/i18n/catalog.ts`、`src/core/i18n/en.ts`、`src/core/i18n/ja.ts`、`src/core/sendMessages.ts`、`src/core/i18n/locale.ts`、`tsconfig.json`、`tsconfig.core.json`、`test/catalog.test.ts`、`test/sendMessages.test.ts`、`test/jobCodes.test.ts`（新規）

-   [ ] `Catalog` に、設計書 3.13 の表の節を、今の節と別の名前で加える（1.3）。キーの型を持つ節は `Record` にする。文言は設計書 3.13 の各項目（Slack に触れない、結果不明の範囲を断定しすぎない、生成の開始の `invalid_body` の最初の手順、工程の名前）に従い、用語は `docs/translation_glossary.md` に合わせる。
-   [ ] `sendMessages.ts` に、3 つの操作の結果と理由の文言を選ぶ関数を加える。`switch` の網羅性で欠けを知らせる形にする。
-   [ ] 経過時間・所要時間・推定の料金を整形する関数を加える（設計書 3.13。`locale.ts` の `formatCount` と同じ場所）。`tsconfig.json` と `tsconfig.core.json` の `lib` に `ES2023.Intl` を加える。
-   [ ] `test/jobCodes.test.ts` を作る。`development.md`「ジョブの API のコード」の 6 つの表の値の集合が、`response.ts` の `Record<…, true>` の集合と等しいことを確かめる（設計書 7.1）。表は見出しと列の名前で探し、値と構造だけを比べる。
-   [ ] テスト: 失敗の理由の 8 つの文言が互いに違うこと（AC-14）、新しい節のどの文言も Slack に触れないこと（AC-04 の値の部分）、英語と日本語の両方に文言があること（AC-37）、時間と料金の整形の例（`0.1234`、59 秒と 60 秒の境目）。

**完了の条件:** `make ext-check` が通る。`ja.ts` の失敗の理由の文言を 1 つ別の理由の文言と同じにするように壊し、AC-14 のテストが失敗することを確かめる。`response.ts` の工程の状態の集合から 1 つを消し、`test/jobCodes.test.ts` が失敗することを確かめる。

### フェーズ 4: SW と manifest

#### ステップ 4-1: ブラウザの API のラッパーと fake

**変更するファイル:** `src/browser/chromeDeps.ts`、`test/chromeDeps.test.ts`、`test/helpers/chrome.ts`（新規）

-   [ ] `chromeDeps.ts` に、設計書 3.18 の行のラッパー（`runtime.sendMessage`・`onMessage`、`tabs.create/update/remove/get/getCurrent`、`windows.update`、`BroadcastChannel`、クリップボード、ダウンロード、問い合わせの ID の乱数）を加える。Promise の拒否と例外は、どれも失敗の値に変える（設計書 4.1）。
-   [ ] `test/helpers/chrome.ts` に、それらのインタフェースの形の fake を作る（設計書 7.1）。`BroadcastChannel` の fake は、同じテストの中の複数の端点の間で届く。
-   [ ] テスト: 各ラッパーが拒否と例外を失敗の値に変えること、`lastError` を読むこと。

**完了の条件:** `make ext-check` が通る。`tabs.remove` のラッパーが拒否をそのまま投げるように壊し、そのテストが失敗することを確かめる。

#### ステップ 4-2: 同じキーのビューアの問い合わせ

**変更するファイル:** `src/viewerQuery.ts`（新規）、`test/viewerQuery.test.ts`（新規）

-   [ ] `findViewers` を作る（設計書 3.4.2）。`find` を送り、`viewerQueryWindowMs` の間、同じ `queryId` の答えを `parseViewerChannelMessage` で検査して集める。
-   [ ] テスト: fake の時計で 1 秒の境目、違う `queryId`・形の違う答えを無視すること、答えがない場合。

**完了の条件:** `make ext-check` が通る。`queryId` を比べないように壊し、違う問いの答えを無視するテストが失敗することを確かめる。

#### ステップ 4-3: 生成の開始の調整と SW の入口

**変更するファイル:** `src/startCoordinator.ts`（新規）、`src/background.ts`（新規）、`test/startCoordinator.test.ts`（新規）

-   [ ] `startCoordinator.ts` に、設計書 3.4.1 のリスナーの契約（送り手の確かめ、`true`・`false` の返し方、`AwaitStart` の受け付けと保留）と、6.1 の流れ（待ち受けの記録、背面のタブ、生成の開始を 1 回、`find`・`take-over`・閉じる前の確認、結果の配布、35 秒で記録を捨てる）を作る。ログは設計書 4.4 の SW の 4 つのラベルで、ログの関数を通した値だけを書く。
-   [ ] `background.ts` は、`runtime.onMessage` のリスナーを最上位で同期的に登録し、依存を同期の処理だけで組み立てる（設計書 3.4.1 の 1）。
-   [ ] テスト: 設計書 7.3 の AC-05 の 7 つの送り手と形（どれも `sendNative` と `tabs.create` を呼ばず `false` を返す）、7.1 の SW の 4 つの順序の組み合わせ、生成の開始のどの結果でも送り直さないこと（AC-03）、ポップアップへの応答が届かなくても結果をビューアに届けること（AC-02）、背面のタブの位置（AC-06）、閉じる・閉じない・キーが違う場合（AC-08）、ログに目印の文字列が現れないこと。

**完了の条件:** `make ext-check` が通る。`sender.tab` の確かめを外すように壊し、AC-05 の注入したスクリプトのテストが失敗することを確かめる。`waiting` の確かめを外すように壊し、閉じない場合のテストが失敗することを確かめる。

#### ステップ 4-4: manifest・ビルド・lint

**変更するファイル:** `static/manifest.json`、`scripts/check-dist.ts`、`eslint.config.js`、`test/manifest.test.ts`、`test/checkDist.test.ts`、`test/lintRules.test.ts`

-   [ ] `manifest.json` を設計書 3.15 のとおりにする（`background`、`minimum_chrome_version` を `110`）。
-   [ ] `check-dist.ts` の `referenceViolations`（`:121`）に `background.service_worker` を加える（設計書 3.15）。
-   [ ] `eslint.config.js` で、`windows` を `chromeDeps.ts` でだけ許し、`menuPathMessage` の「the extension sends only from the popup」を改める（設計書 3.17）。`brandNames` に `ArticleKey` を加え、`articleKey.ts` の `creatorConfig` を加える。
-   [ ] テスト: `test/manifest.test.ts` の「Chrome 103」（`:124-126`）とキーの一覧（`:102-118`）、`test/checkDist.test.ts` の SW の参照の欠け、`test/lintRules.test.ts:382` の `windows` の行（ほかのファイルで禁止され、`chromeDeps.ts` で許される）、`ArticleKey` と `UsdAmount` の型の断定の禁止。

**完了の条件:** `make ext-check` が通る。`dist/background.js` を消した木で `checkDist` が違反を報告することを、テストで確かめる。

#### ステップ 4-5: 入口のガードのテスト

**変更するファイル:** `test/entryPoints.test.ts`（新規）

-   [ ] TypeScript の構文木を読み（`scripts/check-dist.ts` と同じく `typescript` の API を使う）、次のことを確かめるテストを作る（1.3）。
    -   `background.ts` が `runtime.onMessage.addListener` をモジュールの最上位で呼び、その呼び出しが関数の中にも `await` の後にもないこと（設計書 3.4.1 の 1、7.1）。
    -   `runtime.onMessage` を登録するのが `background.ts` だけで、`onMessageExternal` と `externally_connectable` を使わないこと（設計書 3.17）。
    -   `createSendDeps` を呼ぶ入口が `popup.ts`・`background.ts`・`viewer.ts` だけであること、`startMessage` を呼ぶのが `startCoordinator.ts` の検査を通った求めの処理の中だけであること（設計書 3.17）。`viewer.ts` はフェーズ 5 で加わるので、入口の集合の確かめは、ない入口を許す形にせず、フェーズ 5 でこのテストに加える。`publishMessage` の確かめも、ステップ 5-5 で加える。

**完了の条件:** `make ext-check` が通る。`background.ts` のリスナーの登録を `await` の後に移すように壊し、このテストが失敗することを確かめる。`startCoordinator.ts` の外に `startMessage` の呼び出しを加えるように壊し、このテストが失敗することを確かめる。

### フェーズ 5: ビューア

#### ステップ 5-1: 骨格と CSS の分け方

**変更するファイル:** `static/theme.css`（新規）、`static/style.css`、`static/viewer.css`（新規）、`static/viewer.html`（新規）、`static/popup.html`、`test/style.test.ts`、`test/popupHtml.test.ts`、`test/helpers/dom.ts`

-   [ ] 色の変数と共有の規則を `style.css` から `theme.css` に移し、ビューアの配置を `viewer.css` に置く（設計書 3.14）。送る前のポップアップの値は変えない（設計書 3.8）。
-   [ ] `viewer.html` に、見出し・状態・記事・操作の各要素と、2 つの `role="status"` を置く（設計書 3.18）。`popup.html` と `viewer.html` は、`theme.css` を先に読み込む。
-   [ ] `test/helpers/dom.ts` に、出荷する `viewer.html` から骨格を作る補助を加える。
-   [ ] テスト: `test/style.test.ts` のコントラストの計算を `theme.css` の変数と新しい組（ビューアの失敗の色、工程の記号、ロード中の印、フォーカスの印）に広げる。ライトとダークの両方、`prefers-reduced-motion` で印と `running` の記号の `animation` を止める規則、押せる要素の幅と高さの最小値（AC-45・AC-48・AC-50・AC-51）。

**完了の条件:** `make ext-check` が通る。`theme.css` のダークの失敗の色の明るさを下げて、そのコントラストのテストが失敗することを確かめる。

#### ステップ 5-2: 状態と本文の描画の部品

**変更するファイル:** `src/ui/icons.ts`、`src/ui/renderJob.ts`（新規）、`src/ui/articleBody.ts`（新規）、`test/renderJob.test.ts`（新規）、`test/articleBody.test.ts`（新規）

-   [ ] `icons.ts` に、工程の状態の 5 つの記号とロード中の印を加える（設計書 3.14）。
-   [ ] `renderJob.ts` に、工程のチェックリスト、経過時間、料金、所要時間、ロード中の表示を描く関数を作る（設計書 3.14 の工程の表示・ロード中の印・経過時間の行）。今の `renderSend.ts` の `requestId` の表示（`src/ui/renderSend.ts:16-22`）の役割も引き継ぐ。
-   [ ] `articleBody.ts` に `renderArticleBody` を作る（設計書 3.12）。
-   [ ] テスト: 工程の状態ごとに記号の形と状態の語が違うこと（AC-11・AC-46・AC-48）、経過時間の要素が状態の領域の外で `aria-live` を持たないこと（AC-47）、`renderArticleBody` に AC-25 の入力を与えると子の要素が 1 つもできず `textContent` が一致すること。

**完了の条件:** `make ext-check` が通る。`renderArticleBody` が記事の中の Markdown のリンクを `<a>` の要素にするように壊し、AC-25 のテストが失敗することを確かめる。

#### ステップ 5-3: ビューアの描画

**変更するファイル:** `src/ui/renderViewer.ts`（新規）、`test/renderViewer.test.ts`（新規）

-   [ ] `renderViewer.ts` に、見出し・チャンネル名・元動画へのリンク・`ViewerState` ごとの状態・記事・投稿と保存とコピーの操作・操作の結果を描く関数を作る（設計書 3.6・3.9・3.12・3.14）。投稿のボタンは `publishControl` の結果で `disabled` と表示を決める。状態の領域には、表示が変わったときだけ書く。
-   [ ] テスト: `ViewerState` の各種類の要素と文言、AC-26 のリンクの属性、AC-27 の見出しとチャンネル名、`PublishRow` の各行の `disabled` と表示（AC-28・AC-32）、`lost` で記事と保存・コピーを残すこと（AC-15）、DOM の順（AC-44）、2 つの状態の領域の書き分け（AC-47）、ロード中の表示と文言（AC-52）。

**完了の条件:** `make ext-check` が通る。投稿のボタンの `disabled` を `publishControl` によらず外すように壊し、AC-32 のテストが失敗することを確かめる。

#### ステップ 5-4: ポーリングの部品

**変更するファイル:** `src/poller.ts`（新規）、`test/poller.test.ts`（新規）

-   [ ] `createPoller` を、設計書 3.7 の契約の 1〜5 のとおりに作る（1 つまで、1 つのタイマー、世代、見えたときの取得、`afterStatus` の判断）。
-   [ ] テスト（fake の時計）: 間隔、前の結果の前に次を送らないこと（AC-19）、`fetchNow` を N 回繰り返しても取得が間隔ごとに 1 回のままであること、世代（引き継ぎの前に送った取得の `failed` で止まらない、投稿の前に送った取得の `not_posted` で止まらない）、止まった後の `bumpEpoch`。

**完了の条件:** `make ext-check` が通る。前の世代の結果で止めるように壊し、世代のテストが失敗することを確かめる。

#### ステップ 5-5: ビューアの経路の処理と入口

**変更するファイル:** `src/viewerApp.ts`（新規）、`src/viewer.ts`（新規）、`test/viewerApp.test.ts`（新規）、`test/entryPoints.test.ts`

-   [ ] `viewerApp.ts` に `startViewer` を作る（設計書 3.6・6.3）。URL の検査、`AwaitStart` と 35 秒の期限、受け付けで `k` を加える `history.replaceState`、ポーリング、`find` への答え、引き継ぎの表（設計書 3.6）、投稿（3.9・6.5）、保存（3.10）、コピー（3.11）、`applyPageLocale` とページのタイトル、ログ（設計書 4.4 のビューアのラベル）を組み立てる。
-   [ ] `viewer.ts` は、`chromeDeps.ts` から依存を作って `startViewer` に渡す。
-   [ ] `test/entryPoints.test.ts` の入口の集合に `viewer.ts` を加え、`publishMessage` を呼ぶのが投稿のボタンの `click` の処理の中だけであることの確かめを加える（設計書 3.17）。
-   [ ] テスト: 6.3 の流れの各分岐（AC-07・AC-10・AC-17・AC-43）、再読み込みの後の経過時間（AC-12）、状態の取得の失敗で生成の開始と投稿の操作を送らないこと（AC-16）、投稿の流れと拒否・未送信・結果不明で送り直さないこと（AC-29〜AC-31）、保存のバイト列（改行を含む記事、BOM なし。AC-33）、コピーの成功と失敗（AC-35・AC-36）、引き継ぎの 2 つの行（AC-08・AC-22）、`lang`（AC-49）、ロード中の表示を 2 回目以降の取得で出さないこと（AC-52）、ログに目印の文字列が現れないこと。

**完了の条件:** `make ext-check` が通る。期限の後に届いた結果を表示するように壊し、AC-10 のテストが失敗することを確かめる。保存の `Blob` に BOM を足すように壊し、AC-33 のテストが失敗することを確かめる。

#### ステップ 5-6: 手動確認のプレビュー

**変更するファイル:** `scripts/popup-preview.ts`、`test/popupPreview.test.ts`

-   [ ] ビューアの各状態（`ViewerState` の 5 種類、`job` の 4 種類、`PublishRow` の 7 行）のページを書き出す（設計書 3.18）。ポップアップの新しい状態のページは、フェーズ 6 で加える。
-   [ ] テスト: 書き出したページが、状態ごとに 1 つずつあり、`theme.css` と `viewer.css` を参照すること。

**完了の条件:** `make ext-check` が通る。

### フェーズ 6: ポップアップ

#### ステップ 6-1: ポップアップの画面の内容

**変更するファイル:** `src/core/popupView.ts`、`test/popupView.test.ts`

-   [ ] 設計書 3.8 の `JobPopupView`・`PopupJobState`・`PrimaryAction`・`primaryAction` を加える。
-   [ ] テスト: `primaryAction` の表の各行（AC-20・AC-22・AC-23）。

**完了の条件:** `make ext-check` が通る。

#### ステップ 6-2: ポップアップの描画

**変更するファイル:** `src/ui/render.ts`、`test/render.test.ts`、`scripts/popup-preview.ts`、`test/popupPreview.test.ts`

-   [ ] `renderPopup` に `job` の種類を加える。`#checks` の位置に `renderJob.ts` で状態を描き、状態の変化の要約を `#status` に書き、主ボタンの名前と押せるかを `primaryAction` で決める（設計書 3.8・3.14 のポップアップの行）。
-   [ ] プレビューに、ポップアップの新しい状態のページを加える。
-   [ ] テスト: `PopupJobState` の各種類で、動画のセクションと詳細が `collected` と同じであること、`#checks` と `#status` の中身、主ボタンの名前と `disabled`（AC-20・AC-23・AC-46）、状態ごとの文言（設計書 3.8 の表）。

**完了の条件:** `make ext-check` が通る。

#### ステップ 6-3: ポップアップの送信の経路

**変更するファイル:** `src/launch.ts`、`src/popup.ts`、`test/launch.test.ts`

-   [ ] `runPopupSend` を、設計書 3.8 の「主ボタンを押した後」の流れ（`StartRequest` を SW に送る、`StartReply` の検査と 35 秒の期限、ポーリング）と、「ビューアを開く」（6.6）・「やり直す」に改める。ビューアの種は、ポップアップが表示した値から作り、対になっていないサロゲートを表示用の写しで U+FFFD に置き換える（設計書 3.5）。ログは設計書 4.4 のポップアップのラベルで、ログの関数を通した値だけを書く。
-   [ ] `popup.ts` に新しい依存を渡す。
-   [ ] テスト: 主ボタンで `StartRequest` を 1 回だけ送り、ポップアップ自身はホストに生成の開始を送らないこと（AC-01）、`sending` から各状態への移り変わり（AC-20・AC-23）、「ビューアを開く」の 3 つの分岐と生成の開始を送らないこと（AC-21）、「やり直す」が同じ入力を 1 回だけ送ること（AC-22）、開き直したポップアップが何も送らないこと（AC-24。今の `test/launch.test.ts:484` の describe の「does not send until it is clicked」を新しい経路で残す）、ログに目印の文字列が現れないこと。

**完了の条件:** `make ext-check` が通る。「ビューアを開く」が `tabs.get` で見つかったタブを使わずに `findViewers` に進むように壊し、AC-21 の 1 つ目の分岐のテストが失敗することを確かめる。

### フェーズ 7: 古いコードの削除と文書

#### ステップ 7-1: 古い部品をなくす

**変更するファイル:** 1.4 の 2 つの表の各ファイル

-   [ ] `src/core/articlesResponse.ts` と `test/articlesResponse.test.ts` をなくす。
-   [ ] `native.ts` の古い包みの `classifyNativeReply` をなくす。
-   [ ] `send.ts` の `sendArticle` をなくし、`SendDeps.sendNative` の引数の型を `HostMessage` だけにする。`test/send.test.ts:38` の describe を `exchange` のテストに置き換える。
-   [ ] `src/ui/renderSend.ts` と `test/renderSend.test.ts` をなくす。
-   [ ] `sendMessages.ts` の `serverRejectMessage`・`sendOutcomeMessage` と、`test/sendMessages.test.ts:60`・`:84` の describe をなくし、新しい文言の選び方のテストが Slack の案内の不在を確かめる形にする（設計書 3.17）。
-   [ ] `launch.ts` の `logSend` と、`test/launch.test.ts:877` の describe（`send log`）をなくす。
-   [ ] 文言の古い節（`serverReject`・`notSent`・`accepted`・`noResponse`・`hostFailed`・`unexpected`・`sending`・`primaryButton`）と、`en.ts`・`ja.ts` の値をなくす。`test/typecheck.test.ts:160` の欠かす項目を、新しい節の項目に替える。
-   [ ] Slack の方針を書いたコメント（`catalog.ts:29-32`、`sendMessages.ts:25`）を改める。
-   [ ] 消す前と後で、`node --test --experimental-test-coverage --test-reporter=lcov` の関数の実行回数を比べる（1.2）。

**完了の条件:** `make ext-check` が通る（コードの参照の残りは、型の検査が知らせる）。コメントと文書の残りは、6 章の横断の確かめで見る。

#### ステップ 7-2: ガードの更新

**変更するファイル:** `test/serverReasons.test.ts`、`test/metadataDocs.test.ts`、`test/typecheck.test.ts`

-   [ ] `test/serverReasons.test.ts` の突き合わせの相手を、新しい `ServerRejectReason` の集合と、理由から状態コードへの `Record` に改める（設計書 3.17、AC-38）。
-   [ ] `test/metadataDocs.test.ts` の未送信の文言の場所を生成の開始の節に改め、主ボタンの 3 つの名前を確かめる形にする（設計書 3.17）。
-   [ ] `test/typecheck.test.ts` に、`SendDeps.sendNative` に `HostMessage` でない値（今の形の本文）を渡すコードが型の検査を通らない例を加える（AC-01）。

**完了の条件:** ステップ 7-3・7-4 と一緒に `make ext-test` が通る。

#### ステップ 7-3: README

**変更するファイル:** `README.md`、`README.en.md`

-   [ ] `README.md` の「ブラウザ拡張」節・「localhost サーバ」節・「拒否の理由」の表を、要件書 F-010 と設計書 3.18 の行（Chrome 110、拡張の更新、ビューアの見出しは URL の値であること、キーを受け取る前にビューアとポップアップを閉じた場合の手順、拡張とサーバとホストを同じ版にそろえること）のとおりに書き直す。今の経路の Slack の案内を取り除く。「必要なもの」の「Chrome または Brave」（`README.md:78`）に版を加える。
-   [ ] `/mktrans README.md` で `README.en.md` に反映し、同じコミットにする。

**完了の条件:** `make readme-sync-check` と `make ext-test`（`metadataDocs.test.ts`）が通る。`go test -tags test ./cmd/yt2column-nmhost` の `TestReadmeHostExitCodes` が通る。

#### ステップ 7-4: 開発の手順・セキュリティ・概要・用語集

**変更するファイル:** `docs/dev/guides/development.md`、`docs/dev/security.md`、`docs/dev/project_overview.md`、`CLAUDE.md`、`docs/translation_glossary.md`

-   [ ] `development.md` の「ジョブの API」の今の拡張の記述と、拒否の理由の表の「拡張の表示」の列を改める（要件書 F-009・F-010）。
-   [ ] `security.md` §8.1・§8.2 を、要件書 F-010・5 章と設計書 5.1 の残余の列（履歴に残る値など）のとおりに改める。
-   [ ] `project_overview.md` のツールの説明と `extension/` の説明、`CLAUDE.md` の「Architecture Overview」の主経路の説明を改める。
-   [ ] `translation_glossary.md` に要件書 F-010 の用語を加え、「結果不明」「主ボタン」の定義と、「工程」の項（設計書 3.13 の 3 つの名前）を改める。

**完了の条件:** `make test` が通る（`doc-guard` が走らせる Go のガードを含む）。

#### ステップ 7-5: 統合の手動の確認

**変更するファイル:** `docs/tasks/0050_viewer_tab/03_implementation_plan.md`（4.3 の記録）

-   [ ] ビルドした拡張をサーバ・ホストとともに動かし、設計書 7.2 の「完成の後」の項目を確かめる（AC-02・AC-06・AC-08・AC-21・AC-33・AC-35・AC-44・AC-47、背面に 5 分以上置いたビューア、ライトとダークと `prefers-reduced-motion` の見え方、送る前のポップアップが変わっていないこと）。結果を 4.3 の表に記録する。

**完了の条件:** 4.3 の統合の確認の表のすべての行に結果がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

| マイルストーン | フェーズ | 成果物 |
|---|---|---|
| M1: ブラウザの前提の確定 | 1 | 4.3 の 3.16 の表の記録 |
| M2: やり取りと検査 | 2 | `exchange`・厳格な JSON・3 つの操作の応答の検査・Go のガード |
| M3: 純粋な判断と文言 | 3 | ビューアの URL・メッセージ・ポーリングの規則・投稿のボタン・ファイル名・新しい文言の節 |
| M4: SW | 4 | `background.ts`・`startCoordinator.ts`・manifest・lint の例外・入口のガード |
| M5: ビューア | 5 | `viewer.html`・CSS・描画・ポーリングの部品・`viewerApp.ts` |
| M6: 切り替えと文書 | 6・7 | ポップアップの新しい経路、古い部品の削除、文書、統合の確認 |

設計書 8 章の依存のとおり、フェーズ 2 と 3 はほかのフェーズに依存せず、フェーズ 4 は 2 に、フェーズ 5 と 6 は 2〜4 に依存する。フェーズ 1 の結果で設計書 3.16 の代わりの方法に切り替えると、フェーズ 4〜6 の作りが変わる。そのため、フェーズ 1 を最初に行う。

## 4. テスト戦略 (Test Strategy)

### 4.1. 加える・変えるテスト

ユニットテストは、設計書 7.1・7.3 の方針で、`node --test` と jsdom と `test/helpers/` の fake で組み立てる。実際のブラウザ・ホスト・サーバは呼ばない（要件書 5 章）。時刻とタイマーは注入し、fake の時計で進める。

-   **新しいテスト:** `articleKey`・`hostMessage`・`strictJson`・`viewerUrl`・`extensionMessages`・`jobView`・`poll`・`fileName`・`viewerQuery`・`startCoordinator`・`entryPoints`・`renderJob`・`articleBody`・`renderViewer`・`poller`・`viewerApp`・`jobCodes`（各ステップ）。
-   **書き換えるテスト:** `response`・`native`・`send`・`prepare`・`acceptedUrl`・`sendMessages`・`catalog`・`chromeDeps`・`manifest`・`checkDist`・`lintRules`・`style`・`popupHtml`・`popupPreview`・`popupView`・`render`・`launch`・`serverReasons`・`metadataDocs`・`secretsAbsent`・`typecheck`。
-   **一時的なテスト:** `articlesResponse`（ステップ 2-3 で作り、7-1 でなくす）。
-   **Go のテスト:** `cmd/yt2column/extension_docs_test.go::TestNativeHostContract` に応答の上限の subtest を加える（ステップ 2-5）。
-   **網羅の目標:** 新しい `src/core/` の関数と、`startCoordinator.ts`・`poller.ts`・`viewerApp.ts`・`viewerQuery.ts` の公開の関数は、どれもテストから呼ばれる（7-1 の lcov の比べ方で、実行回数が 0 の関数がないことを確かめる）。

### 4.2. 互換性のテスト

-   **古い経路:** フェーズ 2〜5 の間、ポップアップは `/articles` の経路で動く。`test/articlesResponse.test.ts` と、今の `test/launch.test.ts` の送信のテストが、その振る舞いを確かめ続ける（1.3）。
-   **古いホスト:** 古いホストが新しい形の生成の開始を `/articles` へ中継した応答を `classifyStart` に通すテスト（ステップ 2-3）。
-   **ビューアの URL の互換性:** 今の版が作った URL を読めること（設計書 3.5）は、往復の性質のテスト（ステップ 3-1）で確かめる。

### 4.3. 手動の確認の記録

#### 4.3.1. 設計書 3.16 の確認（ステップ 1-1）

試作は作業ツリーの外に作る（1.3）。各行の確かめ方は設計書 3.16 の表のとおりである。

| 振る舞い（設計書 3.16 の行） | ブラウザと版 | 確かめた日 | 結果 |
|---|---|---|---|
| `tabs` の権限なしの `tabs.create`・`update`・`remove`・`get` | - | - | 未実施 |
| `tabs.getCurrent()` が自分のタブを返す | - | - | 未実施 |
| `windows.update` が権限なしで使える | - | - | 未実施 |
| SW の `BroadcastChannel` | - | - | 未実施 |
| SW の寿命（19 秒遅らせるサーバ、25 秒応答しないホスト） | - | - | 未実施 |
| 止まっていた SW が、閉じたポップアップのメッセージを受け取る | - | - | 未実施 |
| `<a download>` と Blob の URL の保存 | - | - | 未実施 |
| 権限なしの `navigator.clipboard.writeText` | - | - | 未実施 |
| `Intl.NumberFormat.prototype.format` の 10 進の文字列 | - | - | 未実施 |
| 凍結・破棄されたタブが問いに答えない | - | - | 未実施 |

#### 4.3.2. 統合の確認（ステップ 7-5）

| 確かめること（設計書 7.2） | 関わる AC | 確かめた日 | 結果 |
|---|---|---|---|
| 主ボタンの直後にポップアップを閉じても記事が出る（SW が止まった後の最初の送信を含む） | AC-02 | - | 未実施 |
| 背面のタブで開き、作業中のタブが切り替わらない | AC-06 | - | 未実施 |
| 同じ入力の送り直しで新しいタブが閉じる。破棄・凍結した既存のタブでは残る | AC-08 | - | 未実施 |
| 「ビューアを開く」の 3 つの分岐 | AC-21 | - | 未実施 |
| 保存したファイルを `cmp` で `--out` のファイルと比べる | AC-33 | - | 未実施 |
| コピーした文字列を貼り付けて比べる | AC-35 | - | 未実施 |
| キーボードだけの操作とフォーカスの順 | AC-44 | - | 未実施 |
| VoiceOver で状態の変化が読み上げられ、経過時間が読み上げられない | AC-47 | - | 未実施 |
| 背面に 5 分以上置いたビューアを前面に出すと、すぐに状態が更新される | - | - | 未実施 |
| ライトとダーク、`prefers-reduced-motion` の見え方（プレビュー） | AC-45・AC-48・AC-51 | - | 未実施 |
| 送る前のポップアップの画面が変わっていない（プレビュー） | - | - | 未実施 |

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 設計書 3.16 のブラウザの振る舞いが文書と違う | SW・ビューア・ポップアップの作りが変わる | フェーズ 1 を最初に行い、崩れた行は設計書 3.16 の代わりの方法に、設計書を改めてから切り替える（ステップ 1-1） |
| SW が生成の開始のやり取りの途中で止められる | 結果がビューアに届かない | Chrome 110 を最小の版にし（設計書 1.2）、3.4.3 の見積もりをフェーズ 1 の 2 つの条件で確かめる。崩れれば `runtime.connect` のポートに切り替える |
| 厳格な JSON の解析が、Go の `internal/strictjson` と違う入力を受理する | 記事の取り違え、保存の U+FFFD | Go のテストの拒否の表の入力を同じバイト列で確かめる（ステップ 2-2） |
| 並べて加える間に、新旧の同じ名前の型を取り違える | 古い経路が新しい検査を使う、またはその逆 | 1.4 の台帳で古い名前を `Articles…` に改め、同じ名前が 2 つのファイルにない状態にする。フェーズ 7 の名前の残りの確かめ（ステップ 7-1） |
| 最後の PR（切り替え・削除・文書）が大きい | レビューの見落とし | レビューの観点を PR の作成ポイントに分けて挙げ、フェーズ 6 とフェーズ 7 のステップをコミットで分ける |
| ポーリングの世代の競合 | 生成し直している記事や投稿中の記事のポーリングが止まる | `poller.ts` を 1 つの部品にし、fake の時計で世代の 2 つの例を確かめる（ステップ 5-4） |
| 秘密情報の語のガードを狭めて、Webhook の URL の設定を見逃す | 拡張が URL を持つ変更に気付かない | 狭めた語が設定の名前を拒否し続けることのテストを加える（ステップ 2-3）。保存しないことは lint の規則が守る |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1: 確認の試作（ステップ 1-1）
-   [ ] フェーズ 2: やり取りと検査（ステップ 2-1〜2-5）
-   [ ] フェーズ 3: 純粋な判断と文言（ステップ 3-1〜3-5）
-   [ ] フェーズ 4: SW と manifest（ステップ 4-1〜4-5）
-   [ ] フェーズ 5: ビューア（ステップ 5-1〜5-6）
-   [ ] フェーズ 6: ポップアップ（ステップ 6-1〜6-3）
-   [ ] フェーズ 7: 古いコードの削除と文書（ステップ 7-1〜7-5）
-   [ ] 7 章の受け入れ基準の検証の各行を確かめた

**横断の確かめ（`make lint` と `make test` が見つけないもの）:**

-   [ ] 1.4 の台帳の「並べて加える間の名前」と古い部品の名前が、フェーズ 7 の後の `extension/` のコメントと、`docs/`・README・`CLAUDE.md` に残っていない（コードの参照は型の検査が知らせる）。
-   [ ] `/articles` の経路と Slack を確かめる案内を述べるコメントが、`extension/src/` に残っていない（設計書 3.13 の「Slack に触れない」）。
-   [ ] 新しい用語（ビューア・Service Worker・ビューアの URL・送信の時刻・ポーリング・やり取りの結果）と工程の名前が、`docs/translation_glossary.md`・README・文言で同じ語を使っている（PR のレビューで読む）。

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類: `test` は実行するテスト、`static` は型の検査・lint・構文木のガードのテスト、`manual` は PR のレビューで読むことか、4.3 の手動の確認である。

| AC | 検証 | 種類 | ステップ |
|---|---|---|---|
| AC-01 | `extension/test/startCoordinator.test.ts::start message`（ホストが受け取る生成の開始が 1 つで、同じ収集の値からの本文と同じメンバと値）、`extension/test/hostMessage.test.ts::host messages`、`extension/test/launch.test.ts::popup job send`（ポップアップはホストに生成の開始を送らない）、`extension/test/typecheck.test.ts::sendNative accepts only HostMessage` | test・static | 2-1・4-3・6-3・7-2 |
| AC-02 | `extension/test/startCoordinator.test.ts::popup closed`、4.3.2 | test・manual | 4-3・7-5 |
| AC-03 | `extension/test/startCoordinator.test.ts::no resend`（拒否・未送信・応答なし・ホストの失敗・想定外の応答のそれぞれ） | test | 4-3 |
| AC-04 | `extension/test/catalog.test.ts::catalogs`（新しい節のどの文言も Slack に触れない）、英語と日本語の文言を PR のレビューで読む | test・manual | 3-5・7-1 |
| AC-05 | `extension/test/startCoordinator.test.ts::start request sender checks`（設計書 7.3 の 7 つ） | test | 4-3 |
| AC-06 | `extension/test/startCoordinator.test.ts::viewer tab`（`active:false`、YouTube のタブの隣）、`extension/test/launch.test.ts::popup job states`（押した後も状態を表示する）、4.3.2 | test・manual | 4-3・6-3・7-5 |
| AC-07 | `extension/test/viewerApp.test.ts::start result`（受け付けで `k` を加える）、`extension/test/viewerApp.test.ts::reopened with a key`、`extension/test/viewerUrl.test.ts::round trip` | test | 3-1・5-5 |
| AC-08 | `extension/test/startCoordinator.test.ts::reuse`（閉じる・`waiting` なし・`took-over` なし・`tabs.remove` の失敗・キーが違う）、`extension/test/viewerApp.test.ts::take-over`、4.3.2 | test・manual | 4-3・5-5・7-5 |
| AC-09 | `extension/test/viewerUrl.test.ts::no transcript or description`（目印の文字列） | test | 3-1 |
| AC-10 | `extension/test/viewerApp.test.ts::start result deadline` | test | 5-5 |
| AC-11 | `extension/test/poll.test.ts::afterStatus`、`extension/test/renderJob.test.ts::step checklist` | test | 3-3・5-2 |
| AC-12 | `extension/test/viewerApp.test.ts::elapsed time`（fake の時計、再読み込みの後） | test | 5-5 |
| AC-13 | `extension/test/poll.test.ts::afterStatus`、`extension/test/renderViewer.test.ts::done` | test | 3-3・5-3 |
| AC-14 | `extension/test/poll.test.ts::afterStatus`、`extension/test/renderViewer.test.ts::failed`、`extension/test/catalog.test.ts::catalogs`（8 つの文言が互いに違う） | test | 3-3・3-5・5-3 |
| AC-15 | `extension/test/poll.test.ts::afterStatus`（`kept`）、`extension/test/renderViewer.test.ts::lost` | test | 3-3・5-3 |
| AC-16 | `extension/test/poll.test.ts::afterStatus`、`extension/test/viewerApp.test.ts::status exchange failures` | test | 3-3・5-5 |
| AC-17 | `extension/test/viewerApp.test.ts::start failed`、`extension/test/renderViewer.test.ts::start-failed` | test | 5-3・5-5 |
| AC-18 | `extension/test/poll.test.ts::afterStatus`（`posting`）、`extension/test/viewerApp.test.ts::posting on load` | test | 3-3・5-5 |
| AC-19 | `extension/test/poller.test.ts::one in flight` | test | 5-4 |
| AC-20 | `extension/test/popupView.test.ts::primaryAction`、`extension/test/render.test.ts::job view`、`extension/test/launch.test.ts::popup job states` | test | 6-1・6-2・6-3 |
| AC-21 | `extension/test/launch.test.ts::open viewer`（3 つの分岐と、生成の開始を送らないこと）、`extension/test/viewerQuery.test.ts::findViewers`、4.3.2 | test・manual | 4-2・6-3・7-5 |
| AC-22 | `extension/test/launch.test.ts::retry`、`extension/test/startCoordinator.test.ts::reuse`、`extension/test/viewerApp.test.ts::take-over`（失敗か不明から `loading` に戻り、新しい送信の時刻で数える） | test | 4-3・5-5・6-3 |
| AC-23 | `extension/test/popupView.test.ts::primaryAction`、`extension/test/render.test.ts::job view`、`extension/test/launch.test.ts::popup job states` | test | 6-1・6-2・6-3 |
| AC-24 | `extension/test/launch.test.ts::popup send`（開き直したポップアップは押すまで何も送らない） | test | 6-3 |
| AC-25 | `extension/test/articleBody.test.ts::renderArticleBody` | test | 5-2 |
| AC-26 | `extension/test/renderViewer.test.ts::video link`、`extension/test/viewerUrl.test.ts::parseViewerUrl`（受け付けない URL を拒否する） | test | 3-1・5-3 |
| AC-27 | `extension/test/renderViewer.test.ts::heading`、`extension/test/viewerApp.test.ts::page title` | test | 5-3・5-5 |
| AC-28 | `extension/test/jobView.test.ts::publishControl`、`extension/test/renderViewer.test.ts::publish rows` | test | 3-3・5-3 |
| AC-29 | `extension/test/viewerApp.test.ts::publish` | test | 5-5 |
| AC-30 | `extension/test/jobView.test.ts::publishControl`、`extension/test/viewerApp.test.ts::publish` | test | 3-3・5-5 |
| AC-31 | `extension/test/viewerApp.test.ts::publish failures`（送り直さない）、英語と日本語の文言を PR のレビューで読む | test・manual | 5-5・7-1 |
| AC-32 | `extension/test/jobView.test.ts::publishControl`、`extension/test/renderViewer.test.ts::publish rows`（`disabled`） | test | 3-3・5-3 |
| AC-33 | `extension/test/viewerApp.test.ts::save`（バイト列、改行、BOM なし）、4.3.2 | test・manual | 5-5・7-5 |
| AC-34 | `extension/test/fileName.test.ts::articleFileName` | test | 3-4 |
| AC-35 | `extension/test/viewerApp.test.ts::copy`、4.3.2 | test・manual | 5-5・7-5 |
| AC-36 | `extension/test/viewerApp.test.ts::copy`（拒否と例外） | test | 5-5 |
| AC-37 | `make ext-typecheck`（`Catalog` の `Record` の節）、`extension/test/catalog.test.ts::catalogs`、`extension/test/typecheck.test.ts::a Catalog missing an item fails typecheck` | static・test | 3-5・7-1 |
| AC-38 | `extension/test/serverReasons.test.ts` | test | 7-2・7-4 |
| AC-39 | 文書を PR のレビューで読む | manual | 7-3・7-4 |
| AC-40 | `make ext-test`（`serverReasons.test.ts`・`metadataDocs.test.ts`・`manifest.test.ts`）、`make test`（`cmd/yt2column::TestReadmeSyncMarker`・`TestExtensionDocumentation`・`TestNativeHostContract`・`TestDeclaredPermissions`） | test | 7-2・7-3・7-4 |
| AC-41 | `extension/test/response.test.ts::classifyStatus`、`extension/test/strictJson.test.ts::parseStrictJson` | test | 2-2・2-3 |
| AC-42 | `extension/test/response.test.ts::classifyStatus`（256 KiB いっぱいの完了した記事） | test | 2-3 |
| AC-43 | `extension/test/viewerUrl.test.ts::parseViewerUrl`、`extension/test/viewerApp.test.ts::invalid link`（状態の取得を送らない） | test | 3-1・5-5 |
| AC-44 | `extension/test/renderViewer.test.ts::order`（DOM の順と、ネイティブの要素）、4.3.2 | test・manual | 5-3・7-5 |
| AC-45 | `extension/test/style.test.ts`（フォーカスの印のコントラスト、ライトとダーク）、4.3.2 | test・manual | 5-1・7-5 |
| AC-46 | `extension/test/renderJob.test.ts::step checklist`（状態の語）、`extension/test/renderViewer.test.ts::controls`、`extension/test/render.test.ts::job view` | test | 5-2・5-3・6-2 |
| AC-47 | `extension/test/renderViewer.test.ts::status regions`、`extension/test/renderJob.test.ts::elapsed time`（`aria-live` なし）、`extension/test/viewerApp.test.ts::announces on change`、4.3.2 | test・manual | 5-2・5-3・5-5・7-5 |
| AC-48 | `extension/test/style.test.ts`（文字と図形のコントラスト）、`extension/test/renderJob.test.ts::step checklist`（形の違い） | test | 5-1・5-2 |
| AC-49 | `extension/test/viewerApp.test.ts::page language` | test | 5-5 |
| AC-50 | `extension/test/style.test.ts`（押せる要素の幅と高さ） | test | 5-1 |
| AC-51 | `extension/test/style.test.ts`（`prefers-reduced-motion` の規則）、4.3.2 | test・manual | 5-1・7-5 |
| AC-52 | `extension/test/renderViewer.test.ts::loading`、`extension/test/viewerApp.test.ts::loading` | test | 5-3・5-5 |

`design_handoff.md` の H-01〜H-14 は、どれも設計書の付録 A に扱いが記録されている。`implementation_handoff.md` はない。

## 8. 成功基準 (Success Criteria)

-   **機能:** 7 章のすべての行の検証が通り、4.3 のすべての行に結果がある。
-   **品質:** グリーンゲート（`make test && make lint`）と `make ext-check` が通る。新しいテストのそれぞれについて、壊して失敗することを確かめ、コミットのメッセージに書いた（1.2）。
-   **セキュリティ:** AC-05・AC-25・AC-41・AC-43 のテスト、ログの目印のテスト、入口のガードのテスト（`test/entryPoints.test.ts`）、狭めた秘密情報の語のガードが通る。manifest の権限は変わらない（`test/manifest.test.ts`、`cmd/yt2column::TestDeclaredPermissions`）。
-   **文書:** 要件書 F-010 の文書を改め、`README.en.md` が `/mktrans` で同期している（`make readme-sync-check`）。

## 9. 次のステップ (Next Steps)

-   最後の PR のマージの後、タスクの issue（#503）を閉じる（`/runpr` が案内する）。
-   #504（Markdown の描画）は、`renderArticleBody`（設計書 3.12）を差し替える。
-   #526（`/articles` とホストの今の形の中継の削除）は、拡張の変更なしに進められる（設計書 9 章）。
