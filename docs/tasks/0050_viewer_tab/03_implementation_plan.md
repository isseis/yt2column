# 実装計画書：拡張を新しい API に移し、ビューアのタブで記事を表示・投稿・保存する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-31 |
| Review date | 2026-11-01 |
| Reviewer | isseis |
| Comments | 2026-11-01 (decision, approved in session): ステップ 1-1 の確認で、設計書 3.16 の行 10 が崩れた（凍結されたタブは問いに答え、破棄されたタブには新しいタブの ID が付く）。レビュアー（isseis）がセッションの中で扱いを選び、承認した。凍結されたタブは期待を観察に合わせ、破棄されたタブは「ビューアを開く」の重複を制約として受け入れた（要件 F-004・AC-21、設計書 3.16・4.2・6.6・7.2）。これに合わせて 4.3.2 の AC-08 の行を改めた。行 6 は、送り方を変えないことをレビュアーが選んだ（設計書 3.4.1 の項目 5）。ほかの決定は変えていない。<br>2026-11-01 (editorial correction): PR-4 の実装のレビューで、ステップ 3-3 の `afterStatus` の記述に、直前が `kept` を持つ `lost` の場合を加えた（設計書 3.6 の同じ訂正。AC-15）。あわせて設計書 3.9 の `PublishOpNotice` の `unknown` にやり取りの結果を持たせた。決定は変えていない。<br>2026-11-01 (editorial correction): PR-5 の実装で、ステップ 4-3 の求めの検査に合わせ、`parseStartRequest`（ステップ 3-2）が、落ちた検査の種類（`shape`・`size`）と、通ったときは測った `PreparedRequest` を返す形にした（設計書 3.4 の同じ訂正。4.4 のログのため）。検査の内容は変えていない。決定は変えていない。<br>2026-11-02 (editorial correction): PR-6 の実装で、7 章の `renderViewer.test.ts`・`renderJob.test.ts` の仮の describe の名前を、実装で決めた名前に改めた。あわせて、設計書 3.14 の状態の表示の要約に投稿の状態の変化を加えた訂正（AC-47）に合わせ、AC-47 の行に `publish state announcements` を加えた。決定は変えていない。<br>2026-11-02 (editorial correction): PR-7 の実装で、設計書 3.7 の `createPoller` が注入で受け取るものを、状態の取得とタイマーに改めた（部品は時刻を読まないので `now()` を除いた）。ステップ 5-4 の記述は変わらない。決定は変えていない。<br>2026-11-02 (editorial correction): PR-7 の実装で、設計書 3.6・3.8 の `loading` に `notice` を加えた（最初の状態の取得の失敗を表示するため。AC-16）。PR-6 の `renderViewer` を合わせて改めた。ステップ 6-1・6-2 は、改めた `PopupJobState` に従う。決定は変えていない。 |

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
-   **古い経路は、ポップアップを切り替えるまで残す（1.3）。** フェーズ 2〜5 は、新しい部品を `/articles` の経路と並べて加える。どの PR をマージした後も、main の拡張は今の経路で動く。古い部品は、ポップアップを切り替えるステップ 6-3 と同じ PR（PR-9。3.2）で取り除く。
-   **テストが失敗しうることを、壊して確かめる。** 新しいテストと、確認の内容を変えたテストのそれぞれについて、確かめる経路を壊し、そのテストが失敗することを実装のときに確かめ、結果をコミットのメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。各ステップの「完了の条件」に、壊す対象を挙げる。負の例のテストは、受理される入力から 1 つの値だけを変えたものにし、変える前の入力が受理されることを先に確かめる（`CLAUDE.md`「A layered path needs inputs only one layer can handle」）。
-   **テストを消すときは、守っていた性質の行き先を確かめる。** 消すテストと書き換えるテストの確認ごとの行き先を 1.4 の表に書き、引き継ぐテストに同じ確認があることをレビューで確かめる。あわせて、拡張には `go tool cover` がないので、タスク `0046` と同じく `node --test --experimental-test-coverage --test-reporter=lcov` の出力を、消す前と後で比べる。消した関数を除く `src/` の各関数について、実行回数（lcov の `FN`・`FNDA`）が 0 にならないことを確かめ、比べたコマンドと結果をコミットのメッセージに書く。この比べは、テストを消すか移すステップ（2-1・2-4・6-3・7-1）のそれぞれで行う（`CLAUDE.md`「Deleting a test is a claim that must be checked」の拡張での読み替え）。
-   **各ステップの後でゲートが通る。** 原則としてステップごとにコミットし、どのコミットでも `make ext-check` が通る状態を保つ。`extension/src/` か `extension/static/` を変えるステップでは、`make test` も通す。Go のガードが拡張のファイルを読むためである（`internal/pipeline/relay_guard_test.go:328` の `TestSharedTokenRemoved` は 2 つのディレクトリの全体を走査し、`TestNativeHostContract` と `TestDeclaredPermissions` は `send.ts`・`native.ts`・`exchange.ts`・`manifest.json` を読む）。各ステップの完了の条件は、`make ext-check` だけを挙げていても、この規則で `make test` を含む。例外はフェーズ 7 のステップ 7-1〜7-4 で、1 つのコミットにする（1.3）。PR の前には、グリーンゲート（`make test && make lint`）も通す。
-   **拡張のソースは英語で書く。** コメント・識別子・文字列リテラルは英語である。例外は `CLAUDE.md`「Go Idioms」の 3 つ（`ja.ts` の文言、日本語の文言の性質を確かめるテストの語の表、日本語の文書に一致させるガードのテストのリテラル）だけである。`test/jobCodes.test.ts` が表を探す日本語の見出しと列の名前は、3 つ目の例外に当たり、今の `test/helpers/developmentDoc.ts` に置く（設計書 7.1）。
-   **秘密情報の語を書かない。** `test/secretsAbsent.test.ts:25` は、`src/` と `static/` のどのファイルでも `token` の語を禁じる（大文字と小文字を区別しない）。ステップ 2-3 で狭めた後も、`webhook url` の語は禁じたままである。厳格な JSON の解析の識別子とコメントでは、字句を指す語として `token` を使わない（別の語を選ぶ）。英語の文言でも、「Webhook URL is not set」のような言い方で `webhook url` を使わない。
-   **ソースの値をログに出さない。** 新しいログは、設計書 4.4 のログの関数を通した値だけを書く。
-   **フェーズは設計書 8 章の順序に従う。** フェーズ 1〜7 は、設計書 8 章の 1〜7 に当たる。

### 1.3. 計画の際に決めたこと（2026-10-31）

-   **新しい部品は古い経路と並べて加え、古い部品はポップアップの切り替えと同じ PR で取り除く（レビュアーが選んだ）:** 今のポップアップは、`sendArticle`（`src/send.ts:28-55`）→ `classifyNativeReply`（`src/core/native.ts:114-171`）→ `classifyResponse`（`src/core/response.ts:158-223`）の経路で `/articles` を使う。設計書 8 章の 2 は「`response.ts` を書き換える」と書くが、その場で書き換えると、ポップアップを切り替えるまでの間、main の拡張が動かなくなる。そこで、今の `/articles` の検査を一時的なファイル `src/core/articlesResponse.ts` に移し（ステップ 2-1）、`response.ts` を新しい 3 つの操作の検査として書く（1.4 の台帳）。文言の新しい節も、古い節と別の名前で加える。古いファイルと節は、フェーズ 6・7 で取り除く。最終の形は設計書のとおりで、途中の形だけが設計書の記述と違う。設計書 8 章の 7 が `/articles` の検査の削除を最後の段階に置いているので、設計の決定は変えない。
    -   並べて加える間の、古い経路の振る舞いの変化は 1 つだけである。拡張自身の 25 秒の時間切れが、想定外の応答ではなく応答なしになる（ステップ 2-4。設計書 3.1）。どちらの文言も今は結果不明として Slack を確かめるよう案内するので、利用者がすることは変わらない。
-   **フェーズ 1 の試作と道具は、リポジトリに入れない（レビュアーが選んだ）:** 設計書 3.16 の確認に使う試作の拡張、応答を約 19 秒遅らせるテスト用のサーバ、25 秒応答しないテスト用のホストは、作業ツリーの外（スクラッチ）で作る。本書には、手順の要点、ブラウザの版、確かめた結果だけを記録する（4.3）。捨てるコードをレビューする手間に見合わないためである。
-   **ポップアップの切り替え（ステップ 6-3）と、古い部品の削除・文書（フェーズ 7）は、同じ PR にする（レビュアーが選んだ）:** 切り替えると、README の「結果は Slack に届く」などの案内が画面と食い違う。同じ PR にすれば、main の文書と挙動が食い違う期間がない。ポップアップの画面の内容と描画（6-1・6-2）は、まだ到達しない種類を加えるだけなので、その前の PR に分ける（3.2）。
-   **最小の Chrome の版は、README より先に上げる:** `minimum_chrome_version` は PR-5（ステップ 4-4）で 110 になり、README の版の記述とそのガードは PR-9（ステップ 7-2・7-3）で加わる。その間、main の拡張は Chrome 103〜109 に読み込めないが、README はそれを書いていない。利用者は 1 人で、使うブラウザは 110 より新しいので、この間を許す。
-   **ステップ 7-1〜7-4 は 1 つのコミットにする:** 古い文言の節をなくすと、それを読むガード（`test/metadataDocs.test.ts:51` の主ボタンの名前）の書き換えと README の書き直しが要り、`:66` を読む確認を消すことになる。ガードの突き合わせの相手を新しい理由の集合に替えると、`development.md` の表の書き直しが要る。README の変更は `/mktrans` の `README.en.md` と同じコミットにする決まりもある（`CLAUDE.md`「Translation Guidelines」）。そのため、これらを 1 つの単位にし、1 回のゲートで確かめる。
-   **秘密情報の語のガードを狭める（レビュアーが選び、設計書 3.17 に加えた）:** `test/secretsAbsent.test.ts:10-18` は `webhook` と `SLACK_WEBHOOK_URL` の語を禁じ、本設計の応答のメンバ・理由・文言と両立しない。設計書 3.17「秘密情報の語のガード」のとおりに狭める（ステップ 2-3）。
-   **ブランドの型の別名の lint の規則を狭める（レビュアーが選び、設計書 3.17 に加えた）:** `eslint.config.js:110-147` の規則は、作り手でないファイルで、ブランドの名前を右辺のどこかに含む型の別名を禁じる。`type X = { readonly id: RequestId } | …` が `no-restricted-syntax` の違反になることを、`bf314092` で `npx eslint --stdin --stdin-filename src/core/sample.ts` に通して確かめた（同じ内容の `interface` は通る）。本設計の判別共用体と、ステップ 2-1 で `articlesResponse.ts` に移す `SendOutcome`（`RequestId` を含む）がこれに当たるので、設計書 3.17「ブランドの型の別名の lint の規則」のとおりに狭める（ステップ 2-1）。
-   **`docs/dev/production_profile.md` も改める（レビュアーが選んだ）:** `production_profile.md:62-65` は「拡張のポップアップは受理（`202`）を示し、失敗の通知が Webhook へ投稿される」と書く。要件書 F-010 の文書の一覧にないが、本タスクの後は古くなるので、ステップ 7-4 で改め、AC-39 の読む対象に含める。要件書の一覧の漏れを補うもので、決定は変えない。
-   **ビューアの URL の書き換えと、ページが見えるかどうかは、依存として注入する:** jsdom 30.1.2 は、`chrome-extension://` の URL の文書で `history.replaceState` を呼ぶと `SecurityError` を投げ、`document.visibilityState` は `prerender` を返す（`bf314092` の `extension/node_modules` で確かめた）。そのため、`startViewer` は URL の書き換え・見えるかどうか・見えるようになったことの通知を、依存として受け取る。`viewer.ts` は、それぞれを `history.replaceState`・`document.visibilityState`・`visibilitychange` で作る。
-   **設計書 3.17 の「送る場所」の確かめと SW のリスナーの登録の確かめは、ガードのテストにする:** 設計書 3.17 は、送ってよいきっかけの静的な確かめを「実装計画に記録する」と書き、7.1 はリスナーの登録を構文木で確かめると書く。本書は、どちらも一度だけ実行して記録するのではなく、`test/entryPoints.test.ts`（新規）として、TypeScript の構文木を読むテストにする（ステップ 4-5・5-5）。後の変更で崩れたときに気付けるようにするためである。
-   **ログの値を作る関数は `src/core/response.ts` に置く:** 設計書 4.4 は `startLogView`・`statusLogView`・`publishLogView` の置き場所を決めていない。これらは結果の型（`…OutcomeLog`）だけを読むので、その型と同じ `response.ts` に置く。
-   **`chrome.windows` を参照するのは `chromeDeps.ts` の 1 か所だけにする:** `eslint.config.js:99` の `menuPathNames` の禁止は、テストを含むすべてのファイルに当たる。設計書 3.17 は `chromeDeps.ts` だけを例外にする。そこで、ウィンドウを前面に出すラッパーは、API の形の引数（例: `focusWindow` という名前の依存）を受け取る関数にし、その引数に `chrome.windows` を結び付けるのは `chromeDeps.ts` の中の 1 か所だけにする。入口（`popup.ts` など）は `chrome.windows` を渡さない。依存のメンバと fake の名前にも `windows` を使わず、ラッパーのテストは fake をその引数で渡す。

### 1.4. 既存コード調査結果

`bf314092` で確かめた。設計書 1.4・3.17・3.18 が挙げる部品は、すべて設計書の場所にある。以下は、設計書に書かれていないことと、計画の上で注意が要ることである。

**名前の変更の台帳:** 並べて加える間（1.3）だけ、今の `/articles` の部品を次の名前に移す。本書のほかの場所と、コード・テストの参照は、この表に従う。

