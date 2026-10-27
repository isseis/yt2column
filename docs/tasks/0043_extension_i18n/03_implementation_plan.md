# 実装計画書：ブラウザ拡張の UI の多言語化（英語と日本語）

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-27 |
| Review date | 2026-10-27 |
| Reviewer | isseis |
| Comments | 2026-10-27: ステップ 3-6 の実機の確認は Chrome だけで行う。Chrome と Brave に固有のコードがないためで、レビュアーが本セッションで承認した（Brave は同じ結果と見込む）。 |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、「設計書 3.2」などは設計書の節を指す。既存のコードと文書についての記述は、コミット `4efdddf` で確かめた。`file:line` はこのコミットの行番号を指す。拡張のファイルのパスは、設計書と同じく `extension/` を省いて書く。本書の「差分の確認」は、設計書の「`diff` の検証」と同じものを指す。

設計書が引く行番号はコミット `8a39cfc` のものである。拡張のディレクトリ（`extension/`）は 2 つのコミットで同じなので、拡張のファイルの行番号はそのまま使える。`CLAUDE.md` と `README.md` は、その後の変更で行がずれている。

| 設計書の参照（`8a39cfc`） | 本書の参照（`4efdddf`） |
|---|---|
| `CLAUDE.md:203`（「Declare, don't infer」） | `CLAUDE.md:234`（例外の記述は `:241-263`） |
| `CLAUDE.md:397-400`（拡張の言語の規則） | `CLAUDE.md:430-433` |
| `README.md:908-912`（拡張に設定がないことの段落） | `README.md:1002-1006` |
| `README.md:922-926`（促しのメッセージの引用） | `README.md:1016-1020` |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、次を行う。

-   言語の型・言語の判定・文言の集合（`src/core/i18n/`）を加え、ポップアップのすべての文言を英語と日本語の文言の集合に移す（F-001・F-002）。
-   振り分けの関数と表示の関数が言語を受け取る形にし、ポップアップの入口（`startPopup`）が表示言語から言語を決めて、ページの `lang` 属性・書体・行間を合わせる（F-001・F-003）。
-   manifest の名前と説明を `_locales` で多言語にする（F-004）。
-   `CLAUDE.md` の言語の規則と「Declare, don't infer」の例外、README の言語の決まり方の段落を書く（F-005）。

サーバ・ホスト・Go のソースは変えない（要件書 2.3）。

### 1.2. 実装の方針

-   **設計は設計書に従い、本書は繰り返さない。** 型と関数の形は設計書 3.1〜3.3・3.5、日本語の文言の字句は設計書 3.4、manifest と `_locales` の値は設計書 3.6、テストの構成は設計書 3.9 が定める。本書のステップは、どの節に従って何を変えるかと、どう確かめるかだけを示す。
-   **英語の文言は一字一句そのまま移す（AC-07）。** 今のソースの英語の文字列リテラルを、書き直さずに `en.ts` へ写す。変わっていないことは、差分の確認のスクリプト（ステップ 2-5）で確かめる。
-   **テストが失敗しうることを、壊して確かめる。** 新しいテストと、言語ごとに繰り返す形にしたテストのそれぞれについて、そのテストが通る経路を壊して失敗することを、実装のときに確かめる。各ステップに挙げた壊し方は最低限のもので、それ以外のテストにも 1 つずつ壊し方を当てる。結果はコミットのメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。
-   **各ステップの後で `make ext-check` が通る。** ステップごとにコミットし、どのコミットでも拡張の型の検査・lint・テスト・ビルドが通る状態を保つ。
-   **拡張のソースは英語で書く。** コメント・識別子・文字列リテラルは英語である。例外は設計書 3.7 の 3 つ（`ja.ts` の文言、日本語の文言の性質を確かめるテストの語の表、日本語の文書に一致させるガードのテストのリテラル）だけで、フェーズ 1 で `CLAUDE.md` に書く。
-   **新しいソースは秘密情報の語を含まない。** `test/secretsAbsent.test.ts:10-31` は `src/` と `static/` のすべてのファイルで、`token`・`webhook`・`Authorization` などの語を大文字・小文字を区別せずに禁じる。`src/core/i18n/` のファイル（コメントを含む）と `static/_locales/` のファイルにもこの検査が当たる。
-   **フェーズは設計書 8 章の順序に従う。** PR は 3 つに分ける（1.3、3.2）。

### 1.3. 計画の際に決めたこと（2026-10-27）

-   **PR は 3 つに分ける（レビュアーが選んだ）:** PR-1 はフェーズ 1・2（`CLAUDE.md` と多言語の土台）、PR-2 はフェーズ 3（入口・ページ・CSS）、PR-3 はフェーズ 4・5（manifest・`_locales`・README）とする。いちばん大きく、危険も大きいフェーズ 2 を、それだけでレビューできるようにするためである。PR-2 のマージから PR-3 のマージまでの間は、README がまだ言語の決まり方を説明していない。利用者は 1 人で、表示そのものは正しいので、この間を許す。2 つの PR にする案（1+2 / 3+4+5、1+2+3 / 4+5）は採らなかった。
-   **PR-1 の間は、`popup.ts` が言語に `"en"` を固定で渡す:** フェーズ 2 は `PopupLaunchDeps`・`PopupSendDeps` に `locale` を加える（設計書 3.3）。`startPopup` はフェーズ 3 で加えるので、PR-1 の `popup.ts` は依存に `locale: "en"` を書く。ステップ 3-3 がこれを `startPopup` に置き換える。PR-1 のマージの後も、ポップアップは英語で表示される。英語の表示の変化は、AC-13 の意図した変更（1000 以上の文字数と行数に桁区切りが入る）だけである。
-   **実機の確認は PR-2 と PR-3 に分ける:** ポップアップの日本語の表示・`lang` 属性・書体と行間（AC-01〜AC-03・AC-14〜AC-16）は、それを入れる PR-2 で確かめる（ステップ 3-6）。拡張の管理ページの名前・説明・拡張 ID（AC-19・AC-20）は、manifest を変える PR-3 で確かめる（ステップ 5-2）。`popup.ts` の組み立て（`readUiLanguage(chrome.i18n)` を渡すこと）は単体テストを持たないので、PR-2 の実機の確認がこれを確かめる。
-   **差分の確認のスクリプトは TypeScript で書き、Node.js で実行する:** 確かめる対象は拡張の TypeScript の表示の関数で、Go からは実行できない。Node.js は拡張の開発環境にあり（`extension/.node-version` の `24.21.0`）、新しい依存を加えない。スクリプトは `docs/tasks/0043_extension_i18n/_checks/english_text.ts` に置き、リポジトリのルートで `node docs/tasks/0043_extension_i18n/_checks/english_text.ts` と実行する。拡張の `tsc`・ESLint・Prettier の対象（`extension/` の下）の外なので、これらの検査は当たらない。使い捨てのスクリプトで、マージの後は保守しないので、これらの検査の対象外でよい（`requirements_process.md` §4）。
-   **差分の確認は、merge-base と head の両方を head の呼び出しの形で実行する:** head の形は、表示の関数に言語 `"en"` を渡し、`runPopupSend` の依存に `locale: "en"` を加える形である。PR-1 の merge-base の関数は言語を受け取らないが、JavaScript は余分な引数と余分なプロパティを無視するので、同じ呼び出しで今の振る舞いのまま動く。PR-2 の merge-base（PR-1 のマージの後）の関数は言語を受け取る。どちらの側も同じ呼び出しで済むので、呼び出しの形を選ぶ引数は設けない。
-   **差分の確認は PR-1 と PR-2 で実行し、PR-3 では実行しない:** 設計書 3.9 は、英語の文言や表示のコードを変える PR のそれぞれで実行すると定める。PR-1 は文言と表示の関数を、PR-2 は `launch.ts`（`runPopupSend` と同じファイル）を変える。PR-3 は manifest・`_locales`・README だけを変え、ポップアップの表示のコードに触れない。
-   **差分の確認は、桁区切りの入らない数で取り出す:** AC-13 は、英語の表示にも桁区切りを加える意図した変更である。スクリプトが描画する要約の文字数と行数は 1000 未満にし、この意図した変更を差分に含めない。桁区切りは、AC-13 のテスト（ステップ 2-1・2-4）で確かめる。
-   **型の検査で欠けを見つけるテストは、`runTsc` とは別の設定で `tsc` を動かす:** 今の `runTsc`（`test/typecheck.test.ts:56-86`）は、一時的なディレクトリを `rootDir` にする。一時的なファイルが拡張の `src/` を import すると、`rootDir` の外のファイルとして TS6059 で失敗する。コミット `4efdddf` で、次の設定を試した。一時的なディレクトリのファイルから `src/core/types.ts` を相対パスで `import type` し、`tsconfig.json` を `extends` して、`rootDir`・`outDir` を上書きしない。この設定で `tsc -p` を実行すると、`Record<RejectionReason, string>` の値から 1 つのキーを除いたときは TS2741（`Property '"empty-title"' is missing ...`）で終了コード 2 になり、すべてのキーを持つときは終了コード 0 になった（`rootDir` を上書きした設定では TS6059 も、絶対パスの import では TS2877 も出た）。テストはこの設定を使う。
-   **`expectMessage` の補助を `test/helpers/dom.ts` へ移す:** 同じ補助が `test/launch.test.ts:173-179` と `test/renderSend.test.ts:14-20` にある。両方の言語で表示を確かめるときに、もう一方の言語の文言を表示しないことも確かめるので、この補助を 1 つにまとめて使う（DRY）。
-   **テストのファイルの分け方:** 設計書 3.11 のとおりにする。ただし、言語ごとの語の表で確かめる性質（AC-09）は、設計書 3.9 のとおり `messages.test.ts`・`sendMessages.test.ts` を言語ごとに繰り返す形で確かめ、`catalog.test.ts` は文言の集合の全体をたどる検査（空でないこと、手順の規則）だけを持つ。既存の `messages.test.ts`・`sendMessages.test.ts` の「空でない」の検査は、同じ `it` の中で文言が互いに異なることも確かめているので残す。`catalog.test.ts` は、振り分けの関数を経ない項目（欄の名前・送信ボタン・送信中の案内・促しのメッセージ）も含めて全体をたどるので、重なりは理由ごとの文言の本文に限られる。`applyPageLocale` のテストも、設計書のとおり `test/locale.test.ts` に置く。
-   **AC-10・AC-11 は PR のレビューで照らす（`manual`）:** どちらも、日本語の文言の字句が用語集または設計書 3.4 の観測の記録と一致することを求める。比べる相手の用語集の表記は、表のセルの中の文の一部（「概要欄が折りたたまれているとき、ポップアップで「もっと見る」（"...more"）を開いて…」、`docs/translation_glossary.md:61`）で、語を取り出すには、文の中の語を照合することになる。設計書 3.4 の表はタスクの記録である。`CLAUDE.md`「Document tests check values, structure, and executable examples, never prose」は、文の意味を語で照らすことと、タスクの記録を確かめることを禁じるので、テストにしない。設計書 7.4 も、この 2 つを `manual` と定めている。

