# 実装計画書：ブラウザ拡張の右クリックメニューの経路の削除

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

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

-   `design_handoff.md` の H-01〜H-05 は、設計書 3.12 がすべて記録している。本計画は、それぞれを次のステップで実施する。H-01 はフェーズ 3・4 とステップ 6-1、H-02 はステップ 3-4・3-5・3-9・3-10、H-03 はフェーズ 1 とステップ 6-3、H-04 は変更なし（設計書 3.6。コードも文書も変えない）、H-05 はステップ 7-1 で行う。
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
-   [ ] **ステップ 1-1**: 削除の前の lcov の報告を取る（設計書 3.11）。作業ツリーの `extension/` が `81cb30d` と同じであることを `git diff --quiet 81cb30d -- extension` の終了コード 0 で確かめる。そのうえで、§1.3「カバレッジの取り方の確かめ」のコマンドで報告を取り、リポジトリの外に保存する。報告は `81cb30d` を取り出せば作り直せるので、コミットしない。
    -   **対象:** 本計画（§5.3）。**完了:** 実行したコマンド・node の版・比べた元のコミット・報告の保存場所を §5.3 に記録した。

**完了条件:** §5.3 に基準の報告の記録がある。コードは変えない。

### フェーズ 2: ポップアップの経路のテストの補強

**対象ファイル**
-   変更: `extension/test/launch.test.ts`、本計画（§5.2）

**タスク**
-   [ ] **ステップ 2-1**: §5.2 に、削除・改めるテスト（§1.3 の `launch.test.ts` の表、`renderSend.test.ts`・`summary.test.ts`・`sendMessages.test.ts`・`chromeDeps.test.ts`・`resultView.test.ts` の削除の対象）の検査を、`assert` 1 つにつき 1 件として挙げる。そして、本タスクの後に同じ値を確かめるテスト（既存のテストの名前、または本フェーズで加えるテスト）か、確かめる必要がなくなった理由（削除するコードだけの振る舞い）を対応づける（設計書 3.10.2「確かめ方」）。設計書 3.10.2 の表にない対応先が要る検査は、ステップ 2-2 に加える。
    -   **対象:** 本計画（§5.2）。**完了:** 削除・改めるすべてのテストの検査に、対応先か理由がある。
-   [ ] **ステップ 2-2**: 設計書 3.10.2 の「加えるテスト」の表の 6 つの振る舞いと、ステップ 2-1 で見つかった振る舞いを、ポップアップの経路で確かめる検査を `launch.test.ts` に加える。既存のテストにポップアップの側の検査を加えてもよい。たとえば「sends the same body and classification through both paths」には、ポップアップの本文にリテラルの期待値を当てる検査を加える。「token never leaks」は、ポップアップの起動・送信・送らない理由のログ（`PopupLaunchDeps.log` と `PopupSendDeps.log` の両方）を記録して、検査の対象にする。送らない理由のログは、トークンを読み込んだうえで送らない場合（たとえば `port-invalid` で、トークンが `identifiableToken` の場合）のログを記録して確かめる。設計書 3.10.2 の表のほかに、既に次の 2 つが分かっている。ポップアップの起動のログが収集の結果（`outcome`）を持つこと（メニューの経路では `launch.test.ts:435` が確かめていた）と、送らない理由のログがラベル `yt2column blocked` の下に `reason` を持つこと（`:803-807`）である。`body-too-large` の確かめは、既存の `popupBlocks` の表（`:1001`）に行を加えて行う。この段階ではメニューの経路のコードとテストが残っている。加えた検査は、現在のコードで通ることを確かめる。
    -   **対象:** `extension/test/launch.test.ts`。**完了:** `make ext-test` が通る。
-   [ ] **ステップ 2-3**: `popup send` の「shows the sending notice and a disabled button while sending」（`launch.test.ts:922`）に、送信中の表示が 2 つの文（`Do not close this window until the result appears.`・`If you close it, check Slack before sending again.`）を含むことを、`sendingMessage` を使わずに文字列の全体で確かめる検査を加える（AC-08、設計書 3.4）。
    -   **対象:** `extension/test/launch.test.ts`。**完了:** `make ext-test` が通る。
