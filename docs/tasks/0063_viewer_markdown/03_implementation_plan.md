# 実装計画書：ビューアで記事の Markdown を描画する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-06 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は、[01_requirements.md](01_requirements.md)（以下「要件」）と [02_architecture.md](02_architecture.md)（以下「設計書」）に従って、実装の手順と確かめ方を定める。F-…・AC-…・H-… は要件の項目を、B-1〜B-7・I-1〜I-5 は要件 3.2 の規則を指す。既存のコードの行番号は、コミット `22b24bad` で確かめた。設計書が行番号を確かめたコミット `b9e33644` から `22b24bad` までに、`extension/` の変更はない（`git diff --stat b9e33644 22b24bad -- extension` が空）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、次を行う。

-   表示用の Markdown を記事の木にする解釈の関数 `parseArticleMarkdown` と、リンクにしてよい URL のブランド型 `LinkUrl` を作る（設計書 3.1・3.3・3.6）。
-   描き直しの判定を共通の関数 `drawOnKeyChange` に取り出し、状態の表示と記事の本文の両方で使う（設計書 3.4）。
-   記事の木から DOM を作る組み立ての関数 `buildArticleBody` と、ビューアが呼ぶ描画の関数 `drawArticleBody` を作り、`renderViewer` の本文の表示をこれに切り替える（設計書 3.2・3.4）。
-   記事の文字組みの CSS を加える（設計書 3.5）。
-   方針の変更に合わせて、security.md・CLAUDE.md・README を改める（設計書 3.7）。

サーバ・ホスト・応答の形と、Go のコードは変えない（要件 4.4）。

### 1.2. 実装の方針

-   **設計は設計書に従い、本書は繰り返さない。** 型と関数の形は設計書 3.1〜3.4、CSS の値は 3.5、時間の要求は 3.6、手動の確認の項目は 3.8、テストの構成は 7 章が定める。本書のステップは、どの節に従って何を作り、どう確かめるかだけを示す。
-   **テストが失敗しうることを、壊して確かめる。** 新しいテストと、確認の内容を変えたテストのそれぞれについて、確かめる経路を壊し、そのテストが失敗することを実装のときに確かめ、結果をコミットのメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。各ステップの「完了の条件」に、壊す対象を挙げる。
-   **規則の境目は、片方の規則だけが当たる入力で確かめる。** インラインの規則の順序（I-1・I-2・I-5・I-3・I-4）の確認では、順序を入れ替えたときに結果が変わる入力を使う（`CLAUDE.md`「A layered path needs inputs only one layer can handle」）。
-   **テストを消すときは、そのコミットを分けて比べる。** `it` を消すコミットは、消すことだけを含め、`make ext-coverage-compare BASE=<親のコミット>` の出力を最後の行までコミットのメッセージと PR の説明に書く（`CLAUDE.md`「Deleting a test is a claim that must be checked」）。対象は、ステップ 4-4 と、`it` を消すことになった場合のほかのステップである。
-   **古い表示は、切り替えのステップまで残す。** フェーズ 2〜4 の前半は、新しい部品を `renderArticleBody` と並べて加える。`renderViewer` を切り替えるのはステップ 4-3、`renderArticleBody` を消すのはステップ 4-4 である。
-   **コードのコメントは、そのコードを変えるステップで直す。** `articleBody.ts` の先頭のコメントはステップ 4-1・4-4 で、`renderViewer` のコメントはステップ 4-3 で直す。コードを変えない `response.ts` のコメントだけを、文書と同じフェーズ 6 で直す（設計書 3.7）。
-   **テストの名前は仮のもの。** 7 章のテストの場所（`ファイル::describe の名前`）は仮の名前で、実装で決めた名前に合わせて本書を直す（編集上の訂正）。

### 1.3. 計画の際に決めたこと（2026-11-06）

-   **PR は 4 つに分ける: フェーズ 1 / フェーズ 2 / フェーズ 3・4 / フェーズ 5・6。**
    -   理由: 表示を切り替えるフェーズ 4 は、DOM の組み立て、要素と属性の一覧、ランダムな入力のテストという安全性の要を含む。CSS と文書の差分と分けて、これを集中してレビューできるようにする。
    -   受け入れること: フェーズ 3・4 の PR をマージしてから、フェーズ 5・6 の PR をマージするまでの間、main のビューアは Markdown を描画するが、CSS は今のままで（見出しとリストはブラウザの既定のスタイル）、security.md §8.1 は「`textContent` だけで表示する」のままである。利用者はひとりで、issue はタスクの最後の PR まで開いたままなので、受け入れる。
    -   採らなかった案: フェーズ 1 / フェーズ 2・3 / フェーズ 4・5・6 の 3 つ。main が描画と食い違う期間はなくなるが、最後の PR が大きくなり、安全性の要が CSS と文書の差分に紛れる。
    -   ADR の判断: 作らない。このタスクの PR の分け方で、タスクの中で閉じる。
-   **フェーズ 1 では、描画の時間を、最悪の場合と同じ数の要素を持つ合成の DOM で測り、実装の後に実物のコードで測り直す。**
    -   理由: フェーズ 1 の時点では解釈の関数がない。組み立てと配置の時間を先に測っておけば、設計書 3.8 の代わりの方法（文字のノードをまとめる）が要るかを、組み立ての関数を書く前に判断できる。解釈の時間は、ステップ 2-4 の node のテストで測る。実物のコードでの解釈・組み立て・配置の時間は、ステップ 6-4 で測って記録する（設計書 7.2）。
    -   採らなかった案: 実装の後に 1 回だけ測る。上限を超えた場合に、組み立ての関数を作り直すことになる。
    -   ADR の判断: 作らない。確かめる時期と方法の決定で、タスクの中で閉じる。

### 1.4. 既存コード調査結果

**変えるもの:**

