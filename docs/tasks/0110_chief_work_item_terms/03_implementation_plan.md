# 実装計画書：chief の状況の表の用語を「作業項目」で整理し直す

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

用語集に「タスクの進行」「作業項目」「作業項目の状態」を足し、その語で、chief のガイドの 3.1 と ADR-0046・ADR-0035・サンプルの説明、0102・0105 の承認済みの文書の古い用語を直す（設計書 3 章）。

### 1.2. 実装の原則

-   **語の置き換えは設計書 3.2 節の対応の表に従う。** 文の言い回しは、置き換えた語が自然に読める範囲でだけ変える。
-   **意味は変えない。** 値の集合、各値の決まり方、表示する文字列、決定、Document Status、変更履歴は変えない（AC-16・AC-17）。
-   **コードと `/chief` の定義の表は変えない**（F-005）。`internal/chief` の Go のファイル、`chief.md` の表の列の見出しと値の対応の表、protocol 番号は変えない。
-   **編集上の訂正として記録する。** ADR と承認済みの文書の訂正は、コミットのメッセージに訂正の行を書き、PR の説明に訂正の段落を書く（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」、`docs/dev/conventions/adr.md`「Editorial corrections」の「Recording」）。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   なし。人に聞くべき未決の判断はなかった。フェーズは 1 つ、PR は 1 本である（3.2 節）。差分の範囲の AC（AC-14・AC-16・AC-19）は、記録した diff を読んで確かめる。差分のほとんどは 1 行の中の語の置き換えで、語と言い回しに限られているかは、スクリプトで判定するより読むほうが確かなためである（`requirements_process.md` §4「`diff` verification」の「Proportion」）。
    -   ADR: なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `910bb12b`（`origin/main`）で確かめた。

-   古い用語の場所は、要件書 1 章の背景と設計書 3.3〜3.5 節のとおりである。今の仕様を持つ文書のうち、chief の状況の表の意味で古い用語を使うのは、`docs/dev/guides/chief_guide.md:57`・`:63`・`:64`・`:82`、`docs/dev/adr/0046-chief-of-staff.md:79`・`:175`・`:182`・`:396`、`docs/dev/adr/0035-language-policy.md:72`、`internal/chief/testdata/README.md:151` である。
-   `git grep -e 進行の位置 -e 文書の段 -e 進み具合 -- ':!docs/tasks'` は、上のほかに、ほかの意味の「進み具合」（`docs/dev/adr/0013-review-setup.md:24`・`0027-article-job-api.md:19-20`・`0031-secret-closure.md:19`・`0050-fable-partial-review.md:28`、`docs/dev/guides/development.md:54`、`docs/dev/security.md:122-128`、`docs/research/transcript_acquisition/README.md:3`）と、実際のサンプル（`internal/chief/testdata/transcript/claude-recap-notification.jsonl`・`claude-synthetic.jsonl`）を返す。これらは変えない（要件書 2.3 節）。
-   CLAUDE.md と `.claude/commands/chief.md` には、今、古い用語もその英訳もない。`chief.md` は出力の項目の名前として `position` を使う（`chief.md:11`・`:154`）が、これは出力の項目なので変えない（F-005）。
-   0102 と 0105 のブランチ（`isseis/0102-chief-text-table-05`・`isseis/0105-chief-post-merge-next-command-02`）には、2026-11-19 の時点で `origin/main` より先のコミットがない。
-   ガイドの 3.1 は `chief_guide.md:55-94` にある。値の一覧は `:82`、「調べ方」は `:92` である。用語集の chief の語は「ラウンド」（`docs/translation_glossary.md:144`）である。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 用語集と文書の用語の整理（設計書 3 章、AC-01〜AC-20）

#### ステップ 1-1: 用語集

**変えるファイル:** `docs/translation_glossary.md`

- [ ] 「ラウンド」の行の後に、設計書 3.1 節の 3 行を足す（AC-01〜AC-04）

**完了の条件:** 3 つの語があり、備考に定義と旧称がある。

#### ステップ 1-2: ガイドの 3.1

**変えるファイル:** `docs/dev/guides/chief_guide.md`

