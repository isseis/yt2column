# 実装計画書：ブラウザ拡張の右クリックメニューの経路の削除

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-19 |
| Review date | 2026-10-19 |
| Reviewer | isseis |
| Comments | 承認の後に `/mkplan2` で PR の境界を加えた（2026-10-19。判断は変えていない）。フェーズ 3 を 2 つの PR に分けるため、ステップ 3-1〜3-12 の順を入れ替えて番号を付け直し（旧 3-1・3-4・3-5・3-9・3-10・3-11・3-12・3-2・3-3・3-6・3-8・3-7 → 新 3-1〜3-12）、PR-3 の確認のステップ 3-13 を加えた。ほかの節の番号の参照も合わせた。PR のレビューを受けて、完了の行のないステップに、計画に既にある検査を述べ直す「対象」「完了」の行を加えた（2026-10-19。判断は変えていない） |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）が定める、ブラウザ拡張の右クリックメニューの経路（以下、メニューの経路）の削除を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに行う。具体的には次のことを行う。

-   service worker・結果のウィンドウ・`chrome.storage.session` の使用・ツールバーのバッジと、manifest の `background`・`contextMenus` を削除する（設計書 3.1・3.3）。
-   メニューの経路だけが使っていた共有コードを、設計書 3.2.1 の基準で削除し、送らない理由の表示を `renderBlockReason` に置き換える（設計書 3.2）。
-   削除する前に、メニューの経路のテストだけが確かめていた振る舞いを、ポップアップの経路のテストで確かめるようにする（設計書 3.10.2）。
-   削除した API の再導入を lint で禁止し、README・`docs/dev/security.md` と文書の検査を改める（設計書 3.5.2・3.9）。
-   削除の前後のカバレッジの比較と、Chrome・Brave の手動確認を行い、本計画に記録する（設計書 3.11・7.2）。

本計画が使う記号は次のとおりである。AC-NN・F-NNN は要件書の項番、H-NN は [design_handoff.md](design_handoff.md) の項目を指す。「§N」は本計画の節、「設計書 N」は設計書の節を指す。用語（メニューの経路・ポップアップの経路・結果のウィンドウ・入口のモジュール・本番の参照元）は設計書の冒頭と同じ意味で使う。

フェーズは設計書 8 章の実装優先順位 1〜7 と 1 対 1 に対応させる（フェーズ N = 優先順位 N）。

### 1.2. 実装原則

-   設計書 1.1 の設計原則に従う。削除だけを行い、ポップアップの経路の振る舞いと文言を変えない。削除の範囲は設計書 3.2.1 の基準に限る。依存パッケージを加えない。
-   参照する側から順に削除し、各フェーズの終わりで `make ext-check` → `make test` → `make lint` が通る状態にする（設計書 8 章）。`make test`・`make lint` は Go だけを対象にするので、拡張の検査には `make ext-check` が要る。
-   加える・改める検査は、対象を壊して失敗することを確かめ、コミットのメッセージに記す（CLAUDE.md「Every test must be able to fail for its stated reason」）。各フェーズの最後のステップがこれを行う。
-   拡張の TypeScript のコメント・識別子・文字列、Go のテストのコメント・識別子・文字列は英語で書く。文書のテストが日本語の見出しや語句と照合するリテラルだけは日本語を使う（CLAUDE.md「Go Idioms」）。
-   本タスクのすべての PR を、`0026_native_messaging_relay` の PR-2 を始める前にマージする（要件書 §5）。

### 1.3. 既存コード調査結果

本節の `file:line` は HEAD `0a06b94` で読んだ。`git diff --quiet 81cb30d 0a06b94 -- extension README.md docs/dev/security.md cmd/yt2column/extension_docs_test.go` は終了コード 0 で、本タスクが変えるファイルは設計書が引くコミット `81cb30d` と同じである。そのため、設計書の行番号もそのまま使える。

**削除するファイルと、メニューの経路の処理**

-   `extension/src/background.ts`・`extension/src/result.ts`・`extension/static/result.html` は、設計書 2.1 のとおり削除の対象である。`background.ts` だけが `chrome.contextMenus`（`:56`・`:73`・`:95`）・`chrome.windows`（`:40`）・`chrome.action`（`:27`）を参照する。`result.ts` だけが `schedule` を import する（`result.ts:6`）。
-   `extension/src/launch.ts` の削除の対象は次のとおりである。`ResultViewStore`（`:36`）・`MenuLaunchDeps`（`:54`）・`ResultWindowDeps`（`:89`）・ラベルの定数（`:96`・`:99-104`・`:106`）・`resultWaitMs`（`:111`）・`attempt`（`:124`）・`runMenuLaunch`（`:228`）・`openResultWindow`（`:304`）・`runResultWindow`（`:413`）。これらを削除すると、`sendTimeoutMs`・`parseResultView`・`ResultView`・`unreadableResultMessage`・`renderResultView` の import（`:14-16`・`:20`・`:23-27`）のうち、`renderResultView` 以外は使われなくなる。`renderResultView` は `runPopupSend`（`:377`）が使い続ける。
-   `extension/src/browser/chromeDeps.ts` の削除の対象は、`StorageChangeEvents`・`StorageChangeListener`（`:23-32`）、`WindowsApi`（`:33`）、`ActionApi`（`:40`）、`createResultViewStore`（`:206`）、`createResultWindowOpener`（`:242`）、`Badge`（`:251`）、`createBadge`（`:257`）、`schedule`（`:289`）である。`chromeDeps.ts:3` は `ResultViewStore` を `launch.ts` から型として import している。このため、`launch.ts` の `ResultViewStore` と `chromeDeps.ts` の `createResultViewStore` は、同じフェーズで削除する（フェーズ 3）。

**共有コード（設計書 3.2.1 の表）**

-   `renderResultView`（`ui/renderSend.ts:56`）を本番で呼ぶのは `runPopupSend` の `blocked` の場合（`launch.ts:377`）と、`runResultWindow`（`launch.ts:463`・`:469`）だけである。`summaryNode`（`ui/renderSend.ts:21`）は `renderResultView` の `sending`・`sent` の場合だけが使い、`.send-summary`（`static/style.css:85`）は `summaryNode` だけが使う。
-   `core/resultView.ts` は `parseSummary`・`isRejectionReason`・`hasExactKeys`（`core/summary.ts`）、`unexpectedChecks`・`isServerRejectReason`・`parseRequestId`（`core/response.ts`）、`preflightReasons`（`core/prepare.ts`）を import する（`core/resultView.ts:5-13`）。`parseSummary`・`isRejectionReason`・`unexpectedChecks` は、`core/resultView.ts` のほかに本番の参照元を持たない。`hasExactKeys` は `core/response.ts:190`・`:194` でも使われる。`core/summary.ts:2` の `rejectionReasons` の import は、`isRejectionReason`（`:83-88`）だけが使う。
-   `unreadableResultMessage`（`core/sendMessages.ts:204`）の本番の参照元は `runResultWindow` だけである。テストでは `sendMessages.test.ts:18`・`:120` と `launch.test.ts` が使う。

**テスト**

-   `extension/test/launch.test.ts` のテストの扱いを次の表に示す。「削除」はメニューの経路か結果のウィンドウだけを確かめるテスト、「改める」は両方の経路を確かめるテストで、ポップアップの経路の部分だけを残す（設計書 3.10.2）。

| describe | テスト（`it` の名前。行） | 扱い |
|---|---|---|
| launch paths | collects the same input through the menu and popup paths（`:417`） | 改める（§2.1 の名前の表） |
| launch paths | sends the same body and classification through both paths（`:442`） | 改める（同上） |
| launch paths | does not read settings or send for a rejected collection（`:499`） | 削除 |
| launch paths | reads the metadata once, from the collected tab and URL（`:515`） | 改める |
| launch paths | does not read the metadata for a ${name} collection（`:548`。4 行） | 改める |
| launch paths | sends without metadata and logs an error when ${name}（`:599`。3 行） | 改める |
| launch paths | treats a missing tab as collection-failed in both paths（`:635`） | 改める（§2.1 の名前の表） |
| launch paths | ignores info.selectionText and reads the selection itself（`:664`） | 削除 |
| launch paths | keeps untrusted text out of the fixed log labels（`:678`） | 改める |
| launch paths | clears the badge at the start of the menu path（`:688`） | 削除 |
| launch paths | signals the badge and does not throw when the key cannot be created（`:694`） | 削除 |
| launch paths | does not open a window and signals the badge when put fails（`:706`） | 削除 |
| launch paths | does not send when the result window cannot be opened（`:715`） | 削除 |
| launch paths | does not reject when the window and the removal both fail（`:727`） | 削除 |
| menu send | `:743`・`:756`・`:792`（4 行）・`:811`・`:833`・`:848`・`:860` の 7 つ | すべて削除 |
| video metadata log | records ${name} at info（`:1076`。2 行）、keeps the read values and the read error out of the logs（`:1088`） | 改める（補助 `metadataLogs` をポップアップの経路だけにする） |
| result window | `:1167`・`:1189`・`:1203`・`:1227`・`:1237`・`:1248`・`:1262`・`:1271`・`:1280`・`:1290`・`:1300` の 11 個 | すべて削除 |
| token never leaks | keeps the token out of the logs, the store and the display（`:1318`） | 改める（§2.1 の名前の表） |

-   上の表の「削除」と「改める」のほかの `launch.test.ts` のテスト（`launchContextFromTab`・`popup send`・`display` と、`launch paths` の `:652`）は変えない。ただし、`popup send` の `:922` には AC-08 の検査を加える（ステップ 2-3）。
-   `launch.test.ts` の補助のうち、`menuInfo`（`:76`）・`FakeStore`/`fakeStore`（`:196-264`）・`FakeSchedule`/`fakeSchedule`（`:266-291`）・`MenuHarness`/`MenuOptions`/`menuHarness`（`:293-344`）・`collectedSummary`（`:157`）と、`result window` の中の `sentView`・`resultDeps`（`:1139-1166`）は、メニューの経路と結果のウィンドウのテストだけが使う。
-   `extension/test/chromeDeps.test.ts` の「result view store」（`:436-486`）の 4 つのテストと、補助の `EventRecorder`/`recordingEvents`（`:405-432`）・`resultView`（`:434`）は、`createResultViewStore` だけを確かめる。`memoryStorage`・`recordingArea` は「settings store」も使うので残す。
-   `extension/test/renderSend.test.ts` の「renderResultView」（`:100-145`）の 4 つのテストのうち、`blocked` の場合（`:105`）は `renderBlockReason` のテストに改め、残りの 3 つ（`:113`・`:125`・`:136`）は削除する。`:113` は、送信中の文言を定数と独立に確かめる唯一のテストである（設計書 3.4）。このため、`:113` を削除する前に、同じ検査をポップアップの経路に加える（ステップ 2-3）。
-   `extension/test/summary.test.ts` の「parseSummary」（`:105-176`）は 9 つのテスト（`:157` は表で複数行）を持つ。主な対象は `parseSummary` だが、`:106` と `:111` は `parseSummary(summarize(o))` の往復で、`summarize` の結果のキーの集合（収集した要約が 9 つのキーを持つこと、拒否が `kind`・`reason` だけを持ち `cause` を落とすこと）も確かめている。残る「summarize」の describe（`:30-103`）は個々のフィールドしか確かめないので、`:106`・`:111` を削除する前に、同じ性質を `summarize` のテストで確かめる（ステップ 4-8）。`extension/test/sendMessages.test.ts:119` は `unreadableResultMessage` だけを確かめる。`extension/test/resultView.test.ts` は `parseResultView` だけを確かめる。
-   `extension/test/manifest.test.ts:19`・`:30`・`:87`、`extension/test/checkDist.test.ts:26-40`（`writeManifest`）・`:137-193`、`extension/test/repository.test.ts:60` は、設計書 3.3・3.10.2 のとおりに改める。
-   `extension/test/lintRules.test.ts` の「storage.sync」（`:230-259`）は、4 つの参照の形を `expectViolation` で確かめる。加える規則のテストは同じ形で書く。

**lint の規則**

-   `eslint.config.js` の `restrictedSyntax`（`:84-139`）は、`files` を指定しない設定（`:171-177`）を通してすべてのファイルに適用され、`creatorConfig` は、型の作成元である 4 つのファイルで、この設定の規則を置き換える。テストのファイルも対象である。`chrome.storage.sync` の禁止（`:21-25`・`:121-139`）は、`MemberExpression`（ドット・計算済みの文字列・テンプレート）と `Property` のキーの 4 つの形で書かれている。`contextMenus`・`session`・`windows`・`setBadgeText` という名前のメンバとキーは、HEAD では `background.ts`・`result.ts`・`launch.ts`・`chromeDeps.ts`・`chromeDeps.test.ts`・`launch.test.ts` だけにある。この範囲は `rg -n "\b(session|windows|contextMenus|setBadgeText)\b" extension/src extension/test extension/scripts` で確かめた。ほかに `core/resultView.ts:1`・`:19` のコメントにも `session` の語があるが、フェーズ 4 でファイルごと削除する。上の名前の使用箇所は、いずれもフェーズ 3・4 で削除する。`manifest.test.ts:22` の `"contextMenus"` は配列の中の文字列なので、規則の対象にならない。

