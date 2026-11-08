# 要件定義書：internal/strictjson の使い手の一覧をコードに合わせ、一覧の置き場所を 1 か所にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-08 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`internal/strictjson` を使うパッケージの一覧（以下「使い手の一覧」）をコードに合わせ、一覧を書く場所を `docs/dev/conventions/package_reference.md` の 1 か所にし、一覧とコードのずれを `make test` で見つけられるようにするための要件を定義する。

**背景:**

`package_reference.md` の `internal/strictjson` の行は、使い手として `internal/transcript`・`internal/llm/deepseek`・`internal/llm/claude`・`internal/publisher`・`internal/writer`・`internal/artifact`・`internal/fixprguard` の 7 つを挙げる（`docs/dev/conventions/package_reference.md:29`）。しかし、`cmd/` と `internal/` の下で、テストでない Go のソースから `internal/strictjson` を import するパッケージは、2026-11-08 の `origin/main`（`56c3cbea`）では 11 個ある。一覧にない 4 つは `internal/server`・`internal/nmhost`・`internal/codexreview`・`internal/coveragecompare` である（`git grep -l '"github.com/isseis/yt2column/internal/strictjson"' -- '*.go'` の結果のうち、`_test.go` を除いたもの）。

使い手を書いた文書は、ほかにもある。

-   `CLAUDE.md` の「Architecture Overview」の `internal/strictjson` の項目は、`internal/transcript`・`internal/llm/deepseek`・`internal/llm/claude`・`internal/publisher`・`internal/writer` の 5 つだけを挙げる（`CLAUDE.md:312-315`）。
-   `docs/dev/project_overview.md` のディレクトリの構成の `internal/strictjson/` の行は、`transcript・deepseek・claude・writer・publisher が共有` と書く（`docs/dev/project_overview.md:112`）。
-   ADR-0001 の「決定」は、11 個すべてを挙げる（`docs/dev/adr/0001-llm-adapter-stdlib.md:28`）。この一覧は、タスク 0065 の PR ② で、ADR-0001 から A2 の ADR（ADR-0020）へ移す部分に含まれる。0065 の設計書は、移し先では一覧を写さずに、要約して正本の文書へリンクすると定める（`docs/tasks/0065_adr_remaining/02_architecture.md` 3.2 と 6.2 の「決定」の項目）。

0065 の設計書は、A2 の主題の仕様の正本を `package_reference.md` の `internal/strictjson` の行とし（`docs/tasks/0065_adr_remaining/02_architecture.md` 3.2 の表の A2 の行）、上の 4 つが一覧にないことを正本の文書とコードのずれとして記録した（同 3.5 の表の A2 の行）。本 issue は、そのずれを扱うために作られた。

一覧とコードを比べるテストは、今はない。`TestPackageReferenceListsPackages` は、`cmd/` と `internal/` のパッケージのそれぞれに `package_reference.md` の行があることだけを確かめる（`internal/pipeline/pipeline_test.go:625-686`）。

