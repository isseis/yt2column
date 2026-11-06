# 設計への申し送り：用語「段階」（stage）を「ステージ」に改める

要件定義書 [01_requirements.md](01_requirements.md) の作成で出た、設計で決める事項を記す。`02_architecture.md` は、各項目の扱いを記録する。

## H-01: `cmd/yt2column/docs_test.go` のコメントと識別子の stage

本番の構成の表を確かめるテスト（`checkProfileTable`、`cmd/yt2column/docs_test.go:565-590`）は、コメントで表の行を「stage」と呼び（例: 「each stage has exactly one row」、38 行目の「the profile's configuration block and stage table」）、識別子 `profileStages` を使う。要件は、列名の literal とその失敗のメッセージを「工程」に変えること（AC-08）と、Go の変更をこのファイルに限ること（AC-16）だけを定める。コメントの stage を step に直すか、識別子の名前を変えるかは設計で決める。`CLAUDE.md:650` の「stage table」を「step table」に直す（AC-12）ので、38 行目のコメントもそれに揃えるかどうかを合わせて決める。

## H-02: 言い換えた文の文言

付録 A は、各箇所に割り当てる語を定めるが、語を変えたときの文の組み立て（助数詞の「つの」を補うか、例: 「3段階」→「3 つのステージ」、「2 段階を置き」→「2 つの工程を置き」）は定めない。ADR の題名を言い換えるときは、索引の題名と同じ文言にする（AC-07）。
