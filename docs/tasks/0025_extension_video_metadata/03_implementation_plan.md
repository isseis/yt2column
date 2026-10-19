# 実装計画書：ブラウザ拡張による動画タイトル・チャンネル名・概要欄の取得と送信

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-18 |
| Review date | 2026-10-18 |
| Reviewer | isesis |
| Comments | 編集上の訂正（2026-10-18。判断は変えていない）。フェーズ 2 の対象ファイルに `extension/src/launch.ts` を加えた（ステップ 2-3 が既に `MetadataReader` を launch.ts の型としているため、フェーズ 2 の作業内容は変えず、対象の記載を合わせただけである）。ステップ 2-4・2-5 の確かめる手段を `document.body.outerHTML` から jsdom の直列化（`dom.serialize()`）に改めた（`outerHTML` は拡張の ESLint の `no-restricted-syntax` が許さないため。何を確かめるかは変えていない）。フェーズ 3 の対象ファイルに `extension/test/render.test.ts` と `extension/test/helpers/dom.ts` を加え、ステップ 3-2・3-3 に表示の上限の export と値のクラス名を記した（実装で決めるとしていた詳細を実行した結果の記載である。判断は変えていない） |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定義した、ブラウザ拡張が動画ページの DOM から動画タイトル・チャンネル名・概要欄を読み、API の任意フィールド（`title`・`channelName`・`description`）として送る部分を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。具体的には、次のことを行う。

-   DOM も `chrome.*` も参照しない受理の判定と値の省略を、`extension/src/core/metadata.ts` に作る（設計書 3.2）。
-   ページの読み取りの注入（`readPageMetadata`）と、その依存の interface（`MetadataReader`）の実装を `extension/src/browser/chromeDeps.ts` に作る（設計書 3.3）。
-   ポップアップの表示に動画メタデータと促しのメッセージを加える（設計書 3.6）。
-   メニューとポップアップの経路にメタデータの読み取りを組み込み、送る前の確認（`prepareSend`）へ渡す（設計書 3.4・3.5）。
-   省略の理由と読み取りの失敗を開発者向けのログに残す（設計書 3.7）。
-   Chrome と Brave で手動の確認を行い、結果を本計画（§5.1）に記録する。README・`docs/dev/security.md` を改める（要件書 F-005）。

本計画が使う記号は次のとおりである。AC-NN は要件書の受け入れ基準、F-NNN は要件書の機能要件、H-NN は [design_handoff.md](design_handoff.md)、I-NN は [implementation_handoff.md](implementation_handoff.md) の項目を指す。「§N」は本計画の節、「設計書 N」は設計書の節を指す。

本計画は、設計書 8 章の実装優先順位 1〜4 を次のようにフェーズへ対応させ、PR を 5 つにする（§3.2）。優先順位 1→フェーズ 1、2→フェーズ 2、3→フェーズ 3・4（表示と経路の 2 つに分ける）、4→フェーズ 5。

### 1.2. 実装原則

-   設計書 1.1 の設計原則に従う。特に、DOM に依存する判定を `core/` の純粋な関数に置くこと、ページの読み取りの結果を信頼できない入力として扱い、定めた受理の形に合うものだけを受け入れること、取得の失敗を起動の失敗にも拒否の理由にもしないこと、ページを操作しないこと、untrusted テキストを `textContent` だけで表示し本文へは JSON の文字列として入れることを守る。
-   Go のソースを変更しない（設計書 1.3）。本タスクは拡張の TypeScript のソース・テスト・`static/style.css` と、README・`docs/dev/security.md`・`docs/translation_glossary.md` だけを変更する。manifest と権限は変更しない（AC-19）。
-   拡張のソースのコメント・識別子・文字列リテラル・利用者に表示する文言は英語で書く（`_context.md` の「Source-language rule」、CLAUDE.md）。`AC-NN`・`F-NNN`・`H-NN`・`I-NN` はソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
-   テストの名前（`describe` の名前）は計画上の名前である。実装で変える場合は、§5 を同じコミットで更新する。
-   各テストと lint の規則は、対象の処理を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」）。壊す対象は各フェーズの最後のステップに挙げる。
-   各フェーズの完了条件は、`make ext-check` → `make test` → `make lint` が通ることである（§3.2）。ただしステップ 4-1・4-2 は、テストを更新する前なので `make ext-check` が通らない。本番のコードだけを型検査する `tsc --noEmit -p tsconfig.build.json`（`extension/` で実行）で確かめる（0020 の計画と同じ）。このコマンドは本計画の作成時に実行し、HEAD `0842685` で成功することを確かめた。

### 1.3. 既存コード調査結果

HEAD `0842685` で確認した。設計書は既存のファイルの行番号をコミット `c93a87b` で記している。`git diff --name-only c93a87b HEAD -- extension README.md docs/dev cmd .github Makefile` の出力は `cmd/yt2column/integration_slack_test.go`・`cmd/yt2column/integration_test.go`・`docs/dev/developer_guide/package_reference.md` の 3 件だけで、`extension/`・`README.md`・`docs/dev/security.md` は変わっていない（2026-10-18 に実行）。したがって設計書の `file:line` は HEAD `0842685` でもそのまま有効である。以下の `file:line` も HEAD `0842685` のものである。

**再利用する既存の実装（変更しない、または振る舞いを変えずに広げるもの）**

-   `ArticleMetadata`（`extension/src/core/prepare.ts:12-17`）は、`exactOptionalPropertyTypes: true` の下で `title`・`channelName`・`description` を `string | undefined` として持つ。設計書 3.2 の `VideoMetadata` はこの型の別名にする。同じ形を宣言し直さない。
-   `buildArticleBody`（`extension/src/core/prepare.ts:32-49`）は既に `ArticleMetadata` の非空のメンバを本文へ入れる（0020 で実装済み）。本タスクは変更しない。既存の `prepare.test.ts`（`:73-105`）が、非空のメンバが入ること・空と欠落のメンバが入らないこと・タブのタイトルが入らないことを既に確かめている。本計画はこの網羅を繰り返さず、足りない行（AC-08・AC-21 と I-01）だけを加える。
-   `prepareSend`（`extension/src/core/prepare.ts:95-124`）は既に第 3 引数 `metadata: ArticleMetadata` を受け取る。本タスクは変更せず、`result.values` を渡すだけである（設計書 3.4）。本文の大きさの判定（`maxBodyBytes`。`:52`）は 1 か所のままで、メタデータを含めた本文に適用する（I-01）。
-   `whitespaceOnly`（`extension/src/core/collect.ts:43`、`/^\p{White_Space}*$/u`）は、0007 の `collect` が選択範囲とタブのタイトルの空判定に使う。設計書 3.2 は `acceptMetadata` の空白のみの判定を「同じ規則」とするので、本計画はこの定数に `export` を付け、`metadata.ts` から import して再利用する（同じ正規表現を書き直さない）。`collect` の振る舞いは変わらない。
-   `field`（`extension/src/ui/render.ts:19-38`）・`displayValue`（`:9-11`）・`messageNodes`（`:69-88`）・`ellipsis`（`:6`）は `render.ts` の中にある。設計書 3.6 の `renderMetadata` は `render.ts` に置くので、これらをそのまま再利用する（同じ組み立てを書き直さない）。
-   `truncate`（`extension/src/core/summary.ts:26-35`）はコードポイント単位で切り詰め、サロゲートペアを割らない。`renderMetadata` の表示の切り詰めに再利用するため、`export` を付ける（振る舞いは変えない。設計書 3.6）。
-   `attempt`（`extension/src/launch.ts:102-112`）は失敗を記録して投げない補助で、本タスクの経路の処理でも使える。`Logger`（`:23-26`）・`ResultViewStore`（`:29-39`）はそのまま使う。
-   `createSelectionReader`（`extension/src/browser/chromeDeps.ts:78-94`）は、注入の結果を実行時に検査する実装の手本である。`createMetadataReader` は同じ形にする（設計書 3.3）。