| 今の名前（`bf314092`） | 並べて加える間の名前（ステップ 2-1 から） | フェーズ 7 の後 |
|---|---|---|
| `src/core/response.ts` の `/articles` の部分（`ServerRejectReason`・`serverRejectStatus`・`isServerRejectReason`・`maxResponseBytes`・`ReceivedResponse`・`UnexpectedCheck`・`SendOutcome`・`SendResult`・`toSendOutcome`・`ResponseResult`・`classifyResponse`） | `src/core/articlesResponse.ts` の `ArticlesRejectReason`・`articlesRejectStatus`・`isArticlesRejectReason`・`maxArticlesResponseBytes`・`ArticlesReceivedResponse`・`ArticlesUnexpectedCheck`・`SendOutcome`・`SendResult`・`toSendOutcome`・`ArticlesResponseResult`・`classifyArticlesResponse` | なし（ファイルごと削除） |
| `NotSentCause`（`src/core/response.ts:68`） | `src/core/exchange.ts` の `NotSentCause`（同じ値。設計書 3.1）。`articlesResponse.ts` はこれを import する | 同じ |
| `UnexpectedCheck`（新しいもの） | `src/core/exchange.ts` の `UnexpectedCheck`（設計書 3.3）。`ArticlesUnexpectedCheck` とは別の型で、同じ名前の型は 2 つのファイルにない | 同じ |
| `RequestId`・`parseRequestId`（`src/core/response.ts:42-53`） | 新しい `src/core/response.ts` に残す | 同じ |
| `hasExactKeys`（`src/core/response.ts:131-141`） | `src/core/exchange.ts` に移す（新しい `response.ts` は厳格な JSON の木を読むので使わない。今の呼び出しは `native.ts:139`・`:149` と `classifyResponse` だけである）。`native.ts` と `articlesResponse.ts` は `exchange.ts` から import するので、設計書 2.2 の「`native.ts` と `response.ts` は互いに import しない」と、`exchange.ts` がどのモジュールも import しないことを保つ | `exchange.ts`（`native.ts` だけが使う） |
| `classifyNativeReply`（`src/core/native.ts:114`） | `classifyHostReply`（`HostOutcome` を返す。設計書 3.1）と、それを `SendResult` に変える `classifyNativeReply`（古い経路のための包み） | `classifyHostReply` だけ |
| `test/response.test.ts` の `classifyResponse`・`toSendOutcome`・`serverRejectStatus` の describe | `test/articlesResponse.test.ts` | なし（ファイルごと削除） |

文言の新しい節の名前は、実装のときに今の節と重ならない名前を選び、フェーズ 7 の後も改めない。

**古い部品と、それを使う場所:**

| 部品 | 定義 | 使う場所 | なくすステップ |
|---|---|---|---|
| `logSend`・`sendLabel` | `src/launch.ts:133-150`・`:69` | `src/launch.ts:228`、`test/launch.test.ts:65`・`:517-521`・`:638-641`・`:750`・`:780`・`:877` の describe（`send log`） | 6-3 |
| `runPopupSend` の古い送信の処理と、その import（`sendArticle`・`renderSendOutcome`・`toSendOutcome`・`SendResult`） | `src/launch.ts:19`・`:24`・`:203-233` | `test/launch.test.ts:484` の describe（`popup send`） | 6-3 |
| `primaryButton` | `src/core/i18n/catalog.ts:61`、`en.ts:148`、`ja.ts` の対応する行 | `src/ui/render.ts:305`、`test/render.test.ts:172`・`:344`・`:354`・`:573`、`test/launch.test.ts:1034-1066`、`test/metadataDocs.test.ts:51`、`test/typecheck.test.ts:160` | 6-3 で使う場所（送る前の主ボタンを含む）を主ボタンの 3 つの名前に替え、7-1 で定義をなくす。6-2 は `job` の種類にだけ新しい名前を使い、送る前の画面の名前は変えない |
| `sending` | `catalog.ts:39`、`en.ts:116-122`、`ja.ts` の対応する行 | `src/launch.ts:224`、`test/launch.test.ts:540`、`test/sendMessages.test.ts:195`・`:257` | 6-3 で `launch.ts` と `launch.test.ts` の使う場所を替える。`sendMessages.test.ts` の使う場所は、7-1 でその describe とともになくし、7-1 で定義をなくす |
| `sendArticle` | `src/send.ts:28-55` | `test/send.test.ts:38` の describe | 7-1 |
| `renderSendOutcome` | `src/ui/renderSend.ts:26-34` | `test/renderSend.test.ts`（全体） | 7-1 |
| `serverRejectMessage`・`sendOutcomeMessage` | `src/core/sendMessages.ts:18-23`・`:57-80` | `src/ui/renderSend.ts:7`、`test/sendMessages.test.ts:60`・`:84` の describe | 7-1 |
| 文言の節 `serverReject`・`notSent`・`accepted`・`noResponse`・`hostFailed`・`unexpected` | `catalog.ts:27-37`、`en.ts:40-115`、`ja.ts` の対応する行 | `src/core/sendMessages.ts`、`test/catalog.test.ts`、`test/sendMessages.test.ts`、`test/metadataDocs.test.ts:66`（7-2 で確認ごと消す） | 7-1 |
| `/articles` の Slack の案内のコメント | `catalog.ts:29-32`、`sendMessages.ts:25` | - | 7-1 |

**書き換える・なくすテストが守っていた性質の行き先:** 1.2 の lcov の比べ方は関数の実行回数しか見ないので、文言や表示の否定の確認が消えても気付かない。そのため、確認ごとの行き先を次の表に決める。

| 今の確認 | 行き先 |
|---|---|
| `test/sendMessages.test.ts:244`「設定画面・トークン・ポートに触れない」（今の節のほか、`rejection`・`preflight`・`expandPrompt`・`descriptionNotSent` も見る） | `test/catalog.test.ts::catalogs` に移し、文言の集合のすべての節（古い節と新しい節）を見る形にする（ステップ 3-5） |
| `test/sendMessages.test.ts:216`・`:223`（ホストの失敗の文言がホストのインストールの手順を、未送信の文言がサーバの起動を示す） | 3 つの操作のやり取りの結果の節のそれぞれで確かめる（ステップ 3-5） |
| `test/sendMessages.test.ts:230`（想定外の応答の検査の種類によらず同じ文言）、`:72`・`:168`（理由ごとと結果ごとの文言が互いに違う） | 新しい文言の選び方のテスト（ステップ 3-5） |
| `test/renderSend.test.ts:52`・`:60`・`:73`（検査の種類を表示しない、`requestId` を文字として描く、描き直しで中身を置き換える） | `test/renderJob.test.ts`（ステップ 5-2） |
| `test/launch.test.ts:877`（`send log`。ログのキーの集合、ブラウザのエラーの文字列と想定外の応答の状態コードをログに書くが表示しない） | SW・ポップアップ・ビューアのログのテスト（ステップ 4-3・5-5・6-3） |
| `test/launch.test.ts:484`（`popup send`。続けて押しても 1 回だけ送る、切り詰めて表示した動画タイトルの全文を送る、展開の促しを表示している間も、表示した概要欄を送る、大きすぎるときと拒否のときに送らない） | `test/launch.test.ts` の新しい経路のテスト（ステップ 6-3）。全文は `StartRequest.input` で、表示した値は `StartRequest.viewer` で確かめる |
| `test/send.test.ts:82`・`:92`・`:102`・`:39`（契約に反して `sendNative` が拒否・例外を出す、応答を読む間に例外が起きる、上限のタイマーを取り消す） | `test/send.test.ts` の `exchange` のテスト（ステップ 2-4） |
| `test/native.test.ts:272`・`:281`（1,024 バイトの境目） | `test/native.test.ts` の古い包みのテスト（ステップ 2-4）。512 KiB の境目は `classifyHostReply` のテストで確かめる |

**既存のガードとの衝突:** 次の 4 つがある（箇条書きは 3 項目で、最初の項目が 2 つ分を扱う）。
-   `test/secretsAbsent.test.ts:10-18` と、`eslint.config.js:110-147` のブランドの型の別名の規則は、1.3 のとおり狭める。
-   `test/sendMessages.test.ts:47-50`・`:176-211` は、結果不明の文言が Slack を確かめるよう求めることを確かめる。これは設計書 3.17 が書き換えると決めたテストで、フェーズ 7 でなくし、新しい文言の確かめ（ステップ 3-5）に任せる。
-   lint の `menuPathNames`（`eslint.config.js:99`）は `windows` を禁じ、設計書 3.17 のとおり `chromeDeps.ts` でだけ許す（ステップ 4-1、1.3）。

`networkNames`（`eslint.config.js:58`）は、新しいコードが使わない API なので変えない。

**外部の前提の確かめ:**

-   **`ES2023.Intl`:** `node_modules/typescript/lib/lib.es2023.intl.d.ts:57` の `NumberFormat.format` は `number | bigint | StringNumericLiteral` を受け取る（TypeScript 6.0.3、`extension/node_modules`）。設計書 3.13 の型の検査の前提は成り立つ。
-   **`chrome.tabs.getCurrent`:** `node_modules/@types/chrome/index.d.ts:12002` に `getCurrent(): Promise<Tab | undefined>` がある。ブラウザでの振る舞いはフェーズ 1 で確かめる（設計書 3.16）。
-   **`src/core/` の型:** `tsconfig.core.json` は `lib` が `ES2022` だけで、`TextDecoder`（`fatal`・`ignoreBOM`）・`TextEncoder`・`URL`・`URLSearchParams.getAll` を `types/core-encoding.d.ts`・`types/core-url.d.ts` で宣言する。`strictJson.ts` の UTF-8 の検査と `watchVideoId` は、この宣言の中で書ける。`encodeURIComponent`・`decodeURIComponent` は ES の標準の関数なので、`viewerUrl.ts` も `src/core/` に置ける。`BroadcastChannel` と `crypto.getRandomValues` を呼ぶのは `chromeDeps.ts` だけで、`viewerQuery.ts` は通信路と乱数を依存として受け取る（設計書 1.1）。
-   **jsdom:** `history.replaceState` と `visibilityState` は 1.3 のとおり。`Blob.prototype.arrayBuffer` は jsdom 30.1.2 にあり、保存のバイト列をテストで読める。`URL.createObjectURL`・`navigator.clipboard`・`BroadcastChannel` は jsdom にないので、どれも依存として fake を渡す（設計書 7.1）。
-   **Node.js:** `extension/.node-version` は `24.21.0` で、`node --version` も `v24.21.0` である。
-   **時刻の fake:** 今の `manualSchedule`（`test/helpers/native.ts:73`）の `fire()` は、待ち時間によらず置かれたすべての処理を呼ぶ。1 秒・2 秒・25 秒・35 秒の境目を確かめるには足りないので、時刻を進める fake の時計を加える（ステップ 4-1）。
-   **Go の厳格な JSON のテスト:** 設計書 7.1 が「`internal/strictjson` のテストが拒否する入力の代表」と呼ぶものは、`internal/strictjson/strictjson_test.go:37-50` の `TestParseObject` の拒否の表である。拡張の `parseStrictJson` に当たるのは、そのうち `invalid utf8`・`unpaired high`・`unpaired low`・`unpaired in ignored`・`trailing object`・`trailing text`・`trailing char`・`empty`・`truncated` の行である。`top-level null`・`array`・`string`・`number` の行は、`parseStrictJson` がどの値も返す設計（設計書 3.2）なので、応答の検査（`classifyStatus` など）が拒否する。重複したメンバと BOM の拒否は、Go の `ParseObject` が行わない（Go は重複を `Collect` で拒否する。`strictjson_test.go:133`）ので、拡張の側だけで確かめる。
-   **ホストの応答の上限:** `internal/nmhost/test_helpers.go:80` の `MaxResponseBytesForTest` は `maxResponseBytes`（`internal/nmhost/relay.go:43`、`512 << 10`）である。Go のガードは、テスト用の補助を加えずにこれを読める。
-   **README の拒否の理由の表:** `cmd/yt2column-server/docs_test.go:122` の `TestServerDocsContract/rejection_reasons` は、README の表の「拡張の表示」の分類が `development.md` の表と同じであることを確かめる。`test/serverReasons.test.ts` は `development.md` の表を拡張の集合と突き合わせる。2 つで、README の表も拡張の集合と合うことになる（AC-38）。
-   **テスト用のホストの登録:** `yt2column-nmhost install` と `uninstall` は、自分の manifest（`internal/nmhost/manifest.go:338` の `isOwnManifest`。ホストの名前・`stdio`・絶対パス・拡張のオリジンの `allowed_origins`）だけを上書き・削除する。試作の拡張は manifest の `key` を同じにして同じ拡張の ID にし（本物の拡張は先に外す）、本物のホストの登録に戻せなければ、テスト用のホストの manifest を手で消す（ステップ 1-1）。

**型の作り手を限ること:** `ArticleKey`・`UsdAmount` は、`eslint.config.js:149` の `brandNames` に加え、`creatorConfig`（`:198-212`）でそれぞれ `src/core/articleKey.ts` と `src/core/response.ts` にだけ作ることを許す（設計書 3.1）。`PreparedRequest` は、今の `prepareSend` に加えて `prepareStartInput` が作る。どちらも `src/core/prepare.ts` にあり、今の `creatorConfig("src/core/prepare.ts", ["PreparedRequest"])` のままで足りる。ブランドを持たない判別共用体（`JobStatus`・`ViewerState` など）は、作る経路が検査の関数だけであることを型では守らない。表示の側はそれを受け取るだけなので、ガードは加えない。

**テストの補助:** 拡張のテストの補助は `test/helpers/` に置く（今の `dom.ts`・`native.ts`・`logger.ts`・`developmentDoc.ts`・`repoFile.ts` など）。`docs/dev/conventions/test_organization.md` は Go の規則（`testutil/`・`test_helpers.go`・`//go:build test`）で、拡張には当たらない。本タスクは次のように補助を広げる。
-   新しく加えるもの: `test/helpers/chrome.ts`（`runtime`・`tabs`・ウィンドウを前面に出す依存・`BroadcastChannel`・クリップボード・ダウンロードの fake）、`test/helpers/clock.ts`（時刻を進める fake の時計）。
-   広げるもの: `test/helpers/native.ts`（新しい形のメッセージ）、`test/helpers/dom.ts`（ビューアの骨格）、`test/helpers/developmentDoc.ts`（「ジョブの API のコード」の表の見出し）。`test/serverReasons.test.ts` の非公開の `cells` は、`test/jobCodes.test.ts` と共有するために `test/helpers/repoFile.ts` に移す。

