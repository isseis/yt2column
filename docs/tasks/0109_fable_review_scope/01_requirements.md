# 要件定義書：Fable の部分レビューの prior-docs の役目を決定の食い違いに絞る

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、Fable の部分レビュー（`.claude/commands/_lib/fable-review.md`、[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）の有効性を集計した結果（issue #1008）を受けて、依頼の範囲と依頼の仕方のうち、変えるものと変えないものを定める。変えるのは、Fable に与える `prior-docs`（先行文書との照合）の役目だけである。

対応 issue: [#1008](https://github.com/isseis/yt2column/issues/1008)（関連: [#936](https://github.com/isseis/yt2column/issues/936)、タスク 0098）

**本書の記述範囲:** 本書は、`prior-docs` の役目で Fable に報告させるものと報告させないもの、そして変えないものを定める。役目の文を `fable-review.md` のどこに置くかは `02_architecture.md` で、確かめ方は `03_implementation_plan.md` で定める。

**背景:**

-   Fable の部分レビューは、PR を開く前に、4 つの基準（`trust-boundary`・`prior-docs`・`external-claim`・`undecided-choice`）に当たる箇所だけを Fable にレビューさせる。基準ごとに狭い役目を与える（`fable-review.md` の「Scope criteria」と「The prompt」）。今の `prior-docs` の役目は「変更が先行文書や ADR と食い違う箇所、または断りなく離れた箇所を見つける。KIND `documents` では、PR の文書どうしの食い違いも見る」である（`fable-review.md:280-282`）。相互参照や行番号の誤りを報告の対象から外す、とは書いていない。
-   issue #1008 は、今の仕組みで Fable が走った 12 件の PR の記録を集計した。指摘は 69 件（Major 12、Minor 57）で、rejected は 0 件だった。PR ごとの表では、12 回中 9 回で Major が出た（issue の全体の表は 10 回とするが、PR ごとの表と合わない）。基準ごとの集計で、`prior-docs` は指摘が最も多い 33 件だったが、Major は 4 件（12%）だった。大半は Minor の相互参照の誤り、行番号のずれ、言い回しの食い違いだった。Major の割合が高かったのは、`undecided-choice`（33%）と `trust-boundary`（25%）である。
-   同じ集計で、実装の PR は 4 件しかなく、そこで出た Major 2 件はどちらも文書の側への指摘だった。そのうち #989 の 1 件は `exclusion-error`、つまり範囲から外した箇所を Fable が拾い直した指摘だった。
-   OpenCode のセッションでは Fable を呼べず、#992 と #994 は `not run agent-cannot-call` と記録された（`fable-review.md:142-145`）。

以上の行番号は、コミット `c6fe505a`（`origin/main`）で確かめた。

**issue #1008 で決めたこと:**

-   **集計の対象と方法、および集計の結果は issue #1008 に置き、本書は結果を背景として引く。** 理由: 集計は判断の材料であり、決めたことではないため。ADR: なし（タスクの中で閉じる）。

**要件定義の際に決めたこと（2026-11-19）:**

-   **進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。** 変わるのは `fable-review.md` の 1 か所（`prior-docs` の役目）と ADR-0050 で、新しい仕組みは作らず、未決の判断も少ない。ADR-0050 の決定は変わるが、変わるのは 1 つの基準の役目の範囲だけである。除外される変更に当たらない理由は、次のどれも変えないことである（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   `docs/dev/security.md` の規則と前提
    -   開発用ツールの信頼の境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）。Fable の出力の扱い（コミットや PR の説明に写さない）と、レビューのスレッドの本文や Bot への依頼の扱いは変えない
    -   外部の依存。Fable は Claude Code の Agent ツールで選べるモデルの 1 つで、今も使っている
    -   ADR: なし（進め方の選択で、本タスクの中で閉じる）。
-   **`prior-docs` の役目を、決定の食い違いに絞る（issue の推奨案 3、F-001）。** 報告させるのは、変更が先行文書・ADR・同じ PR のほかの文書の決定と食い違う点、または断りなく離れた点である。報告させないのは、読み手が取る決定を変えない行番号のずれ・言い回しの違い・参照先のずれである。理由: `prior-docs` の指摘 33 件の大半は Minor の相互参照・行番号・言い回しで、Major の割合は 12% と、ほかの基準より低いため。役目を狭めて、Fable の注意を決定の食い違いに向ける。報告させないことにした誤りは、セルフレビュー・`japrose`・Codex の PR レビュー・人のレビューで見つけることにする。Fable が見つけていた Minor の一部が残ることは、受け入れる。ADR: [ADR-0050](../../dev/adr/0050-fable-partial-review.md) を更新する。
    -   採らない案: 今のままにする案。Minor の指摘が多く、役目が広いままになる。
-   **相互参照の誤りのうち、読み手を別の決定に導くものは、決定の食い違いとして報告の対象に残す（F-001）。** たとえば、ADR の別の決定や、決定が違う節を指してしまう参照である。理由: そうした参照は、読み手に誤った決定を取らせるので、決定の食い違いと同じ害を持つため。同じ理由で、言い回しの違いのうち、求めること・許すこと・退けることを変えるものも、決定の食い違いとして報告させる（この言い回しの違いの扱いは、2026-11-19 のセルフレビューの指摘を受けて、人に問わずに加えた）。ADR: ADR-0050（上の項と同じ更新）。
    -   採らない案: 相互参照の誤りを、種類を問わずすべて外す案。この案では、決定を取り違えさせる参照が Fable の役目から漏れる。
-   **役目の外の指摘を Fable がそれでも挙げたときは、ほかの指摘と同じ規則でトリアージする（F-002）。** 役目の外であることだけを理由に退けない。記録の基準の値は、今の規則（`fable-review.md:465-469`）のとおり、その箇所の基準（`prior-docs`）のままとする。役目の外の指摘がどれだけ減ったかを測るかどうかは、#936 に任せる。理由: 正しい指摘なら直すのは安いため。また、役目の外を理由に退けると rejected の件数がふくらみ、[#936](https://github.com/isseis/yt2column/issues/936) の集計で、指摘が当たった割合を正しく測れなくなるため。ADR: なし（トリアージの規則は変わらない）。
    -   採らない案: 役目の外として rejected にする案。上の理由で採らない。
-   **文書の PR での Fable の部分レビューは、今のまま続ける（issue の推奨案 1）。** 理由: 文書の PR 8 回のうち 7 回で Major が出たため。ADR: なし（ADR-0050 の決定は変わらない）。
-   **実装の PR で Fable を走らせる条件は、今は変えない（issue の推奨案 2、2.3 節）。** 理由: 実装の PR は 4 件しかなく、決めるには少ないため。また、推奨案 2 は、範囲に入る箇所がなければ Fable を呼ばない。そうすると、#989 の Major のような `exclusion-error` の指摘を失う。今の仕組みは、範囲に入る箇所がないときも除外の一覧を Fable に確かめさせる（`fable-review.md:268-269`）。実装の PR の件数が増えたら、見直す。ADR: なし（ADR-0050 の決定は変わらない）。
    -   採らない案: 今決めて、計画どおりの実装だけの PR では Fable を呼ばず、新しい `not run` の理由を記録する案。上の理由で今は採らない。この案を採ると、判定のスクリプト（`.claude/commands/_lib/review-records.sh:193` の `not run` の理由の一覧）と ADR-0050 も変えることになる。
-   **OpenCode のセッションで Fable が走らないことは、今のまま受け入れる（issue の推奨案 5、2.3 節）。** `not run agent-cannot-call` を記録して進む。理由: OpenCode で `/runplan` が走るのは、実装モデル要件が `sonnet` の PR だけで（`.claude/commands/_context.md`「Command routing」の `runplan-sonnet` の行）、計画どおりに書く実装である。これは、Fable の効果が最も薄い種類の PR である。ADR: なし（ADR-0050 の決定のとおりである）。
    -   採らない案: チェックポイントで Claude Code のセッションから Fable を後追いで走らせる案。chief の送り先（[ADR-0046](../../dev/adr/0046-chief-of-staff.md)）か `/runpr` の手順を変えることになる。
-   **記録を足すこと（issue の推奨案 4）は、本タスクでは扱わず、[#936](https://github.com/isseis/yt2column/issues/936) に任せる（2.3 節）。** 理由: Codex の PR レビューと `/fixpr` の指摘を Fable の範囲の内か外かで分けて記録することと、誤った却下を記録することは、#936 の「やること 1」と同じ内容であるため。却下の理由を記録する規則は、今もある（`fable-review.md:497-500`）。Fable の指摘の正確さを人が判定するかどうかも、記録の集計を扱う #936 で決めるのが自然である。ADR: なし（ADR-0050 の決定は変わらない）。
    -   採らない案: 本タスクで記録の行と `/runpr` または `/fixpr` の手順を変える案。#936 と範囲が重なり、新しい仕組みを作ることになる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   Fable の `prior-docs` の役目を決定の食い違いに絞り、決定を変えない誤り（行番号のずれ、言い回しの違いなど）を Fable に報告させないようにする。

### 2.2. スコープ (In Scope)

-   `fable-review.md` の、Fable に与える `prior-docs` の役目の変更（F-001）。
-   ADR-0050 の更新。決定の更新は本書と同じ PR で行う。役目の変更が実装されるまでの食い違いの文は、実装の PR で除く（F-003）。

### 2.3. スコープ外 (Out of Scope)

-   範囲の選び方（`fable-review.md` の「Scope criteria」の表）の変更。本タスクが変えるのは、Fable に報告させるものだけである（F-002）。
-   実装の PR で Fable を走らせる条件の変更（issue の推奨案 2）。理由は 1 章の「要件定義の際に決めたこと」にある。
-   OpenCode のセッションで Fable を走らせること（issue の推奨案 5）。理由は 1 章にある。
-   記録を足すこと（issue の推奨案 4）。[#936](https://github.com/isseis/yt2column/issues/936) で扱う。理由は 1 章にある。
-   トリアージの規則（`.claude/commands/_lib/local-cli-review.md`「Triage rules」）の変更（F-002）。
-   ほかの基準（`trust-boundary`・`external-claim`・`undecided-choice`）の役目の変更。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: prior-docs の役目を決定の食い違いに絞る

Fable に与える `prior-docs` の役目を、決定の食い違いを見ることに絞る。KIND `documents` と KIND `implementation` の両方に当てはまる。範囲から外した箇所を Fable が `prior-docs` に当たると見てレビューするとき（`exclusion-error`）も、この役目でレビューする。

**Acceptance Criteria**:
- **AC-01**: `fable-review.md` が Fable に与える `prior-docs` の役目は、変更が先行文書・ADR の決定と食い違う点、または断りなく離れた点を報告させる。KIND `documents` では、同じ PR の文書どうしの決定の食い違いも報告させる。求めること・許すこと・退けることを変える言い回しの違いは、決定の食い違いに含める。
- **AC-02**: 同じ役目は、読み手を別の決定に導く相互参照の誤り（別の決定や、決定が違う節を指す参照）を、報告させるものに含める。
- **AC-03**: 同じ役目は、読み手が取る決定を変えない行番号のずれ・言い回しの違い・参照先のずれを報告させないと、Fable に伝える。

#### F-002: 範囲の選び方とトリアージを変えない

**Acceptance Criteria**:
- **AC-04**: 実装の PR の差分は、次に挙げるものに限る。たとえば、`fable-review.md` の「Scope criteria」の表、記録の行の形、`.claude/commands/_lib/local-cli-review.md`、`.claude/commands/_lib/review-records.sh` は変わらない。
    -   `fable-review.md` の、Fable に与える `prior-docs` の役目の文
    -   ADR-0050 の、F-003 の食い違いの文の削除
    -   本タスクのディレクトリ（`docs/tasks/0109_fable_review_scope/`）

#### F-003: ADR-0050 を更新する

**Acceptance Criteria**:
- **AC-05**: 本書の PR で、ADR-0050 の「決定」が、`prior-docs` の役目を決定の食い違いに絞ること（AC-01〜AC-03）とその理由を述べる。「背景」「採らなかった案」「結果」「経緯」にも、本書の決定とその根拠・受け入れる代償を加える。
- **AC-06**: 本書の PR で、ADR-0050 の「決定」は、`fable-review.md` の役目の文がまだ絞られていないことを、食い違いの文として issue #1008 へのリンクとともに述べる。実装の PR で、その食い違いの文を除く。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。手順の文を 1 か所変えるだけである。Fable の 1 回の時間とトークンが減るかは、本タスクの要件にしない（#936 の集計で見る）。

### 4.2. セキュリティ (Security)
-   N/A。`docs/dev/security.md` の条件（外部コマンドの起動、API キー、キャッシュ、ネットワーク、信頼できない文字列の埋め込み）のどれにも当たらない。Fable に送るもの、Fable の出力の扱い（`fable-review.md`「Checking the output」）、Fable に求める制約（「The prompt」の Constraints）は変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。Fable が走るかどうかの条件と、記録が有効かの判定を変えない。

### 4.4. 互換性 (Compatibility)
-   記録の行の形を変えないので、既存の記録の読み方（`fable-review.md`「Reading the record lines」）と、#936 が読む PR の説明の記録は、そのまま読める。`prior-docs` の値も変えない。

### 4.5. 保守性 (Maintainability)
-   N/A。ほかの手順への影響はない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   コマンドの定義と共通の手順は英語で書く（`CLAUDE.md`「Document language」の原則 (b)）。ADR は日本語で書く（原則 (a)）。
-   ADR の更新は `docs/dev/conventions/adr.md` に従う。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **Fable の部分レビュー:** タスクの PR を開く前に、決まった基準に当たる箇所だけを Fable のサブエージェントにレビューさせる手順（`fable-review.md`）。
-   **`prior-docs` の役目:** Fable の部分レビューで、基準 `prior-docs` の箇所について Fable に与える役目（`fable-review.md`「The prompt」の Role）。
-   **決定の食い違い:** 変更が、先行文書・ADR・同じ PR のほかの文書の決定と食い違うか、断りなく離れること。求めること・許すこと・退けることを変える言い回しの違いと、読み手を別の決定に導く相互参照の誤りを含む（F-001）。
