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

用語集に「タスクの進行」「作業項目」「作業項目の表示」「状態の詳細」を足す。その語で、chief のガイドの 3.1、ADR-0046 の決定、サンプルの説明、0102・0105 の承認済みの文書の古い用語を直す（設計書 3 章）。

### 1.2. 実装の原則

-   **語の置き換えは設計書 3.2 節の対応の表に従う。** 言い回しは、置き換えた語が自然に読める範囲でだけ変える。迷ったときは「作業項目の表示」にする。
-   **意味は変えない。** 値の集合、各値の決まり方、表示する文字列、例外の範囲、決定、Document Status、変更履歴は変えない（AC-16・AC-17）。
-   **コードと `/chief` の定義の表は変えない**（F-005）。`internal/chief` の Go のファイル、`chief.md` の表の列の見出しと値の対応の表、出力の項目の名前、protocol 番号は変えない。
-   **直す場所は中身で探す。** 設計書の行番号は目安である。0102 の PR-1（#1012）などが先にマージされていれば、ずれている（設計書 1 章）。
-   **編集上の訂正として記録する。** ADR と承認済みの文書の訂正は、コミットのメッセージに訂正の行を書き、PR の説明に訂正の段落を書く（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」、`docs/dev/conventions/adr.md`「Editorial corrections」の「Recording」）。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   なし。人に聞くべき未決の判断はなかった。フェーズは 1 つ、PR は 1 本である（3.2 節）。差分を確かめる AC は、記録した diff を読んで確かめる。差分のほとんどは段落の中の語の置き換えで、語と言い回しに限られているかは、スクリプトで判定するより読むほうが確かなためである（`requirements_process.md` §4「`diff` verification」の「Proportion」）。日本語の段落は 1 行が長いので、読む量は置き換えの数より多い。それでも、0102 と 0105 の記録は 5 つのファイルの 40 か所ほどの置き換えで、読み切れる量と見積もる。読み切れないと分かったら、`requirements_process.md` の「Output too long to read」に従う。
    -   ADR: なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `910bb12b`（`origin/main`）で確かめた。

-   古い用語の場所は、要件書 1 章の背景と設計書 3.3〜3.5 節のとおりである。今の仕様を持つ文書のうち、chief の状況の表の意味で古い用語を使うのは、`docs/dev/guides/chief_guide.md:57`・`:63`・`:64`・`:82`、`docs/dev/adr/0046-chief-of-staff.md:79`・`:175`・`:182`、`internal/chief/testdata/README.md:151` である。ADR-0046 の `:396` と ADR-0035 の `:72` は経緯で、変えない（要件書 1 章）。
-   `git grep -e 進行の位置 -e 文書の段 -e 進み具合 -- ':!docs/tasks'` は、上のほかに次を返す。どれも変えない（要件書 2.3 節）。
    -   ほかの意味の「進み具合」：`docs/dev/adr/0013-review-setup.md:24`・`0027-article-job-api.md:19-20`・`0031-secret-closure.md:19`・`0035-language-policy.md:71`・`0050-fable-partial-review.md:28`、`docs/dev/guides/development.md:54`、`docs/dev/security.md:122-128`、`docs/research/transcript_acquisition/README.md:3`
    -   実際のサンプル：`internal/chief/testdata/transcript/claude-recap-notification.jsonl`・`claude-synthetic.jsonl`
-   CLAUDE.md には、今、古い用語もその英訳もない。`.claude/commands/chief.md` の英語の文の "position"（`chief.md:11`・`:154`）は、出力の項目の `position` に合わせた語なので変えない（要件書 1 章）。
-   2026-11-19 の時点で、0102 の PR-1（#1012、`isseis/0102-chief-text-table-05`）が開いている。PR-1 は、0102 の 3 つの文書、ADR-0046（`:175` の文の後ろへの追記と、決定と経緯の項目の追加）、`internal/chief/testdata/README.md`（タスク 0102 のサンプルの節の追加）を変える。古い用語は足さない。0105 のブランチ（`isseis/0105-chief-post-merge-next-command-02`）には、`origin/main` より先のコミットがない。
-   ガイドの 3.1 は `chief_guide.md:55-100` にある。「進行の位置」の段落は `:82`、「調べ方」は `:99`、8 章の「1 段前のコマンド」は `:169` である。用語集の chief の語は「ラウンド」（`docs/translation_glossary.md:144`）である。ガイドの 3.1 を読む文書のガードはない（`cmd/chief/docs_test.go` が読むのは、サブコマンド・終了コード・値の表である）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 用語集と文書の用語の整理（設計書 3 章、AC-01〜AC-20）

