# 実装計画書：ブラウザ拡張からサーバへの送信と結果の表示

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-16 |
| Review date | 2026-10-16 |
| Reviewer | isseis |
| Comments | AC-15a/AC-15b の対応表に launch.test.ts の経路テストを加えた編集上の訂正で、判断は変わっていない |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定義した、ブラウザ拡張からサーバへの送信と結果の表示を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。具体的には、次のことを行う。

-   設定（共有トークンとポート）の検証と保存、送る前の確認と本文の組み立て、応答の解釈、文言を、ブラウザの API に依存しない `core/` に作る。
-   送信（`sendArticle`）と、保存先・`fetch` の実装を作る。
-   manifest に `host_permissions`・`options_ui` を加え、オプションページを作る。
-   メニューの経路を「収集した内容の表示」から「送信と送信の結果の表示」に改め、ポップアップに送信のボタンを加える。
-   Chrome と Brave で手動の確認を行い、結果を本計画に記録する。README・security.md・CLAUDE.md を改める。

本計画が使う記号は次のとおりである。AC-NN は要件書の受け入れ基準、F-NNN は要件書の機能要件、H-NN は [design_handoff.md](design_handoff.md) の項目を指す。「§N」は本計画の節、「設計書 N」は設計書の節を指す。

本計画は、設計書 8 章の実装優先順位 1〜5 を、そのままフェーズ 1〜5 とする。

### 1.2. 実装原則

-   設計書 1.1 の設計原則に従う。特に、判定を `core/` に置くこと、`PreparedRequest` を `prepareSend` だけが作れる型にすること、応答を信頼できない入力として受理する形だけを受け入れること、結果が分からなければ Slack を確かめるよう案内して自動で再送しないこと、共有トークンを保存先と `Authorization` ヘッダにだけ置くことを守る。
-   新設するファイルは設計書 2.1・3.14 に挙げたものと、`extension/test/helpers/` の補助のモジュール（ステップ 1-6・2-3・3-3。§1.3「テストの補助」）に限る。
-   Go のソースを変更しない。Go のテストは、`cmd/yt2column/extension_docs_test.go` の文書のガード（ステップ 5-5。設計書 3.12）だけを加える。
-   拡張のソースのコメント・識別子・文字列リテラル・利用者に表示する文言は英語で書く（CLAUDE.md「Go Idioms」の拡張の規則）。例外は、日本語の文書の見出しや語句と照合するためにテストが持つ文字列リテラル（`manifest.test.ts`・`extension_docs_test.go` が security.md・本計画を読む部分）だけとする。`AC-NN`・`F-NNN`・`H-NN` はソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。Go のテストで本計画の表の行の名前を照合するときは、既存の `acLabel`（`cmd/yt2column/extension_docs_test.go:480-486`）で組み立てる。
-   テストの名前（`describe` の名前、Go のテストの関数名）は計画上の名前である。実装で変える場合は、§5 を同じコミットで更新する。
-   各テストと lint の規則は、対象の処理を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」、設計書 3.12）。壊す対象と、それを検出するテストは、各フェーズの最後のステップに挙げる。
-   各フェーズの完了条件は、`make ext-check` → `make test` → `make lint` が通ることである。`make test` は、security.md と README を読む Go のガードのテスト（§1.3「文書」）を含む。

### 1.3. 既存コード調査結果

HEAD `e8b21fa`（ブランチ `issei/browser-extension-03`）で確認した。設計書は既存のファイルの行番号をコミット `c4b656e` で記している。`git diff --name-only c4b656e e8b21fa -- extension cmd README.md docs/dev CLAUDE.md Makefile .github` の出力は空だった（2026-10-16 に実行）。したがって、設計書の `file:line` は HEAD `e8b21fa` でもそのまま有効である。以下の `file:line` も HEAD `e8b21fa` のものである。

**`core/`（変更するもの）**

-   `hasExactKeys`（`extension/src/core/summary.ts:69-79`）と `isRejectionReason`（`:81-87`）は非公開である。ステップ 1-1 で `export` を付ける。振る舞いは変えない（設計書 3.11）。既存の `summary.test.ts` の「parseSummary」（`extension/test/summary.test.ts:105-176`）が、`parseSummary` を通してこれらの判定を確かめている。この `describe` は変更しない。
-   `rejectionReasons`（`extension/src/core/types.ts:13-18`）は `Record<RejectionReason, true>` で、`preflightReasons`・`unexpectedChecks` は同じ形で作る（設計書 3.4・3.6）。
-   `RejectionMessage`（`extension/src/core/messages.ts:3-8`）と `rejectionMessage`（`:15-46`）は変更せず、`core/sendMessages.ts` から使う（設計書 3.7）。

**型検査**

-   `tsconfig.json` の `include` は `src`・`test`・`scripts`（`extension/tsconfig.json`）で、`make ext-typecheck` はテストも型検査する。`tsconfig.build.json` の `include` は `src` だけである。そのため、本番のコードの interface を変えるステップは、テストを直すステップまで `make ext-typecheck` が通らない。ステップ 2-2・4-2・4-3 の完了条件は、この前提で書く。
-   `tsconfig.core.json` は `lib: ["ES2022"]`・`types: []` で、`include` は `src/core` と `types/core-url.d.ts` である。`TextEncoder`・`TextDecoder` の宣言はない。`types/core-encoding.d.ts` を加えて `include` に足す（設計書 2.1）。
-   `typecheck.test.ts` の `runTsc`（`extension/test/typecheck.test.ts:51-75`）は、一時的な設定を `include: []` で作り、`tsconfig.core.json` のときは `types/core-url.d.ts` を `files` に直接足す（`:66-70`）。`tsconfig.core.json` の `include` は読まない。そのため、`core-encoding.d.ts` を必要とするテストの行（ステップ 1-1）を加えるには、`runTsc` にもこのファイルを足す必要がある。`tsconfig.core.json` の `include` から外したことは、`make ext-typecheck`（`tsc -p tsconfig.core.json`）が検出する。

**lint の規則**

-   型の表明の禁止は `CollectedInput` の 1 つで、`files: ["src/**/*.ts", "scripts/**/*.ts"]`・`ignores: ["src/core/collect.ts"]` の 1 つの設定のオブジェクトにある（`extension/eslint.config.js:85-102`）。選択子は `typeAnnotation.typeName.name="CollectedInput"`（`:96`）で、表明する型がその名前の型の参照そのものである場合だけに一致する。共通の選択子 `restrictedSyntax`（`:23-61`）は、ファイルを限定しない設定のオブジェクト（`:70-84`）を通じてテストのファイルにも当たる。flat config では後の設定のオブジェクトが `no-restricted-syntax` を丸ごと置き換えるので、ファイルを限定する設定のオブジェクトは、それぞれ `...restrictedSyntax` を並べ直す必要がある（設計書 3.11）。
-   既存のメンバの禁止は、`.name`・`["name"]`・`` [`name`] `` の 3 つの参照の書き方（`:24-35`）と、分割代入のキー（`:36-40`）を対象にする。`storage.sync` の禁止も同じ 4 つの書き方を対象にする。既存のソースとテストは `storage.sync` を参照しない（`rg -n "storage\.sync" extension` の結果は 0 件。終了コード 1。2026-10-16、HEAD `e8b21fa`）。
-   既存の `lintRules.test.ts` は、共通の選択子を `src/sample.ts` と `test/sample.test.ts` の名前だけで確かめる（`extension/test/lintRules.test.ts:24`）。既存の「CollectedInput assertions」（`:126-164`）の確かめ方（作り手のファイルとテストで許され、ほかのソースとスクリプトで禁じられること、山括弧の表明も禁じられること）は、5 つのブランド付きの型を対象にする「branded type assertions」の表の行として残す（設計書 3.11）。

**ブランド付きの型の作り方の制約（Conditional checks の「具体的な型の作り方」）**

-   設計書 3.2 のブランド付きの型（`SharedToken`・`ServerPort`・`PreparedRequest`・`RequestId`）は、型にだけ存在する `unique symbol` を目印にする。目印をモジュールの外へ公開しないので、ほかのモジュールはリテラルでこれらの型を作れない。本計画は、リーフパッケージへの移動ではなく、lint で禁止する方式（設計書 3.11）を採る。作り手のファイルとテスト以外で、ブランドの値を作れる書き方と、その扱いを次に示す。

| 書き方 | 扱い | 確かめるテスト |
|---|---|---|
| `value as T`・`<T>value` | 禁止する | `lintRules.test.ts::branded type assertions` |
| 表明する型の中にブランドの型が現れる（`as T \| undefined`、`as Readonly<T>`） | 禁止する（表明する型の中のブランドの型の参照すべてに一致させる） | 同上 |
| 名前空間の import 経由の参照（`as settings.SharedToken`） | 禁止する | 同上 |
| 利用者定義の型の述語（`(v): v is T`） | 禁止する | 同上 |
| 別名の import（`import { RequestId as R }`）・型の別名（`type R = RequestId`）を介した表明 | 別名を作ることを禁止する | 同上 |
| 型の付かない値（`JSON.parse` の結果などの `any`）をブランドの型の変数に代入する | 禁止しない。型情報を使う lint（`no-unsafe-assignment`）は現在の設定（`tseslint.configs.recommended`。`extension/eslint.config.js:66`）にない。残るリスクとして受け入れる（§6.1） | なし |

**`launch.ts`・`browser/chromeDeps.ts`・エントリポイント**

