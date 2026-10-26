# 要件定義書：ブラウザ拡張の UI の多言語化（英語と日本語）

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-27 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ブラウザ拡張の UI を英語と日本語の 2 言語で表示するための要件を定義する（[#506](https://github.com/isseis/yt2column/issues/506)）。言語はブラウザの表示言語から決め、拡張の中に切り替えの設定は設けない。

**背景:**

今の拡張は、利用者に表示する文言も英語で書く決まりになっている（`CLAUDE.md:397-400`、`docs/tasks/0007_browser_extension_skeleton/01_requirements.md:92`）。拡張の主な利用者は日本語の話者であり、記事の本文はプロンプトの指示によって素材の言語によらず日本語で書かれる（issue #506 の「背景」）。日本語の利用者が、英語の UI と日本語の記事を行き来することになっている。

ポップアップに表示する文言は、次の場所に英語で書かれている。

-   収集の拒否の理由と促しのメッセージ: `extension/src/core/messages.ts:18`（`expandPromptMessage`）・`extension/src/core/messages.ts:31`（`rejectionMessage`）
-   送信しなかった理由・サーバの拒否の理由・送信の結果: `extension/src/core/sendMessages.ts:18`・`:38`・`:92`・`:115`・`:145`
-   送信中の案内: `extension/src/ui/renderSend.ts:12`（`sendingMessage`）
-   要約と動画メタデータの欄の名前: `extension/src/ui/render.ts:48-66`・`extension/src/ui/render.ts:125-129`
-   送信ボタン: `extension/src/launch.ts:207`（`Send to yt2column`）

サーバは拒否の理由を固定の値（`reason`）で返し、拡張がそれを文言に変える（`extension/src/core/sendMessages.ts:38`）。このため、サーバを変えずに拡張の中だけで多言語にできる。

ポップアップの再設計（[#507](https://github.com/isseis/yt2column/issues/507)）と Viewer（[#503](https://github.com/isseis/yt2column/issues/503)）は、文言をほぼすべて書き直す。その前に多言語の土台を入れて、二度手間を避ける。本タスクは親 issue [#501](https://github.com/isseis/yt2column/issues/501) の子 issue の 1 つで、[#502](https://github.com/isseis/yt2column/issues/502) と並行して進められ、#507 は本タスクを待つ。

対応 issue: [#506](https://github.com/isseis/yt2column/issues/506)（親: [#501](https://github.com/isseis/yt2column/issues/501)、関連: [#507](https://github.com/isseis/yt2column/issues/507)・[#503](https://github.com/isseis/yt2column/issues/503)・[#298](https://github.com/isseis/yt2column/issues/298)）

**本書の記述範囲:** 本書は、観測できる振る舞い（どの言語をいつ選び、何をその言語で表示し、何を表示どおりに残すか）と、文言が満たすべき性質を定める。日本語の文言の正確な字句、言語の判定を置く場所、テストで言語を切り替える手段、README のガードのテストと英語の文言を対応させる手段、ページの `lang` 属性と書体を設定する手段、日本語の行間の値は、設計（`02_architecture.md`）で決める。要件の検討で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #506 で決めたこと:**

-   **対象:** 拡張の UI（今のポップアップと、#503 以降の Viewer の外枠）。記事の本文、サーバのログ、CLI、README の翻訳は対象外とする。サーバを変えずに、拡張の中だけで多言語にできるためである。
-   **言語:** `en` と `ja`。言語は型（例: `Locale = "en" | "ja"`）で表す。
-   **言語の決め方:** ブラウザの表示言語（`chrome.i18n.getUILanguage()`）が日本語なら `ja`、それ以外は `en` にする。知らない言語のときは `en`。拡張の中で言語を切り替える設定は設けない。拡張は設定画面を持たず、何も保存しない、という今の性質を保つためである（`README.md:908`）。
-   **文言の持ち方:** TypeScript の表に、言語ごとの文言を持つ。表を `Record<Locale, …>` の型にし、片方の言語の文言が抜けたら型の検査で落ちるようにする。数を埋め込む文言は、値を受け取る関数で組み立てる（英語の複数形、日本語の単位の位置が言語で異なるため）。
-   **manifest:** 拡張の名前と説明は、Chrome 標準の `_locales/<lang>/messages.json` で多言語にする。
-   **YouTube のボタンの名前:** 手順の中で示す YouTube のボタンの名前（`"...more"`、`"Show transcript"` など）は、拡張の言語に合わせる。YouTube の表示言語と拡張の言語が違うと名前が画面と一致しないが、まれな場合として許容する。
-   **数と日時:** `Intl.NumberFormat` と `Intl.DateTimeFormat` を、選んだ言語で使う。
-   **ページの言語:** 拡張のページの `lang` 属性を、選んだ言語に合わせる。日本語では、書体の候補（`Hiragino Sans`、`Noto Sans JP`、`Yu Gothic UI`）と行間を日本語に合わせる。
-   **画面の構成は変えない:** 今のポップアップのすべての文言を表に移して日本語版を加えるが、画面の構成は変えない。再設計は #507 で行う。
-   **CLAUDE.md の改定:** 識別子とコメントは英語のままとし、利用者に表示する文言は言語ごとの表に置き、英語と日本語の両方を必須にする。
-   **用語:** 日本語の文言は、`docs/translation_glossary.md` の用語に合わせる。

**要件定義の際に決めたこと（2026-10-27）:**

-   **日本語と判定する値:** 表示言語の値のうち、最初の `-` より前の部分（言語の部分）が `ja` に一致するもの（大文字・小文字は区別しない。例: `ja`、`ja-JP`）を日本語とする（F-001）。issue の「`ja` で始まれば」を文字どおりの前方一致にすると、`jam`（ジャマイカ・クレオール語）のように `ja` で始まる別の言語の値も日本語になってしまうためである。
-   **manifest の名前と説明:** 名前は製品名なので、両方の言語で `yt2column` のままとする。説明（`description`）を新しく加え、言語ごとに訳す（F-004）。今の manifest は `description` を持たない（`extension/static/manifest.json:1-11`）。
-   **YouTube のボタンの名前の日本語の表記:** 本書では表記を定めず、「日本語表示の YouTube で観測した表記と一致すること」だけを要件とする（AC-11）。表記は設計で観測し、観測の条件（日付・ブラウザ・YouTube の表示言語）とともに `02_architecture.md` に記録する。日本語の表記はまだ誰も確かめていないので、要件書が確かめていない表記に依存しないためである。
-   **開発者向けのログは英語のまま:** ポップアップのコンソールに出すログ（`extension/src/browser/chromeDeps.ts:148-153`）は利用者向けの UI ではないので、多言語化の対象外とし、選んだ言語によらず英語のままとする（F-002）。README は、開発者がポップアップのコンソールでログを読む手順を英語で説明している（`README.md:972-979`）。
-   **README に言語の決まり方を書く:** README は翻訳しないが、「ポップアップはブラウザの表示言語に合わせて英語か日本語で表示され、README は英語の文言を引用している」ことを説明する文を加える（F-005）。README はポップアップの英語の文言を引用している（`README.md:916`・`README.md:922-926`）。日本語の UI の利用者が、引用と画面の違いに戸惑わないようにするためである。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   ブラウザの表示言語が日本語の利用者には、拡張の UI を日本語で表示する。それ以外の利用者には、今までどおり英語で表示する。
-   どちらの言語でも、今の文言が持つ性質（理由ごとに異なる文言、Slack の確認を求める場合と求めない場合の区別など）を保つ。
-   #507 と #503 が、言語ごとの文言を前提に文言を書けるようにする。

### 2.2. スコープ (In Scope)

-   言語の型、言語の決め方、言語ごとの文言の表（多言語の土台）
-   今のポップアップに表示するすべての文言の、表への移動と日本語版の追加（1 章「背景」の一覧）
-   数の表示の、選んだ言語の書式への変更
-   ポップアップのページの `lang` 属性と、日本語の書体・行間
-   `_locales/en`・`_locales/ja` と、manifest の `default_locale`・`description`
-   CLAUDE.md の言語の規則の改定
-   README への、ポップアップの言語の決まり方の説明の追加

### 2.3. スコープ外 (Out of Scope)

-   記事の本文、サーバのログ、CLI、README の翻訳（issue #506 の「決めたこと」）。
-   Slack の脚注と失敗の通知の言語。Slack への投稿は #501 で任意になり、通貨の表記は #298 で扱う（issue #506 の「対象外」）。
-   英語と日本語以外の言語（issue #506 の「対象外」）。
-   拡張の中で言語を切り替える設定（issue #506 の「決めたこと」）。
-   開発者向けのコンソールのログの多言語化（1 章「要件定義の際に決めたこと」）。
-   ポップアップの画面の構成の変更（#507）と Viewer（#503）。Viewer の外枠は、本タスクの土台を使って #503 で多言語にする。
-   日時の表示。今のポップアップは日時を表示しない。日時を表示する画面を加えるときは、5 章の制約に従う。
-   サーバ・ホスト・API の変更。拒否の理由（`reason`）の値は変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 表示する言語の決定

拡張は、ブラウザの表示言語から、表示する言語を `en` か `ja` に決める。利用者が拡張の中で言語を選ぶ手段はない。

**Acceptance Criteria**:
- **AC-01**: ブラウザの表示言語の値の、最初の `-` より前の部分が、大文字・小文字を区別せずに `ja` と一致するとき（例: `ja`、`ja-JP`、`JA`）、拡張は日本語で表示する。
- **AC-02**: それ以外の値（例: `en`、`en-US`、`fr`、`zh-CN`、`jam`、`jv`、空文字列）のとき、拡張は英語で表示する。
- **AC-03**: 1 つのページ（ポップアップ）に表示する文言は、すべて同じ 1 つの言語である。英語と日本語の文言が 1 つのページに混ざらない（ページの値・サーバの値として表示するもの（AC-08）は除く）。
- **AC-04**: 拡張は言語の設定画面を持たず、言語の選択を保存しない。manifest の `permissions` は今の `activeTab`・`scripting`・`nativeMessaging` のままで（`extension/static/manifest.json:10`）、`options_ui` を宣言しない。

#### F-002: 言語ごとの文言

ポップアップに表示する文言を、言語ごとの表から選んだ言語で表示する。

**Acceptance Criteria**:
- **AC-05**: 選んだ言語で、次の文言を表示する。収集の拒否の理由（4 つ）、送信しなかった理由（`body-too-large`）、サーバの拒否の理由（6 つ）、送信の結果（受け付けた・未送信の 2 つの原因・応答なし・ホストの失敗・想定外の応答）、送信中の案内、促しのメッセージ、要約と動画メタデータの欄の名前（8 つ）、送信ボタンの名前。それぞれの手順（`steps`）も同じ言語で表示する。
- **AC-06**: どちらかの言語の文言が 1 つでも欠けていると、`make ext-typecheck` が失敗する。
- **AC-07**: 英語の文言は、本タスクの前の文言と一字一句同じである（1 章「背景」の一覧の場所にある文言）。
- **AC-08**: ページやサーバから得た値（タブのタイトル、URL、選択範囲のプレビュー、動画タイトル、チャンネル名、概要欄、リクエスト ID）は、言語によらず、今までどおり得たままの値を表示する。翻訳も書き換えもしない。
- **AC-09**: どちらの言語でも、文言は次の性質を満たす（今の英語の文言が満たしている性質。`extension/test/messages.test.ts`・`extension/test/sendMessages.test.ts`）。
    -   収集の拒否の理由、送信しなかった理由、サーバの拒否の理由、送信の結果は、それぞれ同じ言語の中で互いに異なる文言である。
    -   未送信（`server-unavailable`・`message-too-large`）の文言は、Slack の確認を求めない。
    -   結果不明（応答なし・ホストの失敗・想定外の応答）の文言は、送り直す前に Slack を確かめるよう案内する。
    -   受け付けた場合と `too_many_requests` の文言は、Slack を確かめるよう案内する。
    -   `body-too-large` の文言は、上限（4 MiB）を示す。
    -   ホストの失敗の文言は `yt2column-nmhost install` を、`server-unavailable` の文言はサーバの起動を示す。
    -   想定外の応答は、検査の種類によらず 1 つの文言で、検査の種類を表示しない。
    -   オプションページ・トークン・ポートに触れない。
    -   コマンド名（`yt2column-server`、`yt2column-nmhost install chrome` など）・`Slack`・URL の例は、言語によらず同じ表記で示す。
- **AC-10**: 日本語の文言は、`docs/translation_glossary.md` にある用語を使う（例: 文字起こし、概要欄、動画タイトル、チャンネル名、リクエスト ID、ホスト、未送信）。
- **AC-11**: 日本語の文言の中で示す YouTube のボタンやリンクの名前（英語の `"...more"`・`"Show transcript"` に当たるもの）は、日本語表示の YouTube で観測した表記と一致する。観測の条件（日付・ブラウザ・YouTube の表示言語）と表記は `02_architecture.md` に記録されている。
- **AC-12**: 開発者向けのコンソールのログに出す文字列は、選んだ言語によらず英語である。

#### F-003: 数の書式とページの言語

ポップアップに表示する数とページの言語の情報を、選んだ言語に合わせる。

**Acceptance Criteria**:
- **AC-13**: 要約の文字数と行数は、選んだ言語の数の書式（`Intl.NumberFormat`）で表示する。例: 文字数 `12345` は、英語でも日本語でも `12,345` と表示する。今の表示（`12345`。`extension/src/ui/render.ts:55-60`）から、英語でも桁区切りが加わる。
- **AC-14**: ポップアップのページの `html` 要素の `lang` 属性は、選んだ言語（`en` または `ja`）である。今は常に `en` である（`extension/static/popup.html:2`）。
- **AC-15**: 日本語で表示するとき、ページの書体の候補は、`Hiragino Sans`・`Noto Sans JP`・`Yu Gothic UI` を、汎用の書体（`sans-serif`）より前に含む。
- **AC-16**: 日本語で表示するときの本文の行間は、英語で表示するときの行間より広い。値は設計で決める。
- **AC-17**: 英語で表示するときの書体と行間は、今のまま変わらない（`extension/static/style.css:1-5`）。

#### F-004: manifest の名前と説明

拡張の管理ページ（`chrome://extensions/`）に表示する名前と説明を、Chrome 標準の `_locales` で多言語にする。

**Acceptance Criteria**:
- **AC-18**: ビルドした拡張（`extension/dist/`）は、`_locales/en/messages.json` と `_locales/ja/messages.json` を含み、manifest の `default_locale` は `en` である。
- **AC-19**: manifest の `name` と `description` は、`_locales` の文言を参照する。拡張の名前は、どちらの言語でも `yt2column` である。説明は、言語ごとに訳した文言である。
- **AC-20**: manifest の最上位のキーは、今のキー（`extension/test/manifest.test.ts:32-40`）に `default_locale` と `description` を加えたものとちょうど一致する。`key` は変わらず、拡張 ID は `clfmbbcdpnjcefbdihdoahomaifbabkk` のままである。

#### F-005: 文書の改定

言語の規則と、ポップアップの言語の決まり方を文書に記す。

**Acceptance Criteria**:
- **AC-21**: CLAUDE.md の「Go Idioms」の拡張の言語の規則（`CLAUDE.md:397-400`）が、次のように改められている。拡張のソースのコメントと識別子は英語である。利用者に表示する文言は、言語ごとの文言の表に置き、英語と日本語の両方を必須とする。開発者向けのログは英語である。日本語の文書の見出しや表のセルに一致させるガードのテストの文字列リテラルを例外とする、今の規定は保つ。
- **AC-22**: README の「Browser extension」の節が、ポップアップはブラウザの表示言語が日本語なら日本語、それ以外は英語で表示されること、拡張の中に言語の設定はないこと、README が引用するポップアップの文言は英語版であることを説明している。
- **AC-23**: README が引用している促しのメッセージ（`README.md:922-926`）は、英語版の文言と一致したままである。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。文言の表を引くだけの処理で、送信の経路の時間に影響しない。

### 4.2. セキュリティ (Security)

-   条件付きのガイド（`docs/dev/security.md`）: N/A。本タスクは外部コマンドの起動、秘密情報、キャッシュ、ネットワークへの送信、信頼できないテキストのプロンプトや記事への埋め込みのどれにも触れない。
-   **AC-24**: 言語によらず、ポップアップは文言とページの値を、HTML として解釈しない文字列として表示する（今の規則。`extension/src/ui/render.ts:97-101`）。言語の表の文言にも、この規則を当てはめる。
-   manifest の `permissions` は増やさない（AC-04）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   ブラウザの表示言語が取得できない場合や想定外の値の場合も、ポップアップは英語で表示し、収集と送信の振る舞いは変わらない（AC-02）。

### 4.4. 互換性 (Compatibility)

-   `minimum_chrome_version`（`"103"`。`extension/static/manifest.json:6`）は変えない。
-   Chrome と Brave の両方で、F-001 から F-004 の振る舞いになる。
-   サーバ・ホスト・API（拒否の理由の値を含む）は変えない。

### 4.5. 保守性 (Maintainability)

-   言語を 1 つ加えるときは、言語の型に値を加え、表に文言を加えれば済み、文言を表示するコードを変えずに済むこと。表に文言が欠けていれば型の検査で失敗する（AC-06）。
-   #503 の Viewer は、本タスクの言語の型・言語の決め方・文言の表を、そのまま使えること。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   **CLAUDE.md の方針の変更:** 「Go Idioms」の「利用者に表示する文字列も英語で書く」規則（`CLAUDE.md:397-400`）と、それを定めた 0007 の要件（`docs/tasks/0007_browser_extension_skeleton/01_requirements.md:92`）を、本タスクで改める（AC-21）。理由は 1 章「背景」のとおり、主な利用者が日本語の話者であることである。0007 の要件書は完了したタスクの記録として変えない。
-   **0007 の manifest の設計の変更:** 0007 の設計は manifest の最上位のキーを表で固定している（`docs/tasks/0007_browser_extension_skeleton/02_architecture.md:268-281`）。本タスクは `default_locale` と `description` を加える（AC-20）。Chrome は `_locales` を持つ拡張に `default_locale` を求め、issue #506 が `_locales` による名前と説明の多言語化を決めたためである。
-   **CLAUDE.md「Declare, don't infer」の例外:** 方針（`CLAUDE.md:203`）は、呼び出し元から渡された文字列の内容を調べて振る舞いを選ぶことを禁じる。言語の決定（F-001）は、ブラウザが返す表示言語の文字列を調べて `en` か `ja` を選ぶ。範囲: 例外は、表示言語の文字列を言語の型に変える 1 か所だけに限る。それより後の処理は、文字列ではなく言語の型で振る舞いを選ぶ。理由: 表示言語はブラウザが返す外部の値で、拡張の型では表せない。境界: 日本語と判定できない値は、広く解釈せず英語にする（AC-02）。
-   **日時の表示:** 日時を表示する画面（#503 など）を加えるときは、`Intl.DateTimeFormat` を選んだ言語で使う（issue #506 の「決めたこと」）。本タスクの時点では日時を表示しない。
-   **数を埋め込む文言:** 数などの値を埋め込む文言を加えるときは、値を受け取って言語ごとに組み立てる（issue #506 の「決めたこと」）。今のポップアップの文言に、実行時の値を埋め込むものはない（`4 MiB` は固定の文言である）。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **表示言語:** ブラウザが UI に使う言語（`chrome.i18n.getUILanguage()` が返す値）。YouTube の表示言語とは別である。
-   **選んだ言語:** F-001 で決めた、拡張が表示に使う言語（`en` または `ja`）。
-   **文言の表:** 言語ごとの文言を持つ、拡張の TypeScript の表。
-   **促しのメッセージ（expand prompt）:** 概要欄が折りたたまれているときに表示する固定の文言（用語集のとおり）。