Go のガードの変更は、既存の `nmhost.MaxResponseBytesForTest` を使うので、Go のテストの補助は加えない。

**実装の方法が 2 つ以上ある箇所:** ない。設計書が型・関数・メッセージ・CSS の分け方を決めている。並べて加える方法、試作の置き場所、コミットの単位、2 つのガードの狭め方、注入する依存、ガードのテストにする確かめ、ログの関数の置き場所は 1.3 で決めた。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 確認の試作

#### ステップ 1-1: 設計書 3.16 の振る舞いを試作で確かめる

**変更するファイル:** `docs/tasks/0050_viewer_tab/03_implementation_plan.md`（4.3 の記録）、崩れた行がある場合は `docs/tasks/0050_viewer_tab/02_architecture.md`（崩れた行の扱いが要件を変える場合は `docs/tasks/0050_viewer_tab/01_requirements.md` も。行 10 で改めた）

-   [x] 作業ツリーの外に試作の拡張を作る。manifest に `background`（`type: "module"`）と `viewer.html` を持ち、権限は今の 3 つのままにする。manifest の `key` は今の拡張と同じにする（1.4 のテスト用のホストの登録）。設計書 3.16 の表の各行の API を呼ぶ最小の処理だけを持つ。
-   [x] 応答を約 19 秒遅らせるテスト用のサーバと、25 秒応答しないテスト用のホストを、作業ツリーの外に作る。確かめの後、`yt2column-nmhost install <browser>` で本物のホストの登録に戻し、戻らなければテスト用の manifest を手で消してから入れ直す。
-   [x] 利用者が、Chrome（110 以降の今の版）と Brave で、設計書 3.16 の表の 10 行を表の「確かめ方」のとおりに確かめる。結果を 4.3.1 の表に記録する（ブラウザと版、確かめた日、結果）。
-   [x] 崩れた行があれば、設計書 3.16 の「崩れた場合の方法」に切り替えることを、レビュアーとその場で決め、設計書を改める（`requirements_process.md`「Editing an approved document」）。「なし」の行（`tabs` の API、`<a download>`）が崩れたら、フェーズ 2 に進まず、レビュアーと扱いを決める。

**完了の条件:** 4.3.1 の表のすべての行に結果がある。崩れた行の扱いが、設計書と本書に記録されている。

### PR-1 作成ポイント: browser behavior checks

**対象ステップ**: 1-1

**推奨タイトル**: `docs(0050): Service Worker とビューアのブラウザの振る舞いを試作で確かめた結果を記録する`

**レビュー観点**: 4.3.1 の 10 行の結果と、確かめたブラウザと版 / 崩れた行と、設計書 3.16 の代わりの方法への切り替え（あれば設計書の改訂）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-1 が試作による実現性の確認（Phase 0 型の PoC）であるため

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/600
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: やり取りと検査

#### ステップ 2-1: 古い検査の移動、やり取りの型、ホストへのメッセージ

**変更するファイル:** `src/core/articlesResponse.ts`（新規、一時的）、`src/core/response.ts`、`src/core/exchange.ts`（新規）、`src/core/articleKey.ts`（新規）、`src/core/hostMessage.ts`（新規）、`src/core/prepare.ts`、`src/core/native.ts`、`src/core/sendMessages.ts`、`src/core/i18n/catalog.ts`、`src/ui/renderSend.ts`、`src/send.ts`、`src/launch.ts`、`src/browser/chromeDeps.ts`、`eslint.config.js`、`test/articlesResponse.test.ts`（新規、一時的）、`test/response.test.ts`、`test/native.test.ts`、`test/send.test.ts`、`test/renderSend.test.ts`、`test/sendMessages.test.ts`、`test/launch.test.ts`、`test/serverReasons.test.ts`、`test/prepare.test.ts`、`test/helpers/native.ts`、`test/lintRules.test.ts`、`test/exchange.test.ts`（新規）、`test/articleKey.test.ts`（新規）、`test/hostMessage.test.ts`（新規）

-   [x] `eslint.config.js` のブランドの型の別名の規則を、設計書 3.17 のとおりに狭める。`test/lintRules.test.ts:147` の describe に、許す例（オブジェクトの型のメンバの中の参照）と禁じる例（ブランドそのものの別名、共用体・交差型の要素）を置く。
-   [x] 1.4 の台帳のとおりに、今の `/articles` の検査を `articlesResponse.ts` に、`hasExactKeys` を `exchange.ts` に移し、古い部品の import を移す。古い経路の振る舞いは変えない。`test/serverReasons.test.ts` の突き合わせの相手は `articlesRejectStatus` にする（`development.md` の表は、ポップアップを切り替えるまで今のまま正しい）。
-   [x] `exchange.ts` に、設計書 3.1 の `NotSentCause`・`ExchangeFailure`・`ExchangeFailureLog`・`HostOutcome`・`maxResponseBytes`（`524_288`）と、3.3 の新しい `UnexpectedCheck` を置く。どのモジュールも import しない。
-   [x] `articleKey.ts` に `ArticleKey`・`maxKeyLength`・`parseArticleKey` を置き、`brandNames` に `ArticleKey` を、`creatorConfig` に `articleKey.ts` を加える（設計書 3.1）。
-   [x] `prepare.ts` に `StartInput` を加え、`ArticleBody.message` の型を `StartInput` にする。`StartInput` の UTF-8 の大きさを測る関数を 1 つ置き、`buildArticleBody` と新しい `prepareStartInput` の両方がそれを使う（設計書 3.1）。
-   [x] `hostMessage.ts` に `HostMessage` と `startMessage`・`statusMessage`・`publishMessage` を置く（設計書 3.1）。`SendDeps.sendNative` の引数の型を、並べて加える間は `HostMessage | StartInput`（古い経路の本文は `StartInput` である）にし（`src/send.ts:16`、`chromeDeps.ts` の `createSendDeps`、`test/helpers/native.ts`）、フェーズ 7 で `HostMessage` だけにする。
-   [x] テスト: `hasExactKeys` の過不足と継承したメンバ（`test/exchange.test.ts`、新規。今は呼び出し側のテストからしか通らない）、`parseArticleKey` の空・上限の境目・上限を超える値・文字列でない値、`prepareStartInput` が `prepareSend` と同じ大きさの判定をすること、3 つのメッセージの形（期待する値はリテラルで書く）。

**完了の条件:** `make ext-check` と `make test` が通る。1.2 の lcov の比べ方で、移したテストの関数の実行回数が変わらないことを確かめる。`prepareStartInput` の大きさの判定を `>=` に変えるように壊し、4 MiB ちょうどの入力のテストが失敗することを確かめる。型の別名の規則を元に戻すように壊し、許す例のテストが失敗することを確かめる。

#### ステップ 2-2: 厳格な JSON の解析

**変更するファイル:** `src/core/strictJson.ts`（新規）、`test/strictJson.test.ts`（新規）

-   [x] 設計書 3.2 の `JsonValue`・`StrictJsonError`・`parseStrictJson` と、表の 6 つの規則を作る。`StrictJsonError` の種類が `exchange.ts` の `UnexpectedCheck` に含まれることを、型の検査で確かめる形にする（設計書 3.3）。
-   [x] テスト: 表の規則ごとに、拒否する入力と受理する入力を置く。1.4 の Go の拒否の表の 9 行の入力を、同じバイト列で拒否する。重複したメンバ（入れ子の中も）と BOM は、拡張の側だけの行として置く。深さの上限の境目（16 段と 17 段）も置く。

**完了の条件:** `make ext-check` と `make test`（`TestSharedTokenRemoved`）が通る。重複したメンバの検査を外すように壊し、重複のテストが失敗することを確かめる。

### PR-2 作成ポイント: old check move, exchange types and strict JSON

**対象ステップ**: 2-1 / 2-2

**推奨タイトル**: `feat(0050): 古い応答の検査を移し、やり取りの型と厳格な JSON の解析を加える`

**レビュー観点**: 1.4 の台帳のとおりの移動で、今のポップアップの振る舞いが変わらないこと / 狭めたブランドの型の別名の規則が、禁じる例を拒否し続けること / 厳格な JSON の解析が設計書 3.2 の規則と Go の拒否の表の入力を拒否すること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-1 がブランドの型の lint の規則（型の作り手を限る守り）を狭め、2-2 が信頼できない入力の解析器を新しく作る、検査の重い 2 つのステップを持つため

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/604
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 2-3: サーバの応答の検査と、秘密情報の語のガード

**変更するファイル:** `src/core/response.ts`、`eslint.config.js`、`test/response.test.ts`、`test/secretsAbsent.test.ts`、`test/lintRules.test.ts`

-   [x] `response.ts` に、設計書 3.3 の型、受理する形の表、拒否の理由の 3 つの `Record`、`classifyStart`・`classifyStatus`・`classifyPublish`、`toStartOutcome`・`toStatusOutcome`・`toPublishOutcome` を作る。集合はどれも `Record<…, true>` の定数とともに宣言する（設計書 3.3）。
-   [x] 同じ `response.ts` に、設計書 4.4 の `startLogView`・`statusLogView`・`publishLogView` を置く（1.3）。
-   [x] `eslint.config.js` の `brandNames` に `UsdAmount` を加え、`response.ts` の `creatorConfig` が作ってよい型に加える。
-   [x] `test/secretsAbsent.test.ts` を、設計書 3.17「秘密情報の語のガード」のとおりに狭める。狭めた語が、`webhook url` などの設定の名前と、文言の文字列リテラルの外の `SLACK_WEBHOOK_URL`（文言のファイルの中の識別子を含む）を拒否し続けることを確かめるテストを加える。
-   [x] テスト（`test/response.test.ts` を新しい検査のテストにする）: AC-41 の各項目に 1 つ以上の入力、1.4 の Go の表のうち最上位がオブジェクトでない 4 行、古いホストの応答（`/articles` の `invalid_body`）が生成の開始の `invalid_body` の拒否になること（設計書 7.1）、3 つの操作の拒否の理由の集合と状態コードの組、ログの関数が記事・キーを返す値に含めないこと（目印の文字列で確かめる。設計書 7.3）。
-   [x] テスト（AC-42）: 256 KiB いっぱいの完了した記事の応答を、ホストの応答の外側（`classifyHostReply`、ステップ 2-4）を通して `classifyStatus` に渡し、記事が一致することを確かめる。本文は、Go の `encoding/json` の `Marshal` が書くエスケープ（`<`・`>`・`&` に対する `\u003c`・`\u003e`・`\u0026`、U+2028・U+2029 に対する `\u2028`・`\u2029`。`encoding/json` の文書の `Marshal` の項）を含める。ステップ 2-4 の前は、このテストを `classifyStatus` だけで書き、2-4 で外側を通す形にする。

**完了の条件:** `make ext-check` と `make test` が通る。メンバの集合の検査に任意のメンバを許すように壊し、AC-41 の「宣言していないメンバ」のテストが失敗することを確かめる。`statusLogView` が記事を返す値に入れるように壊し、ログのテストが失敗することを確かめる。秘密情報の語の表から `webhook_url` を消すように壊し、設定の名前のテストが失敗することを確かめる。

#### ステップ 2-4: ホストの応答の外側と、1 回のやり取り

**変更するファイル:** `src/core/native.ts`、`src/core/articlesResponse.ts`、`src/send.ts`、`src/core/sendMessages.ts`、`test/native.test.ts`、`test/send.test.ts`、`test/sendMessages.test.ts`、`test/articlesResponse.test.ts`、`test/response.test.ts`

-   [x] `native.ts` に `classifyHostReply` を作り、本文のバイト列と状態コードを返す。拡張自身の時間切れを応答なしにする（設計書 3.1）。
-   [x] 古い `classifyNativeReply` を、`classifyHostReply` の結果を `SendResult` に変える包みにする。包みは、本文に 1,024 バイトの上限を当て（超えれば今と同じ `body-over-limit`）、`classifyArticlesResponse` に渡す。外側の検査の種類は、同じ名前の `ArticlesUnexpectedCheck` に写す。時間切れは応答なしになるので、`ArticlesUnexpectedCheck` から `extension-timeout` を除き、`sendMessages.ts:36` の行を除く（1.3）。
-   [x] `send.ts` に `exchange`（`HostMessage` を受け取る。設計書 3.1）を作る。`sendTimeoutMs` の宣言の形は変えない。`sendArticle` は、ポップアップを切り替えるまで残し、`exchange` と同じ 1 回の送信の処理を共有する。
-   [x] テスト: `classifyHostReply` の検査の順と内容が今の `classifyNativeReply` と同じであること（`test/native.test.ts:37` の describe を移す）、512 KiB の境目、時間切れが応答なしになること。古い包みの 1,024 バイトの境目（1.4 の行き先の表）。`exchange` が 1 回だけ送り送り直さないこと、25 秒の後に届いた応答を無視すること、1.4 の行き先の表の `send.test.ts` の 4 つの確認。

**完了の条件:** `make ext-check` と `make test`（`TestNativeHostContract`）が通る。1.2 の lcov の比べ方で、移したテストの関数の実行回数が変わらないことを確かめる。`exchange` が時間切れの後にもう一度送るように壊し、送り直さないことのテストが失敗することを確かめる。

#### ステップ 2-5: 応答の上限の Go のガード

**変更するファイル:** `cmd/yt2column/extension_docs_test.go`

-   [x] `TestNativeHostContract` に、`extension/src/core/exchange.ts` の `export const maxResponseBytes = 524_288;` を `sendTimeoutMs`（`:120`）と同じ固定の形で読み、`nmhost.MaxResponseBytesForTest` と等しいことを確かめる subtest を加える（設計書 3.3）。読み取りのパスの定数は、今の `extensionSendPath`（`:29`）と同じ形で加える。

**完了の条件:** `make fmt`・`make test`・`make lint` が通る。`exchange.ts` の値を 1 だけ変えて、この subtest が失敗することを確かめる。

