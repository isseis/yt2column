# 実装計画書：同じ動画の記事があるとき、生成し直すか既存の記事を開くかを選べるようにする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-04 |
| Review date | 2026-11-04 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.2 節」などは設計書の節を指す。用語は要件書 6 章と設計書の冒頭（送り方など）に従う。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-08 は、設計書の付録 A がすべて記録している。

既存のコードについての記述は、コミット `edf65e1c`（2026-11-04 の `main`）で確かめた。設計書が行番号を引いたコミット `f495b3ec` から `edf65e1c` までに、`internal/articlecache`・`internal/server`・`extension/src`・`extension/test`・`extension/static`・`README.md` は変わっていない（`git diff --stat f495b3ec edf65e1c -- <これらのパス>` の出力が空）。ただし、本タスクは 0059 の PR-2 のマージの後に始める（1.3 節）。0059 の PR-2 は、`internal/server`・`cmd/yt2column-server` と、`README.md`・`docs/dev/guides/development.md`・`docs/dev/security.md`・`docs/dev/conventions/package_reference.md` を書き換える。したがって、本書と設計書の `file:line` は、実装を始めるときに引き直す（ステップ 1-1）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

ポップアップの主ボタンで照会付きの生成の開始を送り、同じ動画の記事がサーバにあれば、生成を始めずに同じ動画の応答を返す。ポップアップは選択の状態を、ビューアは既存の記事を表示する。何を作るか（キャッシュの `Begin` の判定、本文の `mode`、同じ動画の応答、拡張の送り方、選択の状態、ビューアの URL の `e`、文言、文書）は設計書 1〜6 章で決まっている。本書は、それをどの順序で、どの PR に分け、どのテストで検証するかを定める。

### 1.2. 実装の原則

-   フェーズは設計書 8 章の 5 つの項目に合わせ、1 つのフェーズを 1 つの PR にする（1.3 節）。どの PR の時点でも、拡張・ホスト・サーバをその版でそろえれば、今と同じく記事を生成できる。
-   型・関数の形・応答の形・ログの属性・状態の移り方は設計書の節を参照し、本書では繰り返さない。テストの表の行、確かめの書き方、文言の最終の文は、コードを書くときに決める。
-   加えたテストと書き換えた確かめは、それが確かめる対象を壊して失敗することを実装のときに確かめ、コミットメッセージにそう書く（CLAUDE.md「Testing Strategy」）。何もしないこと（何も送らない、キャッシュを変えない、Webhook を呼ばない）の確かめには、確かめる処理まで届いたことを示す肯定の印（同じ動画の応答が返ったこと、ログの `outcome=same_video` など）を組み合わせる。
-   Go と拡張のコメント・識別子・文字列リテラルは英語で書く（CLAUDE.md「Go Idioms」）。利用者に見せる文は、英語と日本語の両方のメッセージカタログに置く。
-   各フェーズの終わりに、`make fmt`・`make test`・`make lint`・`make deadcode` を通す。拡張を変えるフェーズ（2〜4）は `make ext-check` も通す。README と `docs/dev/guides/development.md` を変えるフェーズは `make ext-test` も通す（CLAUDE.md「Development Notes」）。
-   文書のガードは、CI の `doc-guard` と同じ `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline` で実行する（以下「文書のガード」）。

### 1.3. 計画の際に決めたこと（2026-11-04）

-   **本タスクは 0059 の PR-2（`/articles` の削除）のマージの後に始める（レビュアーと決めた）:** 今の `/articles`（`internal/server/handler.go:145`）と生成の開始（`jobs_api.go:140`）は、同じ `readRequest` と `validateBody`（`request.go:39`・`:62`）を通る。設計書 3.3 節のとおり `validateBody` で `mode` を必須にすると、0059 より先なら `/articles` も `mode` を求めるようになり、要件書 2.3 の「`/articles` の振る舞いは変えない」と食い違う。`development.md` の `request.json` の例（`:69`）と、サーバのテストの補助 `validBody`（`internal/server/test_helpers.go:328`）も、`/articles` とジョブの API の両方で使われている。0059 の PR-2 は同じファイルを大きく書き換えるので、先に入れると衝突の解消も 2 回要る。生成の開始の経路だけで `mode` を検査する分岐を加える案は、0059 の後で消すコードになるので採らない。
-   **PR は設計書 8 章の項目ごとに 5 つにする（レビュアーと決めた）:** PR-1 キャッシュ、PR-2 サーバと拡張のやり取り（Service Worker（以下 SW）を含む）、PR-3 ビューア、PR-4 ポップアップ、PR-5 文書である。項目 2 と 3 を 1 つにまとめる案は、PR-2 が大きくなるので採らない。
-   **ポップアップは PR-4 まで `mode: "plain"` で送る:** PR-2 で拡張は `mode` を送るようになり、SW は 2 つの送り方を扱えるようになる。主ボタンが照会付きで送るのは、選択の状態ができる PR-4 からである。こうすると、PR-2・PR-3 の時点では同じ動画の応答がポップアップに届かず、どの PR の時点でも利用者から見た振る舞いが今と同じか、完成した形になる。
-   **PR-2 の暫定の扱い:** PR-2 で `StartOutcome` に `same-video` が加わる（設計書 3.4.2 節）ので、ビューアとポップアップは、この種類を型の上で扱う必要がある。PR-2 では、どちらも `same-video` を `accepted` と同じに扱う（キーを受け取ったものとしてポーリングする）。ビューアの扱いは、設計書 3.7 節の振る舞いから `e=1` と既存の記事の表示を除いたもので、PR-3 で完成させる。ポップアップの扱いは、PR-2・PR-3 の時点では同じ動画の応答が届かないので実行されない経路であり、PR-4 で選択の状態に置き換える。`StartFailure` から `same-video` を除くこと（設計書 3.6 節）は PR-2 で行う。文言を選ぶ `startResultMessage`（`src/core/sendMessages.ts:88`）は、引数から `same-video` を除く。呼び出し元（`src/ui/renderJob.ts:179`・`:218`）は `StartFailure` だけを渡すので、`same-video` の文言は要らない。
-   **送る前の本文の大きさの確かめは、`mode` の分を含めて測る:** 拡張は今、`JSON.stringify(StartInput)` の UTF-8 のバイト数を、サーバと同じ上限（4 MiB）と比べる（`extension/src/core/prepare.ts:26`・`:93`）。PR-2 から送る本文は `StartInput` に `mode` を加えたもの（設計書 3.4.1 節の `StartBody`）なので、今の測り方では、`StartInput` だけの大きさが上限を 17 バイト以内で下回る本文が拡張の確かめを通り、サーバに `body_too_large` で拒否される。そこで、測る値に、長いほうの送り方の `mode` のメンバ（`,"mode":"inquiry"`）のバイト数を加え、どちらの送り方でも、拡張の確かめを通る本文がサーバの上限を超えないようにする。設計書 3.4.1 節の「`StartInput` は今の形のまま」は変えない（測り方だけを変える）。
-   **引き継ぎの `answer` は PR-2 で加える:** `answer` を送るのは SW、読むのはビューアである（設計書 3.5 節）。メッセージの形と検査（`parseViewerChannelMessage`）と SW が送ることは PR-2 で、ビューアが `answer` で `e=1` を加えることは PR-3 で行う。PR-2 のビューアは `answer` を読んでも振る舞いを変えない。
-   **設計書 8 章の項目 5（文書）の中身の一部を、その振る舞いを変える PR に入れる:** フェーズの順は変えず、項目 5 の文書のうち次のものを先のフェーズに置く。フェーズ 5 は、残りの文書を扱う。
    -   `docs/dev/guides/development.md` の「ジョブの API」（本文の `mode`、同じ動画の応答、照会付きの生成の開始の振る舞い）は PR-2 に入れる。PR-2 から `mode` のない本文は拒否されるので、PR-5 まで待つと、その間 `request.json` の例が拒否される本文になるためである。
    -   `README.md`（と `/mktrans` で `README.en.md`）は PR-4 に入れる。`extension/test/metadataDocs.test.ts:56` は、カタログの `popup.primary` のすべての名前を README が引用していることを確かめる。選択の状態の主ボタンの名前も、ほかの主ボタンの名前と同じくこのカタログの値に置く（`render.ts:397` の `PrimaryAction` からカタログの名前への対応）。そのため、PR-4 でカタログに加えたとき、同じ PR の README がその名前を引用していなければ、このテストが失敗する。
    -   `docs/translation_glossary.md` の新しい用語（要件書 F-005 が挙げる 8 つ）は PR-4 に入れ、`/mktrans` の前に加える。`/mktrans` は用語集を読んで訳し、用語集にない語を加える（`.claude/commands/mktrans.md`「Load Glossary」「Update Glossary」）ためである。
    -   `docs/dev/conventions/package_reference.md` は、そのパッケージを変える PR に入れる（`internal/articlecache` は PR-1、`internal/server` は PR-2）。
    -   PR-5 には、`docs/dev/security.md` §8 と、用語集の既存の用語（主ボタン・送信の時刻・ビューアの URL）の定義の改訂が残る。`security.md` の記述が PR-2 から PR-5 まで遅れることは受け入れる。利用者は開発者 1 人で、遅れる間の害は文書が古いことにとどまる。
-   **AC の検証に `diff` と `_checks/` は使わない:** 本タスクの AC は、変更の前後の関係（何かが残る、変わらない）を述べるものがない。前の版と変わらないことを求める AC-09・AC-22 などは、今の振る舞いを確かめる既存のテストがそのまま通ることと、照会なしと照会付きを並べたテストで確かめる。

### 1.4. 既存コード調査結果

**キャッシュ（`internal/articlecache`）**