### 1.4. 既存コード調査結果

**文言を持つ場所と、その参照（フェーズ 2 で変える）**

| 記号 | 場所 | 参照するファイル |
|---|---|---|
| `Message`・`RejectionMessage` | `src/core/messages.ts:4-12` | `src/ui/render.ts:2`（`Message`）、`src/core/sendMessages.ts:8`・`src/ui/renderSend.ts:5`（`RejectionMessage`）、`test/launch.test.ts:11`・`test/renderSend.test.ts:4`・`test/sendMessages.test.ts:5` |
| `expandPromptMessage`（定数） | `src/core/messages.ts:18-24` | `src/ui/render.ts:1`・`:164`、`test/launch.test.ts:10`、`test/renderMetadata.test.ts:4`、`test/metadataDocs.test.ts:4`・`:32-33` |
| `rejectionMessage` | `src/core/messages.ts:31-63` | `src/core/sendMessages.ts:7`・`:24`、`src/ui/render.ts:94`、`test/messages.test.ts`・`test/sendMessages.test.ts`・`test/render.test.ts` |
| `blockMessage`・`serverRejectMessage`・`sendOutcomeMessage` | `src/core/sendMessages.ts:18`・`:38`・`:145` | `src/ui/renderSend.ts:8`・`:22`・`:47`、`test/sendMessages.test.ts`（関数を `.map` に直接渡す呼び出しが `:207-211` にある）・`test/renderSend.test.ts`・`test/launch.test.ts:18` |
| `notSentMessage`・`unexpectedMessage`（export しない） | `src/core/sendMessages.ts:92`・`:115` | `src/core/sendMessages.ts:155`・`:173` |
| `sendingMessage` | `src/ui/renderSend.ts:12-18` | `src/launch.ts:22`・`:218`、`test/launch.test.ts:20`・`:493` |
| 要約の欄の名前（`Title`・`URL`・`Characters`・`Lines`・`Preview`） | `src/ui/render.ts:48-66`（`collectedNodes`） | なし（export しない関数の中） |
| 動画メタデータの欄の名前 | `src/ui/render.ts:125-129`（`metadataRows`） | なし |
| 送信ボタンの名前 | `src/launch.ts:207` | なし |

`renderSummary`・`renderMetadata` を呼ぶのは `src/launch.ts:163`・`:173`、`renderSendOutcome`・`renderBlockReason` を呼ぶのは `src/launch.ts:198`・`:223` だけである。テストでは、`test/render.test.ts`・`test/renderMetadata.test.ts`・`test/renderSend.test.ts`・`test/launch.test.ts` が呼ぶ。`runPopupLaunch`・`runPopupSend` を呼ぶのは `src/popup.ts:18`・`:24` と `test/launch.test.ts` である。拡張の外（`README.md`・`CLAUDE.md`・`docs/dev/`・Go のソース）でこれらの記号を名前で引くのは、用語集の `expandPromptMessage`（`docs/translation_glossary.md:61`）だけである。関数になっても名前は変わらないので、用語集は変えない。

テストの値の検査は、英語の欄の名前で値を探す（`test/helpers/dom.ts` の `valueOf`・`valueElement`。例: `test/render.test.ts:27-31`・`:54-55`、`test/renderMetadata.test.ts:30-37`・`:44-46`・`:56-58`）。日本語で実行するときは、`catalogs[locale]` の欄の名前で探す必要がある。英語の名前で探すと、欄が見つからない場合の `undefined` の検査（`test/renderMetadata.test.ts:45-46`）が、何も確かめないまま通ってしまう。

文言の書き方を述べている古いコメントは次のとおりで、ステップ 2-3 で新しい形に合わせる: `src/core/messages.ts:14-17`（`expandPromptMessage` の定数の説明）・`:26-30`（「distinct English message」と `never` の網羅の説明）、`src/core/sendMessages.ts:1-5`（「English messages ...」と各 `switch` の説明）、`src/ui/render.ts:117`・`:123-124`（`MetadataRow` の `label` の説明）、`src/popup.ts:1-3`（ステップ 3-3 で直す）。

**移動・削除する記号の台帳**

| 記号 | 今 | 本タスクの後 |
|---|---|---|
| `Message` | `src/core/messages.ts` で定義 | `src/core/i18n/catalog.ts` で定義（形は変えない）。`messages.ts` は定義しない |
| `RejectionMessage` | `src/core/messages.ts` | 変えない（`Message` の別名のまま、`catalog.ts` の `Message` を指す） |
| `expandPromptMessage` | 定数 | 言語を受け取る関数（設計書 3.2） |
| `sendingMessage` | `src/ui/renderSend.ts` の定数 | 削除。文言は `Catalog` の `sending` |

本書のほかの節は、この表の記号の名前と場所に従う。

**再利用するもの**