-   [ ] **ステップ 2-4**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は、ステップ 2-2・2-3 で加えた検査のそれぞれである（§5.2 の対応先として加えた検査を含む）。たとえば次のとおりである。`sendingMessage` の 2 つの文の一方を変える（ステップ 2-3 の検査）。`runPopupSend` が `prepareSend` に渡す値の `title` をタブのタイトルに替える。`runPopupSend` が `prepareSend` に `result.metadata.values` ではなく `{}` を渡す（メタデータによる `body-too-large` の行）。読み取りの失敗でもメタデータを省略しない（メタデータなしの本文の検査）。`logSend` の失敗を info に記録する。`logBlocked` の振り分けを逆にする。`logBlocked` から `reason` を落とす。起動のログから `outcome` を落とす。ポップアップの起動・送信・送らない理由のログのそれぞれにトークンを入れる（3 か所を 1 つずつ）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと加えた検査が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、§5.2 がすべての検査に対応先か理由を持つ。

### フェーズ 3: 入口と経路の削除

**対象ファイル**
-   削除: `extension/src/background.ts`、`extension/src/result.ts`、`extension/static/result.html`
-   変更: `extension/src/launch.ts`、`extension/src/browser/chromeDeps.ts`、`extension/static/manifest.json`、`extension/scripts/check-dist.ts`、`docs/dev/security.md`、`extension/test/launch.test.ts`、`extension/test/chromeDeps.test.ts`、`extension/test/manifest.test.ts`、`extension/test/checkDist.test.ts`、`extension/test/repository.test.ts`

**タスク**
-   [ ] **ステップ 3-1**: 入口のモジュールとページを削除する。
    -   [ ] `extension/src/background.ts` を削除する。
    -   [ ] `extension/src/result.ts` を削除する。
    -   [ ] `extension/static/result.html` を削除する。
-   [ ] **ステップ 3-2**: `launch.ts` から、§1.3「削除するファイルと、メニューの経路の処理」に挙げた `launch.ts` の削除の対象を削除する。使われなくなった import を除く。`runPopupSend` の `renderResultView` の呼び出しは、フェーズ 4 まで残す。
-   [ ] **ステップ 3-3**: `chromeDeps.ts` から、`StorageChangeEvents`・`StorageChangeListener`・`WindowsApi`・`ActionApi`・`createResultViewStore`・`createResultWindowOpener`・`Badge`・`createBadge` を削除し、`ResultViewStore` の import を除く（設計書 3.1）。`schedule` はフェーズ 4 で削除する。
-   [ ] **ステップ 3-4**: `static/manifest.json` から `background` のメンバを削除し、`permissions` を `["activeTab", "scripting", "storage"]` にする（設計書 3.3.1）。同じステップで、`docs/dev/security.md` §8.1 の「要求する権限」（`:102`）を設計書 3.9.2 の 1 つ目の項目のとおりに改める（設計書 8 章の優先順位 3）。`TestExtensionDocumentation` の権限の一致の検査（`extension_docs_test.go:86-115`）は manifest と §8.1 の一覧を両方向で比べるので、2 つを同じステップで改める。
-   [ ] **ステップ 3-5**: `scripts/check-dist.ts` を設計書 3.3.2 のとおりに改める。`referenceViolations` の `background.service_worker` の項目と、manifest の型の `background` のメンバを削除する。`static/manifest.json` と `dist/manifest.json` が両方あるときに、2 つのバイト列が同じでなければ違反を報告する検査を加える。どちらかがない場合は、この検査は何も報告しない。コメントを次のように改める。
    -   冒頭の 3 行目を、`// imports, with the manifest and HTML references resolving inside dist/.` から `// imports, with the manifest and HTML references resolving inside dist/ and` と `// dist/manifest.json a byte-for-byte copy of static/manifest.json.` の 2 行に改める。
    -   `referenceViolations` のコメントの `The manifest points at the service worker, the popup page and the options` と `page; an HTML page points at its scripts and stylesheets.` の 2 行を、`The manifest points at the popup page and the options page; an HTML page` と `points at its scripts and stylesheets.` に改める。
