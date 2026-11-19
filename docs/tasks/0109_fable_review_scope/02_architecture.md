# 設計書：Fable の部分レビューの prior-docs の役目を決定の食い違いに絞る

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 設計の全体像

本タスクは、`.claude/commands/_lib/fable-review.md` の「The prompt」の節で、Role の `prior-docs` の項（`fable-review.md:280-282`）を書き換える（[01_requirements.md](01_requirements.md) F-001）。あわせて、ADR-0050 の食い違いの文を除く（AC-06）。ほかの節は変えない（F-002）。

**設計原則:**

-   **報告させるものだけを変え、範囲の選び方は変えない。** 「Scope criteria」の表の 2 行目（`fable-review.md:258`）は、セッションがどの箇所を範囲に入れるかの基準である。相互参照が正しいかどうかで箇所を選んでいるわけではない。役目の文だけを変えれば、要件（報告させるもの）が満たされる。

**設計の際に決めたこと（2026-11-19）:**

-   **項の文（3.2 節）は本書で決め、実装はそれを写す。** 理由: 文の境目（報告させる誤りと報告させない誤り）がこのタスクの中身で、実装モデル要件 `sonnet` の PR は OpenCode で走り、Fable の部分レビューも Opus のレビューも受けないため（`.claude/commands/_context.md`「Command routing」と「PR marker conventions」）。本書の PR で文を決めれば、文はこの PR のレビューを受ける。ADR: なし（タスクの中で閉じる）。2026-11-19 のセルフレビューの指摘を受けて、人に問わずに決めた。
-   **変える場所は「The prompt」の Role の `prior-docs` の項だけとし、「Scope criteria」の表は変えない。** 理由: 要件が変えるのは Fable に報告させるものであり、範囲に入れる箇所の基準ではないため。表も変えると、範囲の選び方が変わり、要件の外に出る（要件書 2.3 節）。2026-11-19 に人が選んだ。ADR: なし（ADR-0050 の決定は要件の決定として更新済みで、文の置き場所はタスクの中で閉じる）。
    -   採らない案: Role と「Scope criteria」の表の両方を変える案。上の理由で採らない。

本書の行番号は、コミット `c6fe505a`（`origin/main`）で確かめた。

概念モデルとシステム構成の Mermaid の図は省く。手順の文を 1 か所変えるだけで、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

### 3.1. 変更するファイル

| ファイル | 変更 | 責務 |
|---|---|---|
| `.claude/commands/_lib/fable-review.md` | 「The prompt」の Role の `prior-docs` の項の書き換え | Fable に与える `prior-docs` の役目（AC-01〜AC-03） |
| `docs/dev/adr/0050-fable-partial-review.md` | 「決定」の食い違いの文の削除 | 実装が決定に追いついたことの反映（AC-06） |

### 3.2. 役目の文が述べること

書き換えた `prior-docs` の項は、英語で次のことを述べる（`CLAUDE.md`「Document language」の原則 (b)）。

1.  変更が、先行文書・ADR の決定と食い違う点、または断りなく離れた点を見つける。KIND `documents` では、同じ PR の文書どうしの決定の食い違いも見る。今の項の、食い違う点と断りなく離れた点を見る形は保ち、見る対象を決定に絞る。求めること・許すこと・退けることを変える言い回しの違いは、決定の食い違いとして見る（AC-01）。
2.  読み手を別の決定に導く相互参照の誤り（別の決定や、決定が違う節を指す参照、参照先が引いた理由や前提を支えない `file:line` の引用）は、決定の食い違いとして見る（AC-02）。
3.  読み手が取る決定を変えない行番号のずれ・言い回しの違い・参照先のずれは報告しない（AC-03）。

項の文は次のとおりとする。実装は、これを `fable-review.md:280-282` の項と置き換える。境目の判断が要る文なので、文言を実装に任せず、本書の PR のレビュー（Fable の部分レビューを含む）で読む。

```markdown
  - `prior-docs`: find where the change contradicts, or silently departs
    from, a decision of the prior documents or an ADR (for KIND
    `documents`, also where the documents of the PR contradict one
    another's decisions). A decision is what a document requires, allows,
    or rejects, with the reasons and premises it gives. Report a wording
    difference that changes what is required, allowed, or rejected, and a
    reference or a `file:line` citation that leads the reader to a
    different decision or whose target does not support the premise it is
    cited for. Do not report an error that leaves every decision as it is:
    a line number that drifted but still points to text supporting the same
    claim, a wording difference with the same meaning, or a reference whose
    target moved but still holds the same decision;
```

Role の項は、基準ごとに Fable に与える役目である。そのため、範囲から外した箇所を Fable が `prior-docs` に当たると見てレビューするとき（`exclusion-error`）も、この項がそのまま当てはまる。「Left out」の項（`fable-review.md:292-294`）は変えない。

### 3.3. 変えないもの

-   「Scope criteria」の表、「Output form」、「Checking the output」、「Record lines」と「Reading the record lines」。記録の行の形と、`prior-docs` の値は変わらない（要件書 4.4 節）。
-   `.claude/commands/_lib/local-cli-review.md` の「Triage rules」。役目の外の指摘も、今の規則でトリアージされる（要件書 1 章）。
-   `.claude/commands/_lib/review-records.sh`。記録が有効かの判定は変わらない。

## 4. エラーハンドリング設計

N/A。手順の分岐を変えない。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の条件のどれにも当たらない（[01_requirements.md](01_requirements.md) 4.2 節）。「The prompt」の Constraints の項は変えない。

## 6. 処理フローの詳細

N/A。Fable の部分レビューの流れ（起動・待ち合わせ・トリアージ・記録）は変わらない。

## 7. テスト戦略

-   AC-01〜AC-03 は、役目の文の内容だけを定めるので、PR のレビューで読む（`manual`）。文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。
-   AC-04 は、実装の PR の差分を記録して読む（`diff`）。
-   AC-05 と AC-06 の前半（決定の更新と食い違いの文）は、3 文書の PR のレビューで読む（`manual`）。AC-06 の後半（食い違いの文の削除）は、AC-04 の diff の記録で読む。
-   `fable-review.md` を読む既存のテスト（`cmd/yt2column/fable_state_docs_test.go`・`cmd/yt2column/review_records_test.go`）が見る節（状態の確かめのブロックと記録の行）は変えないので、それらのテストは変わらない。テストは `make test` で通ることを確かめる。

## 8. 実装の優先順位

1 フェーズ、1 本の PR で、役目の文の書き換えと ADR-0050 の食い違いの文の削除を行う。

## 9. 将来の拡張性

-   ほかの基準の役目や、実装の PR で Fable を走らせる条件を見直すときは、#936 の集計をもとに、別のタスクで行う（要件書 2.3 節）。
