# 設計への申し送り

`01_requirements.md` を書く際に出た、設計で決めることを挙げる。`02_architecture.md` は、各項目について、採った方法か、当てはまらない理由を記録する。

## H-01: `mkreq.md` に AC の書き方の規則を書くか

AC の書き方の規則（AC-05）は、規約の §1 に置く。`mkreq.md` の step 6 は、規約の §1 に従うと定めている。そのため、`mkreq.md` の本文に同じ規則を書き足すかどうかは、規則の正本を 1 つに保つことと、`/mkreq` が規則を見落とさないこととの兼ね合いで決める。人は、AC の並べ挙げを指摘する項目を `mkreq.md` のチェックリストに足す案を選ばなかった（要件定義書 1 章）。設計は、この選択と矛盾しない形にする。

- 関連: F-003、AC-05、AC-10

## H-02: 文書を読む既存の guard への影響

`mkplan.md`・`runplan.md`・`weakreview.md`・`CLAUDE.md`・規約を読む Go の guard（`cmd/yt2column` の `TestRunprDocContract`、`TestInstructionDocPaths`、`CLAUDE.md` の対象を読む guard など）が、変える節の見出しや値を読んでいないかを調べる。読んでいるときは、その guard を同じ PR で直すか、変え方を guard が読む値に触れない形にする。

- 関連: AC-10

## H-03: 規則の置き場所と言い回し

AC-01〜AC-08 の規則を、規約と各コマンドの定義のどの節に、どの言い回しで書くか。規則を 1 か所に置き、ほかから参照するか、各所に短く書くかを決める。チェックリストの項目は、`_lib/simple-process.md` がそのまま写して使うので（AC-09）、項目の形を保つ。

- 関連: F-001〜F-006

## H-04: 記録した差分の読み取りの、計画での形

計画の確かめの行に、`git diff` の出力・実行したコマンド・比べたパス・コミットの ID をどう記録するか（コードブロックの形、出力が長いときの扱い）。記録し直しが要るかを、PR を作る前とマージの前に、どの手順のどこで確かめるかも決める（AC-03）。マージの前の記録し直しを `runpr.md` か `fixpr.md` が受け持つときは、そのファイルも変える（AC-10）。

- 関連: F-002、AC-02、AC-03

## H-05: guard-test literal exception の言い回し

今の例外は、範囲を Go の guard のテスト（`_test.go`）と `internal/doctestutil` とし、拡張の guard のテストにも当てはめている（`CLAUDE.md`「Go Idioms」）。`_checks/` のスクリプトは、Go に限らずシェルでもありうる。`_checks/` ではリテラルの種類を限らない（AC-07）。例外の範囲・理由・境界を、既存の書き方に合わせてどう足すか決める。また、`mkplan.md`・`runplan.md`・`weakreview.md` の英語だけを求める規則と、`runplan.md` の非 ASCII の検査（step 6.5 の Check 2）が、例外に当たるリテラルを指摘しない形を決める（各所に書くか、`CLAUDE.md`「Go Idioms」を参照するか）。

- 関連: F-005、AC-07
