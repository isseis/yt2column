# 要件定義書：security.md §6 の出典のリンクの項目を、どの記事にも当てはまる形に直す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`docs/dev/security.md` §6「信頼できないテキストの取り扱い」にある出典のリンクの項目を、コードの今の振る舞いに合わせるための要件を定義する（[#725](https://github.com/isseis/yt2column/issues/725)）。今の項目は出典のリンクの保証を Slack への投稿に限っているように読めるが、コードはどの記事にも出典のリンクを付けている。

**背景:**

`docs/dev/security.md:71` の項目は、「生成記事を Slack に投稿する前に、出典リンクがコード側で付与されていること（LLM の出力に依存しないこと）を確認する」と述べる。この文は、出典のリンクの保証を Slack への投稿に限っているように読める。また、§6 のほかの項目の多くは守る規則を述べるが、この項目は投稿の前に行う点検として書かれている。

一方、今のコードでは、出典のリンクは投稿に限らず、どの記事にも付く。

-   `newArticle` は、記事の本文（`Article.Body`）を、LLM が書いた本文、区切りの `\n\n`、出典のブロックの順につないで作る（`internal/writer/output.go:220`）。出典のブロックは `出典: <URL>` の形の自動リンクで（`internal/writer/output.go:29`・`:239`）、URL は検証した動画 ID から組み立て、生成したテキストの URL は使わない（`internal/writer/output.go:210-215` のコメント、[ADR-0034](../../dev/adr/0034-template-allowlist.md)）。
-   サーバが返す表示用の Markdown は、見出しとチャンネル名の行の後に `Article.Body` をそのまま置く（`internal/publisher/display_render.go:28-35`）。ビューアはこれを表示し、ビューアからの保存とコピーもこの文字列を変えずに使う（`docs/dev/security.md` §8.1 の「記事を untrusted テキストとして描画する」の項目）。
-   `--out` のファイルは、フロントマターの後に同じ表示用の Markdown を書く（`internal/publisher/file_render.go:66`）。
-   Webhook への投稿は、`Article.Body` から本文を作る（`internal/publisher/slack_render.go:74`）。
-   CLI は、タスク 0062 から Webhook へ投稿しない（ADR-0034 の「決定」の最後の項目）。

文書はコードより狭いが、矛盾はしない。タスク 0065 は、ADR-0034 の「決定」の最後の項目にコードの振る舞いを書き、このずれを本 issue にリンクした（`docs/tasks/0065_adr_remaining/02_architecture.md` 3.5 の C4 の行）。その項目の後半の 3 文（`docs/dev/adr/0034-template-allowlist.md:24`。以下「ずれの記述」と呼ぶ。6 章も参照）は、§6 の項目が投稿する記事に限っているように読めること、コードは投稿しない記事にも出典のブロックを付けること、このずれを本 issue で扱うことを述べる。§6 を直すと、ずれの記述は事実でなくなる。

**軽量プロセスを取らない理由:** この変更は `docs/dev/security.md` を変える。このファイルは軽量プロセスの条件の表で除外されている（`docs/dev/conventions/requirements_process.md`「Mechanically checked conditions」の `exclude docs/dev/security.md`）。また、この変更は保証の対象を投稿する記事からすべての記事へ広げ、文の意味を変える。これは「常に通常プロセスになる変更」の「A change of wording that changes a meaning」と「A change within the scope of `docs/dev/security.md`」に当たる（同書「Changes that always take the normal process」）。