-   [ ] **ステップ 3-6**: `launch.test.ts` から、§1.3 の表で「削除」としたテストを削除する。
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
-   [ ] **ステップ 3-7**: `launch.test.ts` の、§1.3 の表で「改める」としたテストから、メニューの経路の部分を削除する。ポップアップの経路の検査は期待値を変えずに残す（設計書 3.10.2「改めるテスト」）。テストの名前は §2.1 の表のとおりに改める。
    -   [ ] collects the same input through the menu and popup paths（`:417`）
    -   [ ] sends the same body and classification through both paths（`:442`）
    -   [ ] reads the metadata once, from the collected tab and URL（`:515`）
    -   [ ] does not read the metadata for a ${name} collection（`:548`）
    -   [ ] sends without metadata and logs an error when ${name}（`:599`）
    -   [ ] treats a missing tab as collection-failed in both paths（`:635`）
    -   [ ] keeps untrusted text out of the fixed log labels（`:678`）
    -   [ ] `video metadata log` の補助 `metadataLogs` と、それを使う 2 つのテスト（`:1076`・`:1088`）
    -   [ ] keeps the token out of the logs, the store and the display（`:1318`。名前は §2.1 の表のとおりに改める）
-   [ ] **ステップ 3-8**: `chromeDeps.test.ts` から次を削除する。
    -   [ ] 「result view store」の describe とその 4 つのテスト（`:437`・`:448`・`:460`・`:474`）
    -   [ ] 補助の `EventRecorder`・`recordingEvents`・`resultView` と、使われなくなった import
-   [ ] **ステップ 3-9**: `manifest.test.ts` を設計書 3.3.1 のとおりに改める。「permissions」は 3 つの権限と完全に一致することを確かめ、名前を §2.1 の表のとおりに改める。「undeclared keys」の集合から `background` を除く。
    -   [ ] 「loads the service worker as a module」（`:87`）を削除する。
-   [ ] **ステップ 3-10**: `checkDist.test.ts` を設計書 3.3.2 のとおりに改める。`writeManifest` を §2.1 の表のとおりにし、`background` のメンバを書かないようにする。「accepts a manifest whose references exist」（`:137`）から `background.ts`・`background.js` を除く。manifest の複製の同一性の検査のテストを加える。`dist/manifest.json` の空白だけを変えたときに同一性の違反が 1 つだけ報告されること（バイト列で比べること）と、`static/manifest.json` だけ、または `dist/manifest.json` だけがある場合に、既存の `missing`・`not built from` の違反だけが報告され、同一性の違反が加わらないことを確かめる。
    -   [ ] `missingReferences` から `name: "service worker"` の行を削除し、残る行から `serviceWorker` のフィールドを除く。
-   [ ] **ステップ 3-11**: `repository.test.ts:60` の入力を §2.1 の表のとおりに改める。
-   [ ] **ステップ 3-12**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は次のとおりである。`static/manifest.json` に `contextMenus` を戻す（`manifest.test.ts` の「permissions」）。`background` を戻す（「undeclared keys」）。§8.1 の一覧に `contextMenus` を戻す（`TestExtensionDocumentation` の権限の一致）。`check-dist.ts` の同一性の検査を外す。どちらかのファイルがない場合にも同一性の違反を報告させる（いずれもステップ 3-10 で加えたテストで確かめる）。AC-02 の集合の比較は、既存の `checkDist.test.ts` の「rejects an extra file」（`:87`）が確かめるので、新しく壊す対象にしない。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと名前を挙げた検査が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** `make ext-check` → `make test` → `make lint` が通る（`security.md` を変えたので `make ext-test` を含む `make ext-check` が要る）。`make ext-build` の後の `extension/dist/` に `background.js`・`result.js`・`result.html` がない。

### フェーズ 4: 共有コードの整理

**対象ファイル**
-   削除: `extension/src/core/resultView.ts`、`extension/test/resultView.test.ts`
-   変更: `extension/src/ui/renderSend.ts`、`extension/src/launch.ts`、`extension/src/core/summary.ts`、`extension/src/core/response.ts`、`extension/src/core/sendMessages.ts`、`extension/src/core/types.ts`、`extension/src/browser/chromeDeps.ts`、`extension/static/style.css`、`extension/test/renderSend.test.ts`、`extension/test/summary.test.ts`、`extension/test/sendMessages.test.ts`

**タスク**
-   [ ] **ステップ 4-1**: `renderResultView` を `renderBlockReason` に置き換える（設計書 3.2.2）。表示は現在の `blocked` の場合と同じにする。`summaryNode` を削除し、`CollectedSummary`・`ResultView`・`renderSummary` の import を除く。`runPopupSend`（`launch.ts:377`）の呼び出しを `renderBlockReason(container, prepared.reason)` に改める。コメントを次のように改める（設計書 3.2.6）。
    -   `ui/renderSend.ts:1` の `// Renders a send outcome and a result-view value as text-only elements. Every` を `// Renders a send outcome and a block reason as text-only elements. Every` に改める（`:2-3` は変えない）。
    -   `ui/renderSend.ts:55` の `renderResultView` のコメント（`a result view`）は、`renderBlockReason` のコメントとして書き直す（設計書 3.2.2 の宣言のコメント）。
    -   `ui/renderSend.ts:11` の `/** The "sending" notice; the popup and the result window both show it. */` を `/** The "sending" notice the popup shows while a send is in flight. */` に改める。
