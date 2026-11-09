# 要件定義書：ADR の編集上の訂正を、コマンドの定義と requirements_process.md の訂正の記録に対応させる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ADR の編集上の訂正を、タスク文書の編集上の訂正と同じ記録の手順で運べるようにするための要件を定義する。対象は、訂正の行と訂正の段落の定義（`docs/dev/conventions/requirements_process.md`）と、それを運ぶコマンドの定義（`/fixpr`・`/runplan`・ローカルの CLI レビュー・`_lib/task-pr-handoff.md`）である。

**背景:**

タスク 0070 は、決定を変えずに ADR を直す変更を編集上の訂正として認めた。その記録は、コミットメッセージの訂正の行と PR の説明の訂正の段落に置く（`docs/dev/conventions/adr.md`「Editorial corrections」の「Recording」）。ただし、記録の形と、それを運ぶコマンドの手順は、今はタスク文書だけを対象にしている。

-   訂正の行の `<document path>` は、タスク文書のリポジトリ相対のパスと定められている（`docs/dev/conventions/requirements_process.md:388`「Recording in the PR description」）。規約は、この形を ADR のパスにも当てはめると述べる（`docs/dev/conventions/adr.md:298-303`）。
-   `/fixpr` の修正のエージェントが返す `editorial` の項目は、承認済みのタスク文書への訂正だけである（`.claude/commands/fixpr.md:484-489`）。このセッションの確かめも、項目のパスが `docs/tasks/*/0[123]_*.md` であり、`HEAD` と作業ツリーで `approved` であることを求める（`.claude/commands/fixpr.md:531-548`）。
-   ローカルの CLI レビューの修正で訂正の行を書くのも、承認済みのタスク文書への訂正だけである（`.claude/commands/_lib/local-cli-review.md:205-209`、step 7 の表の `Editorial correction:` の行。同書 234 行）。
-   `/runplan` が訂正の行を書くのは、承認済みのタスク文書と実装のずれを直すときだけである（`.claude/commands/runplan.md:63`）。

一方、訂正の行を集めて PR の説明に載せる手順は、パスを問わない。`_lib/task-pr-handoff.md` Part B step 3 は、`git log origin/main..HEAD` のすべての訂正の行から訂正の段落を書く（`.claude/commands/_lib/task-pr-handoff.md:254-260`）。`/fixpr` の Phase 6 も、そのラウンドのコミットの訂正の行をすべて集めて追記する（`.claude/commands/fixpr.md:837-851`）。`fixprguard finish` は、本文のどの行も `Editorial correction: ` で始まることを確かめるが、パスは確かめない（`internal/fixprguard/write.go:112-127`）。

コマンドが ADR の訂正を運ばないため、0070 は、PR を作った後には ADR に編集上の訂正を加えないと決めた（`docs/dev/conventions/adr.md:308-314`「After the PR is created」、`docs/tasks/0070_adr_writing_criteria/02_architecture.md:67-74`）。PR を作った後の訂正は、訂正の行にも PR の説明にも載らず、レビュアーが分類に異を唱える機会がないためである。0070 の設計書は、この制限を本 issue で見直すとした（同書 9 章）。

**軽量プロセスを取らない理由:** この変更は、`docs/dev/conventions/` の規則と `.claude/commands/` の手順の意味を変える。また、軽量プロセスの除外のパスである `docs/dev/conventions/requirements_process.md` を変える（`docs/dev/conventions/requirements_process.md`「Changes that always take the normal process」「Mechanically checked conditions」）。

