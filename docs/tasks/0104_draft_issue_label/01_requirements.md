# 要件定義書：`/mkreq` はラベル `draft` が付いた issue からタスクを始めない

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-18 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、GitHub のラベル `draft` で下書きと宣言された issue から、`/mkreq` がタスクを始めないようにする要件を定義する。

**背景:**

-   まだ着手しない issue を、下書きとして先に起票しておきたい場合がある。例は #928・#930・#931・#932 で、どれもタスク 0092 の完了後に改訂して正式なものにする予定である。
-   下書きであることを題名の印（「【下書き】」など）で示しても、`/mkreq` はそれを機械的に判定できない。題名の文字列で判定すれば、CLAUDE.md の「Declare, don't infer」に反する。そこで、下書きであることを GitHub のラベル `draft` で宣言する。ラベルはすでにあり、上の 4 件に付けてある。
-   今の `/mkreq` は、step 1 で `gh issue view <issue> --json number,title,body,state,url,comments` で issue を読み、閉じていれば止まる（`.claude/commands/mkreq.md:48-49`）。ラベルは読まないので、下書きの issue からもタスクを始めてしまう。
-   chief は、タスク 0092 で、ラベル `draft` が付いた開いた issue では workspace を作らないようにした（`internal/chief/issue.go:58-60`・`:239-246`、[0092 要件定義書](../0092_chief_task_workspace/01_requirements.md) AC-05）。ラベルの名前がバイト列で `draft` と等しいときだけ下書きとし、閉じた issue はラベルによらず閉じた issue として扱う。