**型と interface の変更（振る舞いは変えない。設計書 3.8）**

本計画で interface の形を変えるものは、この表を正とする。タスクの本文と §5 はこの表に合わせる。

| 種類 | 旧 | 新 | 場所 | ステップ |
|---|---|---|---|---|
| interface | `ScriptingApi.executeScript({ func: () => PageSelection })` | `executeScript<Args, Result>({ func, args })`（`args` 付きの総称型） | `extension/src/browser/chromeDeps.ts:7-12` | 2-1 |
| 関数の戻り値 | `runPopupLaunch(): Promise<CollectOutcome>` | `Promise<PopupLaunchResult>` | `extension/src/launch.ts:245-260` | 4-1 |
| 関数の引数 | `runPopupSend(container, outcome: CollectOutcome, deps)` | `runPopupSend(container, result: PopupLaunchResult, deps)` | `extension/src/launch.ts:267-308` | 4-1 |
| interface | `RejectionMessage`（独立の interface） | `Message` の別名（`Message` を新設） | `extension/src/core/messages.ts:3-8` | 1-1 |
| 関数の引数 | `messageNodes(doc, message: RejectionMessage)` | `messageNodes(doc, message: Message)` | `extension/src/ui/render.ts:69-88` | 3-2 |
| 定数 | `whitespaceOnly`（非公開） | `whitespaceOnly`（`export`） | `extension/src/core/collect.ts:43` | 1-2 |
| 関数 | `truncate`（非公開） | `truncate`（`export`） | `extension/src/core/summary.ts:26-35` | 3-1 |

新設する名前（`PageMetadata`・`VideoMetadata`・`MetadataField`・`OmissionReason`・`MetadataOmission`・`MetadataCheck`・`MetadataRead`・`MetadataResult`・`parsePageMetadata`・`acceptMetadata`・`readFailureResult`・`emptyMetadataResult`・`MetadataReader`・`readMetadata`・`PopupLaunchResult`・`renderMetadata`・`expandPromptMessage`・`metadataLogLabel`）は設計書 3.2・3.3・3.5・3.6・3.7 のとおりである。既存の名前を削除・改名するものはない。

**既存のテストで更新が必要なもの**

-   `extension/test/launch.test.ts` は `runPopupLaunch` の戻り値と `runPopupSend` の第 2 引数を使う（`:354`・`:386`・`:391`・`:428`・`:445`・`:471`・`:650`・`:659`・`:673`・`:683`・`:702`・`:736`・`:929`・`:936`・`:956`）。`menuHarness`（`:248-275`）と `popupHarness`（`:283-292`）に `metadataReader` を加える。`launch paths`・`menu send`・`popup send` の行を新しい振る舞いに合わせ、`video metadata log` の行を加える（§2 フェーズ 4）。
-   `extension/test/chromeDeps.test.ts` の `Injection` 型（`:18`）は `Parameters<ScriptingApi["executeScript"]>[0]` で、`executeScript` を総称型にすると引数の型が変わる。`recordingScripting`（`:26-39`）と合わせて改める（ステップ 2-1）。`createMetadataReader` と注入関数の行を加える（ステップ 2-4）。
-   `extension/test/prepare.test.ts` は既存の本文の行（`:73-105`）を維持し、AC-08・AC-21・I-01 の行だけを加える（ステップ 4-4）。
-   `extension/test/messages.test.ts`・`extension/test/render.test.ts`・`extension/test/summary.test.ts`・`extension/test/sendMessages.test.ts`・`extension/test/renderSend.test.ts` は、`Message` の別名化と `messageNodes` の引数の変更の後もそのまま通る（`RejectionMessage` は `Message` の別名なので型の実質は変わらない）。編集は不要である（設計書 3.9 の「変更」は型の一般化を指す）。
-   `extension/test/manifest.test.ts` の権限の検査は変更しない（AC-19）。ステップ 5-5 で、Markdown の節を切り出す補助 `section` を `metadataDocs.test.ts` と共用するため `extension/test/helpers/repoFile.ts` の `markdownSection` へ移した。移すときに、見出しを行全体で照合し、レベル 1〜3 の見出しで節を終える規則に改め、その規則を `extension/test/markdownSection.test.ts` で確かめる（権限の検査の内容は変えていない）。

**既存のテストの入力の網羅（重複を避ける）**

-   `metadata.test.ts` が受理の形と省略を網羅する。`launch.test.ts` は経路の配線（同じ `prepareSend` を通ること、読み取りの失敗でも送信に進むこと、ログの内容）を確かめ、`metadata.test.ts` の入力の表を繰り返さない。
-   `renderMetadata.test.ts` は表示の手段（`textContent`、要素にならないこと、`dir`、促しの出る条件）を確かめ、文言の区別は `messages.test.ts` に任せる。

**文書**

-   README の「Browser extension」の節は `README.md:442` から始まる。Go のガード `TestExtensionSendDocumentation`（`cmd/yt2column/extension_docs_test.go:158-249`）が、この節が「never sends」を含まず、`shared token`・`Options`・`Context menu`・`Toolbar icon`・`check Slack`・`requestId`・`Site access`・`127.0.0.1` を含むこと、`docs/dev/security.md` §8.1 が `#111` を含まず `chrome.storage.local`・`開発者向けのログ`・`API キー`・`Webhook URL`・`再送しない`・`本文の文字列` を含むことを確かめる。本タスクの文書の変更は、これらの語句を保った追加とする。
-   `docs/dev/security.md` §8.1 の見出し「### 8.1. ブラウザ拡張（#110）」と「**要求する権限**」の一覧は、Go のガード `TestExtensionDocumentation`（`cmd/yt2column/extension_docs_test.go:85-115`。一覧は `declaredPermissions` が読む）が読む。`extension/test/manifest.test.ts:57-78` は §8.1 のホストの権限の行だけを読む。見出しと一覧を変えず、新しい項目を別の箇条として加える。
-   `docs/translation_glossary.md` には、動画タイトル・チャンネル名・概要欄・動画メタデータ・促しのメッセージの訳語がない（2026-10-18 に確認）。要件書 6. の「用語は translation_glossary.md と統一する」に従い、ステップ 5-4 で加える。これは `make lint`・`make test` では検出できないため、§4.4 の横断検索で確かめる。
-   `extension/scripts/has-extension-changes.sh` は既に `README.md` と `docs/dev/security.md` を起動の条件に含む（`:17-20`）。本タスクの文書の変更で拡張のジョブが起動するので、この条件は変更しない。

**AC-18 の静的な確認の置き場所（本計画の決定）**

-   設計書 7.4 は AC-18 の確かめ方を「静的な確認」とするが、実行する検査を特定していない。設計書 1.3 は「テスト」の変更を本タスクの範囲に含む。そこで、本計画は AC-18 の検査を拡張の新しいテスト `extension/test/metadataDocs.test.ts` に置く（README・`docs/dev/security.md` の記載を `readRepoFile` で読んで確かめる）。`extension/test/serverReasons.test.ts` が README を読むのと同じ形である。
-   本計画では、次の編集上の訂正を同じ変更で行った（設計書の `Comments` に記録した）。`metadataDocs.test.ts` と `core/collect.ts` の `whitespaceOnly` の `export`、および `docs/translation_glossary.md`（用語の追加）を設計書に加え、AC-09・AC-10・AC-18・AC-19 の確かめ方を、実行される検査に合わせて設計書 2.1・3.8・3.9・3.11・7.4 に反映した。設計の判断（何を確かめるか）は変えず、確かめる手段を実行される検査に定めるだけである（`requirements_process.md`「Editing an approved document」の編集上の訂正）。
-   計画だけの変更（`docs/` のみ）では拡張のジョブは起動しないが、AC-18 の対象（README・`docs/dev/security.md`）を変えるときは起動するので、検査は対象の変更とともに実行される。

