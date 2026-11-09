# 設計への申し送り（0075）

要件定義の際に挙がった、設計で決める事項を記録する。`02_architecture.md` は、各項目をどう扱ったかを記録する。

- **H-01: 軽量プロセスのためだけにある入力と手順を、残すか除くか。** `_lib/task-pr-handoff.md` の `ISSUE_KEYWORD`（`closes` を渡すのは `runissue.md` だけ）、Part B の step 3〜7 を単独で使う呼び出し方、`_context.md` の「Lightweight PR title」「Lightweight commit subject」の行が当たる。呼び出し元がなくなる入力は、除くのが YAGNI に合う。除くときは、残る呼び出し元の振る舞いが変わらないこと（要件定義書 F-003）を確かめる。
- **H-02: `runpr.md` の Lightweight PR の分類を除いたときの手順の組み立て。** 分類（`runpr.md:298-337`）、LW_ISSUE を閉じる手順（`:885` 付近）、`gh issue close` の承認（`:88` 付近）、`/runissue` が止まった後の再開の記述（`:120-141` 付近）が当たる。Standalone change の扱いを変えずに除く方法を決める（要件定義書 AC-06）。
- **H-03: guard のテストの書き換えと削除の分け方。** `codex_review_docs_test.go` の `runissue.md` と `mkreq.md` の Lightweight triage の箇所、`runissueLinkedPRsQuery`、`fixpr_docs_test.go` の `runissuePath` と mention check の対象の一覧が当たる。テストの関数を丸ごと消すものと、表の行を消すものを分け、`make coverage-compare` を行うコミットの単位を決める（要件定義書 AC-07・AC-13）。
- **H-04: `_checks/` のスクリプトの検索の形。** 軽量プロセスへの参照（要件定義書 3 章の冒頭）を見つける語の一覧と、参照に当たらない一致（`test_organization.md:14` など）の除き方を決める（要件定義書 AC-11）。
- **H-05: PR の分け方。** 約 3,000 行の削除と 20 を超えるファイルの書き換えを、何本の PR にするかを決める。途中の PR がマージされた `main` でも、コマンドが壊れた参照を持たない順序にする。
- **H-06: ラベルの削除を、どのコマンドのどの手順で行うか。** 最後の実装の PR のマージの後に行う手順の置き場所（計画書の最後のステップ、`/runpr` のマージの後の報告など）を決める（要件定義書 AC-09・AC-10）。