-   `rejectionReasons`（`src/core/types.ts:13-18`）・`preflightReasons`（`src/core/prepare.ts`）・`serverRejectStatus`（`src/core/response.ts`）: テストが理由の一覧をたどるのに今も使っている（`test/sendMessages.test.ts:34-37`・`:71`）。言語ごとに繰り返すテストもこれを使う。`locales` も同じ形（`Record<Locale, true>`）にする（設計書 3.1）。
-   `test/helpers/dom.ts` の `container`・`query`・`valueOf`・`valueElement`、`test/helpers/logger.ts` の `recordingLogger`、`test/helpers/native.ts` の `testSendDeps`・`deferredNative`・`recordingNative`・`responseReply`。
-   `test/typecheck.test.ts` の一時的なディレクトリと `tsc` の実行の方式（1.3 の設定の違いを除く）と、エラーの番号をファイルの行に結びつけて確かめる形（`:103-106`）。
-   `test/manifest.test.ts` の `manifest` の読み込みと、拡張 ID の検査（`:63-85`、変えない）。
-   `test/metadataDocs.test.ts` の `flatSection`・`expectPhrases`（比べる相手だけを変える）。

**ビルドと検査（変えないもの）**

-   `scripts/copy-static.ts` は `static/` を再帰的に `dist/` へ写し、`scripts/check-dist.ts` は `static/` の下のファイルを再帰的に期待するファイルに加える（`scripts/check-dist.ts:22-61`）。`static/_locales/` は変更なしで写り、検査される（設計書 3.6）。
-   `tsconfig.core.json` の `include` は `src/core` で、その下のディレクトリも含むので、`src/core/i18n/` もブラウザの型なしで検査される。
-   `eslint.config.js` の `chrome.storage` の禁止（`:44-52`）と HTML を解釈する API の禁止（`:5-19`）は、そのまま当たる（AC-04・AC-24）。
-   Go のテストのうち `CLAUDE.md` を読むものは、`cmd/yt2column/extension_docs_test.go`（`ext-` のターゲットの一覧）、`cmd/yt2column/docs_test.go`（`## Quick Links`）、`cmd/yt2column/codex_review_docs_test.go:93` の `TestCodexMentionOnlyInRequestRow`（`CLAUDE.md` の全体で Codex への言及の形を確かめる）である。フェーズ 1 で書く文は、Codex に言及しない。

**新しく加えるもの**

-   `src/core/i18n/locale.ts`・`catalog.ts`・`en.ts`・`ja.ts`、`src/ui/pageLocale.ts`、`static/_locales/en/messages.json`・`static/_locales/ja/messages.json`（設計書 2.1）。
-   `test/locale.test.ts`・`test/catalog.test.ts`・`test/style.test.ts`（設計書 3.11）。
-   差分の確認のスクリプト `docs/tasks/0043_extension_i18n/_checks/english_text.ts`。同じ置き方の前例に `docs/tasks/0040_production_profile/_checks/main.go` がある（Go で書かれている。本タスクは 1.3 のとおり TypeScript で書く）。コミット `4efdddf` で、`git archive HEAD extension/src extension/test/helpers extension/package.json` を一時的なディレクトリに展開し、`extension/node_modules` を作業ツリーの `extension/node_modules` へのシンボリックリンクにした。そのディレクトリに置いた `.ts` のファイル（`test/helpers/dom.ts` の `container` と `src/ui/render.ts` の `renderSummary` を import して、描画した文字列を出す）を `node` で実行した。終了コード 0 で、`not-watch-page` の英語の文言と 3 つの手順を出した。

**型の構成の経路:** 本タスクは、具象の型で振る舞いを分ける設計を持たないので、該当しない。`Locale` は文字列のリテラルの合併型で、値は型の検査が `"en"`・`"ja"` のどちらかに限る。

**`implementation_handoff.md`:** ない。`design_handoff.md` の H-01〜H-10 は、すべて設計書 3.10 に記録されている。

## 2. 実装ステップ (Implementation Steps)

テストの場所は `path::describe の名前` の形で書く。`describe` の名前は本書で決めたもので、実装はこの名前を使う。既存の `describe` には今の名前を書く。

### フェーズ 1: CLAUDE.md の言語の規則と例外

#### ステップ 1-1: `CLAUDE.md` を改める

**変更するファイル:** `CLAUDE.md`

-   [x] 「Go Idioms」の拡張の言語の規則（`CLAUDE.md:430-433`）を、設計書 3.7 の 1 つめの項目のとおりに書き直す。例外は 3 つ（`src/core/i18n/` の英語でない言語の文言の集合のファイルの文言、英語でない言語の文言の性質を確かめるテストの文字列リテラル、日本語の文書の見出しや表のセルに一致させるガードのテストの文字列リテラル）とし、利用者に表示する文言は文言の集合に置いて英語と日本語の両方を必須とすること、開発者向けのログは英語であることを書く（AC-21）。
-   [x] 「Declare, don't infer」の `internal/transcript` の例外の記述（`CLAUDE.md:241-263`）の後に、言語の判定（`resolveLocale`）の例外を、同じ形（Scope・Reason・Boundary）で英語で書く（設計書 3.1・3.7）。

**完了の条件:** `make test`（`CLAUDE.md` を読む Go のガードを含む）と `make lint` が通る。コミットする。

### フェーズ 2: 多言語の土台

#### ステップ 2-1: 言語の型と言語の判定

**変更するファイル:** `src/core/i18n/locale.ts`（新規）、`test/locale.test.ts`（新規）

-   [x] 設計書 3.1 のとおり、`Locale`・`locales`・`resolveLocale`・`formatCount` を書く。`resolveLocale` は `locales` のキーと比べる（設計書 3.1 の規則）。
-   [x] `test/locale.test.ts::resolveLocale` に、要件書 AC-01・AC-02 の例の値（`ja`・`ja-JP`・`JA`・`en`・`en-US`・`fr`・`zh-CN`・`jam`・`jv`・`ja_JP`・空文字列・`undefined`）を渡して結果を確かめるテストを書く。
-   [x] `test/locale.test.ts::formatCount` に、`Intl.NumberFormat.supportedLocalesOf` が `en` と `ja` の両方を返すことを前提として確かめてから、`formatCount(12345, "en")`・`formatCount(12345, "ja")` が `12,345` になることを確かめるテストを書く（設計書 3.3、H-10）。
-   [x] 壊して確かめる: 判定を前方一致（`startsWith("ja")`）にする、小文字にしない、区切りに `_` も使う、`formatCount` を `String(value)` にする、の 4 つで、それぞれテストが失敗することを確かめる。

**完了の条件:** `make ext-check` が通る。コミットする。

#### ステップ 2-2: 文言の集合

**変更するファイル:** `src/core/types.ts`、`src/core/i18n/catalog.ts`・`en.ts`・`ja.ts`（新規）、`src/core/messages.ts`、`src/ui/render.ts`、`test/catalog.test.ts`（新規）、`test/typecheck.test.ts`

-   [x] `src/core/types.ts` に `SummaryField` を加える（設計書 3.2）。
-   [x] `catalog.ts` に `Message`・`Catalog`・`catalogs` を書く（設計書 3.2）。`Message` の定義を `messages.ts` から移し、`messages.ts` の `RejectionMessage` は `catalog.ts` の `Message` の別名にする（1.4 の台帳）。`src/ui/render.ts:2` の `Message` の import を `catalog.ts` からの import に変える。
-   [x] `en.ts` に、1.4 の表の場所にある英語の文字列リテラルを、そのまま写す。欄の名前と送信ボタンの名前は設計書 3.4 の「英語（今の字句）」の列と照らす。この時点では、振り分けと表示の関数はまだ今の文字列リテラルを使う（ステップ 2-3 で `catalogs` に切り替える）。
-   [x] `ja.ts` に、設計書 3.4 の表の日本語の文言をそのまま写す（AC-10・AC-11 の字句は設計書 3.4 が決めている）。
-   [x] `test/catalog.test.ts::catalogs` に、`locales` の各言語について、すべての `Message` の本文と手順、すべての欄の名前と送信ボタンの名前が空でないことを確かめるテストを書く。あわせて、`en` と `ja` の同じ位置の `Message` を組にして、日本語の文言が手順を持つのは英語の文言が手順を持つときだけであることを確かめるテストを書く（設計書 3.9、AC-06）。
-   [x] `test/typecheck.test.ts` に、既存の `describe` とは別の `describe`（`a Catalog missing an item fails typecheck`、自身の一時的なディレクトリを持つ）を加え、`Catalog` の値から項目を 1 つ除いた一時的なモジュールの型の検査が、そのモジュールの行の TS2741 で失敗し、すべての項目を持つ対照のモジュールの型の検査が通ることを確かめるテストを加える（1.3 の設定、エラーの確かめ方は `typecheck.test.ts:103-106` と同じ）。除く項目は、最上位の項目（例: `sendButton`）と、`Record` の節の 1 つのキー（例: `rejection` の 1 つの理由）の 2 つとする。
-   [x] 壊して確かめる: `ja.ts` の 1 つの本文を空にする、`ja.ts` の 1 つの手順を空にする、英語が手順を持たない文言に日本語の手順を 1 つ加える、の 3 つで `catalogs` のテストが失敗することを確かめる。あわせて、`Catalog` の最上位の項目と `Record` の節を省略できる型（`?` や `Partial<…>`）にすると欠けのテストが失敗し、対照のモジュールから項目を除くと対照のテストが失敗することを確かめる。

