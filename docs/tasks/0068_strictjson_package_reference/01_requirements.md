# 要件定義書：internal/strictjson の使い手の一覧をコードに合わせ、一覧の置き場所を 1 か所にする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-08 |
| Review date | 2026-11-09 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`internal/strictjson` を使うパッケージの一覧（以下「使い手の一覧」）をコードに合わせ、一覧を書く場所を `docs/dev/conventions/package_reference.md` の 1 か所にし、一覧とコードのずれを `make test` で見つけられるようにするための要件を定義する。

**背景:**

`package_reference.md` の `internal/strictjson` の行は、使い手として `internal/transcript`・`internal/llm/deepseek`・`internal/llm/claude`・`internal/publisher`・`internal/writer`・`internal/artifact`・`internal/fixprguard` の 7 つを挙げる（`docs/dev/conventions/package_reference.md:29`）。しかし、`cmd/` と `internal/` の下で、テストでない Go のソースから `internal/strictjson` を import するパッケージは、2026-11-08 の `origin/main`（`56c3cbea`）では 11 個ある。一覧にない 4 つは `internal/server`・`internal/nmhost`・`internal/codexreview`・`internal/coveragecompare` である（`git grep -l '"github.com/isseis/yt2column/internal/strictjson"' -- '*.go'` の結果のうち、`_test.go` を除いたもの）。

使い手を書いた文書は、ほかにもある。

-   `CLAUDE.md` の「Architecture Overview」の `internal/strictjson` の項目は、`internal/transcript`・`internal/llm/deepseek`・`internal/llm/claude`・`internal/publisher`・`internal/writer` の 5 つだけを挙げる（`CLAUDE.md:312-315`）。
-   `docs/dev/project_overview.md` のディレクトリの構成の `internal/strictjson/` の行は、`transcript・deepseek・claude・writer・publisher が共有` と書く（`docs/dev/project_overview.md:112`）。
-   ADR-0001 の「決定」は、11 個すべてを挙げる（`docs/dev/adr/0001-llm-adapter-stdlib.md:28`）。この一覧は、タスク 0065 の PR ② が ADR-0001 から移す部分に含まれる。移し先は、0065 が A2 と呼ぶ主題（厳格な JSON の部品）の ADR である（以下「A2 の ADR」。今はブランチ `isseis/0065-adr-remaining-05` にだけあり、番号は ADR-0020）。0065 の設計書は、移し先では一覧を写さずに、要約して正本の文書へリンクすると定める（`docs/tasks/0065_adr_remaining/02_architecture.md` 3.2 と 6.2 の「決定」の項目）。

0065 の設計書は、A2 の主題の仕様の正本を `package_reference.md` の `internal/strictjson` の行とし（`docs/tasks/0065_adr_remaining/02_architecture.md` 3.2 の表の A2 の行）、上の 4 つが一覧にないことを正本の文書とコードのずれとして記録した（同 3.5 の表の A2 の行）。本 issue は、そのずれを扱うために作られた。

一覧とコードを比べるテストは、今はない。`TestPackageReferenceListsPackages` は、`cmd/` と `internal/` のパッケージのそれぞれに `package_reference.md` の行があることだけを確かめる（`internal/pipeline/pipeline_test.go:625-686`）。