**文書と文書の検査**

-   `README.md` の「Browser extension」節で改める箇所は、設計書 3.9.1 が挙げる `:573-583`・`:594`・`:615-617`・`:623-625` である。同じ節の `right-click` は、オプションページを開く手順（`:561`）にも使われている。
-   `docs/dev/security.md` §8.1 の `:102`・`:104` が、`contextMenus`・`chrome.storage.session`・結果のウィンドウに触れる。§8.1 のほかの行は触れない。
-   `cmd/yt2column/extension_docs_test.go` は、`TestExtensionSendDocumentation` が README の節に `Context menu`・`Toolbar icon` を求める（`:184`・`:185`）。`TestExtensionDocumentation` は、manifest の権限と §8.1 の一覧の一致を両方向で確かめる（`:86-115`）。手動確認の表の各欄が埋まっていることの検査（`checkManualCheckTable`、`:552`）と、`acLabel`（`:595`）が既にある。`checkManualCheckTable` は、見出し `### 5.1. 手動の確認と記録` の下の 1 列目が `AC` の 5 列の表を読む。本計画 §5.1 はこの形で書き、記録が埋まっていることをこの関数で確かめる（ステップ 7-3）。
-   拡張の node のテスト `metadataDocs.test.ts`・`serverReasons.test.ts` が README と §8.1 から読む語句は、設計書 3.9.1 の改め方では消えない。改めた後に `make ext-test` で確かめる。
-   `docs/translation_glossary.md`・`docs/dev/project_overview.md`・`docs/dev/developer_guide/package_reference.md`・`CLAUDE.md` には、メニューの経路・結果のウィンドウ・service worker への言及がない（HEAD で `rg -n -i "context menu|service worker|result window|結果のウィンドウ|メニューの経路"` を実行して確かめた）。このため、これらは改めない。

**カバレッジの取り方の確かめ**

-   本計画の作成時に、HEAD `0a06b94`（`extension/` は `81cb30d` と同じ）の `extension/` で、`node --test --experimental-test-coverage --test-reporter=lcov --test-reporter-destination=<リポジトリの外のファイル> "test/**/*.test.ts"` を node `v24.21.0`（`extension/.node-version` と同じ）で実行した。終了コードは 0 だった。報告は 20 個の `SF:` を持ち、`src/launch.ts` には `FN`・`FNDA`・`BRDA`・`DA` があった。たとえば `logSend` の error の分岐は `BRDA:217,23,0,1`・`DA:218,1` で、到達回数は 1 である。名前のない関数は `anonymous_<番号>` で報告された（`FN:390,anonymous_10` など）。設計書 3.11 の前提は、この結果と一致する。

**具体的な型による振り分け**

-   本タスクは、具体的な型で振る舞いを振り分ける設計を加えない。`ResultView` の判別のユニオンは、型ごと削除する。

### 1.4. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-05 は、設計書 3.12 がすべて記録している。本計画は、それぞれを次のステップで実施する。H-01 はフェーズ 3・4 とステップ 6-1、H-02 はステップ 3-2・3-3・3-4・3-5、H-03 はフェーズ 1 とステップ 6-3、H-04 は変更なし（設計書 3.6。コードも文書も変えない）、H-05 はステップ 7-1 で行う。
-   `implementation_handoff.md` はない。

## 2. 実装ステップ (Implementation Steps)

### 2.1. 名前を変えるものの一覧

本計画が名前を変えるものは、次の表だけである。ほかの節は、この表の新しい名前を使う。

| 種類 | 場所 | 旧 | 新 |
|---|---|---|---|
| 関数 | `extension/src/ui/renderSend.ts` | `renderResultView(container, view: ResultView)` | `renderBlockReason(container, reason: BlockReason)`（設計書 3.2.2） |
| describe | `extension/test/renderSend.test.ts` | `renderResultView` | `renderBlockReason` |
| 関数の置き場所 | `hasExactKeys` | `core/summary.ts` の export | `core/response.ts` の export しない関数（名前は変えない。設計書 3.2.5） |
| テスト | `extension/test/manifest.test.ts` | declares exactly the four permissions the design requires | declares exactly the three permissions the design requires |
| テストの補助 | `extension/test/checkDist.test.ts` | `writeManifest(root, serviceWorker, defaultPopup, optionsPage)` | `writeManifest(root, defaultPopup, optionsPage)` |
| テストの入力 | `extension/test/repository.test.ts` | `extension/dist/background.js` | `extension/dist/popup.js`（テストの名前は `ignores ${file}` から作られるので、合わせて変わる） |
| テスト | `extension/test/launch.test.ts` | collects the same input through the menu and popup paths | collects the selection and the tab title through the popup path |
| テスト | `extension/test/launch.test.ts` | sends the same body and classification through both paths | sends the video title, not the tab title, in the body |
| テスト | `extension/test/launch.test.ts` | treats a missing tab as collection-failed in both paths | treats a missing tab as collection-failed |
| テスト | `extension/test/launch.test.ts` | keeps the token out of the logs, the store and the display | keeps the token out of the logs and the display（確かめる対象の store がなくなる） |

### フェーズ 1: カバレッジの基準の取得

**対象ファイル**
-   変更: 本計画（§5.3）

**タスク**
-   [x] **ステップ 1-1**: 削除の前の lcov の報告を取る（設計書 3.11）。作業ツリーの `extension/` が `81cb30d` と同じであることを `git diff --quiet 81cb30d -- extension` の終了コード 0 で確かめる。そのうえで、§1.3「カバレッジの取り方の確かめ」のコマンドで報告を取り、リポジトリの外に保存する。報告は `81cb30d` を取り出せば作り直せるので、コミットしない。
    -   **対象:** 本計画（§5.3）。**完了:** 実行したコマンド・node の版・比べた元のコミット・報告の保存場所を §5.3 に記録した。

**完了条件:** §5.3 に基準の報告の記録がある。コードは変えない。

### フェーズ 2: ポップアップの経路のテストの補強

**対象ファイル**
-   変更: `extension/test/launch.test.ts`、本計画（§5.2）

**タスク**
-   [x] **ステップ 2-1**: §5.2 に、削除・改めるテスト（§1.3 の `launch.test.ts` の表、`renderSend.test.ts`・`summary.test.ts`・`sendMessages.test.ts`・`chromeDeps.test.ts`・`resultView.test.ts` の削除の対象）の検査を、`assert` 1 つにつき 1 件として挙げる。そして、本タスクの後に同じ値を確かめるテスト（既存のテストの名前、または本フェーズで加えるテスト）か、確かめる必要がなくなった理由（削除するコードだけの振る舞い）を対応づける（設計書 3.10.2「確かめ方」）。設計書 3.10.2 の表にない対応先が要る検査は、ステップ 2-2 に加える。
    -   **対象:** 本計画（§5.2）。**完了:** 削除・改めるすべてのテストの検査に、対応先か理由がある。
-   [x] **ステップ 2-2**: 設計書 3.10.2 の「加えるテスト」の表の 6 つの振る舞いと、ステップ 2-1 で見つかった振る舞いを、ポップアップの経路で確かめる検査を `launch.test.ts` に加える。既存のテストにポップアップの側の検査を加えてもよい。たとえば「sends the same body and classification through both paths」には、ポップアップの本文にリテラルの期待値を当てる検査を加える。「token never leaks」は、ポップアップの起動・送信・送らない理由のログ（`PopupLaunchDeps.log` と `PopupSendDeps.log` の両方）を記録して、検査の対象にする。送らない理由のログは、トークンを読み込んだうえで送らない場合（たとえば `port-invalid` で、トークンが `identifiableToken` の場合）のログを記録して確かめる。設計書 3.10.2 の表のほかに、既に次の 2 つが分かっている。ポップアップの起動のログが収集の結果（`outcome`）を持つこと（メニューの経路では `launch.test.ts:435` が確かめていた）と、送らない理由のログがラベル `yt2column blocked` の下に `reason` を持つこと（`:803-807`）である。`body-too-large` の確かめは、既存の `popupBlocks` の表（`:1001`）に行を加えて行う。この段階ではメニューの経路のコードとテストが残っている。加えた検査は、現在のコードで通ることを確かめる。
    -   **対象:** `extension/test/launch.test.ts`。**完了:** `make ext-test` が通る。
-   [x] **ステップ 2-3**: `popup send` の「shows the sending notice and a disabled button while sending」（`launch.test.ts:922`）に、送信中の表示が 2 つの文（`Do not close this window until the result appears.`・`If you close it, check Slack before sending again.`）を含むことを、`sendingMessage` を使わずに文字列の全体で確かめる検査を加える（AC-08、設計書 3.4）。
    -   **対象:** `extension/test/launch.test.ts`。**完了:** `make ext-test` が通る。
-   [x] **ステップ 2-4**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は、ステップ 2-2・2-3 で加えた検査のそれぞれである（§5.2 の対応先として加えた検査を含む）。たとえば次のとおりである。`sendingMessage` の 2 つの文の一方を変える（ステップ 2-3 の検査）。`runPopupSend` が `prepareSend` に渡す値の `title` をタブのタイトルに替える。`runPopupSend` が `prepareSend` に `result.metadata.values` ではなく `{}` を渡す（メタデータによる `body-too-large` の行）。読み取りの失敗でもメタデータを省略しない（メタデータなしの本文の検査）。`logSend` の失敗を info に記録する。`logBlocked` の振り分けを逆にする。`logBlocked` から `reason` を落とす。起動のログから `outcome` を落とす。ポップアップの起動・送信・送らない理由のログのそれぞれにトークンを入れる（3 か所を 1 つずつ）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと加えた検査が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、§5.2 がすべての検査に対応先か理由を持つ。

### PR-1 作成ポイント: popup-path test reinforcement

**対象ステップ**: 1-1 / 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `test(0027): cover menu-only behaviors through the popup path`

**レビュー観点**: §5.2 の対応の表が削除・改めるテストのすべての検査に対応先か理由を持つこと / 加えた検査がメニューの経路のコードが残る現在のコードで通り、各検査に壊して失敗することの確認があること / 「token never leaks」がポップアップの起動・送信・送らない理由の 3 つのログをトークンを読み込む場合で確かめること / 送信中の表示の 2 つの文を `sendingMessage` と独立に確かめること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-1・2-2 は、削除の安全性がかかる孤立した高リスクの作業である（削除するテストの検査ごとに、ポップアップの経路で確かめ続けるべき不変条件を漏れなく見つける判断を要する）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 入口と経路の削除

**対象ファイル**
-   削除: `extension/src/background.ts`、`extension/src/result.ts`、`extension/static/result.html`
-   変更: `extension/src/launch.ts`、`extension/src/browser/chromeDeps.ts`、`extension/static/manifest.json`、`extension/scripts/check-dist.ts`、`docs/dev/security.md`、`extension/test/launch.test.ts`、`extension/test/chromeDeps.test.ts`、`extension/test/manifest.test.ts`、`extension/test/checkDist.test.ts`、`extension/test/repository.test.ts`

フェーズ 3 は 2 つの PR に分ける。ステップ 3-1〜3-7（入口のモジュールとページ、manifest、成果物の検査）を PR-2、ステップ 3-8〜3-13（`launch.ts`・`chromeDeps.ts` のメニューの経路の処理とそのテスト）を PR-3 とする（§3.2）。PR-2 の後も、`runMenuLaunch` などは本番の参照元のない export として残り、テストが使い続けるので、PR-2 は単独で通る。

**タスク**
-   [ ] **ステップ 3-1**: 入口のモジュールとページを削除する。
    -   [ ] `extension/src/background.ts` を削除する。
    -   [ ] `extension/src/result.ts` を削除する。
    -   [ ] `extension/static/result.html` を削除する。
    -   **対象:** `extension/src/background.ts`・`extension/src/result.ts`・`extension/static/result.html`（削除）。**完了:** 3 つのファイルがない。`make ext-build` の後の `extension/dist/` にそれらの成果物がないことは、PR-2 の完了条件としてステップ 3-7 で確かめる。
-   [ ] **ステップ 3-2**: `static/manifest.json` から `background` のメンバを削除し、`permissions` を `["activeTab", "scripting", "storage"]` にする（設計書 3.3.1）。同じステップで、`docs/dev/security.md` §8.1 の「要求する権限」（`:102`）を設計書 3.9.2 の 1 つ目の項目のとおりに改める（設計書 8 章の優先順位 3）。`TestExtensionDocumentation` の権限の一致の検査（`extension_docs_test.go:86-115`）は manifest と §8.1 の一覧を両方向で比べるので、2 つを同じステップで改める。
    -   **対象:** `extension/static/manifest.json`、`docs/dev/security.md`。**完了:** ステップ 3-4 の後に、`manifest.test.ts` の「permissions」と `TestExtensionDocumentation` の権限の一致が、ステップ 3-7 の `make ext-check` → `make test` で通る。