**完了の条件:** `make ext-check` が通る。コミットする。

#### ステップ 2-3: 振り分けと表示の関数に言語を渡す

**変更するファイル:** `src/core/messages.ts`、`src/core/sendMessages.ts`、`src/ui/render.ts`、`src/ui/renderSend.ts`、`src/launch.ts`、`src/popup.ts`、`test/messages.test.ts`・`test/sendMessages.test.ts`・`test/render.test.ts`・`test/renderMetadata.test.ts`・`test/renderSend.test.ts`・`test/launch.test.ts`・`test/metadataDocs.test.ts`

-   [x] `messages.ts`・`sendMessages.ts` の関数を、設計書 3.2 の新しい形（言語を受け取り、`catalogs[locale]` から文言を返す）にする。`switch` を残す関数と `Record` の引き当てに置き換える関数は、設計書 3.2 のとおりに分ける。
-   [x] `render.ts` の `renderSummary`・`renderMetadata` を、設計書 3.3 のとおり言語を受け取る形にする。要約の欄は `SummaryField` をキーにした表でクラス名と表示の向きを持ち、欄の名前を `summaryLabels` から、数を `formatCount` で表示する。`metadataRows` は `label` を持たない形にして、欄の名前を `metadataLabels` から取る（設計書 3.2・3.3）。
-   [x] `renderSend.ts` から `sendingMessage` を除き、`renderSendOutcome`・`renderBlockReason` を言語を受け取る形にする（1.4 の台帳）。
-   [x] `launch.ts` の `PopupLaunchDeps`・`PopupSendDeps` に `locale` を加え、表示の関数へ渡す。送信中の案内と送信ボタンの名前は `catalogs[deps.locale]` の `sending`・`sendButton` から取る。ログ（`logSend`・`readMetadata`・`deps.log.*`）は `deps.locale` を参照しない（設計書 3.3）。
-   [x] `popup.ts` の 2 つの依存に `locale: "en"` を加える（1.3）。
-   [x] 1.4 の古いコメント（`src/popup.ts:1-3` を除く）を、新しい形に合わせて書き直す。
-   [x] 既存のテストの呼び出しに `"en"` を渡し、`sendingMessage` の import（`test/launch.test.ts:20`）を `catalogs.en.sending` に置き換えて、今の検査のまま通す。関数を `.map` に直接渡す呼び出し（`test/sendMessages.test.ts:207-211`）は、配列の添字が言語の引数に入るので、言語を渡す関数に書き換える。英語の欄の名前を文字列で書いているテスト（`test/render.test.ts:27-31`、`test/renderMetadata.test.ts:30-37`・`:56-58`）は文字列のまま残す（設計書 3.8）。`metadataDocs.test.ts` は `expandPromptMessage("en")` と比べる（AC-23、H-01）。

**完了の条件:** `make ext-check` が通る。コミットする。

#### ステップ 2-4: 両方の言語で確かめるテスト

**変更するファイル:** `test/helpers/dom.ts`、`test/messages.test.ts`・`test/sendMessages.test.ts`・`test/render.test.ts`・`test/renderMetadata.test.ts`・`test/renderSend.test.ts`・`test/launch.test.ts`

-   [x] `expectMessage` を `test/helpers/dom.ts` へ移し（1.3）、`test/launch.test.ts`・`test/renderSend.test.ts` の 2 つの定義を除く。
-   [x] `messages.test.ts::rejectionMessage` と `sendMessages.test.ts::blockMessage`・`::serverRejectMessage`・`::sendOutcomeMessage` のテストを、`locales` の各言語について繰り返す形にする。言語によらない語（`Slack`・`4 MiB`・`yt2column-nmhost install`・`yt2column-server`）の検査はそのまま両方の言語に当て、言語によって字句が異なる性質（結果不明の案内の語の並び、オプションページ・トークン・ポートの禁止する語）は、テストの中の言語ごとの表で確かめる（設計書 3.9、AC-09）。
-   [x] 同じ `describe` に、各振り分けの関数がすべての理由と結果について、`catalogs[locale]` の対応する節の文言を返すことを、両方の言語で確かめるテストを加える。`blockMessage` の `body-too-large` は `preflight` の節、収集の拒否の理由は `rejection` の節と比べる。`messages.test.ts::expandPromptMessage` を加え、`catalogs[locale].expandPrompt` を返すことを確かめる（AC-05）。
-   [x] `render.test.ts::renderSummary`・`renderMetadata.test.ts::renderMetadata`・`renderSend.test.ts::renderSendOutcome`・`::renderBlockReason`・`launch.test.ts::popup send` のテストを両方の言語で実行し、表示された文言と欄の名前・送信ボタンの名前・送信中の案内が `catalogs[locale]` のものと一致することを確かめる。値の検査は `catalogs[locale]` の欄の名前で値を探す（1.4）。もう一方の言語の文言を表示しないことの確認は、`en` と `ja` で字句が異なる項目に限る。その項目の字句が実際に異なることを先に確かめてから、もう一方の言語の文言が表示されないことを確かめる（設計書 3.9、AC-03・AC-05）。値をそのまま表示するテストと、マークアップを文字列として表示するテストも両方の言語で実行する（AC-08・AC-24）。
-   [x] `render.test.ts::renderSummary` に、文字数が 4 桁以上の要約を両方の言語で描画し、桁区切りが入ることを確かめるテストを加える（AC-13）。
-   [x] `launch.test.ts::logs by locale` に、同じ fake の依存で言語だけを `en` と `ja` に変えて `runPopupLaunch`・`runPopupSend`（送信まで）を実行し、記録したログ（ラベルと値）が一致することを確かめるテストを加える（AC-12）。
-   [x] 壊して確かめる（各テストの経路に 1 つずつ）:
    -   振り分け: `blockMessage` の `body-too-large` に `preflight` 以外の節の文言を返させる。`serverRejectMessage` が言語を無視して `catalogs.en` から取る。`notSentMessage` の 2 つの原因のキーを入れ替える。
    -   表示: `renderSummary`・`renderMetadata`・`renderSendOutcome` が文言と欄の名前を常に `catalogs.en` から取る。`runPopupSend` が `sendButton`・`sending` を常に `catalogs.en` から取る。`renderSummary` の数を `String` に戻す。`ja` で値を書き換えて表示する（AC-08）。`ja` の表示で `textContent` の代わりに HTML として解釈する経路を作る（lint を一時的に外して確かめ、元に戻す。AC-24）。
    -   性質とログ: `ja.ts` の結果不明の 1 つの文言から「確かめて」を除く。`runPopupSend` のログに `deps.locale` を加える。

**完了の条件:** `make ext-check` が通る。コミットする。

#### ステップ 2-5: 英語の文言の差分の確認

**変更するファイル:** `docs/tasks/0043_extension_i18n/_checks/english_text.ts`（新規）、本書