### PR-3 作成ポイント: response checks, host reply and Go guard

**対象ステップ**: 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0050): 新しい API の応答の検査とホストとの 1 回のやり取りを加える`

**レビュー観点**: 応答の検査が AC-41 の形を拒否し、256 KiB の記事を受理すること（AC-42） / 狭めた秘密情報の語のガードが設定の名前を拒否し続けること / 古い経路の振る舞いの変化が時間切れの扱いだけであること / 応答の上限の Go のガード

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-3 が秘密情報の語のガード（セキュリティの検査）を狭め、信頼できない応答の受理の境界を定めるため

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/606
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 純粋な判断と文言

フェーズ 3 は、フェーズ 2 の `prepareStartInput`・`parseArticleKey`（2-1）と、`response.ts` の型と集合（2-3）を使う。

#### ステップ 3-1: ビューアの URL

**変更するファイル:** `src/core/viewerUrl.ts`（新規）、`test/viewerUrl.test.ts`（新規）

-   [x] 設計書 3.5 の `ViewerSeed`・`ViewerParams`・`ViewerUrlError`・`viewerQuery`・`parseViewerUrl` を作る。表の 6 つの名前・上限・検査、作り方と読み方の規則、`+` の扱いに従う。
-   [x] テスト: 作った URL が読み方の規則を通り、同じ値に戻ること。入力は、`+`・`%`・`&`・`=`・`#`・BMP の外の文字・U+FFFD・各上限ちょうどの長さの値を含む決まった組にする。AC-43 の拒否の各項目に 1 つ以上の URL、各上限の境目、`ViewerSeed` から作った URL が字幕と概要欄の目印の文字列を含まないこと（AC-09。設計書 7.3）。

**完了の条件:** `make ext-check` が通る。重複した名前を後勝ちで受理するように壊し、AC-43 の重複のテストが失敗することを確かめる。

#### ステップ 3-2: 拡張の中のメッセージ

**変更するファイル:** `src/core/extensionMessages.ts`（新規）、`test/extensionMessages.test.ts`（新規）

-   [x] 設計書 3.4 のメッセージの型、5 つの `parse…` と `chooseViewer` を作る。`parseStartRequest` は設計書 3.4.1 の項目 3 の 4 の検査を行い、例外が起きても拒否にする。
-   [x] テスト: 各メッセージの受理と拒否（メンバの過不足、型の違い、上限）、`StartRequest` の 4 MiB の境目、対になっていないサロゲート、未来の `sentAt`、`chooseViewer` の順（送信の時刻、同じならタブの ID）。

**完了の条件:** `make ext-check` が通る。`parseStartRequest` が大きさの検査を飛ばすように壊し、4 MiB を超える求めのテストが失敗することを確かめる。

#### ステップ 3-3: 表示の状態・ポーリングの規則・投稿のボタン

**変更するファイル:** `src/core/jobView.ts`（新規）、`src/core/poll.ts`（新規）、`test/jobView.test.ts`（新規）、`test/poll.test.ts`（新規）

-   [x] `jobView.ts` に、設計書 3.6 の `ViewerState`・`StartFailure`・`JobDisplay`・`RetryingStatus`・`StoppingStatus`・`PollNotice` と、3.9 の `PublishRow`・`PublishControl`・`publishControl`・`PublishOpNotice` を作る。
-   [x] `poll.ts` に、設計書 3.7 の定数と `PollStep`・`afterStatus` を作る。3.6 の表の行のとおりに表示と続けるか止めるかを決め、`unknown` で直前の `done` の記事（直前が `kept` を持つ `lost` ならその `kept`）を `kept` に残す。
-   [x] テスト: `afterStatus` の表の各行（AC-11・AC-13〜AC-16・AC-18）、`publishControl` の 7 つの行と、2 つの行が同時に当てはまる組み合わせ（AC-28）、`publishing` の間と `lost` で押せないこと（AC-32）。

**完了の条件:** `make ext-check` が通る。`publishControl` の行の順を入れ替えるように壊し、重なりのテストが失敗することを確かめる。`afterStatus` が `posting` で止めるように壊し、AC-18 の行が失敗することを確かめる。

#### ステップ 3-4: ファイル名と動画 ID

**変更するファイル:** `src/core/fileName.ts`（新規）、`src/core/acceptedUrl.ts`、`test/fileName.test.ts`（新規）、`test/acceptedUrl.test.ts`

-   [x] `acceptedUrl.ts` に `watchVideoId` を加える（設計書 3.10）。`isAcceptedWatchUrl` と同じ解析を使う。
-   [x] `fileName.ts` に `articleFileName` を作り、設計書 3.10 の表の規則を当てる。
-   [x] テスト: `watchVideoId` が受け付ける URL の `v` を返し、受け付けない URL（`isAcceptedWatchUrl` の拒否の例）では値を返さないこと。表の置き換える文字の各群、先頭の `.`、UTF-8 で 200 バイトの境目とコードポイントの境目での切り詰め、`t` がない・空白だけのときの動画 ID、どの場合もパスの区切りと制御文字を含まず `.` で始まらず `.md` で終わること（AC-34）。

**完了の条件:** `make ext-check` が通る。切り詰めをバイトの途中で行うように壊し、境目のテストが失敗することを確かめる。

#### ステップ 3-5: 文言の新しい節と、コードの表のガード

**変更するファイル:** `src/core/i18n/catalog.ts`、`src/core/i18n/en.ts`、`src/core/i18n/ja.ts`、`src/core/sendMessages.ts`、`src/core/i18n/locale.ts`、`tsconfig.json`、`tsconfig.core.json`、`test/catalog.test.ts`、`test/sendMessages.test.ts`、`test/jobCodes.test.ts`（新規）、`test/helpers/developmentDoc.ts`、`test/helpers/repoFile.ts`、`test/serverReasons.test.ts`

-   [x] `Catalog` に、設計書 3.13 の表の節を、今の節と別の名前で加える（1.3）。キーの型を持つ節は `Record` にする。文言は設計書 3.13 の各項目（Slack に触れない、結果不明の範囲を断定しすぎない、生成の開始の `invalid_body` の最初の手順、工程の名前）に従い、用語は `docs/translation_glossary.md` に合わせる。
-   [x] `sendMessages.ts` に、3 つの操作の結果と理由の文言を選ぶ関数を加える。`switch` の網羅性で欠けを知らせる形にする。
-   [x] 経過時間・所要時間・推定の料金を整形する関数を加える（設計書 3.13。`locale.ts` の `formatCount` と同じ場所）。`tsconfig.json` と `tsconfig.core.json` の `lib` に `ES2023.Intl` を加える。
-   [x] `test/jobCodes.test.ts` を作る。`development.md`「ジョブの API のコード」の 6 つの表の値の集合が、`response.ts` の `Record<…, true>` の集合と等しいことを確かめる（設計書 7.1）。表の見出しは `test/helpers/developmentDoc.ts` に置き、表の読み取りは `test/helpers/repoFile.ts`（`readRepoFile`・`markdownSection` と、`test/serverReasons.test.ts` から移す `cells`）を使う。
-   [x] テスト: 1.4 の行き先の表の `sendMessages.test.ts` の確認（設定画面・トークン・ポートに触れない確認は、すべての節に広げる）、失敗の理由の 8 つの文言が互いに違うこと（AC-14）、英語と日本語の両方に文言があること（AC-37）、時間と料金の整形の例（`0.1234`、59 秒と 60 秒の境目）。
-   [x] テスト（AC-04 の値の部分。`test/catalog.test.ts` だけに置く）: 送信中の文言と、生成の開始のやり取りの結果・拒否の理由の文言が、Slack に触れないこと。投稿の状態・理由と、Webhook が設定されていないことの文言は Slack に触れてよいので対象にしない。走査が文言の集合に届くことの対照として、投稿の文言の少なくとも 1 つが Slack を含むことも確かめる。

**完了の条件:** `make ext-check` が通る。`ja.ts` の失敗の理由の文言を 1 つ別の理由の文言と同じにするように壊し、AC-14 のテストが失敗することを確かめる。`development.md` の工程の状態の表に 1 行を加えるように壊し（作業ツリーの中だけで、コミットしない）、`test/jobCodes.test.ts` が失敗することを確かめる。

### PR-4 作成ポイント: pure viewer decisions and messages

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0050): ビューアの URL・ポーリングの規則・投稿のボタン・ファイル名と新しい文言を加える`

**レビュー観点**: ビューアの URL と拡張の中のメッセージの読み方が補正せずに拒否すること（AC-05・AC-43） / `afterStatus` と `publishControl` が設計書の表の行どおりであること / 新しい文言が Slack に触れず、結果不明の範囲を断定しすぎないこと / `jobCodes.test.ts` が値と構造だけを比べること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-3 のポーリングの規則と投稿のボタンが表で決まる状態の移り変わり（状態機械）を持ち、3-1・3-2 が信頼できない URL とメッセージを読むため

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/608
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: SW と manifest

#### ステップ 4-1: ブラウザの API のラッパーと fake

**変更するファイル:** `src/browser/chromeDeps.ts`、`eslint.config.js`、`test/chromeDeps.test.ts`、`test/lintRules.test.ts`、`test/helpers/chrome.ts`（新規）、`test/helpers/clock.ts`（新規）、`test/fakes.test.ts`（新規）

-   [x] `chromeDeps.ts` に、設計書 3.18 の行のラッパー（`runtime.sendMessage`・`onMessage`、`tabs.create/update/remove/get/getCurrent`、ウィンドウを前面に出す操作、`BroadcastChannel`、クリップボード、ダウンロード、問い合わせの ID の乱数）を加える。Promise の拒否と例外は、どれも失敗の値に変える（設計書 4.1）。`chrome.windows` は 1.3 のとおり `chromeDeps.ts` だけが読む。
-   [x] `eslint.config.js` で、`windows` を `chromeDeps.ts` でだけ許し、`menuPathMessage` の「the extension sends only from the popup」を改める（設計書 3.17）。`test/lintRules.test.ts:382` の `windows` の行を、ほかのファイルで禁止され `chromeDeps.ts` で許される形にする。
-   [x] `test/helpers/clock.ts` に、`now()`・`schedule()`・`advance(ms)`（期限の順に、期限の来た処理だけを呼ぶ）を持つ fake の時計を作る（1.4）。
-   [x] `test/helpers/chrome.ts` に、ラッパーのインタフェースの形の fake を作る（設計書 7.1）。順序のテストのために、次のことができる形にする: `tabs.create` の完了をテストが進める、`tabs.remove` などを失敗させる、閉じたポップアップへの `sendResponse` が届かない、リスナーが `false` を返すと送り手の `sendMessage` が失敗する、`runtime.id` と `getURL` を持つ。`BroadcastChannel` の fake は、同じテストの中の複数の端点の間で非同期に届き、送った端点自身には届かず、`close` の後は届かない。
-   [x] テスト: 各ラッパーが拒否と例外を失敗の値に変えること。`test/fakes.test.ts` で、`BroadcastChannel` の fake の届き方と、時計の `advance` の順を確かめる。

**完了の条件:** `make ext-check` が通る。`tabs.remove` のラッパーが拒否をそのまま投げるように壊し、そのテストが失敗することを確かめる。`BroadcastChannel` の fake が送った端点自身にも届けるように壊し、`test/fakes.test.ts` が失敗することを確かめる。

#### ステップ 4-2: 同じキーのビューアの問い合わせ

**変更するファイル:** `src/viewerQuery.ts`（新規）、`test/viewerQuery.test.ts`（新規）

-   [x] `findViewers` を作る（設計書 3.4.2）。通信路と問い合わせの ID の乱数を依存として受け取り、`find` を送り、`viewerQueryWindowMs` の間、同じ `queryId` の答えを `parseViewerChannelMessage` で検査して集める。
-   [x] テスト（fake の時計）: 1 秒の境目、違う `queryId`・形の違う答えを無視すること、答えがない場合。

**完了の条件:** `make ext-check` が通る。`queryId` を比べないように壊し、違う問いの答えを無視するテストが失敗することを確かめる。

#### ステップ 4-3: 生成の開始の調整と SW の入口

**変更するファイル:** `src/startCoordinator.ts`（新規）、`src/background.ts`（新規）、`test/startCoordinator.test.ts`（新規）

-   [x] `startCoordinator.ts` に、設計書 3.4.1 のリスナーの契約（送り手の確かめ、`true`・`false` の返し方、`AwaitStart` の受け付けと保留）と、6.1 の流れ（待ち受けの記録、背面のタブ、生成の開始を 1 回、`find`・`take-over`・閉じる前の確認、結果の配布、35 秒で記録を捨てる）を作る。検査を通った `StartRequest` を処理する関数は 1 つにし、`startMessage` はその中でだけ呼ぶ（ステップ 4-5 のガードがその名前を確かめる）。ログは設計書 4.4 の SW の 4 つのラベルで、ログの関数を通した値だけを書く。
-   [x] `background.ts` は、`chromeDeps.ts` のリスナーを登録するラッパーを最上位で同期的に呼び、依存を同期の処理だけで組み立てる（設計書 3.4.1 の 1）。
-   [x] テスト（`StartRequest` の送り手。AC-05）: 受理される求めを先に確かめ、そこから 1 つの値だけを変えた求めを置く。設計書 7.3 の 7 つと、`sender.url` が `popup.html?x` と `popup.html#x` のもの。どれも `sendNative` と `tabs.create` を呼ばず `false` を返す。
-   [x] テスト（`AwaitStart`）: `sender.id` の違い、`sender.tab.id` がない、`?` より前が `viewer.html` でない場合に `false` を返すこと、タブと `sentAt` が合わないと `none` を答えること、別のタブの結果を受け取れないこと、35 秒の後に記録を捨てること。
-   [x] テスト（流れ）: 設計書 7.1 の SW の 4 つの順序の組み合わせ、生成の開始のどの結果でも送り直さないこと（AC-03）、ポップアップへの応答が届かなくても結果をビューアに届けること（AC-02）、背面のタブの位置（AC-06）、閉じる・閉じない・キーが違う場合と、引き継ぎで `tabs.update` とウィンドウを前面に出す操作を呼ばないこと（AC-08）。
-   [x] テスト（入力とログ）: ホストが受け取る生成の開始の `input` が、テストの値から書いたリテラルの期待する値と一致すること（AC-01）。ログに目印の文字列が現れず、ブラウザのエラーの文字列はログに書くが結果に入らないこと。