**I-01（読み取りの結果の大きさの上限）への対応**

-   読み取りの境界に独立した大きさの上限を設けない（要件書 §2.3・§3.2 の決定どおり）。値の大きさは 0020 の本文の上限（`maxBodyBytes` の 4,194,304）に含め、本文が上限を超える場合は既存の `body-too-large` の扱いに委ねる。上限の判定は `prepareSend` の 1 か所だけに置く。
-   この方針の検証を、ステップ 4-4 の `prepare.test.ts`（大きなメタデータを含めた本文が上限を超えると `body-too-large`）と `metadata.test.ts`（`acceptMetadata` が大きな値でも省略しない）、`launch.test.ts`（大きな値でも起動が成功し、`body-too-large` の既存の表示になる）に置く。

### 1.4. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-05 は、すべて設計書 3.10 に採った方針が記録されている。本計画は設計書のその方針に従う（H-01・H-02 はフェーズ 2、H-03 はフェーズ 4、H-04 はフェーズ 3、H-05 はフェーズ 4）。
-   `implementation_handoff.md` の I-01 は、§1.3「I-01 への対応」のとおりに扱う（読み取りの境界に上限を設けず、本文の上限の 1 か所に委ねる）。検証はステップ 1-4・4-4 のテストに置く。
-   `implementation_handoff.md` の I-02 は、§1.3 の再利用する既存の実装（`truncate` に `export` を付けて再利用し、`…` の付加は `displayValue` を使う）のとおりに扱う。実装はステップ 3-1・3-2、検証はステップ 3-4・3-5 に置く。
-   当てはまらない項目はない。

## 2. 実装ステップ (Implementation Steps)

各ステップの末尾に **対象:**（そのステップが触るファイル）と **完了:**（そのステップの検証）を記す。フェーズの「完了条件」は、そのフェーズのすべてのステップの後に確かめる。設計書の節を参照し、設計の詳細を本計画に書き写さない。

### フェーズ 1: core の判定層

**対象ファイル**
-   新設: `extension/src/core/metadata.ts`、`extension/test/metadata.test.ts`
-   変更: `extension/src/core/messages.ts`、`extension/src/core/collect.ts`（`whitespaceOnly` の `export` のみ）

**タスク**
-   [x] **ステップ 1-1**: `core/messages.ts` に `Message` interface（`text` と `steps`）を加え、`RejectionMessage` をその別名にする。`expandPromptMessage: Message` を加える（文言と手順は設計書 3.6 の例に従い、README の手順と合わせる）。`rejectionMessage` の戻り値の型は `RejectionMessage` のままとし、振る舞いを変えない（設計書 3.6・3.8）。
    -   **対象:** `extension/src/core/messages.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 1-2**: `core/collect.ts` の `whitespaceOnly`（`:43`）に `export` を付ける。振る舞いは変えない。
    -   **対象:** `extension/src/core/collect.ts`。**完了:** `make ext-typecheck` と既存の `collect.test.ts` が通る。
-   [x] **ステップ 1-3**: `core/metadata.ts` を設計書 3.2 のとおりに作る（`PageMetadata`・`VideoMetadata`・`MetadataField`・`OmissionReason`・`MetadataOmission`・`MetadataCheck`・`MetadataRead`・`MetadataResult`・`parsePageMetadata`・`acceptMetadata`・`readFailureResult`・`emptyMetadataResult`）。`parsePageMetadata` の検査の順序は設計書 3.2 の 1〜5 のとおりとし、重複の検出は引用符の数ではなく、文字列とエスケープを追う小さな走査で行う。`acceptMetadata` は `collect.ts` の `whitespaceOnly` を import して使い、値を補正・切り詰めしない。`VideoMetadata` は `prepare.ts` の `ArticleMetadata` の別名にする。
    -   **対象:** `extension/src/core/metadata.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 1-4**: `test/metadata.test.ts` を作る（`parsePageMetadata`・`acceptMetadata`・`readFailureResult`・`emptyMetadataResult`）。設計書 3.2・3.9 のとおり、次を含める。不正な JSON、最上位が配列・`null`・数値、未知のメンバ、`descriptionCollapsed` の欠落、重複したメンバ、値の型の違い（`null`・数値・真偽値・オブジェクト・配列）で、返る `check` が設計書 3.2 の値になること。欠落・空白のみ（全角空白 `\u3000` を含む）・対になっていないサロゲート（`"\uD800"`・`"a\uDC00b"`）の省略と理由、値（前後の空白・改行・全角空白・サロゲートペア）の保持、`showExpandPrompt`（折りたたみ時かつ概要欄を受け入れたときだけ真）、`readFailureResult`・`emptyMetadataResult` の 3 つの `read-failure` と値・促しの欠落、大きな値（I-01。省略しない）。
    -   **対象:** `extension/test/metadata.test.ts`。**完了:** `metadata.test.ts` が通る。
-   [x] **ステップ 1-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。`parsePageMetadata` の検査 1〜5 を 1 つずつ外す、重複の検出を外す、`descriptionCollapsed` の必須を外す、`acceptMetadata` の空白のみ・対になっていないサロゲートの判定を 1 つずつ外す、`showExpandPrompt` の条件（概要欄を受け入れたときだけ）を外す、`readFailureResult` の省略を 2 つにする（いずれも `metadata.test.ts`）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`metadata.test.ts` が通る。

### PR-1 作成ポイント: core video-metadata judgement

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `feat(0025): add core video-metadata judgement`

**レビュー観点**: `parsePageMetadata` が定めた受理の形に合うものだけを受け入れ、未知・重複のメンバと値の型の違いを 1 つの境界で拒否すること / `acceptMetadata` が欠落・空白のみ・対になっていないサロゲートを省略し、値を補正・切り詰めず、対になったサロゲートを保つこと / `showExpandPrompt` が折りたたみ時かつ概要欄を受け入れたときだけ真になること / `RejectionMessage` の別名化と `whitespaceOnly` の `export` が既存の振る舞いを変えないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-3 の受理の境界（メンバの閉じた集合・重複の検出・値の型・省略の判定）は、設計書 3.2 が順序と各検査を確定しているものの、実装の選択が誤りやすい。重複の検出を引用符の数ではなく文字列とエスケープを追う走査で行う点と、省略の判定を 0007 の規則と一致させる点で誤りやすいためである。設計書 3.2 が採らなかった方法（0020 の引用符の数の方法）との競合があるため、frontier-recommended とする。

-   [x] グリーンゲート（`make ext-check` → `make test` → `make lint`。§3.2）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: ページの読み取りの注入

**対象ファイル**
-   変更: `extension/src/browser/chromeDeps.ts`、`extension/test/chromeDeps.test.ts`、`extension/src/launch.ts`（`MetadataReader` の宣言のみ。振る舞いは変えない。ステップ 2-3）

**タスク**
-   [x] **ステップ 2-1**: `ScriptingApi`（`chromeDeps.ts:7-12`）を、`func` と `args` を総称型で受け取る形にする（設計書 3.3）。`createSelectionReader`（`:78-94`）の `executeScript` に `args: []` を加える。`chromeDeps.test.ts` の `Injection` 型（`:18`）と `recordingScripting`（`:26-39`）を合わせて改める。選択範囲の読み取りの振る舞いは変えない。
    -   **対象:** `extension/src/browser/chromeDeps.ts`・`extension/test/chromeDeps.test.ts`。**完了:** `make ext-typecheck` と既存の `selection reader` の行が通る。
