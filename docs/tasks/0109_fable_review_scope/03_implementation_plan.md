# 実装計画書：Fable の部分レビューの prior-docs の役目を決定の食い違いに絞る

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`.claude/commands/_lib/fable-review.md` の「The prompt」の Role の `prior-docs` の項を書き換え、Fable に報告させるものを決定の食い違いに絞る（設計書 3.2 節）。ADR-0050 の食い違いの文を除く（AC-06）。

### 1.2. 実装の原則

-   **変えるのは Role の `prior-docs` の項と、ADR-0050 の食い違いの文だけとする**（AC-04、設計書 3.3 節）。
-   **`fable-review.md` は英語で書く**（`CLAUDE.md`「Document language」の原則 (b)）。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   なし。人に聞くべき未決の判断はなかった。フェーズは 1 つ、PR は 1 本である（3.2 節）。実装の方針も 1 つしかない。変更の範囲（AC-04）は、記録した diff を読んで確かめる。読む diff は 2 つのファイルの数行で、スクリプトを書くより diff を読むほうが手間が少ないためである（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」）。
    -   ADR: なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `c6fe505a`（`origin/main`）で確かめた。

-   `prior-docs` の役目は `fable-review.md:280-282` にある。「Scope criteria」の表の 2 行目は `fable-review.md:258`、「Left out」の項は `fable-review.md:292-294` で、どちらも変えない（設計書 3.3 節）。
-   `prior-docs` という語は、`.claude/` と `docs/dev/` と `CLAUDE.md` の中では、`fable-review.md` の 4 か所（`:258`・`:280`・`:324`・`:465`）と ADR-0050 にだけ現れる（`grep -rn 'prior-docs\|先行文書との照合' .claude docs/dev CLAUDE.md` で確かめた）。役目を述べるのは `:280` だけで、ほかの 3 か所は表の行・出力の形・記録の値である。
-   `fable-review.md` を読むテストは `cmd/yt2column/fable_state_docs_test.go` と `cmd/yt2column/review_records_test.go` の 2 つで、状態の確かめのブロックと記録の行を読む（設計書 7 章）。Role の項は読まない。
-   ADR-0050 の食い違いの文は、本タスクの要件定義書のコミットで「決定」に加えた（AC-06）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: prior-docs の役目の書き換え（設計書 3 章、AC-01〜AC-06）

#### ステップ 1-1: 役目の文

**変えるファイル:** `.claude/commands/_lib/fable-review.md`

- [ ] 「The prompt」の Role の `prior-docs` の項を、設計書 3.2 節の 3 点を述べる文に書き換える（AC-01〜AC-03）。ほかの項と節は変えない

**完了の条件:** Role の `prior-docs` の項が設計書 3.2 節の 3 点を述べる。

#### ステップ 1-2: ADR-0050 の食い違いの文

**変えるファイル:** `docs/dev/adr/0050-fable-partial-review.md`

- [ ] 「決定」の、`prior-docs` の役目の項の下にある食い違いの文（`**食い違い:**` で始まる行）を除く（AC-06）。ほかの行は変えない

**完了の条件:** ADR-0050 に、`fable-review.md` の役目の文がまだ絞られていないという食い違いの文がない。

#### ステップ 1-3: 仕上げ

- [ ] `make test`・`make lint` を通す
- [ ] 7 章の AC-04 の diff の記録を書く

**完了の条件:** グリーンゲートが通り、diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 役目の文の書き換え、ADR-0050 の食い違いの文の削除、diff の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-3 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（手順の文 1 か所の書き換えと ADR の 1 行の削除で、方式の選択も危険度の高いステップもない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   AC-01〜AC-03 は役目の文の内容だけを定めるので、PR のレビューで読む（`manual`）。文書のテストは足さない（`CLAUDE.md`「Testing Strategy」）。
-   AC-04 と AC-06 の後半は、記録した diff を読む（`diff`）。
-   AC-05 と AC-06 の前半は、3 文書の PR（本書の PR）のレビューで読む（`manual`）。
-   既存の `fable-review.md` を読むテスト（1.4 節）は変えず、`make test` で通ることを確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 書き換えた文が、決定を変えない誤りの例を挙げすぎ、決定の食い違いまで報告させなくなる | 設計書 3.2 節の 2 点目（別の決定に導く参照は見る）を文に残す。PR のレビューで AC-01〜AC-03 を読む |
| 役目の文のほかに、Scope criteria の表や記録の行まで変えてしまう | AC-04 の diff の記録で、変わった行を読む |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-3）
- [ ] PR を作成した
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | 実装の PR のレビューで、Role の `prior-docs` の項が決定の食い違い（KIND `documents` では文書どうしの食い違いも）を報告させることを読む |
| AC-02 | `manual` | 同じレビューで、別の決定に導く相互参照の誤りを報告させるものに含めることを読む |
| AC-03 | `manual` | 同じレビューで、決定を変えない誤りを報告させないと伝えることを読む |
| AC-04 | `diff` | 下の diff の記録にある、変わったファイルの一覧と 2 つのファイルの差分を、実装の PR のレビューで読む |
| AC-05 | `manual` | 3 文書の PR のレビューで、ADR-0050 の「決定」「採らなかった案」「経緯」を読む |
| AC-06 | `manual`・`diff` | 前半は 3 文書の PR のレビューで ADR-0050 の食い違いの文を読む。後半（削除）は、下の diff の記録で読む |

Diff record (AC-04, AC-06):

- Compared paths: `.claude/commands/_lib/fable-review.md`, `docs/dev/adr/0050-fable-partial-review.md`

## 8. 成功の基準 (Success Criteria)

-   7 章のすべての AC の検証が通り、グリーンゲートが通る。

## 9. 次のステップ (Next Steps)

-   3 つの文書が承認されたら、`/runplan 0109` でフェーズ 1 を実装し、PR を開く。
