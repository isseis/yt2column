# 設計書：/chief の step 4 の質問を AskUserQuestion で聞くことを明示する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-18 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 設計の全体像

本タスクでは、`/chief` の定義（`.claude/commands/chief.md`）の 3 か所の文を書き換える。step 4 の問いを `AskUserQuestion` で聞くことと、step 3 の番号つきの一覧との境目を明示する（[01_requirements.md](01_requirements.md) F-001）。`chief` の道具と、呼び出すサブコマンドと引数は変えない（F-002）。

**設計原則:**

-   **境目は両側に書く。** step 4 の冒頭が対象の問いと例外を挙げ、step 3 の「待ち受けの間」の項目が step 4 を参照する（AC-02・AC-05）。どちらの節から読んでも境目が分かるようにするためである。
-   **問いを列挙する。** step 4 の冒頭は、対象の問いを 4 つとも名前で挙げる（AC-01）。「follow-up questions」のような総称だけでは、どの問いが当たるかを読み手が決めることになるためである。
-   **理由も書く。** step 4 の冒頭は、テキストで聞くと、人が打った数字が step 3 の一覧の項目の番号として読まれうることを理由として書く。理由のない規則は、step 3 の原則と比べたときに軽く読まれやすいためである。

**設計の際に決めたこと（2026-11-18）:**

-   **`/mkreq` の issue 番号を聞く `AskUserQuestion` の選択肢は、「送らない」と「ほかの項目を見る」とする。人は issue 番号を自由記述の答えに書く。** PR の詳細の後の問いの「送らない」「ほかの PR を見る」（`chief.md:321-331`）と同じ形にするためである。`AskUserQuestion` は選択肢を 2 つ以上求めるので、選ぶ形の選択肢が 2 つ要る。「ほかの項目を見る」は、PR の詳細の後の「ほかの PR を見る」と同じく、残りの項目を step 3 のとおり示し直す。
    -   採らなかった案：「送らない」だけにする案。`AskUserQuestion` の選択肢の数の下限に届かない。
    -   ADR：なし。本タスクの中で閉じる。

概念モデルの Mermaid の図は省く。変わるのはコマンドの定義の文だけで、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

変更するファイルは `.claude/commands/chief.md` だけである。書き換える 3 か所と、書き換えた後の文の要旨は次のとおりである。文の細部は実装で決めてよいが、各 AC が求める内容を落とさない。

### 3.1. step 4 の冒頭（`chief.md:296-297`、AC-01〜AC-03）

今の 1 文（「The follow-up questions below are asked with `AskUserQuestion` in both modes, since the human has just chosen and is answering.」）を、次の内容の段落に置き換える。

-   この節で人に聞く問いは、待ち受けの間も `once` でも、すべて `AskUserQuestion` で聞く。人が選んだ直後に答える問いだからである。
-   対象の問いは、中継する選択肢の選択、PR の詳細を示した後の問い、`停止を解く` の確認、`/mkreq` の提案での issue 番号の 4 つである。
-   これらの問いを、テキストで書かない。step 3 の番号つきの一覧（`[<number>] <description>` の行）は、ラウンドの最後に出す選択肢にだけ使う。テキストで聞くと、人がそれに答えて打った数字が、step 3 の一覧の項目の番号として読まれうる（step 3 の「Reading the answer」）。
-   モデルの切り替えの後に送り直すかどうかの問い（step 4 の末尾の「Sending again after a model switch」）は対象でない。ラウンドをまたいで待つ問いで、step 3 の一覧に示す。

### 3.2. step 4 の提案の項目（`chief.md:341-343`、AC-04）

「When it is `mkreq`, ask the human for the issue number first, then run ...」の「ask the human for the issue number first」を、次の内容に書き換える。`chief send` の行は変えない。

-   issue 番号は、選択肢が「送らない」と「ほかの項目を見る」の `AskUserQuestion` で聞く。人は番号を自由記述の答えに書く。
-   自由記述の答えが数字だけでなければ、選択として扱わない。そう言って、何も送らない。
-   「ほかの項目を見る」は、残りの項目を step 3 のとおり示し直す。
-   数字だけの答えのときに、今の `chief send --workspace <workspace> --command mkreq --issue <issue> --protocol 3` を呼ぶ。issue が存在するか、閉じていないかなどは、今のとおり道具が確かめる。

### 3.3. step 3 の「待ち受けの間」の項目（`chief.md:259-274`、AC-05）

項目の終わりに、次の内容の 1 文を足す。

-   この項目の規則はラウンドの最後に出す選択肢のためのもので、人が項目を選んだ直後に聞く step 4 の問いは、待ち受けの間も `AskUserQuestion` で聞く（step 4）。

### 3.4. 変えないもの

-   `chief` を呼び出す行（`chief <subcommand> ...` のコードスパン）と、その `--protocol 3`（AC-07）。
-   `cmd/chief/docs_test.go` のガードが読む値と構造（呼び出しの形、protocol 番号、表示の値の表）。どのガードも文の中身を読まない（AC-08）。

## 4. エラーハンドリング設計

N/A。道具のエラーの扱いを変えない。issue 番号でない自由記述の答えの扱いは 3.2 節にある。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の条件のどれにも当たらない（[01_requirements.md](01_requirements.md) 4.2 節）。chief が送る入力と、人が選ぶまで送らない規則（`chief.md:34-37`）は変えない。

## 6. 処理フローの詳細

N/A。人が項目を選んでから送るまでの流れは変わらない。変わるのは、step 4 の問いをどの形で出すかを述べる文だけである。

## 7. テスト戦略

-   文の中身（AC-01〜AC-05）は、PR のレビューで読んで確かめる（`manual`）。文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。
-   変更の範囲（AC-06・AC-07）は、記録した diff を PR のレビューで読んで確かめる（`diff`。`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の「Recorded diff reading」）。差分は 1 つの文書の数段落で、スクリプトを書くより diff を読むほうが手間が少ない。
-   既存のガード（AC-08）は、`go test -tags test ./cmd/chief` で確かめる。

## 8. 実装の優先順位

1 フェーズ、1 本の PR で、3.1〜3.3 節の 3 か所を書き換える。

## 9. 将来の拡張性

step 4 に新しい問いを足すときは、3.1 節の段落の対象の問いの一覧にも足す。