-   **本文の表示**: `renderArticleBody`（`extension/src/ui/articleBody.ts:13-22`）が本文を表示する唯一の関数で、呼び出し元は `renderViewer` の 1 か所（`extension/src/ui/renderViewer.ts:319-321`）だけである（`rg -n renderArticleBody extension/src extension/scripts` の結果、`22b24bad`）。ステップ 4-4 で関数とテストを消す。
-   **描き直しの判定**: `drawStateOnChange`（`extension/src/ui/renderJob.ts:361-403`）は、ビューア（`renderViewer.ts:294`）とポップアップ（`extension/src/ui/render.ts:591`）から呼ばれる。ステップ 3-1 で、鍵と子のノードを比べる部分を `drawOnKeyChange` に移す。比較の振る舞いは、今のテスト `extension/test/render.test.ts` の「keeps the checks' nodes and writes only the elapsed time when nothing else changes」（`:1067`）と「draws the job state again after the collected view replaced the checks」（`:1088`）、`extension/test/renderViewer.test.ts:523` が確かめている。
-   **`renderViewer` の順序**: 今は本文（`renderViewer.ts:318-321`）の後に操作のボタン（`:323-335`）を設定している。ステップ 4-3 で順序を入れ替える。記事を隠す処理（`:308-317`）は変えない。
-   **`URL` の型**: `extension/types/core-url.d.ts` は `href` を宣言していない。ステップ 2-1 で加える。
-   **ブランド型の lint**: `brandNames`（`extension/eslint.config.js:166-172`）と作成者の設定（`:297-300` の `creatorConfig`）、テストの表 `brands`（`extension/test/lintRules.test.ts:131-137`）に、`LinkUrl` と `src/core/articleMarkdown.ts` を加える（ステップ 2-3）。`lintRules.test.ts` の `brands` を回すテスト（`:149-` ほか）が、加えた行も確かめる。HTML を解釈する API の禁止（`commonSyntax`、`eslint.config.js:177-216`）は、すべてのファイルの設定と `src/**/*.ts` の設定に入っているので、新しいファイルの設定を加える必要はない。

**書き直すテスト:**

-   `extension/test/articleBody.test.ts`（全体、`renderArticleBody` のテスト）: 子の要素がない、`textContent` が Markdown と一致する、属性が `class` と `dir` だけ、`dir` が `auto`。新しい振る舞いのテストはステップ 4-1・4-2 で加え、古いテストはステップ 4-4 で関数と一緒に消す。
-   `extension/test/renderViewer.test.ts:240-253`・`:255-268`: `.article-body` の `textContent` を `article.markdown` と比べる。テストの記事（`extension/test/helpers/job.ts:29` の `"# Title\n\nBody"`）は、描画すると `textContent` が `TitleBody` になるので、ステップ 4-3 で期待する値を直す。
-   `extension/test/renderViewer.test.ts:705-727`（「keeps the article element when its text is unchanged …」）: ステップ 4-3 で設計書 7.1 の描き直しのテストに書き直す。
-   `extension/test/style.test.ts` の次の 3 つ（ステップ 5-1 で直す）:
    -   `:748-757`「keeps the article's line breaks and spaces and its text direction」: `.article-body` に `pre-wrap` と `plaintext` があることを確かめる。設計書 3.5 では、これらを段落・項目・見出し・コードブロックにかける。
    -   `:836-849`「applies the Japanese rules to the root and body only」: `:lang(ja)` を含む選択子の集合を `:root:lang(ja)` と `body:lang(ja)` に限る。設計書 3.5 のとおり `:root:lang(ja)` の規則に `.article-body` を加えると、選択子の文字列が変わる。
    -   `:858-866`「sets a line height only on the Japanese body」: `line-height` を持つ規則を `body:lang(ja)` に限る。記事の行の高さ（1.8）を加えると失敗する。

**変えずに通ることで確かめるテスト:**

-   保存とコピー（AC-30）: `extension/test/viewerApp.test.ts:800` のコピーのテストは、`article.markdown`（`"# Title\n\nBody"`）がそのまま書き込まれることを確かめる。この記事は描画で除かれる記号（`# `）を含むので、描画した文字列を書き込む退行は、このテストで見つかる。設計書 3.9 はこのテストを「変更」としているが、設計書 7.1 の意図（記号を含む Markdown を使う）はすでに満たしているので、記事は変えない。ステップ 4-3 で、コピーを描画した文字列に変えると失敗することを確かめる。
-   状態の表示（AC-31）: `renderJob.test.ts`・`render.test.ts`・`renderViewer.test.ts` の状態・動画のセクション・操作のテスト。

**再利用するもの:**

-   jsdom の文書: `extension/test/helpers/dom.ts` の `container()`・`viewerDocument()`・`viewerElements()`。
-   CSS の解析: `extension/test/style.test.ts` の `effectiveValue` と `contrastPairs`（`:252-365`）。
-   手動の確認のページ: `extension/scripts/popup-preview.ts` は、ビューアの状態のページを、本物の `renderViewer` と、出荷する `viewer.html`・CSS で書き出す（スクリプトを動かさない）。完了した記事のページの表示用の Markdown（`:219-223`）を、見出し・リスト・コード・リンク・双方向の文字を含む形に変え、ステップ 6-4 の文字組みと双方向の確認に使う（ステップ 5-2）。描画の時間は、スクリプトを動かさないこのページでは測れないので、別の方法で測る（ステップ 6-4）。

**外部の前提の確認:**

-   URL の判定（設計書 3.3 の 4 つの条件）を、AC-13・AC-14 の例と追加の 3 つの例に対して、node `v24.21.0`（`extension/.node-version` と同じ）で実行した（`22b24bad`）。リンクになったのは `https://www.youtube.com/watch?v=abcdefghijk`・`https://xn--nxasmq6b.com/`・`https://localhost/`・`https://127.0.0.1:8443/` だけで、AC-14 の例はすべてリンクにならなかった（`https://` は `new URL` が例外を投げた）。Chrome と Brave で同じ結果になるかは、ステップ 1-1 で確かめる。
-   拡張の最小の Chrome の版は 110（`extension/static/manifest.json:8`）。