**完了の条件:** `make ext-check` が通る。`sender.tab` の確かめを外すように壊し、AC-05 の注入したスクリプトのテストが失敗することを確かめる。`sender.url` の比べ方を前方一致に変えるように壊し、`popup.html?x` のテストが失敗することを確かめる。`waiting` の確かめを外すように壊し、閉じない場合のテストが失敗することを確かめる。

#### ステップ 4-4: manifest とビルド

**変更するファイル:** `static/manifest.json`、`scripts/check-dist.ts`、`test/manifest.test.ts`、`test/checkDist.test.ts`

-   [x] `manifest.json` を設計書 3.15 のとおりにする（`background`、`minimum_chrome_version` を `110`）。
-   [x] `check-dist.ts` の `referenceViolations`（`:121`）に `background.service_worker` を加える（設計書 3.15）。
-   [x] テスト: `test/manifest.test.ts` の「Chrome 103」（`:124-126`）とキーの一覧（`:102-118`）。`test/checkDist.test.ts` に、`src/` にも `dist/` にもない SW を manifest が名指すと参照の違反になることを、`:138-143` の今のテストと同じ形で加える。

**完了の条件:** `make ext-check` と `make test`（`TestDeclaredPermissions`）が通る。`referenceViolations` から `background.service_worker` の読み取りを外すように壊し、その違反のテストが失敗することを確かめる。

#### ステップ 4-5: 入口のガードのテスト

**変更するファイル:** `test/entryPoints.test.ts`（新規）

-   [x] TypeScript の構文木を読み（`scripts/check-dist.ts:12`・`:69` と同じく `typescript` の `createSourceFile` を使う）、次のことを確かめるテストを作る（1.3）。
    -   `chromeDeps.ts` のリスナーを登録するラッパーの中で、`onMessage.addListener` を同期的に呼ぶ（`await` の後でない）こと。`onMessage.addListener` の呼び出しが、`src/` でそのラッパーの中だけにあること。
    -   そのラッパーを import するのが `background.ts` だけで、`background.ts` がそれをモジュールの最上位の文として呼び、その前に `await` がないこと（設計書 3.4.1 の 1、7.1）。
    -   `onMessageExternal` と `externally_connectable` が `src/` と `static/` にないこと（設計書 3.17）。
    -   `createSendDeps` を呼ぶ入口が `popup.ts` と `background.ts` だけであること（ステップ 5-5 で `viewer.ts` を加える）。`startMessage` を呼ぶのが、`startCoordinator.ts` の検査を通った求めを処理する関数（ステップ 4-3 で名前を決める）の中だけであること（設計書 3.17）。

**完了の条件:** `make ext-check` が通る。上の確かめのそれぞれについて、確かめる経路を壊すと、このテストが失敗することを確かめる。壊し方は、ラッパーの中の `addListener` を `await` の後に移す、`src/` のほかのファイルに `onMessage.addListener` を加える、`popup.ts` から登録のラッパーを呼ぶ、`background.ts` の登録の前に `await` を置く、`onMessageExternal` を加える、`createSendDeps` をほかのモジュールから呼ぶ、`startMessage` を別の関数から呼ぶ、の 7 つである。

### PR-5 作成ポイント: service worker coordinator and manifest

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0050): Service Worker の生成の開始の調整と manifest を加える`

**レビュー観点**: 生成の開始の求めの送り手の確かめ（AC-05）と `AwaitStart` の受け付け / 閉じる前の確認と、タブの作成・`find`・`take-over` の順序の競合 / fake の届き方と時計が順序のテストを正しく支えること / `windows` の lint の例外と入口のガードのテストの確かめの定め方

**実装モデル要件**: frontier-required

**判定理由**: ステップ 4-3 が送り手の検査（セキュリティの検査）と、タブ・メッセージの順序の競合（並行性）を持つため

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/612
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: ビューア

#### ステップ 5-1: 骨格と CSS の分け方

**変更するファイル:** `static/theme.css`（新規）、`static/style.css`、`static/viewer.css`（新規）、`static/viewer.html`（新規）、`static/popup.html`、`scripts/popup-preview.ts`、`test/style.test.ts`、`test/popupHtml.test.ts`、`test/popupPreview.test.ts`、`test/helpers/dom.ts`、`docs/tasks/0050_viewer_tab/_checks/presend_popup_styles.ts`（新規）

-   [x] 色の変数と共有の規則を `style.css` から `theme.css` に移し、ビューアの配置を `viewer.css` に置く（設計書 3.14）。送る前のポップアップの値は変えない（設計書 3.8）。
-   [x] `viewer.html` に、見出し・状態・記事・操作の各要素と、2 つの `role="status"` を置く（設計書 3.18）。`<script>` はステップ 5-5 で加える（`viewer.js` がないと `check-dist.ts` の参照の検査が失敗するため）。`popup.html` と `viewer.html` は、`theme.css` を先に読み込む。
-   [x] `scripts/popup-preview.ts` が `theme.css` も写して参照するようにし、`test/popupPreview.test.ts:149-153` の `<link>` の確かめを改める。
-   [x] `test/helpers/dom.ts` に、出荷する `viewer.html` から骨格を作る補助を加える。
-   [x] テスト: `test/style.test.ts` のコントラストの計算を `theme.css` の変数と新しい組（ビューアの失敗の色、工程の記号、ロード中の印、フォーカスの印）に広げる。ライトとダークの両方、`prefers-reduced-motion` で印と `running` の記号の `animation` を止める規則、押せる要素の幅と高さの最小値（AC-45・AC-48・AC-50・AC-51）。
-   [x] `_checks/presend_popup_styles.ts` を作る。マージベースの `style.css` と、PR の head の `theme.css`＋`style.css` を jsdom で読み、送る前のポップアップの骨格の各要素（プレビューが描く状態）に当たる宣言の値が変わらないことを確かめる（`requirements_process.md` §4 の `diff`）。PR の作成の前に一度走らせ、出力と 2 つのコミットの ID を 7 章の AC-24 の行に記録する。

**完了の条件:** `make ext-check` が通る。`theme.css` のダークの失敗の色の明るさを下げて、そのコントラストのテストが失敗することを確かめる。`_checks/presend_popup_styles.ts` が、`theme.css` の今の変数の値を 1 つ変えると差分を報告することを確かめる。

#### ステップ 5-2: 状態と本文の描画の部品

**変更するファイル:** `src/ui/icons.ts`、`src/ui/renderJob.ts`（新規）、`src/ui/articleBody.ts`（新規）、`test/renderJob.test.ts`（新規）、`test/articleBody.test.ts`（新規）

-   [x] `icons.ts` に、工程の状態の 5 つの記号とロード中の印を加える（設計書 3.14）。
-   [x] `renderJob.ts` に、工程のチェックリスト、経過時間、料金、所要時間、ロード中の表示、`requestId` の表示を描く関数を作る（設計書 3.14）。文言は `messageNodes`（`src/ui/render.ts:32`）で描く。
-   [x] `articleBody.ts` に `renderArticleBody` を作る（設計書 3.12）。
-   [x] テスト: 工程の状態ごとに記号の形と状態の語が違うこと（AC-11・AC-46・AC-48）、経過時間の要素が状態の領域の外で `aria-live` を持たないこと（AC-47）、1.4 の行き先の表の `renderSend.test.ts` の 3 つの確認、`renderArticleBody` に AC-25 の入力を与えると子の要素が 1 つもできず `textContent` が一致すること。

**完了の条件:** `make ext-check` が通る。`renderArticleBody` が記事の中の Markdown のリンクを `<a>` の要素にするように壊し、AC-25 のテストが失敗することを確かめる。

#### ステップ 5-3: ビューアの描画

**変更するファイル:** `src/ui/renderViewer.ts`（新規）、`test/renderViewer.test.ts`（新規）

-   [x] `renderViewer.ts` に、見出し・チャンネル名・元動画へのリンク・`ViewerState` ごとの状態・記事・投稿と保存とコピーの操作・操作の結果を描く関数を作る（設計書 3.6・3.9・3.12・3.14）。投稿のボタンは `publishControl` の結果で `disabled` と表示を決める。状態の領域には、表示が変わったときだけ書く。
-   [x] テスト: `ViewerState` の各種類の要素と文言、AC-26 のリンクの属性、AC-27 の見出しとチャンネル名、`PublishRow` の各行の `disabled` と表示（AC-28・AC-32）、`lost` で記事と保存・コピーを残すこと（AC-15）、DOM の順（AC-44）、2 つの状態の領域の書き分け（AC-47）、ロード中の表示と文言（AC-52）。

**完了の条件:** `make ext-check` が通る。投稿のボタンの `disabled` を `publishControl` によらず外すように壊し、AC-32 のテストが失敗することを確かめる。

### PR-6 作成ポイント: viewer skeleton, styles and renderers

**対象ステップ**: 5-1 / 5-2 / 5-3

**推奨タイトル**: `feat(0050): ビューアの骨格・CSS・描画の部品を加える`

**レビュー観点**: `theme.css` への移動で送る前のポップアップの値が変わらないこと（`_checks/presend_popup_styles.ts` の結果） / 記事を文字としてだけ表示すること（AC-25） / 状態の領域・記号・コントラスト・押せる大きさ・動きの規則（AC-44〜AC-52）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 5-2 の `renderArticleBody` が、信頼できない LLM の出力を文字としてだけ表示する境界（設計書 3.12、AC-25）であるため

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/616
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 5-4: ポーリングの部品

**変更するファイル:** `src/poller.ts`（新規）、`test/poller.test.ts`（新規）

-   [x] `createPoller` を、設計書 3.7 の契約の 1〜5 のとおりに作る（1 つまで、1 つのタイマー、世代、見えたときの取得、`afterStatus` の判断）。
-   [x] テスト（fake の時計）: 間隔、前の結果の前に次を送らないこと（AC-19）、取得の最中と止まっている間の `fetchNow` が何もしないこと、どのきっかけでもタイマーを取り消すこと（`fetchNow` を N 回繰り返しても取得が間隔ごとに 1 回のまま）、世代（引き継ぎの前に送った取得の `failed` で止まらない、投稿の前に送った取得の `not_posted` で止まらない、前の世代の結果の後に新しい世代の取得をすぐに 1 回送る）、止まった後の `bumpEpoch`、`stop`。

**完了の条件:** `make ext-check` が通る。前の世代の結果で止めるように壊し、世代のテストが失敗することを確かめる。

#### ステップ 5-5: ビューアの経路の処理と入口

**変更するファイル:** `src/viewerApp.ts`（新規）、`src/viewer.ts`（新規）、`static/viewer.html`、`test/viewerApp.test.ts`（新規）、`test/entryPoints.test.ts`

-   [ ] `viewerApp.ts` に `startViewer` を作る（設計書 3.6・6.3）。`startViewer` が受け持つのは次のとおりである: URL の検査、`AwaitStart` と 35 秒の期限、受け付けで `k` を加える URL の書き換え、ポーリングと見えるようになったときの取得、`find` への答え、`take-over` の受け入れ（同じ `queryId` に `present` を答えた場合だけ）と引き継ぎの表（設計書 3.6）、投稿（3.9・6.5）、保存（3.10）、コピー（3.11）、`applyPageLocale` とページのタイトル、ログ（設計書 4.4 のビューアのラベル）。URL の書き換えと見えるかどうかは依存として受け取る（1.3）。投稿のボタンを押したときの処理は 1 つの名前の関数にし、`publishMessage` はその中でだけ呼ぶ。
-   [ ] `viewer.ts` は、`chromeDeps.ts` と `document`・`history` から依存を作って `startViewer` に渡す。`viewer.html` に `<script type="module" src="viewer.js">` を加える。
-   [ ] `test/entryPoints.test.ts` の `createSendDeps` の入口に `viewer.ts` を加え、`publishMessage` を呼ぶのが投稿のボタンを押したときの関数の中だけであることの確かめを加える（設計書 3.17）。
-   [ ] テスト（表示と URL）: 6.3 の流れの各分岐（AC-07・AC-10・AC-17・AC-43）、注入した書き換えに渡った URL が `k` を持つこと、再読み込みの後の経過時間（AC-12）、ページのタイトル（AC-27）、`lang`（AC-49）、ロード中の表示を 2 回目以降の取得で出さないこと（AC-52）、状態の領域に表示が変わったときだけ書くこと（AC-47）、見えるようになったときに 1 回取得すること。
-   [ ] テスト（操作）: 状態の取得の失敗で生成の開始と投稿の操作を送らないこと（AC-16）、投稿の流れと拒否・未送信・結果不明で送り直さないこと（AC-29〜AC-31）、押せない行でクリックしても送らないこと（AC-32）、保存のバイト列（改行を含む記事、BOM なし。AC-33）、コピーの成功と失敗（AC-35・AC-36）。
-   [ ] テスト（引き継ぎとログ）: 引き継ぎの 2 つの行と、`present` を答えていない `take-over` を無視すること（AC-08・AC-22）、ログに目印の文字列が現れないこと。

**完了の条件:** `make ext-check` が通る。期限の後に届いた結果を表示するように壊し、AC-10 のテストが失敗することを確かめる。保存の `Blob` に BOM を足すように壊し、AC-33 のテストが失敗することを確かめる。`publishMessage` を別の関数から呼ぶように壊し、`test/entryPoints.test.ts` が失敗することを確かめる。

#### ステップ 5-6: 手動確認のプレビュー

**変更するファイル:** `scripts/popup-preview.ts`、`test/popupPreview.test.ts`

-   [ ] ビューアの各状態（`ViewerState` の 5 種類、`job` の 4 種類、`PublishRow` の 7 行）のページを書き出す（設計書 3.18）。ポップアップの新しい状態のページは、ステップ 6-2 で加える。
-   [ ] テスト: 書き出したページが状態ごとに 1 つずつあり、`theme.css` と `viewer.css` を参照すること。

**完了の条件:** `make ext-check` が通る。ビューアの状態の 1 つを書き出さないように壊し、状態ごとのページの数のテストが失敗することを確かめる。

### PR-7 作成ポイント: poller and viewer path

**対象ステップ**: 5-4 / 5-5 / 5-6

**推奨タイトル**: `feat(0050): ポーリングの部品とビューアの経路の処理を加える`

**レビュー観点**: ポーリングの世代と 1 つまでの規則（AC-19） / 結果を待つ期限と引き継ぎの表、`present` を答えた場合だけの `take-over` の受け入れ（AC-08・AC-10・AC-22） / 投稿の操作を送る場所の限定と、自動で送り直さないこと（AC-29〜AC-31） / 注入した URL の書き換えと見えるかどうかの依存

**実装モデル要件**: frontier-required

**判定理由**: ステップ 5-5 が、`take-over` の受け入れと投稿の操作を送る場所の限定（セキュリティの検査）と、世代と期限を持つ状態の移り変わりを持つため

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: ポップアップ

フェーズ 6 は、フェーズ 4 の SW と、フェーズ 5 の `renderJob.ts`（5-2）とプレビュー（5-6）を使う。

#### ステップ 6-1: ポップアップの画面の内容

**変更するファイル:** `src/core/popupView.ts`、`test/popupView.test.ts`

-   [ ] 設計書 3.8 の `JobPopupView`・`PopupJobState`・`PrimaryAction`・`primaryAction` を加える。
-   [ ] テスト: `primaryAction` の表の各行（AC-20・AC-22・AC-23）。

**完了の条件:** `make ext-check` が通る。`primaryAction` が `loading` で押せない操作を返すように壊し、その行のテストが失敗することを確かめる。

#### ステップ 6-2: ポップアップの描画

**変更するファイル:** `src/ui/render.ts`、`test/render.test.ts`、`scripts/popup-preview.ts`、`test/popupPreview.test.ts`

-   [ ] `renderPopup` に `job` の種類を加える。`#checks` の位置に `renderJob.ts` で状態を描き、状態の変化の要約を `#status` に書き、主ボタンの名前と押せるかを `primaryAction` で決める（設計書 3.8・3.14 のポップアップの行）。`job` の種類の主ボタンの名前は新しい節の名前を使う。送る前の画面の主ボタンの名前（`primaryButton`）は、ステップ 6-3 まで変えない（1.4）。
-   [ ] プレビューに、ポップアップの新しい状態のページを加える。
-   [ ] テスト: `PopupJobState` の各種類で、動画のセクションと詳細が `collected` と同じであること、`#checks` と `#status` の中身、主ボタンの名前と `disabled`（AC-20・AC-23・AC-46）、状態ごとの文言（設計書 3.8 の表）、DOM の順（AC-44）。

