# 要件定義書：README の表示の言語の段落を、ビューアにも触れる形に直す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、README の「ブラウザ拡張」節にある表示の言語の段落を、拡張の今の振る舞いに合わせるための要件を定義する（[#719](https://github.com/isseis/yt2column/issues/719)）。今の段落はポップアップの表示の言語しか述べていないが、拡張はビューアも同じ規則で選んだ言語で表示している。

**背景:**

`README.md` の「ブラウザ拡張」節の表示の言語の段落（`README.md:574-575`）は、次の 3 文からなる。

1.  ポップアップは、ブラウザの UI の言語が日本語のときは日本語で、それ以外のときは英語で表示する。
2.  拡張に言語の設定はない。
3.  この README が引用するポップアップの文言（概要欄が折りたたまれているときのメッセージなど）は日本語の版である。

英語版の `README.en.md:786-789` も同じ内容で、3 文目は「英語の版である」となっている。どちらの版も、ビューアの表示の言語には触れていない。

一方、今のコードでは、ビューアもポップアップと同じ規則で表示の言語を選ぶ。

-   ビューアの入口は、ブラウザの UI の言語を読む（`extension/src/viewer.ts:19`）。
-   ビューアは、ポップアップと同じ `resolveLocale` で言語を決め、ページの `lang` をその言語にする（`extension/src/viewerApp.ts:178-179`）。ポップアップの同じ処理は `extension/src/launch.ts:681-682` にある。
-   `resolveLocale` は、UI の言語の最初の `-` より前の部分を小文字にして、それが `ja` のときだけ日本語を選ぶ。それ以外の値は、値がない場合も含めて英語になる（`extension/src/core/i18n/locale.ts`、`resolveLocale`）。
-   ビューアの文言は、ポップアップと同じ英語と日本語のメッセージカタログから取る（`extension/src/core/i18n/catalog.ts:148`）。

ビューアを 2 言語にしたのはタスク 0050 である（`docs/tasks/0050_viewer_tab/01_requirements.md` の「issue #503 で決めたこと」の「多言語（#506 の決定）」と AC-49）。段落のもとの英語の文は、ビューアがまだなかったタスク 0043 で書かれた（#529）。その後、タスク 0048 で、日本語の `README.md` を正とする形に移った（#586）。ビューアを加えたときに、この段落は直されなかった。

README の「ブラウザ拡張」節は、ポップアップの文言だけでなく、ビューアの文言も太字で引用している。たとえば **Slack に投稿する**（`README.md:189`）と **記事が失われた**（`README.md:245`）である。ビューアの投稿・保存・コピーの名前は、拡張の node のテストがカタログと照合している（`extension/test/metadataDocs.test.ts:58-60`）。したがって、3 文目がポップアップの文言だけを挙げていると、ビューアの引用がどちらの言語の版なのかが README に書かれないことになる。

タスク 0065 で作る ADR-0029（`docs/dev/adr/0029-extension-ui-languages.md`、本書の作成時点では `main` にない）は、その「決定」に、コードの振る舞いと、README とのこのずれを記す。ずれの記述には本 issue へのリンクが付く（`docs/tasks/0065_adr_remaining/01_requirements.md` AC-03）。README を直すと、このずれの記述は事実でなくなる。