-   [ ] **ステップ 4-2**: `core/resultView.ts` と `test/resultView.test.ts` を削除する。
    -   [ ] `extension/src/core/resultView.ts` を削除する。
    -   [ ] `extension/test/resultView.test.ts` を削除する。
-   [ ] **ステップ 4-3**: `core/summary.ts` から `parseSummary`・`collectedKeys`・`rejectedKeys`・`isRejectionReason` と、`rejectionReasons`・`RejectionReason` の使われなくなった import を削除する。`hasExactKeys` を `core/response.ts` の export しない関数に移し、`core/response.ts:6` の import を除く（設計書 3.2.5）。`core/response.ts` から `unexpectedChecks` を削除する。`UnexpectedCheck` の型は残す。
-   [ ] **ステップ 4-4**: `core/sendMessages.ts` から `unreadableResultMessage` を削除する。
-   [ ] **ステップ 4-5**: `browser/chromeDeps.ts` から `schedule` を削除する（設計書 3.2.4）。`:15` のコメント `/** The chrome.storage area surface used (session for result views, local for settings). */` を `/** The chrome.storage area surface used (local, for settings). */` に改める。`StorageArea` の型は変えない（設計書 3.2.1）。
-   [ ] **ステップ 4-6**: `static/style.css` から `.send-summary` の規則（`:85-87`）を削除する。
-   [ ] **ステップ 4-7**: コメントを改める（設計書 3.2.6）。
    -   `core/types.ts:27` の `/** Display-only digest of an outcome; what the result window receives. */` を `/** Display-only digest of an outcome; what the popup summary shows. */` に改める。
    -   `core/response.ts:102` の `/** A send result without the cause of "unknown": what is displayed and stored. */` を `/** A send result without the cause of "unknown": what is displayed. */` に改める。
    -   `core/response.ts:123` の `/** Drops the cause of "unknown"; the only way a SendResult becomes a stored or displayed value. */` を `/** Drops the cause of "unknown"; the only way a SendResult becomes a displayed value. */` に改める。
-   [ ] **ステップ 4-8**: テストを改める。
    -   [ ] `summary.test.ts`: 「parseSummary」を削除する前に、「summarize」の describe に、収集した要約がちょうど 9 つのキーを持つことと、`cause` を持つ拒否（すべての拒否の理由）が `kind`・`reason` だけの値になることを、値の全体で確かめる検査を加える（§1.3）。
    -   [ ] `renderSend.test.ts`: describe を §2.1 の表のとおりにし、`blocked` の行（`:105`）を `renderBlockReason` を呼ぶ形に改める。
    -   [ ] `renderSend.test.ts`: 「shows the sending notice with the close guidance and the summary」（`:113`）を削除する（検査はステップ 2-3 が引き継ぐ）。
    -   [ ] `renderSend.test.ts`: 「shows the sent outcome and its request id alongside the summary」（`:125`）を削除する。
    -   [ ] `renderSend.test.ts`: 「displays markup in the summary as text, not elements」（`:136`）を削除する。使われなくなった `collected`・`malicious`・`sendingMessage` などの import と定数を除く。
    -   [ ] `summary.test.ts`: 「parseSummary」の describe と、その 9 つのテスト（`:106`・`:111`・`:123`・`:129`・`:133`・`:141`・`:157`・`:163`・`:170`）を削除し、使われなくなった import（`parseSummary`・`rejectionReasons`・`RejectionReason`）を除く。
    -   [ ] `sendMessages.test.ts`: 「tells the user to check Slack when a result cannot be displayed」（`:119`）を削除し、`unreadableResultMessage` の import を除く。
-   [ ] **ステップ 4-9**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は次のとおりである。`renderBlockReason` が `blockMessage` 以外の文言を描く（`renderSend.test.ts` の `renderBlockReason`）。`runPopupSend` が `renderBlockReason` を呼ばない（`launch.test.ts` の `shows the ${reason} reason and no button without sending`）。`hasExactKeys` が余分なキーを許す（`response.test.ts` の member-set の検査）。`summarize` が収集した要約にキーを 1 つ加える、拒否の `cause` を残す（ステップ 4-8 で加えた `summarize` の検査）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと名前を挙げた検査が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** `make ext-check` → `make test` → `make lint` が通る。