-   `SummaryStore`（`extension/src/launch.ts:14-23`）は `put`・`take`・`remove` を持ち、`MenuLaunchDeps`（`:25-34`）と `ResultWindowDeps`（`:42-45`）が使う。`createSummaryStore`（`extension/src/browser/chromeDeps.ts:95-109`）が実装し、`background.ts:10,27` と `result.ts:3,7` が `chrome.storage.session` で作る。
-   `StorageArea`（`extension/src/browser/chromeDeps.ts:13-17`）は `get(keys: string)` で、コメントは「The chrome.storage.session surface used.」（`:12`）である。
-   `missingResultText`（`extension/src/launch.ts:59-60`）は、`runResultWindow`（`:160-190`）の 3 か所で `renderNotice`（`:149-154`）に渡される。`renderNotice` の呼び出し元はこの 3 か所だけである。
-   ログのラベルのうち `putLabel`（`"yt2column store summary"`）・`removeLabel`（`"yt2column remove summary"`）・`keyLabel`（`"yt2column create summary key"`）は「summary」を含む（`extension/src/launch.ts:51-56`）。
-   `runPopupLaunch` の doc コメント（`extension/src/launch.ts:128-129`）は「returning the outcome for #111's send button」である。`popup.ts:10` は `document.body` に描画する。
-   `background.ts:39-43` の `removeAllMenuItems` のコメントは「the manifest supports Chrome 102 for chrome.storage.session」である。
-   `Logger`（`extension/src/launch.ts:8-11`）は `optionsPage.ts` からも使う（設計書 2.2）。`attempt`（`:73-82`）は失敗を記録して投げない補助で、本タスクの経路の処理でも使う。

**変更・削除する名前の一覧（名前の台帳）**

本計画で名前を変える・消すものは、この表を正とする。タスクの本文と §5 は、この表に合わせる。

| 種類 | 旧 | 新 | 場所 | ステップ |
|---|---|---|---|---|
| interface | `SummaryStore` | `ResultViewStore`（`put`・`get`・`watch`・`remove`） | `extension/src/launch.ts` | 2-2（追加）・4-2（旧名の削除） |
| 関数 | `createSummaryStore` | `createResultViewStore` | `extension/src/browser/chromeDeps.ts` | 2-2（追加）・4-2（旧名の削除） |
| メソッド | `SummaryStore.take` | なし（`get`・`watch`・`remove` で置き換える） | 同上 | 4-2 |
| 定数 | `missingResultText` | なし（`unreadableResultMessage`。`core/sendMessages.ts`） | `extension/src/launch.ts` | 4-2 |
| 関数 | `renderNotice`（非公開） | なし（読めない場合も `RejectionMessage` を `messageNodes` で描く） | `extension/src/launch.ts` | 4-2 |
| ログのラベル | `putLabel`・`removeLabel`・`keyLabel` の値の「summary」 | 「result view」（例: `"yt2column store result view"`） | `extension/src/launch.ts` | 4-2 |
| 関数 | `rejectedNodes`（非公開） | `messageNodes`（公開。文言と手順の部分） | `extension/src/ui/render.ts` | 4-1 |
| テストの `describe` | `summary store` | `result view store` | `extension/test/chromeDeps.test.ts` | 4-6（旧名の削除） |
| テストの `it` | `requires Chrome 102 for storage.session` | `requires Chrome 103 for AbortSignal.timeout` | `extension/test/manifest.test.ts` | 3-1 |
| テストの補助 | `recordingStore`（`take` を持つ。`launch.test.ts:55-92`） | `ResultViewStore` の fake | `extension/test/launch.test.ts` | 4-5 |
| テストの補助 | `recordingLogger`（`launch.test.ts:30-53`） | 同じ名前で `test/helpers/logger.ts` へ移す | `extension/test/` | 3-3 |

**既存のテストで更新が必要なもの**

設計書 3.11 の表の「更新が必要な既存のテスト」をそのまま使う。行番号は HEAD `e8b21fa` でも同じである。

-   `launch.test.ts` のメニューの経路の行: `menuHarness`（`extension/test/launch.test.ts:146-172`）、2 つの経路で保存した要約を比べる行（`:229-`・`:248-`・`:262-`）、メニューの経路を実行する行（`:288-`・`:301-`）、`put`・`windows.create` の失敗の行（`:311-`〜`:350-`）。
-   `launch.test.ts` の結果のウィンドウの行（`:396-`・`:414-`・`:436-`・`:449-`・`:459-`・`:474-`）は `take` と `missingResultText` を使う。
-   `chromeDeps.test.ts` の `memoryStorage`（`extension/test/chromeDeps.test.ts:97-110`）は `get(key)` を 1 つのキーで扱う。「summary store」（`:112-119`）は `take` を確かめる。
-   `ciChanges.test.ts` は、起動しない一覧に `["docs/dev/security.md"]`（`extension/test/ciChanges.test.ts:52`）と `["README.md"]`（`:53`）を持つ。起動する一覧の `["README.md", "extension/package.json"]`（`:28`）は、`README.md` が単独で起動するようになると、2 つの要素のどちらが起動させたかを区別しない行になる。起動の条件の各パターンについて、先頭以外に現れるパスと、後ろに続きを持つパスが起動しないことを確かめる行がある（`:55-62`）。
-   `manifest.test.ts` の「undeclared keys」（`extension/test/manifest.test.ts:28-41`）と「requires Chrome 102 for storage.session」（`:48-50`）。
-   `checkDist.test.ts` の manifest の参照の行（`extension/test/checkDist.test.ts:135-157`）と、`check-dist.ts` の参照の検査（`extension/scripts/check-dist.ts:120-140`）。

**テストの補助**

-   `recordingLogger`（`extension/test/launch.test.ts:30-53`）は `launch.test.ts` の中にあり、`optionsPage.test.ts` も同じものを使う。`extension/test/helpers/logger.ts` に移す（ステップ 3-3）。
-   設定の読み込みの fake（`SettingsReader`。成功・失敗・呼ばれた回数）は `prepare.test.ts` と `launch.test.ts` が使う。`extension/test/helpers/settings.ts` に置く（ステップ 1-6）。識別できる 64 文字の共有トークン（AC-09・AC-32）もここに置く。
-   fake の `FetchFn`（呼び出しの記録、応答の本文を Node.js の `ReadableStream` で渡す、拒否、読み込みの途中の失敗）は `send.test.ts` と `launch.test.ts` が使う。`extension/test/helpers/fetch.ts` に置く（ステップ 2-3）。
-   補助のモジュールは、0007 と同じく `node --test` のパターン（`test/**/*.test.ts`）に一致しない名前にする。`memoryStorage` は `chromeDeps.test.ts` だけが使うので移さない。

**文書**

