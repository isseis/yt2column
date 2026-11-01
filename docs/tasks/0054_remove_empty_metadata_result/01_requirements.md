# 要件定義書：使われなくなった emptyMetadataResult を削除する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-01 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ブラウザ拡張の本番のコードから呼ばれなくなった関数 `emptyMetadataResult` と、その関数だけを確かめるテストを削除するための要件を定義する（[#560](https://github.com/isseis/yt2column/issues/560)）。拡張の振る舞いは変えない。

**背景:**

`emptyMetadataResult` は、タスク `0025_extension_video_metadata` で、収集が拒否されたときの動画メタデータの結果（値・促し・省略のどれもない結果）として作られた（0025 `02_architecture.md` 3.2・3.5）。タスク `0046_popup_redesign`（[#507](https://github.com/isseis/yt2column/issues/507)、PR [#559](https://github.com/isseis/yt2column/pull/559)）で、収集が拒否されたときの画面の内容（`RejectedView`）は動画メタデータを持たなくなった。その結果、この関数の本番の呼び出しはなくなった。0046 の計画は、なくすかどうかを「本タスクの後に別に決める」とした（0046 `03_implementation_plan.md` 1.4・10 章）。

現在の状態は次のとおりである。

-   定義: `extension/src/core/metadata.ts:218-220`。
-   本番の呼び出し: `extension/src` には定義のほかに `emptyMetadataResult` の名前が現れない。
-   テスト: `extension/test/metadata.test.ts:6`（import）と、`extension/test/metadata.test.ts:337-345` の `describe("emptyMetadataResult")` だけが使う。
-   後続の [#503](https://github.com/isseis/yt2column/issues/503)（Viewer）の生成中などの状態で使う見込みはない（0046 `03_implementation_plan.md` 1.4）。

動画メタデータの結果を表す手段は、ほかに 2 つあり、どちらも本番で使われている。

-   `acceptMetadata`（`extension/src/core/metadata.ts:174`）: 読み取りに成功したページの値を受け入れる。値が 1 つもないページ（`{ descriptionCollapsed: false }`）を渡せば、値と促しのない結果になる。本番の呼び出しは `extension/src/launch.ts:112`。
-   `readFailureResult`（`extension/src/core/metadata.ts:208`）: 読み取りに失敗したときの結果。本番の呼び出しは `extension/src/launch.ts:99`・`:105`。

対応 issue: [#560](https://github.com/isseis/yt2column/issues/560)（関連: [#507](https://github.com/isseis/yt2column/issues/507)・PR [#559](https://github.com/isseis/yt2column/pull/559)・[#503](https://github.com/isseis/yt2column/issues/503)）

**本書の記述範囲:** 本書は、何を削除し、何を変えないか、削除の後に何を確かめるかを定める。テストの網羅の報告の取り方と比べ方は、設計（`02_architecture.md`）で決める。要件の作成で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #560 で決めたこと:**

-   **削除する:** `emptyMetadataResult` を削除する。本番の呼び出しがなく、#503 で使う見込みもないためである。
-   **テストも削除する:** `emptyMetadataResult` だけを確かめるテストを削除する。確かめる対象がなくなるためである。
-   **テストの削除を確かめる:** テストを削除した後、拡張の `node --test --experimental-test-coverage` の出力で、ほかの `src/` の関数の実行回数が 0 にならないことを確かめる。[CLAUDE.md](../../../CLAUDE.md)「Deleting a test is a claim that must be checked」は `go tool cover -func` で確かめることを求めるが、拡張には `go tool cover` がないため、拡張での読み替えとして node の網羅の報告を使う（0046 `03_implementation_plan.md` 1 章と同じ読み替え）。
-   **代わりの手段を確かめる:** 収集に成功した入力にメタデータがない状態は `acceptMetadata({ descriptionCollapsed: false })` で、読み取りの失敗は `readFailureResult()` で表せる。どちらも本番で使われていることを確かめる。

**要件定義の際に決めたこと（2026-11-01）:**

-   **対象は `emptyMetadataResult` だけにする:** 拡張には、定義したファイルの外の `src/` から使われていない export がほかにもある。その多くは、進行中のタスク `0050_viewer_tab` の部品（`src/core/response.ts` の `classifyStart` など、後続の PR で使う予定のもの）か、定義したファイルの中で使われている型である。issue #560 が対象とするのは `emptyMetadataResult` だけで、ほかの export の要否は、それぞれのタスクの事情で決まるためである（2.3）。
-   **過去のタスク文書は変えない:** 0025・0046 のタスク文書は `emptyMetadataResult` を名前で挙げている。これらは各タスクの時点の決定の記録であり、後のタスクで部品が変わっても書き換えないためである（2.3）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   本番で使われない関数とそのテストをなくし、拡張の保守の対象を減らす。
-   拡張の振る舞い（ポップアップの表示、ログ、送る内容）を変えない。
-   テストの削除で、ほかの関数の確認が失われていないことを確かめる。

### 2.2. スコープ (In Scope)

-   `extension/src/core/metadata.ts` の `emptyMetadataResult` の削除。
-   `extension/test/metadata.test.ts` の `emptyMetadataResult` の import と、`describe("emptyMetadataResult")` の削除。
-   削除の前と後の、拡張のテストの網羅の報告の比較。
-   代わりの手段（`acceptMetadata`・`readFailureResult`）に本番の呼び出しがあることの確認。

### 2.3. スコープ外 (Out of Scope)

-   `emptyMetadataResult` 以外の、使われていない export の削除。理由は 1 章「要件定義の際に決めたこと」のとおり。
-   `MetadataResult` などの型、`acceptMetadata`・`readFailureResult` の振る舞いとテストの変更。本タスクは削除だけを行う。
-   0025・0046 などの過去のタスク文書の書き換え。理由は 1 章「要件定義の際に決めたこと」のとおり。
-   Go のコードとサーバ・Native Messaging のホストの変更。`emptyMetadataResult` は拡張の中だけの関数である。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: emptyMetadataResult とそのテストの削除

`emptyMetadataResult` の定義と、それだけを確かめるテストを削除する。拡張の振る舞いは変えない。

**Acceptance Criteria**:
- **AC-01**: `extension/src` と `extension/test` の追跡されているファイルに、`emptyMetadataResult` という名前が現れない。
- **AC-02**: `extension/src/core/metadata.ts` から削除されるのは `emptyMetadataResult` の定義とその説明のコメントだけで、ほかの export の名前・型・振る舞いは変わらない。
- **AC-03**: `extension/test/metadata.test.ts` から削除されるのは `emptyMetadataResult` の import と `describe("emptyMetadataResult")` だけで、ほかのテストの入力と期待する値は変わらない。
- **AC-04**: 削除の後、`make ext-check`（型の検査、lint、書式の検査、テスト、ビルド）が成功する。

#### F-002: テストの削除の確認

テストを削除しても、ほかの `src/` の関数の確認が失われていないことを確かめる。

**Acceptance Criteria**:
- **AC-05**: 削除の前と後で拡張のテストの網羅の報告を取り、`emptyMetadataResult` を除く `src/` の各関数について、削除の前に実行回数が 1 以上だった関数は、削除の後も実行回数が 1 以上である。
- **AC-06**: AC-05 の比較に使ったコマンド、比べた 2 つの時点、結果が、その比較を行ったコミットのメッセージに書かれている。

#### F-003: 代わりの手段の確認

`emptyMetadataResult` がなくても、動画メタデータの結果の 2 つの状態を本番で使われている関数で表せることを確かめる。

**Acceptance Criteria**:
- **AC-07**: 削除の後、`acceptMetadata` と `readFailureResult` のそれぞれに、`extension/src` の中で定義のほかに少なくとも 1 か所の呼び出しがある。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A（実行される処理を変えない）。

### 4.2. セキュリティ (Security)
-   N/A。本タスクは外部コマンドの呼び出し、API キー・Webhook URL、キャッシュ、ネットワークでの送信、信頼できない文字列の埋め込みのどれも変えない（条件付きのガイドの適用条件に当たらない）。信頼できない入力を新たに受け取らない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A（実行される処理を変えない）。

### 4.4. 互換性 (Compatibility)
-   拡張の利用者に見える振る舞い（ポップアップの表示、送る内容）と開発者向けのログは変わらない（AC-02〜AC-04）。

### 4.5. 保守性 (Maintainability)
-   本番で使われない関数とそのテストがなくなる（F-001）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   [CLAUDE.md](../../../CLAUDE.md)「Deleting a test is a claim that must be checked」に従い、テストの削除を網羅の報告で確かめる。拡張での読み替えは 1 章「issue #560 で決めたこと」のとおり。
-   拡張に依存パッケージを加えない（[CLAUDE.md](../../../CLAUDE.md)「Dependencies」）。

## 6. 用語集 (Glossary)

-   **動画メタデータ:** 拡張が動画ページから読む動画タイトル・チャンネル名・概要欄（0025 の要件定義書と同じ）。
-   **網羅の報告:** node のテストランナーが `--experimental-test-coverage` で出す、ファイル・関数ごとの実行の記録。
-   **実行回数:** 網羅の報告の中で、関数ごとに記録される、テストの実行中にその関数が呼ばれた回数。
