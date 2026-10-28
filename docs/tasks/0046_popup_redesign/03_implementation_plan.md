# 実装計画書：ポップアップの画面の再設計

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-28 |
| Review date | 2026-10-28 |
| Reviewer | isseis |
| Comments | 2026-10-28 (editorial): `/runplan` の PR-1 のレビューの指摘で、ステップ 1-4 の概要欄の文章の確認を改めた。画面の内容は、押せる主ボタンのために送るリクエスト（本文に概要欄を含む）を持つので、「画面の内容のどこにも入らない」とは言えない。概要欄の行が文章を持つ場所を持たないことを設計（3.2）のとおりにし、表示の確認はステップ 3-2 の描き方のテストで行う。7 章の AC-15 の行も、描き方のテストだけを挙げる形に直した。決定は変えていない。 |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、「設計書 3.2」などは設計書の節を指す。既存のコードと文書についての記述は、コミット `a04e5d82` で確かめた。`file:line` はこのコミットの行番号を指す。拡張のファイルのパスは、設計書と同じく `extension/` を省いて書く（例: `src/launch.ts`）。テストの場所は `path::describe の名前` で示す。

設計書が引く行番号はコミット `f488112` のものである。拡張のディレクトリ（`extension/`）と `docs/translation_glossary.md` は 2 つのコミットで同じなので、拡張のファイルの行番号はそのまま使える（`git diff --stat f488112 a04e5d82 -- extension docs/translation_glossary.md` の出力が空）。`README.md` は、その後の変更で 55 行ずれている。

| 設計書の参照（`f488112`） | 本書の参照（`a04e5d82`） |
|---|---|
| `README.md:1074-1097`（ポップアップの説明） | `README.md:1129-1152` |
| `README.md:1081-1088`（促しのメッセージの引用） | `README.md:1136-1143` |
| `README.md:1090-1097`（値が取れないとき） | `README.md:1145-1152` |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、次を行う。

-   画面の内容（`PopupView`）と、それを作る `buildCollectedView` を加え、確認の状態を型で宣言する（設計書 3.1・3.2）。
-   文言の集合に新しい項目を加え、促しのメッセージを短くする（設計書 3.5）。
-   骨格（`popup.html`）・`renderPopup`・記号の SVG・色の変数を加え、新しい構成で描く（設計書 3.4・3.6・3.7）。
-   経路の処理を、読み取りと判定が終わってから一度に描く形に変える（設計書 3.3）。
-   README と用語集を新しい画面に合わせる（設計書 3.8）。

送信の経路・送信の結果の文言・リクエストの本文・manifest は変えない（要件書 2.3、AC-19・AC-20・AC-35）。Go のソースは変えない。

### 1.2. 実装の方針

-   **設計は設計書に従い、本書は繰り返さない。** 型と関数の形は設計書 3.1〜3.4、文言の字句は設計書 3.5、色の値とコントラストの表は設計書 3.6、テストの構成は設計書 3.9・3.10 が定める。本書のステップは、どの節に従って何を変えるか、どう確かめるかだけを示す。
-   **古い表示は、新しい表示に切り替えるまで残す。** 新しい部品（`popupView.ts`・`renderPopup`・`icons.ts`・CSS の規則・文言の新しい項目）は、古い部品（`summarize`・`renderSummary`・`renderMetadata`・`summaryLabels` など）と並べて加える。古い部品は、経路の処理を切り替えるステップ 4-3 でまとめてなくす。どの PR の後でも、ポップアップは古い表示か新しい表示のどちらかで動く（1.3）。
-   **テストが失敗しうることを、壊して確かめる。** 新しいテストと、確認の内容を変えたテストのそれぞれについて、そのテストが確かめる経路を壊し、テストが失敗することを実装のときに確かめ、結果をコミットのメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。
-   **テストを消すときは、守っていた性質の行き先を確かめる。** 拡張には `go tool cover` がないので、代わりに `node --test --experimental-test-coverage --test-reporter=lcov`（Node.js 24.21.0 で動くことを `a04e5d82` で確かめた）を使う。テストを消す前と後でその出力を比べ、消した関数を除く `src/` の各関数について、実行回数（lcov の `FN`・`FNDA`）が 0 にならないことを確かめる。関数の網羅率からは、確認が消えたことは分からない。そのため、消すテストの確認ごとの行き先を 1.4 に書き、引き継ぐテストに同じ確認があることをレビューで確かめる。比べたコマンドと結果をコミットのメッセージに書く（`CLAUDE.md`「Deleting a test is a claim that must be checked」の拡張での読み替え）。
-   **各ステップの後で `make ext-check` が通る。** 原則としてステップごとにコミットし（例外はステップ 3-4）、どのコミットでも拡張の型の検査・lint・書式・テスト・ビルドが通る状態を保つ。PR の前にはグリーンゲート（`make test && make lint`）も通す。
-   **拡張のソースは英語で書く。** コメント・識別子・文字列リテラルは英語である。例外は `CLAUDE.md`「Go Idioms」の 3 つ（`ja.ts` の文言、日本語の文言の性質を確かめるテストの語の表、日本語の文書に一致させるガードのテストのリテラル）だけである。
-   **新しいソースは秘密情報の語を含まない。** `test/secretsAbsent.test.ts` は `src/` と `static/` のすべてのファイルで、`token`・`webhook` などの語を禁じる。新しいファイル（`popupView.ts`・`icons.ts`）と書き換える `popup.html`・`style.css` にもこの検査が当たる。
-   **フェーズは設計書 8 章の順序に従う。** PR は 4 つに分ける（1.3、3.2）。

### 1.3. 計画の際に決めたこと（2026-10-28）

-   **PR は 4 つに分ける（レビュアーが選んだ）:** PR-1 はフェーズ 1・2（DOM を使わない画面の内容と文言の新しい項目）、PR-2 はステップ 3-1〜3-3（`renderPopup`・記号・CSS を古い表示と並べて加える）、PR-3 はステップ 3-4 とフェーズ 4（骨格に置き換え、経路の処理を切り替え、古い部品をなくす）、PR-4 はフェーズ 5（README・用語集・手動の確認）とする。いちばん危険な切り替え（PR-3）を、それだけでレビューできるようにするためである。3 つに分ける案（フェーズ 3 とフェーズ 4 を 1 つの PR にする）は、描き方・CSS・経路の処理と多くのテストの書き直しが 1 つの PR に集まるので採らなかった。
-   **`popup.html` の骨格はフェーズ 3 の最後のステップ（3-4）に置き、PR-3 に含める:** 今の `startPopup` は `#summary` と `#send` を探し（`src/launch.ts:262-266`）、`test/launch.test.ts::startPopup` は出荷する `popup.html` を読む（`test/launch.test.ts:961-968`）。骨格だけを先に置き換えると、ポップアップが何も表示せず、このテストも失敗する。そのため、骨格と経路の処理の切り替えを同じ PR に入れる。設計書 8 章のフェーズの順序は変えず、PR の境界をフェーズ 3 の中に置く。
-   **PR-3 のマージから PR-4 のマージまで、README は古い画面を説明する:** README の書き直し（ステップ 5-1）は PR-4 で行う。この間、README のボタンの名前（**Send to yt2column**）は画面と食い違う。利用者は 1 人で、画面そのものは正しいので、この間を許す。促しのメッセージの引用だけは、ガードのテスト（`test/metadataDocs.test.ts:28-35`）が文言と比べるので、文言を変えるステップ 2-1 で直す。
-   **PR-1 のマージから PR-3 のマージまで、短い促しのメッセージが古い画面に出る:** ステップ 2-1 で促しのメッセージを、概要欄の行の中に置く前提の短い文（設計書 3.5、主語の「概要欄」を省いた文）に変える。古い画面では、この文が概要欄の値の下に単独で出るので、何が折りたたまれているかがやや読み取りにくい。概要欄の値のすぐ下に出ることと、利用者が 1 人であることから、この間を許す。
-   **本文が大きすぎる状態の手動の確認には、開発用のプレビューのスクリプトを使う（レビュアーが選んだ。設計書 7.2 が計画に任せた方法）:** `scripts/popup-preview.ts` を加える（ステップ 5-3）。このスクリプトは、最悪の場合を含む入力から `buildCollectedView` と `renderPopup` で画面を描き、`style.css` と並べた静的な HTML を、引数で受け取るディレクトリに書き出す。利用者はそれを幅 360px の Chrome で開いて確かめる。`scripts/` は型の検査と lint の対象だが、`dist/` に写るのは `static/` とコンパイルした `src/` だけなので（`scripts/copy-static.ts:6-12`、`tsconfig.build.json` の `include` が `["src"]`）、スクリプトは成果物に入らない。

    4 MiB を超える選択範囲を手で作るのは難しく、右から左だけの末尾・200 コードポイントのチャンネル名・U+202E を含む値も実際の YouTube では用意しにくいので、AC-04・AC-09・AC-17・AC-33・AC-40 の確認にも同じスクリプトを使う。キーボード・VoiceOver・ブラウザの外観の設定は、実際のポップアップで確かめる。`maxBodyBytes` を手元で一時的に下げてビルドする案は、戻し忘れの危険があり、一時的とはいえ成果物に入るので採らなかった。