-   [ ] **ステップ 3-3**: `scripts/check-dist.ts` を設計書 3.3.2 のとおりに改める。`referenceViolations` の `background.service_worker` の項目と、manifest の型の `background` のメンバを削除する。`static/manifest.json` と `dist/manifest.json` が両方あるときに、2 つのバイト列が同じでなければ違反を報告する検査を加える。どちらかがない場合は、この検査は何も報告しない。コメントを次のように改める。
    -   冒頭の 3 行目を、`// imports, with the manifest and HTML references resolving inside dist/.` から `// imports, with the manifest and HTML references resolving inside dist/ and` と `// dist/manifest.json a byte-for-byte copy of static/manifest.json.` の 2 行に改める。
    -   `referenceViolations` のコメントの `The manifest points at the service worker, the popup page and the options` と `page; an HTML page points at its scripts and stylesheets.` の 2 行を、`The manifest points at the popup page and the options page; an HTML page` と `points at its scripts and stylesheets.` に改める。
    -   **対象:** `extension/scripts/check-dist.ts`。**完了:** ステップ 3-5 で改める `checkDist.test.ts` を含む `make ext-check` が、ステップ 3-7 で通る。
-   [ ] **ステップ 3-4**: `manifest.test.ts` を設計書 3.3.1 のとおりに改める。「permissions」は 3 つの権限と完全に一致することを確かめ、名前を §2.1 の表のとおりに改める。「undeclared keys」の集合から `background` を除く。
    -   [ ] 「loads the service worker as a module」（`:87`）を削除する。
    -   **対象:** `extension/test/manifest.test.ts`。**完了:** §5 の AC-01・AC-04 の行の manifest のテストが、ステップ 3-7 の `make ext-check` で通る。
-   [ ] **ステップ 3-5**: `checkDist.test.ts` を設計書 3.3.2 のとおりに改める。`writeManifest` を §2.1 の表のとおりにし、`background` のメンバを書かないようにする。「accepts a manifest whose references exist」（`:137`）から `background.ts`・`background.js` を除く。manifest の複製の同一性の検査のテストを加える。`dist/manifest.json` の空白だけを変えたときに同一性の違反が 1 つだけ報告されること（バイト列で比べること）と、`static/manifest.json` だけ、または `dist/manifest.json` だけがある場合に、既存の `missing`・`not built from` の違反だけが報告され、同一性の違反が加わらないことを確かめる。
    -   [ ] `missingReferences` から `name: "service worker"` の行を削除し、残る行から `serviceWorker` のフィールドを除く。
    -   **対象:** `extension/test/checkDist.test.ts`。**完了:** 加えた同一性のテストと既存の `checkDist.test.ts` のテストが、ステップ 3-7 の `make ext-check` で通る。
-   [ ] **ステップ 3-6**: `repository.test.ts:60` の入力を §2.1 の表のとおりに改める。
    -   **対象:** `extension/test/repository.test.ts`。**完了:** `make ext-test` が、ステップ 3-7 の `make ext-check` で通る。
-   [ ] **ステップ 3-7**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は次のとおりである。`static/manifest.json` に `contextMenus` を戻す（`manifest.test.ts` の「permissions」）。`background` を戻す（「undeclared keys」）。§8.1 の一覧に `contextMenus` を戻す（`TestExtensionDocumentation` の権限の一致）。`check-dist.ts` の同一性の検査を外す。どちらかのファイルがない場合にも同一性の違反を報告させる（いずれもステップ 3-5 で加えたテストで確かめる）。AC-02 の集合の比較は、既存の `checkDist.test.ts` の「rejects an extra file」（`:87`）が確かめるので、新しく壊す対象にしない。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと名前を挙げた検査が失敗することを確かめ、コミットのメッセージに記した。`make ext-build` の後の `extension/dist/` に `background.js`・`result.js`・`result.html` がない（PR-2 の完了条件）。

### PR-2 作成ポイント: entry points, manifest and build check

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7

**推奨タイトル**: `feat(0027): remove the service worker and result window from the extension`

**レビュー観点**: manifest から `background` と `contextMenus` が消え、`security.md` §8.1 の権限の一覧と同じ PR で一致すること / `check-dist.ts` の manifest の複製の同一性の検査が、片方がない場合に何も加えず、バイト列で比べること / `background.service_worker` の参照の検査と、その `writeManifest` の引数がすべて消えていること

**実装モデル要件**: frontier-recommended

**判定理由**: mkplan の Conditional checks の 2 つに当たる（ステップ 3-3 の成果物の検査に、両方のファイルがある場合だけ働く複数の条件の検査を加えること、ステップ 3-2 で manifest の権限を、それを固定する `manifest.test.ts`・`TestExtensionDocumentation`・`security.md` §8.1 と同じ変更で改めること）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

フェーズ 3 のタスクの続き（PR-3）:

-   [ ] **ステップ 3-8**: `launch.ts` から、§1.3「削除するファイルと、メニューの経路の処理」に挙げた `launch.ts` の削除の対象を削除する。使われなくなった import を除く。`runPopupSend` の `renderResultView` の呼び出しは、フェーズ 4 まで残す。
    -   **対象:** `extension/src/launch.ts`。**完了:** `make ext-typecheck` は同じ PR の後のステップ（3-10〜3-12）の後に通る。確かめるのはステップ 3-13 のグリーンゲートである。
-   [ ] **ステップ 3-9**: `chromeDeps.ts` から、`StorageChangeEvents`・`StorageChangeListener`・`WindowsApi`・`ActionApi`・`createResultViewStore`・`createResultWindowOpener`・`Badge`・`createBadge` を削除し、`ResultViewStore` の import を除く（設計書 3.1）。`schedule` はフェーズ 4 で削除する。
    -   **対象:** `extension/src/browser/chromeDeps.ts`。**完了:** `make ext-typecheck` はステップ 3-11 の後に通る。確かめるのはステップ 3-13 のグリーンゲートである。
-   [ ] **ステップ 3-10**: `launch.test.ts` から、§1.3 の表で「削除」としたテストを削除する。
    -   [ ] does not read settings or send for a rejected collection（`:499`）
    -   [ ] ignores info.selectionText and reads the selection itself（`:664`）
    -   [ ] clears the badge at the start of the menu path（`:688`）
    -   [ ] signals the badge and does not throw when the key cannot be created（`:694`）
    -   [ ] does not open a window and signals the badge when put fails（`:706`）
    -   [ ] does not send when the result window cannot be opened（`:715`）
    -   [ ] does not reject when the window and the removal both fail（`:727`）
    -   [ ] `menu send` の describe とその 7 つのテスト（`:743`・`:756`・`:792`・`:811`・`:833`・`:848`・`:860`）
    -   [ ] `result window` の describe とその 11 個のテスト（`:1167`〜`:1300`）
    -   [ ] メニューの経路と結果のウィンドウのテストだけが使う補助（§1.3 に挙げたもの。`flush` はポップアップのテストも使うので残す）と、使われなくなった import・定数（`acceptedOutcome` など）
    -   **対象:** `extension/test/launch.test.ts`。**完了:** 上のチェックボックスがすべて済み、ステップ 3-12 の後に `make ext-check` が通る（ステップ 3-13）。
-   [ ] **ステップ 3-11**: `chromeDeps.test.ts` から次を削除する。
    -   [ ] 「result view store」の describe とその 4 つのテスト（`:437`・`:448`・`:460`・`:474`）
    -   [ ] 補助の `EventRecorder`・`recordingEvents`・`resultView` と、使われなくなった import
    -   **対象:** `extension/test/chromeDeps.test.ts`。**完了:** 上のチェックボックスがすべて済み、`make ext-check` が通る（ステップ 3-13）。
-   [ ] **ステップ 3-12**: `launch.test.ts` の、§1.3 の表で「改める」としたテストから、メニューの経路の部分を削除する。ポップアップの経路の検査は期待値を変えずに残す（設計書 3.10.2「改めるテスト」）。テストの名前は §2.1 の表のとおりに改める。
    -   [ ] collects the same input through the menu and popup paths（`:417`）
    -   [ ] sends the same body and classification through both paths（`:442`）
    -   [ ] reads the metadata once, from the collected tab and URL（`:515`）
    -   [ ] does not read the metadata for a ${name} collection（`:548`）
    -   [ ] sends without metadata and logs an error when ${name}（`:599`）
    -   [ ] treats a missing tab as collection-failed in both paths（`:635`）
    -   [ ] keeps untrusted text out of the fixed log labels（`:678`）
    -   [ ] `video metadata log` の補助 `metadataLogs` と、それを使う 2 つのテスト（`:1076`・`:1088`）
    -   [ ] keeps the token out of the logs, the store and the display（`:1318`。名前は §2.1 の表のとおりに改める）
    -   **対象:** `extension/test/launch.test.ts`。**完了:** 上のチェックボックスがすべて済み、改めたテストが §2.1 の表の名前で `make ext-check` を通る（ステップ 3-13）。
-   [ ] **ステップ 3-13**: `make ext-check` → `make test` → `make lint` を通す。ステップ 3-10〜3-12 で削除・改めたテストの検査が、§5.2 の対応先で確かめられていることを確かめる。改めたテストのポップアップの経路の検査は期待値を変えないので、新しく壊す対象は加えない（ポップアップの経路で加えた検査を壊して失敗することは、ステップ 2-4 で確かめた）。
    -   **対象:** なし（確認のみ）。**完了:** グリーンゲートが通り、§5.2 の対応先がすべて残っている。

**完了条件:** PR-2（ステップ 3-7）と PR-3（ステップ 3-13）のそれぞれで、`make ext-check` → `make test` → `make lint` が通る（PR-2 は `security.md` を変えるので `make ext-test` を含む `make ext-check` が要る）。`make ext-build` の後の `extension/dist/` に `background.js`・`result.js`・`result.html` がない。

### PR-3 作成ポイント: menu path code and tests

**対象ステップ**: 3-8 / 3-9 / 3-10 / 3-11 / 3-12 / 3-13

**推奨タイトル**: `feat(0027): remove the menu-path and result-window code`

**レビュー観点**: 削除が §1.3 の対象（`runMenuLaunch`・`runResultWindow`・`ResultViewStore`・`createResultViewStore` ほか）に限られ、ポップアップの経路の処理が変わっていないこと / 改めたテスト（ステップ 3-12。「token never leaks」のポップアップのログの検査を含む）が、ポップアップの経路の検査を名前の変更だけでなく期待値ごと残していること / 削除したテストの検査が §5.2 の対応先で確かめられていること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（削除と、ポップアップの経路の検査をそのまま残すテストの整理に限られ、対応先の判断は PR-1 で済んでいる）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 共有コードの整理

**対象ファイル**
-   削除: `extension/src/core/resultView.ts`、`extension/test/resultView.test.ts`
-   変更: `extension/src/ui/renderSend.ts`、`extension/src/launch.ts`、`extension/src/core/summary.ts`、`extension/src/core/response.ts`、`extension/src/core/sendMessages.ts`、`extension/src/core/types.ts`、`extension/src/browser/chromeDeps.ts`、`extension/static/style.css`、`extension/test/renderSend.test.ts`、`extension/test/summary.test.ts`、`extension/test/sendMessages.test.ts`

**タスク**
-   [ ] **ステップ 4-1**: `renderResultView` を `renderBlockReason` に置き換える（設計書 3.2.2）。表示は現在の `blocked` の場合と同じにする。`summaryNode` を削除し、`CollectedSummary`・`ResultView`・`renderSummary` の import を除く。`runPopupSend`（`launch.ts:377`）の呼び出しを `renderBlockReason(container, prepared.reason)` に改める。コメントを次のように改める（設計書 3.2.6）。
    -   `ui/renderSend.ts:1` の `// Renders a send outcome and a result-view value as text-only elements. Every` を `// Renders a send outcome and a block reason as text-only elements. Every` に改める（`:2-3` は変えない）。
    -   `ui/renderSend.ts:55` の `renderResultView` のコメント（`a result view`）は、`renderBlockReason` のコメントとして書き直す（設計書 3.2.2 の宣言のコメント）。
    -   `ui/renderSend.ts:11` の `/** The "sending" notice; the popup and the result window both show it. */` を `/** The "sending" notice the popup shows while a send is in flight. */` に改める。
    -   **対象:** `extension/src/ui/renderSend.ts`、`extension/src/launch.ts`。**完了:** `make ext-typecheck` は同じ PR の後のステップ（4-8）の後に通る。確かめるのはステップ 4-9 のグリーンゲートである。
-   [ ] **ステップ 4-2**: `core/resultView.ts` と `test/resultView.test.ts` を削除する。
    -   [ ] `extension/src/core/resultView.ts` を削除する。
    -   [ ] `extension/test/resultView.test.ts` を削除する。
    -   **対象:** `extension/src/core/resultView.ts`、`extension/test/resultView.test.ts`（削除）。**完了:** 2 つのファイルがない。`make ext-check` はステップ 4-9 で通す。
-   [ ] **ステップ 4-3**: `core/summary.ts` から `parseSummary`・`collectedKeys`・`rejectedKeys`・`isRejectionReason` と、`rejectionReasons`・`RejectionReason` の使われなくなった import を削除する。`hasExactKeys` を `core/response.ts` の export しない関数に移し、`core/response.ts:6` の import を除く（設計書 3.2.5）。`core/response.ts` から `unexpectedChecks` を削除する。`UnexpectedCheck` の型は残す。
    -   **対象:** `extension/src/core/summary.ts`、`extension/src/core/response.ts`。**完了:** `response.test.ts` の既存の検査を含む `make ext-test` が、ステップ 4-8 の後に通る（ステップ 4-9）。