- [ ] 3.1 を設計書 3.3 節のとおりに書き直す。`:82` の段落を、タスクの進行・今の作業項目・作業項目の状態の 3 つの段落に替え、「調べ方」に出力の項目の対応を足す（AC-05〜AC-09）
- [ ] 実装の PR を作る前に `git fetch origin main` をして、0102 か 0105 の実装がガイドの 3.1 を先に変えていれば、その時点の 3.1 に同じ内容を入れる（設計書 3.3 節の「先に変わったときの扱い」、AC-10）

**完了の条件:** 3.1 が新しい用語で書かれ、値の集合と各値の決まり方は前と同じである。

#### ステップ 1-3: ADR とサンプルの説明

**変えるファイル:** `docs/dev/adr/0046-chief-of-staff.md`、`docs/dev/adr/0035-language-policy.md`、`internal/chief/testdata/README.md`

- [ ] 設計書 3.4 節の表のとおりに直す（AC-11〜AC-13）。経緯の項目は足さない（AC-14）
- [ ] コミットのメッセージに、ADR の編集上の訂正の行を書く（AC-14）

**完了の条件:** 3 つのファイルの古い用語が新しい用語になり、ADR の決定の中身は変わらない。

#### ステップ 1-4: 0102 と 0105 の文書

**変えるファイル:** 要件書 F-004 の対象の文書（設計書 3.5 節の表の 5 つ。0105 のほかの文書が `origin/main` にあれば、それも）

- [ ] 設計書 3.2 節の対応と 3.5 節の扱いに従って直す（AC-15・AC-16）。Document Status と変更履歴は変えない（AC-17）
- [ ] 0102 の実装が CLAUDE.md・ガイド・`chief.md` の文に古い用語かその英訳をすでに書いていれば、その箇所も直す（AC-18）
- [ ] コミットのメッセージに、承認済みの文書の編集上の訂正の行を書く（AC-17）

**完了の条件:** 対象の文書の古い用語が新しい用語になり、決定・振る舞い・表示する文字列・状態は変わらない。

#### ステップ 1-5: 仕上げ

- [ ] `git grep -n -e 進行の位置 -e 文書の段 -e 進み具合 -- ':!docs/tasks/0092_*' ':!docs/tasks/0093_*' ':!docs/tasks/0110_*'` の結果を読み、chief の状況の表の意味の箇所が残っていないことを確かめる。ほかの意味の箇所（1.4 節）と、0102 の設計書の 0093 の語との対応（設計書 3.5 節）は除く。結果と確かめたことを 7 章の AC-10・AC-15 の行に書く
- [ ] `make test` と `make lint` を通す（ガイドと ADR を読む既存のガードを含む）
- [ ] 7 章の AC-14・AC-16・AC-19 の diff の記録を書く