-   **変えない値の差分の確認を加える:** AC-20（送信中と送信の結果の文言）・AC-23（拒否のカードの文と手順）・AC-11（大きすぎるときの文と手順）は本タスクの前と同じ文言であること、AC-19 は同じリクエストの本文であること、AC-35 は同じ manifest の権限であることを求める。どれも本タスクの前後を比べる性質なので、`docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` で確かめる（ステップ 2-2）。設計書 7.4 は表示と経路の処理のテストだけを挙げる。表示のテストは文言の集合の値を表示することを確かめるだけで、値が変わっていないことは確かめない。経路の処理のテストはステップ 4-2 で書き換えるので、期待する値を変えていないことを、書き換えたテスト自身は示せない。

### 1.4. 既存コード調査結果

`a04e5d82` で確かめた。設計書 1.3・3.9・3.12 が挙げる部品は、すべて設計書の場所にある。以下は、設計書に書かれていないことと、計画の上で注意が要ることである。

**古い部品と、それを使う場所（ステップ 4-3 でなくす）:**

| 部品 | 定義 | 使う場所 |
|---|---|---|
| `summarize`・`previewLimit`・`titleLimit`・`urlLimit` | `src/core/summary.ts:4-6`・`:35-54` | `src/launch.ts:17`・`:180`、`test/summary.test.ts`（全体） |
| `OutcomeSummary`・`SummaryField` | `src/core/types.ts:21`・`:31-43` | `src/core/summary.ts:2`、`src/core/i18n/catalog.ts:4`・`:47`、`src/ui/render.ts:8`、`test/render.test.ts:8`、`test/summary.test.ts:7` |
| `renderSummary`・`renderMetadata`・`field`・`summaryStyles`・`metadataRows` | `src/ui/render.ts:24-72`・`:94-174` | `src/launch.ts:22`・`:180`・`:190`、`test/render.test.ts`、`test/renderMetadata.test.ts` |
| `summaryLabels`・`metadataLabels`・`sendButton` | `src/core/i18n/catalog.ts:47-50`、`en.ts:124-136`、`ja.ts:127-139` | `src/ui/render.ts:68`・`:149`、`src/launch.ts:225`、`test/render.test.ts`、`test/renderMetadata.test.ts`、`test/launch.test.ts:466`・`:473`・`:931`・`:1012-1023`・`:1050-1071`、`test/typecheck.test.ts:160` |
| `PopupLaunchResult` | `src/launch.ts:47-50` | `test/launch.test.ts:38`・`:147`・`:438`・`:604` |
| `valueElement`・`valueOf`（`.summary-field` で値を探す） | `test/helpers/dom.ts:27-42` | `test/render.test.ts`、`test/launch.test.ts` |
| CSS の `.summary-*`・`.metadata-*` の規則 | `static/style.css:13-55` | 古い描き方の class |

`MetadataField`（`src/core/metadata.ts:16`）は `metadata.ts` の中でも使うので残し、`catalog.ts:1` と `render.ts:6` の import だけをなくす。`emptyMetadataResult`（`src/core/metadata.ts:218`）は、ステップ 4-1 の後は `src/` から使われなくなる（今の使う場所は `src/launch.ts:182` だけ）。ステップ 4-2 の後は、テストの補助の既定値（`test/launch.test.ts:146`。ステップ 4-2 で補助を書き換えた後に残るかを確かめる）と `test/metadata.test.ts::emptyMetadataResult` だけが使う。設計書 3.12 が `metadata.ts` を「変更なし」としているので、本タスクでは残す。後続の #503（設計書 9 章）の生成中などの状態で使う見込みもないので、なくすかは本タスクの後に別に決める（10 章）。

**フェーズ 1 で先になくす部品:** `BlockReason`（`src/core/prepare.ts:64`）・`blockMessage`（`src/core/sendMessages.ts:20-37`）・`renderBlockReason`（`src/ui/renderSend.ts:38-45`）。`prepareSend` を `CollectedInput` を受け取る形に変えると、これらは型の上で要らなくなる。今の `runPopupSend` は、拒否のときに `prepareSend` より前に戻る（`src/launch.ts:206-210`）ので、`prepareSend(outcome.input, …)` と書き換えられる。大きすぎるときの表示（`src/launch.ts:212-216`）は、ステップ 4-1 までの間、`messageNodes` に文言の集合の `preflight[reason]` を渡して描く（同じ文言で、`blockMessage` は `body-too-large` に対してこれを返していた、`src/core/sendMessages.ts:30-31`）。

**なくすテストが守っていた性質の行き先:**

-   `test/sendMessages.test.ts::blockMessage`（`:63-119`）: 収集の理由を `rejectionMessage` に任せること（`:65-73`）は、`blockMessage` がなくなるので不要。`rejection` と `preflight` の文の区別（`:91-103`）は、2 つが別の場所（拒否のカードと選択範囲の行）に出るので不要。`not-watch-page` の手順に URL の例があること（`:105-111`）と、`body-too-large` が `4 MiB` を述べること（`:113-117`）は、それぞれ `test/messages.test.ts::rejectionMessage` と `test/catalog.test.ts::catalogs` に移す（ステップ 1-3）。
-   `test/sendMessages.test.ts::sendOutcomeMessage` の、設定画面などに触れない確認（`:304-322`）は、`blockMessage` を通す部分を文言の集合の `rejection`・`preflight` を直接たどる形に変え、新しい `descriptionNotSent` を加える（ステップ 1-3・2-1）。
-   `test/prepare.test.ts::prepareSend` の、拒否の理由を通す確認と拒否が大きさの判定より先である確認（`:133-155`）は、拒否を渡す呼び出しが型の検査で書けなくなるのでなくす（設計書 3.9）。
-   `test/renderSend.test.ts::renderBlockReason`（`:86-100`）は、拒否のカードと選択範囲の行の表示のテスト（`test/render.test.ts::renderPopup`、ステップ 3-2）が引き継ぐ。
-   `test/summary.test.ts::summarize` の、数え方と先頭の切り詰めの確認は、関数を直接呼ぶテストに移す（ステップ 1-1）。
-   `test/render.test.ts::renderSummary`（`:33-161`）: 英語の名前をリテラルで固定する確認（`:34-50`）は、`renderPopup` の英語の行の名前と主ボタンの名前をリテラルで比べる確認に移す。数の書式（`:77-88`）、切り詰めた値の省略記号（`:103-112`）、手順を `ol > li` で描き、手順がなければ `<ol>` を描かないこと（`:125-149`）、2 回描いても要素が増えないこと（`:151-160`）は、`test/render.test.ts::renderPopup` に移す（ステップ 3-2）。
-   `test/renderMetadata.test.ts`（全体）の、値を `textContent` で設定すること・表示の上限で切り詰めること・促しのメッセージの手順の確認は、`test/popupView.test.ts::buildCollectedView`（上限と切り詰め）と `test/render.test.ts::renderPopup`（`textContent`・手順）が引き継ぐ。概要欄の文章を表示する確認は、AC-15 で主張が逆になる。

**テストの補助:** 拡張のテストの補助は `test/helpers/` に置く（`dom.ts`・`logger.ts`・`native.ts` など）。`docs/dev/developer_guide/test_organization.md` は Go の規則（`testutil/`・`test_helpers.go`・`//go:build test`）で、拡張には当たらない。本タスクは `test/helpers/dom.ts` に骨格の要素を作る補助を加える（ステップ 3-2）。新しい補助のファイルは加えない。

**jsdom の CSS の解釈:** `style.test.ts` は jsdom の CSSOM で `style.css` を読む（`test/style.test.ts:50-81`）。設計書 3.4・3.6 の規則が jsdom で読めることを、jsdom 30.1.2（`extension/node_modules`）で確かめた。`@media (prefers-color-scheme: dark)` の中の `:root` は `CSSMediaRule` の子として、`.x:has(> :dir(rtl))`・`-webkit-line-clamp`・`[hidden]` の `!important`・`:focus-visible` の `outline` と `var(--…)` は、どれも宣言のとおりに読める。`background` の一括指定は `getPropertyValue("background")` でしか読めないので、色の変数の参照を確かめる規則は `background-color` で書く。

**Node.js の網羅率:** `node --test --experimental-test-coverage test/summary.test.ts` が `summary.ts` の行・分岐・関数の網羅率を出すことを確かめた（Node.js 24.21.0）。

**実装の方法が 2 つ以上ある箇所:** ない。設計書が型・関数・CSS の方法を決めている。プレビューのスクリプトの置き場所と、変えない文言の差分の確認は 1.3 で決めた。

**`PreparedRequest` を作れる場所:** `SelectionStatus` の `sendable` が持つ `PreparedRequest` は、`prepareSend` だけが作る（`src/core/prepare.ts:85`）。ほかの場所での型の断定は ESLint の規則が禁じる（`test/lintRules.test.ts::branded type assertions`）。`buildCollectedView` は `prepareSend` の結果を受け取って `sendable` を作るので、この保証は変わらない。`CheckState` は理由の型の各 `kind` にリテラル型で結び付く（設計書 3.2）ので、`sendable` の理由に `warning` の状態を付けるコードは、型の検査を通らない。これは型の検査だけで守られ、ガードのテストは要らない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 画面の内容

