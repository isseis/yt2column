# 要件定義書：ADR-0024「結果」の、サーバが環境から読む変数の記述の更新

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

**目的:** 本書は、ADR-0024（`docs/dev/adr/0024-env-access-in-config.md`）の「結果」の最後の項目を、今のコードに合わせる要件を定義する。

**背景:**

-   ADR-0024 の「結果」の最後の項目は、「[ADR-0018](../../dev/adr/0018-server-config-file.md) は、サーバが設定ファイルだけから設定を読み、環境からは `HOME` と `GODEBUG` だけを読むと決めている。この決定はまだコードに入っていない。実装されると、サーバが環境から読むのはその 2 つだけになる。」と述べる（`docs/dev/adr/0024-env-access-in-config.md:35`）。
-   ADR-0018 の決定は、タスク 0066 の実装の PR（#735・#740）で `main` に入った。サーバの `main` は `os.LookupEnv` を設定の読み込みに渡すだけである（`cmd/yt2column-server/main.go:32`）。`internal/config` の `LoadServer` は、その関数で `HOME`（設定ファイルとソケットのパスの導出。`internal/config/config.go:361`・`internal/config/config.go:378`）と `GODEBUG`（HTTP/2 のデバッグログの警告。`internal/config/config.go:391`）だけを読み、LLM と Webhook の設定を設定ファイルから読む（`internal/config/config.go:370-371`）。`docs/dev/security.md` §8.2 と `docs/dev/project_overview.md`「設定（環境変数）」も、この今の状態を述べている。
-   そのため、「この決定はまだコードに入っていない」という記述は、今のコードと食い違う。タスク 0073（#729）の PR-2 が見つけた。タスク 0073 は ADR の「結果」を変えないので、規約（`docs/dev/conventions/adr.md`「Editorial corrections」の「Discrepancy statements」）に従い、ADR を直さずに issue #776 を立てた。