-   [ ] **ステップ 4-4**: `core/sendMessages.ts` から `unreadableResultMessage` を削除する。
    -   **対象:** `extension/src/core/sendMessages.ts`。**完了:** `make ext-typecheck` はステップ 4-8 の後に通る（ステップ 4-9）。
-   [ ] **ステップ 4-5**: `browser/chromeDeps.ts` から `schedule` を削除する（設計書 3.2.4）。`:15` のコメント `/** The chrome.storage area surface used (session for result views, local for settings). */` を `/** The chrome.storage area surface used (local, for settings). */` に改める。`StorageArea` の型は変えない（設計書 3.2.1）。
    -   **対象:** `extension/src/browser/chromeDeps.ts`。**完了:** `make ext-check` が通る（ステップ 4-9）。
-   [ ] **ステップ 4-6**: `static/style.css` から `.send-summary` の規則（`:85-87`）を削除する。
    -   **対象:** `extension/static/style.css`。**完了:** `make ext-check` が通る（ステップ 4-9）。
-   [ ] **ステップ 4-7**: コメントを改める（設計書 3.2.6）。
    -   `core/types.ts:27` の `/** Display-only digest of an outcome; what the result window receives. */` を `/** Display-only digest of an outcome; what the popup summary shows. */` に改める。
    -   `core/response.ts:102` の `/** A send result without the cause of "unknown": what is displayed and stored. */` を `/** A send result without the cause of "unknown": what is displayed. */` に改める。
    -   `core/response.ts:123` の `/** Drops the cause of "unknown"; the only way a SendResult becomes a stored or displayed value. */` を `/** Drops the cause of "unknown"; the only way a SendResult becomes a displayed value. */` に改める。
    -   **対象:** `extension/src/core/types.ts`、`extension/src/core/response.ts`。**完了:** コメントだけの変更で、`make ext-check` が通る（ステップ 4-9）。
-   [ ] **ステップ 4-8**: テストを改める。
    -   [ ] `summary.test.ts`: 「parseSummary」を削除する前に、「summarize」の describe に、収集した要約がちょうど 9 つのキーを持つことと、`cause` を持つ拒否（すべての拒否の理由）が `kind`・`reason` だけの値になることを、値の全体で確かめる検査を加える（§1.3）。
    -   [ ] `renderSend.test.ts`: describe を §2.1 の表のとおりにし、`blocked` の行（`:105`）を `renderBlockReason` を呼ぶ形に改める。
    -   [ ] `renderSend.test.ts`: 「shows the sending notice with the close guidance and the summary」（`:113`）を削除する（検査はステップ 2-3 が引き継ぐ）。
    -   [ ] `renderSend.test.ts`: 「shows the sent outcome and its request id alongside the summary」（`:125`）を削除する。
    -   [ ] `renderSend.test.ts`: 「displays markup in the summary as text, not elements」（`:136`）を削除する。使われなくなった `collected`・`malicious`・`sendingMessage` などの import と定数を除く。
    -   [ ] `summary.test.ts`: 「parseSummary」の describe と、その 9 つのテスト（`:106`・`:111`・`:123`・`:129`・`:133`・`:141`・`:157`・`:163`・`:170`）を削除し、使われなくなった import（`parseSummary`・`rejectionReasons`・`RejectionReason`）を除く。
    -   [ ] `sendMessages.test.ts`: 「tells the user to check Slack when a result cannot be displayed」（`:119`）を削除し、`unreadableResultMessage` の import を除く。
    -   **対象:** `extension/test/summary.test.ts`、`extension/test/renderSend.test.ts`、`extension/test/sendMessages.test.ts`。**完了:** 上のチェックボックスがすべて済み、`make ext-check` が通る（ステップ 4-9）。
-   [ ] **ステップ 4-9**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は次のとおりである。`renderBlockReason` が `blockMessage` 以外の文言を描く（`renderSend.test.ts` の `renderBlockReason`）。`runPopupSend` が `renderBlockReason` を呼ばない（`launch.test.ts` の `shows the ${reason} reason and no button without sending`）。`hasExactKeys` が余分なキーを許す（`response.test.ts` の member-set の検査）。`summarize` が収集した要約にキーを 1 つ加える、拒否の `cause` を残す（ステップ 4-8 で加えた `summarize` の検査）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと名前を挙げた検査が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** `make ext-check` → `make test` → `make lint` が通る。

### PR-4 作成ポイント: shared code cleanup

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9

**推奨タイトル**: `feat(0027): replace renderResultView with renderBlockReason`

**レビュー観点**: `renderBlockReason` の表示が従来の `blocked` の場合と同じであること / `hasExactKeys` が `core/response.ts` の export しない関数に移り、応答の検査が変わらないこと / `parseSummary` を削除する前に `summarize` のキーの集合の検査が加わっていること / 設計書 3.2.1 の表で削除としたシンボルだけが削除されていること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（設計書 3.2 が置き換えの形と削除の対象を確定しており、§1.3 に競合する実装の案がない）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: lint の規則と文書

**対象ファイル**
-   変更: `extension/eslint.config.js`、`extension/test/lintRules.test.ts`、`README.md`、`docs/dev/security.md`、`cmd/yt2column/extension_docs_test.go`

**タスク**
-   [ ] **ステップ 5-1**: `eslint.config.js` の `restrictedSyntax` に、`contextMenus`・`session`・`windows`・`setBadgeText` という名前のメンバの参照とキーを禁止する規則を加える。形は `chrome.storage.sync` の禁止と同じ 4 つの参照の形にする（設計書 3.5.2）。理由のコメントと `message` は英語で書く。`ImportExpression` の項目のコメント `// A Manifest V3 service worker rejects import(); dist/ is also` と `// checked for it after the build.` を、`// check-dist.ts follows only static imports when it checks that every module`、`// dist/ loads is in dist/, so import() is not allowed; dist/ is also checked`、`// for it after the build.` の 3 行に改める（設計書 3.8）。規則の `selector`・`message` は変えない。
    -   **対象:** `extension/eslint.config.js`。**完了:** `make ext-lint` が通る（ステップ 5-6）。
-   [ ] **ステップ 5-2**: `lintRules.test.ts` に、加えた規則の describe を加える。4 つの名前のそれぞれについて、4 つの参照の形が `no-restricted-syntax` を報告することを、既存の `expectViolation` で確かめる。型の位置の参照（`chrome.windows.Window` など）が報告されないことも確かめる。
    -   **対象:** `extension/test/lintRules.test.ts`。**完了:** `make ext-test` が通る（ステップ 5-6）。
-   [ ] **ステップ 5-3**: `README.md` の「Browser extension」節を設計書 3.9.1 のとおりに改める。2 つの起動の段落をポップアップだけの起動の段落にし、`:594` の文を削除し、`:615-617` から結果のウィンドウを除き、ログの見方と更新の手順を書き改める。更新の手順は `make ext-build` の語を、ログの見方は `Inspect popup` の語を含める（ステップ 5-5 が固定する）。節の中で `context menu`・`choose the extension's item`・`result window`・`service worker` を使わない（大文字と小文字を区別しない）。
    -   **対象:** `README.md`。**完了:** ステップ 5-5 の後に、`TestExtensionSendDocumentation` を含む `make test` と、`metadataDocs.test.ts`・`serverReasons.test.ts` を含む `make ext-test` が通る（ステップ 5-6、フェーズ 5 の完了条件）。
-   [ ] **ステップ 5-4**: `docs/dev/security.md` §8.1 の `:104` を設計書 3.9.2 の 2 つ目の項目のとおりに改める（`:102` はステップ 3-2 で改めた）。
    -   **対象:** `docs/dev/security.md`。**完了:** ステップ 5-5 の後に、`TestExtensionDocumentation` を含む `make test` と、`manifest.test.ts`・`metadataDocs.test.ts` を含む `make ext-test` が通る（ステップ 5-6、フェーズ 5 の完了条件）。
-   [ ] **ステップ 5-5**: `cmd/yt2column/extension_docs_test.go` を設計書 3.9.3 のとおりに改める。
    -   `TestExtensionSendDocumentation` の「README describes sending from the browser extension」で、求める語句から `Context menu`・`Toolbar icon` を除き、`Send to yt2column` を加える。ステップ 5-3 で加える更新の手順とログの見方の案内も、`make ext-build` と `Inspect popup` を求める語句に加えて固定する。節を小文字にした文字列に、ステップ 5-3 の 4 つの語句が含まれないことを確かめる検査を加える。
    -   `TestExtensionDocumentation` に、§8.1 に `結果のウィンドウ`・`chrome.storage.session`・`contextMenus` が含まれないことと、「要求する権限」の項目が `storage` の用途として `chrome.storage.local` を挙げることを確かめるサブテストを加える。
    -   **対象:** `cmd/yt2column/extension_docs_test.go`。**完了:** `make test` が通る（ステップ 5-6）。