#### ステップ 1-1: 用語集

**変えるファイル:** `docs/translation_glossary.md`

- [ ] 「ラウンド」の行の後に、設計書 3.1 節の 4 行を足す（AC-01〜AC-05）

**完了の条件:** 4 つの語があり、備考に定義と旧称がある。

#### ステップ 1-2: ガイド

**変えるファイル:** `docs/dev/guides/chief_guide.md`

- [ ] `git fetch origin main` をして `origin/main` を取り込み、0102 か 0105 の実装がガイドの 3.1 を先に変えていないかを見る（設計書 3.3 節の「先に変わったときの扱い」）
- [ ] 設計書 3.3 節のとおりに書き直す。「進行の位置」の段落を、タスクの進行・作業項目の表示・状態の詳細の 3 つの段落に替え、「調べ方」に出力の項目の対応を足し、8 章の 1 語を直す（AC-06〜AC-11）

**完了の条件:** 3.1 が新しい用語で書かれ、値の集合と各値の決まり方は前と同じである。

#### ステップ 1-3: ADR の決定とサンプルの説明

**変えるファイル:** `docs/dev/adr/0046-chief-of-staff.md`、`internal/chief/testdata/README.md`

- [ ] 設計書 3.4 節の表のとおりに直す（AC-12・AC-13）。経緯は変えない（AC-14）
- [ ] コミットのメッセージに、ADR の編集上の訂正の行を書く（AC-14）

**完了の条件:** 2 つのファイルの古い用語が新しい用語になり、ADR の決定の中身と経緯は変わらない。

#### ステップ 1-4: 0102 と 0105 の文書

**変えるファイル:** 要件書 F-004 の対象の文書

- [ ] `origin/main` に 0105 の `02_architecture.md` などがあるかを見て、対象の文書を決める（要件書 F-004）
- [ ] 設計書 3.2 節の対応と 3.5 節の扱いに従って直す（AC-15・AC-16）。Document Status と変更履歴は変えない（AC-17）
- [ ] 0102 の実装が CLAUDE.md・ガイド・`chief.md` の文に古い用語かその英訳をすでに書いていれば、その箇所も直す（AC-18）
- [ ] コミットのメッセージに、承認済みの文書の編集上の訂正の行を書く（AC-17）

**完了の条件:** 対象の文書の古い用語が新しい用語になり、決定・振る舞い・例外の範囲・表示する文字列・状態は変わらない。

#### ステップ 1-5: 仕上げ

- [ ] 実装の助けとして、`git grep -n -e 進行の位置 -e 文書の段 -e 進み具合 -- docs/translation_glossary.md docs/dev/guides/chief_guide.md docs/dev/adr/0046-chief-of-staff.md internal/chief/testdata/README.md CLAUDE.md .claude/commands/chief.md 'docs/tasks/0102_*' 'docs/tasks/0105_*'` を実行し、直し忘れを探す。用語集の旧称の記載と、ADR-0046 の経緯は除く。これは検証の記録ではない（検証は 7 章の diff の記録）
- [ ] `make test` と `make lint` を通す（ガイドと ADR を読む既存のガードを含む）
- [ ] 7 章の diff の記録を書く

