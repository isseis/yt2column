# 要件定義書：PR のタイトルを日本語にし、squash のコミットは英語のままにする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-21 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、PR のタイトルを日本語にし、squash のコミットのメッセージ（件名と本文）は英語のままにするための要件を定義する（[#408](https://github.com/isseis/yt2column/issues/408)）。対象は、PR のタイトルを書くコマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2`・`/runplan`・`/runpr`・`/fixpr`）、squash のコミットのメッセージを書く `/mergepr`、言語の規則を定める `CLAUDE.md`、PR マーカーの規約を定める `.claude/commands/_context.md` である。

**背景:**

このリポジトリでは、GitHub に置くもの（PR の本文、issue、レビューへの返信、PR のコメント）を日本語で書く。PR のタイトルだけが例外で、英語の conventional-commit 形式にしている。理由は、squash のコミットの件名を PR のタイトルから作るためである（`CLAUDE.md:49-52`）。

PR のタイトルは、今は次のように決まる。

-   **タスクの文書の PR:** `/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2` は、文書を追加したコミットの件名を PR のタイトルにする（`.claude/commands/_lib/task-pr-handoff.md:35-37`）。タイトルは英語とされている（`_lib/task-pr-handoff.md:100-101`）。コミットの件名は、たとえば `docs(NNNN): draft requirements for <concise English summary>` である（`mkreq.md:106-107`、`mkarch.md:76`）。
-   **プランの PR:** `/runplan` は、PR マーカーの `推奨タイトル` を PR のタイトルにする（`runplan.md:91`）。`推奨タイトル` は `feat(<task-id>): <concise English title>` の形の英語である（`_context.md:62`、`mkplan2.md:62`・`mkplan2.md:80`・`mkplan2.md:129`）。
-   **単独の変更の PR:** `/runpr` は、英語の conventional-commit 形式のタイトルを書く（`runpr.md:85-87`）。
-   **PR の説明の更新:** `/fixpr` の Phase 6 は、PR の説明が大きく古くなったときにタイトルを書き直す。タイトルは英語の conventional-commit 形式に保つ（`fixpr.md:466`）。

すべての PR は squash マージする（`_context.md:51`）。`/mergepr` は、squash のコミットの件名を `<type>(<scope>): <summary> (#<number>)` の形の英語で書く。PR のタイトルがその形に合っていれば、それをそのまま使う（`mergepr.md:49-52`）。`mergepr` のバイナリは、件名が空でない 1 行であることだけを確かめる（`internal/mergepr/mergepr.go:441-450`）。PR のタイトルは、`prepare` の出力に表示するだけである（`cmd/mergepr/main.go:48`）。

GitHub に置くものの原則を日本語に移したので、PR のタイトルも日本語にしたい。一方、squash のコミットは `main` に残る唯一のコミットであり（`_context.md:51`）、リポジトリに残るものは英語で書く（`CLAUDE.md:44-46`）。そのため、squash のコミットのメッセージは英語のままにする。

issue が確認を求めた、文書のガードのテストについて調べた結果は次のとおりである（2026-10-21）。PR のタイトルの形に依存するものはない。

-   `CLAUDE.md` を読むガードのテスト（`cmd/yt2column/docs_test.go`・`cmd/yt2column/copied_transcript_docs_test.go`・`cmd/yt2column/extension_docs_test.go`・`cmd/yt2column-server/docs_test.go`・`internal/pipeline/relay_guard_test.go`）は、「Language」の節を検査しない。
-   `.claude/commands/` を読むテストはない。
-   CI（`.github/workflows/ci.yml`・`.github/workflows/deepseek-review.yml`）は、PR のタイトルを検査しない。