### フェーズ 5: lint の規則と文書

**対象ファイル**
-   変更: `extension/eslint.config.js`、`extension/test/lintRules.test.ts`、`README.md`、`docs/dev/security.md`、`cmd/yt2column/extension_docs_test.go`

**タスク**
-   [ ] **ステップ 5-1**: `eslint.config.js` の `restrictedSyntax` に、`contextMenus`・`session`・`windows`・`setBadgeText` という名前のメンバの参照とキーを禁止する規則を加える。形は `chrome.storage.sync` の禁止と同じ 4 つの参照の形にする（設計書 3.5.2）。理由のコメントと `message` は英語で書く。`ImportExpression` の項目のコメント `// A Manifest V3 service worker rejects import(); dist/ is also` と `// checked for it after the build.` を、`// check-dist.ts follows only static imports when it checks that every module`、`// dist/ loads is in dist/, so import() is not allowed; dist/ is also checked`、`// for it after the build.` の 3 行に改める（設計書 3.8）。規則の `selector`・`message` は変えない。
-   [ ] **ステップ 5-2**: `lintRules.test.ts` に、加えた規則の describe を加える。4 つの名前のそれぞれについて、4 つの参照の形が `no-restricted-syntax` を報告することを、既存の `expectViolation` で確かめる。型の位置の参照（`chrome.windows.Window` など）が報告されないことも確かめる。
-   [ ] **ステップ 5-3**: `README.md` の「Browser extension」節を設計書 3.9.1 のとおりに改める。2 つの起動の段落をポップアップだけの起動の段落にし、`:594` の文を削除し、`:615-617` から結果のウィンドウを除き、ログの見方と更新の手順を書き改める。更新の手順は `make ext-build` の語を、ログの見方は `Inspect popup` の語を含める（ステップ 5-5 が固定する）。節の中で `context menu`・`choose the extension's item`・`result window`・`service worker` を使わない（大文字と小文字を区別しない）。
-   [ ] **ステップ 5-4**: `docs/dev/security.md` §8.1 の `:104` を設計書 3.9.2 の 2 つ目の項目のとおりに改める（`:102` はステップ 3-4 で改めた）。
-   [ ] **ステップ 5-5**: `cmd/yt2column/extension_docs_test.go` を設計書 3.9.3 のとおりに改める。
    -   `TestExtensionSendDocumentation` の「README describes sending from the browser extension」で、求める語句から `Context menu`・`Toolbar icon` を除き、`Send to yt2column` を加える。ステップ 5-3 で加える更新の手順とログの見方の案内も、`make ext-build` と `Inspect popup` を求める語句に加えて固定する。節を小文字にした文字列に、ステップ 5-3 の 4 つの語句が含まれないことを確かめる検査を加える。
    -   `TestExtensionDocumentation` に、§8.1 に `結果のウィンドウ`・`chrome.storage.session`・`contextMenus` が含まれないことと、「要求する権限」の項目が `storage` の用途として `chrome.storage.local` を挙げることを確かめるサブテストを加える。