対応 issue: [#713](https://github.com/isseis/yt2column/issues/713)（関連: タスク 0065 の A2。`docs/tasks/0065_adr_remaining/`）

**本書の記述範囲:** 本書は、変更の後に文書が何を述べ、`make test` がどの場合に失敗するかを定める。一覧の文の形（並べる順序、区切り）、テストを置くパッケージ、一覧とコードを読む手段は、設計（`02_architecture.md`）で決める。要件の作成で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #713 で決めたこと:**

-   **一覧を直す場所:** `package_reference.md` の `internal/strictjson` の行の使い手の一覧に、実際に使うパッケージを足す。0065 の設計書が、この行をこの主題の仕様の正本とするためである。
-   **ADR との分担:** A2 の ADR の「決定」はコードの振る舞いを書き、`package_reference.md` の一覧の更新は本タスクで行う。

**要件定義の際に決めたこと（2026-11-08）:**

-   **ほかの文書の一覧は、要約とリンクに置き換える（F-002）。** `CLAUDE.md` と `project_overview.md` の使い手の一覧は、どちらも古い。足りない分を足すと、同じ一覧が 3 か所に残り、使い手が増えるたびに 3 か所を直すことになる。一覧を消し、複数のパッケージが共有することと、一覧が `package_reference.md` にあることだけを書く。これは、0065 の設計書の「正本の一覧を書き写さず、要約してリンクする」という扱いと同じである。ADR の判断：ADR は作らない。この決定は、後のタスクにも「使い手の一覧は `package_reference.md` だけを直す」という形で及ぶ。しかし、正本の一覧を書き写さないという 0065 の設計書の既存の扱いを当てはめただけで、新しい決定ではない（CLAUDE.md「Design decisions (ADR)」）。
-   **一覧とコードのずれを `make test` で見つける（F-003）。** 一覧には、4 つのパッケージが足されないまま残っていた。文書の値をコードと比べるテストは、CLAUDE.md「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」が許す種類である。同じ節は、誤りが起きにくい確認、害が小さい確認、維持の費用が防ぐ誤りを上回る確認を足さないとする。このテストはどれにも当たらないと判断した。誤りは実際に 4 回起き、気づかれずに残った。F-002 の後は、`package_reference.md` が使い手の一覧を持つ唯一の文書になる（ただし、ADR-0001 の一覧は、タスク 0065 で移されるまで残る）。A2 の ADR も、ここを一覧の置き場所とする。そのため、一覧が古くなっても照合できる別の記述がなく、読み手は誤った一覧をそのまま信じる。維持の費用は、使い手を足すときに一覧に 1 つ足すだけである。ADR の判断：ADR は作らない。ADR-0016 の方針の中でテストを 1 つ足すもので、方針を変えないためである。
-   **A2 の ADR（ADR-0020）の古くなる文は、本タスクの実装 PR が削除する（F-004）。** ADR-0020 は #716 で `origin/main` に入った。その「決定」の「この部品を使うパッケージの一覧は」で始まる項目は、一覧に 4 つがないことを #713 で扱うと書く（`docs/dev/adr/0020-strict-json.md:23`）。本タスクが一覧を直すと、この文は古くなる。ADR はコードに合わせるので、一覧を直す PR で、その項目を最初の文（一覧は `package_reference.md` が持つ）だけにする。この削除は決定を変えない編集上の修正で、経緯の項目を足さない。あわせて、一覧とコードのずれを `make test` が見つけることを「結果」に 1 文足し、本タスクの経緯の項目を足す。照合のテストは ADR-0020 の決定から生じる新しい帰結なので、ADR を変える PR として `docs/dev/conventions/adr.md`「Approval and the PR」に収まる。タスク 0065 の最後の PR（#727）は、ADR-0020 の「決定」が #713 にリンクすることを、マージまで一度きりの確認で求める。そのため、ADR-0020 の変更は #727 のマージより前に `main` に入れない（5 章）。当初は、本タスクの PR は ADR-0020 に触れず、0065 の後続の PR が削除すると決めていた。しかし、タスク 0065 の側から、一覧を直す PR で削除するよう依頼があった（2026-11-09 に改めた）。ADR の判断：ADR は新しく作らず、ADR-0020 を上のとおり変える。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `package_reference.md` の使い手の一覧を、コードの事実に合わせる。
-   使い手の一覧を書く文書を `package_reference.md` の 1 か所にし、`CLAUDE.md` と `project_overview.md` の古い一覧をなくす。ADR-0001 の一覧はタスク 0065 の PR ② が移すので（2.3）、1 か所になるのはその PR が `origin/main` に入った後である。
-   使い手が増えたり減ったりしたのに一覧を直し忘れたとき、`make test` と CI で気づけるようにする。

### 2.2. スコープ (In Scope)

-   `docs/dev/conventions/package_reference.md` の `internal/strictjson` の行の使い手の一覧（F-001）
-   `CLAUDE.md` の「Architecture Overview」の `internal/strictjson` の項目と、`docs/dev/project_overview.md` のディレクトリの構成の `internal/strictjson/` の行（F-002）
-   使い手の一覧とコードを比べる Go のテスト（F-003）
-   `docs/dev/adr/0020-strict-json.md` の「決定」の #713 の文と、「結果」と「経緯」への追記（F-004）

### 2.3. スコープ外 (Out of Scope)

-   **ADR-0001 の使い手の一覧（`docs/dev/adr/0001-llm-adapter-stdlib.md:28`）。** 今の一覧はコードと合っている。タスク 0065 の PR ② が、この一覧を ADR-0001 から移し、移し先では一覧を写さない（1 章の背景）。本タスクで直すと、0065 の作業と重なり、ADR の決定を変えない PR が ADR を変えることになる（`docs/dev/conventions/adr.md`「Approval and the PR」）。0065 の PR ② が入るまでに使い手が増減すると、ADR-0001 の一覧は古くなり、F-003 のテストはそれを見つけない。
-   **タスクの文書（`docs/tasks/` の下）に書かれた使い手。** タスクの文書は、その時点の記録であり、今の仕様を述べる文書ではない。
-   **`package_reference.md` の `internal/strictjson` の行の、使い手の一覧のほかの記述（部品の契約の説明）。** issue が扱うのは使い手の一覧だけである。
-   **`internal/strictjson` のほかの部品の一覧（たとえば `CLAUDE.md` のほかの項目や、`package_reference.md` のほかの行の依存の記述）。** issue が扱うのは `internal/strictjson` の使い手だけである。
-   **`docs/tasks/0018_article_quality/_material_check` の import。** タスクの中の一度きりの道具で、`package_reference.md` のパッケージの表の対象（`cmd/`・`internal/`・`testutil`・`prompts`。`internal/pipeline/pipeline_test.go:625-628`）に入らない。

## 3. 機能要件 (Functional Requirements)

本章の「使い手」は、`cmd/` または `internal/` の下のパッケージのうち、`_test.go` でない Go のソースファイルが `github.com/isseis/yt2column/internal/strictjson` を import するものをいう。`internal/strictjson` 自身は使い手に含めない。`_test.go` だけが import するパッケージは使い手に含めない。ビルドタグ付きのテスト用のソース（`//go:build test` の `test_helpers.go` など）は `_test.go` でないので、import していれば使い手に数える。この定義は、`package_reference.md` のパッケージの表に行を求める既存のテストが、`_test.go` でないソースを持つパッケージを数えるのと同じ範囲である（`internal/pipeline/pipeline_test.go:625-648`）。表と一覧で、パッケージを数える範囲をそろえるためである。

### 3.1. 機能一覧

#### F-001: package_reference.md の使い手の一覧をコードに合わせる

`package_reference.md` の `internal/strictjson` の行の使い手の一覧を、実装 PR の head のコードの使い手と同じ集合にする。

**Acceptance Criteria**:
- **AC-01**: 実装 PR の head で、`package_reference.md` の `internal/strictjson` の行が使い手として挙げるパッケージの集合は、同じ head のコードの使い手の集合と等しい。2026-11-08 の `origin/main` のコードでは、1 章の背景の 7 つに `internal/server`・`internal/nmhost`・`internal/codexreview`・`internal/coveragecompare` を足した 11 個である。
- **AC-02**: `package_reference.md` の `internal/strictjson` の行のうち、使い手の一覧でない記述は、マージの基点と実装 PR の head で変わらない。`package_reference.md` のほかの行も変わらない。

#### F-002: ほかの文書の一覧を要約とリンクに置き換える

`CLAUDE.md` と `project_overview.md` は、使い手を列挙せず、一覧が `package_reference.md` にあることを示す。

**Acceptance Criteria**:
- **AC-03**: 実装 PR の head で、`CLAUDE.md` の「Architecture Overview」の `internal/strictjson` の項目は、使い手のパッケージの名前を挙げない。その項目は、複数のパッケージがこの部品を共有することと、使い手の一覧が `package_reference.md` にあることを述べ、`package_reference.md` へのリンクを持つ。部品が何を拒否するかの記述（`encoding/json` が黙って補正するバイト列と構造を拒否すること）は残る。
- **AC-04**: 実装 PR の head で、`project_overview.md` のディレクトリの構成の `internal/strictjson/` の行は、使い手のパッケージの名前を挙げず、使い手の一覧が `package_reference.md` にあることを述べる。部品の説明（検証を通った JSON 文書から値を取り出す厳格な部品であること）は残る。
- **AC-05**: AC-03 の項目と AC-04 の行のうち、使い手の記述でない部分は、マージの基点と実装 PR の head で同じ意味を述べる。`CLAUDE.md` と `project_overview.md` のほかの記述を変えるのは、F-003 のテストを足すために、設計で必要と判断した変更（たとえば CLAUDE.md「Development Notes」の guard の列挙や `doc-guard` の対象）に限る。

#### F-003: 使い手の一覧とコードのずれで make test を失敗させる

使い手の一覧とコードの使い手が食い違うとき、`make test` が失敗し、どのパッケージが食い違うかを示す。

**Acceptance Criteria**:
- **AC-06**: コードに使い手があり、そのパッケージが `package_reference.md` の使い手の一覧にないとき、`make test` は失敗し、その失敗の出力はそのパッケージの名前を示す。
- **AC-07**: `package_reference.md` の使い手の一覧が、使い手でないパッケージ（存在しないパッケージ、`_test.go` だけが import するパッケージ、`internal/strictjson` 自身など）を挙げるとき、`make test` は失敗し、その失敗の出力はそのパッケージの名前を示す。
- **AC-08**: `package_reference.md` の使い手の一覧が、同じパッケージを 2 回以上挙げるとき、`make test` は失敗し、その失敗の出力はそのパッケージの名前を示す。
- **AC-09**: `package_reference.md` に `internal/strictjson` の行がないとき、またはその行から使い手の一覧を 1 つも読み取れないとき、`make test` は成功扱いにせず失敗する。コードから使い手を 1 つも見つけられないときも、同じく失敗する。
- **AC-10**: AC-01 を満たす文書とコードでは、このテストは `make test` で成功する。
- **AC-11**: このテストは、CI の `doc-guard` ジョブでも実行される。文書だけを変える PR でも、一覧のずれが CI で見つかるためである（ADR-0016「決定」の 3 つ目の項目）。
- **AC-12**: このテストは、使い手の一覧の写しを持たない。実際のリポジトリの文書とソースを比べるとき、テストが持つパッケージの名前は、対象の部品 `internal/strictjson` の名前と import のパスだけである。失敗の経路を確かめるために作った入力（H-05）の中の名前は、これに当たらない（CLAUDE.md「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」の、テストにリテラルを写さない規則）。

一覧の文の形に合わない書き方（コードスパンでない名前など）の扱いは、一覧の形と合わせて設計で決める（design_handoff.md H-01）。

#### F-004: ADR-0020 をコードに合わせる

ADR-0020 の「決定」から、一覧が直ったことで古くなる文を消し、照合のテストの帰結を「結果」に記録する。

**Acceptance Criteria**:
- **AC-13**: 実装 PR の head で、ADR-0020 の「決定」の「この部品を使うパッケージの一覧は」で始まる項目は、その最初の文（太字の文）だけからなる。「決定」のほかの項目は、マージの基点と実装 PR の head で変わらない。
- **AC-14**: 実装 PR の head で、ADR-0020 の「結果」は、一覧とコードの使い手が食い違うと `make test` が失敗することを述べる 1 文を持つ。「経緯」の末尾には、本タスクの文書へのリンクを持つ項目が 1 つ足される（`docs/dev/conventions/adr.md`「History entries」）。ADR-0020 のほかの記述は変わらない。
- **AC-15**: 実装 PR の説明と、ADR-0020 を変えるコミットのメッセージは、AC-13 の削除を、決定を変えない修正として記録する（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」）。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A（文書とテストの変更で、製品の実行時の処理を変えない）。

### 4.2. セキュリティ (Security)

-   N/A。本タスクは、外部のコマンドの起動、API キーと Webhook URL、キャッシュディレクトリ、ネットワークへの送信、信頼できないテキストをプロンプトや記事に埋め込むことのどれにも触れない（`docs/dev/security.md` を適用する条件に当たらない）。テストが読むのは、リポジトリの中の文書とソースだけで、信頼できない入力を消費しない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   N/A（製品の振る舞いを変えない）。

### 4.4. 互換性 (Compatibility)

-   N/A（製品の振る舞い、API、設定を変えない）。

### 4.5. 保守性 (Maintainability)

-   使い手を足す、または外す変更は、同じ変更で `package_reference.md` の一覧を直せば、`make test` を通る。`CLAUDE.md` と `project_overview.md` を直す必要はない（F-002）。ADR-0001 の一覧は、タスク 0065 の PR ② が移すまで残る（2.3）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   文書のテストは、CLAUDE.md「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」と、同節の続く 3 つの規則（ADR-0016）に従う。同じ文書の同じ値を、同じ照合先（コードなど）と比べるテストを 2 つ作らない。
-   テストを足す変更は、CLAUDE.md「Testing Strategy」の「Every test must be able to fail for its stated reason」に従い、テストが対象とする誤りをコードや文書に入れ、テストが失敗することを確かめ、そのことをコミットメッセージに書く。
-   `doc-guard` の CI ジョブの対象のパッケージを変えるときは、同じ PR で CLAUDE.md「Development Notes」、`.github/workflows/ci.yml`、ADR-0016 をそろえる（ADR-0016「決定」の 3 つ目の項目）。
-   `CLAUDE.md` と `project_overview.md` の書き換えは、それぞれの文書の言語に従う（`CLAUDE.md` は英語、`project_overview.md` は日本語。CLAUDE.md「Document language」）。
-   ADR-0020 を変える PR は、タスク 0065 の PR #727 がマージされるまでマージしない。#727 の一度きりの確認が、ADR-0020 の「決定」が #713 にリンクすることを求めるためである。マージの前に `gh pr view 727 --json state` で確かめ、#727 が先にマージされない見込みのときは、ADR-0020 の変更を PR から外してレビュアーに確かめる（F-004）。

## 6. 用語集 (Glossary)

-   **部品:** 本書では、使い手が使う側の `internal/strictjson` を指す。
-   **使い手:** 3 章の冒頭で定めた、`internal/strictjson` を import するパッケージ。A2 の ADR は同じものを「利用者」と呼ぶが、翻訳用語集では「利用者」は製品の利用者（end user）を指すので、本書は「使い手」を使う。
-   **使い手の一覧:** `package_reference.md` の `internal/strictjson` の行が、使い手として挙げるパッケージの並び。
-   **正本:** ある主題の今の仕様を持つ文書（CLAUDE.md「Design decisions (ADR)」の "a document that holds the current specification"）。
-   ほかの用語は `docs/translation_glossary.md` と統一する。

## 変更履歴

- 2026-11-08 承認（isseis、#717）
- 2026-11-09 draft への戻し：ADR-0020 の #713 の文の削除と「結果」の追記を本タスクで行う（#721）
- 2026-11-09 承認（isseis、#721）