**その他:**

-   `implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-09 は、すべて設計書の付録 A に記録されている。
-   新しい型は記事の木だけで、具体的な型による振り分けはない。`LinkUrl` を作る経路は、lint の規則で `src/core/articleMarkdown.ts` に限る（設計書 1.1・3.9）。外側の型への型の主張は lint で防げないので、DOM の属性の値をテストで確かめる（ステップ 4-1・4-2）。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: ブラウザの振る舞いの確認

#### ステップ 1-1: 設計書 3.8 の振る舞いを試作で確かめる

**変更するファイル:** `docs/tasks/0063_viewer_markdown/03_implementation_plan.md`（4.3.1 の記録）。崩れた行の扱いで設計書・要件・ADR-0017 を変える場合は、その文書も。

-   [ ] 作業ツリーの外に、試作のページ（HTML と、そのページの中のスクリプト）を作る。設計書 3.5 の CSS の規則のうち、方向と空白に関わるもの（`pre-wrap`・`plaintext`・リンクの `dir="ltr"` と `unicode-bidi: isolate`）を写す。
-   [ ] 試作で、設計書 3.8 の表の 1〜3 行目を、表の「確かめ方」のとおりに確かめる。3 行目の node の結果は 1.4 の「外部の前提の確認」を使う。
-   [ ] 4 行目は、1.3 の決定のとおり、最悪の場合と同じ数の要素を持つ合成の DOM（例: 512 KiB の `*a*` の繰り返しの 1 行に当たる `em` と文字のノード）を、文書につないでいない `div` の中で作り、`#article` に当たる要素の子を置き換え、配置を強制して、組み立てと配置の時間を分けて測る。
-   [ ] 利用者が、Chrome（110 以降の今の版）と Brave で試作を開き、結果を 4.3.1 の表に記録する（ブラウザと版、確かめた日、結果）。
-   [ ] 崩れた行があれば、設計書 3.8 の「崩れた場合の代わり」に切り替えるかを、レビュアーとその場で決める。要件と ADR-0017 を変える代わり（1 行目）や、要件 AC-33 か描画の規則の見直し（4 行目の配置）が要る場合は、フェーズ 2 に進まない。文書を改める場合は `requirements_process.md`「Editing an approved document」に従う。

**完了の条件:** 4.3.1 の表のすべての行に、Chrome と Brave の結果がある。崩れた行の扱いが、本書（と、変えた場合は設計書）に記録されている。

### フェーズ 2: 解釈の関数

#### ステップ 2-1: インラインの規則とリンクの判定

**変更するファイル:** `extension/src/core/articleMarkdown.ts`（新規）、`extension/types/core-url.d.ts`、`extension/test/articleMarkdown.test.ts`（新規）

-   [ ] `core-url.d.ts` の `URL` に `readonly href: string` を加える（設計書 3.3）。
-   [ ] 記事の木の型（`ArticleInline`・`ArticleLine`・`ArticleItem`・`ArticleBlock`）と `LinkUrl` を、設計書 3.1 の形で書く。`LinkUrl` の名前とファイルは変えない。
-   [ ] 1 行の文字列をインラインの列にする内部の関数を書く。要件 3.2 のインラインの規則（I-1〜I-5 と「入れ子」）を、設計書 3.6 の時間の要求（閉じる記号を開く記号ごとに探し直さない）を満たす形で実装する。リンクの判定は設計書 3.3 の 4 つの条件で、`new URL` の例外は関数の中で受け止める。
-   [ ] テスト（解釈の関数の公開の入口を使う。インラインだけを書き出す必要があれば、ステップ 2-2 の前に仮の書き出しを置かず、ステップ 2-2 と同じコミットにまとめてよい）: 設計書 7.1 の AC-08〜AC-15 の例、H-03 の境目の入力（`\**a**`・`**a\***`・`**a \** b**`・`*a**b*`・`***語***`・`****` など）、AC-10 の行末の `\`、I-5 と I-3 の順序が結果を変える入力（例: `**<https://a.example/>**` と `<https://a.example/**>`）。期待する記事の木は手で書く。
-   [ ] テスト: AC-12 の文字列の一致を、例ごとに、手で書いた期待する文字列と比べる（設計書 7.1）。

**完了の条件:** `make ext-check` が通る。次を壊して、名前を挙げたテストが失敗することを確かめる: I-5 と I-3 の順序を入れ替える（順序の入力）、`href === 文字列` の比較を外す（`<https://example.com>` の例）、ユーザー名の条件を外す（`<https://user:pass@example.com/>` の例）、強調の中で I-2 を当てはめる（AC-11 の例）。

#### ステップ 2-2: ブロックの規則と `parseArticleMarkdown`

**変更するファイル:** `extension/src/core/articleMarkdown.ts`、`extension/test/articleMarkdown.test.ts`

-   [ ] `parseArticleMarkdown` を書く。要件 3.2 の「行」とブロックの規則 B-1〜B-7 を、設計書 3.1 の記事の木の約束（見出しの段は記法の意味、行の空白は詰めない、番号付きリストの開始番号は数値、記号と区切りの文字は持たずに別のリストにする）と、設計書 3.6 の時間の要求（閉じる行の有無を、開く行ごとに以降の行を探し直さずに決める）で実装する。再帰を使わない。
-   [ ] テスト: 設計書 7.1 の AC-01〜AC-07・AC-18・AC-19 の例、B-3 の閉じる行の形（行頭の 0〜3 個の U+0020、開いた並び以上の長さ、後ろの空白、4 個の空白では閉じない、`` ` `` の並びの後ろに `` ` `` のある行は開かない）、CR を含む行、`007.` の開始番号、10 桁の数字の行。
-   [ ] テスト: 表示用の Markdown の全体の形（`internal/writer/output.go:236-240` の出典の行と、`internal/publisher/file_render.go:87-99` のチャンネル名の行の頭のエスケープを含む。AC-07・AC-13）。値は架空のもの（`docs/dev/security.md` §7）。
-   [ ] テスト: AC-21 の文字列（設計書 3.1「AC-21 の形」）を、例ごとに、手で書いた期待する文字列と比べる。2 つ以上の項目のリストを含む例を入れる。