-   `Begin(key Key, acquire func() bool) (BeginOutcome, *Run, *Eviction)` が `cache.go:82` にある。記録の世代の番号（`record.gen`、`record.go:140`）は、`Begin` が作るときとやり直すときに `nextGen` の値を入れる（`cache.go:110-118`）。設計書 1.2 節のとおり、`nextGen` は投稿の開始でも増える。
-   `KeyInput`（`key.go:31-37`）を作るのは、製品のコードでは `validateBody`（`internal/server/request.go:118`）だけで、テストでは `key_test.go:14`・`:55` と `internal/server/request_test.go:346`・`:355` である（`git grep -n "KeyInput{" -- '*.go'` の出力が 5 行）。
-   `cache_test.go` の `Begin` の呼び出しは 14 か所（`:74`・`:133`・`:148`・`:160`・`:170`・`:203`・`:231`・`:264`・`:291`・`:299`・`:317`・`:347`・`:445`・`:633`）で、テストのキーは `testKey(n)`（`:49-53`、`baseKeyInput` の字幕だけを変える）が作る。`testKey` を `Subject` を返す形にするか、`Subject` を作る補助を並べるかは実装で決める。どちらでも、`Subject` は `Keyer.Subject` からしか作らない（設計書 3.2 節）。
-   同時の `Begin` のテストは `TestBeginConcurrentAcquiresOnce`（`cache_test.go:611`、16 の goroutine をそろえて同じキーで呼ぶ）にある。同じ動画の同時の照会のテストは、これと同じ組み立てにする（設計書 7.1 節）。
-   `DumpRecordsForTest`（`test_helpers.go:24`）は記録のすべての項目を反射でたどるので、`record.videoID` を加えると、その出力にも動画 ID が出る。このテストの補助を使って、記録に字幕が残らないことを確かめているテストは、動画 ID が出ることに影響されない（動画 ID は字幕の目印を含まない）。この補助は記録の map を並べ替えずにたどる（`:28`）ので、記録が 2 つ以上あるときの前後の比べ方には使わない。
-   `Begin` の候補は記録の map をたどって探す（設計書 3.2 節）ので、たどる順は実行ごとに変わる。

**サーバ（`internal/server`）**

-   `startHandler`（`jobs_api.go:67-155`）は、`s.keyer.Key(input.key)`（`:117`）と `s.cache.Begin(key, s.acquireSlot)`（`:118`）を呼び、生成の開始のログの行（`:154`）に `requestId`・`key`・`videoId`・`outcome` を書く。`BeginFull` は `default` の分岐で扱う（`:144-148`）。
-   製品のコードで `Begin` を呼ぶのは `jobs_api.go:118` だけである。テストで `Begin` を呼ぶのは、`generation_test.go:49`、`handler_test.go:131`・`:138`、`jobs_api_test.go:115`、`publish_test.go:232`・`:648`、`test_helpers.go:139`（`beginGeneration`）である（`git grep -n "\.Begin(" -- '*.go'` の `internal/server` の行）。`ts.keyer.Key(...)` の 10 か所のうち、結果を `Begin` に渡す 5 か所（`generation_test.go:48`、`jobs_api_test.go:114`、`publish_test.go:230`・`:647`、`test_helpers.go:138`）は `ts.keyer.Subject(...)` に替える。`handler_test.go:131`・`:138` は `publishTestKey`（`test_helpers.go:348`）のキーを `Begin` に渡すので、同じ入力の `Subject` を返す補助を使う。キーだけを求めるほかの 5 か所は、`Key` が残るので変えなくてよい。
-   生成の開始の本文を作るテストの補助は `validBody`（`test_helpers.go:226`）・`bodyWith`（`jobs_api_test.go:27`）と、`cmd/yt2column-server/test_helpers.go:401` の `requestBody`（`server_test.go` と `relay_e2e_test.go:313` が使う）で、`json.Marshal` で本文を作るテスト（`status_test.go:179` の `TestResponsesOmitInputs` など）もある。受理を期待するテストは、`mode` を加え忘れると失敗する。一方、`invalid_body` を期待する行（`request_test.go:44-65` の `invalidBodyCases`、`TestStartRejectsBody` の任意のメンバの拒否の行など）は、本文の形の誤りがどれも同じ `invalid_body` になる（`request.go:60-90`）ので、`mode` がなくても通ってしまい、行が名指す検査を確かめなくなる。そのため、これらの行にも正しい `mode` を加える（ステップ 2-1）。
-   `TestStartSameInputGeneratesOnce`（`jobs_api_test.go:141`）は、ログの `outcome=started` と `outcome=existing` の行を数える（`:161`）。`mode` の属性が加わっても、数え方は変わらない。
-   `TestStartKeyEquivalence`（`jobs_api_test.go:170`）は、検査を通った値に届かない違いだけの本文が同じキーになることを確かめる。`mode` だけが違う本文が同じキーになること（設計書 7.1 節）は、この表の行として加える。
-   `TestStartConcurrentSameKey`（`jobs_api_test.go:329`）は、読み込みの枠の数（`maxReading`）と同じ数の goroutine から同時に送る。同じ動画の同時の照会のサーバのテストは、これと同じ組み立てにする（設計書 7.1 節、H-07）。
-   panic の確かめは `TestStartHandlerPanicCleanup`（`jobs_api_test.go:560`）が `beforeGenerationJob`（`server.go:109`）で行う。同じ動画の応答の経路は `Begin` の後に生成を始めないので、`beforeGenerationJob` を通らない。`startHandler` の panic の回復は、関数の先頭の `defer`（`jobs_api.go:70-90`）で、どの分岐にも当たる。
-   処理中の記録を保つテストの仕組みは `holdAtDraft`（`TestStartConcurrentSameKey` などが使う）にある。
-   応答とログの漏れの確かめは、`TestResponsesOmitInputs`（`status_test.go:173`）が目印の値で行う。同じ動画の応答とその行のログを、この確かめに加える（設計書 7.3 節）。
-   本文の検査のテストは `request_test.go` にあり、`TestStartRejectsBody`（`:100`）が拒否の表（`invalidBodyCases` などから行を作り、`requireStartUntouched`（`:29`）で記録・Writer・枠が変わらないことを確かめる）、`TestValidateBodyKeyInput`（`:333`）が `KeyInput` の値を確かめる。

**拡張（`extension/`）**

-   `startMessage`（`src/core/hostMessage.ts:14`）を呼ぶのは `src/startCoordinator.ts:359` の `runAcceptedStart` だけで、`test/entryPoints.test.ts:260` がそれを確かめる。引数が増えても、このテストは変わらない。
-   `classifyStart`（`src/core/response.ts:559`）は `classify`（`:537`）の上にある。拒否の理由の表のテスト（`test/response.test.ts:235` の `reasonTable`）は操作ごとの分類の関数を並べるので、`classifyStartInquiry` の行を加える。
-   `StartOutcome` の種類で分ける箇所は、`src/launch.ts:395`、`src/viewerApp.ts:286`、`src/startCoordinator.ts:272`・`:363`、`src/core/extensionMessages.ts:341`（`startOutcome` の検査）、`src/core/response.ts` の `toStartOutcome`（`:650`）と `StartLogView`・`startLogView`（`:689`・`:820`）、`src/core/sendMessages.ts:88` の `startResultMessage`（`never` で網羅を確かめる）である（`grep -rn '"accepted"' src` と `StartOutcome` を使う箇所で確かめた）。`same-video` を加えるとき、これらをすべて直す。`src/core/jobView.ts:37` の `StartFailure` も直す。
-   `parseViewerChannelMessage`（`extensionMessages.ts:403`）は、`waiting` と `take-over` を同じ分岐で検査する（`:425-433`）。`take-over` に `answer` を加えるので、分岐を分ける。型のついた `take-over` のメッセージを作るテストは `test/viewerQuery.test.ts:132-137` にもある。この検査と `parseStartRequest` は拒否の理由を返さない（`undefined` か `shape`）ので、`answer` や `mode` を加え忘れた拒否の行は、行が名指す誤りを確かめなくなる。
-   `parseStartRequest`（`extensionMessages.ts:272`）は、メンバの集合を `["type", "input", "viewer", "source"]` で確かめる。テストで `type: "start"` の求めを作るのは `test/extensionMessages.test.ts`・`test/startCoordinator.test.ts`・`test/launch.test.ts` である。
-   `ViewerParams`（`src/core/viewerUrl.ts:24`）は今、`ViewerSeed` に任意の `key` を加えたインターフェースである。設計書 3.7 節のとおり判別共用体にするので、`viewerQuery` と `parseViewerUrl` の呼び出し元（`launch.ts:453`、`startCoordinator.ts:345`、`viewerApp.ts`、`src/ui/renderViewer.ts:18`・`:55`）を直す。
-   経過時間は `renderJob.ts` の `jobNodes`（`:290`）と `drawStateOnChange` の描き直し（`:362-380`）が描く。ビューアは `viewerApp.ts:212` で経過時間を、`:217-230` で毎秒の描き直しを決める。
-   ポップアップの要素は `findPopupElements`（`launch.ts:505`）が `popup.html` の id で探す。`test/popupHtml.test.ts:61` は操作のセクションの子が主ボタンと `#status` であることを確かめるので、副次のボタンを加えるときに直す。
-   README のガード `test/metadataDocs.test.ts:51-62` は、カタログの `popup.primary` の値すべてと、ビューアのボタンなどの名前を README が太字で引用していることを確かめる（1.3 節）。
-   `test/style.test.ts` のポップアップの大きさの確かめは `#primary` だけ（`:552`）で、`:682` はビューアの決まった選択子（`.action-button`・`#video-link`）だけを確かめる。`#secondary` の大きさの確かめは加える必要がある。色の組のコントラストは `:401` が `contrastPairs`（`:251`）で、押せないボタンの色は `:415` が、動きを止める規則は決まった選択子の一覧（`:724` の `moving`）で確かめる。
-   ポップアップを開いただけでは何も送らないことは、`launch.test.ts` の `describe("popup job send")`（`:646`）が確かめている。ポップアップは開くたびに新しい文書で、送信の記録を持たない（設計書 3.6 節「開き直したとき」）。
-   AC-16 の「スクロールせずに見える」ことの手動の確かめには、`extension/scripts/popup-preview.ts` を使う（`0046` の AC-17 の確かめと同じ。`docs/tasks/0046_popup_redesign/03_implementation_plan.md:373`）。このスクリプトはポップアップの状態ごとにページを書くので、`choice` と `choice-ended` を加える。