-   README の拒否の理由の表は `README.md:266-279` にある。Go のテスト `rejection_reasons`（`cmd/yt2column-server/docs_test.go:88-114`）が、サーバの組とちょうど一致することを確かめている。`serverReasons.test.ts` はこの表を同じ形で読む。
-   README の「Browser extension」の節（`README.md:370-412`）は「At this point it never sends anything」（`:374-376`）を、「The shared token」の節は「once it sends requests (#111)」（`:215-217`）を持つ。
-   security.md §8 の冒頭（`docs/dev/security.md:89`）は「収集した字幕・URL・タイトル」を送るとし、`:95` は「トークンの受け渡しと保存先は #109・#111 で決める」、§8.1 の `:99` は「サーバへの送信は #111 で加える」、`:101` は「**要求する権限**」の一覧、§8.2 の `:120` は「確定は #111」である。
-   Go のガードのテストは security.md を次のとおり読む。`cmd/yt2column/extension_docs_test.go:85-115` は §8.1 の見出し「### 8.1. ブラウザ拡張（#110）」と「**要求する権限**」の一覧を読む。一覧は最初の `。` までで、括弧の外のコードスパンの直後に `（` が続くものを権限として読む（`declaredPermissions`。`:395-449`）。`cmd/yt2column-server/docs_test.go:130-148` は §8.2 の見出しと `chrome.storage.local` などの語を読む。したがって、security.md の変更（ステップ 3-2・5-3）では次の 4 点を守る。見出しを変えない。`host_permissions` は「**要求する権限**」の一覧の外に書く。`storage` の用途に `chrome.storage.local` を加えるときは、`storage` の理由の括弧の中に書く。§8.2 には `chrome.storage.local` を残す。
-   0007 の手動の確認の記録は、`cmd/yt2column/extension_docs_test.go:144-192` が「### 5.1. 手動の確認と記録」の表の各行の名前と、すべての欄が埋まっていることを確かめている。本タスクの AC-30 と §5.1 の検査は、同じファイルに同じ形で加える（設計書 3.12）。CI の `doc-guard` のジョブは `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline` を実行する（`.github/workflows/ci.yml:184`）ので、文書だけの変更でもこの検査が実行される。
-   **AC-30 の静的な確認の置き場所（設計書の修正）:** 設計書 7.4 は当初、AC-30 の確かめ方を「文書（静的な確認）」とし、実行する検査を定めていなかった。文書のテキストの同期のガードを Go のテストに置くタスク 0012 の方針に合わせ、AC-30 の確かめ方も Go のガードのテストにすることにし、本計画の作成と同じ変更で設計書 1.3・3.12・3.14・7.3・7.4 を改めた。「Go のテストを変更しない」とした決定を改めるので、設計書の状態を `draft` に戻した（`requirements_process.md`「Editing an approved document」）。
-   `docs/dev/project_overview.md:98` は `extension/` を「選択範囲の収集と表示」と説明している。送信を加えた後は古くなるので、ステップ 5-3 で改める。Go のテスト（`extension_docs_test.go:117-126` の「adds TypeScript and extension/ to the project overview」）が確かめるのは `extension/` の語の有無だけである。
-   CLAUDE.md の「Development Notes」（`CLAUDE.md:241-248`）は、文書だけの変更は `make ext-test` を要しないとし、`doc-guard` のジョブのコマンドを `./cmd/yt2column ./internal/pipeline` と書いている。後者は `ci.yml:184` と食い違っている（`./cmd/yt2column-server` がない）。ステップ 5-3 で、設計書 3.14 の変更と合わせて直す。

**AC-33 の前提**

-   `rg -n -i -e webhook -e 'api.?key' -e DEEPSEEK -e ANTHROPIC extension/src extension/static` の結果は 0 件（終了コード 1。2026-10-16、HEAD `e8b21fa`）。`secretsAbsent.test.ts`（ステップ 4-6）は、現在の拡張のソースで成功する。

### 1.4. 申し送りの項目への対応

-   `implementation_handoff.md` はない。
-   [design_handoff.md](design_handoff.md) の H-01〜H-07 は、すべて設計書 3.13 に記録されている。本計画は設計書のその方針に従って実装する（H-01・H-05 はフェーズ 4、H-02・H-03・H-04 はフェーズ 1・2、H-06 はフェーズ 1〜3、H-07 はフェーズ 3）。

## 2. 実装ステップ (Implementation Steps)

各ステップの末尾に **対象:**（そのステップが触るファイル）と **完了:**（そのステップの検証）を記す。フェーズの「完了条件」は、そのフェーズのすべてのステップの後に確かめる。

### フェーズ 1: core

**対象ファイル**
-   新設: `extension/types/core-encoding.d.ts`、`extension/src/core/settings.ts`・`prepare.ts`・`response.ts`・`sendMessages.ts`・`resultView.ts`、`extension/test/settings.test.ts`・`prepare.test.ts`・`response.test.ts`・`sendMessages.test.ts`・`resultView.test.ts`、`extension/test/helpers/settings.ts`
-   変更: `extension/src/core/summary.ts`、`extension/tsconfig.core.json`、`extension/eslint.config.js`、`extension/test/lintRules.test.ts`・`typecheck.test.ts`

**タスク**
-   [x] **ステップ 1-1**: `types/core-encoding.d.ts` を作り（`core/` が使う `TextEncoder.encode` と、`TextDecoder` の構築子・`decode` の宣言だけ）、`tsconfig.core.json` の `include` に足す。`typecheck.test.ts` の `runTsc` が、`tsconfig.core.json` を検査するときに足す宣言のファイルへ `core-encoding.d.ts` を加える。あわせて、`TextEncoder`・`TextDecoder` を使うファイルを `tsconfig.core.json` が受理することを確かめる行を加える（§1.3「型検査」）。`core/summary.ts` の `hasExactKeys`・`isRejectionReason` に `export` を付ける。
    -   **対象:** `extension/types/core-encoding.d.ts`・`tsconfig.core.json`・`src/core/summary.ts`・`test/typecheck.test.ts`。**完了:** `make ext-typecheck` と `typecheck.test.ts` が通る。
-   [x] **ステップ 1-2**: `core/settings.ts` を設計書 3.3 のとおりに作る（`defaultPort`・`parseToken`・`parsePort`・`checkSettings`・`SettingsReader`・`SettingsStore`）。補正をしないこと、`[0-9]` を ASCII に限ることは設計書 3.3 の「受理する形」のとおりとする。
    -   **対象:** `extension/src/core/settings.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 1-3**: `core/prepare.ts` を設計書 3.4 のとおりに作る（`buildArticleBody`・`maxBodyBytes`・`PreparedRequest`・`preflightReasons`・`prepareSend`）。確認の順序は設計書 3.4 のフローチャートのとおりで、収集の拒否では `settings.load` を呼ばない。バイト数は `TextEncoder` で数える。
    -   **対象:** `extension/src/core/prepare.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 1-4**: `core/response.ts` を設計書 3.6 のとおりに作る（`serverRejectStatus`・`isServerRejectReason`・`parseRequestId`・`maxResponseBytes`・`unexpectedChecks`・`toSendOutcome`・`classifyResponse`）。判定は 3.6 の表の順 1〜9 とし、解析した値は `Object.keys`・`Object.hasOwn`（または `hasExactKeys`）だけで調べる。
    -   **対象:** `extension/src/core/response.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 1-5**: `core/sendMessages.ts`（設計書 3.7）と `core/resultView.ts`（設計書 3.8 の `ResultView`・`parseResultView`）を作る。`blockMessage` は `RejectionReason` を `rejectionMessage` に渡す。3 つの関数の `switch` は `default` で `never` を受ける。`parseResultView` は設計書 3.8 に挙げた既存の検査の関数を使い、再実装しない。
    -   **対象:** `extension/src/core/sendMessages.ts`・`resultView.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 1-6**: lint の規則を設計書 3.11 の「lint の規則の構成」のとおりに改める。ブランド付きの 4 つの型と `CollectedInput` について §1.3 の表の「禁止する」の書き方をすべて禁止し、`storage.sync` を §1.3 の 4 つの書き方で禁止する。作り手のファイルごとの設定のオブジェクトは `...restrictedSyntax` を並べ直す。`test/helpers/settings.ts` を作り（§1.3「テストの補助」）、テストを作る。
    -   `settings.test.ts`・`prepare.test.ts`・`response.test.ts`・`sendMessages.test.ts`・`resultView.test.ts`: 設計書 3.12 の表の「対象」の各項目と、要件書の AC-02・AC-03・AC-07・AC-08・AC-16・AC-28 に挙がった入力の例をすべて含める。`parseResultView` は、`unexpected` の `status` の境界（99・100・599・600・整数でない値）を含める。`classifyResponse` は、`__proto__` などの名前のメンバ（設計書 3.12）を含める。
    -   `lintRules.test.ts`（「branded type assertions」「storage.sync」）: 作り手のファイルごとに、自身の型の表明が許され、ほかの 4 つの型の表明が禁止されること。`src/`・`scripts/` のほかのファイルでは 5 つのブランド付きの型への表明が禁止され、ほかの型への表明（既存の `as Record<string, unknown>` など）は禁止されないこと。テストのファイルでは 5 つの型への表明が許されること。§1.3 の表で「禁止する」とした書き方のそれぞれが違反になること。`chrome.storage.sync` の 4 つの書き方が違反になり、`chrome.storage.local` は違反にならないこと。作り手のファイルのそれぞれで、共通の選択子（`innerHTML` への代入、`import()`、`storage.sync`）が違反になること。
    -   **対象:** `extension/eslint.config.js`・`test/lintRules.test.ts`・`test/helpers/settings.ts`・`test/settings.test.ts`・`prepare.test.ts`・`response.test.ts`・`sendMessages.test.ts`・`resultView.test.ts`。**完了:** これらのテストと `make ext-lint` が通る。
-   [x] **ステップ 1-7**: 壊して失敗することを確かめ、コミットメッセージに記録する。括弧の中は、それを検出するテストである。
    -   `parseToken`・`parsePort` の各条件を 1 つずつ外す、`trim` を加える、`checkSettings` の判定の順序を入れ替える、ポートの欠落と不正を同じ扱いにする（`settings.test.ts`）。
    -   `prepareSend` の確認の順序を入れ替える、収集の拒否で `load` を呼ぶ、バイト数を `length` で数える、上限の比較を `<` にする、本文にタイトル・トークン・空の任意のメンバを入れる（`prepare.test.ts`）。
    -   `classifyResponse` の順 2〜9 の各判定を 1 つずつ外す、`fatal`・`ignoreBOM` を外す、`Object.hasOwn` を `in` に替える、`toSendOutcome` で `cause` を残す（`response.test.ts`）。
    -   2 つの理由の文言を同じにする、`unknown` の文言から Slack を確かめる案内を消す（`sendMessages.test.ts`）。
    -   `parseResultView` の各検査を 1 つずつ外す、`status` の範囲を 100〜599 から変える（`resultView.test.ts`）。
    -   lint の各選択子を 1 つずつ外す、作り手のファイルの例外を広げる・狭める、作り手のファイルの設定のオブジェクトから `...restrictedSyntax` を外す（`lintRules.test.ts`）。
    -   `runTsc` から `core-encoding.d.ts` を外す（`typecheck.test.ts` の加えた行）。`tsconfig.core.json` の `include` から外す（`make ext-typecheck`）。
    -   続けて `make ext-check` → `make test` → `make lint` を通す。**対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、ステップ 1-6 のテストが通る。

### PR-1 作成ポイント: core judgement layer and lint guards

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `feat(0020): add core send judgement layer and lint guards`

**レビュー観点**: ブランド付きの型で、確認を通った値だけが送信の入力になること / 応答を受理する形だけで解釈し、応答の本文を表示に渡さないこと / lint のブランド付きの型の禁止が作り手のファイルごとに正しい範囲であること / 既存の `collect`・`summary` の振る舞いを変えていないこと

**実装モデル要件**: frontier-recommended

**判定理由**: §1.3「ブランド付きの型の作り方の制約」が `SharedToken`・`ServerPort`・`PreparedRequest`・`RequestId` の作り方に「リーフパッケージへの移動」と「lint での禁止」の 2 案を挙げており、「競合する実装の案」の引き金に当たるため。ステップ 1-4 の応答の解析は複雑だが、設計書 3.6 が判定の順序と各検査を確定した決定的な処理で、回復の流れ・並行・状態機械ではない。

- [x] グリーンゲート（`_context.md` の "Green gate" に `make ext-check` を加えたもの。§3.2 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 送信と保存先

**対象ファイル**
-   新設: `extension/src/send.ts`、`extension/test/send.test.ts`、`extension/test/helpers/fetch.ts`
-   変更: `extension/src/browser/chromeDeps.ts`、`extension/src/launch.ts`（`ResultViewStore` の追加だけ）、`extension/test/chromeDeps.test.ts`

**タスク**
-   [x] **ステップ 2-1**: `send.ts` を設計書 3.5 のとおりに作る（`FetchResponse`・`FetchFn`・`SendDeps`・`sendTimeoutMs`・`sendArticle`）。`RequestInit` は設計書 3.5 の表の値とし、本文は `getReader()` で `maxResponseBytes` まで読み、超えたら `cancel()` する。`sendArticle` は例外を投げず、`fetch` を 1 回だけ呼ぶ。
    -   **対象:** `extension/src/send.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 2-2**: `chromeDeps.ts` を改める。`StorageArea` を複数のキーに広げ（`string | string[]`。`@types/chrome` の型をそのまま代入できる可変の配列とし、`readonly` は付けない。設計書 3.3）、コメント（`:12`）を、session と local の両方に使う型であるという説明に改める。`StorageChangeEvents`・`StorageChangeListener`（設計書 3.8）、`createSettingsStore(area)`（設計書 3.3 の「保存先」の表）、`createResultViewStore(area, events)`（設計書 3.8）、`SendDeps` の実装、`schedule` を加える。`SendDeps.fetch` は大域の `fetch` をそのまま入れず、`(url, init) => fetch(url, init)` で包む（`fetch` を別のオブジェクトのメソッドとして呼ぶと、ブラウザは `TypeError: Illegal invocation` を投げ、`sendArticle` はそれを「結果が分からない」にしてしまう）。`ResultViewStore` の interface を `launch.ts` に加える（フェーズ 4 の前提）。`SummaryStore`・`createSummaryStore` はフェーズ 4 で使う側を改めるまで残す（§1.3 の名前の台帳）。`chromeDeps.test.ts` の `memoryStorage` を、配列のキーも受け取るように改める（`StorageArea` の型の変更に合わせる）。
    -   **対象:** `extension/src/browser/chromeDeps.ts`・`src/launch.ts`・`test/chromeDeps.test.ts`。**完了:** `make ext-typecheck` と既存の `chromeDeps.test.ts` が通る。
-   [x] **ステップ 2-3**: `test/helpers/fetch.ts` を作り（§1.3「テストの補助」）、テストを作る。
    -   `send.test.ts`（「sendArticle」）: 設計書 3.12 の表の行。URL・`RequestInit` の各項目とヘッダの完全な集合、`fetch` の拒否（`TypeError`・`TimeoutError`）で `unknown` になり `fetch` が 1 回だけ呼ばれること、本文の読み込みの失敗、上限ちょうどの本文と 1 バイト超えた本文（超えたら `cancel` を呼び、それ以上読まない）、`body` が `null` の場合、`opaqueredirect` の場合に本文を読まないこと。`classifyResponse` が例外を投げた場合の `classification-error` は、`classifyResponse` がどの `ReceivedResponse` でも例外を投げない設計（設計書 3.6）のため、ユニットテストで再現できない。防御的な分岐として残し、未実行の分岐として記録する（`classifyResponse` を差し替えられる形にすると設計書 3.5 の `SendDeps` から外れるため、テストのためだけに依存を足さない）。
    -   `chromeDeps.test.ts`（「settings store」「result view store」）: 保存と消去が渡された保存先の決まったキーにだけ書き、もう片方のキーに触れないこと、`load` が 2 つのキーを 1 回の `get` で読むこと、消去の後の `load` でトークンが欠落すること。`get`・`remove` の動作と、`watch` が、`session` 以外の領域・別のキー・削除の通知をリスナに渡さないこと、停止した後に通知を渡さないこと。
    -   **対象:** `extension/test/helpers/fetch.ts`・`test/send.test.ts`・`test/chromeDeps.test.ts`。**完了:** これらのテストが通る。
-   [x] **ステップ 2-4**: 壊して失敗することを確かめ、コミットメッセージに記録する。
    -   `RequestInit` の各項目を 1 つずつ変える・ヘッダを 1 つ足す、`fetch` の拒否で再送する、本文を `arrayBuffer()` で読む、上限を超えても読み続ける、`opaqueredirect` で本文を読む、`classifyResponse` の例外を捕まえない（`send.test.ts`）。
    -   `createSettingsStore` が 2 つのキーを 1 つのオブジェクトに書く、`clearToken` がポートのキーも消す、`watch` の `areaName`・キー・`newValue` の各条件を 1 つずつ外す、`watch` の停止を何もしない関数にする（`chromeDeps.test.ts`）。
    -   続けて `make ext-check` → `make test` → `make lint` を通す。**対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`send.test.ts`・`chromeDeps.test.ts` が通る。

### PR-2 作成ポイント: send and storage dependencies

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `feat(0020): add article sending and storage dependencies`

**レビュー観点**: `fetch` を 1 回だけ呼び、どの結果でも再送しないこと / `RequestInit` の各項目とヘッダの集合が設計書 3.5 と一致すること / 応答の本文を上限まで読み、超えたら読むのをやめること / 設定と表示内容が、渡された保存先の決まったキーにだけ書かれること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（§1.3 に競合する実装の案がない）。送信の処理（ステップ 2-1）は設計書 3.5 が `RequestInit` の値・本文の読み方・時間の上限を確定しており、直線的な処理で、回復の流れ・並行・状態機械に当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" に `make ext-check` を加えたもの。§3.2 参照）がパスしていることを確認した
- [x] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: manifest とオプションページ

**対象ファイル**
-   新設: `extension/static/options.html`、`extension/src/optionsPage.ts`・`options.ts`、`extension/test/optionsPage.test.ts`、`extension/test/helpers/logger.ts`
-   変更: `extension/static/manifest.json`・`style.css`、`extension/src/background.ts`（コメントだけ）、`extension/scripts/check-dist.ts`・`has-extension-changes.sh`（`docs/dev/security.md` の追加だけ）、`extension/test/manifest.test.ts`・`checkDist.test.ts`・`ciChanges.test.ts`・`launch.test.ts`（`recordingLogger` の移動だけ）、`docs/dev/security.md`（§8.1 の `host_permissions` の項目だけ）

**タスク**
-   [ ] **ステップ 3-1**: `manifest.json` に設計書 3.1 の表の 3 項目を加える・改める。`background.ts:39-43` のコメントを、最小の版が Chrome 103 であることに合わせて改める。`manifest.test.ts` を設計書 3.12 の行のとおりに改める（「undeclared keys」の最上位のキーの集合、「host permissions」、「options page」、`minimum_chrome_version`、`permissions` が 4 つのまま）。テストの名前は §1.3 の名前の台帳に従う。
    -   **対象:** `extension/static/manifest.json`・`src/background.ts`・`test/manifest.test.ts`。**完了:** `manifest.test.ts` が通る。
-   [ ] **ステップ 3-2**: security.md §8.1 に、`host_permissions` の `http://127.0.0.1/*` とその理由を、「**要求する権限**」の一覧の外の別の項目として加える（§1.3「文書」）。及ぶ範囲などの残りの記述はステップ 5-3 で加える。`manifest.test.ts` の「host permissions」に、security.md §8.1 が `host_permissions` の各値を記していることの行を加える。この時点から node のテストが security.md を読むので、`has-extension-changes.sh` の起動の条件に `docs/dev/security.md` を加え（設計書 3.11 の 3 つのパスのうち 1 つ。残りはステップ 4-4）、`ciChanges.test.ts` の起動しない一覧から `["docs/dev/security.md"]` を除き、起動する一覧に加える。
    -   **対象:** `docs/dev/security.md`・`extension/test/manifest.test.ts`・`scripts/has-extension-changes.sh`・`test/ciChanges.test.ts`。**完了:** `manifest.test.ts`・`ciChanges.test.ts` と `make test`（`extension_docs_test.go` の §8.1 の権限の一覧）が通る。
-   [ ] **ステップ 3-3**: `optionsPage.ts`（設計書 3.10）、`options.ts`（`chrome.storage.local` の `createSettingsStore` で `runOptionsPage` を呼ぶ）、`static/options.html`（設計書 3.14）を作り、`style.css` にオプションページの書式を加える。`recordingLogger` を `test/helpers/logger.ts` に移し、`launch.test.ts` はそこから読む。`optionsPage.test.ts`（「runOptionsPage」）を設計書 3.12 の行のとおりに作る（保存・拒否・消去、AC-02・AC-03 の各入力で保存先が変わらないこと、トークンの 3 つの表示とポートの 3 つの状態、保存と読み込みの失敗、ログ・表示・欄にトークンが現れないこと、トークンの欄の `type`・`autocomplete`）。
    -   **対象:** `extension/src/optionsPage.ts`・`src/options.ts`・`static/options.html`・`static/style.css`・`test/helpers/logger.ts`・`test/launch.test.ts`・`test/optionsPage.test.ts`。**完了:** `optionsPage.test.ts` と `launch.test.ts` が通る。
-   [ ] **ステップ 3-4**: `check-dist.ts` の manifest の参照の検査に `options_ui.page` を加える。`checkDist.test.ts` の manifest の行と、その manifest を書く補助関数にオプションページの引数を加え、`options_ui.page` が `dist/` にない場合を拒否する行を加える（§1.3）。
    -   **対象:** `extension/scripts/check-dist.ts`・`test/checkDist.test.ts`。**完了:** `checkDist.test.ts` と `make ext-build` が通る。
-   [ ] **ステップ 3-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。
    -   `host_permissions` に `http://localhost/*` を足す・ポートを書く、`options_ui` を消す・`open_in_tab` を変える、`minimum_chrome_version` を `"102"` に戻す、security.md §8.1 から `http://127.0.0.1/*` を消す（`manifest.test.ts`）。
    -   `has-extension-changes.sh` から `docs/dev/security.md` を外す（`ciChanges.test.ts`）。
    -   トークンの保存で `parseToken` を外す、保存後に欄を空にしない、設定済みの表示にトークンの先頭を含める、ポートの拒否でトークンの保存も止める、消去でポートも消す、保存の失敗を `log.info` に出す（`optionsPage.test.ts`）。
    -   `check-dist` の `options_ui.page` の検査を外す（`checkDist.test.ts`）。
    -   続けて `make ext-check` → `make test` → `make lint` を通す。**対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`manifest.test.ts`・`optionsPage.test.ts`・`checkDist.test.ts`・`ciChanges.test.ts` が通る。

### PR-3 作成ポイント: manifest and options page

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0020): add manifest permissions and options page`

**レビュー観点**: `host_permissions` と `options_ui` が設計書 3.1 と一致し、security.md §8.1 と同期していること / トークンとポートの検証が補正をせず、拒否したときも保存先の既存の値を変えないこと / トークンの値が表示・ログ・欄の値の照合に出ないこと / `check-dist` が `options_ui.page` の参照も確かめること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（§1.3 に競合する実装の案がない）。`host_permissions` は `127.0.0.1` に限った固定の 1 項目で、設計書 3.1 が値を確定しており、段階的な公開や振る舞いの同時の上げ下げを伴わないので、セキュリティゲート・移行の引き金に当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" に `make ext-check` を加えたもの。§3.2 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 経路の処理と表示

**対象ファイル**
-   新設: `extension/src/ui/renderSend.ts`、`extension/test/renderSend.test.ts`・`serverReasons.test.ts`・`secretsAbsent.test.ts`
-   変更: `extension/src/ui/render.ts`、`extension/src/launch.ts`、`extension/src/browser/chromeDeps.ts`（旧名の削除）、`extension/src/background.ts`・`popup.ts`・`result.ts`、`extension/static/popup.html`・`style.css`、`extension/scripts/has-extension-changes.sh`、`extension/test/launch.test.ts`・`chromeDeps.test.ts`・`ciChanges.test.ts`

ステップ 4-2 から 4-6 までは、本番のコードの interface の変更とテストの更新を分けて書く。`make ext-typecheck` はテストも型検査するので（§1.3「型検査」）、ステップ 4-2・4-3 の完了は本番のコードだけを型検査する `tsc --noEmit -p tsconfig.build.json` で確かめ、`make ext-typecheck` はステップ 4-6 の後に通す。

**タスク**
-   [ ] **ステップ 4-1**: `ui/render.ts` の `rejectedNodes` の文言と手順の部分を `messageNodes` として公開し、`renderSummary` はそれを使い続ける（振る舞いは変えない。既存の `render.test.ts` を変更せずに通す）。`ui/renderSend.ts` に `renderSendOutcome`・`renderResultView` を設計書 3.9 のとおりに作り、`style.css` に送信の表示の書式（リクエスト ID の `user-select: all` など）を加える。`renderSend.test.ts`（「renderSendOutcome」「renderResultView」）を設計書 3.12 の行のとおりに作る（送信中の表示（文言と閉じないことの案内）も確かめる。AC-22）。
    -   **対象:** `extension/src/ui/render.ts`・`src/ui/renderSend.ts`・`static/style.css`・`test/renderSend.test.ts`。**完了:** `make ext-typecheck` と `render.test.ts`・`renderSend.test.ts` が通る。

### PR-4 作成ポイント: send result rendering

**対象ステップ**: 4-1

**推奨タイトル**: `feat(0020): render send outcomes and result views`

**レビュー観点**: 文言と手順の要素の作成を `messageNodes` に集約し、`renderSummary` の振る舞いを変えていないこと / リクエスト ID を文言とは別の要素に `user-select: all` で表示し、状態コードは想定外の応答にだけ表示し、リダイレクトには表示しないこと / 応答の本文の文字列がどの表示にも現れないこと / 送信中の表示に閉じないことの案内が含まれること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（表示の組み立てとそのテストに限られ、§1.3 に競合する実装の案がなく、分離すべき高リスク・複雑なステップもない）。

- [ ] グリーンゲート（`_context.md` の "Green gate" に `make ext-check` を加えたもの。§3.2 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

-   [ ] **ステップ 4-2**: `launch.ts` を設計書 3.8 のとおりに改める。`MenuLaunchDeps`・`ResultWindowDeps` の変更、`PopupSendDeps`・`runPopupSend`・`resultWaitMs` の追加、`runMenuLaunch`（失敗の扱いはシーケンス図とその下の箇条のとおり）、`runResultWindow`（状態図のとおり）、開発者向けのログ（設計書 3.8 の「開発者向けのログ」）。名前の台帳（§1.3）のとおり、`SummaryStore`・`take`・`missingResultText`・`renderNotice`・`createSummaryStore` を削り、ログのラベルを改め、`runPopupLaunch` の doc コメント（`:128-129`）から「#111's send button」を除く。
    -   **対象:** `extension/src/launch.ts`・`src/browser/chromeDeps.ts`。**完了:** なし（ステップ 4-3 の後に型検査する）。
-   [ ] **ステップ 4-3**: エントリポイントと `popup.html` を改める。`background.ts` はメニューの経路の依存に設定の読み込み（`chrome.storage.local`）・`SendDeps`・`now`・`createResultViewStore` を加える。`popup.html` に要約の領域と送信の領域を置き、`popup.ts` は前者で `runPopupLaunch`、後者で `runPopupSend` を呼ぶ。`result.ts` は `createResultViewStore(chrome.storage.session, chrome.storage.onChanged)` と `schedule` で `runResultWindow` を呼ぶ。
    -   **対象:** `extension/src/background.ts`・`src/popup.ts`・`src/result.ts`・`static/popup.html`。**完了:** `tsc --noEmit -p tsconfig.build.json` が通る。
-   [ ] **ステップ 4-4**: `has-extension-changes.sh` の起動の条件に `README.md`・`internal/server/response.go` を加え、冒頭のコメントの起動の条件の説明を改める（設計書 3.11。`docs/dev/security.md` はステップ 3-2 で加えた）。
    -   **対象:** `extension/scripts/has-extension-changes.sh`。**完了:** なし。`ciChanges.test.ts` はステップ 4-6 で改めるまで失敗する（フェーズの完了条件で確かめる）。
-   [ ] **ステップ 4-5**: `launch.test.ts` を改める。`recordingStore` を `ResultViewStore` の fake（`put`・`get`・`watch`・`remove` を記録し、通知を任意の時点で起こせ、各操作を失敗させられる）に置き換え、`menuHarness` に設定・`SendDeps`・`now` を加える。§1.3 に挙げたメニューの経路と結果のウィンドウの行を新しい振る舞いに合わせて改め、設計書 3.12 の `launch.test.ts` の行の項目を「launch paths」「menu send」「popup send」「result window」「token never leaks」に加える。ポップアップの送信中の表示（文言と閉じないことの案内、ボタンの `disabled`）と、送らない場合にボタンを出さないことを確かめる行を含める（AC-14・AC-22）。あわせて、設定の読み込みの失敗（`settings-unreadable`、AC-15a）と保存されたポートの形が正しくない場合（`port-invalid`、AC-15b）に、メニューの経路とポップアップの経路のどちらでも要求を送らず（`fetch` を呼ばず）理由が示されることを確かめる行を含める。
    -   **対象:** `extension/test/launch.test.ts`。**完了:** `launch.test.ts` が通る。
-   [ ] **ステップ 4-6**: 残りのテストを作る・改める。
    -   `serverReasons.test.ts`（「server reasons match README」）: `serverRejectStatus` の組が README の拒否の理由の表とちょうど一致すること（設計書 3.12）。表は `test/helpers/repoFile.ts` の `readRepoFile` で読む。
    -   `secretsAbsent.test.ts`（「no API key or Webhook settings」）: 設計書 3.12 の行の語が `src/`・`static/` のファイルに現れないこと。
    -   `ciChanges.test.ts`: 起動する一覧に `README.md`・`internal/server/response.go` をそれぞれ単独の一覧として加え、起動しない一覧から `["README.md"]` を除く。`:28` の行は `["docs/dev/project_overview.md", "extension/package.json"]` に替える。新しい 3 つのパスのそれぞれについて、先頭以外に現れる・後ろに続きを持つパス（`docs/README.md`・`README.md.bak`・`x/docs/dev/security.md`・`docs/dev/security.md.orig`・`x/internal/server/response.go`・`internal/server/response.go.orig`）と、同じディレクトリのほかのファイル（`docs/dev/project_overview.md`・`internal/server/handler.go`）が起動しない行を加える。
    -   `chromeDeps.test.ts`: 「summary store」の `describe` を消す（名前の台帳）。
    -   **対象:** `extension/test/serverReasons.test.ts`・`secretsAbsent.test.ts`・`ciChanges.test.ts`・`chromeDeps.test.ts`。**完了:** `make ext-typecheck` とこれらのテストが通る。
-   [ ] **ステップ 4-7**: 壊して失敗することを確かめ、コミットメッセージに記録する。
    -   `messageNodes` で手順のリストを作らない（`render.test.ts`・`renderSend.test.ts`）。想定外の応答の表示に状態コードを出さない、リダイレクトの表示に状態コードを出す、送信中の表示から閉じないことの案内を消す（`renderSend.test.ts`）。表示に `innerHTML` を使う（`make ext-lint`。値が 16 進や固定の文言に限られる要素はテストでは区別できない）。
    -   `runMenuLaunch` で送信中の `put` の失敗やウィンドウの失敗の後にも送る、`toSendOutcome` を通さずに `put` する、収集の拒否で設定を読む（`launch.test.ts::menu send`）。
    -   ポップアップを開いた時点で送る、送らない場合にもボタンを出す、送信中・送信後にボタンを押せるようにする、クリックの重複を状態で防ぐ仕組みを外す（`launch.test.ts::popup send`）。
    -   結果のウィンドウで `watch` と `get` の順を入れ替える、`Final` の後の通知で表示を変える、待ち時間の上限の予約を外す、`Final` で `remove` しない・予約を取り消さない、読めない場合を `log.info` に出す、`unreadableResultMessage` を「もう一度起動する」の文言にする（`launch.test.ts::result window`・`sendMessages.test.ts`）。
    -   ログにトークンを渡す（`launch.test.ts::token never leaks`）。
    -   `serverRejectStatus` の 1 つの組の状態コードを変える（`serverReasons.test.ts`）。`has-extension-changes.sh` から 2 つのパスを 1 つずつ外し、各パターンの先頭・末尾の一致をゆるめる（`ciChanges.test.ts`）。`src/` に `webhook` を含むコメントを 1 行足す（`secretsAbsent.test.ts`）。
    -   続けて `make ext-check` → `make test` → `make lint` を通す。最後に `node --test --experimental-test-coverage` の出力で、`core/`・`send.ts`・`launch.ts`・`optionsPage.ts`・`ui/renderSend.ts` に実行されない分岐が残っていないかを確かめ、残る分岐をコミットメッセージに挙げる。**対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、ステップ 4-5・4-6 のテストが通る。

### PR-5 作成ポイント: launch paths, entry points, and CI change detection

**対象ステップ**: 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7

**推奨タイトル**: `feat(0020): wire the launch paths to send and display results`

**レビュー観点**: ポップアップとメニューの経路が同じ `prepareSend`・`sendArticle` を使うこと / 結果のウィンドウの一方向の状態の移り方と、通知と `get` の順序に依存する送信結果の受け渡し / 送信中に表示できなければ送らず、送信の結果を `toSendOutcome` を通してから保存先に書くこと / ポップアップが 1 回だけ送り、送信中と送信後にボタンを押せない状態のままにすること / トークンがログ・保存先へ渡す値・表示のどこにも現れないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-2 の `runResultWindow` の状態機械と、通知と `get` の順序に依存する送信結果の受け渡しが、高リスク・複雑なステップであるため。この PR は送信の経路の処理に集中し、表示（ステップ 4-1）は PR-4 に分けて、状態機械のレビューを単純な表示の変更で妨げないようにしている。

- [ ] グリーンゲート（`_context.md` の "Green gate" に `make ext-check` を加えたもの。§3.2 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 手動の確認と文書

**対象ファイル**
-   変更: `README.md`、`docs/dev/security.md`、`docs/dev/project_overview.md`（`:98` の 1 行）、`CLAUDE.md`、`cmd/yt2column/extension_docs_test.go`、本計画（§5.1）

**タスク**
-   [ ] **ステップ 5-1**: 設計書 7.2 の手動の確認を、macOS の Chrome と Brave の作業時点の安定版で行い、§5.1 に記録する。記事の生成に至らない入力は §5.1 の表の「確認の内容」のとおりとし、LLM API と Slack Webhook を実際に呼ぶのは AC-34 の行だけとする。AC-34 の実行の前に利用者の承認を得る（CLAUDE.md「Tool Execution Safety」）。一時的なローカルのサーバ（`429` を返すもの・`302` を返すもの・応答しないもの）は、作業環境にある Node.js（`extension/.node-version` の版）の `node:http` で書き、作業ディレクトリの外に置き、リポジトリに含めない。
    -   **対象:** 本計画（§5.1）。**完了:** §5.1 の AC-30 以外のすべての行の欄が埋まっている。
-   [ ] **ステップ 5-2**: README を設計書 3.14 の `README.md` の行のとおりに改める（「Browser extension」の節と「The shared token」の節）。
    -   **対象:** `README.md`。**完了:** `make test`（README を読む Go のテスト）と `serverReasons.test.ts` が通る。
-   [ ] **ステップ 5-3**: security.md を設計書 3.14 の `docs/dev/security.md` の行のとおりに改める（見出しを変えず、§1.3「文書」の Go のテストの制約を守る）。`project_overview.md:98` の `extension/` の説明に送信を加える。CLAUDE.md の「Development Notes」を設計書 3.14 の `CLAUDE.md` の行のとおりに改め、同じ段落の `doc-guard` のコマンドを `ci.yml:184` に合わせる（§1.3「文書」）。
    -   **対象:** `docs/dev/security.md`・`docs/dev/project_overview.md`・`CLAUDE.md`。**完了:** `make test`（security.md・project_overview.md を読む Go のテスト）と `manifest.test.ts` が通る。
-   [ ] **ステップ 5-4**: 文書を事実と照合する。README の拡張の手順（オプションページの開き方、トークンの貼り付け、2 つの起動の手段からの送信）を、ステップ 5-1 で使ったビルドで上から実行して確かめる。security.md の権限・保存先・残るリスクの記述を設計書 3.1・3.3・5.4 と照合する。CLAUDE.md の記述を `has-extension-changes.sh` の条件と `ci.yml` の `doc-guard` のジョブと照合する。結果を §5.1 の AC-30 の行に記録する。
    -   **対象:** 本計画（§5.1）。**完了:** §5.1 のすべての行の欄が埋まっている。
-   [ ] **ステップ 5-5**: `cmd/yt2column/extension_docs_test.go` に `TestExtensionSendDocumentation` を加える（設計書 3.12）。既存の `readDoc`・`sectionByHeading`・`findTable`・`acLabel` を使う。確かめる内容は次のとおりである。照合する語句は実装で決め、文書の書きぶりに合わせる。
    -   README: 「Browser extension」の節が「never sends」を含まず、オプションページでのトークンの設定、2 つの起動の手段からの送信、「結果が分からない」の表示で Slack を確かめること、`requestId` での照合、`127.0.0.1` のサイトへのアクセスを含むこと。「The shared token」の節が「#111」を含まないこと。
    -   security.md: §8 の冒頭が「#109・#111 で決める」を含まず、タブのタイトルを送らないことを含むこと。§8.1 が「サーバへの送信は #111 で加える」を含まず、`chrome.storage.local`、トークンをログ・表示・本文に出さないこと、API キーと Webhook URL を持たないこと、自動で再送しないこと、応答の本文を表示しないことを含むこと。§8.2 が「確定は #111」を含まないこと。
    -   本計画の §5.1: 表の行の名前が §5.1 の表のとおりで、すべての欄が埋まっていること（0007 の検査と同じ形）。
    -   **対象:** `cmd/yt2column/extension_docs_test.go`。**完了:** `make fmt` の後に `make test` と `make lint` が通る。
-   [ ] **ステップ 5-6**: 壊して失敗することを確かめ、コミットメッセージに記録する。README に「never sends」を戻す、ステップ 5-5 の確かめる語句を含む文を README・security.md から 1 つずつ消す、§8.2 に「確定は #111」を戻す、§5.1 の 1 つの欄を空にする、§5.1 の 1 行を消す（いずれも `TestExtensionSendDocumentation`）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、§5.1 のすべての行が記録されている。

### PR-6 作成ポイント: manual verification and documentation

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6

**推奨タイトル**: `docs(0020): record manual verification and update send documentation`

**レビュー観点**: §5.1 の表のすべての欄が、実際の確認の結果で埋まっていること / Go のガード `TestExtensionSendDocumentation` が、README・security.md に残る #111 で決めるとした古い記述を検出すること / security.md の見出しと「**要求する権限**」の一覧を変えずに `host_permissions` を一覧の外に書いていること / CLAUDE.md の `doc-guard` のコマンドが `ci.yml` のジョブと一致すること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（手動の確認の記録、文書の更新、および既存の Go のガードと同じ形の検査に限られ、§1.3 に競合する実装の案がない）。手動の確認（ステップ 5-1）が実際のサーバ・Slack・LLM API を呼ぶのは人間の操作による受け入れの確認であり、自動テストや CI の外部資源の面ではないので、パネルモードの引き金に当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" に `make ext-check` を加えたもの。§3.2 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の条件 |
|---|---|---|
| M1: core | フェーズ 1 | `settings`・`prepare`・`response`・`sendMessages`・`resultView`・`lintRules`・`typecheck` のテストが通る |
| M2: 送信と保存先 | フェーズ 2 | `send`・`chromeDeps` のテストが通る |
| M3: manifest とオプションページ | フェーズ 3 | `manifest`・`optionsPage`・`checkDist`・`ciChanges` のテストが通り、`make ext-build` の成果物にオプションページが含まれる |
| M4: 経路の処理と表示 | フェーズ 4 | `launch`・`renderSend`・`serverReasons`・`secretsAbsent`・`ciChanges` のテストが通る |
| M5: 確認と文書 | フェーズ 5 | §5.1 の記録がそろい、`TestExtensionSendDocumentation` と `make test` が通る |

### 3.2. PR 構成

PR は、フェーズ 4 を 2 つに分け、そのほかはフェーズと 1 対 1 に対応させる（PR-1〜PR-6）。各 PR は主たる関心事（`core/` の判定 / 送信と保存先 / manifest とオプションページ / 送信の結果の表示 / 経路の処理とエントリポイント / 手動の確認と文書）を持ち、単独でグリーンゲートを通せる単位とする。PR の境界と実装モデル要件の根拠は、各フェーズの末尾の「PR-N 作成ポイント」に記す。

本タスクの各 PR のグリーンゲートは、`_context.md` の "Green gate"（`make test && make lint`）に `make ext-check` を加えた `make ext-check` → `make test` → `make lint`（各フェーズの完了条件と同じ）とする。`make test`・`make lint` は Go のソースだけを対象にし、拡張の TypeScript の型検査・lint・テストを確かめないためである。

フェーズ 4 では、送信の結果の表示（ステップ 4-1）を PR-4 に分ける。これは、送信の経路の処理（ステップ 4-2 の `runResultWindow` の状態機械）を、単純な表示の変更で妨げずにレビューできるようにするためである。依存の向きは、`ui/renderSend.ts`（ステップ 4-1）→ `launch.ts`（ステップ 4-2）→ エントリポイント（ステップ 4-3）である。ステップ 4-1 は `launch.ts` を変更しないので、PR-4 は単独でグリーンゲートを通せる。ステップの順序は変えていないので、ステップの番号も §2 のままである。フェーズ 1 の `core/` の 5 つのモジュールとそのテスト、lint の規則は、lint の規則が 5 つのブランド付きの型すべてを一度に扱い、テストがその規則の対象になるため、1 つの PR にまとめる。分けると、テストのないモジュールが先に出るか、対象のない lint の規則が先に出る。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | `core/` のブラウザに依存しない判定（設定の検証・本文の組み立て・応答の解釈・文言・表示内容の解析）とそのテスト、lint のブランド付きの型と `storage.sync` の禁止、型検査の宣言 | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 | `send.ts` の送信と応答の受け取り、`chromeDeps.ts` の保存先・`fetch` の実装、そのテスト | standard |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | manifest の権限とオプションページ、`optionsPage.ts`、`check-dist` の検査、security.md §8.1 の `host_permissions` の項目、そのテスト | standard |
| PR-4 | 4-1 | 送信の結果の表示（`ui/renderSend.ts`、`ui/render.ts` の `messageNodes` の公開）とそのテスト | standard |
| PR-5 | 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 | `launch.ts` の経路の処理と結果のウィンドウ、エントリポイント、CI の変更検出の条件、そのテスト | frontier-recommended |
| PR-6 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 | 手動の確認の記録、README・security.md・CLAUDE.md・project_overview.md の更新、Go の文書のガード | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。フェーズ 1 の `core/` は、ほかのすべてのフェーズが使う。フェーズ 2 は、フェーズ 4 の経路の処理が使う `ResultViewStore` の interface を先に加え、従来の `SummaryStore` を残すことで、フェーズ 2 だけでも既存の経路がそのまま動く状態を保つ。フェーズ 3 の `manifest.test.ts` が security.md §8.1 を読むので、§8.1 の `host_permissions` の項目と、`has-extension-changes.sh` の `docs/dev/security.md` の条件を、前提としてフェーズ 3 で加える。security.md の残りはフェーズ 5 で改める。手動の確認は、すべての経路がそろうフェーズ 5 で行う。

各フェーズのグリーンゲートは、`_context.md` の "Green gate"（`make test && make lint`）に `make ext-check` を加えた `make ext-check` → `make test` → `make lint` とする（0007 と同じ）。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   設計書 3.12・7.1 のとおり、`node --test` で、fake の依存と jsdom を使う。実際のブラウザも実際のサーバも起動しない。応答の本文は API の設計書 §3.4.2 の形から合成した文字列だけを使う。
-   判定の網羅は `core/` のテスト（`settings`・`prepare`・`response`・`sendMessages`・`resultView`）と `send.test.ts` で行う。`launch.test.ts` は経路の配線（同じ `prepareSend`・`sendArticle` を通ること、確認の時点、保存先と表示の失敗の扱い、状態の移り方）を確かめ、`core/` のテストの入力の表を繰り返さない。`renderSend.test.ts` は表示の手段（`textContent`、リクエスト ID・状態コードの要素）を確かめ、文言の区別は `sendMessages.test.ts` に任せる。
-   AC-32 は、`test/helpers/settings.ts` の識別できる 64 文字のトークンを使い、fake のロガー・保存先に書かれた値・描画した DOM の `textContent` を文字列にしてから、その値を含まないことを確かめる（設計書 3.12）。
-   `chromeDeps.ts` のうち `chrome` の API と大域の `fetch` をそのまま呼ぶ部分（`SendDeps` の実装、`schedule`）とエントリポイントは、0007 と同じくユニットテストせず、手動の確認で確かめる。`SendDeps.fetch` の包み方の誤り（ステップ 2-2）は、受け付けた応答を表示する AC-34 の手動の確認で確かめる。

### 4.2. 統合テスト

ブラウザとサーバを起動する自動テストは設けない（要件書 5.）。設計書 7.2 の手動の確認をステップ 5-1 で行い、§5.1 に記録する。

### 4.3. 後方互換性

-   収集の部分（`collect`・`SelectionReader`・`CollectedInput`・`renderSummary`・`rejectionMessage`）の振る舞いを変えない。既存の `collect.test.ts`・`summary.test.ts`・`render.test.ts`・`messages.test.ts` を変更せずに通すことで確かめる。
-   Go のソースを変更しない。security.md・README を読む既存の Go のテスト（§1.3「文書」）は、各フェーズの `make test` で通ることを確かめる。

### 4.4. 網羅率

ステップ 4-7 で、`node --test --experimental-test-coverage` の出力を見て、ユニットテストの対象のファイルに実行されない分岐が残っていないかを確かめる（0007 と同じ）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

ラベル: `test` は失敗しうる実行のテスト、`static` は設定・文書を確かめるテストや lint の規則、`manual` は手動の確認である。拡張のテストの場所は `extension/test/<file>::<describe の名前>`、Go のテストは `<path>::<関数名>` で示す。名前は計画上の名前である（§1.2）。「実装」の列は、実装のステップとテストを作るステップを挙げる。

| AC | ラベル | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test・manual | `optionsPage.test.ts::runOptionsPage`（保存と設定済みの表示、値を表示しない）、§5.1 | ステップ 1-2・3-3 |
| AC-02 | test | `settings.test.ts::parseToken`、`optionsPage.test.ts::runOptionsPage`（拒否した入力で保存先が変わらない） | ステップ 1-2・1-6・3-3 |
| AC-03 | test | `settings.test.ts::parsePort`、`optionsPage.test.ts::runOptionsPage` | ステップ 1-2・1-6・3-3 |
| AC-04 | test | `settings.test.ts::checkSettings`、`prepare.test.ts::prepareSend`（`PreparedRequest.url` が `:8765`） | ステップ 1-2・1-3・1-6 |
| AC-05 | test | `optionsPage.test.ts::runOptionsPage`（消去が `clearToken` を呼ぶ）、`chromeDeps.test.ts::settings store`（消去後の `load` でトークンが欠落）、`prepare.test.ts::prepareSend`（`token-missing`） | ステップ 1-3・1-6・2-2・2-3・3-3 |
| AC-06 | test・static | `chromeDeps.test.ts::settings store`、`lintRules.test.ts::storage.sync` | ステップ 1-6・2-2・2-3 |
| AC-07 | test | `prepare.test.ts::buildArticleBody` | ステップ 1-3・1-6 |
| AC-08 | test | `prepare.test.ts::buildArticleBody` | ステップ 1-3・1-6 |
| AC-09 | test | `prepare.test.ts::buildArticleBody` | ステップ 1-3・1-6 |
| AC-10 | test | `send.test.ts::sendArticle` | ステップ 2-1・2-3 |
| AC-11 | test | `send.test.ts::sendArticle` | ステップ 2-1・2-3 |
| AC-12 | test・manual | `send.test.ts::sendArticle`（`redirect: "manual"`、`opaqueredirect`）、`renderSend.test.ts::renderSendOutcome`、§5.1 | ステップ 2-1・2-3・4-1 |
| AC-13 | test・static・manual | `send.test.ts::sendArticle`（ヘッダの完全な集合）、`manifest.test.ts::host permissions`、§5.1 | ステップ 2-1・2-3・3-1 |
| AC-14 | test・manual | `prepare.test.ts::prepareSend`、`sendMessages.test.ts::blockMessage`、`launch.test.ts::menu send`・`popup send`（送信が呼ばれず理由を表示）、§5.1 | ステップ 1-3・1-5・1-6・4-2・4-5 |
| AC-15 | test | `settings.test.ts::checkSettings`、`prepare.test.ts::prepareSend` | ステップ 1-2・1-3・1-6 |
| AC-15a | test | `prepare.test.ts::prepareSend`、`sendMessages.test.ts::blockMessage`、`renderSend.test.ts::renderResultView`、`launch.test.ts::menu send`・`popup send`（設定の読み込み失敗で送信が呼ばれない） | ステップ 1-3・1-5・1-6・4-1・4-2・4-5 |
| AC-15b | test | `settings.test.ts::checkSettings`、`sendMessages.test.ts::blockMessage`、`renderSend.test.ts::renderResultView`、`launch.test.ts::menu send`・`popup send`（保存されたポートの形が正しくないときに送信が呼ばれない） | ステップ 1-2・1-5・1-6・4-1・4-2・4-5 |
| AC-16 | test | `prepare.test.ts::prepareSend`（多バイト文字で 4,194,304 と 4,194,305）、`sendMessages.test.ts::blockMessage`、`renderSend.test.ts::renderResultView` | ステップ 1-3・1-5・1-6・4-1 |
| AC-17 | test | `prepare.test.ts::prepareSend`（`load` を呼ばない）、`launch.test.ts::menu send`・`popup send`（送信も読み込みもしない） | ステップ 1-3・1-6・4-2・4-5 |
| AC-18 | test・manual | `launch.test.ts::menu send`、§5.1 | ステップ 4-2・4-5 |
| AC-19 | test | `launch.test.ts::popup send` | ステップ 4-2・4-5 |
| AC-20 | test | `launch.test.ts::popup send` | ステップ 4-2・4-5 |
| AC-21 | test | `launch.test.ts::launch paths`（同じ本文・同じ結果の分類） | ステップ 4-2・4-5 |
| AC-22 | test・manual | `renderSend.test.ts::renderResultView`（送信中の表示）、`launch.test.ts::popup send`（送信中の表示とボタン）、§5.1 | ステップ 4-1・4-2・4-5 |
| AC-23 | test・manual | `response.test.ts::classifyResponse`、`renderSend.test.ts::renderSendOutcome`、§5.1 | ステップ 1-4・1-6・4-1 |
| AC-24 | test・manual | `response.test.ts::classifyResponse`、`sendMessages.test.ts::serverRejectMessage`、`serverReasons.test.ts::server reasons match README`、§5.1 | ステップ 1-4・1-5・1-6・4-6 |
| AC-25 | test | `response.test.ts::classifyResponse`、`renderSend.test.ts::renderSendOutcome` | ステップ 1-4・1-6・4-1 |
| AC-26 | test | `send.test.ts::sendArticle`、`sendMessages.test.ts::sendOutcomeMessage` | ステップ 1-5・1-6・2-1・2-3 |
| AC-27 | test・manual | `send.test.ts::sendArticle`（`fetch` の `TypeError` で `unknown`）、`renderSend.test.ts::renderSendOutcome`（`unknown` の表示）、§5.1 | ステップ 2-1・2-3・4-1 |
| AC-28 | test | `response.test.ts::classifyResponse`、`renderSend.test.ts::renderSendOutcome` | ステップ 1-4・1-6・4-1 |
| AC-29 | test | `renderSend.test.ts::renderSendOutcome` | ステップ 4-1 |
| AC-30 | static・manual | `cmd/yt2column/extension_docs_test.go::TestExtensionSendDocumentation`、§5.1（ステップ 5-4 の照合） | ステップ 3-2・5-2・5-3・5-4・5-5 |
| AC-31 | test | `manifest.test.ts::permissions`・`undeclared keys`・`host permissions`・`options page` | ステップ 3-1・3-2 |
| AC-32 | test | `launch.test.ts::token never leaks`、`optionsPage.test.ts::runOptionsPage`、`resultView.test.ts::parseResultView`（`unknown` の `cause` を拒否） | ステップ 1-5・1-6・3-3・4-2・4-5 |
| AC-33 | static | `secretsAbsent.test.ts::no API key or Webhook settings` | ステップ 4-6 |
| AC-34 | test・manual | `launch.test.ts::launch paths`（fake の `FetchFn` での 2 つの経路の送信）、§5.1（実際のサーバと Slack） | ステップ 4-2・4-5・5-1 |

### 5.1. 手動の確認と記録

実装のときに埋める。確認の内容は設計書 7.2 の表のとおりで、記事の生成に至らない入力を「確認の内容」に記す。`TestExtensionSendDocumentation`（ステップ 5-5）が、各行の欄が埋まっていることを確かめる。このテストは空の欄があると失敗するので、すべての欄を埋めた後（ステップ 5-4 の後）に加える。

| AC | 確認の内容（設計書 7.2） | ブラウザ・環境 | 日付 | 結果 |
|---|---|---|---|---|
| AC-01 | トークンを保存し、オプションページを開き直すと「設定済み」だけが表示される | | | |
| AC-13 | 字幕の書式に合わない選択範囲（`invalid_transcript`）で送り、サーバのログで `Origin` が `chrome-extension://clfmbbcdpnjcefbdihdoahomaifbabkk` で、`OPTIONS` が届いていないことを、Chrome と Brave のそれぞれで確かめる | | | |
| AC-14 | トークンを消去した状態で、両方の経路で理由と手順が表示される | | | |
| AC-18・AC-22・AC-23 | 字幕の書式に合わない選択範囲（`invalid_transcript`）で、メニューの経路で送信中の後に送信の結果とリクエスト ID が表示され、ポップアップで送信中の表示とボタンが押せないことを確かめる。受付（`202`）の表示は AC-34 の実行のときに記録する | | | |
| AC-24 | `unauthorized`（オプションページに別の 64 文字を保存する）・`invalid_transcript`（実際のサーバ）と、`too_many_requests`（一時的なサーバ）の表示 | | | |
| AC-27 | サーバを起動していない状態で、両方の経路で「結果が分からない」 | | | |
| AC-34 | 視聴中の YouTube ページで字幕を選択し、両方の経路から送ると、記事が Slack に投稿される（Chrome。利用者の承認を得てから行う） | | | |
| AC-12 | `302` を返す一時的なサーバで、リダイレクト先に要求が届かず、リダイレクトの表示になる | | | |
| 設計書 3.5 | 応答しない一時的なサーバに、メニューの経路から送る。25 秒後に「結果が分からない」が表示され、記録が残る | | | |
| 設計書 5.4 | 1〜4 MiB の選択範囲を誤ったトークンで両方の経路から送ったときの表示（`unauthorized` か「結果が分からない」か） | | | |
| 設計書 3.10 | トークンを保存したとき、Chrome と Brave がパスワードの保存を勧めない | | | |
| AC-30 | ステップ 5-4 の文書の照合 | | | |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| service worker が 30 秒を超える `fetch` で止まる挙動を、作業時点の Chrome・Brave で確かめていない（設計書 3.5） | メニューの経路で、時間の上限の前に service worker が止まる | 設計書 3.8 の結果のウィンドウの待ち時間の上限で「結果が分からない」を表示する。ステップ 5-1 で確かめ、§5.1 に記録する。設計書の前提と違う結果なら、作業を止めて利用者に報告する |
| Node.js の `ReadableStream`・`TextDecoder` の振る舞いが、ブラウザの `fetch` の応答と異なる | ユニットテストが通っても、ブラウザで本文の読み込みや復号が違う | `send.ts` と `classifyResponse` が使うのは WHATWG の標準の API（`getReader`・`cancel`・`TextDecoder` の `fatal`・`ignoreBOM`）だけにする。ステップ 5-1 の手動の確認（受付・拒否・`302`）で、実際のブラウザでの振る舞いを確かめる |
| jsdom がパスワードの欄の保存の提案や `user-select` を再現しない | 表示のテストで確かめられない項目がある | 要素の属性（`type`・`autocomplete`・クラス）だけをユニットテストで確かめ、実際の振る舞いはステップ 5-1 の手動の確認で確かめる |
| ブランドの型の値を、型の付かない値（`any`）の代入で作れる（§1.3 の表の最後の行）。型の引数で戻り値の型を決める関数（`f<RequestId>()`）や `Object.assign` など、型の表明を使わない抜け道も同じ扱いとする | `prepareSend` を通らない値を `sendArticle` に渡すコードを書けてしまう | 受け入れる。設計書 3.2 の lint の禁止は型の表明を対象にする。型情報を使う lint の導入は本タスクの範囲の外とする |
| ステップ 3-2 から 5-3 までの間、CLAUDE.md の「文書だけの変更は `make ext-test` を要しない」の記述と、security.md・README を読む node のテストが食い違う | その間に security.md・README だけを変える PR で、node のテストを手元で実行せず、CI で初めて失敗に気づく | CI では、`has-extension-changes.sh` の条件（ステップ 3-2・4-4）で拡張のジョブが起動する。CLAUDE.md の更新はステップ 5-3 で行う（設計書 8 章の順序） |
| `serverReasons.test.ts` が README の表の書式に依存する | README の書式の変更で、拡張の挙動と関係なくテストが失敗する | Go のテスト（`cmd/yt2column-server/docs_test.go`）と同じ表の読み方にし、README の変更の PR で拡張のジョブを起動する（ステップ 4-4） |

