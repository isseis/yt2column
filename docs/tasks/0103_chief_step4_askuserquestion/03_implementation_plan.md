# 実装計画書：/chief の step 4 の質問を AskUserQuestion で聞くことを明示する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-18 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`.claude/commands/chief.md` の 3 か所の文を書き換え、step 4 の問いを `AskUserQuestion` で聞くことと、step 3 の番号つきの一覧との境目を明示する（設計書 3.1〜3.3 節）。

### 1.2. 実装の原則

-   **変えるのは文だけにする**（AC-06・AC-07）。`chief` を呼び出す行と `--protocol 3` は変えない。
-   **コマンドの定義は英語で書く**（`CLAUDE.md`「Document language」の原則 (b)）。選択肢の名前（`送らない`・`ほかの項目を見る`）は、今の `chief.md` が人に示す選択肢の名前を日本語で書いているのに合わせ、日本語で書く。

### 1.3. 計画の際に決めたこと（2026-11-18）

-   **実装の PR は 1 本とし、フェーズも 1 つにする。** 変えるのは 1 つのファイルの 3 か所で、分ける理由がないためである。
    -   ADR：なし。本タスクの中で閉じる。
-   **変更の範囲（AC-06・AC-07）は、記録した diff を読んで確かめる。スクリプトは書かない。** 差分は 1 つの文書の数段落で、スクリプトを書くより、diff を読むほうが手間が少ないためである（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の「Proportion」）。
    -   ADR：なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `a2fbb8f0`（`origin/main`）で確かめた。

-   step 3 の「待ち受けの間」の項目は `chief.md:259-274`、`once` の項目は `chief.md:275-278`、答えの読み方は `chief.md:285-292` にある。
-   step 4 の冒頭の 1 文は `chief.md:296-297` にある。step 4 の問いは、中継する選択肢の選択（`chief.md:312-320`）、PR の詳細の後の問い（`chief.md:321-331`）、`/mkreq` の issue 番号（`chief.md:341-343`）、`停止を解く` の確認（`chief.md:347-350`）である。モデルの切り替えの後の問いは `chief.md:352-357` にある。
-   `chief.md` を読むガードは `cmd/chief/docs_test.go` の `TestDocumentedProtocolMatches`（`:205`）・`TestDocumentedInvocationsParse`（`:257`）・`TestNoDirectHerdrInvocation`（`:290`）・`TestDisplayValuesMatchTool`（`:469`）で、protocol 番号、呼び出しの形、herdr の直接の呼び出しがないこと、表示の値の表を読む。`TestDocumentedInvocationsParse` は、地の文の中でも `chief` で始まるコードスパンをすべて呼び出しとして読む。そのため、新しい文に不完全な呼び出しをコードスパンで書くと、このテストが落ちる（設計書 3.4 節）。`internal/chief/detail_test.go:312` のコメントも `chief.md` に触れるが、出力の欄の名前についてで、本タスクの文とは関わらない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: `chief.md` の文の書き換え（設計書 3 章、AC-01〜AC-08）

#### ステップ 1-1: step 4 の冒頭

**変えるファイル:** `.claude/commands/chief.md`

- [ ] 設計書 3.1 節のとおり、`chief.md:296-297` の 1 文を、対象の 4 つの問い、テキストで聞かないことと理由、テキストで示すもの、step 3 の選択肢の示し直しの形、モデルの切り替えの後の問いの扱い（待ち受けの間と `once`）を述べる段落に置き換える

**完了の条件:** 段落が AC-01〜AC-03 の内容を述べる。

#### ステップ 1-2: 提案の項目の issue 番号の問い

**変えるファイル:** `.claude/commands/chief.md`

- [ ] 設計書 3.2 節のとおり、「ask the human for the issue number first」を、選択肢が `送らない` と `ほかの項目を見る` の `AskUserQuestion` で聞き、番号を自由記述の答えに書くこと、数字だけでない答えではそう言って何も送らないこと、`送らない` と `ほかの項目を見る` を選んだ後の振る舞いに書き換える。`chief send` の行は変えない