**文書を読むガード**

-   `cmd/yt2column-server/docs_test.go::TestServerDocsContract` は、`development.md` の curl の例のメソッドとパス（`:106-115`）と拒否の理由の表を確かめる。本タスクはパスも拒否の理由も加えないので、ガードは変えない（設計書 3.9 節）。
-   `extension/test/serverReasons.test.ts` は `development.md` の拒否の理由の表を読む。表は変わらない。

**型による保証の選び方:** `Subject` はフィールドを公開しないので、`internal/articlecache` の外では `Keyer.Subject` からしか作れない（コンパイラが保証する）。パッケージの中では複合リテラルでも作れるが、製品のコードで `Subject` を作るのは `key.go` の `Keyer.Subject` だけにする。このパッケージの中の作り方は、ガードのテストを設けず、PR のレビューで確かめる。`Subject` で振る舞いを分けることはなく、誤って作っても起きるのはキーと動画 ID の組の食い違いにとどまる。設計書 3.2 節は、空の動画 ID をどの記録とも一致させないので、零値の `Subject` が無関係な記事を返すこともない。リーフパッケージに移す案は、`Key` と `Keyer` を同じパッケージに置く今の構成を崩すので採らない。`StartMode` と `BeginOutcome` は `switch` で分け、`StartMode` の零値は照会付き（設計書 3.2 節）、`BeginOutcome` の零値は `BeginFull` のままである。

### 1.5. 名前の変更の一覧

本タスクは既存の名前を変えない。加える名前は設計書 3 章のとおりである。テストの名前は 7 章の表のとおりとし、実装で変えたときは、この表に行を加えて 7 章を合わせる。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: キャッシュ（設計書 8 章の 1）

#### ステップ 1-1: 前提の確かめ

**変更するファイル:** 本書（行番号を引き直したときだけ）

- [x] 0059 の PR-2 が `main` にマージされていることを確かめる（1.3 節）。マージされていなければ、実装を始めずに止まって報告する。
- [x] 0059 の PR-2 が変えたファイルについて、本書 1.4 節と設計書の `file:line` を引き直す。名前が変わったもの、なくなったものがあれば、本書を直す（設計書の行番号だけの食い違いは、設計書を直さない。名前や振る舞いの食い違いは、設計書を先に直す。`requirements_process.md`「Editing an approved document」）。

**完了の条件:** 本書の 1.4 節の名前が、その時点の `main` にある。

#### ステップ 1-2: `Subject` と `KeyInput.VideoID`

**変更するファイル:** `internal/articlecache/key.go`、`key_test.go`、`internal/server/request.go`、`request_test.go`

- [x] `KeyInput` に `VideoID` を加え、`Subject` と `Keyer.Subject`・`Subject.Key` を設計書 3.2 節のとおりに加える。`VideoID` はキーの計算に入れない。
- [x] `Keyer.Subject` のテストを `key_test.go` に加える。キーが同じ入力の `Keyer.Key` と等しいこと、動画 ID だけが違う 2 つの入力のキーが等しいこと（動画 ID がキーに入らない）。`Subject` の動画 ID が `KeyInput.VideoID` であることは、`Begin` のテスト（ステップ 1-3）で確かめる。
- [x] `validateBody` が `KeyInput.VideoID` を、`VideoURL` と同じ `transcript.ValidateVideoURL` の呼び出しの戻り値から埋める（設計書 3.2 節）。`TestValidateBodyKeyInput` に、`youtu.be` の書き方の URL で `VideoID` が動画 ID になることの確かめを加える。

**完了の条件:** `go test -tags test ./internal/articlecache/ ./internal/server/` が通る。

#### ステップ 1-3: `Begin` の新しい形と同じ動画の判定

**変更するファイル:** `internal/articlecache/cache.go`、`record.go`、`cache_test.go`、`internal/server/jobs_api.go`、`generation_test.go`、`handler_test.go`、`jobs_api_test.go`、`publish_test.go`、`test_helpers.go`

- [x] `StartMode`・`BeginSameVideo`・`BeginResult` を加え、`Begin` を `Begin(subject Subject, mode StartMode, acquire func() bool) BeginResult` にする。排他制御の中の判定の順は設計書 3.2 節の表のとおりで、同じ動画の記事・最新・空の動画 ID の扱いも同じ節のとおりである。
- [x] `record` に `videoID` を加え、作るときとやり直すときに `subject` の動画 ID を入れる。`Snapshot` には出さない。
- [x] `startHandler` を、`s.keyer.Subject(input.key)` と `articlecache.StartPlain` で `Begin` を呼ぶ形にする。このフェーズではサーバの振る舞いを変えないので、`BeginSameVideo` は届かない。`switch` には `BeginSameVideo` の分岐を設けて `internal_error` で拒否し、ログに書く（設計書 4.2 節の「`plain` の送信で `BeginSameVideo` が返るのは `Begin` が誤っている場合」の扱いを、このフェーズで入れる）。この分岐は本物の `Cache` からは届かない守りの分岐なので、テストを書かない。そのことを分岐のコメントに書く。
- [x] `cache_test.go` の 14 か所と `internal/server` のテストの `Begin` の呼び出し（1.4 節）を新しい形に書き換える。既存の振る舞いの期待は変えない。既存の `Begin` のテストは `StartPlain` で呼ぶ。
- [x] `TestBeginSameVideo` を `cache_test.go` に加える。設計書 7.1 節の `articlecache.Begin` の行が挙げることを、表の行で確かめる（順 1〜3、失敗を含めない、今のキーを含めない、世代の番号の最大でやり直した記録を含む、空の動画 ID、記録の数・状態・世代の番号が変わらない、`acquire` を呼ばない、生成の枠がない・捨てられる記録がないとき、`StartMode` の零値）。次を守る。
    -   記録の前後の比べ方は、キーごとの記録の写し（世代の番号を含むすべての項目）で行う（`DumpRecordsForTest` の文字列は記録が 2 つ以上のとき順が定まらない。1.4 節。`Lookup` の `Snapshot` は世代の番号を持たない）。
    -   最新の行は、偽の時計を記録ごとに進め、同じ動画の候補を 3 つ以上にする。
    -   生成の枠がない・捨てられる記録がない行は、同じ状態で `StartPlain` の `Begin` が `BeginNoSlot`・`BeginFull` を返すことも確かめ、その状態が本当に埋まっていることを示す（`TestBeginPlainIgnoresSameVideo` が同じ表の行を `StartPlain` で確かめる）。
    -   今のキーの記録は、順 1（処理中・完了）と失敗の除外のどちらかで候補から外れるので、今のキーを除く条件を別に設けない。
- [x] `TestBeginPlainIgnoresSameVideo` を加え、`StartPlain` では同じ動画の記事があっても今と同じ結果になることを確かめる。
- [x] `TestBeginConcurrentSameVideo` を加える。`TestBeginConcurrentAcquiresOnce` と同じく 16 の goroutine をそろえる。1 回では排他制御を 2 つに分けた誤りをまれにしか捉えないので、この組み立てをテストの中で 200 回繰り返す。各回で同じ動画の違うキーで `StartInquiry` の `Begin` を呼び、`BeginStarted` がちょうど 1 つ、ほかはそのキーの `BeginSameVideo`、`acquire` の呼び出しが 1 回であることを確かめる（設計書 7.1 節）。
- [x] 加えたテストの各行が、その判定（順 2 の位置、失敗の除外、世代の番号の比べ方、空の動画 ID の除外、排他制御の中での判定）を壊すと失敗することを確かめ、コミットメッセージに書く。候補は map の順にたどるので、壊した確かめも下の繰り返しの実行で行う（「最初に見つけたもの」を返すように壊したときは、1 回の実行では偶然に通りうる）。
- [x] 加えたテストの行と `TestBeginConcurrentSameVideo` を `go test -tags test -race -count=20 -v -run 'TestBegin' ./internal/articlecache/` で繰り返し実行し、不安定さが出ないことを確かめる。出力の `--- PASS` の行が 0 でないこと（テストが実行されたこと）も確かめる。このパッケージのテストのファイルはどれも `//go:build test` を持つので、`-tags test` がないと何も実行されない。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。

#### ステップ 1-4: パッケージの説明

**変更するファイル:** `docs/dev/conventions/package_reference.md`

- [x] `internal/articlecache` の行に、記録が動画 ID を持つこと、`Begin` が `Subject` と送り方を受け取り、照会付きで同じ動画の記事があれば何も変えずにそのキーを返すことを加える（設計書 3.9 節）。

**完了の条件:** 文書のガードが通る。

### PR-1 作成ポイント: article cache same-video lookup

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4

**推奨タイトル**: `feat(0058): キャッシュの Begin に同じ動画の記事の判定を加える`

**レビュー観点**: `Begin` の判定の順（今のキー、同じ動画の記事、件数の上限と生成の枠）と、照会付きで同じ動画の記事を返すときにキャッシュを何も変えないこと / 最新の 1 件を世代の番号で選ぶこと（やり直した記録を含む） / 同時の照会のテストが排他制御の中の判定を確かめていること / サーバは `StartPlain` を渡し、振る舞いを変えないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-3 が、排他制御の中で判定と生成の開始を行う並行の処理を変える（隔離すべき並行性の処理）

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: サーバと拡張のやり取り（設計書 8 章の 2）

このフェーズの後、サーバは `mode` のない本文を拒否し、拡張はすべての生成の開始に `mode` を付けて送る。ポップアップは `plain` で送る（1.3 節）。

#### ステップ 2-1: 本文の `mode`、同じ動画の応答、ログ

**変更するファイル:** `internal/server/request.go`、`jobs_api.go`、`response.go`、`request_test.go`、`jobs_api_test.go`、`status_test.go`、`test_helpers.go`、`cmd/yt2column-server/test_helpers.go` と、生成の開始の本文を作るほかのテスト（1.4 節）