-   [ ] **ステップ 5-6**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は次のとおりである。`src/` の 1 つのファイルに `chrome.contextMenus`・`chrome.storage.session`・`chrome.windows`・`chrome.action.setBadgeText` をそれぞれ書く（`make ext-lint`）。加えた規則から 1 つの名前（たとえば `session`）を外し、その名前の行だけが `lintRules.test.ts` で失敗することを確かめる。テンプレートの参照の形の項目を外し、その形の行が失敗することを確かめる。README に `Context menu` の項目を戻す。`service worker` の語を戻す。`Send to yt2column`・`make ext-build`・`Inspect popup` をそれぞれ消す（`TestExtensionSendDocumentation`）。§8.1 の「要求する権限」から `chrome.storage.local` を消す（加えたサブテスト）。§8.1 に `contextMenus` の権限を戻す（`TestExtensionDocumentation` の権限の一致と、加えたサブテスト）。§8.1 に `結果のウィンドウ` の語を戻す（加えたサブテスト）。続けて `make fmt` → `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと名前を挙げた検査が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** `make ext-check` → `make test` → `make lint` が通る。`README.md` と `docs/dev/security.md` を変えたので、`make ext-test`（`metadataDocs.test.ts`・`serverReasons.test.ts`・`manifest.test.ts`）も通す（CLAUDE.md「Development Notes」）。

### PR-5 作成ポイント: lint rule and documentation

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6

**推奨タイトル**: `feat(0027): forbid removed menu APIs and document the popup`

**レビュー観点**: 加えた lint の規則が 4 つの名前を 4 つの参照の形で禁止し、型の位置の参照を禁止しないこと / README の「Browser extension」節がポップアップだけの起動・更新の手順・ログの見方を述べ、禁止する語句を含まないこと / `security.md` §8.1 から結果のウィンドウへの言及が消え、Go の文書の検査が語句を固定していること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（既存の `chrome.storage.sync` の規則と文書の検査の形を流用し、競合する実装の案がない。lint の規則は削除した API の再導入を禁止する検査だが、既存の規則と同じ形の項目を加えるだけで、新しい仕組みを持たないので security-gate の段階的な変更に当たらない）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: 整理の確認

**対象ファイル**
-   変更: 本計画（§5.1〜§5.4）。カバレッジの比較で到達しなくなったものがあれば `extension/test/launch.test.ts` ほか

**タスク**
-   [ ] **ステップ 6-1**: 設計書 3.2.7 の確認を行う。本タスクが本番の参照元を変えたモジュール（`launch.ts`・`browser/chromeDeps.ts`・`ui/renderSend.ts`・`core/summary.ts`・`core/response.ts`・`core/sendMessages.ts`・`core/prepare.ts`・`core/types.ts`・`send.ts`）の export について、本番の参照元とテストの利用を確かめる。結果が設計書 3.2.1 の表と一致することを確かめ、§5.1 の「設計書 3.2.7」の行に記録する。一致しない export があれば、設計書 3.2.1 の基準で扱いを決めてから進める。
    -   **対象:** 本計画（§5.1・§5.4）。**完了:** §5.1 の「設計書 3.2.7」の行と、§5.4 の該当の記録を記入した。
-   [ ] **ステップ 6-2**: AC-05a の静的な確認を行う（設計書 3.5.1）。`extension/src/` で次の 3 つを確かめ、§5.1 の「AC-05a」の行に記録する。`sendArticle` を呼ぶ箇所が `runPopupSend` の `click` の処理の中の 1 か所だけであること。`createSendDeps` を呼ぶ入口のモジュールが `popup.ts` だけであること。ネットワークを使う API（`fetch`・`XMLHttpRequest`・`sendBeacon`・`WebSocket`・`EventSource`）を使う箇所が `createSendDeps` の中の `fetch` の 1 か所だけであること。実行したコマンドとその出力とコミットは §5.4 に記録し、§5.1 の欄からは §5.4 を参照する（表の欄に `|` や改行を入れない）。
    -   **対象:** 本計画（§5.1・§5.4）。**完了:** §5.1 の「AC-05a」の行と、§5.4 の該当の記録を記入した。
-   [ ] **ステップ 6-3**: 削除の後の lcov の報告を、ステップ 1-1 と同じコマンドで取り、設計書 3.11 のとおりに比べる。比べるファイルは、設計書 3.11「行の対応」の 7 つと、2 つの報告の両方にあるほかのファイル（変更のないファイルは行をそのまま対応づける）である。行・分岐・関数の到達を、ソースの差分で対応づけて比べる。node の lcov は、同じソースでも実行ごとに `BRDA` の block の番号と名前のない関数の番号が変わり、到達しなかった分岐の `BRDA` を出さないことがある。このため、`DA` は対応づけた行で、`FN` は名前ではなく対応づけた開始行で、`BRDA` は対応づけた行と、その行の中の分岐の順で比べ、記録のない分岐は到達していないものとして数える。分岐だけが到達しなくなったと出た箇所は、ソースを読んで判断し、§5.3 に記録する。名前を変えた関数（`renderResultView` → `renderBlockReason`）と、移した関数（`hasExactKeys`）は、対応を明示する。削除の前に到達していて、削除の後に到達しなくなった行・分岐・関数があれば、ポップアップの経路のテストを加えて到達させ、§5.2 に加える。結果を §5.3 に記録する。
    -   **対象:** 本計画（§5.3。テストを加えた場合は §5.2）、到達しなくなったものがあれば `extension/test/launch.test.ts` ほか。**完了:** §5.3 に比較の結果を記録し、§5.1 の「AC-09」の行を記入した。
-   [ ] **ステップ 6-4**: 本タスクで Go のテストを削除していないことを確かめ、§5.1 の「AC-10」の行に記録する。本タスクの最初のコミットの親から HEAD までの Go のファイルの差分に、ファイルの削除とテスト関数の削除がないことを確かめ、実行したコマンドとその出力を §5.4 に記録する。
    -   **対象:** 本計画（§5.1・§5.4）。**完了:** §5.1 の「AC-10」の行と、§5.4 の該当の記録を記入した。
-   [ ] **ステップ 6-5**: §5.2 のすべての検査に、対応先か理由があることを確かめる。
    -   **対象:** 本計画（§5.1・§5.2）。**完了:** §5.1 の「設計書 3.10.2」の行を記入した。
-   [ ] **ステップ 6-6**: ステップ 6-3 でテストを加えた場合は、加えた検査について、対象を壊して失敗することを確かめ、コミットのメッセージに記す。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** ステップ 6-3 で加えたテスト（加えた場合）。**完了:** 加えた検査について壊して失敗することをコミットのメッセージに記し、`make ext-check` → `make test` → `make lint` が通る。

**完了条件:** §5.1 の「設計書 3.2.7」「AC-05a」「AC-09」「AC-10」「設計書 3.10.2」の行が埋まり、§5.2・§5.3 が完成している。`make ext-check` → `make test` → `make lint` が通る。

### PR-6 作成ポイント: cleanup checks and coverage comparison

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6

**推奨タイトル**: `test(0027): record the cleanup checks and coverage comparison`

**レビュー観点**: §5.3 のカバレッジの比較が、行・分岐・関数をソースの差分で対応づけ、node の lcov の番号の揺れを到達の喪失と取り違えていないこと / 到達しなくなったものに加えたテストがあれば、壊して失敗することの確認があること / §5.1 の「設計書 3.2.7」「AC-05a」「AC-09」「AC-10」「設計書 3.10.2」の欄と §5.4 のコマンドと出力が、実際の確認の結果であること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 6-3 は孤立した複雑な作業である（2 つの lcov の報告を、実行ごとに番号が変わる分岐と関数について、ソースの差分で対応づけて比べ、到達しなくなったものを判断する）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: 手動確認と記録の検査

**対象ファイル**
-   変更: 本計画（§5.1）、`cmd/yt2column/extension_docs_test.go`

**タスク**
-   [ ] **ステップ 7-1**: 設計書 7.2 の手順 1〜6 を、macOS の Chrome と Brave の、作業時点で最新の安定版で行う。結果を §5.1 の「AC-04・AC-05」と「設計書 3.7」の行に記録する。手順 1 で `81cb30d` の作業ツリーに `make ext-install` を実行する場合は、ネットワークを使うので、事前に利用者の承認を得る（CLAUDE.md「Tool Execution Safety」）。
    -   **対象:** 本計画（§5.1）。**完了:** §5.1 の「AC-04・AC-05」と「設計書 3.7」の行を記入した。
-   [ ] **ステップ 7-2**: 設計書 7.2 の手順 7 を行い、§5.1 の「AC-07」の行に記録する。サーバが LLM API を呼び、Webhook に投稿するので、事前に利用者の承認を得る。`main` のサーバが HTTP で待ち受けている間（0026 の PR-2 のマージの前）に行う。
    -   **対象:** 本計画（§5.1）。**完了:** §5.1 の「AC-07」の行を記入した。
-   [ ] **ステップ 7-3**: `cmd/yt2column/extension_docs_test.go` に、本計画の §5.1 の表の各欄が埋まっていることを確かめるテスト `TestContextMenuRemovalRecords` を加える。既存の `checkManualCheckTable` を使い、本計画のパスの定数を加える。行の 1 列目の期待値は §5.1 の表の 8 行と同じ順にし、AC の行は `acLabel` で作る（`AC-05a` は `acLabel(5) + "a"`）。`設計書 …` の行は文字列のリテラルで書く。あわせて、§5.2・§5.3・§5.4 に「（実施のときに記入する）」が残っていないことを確かめる。このテストは設計書 7 章にない、本計画が加える記録の検査で、0007・0020 の計画の手動確認の表の検査（`TestExtensionDocumentation`・`TestExtensionSendDocumentation` の「fills every cell of the plan's manual-check table」）と同じ形である。
    -   **対象:** `cmd/yt2column/extension_docs_test.go`。**完了:** `TestContextMenuRemovalRecords` を含む `make test` が通る（ステップ 7-4）。
-   [ ] **ステップ 7-4**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は、§5.1 の 1 つの欄を空にすること、1 つの行を消すこと、2 つの行を入れ替えること、§5.2〜§5.4 のいずれかに「（実施のときに記入する）」を戻すことである（`TestContextMenuRemovalRecords`）。続けて `make fmt` → `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと `TestContextMenuRemovalRecords` が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** §5.1 のすべての欄が埋まり、`TestContextMenuRemovalRecords` を含む `make test` が通る。

### PR-7 作成ポイント: manual verification and records guard

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4

**推奨タイトル**: `test(0027): record manual verification and guard the plan records`

**レビュー観点**: §5.1 の「AC-04・AC-05」「設計書 3.7」「AC-07」の欄が Chrome と Brave の実際の確認の結果で埋まっていること / 手動確認（ステップ 7-2）が 0026 の PR-2 の前に、利用者の承認を得て行われたこと / `TestContextMenuRemovalRecords` が §5.1 の行の順と §5.2〜§5.4 の記入を固定し、壊して失敗することの確認があること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない。ステップ 7-1・7-2 はネットワーク・LLM API・Webhook を使うが、利用者の承認を得て人が行う手動確認で、自動の統合テストや CI の仕組みを加えない。コードの変更は、既存の `checkManualCheckTable` を使う文書の検査 1 つだけである。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の条件 |
|---|---|---|
| M1: 基準 | フェーズ 1 | §5.3 に基準の報告の記録がある |
| M2: テストの補強 | フェーズ 2 | 加えた検査が現在のコードで通り、§5.2 が完成している |
| M3: 入口と経路の削除 | フェーズ 3 | manifest に `background`・`contextMenus` がなく、成果物に service worker と結果のウィンドウがない |
| M4: 共有コードの整理 | フェーズ 4 | `core/resultView.ts` がなく、送らない理由の表示が `renderBlockReason` になっている |
| M5: lint と文書 | フェーズ 5 | 削除した API の再導入を lint が禁止し、README・security.md と文書の検査が改まっている |
| M6: 整理の確認 | フェーズ 6 | §5.1 の 5 つの行と、§5.2〜§5.4 が埋まっている |
| M7: 手動確認 | フェーズ 7 | §5.1 のすべての欄が埋まり、`TestContextMenuRemovalRecords` が通る |

フェーズ 2 以降の各フェーズの完了条件には、上の表の条件に加えて、`make ext-check` → `make test` → `make lint` が通ることを含める。`make ext-check` の前に `make ext-install` が要る（`extension/node_modules` がない場合。ネットワークを使う）。PR の分け方は §3.2 のとおりである。

### 3.2. PR 構成

PR はフェーズと対応させ、次の 3 か所だけ変える。フェーズ 3 は、ファイルと manifest の削除（PR-2）と、`launch.ts`・`chromeDeps.ts` のコードとテストの削除（PR-3）に分ける。前者は権限と成果物の構成を変え、後者は削除の量が多いので、別々にレビューする。フェーズ 1 はコードを変えないので PR-1 に含める。フェーズ 6（PR-6）とフェーズ 7（PR-7）は分ける。PR-7 は手動確認と利用者の承認を待つので、PR-6 までを手動確認を待たずにマージできるようにするためである（§6.2）。PR-2 の後、PR-5 のマージまでの間は、`main` の README と `security.md` §8.1 の `:104` にメニューの経路と結果のウィンドウの記述が残る（`TestExtensionSendDocumentation` が `Context menu` を求め続けるため）。本タスクの PR は 0026 の PR-2 の前に続けてマージするので、この間の食い違いは受け入れる。各 PR のグリーンゲートは、`_context.md` の "Green gate"（`make test && make lint`）に `make ext-check` を加えたものとする（§1.2）。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 2-2 / 2-3 / 2-4 | カバレッジの基準の記録、§5.2 の対応の表、ポップアップの経路の検査の追加 | frontier-recommended |
| PR-2 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 | `background.ts`・`result.ts`・`result.html` の削除、manifest と `security.md` §8.1 の権限、`check-dist.ts` とそのテスト | frontier-recommended |
| PR-3 | 3-8 / 3-9 / 3-10 / 3-11 / 3-12 / 3-13 | `launch.ts`・`chromeDeps.ts` のメニューの経路の処理と、そのテストの削除・整理 | standard |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9 | `renderBlockReason` への置き換え、`core/resultView.ts` ほか共有コードの削除、`summarize` の検査 | standard |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 | lint の規則とそのテスト、README・`security.md`・Go の文書の検査 | standard |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 | 整理の確認とカバレッジの比較の記録（到達しなくなったものがあればテストの追加） | frontier-recommended |
| PR-7 | 7-1 / 7-2 / 7-3 / 7-4 | Chrome・Brave の手動確認の記録と `TestContextMenuRemovalRecords` | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。フェーズ 2 は、削除の前に加えた検査が現在のコードで通ることを確かめるために、フェーズ 3 より前に置く。フェーズ 3 で `launch.ts` の `ResultViewStore` と `chromeDeps.ts` の `createResultViewStore` を一緒に削除するのは、後者が前者の型を import している（`chromeDeps.ts:3`）ためである。`renderResultView` の `sending`・`sent` の場合を削除できるのは、`runResultWindow` がなくなった後（フェーズ 3 の後）である。このため、`renderBlockReason` への置き換えはフェーズ 4 で行う。フェーズ 3 では、manifest の権限と同じステップで `security.md` §8.1 の「要求する権限」も改める（設計書 8 章の優先順位 3。`TestExtensionDocumentation` が両者を両方向で比べるため）。フェーズ 5 の lint の規則は、フェーズ 3・4 で対象の名前がソースからなくなった後に加える。フェーズ 7 の記録の検査は、§5.1 の最後の欄が埋まった後に加える。

本タスクのすべての PR は、0026 の PR-2 を始める前にマージする（要件書 §5）。ステップ 7-2 は、`main` のサーバが HTTP で待ち受けている間に行う。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   設計書 7.1 のとおりである。拡張のテストは `node --test` で、fake の依存と jsdom を使う。実際のブラウザもサーバも起動しない。
-   ポップアップの経路の既存のテストは、期待値を変えずに通す（AC-06）。メニューの経路のテストだけが確かめていた振る舞いは、削除の前にポップアップの経路のテストで確かめるようにする（フェーズ 2）。§5.2 の対応の表で、削除・改める検査ごとに対応先を記録する。
-   ポップアップの経路の検査（フェーズ 2）と `summarize` の検査（ステップ 4-8）のほかに、新しく確かめるものは、manifest の複製の同一性（`checkDist.test.ts`）、lint の規則（`lintRules.test.ts`）、文書の語句（`extension_docs_test.go`）、§5.1 の記録（`TestContextMenuRemovalRecords`）である。新しいテストの補助のファイルは加えない（`docs/dev/developer_guide/test_organization.md` の `testutil/`・`test_helpers.go` の規則に当たるものはない）。

### 4.2. 統合テスト（手動）

ブラウザを起動する自動テストは設けない。設計書 7.2 の手順を、ステップ 7-1・7-2 で行い、§5.1 に記録する。

### 4.3. 後方互換性

-   ポップアップの経路とオプションページの振る舞い・文言・設定の保存（`chrome.storage.local`）を変えない。`popup send`・`launchContextFromTab`・`display` の describe と、`optionsPage.test.ts`・`settings.test.ts`・`send.test.ts`・`response.test.ts`・`prepare.test.ts`・`render.test.ts`・`renderMetadata.test.ts` を、期待値を変えずに通すことで確かめる（AC-06）。
-   サーバ（Go）の本番のコードを変えない。Go の変更は `extension_docs_test.go` だけである。
-   以前の版が残した `chrome.storage.session` の値とバッジは、ステップ 7-1 で確かめる（設計書 3.7）。

