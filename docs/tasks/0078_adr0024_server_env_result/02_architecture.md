# 設計書：ADR-0024「結果」の、サーバが環境から読む変数の記述の更新

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-10 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 設計の全体像

本タスクは、ADR-0024（`docs/dev/adr/0024-env-access-in-config.md`）の「結果」の最後の項目 1 つを、文書の上で書き直すだけである（[01_requirements.md](01_requirements.md) F-001・F-002）。コードは変えない。

**設計原則:**

-   **書き直した文は、出典の文書がすでに持つ事実だけを述べる。** 事実の出典は、[ADR-0018](../../dev/adr/0018-server-config-file.md) の「決定」と `docs/dev/security.md` §8.2 である。新しい事実を足すと、編集上の訂正の種類 1 に当たらなくなる（`docs/dev/conventions/adr.md`「Editorial corrections」）。
-   **ADR-0024 のほかの部分には触れない（AC-02・AC-03）。** 変更を 1 項目に限ると、編集上の訂正であることをレビュアーが diff で確かめやすい。

**設計の際に決めたこと（2026-11-10）:**

-   要件が開いたままにした設計の選択はない。書き直した文の形は、要件定義の際に決めたことで決まっている（[01_requirements.md](01_requirements.md) 1 章）。言い回しは `03_implementation_plan.md` に、変更の前と後の全文で書く。

概念モデルの Mermaid の図は省く。変わるのは 1 つの文書の 1 項目だけで、コンポーネントの関係がない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

変更する文書は 1 つである。

| 文書 | 変更 |
|---|---|
| `docs/dev/adr/0024-env-access-in-config.md` | 「結果」の最後の項目を書き直す（F-001） |

書き直した項目は、次の 3 つを述べる。

-   ADR-0018 へのリンク（今の項目にあるリンクを残す）。
-   ADR-0018 の決定により、サーバのコードがプロセスの環境から読むのは `HOME` と `GODEBUG` だけであること（AC-01）。
-   その事実の正本である `docs/dev/security.md` §8.2 へのリンク。ADR は、コードの振る舞いを述べるとき、正本の文書にリンクする（`docs/dev/conventions/adr.md`「Writing an ADR」の「Links」）。

既存のテストで、この項目の文を確かめるものはない。`TestADRRecords`（`cmd/yt2column/adr_test.go`）は、ADR のファイル名・節の見出し・状態の表と、索引との対応を確かめる。本タスクはそのどれも変えない。

## 4. エラーハンドリング設計

N/A。実行時の処理を変えない。

## 5. セキュリティ考慮事項

N/A。ADR の記述を今のコードに合わせるだけで、`docs/dev/security.md` の規則と前提、秘密情報の扱い、信頼境界を変えない（[01_requirements.md](01_requirements.md) 4.2）。

## 6. 処理フローの詳細

N/A。処理のフローがない。

## 7. テスト戦略

-   **書き直した文の内容（AC-01）:** 自然言語の内容なので、PR のレビューで読んで確かめる（`manual`）。
-   **ほかの部分を変えないこと（AC-02・AC-03）:** 変更の前と後の関係なので、タスクの `_checks/` に置くスクリプトで、マージベースと PR の head を比べる（`diff`。`docs/dev/conventions/requirements_process.md`「4. Acceptance Tests」）。
-   **訂正の行（AC-04）:** 同じスクリプトが、マージベースから head までのコミットのメッセージを読み、ADR-0024 を名指す訂正の行が 1 行あることを確かめる。
-   **訂正の段落（AC-05）:** PR の説明を読んで確かめる（`manual`）。段落は `/runplan` が PR を作るときに訂正の行から書く（`.claude/commands/_lib/task-pr-handoff.md` Part B step 3）。
-   恒久のテストは足さない。文の言い回しを確かめる文書テストは、`CLAUDE.md`「Testing Strategy」が禁じる。

## 8. 実装の優先順位

フェーズは 1 つで、実装の PR も 1 つである。ADR-0024 を書き直し、`_checks/` のスクリプトを足して PR の上で 1 回走らせる。

## 9. 将来の拡張性

N/A。拡張の対象がない。