**完了の条件:** グリーンゲートが通り、diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 用語集の 4 つの語、ガイドの書き直し、ADR の決定とサンプルの説明の訂正、0102・0105 の文書の訂正、diff の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-5 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（文書の語を決まった対応で置き換えるだけで、コードの変更も方式の選択もない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   恒久のテストは足さない（設計書 7 章）。古い用語がないことを確かめるテストは、`CLAUDE.md`「Testing Strategy」が足さないと定めている。
-   既存の文書のガード（`make test` の `doc-guard` に当たるテスト）は、ガイドの値の表と ADR の形を読む。ステップ 1-5 で通ることを確かめる。
-   差分と古い用語が残っていないことは、7 章の diff の記録を PR のレビューで読んで確かめる。
-   用語集とガイドの新しい段落の中身は、PR のレビューで読む。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 0102 の PR-1（#1012）と同じファイル（0102 の文書、ADR-0046、`internal/chief/testdata/README.md`）を変え、マージで衝突する | 後からマージする側が `origin/main` を取り込み、新しい用語で解く。直す場所は中身で探す（1.2 節） |
| 本タスクのマージの前に作られた 0102 のブランチが、古い用語をガイド・CLAUDE.md・`chief.md` に書く | 本タスクより先にマージされたものは、ステップ 1-2 とステップ 1-4 で直す（AC-11・AC-18）。本タスクより後にマージされるものは防げないので、既知のリスクとして、その 0102 の PR の中で直す（要件書 4.3 節） |
| ほかの意味の「進み具合」や、タスクの文書を指す「文書」まで置き換える | 設計書 3.2 節の注意に従い、chief の状況の表の意味の箇所だけを直す。diff の記録を PR のレビューで読む |
| 置き換えで、0102 の例外の範囲など、承認済みの文書の決定を狭める | 値の集合を指す箇所は「作業項目の表示」にし、迷ったら広い側に倒す（設計書 3.2 節）。diff の記録を PR のレビューで読む（AC-16） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-5）
- [ ] PR を作成した
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR のレビューで、用語集の「タスクの進行」の定義を読む |
| AC-02 | `manual` | 同じく「作業項目」の定義を読む |
| AC-03 | `manual` | 同じく「作業項目の表示」の定義と旧称、組の書き方を読む |
| AC-04 | `manual` | 同じく「状態の詳細」の定義と旧称を読む |
| AC-05 | `manual` | 4 つの語の英語の訳語が、用語集のほかの語の訳語と同じでないことを読む |
| AC-06 | `manual` | PR のレビューで、ガイドのタスクの進行の段落を読む |
| AC-07 | `diff` | 下のガイドの diff の記録で、作業項目の表示の段落の値の集合と決まり方が、消えた「進行の位置」の段落と同じことを読む |
| AC-08 | `manual` | PR のレビューで、ガイドの簡易な進め方の扱いを読む |
| AC-09 | `diff` | 下のガイドの diff の記録で、状態の詳細の段落の値の集合が、消えた段落と同じことを読む |
| AC-10 | `manual` | PR のレビューで、ガイドの「調べ方」の出力の項目の対応を読む |
| AC-11 | `diff` | 下のガイドの diff の記録で、古い用語を含む行がすべて消えたか、新しい用語に替わったことを読む |
| AC-12 | `diff` | 下の ADR の diff の記録で、決定の 3 つの文の語が替わり、中身が変わらないことを読む |
| AC-13 | `manual` | PR のレビューで、`internal/chief/testdata/README.md` の 1 行を読む |
| AC-14 | `diff` | 下の ADR の diff の記録を読む。訂正の行と訂正の段落は、コミットのメッセージと PR の説明で読む |
| AC-15 | `diff` | 下の 0102・0105 の diff の記録で、古い用語を含む行がすべて替わったことを読む |
| AC-16 | `diff` | 同じ記録で、差分が語の置き換えと言い回しに限られることを読む |
| AC-17 | `diff` | 同じ記録に、Document Status の表と変更履歴の行がないことを読む。訂正の行と訂正の段落は、コミットのメッセージと PR の説明で読む |
| AC-18 | `manual` | ステップ 1-4 で、0102 の実装が古い用語かその英訳を書いていたかを確かめ、書いていたなら直した箇所をここに書く。書いていなければ「当たらない」と書く。直した箇所は下の差分の範囲の記録で読む |
| AC-19 | `diff` | 下の差分の範囲の記録（変わったファイルの一覧を含む）を読む |
| AC-20 | `diff` | 差分の範囲の記録の、変わったファイルの一覧に `internal/chief/records.go` がないことを読む |

Diff record (AC-07, AC-09, AC-11):

- Compared paths: `docs/dev/guides/chief_guide.md`

Diff record (AC-12, AC-14):

- Compared paths: `docs/dev/adr/0046-chief-of-staff.md`

Diff record (AC-15, AC-16, AC-17):

- Compared paths: `docs/tasks/0102_chief_text_table/01_requirements.md`, `docs/tasks/0102_chief_text_table/02_architecture.md`, `docs/tasks/0102_chief_text_table/03_implementation_plan.md`, `docs/tasks/0102_chief_text_table/design_handoff.md`, `docs/tasks/0105_chief_post_merge_next_command/01_requirements.md`, `docs/tasks/0105_chief_post_merge_next_command/02_architecture.md`, `docs/tasks/0105_chief_post_merge_next_command/03_implementation_plan.md`, `docs/tasks/0105_chief_post_merge_next_command/design_handoff.md`, `docs/tasks/0105_chief_post_merge_next_command/implementation_handoff.md`

Diff record (AC-19, AC-20):

- Compared paths: `CLAUDE.md`, `.claude/commands/chief.md`

## 8. 成功の基準 (Success Criteria)

-   7 章のすべての AC の検証が通り、グリーンゲートが通る。

## 9. 次のステップ (Next Steps)

-   3 つの文書が承認されたら、`/runplan 0110` でフェーズ 1 を実装し、PR を開く。