対応 issue: [#776](https://github.com/isseis/yt2column/issues/776)（関連: [#729](https://github.com/isseis/yt2column/issues/729)（タスク 0073）、[#576](https://github.com/isseis/yt2column/issues/576)（タスク 0066））

**本書の記述範囲:** 本書は、書き直した後の項目が述べること、ADR-0024 のほかの部分を変えないこと、直しの分類と記録を定める。書き直す文の言い回しと確かめ方は `02_architecture.md` と `03_implementation_plan.md` で決める。

**issue #776 で決めたこと:**

-   **ADR-0024 の「結果」の最後の項目を、ADR-0018 の決定が実装された今の状態に合わせて書き直すか、消す。** 今のコードと食い違うためである。
    -   ADR：ADR-0024 を直す。直しが編集上の訂正か決定の変更かは、下の「要件定義の際に決めたこと」で決める。
-   **編集上の訂正か決定の変更かは、直す PR で規約の「Editorial corrections」に照らして決める。**

**要件定義の際に決めたこと（2026-11-10）:**

-   **進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。** 直すのは ADR の 1 項目だけで、関わるコンポーネントもコマンドもない。開いている決定は少なく、どの ADR の決定も変えず、新しい仕組みを作らない。除外の判断：`docs/dev/security.md` の規則と前提（脅威モデル）を変えず、ADR の記述を今のコードに合わせるだけである。開発ツールの信頼境界にも、外部依存にも触れない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   **項目は消さずに書き直す。ADR-0018 へのリンクを残したまま、サーバのコードがプロセスの環境から読むのは `HOME` と `GODEBUG` だけであることを、今の事実として述べる（F-001）。** ADR-0024 の「決定」は ADR-0018 にリンクしていない。この項目を消すと、ADR-0024 の読み手は、サーバの入口が渡す関数で実際に読まれる変数までたどれなくなる。また、この事実は ADR-0018 の「決定」と `docs/dev/security.md` §8.2 がすでに述べており、書き直しは新しい事実を足さない。
    -   採らなかった案：項目を消す案。同じ事実は ADR-0018 の「決定」にあるので、重複はなくなる。しかし、ADR-0024 から ADR-0018 へのつながりが失われる。
    -   ADR：ADR-0024 を編集上の訂正で直す（次の項目）。決定は変わらない。
-   **この直しは編集上の訂正として扱う。「経緯」に項目を足さず、直したコミットのメッセージに訂正の行を、PR の説明に訂正の段落を書く（F-002）。** 直した後の記述は ADR-0024 の「決定」と食い違わない。直しは、古くなった「未実装」の記述を、ADR-0018 と `docs/dev/security.md` がすでに持つ内容に合わせるだけである。規約の「Editorial corrections」の種類 1（基準に合わせた書き直しで、新しい事実を足さないもの）と種類 3（参照先と合わなくなった記述の修正）に当たる。PR のレビュアーが分類に異を唱えたときは、規約に従い決定の変更として扱う。
    -   採らなかった案：決定の変更として扱い、「経緯」に項目を足す案。ADR-0024 の決定は変わらないので、経緯に足す出来事がない。
    -   ADR：ADR-0024 の「決定」は変えない。新しい ADR は作らない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   ADR-0024 の「結果」が、今のコードと食い違わないようにする。

### 2.2. スコープ (In Scope)

-   ADR-0024 の「結果」の最後の項目の書き直し。

### 2.3. スコープ外 (Out of Scope)

-   ADR-0024 のほかの部分と、ほかの ADR の変更。issue #776 が扱うのはこの 1 項目だけである。ADR-0018 と ADR-0003 には、未実装を前提にする記述がない（`grep -n "まだ\|実装され"` で確かめた）。
-   コードと、`docs/dev/security.md`・`docs/dev/project_overview.md` の変更。どちらもすでに今の状態を述べている（1 章の背景）。
-   ADR-0024 を、今の ADR の書き方の基準（`docs/dev/conventions/adr.md`「Writing an ADR」）にそろえ直すこと。issue #776 は求めていない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 「結果」の最後の項目を今の状態に合わせる

ADR-0024 の「結果」の最後の項目を、ADR-0018 の決定が実装された今の状態を述べる文に書き直す。

**Acceptance Criteria**:
- **AC-01**: ADR-0024 の「結果」の最後の項目は、ADR-0018 へのリンクを持ち、サーバのコードがプロセスの環境から読むのは `HOME` と `GODEBUG` だけであることを、今の事実として述べる。ADR-0018 の決定がまだコードに入っていないことや、これから実装されることを述べる記述を含まない。
- **AC-02**: ADR-0024 のうち、「結果」の最後の項目より外の部分は、変更の前と同じである。題名、「状態」「背景」「決定」「採らなかった案」「経緯」の各節、「結果」のほかの 3 項目と、項目の数と順序が変わらず、「経緯」に項目が足されない。
- **AC-03**: ADR の索引（`docs/dev/adr/README.md`）とほかの ADR は、変更の前と同じである。

#### F-002: 直しを編集上の訂正として記録する

**Acceptance Criteria**:
- **AC-04**: F-001 の直しを含むコミットのメッセージに、ADR-0024 のパス（`docs/dev/adr/0024-env-access-in-config.md`）を名指す訂正の行が 1 行あり、その形は `docs/dev/conventions/requirements_process.md`「Recording in the PR description」の訂正の行に従う。
- **AC-05**: F-001 の直しを含む PR の説明に、AC-04 の訂正の行を日本語にした項目を持つ訂正の段落がある。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文書の変更だけで、実行時の処理を変えない。

### 4.2. セキュリティ (Security)
-   N/A。直すのは ADR の記述だけで、コード・秘密情報の扱い・`docs/dev/security.md` の規則と前提を変えない。書き直す記述の事実は、`docs/dev/security.md` §8.2 がすでに述べている。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。文書の変更だけである。

### 4.4. 互換性 (Compatibility)
-   N/A。文書の変更だけである。

### 4.5. 保守性 (Maintainability)
-   ADR の検査 `TestADRRecords`（`cmd/yt2column/adr_test.go`）を含む文書のガードが、変更の後も通る。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR の直しは、`docs/dev/conventions/adr.md`「Editorial corrections」の編集上の訂正の規則に従う。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **編集上の訂正:** ADR の決定を変えずに書き方を直すこと（`docs/dev/conventions/adr.md`「Editorial corrections」）。
-   **訂正の行・訂正の段落:** 編集上の訂正を、コミットメッセージと PR の説明に記録する形（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」）。
