# 要件定義書：使われなくなった emptyMetadataResult を削除する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-01 |
| Review date | 2026-11-01 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ブラウザ拡張の本番のコードから呼ばれなくなった関数 `emptyMetadataResult` と、その関数だけを確かめるテストを削除するための要件を定義する（[#560](https://github.com/isseis/yt2column/issues/560)）。拡張の振る舞いは変えない。

**背景:**

`emptyMetadataResult` は、タスク `0025_extension_video_metadata` で、収集が拒否されてページを読まなかったときの動画メタデータの結果として作られた（0025 `02_architecture.md` 3.2・3.5）。この関数は、値も促しのメッセージもなく、省略の一覧（値を渡さない項目と、その理由の一覧）も空である結果を返す（`extension/src/core/metadata.ts:218-220`）。

タスク `0046_popup_redesign`（[#507](https://github.com/isseis/yt2column/issues/507)、PR [#559](https://github.com/isseis/yt2column/pull/559)）で、収集が拒否されたときの画面の内容（`RejectedView`）は収集の拒否の理由だけを持つ形になり、動画メタデータを持たなくなった（`extension/src/core/popupView.ts:100-103`）。そのため「収集が拒否されてページを読まなかった」状態を動画メタデータの結果で表す必要がなくなり、この関数の本番の呼び出しもなくなった。0046 の計画は、なくすかどうかを「本タスクの後に別に決める」とした（0046 `03_implementation_plan.md` 1.4・10 章）。

現在の状態は次のとおりである。

-   定義: `extension/src/core/metadata.ts:218-220`。
-   本番の呼び出し: なし。`git grep -n emptyMetadataResult -- extension/src` が返すのは定義の行（`extension/src/core/metadata.ts:218`）だけである。
-   テスト: `extension/test/metadata.test.ts:6`（import）と、`extension/test/metadata.test.ts:337-345` の `describe("emptyMetadataResult")` だけが使う。この `describe` が呼ぶ関数は `emptyMetadataResult` だけである。
-   後続の [#503](https://github.com/isseis/yt2column/issues/503)（Viewer）でも、生成中などの状態の表示に使う見込みはない（0046 `03_implementation_plan.md` 1.4）。

issue #560 は、動画メタデータの結果を作る既存の関数として次の 2 つを挙げる。いずれも本番で使われている。ただし、どちらも `emptyMetadataResult` と同じ結果は返さない。それぞれ別の状態を表し、省略の一覧は空にならない。

-   `acceptMetadata`（`extension/src/core/metadata.ts:174`）: 読み取りに成功したページの値を受け入れる。値が 1 つもないページ（`{ descriptionCollapsed: false }`）を渡すと、値と促しのメッセージのない結果になり、3 つの項目（動画タイトル・チャンネル名・概要欄）がどれも `absent` として省略の一覧に入る。本番の呼び出しは `extension/src/launch.ts:112`。
-   `readFailureResult`（`extension/src/core/metadata.ts:208`）: 読み取りに失敗したときの結果。3 つの項目がどれも `read-failure` として省略の一覧に入る。本番の呼び出しは `extension/src/launch.ts:99`・`:105`。

対応 issue: [#560](https://github.com/isseis/yt2column/issues/560)（関連: [#507](https://github.com/isseis/yt2column/issues/507)・PR [#559](https://github.com/isseis/yt2column/pull/559)・[#503](https://github.com/isseis/yt2column/issues/503)）

**本書の記述範囲:** 本書は、何を削除し、何を変えないか、削除の後に何を確かめるかを定める。カバレッジの報告の取り方と比べ方は、設計（`02_architecture.md`）で決める。要件の作成で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #560 で決めたこと:**

-   **削除する:** `emptyMetadataResult` を削除する。本番の呼び出しがなく、#503 で使う見込みもないためである。
-   **テストも削除する:** `emptyMetadataResult` だけを確かめるテストを削除する。確かめる対象がなくなるためである。
-   **テストの削除を確かめる:** テストを削除した後、拡張の `node --test --experimental-test-coverage` の出力で、ほかの `src/` の関数の実行回数が 0 にならないことを確かめる（F-002）。これは [CLAUDE.md](../../../CLAUDE.md)「Deleting a test is a claim that must be checked」の例外である（下の「方針の例外」）。
-   **既存の手段を確かめる:** issue #560 は、収集に成功した入力にメタデータがない状態は `acceptMetadata({ descriptionCollapsed: false })` で、読み取りの失敗は `readFailureResult()` で表せることを確かめるとした。どちらも本番で使われていることは、背景に挙げた呼び出しの場所で確かめた。

**方針の例外:**

-   **方針:** [CLAUDE.md](../../../CLAUDE.md)「Testing Strategy」の「Deleting a test is a claim that must be checked」は、テストを削除した後に `go tool cover -func` の結果が関数ごとに変わらないことを確かめるよう求める。
-   **例外の内容:** 本タスクでは、拡張の node のカバレッジの報告で、`emptyMetadataResult` を除く `src/` の各関数の実行回数が、削除の前に 1 以上なら削除の後も 1 以上であることを確かめる（AC-05）。比べるのは関数の実行回数が 0 になるかどうかであり、回数が変わらないことは求めない。行と分岐は比べない。
-   **理由:** 拡張には `go tool cover` がない。issue #560 が「0 にならないこと」を確かめる形を決めた。0046 も同じ読み替えをした（0046 `03_implementation_plan.md` 1.2）。削除する `describe` は `emptyMetadataResult` だけを呼ぶので、ほかの関数の行と分岐はこの `describe` に依存しない。0027 は行と分岐まで比べた（0027 `02_architecture.md` 3.11）が、本タスクでは関数の比較で足りる。

**要件定義の際に決めたこと（2026-11-01）:**

-   **対象は `emptyMetadataResult` だけにする:** 拡張には、定義されたファイル以外の `src/` のどこからも使われていない export がほかにもある。たとえば、進行中のタスク `0050_viewer_tab` が加えた `classifyStart`（`extension/src/core/response.ts:559`）は、後続の PR で使う予定の関数である。issue #560 が対象とするのは `emptyMetadataResult` だけで、ほかの export の要否は、それぞれのタスクの事情で決まるためである（2.3）。
-   **過去のタスク文書は変えない:** 0025・0046 のタスク文書は `emptyMetadataResult` を名前で挙げている。これらは各タスクの時点の決定の記録であり、後のタスクで関数が変わっても書き換えないためである（2.3）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   本番で使われない関数とそのテストをなくし、拡張の保守の対象を減らす。
-   拡張の振る舞い（ポップアップの表示、ログ、送る内容）を変えない。
-   テストの削除で、ほかの関数の確認が失われていないことを確かめる。

### 2.2. スコープ (In Scope)

-   `extension/src/core/metadata.ts` の `emptyMetadataResult` の削除。
-   `extension/test/metadata.test.ts` の `emptyMetadataResult` の import と、`describe("emptyMetadataResult")` の削除。
-   削除の前と後の、拡張のテストのカバレッジの報告の比較。

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
- **AC-02**: `extension/src` の変更は `extension/src/core/metadata.ts` の `emptyMetadataResult` の定義、その説明のコメント、その直前の空行の削除だけで、ほかの行の内容は変わらない。
- **AC-03**: `extension/test` の変更は `extension/test/metadata.test.ts` の `emptyMetadataResult` の import と、`describe("emptyMetadataResult")` とその直前の空行の削除だけで、ほかの行の内容は変わらない。
- **AC-04**: 削除の後、`make ext-check`（型の検査、lint、書式の検査、テスト、ビルド）が成功する。

#### F-002: テストの削除の確認

テストを削除しても、ほかの `src/` の関数の確認が失われていないことを確かめる。

**Acceptance Criteria**:
- **AC-05**: 削除の前と後で拡張のテストのカバレッジの報告を取り、`emptyMetadataResult` を除く `src/` の各関数について、削除の前に実行回数が 1 以上だった関数は、削除の後も実行回数が 1 以上である。満たさない関数があれば、本タスクの変更を PR に出さず、その関数とともにレビュアーに報告する。
- **AC-06**: AC-05 の比較に使ったコマンド、比べた 2 つの時点、結果が、テストを削除するコミットのメッセージに書かれている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A（実行される処理を変えない）。

### 4.2. セキュリティ (Security)
-   N/A。本タスクは外部コマンドの呼び出し、API キー・Webhook URL、キャッシュ、ネットワークでの送信、信頼できない文字列の埋め込みのどれも変えない（`docs/dev/security.md` を適用する条件に当たらない）。信頼できない入力を新たに受け取らない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A（実行される処理を変えない）。利用者に見えるエラーの振る舞いも変わらない。

### 4.4. 互換性 (Compatibility)
-   拡張の利用者に見える振る舞い（ポップアップの表示、送る内容）と開発者向けのログは変わらない。`extension/src` で変わるのは呼ばれていない関数の削除だけで（AC-02）、`extension/src/launch.ts` などの呼び出し側は変わらないためである。

### 4.5. 保守性 (Maintainability)
-   本番で使われない関数とそのテストがなくなる（F-001）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   テストの削除は、1 章「方針の例外」のとおりカバレッジの報告で確かめる。
-   拡張に依存パッケージを加えない（[CLAUDE.md](../../../CLAUDE.md)「Dependencies」）。

## 6. 用語集 (Glossary)

-   **動画メタデータ:** 拡張が動画ページから読む動画タイトル・チャンネル名・概要欄（0025 の要件定義書と同じ）。
-   **促しのメッセージ:** `docs/translation_glossary.md` のとおり。
-   **カバレッジの報告:** node のテストランナーが `--experimental-test-coverage` で出す、ファイル・関数ごとの実行の記録（0027 と同じ用語）。
-   **実行回数:** カバレッジの報告の中で、関数ごとに記録される、テストの実行中にその関数が呼ばれた回数。