-   [x] 設計書 3.9 の「英語の文言が変わらないこと」のとおりに、merge-base と head の 2 つの版の表示された英語の文字列を同じ方法で取り出して比べるスクリプトを書く（1.3）。
    -   merge-base は `git merge-base origin/main HEAD`、head は `HEAD` とする。2 つの版の `extension/` のソースは、作業ツリーではなくコミットから一時的なディレクトリに取り出し、どちらも作業ツリーの `extension/node_modules` を使う。merge-base と head の `extension/package-lock.json` が同じことを、前提として先に確かめる。`extension/node_modules` は、実行の前に `make ext-install` で head の lockfile から入れたものとする。
    -   次の関数を、どちらの版も head の呼び出しの形で実行する（1.3）。
        -   `renderSummary`: すべての収集の拒否の理由と、文字数と行数が 1000 未満の要約。
        -   `renderMetadata`: すべての欄と促しのメッセージ。
        -   `renderSendOutcome`: すべての結果の種類・サーバの拒否の理由・未送信の原因・想定外の検査。
        -   `renderBlockReason`: すべての送信しなかった理由。
        -   `runPopupSend`: ボタンの名前と、ボタンを押した後の送信中の案内。
    -   表示された文字列は、要素の境目が分かる形（欄の名前・値・本文の `p`・手順の `li` ごとに 1 行）で書き出す。
    -   一致すれば合格、一致しなければ違う行を出して 0 以外で終わる。判定の名前に AC の番号を使わない。
-   [x] 壊して確かめる: push しない使い捨てのブランチで、`en.ts` の 1 つの文言の 1 文字を変えたコミット、1 つの欄の名前を変えたコミット、本文の末尾の 1 語を最初の手順の先頭へ移したコミットを作り、スクリプトがそれぞれ不合格になることを確かめてから、ブランチを消す。結果をスクリプトのコミットのメッセージに書く。
-   [x] `git fetch origin main`（`_lib/task-pr-handoff.md` の Authorization が `/runplan` に認める）の後、PR-1 の head でスクリプトを実行し、出力と、merge-base と head の 2 つのコミットの ID を 7 章の AC-07 の行に記録する。記録を本書にコミットする。この後、`extension/src/` が変わったときや `main` を取り込んだときは、実行し直して記録を置き換える。

**完了の条件:** `make ext-check`・`make test`・`make lint` が通り、7 章の AC-07 の行に PR-1 の記録がある。

### PR-1 作成ポイント: CLAUDE.md language rules and i18n foundation

**対象ステップ**: 1-1 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0043): 拡張の文言を英語と日本語の文言の集合に移す`

**レビュー観点**: `en.ts` の英語の文言が今の字句と一字一句同じで、差分の確認の記録が 7 章にあるか / `ja.ts` の文言が設計書 3.4 の表と一致し、用語集の用語を使っているか / 振り分けの関数が、残す `switch` と `Record` の引き当てを設計書 3.2 のとおりに分けているか / `CLAUDE.md` の 2 つの改定が設計書 3.1・3.7 の範囲と一致するか

**実装モデル要件**: frontier-recommended

**判定理由**: 危険の大きい手順が 2 つある。ステップ 2-3 は文言を持つすべての関数と表示の経路を一度に書き換え、ステップ 2-5 は 2 つの版の拡張のコードを取り出して実行し、表示を比べる

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/520
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 入口とページ

#### ステップ 3-1: 表示言語の読み取り

**変更するファイル:** `src/browser/chromeDeps.ts`、`test/chromeDeps.test.ts`

-   [x] 設計書 3.1 のとおり、`I18nApi` と `readUiLanguage` を加える。
-   [x] `test/chromeDeps.test.ts::readUiLanguage` に、文字列を返す fake でその値を返すこと、例外を投げる fake と文字列でない値を返す fake で `undefined` を返し例外を投げないことを確かめるテストを加える（AC-02）。
-   [x] 壊して確かめる: `try`/`catch` を除く、文字列かどうかの確かめを除く、の 2 つでテストが失敗することを確かめる。

**完了の条件:** `make ext-check` が通る。コミットする。

#### ステップ 3-2: ページの言語

**変更するファイル:** `src/ui/pageLocale.ts`（新規）、`test/locale.test.ts`

-   [x] 設計書 3.5 のとおり、`applyPageLocale` を書く。
-   [x] `test/locale.test.ts::applyPageLocale` に、jsdom の文書に `en`・`ja` のそれぞれを適用して、`html` 要素の `lang` 属性がその値になることを確かめるテストを加える（AC-14）。
-   [x] 壊して確かめる: `lang` 属性を `html` 要素ではなく `body` 要素に設定する、常に `en` を設定する、の 2 つでテストが失敗することを確かめる。

**完了の条件:** `make ext-check` が通る。コミットする。

#### ステップ 3-3: ポップアップの入口

**変更するファイル:** `src/launch.ts`、`src/popup.ts`、`test/launch.test.ts`

-   [x] 設計書 3.3 のとおり、`PopupStartDeps` と `startPopup` を `launch.ts` に加える。表示言語を読んだ直後に、固定のラベル（`yt2column ui language`）で読んだ値をログに出し、`resolveLocale`・`applyPageLocale` の後に `runPopupLaunch`・`runPopupSend` を呼ぶ（設計書 6 章の順序）。
-   [x] `popup.ts` を、ブラウザの API から `PopupStartDeps` を組み立てて `startPopup(document, …)` を呼ぶだけの形にする。ステップ 2-3 の `locale: "en"` を除く。表示言語は `readUiLanguage(chrome.i18n)` で読む。ファイルの冒頭のコメント（`src/popup.ts:1-3`）を新しい形に合わせる。
-   [x] `test/launch.test.ts::startPopup` に、`startPopup` を jsdom の文書（`#summary`・`#send` を持つ）と fake の依存で実行するテストを加える（設計書 3.9 の「入口の組み立て」）。表示言語の依存は、`getUILanguage` を差し替えた `I18nApi` の fake を `readUiLanguage` に渡す関数（`() => readUiLanguage(fake)`）として作る。
    -   `getUILanguage` が `ja-JP` を返すとき、`lang` 属性が `ja` で、表示が `catalogs.ja` の文言であること（AC-01・AC-03）。
    -   `getUILanguage` が例外を投げるとき、`lang` 属性が `en` で、英語で表示され、収集の結果・ログの収集の結果・送信のボタンの表示が、`getUILanguage` が `en-US` を返すときと同じであること（AC-02）。
    -   fake の `activeTab` が呼ばれた時点で `lang` 属性が設定済みであること（AC-14）。
    -   表示言語のログが固定のラベルで 1 つだけ、読んだ値のまま（例外のときは `undefined`）記録されること（AC-12）。
-   [x] 壊して確かめる: `applyPageLocale` を `runPopupLaunch` の後に呼ぶ、`startPopup` が `runPopupLaunch`・`runPopupSend` に常に `"en"` を渡す、表示言語のログのラベルに言語を含める、の 3 つでテストが失敗することを確かめる。

**完了の条件:** `make ext-check` が通る。コミットする。

#### ステップ 3-4: 日本語の書体と行間

**変更するファイル:** `static/style.css`、`test/style.test.ts`（新規）

-   [x] 設計書 3.5 のとおり、`:root:lang(ja)`（書体の候補）と `body:lang(ja)`（`line-height: 1.7`）の 2 つの規則を `style.css` の末尾に加える。今の規則は変えない。
-   [x] `test/style.test.ts::Japanese typography` に、`style.css` を jsdom の CSSOM で読み、設計書 3.9 の「ページの言語・書体・行間」の 4 つの性質を確かめるテストを書く（AC-15〜AC-17）。
-   [x] 壊して確かめる: 書体の候補の順を入れ替える、行間を `1.5` にする、セレクタを `:lang(ja)` だけにする、`:root` の書体を変える、`body` の規則に `line-height` を加える、の 5 つで、それぞれテストが失敗することを確かめる。

**完了の条件:** `make ext-check` が通る。コミットする。

#### ステップ 3-5: 差分の確認の実行（PR-2）

**変更するファイル:** 本書