**完了の条件:** `make ext-check` が通る。`job` の種類で動画のセクションを描かないように壊し、動画のセクションが残ることのテストが失敗することを確かめる。

### PR-8 作成ポイント: popup job view and renderer

**対象ステップ**: 6-1 / 6-2

**推奨タイトル**: `feat(0050): ポップアップの生成中・生成済み・失敗の状態の画面を加える`

**レビュー観点**: `primaryAction` の表の各行（AC-20・AC-22・AC-23） / `job` の種類で動画のセクションと詳細が変わらないこと / 送る前の画面の主ボタンの名前を変えていないこと（6-3 まで）

**実装モデル要件**: standard

**判定理由**: 画面の内容と描画は設計書 3.8 が決めており、まだ到達しない種類を加えるだけで、状態機械・並行性・セキュリティの検査に当たるステップを含まないため

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 6-3: ポップアップの送信の経路

**変更するファイル:** `src/launch.ts`、`src/popup.ts`、`src/ui/render.ts`、`test/launch.test.ts`、`test/render.test.ts`

-   [ ] `runPopupSend` を、設計書 3.8 の「主ボタンを押した後」の流れ（`StartRequest` を SW に送る、`StartReply` の検査と 35 秒の期限、ポーリング）と、「ビューアを開く」（6.6）・「やり直す」に改める。ビューアの種（`ViewerSeed`）は、ポップアップが表示した値から作り、対になっていないサロゲートを、表示用に作る写しの中で U+FFFD に置き換える（設計書 3.5）。ログは設計書 4.4 のポップアップのラベルで、ログの関数を通した値だけを書く。
-   [ ] 1.4 の表の 6-3 の古い部品（`logSend`・`sendLabel`、古い送信の処理と import、`sending` の使う場所）と、それを確かめる `test/launch.test.ts` の確認をなくす。
-   [ ] 送る前の画面の主ボタンの名前を、`primaryButton` から新しい節の名前に替える（`src/ui/render.ts:305` と、1.4 の表の `test/render.test.ts`・`test/launch.test.ts` の使う場所）。
-   [ ] `popup.ts` に新しい依存を渡す。
-   [ ] テスト: 主ボタンで `StartRequest` を 1 回だけ送り、ポップアップ自身はホストに生成の開始を送らないこと（AC-01）、`sending` から各状態への移り変わり（AC-20・AC-23）、「ビューアを開く」の 3 つの分岐と生成の開始を送らないこと（AC-21。1 つ目の分岐は、通信路の fake が別のタブを答える形で確かめる）、「やり直す」が同じ入力を 1 回だけ送ること（AC-22）、開き直したポップアップが押すまで `sendNative`・`runtime.sendMessage`・状態の取得のどれも送らないこと（AC-24）、1.4 の行き先の表の `popup send` と `send log` の確認、ログに目印の文字列が現れないこと。

**完了の条件:** `make ext-check` が通る。1.2 の lcov の比べ方で、消した確認の関数の実行回数を確かめる。「ビューアを開く」が `tabs.get` で見つかったタブを使わずに `findViewers` に進むように壊し、AC-21 の 1 つ目の分岐のテストが失敗することを確かめる。

### フェーズ 7: 古いコードの削除と文書

ステップ 7-1〜7-4 は 1 つのコミットにする（1.3）。完了の条件は、ステップ 7-4 の後にまとめて確かめる。

#### ステップ 7-1: 古い部品をなくす

**変更するファイル:** 1.4 の「古い部品と、それを使う場所」の表のうち 7-1 の行と、台帳の「フェーズ 7 の後」の列が「なし」の行の各ファイル（`src/core/articlesResponse.ts`・`src/core/native.ts`・`src/send.ts`・`src/ui/renderSend.ts`・`src/core/sendMessages.ts`・`src/core/i18n/catalog.ts`・`en.ts`・`ja.ts`・`src/browser/chromeDeps.ts`・`test/helpers/native.ts`・`test/articlesResponse.test.ts`・`test/native.test.ts`・`test/send.test.ts`・`test/renderSend.test.ts`・`test/sendMessages.test.ts`・`test/catalog.test.ts`・`test/typecheck.test.ts`）

-   [ ] `src/core/articlesResponse.ts` と `test/articlesResponse.test.ts` をなくす。
-   [ ] `native.ts` の古い包みの `classifyNativeReply` と、その 1,024 バイトの境目のテストをなくす。
-   [ ] `send.ts` の `sendArticle` をなくし、`SendDeps.sendNative` の引数の型を `HostMessage` だけにする。`test/send.test.ts:38` の describe を `exchange` のテストに置き換える。
-   [ ] `src/ui/renderSend.ts` と `test/renderSend.test.ts` をなくす。
-   [ ] `sendMessages.ts` の `serverRejectMessage`・`sendOutcomeMessage` と、`test/sendMessages.test.ts:60`・`:84` の describe をなくす（確認の行き先は 1.4）。
-   [ ] 文言の古い節（`serverReject`・`notSent`・`accepted`・`noResponse`・`hostFailed`・`unexpected`・`sending`・`primaryButton`）と、`en.ts`・`ja.ts` の値をなくす。`test/typecheck.test.ts:160` の欠かす項目を、新しい節の項目に替える。
-   [ ] Slack の方針を書いたコメント（`catalog.ts:29-32`、`sendMessages.ts:25`）を改める。
-   [ ] 1.2 の lcov の比べ方で、消した関数のほかに実行回数が 0 になった関数がないことを確かめる。

#### ステップ 7-2: ガードの更新

**変更するファイル:** `test/serverReasons.test.ts`、`test/metadataDocs.test.ts`、`test/typecheck.test.ts`、`test/manifest.test.ts`

-   [ ] `test/serverReasons.test.ts` の突き合わせの相手を、新しい `ServerRejectReason` の集合と、理由から状態コードへの `Record` に改める（設計書 3.17、AC-38）。
-   [ ] `test/metadataDocs.test.ts` の主ボタンの名前の確認を、3 つの名前を確かめる形にする。「socket directory documentation reference」の確認（`:64-78`）は、付け替えずに消す（設計書 3.17）。消す確認が守っていた `README.en.md` の存在は、`cmd/yt2column::TestReadmeVersionsShareStructure` と `cmd/yt2column-server::TestStartupFailures` が引き続き確かめるので、カバレッジは狭まらない。このことを、2 つのテストの名前とともにコミットのメッセージに書く。
-   [ ] `test/typecheck.test.ts` に、`SendDeps.sendNative` に今の形の本文（`StartInput`）を渡すコードが型の検査を通らない例と、`HostMessage` を渡すコードが通る対照を、`:155` の今の形と同じく加える（AC-01）。
-   [ ] `test/manifest.test.ts` に、両方の README が書く最小の Chrome の版が `minimum_chrome_version` と等しいことを確かめるテストを加える（ステップ 7-3 で README に書く値）。

#### ステップ 7-3: README

**変更するファイル:** `README.md`、`README.en.md`

-   [ ] `README.md` の「ブラウザ拡張」節・「localhost サーバ」節・「拒否の理由」の表を、要件書 F-010 と設計書 3.18 の行（Chrome 110、拡張の更新、ビューアの見出しは URL の値であること、キーを受け取る前にビューアとポップアップを閉じた場合の手順、拡張とサーバとホストを同じ版にそろえること）のとおりに書き直す。今の経路の Slack の案内を取り除く。「必要なもの」の「Chrome または Brave」（`README.md:78`）に版を加える。
-   [ ] `/mktrans README.md` で `README.en.md` に反映する。

#### ステップ 7-4: 開発の手順・セキュリティ・概要・用語集