#### ステップ 1-1: 切り詰めと数え方

**変更するファイル:** `src/core/summary.ts`、`test/summary.test.ts`

-   [x] `DisplayText` に名前を付け、`truncate` の戻り値の型にする。`truncateTail` を加え、`countCharacters`・`countLines` を export する（設計書 3.1）。`summarize` と古い上限の定数は、ステップ 4-3 までそのまま残す。
-   [x] `test/summary.test.ts` に、`truncate`・`truncateTail`・`countCharacters`・`countLines` を直接呼ぶ describe を加える。後ろからの切り詰めの境界（ちょうど上限、上限を 1 超える、境界にサロゲートペアが来る場合）と、今 `summarize` を通して確かめている数え方と先頭の切り詰めの性質を、ここで確かめる（設計書 3.10）。

**完了の条件:** `make ext-check` が通る。`truncateTail` の上限の比較を 1 つずらして壊し、境界のテストが失敗することを確かめる。

#### ステップ 1-2: 確認の状態と行の名前の型

**変更するファイル:** `src/core/types.ts`

-   [x] `CheckState` と `CheckName` を加える（設計書 3.2）。`SummaryField`・`OutcomeSummary` はステップ 4-3 でなくす。

**完了の条件:** `make ext-typecheck` が通る。

#### ステップ 1-3: `prepareSend` の引数を変え、`BlockReason` をなくす

**変更するファイル:** `src/core/prepare.ts`、`src/core/sendMessages.ts`、`src/core/i18n/catalog.ts`、`src/ui/renderSend.ts`、`src/launch.ts`、`test/prepare.test.ts`、`test/sendMessages.test.ts`、`test/renderSend.test.ts`、`test/messages.test.ts`、`test/catalog.test.ts`、`test/launch.test.ts`

-   [x] `prepareSend` を `CollectedInput` を受け取る形にし、`PrepareOutcome` の `blocked` の理由を `PreflightReason` にする。`BlockReason` をなくす（設計書 3.3）。
-   [x] `blockMessage` と、`sendMessages.ts` の先頭のコメントの「a launch that sends nothing」の記述をなくす。
-   [x] `renderBlockReason` と、`renderSend.ts` の先頭のコメントの「and a block reason」をなくす。
-   [x] `catalog.ts:26` の `preflight` のコメント（`Block reasons that are not collection reasons.`）を、`BlockReason` に触れない説明に変える。
-   [x] `runPopupSend` を、`prepareSend(outcome.input, …)` を呼び、大きすぎるときは `messageNodes` で文言の集合の `preflight[reason]` を描く形にする（1.4）。ログ（`yt2column blocked`）は変えない。この形はステップ 4-1 で置き換える。
-   [x] `test/prepare.test.ts::prepareSend` の 2 つのテスト（`passes a collection reason through without its cause`、`checks the collection before the body size`）をなくし、残りのテストの引数を `CollectedInput` に変える。
-   [x] `test/sendMessages.test.ts::blockMessage` の describe をなくす。URL の例の確認を `test/messages.test.ts::rejectionMessage` に、`4 MiB` の確認を `test/catalog.test.ts::catalogs` に移す（1.4）。設定画面などに触れない確認を、文言の集合の `rejection`・`preflight` を直接たどる形に変える。
-   [x] `test/renderSend.test.ts::renderBlockReason` の describe をなくす。
-   [x] `test/launch.test.ts::popup send` の大きすぎるときのテスト（`:600-645`）の期待する文言を、`blockMessage("body-too-large", l)` から文言の集合の `preflight["body-too-large"]` に変える。

**完了の条件:** `make ext-check` が通る。消したテストについて、1.2 の比較で、消した関数（`blockMessage`・`renderBlockReason`）を除く `src/` の各関数の実行回数が 0 にならない。移した 2 つの確認について、対象の文言の URL の例と `4 MiB` をそれぞれ消して壊し、失敗することを確かめる。

#### ステップ 1-4: 画面の内容と `buildCollectedView`

**変更するファイル:** `src/core/popupView.ts`（新規）、`src/ui/render.ts`、`test/popupView.test.ts`（新規）、`test/renderMetadata.test.ts`、`test/launch.test.ts`

-   [x] `src/core/popupView.ts` に、設計書 3.2 の型（`VideoView`・`SelectionStatus`・`SelectionCheck`・`DescriptionStatus`・`DescriptionCheck`・`DetailsView`・`CollectedView`・`RejectedView`・`PopupView`）と `buildCollectedView` を書く。表示の上限は、`metadataDisplayLimit`（200）を `src/ui/render.ts:121` から移し、タブのタイトルと URL の上限（1000）をここに定義する。`summary.ts` の `titleLimit`・`urlLimit` は、ステップ 4-3 で `summarize` と一緒になくすまで残る（その間だけ、同じ値の定数が 2 か所にある）。
-   [x] `src/ui/render.ts` の `renderMetadata` が `metadataDisplayLimit` を `popupView.ts` から import する形にし、`test/renderMetadata.test.ts:9` と `test/launch.test.ts:27` の import 元を変える。
-   [x] 改行の置き換えは、切り詰めた後に画面の内容の中で行う（設計書 3.1 の順序）。
-   [x] `test/popupView.test.ts` に `buildCollectedView` のテストを書く（設計書 3.10 の行）。見出しをタブのタイトルで代えることとチャンネル名の有無（AC-02・AC-03）、選択範囲の理由と確認の状態（AC-07・AC-11）、プレビューの切り詰めと改行の置き換えの順序と境界（AC-08・AC-10）、概要欄の 3 つの理由と文字数と確認の状態（AC-12〜AC-14・AC-16）、詳細の値と上限（AC-06）。`sendable` の理由が持つリクエストが、渡した `prepareSend` の結果のリクエストと同じであることも確かめる。概要欄の文章を表示しないこと（AC-15）は、概要欄の行が文章を持つ場所を持たない型であることにし、表示の確認は描き方のテスト（ステップ 3-2）で行う。

**完了の条件:** `make ext-check` が通る。改行の置き換えを切り詰めの前に移して壊し、境界のテスト（100 コードポイントの中に改行がある場合）が失敗することを確かめる。

### フェーズ 2: 文言の集合

#### ステップ 2-1: 新しい項目と促しのメッセージ

**変更するファイル:** `src/core/i18n/catalog.ts`、`src/core/i18n/en.ts`、`src/core/i18n/ja.ts`、`README.md`、`test/catalog.test.ts`、`test/typecheck.test.ts`、`test/sendMessages.test.ts`

-   [x] `Catalog` に、設計書 3.5 の新しい項目（`checkNames`・`checkStates`・`selectionSize`・`descriptionFull`・`descriptionNotSent`・`details`・`primaryButton`）を加える。`summaryLabels`・`metadataLabels`・`sendButton` は、使う古い描き方がなくなるステップ 4-3 まで残す。
-   [x] `en.ts`・`ja.ts` に、設計書 3.5 の表の文言を書き、促しのメッセージ（`expandPrompt`）を表の短い文言に変える。数を埋め込む関数は、その言語の `formatCount` で整形し、英語は数が 1 のときに単数形にする（設計書 3.5）。
-   [x] `README.md:1136-1143` の促しのメッセージの引用を、新しい英語の文言に変える。引用の前後の文（`the popup says "…" with these steps:`、`The button stays usable: …`）は、引用に合わせて最小限だけ直す。ほかの説明はステップ 5-1 で書き直す（1.3）。
-   [x] `test/catalog.test.ts` の、文言の集合をたどる処理が関数の項目を扱えるようにする。関数の項目は、代表の数（1 と 1 より大きい数）で呼んだ結果が空でない文字列であることを確かめる。英語の単数形と複数形を確かめるテストを加える（設計書 3.10）。促しのメッセージの手順が、言語ごとに 0043 AC-11 の YouTube のボタンの表記（英語 `"...more"`、日本語「もっと見る」）を含むことを確かめるテストも加える（AC-30。日本語の表記は、`CLAUDE.md` が認める日本語の文言の性質を確かめる語の表として書く）。
-   [x] `test/typecheck.test.ts:160` の、項目が欠けたことを型の検査が見つける確認の対象を、`sendButton` から `primaryButton` に変える（設計書 3.9。`sendButton` はステップ 4-3 でなくなる）。
-   [x] `test/sendMessages.test.ts` の、設定画面などに触れない確認に、`descriptionNotSent` を加える。

**完了の条件:** `make ext-check` が通る（`test/metadataDocs.test.ts` は README の新しい引用と新しい文言を比べて通る）。`selectionSize` の英語の単数形の分岐を外して壊し、単数形のテストが失敗することを確かめる。

#### ステップ 2-2: 変えない値の差分の確認のスクリプト

**変更するファイル:** `docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts`（新規）、本書