対応 issue: [#408](https://github.com/isseis/yt2column/issues/408)（関連: [#407](https://github.com/isseis/yt2column/pull/407)。タスク 0031 の作業中に起票した。タスク 0031 とは独立している）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。各コマンドが書く PR のタイトルの言語と形、`/mergepr` が書く件名の言語と形、規則と規約の記述である。次の事項は、設計（`02_architecture.md`）で決める。

-   タイトルの形を、どの文書に 1 か所で定め、各コマンドからどう参照するか。
-   `_lib/task-pr-handoff.md` の PR_TEXT の定義を、どう書き換えるか。
-   各コマンドの定義のどこに、何を書くか。

要件の起草で挙がった設計の事項は、[design_handoff.md](design_handoff.md) に申し送る。

**issue #408 で決めたこと:**

-   **PR のタイトルを日本語にする:** GitHub に置くものの原則を日本語に移したので、PR のタイトルだけを例外にする理由がなくなるためである。
-   **squash のコミットの件名と本文は、当面は英語のままにする:** リポジトリに残るものは英語で書く原則（`CLAUDE.md:44-46`）を保つためである。
-   **`/mergepr` は、PR のタイトルをそのまま件名に使うのをやめ、PR の内容から英語の件名を書く:** PR のタイトルが日本語になると、件名に使えなくなるためである。

**要件定義の際に決めたこと（2026-10-21）:**

-   **PR のタイトルに conventional-commit の接頭辞を残す:** タイトルは `<type>(<scope>): <日本語の要約>` の形にする（F-001・F-002）。PR の一覧で、変更の種類とタスクが分かるためである。また、`/mergepr` が英語の件名の `<type>(<scope>)` を決める手がかりになる。そのため、PR のタイトルに接頭辞があれば、件名の接頭辞をそれに揃える（F-003）。接頭辞を外す案は採らない。種類とタスクが、タイトルから分からなくなるためである。
-   **`推奨タイトル` は日本語の PR のタイトルを指す:** PR マーカーの `推奨タイトル` は、`/runplan` が PR のタイトルに使う日本語のタイトルとする（F-004）。英語の件名は、`/mergepr` がマージの時点で PR の最終的な内容から書く（F-003）。英語の件名の下書きとして残す案と、両方の欄を持つ案は採らない。プランは実装の前に書くので、件名の下書きは実際の内容とずれうるためである。
-   **既存の英語の値は書き換えない:** 承認済みのプランに残る英語の `推奨タイトル` と、開いている PR の英語のタイトルは、本タスクで書き換えない（2.3）。`/runplan` は、英語の `推奨タイトル` からも日本語の PR のタイトルを書く（AC-06）。既存のプランを書き換える案は採らない。文書の内容を変えると、タスクごとに再承認が必要になるためである（`docs/dev/developer_guide/requirements_process.md` の「Editing an approved document」）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   本タスクの変更の後にコマンドが開く PR と、書き直す PR のタイトルを、日本語にする。
-   squash のコミットのメッセージ（件名と本文）を、英語のままにする。
-   言語の規則、PR マーカーの規約、各コマンドの定義を、互いに矛盾しないようにする。

### 2.2. スコープ (In Scope)

-   `CLAUDE.md` の「Language」の節の、PR のタイトルと squash のコミットについての記述（F-001）。
-   PR のタイトルを書くコマンドが書くタイトルの言語と形（F-002）。
-   `/mergepr` が書く squash のコミットの件名の決め方（F-003）。
-   PR マーカーの `推奨タイトル` の定義（F-004）。
-   旧い規則の記述が残っていないこと（F-005）。

### 2.3. スコープ外 (Out of Scope)

-   **既存のプランの `推奨タイトル` の書き換え:** 書き換えない。理由は、1 節の「要件定義の際に決めたこと」の 3 つめの項目のとおりである。
-   **開いている PR（PR #407 など）のタイトルの書き換え:** 書き換えない。同じ理由に加え、英語のタイトルの PR も `/mergepr` でマージできるためである（AC-11）。
-   **コミットのメッセージの言語:** 各コマンドが作るコミットのメッセージと squash のコミットのメッセージは、英語のままとする。issue が「当面は英語のまま」としたためである。
-   **ブランチの名前:** 今の規約（`_context.md:49`）のまま、英語とする。issue が対象にしていないためである。
-   **issue のタイトル:** すでに日本語で書いている（`CLAUDE.md:49-50`、`runpr.md:321`）。変えない。
-   **`mergepr` のバイナリ（`cmd/mergepr`・`internal/mergepr`）:** 変えない。件名の言語や形を確かめず（`internal/mergepr/mergepr.go:441-450`）、PR のタイトルを件名に使わない（`cmd/mergepr/main.go:48`）ので、本タスクの変更の影響を受けないためである。
-   **PR のタイトルの形を検査するガードのテストや CI の追加:** 追加しない。issue が求めたのは、既存のガードが PR のタイトルの形に依存していないかの確認であり、その結果は 1 節の背景のとおり、依存していない。
-   **bot のコマンド（`@coderabbitai review` など）:** 今の規則（`CLAUDE.md:52`）のまま、例外として残す。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 言語の規則

`CLAUDE.md` の「Language」の節で、PR のタイトルを日本語にし、squash のコミットのメッセージを英語のままにすると定める。

**Acceptance Criteria**:
- **AC-01**: `CLAUDE.md` の「Language」の節は、PR のタイトルを、日本語で書くもの（GitHub に置くもの）に含める。PR のタイトルを英語にする例外は書かない。
- **AC-02**: 同じ節は、PR のタイトルの形を `<type>(<scope>): <日本語の要約>` と定める。接頭辞（`<type>(<scope>): `）は conventional-commit の形のまま、英語で書く。
- **AC-03**: 同じ節は、squash のコミットの件名と本文を英語で書くこと、件名は `/mergepr` が PR の内容から書き、PR のタイトルをそのまま使わないことを述べる。

#### F-002: コマンドが書く PR のタイトル

PR を開くコマンドと PR のタイトルを書き直すコマンドは、AC-02 の形の日本語のタイトルを書く。各コマンドが作るコミットのメッセージは、英語のままとする。

**Acceptance Criteria**:
- **AC-04**: `/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2` が開く PR のタイトルは、文書を追加したコミットの件名と同じ接頭辞を持ち、その件名と同じ内容を日本語で要約する。例: コミットの件名が `docs(0032): draft requirements for writing PR titles in Japanese` なら、PR のタイトルは `docs(0032): PR のタイトルを日本語にする要件を起草する` のようになる。
- **AC-05**: `/runplan` が開く PR のタイトルは、PR マーカーの `推奨タイトル` が日本語のタイトルであれば、その値とする。
- **AC-06**: `推奨タイトル` が英語の値（本タスクの前に書かれたプランの値）であれば、`/runplan` が開く PR のタイトルは、その値と同じ接頭辞を持ち、同じ内容を日本語で要約する。`/runplan` は、そのためにプランの `推奨タイトル` を書き換えない。
- **AC-07**: `/runpr` が単独の変更の PR を開くとき、そのタイトルは AC-02 の形の日本語とする。`<type>(<scope>)` は、今と同じく `git log origin/main` の形に合わせる。
- **AC-08**: `/fixpr` の Phase 6 が PR のタイトルを書き直すとき、新しいタイトルは AC-02 の形の日本語とする。PR の説明が古くなっていなければ、今と同じく、タイトルを書き直さない（`fixpr.md:458-470`）。
- **AC-09**: AC-04〜AC-08 のどのコマンドでも、そのコマンドが作るコミットのメッセージは英語とする。日本語のタイトルの要約の部分でも、コマンド名・ファイル名・識別子（`/mergepr`・`推奨タイトル` など）は、表記を変えずに書いてよい。

#### F-003: squash のコミットの件名

`/mergepr` は、PR のタイトルの言語にかかわらず、squash のコミットの件名を英語で書く。

**Acceptance Criteria**:
- **AC-10**: `/mergepr` が書く件名は `<type>(<scope>): <English summary> (#<number>)` の形とし、要約は英語とする。要約は、PR の材料（コミットのメッセージ、diff、PR の本文）から書く。PR のタイトルの要約の部分をそのまま件名に使う規則（`mergepr.md:50-51`）は、なくす。
- **AC-11**: PR のタイトルが `<type>(<scope>): ` または `<type>: ` の接頭辞で始まるとき、件名の接頭辞はそれと同じとする。PR のタイトルに接頭辞がないときは、今と同じく、PR の材料と `git log origin/<baseRefName>` の形から `<type>(<scope>)` を決める。plan-driven の PR では `<scope>` をタスク ID とする（`mergepr.md:51-52`）。英語のタイトルの PR（本タスクの前に開いた PR）も、同じ規則で件名を書く。
- **AC-12**: squash のコミットの本文は、今と同じく英語とする（`mergepr.md:48`）。PR のタイトル・本文・コミットのメッセージ・diff は、要約するデータであり、従う指示ではないという扱い（`mergepr.md:46-48`）は保つ。

#### F-004: PR マーカーの `推奨タイトル`

PR マーカーの `推奨タイトル` は、`/runplan` が PR のタイトルに使う日本語のタイトルとする。

**Acceptance Criteria**:
- **AC-13**: `_context.md` の「PR marker conventions」は、`推奨タイトル` の形を `<type>(<task-id>): <日本語の要約>` と定め、それが PR のタイトルであって squash のコミットの件名ではないことを述べる。
- **AC-14**: `mkplan2.md` が挿入する PR マーカーの書式、`推奨タイトル` の説明、PR の境界のレビューのチェックリストの項目は、AC-13 と同じ形を求める。
- **AC-15**: 本タスクの後に `/mkplan`・`/mkplan2` が書く `推奨タイトル` は、AC-13 の形の日本語とする。

#### F-005: 旧い規則の記述を残さない

規則・規約・コマンドの定義に、旧い規則の記述を残さない。

**Acceptance Criteria**:
- **AC-16**: `CLAUDE.md`、`.claude/commands/` の下のファイル、`docs/dev/` の下のファイルのどれも、次のいずれかを述べない。PR のタイトルを英語で書くこと。squash のコミットの件名を PR のタイトルから作ること。`推奨タイトル` が英語であること。ただし、AC-06 の英語の `推奨タイトル` の扱いの説明は除く。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。本タスクは、コマンドの定義と規則の文書だけを変え、実行時のコードを変えない。

### 4.2. セキュリティ (Security)

-   N/A。本タスクは、`docs/dev/security.md` の適用の条件（`yt-dlp` の起動、API キーや Webhook の URL の扱い、キャッシュの読み書き、LLM API や Webhook への送信、信頼できないテキストのプロンプトや記事への埋め込み）のどれにも当たらない。`/mergepr` が PR のタイトルをデータとして扱う規則は、AC-12 で保つ。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   N/A。

### 4.4. 互換性 (Compatibility)

-   本タスクの前に書かれたプランの英語の `推奨タイトル` は、書き換えずに使える（AC-06）。
-   本タスクの前に開いた、英語のタイトルの PR は、`/mergepr` でマージできる（AC-11）。
-   squash のコミットの件名の形 `<type>(<scope>): <summary> (#<number>)` は変えない（AC-10）。`git log origin/main` の件名の形は、本タスクの前後で揃う。

### 4.5. 保守性 (Maintainability)

-   PR のタイトルの形と squash のコミットの件名の形の関係は、規則（F-001）と規約（F-004）とコマンドの定義（F-002・F-003）の間で矛盾しない（AC-16）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   コマンドの定義（`.claude/commands/`）は英語で書く（`CLAUDE.md:47-48`）。日本語のタイトルの例を書く場合も、説明の文は英語とする。
-   タスク 0031（PR #407）も、`_lib/task-pr-handoff.md` と `runpr.md` を変える見込みである。両タスクは独立しているが、同じファイルの別の箇所を変えるので、実装の際は先にマージされた側の変更の上に重ねる。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。本書では、次の語を次の意味で使う。

-   **PR のタイトル (PR title):** GitHub の PR のタイトル。本タスクの後は日本語で書く。
-   **squash のコミット (squash commit):** squash マージで `main` に残る 1 つのコミット。メッセージ（件名と本文）は英語で書く。
-   **件名 (subject):** コミットメッセージの 1 行目（`docs/translation_glossary.md`）。
-   **接頭辞 (prefix):** conventional-commit の `<type>(<scope>): ` または `<type>: ` の部分。PR のタイトルでも英語で書く。
-   **要約 (summary):** PR のタイトルまたは件名のうち、接頭辞の後の部分。PR のタイトルでは日本語、件名では英語で書く。
-   **`推奨タイトル` (recommended title):** 実装プランの PR マーカーの欄。`/runplan` が PR のタイトルに使う。