**完了の条件:** `make ext-check` が通る。次を壊して、名前を挙げたテストが失敗することを確かめる: 閉じる行の行頭の空白の上限を 3 から 4 にする（4 個の空白の例）、記号が変わってもリストを分けない（AC-03 の例）、開始番号に最初の項目の値を使わない（AC-04 の `3.` の例）、B-6 で項目の続きにしない（AC-05 の例）。

#### ステップ 2-3: `LinkUrl` の lint の規則

**変更するファイル:** `extension/eslint.config.js`、`extension/test/lintRules.test.ts`

-   [ ] `brandNames` に `LinkUrl` を加え、`creatorConfig("src/core/articleMarkdown.ts", ["LinkUrl"])` を加える（設計書 3.9・7.3）。
-   [ ] `lintRules.test.ts` の `brands` の表に `{ type: "LinkUrl", creator: "src/core/articleMarkdown.ts" }` を加える。

**完了の条件:** `make ext-check` が通る。`brandNames` から `LinkUrl` を外すと、`lintRules.test.ts` の `LinkUrl` の行（ほかのファイルでの型の主張を禁じるテスト）が失敗することを確かめる。

#### ステップ 2-4: 解釈の性能とランダムな入力

**変更するファイル:** `extension/test/articleMarkdown.test.ts`、`extension/test/helpers/randomMarkdown.ts`（新規）

-   [ ] `helpers/randomMarkdown.ts` に、種を固定した擬似乱数と、設計書 7.3 の文字の集合から文字列を作る関数を書く。npm のパッケージは使わない。
-   [ ] テスト（性能、AC-33）: 設計書 7.1「性能」の入力を、`maxResponseBytes`（`extension/src/core/exchange.ts:93`）の長さで、時間を測る範囲の外で作り、`parseArticleMarkdown` の呼び出しだけが 1 秒以内に終わることを確かめる。
-   [ ] テスト（ランダムな入力）: 3 つの種で各 500 個、長さ 0〜300 文字の入力について、解釈が例外を投げないこと、記事の木のすべての `LinkUrl` が印字可能な ASCII の文字だけから成ること（設計書 3.3）。失敗の報告に種と入力を出す。

**完了の条件:** `make ext-check` が通る。閉じる記号を開く記号ごとに行の終わりまで探す形に一時的に変え、性能のテストのうち閉じない記号の入力が失敗することを確かめる。同じく、閉じる行を開く行ごとに以降の行で探す形に変え、閉じない開く行の入力が失敗することを確かめる。

### フェーズ 3: 描き直しの共通の関数

#### ステップ 3-1: `drawOnKeyChange` を取り出す

**変更するファイル:** `extension/src/ui/drawOnChange.ts`（新規）、`extension/src/ui/renderJob.ts`

-   [ ] 設計書 3.4 の `drawOnKeyChange` を書き、`drawStateOnChange` の鍵と子のノードの比較をこれに置き換える。`drawStateOnChange` の振る舞い（描かなかったときだけ経過時間の値を書き換える）は変えない。
-   [ ] 覚えた鍵は `WeakMap` に置き、DOM の属性に書かない（設計書 3.4）。

**完了の条件:** `make ext-check` が、既存のテストを変えずに通る。`drawOnKeyChange` の鍵の比較を外すと `render.test.ts:1067` のテストが、子のノードの比較を外すと `render.test.ts:1088` のテストが失敗することを確かめる。

### フェーズ 4: 組み立てと描画

#### ステップ 4-1: `buildArticleBody` と `drawArticleBody`

**変更するファイル:** `extension/src/ui/articleBody.ts`、`extension/test/articleBody.test.ts`、`extension/test/helpers/articleDom.ts`（新規）

-   [ ] `buildArticleBody` を、設計書 3.2 の表の要素と属性だけで書く。木の種類の `switch` は `default` で `never` に代入して例外を投げる形にする。`a` を作り `href` を設定する関数は 1 つだけで、`LinkUrl` だけを受け取る。子は 1 つずつ加え、配列を引数に広げない。ブロックの間と `li` の間に文字のノードを置かない。
-   [ ] `drawArticleBody` を、表示用の Markdown を鍵とする `drawOnKeyChange` の呼び出しで書く。描くときは、作った `div` 1 つで `container` の子を置き換える。`renderArticleBody` はステップ 4-4 まで残す。
-   [ ] `articleBody.ts` の先頭のコメントを、解釈と組み立てを分けた構成に合わせて書き直す。
-   [ ] `helpers/articleDom.ts` に、作った DOM の木をたどり、各要素の名前、属性の名前と値を設計書 3.2 の表と 7.1 の値の条件で確かめる関数を書く。値は解釈の関数を使わずに確かめる（`a` の `href` は `https://` で始まり、`new URL(値).href` と一致し、ユーザー名とパスワードを持たず、`a` の `textContent` と一致する。`ol` の `start` は 1〜9 桁の 10 進数）。
-   [ ] テスト（`buildArticleBody`）: 設計書 7.1 の組み立ての項目（見出しの段、`ol` の `start`、`pre` と `code`、リンクの属性と文字、記事の要素の `lang` と `dir`）。解釈の関数では作れない記事の木（例: 空の項目の列）も、手で書いた木から確かめる。
-   [ ] テスト（`drawArticleBody`）: `textContent` が、手で書いた期待する文字列と一致すること（AC-12・AC-21）。AC-18・AC-19 の入力に `helpers/articleDom.ts` の関数を当てはめ、タグから要素も属性も作られないこと（AC-18・AC-20）。AC-14・AC-15 の例で `a` が作られないこと。

