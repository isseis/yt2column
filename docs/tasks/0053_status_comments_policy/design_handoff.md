# 設計への申し送り：0053 タスク文書の Comments と変更履歴

要件定義（[01_requirements.md](01_requirements.md)）の作成中に挙がった、設計で決める事項。`02_architecture.md` は、項目ごとに採った方法、または当てはまらない理由を記録する。

- **H-01: `/fixpr` が編集上の訂正を PR 本文に追記する手段。** 現在、`/fixpr` が PR 本文を書き換えるのは Phase 6 で「大きく古い」と判断したときだけで、`fixprguard edit-pr` を通る（`.claude/commands/fixpr.md:680-723`）。AC-03 は、古くない場合でも訂正の説明を追記するよう求める。修正の Phase から訂正の一覧をどう受け渡すか、Phase 6 の判定とどう組み合わせるか、既存の本文の「編集上の訂正」の節にどう追記するか（PR 本文は信頼できないデータとして扱う。`fixpr.md:690-692`）を決める。
- **H-02: PR を作った後に PR 番号を書き足す手順（AC-13）。** `runplan.md` には PR の URL を計画書に書く AFTER_CREATE がある（`.claude/commands/_lib/task-pr-handoff.md` の Inputs）。`mkarch.md` と `mkplan2.md` には、まだない。書き足す先が `Comments` ではなく変更履歴の行であることを踏まえ、AFTER_CREATE を使うか、ほかの方法を採るかを決める。
- **H-03: 変更履歴の 1 行の書式（AC-08）。** 日付、種類、要旨、承認者、PR 番号の並べ方と、種類の表記（例: 承認・決定の変更・再承認・取り下げ）を定める。規則の英語の本文に、日本語の見出しと行の例をどう書くかも含む。
- **H-04: `runpr.md` の PRE_MERGE の「既に記録したものは除く」との関係。** PRE_MERGE は既に記録したものを除く（`.claude/commands/runpr.md:204-206`）。中断の後にもう一度実行したとき、変更履歴に承認の行を二重に書かない判定をどう定めるかを決める。
- **H-05: 確かめの方法。** AC-01〜AC-15 と AC-17 は規則とコマンドの定義の文言なので、PR のレビューで読んで確かめる（`manual`）想定である。AC-16 は変更の前後の関係なので、`_checks/` の diff の確かめ（`docs/dev/conventions/requirements_process.md` §4）の候補になる。文書の文言を照合する恒久のテストは加えない（`CLAUDE.md`「Document tests check values, structure, and executable examples, never prose」）。
- **H-06: 書き込む箇所の洗い出し。** 要件定義では、`Comments` に触れるコマンドの定義を `grep` で探して 2.2 に挙げた。設計では、PR 本文の書き方を定めている箇所（`task-pr-handoff.md` Part B step 3、`runpr.md` の単独の変更の PR、`runplan.md` の単一 PR の場合）と、`CLAUDE.md`・`AGENTS.md`・`docs/dev/guides/` に関係する記述がないかを改めて確かめる。