-   [x] **ステップ 2-2**: `readPageMetadata(expectedUrl)` を加える（設計書 3.3）。動画ページのトップフレームで実行し、外の名前を参照しない。`location.href` が `expectedUrl` と一致しなければ `null` を返す。動画タイトル・チャンネル名・概要欄を設計書 3.1 のセレクタで読み、概要欄は `is-expanded` 属性だけで全文と冒頭を選び、折りたたみの状態を `descriptionCollapsed` として返す。値のないメンバは結果に含めず、`JSON.stringify` した文字列を返す。
    -   **対象:** `extension/src/browser/chromeDeps.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 2-3**: `createMetadataReader(scripting)` を加える（設計書 3.3）。`MetadataReader` は `launch.ts` が持つ型を import する（`chromeDeps.ts:3` が既に `launch.ts` の型を import しているので循環しない）。注入の結果が文字列でなければ拒否する（`null` を含む）。`MetadataReader` の宣言を `launch.ts` に加える（型のみ。フェーズ 3 の経路の変更は含めない）。
    -   **対象:** `extension/src/browser/chromeDeps.ts`・`extension/src/launch.ts`。**完了:** `make ext-typecheck` が通る。
-   [x] **ステップ 2-4**: `chromeDeps.test.ts` に `metadata reader` と `page metadata injection` の行を加える（設計書 3.9・7.1）。`createMetadataReader` について、fake の `executeScript` が文字列以外（`null` を含む）を返すか例外を投げると `read` が失敗すること。注入関数のソースを jsdom の `document` と `location` で実行し（`vm.runInNewContext`。0007 の `selection reader` と同じ方法）、設計書 3.1 のセレクタで 3 つの値と折りたたみの状態を読み、URL が一致しなければ `null` を返し、値のないメンバを含めないこと。概要欄は、`is-expanded` の有無だけで全文と冒頭を選ぶことを、`is-expanded` と `#expanded` の中身が食い違う入力（`is-expanded` があり `#expanded` が空の入力と、`is-expanded` がなく `#expanded` に値がある入力）で確かめる（設計書 3.3 の「Declare, don't infer」。`#expanded` の中身では選ばない）。あわせて、注入関数がページを操作しないことを、実行の前後で jsdom の DOM が変わらないこと（`dom.serialize()` の比較。`outerHTML` は ESLint の `no-restricted-syntax` が許さないため、jsdom の直列化を使う）と、注入関数のソース（`executeScript` に渡した `func.toString()`）がページを操作する呼び出し（`.click(`・`dispatchEvent`・`.focus(`・`.blur(`・`.submit(`・`requestSubmit(`・`scrollIntoView`・`.value =`）を含まないことの両方で確かめる（AC-19）。注入関数はページで実行される唯一のコードなので、この 2 つで「ページを操作するコードを持たない」を確かめる。
    -   **対象:** `extension/test/chromeDeps.test.ts`。**完了:** `chromeDeps.test.ts` が通る。
-   [x] **ステップ 2-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。`createMetadataReader` の文字列以外の拒否を外す、注入関数の URL の確認を外す、`is-expanded` の判定を `#expanded` の中身の有無に替える、折りたたみの状態を常に `false` にする、値のないメンバを結果に含める（いずれも `chromeDeps.test.ts`）。注入関数にページを操作する 1 行（例: 概要欄の要素の `.click()`）を加える（`chromeDeps.test.ts` のページを操作しないことの行。`dom.serialize()` は変わらないので、ソースの走査の行が落ちる）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`chromeDeps.test.ts` が通る。

### PR-2 作成ポイント: page metadata injection

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0025): read video metadata from the page`

**レビュー観点**: `ScriptingApi` の総称化が選択範囲の読み取りの振る舞いを変えないこと / 注入関数が外の名前を参照せず、`location.href` の確認で別の動画の値を混ぜないこと / 概要欄の全文と冒頭の選択と折りたたみの状態が同じ `is-expanded` 属性だけで決まること / `createMetadataReader` が文字列以外を拒否すること / 注入が DOM を変えないこと

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（§1.3 に競合する実装の案がない）。注入関数は設計書 3.1・3.3 がセレクタ・URL の確認・全文と冒頭の選択を確定しており、直線的な読み取りで、回復の流れ・並行・状態機械に当たらない。

-   [x] グリーンゲート（`make ext-check` → `make test` → `make lint`。§3.2）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 表示

**対象ファイル**
-   新設: `extension/test/renderMetadata.test.ts`
-   変更: `extension/src/ui/render.ts`、`extension/src/core/summary.ts`（`truncate` の `export` のみ）、`extension/static/style.css`、`extension/test/render.test.ts`・`extension/test/helpers/dom.ts`（`valueOf` を共用するための整理。振る舞いは変えない）

**タスク**
-   [x] **ステップ 3-1**: `core/summary.ts` の `truncate`（`:26-35`）に `export` を付ける。振る舞いは変えない。
    -   **対象:** `extension/src/core/summary.ts`。**完了:** `make ext-typecheck` と既存の `summary.test.ts` が通る。
-   [x] **ステップ 3-2**: `ui/render.ts` の `messageNodes` の引数の型を `Message` にする（`RejectionMessage` の別名なので実質は変わらない）。`renderMetadata(container, result)` を加える（設計書 3.6）。`container` の子を置き換えずに追加する。値は既存の `field` で作り（ラベルは `"Video title"`・`"Channel"`・`"Description"`）、`values` にあるフィールドの行だけを作る。表示は `truncate` と `displayValue` でコードポイント単位に切り詰め（上限は実装で決める固定値。例: 200 コードポイント。境界をテストできるよう `metadataDisplayLimit` として export する）、`dirAuto: true` を渡す。値は `textContent` だけで設定する。`result.showExpandPrompt` が真のとき、`messageNodes` で `expandPromptMessage` を追加する。
    -   **対象:** `extension/src/ui/render.ts`。**完了:** `make ext-typecheck` と既存の `render.test.ts` が通る。
-   [x] **ステップ 3-3**: `static/style.css` の `unicode-bidi: plaintext` の規則（`:31-36`）に 3 つの値の新しいクラス名（`.metadata-title`・`.metadata-channel`・`.metadata-description`）を加える。概要欄の値のクラスは `white-space: pre-wrap` の規則（`:39-41`）にも加える（設計書 3.6）。
    -   **対象:** `extension/static/style.css`。**完了:** `make ext-build` が通る。
-   [x] **ステップ 3-4**: `test/renderMetadata.test.ts` を作る（`renderMetadata`。jsdom）。設計書 3.6・3.9 のとおり、次を含める。3 つの値がラベル付きで `textContent` に表示され、要素にならないこと、`<img src=x onerror=alert(1)>` を値に含めても `querySelector("img")` が `null` で `textContent` に元の文字列が含まれること、表示の切り詰め（上限ちょうど・1 超過・サロゲートペアを割らない）、`dir="auto"` が付くこと、既存の子を置き換えないこと、促しが `showExpandPrompt` が真のときだけ出て、`text` と `steps` が表示され、ページの文字列（概要欄など）を含まないこと。
    -   **対象:** `extension/test/renderMetadata.test.ts`。**完了:** `renderMetadata.test.ts` が通る。
-   [x] **ステップ 3-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。`renderMetadata` を `replaceChildren` に替える、`textContent` を HTML を解釈する書き方に替える、`truncate` を使わず切り詰めない、`dirAuto` を外す、促しを常に出す・`showExpandPrompt` を見ずに出す、促しの文言に概要欄の文字列を混ぜる（いずれも `renderMetadata.test.ts`）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`renderMetadata.test.ts` と既存の `render.test.ts` が通る。

### PR-3 作成ポイント: metadata display

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0025): render video metadata in the popup`

**レビュー観点**: `renderMetadata` が `renderSummary` の後に追加され、既存の表示を置き換えないこと / 値が `textContent` だけで表示され要素にならないこと / 表示だけを切り詰め、送る値を切り詰めないこと / `dir="auto"` と `unicode-bidi: plaintext` で bidi を分離し、概要欄の改行を保つこと / 促しが折りたたみ時だけ出て、ページの文字列を含まないこと

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（表示の組み立てとそのテストに限られ、§1.3 に競合する実装の案がない）。