対応 issue: [#732](https://github.com/isseis/yt2column/issues/732)（前提: [#728](https://github.com/isseis/yt2column/issues/728)、タスク 0070。関連: [#729](https://github.com/isseis/yt2column/issues/729)、[#747](https://github.com/isseis/yt2column/issues/747)、[ADR-0039](../../dev/adr/0039-adr-writing-criteria.md)（ADR の背景と決定の書き方と、決定を変えない編集上の訂正）、[ADR-0036](../../dev/adr/0036-task-document-status-records.md)（タスク文書の状態と訂正の記録））

**本書の記述範囲:** 本書は、訂正の行と訂正の段落が ADR を含むことと、各コマンドが ADR の訂正を記録して運ぶことを、観測できる振る舞いとして定める。各コマンドの定義の英語の文面、`/fixpr` が ADR の項目を確かめる方法、文書のテストの変え方は、`02_architecture.md` で決める（[design_handoff.md](design_handoff.md)）。

**issue #732 で決めたこと:**

-   **`requirements_process.md` の訂正の行と訂正の段落の定義を、ADR のパスも含む形に広げる:** 0070 は規約の側で形を ADR に当てはめただけで、形を定める `requirements_process.md` はタスク文書だけを述べている。ADR：[ADR-0039](../../dev/adr/0039-adr-writing-criteria.md) を更新する（要件定義の際に決めたことの 1 つ目の項目）。
-   **`/fixpr`・`/runplan`・ローカルの CLI レビュー・`_lib/task-pr-handoff.md` が、ADR の編集上の訂正も運ぶようにする:** ADR の訂正の記録を、エージェントが手で書くことに頼らず、手順で保証するためである。ADR：ADR-0039 を更新する（同上）。
-   **関係する文書のテスト（`fixprguard` の呼び出しを含む）と ADR-0036・ADR-0039 をそろえる:** ADR-0039 は更新し、ADR-0036 は変えない（要件定義の際に決めたことの 1 つ目の項目）。
-   **#728 が `main` に入った後に着手する:** 0070 の実装（#745）は `main` に入っている。ADR：なし。作業の順序の決定で、本タスクの中で閉じる。

**要件定義の際に決めたこと（2026-11-09）:**

-   **PR を作った後にも、ADR に編集上の訂正を加えてよいとする:** 規約の「After the PR is created」の制限を外す。PR を作った後の `/fixpr` のラウンドとローカルの CLI レビューの修正でも、ADR に訂正を加えてよく、その訂正は訂正の行と PR の説明の訂正の段落に記録される（F-002・F-003・F-005）。
    -   **理由:** 制限の理由は、PR を作った後に ADR の訂正を記録する道筋がないことだけだった（`docs/tasks/0070_adr_writing_criteria/02_architecture.md:67-74`）。本タスクで `/fixpr` とローカルの CLI レビューが ADR の訂正を運ぶので、理由がなくなる。制限を残すと、レビューで見つかった古いリンクや誤字を ADR について直すのに、別の PR が要る。
    -   **採らなかった案:** 制限を残し、コマンドは PR を作る前の ADR の訂正だけを運ぶ案は、`/fixpr` の `editorial` をタスク文書に限ったままにでき変更は小さいが、制限を設けた理由がなくなった後も別の PR を求めるので採らない。
    -   ADR：[ADR-0039](../../dev/adr/0039-adr-writing-criteria.md) を更新する。ADR-0039 の「決定」は、PR を作った後には ADR に訂正を加えないと定めており、それを変えるので、`CLAUDE.md`「Design decisions (ADR)」の基準で major である（既存の ADR の決定を変える決定）。ADR-0036 は、ADR の編集上の訂正をどこに記録するかを ADR-0039 に委ねており（「決定」の 1 つ目の項目）、タスク文書についての決定は変わらないので、更新しない。
-   **`/runissue` は変えない:** 軽量プロセスの W（誤字の修正）で ADR を直したときの訂正の行の扱いは、本タスクの範囲に入れない（2.3）。
    -   **理由:** 軽量プロセスは削除する予定である（レビュアーの判断）。
    -   **採らなかった案:** `runissue.md` に、ADR への編集上の訂正には訂正の行を書くと足す案は、削除する予定の手順を変えることになるので採らない。
    -   ADR：なし。作業の範囲の決定で、本タスクの中で閉じる。
-   **squash コミットの本文に ADR の訂正の行を引き継ぐかどうかは、本タスクでは扱わない:** [#747](https://github.com/isseis/yt2column/issues/747) で扱う（2.3）。0070 の設計書は、これを本 issue で検討できるとしていた（`docs/tasks/0070_adr_writing_criteria/02_architecture.md:709`）。
    -   **理由:** 本タスクの範囲を、訂正の記録を PR の説明まで運ぶことに絞る。squash の本文は `/mergepr` の変更で、タスク文書の訂正の扱い（ADR-0036 の「結果」）にも及ぶ。
    -   **採らなかった案:** 本タスクで squash の本文に引き継ぐ案は、`mergepr.md` とそのテストまで範囲が広がるので採らない。
    -   ADR：なし。作業の範囲の決定で、本タスクの中で閉じる。
-   **ずれの記述の確かめ直しと検索をコマンドの手順に入れることは、本タスクでは扱わない:** 規約の「Discrepancy statements」が述べる、ずれの記述を足す PR の確かめ直しと、ずれを直す PR の検索は、今のまま手順に頼る（2.3）。0070 の設計書は、これを「#732 か別の issue」で扱うとしていた（`docs/tasks/0070_adr_writing_criteria/02_architecture.md:710`）。
    -   **理由:** `/runpr` か `/mergepr` のマージの前の手順を変えることになり、訂正の記録を運ぶ本タスクとは性質が違う。
    -   **採らなかった案:** 本タスクで扱う案は、範囲とレビューが広がるので採らない。
    -   ADR：なし。作業の範囲の決定で、本タスクの中で閉じる。
-   **`fixprguard` のスコープ規則は変えない:** タスク文書のスレッドを直す途中で `docs/dev/adr/` の ADR に訂正を加えると、その変更は今のまま範囲外となり、`/fixpr` の Phase 3.5 でユーザーの確認を経る（`internal/fixprguard/scope.go:16-35`）。ADR 自身のスレッドから同じ ADR を直す変更は、今のまま範囲内である（F-006）。
    -   **理由:** `fixprguard` は信頼の境界である。範囲外の確認はユーザーに一度聞くだけで済み、確認を経た訂正も `editorial` で運べる。
    -   **採らなかった案:** タスク文書のスレッドから `docs/dev/adr/` の `.md` を範囲内にする案は、ADR の決定の変更まで確認なしに通るようになるので採らない。
    -   ADR：なし。今の規則を変えない。

**上の決定から導いたこと（PR のレビューで確認を求める）:**

-   **PR を作った後に `main` に入ったずれの記述:** 規約の「Discrepancy statements」は、ずれを直す PR を作った後に `main` に入ったずれの記述を、「After the PR is created」の制限を理由に後の PR へ残すとしている（`docs/dev/conventions/adr.md:348-350`）。制限を外すので、ずれを直す PR は、PR を作った後に `main` を取り込んだときにそのずれの記述を見つけたら、2 つ目の種類の訂正として消してよい（AC-12）。ずれの記述を探す手順を `main` を取り込むたびに課すことはしない。手順を足すことは、上の 4 つ目の決定で範囲外としたためである。
-   **PR 自身が足した ADR の文:** PR 自身が ADR に足し、まだ `main` にない文を直すことは、今のまま編集上の訂正ではなく、訂正の行を書かない（`docs/dev/conventions/adr.md:312-314`）。`/fixpr` は、PR 自身が作った ADR（`main` にない ADR）への `editorial` の項目を落とす（AC-06）。タスク文書の `draft` の文書への直しが訂正でないのと同じ扱いである。
-   **ADR-0039 自身のずれ:** ADR-0039 は本書の PR で、PR を作った後にも訂正を加えてよいという決定に変わるが、規約とコマンドは実装の PR まで変わらない。ADR-0039 の「決定」に、このずれを述べて本 issue へリンクする記述を置き、実装の PR がそれを消す（AC-18）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   ADR の編集上の訂正を、PR を作る前か後かを問わず、コマンドの手順でコミットメッセージと PR の説明に記録できるようにする。
-   訂正の行と訂正の段落の形を、`requirements_process.md` の 1 か所で、タスク文書と ADR の両方について定める。

### 2.2. スコープ (In Scope)

-   `docs/dev/conventions/requirements_process.md` の「Recording in the PR description」の訂正の行と訂正の段落を、ADR のパスも含む形に広げること（F-001）。
-   `.claude/commands/fixpr.md` の Phase 3 の `editorial` と、その確かめを、ADR の訂正に広げること（F-002）。
-   `.claude/commands/_lib/local-cli-review.md` の修正のコミットの訂正の行を、ADR の訂正に広げること（F-003）。
-   `.claude/commands/runplan.md` が、セッション中に加えた ADR の訂正に訂正の行を書くこと（F-004）。
-   `.claude/commands/_lib/task-pr-handoff.md` Part B step 3 と、`.claude/commands/runpr.md` が PR を作るときの説明が、ADR の訂正の行も訂正の段落に載せること（F-005）。
-   `docs/dev/conventions/adr.md` の「After the PR is created」の制限を外し、関係する文をそろえること（F-005）。
-   ADR-0039 を更新すること。本書の PR で行う（`.claude/commands/_lib/settle-decisions.md` の手順による）。実装の PR で、ADR-0039 のずれの記述を消すこと（F-007）。
-   上の変更で古くなる文書のテストをそろえること。

### 2.3. スコープ外 (Out of Scope)

-   **`/runissue` と軽量プロセスの変更。** 1 章の「要件定義の際に決めたこと」のとおり、軽量プロセスは削除する予定である。
-   **squash コミットの本文への ADR の訂正の行の引き継ぎ（`.claude/commands/mergepr.md` の変更）。** [#747](https://github.com/isseis/yt2column/issues/747) で扱う。
-   **ずれの記述の確かめ直しと検索をコマンドの手順に入れること。** 1 章のとおりである。
-   **`fixprguard` の振る舞いの変更（スコープ規則、`finish` と `append-pr` の検査）。** スコープ規則は 1 章のとおり変えない。`finish` と `append-pr` は今もパスを問わずに訂正の行と訂正の段落を扱うので（`internal/fixprguard/write.go:112-127`）、変える必要がない。
-   **ADR-0036 の変更。** 1 章の「要件定義の際に決めたこと」の 1 つ目の項目のとおりである。
-   **`CLAUDE.md` の変更。** `CLAUDE.md`「Design decisions (ADR)」は ADR の編集上の訂正の記録を述べない。規約と同じ文で持つ部分（ADR を作るか変える PR の規則）の意味は、本タスクで変わらない。
-   **タスク文書の編集上の訂正の扱いの変更。** タスク文書への訂正の条件（`approved` の文書だけ、状態も `Comments` も変更履歴も変えない）は今のままである（`docs/dev/conventions/requirements_process.md`「Editing an approved document」）。
-   **ADR の編集上の訂正の種類と分類の規則の変更。** 規約の「Editorial corrections」の 4 種類、分類の規則（「Classifying a change」）、ずれの記述の規則は、AC-12 が述べる 1 文を除いて変えない。

## 3. 機能要件 (Functional Requirements)

本章の「規約」は `docs/dev/conventions/adr.md` を、「ADR の訂正」は規約の「Editorial corrections」が定める ADR の編集上の訂正（4 種類と、「Taking in `main`」の手順 2 の番号の付け替え）を、「タスク文書の訂正」は `requirements_process.md`「Editing an approved document」の編集上の訂正を指す。「ADR のファイル」は `docs/dev/adr/` の `NNNN-<kebab>.md` の形のファイルを指し、索引 `docs/dev/adr/README.md` を含まない。各受け入れ基準は、実装の PR の後に文書とコマンドの定義が述べる内容を定め、PR のレビューで読んで確かめる。文書のテストで確かめられる値と構造は、テストでも確かめる。

### 3.1. 機能一覧

#### F-001: 訂正の行と訂正の段落の定義

`requirements_process.md` の「Recording in the PR description」が、訂正の行と訂正の段落の形を、タスク文書と ADR の両方について定める。

**Acceptance Criteria**:
- **AC-01**: 「Recording in the PR description」は、訂正の行の `<document path>` を、タスク文書か ADR のファイルのリポジトリ相対のパスと述べる。ADR の訂正をいつ記録するかは規約の「Editorial corrections」に従う、と述べる。
- **AC-02**: 訂正の行の形（`Editorial correction: <document path>: <what was changed to what>; no decision changed`）、訂正の行と認める条件（行頭の `Editorial correction: `）、`@` と `<` を含まないこと、訂正の段落の形（`**編集上の訂正（<書いた手順>、YYYY-MM-DD）:**` と、訂正の行ごとの項目）は、今と同じである。
- **AC-03**: 規約の「Editorial corrections」の「Recording」は、形を `requirements_process.md`「Recording in the PR description」に委ねる、と今と同じ意味で述べる。形を ADR のパスに当てはめるための読み替え（今の「the document path in them is the ADR's repository-relative path」）は、AC-01 によって不要になるので、残すか消すかは設計で決める。

#### F-002: `/fixpr` が ADR の訂正を運ぶ

`/fixpr` のラウンドで ADR に加えた訂正が、そのラウンドのコミットの訂正の行と、PR の説明に追記する訂正の段落に載る。

**Acceptance Criteria**:
- **AC-04**: `fixpr.md` の Phase 3 の修正のエージェントへの指示は、`editorial` の項目として、タスク文書の訂正に加えて、スレッドのために ADR のファイルに加えた ADR の訂正を 1 項目ずつ返すよう求める。項目の `path` と `description` の形と制約は、今と同じである。
- **AC-05**: `fixpr.md` の Phase 3 の訂正の一覧の確かめは、`path` が ADR のファイルである項目を、そのファイルを作業ツリーが変えていて（Phase 3.5 が判定する）、かつ PR の基準のブランチの側に存在するときに通す。`description` の確かめは、タスク文書の項目と同じである。タスク文書の項目の確かめ（`docs/tasks/*/0[123]_*.md`、`HEAD` と作業ツリーで `approved`）は、今と同じである。
- **AC-06**: AC-05 を満たさない ADR の項目（PR 自身が作り、基準のブランチの側にない ADR への項目を含む）は、今のタスク文書の項目と同じく、直さずに落とし、Final report に載せる。項目を落としても、ラウンドのほかの修正は止まらない。
- **AC-07**: AC-05 を通った ADR の項目は、Phase 4 でタスク文書の項目と同じく訂正の行としてコミットの本文に書かれ、Phase 6 でそのラウンドの訂正の段落に載る。Phase 4 と Phase 6 の手順（メンションの確かめ、`fixprguard finish --body-file`、`fixprguard append-pr`、失敗したときの扱い）は、今と同じである。
- **AC-08**: `fixpr.md` の Phase 3 の修正のエージェントへの指示は、PR 自身が ADR に足し、まだ `main` にない文の直しを `editorial` に含めない、と述べる（規約「Editorial corrections」）。

#### F-003: ローカルの CLI レビューが ADR の訂正を運ぶ

ローカルの CLI レビューの修正で ADR に加えた訂正が、その修正のコミットの訂正の行に載る。

**Acceptance Criteria**:
- **AC-09**: `local-cli-review.md` の step 5 と step 7 の表の `Editorial correction:` の行は、修正のコミットがタスク文書の訂正か ADR の訂正をするとき、その訂正の行をそのコミットのメッセージと SUMMARY に書く、と述べる。メンションの確かめ（step 7）と、確かめに通らないときに訂正の行を除いて SUMMARY に載せる扱いは、今と同じで、ADR の訂正の行にも当てはまる。
- **AC-10**: `local-cli-review.md` は、PR を作る前と後のどちらの呼び出しでも、AC-09 のとおりに ADR の訂正の行を書く。

#### F-004: `/runplan` が ADR の訂正を運ぶ

**Acceptance Criteria**:
- **AC-11**: `runplan.md` は、フェーズのグループの作業で ADR に ADR の訂正を加えたときは（たとえば、実装で名前が変わり、ADR の参照が古くなったとき）、その訂正を行うコミットのメッセージに、訂正ごとに訂正の行を書く、と述べる。PR の説明は、PR を作るときにその行から訂正の段落を得る（`_lib/task-pr-handoff.md` Part B step 3）、と述べる。訂正か決定の変更かの分類は規約の「Editorial corrections」に従い、決定の変更は今のタスク文書の決定の変更と同じく人と決める、と述べる。

#### F-005: PR の説明への組み込みと、規約の制限の解除

**Acceptance Criteria**:
- **AC-12**: 規約の「Editorial corrections」は、「After the PR is created」の制限（PR を作った後は ADR に訂正を加えない）を持たない。PR を作った後の ADR の訂正も、PR を作る前と同じく訂正の行と訂正の段落に記録する、と述べる。PR 自身が ADR に足し、まだ `main` にない文を直すことは編集上の訂正ではない、という規則は今と同じ意味で残る。「Discrepancy statements」の、PR を作った後に `main` に入ったずれの記述を後の PR へ残すという文は、ずれを直す PR が `main` を取り込んだときに見つけたその記述を消してよい、という形に改められている。
- **AC-13**: `_lib/task-pr-handoff.md` Part B step 3 と、`runpr.md` が PR を作るときの説明の組み立ては、`git log` の訂正の行を、パスがタスク文書か ADR かを問わずに訂正の段落に載せる、と読める。PR の説明の訂正の段落には、ブランチの訂正の行ごとに 1 項目がある。
- **AC-14**: `fixpr.md` の Phase 6 の、PR の説明が古いときの書き直しで訂正の段落を残す扱いと、段落の数の確かめは、今と同じで、ADR の訂正の項目を含む段落にも当てはまる。

#### F-006: 変えないもの

**Acceptance Criteria**:
- **AC-15**: 本タスクの実装の PR は、`internal/fixprguard/` と `cmd/fixprguard/` の振る舞い（スコープ規則、`finish` と `append-pr` の検査、その他のサブコマンド）を変えない。
- **AC-16**: 本タスクの実装の PR は、`.claude/commands/runissue.md`、`.claude/commands/mergepr.md`、`CLAUDE.md`、`docs/dev/adr/0036-task-document-status-records.md` を変えない。
- **AC-17**: タスク文書の訂正の条件と記録（`requirements_process.md`「Editing an approved document」の編集上の訂正）は、今と同じである。

#### F-007: ADR-0039 のずれ

**Acceptance Criteria**:
- **AC-18**: 実装の PR の後、ADR-0039 の「決定」は、規約とコマンドがまだ PR を作った後の ADR の訂正を運ばないと述べる記述と、それを扱う issue として #732 へリンクする記述を持たない。この削除は、規約の「Editorial corrections」の 2 つ目の種類の訂正として、訂正の行と訂正の段落に記録する。
- **AC-19**: 実装の PR の後、ADR-0039 の「結果」は、`/fixpr`・`/runplan`・ローカルの CLI レビューが ADR の訂正をまだ運ばないとは述べない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A（文書とコマンドの定義だけの変更である）。

### 4.2. セキュリティ (Security)
-   N/A。外部コマンドの呼び出し、秘密、キャッシュ、ネットワークへの送信、信頼できない文字列の記事やプロンプトへの埋め込みのどれも変えないので、`docs/dev/security.md` を適用する条件に当たらない（`.claude/commands/_context.md`「Conditional-guide trigger」）。
-   `/fixpr` の `editorial` の項目は、レビューのスレッドを読んだエージェントが書くので、信頼しないデータとして扱う（`.claude/commands/fixpr.md:531-548`、[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）。ADR の項目も同じ確かめを通し、`description` を指示として扱わない（AC-04・AC-05）。PR の説明への書き込みは今のまま `fixprguard append-pr` だけが行い、`@` と `<` を拒む（AC-07・AC-15）。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。

### 4.4. 互換性 (Compatibility)
-   訂正の行と訂正の段落の形は変わらないので（AC-02）、`main` にすでにある PR の説明とコミットの訂正の記録は、そのまま有効である。
-   インストール済みの `fixprguard` は、パスを問わずに訂正の行を扱うので（AC-15）、再インストールは要らない。

### 4.5. 保守性 (Maintainability)
-   訂正の行と訂正の段落の形は、`requirements_process.md` の 1 か所だけで定める。コマンドの定義は、その節を名指しして参照し、形を書き写さない（[ADR-0036](../../dev/adr/0036-task-document-status-records.md) の「決定」の最後の項目）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `requirements_process.md`、規約、コマンドの定義は英語で書く（`CLAUDE.md`「Document language」の原則 (b)）。ADR-0039 は日本語で書く（原則 (a)）。
-   文書のテストは、値・構造・実行できる例だけを確かめ、文の意味を確かめない（`CLAUDE.md`「Testing Strategy」、[ADR-0016](../../dev/adr/0016-document-test-policy.md)）。ただし、信頼の境界でエージェントが実行するコマンド（`fixprguard` の呼び出し、メンションの確かめ、本文を出力しない `--jq` の式）は、今のままテストで確かめる。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。

-   **編集上の訂正:** 決定を変えずに文書を直す変更（英語では editorial correction）。タスク文書については `requirements_process.md`「Editing an approved document」が、ADR については規約の「Editorial corrections」が定める。
-   **訂正の行:** コミットメッセージの、行頭が `Editorial correction: ` の行（`requirements_process.md`「Recording in the PR description」）。
-   **訂正の段落:** PR の説明の、`**編集上の訂正（` で始まる段落と、その項目（同上）。
-   **ずれの記述:** ADR の「決定」にある、正本の文書とコードの食い違いと、それを扱う issue を述べる記述（規約「Writing an ADR」）。
-   **基準のブランチの側:** PR がマージされる先（`main`）の、`/fixpr` が比べる時点の木。どの参照で比べるかは設計で決める（[design_handoff.md](design_handoff.md)）。