-   [x] merge-base と PR の先頭のコミットの 2 つで、次の 3 つが同じであることを確かめるスクリプトを書く。(1) 文言の集合のうち本タスクが変えない項目（`rejection`・`preflight`・`serverReject`・`notSent`・`accepted`・`noResponse`・`hostFailed`・`unexpected`・`sending`）の、英語と日本語の両方の値（AC-11・AC-20・AC-23）。(2) スクリプトに固定した数組の入力（動画メタデータのある・ない・空の値を含む）を `buildArticleBody` に渡した結果と、`maxBodyBytes` の値（AC-19）。(3) `extension/static/manifest.json` のバイト列（AC-35）。2 つのコミットの `extension/src` を一時的なディレクトリに取り出して読み込む方法は、0043 の `docs/tasks/0043_extension_i18n/_checks/english_text.ts` に倣う（0043 の実装計画書 1.4 に、`git archive` で取り出して `node_modules` をシンボリックリンクにした記録がある）。
-   [x] リポジトリのルートで実行し、終了コードと出力を、merge-base と確かめたコミットの ID と一緒に 7 章の「差分の確認の実行の記録」に書く（PR-1）。記録のコミットは本書だけを変えるので、確かめたコミットと PR の先頭の違いは本書だけである。
-   [x] スクリプトが差分を見つけることを、(1)〜(3) のそれぞれについて一時的に 1 か所を変えて確かめ、元に戻す。その結果をコミットのメッセージに書く。
-   [x] PR-2 と PR-4 では実行しない。PR-2 は文言の集合・`prepare.ts`・manifest を変えず、PR-4 は README・用語集・スクリプトとそのテストだけを変えるためである。

**完了の条件:** PR-1 のブランチで、スクリプトが終了コード 0 で終わる。`make test`・`make ext-test` と CI では実行しない（`requirements_process.md` §4）。

### PR-1 作成ポイント: popup view model and catalog items

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2

**推奨タイトル**: `feat(0046): ポップアップの画面の内容と新しい文言を加える`

**レビュー観点**: `buildCollectedView` の状態の決め方とプレビューの切り詰めの順序（設計書 3.1・3.2） / `prepareSend` の引数の変更と、消したテストの性質の行き先（1.4） / 新しい文言の英語と日本語、用語集との一致（AC-29・AC-30） / 変えない文言の差分の確認の結果

**実装モデル要件**: standard

**判定理由**: 型と関数の形は設計書が決めており、DOM を使わない追加と機械的な削除だけで、`mkplan.md` の Conditional checks にも当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/549
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 骨格と描き方

#### ステップ 3-1: 記号の SVG

**変更するファイル:** `src/ui/icons.ts`（新規）、`test/render.test.ts`

-   [x] `checkIcon` を、`createElementNS` で SVG を作る関数として書く（設計書 3.4「記号」）。`ok` は円の中のチェック、`warning` は三角の中の感嘆符で、どちらも `aria-hidden="true"` を持つ。色は CSS の変数で付けるので、SVG に色の値を書かない。
-   [x] `test/render.test.ts` に `checkIcon` の describe を加え、2 つの状態で形の要素が異なること、`aria-hidden` を持つこと、文字（`✓`・`⚠`）を含まないことを確かめる。

**完了の条件:** `make ext-check` が通る。

#### ステップ 3-2: `renderPopup`

**変更するファイル:** `src/ui/render.ts`、`static/style.css`、`test/render.test.ts`、`test/renderMetadata.test.ts`、`test/helpers/dom.ts`

-   [x] `ActionControls`・`PopupElements`・`renderPopup` と、セクションごとの描き方を `src/ui/render.ts` に加える（設計書 3.4）。古い `renderSummary`・`renderMetadata` は、ステップ 4-3 まで残す。
-   [x] `messageNodes` の要素の class（`summary-message`・`summary-steps`）を、要約とも `<summary>` 要素とも紛らわしくない名前に変える（設計書 3.4 の最後の項目）。`rg -n "summary-message|summary-steps" extension` で見つかる、テストのセレクタと `style.css` の規則をすべて新しい名前に変える。
-   [x] `test/helpers/dom.ts` に、設計書 3.4 の骨格の要素を持つ文書を作り、`PopupElements` を返す補助を加える。この PR では骨格をこの補助の中の文字列で作り、ステップ 3-4 で出荷する `popup.html` を読む形に変える。
-   [x] `test/render.test.ts` に `renderPopup` の describe を加え（設計書 3.10 の行）、次を確かめる。
    -   構造と属性: セクションの順序と表示・非表示（AC-01・AC-22）、詳細が閉じていることと、URL とタブのタイトルが詳細の外にないこと（AC-06）、見出しが `h1` であること、確認の行の DOM の順序（記号・行の名前・状態の語・内容）と記号の `aria-hidden`、⚠ の行だけが警告の class を持つこと（AC-16・AC-27・AC-38）。
    -   表示しないものと文言: 概要欄の文章を描かないこと（AC-15）、両方の言語の文言で、もう一方の言語の文言を描かないこと（AC-29）、AC-32 の文字列が要素にならないこと、値の要素の `dir="auto"`（AC-33 の構造）。
    -   拒否のカードと主ボタン: 拒否のカードの理由ごとの文と手順と、`aria-labelledby` の指す要素が理由の文であること（AC-23・AC-38）、主ボタンの名前と、`renderPopup` が `disabled` を外さないこと（AC-17・AC-22）、2 回描いても要素が増えないこと。
    -   確認の行の内容: `selectionSize` の文言（AC-07）、先頭のプレビューの後ろと末尾のプレビューの前と、切り詰めた見出し・詳細の値の後ろの省略記号（AC-08）、`tail` がないときに末尾のプレビューの要素がないこと（AC-10）、`blocked` のときの `preflight["body-too-large"]` の文と手順（AC-11）、`descriptionFull` の文言（AC-12）、促しのメッセージの文と手順（AC-13）、`descriptionNotSent` の文と、手順の `<ol>` がないこと（AC-14）を確かめる。
    -   引き継ぎ: 1.4 の `test/render.test.ts::renderSummary` の行き先の確認（英語の名前のリテラル、数の書式、手順の `ol > li`）もここに書く。

**完了の条件:** `make ext-check` が通る。⚠ の行の警告の class を ✓ の行にも付けて壊し AC-16 のテストが、詳細に `open` を付けて壊し AC-01 のテストが、それぞれ失敗することを確かめる。

#### ステップ 3-3: 配色・レイアウト・フォーカスの CSS

**変更するファイル:** `static/style.css`、`test/style.test.ts`

-   [x] 次の規則を `static/style.css` に書く。
    -   色の変数（設計書 3.6）: ライトの `:root` と、`@media (prefers-color-scheme: dark)` の中の `:root`。
    -   `[hidden]` の規則（設計書 3.4）。
    -   幅 360px・主ボタン・`<summary>` の高さ（設計書 3.7）。`<summary>` は上下の余白に加えて `min-height: 44px` を宣言し、高さが書体の行の高さに左右されないようにする。
    -   見出しとチャンネル名の 2 行の制限、プレビューの 1 行の規則と末尾のプレビューの向きの規則、見た目では隠す要素の規則、`:focus-visible` の輪郭、確認の行・拒否のカードの規則。
-   [x] 規則は色を `var(--…)` で参照し、値を直接書かない。`body` の `min-width: 20rem` は幅の固定に置き換える。古い `.summary-*`・`.metadata-*` の規則は、ステップ 4-3 まで残す。
-   [x] 末尾のプレビューの向きを戻す `:has(> :dir(rtl))` の規則は、ほかのセレクタと 1 つの規則にまとめない（設計書 1.1）。
-   [x] 新しい規則は `line-height` を宣言しない（設計書 3.7、`test/style.test.ts:164-171`）。
-   [x] `test/style.test.ts` に、設計書 3.6「テスト」のコントラスト比の計算（ライトとダークの両方で、設計書 3.6 の 9 組が基準以上）と、その組の変数を対象の規則が `var(--…)` で参照していることの確認を加える。設計書 3.10 の `style.test.ts` の行の残りの宣言の確認（`[hidden]` の `display: none !important`、主ボタンと `<summary>` の 44px 以上、`-webkit-line-clamp: 2`、プレビューの 1 行と末尾の向き、見た目では隠す要素が `display: none`・`visibility: hidden` を使わないこと、`:focus-visible` の輪郭）も加える。ライトとダークの規則を分けて読むため、`parseStyleSheet` の戻り値に、どの `@media` の中の規則かを持たせる。

**完了の条件:** `make ext-check` が通る。`--muted` のライトの値を `#9aa0a6` にして壊し、コントラスト比のテストが失敗することを確かめる。⚠ の行の背景の規則に `var(--warn-bg)` 以外の値を書いて壊し、変数の参照のテストが失敗することを確かめる。

### PR-2 作成ポイント: popup renderer, icons and styles

**対象ステップ**: 3-1 / 3-2 / 3-3

**推奨タイトル**: `feat(0046): ポップアップの新しい描き方と配色を加える`

**レビュー観点**: `renderPopup` の要素の構造と、支援技術に渡す名前と順序（設計書 3.4、AC-27・AC-38） / 色の変数とコントラスト比のテストが、設計書 3.6 の表と実際の規則の両方を確かめていること / 古い描き方と並べて加えたことで、今のポップアップの表示が壊れていないこと（幅の固定と class の名前の変更）