**完了の条件:** グリーンゲートが通り、古い用語の確かめと diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 用語集の 3 つの語、ガイドの 3.1、ADR とサンプルの説明の訂正、0102・0105 の文書の訂正、確かめの記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-5 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（文書の語を決まった対応で置き換えるだけで、コードの変更も方式の選択もない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   恒久のテストは足さない（設計書 7 章）。古い用語がないことを確かめるテストは、`CLAUDE.md`「Testing Strategy」が禁じる。
-   既存の文書のガード（`make test` の `doc-guard` に当たるテスト）は、ガイドの値と ADR の形を読む。ステップ 1-5 で通ることを確かめる。
-   古い用語が残っていないこと（AC-10・AC-15）は、ステップ 1-5 で `git grep` の結果を 1 度読む。
-   差分の範囲（AC-14・AC-16・AC-19）は、記録した diff を PR のレビューで読む。
-   用語集とガイドの中身（AC-01〜AC-09）、ADR とサンプルの説明（AC-11〜AC-13）は、PR のレビューで読む。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 0102 の実装が先に進み、古い用語をガイド・CLAUDE.md・`chief.md` に書く | ステップ 1-2 とステップ 1-4 で、PR を作る前に `origin/main` を取り込み、書かれた箇所も直す（AC-10・AC-18） |
| 本タスクの訂正と 0102・0105 の実装が同じ文書を同時に変え、マージで衝突する | 衝突したら、後からマージする側が `origin/main` を取り込み、新しい用語で解く。0102・0105 のブランチには 2026-11-19 の時点でコミットがない（1.4 節） |
| ほかの意味の「進み具合」や、タスクの文書を指す「文書」まで置き換える | 設計書 3.2 節の注意に従い、chief の状況の表の意味の箇所だけを直す。diff の記録を PR のレビューで読む |
| 置き換えが言い回しを超えて、承認済みの文書の決定や振る舞いを変える | 置き換えの対応を決めておき（設計書 3.2 節）、diff の記録を PR のレビューで読む（AC-16） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-5）
- [ ] PR を作成した
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR のレビューで、用語集の「タスクの進行」の定義を読む |
| AC-02 | `manual` | 同じく「作業項目」の定義と旧称を読む |
| AC-03 | `manual` | 同じく「作業項目の状態」の定義と旧称、「進行の位置」の旧称を読む |
| AC-04 | `manual` | 3 つの語の英語の訳語が、用語集のほかの語の訳語と重ならないことを読む |
| AC-05 | `manual` | PR のレビューで、ガイドの 3.1 のタスクの進行の段落を読む |
| AC-06 | `manual` | 同じく今の作業項目の段落を読み、値の集合と決まり方が前の `chief_guide.md:82` と同じことを確かめる |
| AC-07 | `manual` | 同じく簡易な進め方の扱いを読む |
| AC-08 | `manual` | 同じく作業項目の状態の段落を読み、値の集合が前と同じことを確かめる |
| AC-09 | `manual` | 同じく「調べ方」の出力の項目の対応を読む |
| AC-10 | `manual` | ステップ 1-5 の `git grep` の結果を読む。結果と確かめたことをここに書く |
| AC-11 | `manual` | PR のレビューで、ADR-0046 の 4 か所を読む |
| AC-12 | `manual` | 同じく ADR-0035 の経緯の 1 か所を読む |
| AC-13 | `manual` | 同じく `internal/chief/testdata/README.md` の 1 行を読む |
| AC-14 | `diff` | 下の AC-14 の diff の記録を、PR のレビューで読む。訂正の行と訂正の段落は、コミットのメッセージと PR の説明で読む |
| AC-15 | `manual` | ステップ 1-5 の `git grep` の結果と、AC-16 の diff の記録を読む |
| AC-16 | `diff` | 下の AC-16 の diff の記録を、PR のレビューで読む |
| AC-17 | `diff` | AC-16 の diff の記録に、Document Status の表と変更履歴の行がないことを読む。訂正の行と訂正の段落は、コミットのメッセージと PR の説明で読む |
| AC-18 | `manual` | ステップ 1-4 で、0102 の実装が古い用語かその英訳を書いていたかを確かめ、書いていたなら直した箇所をここに書く。書いていなければ「当たらない」と書く |
| AC-19 | `diff` | 下の AC-19 の diff の記録（変わったファイルの一覧を含む）を、PR のレビューで読む |
| AC-20 | `diff` | AC-19 の記録の変わったファイルの一覧に、`internal/chief/records.go` がないことを読む |

Diff record (AC-14):

- Compared paths: `docs/dev/adr/0046-chief-of-staff.md`, `docs/dev/adr/0035-language-policy.md`

Diff record (AC-16):

- Compared paths: `docs/tasks/0102_chief_text_table/01_requirements.md`, `docs/tasks/0102_chief_text_table/02_architecture.md`, `docs/tasks/0102_chief_text_table/03_implementation_plan.md`, `docs/tasks/0102_chief_text_table/design_handoff.md`, `docs/tasks/0105_chief_post_merge_next_command/01_requirements.md`

Diff record (AC-19):

- Compared paths: `CLAUDE.md`, `.claude/commands/chief.md`

## 8. 成功の基準 (Success Criteria)

-   7 章のすべての AC の検証が通り、グリーンゲートが通る。

## 9. 次のステップ (Next Steps)

-   3 つの文書が承認されたら、`/runplan 0110` でフェーズ 1 を実装し、PR を開く。