-   [x] グリーンゲート（`make ext-check` → `make test` → `make lint`。§3.2）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 経路と配線

**対象ファイル**
-   変更: `extension/src/launch.ts`、`extension/src/popup.ts`、`extension/src/background.ts`、`extension/test/launch.test.ts`、`extension/test/prepare.test.ts`

**タスク**
-   [x] **ステップ 4-1**: `launch.ts` を設計書 3.5・3.7 のとおりに改める。`MetadataReader`（`read(tabId, expectedUrl): Promise<string>`）を宣言し、`MenuLaunchDeps`・`PopupLaunchDeps` に `metadataReader` を加える。`PopupLaunchResult`（`outcome` と `metadata`）を加える。内部の補助 `readMetadata(tabId, expectedUrl, reader, log)` を加え、読み取りの失敗・形の不一致・値の省略を `metadataLogLabel` でログに残す（値そのものは残さない。設計書 3.7）。`runMenuLaunch` は収集に成功したときだけ `readMetadata` を呼び、`prepareSend` の第 3 引数に `result.values` を渡す（現在の `{}` を置き換える。`:181`）。`runPopupLaunch` は収集に成功したときだけ `readMetadata` を呼び、`renderSummary` の後に `renderMetadata` を呼んで `PopupLaunchResult` を返す（収集が拒否なら `metadata` は `emptyMetadataResult()`）。`runPopupSend` は第 2 引数を `PopupLaunchResult` に変え、`result.outcome` と `result.metadata.values` を使う。促しのメッセージがあってもボタンの扱いを変えない。既存のログ（`menuLogLabel`・`popupLogLabel`）を変えない。
    -   **対象:** `extension/src/launch.ts`。**完了:** `tsc --noEmit -p tsconfig.build.json` が通る（テストの更新はステップ 4-3 の後）。
-   [x] **ステップ 4-2**: エントリポイントを改める。`popup.ts` は `runPopupLaunch` の結果（`PopupLaunchResult`）をそのまま `runPopupSend` へ渡し、依存に `createMetadataReader(chrome.scripting)` を加える。`background.ts` の `menuDeps` に `createMetadataReader(chrome.scripting)` を加える。
    -   **対象:** `extension/src/popup.ts`・`extension/src/background.ts`。**完了:** `tsc --noEmit -p tsconfig.build.json` が通る。
-   [x] **ステップ 4-3**: `launch.test.ts` を改める。`menuHarness`・`popupHarness` に `metadataReader` の fake を加える。`runPopupLaunch` の戻り値（`.outcome`・`.metadata`）と `runPopupSend` の第 2 引数に合わせて既存の行を改める。`video metadata log` の行を加え、設計書 3.9・7.2 のとおり次を確かめる。読み取りの失敗（注入の失敗・URL の不一致・形の不一致）でも起動が成功し、3 つの `read-failure` の省略が `error` に記録されること、フィールドの名前と省略の理由（`absent`・`blank`・`unpaired-surrogate`）が記録されること、ログに読み取った値が含まれないこと（識別できる文字列を値に使い、ログの文字列に現れないことを確かめる）、メタデータの取得の失敗が既存の拒否の理由（`not-watch-page`・`collection-failed`・`empty-selection`・`empty-title`）にならないこと。`popup send` に、促しがあってもボタンが押せて冒頭のみで送ることを確かめる行を加える。大きなメタデータでも起動が成功し、本文が上限を超える場合に `body-too-large` で止まることを確かめる行を加える（I-01）。タブのタイトルと動画タイトルに別の識別できる文字列を使い、両経路で本文の `title` が動画タイトルで、タブのタイトルが本文のどこにも現れないことを確かめる行を加える（AC-05・AC-09。`buildArticleBody` は `input.title` を読まないので、この結線の誤りはこの行でしか検出できない）。既存の収集の結果・拒否の理由・順序が変わらないこと（AC-05）を確かめる。収集が拒否されたときに `metadataReader` が呼ばれないこと、URL の不一致で 3 つの `read-failure` になること（AC-04a）、表示の切り詰めの長さを超える概要欄でも本文には切り詰める前の値が入ること（AC-11）を確かめる行を加える。
    -   **対象:** `extension/test/launch.test.ts`。**完了:** `launch.test.ts` が通る。
-   [x] **ステップ 4-4**: `prepare.test.ts` に、メタデータを含めた本文の行を加える（設計書 3.9、AC-08・AC-21、I-01）。既存の本文の行（`:73-105`）は維持する。大きなメタデータを含めた本文の UTF-8 のバイト数が `maxBodyBytes` を超えると `body-too-large` になること（メタデータを含めない場合と比べて、メタデータが上限に算入されること）、`<img src=x onerror=alert(1)>` を値に含めると本文にはその文字列が JSON の文字列として入ること。
    -   **対象:** `extension/test/prepare.test.ts`。**完了:** `prepare.test.ts` が通る。
-   [x] **ステップ 4-5**: 壊して失敗することを確かめ、コミットメッセージに記録する。`runMenuLaunch` の収集の拒否でも `readMetadata` を呼ぶ、`prepareSend` に `result.values` を渡さない、`runPopupLaunch` が収集の拒否でもページを読む、`runPopupSend` が `result.outcome` でなく別の値を使う、`readMetadata` のログに読み取った値（識別できる文字列）を渡す、読み取りの失敗を `RejectionReason` にする、促しがあるとボタンを出さない（いずれも `launch.test.ts`）。`prepareSend` でメタデータを本文に含めない（`prepare.test.ts`）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`launch.test.ts`・`prepare.test.ts` が通る。