-   [ ] **ステップ 5-6**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は次のとおりである。`src/` の 1 つのファイルに `chrome.contextMenus`・`chrome.storage.session`・`chrome.windows`・`chrome.action.setBadgeText` をそれぞれ書く（`make ext-lint`）。加えた規則から 1 つの名前（たとえば `session`）を外し、その名前の行だけが `lintRules.test.ts` で失敗することを確かめる。テンプレートの参照の形の項目を外し、その形の行が失敗することを確かめる。README に `Context menu` の項目を戻す。`service worker` の語を戻す。`Send to yt2column`・`make ext-build`・`Inspect popup` をそれぞれ消す（`TestExtensionSendDocumentation`）。§8.1 の「要求する権限」から `chrome.storage.local` を消す（加えたサブテスト）。§8.1 に `contextMenus` の権限を戻す（`TestExtensionDocumentation` の権限の一致と、加えたサブテスト）。§8.1 に `結果のウィンドウ` の語を戻す（加えたサブテスト）。続けて `make fmt` → `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと名前を挙げた検査が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** `make ext-check` → `make test` → `make lint` が通る。`README.md` と `docs/dev/security.md` を変えたので、`make ext-test`（`metadataDocs.test.ts`・`serverReasons.test.ts`・`manifest.test.ts`）も通す（CLAUDE.md「Development Notes」）。

### フェーズ 6: 整理の確認

**対象ファイル**
-   変更: 本計画（§5.1〜§5.4）。カバレッジの比較で到達しなくなったものがあれば `extension/test/launch.test.ts` ほか

**タスク**
-   [ ] **ステップ 6-1**: 設計書 3.2.7 の確認を行う。本タスクが本番の参照元を変えたモジュール（`launch.ts`・`browser/chromeDeps.ts`・`ui/renderSend.ts`・`core/summary.ts`・`core/response.ts`・`core/sendMessages.ts`・`core/prepare.ts`・`core/types.ts`・`send.ts`）の export について、本番の参照元とテストの利用を確かめる。結果が設計書 3.2.1 の表と一致することを確かめ、§5.1 の「設計書 3.2.7」の行に記録する。一致しない export があれば、設計書 3.2.1 の基準で扱いを決めてから進める。
-   [ ] **ステップ 6-2**: AC-05a の静的な確認を行う（設計書 3.5.1）。`extension/src/` で次の 3 つを確かめ、§5.1 の「AC-05a」の行に記録する。`sendArticle` を呼ぶ箇所が `runPopupSend` の `click` の処理の中の 1 か所だけであること。`createSendDeps` を呼ぶ入口のモジュールが `popup.ts` だけであること。ネットワークを使う API（`fetch`・`XMLHttpRequest`・`sendBeacon`・`WebSocket`・`EventSource`）を使う箇所が `createSendDeps` の中の `fetch` の 1 か所だけであること。実行したコマンドとその出力とコミットは §5.4 に記録し、§5.1 の欄からは §5.4 を参照する（表の欄に `|` や改行を入れない）。
-   [ ] **ステップ 6-3**: 削除の後の lcov の報告を、ステップ 1-1 と同じコマンドで取り、設計書 3.11 のとおりに比べる。比べるファイルは、設計書 3.11「行の対応」の 7 つと、2 つの報告の両方にあるほかのファイル（変更のないファイルは行をそのまま対応づける）である。行・分岐・関数の到達を、ソースの差分で対応づけて比べる。node の lcov は、同じソースでも実行ごとに `BRDA` の block の番号と名前のない関数の番号が変わり、到達しなかった分岐の `BRDA` を出さないことがある。このため、`DA` は対応づけた行で、`FN` は名前ではなく対応づけた開始行で、`BRDA` は対応づけた行と、その行の中の分岐の順で比べ、記録のない分岐は到達していないものとして数える。分岐だけが到達しなくなったと出た箇所は、ソースを読んで判断し、§5.3 に記録する。名前を変えた関数（`renderResultView` → `renderBlockReason`）と、移した関数（`hasExactKeys`）は、対応を明示する。削除の前に到達していて、削除の後に到達しなくなった行・分岐・関数があれば、ポップアップの経路のテストを加えて到達させ、§5.2 に加える。結果を §5.3 に記録する。
-   [ ] **ステップ 6-4**: 本タスクで Go のテストを削除していないことを確かめ、§5.1 の「AC-10」の行に記録する。本タスクの最初のコミットの親から HEAD までの Go のファイルの差分に、ファイルの削除とテスト関数の削除がないことを確かめ、実行したコマンドとその出力を §5.4 に記録する。
-   [ ] **ステップ 6-5**: §5.2 のすべての検査に、対応先か理由があることを確かめる。
-   [ ] **ステップ 6-6**: ステップ 6-3 でテストを加えた場合は、加えた検査について、対象を壊して失敗することを確かめ、コミットのメッセージに記す。続けて `make ext-check` → `make test` → `make lint` を通す。

**完了条件:** §5.1 の「設計書 3.2.7」「AC-05a」「AC-09」「AC-10」「設計書 3.10.2」の行が埋まり、§5.2・§5.3 が完成している。`make ext-check` → `make test` → `make lint` が通る。

### フェーズ 7: 手動確認と記録の検査

**対象ファイル**
-   変更: 本計画（§5.1）、`cmd/yt2column/extension_docs_test.go`

**タスク**
-   [ ] **ステップ 7-1**: 設計書 7.2 の手順 1〜6 を、macOS の Chrome と Brave の、作業時点で最新の安定版で行う。結果を §5.1 の「AC-04・AC-05」と「設計書 3.7」の行に記録する。手順 1 で `81cb30d` の作業ツリーに `make ext-install` を実行する場合は、ネットワークを使うので、事前に利用者の承認を得る（CLAUDE.md「Tool Execution Safety」）。
-   [ ] **ステップ 7-2**: 設計書 7.2 の手順 7 を行い、§5.1 の「AC-07」の行に記録する。サーバが LLM API を呼び、Webhook に投稿するので、事前に利用者の承認を得る。`main` のサーバが HTTP で待ち受けている間（0026 の PR-2 のマージの前）に行う。
-   [ ] **ステップ 7-3**: `cmd/yt2column/extension_docs_test.go` に、本計画の §5.1 の表の各欄が埋まっていることを確かめるテスト `TestContextMenuRemovalRecords` を加える。既存の `checkManualCheckTable` を使い、本計画のパスの定数を加える。行の 1 列目の期待値は §5.1 の表の 8 行と同じ順にし、AC の行は `acLabel` で作る（`AC-05a` は `acLabel(5) + "a"`）。`設計書 …` の行は文字列のリテラルで書く。あわせて、§5.2・§5.3・§5.4 に「（実施のときに記入する）」が残っていないことを確かめる。このテストは設計書 7 章にない、本計画が加える記録の検査で、0007・0020 の計画の手動確認の表の検査（`TestExtensionDocumentation`・`TestExtensionSendDocumentation` の「fills every cell of the plan's manual-check table」）と同じ形である。
-   [ ] **ステップ 7-4**: 壊して失敗することを確かめ、コミットのメッセージに記す。対象は、§5.1 の 1 つの欄を空にすること、1 つの行を消すこと、2 つの行を入れ替えること、§5.2〜§5.4 のいずれかに「（実施のときに記入する）」を戻すことである（`TestContextMenuRemovalRecords`）。続けて `make fmt` → `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊すと `TestContextMenuRemovalRecords` が失敗することを確かめ、コミットのメッセージに記した。