### 4.4. 横断検索

`make lint` と `make test` が検出できない残りを、次の 1 項目だけ手で確かめる。ほかの残りは、AC の表の検査（lint の規則・文書の検査）と `no-unused-vars`・`tsc` が検出する。

-   [ ] フェーズ 5 の後に、`extension/` の下（`node_modules`・`dist` を除き、`eslint.config.js` などの直下のファイルを含む）のコメントと文字列に、`result window`・`result view`・`ResultView`・`service worker`・`context menu`・`menu path` が残っていないことを `rg -n -i` で確かめる。`lintRules.test.ts` の入力の文字列と、ステップ 5-1 で加える規則のコメントと `message`（削除した API の再導入を禁止する理由として、これらの語を使ってよい）は除く。結果をステップ 6-1 の §5.1 の行に添える。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

ラベルは、`test`（実行され、誤った振る舞いで失敗する）、`static`（ガードのテスト・`make` の対象・コミットしたスクリプトが、文字列や構成を確かめる）、`manual`（人の確認と記録）である。テストの場所は `path::describe > it`（Go は `path::TestName`）で書く。

| AC | 種類 | 検証 | 実装のステップ |
|---|---|---|---|
| AC-01 | test | `extension/test/manifest.test.ts::permissions > declares exactly the three permissions the design requires`、`::undeclared keys > declares exactly the keys the design lists`（`static/manifest.json`） | 3-2・3-4 |
| AC-01 | static | `make ext-build` の `check-dist.ts` の manifest の複製の同一性の検査（`dist/manifest.json`）と、`extension/test/checkDist.test.ts::checkDist` に加えるそのテスト | 3-3・3-5 |
| AC-02 | static | `make ext-build` の `check-dist.ts` の既存のファイルの集合の比較。`extension/test/checkDist.test.ts::checkDist > rejects an extra file` | 3-1 |
| AC-03 | static | `make ext-lint`（加える `no-restricted-syntax` の規則）と、`extension/test/lintRules.test.ts` に加える describe | 3-1・3-8・3-9・3-10・3-11（使用の削除）、5-1・5-2（規則） |
| AC-04 | test | `extension/test/manifest.test.ts::permissions`（`contextMenus` がない）・`::undeclared keys`（`background` がない） | 3-2・3-4 |
| AC-04 | manual | §5.1 の「AC-04・AC-05」の行（`cmd/yt2column/extension_docs_test.go::TestContextMenuRemovalRecords` が欄の有無を確かめる） | 7-1・7-3 |
| AC-05 | test | AC-04 と同じ manifest のテスト | 3-2・3-4 |
| AC-05 | manual | §5.1 の「AC-04・AC-05」の行（同上） | 7-1・7-3 |
| AC-05a | test | `extension/test/launch.test.ts::popup send > shows a send button and does not send until it is clicked`・`> sends once, disables the button and shows the result`・`> sends only once when the button is clicked repeatedly`、`extension/test/manifest.test.ts::undeclared keys` | 既存・3-4 |
| AC-05a | manual | §5.1 の「AC-05a」の行（送信とネットワークの API の呼び出し箇所の確認） | 6-2・7-3 |
| AC-06 | test | `make ext-test` の、ポップアップの経路の既存のテスト（§4.3）と、フェーズ 2 で加える検査 | 2-2・3-12 |
| AC-06 | manual | §5.2 の対応の表 | 2-1・6-5 |
| AC-07 | test | `extension/test/launch.test.ts::popup send > sends once, disables the button and shows the result`（受付の表示）・`> shows the token-missing reason and no button without sending` | 既存 |
| AC-07 | manual | §5.1 の「AC-07」の行 | 7-2・7-3 |
| AC-08 | test | `extension/test/launch.test.ts::popup send > shows the sending notice and a disabled button while sending`（2 つの文の全体の検査） | 2-3 |
| AC-09 | static | `cmd/yt2column/extension_docs_test.go::TestContextMenuRemovalRecords`（§5.1 の「AC-09」の行が埋まっている） | 6-3・7-3 |
| AC-09 | manual | §5.3 のカバレッジの比較 | 1-1・6-3 |
| AC-10 | static | `cmd/yt2column/extension_docs_test.go::TestContextMenuRemovalRecords`（§5.1 の「AC-10」の行が埋まっている） | 6-4・7-3 |
| AC-11 | static | `cmd/yt2column/extension_docs_test.go::TestExtensionSendDocumentation`（README の求める語句と禁止する語句）、`extension/test/metadataDocs.test.ts::video metadata documentation` | 5-3・5-5 |
| AC-12 | static | `cmd/yt2column/extension_docs_test.go::TestExtensionDocumentation`（manifest の権限と §8.1 の一覧の一致と、加える §8.1 の語句の禁止）、`extension/test/manifest.test.ts::host permissions > is recorded in the extension section of security.md` | 3-2・5-4・5-5 |
| AC-13 | static | `make ext-check`・`make test`・`make lint`（各フェーズの完了条件）と、`doc-guard` の CI ジョブ | 全フェーズ |

AC-02 は、ソースを削除したことで成り立つ。`static/result.html` や `src/result.ts` を戻せば、集合の比較はそれを成果物として受け入れる。これはファイル名を列挙しない設計の選択による（設計書 1.1・3.3.2）。

AC-09・AC-10 は、確かめた結果を本計画に記録することを求める基準である。このため、`static` の検証は記録があることの検査とし、比較そのものは `manual` として §5.3 と §5.1 に記録する。

### 5.1. 手動の確認と記録

各行の「結果」と「日付・コミット・版」は、実施したときに記入する。空の欄があると `TestContextMenuRemovalRecords`（ステップ 7-3）が失敗する。

| AC | 確かめること | 手順 | 結果 | 日付・コミット・版 |
|---|---|---|---|---|
| AC-04・AC-05 | 以前の版を読み込み直した後と、ブラウザを再起動した後に、文字起こしパネルの行の右クリックメニューに拡張の項目がない（Chrome・Brave） | 設計書 7.2 の手順 1〜4・6 | | |
| 設計書 3.7 | 読み込み直しの後に、`chrome.storage.session` が空で、バッジがない（Chrome・Brave。H-05） | 設計書 7.2 の手順 2・3・5 | | |
| AC-07 | ポップアップから送って受付（`202`）が表示され、共有トークンを削除すると送らない理由が表示される（Chrome・Brave） | 設計書 7.2 の手順 7 | | |
| AC-05a | 送信とネットワークの API の呼び出し箇所が、設計書 3.5.1 のとおり 1 か所ずつである | ステップ 6-2 | | |
| 設計書 3.2.7 | 本番の参照元を変えたモジュールの export の参照が、設計書 3.2.1 の表と一致する。§4.4 の横断検索の結果 | ステップ 6-1 | | |
| AC-09 | 削除の前に到達していた、残る行・分岐・関数が、削除の後も到達している | ステップ 1-1・6-3（詳細は §5.3） | | |
| AC-10 | Go のテストを削除していない | ステップ 6-4 | | |
| 設計書 3.10.2 | 削除・改めるテストのすべての検査に、対応先か理由がある | ステップ 2-1・6-5（詳細は §5.2） | | |

### 5.2. 削除・改めるテストの検査の対応表

ステップ 2-1 で作り、ステップ 6-3 で加えたテストがあれば追記する。列は、元のテスト（ファイル・describe・`it`）、検査の内容、対応先（本タスクの後のテストの名前）または理由、とする。

元のテストの行番号は `81cb30d` の行を指す。テストの名前は `describe > it` で書き、ファイルを省いたものは `extension/test/launch.test.ts` のテストである。対応先のうち「（残す）」は、同じテストの同じ検査を、期待値を変えずに残すことを表す。「（加えた）」は、ステップ 2-2・2-3 で加えた検査を表す。テストの名前は §2.1 の表の新しい名前で書く。ループや表で複数回実行される `assert` は、ソースの 1 か所を 1 件とする。ただし、メニューとポップアップの両方の記録をたどるループの `assert`（`:599`・`:1076`・`:1088`）は、対応先が経路で分かれるので、経路ごとに 2 件に分ける。

確かめる必要がなくなった理由は、次の記号で書く。

-   **R1:** 削除するストア（`ResultViewStore`・`chrome.storage.session` への受け渡し）だけの振る舞い。ポップアップの経路は値を保存せず、同じ容器に描く。
-   **R2:** 削除するバッジ・結果のウィンドウを開く処理・表示用のキー（`newKey`）だけの振る舞い。
-   **R3:** 削除する結果のウィンドウ（`runResultWindow`）だけの振る舞い。
-   **R4:** メニューの経路の入力（`chrome.contextMenus.OnClickData`）と、そのログのラベルだけの振る舞い。
-   **R5:** 削除する関数（`parseSummary`・`parseResultView`・`createResultViewStore`・`unreadableResultMessage`・`summaryNode`）だけの振る舞い。
-   **R6:** メニューの経路の値とポップアップの経路の値の比較。比べる相手がなくなる。確かめていた値は、対応先がリテラルの期待値で確かめる。
-   **R7:** 削除する `runMenuLaunch` の側の検査。同じテストの、ポップアップの経路の同じ期待値の検査は残す。