対応 issue: [#934](https://github.com/isseis/yt2column/issues/934)（関連: [#839](https://github.com/isseis/yt2column/issues/839)（タスク 0092）、[#928](https://github.com/isseis/yt2column/issues/928)（ラベル `draft` を最初に使う issue））

本書は、`/mkreq` が下書きの issue で止まるときの振る舞いと報告、ラベルの定義の置き場所、ADR を決める。判定に使うコマンドと手順の書き方は `02_architecture.md` が決める。本タスクは簡易な進め方をとるので、`02_architecture.md` と `03_implementation_plan.md` も同じ PR で書く。

### issue #934 で決めたこと

-   **下書きであることは、GitHub のラベル `draft` で宣言する。題名や本文の印では判定しない。** 題名の文字列で振る舞いを選ぶと、CLAUDE.md の「Declare, don't infer」に反するためである。
    -   ADR：[ADR-0053](../../dev/adr/0053-draft-issue-label.md) を作る（要件定義の際に決めたことの ADR の項を参照）。
-   **`/mkreq` は、step 1 で issue のラベルを読み、ラベル `draft` が付いていれば、閉じた issue と同じように止まる。下書きであることと、ラベルを外せば正式になることを報告する（F-001）。** 下書きの issue からタスクを始めないためである。ラベルを読む方法は設計で決め、issue が挙げた「今の `gh issue view` の `--json` に `labels` を足す」形ではなく、判定のための別の `gh issue view` で読む（[02_architecture.md](02_architecture.md)「設計の際に決めたこと（2026-11-18）」）。
    -   ADR：ADR-0053。
-   **下書きの issue のラベル（`draft`）を、`.claude/commands/_context.md` の「Process convention」の表に 1 行で定義する。そのラベルが付いた issue からはタスクを始めないこと、外せば正式になることを書く（F-002）。** コマンドが参照する、プロジェクト固有の値の置き場所だからである。
    -   ADR：ADR-0053。
-   **正式にするときは、人がラベル `draft`（と本文の注記）を外す。** ラベルを外す操作だけで、下書きと正式を切り替えられるようにするためである。
    -   ADR：ADR-0053。
-   **chief が workspace を作る前の issue の確かめで、ラベル `draft` の issue を拒むことは、本タスクの範囲外とする。** タスク 0092 を担当するエージェントに依頼したためである。この依頼は 0092 で実装済みである（背景を参照）。
    -   ADR：なし。本タスクでは決めない。chief の決定は ADR-0046 にある。

### 要件定義の際に決めたこと（2026-11-18）

-   **進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。** 変える場所は `_context.md` の 1 行、`mkreq.md` の step 1、ADR の 3 つだけである。新しい仕組みは作らず、開いている決定も少ない。既存の ADR の決定も変えない。除外の判断：`docs/dev/security.md` の規則と前提を変えない。レビューのスレッドや Bot への依頼の扱いに触れず、`fixprguard`・`codexreview`・`mergepr` も変えないので、開発の道具の信頼境界を変えない。外部依存も足さない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。人の答え：簡易な進め方。
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   **下書きの issue の方針は、新しい ADR（ADR-0053）にする。ADR-0046 は変えない。** この方針は、chief だけでなく `/mkreq` も縛る、開発プロセスの方針である。chief の ADR-0046 に `/mkreq` の振る舞いを入れると、主題が chief からずれる。ADR-0053 は、chief が下書きの issue を拒むことを ADR-0046 へのリンクで示す。人の答え：新しい ADR を作る。
    -   ADR：[ADR-0053](../../dev/adr/0053-draft-issue-label.md) を作る。開発プロセスの方針で、`/mkreq` と chief の 2 つを縛るので、主要な決定に当たる（CLAUDE.md「Design decisions (ADR)」）。
-   **下書きの印とみなすのは、名前がバイト列でちょうど `draft` と等しいラベルだけとし、chief と同じにする。`Draft`・`DRAFT`・`draft-x` のラベルは印とみなさない。閉じた issue は、ラベルによらず閉じた issue として止まり、閉じていることを報告する（F-001）。** 2 つの道具の判定が食い違うと、chief は workspace を作ったのに `/mkreq` が止まる、またはその逆が起きるためである。人の答え：chief と同じ。
    -   ADR：ADR-0053。
-   **変える文書は、`_context.md`・`mkreq.md`・ADR（と ADR の索引）だけとする。`requirements_process.md` などの手順の要約には、下書きの issue のことを足さない。** ラベルの意味の正本を `_context.md` の 1 か所に置くためである。人の答え：issue の 3 つだけ。
    -   ADR：なし。本タスクの中で閉じる、範囲の選択である。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   下書きの issue から、`/mkreq` がタスクを始めないようにする。タスク番号の決定と、ブランチやタスクのディレクトリの作成も含む。
-   下書きであることの判定を、`/mkreq` と chief で同じにする。

### 2.2. スコープ (In Scope)

-   `.claude/commands/mkreq.md` の step 1 に、下書きの issue の確かめを加える（F-001）。
-   `.claude/commands/_context.md` の「Process convention」の表に、下書きの issue のラベルの行を加える（F-002）。
-   ADR-0053 の作成と、ADR の索引への行の追加（F-003）。

### 2.3. スコープ外 (Out of Scope)

-   chief の issue の確かめの変更。タスク 0092 ですでに、ラベル `draft` が付いた開いた issue を拒む（1 章の背景）。
-   ADR-0046 の変更。ADR-0053 から ADR-0046 へリンクし、ADR-0046 の文はそのまま残す（要件定義の際に決めたこと）。
-   `requirements_process.md`・CLAUDE.md・`chief_guide.md` などへの、下書きの issue の説明の追加（要件定義の際に決めたこと）。
-   ラベル `draft` の作成と、#928・#930・#931・#932 へのラベルの付け外し。ラベルはすでにあり、4 件に付いている。
-   `/mkreq` 以外のコマンド（`/mkarch`・`/mkplan`・`/runplan` など）での確かめ。これらはタスクのディレクトリから始まり、issue からタスクを始めない。
-   ラベルを付け外しする道具。下書きと正式の切り替えは、人がラベルを外して行う。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: `/mkreq` は下書きの issue で止まる

`/mkreq <issue> [task]` は、step 1 で issue が下書きの issue かを確かめる。下書きの issue なら、タスク番号を決める前に止まり、何も作らない。下書きの issue とは、ラベル `draft` が付いた開いた issue である（6 章）。

**Acceptance Criteria**:
- **AC-01**: `mkreq.md` の step 1 は、issue が閉じているかと、このリポジトリの issue かを確かめた後、重複の検索（`git fetch origin` とその後の検索）より前に、issue が下書きの issue かを確かめる。下書きの issue なら止まり、下書きの issue であることと、ラベル `draft` を外せば正式な issue になり `/mkreq` を走らせ直せることを報告する、と述べる。確かめの位置が `git fetch origin` と重複の検索より前なので、`[task]` を与えたときも、止まった簡易な進め方の run を続けるときも、同じく止まる。
- **AC-02**: `mkreq.md` の step 1 の確かめは、名前がバイト列でちょうど `draft` と等しいラベルがあるときだけ下書きの issue とする。`Draft`・`DRAFT`・`draft-x` のように名前が `draft` と等しくないラベルだけの issue と、ラベルのない issue では止まらない。`mkreq.md` は下書きの issue の定義を `_context.md` の行（AC-06）の名前で参照し、定義を書き写さない。
- **AC-03**: `mkreq.md` の step 1 は、下書きの確かめを閉じているかの確かめの後に置く。閉じた issue は、ラベルによらず今までどおり閉じた issue として止まり、そう報告する。
- **AC-04**: `mkreq.md` の step 1 は、ラベルを読めないとき、または確かめのコマンドが失敗したときは止まり、確かめられなかったことを報告する、と述べる。そのとき、下書きでない issue として先へ進まない。
- **AC-05**: ラベル `draft` が付いた開いた issue（#928）で、本タスクの実装を入れたブランチの `/mkreq` を走らせると、step 1 で止まる。下書きの issue であることと、ラベルを外せば正式になることを報告する。`numreserve` は走らず、ブランチもタスクのディレクトリも作られない。

#### F-002: 下書きの issue のラベルを `_context.md` に定義する

**Acceptance Criteria**:
- **AC-06**: `_context.md` の「Process convention」の表に、下書きの issue のラベルの行がちょうど 1 つある。その行は次を述べる。
    -   ラベルの名前は `draft` で、名前がバイト列でちょうど等しいラベルだけが当たる。
    -   ラベル `draft` が付いた開いた issue は下書きの issue で、そこからタスクを始めない。`/mkreq` は step 1 で止まり、chief はその issue の workspace を作らない。
    -   閉じた issue は、ラベルによらず閉じた issue として扱う。
    -   ラベルを外せば、正式な issue になる。

#### F-003: ADR-0053 を作る

**Acceptance Criteria**:
- **AC-07**: `docs/dev/adr/0053-draft-issue-label.md` があり、状態は `有効` である。ADR の索引（`docs/dev/adr/README.md`）に、番号の位置で、その行がある。`TestADRRecords` が通る。
- **AC-08**: ADR-0053 の「決定」は、本書と `02_architecture.md` の決定（下書きをラベル `draft` で宣言すること、名前の等しさだけで判定すること、`/mkreq` が止まること、chief が workspace を作らないこと（ADR-0046 へのリンク）、正式にするには人がラベルを外すこと）を述べ、ラベルの定義の正本として `_context.md` にリンクする。
- **AC-09**: 実装の PR がマージされた後の ADR-0053 の「決定」には、`mkreq.md` がまだ下書きの issue で止まらないことを述べる食い違いの記述がない。本書の PR では、その食い違いの記述が issue #934 にリンクして「決定」にある（`docs/dev/conventions/adr.md`「Writing an ADR」）。

#### F-004: ほかの部分を変えない

**Acceptance Criteria**:
- **AC-10**: 実装の PR の `mkreq.md` の差分は、step 1 の中に限る。`_context.md` の差分は、「Process convention」の表への 1 行の追加に限る。ADR-0053 の差分は、AC-09 の食い違いの記述の削除に限る。この 3 つと本タスクのディレクトリの外では、ファイルの変更・追加・削除がない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。`/mkreq` が GitHub に問い合わせる回数が増えるだけで、走る時間は人が気づくほど変わらない。

### 4.2. セキュリティ (Security)
-   N/A。`docs/dev/security.md` の規則と前提に触れない。ラベルの名前は GitHub から読む値だが、名前が `draft` と等しいかを見るだけで、名前で手順を選んだり、会話に命令として取り込んだりしない。読めないときや確かめが失敗したときは止まる（AC-04）。chief と違い、`/mkreq` は読んだラベルが issue のラベルのすべてかを確かめない（chief は件数を照らし合わせる。[0092 設計書](../0092_chief_task_workspace/02_architecture.md) 1.5）。ラベルが極めて多い issue で `draft` が読んだ範囲に入らなければ、`/mkreq` は先へ進みうる。これは chief との違いとして受け入れる。下書きの issue にラベルを多数付ける運用はなく、先へ進んでも、人が最初の問いで止められるためである。ラベルを付けられる人が下書きの印を付けても、`/mkreq` が止まるだけで、タスクが作られることはない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   ラベルを読めないときは止まり、下書きでないとみなして進めない（AC-04）。下書きの issue からタスク番号を決めると、取り消せない記録の tag が残るためである（[ADR-0051](../../dev/adr/0051-number-reservation.md)）。

### 4.4. 互換性 (Compatibility)
-   ラベル `draft` が付いていない issue での `/mkreq` の振る舞いは変わらない。

### 4.5. 保守性 (Maintainability)
-   ラベルの名前 `draft` は、`_context.md`、`mkreq.md` の step 1 の確かめのコマンド、chief のコード（`internal/chief/issue.go:60`）の 3 か所にある。名前を変えるときは 3 つとも変える（ADR-0053 の「結果」）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   CLAUDE.md の「Declare, don't infer」に従い、題名や本文の文字列では判定しない。
-   CLAUDE.md の「Document tests check values, structure, and executable examples, never prose」に従う。`mkreq.md` と `_context.md` の文の中身は、PR のレビューで読んで確かめる。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **下書きの issue:** 名前がバイト列でちょうど `draft` と等しいラベルが付いた、開いた issue。そこからタスクを始めない。文書の状態の `draft`（タスク文書の Status の値）とは別のものである。
-   **正式な issue:** 下書きの issue でない開いた issue。人がラベル `draft` を外すと、下書きの issue は正式な issue になる。
-   **食い違いの記述:** ADR の決定と、その仕様を持つ文書やコードが食い違うときに、「決定」に残す記述（`docs/dev/conventions/adr.md`「Writing an ADR」）。