**完了条件:** §5.1 のすべての欄が埋まり、`TestContextMenuRemovalRecords` を含む `make test` が通る。

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

フェーズ 2 以降の各フェーズの完了条件には、上の表の条件に加えて、`make ext-check` → `make test` → `make lint` が通ることを含める。`make ext-check` の前に `make ext-install` が要る（`extension/node_modules` がない場合。ネットワークを使う）。PR の分け方は、本計画の承認の後に決める。

### 3.2. 実装順序の根拠

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
| AC-01 | test | `extension/test/manifest.test.ts::permissions > declares exactly the three permissions the design requires`、`::undeclared keys > declares exactly the keys the design lists`（`static/manifest.json`） | 3-4・3-9 |
| AC-01 | static | `make ext-build` の `check-dist.ts` の manifest の複製の同一性の検査（`dist/manifest.json`）と、`extension/test/checkDist.test.ts::checkDist` に加えるそのテスト | 3-5・3-10 |
| AC-02 | static | `make ext-build` の `check-dist.ts` の既存のファイルの集合の比較。`extension/test/checkDist.test.ts::checkDist > rejects an extra file` | 3-1 |
| AC-03 | static | `make ext-lint`（加える `no-restricted-syntax` の規則）と、`extension/test/lintRules.test.ts` に加える describe | 3-1・3-2・3-3・3-6・3-8（使用の削除）、5-1・5-2（規則） |
| AC-04 | test | `extension/test/manifest.test.ts::permissions`（`contextMenus` がない）・`::undeclared keys`（`background` がない） | 3-4・3-9 |
| AC-04 | manual | §5.1 の「AC-04・AC-05」の行（`cmd/yt2column/extension_docs_test.go::TestContextMenuRemovalRecords` が欄の有無を確かめる） | 7-1・7-3 |
| AC-05 | test | AC-04 と同じ manifest のテスト | 3-4・3-9 |
| AC-05 | manual | §5.1 の「AC-04・AC-05」の行（同上） | 7-1・7-3 |
| AC-05a | test | `extension/test/launch.test.ts::popup send > shows a send button and does not send until it is clicked`・`> sends once, disables the button and shows the result`・`> sends only once when the button is clicked repeatedly`、`extension/test/manifest.test.ts::undeclared keys` | 既存・3-9 |
| AC-05a | manual | §5.1 の「AC-05a」の行（送信とネットワークの API の呼び出し箇所の確認） | 6-2・7-3 |
| AC-06 | test | `make ext-test` の、ポップアップの経路の既存のテスト（§4.3）と、フェーズ 2 で加える検査 | 2-2・3-7 |
| AC-06 | manual | §5.2 の対応の表 | 2-1・6-5 |
| AC-07 | test | `extension/test/launch.test.ts::popup send > sends once, disables the button and shows the result`（受付の表示）・`> shows the token-missing reason and no button without sending` | 既存 |
| AC-07 | manual | §5.1 の「AC-07」の行 | 7-2・7-3 |
| AC-08 | test | `extension/test/launch.test.ts::popup send > shows the sending notice and a disabled button while sending`（2 つの文の全体の検査） | 2-3 |
| AC-09 | static | `cmd/yt2column/extension_docs_test.go::TestContextMenuRemovalRecords`（§5.1 の「AC-09」の行が埋まっている） | 6-3・7-3 |
| AC-09 | manual | §5.3 のカバレッジの比較 | 1-1・6-3 |
| AC-10 | static | `cmd/yt2column/extension_docs_test.go::TestContextMenuRemovalRecords`（§5.1 の「AC-10」の行が埋まっている） | 6-4・7-3 |
| AC-11 | static | `cmd/yt2column/extension_docs_test.go::TestExtensionSendDocumentation`（README の求める語句と禁止する語句）、`extension/test/metadataDocs.test.ts::video metadata documentation` | 5-3・5-5 |
| AC-12 | static | `cmd/yt2column/extension_docs_test.go::TestExtensionDocumentation`（manifest の権限と §8.1 の一覧の一致と、加える §8.1 の語句の禁止）、`extension/test/manifest.test.ts::host permissions > is recorded in the extension section of security.md` | 3-4・5-4・5-5 |
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

