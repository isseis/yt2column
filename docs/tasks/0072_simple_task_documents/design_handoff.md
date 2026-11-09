# 設計への申し送り：開発文書を変更の規模に合わせて簡易にとどめる分岐

要件定義書（[01_requirements.md](01_requirements.md)）を書く際に挙がった、設計で決めることを記録する。`02_architecture.md` は、各項目をどう扱ったかを記す。

## H-01: 進め方の記録の形と読み方

AC-03 は、後のコマンドが文の意味を解釈せずに進め方を読み取れる形で記録することを求める。記録を置く場所（Document Status の表の行、概要の決まった行など）、取りうる値、値が欠けているか壊れているときの扱い（AC-17 の、記録を持たない文書との区別を含む）を設計で決める。`CLAUDE.md` の「Declare, don't infer」と、`/runpr` の承認が Document Status の表の行を読む仕組み（`docs/dev/conventions/requirements_process.md`「Cell text」）との関係も考える。コマンドは、記録を作業中のブランチではなく `origin/main` の版から読むか（`/mkarch` の step 1.5 の後に読む版）も決める。

## H-02: 3 文書のファイルの構成

簡易な進め方でも 3 つのファイルに分けるか、1 つにまとめるかを設計で決める。`/runpr` の承認と次のコマンドの選び方（`.claude/commands/runpr.md:349`・`:378-380`）、計画書の PR の定義（`.claude/commands/_context.md`「Plan PR」）、`/runplan`、タスクの識別の規則（`docs/dev/conventions/task_identification.md`）は、3 つのファイル名を読む。3 つに分ければこれらを変えずに使える見込みがある。

## H-03: `/mkreq` の定義の分け方

簡易な進め方では、`/mkreq` が、今の `/mkarch`・`/mkplan` の決定の確かめ方、文書の書き方、レビューのチェックリスト、PR の境界の設計を使う（AC-04・AC-08）。これらを `/mkreq` に書き写すと、同じ手順が 2 か所になる（要件定義書 4.5）。`/mkarch`・`/mkplan` の手順を `_lib/` に移して共有するか、`/mkreq` から `/mkarch`・`/mkplan` の該当する step を参照するかなど、定義の分け方を設計で決める。

## H-04: 3 文書のレビューの単位

AC-06 は 3 文書のレビューを 1 人のレビュアーで行うことを求める。3 文書をまとめて 1 回レビューするか、文書ごとに 1 回ずつレビューするか、各文書のチェックリスト（`/mkreq` の step 7、`/mkarch` の step 8、`/mkplan` の step 8）をどう渡すかを設計で決める。

## H-05: 通常の進め方へ戻す手順を誰が行うか

AC-14 の戻し方を、どのコマンドがどの契機で行うかを設計で決める。候補は、レビューのスレッドを受けた `/fixpr`、`/runpr` のマージの質問の選択肢、人の手での編集である。`/runpr` の次のコマンドの選び方（`.claude/commands/runpr.md:378-380`）が、戻した後の PR で `/mkarch` を勧めること（AC-15）も確かめる。PR の題名と説明（`.claude/commands/_context.md`「Task-document PR title」）を戻した後にどう直すかも決める。

## H-06: 3 文書の PR の題名と説明

`.claude/commands/_lib/task-pr-handoff.md` の Part A・Part B は、1 つの文書を作るコマンドを前提に書かれている。3 文書の PR のブランチ名、PR の題名（`docs(<task-id>): ...`）、コミットの件名（今は文書ごとに `docs(NNNN): draft requirements for ...` などの形）を設計で決める。

## H-07: 既存の文書のテスト

コマンドの定義と規約を読む既存の Go のテスト（`TestRunprDocContract` など。`CLAUDE.md`「Development Notes」）が、変更した定義で失敗しないかを確かめる。簡易な進め方について値と構造を確かめるテストを足すかは、`CLAUDE.md` の「Document tests check values, structure, and executable examples, never prose.」と [ADR-0016](../../dev/adr/0016-document-test-policy.md) に照らして決める。