- [ ] `validateBody` に `mode` の検査と `startMode` を、`jobInput` に `mode` を加える（設計書 3.1 節・3.3 節）。`mode` を読む位置は設計書 3.3 節のとおりで、`invalid_video_url`・`invalid_transcript` より前である。
- [ ] `startHandler` が `input.mode` を `Begin` に渡し、`BeginSameVideo` で同じ動画の応答を書き、ログに設計書 3.3 節の行を書く。生成の開始のログの行に `mode` の属性を加える。同じ動画の応答を書く関数は `response.go` に置く。
- [ ] 本文を作るテストの補助（`validBody`・`bodyWith`・`requestBody`）は `mode` を持つ本文を作る形にし、既存のテストは `plain` で送る。`json.Marshal` や文字列で本文を作るテストも同じく直す。既存の行の期待する値は変えない。
- [ ] `invalid_body` を期待する既存の行（1.4 節）にも正しい `"mode":"plain"` を加え、各行の誤りが、その行が名指すものだけになるようにする。行の本文を、受理されることを先に確かめた正しい本文から作る形にすると、加え忘れが失敗として出る。
- [ ] `TestStartRejectsBody` に、`mode` がない・文字列でない（`true`・`1`・`null`）・集合にない値（大文字、前後の空白、空の文字列）・重複の行を加え、`invalid_body` で拒否されることを確かめる（設計書 3.1 節、AC-31）。生成を始めず、キャッシュも変わらないことは、この表の拒否の行が確かめている 3 点と同じ確かめ（`requireStartUntouched`）で示す。3 点は、`DumpRecordsForTest` が空であること、Writer が呼ばれないこと、枠が取られないことである。`Begin` を呼ばないことは、この 3 点から導く。
- [ ] `TestStartKeyEquivalence` に、`mode` だけが違う本文が同じキーになる行を加える。表のグループは同じ動画を使うので、`inquiry` の本文はグループの最初に置かない（最初に置くと、同じ動画のほかの記事があれば同じ動画の応答が返りうる）。
- [ ] 照会付きの生成の開始のテストを `jobs_api_test.go` に加える（設計書 7.1 節のサーバの行）。テストの名前と確かめることは次のとおりで、表の行の組み立ては実装で決める。
    -   `TestStartInquirySameVideo`: 同じ動画（`youtu.be` と `watch?v=` の書き方の違い、字幕だけ違う、タイトル・チャンネル名・概要欄だけ違う）の完了の記事と処理中の記事（`holdAtDraft` で処理中に保つ）で、`200` の同じ動画の応答、メンバの集合、キー、LLM の fake と Webhook の fake が呼ばれないこと、ログの行の属性。ログは、その `requestId` を持つ行がちょうど 1 行で、送った `videoUrl` の文字列（`youtu.be` の形のまま）を含まないことも確かめる（AC-01・AC-02・AC-10・AC-34）。
    -   `TestStartInquiryNoSameVideo`: 同じ動画の記録が失敗だけのとき、別の動画の記事だけのときに、生成が始まり受け付けの応答が返ること（AC-03）。
    -   `TestStartInquiryOwnKey`: 今の入力のキーが処理中か完了なら、同じ動画のほかの記事があっても受け付けの応答で今のキーが返り、Writer の呼び出しの数が増えないこと。今の入力のキーが失敗で同じ動画のほかの記事が完了なら、同じ動画の応答が返り、失敗の記録がやり直されないこと（AC-04・AC-05）。
    -   `TestStartInquiryLatest`: 同じ動画の記事が 2 つ以上あるとき、最後に受け付けた記録（失敗の後のやり直しを含む）のキーが返ること（AC-06）。
    -   `TestStartInquiryWhenFull`: 生成の枠に空きがないとき、キャッシュが件数の上限で捨てられる記録がないときにも、同じ動画の応答が返ること。同じ状態で、別の動画の照会付きの生成の開始が `too_many_requests` で拒否されることも確かめ、その状態が本当に埋まっていることを示す（AC-07）。
    -   `TestStartConcurrentSameVideo`: `TestStartConcurrentSameKey` と同じ組み立てで、読み込みの枠の数の照会付きの生成の開始を同時に送り、LLM の fake の呼び出しが 1 回分で、ほかが始まった記録のキーの同じ動画の応答を受け取ること（AC-08）。
    -   `TestStartPlainIgnoresSameVideo`: 照会なしの生成の開始は、同じ動画の記事があっても今の入力の記事を生成すること（AC-09）。
- [ ] `TestStartHandlerPanicCleanup` に、照会付きの生成の開始の 2 つの経路の行を加える。生成を始める経路（今の `beforeGenerationJob`）と、同じ動画の応答を書く経路（テストで panic を起こせる箇所を、実装で 1 つ設ける）である。どちらも `internal_error` で拒否し、後のリクエストに応答することを確かめる（AC-11）。
- [ ] `TestResponsesOmitInputs` に、同じ動画の応答とそのログの行を加える。目印に、送った `videoUrl` の文字列を加える（設計書 7.3 節、AC-10・AC-34）。
- [ ] 加えたテストの行が、それぞれの検査・分岐を壊すと失敗することを確かめ、コミットメッセージに書く。`mode` の検査では、`startMode` が知らない値に `StartPlain, true` を返すようにしたときと、`mode` がない本文を `plain` として受理するようにしたときに、AC-31 の行が失敗することを確かめる。ログでは、`videoUrl` の文字列を属性に加えたときに AC-34 の確かめが失敗することを確かめる。

**完了の条件:** `go test -tags test ./internal/server/ ./cmd/yt2column-server/` が通る。

#### ステップ 2-2: 拡張のホストのメッセージと応答の判定

**変更するファイル:** `extension/src/core/hostMessage.ts`、`src/core/extensionMessages.ts`（`SendMode` の型だけ）、`src/core/response.ts`、`src/core/jobView.ts`、`src/core/sendMessages.ts`、`src/core/prepare.ts`、`test/hostMessage.test.ts`、`test/response.test.ts`、`test/sendMessages.test.ts`、`test/prepare.test.ts`

- [ ] `StartBody` と、送り方を引数に取る `startMessage` を設計書 3.4.1 節のとおりに加える。`SendMode` は設計書 3.5 節のとおり `src/core/extensionMessages.ts` に置く。
- [ ] `StartOutcome` に `same-video` を、`classifyStartInquiry` を設計書 3.4.2 節のとおりに加える。`classifyStart` は変えない。`toStartOutcome` と `StartLogView`・`startLogView` に `same-video` を加え、`startLogView` は `same-video` のキーを書かない（設計書 3.5 節）。
- [ ] `StartFailure` から `same-video` を除き、`startResultMessage` の引数からも除く（1.3 節）。
- [ ] 送る前の本文の大きさの確かめに、長いほうの `mode` のメンバのバイト数を加える（1.3 節）。`prepare.test.ts` の上限の境界の行（`:139-152` 付近）を、その測り方に合わせ、上限ちょうどの本文に `inquiry` の `mode` を加えた本文がサーバの上限を超えないことを確かめる。
- [ ] `hostMessage.test.ts` で、`startMessage` が `input` に送り方の `mode` を加えることを、2 つの値で確かめる（AC-31 の拡張の側）。
- [ ] `response.test.ts` に `classifyStartInquiry` のテストを加える。設計書 3.4.2 節の表の各行、`0050` AC-41 の形の誤りの一覧を同じ動画の応答に当てたもの、空のキー、文字列でないキー、状態コードと `status` の組の誤り（AC-32）。`reasonTable` に `classifyStartInquiry` の行を加える。照会なしの応答に `same_video` が来たら `classifyStart` が想定外の応答にすることの行を加える。
- [ ] `startLogView` の今のテスト（`response.test.ts:1002` 付近）に、`same-video` のキーがログに出ない行を加える。

**完了の条件:** `make ext-typecheck` は、ステップ 2-3 を終えるまで通らなくてよい。`node --test` で `hostMessage.test.ts`・`response.test.ts`・`sendMessages.test.ts`・`prepare.test.ts` が通る。

#### ステップ 2-3: SW の送り方と引き継ぎ、ビューアとポップアップの暫定の扱い

**変更するファイル:** `extension/src/core/extensionMessages.ts`、`src/startCoordinator.ts`、`src/launch.ts`、`src/viewerApp.ts`、`test/extensionMessages.test.ts`、`test/startCoordinator.test.ts`、`test/launch.test.ts`、`test/viewerApp.test.ts`、`test/viewerQuery.test.ts`

- [ ] `SendMode` と `StartRequest.mode`、引き継ぎの `answer` を設計書 3.5 節のとおりに加え、`parseStartRequest` と `parseViewerChannelMessage` で検査する。`startOutcome` の検査（`parseStartReply`・`parseAwaitStartReply`）は `same-video` を受理する。
- [ ] SW は、求めの `mode` を `startMessage` に渡し、`mode` の `switch` で `classifyStartInquiry` か `classifyStart` を選ぶ。`same-video` でも同じキーのビューアの引き継ぎを行い、引き継ぎのメッセージの `answer` に結果の種類を書く。生成の開始の結果のログは、`same-video` を `info` で書く（設計書 3.5 節）。
- [ ] ポップアップは `mode: "plain"` で送る（1.3 節）。`same-video` の結果は `accepted` と同じに扱う（1.3 節の暫定の扱い）。
- [ ] ビューアは `same-video` の結果を `accepted` と同じに扱い、引き継ぎの `answer` を読んでも振る舞いを変えない（1.3 節）。
- [ ] テストの求めとメッセージの組み立て（1.4 節のテストのファイル）に `mode` と `answer` を加える。既存の行の期待する値は変えない。拒否を期待する既存の行（`extensionMessages.test.ts` の求めと引き継ぎのメッセージの誤りの行）にも正しい `mode`・`answer` を加え、各行の誤りが、その行が名指すものだけになるようにする（1.4 節）。
- [ ] `extensionMessages.test.ts` に、`mode` の誤り（ない、集合にない値、文字列でない）の求めの拒否、`answer` の誤り（ない、集合にない値）の引き継ぎのメッセージの拒否、`same-video` の結果の受理を加える（AC-33）。
- [ ] `startCoordinator.test.ts` に次を加える（設計書 7.2 節の SW の行）。
    -   `mode: "inquiry"` の求めで、ホストが `input` の `mode` が `inquiry` の `start` を 1 つだけ受け取ること。`mode: "plain"` の求めで、`plain` の `start` を受け取ること（AC-15・AC-19 の SW の側）。
    -   `mode` の誤った求めで、ホストに何も送らずタブも開かないこと（AC-33）。
    -   同じ動画の応答の後、決めた時間が過ぎてもホストが受け取る `start` が 1 つのままであること（AC-17 の SW の側）。
    -   同じ動画の応答で、同じキーのビューアがあれば引き継ぎが `answer: "same-video"` を持ち、新しいタブを閉じること。受け付けの応答では `answer: "accepted"` を持つこと（AC-26 の SW の側）。
    -   同じ動画の応答の結果のログが `info` で、キーを書かないこと。