**変更するファイル:** `docs/dev/guides/development.md`、`docs/dev/security.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`CLAUDE.md`、`docs/translation_glossary.md`

-   [ ] `development.md` の「ジョブの API」の今の拡張の記述と、拒否の理由の表の「拡張の表示」の列を改める（要件書 F-009・F-010）。
-   [ ] `security.md` §8.1・§8.2 を、要件書 F-010・5 章と設計書 5.1 の残余の列（履歴に残る値など）のとおりに改める。
-   [ ] `project_overview.md` のツールの説明と `extension/` の説明、`production_profile.md:62-65` のポップアップと失敗の通知の記述（1.3）、`CLAUDE.md` の「Architecture Overview」の主経路の説明を改める。
-   [ ] `translation_glossary.md` に要件書 F-010 の用語を加え、「結果不明」「主ボタン」の定義と、「工程」の項（設計書 3.13 の 3 つの名前）を改める。
-   [ ] 文書に書く数や名前（ポーリングの間隔、期限、上限など）は、すでにコードの値と突き合わせるガードがある値か、ステップ 7-2 でガードを加えた値に限る。ガードのない値を書くときは、その値のガードをステップ 7-2 に加える。

**完了の条件（ステップ 7-1〜7-4）:** `make ext-check`・`make readme-sync-check`・`make test`（`doc-guard` が走らせる Go のガードと、`cmd/yt2column-nmhost::TestReadmeHostExitCodes`）・`make lint` が通る。`test/serverReasons.test.ts` の新しい集合から 1 つの理由を消すように壊し、このテストが失敗することを確かめる。README の最小の Chrome の版を 1 つ変えるように壊し、ステップ 7-2 の版のテストが失敗することを確かめる。

#### ステップ 7-5: 統合の手動の確認

**変更するファイル:** `docs/tasks/0050_viewer_tab/03_implementation_plan.md`（4.3.2 の記録）

-   [ ] ビルドした拡張をサーバ・ホストとともに動かし、設計書 7.2 の「完成の後」の項目を確かめる。結果を 4.3.2 の表に記録する。サーバは LLM の API を呼び、Webhook が設定されていれば投稿もするので、実行の前に利用者の承認を得る（`CLAUDE.md`「Tool Execution Safety」）。

**完了の条件:** 4.3.2 の表のすべての行に結果がある。

### PR-9 作成ポイント: popup switch-over, old path removal and documents

**対象ステップ**: 6-3 / 7-1 / 7-2 / 7-3 / 7-4 / 7-5

**推奨タイトル**: `feat(0050): ポップアップの送信を新しい API に切り替え、古い経路を取り除き、文書を改める`

**レビュー観点**: ポップアップの新しい経路と「ビューアを開く」「やり直す」（AC-20〜AC-24） / 古い部品の削除と 1.4 の行き先の表の確認が残っていること / README・`development.md`・`security.md`・用語集が新しい振る舞いと合うこと（AC-39） / 4.3.2 の統合の確認の記録

**実装モデル要件**: frontier-required

**判定理由**: ステップ 6-3 と 7-1 が、送信の経路の切り替えと古い経路の削除を同時に行う移行（migration）であるため

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

| マイルストーン | フェーズ | 成果物 |
|---|---|---|
| M1: ブラウザの前提の確定 | 1 | 4.3.1 の表の記録 |
| M2: やり取りと検査 | 2 | 古い検査の移動・`exchange`・厳格な JSON・3 つの操作の応答の検査・Go のガード |
| M3: 純粋な判断と文言 | 3 | ビューアの URL・メッセージ・ポーリングの規則・投稿のボタン・ファイル名・新しい文言の節 |
| M4: SW | 4 | `background.ts`・`startCoordinator.ts`・manifest・lint の例外・入口のガード |
| M5: ビューア | 5 | `viewer.html`・CSS・描画・ポーリングの部品・`viewerApp.ts` |
| M6: 切り替えと文書 | 6・7 | ポップアップの新しい経路、古い部品の削除、文書、統合の確認 |

フェーズは設計書 8 章の順に行う。各フェーズが前のフェーズの何を使うかは、そのフェーズの冒頭に書いた。フェーズ 1 の結果で設計書 3.16 の代わりの方法に切り替えると、フェーズ 4〜6 の作りが変わるので、フェーズ 1 を最初に行う。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容（英語のラベル） | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 | browser behavior checks | frontier-required |
| PR-2 | 2-1 / 2-2 | old check move, exchange types and strict JSON | frontier-recommended |
| PR-3 | 2-3 / 2-4 / 2-5 | response checks, host reply and Go guard | frontier-required |
| PR-4 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | pure viewer decisions and messages | frontier-recommended |
| PR-5 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | service worker coordinator and manifest | frontier-required |
| PR-6 | 5-1 / 5-2 / 5-3 | viewer skeleton, styles and renderers | frontier-recommended |
| PR-7 | 5-4 / 5-5 / 5-6 | poller and viewer path | frontier-required |
| PR-8 | 6-1 / 6-2 | popup job view and renderer | standard |
| PR-9 | 6-3 / 7-1 / 7-2 / 7-3 / 7-4 / 7-5 | popup switch-over, old path removal and documents | frontier-required |

分け方の理由:

-   どの PR の後も古い経路で拡張が動くよう、新しい部品を加える PR-1〜PR-8 と、切り替える PR-9 を分けた（1.3）。PR-5〜PR-7 の間、`viewer.html` は SW から開かれうるが、生成の開始の求めを送れるのはポップアップだけで（設計書 3.4.1）、ポップアップは PR-9 まで求めを送らない。そのため、スクリプトのない `viewer.html`（PR-6）が利用者に開かれることはない。
-   試作の確認（PR-1）は文書だけの PR にし、設計書の改訂が要る場合も、コードの PR と分けて読めるようにした。PR-2 以降は PR-1 のマージの後に始める。
-   フェーズ 2 は、移動と新しい型と解析器（PR-2）と、応答の検査・ガードを狭めること・古い経路の振る舞いの変化（PR-3）に分けた。機械的な移動の差分と、意味のある変更を別々に読めるようにするためである。
-   危険度の高い部分（送り手の検査と順序の競合を持つ SW、世代と期限と引き継ぎを持つビューアの経路）は、それぞれ PR-5 と PR-7 にまとめた。PR-5 で 4-3 の後に 4-4・4-5 を置くのは、4-4 の `background.service_worker` の参照に 4-3 の `background.ts` が要り、4-5 が 4-3 で決める関数の名前を確かめるためである。
-   ポップアップの画面の内容と描画（6-1・6-2）は、まだ到達しない種類を加えるだけなので PR-8 に分け、切り替え（6-3）と古い部品の削除と文書を PR-9 にまとめた（1.3）。

## 4. テスト戦略 (Test Strategy)

### 4.1. 加える・変えるテスト

ユニットテストは、設計書 7.1・7.3 の方針で、`node --test` と jsdom と `test/helpers/` の fake で組み立てる。実際のブラウザ・ホスト・サーバは呼ばない（要件書 5 章）。時刻とタイマーは注入し、`test/helpers/clock.ts` の fake の時計で進める。

-   **新しいテスト:** `exchange`・`articleKey`・`hostMessage`・`strictJson`・`viewerUrl`・`extensionMessages`・`jobView`・`poll`・`fileName`・`fakes`・`viewerQuery`・`startCoordinator`・`entryPoints`・`renderJob`・`articleBody`・`renderViewer`・`poller`・`viewerApp`・`jobCodes`（各ステップ）。
-   **書き換えるテスト:** `response`・`native`・`send`・`prepare`・`acceptedUrl`・`sendMessages`・`catalog`・`chromeDeps`・`manifest`・`checkDist`・`lintRules`・`style`・`popupHtml`・`popupPreview`・`popupView`・`render`・`launch`・`serverReasons`・`metadataDocs`・`secretsAbsent`・`typecheck`。
-   **一時的なテスト:** `articlesResponse`（ステップ 2-1 で作り、7-1 でなくす）。
-   **消すテスト:** `renderSend`（7-1）。確認の行き先は 1.4 の表のとおり。
-   **Go のテスト:** `cmd/yt2column/extension_docs_test.go::TestNativeHostContract` に応答の上限の subtest を加える（ステップ 2-5）。
-   **網羅の目標:** 新しい `src/core/` の関数と、`startCoordinator.ts`・`poller.ts`・`viewerApp.ts`・`viewerQuery.ts` の公開の関数は、どれもテストから呼ばれる（7-1 の lcov の比べ方で、実行回数が 0 の関数がないことを確かめる）。

### 4.2. 互換性のテスト

-   **古い経路:** フェーズ 2〜5 の間、ポップアップは `/articles` の経路で動く。`test/articlesResponse.test.ts`、`test/native.test.ts` の古い包みのテスト、今の `test/launch.test.ts` の送信のテストが、その振る舞いを確かめ続ける（1.3）。
-   **古いホスト:** 古いホストが新しい形の生成の開始を `/articles` へ中継した応答を `classifyStart` に通すテスト（ステップ 2-3）。
-   **送る前のポップアップ:** `_checks/presend_popup_styles.ts`（ステップ 5-1）が CSS の値を確かめ、`test/render.test.ts` の `collected` のテストが描画を確かめる。このテストの期待する値は、ステップ 6-3 で主ボタンの名前だけを替え、ほかは変えない。

### 4.3. 手動の確認の記録

#### 4.3.1. 設計書 3.16 の確認（ステップ 1-1）

試作は作業ツリーの外に作る（1.3）。各行の確かめ方は設計書 3.16 の表のとおりである。

手順の要点:

-   試作の拡張は、manifest の `key` を今の拡張と同じにし（拡張の ID は `clfmbbcdpnjcefbdihdoahomaifbabkk`）、権限を今の 3 つのままにした。ポップアップが SW に生成の開始を送り、SW が背面にビューアのタブを開いて、ホストに新しい形の `start` を送る。結果は `BroadcastChannel` でビューアに送り、ページに表示する。そのため、SW の DevTools を開かずに確かめた。SW の停止は `chrome://serviceworker-internals` で見た。
-   応答を遅らせるテスト用のサーバは、本物のソケットのパスで待ち受け、どの要求にも 19 秒後に `202` を返す。本物のホストを通して届いた（19.0〜19.1 秒）。
-   25 秒応答しないテスト用のホストは、40 秒眠るだけのスクリプトである。ホストの manifest の `path` だけをこれに替え、確かめた後に `yt2column-nmhost install chrome` で本物のホストに戻した。macOS では Chrome と Brave が同じ manifest のファイル（`~/Library/Application Support/Google/Chrome/NativeMessagingHosts/`）を使う。
-   行 6 は、送り方と閉じ方を変えて、SW が止まった後に 1 回ずつ確かめた。`sendMessage` の直後に同期的に `window.close`、ポートで送った直後に同期的に `window.close`、`sendMessage` の 100 ms 後に `window.close`、送った後に利用者が手で閉じる、の 4 つである。
-   行 10 は、試作のポップアップから、問い合わせと `tabs.get` を同時に行って確かめた。凍結は `chrome://discards` で、破棄は `tabs.discard` で行った。

| 振る舞い（設計書 3.16 の行） | ブラウザと版 | 確かめた日 | 結果 |
|---|---|---|---|
| `tabs` の権限なしの `tabs.create`・`update`・`remove`・`get` | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 期待どおり。`create` は `active:false`・`index`・`openerTabId` のとおりに背面で開き、作業中のタブは切り替わらない。同じキーの引き継ぎで、`get`・`update`・`remove` も成功した |
| `tabs.getCurrent()` が自分のタブを返す | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 期待どおり |
| `windows.update` が権限なしで使える | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 期待どおり。別のウィンドウに移した既存のビューアのウィンドウが前面に出た |
| SW の `BroadcastChannel` | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 期待どおり。SW とビューアの間で、問い合わせ・答え・結果が届いた |
| SW の寿命（19 秒遅らせるサーバ、25 秒応答しないホスト） | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 期待どおり。どちらも止まっていた SW が送信で起き、同じ SW のまま終わった（サーバ: 20.0〜20.1 秒、ホスト: 26.0 秒） |
| 止まっていた SW が、閉じたポップアップのメッセージを受け取る | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 設計書の使い方では期待どおり。100 ms の後に閉じた場合と、利用者が手で閉じた場合は届く。送った直後に同期的に `window.close` した場合は、SW が起きても届かない（ポートで送っても同じ）。送り方は変えない（設計書 3.4.1 の項目 5） |
| `<a download>` と Blob の URL の保存 | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 期待どおり。日本語の本文が崩れずに保存された |
| 権限なしの `navigator.clipboard.writeText` | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 期待どおり |
| `Intl.NumberFormat.prototype.format` の 10 進の文字列 | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 期待どおり。`"123456789012345.6789"` は `$123,456,789,012,345.6789` になった（数に変えると `…6700`）。`en-US` と `ja` で同じ |
| 凍結・破棄されたタブが問いに答えない | Chrome 156.0.8078.74・Brave 1.98.62（Chromium 156.0.8078.74） | 2026-11-01 | 崩れた。凍結されたタブは問いに答える。破棄されたタブは答えないが、新しいタブの ID が付き、前の ID は `tabs.get` で見つからない（新しい ID なら `discarded true` で見つかる）。Brave の `brave://discards` は拡張のページを破棄しない。扱いは設計書 3.16 の結果のとおり（Comments） |

#### 4.3.2. 統合の確認（ステップ 7-5）

| 確かめること（設計書 7.2） | 関わる AC | 確かめた日 | 結果 |
|---|---|---|---|
| 主ボタンの直後にポップアップを閉じても記事が出る（SW が止まった後の最初の送信を含む） | AC-02 | - | 未実施 |
| 背面のタブで開き、作業中のタブが切り替わらない | AC-06 | - | 未実施 |
| 同じ入力の送り直しで新しいタブが閉じる。凍結した既存のタブでも閉じ、破棄した既存のタブでは残る（設計書 7.2） | AC-08 | - | 未実施 |
| 「ビューアを開く」の 3 つの分岐 | AC-21 | - | 未実施 |
| 保存したファイルを `cmp` で `--out` のファイルと比べる | AC-33 | - | 未実施 |
| コピーした文字列を貼り付けて比べる | AC-35 | - | 未実施 |
| キーボードだけの操作とフォーカスの順（ビューアとポップアップ） | AC-44 | - | 未実施 |
| VoiceOver で状態の変化が読み上げられ、経過時間が読み上げられない | AC-47 | - | 未実施 |
| 背面に 5 分以上置いたビューアを前面に出すと、すぐに状態が更新される | - | - | 未実施 |
| ライトとダーク、`prefers-reduced-motion` の見え方（プレビュー） | AC-45・AC-48・AC-51 | - | 未実施 |
| 送る前のポップアップの画面が変わっていない（プレビュー） | AC-24 | - | 未実施 |

## 5. リスク管理 (Risk Management)

### 5.1. 技術的なリスク

| リスク | 影響 | 対策 |
|---|---|---|
| 設計書 3.16 のブラウザの振る舞いが文書と違う | SW・ビューア・ポップアップの作りが変わる | フェーズ 1 を最初に行い、崩れた行は設計書 3.16 の代わりの方法に、設計書を改めてから切り替える（ステップ 1-1） |
| SW が生成の開始のやり取りの途中で止められる | 結果がビューアに届かない | Chrome 110 を最小の版にし（設計書 1.2）、3.4.3 の見積もりをフェーズ 1 の 2 つの条件で確かめる。崩れれば `runtime.connect` のポートに切り替える |
| 厳格な JSON の解析が、Go の `internal/strictjson` と違う入力を受理する | 記事の取り違え、保存の U+FFFD | Go のテストの拒否の表の入力を同じバイト列で確かめる（ステップ 2-2） |
| 並べて加える間に、新旧の型を取り違える | 古い経路が新しい検査を使う、またはその逆 | 1.4 の台帳で古い名前を `Articles…` に改め、同じ名前の型が 2 つのファイルにない状態にする（ステップ 2-1）。フェーズ 7 の後の名前の残りは 6 章の横断の確かめで見る |
| fake がブラウザと違う振る舞いをし、順序のテストが違う理由で通る | 競合の欠陥を見逃す | fake の届き方を `test/fakes.test.ts` で確かめ（ステップ 4-1）、ブラウザでの振る舞いを 4.3 の手動の確認で見る |
| 最後の PR（切り替え・削除・文書）が大きい | レビューの見落とし | 到達しない画面の追加（6-1・6-2）を PR-8 に分け、PR-9 のレビューの観点を PR の作成ポイントに分けて挙げ、6-3 のコミットと、7-1〜7-4 の 1 つのコミットを分ける |
| ポーリングの世代の競合 | 生成し直している記事や投稿中の記事のポーリングが止まる | `poller.ts` を 1 つの部品にし、fake の時計で世代の例を確かめる（ステップ 5-4） |
| 2 つのガード（秘密情報の語、ブランドの型の別名）を狭めて、守りが弱まる | 拡張が Webhook の URL を持つ変更や、ブランドの型の別名を使って検査を通さずにブランドを付ける変更に気付かない | 狭めた後も禁じる例のテストを加える（ステップ 2-1・2-3）。保存しないことは lint の規則が守る |

### 5.2. 日程のリスク