### PR-4 作成ポイント: launch paths and wiring

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0025): wire video metadata into the launch paths`

**レビュー観点**: ページの読み取りが収集に成功したときだけ 1 回行われること / 読み取りの失敗と値の省略が起動を止めず、既存の拒否の理由を増やさないこと / ログに省略のフィールドと理由だけが残り、読み取った値が残らないこと / ポップアップとメニューの経路が同じ `prepareSend` に `result.values` を渡すこと / 促しがあってもポップアップのボタンの扱いが変わらないこと / メタデータの大きさが本文の上限の 1 か所に委ねられていること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（§1.3 に競合する実装の案がない）。`readMetadata` の配線は設計書 3.5 が順序と分岐を確定しており、状態機械・回復の流れ・並行を含まない。

-   [x] グリーンゲート（`make ext-check` → `make test` → `make lint`。§3.2）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 手動の確認と文書

**対象ファイル**
-   新設: `extension/test/metadataDocs.test.ts`
-   変更: `README.md`、`docs/dev/security.md`、`docs/translation_glossary.md`、本計画（§5.1）

**タスク**
-   [x] **ステップ 5-1**: 設計書 7.2 の手動の確認を、macOS の Chrome と Brave の作業時点の安定版で行い、§5.1 に記録する。確認の内容は §5.1 の表のとおりとする。
    -   **対象:** 本計画（§5.1）。**完了:** §5.1 の冒頭に定める完了の規則を満たす。
-   [x] **ステップ 5-2**: README の「Browser extension」の節を設計書 3.11 の `README.md` の行のとおりに改める。拡張が動画タイトル・チャンネル名・概要欄を収集して送ることを記し、動画ページで起動して送る手順に、概要欄が折りたたまれているときの促しのメッセージ（「…もっと見る」を開いてからポップアップを開き直す）を加え、取得できない値があることを記す。促しの手順は `expandPromptMessage`（`extension/src/core/messages.ts`）の文言をそのまま写す。「...more」が YouTube の英語の表示と一致するかはステップ 5-1 で確かめる。§1.3「文書」の Go のガードが読む語句を保つ。
    -   **対象:** `README.md`。**完了:** `make test`（`TestExtensionSendDocumentation`）が通る。
-   [x] **ステップ 5-3**: `docs/dev/security.md` §8.1 を設計書 3.11 の行のとおりに改める。拡張が動画ページの DOM から動画タイトル・チャンネル名・概要欄を読むこと、untrusted テキストとして扱い HTML として解釈せず本文へは JSON の文字列として入れること、新しい権限を加えず既存の `scripting` の注入で DOM を読むだけでページを操作しないこと、取得に失敗しても起動を失敗させないことを加える。見出しと「**要求する権限**」の一覧を変えず、新しい箇条として加える（§1.3「文書」）。
    -   **対象:** `docs/dev/security.md`。**完了:** `make test`（`TestExtensionDocumentation`・`TestExtensionSendDocumentation`）と `manifest.test.ts` が通る。
-   [x] **ステップ 5-4**: `docs/translation_glossary.md` に、動画タイトル・チャンネル名・概要欄・動画メタデータ・促しのメッセージの日本語と英語の訳語を加える（要件書 6.）。README と `security.md` の英語の表現と一致させる。
    -   **対象:** `docs/translation_glossary.md`。**完了:** §4.4 の横断検索で用語の一致を確かめる。
-   [x] **ステップ 5-5**: `test/metadataDocs.test.ts` を作る（AC-18 の静的な確認。§1.3）。`readRepoFile` で README と `docs/dev/security.md` を読み、設計書 3.11 と要件書 F-005 の内容（README: 動画タイトル・チャンネル名・概要欄を収集して送ること、促しのメッセージの手順、取得できない値があること。security.md §8.1: DOM から読むこと、untrusted テキストの扱い、新しい権限を加えないこと、ページを操作しないこと、取得に失敗しても起動を失敗させないこと）を含むことを確かめる。照合する語句は実装で決め、文書の書きぶりに合わせる。
    -   **対象:** `extension/test/metadataDocs.test.ts`。**完了:** `metadataDocs.test.ts` が通る。
-   [x] **ステップ 5-6**: 壊して失敗することを確かめ、コミットメッセージに記録する。README・`security.md` からステップ 5-5 の確かめる語句を含む文を 1 つずつ消す、§5.1 の 1 つの欄を空にする（`metadataDocs.test.ts` と、§5.1 はレビューで確認）。README に「never sends」を戻す（`TestExtensionSendDocumentation`）。続けて `make ext-check` → `make test` → `make lint` を通す。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確認し、コミットメッセージに記録する。

**完了条件:** `make ext-check` → `make test` → `make lint` が通り、`metadataDocs.test.ts` がそろい、§5.1 が冒頭の完了の規則を満たす。

### PR-5 作成ポイント: manual verification and documentation

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6

**推奨タイトル**: `docs(0025): record manual verification and update metadata documentation`

**レビュー観点**: §5.1 のすべての欄が実際の確認の結果で埋まり、冒頭の完了の規則を満たすこと / `metadataDocs.test.ts` が README・`security.md` の新しい記載を実行される検査で固定すること / README の促しの手順が `expandPromptMessage` の文言と一致すること / `security.md` §8.1 の見出しと「**要求する権限**」の一覧を変えず、新しい権限を加えていないこと / 用語集の訳語が README・`security.md` の表現と一致すること

**実装モデル要件**: standard

**判定理由**: どのトリガーにも当たらない（手動の確認の記録、文書の更新、および文書を読む拡張のテストに限られ、§1.3 に競合する実装の案がない）。手動の確認は人間の操作による受け入れの確認であり、自動テストや CI のように外部資源を要するものではない。

-   [x] グリーンゲート（`make ext-check` → `make test` → `make lint`。§3.2）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の条件 |
|---|---|---|
| M1: core の判定層 | フェーズ 1 | `metadata.test.ts` が通り、`messages.test.ts`・`collect.test.ts` が変わらず通る |
| M2: ページの読み取りの注入 | フェーズ 2 | `chromeDeps.test.ts` が通り、注入が DOM を変えない |
| M3: 表示 | フェーズ 3 | `renderMetadata.test.ts` と既存の `render.test.ts` が通る |
| M4: 経路と配線 | フェーズ 4 | `launch.test.ts`・`prepare.test.ts` が通る |
| M5: 確認と文書 | フェーズ 5 | §5.1 が冒頭の完了の規則を満たし、`metadataDocs.test.ts` と `make test` が通る |

### 3.2. PR 構成

PR は、設計書 8 章のフェーズ 3（経路と表示）を表示と経路の 2 つに分け、そのほかはフェーズと 1 対 1 に対応させる（PR-1〜PR-5）。各 PR は主たる関心事を持ち、単独でグリーンゲートを通せる単位とする。PR の境界と実装モデル要件の根拠は、各フェーズの末尾の「PR-N 作成ポイント」に記す。

本タスクの各 PR のグリーンゲートは、`_context.md` の "Green gate"（`make test && make lint`）に `make ext-check` を加えた `make ext-check` → `make test` → `make lint`（各フェーズの完了条件と同じ）とする。`make test`・`make lint` は Go のソースだけを対象にし、拡張の TypeScript の型検査・lint・テストを確かめないためである。`make ext-check` の前に `make ext-install` が要る（`extension/node_modules` が無い場合）。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | `core/metadata.ts` の受理と省略の判定、`core/messages.ts` の `Message` の一般化と促しの文言、`whitespaceOnly` の再利用、そのテスト | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | `ScriptingApi` の総称化、`readPageMetadata`、`createMetadataReader`、そのテスト | standard |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | `renderMetadata`、`truncate` の再利用、`style.css` の bidi と改行、そのテスト | standard |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | `launch.ts` の `readMetadata` と経路の変更、エントリポイントの配線、そのテスト | standard |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 | 手動の確認の記録、README・`security.md`・用語集の更新、`metadataDocs.test.ts` | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。フェーズ 1 の `core/` は、ほかのすべてのフェーズが使う（`MetadataResult` の型、`Message`、促しの文言）。フェーズ 2 の注入はフェーズ 4 の経路が使い、フェーズ 3 の表示はフェーズ 4 の `runPopupLaunch` が使う。したがって、表示（PR-3）を経路（PR-4）より先に置く。フェーズ 5 の手動の確認は、すべての経路がそろってから行う。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   設計書 3.9・7.1 のとおり、`node --test` で、fake の依存と jsdom を使う。実際のブラウザも実際のサーバも起動しない。テストは合成した値だけを使い、実際の YouTube のページの実データも、ページの HTML のスナップショットも加えない（要件書 5.）。
-   受理の形と省略の網羅は `metadata.test.ts` で行う。経路の配線は `launch.test.ts` が確かめ、`metadata.test.ts` の入力の表を繰り返さない。表示の手段は `renderMetadata.test.ts` が確かめ、文言の区別は `messages.test.ts` に任せる。
-   注入関数のテストは、関数のソースを `vm.runInNewContext` で実行する（0007 の `chromeDeps.test.ts` と同じ方法）。本タスクはセレクタを確かめるため、jsdom で組み立てた最小の DOM の `document` と `location` を文脈に渡す。
-   AC-16a は、`launch.test.ts` の `video metadata log` で、識別できる文字列を値に使い、fake のロガーに書かれた値を文字列にしてから、その値を含まないことを確かめる（設計書 3.9）。
-   `chromeDeps.ts` のうち `chrome` の API をそのまま呼ぶ部分とエントリポイント（`popup.ts`・`background.ts`）は、0007 と同じくユニットテストせず、手動の確認で確かめる。

### 4.2. 統合テスト（手動）

ブラウザを起動する自動テストは設けない（要件書 5.）。設計書 7.2 の手動の確認をステップ 5-1 で行い、§5.1 に記録する。

### 4.3. 後方互換性

-   収集の部分（`collect`・`SelectionReader`・`CollectedInput`・`renderSummary`）の振る舞いを変えない。既存の `collect.test.ts`・`summary.test.ts`・`render.test.ts`・`messages.test.ts` を変更せずに通すことで確かめる。
-   送信のクライアント（`buildArticleBody`・`prepareSend`・`sendArticle`・`classifyResponse`・`renderSend.ts`）の振る舞いを変えない。既存の `prepare.test.ts`（本文の行）・`send.test.ts`・`response.test.ts`・`renderSend.test.ts` を変更せずに通すことで確かめる。
-   Go のソースを変更しない。README・`security.md` を読む既存の Go のテストは、各フェーズの `make test` で通ることを確かめる。

### 4.4. 網羅率と横断検索

-   ステップ 4-5 で、`node --test --experimental-test-coverage` の出力を見て、`core/metadata.ts`・`ui/render.ts`・`launch.ts`・`browser/chromeDeps.ts` に実行されない分岐が残っていないかを確かめる（0007・0020 と同じ）。
-   `make lint`・`make test` で検出できない残存参照として、次を手で確かめる。
    -   `docs/translation_glossary.md` の新しい用語（動画タイトル・チャンネル名・概要欄・動画メタデータ・促しのメッセージ）の日本語と英語が、README と `docs/dev/security.md` の表現と一致すること（ステップ 5-4 の完了時）。
-   それ以外の項目（`ScriptingApi`・`runPopupLaunch`・`runPopupSend`・`messageNodes` の型の変更に伴う残存参照）は、`make ext-typecheck` が検出するので、横断検索の項目にしない。既存の名前を削除・改名するものはないので、残存参照の横断検索は要らない。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

ラベル: `test` は実際に実行され、失敗しうるテスト、`static` は文書・設定を確かめる、実行される検査、`manual` は手動の確認である。拡張のテストの場所は `extension/test/<file>::<describe の名前>`、Go のテストは `<path>::<関数名>` で示す。名前は計画上の名前である（§1.2）。「実装」の列は、実装のステップとテストを作るステップを挙げる。`manual` は `test`・`static` を置き換えず、補う。

| AC | ラベル | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test・manual | `extension/test/metadata.test.ts::acceptMetadata`（値の保持）、`extension/test/chromeDeps.test.ts::page metadata injection`（DOM の値をそのまま読む）、§5.1 | 1-3・1-4・2-2・2-4 |
| AC-02 | test | `metadata.test.ts::acceptMetadata`（欠落・空白のみを省略）、`launch.test.ts::launch paths`（省略があっても起動が成功する） | 1-3・1-4・4-1・4-3 |
| AC-03 | test・manual | `metadata.test.ts::acceptMetadata`（`showExpandPrompt`）、`chromeDeps.test.ts::page metadata injection`（`is-expanded` で全文と冒頭）、§5.1 | 1-3・1-4・2-2・2-4 |
| AC-04 | test | `chromeDeps.test.ts::metadata reader`（文字列以外・例外を拒否）、`launch.test.ts::launch paths`（読み取りの失敗でも収集は成功） | 2-3・2-4・4-1・4-3 |
| AC-04a | test | `chromeDeps.test.ts::page metadata injection`（`location.href` が一致しなければ `null`）、`chromeDeps.test.ts::metadata reader`（`null` を拒否）、`launch.test.ts::launch paths`（URL の不一致で 3 つの `read-failure`、収集の拒否ではメタデータを読まない） | 2-2・2-3・2-4・4-1・4-3 |
| AC-05 | test | `launch.test.ts::launch paths`（既存の収集の結果・拒否の理由・順序が不変。タブのタイトルと動画タイトルの識別できる文字列を使い、本文の `title` が動画タイトルであること）、`prepare.test.ts::buildArticleBody`（`input.title` を本文に入れない）、既存の `collect.test.ts` | 4-1・4-3・4-4 |
| AC-06 | test | `prepare.test.ts::buildArticleBody`（宣言したメンバだけ）、`launch.test.ts::launch paths`（メタデータを渡した本文） | 4-1・4-3・4-4 |
| AC-07 | test | `metadata.test.ts::acceptMetadata`（欠落・空白のみ・空文字列の省略）、`prepare.test.ts::buildArticleBody`（空・欠落のメンバを入れない） | 1-3・1-4・4-4 |
| AC-08 | test | `prepare.test.ts::prepareSend`（メタデータを含めた本文の上限。I-01）、`launch.test.ts::menu send`（`body-too-large`） | 4-1・4-3・4-4 |
| AC-09 | test | `launch.test.ts::launch paths`（タブのタイトルの識別できる文字列が本文に現れない）、`prepare.test.ts::buildArticleBody`（`input.title` が本文に現れない） | 4-3・4-4 |
| AC-10 | test | `metadata.test.ts::acceptMetadata`（対になっていないサロゲートの省略、対になったサロゲートの保持）、`launch.test.ts::video metadata log`（省略の理由） | 1-3・1-4・4-1・4-3 |
| AC-11 | test・manual | `renderMetadata.test.ts::renderMetadata`（jsdom で 3 つの値を表示、省略した値の行を作らない、表示だけをコードポイントで切り詰めサロゲートペアを割らない）、`launch.test.ts::popup send`（本文には切り詰める前の値が入る）、§5.1 | 3-2・3-4・4-1・4-3 |
| AC-12 | test・manual | `metadata.test.ts::acceptMetadata`（`showExpandPrompt`）、`renderMetadata.test.ts::renderMetadata`（促しが折りたたみ時だけ）、§5.1 | 1-3・1-4・3-2・3-4 |
| AC-13 | test | `launch.test.ts::popup send`（促しがあってもボタンが押せ、冒頭のみで送る） | 4-1・4-3 |
| AC-14 | test・manual | `renderMetadata.test.ts::renderMetadata`（要素にならず `textContent` に現れる、促しがページの文字列を含まない）、§5.1（bidi の分離） | 3-2・3-4 |
| AC-15 | test・manual | `launch.test.ts::menu send`（促しを出さず、確認を挟まずに送る）、§5.1 | 4-1・4-3 |
| AC-16 | test | `launch.test.ts::video metadata log`（読み取りの失敗で起動が成功、フィールド名と理由、拒否の理由が増えない） | 4-1・4-3 |
| AC-16a | test・manual | `launch.test.ts::video metadata log`（省略の理由、値がログに現れない）、`metadata.test.ts::acceptMetadata`（`omissions`）、§5.1 | 1-3・1-4・4-1・4-3 |
| AC-17 | test | `launch.test.ts::launch paths`・`::menu send`（読み取りの失敗でも送信に進み、拒否の理由にならない） | 4-1・4-3 |
| AC-18 | static・manual | `extension/test/metadataDocs.test.ts::video metadata documentation`（README・`docs/dev/security.md` の記載）、§5.1 | 5-2・5-3・5-5 |
| AC-19 | test・static・manual | `extension/test/manifest.test.ts::permissions`・`::undeclared keys`・`::host permissions`（変更しない）、`chromeDeps.test.ts::page metadata injection`（注入が DOM を変えず、注入関数のソースがページを操作する呼び出しを含まない）、§5.1 | 2-4・5-1 |
| AC-20 | test | `metadata.test.ts::parsePageMetadata`（不正な形・未知・重複を拒否）、`launch.test.ts::launch paths`（拒否でも起動が成功） | 1-3・1-4・2-3・4-1・4-3 |
| AC-21 | test | `prepare.test.ts::buildArticleBody`（文字列として入る）、`renderMetadata.test.ts::renderMetadata`（要素にならない） | 3-2・3-4・4-4 |

### 5.1. 手動の確認と記録

実装のときに埋める。確認の内容は設計書 7.2 の表のとおりである。ステップ 5-1 で行い、ステップ 5-6 で 1 つの欄を空にして失敗することを確かめる（§5.1 の記録はレビューでも確認する）。

**完了の規則:** すべての行の欄が埋まり、すべての行の結果が成功であるときだけ、§5.1 を完了とする。AC 番号のない行も同じである。結果が失敗の行、または未確認の行があれば完了としない。例外は「3.1 のセレクタ」の行のログイン状態の確認だけである。ログイン状態で確かめられない場合に限り、設計書 5.4 のとおり未確認として結果の欄に記録し、リスクとして残してよい（§6.1）。この場合も、ログインしていない状態での確認は成功していなければならない。

| AC・項目 | 確認の内容（設計書 7.2） | ブラウザ・環境 | 日付 | 結果 |
|---|---|---|---|---|
| AC-01・AC-03 | 概要欄が長く「…もっと見る」が出る動画で起動し、ポップアップに動画タイトル・チャンネル名・概要欄（冒頭）が表示される。概要欄を開いてから起動し直すと全文が表示される | macOS。Chrome 155.0.8059.40（公式ビルド、arm64）、Brave 1.97.59（公式ビルド、arm64。Chromium 155.0.8059.85）。いずれもシークレットモード | 2026-10-19 | Chrome: 成功。概要欄を開いた後の表示は、表示の上限（200 コードポイント）で切り詰めて「…」を付ける（設計書 3.6。本文には全文が入る）ため、「全文が表示される」は「折りたたみ時より先の内容が表示される」として確かめた。Brave: 成功 |
| AC-11・AC-12 | 概要欄が折りたたまれているとき促しのメッセージが出て、開いた後は出ない | macOS。Chrome 155.0.8059.40（公式ビルド、arm64）、Brave 1.97.59（公式ビルド、arm64。Chromium 155.0.8059.85）。いずれもシークレットモード | 2026-10-19 | Chrome: 成功。Brave: 成功 |
| AC-15 | メニューの経路では促しのメッセージを出さず、確認を挟まずに送る | macOS。Chrome 155.0.8059.40（公式ビルド、arm64）、Brave 1.97.59（公式ビルド、arm64。Chromium 155.0.8059.85）。いずれもシークレットモード | 2026-10-19 | Chrome: 成功。Brave: 成功 |
| AC-16a | 開発者向けのログ（ブラウザのコンソール）に、省略したフィールドの名前と理由が残り、値そのものは残らない | macOS。Chrome 155.0.8059.40（公式ビルド、arm64）、Brave 1.97.59（公式ビルド、arm64。Chromium 155.0.8059.85）。いずれもシークレットモード | 2026-10-19 | Chrome: 成功。Brave: 成功 |
| 3.1 のセレクタ | 実際の拡張から、動画タイトル・チャンネル名・概要欄と折りたたみの状態が読める（Chrome と Brave。ログイン状態を含む）。取得できない値を含むページでも起動が成功する（設計書 5.4） | macOS。Chrome 155.0.8059.40（公式ビルド、arm64）、Brave 1.97.59（公式ビルド、arm64。Chromium 155.0.8059.85）。いずれもシークレットモード | 2026-10-19 | Chrome・Brave（ログインしていない状態。シークレットモード）: 成功。Brave（ログインした状態。通常のウィンドウ）: 成功 |
| AC-19 | メニューとポップアップの両方の経路で起動し、拡張が起動したタブのページを操作しない（概要欄の「…もっと見る」を開かない）。ページを操作する追加の権限の確認を求められない | macOS。Chrome 155.0.8059.40（公式ビルド、arm64）、Brave 1.97.59（公式ビルド、arm64。Chromium 155.0.8059.85）。いずれもシークレットモード | 2026-10-19 | Chrome: 成功。Brave: 成功 |
| AC-14: 双方向テキスト（bidi）の分離（設計書 3.6・5.5） | 動画タイトル・概要欄に双方向の制御文字（U+202E など）を含む動画で、ポップアップのラベルとほかの値の表示順が変わらない | macOS。Chrome 155.0.8059.40（公式ビルド、arm64）、Brave 1.97.59（公式ビルド、arm64。Chromium 155.0.8059.85）。いずれもシークレットモード | 2026-10-19 | Chrome: 成功。該当する動画が見つからなかったため、DevTools のコンソールで動画タイトル（`ytd-watch-metadata #title h1 yt-formatted-string`）と概要欄（`#expanded yt-attributed-string`）の `textContent` に U+202E を含む文字列を書き込んでから起動して確かめた（拡張は DOM の値を読むので、同じ入力になる。確かめる内容は変えていない）。Brave: 成功 |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| YouTube の DOM の構造が変わり、設計書 3.1 のセレクタが値を取れない、または別の文字列を取る | 値の取得に失敗するか、本文に余分な値が入る | 取得の失敗を拒否の理由にしないので、記事は選択範囲だけで生成され続ける（設計書 5.4）。セレクタはステップ 5-1 の手動の確認で実際のページに当てて確かめる |
| 設計書 3.1 の観測はログインしていないページで行った | ログインしたページで `#owner` の周りが変わると、チャンネル名のセレクタが当たらない | ステップ 5-1 の手動の確認でログイン状態も確かめる。確かめられない場合は、設計書 5.4 の未確認として §5.1 に記録する |
| `parsePageMetadata` の重複の検出を、0020 の引用符の数の方法で実装してしまう | 値のテキストに `"` が含まれると、正しい結果を拒否する | 設計書 3.2 のとおり、文字列とエスケープを追う走査で行う。ステップ 1-5 で、重複の検出を外すと `metadata.test.ts` が落ちることを確かめる |
| `acceptMetadata` の空白のみの判定が `collect` の規則と食い違う | 全角空白や U+0085 の値の扱いが収集と異なる | `collect.ts` の `whitespaceOnly` を `export` して再利用し、同じ正規表現を書き直さない（ステップ 1-2・1-3） |
| jsdom が `unicode-bidi`・`white-space` の実際の描画を再現しない | 表示のテストで確かめられない項目がある | 要素のクラス・`dir` の属性だけをユニットテストで確かめ、実際の表示順はステップ 5-1 の手動の確認で確かめる |
| 計画だけの変更では拡張のジョブが起動しない | AC-18 の対象（README・`security.md`）を変えない変更では `metadataDocs.test.ts` が走らない | AC-18 の対象を変えるときは `has-extension-changes.sh` の条件（`README.md`・`docs/dev/security.md`）で拡張のジョブが起動する（§1.3「文書」） |