| 元のテスト | 検査の内容 | 対応先または理由 |
|---|---|---|
| launch paths > collects the same input through the menu and popup paths（`:417`） | ポップアップの収集の結果が `collected` | launch paths > collects the selection and the tab title through the popup path（残す） |
| 同上 | 収集した `selection` が読み取った選択範囲のまま | 同上（残す） |
| 同上 | 収集した `title` がタブのタイトルのまま | 同上（残す） |
| 同上 | メニューの起動のログの `outcome` がポップアップの収集の結果と等しい | 同上の、ポップアップの起動のログの `outcome` が収集の結果と等しい検査（加えた） |
| 同上 | ストアに置いた `sending` の表示が `summarize` の要約を持つ | R1。ポップアップの要約の表示は `render.test.ts::renderSummary` と display > renders the popup summary as text, not elements が確かめる |
| launch paths > sends the same body and classification through both paths（`:442`） | メニューの本文が文字列 | launch paths > sends the video title, not the tab title, in the body の、ポップアップの本文が文字列である検査（加えた） |
| 同上 | メニューの本文が `videoUrl`・`transcript`・動画タイトル・チャンネル名・説明のリテラルと等しい（タブのタイトルではない） | 同上の、ポップアップの本文に同じリテラルを当てる検査（加えた） |
| 同上 | ポップアップの本文がメニューの本文と等しい | R6（対応先は上の行） |
| 同上 | メニューの `sent` の表示がストアにある | R1 |
| 同上 | メニューの `sent` の表示の結果が受付（`accepted`・`requestId`） | popup send > sends once, disables the button and shows the result の、受付の文言を表示する検査（加えた）と `.send-request-id` の検査 |
| 同上 | ポップアップの `.send-request-id` が `requestId` | 同上（残す） |
| launch paths > does not read settings or send for a rejected collection（`:499`） | 拒否した収集で設定を読まない | popup send > renders nothing and does not read settings for a rejected collection の `loadCalls` の検査 |
| 同上 | 拒否した収集で送らない | 同上の、容器が空である（送信のボタンがなく、送信はボタンからしか始まらない）検査 |
| 同上 | ストアに `blocked`（`not-watch-page`）を置く | R1。ポップアップは拒否の理由を要約の領域に表示し、launch paths > does not read the metadata for a not-watch-page collection が理由を確かめる |
| launch paths > reads the metadata once, from the collected tab and URL（`:515`） | メニューの経路のメタデータの読み取りが 1 回で、タブの ID と URL が収集したもの | R7 |
| 同上 | ポップアップの経路の同じ検査 | launch paths > reads the metadata once, from the collected tab and URL（残す） |
| launch paths > does not read the metadata for a ${name} collection（`:548`。4 行） | メニューの経路でメタデータを読まない | R7 |
| 同上 | ポップアップの経路でメタデータを読まない | launch paths > does not read the metadata for a ${name} collection（残す） |
| 同上 | ポップアップの収集の結果が `rejected` | 同上（残す） |
| 同上 | 拒否の理由が `${name}` | 同上（残す） |
| 同上 | メタデータが空の結果 | 同上（残す） |
| 同上 | ストアに `blocked`（`${name}`）を置く | R1 |
| launch paths > sends without metadata and logs an error when ${name}（`:599`。3 行） | メニューの経路のストアの表示が `sending`・`sent` の順（送った） | R1。ポップアップから送ったことは次の行の本文の検査が確かめる |
| 同上 | メニューの経路の本文が `videoUrl`・`transcript` だけ（メタデータなし） | launch paths > sends without metadata and logs an error when ${name} の、ポップアップから送った本文が `videoUrl`・`transcript` だけである検査（加えた） |
| 同上 | ポップアップの収集の結果が `collected` | 同上（残す） |
| 同上 | ポップアップのメタデータの値が空 | 同上（残す） |
| 同上 | メタデータのログが error で、`omissions`（と `check`）を持つ（メニューの記録） | R7 |
| 同上 | 同じ検査（ポップアップの記録） | 同上（残す） |
| launch paths > treats a missing tab as collection-failed in both paths（`:635`） | ポップアップの収集の結果が `rejected` | launch paths > treats a missing tab as collection-failed（残す） |
| 同上 | 拒否の理由が `collection-failed` | 同上（残す） |
| 同上 | メニューの起動のログの `outcome` が `rejected` | launch paths > treats a missing tab as collection-failed の、ポップアップの起動のログの `outcome` が戻り値の拒否と等しい検査（加えた） |
| 同上 | メニューの起動のログの理由が `collection-failed` | 同上 |
| launch paths > ignores info.selectionText and reads the selection itself（`:664`） | `info.selectionText` があっても収集が `collected` | R4。ポップアップの経路は `info` を持たない |
| 同上 | 収集した選択範囲がページから読んだもの | R4。ページから読んだ選択範囲を収集することは launch paths > collects the selection and the tab title through the popup path が確かめる |
| launch paths > keeps untrusted text out of the fixed log labels（`:678`） | メニューの起動のログのラベルが固定の文字列 | R4 |
| 同上 | ポップアップの起動のログのラベルが固定の文字列 | launch paths > keeps untrusted text out of the fixed log labels（残す） |
| launch paths > clears the badge at the start of the menu path（`:688`） | バッジを 1 回消す | R2 |
| launch paths > signals the badge and does not throw when the key cannot be created（`:694`） | ウィンドウを開かない | R2 |
| 同上 | バッジを 1 回出す | R2 |
| 同上 | error のログが 1 件 | R2 |
| launch paths > does not open a window and signals the badge when put fails（`:706`） | ウィンドウを開かない | R2 |
| 同上 | キーを削除しない | R1 |
| 同上 | バッジを 1 回出す | R2 |
| launch paths > does not send when the result window cannot be opened（`:715`） | キーを削除する | R1 |
| 同上 | バッジを 1 回出す | R2 |
| 同上 | ウィンドウを開けないと送らない | R2（ウィンドウの表示の後に送る順序はメニューの経路だけのもの） |
| launch paths > does not reject when the window and the removal both fail（`:727`） | キーの削除を試みる | R1 |
| 同上 | バッジを 1 回出す | R2 |
| 同上 | error のログが 2 件 | R2 |
| menu send > sends once and stores the sending then the sent view（`:743`） | ストアの表示が `sending`・`sent` の順 | R1 |
| 同上 | 1 回だけ送る | popup send > sends once, disables the button and shows the result の `fetch` の回数の検査 |
| 同上 | `sent` の表示がある | R1 |
| 同上 | 結果が受付（`accepted`・`requestId`） | popup send > sends once, disables the button and shows the result の、受付の文言を表示する検査（加えた）と `.send-request-id` の検査 |
| menu send > stores an unknown result without its cause（`:756`） | `sent` の表示がある | R1 |
| 同上 | 送信の失敗の結果が `cause` のない `unknown` | popup send > logs a failed send at error and shows the unknown result の、`unknown` の文言を表示する検査（加えた）。`cause` を落とすことは `response.test.ts::toSendOutcome > drops the cause of an unknown result` が確かめる |
| 同上 | error のログが 1 件 | popup send > logs a failed send at error and shows the unknown result の、送信の結果のログが error だけにある検査（加えた） |
| menu send > blocks ${reason} without sending（`:792`。4 行） | ストアに `blocked`（`${reason}`）を置く | popup send > shows the ${reason} reason and no button without sending の、送らない理由の文言の検査（`body-too-large` は、加えた「… when the selection takes the body over the limit」の行） |
| 同上 | 送らない | 同上の `fetch` の回数の検査 |
| 同上 | ウィンドウを開く | R2 |
| 同上 | 送らない理由のログが、`settings-unreadable` では error、ほかは info にある | 同上の、ラベル `yt2column blocked` の記録が `error` の列で選んだ一方のレベルにだけあり、他方のレベルにない検査（加えた） |
| 同上 | 送らない理由のログが `reason` を持つ | 同上の、記録の `reason` が `[reason]` と等しい検査（加えた） |
| menu send > sends a collapsed description without a confirmation step（`:811`） | 1 回送る | popup send > keeps the button under the expand prompt and sends the shown description の `fetch` の回数の検査 |
| 同上 | 本文の説明が読み取った折りたたみの説明 | 同上の本文の説明の検査 |
| 同上 | ストアの表示に展開の案内がない | R1。ポップアップでは展開の案内とボタンが並ぶことを同上のテストが確かめる |
| menu send > blocks body-too-large when the metadata takes the body over the limit（`:833`） | ストアに `blocked`（`body-too-large`）を置く | popup send > shows the body-too-large reason and no button without sending when the metadata takes the body over the limit の、送らない理由の文言の検査（加えた） |
| 同上 | ウィンドウを開く | R2 |
| 同上 | 送らない | 同上の `fetch` の回数の検査（加えた） |
| menu send > signals the badge when the blocked view cannot be stored（`:848`） | ウィンドウを開かない | R2 |
| 同上 | バッジを 1 回出す | R2 |
| 同上 | error のログが 1 件（保存の失敗）。送らない理由のログが error にないことも含む | 保存の失敗は R1。`token-missing` の送らない理由が error に記録されないことは、popup send > shows the token-missing reason and no button without sending の、他方のレベルにない検査（加えた）が確かめる |
| menu send > logs and signals the badge when the sent view cannot be stored（`:860`） | 1 回送る | popup send > sends once, disables the button and shows the result の `fetch` の回数の検査 |
| 同上 | バッジを 1 回出す | R2 |
| 同上 | error のログが 1 件（保存の失敗）。受付の送信のログが error にないことも含む | 保存の失敗は R1。受付の送信が info に 1 件だけ記録され error にないことは、popup send > sends once, disables the button and shows the result のログの検査（加えた）が確かめる |
| video metadata log > records ${name} at info（`:1076`。2 行） | メタデータのログが info で、`accepted`・`collapsed`（と `omissions`）を持つ（メニューの記録） | R7 |
| 同上 | 同じ検査（ポップアップの記録） | video metadata log > records ${name} at info（残す。補助 `metadataLogs` をポップアップの経路だけにする） |
| 同上 | error のログがない（メニューの記録。メタデータのログのほか、受付の送信のログが error にないことも含む） | メタデータのログの部分は R7。受付の送信のログの部分は、popup send > sends once, disables the button and shows the result のログの検査（加えた） |
| 同上 | 同じ検査（ポップアップの記録） | 同上（残す） |
| video metadata log > keeps the read values and the read error out of the logs（`:1088`） | 読み取った値・省いた値・拒否した形・読み取りの失敗の文字列がログにない（メニューの記録。送信と送らない理由のログを含む） | メタデータのログの部分は R7。送信と送らない理由のログの部分は、同じテストの補助 `metadataLogs` がポップアップから送る場合と送らない場合（`token-missing`）のログも記録して確かめる（加えた） |
| 同上 | 同じ検査（ポップアップの記録） | video metadata log > keeps the read values and the read error out of the logs（残す） |
| result window > shows a sending view, then the sent result on a notification（`:1167`） | 送信中の文言を表示する | R3。ポップアップの送信中の表示は popup send > shows the sending notice and a disabled button while sending が確かめる |
| 同上 | 通知の後に `.send-request-id` を表示する | R3 |
| 同上 | キーを削除する | R3 |
| 同上 | 監視を止める | R3 |
| 同上 | 待ち時間の上限が `resultWaitMs` | R3 |
| 同上 | 最終の表示で待ち時間の予約を取り消す | R3 |
| result window > keeps the sent result when the get resolves after the notification（`:1189`） | `.send-request-id` を表示し続ける | R3 |
| result window > ignores a get rejection that arrives after the final view（`:1203`） | `.send-request-id` を表示し続ける | R3 |
| 同上 | error のログがない | R3 |
| 同上 | 待ち時間の予約がない | R3 |
| result window > shows a blocked view and removes the key（`:1227`） | 送らない理由の文言を表示する | R3。ポップアップの表示は popup send > shows the token-missing reason and no button without sending が確かめる |
| 同上 | キーを削除する | R3 |
| result window > logs a failure when the key cannot be removed at Final（`:1237`） | 送らない理由の文言を表示する | R3 |
| 同上 | error のログが 1 件 | R3 |
| result window > shows the unknown display at the wait limit and logs it（`:1248`） | `unknown` の文言を表示する | R3 |
| 同上 | キーを削除する | R3 |
| 同上 | error のログが 1 件 | R3 |
| result window > shows the unreadable notice when get fails（`:1262`） | 読めない場合の文言を表示する | R3 |
| 同上 | error のログが 1 件 | R3 |
| result window > shows the unreadable notice when get returns no value（`:1271`） | 読めない場合の文言を表示する | R3 |
| 同上 | error のログが 1 件 | R3 |
| result window > shows the unreadable notice when the stored shape is wrong（`:1280`） | 読めない場合の文言を表示する | R3 |
| 同上 | error のログが 1 件 | R3 |
| result window > shows the unreadable notice without a key and does not touch the store（`:1290`） | 読めない場合の文言を表示する | R3 |
| 同上 | キーを削除しない | R3 |
| 同上 | 監視しない | R3 |
| result window > ignores events after the final view（`:1300`） | `.send-request-id` を表示し続ける | R3 |
| 同上 | キーを 1 回だけ削除する | R3 |
| token never leaks > keeps the token out of the logs, the store and the display（`:1318`） | 記録と表示にトークンの全体がない | token never leaks > keeps the token out of the logs and the display の同じ検査（残す）。対象に、ポップアップの送信のログ（受付と失敗）と、トークンを読み込んだうえで送らない場合（`port-invalid`）のログと表示を加え、`Error` の名前と文言も直列化する。メニューの記録とストアは R7・R1 |
| 同上 | トークンの先頭の 4 文字がない | 同上 |
| 同上 | トークンの末尾の 4 文字がない | 同上 |
| `renderSend.test.ts`: renderResultView > shows the ${reason} block message（`:105`。2 行） | 送らない理由の文言を表示する | `renderSend.test.ts`: renderBlockReason > shows the ${reason} block message（ステップ 4-8 で `renderBlockReason` を呼ぶ形に改める） |
| `renderSend.test.ts`: renderResultView > shows the sending notice with the close guidance and the summary（`:113`） | 送信中の文言（`sendingMessage`）を表示する | popup send > shows the sending notice and a disabled button while sending の `expectMessage` の検査（残す） |
| 同上 | 表示が `close` を含む | 同上の、`Do not close this window until the result appears.` を文の全体で確かめる検査（加えた。AC-08） |
| 同上 | 表示が `Slack` を含む | 同上の、`If you close it, check Slack before sending again.` を文の全体で確かめる検査（加えた。AC-08） |
| 同上 | 表示が要約のタイトルを含む | R5（`summaryNode`）。ポップアップの要約のタイトルは `render.test.ts::renderSummary > shows the title, URL, counts and preview with labels` が確かめる |
| `renderSend.test.ts`: renderResultView > shows the sent outcome and its request id alongside the summary（`:125`） | `.send-request-id` を表示する | `renderSend.test.ts`: renderSendOutcome > shows the ${outcome.kind} message and its request id |
| 同上 | 表示が要約のタイトルを含む | R5（`summaryNode`） |
| `renderSend.test.ts`: renderResultView > displays markup in the summary as text, not elements（`:136`） | 要約のマークアップを要素にしない | R5（`summaryNode`）。ポップアップの要約は display > renders the popup summary as text, not elements と `render.test.ts::renderSummary > displays markup in the title and preview as text, not elements` が確かめる |
| 同上 | 要約のマークアップを文字列として表示する | 同上 |
| `summary.test.ts`: parseSummary > accepts summarize output for a collected input（`:106`） | `parseSummary(summarize(o))` が要約と等しい（収集した要約のキーの集合を含む） | キーの集合は `summary.test.ts`: summarize に、収集した要約がちょうど 9 つのキーを持つことを値の全体で確かめる検査（ステップ 4-8 で加える）。`parseSummary` の受け入れは R5 |
| `summary.test.ts`: parseSummary > accepts summarize output for every rejection reason（`:111`） | すべての拒否の理由で `parseSummary(summarize(o))` が要約と等しい（`cause` を落とす） | `cause` を持つ拒否が `kind`・`reason` だけの値になることを、すべての理由について確かめる検査（ステップ 4-8 で `summary.test.ts`: summarize に加える）。`parseSummary` の受け入れは R5 |
| `summary.test.ts`: parseSummary > rejects a non-object value（`:123`） | オブジェクトでない値を拒む | R5 |
| `summary.test.ts`: parseSummary > rejects an unknown kind（`:129`） | 未知の `kind` を拒む | R5 |
| `summary.test.ts`: parseSummary > rejects a missing field（`:133`） | 前提: 要約が `collected` | R5 |
| 同上 | 前提: 取り除いた `preview` が `x` | R5 |
| 同上 | フィールドの欠けた要約を拒む | R5 |
| `summary.test.ts`: parseSummary > rejects an extra field（`:141`） | 余分なフィールドを拒む | R5 |
| `summary.test.ts`: parseSummary > rejects a collected summary whose ${field} has the wrong type（`:157`。8 行） | 型の違うフィールドを拒む | R5 |
| `summary.test.ts`: parseSummary > rejects an unknown reason（`:163`） | 未知の拒否の理由を拒む | R5 |
| `summary.test.ts`: parseSummary > rejects a rejection with an extra field（`:170`） | 余分なフィールドを持つ拒否を拒む | R5 |
| `sendMessages.test.ts`: sendOutcomeMessage > tells the user to check Slack when a result cannot be displayed（`:119`） | 読めない場合の文言が `Slack` を含む | R5（`unreadableResultMessage`） |
| `chromeDeps.test.ts`: result view store > puts, gets and removes a value under the key（`:437`） | 置いた値を読める | R5（`createResultViewStore`） |
| 同上 | 読んでも値を削除しない | R5 |
| 同上 | 削除の後は読めない | R5 |
| `chromeDeps.test.ts`: result view store > watch reports a value newly stored under the key in session（`:448`） | `session` の新しい値を通知する | R5 |
| `chromeDeps.test.ts`: result view store > watch ignores another area, another key, and a removal（`:460`） | ほかの領域・キーと削除を通知しない | R5 |
| `chromeDeps.test.ts`: result view store > watch stops after the returned function is called（`:474`） | 止めた後は通知しない | R5 |
| `resultView.test.ts`: parseResultView > accepts a blocked view for every block reason（`:38`） | すべての送らない理由の `blocked` を受け入れる | R5（`parseResultView`） |
| `resultView.test.ts`: parseResultView > accepts a sending view（`:49`） | `sending` を受け入れる | R5 |
| `resultView.test.ts`: parseResultView > accepts a sent view for every outcome kind（`:54`） | 各結果の `sent` を受け入れる | R5 |
| `resultView.test.ts`: parseResultView > rejects a sent view whose unknown outcome carries a cause（`:68`） | `cause` を持つ `unknown` を拒む | R5 |
| `resultView.test.ts`: parseResultView > rejects an unexpected outcome with status ${status}（`:77`。4 行） | 範囲の外の状態コードを拒む | R5 |
| `resultView.test.ts`: parseResultView > accepts an unexpected outcome with status ${status}（`:88`。2 行） | 範囲の端の状態コードを受け入れる | R5 |
| `resultView.test.ts`: parseResultView > rejects an unknown unexpected check（`:94`） | 未知の `check` を拒む | R5 |
| `resultView.test.ts`: parseResultView > rejects a non-object value（`:103`） | オブジェクトでない値を拒む | R5 |
| `resultView.test.ts`: parseResultView > rejects an unknown kind（`:109`） | 未知の `kind` を拒む | R5 |
| `resultView.test.ts`: parseResultView > rejects a blocked view with an unknown reason（`:113`） | 未知の送らない理由を拒む | R5 |
| `resultView.test.ts`: parseResultView > rejects a sending view whose summary is a rejection（`:120`） | 拒否の要約を持つ `sending` を拒む | R5 |
| `resultView.test.ts`: parseResultView > rejects a blocked view with an extra field（`:130`） | 余分なフィールドを持つ `blocked` を拒む | R5 |
| `resultView.test.ts`: parseResultView > rejects a sending view missing the summary（`:137`） | 要約のない `sending` を拒む | R5 |
| `resultView.test.ts`: parseResultView > rejects an accepted outcome with an unparseable requestId（`:141`） | 形の違う `requestId` を拒む | R5。`requestId` の検査そのものは `response.test.ts::parseRequestId > rejects ${value}` が確かめる |
| `resultView.test.ts`: parseResultView > rejects a rejected outcome with an unknown reason（`:148`） | 未知のサーバの拒否の理由を拒む | R5。理由の検査そのものは `response.test.ts::serverRejectStatus > is queried through isServerRejectReason` が確かめる |
| `resultView.test.ts`: parseResultView > accepts a rejected outcome for every server reason（`:153`） | すべてのサーバの拒否の理由を受け入れる | R5 |
| `manifest.test.ts`: permissions > declares exactly the four permissions the design requires（`:19`） | `permissions` が `contextMenus` を含む 4 つと一致する | `manifest.test.ts`: permissions > declares exactly the three permissions the design requires（ステップ 3-4 で期待値を 3 つに改める。`contextMenus` がないことを確かめる） |
| `manifest.test.ts`: undeclared keys > declares exactly the keys the design lists（`:30`） | キーの集合が `background` を含むものと一致する | 同じテスト（ステップ 3-4 で集合から `background` を除き、`background` がないことを確かめる） |
| `manifest.test.ts`: fixed fields > loads the service worker as a module（`:87`） | `background` が `background.js` をモジュールとして読み込む | service worker を削除するので不要。`background` がないことは上の「undeclared keys」が確かめる |
| `checkDist.test.ts`: checkDist > accepts a manifest whose references exist（`:137`） | 参照先がすべてある manifest を受け入れる（service worker を含む） | 同じテスト（ステップ 3-5 で service worker の参照を除き、`action.default_popup`・`options_ui.page` の参照を確かめる） |
| `checkDist.test.ts`: checkDist > rejects a manifest ${name} that is not in dist/（`:189`。3 行） | 参照先のない manifest の参照を、参照ごとに違反として報告する | `popup`・`options page` の行は同じテスト（ステップ 3-5 で `serviceWorker` のフィールドを除く）。`service worker` の行は、`check-dist.ts` から `background.service_worker` の項目を削除するので不要（設計書 3.3.2） |
| `repository.test.ts`: build outputs are ignored > ignores ${file}（`:60` の行） | `extension/dist/background.js` が無視される | `repository.test.ts`: build outputs are ignored > ignores extension/dist/popup.js（ステップ 3-6 で入力を残る成果物の名前に改める） |