**完了の条件:** `make ext-check` が通る。次を壊して、名前を挙げたテストが失敗することを確かめる: `a` の `rel` から `noreferrer` を外す（リンクの属性のテスト）、見出しの `title` を `h1` にする（見出しの段のテスト）、項目の間に改行の文字のノードを置く（AC-21 の文字列のテスト）、`helpers/articleDom.ts` の関数が許す属性の一覧に含まれない属性を組み立てで設定する（AC-20 のテスト）。

#### ステップ 4-2: ランダムな入力と最大の大きさの入力

**変更するファイル:** `extension/test/articleBody.test.ts`

-   [ ] テスト（ランダムな入力、設計書 7.3）: ステップ 2-4 と同じ生成の関数と種で、解釈と組み立てが例外を投げないこと、`helpers/articleDom.ts` の確かめが通ること（AC-20）、描画した `textContent` が入力の文字の部分列であること（AC-21 の性質の一部）。失敗の報告に種と入力を出す。
-   [ ] テスト（最大の大きさ、AC-33）: 設計書 7.1「性能」の入力を 512 KiB で解釈し、組み立てても例外を投げないこと。時間は測らない。

**完了の条件:** `make ext-check` が通る。組み立てで子を配列の展開でまとめて加える形に一時的に変え、最大の大きさの入力のテストが失敗することを確かめる（失敗しない場合は、その結果をコミットのメッセージに書く）。`LinkUrl` の代わりに記事の `text` から `href` を作る変更を `buildArticleBody` に入れ、ランダムな入力のテストか AC-18 のテストが失敗することを確かめる。

#### ステップ 4-3: `renderViewer` を切り替える

**変更するファイル:** `extension/src/ui/renderViewer.ts`、`extension/test/renderViewer.test.ts`

-   [ ] `renderViewer` の `textContent` の比較（`:319-321`）を `drawArticleBody` の呼び出しに置き換え、操作のボタンを設定する処理（`:323-335`）をその前に移す（設計書 3.4）。関数のコメント（`:277-284`）を新しい順序と描き直しの判定に合わせて直す。
-   [ ] `renderViewer.test.ts:240-253`・`:255-268` の期待する値を、描画した文字列に直す。
-   [ ] `renderViewer.test.ts:705-727` を、設計書 7.1 の描き直しのテストに書き直す（同じ Markdown で要素を保つ、違う Markdown で置き換える、隠した後に同じ Markdown で描き直す、記事の中のリンクのフォーカスと文字の選択が同じ応答の後も残る。AC-32）。
-   [ ] テスト（操作のボタンの順序）: 設計書 7.1 のとおり、`elements.article` の `replaceChildren` をその要素だけで差し替え、置き換えの時点で保存とコピーのボタンが押せることを確かめる。
-   [ ] テスト（見出しの構造、AC-25）: 出荷する `viewer.html` の文書で、動画のセクションの `h1` の後に、記事の `h2`・`h3` が文書の順に並ぶこと。

**完了の条件:** `make ext-check` が通る。次を壊して、名前を挙げたテストが失敗することを確かめる: ボタンを本文の後に設定する（順序のテスト）、`drawArticleBody` の鍵を毎回変える（描き直しのテスト）。`viewerApp.ts` のコピーを、描画した `textContent` を書き込む形に一時的に変え、`viewerApp.test.ts:800` のテストが失敗することを確かめる（1.4）。

#### ステップ 4-4: `renderArticleBody` とそのテストを消す

**変更するファイル:** `extension/src/ui/articleBody.ts`、`extension/test/articleBody.test.ts`

-   [ ] `renderArticleBody` と、そのテスト（`describe("renderArticleBody", …)` の全体）を、1 つのコミットで消す。このコミットはほかの変更を含まない。
-   [ ] `make ext-coverage-compare BASE=<親のコミット>` を、10 分以上の時間の上限か背景の実行で動かし、出力を最後の行までコミットのメッセージ（ツリーの ID を変えずにメッセージだけを直す）と PR の説明に書く。

**完了の条件:** `make ext-check` が通り、`make ext-coverage-compare` の最後の行が `coverage-compare: PASS` である。消した関数として `renderArticleBody` が挙がる。

### フェーズ 5: 文字組み

#### ステップ 5-1: CSS とそのテスト

**変更するファイル:** `extension/static/viewer.css`、`extension/static/theme.css`、`extension/test/style.test.ts`

-   [ ] 設計書 3.5 の表の値を、`viewer.css` と `theme.css` に書く。`--code-bg` は明るいテーマと暗いテーマの両方で定め、色はすべて変数から取る（`style.test.ts` の「takes every colour from a variable」）。見出しの大きさと余白、リストの字下げ、ブロックの間の余白は実装で決める。番号付きリストの字下げは、9 桁の開始番号の記号が 320 CSS px の幅で記事の枠からはみ出さない値にする（ステップ 6-4 で見る）。
-   [ ] `style.test.ts:748-757` を、設計書 3.5 の空白と方向の規則（段落・項目・見出し・コードブロック）を確かめる形に直す。
-   [ ] `style.test.ts:836-849`・`:858-867` の期待する集合に、記事の規則（`.article-body` を加えた選択子と、記事の行の高さ）を加える。
-   [ ] テスト: 設計書 7.1「CSS」の項目（記事の文字の大きさと行の高さ、記事の内容の最大の幅と `body` の最大の幅、`overflow-wrap: anywhere`、リンクの下線と `unicode-bidi: isolate`、等幅の書体、`--code-bg` の両方のテーマの定義）。`contrastPairs` に、設計書 3.5「コントラスト」の 2 つの組を加える。

