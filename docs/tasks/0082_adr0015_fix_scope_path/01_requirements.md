# 要件定義書：ADR-0015 の修正の範囲の決定を、スレッドの `path` そのものの扱いでガイドとコードに合わせる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-10 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ADR-0015（`docs/dev/adr/0015-untrusted-review-threads.md`）の「決定」のうち修正の範囲の項目を、スレッドの `path` そのものの扱いについて、正本のガイドとコードに合わせる要件を定義する。

**背景:**

-   ADR-0015 の「決定」の修正の範囲の項目は、「修正の範囲は、スレッドの `path` と同じディレクトリと、その `path` があるタスクのディレクトリのうち、許可リストに当たるファイルに限る。」と述べる（コミット `6191c96a` の `docs/dev/adr/0015-untrusted-review-threads.md:20`）。`path` そのものも `path` と同じディレクトリにあるので、この文に従えば、許可リストに当たらない `path`（たとえば `.sh` のファイル）は範囲外になる。
-   一方、正本の `docs/dev/guides/fixprguard_guide.md` 6 章「範囲の規則」は、規則 1 を「`path` そのもの。」とし、「規則 1 には許可リストを当てない。実行関連ファイルでないファイルは、`path` そのものなら種類を問わず範囲に入る。」と述べる（`docs/dev/guides/fixprguard_guide.md:309`・`:316`）。7.2.3「変更範囲」も、スレッドの `path` が `.sh` のファイルならその書き換えは範囲内である、と述べる（`docs/dev/guides/fixprguard_guide.md:415`）。コードも、許可リストを規則 2・3 にだけ当てる（`internal/fixprguard/scope.go:27-29`）。
-   この扱いは、タスク 0036 が決めたものである。0036 は、規則 1 に許可リストを適用しないことを AC-10 とし、規則 1 の変更をスコープ外とした（`docs/tasks/0036_fixprguard_exec_file_scope/01_requirements.md:190`・`:130`）。
-   ADR-0015 のこの文は、タスク 0060 が ADR を作ったとき（PR #685、2026-11-05）に入った（コミット `673f674e` の `docs/dev/adr/0015-untrusted-review-threads.md:22`）。0036 の決定を要約するときに、規則 1 の扱いが落ちたものである。
-   タスク 0073 の ADR の見直し（PR #807）がこのずれを見つけた。書き換えると決定の意味が変わるので、0073 は元の文を残し、issue #811 に記録した（`docs/tasks/0073_adr_context_decision_review/review_s6.md:45`）。

