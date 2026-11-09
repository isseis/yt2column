# 要件定義書：使われていない軽量プロセスの削除

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、軽量プロセス（`docs/dev/conventions/requirements_process.md`「Lightweight Process」）と、それを支えるコマンド・スクリプト・テスト・ラベル・テンプレートを削除する要件を定義する。

**背景:**

-   軽量プロセスは、タスク 0061（#619）で導入した。単純な変更を、タスク文書を作らずに 1 本の PR で進める手順である。`/mkreq` が issue の条件を判定し、人が承認した仕様のコメントを issue に投稿する（`.claude/commands/mkreq.md` の step 1「Lightweight triage」）。`/runissue` がそれを実装し、`lightweight-check.sh` が実際の diff で条件を確かめ直す（`docs/dev/conventions/requirements_process.md:418-617`、[ADR-0012](../../dev/adr/0012-lightweight-check-in-bash.md)）。
-   issue #744 によると、軽量プロセスは導入の後、一度も使われていない。条件が実際の変更にほとんど当てはまらないためである。タスク 0071 と 0072 も、要件定義書に「軽量プロセスを取らない理由」を書いて、通常のプロセスを選んだ（`docs/tasks/0071_security_source_link/01_requirements.md` §1、`docs/tasks/0072_simple_task_documents/01_requirements.md` §1）。
-   一方で、維持のコストは大きい。専用のファイルは 6 つで、計 3,052 行ある。内訳は、`runissue.md` 453 行、`_lib/lightweight-spec.md` 114 行、`_lib/lightweight-check.sh` 686 行、`cmd/yt2column/lightweight_docs_test.go` 1,750 行、`internal/doctestutil/lightweight.go` 27 行、`.github/ISSUE_TEMPLATE/lightweight.md` 22 行である。ほかに、10 を超えるファイルが軽量プロセスに触れている（2.2 節）。
-   文書を作る負荷を減らす手段は、タスク 0072（#743）の簡易な進め方が引き継ぐ。0072 の要件定義書は承認済みで、実装はまだである。0072 は、軽量プロセスの削除を #744 に任せた（`docs/tasks/0072_simple_task_documents/01_requirements.md` §2.3、[ADR-0040](../../dev/adr/0040-simple-task-documents.md) の「結果」）。

**軽量プロセスを取らない理由:** この変更は、約 3,000 行と 20 を超えるファイルにわたり、ADR を廃止する。軽量プロセスの行数・ファイル数の上限を超え、「Changes that always take the normal process」の「A change of wording that changes a meaning」にも当たる（`docs/dev/conventions/requirements_process.md`）。