**完了の条件:** `make ext-check` が通る。`--code-bg` の暗いテーマの値を背景に近い色にすると、コントラストのテストが失敗することを確かめる。記事の行の高さを 1.6 にすると、行の高さのテストが失敗することを確かめる。

#### ステップ 5-2: 手動の確認のページの記事

**変更するファイル:** `extension/scripts/popup-preview.ts`

-   [ ] 完了した記事のページの表示用の Markdown（`:219-223`）を、タイトルの見出し、チャンネル名の行、節の見出し、段落の中の改行、箇条書きと 9 桁の開始番号の番号付きリスト、強い強調と強調、コードスパンと長い行のコードブロック、出典の行とリンク、行頭の右から左の文字と U+202E を含む行を持つ形にする。日本語・右から左の文字・制御文字は、今のファイルの規則どおり `\u` のエスケープで書く。

**完了の条件:** `make ext-check` が通る（`extension/test/popupPreview.test.ts` を含む）。書き出したページの記事が、上の要素を含むことを、ステップ 6-4 の確認の前に開いて見る。

### フェーズ 6: 文書とコメント

#### ステップ 6-1: security.md

**変更するファイル:** `docs/dev/security.md`

-   [ ] §6 のチャンネル名の行の残余のリスク（`:76`）に、ビューアでの結果（リンク・強調になる）を書き足す（要件 1 章「方針の変更」）。
-   [ ] §8.1「記事を untrusted テキストとして表示する」（`:117`）を、要件の規則と設計書の構成（解釈と組み立ての分離、要素と属性の一覧、`https` の自動リンクだけ、新しいタブ・`noopener noreferrer`）に合わせて書き換える。残余のリスクを、要件 4.2 と設計書 5.3 のとおりに書く。
-   [ ] §8.2「返す記事は untrusted なテキスト」（`:140`）の「Markdown の描画の安全性は #504 で扱う」を、§8.1 と要件を指す形に直す。

**完了の条件:** `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview/... ./internal/pipeline` が通る。

#### ステップ 6-2: CLAUDE.md の例外と、`Article.markdown` のコメント

**変更するファイル:** `CLAUDE.md`、`extension/src/core/response.ts`

-   [ ] 「Declare, don't infer」に、設計書 3.7 の範囲・理由・境界で、`parseArticleMarkdown` の例外を英語で加える（H-08）。ほかの例外（`resolveLocale` など）と同じ形にする。
-   [ ] `response.ts:138` の `Article.markdown` のコメント（`The display Markdown, shown and copied unchanged.`）を、描画して表示し、コピーには変えずに使う意味の英語に直す。

**完了の条件:** `make test`・`make lint`・`make ext-check` が通る。

#### ステップ 6-3: README

**変更するファイル:** `README.md`、`README.en.md`

-   [ ] `README.md:627` の「記事を、Markdown を解釈せずに文字のまま表示し」を、記事を見出し・リストなどに整えて表示し、`https` の自動リンクを新しいタブで開けることを示す文に書き換える。
-   [ ] `/mktrans README.md` で `README.en.md` に反映し、2 つを 1 つのコミットにする。

**完了の条件:** `make readme-sync-check`・`make ext-test`・`make test` が通る。

#### ステップ 6-4: 統合の手動の確認

**変更するファイル:** `docs/tasks/0063_viewer_markdown/03_implementation_plan.md`（4.3.2 の記録）

-   [ ] `make ext-build` で作った拡張と、ステップ 5-2 のページで、設計書 7.2 の項目（キーボードとフォーカス、文字組み、スクリーンリーダー）を確かめる。
-   [ ] 描画の時間（1.3 の決定）: `make ext-build` の拡張をパッケージ化せずに読み込み、ビューアのページの開発ツールのコンソールから `dist/` の `parseArticleMarkdown` と `buildArticleBody` を読み込み、設計書 7.1「性能」の入力と記事の形の入力で、解釈・組み立て・配置の時間を分けて測る。
-   [ ] 結果を 4.3.2 の表に記録する（ブラウザと版、確かめた日、結果）。上限を超えた項目や崩れた項目があれば、レビュアーとその場で扱いを決める。

**完了の条件:** 4.3.2 の表のすべての行に結果がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

フェーズの順序は、設計書 8 章の実装優先順位と同じである。

| マイルストーン | フェーズ | 成果物 |
|---|---|---|
| M1 | 1 | 4.3.1 の記録。崩れた振る舞いの扱いが決まっている |
| M2 | 2 | `parseArticleMarkdown`・`LinkUrl` と、その例・性能・ランダムな入力のテスト。まだ呼ばれない |
| M3 | 3・4 | ビューアが記事を描画する。`renderArticleBody` がなくなる |
| M4 | 5・6 | 文字組み、文書、統合の手動の確認 |

フェーズ 2 と 3 は互いに依存しない（設計書 8 章）。フェーズ 4 は、2 の記事の木と 3 の `drawOnKeyChange` に依存する。

## 4. テスト戦略 (Test Strategy)

### 4.1. 加える・変えるテスト

| テストのファイル | 加える・変える | ステップ |
|---|---|---|
| `extension/test/articleMarkdown.test.ts` | 新規。インライン・ブロック・全体の形・文字列の一致・性能・ランダムな入力 | 2-1・2-2・2-4 |
| `extension/test/helpers/randomMarkdown.ts` | 新規。種を固定した擬似乱数と入力の生成 | 2-4 |
| `extension/test/lintRules.test.ts` | `brands` に `LinkUrl` の行 | 2-3 |
| `extension/test/helpers/articleDom.ts` | 新規。DOM の要素と属性の確かめ | 4-1 |
| `extension/test/articleBody.test.ts` | 組み立て・描画・ランダムな入力・最大の大きさを加え、`renderArticleBody` のテストを消す | 4-1・4-2・4-4 |
| `extension/test/renderViewer.test.ts` | 本文の値、描き直し、ボタンの順序、見出しの構造 | 4-3 |
| `extension/test/style.test.ts` | 記事の CSS、コントラストの組、日本語の規則の集合 | 5-1 |