-   [x] `git fetch origin main` の後、PR-2 の head でステップ 2-5 のスクリプトを実行し、出力と 2 つのコミットの ID を 7 章の AC-07 の行に加える。記録を本書にコミットする。この後、`extension/src/` が変わったときや `main` を取り込んだときは、実行し直して記録を置き換える。

**完了の条件:** 7 章の AC-07 の行に PR-2 の記録がある。

#### ステップ 3-6: ポップアップを実機で確かめる

**変更するファイル:** 本書

-   [x] `make ext-build` でビルドした `extension/dist/` を Chrome で読み込み（Brave は、Chrome と Brave に固有のコードがないため実施しない。2026-10-27、レビュアーの判断）、ブラウザの表示言語を日本語と英語に切り替えて（設計書 7.2 の切り替え方）、次を確かめる。
    -   日本語のとき、ポップアップが日本語で表示され、書体が日本語の書体になり、英語の文言が一瞬も表示されないこと。英語のとき、今と同じ英語で表示されること（AC-01〜AC-03・AC-14〜AC-16）。
    -   ポップアップのコンソールの表示言語のログと `document.documentElement.lang` の値。
    -   リクエスト ID が等幅の書体のままであること。リクエスト ID はサーバの応答（受け付けたか拒否した）でだけ表示される。LLM の API を呼ばずにリクエスト ID を得るには、字幕の書式でないテキストを選んで送り、サーバに `invalid_transcript` で拒否させる。拒否でもサーバが Slack に失敗の通知を投稿しうるので、この送信はレビュアーの承認を得てから行う（`CLAUDE.md`「Tool Execution Safety」）。承認されないときは、この項目を `style.test.ts`（`:lang(ja)` の規則が `html` と `body` にだけ当たること）に任せたと記録する。
-   [x] ブラウザの操作はレビュアー（利用者）に依頼する。`/runplan` は、ステップ 3-5 の後、PR を開く前にここで止まり、操作を依頼する。結果（日付・ブラウザと版・表示言語・観察したこと）を 7 章の該当する AC の行に記録し、本書にコミットする。

**完了の条件:** `make ext-check`・`make test`・`make lint` が通り、7 章に PR-2 の実機の確認の記録がある。

### PR-2 作成ポイント: popup entry, page language and Japanese typography

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6

**推奨タイトル**: `feat(0043): ブラウザの表示言語に合わせてポップアップを日本語で表示する`

**レビュー観点**: `startPopup` が最初の描画の前に `lang` 属性を設定し、そのことをテストが確かめているか / 表示言語を調べるのが `resolveLocale` だけで、`popup.ts` が組み立てだけをしているか / `:lang(ja)` の規則が `html` と `body` にだけ当たり、リクエスト ID の等幅の書体を上書きしないか / 差分の確認と実機の確認の記録が 7 章にあるか

**実装モデル要件**: standard

**判定理由**: 前例のない設計判断も競合する実装案もなく、条件付きの検査に 2 つ以上当たる手順もない

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/525
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: manifest と `_locales`

#### ステップ 4-1: manifest と `_locales` を変える

**変更するファイル:** `static/manifest.json`、`static/_locales/en/messages.json`・`static/_locales/ja/messages.json`（新規）、`test/manifest.test.ts`

-   [ ] 設計書 3.6 の 2 つの表のとおりに、manifest を変え、2 つの `messages.json` を書く。
-   [ ] `test/manifest.test.ts::undeclared keys`（`:28-41`）に `default_locale`・`description` を加え、`test/manifest.test.ts::fixed fields` の `name` の検査（`:57-60`）を `__MSG_extName__` にする。`test/manifest.test.ts::locales` を加え、設計書 3.9 の「manifest と `_locales`」の検査（`default_locale`、`description` の形、`_locales` のディレクトリと各 `messages.json` の形、`__MSG_*__` の名前の解決、`extName`・`extDescription` の値）を書く（AC-18〜AC-20）。
-   [ ] 壊して確かめる: `default_locale` を除く、`ja` の `messages.json` から `extDescription` を除く、`messages.json` を JSON として読めない形にする、`message` に `$` を入れる、manifest の `__MSG_extName__` を `messages.json` にない名前にする、`ja` の `extName` を `yt2column` 以外にする、`ja` の `extDescription` を `en` と同じにする、`extDescription` を 133 文字にする、`_locales` に 3 つめのディレクトリを加える、の 9 つで、それぞれテストが失敗することを確かめる。
-   [ ] `make ext-build` を実行し、`check-dist` が `dist/_locales/en/messages.json`・`dist/_locales/ja/messages.json` を含む `dist/` を受け入れることを確かめる（AC-18）。

**完了の条件:** `make ext-check` が通る。コミットする。

### フェーズ 5: README と管理ページの確認

#### ステップ 5-1: README に言語の決まり方を書く

**変更するファイル:** `README.md`

-   [ ] 「Browser extension」の節の、拡張に設定がないことを述べる段落（`README.md:1002-1006`）の後に、設計書 3.7 の段落（表示言語が日本語なら日本語、それ以外は英語で表示されること、拡張の中に言語の設定がないこと、README が引用するポップアップの文言は英語版であること）を英語で加える（AC-22）。促しのメッセージの引用（`README.md:1016-1020`）は変えない（AC-23）。

**完了の条件:** `make ext-test`（README を読む拡張のテスト）と `make test`（`TestReadmeHostExitCodes`）が通る。コミットする。

#### ステップ 5-2: 拡張の管理ページを実機で確かめる

**変更するファイル:** 本書

-   [ ] `make ext-build` でビルドした `extension/dist/` を Chrome と Brave で読み込み、ブラウザの表示言語を日本語と英語に切り替えて、拡張の管理ページ（`chrome://extensions/`）の名前が `yt2column`、説明がその言語の `extDescription` であること（AC-19）と、拡張 ID が `clfmbbcdpnjcefbdihdoahomaifbabkk` のままであること（AC-20）を確かめる。ステップ 3-6 と同じく、`/runplan` はここで止まり、ブラウザの操作をレビュアーに依頼する。結果を 7 章の AC-19・AC-20 の行に記録し、本書にコミットする。

**完了の条件:** `make ext-check`・`make test`・`make lint` が通り、7 章に PR-3 の実機の確認の記録がある。

### PR-3 作成ポイント: manifest locales and README

**対象ステップ**: 4-1 / 5-1 / 5-2

**推奨タイトル**: `feat(0043): 拡張の名前と説明を多言語にし、README に表示言語の決まり方を書く`

**レビュー観点**: `_locales` の形が設計書 3.6 の Chrome の制約に従い、テストがそれを確かめているか / manifest の最上位のキーと拡張 ID が要件書 AC-20 のとおりか / README の段落が設計書 3.7 の 3 点を述べているか / 管理ページの実機の確認の記録が 7 章にあるか

**実装モデル要件**: standard

**判定理由**: 前例のない設計判断も競合する実装案もなく、条件付きの検査に 2 つ以上当たる手順もない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

フェーズの順序は設計書 8 章の順序に従う。

| マイルストーン | フェーズ | 成果物 |
|---|---|---|
| M1: 規則 | 1 | `CLAUDE.md` の拡張の言語の規則と「Declare, don't infer」の例外 |
| M2: 多言語の土台 | 2 | `src/core/i18n/`、言語を受け取る振り分けと表示の関数、両方の言語のテスト、差分の確認（表示は英語のまま） |
| M3: 日本語の表示 | 3 | `readUiLanguage`・`applyPageLocale`・`startPopup`、`:lang(ja)` の規則、ポップアップの実機の確認 |
| M4: manifest と文書 | 4・5 | `_locales`・manifest、README の段落、管理ページの実機の確認 |