**実装モデル要件**: standard

**判定理由**: 描き方と CSS は設計書と試作（設計書 6.3）が決めており、既存の部品を置き換えない追加で、`mkplan.md` の Conditional checks にも当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/552
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 3-4: 骨格

**変更するファイル:** `static/popup.html`、`test/helpers/dom.ts`、`test/popupHtml.test.ts`（新規）

ステップ 4-1・4-2 と同じ PR に入れる（1.3）。このステップだけでは、`startPopup` が新しい骨格の要素を探さないので、`test/launch.test.ts::startPopup` の出荷する `popup.html` を読むテストが失敗する。ステップ 4-1 だけでも、呼び出しの形が変わるので `test/launch.test.ts` が失敗する。そのため、ステップ 3-4・4-1・4-2 を 1 つのコミットにし、どのコミットでも `make ext-check` が通る状態を保つ。

-   [x] `popup.html` の `<body>` を、設計書 3.4 の表の骨格に置き換える（5 つの入れ物を表の順に置き、どれも `hidden`、主ボタンは `disabled`、状態の領域は `role="status"`、拒否のカードは `aria-labelledby="rejection-text"`）。`lang="en"`・`style.css`・`popup.js` の読み込みは変えない。
-   [x] `test/helpers/dom.ts` の骨格の補助を、出荷する `popup.html` を読む形に変える。
-   [x] `test/popupHtml.test.ts` に、出荷する `popup.html` の骨格が設計書 3.4 の表のとおりであること（要素・`id`・順序・`hidden`・`disabled`・`role="status"` の位置が主ボタンの後で操作のセクションの中であること・`aria-labelledby`）と、押せる要素が主ボタン（`<button>`）と詳細の `<summary>` の 2 つだけで、どの要素も `tabindex` を持たないことを確かめるテストを書く（AC-05・AC-20・AC-36・AC-39 の構造）。

**完了の条件:** ステップ 4-1・4-2 と一緒に、`make ext-check` が通る。

### フェーズ 4: 経路の処理

#### ステップ 4-1: 経路の処理を切り替える

**変更するファイル:** `src/launch.ts`、`src/popup.ts`

-   [x] `runPopupLaunch` を、DOM に触れず `PopupView` を返す形にする。`runPopupSend` を、`ActionControls` と `PopupView` を受け取り、`collected` で選択範囲の理由が `sendable` のときだけ主ボタンの `disabled` を外して押されたときの処理を付ける形にする。`startPopup` は、骨格の要素を探し、1 つでも欠けたら何も描かずに終わる形にする。主ボタンは `HTMLButtonElement`、詳細は `HTMLDetailsElement` であることを `doc.defaultView` のコンストラクタで確かめて型を絞り、違う種類の要素は欠けたものとして扱う（型の断定は使わない）。要素がそろったら `renderPopup` を呼び、その後に `runPopupSend` を呼ぶ（設計書 3.3・4.2・6.1）。`PopupLaunchResult` をなくす。
-   [x] 主ボタンを押したときの処理（1 回だけ送る、すぐに `disabled` にする、状態の領域に送信中の案内と結果を描く）と、すべてのログのラベルと内容は今と同じにする（設計書 3.3、4.3）。経路の処理はフォーカスを動かさない（設計書 3.4）。
-   [x] 大きすぎるときのログ（`yt2column blocked`）は `runPopupLaunch` で書く（設計書 3.3）。
-   [x] `src/popup.ts` の先頭のコメントと、`src/launch.ts` の関数の説明を、新しい骨格と順序に合わせる。

**完了の条件:** ステップ 4-2 と一緒に `make ext-check` が通る。

#### ステップ 4-2: 経路の処理のテスト

**変更するファイル:** `test/launch.test.ts`

設計書 3.9 の表の `test/launch.test.ts` の行と、3.10 の行に従って書き換える。

-   [x] `runPopupLaunch` の呼び出しを、container を渡さず `PopupView` を受け取る形にし、`runPopupSend` の呼び出しを、骨格の補助の `ActionControls` と `PopupView` を渡す形にする。ボタンの探し方（`.send-button`）を、骨格の主ボタン（`#primary`）に変える。
-   [x] 動画メタデータの読み取りを止めておき、その間は骨格がすべて `hidden` で主ボタンが `disabled` であること、読み取りを進めた後にまとめて表示され、見出しがタブのタイトルを経ずに動画タイトルであることを確かめるテストを加える（AC-05）。
-   [x] 送信の前後で、動画のセクション・確認のセクション・詳細の内容が変わらないことを確かめるテストを加える（AC-21）。状態の領域が描いた後に送信中の案内と結果を持つこと（AC-20・AC-39）も確かめる。
-   [x] 大きすぎるときに主ボタンが押せず送らないこと（AC-11）、折りたたまれているときと概要欄を送らないときも主ボタンが押せて送ること（AC-13・AC-14）を確かめる。今の `:553-575` は、AC-13 の確認として新しい画面に合わせて残す。
-   [x] `:647-671`（表示を切り詰めても全文を送る）は、題名と前提の説明を、概要欄の文章を表示しないこと（AC-15）に合わせて直し、本文に全文が入ることの確認として残す。
-   [x] `:673-690`（拒否のときに何も描かない）を、拒否のカードと押せない主ボタンを出し、送らないことの確認に変える（AC-22）。
-   [x] `display` の describe（`:928-958`）と `startPopup` の describe（`:960-1122`）の、`#summary`・`#send`・`summaryLabels`・`sendButton` を使う確認を、新しい骨格と文言に変える。骨格の要素が 1 つでも欠けたら何も描かないこと（`:1093-1122` の置き換え）は、5 つの入れ物と主ボタンと状態の領域のそれぞれを欠かした文書で確かめる（設計書 4.2）。
-   [x] 動画タイトルが取れないとき（ページにない・読み取りの失敗）も、主ボタンが押せて送ることを確かめる（AC-03）。
-   [x] 拒否のときに動画メタデータを読まないこと（`:327` の describe）は、返った画面の内容が `rejected` で、動画メタデータの読み取りの呼び出しが 0 回であることを確かめる形に変える（`:341` の `emptyMetadataResult` との比較は、画面の内容が動画メタデータを持たないのでなくす）（AC-24）。本文が今と同じであること（`:252`・`:647`）と、ログが今と同じであること（`logs by locale`・`video metadata log`・`send log`）は、呼び出しの形だけを変え、期待する値は変えない（AC-19。期待する値を変えていないことはステップ 4-4 の差分の確認と PR-3 のレビューで確かめる）。

**完了の条件:** ステップ 3-4・4-1 と一緒に `make ext-check` が通る。`runPopupSend` が `blocked` のときにも `disabled` を外すように壊し、AC-11 のテストが失敗することを確かめる。`startPopup` が `runPopupLaunch` を待つ前に `#video` の `hidden` か `#primary` の `disabled` を外すように壊し、AC-05 のテストが失敗することを確かめる。

#### ステップ 4-3: 古い部品をなくす

**変更するファイル:** `src/core/summary.ts`、`src/core/types.ts`、`src/core/i18n/catalog.ts`、`src/core/i18n/en.ts`、`src/core/i18n/ja.ts`、`src/ui/render.ts`、`static/style.css`、`test/summary.test.ts`、`test/render.test.ts`、`test/renderMetadata.test.ts`、`test/catalog.test.ts`、`test/helpers/dom.ts`

-   [x] `summarize` と、`summary.ts` の `titleLimit`・`urlLimit`・`previewLimit` をなくす。
-   [x] `types.ts` の `SummaryField`・`OutcomeSummary` をなくす。
-   [x] `render.ts` の `renderSummary`・`renderMetadata`・`field`・`summaryStyles`・`collectedNodes`・`rejectedNodes`・`metadataRows` と、それだけが使う型と import（`MetadataField`・`MetadataResult`・`OutcomeSummary`・`SummaryField`）をなくす。
-   [x] `Catalog` の `summaryLabels`・`metadataLabels`・`sendButton` と、`en.ts`・`ja.ts` のその値、`catalog.ts` の `MetadataField`・`SummaryField` の import をなくす。
-   [x] `style.css` の古い `.summary-*`・`.metadata-*` の規則をなくす。
-   [x] `test/catalog.test.ts:14` のコメントの例（`summaryLabels.url`）を、残る項目の例に変える。PR-1 のステップ 2-1 で、例はすでに `details.summary` に変わっていた。
-   [x] `test/summary.test.ts::summarize` の describe、`test/render.test.ts::renderSummary` の describe、`test/renderMetadata.test.ts`（ファイル全体）、`test/helpers/dom.ts` の `valueElement`・`valueOf` をなくす。
-   [x] 1.4 の表の部品の名前を `rg` で探し、`src/`・`test/`・`static/` に残りがないことを確かめる（8 章の横断の確認）。

**完了の条件:** `make ext-check` が通る。消したテストについて、1.2 の比較で、消した関数を除く `src/` の各関数の実行回数が 0 にならない。