設計書 3.10.2 の「加えるテスト」の表の 6 つの振る舞いと、上の表で見つけた 5 つ（受付の送信のログのレベル、ポップアップの起動のログの `outcome`、送らない理由のログの `reason`、拒否した収集の起動のログ、送信と送らない理由のログの読み取った値）の対応先は、次のとおりである。

| 振る舞い | 加えた検査 |
|---|---|
| 送る本文の値（動画タイトルが本文に入る） | launch paths > sends the video title, not the tab title, in the body |
| 読めなかったときにメタデータなしで送る | launch paths > sends without metadata and logs an error when ${name} |
| 送信と送らない理由のログにトークンが出ない | token never leaks > keeps the token out of the logs and the display |
| 送らない理由のログのレベルの振り分け | popup send > shows the ${reason} reason and no button without sending |
| 本文が大きすぎる場合（選択範囲・メタデータ）に送らない | popup send > shows the body-too-large reason and no button without sending when the selection takes the body over the limit・… when the metadata takes the body over the limit |
| 送信の失敗を error で記録する | popup send > logs a failed send at error and shows the unknown result |
| 受付の送信を info に 1 件だけ記録し、error に記録しない | popup send > sends once, disables the button and shows the result |
| ポップアップの起動のログが `outcome` を持つ | launch paths > collects the selection and the tab title through the popup path |
| 送らない理由のログが `reason` を持つ | popup send > shows the ${reason} reason and no button without sending |
| 拒否した収集のポップアップの起動のログが `outcome` を持つ | launch paths > treats a missing tab as collection-failed |
| ポップアップの送信と送らない理由のログに、読み取った値とページの文字列が出ない | video metadata log > keeps the read values and the read error out of the logs |

この段階（ステップ 2-2・2-3 の後）では、上の「改める」テストは、まだ元の名前とメニューの経路の部分を持つ。名前の変更とメニューの経路の部分の削除は、ステップ 3-12 で行う。

### 5.3. カバレッジの比較の結果

ステップ 1-1 で基準の報告の取り方を記録し、ステップ 6-3 で比較の結果を記録する。記録するものは、実行したコマンド、node の版、比べた 2 つのコミット、ファイルごとの比較の結果、到達しなくなったものとその対応、削除の前から到達していなかったものである。

**削除の前の基準の報告（ステップ 1-1、2026-10-19）**

-   作業ツリー: HEAD `0860b6f`。`git diff --quiet 81cb30d -- extension` の終了コードは 0 で、`extension/` は `81cb30d` と同じである。
-   node: `v24.21.0`（`extension/.node-version` と同じ）。
-   コマンド（`extension/` で実行）: `node --test --experimental-test-coverage --test-reporter=lcov --test-reporter-destination=<出力先> "test/**/*.test.ts"`。終了コードは 0 で、報告は 20 個の `SF:` を持つ。§1.3 で挙げた `logSend` の error の分岐は、`BRDA:217,23,0,1`・`DA:218,1` で到達回数 1 だった。
-   保存場所: リポジトリの外の `~/.cache/yt2column-0027/lcov-before-81cb30d.info`（出力先をジョブの一時ディレクトリにして実行し、そこへ複製した）。報告は `81cb30d` の `extension/` で同じコマンドを実行すれば作り直せるので、コミットしない。

**比較の結果（ステップ 6-3）**

（実施のときに記入する）

### 5.4. 確認のコマンドと出力

ステップ 6-1・6-2・6-4 で実行したコマンド、その出力、実行したコミットを記録する。§5.1 の欄はこの節を参照する。

（実施のときに記入する）

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| メニューの経路のテストだけが確かめていた振る舞いを、ポップアップの経路で確かめ損なう | 削除の後に、ポップアップの経路の不具合が検出されない | 削除の前に §5.2 の対応の表を作り（ステップ 2-1）、加えた検査が現在のコードで通ることを確かめてから削除する。カバレッジの比較（ステップ 6-3）で、到達しなくなった行と分岐を見つける |
| 加える lint の規則が、名前だけで判定するので、無関係のオブジェクトの同名のメンバも禁止する | 将来、`session`・`windows` などの名前のメンバを使うコードが lint で失敗する | 既存の `chrome.storage.sync` の規則と同じ割り切りとして受け入れる（設計書 3.5.2）。規則のコメントに、名前だけで判定することを記す |
| `dist/` を作り直さずに読み込み直し、以前の版のメニューの経路が残る | 確認なしの送信が残る | README の更新の手順に、作り直してから読み込み直すことを記す（ステップ 5-3）。手動確認でも作り直しを手順に含める（ステップ 7-1） |
| 以前の版が残した `chrome.storage.session` の値やバッジが、読み込み直しの後も残る | 古い表示が残る | ステップ 7-1 で確かめる。残る場合は、消す処理を加えるかをその時点で判断する（設計書 3.7） |
| 0026 の PR-2 との順序 | 本タスクの PR が PR-2 の後に残ると、ステップ 7-2 の受付の表示を確かめるサーバがなくなる | 本タスクのすべての PR を PR-2 の前にマージする（要件書 §5）。ステップ 7-2 は `main` のサーバが HTTP で待ち受けている間に行う |

### 6.2. スケジュールのリスク

-   0026 は、PR-1 の後、本タスクの完了まで止まる（0026 `03_implementation_plan.md` §1.3「0027 との順序」）。本タスクは削除が中心なので、フェーズ 2 の対応の表とフェーズ 6 のカバレッジの比較に時間を取る。
-   手動確認（フェーズ 7）は、利用者の承認と、Chrome・Brave の操作を要する。フェーズ 6 までの PR は、手動確認を待たずにマージできる。

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 2-2 / 2-3 / 2-4）
-   [ ] PR-2 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7）
-   [ ] PR-3 マージ済み（対象ステップ: 3-8 / 3-9 / 3-10 / 3-11 / 3-12 / 3-13）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 / 4-9）
-   [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6）
-   [ ] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6）
-   [ ] PR-7 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4）
-   [ ] §4.4 の横断検索を行った
-   [ ] §5 のすべての AC の検証が通った
-   [ ] 本タスクのすべての PR を、0026 の PR-2 を始める前にマージした

## 8. 成功基準 (Success Criteria)

-   **機能の完全性:** 拡張の入口のモジュールが `popup.ts`・`options.ts` の 2 つだけになり、manifest に `background`・`contextMenus` がない。送信はポップアップの「Send to yt2column」のボタンからだけ始まる（AC-01〜AC-05a）。
-   **品質:** ポップアップの経路の既存のテストが期待値を変えずに通る（AC-06）。削除の前に到達していた、残る行・分岐・関数が、削除の後も到達している（AC-09）。`make ext-check`・`make test`・`make lint` が通る（AC-13）。
-   **セキュリティ:** 要求する権限が 3 つになり、削除した API の再導入を lint が禁止する（AC-01・AC-03・AC-12）。共有トークンがポップアップのログと表示に出ないことを、改めた「token never leaks」が確かめる。
-   **文書:** README と `docs/dev/security.md` §8.1 がポップアップだけの構成を述べ、文書の検査がメニューの経路への言及を禁止する（AC-11・AC-12）。§5.1 のすべての欄が埋まっている。

## 9. 次のステップ (Next Steps)

-   §3.2 の PR の順に `/runplan` で実装する。
-   本タスクのすべての PR がマージされた後、0026 の PR-2 を始める。0026 のフェーズ 3 の前に、本タスクの後に残った名前（`schedule` の削除、`ResultView` と `core/resultView.ts` の削除、`renderBlockReason`）と行番号を、0026 の計画（§1.3「0027 との順序」）が確かめる。
-   #330（DOM からの文字起こしの読み取り）は、ポップアップの経路だけを対象にして進める（設計書 9 章）。