| リスク | 影響 | 対策 |
|---|---|---|
| フェーズ 1 の手動の確認は利用者の作業を待つ | PR-2 以降が始められない | 確認を PR-1 だけの小さな文書の PR にし、試作と道具を先に用意して、利用者の作業を 10 行の確認に限る（ステップ 1-1） |
| 3.16 の行が崩れ、設計書の改訂が要る | 改訂の承認を待つ | 崩れた場合の方法は設計書 3.16 で先に決めてあるので、改訂はその方法を選ぶことに限られる |
| 最後の PR のレビューに時間がかかる | マージが遅れる | 文書の変更は 7-3・7-4 にまとめ、レビューの観点を分けて示す |

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1）
-   [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2）
-   [x] PR-3 マージ済み（対象ステップ: 2-3 / 2-4 / 2-5）
-   [x] PR-4 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
-   [x] PR-5 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5）
-   [x] PR-6 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3）
-   [ ] PR-7 マージ済み（対象ステップ: 5-4 / 5-5 / 5-6）
-   [ ] PR-8 マージ済み（対象ステップ: 6-1 / 6-2）
-   [ ] PR-9 マージ済み（対象ステップ: 6-3 / 7-1 / 7-2 / 7-3 / 7-4 / 7-5）
-   [ ] 7 章の受け入れ基準の検証の各行を確かめた

**横断の確かめ（`make lint` と `make test` が見つけないもの）:**

-   [ ] 1.4 の台帳の「並べて加える間の名前」と古い部品の名前が、フェーズ 7 の後の `extension/` のコメントと、`docs/`・README・`CLAUDE.md` に残っていない（コードの参照は型の検査が知らせる）。
-   [ ] `/articles` の経路と Slack を確かめる案内を述べるコメントが、`extension/src/` に残っていない（設計書 3.13 の「Slack に触れない」）。
-   [ ] 新しい用語（ビューア・Service Worker・ビューアの URL・送信の時刻・ポーリング・やり取りの結果）と工程の名前が、`docs/translation_glossary.md`・README・文言で同じ語を使っている（PR のレビューで読む）。

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類: `test` は実行するテスト、`static` は型の検査・lint・構文木のガードのテスト、`diff` は `_checks/` のスクリプトを PR のブランチで一度走らせた結果、`manual` は PR のレビューで読むことか、4.3 の手動の確認である。

| AC | 検証 | 種類 | ステップ |
|---|---|---|---|
| AC-01 | `extension/test/startCoordinator.test.ts::start message`（ホストが受け取る生成の開始が 1 つで、`input` がリテラルの期待する値と一致する）、`extension/test/hostMessage.test.ts::host messages`、`extension/test/launch.test.ts::popup job send`（ポップアップはホストに生成の開始を送らない）、`extension/test/typecheck.test.ts::sendNative accepts only HostMessage` | test・static | 2-1・4-3・6-3・7-2 |
| AC-02 | `extension/test/startCoordinator.test.ts::popup closed`、4.3.2 | test・manual | 4-3・7-5 |
| AC-03 | `extension/test/startCoordinator.test.ts::no resend`（拒否・未送信・応答なし・ホストの失敗・想定外の応答のそれぞれ） | test | 4-3 |
| AC-04 | `extension/test/catalog.test.ts::catalogs`（送信中と生成の開始の結果の文言が Slack に触れない。投稿の文言の対照つき）、英語と日本語の文言を PR のレビューで読む | test・manual | 3-5・7-1 |
| AC-05 | `extension/test/startCoordinator.test.ts::start request sender checks`（設計書 7.3 の 7 つと `popup.html?x`・`popup.html#x`、受理される求めから 1 つの値だけを変える） | test | 4-3 |
| AC-06 | `extension/test/startCoordinator.test.ts::viewer tab`（`active:false`、YouTube のタブの隣）、`extension/test/launch.test.ts::popup job states`（押した後も状態を表示する）、4.3.2 | test・manual | 4-3・6-3・7-5 |
| AC-07 | `extension/test/viewerApp.test.ts::start result`（注入した書き換えに渡った URL が `k` を持つ）、`extension/test/viewerApp.test.ts::reopened with a key`、`extension/test/viewerUrl.test.ts::round trip` | test | 3-1・5-5 |
| AC-08 | `extension/test/startCoordinator.test.ts::reuse`（閉じる・`waiting` なし・`took-over` なし・`tabs.remove` の失敗・キーが違う、前面に出さない）、`extension/test/viewerApp.test.ts::take-over`、4.3.2 | test・manual | 4-3・5-5・7-5 |
| AC-09 | `extension/test/viewerUrl.test.ts::no transcript or description`（目印の文字列） | test | 3-1 |
| AC-10 | `extension/test/viewerApp.test.ts::start result deadline` | test | 5-5 |
| AC-11 | `extension/test/poll.test.ts::afterStatus`、`extension/test/renderJob.test.ts::step checklist` | test | 3-3・5-2 |
| AC-12 | `extension/test/viewerApp.test.ts::elapsed time`（fake の時計、再読み込みの後） | test | 5-5 |
| AC-13 | `extension/test/poll.test.ts::afterStatus`、`extension/test/renderViewer.test.ts::renderViewer states` | test | 3-3・5-3 |
| AC-14 | `extension/test/poll.test.ts::afterStatus`、`extension/test/renderViewer.test.ts::renderViewer states`、`extension/test/catalog.test.ts::catalogs`（8 つの文言が互いに違う） | test | 3-3・3-5・5-3 |
| AC-15 | `extension/test/poll.test.ts::afterStatus`（`kept`）、`extension/test/renderViewer.test.ts::renderViewer states` | test | 3-3・5-3 |
| AC-16 | `extension/test/poll.test.ts::afterStatus`、`extension/test/viewerApp.test.ts::status exchange failures` | test | 3-3・5-5 |
| AC-17 | `extension/test/viewerApp.test.ts::start failed`、`extension/test/renderViewer.test.ts::renderViewer states` | test | 5-3・5-5 |
| AC-18 | `extension/test/poll.test.ts::afterStatus`（`posting`）、`extension/test/viewerApp.test.ts::posting on load` | test | 3-3・5-5 |
| AC-19 | `extension/test/poller.test.ts::one in flight` | test | 5-4 |
| AC-20 | `extension/test/popupView.test.ts::primaryAction`、`extension/test/render.test.ts::job view`、`extension/test/launch.test.ts::popup job states` | test | 6-1・6-2・6-3 |
| AC-21 | `extension/test/launch.test.ts::open viewer`（3 つの分岐と、生成の開始を送らないこと）、`extension/test/viewerQuery.test.ts::findViewers`、4.3.2 | test・manual | 4-2・6-3・7-5 |
| AC-22 | `extension/test/launch.test.ts::retry`、`extension/test/startCoordinator.test.ts::reuse`、`extension/test/viewerApp.test.ts::take-over`（失敗か不明から `loading` に戻り、新しい送信の時刻で数える） | test | 4-3・5-5・6-3 |
| AC-23 | `extension/test/popupView.test.ts::primaryAction`、`extension/test/render.test.ts::job view`、`extension/test/launch.test.ts::popup job states` | test | 6-1・6-2・6-3 |
| AC-24 | `extension/test/launch.test.ts::popup job send`（開き直したポップアップは押すまで何も送らない）、`docs/tasks/0050_viewer_tab/_checks/presend_popup_styles.ts`（送る前のポップアップの CSS の値。2026-11-02 に PR-6 の作成の前に実行: マージベース `ed795e8a58e8165f83c9e7953431dbec5432aa2a`、head `369bcd79590db90ff4701a7f91d492a7e87d13e4`、出力 `PASS: 60400 computed declarations identical across 11 pages x 16 variants`、終了コード 0）、4.3.2 | test・diff・manual | 5-1・6-3・7-5 |
| AC-25 | `extension/test/articleBody.test.ts::renderArticleBody` | test | 5-2 |
| AC-26 | `extension/test/renderViewer.test.ts::renderViewerHeader`、`extension/test/viewerUrl.test.ts::parseViewerUrl`（受け付けない URL を拒否する） | test | 3-1・5-3 |
| AC-27 | `extension/test/renderViewer.test.ts::renderViewerHeader`、`extension/test/viewerApp.test.ts::page title` | test | 5-3・5-5 |
| AC-28 | `extension/test/jobView.test.ts::publishControl`、`extension/test/renderViewer.test.ts::publish rows` | test | 3-3・5-3 |
| AC-29 | `extension/test/viewerApp.test.ts::publish` | test | 5-5 |
| AC-30 | `extension/test/jobView.test.ts::publishControl`、`extension/test/viewerApp.test.ts::publish` | test | 3-3・5-5 |
| AC-31 | `extension/test/viewerApp.test.ts::publish failures`（送り直さない）、英語と日本語の文言を PR のレビューで読む | test・manual | 5-5・7-1 |
| AC-32 | `extension/test/jobView.test.ts::publishControl`、`extension/test/renderViewer.test.ts::publish rows`（`disabled`）、`extension/test/viewerApp.test.ts::publish`（押せない行のクリックで送らない） | test | 3-3・5-3・5-5 |
| AC-33 | `extension/test/viewerApp.test.ts::save`（バイト列、改行、BOM なし）、4.3.2 | test・manual | 5-5・7-5 |
| AC-34 | `extension/test/fileName.test.ts::articleFileName` | test | 3-4 |
| AC-35 | `extension/test/viewerApp.test.ts::copy`、4.3.2 | test・manual | 5-5・7-5 |
| AC-36 | `extension/test/viewerApp.test.ts::copy`（拒否と例外） | test | 5-5 |
| AC-37 | `make ext-typecheck`（`Catalog` の `Record` の節）、`extension/test/catalog.test.ts::catalogs`、`extension/test/typecheck.test.ts::a Catalog missing an item fails typecheck` | static・test | 3-5・7-1 |
| AC-38 | `extension/test/serverReasons.test.ts`（`development.md` の表と拡張の集合）、`cmd/yt2column-server/docs_test.go::TestServerDocsContract/rejection_reasons`（README の表と `development.md` の表） | test | 7-2・7-3・7-4 |
| AC-39 | 文書（`production_profile.md` を含む。1.3）を PR のレビューで読む | manual | 7-3・7-4 |
| AC-40 | `make ext-test`（`serverReasons.test.ts`・`metadataDocs.test.ts`・`manifest.test.ts`）、`make test`（`cmd/yt2column::TestReadmeSyncMarker`・`TestExtensionDocumentation`・`TestNativeHostContract`・`TestDeclaredPermissions`、`cmd/yt2column-server::TestServerDocsContract`） | test | 7-2・7-3・7-4 |
| AC-41 | `extension/test/response.test.ts::classifyStatus`、`extension/test/strictJson.test.ts::parseStrictJson`、`extension/test/native.test.ts::classifyHostReply`（上限を超える本文） | test | 2-2・2-3・2-4 |
| AC-42 | `extension/test/response.test.ts::classifyStatus`（256 KiB いっぱいの完了した記事を `classifyHostReply` から通す） | test | 2-3・2-4 |
| AC-43 | `extension/test/viewerUrl.test.ts::parseViewerUrl`、`extension/test/viewerApp.test.ts::invalid link`（状態の取得を送らない） | test | 3-1・5-5 |
| AC-44 | `extension/test/popupHtml.test.ts::viewer.html skeleton`（ビューアの DOM の順）、`extension/test/renderViewer.test.ts::order and controls`（押せる要素の順と、ネイティブの要素）、`extension/test/render.test.ts::job view`（ポップアップの DOM の順）、4.3.2 | test・manual | 5-3・6-2・7-5 |
| AC-45 | `extension/test/style.test.ts`（フォーカスの印のコントラスト、ライトとダーク）、4.3.2 | test・manual | 5-1・7-5 |
| AC-46 | `extension/test/renderJob.test.ts::step checklist`（状態の語）、`extension/test/renderViewer.test.ts::order and controls`、`extension/test/render.test.ts::job view` | test | 5-2・5-3・6-2 |
| AC-47 | `extension/test/renderViewer.test.ts::status regions`・`publish state announcements`、`extension/test/renderJob.test.ts::job state`（`aria-live` なし）、`extension/test/viewerApp.test.ts::announces on change`、4.3.2 | test・manual | 5-2・5-3・5-5・7-5 |
| AC-48 | `extension/test/style.test.ts`（文字と図形のコントラスト）、`extension/test/renderJob.test.ts::step checklist`（形の違い） | test | 5-1・5-2 |
| AC-49 | `extension/test/viewerApp.test.ts::page language` | test | 5-5 |
| AC-50 | `extension/test/style.test.ts`（押せる要素の幅と高さの規則） | test | 5-1 |
| AC-51 | `extension/test/style.test.ts`（`prefers-reduced-motion` の規則）、4.3.2 | test・manual | 5-1・7-5 |
| AC-52 | `extension/test/renderViewer.test.ts::renderViewer states`、`extension/test/viewerApp.test.ts::loading` | test | 5-3・5-5 |

`design_handoff.md` の H-01〜H-14 は、どれも設計書の付録 A に扱いが記録されている。`implementation_handoff.md` はない。

## 8. 成功基準 (Success Criteria)

-   **機能:** 7 章のすべての行の検証が通り、4.3 のすべての行に結果がある。
-   **品質:** グリーンゲート（`make test && make lint`）と `make ext-check` が通る。新しいテストのそれぞれについて、壊して失敗することを確かめ、コミットのメッセージに書いた（1.2）。
-   **セキュリティ:** AC-05・AC-25・AC-41・AC-43 のテスト、ログの目印のテスト、入口のガードのテスト（`test/entryPoints.test.ts`）、狭めた 2 つのガードの禁じる例のテストが通る。manifest の権限は変わらない（`test/manifest.test.ts`、`cmd/yt2column::TestDeclaredPermissions`）。
-   **文書:** 要件書 F-010 の文書と `production_profile.md` を改め、`README.en.md` が `/mktrans` で同期している（`make readme-sync-check`）。

## 9. 次のステップ (Next Steps)

-   最後の PR のマージの後、タスクの issue（#503）を閉じる（`/runpr` が案内する）。
-   #504（Markdown の描画）は、`renderArticleBody`（設計書 3.12）を差し替える。
-   #526（`/articles` とホストの今の形の中継の削除）は、拡張の変更なしに進められる（設計書 9 章）。