#### ステップ 4-4: 差分の確認の実行（PR-3）

**変更するファイル:** 本書

-   [ ] PR-3 のブランチで `docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` を実行し、終了コードと出力を、merge-base と確かめたコミットの ID と一緒に 7 章に記録する（記録のコミットは本書だけを変える）。

**完了の条件:** スクリプトが終了コード 0 で終わる。

### PR-3 作成ポイント: popup skeleton and launch path switch-over

**対象ステップ**: 3-4 / 4-1 / 4-2 / 4-3 / 4-4

**推奨タイトル**: `feat(0046): ポップアップを新しい骨格と描く順序に切り替える`

**レビュー観点**: 主ボタンを押せるのが `collected` で `sendable` のときだけであることと、それ以外のときに押せないまま残ること（設計書 3.3・4.2） / 読み取りと判定が終わるまで何も表示しないことのテスト（AC-05） / 古い部品の削除の漏れと、消したテストの性質の行き先（1.4・8 章） / ログとリクエストの本文が変わらないこと（AC-19、設計書 4.3）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-1 が、送信の状態の遷移（設計書 6.2）と、主ボタンを押せるようにする fail secure の判断を、新しい非同期の順序の後ろへ移す。`mkplan2.md` の「isolated high-risk/complex step（state machines）」に当たる。ステップ 4-1 は骨格（3-4）と呼び出しの形を変えるテスト（4-2）と同じコミットでしか通らないので、PR の中で切り離せない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 文書と手動の確認

#### ステップ 5-1: README

**変更するファイル:** `README.md`、`test/metadataDocs.test.ts`

-   [ ] 「Browser extension」の節のポップアップの説明（`README.md:1129-1152`）を、設計書 3.8 のとおりに書き直す。主ボタンを **Generate article** と書き、動画のセクション・確認のセクション（選択範囲の行と概要欄の行、⚠ のときの理由と手順）・主ボタン・詳細を説明し、値が取れないときの説明に、概要欄の行が ⚠ になることと見出しがタブのタイトルになることを加える。
-   [ ] `test/metadataDocs.test.ts` に、README の節が `catalogs.en.primaryButton` を含むことの確認を加える（設計書 3.8）。促しのメッセージの比較は残す。README が画面の文言をほかにも引用するとき（行の名前、`Details` など）は、その文言ごとに同じ確認を加える。

**完了の条件:** `make ext-test` が通る。README の主ボタンの名前を `Send to yt2column` に戻して壊し、追加した確認が失敗することを確かめる。

#### ステップ 5-2: 用語集

**変更するファイル:** `docs/translation_glossary.md`

-   [ ] 要件書 6 章の用語を加え、「字幕」の行に UI の文言での使い方を注記する（設計書 3.8）。

**完了の条件:** PR のレビューで、加えた用語と要件書 6 章を照らす。

#### ステップ 5-3: プレビューのスクリプト

**変更するファイル:** `scripts/popup-preview.ts`（新規）、`test/popupPreview.test.ts`（新規）

-   [ ] `scripts/popup-preview.ts` を書く（1.3）。手動の確認（ステップ 5-4）に使う状態を、名前を付けた入力の組として持ち、それぞれを `buildCollectedView`（拒否は `RejectedView`）と、出荷する `popup.html` の骨格への `renderPopup` で描き、描く前に `applyPageLocale` でその入力の組の言語をページに設定し（日本語の行間 1.7 は `body:lang(ja)` で決まる、`static/style.css:71-73`）、静的な HTML のファイルとして引数のディレクトリに書き出す。`style.css` も同じディレクトリに写す。入力の組には、少なくとも次を含める。
    -   AC-17 の最悪の場合: 本文が 4 MiB を超え、概要欄が折りたたまれ、見出しが 2 行を超え、チャンネル名が 200 コードポイントの日本語。
    -   AC-09: 末尾が左から右だけ・右から左だけ・向きが混ざった文・U+200F を含むもの。
    -   AC-33: U+202E を含む値。
    -   拒否。
    -   英語と日本語の両方。
-   [ ] `eslint.config.js:238-249` が `scripts/` でも `CollectedInput`・`PreparedRequest` への型の断定を禁じるので、入力は本番の関数で作る。`CollectedInput` は受け付ける URL と偽の `SelectionReader` を渡した `collect` で、動画メタデータは `acceptMetadata`（読み取りの失敗は `readFailureResult`）で、リクエストは `prepareSend` で作る。本文が大きすぎる状態は、4 MiB を超える選択範囲を実際に渡して作る。HTML は `JSDOM` の `serialize()` で書き出す（`outerHTML` は lint が禁じる）。
-   [ ] 書き出す HTML は、外部の資源を読み込まず、スクリプトを含まない。骨格の `<script src="popup.js">` は取り除き、`style.css` への相対の `<link>` は残す。
-   [ ] `test/popupPreview.test.ts` に、スクリプトを一時的なディレクトリに対して実行し、終了コードが 0 で、入力の組ごとの HTML のファイルと `style.css` ができ、各ファイルが主ボタンを持ち `<script>` を持たず、日本語の組のファイルの `<html>` が `lang="ja"` であることを確かめるテストを書く。

**完了の条件:** `make ext-check` が通る。`make ext-build` の後の `dist/` に `popup-preview` を名前に含むファイルがない（`make ext-build` が呼ぶ `check-dist` が通ることと合わせて、ステップ 5-4 の前に確かめる）。

#### ステップ 5-4: 手動の確認

**変更するファイル:** 本書

-   [ ] 設計書 7.2 の表のすべての行を、Chrome と Brave、ライトとダーク、英語と日本語の組み合わせで確かめる。倍率は 100% とする。AC-04・AC-09・AC-17・AC-33・AC-40 は、ステップ 5-3 のスクリプトの出力を幅 360px のウィンドウで開いて確かめる。AC-17 は、通常のウィンドウにはポップアップの高さの上限がないので、開発者ツールで主ボタンの `getBoundingClientRect().bottom` を測り、英語と日本語の最悪の場合の組で 600 CSS ピクセル以下であることを合格の条件とする。そのうえで、実際のポップアップでも見出しが長い動画で AC-04 を確かめる。AC-25・AC-28・AC-36〜AC-39 は、ビルドした拡張を読み込んだ実際のポップアップで確かめる。
-   [ ] 結果を、確かめた日付・ブラウザのバージョン・確かめた人と一緒に 7 章の「手動の確認の記録」に書く。

**完了の条件:** 設計書 7.2 の表のすべての行に結果が記録されている。

### PR-4 作成ポイント: README, glossary, preview script and manual checks

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4

**推奨タイトル**: `chore(0046): README と用語集を新しいポップアップに合わせ、プレビューのスクリプトと手動の確認を加える`

**レビュー観点**: README の説明と英語の文言の一致（AC-31） / 用語集に加えた用語と要件書 6 章の一致（AC-29） / プレビューのスクリプトが成果物に入らないことと、最悪の場合の入力の組が設計書 7.2 の確認を網羅すること / 手動の確認の記録

**実装モデル要件**: standard

**判定理由**: 文書の更新と、既存の関数を呼ぶだけの開発用のスクリプトで、`mkplan.md` の Conditional checks にも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

フェーズは設計書 8 章の順序に従う。フェーズ 2 はフェーズ 1 のステップ 1-2 の型（`CheckName`・`CheckState`）を使い、フェーズ 3 は 1 と 2 の型と文言を使い、フェーズ 4 は 3 を使う。

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1（PR-1） | 画面の内容・`buildCollectedView`・新しい文言・変えない文言の差分の確認 | PR-1 のマージ。ポップアップは古い表示のまま動く |
| M2（PR-2） | `renderPopup`・`checkIcon`・色の変数とコントラストのテスト | PR-2 のマージ。ポップアップは古い表示のまま動く |
| M3（PR-3） | 骨格・経路の処理の切り替え・古い部品の削除 | PR-3 のマージ。ポップアップは新しい表示で動く |
| M4（PR-4） | README・用語集・プレビューのスクリプト・手動の確認の記録 | PR-4 のマージ。7 章のすべての行が確かめられている |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 | 画面の内容と `buildCollectedView`、`prepareSend` の引数の変更、文言の新しい項目と短い促しのメッセージ、差分の確認のスクリプト | standard |
| PR-2 | 3-1 / 3-2 / 3-3 | `checkIcon`・`renderPopup`・色の変数とレイアウトの CSS を、古い表示と並べて加える | standard |
| PR-3 | 3-4 / 4-1 / 4-2 / 4-3 / 4-4 | 骨格への置き換え、経路の処理の切り替え、古い部品の削除 | frontier-recommended |
| PR-4 | 5-1 / 5-2 / 5-3 / 5-4 | README・用語集・プレビューのスクリプト・手動の確認 | standard |

## 4. テスト戦略 (Test Strategy)

### 4.1. 加える・変えるテスト

設計書 3.10 の表のとおりである。ステップとの対応は次のとおり。