- [ ] 加えたテストの行が、それぞれの検査と分岐を壊すと失敗することを確かめ、コミットメッセージに書く。

**完了の条件:** `make ext-check` が通る。

#### ステップ 2-4: 開発者の文書とパッケージの説明

**変更するファイル:** `docs/dev/guides/development.md`、`docs/dev/conventions/package_reference.md`

- [ ] `development.md` の「ジョブの API」に、生成の開始の本文の必須のメンバ `mode`、同じ動画の応答（`200`、`status` が `same_video`）、照会付きの生成の開始の振る舞い（設計書 3.2 節の表）、ログの属性を書き、生成の開始の例の本文（`request.json`）に `mode` を加える（設計書 3.9 節、1.3 節）。拒否の理由は加えない。
- [ ] `development.md` の「ジョブの API のコード」節（要件書 2.2 が対象に挙げる）に、`same_video` や `mode` の値を書くかを決める。この節は状態の取得の応答のコードを並べる節なので、生成の開始の応答の `status` とログの値は「ジョブの API」節に書き、この節は変えないのが今の構成に合う。決めたことと理由を PR の説明に書く。
- [ ] `package_reference.md` の `internal/server` の行に、本文の `mode` と同じ動画の応答を加える。
- [ ] 例の本文が受理されることを確かめる。`development.md` の例の本文を `internal/server` の一時的なテスト（コミットしない）で `validateBody` に通し、受理されることを、使ったコミットとともに 7.1 節に書く。
- [ ] 文書のガードと `make ext-test` を通す。

**完了の条件:** 文書のガードと `make ext-test` が通る。

### PR-2 作成ポイント: inquiry start protocol

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `feat(0058): 生成の開始に送り方を加え、同じ動画の応答を返す`

**レビュー観点**: 本文の `mode` の受理と拒否（補わずに `invalid_body`、キャッシュを変えない）と、同じ動画の応答の形・ログの属性が設計書 3.1 節・3.3 節のとおりであること / 拡張の `classifyStartInquiry` が想定外の応答を見分けること / SW の送り方の選び方と、引き継ぎの `answer` / ポップアップが `plain` で送り、ビューアとポップアップの `same-video` の暫定の扱いが 1.3 節のとおりであること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-1・2-2・2-3 がサーバの本文と拡張の応答・求め・チャネルのメッセージという信頼できない入力の境界を変え、サーバと拡張の振る舞いを同じ PR で同時に変える（mkplan のパネルモードの条件である、セキュリティの境界の変更と、振る舞いの同時の変更に当たる）

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: ビューア（設計書 8 章の 3）

#### ステップ 3-1: ビューアの URL の `e`

**変更するファイル:** `extension/src/core/viewerUrl.ts`、`test/viewerUrl.test.ts` と、`ViewerParams` を使うファイル

- [ ] 名前 `e`、`ViewerParams` の判別共用体、`ViewerUrlError` の `existing` を設計書 3.7 節のとおりに加える。書き出しの順は `k` の後である。`ViewerParams` の形が変わる呼び出し元（1.4 節）を直す。
- [ ] `viewerUrl.test.ts` に、設計書 3.7 節の規則の表の各行（`1` だけ、ほかの値と空の値の拒否、2 回目の拒否、`k` のない `e` の拒否）と、`e` のない今の形の URL が今と同じく読めること、`viewerQuery` と `parseViewerUrl` の往復で `e` が保たれることを加える（AC-27・AC-33）。

**完了の条件:** `make ext-check` が通る。

#### ステップ 3-2: 既存の記事のビューアの表示

**変更するファイル:** `extension/src/viewerApp.ts`、`src/ui/renderViewer.ts`、`src/ui/renderJob.ts`、`static/viewer.html`（要る場合）、`static/viewer.css` などのスタイル、`src/core/i18n/en.ts`・`ja.ts`、`test/viewerApp.test.ts`、`test/renderViewer.test.ts`、`test/renderJob.test.ts`、`test/style.test.ts`、`scripts/popup-preview.ts`、`test/popupPreview.test.ts`

- [ ] ビューアが `same-video` の結果で URL に `k` と `e=1` を加え、引き継ぎの `answer: "same-video"` で、送信をやり直す場合もやり直さない場合も `e=1` を加えることを、設計書 3.7 節の表のとおりにする。`answer: "accepted"` では `e` を変えない。PR-2 の暫定の扱い（1.3 節）はこれで置き換わる。
- [ ] `e=1` のビューアに、既存の記事を表示していることと現在の選択範囲で生成する手順の領域を置き、その要約を `#state-status` に書く（設計書 3.7 節）。文言は英語と日本語のカタログに置く（設計書 3.8 節）。
- [ ] `jobNodes` に経過時間を描くかどうかの値を渡し、既存の記事のビューアでは処理中の経過時間を描かず、毎秒の描き直しもしない。完了の所要時間は今と同じく描く（設計書 3.7 節）。
- [ ] 新しい領域の文字と記号の色を `theme.css` の今の変数から選び、新しい色の組があれば `style.test.ts` の `contrastPairs` に加える（AC-39）。
- [ ] `popup-preview.ts` に、既存の記事のビューアの処理中・完了のページを加え、`popupPreview.test.ts` を合わせる。
- [ ] `viewerApp.test.ts` に次を加える（設計書 7.2 節のビューアの行）。
    -   `same-video` の結果で、URL に `k` と `e=1` が加わり、状態の取得を始め、既存の記事の領域と `#state-status` の要約が出ること。`accepted` の結果では領域が出ず、`e` が加わらないこと（AC-25）。
    -   引き継ぎの `answer: "same-video"` で、ビューアが処理中・完了を表示していた場合（やり直さない）と、失敗を表示していた場合（やり直す）の両方で `e=1` が加わること。送信の時刻とポーリングは今の引き継ぎのとおりであること。`answer: "accepted"` で `e` が変わらないこと（AC-26）。
    -   `e=1` の URL で開き直したビューアが、領域を示し、処理中に経過時間を描かず、毎秒の描き直しを予約しないこと。完了で所要時間を描くこと（AC-27）。
    -   `e` の誤った URL で、リンクが正しくないことを表示し、状態の取得を送らないこと（AC-33）。
- [ ] `renderJob.test.ts` と `renderViewer.test.ts` に、経過時間を描かない指定と、既存の記事の領域の描画を加える。領域の文は、英語と日本語のそれぞれで描き、`catalogs[locale]` の値と一致することを確かめる（AC-40）。`e` のないビューアで経過時間を描く今のテストは、そのまま通ることを確かめる（設計書 3.10 節）。
- [ ] 加えたテストの行が、それぞれの分岐を壊すと失敗することを確かめ、コミットメッセージに書く。PR-2 の暫定の扱い（`same-video` を `accepted` と同じに扱う）に戻すと AC-25 の行が失敗することも確かめる。

**完了の条件:** `make ext-check` が通る。

#### ステップ 3-3: ビューアのブラウザでの確かめ

**変更するファイル:** 本書 7.1 節

- [ ] ビルドした拡張を Chrome と Brave に読み込み、倍率 100% で、`e=1` の URL のビューアを開く（処理中と完了）。ライトとダーク、英語と日本語の組み合わせで、領域の文が読めること（AC-39）、処理中に経過時間が出ないこと（AC-27）、VoiceOver で `#state-status` の要約が読み上げられること（AC-38）を確かめる。
- [ ] 結果を、ステップ 4-4 と同じ形で 7.1 節に書く。

**完了の条件:** 7.1 節に記録がある。

### PR-3 作成ポイント: existing-article viewer

**対象ステップ**: 3-1 / 3-2 / 3-3

**推奨タイトル**: `feat(0058): ビューアに既存の記事を表示していることを示す`

**レビュー観点**: ビューアの URL の `e` の規則（`1` だけ、1 回、`k` が要る、補正しない）と、今の形の URL が読めること / 同じ動画の結果と引き継ぎの `answer` による `e=1` の加え方が設計書 3.7 節の表のとおりであること / 既存の記事のビューアが処理中の経過時間を描かず、完了の所要時間は描くこと / 文言とアクセシビリティ（`#state-status`、コントラスト）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-2 が引き継ぎとやり直しの状態の扱い（状態機械）を変える隔離すべき処理で、ステップ 3-1 が信頼できない入力（ビューアの URL）の受理の規則を変える

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: ポップアップ（設計書 8 章の 4）

#### ステップ 4-1: 選択の状態とボタンの操作

**変更するファイル:** `extension/src/core/popupView.ts`、`test/popupView.test.ts`

- [ ] `ExistingState`、`PopupJobState` の `choice`・`choice-ended`、`PrimaryAction` の `open-existing`・`start-inquiry`、`SecondaryAction` と `secondaryAction` を設計書 3.6 節のとおりに加える。
- [ ] `popupView.test.ts` の `primaryAction` の表に新しい状態の行を加え、`secondaryAction` の表を加える。どちらも、すべての状態の種類について値を確かめる（設計書 3.6 節のボタンの操作の表）。