変えずに通ることを確かめるテストは 1.4 に挙げた。

### 4.2. 互換性のテスト

-   保存とコピーの中身（AC-30）と、記事のほかの表示（AC-31）は、今のテストが変えずに通ることで確かめる（1.4）。
-   サーバ・ホスト・応答の形は変えないので、Go のテストは変えない。`make test` が通ることだけを確かめる。

### 4.3. 手動の確認の記録

#### 4.3.1. 設計書 3.8 の確認（ステップ 1-1）

| 行 | 振る舞い | Chrome（版・日付・結果） | Brave（版・日付・結果） |
|---|---|---|---|
| 1 | `pre-wrap` の段落と項目で、行内の要素をはさんでも改行ごとに文字の方向が決まる（AC-28） | 未確認 | 未確認 |
| 2 | リンクの `dir="ltr"` と `unicode-bidi: isolate` で、直前の制御文字・右から左の文字があっても URL が左から右に並ぶ（AC-17） | 未確認 | 未確認 |
| 3 | `URL` の判定が、AC-13・AC-14 の例と出典の URL で node（1.4）と同じ（H-02） | 未確認 | 未確認 |
| 4 | 合成の DOM（最悪の場合の要素の数）の組み立てと配置の時間（AC-33。1.3） | 未確認 | 未確認 |

#### 4.3.2. 統合の確認（ステップ 6-4）

| 項目 | 確かめること | Chrome（版・日付・結果） | Brave（版・日付・結果） |
|---|---|---|---|
| キーボードとフォーカス（AC-26） | 記事の中のリンクに Tab・Shift+Tab で表示の順に移れ、Enter で新しいタブに開き、フォーカスの輪郭が明るいテーマと暗いテーマで見える | 未確認 | 未確認 |
| 文字組み（AC-22） | 900 CSS px 以上で段落の行の幅が 720px で頭打ちになる。320 CSS px で横のスクロールがない。コードブロックの長い行が折り返す。9 桁の開始番号の記号が枠からはみ出さない | 未確認 | 未確認 |
| 双方向の文字（AC-17・AC-28） | ステップ 5-2 のページで、右から左の文字と U+202E の行が、ほかの行とリンクの並びを変えない | 未確認 | 未確認 |
| スクリーンリーダー（AC-25・AC-26） | アクセシビリティの木で、`h1` の下に `h2`・`h3` が並び、リンクの名前が URL の文字列である | 未確認 | 未確認 |
| ほかの操作の大きさ（AC-29） | ビューアの操作のボタンと元動画へのリンクの大きさが変わらない | 未確認 | 未確認 |
| 描画の時間（AC-33） | 実物のコードでの解釈・組み立て・配置の時間（512 KiB の病的な入力と記事の形の入力） | 未確認 | 未確認 |

## 5. リスク管理 (Risk Management)

### 5.1. 技術的なリスク

| リスク | 影響 | 対策 |
|---|---|---|
| ステップ 1-1 で、改行ごとの方向（行 1）が崩れる | 行を包む要素が要り、要件 F-004 と ADR-0017 を変える | フェーズ 2 に進まず、レビュアーと決める（設計書 3.8） |
| 合成の DOM の配置が 1 秒を超える | 要件 AC-33 か描画の規則の見直しが要る | フェーズ 2 に進む前に、レビュアーと決める（設計書 3.8） |
| インラインの走査が規則と違う結果を出す（高速化の場合分け） | 記号の表示が要件と食い違う | 境目の入力を例のテストで固定し（ステップ 2-1）、ランダムな入力で文字を作らないことを確かめる（ステップ 4-2） |
| jsdom での 512 KiB の組み立てが遅く、`make ext-test` が長くなる | テストの時間が延びる | 最大の大きさの入力の数を、性能のテストの入力の形ごとに 1 つに限る。時間が問題になれば、入力の形を減らすことをレビューで決める |
| 性能のテストが、CI や coverage の計測中に誤って失敗する | テストが不安定になる | 解釈は数十ミリ秒の見込みで、上限との差は 10 倍以上ある（設計書 7.1）。不安定になれば、上限を変えずに原因を調べる |

### 5.2. 日程のリスク

-   ステップ 1-1 は利用者がブラウザで確かめるので、利用者の時間に依存する。崩れた行がなければ、フェーズ 2 と 3 は並べて進められる。

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1（ステップ 1-1）
-   [ ] フェーズ 2（ステップ 2-1〜2-4）
-   [ ] フェーズ 3（ステップ 3-1）
-   [ ] フェーズ 4（ステップ 4-1〜4-4）
-   [ ] フェーズ 5（ステップ 5-1・5-2）
-   [ ] フェーズ 6（ステップ 6-1〜6-4）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストの場所の `describe` の名前は仮のもの（1.2）。種類は `test`（実行して振る舞いを確かめる）、`static`（lint の規則とそのテスト）、`manual`（手動の確認）である。