フェーズ 1 は、日本語の文字列リテラルを加えるフェーズ 2 より前にある（設計書 8 章）。フェーズ 3 はフェーズ 2 の言語を受け取る関数を前提にする。フェーズ 4 はフェーズ 2・3 と独立である。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | `CLAUDE.md` の規則、文言の集合、言語を受け取る振り分けと表示の関数、両方の言語のテスト、差分の確認 | frontier-recommended |
| PR-2 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 | 表示言語の読み取り、ページの言語、`startPopup`、日本語の書体と行間、差分の確認の再実行、ポップアップの実機の確認 | standard |
| PR-3 | 4-1 / 5-1 / 5-2 | manifest と `_locales`、README の段落、管理ページの実機の確認 | standard |

分け方の理由は 1.3 のとおりである。各 PR は、それより前の PR だけを前提にし、単独でグリーンゲートと `make ext-check` を通る。

## 4. テスト戦略 (Test Strategy)

### 4.1. 加える・変えるテスト

テストの構成は設計書 3.9 が定める。本書では、場所とステップだけを示す。すべて `make ext-test`（`node --test`）で実行し、ブラウザ・ネイティブメッセージングのホスト・ネットワークを使わない。

| テスト | 場所 | ステップ |
|---|---|---|
| 言語の判定・数の書式・ページの言語 | `test/locale.test.ts::resolveLocale`・`::formatCount`・`::applyPageLocale`（新規） | 2-1・3-2 |
| 文言の集合の空でないこと・手順の規則 | `test/catalog.test.ts::catalogs`（新規） | 2-2 |
| `Catalog` の欠けで型の検査が失敗すること | `test/typecheck.test.ts::a Catalog missing an item fails typecheck` | 2-2 |
| 振り分けの関数の性質と節の対応（両方の言語） | `test/messages.test.ts::rejectionMessage`・`::expandPromptMessage`、`test/sendMessages.test.ts::blockMessage`・`::serverRejectMessage`・`::sendOutcomeMessage` | 2-3・2-4 |
| 表示と経路の処理（両方の言語） | `test/render.test.ts::renderSummary`、`test/renderMetadata.test.ts::renderMetadata`、`test/renderSend.test.ts::renderSendOutcome`・`::renderBlockReason`、`test/launch.test.ts::popup send` | 2-3・2-4 |
| ログが言語によらないこと | `test/launch.test.ts::logs by locale` | 2-4 |
| README の促しのメッセージ | `test/metadataDocs.test.ts::video metadata documentation` | 2-3 |
| 表示言語の読み取り | `test/chromeDeps.test.ts::readUiLanguage` | 3-1 |
| 入口の組み立て | `test/launch.test.ts::startPopup` | 3-3 |
| 書体と行間 | `test/style.test.ts::Japanese typography`（新規） | 3-4 |
| manifest と `_locales` | `test/manifest.test.ts::undeclared keys`・`::fixed fields`・`::locales` | 4-1 |

-   **言語の繰り返し:** 両方の言語で確かめるテストは、`locales` のキーをたどる。言語の型に値を加えると、`locales` にも加えないと型の検査が失敗するので、テストも新しい言語を自動でたどる。
-   **数の書式の前提:** `formatCount` のテストは、ICU のデータの前提（`supportedLocalesOf`）を先に確かめる（H-10）。
-   **壊して確かめる:** 各テストの壊し方は、各ステップに挙げた。実装者が壊して失敗を確かめ、コミットのメッセージに書く（1.2）。

### 4.2. 差分の確認

AC-07 は、`docs/tasks/0043_extension_i18n/_checks/english_text.ts` で確かめる（ステップ 2-5・3-5、設計書 3.9）。PR-1 と PR-2 のブランチでそれぞれ実行し、結果を 7 章に記録する。`make test`・`make ext-test` と CI では実行せず、マージの後は保守しない（`requirements_process.md` §4）。

### 4.3. 統合テスト（手動）

設計書 7.2 の確認を、ポップアップの部分はステップ 3-6（PR-2）、拡張の管理ページの部分はステップ 5-2（PR-3）で行う（1.3）。

### 4.4. 既存のテスト

既存のテストの更新は、設計書 3.8 の表の更新（manifest の最上位のキーと `name`、関数の呼び出しへの言語の追加、`sendingMessage` の置き換え）と、ステップ 2-4 の言語ごとの繰り返し・`expectMessage` の移動に限る。今の英語での検査の内容は、言語ごとに繰り返す形にした後も `en` の繰り返しに残す。権限の検査（`test/manifest.test.ts:17-26`）、`chrome.storage` と HTML を解釈する API の lint の規則（`test/lintRules.test.ts`）、秘密情報の語の検査（`test/secretsAbsent.test.ts`）は、変えずに新しいファイルにも当たる。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| `en.ts` へ写すときに英語の文言を書き誤る | 英語の利用者の表示が変わる（AC-07） | ステップ 2-5 の差分の確認で、表示された文字列を merge-base と比べる |
| 欄の名前を取り違える（例: 要約の `Title` に動画タイトルの名前を使う） | 欄の意味が変わる | 英語の欄の名前を文字列で書いた既存のテストを残す（設計書 3.8）。差分の確認は表示された文字列を比べる |
| `_locales` の形が Chrome の制約に反する | すべての利用者で拡張を読み込めない | ステップ 4-1 のテストが形を確かめる。ステップ 5-2 で実機に読み込む |
| `lang` 属性の設定が最初の描画より遅れる | 英語の書体で一瞬表示される | ステップ 3-3 のテストが、`activeTab` が呼ばれた時点で `lang` 属性が設定済みであることを確かめる。ステップ 3-6 で実機で見る |
| 実行環境の ICU のデータが一部しかない | `formatCount` のテストが失敗する | 前提の検査を先に置き、書式の不一致と前提の不成立を分けて報告させる（設計書 3.3） |
| 差分の確認のスクリプトが merge-base の版を正しく実行できない | AC-07 を確かめられない | ステップ 2-5 で、壊したコミットに対して不合格になることを確かめる。1.4 で、取り出した版を `node` で実行できることを確かめた |
| 実機の確認のための送信が Slack に失敗の通知を投稿する | 意図しない投稿 | ステップ 3-6 で、送信の前にレビュアーの承認を得る |