対応 issue: [#744](https://github.com/isseis/yt2column/issues/744)（関連: [#743](https://github.com/isseis/yt2column/issues/743)（タスク 0072）、[#619](https://github.com/isseis/yt2column/issues/619)（タスク 0061）、[ADR-0012](../../dev/adr/0012-lightweight-check-in-bash.md)、[ADR-0035](../../dev/adr/0035-language-policy.md)、[ADR-0040](../../dev/adr/0040-simple-task-documents.md)）

**本書の記述範囲:** 本書は、次のことを定める。
-   削除するファイルと、軽量プロセスへの参照を取り除く範囲。
-   削除の後のコマンドの振る舞い。
-   ADR と GitHub のラベルの扱い。
-   削除の確かめ方。

次のことは `02_architecture.md` で決める（`design_handoff.md`）。
-   軽量プロセスのためだけにある手順や入力（`_lib/task-pr-handoff.md` の `ISSUE_KEYWORD` など）を残すか除くか。
-   テストの削除と書き換えの分け方。
-   コミットと PR の分け方。

**issue #744 で決めたこと:**

-   **軽量プロセスと、それを支えるファイルを削除する。** 一度も使われず、維持のコストが大きいためである。
-   **削除するファイルは、`runissue.md`・`_lib/lightweight-spec.md`・`_lib/lightweight-check.sh`・`lightweight_docs_test.go`・`doctestutil/lightweight.go`・`.github/ISSUE_TEMPLATE/lightweight.md` とする。**
-   **軽量プロセスに触れている記述を、指示・規約・コマンド・テストから取り除く。**
-   **ADR-0012 を、ADR の規約（`docs/dev/conventions/adr.md`）に従って廃止する。ADR-0035 などの記述もそろえ、ADR の索引を更新する。**
-   **GitHub の `lightweight` ラベルを、付いた issue がないことを確かめてから削除する。**
-   **削除の確認は、タスクの `_checks/` で一度だけ行う。軽量プロセスが残っていないことを確かめる恒久のテストは足さない。** 消したものが無いことを確かめるテストは、残すと、後の変更で同じ語を使うたびに失敗するためである（`CLAUDE.md`「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose.」）。
-   **既存のタスク文書は書き換えない。** タスク文書は、そのタスクの時点の記録だからである。
-   **開発文書を簡易にとどめる分岐の導入は扱わない（#743）。**

**要件定義の際に決めたこと（2026-11-09）:**

-   **ADR-0012 を `廃止` にし、代わりの ADR を ADR-0040 とする。ADR-0040 と ADR-0035 は、決定の変更として直し、経緯を足す（F-004）。** ADR-0012 の主題は、軽量プロセスの確かめ直しのスクリプトである。軽量プロセスを削除すると、この主題は終わる。文書を作る負荷を減らす役目は、ADR-0040 の簡易な進め方が引き継ぐ（issue #744）。ADR-0040 の「結果」は「軽量プロセスは #744 で削除するまで残る」と書いており、削除の後の状態に改める。ADR-0035 の「決定」は、ADR-0012 を参照している。また、ADR-0035 の経緯には、軽量プロセスのための日本語のリテラルの例外を足した記録がある。この例外を除くので、ADR-0035 の参照を除き、経緯を足す。どちらも、文の誤りを直すだけの編集上の修正ではなく、決定の範囲を変えるので、規約の「迷うときは決定の変更として扱う」に従う（`docs/dev/conventions/adr.md`「Editorial corrections」）。
    -   採らなかった案：代わりの ADR を「なし」にする案。ADR-0012 の主題（bash のスクリプト）だけを見ればこれも成り立つ。しかし、軽量プロセスの役目を引き継ぐ ADR を指すほうが、読み手が今の決定にたどり着きやすい。
    -   採らなかった案：軽量プロセスの削除の ADR を新しく作る案。規約は、主題が終わった ADR を `廃止` にするとし、主題ごとに 1 つの ADR を置く（`docs/dev/conventions/adr.md`「Changing, retiring, and withdrawing」）。
    -   ADR：[ADR-0012](../../dev/adr/0012-lightweight-check-in-bash.md) を廃止し、[ADR-0035](../../dev/adr/0035-language-policy.md) と [ADR-0040](../../dev/adr/0040-simple-task-documents.md) を更新する（本書の PR で行う）。
-   **タスク 0072 の実装と本タスクの実装の順序は、要件では決めない。後からマージする側が、その時点の `main` に残る軽量プロセスへの参照を取り除く（AC-03）。** 両方のタスクが `mkreq.md`・`requirements_process.md`・`CLAUDE.md` を変えるので、後からマージする側が衝突を解く。issue #744 は 0072 を先に進めることを勧めた。理由は、文書を作る負荷を減らす手段がない期間を作らないためである。しかし、軽量プロセスは一度も使われていないので、そのような期間ができても実害はない。
    -   採らなかった案：本タスクの実装を、0072 の最後の実装の PR がマージされた後に始める案。本タスクが、実害のない理由で待つことになる。
    -   ADR：なし。本タスクの中で閉じる、実装の順序の決定である。
-   **GitHub の `lightweight` ラベルは、本タスクの実装の PR がマージされた後に削除する。その前に、ラベルの付いた issue が 0 件であることを確かめ、ユーザーの承認を得る。1 件でもあれば、削除せずに止まって報告する（AC-09・AC-10）。** ラベルの削除は、ネットワークに触れ、元に戻せない操作である（`CLAUDE.md`「Tool Execution Safety」）。マージの前に削除すると、マージまでの間に `main` の `/runpr` が、ラベルを持つ PR の分類を読めなくなる。
    -   採らなかった案：本タスクではラベルに触れず、手での削除を案内するだけにする案。issue #744 はラベルの削除をやることに挙げており、確かめの手順を決めずに人に任せると、付いた issue の確認が抜けうる。
    -   ADR：なし。本タスクの中で閉じる手順の決定である。

**上の決定と issue から導いたこと（PR のレビューで異論があれば改める）:**

-   **issue が挙げていない参照も取り除く（AC-02）。** 調べたところ、issue の一覧にない参照が 3 つあった。`.claude/commands/mktrans.md:35`、`cmd/yt2column/fixpr_docs_test.go:27,986,1004`、ADR-0040 の「結果」である。逆に、issue が挙げた `docs/dev/conventions/test_organization.md` の一致は、`:14` の「Lightweight mocks」だけで、軽量プロセスとは関係がない。そのため、このファイルは変えない。issue の目的は軽量プロセスを残さないことなので、一覧ではなく目的で範囲を決める。ADR：なし。
-   **通常のプロセスの振る舞いは変えない（F-003）。** issue が求めるのは、軽量プロセスの削除だけだからである。ADR：なし。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   使われていない軽量プロセスを削除し、その維持のコストをなくす。
-   削除の後も、通常のプロセスのコマンドが今と同じように動く。

### 2.2. スコープ (In Scope)

-   1 章の 6 つの専用ファイルの削除（F-001）。
-   次のファイルからの、軽量プロセスへの参照の除去（F-002）。実装の時点の `main` で、ほかのファイルに参照があれば、それも含める（AC-03）。
    -   `CLAUDE.md`（`:37-45` の Documents の例外、`:110` の Translation Guidelines、`:207-208` と `:233-238` の Commands、`:673-683` の Development Notes、`:746-757` の Go Idioms の lightweight-check literal exception、`:784-791` の Requirements and Acceptance Criteria）
    -   `AGENTS.md:11`（lightweight PR の例外）
    -   `docs/dev/conventions/requirements_process.md`（`:15-21` の LLM Constraints の例外、`:87-105` の Review Flow、`:418-617` の Lightweight Process の節）
    -   `.claude/commands/mkreq.md`（step 1 の Lightweight triage とその他の参照）、`runpr.md`（Lightweight PR の分類と、その issue を閉じる手順）、`mktrans.md:35`、`_context.md:55-58,87-88`、`_lib/task-pr-handoff.md`（`runissue.md` のための記述）、`_lib/local-cli-review.md:20`
    -   `cmd/yt2column/codex_review_docs_test.go:80,84-85,156-161`、`cmd/yt2column/fixpr_docs_test.go:27,986,1004`
-   ADR-0012 の廃止と、ADR-0035・ADR-0040・ADR の索引の更新（F-004。本書の PR で行う）。
-   GitHub の `lightweight` ラベルの削除（F-005）。
-   削除を一度だけ確かめる、タスクの `_checks/` のスクリプト（AC-11）。

### 2.3. スコープ外 (Out of Scope)

-   既存のタスク文書（0055、0060、0061、0065、0066、0067、0069、0070、0071、0072 など）の書き換え。issue #744 の決定による。
-   開発文書を簡易にとどめる分岐の導入。#743（タスク 0072）で扱う。
-   軽量プロセスが残っていないことを確かめる恒久のテスト。issue #744 の決定による。
-   `docs/dev/conventions/test_organization.md` の変更。1 章による。
-   通常のプロセスの手順の変更。1 章による。ただし、軽量プロセスのためだけにある記述を除いた結果、文の組み立てが変わる部分は含む。
-   ADR-0012 の本文の書き換え。規約は、廃止した ADR を削除せず、そのまま残すとしている（`docs/dev/conventions/adr.md`「Changing, retiring, and withdrawing」）。

## 3. 機能要件 (Functional Requirements)

本章で「軽量プロセスへの参照」とは、軽量プロセス・`/runissue`・`lightweight` ラベル・仕様のコメントとその印・軽量プロセスのブランチ名・1 章の 6 つの専用ファイル・その中で定義する名前（テストの関数、`doctestutil` の定数など）のどれかを指す記述をいう。

### 3.1. 機能一覧

#### F-001: 専用ファイルの削除

軽量プロセスのためだけにあるファイルを削除する。

**Acceptance Criteria**:
- **AC-01**: 実装の最後の PR がマージされた後の `main` に、次の 6 つのファイルがない。`.claude/commands/runissue.md`、`.claude/commands/_lib/lightweight-spec.md`、`.claude/commands/_lib/lightweight-check.sh`、`cmd/yt2column/lightweight_docs_test.go`、`internal/doctestutil/lightweight.go`、`.github/ISSUE_TEMPLATE/lightweight.md`。

#### F-002: 参照の除去

軽量プロセスへの参照を、今効いている指示・規約・コード・テストから取り除く。

**Acceptance Criteria**:
- **AC-02**: 実装の最後の PR がマージされた後の `main` で、`docs/tasks/` と `docs/dev/adr/` の外のファイルは、軽量プロセスへの参照を持たない。軽量プロセスと関係のない意味の語（`docs/dev/conventions/test_organization.md:14` の「Lightweight mocks」など）は、参照に当たらない。
- **AC-03**: AC-02 は、2.2 節に挙げたファイルだけでなく、実装の時点の `main` のすべてのファイルに当てはまる。タスク 0072 の実装が先にマージされて、軽量プロセスへの参照を足した場合も含む。
- **AC-04**: 実装の最後の PR がマージされた後の `main` で、`make test`・`make lint`・`make deadcode` が成功する。`make test` に、`bash` と `--attr-source` を受け付ける `git` を求めるテストのうち、軽量プロセスのためのもの（`TestLightweightCheck`）は残らない。

#### F-003: 削除の後のコマンドの振る舞い

軽量プロセスを除いた後も、通常のプロセスのコマンドは今と同じように動く。

**Acceptance Criteria**:
- **AC-05**: `/mkreq <issue>` は、開いた issue に対して、軽量プロセスの判定をしない。マーカーのコメントの一覧を読まず、軽量プロセスを提案せず、issue にコメントを投稿せず、ラベルを足さない。step 1 の残りの確認（閉じた issue・別のリポジトリの issue・同じ issue の要件定義書が既にある場合に止まること）は今と同じに動き、通常のプロセスで要件定義書を書く。
- **AC-06**: `/runpr` は、PR を Plan PR・Task-document PR・Standalone change のどれかに分類する。head のブランチが `<login>/issue-<N>` の形で、閉じる issue に `lightweight` ラベルがある PR も、Standalone change として扱い、マージの後にその issue を閉じない。Plan PR と Task-document PR の扱いは今と同じである。
- **AC-07**: 削除の後も、GitHub に投稿するコマンドの mention check は、残るすべての投稿の箇所で今と同じに働く。`fixpr_docs_test.go` と `codex_review_docs_test.go` の guard のテストは、残るコマンドの文書について、今と同じことを確かめる。

#### F-004: ADR の廃止と更新

ADR を、ADR の規約に従って廃止し、更新する（本書の PR で行う）。

**Acceptance Criteria**:
- **AC-08**: ADR-0012 の状態の表は、`状態` が `廃止` で、`廃止の日付`・`廃止の理由`・`代わりの ADR`（ADR-0040 へのリンク）の行を持ち、経緯に廃止の記録がある。ADR の索引の ADR-0012 の行は `廃止` である。ADR-0035 の「決定」は ADR-0012 を参照しない。ADR-0035 と ADR-0040 は、軽量プロセスの削除を経緯に記録する。ADR-0040 の「結果」は、軽量プロセスが残ると書かない。`TestADRRecords` が成功する。

#### F-005: ラベルの削除

GitHub の `lightweight` ラベルを、付いた issue がないことを確かめてから削除する。

**Acceptance Criteria**:
- **AC-09**: 実装の最後の PR がマージされた後、ラベルを削除する前に、`lightweight` ラベルの付いた issue と PR（開いたものと閉じたもの）の件数を確かめる。0 件のときは、ユーザーの承認を得てから削除する。削除の後、リポジトリのラベルの一覧に `lightweight` がない。
- **AC-10**: AC-09 の確認で 1 件以上あったとき、ラベルの一覧が読めなかったとき、ユーザーが承認しなかったときは、ラベルを削除せずに止まる。報告は、その理由と、見つかった issue と PR の番号を示す。

#### F-006: 削除の確認

削除が済んだことを、恒久のテストではなく、一度だけ確かめる。

**Acceptance Criteria**:
- **AC-11**: タスクの `_checks/` に、AC-01・AC-02 を、実装の最後の PR の head で確かめるスクリプトを置く。スクリプトは、残った参照があれば、そのファイルと行を出力して失敗する。このスクリプトは、`make test` にも CI にも組み込まない。
- **AC-12**: 軽量プロセスが残っていないことを確かめるテストを、`make test` と CI に足さない。
- **AC-13**: Go のテストを削除するコミットは、`CLAUDE.md`「Testing Strategy」の「Deleting a test is a claim that must be checked.」に従う。削除だけのコミットにし、`make coverage-compare BASE=<親のコミット>` の最後の行が `coverage-compare: PASS` で、その出力をコミットのメッセージと PR の説明に書く。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A（実行時の処理を変えない）。参考として、`TestLightweightCheck` が手元で 30〜50 秒かかっていたので（[ADR-0012](../../dev/adr/0012-lightweight-check-in-bash.md)「結果」）、`make test` はその分だけ短くなる。

### 4.2. セキュリティ (Security)
-   N/A。この変更は、`yt-dlp` を呼ばず、API キーや Webhook の URL を扱わず、キャッシュに触れず、ネットワークに送らず、信頼しないテキストをプロンプトにも記事にも入れない（`.claude/commands/_context.md` の「Conditional-guide trigger」）。
-   開発の道具の信頼の境界については、投稿する箇所が減るだけで、残る箇所の守り（mention check、`fixprguard`）は変えない（AC-07）。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。

### 4.4. 互換性 (Compatibility)
-   削除の後、`lightweight` ラベルと仕様のコメントを持つ issue は、通常のプロセスの issue として扱われる（AC-05・AC-06）。今、そのような issue はない（AC-09 で確かめる）。
-   過去のタスク文書と ADR の経緯は、削除したファイルへのリンクを持ちうる。過去の記録なので、リンクが切れても直さない（2.3 節）。

### 4.5. 保守性 (Maintainability)
-   専用ファイルの計 3,052 行と、ほかのファイルの記述を除くことで、指示と規約を読む量が減る。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR の廃止と更新は、`docs/dev/conventions/adr.md` に従う。
-   テストの削除は、`CLAUDE.md`「Testing Strategy」の「Deleting a test is a claim that must be checked.」に従う（AC-13）。
-   ラベルの削除はネットワークに触れ、元に戻せないので、ユーザーの承認を得てから行う（`CLAUDE.md`「Tool Execution Safety」）。
-   タスク 0072 の実装と並行して進みうる。後からマージする側が、衝突を解き、残る参照を取り除く（1 章）。

## 6. 用語集 (Glossary)

-   **軽量プロセス**：`docs/dev/conventions/requirements_process.md`「Lightweight Process」が定める、タスク文書を作らず issue の仕様のコメントから 1 本の PR にする進め方。本タスクで削除する。
-   **通常のプロセス**：要件定義書・設計書・計画書を作り、承認を得てから実装する進め方。
-   **軽量プロセスへの参照**：3 章の冒頭で定める。
-   **簡易な進め方**：タスク 0072 が定める、3 文書を簡易な内容で 1 本の PR で作る進め方（[ADR-0040](../../dev/adr/0040-simple-task-documents.md)）。
-   用語は `docs/translation_glossary.md` と統一する。