**完了の条件:** `node --test` で `popupView.test.ts` が通る。`PrimaryAction` の値が増えるので、`render.ts` の対応の表を直すステップ 4-2 までは `make ext-typecheck` が通らなくてよい。

#### ステップ 4-2: ポップアップの送信と描画

**変更するファイル:** `extension/src/launch.ts`、`src/ui/render.ts`、`src/core/sendMessages.ts`、`src/core/i18n/en.ts`・`ja.ts`、`static/popup.html`、`static/style.css`、`scripts/popup-preview.ts`、`test/launch.test.ts`、`test/render.test.ts`、`test/sendMessages.test.ts`、`test/popupHtml.test.ts`、`test/style.test.ts`、`test/popupPreview.test.ts`、`test/catalog.test.ts`（要る場合）

- [ ] `runPopupSend` に、送信ごとの送り方の記憶、選択の状態の流れ、副次のボタンの操作を設計書 3.6 節のとおりに加える。主ボタンの最初の押下と `start-inquiry` は `inquiry` で、`start-plain` は `plain` で、「やり直す」は記憶した送り方で送る。PR-2 の暫定の扱い（1.3 節）はこれで置き換わる。
- [ ] `openViewer` が `NewViewerTab` を受け取る形にし、`open-existing` は押した時刻と `e=1` を持つ新しいタブの URL を作る（設計書 3.6 節）。タブを探す順は変えない。
- [ ] `popup.html` の操作のセクションに、主ボタンの後、`#status` の前に `<button id="secondary" type="button" hidden>` を置き、`findPopupElements` が探す。`render.ts` が選択の状態と既存の記事の状態、副次のボタンを描き、`sendMessages.ts` が文言を選ぶ（設計書 3.6 節・3.8 節）。
- [ ] 副次のボタンの見た目（主ボタンより軽い枠線、44×44 CSS ピクセル以上、フォーカスの印は主ボタンと同じ変数）と、既存の記事の状態の記号の動きを止める規則を `style.css` に書く（AC-36・AC-37・AC-41・AC-42）。
- [ ] `style.test.ts` を次のように広げる（1.4 節）。
    -   ポップアップの大きさの確かめ（`:552` 付近）に、`#secondary` の高さと幅が 44 CSS ピクセル以上であることを加える（AC-41）。
    -   `#secondary` が共通の `:focus-visible` の印（`theme.css`）を打ち消さないこと、押せない状態があれば `#secondary:disabled` の色を、押せないボタンの色の確かめ（`:415`）に加えること（AC-36）。
    -   新しい色の組を `contrastPairs` に加え、今の組の色を使う新しい選択子は、その組の `foregroundRules` に加える（AC-39）。
    -   既存の記事の生成中の記号の選択子を、動きを止める確かめの一覧（`:724` の `moving`）に加えるか、`render.test.ts` でその記号が今の動きのクラスを持つことを確かめる（AC-42）。
- [ ] 文言を英語と日本語のカタログに加える。選択の状態の主ボタンの名前は `popup.primary` に置く（1.3 節）。
- [ ] `popup-preview.ts` に `choice`（既存の記事の取得中・生成中・生成済みと、`PollNotice` の各種類）と `choice-ended` のページを加える。`popupPreview.test.ts` は、`#primary` と同じく、`#secondary` の名前と `hidden` が `renderPopup` の描くものと一致することを確かめる。
- [ ] `launch.test.ts` に次を加える（設計書 7.2 節のポップアップの行）。
    -   主ボタンの押下で SW への求めが `mode: "inquiry"` を持ち、1 回だけ送られること（AC-15 のポップアップの側）。
    -   同じ動画の応答で `choice` を描き、動画のセクションと詳細が残ること、2 つのボタンの名前、既存の記事の状態の取得中・生成中・生成済みとその変化、フォーカスを動かさずに `#status` に書く要約（AC-16・AC-38）。文は英語と日本語のそれぞれで `catalogs[locale]` の値と一致することを確かめる（AC-40）。
    -   同じ動画の応答の後、どのボタンも押さなければ、状態の取得が続いていること（肯定の印）を示したうえで、SW への求めが最初の 1 つのままであること（AC-17）。
    -   `open-existing` が SW に求めを送らず、`openViewer` の順でタブを選ぶこと。この送信のタブがあればそれを前面に出して新しいタブを開かない行と、どのビューアもなければ新しいタブを開く行を含める。新しいタブの URL は、送信の時刻と違う時刻に押したとき（`now()` を進める）、押した時刻と `k` と `e=1` を持つこと。押した後も `choice` のままであること（AC-18）。
    -   `start-plain` が、最初の照会付きの送信と同じ入力で `mode: "plain"` の求めを 1 回だけ送り、`sending` を経て `0050` の状態に移ること。既存の記事のタブに何もしないこと（AC-19）。
    -   状態の取得の失敗・不明で `choice-ended` に移り、主ボタンが「記事を生成する」になり、副次のボタンがなくなり、押すと `mode: "inquiry"` の求めが 1 回だけ送られること（AC-20）。
    -   繰り返せば直りうる結果で通知を添えて取得を続け、繰り返しても直らない結果で通知を添えて取得を止め、どちらも 2 つのボタンを保つこと（AC-21）。
    -   照会付きの送信が受け付けられたとき、拒否・未送信・結果不明のとき、`0050` と同じ表示をすること（AC-22）。
    -   照会付きの送信の後の「やり直す」が `inquiry` で、`start-plain` の後の「やり直す」が、2 回続けても `plain` で送ること（AC-23）。
    -   キーボードで押した主ボタンにフォーカスが戻ること、副次のボタンにフォーカスがある間に `choice-ended` になると主ボタンにフォーカスが移ること（AC-35）。
- [ ] `render.test.ts` に、副次のボタンの名前と `hidden`、主ボタンと副次のボタンの DOM の順、押せないボタンの `disabled` を加える（AC-35・AC-37）。`popupHtml.test.ts` の操作のセクションの子の確かめを、主ボタン・副次のボタン・`#status` の順に直す。
- [ ] 加えたテストの行が、それぞれの分岐を壊すと失敗することを確かめ、コミットメッセージに書く。PR-2 の暫定の扱いに戻すと AC-18 の行が失敗することも確かめる。

ポップアップを開き直すと送る前の画面から始まり何も送らないこと（AC-24）は、テストを加えない。ポップアップは開くたびに新しい文書で送信の記録を持たず、開いただけで何も送らないことは今の `describe("popup job send")`（`launch.test.ts:646`）が確かめている（1.4 節）。閉じて開き直す操作を jsdom で再現しても、そのテストが検出できる誤りがない。

**完了の条件:** `make ext-typecheck`・`make ext-lint`・`make ext-fmt-check` が通り、`metadataDocs.test.ts` のほかの拡張のテストが通る。`metadataDocs.test.ts` は、README を直すステップ 4-3 まで通らない（1.3 節）。

#### ステップ 4-3: 用語集の新しい用語と README

**変更するファイル:** `docs/translation_glossary.md`、`README.md`、`README.en.md`、`extension/test/metadataDocs.test.ts`

- [ ] `translation_glossary.md` に、要件書 F-005 が挙げる新しい用語（同じ動画の照会、照会付きの生成の開始、同じ動画の応答、選択の状態、既存の記事、既存の記事のビューア、副次のボタン、現在の選択範囲）を、英語の訳とともに加える。英語は、カタログの英語の文言で使った語にそろえる。`/mktrans` の前に行う（1.3 節）。
- [ ] `README.md` の「ブラウザ拡張」節に、設計書 3.9 節の表の README の行の内容を書く。選択の状態の 2 つのボタンの名前は、カタログの値を太字で引用する（1.3 節、`metadataDocs.test.ts`）。版をそろえる今の記述（`README.md:561-564` 付近。0059 の後の行番号はステップ 1-1 で引き直す）に、拡張だけを新しくすると古いサーバが照会を無視し、選択を示さずに生成しうることを加える。README のジョブの API の説明に、照会の指定と同じ動画の応答を加える（要件書 F-005）。
- [ ] `metadataDocs.test.ts` の引用の確かめに、副次のボタンの名前を加える（設計書 3.8 節）。選択の状態の主ボタンの名前は `popup.primary` の値として、今の確かめが読む。
- [ ] `/mktrans README.md` で `README.en.md` に写し、2 つを同じコミットに入れる（CLAUDE.md「Translation Guidelines」）。`/mktrans` が用語集に語を加えたら、その変更もこの PR に入れる。
- [ ] `make readme-sync-check`、文書のガード、`make ext-check`、`go test -tags test ./cmd/yt2column-nmhost/`（`TestReadmeHostExitCodes`）を通す。

**完了の条件:** 上のテストがすべて通る。

#### ステップ 4-4: ポップアップのブラウザでの確かめ

**変更するファイル:** 本書 7.1 節

確かめは、ビルドした拡張を読み込んだ Chrome と Brave で、倍率 100%、ライトとダーク、英語と日本語の組み合わせで行う（`0046` のステップ 5-4 と同じ）。

- [ ] AC-16: `popup-preview.ts` の出力を幅 360px のウィンドウで開く。見出しが 2 行を超え、チャンネル名が 200 コードポイントの日本語で、`choice` に通知を添えたページ（`choice` は確認のセクションに代わるので、`0046` の最悪の場合の確認の行は出ない）で、開発者ツールで `#secondary` の `getBoundingClientRect().bottom` を測り、英語と日本語の両方で 600 CSS ピクセル以下であれば合格とする。そのうえで、実際のポップアップでも見出しの長い動画で確かめる。
- [ ] AC-35: キーボードだけ（Tab・Shift+Tab・Enter・Space）で、主ボタン・副次のボタン・`choice-ended` の主ボタンを操作でき、フォーカスの順が見た目の順であれば合格とする。
- [ ] AC-36・AC-37: 2 つのボタンのフォーカスの印が見え、副次のボタンが主ボタンより軽い見た目で、名前で区別できれば合格とする。
- [ ] AC-38: VoiceOver で、`choice` になったこと、既存の記事の状態の変化、`choice-ended`、通知が、フォーカスを動かさずに読み上げられれば合格とする。
- [ ] AC-39・AC-40: 新しい文字と記号が読め、表示の言語とページの `lang` が一致すれば合格とする。
- [ ] AC-42: 開発者ツールの Rendering で `prefers-reduced-motion: reduce` を指定し、既存の記事の生成中の記号が止まれば合格とする。
- [ ] 7.1 節に、確かめた日付、ブラウザとその版、確かめた人、AC ごとの結果を書く。