対応 issue: [#725](https://github.com/isseis/yt2column/issues/725)（関連: [ADR-0034](../../dev/adr/0034-template-allowlist.md)、[ADR-0039](../../dev/adr/0039-adr-writing-criteria.md)）

**本書の記述範囲:** 本書は、直した後の §6 の項目が述べる内容と、ADR-0034 のずれの記述の扱いを定める。項目の正確な文面は、実装で決める。

**issue #725 で決めたこと:**

-   **直す対象:** `docs/dev/security.md` §6 の、出典のリンクを Slack への投稿に限っているように読める項目。
-   **直し方の方向:** 投稿に限らず、保存・ビューアでの表示・投稿のどれに使う記事にも出典のリンクをコードで付ける、と述べる形に改める。issue が案として挙げた方向で、コードの今の振る舞い（上の背景）に合う。

**要件定義の際に決めたこと（2026-11-09）:**

-   **項目を、点検ではなく規則の形にする:** 直した後の項目は、出典のリンクを、検証した動画 ID からコードで組み立てて付け、LLM の出力に依存しないこと、それが保存・ビューアでの表示・投稿のどれに使う記事にも当てはまることを、守る規則として述べる（F-001）。今の「投稿の前に確認する」という点検の形を保ち、対象だけを広げる案は採らない。§6 のほかの項目の多くは守る規則を述べる形で、コードもどの記事にも同じ方法で出典を付けるので、点検の時点を投稿の前に置く理由がないからである。ADR：なし。security.md の 1 項目の直しで、ADR-0034 の決定（出典のリンクはコードで付け、どの出力先の記事にも付く）を変えないので、`CLAUDE.md`「Design decisions (ADR)」の基準で major ではない。
-   **ADR-0034 のずれの記述は、本タスクの実装の PR が、編集上の訂正として消す:** §6 を直す実装の PR が、同じ PR で、ADR-0034 の「決定」の最後の項目からずれの記述の 3 文を消す（F-002）。
    -   **根拠:** ADR-0039 の「決定」は、ずれの記述はずれを直す PR が同じ PR で消すこと、決定を下しも変えもしない PR も ADR に編集上の訂正を加えてよいことを定める（`docs/dev/adr/0039-adr-writing-criteria.md` の「決定」の 4 項目めと 5 項目め）。訂正はコミットメッセージと PR の説明に記録し、「経緯」に項目を足さない。
    -   **規約の本文との関係:** 規約の「Approval and the PR」の本文（`docs/dev/conventions/adr.md:215-219`）は、ADR を変えるのはその決定を下すか変える PR だけであると今も述べる。ADR-0039 の決定を規約の本文に足すことは、タスク 0070 の実装（[#728](https://github.com/isseis/yt2column/issues/728)）で扱う。本タスクは、効いている決定である ADR-0039 に従い、0069 のような本タスクに限る規約の例外は設けない。
    -   **採らなかった案:** 0069 と同じく本タスクに限る規約の例外として扱う案は、ADR-0039 が同じ扱いを一般の規則として定めた後なので採らない。§6 だけを直してずれの記述を別の PR で消す案は、ずれの記述が事実でない期間を作るので採らない（ADR-0039）。
    -   ADR の判断：ADR は作らず、変えない。ずれの記述の削除は ADR-0034 の決定を変えず、扱いは ADR-0039 の決定のとおりなので、`CLAUDE.md`「Design decisions (ADR)」の基準で major ではない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `docs/dev/security.md` の読み手が、出典のリンクが LLM の出力によらずコードで付き、それが投稿する記事に限らずどの記事にも当てはまることを、§6 から知ることができる。
-   ADR-0034 に、事実でなくなったずれの記述が残らない。

### 2.2. スコープ (In Scope)

-   `docs/dev/security.md` §6 の、出典のリンクの項目（F-001）。
-   ADR-0034 のずれの記述の削除（F-002）。

### 2.3. スコープ外 (Out of Scope)

-   コードの変更。出典のリンクの付け方は変えない。issue が求めているのは、文書の記述をコードに合わせることだからである。
-   §6 の、出典のリンクの項目の外の記述と、§6 の外の `security.md` の記述。issue が挙げているのはこの項目だけだからである。
-   ADR-0034 の、ずれの記述の外の記述と、ほかの ADR の編集。1 章の「要件定義の際に決めたこと」の範囲の外だからである。
-   `docs/dev/project_overview.md` の記述。「記事の末尾には必ず元動画へのリンク（出典）を付ける」（`docs/dev/project_overview.md:85`）は、すでに記事を出力先で限っていないからである。
-   規約 `docs/dev/conventions/adr.md` の変更。ADR-0039 の決定を規約に足すことは、タスク 0070 の実装（#728）で扱う。
-   §6 の項目の内容を確かめる新しい文書のテスト。項目の文の意味は PR のレビューで読んで確かめる、というのが文書のテストの方針だからである（`CLAUDE.md`「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose.」、[ADR-0016](../../dev/adr/0016-document-test-policy.md)）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: security.md §6 の出典のリンクの項目

`docs/dev/security.md` §6 の出典のリンクの項目を、どの記事にも当てはまる規則の形に改める。

**Acceptance Criteria**:
- **AC-01**: 項目は、出典のリンクを LLM の出力ではなくコードで付けること、その URL を検証した動画 ID から組み立て、生成したテキストの URL を使わないことを述べる。
- **AC-02**: 項目は、出典のリンクが、`--out` のファイルへの保存、ビューアでの表示（ビューアからの保存とコピーを含む）、Webhook への投稿のどれに使う記事にも付くことを述べる。項目は、出典のリンクの保証を Slack への投稿に限る文を含まない。
- **AC-03**: 項目は、投稿の前に行う点検としてではなく、守る規則として書かれている。
- **AC-04**: 変更前と比べて、`docs/dev/security.md` の変更は §6 の出典のリンクの項目の中だけにある。

#### F-002: ADR-0034 のずれの記述の削除

§6 を直す実装の PR が、同じ PR で、ADR-0034 のずれの記述を消す（1 章の「要件定義の際に決めたこと」）。

**Acceptance Criteria**:
- **AC-05**: 実装の PR の後の ADR-0034 の「決定」の最後の項目は、太字の 1 文目（「**出典のブロックは、保存・ビューアでの表示・投稿のどれに使う記事にも付く。**」）、出典のブロックが記事の本文の一部であり 3 つの出力先がこの本文から作られることを述べる文、信頼できないテキストの扱いの仕様の正本が security.md の §6 であることを述べる文からなる。その後にあった、§6 が投稿する記事に限っているように読めること、コードは投稿しない記事にも出典のブロックを付け CLI はタスク 0062 から Webhook へ投稿しないこと、このずれを #725 で扱うことを述べる 3 文はない。
- **AC-06**: 変更前と比べて、実装の PR の `docs/dev/adr/` の下の変更は、AC-05 の 3 文の削除だけにある。経緯の項目は足さない。
- **AC-07**: 実装の PR の、ADR-0034 を変えるコミットのメッセージと PR の説明は、この削除を、決定を変えない編集上の訂正として記す（ADR-0039 の「決定」と、`docs/dev/conventions/requirements_process.md`「Recording in the PR description」の訂正の行と訂正の段落の形）。

#### F-003: 既存の検査

**Acceptance Criteria**:
- **AC-08**: 変更後に `make test` が通る。`docs/dev/security.md` と ADR を読む既存の文書のテスト（`TestADRRecords` を含む）が含まれる。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文書だけの変更で、実行時の処理は変わらない。

### 4.2. セキュリティ (Security)
-   本タスクは `docs/dev/security.md` そのものを直すが、外部のコマンドの呼び出し、API キーと Webhook の URL、キャッシュ、ネットワークへの送信、信頼できない文字列のプロンプトや記事への埋め込みの、どの振る舞いも変えない。このため、`docs/dev/security.md` の適用の条件に当たる新しい振る舞いはなく、信頼できない入力の受け入れの規則（受け入れる形、拒否の番兵のエラー、上限）も N/A である。
-   直した後の項目は、プロンプトインジェクションで生成結果が操作されうるという §6 の前提（`docs/dev/security.md:70`）の下で、出典のリンクが LLM の出力に依存しないという今の保証を、投稿しない記事についても明示する。保証そのものは今のコードのとおりで、強めも弱めもしない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。

### 4.4. 互換性 (Compatibility)
-   N/A。CLI・サーバ・拡張の振る舞いは変わらない。

### 4.5. 保守性 (Maintainability)
-   §6 の項目と ADR-0034 の「決定」が同じ内容を述べる。仕様の正本は §6 で、ADR-0034 はそれを要約してリンクする（`CLAUDE.md`「Design decisions (ADR)」の「Who holds what」）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR の編集上の訂正は、ADR-0039 の「決定」に従う。訂正はコミットメッセージと PR の説明に記録し、「経緯」に項目を足さない。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **出典のリンク・出典のブロック**: 記事の本文の末尾に付く、元の動画へのリンクの行（`出典: <URL>`）。ADR-0034 は「出典のブロック」と呼ぶ。
-   **ビューア**（viewer）: 記事の状態と記事を表示する拡張のページ（`viewer.html`）。用語集の定義に従う。
-   **ずれの記述**: ADR-0034 の「決定」の最後の項目にある、§6 の項目が投稿する記事に限っているように読めることを #725 で扱うとする 3 文。タスク 0065 の要件定義書 AC-03 は、同じものを「食い違い」と呼ぶ。
-   **編集上の訂正**: ADR の決定の内容を変えない直し。ADR-0039 の「決定」が定める。