### 6.2. スケジュールのリスク

| リスク | 対策 |
|---|---|
| ステップ 5-1 の手動の確認は、利用者のブラウザでの操作を必要とする | 文書の作業（ステップ 5-2〜5-5）を先に進め、確認の結果を待つ間に止めない |
| `docs/translation_glossary.md` の更新は要件書 F-005 の文書の列挙にない | 要件書 6. の「用語は translation_glossary.md と統一する」に基づく小さな追加とする。用語の一致は §4.4 の横断検索で確かめる |

## 7. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
-   [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）
-   [x] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
-   [x] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5）
-   [x] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6）
-   [x] §5 のすべての AC の検証が通り、§5.1 が冒頭の完了の規則を満たす
-   [x] §4.4 の横断検索（用語集の用語の一致）を確かめた

## 8. 成功基準 (Success Criteria)

-   **機能:** §5 の `test`・`static` の検証がすべて通る。§5.1 が冒頭の完了の規則を満たす。
-   **品質:** `make ext-check`・`make test`・`make lint` が通る。各テストを壊して失敗することを確かめ、コミットメッセージに記録している。§4.4 の網羅率を確かめている。
-   **セキュリティ:** AC-14・AC-19・AC-21 の検証が通る。値が `textContent` だけで表示され、本文へは JSON の文字列として入り、新しい権限とページを操作するコードがない。
-   **文書:** README・`docs/dev/security.md`・`docs/translation_glossary.md` を更新し、`metadataDocs.test.ts` と `make test` が通る。

## 9. 次のステップ (Next Steps)

-   本計画をレビューし、`approved` にする。
-   `/runplan 0025` で PR-1 から順に実装する。