| AC | 種類 | 確かめ方 | 実装 |
|---|---|---|---|
| AC-01 | test | `extension/test/articleMarkdown.test.ts::parseArticleMarkdown blocks`、`extension/test/articleBody.test.ts::buildArticleBody`（見出しの段） | 2-2・4-1 |
| AC-02 | test | `articleMarkdown.test.ts::parseArticleMarkdown blocks`、`articleBody.test.ts::drawArticleBody`（`textContent`）、`extension/test/style.test.ts::article typography`（`pre-wrap`） | 2-2・4-1・5-1 |
| AC-03 | test | `articleMarkdown.test.ts::parseArticleMarkdown blocks` | 2-2 |
| AC-04 | test | `articleMarkdown.test.ts::parseArticleMarkdown blocks`、`articleBody.test.ts::buildArticleBody`（`start`） | 2-2・4-1 |
| AC-05 | test | `articleMarkdown.test.ts::parseArticleMarkdown blocks` | 2-2 |
| AC-06 | test | `articleMarkdown.test.ts::parseArticleMarkdown blocks`、`articleBody.test.ts::buildArticleBody`（`pre` と `code`） | 2-2・4-1 |
| AC-07 | test | `articleMarkdown.test.ts::parseArticleMarkdown display Markdown` | 2-2 |
| AC-08〜AC-11 | test | `articleMarkdown.test.ts::parseArticleMarkdown inline` | 2-1 |
| AC-12 | test | `articleMarkdown.test.ts::parseArticleMarkdown inline`（文字列）、`articleBody.test.ts::drawArticleBody`（`textContent`） | 2-1・4-1 |
| AC-13 | test, manual | `articleMarkdown.test.ts::parseArticleMarkdown display Markdown`、`articleBody.test.ts::buildArticleBody`（リンク）、4.3.1 の行 3 | 2-1・2-2・4-1・1-1 |
| AC-14・AC-15 | test, manual | `articleMarkdown.test.ts::parseArticleMarkdown inline`、`articleBody.test.ts::drawArticleBody`（`a` がない）、4.3.1 の行 3 | 2-1・4-1・1-1 |
| AC-16 | test, manual | `articleBody.test.ts::buildArticleBody`（`target`・`rel`）、4.3.2 のキーボード | 4-1・6-4 |
| AC-17 | test, manual | `articleBody.test.ts::buildArticleBody`（`a` の `dir`）、`style.test.ts::article typography`（`unicode-bidi: isolate`）、4.3.1 の行 2、4.3.2 の双方向の文字 | 4-1・5-1・1-1・6-4 |
| AC-18・AC-19 | test | `articleMarkdown.test.ts::parseArticleMarkdown blocks`、`articleBody.test.ts::drawArticleBody`（`helpers/articleDom.ts` の確かめ） | 2-2・4-1 |
| AC-20 | test, static | `articleBody.test.ts::drawArticleBody`・`articleBody.test.ts::random input`（`helpers/articleDom.ts` の確かめ）、`extension/test/lintRules.test.ts::branded type assertions`（`LinkUrl`） | 4-1・4-2・2-3 |
| AC-21 | test | `articleMarkdown.test.ts::parseArticleMarkdown blocks`（文字列）、`articleBody.test.ts::drawArticleBody`（`textContent`）、`articleBody.test.ts::random input`（部分列） | 2-2・4-1・4-2 |
| AC-22 | test, manual | `style.test.ts::article typography`（最大の幅、`overflow-wrap`）、4.3.2 の文字組み | 5-1・6-4 |
| AC-23 | test | `style.test.ts::article typography`（文字の大きさと行の高さ） | 5-1 |
| AC-24 | test | `articleBody.test.ts::buildArticleBody`（`lang="ja"`）。ページの `lang` は今の `extension/test/renderViewer.test.ts` のテストが変えずに通ること | 4-1 |
| AC-25 | test, manual | `renderViewer.test.ts::article headings`、4.3.2 のスクリーンリーダー | 4-3・6-4 |
| AC-26 | test, manual | `articleBody.test.ts::buildArticleBody`（リンクの文字が URL の文字列）、`style.test.ts::colour variables`（今の `--focus` の組）、4.3.2 のキーボードとスクリーンリーダー | 4-1・5-1・6-4 |
| AC-27 | test | `style.test.ts::colour variables`（加える 2 つの組）、`style.test.ts::article typography`（リンクの下線） | 5-1 |
| AC-28 | test, manual | `style.test.ts::article typography`（`plaintext`）、4.3.1 の行 1、4.3.2 の双方向の文字 | 5-1・1-1・6-4 |
| AC-29 | test, manual | `style.test.ts::viewer layout` の「makes every pressable element at least 44 CSS pixels wide and tall」（今の選択子の一覧が変わらない）、4.3.2 のほかの操作の大きさ | 5-1・6-4 |
| AC-30 | test | `extension/test/viewerApp.test.ts` の「copies the display Markdown and says so, without moving focus」と保存のテスト（変えずに通る。1.4） | 4-3 |
| AC-31 | test | `renderViewer.test.ts`・`extension/test/renderJob.test.ts`・`extension/test/render.test.ts` の今のテストが変えずに通る | 3-1・4-3 |
| AC-32 | test | `renderViewer.test.ts::article redraw` | 4-3 |
| AC-33 | test, manual | `articleMarkdown.test.ts::parseArticleMarkdown performance`、`articleBody.test.ts::largest input`、4.3.1 の行 4、4.3.2 の描画の時間 | 2-4・4-2・1-1・6-4 |

文書の変更（ステップ 6-1〜6-3）は、要件の方針の変更を書く文の変更で、AC が値や構造を定めていないので、PR のレビューで読む（`manual`）。README の値と構造は、今のガードのテスト（`make readme-sync-check`、`metadataDocs.test.ts` ほか）が確かめる。

## 8. 成功基準 (Success Criteria)

-   7 章のすべての行の確かめが通り、4.3.1 と 4.3.2 の表に結果がある。
-   `make test`・`make lint`・`make ext-check` が通る。
-   ステップ 4-4 の `make ext-coverage-compare` が `PASS` で、その出力がコミットのメッセージと PR の説明にある。
-   security.md・CLAUDE.md・README（と `README.en.md`）が、描画の振る舞いと一致している。

## 9. 次のステップ (Next Steps)

-   本書の承認の後、`/runplan 0063` でフェーズ 1 から実装する。
-   タスクの最後の PR をマージした後、issue #504 を閉じる。