日程のリスク: PR-1 が大きい。ステップごとにコミットを分けるので、レビュアーはコミットの単位で読める。PR-2・PR-3 は、実機の確認でレビュアーの操作を待つ。

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5）
-   [ ] PR-2 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6）
-   [ ] PR-3 マージ済み（対象ステップ: 4-1 / 5-1 / 5-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` はテスト、`static` は `make` のターゲット、`diff` は差分の確認のスクリプト、`manual` は PR のレビューでの読み取りか実機の確認を指す。テストの場所は `path::describe の名前` で示す（2 章の冒頭）。

| AC | 確かめ方 | 実装 | 内容 |
|---|---|---|---|
| AC-01 | test: `extension/test/locale.test.ts::resolveLocale`・`extension/test/launch.test.ts::startPopup` ＋ manual | 2-1・3-3・3-6 | 日本語と判定する値と、入口が日本語で表示することはテスト。実機（PR-2、下の確認の記録）: 日本語の表示言語で日本語で表示された。 |
| AC-02 | test: `extension/test/locale.test.ts::resolveLocale`・`extension/test/chromeDeps.test.ts::readUiLanguage`・`extension/test/launch.test.ts::startPopup` ＋ manual | 2-1・3-1・3-3・3-6 | 英語と判定する値、読み取りの失敗、失敗しても収集と送信が変わらないことはテスト。実機（PR-2）: 英語の表示言語で英語で表示された。 |
| AC-03 | test: `extension/test/render.test.ts::renderSummary`・`extension/test/renderMetadata.test.ts::renderMetadata`・`extension/test/renderSend.test.ts::renderSendOutcome`・`::renderBlockReason`・`extension/test/launch.test.ts::popup send`・`::startPopup` ＋ manual | 2-4・3-3・3-6 | 選んだ言語の文言を表示し、もう一方の言語の文言を表示しないこと。実機（PR-2）: どちらの言語でも、もう一方の言語の文言は表示されなかった。 |
| AC-04 | test: `extension/test/manifest.test.ts::permissions`・`::undeclared keys`・`extension/test/lintRules.test.ts::chrome.storage`（既存） | 4-1 | 権限が 3 つのままで、`options_ui` がなく、`chrome.storage` を参照できないこと |
| AC-05 | test: `extension/test/messages.test.ts::rejectionMessage`・`::expandPromptMessage`・`extension/test/sendMessages.test.ts::blockMessage`・`::serverRejectMessage`・`::sendOutcomeMessage` ＋ AC-03 の表示のテスト | 2-3・2-4 | 振り分けの関数がすべての理由と結果について選んだ言語の節の文言を返すこと、表示がそれを表示すること |
| AC-06 | static: `make ext-typecheck` ＋ test: `extension/test/typecheck.test.ts::a Catalog missing an item fails typecheck`・`extension/test/catalog.test.ts::catalogs` | 2-2 | 欠けで型の検査が失敗すること、本文・手順が空でないこと、手順の規則 |
| AC-07 | diff: `docs/tasks/0043_extension_i18n/_checks/english_text.ts` | 2-2・2-3・2-5・3-5 | 実行の記録（PR-1）: 2026-10-27、merge-base `30dbae078cbdd267f2ca1ff1bb27bf3ef3cd6f5d`、head `5e3ab74d77d85ad0a7869ea74cc21081588ecb8c`、終了コード 0、出力 `PASS: 121 displayed elements identical`。実行の記録（PR-2）: 2026-10-27、merge-base `9b4c0ef6794f0469244780869d42e8ec25275fca`、head `44b9822687c00618a0b8b235d5eb2f6bab0c7d92`、終了コード 0、出力 `PASS: 121 displayed elements identical`。 |
| AC-08 | test: `extension/test/render.test.ts::renderSummary`・`extension/test/renderMetadata.test.ts::renderMetadata`・`extension/test/renderSend.test.ts::renderSendOutcome` | 2-4 | 両方の言語で、値をそのまま表示すること |
| AC-09 | test: `extension/test/messages.test.ts::rejectionMessage`・`extension/test/sendMessages.test.ts::blockMessage`・`::serverRejectMessage`・`::sendOutcomeMessage` | 2-4 | 両方の言語で、各性質を言語によらない語と言語ごとの語の表で確かめる |
| AC-10 | manual | 2-2 | 日本語の文言の用語を用語集と照らす（PR-1 のレビュー。テストにしない理由は 1.3） |
| AC-11 | manual | 2-2 | 日本語の文言の YouTube のボタンの表記を、設計書 3.4 の観測の記録と照らす（PR-1 のレビュー。テストにしない理由は 1.3） |
| AC-12 | test: `extension/test/launch.test.ts::logs by locale`・`::startPopup` | 2-4・3-3 | 言語だけを変えて記録したログが一致すること、表示言語のログのラベルが固定であること。実機（PR-2）: コンソールの `yt2column ui language` のログが表示言語に一致した。 |
| AC-13 | test: `extension/test/locale.test.ts::formatCount`・`extension/test/render.test.ts::renderSummary` | 2-1・2-4 | 両方の言語で `12,345`、要約の表示に桁区切りが入ること |
| AC-14 | test: `extension/test/locale.test.ts::applyPageLocale`・`extension/test/launch.test.ts::startPopup` ＋ manual | 3-2・3-3・3-6 | `lang` 属性と、最初の描画の前に設定されること。実機（PR-2）: `document.documentElement.lang` が `ja` / `en` であった。 |
| AC-15 | test: `extension/test/style.test.ts::Japanese typography` ＋ manual | 3-4・3-6 | `:root:lang(ja)` の書体の候補の順。実機（PR-2）: 日本語のとき、書体が日本語の書体になった。 |
| AC-16 | test: `extension/test/style.test.ts::Japanese typography` ＋ manual | 3-4・3-6 | `body:lang(ja)` の行間。実機（PR-2）: 行間が広く見え、DevTools で `body:lang(ja) { line-height: 1.7; }` の適用を確認した。 |
| AC-17 | test: `extension/test/style.test.ts::Japanese typography` | 3-4 | 英語の書体が今のままで、ほかの規則が行間を持たないこと |
| AC-18 | static: `make ext-build`（`check-dist`） ＋ test: `extension/test/manifest.test.ts::locales` | 4-1 | `dist/` が `_locales` の 2 つのファイルを含むこと、`default_locale` が `en` で `_locales` の形が正しいこと |
| AC-19 | test: `extension/test/manifest.test.ts::fixed fields`・`::locales` ＋ manual | 4-1・5-2 | `name`・`description` が `_locales` を参照し、名前が両方の言語で `yt2column`、説明が言語ごとに異なること。拡張の管理ページの表示は実機の確認（ステップ 5-2） |
| AC-20 | test: `extension/test/manifest.test.ts::undeclared keys`・`::key` ＋ manual | 4-1・5-2 | 最上位のキーの集合と、`key` から得る拡張 ID。拡張の管理ページの ID は実機の確認（ステップ 5-2） |
| AC-21 | manual | 1-1 | `CLAUDE.md` の規則の文面（PR-1 のレビュー） |
| AC-22 | manual | 5-1 | README の段落の文面（PR-3 のレビュー） |
| AC-23 | test: `extension/test/metadataDocs.test.ts::video metadata documentation` | 2-3 | README の引用が `expandPromptMessage("en")` と一致すること |
| AC-24 | test: `extension/test/lintRules.test.ts::html parsing APIs`（既存）・`extension/test/render.test.ts::renderSummary`・`extension/test/renderMetadata.test.ts::renderMetadata`・`extension/test/renderSend.test.ts::renderSendOutcome`・`::renderBlockReason` | 2-4 | lint の規則と、両方の言語で文言と値を文字列として表示すること |

`manual` だけの AC は AC-10・AC-11・AC-21・AC-22 である。AC-21・AC-22 は文書の文面だけを定める AC である（`requirements_process.md` §4）。AC-10・AC-11 をテストにしない理由は 1.3 に書いた。

**実機の確認の記録（PR-2、ステップ 3-6）:** 2026-10-27、レビュアー（isseis）が Chrome 156.0.8078.48（公式ビルド、arm64）で確認した。OS の環境設定は日本語とし、Chrome を既定の起動で開いた場合（日本語）と `open -a "Google Chrome" --args -AppleLanguages '(en)'` で起動した場合（英語）の 2 つを試した。日本語のときはポップアップが日本語で表示され、`document.documentElement.lang` が `ja`、コンソールの `yt2column ui language` のログが表示言語に一致した。英語のときは今までどおり英語で表示され、`lang` が `en` であった。どちらのときも、もう一方の言語の文言は表示されなかった。日本語のとき、書体が日本語の書体になり、行間が広く見えた（DevTools で `body:lang(ja) { line-height: 1.7; }` の適用を確認）。リクエスト ID は、字幕でないテキストを送って `invalid_transcript` の拒否を受け、DevTools で `.send-request-id` の `font-family: ui-monospace, monospace` が適用されていることを確認した。Brave は、Chrome と Brave に固有のコードがないため実施していない（レビュアーの判断）。

## 8. 成功基準 (Success Criteria)

-   **機能の完全性:** 7 章のすべての AC の確かめ方が実行され、`test`・`static`・`diff` が合格し、`manual` の行が PR のレビューと実機の確認で確かめられている。
-   **品質:** 各 PR で `make ext-check`・`make test`・`make lint` が通る。新しいテストと言語ごとに繰り返す形にしたテストは、壊し方で失敗することが確かめられ、コミットのメッセージに書かれている。
-   **セキュリティ:** 文言は `textContent` だけで表示される（AC-24）。manifest の `permissions` は増えない（AC-04）。表示言語の文字列は、ログにだけ出て、表示にも送信にも使われない（設計書 5.1）。
-   **文書:** `CLAUDE.md` の 2 つの改定と README の段落が、設計書 3.7 と一致する。

## 9. 次のステップ

-   本書の承認の後、`/runplan 0043` で PR-1（フェーズ 1・2）を実装して開く。PR-2・PR-3 も、前の PR のマージの後に `/runplan 0043` で同じように進める。
-   PR-3 のマージの後、issue #506 を閉じる（`/runpr` が最後の PR の後に案内する）。