**完了の条件:** 項目が AC-04 の内容を述べる。

#### ステップ 1-3: step 3 の「待ち受けの間」の項目

**変えるファイル:** `.claude/commands/chief.md`

- [ ] 設計書 3.3 節のとおり、`chief.md:259-274` の項目の終わりに、step 4 の問いはこの項目の例外で、待ち受けの間も `AskUserQuestion` で聞くことを述べる 1 文を足す

**完了の条件:** 項目が AC-05 の内容を述べる。

#### ステップ 1-4: 仕上げ

- [ ] `go test -tags test ./cmd/chief` を通す（AC-08）
- [ ] `make test` と `make lint` を通す
- [ ] 7 章の AC-06・AC-07 の diff の記録を書く

**完了の条件:** テストが通り、diff の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `chief.md` の 3 か所の書き換えと、diff の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-4 を含む。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（1 つのコマンドの定義の文の書き換えで、方式の選択も危険の高いステップもない）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   新しいテストは足さない。変わるのは文だけで、文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。
-   既存のガード（`go test -tags test ./cmd/chief`）が、呼び出しの形と protocol 番号が変わらないことを確かめる（AC-08）。
-   変更の範囲は、記録した diff の読み取りで確かめる（AC-06・AC-07）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 書き換えた文が、`chief` を呼び出す行を変えてしまう | AC-07 の diff の記録と、`TestDocumentedInvocationsParse` で確かめる |
| step 4 の新しい段落が、step 3 の原則と再び食い違って読める | 段落に理由を書き、step 3 から step 4 を参照する（設計書 1 章の設計原則）。PR のレビューで両方の節を読む |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-4）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR のレビューで、step 4 の冒頭が 4 つの問いを挙げ、待ち受けの間も `once` でも `AskUserQuestion` で聞くと述べることを読んで確かめる |
| AC-02 | `manual` | 同じく、step 4 の問いをテキストで聞かないこと、pane の選択肢と PR の詳細はテキストで示すこと、番号つきの一覧は step 3 の選択肢にだけ使い、示し直しはモードごとの step 3 の形で示すことを述べることを読んで確かめる |
| AC-03 | `manual` | 同じく、モデルの切り替えの後の問いが対象でなく、待ち受けの間は step 3 の一覧に示し、`once` では止まる前に `AskUserQuestion` で聞き、答えを今の「Sending again after a model switch」のとおりに扱うと述べることを読んで確かめる |
| AC-04 | `manual` | 同じく、提案の項目が、issue 番号を `送らない`・`ほかの項目を見る` の `AskUserQuestion` で聞くこと、数字だけでない答えではそう言って何も送らないこと、2 つの選択肢を選んだ後の振る舞い（`once` で示し直す項目がないときを含む）を述べることを読んで確かめる |
| AC-05 | `manual` | 同じく、step 3 の「待ち受けの間」の項目が、step 4 の問いを例外として述べ、step 4 を参照することを読んで確かめる |
| AC-06 | `diff` | 下の記録のファイルの一覧を、PR のレビューで読む |
| AC-07 | `diff` | 下の記録の `chief.md` の差分を、PR のレビューで読む。差分が 3 か所に限られ、`chief <subcommand> ...` の行が変わらないことを確かめる |
| AC-08 | `test` | `cmd/chief/docs_test.go`（`go test -tags test ./cmd/chief`） |

Diff record (AC-06):

- Compared paths: `.claude/commands/chief.md`

この記録は、パスの差分のほかに、差分全体のファイルの一覧（`File list`）を持つ。AC-06 はファイルの一覧で確かめる。比べるパスは 1 つ以上要るが、本タスクのディレクトリは本計画を含むので比べるパスにできない。そのため、比べるパスは `chief.md` とし、そのパスの差分は AC-07 の記録と同じものになる。

Diff record (AC-07):

- Compared paths: `.claude/commands/chief.md`

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-08 がすべて確かめられている。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   次に `/chief` を使うとき、PR の詳細を示した後の問いが `AskUserQuestion` で出ることを観察する。