対応 issue: [#811](https://github.com/isseis/yt2column/issues/811)（関連: [#729](https://github.com/isseis/yt2column/issues/729)（タスク 0073）、[#425](https://github.com/isseis/yt2column/issues/425)（タスク 0036））

**本書の記述範囲:** 本書は、直した後の ADR-0015 が述べること、ADR-0015 のほかの部分とほかのファイルを変えないこと、「採らなかった案」と「経緯」への記録を定める。直しは決定の変更なので、ADR-0015 の書き換えは本書の PR で行う（下の「要件定義の際に決めたこと」）。確かめ方は `02_architecture.md` と `03_implementation_plan.md` で決める。

**issue #811 で決めたこと:**

-   **ADR-0015 の修正の範囲の決定と、正本のガイド・コードとのずれを直す。** 直し方（決定をコードに合わせるか、コードを決定に合わせるか）は、直す変更で決める。
    -   ADR：ADR-0015 を更新する。どちらの直し方でも決定の変更になる。

**要件定義の際に決めたこと（2026-11-10）:**

-   **ADR-0015 の決定を、コードとガイドに合わせる（issue の案 1）。** 修正の範囲の項目を、「スレッドの `path` そのもの（許可リストを当てない）と、`path` と同じディレクトリおよびその `path` があるタスクのディレクトリのファイルのうち許可リストに当たるもの」の形に書き換える。そのうえで、`path` そのものは実行関連ファイルでなければ名前や拡張子を問わず範囲に入れることと、その理由を書く（F-001）。理由は、スレッドが直すよう求めているのはそのファイルであり、許可リストを当てると `.ts` や `.sh` のファイルへの指摘を直すたびに利用者の確認が入るためである。動いている規則はタスク 0036 が決めたもので、ガイドとコードはすでにそれに従っている。ずれているのは ADR の要約だけである。
    -   採らなかった案：コードを ADR に合わせ、規則 1 にも許可リストを当てる案（issue の案 2）。スレッドが指す `.ts` や `.sh` のファイルを直すたびに利用者の確認が入り、`/runpr` の繰り返しのたびに利用者を待たせる。`fixprguard` の変更、`--protocol` の更新、ガイドの改訂も要る。
    -   ADR：ADR-0015 を本書の PR で更新する（「決定」の修正の範囲の項目、「採らなかった案」の項目 1 つ、「経緯」の項目 1 つ）。新しい ADR は作らない。
-   **この直しは編集上の訂正とせず、決定の変更として扱う。ADR-0015 は本書の PR で直し、「経緯」の最後に項目を 1 つ足す（F-003）。** 書き換えは「決定」の文の意味を変える。規約は、決定の意味を変える変更を編集上の訂正としない（`docs/dev/conventions/adr.md`「Editorial corrections」）。決定の変更は、その決定をする PR が ADR を変える（`docs/dev/conventions/adr.md`「Approval and the PR」）。
    -   ADR：上の項目と同じ。
-   **「採らなかった案」に、規則 1 にも許可リストを当てる案を、理由とともに加える（F-002）。** 規約は、決定を変えた文書が理由を述べるときに限り、過去の決定を「採らなかった案」にも載せるとする（`docs/dev/conventions/adr.md`「Changing, retiring, and withdrawing」）。本書は上の項目で理由を述べている。載せておけば、同じ案を再び検討することも防げる。
    -   ADR：上の項目と同じ。
-   **進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。** 変わるのは ADR-0015 の 1 項目と、「採らなかった案」「経緯」への項目の追加だけで、関わるコンポーネントもコマンドもない。開いている決定は少なく、新しい仕組みも作らない。ADR-0015 の「決定」の文は変わるが、規則そのものはタスク 0036 が決めたままであり、変わらない。除外の判断：ADR の記述を今のコードとガイドに合わせるだけで、`fixprguard`・`codexreview`・`mergepr` を変えず、動いている信頼境界を変えない。`docs/dev/security.md` の規則と前提にも、外部依存にも触れない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   ADR-0015 の修正の範囲の決定が、正本のガイドとコードと食い違わないようにする。

### 2.2. スコープ (In Scope)

-   ADR-0015 の「決定」の修正の範囲の項目の書き換え（F-001）。
-   ADR-0015 の「採らなかった案」への、項目 1 つの追加（F-002）。
-   ADR-0015 の「経緯」への、この直しの項目 1 つの追加（F-003）。

### 2.3. スコープ外 (Out of Scope)

-   コード（`internal/fixprguard/scope.go`）とガイド（`docs/dev/guides/fixprguard_guide.md`）の変更。どちらもすでにタスク 0036 の決定に従っている（1 章の背景）。
-   ADR-0015 のほかの部分と、ほかの ADR の変更。issue #811 が扱うのはこの 1 項目だけである。`docs/dev/` と指示ファイルの中で、修正の範囲の許可リストを要約した文はこの 1 か所だけである（コミット `6191c96a` で `git grep -n '許可リストに当たる' -- docs CLAUDE.md .claude` の出力のうち、タスクの文書を除くと ADR-0015 の 1 行と、規則 3 を述べるガイドの 1 行だけ）。
-   ADR の索引（`docs/dev/adr/README.md`）の変更。ADR-0015 の題名は変わらない。
-   ずれの記述の削除。`origin/main` の ADR に、issue #811 にリンクしたずれの記述はない（コミット `6191c96a` で `git grep -nE 'issues/811([^0-9]|$)' -- docs/dev/adr/` の出力は空）。
-   ADR-0015 を、今の ADR の書き方の基準（`docs/dev/conventions/adr.md`「Writing an ADR」）にそろえ直すこと。issue #811 の対象ではない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 「決定」の修正の範囲の項目を、ガイドとコードに合わせる

ADR-0015 の「決定」の修正の範囲の項目を、スレッドの `path` そのものには許可リストを当てない形に書き換え、その理由を書く。

**Acceptance Criteria**:
- **AC-01**: ADR-0015 の「決定」の修正の範囲の項目は、次を述べる。
    -   修正の範囲は、スレッドの `path` そのもの（許可リストを当てない）と、`path` と同じディレクトリおよびその `path` があるタスクのディレクトリのファイルのうち許可リストに当たるもの、である。
    -   `path` そのものには許可リストを当てず、実行関連ファイルでなければ名前や拡張子を問わず範囲に入る。
    -   `path` そのものに許可リストを当てない理由。
- **AC-02**: ADR-0015 の「決定」の差分は、修正の範囲の項目の 1 行の書き換えに限る。その 1 行の中の差分は、先頭の太字の文の書き換えと、許可リストの中身を述べる文の直後への文の挿入に限る。

#### F-002: 「採らなかった案」に、規則 1 にも許可リストを当てる案を加える

**Acceptance Criteria**:
- **AC-03**: ADR-0015 の「採らなかった案」の最後に、項目がちょうど 1 つ足されている。その項目は、規則 1（スレッドの `path` そのもの）にも許可リストを当てる案と、それを採らない理由を述べ、本タスクの要件定義書（`../../tasks/0082_adr0015_fix_scope_path/01_requirements.md`）にリンクする。

#### F-003: 直しを決定の変更として「経緯」に記録する

**Acceptance Criteria**:
- **AC-04**: ADR-0015 の「経緯」の最後に、項目がちょうど 1 つ足されている。その項目は `docs/dev/conventions/adr.md`「History entries」の形（日付、その時点の変更の要旨、出典の文書へのリンク）に従う。要旨は、修正の範囲の決定を、規則 1 に許可リストを当てない形にし、タスク 0036 の決定と道具の実装に合わせたことを述べる。リンク先は本タスクの要件定義書である。

#### F-004: ほかの部分とほかのファイルを変えない

**Acceptance Criteria**:
- **AC-05**: ADR-0015 の差分は、AC-02 の 1 行の書き換え、AC-03 の 1 行の追加、AC-04 の 1 行の追加に限る。
- **AC-06**: ADR-0015 と本タスクのディレクトリ（`docs/tasks/0082_adr0015_fix_scope_path/`）の外のファイルは、ADR の索引（`docs/dev/adr/README.md`）とほかの ADR を含め、変更の前と同じである。その外では、ファイルの追加も削除もない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文書の変更だけで、実行時の処理を変えない。

### 4.2. セキュリティ (Security)
-   N/A。直すのは ADR の記述だけで、`fixprguard` の範囲の規則・コード・`docs/dev/security.md` の規則と前提を変えない。規則 1 で範囲に入るファイルの残余のリスクは、ガイドの 7.2.3 がすでに述べている（`docs/dev/guides/fixprguard_guide.md:415-416`）。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。文書の変更だけである。

### 4.4. 互換性 (Compatibility)
-   N/A。文書の変更だけである。

### 4.5. 保守性 (Maintainability)
-   ADR の検査 `TestADRRecords`（`cmd/yt2column/adr_test.go:434`）を含む文書のガードが、変更の後も通る。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR の直しは、`docs/dev/conventions/adr.md` の決定の変更の規則（「Approval and the PR」「History entries」「Changing, retiring, and withdrawing」）に従う。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **規則 1:** `fixprguard` の範囲の規則のうち、スレッドの `path` そのものを範囲に入れる規則（`docs/dev/guides/fixprguard_guide.md` 6 章）。
-   **許可リスト:** 範囲の規則の規則 2・3 で範囲に入りうるファイルの種類（`*.md`・`*.txt`・`testdata/` の下のファイルと、`.go` の規則に当たるファイル。`docs/dev/guides/fixprguard_guide.md` 6 章）。
-   **実行関連ファイル:** スレッドの著者と `path` にかかわらず範囲外とするファイル（拒否リスト。`docs/dev/guides/fixprguard_guide.md` 6 章）。
-   **決定の変更:** 編集上の訂正に当たらない ADR の変更。その決定をする PR が ADR を変え、「経緯」に項目を足す（`docs/dev/conventions/adr.md`「Approval and the PR」「History entries」）。