| テスト | ステップ |
|---|---|
| `test/summary.test.ts` | 1-1（追加）・4-3（`summarize` の describe を削除） |
| `test/popupView.test.ts`（新規） | 1-4 |
| `test/prepare.test.ts`・`test/renderSend.test.ts`・`test/messages.test.ts` | 1-3 |
| `test/sendMessages.test.ts` | 1-3・2-1 |
| `test/catalog.test.ts` | 1-3・2-1・4-3（コメント） |
| `test/typecheck.test.ts` | 2-1 |
| `test/render.test.ts` | 3-1・3-2（追加）・4-3（`renderSummary` の describe を削除） |
| `test/style.test.ts` | 3-3 |
| `test/popupHtml.test.ts`（新規） | 3-4 |
| `test/launch.test.ts` | 1-3（期待する文言）・1-4（import 元）・4-2（書き換え） |
| `test/renderMetadata.test.ts` | 1-4（import 元）・3-2（class の名前）・4-3（削除） |
| `test/metadataDocs.test.ts` | 5-1 |
| `test/popupPreview.test.ts`（新規） | 5-3 |

テストは英語と日本語の両方で実行する（0043 の方針、設計書 7.1）。

### 4.2. 差分の確認

AC-11・AC-20・AC-23 の文言、AC-19 のリクエストの本文、AC-35 の manifest が本タスクの前と同じであることは、`docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` で確かめる（ステップ 2-2・4-4）。文言の集合を変える PR-1 と PR-3 のブランチでそれぞれ実行し（PR-2・PR-4 で実行しない理由はステップ 2-2）、結果を 7 章に記録する。`make test`・`make ext-test` と CI では実行せず、マージの後は保守しない（`requirements_process.md` §4）。

### 4.3. 統合テスト（手動）

設計書 7.2 のとおりである（ステップ 5-4）。jsdom はレイアウトを計算しないので、見た目（2 行の省略、プレビューの見え方、主ボタンが見えること、色、大きさ、フォーカスの表示）と支援技術の読み上げは、CSS の宣言のテストと手動の確認で確かめる。

### 4.4. 既存のテスト

次のテストは変えずに通す。`test/lintRules.test.ts`（HTML を解釈する API の禁止、型の断定の禁止。AC-32・AC-34）、`test/manifest.test.ts`（権限と CSP。AC-34・AC-35）、`test/checkDist.test.ts`（成果物に外部の資源がないこと。AC-34）、`test/metadata.test.ts`、`test/collect.test.ts`、`test/send.test.ts`、`test/secretsAbsent.test.ts`。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| PR-1・PR-2 の間に古い表示を壊す | マージの後のポップアップが崩れる | 古い部品を残し、`make ext-check` の既存のテスト（`render.test.ts`・`renderMetadata.test.ts`・`launch.test.ts`）を通す。PR-2 の幅の固定と class の名前の変更は、レビュー観点に挙げる |
| 骨格だけを先に変えて、ポップアップが何も表示しなくなる | 利用者が送れない | 骨格と経路の処理の切り替えを同じコミット（ステップ 3-4・4-1）と同じ PR（PR-3）に入れる（1.3） |
| コントラストのテストの組の表が、実際の規則とずれる | テストが通っても、規則が別の色を使う | 対象の規則が組の変数を参照していることを同じテストで確かめる（ステップ 3-3、設計書 3.6） |
| jsdom が読めない CSS の書き方をする | 宣言のテストが規則を見つけない | jsdom で読めることを確かめた書き方（1.4）を使う。`background` の一括指定ではなく `background-color` で書く |
| 消したテストが守っていた性質が残らない | 退行を見逃す | 1.4 に行き先を書き、1.2 の比較をステップ 1-3・4-3 の完了の条件にする |
| 実機の見た目が試作と違う | AC-17 などを満たさない | ステップ 5-4 の手動の確認で見つけ、直す。直し方が設計書と食い違うときは、設計書を先に直す（`requirements_process.md`「Editing an approved document」） |
| PR-3 と PR-4 の間、README が画面と食い違う | 利用者が説明と画面の違いに戸惑う | 利用者は 1 人で、画面は正しい。PR-4 をすぐ続ける（1.3） |