（実施のときに記入する）

### 5.3. カバレッジの比較の結果

ステップ 1-1 で基準の報告の取り方を記録し、ステップ 6-3 で比較の結果を記録する。記録するものは、実行したコマンド、node の版、比べた 2 つのコミット、ファイルごとの比較の結果、到達しなくなったものとその対応、削除の前から到達していなかったものである。

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

-   [ ] フェーズ 1: カバレッジの基準の取得
-   [ ] フェーズ 2: ポップアップの経路のテストの補強
-   [ ] フェーズ 3: 入口と経路の削除
-   [ ] フェーズ 4: 共有コードの整理
-   [ ] フェーズ 5: lint の規則と文書
-   [ ] フェーズ 6: 整理の確認
-   [ ] フェーズ 7: 手動確認と記録の検査
-   [ ] §4.4 の横断検索を行った
-   [ ] §5 のすべての AC の検証が通った
-   [ ] 本タスクのすべての PR を、0026 の PR-2 を始める前にマージした

## 8. 成功基準 (Success Criteria)

-   **機能の完全性:** 拡張の入口のモジュールが `popup.ts`・`options.ts` の 2 つだけになり、manifest に `background`・`contextMenus` がない。送信はポップアップの「Send to yt2column」のボタンからだけ始まる（AC-01〜AC-05a）。
-   **品質:** ポップアップの経路の既存のテストが期待値を変えずに通る（AC-06）。削除の前に到達していた、残る行・分岐・関数が、削除の後も到達している（AC-09）。`make ext-check`・`make test`・`make lint` が通る（AC-13）。
-   **セキュリティ:** 要求する権限が 3 つになり、削除した API の再導入を lint が禁止する（AC-01・AC-03・AC-12）。共有トークンがポップアップのログと表示に出ないことを、改めた「token never leaks」が確かめる。
-   **文書:** README と `docs/dev/security.md` §8.1 がポップアップだけの構成を述べ、文書の検査がメニューの経路への言及を禁止する（AC-11・AC-12）。§5.1 のすべての欄が埋まっている。

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/mkplan2` で PR の境界を本計画に加え、`/runplan` で実装する。
-   本タスクのすべての PR がマージされた後、0026 の PR-2 を始める。0026 のフェーズ 3 の前に、本タスクの後に残った名前（`schedule` の削除、`ResultView` と `core/resultView.ts` の削除、`renderBlockReason`）と行番号を、0026 の計画（§1.3「0027 との順序」）が確かめる。
-   #330（DOM からの文字起こしの読み取り）は、ポップアップの経路だけを対象にして進める（設計書 9 章）。