対応 issue: [#713](https://github.com/isseis/yt2column/issues/713)（関連: タスク 0065 の A2。`docs/tasks/0065_adr_remaining/`）

**本書の記述範囲:** 本書は、変更の後に文書が何を述べ、`make test` がどの場合に失敗するかを定める。一覧の文の形（並べる順序、区切り）、テストを置くパッケージ、一覧とコードを読む手段は、設計（`02_architecture.md`）で決める。要件の作成で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #713 で決めたこと:**

-   **一覧を直す場所:** `package_reference.md` の `internal/strictjson` の行の使い手の一覧に、実際に使うパッケージを足す。0065 の設計書が、この行をこの主題の仕様の正本とするためである。
-   **ADR-0020 との分担:** ADR-0020 の「決定」はコードの振る舞いを書き、`package_reference.md` の一覧の更新は本タスクで行う。

**要件定義の際に決めたこと（2026-11-08）:**

-   **ほかの文書の一覧は、要約とリンクに置き換える（F-002）。** `CLAUDE.md` と `project_overview.md` の使い手の一覧は、どちらも古い。足りない分を足すと、同じ一覧が 3 か所に残り、使い手が増えるたびに 3 か所を直すことになる。一覧を消し、複数のパッケージが共有することと、一覧が `package_reference.md` にあることだけを書く。これは、0065 の設計書の「正本の一覧を書き写さず、要約してリンクする」という扱いと同じである。ADR の判断：ADR は作らない。文書の記述の置き場所の決定で、本タスクの中で閉じ、ほかのタスクや部品を縛らないためである（CLAUDE.md「Design decisions (ADR)」）。
-   **一覧とコードのずれを `make test` で見つける（F-003）。** 一覧には、4 つのパッケージが足されないまま残っていた。本タスクで一覧を正本にするので、使い手が増えたときに一覧の更新を忘れても、テストが失敗して気づけるようにする。文書の値をコードと比べるテストは、CLAUDE.md「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」が許す種類である。ずれの害は小さい（古い文書の記述が 1 行残る）が、ずれは実際に起き、気づかれずに残った。テストの維持の費用は小さい（使い手を足すときに一覧に 1 語足すだけ）と判断した。ADR の判断：ADR は作らない。ADR-0016 の方針の中でテストを 1 つ足すもので、方針を変えないためである。
-   **ADR-0020 の古くなる文は、後からマージする側が直す（F-004）。** ADR-0020 は、タスク 0065 のブランチ（`isseis/0065-adr-remaining-05`）にだけあり、2026-11-08 の時点で `origin/main` にない。その「決定」は、一覧に 4 つがないことを #713 で扱うと書く。本タスクが一覧を直すと、この文は古くなる。本タスクの実装 PR をマージする時点で ADR-0020 が `origin/main` にあれば、その PR が直す。なければ、0065 が、ずれのない形で ADR-0020 を書く。ADR の判断：ADR-0020 の「決定」の事実の記述を直すだけで、決定は変えないので、新しい ADR は作らない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `package_reference.md` の使い手の一覧を、コードの事実に合わせる。
-   使い手の一覧を書く文書を `package_reference.md` の 1 か所にし、ほかの文書の古い一覧をなくす。
-   使い手が増えたり減ったりしたのに一覧を直し忘れたとき、`make test` と CI で気づけるようにする。

### 2.2. スコープ (In Scope)

-   `docs/dev/conventions/package_reference.md` の `internal/strictjson` の行の使い手の一覧（F-001）
-   `CLAUDE.md` の「Architecture Overview」の `internal/strictjson` の項目と、`docs/dev/project_overview.md` のディレクトリの構成の `internal/strictjson/` の行（F-002）
-   使い手の一覧とコードを比べる Go のテスト（F-003）
-   ADR-0020 が `origin/main` にあるときの、その「決定」のずれの記述（F-004）

### 2.3. スコープ外 (Out of Scope)

-   **ADR-0001 の使い手の一覧（`docs/dev/adr/0001-llm-adapter-stdlib.md:28`）。** 今の一覧はコードと合っている。タスク 0065 の PR ② が、この一覧を ADR-0001 から移し、移し先では一覧を写さない（1 章の背景）。本タスクで直すと、0065 の作業と重なる。
-   **タスクの文書（`docs/tasks/` の下）に書かれた使い手。** タスクの文書は、その時点の記録であり、今の仕様を述べる文書ではない。
-   **`package_reference.md` の `internal/strictjson` の行の、使い手の一覧のほかの記述（部品の契約の説明）。** issue が扱うのは使い手の一覧だけである。
-   **`internal/strictjson` のほかの部品の一覧（たとえば `CLAUDE.md` のほかの項目や、`package_reference.md` のほかの行の依存の記述）。** issue が扱うのは `internal/strictjson` の使い手だけである。
-   **`docs/tasks/0018_article_quality/_material_check` の import。** タスクの中の一度きりの道具で、`package_reference.md` のパッケージの表の対象（`cmd/`・`internal/`・`testutil`・`prompts`。`internal/pipeline/pipeline_test.go:625-628`）に入らない。

## 3. 機能要件 (Functional Requirements)

本章の「使い手」は、`cmd/` または `internal/` の下のパッケージのうち、`_test.go` でない Go のソースファイルが `github.com/isseis/yt2column/internal/strictjson` を import するものをいう。`internal/strictjson` 自身は使い手に含めない。`_test.go` だけが import するパッケージは使い手に含めない。

### 3.1. 機能一覧

#### F-001: package_reference.md の使い手の一覧をコードに合わせる

`package_reference.md` の `internal/strictjson` の行の使い手の一覧を、実装 PR の基点のコードの使い手と同じ集合にする。

**Acceptance Criteria**:
- **AC-01**: 実装 PR の head で、`package_reference.md` の `internal/strictjson` の行が使い手として挙げるパッケージの集合は、同じ head のコードの使い手の集合と等しい。2026-11-08 の `origin/main` のコードでは、1 章の背景の 7 つに `internal/server`・`internal/nmhost`・`internal/codexreview`・`internal/coveragecompare` を足した 11 個である。
- **AC-02**: `package_reference.md` の `internal/strictjson` の行のうち、使い手の一覧でない記述は、マージの基点と実装 PR の head で変わらない。`package_reference.md` のほかの行も変わらない。

#### F-002: ほかの文書の一覧を要約とリンクに置き換える

`CLAUDE.md` と `project_overview.md` は、使い手を列挙せず、一覧が `package_reference.md` にあることを示す。

**Acceptance Criteria**:
- **AC-03**: 実装 PR の head で、`CLAUDE.md` の「Architecture Overview」の `internal/strictjson` の項目は、使い手のパッケージの名前を挙げない。その項目は、複数のパッケージがこの部品を共有することと、使い手の一覧が `package_reference.md` にあることを述べ、`package_reference.md` へのリンクを持つ。部品が何を拒否するかの記述（`encoding/json` が黙って補正するバイト列と構造を拒否すること）は残る。
- **AC-04**: 実装 PR の head で、`project_overview.md` のディレクトリの構成の `internal/strictjson/` の行は、使い手のパッケージの名前を挙げず、使い手の一覧が `package_reference.md` にあることを述べる。部品の説明（検証を通った JSON 文書から値を取り出す厳格な部品であること）は残る。
- **AC-05**: `CLAUDE.md` と `project_overview.md` の、AC-03・AC-04 の項目と行のほかの部分は、マージの基点と実装 PR の head で変わらない。

#### F-003: 使い手の一覧とコードのずれで make test を失敗させる

使い手の一覧とコードの使い手が食い違うとき、`make test` が失敗し、どのパッケージが食い違うかを示す。

**Acceptance Criteria**:
- **AC-06**: コードに使い手があり、そのパッケージが `package_reference.md` の使い手の一覧にないとき、`make test` は失敗し、その失敗の出力はそのパッケージの名前を示す。
- **AC-07**: `package_reference.md` の使い手の一覧が、使い手でないパッケージ（存在しないパッケージ、`_test.go` だけが import するパッケージ、`internal/strictjson` 自身を含む）を挙げるとき、`make test` は失敗し、その失敗の出力はそのパッケージの名前を示す。
- **AC-08**: `package_reference.md` に `internal/strictjson` の行がないとき、またはその行から使い手の一覧を 1 つも読み取れないとき、`make test` は成功せずに失敗する。コードから使い手を 1 つも見つけられないときも、同じく失敗する。
- **AC-09**: AC-01 を満たす文書とコードでは、このテストは `make test` で成功する。
- **AC-10**: このテストは、CI の `doc-guard` ジョブでも実行される。文書だけを変える PR でも、一覧のずれが CI で見つかるためである（ADR-0016「決定」の 3 つ目の項目）。
- **AC-11**: このテストの使い手の一覧は文書から読み、コードの使い手はリポジトリのソースから求める。どちらも、パッケージの名前をテストの中に書き写した値と比べない（CLAUDE.md「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」の、テストにリテラルを写さない規則）。

#### F-004: ADR-0020 のずれの記述を直す

本タスクが一覧を直すと、ADR-0020 の「決定」の、一覧に 4 つがないという記述は事実に合わなくなる。

**Acceptance Criteria**:
- **AC-12**: 本タスクの実装 PR をマージする時点で、`origin/main` に `docs/dev/adr/0020-strict-json.md` があるとき、マージする PR の head の ADR-0020 の「決定」は、`package_reference.md` の一覧にない使い手があるとは述べない。使い手の一覧を `package_reference.md` が持つという記述は残る。ADR-0020 の変更は、ADR の規約（`docs/dev/conventions/adr.md`）に従う。
- **AC-13**: AC-12 の時点で、`origin/main` に `docs/dev/adr/0020-strict-json.md` がないとき、実装 PR は ADR-0020 を作らず、PR の説明に、ADR-0020 を `origin/main` に入れる側（タスク 0065）が、ずれの記述のない形で ADR-0020 を書く必要があることを書く。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A（文書とテストの変更で、製品の実行時の処理を変えない）。

### 4.2. セキュリティ (Security)

-   N/A。本タスクは、外部のコマンドの起動、API キーと Webhook URL、キャッシュディレクトリ、ネットワークへの送信、信頼できないテキストのプロンプトや記事への埋め込みのどれにも触れない（`docs/dev/security.md` を適用する条件に当たらない）。テストが読むのは、リポジトリの中の文書とソースだけである。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   N/A（製品の振る舞いを変えない）。

### 4.4. 互換性 (Compatibility)

-   N/A（製品の振る舞い、API、設定を変えない）。

### 4.5. 保守性 (Maintainability)

-   使い手を足す、または外す変更は、同じ変更で `package_reference.md` の一覧を直せば、`make test` を通る。ほかの文書を直す必要はない（F-002）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   文書のテストは、CLAUDE.md「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」と、その後の 3 つの規則（ADR-0016）に従う。同じ文書の同じ値を、同じ元と比べるテストを 2 つ作らない。
-   テストを足す変更は、CLAUDE.md「Testing Strategy」の「Every test must be able to fail for its stated reason」に従い、テストが対象とする誤りを入れて失敗することを確かめ、そのことをコミットメッセージに書く。
-   `CLAUDE.md` と `project_overview.md` の書き換えは、それぞれの文書の言語に従う（`CLAUDE.md` は英語、`project_overview.md` は日本語。CLAUDE.md「Document language」）。

## 6. 用語集 (Glossary)

-   **使い手:** 3 章の冒頭で定めた、`internal/strictjson` を import するパッケージ。
-   **使い手の一覧:** `package_reference.md` の `internal/strictjson` の行が、使い手として挙げるパッケージの並び。
-   **正本:** ある主題の今の仕様を持つ文書（CLAUDE.md「Design decisions (ADR)」の "a document that holds the current specification"）。
-   ほかの用語は `docs/translation_glossary.md` と統一する。