日程のリスクは小さい。PR-3 がいちばん大きいので、PR-3 のレビューで直しが多く出たときは、ステップ 4-3（古い部品の削除）を別の PR に分けることを、そのときにレビュアーに相談する。

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2）
-   [x] PR-2 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3）
-   [ ] PR-3 マージ済み（対象ステップ: 3-4 / 4-1 / 4-2 / 4-3 / 4-4）
-   [ ] PR-4 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` はテスト、`static` は `make` のターゲット、`diff` は差分の確認のスクリプト、`manual` は PR のレビューでの読み取りか手動の確認を指す。

| AC | 確かめ方 | ステップ | 内容 |
|---|---|---|---|
| AC-01 | test: `extension/test/render.test.ts::renderPopup` | 3-2 | セクションの順序と、詳細が閉じていること |
| AC-02 | test: `extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup` | 1-4・3-2 | 動画タイトルを見出しにし、チャンネル名を描くこと |
| AC-03 | test: `extension/test/popupView.test.ts::buildCollectedView`・`extension/test/launch.test.ts::popup send` | 1-4・4-2 | 見出しがタブのタイトルになること、チャンネル名の行がないこと、送信に進めること |
| AC-04 | test: `extension/test/style.test.ts`（`-webkit-line-clamp: 2` の宣言）＋ manual | 3-3・5-4 | 宣言はテスト。2 行で省略記号が付くことは手動 |
| AC-05 | test: `extension/test/launch.test.ts::startPopup`・`extension/test/popupHtml.test.ts`・`extension/test/style.test.ts`（`[hidden]`） | 3-3・3-4・4-2 | 読み取りの間は何も表示せず主ボタンが押せないこと、骨格が `hidden` で始まること、`hidden` が上書きされないこと |
| AC-06 | test: `extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup` | 1-4・3-2 | 詳細の値と、詳細の外にないこと |
| AC-07 | test: `extension/test/summary.test.ts`（数え方）・`extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup`・`extension/test/catalog.test.ts::catalogs` | 1-1・1-4・2-1・3-2 | 数え方、`sendable` が ✓ であること、数の書式 |
| AC-08 | test: `extension/test/summary.test.ts`（`truncate`・`truncateTail`）・`extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup` | 1-1・1-4・3-2 | 100 コードポイントの切り詰め、改行の置き換えの順序、省略記号の位置、サロゲートペア |
| AC-09 | test: `extension/test/style.test.ts`（プレビューの 1 行と末尾の向きの宣言）＋ manual | 3-3・5-4 | 宣言はテスト。見え方は手動（向きが混ざった文は保証の外、設計書 1.1） |
| AC-10 | test: `extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup` | 1-4・3-2 | 100 コードポイント以下で末尾のプレビューがないこと |
| AC-11 | test: `extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup`・`extension/test/launch.test.ts::popup send`・`extension/test/prepare.test.ts::prepareSend` ＋ diff: `docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` | 1-3・1-4・2-2・3-2・4-2・4-4 | ⚠ と理由と手順、主ボタンが押せず送らないこと、大きさの判定。文言が前と同じことは diff（実行の記録は下） |
| AC-12 | test: `extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup` | 1-4・3-2 | ✓ と全文と文字数 |
| AC-13 | test: `extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup`・`extension/test/launch.test.ts::popup send` | 1-4・3-2・4-2 | ⚠ と手順、文字数がないこと、主ボタンが押せて冒頭を送ること |
| AC-14 | test: `extension/test/popupView.test.ts::buildCollectedView`・`extension/test/render.test.ts::renderPopup`・`extension/test/launch.test.ts::popup send` | 1-4・3-2・4-2 | ⚠ と手順がないこと、主ボタンが押せること |
| AC-15 | test: `extension/test/render.test.ts::renderPopup` | 3-2 | 概要欄の文章を表示しないこと |
| AC-16 | test: `extension/test/render.test.ts::renderPopup`・`extension/test/style.test.ts` | 3-2・3-3 | ⚠ の行だけが警告の class を持ち、その規則だけが `--warn-bg` を参照すること |
| AC-17 | test: `extension/test/render.test.ts::renderPopup`（名前）・`extension/test/style.test.ts`（幅と 2 行の宣言）＋ manual | 3-2・3-3・5-3・5-4 | 名前と宣言はテスト。主ボタンが見えることは、プレビューのスクリプトの最悪の場合で手動 |
| AC-18 | test: `extension/test/launch.test.ts::popup send` | 4-2 | 1 回だけ送り、送信中と送信の後は押せないこと |
| AC-19 | test: `extension/test/launch.test.ts::launch paths`・`::popup send`・`extension/test/prepare.test.ts::buildArticleBody`・`::prepareSend` ＋ diff: `docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` | 1-3・2-2・4-2・4-4 | 送る本文と大きさの判定はテスト。同じ入力から前と同じ本文ができることは diff |
| AC-20 | test: `extension/test/launch.test.ts::popup send`・`extension/test/popupHtml.test.ts` ＋ diff: `docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` | 2-2・3-4・4-2・4-4 | 状態の領域が操作のセクションの中で主ボタンの後にあり、送信中の案内と結果を持つこと。文言が前と同じことは diff |
| AC-21 | test: `extension/test/launch.test.ts::popup send` | 4-2 | 送信の前後で 3 つの入れ物の内容が変わらないこと |
| AC-22 | test: `extension/test/render.test.ts::renderPopup`・`extension/test/launch.test.ts::popup send` | 3-2・4-2 | 拒否のカードと押せない主ボタンだけを表示し、送らないこと |
| AC-23 | test: `extension/test/render.test.ts::renderPopup`・`extension/test/messages.test.ts::rejectionMessage` ＋ diff: `docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` | 1-3・2-2・3-2・4-4 | 理由ごとの文と手順、互いに異なること。文言が前と同じことは diff |
| AC-24 | test: `extension/test/launch.test.ts::launch paths` | 4-2 | 拒否のときに動画メタデータを読まないこと（今のテストを保つ） |
| AC-25 | test: `extension/test/style.test.ts`（ライトとダークの変数の 2 組）＋ manual | 3-3・5-4 | 変数の 2 組はテスト。配色の切り替えは手動 |
| AC-26 | test: `extension/test/style.test.ts`（コントラスト比と変数の参照） | 3-3 | 9 組が両方のテーマで基準以上 |
| AC-27 | test: `extension/test/render.test.ts::renderPopup`・`extension/test/style.test.ts`（見た目では隠す規則） | 3-2・3-3 | 状態の語が行の名前の直後にあり、支援技術から隠れないこと |
| AC-28 | test: `extension/test/style.test.ts`（44px の宣言）＋ manual | 3-3・5-4 | 宣言はテスト。大きさは手動 |
| AC-29 | static: `make ext-typecheck` ＋ test: `extension/test/catalog.test.ts::catalogs`・`extension/test/typecheck.test.ts::a Catalog missing an item fails typecheck`・`extension/test/render.test.ts::renderPopup` ＋ manual | 2-1・3-2・5-2 | 欠けで型の検査が失敗すること、空でないこと、両方の言語で描くこと。日本語の用語と用語集の一致は PR-1・PR-4 のレビューで読む |
| AC-30 | test: `extension/test/catalog.test.ts::catalogs` | 2-1 | 促しのメッセージの手順が、言語ごとの 0043 AC-11 の表記を含むこと |
| AC-31 | test: `extension/test/metadataDocs.test.ts::video metadata documentation` ＋ manual | 2-1・5-1 | 主ボタンの名前と促しのメッセージの引用はテスト。説明の文は PR-4 のレビューで読む |
| AC-32 | test: `extension/test/render.test.ts::renderPopup`・`extension/test/lintRules.test.ts::html parsing APIs` | 3-2 | 6 か所の値で要素にならないこと |
| AC-33 | test: `extension/test/render.test.ts::renderPopup`（`dir="auto"` の構造）・`extension/test/style.test.ts`（`unicode-bidi` の宣言）＋ manual | 3-2・3-3・5-4 | 構造と宣言はテスト。表示順は手動 |
| AC-34 | test: `extension/test/lintRules.test.ts::code from strings`・`::html parsing APIs`・`extension/test/manifest.test.ts`・`extension/test/checkDist.test.ts::checkDist` ＋ static: `make ext-build`（`check-dist`） | 既存のテスト（4.4） | 外部の資源・文字列からのコード・CSP（今のテストを保つ） |
| AC-35 | test: `extension/test/manifest.test.ts::permissions`・`::undeclared keys` ＋ diff: `docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` | 2-2・4-4 | 権限の値はテスト。manifest が前と同じであることは diff |
| AC-36 | test: `extension/test/popupHtml.test.ts`・`extension/test/render.test.ts::renderPopup` ＋ manual | 3-2・3-4・5-4 | 押せる要素が `<button>` と `<summary>` で、`tabindex` を持たず、DOM の順序が表示の順序であることはテスト。キーボードでの操作は手動 |
| AC-37 | test: `extension/test/style.test.ts`（`:focus-visible` の輪郭とコントラスト比）＋ manual | 3-3・5-4 | 宣言と比はテスト。見え方は手動 |
| AC-38 | test: `extension/test/render.test.ts::renderPopup`・`extension/test/popupHtml.test.ts` ＋ manual | 3-2・3-4・5-4 | 要素の役割・名前・DOM の順序・`aria-labelledby`・`aria-hidden` はテスト。読み上げは VoiceOver で手動 |
| AC-39 | test: `extension/test/popupHtml.test.ts`・`extension/test/launch.test.ts::popup send` ＋ manual | 3-4・4-2・5-4 | `role="status"` が骨格にあり、送信中の案内と結果がそこに描かれること。読み上げは手動 |
| AC-40 | test: `extension/test/render.test.ts::checkIcon`・`extension/test/style.test.ts`（`--ok`・`--warn`・`--warn-bg` の変数と参照） ＋ manual | 3-1・3-3・5-3・5-4 | 2 つの記号の形が異なることと、色が変数で決まることはテスト。色覚の模擬で区別がつくことは手動 |

AC-36 のキーボードでの操作と AC-40 の色覚の模擬は、テストの後に手動でも確かめる。jsdom はフォーカスの移動とレイアウトを計算せず、色覚の模擬はブラウザの開発者ツールの機能だからである（設計書 3.10・7.2）。

**差分の確認の実行の記録:**

- PR-1（2026-10-28）: リポジトリのルートで `node docs/tasks/0046_popup_redesign/_checks/unchanged_values.ts` を実行し、終了コード 0。merge-base は `c0b4e3fe86335b0dcd89eaca3896911c1590b8c2`、確かめたコミットは `ecb30952521eb5b1a764fca0ed058a5be36ffc5e`（記録のコミットは本書だけを変える。ステップ 2-2）。出力は次の 4 行である。

    ```
    merge-base c0b4e3fe86335b0dcd89eaca3896911c1590b8c2
    head       ecb30952521eb5b1a764fca0ed058a5be36ffc5e
    PASS: the unchanged catalog items and the built bodies are identical
    PASS: extension/static/manifest.json is identical (sha256 47091ca6796f153ed03ffbbb55573067a88da30bbae1ea20186407b2e2bed8d5)
    ```

**手動の確認の記録:** （ステップ 5-4 で記録する）

## 8. 横断の確認 (Cross-Search Checklist)

`make lint`・`make ext-check` では見つからない残りを、ステップ 4-3 で確かめる。

-   [ ] 1.4 の表の部品の名前（`summarize`・`OutcomeSummary`・`SummaryField`・`renderSummary`・`renderMetadata`・`summaryLabels`・`metadataLabels`・`sendButton`・`PopupLaunchResult`・`valueOf`・`valueElement`）と、フェーズ 1 でなくす名前（`BlockReason`・`blockMessage`・`renderBlockReason`）が、`extension/src`・`extension/test`・`extension/static` のコードとコメントに残っていない。
-   [ ] 古い class の名前（`summary-field`・`summary-label`・`summary-value`・`summary-title`・`summary-url`・`summary-preview`・`summary-message`・`summary-steps`・`metadata-title`・`metadata-channel`・`metadata-description`・`send-button`・`send-message`）と、古い入れ物の `id`（`summary`・`send`）が、`extension/src`・`extension/test`・`extension/static` に残っていない。
-   [ ] `src/popup.ts`・`src/launch.ts`・`src/ui/render.ts`・`src/ui/renderSend.ts`・`src/core/sendMessages.ts` の先頭のコメントと関数の説明が、新しい骨格と経路の処理を述べている（PR-3 のレビューで読む）。
-   [ ] 日本語の文言（`ja.ts` の新しい項目）が、用語集の用語（ステップ 5-2 で加える用語を含む）と一致する（PR-1・PR-4 のレビューで読む）。

## 9. 成功基準 (Success Criteria)

-   **機能の完全性:** 7 章のすべての AC の確かめ方が実行され、`test`・`static`・`diff` が合格し、`manual` の行が PR のレビューと手動の確認で確かめられている。
-   **品質:** 各 PR で `make ext-check`・`make test`・`make lint` が通る。新しいテストと、確認の内容を変えたテストは、実装を壊して失敗することが確かめられ、コミットのメッセージに書かれている。消したテストについて、1.2 の比較の結果がコミットのメッセージに書かれている。
-   **セキュリティ:** 値は `textContent` だけで表示される（AC-32）。外部の資源を読み込まず、manifest の権限と CSP は変わらない（AC-34・AC-35）。プレビューのスクリプトは `dist/` に入らない。
-   **文書:** README の説明が新しい画面と一致し（AC-31）、用語集に要件書 6 章の用語がある。

## 10. 次のステップ

-   本書の承認の後、`/runplan 0046` で PR-1（フェーズ 1・2）を実装して開く。PR-2〜PR-4 も、前の PR のマージの後に `/runplan 0046` で同じように進める。
-   PR-4 のマージの後、issue #507 を閉じる（`/runpr` が最後の PR の後に案内する）。後続の #503 は、設計書 9 章の構造の上に作る。
-   本番のコードから使われなくなる `emptyMetadataResult` をなくすかを、別に決める（1.4）。
