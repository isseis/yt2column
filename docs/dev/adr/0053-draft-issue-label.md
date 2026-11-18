# ADR-0053: 下書きの issue をラベル draft で宣言し、/mkreq と chief はそこからタスクを始めない

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

まだ着手しない issue を、下書きとして先に起票しておきたい場合がある。下書きの issue からタスクを始めると、タスク番号の記録の tag が残り、取り消せない（[ADR-0051](0051-number-reservation.md)）。chief が下書きの issue で workspace を作ると、タスクができないので削除の提案の対象にもならず、残り続ける。そのため、タスクを始める道具は、issue が下書きかを機械的に判定できなければならない。題名の印（「【下書き】」など）で示すと、判定は題名の文字列を読むことになり、CLAUDE.md の「Declare, don't infer」に反する。

## 決定

-   **issue が下書きであることは、GitHub のラベル `draft` で宣言する。題名や本文の印では判定しない。** 振る舞いを、人が書いた文字列の中身ではなく、宣言された値で選ぶためである（CLAUDE.md の「Declare, don't infer」）。ラベルの定義の正本は [_context.md](../../../.claude/commands/_context.md) の「Process convention」の表にある。
-   **下書きの印は、名前がバイト列でちょうど `draft` と等しいラベルだけとし、開いた issue だけを下書きの issue とする。閉じた issue は、ラベルによらず閉じた issue として扱う。** `/mkreq` と chief の判定が食い違うと、片方が通した issue でもう片方が止まるためである。閉じた issue を先に見るのは、閉じていることのほうが、人が番号の打ち間違いに気づきやすいためである（[0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) 1.5）。
-   **`/mkreq` は、下書きの issue からタスクを始めない。step 1 で、タスク番号を決める前に止まり、下書きの issue であることと、ラベルを外せば正式になることを報告する。ラベルを読めないときも止まる。** タスク番号の記録は取り消せないので、下書きの issue から番号を決めないためである。手順は [mkreq.md](../../../.claude/commands/mkreq.md) の step 1 にあり、要件は [0104 要件定義書](../../tasks/0104_draft_issue_label/01_requirements.md) の F-001 にある。
-   **chief は、下書きの issue で workspace を作らない。** 決定とその理由は [ADR-0046](0046-chief-of-staff.md) にある。
-   **下書きの issue を正式にするには、人がラベル `draft` を外す。道具はラベルを付け外ししない。** 着手してよいと判断するのは人だからである。
-   **食い違い:** `mkreq.md` の step 1 は、まだ下書きの issue で止まらない。この ADR の決定に合っていない（[#934](https://github.com/isseis/yt2column/issues/934)）。

## 採らなかった案

-   **題名の印（「【下書き】」など）で下書きを示す。** 判定が題名の文字列を読むことになり、CLAUDE.md の「Declare, don't infer」に反する（[0104 要件定義書](../../tasks/0104_draft_issue_label/01_requirements.md)「issue #934 で決めたこと」）。
-   **大文字と小文字を区別せずにラベルの名前を比べる。** chief の判定と食い違う（[0104 要件定義書](../../tasks/0104_draft_issue_label/01_requirements.md)「要件定義の際に決めたこと（2026-11-18）」）。
-   **この方針を ADR-0046 に書き足す。** ADR-0046 は chief の ADR で、`/mkreq` の振る舞いを入れると主題が chief からずれる（[0104 要件定義書](../../tasks/0104_draft_issue_label/01_requirements.md)「要件定義の際に決めたこと（2026-11-18）」）。

## 結果

-   ラベルの名前 `draft` は、`_context.md` と chief のコード（`internal/chief/issue.go`）の 2 か所にある。名前や判定の規則を変えるときは、`_context.md`・`mkreq.md`・chief を同じ PR で変える。
-   下書きの issue を起票するときは、人がラベル `draft` を付ける。付け忘れた issue は、`/mkreq` と chief から見て正式な issue である。

## 経緯

- 2026-11-18 下書きの issue をラベル `draft` で宣言し、名前がちょうど `draft` のラベルが付いた開いた issue から `/mkreq` がタスクを始めないことにした。chief が workspace を作らないことは ADR-0046 に置いた（[0104 要件定義書](../../tasks/0104_draft_issue_label/01_requirements.md)「issue #934 で決めたこと」「要件定義の際に決めたこと（2026-11-18）」）