**完了の条件:** 7.1 節に記録がある。

### PR-4 作成ポイント: popup choice state

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4

**推奨タイトル**: `feat(0058): ポップアップに同じ動画の記事の選択の状態を加える`

**レビュー観点**: 選択の状態の流れ（ポーリングの結果の写し方、`choice-ended`、`open-existing` が何も送らないこと）と、「やり直す」の送り方が設計書 3.6 節のとおりであること / 主ボタンが `inquiry` で送るようになり、自動で送り直さないこと / アクセシビリティ（フォーカスの移し方、名前、44 CSS ピクセル、コントラスト、動き）とブラウザでの確かめの記録 / README と `README.en.md` の記述がそろっていること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-2 がポップアップの状態の移り方とフォーカスの扱いを変える（隔離すべき状態機械の変更）

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 残りの文書（設計書 8 章の 5 のうち、先のフェーズに入れなかったもの）

#### ステップ 5-1: セキュリティの文書と用語集の定義

**変更するファイル:** `docs/dev/security.md`、`docs/translation_glossary.md`

- [ ] `security.md` §8 に、設計書 3.9 節の表の `security.md` の行の内容（照会付きの生成の開始が動画単位で記事の有無とキーを返すこと、それを受け入れる理由、`0044` の T-10 の再評価、記録が動画 ID を持つこと、サーバのログに出す値）を書く（設計書 5.2 節）。
- [ ] `translation_glossary.md` の主ボタン・送信の時刻・ビューアの URL の定義を、要件書 F-005 のとおりに改める。ステップ 4-3 で加えた新しい用語の定義が、実装された振る舞いと合っていることも読み直す。
- [ ] 文書のガードを通す。

**完了の条件:** 文書のガードが通る。

### PR-5 作成ポイント: security document and glossary definitions

**対象ステップ**: 5-1

**推奨タイトル**: `docs(0058): 同じ動画の照会のセキュリティの評価を書き、用語の定義を改める`

**レビュー観点**: `security.md` §8 の T-10 の再評価と返す範囲が設計書 5.2 節のとおりであること / 用語集の改めた定義が要件書 F-005 と実装された振る舞いに合っていること

**実装モデル要件**: standard

**判定理由**: 文書だけの変更で、どの条件にも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: キャッシュ | `Subject`、`Begin` の新しい形と同じ動画の判定、`record.videoID` | PR-1 がマージされた |
| M2: やり取り | 本文の `mode`、同じ動画の応答とログ、拡張の送り方と `classifyStartInquiry`、送る前の大きさの確かめ、SW の引き継ぎ、`development.md` | PR-2 がマージされた |
| M3: ビューア | URL の `e`、既存の記事のビューアの表示、ブラウザでの確かめ | PR-3 がマージされた |
| M4: ポップアップ | 選択の状態、副次のボタン、主ボタンの照会付きの送信、用語集の新しい用語、README、ブラウザでの確かめ | PR-4 がマージされた |
| M5: 残りの文書 | `security.md` §8、用語集の既存の用語の定義 | PR-5 がマージされた |

M1 の前に、0059 の PR-2 がマージされている必要がある（1.3 節）。M2 は M1 の `Begin` の形に依存する。M3 と M4 は M2 の `same-video` の結果に依存する。M4 の「既存の記事を表示する」が開く新しいタブは、M3 の `e=1` の規則に依存する。M5 は M4 の後、実装された振る舞いに合わせて用語集の定義を改める。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 | `Subject`、`Begin` の同じ動画の判定、サーバは `StartPlain` を渡す | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 | 本文の `mode`、同じ動画の応答とログ、拡張の送り方と応答の判定、SW の引き継ぎ、`development.md` | frontier-required |
| PR-3 | 3-1 / 3-2 / 3-3 | ビューアの URL の `e`、既存の記事のビューア、ブラウザでの確かめ | frontier-recommended |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 | 選択の状態、副次のボタン、主ボタンの照会付きの送信、用語集の新しい用語、README、ブラウザでの確かめ | frontier-recommended |
| PR-5 | 5-1 | `security.md` §8、用語集の既存の用語の定義 | standard |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどのテストで確かめるかは本書 7 章に示す。