### 6.2. スケジュールのリスク

| リスク | 対策 |
|---|---|
| 設計書の再承認（§1.3「文書」）を待つ | 本計画の承認の前に、設計書の変更（1.3・3.12・3.14・7.3・7.4）を同時にレビューしてもらう |
| 手動の確認（ステップ 5-1）は、利用者のブラウザでの操作と、AC-34 での LLM API・Slack の実際の呼び出しの承認を必要とする | 文書の作業（ステップ 5-2・5-3）を先に進め、確認の結果を待つ間に止めない |

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4）
-   [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1）
-   [ ] PR-5 マージ済み（対象ステップ: 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7）
-   [ ] PR-6 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6）
-   [ ] §5 のすべての AC の検証が通り、§5.1 の記録がそろっている

## 8. 成功基準 (Success Criteria)

-   **機能:** §5 の `test`・`static` の検証がすべて通る。§5.1 のすべての行が記録されている。
-   **品質:** `make ext-check`・`make test`・`make lint` が通る。各テストと lint の規則を壊して失敗することを確かめ、コミットメッセージに記録している。§4.4 の網羅率を確かめている。
-   **セキュリティ:** AC-06・AC-09・AC-25・AC-29・AC-31・AC-32・AC-33 の検証が通る。ブランド付きの型の禁止（ステップ 1-6）が lint で効いている。
-   **文書:** ステップ 5-4 の照合を終え、`TestExtensionSendDocumentation` と `make test` が通る。

## 9. 次のステップ (Next Steps)

-   設計書の変更（§1.3「文書」）と本計画をレビューし、設計書を再承認した後に、本計画を `approved` にする。
-   `/runplan 0020` で PR-1 から順に実装する。