対応 issue: [#719](https://github.com/isseis/yt2column/issues/719)（関連: タスク 0065 の ADR-0029）

**本書の記述範囲:** 本書は、直した後の段落が述べる内容、英語版への写し、ADR-0029 のずれの記述の扱いを定める。段落の正確な文面は、実装で決める。英語版の文面は `/mktrans` が決める。

**issue #719 で決めたこと:**

-   **直す箇所:** `README.md` の段落を、ポップアップとビューアの両方に触れる形に改め、`/mktrans README.md` で `README.en.md` に写す。日本語を正とし、英語版は `/mktrans` で写すのが README の規則だからである（`CLAUDE.md`「Translation Guidelines」）。
-   **テスト:** 直す変更では `make ext-test` を実行する。README の「ブラウザ拡張」節は、拡張の node のテスト（`metadataDocs.test.ts`）が読むからである（`CLAUDE.md`「Development Notes」）。

**要件定義の際に決めたこと（2026-11-09）:**

-   **3 文目をビューアにも広げる:** 「この README が引用する文言は日本語の版である」という 3 文目を、ポップアップとビューアの両方の文言に当てはめる（F-001）。README はビューアの文言も引用しているので、3 文目がポップアップに限られていると、ビューアの引用がどの版なのかを読み手が判断できないからである。ADR：なし。README の 1 文の範囲の決定で、このタスクの中で閉じるので、`CLAUDE.md`「Design decisions (ADR)」の基準で major ではない。
-   **ADR-0029 のずれの記述は、後から `main` に入る側が扱う:** 本タスクの実装の PR の基点で、ADR-0029 がこのずれの記述を持っていれば、その PR で記述を消す（F-003）。本タスクの修正が先に `main` に入れば、ADR-0029 を書くタスク 0065 は、README とコードが一致しているので、ずれを記さない。消すのを片方のタスクに固定すると、もう片方が先に入った場合に、事実でない記述が残るか、まだない記述を消す作業が生じるからである。issue #713 と ADR-0020 のずれの記述でも、同じ扱いにした。ADR：なし。既存の ADR の決定を変えるのではなく、事実でなくなった 1 文を消す後始末で、このタスクの中で閉じるので、major ではない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   README の読み手が、ポップアップとビューアの両方が、ブラウザの UI の言語に合わせて日本語か英語で表示されることを、README から知ることができる。
-   README が引用するポップアップとビューアの文言が、どちらの言語の版なのかが README に書かれている。
-   README と ADR-0029 が、表示の言語について同じことを述べる。

### 2.2. スコープ (In Scope)

-   `README.md` の「ブラウザ拡張」節の、表示の言語の段落（F-001）。
-   その変更の `README.en.md` への写し（F-002）。
-   ADR-0029 が `main` にあり、ずれの記述を持つ場合の、その記述の削除（F-003）。

### 2.3. スコープ外 (Out of Scope)

-   拡張のコードとメッセージカタログ。表示の言語の振る舞いは変えない。issue が求めているのは README の記述をコードに合わせることだからである。
-   数と日時の書式（`Intl.NumberFormat` と `Intl.DateTimeFormat` を、選んだ言語で使うこと）を README に書き足すこと。issue が求めていないからである。
-   表示の言語の段落の外にある README の記述。issue が挙げているのはこの段落だけだからである。
-   表示の言語を確かめる新しい文書のテスト。段落の文の意味は PR のレビューで読んで確かめる、というのが文書のテストの方針だからである（`CLAUDE.md`「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose.」、[ADR-0016](../../dev/adr/0016-document-test-policy.md)）。
-   タスク 0065 が書く ADR-0029 の内容のうち、本書の F-003 が扱うずれの記述以外のもの。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: `README.md` の表示の言語の段落

`README.md` の「ブラウザ拡張」節の表示の言語の段落を、ポップアップとビューアの両方に触れる形に改める。

**Acceptance Criteria**:
- **AC-01**: 段落は、ポップアップとビューアの両方が、ブラウザの UI の言語が日本語のときは日本語で、それ以外のときは英語で表示されることを述べる。
- **AC-02**: 段落は、拡張に言語の設定がないことを、今と同じく述べる。
- **AC-03**: 段落は、この README が引用するポップアップとビューアの文言が日本語の版であることを述べる。
- **AC-04**: 変更前と比べて、`README.md` の変更は表示の言語の段落の中だけにある。

#### F-002: `README.en.md` への写し

F-001 の変更を `/mktrans README.md` で `README.en.md` に写す。

**Acceptance Criteria**:
- **AC-05**: `README.en.md` の「Browser extension」節の対応する段落は、AC-01 と AC-02 の内容を英語で述べ、この README が引用するポップアップとビューアの文言が英語の版であることを述べる。
- **AC-06**: `README.en.md` の 1 行目の同期の目印は、変更後の `README.md` を指し、`make readme-sync-check` が通る。
- **AC-07**: 変更前と比べて、`README.en.md` の変更は、同期の目印の行と、表示の言語の段落の中だけにある。
- **AC-08**: README の「ブラウザ拡張」節を読む既存の文書のテスト（`make ext-test` の `metadataDocs.test.ts` など）が、変更後も通る。

#### F-003: ADR-0029 のずれの記述

本タスクの実装の PR の基点の `main` にある ADR-0029 が、README の表示の言語の段落がビューアに触れていないというずれの記述を持つときに、それを消す。

**Acceptance Criteria**:
- **AC-09**: 実装の PR の基点の `main` に ADR-0029 があり、その「決定」の節が本 issue へのリンクを伴うずれの記述を持つときは、PR の後の ADR-0029 はその記述を持たず、本 issue へのリンクも持たない。ADR-0029 の「決定」の、ほかの記述は変わらない。
- **AC-10**: 実装の PR の基点の `main` に ADR-0029 がないとき、または ADR-0029 がずれの記述を持たないときは、PR は `docs/dev/adr/` の下のファイルを変えない。この場合は、ADR-0029 を書くタスク 0065 がずれを記さなくてよいことを、PR の説明に書く。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文書だけの変更で、実行時の処理は変わらない。

### 4.2. セキュリティ (Security)
-   N/A。外部のコマンドの呼び出し、API キーと Webhook の URL、キャッシュ、ネットワークへの送信、信頼できない文字列のプロンプトや記事への埋め込みのどれにも当たらないので、`docs/dev/security.md` の適用の条件を満たさない。信頼できない入力も扱わない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。

### 4.4. 互換性 (Compatibility)
-   N/A。拡張・サーバ・ホストの振る舞いは変わらない。

### 4.5. 保守性 (Maintainability)
-   `README.md` を先に変え、`README.en.md` は `/mktrans` で写し、2 つを 1 つのコミットに入れる（`CLAUDE.md`「Translation Guidelines」）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   README の「ブラウザ拡張」節は拡張の node のテストが読むので、変更の後に `make ext-test` を実行する（`CLAUDE.md`「Development Notes」）。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **ビューア**（viewer）: 記事の状態と記事を表示する拡張のページ（`viewer.html`）。用語集の定義に従う。
-   **ポップアップ**（popup）: 拡張のツールバーのアイコンをクリックして開く画面。
-   **表示の言語**: 拡張が画面の文言を表示する言語。ブラウザの UI の言語から、日本語か英語のどちらかに決まる。
-   **ずれの記述**: ADR-0029 の「決定」にある、README の段落とコードが食い違っていることの記述（`docs/tasks/0065_adr_remaining/01_requirements.md` AC-03）。