-   **単体テスト（Go）:** `Begin` の判定はキャッシュのテストで行ごとに確かめ（ステップ 1-3）、サーバのテストは本文から応答とログまでを確かめる（ステップ 2-1）。同時の照会は、キャッシュでは `Begin` の呼び出しをそろえて、サーバでは読み込みの枠の数と同じ数の goroutine から送って確かめる。
-   **単体テスト（拡張）:** `0050` の fake（`launch.test.ts`・`startCoordinator.test.ts` の依存の注入、ホストの返事の fake、`BroadcastChannel` の fake）を使う（設計書 7.2 節、H-07）。実際のブラウザ・ホスト・サーバ・LLM は呼ばない。
-   **統合テスト:** 加えない。ホストは変えない（設計書 3.1 節）ので、`cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd` の生成の開始の本文に `mode` を加え、今のまま通ることを確かめる（ステップ 2-1 の「本文を作るほかのテスト」に含まれる）。
-   **後方互換:** 照会なしの生成の開始、状態の取得、投稿の操作の振る舞いは、既存のテストが通ることで確かめる。本文の補助に `mode` を加えるほかは、既存の行の期待する値を変えない。前の版との組み合わせは要件としない（要件書 4.4）。
-   **セキュリティ:** 応答とログの漏れは目印の値で（AC-10・AC-34）、照会の指定の誤った形は拒否の表で（AC-31）、ほかの送り手とメッセージは拡張のテストで確かめる（AC-33）。設計書 7.3 節のとおりである。
-   **壊して確かめる:** 加えたテストの行は、その対象を壊して失敗することを実装のときに確かめ、コミットメッセージに書く（1.2 節）。
-   **カバレッジ:** 数値の目標は置かない。`make test` と `make ext-test` が通り、7 章の各行のテストがあることを基準にする。本タスクはテストを削除しないので、`make coverage-compare` は使わない。
-   **手動の確かめ:** AC-16 のスクロールと、AC-35〜AC-42 のブラウザでの見え方は、ステップ 3-3・4-4 で確かめて 7.1 節に記録する。文言（AC-28・AC-30）と文書（AC-29）は PR のレビューで読む。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 0059 の PR-2 が遅れる | 本タスクを始められない | 1.3 節のとおり待つ。待たずに始める必要が出たら、`mode` の検査の置き場所を改めてレビュアーと決める |
| 0059 の後で、本書と設計書の `file:line` や名前がずれる | 実装者が誤った箇所を直す | ステップ 1-1 で引き直す。名前や振る舞いの食い違いは設計書を先に直す |
| 同時の照会のテストが不安定になる | CI が時々失敗する | キャッシュのテストは goroutine をそろえて `Begin` を呼び、`-tags test -race -count=20` で繰り返し実行し、テストが実行されたことも確かめる（ステップ 1-3）。サーバのテストは結果だけを確かめ、排他制御はキャッシュのテストが確かめる（`TestStartConcurrentSameKey` と同じ分担） |
| 本文を作るテストに `mode` を加え忘れる | 受理を期待するテストは失敗する。`invalid_body` を期待する行は、`mode` がないことで拒否されて通り、行が名指す検査を確かめなくなる | 補助をまとめて直し、拒否の行にも正しい `mode` を加える。拒否の行の本文は、受理されることを確かめた正しい本文から作る（ステップ 2-1・2-3、1.4 節） |
| PR-2 の暫定の扱いが PR-3・PR-4 で置き換わらずに残る | 選択の状態や `e=1` が出ない | ステップ 3-2・4-2 で置き換えることをチェックボックスにし、暫定の扱いに戻すと AC-25・AC-18 の行が失敗することを確かめる |
| `metadataDocs.test.ts` が PR-4 で失敗する | PR-4 がグリーンゲートを通らない | README をカタログと同じ PR に入れる（1.3 節、ステップ 4-3） |
| `/mktrans` が用語集を書き換え、PR-5 の用語集の変更と重なる | 同じ用語を 2 度加える | 新しい用語は `/mktrans` の前に PR-4 で加え、PR-5 は既存の用語の定義だけを改める（1.3 節） |
| ポップアップの高さが 600 CSS ピクセルを超える | AC-16 を満たさない | ステップ 4-4 で最悪の場合を測る。直し方が設計書と食い違うときは設計書を先に直す |
| 予定の遅れ | なし | 5 つの PR はそれぞれ単独で動く形にしてあり、途中で止めても製品は使える（1.3 節） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4）
- [ ] PR-5 マージ済み（対象ステップ: 5-1）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（実行するテストや `make` の対象）と `manual`（PR のレビューで読む、ブラウザで確かめる）である。本タスクに `static` の確かめはない。本タスクに `diff` の確かめはない（1.3 節）。拡張のテストは `make ext-test` で、Go のテストは `make test` で実行する。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | test | `internal/articlecache/cache_test.go::TestBeginSameVideo`（完了の記事の行、記録が変わらない）、`internal/server/jobs_api_test.go::TestStartInquirySameVideo`（`youtu.be` と `watch?v=`、字幕だけ違う、LLM が呼ばれない） | 1-3・2-1 |
| AC-02 | test | `internal/articlecache/cache_test.go::TestBeginSameVideo`（処理中の記事の行）、`internal/server/jobs_api_test.go::TestStartInquirySameVideo`（タイトル・チャンネル名・概要欄だけ違う） | 1-3・2-1 |
| AC-03 | test | `internal/articlecache/cache_test.go::TestBeginSameVideo`（失敗だけ、別の動画だけの行）、`internal/server/jobs_api_test.go::TestStartInquiryNoSameVideo` | 1-3・2-1 |
| AC-04 | test | `internal/articlecache/cache_test.go::TestBeginSameVideo`（順 1 の行）、`internal/server/jobs_api_test.go::TestStartInquiryOwnKey` | 1-3・2-1 |
| AC-05 | test | `internal/articlecache/cache_test.go::TestBeginSameVideo`（今のキーが失敗の行）、`internal/server/jobs_api_test.go::TestStartInquiryOwnKey` | 1-3・2-1 |
| AC-06 | test | `internal/articlecache/cache_test.go::TestBeginSameVideo`（世代の番号の最大、やり直した記録）、`internal/server/jobs_api_test.go::TestStartInquiryLatest` | 1-3・2-1 |
| AC-07 | test | `internal/articlecache/cache_test.go::TestBeginSameVideo`（生成の枠がない、捨てられる記録がない行）、`internal/server/jobs_api_test.go::TestStartInquiryWhenFull` | 1-3・2-1 |
| AC-08 | test | `internal/articlecache/cache_test.go::TestBeginConcurrentSameVideo`、`internal/server/jobs_api_test.go::TestStartConcurrentSameVideo` | 1-3・2-1 |
| AC-09 | test | `internal/articlecache/cache_test.go::TestBeginPlainIgnoresSameVideo`、`internal/server/jobs_api_test.go::TestStartPlainIgnoresSameVideo`、既存の生成の開始のテスト（`TestStartAccepted` など）が `plain` で通ること | 1-3・2-1 |
| AC-10 | test | `internal/server/jobs_api_test.go::TestStartInquirySameVideo`（メンバの集合、Webhook の fake が呼ばれない）、`internal/server/status_test.go::TestResponsesOmitInputs`（同じ動画の応答の目印） | 2-1 |
| AC-11 | test | `internal/server/jobs_api_test.go::TestStartHandlerPanicCleanup`（照会付きの生成を始める経路と、同じ動画の応答を書く経路の行） | 2-1 |
| AC-15 | test | `extension/test/launch.test.ts`（主ボタンの求めが `mode: "inquiry"` を 1 回）、`extension/test/startCoordinator.test.ts`（ホストが `inquiry` の `start` を 1 つ）、`extension/test/hostMessage.test.ts` | 2-2・2-3・4-2 |
| AC-16 | test・manual | test: `extension/test/launch.test.ts`（`choice` の描画、状態の変化）、`extension/test/render.test.ts`。manual: ステップ 4-4 の AC-16 の確かめ（7.1 節） | 4-2・4-4 |
| AC-17 | test | `extension/test/startCoordinator.test.ts`（同じ動画の応答の後に `start` が増えない）、`extension/test/launch.test.ts`（求めが最初の 1 つのまま） | 2-3・4-2 |
| AC-18 | test | `extension/test/launch.test.ts`（`open-existing` のタブの順、`e=1` と押した時刻の URL、求めを送らない） | 4-2 |
| AC-19 | test | `extension/test/launch.test.ts`（`start-plain`）、`extension/test/startCoordinator.test.ts`（`plain` の `start` を 1 つ） | 2-3・4-2 |
| AC-20 | test | `extension/test/launch.test.ts`（`choice-ended`）、`extension/test/popupView.test.ts`（`primaryAction`・`secondaryAction`） | 4-1・4-2 |
| AC-21 | test | `extension/test/launch.test.ts`（通知と取得の続け方・止め方） | 4-2 |
| AC-22 | test | `extension/test/launch.test.ts`（照会付きの送信の受け付け・拒否・未送信・結果不明の表示）と、`0050` の状態の既存のテストが通ること | 4-2 |
| AC-23 | test | `extension/test/launch.test.ts`（送り方ごとの「やり直す」） | 4-2 |
| AC-24 | test | `extension/test/launch.test.ts` の `describe("popup job send")`（開いただけでは何も送らない、今のテスト。ポップアップは送信の記録を持たない。ステップ 4-2 の後の段落） | 4-2 |
| AC-25 | test | `extension/test/viewerApp.test.ts`（`same-video` の結果で `k` と `e=1`、領域、`accepted` では出ない） | 3-2 |
| AC-26 | test | `extension/test/viewerApp.test.ts`（`answer: "same-video"` の引き継ぎ）、`extension/test/startCoordinator.test.ts`（`answer` を持つ引き継ぎ） | 2-3・3-2 |
| AC-27 | test | `extension/test/viewerApp.test.ts`（`e=1` の URL で開き直す、経過時間を描かない、所要時間を描く）、`extension/test/viewerUrl.test.ts`（`e` の往復）、`extension/test/renderJob.test.ts` | 3-1・3-2 |
| AC-28 | manual | PR-3 のレビューで、英語と日本語の文言を読む | 3-2 |
| AC-29 | manual・test | manual: PR-2・PR-4・PR-5 のレビューで README（両方）・`development.md`・`security.md`・`translation_glossary.md` を読む。test: 文書のガード（`TestServerDocsContract` の curl の例と拒否の理由の表、変わらない）、`make readme-sync-check`、`extension/test/serverReasons.test.ts`、`extension/test/metadataDocs.test.ts`（README のボタンの名前）。拒否の理由とコードは加えない。`development.md` の例の本文が受理されることは、ステップ 2-4 の一度の確かめ（7.1 節） | 2-4・4-3・5-1 |
| AC-30 | manual | PR-3・PR-4 のレビューで、ポップアップ・ビューア・README の英語と日本語の文言を読む | 3-2・4-2・4-3 |
| AC-31 | test | `internal/server/request_test.go::TestStartRejectsBody`（`mode` の行、`Begin` を呼ばない、キャッシュが変わらない）、`extension/test/hostMessage.test.ts`（`startMessage` が `mode` を付ける） | 2-1・2-2 |
| AC-32 | test | `extension/test/response.test.ts`（`classifyStartInquiry` の describe と `reasonTable` の行） | 2-2 |
| AC-33 | test | `extension/test/extensionMessages.test.ts`（`mode`・`answer` の誤り）、`extension/test/startCoordinator.test.ts`（誤った求め、ほかの送り手）、`extension/test/viewerUrl.test.ts`（`e` の規則）、`extension/test/viewerApp.test.ts`（`e` の誤った URL） | 2-3・3-1・3-2 |
| AC-34 | test | `internal/server/jobs_api_test.go::TestStartInquirySameVideo`（ログの行の属性、`requestId` の行が 1 行、送った `videoUrl` の文字列がない）、`internal/server/status_test.go::TestResponsesOmitInputs`（ログに目印がない） | 2-1 |
| AC-35 | test・manual | test: `extension/test/launch.test.ts`（フォーカスの戻し方と移し方）、`extension/test/render.test.ts`（DOM の順）。manual: ステップ 4-4 のキーボードの確かめ | 4-2・4-4 |
| AC-36 | test・manual | test: `extension/test/style.test.ts`（`#secondary` が共通の `:focus-visible` の印を打ち消さない、押せないボタンの色、フォーカスの印の色の組）。manual: ステップ 4-4 | 4-2・4-4 |
| AC-37 | test | `extension/test/render.test.ts`（名前、`hidden`、`disabled`）、`extension/test/popupHtml.test.ts`（ネイティブの `<button>`） | 4-2 |
| AC-38 | test・manual | test: `extension/test/launch.test.ts`（フォーカスを動かさずに `#status` に書く要約）、`extension/test/viewerApp.test.ts`（`#state-status` の要約）。manual: ステップ 3-3・4-4 の VoiceOver の確かめ | 3-2・3-3・4-2・4-4 |
| AC-39 | test・manual | test: `extension/test/style.test.ts`（`contrastPairs` に加えた組と `foregroundRules`、ライトとダーク）、`extension/test/render.test.ts`（既存の記事の状態の文字）。manual: ステップ 3-3・4-4 | 3-2・3-3・4-2・4-4 |
| AC-40 | test | `extension/test/launch.test.ts`・`extension/test/renderViewer.test.ts`（新しい文を英語と日本語で描き、`catalogs[locale]` の値と一致する）、`extension/test/catalog.test.ts`（すべてのカタログのすべての文言が空でない）、`extension/test/locale.test.ts`（`lang`、変えない） | 3-2・4-2 |
| AC-41 | test | `extension/test/style.test.ts`（ポップアップの大きさの確かめに加える `#secondary` の高さと幅） | 4-2 |
| AC-42 | test・manual | test: `extension/test/style.test.ts`（`moving` の一覧に加えた選択子の `prefers-reduced-motion` の規則）か `extension/test/render.test.ts`（記号が今の動きのクラスを持つ）。manual: ステップ 4-4 | 4-2・4-4 |

AC-12・AC-13・AC-14 は要件書の欠番である。

### 7.1. 実行の記録

（ステップ 2-4・3-3・4-4 で記入する。）

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 7 章のすべての行の確かめが済み、7.1 節に `development.md` の例の本文の確かめと、ブラウザでの確かめの記録がある。
-   **品質:** 各 PR でグリーンゲート（`make test && make lint`）、`make deadcode`、`make ext-check` が通る。加えたテストは、対象を壊して失敗することをコミットメッセージに記録している。
-   **セキュリティ:** 同じ動画の応答とそのログが、字幕・`videoUrl` の文字列・動画メタデータ・秘密の値を含まない（AC-10・AC-34）。`mode` の誤った本文が生成を始めずに拒否される（AC-31）。
-   **文書:** README（両方）・`development.md`・`security.md`・用語集・`package_reference.md` が、本タスクの後の振る舞いを説明している。

## 9. 次のステップ (Next Steps)

-   PR-5 のマージの後、`/runpr` が issue #593 を閉じる案内をする（本タスクの最後の PR のため）。
-   最近の記事の一覧やキャッシュの永続化が要るときは、別のタスクで設計する（設計書 9 章）。

## 変更履歴

- 2026-11-04 承認（isseis、#657）
